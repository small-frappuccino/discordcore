# Domain Architecture: archive

## Layout Topology
```text
archive/
├── tar
│   ├── common.go
│   ├── format.go
│   ├── reader.go
│   ├── stat_actime1.go
│   ├── stat_actime2.go
│   ├── stat_unix.go
│   ├── strconv.go
│   └── writer.go
└── zip
    ├── reader.go
    ├── register.go
    ├── struct.go
    └── writer.go
```

## Source Stream Aggregation

// === FILE: references/go/src/archive/tar/common.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tar implements access to tar archives.
//
// Tape archives (tar) are a file format for storing a sequence of files that
// can be read and written in a streaming manner.
// This package aims to cover most variations of the format,
// including those produced by GNU and BSD tar tools.
package tar

import (
	"errors"
	"fmt"
	"internal/godebug"
	"io/fs"
	"maps"
	"math"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// BUG: Use of the Uid and Gid fields in Header could overflow on 32-bit
// architectures. If a large value is encountered when decoding, the result
// stored in Header will be the truncated version.

var tarinsecurepath = godebug.New("tarinsecurepath")

var (
	ErrHeader          = errors.New("archive/tar: invalid tar header")
	ErrWriteTooLong    = errors.New("archive/tar: write too long")
	ErrFieldTooLong    = errors.New("archive/tar: header field too long")
	ErrWriteAfterClose = errors.New("archive/tar: write after close")
	ErrInsecurePath    = errors.New("archive/tar: insecure file path")
	errMissData        = errors.New("archive/tar: sparse file references non-existent data")
	errUnrefData       = errors.New("archive/tar: sparse file contains unreferenced data")
	errWriteHole       = errors.New("archive/tar: write non-NUL byte in sparse hole")
	errSparseTooLong   = errors.New("archive/tar: sparse map too long")
)

type headerError []string

func (he headerError) Error() string {
	const prefix = "archive/tar: cannot encode header"
	var ss []string
	for _, s := range he {
		if s != "" {
			ss = append(ss, s)
		}
	}
	if len(ss) == 0 {
		return prefix
	}
	return fmt.Sprintf("%s: %v", prefix, strings.Join(ss, "; and "))
}

// Type flags for Header.Typeflag.
const (
	// Type '0' indicates a regular file.
	TypeReg = '0'

	// Deprecated: Use TypeReg instead.
	TypeRegA = '\x00'

	// Type '1' to '6' are header-only flags and may not have a data body.
	TypeLink    = '1' // Hard link
	TypeSymlink = '2' // Symbolic link
	TypeChar    = '3' // Character device node
	TypeBlock   = '4' // Block device node
	TypeDir     = '5' // Directory
	TypeFifo    = '6' // FIFO node

	// Type '7' is reserved.
	TypeCont = '7'

	// Type 'x' is used by the PAX format to store key-value records that
	// are only relevant to the next file.
	// This package transparently handles these types.
	TypeXHeader = 'x'

	// Type 'g' is used by the PAX format to store key-value records that
	// are relevant to all subsequent files.
	// This package only supports parsing and composing such headers,
	// but does not currently support persisting the global state across files.
	TypeXGlobalHeader = 'g'

	// Type 'S' indicates a sparse file in the GNU format.
	TypeGNUSparse = 'S'

	// Types 'L' and 'K' are used by the GNU format for a meta file
	// used to store the path or link name for the next file.
	// This package transparently handles these types.
	TypeGNULongName = 'L'
	TypeGNULongLink = 'K'
)

// Keywords for PAX extended header records.
const (
	paxNone     = "" // Indicates that no PAX key is suitable
	paxPath     = "path"
	paxLinkpath = "linkpath"
	paxSize     = "size"
	paxUid      = "uid"
	paxGid      = "gid"
	paxUname    = "uname"
	paxGname    = "gname"
	paxMtime    = "mtime"
	paxAtime    = "atime"
	paxCtime    = "ctime"   // Removed from later revision of PAX spec, but was valid
	paxCharset  = "charset" // Currently unused
	paxComment  = "comment" // Currently unused

	paxSchilyXattr = "SCHILY.xattr."

	// Keywords for GNU sparse files in a PAX extended header.
	paxGNUSparse          = "GNU.sparse."
	paxGNUSparseNumBlocks = "GNU.sparse.numblocks"
	paxGNUSparseOffset    = "GNU.sparse.offset"
	paxGNUSparseNumBytes  = "GNU.sparse.numbytes"
	paxGNUSparseMap       = "GNU.sparse.map"
	paxGNUSparseName      = "GNU.sparse.name"
	paxGNUSparseMajor     = "GNU.sparse.major"
	paxGNUSparseMinor     = "GNU.sparse.minor"
	paxGNUSparseSize      = "GNU.sparse.size"
	paxGNUSparseRealSize  = "GNU.sparse.realsize"
)

// basicKeys is a set of the PAX keys for which we have built-in support.
// This does not contain "charset" or "comment", which are both PAX-specific,
// so adding them as first-class features of Header is unlikely.
// Users can use the PAXRecords field to set it themselves.
var basicKeys = map[string]bool{
	paxPath: true, paxLinkpath: true, paxSize: true, paxUid: true, paxGid: true,
	paxUname: true, paxGname: true, paxMtime: true, paxAtime: true, paxCtime: true,
}

// A Header represents a single header in a tar archive.
// Some fields may not be populated.
//
// For forward compatibility, users that retrieve a Header from Reader.Next,
// mutate it in some ways, and then pass it back to Writer.WriteHeader
// should do so by creating a new Header and copying the fields
// that they are interested in preserving.
type Header struct {
	// Typeflag is the type of header entry.
	// The zero value is automatically promoted to either TypeReg or TypeDir
	// depending on the presence of a trailing slash in Name.
	Typeflag byte

	Name     string // Name of file entry
	Linkname string // Target name of link (valid for TypeLink or TypeSymlink)

	Size  int64  // Logical file size in bytes
	Mode  int64  // Permission and mode bits
	Uid   int    // User ID of owner
	Gid   int    // Group ID of owner
	Uname string // User name of owner
	Gname string // Group name of owner

	// If the Format is unspecified, then Writer.WriteHeader rounds ModTime
	// to the nearest second and ignores the AccessTime and ChangeTime fields.
	//
	// To use AccessTime or ChangeTime, specify the Format as PAX or GNU.
	// To use sub-second resolution, specify the Format as PAX.
	ModTime    time.Time // Modification time
	AccessTime time.Time // Access time (requires either PAX or GNU support)
	ChangeTime time.Time // Change time (requires either PAX or GNU support)

	Devmajor int64 // Major device number (valid for TypeChar or TypeBlock)
	Devminor int64 // Minor device number (valid for TypeChar or TypeBlock)

	// Xattrs stores extended attributes as PAX records under the
	// "SCHILY.xattr." namespace.
	//
	// The following are semantically equivalent:
	//  h.Xattrs[key] = value
	//  h.PAXRecords["SCHILY.xattr."+key] = value
	//
	// When Writer.WriteHeader is called, the contents of Xattrs will take
	// precedence over those in PAXRecords.
	//
	// Deprecated: Use PAXRecords instead.
	Xattrs map[string]string

	// PAXRecords is a map of PAX extended header records.
	//
	// User-defined records should have keys of the following form:
	//	VENDOR.keyword
	// Where VENDOR is some namespace in all uppercase, and keyword may
	// not contain the '=' character (e.g., "GOLANG.pkg.version").
	// The key and value should be non-empty UTF-8 strings.
	//
	// When Writer.WriteHeader is called, PAX records derived from the
	// other fields in Header take precedence over PAXRecords.
	PAXRecords map[string]string

	// Format specifies the format of the tar header.
	//
	// This is set by Reader.Next as a best-effort guess at the format.
	// Since the Reader liberally reads some non-compliant files,
	// it is possible for this to be FormatUnknown.
	//
	// If the format is unspecified when Writer.WriteHeader is called,
	// then it uses the first format (in the order of USTAR, PAX, GNU)
	// capable of encoding this Header (see Format).
	Format Format
}

// sparseEntry represents a Length-sized fragment at Offset in the file.
type sparseEntry struct{ Offset, Length int64 }

func (s sparseEntry) endOffset() int64 { return s.Offset + s.Length }

// A sparse file can be represented as either a sparseDatas or a sparseHoles.
// As long as the total size is known, they are equivalent and one can be
// converted to the other form and back. The various tar formats with sparse
// file support represent sparse files in the sparseDatas form. That is, they
// specify the fragments in the file that has data, and treat everything else as
// having zero bytes. As such, the encoding and decoding logic in this package
// deals with sparseDatas.
//
// However, the external API uses sparseHoles instead of sparseDatas because the
// zero value of sparseHoles logically represents a normal file (i.e., there are
// no holes in it). On the other hand, the zero value of sparseDatas implies
// that the file has no data in it, which is rather odd.
//
// As an example, if the underlying raw file contains the 10-byte data:
//
//	var compactFile = "abcdefgh"
//
// And the sparse map has the following entries:
//
//	var spd sparseDatas = []sparseEntry{
//		{Offset: 2,  Length: 5},  // Data fragment for 2..6
//		{Offset: 18, Length: 3},  // Data fragment for 18..20
//	}
//	var sph sparseHoles = []sparseEntry{
//		{Offset: 0,  Length: 2},  // Hole fragment for 0..1
//		{Offset: 7,  Length: 11}, // Hole fragment for 7..17
//		{Offset: 21, Length: 4},  // Hole fragment for 21..24
//	}
//
// Then the content of the resulting sparse file with a Header.Size of 25 is:
//
//	var sparseFile = "\x00"*2 + "abcde" + "\x00"*11 + "fgh" + "\x00"*4
type (
	sparseDatas []sparseEntry
	sparseHoles []sparseEntry
)

// validateSparseEntries reports whether sp is a valid sparse map.
// It does not matter whether sp represents data fragments or hole fragments.
func validateSparseEntries(sp []sparseEntry, size int64) bool {
	// Validate all sparse entries. These are the same checks as performed by
	// the BSD tar utility.
	if size < 0 {
		return false
	}
	var pre sparseEntry
	for _, cur := range sp {
		switch {
		case cur.Offset < 0 || cur.Length < 0:
			return false // Negative values are never okay
		case cur.Offset > math.MaxInt64-cur.Length:
			return false // Integer overflow with large length
		case cur.endOffset() > size:
			return false // Region extends beyond the actual size
		case pre.endOffset() > cur.Offset:
			return false // Regions cannot overlap and must be in order
		}
		pre = cur
	}
	return true
}

// alignSparseEntries mutates src and returns dst where each fragment's
// starting offset is aligned up to the nearest block edge, and each
// ending offset is aligned down to the nearest block edge.
//
// Even though the Go tar Reader and the BSD tar utility can handle entries
// with arbitrary offsets and lengths, the GNU tar utility can only handle
// offsets and lengths that are multiples of blockSize.
func alignSparseEntries(src []sparseEntry, size int64) []sparseEntry {
	dst := src[:0]
	for _, s := range src {
		pos, end := s.Offset, s.endOffset()
		pos += blockPadding(+pos) // Round-up to nearest blockSize
		if end != size {
			end -= blockPadding(-end) // Round-down to nearest blockSize
		}
		if pos < end {
			dst = append(dst, sparseEntry{Offset: pos, Length: end - pos})
		}
	}
	return dst
}

// invertSparseEntries converts a sparse map from one form to the other.
// If the input is sparseHoles, then it will output sparseDatas and vice-versa.
// The input must have been already validated.
//
// This function mutates src and returns a normalized map where:
//   - adjacent fragments are coalesced together
//   - only the last fragment may be empty
//   - the endOffset of the last fragment is the total size
func invertSparseEntries(src []sparseEntry, size int64) []sparseEntry {
	dst := src[:0]
	var pre sparseEntry
	for _, cur := range src {
		if cur.Length == 0 {
			continue // Skip empty fragments
		}
		pre.Length = cur.Offset - pre.Offset
		if pre.Length > 0 {
			dst = append(dst, pre) // Only add non-empty fragments
		}
		pre.Offset = cur.endOffset()
	}
	pre.Length = size - pre.Offset // Possibly the only empty fragment
	return append(dst, pre)
}

// fileState tracks the number of logical (includes sparse holes) and physical
// (actual in tar archive) bytes remaining for the current file.
//
// Invariant: logicalRemaining >= physicalRemaining
type fileState interface {
	logicalRemaining() int64
	physicalRemaining() int64
}

// allowedFormats determines which formats can be used.
// The value returned is the logical OR of multiple possible formats.
// If the value is FormatUnknown, then the input Header cannot be encoded
// and an error is returned explaining why.
//
// As a by-product of checking the fields, this function returns paxHdrs, which
// contain all fields that could not be directly encoded.
// A value receiver ensures that this method does not mutate the source Header.
func (h Header) allowedFormats() (format Format, paxHdrs map[string]string, err error) {
	format = FormatUSTAR | FormatPAX | FormatGNU
	paxHdrs = make(map[string]string)

	var whyNoUSTAR, whyNoPAX, whyNoGNU string
	var preferPAX bool // Prefer PAX over USTAR
	verifyString := func(s string, size int, name, paxKey string) {
		// NUL-terminator is optional for path and linkpath.
		// Technically, it is required for uname and gname,
		// but neither GNU nor BSD tar checks for it.
		tooLong := len(s) > size
		allowLongGNU := paxKey == paxPath || paxKey == paxLinkpath
		if hasNUL(s) || (tooLong && !allowLongGNU) {
			whyNoGNU = fmt.Sprintf("GNU cannot encode %s=%q", name, s)
			format.mustNotBe(FormatGNU)
		}
		if !isASCII(s) || tooLong {
			canSplitUSTAR := paxKey == paxPath
			if _, _, ok := splitUSTARPath(s); !canSplitUSTAR || !ok {
				whyNoUSTAR = fmt.Sprintf("USTAR cannot encode %s=%q", name, s)
				format.mustNotBe(FormatUSTAR)
			}
			if paxKey == paxNone {
				whyNoPAX = fmt.Sprintf("PAX cannot encode %s=%q", name, s)
				format.mustNotBe(FormatPAX)
			} else {
				paxHdrs[paxKey] = s
			}
		}
		if v, ok := h.PAXRecords[paxKey]; ok && v == s {
			paxHdrs[paxKey] = v
		}
	}
	verifyNumeric := func(n int64, size int, name, paxKey string) {
		if !fitsInBase256(size, n) {
			whyNoGNU = fmt.Sprintf("GNU cannot encode %s=%d", name, n)
			format.mustNotBe(FormatGNU)
		}
		if !fitsInOctal(size, n) {
			whyNoUSTAR = fmt.Sprintf("USTAR cannot encode %s=%d", name, n)
			format.mustNotBe(FormatUSTAR)
			if paxKey == paxNone {
				whyNoPAX = fmt.Sprintf("PAX cannot encode %s=%d", name, n)
				format.mustNotBe(FormatPAX)
			} else {
				paxHdrs[paxKey] = strconv.FormatInt(n, 10)
			}
		}
		if v, ok := h.PAXRecords[paxKey]; ok && v == strconv.FormatInt(n, 10) {
			paxHdrs[paxKey] = v
		}
	}
	verifyTime := func(ts time.Time, size int, name, paxKey string) {
		if ts.IsZero() {
			return // Always okay
		}
		if !fitsInBase256(size, ts.Unix()) {
			whyNoGNU = fmt.Sprintf("GNU cannot encode %s=%v", name, ts)
			format.mustNotBe(FormatGNU)
		}
		isMtime := paxKey == paxMtime
		fitsOctal := fitsInOctal(size, ts.Unix())
		if (isMtime && !fitsOctal) || !isMtime {
			whyNoUSTAR = fmt.Sprintf("USTAR cannot encode %s=%v", name, ts)
			format.mustNotBe(FormatUSTAR)
		}
		needsNano := ts.Nanosecond() != 0
		if !isMtime || !fitsOctal || needsNano {
			preferPAX = true // USTAR may truncate sub-second measurements
			if paxKey == paxNone {
				whyNoPAX = fmt.Sprintf("PAX cannot encode %s=%v", name, ts)
				format.mustNotBe(FormatPAX)
			} else {
				paxHdrs[paxKey] = formatPAXTime(ts)
			}
		}
		if v, ok := h.PAXRecords[paxKey]; ok && v == formatPAXTime(ts) {
			paxHdrs[paxKey] = v
		}
	}

	// Check basic fields.
	var blk block
	v7 := blk.toV7()
	ustar := blk.toUSTAR()
	gnu := blk.toGNU()
	verifyString(h.Name, len(v7.name()), "Name", paxPath)
	verifyString(h.Linkname, len(v7.linkName()), "Linkname", paxLinkpath)
	verifyString(h.Uname, len(ustar.userName()), "Uname", paxUname)
	verifyString(h.Gname, len(ustar.groupName()), "Gname", paxGname)
	verifyNumeric(h.Mode, len(v7.mode()), "Mode", paxNone)
	verifyNumeric(int64(h.Uid), len(v7.uid()), "Uid", paxUid)
	verifyNumeric(int64(h.Gid), len(v7.gid()), "Gid", paxGid)
	verifyNumeric(h.Size, len(v7.size()), "Size", paxSize)
	verifyNumeric(h.Devmajor, len(ustar.devMajor()), "Devmajor", paxNone)
	verifyNumeric(h.Devminor, len(ustar.devMinor()), "Devminor", paxNone)
	verifyTime(h.ModTime, len(v7.modTime()), "ModTime", paxMtime)
	verifyTime(h.AccessTime, len(gnu.accessTime()), "AccessTime", paxAtime)
	verifyTime(h.ChangeTime, len(gnu.changeTime()), "ChangeTime", paxCtime)

	// Check for header-only types.
	var whyOnlyPAX, whyOnlyGNU string
	switch h.Typeflag {
	case TypeReg, TypeChar, TypeBlock, TypeFifo, TypeGNUSparse:
		// Exclude TypeLink and TypeSymlink, since they may reference directories.
		if strings.HasSuffix(h.Name, "/") {
			return FormatUnknown, nil, headerError{"filename may not have trailing slash"}
		}
	case TypeXHeader, TypeGNULongName, TypeGNULongLink:
		return FormatUnknown, nil, headerError{"cannot manually encode TypeXHeader, TypeGNULongName, or TypeGNULongLink headers"}
	case TypeXGlobalHeader:
		h2 := Header{Name: h.Name, Typeflag: h.Typeflag, Xattrs: h.Xattrs, PAXRecords: h.PAXRecords, Format: h.Format}
		if !reflect.DeepEqual(h, h2) {
			return FormatUnknown, nil, headerError{"only PAXRecords should be set for TypeXGlobalHeader"}
		}
		whyOnlyPAX = "only PAX supports TypeXGlobalHeader"
		format.mayOnlyBe(FormatPAX)
	}
	if !isHeaderOnlyType(h.Typeflag) && h.Size < 0 {
		return FormatUnknown, nil, headerError{"negative size on header-only type"}
	}

	// Check PAX records.
	if len(h.Xattrs) > 0 {
		for k, v := range h.Xattrs {
			paxHdrs[paxSchilyXattr+k] = v
		}
		whyOnlyPAX = "only PAX supports Xattrs"
		format.mayOnlyBe(FormatPAX)
	}
	if len(h.PAXRecords) > 0 {
		for k, v := range h.PAXRecords {
			switch _, exists := paxHdrs[k]; {
			case exists:
				continue // Do not overwrite existing records
			case h.Typeflag == TypeXGlobalHeader:
				paxHdrs[k] = v // Copy all records
			case !basicKeys[k] && !strings.HasPrefix(k, paxGNUSparse):
				paxHdrs[k] = v // Ignore local records that may conflict
			}
		}
		whyOnlyPAX = "only PAX supports PAXRecords"
		format.mayOnlyBe(FormatPAX)
	}
	for k, v := range paxHdrs {
		if !validPAXRecord(k, v) {
			return FormatUnknown, nil, headerError{fmt.Sprintf("invalid PAX record: %q", k+" = "+v)}
		}
	}

	// TODO(dsnet): Re-enable this when adding sparse support.
	// See https://golang.org/issue/22735
	/*
		// Check sparse files.
		if len(h.SparseHoles) > 0 || h.Typeflag == TypeGNUSparse {
			if isHeaderOnlyType(h.Typeflag) {
				return FormatUnknown, nil, headerError{"header-only type cannot be sparse"}
			}
			if !validateSparseEntries(h.SparseHoles, h.Size) {
				return FormatUnknown, nil, headerError{"invalid sparse holes"}
			}
			if h.Typeflag == TypeGNUSparse {
				whyOnlyGNU = "only GNU supports TypeGNUSparse"
				format.mayOnlyBe(FormatGNU)
			} else {
				whyNoGNU = "GNU supports sparse files only with TypeGNUSparse"
				format.mustNotBe(FormatGNU)
			}
			whyNoUSTAR = "USTAR does not support sparse files"
			format.mustNotBe(FormatUSTAR)
		}
	*/

	// Check desired format.
	if wantFormat := h.Format; wantFormat != FormatUnknown {
		if wantFormat.has(FormatPAX) && !preferPAX {
			wantFormat.mayBe(FormatUSTAR) // PAX implies USTAR allowed too
		}
		format.mayOnlyBe(wantFormat) // Set union of formats allowed and format wanted
	}
	if format == FormatUnknown {
		switch h.Format {
		case FormatUSTAR:
			err = headerError{"Format specifies USTAR", whyNoUSTAR, whyOnlyPAX, whyOnlyGNU}
		case FormatPAX:
			err = headerError{"Format specifies PAX", whyNoPAX, whyOnlyGNU}
		case FormatGNU:
			err = headerError{"Format specifies GNU", whyNoGNU, whyOnlyPAX}
		default:
			err = headerError{whyNoUSTAR, whyNoPAX, whyNoGNU, whyOnlyPAX, whyOnlyGNU}
		}
	}
	return format, paxHdrs, err
}

// FileInfo returns an fs.FileInfo for the Header.
func (h *Header) FileInfo() fs.FileInfo {
	return headerFileInfo{h}
}

// headerFileInfo implements fs.FileInfo.
type headerFileInfo struct {
	h *Header
}

func (fi headerFileInfo) Size() int64        { return fi.h.Size }
func (fi headerFileInfo) IsDir() bool        { return fi.Mode().IsDir() }
func (fi headerFileInfo) ModTime() time.Time { return fi.h.ModTime }
func (fi headerFileInfo) Sys() any           { return fi.h }

// Name returns the base name of the file.
func (fi headerFileInfo) Name() string {
	if fi.IsDir() {
		return path.Base(path.Clean(fi.h.Name))
	}
	return path.Base(fi.h.Name)
}

// Mode returns the permission and mode bits for the headerFileInfo.
func (fi headerFileInfo) Mode() (mode fs.FileMode) {
	// Set file permission bits.
	mode = fs.FileMode(fi.h.Mode).Perm()

	// Set setuid, setgid and sticky bits.
	if fi.h.Mode&c_ISUID != 0 {
		mode |= fs.ModeSetuid
	}
	if fi.h.Mode&c_ISGID != 0 {
		mode |= fs.ModeSetgid
	}
	if fi.h.Mode&c_ISVTX != 0 {
		mode |= fs.ModeSticky
	}

	// Set file mode bits; clear perm, setuid, setgid, and sticky bits.
	switch m := fs.FileMode(fi.h.Mode) &^ 07777; m {
	case c_ISDIR:
		mode |= fs.ModeDir
	case c_ISFIFO:
		mode |= fs.ModeNamedPipe
	case c_ISLNK:
		mode |= fs.ModeSymlink
	case c_ISBLK:
		mode |= fs.ModeDevice
	case c_ISCHR:
		mode |= fs.ModeDevice
		mode |= fs.ModeCharDevice
	case c_ISSOCK:
		mode |= fs.ModeSocket
	}

	switch fi.h.Typeflag {
	case TypeSymlink:
		mode |= fs.ModeSymlink
	case TypeChar:
		mode |= fs.ModeDevice
		mode |= fs.ModeCharDevice
	case TypeBlock:
		mode |= fs.ModeDevice
	case TypeDir:
		mode |= fs.ModeDir
	case TypeFifo:
		mode |= fs.ModeNamedPipe
	}

	return mode
}

func (fi headerFileInfo) String() string {
	return fs.FormatFileInfo(fi)
}

// sysStat, if non-nil, populates h from system-dependent fields of fi.
var sysStat func(fi fs.FileInfo, h *Header, doNameLookups bool) error

const (
	// Mode constants from the USTAR spec:
	// See http://pubs.opengroup.org/onlinepubs/9699919799/utilities/pax.html#tag_20_92_13_06
	c_ISUID = 04000 // Set uid
	c_ISGID = 02000 // Set gid
	c_ISVTX = 01000 // Save text (sticky bit)

	// Common Unix mode constants; these are not defined in any common tar standard.
	// Header.FileInfo understands these, but FileInfoHeader will never produce these.
	c_ISDIR  = 040000  // Directory
	c_ISFIFO = 010000  // FIFO
	c_ISREG  = 0100000 // Regular file
	c_ISLNK  = 0120000 // Symbolic link
	c_ISBLK  = 060000  // Block special file
	c_ISCHR  = 020000  // Character special file
	c_ISSOCK = 0140000 // Socket
)

// FileInfoHeader creates a partially-populated [Header] from fi.
// If fi describes a symlink, FileInfoHeader records link as the link target.
// If fi describes a directory, a slash is appended to the name.
//
// Since fs.FileInfo's Name method only returns the base name of
// the file it describes, it may be necessary to modify Header.Name
// to provide the full path name of the file.
//
// If fi implements [FileInfoNames]
// Header.Gname and Header.Uname
// are provided by the methods of the interface.
func FileInfoHeader(fi fs.FileInfo, link string) (*Header, error) {
	if fi == nil {
		return nil, errors.New("archive/tar: FileInfo is nil")
	}
	fm := fi.Mode()
	h := &Header{
		Name:    fi.Name(),
		ModTime: fi.ModTime(),
		Mode:    int64(fm.Perm()), // or'd with c_IS* constants later
	}
	switch {
	case fm.IsRegular():
		h.Typeflag = TypeReg
		h.Size = fi.Size()
	case fi.IsDir():
		h.Typeflag = TypeDir
		h.Name += "/"
	case fm&fs.ModeSymlink != 0:
		h.Typeflag = TypeSymlink
		h.Linkname = link
	case fm&fs.ModeDevice != 0:
		if fm&fs.ModeCharDevice != 0 {
			h.Typeflag = TypeChar
		} else {
			h.Typeflag = TypeBlock
		}
	case fm&fs.ModeNamedPipe != 0:
		h.Typeflag = TypeFifo
	case fm&fs.ModeSocket != 0:
		return nil, fmt.Errorf("archive/tar: sockets not supported")
	default:
		return nil, fmt.Errorf("archive/tar: unknown file mode %v", fm)
	}
	if fm&fs.ModeSetuid != 0 {
		h.Mode |= c_ISUID
	}
	if fm&fs.ModeSetgid != 0 {
		h.Mode |= c_ISGID
	}
	if fm&fs.ModeSticky != 0 {
		h.Mode |= c_ISVTX
	}
	// If possible, populate additional fields from OS-specific
	// FileInfo fields.
	if sys, ok := fi.Sys().(*Header); ok {
		// This FileInfo came from a Header (not the OS). Use the
		// original Header to populate all remaining fields.
		h.Uid = sys.Uid
		h.Gid = sys.Gid
		h.Uname = sys.Uname
		h.Gname = sys.Gname
		h.AccessTime = sys.AccessTime
		h.ChangeTime = sys.ChangeTime
		h.Xattrs = maps.Clone(sys.Xattrs)
		if sys.Typeflag == TypeLink {
			// hard link
			h.Typeflag = TypeLink
			h.Size = 0
			h.Linkname = sys.Linkname
		}
		h.PAXRecords = maps.Clone(sys.PAXRecords)
	}
	var doNameLookups = true
	if iface, ok := fi.(FileInfoNames); ok {
		doNameLookups = false
		var err error
		h.Gname, err = iface.Gname()
		if err != nil {
			return nil, err
		}
		h.Uname, err = iface.Uname()
		if err != nil {
			return nil, err
		}
	}
	if sysStat != nil {
		return h, sysStat(fi, h, doNameLookups)
	}
	return h, nil
}

// FileInfoNames extends [fs.FileInfo].
// Passing an instance of this to [FileInfoHeader] permits the caller
// to avoid a system-dependent name lookup by specifying the Uname and Gname directly.
type FileInfoNames interface {
	fs.FileInfo
	// Uname should give a user name.
	Uname() (string, error)
	// Gname should give a group name.
	Gname() (string, error)
}

// isHeaderOnlyType checks if the given type flag is of the type that has no
// data section even if a size is specified.
func isHeaderOnlyType(flag byte) bool {
	switch flag {
	case TypeLink, TypeSymlink, TypeChar, TypeBlock, TypeDir, TypeFifo:
		return true
	default:
		return false
	}
}

```

// === FILE: references/go/src/archive/tar/format.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tar

import "strings"

// Format represents the tar archive format.
//
// The original tar format was introduced in Unix V7.
// Since then, there have been multiple competing formats attempting to
// standardize or extend the V7 format to overcome its limitations.
// The most common formats are the USTAR, PAX, and GNU formats,
// each with their own advantages and limitations.
//
// The following table captures the capabilities of each format:
//
//	                  |  USTAR |       PAX |       GNU
//	------------------+--------+-----------+----------
//	Name              |   256B | unlimited | unlimited
//	Linkname          |   100B | unlimited | unlimited
//	Size              | uint33 | unlimited |    uint89
//	Mode              | uint21 |    uint21 |    uint57
//	Uid/Gid           | uint21 | unlimited |    uint57
//	Uname/Gname       |    32B | unlimited |       32B
//	ModTime           | uint33 | unlimited |     int89
//	AccessTime        |    n/a | unlimited |     int89
//	ChangeTime        |    n/a | unlimited |     int89
//	Devmajor/Devminor | uint21 |    uint21 |    uint57
//	------------------+--------+-----------+----------
//	string encoding   |  ASCII |     UTF-8 |    binary
//	sub-second times  |     no |       yes |        no
//	sparse files      |     no |       yes |       yes
//
// The table's upper portion shows the [Header] fields, where each format reports
// the maximum number of bytes allowed for each string field and
// the integer type used to store each numeric field
// (where timestamps are stored as the number of seconds since the Unix epoch).
//
// The table's lower portion shows specialized features of each format,
// such as supported string encodings, support for sub-second timestamps,
// or support for sparse files.
//
// The Writer currently provides no support for sparse files.
type Format int

// Constants to identify various tar formats.
const (
	// Deliberately hide the meaning of constants from public API.
	_ Format = (1 << iota) / 4 // Sequence of 0, 0, 1, 2, 4, 8, etc...

	// FormatUnknown indicates that the format is unknown.
	FormatUnknown

	// The format of the original Unix V7 tar tool prior to standardization.
	formatV7

	// FormatUSTAR represents the USTAR header format defined in POSIX.1-1988.
	//
	// While this format is compatible with most tar readers,
	// the format has several limitations making it unsuitable for some usages.
	// Most notably, it cannot support sparse files, files larger than 8GiB,
	// filenames larger than 256 characters, and non-ASCII filenames.
	//
	// Reference:
	//	http://pubs.opengroup.org/onlinepubs/9699919799/utilities/pax.html#tag_20_92_13_06
	FormatUSTAR

	// FormatPAX represents the PAX header format defined in POSIX.1-2001.
	//
	// PAX extends USTAR by writing a special file with Typeflag TypeXHeader
	// preceding the original header. This file contains a set of key-value
	// records, which are used to overcome USTAR's shortcomings, in addition to
	// providing the ability to have sub-second resolution for timestamps.
	//
	// Some newer formats add their own extensions to PAX by defining their
	// own keys and assigning certain semantic meaning to the associated values.
	// For example, sparse file support in PAX is implemented using keys
	// defined by the GNU manual (e.g., "GNU.sparse.map").
	//
	// Reference:
	//	http://pubs.opengroup.org/onlinepubs/009695399/utilities/pax.html
	FormatPAX

	// FormatGNU represents the GNU header format.
	//
	// The GNU header format is older than the USTAR and PAX standards and
	// is not compatible with them. The GNU format supports
	// arbitrary file sizes, filenames of arbitrary encoding and length,
	// sparse files, and other features.
	//
	// It is recommended that PAX be chosen over GNU unless the target
	// application can only parse GNU formatted archives.
	//
	// Reference:
	//	https://www.gnu.org/software/tar/manual/html_node/Standard.html
	FormatGNU

	// Schily's tar format, which is incompatible with USTAR.
	// This does not cover STAR extensions to the PAX format; these fall under
	// the PAX format.
	formatSTAR

	formatMax
)

func (f Format) has(f2 Format) bool   { return f&f2 != 0 }
func (f *Format) mayBe(f2 Format)     { *f |= f2 }
func (f *Format) mayOnlyBe(f2 Format) { *f &= f2 }
func (f *Format) mustNotBe(f2 Format) { *f &^= f2 }

var formatNames = map[Format]string{
	formatV7: "V7", FormatUSTAR: "USTAR", FormatPAX: "PAX", FormatGNU: "GNU", formatSTAR: "STAR",
}

func (f Format) String() string {
	var ss []string
	for f2 := Format(1); f2 < formatMax; f2 <<= 1 {
		if f.has(f2) {
			ss = append(ss, formatNames[f2])
		}
	}
	switch len(ss) {
	case 0:
		return "<unknown>"
	case 1:
		return ss[0]
	default:
		return "(" + strings.Join(ss, " | ") + ")"
	}
}

// Magics used to identify various formats.
const (
	magicGNU, versionGNU     = "ustar ", " \x00"
	magicUSTAR, versionUSTAR = "ustar\x00", "00"
	trailerSTAR              = "tar\x00"
)

// Size constants from various tar specifications.
const (
	blockSize  = 512 // Size of each block in a tar stream
	nameSize   = 100 // Max length of the name field in USTAR format
	prefixSize = 155 // Max length of the prefix field in USTAR format

	// Max length of a special file (PAX header, GNU long name or link).
	// This matches the limit used by libarchive.
	maxSpecialFileSize = 1 << 20

	// Maximum number of sparse file entries.
	// We should never actually hit this limit
	// (every sparse encoding will first be limited by maxSpecialFileSize),
	// but this adds an additional layer of defense.
	maxSparseFileEntries = 1 << 20
)

// blockPadding computes the number of bytes needed to pad offset up to the
// nearest block edge where 0 <= n < blockSize.
func blockPadding(offset int64) (n int64) {
	return -offset & (blockSize - 1)
}

var zeroBlock block

type block [blockSize]byte

// Convert block to any number of formats.
func (b *block) toV7() *headerV7       { return (*headerV7)(b) }
func (b *block) toGNU() *headerGNU     { return (*headerGNU)(b) }
func (b *block) toSTAR() *headerSTAR   { return (*headerSTAR)(b) }
func (b *block) toUSTAR() *headerUSTAR { return (*headerUSTAR)(b) }
func (b *block) toSparse() sparseArray { return sparseArray(b[:]) }

// getFormat checks that the block is a valid tar header based on the checksum.
// It then attempts to guess the specific format based on magic values.
// If the checksum fails, then FormatUnknown is returned.
func (b *block) getFormat() Format {
	// Verify checksum.
	var p parser
	value := p.parseOctal(b.toV7().chksum())
	chksum1, chksum2 := b.computeChecksum()
	if p.err != nil || (value != chksum1 && value != chksum2) {
		return FormatUnknown
	}

	// Guess the magic values.
	magic := string(b.toUSTAR().magic())
	version := string(b.toUSTAR().version())
	trailer := string(b.toSTAR().trailer())
	switch {
	case magic == magicUSTAR && trailer == trailerSTAR:
		return formatSTAR
	case magic == magicUSTAR:
		return FormatUSTAR | FormatPAX
	case magic == magicGNU && version == versionGNU:
		return FormatGNU
	default:
		return formatV7
	}
}

// setFormat writes the magic values necessary for specified format
// and then updates the checksum accordingly.
func (b *block) setFormat(format Format) {
	// Set the magic values.
	switch {
	case format.has(formatV7):
		// Do nothing.
	case format.has(FormatGNU):
		copy(b.toGNU().magic(), magicGNU)
		copy(b.toGNU().version(), versionGNU)
	case format.has(formatSTAR):
		copy(b.toSTAR().magic(), magicUSTAR)
		copy(b.toSTAR().version(), versionUSTAR)
		copy(b.toSTAR().trailer(), trailerSTAR)
	case format.has(FormatUSTAR | FormatPAX):
		copy(b.toUSTAR().magic(), magicUSTAR)
		copy(b.toUSTAR().version(), versionUSTAR)
	default:
		panic("invalid format")
	}

	// Update checksum.
	// This field is special in that it is terminated by a NULL then space.
	var f formatter
	field := b.toV7().chksum()
	chksum, _ := b.computeChecksum() // Possible values are 256..128776
	f.formatOctal(field[:7], chksum) // Never fails since 128776 < 262143
	field[7] = ' '
}

// computeChecksum computes the checksum for the header block.
// POSIX specifies a sum of the unsigned byte values, but the Sun tar used
// signed byte values.
// We compute and return both.
func (b *block) computeChecksum() (unsigned, signed int64) {
	for i, c := range b {
		if 148 <= i && i < 156 {
			c = ' ' // Treat the checksum field itself as all spaces.
		}
		unsigned += int64(c)
		signed += int64(int8(c))
	}
	return unsigned, signed
}

// reset clears the block with all zeros.
func (b *block) reset() {
	*b = block{}
}

type headerV7 [blockSize]byte

func (h *headerV7) name() []byte     { return h[000:][:100] }
func (h *headerV7) mode() []byte     { return h[100:][:8] }
func (h *headerV7) uid() []byte      { return h[108:][:8] }
func (h *headerV7) gid() []byte      { return h[116:][:8] }
func (h *headerV7) size() []byte     { return h[124:][:12] }
func (h *headerV7) modTime() []byte  { return h[136:][:12] }
func (h *headerV7) chksum() []byte   { return h[148:][:8] }
func (h *headerV7) typeFlag() []byte { return h[156:][:1] }
func (h *headerV7) linkName() []byte { return h[157:][:100] }

type headerGNU [blockSize]byte

func (h *headerGNU) v7() *headerV7       { return (*headerV7)(h) }
func (h *headerGNU) magic() []byte       { return h[257:][:6] }
func (h *headerGNU) version() []byte     { return h[263:][:2] }
func (h *headerGNU) userName() []byte    { return h[265:][:32] }
func (h *headerGNU) groupName() []byte   { return h[297:][:32] }
func (h *headerGNU) devMajor() []byte    { return h[329:][:8] }
func (h *headerGNU) devMinor() []byte    { return h[337:][:8] }
func (h *headerGNU) accessTime() []byte  { return h[345:][:12] }
func (h *headerGNU) changeTime() []byte  { return h[357:][:12] }
func (h *headerGNU) sparse() sparseArray { return sparseArray(h[386:][:24*4+1]) }
func (h *headerGNU) realSize() []byte    { return h[483:][:12] }

type headerSTAR [blockSize]byte

func (h *headerSTAR) v7() *headerV7      { return (*headerV7)(h) }
func (h *headerSTAR) magic() []byte      { return h[257:][:6] }
func (h *headerSTAR) version() []byte    { return h[263:][:2] }
func (h *headerSTAR) userName() []byte   { return h[265:][:32] }
func (h *headerSTAR) groupName() []byte  { return h[297:][:32] }
func (h *headerSTAR) devMajor() []byte   { return h[329:][:8] }
func (h *headerSTAR) devMinor() []byte   { return h[337:][:8] }
func (h *headerSTAR) prefix() []byte     { return h[345:][:131] }
func (h *headerSTAR) accessTime() []byte { return h[476:][:12] }
func (h *headerSTAR) changeTime() []byte { return h[488:][:12] }
func (h *headerSTAR) trailer() []byte    { return h[508:][:4] }

type headerUSTAR [blockSize]byte

func (h *headerUSTAR) v7() *headerV7     { return (*headerV7)(h) }
func (h *headerUSTAR) magic() []byte     { return h[257:][:6] }
func (h *headerUSTAR) version() []byte   { return h[263:][:2] }
func (h *headerUSTAR) userName() []byte  { return h[265:][:32] }
func (h *headerUSTAR) groupName() []byte { return h[297:][:32] }
func (h *headerUSTAR) devMajor() []byte  { return h[329:][:8] }
func (h *headerUSTAR) devMinor() []byte  { return h[337:][:8] }
func (h *headerUSTAR) prefix() []byte    { return h[345:][:155] }

type sparseArray []byte

func (s sparseArray) entry(i int) sparseElem { return sparseElem(s[i*24:]) }
func (s sparseArray) isExtended() []byte     { return s[24*s.maxEntries():][:1] }
func (s sparseArray) maxEntries() int        { return len(s) / 24 }

type sparseElem []byte

func (s sparseElem) offset() []byte { return s[00:][:12] }
func (s sparseElem) length() []byte { return s[12:][:12] }

```

// === FILE: references/go/src/archive/tar/reader.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tar

import (
	"bytes"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Reader provides sequential access to the contents of a tar archive.
// Reader.Next advances to the next file in the archive (including the first),
// and then Reader can be treated as an io.Reader to access the file's data.
type Reader struct {
	r    io.Reader
	pad  int64      // Amount of padding (ignored) after current file entry
	curr fileReader // Reader for current file entry
	blk  block      // Buffer to use as temporary local storage

	// err is a persistent error.
	// It is only the responsibility of every exported method of Reader to
	// ensure that this error is sticky.
	err error
}

type fileReader interface {
	io.Reader
	fileState

	WriteTo(io.Writer) (int64, error)
}

// NewReader creates a new [Reader] reading from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: r, curr: &regFileReader{r, 0}}
}

// Next advances to the next entry in the tar archive.
// The Header.Size determines how many bytes can be read for the next file.
// Any remaining data in the current file is automatically discarded.
// At the end of the archive, Next returns the error io.EOF.
//
// If Next encounters a non-local file name (as defined by [filepath.IsLocal])
// and the GODEBUG environment variable contains `tarinsecurepath=0`,
// Only file names are validated, not link targets.
// Next returns the header with an [ErrInsecurePath] error.
// A future version of Go may introduce this behavior by default.
// Programs that want to accept non-local names can ignore
// the [ErrInsecurePath] error and use the returned header.
func (tr *Reader) Next() (*Header, error) {
	if tr.err != nil {
		return nil, tr.err
	}
	hdr, err := tr.next()
	tr.err = err
	if err == nil && !filepath.IsLocal(hdr.Name) {
		if tarinsecurepath.Value() == "0" {
			tarinsecurepath.IncNonDefault()
			err = ErrInsecurePath
		}
	}
	return hdr, err
}

func (tr *Reader) next() (*Header, error) {
	var paxHdrs map[string]string
	var gnuLongName, gnuLongLink string

	// Externally, Next iterates through the tar archive as if it is a series of
	// files. Internally, the tar format often uses fake "files" to add meta
	// data that describes the next file. These meta data "files" should not
	// normally be visible to the outside. As such, this loop iterates through
	// one or more "header files" until it finds a "normal file".
	format := FormatUSTAR | FormatPAX | FormatGNU
	for {
		// Discard the remainder of the file and any padding.
		if err := discard(tr.r, tr.curr.physicalRemaining()); err != nil {
			return nil, err
		}
		if _, err := tryReadFull(tr.r, tr.blk[:tr.pad]); err != nil {
			return nil, err
		}
		tr.pad = 0

		hdr, rawHdr, err := tr.readHeader()
		if err != nil {
			return nil, err
		}
		if err := tr.handleRegularFile(hdr); err != nil {
			return nil, err
		}
		format.mayOnlyBe(hdr.Format)

		// Check for PAX/GNU special headers and files.
		switch hdr.Typeflag {
		case TypeXHeader, TypeXGlobalHeader:
			format.mayOnlyBe(FormatPAX)
			paxHdrs, err = parsePAX(tr)
			if err != nil {
				return nil, err
			}
			if hdr.Typeflag == TypeXGlobalHeader {
				mergePAX(hdr, paxHdrs)
				return &Header{
					Name:       hdr.Name,
					Typeflag:   hdr.Typeflag,
					Xattrs:     hdr.Xattrs,
					PAXRecords: hdr.PAXRecords,
					Format:     format,
				}, nil
			}
			continue // This is a meta header affecting the next header
		case TypeGNULongName, TypeGNULongLink:
			format.mayOnlyBe(FormatGNU)
			realname, err := readSpecialFile(tr)
			if err != nil {
				return nil, err
			}

			var p parser
			switch hdr.Typeflag {
			case TypeGNULongName:
				gnuLongName = p.parseString(realname)
			case TypeGNULongLink:
				gnuLongLink = p.parseString(realname)
			}
			continue // This is a meta header affecting the next header
		default:
			// The old GNU sparse format is handled here since it is technically
			// just a regular file with additional attributes.

			if err := mergePAX(hdr, paxHdrs); err != nil {
				return nil, err
			}
			if gnuLongName != "" {
				hdr.Name = gnuLongName
			}
			if gnuLongLink != "" {
				hdr.Linkname = gnuLongLink
			}
			if hdr.Typeflag == TypeRegA {
				if strings.HasSuffix(hdr.Name, "/") {
					hdr.Typeflag = TypeDir // Legacy archives use trailing slash for directories
				} else {
					hdr.Typeflag = TypeReg
				}
			}

			// The extended headers may have updated the size.
			// Thus, setup the regFileReader again after merging PAX headers.
			if err := tr.handleRegularFile(hdr); err != nil {
				return nil, err
			}

			// Sparse formats rely on being able to read from the logical data
			// section; there must be a preceding call to handleRegularFile.
			if err := tr.handleSparseFile(hdr, rawHdr); err != nil {
				return nil, err
			}

			// Set the final guess at the format.
			if format.has(FormatUSTAR) && format.has(FormatPAX) {
				format.mayOnlyBe(FormatUSTAR)
			}
			hdr.Format = format
			return hdr, nil // This is a file, so stop
		}
	}
}

// handleRegularFile sets up the current file reader and padding such that it
// can only read the following logical data section. It will properly handle
// special headers that contain no data section.
func (tr *Reader) handleRegularFile(hdr *Header) error {
	nb := hdr.Size
	if isHeaderOnlyType(hdr.Typeflag) {
		nb = 0
	}
	if nb < 0 {
		return ErrHeader
	}

	tr.pad = blockPadding(nb)
	tr.curr = &regFileReader{r: tr.r, nb: nb}
	return nil
}

// handleSparseFile checks if the current file is a sparse format of any type
// and sets the curr reader appropriately.
func (tr *Reader) handleSparseFile(hdr *Header, rawHdr *block) error {
	var spd sparseDatas
	var err error
	if hdr.Typeflag == TypeGNUSparse {
		spd, err = tr.readOldGNUSparseMap(hdr, rawHdr)
	} else {
		spd, err = tr.readGNUSparsePAXHeaders(hdr)
	}

	// If sp is non-nil, then this is a sparse file.
	// Note that it is possible for len(sp) == 0.
	if err == nil && spd != nil {
		if isHeaderOnlyType(hdr.Typeflag) || !validateSparseEntries(spd, hdr.Size) {
			return ErrHeader
		}
		sph := invertSparseEntries(spd, hdr.Size)
		tr.curr = &sparseFileReader{tr.curr, sph, 0}
	}
	return err
}

// readGNUSparsePAXHeaders checks the PAX headers for GNU sparse headers.
// If they are found, then this function reads the sparse map and returns it.
// This assumes that 0.0 headers have already been converted to 0.1 headers
// by the PAX header parsing logic.
func (tr *Reader) readGNUSparsePAXHeaders(hdr *Header) (sparseDatas, error) {
	// Identify the version of GNU headers.
	var is1x0 bool
	major, minor := hdr.PAXRecords[paxGNUSparseMajor], hdr.PAXRecords[paxGNUSparseMinor]
	switch {
	case major == "0" && (minor == "0" || minor == "1"):
		is1x0 = false
	case major == "1" && minor == "0":
		is1x0 = true
	case major != "" || minor != "":
		return nil, nil // Unknown GNU sparse PAX version
	case hdr.PAXRecords[paxGNUSparseMap] != "":
		is1x0 = false // 0.0 and 0.1 did not have explicit version records, so guess
	default:
		return nil, nil // Not a PAX format GNU sparse file.
	}
	hdr.Format.mayOnlyBe(FormatPAX)

	// Update hdr from GNU sparse PAX headers.
	if name := hdr.PAXRecords[paxGNUSparseName]; name != "" {
		hdr.Name = name
	}
	size := hdr.PAXRecords[paxGNUSparseSize]
	if size == "" {
		size = hdr.PAXRecords[paxGNUSparseRealSize]
	}
	if size != "" {
		n, err := strconv.ParseInt(size, 10, 64)
		if err != nil {
			return nil, ErrHeader
		}
		hdr.Size = n
	}

	// Read the sparse map according to the appropriate format.
	if is1x0 {
		return readGNUSparseMap1x0(tr.curr)
	}
	return readGNUSparseMap0x1(hdr.PAXRecords)
}

// mergePAX merges paxHdrs into hdr for all relevant fields of Header.
func mergePAX(hdr *Header, paxHdrs map[string]string) (err error) {
	for k, v := range paxHdrs {
		if v == "" {
			continue // Keep the original USTAR value
		}
		var id64 int64
		switch k {
		case paxPath:
			hdr.Name = v
		case paxLinkpath:
			hdr.Linkname = v
		case paxUname:
			hdr.Uname = v
		case paxGname:
			hdr.Gname = v
		case paxUid:
			id64, err = strconv.ParseInt(v, 10, 64)
			hdr.Uid = int(id64) // Integer overflow possible
		case paxGid:
			id64, err = strconv.ParseInt(v, 10, 64)
			hdr.Gid = int(id64) // Integer overflow possible
		case paxAtime:
			hdr.AccessTime, err = parsePAXTime(v)
		case paxMtime:
			hdr.ModTime, err = parsePAXTime(v)
		case paxCtime:
			hdr.ChangeTime, err = parsePAXTime(v)
		case paxSize:
			hdr.Size, err = strconv.ParseInt(v, 10, 64)
		default:
			if strings.HasPrefix(k, paxSchilyXattr) {
				if hdr.Xattrs == nil {
					hdr.Xattrs = make(map[string]string)
				}
				hdr.Xattrs[k[len(paxSchilyXattr):]] = v
			}
		}
		if err != nil {
			return ErrHeader
		}
	}
	hdr.PAXRecords = paxHdrs
	return nil
}

// parsePAX parses PAX headers.
// If an extended header (type 'x') is invalid, ErrHeader is returned.
func parsePAX(r io.Reader) (map[string]string, error) {
	buf, err := readSpecialFile(r)
	if err != nil {
		return nil, err
	}
	sbuf := string(buf)

	// For GNU PAX sparse format 0.0 support.
	// This function transforms the sparse format 0.0 headers into format 0.1
	// headers since 0.0 headers were not PAX compliant.
	var sparseMap []string

	paxHdrs := make(map[string]string)
	for len(sbuf) > 0 {
		key, value, residual, err := parsePAXRecord(sbuf)
		if err != nil {
			return nil, ErrHeader
		}
		sbuf = residual

		switch key {
		case paxGNUSparseOffset, paxGNUSparseNumBytes:
			// Validate sparse header order and value.
			if (len(sparseMap)%2 == 0 && key != paxGNUSparseOffset) ||
				(len(sparseMap)%2 == 1 && key != paxGNUSparseNumBytes) ||
				strings.Contains(value, ",") {
				return nil, ErrHeader
			}
			sparseMap = append(sparseMap, value)
		default:
			paxHdrs[key] = value
		}
	}
	if len(sparseMap) > 0 {
		paxHdrs[paxGNUSparseMap] = strings.Join(sparseMap, ",")
	}
	return paxHdrs, nil
}

// readHeader reads the next block header and assumes that the underlying reader
// is already aligned to a block boundary. It returns the raw block of the
// header in case further processing is required.
//
// The err will be set to io.EOF only when one of the following occurs:
//   - Exactly 0 bytes are read and EOF is hit.
//   - Exactly 1 block of zeros is read and EOF is hit.
//   - At least 2 blocks of zeros are read.
func (tr *Reader) readHeader() (*Header, *block, error) {
	// Two blocks of zero bytes marks the end of the archive.
	if _, err := io.ReadFull(tr.r, tr.blk[:]); err != nil {
		return nil, nil, err // EOF is okay here; exactly 0 bytes read
	}
	if bytes.Equal(tr.blk[:], zeroBlock[:]) {
		if _, err := io.ReadFull(tr.r, tr.blk[:]); err != nil {
			return nil, nil, err // EOF is okay here; exactly 1 block of zeros read
		}
		if bytes.Equal(tr.blk[:], zeroBlock[:]) {
			return nil, nil, io.EOF // normal EOF; exactly 2 block of zeros read
		}
		return nil, nil, ErrHeader // Zero block and then non-zero block
	}

	// Verify the header matches a known format.
	format := tr.blk.getFormat()
	if format == FormatUnknown {
		return nil, nil, ErrHeader
	}

	var p parser
	hdr := new(Header)

	// Unpack the V7 header.
	v7 := tr.blk.toV7()
	hdr.Typeflag = v7.typeFlag()[0]
	hdr.Name = p.parseString(v7.name())
	hdr.Linkname = p.parseString(v7.linkName())
	hdr.Size = p.parseNumeric(v7.size())
	hdr.Mode = p.parseNumeric(v7.mode())
	hdr.Uid = int(p.parseNumeric(v7.uid()))
	hdr.Gid = int(p.parseNumeric(v7.gid()))
	hdr.ModTime = time.Unix(p.parseNumeric(v7.modTime()), 0)

	// Unpack format specific fields.
	if format > formatV7 {
		ustar := tr.blk.toUSTAR()
		hdr.Uname = p.parseString(ustar.userName())
		hdr.Gname = p.parseString(ustar.groupName())
		hdr.Devmajor = p.parseNumeric(ustar.devMajor())
		hdr.Devminor = p.parseNumeric(ustar.devMinor())

		var prefix string
		switch {
		case format.has(FormatUSTAR | FormatPAX):
			hdr.Format = format
			ustar := tr.blk.toUSTAR()
			prefix = p.parseString(ustar.prefix())

			// For Format detection, check if block is properly formatted since
			// the parser is more liberal than what USTAR actually permits.
			notASCII := func(r rune) bool { return r >= 0x80 }
			if bytes.IndexFunc(tr.blk[:], notASCII) >= 0 {
				hdr.Format = FormatUnknown // Non-ASCII characters in block.
			}
			nul := func(b []byte) bool { return int(b[len(b)-1]) == 0 }
			if !(nul(v7.size()) && nul(v7.mode()) && nul(v7.uid()) && nul(v7.gid()) &&
				nul(v7.modTime()) && nul(ustar.devMajor()) && nul(ustar.devMinor())) {
				hdr.Format = FormatUnknown // Numeric fields must end in NUL
			}
		case format.has(formatSTAR):
			star := tr.blk.toSTAR()
			prefix = p.parseString(star.prefix())
			hdr.AccessTime = time.Unix(p.parseNumeric(star.accessTime()), 0)
			hdr.ChangeTime = time.Unix(p.parseNumeric(star.changeTime()), 0)
		case format.has(FormatGNU):
			hdr.Format = format
			var p2 parser
			gnu := tr.blk.toGNU()
			if b := gnu.accessTime(); b[0] != 0 {
				hdr.AccessTime = time.Unix(p2.parseNumeric(b), 0)
			}
			if b := gnu.changeTime(); b[0] != 0 {
				hdr.ChangeTime = time.Unix(p2.parseNumeric(b), 0)
			}

			// Prior to Go1.8, the Writer had a bug where it would output
			// an invalid tar file in certain rare situations because the logic
			// incorrectly believed that the old GNU format had a prefix field.
			// This is wrong and leads to an output file that mangles the
			// atime and ctime fields, which are often left unused.
			//
			// In order to continue reading tar files created by former, buggy
			// versions of Go, we skeptically parse the atime and ctime fields.
			// If we are unable to parse them and the prefix field looks like
			// an ASCII string, then we fallback on the pre-Go1.8 behavior
			// of treating these fields as the USTAR prefix field.
			//
			// Note that this will not use the fallback logic for all possible
			// files generated by a pre-Go1.8 toolchain. If the generated file
			// happened to have a prefix field that parses as valid
			// atime and ctime fields (e.g., when they are valid octal strings),
			// then it is impossible to distinguish between a valid GNU file
			// and an invalid pre-Go1.8 file.
			//
			// See https://golang.org/issues/12594
			// See https://golang.org/issues/21005
			if p2.err != nil {
				hdr.AccessTime, hdr.ChangeTime = time.Time{}, time.Time{}
				ustar := tr.blk.toUSTAR()
				if s := p.parseString(ustar.prefix()); isASCII(s) {
					prefix = s
				}
				hdr.Format = FormatUnknown // Buggy file is not GNU
			}
		}
		if len(prefix) > 0 {
			hdr.Name = prefix + "/" + hdr.Name
		}
	}
	return hdr, &tr.blk, p.err
}

// readOldGNUSparseMap reads the sparse map from the old GNU sparse format.
// The sparse map is stored in the tar header if it's small enough.
// If it's larger than four entries, then one or more extension headers are used
// to store the rest of the sparse map.
//
// The Header.Size does not reflect the size of any extended headers used.
// Thus, this function will read from the raw io.Reader to fetch extra headers.
// This method mutates blk in the process.
func (tr *Reader) readOldGNUSparseMap(hdr *Header, blk *block) (sparseDatas, error) {
	// Make sure that the input format is GNU.
	// Unfortunately, the STAR format also has a sparse header format that uses
	// the same type flag but has a completely different layout.
	if blk.getFormat() != FormatGNU {
		return nil, ErrHeader
	}
	hdr.Format.mayOnlyBe(FormatGNU)

	var p parser
	hdr.Size = p.parseNumeric(blk.toGNU().realSize())
	if p.err != nil {
		return nil, p.err
	}
	s := blk.toGNU().sparse()
	spd := make(sparseDatas, 0, s.maxEntries())
	totalSize := len(s)
	for totalSize < maxSpecialFileSize {
		for i := 0; i < s.maxEntries(); i++ {
			// This termination condition is identical to GNU and BSD tar.
			if s.entry(i).offset()[0] == 0x00 {
				break // Don't return, need to process extended headers (even if empty)
			}
			offset := p.parseNumeric(s.entry(i).offset())
			length := p.parseNumeric(s.entry(i).length())
			if p.err != nil {
				return nil, p.err
			}
			var err error
			spd, err = appendSparseEntry(spd, sparseEntry{Offset: offset, Length: length})
			if err != nil {
				return nil, err
			}
		}

		if s.isExtended()[0] > 0 {
			// There are more entries. Read an extension header and parse its entries.
			if _, err := mustReadFull(tr.r, blk[:]); err != nil {
				return nil, err
			}
			s = blk.toSparse()
			totalSize += len(s)
			continue
		}
		return spd, nil // Done
	}
	return nil, errSparseTooLong
}

// readGNUSparseMap1x0 reads the sparse map as stored in GNU's PAX sparse format
// version 1.0. The format of the sparse map consists of a series of
// newline-terminated numeric fields. The first field is the number of entries
// and is always present. Following this are the entries, consisting of two
// fields (offset, length). This function must stop reading at the end
// boundary of the block containing the last newline.
//
// Note that the GNU manual says that numeric values should be encoded in octal
// format. However, the GNU tar utility itself outputs these values in decimal.
// As such, this library treats values as being encoded in decimal.
func readGNUSparseMap1x0(r io.Reader) (sparseDatas, error) {
	var (
		cntNewline int64
		buf        bytes.Buffer
		blk        block
		totalSize  int
	)

	// feedTokens copies data in blocks from r into buf until there are
	// at least cnt newlines in buf. It will not read more blocks than needed.
	feedTokens := func(n int64) error {
		for cntNewline < n {
			totalSize += len(blk)
			if totalSize > maxSpecialFileSize {
				return errSparseTooLong
			}
			if _, err := mustReadFull(r, blk[:]); err != nil {
				return err
			}
			buf.Write(blk[:])
			for _, c := range blk {
				if c == '\n' {
					cntNewline++
				}
			}
		}
		return nil
	}

	// nextToken gets the next token delimited by a newline. This assumes that
	// at least one newline exists in the buffer.
	nextToken := func() string {
		cntNewline--
		tok, _ := buf.ReadString('\n')
		return strings.TrimRight(tok, "\n")
	}

	// Parse for the number of entries.
	// Use integer overflow resistant math to check this.
	if err := feedTokens(1); err != nil {
		return nil, err
	}
	numEntries, err := strconv.ParseInt(nextToken(), 10, 0) // Intentionally parse as native int
	if err != nil || numEntries < 0 || int(2*numEntries) < int(numEntries) {
		return nil, ErrHeader
	}

	// Parse for all member entries.
	// numEntries is trusted after this since feedTokens limits the number of
	// tokens based on maxSpecialFileSize.
	if err := feedTokens(2 * numEntries); err != nil {
		return nil, err
	}
	spd := make(sparseDatas, 0, numEntries)
	for i := int64(0); i < numEntries; i++ {
		offset, err1 := strconv.ParseInt(nextToken(), 10, 64)
		length, err2 := strconv.ParseInt(nextToken(), 10, 64)
		if err1 != nil || err2 != nil {
			return nil, ErrHeader
		}
		spd, err = appendSparseEntry(spd, sparseEntry{Offset: offset, Length: length})
		if err != nil {
			return nil, err
		}
	}
	return spd, nil
}

// readGNUSparseMap0x1 reads the sparse map as stored in GNU's PAX sparse format
// version 0.1. The sparse map is stored in the PAX headers.
func readGNUSparseMap0x1(paxHdrs map[string]string) (sparseDatas, error) {
	// Get number of entries.
	// Use integer overflow resistant math to check this.
	numEntriesStr := paxHdrs[paxGNUSparseNumBlocks]
	numEntries, err := strconv.ParseInt(numEntriesStr, 10, 0) // Intentionally parse as native int
	if err != nil || numEntries < 0 || int(2*numEntries) < int(numEntries) {
		return nil, ErrHeader
	}

	// There should be two numbers in sparseMap for each entry.
	sparseMap := strings.Split(paxHdrs[paxGNUSparseMap], ",")
	if len(sparseMap) == 1 && sparseMap[0] == "" {
		sparseMap = sparseMap[:0]
	}
	if int64(len(sparseMap)) != 2*numEntries {
		return nil, ErrHeader
	}

	// Loop through the entries in the sparse map.
	// numEntries is trusted now.
	spd := make(sparseDatas, 0, numEntries)
	for len(sparseMap) >= 2 {
		offset, err1 := strconv.ParseInt(sparseMap[0], 10, 64)
		length, err2 := strconv.ParseInt(sparseMap[1], 10, 64)
		if err1 != nil || err2 != nil {
			return nil, ErrHeader
		}
		spd, err = appendSparseEntry(spd, sparseEntry{Offset: offset, Length: length})
		if err != nil {
			return nil, err
		}
		sparseMap = sparseMap[2:]
	}
	return spd, nil
}

func appendSparseEntry(spd sparseDatas, ent sparseEntry) (sparseDatas, error) {
	if len(spd) >= maxSparseFileEntries {
		return nil, errSparseTooLong
	}
	return append(spd, ent), nil
}

// Read reads from the current file in the tar archive.
// It returns (0, io.EOF) when it reaches the end of that file,
// until [Next] is called to advance to the next file.
//
// If the current file is sparse, then the regions marked as a hole
// are read back as NUL-bytes.
//
// Calling Read on special types like [TypeLink], [TypeSymlink], [TypeChar],
// [TypeBlock], [TypeDir], and [TypeFifo] returns (0, [io.EOF]) regardless of what
// the [Header.Size] claims.
func (tr *Reader) Read(b []byte) (int, error) {
	if tr.err != nil {
		return 0, tr.err
	}
	n, err := tr.curr.Read(b)
	if err != nil && err != io.EOF {
		tr.err = err
	}
	return n, err
}

// writeTo writes the content of the current file to w.
// The bytes written matches the number of remaining bytes in the current file.
//
// If the current file is sparse and w is an io.WriteSeeker,
// then writeTo uses Seek to skip past holes defined in Header.SparseHoles,
// assuming that skipped regions are filled with NULs.
// This always writes the last byte to ensure w is the right size.
//
// TODO(dsnet): Re-export this when adding sparse file support.
// See https://golang.org/issue/22735
func (tr *Reader) writeTo(w io.Writer) (int64, error) {
	if tr.err != nil {
		return 0, tr.err
	}
	n, err := tr.curr.WriteTo(w)
	if err != nil {
		tr.err = err
	}
	return n, err
}

// regFileReader is a fileReader for reading data from a regular file entry.
type regFileReader struct {
	r  io.Reader // Underlying Reader
	nb int64     // Number of remaining bytes to read
}

func (fr *regFileReader) Read(b []byte) (n int, err error) {
	if int64(len(b)) > fr.nb {
		b = b[:fr.nb]
	}
	if len(b) > 0 {
		n, err = fr.r.Read(b)
		fr.nb -= int64(n)
	}
	switch {
	case err == io.EOF && fr.nb > 0:
		return n, io.ErrUnexpectedEOF
	case err == nil && fr.nb == 0:
		return n, io.EOF
	default:
		return n, err
	}
}

func (fr *regFileReader) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, struct{ io.Reader }{fr})
}

