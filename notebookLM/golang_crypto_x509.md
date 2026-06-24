# Domain Architecture: crypto/x509

## Layout Topology
```text
crypto/x509/
├── internal
│   └── macos
│       ├── corefoundation.go
│       ├── corefoundation.s
│       ├── security.go
│       └── security.s
├── pkix
│   └── pkix.go
├── cert_pool.go
├── constraints.go
├── oid.go
├── parser.go
├── pem_decrypt.go
├── pkcs1.go
├── pkcs8.go
├── platform_root_cert.pem
├── platform_root_key.pem
├── root.go
├── root_aix.go
├── root_bsd.go
├── root_darwin.go
├── root_linux.go
├── root_plan9.go
├── root_solaris.go
├── root_unix.go
├── root_wasm.go
├── root_windows.go
├── sec1.go
├── test-file.crt
├── verify.go
├── x509.go
├── x509_string.go
└── x509_test_import.go
```

## Source Stream Aggregation

// === FILE: references/go/src/crypto/x509/cert_pool.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"bytes"
	"crypto/sha256"
	"encoding/pem"
	"sync"
)

type sum224 [sha256.Size224]byte

// CertPool is a set of certificates.
type CertPool struct {
	byName map[string][]int // cert.RawSubject => index into lazyCerts

	// lazyCerts contains funcs that return a certificate,
	// lazily parsing/decompressing it as needed.
	lazyCerts []lazyCert

	// haveSum maps from sum224(cert.Raw) to true. It's used only
	// for AddCert duplicate detection, to avoid CertPool.contains
	// calls in the AddCert path (because the contains method can
	// call getCert and otherwise negate savings from lazy getCert
	// funcs).
	haveSum map[sum224]bool

	// systemPool indicates whether this is a special pool derived from the
	// system roots. If it includes additional roots, it requires doing two
	// verifications, one using the roots provided by the caller, and one using
	// the system platform verifier.
	systemPool bool
}

// lazyCert is minimal metadata about a Cert and a func to retrieve it
// in its normal expanded *Certificate form.
type lazyCert struct {
	// rawSubject is the Certificate.RawSubject value.
	// It's the same as the CertPool.byName key, but in []byte
	// form to make CertPool.Subjects (as used by crypto/tls) do
	// fewer allocations.
	rawSubject []byte

	// constraint is a function to run against a chain when it is a candidate to
	// be added to the chain. This allows adding arbitrary constraints that are
	// not specified in the certificate itself.
	constraint func([]*Certificate) error

	// getCert returns the certificate.
	//
	// It is not meant to do network operations or anything else
	// where a failure is likely; the func is meant to lazily
	// parse/decompress data that is already known to be good. The
	// error in the signature primarily is meant for use in the
	// case where a cert file existed on local disk when the program
	// started up is deleted later before it's read.
	getCert func() (*Certificate, error)
}

// NewCertPool returns a new, empty CertPool.
func NewCertPool() *CertPool {
	return &CertPool{
		byName:  make(map[string][]int),
		haveSum: make(map[sum224]bool),
	}
}

// len returns the number of certs in the set.
// A nil set is a valid empty set.
func (s *CertPool) len() int {
	if s == nil {
		return 0
	}
	return len(s.lazyCerts)
}

// cert returns cert index n in s.
func (s *CertPool) cert(n int) (*Certificate, func([]*Certificate) error, error) {
	cert, err := s.lazyCerts[n].getCert()
	return cert, s.lazyCerts[n].constraint, err
}

// Clone returns a copy of s.
func (s *CertPool) Clone() *CertPool {
	p := &CertPool{
		byName:     make(map[string][]int, len(s.byName)),
		lazyCerts:  make([]lazyCert, len(s.lazyCerts)),
		haveSum:    make(map[sum224]bool, len(s.haveSum)),
		systemPool: s.systemPool,
	}
	for k, v := range s.byName {
		indexes := make([]int, len(v))
		copy(indexes, v)
		p.byName[k] = indexes
	}
	for k := range s.haveSum {
		p.haveSum[k] = true
	}
	copy(p.lazyCerts, s.lazyCerts)
	return p
}

// SystemCertPool returns a copy of the system cert pool.
//
// The environment variables SSL_CERT_FILE and SSL_CERT_DIR can be used to
// override the system default locations for the SSL certificate file and SSL
// certificate files directory, respectively. The latter can be a
// colon-separated list, or a semicolon-separated list on Windows. On platforms
// which have system APIs for certificate verification (macOS and Windows),
// setting SSL_CERT_FILE or SSL_CERT_DIR will prevent those APIs from being
// used, unless the x509sslcertoverrideplatform=0 GODEBUG setting is used. (This
// changed in Go 1.27.)
//
// Any mutations to the returned pool are not written to disk and do not affect
// any other pool returned by SystemCertPool.
//
// New changes in the system cert pool might not be reflected in subsequent calls.
func SystemCertPool() (*CertPool, error) {
	if sysRoots := systemRootsPool(); sysRoots != nil {
		return sysRoots.Clone(), nil
	}

	return loadSystemRoots()
}

type potentialParent struct {
	cert       *Certificate
	constraint func([]*Certificate) error
}

// findPotentialParents returns the certificates in s which might have signed
// cert.
func (s *CertPool) findPotentialParents(cert *Certificate) []potentialParent {
	if s == nil {
		return nil
	}

	// consider all candidates where cert.Issuer matches cert.Subject.
	// when picking possible candidates the list is built in the order
	// of match plausibility as to save cycles in buildChains:
	//   AKID and SKID match
	//   AKID present, SKID missing / AKID missing, SKID present
	//   AKID and SKID don't match
	var matchingKeyID, oneKeyID, mismatchKeyID []potentialParent
	for _, c := range s.byName[string(cert.RawIssuer)] {
		candidate, constraint, err := s.cert(c)
		if err != nil {
			continue
		}
		kidMatch := bytes.Equal(candidate.SubjectKeyId, cert.AuthorityKeyId)
		switch {
		case kidMatch:
			matchingKeyID = append(matchingKeyID, potentialParent{candidate, constraint})
		case (len(candidate.SubjectKeyId) == 0 && len(cert.AuthorityKeyId) > 0) ||
			(len(candidate.SubjectKeyId) > 0 && len(cert.AuthorityKeyId) == 0):
			oneKeyID = append(oneKeyID, potentialParent{candidate, constraint})
		default:
			mismatchKeyID = append(mismatchKeyID, potentialParent{candidate, constraint})
		}
	}

	found := len(matchingKeyID) + len(oneKeyID) + len(mismatchKeyID)
	if found == 0 {
		return nil
	}
	candidates := make([]potentialParent, 0, found)
	candidates = append(candidates, matchingKeyID...)
	candidates = append(candidates, oneKeyID...)
	candidates = append(candidates, mismatchKeyID...)
	return candidates
}

func (s *CertPool) contains(cert *Certificate) bool {
	if s == nil {
		return false
	}
	return s.haveSum[sha256.Sum224(cert.Raw)]
}

// AddCert adds a certificate to a pool.
func (s *CertPool) AddCert(cert *Certificate) {
	if cert == nil {
		panic("adding nil Certificate to CertPool")
	}
	s.addCertFunc(sha256.Sum224(cert.Raw), string(cert.RawSubject), func() (*Certificate, error) {
		return cert, nil
	}, nil)
}

// addCertFunc adds metadata about a certificate to a pool, along with
// a func to fetch that certificate later when needed.
//
// The rawSubject is Certificate.RawSubject and must be non-empty.
// The getCert func may be called 0 or more times.
func (s *CertPool) addCertFunc(rawSum224 sum224, rawSubject string, getCert func() (*Certificate, error), constraint func([]*Certificate) error) {
	if getCert == nil {
		panic("getCert can't be nil")
	}

	// Check that the certificate isn't being added twice.
	if s.haveSum[rawSum224] {
		return
	}

	s.haveSum[rawSum224] = true
	s.lazyCerts = append(s.lazyCerts, lazyCert{
		rawSubject: []byte(rawSubject),
		getCert:    getCert,
		constraint: constraint,
	})
	s.byName[rawSubject] = append(s.byName[rawSubject], len(s.lazyCerts)-1)
}

// AppendCertsFromPEM attempts to parse a series of PEM encoded certificates.
// It appends any certificates found to s and reports whether any certificates
// were successfully parsed.
//
// On many Linux systems, /etc/ssl/cert.pem will contain the system wide set
// of root CAs in a format suitable for this function.
func (s *CertPool) AppendCertsFromPEM(pemCerts []byte) (ok bool) {
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}

		certBytes := block.Bytes
		cert, err := ParseCertificate(certBytes)
		if err != nil {
			continue
		}
		var lazyCert struct {
			sync.Once
			v *Certificate
		}
		s.addCertFunc(sha256.Sum224(cert.Raw), string(cert.RawSubject), func() (*Certificate, error) {
			lazyCert.Do(func() {
				// This can't fail, as the same bytes already parsed above.
				lazyCert.v, _ = ParseCertificate(certBytes)
				certBytes = nil
			})
			return lazyCert.v, nil
		}, nil)
		ok = true
	}

	return ok
}

// Subjects returns a list of the DER-encoded subjects of
// all of the certificates in the pool.
//
// Deprecated: if s was returned by [SystemCertPool], Subjects
// will not include the system roots.
func (s *CertPool) Subjects() [][]byte {
	res := make([][]byte, s.len())
	for i, lc := range s.lazyCerts {
		res[i] = lc.rawSubject
	}
	return res
}

// Equal reports whether s and other are equal.
func (s *CertPool) Equal(other *CertPool) bool {
	if s == nil || other == nil {
		return s == other
	}
	if s.systemPool != other.systemPool || len(s.haveSum) != len(other.haveSum) {
		return false
	}
	for h := range s.haveSum {
		if !other.haveSum[h] {
			return false
		}
	}
	return true
}

// AddCertWithConstraint adds a certificate to the pool with the additional
// constraint. When Certificate.Verify builds a chain which is rooted by cert,
// it will additionally pass the whole chain to constraint to determine its
// validity. If constraint returns a non-nil error, the chain will be discarded.
// constraint may be called concurrently from multiple goroutines.
func (s *CertPool) AddCertWithConstraint(cert *Certificate, constraint func([]*Certificate) error) {
	if cert == nil {
		panic("adding nil Certificate to CertPool")
	}
	s.addCertFunc(sha256.Sum224(cert.Raw), string(cert.RawSubject), func() (*Certificate, error) {
		return cert, nil
	}, constraint)
}

```

// === FILE: references/go/src/crypto/x509/constraints.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"bytes"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strings"
)

// This file contains the data structures and functions necessary for
// efficiently checking X.509 name constraints. The method for constraint
// checking implemented in this file is based on a technique originally
// described by davidben@google.com.
//
// The basic concept is based on the fact that constraints describe possibly
// overlapping subtrees that we need to match against. If sorted in lexicographic
// order, and then pruned, removing any subtrees that overlap with preceding
// subtrees, a simple binary search can be used to find the nearest matching
// prefix. This reduces the complexity of name constraint checking from
// quadratic to log linear complexity.
//
// A close reading of RFC 5280 may suggest that constraints could also be
// implemented as a trie (or radix tree), which would present the possibility of
// doing construction and matching in linear time, but the memory cost of
// implementing them is actually quite high, and in the worst case (where each
// node has a high number of children) can be abused to require a program to use
// significant amounts of memory. The log linear approach taken here is
// extremely cheap in terms of memory because we directly alias the already
// parsed constraints, thus avoiding the need to do significant additional
// allocations.
//
// The basic data structure is nameConstraintsSet, which implements the sorting,
// pruning, and querying of the prefix sets.
//
// In order to check IP, DNS, URI, and email constraints, we need to use two
// different techniques, one for IP addresses, which is quite simple, and one
// for DNS names, which additionally compose the portions of URIs and emails we
// care about (technically we also need some special logic for email addresses
// as well for when constraints comprise of full email addresses) which is
// slightly more complex.
//
// IP addresses use two nameConstraintsSets, one for IPv4 addresses and one for
// IPv6 addresses, with no additional logic.
//
// DNS names require some extra logic in order to handle the distinctions
// between permitted and excluded subtrees, as well as for wildcards, and the
// semantics of leading period constraints (i.e. '.example.com'). This logic is
// implemented in the dnsConstraints type.
//
// Email addresses also require some additional logic, which does not make use
// of nameConstraintsSet, to handle constraints which define full email
// addresses (i.e. 'test@example.com'). For bare domain constraints, we use the
// dnsConstraints type described above, querying the domain portion of the email
// address. For full email addresses, we also hold a map of email addresses with
// the domain portion of the email lowercased, since it is case insensitive. When
// looking up an email address in the constraint set, we first check the full
// email address map, and if we don't find anything, we check the domain portion
// of the email address against the dnsConstraints.

type nameConstraintsSet[T *net.IPNet | string, V net.IP | string] struct {
	set []T
}

// sortAndPrune sorts the constraints using the provided comparison function, and then
// prunes any constraints that are subsets of preceding constraints using the
// provided subset function.
func (nc *nameConstraintsSet[T, V]) sortAndPrune(cmp func(T, T) int, subset func(T, T) bool) {
	if len(nc.set) < 2 {
		return
	}

	slices.SortFunc(nc.set, cmp)

	if len(nc.set) < 2 {
		return
	}
	writeIndex := 1
	for readIndex := 1; readIndex < len(nc.set); readIndex++ {
		if !subset(nc.set[writeIndex-1], nc.set[readIndex]) {
			nc.set[writeIndex] = nc.set[readIndex]
			writeIndex++
		}
	}
	nc.set = nc.set[:writeIndex]
}

// search does a binary search over the constraints set for the provided value
// s, using the provided comparison function cmp to find the lower bound, and
// the match function to determine if the found constraint is a prefix of s. If
// a matching constraint is found, it is returned along with true. If no
// matching constraint is found, the zero value of T and false are returned.
func (nc *nameConstraintsSet[T, V]) search(s V, cmp func(T, V) int, match func(T, V) bool) (lowerBound T, exactMatch bool) {
	if len(nc.set) == 0 {
		return lowerBound, false
	}
	// Look for the lower bound of s in the set.
	i, found := slices.BinarySearchFunc(nc.set, s, cmp)
	// If we found an exact match, return it
	if found {
		return nc.set[i], true
	}

	if i < 0 {
		return lowerBound, false
	}

	var constraint T
	if i == 0 {
		constraint = nc.set[0]
	} else {
		constraint = nc.set[i-1]
	}
	if match(constraint, s) {
		return constraint, true
	}
	return lowerBound, false
}

func ipNetworkSubset(a, b *net.IPNet) bool {
	if !a.Contains(b.IP) {
		return false
	}
	broadcast := make(net.IP, len(b.IP))
	for i := range b.IP {
		broadcast[i] = b.IP[i] | (^b.Mask[i])
	}
	return a.Contains(broadcast)
}

func ipNetworkCompare(a, b *net.IPNet) int {
	i := bytes.Compare(a.IP, b.IP)
	if i != 0 {
		return i
	}
	return bytes.Compare(a.Mask, b.Mask)
}

func ipBinarySearch(constraint *net.IPNet, target net.IP) int {
	return bytes.Compare(constraint.IP, target)
}

func ipMatch(constraint *net.IPNet, target net.IP) bool {
	return constraint.Contains(target)
}

type ipConstraints struct {
	// NOTE: we could store IP network prefixes as a pre-processed byte slice
	// (i.e. by masking the IP) and doing the byte prefix checking using faster
	// techniques, but this would require allocating new byte slices, which is
	// likely significantly more expensive than just operating on the
	// pre-allocated *net.IPNet and net.IP objects directly.

	ipv4 *nameConstraintsSet[*net.IPNet, net.IP]
	ipv6 *nameConstraintsSet[*net.IPNet, net.IP]
}

func newIPNetConstraints(l []*net.IPNet) interface {
	query(net.IP) (*net.IPNet, bool)
} {
	if len(l) == 0 {
		return nil
	}
	var ipv4, ipv6 []*net.IPNet
	for _, n := range l {
		// Subtrees may carry non-zero host bits. Sort and search need the masked
		// network address, so use a copy and leave the parsed constraint as encoded.
		if masked := n.IP.Mask(n.Mask); masked != nil && !masked.Equal(n.IP) {
			n = &net.IPNet{IP: masked, Mask: n.Mask}
		}
		if len(n.IP) == net.IPv4len {
			ipv4 = append(ipv4, n)
		} else {
			ipv6 = append(ipv6, n)
		}
	}
	var v4c, v6c *nameConstraintsSet[*net.IPNet, net.IP]
	if len(ipv4) > 0 {
		v4c = &nameConstraintsSet[*net.IPNet, net.IP]{
			set: ipv4,
		}
		v4c.sortAndPrune(ipNetworkCompare, ipNetworkSubset)
	}
	if len(ipv6) > 0 {
		v6c = &nameConstraintsSet[*net.IPNet, net.IP]{
			set: ipv6,
		}
		v6c.sortAndPrune(ipNetworkCompare, ipNetworkSubset)
	}
	return &ipConstraints{ipv4: v4c, ipv6: v6c}
}

func (ipc *ipConstraints) query(ip net.IP) (*net.IPNet, bool) {
	var c *nameConstraintsSet[*net.IPNet, net.IP]
	if len(ip) == net.IPv4len {
		c = ipc.ipv4
	} else {
		c = ipc.ipv6
	}
	if c == nil {
		return nil, false
	}
	return c.search(ip, ipBinarySearch, ipMatch)
}

// dnsHasSuffix case-insensitively checks if DNS name b is a label suffix of DNS
// name a, meaning that example.com is not considered a suffix of
// testexample.com, but is a suffix of test.example.com.
//
// dnsHasSuffix supports the URI "leading period" constraint semantics, which
// while not explicitly defined for dNSNames in RFC 5280, are widely supported
// (see errata 5997). In particular, a constraint of ".example.com" is
// considered to only match subdomains of example.com, but not example.com
// itself.
//
// a and b must both be non-empty strings representing (mostly) valid DNS names.
func dnsHasSuffix(a, b string) bool {
	lenA := len(a)
	lenB := len(b)
	if lenA > lenB {
		return false
	}
	i := lenA - 1
	offset := lenA - lenB
	for ; i >= 0; i-- {
		ar, br := a[i], b[i-(offset)]
		if ar == br {
			continue
		}
		if br < ar {
			ar, br = br, ar
		}
		if 'A' <= ar && ar <= 'Z' && br == ar+'a'-'A' {
			continue
		}
		return false
	}

	if a[0] != '.' && lenB > lenA && b[lenB-lenA-1] != '.' {
		return false
	}

	return true
}

// dnsCompareTable contains the ASCII alphabet mapped from a characters index in
// the table to its lowercased form.
var dnsCompareTable [256]byte

func init() {
	// NOTE: we don't actually need the
	// full alphabet, but calculating offsets would be more expensive than just
	// having redundant characters.
	for i := 0; i < 256; i++ {
		c := byte(i)
		if 'A' <= c && c <= 'Z' {
			// Lowercase uppercase characters A-Z.
			c += 'a' - 'A'
		}
		dnsCompareTable[i] = c
	}
	// Set the period character to 0 so that we get the right sorting behavior.
	//
	// In particular, we need the period character to sort before the only
	// other valid DNS name character which isn't a-z or 0-9, the hyphen,
	// otherwise a name with a dash would be incorrectly sorted into the middle
	// of another tree.
	//
	// For example, imagine a certificate with the constraints "a.com", "a.a.com", and
	// "a-a.com". These would sort as "a.com", "a-a.com", "a.a.com", which would break
	// the pruning step since we wouldn't see that "a.a.com" is a subset of "a.com".
	// Sorting the period before the hyphen ensures that "a.a.com" sorts before "a-a.com".
	dnsCompareTable['.'] = 0
}

// dnsCompare is a case-insensitive reversed implementation of strings.Compare
// that operates from the end to the start of the strings. This is more
// efficient that allocating reversed version of a and b and using
// strings.Compare directly (even though it is highly optimized).
//
// NOTE: this function treats the period character ('.') as sorting above every
// other character, which is necessary for us to properly sort names into their
// correct order. This is further discussed in the init function above.
func dnsCompare(a, b string) int {
	idxA := len(a) - 1
	idxB := len(b) - 1

	for idxA >= 0 && idxB >= 0 {
		byteA := dnsCompareTable[a[idxA]]
		byteB := dnsCompareTable[b[idxB]]
		if byteA == byteB {
			idxA--
			idxB--
			continue
		}
		ret := 1
		if byteA < byteB {
			ret = -1
		}
		return ret
	}

	ret := 0
	if idxA < idxB {
		ret = -1
	} else if idxB < idxA {
		ret = 1
	}
	return ret
}

type dnsConstraints struct {
	// all lets us short circuit the query logic if we see a zero length
	// constraint which permits or excludes everything.
	all bool

	// permitted indicates if these constraints are for permitted or excluded
	// names.
	permitted bool

	constraints *nameConstraintsSet[string, string]

	// parentConstraints contains a subset of constraints which are used for
	// wildcard SAN queries, which are constructed by removing the first label
	// from the constraints in constraints. parentConstraints is only populated
	// if permitted is false.
	parentConstraints map[string]string
}

func newDNSConstraints(l []string, permitted bool) interface{ query(string) (string, bool) } {
	if len(l) == 0 {
		return nil
	}
	for _, n := range l {
		if len(n) == 0 {
			return &dnsConstraints{all: true}
		}
	}
	constraints := slices.Clone(l)

	nc := &dnsConstraints{
		constraints: &nameConstraintsSet[string, string]{
			set: constraints,
		},
		permitted: permitted,
	}

	nc.constraints.sortAndPrune(dnsCompare, dnsHasSuffix)

	if !permitted {
		parentConstraints := map[string]string{}
		for _, name := range nc.constraints.set {
			name = strings.ToLower(name)
			trimmedName := trimFirstLabel(name)
			if trimmedName == "" {
				continue
			}
			parentConstraints[trimmedName] = name
		}
		if len(parentConstraints) > 0 {
			nc.parentConstraints = parentConstraints
		}
	}

	return nc
}

func (dnc *dnsConstraints) query(s string) (string, bool) {
	if dnc.all {
		return "", true
	}

	constraint, match := dnc.constraints.search(s, dnsCompare, dnsHasSuffix)
	if match {
		return constraint, true
	}

	if !dnc.permitted && len(s) > 0 && s[0] == '*' {
		s = strings.ToLower(s)
		trimmed := trimFirstLabel(s)
		if constraint, found := dnc.parentConstraints[trimmed]; found {
			return constraint, true
		}
	}
	return "", false
}

type emailConstraints struct {
	dnsConstraints interface{ query(string) (string, bool) }

	// fullEmails is map of rfc2821Mailboxs that are fully specified in the
	// constraints, which we need to check for separately since they don't
	// follow the same matching rules as the domain-based constraints. The
	// domain portion of the rfc2821Mailbox has been lowercased, since the
	// domain portion is case insensitive. When checking the map for an email,
	// the domain portion of the query should also be lowercased.
	fullEmails map[rfc2821Mailbox]struct{}
}

func newEmailConstraints(l []string, permitted bool) interface {
	query(rfc2821Mailbox) (string, bool)
} {
	if len(l) == 0 {
		return nil
	}
	exactMap := map[rfc2821Mailbox]struct{}{}
	var domains []string
	for _, c := range l {
		if !strings.ContainsRune(c, '@') {
			domains = append(domains, c)
			continue
		}
		parsed, ok := parseRFC2821Mailbox(c)
		if !ok {
			// We've already parsed these addresses in parseCertificate, and
			// treat failures as a hard failure for parsing. The only way we can
			// get a parse failure here is if the caller has mutated the
			// certificate since parsing.
			continue
		}
		parsed.domain = strings.ToLower(parsed.domain)
		exactMap[parsed] = struct{}{}
	}
	ec := &emailConstraints{
		fullEmails: exactMap,
	}
	if len(domains) > 0 {
		ec.dnsConstraints = newDNSConstraints(domains, permitted)
	}
	return ec
}

func (ec *emailConstraints) query(s rfc2821Mailbox) (string, bool) {
	if len(ec.fullEmails) > 0 {
		if _, ok := ec.fullEmails[s]; ok {
			return fmt.Sprintf("%s@%s", s.local, s.domain), true
		}
	}
	if ec.dnsConstraints == nil {
		return "", false
	}
	constraint, found := ec.dnsConstraints.query(s.domain)
	return constraint, found
}

type constraints[T any, V any] struct {
	constraintType string
	permitted      interface{ query(V) (T, bool) }
	excluded       interface{ query(V) (T, bool) }
}

func checkConstraints[T string | *net.IPNet, V any, P string | net.IP | parsedURI | rfc2821Mailbox](c constraints[T, V], s V, p P) error {
	if c.permitted != nil {
		if _, found := c.permitted.query(s); !found {
			return fmt.Errorf("%s %q is not permitted by any constraint", c.constraintType, p)
		}
	}
	if c.excluded != nil {
		if constraint, found := c.excluded.query(s); found {
			return fmt.Errorf("%s %q is excluded by constraint %q", c.constraintType, p, constraint)
		}
	}
	return nil
}

type chainConstraints struct {
	ip    constraints[*net.IPNet, net.IP]
	dns   constraints[string, string]
	uri   constraints[string, string]
	email constraints[string, rfc2821Mailbox]

	index int
	next  *chainConstraints
}

func (cc *chainConstraints) check(dns []string, uris []parsedURI, emails []rfc2821Mailbox, ips []net.IP) error {
	for _, ip := range ips {
		if err := checkConstraints(cc.ip, ip, ip); err != nil {
			return err
		}
	}
	for _, d := range dns {
		if !domainNameValid(d, false) {
			return fmt.Errorf("x509: cannot parse dnsName %q", d)
		}
		if err := checkConstraints(cc.dns, d, d); err != nil {
			return err
		}
	}
	for _, u := range uris {
		if !domainNameValid(u.domain, false) {
			return fmt.Errorf("x509: internal error: URI SAN %q failed to parse", u)
		}
		if err := checkConstraints(cc.uri, u.domain, u); err != nil {
			return err
		}
	}
	for _, e := range emails {
		if !domainNameValid(e.domain, false) {
			return fmt.Errorf("x509: cannot parse rfc822Name %q", e)
		}
		if err := checkConstraints(cc.email, e, e); err != nil {
			return err
		}
	}
	return nil
}

func checkChainConstraints(chain []*Certificate) error {
	var currentConstraints *chainConstraints
	var last *chainConstraints
	for i, c := range chain {
		if !c.hasNameConstraints() {
			continue
		}
		cc := &chainConstraints{
			ip:    constraints[*net.IPNet, net.IP]{"IP address", newIPNetConstraints(c.PermittedIPRanges), newIPNetConstraints(c.ExcludedIPRanges)},
			dns:   constraints[string, string]{"DNS name", newDNSConstraints(c.PermittedDNSDomains, true), newDNSConstraints(c.ExcludedDNSDomains, false)},
			uri:   constraints[string, string]{"URI", newDNSConstraints(c.PermittedURIDomains, true), newDNSConstraints(c.ExcludedURIDomains, false)},
			email: constraints[string, rfc2821Mailbox]{"email address", newEmailConstraints(c.PermittedEmailAddresses, true), newEmailConstraints(c.ExcludedEmailAddresses, false)},
			index: i,
		}
		if currentConstraints == nil {
			currentConstraints = cc
			last = cc
		} else if last != nil {
			last.next = cc
			last = cc
		}
	}
	if currentConstraints == nil {
		return nil
	}

	for i, c := range chain {
		if !c.hasSANExtension() {
			continue
		}
		if i >= currentConstraints.index {
			for currentConstraints.index <= i {
				if currentConstraints.next == nil {
					return nil
				}
				currentConstraints = currentConstraints.next
			}
		}

		uris, err := parseURIs(c.URIs)
		if err != nil {
			return err
		}
		emails, err := parseMailboxes(c.EmailAddresses)
		if err != nil {
			return err
		}

		for n := currentConstraints; n != nil; n = n.next {
			if err := n.check(c.DNSNames, uris, emails, c.IPAddresses); err != nil {
				return err
			}
		}
	}

	return nil
}

type parsedURI struct {
	uri    *url.URL
	domain string
}

func (u parsedURI) String() string {
	return u.uri.String()
}

func parseURIs(uris []*url.URL) ([]parsedURI, error) {
	parsed := make([]parsedURI, 0, len(uris))
	for _, uri := range uris {
		host := strings.ToLower(uri.Host)
		if len(host) == 0 {
			return nil, fmt.Errorf("URI with empty host (%q) cannot be matched against constraints", uri.String())
		}
		if strings.Contains(host, ":") && !strings.HasSuffix(host, "]") {
			var err error
			host, _, err = net.SplitHostPort(uri.Host)
			if err != nil {
				return nil, fmt.Errorf("cannot parse URI host %q: %v", uri.Host, err)
			}
		}

		// netip.ParseAddr will reject the URI IPv6 literal form "[...]", so we
		// check if _either_ the string parses as an IP, or if it is enclosed in
		// square brackets.
		if _, err := netip.ParseAddr(host); err == nil || (strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]")) {
			return nil, fmt.Errorf("URI with IP (%q) cannot be matched against constraints", uri.String())
		}

		parsed = append(parsed, parsedURI{uri, host})
	}
	return parsed, nil
}

func parseMailboxes(emails []string) ([]rfc2821Mailbox, error) {
	parsed := make([]rfc2821Mailbox, 0, len(emails))
	for _, email := range emails {
		mailbox, ok := parseRFC2821Mailbox(email)
		if !ok {
			return nil, fmt.Errorf("cannot parse rfc822Name %q", email)
		}
		mailbox.domain = strings.ToLower(mailbox.domain)
		parsed = append(parsed, mailbox)
	}
	return parsed, nil
}

func trimFirstLabel(dnsName string) string {
	firstDotInd := strings.IndexByte(dnsName, '.')
	if firstDotInd < 0 {
		// Constraint is a single label, we cannot trim it.
		return ""
	}
	return dnsName[firstDotInd:]
}

```

// === FILE: references/go/src/crypto/x509/internal/macos/corefoundation.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin

// Package macos provides cgo-less wrappers for Core Foundation and
// Security.framework, similarly to how package syscall provides access to
// libSystem.dylib.
package macos

import (
	"bytes"
	"errors"
	"internal/abi"
	"runtime"
	"time"
	"unsafe"
)

// Core Foundation linker flags for the external linker. See Issue 42459.
//
//go:cgo_ldflag "-framework"
//go:cgo_ldflag "CoreFoundation"

// CFRef is an opaque reference to a Core Foundation object. It is a pointer,
// but to memory not owned by Go, so not an unsafe.Pointer.
type CFRef uintptr

// CFDataToSlice returns a copy of the contents of data as a bytes slice.
func CFDataToSlice(data CFRef) []byte {
	length := CFDataGetLength(data)
	ptr := CFDataGetBytePtr(data)
	src := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), length)
	return bytes.Clone(src)
}

// CFStringToString returns a Go string representation of the passed
// in CFString, or an empty string if it's invalid.
func CFStringToString(ref CFRef) string {
	data, err := CFStringCreateExternalRepresentation(ref)
	if err != nil {
		return ""
	}
	b := CFDataToSlice(data)
	CFRelease(data)
	return string(b)
}

// TimeToCFDateRef converts a time.Time into an apple CFDateRef.
func TimeToCFDateRef(t time.Time) CFRef {
	secs := t.Sub(time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)).Seconds()
	ref := CFDateCreate(secs)
	return ref
}

type CFString CFRef

const kCFAllocatorDefault = 0
const kCFStringEncodingUTF8 = 0x08000100

//go:cgo_import_dynamic x509_CFDataCreate CFDataCreate "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func BytesToCFData(b []byte) CFRef {
	p := unsafe.Pointer(unsafe.SliceData(b))
	ret := syscall(abi.FuncPCABI0(x509_CFDataCreate_trampoline), kCFAllocatorDefault, uintptr(p), uintptr(len(b)), 0, 0, 0)
	runtime.KeepAlive(p)
	return CFRef(ret)
}
func x509_CFDataCreate_trampoline()

//go:cgo_import_dynamic x509_CFStringCreateWithBytes CFStringCreateWithBytes "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

// StringToCFString returns a copy of the UTF-8 contents of s as a new CFString.
func StringToCFString(s string) CFString {
	p := unsafe.Pointer(unsafe.StringData(s))
	ret := syscall(abi.FuncPCABI0(x509_CFStringCreateWithBytes_trampoline), kCFAllocatorDefault, uintptr(p),
		uintptr(len(s)), uintptr(kCFStringEncodingUTF8), 0 /* isExternalRepresentation */, 0)
	runtime.KeepAlive(p)
	return CFString(ret)
}
func x509_CFStringCreateWithBytes_trampoline()

//go:cgo_import_dynamic x509_CFDictionaryGetValueIfPresent CFDictionaryGetValueIfPresent "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFDictionaryGetValueIfPresent(dict CFRef, key CFString) (value CFRef, ok bool) {
	ret := syscall(abi.FuncPCABI0(x509_CFDictionaryGetValueIfPresent_trampoline), uintptr(dict), uintptr(key),
		uintptr(unsafe.Pointer(&value)), 0, 0, 0)
	if ret == 0 {
		return 0, false
	}
	return value, true
}
func x509_CFDictionaryGetValueIfPresent_trampoline()

const kCFNumberSInt32Type = 3

//go:cgo_import_dynamic x509_CFNumberGetValue CFNumberGetValue "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFNumberGetValue(num CFRef) (int32, error) {
	var value int32
	ret := syscall(abi.FuncPCABI0(x509_CFNumberGetValue_trampoline), uintptr(num), uintptr(kCFNumberSInt32Type),
		uintptr(unsafe.Pointer(&value)), 0, 0, 0)
	if ret == 0 {
		return 0, errors.New("CFNumberGetValue call failed")
	}
	return value, nil
}
func x509_CFNumberGetValue_trampoline()

//go:cgo_import_dynamic x509_CFDataGetLength CFDataGetLength "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFDataGetLength(data CFRef) int {
	ret := syscall(abi.FuncPCABI0(x509_CFDataGetLength_trampoline), uintptr(data), 0, 0, 0, 0, 0)
	return int(ret)
}
func x509_CFDataGetLength_trampoline()

//go:cgo_import_dynamic x509_CFDataGetBytePtr CFDataGetBytePtr "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFDataGetBytePtr(data CFRef) uintptr {
	ret := syscall(abi.FuncPCABI0(x509_CFDataGetBytePtr_trampoline), uintptr(data), 0, 0, 0, 0, 0)
	return ret
}
func x509_CFDataGetBytePtr_trampoline()

//go:cgo_import_dynamic x509_CFArrayGetCount CFArrayGetCount "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFArrayGetCount(array CFRef) int {
	ret := syscall(abi.FuncPCABI0(x509_CFArrayGetCount_trampoline), uintptr(array), 0, 0, 0, 0, 0)
	return int(ret)
}
func x509_CFArrayGetCount_trampoline()

//go:cgo_import_dynamic x509_CFArrayGetValueAtIndex CFArrayGetValueAtIndex "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFArrayGetValueAtIndex(array CFRef, index int) CFRef {
	ret := syscall(abi.FuncPCABI0(x509_CFArrayGetValueAtIndex_trampoline), uintptr(array), uintptr(index), 0, 0, 0, 0)
	return CFRef(ret)
}
func x509_CFArrayGetValueAtIndex_trampoline()

//go:cgo_import_dynamic x509_CFEqual CFEqual "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFEqual(a, b CFRef) bool {
	ret := syscall(abi.FuncPCABI0(x509_CFEqual_trampoline), uintptr(a), uintptr(b), 0, 0, 0, 0)
	return ret == 1
}
func x509_CFEqual_trampoline()

//go:cgo_import_dynamic x509_CFRelease CFRelease "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFRelease(ref CFRef) {
	syscall(abi.FuncPCABI0(x509_CFRelease_trampoline), uintptr(ref), 0, 0, 0, 0, 0)
}
func x509_CFRelease_trampoline()

//go:cgo_import_dynamic x509_CFArrayCreateMutable CFArrayCreateMutable "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFArrayCreateMutable() CFRef {
	ret := syscall(abi.FuncPCABI0(x509_CFArrayCreateMutable_trampoline), kCFAllocatorDefault, 0, 0 /* kCFTypeArrayCallBacks */, 0, 0, 0)
	return CFRef(ret)
}
func x509_CFArrayCreateMutable_trampoline()

//go:cgo_import_dynamic x509_CFArrayAppendValue CFArrayAppendValue "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFArrayAppendValue(array CFRef, val CFRef) {
	syscall(abi.FuncPCABI0(x509_CFArrayAppendValue_trampoline), uintptr(array), uintptr(val), 0, 0, 0, 0)
}
func x509_CFArrayAppendValue_trampoline()

//go:cgo_import_dynamic x509_CFDateCreate CFDateCreate "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFDateCreate(seconds float64) CFRef {
	ret := syscall(abi.FuncPCABI0(x509_CFDateCreate_trampoline), kCFAllocatorDefault, 0, 0, 0, 0, seconds)
	return CFRef(ret)
}
func x509_CFDateCreate_trampoline()

//go:cgo_import_dynamic x509_CFErrorCopyDescription CFErrorCopyDescription "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFErrorCopyDescription(errRef CFRef) CFRef {
	ret := syscall(abi.FuncPCABI0(x509_CFErrorCopyDescription_trampoline), uintptr(errRef), 0, 0, 0, 0, 0)
	return CFRef(ret)
}
func x509_CFErrorCopyDescription_trampoline()

//go:cgo_import_dynamic x509_CFErrorGetCode CFErrorGetCode "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFErrorGetCode(errRef CFRef) int {
	return int(syscall(abi.FuncPCABI0(x509_CFErrorGetCode_trampoline), uintptr(errRef), 0, 0, 0, 0, 0))
}
func x509_CFErrorGetCode_trampoline()

//go:cgo_import_dynamic x509_CFStringCreateExternalRepresentation CFStringCreateExternalRepresentation "/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation"

func CFStringCreateExternalRepresentation(strRef CFRef) (CFRef, error) {
	ret := syscall(abi.FuncPCABI0(x509_CFStringCreateExternalRepresentation_trampoline), kCFAllocatorDefault, uintptr(strRef), kCFStringEncodingUTF8, 0, 0, 0)
	if ret == 0 {
		return 0, errors.New("string can't be represented as UTF-8")
	}
	return CFRef(ret), nil
}
func x509_CFStringCreateExternalRepresentation_trampoline()

// syscall is implemented in the runtime package (runtime/sys_darwin.go)
func syscall(fn, a1, a2, a3, a4, a5 uintptr, f1 float64) uintptr

// ReleaseCFArray iterates through an array, releasing its contents, and then
// releases the array itself. This is necessary because we cannot, easily, set the
// CFArrayCallBacks argument when creating CFArrays.
func ReleaseCFArray(array CFRef) {
	for i := 0; i < CFArrayGetCount(array); i++ {
		ref := CFArrayGetValueAtIndex(array, i)
		CFRelease(ref)
	}
	CFRelease(array)
}

```

// === FILE: references/go/src/crypto/x509/internal/macos/corefoundation.s ===
```text
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin

#include "textflag.h"

// The trampolines are ABIInternal as they are address-taken in
// Go code.

TEXT ·x509_CFArrayGetCount_trampoline(SB),NOSPLIT,$0-0
	JMP	x509_CFArrayGetCount(SB)
TEXT ·x509_CFArrayGetValueAtIndex_trampoline(SB),NOSPLIT,$0-0
	JMP	x509_CFArrayGetValueAtIndex(SB)
TEXT ·x509_CFDataGetBytePtr_trampoline(SB),NOSPLIT,$0-0
	JMP	x509_CFDataGetBytePtr(SB)
TEXT ·x509_CFDataGetLength_trampoline(SB),NOSPLIT,$0-0
	JMP	x509_CFDataGetLength(SB)
TEXT ·x509_CFStringCreateWithBytes_trampoline(SB),NOSPLIT,$0-0
	JMP	x509_CFStringCreateWithBytes(SB)
TEXT ·x509_CFRelease_trampoline(SB),NOSPLIT,$0-0
	JMP	x509_CFRelease(SB)
TEXT ·x509_CFDictionaryGetValueIfPresent_trampoline(SB),NOSPLIT,$0-0
	JMP	x509_CFDictionaryGetValueIfPresent(SB)
TEXT ·x509_CFNumberGetValue_trampoline(SB),NOSPLIT,$0-0
	JMP	x509_CFNumberGetValue(SB)
TEXT ·x509_CFEqual_trampoline(SB),NOSPLIT,$0-0
	JMP	x509_CFEqual(SB)
TEXT ·x509_CFArrayCreateMutable_trampoline(SB),NOSPLIT,$0-0
	JMP x509_CFArrayCreateMutable(SB)
TEXT ·x509_CFArrayAppendValue_trampoline(SB),NOSPLIT,$0-0
	JMP x509_CFArrayAppendValue(SB)
TEXT ·x509_CFDateCreate_trampoline(SB),NOSPLIT,$0-0
	JMP x509_CFDateCreate(SB)
TEXT ·x509_CFDataCreate_trampoline(SB),NOSPLIT,$0-0
	JMP x509_CFDataCreate(SB)
TEXT ·x509_CFErrorCopyDescription_trampoline(SB),NOSPLIT,$0-0
	JMP x509_CFErrorCopyDescription(SB)
TEXT ·x509_CFErrorGetCode_trampoline(SB),NOSPLIT,$0-0
	JMP x509_CFErrorGetCode(SB)
TEXT ·x509_CFStringCreateExternalRepresentation_trampoline(SB),NOSPLIT,$0-0
	JMP x509_CFStringCreateExternalRepresentation(SB)

```

// === FILE: references/go/src/crypto/x509/internal/macos/security.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin

package macos

import (
	"errors"
	"internal/abi"
	"strconv"
	"unsafe"
)

// Security.framework linker flags for the external linker. See Issue 42459.
//
//go:cgo_ldflag "-framework"
//go:cgo_ldflag "Security"

// Based on https://opensource.apple.com/source/Security/Security-59306.41.2/base/Security.h

const (
	// various macOS error codes that can be returned from
	// SecTrustEvaluateWithError that we can map to Go cert
	// verification error types.
	ErrSecCertificateExpired = -67818
	ErrSecHostNameMismatch   = -67602
	ErrSecNotTrusted         = -67843
)

type OSStatus struct {
	call   string
	status int32
}

func (s OSStatus) Error() string {
	return s.call + " error: " + strconv.Itoa(int(s.status))
}

//go:cgo_import_dynamic x509_SecTrustCreateWithCertificates SecTrustCreateWithCertificates "/System/Library/Frameworks/Security.framework/Versions/A/Security"

func SecTrustCreateWithCertificates(certs CFRef, policies CFRef) (CFRef, error) {
	var trustObj CFRef
	ret := syscall(abi.FuncPCABI0(x509_SecTrustCreateWithCertificates_trampoline), uintptr(certs), uintptr(policies),
		uintptr(unsafe.Pointer(&trustObj)), 0, 0, 0)
	if int32(ret) != 0 {
		return 0, OSStatus{"SecTrustCreateWithCertificates", int32(ret)}
	}
	return trustObj, nil
}
func x509_SecTrustCreateWithCertificates_trampoline()

//go:cgo_import_dynamic x509_SecCertificateCreateWithData SecCertificateCreateWithData "/System/Library/Frameworks/Security.framework/Versions/A/Security"

func SecCertificateCreateWithData(b []byte) (CFRef, error) {
	data := BytesToCFData(b)
	defer CFRelease(data)
	ret := syscall(abi.FuncPCABI0(x509_SecCertificateCreateWithData_trampoline), kCFAllocatorDefault, uintptr(data), 0, 0, 0, 0)
	// Returns NULL if the data passed in the data parameter is not a valid
	// DER-encoded X.509 certificate.
	if ret == 0 {
		return 0, errors.New("SecCertificateCreateWithData: invalid certificate")
	}
	return CFRef(ret), nil
}
func x509_SecCertificateCreateWithData_trampoline()

//go:cgo_import_dynamic x509_SecPolicyCreateSSL SecPolicyCreateSSL "/System/Library/Frameworks/Security.framework/Versions/A/Security"

func SecPolicyCreateSSL(name string) (CFRef, error) {
	var hostname CFString
	if name != "" {
		hostname = StringToCFString(name)
		defer CFRelease(CFRef(hostname))
	}
	ret := syscall(abi.FuncPCABI0(x509_SecPolicyCreateSSL_trampoline), 1 /* true */, uintptr(hostname), 0, 0, 0, 0)
	if ret == 0 {
		return 0, OSStatus{"SecPolicyCreateSSL", int32(ret)}
	}
	return CFRef(ret), nil
}
func x509_SecPolicyCreateSSL_trampoline()

//go:cgo_import_dynamic x509_SecTrustSetVerifyDate SecTrustSetVerifyDate "/System/Library/Frameworks/Security.framework/Versions/A/Security"

func SecTrustSetVerifyDate(trustObj CFRef, dateRef CFRef) error {
	ret := syscall(abi.FuncPCABI0(x509_SecTrustSetVerifyDate_trampoline), uintptr(trustObj), uintptr(dateRef), 0, 0, 0, 0)
	if int32(ret) != 0 {
		return OSStatus{"SecTrustSetVerifyDate", int32(ret)}
	}
	return nil
}
func x509_SecTrustSetVerifyDate_trampoline()

//go:cgo_import_dynamic x509_SecTrustEvaluate SecTrustEvaluate "/System/Library/Frameworks/Security.framework/Versions/A/Security"

func SecTrustEvaluate(trustObj CFRef) (CFRef, error) {
	var result CFRef
	ret := syscall(abi.FuncPCABI0(x509_SecTrustEvaluate_trampoline), uintptr(trustObj), uintptr(unsafe.Pointer(&result)), 0, 0, 0, 0)
	if int32(ret) != 0 {
		return 0, OSStatus{"SecTrustEvaluate", int32(ret)}
	}
	return CFRef(result), nil
}
func x509_SecTrustEvaluate_trampoline()

//go:cgo_import_dynamic x509_SecTrustEvaluateWithError SecTrustEvaluateWithError "/System/Library/Frameworks/Security.framework/Versions/A/Security"

func SecTrustEvaluateWithError(trustObj CFRef) (int, error) {
	var errRef CFRef
	ret := syscall(abi.FuncPCABI0(x509_SecTrustEvaluateWithError_trampoline), uintptr(trustObj), uintptr(unsafe.Pointer(&errRef)), 0, 0, 0, 0)
	if int32(ret) != 1 {
		errStr := CFErrorCopyDescription(errRef)
		err := errors.New(CFStringToString(errStr))
		errCode := CFErrorGetCode(errRef)
		CFRelease(errRef)
		CFRelease(errStr)
		return errCode, err
	}
	return 0, nil
}
func x509_SecTrustEvaluateWithError_trampoline()

//go:cgo_import_dynamic x509_SecCertificateCopyData SecCertificateCopyData "/System/Library/Frameworks/Security.framework/Versions/A/Security"

func SecCertificateCopyData(cert CFRef) ([]byte, error) {
	ret := syscall(abi.FuncPCABI0(x509_SecCertificateCopyData_trampoline), uintptr(cert), 0, 0, 0, 0, 0)
	if ret == 0 {
		return nil, errors.New("x509: invalid certificate object")
	}
	b := CFDataToSlice(CFRef(ret))
	CFRelease(CFRef(ret))
	return b, nil
}
func x509_SecCertificateCopyData_trampoline()

//go:cgo_import_dynamic x509_SecTrustCopyCertificateChain SecTrustCopyCertificateChain "/System/Library/Frameworks/Security.framework/Versions/A/Security"

func SecTrustCopyCertificateChain(trustObj CFRef) (CFRef, error) {
	ret := syscall(abi.FuncPCABI0(x509_SecTrustCopyCertificateChain_trampoline), uintptr(trustObj), 0, 0, 0, 0, 0)
	if ret == 0 {
		return 0, OSStatus{"SecTrustCopyCertificateChain", int32(ret)}
	}
	return CFRef(ret), nil
}
func x509_SecTrustCopyCertificateChain_trampoline()

```

// === FILE: references/go/src/crypto/x509/internal/macos/security.s ===
```text
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin

#include "textflag.h"

// The trampolines are ABIInternal as they are address-taken in
// Go code.

TEXT ·x509_SecTrustCreateWithCertificates_trampoline(SB),NOSPLIT,$0-0
	JMP x509_SecTrustCreateWithCertificates(SB)
TEXT ·x509_SecCertificateCreateWithData_trampoline(SB),NOSPLIT,$0-0
	JMP x509_SecCertificateCreateWithData(SB)
TEXT ·x509_SecPolicyCreateSSL_trampoline(SB),NOSPLIT,$0-0
	JMP x509_SecPolicyCreateSSL(SB)
TEXT ·x509_SecTrustSetVerifyDate_trampoline(SB),NOSPLIT,$0-0
	JMP x509_SecTrustSetVerifyDate(SB)
TEXT ·x509_SecTrustEvaluate_trampoline(SB),NOSPLIT,$0-0
	JMP x509_SecTrustEvaluate(SB)
TEXT ·x509_SecTrustEvaluateWithError_trampoline(SB),NOSPLIT,$0-0
	JMP x509_SecTrustEvaluateWithError(SB)
TEXT ·x509_SecCertificateCopyData_trampoline(SB),NOSPLIT,$0-0
	JMP x509_SecCertificateCopyData(SB)
TEXT ·x509_SecTrustCopyCertificateChain_trampoline(SB),NOSPLIT,$0-0
	JMP x509_SecTrustCopyCertificateChain(SB)

```

// === FILE: references/go/src/crypto/x509/oid.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"bytes"
	"encoding/asn1"
	"errors"
	"math"
	"math/big"
	"math/bits"
	"strconv"
	"strings"
)

var (
	errInvalidOID = errors.New("invalid oid")
)

// An OID represents an ASN.1 OBJECT IDENTIFIER.
type OID struct {
	der []byte
}

// ParseOID parses a Object Identifier string, represented by ASCII numbers separated by dots.
func ParseOID(oid string) (OID, error) {
	var o OID
	return o, o.unmarshalOIDText(oid)
}

func newOIDFromDER(der []byte) (OID, bool) {
	if len(der) == 0 || der[len(der)-1]&0x80 != 0 {
		return OID{}, false
	}

	start := 0
	for i, v := range der {
		// ITU-T X.690, section 8.19.2:
		// The subidentifier shall be encoded in the fewest possible octets,
		// that is, the leading octet of the subidentifier shall not have the value 0x80.
		if i == start && v == 0x80 {
			return OID{}, false
		}
		if v&0x80 == 0 {
			start = i + 1
		}
	}

	return OID{der}, true
}

// OIDFromInts creates a new OID using ints, each integer is a separate component.
func OIDFromInts(oid []uint64) (OID, error) {
	if len(oid) < 2 || oid[0] > 2 || (oid[0] < 2 && oid[1] >= 40) {
		return OID{}, errInvalidOID
	}

	length := base128IntLength(oid[0]*40 + oid[1])
	for _, v := range oid[2:] {
		length += base128IntLength(v)
	}

	der := make([]byte, 0, length)
	der = appendBase128Int(der, oid[0]*40+oid[1])
	for _, v := range oid[2:] {
		der = appendBase128Int(der, v)
	}
	return OID{der}, nil
}

func base128IntLength(n uint64) int {
	if n == 0 {
		return 1
	}
	return (bits.Len64(n) + 6) / 7
}

func appendBase128Int(dst []byte, n uint64) []byte {
	for i := base128IntLength(n) - 1; i >= 0; i-- {
		o := byte(n >> uint(i*7))
		o &= 0x7f
		if i != 0 {
			o |= 0x80
		}
		dst = append(dst, o)
	}
	return dst
}

func base128BigIntLength(n *big.Int) int {
	if n.Cmp(big.NewInt(0)) == 0 {
		return 1
	}
	return (n.BitLen() + 6) / 7
}

func appendBase128BigInt(dst []byte, n *big.Int) []byte {
	if n.Cmp(big.NewInt(0)) == 0 {
		return append(dst, 0)
	}

	for i := base128BigIntLength(n) - 1; i >= 0; i-- {
		o := byte(big.NewInt(0).Rsh(n, uint(i)*7).Bits()[0])
		o &= 0x7f
		if i != 0 {
			o |= 0x80
		}
		dst = append(dst, o)
	}
	return dst
}

// AppendText implements [encoding.TextAppender]
func (o OID) AppendText(b []byte) ([]byte, error) {
	return append(b, o.String()...), nil
}

// MarshalText implements [encoding.TextMarshaler]
func (o OID) MarshalText() ([]byte, error) {
	return o.AppendText(nil)
}

// UnmarshalText implements [encoding.TextUnmarshaler]
func (o *OID) UnmarshalText(text []byte) error {
	return o.unmarshalOIDText(string(text))
}

func (o *OID) unmarshalOIDText(oid string) error {
	// (*big.Int).SetString allows +/- signs, but we don't want
	// to allow them in the string representation of Object Identifier, so
	// reject such encodings.
	for _, c := range oid {
		isDigit := c >= '0' && c <= '9'
		if !isDigit && c != '.' {
			return errInvalidOID
		}
	}

	var (
		firstNum  string
		secondNum string
	)

	var nextComponentExists bool
	firstNum, oid, nextComponentExists = strings.Cut(oid, ".")
	if !nextComponentExists {
		return errInvalidOID
	}
	secondNum, oid, nextComponentExists = strings.Cut(oid, ".")

	var (
		first  = big.NewInt(0)
		second = big.NewInt(0)
	)

	if _, ok := first.SetString(firstNum, 10); !ok {
		return errInvalidOID
	}
	if _, ok := second.SetString(secondNum, 10); !ok {
		return errInvalidOID
	}

	if first.Cmp(big.NewInt(2)) > 0 || (first.Cmp(big.NewInt(2)) < 0 && second.Cmp(big.NewInt(40)) >= 0) {
		return errInvalidOID
	}

	firstComponent := first.Mul(first, big.NewInt(40))
	firstComponent.Add(firstComponent, second)

	der := appendBase128BigInt(make([]byte, 0, 32), firstComponent)

	for nextComponentExists {
		var strNum string
		strNum, oid, nextComponentExists = strings.Cut(oid, ".")
		b, ok := big.NewInt(0).SetString(strNum, 10)
		if !ok {
			return errInvalidOID
		}
		der = appendBase128BigInt(der, b)
	}

	o.der = der
	return nil
}

// AppendBinary implements [encoding.BinaryAppender]
func (o OID) AppendBinary(b []byte) ([]byte, error) {
	return append(b, o.der...), nil
}

// MarshalBinary implements [encoding.BinaryMarshaler]
func (o OID) MarshalBinary() ([]byte, error) {
	return o.AppendBinary(nil)
}

// UnmarshalBinary implements [encoding.BinaryUnmarshaler]
func (o *OID) UnmarshalBinary(b []byte) error {
	oid, ok := newOIDFromDER(bytes.Clone(b))
	if !ok {
		return errInvalidOID
	}
	*o = oid
	return nil
}

// Equal returns true when oid and other represents the same Object Identifier.
func (oid OID) Equal(other OID) bool {
	// There is only one possible DER encoding of
	// each unique Object Identifier.
	return bytes.Equal(oid.der, other.der)
}

func parseBase128Int(bytes []byte, initOffset int) (ret, offset int, failed bool) {
	offset = initOffset
	var ret64 int64
	for shifted := 0; offset < len(bytes); shifted++ {
		// 5 * 7 bits per byte == 35 bits of data
		// Thus the representation is either non-minimal or too large for an int32
		if shifted == 5 {
			failed = true
			return
		}
		ret64 <<= 7
		b := bytes[offset]
		// integers should be minimally encoded, so the leading octet should
		// never be 0x80
		if shifted == 0 && b == 0x80 {
			failed = true
			return
		}
		ret64 |= int64(b & 0x7f)
		offset++
		if b&0x80 == 0 {
			ret = int(ret64)
			// Ensure that the returned value fits in an int on all platforms
			if ret64 > math.MaxInt32 {
				failed = true
			}
			return
		}
	}
	failed = true
	return
}

// EqualASN1OID returns whether an OID equals an asn1.ObjectIdentifier. If
// asn1.ObjectIdentifier cannot represent the OID specified by oid, because
// a component of OID requires more than 31 bits, it returns false.
func (oid OID) EqualASN1OID(other asn1.ObjectIdentifier) bool {
	if len(other) < 2 {
		return false
	}
	v, offset, failed := parseBase128Int(oid.der, 0)
	if failed {
		// This should never happen, since we've already parsed the OID,
		// but just in case.
		return false
	}
	if v < 80 {
		a, b := v/40, v%40
		if other[0] != a || other[1] != b {
			return false
		}
	} else {
		a, b := 2, v-80
		if other[0] != a || other[1] != b {
			return false
		}
	}

	i := 2
	for ; offset < len(oid.der); i++ {
		v, offset, failed = parseBase128Int(oid.der, offset)
		if failed {
			// Again, shouldn't happen, since we've already parsed
			// the OID, but better safe than sorry.
			return false
		}
		if i >= len(other) || v != other[i] {
			return false
		}
	}

	return i == len(other)
}

// String returns the string representation of the Object Identifier.
func (oid OID) String() string {
	var b strings.Builder
	b.Grow(32)
	const (
		valSize         = 64 // size in bits of val.
		bitsPerByte     = 7
		maxValSafeShift = (1 << (valSize - bitsPerByte)) - 1
	)
	var (
		start    = 0
		val      = uint64(0)
		numBuf   = make([]byte, 0, 21)
		bigVal   *big.Int
		overflow bool
	)
	for i, v := range oid.der {
		curVal := v & 0x7F
		valEnd := v&0x80 == 0
		if valEnd {
			if start != 0 {
				b.WriteByte('.')
			}
		}
		if !overflow && val > maxValSafeShift {
			if bigVal == nil {
				bigVal = new(big.Int)
			}
			bigVal = bigVal.SetUint64(val)
			overflow = true
		}
		if overflow {
			bigVal = bigVal.Lsh(bigVal, bitsPerByte).Or(bigVal, big.NewInt(int64(curVal)))
			if valEnd {
				if start == 0 {
					b.WriteString("2.")
					bigVal = bigVal.Sub(bigVal, big.NewInt(80))
				}
				numBuf = bigVal.Append(numBuf, 10)
				b.Write(numBuf)
				numBuf = numBuf[:0]
				val = 0
				start = i + 1
				overflow = false
			}
			continue
		}
		val <<= bitsPerByte
		val |= uint64(curVal)
		if valEnd {
			if start == 0 {
				if val < 80 {
					b.Write(strconv.AppendUint(numBuf, val/40, 10))
					b.WriteByte('.')
					b.Write(strconv.AppendUint(numBuf, val%40, 10))
				} else {
					b.WriteString("2.")
					b.Write(strconv.AppendUint(numBuf, val-80, 10))
				}
			} else {
				b.Write(strconv.AppendUint(numBuf, val, 10))
			}
			val = 0
			start = i + 1
		}
	}
	return b.String()
}

func (oid OID) toASN1OID() (asn1.ObjectIdentifier, bool) {
	out := make([]int, 0, len(oid.der)+1)

	const (
		valSize         = 31 // amount of usable bits of val for OIDs.
		bitsPerByte     = 7
		maxValSafeShift = (1 << (valSize - bitsPerByte)) - 1
	)

	val := 0

	for _, v := range oid.der {
		if val > maxValSafeShift {
			return nil, false
		}

		val <<= bitsPerByte
		val |= int(v & 0x7F)

		if v&0x80 == 0 {
			if len(out) == 0 {
				if val < 80 {
					out = append(out, val/40)
					out = append(out, val%40)
				} else {
					out = append(out, 2)
					out = append(out, val-80)
				}
				val = 0
				continue
			}
			out = append(out, val)
			val = 0
		}
	}

	return out, true
}

// OIDFromASN1OID creates a new OID using asn1OID.
func OIDFromASN1OID(asn1OID asn1.ObjectIdentifier) (OID, error) {
	uint64OID := make([]uint64, 0, len(asn1OID))
	for _, component := range asn1OID {
		if component < 0 {
			return OID{}, errors.New("x509: OID components must be non-negative")
		}
		uint64OID = append(uint64OID, uint64(component))
	}
	return OIDFromInts(uint64OID)
}

```

// === FILE: references/go/src/crypto/x509/parser.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"bytes"
	"crypto/dsa"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/mldsa"
	"crypto/rsa"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
	"internal/godebug"
	"math"
	"math/big"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/crypto/cryptobyte"
	cryptobyte_asn1 "golang.org/x/crypto/cryptobyte/asn1"
)

// isPrintable reports whether the given b is in the ASN.1 PrintableString set.
// This is a simplified version of encoding/asn1.isPrintable.
func isPrintable(b byte) bool {
	return 'a' <= b && b <= 'z' ||
		'A' <= b && b <= 'Z' ||
		'0' <= b && b <= '9' ||
		'\'' <= b && b <= ')' ||
		'+' <= b && b <= '/' ||
		b == ' ' ||
		b == ':' ||
		b == '=' ||
		b == '?' ||
		// This is technically not allowed in a PrintableString.
		// However, x509 certificates with wildcard strings don't
		// always use the correct string type so we permit it.
		b == '*' ||
		// This is not technically allowed either. However, not
		// only is it relatively common, but there are also a
		// handful of CA certificates that contain it. At least
		// one of which will not expire until 2027.
		b == '&'
}

// parseASN1String parses the ASN.1 string types T61String, PrintableString,
// UTF8String, BMPString, IA5String, and NumericString. This is mostly copied
// from the respective encoding/asn1.parse... methods, rather than just
// increasing the API surface of that package.
func parseASN1String(tag cryptobyte_asn1.Tag, value []byte) (string, error) {
	switch tag {
	case cryptobyte_asn1.T61String:
		// T.61 is a defunct ITU 8-bit character encoding which preceded Unicode.
		// T.61 uses a code page layout that _almost_ exactly maps to the code
		// page layout of the ISO 8859-1 (Latin-1) character encoding, with the
		// exception that a number of characters in Latin-1 are not present
		// in T.61.
		//
		// Instead of mapping which characters are present in Latin-1 but not T.61,
		// we just treat these strings as being encoded using Latin-1. This matches
		// what most of the world does, including BoringSSL.
		buf := make([]byte, 0, len(value))
		for _, v := range value {
			// All the 1-byte UTF-8 runes map 1-1 with Latin-1.
			buf = utf8.AppendRune(buf, rune(v))
		}
		return string(buf), nil
	case cryptobyte_asn1.PrintableString:
		for _, b := range value {
			if !isPrintable(b) {
				return "", errors.New("invalid PrintableString")
			}
		}
		return string(value), nil
	case cryptobyte_asn1.UTF8String:
		if !utf8.Valid(value) {
			return "", errors.New("invalid UTF-8 string")
		}
		return string(value), nil
	case cryptobyte_asn1.Tag(asn1.TagBMPString):
		// BMPString uses the defunct UCS-2 16-bit character encoding, which
		// covers the Basic Multilingual Plane (BMP). UTF-16 was an extension of
		// UCS-2, containing all of the same code points, but also including
		// multi-code point characters (by using surrogate code points). We can
		// treat a UCS-2 encoded string as a UTF-16 encoded string, as long as
		// we reject out the UTF-16 specific code points. This matches the
		// BoringSSL behavior.

		if len(value)%2 != 0 {
			return "", errors.New("invalid BMPString")
		}

		// Strip terminator if present.
		if l := len(value); l >= 2 && value[l-1] == 0 && value[l-2] == 0 {
			value = value[:l-2]
		}

		s := make([]uint16, 0, len(value)/2)
		for len(value) > 0 {
			point := uint16(value[0])<<8 + uint16(value[1])
			// Reject UTF-16 code points that are permanently reserved
			// noncharacters (0xfffe, 0xffff, and 0xfdd0-0xfdef) and surrogates
			// (0xd800-0xdfff).
			if point == 0xfffe || point == 0xffff ||
				(point >= 0xfdd0 && point <= 0xfdef) ||
				(point >= 0xd800 && point <= 0xdfff) {
				return "", errors.New("invalid BMPString")
			}
			s = append(s, point)
			value = value[2:]
		}

		return string(utf16.Decode(s)), nil
	case cryptobyte_asn1.IA5String:
		s := string(value)
		if isIA5String(s) != nil {
			return "", errors.New("invalid IA5String")
		}
		return s, nil
	case cryptobyte_asn1.Tag(asn1.TagNumericString):
		for _, b := range value {
			if !('0' <= b && b <= '9' || b == ' ') {
				return "", errors.New("invalid NumericString")
			}
		}
		return string(value), nil
	}
	return "", fmt.Errorf("unsupported string type: %v", tag)
}

// readASN1Any parses types documented at [pkix.AttributeTypeAndValue].
func readASN1Any(der *cryptobyte.String) (any, error) {
	var fullValue cryptobyte.String
	var valueTag cryptobyte_asn1.Tag
	if !der.ReadAnyASN1Element(&fullValue, &valueTag) {
		return nil, errors.New("invalid ASN.1 element")
	}
	switch valueTag {
	case cryptobyte_asn1.T61String, cryptobyte_asn1.PrintableString,
		cryptobyte_asn1.UTF8String, cryptobyte_asn1.Tag(asn1.TagBMPString),
		cryptobyte_asn1.IA5String, cryptobyte_asn1.Tag(asn1.TagNumericString):
		var rawValue []byte
		if !fullValue.ReadASN1((*cryptobyte.String)(&rawValue), valueTag) {
			return nil, errors.New("invalid ASN.1 element")
		}
		return parseASN1String(valueTag, rawValue)
	case cryptobyte_asn1.INTEGER:
		var i int64
		if !fullValue.ReadASN1Integer(&i) {
			return nil, errors.New("invalid ASN.1 integer")
		}
		return i, nil
	case cryptobyte_asn1.BIT_STRING:
		var bs asn1.BitString
		if !fullValue.ReadASN1BitString(&bs) {
			return nil, errors.New("invalid ASN.1 BIT STRING")
		}
		return bs, nil
	case cryptobyte_asn1.OCTET_STRING:
		var s []byte
		if !fullValue.ReadASN1((*cryptobyte.String)(&s), cryptobyte_asn1.OCTET_STRING) {
			return nil, errors.New("invalid ASN.1 OCTET STRING")
		}
		return s, nil
	case cryptobyte_asn1.OBJECT_IDENTIFIER:
		var oid asn1.ObjectIdentifier
		if !fullValue.ReadASN1ObjectIdentifier(&oid) {
			return nil, errors.New("invalid ASN.1 OBJECT IDENTIFIER")
		}
		return oid, nil
	case cryptobyte_asn1.UTCTime, cryptobyte_asn1.GeneralizedTime:
		out, err := readASN1Time(&fullValue)
		return out, err
	case cryptobyte_asn1.BOOLEAN:
		var b bool
		if !fullValue.ReadASN1Boolean(&b) {
			return nil, errors.New("invalid ASN.1 BOOLEAN")
		}
		return b, nil
	case cryptobyte_asn1.NULL:
		return nil, nil
	default:
		var v asn1.RawValue
		v.Class = int(valueTag >> 6)
		v.IsCompound = valueTag&0x20 == 0x20
		v.Tag = int(valueTag & 0x1f)
		v.FullBytes = fullValue
		if !fullValue.ReadAnyASN1((*cryptobyte.String)(&v.Bytes), &valueTag) {
			return nil, errors.New("invalid ASN.1 element")
		}
		return v, nil
	}
}

// parseName parses a DER encoded Name as defined in RFC 5280. We may
// want to export this function in the future for use in crypto/tls.
func parseName(raw cryptobyte.String) (*pkix.RDNSequence, error) {
	if !raw.ReadASN1(&raw, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: invalid RDNSequence")
	}

	var rdnSeq pkix.RDNSequence
	for !raw.Empty() {
		var rdnSet pkix.RelativeDistinguishedNameSET
		var set cryptobyte.String
		if !raw.ReadASN1(&set, cryptobyte_asn1.SET) {
			return nil, errors.New("x509: invalid RDNSequence")
		}
		for !set.Empty() {
			var atav cryptobyte.String
			if !set.ReadASN1(&atav, cryptobyte_asn1.SEQUENCE) {
				return nil, errors.New("x509: invalid RDNSequence: invalid attribute")
			}
			var attr pkix.AttributeTypeAndValue
			if !atav.ReadASN1ObjectIdentifier(&attr.Type) {
				return nil, errors.New("x509: invalid RDNSequence: invalid attribute type")
			}
			var err error
			attr.Value, err = readASN1Any(&atav)
			if err != nil {
				return nil, fmt.Errorf("x509: invalid RDNSequence: invalid attribute value: %s", err)
			}
			rdnSet = append(rdnSet, attr)
		}

		rdnSeq = append(rdnSeq, rdnSet)
	}

	return &rdnSeq, nil
}

func parseAI(der cryptobyte.String) (pkix.AlgorithmIdentifier, error) {
	ai := pkix.AlgorithmIdentifier{}
	if !der.ReadASN1ObjectIdentifier(&ai.Algorithm) {
		return ai, errors.New("x509: malformed OID")
	}
	if der.Empty() {
		return ai, nil
	}
	var params cryptobyte.String
	var tag cryptobyte_asn1.Tag
	if !der.ReadAnyASN1Element(&params, &tag) {
		return ai, errors.New("x509: malformed parameters")
	}
	ai.Parameters.Tag = int(tag)
	ai.Parameters.FullBytes = params
	return ai, nil
}

func readASN1Time(der *cryptobyte.String) (time.Time, error) {
	var t time.Time
	switch {
	case der.PeekASN1Tag(cryptobyte_asn1.UTCTime):
		if !der.ReadASN1UTCTime(&t) {
			return t, errors.New("x509: malformed UTCTime")
		}
	case der.PeekASN1Tag(cryptobyte_asn1.GeneralizedTime):
		if !der.ReadASN1GeneralizedTime(&t) {
			return t, errors.New("x509: malformed GeneralizedTime")
		}
	default:
		return t, errors.New("x509: unsupported time format")
	}
	return t, nil
}

func parseValidity(der cryptobyte.String) (time.Time, time.Time, error) {
	notBefore, err := readASN1Time(&der)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	notAfter, err := readASN1Time(&der)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	return notBefore, notAfter, nil
}

func parseExtension(der cryptobyte.String) (pkix.Extension, error) {
	var ext pkix.Extension
	if !der.ReadASN1ObjectIdentifier(&ext.Id) {
		return ext, errors.New("x509: malformed extension OID field")
	}
	if der.PeekASN1Tag(cryptobyte_asn1.BOOLEAN) {
		if !der.ReadASN1Boolean(&ext.Critical) {
			return ext, errors.New("x509: malformed extension critical field")
		}
	}
	var val cryptobyte.String
	if !der.ReadASN1(&val, cryptobyte_asn1.OCTET_STRING) {
		return ext, errors.New("x509: malformed extension value field")
	}
	ext.Value = val
	return ext, nil
}

func parsePublicKey(keyData *publicKeyInfo) (any, error) {
	oid := keyData.Algorithm.Algorithm
	params := keyData.Algorithm.Parameters
	data := keyData.PublicKey.RightAlign()
	switch {
	case oid.Equal(oidPublicKeyRSA):
		// RSA public keys must have a NULL in the parameters.
		// See RFC 3279, Section 2.3.1.
		if !bytes.Equal(params.FullBytes, asn1.NullBytes) {
			return nil, errors.New("x509: RSA key missing NULL parameters")
		}

		der := cryptobyte.String(data)
		p := &pkcs1PublicKey{N: new(big.Int)}
		if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
			return nil, errors.New("x509: invalid RSA public key")
		}
		if !der.ReadASN1Integer(p.N) {
			return nil, errors.New("x509: invalid RSA modulus")
		}
		if !der.ReadASN1Integer(&p.E) {
			return nil, errors.New("x509: invalid RSA public exponent")
		}

		if p.N.Sign() <= 0 {
			return nil, errors.New("x509: RSA modulus is not a positive number")
		}
		if p.E <= 0 {
			return nil, errors.New("x509: RSA public exponent is not a positive number")
		}

		pub := &rsa.PublicKey{
			E: p.E,
			N: p.N,
		}
		return pub, nil
	case oid.Equal(oidPublicKeyECDSA):
		paramsDer := cryptobyte.String(params.FullBytes)
		namedCurveOID := new(asn1.ObjectIdentifier)
		if !paramsDer.ReadASN1ObjectIdentifier(namedCurveOID) {
			return nil, errors.New("x509: invalid ECDSA parameters")
		}
		namedCurve := namedCurveFromOID(*namedCurveOID)
		if namedCurve == nil {
			return nil, errors.New("x509: unsupported elliptic curve")
		}
		return ecdsa.ParseUncompressedPublicKey(namedCurve, data)
	case oid.Equal(oidPublicKeyEd25519):
		// RFC 8410, Section 3
		// > For all of the OIDs, the parameters MUST be absent.
		if len(params.FullBytes) != 0 {
			return nil, errors.New("x509: Ed25519 key encoded with illegal parameters")
		}
		if len(data) != ed25519.PublicKeySize {
			return nil, errors.New("x509: wrong Ed25519 public key size")
		}
		return ed25519.PublicKey(data), nil
	case oid.Equal(oidPublicKeyMLDSA44), oid.Equal(oidPublicKeyMLDSA65), oid.Equal(oidPublicKeyMLDSA87):
		if len(params.FullBytes) != 0 {
			return nil, errors.New("x509: ML-DSA key encoded with illegal parameters")
		}
		params, ok := mldsaParametersFromOID(oid)
		if !ok {
			return nil, errors.New("x509: unsupported ML-DSA parameters")
		}
		return mldsa.NewPublicKey(params, data)
	case oid.Equal(oidPublicKeyX25519):
		// RFC 8410, Section 3
		// > For all of the OIDs, the parameters MUST be absent.
		if len(params.FullBytes) != 0 {
			return nil, errors.New("x509: X25519 key encoded with illegal parameters")
		}
		return ecdh.X25519().NewPublicKey(data)
	case oid.Equal(oidPublicKeyDSA):
		der := cryptobyte.String(data)
		y := new(big.Int)
		if !der.ReadASN1Integer(y) {
			return nil, errors.New("x509: invalid DSA public key")
		}
		pub := &dsa.PublicKey{
			Y: y,
			Parameters: dsa.Parameters{
				P: new(big.Int),
				Q: new(big.Int),
				G: new(big.Int),
			},
		}
		paramsDer := cryptobyte.String(params.FullBytes)
		if !paramsDer.ReadASN1(&paramsDer, cryptobyte_asn1.SEQUENCE) ||
			!paramsDer.ReadASN1Integer(pub.Parameters.P) ||
			!paramsDer.ReadASN1Integer(pub.Parameters.Q) ||
			!paramsDer.ReadASN1Integer(pub.Parameters.G) {
			return nil, errors.New("x509: invalid DSA parameters")
		}
		if pub.Y.Sign() <= 0 || pub.Parameters.P.Sign() <= 0 ||
			pub.Parameters.Q.Sign() <= 0 || pub.Parameters.G.Sign() <= 0 {
			return nil, errors.New("x509: zero or negative DSA parameter")
		}
		return pub, nil
	default:
		return nil, errors.New("x509: unknown public key algorithm")
	}
}

func parseKeyUsageExtension(der cryptobyte.String) (KeyUsage, error) {
	var usageBits asn1.BitString
	if !der.ReadASN1BitString(&usageBits) {
		return 0, errors.New("x509: invalid key usage")
	}

	var usage int
	for i := 0; i < 9; i++ {
		if usageBits.At(i) != 0 {
			usage |= 1 << uint(i)
		}
	}
	return KeyUsage(usage), nil
}

func parseBasicConstraintsExtension(der cryptobyte.String) (bool, int, error) {
	var isCA bool
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return false, 0, errors.New("x509: invalid basic constraints")
	}
	if der.PeekASN1Tag(cryptobyte_asn1.BOOLEAN) {
		if !der.ReadASN1Boolean(&isCA) {
			return false, 0, errors.New("x509: invalid basic constraints")
		}
	}

	maxPathLen := -1
	if der.PeekASN1Tag(cryptobyte_asn1.INTEGER) {
		var mpl uint
		if !der.ReadASN1Integer(&mpl) || mpl > math.MaxInt {
			return false, 0, errors.New("x509: invalid basic constraints")
		}
		maxPathLen = int(mpl)
	}

	return isCA, maxPathLen, nil
}

func forEachSAN(der cryptobyte.String, callback func(tag int, data []byte) error) error {
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return errors.New("x509: invalid subject alternative names")
	}
	for !der.Empty() {
		var san cryptobyte.String
		var tag cryptobyte_asn1.Tag
		if !der.ReadAnyASN1(&san, &tag) {
			return errors.New("x509: invalid subject alternative name")
		}
		if err := callback(int(tag^0x80), san); err != nil {
			return err
		}
	}

	return nil
}

func parseSANExtension(der cryptobyte.String) (dnsNames, emailAddresses []string, ipAddresses []net.IP, uris []*url.URL, err error) {
	err = forEachSAN(der, func(tag int, data []byte) error {
		switch tag {
		case nameTypeEmail:
			email := string(data)
			if err := isIA5String(email); err != nil {
				return errors.New("x509: SAN rfc822Name is malformed")
			}
			emailAddresses = append(emailAddresses, email)
		case nameTypeDNS:
			name := string(data)
			if err := isIA5String(name); err != nil {
				return errors.New("x509: SAN dNSName is malformed")
			}
			dnsNames = append(dnsNames, string(name))
		case nameTypeURI:
			uriStr := string(data)
			if err := isIA5String(uriStr); err != nil {
				return errors.New("x509: SAN uniformResourceIdentifier is malformed")
			}
			uri, err := url.Parse(uriStr)
			if err != nil {
				return fmt.Errorf("x509: cannot parse URI %q: %s", uriStr, err)
			}
			if len(uri.Host) > 0 && !domainNameValid(uri.Host, false) {
				return fmt.Errorf("x509: cannot parse URI %q: invalid domain", uriStr)
			}
			uris = append(uris, uri)
		case nameTypeIP:
			switch len(data) {
			case net.IPv6len:
				if net.IP(data).To4() != nil {
					return errors.New("x509: SAN iPAddress contains IPv4-mapped IPv6 address")
				}
				ipAddresses = append(ipAddresses, data)
			case net.IPv4len:
				ipAddresses = append(ipAddresses, data)
			default:
				return errors.New("x509: cannot parse IP address of length " + strconv.Itoa(len(data)))
			}
		}

		return nil
	})

	return
}

func parseAuthorityKeyIdentifier(e pkix.Extension) ([]byte, error) {
	// RFC 5280, Section 4.2.1.1
	if e.Critical {
		// Conforming CAs MUST mark this extension as non-critical
		return nil, errors.New("x509: authority key identifier incorrectly marked critical")
	}
	val := cryptobyte.String(e.Value)
	var akid cryptobyte.String
	if !val.ReadASN1(&akid, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: invalid authority key identifier")
	}
	if akid.PeekASN1Tag(cryptobyte_asn1.Tag(0).ContextSpecific()) {
		if !akid.ReadASN1(&akid, cryptobyte_asn1.Tag(0).ContextSpecific()) {
			return nil, errors.New("x509: invalid authority key identifier")
		}
		return akid, nil
	}
	return nil, nil
}

func parseExtKeyUsageExtension(der cryptobyte.String) ([]ExtKeyUsage, []asn1.ObjectIdentifier, error) {
	var extKeyUsages []ExtKeyUsage
	var unknownUsages []asn1.ObjectIdentifier
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return nil, nil, errors.New("x509: invalid extended key usages")
	}
	for !der.Empty() {
		var eku asn1.ObjectIdentifier
		if !der.ReadASN1ObjectIdentifier(&eku) {
			return nil, nil, errors.New("x509: invalid extended key usages")
		}
		if extKeyUsage, ok := extKeyUsageFromOID(eku); ok {
			extKeyUsages = append(extKeyUsages, extKeyUsage)
		} else {
			unknownUsages = append(unknownUsages, eku)
		}
	}
	return extKeyUsages, unknownUsages, nil
}

func parseCertificatePoliciesExtension(der cryptobyte.String) ([]OID, error) {
	var oids []OID
	seenOIDs := map[string]bool{}
	if !der.ReadASN1(&der, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: invalid certificate policies")
	}
	for !der.Empty() {
		var cp cryptobyte.String
		var OIDBytes cryptobyte.String
		if !der.ReadASN1(&cp, cryptobyte_asn1.SEQUENCE) || !cp.ReadASN1(&OIDBytes, cryptobyte_asn1.OBJECT_IDENTIFIER) {
			return nil, errors.New("x509: invalid certificate policies")
		}
		if seenOIDs[string(OIDBytes)] {
			return nil, errors.New("x509: invalid certificate policies")
		}
		seenOIDs[string(OIDBytes)] = true
		oid, ok := newOIDFromDER(OIDBytes)
		if !ok {
			return nil, errors.New("x509: invalid certificate policies")
		}
		oids = append(oids, oid)
	}
	return oids, nil
}

// isValidIPMask reports whether mask consists of zero or more 1 bits, followed by zero bits.
func isValidIPMask(mask []byte) bool {
	seenZero := false

	for _, b := range mask {
		if seenZero {
			if b != 0 {
				return false
			}

			continue
		}

		switch b {
		case 0x00, 0x80, 0xc0, 0xe0, 0xf0, 0xf8, 0xfc, 0xfe:
			seenZero = true
		case 0xff:
		default:
			return false
		}
	}

	return true
}

func parseNameConstraintsExtension(out *Certificate, e pkix.Extension) (unhandled bool, err error) {
	// RFC 5280, 4.2.1.10

	// NameConstraints ::= SEQUENCE {
	//      permittedSubtrees       [0]     GeneralSubtrees OPTIONAL,
	//      excludedSubtrees        [1]     GeneralSubtrees OPTIONAL }
	//
	// GeneralSubtrees ::= SEQUENCE SIZE (1..MAX) OF GeneralSubtree
	//
	// GeneralSubtree ::= SEQUENCE {
	//      base                    GeneralName,
	//      minimum         [0]     BaseDistance DEFAULT 0,
	//      maximum         [1]     BaseDistance OPTIONAL }
	//
	// BaseDistance ::= INTEGER (0..MAX)

	outer := cryptobyte.String(e.Value)
	var toplevel, permitted, excluded cryptobyte.String
	var havePermitted, haveExcluded bool
	if !outer.ReadASN1(&toplevel, cryptobyte_asn1.SEQUENCE) ||
		!outer.Empty() ||
		!toplevel.ReadOptionalASN1(&permitted, &havePermitted, cryptobyte_asn1.Tag(0).ContextSpecific().Constructed()) ||
		!toplevel.ReadOptionalASN1(&excluded, &haveExcluded, cryptobyte_asn1.Tag(1).ContextSpecific().Constructed()) ||
		!toplevel.Empty() {
		return false, errors.New("x509: invalid NameConstraints extension")
	}

	if !havePermitted && !haveExcluded || len(permitted) == 0 && len(excluded) == 0 {
		// From RFC 5280, Section 4.2.1.10:
		//   “either the permittedSubtrees field
		//   or the excludedSubtrees MUST be
		//   present”
		return false, errors.New("x509: empty name constraints extension")
	}

	getValues := func(subtrees cryptobyte.String) (dnsNames []string, ips []*net.IPNet, emails, uriDomains []string, err error) {
		for !subtrees.Empty() {
			var seq, value cryptobyte.String
			var tag cryptobyte_asn1.Tag
			if !subtrees.ReadASN1(&seq, cryptobyte_asn1.SEQUENCE) ||
				!seq.ReadAnyASN1(&value, &tag) {
				return nil, nil, nil, nil, fmt.Errorf("x509: invalid NameConstraints extension")
			}

			var (
				dnsTag   = cryptobyte_asn1.Tag(2).ContextSpecific()
				emailTag = cryptobyte_asn1.Tag(1).ContextSpecific()
				ipTag    = cryptobyte_asn1.Tag(7).ContextSpecific()
				uriTag   = cryptobyte_asn1.Tag(6).ContextSpecific()
			)

			switch tag {
			case dnsTag:
				domain := string(value)
				if err := isIA5String(domain); err != nil {
					return nil, nil, nil, nil, errors.New("x509: invalid constraint value: " + err.Error())
				}

				if !domainNameValid(domain, true) {
					return nil, nil, nil, nil, fmt.Errorf("x509: failed to parse dnsName constraint %q", domain)
				}
				dnsNames = append(dnsNames, domain)

			case ipTag:
				l := len(value)
				var ip, mask []byte

				switch l {
				case 8:
					ip = value[:4]
					mask = value[4:]

				case 32:
					ip = value[:16]
					mask = value[16:]

				default:
					return nil, nil, nil, nil, fmt.Errorf("x509: IP constraint contained value of length %d", l)
				}

				if !isValidIPMask(mask) {
					return nil, nil, nil, nil, fmt.Errorf("x509: IP constraint contained invalid mask %x", mask)
				}

				if len(ip) == net.IPv6len && net.IP(ip).To4() != nil {
					return nil, nil, nil, nil, errors.New("x509: IP constraint contained IPv4-mapped IPv6 address")
				}

				ips = append(ips, &net.IPNet{IP: net.IP(ip), Mask: net.IPMask(mask)})

			case emailTag:
				constraint := string(value)
				if err := isIA5String(constraint); err != nil {
					return nil, nil, nil, nil, errors.New("x509: invalid constraint value: " + err.Error())
				}

				// If the constraint contains an @ then
				// it specifies an exact mailbox name.
				if strings.Contains(constraint, "@") {
					if _, ok := parseRFC2821Mailbox(constraint); !ok {
						return nil, nil, nil, nil, fmt.Errorf("x509: failed to parse rfc822Name constraint %q", constraint)
					}
				} else {
					if !domainNameValid(constraint, true) {
						return nil, nil, nil, nil, fmt.Errorf("x509: failed to parse rfc822Name constraint %q", constraint)
					}
				}
				emails = append(emails, constraint)

			case uriTag:
				domain := string(value)
				if err := isIA5String(domain); err != nil {
					return nil, nil, nil, nil, errors.New("x509: invalid constraint value: " + err.Error())
				}

				if net.ParseIP(domain) != nil {
					return nil, nil, nil, nil, fmt.Errorf("x509: failed to parse URI constraint %q: cannot be IP address", domain)
				}

				if !domainNameValid(domain, true) {
					return nil, nil, nil, nil, fmt.Errorf("x509: failed to parse URI constraint %q", domain)
				}
				uriDomains = append(uriDomains, domain)

			default:
				unhandled = true
			}
		}

		return dnsNames, ips, emails, uriDomains, nil
	}

	if out.PermittedDNSDomains, out.PermittedIPRanges, out.PermittedEmailAddresses, out.PermittedURIDomains, err = getValues(permitted); err != nil {
		return false, err
	}
	if out.ExcludedDNSDomains, out.ExcludedIPRanges, out.ExcludedEmailAddresses, out.ExcludedURIDomains, err = getValues(excluded); err != nil {
		return false, err
	}
	out.PermittedDNSDomainsCritical = e.Critical

	return unhandled, nil
}

func processExtensions(out *Certificate) error {
	var err error
	for _, e := range out.Extensions {
		unhandled := false

		if len(e.Id) == 4 && e.Id[0] == 2 && e.Id[1] == 5 && e.Id[2] == 29 {
			switch e.Id[3] {
			case 15:
				out.KeyUsage, err = parseKeyUsageExtension(e.Value)
				if err != nil {
					return err
				}
			case 19:
				out.IsCA, out.MaxPathLen, err = parseBasicConstraintsExtension(e.Value)
				if err != nil {
					return err
				}
				out.BasicConstraintsValid = true
				out.MaxPathLenZero = out.MaxPathLen == 0
			case 17:
				out.DNSNames, out.EmailAddresses, out.IPAddresses, out.URIs, err = parseSANExtension(e.Value)
				if err != nil {
					return err
				}

				if len(out.DNSNames) == 0 && len(out.EmailAddresses) == 0 && len(out.IPAddresses) == 0 && len(out.URIs) == 0 {
					// If we didn't parse anything then we do the critical check, below.
					unhandled = true
				}

			case 30:
				unhandled, err = parseNameConstraintsExtension(out, e)
				if err != nil {
					return err
				}

			case 31:
				// RFC 5280, 4.2.1.13

				// CRLDistributionPoints ::= SEQUENCE SIZE (1..MAX) OF DistributionPoint
				//
				// DistributionPoint ::= SEQUENCE {
				//     distributionPoint       [0]     DistributionPointName OPTIONAL,
				//     reasons                 [1]     ReasonFlags OPTIONAL,
				//     cRLIssuer               [2]     GeneralNames OPTIONAL }
				//
				// DistributionPointName ::= CHOICE {
				//     fullName                [0]     GeneralNames,
				//     nameRelativeToCRLIssuer [1]     RelativeDistinguishedName }
				val := cryptobyte.String(e.Value)
				if !val.ReadASN1(&val, cryptobyte_asn1.SEQUENCE) {
					return errors.New("x509: invalid CRL distribution points")
				}
				for !val.Empty() {
					var dpDER cryptobyte.String
					if !val.ReadASN1(&dpDER, cryptobyte_asn1.SEQUENCE) {
						return errors.New("x509: invalid CRL distribution point")
					}
					var dpNameDER cryptobyte.String
					var dpNamePresent bool
					if !dpDER.ReadOptionalASN1(&dpNameDER, &dpNamePresent, cryptobyte_asn1.Tag(0).Constructed().ContextSpecific()) {
						return errors.New("x509: invalid CRL distribution point")
					}
					if !dpNamePresent {
						continue
					}
					if !dpNameDER.ReadASN1(&dpNameDER, cryptobyte_asn1.Tag(0).Constructed().ContextSpecific()) {
						return errors.New("x509: invalid CRL distribution point")
					}
					for !dpNameDER.Empty() {
						if !dpNameDER.PeekASN1Tag(cryptobyte_asn1.Tag(6).ContextSpecific()) {
							break
						}
						var uri cryptobyte.String
						if !dpNameDER.ReadASN1(&uri, cryptobyte_asn1.Tag(6).ContextSpecific()) {
							return errors.New("x509: invalid CRL distribution point")
						}
						out.CRLDistributionPoints = append(out.CRLDistributionPoints, string(uri))
					}
				}

			case 35:
				out.AuthorityKeyId, err = parseAuthorityKeyIdentifier(e)
				if err != nil {
					return err
				}
			case 36:
				val := cryptobyte.String(e.Value)
				if !val.ReadASN1(&val, cryptobyte_asn1.SEQUENCE) {
					return errors.New("x509: invalid policy constraints extension")
				}
				if val.PeekASN1Tag(cryptobyte_asn1.Tag(0).ContextSpecific()) {
					var v int64
					if !val.ReadASN1Int64WithTag(&v, cryptobyte_asn1.Tag(0).ContextSpecific()) {
						return errors.New("x509: invalid policy constraints extension")
					}
					out.RequireExplicitPolicy = int(v)
					// Check for overflow.
					if int64(out.RequireExplicitPolicy) != v {
						return errors.New("x509: policy constraints requireExplicitPolicy field overflows int")
					}
					out.RequireExplicitPolicyZero = out.RequireExplicitPolicy == 0
				}
				if val.PeekASN1Tag(cryptobyte_asn1.Tag(1).ContextSpecific()) {
					var v int64
					if !val.ReadASN1Int64WithTag(&v, cryptobyte_asn1.Tag(1).ContextSpecific()) {
						return errors.New("x509: invalid policy constraints extension")
					}
					out.InhibitPolicyMapping = int(v)
					// Check for overflow.
					if int64(out.InhibitPolicyMapping) != v {
						return errors.New("x509: policy constraints inhibitPolicyMapping field overflows int")
					}
					out.InhibitPolicyMappingZero = out.InhibitPolicyMapping == 0
				}
			case 37:
				out.ExtKeyUsage, out.UnknownExtKeyUsage, err = parseExtKeyUsageExtension(e.Value)
				if err != nil {
					return err
				}
			case 14: // RFC 5280, 4.2.1.2
				if e.Critical {
					// Conforming CAs MUST mark this extension as non-critical
					return errors.New("x509: subject key identifier incorrectly marked critical")
				}
				val := cryptobyte.String(e.Value)
				var skid cryptobyte.String
				if !val.ReadASN1(&skid, cryptobyte_asn1.OCTET_STRING) {
					return errors.New("x509: invalid subject key identifier")
				}
				out.SubjectKeyId = skid
			case 32:
				out.Policies, err = parseCertificatePoliciesExtension(e.Value)
				if err != nil {
					return err
				}
				out.PolicyIdentifiers = make([]asn1.ObjectIdentifier, 0, len(out.Policies))
				for _, oid := range out.Policies {
					if oid, ok := oid.toASN1OID(); ok {
						out.PolicyIdentifiers = append(out.PolicyIdentifiers, oid)
					}
				}
			case 33:
				val := cryptobyte.String(e.Value)
				if !val.ReadASN1(&val, cryptobyte_asn1.SEQUENCE) {
					return errors.New("x509: invalid policy mappings extension")
				}
				for !val.Empty() {
					var s cryptobyte.String
					var issuer, subject cryptobyte.String
					if !val.ReadASN1(&s, cryptobyte_asn1.SEQUENCE) ||
						!s.ReadASN1(&issuer, cryptobyte_asn1.OBJECT_IDENTIFIER) ||
						!s.ReadASN1(&subject, cryptobyte_asn1.OBJECT_IDENTIFIER) {
						return errors.New("x509: invalid policy mappings extension")
					}
					out.PolicyMappings = append(out.PolicyMappings, PolicyMapping{OID{issuer}, OID{subject}})
				}
			case 54:
				val := cryptobyte.String(e.Value)
				if !val.ReadASN1Integer(&out.InhibitAnyPolicy) {
					return errors.New("x509: invalid inhibit any policy extension")
				}
				out.InhibitAnyPolicyZero = out.InhibitAnyPolicy == 0
			default:
				// Unknown extensions are recorded if critical.
				unhandled = true
			}
		} else if e.Id.Equal(oidExtensionAuthorityInfoAccess) {
			// RFC 5280 4.2.2.1: Authority Information Access
			if e.Critical {
				// Conforming CAs MUST mark this extension as non-critical
				return errors.New("x509: authority info access incorrectly marked critical")
			}
			val := cryptobyte.String(e.Value)
			if !val.ReadASN1(&val, cryptobyte_asn1.SEQUENCE) {
				return errors.New("x509: invalid authority info access")
			}
			for !val.Empty() {
				var aiaDER cryptobyte.String
				if !val.ReadASN1(&aiaDER, cryptobyte_asn1.SEQUENCE) {
					return errors.New("x509: invalid authority info access")
				}
				var method asn1.ObjectIdentifier
				if !aiaDER.ReadASN1ObjectIdentifier(&method) {
					return errors.New("x509: invalid authority info access")
				}
				if !aiaDER.PeekASN1Tag(cryptobyte_asn1.Tag(6).ContextSpecific()) {
					continue
				}
				if !aiaDER.ReadASN1(&aiaDER, cryptobyte_asn1.Tag(6).ContextSpecific()) {
					return errors.New("x509: invalid authority info access")
				}
				switch {
				case method.Equal(oidAuthorityInfoAccessOcsp):
					out.OCSPServer = append(out.OCSPServer, string(aiaDER))
				case method.Equal(oidAuthorityInfoAccessIssuers):
					out.IssuingCertificateURL = append(out.IssuingCertificateURL, string(aiaDER))
				}
			}
		} else {
			// Unknown extensions are recorded if critical.
			unhandled = true
		}

		if e.Critical && unhandled {
			out.UnhandledCriticalExtensions = append(out.UnhandledCriticalExtensions, e.Id)
		}
	}

	return nil
}

var x509negativeserial = godebug.New("x509negativeserial")

func parseCertificate(der []byte) (*Certificate, error) {
	cert := &Certificate{}

	input := cryptobyte.String(der)
	// we read the SEQUENCE including length and tag bytes so that
	// we can populate Certificate.Raw, before unwrapping the
	// SEQUENCE so it can be operated on
	if !input.ReadASN1Element(&input, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed certificate")
	}
	cert.Raw = input
	if !input.ReadASN1(&input, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed certificate")
	}

	var tbs cryptobyte.String
	// do the same trick again as above to extract the raw
	// bytes for Certificate.RawTBSCertificate
	if !input.ReadASN1Element(&tbs, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed tbs certificate")
	}
	cert.RawTBSCertificate = tbs
	if !tbs.ReadASN1(&tbs, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed tbs certificate")
	}

	if !tbs.ReadOptionalASN1Integer(&cert.Version, cryptobyte_asn1.Tag(0).Constructed().ContextSpecific(), 0) {
		return nil, errors.New("x509: malformed version")
	}
	if cert.Version < 0 {
		return nil, errors.New("x509: malformed version")
	}
	// for backwards compat reasons Version is one-indexed,
	// rather than zero-indexed as defined in 5280
	cert.Version++
	if cert.Version > 3 {
		return nil, errors.New("x509: invalid version")
	}

	serial := new(big.Int)
	if !tbs.ReadASN1Integer(serial) {
		return nil, errors.New("x509: malformed serial number")
	}
	if serial.Sign() == -1 {
		if x509negativeserial.Value() != "1" {
			return nil, errors.New("x509: negative serial number")
		} else {
			x509negativeserial.IncNonDefault()
		}
	}
	cert.SerialNumber = serial

	var sigAISeq cryptobyte.String
	if !tbs.ReadASN1Element(&sigAISeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed signature algorithm identifier")
	}
	cert.RawSignatureAlgorithm = sigAISeq
	if !sigAISeq.ReadASN1(&sigAISeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed signature algorithm identifier")
	}
	// Before parsing the inner algorithm identifier, extract
	// the outer algorithm identifier and make sure that they
	// match.
	var outerSigAISeq cryptobyte.String
	if !input.ReadASN1(&outerSigAISeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed algorithm identifier")
	}
	if !bytes.Equal(outerSigAISeq, sigAISeq) {
		return nil, errors.New("x509: inner and outer signature algorithm identifiers don't match")
	}
	sigAI, err := parseAI(sigAISeq)
	if err != nil {
		return nil, err
	}
	cert.SignatureAlgorithm = getSignatureAlgorithmFromAI(sigAI)

	var issuerSeq cryptobyte.String
	if !tbs.ReadASN1Element(&issuerSeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed issuer")
	}
	cert.RawIssuer = issuerSeq
	issuerRDNs, err := parseName(issuerSeq)
	if err != nil {
		return nil, err
	}
	cert.Issuer.FillFromRDNSequence(issuerRDNs)

	var validity cryptobyte.String
	if !tbs.ReadASN1(&validity, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed validity")
	}
	cert.NotBefore, cert.NotAfter, err = parseValidity(validity)
	if err != nil {
		return nil, err
	}

	var subjectSeq cryptobyte.String
	if !tbs.ReadASN1Element(&subjectSeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed issuer")
	}
	cert.RawSubject = subjectSeq
	subjectRDNs, err := parseName(subjectSeq)
	if err != nil {
		return nil, err
	}
	cert.Subject.FillFromRDNSequence(subjectRDNs)

	var spki cryptobyte.String
	if !tbs.ReadASN1Element(&spki, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed spki")
	}
	cert.RawSubjectPublicKeyInfo = spki
	if !spki.ReadASN1(&spki, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed spki")
	}
	var pkAISeq cryptobyte.String
	if !spki.ReadASN1(&pkAISeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed public key algorithm identifier")
	}
	pkAI, err := parseAI(pkAISeq)
	if err != nil {
		return nil, err
	}
	cert.PublicKeyAlgorithm = getPublicKeyAlgorithmFromOID(pkAI.Algorithm)
	var spk asn1.BitString
	if !spki.ReadASN1BitString(&spk) {
		return nil, errors.New("x509: malformed subjectPublicKey")
	}
	if cert.PublicKeyAlgorithm != UnknownPublicKeyAlgorithm {
		cert.PublicKey, err = parsePublicKey(&publicKeyInfo{
			Algorithm: pkAI,
			PublicKey: spk,
		})
		if err != nil {
			return nil, err
		}
	}

	if cert.Version > 1 {
		if !tbs.SkipOptionalASN1(cryptobyte_asn1.Tag(1).ContextSpecific()) {
			return nil, errors.New("x509: malformed issuerUniqueID")
		}
		if !tbs.SkipOptionalASN1(cryptobyte_asn1.Tag(2).ContextSpecific()) {
			return nil, errors.New("x509: malformed subjectUniqueID")
		}
		if cert.Version == 3 {
			var extensions cryptobyte.String
			var present bool
			if !tbs.ReadOptionalASN1(&extensions, &present, cryptobyte_asn1.Tag(3).Constructed().ContextSpecific()) {
				return nil, errors.New("x509: malformed extensions")
			}
			if present {
				seenExts := make(map[string]bool)
				if !extensions.ReadASN1(&extensions, cryptobyte_asn1.SEQUENCE) {
					return nil, errors.New("x509: malformed extensions")
				}
				for !extensions.Empty() {
					var extension cryptobyte.String
					if !extensions.ReadASN1(&extension, cryptobyte_asn1.SEQUENCE) {
						return nil, errors.New("x509: malformed extension")
					}
					ext, err := parseExtension(extension)
					if err != nil {
						return nil, err
					}
					oidStr := ext.Id.String()
					if seenExts[oidStr] {
						return nil, fmt.Errorf("x509: certificate contains duplicate extension with OID %q", oidStr)
					}
					seenExts[oidStr] = true
					cert.Extensions = append(cert.Extensions, ext)
				}
				err = processExtensions(cert)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	var signature asn1.BitString
	if !input.ReadASN1BitString(&signature) {
		return nil, errors.New("x509: malformed signature")
	}
	cert.Signature = signature.RightAlign()

	return cert, nil
}

// ParseCertificate parses a single certificate from the given ASN.1 DER data.
//
// Before Go 1.23, ParseCertificate accepted certificates with negative serial
// numbers. This behavior can be restored by including "x509negativeserial=1" in
// the GODEBUG environment variable.
func ParseCertificate(der []byte) (*Certificate, error) {
	cert, err := parseCertificate(der)
	if err != nil {
		return nil, err
	}
	if len(der) != len(cert.Raw) {
		return nil, errors.New("x509: trailing data")
	}
	return cert, nil
}

// ParseCertificates parses one or more certificates from the given ASN.1 DER
// data. The certificates must be concatenated with no intermediate padding.
func ParseCertificates(der []byte) ([]*Certificate, error) {
	var certs []*Certificate
	for len(der) > 0 {
		cert, err := parseCertificate(der)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
		der = der[len(cert.Raw):]
	}
	return certs, nil
}

// The X.509 standards confusingly 1-indexed the version names, but 0-indexed
// the actual encoded version, so the version for X.509v2 is 1.
const x509v2Version = 1

// ParseRevocationList parses a X509 v2 [Certificate] Revocation List from the given
// ASN.1 DER data.
func ParseRevocationList(der []byte) (*RevocationList, error) {
	rl := &RevocationList{}

	input := cryptobyte.String(der)
	// we read the SEQUENCE including length and tag bytes so that
	// we can populate RevocationList.Raw, before unwrapping the
	// SEQUENCE so it can be operated on
	if !input.ReadASN1Element(&input, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed crl")
	}
	rl.Raw = input
	if !input.ReadASN1(&input, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed crl")
	}

	var tbs cryptobyte.String
	// do the same trick again as above to extract the raw
	// bytes for Certificate.RawTBSCertificate
	if !input.ReadASN1Element(&tbs, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed tbs crl")
	}
	rl.RawTBSRevocationList = tbs
	if !tbs.ReadASN1(&tbs, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed tbs crl")
	}

	var version int
	if !tbs.PeekASN1Tag(cryptobyte_asn1.INTEGER) {
		return nil, errors.New("x509: unsupported crl version")
	}
	if !tbs.ReadASN1Integer(&version) {
		return nil, errors.New("x509: malformed crl")
	}
	if version != x509v2Version {
		return nil, fmt.Errorf("x509: unsupported crl version: %d", version)
	}

	var sigAISeq cryptobyte.String
	if !tbs.ReadASN1Element(&sigAISeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed signature algorithm identifier")
	}
	rl.RawSignatureAlgorithm = sigAISeq
	if !sigAISeq.ReadASN1(&sigAISeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed signature algorithm identifier")
	}
	// Before parsing the inner algorithm identifier, extract
	// the outer algorithm identifier and make sure that they
	// match.
	var outerSigAISeq cryptobyte.String
	if !input.ReadASN1(&outerSigAISeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed algorithm identifier")
	}
	if !bytes.Equal(outerSigAISeq, sigAISeq) {
		return nil, errors.New("x509: inner and outer signature algorithm identifiers don't match")
	}
	sigAI, err := parseAI(sigAISeq)
	if err != nil {
		return nil, err
	}
	rl.SignatureAlgorithm = getSignatureAlgorithmFromAI(sigAI)

	var signature asn1.BitString
	if !input.ReadASN1BitString(&signature) {
		return nil, errors.New("x509: malformed signature")
	}
	rl.Signature = signature.RightAlign()

	var issuerSeq cryptobyte.String
	if !tbs.ReadASN1Element(&issuerSeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed issuer")
	}
	rl.RawIssuer = issuerSeq
	issuerRDNs, err := parseName(issuerSeq)
	if err != nil {
		return nil, err
	}
	rl.Issuer.FillFromRDNSequence(issuerRDNs)

	rl.ThisUpdate, err = readASN1Time(&tbs)
	if err != nil {
		return nil, err
	}
	if tbs.PeekASN1Tag(cryptobyte_asn1.GeneralizedTime) || tbs.PeekASN1Tag(cryptobyte_asn1.UTCTime) {
		rl.NextUpdate, err = readASN1Time(&tbs)
		if err != nil {
			return nil, err
		}
	}

	if tbs.PeekASN1Tag(cryptobyte_asn1.SEQUENCE) {
		var revokedSeq cryptobyte.String
		if !tbs.ReadASN1(&revokedSeq, cryptobyte_asn1.SEQUENCE) {
			return nil, errors.New("x509: malformed crl")
		}
		for !revokedSeq.Empty() {
			rce := RevocationListEntry{}

			var certSeq cryptobyte.String
			if !revokedSeq.ReadASN1Element(&certSeq, cryptobyte_asn1.SEQUENCE) {
				return nil, errors.New("x509: malformed crl")
			}
			rce.Raw = certSeq
			if !certSeq.ReadASN1(&certSeq, cryptobyte_asn1.SEQUENCE) {
				return nil, errors.New("x509: malformed crl")
			}

			rce.SerialNumber = new(big.Int)
			if !certSeq.ReadASN1Integer(rce.SerialNumber) {
				return nil, errors.New("x509: malformed serial number")
			}
			rce.RevocationTime, err = readASN1Time(&certSeq)
			if err != nil {
				return nil, err
			}
			var extensions cryptobyte.String
			var present bool
			if !certSeq.ReadOptionalASN1(&extensions, &present, cryptobyte_asn1.SEQUENCE) {
				return nil, errors.New("x509: malformed extensions")
			}
			if present {
				for !extensions.Empty() {
					var extension cryptobyte.String
					if !extensions.ReadASN1(&extension, cryptobyte_asn1.SEQUENCE) {
						return nil, errors.New("x509: malformed extension")
					}
					ext, err := parseExtension(extension)
					if err != nil {
						return nil, err
					}
					if ext.Id.Equal(oidExtensionReasonCode) {
						val := cryptobyte.String(ext.Value)
						if !val.ReadASN1Enum(&rce.ReasonCode) {
							return nil, fmt.Errorf("x509: malformed reasonCode extension")
						}
					}
					rce.Extensions = append(rce.Extensions, ext)
				}
			}

			rl.RevokedCertificateEntries = append(rl.RevokedCertificateEntries, rce)
			rcDeprecated := pkix.RevokedCertificate{
				SerialNumber:   rce.SerialNumber,
				RevocationTime: rce.RevocationTime,
				Extensions:     rce.Extensions,
			}
			rl.RevokedCertificates = append(rl.RevokedCertificates, rcDeprecated)
		}
	}

	var extensions cryptobyte.String
	var present bool
	if !tbs.ReadOptionalASN1(&extensions, &present, cryptobyte_asn1.Tag(0).Constructed().ContextSpecific()) {
		return nil, errors.New("x509: malformed extensions")
	}
	if present {
		if !extensions.ReadASN1(&extensions, cryptobyte_asn1.SEQUENCE) {
			return nil, errors.New("x509: malformed extensions")
		}
		for !extensions.Empty() {
			var extension cryptobyte.String
			if !extensions.ReadASN1(&extension, cryptobyte_asn1.SEQUENCE) {
				return nil, errors.New("x509: malformed extension")
			}
			ext, err := parseExtension(extension)
			if err != nil {
				return nil, err
			}
			if ext.Id.Equal(oidExtensionAuthorityKeyId) {
				rl.AuthorityKeyId, err = parseAuthorityKeyIdentifier(ext)
				if err != nil {
					return nil, err
				}
			} else if ext.Id.Equal(oidExtensionCRLNumber) {
				value := cryptobyte.String(ext.Value)
				rl.Number = new(big.Int)
				if !value.ReadASN1Integer(rl.Number) {
					return nil, errors.New("x509: malformed crl number")
				}
			}
			rl.Extensions = append(rl.Extensions, ext)
		}
	}

	return rl, nil
}

// domainNameValid is an alloc-less version of the checks that
// domainToReverseLabels does.
func domainNameValid(s string, constraint bool) bool {
	// TODO(#75835): This function omits a number of checks which we
	// really should be doing to enforce that domain names are valid names per
	// RFC 1034. We previously enabled these checks, but this broke a
	// significant number of certificates we previously considered valid, and we
	// happily create via CreateCertificate (et al). We should enable these
	// checks, but will need to gate them behind a GODEBUG.
	//
	// I have left the checks we previously enabled, noted with "TODO(#75835)" so
	// that we can easily re-enable them once we unbreak everyone.

	// TODO(#75835): this should only be true for constraints.
	if len(s) == 0 {
		return true
	}

	// Do not allow trailing period (FQDN format is not allowed in SANs or
	// constraints).
	if s[len(s)-1] == '.' {
		return false
	}

	// TODO(#75835): domains must have at least one label, cannot have
	// a leading empty label, and cannot be longer than 253 characters.
	// if len(s) == 0 || (!constraint && s[0] == '.') || len(s) > 253 {
	// 	return false
	// }

	lastDot := -1
	if constraint && s[0] == '.' {
		s = s[1:]
	}

	for i := 0; i <= len(s); i++ {
		if i < len(s) && (s[i] < 33 || s[i] > 126) {
			// Invalid character.
			return false
		}
		if i == len(s) || s[i] == '.' {
			labelLen := i
			if lastDot >= 0 {
				labelLen -= lastDot + 1
			}
			if labelLen == 0 {
				return false
			}
			// TODO(#75835): labels cannot be longer than 63 characters.
			// if labelLen > 63 {
			// 	return false
			// }
			lastDot = i
		}
	}

	return true
}

```

// === FILE: references/go/src/crypto/x509/pem_decrypt.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

// RFC 1423 describes the encryption of PEM blocks. The algorithm used to
// generate a key from the password was derived by looking at the OpenSSL
// implementation.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/md5"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"io"
	"strings"
)

type PEMCipher int

// Possible values for the EncryptPEMBlock encryption algorithm.
const (
	_ PEMCipher = iota
	PEMCipherDES
	PEMCipher3DES
	PEMCipherAES128
	PEMCipherAES192
	PEMCipherAES256
)

// rfc1423Algo holds a method for enciphering a PEM block.
type rfc1423Algo struct {
	cipher     PEMCipher
	name       string
	cipherFunc func(key []byte) (cipher.Block, error)
	keySize    int
	blockSize  int
}

// rfc1423Algos holds a slice of the possible ways to encrypt a PEM
// block. The ivSize numbers were taken from the OpenSSL source.
var rfc1423Algos = []rfc1423Algo{{
	cipher:     PEMCipherDES,
	name:       "DES-CBC",
	cipherFunc: des.NewCipher,
	keySize:    8,
	blockSize:  des.BlockSize,
}, {
	cipher:     PEMCipher3DES,
	name:       "DES-EDE3-CBC",
	cipherFunc: des.NewTripleDESCipher,
	keySize:    24,
	blockSize:  des.BlockSize,
}, {
	cipher:     PEMCipherAES128,
	name:       "AES-128-CBC",
	cipherFunc: aes.NewCipher,
	keySize:    16,
	blockSize:  aes.BlockSize,
}, {
	cipher:     PEMCipherAES192,
	name:       "AES-192-CBC",
	cipherFunc: aes.NewCipher,
	keySize:    24,
	blockSize:  aes.BlockSize,
}, {
	cipher:     PEMCipherAES256,
	name:       "AES-256-CBC",
	cipherFunc: aes.NewCipher,
	keySize:    32,
	blockSize:  aes.BlockSize,
},
}

// deriveKey uses a key derivation function to stretch the password into a key
// with the number of bits our cipher requires. This algorithm was derived from
// the OpenSSL source.
func (c rfc1423Algo) deriveKey(password, salt []byte) []byte {
	hash := md5.New()
	out := make([]byte, c.keySize)
	var digest []byte

	for i := 0; i < len(out); i += len(digest) {
		hash.Reset()
		hash.Write(digest)
		hash.Write(password)
		hash.Write(salt)
		digest = hash.Sum(digest[:0])
		copy(out[i:], digest)
	}
	return out
}

// IsEncryptedPEMBlock returns whether the PEM block is password encrypted
// according to RFC 1423.
//
// Deprecated: Legacy PEM encryption as specified in RFC 1423 is insecure by
// design. Since it does not authenticate the ciphertext, it is vulnerable to
// padding oracle attacks that can let an attacker recover the plaintext.
func IsEncryptedPEMBlock(b *pem.Block) bool {
	_, ok := b.Headers["DEK-Info"]
	return ok
}

// IncorrectPasswordError is returned when an incorrect password is detected.
var IncorrectPasswordError = errors.New("x509: decryption password incorrect")

// DecryptPEMBlock takes a PEM block encrypted according to RFC 1423 and the
// password used to encrypt it and returns a slice of decrypted DER encoded
// bytes. It inspects the DEK-Info header to determine the algorithm used for
// decryption. If no DEK-Info header is present, an error is returned. If an
// incorrect password is detected an [IncorrectPasswordError] is returned. Because
// of deficiencies in the format, it's not always possible to detect an
// incorrect password. In these cases no error will be returned but the
// decrypted DER bytes will be random noise.
//
// Deprecated: Legacy PEM encryption as specified in RFC 1423 is insecure by
// design. Since it does not authenticate the ciphertext, it is vulnerable to
// padding oracle attacks that can let an attacker recover the plaintext.
func DecryptPEMBlock(b *pem.Block, password []byte) ([]byte, error) {
	dek, ok := b.Headers["DEK-Info"]
	if !ok {
		return nil, errors.New("x509: no DEK-Info header in block")
	}

	mode, hexIV, ok := strings.Cut(dek, ",")
	if !ok {
		return nil, errors.New("x509: malformed DEK-Info header")
	}

	ciph := cipherByName(mode)
	if ciph == nil {
		return nil, errors.New("x509: unknown encryption mode")
	}
	iv, err := hex.DecodeString(hexIV)
	if err != nil {
		return nil, err
	}
	if len(iv) != ciph.blockSize {
		return nil, errors.New("x509: incorrect IV size")
	}

	// Based on the OpenSSL implementation. The salt is the first 8 bytes
	// of the initialization vector.
	key := ciph.deriveKey(password, iv[:8])
	block, err := ciph.cipherFunc(key)
	if err != nil {
		return nil, err
	}

	if len(b.Bytes)%block.BlockSize() != 0 {
		return nil, errors.New("x509: encrypted PEM data is not a multiple of the block size")
	}

	data := make([]byte, len(b.Bytes))
	dec := cipher.NewCBCDecrypter(block, iv)
	dec.CryptBlocks(data, b.Bytes)

	// Blocks are padded using a scheme where the last n bytes of padding are all
	// equal to n. It can pad from 1 to blocksize bytes inclusive. See RFC 1423.
	// For example:
	//	[x y z 2 2]
	//	[x y 7 7 7 7 7 7 7]
	// If we detect a bad padding, we assume it is an invalid password.
	dlen := len(data)
	if dlen == 0 || dlen%ciph.blockSize != 0 {
		return nil, errors.New("x509: invalid padding")
	}
	last := int(data[dlen-1])
	if dlen < last {
		return nil, IncorrectPasswordError
	}
	if last == 0 || last > ciph.blockSize {
		return nil, IncorrectPasswordError
	}
	for _, val := range data[dlen-last:] {
		if int(val) != last {
			return nil, IncorrectPasswordError
		}
	}
	return data[:dlen-last], nil
}

// EncryptPEMBlock returns a PEM block of the specified type holding the
// given DER encoded data encrypted with the specified algorithm and
// password according to RFC 1423.
//
// Deprecated: Legacy PEM encryption as specified in RFC 1423 is insecure by
// design. Since it does not authenticate the ciphertext, it is vulnerable to
// padding oracle attacks that can let an attacker recover the plaintext.
func EncryptPEMBlock(rand io.Reader, blockType string, data, password []byte, alg PEMCipher) (*pem.Block, error) {
	ciph := cipherByKey(alg)
	if ciph == nil {
		return nil, errors.New("x509: unknown encryption mode")
	}
	iv := make([]byte, ciph.blockSize)
	if _, err := io.ReadFull(rand, iv); err != nil {
		return nil, errors.New("x509: cannot generate IV: " + err.Error())
	}
	// The salt is the first 8 bytes of the initialization vector,
	// matching the key derivation in DecryptPEMBlock.
	key := ciph.deriveKey(password, iv[:8])
	block, err := ciph.cipherFunc(key)
	if err != nil {
		return nil, err
	}
	enc := cipher.NewCBCEncrypter(block, iv)
	pad := ciph.blockSize - len(data)%ciph.blockSize
	encrypted := make([]byte, len(data), len(data)+pad)
	// We could save this copy by encrypting all the whole blocks in
	// the data separately, but it doesn't seem worth the additional
	// code.
	copy(encrypted, data)
	// See RFC 1423, Section 1.1.
	for i := 0; i < pad; i++ {
		encrypted = append(encrypted, byte(pad))
	}
	enc.CryptBlocks(encrypted, encrypted)

	return &pem.Block{
		Type: blockType,
		Headers: map[string]string{
			"Proc-Type": "4,ENCRYPTED",
			"DEK-Info":  ciph.name + "," + hex.EncodeToString(iv),
		},
		Bytes: encrypted,
	}, nil
}

func cipherByName(name string) *rfc1423Algo {
	for i := range rfc1423Algos {
		alg := &rfc1423Algos[i]
		if alg.name == name {
			return alg
		}
	}
	return nil
}

func cipherByKey(key PEMCipher) *rfc1423Algo {
	for i := range rfc1423Algos {
		alg := &rfc1423Algos[i]
		if alg.cipher == key {
			return alg
		}
	}
	return nil
}

```

// === FILE: references/go/src/crypto/x509/pkcs1.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"crypto/rsa"
	"encoding/asn1"
	"errors"
	"internal/godebug"
	"math/big"
)

// pkcs1PrivateKey is a structure which mirrors the PKCS #1 ASN.1 for an RSA private key.
type pkcs1PrivateKey struct {
	Version int
	N       *big.Int
	E       int
	D       *big.Int
	P       *big.Int
	Q       *big.Int
	Dp      *big.Int `asn1:"optional"`
	Dq      *big.Int `asn1:"optional"`
	Qinv    *big.Int `asn1:"optional"`

	AdditionalPrimes []pkcs1AdditionalRSAPrime `asn1:"optional,omitempty"`
}

type pkcs1AdditionalRSAPrime struct {
	Prime *big.Int

	// We ignore these values because rsa will calculate them.
	Exp   *big.Int
	Coeff *big.Int
}

// pkcs1PublicKey reflects the ASN.1 structure of a PKCS #1 public key.
type pkcs1PublicKey struct {
	N *big.Int
	E int
}

// x509rsacrt, if zero, makes ParsePKCS1PrivateKey ignore and recompute invalid
// CRT values in the RSA private key.
var x509rsacrt = godebug.New("x509rsacrt")

// ParsePKCS1PrivateKey parses an [RSA] private key in PKCS #1, ASN.1 DER form.
//
// This kind of key is commonly encoded in PEM blocks of type "RSA PRIVATE KEY".
//
// Before Go 1.24, the CRT parameters were ignored and recomputed. To restore
// the old behavior, use the GODEBUG=x509rsacrt=0 environment variable.
func ParsePKCS1PrivateKey(der []byte) (*rsa.PrivateKey, error) {
	var priv pkcs1PrivateKey
	rest, err := asn1.Unmarshal(der, &priv)
	if len(rest) > 0 {
		return nil, asn1.SyntaxError{Msg: "trailing data"}
	}
	if err != nil {
		if _, err := asn1.Unmarshal(der, &ecPrivateKey{}); err == nil {
			return nil, errors.New("x509: failed to parse private key (use ParseECPrivateKey instead for this key format)")
		}
		if _, err := asn1.Unmarshal(der, &pkcs8{}); err == nil {
			return nil, errors.New("x509: failed to parse private key (use ParsePKCS8PrivateKey instead for this key format)")
		}
		return nil, err
	}

	if priv.Version > 1 {
		return nil, errors.New("x509: unsupported private key version")
	}

	if priv.N.Sign() <= 0 || priv.D.Sign() <= 0 || priv.P.Sign() <= 0 || priv.Q.Sign() <= 0 ||
		priv.Dp != nil && priv.Dp.Sign() <= 0 ||
		priv.Dq != nil && priv.Dq.Sign() <= 0 ||
		priv.Qinv != nil && priv.Qinv.Sign() <= 0 {
		return nil, errors.New("x509: private key contains zero or negative value")
	}

	key := new(rsa.PrivateKey)
	key.PublicKey = rsa.PublicKey{
		E: priv.E,
		N: priv.N,
	}

	key.D = priv.D
	key.Primes = make([]*big.Int, 2+len(priv.AdditionalPrimes))
	key.Primes[0] = priv.P
	key.Primes[1] = priv.Q
	key.Precomputed.Dp = priv.Dp
	key.Precomputed.Dq = priv.Dq
	key.Precomputed.Qinv = priv.Qinv
	for i, a := range priv.AdditionalPrimes {
		if a.Prime.Sign() <= 0 {
			return nil, errors.New("x509: private key contains zero or negative prime")
		}
		key.Primes[i+2] = a.Prime
		// We ignore the other two values because rsa will calculate
		// them as needed.
	}

	key.Precompute()
	if err := key.Validate(); err != nil {
		// If x509rsacrt=0 is set, try dropping the CRT values and
		// rerunning precomputation and key validation.
		if x509rsacrt.Value() == "0" {
			key.Precomputed.Dp = nil
			key.Precomputed.Dq = nil
			key.Precomputed.Qinv = nil
			key.Precompute()
			if err := key.Validate(); err == nil {
				x509rsacrt.IncNonDefault()
				return key, nil
			}
		}

		return nil, err
	}

	return key, nil
}

// MarshalPKCS1PrivateKey converts an [RSA] private key to PKCS #1, ASN.1 DER form.
//
// This kind of key is commonly encoded in PEM blocks of type "RSA PRIVATE KEY".
// For a more flexible key format which is not [RSA] specific, use
// [MarshalPKCS8PrivateKey].
//
// The key must have passed validation by calling [rsa.PrivateKey.Validate]
// first. MarshalPKCS1PrivateKey calls [rsa.PrivateKey.Precompute], which may
// modify the key if not already precomputed.
func MarshalPKCS1PrivateKey(key *rsa.PrivateKey) []byte {
	key.Precompute()

	version := 0
	if len(key.Primes) > 2 {
		version = 1
	}

	priv := pkcs1PrivateKey{
		Version: version,
		N:       key.N,
		E:       key.PublicKey.E,
		D:       key.D,
		P:       key.Primes[0],
		Q:       key.Primes[1],
		Dp:      key.Precomputed.Dp,
		Dq:      key.Precomputed.Dq,
		Qinv:    key.Precomputed.Qinv,
	}

	priv.AdditionalPrimes = make([]pkcs1AdditionalRSAPrime, len(key.Precomputed.CRTValues))
	for i, values := range key.Precomputed.CRTValues {
		priv.AdditionalPrimes[i].Prime = key.Primes[2+i]
		priv.AdditionalPrimes[i].Exp = values.Exp
		priv.AdditionalPrimes[i].Coeff = values.Coeff
	}

	b, _ := asn1.Marshal(priv)
	return b
}

// ParsePKCS1PublicKey parses an [RSA] public key in PKCS #1, ASN.1 DER form.
//
// This kind of key is commonly encoded in PEM blocks of type "RSA PUBLIC KEY".
func ParsePKCS1PublicKey(der []byte) (*rsa.PublicKey, error) {
	var pub pkcs1PublicKey
	rest, err := asn1.Unmarshal(der, &pub)
	if err != nil {
		if _, err := asn1.Unmarshal(der, &publicKeyInfo{}); err == nil {
			return nil, errors.New("x509: failed to parse public key (use ParsePKIXPublicKey instead for this key format)")
		}
		return nil, err
	}
	if len(rest) > 0 {
		return nil, asn1.SyntaxError{Msg: "trailing data"}
	}

	if pub.N.Sign() <= 0 || pub.E <= 0 {
		return nil, errors.New("x509: public key contains zero or negative value")
	}
	if pub.E > 1<<31-1 {
		return nil, errors.New("x509: public key contains large public exponent")
	}

	return &rsa.PublicKey{
		E: pub.E,
		N: pub.N,
	}, nil
}

// MarshalPKCS1PublicKey converts an [RSA] public key to PKCS #1, ASN.1 DER form.
//
// This kind of key is commonly encoded in PEM blocks of type "RSA PUBLIC KEY".
func MarshalPKCS1PublicKey(key *rsa.PublicKey) []byte {
	derBytes, _ := asn1.Marshal(pkcs1PublicKey{
		N: key.N,
		E: key.E,
	})
	return derBytes
}

```

// === FILE: references/go/src/crypto/x509/pkcs8.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/mldsa"
	"crypto/rsa"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"fmt"
)

// pkcs8 reflects an ASN.1, PKCS #8 PrivateKey. See
// ftp://ftp.rsasecurity.com/pub/pkcs/pkcs-8/pkcs-8v1_2.asn
// and RFC 5208.
type pkcs8 struct {
	Version    int
	Algo       pkix.AlgorithmIdentifier
	PrivateKey []byte
	// optional attributes omitted.
}

// ParsePKCS8PrivateKey parses an unencrypted private key in PKCS #8, ASN.1 DER form.
//
// It returns a *[rsa.PrivateKey], an *[ecdsa.PrivateKey], an [ed25519.PrivateKey] (not
// a pointer), a *[mldsa.PrivateKey], or an *[ecdh.PrivateKey] (for X25519).
// More types might be supported in the future.
//
// This kind of key is commonly encoded in PEM blocks of type "PRIVATE KEY".
//
// Before Go 1.24, the CRT parameters of RSA keys were ignored and recomputed.
// To restore the old behavior, use the GODEBUG=x509rsacrt=0 environment variable.
func ParsePKCS8PrivateKey(der []byte) (key any, err error) {
	var privKey pkcs8
	if _, err := asn1.Unmarshal(der, &privKey); err != nil {
		if _, err := asn1.Unmarshal(der, &ecPrivateKey{}); err == nil {
			return nil, errors.New("x509: failed to parse private key (use ParseECPrivateKey instead for this key format)")
		}
		if _, err := asn1.Unmarshal(der, &pkcs1PrivateKey{}); err == nil {
			return nil, errors.New("x509: failed to parse private key (use ParsePKCS1PrivateKey instead for this key format)")
		}
		return nil, err
	}
	switch {
	case privKey.Algo.Algorithm.Equal(oidPublicKeyRSA):
		key, err = ParsePKCS1PrivateKey(privKey.PrivateKey)
		if err != nil {
			return nil, errors.New("x509: failed to parse RSA private key embedded in PKCS#8: " + err.Error())
		}
		return key, nil

	case privKey.Algo.Algorithm.Equal(oidPublicKeyECDSA):
		bytes := privKey.Algo.Parameters.FullBytes
		namedCurveOID := new(asn1.ObjectIdentifier)
		if _, err := asn1.Unmarshal(bytes, namedCurveOID); err != nil {
			namedCurveOID = nil
		}
		key, err = parseECPrivateKey(namedCurveOID, privKey.PrivateKey)
		if err != nil {
			return nil, errors.New("x509: failed to parse EC private key embedded in PKCS#8: " + err.Error())
		}
		return key, nil

	case privKey.Algo.Algorithm.Equal(oidPublicKeyEd25519):
		if l := len(privKey.Algo.Parameters.FullBytes); l != 0 {
			return nil, errors.New("x509: invalid Ed25519 private key parameters")
		}
		var curvePrivateKey []byte
		if _, err := asn1.Unmarshal(privKey.PrivateKey, &curvePrivateKey); err != nil {
			return nil, fmt.Errorf("x509: invalid Ed25519 private key: %v", err)
		}
		if l := len(curvePrivateKey); l != ed25519.SeedSize {
			return nil, fmt.Errorf("x509: invalid Ed25519 private key length: %d", l)
		}
		return ed25519.NewKeyFromSeed(curvePrivateKey), nil

	case privKey.Algo.Algorithm.Equal(oidPublicKeyMLDSA44),
		privKey.Algo.Algorithm.Equal(oidPublicKeyMLDSA65),
		privKey.Algo.Algorithm.Equal(oidPublicKeyMLDSA87):
		if l := len(privKey.Algo.Parameters.FullBytes); l != 0 {
			return nil, errors.New("x509: invalid ML-DSA private key parameters")
		}
		if l := len(privKey.PrivateKey); l == 0 {
			return nil, fmt.Errorf("x509: invalid ML-DSA private key length: %d", l)
		}
		switch privKey.PrivateKey[0] {
		case 0x80: // IMPLICIT [0] OCTET STRING (seed)
		case 0x04: // OCTET STRING (expandedKey)
			return nil, errors.New("x509: semi-expanded ML-DSA private keys without seed are not supported")
		case 0x30: // SEQUENCE (both)
			return nil, errors.New(`x509: ML-DSA private keys with both seed and expanded key are not supported, use e.g. "openssl pkey -provparam ml-dsa.output_formats=seed-only" to convert to a seed-only key`)
		default:
			return nil, fmt.Errorf("x509: invalid ML-DSA private key: invalid ASN.1 tag %02x", privKey.PrivateKey[0])
		}
		if l := len(privKey.PrivateKey); l != 2+mldsa.PrivateKeySize {
			return nil, fmt.Errorf("x509: invalid ML-DSA private key length: %d", l)
		}
		if privKey.PrivateKey[1] != mldsa.PrivateKeySize {
			return nil, fmt.Errorf("x509: invalid ML-DSA private key ASN.1 encoding")
		}
		params, ok := mldsaParametersFromOID(privKey.Algo.Algorithm)
		if !ok {
			return nil, errors.New("x509: unknown ML-DSA parameters")
		}
		return mldsa.NewPrivateKey(params, privKey.PrivateKey[2:])

	case privKey.Algo.Algorithm.Equal(oidPublicKeyX25519):
		if l := len(privKey.Algo.Parameters.FullBytes); l != 0 {
			return nil, errors.New("x509: invalid X25519 private key parameters")
		}
		var curvePrivateKey []byte
		if _, err := asn1.Unmarshal(privKey.PrivateKey, &curvePrivateKey); err != nil {
			return nil, fmt.Errorf("x509: invalid X25519 private key: %v", err)
		}
		return ecdh.X25519().NewPrivateKey(curvePrivateKey)

	default:
		return nil, fmt.Errorf("x509: PKCS#8 wrapping contained private key with unknown algorithm: %v", privKey.Algo.Algorithm)
	}
}

// MarshalPKCS8PrivateKey converts a private key to PKCS #8, ASN.1 DER form.
//
// The following key types are currently supported: *[rsa.PrivateKey],
// *[ecdsa.PrivateKey], [ed25519.PrivateKey] (not a pointer), *[mldsa.PrivateKey],
// and *[ecdh.PrivateKey]. Unsupported key types result in an error.
//
// This kind of key is commonly encoded in PEM blocks of type "PRIVATE KEY".
//
// MarshalPKCS8PrivateKey runs [rsa.PrivateKey.Precompute] on RSA keys.
func MarshalPKCS8PrivateKey(key any) ([]byte, error) {
	var privKey pkcs8

	switch k := key.(type) {
	case *rsa.PrivateKey:
		privKey.Algo = pkix.AlgorithmIdentifier{
			Algorithm:  oidPublicKeyRSA,
			Parameters: asn1.NullRawValue,
		}
		k.Precompute()
		if err := k.Validate(); err != nil {
			return nil, err
		}
		privKey.PrivateKey = MarshalPKCS1PrivateKey(k)

	case *ecdsa.PrivateKey:
		oid, ok := oidFromNamedCurve(k.Curve)
		if !ok {
			return nil, errors.New("x509: unknown curve while marshaling to PKCS#8")
		}
		oidBytes, err := asn1.Marshal(oid)
		if err != nil {
			return nil, errors.New("x509: failed to marshal curve OID: " + err.Error())
		}
		privKey.Algo = pkix.AlgorithmIdentifier{
			Algorithm: oidPublicKeyECDSA,
			Parameters: asn1.RawValue{
				FullBytes: oidBytes,
			},
		}
		if privKey.PrivateKey, err = marshalECPrivateKeyWithOID(k, nil); err != nil {
			return nil, errors.New("x509: failed to marshal EC private key while building PKCS#8: " + err.Error())
		}

	case ed25519.PrivateKey:
		privKey.Algo = pkix.AlgorithmIdentifier{
			Algorithm: oidPublicKeyEd25519,
		}
		curvePrivateKey, err := asn1.Marshal(k.Seed())
		if err != nil {
			return nil, fmt.Errorf("x509: failed to marshal private key: %v", err)
		}
		privKey.PrivateKey = curvePrivateKey

	case *mldsa.PrivateKey:
		oid, ok := oidFromMLDSAParameters(k.PublicKey().Parameters())
		if !ok {
			return nil, errors.New("x509: unknown ML-DSA parameters while marshaling to PKCS#8")
		}
		privKey.Algo = pkix.AlgorithmIdentifier{
			Algorithm: oid,
		}
		privKey.PrivateKey = append([]byte{0x80, mldsa.PrivateKeySize}, k.Bytes()...)

	case *ecdh.PrivateKey:
		if k.Curve() == ecdh.X25519() {
			privKey.Algo = pkix.AlgorithmIdentifier{
				Algorithm: oidPublicKeyX25519,
			}
			var err error
			if privKey.PrivateKey, err = asn1.Marshal(k.Bytes()); err != nil {
				return nil, fmt.Errorf("x509: failed to marshal private key: %v", err)
			}
		} else {
			oid, ok := oidFromECDHCurve(k.Curve())
			if !ok {
				return nil, errors.New("x509: unknown curve while marshaling to PKCS#8")
			}
			oidBytes, err := asn1.Marshal(oid)
			if err != nil {
				return nil, errors.New("x509: failed to marshal curve OID: " + err.Error())
			}
			privKey.Algo = pkix.AlgorithmIdentifier{
				Algorithm: oidPublicKeyECDSA,
				Parameters: asn1.RawValue{
					FullBytes: oidBytes,
				},
			}
			if privKey.PrivateKey, err = marshalECDHPrivateKey(k); err != nil {
				return nil, errors.New("x509: failed to marshal EC private key while building PKCS#8: " + err.Error())
			}
		}

	default:
		return nil, fmt.Errorf("x509: unknown key type while marshaling PKCS#8: %T", key)
	}

	return asn1.Marshal(privKey)
}

```

// === FILE: references/go/src/crypto/x509/pkix/pkix.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pkix contains shared, low level structures used for ASN.1 parsing
// and serialization of X.509 certificates, CRL and OCSP.
package pkix

import (
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// AlgorithmIdentifier represents the ASN.1 structure of the same name. See RFC
// 5280, section 4.1.1.2.
type AlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type RDNSequence []RelativeDistinguishedNameSET

var attributeTypeNames = map[string]string{
	"2.5.4.6":  "C",
	"2.5.4.10": "O",
	"2.5.4.11": "OU",
	"2.5.4.3":  "CN",
	"2.5.4.5":  "SERIALNUMBER",
	"2.5.4.7":  "L",
	"2.5.4.8":  "ST",
	"2.5.4.9":  "STREET",
	"2.5.4.17": "POSTALCODE",
}

// String returns a string representation of the sequence r,
// roughly following the RFC 2253 Distinguished Names syntax.
func (r RDNSequence) String() string {
	var buf strings.Builder
	for i := 0; i < len(r); i++ {
		rdn := r[len(r)-1-i]
		if i > 0 {
			buf.WriteByte(',')
		}
		for j, tv := range rdn {
			if j > 0 {
				buf.WriteByte('+')
			}

			oidString := tv.Type.String()
			typeName, ok := attributeTypeNames[oidString]
			if !ok {
				// RFC 2253 §2.4: if the value's ASN.1 type has a string
				// representation, render it as a string; otherwise hex-encode
				// the DER.
				if _, ok := tv.Value.(string); !ok {
					derBytes, err := asn1.Marshal(tv.Value)
					if err == nil {
						buf.WriteString(oidString)
						buf.WriteString("=#")
						buf.WriteString(hex.EncodeToString(derBytes))
						continue // No value escaping necessary.
					}
				}

				typeName = oidString
			}

			valueString := fmt.Sprint(tv.Value)
			escaped := make([]rune, 0, len(valueString))

			for k, c := range valueString {
				escape := false

				switch c {
				case ',', '+', '"', '\\', '<', '>', ';':
					escape = true

				case ' ':
					escape = k == 0 || k == len(valueString)-1

				case '#':
					escape = k == 0
				}

				if escape {
					escaped = append(escaped, '\\', c)
				} else {
					escaped = append(escaped, c)
				}
			}

			buf.WriteString(typeName)
			buf.WriteByte('=')
			buf.WriteString(string(escaped))
		}
	}

	return buf.String()
}

type RelativeDistinguishedNameSET []AttributeTypeAndValue

// AttributeTypeAndValue mirrors the ASN.1 structure of the same name in
// RFC 5280, Section 4.1.2.4.
//
// When parsed as part of a pkix.Name structure in a crypto/x509 type,
// the Value will be
//
//   - a string if the ASN.1 type is PrintableString, IA5String,
//     NumericString, BMPString, T61String, or UTF8String;
//   - an int64 if the ASN.1 type is INTEGER;
//   - an asn1.BitString if the ASN.1 type is BIT STRING;
//   - a []byte if the ASN.1 type is OCTET STRING;
//   - an asn1.ObjectIdentifier if the ASN.1 type is OBJECT IDENTIFIER;
//   - a time.Time if the ASN.1 type is UTCTIME or GENERALIZEDTIME;
//   - a bool if the ASN.1 type is BOOLEAN;
//   - nil if the ASN.1 type is NULL;
//   - an asn1.RawValue otherwise.
type AttributeTypeAndValue struct {
	Type  asn1.ObjectIdentifier
	Value any
}

// AttributeTypeAndValueSET represents a set of ASN.1 sequences of
// [AttributeTypeAndValue] sequences from RFC 2986 (PKCS #10).
type AttributeTypeAndValueSET struct {
	Type  asn1.ObjectIdentifier
	Value [][]AttributeTypeAndValue `asn1:"set"`
}

// Extension represents the ASN.1 structure of the same name. See RFC
// 5280, section 4.2.
type Extension struct {
	Id       asn1.ObjectIdentifier
	Critical bool `asn1:"optional"`
	Value    []byte
}

// Name represents an X.509 distinguished name. This only includes the common
// elements of a DN. Note that Name is only an approximation of the X.509
// structure. If an accurate representation is needed, asn1.Unmarshal the raw
// subject or issuer as an [RDNSequence].
type Name struct {
	Country, Organization, OrganizationalUnit []string
	Locality, Province                        []string
	StreetAddress, PostalCode                 []string
	SerialNumber, CommonName                  string

	// Names contains all parsed attributes. When parsing distinguished names,
	// this can be used to extract non-standard attributes that are not parsed
	// by this package. When marshaling to RDNSequences, the Names field is
	// ignored, see ExtraNames.
	Names []AttributeTypeAndValue

	// ExtraNames contains attributes to be copied, raw, into any marshaled
	// distinguished names. Values override any attributes with the same OID.
	// The ExtraNames field is not populated when parsing, see Names.
	ExtraNames []AttributeTypeAndValue
}

// FillFromRDNSequence populates n from the provided [RDNSequence].
// Multi-entry RDNs are flattened, all entries are added to the
// relevant n fields, and the grouping is not preserved.
func (n *Name) FillFromRDNSequence(rdns *RDNSequence) {
	for _, rdn := range *rdns {
		if len(rdn) == 0 {
			continue
		}

		for _, atv := range rdn {
			n.Names = append(n.Names, atv)
			value, ok := atv.Value.(string)
			if !ok {
				continue
			}

			t := atv.Type
			if len(t) == 4 && t[0] == 2 && t[1] == 5 && t[2] == 4 {
				switch t[3] {
				case 3:
					n.CommonName = value
				case 5:
					n.SerialNumber = value
				case 6:
					n.Country = append(n.Country, value)
				case 7:
					n.Locality = append(n.Locality, value)
				case 8:
					n.Province = append(n.Province, value)
				case 9:
					n.StreetAddress = append(n.StreetAddress, value)
				case 10:
					n.Organization = append(n.Organization, value)
				case 11:
					n.OrganizationalUnit = append(n.OrganizationalUnit, value)
				case 17:
					n.PostalCode = append(n.PostalCode, value)
				}
			}
		}
	}
}

var (
	oidCountry            = []int{2, 5, 4, 6}
	oidOrganization       = []int{2, 5, 4, 10}
	oidOrganizationalUnit = []int{2, 5, 4, 11}
	oidCommonName         = []int{2, 5, 4, 3}
	oidSerialNumber       = []int{2, 5, 4, 5}
	oidLocality           = []int{2, 5, 4, 7}
	oidProvince           = []int{2, 5, 4, 8}
	oidStreetAddress      = []int{2, 5, 4, 9}
	oidPostalCode         = []int{2, 5, 4, 17}
)

// appendRDNs appends a relativeDistinguishedNameSET to the given RDNSequence
// and returns the new value. The relativeDistinguishedNameSET contains an
// attributeTypeAndValue for each of the given values. See RFC 5280, A.1, and
// search for AttributeTypeAndValue.
func (n Name) appendRDNs(in RDNSequence, values []string, oid asn1.ObjectIdentifier) RDNSequence {
	if len(values) == 0 || oidInAttributeTypeAndValue(oid, n.ExtraNames) {
		return in
	}

	s := make([]AttributeTypeAndValue, len(values))
	for i, value := range values {
		s[i].Type = oid
		s[i].Value = value
	}

	return append(in, s)
}

// ToRDNSequence converts n into a single [RDNSequence]. The following
// attributes are encoded as multi-value RDNs:
//
//   - Country
//   - Organization
//   - OrganizationalUnit
//   - Locality
//   - Province
//   - StreetAddress
//   - PostalCode
//
// Each ExtraNames entry is encoded as an individual RDN.
func (n Name) ToRDNSequence() (ret RDNSequence) {
	ret = n.appendRDNs(ret, n.Country, oidCountry)
	ret = n.appendRDNs(ret, n.Province, oidProvince)
	ret = n.appendRDNs(ret, n.Locality, oidLocality)
	ret = n.appendRDNs(ret, n.StreetAddress, oidStreetAddress)
	ret = n.appendRDNs(ret, n.PostalCode, oidPostalCode)
	ret = n.appendRDNs(ret, n.Organization, oidOrganization)
	ret = n.appendRDNs(ret, n.OrganizationalUnit, oidOrganizationalUnit)
	if len(n.CommonName) > 0 {
		ret = n.appendRDNs(ret, []string{n.CommonName}, oidCommonName)
	}
	if len(n.SerialNumber) > 0 {
		ret = n.appendRDNs(ret, []string{n.SerialNumber}, oidSerialNumber)
	}
	for _, atv := range n.ExtraNames {
		ret = append(ret, []AttributeTypeAndValue{atv})
	}

	return ret
}

// String returns the string form of n, roughly following
// the RFC 2253 Distinguished Names syntax.
func (n Name) String() string {
	var rdns RDNSequence
	// If there are no ExtraNames, surface the parsed value (all entries in
	// Names) instead.
	if n.ExtraNames == nil {
		for _, atv := range n.Names {
			t := atv.Type
			if len(t) == 4 && t[0] == 2 && t[1] == 5 && t[2] == 4 {
				switch t[3] {
				case 3, 5, 6, 7, 8, 9, 10, 11, 17:
					// These attributes were already parsed into named fields.
					continue
				}
			}
			// Place non-standard parsed values at the beginning of the sequence
			// so they will be at the end of the string. See Issue 39924.
			rdns = append(rdns, []AttributeTypeAndValue{atv})
		}
	}
	rdns = append(rdns, n.ToRDNSequence()...)
	return rdns.String()
}

// oidInAttributeTypeAndValue reports whether a type with the given OID exists
// in atv.
func oidInAttributeTypeAndValue(oid asn1.ObjectIdentifier, atv []AttributeTypeAndValue) bool {
	for _, a := range atv {
		if a.Type.Equal(oid) {
			return true
		}
	}
	return false
}

// CertificateList represents the ASN.1 structure of the same name. See RFC
// 5280, section 5.1. Use Certificate.CheckCRLSignature to verify the
// signature.
//
// Deprecated: x509.RevocationList should be used instead.
type CertificateList struct {
	TBSCertList        TBSCertificateList
	SignatureAlgorithm AlgorithmIdentifier
	SignatureValue     asn1.BitString
}

// HasExpired reports whether certList should have been updated by now.
func (certList *CertificateList) HasExpired(now time.Time) bool {
	return !now.Before(certList.TBSCertList.NextUpdate)
}

// TBSCertificateList represents the ASN.1 structure of the same name. See RFC
// 5280, section 5.1.
//
// Deprecated: x509.RevocationList should be used instead.
type TBSCertificateList struct {
	Raw                 asn1.RawContent
	Version             int `asn1:"optional,default:0"`
	Signature           AlgorithmIdentifier
	Issuer              RDNSequence
	ThisUpdate          time.Time
	NextUpdate          time.Time            `asn1:"optional"`
	RevokedCertificates []RevokedCertificate `asn1:"optional"`
	Extensions          []Extension          `asn1:"tag:0,optional,explicit"`
}

// RevokedCertificate represents the ASN.1 structure of the same name. See RFC
// 5280, section 5.1.
type RevokedCertificate struct {
	SerialNumber   *big.Int
	RevocationTime time.Time
	Extensions     []Extension `asn1:"optional"`
}

```

// === FILE: references/go/src/crypto/x509/platform_root_cert.pem ===
```text
-----BEGIN CERTIFICATE-----
MIIB/DCCAaOgAwIBAgICIzEwCgYIKoZIzj0EAwIwLDEqMCgGA1UEAxMhR28gcGxh
dGZvcm0gdmVyaWZpZXIgdGVzdGluZyByb290MB4XDTIzMDUyNjE3NDQwMVoXDTI4
MDUyNDE4NDQwMVowLDEqMCgGA1UEAxMhR28gcGxhdGZvcm0gdmVyaWZpZXIgdGVz
dGluZyByb290MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE5dNQY4FY29i2g3xx
7FyH4XiZz0C0AM4uyPUsXCZNb7CsctHDLhLtzABWSfFz76j+oVhq+qKrwIHsLX+7
f6YTQqOBtDCBsTAOBgNVHQ8BAf8EBAMCAgQwEwYDVR0lBAwwCgYIKwYBBQUHAwEw
DwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUEJInRbtQR6xTUSwvtdAe9A4XHwQw
WgYDVR0eAQH/BFAwTqAaMBiCFnRlc3RpbmcuZ29sYW5nLmludmFsaWShMDAKhwgA
AAAAAAAAADAihyAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADAKBggq
hkjOPQQDAgNHADBEAiBgzgLyQm4rK1AuIcElH3MdRqlteq3nzZCxKOI4xHXYjQIg
BCSzaCb1+/AK+mhRubrdebFYlUdveTH98wAfKQHaw64=
-----END CERTIFICATE-----

```

// === FILE: references/go/src/crypto/x509/platform_root_key.pem ===
```text
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIHhv8LVzb9gqJzAY0P442+FW0oqbfBrLnfqxyyAujOFSoAoGCCqGSM49
AwEHoUQDQgAE5dNQY4FY29i2g3xx7FyH4XiZz0C0AM4uyPUsXCZNb7CsctHDLhLt
zABWSfFz76j+oVhq+qKrwIHsLX+7f6YTQg==
-----END EC PRIVATE KEY-----

```

// === FILE: references/go/src/crypto/x509/root.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"internal/godebug"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	_ "unsafe" // for linkname
)

// systemRoots should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/breml/rootcerts
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname systemRoots
var (
	once             sync.Once
	systemRootsMu    sync.RWMutex
	systemRoots      *CertPool
	systemRootsErr   error
	fallbacksSet     bool
	useFallbackRoots bool
)

func systemRootsPool() *CertPool {
	once.Do(initSystemRoots)
	systemRootsMu.RLock()
	defer systemRootsMu.RUnlock()
	return systemRoots
}

func initSystemRoots() {
	systemRootsMu.Lock()
	defer systemRootsMu.Unlock()

	fallbackRoots := systemRoots
	systemRoots, systemRootsErr = loadSystemRoots()
	if systemRootsErr != nil {
		systemRoots = nil
	}

	if fallbackRoots == nil {
		return // no fallbacks to try
	}

	systemCertsAvail := systemRoots != nil && (systemRoots.len() > 0 || systemRoots.systemPool)

	if !useFallbackRoots && systemCertsAvail {
		return
	}

	if useFallbackRoots && systemCertsAvail {
		x509usefallbackroots.IncNonDefault() // overriding system certs with fallback certs.
	}

	systemRoots, systemRootsErr = fallbackRoots, nil
}

var x509usefallbackroots = godebug.New("x509usefallbackroots")

// SetFallbackRoots sets the roots to use during certificate verification, if no
// custom roots are specified and a platform verifier or a system certificate
// pool is not available (for instance in a container which does not have a root
// certificate bundle). SetFallbackRoots will panic if roots is nil.
//
// SetFallbackRoots may only be called once, if called multiple times it will
// panic.
//
// The fallback behavior can be forced on all platforms, even when there is a
// system certificate pool, by setting GODEBUG=x509usefallbackroots=1 (note that
// on Windows and macOS this will disable usage of the platform verification
// APIs and cause the pure Go verifier to be used). Setting
// x509usefallbackroots=1 without calling SetFallbackRoots has no effect.
func SetFallbackRoots(roots *CertPool) {
	if roots == nil {
		panic("roots must be non-nil")
	}

	systemRootsMu.Lock()
	defer systemRootsMu.Unlock()

	if fallbacksSet {
		panic("SetFallbackRoots has already been called")
	}
	fallbacksSet = true

	// Handle case when initSystemRoots was not yet executed.
	// We handle that specially instead of calling loadSystemRoots, to avoid
	// spending excessive amount of cpu here, since the SetFallbackRoots in most cases
	// is going to be called at program startup.
	if systemRoots == nil && systemRootsErr == nil {
		systemRoots = roots
		useFallbackRoots = x509usefallbackroots.Value() == "1"
		return
	}

	once.Do(func() { panic("unreachable") }) // asserts that system roots were indeed loaded before.

	forceFallbackRoots := x509usefallbackroots.Value() == "1"
	systemCertsAvail := systemRoots != nil && (systemRoots.len() > 0 || systemRoots.systemPool)

	if !forceFallbackRoots && systemCertsAvail {
		return
	}

	if forceFallbackRoots && systemCertsAvail {
		x509usefallbackroots.IncNonDefault() // overriding system certs with fallback certs.
	}

	systemRoots, systemRootsErr = roots, nil
}

const (
	// certFileEnv is the environment variable which identifies where to locate
	// the SSL certificate file. If set this overrides the system default.
	certFileEnv = "SSL_CERT_FILE"

	// certDirEnv is the environment variable which identifies which directory
	// to check for SSL certificate files. If set this overrides the system default.
	// See https://docs.openssl.org/4.0/man1/openssl-rehash/#environment.
	certDirEnv = "SSL_CERT_DIR"
)

var x509sslcertoverrideplatform = godebug.New("x509sslcertoverrideplatform")

func loadSystemRoots() (*CertPool, error) {
	certFilePath, certDirPath := os.Getenv(certFileEnv), os.Getenv(certDirEnv)

	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		if certFilePath == "" && certDirPath == "" {
			return &CertPool{systemPool: true}, nil
		}
		if x509sslcertoverrideplatform.Value() == "0" {
			x509sslcertoverrideplatform.IncNonDefault()
			return &CertPool{systemPool: true}, nil
		}
	}

	return loadOnDiskRoots(certFilePath, certDirPath)
}

func loadOnDiskRoots(certFilePath, certDirPath string) (*CertPool, error) {
	roots := NewCertPool()

	files := certFiles
	if certFilePath != "" {
		files = []string{certFilePath}
	}

	var firstErr error
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err == nil {
			roots.AppendCertsFromPEM(data)
			break
		}
		if firstErr == nil && !os.IsNotExist(err) {
			firstErr = err
		}
	}

	dirs := certDirectories
	if certDirPath != "" {
		// OpenSSL and BoringSSL both use ":" as the SSL_CERT_DIR separator on
		// Unix-like systems, and ";" on Windows.
		// See:
		//  * https://golang.org/issue/35325
		//  * https://docs.openssl.org/4.0/man1/openssl-rehash/#environment
		dirs = filepath.SplitList(certDirPath)
	}

	for _, directory := range dirs {
		fis, err := readUniqueDirectoryEntries(directory)
		if err != nil {
			if firstErr == nil && !os.IsNotExist(err) {
				firstErr = err
			}
			continue
		}
		for _, fi := range fis {
			data, err := os.ReadFile(filepath.Join(directory, fi.Name()))
			if err == nil {
				roots.AppendCertsFromPEM(data)
			}
		}
	}

	if roots.len() > 0 || firstErr == nil {
		return roots, nil
	}

	return nil, firstErr
}

// readUniqueDirectoryEntries is like os.ReadDir but omits
// symlinks that point within the directory.
func readUniqueDirectoryEntries(dir string) ([]fs.DirEntry, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	uniq := files[:0]
	for _, f := range files {
		if !isSameDirSymlink(f, dir) {
			uniq = append(uniq, f)
		}
	}
	return uniq, nil
}

// isSameDirSymlink reports whether f in dir is a symlink with a
// target not containing a slash.
func isSameDirSymlink(f fs.DirEntry, dir string) bool {
	if f.Type()&fs.ModeSymlink == 0 {
		return false
	}
	target, err := os.Readlink(filepath.Join(dir, f.Name()))
	return err == nil && !strings.ContainsRune(target, filepath.Separator)
}

```

// === FILE: references/go/src/crypto/x509/root_aix.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

// Possible certificate files; stop after finding one.
var certFiles = []string{
	"/var/ssl/certs/ca-bundle.crt",
}

// Possible directories with certificate files; all will be read.
var certDirectories = []string{
	"/var/ssl/certs",
}

```

// === FILE: references/go/src/crypto/x509/root_bsd.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build dragonfly || freebsd || netbsd || openbsd

package x509

// Possible certificate files; stop after finding one.
var certFiles = []string{
	"/usr/local/etc/ssl/cert.pem",            // FreeBSD
	"/etc/ssl/cert.pem",                      // OpenBSD
	"/usr/local/share/certs/ca-root-nss.crt", // DragonFly
	"/etc/openssl/certs/ca-certificates.crt", // NetBSD
}

// Possible directories with certificate files; all will be read.
var certDirectories = []string{
	"/etc/ssl/certs",         // FreeBSD 12.2+
	"/usr/local/share/certs", // FreeBSD
	"/etc/openssl/certs",     // NetBSD
}

```

// === FILE: references/go/src/crypto/x509/root_darwin.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"crypto/x509/internal/macos"
	"errors"
	"fmt"
)

// macOS has no default SSL_CERT_{FILE,DIR} paths.
var certFiles, certDirectories []string

func (c *Certificate) systemVerify(opts *VerifyOptions) (chains [][]*Certificate, err error) {
	certs := macos.CFArrayCreateMutable()
	defer macos.ReleaseCFArray(certs)
	leaf, err := macos.SecCertificateCreateWithData(c.Raw)
	if err != nil {
		return nil, errors.New("invalid leaf certificate")
	}
	macos.CFArrayAppendValue(certs, leaf)
	if opts.Intermediates != nil {
		for _, lc := range opts.Intermediates.lazyCerts {
			c, err := lc.getCert()
			if err != nil {
				return nil, err
			}
			sc, err := macos.SecCertificateCreateWithData(c.Raw)
			if err != nil {
				return nil, err
			}
			macos.CFArrayAppendValue(certs, sc)
		}
	}

	policies := macos.CFArrayCreateMutable()
	defer macos.ReleaseCFArray(policies)
	sslPolicy, err := macos.SecPolicyCreateSSL(opts.DNSName)
	if err != nil {
		return nil, err
	}
	macos.CFArrayAppendValue(policies, sslPolicy)

	trustObj, err := macos.SecTrustCreateWithCertificates(certs, policies)
	if err != nil {
		return nil, err
	}
	defer macos.CFRelease(trustObj)

	if !opts.CurrentTime.IsZero() {
		dateRef := macos.TimeToCFDateRef(opts.CurrentTime)
		defer macos.CFRelease(dateRef)
		if err := macos.SecTrustSetVerifyDate(trustObj, dateRef); err != nil {
			return nil, err
		}
	}

	// TODO(roland): we may want to allow passing in SCTs via VerifyOptions and
	// set them via SecTrustSetSignedCertificateTimestamps, since Apple will
	// always enforce its SCT requirements, and there are still _some_ people
	// using TLS or OCSP for that.

	if ret, err := macos.SecTrustEvaluateWithError(trustObj); err != nil {
		switch ret {
		case macos.ErrSecCertificateExpired:
			return nil, CertificateInvalidError{c, Expired, err.Error()}
		case macos.ErrSecHostNameMismatch:
			return nil, HostnameError{c, opts.DNSName}
		case macos.ErrSecNotTrusted:
			return nil, UnknownAuthorityError{Cert: c}
		default:
			return nil, fmt.Errorf("x509: %s", err)
		}
	}

	chain := [][]*Certificate{{}}
	chainRef, err := macos.SecTrustCopyCertificateChain(trustObj)
	if err != nil {
		return nil, err
	}
	defer macos.CFRelease(chainRef)
	for i := 0; i < macos.CFArrayGetCount(chainRef); i++ {
		certRef := macos.CFArrayGetValueAtIndex(chainRef, i)
		cert, err := exportCertificate(certRef)
		if err != nil {
			return nil, err
		}
		chain[0] = append(chain[0], cert)
	}
	if len(chain[0]) == 0 {
		// This should _never_ happen, but to be safe
		return nil, errors.New("x509: macos certificate verification internal error")
	}

	if opts.DNSName != "" {
		// If we have a DNS name, apply our own name verification
		if err := chain[0][0].VerifyHostname(opts.DNSName); err != nil {
			return nil, err
		}
	}

	keyUsages := opts.KeyUsages
	if len(keyUsages) == 0 {
		keyUsages = []ExtKeyUsage{ExtKeyUsageServerAuth}
	}

	// If any key usage is acceptable then we're done.
	for _, usage := range keyUsages {
		if usage == ExtKeyUsageAny {
			return chain, nil
		}
	}

	if !checkChainForKeyUsage(chain[0], keyUsages) {
		return nil, CertificateInvalidError{c, IncompatibleUsage, ""}
	}

	return chain, nil
}

// exportCertificate returns a *Certificate for a SecCertificateRef.
func exportCertificate(cert macos.CFRef) (*Certificate, error) {
	data, err := macos.SecCertificateCopyData(cert)
	if err != nil {
		return nil, err
	}
	return ParseCertificate(data)
}

```

// === FILE: references/go/src/crypto/x509/root_linux.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import "internal/goos"

// Possible certificate files; stop after finding one.
var certFiles = []string{
	"/etc/ssl/certs/ca-certificates.crt",                // Debian/Ubuntu/Gentoo etc.
	"/etc/pki/tls/certs/ca-bundle.crt",                  // Fedora/RHEL 6
	"/etc/ssl/ca-bundle.pem",                            // OpenSUSE
	"/etc/pki/tls/cacert.pem",                           // OpenELEC
	"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", // CentOS/RHEL 7
	"/etc/ssl/cert.pem",                                 // Alpine Linux
}

// Possible directories with certificate files; all will be read.
var certDirectories = []string{
	"/etc/ssl/certs",     // SLES10/SLES11, https://golang.org/issue/12139
	"/etc/pki/tls/certs", // Fedora/RHEL
}

func init() {
	if goos.IsAndroid == 1 {
		certDirectories = append(certDirectories,
			"/system/etc/security/cacerts",    // Android system roots
			"/data/misc/keychain/certs-added", // User trusted CA folder
		)
	}
}

```

// === FILE: references/go/src/crypto/x509/root_plan9.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build plan9

package x509

// Possible certificate files; stop after finding one.
var certFiles = []string{
	"/sys/lib/tls/ca.pem",
}

var certDirectories = []string{}

func (c *Certificate) systemVerify(opts *VerifyOptions) (chains [][]*Certificate, err error) {
	return nil, nil
}

```

// === FILE: references/go/src/crypto/x509/root_solaris.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

// Possible certificate files; stop after finding one.
var certFiles = []string{
	"/etc/certs/ca-certificates.crt",     // Solaris 11.2+
	"/etc/ssl/certs/ca-certificates.crt", // Joyent SmartOS
	"/etc/ssl/cacert.pem",                // OmniOS
}

// Possible directories with certificate files; all will be read.
var certDirectories = []string{
	"/etc/certs/CA",
}

```

// === FILE: references/go/src/crypto/x509/root_unix.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris || wasip1

package x509

func (c *Certificate) systemVerify(opts *VerifyOptions) (chains [][]*Certificate, err error) {
	return nil, nil
}

```

// === FILE: references/go/src/crypto/x509/root_wasm.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build wasm

package x509

// Possible certificate files; stop after finding one.
var certFiles = []string{}

// Possible directories with certificate files; all will be read.
var certDirectories = []string{}

```

// === FILE: references/go/src/crypto/x509/root_windows.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"bytes"
	"errors"
	"strings"
	"syscall"
	"unsafe"
)

// Windows has no default SSL_CERT_{FILE,DIR} paths.
var certFiles, certDirectories []string

// Creates a new *syscall.CertContext representing the leaf certificate in an in-memory
// certificate store containing itself and all of the intermediate certificates specified
// in the opts.Intermediates CertPool.
//
// A pointer to the in-memory store is available in the returned CertContext's Store field.
// The store is automatically freed when the CertContext is freed using
// syscall.CertFreeCertificateContext.
func createStoreContext(leaf *Certificate, opts *VerifyOptions) (*syscall.CertContext, error) {
	var storeCtx *syscall.CertContext

	leafCtx, err := syscall.CertCreateCertificateContext(syscall.X509_ASN_ENCODING|syscall.PKCS_7_ASN_ENCODING, &leaf.Raw[0], uint32(len(leaf.Raw)))
	if err != nil {
		return nil, err
	}
	defer syscall.CertFreeCertificateContext(leafCtx)

	handle, err := syscall.CertOpenStore(syscall.CERT_STORE_PROV_MEMORY, 0, 0, syscall.CERT_STORE_DEFER_CLOSE_UNTIL_LAST_FREE_FLAG, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CertCloseStore(handle, 0)

	err = syscall.CertAddCertificateContextToStore(handle, leafCtx, syscall.CERT_STORE_ADD_ALWAYS, &storeCtx)
	if err != nil {
		return nil, err
	}

	if opts.Intermediates != nil {
		for i := 0; i < opts.Intermediates.len(); i++ {
			intermediate, _, err := opts.Intermediates.cert(i)
			if err != nil {
				return nil, err
			}
			ctx, err := syscall.CertCreateCertificateContext(syscall.X509_ASN_ENCODING|syscall.PKCS_7_ASN_ENCODING, &intermediate.Raw[0], uint32(len(intermediate.Raw)))
			if err != nil {
				return nil, err
			}

			err = syscall.CertAddCertificateContextToStore(handle, ctx, syscall.CERT_STORE_ADD_ALWAYS, nil)
			syscall.CertFreeCertificateContext(ctx)
			if err != nil {
				return nil, err
			}
		}
	}

	return storeCtx, nil
}

// extractSimpleChain extracts the final certificate chain from a CertSimpleChain.
func extractSimpleChain(simpleChain **syscall.CertSimpleChain, count int) (chain []*Certificate, err error) {
	if simpleChain == nil || count == 0 {
		return nil, errors.New("x509: invalid simple chain")
	}

	simpleChains := unsafe.Slice(simpleChain, count)
	lastChain := simpleChains[count-1]
	elements := unsafe.Slice(lastChain.Elements, lastChain.NumElements)
	for i := 0; i < int(lastChain.NumElements); i++ {
		// Copy the buf, since ParseCertificate does not create its own copy.
		cert := elements[i].CertContext
		encodedCert := unsafe.Slice(cert.EncodedCert, cert.Length)
		buf := bytes.Clone(encodedCert)
		parsedCert, err := ParseCertificate(buf)
		if err != nil {
			return nil, err
		}
		chain = append(chain, parsedCert)
	}

	return chain, nil
}

// checkChainTrustStatus checks the trust status of the certificate chain, translating
// any errors it finds into Go errors in the process.
func checkChainTrustStatus(c *Certificate, chainCtx *syscall.CertChainContext) error {
	if chainCtx.TrustStatus.ErrorStatus != syscall.CERT_TRUST_NO_ERROR {
		status := chainCtx.TrustStatus.ErrorStatus
		switch status {
		case syscall.CERT_TRUST_IS_NOT_TIME_VALID:
			return CertificateInvalidError{c, Expired, ""}
		case syscall.CERT_TRUST_IS_NOT_VALID_FOR_USAGE:
			return CertificateInvalidError{c, IncompatibleUsage, ""}
		// TODO(filippo): surface more error statuses.
		default:
			return UnknownAuthorityError{c, nil, nil}
		}
	}
	return nil
}

// checkChainSSLServerPolicy checks that the certificate chain in chainCtx is valid for
// use as a certificate chain for a SSL/TLS server.
func checkChainSSLServerPolicy(c *Certificate, chainCtx *syscall.CertChainContext, opts *VerifyOptions) error {
	servernamep, err := syscall.UTF16PtrFromString(strings.TrimSuffix(opts.DNSName, "."))
	if err != nil {
		return err
	}
	sslPara := &syscall.SSLExtraCertChainPolicyPara{
		AuthType:   syscall.AUTHTYPE_SERVER,
		ServerName: servernamep,
	}
	sslPara.Size = uint32(unsafe.Sizeof(*sslPara))

	para := &syscall.CertChainPolicyPara{
		ExtraPolicyPara: (syscall.Pointer)(unsafe.Pointer(sslPara)),
	}
	para.Size = uint32(unsafe.Sizeof(*para))

	status := syscall.CertChainPolicyStatus{}
	err = syscall.CertVerifyCertificateChainPolicy(syscall.CERT_CHAIN_POLICY_SSL, chainCtx, para, &status)
	if err != nil {
		return err
	}

	// TODO(mkrautz): use the lChainIndex and lElementIndex fields
	// of the CertChainPolicyStatus to provide proper context, instead
	// using c.
	if status.Error != 0 {
		switch status.Error {
		case syscall.CERT_E_EXPIRED:
			return CertificateInvalidError{c, Expired, ""}
		case syscall.CERT_E_CN_NO_MATCH:
			return HostnameError{c, opts.DNSName}
		case syscall.CERT_E_UNTRUSTEDROOT:
			return UnknownAuthorityError{c, nil, nil}
		default:
			return UnknownAuthorityError{c, nil, nil}
		}
	}

	return nil
}

// windowsExtKeyUsageOIDs are the C NUL-terminated string representations of the
// OIDs for use with the Windows API.
var windowsExtKeyUsageOIDs = make(map[ExtKeyUsage][]byte, len(extKeyUsageOIDs))

func init() {
	for _, eku := range extKeyUsageOIDs {
		windowsExtKeyUsageOIDs[eku.extKeyUsage] = []byte(eku.oid.String() + "\x00")
	}
}

func verifyChain(c *Certificate, chainCtx *syscall.CertChainContext, opts *VerifyOptions) (chain []*Certificate, err error) {
	err = checkChainTrustStatus(c, chainCtx)
	if err != nil {
		return nil, err
	}

	if opts != nil && len(opts.DNSName) > 0 {
		err = checkChainSSLServerPolicy(c, chainCtx, opts)
		if err != nil {
			return nil, err
		}
	}

	chain, err = extractSimpleChain(chainCtx.Chains, int(chainCtx.ChainCount))
	if err != nil {
		return nil, err
	}
	if len(chain) == 0 {
		return nil, errors.New("x509: internal error: system verifier returned an empty chain")
	}

	// Mitigate CVE-2020-0601, where the Windows system verifier might be
	// tricked into using custom curve parameters for a trusted root, by
	// double-checking all ECDSA signatures. If the system was tricked into
	// using spoofed parameters, the signature will be invalid for the correct
	// ones we parsed. (We don't support custom curves ourselves.)
	for i, parent := range chain[1:] {
		if parent.PublicKeyAlgorithm != ECDSA {
			continue
		}
		if err := parent.CheckSignature(chain[i].SignatureAlgorithm,
			chain[i].RawTBSCertificate, chain[i].Signature); err != nil {
			return nil, err
		}
	}
	return chain, nil
}

// systemVerify is like Verify, except that it uses CryptoAPI calls
// to build certificate chains and verify them.
func (c *Certificate) systemVerify(opts *VerifyOptions) (chains [][]*Certificate, err error) {
	storeCtx, err := createStoreContext(c, opts)
	if err != nil {
		return nil, err
	}
	defer syscall.CertFreeCertificateContext(storeCtx)

	para := new(syscall.CertChainPara)
	para.Size = uint32(unsafe.Sizeof(*para))

	keyUsages := opts.KeyUsages
	if len(keyUsages) == 0 {
		keyUsages = []ExtKeyUsage{ExtKeyUsageServerAuth}
	}
	oids := make([]*byte, 0, len(keyUsages))
	for _, eku := range keyUsages {
		if eku == ExtKeyUsageAny {
			oids = nil
			break
		}
		if oid, ok := windowsExtKeyUsageOIDs[eku]; ok {
			oids = append(oids, &oid[0])
		}
	}
	if oids != nil {
		para.RequestedUsage.Type = syscall.USAGE_MATCH_TYPE_OR
		para.RequestedUsage.Usage.Length = uint32(len(oids))
		para.RequestedUsage.Usage.UsageIdentifiers = &oids[0]
	} else {
		para.RequestedUsage.Type = syscall.USAGE_MATCH_TYPE_AND
		para.RequestedUsage.Usage.Length = 0
		para.RequestedUsage.Usage.UsageIdentifiers = nil
	}

	var verifyTime *syscall.Filetime
	if opts != nil && !opts.CurrentTime.IsZero() {
		ft := syscall.NsecToFiletime(opts.CurrentTime.UnixNano())
		verifyTime = &ft
	}

	// The default is to return only the highest quality chain,
	// setting this flag will add additional lower quality contexts.
	// These are returned in the LowerQualityChains field.
	const CERT_CHAIN_RETURN_LOWER_QUALITY_CONTEXTS = 0x00000080

	// CertGetCertificateChain will traverse Windows's root stores in an attempt to build a verified certificate chain
	var topCtx *syscall.CertChainContext
	err = syscall.CertGetCertificateChain(syscall.Handle(0), storeCtx, verifyTime, storeCtx.Store, para, CERT_CHAIN_RETURN_LOWER_QUALITY_CONTEXTS, 0, &topCtx)
	if err != nil {
		return nil, err
	}
	defer syscall.CertFreeCertificateChain(topCtx)

	chain, topErr := verifyChain(c, topCtx, opts)
	if topErr == nil {
		chains = append(chains, chain)
	}

	if lqCtxCount := topCtx.LowerQualityChainCount; lqCtxCount > 0 {
		lqCtxs := unsafe.Slice(topCtx.LowerQualityChains, lqCtxCount)
		for _, ctx := range lqCtxs {
			chain, err := verifyChain(c, ctx, opts)
			if err == nil {
				chains = append(chains, chain)
			}
		}
	}

	if len(chains) == 0 {
		// Return the error from the highest quality context.
		return nil, topErr
	}

	return chains, nil
}

```

// === FILE: references/go/src/crypto/x509/sec1.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/asn1"
	"errors"
	"fmt"
)

const ecPrivKeyVersion = 1

// ecPrivateKey reflects an ASN.1 Elliptic Curve Private Key Structure.
// References:
//
//	RFC 5915
//	SEC1 - http://www.secg.org/sec1-v2.pdf
//
// Per RFC 5915 the NamedCurveOID is marked as ASN.1 OPTIONAL, however in
// most cases it is not.
type ecPrivateKey struct {
	Version       int
	PrivateKey    []byte
	NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
	PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
}

// ParseECPrivateKey parses an EC private key in SEC 1, ASN.1 DER form.
//
// This kind of key is commonly encoded in PEM blocks of type "EC PRIVATE KEY".
func ParseECPrivateKey(der []byte) (*ecdsa.PrivateKey, error) {
	return parseECPrivateKey(nil, der)
}

// MarshalECPrivateKey converts an EC private key to SEC 1, ASN.1 DER form.
//
// This kind of key is commonly encoded in PEM blocks of type "EC PRIVATE KEY".
// For a more flexible key format which is not EC specific, use
// [MarshalPKCS8PrivateKey].
func MarshalECPrivateKey(key *ecdsa.PrivateKey) ([]byte, error) {
	oid, ok := oidFromNamedCurve(key.Curve)
	if !ok {
		return nil, errors.New("x509: unknown elliptic curve")
	}

	return marshalECPrivateKeyWithOID(key, oid)
}

// marshalECPrivateKeyWithOID marshals an EC private key into ASN.1, DER format and
// sets the curve ID to the given OID, or omits it if OID is nil.
func marshalECPrivateKeyWithOID(key *ecdsa.PrivateKey, oid asn1.ObjectIdentifier) ([]byte, error) {
	privateKey, err := key.Bytes()
	if err != nil {
		return nil, err
	}
	publicKey, err := key.PublicKey.Bytes()
	if err != nil {
		return nil, err
	}
	return asn1.Marshal(ecPrivateKey{
		Version:       1,
		PrivateKey:    privateKey,
		NamedCurveOID: oid,
		PublicKey:     asn1.BitString{Bytes: publicKey},
	})
}

// marshalECDHPrivateKey marshals an EC private key into ASN.1, DER format
// suitable for NIST curves.
func marshalECDHPrivateKey(key *ecdh.PrivateKey) ([]byte, error) {
	return asn1.Marshal(ecPrivateKey{
		Version:    1,
		PrivateKey: key.Bytes(),
		PublicKey:  asn1.BitString{Bytes: key.PublicKey().Bytes()},
	})
}

// parseECPrivateKey parses an ASN.1 Elliptic Curve Private Key Structure.
// The OID for the named curve may be provided from another source (such as
// the PKCS8 container) - if it is provided then use this instead of the OID
// that may exist in the EC private key structure.
func parseECPrivateKey(namedCurveOID *asn1.ObjectIdentifier, der []byte) (key *ecdsa.PrivateKey, err error) {
	var privKey ecPrivateKey
	if _, err := asn1.Unmarshal(der, &privKey); err != nil {
		if _, err := asn1.Unmarshal(der, &pkcs8{}); err == nil {
			return nil, errors.New("x509: failed to parse private key (use ParsePKCS8PrivateKey instead for this key format)")
		}
		if _, err := asn1.Unmarshal(der, &pkcs1PrivateKey{}); err == nil {
			return nil, errors.New("x509: failed to parse private key (use ParsePKCS1PrivateKey instead for this key format)")
		}
		return nil, errors.New("x509: failed to parse EC private key: " + err.Error())
	}
	if privKey.Version != ecPrivKeyVersion {
		return nil, fmt.Errorf("x509: unknown EC private key version %d", privKey.Version)
	}

	var curve elliptic.Curve
	if namedCurveOID != nil {
		curve = namedCurveFromOID(*namedCurveOID)
	} else {
		curve = namedCurveFromOID(privKey.NamedCurveOID)
	}
	if curve == nil {
		return nil, errors.New("x509: unknown elliptic curve")
	}

	size := (curve.Params().N.BitLen() + 7) / 8
	privateKey := make([]byte, size)

	// Some private keys have leading zero padding. This is invalid
	// according to [SEC1], but this code will ignore it.
	for len(privKey.PrivateKey) > len(privateKey) {
		if privKey.PrivateKey[0] != 0 {
			return nil, errors.New("x509: invalid private key length")
		}
		privKey.PrivateKey = privKey.PrivateKey[1:]
	}

	// Some private keys remove all leading zeros, this is also invalid
	// according to [SEC1] but since OpenSSL used to do this, we ignore
	// this too.
	copy(privateKey[len(privateKey)-len(privKey.PrivateKey):], privKey.PrivateKey)

	return ecdsa.ParseRawPrivateKey(curve, privateKey)
}

```

// === FILE: references/go/src/crypto/x509/test-file.crt ===
```text
-----BEGIN CERTIFICATE-----
MIIFbTCCA1WgAwIBAgIJAN338vEmMtLsMA0GCSqGSIb3DQEBCwUAME0xCzAJBgNV
BAYTAlVLMRMwEQYDVQQIDApUZXN0LVN0YXRlMRUwEwYDVQQKDAxHb2xhbmcgVGVz
dHMxEjAQBgNVBAMMCXRlc3QtZmlsZTAeFw0xNzAyMDEyMzUyMDhaFw0yNzAxMzAy
MzUyMDhaME0xCzAJBgNVBAYTAlVLMRMwEQYDVQQIDApUZXN0LVN0YXRlMRUwEwYD
VQQKDAxHb2xhbmcgVGVzdHMxEjAQBgNVBAMMCXRlc3QtZmlsZTCCAiIwDQYJKoZI
hvcNAQEBBQADggIPADCCAgoCggIBAPMGiLjdiffQo3Xc8oUe7wsDhSaAJFOhO6Qs
i0xYrYl7jmCuz9rGD2fdgk5cLqGazKuQ6fIFzHXFU2BKs4CWXt9KO0KFEhfvZeuW
jG5d7C1ZUiuKOrPqjKVu8SZtFPc7y7Ke7msXzY+Z2LLyiJJ93LCMq4+cTSGNXVlI
KqUxhxeoD5/QkUPyQy/ilu3GMYfx/YORhDP6Edcuskfj8wRh1UxBejP8YPMvI6St
cE2GkxoEGqDWnQ/61F18te6WI3MD29tnKXOkXVhnSC+yvRLljotW2/tAhHKBG4tj
iQWT5Ri4Wrw2tXxPKRLsVWc7e1/hdxhnuvYpXkWNhKsm002jzkFXlzfEwPd8nZdw
5aT6gPUBN2AAzdoqZI7E200i0orEF7WaSoMfjU1tbHvExp3vyAPOfJ5PS2MQ6W03
Zsy5dTVH+OBH++rkRzQCFcnIv/OIhya5XZ9KX9nFPgBEP7Xq2A+IjH7B6VN/S/bv
8lhp2V+SQvlew9GttKC4hKuPsl5o7+CMbcqcNUdxm9gGkN8epGEKCuix97bpNlxN
fHZxHE5+8GMzPXMkCD56y5TNKR6ut7JGHMPtGl5lPCLqzG/HzYyFgxsDfDUu2B0A
GKj0lGpnLfGqwhs2/s3jpY7+pcvVQxEpvVTId5byDxu1ujP4HjO/VTQ2P72rE8Ft
C6J2Av0tAgMBAAGjUDBOMB0GA1UdDgQWBBTLT/RbyfBB/Pa07oBnaM+QSJPO9TAf
BgNVHSMEGDAWgBTLT/RbyfBB/Pa07oBnaM+QSJPO9TAMBgNVHRMEBTADAQH/MA0G
CSqGSIb3DQEBCwUAA4ICAQB3sCntCcQwhMgRPPyvOCMyTcQ/Iv+cpfxz2Ck14nlx
AkEAH2CH0ov5GWTt07/ur3aa5x+SAKi0J3wTD1cdiw4U/6Uin6jWGKKxvoo4IaeK
SbM8w/6eKx6UbmHx7PA/eRABY9tTlpdPCVgw7/o3WDr03QM+IAtatzvaCPPczake
pbdLwmBZB/v8V+6jUajy6jOgdSH0PyffGnt7MWgDETmNC6p/Xigp5eh+C8Fb4NGT
xgHES5PBC+sruWp4u22bJGDKTvYNdZHsnw/CaKQWNsQqwisxa3/8N5v+PCff/pxl
r05pE3PdHn9JrCl4iWdVlgtiI9BoPtQyDfa/OEFaScE8KYR8LxaAgdgp3zYncWls
BpwQ6Y/A2wIkhlD9eEp5Ib2hz7isXOs9UwjdriKqrBXqcIAE5M+YIk3+KAQKxAtd
4YsK3CSJ010uphr12YKqlScj4vuKFjuOtd5RyyMIxUG3lrrhAu2AzCeKCLdVgA8+
75FrYMApUdvcjp4uzbBoED4XRQlx9kdFHVbYgmE/+yddBYJM8u4YlgAL0hW2/D8p
z9JWIfxVmjJnBnXaKGBuiUyZ864A3PJndP6EMMo7TzS2CDnfCYuJjvI0KvDjFNmc
rQA04+qfMSEz3nmKhbbZu4eYLzlADhfH8tT4GMtXf71WLA5AUHGf2Y4+HIHTsmHG
vQ==
-----END CERTIFICATE-----

```

// === FILE: references/go/src/crypto/x509/verify.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509

import (
	"bytes"
	"crypto"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"iter"
	"maps"
	"net"
	"net/netip"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type InvalidReason int

const (
	// NotAuthorizedToSign results when a certificate is signed by another
	// which isn't marked as a CA certificate.
	NotAuthorizedToSign InvalidReason = iota
	// Expired results when a certificate has expired, based on the time
	// given in the VerifyOptions.
	Expired
	// CANotAuthorizedForThisName results when an intermediate or root
	// certificate has a name constraint which doesn't permit a DNS or
	// other name (including IP address) in the leaf certificate.
	CANotAuthorizedForThisName
	// TooManyIntermediates results when a path length constraint is
	// violated.
	TooManyIntermediates
	// IncompatibleUsage results when the certificate's key usage indicates
	// that it may only be used for a different purpose.
	IncompatibleUsage
	// NameMismatch results when the subject name of a parent certificate
	// does not match the issuer name in the child.
	NameMismatch
	// NameConstraintsWithoutSANs is a legacy error and is no longer returned.
	NameConstraintsWithoutSANs
	// UnconstrainedName results when a CA certificate contains permitted
	// name constraints, but leaf certificate contains a name of an
	// unsupported or unconstrained type.
	UnconstrainedName
	// TooManyConstraints results when the number of comparison operations
	// needed to check a certificate exceeds the limit set by
	// VerifyOptions.MaxConstraintComparisions. This limit exists to
	// prevent pathological certificates can consuming excessive amounts of
	// CPU time to verify.
	TooManyConstraints
	// CANotAuthorizedForExtKeyUsage results when an intermediate or root
	// certificate does not permit a requested extended key usage.
	CANotAuthorizedForExtKeyUsage
	// NoValidChains results when there are no valid chains to return.
	NoValidChains
)

// CertificateInvalidError results when an odd error occurs. Users of this
// library probably want to handle all these errors uniformly.
type CertificateInvalidError struct {
	Cert   *Certificate
	Reason InvalidReason
	Detail string
}

func (e CertificateInvalidError) Error() string {
	switch e.Reason {
	case NotAuthorizedToSign:
		return "x509: certificate is not authorized to sign other certificates"
	case Expired:
		return "x509: certificate has expired or is not yet valid: " + e.Detail
	case CANotAuthorizedForThisName:
		return "x509: a root or intermediate certificate is not authorized to sign for this name: " + e.Detail
	case CANotAuthorizedForExtKeyUsage:
		return "x509: a root or intermediate certificate is not authorized for an extended key usage: " + e.Detail
	case TooManyIntermediates:
		return "x509: too many intermediates for path length constraint"
	case IncompatibleUsage:
		return "x509: certificate specifies an incompatible key usage"
	case NameMismatch:
		return "x509: issuer name does not match subject from issuing certificate"
	case NameConstraintsWithoutSANs:
		return "x509: issuer has name constraints but leaf doesn't have a SAN extension"
	case UnconstrainedName:
		return "x509: issuer has name constraints but leaf contains unknown or unconstrained name: " + e.Detail
	case NoValidChains:
		s := "x509: no valid chains built"
		if e.Detail != "" {
			s = fmt.Sprintf("%s: %s", s, e.Detail)
		}
		return s
	}
	return "x509: unknown error"
}

// HostnameError results when the set of authorized names doesn't match the
// requested name.
type HostnameError struct {
	Certificate *Certificate
	Host        string
}

func (h HostnameError) Error() string {
	c := h.Certificate
	maxNamesIncluded := 100

	if !c.hasSANExtension() && matchHostnames(c.Subject.CommonName, splitHostname(h.Host)) {
		return "x509: certificate relies on legacy Common Name field, use SANs instead"
	}

	var valid strings.Builder
	if ip := net.ParseIP(h.Host); ip != nil {
		// Trying to validate an IP
		if len(c.IPAddresses) == 0 {
			return "x509: cannot validate certificate for " + h.Host + " because it doesn't contain any IP SANs"
		}
		if len(c.IPAddresses) >= maxNamesIncluded {
			return fmt.Sprintf("x509: certificate is valid for %d IP SANs, but none matched %s", len(c.IPAddresses), h.Host)
		}
		for _, san := range c.IPAddresses {
			if valid.Len() > 0 {
				valid.WriteString(", ")
			}
			valid.WriteString(san.String())
		}
	} else {
		if len(c.DNSNames) >= maxNamesIncluded {
			return fmt.Sprintf("x509: certificate is valid for %d names, but none matched %s", len(c.DNSNames), h.Host)
		}
		valid.WriteString(strings.Join(c.DNSNames, ", "))
	}

	if valid.Len() == 0 {
		return "x509: certificate is not valid for any names, but wanted to match " + h.Host
	}
	return "x509: certificate is valid for " + valid.String() + ", not " + h.Host
}

// UnknownAuthorityError results when the certificate issuer is unknown
type UnknownAuthorityError struct {
	Cert *Certificate
	// hintErr contains an error that may be helpful in determining why an
	// authority wasn't found.
	hintErr error
	// hintCert contains a possible authority certificate that was rejected
	// because of the error in hintErr.
	hintCert *Certificate
}

func (e UnknownAuthorityError) Error() string {
	s := "x509: certificate signed by unknown authority"
	if e.hintErr != nil {
		certName := e.hintCert.Subject.CommonName
		if len(certName) == 0 {
			if len(e.hintCert.Subject.Organization) > 0 {
				certName = e.hintCert.Subject.Organization[0]
			} else {
				certName = "serial:" + e.hintCert.SerialNumber.String()
			}
		}
		s += fmt.Sprintf(" (possibly because of %q while trying to verify candidate authority certificate %q)", e.hintErr, certName)
	}
	return s
}

// SystemRootsError results when we fail to load the system root certificates.
type SystemRootsError struct {
	Err error
}

func (se SystemRootsError) Error() string {
	msg := "x509: failed to load system roots and no roots provided"
	if se.Err != nil {
		return msg + "; " + se.Err.Error()
	}
	return msg
}

func (se SystemRootsError) Unwrap() error { return se.Err }

// errNotParsed is returned when a certificate without ASN.1 contents is
// verified. Platform-specific verification needs the ASN.1 contents.
var errNotParsed = errors.New("x509: missing ASN.1 contents; use ParseCertificate")

// VerifyOptions contains parameters for Certificate.Verify.
type VerifyOptions struct {
	// DNSName, if set, is checked against the leaf certificate with
	// Certificate.VerifyHostname or the platform verifier.
	DNSName string

	// Intermediates is an optional pool of certificates that are not trust
	// anchors, but can be used to form a chain from the leaf certificate to a
	// root certificate.
	Intermediates *CertPool
	// Roots is the set of trusted root certificates the leaf certificate needs
	// to chain up to. If nil, the system roots or the platform verifier are used.
	Roots *CertPool

	// CurrentTime is used to check the validity of all certificates in the
	// chain. If zero, the current time is used.
	CurrentTime time.Time

	// KeyUsages specifies which Extended Key Usage values are acceptable. A
	// chain is accepted if it allows any of the listed values. An empty list
	// means ExtKeyUsageServerAuth. To accept any key usage, include ExtKeyUsageAny.
	KeyUsages []ExtKeyUsage

	// MaxConstraintComparisions is the maximum number of comparisons to
	// perform when checking a given certificate's name constraints. If
	// zero, a sensible default is used. This limit prevents pathological
	// certificates from consuming excessive amounts of CPU time when
	// validating. It does not apply to the platform verifier.
	MaxConstraintComparisions int

	// CertificatePolicies specifies which certificate policy OIDs are
	// acceptable during policy validation. An empty CertificatePolices
	// field implies any valid policy is acceptable.
	CertificatePolicies []OID

	// The following policy fields are unexported, because we do not expect
	// users to actually need to use them, but are useful for testing the
	// policy validation code.

	// inhibitPolicyMapping indicates if policy mapping should be allowed
	// during path validation.
	inhibitPolicyMapping bool

	// requireExplicitPolicy indidicates if explicit policies must be present
	// for each certificate being validated.
	requireExplicitPolicy bool

	// inhibitAnyPolicy indicates if the anyPolicy policy should be
	// processed if present in a certificate being validated.
	inhibitAnyPolicy bool
}

const (
	leafCertificate = iota
	intermediateCertificate
	rootCertificate
)

// rfc2821Mailbox represents a “mailbox” (which is an email address to most
// people) by breaking it into the “local” (i.e. before the '@') and “domain”
// parts.
type rfc2821Mailbox struct {
	local, domain string
}

func (s rfc2821Mailbox) String() string {
	return fmt.Sprintf("%s@%s", s.local, s.domain)
}

// parseRFC2821Mailbox parses an email address into local and domain parts,
// based on the ABNF for a “Mailbox” from RFC 2821. According to RFC 5280,
// Section 4.2.1.6 that's correct for an rfc822Name from a certificate: “The
// format of an rfc822Name is a "Mailbox" as defined in RFC 2821, Section 4.1.2”.
func parseRFC2821Mailbox(in string) (mailbox rfc2821Mailbox, ok bool) {
	if len(in) == 0 {
		return mailbox, false
	}

	localPartBytes := make([]byte, 0, len(in)/2)

	if in[0] == '"' {
		// Quoted-string = DQUOTE *qcontent DQUOTE
		// non-whitespace-control = %d1-8 / %d11 / %d12 / %d14-31 / %d127
		// qcontent = qtext / quoted-pair
		// qtext = non-whitespace-control /
		//         %d33 / %d35-91 / %d93-126
		// quoted-pair = ("\" text) / obs-qp
		// text = %d1-9 / %d11 / %d12 / %d14-127 / obs-text
		//
		// (Names beginning with “obs-” are the obsolete syntax from RFC 2822,
		// Section 4. Since it has been 16 years, we no longer accept that.)
		in = in[1:]
	QuotedString:
		for {
			if len(in) == 0 {
				return mailbox, false
			}
			c := in[0]
			in = in[1:]

			switch {
			case c == '"':
				break QuotedString

			case c == '\\':
				// quoted-pair
				if len(in) == 0 {
					return mailbox, false
				}
				if in[0] == 11 ||
					in[0] == 12 ||
					(1 <= in[0] && in[0] <= 9) ||
					(14 <= in[0] && in[0] <= 127) {
					localPartBytes = append(localPartBytes, in[0])
					in = in[1:]
				} else {
					return mailbox, false
				}

			case c == 11 ||
				c == 12 ||
				// Space (char 32) is not allowed based on the
				// BNF, but RFC 3696 gives an example that
				// assumes that it is. Several “verified”
				// errata continue to argue about this point.
				// We choose to accept it.
				c == 32 ||
				c == 33 ||
				c == 127 ||
				(1 <= c && c <= 8) ||
				(14 <= c && c <= 31) ||
				(35 <= c && c <= 91) ||
				(93 <= c && c <= 126):
				// qtext
				localPartBytes = append(localPartBytes, c)

			default:
				return mailbox, false
			}
		}
	} else {
		// Atom ("." Atom)*
	NextChar:
		for len(in) > 0 {
			// atext from RFC 2822, Section 3.2.4
			c := in[0]

			switch {
			case c == '\\':
				// Examples given in RFC 3696 suggest that
				// escaped characters can appear outside of a
				// quoted string. Several “verified” errata
				// continue to argue the point. We choose to
				// accept it.
				in = in[1:]
				if len(in) == 0 {
					return mailbox, false
				}
				fallthrough

			case ('0' <= c && c <= '9') ||
				('a' <= c && c <= 'z') ||
				('A' <= c && c <= 'Z') ||
				c == '!' || c == '#' || c == '$' || c == '%' ||
				c == '&' || c == '\'' || c == '*' || c == '+' ||
				c == '-' || c == '/' || c == '=' || c == '?' ||
				c == '^' || c == '_' || c == '`' || c == '{' ||
				c == '|' || c == '}' || c == '~' || c == '.':
				localPartBytes = append(localPartBytes, in[0])
				in = in[1:]

			default:
				break NextChar
			}
		}

		if len(localPartBytes) == 0 {
			return mailbox, false
		}

		// From RFC 3696, Section 3:
		// “period (".") may also appear, but may not be used to start
		// or end the local part, nor may two or more consecutive
		// periods appear.”
		twoDots := []byte{'.', '.'}
		if localPartBytes[0] == '.' ||
			localPartBytes[len(localPartBytes)-1] == '.' ||
			bytes.Contains(localPartBytes, twoDots) {
			return mailbox, false
		}
	}

	if len(in) == 0 || in[0] != '@' {
		return mailbox, false
	}
	in = in[1:]

	// The RFC species a format for domains, but that's known to be
	// violated in practice so we accept that anything after an '@' is the
	// domain part.
	if !domainNameValid(in, false) {
		return mailbox, false
	}

	// Reject domain names containing @.
	if strings.ContainsRune(in, '@') {
		return mailbox, false
	}

	mailbox.local = string(localPartBytes)
	mailbox.domain = in
	return mailbox, true
}

// domainToReverseLabels converts a textual domain name like foo.example.com to
// the list of labels in reverse order, e.g. ["com", "example", "foo"].
func domainToReverseLabels(domain string) (reverseLabels []string, ok bool) {
	reverseLabels = make([]string, 0, strings.Count(domain, ".")+1)
	for len(domain) > 0 {
		if i := strings.LastIndexByte(domain, '.'); i == -1 {
			reverseLabels = append(reverseLabels, domain)
			domain = ""
		} else {
			reverseLabels = append(reverseLabels, domain[i+1:])
			domain = domain[:i]
			if i == 0 { // domain == ""
				// domain is prefixed with an empty label, append an empty
				// string to reverseLabels to indicate this.
				reverseLabels = append(reverseLabels, "")
			}
		}
	}

	if len(reverseLabels) > 0 && len(reverseLabels[0]) == 0 {
		// An empty label at the end indicates an absolute value.
		return nil, false
	}

	for _, label := range reverseLabels {
		if len(label) == 0 {
			// Empty labels are otherwise invalid.
			return nil, false
		}

		for _, c := range label {
			if c < 33 || c > 126 {
				// Invalid character.
				return nil, false
			}
		}
	}

	return reverseLabels, true
}

// isValid performs validity checks on c given that it is a candidate to append
// to the chain in currentChain.
func (c *Certificate) isValid(certType int, currentChain []*Certificate, opts *VerifyOptions) error {
	if len(c.UnhandledCriticalExtensions) > 0 {
		return UnhandledCriticalExtension{}
	}

	if len(currentChain) > 0 {
		child := currentChain[len(currentChain)-1]
		if !bytes.Equal(child.RawIssuer, c.RawSubject) {
			return CertificateInvalidError{c, NameMismatch, ""}
		}
	}

	now := opts.CurrentTime
	if now.IsZero() {
		now = time.Now()
	}
	if now.Before(c.NotBefore) {
		return CertificateInvalidError{
			Cert:   c,
			Reason: Expired,
			Detail: fmt.Sprintf("current time %s is before %s", now.Format(time.RFC3339), c.NotBefore.Format(time.RFC3339)),
		}
	} else if now.After(c.NotAfter) {
		return CertificateInvalidError{
			Cert:   c,
			Reason: Expired,
			Detail: fmt.Sprintf("current time %s is after %s", now.Format(time.RFC3339), c.NotAfter.Format(time.RFC3339)),
		}
	}

	if certType == intermediateCertificate || certType == rootCertificate {
		if len(currentChain) == 0 {
			return errors.New("x509: internal error: empty chain when appending CA cert")
		}
	}

	// KeyUsage status flags are ignored. From Engineering Security, Peter
	// Gutmann: A European government CA marked its signing certificates as
	// being valid for encryption only, but no-one noticed. Another
	// European CA marked its signature keys as not being valid for
	// signatures. A different CA marked its own trusted root certificate
	// as being invalid for certificate signing. Another national CA
	// distributed a certificate to be used to encrypt data for the
	// country’s tax authority that was marked as only being usable for
	// digital signatures but not for encryption. Yet another CA reversed
	// the order of the bit flags in the keyUsage due to confusion over
	// encoding endianness, essentially setting a random keyUsage in
	// certificates that it issued. Another CA created a self-invalidating
	// certificate by adding a certificate policy statement stipulating
	// that the certificate had to be used strictly as specified in the
	// keyUsage, and a keyUsage containing a flag indicating that the RSA
	// encryption key could only be used for Diffie-Hellman key agreement.

	if certType == intermediateCertificate && (!c.BasicConstraintsValid || !c.IsCA) {
		return CertificateInvalidError{c, NotAuthorizedToSign, ""}
	}

	if c.BasicConstraintsValid && c.MaxPathLen >= 0 {
		numIntermediates := len(currentChain) - 1
		if numIntermediates > c.MaxPathLen {
			return CertificateInvalidError{c, TooManyIntermediates, ""}
		}
	}

	return nil
}

// Verify attempts to verify c by building one or more chains from c to a
// certificate in opts.Roots, using certificates in opts.Intermediates if
// needed. If successful, it returns one or more chains where the first
// element of the chain is c and the last element is from opts.Roots.
//
// If opts.Roots is nil, the platform verifier might be used, and
// verification details might differ from what is described below. If system
// roots are unavailable the returned error will be of type SystemRootsError.
//
// Name constraints in the intermediates will be applied to all names claimed
// in the chain, not just opts.DNSName. Thus it is invalid for a leaf to claim
// example.com if an intermediate doesn't permit it, even if example.com is not
// the name being validated. Note that DirectoryName constraints are not
// supported.
//
// Name constraint validation follows the rules from RFC 5280, with the
// addition that DNS name constraints may use the leading period format
// defined for emails and URIs. When a constraint has a leading period
// it indicates that at least one additional label must be prepended to
// the constrained name to be considered valid.
//
// Extended Key Usage values are enforced nested down a chain, so an intermediate
// or root that enumerates EKUs prevents a leaf from asserting an EKU not in that
// list. (While this is not specified, it is common practice in order to limit
// the types of certificates a CA can issue.)
//
// Certificates that use SHA1WithRSA and ECDSAWithSHA1 signatures are not supported,
// and will not be used to build chains.
//
// Certificates other than c in the returned chains should not be modified.
//
// WARNING: this function doesn't do any revocation checking.
func (c *Certificate) Verify(opts VerifyOptions) ([][]*Certificate, error) {
	// Platform-specific verification needs the ASN.1 contents so
	// this makes the behavior consistent across platforms.
	if len(c.Raw) == 0 {
		return nil, errNotParsed
	}
	for i := 0; i < opts.Intermediates.len(); i++ {
		c, _, err := opts.Intermediates.cert(i)
		if err != nil {
			return nil, fmt.Errorf("crypto/x509: error fetching intermediate: %w", err)
		}
		if len(c.Raw) == 0 {
			return nil, errNotParsed
		}
	}

	// Use platform verifiers, where available, if Roots is from SystemCertPool.
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		// Don't use the system verifier if the system pool was replaced with a non-system pool,
		// i.e. if SetFallbackRoots was called with x509usefallbackroots=1.
		systemPool := systemRootsPool()
		if opts.Roots == nil && (systemPool == nil || systemPool.systemPool) {
			return c.systemVerify(&opts)
		}
		if opts.Roots != nil && opts.Roots.systemPool {
			platformChains, err := c.systemVerify(&opts)
			// If the platform verifier succeeded, or there are no additional
			// roots, return the platform verifier result. Otherwise, continue
			// with the Go verifier.
			if err == nil || opts.Roots.len() == 0 {
				return platformChains, err
			}
		}
	}

	if opts.Roots == nil {
		opts.Roots = systemRootsPool()
		if opts.Roots == nil {
			return nil, SystemRootsError{systemRootsErr}
		}
	}

	err := c.isValid(leafCertificate, nil, &opts)
	if err != nil {
		return nil, err
	}

	if len(opts.DNSName) > 0 {
		err = c.VerifyHostname(opts.DNSName)
		if err != nil {
			return nil, err
		}
	}

	var candidateChains [][]*Certificate
	if opts.Roots.contains(c) {
		candidateChains = [][]*Certificate{{c}}
	} else {
		candidateChains, err = c.buildChains([]*Certificate{c}, nil, &opts)
		if err != nil {
			return nil, err
		}
	}

	anyKeyUsage := false
	for _, eku := range opts.KeyUsages {
		if eku == ExtKeyUsageAny {
			// The presence of anyExtendedKeyUsage overrides any other key usage.
			anyKeyUsage = true
			break
		}
	}

	if len(opts.KeyUsages) == 0 {
		opts.KeyUsages = []ExtKeyUsage{ExtKeyUsageServerAuth}
	}

	var invalidPoliciesChains int
	var incompatibleKeyUsageChains int
	var constraintsHintErr error
	candidateChains = slices.DeleteFunc(candidateChains, func(chain []*Certificate) bool {
		if !policiesValid(chain, opts) {
			invalidPoliciesChains++
			return true
		}
		// If any key usage is acceptable, no need to check the chain for
		// key usages.
		if !anyKeyUsage && !checkChainForKeyUsage(chain, opts.KeyUsages) {
			incompatibleKeyUsageChains++
			return true
		}
		if err := checkChainConstraints(chain); err != nil {
			if constraintsHintErr == nil {
				constraintsHintErr = CertificateInvalidError{c, CANotAuthorizedForThisName, err.Error()}
			}
			return true
		}
		return false
	})

	if len(candidateChains) == 0 {
		if constraintsHintErr != nil {
			return nil, constraintsHintErr // Preserve previous constraint behavior
		}
		var details []string
		if incompatibleKeyUsageChains > 0 {
			if invalidPoliciesChains == 0 {
				return nil, CertificateInvalidError{c, IncompatibleUsage, ""}
			}
			details = append(details, fmt.Sprintf("%d candidate chains with incompatible key usage", incompatibleKeyUsageChains))
		}
		if invalidPoliciesChains > 0 {
			details = append(details, fmt.Sprintf("%d candidate chains with invalid policies", invalidPoliciesChains))
		}
		err = CertificateInvalidError{c, NoValidChains, strings.Join(details, ", ")}
		return nil, err
	}

	return candidateChains, nil
}

func appendToFreshChain(chain []*Certificate, cert *Certificate) []*Certificate {
	n := make([]*Certificate, len(chain)+1)
	copy(n, chain)
	n[len(chain)] = cert
	return n
}

// alreadyInChain checks whether a candidate certificate is present in a chain.
// Rather than doing a direct byte for byte equivalency check, we check if the
// subject, public key, and SAN, if present, are equal. This prevents loops that
// are created by mutual cross-signatures, or other cross-signature bridge
// oddities.
func alreadyInChain(candidate *Certificate, chain []*Certificate) bool {
	type pubKeyEqual interface {
		Equal(crypto.PublicKey) bool
	}

	var candidateSAN *pkix.Extension
	for _, ext := range candidate.Extensions {
		if ext.Id.Equal(oidExtensionSubjectAltName) {
			candidateSAN = &ext
			break
		}
	}

	for _, cert := range chain {
		if !bytes.Equal(candidate.RawSubject, cert.RawSubject) {
			continue
		}
		// We enforce the canonical encoding of SPKI (by only allowing the
		// correct AI parameter encodings in parseCertificate), so it's safe to
		// directly compare the raw bytes.
		if !bytes.Equal(candidate.RawSubjectPublicKeyInfo, cert.RawSubjectPublicKeyInfo) {
			continue
		}
		var certSAN *pkix.Extension
		for _, ext := range cert.Extensions {
			if ext.Id.Equal(oidExtensionSubjectAltName) {
				certSAN = &ext
				break
			}
		}
		if candidateSAN == nil && certSAN == nil {
			return true
		} else if candidateSAN == nil || certSAN == nil {
			return false
		}
		if bytes.Equal(candidateSAN.Value, certSAN.Value) {
			return true
		}
	}
	return false
}

// maxChainSignatureChecks is the maximum number of CheckSignatureFrom calls
// that an invocation of buildChains will (transitively) make. Most chains are
// less than 15 certificates long, so this leaves space for multiple chains and
// for failed checks due to different intermediates having the same Subject.
const maxChainSignatureChecks = 100

var errSignatureLimit = errors.New("x509: signature check attempts limit reached while verifying certificate chain")

func (c *Certificate) buildChains(currentChain []*Certificate, sigChecks *int, opts *VerifyOptions) (chains [][]*Certificate, err error) {
	var (
		hintErr  error
		hintCert *Certificate
	)

	considerCandidate := func(certType int, candidate potentialParent) {
		if sigChecks == nil {
			sigChecks = new(int)
		}
		*sigChecks++
		if *sigChecks > maxChainSignatureChecks {
			err = errSignatureLimit
			return
		}

		if candidate.cert.PublicKey == nil || alreadyInChain(candidate.cert, currentChain) {
			return
		}

		if err := c.CheckSignatureFrom(candidate.cert); err != nil {
			if hintErr == nil {
				hintErr = err
				hintCert = candidate.cert
			}
			return
		}

		err = candidate.cert.isValid(certType, currentChain, opts)
		if err != nil {
			if hintErr == nil {
				hintErr = err
				hintCert = candidate.cert
			}
			return
		}

		if candidate.constraint != nil {
			if err := candidate.constraint(currentChain); err != nil {
				if hintErr == nil {
					hintErr = err
					hintCert = candidate.cert
				}
				return
			}
		}

		switch certType {
		case rootCertificate:
			chains = append(chains, appendToFreshChain(currentChain, candidate.cert))
		case intermediateCertificate:
			var childChains [][]*Certificate
			childChains, err = candidate.cert.buildChains(appendToFreshChain(currentChain, candidate.cert), sigChecks, opts)
			chains = append(chains, childChains...)
		}
	}

candidateLoop:
	for _, parents := range []struct {
		certType   int
		potentials []potentialParent
	}{
		{rootCertificate, opts.Roots.findPotentialParents(c)},
		{intermediateCertificate, opts.Intermediates.findPotentialParents(c)},
	} {
		for _, parent := range parents.potentials {
			considerCandidate(parents.certType, parent)
			if err == errSignatureLimit {
				break candidateLoop
			}
		}
	}

	if len(chains) > 0 {
		err = nil
	}
	if len(chains) == 0 && err == nil {
		err = UnknownAuthorityError{c, hintErr, hintCert}
	}

	return
}

func validHostnamePattern(host string) bool { return validHostname(host, true) }
func validHostnameInput(host string) bool   { return validHostname(host, false) }

// validHostname reports whether host is a valid hostname that can be matched or
// matched against according to RFC 6125 2.2, with some leniency to accommodate
// legacy values.
func validHostname(host string, isPattern bool) bool {
	if !isPattern {
		host = strings.TrimSuffix(host, ".")
	}
	if len(host) == 0 {
		return false
	}
	if host == "*" {
		// Bare wildcards are not allowed, they are not valid DNS names,
		// nor are they allowed per RFC 6125.
		return false
	}

	for i, part := range strings.Split(host, ".") {
		if part == "" {
			// Empty label.
			return false
		}
		if isPattern && i == 0 && part == "*" {
			// Only allow full left-most wildcards, as those are the only ones
			// we match, and matching literal '*' characters is probably never
			// the expected behavior.
			continue
		}
		for j, c := range part {
			if 'a' <= c && c <= 'z' {
				continue
			}
			if '0' <= c && c <= '9' {
				continue
			}
			if 'A' <= c && c <= 'Z' {
				continue
			}
			if c == '-' && j != 0 {
				continue
			}
			if c == '_' {
				// Not a valid character in hostnames, but commonly
				// found in deployments outside the WebPKI.
				continue
			}
			return false
		}
	}

	return true
}

func matchExactly(hostA, hostB string) bool {
	if hostA == "" || hostA == "." || hostB == "" || hostB == "." {
		return false
	}
	return toLowerCaseASCII(hostA) == toLowerCaseASCII(hostB)
}

func matchHostnames(pattern string, hostParts []string) bool {
	pattern = toLowerCaseASCII(pattern)

	if len(pattern) == 0 || len(hostParts) == 0 {
		return false
	}

	patternParts := strings.Split(pattern, ".")

	if len(patternParts) != len(hostParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if i == 0 && patternPart == "*" {
			continue
		}
		if patternPart != hostParts[i] {
			return false
		}
	}

	return true
}

// toLowerCaseASCII returns a lower-case version of in. See RFC 6125 6.4.1. We use
// an explicitly ASCII function to avoid any sharp corners resulting from
// performing Unicode operations on DNS labels.
func toLowerCaseASCII(in string) string {
	// If the string is already lower-case then there's nothing to do.
	isAlreadyLowerCase := true
	for _, c := range in {
		if c == utf8.RuneError {
			// If we get a UTF-8 error then there might be
			// upper-case ASCII bytes in the invalid sequence.
			isAlreadyLowerCase = false
			break
		}
		if 'A' <= c && c <= 'Z' {
			isAlreadyLowerCase = false
			break
		}
	}

	if isAlreadyLowerCase {
		return in
	}

	out := []byte(in)
	for i, c := range out {
		if 'A' <= c && c <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return string(out)
}

// VerifyHostname returns nil if c is a valid certificate for the named host.
// Otherwise it returns an error describing the mismatch.
//
// IP addresses can be optionally enclosed in square brackets and are checked
// against the IPAddresses field. Other names are checked case insensitively
// against the DNSNames field. If the names are valid hostnames, the certificate
// fields can have a wildcard as the complete left-most label (e.g. *.example.com).
//
// Note that the legacy Common Name field is ignored.
func (c *Certificate) VerifyHostname(h string) error {
	// IP addresses may be written in [ ].
	candidateIP := h
	if len(h) >= 3 && h[0] == '[' && h[len(h)-1] == ']' {
		candidateIP = h[1 : len(h)-1]
	}
	// We use netip.ParseAddr() to allow IPv6 scoped addresses.
	if addr, err := netip.ParseAddr(candidateIP); err == nil {
		// We only match IP addresses against IP SANs.
		// See RFC 6125, Appendix B.2.
		ip := net.IP(addr.AsSlice())
		for _, candidate := range c.IPAddresses {
			if ip.Equal(candidate) {
				return nil
			}
		}
		return HostnameError{c, ip.String()}
	}

	candidateName := toLowerCaseASCII(h) // Save allocations inside the loop.
	validCandidateName := validHostnameInput(candidateName)
	hostParts := splitHostname(candidateName)

	for _, match := range c.DNSNames {
		// Ideally, we'd only match valid hostnames according to RFC 6125 like
		// browsers (more or less) do, but in practice Go is used in a wider
		// array of contexts and can't even assume DNS resolution. Instead,
		// always allow perfect matches, and only apply wildcard and trailing
		// dot processing to valid hostnames.
		if validCandidateName && validHostnamePattern(match) {
			if matchHostnames(match, hostParts) {
				return nil
			}
		} else {
			if matchExactly(match, candidateName) {
				return nil
			}
		}
	}

	return HostnameError{c, h}
}

func splitHostname(host string) []string {
	return strings.Split(toLowerCaseASCII(strings.TrimSuffix(host, ".")), ".")
}

func checkChainForKeyUsage(chain []*Certificate, keyUsages []ExtKeyUsage) bool {
	usages := make([]ExtKeyUsage, len(keyUsages))
	copy(usages, keyUsages)

	if len(chain) == 0 {
		return false
	}

	usagesRemaining := len(usages)

	// We walk down the list and cross out any usages that aren't supported
	// by each certificate. If we cross out all the usages, then the chain
	// is unacceptable.

NextCert:
	for i := len(chain) - 1; i >= 0; i-- {
		cert := chain[i]
		if len(cert.ExtKeyUsage) == 0 && len(cert.UnknownExtKeyUsage) == 0 {
			// The certificate doesn't have any extended key usage specified.
			continue
		}

		for _, usage := range cert.ExtKeyUsage {
			if usage == ExtKeyUsageAny {
				// The certificate is explicitly good for any usage.
				continue NextCert
			}
		}

		const invalidUsage = -1

	NextRequestedUsage:
		for i, requestedUsage := range usages {
			if requestedUsage == invalidUsage {
				continue
			}

			for _, usage := range cert.ExtKeyUsage {
				if requestedUsage == usage {
					continue NextRequestedUsage
				}
			}

			usages[i] = invalidUsage
			usagesRemaining--
			if usagesRemaining == 0 {
				return false
			}
		}
	}

	return true
}

func mustNewOIDFromInts(ints []uint64) OID {
	oid, err := OIDFromInts(ints)
	if err != nil {
		panic(fmt.Sprintf("OIDFromInts(%v) unexpected error: %v", ints, err))
	}
	return oid
}

type policyGraphNode struct {
	validPolicy       OID
	expectedPolicySet []OID
	// we do not implement qualifiers, so we don't track qualifier_set

	parents  map[*policyGraphNode]bool
	children map[*policyGraphNode]bool
}

func newPolicyGraphNode(valid OID, parents []*policyGraphNode) *policyGraphNode {
	n := &policyGraphNode{
		validPolicy:       valid,
		expectedPolicySet: []OID{valid},
		children:          map[*policyGraphNode]bool{},
		parents:           map[*policyGraphNode]bool{},
	}
	for _, p := range parents {
		p.children[n] = true
		n.parents[p] = true
	}
	return n
}

type policyGraph struct {
	strata []map[string]*policyGraphNode
	// map of OID -> nodes at strata[depth-1] with OID in their expectedPolicySet
	parentIndex map[string][]*policyGraphNode
	depth       int
}

var anyPolicyOID = mustNewOIDFromInts([]uint64{2, 5, 29, 32, 0})

func newPolicyGraph() *policyGraph {
	root := policyGraphNode{
		validPolicy:       anyPolicyOID,
		expectedPolicySet: []OID{anyPolicyOID},
		children:          map[*policyGraphNode]bool{},
		parents:           map[*policyGraphNode]bool{},
	}
	return &policyGraph{
		depth:  0,
		strata: []map[string]*policyGraphNode{{string(anyPolicyOID.der): &root}},
	}
}

func (pg *policyGraph) insert(n *policyGraphNode) {
	pg.strata[pg.depth][string(n.validPolicy.der)] = n
}

func (pg *policyGraph) parentsWithExpected(expected OID) []*policyGraphNode {
	if pg.depth == 0 {
		return nil
	}
	return pg.parentIndex[string(expected.der)]
}

func (pg *policyGraph) parentWithAnyPolicy() *policyGraphNode {
	if pg.depth == 0 {
		return nil
	}
	return pg.strata[pg.depth-1][string(anyPolicyOID.der)]
}

func (pg *policyGraph) parents() iter.Seq[*policyGraphNode] {
	if pg.depth == 0 {
		return nil
	}
	return maps.Values(pg.strata[pg.depth-1])
}

func (pg *policyGraph) leaves() map[string]*policyGraphNode {
	return pg.strata[pg.depth]
}

func (pg *policyGraph) leafWithPolicy(policy OID) *policyGraphNode {
	return pg.strata[pg.depth][string(policy.der)]
}

func (pg *policyGraph) deleteLeaf(policy OID) {
	n := pg.strata[pg.depth][string(policy.der)]
	if n == nil {
		return
	}
	for p := range n.parents {
		delete(p.children, n)
	}
	for c := range n.children {
		delete(c.parents, n)
	}
	delete(pg.strata[pg.depth], string(policy.der))
}

func (pg *policyGraph) validPolicyNodes() []*policyGraphNode {
	var validNodes []*policyGraphNode
	for i := pg.depth; i >= 0; i-- {
		for _, n := range pg.strata[i] {
			if n.validPolicy.Equal(anyPolicyOID) {
				continue
			}

			if len(n.parents) == 1 {
				for p := range n.parents {
					if p.validPolicy.Equal(anyPolicyOID) {
						validNodes = append(validNodes, n)
					}
				}
			}
		}
	}
	return validNodes
}

func (pg *policyGraph) prune() {
	for i := pg.depth - 1; i > 0; i-- {
		for _, n := range pg.strata[i] {
			if len(n.children) == 0 {
				for p := range n.parents {
					delete(p.children, n)
				}
				delete(pg.strata[i], string(n.validPolicy.der))
			}
		}
	}
}

func (pg *policyGraph) incrDepth() {
	pg.parentIndex = map[string][]*policyGraphNode{}
	for _, n := range pg.strata[pg.depth] {
		for _, e := range n.expectedPolicySet {
			pg.parentIndex[string(e.der)] = append(pg.parentIndex[string(e.der)], n)
		}
	}

	pg.depth++
	pg.strata = append(pg.strata, map[string]*policyGraphNode{})
}

func policiesValid(chain []*Certificate, opts VerifyOptions) bool {
	// The following code implements the policy verification algorithm as
	// specified in RFC 5280 and updated by RFC 9618. In particular the
	// following sections are replaced by RFC 9618:
	//	* 6.1.2 (a)
	//	* 6.1.3 (d)
	//	* 6.1.3 (e)
	//	* 6.1.3 (f)
	//	* 6.1.4 (b)
	//	* 6.1.5 (g)

	if len(chain) == 1 {
		return true
	}

	// n is the length of the chain minus the trust anchor
	n := len(chain) - 1

	pg := newPolicyGraph()
	var inhibitAnyPolicy, explicitPolicy, policyMapping int
	if !opts.inhibitAnyPolicy {
		inhibitAnyPolicy = n + 1
	}
	if !opts.requireExplicitPolicy {
		explicitPolicy = n + 1
	}
	if !opts.inhibitPolicyMapping {
		policyMapping = n + 1
	}

	initialUserPolicySet := map[string]bool{}
	for _, p := range opts.CertificatePolicies {
		initialUserPolicySet[string(p.der)] = true
	}
	// If the user does not pass any policies, we consider
	// that equivalent to passing anyPolicyOID.
	if len(initialUserPolicySet) == 0 {
		initialUserPolicySet[string(anyPolicyOID.der)] = true
	}

	for i := n - 1; i >= 0; i-- {
		cert := chain[i]

		isSelfSigned := bytes.Equal(cert.RawIssuer, cert.RawSubject)

		// 6.1.3 (e) -- as updated by RFC 9618
		if len(cert.Policies) == 0 {
			pg = nil
		}

		// 6.1.3 (f) -- as updated by RFC 9618
		if explicitPolicy == 0 && pg == nil {
			return false
		}

		if pg != nil {
			pg.incrDepth()

			policies := map[string]bool{}

			// 6.1.3 (d) (1) -- as updated by RFC 9618
			for _, policy := range cert.Policies {
				policies[string(policy.der)] = true

				if policy.Equal(anyPolicyOID) {
					continue
				}

				// 6.1.3 (d) (1) (i) -- as updated by RFC 9618
				parents := pg.parentsWithExpected(policy)
				if len(parents) == 0 {
					// 6.1.3 (d) (1) (ii) -- as updated by RFC 9618
					if anyParent := pg.parentWithAnyPolicy(); anyParent != nil {
						parents = []*policyGraphNode{anyParent}
					}
				}
				if len(parents) > 0 {
					pg.insert(newPolicyGraphNode(policy, parents))
				}
			}

			// 6.1.3 (d) (2) -- as updated by RFC 9618
			// NOTE: in the check "n-i < n" our i is different from the i in the specification.
			// In the specification chains go from the trust anchor to the leaf, whereas our
			// chains go from the leaf to the trust anchor, so our i's our inverted. Our
			// check here matches the check "i < n" in the specification.
			if policies[string(anyPolicyOID.der)] && (inhibitAnyPolicy > 0 || (n-i < n && isSelfSigned)) {
				missing := map[string][]*policyGraphNode{}
				leaves := pg.leaves()
				for p := range pg.parents() {
					for _, expected := range p.expectedPolicySet {
						if leaves[string(expected.der)] == nil {
							missing[string(expected.der)] = append(missing[string(expected.der)], p)
						}
					}
				}

				for oidStr, parents := range missing {
					pg.insert(newPolicyGraphNode(OID{der: []byte(oidStr)}, parents))
				}
			}

			// 6.1.3 (d) (3) -- as updated by RFC 9618
			pg.prune()

			if i != 0 {
				// 6.1.4 (b) -- as updated by RFC 9618
				if len(cert.PolicyMappings) > 0 {
					// collect map of issuer -> []subject
					mappings := map[string][]OID{}

					for _, mapping := range cert.PolicyMappings {
						if policyMapping > 0 {
							if mapping.IssuerDomainPolicy.Equal(anyPolicyOID) || mapping.SubjectDomainPolicy.Equal(anyPolicyOID) {
								// Invalid mapping
								return false
							}
							mappings[string(mapping.IssuerDomainPolicy.der)] = append(mappings[string(mapping.IssuerDomainPolicy.der)], mapping.SubjectDomainPolicy)
						} else {
							// 6.1.4 (b) (3) (i) -- as updated by RFC 9618
							pg.deleteLeaf(mapping.IssuerDomainPolicy)
						}
					}

					// 6.1.4 (b) (3) (ii) -- as updated by RFC 9618
					pg.prune()

					for issuerStr, subjectPolicies := range mappings {
						// 6.1.4 (b) (1) -- as updated by RFC 9618
						if matching := pg.leafWithPolicy(OID{der: []byte(issuerStr)}); matching != nil {
							matching.expectedPolicySet = subjectPolicies
						} else if matching := pg.leafWithPolicy(anyPolicyOID); matching != nil {
							// 6.1.4 (b) (2) -- as updated by RFC 9618
							n := newPolicyGraphNode(OID{der: []byte(issuerStr)}, []*policyGraphNode{matching})
							n.expectedPolicySet = subjectPolicies
							pg.insert(n)
						}
					}
				}
			}
		}

		if i != 0 {
			// 6.1.4 (h)
			if !isSelfSigned {
				if explicitPolicy > 0 {
					explicitPolicy--
				}
				if policyMapping > 0 {
					policyMapping--
				}
				if inhibitAnyPolicy > 0 {
					inhibitAnyPolicy--
				}
			}

			// 6.1.4 (i)
			if (cert.RequireExplicitPolicy > 0 || cert.RequireExplicitPolicyZero) && cert.RequireExplicitPolicy < explicitPolicy {
				explicitPolicy = cert.RequireExplicitPolicy
			}
			if (cert.InhibitPolicyMapping > 0 || cert.InhibitPolicyMappingZero) && cert.InhibitPolicyMapping < policyMapping {
				policyMapping = cert.InhibitPolicyMapping
			}
			// 6.1.4 (j)
			if (cert.InhibitAnyPolicy > 0 || cert.InhibitAnyPolicyZero) && cert.InhibitAnyPolicy < inhibitAnyPolicy {
				inhibitAnyPolicy = cert.InhibitAnyPolicy
			}
		}
	}

	// 6.1.5 (a)
	if explicitPolicy > 0 {
		explicitPolicy--
	}

	// 6.1.5 (b)
	if chain[0].RequireExplicitPolicyZero {
		explicitPolicy = 0
	}

	// 6.1.5 (g) (1) -- as updated by RFC 9618
	var validPolicyNodeSet []*policyGraphNode
	// 6.1.5 (g) (2) -- as updated by RFC 9618
	if pg != nil {
		validPolicyNodeSet = pg.validPolicyNodes()
		// 6.1.5 (g) (3) -- as updated by RFC 9618
		if currentAny := pg.leafWithPolicy(anyPolicyOID); currentAny != nil {
			validPolicyNodeSet = append(validPolicyNodeSet, currentAny)
		}
	}

	// 6.1.5 (g) (4) -- as updated by RFC 9618
	authorityConstrainedPolicySet := map[string]bool{}
	for _, n := range validPolicyNodeSet {
		authorityConstrainedPolicySet[string(n.validPolicy.der)] = true
	}
	// 6.1.5 (g) (5) -- as updated by RFC 9618
	userConstrainedPolicySet := maps.Clone(authorityConstrainedPolicySet)
	// 6.1.5 (g) (6) -- as updated by RFC 9618
	if len(initialUserPolicySet) != 1 || !initialUserPolicySet[string(anyPolicyOID.der)] {
		// 6.1.5 (g) (6) (i) -- as updated by RFC 9618
		for p := range userConstrainedPolicySet {
			if !initialUserPolicySet[p] {
				delete(userConstrainedPolicySet, p)
			}
		}
		// 6.1.5 (g) (6) (ii) -- as updated by RFC 9618
		if authorityConstrainedPolicySet[string(anyPolicyOID.der)] {
			for policy := range initialUserPolicySet {
				userConstrainedPolicySet[policy] = true
			}
		}
	}

	if explicitPolicy == 0 && len(userConstrainedPolicySet) == 0 {
		return false
	}

	return true
}

```

// === FILE: references/go/src/crypto/x509/x509.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package x509 implements a subset of the X.509 standard.
//
// It allows parsing and generating certificates, certificate signing
// requests, certificate revocation lists, and encoded public and private keys.
// It provides a certificate verifier, complete with a chain builder.
//
// The package targets the X.509 technical profile defined by the IETF (RFC
// 2459/3280/5280), and as further restricted by the CA/Browser Forum Baseline
// Requirements. There is minimal support for features outside of these
// profiles, as the primary goal of the package is to provide compatibility
// with the publicly trusted TLS certificate ecosystem and its policies and
// constraints.
//
// On macOS and Windows, certificate verification is handled by system APIs, but
// the package aims to apply consistent validation rules across operating
// systems.
package x509

import (
	"bytes"
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/fips140"
	"crypto/mldsa"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"internal/godebug"
	"io"
	"math/big"
	"net"
	"net/url"
	"strconv"
	"time"
	"unicode"

	// Explicitly import these for their crypto.RegisterHash init side-effects.
	// Keep these as blank imports, even if they're imported above.
	_ "crypto/sha1"
	_ "crypto/sha256"
	_ "crypto/sha512"

	"golang.org/x/crypto/cryptobyte"
	cryptobyte_asn1 "golang.org/x/crypto/cryptobyte/asn1"
)

// pkixPublicKey reflects a PKIX public key structure. See SubjectPublicKeyInfo
// in RFC 3280.
type pkixPublicKey struct {
	Algo      pkix.AlgorithmIdentifier
	BitString asn1.BitString
}

// ParsePKIXPublicKey parses a public key in PKIX, ASN.1 DER form. The encoded
// public key is a SubjectPublicKeyInfo structure (see RFC 5280, Section 4.1).
//
// It returns a *[rsa.PublicKey], *[dsa.PublicKey], *[ecdsa.PublicKey],
// [ed25519.PublicKey] (not a pointer), *[mldsa.PublicKey], or *[ecdh.PublicKey]
// (for X25519). More types might be supported in the future.
//
// This kind of key is commonly encoded in PEM blocks of type "PUBLIC KEY".
func ParsePKIXPublicKey(derBytes []byte) (pub any, err error) {
	var pki publicKeyInfo
	if rest, err := asn1.Unmarshal(derBytes, &pki); err != nil {
		if _, err := asn1.Unmarshal(derBytes, &pkcs1PublicKey{}); err == nil {
			return nil, errors.New("x509: failed to parse public key (use ParsePKCS1PublicKey instead for this key format)")
		}
		return nil, err
	} else if len(rest) != 0 {
		return nil, errors.New("x509: trailing data after ASN.1 of public-key")
	}
	return parsePublicKey(&pki)
}

func marshalPublicKey(pub any) (publicKeyBytes []byte, publicKeyAlgorithm pkix.AlgorithmIdentifier, err error) {
	switch pub := pub.(type) {
	case *rsa.PublicKey:
		publicKeyBytes, err = asn1.Marshal(pkcs1PublicKey{
			N: pub.N,
			E: pub.E,
		})
		if err != nil {
			return nil, pkix.AlgorithmIdentifier{}, err
		}
		publicKeyAlgorithm.Algorithm = oidPublicKeyRSA
		// This is a NULL parameters value which is required by
		// RFC 3279, Section 2.3.1.
		publicKeyAlgorithm.Parameters = asn1.NullRawValue
	case *ecdsa.PublicKey:
		oid, ok := oidFromNamedCurve(pub.Curve)
		if !ok {
			return nil, pkix.AlgorithmIdentifier{}, errors.New("x509: unsupported elliptic curve")
		}
		publicKeyBytes, err = pub.Bytes()
		if err != nil {
			return nil, pkix.AlgorithmIdentifier{}, err
		}
		publicKeyAlgorithm.Algorithm = oidPublicKeyECDSA
		var paramBytes []byte
		paramBytes, err = asn1.Marshal(oid)
		if err != nil {
			return
		}
		publicKeyAlgorithm.Parameters.FullBytes = paramBytes
	case ed25519.PublicKey:
		publicKeyBytes = pub
		publicKeyAlgorithm.Algorithm = oidPublicKeyEd25519
	case *mldsa.PublicKey:
		oid, ok := oidFromMLDSAParameters(pub.Parameters())
		if !ok {
			return nil, pkix.AlgorithmIdentifier{}, errors.New("x509: unsupported ML-DSA parameters")
		}
		publicKeyBytes = pub.Bytes()
		publicKeyAlgorithm.Algorithm = oid
	case *ecdh.PublicKey:
		publicKeyBytes = pub.Bytes()
		if pub.Curve() == ecdh.X25519() {
			publicKeyAlgorithm.Algorithm = oidPublicKeyX25519
		} else {
			oid, ok := oidFromECDHCurve(pub.Curve())
			if !ok {
				return nil, pkix.AlgorithmIdentifier{}, errors.New("x509: unsupported elliptic curve")
			}
			publicKeyAlgorithm.Algorithm = oidPublicKeyECDSA
			var paramBytes []byte
			paramBytes, err = asn1.Marshal(oid)
			if err != nil {
				return
			}
			publicKeyAlgorithm.Parameters.FullBytes = paramBytes
		}
	default:
		return nil, pkix.AlgorithmIdentifier{}, fmt.Errorf("x509: unsupported public key type: %T", pub)
	}

	return publicKeyBytes, publicKeyAlgorithm, nil
}

// MarshalPKIXPublicKey converts a public key to PKIX, ASN.1 DER form.
// The encoded public key is a SubjectPublicKeyInfo structure
// (see RFC 5280, Section 4.1).
//
// The following key types are currently supported: *[rsa.PublicKey],
// *[ecdsa.PublicKey], [ed25519.PublicKey] (not a pointer), *[mldsa.PublicKey],
// and *[ecdh.PublicKey]. Unsupported key types result in an error.
//
// This kind of key is commonly encoded in PEM blocks of type "PUBLIC KEY".
func MarshalPKIXPublicKey(pub any) ([]byte, error) {
	var publicKeyBytes []byte
	var publicKeyAlgorithm pkix.AlgorithmIdentifier
	var err error

	if publicKeyBytes, publicKeyAlgorithm, err = marshalPublicKey(pub); err != nil {
		return nil, err
	}

	pkix := pkixPublicKey{
		Algo: publicKeyAlgorithm,
		BitString: asn1.BitString{
			Bytes:     publicKeyBytes,
			BitLength: 8 * len(publicKeyBytes),
		},
	}

	ret, _ := asn1.Marshal(pkix)
	return ret, nil
}

// These structures reflect the ASN.1 structure of X.509 certificates.:

type certificate struct {
	TBSCertificate     tbsCertificate
	SignatureAlgorithm pkix.AlgorithmIdentifier
	SignatureValue     asn1.BitString
}

type tbsCertificate struct {
	Raw                asn1.RawContent
	Version            int `asn1:"optional,explicit,default:0,tag:0"`
	SerialNumber       *big.Int
	SignatureAlgorithm pkix.AlgorithmIdentifier
	Issuer             asn1.RawValue
	Validity           validity
	Subject            asn1.RawValue
	PublicKey          publicKeyInfo
	UniqueId           asn1.BitString   `asn1:"optional,tag:1"`
	SubjectUniqueId    asn1.BitString   `asn1:"optional,tag:2"`
	Extensions         []pkix.Extension `asn1:"omitempty,optional,explicit,tag:3"`
}

type dsaAlgorithmParameters struct {
	P, Q, G *big.Int
}

type validity struct {
	NotBefore, NotAfter time.Time
}

type publicKeyInfo struct {
	Raw       asn1.RawContent
	Algorithm pkix.AlgorithmIdentifier
	PublicKey asn1.BitString
}

// RFC 5280,  4.2.1.1
type authKeyId struct {
	Id []byte `asn1:"optional,tag:0"`
}

type SignatureAlgorithm int

const (
	UnknownSignatureAlgorithm SignatureAlgorithm = iota

	MD2WithRSA  // Unsupported.
	MD5WithRSA  // Only supported for signing, not verification.
	SHA1WithRSA // Only supported for signing, and verification of CRLs, CSRs, and OCSP responses.
	SHA256WithRSA
	SHA384WithRSA
	SHA512WithRSA
	DSAWithSHA1   // Unsupported.
	DSAWithSHA256 // Unsupported.
	ECDSAWithSHA1 // Only supported for signing, and verification of CRLs, CSRs, and OCSP responses.
	ECDSAWithSHA256
	ECDSAWithSHA384
	ECDSAWithSHA512
	SHA256WithRSAPSS
	SHA384WithRSAPSS
	SHA512WithRSAPSS
	PureEd25519
	MLDSA44
	MLDSA65
	MLDSA87
)

func (algo SignatureAlgorithm) isRSAPSS() bool {
	for _, details := range signatureAlgorithmDetails {
		if details.algo == algo {
			return details.isRSAPSS
		}
	}
	return false
}

func (algo SignatureAlgorithm) hashFunc() crypto.Hash {
	for _, details := range signatureAlgorithmDetails {
		if details.algo == algo {
			return details.hash
		}
	}
	return crypto.Hash(0)
}

func (algo SignatureAlgorithm) String() string {
	for _, details := range signatureAlgorithmDetails {
		if details.algo == algo {
			return details.name
		}
	}
	return strconv.Itoa(int(algo))
}

type PublicKeyAlgorithm int

const (
	UnknownPublicKeyAlgorithm PublicKeyAlgorithm = iota
	RSA
	DSA // Only supported for parsing.
	ECDSA
	Ed25519
	MLDSA
)

var publicKeyAlgoName = [...]string{
	RSA:     "RSA",
	DSA:     "DSA",
	ECDSA:   "ECDSA",
	Ed25519: "Ed25519",
	MLDSA:   "ML-DSA",
}

func (algo PublicKeyAlgorithm) String() string {
	if 0 < algo && int(algo) < len(publicKeyAlgoName) {
		return publicKeyAlgoName[algo]
	}
	return strconv.Itoa(int(algo))
}

// OIDs for signature algorithms
//
//	pkcs-1 OBJECT IDENTIFIER ::= {
//		iso(1) member-body(2) us(840) rsadsi(113549) pkcs(1) 1 }
//
// RFC 3279 2.2.1 RSA Signature Algorithms
//
//	md5WithRSAEncryption OBJECT IDENTIFIER ::= { pkcs-1 4 }
//
//	sha-1WithRSAEncryption OBJECT IDENTIFIER ::= { pkcs-1 5 }
//
//	dsaWithSha1 OBJECT IDENTIFIER ::= {
//		iso(1) member-body(2) us(840) x9-57(10040) x9cm(4) 3 }
//
// RFC 3279 2.2.3 ECDSA Signature Algorithm
//
//	ecdsa-with-SHA1 OBJECT IDENTIFIER ::= {
//		iso(1) member-body(2) us(840) ansi-x962(10045)
//		signatures(4) ecdsa-with-SHA1(1)}
//
// RFC 4055 5 PKCS #1 Version 1.5
//
//	sha256WithRSAEncryption OBJECT IDENTIFIER ::= { pkcs-1 11 }
//
//	sha384WithRSAEncryption OBJECT IDENTIFIER ::= { pkcs-1 12 }
//
//	sha512WithRSAEncryption OBJECT IDENTIFIER ::= { pkcs-1 13 }
//
// RFC 5758 3.1 DSA Signature Algorithms
//
//	dsaWithSha256 OBJECT IDENTIFIER ::= {
//		joint-iso-ccitt(2) country(16) us(840) organization(1) gov(101)
//		csor(3) algorithms(4) id-dsa-with-sha2(3) 2}
//
// RFC 5758 3.2 ECDSA Signature Algorithm
//
//	ecdsa-with-SHA256 OBJECT IDENTIFIER ::= { iso(1) member-body(2)
//		us(840) ansi-X9-62(10045) signatures(4) ecdsa-with-SHA2(3) 2 }
//
//	ecdsa-with-SHA384 OBJECT IDENTIFIER ::= { iso(1) member-body(2)
//		us(840) ansi-X9-62(10045) signatures(4) ecdsa-with-SHA2(3) 3 }
//
//	ecdsa-with-SHA512 OBJECT IDENTIFIER ::= { iso(1) member-body(2)
//		us(840) ansi-X9-62(10045) signatures(4) ecdsa-with-SHA2(3) 4 }
//
// RFC 8410 3 Curve25519 and Curve448 Algorithm Identifiers
//
//	id-Ed25519   OBJECT IDENTIFIER ::= { 1 3 101 112 }
var (
	oidSignatureMD5WithRSA      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 4}
	oidSignatureSHA1WithRSA     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 5}
	oidSignatureSHA256WithRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}
	oidSignatureSHA384WithRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12}
	oidSignatureSHA512WithRSA   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13}
	oidSignatureRSAPSS          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	oidSignatureDSAWithSHA1     = asn1.ObjectIdentifier{1, 2, 840, 10040, 4, 3}
	oidSignatureDSAWithSHA256   = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 3, 2}
	oidSignatureECDSAWithSHA1   = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 1}
	oidSignatureECDSAWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
	oidSignatureECDSAWithSHA384 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3}
	oidSignatureECDSAWithSHA512 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 4}
	oidSignatureEd25519         = asn1.ObjectIdentifier{1, 3, 101, 112}

	oidSHA256 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidSHA384 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	oidSHA512 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3}

	oidMGF1 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}

	// oidISOSignatureSHA1WithRSA means the same as oidSignatureSHA1WithRSA
	// but it's specified by ISO. Microsoft's makecert.exe has been known
	// to produce certificates with this OID.
	oidISOSignatureSHA1WithRSA = asn1.ObjectIdentifier{1, 3, 14, 3, 2, 29}
)

var signatureAlgorithmDetails = []struct {
	algo       SignatureAlgorithm
	name       string
	oid        asn1.ObjectIdentifier
	params     asn1.RawValue
	pubKeyAlgo PublicKeyAlgorithm
	hash       crypto.Hash
	isRSAPSS   bool
}{
	{MD5WithRSA, "MD5-RSA", oidSignatureMD5WithRSA, asn1.NullRawValue, RSA, crypto.MD5, false},
	{SHA1WithRSA, "SHA1-RSA", oidSignatureSHA1WithRSA, asn1.NullRawValue, RSA, crypto.SHA1, false},
	{SHA1WithRSA, "SHA1-RSA", oidISOSignatureSHA1WithRSA, asn1.NullRawValue, RSA, crypto.SHA1, false},
	{SHA256WithRSA, "SHA256-RSA", oidSignatureSHA256WithRSA, asn1.NullRawValue, RSA, crypto.SHA256, false},
	{SHA384WithRSA, "SHA384-RSA", oidSignatureSHA384WithRSA, asn1.NullRawValue, RSA, crypto.SHA384, false},
	{SHA512WithRSA, "SHA512-RSA", oidSignatureSHA512WithRSA, asn1.NullRawValue, RSA, crypto.SHA512, false},
	{SHA256WithRSAPSS, "SHA256-RSAPSS", oidSignatureRSAPSS, pssParametersSHA256, RSA, crypto.SHA256, true},
	{SHA384WithRSAPSS, "SHA384-RSAPSS", oidSignatureRSAPSS, pssParametersSHA384, RSA, crypto.SHA384, true},
	{SHA512WithRSAPSS, "SHA512-RSAPSS", oidSignatureRSAPSS, pssParametersSHA512, RSA, crypto.SHA512, true},
	{DSAWithSHA1, "DSA-SHA1", oidSignatureDSAWithSHA1, emptyRawValue, DSA, crypto.SHA1, false},
	{DSAWithSHA256, "DSA-SHA256", oidSignatureDSAWithSHA256, emptyRawValue, DSA, crypto.SHA256, false},
	{ECDSAWithSHA1, "ECDSA-SHA1", oidSignatureECDSAWithSHA1, emptyRawValue, ECDSA, crypto.SHA1, false},
	{ECDSAWithSHA256, "ECDSA-SHA256", oidSignatureECDSAWithSHA256, emptyRawValue, ECDSA, crypto.SHA256, false},
	{ECDSAWithSHA384, "ECDSA-SHA384", oidSignatureECDSAWithSHA384, emptyRawValue, ECDSA, crypto.SHA384, false},
	{ECDSAWithSHA512, "ECDSA-SHA512", oidSignatureECDSAWithSHA512, emptyRawValue, ECDSA, crypto.SHA512, false},
	{PureEd25519, "Ed25519", oidSignatureEd25519, emptyRawValue, Ed25519, crypto.Hash(0) /* no pre-hashing */, false},
	{MLDSA44, "ML-DSA-44", oidPublicKeyMLDSA44, emptyRawValue, MLDSA, crypto.Hash(0) /* no pre-hashing */, false},
	{MLDSA65, "ML-DSA-65", oidPublicKeyMLDSA65, emptyRawValue, MLDSA, crypto.Hash(0) /* no pre-hashing */, false},
	{MLDSA87, "ML-DSA-87", oidPublicKeyMLDSA87, emptyRawValue, MLDSA, crypto.Hash(0) /* no pre-hashing */, false},
}

var emptyRawValue = asn1.RawValue{}

// DER encoded RSA PSS parameters for the
// SHA256, SHA384, and SHA512 hashes as defined in RFC 3447, Appendix A.2.3.
// The parameters contain the following values:
//   - hashAlgorithm contains the associated hash identifier with NULL parameters
//   - maskGenAlgorithm always contains the default mgf1SHA1 identifier
//   - saltLength contains the length of the associated hash
//   - trailerField always contains the default trailerFieldBC value
var (
	pssParametersSHA256 = asn1.RawValue{FullBytes: []byte{48, 52, 160, 15, 48, 13, 6, 9, 96, 134, 72, 1, 101, 3, 4, 2, 1, 5, 0, 161, 28, 48, 26, 6, 9, 42, 134, 72, 134, 247, 13, 1, 1, 8, 48, 13, 6, 9, 96, 134, 72, 1, 101, 3, 4, 2, 1, 5, 0, 162, 3, 2, 1, 32}}
	pssParametersSHA384 = asn1.RawValue{FullBytes: []byte{48, 52, 160, 15, 48, 13, 6, 9, 96, 134, 72, 1, 101, 3, 4, 2, 2, 5, 0, 161, 28, 48, 26, 6, 9, 42, 134, 72, 134, 247, 13, 1, 1, 8, 48, 13, 6, 9, 96, 134, 72, 1, 101, 3, 4, 2, 2, 5, 0, 162, 3, 2, 1, 48}}
	pssParametersSHA512 = asn1.RawValue{FullBytes: []byte{48, 52, 160, 15, 48, 13, 6, 9, 96, 134, 72, 1, 101, 3, 4, 2, 3, 5, 0, 161, 28, 48, 26, 6, 9, 42, 134, 72, 134, 247, 13, 1, 1, 8, 48, 13, 6, 9, 96, 134, 72, 1, 101, 3, 4, 2, 3, 5, 0, 162, 3, 2, 1, 64}}
)

// pssParameters reflects the parameters in an AlgorithmIdentifier that
// specifies RSA PSS. See RFC 3447, Appendix A.2.3.
type pssParameters struct {
	// The following three fields are not marked as
	// optional because the default values specify SHA-1,
	// which is no longer suitable for use in signatures.
	Hash         pkix.AlgorithmIdentifier `asn1:"explicit,tag:0"`
	MGF          pkix.AlgorithmIdentifier `asn1:"explicit,tag:1"`
	SaltLength   int                      `asn1:"explicit,tag:2"`
	TrailerField int                      `asn1:"optional,explicit,tag:3,default:1"`
}

func getSignatureAlgorithmFromAI(ai pkix.AlgorithmIdentifier) SignatureAlgorithm {
	if ai.Algorithm.Equal(oidSignatureEd25519) ||
		ai.Algorithm.Equal(oidPublicKeyMLDSA44) ||
		ai.Algorithm.Equal(oidPublicKeyMLDSA65) ||
		ai.Algorithm.Equal(oidPublicKeyMLDSA87) {
		// RFC 8410, Section 3
		// > For all of the OIDs, the parameters MUST be absent.
		// RFC 9881, Section 2
		// > The contents of the parameters component for each algorithm MUST be absent.
		if len(ai.Parameters.FullBytes) != 0 {
			return UnknownSignatureAlgorithm
		}
	}

	if !ai.Algorithm.Equal(oidSignatureRSAPSS) {
		for _, details := range signatureAlgorithmDetails {
			if ai.Algorithm.Equal(details.oid) {
				return details.algo
			}
		}
		return UnknownSignatureAlgorithm
	}

	// RSA PSS is special because it encodes important parameters
	// in the Parameters.

	var params pssParameters
	if _, err := asn1.Unmarshal(ai.Parameters.FullBytes, &params); err != nil {
		return UnknownSignatureAlgorithm
	}

	var mgf1HashFunc pkix.AlgorithmIdentifier
	if _, err := asn1.Unmarshal(params.MGF.Parameters.FullBytes, &mgf1HashFunc); err != nil {
		return UnknownSignatureAlgorithm
	}

	// PSS is greatly overburdened with options. This code forces them into
	// three buckets by requiring that the MGF1 hash function always match the
	// message hash function (as recommended in RFC 3447, Section 8.1), that the
	// salt length matches the hash length, and that the trailer field has the
	// default value.
	if (len(params.Hash.Parameters.FullBytes) != 0 && !bytes.Equal(params.Hash.Parameters.FullBytes, asn1.NullBytes)) ||
		!params.MGF.Algorithm.Equal(oidMGF1) ||
		!mgf1HashFunc.Algorithm.Equal(params.Hash.Algorithm) ||
		(len(mgf1HashFunc.Parameters.FullBytes) != 0 && !bytes.Equal(mgf1HashFunc.Parameters.FullBytes, asn1.NullBytes)) ||
		params.TrailerField != 1 {
		return UnknownSignatureAlgorithm
	}

	switch {
	case params.Hash.Algorithm.Equal(oidSHA256) && params.SaltLength == 32:
		return SHA256WithRSAPSS
	case params.Hash.Algorithm.Equal(oidSHA384) && params.SaltLength == 48:
		return SHA384WithRSAPSS
	case params.Hash.Algorithm.Equal(oidSHA512) && params.SaltLength == 64:
		return SHA512WithRSAPSS
	}

	return UnknownSignatureAlgorithm
}

var (
	// RFC 3279, 2.3 Public Key Algorithms
	//
	//	pkcs-1 OBJECT IDENTIFIER ::== { iso(1) member-body(2) us(840)
	//		rsadsi(113549) pkcs(1) 1 }
	//
	// rsaEncryption OBJECT IDENTIFIER ::== { pkcs1-1 1 }
	//
	//	id-dsa OBJECT IDENTIFIER ::== { iso(1) member-body(2) us(840)
	//		x9-57(10040) x9cm(4) 1 }
	oidPublicKeyRSA = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	oidPublicKeyDSA = asn1.ObjectIdentifier{1, 2, 840, 10040, 4, 1}
	// RFC 5480, 2.1.1 Unrestricted Algorithm Identifier and Parameters
	//
	//	id-ecPublicKey OBJECT IDENTIFIER ::= {
	//		iso(1) member-body(2) us(840) ansi-X9-62(10045) keyType(2) 1 }
	oidPublicKeyECDSA = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
	// RFC 8410, Section 3
	//
	//	id-X25519    OBJECT IDENTIFIER ::= { 1 3 101 110 }
	//	id-Ed25519   OBJECT IDENTIFIER ::= { 1 3 101 112 }
	oidPublicKeyX25519  = asn1.ObjectIdentifier{1, 3, 101, 110}
	oidPublicKeyEd25519 = asn1.ObjectIdentifier{1, 3, 101, 112}
	// RFC 9881, Section 2
	//
	//	id-ml-dsa-44 OBJECT IDENTIFIER ::= { joint-iso-itu-t(2)
	//		country(16) us(840) organization(1) gov(101) csor(3)
	//		nistAlgorithm(4) sigAlgs(3) id-ml-dsa-44(17) }
	//
	//	id-ml-dsa-65 OBJECT IDENTIFIER ::= { joint-iso-itu-t(2)
	//		country(16) us(840) organization(1) gov(101) csor(3)
	//		nistAlgorithm(4) sigAlgs(3) id-ml-dsa-65(18) }
	//
	//	id-ml-dsa-87 OBJECT IDENTIFIER ::= { joint-iso-itu-t(2)
	//		country(16) us(840) organization(1) gov(101) csor(3)
	//		nistAlgorithm(4) sigAlgs(3) id-ml-dsa-87(19) }
	oidPublicKeyMLDSA44 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 3, 17}
	oidPublicKeyMLDSA65 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 3, 18}
	oidPublicKeyMLDSA87 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 3, 19}
)

// getPublicKeyAlgorithmFromOID returns the exposed PublicKeyAlgorithm
// identifier for public key types supported in certificates and CSRs. Marshal
// and Parse functions may support a different set of public key types.
func getPublicKeyAlgorithmFromOID(oid asn1.ObjectIdentifier) PublicKeyAlgorithm {
	switch {
	case oid.Equal(oidPublicKeyRSA):
		return RSA
	case oid.Equal(oidPublicKeyDSA):
		return DSA
	case oid.Equal(oidPublicKeyECDSA):
		return ECDSA
	case oid.Equal(oidPublicKeyEd25519):
		return Ed25519
	case oid.Equal(oidPublicKeyMLDSA44),
		oid.Equal(oidPublicKeyMLDSA65),
		oid.Equal(oidPublicKeyMLDSA87):
		// ML-DSA is not available in FIPS 140-3 module v1.0.0.
		if fips140.Version() == "v1.0.0" {
			return UnknownPublicKeyAlgorithm
		}
		return MLDSA
	}
	return UnknownPublicKeyAlgorithm
}

// RFC 5480, 2.1.1.1. Named Curve
//
//	secp224r1 OBJECT IDENTIFIER ::= {
//	  iso(1) identified-organization(3) certicom(132) curve(0) 33 }
//
//	secp256r1 OBJECT IDENTIFIER ::= {
//	  iso(1) member-body(2) us(840) ansi-X9-62(10045) curves(3)
//	  prime(1) 7 }
//
//	secp384r1 OBJECT IDENTIFIER ::= {
//	  iso(1) identified-organization(3) certicom(132) curve(0) 34 }
//
//	secp521r1 OBJECT IDENTIFIER ::= {
//	  iso(1) identified-organization(3) certicom(132) curve(0) 35 }
//
// NB: secp256r1 is equivalent to prime256v1
var (
	oidNamedCurveP224 = asn1.ObjectIdentifier{1, 3, 132, 0, 33}
	oidNamedCurveP256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 3, 1, 7}
	oidNamedCurveP384 = asn1.ObjectIdentifier{1, 3, 132, 0, 34}
	oidNamedCurveP521 = asn1.ObjectIdentifier{1, 3, 132, 0, 35}
)

func namedCurveFromOID(oid asn1.ObjectIdentifier) elliptic.Curve {
	switch {
	case oid.Equal(oidNamedCurveP224):
		return elliptic.P224()
	case oid.Equal(oidNamedCurveP256):
		return elliptic.P256()
	case oid.Equal(oidNamedCurveP384):
		return elliptic.P384()
	case oid.Equal(oidNamedCurveP521):
		return elliptic.P521()
	}
	return nil
}

func oidFromNamedCurve(curve elliptic.Curve) (asn1.ObjectIdentifier, bool) {
	switch curve {
	case elliptic.P224():
		return oidNamedCurveP224, true
	case elliptic.P256():
		return oidNamedCurveP256, true
	case elliptic.P384():
		return oidNamedCurveP384, true
	case elliptic.P521():
		return oidNamedCurveP521, true
	}

	return nil, false
}

func oidFromECDHCurve(curve ecdh.Curve) (asn1.ObjectIdentifier, bool) {
	switch curve {
	case ecdh.X25519():
		return oidPublicKeyX25519, true
	case ecdh.P256():
		return oidNamedCurveP256, true
	case ecdh.P384():
		return oidNamedCurveP384, true
	case ecdh.P521():
		return oidNamedCurveP521, true
	}

	return nil, false
}

func mldsaParametersFromOID(oid asn1.ObjectIdentifier) (mldsa.Parameters, bool) {
	switch {
	case oid.Equal(oidPublicKeyMLDSA44):
		return mldsa.MLDSA44(), true
	case oid.Equal(oidPublicKeyMLDSA65):
		return mldsa.MLDSA65(), true
	case oid.Equal(oidPublicKeyMLDSA87):
		return mldsa.MLDSA87(), true
	}
	return mldsa.Parameters{}, false
}

func oidFromMLDSAParameters(params mldsa.Parameters) (asn1.ObjectIdentifier, bool) {
	switch {
	case params == mldsa.MLDSA44():
		return oidPublicKeyMLDSA44, true
	case params == mldsa.MLDSA65():
		return oidPublicKeyMLDSA65, true
	case params == mldsa.MLDSA87():
		return oidPublicKeyMLDSA87, true
	}
	return nil, false
}

// KeyUsage represents the set of actions that are valid for a given key. It's
// a bitmap of the KeyUsage* constants.
type KeyUsage int

//go:generate stringer -linecomment -type=KeyUsage,ExtKeyUsage -output=x509_string.go

const (
	KeyUsageDigitalSignature  KeyUsage = 1 << iota // digitalSignature
	KeyUsageContentCommitment                      // contentCommitment
	KeyUsageKeyEncipherment                        // keyEncipherment
	KeyUsageDataEncipherment                       // dataEncipherment
	KeyUsageKeyAgreement                           // keyAgreement
	KeyUsageCertSign                               // keyCertSign
	KeyUsageCRLSign                                // cRLSign
	KeyUsageEncipherOnly                           // encipherOnly
	KeyUsageDecipherOnly                           // decipherOnly
)

// RFC 5280, 4.2.1.12  Extended Key Usage
//
//	anyExtendedKeyUsage OBJECT IDENTIFIER ::= { id-ce-extKeyUsage 0 }
//
//	id-kp OBJECT IDENTIFIER ::= { id-pkix 3 }
//
//	id-kp-serverAuth             OBJECT IDENTIFIER ::= { id-kp 1 }
//	id-kp-clientAuth             OBJECT IDENTIFIER ::= { id-kp 2 }
//	id-kp-codeSigning            OBJECT IDENTIFIER ::= { id-kp 3 }
//	id-kp-emailProtection        OBJECT IDENTIFIER ::= { id-kp 4 }
//	id-kp-timeStamping           OBJECT IDENTIFIER ::= { id-kp 8 }
//	id-kp-OCSPSigning            OBJECT IDENTIFIER ::= { id-kp 9 }
//
// https://www.iana.org/assignments/smi-numbers/smi-numbers.xhtml#smi-numbers-1.3.6.1.5.5.7.3
var (
	oidExtKeyUsageAny                            = asn1.ObjectIdentifier{2, 5, 29, 37, 0}
	oidExtKeyUsageServerAuth                     = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 1}
	oidExtKeyUsageClientAuth                     = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 2}
	oidExtKeyUsageCodeSigning                    = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 3}
	oidExtKeyUsageEmailProtection                = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 4}
	oidExtKeyUsageIPSECEndSystem                 = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 5}
	oidExtKeyUsageIPSECTunnel                    = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 6}
	oidExtKeyUsageIPSECUser                      = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 7}
	oidExtKeyUsageTimeStamping                   = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 8}
	oidExtKeyUsageOCSPSigning                    = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 9}
	oidExtKeyUsageMicrosoftServerGatedCrypto     = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 10, 3, 3}
	oidExtKeyUsageNetscapeServerGatedCrypto      = asn1.ObjectIdentifier{2, 16, 840, 1, 113730, 4, 1}
	oidExtKeyUsageMicrosoftCommercialCodeSigning = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 22}
	oidExtKeyUsageMicrosoftKernelCodeSigning     = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 61, 1, 1}
)

// ExtKeyUsage represents an extended set of actions that are valid for a given key.
// Each of the ExtKeyUsage* constants define a unique action.
type ExtKeyUsage int

const (
	ExtKeyUsageAny                            ExtKeyUsage = iota // anyExtendedKeyUsage
	ExtKeyUsageServerAuth                                        // serverAuth
	ExtKeyUsageClientAuth                                        // clientAuth
	ExtKeyUsageCodeSigning                                       // codeSigning
	ExtKeyUsageEmailProtection                                   // emailProtection
	ExtKeyUsageIPSECEndSystem                                    // ipsecEndSystem
	ExtKeyUsageIPSECTunnel                                       // ipsecTunnel
	ExtKeyUsageIPSECUser                                         // ipsecUser
	ExtKeyUsageTimeStamping                                      // timeStamping
	ExtKeyUsageOCSPSigning                                       // OCSPSigning
	ExtKeyUsageMicrosoftServerGatedCrypto                        // msSGC
	ExtKeyUsageNetscapeServerGatedCrypto                         // nsSGC
	ExtKeyUsageMicrosoftCommercialCodeSigning                    // msCodeCom
	ExtKeyUsageMicrosoftKernelCodeSigning                        // msKernelCode
)

// extKeyUsageOIDs contains the mapping between an ExtKeyUsage and its OID.
var extKeyUsageOIDs = []struct {
	extKeyUsage ExtKeyUsage
	oid         asn1.ObjectIdentifier
}{
	{ExtKeyUsageAny, oidExtKeyUsageAny},
	{ExtKeyUsageServerAuth, oidExtKeyUsageServerAuth},
	{ExtKeyUsageClientAuth, oidExtKeyUsageClientAuth},
	{ExtKeyUsageCodeSigning, oidExtKeyUsageCodeSigning},
	{ExtKeyUsageEmailProtection, oidExtKeyUsageEmailProtection},
	{ExtKeyUsageIPSECEndSystem, oidExtKeyUsageIPSECEndSystem},
	{ExtKeyUsageIPSECTunnel, oidExtKeyUsageIPSECTunnel},
	{ExtKeyUsageIPSECUser, oidExtKeyUsageIPSECUser},
	{ExtKeyUsageTimeStamping, oidExtKeyUsageTimeStamping},
	{ExtKeyUsageOCSPSigning, oidExtKeyUsageOCSPSigning},
	{ExtKeyUsageMicrosoftServerGatedCrypto, oidExtKeyUsageMicrosoftServerGatedCrypto},
	{ExtKeyUsageNetscapeServerGatedCrypto, oidExtKeyUsageNetscapeServerGatedCrypto},
	{ExtKeyUsageMicrosoftCommercialCodeSigning, oidExtKeyUsageMicrosoftCommercialCodeSigning},
	{ExtKeyUsageMicrosoftKernelCodeSigning, oidExtKeyUsageMicrosoftKernelCodeSigning},
}

func extKeyUsageFromOID(oid asn1.ObjectIdentifier) (eku ExtKeyUsage, ok bool) {
	for _, pair := range extKeyUsageOIDs {
		if oid.Equal(pair.oid) {
			return pair.extKeyUsage, true
		}
	}
	return
}

func oidFromExtKeyUsage(eku ExtKeyUsage) (oid asn1.ObjectIdentifier, ok bool) {
	for _, pair := range extKeyUsageOIDs {
		if eku == pair.extKeyUsage {
			return pair.oid, true
		}
	}
	return
}

// OID returns the ASN.1 object identifier of the EKU.
func (eku ExtKeyUsage) OID() OID {
	asn1OID, ok := oidFromExtKeyUsage(eku)
	if !ok {
		panic("x509: internal error: known ExtKeyUsage has no OID")
	}
	oid, err := OIDFromASN1OID(asn1OID)
	if err != nil {
		panic("x509: internal error: known ExtKeyUsage has invalid OID")
	}
	return oid
}

// A Certificate represents an X.509 certificate.
type Certificate struct {
	Raw                     []byte // Complete ASN.1 DER content (certificate, signature algorithm and signature).
	RawTBSCertificate       []byte // Certificate part of raw ASN.1 DER content.
	RawSubjectPublicKeyInfo []byte // DER encoded SubjectPublicKeyInfo.
	RawSubject              []byte // DER encoded Subject
	RawIssuer               []byte // DER encoded Issuer
	RawSignatureAlgorithm   []byte // DER encoded AlgorithmIdentifier

	Signature          []byte
	SignatureAlgorithm SignatureAlgorithm

	PublicKeyAlgorithm PublicKeyAlgorithm
	PublicKey          any

	Version             int
	SerialNumber        *big.Int
	Issuer              pkix.Name
	Subject             pkix.Name
	NotBefore, NotAfter time.Time // Validity bounds.
	KeyUsage            KeyUsage

	// Extensions contains raw X.509 extensions. When parsing certificates,
	// this can be used to extract non-critical extensions that are not
	// parsed by this package. When marshaling certificates, the Extensions
	// field is ignored, see ExtraExtensions.
	Extensions []pkix.Extension

	// ExtraExtensions contains extensions to be copied, raw, into any
	// marshaled certificates. Values override any extensions that would
	// otherwise be produced based on the other fields. The ExtraExtensions
	// field is not populated when parsing certificates, see Extensions.
	ExtraExtensions []pkix.Extension

	// UnhandledCriticalExtensions contains a list of extension IDs that
	// were not (fully) processed when parsing. Verify will fail if this
	// slice is non-empty, unless verification is delegated to an OS
	// library which understands all the critical extensions.
	//
	// Users can access these extensions using Extensions and can remove
	// elements from this slice if they believe that they have been
	// handled.
	UnhandledCriticalExtensions []asn1.ObjectIdentifier

	ExtKeyUsage        []ExtKeyUsage           // Sequence of extended key usages.
	UnknownExtKeyUsage []asn1.ObjectIdentifier // Encountered extended key usages unknown to this package.

	// BasicConstraintsValid indicates whether IsCA, MaxPathLen,
	// and MaxPathLenZero are valid.
	BasicConstraintsValid bool
	IsCA                  bool

	// MaxPathLen and MaxPathLenZero indicate the presence and
	// value of the BasicConstraints' "pathLenConstraint".
	//
	// When parsing a certificate, a positive non-zero MaxPathLen
	// means that the field was specified, -1 means it was unset,
	// and MaxPathLenZero being true mean that the field was
	// explicitly set to zero. The case of MaxPathLen==0 with MaxPathLenZero==false
	// should be treated equivalent to -1 (unset).
	//
	// When generating a certificate, an unset pathLenConstraint
	// can be requested with either MaxPathLen == -1 or using the
	// zero value for both MaxPathLen and MaxPathLenZero.
	MaxPathLen int
	// MaxPathLenZero indicates that BasicConstraintsValid==true
	// and MaxPathLen==0 should be interpreted as an actual
	// maximum path length of zero. Otherwise, that combination is
	// interpreted as MaxPathLen not being set.
	MaxPathLenZero bool

	SubjectKeyId   []byte
	AuthorityKeyId []byte

	// RFC 5280, 4.2.2.1 (Authority Information Access)
	OCSPServer            []string
	IssuingCertificateURL []string

	// Subject Alternate Name values. (Note that these values may not be valid
	// if invalid values were contained within a parsed certificate. For
	// example, an element of DNSNames may not be a valid DNS domain name.)
	DNSNames       []string
	EmailAddresses []string
	IPAddresses    []net.IP
	URIs           []*url.URL

	// Name constraints
	PermittedDNSDomainsCritical bool // if true then the name constraints are marked critical.
	PermittedDNSDomains         []string
	ExcludedDNSDomains          []string
	PermittedIPRanges           []*net.IPNet
	ExcludedIPRanges            []*net.IPNet
	PermittedEmailAddresses     []string
	ExcludedEmailAddresses      []string
	PermittedURIDomains         []string
	ExcludedURIDomains          []string

	// CRL Distribution Points
	CRLDistributionPoints []string

	// PolicyIdentifiers contains asn1.ObjectIdentifiers, the components
	// of which are limited to int32. If a certificate contains a policy which
	// cannot be represented by asn1.ObjectIdentifier, it will not be included in
	// PolicyIdentifiers, but will be present in Policies, which contains all parsed
	// policy OIDs.
	// See CreateCertificate for context about how this field and the Policies field
	// interact.
	PolicyIdentifiers []asn1.ObjectIdentifier

	// Policies contains all policy identifiers included in the certificate.
	// See CreateCertificate for context about how this field and the PolicyIdentifiers field
	// interact.
	// In Go 1.22, encoding/gob cannot handle and ignores this field.
	Policies []OID

	// InhibitAnyPolicy and InhibitAnyPolicyZero indicate the presence and value
	// of the inhibitAnyPolicy extension.
	//
	// The value of InhibitAnyPolicy indicates the number of additional
	// certificates in the path after this certificate that may use the
	// anyPolicy policy OID to indicate a match with any other policy.
	//
	// When parsing a certificate, a positive non-zero InhibitAnyPolicy means
	// that the field was specified, -1 means it was unset, and
	// InhibitAnyPolicyZero being true mean that the field was explicitly set to
	// zero. The case of InhibitAnyPolicy==0 with InhibitAnyPolicyZero==false
	// should be treated equivalent to -1 (unset).
	InhibitAnyPolicy int
	// InhibitAnyPolicyZero indicates that InhibitAnyPolicy==0 should be
	// interpreted as an actual maximum path length of zero. Otherwise, that
	// combination is interpreted as InhibitAnyPolicy not being set.
	InhibitAnyPolicyZero bool

	// InhibitPolicyMapping and InhibitPolicyMappingZero indicate the presence
	// and value of the inhibitPolicyMapping field of the policyConstraints
	// extension.
	//
	// The value of InhibitPolicyMapping indicates the number of additional
	// certificates in the path after this certificate that may use policy
	// mapping.
	//
	// When parsing a certificate, a positive non-zero InhibitPolicyMapping
	// means that the field was specified, -1 means it was unset, and
	// InhibitPolicyMappingZero being true mean that the field was explicitly
	// set to zero. The case of InhibitPolicyMapping==0 with
	// InhibitPolicyMappingZero==false should be treated equivalent to -1
	// (unset).
	InhibitPolicyMapping int
	// InhibitPolicyMappingZero indicates that InhibitPolicyMapping==0 should be
	// interpreted as an actual maximum path length of zero. Otherwise, that
	// combination is interpreted as InhibitAnyPolicy not being set.
	InhibitPolicyMappingZero bool

	// RequireExplicitPolicy and RequireExplicitPolicyZero indicate the presence
	// and value of the requireExplicitPolicy field of the policyConstraints
	// extension.
	//
	// The value of RequireExplicitPolicy indicates the number of additional
	// certificates in the path after this certificate before an explicit policy
	// is required for the rest of the path. When an explicit policy is required,
	// each subsequent certificate in the path must contain a required policy OID,
	// or a policy OID which has been declared as equivalent through the policy
	// mapping extension.
	//
	// When parsing a certificate, a positive non-zero RequireExplicitPolicy
	// means that the field was specified, -1 means it was unset, and
	// RequireExplicitPolicyZero being true mean that the field was explicitly
	// set to zero. The case of RequireExplicitPolicy==0 with
	// RequireExplicitPolicyZero==false should be treated equivalent to -1
	// (unset).
	RequireExplicitPolicy int
	// RequireExplicitPolicyZero indicates that RequireExplicitPolicy==0 should be
	// interpreted as an actual maximum path length of zero. Otherwise, that
	// combination is interpreted as InhibitAnyPolicy not being set.
	RequireExplicitPolicyZero bool

	// PolicyMappings contains a list of policy mappings included in the certificate.
	PolicyMappings []PolicyMapping
}

// PolicyMapping represents a policy mapping entry in the policyMappings extension.
type PolicyMapping struct {
	// IssuerDomainPolicy contains a policy OID the issuing certificate considers
	// equivalent to SubjectDomainPolicy in the subject certificate.
	IssuerDomainPolicy OID
	// SubjectDomainPolicy contains a OID the issuing certificate considers
	// equivalent to IssuerDomainPolicy in the subject certificate.
	SubjectDomainPolicy OID
}

// ErrUnsupportedAlgorithm results from attempting to perform an operation that
// involves algorithms that are not currently implemented.
var ErrUnsupportedAlgorithm = errors.New("x509: cannot verify signature: algorithm unimplemented")

// An InsecureAlgorithmError indicates that the [SignatureAlgorithm] used to
// generate the signature is not secure, and the signature has been rejected.
type InsecureAlgorithmError SignatureAlgorithm

func (e InsecureAlgorithmError) Error() string {
	return fmt.Sprintf("x509: cannot verify signature: insecure algorithm %v", SignatureAlgorithm(e))
}

// ConstraintViolationError results when a requested usage is not permitted by
// a certificate. For example: checking a signature when the public key isn't a
// certificate signing key.
type ConstraintViolationError struct{}

func (ConstraintViolationError) Error() string {
	return "x509: invalid signature: parent certificate cannot sign this kind of certificate"
}

func (c *Certificate) Equal(other *Certificate) bool {
	if c == nil || other == nil {
		return c == other
	}
	return bytes.Equal(c.Raw, other.Raw)
}

func (c *Certificate) hasSANExtension() bool {
	return oidInExtensions(oidExtensionSubjectAltName, c.Extensions)
}

// CheckSignatureFrom verifies that the signature on c is a valid signature from parent.
//
// This is a low-level API that performs very limited checks, and not a full
// path verifier. Most users should use [Certificate.Verify] instead.
func (c *Certificate) CheckSignatureFrom(parent *Certificate) error {
	// RFC 5280, 4.2.1.9:
	// "If the basic constraints extension is not present in a version 3
	// certificate, or the extension is present but the cA boolean is not
	// asserted, then the certified public key MUST NOT be used to verify
	// certificate signatures."
	if parent.Version == 3 && !parent.BasicConstraintsValid ||
		parent.BasicConstraintsValid && !parent.IsCA {
		return ConstraintViolationError{}
	}

	if parent.KeyUsage != 0 && parent.KeyUsage&KeyUsageCertSign == 0 {
		return ConstraintViolationError{}
	}

	if parent.PublicKeyAlgorithm == UnknownPublicKeyAlgorithm {
		return ErrUnsupportedAlgorithm
	}

	return checkSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature, parent.PublicKey, false)
}

// CheckSignature verifies that signature is a valid signature over signed from
// c's public key.
//
// This is a low-level API that performs no validity checks on the certificate.
//
// [MD5WithRSA] signatures are rejected, while [SHA1WithRSA] and [ECDSAWithSHA1]
// signatures are currently accepted.
func (c *Certificate) CheckSignature(algo SignatureAlgorithm, signed, signature []byte) error {
	return checkSignature(algo, signed, signature, c.PublicKey, true)
}

func (c *Certificate) hasNameConstraints() bool {
	return oidInExtensions(oidExtensionNameConstraints, c.Extensions)
}

func (c *Certificate) getSANExtension() []byte {
	for _, e := range c.Extensions {
		if e.Id.Equal(oidExtensionSubjectAltName) {
			return e.Value
		}
	}
	return nil
}

func signaturePublicKeyAlgoMismatchError(expectedPubKeyAlgo PublicKeyAlgorithm, pubKey any) error {
	return fmt.Errorf("x509: signature algorithm specifies an %s public key, but have public key of type %T", expectedPubKeyAlgo.String(), pubKey)
}

func signatureMLDSAParametersMismatchError(expectedSigAlgo SignatureAlgorithm, pubKey *mldsa.PublicKey) error {
	return fmt.Errorf("x509: signature algorithm specifies an ML-DSA public key with %s parameters, but have a public key with %s parameters", expectedSigAlgo, pubKey.Parameters())
}

// checkSignature verifies that signature is a valid signature over signed from
// a crypto.PublicKey.
func checkSignature(algo SignatureAlgorithm, signed, signature []byte, publicKey crypto.PublicKey, allowSHA1 bool) (err error) {
	var hashType crypto.Hash
	var pubKeyAlgo PublicKeyAlgorithm

	for _, details := range signatureAlgorithmDetails {
		if details.algo == algo {
			hashType = details.hash
			pubKeyAlgo = details.pubKeyAlgo
			break
		}
	}

	switch hashType {
	case crypto.Hash(0):
		if pubKeyAlgo != Ed25519 && pubKeyAlgo != MLDSA {
			return ErrUnsupportedAlgorithm
		}
	case crypto.MD5:
		return InsecureAlgorithmError(algo)
	case crypto.SHA1:
		// SHA-1 signatures are only allowed for CRLs and CSRs.
		if !allowSHA1 {
			return InsecureAlgorithmError(algo)
		}
		fallthrough
	default:
		if !hashType.Available() {
			return ErrUnsupportedAlgorithm
		}
		h := hashType.New()
		h.Write(signed)
		signed = h.Sum(nil)
	}

	switch pub := publicKey.(type) {
	case *rsa.PublicKey:
		if pubKeyAlgo != RSA {
			return signaturePublicKeyAlgoMismatchError(pubKeyAlgo, pub)
		}
		if algo.isRSAPSS() {
			return rsa.VerifyPSS(pub, hashType, signed, signature, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
		} else {
			return rsa.VerifyPKCS1v15(pub, hashType, signed, signature)
		}
	case *ecdsa.PublicKey:
		if pubKeyAlgo != ECDSA {
			return signaturePublicKeyAlgoMismatchError(pubKeyAlgo, pub)
		}
		if !ecdsa.VerifyASN1(pub, signed, signature) {
			return errors.New("x509: ECDSA verification failure")
		}
		return
	case ed25519.PublicKey:
		if pubKeyAlgo != Ed25519 {
			return signaturePublicKeyAlgoMismatchError(pubKeyAlgo, pub)
		}
		if !ed25519.Verify(pub, signed, signature) {
			return errors.New("x509: Ed25519 verification failure")
		}
		return
	case *mldsa.PublicKey:
		if pubKeyAlgo != MLDSA {
			return signaturePublicKeyAlgoMismatchError(pubKeyAlgo, pub)
		}
		switch pub.Parameters() {
		case mldsa.MLDSA44():
			if algo != MLDSA44 {
				return signatureMLDSAParametersMismatchError(algo, pub)
			}
		case mldsa.MLDSA65():
			if algo != MLDSA65 {
				return signatureMLDSAParametersMismatchError(algo, pub)
			}
		case mldsa.MLDSA87():
			if algo != MLDSA87 {
				return signatureMLDSAParametersMismatchError(algo, pub)
			}
		default:
			return fmt.Errorf("x509: unknown ML-DSA parameters: %s", pub.Parameters())
		}
		if err := mldsa.Verify(pub, signed, signature, nil); err != nil {
			return fmt.Errorf("x509: ML-DSA verification failure: %w", err)
		}
		return
	}
	return ErrUnsupportedAlgorithm
}

// CheckCRLSignature checks that the signature in crl is from c.
//
// Deprecated: Use [RevocationList.CheckSignatureFrom] instead.
func (c *Certificate) CheckCRLSignature(crl *pkix.CertificateList) error {
	algo := getSignatureAlgorithmFromAI(crl.SignatureAlgorithm)
	return c.CheckSignature(algo, crl.TBSCertList.Raw, crl.SignatureValue.RightAlign())
}

type UnhandledCriticalExtension struct{}

func (h UnhandledCriticalExtension) Error() string {
	return "x509: unhandled critical extension"
}

type basicConstraints struct {
	IsCA       bool `asn1:"optional"`
	MaxPathLen int  `asn1:"optional,default:-1"`
}

// RFC 5280 4.2.1.4
type policyInformation struct {
	Policy asn1.ObjectIdentifier
	// policyQualifiers omitted
}

const (
	nameTypeEmail = 1
	nameTypeDNS   = 2
	nameTypeURI   = 6
	nameTypeIP    = 7
)

// RFC 5280, 4.2.2.1
type authorityInfoAccess struct {
	Method   asn1.ObjectIdentifier
	Location asn1.RawValue
}

// RFC 5280, 4.2.1.14
type distributionPoint struct {
	DistributionPoint distributionPointName `asn1:"optional,tag:0"`
	Reason            asn1.BitString        `asn1:"optional,tag:1"`
	CRLIssuer         asn1.RawValue         `asn1:"optional,tag:2"`
}

type distributionPointName struct {
	FullName     []asn1.RawValue  `asn1:"optional,tag:0"`
	RelativeName pkix.RDNSequence `asn1:"optional,tag:1"`
}

func reverseBitsInAByte(in byte) byte {
	b1 := in>>4 | in<<4
	b2 := b1>>2&0x33 | b1<<2&0xcc
	b3 := b2>>1&0x55 | b2<<1&0xaa
	return b3
}

// asn1BitLength returns the bit-length of bitString by considering the
// most-significant bit in a byte to be the "first" bit. This convention
// matches ASN.1, but differs from almost everything else.
func asn1BitLength(bitString []byte) int {
	bitLen := len(bitString) * 8

	for i := range bitString {
		b := bitString[len(bitString)-i-1]

		for bit := uint(0); bit < 8; bit++ {
			if (b>>bit)&1 == 1 {
				return bitLen
			}
			bitLen--
		}
	}

	return 0
}

var (
	oidExtensionSubjectKeyId          = []int{2, 5, 29, 14}
	oidExtensionKeyUsage              = []int{2, 5, 29, 15}
	oidExtensionExtendedKeyUsage      = []int{2, 5, 29, 37}
	oidExtensionAuthorityKeyId        = []int{2, 5, 29, 35}
	oidExtensionBasicConstraints      = []int{2, 5, 29, 19}
	oidExtensionSubjectAltName        = []int{2, 5, 29, 17}
	oidExtensionCertificatePolicies   = []int{2, 5, 29, 32}
	oidExtensionNameConstraints       = []int{2, 5, 29, 30}
	oidExtensionCRLDistributionPoints = []int{2, 5, 29, 31}
	oidExtensionAuthorityInfoAccess   = []int{1, 3, 6, 1, 5, 5, 7, 1, 1}
	oidExtensionCRLNumber             = []int{2, 5, 29, 20}
	oidExtensionReasonCode            = []int{2, 5, 29, 21}
)

var (
	oidAuthorityInfoAccessOcsp    = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 48, 1}
	oidAuthorityInfoAccessIssuers = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 48, 2}
)

// oidInExtensions reports whether an extension with the given oid exists in
// extensions.
func oidInExtensions(oid asn1.ObjectIdentifier, extensions []pkix.Extension) bool {
	for _, e := range extensions {
		if e.Id.Equal(oid) {
			return true
		}
	}
	return false
}

// marshalSANs marshals a list of addresses into a the contents of an X.509
// SubjectAlternativeName extension.
func marshalSANs(dnsNames, emailAddresses []string, ipAddresses []net.IP, uris []*url.URL) (derBytes []byte, err error) {
	var rawValues []asn1.RawValue
	for _, name := range dnsNames {
		if err := isIA5String(name); err != nil {
			return nil, err
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: nameTypeDNS, Class: 2, Bytes: []byte(name)})
	}
	for _, email := range emailAddresses {
		if err := isIA5String(email); err != nil {
			return nil, err
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: nameTypeEmail, Class: 2, Bytes: []byte(email)})
	}
	for _, rawIP := range ipAddresses {
		// If possible, we always want to encode IPv4 addresses in 4 bytes.
		ip := rawIP.To4()
		if ip == nil {
			ip = rawIP
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: nameTypeIP, Class: 2, Bytes: ip})
	}
	for _, uri := range uris {
		uriStr := uri.String()
		if err := isIA5String(uriStr); err != nil {
			return nil, err
		}
		rawValues = append(rawValues, asn1.RawValue{Tag: nameTypeURI, Class: 2, Bytes: []byte(uriStr)})
	}
	return asn1.Marshal(rawValues)
}

func isIA5String(s string) error {
	for _, r := range s {
		// Per RFC5280 "IA5String is limited to the set of ASCII characters"
		if r > unicode.MaxASCII {
			return fmt.Errorf("x509: %q cannot be encoded as an IA5String", s)
		}
	}

	return nil
}

var x509usepolicies = godebug.New("x509usepolicies")

func buildCertExtensions(template *Certificate, subjectIsEmpty bool, authorityKeyId []byte, subjectKeyId []byte) (ret []pkix.Extension, err error) {
	ret = make([]pkix.Extension, 10 /* maximum number of elements. */)
	n := 0

	if template.KeyUsage != 0 &&
		!oidInExtensions(oidExtensionKeyUsage, template.ExtraExtensions) {
		ret[n], err = marshalKeyUsage(template.KeyUsage)
		if err != nil {
			return nil, err
		}
		n++
	}

	if (len(template.ExtKeyUsage) > 0 || len(template.UnknownExtKeyUsage) > 0) &&
		!oidInExtensions(oidExtensionExtendedKeyUsage, template.ExtraExtensions) {
		ret[n], err = marshalExtKeyUsage(template.ExtKeyUsage, template.UnknownExtKeyUsage)
		if err != nil {
			return nil, err
		}
		n++
	}

	if template.BasicConstraintsValid && !oidInExtensions(oidExtensionBasicConstraints, template.ExtraExtensions) {
		ret[n], err = marshalBasicConstraints(template.IsCA, template.MaxPathLen, template.MaxPathLenZero)
		if err != nil {
			return nil, err
		}
		n++
	}

	if len(subjectKeyId) > 0 && !oidInExtensions(oidExtensionSubjectKeyId, template.ExtraExtensions) {
		ret[n].Id = oidExtensionSubjectKeyId
		ret[n].Value, err = asn1.Marshal(subjectKeyId)
		if err != nil {
			return
		}
		n++
	}

	if len(authorityKeyId) > 0 && !oidInExtensions(oidExtensionAuthorityKeyId, template.ExtraExtensions) {
		ret[n].Id = oidExtensionAuthorityKeyId
		ret[n].Value, err = asn1.Marshal(authKeyId{authorityKeyId})
		if err != nil {
			return
		}
		n++
	}

	if (len(template.OCSPServer) > 0 || len(template.IssuingCertificateURL) > 0) &&
		!oidInExtensions(oidExtensionAuthorityInfoAccess, template.ExtraExtensions) {
		ret[n].Id = oidExtensionAuthorityInfoAccess
		var aiaValues []authorityInfoAccess
		for _, name := range template.OCSPServer {
			aiaValues = append(aiaValues, authorityInfoAccess{
				Method:   oidAuthorityInfoAccessOcsp,
				Location: asn1.RawValue{Tag: 6, Class: 2, Bytes: []byte(name)},
			})
		}
		for _, name := range template.IssuingCertificateURL {
			aiaValues = append(aiaValues, authorityInfoAccess{
				Method:   oidAuthorityInfoAccessIssuers,
				Location: asn1.RawValue{Tag: 6, Class: 2, Bytes: []byte(name)},
			})
		}
		ret[n].Value, err = asn1.Marshal(aiaValues)
		if err != nil {
			return
		}
		n++
	}

	if (len(template.DNSNames) > 0 || len(template.EmailAddresses) > 0 || len(template.IPAddresses) > 0 || len(template.URIs) > 0) &&
		!oidInExtensions(oidExtensionSubjectAltName, template.ExtraExtensions) {
		ret[n].Id = oidExtensionSubjectAltName
		// From RFC 5280, Section 4.2.1.6:
		// “If the subject field contains an empty sequence ... then
		// subjectAltName extension ... is marked as critical”
		ret[n].Critical = subjectIsEmpty
		ret[n].Value, err = marshalSANs(template.DNSNames, template.EmailAddresses, template.IPAddresses, template.URIs)
		if err != nil {
			return
		}
		n++
	}

	usePolicies := x509usepolicies.Value() != "0"
	if ((!usePolicies && len(template.PolicyIdentifiers) > 0) || (usePolicies && len(template.Policies) > 0)) &&
		!oidInExtensions(oidExtensionCertificatePolicies, template.ExtraExtensions) {
		ret[n], err = marshalCertificatePolicies(template.Policies, template.PolicyIdentifiers)
		if err != nil {
			return nil, err
		}
		n++
	}

	if (len(template.PermittedDNSDomains) > 0 || len(template.ExcludedDNSDomains) > 0 ||
		len(template.PermittedIPRanges) > 0 || len(template.ExcludedIPRanges) > 0 ||
		len(template.PermittedEmailAddresses) > 0 || len(template.ExcludedEmailAddresses) > 0 ||
		len(template.PermittedURIDomains) > 0 || len(template.ExcludedURIDomains) > 0) &&
		!oidInExtensions(oidExtensionNameConstraints, template.ExtraExtensions) {
		ret[n].Id = oidExtensionNameConstraints
		ret[n].Critical = template.PermittedDNSDomainsCritical

		ipAndMask := func(ipNet *net.IPNet) ([]byte, error) {
			maskedIP := ipNet.IP.Mask(ipNet.Mask)
			// This is extremely unlikely to actually happen, but lets save people from doing something they
			// probably shouldn't.
			if len(maskedIP) == net.IPv6len && maskedIP.To4() != nil {
				return nil, errors.New("x509: IP constraint contained IPv4-mapped IPv6 address with a IPv6 mask")
			}
			ipAndMask := make([]byte, 0, len(maskedIP)+len(ipNet.Mask))
			ipAndMask = append(ipAndMask, maskedIP...)
			ipAndMask = append(ipAndMask, ipNet.Mask...)
			return ipAndMask, nil
		}

		serialiseConstraints := func(dns []string, ips []*net.IPNet, emails []string, uriDomains []string) (der []byte, err error) {
			var b cryptobyte.Builder

			for _, name := range dns {
				if err = isIA5String(name); err != nil {
					return nil, err
				}

				b.AddASN1(cryptobyte_asn1.SEQUENCE, func(b *cryptobyte.Builder) {
					b.AddASN1(cryptobyte_asn1.Tag(2).ContextSpecific(), func(b *cryptobyte.Builder) {
						b.AddBytes([]byte(name))
					})
				})
			}

			for _, ipNet := range ips {
				encodedIPNet, err := ipAndMask(ipNet)
				if err != nil {
					return nil, err
				}
				b.AddASN1(cryptobyte_asn1.SEQUENCE, func(b *cryptobyte.Builder) {
					b.AddASN1(cryptobyte_asn1.Tag(7).ContextSpecific(), func(b *cryptobyte.Builder) {
						b.AddBytes(encodedIPNet)
					})
				})
			}

			for _, email := range emails {
				if err = isIA5String(email); err != nil {
					return nil, err
				}

				b.AddASN1(cryptobyte_asn1.SEQUENCE, func(b *cryptobyte.Builder) {
					b.AddASN1(cryptobyte_asn1.Tag(1).ContextSpecific(), func(b *cryptobyte.Builder) {
						b.AddBytes([]byte(email))
					})
				})
			}

			for _, uriDomain := range uriDomains {
				if err = isIA5String(uriDomain); err != nil {
					return nil, err
				}

				b.AddASN1(cryptobyte_asn1.SEQUENCE, func(b *cryptobyte.Builder) {
					b.AddASN1(cryptobyte_asn1.Tag(6).ContextSpecific(), func(b *cryptobyte.Builder) {
						b.AddBytes([]byte(uriDomain))
					})
				})
			}

			return b.Bytes()
		}

		permitted, err := serialiseConstraints(template.PermittedDNSDomains, template.PermittedIPRanges, template.PermittedEmailAddresses, template.PermittedURIDomains)
		if err != nil {
			return nil, err
		}

		excluded, err := serialiseConstraints(template.ExcludedDNSDomains, template.ExcludedIPRanges, template.ExcludedEmailAddresses, template.ExcludedURIDomains)
		if err != nil {
			return nil, err
		}

		var b cryptobyte.Builder
		b.AddASN1(cryptobyte_asn1.SEQUENCE, func(b *cryptobyte.Builder) {
			if len(permitted) > 0 {
				b.AddASN1(cryptobyte_asn1.Tag(0).ContextSpecific().Constructed(), func(b *cryptobyte.Builder) {
					b.AddBytes(permitted)
				})
			}

			if len(excluded) > 0 {
				b.AddASN1(cryptobyte_asn1.Tag(1).ContextSpecific().Constructed(), func(b *cryptobyte.Builder) {
					b.AddBytes(excluded)
				})
			}
		})

		ret[n].Value, err = b.Bytes()
		if err != nil {
			return nil, err
		}
		n++
	}

	if len(template.CRLDistributionPoints) > 0 &&
		!oidInExtensions(oidExtensionCRLDistributionPoints, template.ExtraExtensions) {
		ret[n].Id = oidExtensionCRLDistributionPoints

		var crlDp []distributionPoint
		for _, name := range template.CRLDistributionPoints {
			dp := distributionPoint{
				DistributionPoint: distributionPointName{
					FullName: []asn1.RawValue{
						{Tag: 6, Class: 2, Bytes: []byte(name)},
					},
				},
			}
			crlDp = append(crlDp, dp)
		}

		ret[n].Value, err = asn1.Marshal(crlDp)
		if err != nil {
			return
		}
		n++
	}

	// Adding another extension here? Remember to update the maximum number
	// of elements in the make() at the top of the function and the list of
	// template fields used in CreateCertificate documentation.

	return append(ret[:n], template.ExtraExtensions...), nil
}

func marshalKeyUsage(ku KeyUsage) (pkix.Extension, error) {
	ext := pkix.Extension{Id: oidExtensionKeyUsage, Critical: true}

	var a [2]byte
	a[0] = reverseBitsInAByte(byte(ku))
	a[1] = reverseBitsInAByte(byte(ku >> 8))

	l := 1
	if a[1] != 0 {
		l = 2
	}

	bitString := a[:l]
	var err error
	ext.Value, err = asn1.Marshal(asn1.BitString{Bytes: bitString, BitLength: asn1BitLength(bitString)})
	return ext, err
}

func marshalExtKeyUsage(extUsages []ExtKeyUsage, unknownUsages []asn1.ObjectIdentifier) (pkix.Extension, error) {
	ext := pkix.Extension{Id: oidExtensionExtendedKeyUsage}

	oids := make([]asn1.ObjectIdentifier, len(extUsages)+len(unknownUsages))
	for i, u := range extUsages {
		if oid, ok := oidFromExtKeyUsage(u); ok {
			oids[i] = oid
		} else {
			return ext, errors.New("x509: unknown extended key usage")
		}
	}

	copy(oids[len(extUsages):], unknownUsages)

	var err error
	ext.Value, err = asn1.Marshal(oids)
	return ext, err
}

func marshalBasicConstraints(isCA bool, maxPathLen int, maxPathLenZero bool) (pkix.Extension, error) {
	ext := pkix.Extension{Id: oidExtensionBasicConstraints, Critical: true}
	// Leaving MaxPathLen as zero indicates that no maximum path
	// length is desired, unless MaxPathLenZero is set. A value of
	// -1 causes encoding/asn1 to omit the value as desired.
	if maxPathLen == 0 && !maxPathLenZero {
		maxPathLen = -1
	}
	var err error
	ext.Value, err = asn1.Marshal(basicConstraints{isCA, maxPathLen})
	return ext, err
}

func marshalCertificatePolicies(policies []OID, policyIdentifiers []asn1.ObjectIdentifier) (pkix.Extension, error) {
	ext := pkix.Extension{Id: oidExtensionCertificatePolicies}

	b := cryptobyte.NewBuilder(make([]byte, 0, 128))
	b.AddASN1(cryptobyte_asn1.SEQUENCE, func(child *cryptobyte.Builder) {
		if x509usepolicies.Value() != "0" {
			x509usepolicies.IncNonDefault()
			for _, v := range policies {
				child.AddASN1(cryptobyte_asn1.SEQUENCE, func(child *cryptobyte.Builder) {
					child.AddASN1(cryptobyte_asn1.OBJECT_IDENTIFIER, func(child *cryptobyte.Builder) {
						if len(v.der) == 0 {
							child.SetError(errors.New("invalid policy object identifier"))
							return
						}
						child.AddBytes(v.der)
					})
				})
			}
		} else {
			for _, v := range policyIdentifiers {
				child.AddASN1(cryptobyte_asn1.SEQUENCE, func(child *cryptobyte.Builder) {
					child.AddASN1ObjectIdentifier(v)
				})
			}
		}
	})

	var err error
	ext.Value, err = b.Bytes()
	return ext, err
}

func buildCSRExtensions(template *CertificateRequest) ([]pkix.Extension, error) {
	var ret []pkix.Extension

	if (len(template.DNSNames) > 0 || len(template.EmailAddresses) > 0 || len(template.IPAddresses) > 0 || len(template.URIs) > 0) &&
		!oidInExtensions(oidExtensionSubjectAltName, template.ExtraExtensions) {
		sanBytes, err := marshalSANs(template.DNSNames, template.EmailAddresses, template.IPAddresses, template.URIs)
		if err != nil {
			return nil, err
		}

		ret = append(ret, pkix.Extension{
			Id:    oidExtensionSubjectAltName,
			Value: sanBytes,
		})
	}

	return append(ret, template.ExtraExtensions...), nil
}

func subjectBytes(cert *Certificate) ([]byte, error) {
	if len(cert.RawSubject) > 0 {
		return cert.RawSubject, nil
	}

	return asn1.Marshal(cert.Subject.ToRDNSequence())
}

// signingParamsForKey returns the signature algorithm and its Algorithm
// Identifier to use for signing, based on the key type. If sigAlgo is not zero
// then it overrides the default.
func signingParamsForKey(key crypto.Signer, sigAlgo SignatureAlgorithm) (SignatureAlgorithm, pkix.AlgorithmIdentifier, error) {
	var ai pkix.AlgorithmIdentifier
	var pubType PublicKeyAlgorithm
	var defaultAlgo SignatureAlgorithm

	switch pub := key.Public().(type) {
	case *rsa.PublicKey:
		pubType = RSA
		defaultAlgo = SHA256WithRSA

	case *ecdsa.PublicKey:
		pubType = ECDSA
		switch pub.Curve {
		case elliptic.P224(), elliptic.P256():
			defaultAlgo = ECDSAWithSHA256
		case elliptic.P384():
			defaultAlgo = ECDSAWithSHA384
		case elliptic.P521():
			defaultAlgo = ECDSAWithSHA512
		default:
			return 0, ai, errors.New("x509: unsupported elliptic curve")
		}

	case ed25519.PublicKey:
		pubType = Ed25519
		defaultAlgo = PureEd25519

	case *mldsa.PublicKey:
		pubType = MLDSA
		switch pub.Parameters() {
		case mldsa.MLDSA44():
			defaultAlgo = MLDSA44
		case mldsa.MLDSA65():
			defaultAlgo = MLDSA65
		case mldsa.MLDSA87():
			defaultAlgo = MLDSA87
		default:
			return 0, ai, fmt.Errorf("x509: unsupported ML-DSA parameters: %s", pub.Parameters())
		}

	default:
		return 0, ai, errors.New("x509: only RSA, ECDSA, ML-DSA and Ed25519 keys supported")
	}

	if sigAlgo == 0 {
		sigAlgo = defaultAlgo
	}

	for _, details := range signatureAlgorithmDetails {
		if details.algo == sigAlgo {
			if details.pubKeyAlgo != pubType {
				return 0, ai, errors.New("x509: requested SignatureAlgorithm does not match private key type")
			}
			if pubType == MLDSA && sigAlgo != defaultAlgo {
				return 0, ai, errors.New("x509: requested SignatureAlgorithm does not match ML-DSA parameters")
			}
			if details.hash == crypto.MD5 {
				return 0, ai, errors.New("x509: signing with MD5 is not supported")
			}

			return sigAlgo, pkix.AlgorithmIdentifier{
				Algorithm:  details.oid,
				Parameters: details.params,
			}, nil
		}
	}

	return 0, ai, errors.New("x509: unknown SignatureAlgorithm")
}

func signTBS(tbs []byte, key crypto.Signer, sigAlg SignatureAlgorithm, rand io.Reader) ([]byte, error) {
	hashFunc := sigAlg.hashFunc()

	var signerOpts crypto.SignerOpts = hashFunc
	if sigAlg.isRSAPSS() {
		signerOpts = &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hashFunc,
		}
	}

	signature, err := crypto.SignMessage(key, rand, tbs, signerOpts)
	if err != nil {
		return nil, err
	}

	// Check the signature to ensure the crypto.Signer behaved correctly.
	if err := checkSignature(sigAlg, tbs, signature, key.Public(), true); err != nil {
		return nil, fmt.Errorf("x509: signature returned by signer is invalid: %w", err)
	}

	return signature, nil
}

// emptyASN1Subject is the ASN.1 DER encoding of an empty Subject, which is
// just an empty SEQUENCE.
var emptyASN1Subject = []byte{0x30, 0}

// CreateCertificate creates a new X.509 v3 certificate based on a template.
// The following members of template are currently used:
//
//   - AuthorityKeyId
//   - BasicConstraintsValid
//   - CRLDistributionPoints
//   - DNSNames
//   - EmailAddresses
//   - ExcludedDNSDomains
//   - ExcludedEmailAddresses
//   - ExcludedIPRanges
//   - ExcludedURIDomains
//   - ExtKeyUsage
//   - ExtraExtensions
//   - IPAddresses
//   - IsCA
//   - IssuingCertificateURL
//   - KeyUsage
//   - MaxPathLen
//   - MaxPathLenZero
//   - NotAfter
//   - NotBefore
//   - OCSPServer
//   - PermittedDNSDomains
//   - PermittedDNSDomainsCritical
//   - PermittedEmailAddresses
//   - PermittedIPRanges
//   - PermittedURIDomains
//   - PolicyIdentifiers (see note below)
//   - Policies (see note below)
//   - SerialNumber
//   - SignatureAlgorithm
//   - Subject
//   - SubjectKeyId
//   - URIs
//   - UnknownExtKeyUsage
//
// The certificate is signed by parent. If parent is equal to template then the
// certificate is self-signed. The parameter pub is the public key of the
// certificate to be generated and priv is the private key of the signer.
//
// The returned slice is the certificate in DER encoding.
//
// The currently supported key types are *rsa.PublicKey, *ecdsa.PublicKey,
// ed25519.PublicKey, and *mldsa.PublicKey. pub must be a supported key type,
// and priv must be a crypto.Signer or crypto.MessageSigner with a supported
// public key.
//
// The AuthorityKeyId will be taken from the SubjectKeyId of parent, if any,
// unless the resulting certificate is self-signed. Otherwise the value from
// template will be used.
//
// If SubjectKeyId from template is empty and the template is a CA, SubjectKeyId
// will be generated from the hash of the public key.
//
// If template.SerialNumber is nil, a serial number will be generated which
// conforms to RFC 5280, Section 4.1.2.2 using entropy from rand.
//
// The PolicyIdentifier and Policies fields can both be used to marshal certificate
// policy OIDs. By default, only the Policies is marshaled, but if the
// GODEBUG setting "x509usepolicies" has the value "0", the PolicyIdentifiers field will
// be marshaled instead of the Policies field. This changed in Go 1.24. The Policies field can
// be used to marshal policy OIDs which have components that are larger than 31
// bits.
//
// IP addresses in IPAddresses which are in their IPv4-mapped IPv6 form will always be encoded
// in their IPv4 form.
func CreateCertificate(rand io.Reader, template, parent *Certificate, pub, priv any) ([]byte, error) {
	key, ok := priv.(crypto.Signer)
	if !ok {
		return nil, errors.New("x509: certificate private key does not implement crypto.Signer")
	}

	serialNumber := template.SerialNumber
	if serialNumber == nil {
		// Generate a serial number following RFC 5280, Section 4.1.2.2 if one
		// is not provided. The serial number must be positive and at most 20
		// octets *when encoded*.
		serialBytes := make([]byte, 20)
		if _, err := io.ReadFull(rand, serialBytes); err != nil {
			return nil, err
		}
		// If the top bit is set, the serial will be padded with a leading zero
		// byte during encoding, so that it's not interpreted as a negative
		// integer. This padding would make the serial 21 octets so we clear the
		// top bit to ensure the correct length in all cases.
		serialBytes[0] &= 0b0111_1111
		serialNumber = new(big.Int).SetBytes(serialBytes)
	}

	// RFC 5280 Section 4.1.2.2: serial number must be positive
	//
	// We _should_ also restrict serials to <= 20 octets, but it turns out a lot of people
	// get this wrong, in part because the encoding can itself alter the length of the
	// serial. For now we accept these non-conformant serials.
	if serialNumber.Sign() == -1 {
		return nil, errors.New("x509: serial number must be positive")
	}

	if template.BasicConstraintsValid && template.MaxPathLen < -1 {
		return nil, errors.New("x509: invalid MaxPathLen, must be greater or equal to -1")
	}

	if template.BasicConstraintsValid && !template.IsCA && template.MaxPathLen != -1 && (template.MaxPathLen != 0 || template.MaxPathLenZero) {
		return nil, errors.New("x509: only CAs are allowed to specify MaxPathLen")
	}

	signatureAlgorithm, algorithmIdentifier, err := signingParamsForKey(key, template.SignatureAlgorithm)
	if err != nil {
		return nil, err
	}

	publicKeyBytes, publicKeyAlgorithm, err := marshalPublicKey(pub)
	if err != nil {
		return nil, err
	}
	if getPublicKeyAlgorithmFromOID(publicKeyAlgorithm.Algorithm) == UnknownPublicKeyAlgorithm {
		return nil, fmt.Errorf("x509: unsupported public key type: %T", pub)
	}

	asn1Issuer, err := subjectBytes(parent)
	if err != nil {
		return nil, err
	}

	asn1Subject, err := subjectBytes(template)
	if err != nil {
		return nil, err
	}

	authorityKeyId := template.AuthorityKeyId
	if !bytes.Equal(asn1Issuer, asn1Subject) && len(parent.SubjectKeyId) > 0 {
		authorityKeyId = parent.SubjectKeyId
	}

	subjectKeyId := template.SubjectKeyId
	if len(subjectKeyId) == 0 && template.IsCA {
		if x509sha256skid.Value() == "0" {
			x509sha256skid.IncNonDefault()
			// SubjectKeyId generated using method 1 in RFC 5280, Section 4.2.1.2:
			//   (1) The keyIdentifier is composed of the 160-bit SHA-1 hash of the
			//   value of the BIT STRING subjectPublicKey (excluding the tag,
			//   length, and number of unused bits).
			h := sha1.Sum(publicKeyBytes)
			subjectKeyId = h[:]
		} else {
			// SubjectKeyId generated using method 1 in RFC 7093, Section 2:
			//    1) The keyIdentifier is composed of the leftmost 160-bits of the
			//    SHA-256 hash of the value of the BIT STRING subjectPublicKey
			//    (excluding the tag, length, and number of unused bits).
			h := sha256.Sum256(publicKeyBytes)
			subjectKeyId = h[:20]
		}
	}

	// Check that the signer's public key matches the private key, if available.
	type privateKey interface {
		Equal(crypto.PublicKey) bool
	}
	if privPub, ok := key.Public().(privateKey); !ok {
		return nil, errors.New("x509: internal error: supported public key does not implement Equal")
	} else if parent.PublicKey != nil && !privPub.Equal(parent.PublicKey) {
		return nil, errors.New("x509: provided PrivateKey doesn't match parent's PublicKey")
	}

	extensions, err := buildCertExtensions(template, bytes.Equal(asn1Subject, emptyASN1Subject), authorityKeyId, subjectKeyId)
	if err != nil {
		return nil, err
	}

	encodedPublicKey := asn1.BitString{BitLength: len(publicKeyBytes) * 8, Bytes: publicKeyBytes}
	c := tbsCertificate{
		Version:            2,
		SerialNumber:       serialNumber,
		SignatureAlgorithm: algorithmIdentifier,
		Issuer:             asn1.RawValue{FullBytes: asn1Issuer},
		Validity:           validity{template.NotBefore.UTC(), template.NotAfter.UTC()},
		Subject:            asn1.RawValue{FullBytes: asn1Subject},
		PublicKey:          publicKeyInfo{nil, publicKeyAlgorithm, encodedPublicKey},
		Extensions:         extensions,
	}

	tbsCertContents, err := asn1.Marshal(c)
	if err != nil {
		return nil, err
	}
	c.Raw = tbsCertContents

	signature, err := signTBS(tbsCertContents, key, signatureAlgorithm, rand)
	if err != nil {
		return nil, err
	}

	return asn1.Marshal(certificate{
		TBSCertificate:     c,
		SignatureAlgorithm: algorithmIdentifier,
		SignatureValue:     asn1.BitString{Bytes: signature, BitLength: len(signature) * 8},
	})
}

var x509sha256skid = godebug.New("x509sha256skid")

// pemCRLPrefix is the magic string that indicates that we have a PEM encoded
// CRL.
var pemCRLPrefix = []byte("-----BEGIN X509 CRL")

// pemType is the type of a PEM encoded CRL.
var pemType = "X509 CRL"

// ParseCRL parses a CRL from the given bytes. It's often the case that PEM
// encoded CRLs will appear where they should be DER encoded, so this function
// will transparently handle PEM encoding as long as there isn't any leading
// garbage.
//
// Deprecated: Use [ParseRevocationList] instead.
func ParseCRL(crlBytes []byte) (*pkix.CertificateList, error) {
	if bytes.HasPrefix(crlBytes, pemCRLPrefix) {
		block, _ := pem.Decode(crlBytes)
		if block != nil && block.Type == pemType {
			crlBytes = block.Bytes
		}
	}
	return ParseDERCRL(crlBytes)
}

// ParseDERCRL parses a DER encoded CRL from the given bytes.
//
// Deprecated: Use [ParseRevocationList] instead.
func ParseDERCRL(derBytes []byte) (*pkix.CertificateList, error) {
	certList := new(pkix.CertificateList)
	if rest, err := asn1.Unmarshal(derBytes, certList); err != nil {
		return nil, err
	} else if len(rest) != 0 {
		return nil, errors.New("x509: trailing data after CRL")
	}
	return certList, nil
}

// CreateCRL returns a DER encoded CRL, signed by this Certificate, that
// contains the given list of revoked certificates.
//
// Deprecated: this method does not generate an RFC 5280 conformant X.509 v2 CRL.
// To generate a standards compliant CRL, use [CreateRevocationList] instead.
func (c *Certificate) CreateCRL(rand io.Reader, priv any, revokedCerts []pkix.RevokedCertificate, now, expiry time.Time) (crlBytes []byte, err error) {
	key, ok := priv.(crypto.Signer)
	if !ok {
		return nil, errors.New("x509: certificate private key does not implement crypto.Signer")
	}

	signatureAlgorithm, algorithmIdentifier, err := signingParamsForKey(key, 0)
	if err != nil {
		return nil, err
	}

	// Force revocation times to UTC per RFC 5280.
	revokedCertsUTC := make([]pkix.RevokedCertificate, len(revokedCerts))
	for i, rc := range revokedCerts {
		rc.RevocationTime = rc.RevocationTime.UTC()
		revokedCertsUTC[i] = rc
	}

	tbsCertList := pkix.TBSCertificateList{
		Version:             1,
		Signature:           algorithmIdentifier,
		Issuer:              c.Subject.ToRDNSequence(),
		ThisUpdate:          now.UTC(),
		NextUpdate:          expiry.UTC(),
		RevokedCertificates: revokedCertsUTC,
	}

	// Authority Key Id
	if len(c.SubjectKeyId) > 0 {
		var aki pkix.Extension
		aki.Id = oidExtensionAuthorityKeyId
		aki.Value, err = asn1.Marshal(authKeyId{Id: c.SubjectKeyId})
		if err != nil {
			return nil, err
		}
		tbsCertList.Extensions = append(tbsCertList.Extensions, aki)
	}

	tbsCertListContents, err := asn1.Marshal(tbsCertList)
	if err != nil {
		return nil, err
	}
	tbsCertList.Raw = tbsCertListContents

	signature, err := signTBS(tbsCertListContents, key, signatureAlgorithm, rand)
	if err != nil {
		return nil, err
	}

	return asn1.Marshal(pkix.CertificateList{
		TBSCertList:        tbsCertList,
		SignatureAlgorithm: algorithmIdentifier,
		SignatureValue:     asn1.BitString{Bytes: signature, BitLength: len(signature) * 8},
	})
}

// CertificateRequest represents a PKCS #10, certificate signature request.
type CertificateRequest struct {
	Raw                      []byte // Complete ASN.1 DER content (CSR, signature algorithm and signature).
	RawTBSCertificateRequest []byte // Certificate request info part of raw ASN.1 DER content.
	RawSubjectPublicKeyInfo  []byte // DER encoded SubjectPublicKeyInfo.
	RawSubject               []byte // DER encoded Subject.
	RawSignatureAlgorithm    []byte // DER encoded AlgorithmIdentifier.

	Version            int
	Signature          []byte
	SignatureAlgorithm SignatureAlgorithm

	PublicKeyAlgorithm PublicKeyAlgorithm
	PublicKey          any

	Subject pkix.Name

	// Attributes contains the CSR attributes that can parse as
	// pkix.AttributeTypeAndValueSET.
	//
	// Deprecated: Use Extensions and ExtraExtensions instead for parsing and
	// generating the requestedExtensions attribute.
	Attributes []pkix.AttributeTypeAndValueSET

	// Extensions contains all requested extensions, in raw form. When parsing
	// CSRs, this can be used to extract extensions that are not parsed by this
	// package.
	Extensions []pkix.Extension

	// ExtraExtensions contains extensions to be copied, raw, into any CSR
	// marshaled by CreateCertificateRequest. Values override any extensions
	// that would otherwise be produced based on the other fields but are
	// overridden by any extensions specified in Attributes.
	//
	// The ExtraExtensions field is not populated by ParseCertificateRequest,
	// see Extensions instead.
	ExtraExtensions []pkix.Extension

	// Subject Alternate Name values.
	DNSNames       []string
	EmailAddresses []string
	IPAddresses    []net.IP
	URIs           []*url.URL
}

// These structures reflect the ASN.1 structure of X.509 certificate
// signature requests (see RFC 2986):

type tbsCertificateRequest struct {
	Raw           asn1.RawContent
	Version       int
	Subject       asn1.RawValue
	PublicKey     publicKeyInfo
	RawAttributes []asn1.RawValue `asn1:"tag:0"`
}

type certificateRequest struct {
	Raw                asn1.RawContent
	TBSCSR             tbsCertificateRequest
	SignatureAlgorithm struct {
		Raw        asn1.RawContent
		Algorithm  asn1.ObjectIdentifier
		Parameters asn1.RawValue `asn1:"optional"`
	}
	SignatureValue asn1.BitString
}

// oidExtensionRequest is a PKCS #9 OBJECT IDENTIFIER that indicates requested
// extensions in a CSR.
var oidExtensionRequest = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 14}

// newRawAttributes converts AttributeTypeAndValueSETs from a template
// CertificateRequest's Attributes into tbsCertificateRequest RawAttributes.
func newRawAttributes(attributes []pkix.AttributeTypeAndValueSET) ([]asn1.RawValue, error) {
	var rawAttributes []asn1.RawValue
	b, err := asn1.Marshal(attributes)
	if err != nil {
		return nil, err
	}
	rest, err := asn1.Unmarshal(b, &rawAttributes)
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 {
		return nil, errors.New("x509: failed to unmarshal raw CSR Attributes")
	}
	return rawAttributes, nil
}

// parseRawAttributes Unmarshals RawAttributes into AttributeTypeAndValueSETs.
func parseRawAttributes(rawAttributes []asn1.RawValue) []pkix.AttributeTypeAndValueSET {
	var attributes []pkix.AttributeTypeAndValueSET
	for _, rawAttr := range rawAttributes {
		var attr pkix.AttributeTypeAndValueSET
		rest, err := asn1.Unmarshal(rawAttr.FullBytes, &attr)
		// Ignore attributes that don't parse into pkix.AttributeTypeAndValueSET
		// (i.e.: challengePassword or unstructuredName).
		if err == nil && len(rest) == 0 {
			attributes = append(attributes, attr)
		}
	}
	return attributes
}

// parseCSRExtensions parses the attributes from a CSR and extracts any
// requested extensions.
func parseCSRExtensions(rawAttributes []asn1.RawValue) ([]pkix.Extension, error) {
	// pkcs10Attribute reflects the Attribute structure from RFC 2986, Section 4.1.
	type pkcs10Attribute struct {
		Id     asn1.ObjectIdentifier
		Values []asn1.RawValue `asn1:"set"`
	}

	var ret []pkix.Extension
	requestedExts := make(map[string]bool)
	for _, rawAttr := range rawAttributes {
		var attr pkcs10Attribute
		if rest, err := asn1.Unmarshal(rawAttr.FullBytes, &attr); err != nil || len(rest) != 0 || len(attr.Values) == 0 {
			// Ignore attributes that don't parse.
			continue
		}

		if !attr.Id.Equal(oidExtensionRequest) {
			continue
		}

		var extensions []pkix.Extension
		if _, err := asn1.Unmarshal(attr.Values[0].FullBytes, &extensions); err != nil {
			return nil, err
		}
		for _, ext := range extensions {
			oidStr := ext.Id.String()
			if requestedExts[oidStr] {
				return nil, errors.New("x509: certificate request contains duplicate requested extensions")
			}
			requestedExts[oidStr] = true
		}
		ret = append(ret, extensions...)
	}

	return ret, nil
}

// CreateCertificateRequest creates a new certificate request based on a
// template. The following members of template are used:
//
//   - SignatureAlgorithm
//   - Subject
//   - DNSNames
//   - EmailAddresses
//   - IPAddresses
//   - URIs
//   - ExtraExtensions
//   - Attributes (deprecated)
//
// priv is the private key to sign the CSR with, and the corresponding public
// key will be included in the CSR. It must implement crypto.Signer or
// crypto.MessageSigner and its Public() method must return a *rsa.PublicKey or
// a *ecdsa.PublicKey or a ed25519.PublicKey or a *mldsa.PublicKey.
// (A *rsa.PrivateKey, *ecdsa.PrivateKey or ed25519.PrivateKey or
// *mldsa.PrivateKey satisfies this.)
//
// The returned slice is the certificate request in DER encoding.
func CreateCertificateRequest(rand io.Reader, template *CertificateRequest, priv any) (csr []byte, err error) {
	key, ok := priv.(crypto.Signer)
	if !ok {
		return nil, errors.New("x509: certificate private key does not implement crypto.Signer")
	}

	signatureAlgorithm, algorithmIdentifier, err := signingParamsForKey(key, template.SignatureAlgorithm)
	if err != nil {
		return nil, err
	}

	var publicKeyBytes []byte
	var publicKeyAlgorithm pkix.AlgorithmIdentifier
	publicKeyBytes, publicKeyAlgorithm, err = marshalPublicKey(key.Public())
	if err != nil {
		return nil, err
	}

	extensions, err := buildCSRExtensions(template)
	if err != nil {
		return nil, err
	}

	// Make a copy of template.Attributes because we may alter it below.
	attributes := make([]pkix.AttributeTypeAndValueSET, 0, len(template.Attributes))
	for _, attr := range template.Attributes {
		values := make([][]pkix.AttributeTypeAndValue, len(attr.Value))
		copy(values, attr.Value)
		attributes = append(attributes, pkix.AttributeTypeAndValueSET{
			Type:  attr.Type,
			Value: values,
		})
	}

	extensionsAppended := false
	if len(extensions) > 0 {
		// Append the extensions to an existing attribute if possible.
		for _, atvSet := range attributes {
			if !atvSet.Type.Equal(oidExtensionRequest) || len(atvSet.Value) == 0 {
				continue
			}

			// specifiedExtensions contains all the extensions that we
			// found specified via template.Attributes.
			specifiedExtensions := make(map[string]bool)

			for _, atvs := range atvSet.Value {
				for _, atv := range atvs {
					specifiedExtensions[atv.Type.String()] = true
				}
			}

			newValue := make([]pkix.AttributeTypeAndValue, 0, len(atvSet.Value[0])+len(extensions))
			newValue = append(newValue, atvSet.Value[0]...)

			for _, e := range extensions {
				if specifiedExtensions[e.Id.String()] {
					// Attributes already contained a value for
					// this extension and it takes priority.
					continue
				}

				newValue = append(newValue, pkix.AttributeTypeAndValue{
					// There is no place for the critical
					// flag in an AttributeTypeAndValue.
					Type:  e.Id,
					Value: e.Value,
				})
			}

			atvSet.Value[0] = newValue
			extensionsAppended = true
			break
		}
	}

	rawAttributes, err := newRawAttributes(attributes)
	if err != nil {
		return nil, err
	}

	// If not included in attributes, add a new attribute for the
	// extensions.
	if len(extensions) > 0 && !extensionsAppended {
		attr := struct {
			Type  asn1.ObjectIdentifier
			Value [][]pkix.Extension `asn1:"set"`
		}{
			Type:  oidExtensionRequest,
			Value: [][]pkix.Extension{extensions},
		}

		b, err := asn1.Marshal(attr)
		if err != nil {
			return nil, errors.New("x509: failed to serialise extensions attribute: " + err.Error())
		}

		var rawValue asn1.RawValue
		if _, err := asn1.Unmarshal(b, &rawValue); err != nil {
			return nil, err
		}

		rawAttributes = append(rawAttributes, rawValue)
	}

	asn1Subject := template.RawSubject
	if len(asn1Subject) == 0 {
		asn1Subject, err = asn1.Marshal(template.Subject.ToRDNSequence())
		if err != nil {
			return nil, err
		}
	}

	tbsCSR := tbsCertificateRequest{
		Version: 0, // PKCS #10, RFC 2986
		Subject: asn1.RawValue{FullBytes: asn1Subject},
		PublicKey: publicKeyInfo{
			Algorithm: publicKeyAlgorithm,
			PublicKey: asn1.BitString{
				Bytes:     publicKeyBytes,
				BitLength: len(publicKeyBytes) * 8,
			},
		},
		RawAttributes: rawAttributes,
	}

	tbsCSRContents, err := asn1.Marshal(tbsCSR)
	if err != nil {
		return nil, err
	}
	tbsCSR.Raw = tbsCSRContents

	signature, err := signTBS(tbsCSRContents, key, signatureAlgorithm, rand)
	if err != nil {
		return nil, err
	}

	cr := certificateRequest{}
	cr.TBSCSR = tbsCSR
	cr.SignatureAlgorithm.Algorithm = algorithmIdentifier.Algorithm
	cr.SignatureAlgorithm.Parameters = algorithmIdentifier.Parameters
	cr.SignatureValue = asn1.BitString{Bytes: signature, BitLength: len(signature) * 8}
	return asn1.Marshal(cr)
}

// ParseCertificateRequest parses a single certificate request from the
// given ASN.1 DER data.
func ParseCertificateRequest(asn1Data []byte) (*CertificateRequest, error) {
	var csr certificateRequest

	rest, err := asn1.Unmarshal(asn1Data, &csr)
	if err != nil {
		return nil, err
	} else if len(rest) != 0 {
		return nil, asn1.SyntaxError{Msg: "trailing data"}
	}

	return parseCertificateRequest(&csr)
}

func parseCertificateRequest(in *certificateRequest) (*CertificateRequest, error) {
	out := &CertificateRequest{
		Raw:                      in.Raw,
		RawTBSCertificateRequest: in.TBSCSR.Raw,
		RawSubjectPublicKeyInfo:  in.TBSCSR.PublicKey.Raw,
		RawSubject:               in.TBSCSR.Subject.FullBytes,
		RawSignatureAlgorithm:    in.SignatureAlgorithm.Raw,

		Signature: in.SignatureValue.RightAlign(),
		SignatureAlgorithm: getSignatureAlgorithmFromAI(pkix.AlgorithmIdentifier{
			Algorithm:  in.SignatureAlgorithm.Algorithm,
			Parameters: in.SignatureAlgorithm.Parameters,
		}),

		PublicKeyAlgorithm: getPublicKeyAlgorithmFromOID(in.TBSCSR.PublicKey.Algorithm.Algorithm),

		Version:    in.TBSCSR.Version,
		Attributes: parseRawAttributes(in.TBSCSR.RawAttributes),
	}

	var err error
	if out.PublicKeyAlgorithm != UnknownPublicKeyAlgorithm {
		out.PublicKey, err = parsePublicKey(&in.TBSCSR.PublicKey)
		if err != nil {
			return nil, err
		}
	}

	subject, err := parseName(in.TBSCSR.Subject.FullBytes)
	if err != nil {
		return nil, err
	}
	out.Subject.FillFromRDNSequence(subject)

	if out.Extensions, err = parseCSRExtensions(in.TBSCSR.RawAttributes); err != nil {
		return nil, err
	}

	for _, extension := range out.Extensions {
		switch {
		case extension.Id.Equal(oidExtensionSubjectAltName):
			out.DNSNames, out.EmailAddresses, out.IPAddresses, out.URIs, err = parseSANExtension(extension.Value)
			if err != nil {
				return nil, err
			}
		}
	}

	return out, nil
}

// CheckSignature reports whether the signature on c is valid.
func (c *CertificateRequest) CheckSignature() error {
	return checkSignature(c.SignatureAlgorithm, c.RawTBSCertificateRequest, c.Signature, c.PublicKey, true)
}

// RevocationListEntry represents an entry in the revokedCertificates
// sequence of a CRL.
type RevocationListEntry struct {
	// Raw contains the raw bytes of the revokedCertificates entry. It is set when
	// parsing a CRL; it is ignored when generating a CRL.
	Raw []byte

	// SerialNumber represents the serial number of a revoked certificate. It is
	// both used when creating a CRL and populated when parsing a CRL. It must not
	// be nil.
	SerialNumber *big.Int
	// RevocationTime represents the time at which the certificate was revoked. It
	// is both used when creating a CRL and populated when parsing a CRL. It must
	// not be the zero time.
	RevocationTime time.Time
	// ReasonCode represents the reason for revocation, using the integer enum
	// values specified in RFC 5280 Section 5.3.1. When creating a CRL, the zero
	// value will result in the reasonCode extension being omitted. When parsing a
	// CRL, the zero value may represent either the reasonCode extension being
	// absent (which implies the default revocation reason of 0/Unspecified), or
	// it may represent the reasonCode extension being present and explicitly
	// containing a value of 0/Unspecified (which should not happen according to
	// the DER encoding rules, but can and does happen anyway).
	ReasonCode int

	// Extensions contains raw X.509 extensions. When parsing CRL entries,
	// this can be used to extract non-critical extensions that are not
	// parsed by this package. When marshaling CRL entries, the Extensions
	// field is ignored, see ExtraExtensions.
	Extensions []pkix.Extension
	// ExtraExtensions contains extensions to be copied, raw, into any
	// marshaled CRL entries. Values override any extensions that would
	// otherwise be produced based on the other fields. The ExtraExtensions
	// field is not populated when parsing CRL entries, see Extensions.
	ExtraExtensions []pkix.Extension
}

// RevocationList represents a [Certificate] Revocation List (CRL) as specified
// by RFC 5280.
type RevocationList struct {
	// Raw contains the complete ASN.1 DER content of the CRL (tbsCertList,
	// signatureAlgorithm, and signatureValue.)
	Raw []byte
	// RawTBSRevocationList contains just the tbsCertList portion of the ASN.1
	// DER.
	RawTBSRevocationList []byte
	// RawIssuer contains the DER encoded Issuer.
	RawIssuer []byte
	// RawSignatureAlgorithm contains the DER encoded signature algorithm as a
	// PKIX AlgorithmIdentifier.
	RawSignatureAlgorithm []byte

	// Issuer contains the DN of the issuing certificate.
	Issuer pkix.Name
	// AuthorityKeyId is used to identify the public key associated with the
	// issuing certificate. It is populated from the authorityKeyIdentifier
	// extension when parsing a CRL. It is ignored when creating a CRL; the
	// extension is populated from the issuing certificate itself.
	AuthorityKeyId []byte

	Signature []byte
	// SignatureAlgorithm is used to determine the signature algorithm to be
	// used when signing the CRL. If 0 the default algorithm for the signing
	// key will be used.
	SignatureAlgorithm SignatureAlgorithm

	// RevokedCertificateEntries represents the revokedCertificates sequence in
	// the CRL. It is used when creating a CRL and also populated when parsing a
	// CRL. When creating a CRL, it may be empty or nil, in which case the
	// revokedCertificates ASN.1 sequence will be omitted from the CRL entirely.
	RevokedCertificateEntries []RevocationListEntry

	// RevokedCertificates is used to populate the revokedCertificates
	// sequence in the CRL if RevokedCertificateEntries is empty. It may be empty
	// or nil, in which case an empty CRL will be created.
	//
	// Deprecated: Use RevokedCertificateEntries instead.
	RevokedCertificates []pkix.RevokedCertificate

	// Number is used to populate the X.509 v2 cRLNumber extension in the CRL,
	// which should be a monotonically increasing sequence number for a given
	// CRL scope and CRL issuer. It is also populated from the cRLNumber
	// extension when parsing a CRL.
	Number *big.Int

	// ThisUpdate is used to populate the thisUpdate field in the CRL, which
	// indicates the issuance date of the CRL.
	ThisUpdate time.Time
	// NextUpdate is used to populate the nextUpdate field in the CRL, which
	// indicates the date by which the next CRL will be issued. NextUpdate
	// must be greater than ThisUpdate.
	NextUpdate time.Time

	// Extensions contains raw X.509 extensions. When creating a CRL,
	// the Extensions field is ignored, see ExtraExtensions.
	Extensions []pkix.Extension

	// ExtraExtensions contains any additional extensions to add directly to
	// the CRL.
	ExtraExtensions []pkix.Extension
}

// These structures reflect the ASN.1 structure of X.509 CRLs better than
// the existing crypto/x509/pkix variants do. These mirror the existing
// certificate structs in this file.
//
// Notably, we include issuer as an asn1.RawValue, mirroring the behavior of
// tbsCertificate and allowing raw (unparsed) subjects to be passed cleanly.
type certificateList struct {
	TBSCertList        tbsCertificateList
	SignatureAlgorithm pkix.AlgorithmIdentifier
	SignatureValue     asn1.BitString
}

type tbsCertificateList struct {
	Raw                 asn1.RawContent
	Version             int `asn1:"optional,default:0"`
	Signature           pkix.AlgorithmIdentifier
	Issuer              asn1.RawValue
	ThisUpdate          time.Time
	NextUpdate          time.Time                 `asn1:"optional"`
	RevokedCertificates []pkix.RevokedCertificate `asn1:"optional"`
	Extensions          []pkix.Extension          `asn1:"tag:0,optional,explicit"`
}

// CreateRevocationList creates a new X.509 v2 [Certificate] Revocation List,
// according to RFC 5280, based on template.
//
// The CRL is signed by priv which should be a crypto.Signer or
// crypto.MessageSigner associated with the public key in the issuer
// certificate.
//
// The issuer may not be nil, and the crlSign bit must be set in [KeyUsage] in
// order to use it as a CRL issuer.
//
// The issuer distinguished name CRL field and authority key identifier
// extension are populated using the issuer certificate. issuer must have
// SubjectKeyId set.
func CreateRevocationList(rand io.Reader, template *RevocationList, issuer *Certificate, priv crypto.Signer) ([]byte, error) {
	if template == nil {
		return nil, errors.New("x509: template can not be nil")
	}
	if issuer == nil {
		return nil, errors.New("x509: issuer can not be nil")
	}
	if (issuer.KeyUsage & KeyUsageCRLSign) == 0 {
		return nil, errors.New("x509: issuer must have the crlSign key usage bit set")
	}
	if len(issuer.SubjectKeyId) == 0 {
		return nil, errors.New("x509: issuer certificate doesn't contain a subject key identifier")
	}
	if template.NextUpdate.Before(template.ThisUpdate) {
		return nil, errors.New("x509: template.ThisUpdate is after template.NextUpdate")
	}
	if template.Number == nil {
		return nil, errors.New("x509: template contains nil Number field")
	}

	signatureAlgorithm, algorithmIdentifier, err := signingParamsForKey(priv, template.SignatureAlgorithm)
	if err != nil {
		return nil, err
	}

	var revokedCerts []pkix.RevokedCertificate
	// Only process the deprecated RevokedCertificates field if it is populated
	// and the new RevokedCertificateEntries field is not populated.
	if len(template.RevokedCertificates) > 0 && len(template.RevokedCertificateEntries) == 0 {
		// Force revocation times to UTC per RFC 5280.
		revokedCerts = make([]pkix.RevokedCertificate, len(template.RevokedCertificates))
		for i, rc := range template.RevokedCertificates {
			rc.RevocationTime = rc.RevocationTime.UTC()
			revokedCerts[i] = rc
		}
	} else {
		// Convert the ReasonCode field to a proper extension, and force revocation
		// times to UTC per RFC 5280.
		revokedCerts = make([]pkix.RevokedCertificate, len(template.RevokedCertificateEntries))
		for i, rce := range template.RevokedCertificateEntries {
			if rce.SerialNumber == nil {
				return nil, errors.New("x509: template contains entry with nil SerialNumber field")
			}
			if rce.RevocationTime.IsZero() {
				return nil, errors.New("x509: template contains entry with zero RevocationTime field")
			}

			rc := pkix.RevokedCertificate{
				SerialNumber:   rce.SerialNumber,
				RevocationTime: rce.RevocationTime.UTC(),
			}

			// Copy over any extra extensions, except for a Reason Code extension,
			// because we'll synthesize that ourselves to ensure it is correct.
			exts := make([]pkix.Extension, 0, len(rce.ExtraExtensions))
			for _, ext := range rce.ExtraExtensions {
				if ext.Id.Equal(oidExtensionReasonCode) {
					return nil, errors.New("x509: template contains entry with ReasonCode ExtraExtension; use ReasonCode field instead")
				}
				exts = append(exts, ext)
			}

			// Only add a reasonCode extension if the reason is non-zero, as per
			// RFC 5280 Section 5.3.1.
			if rce.ReasonCode != 0 {
				reasonBytes, err := asn1.Marshal(asn1.Enumerated(rce.ReasonCode))
				if err != nil {
					return nil, err
				}

				exts = append(exts, pkix.Extension{
					Id:    oidExtensionReasonCode,
					Value: reasonBytes,
				})
			}

			if len(exts) > 0 {
				rc.Extensions = exts
			}
			revokedCerts[i] = rc
		}
	}

	aki, err := asn1.Marshal(authKeyId{Id: issuer.SubjectKeyId})
	if err != nil {
		return nil, err
	}

	if numBytes := template.Number.Bytes(); len(numBytes) > 20 || (len(numBytes) == 20 && numBytes[0]&0x80 != 0) {
		return nil, errors.New("x509: CRL number exceeds 20 octets")
	}
	crlNum, err := asn1.Marshal(template.Number)
	if err != nil {
		return nil, err
	}

	// Correctly use the issuer's subject sequence if one is specified.
	issuerSubject, err := subjectBytes(issuer)
	if err != nil {
		return nil, err
	}

	tbsCertList := tbsCertificateList{
		Version:    1, // v2
		Signature:  algorithmIdentifier,
		Issuer:     asn1.RawValue{FullBytes: issuerSubject},
		ThisUpdate: template.ThisUpdate.UTC(),
		NextUpdate: template.NextUpdate.UTC(),
		Extensions: []pkix.Extension{
			{
				Id:    oidExtensionAuthorityKeyId,
				Value: aki,
			},
			{
				Id:    oidExtensionCRLNumber,
				Value: crlNum,
			},
		},
	}
	if len(revokedCerts) > 0 {
		tbsCertList.RevokedCertificates = revokedCerts
	}

	if len(template.ExtraExtensions) > 0 {
		tbsCertList.Extensions = append(tbsCertList.Extensions, template.ExtraExtensions...)
	}

	tbsCertListContents, err := asn1.Marshal(tbsCertList)
	if err != nil {
		return nil, err
	}

	// Optimization to only marshal this struct once, when signing and
	// then embedding in certificateList below.
	tbsCertList.Raw = tbsCertListContents

	signature, err := signTBS(tbsCertListContents, priv, signatureAlgorithm, rand)
	if err != nil {
		return nil, err
	}

	return asn1.Marshal(certificateList{
		TBSCertList:        tbsCertList,
		SignatureAlgorithm: algorithmIdentifier,
		SignatureValue:     asn1.BitString{Bytes: signature, BitLength: len(signature) * 8},
	})
}

// CheckSignatureFrom verifies that the signature on rl is a valid signature
// from issuer.
func (rl *RevocationList) CheckSignatureFrom(parent *Certificate) error {
	if parent.Version == 3 && !parent.BasicConstraintsValid ||
		parent.BasicConstraintsValid && !parent.IsCA {
		return ConstraintViolationError{}
	}

	if parent.KeyUsage != 0 && parent.KeyUsage&KeyUsageCRLSign == 0 {
		return ConstraintViolationError{}
	}

	if parent.PublicKeyAlgorithm == UnknownPublicKeyAlgorithm {
		return ErrUnsupportedAlgorithm
	}

	return parent.CheckSignature(rl.SignatureAlgorithm, rl.RawTBSRevocationList, rl.Signature)
}

```

// === FILE: references/go/src/crypto/x509/x509_string.go ===
```go
// Code generated by "stringer -linecomment -type=KeyUsage,ExtKeyUsage -output=x509_string.go"; DO NOT EDIT.

package x509

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[KeyUsageDigitalSignature-1]
	_ = x[KeyUsageContentCommitment-2]
	_ = x[KeyUsageKeyEncipherment-4]
	_ = x[KeyUsageDataEncipherment-8]
	_ = x[KeyUsageKeyAgreement-16]
	_ = x[KeyUsageCertSign-32]
	_ = x[KeyUsageCRLSign-64]
	_ = x[KeyUsageEncipherOnly-128]
	_ = x[KeyUsageDecipherOnly-256]
}

const (
	_KeyUsage_name_0 = "digitalSignaturecontentCommitment"
	_KeyUsage_name_1 = "keyEncipherment"
	_KeyUsage_name_2 = "dataEncipherment"
	_KeyUsage_name_3 = "keyAgreement"
	_KeyUsage_name_4 = "keyCertSign"
	_KeyUsage_name_5 = "cRLSign"
	_KeyUsage_name_6 = "encipherOnly"
	_KeyUsage_name_7 = "decipherOnly"
)

var (
	_KeyUsage_index_0 = [...]uint8{0, 16, 33}
)

func (i KeyUsage) String() string {
	switch {
	case 1 <= i && i <= 2:
		i -= 1
		return _KeyUsage_name_0[_KeyUsage_index_0[i]:_KeyUsage_index_0[i+1]]
	case i == 4:
		return _KeyUsage_name_1
	case i == 8:
		return _KeyUsage_name_2
	case i == 16:
		return _KeyUsage_name_3
	case i == 32:
		return _KeyUsage_name_4
	case i == 64:
		return _KeyUsage_name_5
	case i == 128:
		return _KeyUsage_name_6
	case i == 256:
		return _KeyUsage_name_7
	default:
		return "KeyUsage(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ExtKeyUsageAny-0]
	_ = x[ExtKeyUsageServerAuth-1]
	_ = x[ExtKeyUsageClientAuth-2]
	_ = x[ExtKeyUsageCodeSigning-3]
	_ = x[ExtKeyUsageEmailProtection-4]
	_ = x[ExtKeyUsageIPSECEndSystem-5]
	_ = x[ExtKeyUsageIPSECTunnel-6]
	_ = x[ExtKeyUsageIPSECUser-7]
	_ = x[ExtKeyUsageTimeStamping-8]
	_ = x[ExtKeyUsageOCSPSigning-9]
	_ = x[ExtKeyUsageMicrosoftServerGatedCrypto-10]
	_ = x[ExtKeyUsageNetscapeServerGatedCrypto-11]
	_ = x[ExtKeyUsageMicrosoftCommercialCodeSigning-12]
	_ = x[ExtKeyUsageMicrosoftKernelCodeSigning-13]
}

const _ExtKeyUsage_name = "anyExtendedKeyUsageserverAuthclientAuthcodeSigningemailProtectionipsecEndSystemipsecTunnelipsecUsertimeStampingOCSPSigningmsSGCnsSGCmsCodeCommsKernelCode"

var _ExtKeyUsage_index = [...]uint8{0, 19, 29, 39, 50, 65, 79, 90, 99, 111, 122, 127, 132, 141, 153}

func (i ExtKeyUsage) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_ExtKeyUsage_index)-1 {
		return "ExtKeyUsage(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _ExtKeyUsage_name[_ExtKeyUsage_index[idx]:_ExtKeyUsage_index[idx+1]]
}

```

// === FILE: references/go/src/crypto/x509/x509_test_import.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// This file is run by the x509 tests to ensure that a program with minimal
// imports can sign certificates without errors resulting from missing hash
// functions.
package main

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"time"
)

func main() {
	block, _ := pem.Decode([]byte(pemPrivateKey))
	rsaPriv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		panic("Failed to parse private key: " + err.Error())
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "test",
			Organization: []string{"Σ Acme Co"},
		},
		NotBefore: time.Unix(1000, 0),
		NotAfter:  time.Unix(100000, 0),
		KeyUsage:  x509.KeyUsageCertSign,
	}

	if _, err = x509.CreateCertificate(rand.Reader, &template, &template, &rsaPriv.PublicKey, rsaPriv); err != nil {
		panic("failed to create certificate with basic imports: " + err.Error())
	}
}

var pemPrivateKey = testingKey(`-----BEGIN RSA TESTING KEY-----
MIICXQIBAAKBgQCw0YNSqI9T1VFvRsIOejZ9feiKz1SgGfbe9Xq5tEzt2yJCsbyg
+xtcuCswNhdqY5A1ZN7G60HbL4/Hh/TlLhFJ4zNHVylz9mDDx3yp4IIcK2lb566d
fTD0B5EQ9Iqub4twLUdLKQCBfyhmJJvsEqKxm4J4QWgI+Brh/Pm3d4piPwIDAQAB
AoGASC6fj6TkLfMNdYHLQqG9kOlPfys4fstarpZD7X+fUBJ/H/7y5DzeZLGCYAIU
+QeAHWv6TfZIQjReW7Qy00RFJdgwFlTFRCsKXhG5x+IB+jL0Grr08KbgPPDgy4Jm
xirRHZVtU8lGbkiZX+omDIU28EHLNWL6rFEcTWao/tERspECQQDp2G5Nw0qYWn7H
Wm9Up1zkUTnkUkCzhqtxHbeRvNmHGKE7ryGMJEk2RmgHVstQpsvuFY4lIUSZEjAc
DUFJERhFAkEAwZH6O1ULORp8sHKDdidyleYcZU8L7y9Y3OXJYqELfddfBgFUZeVQ
duRmJj7ryu0g0uurOTE+i8VnMg/ostxiswJBAOc64Dd8uLJWKa6uug+XPr91oi0n
OFtM+xHrNK2jc+WmcSg3UJDnAI3uqMc5B+pERLq0Dc6hStehqHjUko3RnZECQEGZ
eRYWciE+Cre5dzfZkomeXE0xBrhecV0bOq6EKWLSVE+yr6mAl05ThRK9DCfPSOpy
F6rgN3QiyCA9J/1FluUCQQC5nX+PTU1FXx+6Ri2ZCi6EjEKMHr7gHcABhMinZYOt
N59pra9UdVQw9jxCU9G7eMyb0jJkNACAuEwakX3gi27b
-----END RSA TESTING KEY-----
`)

func testingKey(s string) string { return strings.ReplaceAll(s, "TESTING KEY", "PRIVATE KEY") }

```