// logicalRemaining implements fileState.logicalRemaining.
func (fr regFileReader) logicalRemaining() int64 {
	return fr.nb
}

// physicalRemaining implements fileState.physicalRemaining.
func (fr regFileReader) physicalRemaining() int64 {
	return fr.nb
}

// sparseFileReader is a fileReader for reading data from a sparse file entry.
type sparseFileReader struct {
	fr  fileReader  // Underlying fileReader
	sp  sparseHoles // Normalized list of sparse holes
	pos int64       // Current position in sparse file
}

func (sr *sparseFileReader) Read(b []byte) (n int, err error) {
	finished := int64(len(b)) >= sr.logicalRemaining()
	if finished {
		b = b[:sr.logicalRemaining()]
	}

	b0 := b
	endPos := sr.pos + int64(len(b))
	for endPos > sr.pos && err == nil {
		var nf int // Bytes read in fragment
		holeStart, holeEnd := sr.sp[0].Offset, sr.sp[0].endOffset()
		if sr.pos < holeStart { // In a data fragment
			bf := b[:min(int64(len(b)), holeStart-sr.pos)]
			nf, err = tryReadFull(sr.fr, bf)
		} else { // In a hole fragment
			bf := b[:min(int64(len(b)), holeEnd-sr.pos)]
			nf, err = tryReadFull(zeroReader{}, bf)
		}
		b = b[nf:]
		sr.pos += int64(nf)
		if sr.pos >= holeEnd && len(sr.sp) > 1 {
			sr.sp = sr.sp[1:] // Ensure last fragment always remains
		}
	}

	n = len(b0) - len(b)
	switch {
	case err == io.EOF:
		return n, errMissData // Less data in dense file than sparse file
	case err != nil:
		return n, err
	case sr.logicalRemaining() == 0 && sr.physicalRemaining() > 0:
		return n, errUnrefData // More data in dense file than sparse file
	case finished:
		return n, io.EOF
	default:
		return n, nil
	}
}

func (sr *sparseFileReader) WriteTo(w io.Writer) (n int64, err error) {
	ws, ok := w.(io.WriteSeeker)
	if ok {
		if _, err := ws.Seek(0, io.SeekCurrent); err != nil {
			ok = false // Not all io.Seeker can really seek
		}
	}
	if !ok {
		return io.Copy(w, struct{ io.Reader }{sr})
	}

	var writeLastByte bool
	pos0 := sr.pos
	for sr.logicalRemaining() > 0 && !writeLastByte && err == nil {
		var nf int64 // Size of fragment
		holeStart, holeEnd := sr.sp[0].Offset, sr.sp[0].endOffset()
		if sr.pos < holeStart { // In a data fragment
			nf = holeStart - sr.pos
			nf, err = io.CopyN(ws, sr.fr, nf)
		} else { // In a hole fragment
			nf = holeEnd - sr.pos
			if sr.physicalRemaining() == 0 {
				writeLastByte = true
				nf--
			}
			_, err = ws.Seek(nf, io.SeekCurrent)
		}
		sr.pos += nf
		if sr.pos >= holeEnd && len(sr.sp) > 1 {
			sr.sp = sr.sp[1:] // Ensure last fragment always remains
		}
	}

	// If the last fragment is a hole, then seek to 1-byte before EOF, and
	// write a single byte to ensure the file is the right size.
	if writeLastByte && err == nil {
		_, err = ws.Write([]byte{0})
		sr.pos++
	}

	n = sr.pos - pos0
	switch {
	case err == io.EOF:
		return n, errMissData // Less data in dense file than sparse file
	case err != nil:
		return n, err
	case sr.logicalRemaining() == 0 && sr.physicalRemaining() > 0:
		return n, errUnrefData // More data in dense file than sparse file
	default:
		return n, nil
	}
}

func (sr sparseFileReader) logicalRemaining() int64 {
	return sr.sp[len(sr.sp)-1].endOffset() - sr.pos
}
func (sr sparseFileReader) physicalRemaining() int64 {
	return sr.fr.physicalRemaining()
}

type zeroReader struct{}

func (zeroReader) Read(b []byte) (int, error) {
	clear(b)
	return len(b), nil
}

// mustReadFull is like io.ReadFull except it returns
// io.ErrUnexpectedEOF when io.EOF is hit before len(b) bytes are read.
func mustReadFull(r io.Reader, b []byte) (int, error) {
	n, err := tryReadFull(r, b)
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

// tryReadFull is like io.ReadFull except it returns
// io.EOF when it is hit before len(b) bytes are read.
func tryReadFull(r io.Reader, b []byte) (n int, err error) {
	for len(b) > n && err == nil {
		var nn int
		nn, err = r.Read(b[n:])
		n += nn
	}
	if len(b) == n && err == io.EOF {
		err = nil
	}
	return n, err
}

// readSpecialFile is like io.ReadAll except it returns
// ErrFieldTooLong if more than maxSpecialFileSize is read.
func readSpecialFile(r io.Reader) ([]byte, error) {
	buf, err := io.ReadAll(io.LimitReader(r, maxSpecialFileSize+1))
	if len(buf) > maxSpecialFileSize {
		return nil, ErrFieldTooLong
	}
	return buf, err
}

// discard skips n bytes in r, reporting an error if unable to do so.
func discard(r io.Reader, n int64) error {
	// If possible, Seek to the last byte before the end of the data section.
	// Do this because Seek is often lazy about reporting errors; this will mask
	// the fact that the stream may be truncated. We can rely on the
	// io.CopyN done shortly afterwards to trigger any IO errors.
	var seekSkipped int64 // Number of bytes skipped via Seek
	if sr, ok := r.(io.Seeker); ok && n > 1 {
		// Not all io.Seeker can actually Seek. For example, os.Stdin implements
		// io.Seeker, but calling Seek always returns an error and performs
		// no action. Thus, we try an innocent seek to the current position
		// to see if Seek is really supported.
		pos1, err := sr.Seek(0, io.SeekCurrent)
		if pos1 >= 0 && err == nil {
			// Seek seems supported, so perform the real Seek.
			pos2, err := sr.Seek(n-1, io.SeekCurrent)
			if pos2 < 0 || err != nil {
				return err
			}
			seekSkipped = pos2 - pos1
		}
	}

	copySkipped, err := io.CopyN(io.Discard, r, n-seekSkipped)
	if err == io.EOF && seekSkipped+copySkipped < n {
		err = io.ErrUnexpectedEOF
	}
	return err
}

```

// === FILE: references/go/src/archive/tar/stat_actime1.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || linux || dragonfly || openbsd || solaris

package tar

import (
	"syscall"
	"time"
)

func statAtime(st *syscall.Stat_t) time.Time {
	return time.Unix(st.Atim.Unix())
}

func statCtime(st *syscall.Stat_t) time.Time {
	return time.Unix(st.Ctim.Unix())
}

```

// === FILE: references/go/src/archive/tar/stat_actime2.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || freebsd || netbsd

package tar

import (
	"syscall"
	"time"
)

func statAtime(st *syscall.Stat_t) time.Time {
	return time.Unix(st.Atimespec.Unix())
}

func statCtime(st *syscall.Stat_t) time.Time {
	return time.Unix(st.Ctimespec.Unix())
}

```

// === FILE: references/go/src/archive/tar/stat_unix.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package tar

import (
	"io/fs"
	"os/user"
	"runtime"
	"strconv"
	"sync"
	"syscall"
)

func init() {
	sysStat = statUnix
}

// userMap and groupMap cache UID and GID lookups for performance reasons.
// The downside is that renaming uname or gname by the OS never takes effect.
var userMap, groupMap sync.Map // map[int]string

func statUnix(fi fs.FileInfo, h *Header, doNameLookups bool) error {
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	h.Uid = int(sys.Uid)
	h.Gid = int(sys.Gid)
	if doNameLookups {
		// Best effort at populating Uname and Gname.
		// The os/user functions may fail for any number of reasons
		// (not implemented on that platform, cgo not enabled, etc).
		if u, ok := userMap.Load(h.Uid); ok {
			h.Uname = u.(string)
		} else if u, err := user.LookupId(strconv.Itoa(h.Uid)); err == nil {
			h.Uname = u.Username
			userMap.Store(h.Uid, h.Uname)
		}
		if g, ok := groupMap.Load(h.Gid); ok {
			h.Gname = g.(string)
		} else if g, err := user.LookupGroupId(strconv.Itoa(h.Gid)); err == nil {
			h.Gname = g.Name
			groupMap.Store(h.Gid, h.Gname)
		}
	}
	h.AccessTime = statAtime(sys)
	h.ChangeTime = statCtime(sys)

	// Best effort at populating Devmajor and Devminor.
	if h.Typeflag == TypeChar || h.Typeflag == TypeBlock {
		dev := uint64(sys.Rdev) // May be int32 or uint32
		switch runtime.GOOS {
		case "aix":
			var major, minor uint32
			major = uint32((dev & 0x3fffffff00000000) >> 32)
			minor = uint32((dev & 0x00000000ffffffff) >> 0)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "linux":
			// Copied from golang.org/x/sys/unix/dev_linux.go.
			major := uint32((dev & 0x00000000000fff00) >> 8)
			major |= uint32((dev & 0xfffff00000000000) >> 32)
			minor := uint32((dev & 0x00000000000000ff) >> 0)
			minor |= uint32((dev & 0x00000ffffff00000) >> 12)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "darwin", "ios":
			// Copied from golang.org/x/sys/unix/dev_darwin.go.
			major := uint32((dev >> 24) & 0xff)
			minor := uint32(dev & 0xffffff)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "dragonfly":
			// Copied from golang.org/x/sys/unix/dev_dragonfly.go.
			major := uint32((dev >> 8) & 0xff)
			minor := uint32(dev & 0xffff00ff)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "freebsd":
			// Copied from golang.org/x/sys/unix/dev_freebsd.go.
			major := uint32((dev >> 8) & 0xff)
			minor := uint32(dev & 0xffff00ff)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "netbsd":
			// Copied from golang.org/x/sys/unix/dev_netbsd.go.
			major := uint32((dev & 0x000fff00) >> 8)
			minor := uint32((dev & 0x000000ff) >> 0)
			minor |= uint32((dev & 0xfff00000) >> 12)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		case "openbsd":
			// Copied from golang.org/x/sys/unix/dev_openbsd.go.
			major := uint32((dev & 0x0000ff00) >> 8)
			minor := uint32((dev & 0x000000ff) >> 0)
			minor |= uint32((dev & 0xffff0000) >> 8)
			h.Devmajor, h.Devminor = int64(major), int64(minor)
		default:
			// TODO: Implement solaris (see https://golang.org/issue/8106)
		}
	}
	return nil
}

```

// === FILE: references/go/src/archive/tar/strconv.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tar

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// hasNUL reports whether the NUL character exists within s.
func hasNUL(s string) bool {
	return strings.Contains(s, "\x00")
}

// isASCII reports whether the input is an ASCII C-style string.
func isASCII(s string) bool {
	for _, c := range s {
		if c >= 0x80 || c == 0x00 {
			return false
		}
	}
	return true
}

// toASCII converts the input to an ASCII C-style string.
// This is a best effort conversion, so invalid characters are dropped.
func toASCII(s string) string {
	if isASCII(s) {
		return s
	}
	b := make([]byte, 0, len(s))
	for _, c := range s {
		if c < 0x80 && c != 0x00 {
			b = append(b, byte(c))
		}
	}
	return string(b)
}

type parser struct {
	err error // Last error seen
}

type formatter struct {
	err error // Last error seen
}

// parseString parses bytes as a NUL-terminated C-style string.
// If a NUL byte is not found then the whole slice is returned as a string.
func (*parser) parseString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}

// formatString copies s into b, NUL-terminating if possible.
func (f *formatter) formatString(b []byte, s string) {
	if len(s) > len(b) {
		f.err = ErrFieldTooLong
	}
	copy(b, s)
	if len(s) < len(b) {
		b[len(s)] = 0
	}

	// Some buggy readers treat regular files with a trailing slash
	// in the V7 path field as a directory even though the full path
	// recorded elsewhere (e.g., via PAX record) contains no trailing slash.
	if len(s) > len(b) && b[len(b)-1] == '/' {
		n := len(strings.TrimRight(s[:len(b)-1], "/"))
		b[n] = 0 // Replace trailing slash with NUL terminator
	}
}

// fitsInBase256 reports whether x can be encoded into n bytes using base-256
// encoding. Unlike octal encoding, base-256 encoding does not require that the
// string ends with a NUL character. Thus, all n bytes are available for output.
//
// If operating in binary mode, this assumes strict GNU binary mode; which means
// that the first byte can only be either 0x80 or 0xff. Thus, the first byte is
// equivalent to the sign bit in two's complement form.
func fitsInBase256(n int, x int64) bool {
	binBits := uint(n-1) * 8
	return n >= 9 || (x >= -1<<binBits && x < 1<<binBits)
}

// parseNumeric parses the input as being encoded in either base-256 or octal.
// This function may return negative numbers.
// If parsing fails or an integer overflow occurs, err will be set.
func (p *parser) parseNumeric(b []byte) int64 {
	// Check for base-256 (binary) format first.
	// If the first bit is set, then all following bits constitute a two's
	// complement encoded number in big-endian byte order.
	if len(b) > 0 && b[0]&0x80 != 0 {
		// Handling negative numbers relies on the following identity:
		//	-a-1 == ^a
		//
		// If the number is negative, we use an inversion mask to invert the
		// data bytes and treat the value as an unsigned number.
		var inv byte // 0x00 if positive or zero, 0xff if negative
		if b[0]&0x40 != 0 {
			inv = 0xff
		}

		var x uint64
		for i, c := range b {
			c ^= inv // Inverts c only if inv is 0xff, otherwise does nothing
			if i == 0 {
				c &= 0x7f // Ignore signal bit in first byte
			}
			if (x >> 56) > 0 {
				p.err = ErrHeader // Integer overflow
				return 0
			}
			x = x<<8 | uint64(c)
		}
		if (x >> 63) > 0 {
			p.err = ErrHeader // Integer overflow
			return 0
		}
		if inv == 0xff {
			return ^int64(x)
		}
		return int64(x)
	}

	// Normal case is base-8 (octal) format.
	return p.parseOctal(b)
}

// formatNumeric encodes x into b using base-8 (octal) encoding if possible.
// Otherwise it will attempt to use base-256 (binary) encoding.
func (f *formatter) formatNumeric(b []byte, x int64) {
	if fitsInOctal(len(b), x) {
		f.formatOctal(b, x)
		return
	}

	if fitsInBase256(len(b), x) {
		for i := len(b) - 1; i >= 0; i-- {
			b[i] = byte(x)
			x >>= 8
		}
		b[0] |= 0x80 // Highest bit indicates binary format
		return
	}

	f.formatOctal(b, 0) // Last resort, just write zero
	f.err = ErrFieldTooLong
}

func (p *parser) parseOctal(b []byte) int64 {
	// Because unused fields are filled with NULs, we need
	// to skip leading NULs. Fields may also be padded with
	// spaces or NULs.
	// So we remove leading and trailing NULs and spaces to
	// be sure.
	b = bytes.Trim(b, " \x00")

	if len(b) == 0 {
		return 0
	}
	x, perr := strconv.ParseUint(p.parseString(b), 8, 64)
	if perr != nil {
		p.err = ErrHeader
	}
	return int64(x)
}

func (f *formatter) formatOctal(b []byte, x int64) {
	if !fitsInOctal(len(b), x) {
		x = 0 // Last resort, just write zero
		f.err = ErrFieldTooLong
	}

	s := strconv.FormatInt(x, 8)
	// Add leading zeros, but leave room for a NUL.
	if n := len(b) - len(s) - 1; n > 0 {
		s = strings.Repeat("0", n) + s
	}
	f.formatString(b, s)
}

// fitsInOctal reports whether the integer x fits in a field n-bytes long
// using octal encoding with the appropriate NUL terminator.
func fitsInOctal(n int, x int64) bool {
	octBits := uint(n-1) * 3
	return x >= 0 && (n >= 22 || x < 1<<octBits)
}

// parsePAXTime takes a string of the form %d.%d as described in the PAX
// specification. Note that this implementation allows for negative timestamps,
// which is allowed for by the PAX specification, but not always portable.
func parsePAXTime(s string) (time.Time, error) {
	const maxNanoSecondDigits = 9

	// Split string into seconds and sub-seconds parts.
	ss, sn, _ := strings.Cut(s, ".")

	// Parse the seconds.
	secs, err := strconv.ParseInt(ss, 10, 64)
	if err != nil {
		return time.Time{}, ErrHeader
	}
	if len(sn) == 0 {
		return time.Unix(secs, 0), nil // No sub-second values
	}

	// Parse the nanoseconds.
	// Initialize an array with '0's to handle right padding automatically.
	nanoDigits := [maxNanoSecondDigits]byte{'0', '0', '0', '0', '0', '0', '0', '0', '0'}
	for i := range len(sn) {
		switch c := sn[i]; {
		case c < '0' || c > '9':
			return time.Time{}, ErrHeader
		case i < len(nanoDigits):
			nanoDigits[i] = c
		}
	}
	nsecs, _ := strconv.ParseInt(string(nanoDigits[:]), 10, 64) // Must succeed after validation
	if len(ss) > 0 && ss[0] == '-' {
		return time.Unix(secs, -1*nsecs), nil // Negative correction
	}
	return time.Unix(secs, nsecs), nil
}

// formatPAXTime converts ts into a time of the form %d.%d as described in the
// PAX specification. This function is capable of negative timestamps.
func formatPAXTime(ts time.Time) (s string) {
	secs, nsecs := ts.Unix(), ts.Nanosecond()
	if nsecs == 0 {
		return strconv.FormatInt(secs, 10)
	}

	// If seconds is negative, then perform correction.
	sign := ""
	if secs < 0 {
		sign = "-"             // Remember sign
		secs = -(secs + 1)     // Add a second to secs
		nsecs = -(nsecs - 1e9) // Take that second away from nsecs
	}
	return strings.TrimRight(fmt.Sprintf("%s%d.%09d", sign, secs, nsecs), "0")
}

// parsePAXRecord parses the input PAX record string into a key-value pair.
// If parsing is successful, it will slice off the currently read record and
// return the remainder as r.
func parsePAXRecord(s string) (k, v, r string, err error) {
	// The size field ends at the first space.
	nStr, rest, ok := strings.Cut(s, " ")
	if !ok {
		return "", "", s, ErrHeader
	}

	// Parse the first token as a decimal integer.
	n, perr := strconv.ParseInt(nStr, 10, 0) // Intentionally parse as native int
	if perr != nil || n < 5 || n > int64(len(s)) {
		return "", "", s, ErrHeader
	}
	n -= int64(len(nStr) + 1) // convert from index in s to index in rest
	if n <= 0 {
		return "", "", s, ErrHeader
	}

	// Extract everything between the space and the final newline.
	rec, nl, rem := rest[:n-1], rest[n-1:n], rest[n:]
	if nl != "\n" {
		return "", "", s, ErrHeader
	}

	// The first equals separates the key from the value.
	k, v, ok = strings.Cut(rec, "=")
	if !ok {
		return "", "", s, ErrHeader
	}

	if !validPAXRecord(k, v) {
		return "", "", s, ErrHeader
	}
	return k, v, rem, nil
}

// formatPAXRecord formats a single PAX record, prefixing it with the
// appropriate length.
func formatPAXRecord(k, v string) (string, error) {
	if !validPAXRecord(k, v) {
		return "", ErrHeader
	}

	const padding = 3 // Extra padding for ' ', '=', and '\n'
	size := len(k) + len(v) + padding
	size += len(strconv.Itoa(size))
	record := strconv.Itoa(size) + " " + k + "=" + v + "\n"

	// Final adjustment if adding size field increased the record size.
	if len(record) != size {
		size = len(record)
		record = strconv.Itoa(size) + " " + k + "=" + v + "\n"
	}
	return record, nil
}

// validPAXRecord reports whether the key-value pair is valid where each
// record is formatted as:
//
//	"%d %s=%s\n" % (size, key, value)
//
// Keys and values should be UTF-8, but the number of bad writers out there
// forces us to be more liberal.
// Thus, we only reject all keys with NUL, and only reject NULs in values
// for the PAX version of the USTAR string fields.
// The key must not contain an '=' character.
func validPAXRecord(k, v string) bool {
	if k == "" || strings.Contains(k, "=") {
		return false
	}
	switch k {
	case paxPath, paxLinkpath, paxUname, paxGname:
		return !hasNUL(v)
	default:
		return !hasNUL(k)
	}
}

```

// === FILE: references/go/src/archive/tar/writer.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tar

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"time"
)

// Writer provides sequential writing of a tar archive.
// [Writer.WriteHeader] begins a new file with the provided [Header],
// and then Writer can be treated as an io.Writer to supply that file's data.
type Writer struct {
	w    io.Writer
	pad  int64      // Amount of padding to write after current file entry
	curr fileWriter // Writer for current file entry
	hdr  Header     // Shallow copy of Header that is safe for mutations
	blk  block      // Buffer to use as temporary local storage

	// err is a persistent error.
	// It is only the responsibility of every exported method of Writer to
	// ensure that this error is sticky.
	err error
}

// NewWriter creates a new Writer writing to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w, curr: &regFileWriter{w, 0}}
}

type fileWriter interface {
	io.Writer
	fileState

	ReadFrom(io.Reader) (int64, error)
}

// Flush finishes writing the current file's block padding.
// The current file must be fully written before Flush can be called.
//
// This is unnecessary as the next call to [Writer.WriteHeader] or [Writer.Close]
// will implicitly flush out the file's padding.
func (tw *Writer) Flush() error {
	if tw.err != nil {
		return tw.err
	}
	if nb := tw.curr.logicalRemaining(); nb > 0 {
		return fmt.Errorf("archive/tar: missed writing %d bytes", nb)
	}
	if _, tw.err = tw.w.Write(zeroBlock[:tw.pad]); tw.err != nil {
		return tw.err
	}
	tw.pad = 0
	return nil
}

// WriteHeader writes hdr and prepares to accept the file's contents.
// The Header.Size determines how many bytes can be written for the next file.
// If the current file is not fully written, then this returns an error.
// This implicitly flushes any padding necessary before writing the header.
func (tw *Writer) WriteHeader(hdr *Header) error {
	if err := tw.Flush(); err != nil {
		return err
	}
	tw.hdr = *hdr // Shallow copy of Header

	// Avoid usage of the legacy TypeRegA flag, and automatically promote
	// it to use TypeReg or TypeDir.
	if tw.hdr.Typeflag == TypeRegA {
		if strings.HasSuffix(tw.hdr.Name, "/") {
			tw.hdr.Typeflag = TypeDir
		} else {
			tw.hdr.Typeflag = TypeReg
		}
	}

	// Round ModTime and ignore AccessTime and ChangeTime unless
	// the format is explicitly chosen.
	// This ensures nominal usage of WriteHeader (without specifying the format)
	// does not always result in the PAX format being chosen, which
	// causes a 1KiB increase to every header.
	if tw.hdr.Format == FormatUnknown {
		tw.hdr.ModTime = tw.hdr.ModTime.Round(time.Second)
		tw.hdr.AccessTime = time.Time{}
		tw.hdr.ChangeTime = time.Time{}
	}

	allowedFormats, paxHdrs, err := tw.hdr.allowedFormats()
	switch {
	case allowedFormats.has(FormatUSTAR):
		tw.err = tw.writeUSTARHeader(&tw.hdr)
		return tw.err
	case allowedFormats.has(FormatPAX):
		tw.err = tw.writePAXHeader(&tw.hdr, paxHdrs)
		return tw.err
	case allowedFormats.has(FormatGNU):
		tw.err = tw.writeGNUHeader(&tw.hdr)
		return tw.err
	default:
		return err // Non-fatal error
	}
}

func (tw *Writer) writeUSTARHeader(hdr *Header) error {
	// Check if we can use USTAR prefix/suffix splitting.
	var namePrefix string
	if prefix, suffix, ok := splitUSTARPath(hdr.Name); ok {
		namePrefix, hdr.Name = prefix, suffix
	}

	// Pack the main header.
	var f formatter
	blk := tw.templateV7Plus(hdr, f.formatString, f.formatOctal)
	f.formatString(blk.toUSTAR().prefix(), namePrefix)
	blk.setFormat(FormatUSTAR)
	if f.err != nil {
		return f.err // Should never happen since header is validated
	}
	return tw.writeRawHeader(blk, hdr.Size, hdr.Typeflag)
}

func (tw *Writer) writePAXHeader(hdr *Header, paxHdrs map[string]string) error {
	realName, realSize := hdr.Name, hdr.Size

	// TODO(dsnet): Re-enable this when adding sparse support.
	// See https://golang.org/issue/22735
	/*
		// Handle sparse files.
		var spd sparseDatas
		var spb []byte
		if len(hdr.SparseHoles) > 0 {
			sph := append([]sparseEntry{}, hdr.SparseHoles...) // Copy sparse map
			sph = alignSparseEntries(sph, hdr.Size)
			spd = invertSparseEntries(sph, hdr.Size)

			// Format the sparse map.
			hdr.Size = 0 // Replace with encoded size
			spb = append(strconv.AppendInt(spb, int64(len(spd)), 10), '\n')
			for _, s := range spd {
				hdr.Size += s.Length
				spb = append(strconv.AppendInt(spb, s.Offset, 10), '\n')
				spb = append(strconv.AppendInt(spb, s.Length, 10), '\n')
			}
			pad := blockPadding(int64(len(spb)))
			spb = append(spb, zeroBlock[:pad]...)
			hdr.Size += int64(len(spb)) // Accounts for encoded sparse map

			// Add and modify appropriate PAX records.
			dir, file := path.Split(realName)
			hdr.Name = path.Join(dir, "GNUSparseFile.0", file)
			paxHdrs[paxGNUSparseMajor] = "1"
			paxHdrs[paxGNUSparseMinor] = "0"
			paxHdrs[paxGNUSparseName] = realName
			paxHdrs[paxGNUSparseRealSize] = strconv.FormatInt(realSize, 10)
			paxHdrs[paxSize] = strconv.FormatInt(hdr.Size, 10)
			delete(paxHdrs, paxPath) // Recorded by paxGNUSparseName
		}
	*/
	_ = realSize

	// Write PAX records to the output.
	isGlobal := hdr.Typeflag == TypeXGlobalHeader
	if len(paxHdrs) > 0 || isGlobal {
		// Write each record to a buffer.
		var buf strings.Builder
		// Sort keys for deterministic ordering.
		for _, k := range slices.Sorted(maps.Keys(paxHdrs)) {
			rec, err := formatPAXRecord(k, paxHdrs[k])
			if err != nil {
				return err
			}
			buf.WriteString(rec)
		}

		// Write the extended header file.
		var name string
		var flag byte
		if isGlobal {
			name = realName
			if name == "" {
				name = "GlobalHead.0.0"
			}
			flag = TypeXGlobalHeader
		} else {
			dir, file := path.Split(realName)
			name = path.Join(dir, "PaxHeaders.0", file)
			flag = TypeXHeader
		}
		data := buf.String()
		if len(data) > maxSpecialFileSize {
			return ErrFieldTooLong
		}
		if err := tw.writeRawFile(name, data, flag, FormatPAX); err != nil || isGlobal {
			return err // Global headers return here
		}
	}

	// Pack the main header.
	var f formatter // Ignore errors since they are expected
	fmtStr := func(b []byte, s string) { f.formatString(b, toASCII(s)) }
	blk := tw.templateV7Plus(hdr, fmtStr, f.formatOctal)
	blk.setFormat(FormatPAX)
	if err := tw.writeRawHeader(blk, hdr.Size, hdr.Typeflag); err != nil {
		return err
	}

	// TODO(dsnet): Re-enable this when adding sparse support.
	// See https://golang.org/issue/22735
	/*
		// Write the sparse map and setup the sparse writer if necessary.
		if len(spd) > 0 {
			// Use tw.curr since the sparse map is accounted for in hdr.Size.
			if _, err := tw.curr.Write(spb); err != nil {
				return err
			}
			tw.curr = &sparseFileWriter{tw.curr, spd, 0}
		}
	*/
	return nil
}

func (tw *Writer) writeGNUHeader(hdr *Header) error {
	// Use long-link files if Name or Linkname exceeds the field size.
	const longName = "././@LongLink"
	if len(hdr.Name) > nameSize {
		data := hdr.Name + "\x00"
		if err := tw.writeRawFile(longName, data, TypeGNULongName, FormatGNU); err != nil {
			return err
		}
	}
	if len(hdr.Linkname) > nameSize {
		data := hdr.Linkname + "\x00"
		if err := tw.writeRawFile(longName, data, TypeGNULongLink, FormatGNU); err != nil {
			return err
		}
	}

	// Pack the main header.
	var f formatter // Ignore errors since they are expected
	var spd sparseDatas
	var spb []byte
	blk := tw.templateV7Plus(hdr, f.formatString, f.formatNumeric)
	if !hdr.AccessTime.IsZero() {
		f.formatNumeric(blk.toGNU().accessTime(), hdr.AccessTime.Unix())
	}
	if !hdr.ChangeTime.IsZero() {
		f.formatNumeric(blk.toGNU().changeTime(), hdr.ChangeTime.Unix())
	}
	// TODO(dsnet): Re-enable this when adding sparse support.
	// See https://golang.org/issue/22735
	/*
		if hdr.Typeflag == TypeGNUSparse {
			sph := append([]sparseEntry{}, hdr.SparseHoles...) // Copy sparse map
			sph = alignSparseEntries(sph, hdr.Size)
			spd = invertSparseEntries(sph, hdr.Size)

			// Format the sparse map.
			formatSPD := func(sp sparseDatas, sa sparseArray) sparseDatas {
				for i := 0; len(sp) > 0 && i < sa.MaxEntries(); i++ {
					f.formatNumeric(sa.Entry(i).Offset(), sp[0].Offset)
					f.formatNumeric(sa.Entry(i).Length(), sp[0].Length)
					sp = sp[1:]
				}
				if len(sp) > 0 {
					sa.IsExtended()[0] = 1
				}
				return sp
			}
			sp2 := formatSPD(spd, blk.GNU().Sparse())
			for len(sp2) > 0 {
				var spHdr block
				sp2 = formatSPD(sp2, spHdr.Sparse())
				spb = append(spb, spHdr[:]...)
			}

			// Update size fields in the header block.
			realSize := hdr.Size
			hdr.Size = 0 // Encoded size; does not account for encoded sparse map
			for _, s := range spd {
				hdr.Size += s.Length
			}
			copy(blk.V7().Size(), zeroBlock[:]) // Reset field
			f.formatNumeric(blk.V7().Size(), hdr.Size)
			f.formatNumeric(blk.GNU().RealSize(), realSize)
		}
	*/
	blk.setFormat(FormatGNU)
	if err := tw.writeRawHeader(blk, hdr.Size, hdr.Typeflag); err != nil {
		return err
	}

	// Write the extended sparse map and setup the sparse writer if necessary.
	if len(spd) > 0 {
		// Use tw.w since the sparse map is not accounted for in hdr.Size.
		if _, err := tw.w.Write(spb); err != nil {
			return err
		}
		tw.curr = &sparseFileWriter{tw.curr, spd, 0}
	}
	return nil
}

type (
	stringFormatter func([]byte, string)
	numberFormatter func([]byte, int64)
)

// templateV7Plus fills out the V7 fields of a block using values from hdr.
// It also fills out fields (uname, gname, devmajor, devminor) that are
// shared in the USTAR, PAX, and GNU formats using the provided formatters.
//
// The block returned is only valid until the next call to
// templateV7Plus or writeRawFile.
func (tw *Writer) templateV7Plus(hdr *Header, fmtStr stringFormatter, fmtNum numberFormatter) *block {
	tw.blk.reset()

	modTime := hdr.ModTime
	if modTime.IsZero() {
		modTime = time.Unix(0, 0)
	}

	v7 := tw.blk.toV7()
	v7.typeFlag()[0] = hdr.Typeflag
	fmtStr(v7.name(), hdr.Name)
	fmtStr(v7.linkName(), hdr.Linkname)
	fmtNum(v7.mode(), hdr.Mode)
	fmtNum(v7.uid(), int64(hdr.Uid))
	fmtNum(v7.gid(), int64(hdr.Gid))
	fmtNum(v7.size(), hdr.Size)
	fmtNum(v7.modTime(), modTime.Unix())

	ustar := tw.blk.toUSTAR()
	fmtStr(ustar.userName(), hdr.Uname)
	fmtStr(ustar.groupName(), hdr.Gname)
	fmtNum(ustar.devMajor(), hdr.Devmajor)
	fmtNum(ustar.devMinor(), hdr.Devminor)

	return &tw.blk
}

// writeRawFile writes a minimal file with the given name and flag type.
// It uses format to encode the header format and will write data as the body.
// It uses default values for all of the other fields (as BSD and GNU tar does).
func (tw *Writer) writeRawFile(name, data string, flag byte, format Format) error {
	tw.blk.reset()

	// Best effort for the filename.
	name = toASCII(name)
	if len(name) > nameSize {
		name = name[:nameSize]
	}
	name = strings.TrimRight(name, "/")

	var f formatter
	v7 := tw.blk.toV7()
	v7.typeFlag()[0] = flag
	f.formatString(v7.name(), name)
	f.formatOctal(v7.mode(), 0)
	f.formatOctal(v7.uid(), 0)
	f.formatOctal(v7.gid(), 0)
	f.formatOctal(v7.size(), int64(len(data))) // Must be < 8GiB
	f.formatOctal(v7.modTime(), 0)
	tw.blk.setFormat(format)
	if f.err != nil {
		return f.err // Only occurs if size condition is violated
	}

	// Write the header and data.
	if err := tw.writeRawHeader(&tw.blk, int64(len(data)), flag); err != nil {
		return err
	}
	_, err := io.WriteString(tw, data)
	return err
}

// writeRawHeader writes the value of blk, regardless of its value.
// It sets up the Writer such that it can accept a file of the given size.
// If the flag is a special header-only flag, then the size is treated as zero.
func (tw *Writer) writeRawHeader(blk *block, size int64, flag byte) error {
	if err := tw.Flush(); err != nil {
		return err
	}
	if _, err := tw.w.Write(blk[:]); err != nil {
		return err
	}
	if isHeaderOnlyType(flag) {
		size = 0
	}
	tw.curr = &regFileWriter{tw.w, size}
	tw.pad = blockPadding(size)
	return nil
}

// AddFS adds the files from fs.FS to the archive.
// It walks the directory tree starting at the root of the filesystem
// adding each file to the tar archive while maintaining the directory structure.
func (tw *Writer) AddFS(fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		linkTarget := ""
		if typ := d.Type(); typ == fs.ModeSymlink {
			var err error
			linkTarget, err = fs.ReadLink(fsys, name)
			if err != nil {
				return err
			}
		} else if !typ.IsRegular() && typ != fs.ModeDir {
			return errors.New("tar: cannot add non-regular file")
		}
		h, err := FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		h.Name = name
		if d.IsDir() {
			h.Name += "/"
		}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		f, err := fsys.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

// splitUSTARPath splits a path according to USTAR prefix and suffix rules.
// If the path is not splittable, then it will return ("", "", false).
func splitUSTARPath(name string) (prefix, suffix string, ok bool) {
	length := len(name)
	if length <= nameSize || !isASCII(name) {
		return "", "", false
	} else if length > prefixSize+1 {
		length = prefixSize + 1
	} else if name[length-1] == '/' {
		length--
	}

	i := strings.LastIndex(name[:length], "/")
	nlen := len(name) - i - 1 // nlen is length of suffix
	plen := i                 // plen is length of prefix
	if i <= 0 || nlen > nameSize || nlen == 0 || plen > prefixSize {
		return "", "", false
	}
	return name[:i], name[i+1:], true
}

// Write writes to the current file in the tar archive.
// Write returns the error [ErrWriteTooLong] if more than
// Header.Size bytes are written after [Writer.WriteHeader].
//
// Calling Write on special types like [TypeLink], [TypeSymlink], [TypeChar],
// [TypeBlock], [TypeDir], and [TypeFifo] returns (0, [ErrWriteTooLong]) regardless
// of what the [Header.Size] claims.
func (tw *Writer) Write(b []byte) (int, error) {
	if tw.err != nil {
		return 0, tw.err
	}
	n, err := tw.curr.Write(b)
	if err != nil && err != ErrWriteTooLong {
		tw.err = err
	}
	return n, err
}

// readFrom populates the content of the current file by reading from r.
// The bytes read must match the number of remaining bytes in the current file.
//
// If the current file is sparse and r is an io.ReadSeeker,
// then readFrom uses Seek to skip past holes defined in Header.SparseHoles,
// assuming that skipped regions are all NULs.
// This always reads the last byte to ensure r is the right size.
//
// TODO(dsnet): Re-export this when adding sparse file support.
// See https://golang.org/issue/22735
func (tw *Writer) readFrom(r io.Reader) (int64, error) {
	if tw.err != nil {
		return 0, tw.err
	}
	n, err := tw.curr.ReadFrom(r)
	if err != nil && err != ErrWriteTooLong {
		tw.err = err
	}
	return n, err
}

// Close closes the tar archive by flushing the padding, and writing the footer.
// If the current file (from a prior call to [Writer.WriteHeader]) is not fully written,
// then this returns an error.
func (tw *Writer) Close() error {
	if tw.err == ErrWriteAfterClose {
		return nil
	}
	if tw.err != nil {
		return tw.err
	}

	// Trailer: two zero blocks.
	err := tw.Flush()
	for i := 0; i < 2 && err == nil; i++ {
		_, err = tw.w.Write(zeroBlock[:])
	}

	// Ensure all future actions are invalid.
	tw.err = ErrWriteAfterClose
	return err // Report IO errors
}

// regFileWriter is a fileWriter for writing data to a regular file entry.
type regFileWriter struct {
	w  io.Writer // Underlying Writer
	nb int64     // Number of remaining bytes to write
}

func (fw *regFileWriter) Write(b []byte) (n int, err error) {
	overwrite := int64(len(b)) > fw.nb
	if overwrite {
		b = b[:fw.nb]
	}
	if len(b) > 0 {
		n, err = fw.w.Write(b)
		fw.nb -= int64(n)
	}
	switch {
	case err != nil:
		return n, err
	case overwrite:
		return n, ErrWriteTooLong
	default:
		return n, nil
	}
}

func (fw *regFileWriter) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(struct{ io.Writer }{fw}, r)
}

// logicalRemaining implements fileState.logicalRemaining.
func (fw regFileWriter) logicalRemaining() int64 {
	return fw.nb
}

// physicalRemaining implements fileState.physicalRemaining.
func (fw regFileWriter) physicalRemaining() int64 {
	return fw.nb
}

// sparseFileWriter is a fileWriter for writing data to a sparse file entry.
type sparseFileWriter struct {
	fw  fileWriter  // Underlying fileWriter
	sp  sparseDatas // Normalized list of data fragments
	pos int64       // Current position in sparse file
}

func (sw *sparseFileWriter) Write(b []byte) (n int, err error) {
	overwrite := int64(len(b)) > sw.logicalRemaining()
	if overwrite {
		b = b[:sw.logicalRemaining()]
	}

	b0 := b
	endPos := sw.pos + int64(len(b))
	for endPos > sw.pos && err == nil {
		var nf int // Bytes written in fragment
		dataStart, dataEnd := sw.sp[0].Offset, sw.sp[0].endOffset()
		if sw.pos < dataStart { // In a hole fragment
			bf := b[:min(int64(len(b)), dataStart-sw.pos)]
			nf, err = zeroWriter{}.Write(bf)
		} else { // In a data fragment
			bf := b[:min(int64(len(b)), dataEnd-sw.pos)]
			nf, err = sw.fw.Write(bf)
		}
		b = b[nf:]
		sw.pos += int64(nf)
		if sw.pos >= dataEnd && len(sw.sp) > 1 {
			sw.sp = sw.sp[1:] // Ensure last fragment always remains
		}
	}

	n = len(b0) - len(b)
	switch {
	case err == ErrWriteTooLong:
		return n, errMissData // Not possible; implies bug in validation logic
	case err != nil:
		return n, err
	case sw.logicalRemaining() == 0 && sw.physicalRemaining() > 0:
		return n, errUnrefData // Not possible; implies bug in validation logic
	case overwrite:
		return n, ErrWriteTooLong
	default:
		return n, nil
	}
}

func (sw *sparseFileWriter) ReadFrom(r io.Reader) (n int64, err error) {
	rs, ok := r.(io.ReadSeeker)
	if ok {
		if _, err := rs.Seek(0, io.SeekCurrent); err != nil {
			ok = false // Not all io.Seeker can really seek
		}
	}
	if !ok {
		return io.Copy(struct{ io.Writer }{sw}, r)
	}

	var readLastByte bool
	pos0 := sw.pos
	for sw.logicalRemaining() > 0 && !readLastByte && err == nil {
		var nf int64 // Size of fragment
		dataStart, dataEnd := sw.sp[0].Offset, sw.sp[0].endOffset()
		if sw.pos < dataStart { // In a hole fragment
			nf = dataStart - sw.pos
			if sw.physicalRemaining() == 0 {
				readLastByte = true
				nf--
			}
			_, err = rs.Seek(nf, io.SeekCurrent)
		} else { // In a data fragment
			nf = dataEnd - sw.pos
			nf, err = io.CopyN(sw.fw, rs, nf)
		}
		sw.pos += nf
		if sw.pos >= dataEnd && len(sw.sp) > 1 {
			sw.sp = sw.sp[1:] // Ensure last fragment always remains
		}
	}

	// If the last fragment is a hole, then seek to 1-byte before EOF, and
	// read a single byte to ensure the file is the right size.
	if readLastByte && err == nil {
		_, err = mustReadFull(rs, []byte{0})
		sw.pos++
	}

	n = sw.pos - pos0
	switch {
	case err == io.EOF:
		return n, io.ErrUnexpectedEOF
	case err == ErrWriteTooLong:
		return n, errMissData // Not possible; implies bug in validation logic
	case err != nil:
		return n, err
	case sw.logicalRemaining() == 0 && sw.physicalRemaining() > 0:
		return n, errUnrefData // Not possible; implies bug in validation logic
	default:
		return n, ensureEOF(rs)
	}
}

func (sw sparseFileWriter) logicalRemaining() int64 {
	return sw.sp[len(sw.sp)-1].endOffset() - sw.pos
}

func (sw sparseFileWriter) physicalRemaining() int64 {
	return sw.fw.physicalRemaining()
}

// zeroWriter may only be written with NULs, otherwise it returns errWriteHole.
type zeroWriter struct{}

func (zeroWriter) Write(b []byte) (int, error) {
	for i, c := range b {
		if c != 0 {
			return i, errWriteHole
		}
	}
	return len(b), nil
}

// ensureEOF checks whether r is at EOF, reporting ErrWriteTooLong if not so.
func ensureEOF(r io.Reader) error {
	n, err := tryReadFull(r, []byte{0})
	switch {
	case n > 0:
		return ErrWriteTooLong
	case err == io.EOF:
		return nil
	default:
		return err
	}
}

```

// === FILE: references/go/src/archive/zip/reader.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zip

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"internal/godebug"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

var zipinsecurepath = godebug.New("zipinsecurepath")

var (
	ErrFormat       = errors.New("zip: not a valid zip file")
	ErrAlgorithm    = errors.New("zip: unsupported compression algorithm")
	ErrChecksum     = errors.New("zip: checksum error")
	ErrInsecurePath = errors.New("zip: insecure file path")
)

// A Reader serves content from a ZIP archive.
type Reader struct {
	r             io.ReaderAt
	File          []*File
	Comment       string
	decompressors map[uint16]Decompressor

	// Some JAR files are zip files with a prefix that is a bash script.
	// The baseOffset field is the start of the zip file proper.
	baseOffset int64

	// fileList is a list of files sorted by ename,
	// for use by the Open method.
	fileListOnce sync.Once
	fileList     []fileListEntry
}

// A ReadCloser is a [Reader] that must be closed when no longer needed.
type ReadCloser struct {
	f *os.File
	Reader
}

// A File is a single file in a ZIP archive.
// The file information is in the embedded [FileHeader].
// The file content can be accessed by calling [File.Open].
type File struct {
	FileHeader
	zip          *Reader
	zipr         io.ReaderAt
	headerOffset int64 // includes overall ZIP archive baseOffset
}

// OpenReader will open the Zip file specified by name and return a ReadCloser.
//
// If any file inside the archive uses a non-local name
// (as defined by [filepath.IsLocal]) or a name containing backslashes
// and the GODEBUG environment variable contains `zipinsecurepath=0`,
// OpenReader returns the reader with an ErrInsecurePath error.
// A future version of Go may introduce this behavior by default.
// Programs that want to accept non-local names can ignore
// the ErrInsecurePath error and use the returned reader.
func OpenReader(name string) (*ReadCloser, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	r := new(ReadCloser)
	if err = r.init(f, fi.Size()); err != nil && err != ErrInsecurePath {
		f.Close()
		return nil, err
	}
	r.f = f
	return r, err
}

// NewReader returns a new [Reader] reading from r, which is assumed to
// have the given size in bytes.
//
// If any file inside the archive uses a non-local name
// (as defined by [filepath.IsLocal]) or a name containing backslashes
// and the GODEBUG environment variable contains `zipinsecurepath=0`,
// NewReader returns the reader with an [ErrInsecurePath] error.
// A future version of Go may introduce this behavior by default.
// Programs that want to accept non-local names can ignore
// the [ErrInsecurePath] error and use the returned reader.
func NewReader(r io.ReaderAt, size int64) (*Reader, error) {
	if size < 0 {
		return nil, errors.New("zip: size cannot be negative")
	}
	zr := new(Reader)
	var err error
	if err = zr.init(r, size); err != nil && err != ErrInsecurePath {
		return nil, err
	}
	return zr, err
}

func (r *Reader) init(rdr io.ReaderAt, size int64) error {
	end, baseOffset, err := readDirectoryEnd(rdr, size)
	if err != nil {
		return err
	}
	r.r = rdr
	r.baseOffset = baseOffset
	// Since the number of directory records is not validated, it is not
	// safe to preallocate r.File without first checking that the specified
	// number of files is reasonable, since a malformed archive may
	// indicate it contains up to 1 << 128 - 1 files. Since each file has a
	// header which will be _at least_ 30 bytes we can safely preallocate
	// if (data size / 30) >= end.directoryRecords.
	if end.directorySize < uint64(size) && (uint64(size)-end.directorySize)/30 >= end.directoryRecords {
		r.File = make([]*File, 0, end.directoryRecords)
	}
	r.Comment = end.comment
	rs := io.NewSectionReader(rdr, 0, size)
	if _, err = rs.Seek(r.baseOffset+int64(end.directoryOffset), io.SeekStart); err != nil {
		return err
	}
	buf := bufio.NewReader(rs)

	// The count of files inside a zip is truncated to fit in a uint16.
	// Gloss over this by reading headers until we encounter
	// a bad one, and then only report an ErrFormat or UnexpectedEOF if
	// the file count modulo 65536 is incorrect.
	for {
		f := &File{zip: r, zipr: rdr}
		err = readDirectoryHeader(f, buf)
		if err == ErrFormat || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return err
		}
		f.headerOffset += r.baseOffset
		r.File = append(r.File, f)
	}
	if uint16(len(r.File)) != uint16(end.directoryRecords) { // only compare 16 bits here
		// Return the readDirectoryHeader error if we read
		// the wrong number of directory entries.
		return err
	}
	if zipinsecurepath.Value() == "0" {
		for _, f := range r.File {
			if f.Name == "" {
				// Zip permits an empty file name field.
				continue
			}
			// The zip specification states that names must use forward slashes,
			// so consider any backslashes in the name insecure.
			if !filepath.IsLocal(f.Name) || strings.Contains(f.Name, `\`) {
				zipinsecurepath.IncNonDefault()
				return ErrInsecurePath
			}
		}
	}
	return nil
}

// RegisterDecompressor registers or overrides a custom decompressor for a
// specific method ID. If a decompressor for a given method is not found,
// [Reader] will default to looking up the decompressor at the package level.
func (r *Reader) RegisterDecompressor(method uint16, dcomp Decompressor) {
	if r.decompressors == nil {
		r.decompressors = make(map[uint16]Decompressor)
	}
	r.decompressors[method] = dcomp
}

func (r *Reader) decompressor(method uint16) Decompressor {
	dcomp := r.decompressors[method]
	if dcomp == nil {
		dcomp = decompressor(method)
	}
	return dcomp
}

// Close closes the Zip file, rendering it unusable for I/O.
func (rc *ReadCloser) Close() error {
	return rc.f.Close()
}

// DataOffset returns the offset of the file's possibly-compressed
// data, relative to the beginning of the zip file.
//
// Most callers should instead use [File.Open], which transparently
// decompresses data and verifies checksums.
func (f *File) DataOffset() (offset int64, err error) {
	bodyOffset, err := f.findBodyOffset()
	if err != nil {
		return
	}
	return f.headerOffset + bodyOffset, nil
}

// Open returns a [ReadCloser] that provides access to the [File]'s contents.
// Multiple files may be read concurrently.
func (f *File) Open() (io.ReadCloser, error) {
	bodyOffset, err := f.findBodyOffset()
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(f.Name, "/") {
		// The ZIP specification (APPNOTE.TXT) specifies that directories, which
		// are technically zero-byte files, must not have any associated file
		// data. We previously tried failing here if f.CompressedSize64 != 0,
		// but it turns out that a number of implementations (namely, the Java
		// jar tool) don't properly set the storage method on directories
		// resulting in a file with compressed size > 0 but uncompressed size ==
		// 0. We still want to fail when a directory has associated uncompressed
		// data, but we are tolerant of cases where the uncompressed size is
		// zero but compressed size is not.
		if f.UncompressedSize64 != 0 {
			return &dirReader{ErrFormat}, nil
		} else {
			return &dirReader{io.EOF}, nil
		}
	}
	size := int64(f.CompressedSize64)
	r := io.NewSectionReader(f.zipr, f.headerOffset+bodyOffset, size)
	dcomp := f.zip.decompressor(f.Method)
	if dcomp == nil {
		return nil, ErrAlgorithm
	}
	var rc io.ReadCloser = dcomp(r)
	var desr io.Reader
	if f.hasDataDescriptor() {
		desr = io.NewSectionReader(f.zipr, f.headerOffset+bodyOffset+size, dataDescriptorLen)
	}
	rc = &checksumReader{
		rc:   rc,
		hash: crc32.NewIEEE(),
		f:    f,
		desr: desr,
	}
	return rc, nil
}

// OpenRaw returns a [Reader] that provides access to the [File]'s contents without
// decompression.
func (f *File) OpenRaw() (io.Reader, error) {
	bodyOffset, err := f.findBodyOffset()
	if err != nil {
		return nil, err
	}
	r := io.NewSectionReader(f.zipr, f.headerOffset+bodyOffset, int64(f.CompressedSize64))
	return r, nil
}

type dirReader struct {
	err error
}

func (r *dirReader) Read([]byte) (int, error) {
	return 0, r.err
}

func (r *dirReader) Close() error {
	return nil
}

type checksumReader struct {
	rc    io.ReadCloser
	hash  hash.Hash32
	nread uint64 // number of bytes read so far
	f     *File
	desr  io.Reader // if non-nil, where to read the data descriptor
	err   error     // sticky error
}

func (r *checksumReader) Stat() (fs.FileInfo, error) {
	return headerFileInfo{&r.f.FileHeader}, nil
}

func (r *checksumReader) Read(b []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err = r.rc.Read(b)
	r.hash.Write(b[:n])
	r.nread += uint64(n)
	if r.nread > r.f.UncompressedSize64 {
		return 0, ErrFormat
	}
	if err == nil {
		return
	}
	if err == io.EOF {
		if r.nread != r.f.UncompressedSize64 {
			return 0, io.ErrUnexpectedEOF
		}
		if r.desr != nil {
			if err1 := readDataDescriptor(r.desr, r.f); err1 != nil {
				if err1 == io.EOF {
					err = io.ErrUnexpectedEOF
				} else {
					err = err1
				}
			} else if r.hash.Sum32() != r.f.CRC32 {
				err = ErrChecksum
			}
		} else {
			// If there's not a data descriptor, we still compare
			// the CRC32 of what we've read against the file header
			// or TOC's CRC32, if it seems like it was set.
			if r.f.CRC32 != 0 && r.hash.Sum32() != r.f.CRC32 {
				err = ErrChecksum
			}
		}
	}
	r.err = err
	return
}

func (r *checksumReader) Close() error { return r.rc.Close() }

// findBodyOffset does the minimum work to verify the file has a header
// and returns the file body offset.
func (f *File) findBodyOffset() (int64, error) {
	var buf [fileHeaderLen]byte
	if _, err := f.zipr.ReadAt(buf[:], f.headerOffset); err != nil {
		return 0, err
	}
	b := readBuf(buf[:])
	if sig := b.uint32(); sig != fileHeaderSignature {
		return 0, ErrFormat
	}
	b = b[22:] // skip over most of the header
	filenameLen := int(b.uint16())
	extraLen := int(b.uint16())
	return int64(fileHeaderLen + filenameLen + extraLen), nil
}

// readDirectoryHeader attempts to read a directory header from r.
// It returns io.ErrUnexpectedEOF if it cannot read a complete header,
// and ErrFormat if it doesn't find a valid header signature.
func readDirectoryHeader(f *File, r io.Reader) error {
	var buf [directoryHeaderLen]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return err
	}
	b := readBuf(buf[:])
	if sig := b.uint32(); sig != directoryHeaderSignature {
		return ErrFormat
	}
	f.CreatorVersion = b.uint16()
	f.ReaderVersion = b.uint16()
	f.Flags = b.uint16()
	f.Method = b.uint16()
	f.ModifiedTime = b.uint16()
	f.ModifiedDate = b.uint16()
	f.CRC32 = b.uint32()
	f.CompressedSize = b.uint32()
	f.UncompressedSize = b.uint32()
	f.CompressedSize64 = uint64(f.CompressedSize)
	f.UncompressedSize64 = uint64(f.UncompressedSize)
	filenameLen := int(b.uint16())
	extraLen := int(b.uint16())
	commentLen := int(b.uint16())
	b = b[4:] // skipped start disk number and internal attributes (2x uint16)
	f.ExternalAttrs = b.uint32()
	f.headerOffset = int64(b.uint32())
	d := make([]byte, filenameLen+extraLen+commentLen)
	if _, err := io.ReadFull(r, d); err != nil {
		return err
	}
	f.Name = string(d[:filenameLen])
	f.Extra = d[filenameLen : filenameLen+extraLen]
	f.Comment = string(d[filenameLen+extraLen:])

	// Determine the character encoding.
	utf8Valid1, utf8Require1 := detectUTF8(f.Name)
	utf8Valid2, utf8Require2 := detectUTF8(f.Comment)
	switch {
	case !utf8Valid1 || !utf8Valid2:
		// Name and Comment definitely not UTF-8.
		f.NonUTF8 = true
	case !utf8Require1 && !utf8Require2:
		// Name and Comment use only single-byte runes that overlap with UTF-8.
		f.NonUTF8 = false
	default:
		// Might be UTF-8, might be some other encoding; preserve existing flag.
		// Some ZIP writers use UTF-8 encoding without setting the UTF-8 flag.
		// Since it is impossible to always distinguish valid UTF-8 from some
		// other encoding (e.g., GBK or Shift-JIS), we trust the flag.
		f.NonUTF8 = f.Flags&0x800 == 0
	}

	// Best effort to find what we need.
	// Other zip authors might not even follow the basic format,
	// and we'll just ignore the Extra content in that case.
	var modified time.Time
parseExtras:
	for extra := readBuf(f.Extra); len(extra) >= 4; { // need at least tag and size
		fieldTag := extra.uint16()
		fieldSize := int(extra.uint16())
		if len(extra) < fieldSize {
			break
		}
		fieldBuf := extra.sub(fieldSize)

		switch fieldTag {
		case zip64ExtraID:
			// update directory values from the zip64 extra block.
			// They should only be consulted if the sizes read earlier
			// are maxed out.
			// See go.dev/issue/13367 and go.dev/issue/31692.
			if f.UncompressedSize == ^uint32(0) {
				if len(fieldBuf) < 8 {
					return ErrFormat
				}
				f.UncompressedSize64 = fieldBuf.uint64()
			}
			if f.CompressedSize == ^uint32(0) {
				if len(fieldBuf) < 8 {
					return ErrFormat
				}
				f.CompressedSize64 = fieldBuf.uint64()
			}
			if f.headerOffset == int64(^uint32(0)) {
				if len(fieldBuf) < 8 {
					return ErrFormat
				}
				f.headerOffset = int64(fieldBuf.uint64())
			}
		case ntfsExtraID:
			if len(fieldBuf) < 4 {
				continue parseExtras
			}
			fieldBuf.uint32()        // reserved (ignored)
			for len(fieldBuf) >= 4 { // need at least tag and size
				attrTag := fieldBuf.uint16()
				attrSize := int(fieldBuf.uint16())
				if len(fieldBuf) < attrSize {
					continue parseExtras
				}
				attrBuf := fieldBuf.sub(attrSize)
				if attrTag != 1 || attrSize != 24 {
					continue // Ignore irrelevant attributes
				}

				const ticksPerSecond = 1e7    // Windows timestamp resolution
				ts := int64(attrBuf.uint64()) // ModTime since Windows epoch
				secs := ts / ticksPerSecond
				nsecs := (1e9 / ticksPerSecond) * (ts % ticksPerSecond)
				epoch := time.Date(1601, time.January, 1, 0, 0, 0, 0, time.UTC)
				modified = time.Unix(epoch.Unix()+secs, nsecs)
			}
		case unixExtraID, infoZipUnixExtraID:
			if len(fieldBuf) < 8 {
				continue parseExtras
			}
			fieldBuf.uint32()              // AcTime (ignored)
			ts := int64(fieldBuf.uint32()) // ModTime since Unix epoch
			modified = time.Unix(ts, 0)
		case extTimeExtraID:
			if len(fieldBuf) < 5 || fieldBuf.uint8()&1 == 0 {
				continue parseExtras
			}
			ts := int64(fieldBuf.uint32()) // ModTime since Unix epoch
			modified = time.Unix(ts, 0)
		}
	}

	msdosModified := msDosTimeToTime(f.ModifiedDate, f.ModifiedTime)
	f.Modified = msdosModified
	if !modified.IsZero() {
		f.Modified = modified.UTC()

		// If legacy MS-DOS timestamps are set, we can use the delta between
		// the legacy and extended versions to estimate timezone offset.
		//
		// A non-UTC timezone is always used (even if offset is zero).
		// Thus, FileHeader.Modified.Location() == time.UTC is useful for
		// determining whether extended timestamps are present.
		// This is necessary for users that need to do additional time
		// calculations when dealing with legacy ZIP formats.
		if f.ModifiedTime != 0 || f.ModifiedDate != 0 {
			f.Modified = modified.In(timeZone(msdosModified.Sub(modified)))
		}
	}

	return nil
}

func readDataDescriptor(r io.Reader, f *File) error {
	var buf [dataDescriptorLen]byte
	// The spec says: "Although not originally assigned a
	// signature, the value 0x08074b50 has commonly been adopted
	// as a signature value for the data descriptor record.
	// Implementers should be aware that ZIP files may be
	// encountered with or without this signature marking data
	// descriptors and should account for either case when reading
	// ZIP files to ensure compatibility."
	//
	// dataDescriptorLen includes the size of the signature but
	// first read just those 4 bytes to see if it exists.
	if _, err := io.ReadFull(r, buf[:4]); err != nil {
		return err
	}
	off := 0
	maybeSig := readBuf(buf[:4])
	if maybeSig.uint32() != dataDescriptorSignature {
		// No data descriptor signature. Keep these four
		// bytes.
		off += 4
	}
	if _, err := io.ReadFull(r, buf[off:12]); err != nil {
		return err
	}
	b := readBuf(buf[:12])
	if b.uint32() != f.CRC32 {
		return ErrChecksum
	}

	// The two sizes that follow here can be either 32 bits or 64 bits
	// but the spec is not very clear on this and different
	// interpretations has been made causing incompatibilities. We
	// already have the sizes from the central directory so we can
	// just ignore these.

	return nil
}

func readDirectoryEnd(r io.ReaderAt, size int64) (dir *directoryEnd, baseOffset int64, err error) {
	// look for directoryEndSignature in the last 1k, then in the last 65k
	var buf []byte
	var directoryEndOffset int64
	for i, bLen := range []int64{1024, 65 * 1024} {
		if bLen > size {
			bLen = size
		}
		buf = make([]byte, int(bLen))
		if _, err := r.ReadAt(buf, size-bLen); err != nil && err != io.EOF {
			return nil, 0, err
		}
		if p := findSignatureInBlock(buf); p >= 0 {
			buf = buf[p:]
			directoryEndOffset = size - bLen + int64(p)
			break
		}
		if i == 1 || bLen == size {
			return nil, 0, ErrFormat
		}
	}

	// read header into struct
	b := readBuf(buf[4:]) // skip signature
	d := &directoryEnd{
		diskNbr:            uint32(b.uint16()),
		dirDiskNbr:         uint32(b.uint16()),
		dirRecordsThisDisk: uint64(b.uint16()),
		directoryRecords:   uint64(b.uint16()),
		directorySize:      uint64(b.uint32()),
		directoryOffset:    uint64(b.uint32()),
		commentLen:         b.uint16(),
	}
	l := int(d.commentLen)
	if l > len(b) {
		return nil, 0, errors.New("zip: invalid comment length")
	}
	d.comment = string(b[:l])

	// These values mean that the file can be a zip64 file
	if d.directoryRecords == 0xffff || d.directorySize == 0xffffffff || d.directoryOffset == 0xffffffff {
		p, err := findDirectory64End(r, directoryEndOffset)
		if err == nil && p >= 0 {
			directoryEndOffset = p
			err = readDirectory64End(r, p, d)
		}
		if err != nil {
			return nil, 0, err
		}
	}

	maxInt64 := uint64(1<<63 - 1)
	if d.directorySize > maxInt64 || d.directoryOffset > maxInt64 {
		return nil, 0, ErrFormat
	}

	baseOffset = directoryEndOffset - int64(d.directorySize) - int64(d.directoryOffset)

	// Make sure directoryOffset points to somewhere in our file.
	if o := baseOffset + int64(d.directoryOffset); o < 0 || o >= size {
		return nil, 0, ErrFormat
	}

	// If the directory end data tells us to use a non-zero baseOffset,
	// but we would find a valid directory entry if we assume that the
	// baseOffset is 0, then just use a baseOffset of 0.
	// We've seen files in which the directory end data gives us
	// an incorrect baseOffset.
	if baseOffset > 0 {
		off := int64(d.directoryOffset)
		rs := io.NewSectionReader(r, off, size-off)
		if readDirectoryHeader(&File{}, rs) == nil {
			baseOffset = 0
		}
	}

	return d, baseOffset, nil
}

// findDirectory64End tries to read the zip64 locator just before the
// directory end and returns the offset of the zip64 directory end if
// found.
func findDirectory64End(r io.ReaderAt, directoryEndOffset int64) (int64, error) {
	locOffset := directoryEndOffset - directory64LocLen
	if locOffset < 0 {
		return -1, nil // no need to look for a header outside the file
	}
	buf := make([]byte, directory64LocLen)
	if _, err := r.ReadAt(buf, locOffset); err != nil {
		return -1, err
	}
	b := readBuf(buf)
	if sig := b.uint32(); sig != directory64LocSignature {
		return -1, nil
	}
	if b.uint32() != 0 { // number of the disk with the start of the zip64 end of central directory
		return -1, nil // the file is not a valid zip64-file
	}
	p := b.uint64()      // relative offset of the zip64 end of central directory record
	if b.uint32() != 1 { // total number of disks
		return -1, nil // the file is not a valid zip64-file
	}
	return int64(p), nil
}

// readDirectory64End reads the zip64 directory end and updates the
// directory end with the zip64 directory end values.
func readDirectory64End(r io.ReaderAt, offset int64, d *directoryEnd) (err error) {
	buf := make([]byte, directory64EndLen)
	if _, err := r.ReadAt(buf, offset); err != nil {
		return err
	}

	b := readBuf(buf)
	if sig := b.uint32(); sig != directory64EndSignature {
		return ErrFormat
	}

	b = b[12:]                        // skip dir size, version and version needed (uint64 + 2x uint16)
	d.diskNbr = b.uint32()            // number of this disk
	d.dirDiskNbr = b.uint32()         // number of the disk with the start of the central directory
	d.dirRecordsThisDisk = b.uint64() // total number of entries in the central directory on this disk
	d.directoryRecords = b.uint64()   // total number of entries in the central directory
	d.directorySize = b.uint64()      // size of the central directory
	d.directoryOffset = b.uint64()    // offset of start of central directory with respect to the starting disk number

	return nil
}

func findSignatureInBlock(b []byte) int {
	for i := len(b) - directoryEndLen; i >= 0; i-- {
		// defined from directoryEndSignature in struct.go
		if b[i] == 'P' && b[i+1] == 'K' && b[i+2] == 0x05 && b[i+3] == 0x06 {
			// n is length of comment
			n := int(b[i+directoryEndLen-2]) | int(b[i+directoryEndLen-1])<<8
			if n+directoryEndLen+i > len(b) {
				// Truncated comment.
				// Some parsers (such as Info-ZIP) ignore the truncated comment
				// rather than treating it as a hard error.
				return -1
			}
			return i
		}
	}
	return -1
}

type readBuf []byte

func (b *readBuf) uint8() uint8 {
	v := (*b)[0]
	*b = (*b)[1:]
	return v
}

func (b *readBuf) uint16() uint16 {
	v := binary.LittleEndian.Uint16(*b)
	*b = (*b)[2:]
	return v
}

func (b *readBuf) uint32() uint32 {
	v := binary.LittleEndian.Uint32(*b)
	*b = (*b)[4:]
	return v
}

func (b *readBuf) uint64() uint64 {
	v := binary.LittleEndian.Uint64(*b)
	*b = (*b)[8:]
	return v
}

func (b *readBuf) sub(n int) readBuf {
	b2 := (*b)[:n]
	*b = (*b)[n:]
	return b2
}

// A fileListEntry is a File and its ename.
// If file == nil, the fileListEntry describes a directory without metadata.
type fileListEntry struct {
	name  string
	file  *File
	isDir bool
	isDup bool
}

type fileInfoDirEntry interface {
	fs.FileInfo
	fs.DirEntry
}

func (f *fileListEntry) stat() (fileInfoDirEntry, error) {
	if f.isDup {
		return nil, errors.New(f.name + ": duplicate entries in zip file")
	}
	if !f.isDir {
		return headerFileInfo{&f.file.FileHeader}, nil
	}
	return f, nil
}

// Only used for directories.
func (f *fileListEntry) Name() string      { _, elem, _ := split(f.name); return elem }
func (f *fileListEntry) Size() int64       { return 0 }
func (f *fileListEntry) Mode() fs.FileMode { return fs.ModeDir | 0555 }
func (f *fileListEntry) Type() fs.FileMode { return fs.ModeDir }
func (f *fileListEntry) IsDir() bool       { return true }
func (f *fileListEntry) Sys() any          { return nil }

func (f *fileListEntry) ModTime() time.Time {
	if f.file == nil {
		return time.Time{}
	}
	return f.file.FileHeader.Modified.UTC()
}

func (f *fileListEntry) Info() (fs.FileInfo, error) { return f, nil }

func (f *fileListEntry) String() string {
	return fs.FormatDirEntry(f)
}

// toValidName coerces name to be a valid name for fs.FS.Open.
func toValidName(name string) string {
	name = strings.ReplaceAll(name, `\`, `/`)
	p := path.Clean(name)

	p = strings.TrimPrefix(p, "/")

	for strings.HasPrefix(p, "../") {
		p = p[len("../"):]
	}

	return p
}

func (r *Reader) initFileList() {
	r.fileListOnce.Do(func() {
		// Preallocate the minimum size of the index.
		// We may also synthesize additional directory entries.
		r.fileList = make([]fileListEntry, 0, len(r.File))
		// files and knownDirs map from a file/directory name
		// to an index into the r.fileList entry that we are
		// building. They are used to mark duplicate entries.
		files := make(map[string]int)
		knownDirs := make(map[string]int)

		// dirs[name] is true if name is known to be a directory,
		// because it appears as a prefix in a path.
		dirs := make(map[string]bool)

		for _, file := range r.File {
			isDir := len(file.Name) > 0 && file.Name[len(file.Name)-1] == '/'
			name := toValidName(file.Name)
			if name == "" {
				continue
			}

			if idx, ok := files[name]; ok {
				r.fileList[idx].isDup = true
				continue
			}
			if idx, ok := knownDirs[name]; ok {
				r.fileList[idx].isDup = true
				continue
			}

			dir := name
			for {
				if idx := strings.LastIndex(dir, "/"); idx < 0 {
					break
				} else {
					dir = dir[:idx]
				}
				if dirs[dir] {
					break
				}
				dirs[dir] = true
			}

			idx := len(r.fileList)
			entry := fileListEntry{
				name:  name,
				file:  file,
				isDir: isDir,
			}
			r.fileList = append(r.fileList, entry)
			if isDir {
				knownDirs[name] = idx
			} else {
				files[name] = idx
			}
		}
		for dir := range dirs {
			if _, ok := knownDirs[dir]; !ok {
				if idx, ok := files[dir]; ok {
					r.fileList[idx].isDup = true
				} else {
					entry := fileListEntry{
						name:  dir,
						file:  nil,
						isDir: true,
					}
					r.fileList = append(r.fileList, entry)
				}
			}
		}

		slices.SortFunc(r.fileList, func(a, b fileListEntry) int {
			return fileEntryCompare(a.name, b.name)
		})
	})
}

func fileEntryCompare(x, y string) int {
	xdir, xelem, _ := split(x)
	ydir, yelem, _ := split(y)
	if xdir != ydir {
		return strings.Compare(xdir, ydir)
	}
	return strings.Compare(xelem, yelem)
}

// Open opens the named file in the ZIP archive,
// using the semantics of fs.FS.Open:
// paths are always slash separated, with no
// leading / or ../ elements.
func (r *Reader) Open(name string) (fs.File, error) {
	r.initFileList()

	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	e := r.openLookup(name)
	if e == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if e.isDir {
		return &openDir{e, r.openReadDir(name), 0}, nil
	}
	rc, err := e.file.Open()
	if err != nil {
		return nil, err
	}
	return rc.(fs.File), nil
}

func split(name string) (dir, elem string, isDir bool) {
	name, isDir = strings.CutSuffix(name, "/")
	i := strings.LastIndexByte(name, '/')
	if i < 0 {
		return ".", name, isDir
	}
	return name[:i], name[i+1:], isDir
}

var dotFile = &fileListEntry{name: "./", isDir: true}

func (r *Reader) openLookup(name string) *fileListEntry {
	if name == "." {
		return dotFile
	}

	dir, elem, _ := split(name)
	files := r.fileList
	i, _ := slices.BinarySearchFunc(files, dir, func(a fileListEntry, dir string) (ret int) {
		idir, ielem, _ := split(a.name)
		if dir != idir {
			return strings.Compare(idir, dir)
		}
		return strings.Compare(ielem, elem)
	})
	if i < len(files) {
		fname := files[i].name
		if fname == name || len(fname) == len(name)+1 && fname[len(name)] == '/' && fname[:len(name)] == name {
			return &files[i]
		}
	}
	return nil
}

func (r *Reader) openReadDir(dir string) []fileListEntry {
	files := r.fileList
	i, _ := slices.BinarySearchFunc(files, dir, func(a fileListEntry, dir string) int {
		idir, _, _ := split(a.name)
		if dir != idir {
			return strings.Compare(idir, dir)
		}
		// find the first entry with dir
		return +1
	})
	j, _ := slices.BinarySearchFunc(files, dir, func(a fileListEntry, dir string) int {
		jdir, _, _ := split(a.name)
		if dir != jdir {
			return strings.Compare(jdir, dir)
		}
		// find the last entry with dir
		return -1
	})
	return files[i:j]
}

type openDir struct {
	e      *fileListEntry
	files  []fileListEntry
	offset int
}

func (d *openDir) Close() error               { return nil }
func (d *openDir) Stat() (fs.FileInfo, error) { return d.e.stat() }

func (d *openDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.e.name, Err: errors.New("is a directory")}
}

func (d *openDir) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.files) - d.offset
	if count > 0 && n > count {
		n = count
	}
	if n == 0 {
		if count <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		s, err := d.files[d.offset+i].stat()
		if err != nil {
			return nil, err
		} else if s.Name() == "." || !fs.ValidPath(s.Name()) {
			return nil, &fs.PathError{
				Op:   "readdir",
				Path: d.e.name,
				Err:  fmt.Errorf("invalid file name: %v", d.files[d.offset+i].name),
			}
		}
		list[i] = s
	}
	d.offset += n
	return list, nil
}

```

// === FILE: references/go/src/archive/zip/register.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zip

import (
	"compress/flate"
	"errors"
	"io"
	"sync"
)

// A Compressor returns a new compressing writer, writing to w.
// The WriteCloser's Close method must be used to flush pending data to w.
// The Compressor itself must be safe to invoke from multiple goroutines
// simultaneously, but each returned writer will be used only by
// one goroutine at a time.
type Compressor func(w io.Writer) (io.WriteCloser, error)

// A Decompressor returns a new decompressing reader, reading from r.
// The [io.ReadCloser]'s Close method must be used to release associated resources.
// The Decompressor itself must be safe to invoke from multiple goroutines
// simultaneously, but each returned reader will be used only by
// one goroutine at a time.
type Decompressor func(r io.Reader) io.ReadCloser

var flateWriterPool sync.Pool

func newFlateWriter(w io.Writer) io.WriteCloser {
	fw, ok := flateWriterPool.Get().(*flate.Writer)
	if ok {
		fw.Reset(w)
	} else {
		fw, _ = flate.NewWriter(w, 5)
	}
	return &pooledFlateWriter{fw: fw}
}

type pooledFlateWriter struct {
	mu sync.Mutex // guards Close and Write
	fw *flate.Writer
}

func (w *pooledFlateWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fw == nil {
		return 0, errors.New("Write after Close")
	}
	return w.fw.Write(p)
}

func (w *pooledFlateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var err error
	if w.fw != nil {
		err = w.fw.Close()
		flateWriterPool.Put(w.fw)
		w.fw = nil
	}
	return err
}

var flateReaderPool sync.Pool

func newFlateReader(r io.Reader) io.ReadCloser {
	fr, ok := flateReaderPool.Get().(io.ReadCloser)
	if ok {
		fr.(flate.Resetter).Reset(r, nil)
	} else {
		fr = flate.NewReader(r)
	}
	return &pooledFlateReader{fr: fr}
}

type pooledFlateReader struct {
	mu sync.Mutex // guards Close and Read
	fr io.ReadCloser
}

func (r *pooledFlateReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fr == nil {
		return 0, errors.New("Read after Close")
	}
	return r.fr.Read(p)
}

func (r *pooledFlateReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var err error
	if r.fr != nil {
		err = r.fr.Close()
		flateReaderPool.Put(r.fr)
		r.fr = nil
	}
	return err
}

var (
	compressors   sync.Map // map[uint16]Compressor
	decompressors sync.Map // map[uint16]Decompressor
)

func init() {
	compressors.Store(Store, Compressor(func(w io.Writer) (io.WriteCloser, error) { return &nopCloser{w}, nil }))
	compressors.Store(Deflate, Compressor(func(w io.Writer) (io.WriteCloser, error) { return newFlateWriter(w), nil }))

	decompressors.Store(Store, Decompressor(io.NopCloser))
	decompressors.Store(Deflate, Decompressor(newFlateReader))
}

// RegisterDecompressor allows custom decompressors for a specified method ID.
// The common methods [Store] and [Deflate] are built in.
func RegisterDecompressor(method uint16, dcomp Decompressor) {
	if _, dup := decompressors.LoadOrStore(method, dcomp); dup {
		panic("decompressor already registered")
	}
}

// RegisterCompressor registers custom compressors for a specified method ID.
// The common methods [Store] and [Deflate] are built in.
func RegisterCompressor(method uint16, comp Compressor) {
	if _, dup := compressors.LoadOrStore(method, comp); dup {
		panic("compressor already registered")
	}
}

func compressor(method uint16) Compressor {
	ci, ok := compressors.Load(method)
	if !ok {
		return nil
	}
	return ci.(Compressor)
}

func decompressor(method uint16) Decompressor {
	di, ok := decompressors.Load(method)
	if !ok {
		return nil
	}
	return di.(Decompressor)
}

```

// === FILE: references/go/src/archive/zip/struct.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package zip provides support for reading and writing ZIP archives.

See the [ZIP specification] for details.

This package does not support disk spanning.

A note about ZIP64:

To be backwards compatible the FileHeader has both 32 and 64 bit Size
fields. The 64 bit fields will always contain the correct value and
for normal archives both fields will be the same. For files requiring
the ZIP64 format the 32 bit fields will be 0xffffffff and the 64 bit
fields must be used instead.

[ZIP specification]: https://support.pkware.com/pkzip/appnote
*/
package zip

import (
	"io/fs"
	"path"
	"time"
)

// Compression methods.
const (
	Store   uint16 = 0 // no compression
	Deflate uint16 = 8 // DEFLATE compressed
)

const (
	fileHeaderSignature      = 0x04034b50
	directoryHeaderSignature = 0x02014b50
	directoryEndSignature    = 0x06054b50
	directory64LocSignature  = 0x07064b50
	directory64EndSignature  = 0x06064b50
	dataDescriptorSignature  = 0x08074b50 // de-facto standard; required by OS X Finder
	fileHeaderLen            = 30         // + filename + extra
	directoryHeaderLen       = 46         // + filename + extra + comment
	directoryEndLen          = 22         // + comment
	dataDescriptorLen        = 16         // four uint32: descriptor signature, crc32, compressed size, size
	dataDescriptor64Len      = 24         // two uint32: signature, crc32 | two uint64: compressed size, size
	directory64LocLen        = 20         //
	directory64EndLen        = 56         // + extra

	// Constants for the first byte in CreatorVersion.
	creatorFAT    = 0
	creatorUnix   = 3
	creatorNTFS   = 11
	creatorVFAT   = 14
	creatorMacOSX = 19

	// Version numbers.
	zipVersion20 = 20 // 2.0
	zipVersion45 = 45 // 4.5 (reads and writes zip64 archives)

	// Limits for non zip64 files.
	uint16max = (1 << 16) - 1
	uint32max = (1 << 32) - 1

	// Extra header IDs.
	//
	// IDs 0..31 are reserved for official use by PKWARE.
	// IDs above that range are defined by third-party vendors.
	// Since ZIP lacked high precision timestamps (nor an official specification
	// of the timezone used for the date fields), many competing extra fields
	// have been invented. Pervasive use effectively makes them "official".
	//
	// See http://mdfs.net/Docs/Comp/Archiving/Zip/ExtraField
	zip64ExtraID       = 0x0001 // Zip64 extended information
	ntfsExtraID        = 0x000a // NTFS
	unixExtraID        = 0x000d // UNIX
	extTimeExtraID     = 0x5455 // Extended timestamp
	infoZipUnixExtraID = 0x5855 // Info-ZIP Unix extension
)

// FileHeader describes a file within a ZIP file.
// See the [ZIP specification] for details.
//
// [ZIP specification]: https://support.pkware.com/pkzip/appnote
type FileHeader struct {
	// Name is the name of the file.
	//
	// It must be a relative path, not start with a drive letter (such as "C:"),
	// and must use forward slashes instead of back slashes. A trailing slash
	// indicates that this file is a directory and should have no data.
	Name string

	// Comment is any arbitrary user-defined string shorter than 64KiB.
	Comment string

	// NonUTF8 indicates that Name and Comment are not encoded in UTF-8.
	//
	// By specification, the only other encoding permitted should be CP-437,
	// but historically many ZIP readers interpret Name and Comment as whatever
	// the system's local character encoding happens to be.
	//
	// This flag should only be set if the user intends to encode a non-portable
	// ZIP file for a specific localized region. Otherwise, the Writer
	// automatically sets the ZIP format's UTF-8 flag for valid UTF-8 strings.
	NonUTF8 bool

	CreatorVersion uint16
	ReaderVersion  uint16
	Flags          uint16

	// Method is the compression method. If zero, Store is used.
	Method uint16

	// Modified is the modified time of the file.
	//
	// When reading, an extended timestamp is preferred over the legacy MS-DOS
	// date field, and the offset between the times is used as the timezone.
	// If only the MS-DOS date is present, the timezone is assumed to be UTC.
	//
	// When writing, an extended timestamp (which is timezone-agnostic) is
	// always emitted. The legacy MS-DOS date field is encoded according to the
	// location of the Modified time.
	Modified time.Time

	// ModifiedTime is an MS-DOS-encoded time.
	//
	// Deprecated: Use Modified instead.
	ModifiedTime uint16

	// ModifiedDate is an MS-DOS-encoded date.
	//
	// Deprecated: Use Modified instead.
	ModifiedDate uint16

	// CRC32 is the CRC32 checksum of the file content.
	CRC32 uint32

	// CompressedSize is the compressed size of the file in bytes.
	// If either the uncompressed or compressed size of the file
	// does not fit in 32 bits, CompressedSize is set to ^uint32(0).
	//
	// Deprecated: Use CompressedSize64 instead.
	CompressedSize uint32

	// UncompressedSize is the uncompressed size of the file in bytes.
	// If either the uncompressed or compressed size of the file
	// does not fit in 32 bits, UncompressedSize is set to ^uint32(0).
	//
	// Deprecated: Use UncompressedSize64 instead.
	UncompressedSize uint32

	// CompressedSize64 is the compressed size of the file in bytes.
	CompressedSize64 uint64

	// UncompressedSize64 is the uncompressed size of the file in bytes.
	UncompressedSize64 uint64

	// Extra are the extensible data fields. The writer automatically includes
	// the appropriate Zip64 field if necessary, and [Writer.Close] appends the
	// Central Directory version of the Zip64 field to Extra.
	Extra []byte

	ExternalAttrs uint32 // Meaning depends on CreatorVersion
}

// FileInfo returns an fs.FileInfo for the [FileHeader].
func (h *FileHeader) FileInfo() fs.FileInfo {
	return headerFileInfo{h}
}

// headerFileInfo implements [fs.FileInfo].
type headerFileInfo struct {
	fh *FileHeader
}

func (fi headerFileInfo) Name() string { return path.Base(fi.fh.Name) }
func (fi headerFileInfo) Size() int64 {
	if fi.fh.UncompressedSize64 > 0 {
		return int64(fi.fh.UncompressedSize64)
	}
	return int64(fi.fh.UncompressedSize)
}
func (fi headerFileInfo) IsDir() bool { return fi.Mode().IsDir() }
func (fi headerFileInfo) ModTime() time.Time {
	if fi.fh.Modified.IsZero() {
		return fi.fh.ModTime()
	}
	return fi.fh.Modified.UTC()
}
func (fi headerFileInfo) Mode() fs.FileMode { return fi.fh.Mode() }
func (fi headerFileInfo) Type() fs.FileMode { return fi.fh.Mode().Type() }
func (fi headerFileInfo) Sys() any          { return fi.fh }

func (fi headerFileInfo) Info() (fs.FileInfo, error) { return fi, nil }

func (fi headerFileInfo) String() string {
	return fs.FormatFileInfo(fi)
}

// FileInfoHeader creates a partially-populated [FileHeader] from an
// fs.FileInfo.
// Because fs.FileInfo's Name method returns only the base name of
// the file it describes, it may be necessary to modify the Name field
// of the returned header to provide the full path name of the file.
// If compression is desired, callers should set the FileHeader.Method
// field; it is unset by default.
func FileInfoHeader(fi fs.FileInfo) (*FileHeader, error) {
	size := fi.Size()
	fh := &FileHeader{
		Name:               fi.Name(),
		UncompressedSize64: uint64(size),
	}
	fh.SetModTime(fi.ModTime())
	fh.SetMode(fi.Mode())
	if fh.UncompressedSize64 > uint32max {
		fh.UncompressedSize = uint32max
	} else {
		fh.UncompressedSize = uint32(fh.UncompressedSize64)
	}
	return fh, nil
}

type directoryEnd struct {
	diskNbr            uint32 // unused
	dirDiskNbr         uint32 // unused
	dirRecordsThisDisk uint64 // unused
	directoryRecords   uint64
	directorySize      uint64
	directoryOffset    uint64 // relative to file
	commentLen         uint16
	comment            string
}

// timeZone returns a *time.Location based on the provided offset.
// If the offset is non-sensible, then this uses an offset of zero.
func timeZone(offset time.Duration) *time.Location {
	const (
		minOffset   = -12 * time.Hour  // E.g., Baker island at -12:00
		maxOffset   = +14 * time.Hour  // E.g., Line island at +14:00
		offsetAlias = 15 * time.Minute // E.g., Nepal at +5:45
	)
	offset = offset.Round(offsetAlias)
	if offset < minOffset || maxOffset < offset {
		offset = 0
	}
	return time.FixedZone("", int(offset/time.Second))
}

// msDosTimeToTime converts an MS-DOS date and time into a time.Time.
// The resolution is 2s.
// See: https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-dosdatetimetofiletime
func msDosTimeToTime(dosDate, dosTime uint16) time.Time {
	return time.Date(
		// date bits 0-4: day of month; 5-8: month; 9-15: years since 1980
		int(dosDate>>9+1980),
		time.Month(dosDate>>5&0xf),
		int(dosDate&0x1f),

		// time bits 0-4: second/2; 5-10: minute; 11-15: hour
		int(dosTime>>11),
		int(dosTime>>5&0x3f),
		int(dosTime&0x1f*2),
		0, // nanoseconds

		time.UTC,
	)
}

// timeToMsDosTime converts a time.Time to an MS-DOS date and time.
// The resolution is 2s.
// See: https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-filetimetodosdatetime
func timeToMsDosTime(t time.Time) (fDate uint16, fTime uint16) {
	fDate = uint16(t.Day() + int(t.Month())<<5 + (t.Year()-1980)<<9)
	fTime = uint16(t.Second()/2 + t.Minute()<<5 + t.Hour()<<11)
	return
}

// ModTime returns the modification time in UTC using the legacy
// [ModifiedDate] and [ModifiedTime] fields.
//
// Deprecated: Use [Modified] instead.
func (h *FileHeader) ModTime() time.Time {
	return msDosTimeToTime(h.ModifiedDate, h.ModifiedTime)
}

// SetModTime sets the [Modified], [ModifiedTime], and [ModifiedDate] fields
// to the given time in UTC.
//
// Deprecated: Use [Modified] instead.
func (h *FileHeader) SetModTime(t time.Time) {
	t = t.UTC() // Convert to UTC for compatibility
	h.Modified = t
	h.ModifiedDate, h.ModifiedTime = timeToMsDosTime(t)
}

const (
	// Unix constants. The specification doesn't mention them,
	// but these seem to be the values agreed on by tools.
	s_IFMT   = 0xf000
	s_IFSOCK = 0xc000
	s_IFLNK  = 0xa000
	s_IFREG  = 0x8000
	s_IFBLK  = 0x6000
	s_IFDIR  = 0x4000
	s_IFCHR  = 0x2000
	s_IFIFO  = 0x1000
	s_ISUID  = 0x800
	s_ISGID  = 0x400
	s_ISVTX  = 0x200

	msdosDir      = 0x10
	msdosReadOnly = 0x01
)

// Mode returns the permission and mode bits for the [FileHeader].
func (h *FileHeader) Mode() (mode fs.FileMode) {
	switch h.CreatorVersion >> 8 {
	case creatorUnix, creatorMacOSX:
		mode = unixModeToFileMode(h.ExternalAttrs >> 16)
	case creatorNTFS, creatorVFAT, creatorFAT:
		mode = msdosModeToFileMode(h.ExternalAttrs)
	}
	if len(h.Name) > 0 && h.Name[len(h.Name)-1] == '/' {
		mode |= fs.ModeDir
	}
	return mode
}

// SetMode changes the permission and mode bits for the [FileHeader].
func (h *FileHeader) SetMode(mode fs.FileMode) {
	h.CreatorVersion = h.CreatorVersion&0xff | creatorUnix<<8
	h.ExternalAttrs = fileModeToUnixMode(mode) << 16

	// set MSDOS attributes too, as the original zip does.
	if mode&fs.ModeDir != 0 {
		h.ExternalAttrs |= msdosDir
	}
	if mode&0200 == 0 {
		h.ExternalAttrs |= msdosReadOnly
	}
}

func (h *FileHeader) hasDataDescriptor() bool {
	return h.Flags&0x8 != 0
}

func msdosModeToFileMode(m uint32) (mode fs.FileMode) {
	if m&msdosDir != 0 {
		mode = fs.ModeDir | 0777
	} else {
		mode = 0666
	}
	if m&msdosReadOnly != 0 {
		mode &^= 0222
	}
	return mode
}

func fileModeToUnixMode(mode fs.FileMode) uint32 {
	var m uint32
	switch mode & fs.ModeType {
	default:
		m = s_IFREG
	case fs.ModeDir:
		m = s_IFDIR
	case fs.ModeSymlink:
		m = s_IFLNK
	case fs.ModeNamedPipe:
		m = s_IFIFO
	case fs.ModeSocket:
		m = s_IFSOCK
	case fs.ModeDevice:
		m = s_IFBLK
	case fs.ModeDevice | fs.ModeCharDevice:
		m = s_IFCHR
	}
	if mode&fs.ModeSetuid != 0 {
		m |= s_ISUID
	}
	if mode&fs.ModeSetgid != 0 {
		m |= s_ISGID
	}
	if mode&fs.ModeSticky != 0 {
		m |= s_ISVTX
	}
	return m | uint32(mode&0777)
}

func unixModeToFileMode(m uint32) fs.FileMode {
	mode := fs.FileMode(m & 0777)
	switch m & s_IFMT {
	case s_IFBLK:
		mode |= fs.ModeDevice
	case s_IFCHR:
		mode |= fs.ModeDevice | fs.ModeCharDevice
	case s_IFDIR:
		mode |= fs.ModeDir
	case s_IFIFO:
		mode |= fs.ModeNamedPipe
	case s_IFLNK:
		mode |= fs.ModeSymlink
	case s_IFREG:
		// nothing to do
	case s_IFSOCK:
		mode |= fs.ModeSocket
	}
	if m&s_ISGID != 0 {
		mode |= fs.ModeSetgid
	}
	if m&s_ISUID != 0 {
		mode |= fs.ModeSetuid
	}
	if m&s_ISVTX != 0 {
		mode |= fs.ModeSticky
	}
	return mode
}

```

// === FILE: references/go/src/archive/zip/writer.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zip

import (
	"bufio"
	"encoding/binary"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"io/fs"
	"strings"
	"unicode/utf8"
)

var (
	errLongName  = errors.New("zip: FileHeader.Name too long")
	errLongExtra = errors.New("zip: FileHeader.Extra too long")
)

// Writer implements a zip file writer.
type Writer struct {
	cw          *countWriter
	dir         []*header
	last        *fileWriter
	closed      bool
	compressors map[uint16]Compressor
	comment     string

	// testHookCloseSizeOffset if non-nil is called with the size
	// of offset of the central directory at Close.
	testHookCloseSizeOffset func(size, offset uint64)
}

type header struct {
	*FileHeader
	offset uint64
	raw    bool
}

// NewWriter returns a new [Writer] writing a zip file to w.
//
// Note that the exact bytes written to w are not covered by the Go 1
// compatibility promise. Callers, including tests, should not depend on the
// exact written bytes.
func NewWriter(w io.Writer) *Writer {
	return &Writer{cw: &countWriter{w: bufio.NewWriter(w)}}
}

// SetOffset sets the offset of the beginning of the zip data within the
// underlying writer. It should be used when the zip data is appended to an
// existing file, such as a binary executable.
// It must be called before any data is written.
func (w *Writer) SetOffset(n int64) {
	if w.cw.count != 0 {
		panic("zip: SetOffset called after data was written")
	}
	w.cw.count = n
}

// Flush flushes any buffered data to the underlying writer.
// Calling Flush is not normally necessary; calling Close is sufficient.
func (w *Writer) Flush() error {
	return w.cw.w.(*bufio.Writer).Flush()
}

// SetComment sets the end-of-central-directory comment field.
// It can only be called before [Writer.Close].
func (w *Writer) SetComment(comment string) error {
	if len(comment) > uint16max {
		return errors.New("zip: Writer.Comment too long")
	}
	w.comment = comment
	return nil
}

// Close finishes writing the zip file by writing the central directory.
// It does not close the underlying writer.
func (w *Writer) Close() error {
	if w.last != nil && !w.last.closed {
		if err := w.last.close(); err != nil {
			return err
		}
		w.last = nil
	}
	if w.closed {
		return errors.New("zip: writer closed twice")
	}
	w.closed = true

	// write central directory
	start := w.cw.count
	usedZip64 := false
	for _, h := range w.dir {
		// For the Central Directory, we always have the correct sizes.
		//
		// Implementations disagree on what triggers the inclusion of a Zip64
		// extra field: Info-ZIP only writes it if any size or offset EXCEEDS
		// 4GiB - 1, while libarchive writes it if any size REACHES OR EXCEEDS
		// 4GiB - 1, or if the offset EXCEEDS 4GiB - 1. The spec is ambiguous.
		//
		// We conservatively write Zip64 extra fields if any size or offset
		// REACHES OR EXCEEDS 4GiB - 1, to maximize compatibility with readers.
		// There is no ambiguity in parsing, so there is no downside to it.
		//
		// The spec is clear though that all and only the fields that REACH OR
		// EXCEED 4GiB - 1 are included in the Zip64 extra, once it's present.
		readerVersion := h.ReaderVersion
		if h.CompressedSize64 >= uint32max || h.UncompressedSize64 >= uint32max || h.offset >= uint32max {
			usedZip64 = true
			readerVersion = max(readerVersion, zipVersion45)
			var size uint16
			var buf [28]byte // 2x uint16 + up to 3x uint64
			eb := writeBuf(buf[:])
			eb.uint16(zip64ExtraID)
			eb.uint16(0) // size to be filled out later
			if h.UncompressedSize64 >= uint32max {
				eb.uint64(h.UncompressedSize64)
				size += 8
			}
			if h.CompressedSize64 >= uint32max {
				eb.uint64(h.CompressedSize64)
				size += 8
			}
			if h.offset >= uint32max {
				eb.uint64(h.offset)
				size += 8
			}
			sb := writeBuf(buf[2:])
			sb.uint16(size)
			h.Extra = append(h.Extra, buf[:4+size]...)
		}

		var buf [directoryHeaderLen]byte
		b := writeBuf(buf[:])
		b.uint32(uint32(directoryHeaderSignature))
		b.uint16(h.CreatorVersion)
		b.uint16(readerVersion)
		b.uint16(h.Flags)
		b.uint16(h.Method)
		b.uint16(h.ModifiedTime)
		b.uint16(h.ModifiedDate)
		b.uint32(h.CRC32)
		b.uint32(uint32(min(h.CompressedSize64, uint32max)))
		b.uint32(uint32(min(h.UncompressedSize64, uint32max)))
		b.uint16(uint16(len(h.Name)))
		b.uint16(uint16(len(h.Extra)))
		b.uint16(uint16(len(h.Comment)))
		b = b[4:] // skip disk number start and internal file attr (2x uint16)
		b.uint32(h.ExternalAttrs)
		b.uint32(uint32(min(h.offset, uint32max)))
		if _, err := w.cw.Write(buf[:]); err != nil {
			return err
		}
		if _, err := io.WriteString(w.cw, h.Name); err != nil {
			return err
		}
		if _, err := w.cw.Write(h.Extra); err != nil {
			return err
		}
		if _, err := io.WriteString(w.cw, h.Comment); err != nil {
			return err
		}
	}
	end := w.cw.count

	records := uint64(len(w.dir))
	size := uint64(end - start)
	offset := uint64(start)

	if f := w.testHookCloseSizeOffset; f != nil {
		f(size, offset)
	}

	// Emit the Zip64 EOCD records whenever any individual entry needed a Zip64
	// extra field, even if the EOCD's own fields fit in 32 bits, matching
	// Info-ZIP (but not libarchive). See APPNOTE 4.3.9.2: "when Zip64
	// extensions are in use, the EOCD64 record must be present."
	if usedZip64 || records >= uint16max || size >= uint32max || offset >= uint32max {
		var buf [directory64EndLen + directory64LocLen]byte
		b := writeBuf(buf[:])

		// zip64 end of central directory record
		b.uint32(directory64EndSignature)
		b.uint64(directory64EndLen - 12) // length minus signature (uint32) and length fields (uint64)
		b.uint16(zipVersion45)           // version made by
		b.uint16(zipVersion45)           // version needed to extract
		b.uint32(0)                      // number of this disk
		b.uint32(0)                      // number of the disk with the start of the central directory
		b.uint64(records)                // total number of entries in the central directory on this disk
		b.uint64(records)                // total number of entries in the central directory
		b.uint64(size)                   // size of the central directory
		b.uint64(offset)                 // offset of start of central directory with respect to the starting disk number

		// zip64 end of central directory locator
		b.uint32(directory64LocSignature)
		b.uint32(0)           // number of the disk with the start of the zip64 end of central directory
		b.uint64(uint64(end)) // relative offset of the zip64 end of central directory record
		b.uint32(1)           // total number of disks

		if _, err := w.cw.Write(buf[:]); err != nil {
			return err
		}
	}

	// write end record
	var buf [directoryEndLen]byte
	b := writeBuf(buf[:])
	b.uint32(uint32(directoryEndSignature))
	b = b[4:]                                 // skip over disk number and first disk number (2x uint16)
	b.uint16(uint16(min(uint16max, records))) // number of entries this disk
	b.uint16(uint16(min(uint16max, records))) // number of entries total
	b.uint32(uint32(min(uint32max, size)))    // size of directory
	b.uint32(uint32(min(uint32max, offset)))  // start of directory
	b.uint16(uint16(len(w.comment)))          // byte size of EOCD comment
	if _, err := w.cw.Write(buf[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(w.cw, w.comment); err != nil {
		return err
	}

	return w.cw.w.(*bufio.Writer).Flush()
}

// Create adds a file to the zip file using the provided name.
// It returns a [Writer] to which the file contents should be written.
// The file contents will be compressed using the [Deflate] method.
// The name must be a relative path: it must not start with a drive
// letter (e.g. C:) or leading slash, and only forward slashes are
// allowed. To create a directory instead of a file, add a trailing
// slash to the name. Duplicate names will not overwrite previous entries
// and are appended to the zip file.
// The file's contents must be written to the [io.Writer] before the next
// call to [Writer.Create], [Writer.CreateHeader], or [Writer.Close].
func (w *Writer) Create(name string) (io.Writer, error) {
	header := &FileHeader{
		Name:   name,
		Method: Deflate,
	}
	return w.CreateHeader(header)
}

// detectUTF8 reports whether s is a valid UTF-8 string, and whether the string
// must be considered UTF-8 encoding (i.e., not compatible with CP-437, ASCII,
// or any other common encoding).
func detectUTF8(s string) (valid, require bool) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		// Officially, ZIP uses CP-437, but many readers use the system's
		// local character encoding. Most encoding are compatible with a large
		// subset of CP-437, which itself is ASCII-like.
		//
		// Forbid 0x7e and 0x5c since EUC-KR and Shift-JIS replace those
		// characters with localized currency and overline characters.
		if r < 0x20 || r > 0x7d || r == 0x5c {
			if !utf8.ValidRune(r) || (r == utf8.RuneError && size == 1) {
				return false, false
			}
			require = true
		}
	}
	return true, require
}

// prepare performs the bookkeeping operations required at the start of
// CreateHeader and CreateRaw.
func (w *Writer) prepare(fh *FileHeader) error {
	if w.last != nil && !w.last.closed {
		if err := w.last.close(); err != nil {
			return err
		}
	}
	if len(w.dir) > 0 && w.dir[len(w.dir)-1].FileHeader == fh {
		// See https://golang.org/issue/11144 confusion.
		return errors.New("archive/zip: invalid duplicate FileHeader")
	}
	return nil
}

// CreateHeader adds a file to the zip archive using the provided [FileHeader]
// for the file metadata. [Writer] takes ownership of fh and may mutate
// its fields. The caller must not modify fh after calling [Writer.CreateHeader].
//
// This returns a [Writer] to which the file contents should be written.
// The file's contents must be written to the io.Writer before the next
// call to [Writer.Create], [Writer.CreateHeader], [Writer.CreateRaw], or [Writer.Close].
func (w *Writer) CreateHeader(fh *FileHeader) (io.Writer, error) {
	if err := w.prepare(fh); err != nil {
		return nil, err
	}

	// The ZIP format has a sad state of affairs regarding character encoding.
	// Officially, the name and comment fields are supposed to be encoded
	// in CP-437 (which is mostly compatible with ASCII), unless the UTF-8
	// flag bit is set. However, there are several problems:
	//
	//	* Many ZIP readers still do not support UTF-8.
	//	* If the UTF-8 flag is cleared, several readers simply interpret the
	//	name and comment fields as whatever the local system encoding is.
	//
	// In order to avoid breaking readers without UTF-8 support,
	// we avoid setting the UTF-8 flag if the strings are CP-437 compatible.
	// However, if the strings require multibyte UTF-8 encoding and is a
	// valid UTF-8 string, then we set the UTF-8 bit.
	//
	// For the case, where the user explicitly wants to specify the encoding
	// as UTF-8, they will need to set the flag bit themselves.
	utf8Valid1, utf8Require1 := detectUTF8(fh.Name)
	utf8Valid2, utf8Require2 := detectUTF8(fh.Comment)
	switch {
	case fh.NonUTF8:
		fh.Flags &^= 0x800
	case (utf8Require1 || utf8Require2) && (utf8Valid1 && utf8Valid2):
		fh.Flags |= 0x800
	}

	fh.CreatorVersion = fh.CreatorVersion&0xff00 | zipVersion20 // preserve compatibility byte
	fh.ReaderVersion = zipVersion20

	// If Modified is set, this takes precedence over MS-DOS timestamp fields.
	if !fh.Modified.IsZero() {
		// Contrary to the FileHeader.SetModTime method, we intentionally
		// do not convert to UTC, because we assume the user intends to encode
		// the date using the specified timezone. A user may want this control
		// because many legacy ZIP readers interpret the timestamp according
		// to the local timezone.
		//
		// The timezone is only non-UTC if a user directly sets the Modified
		// field directly themselves. All other approaches sets UTC.
		fh.ModifiedDate, fh.ModifiedTime = timeToMsDosTime(fh.Modified)

		// Use "extended timestamp" format since this is what Info-ZIP uses.
		// Nearly every major ZIP implementation uses a different format,
		// but at least most seem to be able to understand the other formats.
		//
		// This format happens to be identical for both local and central header
		// if modification time is the only timestamp being encoded.
		var mbuf [9]byte // 2*SizeOf(uint16) + SizeOf(uint8) + SizeOf(uint32)
		mt := uint32(fh.Modified.Unix())
		eb := writeBuf(mbuf[:])
		eb.uint16(extTimeExtraID)
		eb.uint16(5)  // Size: SizeOf(uint8) + SizeOf(uint32)
		eb.uint8(1)   // Flags: ModTime
		eb.uint32(mt) // ModTime
		fh.Extra = append(fh.Extra, mbuf[:]...)
	}

	var (
		ow io.Writer
		fw *fileWriter
	)
	h := &header{
		FileHeader: fh,
		offset:     uint64(w.cw.count),
	}

	if strings.HasSuffix(fh.Name, "/") {
		// Set the compression method to Store to ensure data length is truly zero,
		// which the writeHeader method always encodes for the size fields.
		// This is necessary as most compression formats have non-zero lengths
		// even when compressing an empty string.
		fh.Method = Store
		fh.Flags &^= 0x8 // we will not write a data descriptor

		// Explicitly clear sizes as they have no meaning for directories.
		fh.CompressedSize = 0
		fh.CompressedSize64 = 0
		fh.UncompressedSize = 0
		fh.UncompressedSize64 = 0

		ow = dirWriter{}
	} else {
		fh.Flags |= 0x8 // we will write a data descriptor

		fw = &fileWriter{
			zipw:      w.cw,
			compCount: &countWriter{w: w.cw},
			crc32:     crc32.NewIEEE(),
		}
		comp := w.compressor(fh.Method)
		if comp == nil {
			return nil, ErrAlgorithm
		}
		var err error
		fw.comp, err = comp(fw.compCount)
		if err != nil {
			return nil, err
		}
		fw.rawCount = &countWriter{w: fw.comp}
		fw.header = h
		ow = fw
	}
	w.dir = append(w.dir, h)
	if err := writeHeader(w.cw, h); err != nil {
		return nil, err
	}
	// If we're creating a directory, fw is nil.
	w.last = fw
	return ow, nil
}

func writeHeader(w io.Writer, h *header) error {
	const maxUint16 = 1<<16 - 1
	if len(h.Name) > maxUint16 {
		return errLongName
	}
	if len(h.Extra) > maxUint16 {
		return errLongExtra
	}

	// The correct behavior of a streaming writer, implemented by Info-ZIP 3.0,
	// would be to write 0xFFFFFFFF in the size fields and then write a Zip64
	// extra field with the sizes at zero (to signal they are stored in a ZIP64
	// data descriptor, in case the file is > 4GiB).
	//
	// We don't do that, and instead write zeroes directly in the size fields,
	// because that wastes 28 bytes for every file smaller than 4GiB, and
	// because it would change the encoding of nearly every zip file created by
	// archive/zip. (No one should rely on it being stable, but still.)
	//
	// Anyway, the Local File Header is not that important, as the Central
	// Directory is authoritative, and there we always write the correct sizes.
	//
	// If we do know the sizes, because [Writer.CreateRaw] is used and the data
	// descriptor flag is not set, then we write them to the header. If either
	// size reaches 4GiB, we write 0xFFFFFFFF placeholders and a Zip64 extra
	// field with BOTH sizes, per the spec and matching Info-ZIP. Note this is
	// different from the Central Directory Zip64 extra field logic, somehow.
	//
	// (One final interesting case that doesn't apply to us: if the input is
	// streaming but the output is seekable, Info-ZIP always writes Zip64 extra
	// fields, and then goes back and patches in the sizes, even for files < 4GiB.)

	var zip64ExtraInfo []byte
	readerVersion := h.ReaderVersion
	noDataDescriptor := h.raw && !h.hasDataDescriptor()
	if noDataDescriptor && (h.CompressedSize64 > uint32max || h.UncompressedSize64 > uint32max) {
		readerVersion = max(readerVersion, zipVersion45)
		zip64ExtraInfo = make([]byte, 20) // 2x uint16 + 2x uint64
		b := writeBuf(zip64ExtraInfo)
		b.uint16(zip64ExtraID)
		b.uint16(16) // size of Zip64 extra field data
		b.uint64(h.UncompressedSize64)
		b.uint64(h.CompressedSize64)
	}

	var buf [fileHeaderLen]byte
	b := writeBuf(buf[:])
	b.uint32(uint32(fileHeaderSignature))
	b.uint16(readerVersion)
	b.uint16(h.Flags)
	b.uint16(h.Method)
	b.uint16(h.ModifiedTime)
	b.uint16(h.ModifiedDate)
	if noDataDescriptor {
		b.uint32(h.CRC32)
		if zip64ExtraInfo != nil {
			b.uint32(uint32max)
			b.uint32(uint32max)
		} else {
			b.uint32(uint32(h.CompressedSize64))
			b.uint32(uint32(h.UncompressedSize64))
		}
	} else {
		b.uint32(0) // crc32
		b.uint32(0) // compressed size
		b.uint32(0) // uncompressed size
	}
	b.uint16(uint16(len(h.Name)))
	b.uint16(uint16(len(h.Extra) + len(zip64ExtraInfo)))
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(w, h.Name); err != nil {
		return err
	}
	if _, err := w.Write(h.Extra); err != nil {
		return err
	}
	if _, err := w.Write(zip64ExtraInfo); err != nil {
		return err
	}
	return nil
}

// CreateRaw adds a file to the zip archive using the provided [FileHeader] and
// returns a [Writer] to which the file contents should be written. The file's
// contents must be written to the io.Writer before the next call to [Writer.Create],
// [Writer.CreateHeader], [Writer.CreateRaw], or [Writer.Close].
//
// In contrast to [Writer.CreateHeader], the bytes passed to Writer are not compressed.
//
// CreateRaw's argument is stored in w. If the argument is a pointer to the embedded
// [FileHeader] in a [File] obtained from a [Reader] created from in-memory data,
// then w will refer to all of that memory.
func (w *Writer) CreateRaw(fh *FileHeader) (io.Writer, error) {
	if err := w.prepare(fh); err != nil {
		return nil, err
	}

	fh.CompressedSize = uint32(min(fh.CompressedSize64, uint32max))
	fh.UncompressedSize = uint32(min(fh.UncompressedSize64, uint32max))

	h := &header{
		FileHeader: fh,
		offset:     uint64(w.cw.count),
		raw:        true,
	}
	w.dir = append(w.dir, h)
	if err := writeHeader(w.cw, h); err != nil {
		return nil, err
	}

	if strings.HasSuffix(fh.Name, "/") {
		w.last = nil
		return dirWriter{}, nil
	}

	fw := &fileWriter{
		header: h,
		zipw:   w.cw,
	}
	w.last = fw
	return fw, nil
}

// Copy copies the file f (obtained from a [Reader]) into w. It copies the raw
// form directly bypassing decompression, compression, and validation.
func (w *Writer) Copy(f *File) error {
	r, err := f.OpenRaw()
	if err != nil {
		return err
	}
	// Copy the FileHeader so w doesn't store a pointer to the data
	// of f's entire archive. See #65499.
	fh := f.FileHeader
	fw, err := w.CreateRaw(&fh)
	if err != nil {
		return err
	}
	_, err = io.Copy(fw, r)
	return err
}

// RegisterCompressor registers or overrides a custom compressor for a specific
// method ID. If a compressor for a given method is not found, [Writer] will
// default to looking up the compressor at the package level.
func (w *Writer) RegisterCompressor(method uint16, comp Compressor) {
	if w.compressors == nil {
		w.compressors = make(map[uint16]Compressor)
	}
	w.compressors[method] = comp
}

// AddFS adds the files from fs.FS to the archive.
// It walks the directory tree starting at the root of the filesystem
// adding each file to the zip using deflate while maintaining the directory structure.
func (w *Writer) AddFS(fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !d.IsDir() && !info.Mode().IsRegular() {
			return errors.New("zip: cannot add non-regular file")
		}
		h, err := FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = name
		if d.IsDir() {
			h.Name += "/"
		}
		h.Method = Deflate
		fw, err := w.CreateHeader(h)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		f, err := fsys.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(fw, f)
		return err
	})
}

func (w *Writer) compressor(method uint16) Compressor {
	comp := w.compressors[method]
	if comp == nil {
		comp = compressor(method)
	}
	return comp
}

type dirWriter struct{}

func (dirWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	return 0, errors.New("zip: write to directory")
}

type fileWriter struct {
	*header
	zipw      io.Writer
	rawCount  *countWriter
	comp      io.WriteCloser
	compCount *countWriter
	crc32     hash.Hash32
	closed    bool
}

func (w *fileWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, errors.New("zip: write to closed file")
	}
	if w.raw {
		return w.zipw.Write(p)
	}
	w.crc32.Write(p)
	return w.rawCount.Write(p)
}

func (w *fileWriter) close() error {
	if w.closed {
		return errors.New("zip: file closed twice")
	}
	w.closed = true
	if w.raw {
		return w.writeDataDescriptor()
	}
	if err := w.comp.Close(); err != nil {
		return err
	}

	// update FileHeader
	fh := w.header.FileHeader
	fh.CRC32 = w.crc32.Sum32()
	fh.CompressedSize64 = uint64(w.compCount.count)
	fh.UncompressedSize64 = uint64(w.rawCount.count)

	if w.CompressedSize64 > uint32max || w.UncompressedSize64 > uint32max {
		fh.CompressedSize = uint32max
		fh.UncompressedSize = uint32max
		fh.ReaderVersion = zipVersion45 // requires 4.5 - File uses ZIP64 format extensions
	} else {
		fh.CompressedSize = uint32(fh.CompressedSize64)
		fh.UncompressedSize = uint32(fh.UncompressedSize64)
	}

	return w.writeDataDescriptor()
}

func (w *fileWriter) writeDataDescriptor() error {
	if !w.hasDataDescriptor() {
		return nil
	}
	// See the comment in [writeHeader] about how and why we don't signal ZIP64
	// mode in the local file header. If one of the sizes turns out to exceed
	// 4GiB, we use the 64-bit sizes anyway, for lack of alternatives.
	//
	// See also https://bugs.openjdk.org/browse/JDK-7073588.
	var buf []byte
	if w.CompressedSize64 > uint32max || w.UncompressedSize64 > uint32max {
		buf = make([]byte, dataDescriptor64Len)
	} else {
		buf = make([]byte, dataDescriptorLen)
	}
	b := writeBuf(buf)
	b.uint32(dataDescriptorSignature) // de-facto standard, required by OS X
	b.uint32(w.CRC32)
	if w.CompressedSize64 > uint32max || w.UncompressedSize64 > uint32max {
		b.uint64(w.CompressedSize64)
		b.uint64(w.UncompressedSize64)
	} else {
		b.uint32(w.CompressedSize)
		b.uint32(w.UncompressedSize)
	}
	_, err := w.zipw.Write(buf)
	return err
}

type countWriter struct {
	w     io.Writer
	count int64
}

func (w *countWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.count += int64(n)
	return n, err
}

type nopCloser struct {
	io.Writer
}

func (w nopCloser) Close() error {
	return nil
}

type writeBuf []byte

func (b *writeBuf) uint8(v uint8) {
	(*b)[0] = v
	*b = (*b)[1:]
}

func (b *writeBuf) uint16(v uint16) {
	binary.LittleEndian.PutUint16(*b, v)
	*b = (*b)[2:]
}

func (b *writeBuf) uint32(v uint32) {
	binary.LittleEndian.PutUint32(*b, v)
	*b = (*b)[4:]
}

func (b *writeBuf) uint64(v uint64) {
	binary.LittleEndian.PutUint64(*b, v)
	*b = (*b)[8:]
}

```

