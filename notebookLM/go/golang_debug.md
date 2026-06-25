# Domain Architecture: debug

## Layout Topology
```text
debug/
├── buildinfo
│   └── buildinfo.go
├── dwarf
│   ├── attr_string.go
│   ├── buf.go
│   ├── class_string.go
│   ├── const.go
│   ├── entry.go
│   ├── line.go
│   ├── open.go
│   ├── tag_string.go
│   ├── type.go
│   ├── typeunit.go
│   └── unit.go
├── elf
│   ├── elf.go
│   ├── file.go
│   └── reader.go
├── gosym
│   ├── pclntab.go
│   └── symtab.go
├── macho
│   ├── fat.go
│   ├── file.go
│   ├── macho.go
│   ├── reloctype.go
│   └── reloctype_string.go
├── pe
│   ├── file.go
│   ├── pe.go
│   ├── section.go
│   ├── string.go
│   └── symbol.go
└── plan9obj
    ├── file.go
    └── plan9obj.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/debug/buildinfo/buildinfo.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package buildinfo provides access to information embedded in a Go binary
// about how it was built. This includes the Go toolchain version, and the
// set of modules used (for binaries built in module mode).
//
// Build information is available for the currently running binary in
// runtime/debug.ReadBuildInfo.
package buildinfo

import (
	"bytes"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"debug/plan9obj"
	"encoding/binary"
	"errors"
	"fmt"
	"internal/saferio"
	"internal/xcoff"
	"io"
	"io/fs"
	"os"
	"runtime/debug"
	_ "unsafe" // for linkname
)

// Type alias for build info. We cannot move the types here, since
// runtime/debug would need to import this package, which would make it
// a much larger dependency.
type BuildInfo = debug.BuildInfo

// errUnrecognizedFormat is returned when a given executable file doesn't
// appear to be in a known format, or it breaks the rules of that format,
// or when there are I/O errors reading the file.
var errUnrecognizedFormat = errors.New("unrecognized file format")

// errNotGoExe is returned when a given executable file is valid but does
// not contain Go build information.
//
// errNotGoExe should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/quay/claircore
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname errNotGoExe
var errNotGoExe = errors.New("not a Go executable")

// The build info blob left by the linker is identified by a 32-byte header,
// consisting of buildInfoMagic (14 bytes), followed by version-dependent
// fields.
var buildInfoMagic = []byte("\xff Go buildinf:")

const (
	buildInfoAlign      = 16
	buildInfoHeaderSize = 32
)

// ReadFile returns build information embedded in a Go binary
// file at the given path. Most information is only available for binaries built
// with module support.
func ReadFile(name string) (info *BuildInfo, err error) {
	defer func() {
		if _, ok := errors.AsType[*fs.PathError](err); ok {
			err = fmt.Errorf("could not read Go build info: %w", err)
		} else if err != nil {
			err = fmt.Errorf("could not read Go build info from %s: %w", name, err)
		}
	}()

	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Read(f)
}

// Read returns build information embedded in a Go binary file
// accessed through the given ReaderAt. Most information is only available for
// binaries built with module support.
func Read(r io.ReaderAt) (*BuildInfo, error) {
	vers, mod, err := readRawBuildInfo(r)
	if err != nil {
		return nil, err
	}
	bi, err := debug.ParseBuildInfo(mod)
	if err != nil {
		return nil, err
	}
	bi.GoVersion = vers
	return bi, nil
}

type exe interface {
	// DataStart returns the virtual address and size of the segment or section that
	// should contain build information. This is either a specially named section
	// or the first writable non-zero data segment.
	DataStart() (uint64, uint64)

	// DataReader returns an io.ReaderAt that reads from addr until the end
	// of segment or section that contains addr.
	DataReader(addr uint64) (io.ReaderAt, error)
}

// readRawBuildInfo extracts the Go toolchain version and module information
// strings from a Go binary. On success, vers should be non-empty. mod
// is empty if the binary was not built with modules enabled.
func readRawBuildInfo(r io.ReaderAt) (vers, mod string, err error) {
	// Read the first bytes of the file to identify the format, then delegate to
	// a format-specific function to load segment and section headers.
	ident := make([]byte, 16)
	if n, err := r.ReadAt(ident, 0); n < len(ident) || err != nil {
		return "", "", errUnrecognizedFormat
	}

	var x exe
	switch {
	case bytes.HasPrefix(ident, []byte("\x7FELF")):
		f, err := elf.NewFile(r)
		if err != nil {
			return "", "", errUnrecognizedFormat
		}
		x = &elfExe{f}
	case bytes.HasPrefix(ident, []byte("MZ")):
		f, err := pe.NewFile(r)
		if err != nil {
			return "", "", errUnrecognizedFormat
		}
		x = &peExe{f}
	case bytes.HasPrefix(ident, []byte("\xFE\xED\xFA")) || bytes.HasPrefix(ident[1:], []byte("\xFA\xED\xFE")):
		f, err := macho.NewFile(r)
		if err != nil {
			return "", "", errUnrecognizedFormat
		}
		x = &machoExe{f}
	case bytes.HasPrefix(ident, []byte("\xCA\xFE\xBA\xBE")) || bytes.HasPrefix(ident, []byte("\xCA\xFE\xBA\xBF")):
		f, err := macho.NewFatFile(r)
		if err != nil || len(f.Arches) == 0 {
			return "", "", errUnrecognizedFormat
		}
		x = &machoExe{f.Arches[0].File}
	case bytes.HasPrefix(ident, []byte{0x01, 0xDF}) || bytes.HasPrefix(ident, []byte{0x01, 0xF7}):
		f, err := xcoff.NewFile(r)
		if err != nil {
			return "", "", errUnrecognizedFormat
		}
		x = &xcoffExe{f}
	case hasPlan9Magic(ident):
		f, err := plan9obj.NewFile(r)
		if err != nil {
			return "", "", errUnrecognizedFormat
		}
		x = &plan9objExe{f}
	default:
		return "", "", errUnrecognizedFormat
	}

	// Read segment or section to find the build info blob.
	// On some platforms, the blob will be in its own section, and DataStart
	// returns the address of that section. On others, it's somewhere in the
	// data segment; the linker puts it near the beginning.
	// See cmd/link/internal/ld.Link.buildinfo.
	dataAddr, dataSize := x.DataStart()
	if dataSize == 0 {
		return "", "", errNotGoExe
	}

	addr, err := searchMagic(x, dataAddr, dataSize)
	if err != nil {
		return "", "", err
	}

	// Read in the full header first.
	header, err := readData(x, addr, buildInfoHeaderSize)
	if err == io.EOF {
		return "", "", errNotGoExe
	} else if err != nil {
		return "", "", err
	}
	if len(header) < buildInfoHeaderSize {
		return "", "", errNotGoExe
	}

	const (
		ptrSizeOffset = 14
		flagsOffset   = 15
		versPtrOffset = 16

		flagsEndianMask   = 0x1
		flagsEndianLittle = 0x0
		flagsEndianBig    = 0x1

		flagsVersionMask = 0x2
		flagsVersionPtr  = 0x0
		flagsVersionInl  = 0x2
	)

	// Decode the blob. The blob is a 32-byte header, optionally followed
	// by 2 varint-prefixed string contents.
	//
	// type buildInfoHeader struct {
	// 	magic       [14]byte
	// 	ptrSize     uint8 // used if flagsVersionPtr
	// 	flags       uint8
	// 	versPtr     targetUintptr // used if flagsVersionPtr
	// 	modPtr      targetUintptr // used if flagsVersionPtr
	// }
	//
	// The version bit of the flags field determines the details of the format.
	//
	// Prior to 1.18, the flags version bit is flagsVersionPtr. In this
	// case, the header includes pointers to the version and modinfo Go
	// strings in the header. The ptrSize field indicates the size of the
	// pointers and the endian bit of the flag indicates the pointer
	// endianness.
	//
	// Since 1.18, the flags version bit is flagsVersionInl. In this case,
	// the header is followed by the string contents inline as
	// length-prefixed (as varint) string contents. First is the version
	// string, followed immediately by the modinfo string.
	flags := header[flagsOffset]
	if flags&flagsVersionMask == flagsVersionInl {
		vers, addr, err = decodeString(x, addr+buildInfoHeaderSize)
		if err != nil {
			return "", "", err
		}
		mod, _, err = decodeString(x, addr)
		if err != nil {
			return "", "", err
		}
	} else {
		// flagsVersionPtr (<1.18)
		ptrSize := int(header[ptrSizeOffset])
		bigEndian := flags&flagsEndianMask == flagsEndianBig
		var bo binary.ByteOrder
		if bigEndian {
			bo = binary.BigEndian
		} else {
			bo = binary.LittleEndian
		}
		var readPtr func([]byte) uint64
		if ptrSize == 4 {
			readPtr = func(b []byte) uint64 { return uint64(bo.Uint32(b)) }
		} else if ptrSize == 8 {
			readPtr = bo.Uint64
		} else {
			return "", "", errNotGoExe
		}
		vers = readString(x, ptrSize, readPtr, readPtr(header[versPtrOffset:]))
		mod = readString(x, ptrSize, readPtr, readPtr(header[versPtrOffset+ptrSize:]))
	}
	if vers == "" {
		return "", "", errNotGoExe
	}
	if len(mod) >= 33 && mod[len(mod)-17] == '\n' {
		// Strip module framing: sentinel strings delimiting the module info.
		// These are cmd/go/internal/modload.infoStart and infoEnd.
		mod = mod[16 : len(mod)-16]
	} else {
		mod = ""
	}

	return vers, mod, nil
}

func hasPlan9Magic(magic []byte) bool {
	if len(magic) >= 4 {
		m := binary.BigEndian.Uint32(magic)
		switch m {
		case plan9obj.Magic386, plan9obj.MagicAMD64, plan9obj.MagicARM:
			return true
		}
	}
	return false
}

func decodeString(x exe, addr uint64) (string, uint64, error) {
	// varint length followed by length bytes of data.

	// N.B. ReadData reads _up to_ size bytes from the section containing
	// addr. So we don't need to check that size doesn't overflow the
	// section.
	b, err := readData(x, addr, binary.MaxVarintLen64)
	if err == io.EOF {
		return "", 0, errNotGoExe
	} else if err != nil {
		return "", 0, err
	}

	length, n := binary.Uvarint(b)
	if n <= 0 {
		return "", 0, errNotGoExe
	}
	addr += uint64(n)

	b, err = readData(x, addr, length)
	if err == io.EOF {
		return "", 0, errNotGoExe
	} else if err == io.ErrUnexpectedEOF {
		// Length too large to allocate. Clearly bogus value.
		return "", 0, errNotGoExe
	} else if err != nil {
		return "", 0, err
	}
	if uint64(len(b)) < length {
		// Section ended before we could read the full string.
		return "", 0, errNotGoExe
	}

	return string(b), addr + length, nil
}

// readString returns the string at address addr in the executable x.
func readString(x exe, ptrSize int, readPtr func([]byte) uint64, addr uint64) string {
	hdr, err := readData(x, addr, uint64(2*ptrSize))
	if err != nil || len(hdr) < 2*ptrSize {
		return ""
	}
	dataAddr := readPtr(hdr)
	dataLen := readPtr(hdr[ptrSize:])
	data, err := readData(x, dataAddr, dataLen)
	if err != nil || uint64(len(data)) < dataLen {
		return ""
	}
	return string(data)
}

const searchChunkSize = 1 << 20 // 1 MB

// searchMagic returns the aligned first instance of buildInfoMagic in the data
// range [addr, addr+size). Returns false if not found.
func searchMagic(x exe, start, size uint64) (uint64, error) {
	end := start + size
	if end < start {
		// Overflow.
		return 0, errUnrecognizedFormat
	}

	// Round up start; magic can't occur in the initial unaligned portion.
	start = (start + buildInfoAlign - 1) &^ (buildInfoAlign - 1)
	if start >= end {
		return 0, errNotGoExe
	}

	var buf []byte
	for start < end {
		// Read in chunks to avoid consuming too much memory if data is large.
		//
		// Normally it would be somewhat painful to handle the magic crossing a
		// chunk boundary, but since it must be 16-byte aligned we know it will
		// fall within a single chunk.
		remaining := end - start
		chunkSize := uint64(searchChunkSize)
		if chunkSize > remaining {
			chunkSize = remaining
		}

		if buf == nil {
			buf = make([]byte, chunkSize)
		} else {
			// N.B. chunkSize can only decrease, and only on the
			// last chunk.
			buf = buf[:chunkSize]
			clear(buf)
		}

		n, err := readDataInto(x, start, buf)
		if err == io.EOF {
			// EOF before finding the magic; must not be a Go executable.
			return 0, errNotGoExe
		} else if err != nil {
			return 0, err
		}

		data := buf[:n]
		for len(data) > 0 {
			i := bytes.Index(data, buildInfoMagic)
			if i < 0 {
				break
			}
			if remaining-uint64(i) < buildInfoHeaderSize {
				// Found magic, but not enough space left for the full header.
				return 0, errNotGoExe
			}
			if i%buildInfoAlign != 0 {
				// Found magic, but misaligned. Keep searching.
				next := (i + buildInfoAlign - 1) &^ (buildInfoAlign - 1)
				if next > len(data) {
					// Corrupt object file: the remaining
					// count says there is more data,
					// but we didn't read it.
					return 0, errNotGoExe
				}
				data = data[next:]
				continue
			}
			// Good match!
			return start + uint64(i), nil
		}

		start += chunkSize
	}

	return 0, errNotGoExe
}

func readData(x exe, addr, size uint64) ([]byte, error) {
	r, err := x.DataReader(addr)
	if err != nil {
		return nil, err
	}

	b, err := saferio.ReadDataAt(r, size, 0)
	if len(b) > 0 && err == io.EOF {
		err = nil
	}
	return b, err
}

func readDataInto(x exe, addr uint64, b []byte) (int, error) {
	r, err := x.DataReader(addr)
	if err != nil {
		return 0, err
	}

	n, err := r.ReadAt(b, 0)
	if n > 0 && err == io.EOF {
		err = nil
	}
	return n, err
}

// elfExe is the ELF implementation of the exe interface.
type elfExe struct {
	f *elf.File
}

func (x *elfExe) DataReader(addr uint64) (io.ReaderAt, error) {
	for _, prog := range x.f.Progs {
		if prog.Vaddr <= addr && addr <= prog.Vaddr+prog.Filesz-1 {
			remaining := prog.Vaddr + prog.Filesz - addr
			return io.NewSectionReader(prog, int64(addr-prog.Vaddr), int64(remaining)), nil
		}
	}
	return nil, errUnrecognizedFormat
}

func (x *elfExe) DataStart() (uint64, uint64) {
	for _, s := range x.f.Sections {
		if s.Name == ".go.buildinfo" {
			return s.Addr, s.Size
		}
	}
	return 0, 0
}

// peExe is the PE (Windows Portable Executable) implementation of the exe interface.
type peExe struct {
	f *pe.File
}

func (x *peExe) imageBase() uint64 {
	switch oh := x.f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		return uint64(oh.ImageBase)
	case *pe.OptionalHeader64:
		return oh.ImageBase
	}
	return 0
}

func (x *peExe) DataReader(addr uint64) (io.ReaderAt, error) {
	addr -= x.imageBase()
	for _, sect := range x.f.Sections {
		if uint64(sect.VirtualAddress) <= addr && addr <= uint64(sect.VirtualAddress+sect.Size-1) {
			remaining := uint64(sect.VirtualAddress+sect.Size) - addr
			return io.NewSectionReader(sect, int64(addr-uint64(sect.VirtualAddress)), int64(remaining)), nil
		}
	}
	return nil, errUnrecognizedFormat
}

func (x *peExe) DataStart() (uint64, uint64) {
	// Assume data is first writable section.
	const (
		IMAGE_SCN_CNT_CODE               = 0x00000020
		IMAGE_SCN_CNT_INITIALIZED_DATA   = 0x00000040
		IMAGE_SCN_CNT_UNINITIALIZED_DATA = 0x00000080
		IMAGE_SCN_MEM_EXECUTE            = 0x20000000
		IMAGE_SCN_MEM_READ               = 0x40000000
		IMAGE_SCN_MEM_WRITE              = 0x80000000
		IMAGE_SCN_MEM_DISCARDABLE        = 0x2000000
		IMAGE_SCN_LNK_NRELOC_OVFL        = 0x1000000
		IMAGE_SCN_ALIGN_32BYTES          = 0x600000
	)
	for _, sect := range x.f.Sections {
		if sect.VirtualAddress != 0 && sect.Size != 0 &&
			sect.Characteristics&^IMAGE_SCN_ALIGN_32BYTES == IMAGE_SCN_CNT_INITIALIZED_DATA|IMAGE_SCN_MEM_READ|IMAGE_SCN_MEM_WRITE {
			return uint64(sect.VirtualAddress) + x.imageBase(), uint64(sect.VirtualSize)
		}
	}
	return 0, 0
}

// machoExe is the Mach-O (Apple macOS/iOS) implementation of the exe interface.
type machoExe struct {
	f *macho.File
}

func (x *machoExe) DataReader(addr uint64) (io.ReaderAt, error) {
	for _, load := range x.f.Loads {
		seg, ok := load.(*macho.Segment)
		if !ok {
			continue
		}
		if seg.Addr <= addr && addr <= seg.Addr+seg.Filesz-1 {
			if seg.Name == "__PAGEZERO" {
				continue
			}
			remaining := seg.Addr + seg.Filesz - addr
			return io.NewSectionReader(seg, int64(addr-seg.Addr), int64(remaining)), nil
		}
	}
	return nil, errUnrecognizedFormat
}

func (x *machoExe) DataStart() (uint64, uint64) {
	// Look for section named "__go_buildinfo".
	for _, sec := range x.f.Sections {
		if sec.Name == "__go_buildinfo" {
			return sec.Addr, sec.Size
		}
	}
	return 0, 0
}

// xcoffExe is the XCOFF (AIX eXtended COFF) implementation of the exe interface.
type xcoffExe struct {
	f *xcoff.File
}

func (x *xcoffExe) DataReader(addr uint64) (io.ReaderAt, error) {
	for _, sect := range x.f.Sections {
		if sect.VirtualAddress <= addr && addr <= sect.VirtualAddress+sect.Size-1 {
			remaining := sect.VirtualAddress + sect.Size - addr
			return io.NewSectionReader(sect, int64(addr-sect.VirtualAddress), int64(remaining)), nil
		}
	}
	return nil, errors.New("address not mapped")
}

func (x *xcoffExe) DataStart() (uint64, uint64) {
	if s := x.f.SectionByType(xcoff.STYP_DATA); s != nil {
		return s.VirtualAddress, s.Size
	}
	return 0, 0
}

// plan9objExe is the Plan 9 a.out implementation of the exe interface.
type plan9objExe struct {
	f *plan9obj.File
}

func (x *plan9objExe) DataStart() (uint64, uint64) {
	if s := x.f.Section("data"); s != nil {
		return uint64(s.Offset), uint64(s.Size)
	}
	return 0, 0
}

func (x *plan9objExe) DataReader(addr uint64) (io.ReaderAt, error) {
	for _, sect := range x.f.Sections {
		if uint64(sect.Offset) <= addr && addr <= uint64(sect.Offset+sect.Size-1) {
			remaining := uint64(sect.Offset+sect.Size) - addr
			return io.NewSectionReader(sect, int64(addr-uint64(sect.Offset)), int64(remaining)), nil
		}
	}
	return nil, errors.New("address not mapped")
}

```

// === FILE: references!/go/src/debug/dwarf/attr_string.go ===
```go
// Code generated by "stringer -type Attr -trimprefix=Attr"; DO NOT EDIT.

package dwarf

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[AttrSibling-1]
	_ = x[AttrLocation-2]
	_ = x[AttrName-3]
	_ = x[AttrOrdering-9]
	_ = x[AttrByteSize-11]
	_ = x[AttrBitOffset-12]
	_ = x[AttrBitSize-13]
	_ = x[AttrStmtList-16]
	_ = x[AttrLowpc-17]
	_ = x[AttrHighpc-18]
	_ = x[AttrLanguage-19]
	_ = x[AttrDiscr-21]
	_ = x[AttrDiscrValue-22]
	_ = x[AttrVisibility-23]
	_ = x[AttrImport-24]
	_ = x[AttrStringLength-25]
	_ = x[AttrCommonRef-26]
	_ = x[AttrCompDir-27]
	_ = x[AttrConstValue-28]
	_ = x[AttrContainingType-29]
	_ = x[AttrDefaultValue-30]
	_ = x[AttrInline-32]
	_ = x[AttrIsOptional-33]
	_ = x[AttrLowerBound-34]
	_ = x[AttrProducer-37]
	_ = x[AttrPrototyped-39]
	_ = x[AttrReturnAddr-42]
	_ = x[AttrStartScope-44]
	_ = x[AttrStrideSize-46]
	_ = x[AttrUpperBound-47]
	_ = x[AttrAbstractOrigin-49]
	_ = x[AttrAccessibility-50]
	_ = x[AttrAddrClass-51]
	_ = x[AttrArtificial-52]
	_ = x[AttrBaseTypes-53]
	_ = x[AttrCalling-54]
	_ = x[AttrCount-55]
	_ = x[AttrDataMemberLoc-56]
	_ = x[AttrDeclColumn-57]
	_ = x[AttrDeclFile-58]
	_ = x[AttrDeclLine-59]
	_ = x[AttrDeclaration-60]
	_ = x[AttrDiscrList-61]
	_ = x[AttrEncoding-62]
	_ = x[AttrExternal-63]
	_ = x[AttrFrameBase-64]
	_ = x[AttrFriend-65]
	_ = x[AttrIdentifierCase-66]
	_ = x[AttrMacroInfo-67]
	_ = x[AttrNamelistItem-68]
	_ = x[AttrPriority-69]
	_ = x[AttrSegment-70]
	_ = x[AttrSpecification-71]
	_ = x[AttrStaticLink-72]
	_ = x[AttrType-73]
	_ = x[AttrUseLocation-74]
	_ = x[AttrVarParam-75]
	_ = x[AttrVirtuality-76]
	_ = x[AttrVtableElemLoc-77]
	_ = x[AttrAllocated-78]
	_ = x[AttrAssociated-79]
	_ = x[AttrDataLocation-80]
	_ = x[AttrStride-81]
	_ = x[AttrEntrypc-82]
	_ = x[AttrUseUTF8-83]
	_ = x[AttrExtension-84]
	_ = x[AttrRanges-85]
	_ = x[AttrTrampoline-86]
	_ = x[AttrCallColumn-87]
	_ = x[AttrCallFile-88]
	_ = x[AttrCallLine-89]
	_ = x[AttrDescription-90]
	_ = x[AttrBinaryScale-91]
	_ = x[AttrDecimalScale-92]
	_ = x[AttrSmall-93]
	_ = x[AttrDecimalSign-94]
	_ = x[AttrDigitCount-95]
	_ = x[AttrPictureString-96]
	_ = x[AttrMutable-97]
	_ = x[AttrThreadsScaled-98]
	_ = x[AttrExplicit-99]
	_ = x[AttrObjectPointer-100]
	_ = x[AttrEndianity-101]
	_ = x[AttrElemental-102]
	_ = x[AttrPure-103]
	_ = x[AttrRecursive-104]
	_ = x[AttrSignature-105]
	_ = x[AttrMainSubprogram-106]
	_ = x[AttrDataBitOffset-107]
	_ = x[AttrConstExpr-108]
	_ = x[AttrEnumClass-109]
	_ = x[AttrLinkageName-110]
	_ = x[AttrStringLengthBitSize-111]
	_ = x[AttrStringLengthByteSize-112]
	_ = x[AttrRank-113]
	_ = x[AttrStrOffsetsBase-114]
	_ = x[AttrAddrBase-115]
	_ = x[AttrRnglistsBase-116]
	_ = x[AttrDwoName-118]
	_ = x[AttrReference-119]
	_ = x[AttrRvalueReference-120]
	_ = x[AttrMacros-121]
	_ = x[AttrCallAllCalls-122]
	_ = x[AttrCallAllSourceCalls-123]
	_ = x[AttrCallAllTailCalls-124]
	_ = x[AttrCallReturnPC-125]
	_ = x[AttrCallValue-126]
	_ = x[AttrCallOrigin-127]
	_ = x[AttrCallParameter-128]
	_ = x[AttrCallPC-129]
	_ = x[AttrCallTailCall-130]
	_ = x[AttrCallTarget-131]
	_ = x[AttrCallTargetClobbered-132]
	_ = x[AttrCallDataLocation-133]
	_ = x[AttrCallDataValue-134]
	_ = x[AttrNoreturn-135]
	_ = x[AttrAlignment-136]
	_ = x[AttrExportSymbols-137]
	_ = x[AttrDeleted-138]
	_ = x[AttrDefaulted-139]
	_ = x[AttrLoclistsBase-140]
}

const _Attr_name = "SiblingLocationNameOrderingByteSizeBitOffsetBitSizeStmtListLowpcHighpcLanguageDiscrDiscrValueVisibilityImportStringLengthCommonRefCompDirConstValueContainingTypeDefaultValueInlineIsOptionalLowerBoundProducerPrototypedReturnAddrStartScopeStrideSizeUpperBoundAbstractOriginAccessibilityAddrClassArtificialBaseTypesCallingCountDataMemberLocDeclColumnDeclFileDeclLineDeclarationDiscrListEncodingExternalFrameBaseFriendIdentifierCaseMacroInfoNamelistItemPrioritySegmentSpecificationStaticLinkTypeUseLocationVarParamVirtualityVtableElemLocAllocatedAssociatedDataLocationStrideEntrypcUseUTF8ExtensionRangesTrampolineCallColumnCallFileCallLineDescriptionBinaryScaleDecimalScaleSmallDecimalSignDigitCountPictureStringMutableThreadsScaledExplicitObjectPointerEndianityElementalPureRecursiveSignatureMainSubprogramDataBitOffsetConstExprEnumClassLinkageNameStringLengthBitSizeStringLengthByteSizeRankStrOffsetsBaseAddrBaseRnglistsBaseDwoNameReferenceRvalueReferenceMacrosCallAllCallsCallAllSourceCallsCallAllTailCallsCallReturnPCCallValueCallOriginCallParameterCallPCCallTailCallCallTargetCallTargetClobberedCallDataLocationCallDataValueNoreturnAlignmentExportSymbolsDeletedDefaultedLoclistsBase"

var _Attr_map = map[Attr]string{
	1:   _Attr_name[0:7],
	2:   _Attr_name[7:15],
	3:   _Attr_name[15:19],
	9:   _Attr_name[19:27],
	11:  _Attr_name[27:35],
	12:  _Attr_name[35:44],
	13:  _Attr_name[44:51],
	16:  _Attr_name[51:59],
	17:  _Attr_name[59:64],
	18:  _Attr_name[64:70],
	19:  _Attr_name[70:78],
	21:  _Attr_name[78:83],
	22:  _Attr_name[83:93],
	23:  _Attr_name[93:103],
	24:  _Attr_name[103:109],
	25:  _Attr_name[109:121],
	26:  _Attr_name[121:130],
	27:  _Attr_name[130:137],
	28:  _Attr_name[137:147],
	29:  _Attr_name[147:161],
	30:  _Attr_name[161:173],
	32:  _Attr_name[173:179],
	33:  _Attr_name[179:189],
	34:  _Attr_name[189:199],
	37:  _Attr_name[199:207],
	39:  _Attr_name[207:217],
	42:  _Attr_name[217:227],
	44:  _Attr_name[227:237],
	46:  _Attr_name[237:247],
	47:  _Attr_name[247:257],
	49:  _Attr_name[257:271],
	50:  _Attr_name[271:284],
	51:  _Attr_name[284:293],
	52:  _Attr_name[293:303],
	53:  _Attr_name[303:312],
	54:  _Attr_name[312:319],
	55:  _Attr_name[319:324],
	56:  _Attr_name[324:337],
	57:  _Attr_name[337:347],
	58:  _Attr_name[347:355],
	59:  _Attr_name[355:363],
	60:  _Attr_name[363:374],
	61:  _Attr_name[374:383],
	62:  _Attr_name[383:391],
	63:  _Attr_name[391:399],
	64:  _Attr_name[399:408],
	65:  _Attr_name[408:414],
	66:  _Attr_name[414:428],
	67:  _Attr_name[428:437],
	68:  _Attr_name[437:449],
	69:  _Attr_name[449:457],
	70:  _Attr_name[457:464],
	71:  _Attr_name[464:477],
	72:  _Attr_name[477:487],
	73:  _Attr_name[487:491],
	74:  _Attr_name[491:502],
	75:  _Attr_name[502:510],
	76:  _Attr_name[510:520],
	77:  _Attr_name[520:533],
	78:  _Attr_name[533:542],
	79:  _Attr_name[542:552],
	80:  _Attr_name[552:564],
	81:  _Attr_name[564:570],
	82:  _Attr_name[570:577],
	83:  _Attr_name[577:584],
	84:  _Attr_name[584:593],
	85:  _Attr_name[593:599],
	86:  _Attr_name[599:609],
	87:  _Attr_name[609:619],
	88:  _Attr_name[619:627],
	89:  _Attr_name[627:635],
	90:  _Attr_name[635:646],
	91:  _Attr_name[646:657],
	92:  _Attr_name[657:669],
	93:  _Attr_name[669:674],
	94:  _Attr_name[674:685],
	95:  _Attr_name[685:695],
	96:  _Attr_name[695:708],
	97:  _Attr_name[708:715],
	98:  _Attr_name[715:728],
	99:  _Attr_name[728:736],
	100: _Attr_name[736:749],
	101: _Attr_name[749:758],
	102: _Attr_name[758:767],
	103: _Attr_name[767:771],
	104: _Attr_name[771:780],
	105: _Attr_name[780:789],
	106: _Attr_name[789:803],
	107: _Attr_name[803:816],
	108: _Attr_name[816:825],
	109: _Attr_name[825:834],
	110: _Attr_name[834:845],
	111: _Attr_name[845:864],
	112: _Attr_name[864:884],
	113: _Attr_name[884:888],
	114: _Attr_name[888:902],
	115: _Attr_name[902:910],
	116: _Attr_name[910:922],
	118: _Attr_name[922:929],
	119: _Attr_name[929:938],
	120: _Attr_name[938:953],
	121: _Attr_name[953:959],
	122: _Attr_name[959:971],
	123: _Attr_name[971:989],
	124: _Attr_name[989:1005],
	125: _Attr_name[1005:1017],
	126: _Attr_name[1017:1026],
	127: _Attr_name[1026:1036],
	128: _Attr_name[1036:1049],
	129: _Attr_name[1049:1055],
	130: _Attr_name[1055:1067],
	131: _Attr_name[1067:1077],
	132: _Attr_name[1077:1096],
	133: _Attr_name[1096:1112],
	134: _Attr_name[1112:1125],
	135: _Attr_name[1125:1133],
	136: _Attr_name[1133:1142],
	137: _Attr_name[1142:1155],
	138: _Attr_name[1155:1162],
	139: _Attr_name[1162:1171],
	140: _Attr_name[1171:1183],
}

func (i Attr) String() string {
	if str, ok := _Attr_map[i]; ok {
		return str
	}
	return "Attr(" + strconv.FormatInt(int64(i), 10) + ")"
}

```

// === FILE: references!/go/src/debug/dwarf/buf.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Buffered reading and decoding of DWARF data streams.

package dwarf

import (
	"bytes"
	"encoding/binary"
	"strconv"
)

// Data buffer being decoded.
type buf struct {
	dwarf  *Data
	order  binary.ByteOrder
	format dataFormat
	name   string
	off    Offset
	data   []byte
	err    error
}

// Data format, other than byte order. This affects the handling of
// certain field formats.
type dataFormat interface {
	// DWARF version number. Zero means unknown.
	version() int

	// 64-bit DWARF format?
	dwarf64() (dwarf64 bool, isKnown bool)

	// Size of an address, in bytes. Zero means unknown.
	addrsize() int
}

// Some parts of DWARF have no data format, e.g., abbrevs.
type unknownFormat struct{}

func (u unknownFormat) version() int {
	return 0
}

func (u unknownFormat) dwarf64() (bool, bool) {
	return false, false
}

func (u unknownFormat) addrsize() int {
	return 0
}

func makeBuf(d *Data, format dataFormat, name string, off Offset, data []byte) buf {
	return buf{d, d.order, format, name, off, data, nil}
}

func (b *buf) uint8() uint8 {
	if len(b.data) < 1 {
		b.error("underflow")
		return 0
	}
	val := b.data[0]
	b.data = b.data[1:]
	b.off++
	return val
}

func (b *buf) bytes(n int) []byte {
	if n < 0 || len(b.data) < n {
		b.error("underflow")
		return nil
	}
	data := b.data[0:n]
	b.data = b.data[n:]
	b.off += Offset(n)
	return data
}

func (b *buf) skip(n int) { b.bytes(n) }

func (b *buf) string() string {
	i := bytes.IndexByte(b.data, 0)
	if i < 0 {
		b.error("underflow")
		return ""
	}

	s := string(b.data[0:i])
	b.data = b.data[i+1:]
	b.off += Offset(i + 1)
	return s
}

func (b *buf) uint16() uint16 {
	a := b.bytes(2)
	if a == nil {
		return 0
	}
	return b.order.Uint16(a)
}

func (b *buf) uint24() uint32 {
	a := b.bytes(3)
	if a == nil {
		return 0
	}
	if b.dwarf.bigEndian {
		return uint32(a[2]) | uint32(a[1])<<8 | uint32(a[0])<<16
	} else {
		return uint32(a[0]) | uint32(a[1])<<8 | uint32(a[2])<<16
	}
}

func (b *buf) uint32() uint32 {
	a := b.bytes(4)
	if a == nil {
		return 0
	}
	return b.order.Uint32(a)
}

func (b *buf) uint64() uint64 {
	a := b.bytes(8)
	if a == nil {
		return 0
	}
	return b.order.Uint64(a)
}

// Read a varint, which is 7 bits per byte, little endian.
// the 0x80 bit means read another byte.
func (b *buf) varint() (c uint64, bits uint) {
	for i := 0; i < len(b.data); i++ {
		byte := b.data[i]
		c |= uint64(byte&0x7F) << bits
		bits += 7
		if byte&0x80 == 0 {
			b.off += Offset(i + 1)
			b.data = b.data[i+1:]
			return c, bits
		}
	}
	return 0, 0
}

// Unsigned int is just a varint.
func (b *buf) uint() uint64 {
	x, bits := b.varint()
	if bits == 0 {
		b.error("underflow")
	}
	return x
}

// Signed int is a sign-extended varint.
func (b *buf) int() int64 {
	ux, bits := b.varint()
	x := int64(ux)
	if x&(1<<(bits-1)) != 0 {
		x |= -1 << bits
	}
	if bits == 0 {
		b.error("underflow")
	}
	return x
}

// Address-sized uint.
func (b *buf) addr() uint64 {
	switch b.format.addrsize() {
	case 1:
		return uint64(b.uint8())
	case 2:
		return uint64(b.uint16())
	case 4:
		return uint64(b.uint32())
	case 8:
		return b.uint64()
	}
	b.error("unknown address size")
	return 0
}

func (b *buf) unitLength() (length Offset, dwarf64 bool) {
	length = Offset(b.uint32())
	if length == 0xffffffff {
		dwarf64 = true
		length = Offset(b.uint64())
	} else if length >= 0xfffffff0 {
		b.error("unit length has reserved value")
	}
	return
}

func (b *buf) error(s string) {
	if b.err == nil {
		b.data = nil
		b.err = DecodeError{b.name, b.off, s}
	}
}

type DecodeError struct {
	Name   string
	Offset Offset
	Err    string
}

func (e DecodeError) Error() string {
	return "decoding dwarf section " + e.Name + " at offset 0x" + strconv.FormatInt(int64(e.Offset), 16) + ": " + e.Err
}

```

// === FILE: references!/go/src/debug/dwarf/class_string.go ===
```go
// Code generated by "stringer -type=Class"; DO NOT EDIT.

package dwarf

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ClassUnknown-0]
	_ = x[ClassAddress-1]
	_ = x[ClassBlock-2]
	_ = x[ClassConstant-3]
	_ = x[ClassExprLoc-4]
	_ = x[ClassFlag-5]
	_ = x[ClassLinePtr-6]
	_ = x[ClassLocListPtr-7]
	_ = x[ClassMacPtr-8]
	_ = x[ClassRangeListPtr-9]
	_ = x[ClassReference-10]
	_ = x[ClassReferenceSig-11]
	_ = x[ClassString-12]
	_ = x[ClassReferenceAlt-13]
	_ = x[ClassStringAlt-14]
	_ = x[ClassAddrPtr-15]
	_ = x[ClassLocList-16]
	_ = x[ClassRngList-17]
	_ = x[ClassRngListsPtr-18]
	_ = x[ClassStrOffsetsPtr-19]
}

const _Class_name = "ClassUnknownClassAddressClassBlockClassConstantClassExprLocClassFlagClassLinePtrClassLocListPtrClassMacPtrClassRangeListPtrClassReferenceClassReferenceSigClassStringClassReferenceAltClassStringAltClassAddrPtrClassLocListClassRngListClassRngListsPtrClassStrOffsetsPtr"

var _Class_index = [...]uint16{0, 12, 24, 34, 47, 59, 68, 80, 95, 106, 123, 137, 154, 165, 182, 196, 208, 220, 232, 248, 266}

func (i Class) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_Class_index)-1 {
		return "Class(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Class_name[_Class_index[idx]:_Class_index[idx+1]]
}

```

// === FILE: references!/go/src/debug/dwarf/const.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Constants

package dwarf

//go:generate stringer -type Attr -trimprefix=Attr

// An Attr identifies the attribute type in a DWARF [Entry.Field].
type Attr uint32

const (
	AttrSibling        Attr = 0x01
	AttrLocation       Attr = 0x02
	AttrName           Attr = 0x03
	AttrOrdering       Attr = 0x09
	AttrByteSize       Attr = 0x0B
	AttrBitOffset      Attr = 0x0C
	AttrBitSize        Attr = 0x0D
	AttrStmtList       Attr = 0x10
	AttrLowpc          Attr = 0x11
	AttrHighpc         Attr = 0x12
	AttrLanguage       Attr = 0x13
	AttrDiscr          Attr = 0x15
	AttrDiscrValue     Attr = 0x16
	AttrVisibility     Attr = 0x17
	AttrImport         Attr = 0x18
	AttrStringLength   Attr = 0x19
	AttrCommonRef      Attr = 0x1A
	AttrCompDir        Attr = 0x1B
	AttrConstValue     Attr = 0x1C
	AttrContainingType Attr = 0x1D
	AttrDefaultValue   Attr = 0x1E
	AttrInline         Attr = 0x20
	AttrIsOptional     Attr = 0x21
	AttrLowerBound     Attr = 0x22
	AttrProducer       Attr = 0x25
	AttrPrototyped     Attr = 0x27
	AttrReturnAddr     Attr = 0x2A
	AttrStartScope     Attr = 0x2C
	AttrStrideSize     Attr = 0x2E
	AttrUpperBound     Attr = 0x2F
	AttrAbstractOrigin Attr = 0x31
	AttrAccessibility  Attr = 0x32
	AttrAddrClass      Attr = 0x33
	AttrArtificial     Attr = 0x34
	AttrBaseTypes      Attr = 0x35
	AttrCalling        Attr = 0x36
	AttrCount          Attr = 0x37
	AttrDataMemberLoc  Attr = 0x38
	AttrDeclColumn     Attr = 0x39
	AttrDeclFile       Attr = 0x3A
	AttrDeclLine       Attr = 0x3B
	AttrDeclaration    Attr = 0x3C
	AttrDiscrList      Attr = 0x3D
	AttrEncoding       Attr = 0x3E
	AttrExternal       Attr = 0x3F
	AttrFrameBase      Attr = 0x40
	AttrFriend         Attr = 0x41
	AttrIdentifierCase Attr = 0x42
	AttrMacroInfo      Attr = 0x43
	AttrNamelistItem   Attr = 0x44
	AttrPriority       Attr = 0x45
	AttrSegment        Attr = 0x46
	AttrSpecification  Attr = 0x47
	AttrStaticLink     Attr = 0x48
	AttrType           Attr = 0x49
	AttrUseLocation    Attr = 0x4A
	AttrVarParam       Attr = 0x4B
	AttrVirtuality     Attr = 0x4C
	AttrVtableElemLoc  Attr = 0x4D
	// The following are new in DWARF 3.
	AttrAllocated     Attr = 0x4E
	AttrAssociated    Attr = 0x4F
	AttrDataLocation  Attr = 0x50
	AttrStride        Attr = 0x51
	AttrEntrypc       Attr = 0x52
	AttrUseUTF8       Attr = 0x53
	AttrExtension     Attr = 0x54
	AttrRanges        Attr = 0x55
	AttrTrampoline    Attr = 0x56
	AttrCallColumn    Attr = 0x57
	AttrCallFile      Attr = 0x58
	AttrCallLine      Attr = 0x59
	AttrDescription   Attr = 0x5A
	AttrBinaryScale   Attr = 0x5B
	AttrDecimalScale  Attr = 0x5C
	AttrSmall         Attr = 0x5D
	AttrDecimalSign   Attr = 0x5E
	AttrDigitCount    Attr = 0x5F
	AttrPictureString Attr = 0x60
	AttrMutable       Attr = 0x61
	AttrThreadsScaled Attr = 0x62
	AttrExplicit      Attr = 0x63
	AttrObjectPointer Attr = 0x64
	AttrEndianity     Attr = 0x65
	AttrElemental     Attr = 0x66
	AttrPure          Attr = 0x67
	AttrRecursive     Attr = 0x68
	// The following are new in DWARF 4.
	AttrSignature      Attr = 0x69
	AttrMainSubprogram Attr = 0x6A
	AttrDataBitOffset  Attr = 0x6B
	AttrConstExpr      Attr = 0x6C
	AttrEnumClass      Attr = 0x6D
	AttrLinkageName    Attr = 0x6E
	// The following are new in DWARF 5.
	AttrStringLengthBitSize  Attr = 0x6F
	AttrStringLengthByteSize Attr = 0x70
	AttrRank                 Attr = 0x71
	AttrStrOffsetsBase       Attr = 0x72
	AttrAddrBase             Attr = 0x73
	AttrRnglistsBase         Attr = 0x74
	AttrDwoName              Attr = 0x76
	AttrReference            Attr = 0x77
	AttrRvalueReference      Attr = 0x78
	AttrMacros               Attr = 0x79
	AttrCallAllCalls         Attr = 0x7A
	AttrCallAllSourceCalls   Attr = 0x7B
	AttrCallAllTailCalls     Attr = 0x7C
	AttrCallReturnPC         Attr = 0x7D
	AttrCallValue            Attr = 0x7E
	AttrCallOrigin           Attr = 0x7F
	AttrCallParameter        Attr = 0x80
	AttrCallPC               Attr = 0x81
	AttrCallTailCall         Attr = 0x82
	AttrCallTarget           Attr = 0x83
	AttrCallTargetClobbered  Attr = 0x84
	AttrCallDataLocation     Attr = 0x85
	AttrCallDataValue        Attr = 0x86
	AttrNoreturn             Attr = 0x87
	AttrAlignment            Attr = 0x88
	AttrExportSymbols        Attr = 0x89
	AttrDeleted              Attr = 0x8A
	AttrDefaulted            Attr = 0x8B
	AttrLoclistsBase         Attr = 0x8C
)

func (a Attr) GoString() string {
	if str, ok := _Attr_map[a]; ok {
		return "dwarf.Attr" + str
	}
	return "dwarf." + a.String()
}

// A format is a DWARF data encoding format.
type format uint32

const (
	// value formats
	formAddr        format = 0x01
	formDwarfBlock2 format = 0x03
	formDwarfBlock4 format = 0x04
	formData2       format = 0x05
	formData4       format = 0x06
	formData8       format = 0x07
	formString      format = 0x08
	formDwarfBlock  format = 0x09
	formDwarfBlock1 format = 0x0A
	formData1       format = 0x0B
	formFlag        format = 0x0C
	formSdata       format = 0x0D
	formStrp        format = 0x0E
	formUdata       format = 0x0F
	formRefAddr     format = 0x10
	formRef1        format = 0x11
	formRef2        format = 0x12
	formRef4        format = 0x13
	formRef8        format = 0x14
	formRefUdata    format = 0x15
	formIndirect    format = 0x16
	// The following are new in DWARF 4.
	formSecOffset   format = 0x17
	formExprloc     format = 0x18
	formFlagPresent format = 0x19
	formRefSig8     format = 0x20
	// The following are new in DWARF 5.
	formStrx          format = 0x1A
	formAddrx         format = 0x1B
	formRefSup4       format = 0x1C
	formStrpSup       format = 0x1D
	formData16        format = 0x1E
	formLineStrp      format = 0x1F
	formImplicitConst format = 0x21
	formLoclistx      format = 0x22
	formRnglistx      format = 0x23
	formRefSup8       format = 0x24
	formStrx1         format = 0x25
	formStrx2         format = 0x26
	formStrx3         format = 0x27
	formStrx4         format = 0x28
	formAddrx1        format = 0x29
	formAddrx2        format = 0x2A
	formAddrx3        format = 0x2B
	formAddrx4        format = 0x2C
	// Extensions for multi-file compression (.dwz)
	// http://www.dwarfstd.org/ShowIssue.php?issue=120604.1
	formGnuRefAlt  format = 0x1f20
	formGnuStrpAlt format = 0x1f21
)

//go:generate stringer -type Tag -trimprefix=Tag

// A Tag is the classification (the type) of an [Entry].
type Tag uint32

const (
	TagArrayType              Tag = 0x01
	TagClassType              Tag = 0x02
	TagEntryPoint             Tag = 0x03
	TagEnumerationType        Tag = 0x04
	TagFormalParameter        Tag = 0x05
	TagImportedDeclaration    Tag = 0x08
	TagLabel                  Tag = 0x0A
	TagLexDwarfBlock          Tag = 0x0B
	TagMember                 Tag = 0x0D
	TagPointerType            Tag = 0x0F
	TagReferenceType          Tag = 0x10
	TagCompileUnit            Tag = 0x11
	TagStringType             Tag = 0x12
	TagStructType             Tag = 0x13
	TagSubroutineType         Tag = 0x15
	TagTypedef                Tag = 0x16
	TagUnionType              Tag = 0x17
	TagUnspecifiedParameters  Tag = 0x18
	TagVariant                Tag = 0x19
	TagCommonDwarfBlock       Tag = 0x1A
	TagCommonInclusion        Tag = 0x1B
	TagInheritance            Tag = 0x1C
	TagInlinedSubroutine      Tag = 0x1D
	TagModule                 Tag = 0x1E
	TagPtrToMemberType        Tag = 0x1F
	TagSetType                Tag = 0x20
	TagSubrangeType           Tag = 0x21
	TagWithStmt               Tag = 0x22
	TagAccessDeclaration      Tag = 0x23
	TagBaseType               Tag = 0x24
	TagCatchDwarfBlock        Tag = 0x25
	TagConstType              Tag = 0x26
	TagConstant               Tag = 0x27
	TagEnumerator             Tag = 0x28
	TagFileType               Tag = 0x29
	TagFriend                 Tag = 0x2A
	TagNamelist               Tag = 0x2B
	TagNamelistItem           Tag = 0x2C
	TagPackedType             Tag = 0x2D
	TagSubprogram             Tag = 0x2E
	TagTemplateTypeParameter  Tag = 0x2F
	TagTemplateValueParameter Tag = 0x30
	TagThrownType             Tag = 0x31
	TagTryDwarfBlock          Tag = 0x32
	TagVariantPart            Tag = 0x33
	TagVariable               Tag = 0x34
	TagVolatileType           Tag = 0x35
	// The following are new in DWARF 3.
	TagDwarfProcedure  Tag = 0x36
	TagRestrictType    Tag = 0x37
	TagInterfaceType   Tag = 0x38
	TagNamespace       Tag = 0x39
	TagImportedModule  Tag = 0x3A
	TagUnspecifiedType Tag = 0x3B
	TagPartialUnit     Tag = 0x3C
	TagImportedUnit    Tag = 0x3D
	TagMutableType     Tag = 0x3E // Later removed from DWARF.
	TagCondition       Tag = 0x3F
	TagSharedType      Tag = 0x40
	// The following are new in DWARF 4.
	TagTypeUnit            Tag = 0x41
	TagRvalueReferenceType Tag = 0x42
	TagTemplateAlias       Tag = 0x43
	// The following are new in DWARF 5.
	TagCoarrayType       Tag = 0x44
	TagGenericSubrange   Tag = 0x45
	TagDynamicType       Tag = 0x46
	TagAtomicType        Tag = 0x47
	TagCallSite          Tag = 0x48
	TagCallSiteParameter Tag = 0x49
	TagSkeletonUnit      Tag = 0x4A
	TagImmutableType     Tag = 0x4B
)

func (t Tag) GoString() string {
	if t <= TagTemplateAlias {
		return "dwarf.Tag" + t.String()
	}
	return "dwarf." + t.String()
}

// Location expression operators.
// The debug info encodes value locations like 8(R3)
// as a sequence of these op codes.
// This package does not implement full expressions;
// the opPlusUconst operator is expected by the type parser.
const (
	opAddr       = 0x03 /* 1 op, const addr */
	opDeref      = 0x06
	opConst1u    = 0x08 /* 1 op, 1 byte const */
	opConst1s    = 0x09 /*	" signed */
	opConst2u    = 0x0A /* 1 op, 2 byte const  */
	opConst2s    = 0x0B /*	" signed */
	opConst4u    = 0x0C /* 1 op, 4 byte const */
	opConst4s    = 0x0D /*	" signed */
	opConst8u    = 0x0E /* 1 op, 8 byte const */
	opConst8s    = 0x0F /*	" signed */
	opConstu     = 0x10 /* 1 op, LEB128 const */
	opConsts     = 0x11 /*	" signed */
	opDup        = 0x12
	opDrop       = 0x13
	opOver       = 0x14
	opPick       = 0x15 /* 1 op, 1 byte stack index */
	opSwap       = 0x16
	opRot        = 0x17
	opXderef     = 0x18
	opAbs        = 0x19
	opAnd        = 0x1A
	opDiv        = 0x1B
	opMinus      = 0x1C
	opMod        = 0x1D
	opMul        = 0x1E
	opNeg        = 0x1F
	opNot        = 0x20
	opOr         = 0x21
	opPlus       = 0x22
	opPlusUconst = 0x23 /* 1 op, ULEB128 addend */
	opShl        = 0x24
	opShr        = 0x25
	opShra       = 0x26
	opXor        = 0x27
	opSkip       = 0x2F /* 1 op, signed 2-byte constant */
	opBra        = 0x28 /* 1 op, signed 2-byte constant */
	opEq         = 0x29
	opGe         = 0x2A
	opGt         = 0x2B
	opLe         = 0x2C
	opLt         = 0x2D
	opNe         = 0x2E
	opLit0       = 0x30
	/* OpLitN = OpLit0 + N for N = 0..31 */
	opReg0 = 0x50
	/* OpRegN = OpReg0 + N for N = 0..31 */
	opBreg0 = 0x70 /* 1 op, signed LEB128 constant */
	/* OpBregN = OpBreg0 + N for N = 0..31 */
	opRegx       = 0x90 /* 1 op, ULEB128 register */
	opFbreg      = 0x91 /* 1 op, SLEB128 offset */
	opBregx      = 0x92 /* 2 op, ULEB128 reg; SLEB128 off */
	opPiece      = 0x93 /* 1 op, ULEB128 size of piece */
	opDerefSize  = 0x94 /* 1-byte size of data retrieved */
	opXderefSize = 0x95 /* 1-byte size of data retrieved */
	opNop        = 0x96
	// The following are new in DWARF 3.
	opPushObjAddr    = 0x97
	opCall2          = 0x98 /* 2-byte offset of DIE */
	opCall4          = 0x99 /* 4-byte offset of DIE */
	opCallRef        = 0x9A /* 4- or 8- byte offset of DIE */
	opFormTLSAddress = 0x9B
	opCallFrameCFA   = 0x9C
	opBitPiece       = 0x9D
	// The following are new in DWARF 4.
	opImplicitValue = 0x9E
	opStackValue    = 0x9F
	// The following a new in DWARF 5.
	opImplicitPointer = 0xA0
	opAddrx           = 0xA1
	opConstx          = 0xA2
	opEntryValue      = 0xA3
	opConstType       = 0xA4
	opRegvalType      = 0xA5
	opDerefType       = 0xA6
	opXderefType      = 0xA7
	opConvert         = 0xA8
	opReinterpret     = 0xA9
	/* 0xE0-0xFF reserved for user-specific */
)

// Basic type encodings -- the value for AttrEncoding in a TagBaseType Entry.
const (
	encAddress      = 0x01
	encBoolean      = 0x02
	encComplexFloat = 0x03
	encFloat        = 0x04
	encSigned       = 0x05
	encSignedChar   = 0x06
	encUnsigned     = 0x07
	encUnsignedChar = 0x08
	// The following are new in DWARF 3.
	encImaginaryFloat = 0x09
	encPackedDecimal  = 0x0A
	encNumericString  = 0x0B
	encEdited         = 0x0C
	encSignedFixed    = 0x0D
	encUnsignedFixed  = 0x0E
	encDecimalFloat   = 0x0F
	// The following are new in DWARF 4.
	encUTF = 0x10
	// The following are new in DWARF 5.
	encUCS   = 0x11
	encASCII = 0x12
)

// Statement program standard opcode encodings.
const (
	lnsCopy           = 1
	lnsAdvancePC      = 2
	lnsAdvanceLine    = 3
	lnsSetFile        = 4
	lnsSetColumn      = 5
	lnsNegateStmt     = 6
	lnsSetBasicBlock  = 7
	lnsConstAddPC     = 8
	lnsFixedAdvancePC = 9

	// DWARF 3
	lnsSetPrologueEnd   = 10
	lnsSetEpilogueBegin = 11
	lnsSetISA           = 12
)

// Statement program extended opcode encodings.
const (
	lneEndSequence = 1
	lneSetAddress  = 2
	lneDefineFile  = 3

	// DWARF 4
	lneSetDiscriminator = 4
)

// Line table directory and file name entry formats.
// These are new in DWARF 5.
const (
	lnctPath           = 0x01
	lnctDirectoryIndex = 0x02
	lnctTimestamp      = 0x03
	lnctSize           = 0x04
	lnctMD5            = 0x05
)

// Location list entry codes.
// These are new in DWARF 5.
const (
	lleEndOfList       = 0x00
	lleBaseAddressx    = 0x01
	lleStartxEndx      = 0x02
	lleStartxLength    = 0x03
	lleOffsetPair      = 0x04
	lleDefaultLocation = 0x05
	lleBaseAddress     = 0x06
	lleStartEnd        = 0x07
	lleStartLength     = 0x08
)

// Unit header unit type encodings.
// These are new in DWARF 5.
const (
	utCompile      = 0x01
	utType         = 0x02
	utPartial      = 0x03
	utSkeleton     = 0x04
	utSplitCompile = 0x05
	utSplitType    = 0x06
)

// Opcodes for DWARFv5 debug_rnglists section.
const (
	rleEndOfList    = 0x0
	rleBaseAddressx = 0x1
	rleStartxEndx   = 0x2
	rleStartxLength = 0x3
	rleOffsetPair   = 0x4
	rleBaseAddress  = 0x5
	rleStartEnd     = 0x6
	rleStartLength  = 0x7
)

```

// === FILE: references!/go/src/debug/dwarf/entry.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// DWARF debug information entry parser.
// An entry is a sequence of data items of a given format.
// The first word in the entry is an index into what DWARF
// calls the ``abbreviation table.''  An abbreviation is really
// just a type descriptor: it's an array of attribute tag/value format pairs.

package dwarf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
)

// a single entry's description: a sequence of attributes
type abbrev struct {
	tag      Tag
	children bool
	field    []afield
}

type afield struct {
	attr  Attr
	fmt   format
	class Class
	val   int64 // for formImplicitConst
}

// a map from entry format ids to their descriptions
type abbrevTable map[uint32]abbrev

// parseAbbrev returns the abbreviation table that starts at byte off
// in the .debug_abbrev section.
func (d *Data) parseAbbrev(off uint64, vers int) (abbrevTable, error) {
	if m, ok := d.abbrevCache[off]; ok {
		return m, nil
	}

	data := d.abbrev
	if off > uint64(len(data)) {
		data = nil
	} else {
		data = data[off:]
	}
	b := makeBuf(d, unknownFormat{}, "abbrev", 0, data)

	// Error handling is simplified by the buf getters
	// returning an endless stream of 0s after an error.
	m := make(abbrevTable)
	for {
		// Table ends with id == 0.
		id := uint32(b.uint())
		if id == 0 {
			break
		}

		// Walk over attributes, counting.
		n := 0
		b1 := b // Read from copy of b.
		b1.uint()
		b1.uint8()
		for {
			tag := b1.uint()
			fmt := b1.uint()
			if tag == 0 && fmt == 0 {
				break
			}
			if format(fmt) == formImplicitConst {
				b1.int()
			}
			n++
		}
		if b1.err != nil {
			return nil, b1.err
		}

		// Walk over attributes again, this time writing them down.
		var a abbrev
		a.tag = Tag(b.uint())
		a.children = b.uint8() != 0
		a.field = make([]afield, n)
		for i := range a.field {
			a.field[i].attr = Attr(b.uint())
			a.field[i].fmt = format(b.uint())
			a.field[i].class = formToClass(a.field[i].fmt, a.field[i].attr, vers, &b)
			if a.field[i].fmt == formImplicitConst {
				a.field[i].val = b.int()
			}
		}
		b.uint()
		b.uint()

		m[id] = a
	}
	if b.err != nil {
		return nil, b.err
	}
	d.abbrevCache[off] = m
	return m, nil
}

// attrIsExprloc indicates attributes that allow exprloc values that
// are encoded as block values in DWARF 2 and 3. See DWARF 4, Figure
// 20.
var attrIsExprloc = map[Attr]bool{
	AttrLocation:      true,
	AttrByteSize:      true,
	AttrBitOffset:     true,
	AttrBitSize:       true,
	AttrStringLength:  true,
	AttrLowerBound:    true,
	AttrReturnAddr:    true,
	AttrStrideSize:    true,
	AttrUpperBound:    true,
	AttrCount:         true,
	AttrDataMemberLoc: true,
	AttrFrameBase:     true,
	AttrSegment:       true,
	AttrStaticLink:    true,
	AttrUseLocation:   true,
	AttrVtableElemLoc: true,
	AttrAllocated:     true,
	AttrAssociated:    true,
	AttrDataLocation:  true,
	AttrStride:        true,
}

// attrPtrClass indicates the *ptr class of attributes that have
// encoding formSecOffset in DWARF 4 or formData* in DWARF 2 and 3.
var attrPtrClass = map[Attr]Class{
	AttrLocation:      ClassLocListPtr,
	AttrStmtList:      ClassLinePtr,
	AttrStringLength:  ClassLocListPtr,
	AttrReturnAddr:    ClassLocListPtr,
	AttrStartScope:    ClassRangeListPtr,
	AttrDataMemberLoc: ClassLocListPtr,
	AttrFrameBase:     ClassLocListPtr,
	AttrMacroInfo:     ClassMacPtr,
	AttrSegment:       ClassLocListPtr,
	AttrStaticLink:    ClassLocListPtr,
	AttrUseLocation:   ClassLocListPtr,
	AttrVtableElemLoc: ClassLocListPtr,
	AttrRanges:        ClassRangeListPtr,
	// The following are new in DWARF 5.
	AttrStrOffsetsBase: ClassStrOffsetsPtr,
	AttrAddrBase:       ClassAddrPtr,
	AttrRnglistsBase:   ClassRngListsPtr,
	AttrLoclistsBase:   ClassLocListPtr,
}

// formToClass returns the DWARF 4 Class for the given form. If the
// DWARF version is less then 4, it will disambiguate some forms
// depending on the attribute.
func formToClass(form format, attr Attr, vers int, b *buf) Class {
	switch form {
	default:
		b.error("cannot determine class of unknown attribute form")
		return 0

	case formIndirect:
		return ClassUnknown

	case formAddr, formAddrx, formAddrx1, formAddrx2, formAddrx3, formAddrx4:
		return ClassAddress

	case formDwarfBlock1, formDwarfBlock2, formDwarfBlock4, formDwarfBlock:
		// In DWARF 2 and 3, ClassExprLoc was encoded as a
		// block. DWARF 4 distinguishes ClassBlock and
		// ClassExprLoc, but there are no attributes that can
		// be both, so we also promote ClassBlock values in
		// DWARF 4 that should be ClassExprLoc in case
		// producers get this wrong.
		if attrIsExprloc[attr] {
			return ClassExprLoc
		}
		return ClassBlock

	case formData1, formData2, formData4, formData8, formSdata, formUdata, formData16, formImplicitConst:
		// In DWARF 2 and 3, ClassPtr was encoded as a
		// constant. Unlike ClassExprLoc/ClassBlock, some
		// DWARF 4 attributes need to distinguish Class*Ptr
		// from ClassConstant, so we only do this promotion
		// for versions 2 and 3.
		if class, ok := attrPtrClass[attr]; vers < 4 && ok {
			return class
		}
		return ClassConstant

	case formFlag, formFlagPresent:
		return ClassFlag

	case formRefAddr, formRef1, formRef2, formRef4, formRef8, formRefUdata, formRefSup4, formRefSup8:
		return ClassReference

	case formRefSig8:
		return ClassReferenceSig

	case formString, formStrp, formStrx, formStrpSup, formLineStrp, formStrx1, formStrx2, formStrx3, formStrx4:
		return ClassString

	case formSecOffset:
		// DWARF 4 defines four *ptr classes, but doesn't
		// distinguish them in the encoding. Disambiguate
		// these classes using the attribute.
		if class, ok := attrPtrClass[attr]; ok {
			return class
		}
		return ClassUnknown

	case formExprloc:
		return ClassExprLoc

	case formGnuRefAlt:
		return ClassReferenceAlt

	case formGnuStrpAlt:
		return ClassStringAlt

	case formLoclistx:
		return ClassLocList

	case formRnglistx:
		return ClassRngList
	}
}

// An Entry is a sequence of attribute/value pairs.
type Entry struct {
	Offset   Offset // offset of Entry in DWARF info
	Tag      Tag    // tag (kind of Entry)
	Children bool   // whether Entry is followed by children
	Field    []Field
}

// A Field is a single attribute/value pair in an [Entry].
//
// A value can be one of several "attribute classes" defined by DWARF.
// The Go types corresponding to each class are:
//
//	DWARF class       Go type        Class
//	-----------       -------        -----
//	address           uint64         ClassAddress
//	block             []byte         ClassBlock
//	constant          int64          ClassConstant
//	flag              bool           ClassFlag
//	reference
//	  to info         dwarf.Offset   ClassReference
//	  to type unit    uint64         ClassReferenceSig
//	string            string         ClassString
//	exprloc           []byte         ClassExprLoc
//	lineptr           int64          ClassLinePtr
//	loclistptr        int64          ClassLocListPtr
//	macptr            int64          ClassMacPtr
//	rangelistptr      int64          ClassRangeListPtr
//
// For unrecognized or vendor-defined attributes, [Class] may be
// [ClassUnknown].
type Field struct {
	Attr  Attr
	Val   any
	Class Class
}

// A Class is the DWARF 4 class of an attribute value.
//
// In general, a given attribute's value may take on one of several
// possible classes defined by DWARF, each of which leads to a
// slightly different interpretation of the attribute.
//
// DWARF version 4 distinguishes attribute value classes more finely
// than previous versions of DWARF. The reader will disambiguate
// coarser classes from earlier versions of DWARF into the appropriate
// DWARF 4 class. For example, DWARF 2 uses "constant" for constants
// as well as all types of section offsets, but the reader will
// canonicalize attributes in DWARF 2 files that refer to section
// offsets to one of the Class*Ptr classes, even though these classes
// were only defined in DWARF 3.
type Class int

const (
	// ClassUnknown represents values of unknown DWARF class.
	ClassUnknown Class = iota

	// ClassAddress represents values of type uint64 that are
	// addresses on the target machine.
	ClassAddress

	// ClassBlock represents values of type []byte whose
	// interpretation depends on the attribute.
	ClassBlock

	// ClassConstant represents values of type int64 that are
	// constants. The interpretation of this constant depends on
	// the attribute.
	ClassConstant

	// ClassExprLoc represents values of type []byte that contain
	// an encoded DWARF expression or location description.
	ClassExprLoc

	// ClassFlag represents values of type bool.
	ClassFlag

	// ClassLinePtr represents values that are an int64 offset
	// into the "line" section.
	ClassLinePtr

	// ClassLocListPtr represents values that are an int64 offset
	// into the "loclist" section.
	ClassLocListPtr

	// ClassMacPtr represents values that are an int64 offset into
	// the "mac" section.
	ClassMacPtr

	// ClassRangeListPtr represents values that are an int64 offset into
	// the "rangelist" section.
	ClassRangeListPtr

	// ClassReference represents values that are an Offset offset
	// of an Entry in the info section (for use with Reader.Seek).
	// The DWARF specification combines ClassReference and
	// ClassReferenceSig into class "reference".
	ClassReference

	// ClassReferenceSig represents values that are a uint64 type
	// signature referencing a type Entry.
	ClassReferenceSig

	// ClassString represents values that are strings. If the
	// compilation unit specifies the AttrUseUTF8 flag (strongly
	// recommended), the string value will be encoded in UTF-8.
	// Otherwise, the encoding is unspecified.
	ClassString

	// ClassReferenceAlt represents values of type int64 that are
	// an offset into the DWARF "info" section of an alternate
	// object file.
	ClassReferenceAlt

	// ClassStringAlt represents values of type int64 that are an
	// offset into the DWARF string section of an alternate object
	// file.
	ClassStringAlt

	// ClassAddrPtr represents values that are an int64 offset
	// into the "addr" section.
	ClassAddrPtr

	// ClassLocList represents values that are an int64 offset
	// into the "loclists" section.
	ClassLocList

	// ClassRngList represents values that are a uint64 offset
	// from the base of the "rnglists" section.
	ClassRngList

	// ClassRngListsPtr represents values that are an int64 offset
	// into the "rnglists" section. These are used as the base for
	// ClassRngList values.
	ClassRngListsPtr

	// ClassStrOffsetsPtr represents values that are an int64
	// offset into the "str_offsets" section.
	ClassStrOffsetsPtr
)

//go:generate stringer -type=Class

func (i Class) GoString() string {
	return "dwarf." + i.String()
}

// Val returns the value associated with attribute [Attr] in [Entry],
// or nil if there is no such attribute.
//
// A common idiom is to merge the check for nil return with
// the check that the value has the expected dynamic type, as in:
//
//	v, ok := e.Val(AttrSibling).(int64)
func (e *Entry) Val(a Attr) any {
	if f := e.AttrField(a); f != nil {
		return f.Val
	}
	return nil
}

// AttrField returns the [Field] associated with attribute [Attr] in
// [Entry], or nil if there is no such attribute.
func (e *Entry) AttrField(a Attr) *Field {
	for i, f := range e.Field {
		if f.Attr == a {
			return &e.Field[i]
		}
	}
	return nil
}

// An Offset represents the location of an [Entry] within the DWARF info.
// (See [Reader.Seek].)
type Offset uint32

// Entry reads a single entry from buf, decoding
// according to the given abbreviation table.
func (b *buf) entry(cu *Entry, u *unit) *Entry {
	atab, ubase, vers := u.atable, u.base, u.vers
	off := b.off
	id := uint32(b.uint())
	if id == 0 {
		return &Entry{}
	}
	a, ok := atab[id]
	if !ok {
		b.error("unknown abbreviation table index")
		return nil
	}
	e := &Entry{
		Offset:   off,
		Tag:      a.tag,
		Children: a.children,
		Field:    make([]Field, len(a.field)),
	}

	resolveStrx := func(strBase, off uint64) string {
		off += strBase
		if uint64(int(off)) != off {
			b.error("DW_FORM_strx offset out of range")
		}

		b1 := makeBuf(b.dwarf, b.format, "str_offsets", 0, b.dwarf.strOffsets)
		b1.skip(int(off))
		is64, _ := b.format.dwarf64()
		if is64 {
			off = b1.uint64()
		} else {
			off = uint64(b1.uint32())
		}
		if b1.err != nil {
			b.err = b1.err
			return ""
		}
		if uint64(int(off)) != off {
			b.error("DW_FORM_strx indirect offset out of range")
		}
		b1 = makeBuf(b.dwarf, b.format, "str", 0, b.dwarf.str)
		b1.skip(int(off))
		val := b1.string()
		if b1.err != nil {
			b.err = b1.err
		}
		return val
	}

	resolveRnglistx := func(rnglistsBase, off uint64) uint64 {
		is64, _ := b.format.dwarf64()
		if is64 {
			off *= 8
		} else {
			off *= 4
		}
		off += rnglistsBase
		if uint64(int(off)) != off {
			b.error("DW_FORM_rnglistx offset out of range")
		}

		b1 := makeBuf(b.dwarf, b.format, "rnglists", 0, b.dwarf.rngLists)
		b1.skip(int(off))
		if is64 {
			off = b1.uint64()
		} else {
			off = uint64(b1.uint32())
		}
		if b1.err != nil {
			b.err = b1.err
			return 0
		}
		if uint64(int(off)) != off {
			b.error("DW_FORM_rnglistx indirect offset out of range")
		}
		return rnglistsBase + off
	}

	for i := range e.Field {
		e.Field[i].Attr = a.field[i].attr
		e.Field[i].Class = a.field[i].class
		fmt := a.field[i].fmt
		if fmt == formIndirect {
			fmt = format(b.uint())
			e.Field[i].Class = formToClass(fmt, a.field[i].attr, vers, b)
		}
		var val any
		switch fmt {
		default:
			b.error("unknown entry attr format 0x" + strconv.FormatInt(int64(fmt), 16))

		// address
		case formAddr:
			val = b.addr()
		case formAddrx, formAddrx1, formAddrx2, formAddrx3, formAddrx4:
			var off uint64
			switch fmt {
			case formAddrx:
				off = b.uint()
			case formAddrx1:
				off = uint64(b.uint8())
			case formAddrx2:
				off = uint64(b.uint16())
			case formAddrx3:
				off = uint64(b.uint24())
			case formAddrx4:
				off = uint64(b.uint32())
			}
			if b.dwarf.addr == nil {
				b.error("DW_FORM_addrx with no .debug_addr section")
			}
			if b.err != nil {
				return nil
			}

			addrBase := int64(u.addrBase())
			var err error
			val, err = b.dwarf.debugAddr(b.format, uint64(addrBase), off)
			if err != nil {
				if b.err == nil {
					b.err = err
				}
				return nil
			}

		// block
		case formDwarfBlock1:
			val = b.bytes(int(b.uint8()))
		case formDwarfBlock2:
			val = b.bytes(int(b.uint16()))
		case formDwarfBlock4:
			val = b.bytes(int(b.uint32()))
		case formDwarfBlock:
			val = b.bytes(int(b.uint()))

		// constant
		case formData1:
			val = int64(b.uint8())
		case formData2:
			val = int64(b.uint16())
		case formData4:
			val = int64(b.uint32())
		case formData8:
			val = int64(b.uint64())
		case formData16:
			val = b.bytes(16)
		case formSdata:
			val = b.int()
		case formUdata:
			val = int64(b.uint())
		case formImplicitConst:
			val = a.field[i].val

		// flag
		case formFlag:
			val = b.uint8() == 1
		// New in DWARF 4.
		case formFlagPresent:
			// The attribute is implicitly indicated as present, and no value is
			// encoded in the debugging information entry itself.
			val = true

		// reference to other entry
		case formRefAddr:
			vers := b.format.version()
			if vers == 0 {
				b.error("unknown version for DW_FORM_ref_addr")
			} else if vers == 2 {
				val = Offset(b.addr())
			} else {
				is64, known := b.format.dwarf64()
				if !known {
					b.error("unknown size for DW_FORM_ref_addr")
				} else if is64 {
					val = Offset(b.uint64())
				} else {
					val = Offset(b.uint32())
				}
			}
		case formRef1:
			val = Offset(b.uint8()) + ubase
		case formRef2:
			val = Offset(b.uint16()) + ubase
		case formRef4:
			val = Offset(b.uint32()) + ubase
		case formRef8:
			val = Offset(b.uint64()) + ubase
		case formRefUdata:
			val = Offset(b.uint()) + ubase

		// string
		case formString:
			val = b.string()
		case formStrp, formLineStrp:
			var off uint64 // offset into .debug_str
			is64, known := b.format.dwarf64()
			if !known {
				b.error("unknown size for DW_FORM_strp/line_strp")
			} else if is64 {
				off = b.uint64()
			} else {
				off = uint64(b.uint32())
			}
			if uint64(int(off)) != off {
				b.error("DW_FORM_strp/line_strp offset out of range")
			}
			if b.err != nil {
				return nil
			}
			var b1 buf
			if fmt == formStrp {
				b1 = makeBuf(b.dwarf, b.format, "str", 0, b.dwarf.str)
			} else {
				if len(b.dwarf.lineStr) == 0 {
					b.error("DW_FORM_line_strp with no .debug_line_str section")
					return nil
				}
				b1 = makeBuf(b.dwarf, b.format, "line_str", 0, b.dwarf.lineStr)
			}
			b1.skip(int(off))
			val = b1.string()
			if b1.err != nil {
				b.err = b1.err
				return nil
			}
		case formStrx, formStrx1, formStrx2, formStrx3, formStrx4:
			var off uint64
			switch fmt {
			case formStrx:
				off = b.uint()
			case formStrx1:
				off = uint64(b.uint8())
			case formStrx2:
				off = uint64(b.uint16())
			case formStrx3:
				off = uint64(b.uint24())
			case formStrx4:
				off = uint64(b.uint32())
			}
			if len(b.dwarf.strOffsets) == 0 {
				b.error("DW_FORM_strx with no .debug_str_offsets section")
			}
			is64, known := b.format.dwarf64()
			if !known {
				b.error("unknown offset size for DW_FORM_strx")
			}
			if b.err != nil {
				return nil
			}
			if is64 {
				off *= 8
			} else {
				off *= 4
			}

			strBase := int64(u.strOffsetsBase())
			val = resolveStrx(uint64(strBase), off)

		case formStrpSup:
			is64, known := b.format.dwarf64()
			if !known {
				b.error("unknown size for DW_FORM_strp_sup")
			} else if is64 {
				val = b.uint64()
			} else {
				val = b.uint32()
			}

		// lineptr, loclistptr, macptr, rangelistptr
		// New in DWARF 4, but clang can generate them with -gdwarf-2.
		// Section reference, replacing use of formData4 and formData8.
		case formSecOffset, formGnuRefAlt, formGnuStrpAlt:
			is64, known := b.format.dwarf64()
			if !known {
				b.error("unknown size for form 0x" + strconv.FormatInt(int64(fmt), 16))
			} else if is64 {
				val = int64(b.uint64())
			} else {
				val = int64(b.uint32())
			}

		// exprloc
		// New in DWARF 4.
		case formExprloc:
			val = b.bytes(int(b.uint()))

		// reference
		// New in DWARF 4.
		case formRefSig8:
			// 64-bit type signature.
			val = b.uint64()
		case formRefSup4:
			val = b.uint32()
		case formRefSup8:
			val = b.uint64()

		// loclist
		case formLoclistx:
			val = b.uint()

		// rnglist
		case formRnglistx:
			off := b.uint()

			rnglistsBase := int64(u.rngListsBase())
			val = resolveRnglistx(uint64(rnglistsBase), off)
		}

		e.Field[i].Val = val
	}
	if b.err != nil {
		return nil
	}
	return e
}

// A Reader allows reading [Entry] structures from a DWARF “info” section.
// The [Entry] structures are arranged in a tree. The [Reader.Next] function
// return successive entries from a pre-order traversal of the tree.
// If an entry has children, its Children field will be true, and the children
// follow, terminated by an [Entry] with [Tag] 0.
type Reader struct {
	b            buf
	d            *Data
	err          error
	unit         int
	lastUnit     bool   // set if last entry returned by Next is TagCompileUnit/TagPartialUnit
	lastChildren bool   // .Children of last entry returned by Next
	lastSibling  Offset // .Val(AttrSibling) of last entry returned by Next
	cu           *Entry // current compilation unit
}

// Reader returns a new Reader for [Data].
// The reader is positioned at byte offset 0 in the DWARF “info” section.
func (d *Data) Reader() *Reader {
	r := &Reader{d: d}
	r.Seek(0)
	return r
}

// AddressSize returns the size in bytes of addresses in the current compilation
// unit.
func (r *Reader) AddressSize() int {
	return r.d.unit[r.unit].asize
}

// ByteOrder returns the byte order in the current compilation unit.
func (r *Reader) ByteOrder() binary.ByteOrder {
	return r.b.order
}

// Seek positions the [Reader] at offset off in the encoded entry stream.
// Offset 0 can be used to denote the first entry.
func (r *Reader) Seek(off Offset) {
	d := r.d
	r.err = nil
	r.lastChildren = false
	if off == 0 {
		if len(d.unit) == 0 {
			return
		}
		u := &d.unit[0]
		r.unit = 0
		r.b = makeBuf(r.d, u, "info", u.off, u.data)
		r.collectDwarf5BaseOffsets(u)
		r.cu = nil
		return
	}

	i := d.offsetToUnit(off)
	if i == -1 {
		r.err = errors.New("offset out of range")
		return
	}
	if i != r.unit {
		r.cu = nil
	}
	u := &d.unit[i]
	r.unit = i
	r.b = makeBuf(r.d, u, "info", off, u.data[off-u.off:])
	r.collectDwarf5BaseOffsets(u)
}

// maybeNextUnit advances to the next unit if this one is finished.
func (r *Reader) maybeNextUnit() {
	for len(r.b.data) == 0 && r.unit+1 < len(r.d.unit) {
		r.nextUnit()
	}
}

// nextUnit advances to the next unit.
func (r *Reader) nextUnit() {
	r.unit++
	u := &r.d.unit[r.unit]
	r.b = makeBuf(r.d, u, "info", u.off, u.data)
	r.cu = nil
	r.collectDwarf5BaseOffsets(u)
}

func (r *Reader) collectDwarf5BaseOffsets(u *unit) {
	if u.vers < 5 || u.unit5 != nil {
		return
	}
	u.unit5 = new(unit5)
	if err := r.d.collectDwarf5BaseOffsets(u); err != nil {
		r.err = err
	}
}

// Next reads the next entry from the encoded entry stream.
// It returns nil, nil when it reaches the end of the section.
// It returns an error if the current offset is invalid or the data at the
// offset cannot be decoded as a valid [Entry].
func (r *Reader) Next() (*Entry, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.maybeNextUnit()
	if len(r.b.data) == 0 {
		return nil, nil
	}
	u := &r.d.unit[r.unit]
	e := r.b.entry(r.cu, u)
	if r.b.err != nil {
		r.err = r.b.err
		return nil, r.err
	}
	r.lastUnit = false
	if e != nil {
		r.lastChildren = e.Children
		if r.lastChildren {
			r.lastSibling, _ = e.Val(AttrSibling).(Offset)
		}
		if e.Tag == TagCompileUnit || e.Tag == TagPartialUnit {
			r.lastUnit = true
			r.cu = e
		}
	} else {
		r.lastChildren = false
	}
	return e, nil
}

// SkipChildren skips over the child entries associated with
// the last [Entry] returned by [Reader.Next]. If that [Entry] did not have
// children or [Reader.Next] has not been called, SkipChildren is a no-op.
func (r *Reader) SkipChildren() {
	if r.err != nil || !r.lastChildren {
		return
	}

	// If the last entry had a sibling attribute,
	// that attribute gives the offset of the next
	// sibling, so we can avoid decoding the
	// child subtrees.
	if r.lastSibling >= r.b.off {
		r.Seek(r.lastSibling)
		return
	}

	if r.lastUnit && r.unit+1 < len(r.d.unit) {
		r.nextUnit()
		return
	}

	for {
		e, err := r.Next()
		if err != nil || e == nil || e.Tag == 0 {
			break
		}
		if e.Children {
			r.SkipChildren()
		}
	}
}

// clone returns a copy of the reader. This is used by the typeReader
// interface.
func (r *Reader) clone() typeReader {
	return r.d.Reader()
}

// offset returns the current buffer offset. This is used by the
// typeReader interface.
func (r *Reader) offset() Offset {
	return r.b.off
}

// SeekPC returns the [Entry] for the compilation unit that includes pc,
// and positions the reader to read the children of that unit.  If pc
// is not covered by any unit, SeekPC returns [ErrUnknownPC] and the
// position of the reader is undefined.
//
// Because compilation units can describe multiple regions of the
// executable, in the worst case SeekPC must search through all the
// ranges in all the compilation units. Each call to SeekPC starts the
// search at the compilation unit of the last call, so in general
// looking up a series of PCs will be faster if they are sorted. If
// the caller wishes to do repeated fast PC lookups, it should build
// an appropriate index using the Ranges method.
func (r *Reader) SeekPC(pc uint64) (*Entry, error) {
	unit := r.unit
	for i := 0; i < len(r.d.unit); i++ {
		if unit >= len(r.d.unit) {
			unit = 0
		}
		r.err = nil
		r.lastChildren = false
		r.unit = unit
		r.cu = nil
		u := &r.d.unit[unit]
		r.b = makeBuf(r.d, u, "info", u.off, u.data)
		r.collectDwarf5BaseOffsets(u)
		e, err := r.Next()
		if err != nil {
			return nil, err
		}
		if e == nil || e.Tag == 0 {
			return nil, ErrUnknownPC
		}
		ranges, err := r.d.Ranges(e)
		if err != nil {
			return nil, err
		}
		for _, pcs := range ranges {
			if pcs[0] <= pc && pc < pcs[1] {
				return e, nil
			}
		}
		unit++
	}
	return nil, ErrUnknownPC
}

// Ranges returns the PC ranges covered by e, a slice of [low,high) pairs.
// Only some entry types, such as [TagCompileUnit] or [TagSubprogram], have PC
// ranges; for others, this will return nil with no error.
func (d *Data) Ranges(e *Entry) ([][2]uint64, error) {
	var ret [][2]uint64

	low, lowOK := e.Val(AttrLowpc).(uint64)

	var high uint64
	var highOK bool
	highField := e.AttrField(AttrHighpc)
	if highField != nil {
		switch highField.Class {
		case ClassAddress:
			high, highOK = highField.Val.(uint64)
		case ClassConstant:
			off, ok := highField.Val.(int64)
			if ok {
				high = low + uint64(off)
				highOK = true
			}
		}
	}

	if lowOK && highOK {
		ret = append(ret, [2]uint64{low, high})
	}

	var u *unit
	if uidx := d.offsetToUnit(e.Offset); uidx >= 0 && uidx < len(d.unit) {
		u = &d.unit[uidx]
	}

	if u != nil && u.vers >= 5 && d.rngLists != nil {
		// DWARF version 5 and later
		field := e.AttrField(AttrRanges)
		if field == nil {
			return ret, nil
		}
		switch field.Class {
		case ClassRangeListPtr:
			ranges, rangesOK := field.Val.(int64)
			if !rangesOK {
				return ret, nil
			}
			cu, base, err := d.baseAddressForEntry(e)
			if err != nil {
				return nil, err
			}
			return d.dwarf5Ranges(u, cu, base, ranges, ret)

		case ClassRngList:
			rnglist, ok := field.Val.(uint64)
			if !ok {
				return ret, nil
			}
			cu, base, err := d.baseAddressForEntry(e)
			if err != nil {
				return nil, err
			}
			return d.dwarf5Ranges(u, cu, base, int64(rnglist), ret)

		default:
			return ret, nil
		}
	}

	// DWARF version 2 through 4
	ranges, rangesOK := e.Val(AttrRanges).(int64)
	if rangesOK && d.ranges != nil {
		_, base, err := d.baseAddressForEntry(e)
		if err != nil {
			return nil, err
		}
		return d.dwarf2Ranges(u, base, ranges, ret)
	}

	return ret, nil
}

// baseAddressForEntry returns the initial base address to be used when
// looking up the range list of entry e.
// DWARF specifies that this should be the lowpc attribute of the enclosing
// compilation unit, however comments in gdb/dwarf2read.c say that some
// versions of GCC use the entrypc attribute, so we check that too.
func (d *Data) baseAddressForEntry(e *Entry) (*Entry, uint64, error) {
	var cu *Entry
	if e.Tag == TagCompileUnit {
		cu = e
	} else {
		i := d.offsetToUnit(e.Offset)
		if i == -1 {
			return nil, 0, errors.New("no unit for entry")
		}
		u := &d.unit[i]
		b := makeBuf(d, u, "info", u.off, u.data)
		cu = b.entry(nil, u)
		if b.err != nil {
			return nil, 0, b.err
		}
	}

	if cuEntry, cuEntryOK := cu.Val(AttrEntrypc).(uint64); cuEntryOK {
		return cu, cuEntry, nil
	} else if cuLow, cuLowOK := cu.Val(AttrLowpc).(uint64); cuLowOK {
		return cu, cuLow, nil
	}

	return cu, 0, nil
}

func (d *Data) dwarf2Ranges(u *unit, base uint64, ranges int64, ret [][2]uint64) ([][2]uint64, error) {
	if ranges < 0 || ranges > int64(len(d.ranges)) {
		return nil, fmt.Errorf("invalid range offset %d (max %d)", ranges, len(d.ranges))
	}
	buf := makeBuf(d, u, "ranges", Offset(ranges), d.ranges[ranges:])
	for len(buf.data) > 0 {
		low := buf.addr()
		high := buf.addr()

		if low == 0 && high == 0 {
			break
		}

		if low == ^uint64(0)>>uint((8-u.addrsize())*8) {
			base = high
		} else {
			ret = append(ret, [2]uint64{base + low, base + high})
		}
	}

	return ret, nil
}

// dwarf5Ranges interprets a debug_rnglists sequence, see DWARFv5 section
// 2.17.3 (page 53).
func (d *Data) dwarf5Ranges(u *unit, cu *Entry, base uint64, ranges int64, ret [][2]uint64) ([][2]uint64, error) {
	if ranges < 0 || ranges > int64(len(d.rngLists)) {
		return nil, fmt.Errorf("invalid rnglist offset %d (max %d)", ranges, len(d.ranges))
	}
	var addrBase int64
	if cu != nil {
		addrBase, _ = cu.Val(AttrAddrBase).(int64)
	}

	buf := makeBuf(d, u, "rnglists", 0, d.rngLists)
	buf.skip(int(ranges))
	for {
		opcode := buf.uint8()
		switch opcode {
		case rleEndOfList:
			if buf.err != nil {
				return nil, buf.err
			}
			return ret, nil

		case rleBaseAddressx:
			baseIdx := buf.uint()
			var err error
			base, err = d.debugAddr(u, uint64(addrBase), baseIdx)
			if err != nil {
				return nil, err
			}

		case rleStartxEndx:
			startIdx := buf.uint()
			endIdx := buf.uint()

			start, err := d.debugAddr(u, uint64(addrBase), startIdx)
			if err != nil {
				return nil, err
			}
			end, err := d.debugAddr(u, uint64(addrBase), endIdx)
			if err != nil {
				return nil, err
			}
			ret = append(ret, [2]uint64{start, end})

		case rleStartxLength:
			startIdx := buf.uint()
			len := buf.uint()
			start, err := d.debugAddr(u, uint64(addrBase), startIdx)
			if err != nil {
				return nil, err
			}
			ret = append(ret, [2]uint64{start, start + len})

		case rleOffsetPair:
			off1 := buf.uint()
			off2 := buf.uint()
			ret = append(ret, [2]uint64{base + off1, base + off2})

		case rleBaseAddress:
			base = buf.addr()

		case rleStartEnd:
			start := buf.addr()
			end := buf.addr()
			ret = append(ret, [2]uint64{start, end})

		case rleStartLength:
			start := buf.addr()
			len := buf.uint()
			ret = append(ret, [2]uint64{start, start + len})
		}
	}
}

// debugAddr returns the address at idx in debug_addr
func (d *Data) debugAddr(format dataFormat, addrBase, idx uint64) (uint64, error) {
	off := idx*uint64(format.addrsize()) + addrBase

	if uint64(int(off)) != off {
		return 0, errors.New("offset out of range")
	}

	b := makeBuf(d, format, "addr", 0, d.addr)
	b.skip(int(off))
	val := b.addr()
	if b.err != nil {
		return 0, b.err
	}
	return val, nil
}

```

// === FILE: references!/go/src/debug/dwarf/line.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dwarf

import (
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// A LineReader reads a sequence of [LineEntry] structures from a DWARF
// "line" section for a single compilation unit. LineEntries occur in
// order of increasing PC and each [LineEntry] gives metadata for the
// instructions from that [LineEntry]'s PC to just before the next
// [LineEntry]'s PC. The last entry will have the [LineEntry.EndSequence] field set.
type LineReader struct {
	buf buf

	// Original .debug_line section data. Used by Seek.
	section []byte

	str     []byte // .debug_str
	lineStr []byte // .debug_line_str

	// Header information
	version              uint16
	addrsize             int
	segmentSelectorSize  int
	minInstructionLength int
	maxOpsPerInstruction int
	defaultIsStmt        bool
	lineBase             int
	lineRange            int
	opcodeBase           int
	opcodeLengths        []int
	directories          []string
	fileEntries          []*LineFile

	programOffset Offset // section offset of line number program
	endOffset     Offset // section offset of byte following program

	initialFileEntries int // initial length of fileEntries

	// Current line number program state machine registers
	state     LineEntry // public state
	fileIndex int       // private state
}

// A LineEntry is a row in a DWARF line table.
type LineEntry struct {
	// Address is the program-counter value of a machine
	// instruction generated by the compiler. This LineEntry
	// applies to each instruction from Address to just before the
	// Address of the next LineEntry.
	Address uint64

	// OpIndex is the index of an operation within a VLIW
	// instruction. The index of the first operation is 0. For
	// non-VLIW architectures, it will always be 0. Address and
	// OpIndex together form an operation pointer that can
	// reference any individual operation within the instruction
	// stream.
	OpIndex int

	// File is the source file corresponding to these
	// instructions.
	File *LineFile

	// Line is the source code line number corresponding to these
	// instructions. Lines are numbered beginning at 1. It may be
	// 0 if these instructions cannot be attributed to any source
	// line.
	Line int

	// Column is the column number within the source line of these
	// instructions. Columns are numbered beginning at 1. It may
	// be 0 to indicate the "left edge" of the line.
	Column int

	// IsStmt indicates that Address is a recommended breakpoint
	// location, such as the beginning of a line, statement, or a
	// distinct subpart of a statement.
	IsStmt bool

	// BasicBlock indicates that Address is the beginning of a
	// basic block.
	BasicBlock bool

	// PrologueEnd indicates that Address is one (of possibly
	// many) PCs where execution should be suspended for a
	// breakpoint on entry to the containing function.
	//
	// Added in DWARF 3.
	PrologueEnd bool

	// EpilogueBegin indicates that Address is one (of possibly
	// many) PCs where execution should be suspended for a
	// breakpoint on exit from this function.
	//
	// Added in DWARF 3.
	EpilogueBegin bool

	// ISA is the instruction set architecture for these
	// instructions. Possible ISA values should be defined by the
	// applicable ABI specification.
	//
	// Added in DWARF 3.
	ISA int

	// Discriminator is an arbitrary integer indicating the block
	// to which these instructions belong. It serves to
	// distinguish among multiple blocks that may all have with
	// the same source file, line, and column. Where only one
	// block exists for a given source position, it should be 0.
	//
	// Added in DWARF 3.
	Discriminator int

	// EndSequence indicates that Address is the first byte after
	// the end of a sequence of target machine instructions. If it
	// is set, only this and the Address field are meaningful. A
	// line number table may contain information for multiple
	// potentially disjoint instruction sequences. The last entry
	// in a line table should always have EndSequence set.
	EndSequence bool
}

// A LineFile is a source file referenced by a DWARF line table entry.
type LineFile struct {
	Name   string
	Mtime  uint64 // Implementation defined modification time, or 0 if unknown
	Length int    // File length, or 0 if unknown
}

// LineReader returns a new reader for the line table of compilation
// unit cu, which must be an [Entry] with tag [TagCompileUnit].
//
// If this compilation unit has no line table, it returns nil, nil.
func (d *Data) LineReader(cu *Entry) (*LineReader, error) {
	if d.line == nil {
		// No line tables available.
		return nil, nil
	}

	// Get line table information from cu.
	off, ok := cu.Val(AttrStmtList).(int64)
	if !ok {
		// cu has no line table.
		return nil, nil
	}
	if off < 0 || off > int64(len(d.line)) {
		return nil, errors.New("AttrStmtList value out of range")
	}
	// AttrCompDir is optional if all file names are absolute. Use
	// the empty string if it's not present.
	compDir, _ := cu.Val(AttrCompDir).(string)

	// Create the LineReader.
	u := &d.unit[d.offsetToUnit(cu.Offset)]
	buf := makeBuf(d, u, "line", Offset(off), d.line[off:])
	// The compilation directory is implicitly directories[0].
	r := LineReader{
		buf:     buf,
		section: d.line,
		str:     d.str,
		lineStr: d.lineStr,
	}

	// Read the header.
	if err := r.readHeader(compDir); err != nil {
		return nil, err
	}

	// Initialize line reader state.
	r.Reset()

	return &r, nil
}

// readHeader reads the line number program header from r.buf and sets
// all of the header fields in r.
func (r *LineReader) readHeader(compDir string) error {
	buf := &r.buf

	// Read basic header fields [DWARF2 6.2.4].
	hdrOffset := buf.off
	unitLength, dwarf64 := buf.unitLength()
	r.endOffset = buf.off + unitLength
	if r.endOffset > buf.off+Offset(len(buf.data)) {
		return DecodeError{"line", hdrOffset, fmt.Sprintf("line table end %d exceeds section size %d", r.endOffset, buf.off+Offset(len(buf.data)))}
	}
	r.version = buf.uint16()
	if buf.err == nil && (r.version < 2 || r.version > 5) {
		// DWARF goes to all this effort to make new opcodes
		// backward-compatible, and then adds fields right in
		// the middle of the header in new versions, so we're
		// picky about only supporting known line table
		// versions.
		return DecodeError{"line", hdrOffset, fmt.Sprintf("unknown line table version %d", r.version)}
	}
	if r.version >= 5 {
		r.addrsize = int(buf.uint8())
		r.segmentSelectorSize = int(buf.uint8())
	} else {
		r.addrsize = buf.format.addrsize()
		r.segmentSelectorSize = 0
	}
	var headerLength Offset
	if dwarf64 {
		headerLength = Offset(buf.uint64())
	} else {
		headerLength = Offset(buf.uint32())
	}
	programOffset := buf.off + headerLength
	if programOffset > r.endOffset {
		return DecodeError{"line", hdrOffset, fmt.Sprintf("malformed line table: program offset %d exceeds end offset %d", programOffset, r.endOffset)}
	}
	r.programOffset = programOffset
	r.minInstructionLength = int(buf.uint8())
	if r.version >= 4 {
		// [DWARF4 6.2.4]
		r.maxOpsPerInstruction = int(buf.uint8())
	} else {
		r.maxOpsPerInstruction = 1
	}
	r.defaultIsStmt = buf.uint8() != 0
	r.lineBase = int(int8(buf.uint8()))
	r.lineRange = int(buf.uint8())

	// Validate header.
	if buf.err != nil {
		return buf.err
	}
	if r.maxOpsPerInstruction == 0 {
		return DecodeError{"line", hdrOffset, "invalid maximum operations per instruction: 0"}
	}
	if r.lineRange == 0 {
		return DecodeError{"line", hdrOffset, "invalid line range: 0"}
	}

	// Read standard opcode length table. This table starts with opcode 1.
	r.opcodeBase = int(buf.uint8())
	r.opcodeLengths = make([]int, r.opcodeBase)
	for i := 1; i < r.opcodeBase; i++ {
		r.opcodeLengths[i] = int(buf.uint8())
	}

	// Validate opcode lengths.
	if buf.err != nil {
		return buf.err
	}
	for i, length := range r.opcodeLengths {
		if known, ok := knownOpcodeLengths[i]; ok && known != length {
			return DecodeError{"line", hdrOffset, fmt.Sprintf("opcode %d expected to have length %d, but has length %d", i, known, length)}
		}
	}

	if r.version < 5 {
		// Read include directories table.
		r.directories = []string{compDir}
		for {
			directory := buf.string()
			if buf.err != nil {
				return buf.err
			}
			if len(directory) == 0 {
				break
			}
			if !pathIsAbs(directory) {
				// Relative paths are implicitly relative to
				// the compilation directory.
				directory = pathJoin(compDir, directory)
			}
			r.directories = append(r.directories, directory)
		}

		// Read file name list. File numbering starts with 1,
		// so leave the first entry nil.
		r.fileEntries = make([]*LineFile, 1)
		for {
			if done, err := r.readFileEntry(); err != nil {
				return err
			} else if done {
				break
			}
		}
	} else {
		dirFormat := r.readLNCTFormat()
		c := buf.uint()
		r.directories = make([]string, c)
		for i := range r.directories {
			dir, _, _, err := r.readLNCT(dirFormat, dwarf64)
			if err != nil {
				return err
			}
			r.directories[i] = dir
		}
		fileFormat := r.readLNCTFormat()
		c = buf.uint()
		r.fileEntries = make([]*LineFile, c)
		for i := range r.fileEntries {
			name, mtime, size, err := r.readLNCT(fileFormat, dwarf64)
			if err != nil {
				return err
			}
			r.fileEntries[i] = &LineFile{name, mtime, int(size)}
		}
	}

	r.initialFileEntries = len(r.fileEntries)

	return buf.err
}

// lnctForm is a pair of an LNCT code and a form. This represents an
// entry in the directory name or file name description in the DWARF 5
// line number program header.
type lnctForm struct {
	lnct int
	form format
}

// readLNCTFormat reads an LNCT format description.
func (r *LineReader) readLNCTFormat() []lnctForm {
	c := r.buf.uint8()
	ret := make([]lnctForm, c)
	for i := range ret {
		ret[i].lnct = int(r.buf.uint())
		ret[i].form = format(r.buf.uint())
	}
	return ret
}

// readLNCT reads a sequence of LNCT entries and returns path information.
func (r *LineReader) readLNCT(s []lnctForm, dwarf64 bool) (path string, mtime uint64, size uint64, err error) {
	var dir string
	for _, lf := range s {
		var str string
		var val uint64
		switch lf.form {
		case formString:
			str = r.buf.string()
		case formStrp, formLineStrp:
			var off uint64
			if dwarf64 {
				off = r.buf.uint64()
			} else {
				off = uint64(r.buf.uint32())
			}
			if uint64(int(off)) != off {
				return "", 0, 0, DecodeError{"line", r.buf.off, "strp/line_strp offset out of range"}
			}
			var b1 buf
			if lf.form == formStrp {
				b1 = makeBuf(r.buf.dwarf, r.buf.format, "str", 0, r.str)
			} else {
				b1 = makeBuf(r.buf.dwarf, r.buf.format, "line_str", 0, r.lineStr)
			}
			b1.skip(int(off))
			str = b1.string()
			if b1.err != nil {
				return "", 0, 0, DecodeError{"line", r.buf.off, b1.err.Error()}
			}
		case formStrpSup:
			// Supplemental sections not yet supported.
			if dwarf64 {
				r.buf.uint64()
			} else {
				r.buf.uint32()
			}
		case formStrx:
			// .debug_line.dwo sections not yet supported.
			r.buf.uint()
		case formStrx1:
			r.buf.uint8()
		case formStrx2:
			r.buf.uint16()
		case formStrx3:
			r.buf.uint24()
		case formStrx4:
			r.buf.uint32()
		case formData1:
			val = uint64(r.buf.uint8())
		case formData2:
			val = uint64(r.buf.uint16())
		case formData4:
			val = uint64(r.buf.uint32())
		case formData8:
			val = r.buf.uint64()
		case formData16:
			r.buf.bytes(16)
		case formDwarfBlock:
			r.buf.bytes(int(r.buf.uint()))
		case formUdata:
			val = r.buf.uint()
		}

		switch lf.lnct {
		case lnctPath:
			path = str
		case lnctDirectoryIndex:
			if val >= uint64(len(r.directories)) {
				return "", 0, 0, DecodeError{"line", r.buf.off, "directory index out of range"}
			}
			dir = r.directories[val]
		case lnctTimestamp:
			mtime = val
		case lnctSize:
			size = val
		case lnctMD5:
			// Ignored.
		}
	}

	if dir != "" && path != "" {
		path = pathJoin(dir, path)
	}

	return path, mtime, size, nil
}

// readFileEntry reads a file entry from either the header or a
// DW_LNE_define_file extended opcode and adds it to r.fileEntries. A
// true return value indicates that there are no more entries to read.
func (r *LineReader) readFileEntry() (bool, error) {
	name := r.buf.string()
	if r.buf.err != nil {
		return false, r.buf.err
	}
	if len(name) == 0 {
		return true, nil
	}
	off := r.buf.off
	dirIndex := int(r.buf.uint())
	if !pathIsAbs(name) {
		if dirIndex >= len(r.directories) {
			return false, DecodeError{"line", off, "directory index too large"}
		}
		name = pathJoin(r.directories[dirIndex], name)
	}
	mtime := r.buf.uint()
	length := int(r.buf.uint())

	// If this is a dynamically added path and the cursor was
	// backed up, we may have already added this entry. Avoid
	// updating existing line table entries in this case. This
	// avoids an allocation and potential racy access to the slice
	// backing store if the user called Files.
	if len(r.fileEntries) < cap(r.fileEntries) {
		fe := r.fileEntries[:len(r.fileEntries)+1]
		if fe[len(fe)-1] != nil {
			// We already processed this addition.
			r.fileEntries = fe
			return false, nil
		}
	}
	r.fileEntries = append(r.fileEntries, &LineFile{name, mtime, length})
	return false, nil
}

// updateFile updates r.state.File after r.fileIndex has
// changed or r.fileEntries has changed.
func (r *LineReader) updateFile() {
	if r.fileIndex < len(r.fileEntries) {
		r.state.File = r.fileEntries[r.fileIndex]
	} else {
		r.state.File = nil
	}
}

// Next sets *entry to the next row in this line table and moves to
// the next row. If there are no more entries and the line table is
// properly terminated, it returns [io.EOF].
//
// Rows are always in order of increasing entry.Address, but
// entry.Line may go forward or backward.
func (r *LineReader) Next(entry *LineEntry) error {
	if r.buf.err != nil {
		return r.buf.err
	}

	// Execute opcodes until we reach an opcode that emits a line
	// table entry.
	for {
		if len(r.buf.data) == 0 {
			return io.EOF
		}
		emit := r.step(entry)
		if r.buf.err != nil {
			return r.buf.err
		}
		if emit {
			return nil
		}
	}
}

// knownOpcodeLengths gives the opcode lengths (in varint arguments)
// of known standard opcodes.
var knownOpcodeLengths = map[int]int{
	lnsCopy:             0,
	lnsAdvancePC:        1,
	lnsAdvanceLine:      1,
	lnsSetFile:          1,
	lnsNegateStmt:       0,
	lnsSetBasicBlock:    0,
	lnsConstAddPC:       0,
	lnsSetPrologueEnd:   0,
	lnsSetEpilogueBegin: 0,
	lnsSetISA:           1,
	// lnsFixedAdvancePC takes a uint8 rather than a varint; it's
	// unclear what length the header is supposed to claim, so
	// ignore it.
}

// step processes the next opcode and updates r.state. If the opcode
// emits a row in the line table, this updates *entry and returns
// true.
func (r *LineReader) step(entry *LineEntry) bool {
	opcode := int(r.buf.uint8())

	if opcode >= r.opcodeBase {
		// Special opcode [DWARF2 6.2.5.1, DWARF4 6.2.5.1]
		adjustedOpcode := opcode - r.opcodeBase
		r.advancePC(adjustedOpcode / r.lineRange)
		lineDelta := r.lineBase + adjustedOpcode%r.lineRange
		r.state.Line += lineDelta
		goto emit
	}

	switch opcode {
	case 0:
		// Extended opcode [DWARF2 6.2.5.3]
		length := Offset(r.buf.uint())
		startOff := r.buf.off
		opcode := r.buf.uint8()

		switch opcode {
		case lneEndSequence:
			r.state.EndSequence = true
			*entry = r.state
			r.resetState()

		case lneSetAddress:
			switch r.addrsize {
			case 1:
				r.state.Address = uint64(r.buf.uint8())
			case 2:
				r.state.Address = uint64(r.buf.uint16())
			case 4:
				r.state.Address = uint64(r.buf.uint32())
			case 8:
				r.state.Address = r.buf.uint64()
			default:
				r.buf.error("unknown address size")
			}

		case lneDefineFile:
			if done, err := r.readFileEntry(); err != nil {
				r.buf.err = err
				return false
			} else if done {
				r.buf.err = DecodeError{"line", startOff, "malformed DW_LNE_define_file operation"}
				return false
			}
			r.updateFile()

		case lneSetDiscriminator:
			// [DWARF4 6.2.5.3]
			r.state.Discriminator = int(r.buf.uint())
		}

		r.buf.skip(int(startOff + length - r.buf.off))

		if opcode == lneEndSequence {
			return true
		}

	// Standard opcodes [DWARF2 6.2.5.2]
	case lnsCopy:
		goto emit

	case lnsAdvancePC:
		r.advancePC(int(r.buf.uint()))

	case lnsAdvanceLine:
		r.state.Line += int(r.buf.int())

	case lnsSetFile:
		r.fileIndex = int(r.buf.uint())
		r.updateFile()

	case lnsSetColumn:
		r.state.Column = int(r.buf.uint())

	case lnsNegateStmt:
		r.state.IsStmt = !r.state.IsStmt

	case lnsSetBasicBlock:
		r.state.BasicBlock = true

	case lnsConstAddPC:
		r.advancePC((255 - r.opcodeBase) / r.lineRange)

	case lnsFixedAdvancePC:
		r.state.Address += uint64(r.buf.uint16())

	// DWARF3 standard opcodes [DWARF3 6.2.5.2]
	case lnsSetPrologueEnd:
		r.state.PrologueEnd = true

	case lnsSetEpilogueBegin:
		r.state.EpilogueBegin = true

	case lnsSetISA:
		r.state.ISA = int(r.buf.uint())

	default:
		// Unhandled standard opcode. Skip the number of
		// arguments that the prologue says this opcode has.
		for i := 0; i < r.opcodeLengths[opcode]; i++ {
			r.buf.uint()
		}
	}
	return false

emit:
	*entry = r.state
	r.state.BasicBlock = false
	r.state.PrologueEnd = false
	r.state.EpilogueBegin = false
	r.state.Discriminator = 0
	return true
}

// advancePC advances "operation pointer" (the combination of Address
// and OpIndex) in r.state by opAdvance steps.
func (r *LineReader) advancePC(opAdvance int) {
	opIndex := r.state.OpIndex + opAdvance
	r.state.Address += uint64(r.minInstructionLength * (opIndex / r.maxOpsPerInstruction))
	r.state.OpIndex = opIndex % r.maxOpsPerInstruction
}

// A LineReaderPos represents a position in a line table.
type LineReaderPos struct {
	// off is the current offset in the DWARF line section.
	off Offset
	// numFileEntries is the length of fileEntries.
	numFileEntries int
	// state and fileIndex are the statement machine state at
	// offset off.
	state     LineEntry
	fileIndex int
}

// Tell returns the current position in the line table.
func (r *LineReader) Tell() LineReaderPos {
	return LineReaderPos{r.buf.off, len(r.fileEntries), r.state, r.fileIndex}
}

// Seek restores the line table reader to a position returned by [LineReader.Tell].
//
// The argument pos must have been returned by a call to [LineReader.Tell] on this
// line table.
func (r *LineReader) Seek(pos LineReaderPos) {
	r.buf.off = pos.off
	r.buf.data = r.section[r.buf.off:r.endOffset]
	r.fileEntries = r.fileEntries[:pos.numFileEntries]
	r.state = pos.state
	r.fileIndex = pos.fileIndex
}

// Reset repositions the line table reader at the beginning of the
// line table.
func (r *LineReader) Reset() {
	// Reset buffer to the line number program offset.
	r.buf.off = r.programOffset
	r.buf.data = r.section[r.buf.off:r.endOffset]

	// Reset file entries list.
	r.fileEntries = r.fileEntries[:r.initialFileEntries]

	// Reset line number program state.
	r.resetState()
}

// resetState resets r.state to its default values
func (r *LineReader) resetState() {
	// Reset the state machine registers to the defaults given in
	// [DWARF4 6.2.2].
	r.state = LineEntry{
		Address:       0,
		OpIndex:       0,
		File:          nil,
		Line:          1,
		Column:        0,
		IsStmt:        r.defaultIsStmt,
		BasicBlock:    false,
		PrologueEnd:   false,
		EpilogueBegin: false,
		ISA:           0,
		Discriminator: 0,
	}
	r.fileIndex = 1
	r.updateFile()
}

// Files returns the file name table of this compilation unit as of
// the current position in the line table. The file name table may be
// referenced from attributes in this compilation unit such as
// [AttrDeclFile].
//
// Entry 0 is always nil, since file index 0 represents "no file".
//
// The file name table of a compilation unit is not fixed. Files
// returns the file table as of the current position in the line
// table. This may contain more entries than the file table at an
// earlier position in the line table, though existing entries never
// change.
func (r *LineReader) Files() []*LineFile {
	return r.fileEntries
}

// ErrUnknownPC is the error returned by LineReader.ScanPC when the
// seek PC is not covered by any entry in the line table.
var ErrUnknownPC = errors.New("ErrUnknownPC")

// SeekPC sets *entry to the [LineEntry] that includes pc and positions
// the reader on the next entry in the line table. If necessary, this
// will seek backwards to find pc.
//
// If pc is not covered by any entry in this line table, SeekPC
// returns [ErrUnknownPC]. In this case, *entry and the final seek
// position are unspecified.
//
// Note that DWARF line tables only permit sequential, forward scans.
// Hence, in the worst case, this takes time linear in the size of the
// line table. If the caller wishes to do repeated fast PC lookups, it
// should build an appropriate index of the line table.
func (r *LineReader) SeekPC(pc uint64, entry *LineEntry) error {
	if err := r.Next(entry); err != nil {
		return err
	}
	if entry.Address > pc {
		// We're too far. Start at the beginning of the table.
		r.Reset()
		if err := r.Next(entry); err != nil {
			return err
		}
		if entry.Address > pc {
			// The whole table starts after pc.
			r.Reset()
			return ErrUnknownPC
		}
	}

	// Scan until we pass pc, then back up one.
	for {
		var next LineEntry
		pos := r.Tell()
		if err := r.Next(&next); err != nil {
			if err == io.EOF {
				return ErrUnknownPC
			}
			return err
		}
		if next.Address > pc {
			if entry.EndSequence {
				// pc is in a hole in the table.
				return ErrUnknownPC
			}
			// entry is the desired entry. Back up the
			// cursor to "next" and return success.
			r.Seek(pos)
			return nil
		}
		*entry = next
	}
}

// pathIsAbs reports whether path is an absolute path (or "full path
// name" in DWARF parlance). This is in "whatever form makes sense for
// the host system", so this accepts both UNIX-style and DOS-style
// absolute paths. We avoid the filepath package because we want this
// to behave the same regardless of our host system and because we
// don't know what system the paths came from.
func pathIsAbs(path string) bool {
	_, path = splitDrive(path)
	return len(path) > 0 && (path[0] == '/' || path[0] == '\\')
}

// pathJoin joins dirname and filename. filename must be relative.
// DWARF paths can be UNIX-style or DOS-style, so this handles both.
func pathJoin(dirname, filename string) string {
	if len(dirname) == 0 {
		return filename
	}
	// dirname should be absolute, which means we can determine
	// whether it's a DOS path reasonably reliably by looking for
	// a drive letter or UNC path.
	drive, dirname := splitDrive(dirname)
	if drive == "" {
		// UNIX-style path.
		return path.Join(dirname, filename)
	}
	// DOS-style path.
	drive2, filename := splitDrive(filename)
	if drive2 != "" {
		if !strings.EqualFold(drive, drive2) {
			// Different drives. There's not much we can
			// do here, so just ignore the directory.
			return drive2 + filename
		}
		// Drives are the same. Ignore drive on filename.
	}
	if !(strings.HasSuffix(dirname, "/") || strings.HasSuffix(dirname, `\`)) && dirname != "" {
		sep := `\`
		if strings.HasPrefix(dirname, "/") {
			sep = `/`
		}
		dirname += sep
	}
	return drive + dirname + filename
}

// splitDrive splits the DOS drive letter or UNC share point from
// path, if any. path == drive + rest
func splitDrive(path string) (drive, rest string) {
	if len(path) >= 2 && path[1] == ':' {
		if c := path[0]; 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' {
			return path[:2], path[2:]
		}
	}
	if len(path) > 3 && (path[0] == '\\' || path[0] == '/') && (path[1] == '\\' || path[1] == '/') {
		// Normalize the path so we can search for just \ below.
		npath := strings.ReplaceAll(path, "/", `\`)
		// Get the host part, which must be non-empty.
		slash1 := strings.IndexByte(npath[2:], '\\') + 2
		if slash1 > 2 {
			// Get the mount-point part, which must be non-empty.
			slash2 := strings.IndexByte(npath[slash1+1:], '\\') + slash1 + 1
			if slash2 > slash1 {
				return path[:slash2], path[slash2:]
			}
		}
	}
	return "", path
}

```

// === FILE: references!/go/src/debug/dwarf/open.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package dwarf provides access to DWARF debugging information loaded from
executable files, as defined in the DWARF 2.0 Standard at
http://dwarfstd.org/doc/dwarf-2.0.0.pdf.

# Security

This package is not designed to be hardened against adversarial inputs, and is
outside the scope of https://go.dev/security/policy. In particular, only basic
validation is done when parsing object files. As such, care should be taken when
parsing untrusted inputs, as parsing malformed files may consume significant
resources, or cause panics.
*/
package dwarf

import (
	"encoding/binary"
	"errors"
)

// Data represents the DWARF debugging information
// loaded from an executable file (for example, an ELF or Mach-O executable).
type Data struct {
	// raw data
	abbrev   []byte
	aranges  []byte
	frame    []byte
	info     []byte
	line     []byte
	pubnames []byte
	ranges   []byte
	str      []byte

	// New sections added in DWARF 5.
	addr       []byte
	lineStr    []byte
	strOffsets []byte
	rngLists   []byte

	// parsed data
	abbrevCache map[uint64]abbrevTable
	bigEndian   bool
	order       binary.ByteOrder
	typeCache   map[Offset]Type
	typeSigs    map[uint64]*typeUnit
	unit        []unit
}

var errSegmentSelector = errors.New("non-zero segment_selector size not supported")

// New returns a new [Data] object initialized from the given parameters.
// Rather than calling this function directly, clients should typically use
// the DWARF method of the File type of the appropriate package [debug/elf],
// [debug/macho], or [debug/pe].
//
// The []byte arguments are the data from the corresponding debug section
// in the object file; for example, for an ELF object, abbrev is the contents of
// the ".debug_abbrev" section.
func New(abbrev, aranges, frame, info, line, pubnames, ranges, str []byte) (*Data, error) {
	d := &Data{
		abbrev:      abbrev,
		aranges:     aranges,
		frame:       frame,
		info:        info,
		line:        line,
		pubnames:    pubnames,
		ranges:      ranges,
		str:         str,
		abbrevCache: make(map[uint64]abbrevTable),
		typeCache:   make(map[Offset]Type),
		typeSigs:    make(map[uint64]*typeUnit),
	}

	// Sniff .debug_info to figure out byte order.
	// 32-bit DWARF: 4 byte length, 2 byte version.
	// 64-bit DWARf: 4 bytes of 0xff, 8 byte length, 2 byte version.
	if len(d.info) < 6 {
		return nil, DecodeError{"info", Offset(len(d.info)), "too short"}
	}
	offset := 4
	if d.info[0] == 0xff && d.info[1] == 0xff && d.info[2] == 0xff && d.info[3] == 0xff {
		if len(d.info) < 14 {
			return nil, DecodeError{"info", Offset(len(d.info)), "too short"}
		}
		offset = 12
	}
	// Fetch the version, a tiny 16-bit number (1, 2, 3, 4, 5).
	x, y := d.info[offset], d.info[offset+1]
	switch {
	case x == 0 && y == 0:
		return nil, DecodeError{"info", 4, "unsupported version 0"}
	case x == 0:
		d.bigEndian = true
		d.order = binary.BigEndian
	case y == 0:
		d.bigEndian = false
		d.order = binary.LittleEndian
	default:
		return nil, DecodeError{"info", 4, "cannot determine byte order"}
	}

	u, err := d.parseUnits()
	if err != nil {
		return nil, err
	}
	d.unit = u
	return d, nil
}

// AddTypes will add one .debug_types section to the DWARF data. A
// typical object with DWARF version 4 debug info will have multiple
// .debug_types sections. The name is used for error reporting only,
// and serves to distinguish one .debug_types section from another.
func (d *Data) AddTypes(name string, types []byte) error {
	return d.parseTypes(name, types)
}

// AddSection adds another DWARF section by name. The name should be a
// DWARF section name such as ".debug_addr", ".debug_str_offsets", and
// so forth. This approach is used for new DWARF sections added in
// DWARF 5 and later.
func (d *Data) AddSection(name string, contents []byte) error {
	var err error
	switch name {
	case ".debug_addr":
		d.addr = contents
	case ".debug_line_str":
		d.lineStr = contents
	case ".debug_str_offsets":
		d.strOffsets = contents
	case ".debug_rnglists":
		d.rngLists = contents
	}
	// Just ignore names that we don't yet support.
	return err
}

```

// === FILE: references!/go/src/debug/dwarf/tag_string.go ===
```go
// Code generated by "stringer -type Tag -trimprefix=Tag"; DO NOT EDIT.

package dwarf

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[TagArrayType-1]
	_ = x[TagClassType-2]
	_ = x[TagEntryPoint-3]
	_ = x[TagEnumerationType-4]
	_ = x[TagFormalParameter-5]
	_ = x[TagImportedDeclaration-8]
	_ = x[TagLabel-10]
	_ = x[TagLexDwarfBlock-11]
	_ = x[TagMember-13]
	_ = x[TagPointerType-15]
	_ = x[TagReferenceType-16]
	_ = x[TagCompileUnit-17]
	_ = x[TagStringType-18]
	_ = x[TagStructType-19]
	_ = x[TagSubroutineType-21]
	_ = x[TagTypedef-22]
	_ = x[TagUnionType-23]
	_ = x[TagUnspecifiedParameters-24]
	_ = x[TagVariant-25]
	_ = x[TagCommonDwarfBlock-26]
	_ = x[TagCommonInclusion-27]
	_ = x[TagInheritance-28]
	_ = x[TagInlinedSubroutine-29]
	_ = x[TagModule-30]
	_ = x[TagPtrToMemberType-31]
	_ = x[TagSetType-32]
	_ = x[TagSubrangeType-33]
	_ = x[TagWithStmt-34]
	_ = x[TagAccessDeclaration-35]
	_ = x[TagBaseType-36]
	_ = x[TagCatchDwarfBlock-37]
	_ = x[TagConstType-38]
	_ = x[TagConstant-39]
	_ = x[TagEnumerator-40]
	_ = x[TagFileType-41]
	_ = x[TagFriend-42]
	_ = x[TagNamelist-43]
	_ = x[TagNamelistItem-44]
	_ = x[TagPackedType-45]
	_ = x[TagSubprogram-46]
	_ = x[TagTemplateTypeParameter-47]
	_ = x[TagTemplateValueParameter-48]
	_ = x[TagThrownType-49]
	_ = x[TagTryDwarfBlock-50]
	_ = x[TagVariantPart-51]
	_ = x[TagVariable-52]
	_ = x[TagVolatileType-53]
	_ = x[TagDwarfProcedure-54]
	_ = x[TagRestrictType-55]
	_ = x[TagInterfaceType-56]
	_ = x[TagNamespace-57]
	_ = x[TagImportedModule-58]
	_ = x[TagUnspecifiedType-59]
	_ = x[TagPartialUnit-60]
	_ = x[TagImportedUnit-61]
	_ = x[TagMutableType-62]
	_ = x[TagCondition-63]
	_ = x[TagSharedType-64]
	_ = x[TagTypeUnit-65]
	_ = x[TagRvalueReferenceType-66]
	_ = x[TagTemplateAlias-67]
	_ = x[TagCoarrayType-68]
	_ = x[TagGenericSubrange-69]
	_ = x[TagDynamicType-70]
	_ = x[TagAtomicType-71]
	_ = x[TagCallSite-72]
	_ = x[TagCallSiteParameter-73]
	_ = x[TagSkeletonUnit-74]
	_ = x[TagImmutableType-75]
}

const (
	_Tag_name_0 = "ArrayTypeClassTypeEntryPointEnumerationTypeFormalParameter"
	_Tag_name_1 = "ImportedDeclaration"
	_Tag_name_2 = "LabelLexDwarfBlock"
	_Tag_name_3 = "Member"
	_Tag_name_4 = "PointerTypeReferenceTypeCompileUnitStringTypeStructType"
	_Tag_name_5 = "SubroutineTypeTypedefUnionTypeUnspecifiedParametersVariantCommonDwarfBlockCommonInclusionInheritanceInlinedSubroutineModulePtrToMemberTypeSetTypeSubrangeTypeWithStmtAccessDeclarationBaseTypeCatchDwarfBlockConstTypeConstantEnumeratorFileTypeFriendNamelistNamelistItemPackedTypeSubprogramTemplateTypeParameterTemplateValueParameterThrownTypeTryDwarfBlockVariantPartVariableVolatileTypeDwarfProcedureRestrictTypeInterfaceTypeNamespaceImportedModuleUnspecifiedTypePartialUnitImportedUnitMutableTypeConditionSharedTypeTypeUnitRvalueReferenceTypeTemplateAliasCoarrayTypeGenericSubrangeDynamicTypeAtomicTypeCallSiteCallSiteParameterSkeletonUnitImmutableType"
)

var (
	_Tag_index_0 = [...]uint8{0, 9, 18, 28, 43, 58}
	_Tag_index_2 = [...]uint8{0, 5, 18}
	_Tag_index_4 = [...]uint8{0, 11, 24, 35, 45, 55}
	_Tag_index_5 = [...]uint16{0, 14, 21, 30, 51, 58, 74, 89, 100, 117, 123, 138, 145, 157, 165, 182, 190, 205, 214, 222, 232, 240, 246, 254, 266, 276, 286, 307, 329, 339, 352, 363, 371, 383, 397, 409, 422, 431, 445, 460, 471, 483, 494, 503, 513, 521, 540, 553, 564, 579, 590, 600, 608, 625, 637, 650}
)

func (i Tag) String() string {
	switch {
	case 1 <= i && i <= 5:
		i -= 1
		return _Tag_name_0[_Tag_index_0[i]:_Tag_index_0[i+1]]
	case i == 8:
		return _Tag_name_1
	case 10 <= i && i <= 11:
		i -= 10
		return _Tag_name_2[_Tag_index_2[i]:_Tag_index_2[i+1]]
	case i == 13:
		return _Tag_name_3
	case 15 <= i && i <= 19:
		i -= 15
		return _Tag_name_4[_Tag_index_4[i]:_Tag_index_4[i+1]]
	case 21 <= i && i <= 75:
		i -= 21
		return _Tag_name_5[_Tag_index_5[i]:_Tag_index_5[i+1]]
	default:
		return "Tag(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}

```

// === FILE: references!/go/src/debug/dwarf/type.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// DWARF type information structures.
// The format is heavily biased toward C, but for simplicity
// the String methods use a pseudo-Go syntax.

package dwarf

import "strconv"

// A Type conventionally represents a pointer to any of the
// specific Type structures ([CharType], [StructType], etc.).
type Type interface {
	Common() *CommonType
	String() string
	Size() int64
}

// A CommonType holds fields common to multiple types.
// If a field is not known or not applicable for a given type,
// the zero value is used.
type CommonType struct {
	ByteSize int64  // size of value of this type, in bytes
	Name     string // name that can be used to refer to type
}

func (c *CommonType) Common() *CommonType { return c }

func (c *CommonType) Size() int64 { return c.ByteSize }

// Basic types

// A BasicType holds fields common to all basic types.
//
// See the documentation for [StructField] for more info on the interpretation of
// the BitSize/BitOffset/DataBitOffset fields.
type BasicType struct {
	CommonType
	BitSize       int64
	BitOffset     int64
	DataBitOffset int64
}

func (b *BasicType) Basic() *BasicType { return b }

func (t *BasicType) String() string {
	if t.Name != "" {
		return t.Name
	}
	return "?"
}

// A CharType represents a signed character type.
type CharType struct {
	BasicType
}

// A UcharType represents an unsigned character type.
type UcharType struct {
	BasicType
}

// An IntType represents a signed integer type.
type IntType struct {
	BasicType
}

// A UintType represents an unsigned integer type.
type UintType struct {
	BasicType
}

// A FloatType represents a floating point type.
type FloatType struct {
	BasicType
}

// A ComplexType represents a complex floating point type.
type ComplexType struct {
	BasicType
}

// A BoolType represents a boolean type.
type BoolType struct {
	BasicType
}

// An AddrType represents a machine address type.
type AddrType struct {
	BasicType
}

// An UnspecifiedType represents an implicit, unknown, ambiguous or nonexistent type.
type UnspecifiedType struct {
	BasicType
}

// qualifiers

// A QualType represents a type that has the C/C++ "const", "restrict", or "volatile" qualifier.
type QualType struct {
	CommonType
	Qual string
	Type Type
}

func (t *QualType) String() string { return t.Qual + " " + t.Type.String() }

func (t *QualType) Size() int64 { return t.Type.Size() }

// An ArrayType represents a fixed size array type.
type ArrayType struct {
	CommonType
	Type          Type
	StrideBitSize int64 // if > 0, number of bits to hold each element
	Count         int64 // if == -1, an incomplete array, like char x[].
}

func (t *ArrayType) String() string {
	return "[" + strconv.FormatInt(t.Count, 10) + "]" + t.Type.String()
}

func (t *ArrayType) Size() int64 {
	if t.Count == -1 {
		return 0
	}
	return t.Count * t.Type.Size()
}

// A VoidType represents the C void type.
type VoidType struct {
	CommonType
}

func (t *VoidType) String() string { return "void" }

// A PtrType represents a pointer type.
type PtrType struct {
	CommonType
	Type Type
}

func (t *PtrType) String() string { return "*" + t.Type.String() }

// A StructType represents a struct, union, or C++ class type.
type StructType struct {
	CommonType
	StructName string
	Kind       string // "struct", "union", or "class".
	Field      []*StructField
	Incomplete bool // if true, struct, union, class is declared but not defined
}

// A StructField represents a field in a struct, union, or C++ class type.
//
// # Bit Fields
//
// The BitSize, BitOffset, and DataBitOffset fields describe the bit
// size and offset of data members declared as bit fields in C/C++
// struct/union/class types.
//
// BitSize is the number of bits in the bit field.
//
// DataBitOffset, if non-zero, is the number of bits from the start of
// the enclosing entity (e.g. containing struct/class/union) to the
// start of the bit field. This corresponds to the DW_AT_data_bit_offset
// DWARF attribute that was introduced in DWARF 4.
//
// BitOffset, if non-zero, is the number of bits between the most
// significant bit of the storage unit holding the bit field to the
// most significant bit of the bit field. Here "storage unit" is the
// type name before the bit field (for a field "unsigned x:17", the
// storage unit is "unsigned"). BitOffset values can vary depending on
// the endianness of the system. BitOffset corresponds to the
// DW_AT_bit_offset DWARF attribute that was deprecated in DWARF 4 and
// removed in DWARF 5.
//
// At most one of DataBitOffset and BitOffset will be non-zero;
// DataBitOffset/BitOffset will only be non-zero if BitSize is
// non-zero. Whether a C compiler uses one or the other
// will depend on compiler vintage and command line options.
//
// Here is an example of C/C++ bit field use, along with what to
// expect in terms of DWARF bit offset info. Consider this code:
//
//	struct S {
//		int q;
//		int j:5;
//		int k:6;
//		int m:5;
//		int n:8;
//	} s;
//
// For the code above, one would expect to see the following for
// DW_AT_bit_offset values (using GCC 8):
//
//	       Little   |     Big
//	       Endian   |    Endian
//	                |
//	"j":     27     |     0
//	"k":     21     |     5
//	"m":     16     |     11
//	"n":     8      |     16
//
// Note that in the above the offsets are purely with respect to the
// containing storage unit for j/k/m/n -- these values won't vary based
// on the size of prior data members in the containing struct.
//
// If the compiler emits DW_AT_data_bit_offset, the expected values
// would be:
//
//	"j":     32
//	"k":     37
//	"m":     43
//	"n":     48
//
// Here the value 32 for "j" reflects the fact that the bit field is
// preceded by other data members (recall that DW_AT_data_bit_offset
// values are relative to the start of the containing struct). Hence
// DW_AT_data_bit_offset values can be quite large for structs with
// many fields.
//
// DWARF also allow for the possibility of base types that have
// non-zero bit size and bit offset, so this information is also
// captured for base types, but it is worth noting that it is not
// possible to trigger this behavior using mainstream languages.
type StructField struct {
	Name          string
	Type          Type
	ByteOffset    int64
	ByteSize      int64 // usually zero; use Type.Size() for normal fields
	BitOffset     int64
	DataBitOffset int64
	BitSize       int64 // zero if not a bit field
}

func (t *StructType) String() string {
	if t.StructName != "" {
		return t.Kind + " " + t.StructName
	}
	return t.Defn()
}

func (f *StructField) bitOffset() int64 {
	if f.BitOffset != 0 {
		return f.BitOffset
	}
	return f.DataBitOffset
}

func (t *StructType) Defn() string {
	s := t.Kind
	if t.StructName != "" {
		s += " " + t.StructName
	}
	if t.Incomplete {
		s += " /*incomplete*/"
		return s
	}
	s += " {"
	for i, f := range t.Field {
		if i > 0 {
			s += "; "
		}
		s += f.Name + " " + f.Type.String()
		s += "@" + strconv.FormatInt(f.ByteOffset, 10)
		if f.BitSize > 0 {
			s += " : " + strconv.FormatInt(f.BitSize, 10)
			s += "@" + strconv.FormatInt(f.bitOffset(), 10)
		}
	}
	s += "}"
	return s
}

// An EnumType represents an enumerated type.
// The only indication of its native integer type is its ByteSize
// (inside [CommonType]).
type EnumType struct {
	CommonType
	EnumName string
	Val      []*EnumValue
}

// An EnumValue represents a single enumeration value.
type EnumValue struct {
	Name string
	Val  int64
}

func (t *EnumType) String() string {
	s := "enum"
	if t.EnumName != "" {
		s += " " + t.EnumName
	}
	s += " {"
	for i, v := range t.Val {
		if i > 0 {
			s += "; "
		}
		s += v.Name + "=" + strconv.FormatInt(v.Val, 10)
	}
	s += "}"
	return s
}

// A FuncType represents a function type.
type FuncType struct {
	CommonType
	ReturnType Type
	ParamType  []Type
}

func (t *FuncType) String() string {
	s := "func("
	for i, t := range t.ParamType {
		if i > 0 {
			s += ", "
		}
		s += t.String()
	}
	s += ")"
	if t.ReturnType != nil {
		s += " " + t.ReturnType.String()
	}
	return s
}

// A DotDotDotType represents the variadic ... function parameter.
type DotDotDotType struct {
	CommonType
}

func (t *DotDotDotType) String() string { return "..." }

// A TypedefType represents a named type.
type TypedefType struct {
	CommonType
	Type Type
}

func (t *TypedefType) String() string { return t.Name }

func (t *TypedefType) Size() int64 { return t.Type.Size() }

// An UnsupportedType is a placeholder returned in situations where we
// encounter a type that isn't supported.
type UnsupportedType struct {
	CommonType
	Tag Tag
}

func (t *UnsupportedType) String() string {
	if t.Name != "" {
		return t.Name
	}
	return t.Name + "(unsupported type " + t.Tag.String() + ")"
}

// typeReader is used to read from either the info section or the
// types section.
type typeReader interface {
	Seek(Offset)
	Next() (*Entry, error)
	clone() typeReader
	offset() Offset
	// AddressSize returns the size in bytes of addresses in the current
	// compilation unit.
	AddressSize() int
}

// Type reads the type at off in the DWARF “info” section.
func (d *Data) Type(off Offset) (Type, error) {
	return d.readType("info", d.Reader(), off, d.typeCache, nil)
}

type typeFixer struct {
	typedefs   []*TypedefType
	arraytypes []*Type
}

func (tf *typeFixer) recordArrayType(t *Type) {
	if t == nil {
		return
	}
	_, ok := (*t).(*ArrayType)
	if ok {
		tf.arraytypes = append(tf.arraytypes, t)
	}
}

func (tf *typeFixer) apply() {
	for _, t := range tf.typedefs {
		t.Common().ByteSize = t.Type.Size()
	}
	for _, t := range tf.arraytypes {
		zeroArray(t)
	}
}

// readType reads a type from r at off of name. It adds types to the
// type cache, appends new typedef types to typedefs, and computes the
// sizes of types. Callers should pass nil for typedefs; this is used
// for internal recursion.
func (d *Data) readType(name string, r typeReader, off Offset, typeCache map[Offset]Type, fixups *typeFixer) (Type, error) {
	if t, ok := typeCache[off]; ok {
		return t, nil
	}
	r.Seek(off)
	e, err := r.Next()
	if err != nil {
		return nil, err
	}
	addressSize := r.AddressSize()
	if e == nil || e.Offset != off {
		return nil, DecodeError{name, off, "no type at offset"}
	}

	// If this is the root of the recursion, prepare to resolve
	// typedef sizes and perform other fixups once the recursion is
	// done. This must be done after the type graph is constructed
	// because it may need to resolve cycles in a different order than
	// readType encounters them.
	if fixups == nil {
		var fixer typeFixer
		defer func() {
			fixer.apply()
		}()
		fixups = &fixer
	}

	// Parse type from Entry.
	// Must always set typeCache[off] before calling
	// d.readType recursively, to handle circular types correctly.
	var typ Type

	nextDepth := 0

	// Get next child; set err if error happens.
	next := func() *Entry {
		if !e.Children {
			return nil
		}
		// Only return direct children.
		// Skip over composite entries that happen to be nested
		// inside this one. Most DWARF generators wouldn't generate
		// such a thing, but clang does.
		// See golang.org/issue/6472.
		for {
			kid, err1 := r.Next()
			if err1 != nil {
				err = err1
				return nil
			}
			if kid == nil {
				err = DecodeError{name, r.offset(), "unexpected end of DWARF entries"}
				return nil
			}
			if kid.Tag == 0 {
				if nextDepth > 0 {
					nextDepth--
					continue
				}
				return nil
			}
			if kid.Children {
				nextDepth++
			}
			if nextDepth > 0 {
				continue
			}
			return kid
		}
	}

	// Get Type referred to by Entry's AttrType field.
	// Set err if error happens. Not having a type is an error.
	typeOf := func(e *Entry) Type {
		tval := e.Val(AttrType)
		var t Type
		switch toff := tval.(type) {
		case Offset:
			if t, err = d.readType(name, r.clone(), toff, typeCache, fixups); err != nil {
				return nil
			}
		case uint64:
			if t, err = d.sigToType(toff); err != nil {
				return nil
			}
		default:
			// It appears that no Type means "void".
			return new(VoidType)
		}
		return t
	}

	switch e.Tag {
	case TagArrayType:
		// Multi-dimensional array.  (DWARF v2 §5.4)
		// Attributes:
		//	AttrType:subtype [required]
		//	AttrStrideSize: size in bits of each element of the array
		//	AttrByteSize: size of entire array
		// Children:
		//	TagSubrangeType or TagEnumerationType giving one dimension.
		//	dimensions are in left to right order.
		t := new(ArrayType)
		typ = t
		typeCache[off] = t
		if t.Type = typeOf(e); err != nil {
			goto Error
		}
		t.StrideBitSize, _ = e.Val(AttrStrideSize).(int64)

		// Accumulate dimensions,
		var dims []int64
		for kid := next(); kid != nil; kid = next() {
			// TODO(rsc): Can also be TagEnumerationType
			// but haven't seen that in the wild yet.
			switch kid.Tag {
			case TagSubrangeType:
				count, ok := kid.Val(AttrCount).(int64)
				if !ok {
					// Old binaries may have an upper bound instead.
					count, ok = kid.Val(AttrUpperBound).(int64)
					if ok {
						count++ // Length is one more than upper bound.
					} else if len(dims) == 0 {
						count = -1 // As in x[].
					}
				}
				dims = append(dims, count)
			case TagEnumerationType:
				err = DecodeError{name, kid.Offset, "cannot handle enumeration type as array bound"}
				goto Error
			}
		}
		if len(dims) == 0 {
			// LLVM generates this for x[].
			dims = []int64{-1}
		}

		t.Count = dims[0]
		for i := len(dims) - 1; i >= 1; i-- {
			t.Type = &ArrayType{Type: t.Type, Count: dims[i]}
		}

	case TagBaseType:
		// Basic type.  (DWARF v2 §5.1)
		// Attributes:
		//	AttrName: name of base type in programming language of the compilation unit [required]
		//	AttrEncoding: encoding value for type (encFloat etc) [required]
		//	AttrByteSize: size of type in bytes [required]
		//	AttrBitOffset: bit offset of value within containing storage unit
		//	AttrDataBitOffset: bit offset of value within containing storage unit
		//	AttrBitSize: size in bits
		//
		// For most languages BitOffset/DataBitOffset/BitSize will not be present
		// for base types.
		name, _ := e.Val(AttrName).(string)
		enc, ok := e.Val(AttrEncoding).(int64)
		if !ok {
			err = DecodeError{name, e.Offset, "missing encoding attribute for " + name}
			goto Error
		}
		switch enc {
		default:
			err = DecodeError{name, e.Offset, "unrecognized encoding attribute value"}
			goto Error

		case encAddress:
			typ = new(AddrType)
		case encBoolean:
			typ = new(BoolType)
		case encComplexFloat:
			typ = new(ComplexType)
			if name == "complex" {
				// clang writes out 'complex' instead of 'complex float' or 'complex double'.
				// clang also writes out a byte size that we can use to distinguish.
				// See issue 8694.
				switch byteSize, _ := e.Val(AttrByteSize).(int64); byteSize {
				case 8:
					name = "complex float"
				case 16:
					name = "complex double"
				}
			}
		case encFloat:
			typ = new(FloatType)
		case encSigned:
			typ = new(IntType)
		case encUnsigned:
			typ = new(UintType)
		case encSignedChar:
			typ = new(CharType)
		case encUnsignedChar:
			typ = new(UcharType)
		}
		typeCache[off] = typ
		t := typ.(interface {
			Basic() *BasicType
		}).Basic()
		t.Name = name
		t.BitSize, _ = e.Val(AttrBitSize).(int64)
		haveBitOffset := false
		haveDataBitOffset := false
		t.BitOffset, haveBitOffset = e.Val(AttrBitOffset).(int64)
		t.DataBitOffset, haveDataBitOffset = e.Val(AttrDataBitOffset).(int64)
		if haveBitOffset && haveDataBitOffset {
			err = DecodeError{name, e.Offset, "duplicate bit offset attributes"}
			goto Error
		}

	case TagClassType, TagStructType, TagUnionType:
		// Structure, union, or class type.  (DWARF v2 §5.5)
		// Attributes:
		//	AttrName: name of struct, union, or class
		//	AttrByteSize: byte size [required]
		//	AttrDeclaration: if true, struct/union/class is incomplete
		// Children:
		//	TagMember to describe one member.
		//		AttrName: name of member [required]
		//		AttrType: type of member [required]
		//		AttrByteSize: size in bytes
		//		AttrBitOffset: bit offset within bytes for bit fields
		//		AttrDataBitOffset: field bit offset relative to struct start
		//		AttrBitSize: bit size for bit fields
		//		AttrDataMemberLoc: location within struct [required for struct, class]
		// There is much more to handle C++, all ignored for now.
		t := new(StructType)
		typ = t
		typeCache[off] = t
		switch e.Tag {
		case TagClassType:
			t.Kind = "class"
		case TagStructType:
			t.Kind = "struct"
		case TagUnionType:
			t.Kind = "union"
		}
		t.StructName, _ = e.Val(AttrName).(string)
		t.Incomplete = e.Val(AttrDeclaration) != nil
		t.Field = make([]*StructField, 0, 8)
		var lastFieldType *Type
		var lastFieldBitSize int64
		var lastFieldByteOffset int64
		for kid := next(); kid != nil; kid = next() {
			if kid.Tag != TagMember {
				continue
			}
			f := new(StructField)
			if f.Type = typeOf(kid); err != nil {
				goto Error
			}
			switch loc := kid.Val(AttrDataMemberLoc).(type) {
			case []byte:
				// TODO: Should have original compilation
				// unit here, not unknownFormat.
				b := makeBuf(d, unknownFormat{}, "location", 0, loc)
				if b.uint8() != opPlusUconst {
					err = DecodeError{name, kid.Offset, "unexpected opcode"}
					goto Error
				}
				f.ByteOffset = int64(b.uint())
				if b.err != nil {
					err = b.err
					goto Error
				}
			case int64:
				f.ByteOffset = loc
			}

			f.Name, _ = kid.Val(AttrName).(string)
			f.ByteSize, _ = kid.Val(AttrByteSize).(int64)
			haveBitOffset := false
			haveDataBitOffset := false
			f.BitOffset, haveBitOffset = kid.Val(AttrBitOffset).(int64)
			f.DataBitOffset, haveDataBitOffset = kid.Val(AttrDataBitOffset).(int64)
			if haveBitOffset && haveDataBitOffset {
				err = DecodeError{name, e.Offset, "duplicate bit offset attributes"}
				goto Error
			}
			f.BitSize, _ = kid.Val(AttrBitSize).(int64)
			t.Field = append(t.Field, f)

			if lastFieldBitSize == 0 && lastFieldByteOffset == f.ByteOffset && t.Kind != "union" {
				// Last field was zero width. Fix array length.
				// (DWARF writes out 0-length arrays as if they were 1-length arrays.)
				fixups.recordArrayType(lastFieldType)
			}
			lastFieldType = &f.Type
			lastFieldByteOffset = f.ByteOffset
			lastFieldBitSize = f.BitSize
		}
		if t.Kind != "union" {
			b, ok := e.Val(AttrByteSize).(int64)
			if ok && b == lastFieldByteOffset {
				// Final field must be zero width. Fix array length.
				fixups.recordArrayType(lastFieldType)
			}
		}

	case TagConstType, TagVolatileType, TagRestrictType:
		// Type modifier (DWARF v2 §5.2)
		// Attributes:
		//	AttrType: subtype
		t := new(QualType)
		typ = t
		typeCache[off] = t
		if t.Type = typeOf(e); err != nil {
			goto Error
		}
		switch e.Tag {
		case TagConstType:
			t.Qual = "const"
		case TagRestrictType:
			t.Qual = "restrict"
		case TagVolatileType:
			t.Qual = "volatile"
		}

	case TagEnumerationType:
		// Enumeration type (DWARF v2 §5.6)
		// Attributes:
		//	AttrName: enum name if any
		//	AttrByteSize: bytes required to represent largest value
		// Children:
		//	TagEnumerator:
		//		AttrName: name of constant
		//		AttrConstValue: value of constant
		t := new(EnumType)
		typ = t
		typeCache[off] = t
		t.EnumName, _ = e.Val(AttrName).(string)
		t.Val = make([]*EnumValue, 0, 8)
		for kid := next(); kid != nil; kid = next() {
			if kid.Tag == TagEnumerator {
				f := new(EnumValue)
				f.Name, _ = kid.Val(AttrName).(string)
				f.Val, _ = kid.Val(AttrConstValue).(int64)
				n := len(t.Val)
				if n >= cap(t.Val) {
					val := make([]*EnumValue, n, n*2)
					copy(val, t.Val)
					t.Val = val
				}
				t.Val = t.Val[0 : n+1]
				t.Val[n] = f
			}
		}

	case TagPointerType:
		// Type modifier (DWARF v2 §5.2)
		// Attributes:
		//	AttrType: subtype [not required!  void* has no AttrType]
		//	AttrAddrClass: address class [ignored]
		t := new(PtrType)
		typ = t
		typeCache[off] = t
		if e.Val(AttrType) == nil {
			t.Type = &VoidType{}
			break
		}
		t.Type = typeOf(e)

	case TagSubroutineType:
		// Subroutine type.  (DWARF v2 §5.7)
		// Attributes:
		//	AttrType: type of return value if any
		//	AttrName: possible name of type [ignored]
		//	AttrPrototyped: whether used ANSI C prototype [ignored]
		// Children:
		//	TagFormalParameter: typed parameter
		//		AttrType: type of parameter
		//	TagUnspecifiedParameter: final ...
		t := new(FuncType)
		typ = t
		typeCache[off] = t
		if t.ReturnType = typeOf(e); err != nil {
			goto Error
		}
		t.ParamType = make([]Type, 0, 8)
		for kid := next(); kid != nil; kid = next() {
			var tkid Type
			switch kid.Tag {
			default:
				continue
			case TagFormalParameter:
				if tkid = typeOf(kid); err != nil {
					goto Error
				}
			case TagUnspecifiedParameters:
				tkid = &DotDotDotType{}
			}
			t.ParamType = append(t.ParamType, tkid)
		}

	case TagTypedef:
		// Typedef (DWARF v2 §5.3)
		// Attributes:
		//	AttrName: name [required]
		//	AttrType: type definition [required]
		t := new(TypedefType)
		typ = t
		typeCache[off] = t
		t.Name, _ = e.Val(AttrName).(string)
		t.Type = typeOf(e)

	case TagUnspecifiedType:
		// Unspecified type (DWARF v3 §5.2)
		// Attributes:
		//	AttrName: name
		t := new(UnspecifiedType)
		typ = t
		typeCache[off] = t
		t.Name, _ = e.Val(AttrName).(string)

	default:
		// This is some other type DIE that we're currently not
		// equipped to handle. Return an abstract "unsupported type"
		// object in such cases.
		t := new(UnsupportedType)
		typ = t
		typeCache[off] = t
		t.Tag = e.Tag
		t.Name, _ = e.Val(AttrName).(string)
	}

	if err != nil {
		goto Error
	}

	{
		b, ok := e.Val(AttrByteSize).(int64)
		if !ok {
			b = -1
			switch t := typ.(type) {
			case *TypedefType:
				// Record that we need to resolve this
				// type's size once the type graph is
				// constructed.
				fixups.typedefs = append(fixups.typedefs, t)
			case *PtrType:
				b = int64(addressSize)
			}
		}
		typ.Common().ByteSize = b
	}
	return typ, nil

Error:
	// If the parse fails, take the type out of the cache
	// so that the next call with this offset doesn't hit
	// the cache and return success.
	delete(typeCache, off)
	return nil, err
}

func zeroArray(t *Type) {
	at := (*t).(*ArrayType)
	if at.Type.Size() == 0 {
		return
	}
	// Make a copy to avoid invalidating typeCache.
	tt := *at
	tt.Count = 0
	*t = &tt
}

```

// === FILE: references!/go/src/debug/dwarf/typeunit.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dwarf

import (
	"fmt"
	"strconv"
)

// Parse the type units stored in a DWARF4 .debug_types section. Each
// type unit defines a single primary type and an 8-byte signature.
// Other sections may then use formRefSig8 to refer to the type.

// The typeUnit format is a single type with a signature. It holds
// the same data as a compilation unit.
type typeUnit struct {
	unit
	toff  Offset // Offset to signature type within data.
	name  string // Name of .debug_type section.
	cache Type   // Cache the type, nil to start.
}

// Parse a .debug_types section.
func (d *Data) parseTypes(name string, types []byte) error {
	b := makeBuf(d, unknownFormat{}, name, 0, types)
	for len(b.data) > 0 {
		base := b.off
		n, dwarf64 := b.unitLength()
		if n != Offset(uint32(n)) {
			b.error("type unit length overflow")
			return b.err
		}
		hdroff := b.off
		vers := int(b.uint16())
		if vers != 4 {
			b.error("unsupported DWARF version " + strconv.Itoa(vers))
			return b.err
		}
		var ao uint64
		if !dwarf64 {
			ao = uint64(b.uint32())
		} else {
			ao = b.uint64()
		}
		atable, err := d.parseAbbrev(ao, vers)
		if err != nil {
			return err
		}
		asize := b.uint8()
		sig := b.uint64()

		var toff uint32
		if !dwarf64 {
			toff = b.uint32()
		} else {
			to64 := b.uint64()
			if to64 != uint64(uint32(to64)) {
				b.error("type unit type offset overflow")
				return b.err
			}
			toff = uint32(to64)
		}

		boff := b.off
		d.typeSigs[sig] = &typeUnit{
			unit: unit{
				base:   base,
				off:    boff,
				data:   b.bytes(int(n - (b.off - hdroff))),
				atable: atable,
				asize:  int(asize),
				vers:   vers,
				is64:   dwarf64,
			},
			toff: Offset(toff),
			name: name,
		}
		if b.err != nil {
			return b.err
		}
	}
	return nil
}

// Return the type for a type signature.
func (d *Data) sigToType(sig uint64) (Type, error) {
	tu := d.typeSigs[sig]
	if tu == nil {
		return nil, fmt.Errorf("no type unit with signature %v", sig)
	}
	if tu.cache != nil {
		return tu.cache, nil
	}

	b := makeBuf(d, tu, tu.name, tu.off, tu.data)
	r := &typeUnitReader{d: d, tu: tu, b: b}
	t, err := d.readType(tu.name, r, tu.toff, make(map[Offset]Type), nil)
	if err != nil {
		return nil, err
	}

	tu.cache = t
	return t, nil
}

// typeUnitReader is a typeReader for a tagTypeUnit.
type typeUnitReader struct {
	d   *Data
	tu  *typeUnit
	b   buf
	err error
}

// Seek to a new position in the type unit.
func (tur *typeUnitReader) Seek(off Offset) {
	tur.err = nil
	doff := off - tur.tu.off
	if doff < 0 || doff >= Offset(len(tur.tu.data)) {
		tur.err = fmt.Errorf("%s: offset %d out of range; max %d", tur.tu.name, doff, len(tur.tu.data))
		return
	}
	tur.b = makeBuf(tur.d, tur.tu, tur.tu.name, off, tur.tu.data[doff:])
}

// AddressSize returns the size in bytes of addresses in the current type unit.
func (tur *typeUnitReader) AddressSize() int {
	return tur.tu.unit.asize
}

// Next reads the next [Entry] from the type unit.
func (tur *typeUnitReader) Next() (*Entry, error) {
	if tur.err != nil {
		return nil, tur.err
	}
	if len(tur.tu.data) == 0 {
		return nil, nil
	}
	e := tur.b.entry(nil, &tur.tu.unit)
	if tur.b.err != nil {
		tur.err = tur.b.err
		return nil, tur.err
	}
	return e, nil
}

// clone returns a new reader for the type unit.
func (tur *typeUnitReader) clone() typeReader {
	return &typeUnitReader{
		d:  tur.d,
		tu: tur.tu,
		b:  makeBuf(tur.d, tur.tu, tur.tu.name, tur.tu.off, tur.tu.data),
	}
}

// offset returns the current offset.
func (tur *typeUnitReader) offset() Offset {
	return tur.b.off
}

```

// === FILE: references!/go/src/debug/dwarf/unit.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dwarf

import (
	"sort"
	"strconv"
)

// DWARF debug info is split into a sequence of compilation units.
// Each unit has its own abbreviation table and address size.

type unit struct {
	base   Offset // byte offset of header within the aggregate info
	off    Offset // byte offset of data within the aggregate info
	data   []byte
	atable abbrevTable
	*unit5 // info specific to DWARF 5 units
	asize  int
	vers   int
	is64   bool  // True for 64-bit DWARF format
	utype  uint8 // DWARF 5 unit type
}

type unit5 struct {
	addrBase       uint64
	strOffsetsBase uint64
	rngListsBase   uint64
	locListsBase   uint64
}

// Implement the dataFormat interface.

func (u *unit) version() int {
	return u.vers
}

func (u *unit) dwarf64() (bool, bool) {
	return u.is64, true
}

func (u *unit) addrsize() int {
	return u.asize
}

func (u *unit) addrBase() uint64 {
	if u.unit5 != nil {
		return u.unit5.addrBase
	}
	return 0
}

func (u *unit) strOffsetsBase() uint64 {
	if u.unit5 != nil {
		return u.unit5.strOffsetsBase
	}
	return 0
}

func (u *unit) rngListsBase() uint64 {
	if u.unit5 != nil {
		return u.unit5.rngListsBase
	}
	return 0
}

func (u *unit) locListsBase() uint64 {
	if u.unit5 != nil {
		return u.unit5.locListsBase
	}
	return 0
}

func (d *Data) parseUnits() ([]unit, error) {
	// Count units.
	nunit := 0
	b := makeBuf(d, unknownFormat{}, "info", 0, d.info)
	for len(b.data) > 0 {
		len, _ := b.unitLength()
		if len != Offset(uint32(len)) {
			b.error("unit length overflow")
			break
		}
		b.skip(int(len))
		if len > 0 {
			nunit++
		}
	}
	if b.err != nil {
		return nil, b.err
	}

	// Again, this time writing them down.
	b = makeBuf(d, unknownFormat{}, "info", 0, d.info)
	units := make([]unit, nunit)
	for i := range units {
		u := &units[i]
		u.base = b.off
		var n Offset
		if b.err != nil {
			return nil, b.err
		}
		for n == 0 {
			n, u.is64 = b.unitLength()
		}
		dataOff := b.off
		vers := b.uint16()
		if vers < 2 || vers > 5 {
			b.error("unsupported DWARF version " + strconv.Itoa(int(vers)))
			break
		}
		u.vers = int(vers)
		if vers >= 5 {
			u.utype = b.uint8()
			u.asize = int(b.uint8())
		}
		var abbrevOff uint64
		if u.is64 {
			abbrevOff = b.uint64()
		} else {
			abbrevOff = uint64(b.uint32())
		}
		atable, err := d.parseAbbrev(abbrevOff, u.vers)
		if err != nil {
			if b.err == nil {
				b.err = err
			}
			break
		}
		u.atable = atable
		if vers < 5 {
			u.asize = int(b.uint8())
		}

		switch u.utype {
		case utSkeleton, utSplitCompile:
			b.uint64() // unit ID
		case utType, utSplitType:
			b.uint64()  // type signature
			if u.is64 { // type offset
				b.uint64()
			} else {
				b.uint32()
			}
		}

		u.off = b.off
		u.data = b.bytes(int(n - (b.off - dataOff)))
	}
	if b.err != nil {
		return nil, b.err
	}
	return units, nil
}

// offsetToUnit returns the index of the unit containing offset off.
// It returns -1 if no unit contains this offset.
func (d *Data) offsetToUnit(off Offset) int {
	// Find the unit after off
	next := sort.Search(len(d.unit), func(i int) bool {
		return d.unit[i].off > off
	})
	if next == 0 {
		return -1
	}
	u := &d.unit[next-1]
	if u.off <= off && off < u.off+Offset(len(u.data)) {
		return next - 1
	}
	return -1
}

func (d *Data) collectDwarf5BaseOffsets(u *unit) error {
	if u.unit5 == nil {
		panic("expected unit5 to be set up already")
	}
	b := makeBuf(d, u, "info", u.off, u.data)
	cu := b.entry(nil, u)
	if cu == nil {
		// Unknown abbreviation table entry or some other fatal
		// problem; bail early on the assumption that this will be
		// detected at some later point.
		return b.err
	}
	if iAddrBase, ok := cu.Val(AttrAddrBase).(int64); ok {
		u.unit5.addrBase = uint64(iAddrBase)
	}
	if iStrOffsetsBase, ok := cu.Val(AttrStrOffsetsBase).(int64); ok {
		u.unit5.strOffsetsBase = uint64(iStrOffsetsBase)
	}
	if iRngListsBase, ok := cu.Val(AttrRnglistsBase).(int64); ok {
		u.unit5.rngListsBase = uint64(iRngListsBase)
	}
	if iLocListsBase, ok := cu.Val(AttrLoclistsBase).(int64); ok {
		u.unit5.locListsBase = uint64(iLocListsBase)
	}
	return nil
}

```

// === FILE: references!/go/src/debug/elf/elf.go ===
```go
/*
 * ELF constants and data structures
 *
 * Derived from:
 * $FreeBSD: src/sys/sys/elf32.h,v 1.8.14.1 2005/12/30 22:13:58 marcel Exp $
 * $FreeBSD: src/sys/sys/elf64.h,v 1.10.14.1 2005/12/30 22:13:58 marcel Exp $
 * $FreeBSD: src/sys/sys/elf_common.h,v 1.15.8.1 2005/12/30 22:13:58 marcel Exp $
 * $FreeBSD: src/sys/alpha/include/elf.h,v 1.14 2003/09/25 01:10:22 peter Exp $
 * $FreeBSD: src/sys/amd64/include/elf.h,v 1.18 2004/08/03 08:21:48 dfr Exp $
 * $FreeBSD: src/sys/arm/include/elf.h,v 1.5.2.1 2006/06/30 21:42:52 cognet Exp $
 * $FreeBSD: src/sys/i386/include/elf.h,v 1.16 2004/08/02 19:12:17 dfr Exp $
 * $FreeBSD: src/sys/powerpc/include/elf.h,v 1.7 2004/11/02 09:47:01 ssouhlal Exp $
 * $FreeBSD: src/sys/sparc64/include/elf.h,v 1.12 2003/09/25 01:10:26 peter Exp $
 * "System V ABI" (http://www.sco.com/developers/gabi/latest/ch4.eheader.html)
 * "ELF for the ARM® 64-bit Architecture (AArch64)" (ARM IHI 0056B)
 * "RISC-V ELF psABI specification" (https://github.com/riscv-non-isa/riscv-elf-psabi-doc/blob/master/riscv-elf.adoc)
 * llvm/BinaryFormat/ELF.h - ELF constants and structures
 *
 * Copyright (c) 1996-1998 John D. Polstra.  All rights reserved.
 * Copyright (c) 2001 David E. O'Brien
 * Portions Copyright 2009 The Go Authors. All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE AUTHOR AND CONTRIBUTORS ``AS IS'' AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
 * FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 * LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 * OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 * SUCH DAMAGE.
 */

package elf

import "strconv"

/*
 * Constants
 */

// Indexes into the Header.Ident array.
const (
	EI_CLASS      = 4  /* Class of machine. */
	EI_DATA       = 5  /* Data format. */
	EI_VERSION    = 6  /* ELF format version. */
	EI_OSABI      = 7  /* Operating system / ABI identification */
	EI_ABIVERSION = 8  /* ABI version */
	EI_PAD        = 9  /* Start of padding (per SVR4 ABI). */
	EI_NIDENT     = 16 /* Size of e_ident array. */
)

// Initial magic number for ELF files.
const ELFMAG = "\177ELF"

// Version is found in Header.Ident[EI_VERSION] and Header.Version.
type Version byte

const (
	EV_NONE    Version = 0
	EV_CURRENT Version = 1
)

var versionStrings = []intName{
	{0, "EV_NONE"},
	{1, "EV_CURRENT"},
}

func (i Version) String() string   { return stringName(uint32(i), versionStrings, false) }
func (i Version) GoString() string { return stringName(uint32(i), versionStrings, true) }

// Class is found in Header.Ident[EI_CLASS] and Header.Class.
type Class byte

const (
	ELFCLASSNONE Class = 0 /* Unknown class. */
	ELFCLASS32   Class = 1 /* 32-bit architecture. */
	ELFCLASS64   Class = 2 /* 64-bit architecture. */
)

var classStrings = []intName{
	{0, "ELFCLASSNONE"},
	{1, "ELFCLASS32"},
	{2, "ELFCLASS64"},
}

func (i Class) String() string   { return stringName(uint32(i), classStrings, false) }
func (i Class) GoString() string { return stringName(uint32(i), classStrings, true) }

// Data is found in Header.Ident[EI_DATA] and Header.Data.
type Data byte

const (
	ELFDATANONE Data = 0 /* Unknown data format. */
	ELFDATA2LSB Data = 1 /* 2's complement little-endian. */
	ELFDATA2MSB Data = 2 /* 2's complement big-endian. */
)

var dataStrings = []intName{
	{0, "ELFDATANONE"},
	{1, "ELFDATA2LSB"},
	{2, "ELFDATA2MSB"},
}

func (i Data) String() string   { return stringName(uint32(i), dataStrings, false) }
func (i Data) GoString() string { return stringName(uint32(i), dataStrings, true) }

// OSABI is found in Header.Ident[EI_OSABI] and Header.OSABI.
type OSABI byte

const (
	ELFOSABI_NONE       OSABI = 0   /* UNIX System V ABI */
	ELFOSABI_HPUX       OSABI = 1   /* HP-UX operating system */
	ELFOSABI_NETBSD     OSABI = 2   /* NetBSD */
	ELFOSABI_LINUX      OSABI = 3   /* Linux */
	ELFOSABI_HURD       OSABI = 4   /* Hurd */
	ELFOSABI_86OPEN     OSABI = 5   /* 86Open common IA32 ABI */
	ELFOSABI_SOLARIS    OSABI = 6   /* Solaris */
	ELFOSABI_AIX        OSABI = 7   /* AIX */
	ELFOSABI_IRIX       OSABI = 8   /* IRIX */
	ELFOSABI_FREEBSD    OSABI = 9   /* FreeBSD */
	ELFOSABI_TRU64      OSABI = 10  /* TRU64 UNIX */
	ELFOSABI_MODESTO    OSABI = 11  /* Novell Modesto */
	ELFOSABI_OPENBSD    OSABI = 12  /* OpenBSD */
	ELFOSABI_OPENVMS    OSABI = 13  /* Open VMS */
	ELFOSABI_NSK        OSABI = 14  /* HP Non-Stop Kernel */
	ELFOSABI_AROS       OSABI = 15  /* Amiga Research OS */
	ELFOSABI_FENIXOS    OSABI = 16  /* The FenixOS highly scalable multi-core OS */
	ELFOSABI_CLOUDABI   OSABI = 17  /* Nuxi CloudABI */
	ELFOSABI_ARM        OSABI = 97  /* ARM */
	ELFOSABI_STANDALONE OSABI = 255 /* Standalone (embedded) application */
)

var osabiStrings = []intName{
	{0, "ELFOSABI_NONE"},
	{1, "ELFOSABI_HPUX"},
	{2, "ELFOSABI_NETBSD"},
	{3, "ELFOSABI_LINUX"},
	{4, "ELFOSABI_HURD"},
	{5, "ELFOSABI_86OPEN"},
	{6, "ELFOSABI_SOLARIS"},
	{7, "ELFOSABI_AIX"},
	{8, "ELFOSABI_IRIX"},
	{9, "ELFOSABI_FREEBSD"},
	{10, "ELFOSABI_TRU64"},
	{11, "ELFOSABI_MODESTO"},
	{12, "ELFOSABI_OPENBSD"},
	{13, "ELFOSABI_OPENVMS"},
	{14, "ELFOSABI_NSK"},
	{15, "ELFOSABI_AROS"},
	{16, "ELFOSABI_FENIXOS"},
	{17, "ELFOSABI_CLOUDABI"},
	{97, "ELFOSABI_ARM"},
	{255, "ELFOSABI_STANDALONE"},
}

func (i OSABI) String() string   { return stringName(uint32(i), osabiStrings, false) }
func (i OSABI) GoString() string { return stringName(uint32(i), osabiStrings, true) }

// Type is found in Header.Type.
type Type uint16

const (
	ET_NONE   Type = 0      /* Unknown type. */
	ET_REL    Type = 1      /* Relocatable. */
	ET_EXEC   Type = 2      /* Executable. */
	ET_DYN    Type = 3      /* Shared object. */
	ET_CORE   Type = 4      /* Core file. */
	ET_LOOS   Type = 0xfe00 /* First operating system specific. */
	ET_HIOS   Type = 0xfeff /* Last operating system-specific. */
	ET_LOPROC Type = 0xff00 /* First processor-specific. */
	ET_HIPROC Type = 0xffff /* Last processor-specific. */
)

var typeStrings = []intName{
	{0, "ET_NONE"},
	{1, "ET_REL"},
	{2, "ET_EXEC"},
	{3, "ET_DYN"},
	{4, "ET_CORE"},
	{0xfe00, "ET_LOOS"},
	{0xfeff, "ET_HIOS"},
	{0xff00, "ET_LOPROC"},
	{0xffff, "ET_HIPROC"},
}

func (i Type) String() string   { return stringName(uint32(i), typeStrings, false) }
func (i Type) GoString() string { return stringName(uint32(i), typeStrings, true) }

// Machine is found in Header.Machine.
type Machine uint16

const (
	EM_NONE          Machine = 0   /* Unknown machine. */
	EM_M32           Machine = 1   /* AT&T WE32100. */
	EM_SPARC         Machine = 2   /* Sun SPARC. */
	EM_386           Machine = 3   /* Intel i386. */
	EM_68K           Machine = 4   /* Motorola 68000. */
	EM_88K           Machine = 5   /* Motorola 88000. */
	EM_860           Machine = 7   /* Intel i860. */
	EM_MIPS          Machine = 8   /* MIPS R3000 Big-Endian only. */
	EM_S370          Machine = 9   /* IBM System/370. */
	EM_MIPS_RS3_LE   Machine = 10  /* MIPS R3000 Little-Endian. */
	EM_PARISC        Machine = 15  /* HP PA-RISC. */
	EM_VPP500        Machine = 17  /* Fujitsu VPP500. */
	EM_SPARC32PLUS   Machine = 18  /* SPARC v8plus. */
	EM_960           Machine = 19  /* Intel 80960. */
	EM_PPC           Machine = 20  /* PowerPC 32-bit. */
	EM_PPC64         Machine = 21  /* PowerPC 64-bit. */
	EM_S390          Machine = 22  /* IBM System/390. */
	EM_V800          Machine = 36  /* NEC V800. */
	EM_FR20          Machine = 37  /* Fujitsu FR20. */
	EM_RH32          Machine = 38  /* TRW RH-32. */
	EM_RCE           Machine = 39  /* Motorola RCE. */
	EM_ARM           Machine = 40  /* ARM. */
	EM_SH            Machine = 42  /* Hitachi SH. */
	EM_SPARCV9       Machine = 43  /* SPARC v9 64-bit. */
	EM_TRICORE       Machine = 44  /* Siemens TriCore embedded processor. */
	EM_ARC           Machine = 45  /* Argonaut RISC Core. */
	EM_H8_300        Machine = 46  /* Hitachi H8/300. */
	EM_H8_300H       Machine = 47  /* Hitachi H8/300H. */
	EM_H8S           Machine = 48  /* Hitachi H8S. */
	EM_H8_500        Machine = 49  /* Hitachi H8/500. */
	EM_IA_64         Machine = 50  /* Intel IA-64 Processor. */
	EM_MIPS_X        Machine = 51  /* Stanford MIPS-X. */
	EM_COLDFIRE      Machine = 52  /* Motorola ColdFire. */
	EM_68HC12        Machine = 53  /* Motorola M68HC12. */
	EM_MMA           Machine = 54  /* Fujitsu MMA. */
	EM_PCP           Machine = 55  /* Siemens PCP. */
	EM_NCPU          Machine = 56  /* Sony nCPU. */
	EM_NDR1          Machine = 57  /* Denso NDR1 microprocessor. */
	EM_STARCORE      Machine = 58  /* Motorola Star*Core processor. */
	EM_ME16          Machine = 59  /* Toyota ME16 processor. */
	EM_ST100         Machine = 60  /* STMicroelectronics ST100 processor. */
	EM_TINYJ         Machine = 61  /* Advanced Logic Corp. TinyJ processor. */
	EM_X86_64        Machine = 62  /* Advanced Micro Devices x86-64 */
	EM_PDSP          Machine = 63  /* Sony DSP Processor */
	EM_PDP10         Machine = 64  /* Digital Equipment Corp. PDP-10 */
	EM_PDP11         Machine = 65  /* Digital Equipment Corp. PDP-11 */
	EM_FX66          Machine = 66  /* Siemens FX66 microcontroller */
	EM_ST9PLUS       Machine = 67  /* STMicroelectronics ST9+ 8/16 bit microcontroller */
	EM_ST7           Machine = 68  /* STMicroelectronics ST7 8-bit microcontroller */
	EM_68HC16        Machine = 69  /* Motorola MC68HC16 Microcontroller */
	EM_68HC11        Machine = 70  /* Motorola MC68HC11 Microcontroller */
	EM_68HC08        Machine = 71  /* Motorola MC68HC08 Microcontroller */
	EM_68HC05        Machine = 72  /* Motorola MC68HC05 Microcontroller */
	EM_SVX           Machine = 73  /* Silicon Graphics SVx */
	EM_ST19          Machine = 74  /* STMicroelectronics ST19 8-bit microcontroller */
	EM_VAX           Machine = 75  /* Digital VAX */
	EM_CRIS          Machine = 76  /* Axis Communications 32-bit embedded processor */
	EM_JAVELIN       Machine = 77  /* Infineon Technologies 32-bit embedded processor */
	EM_FIREPATH      Machine = 78  /* Element 14 64-bit DSP Processor */
	EM_ZSP           Machine = 79  /* LSI Logic 16-bit DSP Processor */
	EM_MMIX          Machine = 80  /* Donald Knuth's educational 64-bit processor */
	EM_HUANY         Machine = 81  /* Harvard University machine-independent object files */
	EM_PRISM         Machine = 82  /* SiTera Prism */
	EM_AVR           Machine = 83  /* Atmel AVR 8-bit microcontroller */
	EM_FR30          Machine = 84  /* Fujitsu FR30 */
	EM_D10V          Machine = 85  /* Mitsubishi D10V */
	EM_D30V          Machine = 86  /* Mitsubishi D30V */
	EM_V850          Machine = 87  /* NEC v850 */
	EM_M32R          Machine = 88  /* Mitsubishi M32R */
	EM_MN10300       Machine = 89  /* Matsushita MN10300 */
	EM_MN10200       Machine = 90  /* Matsushita MN10200 */
	EM_PJ            Machine = 91  /* picoJava */
	EM_OPENRISC      Machine = 92  /* OpenRISC 32-bit embedded processor */
	EM_ARC_COMPACT   Machine = 93  /* ARC International ARCompact processor (old spelling/synonym: EM_ARC_A5) */
	EM_XTENSA        Machine = 94  /* Tensilica Xtensa Architecture */
	EM_VIDEOCORE     Machine = 95  /* Alphamosaic VideoCore processor */
	EM_TMM_GPP       Machine = 96  /* Thompson Multimedia General Purpose Processor */
	EM_NS32K         Machine = 97  /* National Semiconductor 32000 series */
	EM_TPC           Machine = 98  /* Tenor Network TPC processor */
	EM_SNP1K         Machine = 99  /* Trebia SNP 1000 processor */
	EM_ST200         Machine = 100 /* STMicroelectronics (www.st.com) ST200 microcontroller */
	EM_IP2K          Machine = 101 /* Ubicom IP2xxx microcontroller family */
	EM_MAX           Machine = 102 /* MAX Processor */
	EM_CR            Machine = 103 /* National Semiconductor CompactRISC microprocessor */
	EM_F2MC16        Machine = 104 /* Fujitsu F2MC16 */
	EM_MSP430        Machine = 105 /* Texas Instruments embedded microcontroller msp430 */
	EM_BLACKFIN      Machine = 106 /* Analog Devices Blackfin (DSP) processor */
	EM_SE_C33        Machine = 107 /* S1C33 Family of Seiko Epson processors */
	EM_SEP           Machine = 108 /* Sharp embedded microprocessor */
	EM_ARCA          Machine = 109 /* Arca RISC Microprocessor */
	EM_UNICORE       Machine = 110 /* Microprocessor series from PKU-Unity Ltd. and MPRC of Peking University */
	EM_EXCESS        Machine = 111 /* eXcess: 16/32/64-bit configurable embedded CPU */
	EM_DXP           Machine = 112 /* Icera Semiconductor Inc. Deep Execution Processor */
	EM_ALTERA_NIOS2  Machine = 113 /* Altera Nios II soft-core processor */
	EM_CRX           Machine = 114 /* National Semiconductor CompactRISC CRX microprocessor */
	EM_XGATE         Machine = 115 /* Motorola XGATE embedded processor */
	EM_C166          Machine = 116 /* Infineon C16x/XC16x processor */
	EM_M16C          Machine = 117 /* Renesas M16C series microprocessors */
	EM_DSPIC30F      Machine = 118 /* Microchip Technology dsPIC30F Digital Signal Controller */
	EM_CE            Machine = 119 /* Freescale Communication Engine RISC core */
	EM_M32C          Machine = 120 /* Renesas M32C series microprocessors */
	EM_TSK3000       Machine = 131 /* Altium TSK3000 core */
	EM_RS08          Machine = 132 /* Freescale RS08 embedded processor */
	EM_SHARC         Machine = 133 /* Analog Devices SHARC family of 32-bit DSP processors */
	EM_ECOG2         Machine = 134 /* Cyan Technology eCOG2 microprocessor */
	EM_SCORE7        Machine = 135 /* Sunplus S+core7 RISC processor */
	EM_DSP24         Machine = 136 /* New Japan Radio (NJR) 24-bit DSP Processor */
	EM_VIDEOCORE3    Machine = 137 /* Broadcom VideoCore III processor */
	EM_LATTICEMICO32 Machine = 138 /* RISC processor for Lattice FPGA architecture */
	EM_SE_C17        Machine = 139 /* Seiko Epson C17 family */
	EM_TI_C6000      Machine = 140 /* The Texas Instruments TMS320C6000 DSP family */
	EM_TI_C2000      Machine = 141 /* The Texas Instruments TMS320C2000 DSP family */
	EM_TI_C5500      Machine = 142 /* The Texas Instruments TMS320C55x DSP family */
	EM_TI_ARP32      Machine = 143 /* Texas Instruments Application Specific RISC Processor, 32bit fetch */
	EM_TI_PRU        Machine = 144 /* Texas Instruments Programmable Realtime Unit */
	EM_MMDSP_PLUS    Machine = 160 /* STMicroelectronics 64bit VLIW Data Signal Processor */
	EM_CYPRESS_M8C   Machine = 161 /* Cypress M8C microprocessor */
	EM_R32C          Machine = 162 /* Renesas R32C series microprocessors */
	EM_TRIMEDIA      Machine = 163 /* NXP Semiconductors TriMedia architecture family */
	EM_QDSP6         Machine = 164 /* QUALCOMM DSP6 Processor */
	EM_8051          Machine = 165 /* Intel 8051 and variants */
	EM_STXP7X        Machine = 166 /* STMicroelectronics STxP7x family of configurable and extensible RISC processors */
	EM_NDS32         Machine = 167 /* Andes Technology compact code size embedded RISC processor family */
	EM_ECOG1         Machine = 168 /* Cyan Technology eCOG1X family */
	EM_ECOG1X        Machine = 168 /* Cyan Technology eCOG1X family */
	EM_MAXQ30        Machine = 169 /* Dallas Semiconductor MAXQ30 Core Micro-controllers */
	EM_XIMO16        Machine = 170 /* New Japan Radio (NJR) 16-bit DSP Processor */
	EM_MANIK         Machine = 171 /* M2000 Reconfigurable RISC Microprocessor */
	EM_CRAYNV2       Machine = 172 /* Cray Inc. NV2 vector architecture */
	EM_RX            Machine = 173 /* Renesas RX family */
	EM_METAG         Machine = 174 /* Imagination Technologies META processor architecture */
	EM_MCST_ELBRUS   Machine = 175 /* MCST Elbrus general purpose hardware architecture */
	EM_ECOG16        Machine = 176 /* Cyan Technology eCOG16 family */
	EM_CR16          Machine = 177 /* National Semiconductor CompactRISC CR16 16-bit microprocessor */
	EM_ETPU          Machine = 178 /* Freescale Extended Time Processing Unit */
	EM_SLE9X         Machine = 179 /* Infineon Technologies SLE9X core */
	EM_L10M          Machine = 180 /* Intel L10M */
	EM_K10M          Machine = 181 /* Intel K10M */
	EM_AARCH64       Machine = 183 /* ARM 64-bit Architecture (AArch64) */
	EM_AVR32         Machine = 185 /* Atmel Corporation 32-bit microprocessor family */
	EM_STM8          Machine = 186 /* STMicroeletronics STM8 8-bit microcontroller */
	EM_TILE64        Machine = 187 /* Tilera TILE64 multicore architecture family */
	EM_TILEPRO       Machine = 188 /* Tilera TILEPro multicore architecture family */
	EM_MICROBLAZE    Machine = 189 /* Xilinx MicroBlaze 32-bit RISC soft processor core */
	EM_CUDA          Machine = 190 /* NVIDIA CUDA architecture */
	EM_TILEGX        Machine = 191 /* Tilera TILE-Gx multicore architecture family */
	EM_CLOUDSHIELD   Machine = 192 /* CloudShield architecture family */
	EM_COREA_1ST     Machine = 193 /* KIPO-KAIST Core-A 1st generation processor family */
	EM_COREA_2ND     Machine = 194 /* KIPO-KAIST Core-A 2nd generation processor family */
	EM_ARC_COMPACT2  Machine = 195 /* Synopsys ARCompact V2 */
	EM_OPEN8         Machine = 196 /* Open8 8-bit RISC soft processor core */
	EM_RL78          Machine = 197 /* Renesas RL78 family */
	EM_VIDEOCORE5    Machine = 198 /* Broadcom VideoCore V processor */
	EM_78KOR         Machine = 199 /* Renesas 78KOR family */
	EM_56800EX       Machine = 200 /* Freescale 56800EX Digital Signal Controller (DSC) */
	EM_BA1           Machine = 201 /* Beyond BA1 CPU architecture */
	EM_BA2           Machine = 202 /* Beyond BA2 CPU architecture */
	EM_XCORE         Machine = 203 /* XMOS xCORE processor family */
	EM_MCHP_PIC      Machine = 204 /* Microchip 8-bit PIC(r) family */
	EM_INTEL205      Machine = 205 /* Reserved by Intel */
	EM_INTEL206      Machine = 206 /* Reserved by Intel */
	EM_INTEL207      Machine = 207 /* Reserved by Intel */
	EM_INTEL208      Machine = 208 /* Reserved by Intel */
	EM_INTEL209      Machine = 209 /* Reserved by Intel */
	EM_KM32          Machine = 210 /* KM211 KM32 32-bit processor */
	EM_KMX32         Machine = 211 /* KM211 KMX32 32-bit processor */
	EM_KMX16         Machine = 212 /* KM211 KMX16 16-bit processor */
	EM_KMX8          Machine = 213 /* KM211 KMX8 8-bit processor */
	EM_KVARC         Machine = 214 /* KM211 KVARC processor */
	EM_CDP           Machine = 215 /* Paneve CDP architecture family */
	EM_COGE          Machine = 216 /* Cognitive Smart Memory Processor */
	EM_COOL          Machine = 217 /* Bluechip Systems CoolEngine */
	EM_NORC          Machine = 218 /* Nanoradio Optimized RISC */
	EM_CSR_KALIMBA   Machine = 219 /* CSR Kalimba architecture family */
	EM_Z80           Machine = 220 /* Zilog Z80 */
	EM_VISIUM        Machine = 221 /* Controls and Data Services VISIUMcore processor */
	EM_FT32          Machine = 222 /* FTDI Chip FT32 high performance 32-bit RISC architecture */
	EM_MOXIE         Machine = 223 /* Moxie processor family */
	EM_AMDGPU        Machine = 224 /* AMD GPU architecture */
	EM_RISCV         Machine = 243 /* RISC-V */
	EM_LANAI         Machine = 244 /* Lanai 32-bit processor */
	EM_BPF           Machine = 247 /* Linux BPF – in-kernel virtual machine */
	EM_LOONGARCH     Machine = 258 /* LoongArch */

	/* Non-standard or deprecated. */
	EM_486         Machine = 6      /* Intel i486. */
	EM_MIPS_RS4_BE Machine = 10     /* MIPS R4000 Big-Endian */
	EM_ALPHA_STD   Machine = 41     /* Digital Alpha (standard value). */
	EM_ALPHA       Machine = 0x9026 /* Alpha (written in the absence of an ABI) */
)

var machineStrings = []intName{
	{0, "EM_NONE"},
	{1, "EM_M32"},
	{2, "EM_SPARC"},
	{3, "EM_386"},
	{4, "EM_68K"},
	{5, "EM_88K"},
	{7, "EM_860"},
	{8, "EM_MIPS"},
	{9, "EM_S370"},
	{10, "EM_MIPS_RS3_LE"},
	{15, "EM_PARISC"},
	{17, "EM_VPP500"},
	{18, "EM_SPARC32PLUS"},
	{19, "EM_960"},
	{20, "EM_PPC"},
	{21, "EM_PPC64"},
	{22, "EM_S390"},
	{36, "EM_V800"},
	{37, "EM_FR20"},
	{38, "EM_RH32"},
	{39, "EM_RCE"},
	{40, "EM_ARM"},
	{42, "EM_SH"},
	{43, "EM_SPARCV9"},
	{44, "EM_TRICORE"},
	{45, "EM_ARC"},
	{46, "EM_H8_300"},
	{47, "EM_H8_300H"},
	{48, "EM_H8S"},
	{49, "EM_H8_500"},
	{50, "EM_IA_64"},
	{51, "EM_MIPS_X"},
	{52, "EM_COLDFIRE"},
	{53, "EM_68HC12"},
	{54, "EM_MMA"},
	{55, "EM_PCP"},
	{56, "EM_NCPU"},
	{57, "EM_NDR1"},
	{58, "EM_STARCORE"},
	{59, "EM_ME16"},
	{60, "EM_ST100"},
	{61, "EM_TINYJ"},
	{62, "EM_X86_64"},
	{63, "EM_PDSP"},
	{64, "EM_PDP10"},
	{65, "EM_PDP11"},
	{66, "EM_FX66"},
	{67, "EM_ST9PLUS"},
	{68, "EM_ST7"},
	{69, "EM_68HC16"},
	{70, "EM_68HC11"},
	{71, "EM_68HC08"},
	{72, "EM_68HC05"},
	{73, "EM_SVX"},
	{74, "EM_ST19"},
	{75, "EM_VAX"},
	{76, "EM_CRIS"},
	{77, "EM_JAVELIN"},
	{78, "EM_FIREPATH"},
	{79, "EM_ZSP"},
	{80, "EM_MMIX"},
	{81, "EM_HUANY"},
	{82, "EM_PRISM"},
	{83, "EM_AVR"},
	{84, "EM_FR30"},
	{85, "EM_D10V"},
	{86, "EM_D30V"},
	{87, "EM_V850"},
	{88, "EM_M32R"},
	{89, "EM_MN10300"},
	{90, "EM_MN10200"},
	{91, "EM_PJ"},
	{92, "EM_OPENRISC"},
	{93, "EM_ARC_COMPACT"},
	{94, "EM_XTENSA"},
	{95, "EM_VIDEOCORE"},
	{96, "EM_TMM_GPP"},
	{97, "EM_NS32K"},
	{98, "EM_TPC"},
	{99, "EM_SNP1K"},
	{100, "EM_ST200"},
	{101, "EM_IP2K"},
	{102, "EM_MAX"},
	{103, "EM_CR"},
	{104, "EM_F2MC16"},
	{105, "EM_MSP430"},
	{106, "EM_BLACKFIN"},
	{107, "EM_SE_C33"},
	{108, "EM_SEP"},
	{109, "EM_ARCA"},
	{110, "EM_UNICORE"},
	{111, "EM_EXCESS"},
	{112, "EM_DXP"},
	{113, "EM_ALTERA_NIOS2"},
	{114, "EM_CRX"},
	{115, "EM_XGATE"},
	{116, "EM_C166"},
	{117, "EM_M16C"},
	{118, "EM_DSPIC30F"},
	{119, "EM_CE"},
	{120, "EM_M32C"},
	{131, "EM_TSK3000"},
	{132, "EM_RS08"},
	{133, "EM_SHARC"},
	{134, "EM_ECOG2"},
	{135, "EM_SCORE7"},
	{136, "EM_DSP24"},
	{137, "EM_VIDEOCORE3"},
	{138, "EM_LATTICEMICO32"},
	{139, "EM_SE_C17"},
	{140, "EM_TI_C6000"},
	{141, "EM_TI_C2000"},
	{142, "EM_TI_C5500"},
	{143, "EM_TI_ARP32"},
	{144, "EM_TI_PRU"},
	{160, "EM_MMDSP_PLUS"},
	{161, "EM_CYPRESS_M8C"},
	{162, "EM_R32C"},
	{163, "EM_TRIMEDIA"},
	{164, "EM_QDSP6"},
	{165, "EM_8051"},
	{166, "EM_STXP7X"},
	{167, "EM_NDS32"},
	{168, "EM_ECOG1"},
	{168, "EM_ECOG1X"},
	{169, "EM_MAXQ30"},
	{170, "EM_XIMO16"},
	{171, "EM_MANIK"},
	{172, "EM_CRAYNV2"},
	{173, "EM_RX"},
	{174, "EM_METAG"},
	{175, "EM_MCST_ELBRUS"},
	{176, "EM_ECOG16"},
	{177, "EM_CR16"},
	{178, "EM_ETPU"},
	{179, "EM_SLE9X"},
	{180, "EM_L10M"},
	{181, "EM_K10M"},
	{183, "EM_AARCH64"},
	{185, "EM_AVR32"},
	{186, "EM_STM8"},
	{187, "EM_TILE64"},
	{188, "EM_TILEPRO"},
	{189, "EM_MICROBLAZE"},
	{190, "EM_CUDA"},
	{191, "EM_TILEGX"},
	{192, "EM_CLOUDSHIELD"},
	{193, "EM_COREA_1ST"},
	{194, "EM_COREA_2ND"},
	{195, "EM_ARC_COMPACT2"},
	{196, "EM_OPEN8"},
	{197, "EM_RL78"},
	{198, "EM_VIDEOCORE5"},
	{199, "EM_78KOR"},
	{200, "EM_56800EX"},
	{201, "EM_BA1"},
	{202, "EM_BA2"},
	{203, "EM_XCORE"},
	{204, "EM_MCHP_PIC"},
	{205, "EM_INTEL205"},
	{206, "EM_INTEL206"},
	{207, "EM_INTEL207"},
	{208, "EM_INTEL208"},
	{209, "EM_INTEL209"},
	{210, "EM_KM32"},
	{211, "EM_KMX32"},
	{212, "EM_KMX16"},
	{213, "EM_KMX8"},
	{214, "EM_KVARC"},
	{215, "EM_CDP"},
	{216, "EM_COGE"},
	{217, "EM_COOL"},
	{218, "EM_NORC"},
	{219, "EM_CSR_KALIMBA "},
	{220, "EM_Z80 "},
	{221, "EM_VISIUM "},
	{222, "EM_FT32 "},
	{223, "EM_MOXIE"},
	{224, "EM_AMDGPU"},
	{243, "EM_RISCV"},
	{244, "EM_LANAI"},
	{247, "EM_BPF"},
	{258, "EM_LOONGARCH"},

	/* Non-standard or deprecated. */
	{6, "EM_486"},
	{10, "EM_MIPS_RS4_BE"},
	{41, "EM_ALPHA_STD"},
	{0x9026, "EM_ALPHA"},
}

func (i Machine) String() string   { return stringName(uint32(i), machineStrings, false) }
func (i Machine) GoString() string { return stringName(uint32(i), machineStrings, true) }

// Special section indices.
type SectionIndex int

const (
	SHN_UNDEF     SectionIndex = 0      /* Undefined, missing, irrelevant. */
	SHN_LORESERVE SectionIndex = 0xff00 /* First of reserved range. */
	SHN_LOPROC    SectionIndex = 0xff00 /* First processor-specific. */
	SHN_HIPROC    SectionIndex = 0xff1f /* Last processor-specific. */
	SHN_LOOS      SectionIndex = 0xff20 /* First operating system-specific. */
	SHN_HIOS      SectionIndex = 0xff3f /* Last operating system-specific. */
	SHN_ABS       SectionIndex = 0xfff1 /* Absolute values. */
	SHN_COMMON    SectionIndex = 0xfff2 /* Common data. */
	SHN_XINDEX    SectionIndex = 0xffff /* Escape; index stored elsewhere. */
	SHN_HIRESERVE SectionIndex = 0xffff /* Last of reserved range. */
)

var shnStrings = []intName{
	{0, "SHN_UNDEF"},
	{0xff00, "SHN_LOPROC"},
	{0xff20, "SHN_LOOS"},
	{0xfff1, "SHN_ABS"},
	{0xfff2, "SHN_COMMON"},
	{0xffff, "SHN_XINDEX"},
}

func (i SectionIndex) String() string   { return stringName(uint32(i), shnStrings, false) }
func (i SectionIndex) GoString() string { return stringName(uint32(i), shnStrings, true) }

// Section type.
type SectionType uint32

const (
	SHT_NULL             SectionType = 0          /* inactive */
	SHT_PROGBITS         SectionType = 1          /* program defined information */
	SHT_SYMTAB           SectionType = 2          /* symbol table section */
	SHT_STRTAB           SectionType = 3          /* string table section */
	SHT_RELA             SectionType = 4          /* relocation section with addends */
	SHT_HASH             SectionType = 5          /* symbol hash table section */
	SHT_DYNAMIC          SectionType = 6          /* dynamic section */
	SHT_NOTE             SectionType = 7          /* note section */
	SHT_NOBITS           SectionType = 8          /* no space section */
	SHT_REL              SectionType = 9          /* relocation section - no addends */
	SHT_SHLIB            SectionType = 10         /* reserved - purpose unknown */
	SHT_DYNSYM           SectionType = 11         /* dynamic symbol table section */
	SHT_INIT_ARRAY       SectionType = 14         /* Initialization function pointers. */
	SHT_FINI_ARRAY       SectionType = 15         /* Termination function pointers. */
	SHT_PREINIT_ARRAY    SectionType = 16         /* Pre-initialization function ptrs. */
	SHT_GROUP            SectionType = 17         /* Section group. */
	SHT_SYMTAB_SHNDX     SectionType = 18         /* Section indexes (see SHN_XINDEX). */
	SHT_LOOS             SectionType = 0x60000000 /* First of OS specific semantics */
	SHT_GNU_ATTRIBUTES   SectionType = 0x6ffffff5 /* GNU object attributes */
	SHT_GNU_HASH         SectionType = 0x6ffffff6 /* GNU hash table */
	SHT_GNU_LIBLIST      SectionType = 0x6ffffff7 /* GNU prelink library list */
	SHT_GNU_VERDEF       SectionType = 0x6ffffffd /* GNU version definition section */
	SHT_GNU_VERNEED      SectionType = 0x6ffffffe /* GNU version needs section */
	SHT_GNU_VERSYM       SectionType = 0x6fffffff /* GNU version symbol table */
	SHT_HIOS             SectionType = 0x6fffffff /* Last of OS specific semantics */
	SHT_LOPROC           SectionType = 0x70000000 /* reserved range for processor */
	SHT_RISCV_ATTRIBUTES SectionType = 0x70000003 /* RISCV object attributes */
	SHT_MIPS_ABIFLAGS    SectionType = 0x7000002a /* .MIPS.abiflags */
	SHT_HIPROC           SectionType = 0x7fffffff /* specific section header types */
	SHT_LOUSER           SectionType = 0x80000000 /* reserved range for application */
	SHT_HIUSER           SectionType = 0xffffffff /* specific indexes */
)

var shtStrings = []intName{
	{0, "SHT_NULL"},
	{1, "SHT_PROGBITS"},
	{2, "SHT_SYMTAB"},
	{3, "SHT_STRTAB"},
	{4, "SHT_RELA"},
	{5, "SHT_HASH"},
	{6, "SHT_DYNAMIC"},
	{7, "SHT_NOTE"},
	{8, "SHT_NOBITS"},
	{9, "SHT_REL"},
	{10, "SHT_SHLIB"},
	{11, "SHT_DYNSYM"},
	{14, "SHT_INIT_ARRAY"},
	{15, "SHT_FINI_ARRAY"},
	{16, "SHT_PREINIT_ARRAY"},
	{17, "SHT_GROUP"},
	{18, "SHT_SYMTAB_SHNDX"},
	{0x60000000, "SHT_LOOS"},
	{0x6ffffff5, "SHT_GNU_ATTRIBUTES"},
	{0x6ffffff6, "SHT_GNU_HASH"},
	{0x6ffffff7, "SHT_GNU_LIBLIST"},
	{0x6ffffffd, "SHT_GNU_VERDEF"},
	{0x6ffffffe, "SHT_GNU_VERNEED"},
	{0x6fffffff, "SHT_GNU_VERSYM"},
	{0x70000000, "SHT_LOPROC"},
	// We don't list the processor-dependent SectionType,
	// as the values overlap.
	{0x7000002a, "SHT_MIPS_ABIFLAGS"},
	{0x7fffffff, "SHT_HIPROC"},
	{0x80000000, "SHT_LOUSER"},
	{0xffffffff, "SHT_HIUSER"},
}

func (i SectionType) String() string   { return stringName(uint32(i), shtStrings, false) }
func (i SectionType) GoString() string { return stringName(uint32(i), shtStrings, true) }

// Section flags.
type SectionFlag uint32

const (
	SHF_WRITE            SectionFlag = 0x1        /* Section contains writable data. */
	SHF_ALLOC            SectionFlag = 0x2        /* Section occupies memory. */
	SHF_EXECINSTR        SectionFlag = 0x4        /* Section contains instructions. */
	SHF_MERGE            SectionFlag = 0x10       /* Section may be merged. */
	SHF_STRINGS          SectionFlag = 0x20       /* Section contains strings. */
	SHF_INFO_LINK        SectionFlag = 0x40       /* sh_info holds section index. */
	SHF_LINK_ORDER       SectionFlag = 0x80       /* Special ordering requirements. */
	SHF_OS_NONCONFORMING SectionFlag = 0x100      /* OS-specific processing required. */
	SHF_GROUP            SectionFlag = 0x200      /* Member of section group. */
	SHF_TLS              SectionFlag = 0x400      /* Section contains TLS data. */
	SHF_COMPRESSED       SectionFlag = 0x800      /* Section is compressed. */
	SHF_MASKOS           SectionFlag = 0x0ff00000 /* OS-specific semantics. */
	SHF_MASKPROC         SectionFlag = 0xf0000000 /* Processor-specific semantics. */
)

var shfStrings = []intName{
	{0x1, "SHF_WRITE"},
	{0x2, "SHF_ALLOC"},
	{0x4, "SHF_EXECINSTR"},
	{0x10, "SHF_MERGE"},
	{0x20, "SHF_STRINGS"},
	{0x40, "SHF_INFO_LINK"},
	{0x80, "SHF_LINK_ORDER"},
	{0x100, "SHF_OS_NONCONFORMING"},
	{0x200, "SHF_GROUP"},
	{0x400, "SHF_TLS"},
	{0x800, "SHF_COMPRESSED"},
}

func (i SectionFlag) String() string   { return flagName(uint32(i), shfStrings, false) }
func (i SectionFlag) GoString() string { return flagName(uint32(i), shfStrings, true) }

// Section compression type.
type CompressionType int

const (
	COMPRESS_ZLIB   CompressionType = 1          /* ZLIB compression. */
	COMPRESS_ZSTD   CompressionType = 2          /* ZSTD compression. */
	COMPRESS_LOOS   CompressionType = 0x60000000 /* First OS-specific. */
	COMPRESS_HIOS   CompressionType = 0x6fffffff /* Last OS-specific. */
	COMPRESS_LOPROC CompressionType = 0x70000000 /* First processor-specific type. */
	COMPRESS_HIPROC CompressionType = 0x7fffffff /* Last processor-specific type. */
)

var compressionStrings = []intName{
	{1, "COMPRESS_ZLIB"},
	{2, "COMPRESS_ZSTD"},
	{0x60000000, "COMPRESS_LOOS"},
	{0x6fffffff, "COMPRESS_HIOS"},
	{0x70000000, "COMPRESS_LOPROC"},
	{0x7fffffff, "COMPRESS_HIPROC"},
}

func (i CompressionType) String() string   { return stringName(uint32(i), compressionStrings, false) }
func (i CompressionType) GoString() string { return stringName(uint32(i), compressionStrings, true) }

// Prog.Type
type ProgType int

const (
	PT_NULL    ProgType = 0 /* Unused entry. */
	PT_LOAD    ProgType = 1 /* Loadable segment. */
	PT_DYNAMIC ProgType = 2 /* Dynamic linking information segment. */
	PT_INTERP  ProgType = 3 /* Pathname of interpreter. */
	PT_NOTE    ProgType = 4 /* Auxiliary information. */
	PT_SHLIB   ProgType = 5 /* Reserved (not used). */
	PT_PHDR    ProgType = 6 /* Location of program header itself. */
	PT_TLS     ProgType = 7 /* Thread local storage segment */

	PT_LOOS ProgType = 0x60000000 /* First OS-specific. */

	PT_GNU_EH_FRAME ProgType = 0x6474e550 /* Frame unwind information */
	PT_GNU_STACK    ProgType = 0x6474e551 /* Stack flags */
	PT_GNU_RELRO    ProgType = 0x6474e552 /* Read only after relocs */
	PT_GNU_PROPERTY ProgType = 0x6474e553 /* GNU property */
	PT_GNU_MBIND_LO ProgType = 0x6474e555 /* Mbind segments start */
	PT_GNU_MBIND_HI ProgType = 0x6474f554 /* Mbind segments finish */

	PT_PAX_FLAGS ProgType = 0x65041580 /* PAX flags */

	PT_OPENBSD_RANDOMIZE ProgType = 0x65a3dbe6 /* Random data */
	PT_OPENBSD_WXNEEDED  ProgType = 0x65a3dbe7 /* W^X violations */
	PT_OPENBSD_NOBTCFI   ProgType = 0x65a3dbe8 /* No branch target CFI */
	PT_OPENBSD_BOOTDATA  ProgType = 0x65a41be6 /* Boot arguments */

	PT_SUNW_EH_FRAME ProgType = 0x6474e550 /* Frame unwind information */
	PT_SUNWSTACK     ProgType = 0x6ffffffb /* Stack segment */

	PT_HIOS ProgType = 0x6fffffff /* Last OS-specific. */

	PT_LOPROC ProgType = 0x70000000 /* First processor-specific type. */

	PT_ARM_ARCHEXT ProgType = 0x70000000 /* Architecture compatibility */
	PT_ARM_EXIDX   ProgType = 0x70000001 /* Exception unwind tables */

	PT_AARCH64_ARCHEXT ProgType = 0x70000000 /* Architecture compatibility */
	PT_AARCH64_UNWIND  ProgType = 0x70000001 /* Exception unwind tables */

	PT_MIPS_REGINFO  ProgType = 0x70000000 /* Register usage */
	PT_MIPS_RTPROC   ProgType = 0x70000001 /* Runtime procedures */
	PT_MIPS_OPTIONS  ProgType = 0x70000002 /* Options */
	PT_MIPS_ABIFLAGS ProgType = 0x70000003 /* ABI flags */

	PT_RISCV_ATTRIBUTES ProgType = 0x70000003 /* RISC-V ELF attribute section. */

	PT_S390_PGSTE ProgType = 0x70000000 /* 4k page table size */

	PT_HIPROC ProgType = 0x7fffffff /* Last processor-specific type. */
)

var ptStrings = []intName{
	{0, "PT_NULL"},
	{1, "PT_LOAD"},
	{2, "PT_DYNAMIC"},
	{3, "PT_INTERP"},
	{4, "PT_NOTE"},
	{5, "PT_SHLIB"},
	{6, "PT_PHDR"},
	{7, "PT_TLS"},
	{0x60000000, "PT_LOOS"},
	{0x6474e550, "PT_GNU_EH_FRAME"},
	{0x6474e551, "PT_GNU_STACK"},
	{0x6474e552, "PT_GNU_RELRO"},
	{0x6474e553, "PT_GNU_PROPERTY"},
	{0x65041580, "PT_PAX_FLAGS"},
	{0x65a3dbe6, "PT_OPENBSD_RANDOMIZE"},
	{0x65a3dbe7, "PT_OPENBSD_WXNEEDED"},
	{0x65a41be6, "PT_OPENBSD_BOOTDATA"},
	{0x6ffffffb, "PT_SUNWSTACK"},
	{0x6fffffff, "PT_HIOS"},
	{0x70000000, "PT_LOPROC"},
	// We don't list the processor-dependent ProgTypes,
	// as the values overlap.
	{0x7fffffff, "PT_HIPROC"},
}

func (i ProgType) String() string   { return stringName(uint32(i), ptStrings, false) }
func (i ProgType) GoString() string { return stringName(uint32(i), ptStrings, true) }

// Prog.Flag
type ProgFlag uint32

const (
	PF_X        ProgFlag = 0x1        /* Executable. */
	PF_W        ProgFlag = 0x2        /* Writable. */
	PF_R        ProgFlag = 0x4        /* Readable. */
	PF_MASKOS   ProgFlag = 0x0ff00000 /* Operating system-specific. */
	PF_MASKPROC ProgFlag = 0xf0000000 /* Processor-specific. */
)

var pfStrings = []intName{
	{0x1, "PF_X"},
	{0x2, "PF_W"},
	{0x4, "PF_R"},
}

func (i ProgFlag) String() string   { return flagName(uint32(i), pfStrings, false) }
func (i ProgFlag) GoString() string { return flagName(uint32(i), pfStrings, true) }

// Dyn.Tag
type DynTag int

const (
	DT_NULL         DynTag = 0  /* Terminating entry. */
	DT_NEEDED       DynTag = 1  /* String table offset of a needed shared library. */
	DT_PLTRELSZ     DynTag = 2  /* Total size in bytes of PLT relocations. */
	DT_PLTGOT       DynTag = 3  /* Processor-dependent address. */
	DT_HASH         DynTag = 4  /* Address of symbol hash table. */
	DT_STRTAB       DynTag = 5  /* Address of string table. */
	DT_SYMTAB       DynTag = 6  /* Address of symbol table. */
	DT_RELA         DynTag = 7  /* Address of ElfNN_Rela relocations. */
	DT_RELASZ       DynTag = 8  /* Total size of ElfNN_Rela relocations. */
	DT_RELAENT      DynTag = 9  /* Size of each ElfNN_Rela relocation entry. */
	DT_STRSZ        DynTag = 10 /* Size of string table. */
	DT_SYMENT       DynTag = 11 /* Size of each symbol table entry. */
	DT_INIT         DynTag = 12 /* Address of initialization function. */
	DT_FINI         DynTag = 13 /* Address of finalization function. */
	DT_SONAME       DynTag = 14 /* String table offset of shared object name. */
	DT_RPATH        DynTag = 15 /* String table offset of library path. [sup] */
	DT_SYMBOLIC     DynTag = 16 /* Indicates "symbolic" linking. [sup] */
	DT_REL          DynTag = 17 /* Address of ElfNN_Rel relocations. */
	DT_RELSZ        DynTag = 18 /* Total size of ElfNN_Rel relocations. */
	DT_RELENT       DynTag = 19 /* Size of each ElfNN_Rel relocation. */
	DT_PLTREL       DynTag = 20 /* Type of relocation used for PLT. */
	DT_DEBUG        DynTag = 21 /* Reserved (not used). */
	DT_TEXTREL      DynTag = 22 /* Indicates there may be relocations in non-writable segments. [sup] */
	DT_JMPREL       DynTag = 23 /* Address of PLT relocations. */
	DT_BIND_NOW     DynTag = 24 /* [sup] */
	DT_INIT_ARRAY   DynTag = 25 /* Address of the array of pointers to initialization functions */
	DT_FINI_ARRAY   DynTag = 26 /* Address of the array of pointers to termination functions */
	DT_INIT_ARRAYSZ DynTag = 27 /* Size in bytes of the array of initialization functions. */
	DT_FINI_ARRAYSZ DynTag = 28 /* Size in bytes of the array of termination functions. */
	DT_RUNPATH      DynTag = 29 /* String table offset of a null-terminated library search path string. */
	DT_FLAGS        DynTag = 30 /* Object specific flag values. */
	DT_ENCODING     DynTag = 32 /* Values greater than or equal to DT_ENCODING
	   and less than DT_LOOS follow the rules for
	   the interpretation of the d_un union
	   as follows: even == 'd_ptr', even == 'd_val'
	   or none */
	DT_PREINIT_ARRAY   DynTag = 32 /* Address of the array of pointers to pre-initialization functions. */
	DT_PREINIT_ARRAYSZ DynTag = 33 /* Size in bytes of the array of pre-initialization functions. */
	DT_SYMTAB_SHNDX    DynTag = 34 /* Address of SHT_SYMTAB_SHNDX section. */

	DT_LOOS DynTag = 0x6000000d /* First OS-specific */
	DT_HIOS DynTag = 0x6ffff000 /* Last OS-specific */

	DT_VALRNGLO       DynTag = 0x6ffffd00
	DT_GNU_PRELINKED  DynTag = 0x6ffffdf5
	DT_GNU_CONFLICTSZ DynTag = 0x6ffffdf6
	DT_GNU_LIBLISTSZ  DynTag = 0x6ffffdf7
	DT_CHECKSUM       DynTag = 0x6ffffdf8
	DT_PLTPADSZ       DynTag = 0x6ffffdf9
	DT_MOVEENT        DynTag = 0x6ffffdfa
	DT_MOVESZ         DynTag = 0x6ffffdfb
	DT_FEATURE        DynTag = 0x6ffffdfc
	DT_POSFLAG_1      DynTag = 0x6ffffdfd
	DT_SYMINSZ        DynTag = 0x6ffffdfe
	DT_SYMINENT       DynTag = 0x6ffffdff
	DT_VALRNGHI       DynTag = 0x6ffffdff

	DT_ADDRRNGLO    DynTag = 0x6ffffe00
	DT_GNU_HASH     DynTag = 0x6ffffef5
	DT_TLSDESC_PLT  DynTag = 0x6ffffef6
	DT_TLSDESC_GOT  DynTag = 0x6ffffef7
	DT_GNU_CONFLICT DynTag = 0x6ffffef8
	DT_GNU_LIBLIST  DynTag = 0x6ffffef9
	DT_CONFIG       DynTag = 0x6ffffefa
	DT_DEPAUDIT     DynTag = 0x6ffffefb
	DT_AUDIT        DynTag = 0x6ffffefc
	DT_PLTPAD       DynTag = 0x6ffffefd
	DT_MOVETAB      DynTag = 0x6ffffefe
	DT_SYMINFO      DynTag = 0x6ffffeff
	DT_ADDRRNGHI    DynTag = 0x6ffffeff

	DT_VERSYM     DynTag = 0x6ffffff0
	DT_RELACOUNT  DynTag = 0x6ffffff9
	DT_RELCOUNT   DynTag = 0x6ffffffa
	DT_FLAGS_1    DynTag = 0x6ffffffb
	DT_VERDEF     DynTag = 0x6ffffffc
	DT_VERDEFNUM  DynTag = 0x6ffffffd
	DT_VERNEED    DynTag = 0x6ffffffe
	DT_VERNEEDNUM DynTag = 0x6fffffff

	DT_LOPROC DynTag = 0x70000000 /* First processor-specific type. */

	DT_MIPS_RLD_VERSION           DynTag = 0x70000001
	DT_MIPS_TIME_STAMP            DynTag = 0x70000002
	DT_MIPS_ICHECKSUM             DynTag = 0x70000003
	DT_MIPS_IVERSION              DynTag = 0x70000004
	DT_MIPS_FLAGS                 DynTag = 0x70000005
	DT_MIPS_BASE_ADDRESS          DynTag = 0x70000006
	DT_MIPS_MSYM                  DynTag = 0x70000007
	DT_MIPS_CONFLICT              DynTag = 0x70000008
	DT_MIPS_LIBLIST               DynTag = 0x70000009
	DT_MIPS_LOCAL_GOTNO           DynTag = 0x7000000a
	DT_MIPS_CONFLICTNO            DynTag = 0x7000000b
	DT_MIPS_LIBLISTNO             DynTag = 0x70000010
	DT_MIPS_SYMTABNO              DynTag = 0x70000011
	DT_MIPS_UNREFEXTNO            DynTag = 0x70000012
	DT_MIPS_GOTSYM                DynTag = 0x70000013
	DT_MIPS_HIPAGENO              DynTag = 0x70000014
	DT_MIPS_RLD_MAP               DynTag = 0x70000016
	DT_MIPS_DELTA_CLASS           DynTag = 0x70000017
	DT_MIPS_DELTA_CLASS_NO        DynTag = 0x70000018
	DT_MIPS_DELTA_INSTANCE        DynTag = 0x70000019
	DT_MIPS_DELTA_INSTANCE_NO     DynTag = 0x7000001a
	DT_MIPS_DELTA_RELOC           DynTag = 0x7000001b
	DT_MIPS_DELTA_RELOC_NO        DynTag = 0x7000001c
	DT_MIPS_DELTA_SYM             DynTag = 0x7000001d
	DT_MIPS_DELTA_SYM_NO          DynTag = 0x7000001e
	DT_MIPS_DELTA_CLASSSYM        DynTag = 0x70000020
	DT_MIPS_DELTA_CLASSSYM_NO     DynTag = 0x70000021
	DT_MIPS_CXX_FLAGS             DynTag = 0x70000022
	DT_MIPS_PIXIE_INIT            DynTag = 0x70000023
	DT_MIPS_SYMBOL_LIB            DynTag = 0x70000024
	DT_MIPS_LOCALPAGE_GOTIDX      DynTag = 0x70000025
	DT_MIPS_LOCAL_GOTIDX          DynTag = 0x70000026
	DT_MIPS_HIDDEN_GOTIDX         DynTag = 0x70000027
	DT_MIPS_PROTECTED_GOTIDX      DynTag = 0x70000028
	DT_MIPS_OPTIONS               DynTag = 0x70000029
	DT_MIPS_INTERFACE             DynTag = 0x7000002a
	DT_MIPS_DYNSTR_ALIGN          DynTag = 0x7000002b
	DT_MIPS_INTERFACE_SIZE        DynTag = 0x7000002c
	DT_MIPS_RLD_TEXT_RESOLVE_ADDR DynTag = 0x7000002d
	DT_MIPS_PERF_SUFFIX           DynTag = 0x7000002e
	DT_MIPS_COMPACT_SIZE          DynTag = 0x7000002f
	DT_MIPS_GP_VALUE              DynTag = 0x70000030
	DT_MIPS_AUX_DYNAMIC           DynTag = 0x70000031
	DT_MIPS_PLTGOT                DynTag = 0x70000032
	DT_MIPS_RWPLT                 DynTag = 0x70000034
	DT_MIPS_RLD_MAP_REL           DynTag = 0x70000035

	DT_PPC_GOT DynTag = 0x70000000
	DT_PPC_OPT DynTag = 0x70000001

	DT_PPC64_GLINK DynTag = 0x70000000
	DT_PPC64_OPD   DynTag = 0x70000001
	DT_PPC64_OPDSZ DynTag = 0x70000002
	DT_PPC64_OPT   DynTag = 0x70000003

	DT_SPARC_REGISTER DynTag = 0x70000001

	DT_AUXILIARY DynTag = 0x7ffffffd
	DT_USED      DynTag = 0x7ffffffe
	DT_FILTER    DynTag = 0x7fffffff

	DT_HIPROC DynTag = 0x7fffffff /* Last processor-specific type. */
)

var dtStrings = []intName{
	{0, "DT_NULL"},
	{1, "DT_NEEDED"},
	{2, "DT_PLTRELSZ"},
	{3, "DT_PLTGOT"},
	{4, "DT_HASH"},
	{5, "DT_STRTAB"},
	{6, "DT_SYMTAB"},
	{7, "DT_RELA"},
	{8, "DT_RELASZ"},
	{9, "DT_RELAENT"},
	{10, "DT_STRSZ"},
	{11, "DT_SYMENT"},
	{12, "DT_INIT"},
	{13, "DT_FINI"},
	{14, "DT_SONAME"},
	{15, "DT_RPATH"},
	{16, "DT_SYMBOLIC"},
	{17, "DT_REL"},
	{18, "DT_RELSZ"},
	{19, "DT_RELENT"},
	{20, "DT_PLTREL"},
	{21, "DT_DEBUG"},
	{22, "DT_TEXTREL"},
	{23, "DT_JMPREL"},
	{24, "DT_BIND_NOW"},
	{25, "DT_INIT_ARRAY"},
	{26, "DT_FINI_ARRAY"},
	{27, "DT_INIT_ARRAYSZ"},
	{28, "DT_FINI_ARRAYSZ"},
	{29, "DT_RUNPATH"},
	{30, "DT_FLAGS"},
	{32, "DT_ENCODING"},
	{32, "DT_PREINIT_ARRAY"},
	{33, "DT_PREINIT_ARRAYSZ"},
	{34, "DT_SYMTAB_SHNDX"},
	{0x6000000d, "DT_LOOS"},
	{0x6ffff000, "DT_HIOS"},
	{0x6ffffd00, "DT_VALRNGLO"},
	{0x6ffffdf5, "DT_GNU_PRELINKED"},
	{0x6ffffdf6, "DT_GNU_CONFLICTSZ"},
	{0x6ffffdf7, "DT_GNU_LIBLISTSZ"},
	{0x6ffffdf8, "DT_CHECKSUM"},
	{0x6ffffdf9, "DT_PLTPADSZ"},
	{0x6ffffdfa, "DT_MOVEENT"},
	{0x6ffffdfb, "DT_MOVESZ"},
	{0x6ffffdfc, "DT_FEATURE"},
	{0x6ffffdfd, "DT_POSFLAG_1"},
	{0x6ffffdfe, "DT_SYMINSZ"},
	{0x6ffffdff, "DT_SYMINENT"},
	{0x6ffffdff, "DT_VALRNGHI"},
	{0x6ffffe00, "DT_ADDRRNGLO"},
	{0x6ffffef5, "DT_GNU_HASH"},
	{0x6ffffef6, "DT_TLSDESC_PLT"},
	{0x6ffffef7, "DT_TLSDESC_GOT"},
	{0x6ffffef8, "DT_GNU_CONFLICT"},
	{0x6ffffef9, "DT_GNU_LIBLIST"},
	{0x6ffffefa, "DT_CONFIG"},
	{0x6ffffefb, "DT_DEPAUDIT"},
	{0x6ffffefc, "DT_AUDIT"},
	{0x6ffffefd, "DT_PLTPAD"},
	{0x6ffffefe, "DT_MOVETAB"},
	{0x6ffffeff, "DT_SYMINFO"},
	{0x6ffffeff, "DT_ADDRRNGHI"},
	{0x6ffffff0, "DT_VERSYM"},
	{0x6ffffff9, "DT_RELACOUNT"},
	{0x6ffffffa, "DT_RELCOUNT"},
	{0x6ffffffb, "DT_FLAGS_1"},
	{0x6ffffffc, "DT_VERDEF"},
	{0x6ffffffd, "DT_VERDEFNUM"},
	{0x6ffffffe, "DT_VERNEED"},
	{0x6fffffff, "DT_VERNEEDNUM"},
	{0x70000000, "DT_LOPROC"},
	// We don't list the processor-dependent DynTags,
	// as the values overlap.
	{0x7ffffffd, "DT_AUXILIARY"},
	{0x7ffffffe, "DT_USED"},
	{0x7fffffff, "DT_FILTER"},
}

func (i DynTag) String() string   { return stringName(uint32(i), dtStrings, false) }
func (i DynTag) GoString() string { return stringName(uint32(i), dtStrings, true) }

// DT_FLAGS values.
type DynFlag int

const (
	DF_ORIGIN DynFlag = 0x0001 /* Indicates that the object being loaded may
	   make reference to the
	   $ORIGIN substitution string */
	DF_SYMBOLIC DynFlag = 0x0002 /* Indicates "symbolic" linking. */
	DF_TEXTREL  DynFlag = 0x0004 /* Indicates there may be relocations in non-writable segments. */
	DF_BIND_NOW DynFlag = 0x0008 /* Indicates that the dynamic linker should
	   process all relocations for the object
	   containing this entry before transferring
	   control to the program. */
	DF_STATIC_TLS DynFlag = 0x0010 /* Indicates that the shared object or
	   executable contains code using a static
	   thread-local storage scheme. */
)

var dflagStrings = []intName{
	{0x0001, "DF_ORIGIN"},
	{0x0002, "DF_SYMBOLIC"},
	{0x0004, "DF_TEXTREL"},
	{0x0008, "DF_BIND_NOW"},
	{0x0010, "DF_STATIC_TLS"},
}

func (i DynFlag) String() string   { return flagName(uint32(i), dflagStrings, false) }
func (i DynFlag) GoString() string { return flagName(uint32(i), dflagStrings, true) }

// DT_FLAGS_1 values.
type DynFlag1 uint32

const (
	// Indicates that all relocations for this object must be processed before
	// returning control to the program.
	DF_1_NOW DynFlag1 = 0x00000001
	// Unused.
	DF_1_GLOBAL DynFlag1 = 0x00000002
	// Indicates that the object is a member of a group.
	DF_1_GROUP DynFlag1 = 0x00000004
	// Indicates that the object cannot be deleted from a process.
	DF_1_NODELETE DynFlag1 = 0x00000008
	// Meaningful only for filters. Indicates that all associated filtees be
	// processed immediately.
	DF_1_LOADFLTR DynFlag1 = 0x00000010
	// Indicates that this object's initialization section be run before any other
	// objects loaded.
	DF_1_INITFIRST DynFlag1 = 0x00000020
	// Indicates that the object cannot be added to a running process with dlopen.
	DF_1_NOOPEN DynFlag1 = 0x00000040
	// Indicates the object requires $ORIGIN processing.
	DF_1_ORIGIN DynFlag1 = 0x00000080
	// Indicates that the object should use direct binding information.
	DF_1_DIRECT DynFlag1 = 0x00000100
	// Unused.
	DF_1_TRANS DynFlag1 = 0x00000200
	// Indicates that the objects symbol table is to interpose before all symbols
	// except the primary load object, which is typically the executable.
	DF_1_INTERPOSE DynFlag1 = 0x00000400
	// Indicates that the search for dependencies of this object ignores any
	// default library search paths.
	DF_1_NODEFLIB DynFlag1 = 0x00000800
	// Indicates that this object is not dumped by dldump. Candidates are objects
	// with no relocations that might get included when generating alternative
	// objects using.
	DF_1_NODUMP DynFlag1 = 0x00001000
	// Identifies this object as a configuration alternative object generated by
	// crle. Triggers the runtime linker to search for a configuration file $ORIGIN/ld.config.app-name.
	DF_1_CONFALT DynFlag1 = 0x00002000
	// Meaningful only for filtees. Terminates a filters search for any
	// further filtees.
	DF_1_ENDFILTEE DynFlag1 = 0x00004000
	// Indicates that this object has displacement relocations applied.
	DF_1_DISPRELDNE DynFlag1 = 0x00008000
	// Indicates that this object has displacement relocations pending.
	DF_1_DISPRELPND DynFlag1 = 0x00010000
	// Indicates that this object contains symbols that cannot be directly
	// bound to.
	DF_1_NODIRECT DynFlag1 = 0x00020000
	// Reserved for internal use by the kernel runtime-linker.
	DF_1_IGNMULDEF DynFlag1 = 0x00040000
	// Reserved for internal use by the kernel runtime-linker.
	DF_1_NOKSYMS DynFlag1 = 0x00080000
	// Reserved for internal use by the kernel runtime-linker.
	DF_1_NOHDR DynFlag1 = 0x00100000
	// Indicates that this object has been edited or has been modified since the
	// objects original construction by the link-editor.
	DF_1_EDITED DynFlag1 = 0x00200000
	// Reserved for internal use by the kernel runtime-linker.
	DF_1_NORELOC DynFlag1 = 0x00400000
	// Indicates that the object contains individual symbols that should interpose
	// before all symbols except the primary load object, which is typically the
	// executable.
	DF_1_SYMINTPOSE DynFlag1 = 0x00800000
	// Indicates that the executable requires global auditing.
	DF_1_GLOBAUDIT DynFlag1 = 0x01000000
	// Indicates that the object defines, or makes reference to singleton symbols.
	DF_1_SINGLETON DynFlag1 = 0x02000000
	// Indicates that the object is a stub.
	DF_1_STUB DynFlag1 = 0x04000000
	// Indicates that the object is a position-independent executable.
	DF_1_PIE DynFlag1 = 0x08000000
	// Indicates that the object is a kernel module.
	DF_1_KMOD DynFlag1 = 0x10000000
	// Indicates that the object is a weak standard filter.
	DF_1_WEAKFILTER DynFlag1 = 0x20000000
	// Unused.
	DF_1_NOCOMMON DynFlag1 = 0x40000000
)

var dflag1Strings = []intName{
	{0x00000001, "DF_1_NOW"},
	{0x00000002, "DF_1_GLOBAL"},
	{0x00000004, "DF_1_GROUP"},
	{0x00000008, "DF_1_NODELETE"},
	{0x00000010, "DF_1_LOADFLTR"},
	{0x00000020, "DF_1_INITFIRST"},
	{0x00000040, "DF_1_NOOPEN"},
	{0x00000080, "DF_1_ORIGIN"},
	{0x00000100, "DF_1_DIRECT"},
	{0x00000200, "DF_1_TRANS"},
	{0x00000400, "DF_1_INTERPOSE"},
	{0x00000800, "DF_1_NODEFLIB"},
	{0x00001000, "DF_1_NODUMP"},
	{0x00002000, "DF_1_CONFALT"},
	{0x00004000, "DF_1_ENDFILTEE"},
	{0x00008000, "DF_1_DISPRELDNE"},
	{0x00010000, "DF_1_DISPRELPND"},
	{0x00020000, "DF_1_NODIRECT"},
	{0x00040000, "DF_1_IGNMULDEF"},
	{0x00080000, "DF_1_NOKSYMS"},
	{0x00100000, "DF_1_NOHDR"},
	{0x00200000, "DF_1_EDITED"},
	{0x00400000, "DF_1_NORELOC"},
	{0x00800000, "DF_1_SYMINTPOSE"},
	{0x01000000, "DF_1_GLOBAUDIT"},
	{0x02000000, "DF_1_SINGLETON"},
	{0x04000000, "DF_1_STUB"},
	{0x08000000, "DF_1_PIE"},
	{0x10000000, "DF_1_KMOD"},
	{0x20000000, "DF_1_WEAKFILTER"},
	{0x40000000, "DF_1_NOCOMMON"},
}

func (i DynFlag1) String() string   { return flagName(uint32(i), dflag1Strings, false) }
func (i DynFlag1) GoString() string { return flagName(uint32(i), dflag1Strings, true) }

// NType values; used in core files.
type NType int

const (
	NT_PRSTATUS NType = 1 /* Process status. */
	NT_FPREGSET NType = 2 /* Floating point registers. */
	NT_PRPSINFO NType = 3 /* Process state info. */
)

var ntypeStrings = []intName{
	{1, "NT_PRSTATUS"},
	{2, "NT_FPREGSET"},
	{3, "NT_PRPSINFO"},
}

func (i NType) String() string   { return stringName(uint32(i), ntypeStrings, false) }
func (i NType) GoString() string { return stringName(uint32(i), ntypeStrings, true) }

/* Symbol Binding - ELFNN_ST_BIND - st_info */
type SymBind int

const (
	STB_LOCAL  SymBind = 0  /* Local symbol */
	STB_GLOBAL SymBind = 1  /* Global symbol */
	STB_WEAK   SymBind = 2  /* like global - lower precedence */
	STB_LOOS   SymBind = 10 /* Reserved range for operating system */
	STB_HIOS   SymBind = 12 /*   specific semantics. */
	STB_LOPROC SymBind = 13 /* reserved range for processor */
	STB_HIPROC SymBind = 15 /*   specific semantics. */
)

var stbStrings = []intName{
	{0, "STB_LOCAL"},
	{1, "STB_GLOBAL"},
	{2, "STB_WEAK"},
	{10, "STB_LOOS"},
	{12, "STB_HIOS"},
	{13, "STB_LOPROC"},
	{15, "STB_HIPROC"},
}

func (i SymBind) String() string   { return stringName(uint32(i), stbStrings, false) }
func (i SymBind) GoString() string { return stringName(uint32(i), stbStrings, true) }

/* Symbol type - ELFNN_ST_TYPE - st_info */
type SymType int

const (
	STT_NOTYPE  SymType = 0  /* Unspecified type. */
	STT_OBJECT  SymType = 1  /* Data object. */
	STT_FUNC    SymType = 2  /* Function. */
	STT_SECTION SymType = 3  /* Section. */
	STT_FILE    SymType = 4  /* Source file. */
	STT_COMMON  SymType = 5  /* Uninitialized common block. */
	STT_TLS     SymType = 6  /* TLS object. */
	STT_LOOS    SymType = 10 /* Reserved range for operating system */
	STT_HIOS    SymType = 12 /*   specific semantics. */
	STT_LOPROC  SymType = 13 /* reserved range for processor */
	STT_HIPROC  SymType = 15 /*   specific semantics. */

	/* Non-standard symbol types. */
	STT_RELC      SymType = 8  /* Complex relocation expression. */
	STT_SRELC     SymType = 9  /* Signed complex relocation expression. */
	STT_GNU_IFUNC SymType = 10 /* Indirect code object. */
)

var sttStrings = []intName{
	{0, "STT_NOTYPE"},
	{1, "STT_OBJECT"},
	{2, "STT_FUNC"},
	{3, "STT_SECTION"},
	{4, "STT_FILE"},
	{5, "STT_COMMON"},
	{6, "STT_TLS"},
	{8, "STT_RELC"},
	{9, "STT_SRELC"},
	{10, "STT_LOOS"},
	{12, "STT_HIOS"},
	{13, "STT_LOPROC"},
	{15, "STT_HIPROC"},
}

func (i SymType) String() string   { return stringName(uint32(i), sttStrings, false) }
func (i SymType) GoString() string { return stringName(uint32(i), sttStrings, true) }

/* Symbol visibility - ELFNN_ST_VISIBILITY - st_other */
type SymVis int

const (
	STV_DEFAULT   SymVis = 0x0 /* Default visibility (see binding). */
	STV_INTERNAL  SymVis = 0x1 /* Special meaning in relocatable objects. */
	STV_HIDDEN    SymVis = 0x2 /* Not visible. */
	STV_PROTECTED SymVis = 0x3 /* Visible but not preemptible. */
)

var stvStrings = []intName{
	{0x0, "STV_DEFAULT"},
	{0x1, "STV_INTERNAL"},
	{0x2, "STV_HIDDEN"},
	{0x3, "STV_PROTECTED"},
}

func (i SymVis) String() string   { return stringName(uint32(i), stvStrings, false) }
func (i SymVis) GoString() string { return stringName(uint32(i), stvStrings, true) }

/*
 * Relocation types.
 */

// Relocation types for x86-64.
type R_X86_64 int

const (
	R_X86_64_NONE            R_X86_64 = 0  /* No relocation. */
	R_X86_64_64              R_X86_64 = 1  /* Add 64 bit symbol value. */
	R_X86_64_PC32            R_X86_64 = 2  /* PC-relative 32 bit signed sym value. */
	R_X86_64_GOT32           R_X86_64 = 3  /* PC-relative 32 bit GOT offset. */
	R_X86_64_PLT32           R_X86_64 = 4  /* PC-relative 32 bit PLT offset. */
	R_X86_64_COPY            R_X86_64 = 5  /* Copy data from shared object. */
	R_X86_64_GLOB_DAT        R_X86_64 = 6  /* Set GOT entry to data address. */
	R_X86_64_JMP_SLOT        R_X86_64 = 7  /* Set GOT entry to code address. */
	R_X86_64_RELATIVE        R_X86_64 = 8  /* Add load address of shared object. */
	R_X86_64_GOTPCREL        R_X86_64 = 9  /* Add 32 bit signed pcrel offset to GOT. */
	R_X86_64_32              R_X86_64 = 10 /* Add 32 bit zero extended symbol value */
	R_X86_64_32S             R_X86_64 = 11 /* Add 32 bit sign extended symbol value */
	R_X86_64_16              R_X86_64 = 12 /* Add 16 bit zero extended symbol value */
	R_X86_64_PC16            R_X86_64 = 13 /* Add 16 bit signed extended pc relative symbol value */
	R_X86_64_8               R_X86_64 = 14 /* Add 8 bit zero extended symbol value */
	R_X86_64_PC8             R_X86_64 = 15 /* Add 8 bit signed extended pc relative symbol value */
	R_X86_64_DTPMOD64        R_X86_64 = 16 /* ID of module containing symbol */
	R_X86_64_DTPOFF64        R_X86_64 = 17 /* Offset in TLS block */
	R_X86_64_TPOFF64         R_X86_64 = 18 /* Offset in static TLS block */
	R_X86_64_TLSGD           R_X86_64 = 19 /* PC relative offset to GD GOT entry */
	R_X86_64_TLSLD           R_X86_64 = 20 /* PC relative offset to LD GOT entry */
	R_X86_64_DTPOFF32        R_X86_64 = 21 /* Offset in TLS block */
	R_X86_64_GOTTPOFF        R_X86_64 = 22 /* PC relative offset to IE GOT entry */
	R_X86_64_TPOFF32         R_X86_64 = 23 /* Offset in static TLS block */
	R_X86_64_PC64            R_X86_64 = 24 /* PC relative 64-bit sign extended symbol value. */
	R_X86_64_GOTOFF64        R_X86_64 = 25
	R_X86_64_GOTPC32         R_X86_64 = 26
	R_X86_64_GOT64           R_X86_64 = 27
	R_X86_64_GOTPCREL64      R_X86_64 = 28
	R_X86_64_GOTPC64         R_X86_64 = 29
	R_X86_64_GOTPLT64        R_X86_64 = 30
	R_X86_64_PLTOFF64        R_X86_64 = 31
	R_X86_64_SIZE32          R_X86_64 = 32
	R_X86_64_SIZE64          R_X86_64 = 33
	R_X86_64_GOTPC32_TLSDESC R_X86_64 = 34
	R_X86_64_TLSDESC_CALL    R_X86_64 = 35
	R_X86_64_TLSDESC         R_X86_64 = 36
	R_X86_64_IRELATIVE       R_X86_64 = 37
	R_X86_64_RELATIVE64      R_X86_64 = 38
	R_X86_64_PC32_BND        R_X86_64 = 39
	R_X86_64_PLT32_BND       R_X86_64 = 40
	R_X86_64_GOTPCRELX       R_X86_64 = 41
	R_X86_64_REX_GOTPCRELX   R_X86_64 = 42
)

var rx86_64Strings = []intName{
	{0, "R_X86_64_NONE"},
	{1, "R_X86_64_64"},
	{2, "R_X86_64_PC32"},
	{3, "R_X86_64_GOT32"},
	{4, "R_X86_64_PLT32"},
	{5, "R_X86_64_COPY"},
	{6, "R_X86_64_GLOB_DAT"},
	{7, "R_X86_64_JMP_SLOT"},
	{8, "R_X86_64_RELATIVE"},
	{9, "R_X86_64_GOTPCREL"},
	{10, "R_X86_64_32"},
	{11, "R_X86_64_32S"},
	{12, "R_X86_64_16"},
	{13, "R_X86_64_PC16"},
	{14, "R_X86_64_8"},
	{15, "R_X86_64_PC8"},
	{16, "R_X86_64_DTPMOD64"},
	{17, "R_X86_64_DTPOFF64"},
	{18, "R_X86_64_TPOFF64"},
	{19, "R_X86_64_TLSGD"},
	{20, "R_X86_64_TLSLD"},
	{21, "R_X86_64_DTPOFF32"},
	{22, "R_X86_64_GOTTPOFF"},
	{23, "R_X86_64_TPOFF32"},
	{24, "R_X86_64_PC64"},
	{25, "R_X86_64_GOTOFF64"},
	{26, "R_X86_64_GOTPC32"},
	{27, "R_X86_64_GOT64"},
	{28, "R_X86_64_GOTPCREL64"},
	{29, "R_X86_64_GOTPC64"},
	{30, "R_X86_64_GOTPLT64"},
	{31, "R_X86_64_PLTOFF64"},
	{32, "R_X86_64_SIZE32"},
	{33, "R_X86_64_SIZE64"},
	{34, "R_X86_64_GOTPC32_TLSDESC"},
	{35, "R_X86_64_TLSDESC_CALL"},
	{36, "R_X86_64_TLSDESC"},
	{37, "R_X86_64_IRELATIVE"},
	{38, "R_X86_64_RELATIVE64"},
	{39, "R_X86_64_PC32_BND"},
	{40, "R_X86_64_PLT32_BND"},
	{41, "R_X86_64_GOTPCRELX"},
	{42, "R_X86_64_REX_GOTPCRELX"},
}

func (i R_X86_64) String() string   { return stringName(uint32(i), rx86_64Strings, false) }
func (i R_X86_64) GoString() string { return stringName(uint32(i), rx86_64Strings, true) }

// Relocation types for AArch64 (aka arm64)
type R_AARCH64 int

const (
	R_AARCH64_NONE                            R_AARCH64 = 0
	R_AARCH64_P32_ABS32                       R_AARCH64 = 1
	R_AARCH64_P32_ABS16                       R_AARCH64 = 2
	R_AARCH64_P32_PREL32                      R_AARCH64 = 3
	R_AARCH64_P32_PREL16                      R_AARCH64 = 4
	R_AARCH64_P32_MOVW_UABS_G0                R_AARCH64 = 5
	R_AARCH64_P32_MOVW_UABS_G0_NC             R_AARCH64 = 6
	R_AARCH64_P32_MOVW_UABS_G1                R_AARCH64 = 7
	R_AARCH64_P32_MOVW_SABS_G0                R_AARCH64 = 8
	R_AARCH64_P32_LD_PREL_LO19                R_AARCH64 = 9
	R_AARCH64_P32_ADR_PREL_LO21               R_AARCH64 = 10
	R_AARCH64_P32_ADR_PREL_PG_HI21            R_AARCH64 = 11
	R_AARCH64_P32_ADD_ABS_LO12_NC             R_AARCH64 = 12
	R_AARCH64_P32_LDST8_ABS_LO12_NC           R_AARCH64 = 13
	R_AARCH64_P32_LDST16_ABS_LO12_NC          R_AARCH64 = 14
	R_AARCH64_P32_LDST32_ABS_LO12_NC          R_AARCH64 = 15
	R_AARCH64_P32_LDST64_ABS_LO12_NC          R_AARCH64 = 16
	R_AARCH64_P32_LDST128_ABS_LO12_NC         R_AARCH64 = 17
	R_AARCH64_P32_TSTBR14                     R_AARCH64 = 18
	R_AARCH64_P32_CONDBR19                    R_AARCH64 = 19
	R_AARCH64_P32_JUMP26                      R_AARCH64 = 20
	R_AARCH64_P32_CALL26                      R_AARCH64 = 21
	R_AARCH64_P32_GOT_LD_PREL19               R_AARCH64 = 25
	R_AARCH64_P32_ADR_GOT_PAGE                R_AARCH64 = 26
	R_AARCH64_P32_LD32_GOT_LO12_NC            R_AARCH64 = 27
	R_AARCH64_P32_TLSGD_ADR_PAGE21            R_AARCH64 = 81
	R_AARCH64_P32_TLSGD_ADD_LO12_NC           R_AARCH64 = 82
	R_AARCH64_P32_TLSIE_ADR_GOTTPREL_PAGE21   R_AARCH64 = 103
	R_AARCH64_P32_TLSIE_LD32_GOTTPREL_LO12_NC R_AARCH64 = 104
	R_AARCH64_P32_TLSIE_LD_GOTTPREL_PREL19    R_AARCH64 = 105
	R_AARCH64_P32_TLSLE_MOVW_TPREL_G1         R_AARCH64 = 106
	R_AARCH64_P32_TLSLE_MOVW_TPREL_G0         R_AARCH64 = 107
	R_AARCH64_P32_TLSLE_MOVW_TPREL_G0_NC      R_AARCH64 = 108
	R_AARCH64_P32_TLSLE_ADD_TPREL_HI12        R_AARCH64 = 109
	R_AARCH64_P32_TLSLE_ADD_TPREL_LO12        R_AARCH64 = 110
	R_AARCH64_P32_TLSLE_ADD_TPREL_LO12_NC     R_AARCH64 = 111
	R_AARCH64_P32_TLSDESC_LD_PREL19           R_AARCH64 = 122
	R_AARCH64_P32_TLSDESC_ADR_PREL21          R_AARCH64 = 123
	R_AARCH64_P32_TLSDESC_ADR_PAGE21          R_AARCH64 = 124
	R_AARCH64_P32_TLSDESC_LD32_LO12_NC        R_AARCH64 = 125
	R_AARCH64_P32_TLSDESC_ADD_LO12_NC         R_AARCH64 = 126
	R_AARCH64_P32_TLSDESC_CALL                R_AARCH64 = 127
	R_AARCH64_P32_COPY                        R_AARCH64 = 180
	R_AARCH64_P32_GLOB_DAT                    R_AARCH64 = 181
	R_AARCH64_P32_JUMP_SLOT                   R_AARCH64 = 182
	R_AARCH64_P32_RELATIVE                    R_AARCH64 = 183
	R_AARCH64_P32_TLS_DTPMOD                  R_AARCH64 = 184
	R_AARCH64_P32_TLS_DTPREL                  R_AARCH64 = 185
	R_AARCH64_P32_TLS_TPREL                   R_AARCH64 = 186
	R_AARCH64_P32_TLSDESC                     R_AARCH64 = 187
	R_AARCH64_P32_IRELATIVE                   R_AARCH64 = 188
	R_AARCH64_NULL                            R_AARCH64 = 256
	R_AARCH64_ABS64                           R_AARCH64 = 257
	R_AARCH64_ABS32                           R_AARCH64 = 258
	R_AARCH64_ABS16                           R_AARCH64 = 259
	R_AARCH64_PREL64                          R_AARCH64 = 260
	R_AARCH64_PREL32                          R_AARCH64 = 261
	R_AARCH64_PREL16                          R_AARCH64 = 262
	R_AARCH64_MOVW_UABS_G0                    R_AARCH64 = 263
	R_AARCH64_MOVW_UABS_G0_NC                 R_AARCH64 = 264
	R_AARCH64_MOVW_UABS_G1                    R_AARCH64 = 265
	R_AARCH64_MOVW_UABS_G1_NC                 R_AARCH64 = 266
	R_AARCH64_MOVW_UABS_G2                    R_AARCH64 = 267
	R_AARCH64_MOVW_UABS_G2_NC                 R_AARCH64 = 268
	R_AARCH64_MOVW_UABS_G3                    R_AARCH64 = 269
	R_AARCH64_MOVW_SABS_G0                    R_AARCH64 = 270
	R_AARCH64_MOVW_SABS_G1                    R_AARCH64 = 271
	R_AARCH64_MOVW_SABS_G2                    R_AARCH64 = 272
	R_AARCH64_LD_PREL_LO19                    R_AARCH64 = 273
	R_AARCH64_ADR_PREL_LO21                   R_AARCH64 = 274
	R_AARCH64_ADR_PREL_PG_HI21                R_AARCH64 = 275
	R_AARCH64_ADR_PREL_PG_HI21_NC             R_AARCH64 = 276
	R_AARCH64_ADD_ABS_LO12_NC                 R_AARCH64 = 277
	R_AARCH64_LDST8_ABS_LO12_NC               R_AARCH64 = 278
	R_AARCH64_TSTBR14                         R_AARCH64 = 279
	R_AARCH64_CONDBR19                        R_AARCH64 = 280
	R_AARCH64_JUMP26                          R_AARCH64 = 282
	R_AARCH64_CALL26                          R_AARCH64 = 283
	R_AARCH64_LDST16_ABS_LO12_NC              R_AARCH64 = 284
	R_AARCH64_LDST32_ABS_LO12_NC              R_AARCH64 = 285
	R_AARCH64_LDST64_ABS_LO12_NC              R_AARCH64 = 286
	R_AARCH64_LDST128_ABS_LO12_NC             R_AARCH64 = 299
	R_AARCH64_GOT_LD_PREL19                   R_AARCH64 = 309
	R_AARCH64_LD64_GOTOFF_LO15                R_AARCH64 = 310
	R_AARCH64_ADR_GOT_PAGE                    R_AARCH64 = 311
	R_AARCH64_LD64_GOT_LO12_NC                R_AARCH64 = 312
	R_AARCH64_LD64_GOTPAGE_LO15               R_AARCH64 = 313
	R_AARCH64_TLSGD_ADR_PREL21                R_AARCH64 = 512
	R_AARCH64_TLSGD_ADR_PAGE21                R_AARCH64 = 513
	R_AARCH64_TLSGD_ADD_LO12_NC               R_AARCH64 = 514
	R_AARCH64_TLSGD_MOVW_G1                   R_AARCH64 = 515
	R_AARCH64_TLSGD_MOVW_G0_NC                R_AARCH64 = 516
	R_AARCH64_TLSLD_ADR_PREL21                R_AARCH64 = 517
	R_AARCH64_TLSLD_ADR_PAGE21                R_AARCH64 = 518
	R_AARCH64_TLSIE_MOVW_GOTTPREL_G1          R_AARCH64 = 539
	R_AARCH64_TLSIE_MOVW_GOTTPREL_G0_NC       R_AARCH64 = 540
	R_AARCH64_TLSIE_ADR_GOTTPREL_PAGE21       R_AARCH64 = 541
	R_AARCH64_TLSIE_LD64_GOTTPREL_LO12_NC     R_AARCH64 = 542
	R_AARCH64_TLSIE_LD_GOTTPREL_PREL19        R_AARCH64 = 543
	R_AARCH64_TLSLE_MOVW_TPREL_G2             R_AARCH64 = 544
	R_AARCH64_TLSLE_MOVW_TPREL_G1             R_AARCH64 = 545
	R_AARCH64_TLSLE_MOVW_TPREL_G1_NC          R_AARCH64 = 546
	R_AARCH64_TLSLE_MOVW_TPREL_G0             R_AARCH64 = 547
	R_AARCH64_TLSLE_MOVW_TPREL_G0_NC          R_AARCH64 = 548
	R_AARCH64_TLSLE_ADD_TPREL_HI12            R_AARCH64 = 549
	R_AARCH64_TLSLE_ADD_TPREL_LO12            R_AARCH64 = 550
	R_AARCH64_TLSLE_ADD_TPREL_LO12_NC         R_AARCH64 = 551
	R_AARCH64_TLSDESC_LD_PREL19               R_AARCH64 = 560
	R_AARCH64_TLSDESC_ADR_PREL21              R_AARCH64 = 561
	R_AARCH64_TLSDESC_ADR_PAGE21              R_AARCH64 = 562
	R_AARCH64_TLSDESC_LD64_LO12_NC            R_AARCH64 = 563
	R_AARCH64_TLSDESC_ADD_LO12_NC             R_AARCH64 = 564
	R_AARCH64_TLSDESC_OFF_G1                  R_AARCH64 = 565
	R_AARCH64_TLSDESC_OFF_G0_NC               R_AARCH64 = 566
	R_AARCH64_TLSDESC_LDR                     R_AARCH64 = 567
	R_AARCH64_TLSDESC_ADD                     R_AARCH64 = 568
	R_AARCH64_TLSDESC_CALL                    R_AARCH64 = 569
	R_AARCH64_TLSLE_LDST128_TPREL_LO12        R_AARCH64 = 570
	R_AARCH64_TLSLE_LDST128_TPREL_LO12_NC     R_AARCH64 = 571
	R_AARCH64_TLSLD_LDST128_DTPREL_LO12       R_AARCH64 = 572
	R_AARCH64_TLSLD_LDST128_DTPREL_LO12_NC    R_AARCH64 = 573
	R_AARCH64_COPY                            R_AARCH64 = 1024
	R_AARCH64_GLOB_DAT                        R_AARCH64 = 1025
	R_AARCH64_JUMP_SLOT                       R_AARCH64 = 1026
	R_AARCH64_RELATIVE                        R_AARCH64 = 1027
	R_AARCH64_TLS_DTPMOD64                    R_AARCH64 = 1028
	R_AARCH64_TLS_DTPREL64                    R_AARCH64 = 1029
	R_AARCH64_TLS_TPREL64                     R_AARCH64 = 1030
	R_AARCH64_TLSDESC                         R_AARCH64 = 1031
	R_AARCH64_IRELATIVE                       R_AARCH64 = 1032
)

var raarch64Strings = []intName{
	{0, "R_AARCH64_NONE"},
	{1, "R_AARCH64_P32_ABS32"},
	{2, "R_AARCH64_P32_ABS16"},
	{3, "R_AARCH64_P32_PREL32"},
	{4, "R_AARCH64_P32_PREL16"},
	{5, "R_AARCH64_P32_MOVW_UABS_G0"},
	{6, "R_AARCH64_P32_MOVW_UABS_G0_NC"},
	{7, "R_AARCH64_P32_MOVW_UABS_G1"},
	{8, "R_AARCH64_P32_MOVW_SABS_G0"},
	{9, "R_AARCH64_P32_LD_PREL_LO19"},
	{10, "R_AARCH64_P32_ADR_PREL_LO21"},
	{11, "R_AARCH64_P32_ADR_PREL_PG_HI21"},
	{12, "R_AARCH64_P32_ADD_ABS_LO12_NC"},
	{13, "R_AARCH64_P32_LDST8_ABS_LO12_NC"},
	{14, "R_AARCH64_P32_LDST16_ABS_LO12_NC"},
	{15, "R_AARCH64_P32_LDST32_ABS_LO12_NC"},
	{16, "R_AARCH64_P32_LDST64_ABS_LO12_NC"},
	{17, "R_AARCH64_P32_LDST128_ABS_LO12_NC"},
	{18, "R_AARCH64_P32_TSTBR14"},
	{19, "R_AARCH64_P32_CONDBR19"},
	{20, "R_AARCH64_P32_JUMP26"},
	{21, "R_AARCH64_P32_CALL26"},
	{25, "R_AARCH64_P32_GOT_LD_PREL19"},
	{26, "R_AARCH64_P32_ADR_GOT_PAGE"},
	{27, "R_AARCH64_P32_LD32_GOT_LO12_NC"},
	{81, "R_AARCH64_P32_TLSGD_ADR_PAGE21"},
	{82, "R_AARCH64_P32_TLSGD_ADD_LO12_NC"},
	{103, "R_AARCH64_P32_TLSIE_ADR_GOTTPREL_PAGE21"},
	{104, "R_AARCH64_P32_TLSIE_LD32_GOTTPREL_LO12_NC"},
	{105, "R_AARCH64_P32_TLSIE_LD_GOTTPREL_PREL19"},
	{106, "R_AARCH64_P32_TLSLE_MOVW_TPREL_G1"},
	{107, "R_AARCH64_P32_TLSLE_MOVW_TPREL_G0"},
	{108, "R_AARCH64_P32_TLSLE_MOVW_TPREL_G0_NC"},
	{109, "R_AARCH64_P32_TLSLE_ADD_TPREL_HI12"},
	{110, "R_AARCH64_P32_TLSLE_ADD_TPREL_LO12"},
	{111, "R_AARCH64_P32_TLSLE_ADD_TPREL_LO12_NC"},
	{122, "R_AARCH64_P32_TLSDESC_LD_PREL19"},
	{123, "R_AARCH64_P32_TLSDESC_ADR_PREL21"},
	{124, "R_AARCH64_P32_TLSDESC_ADR_PAGE21"},
	{125, "R_AARCH64_P32_TLSDESC_LD32_LO12_NC"},
	{126, "R_AARCH64_P32_TLSDESC_ADD_LO12_NC"},
	{127, "R_AARCH64_P32_TLSDESC_CALL"},
	{180, "R_AARCH64_P32_COPY"},
	{181, "R_AARCH64_P32_GLOB_DAT"},
	{182, "R_AARCH64_P32_JUMP_SLOT"},
	{183, "R_AARCH64_P32_RELATIVE"},
	{184, "R_AARCH64_P32_TLS_DTPMOD"},
	{185, "R_AARCH64_P32_TLS_DTPREL"},
	{186, "R_AARCH64_P32_TLS_TPREL"},
	{187, "R_AARCH64_P32_TLSDESC"},
	{188, "R_AARCH64_P32_IRELATIVE"},
	{256, "R_AARCH64_NULL"},
	{257, "R_AARCH64_ABS64"},
	{258, "R_AARCH64_ABS32"},
	{259, "R_AARCH64_ABS16"},
	{260, "R_AARCH64_PREL64"},
	{261, "R_AARCH64_PREL32"},
	{262, "R_AARCH64_PREL16"},
	{263, "R_AARCH64_MOVW_UABS_G0"},
	{264, "R_AARCH64_MOVW_UABS_G0_NC"},
	{265, "R_AARCH64_MOVW_UABS_G1"},
	{266, "R_AARCH64_MOVW_UABS_G1_NC"},
	{267, "R_AARCH64_MOVW_UABS_G2"},
	{268, "R_AARCH64_MOVW_UABS_G2_NC"},
	{269, "R_AARCH64_MOVW_UABS_G3"},
	{270, "R_AARCH64_MOVW_SABS_G0"},
	{271, "R_AARCH64_MOVW_SABS_G1"},
	{272, "R_AARCH64_MOVW_SABS_G2"},
	{273, "R_AARCH64_LD_PREL_LO19"},
	{274, "R_AARCH64_ADR_PREL_LO21"},
	{275, "R_AARCH64_ADR_PREL_PG_HI21"},
	{276, "R_AARCH64_ADR_PREL_PG_HI21_NC"},
	{277, "R_AARCH64_ADD_ABS_LO12_NC"},
	{278, "R_AARCH64_LDST8_ABS_LO12_NC"},
	{279, "R_AARCH64_TSTBR14"},
	{280, "R_AARCH64_CONDBR19"},
	{282, "R_AARCH64_JUMP26"},
	{283, "R_AARCH64_CALL26"},
	{284, "R_AARCH64_LDST16_ABS_LO12_NC"},
	{285, "R_AARCH64_LDST32_ABS_LO12_NC"},
	{286, "R_AARCH64_LDST64_ABS_LO12_NC"},
	{299, "R_AARCH64_LDST128_ABS_LO12_NC"},
	{309, "R_AARCH64_GOT_LD_PREL19"},
	{310, "R_AARCH64_LD64_GOTOFF_LO15"},
	{311, "R_AARCH64_ADR_GOT_PAGE"},
	{312, "R_AARCH64_LD64_GOT_LO12_NC"},
	{313, "R_AARCH64_LD64_GOTPAGE_LO15"},
	{512, "R_AARCH64_TLSGD_ADR_PREL21"},
	{513, "R_AARCH64_TLSGD_ADR_PAGE21"},
	{514, "R_AARCH64_TLSGD_ADD_LO12_NC"},
	{515, "R_AARCH64_TLSGD_MOVW_G1"},
	{516, "R_AARCH64_TLSGD_MOVW_G0_NC"},
	{517, "R_AARCH64_TLSLD_ADR_PREL21"},
	{518, "R_AARCH64_TLSLD_ADR_PAGE21"},
	{539, "R_AARCH64_TLSIE_MOVW_GOTTPREL_G1"},
	{540, "R_AARCH64_TLSIE_MOVW_GOTTPREL_G0_NC"},
	{541, "R_AARCH64_TLSIE_ADR_GOTTPREL_PAGE21"},
	{542, "R_AARCH64_TLSIE_LD64_GOTTPREL_LO12_NC"},
	{543, "R_AARCH64_TLSIE_LD_GOTTPREL_PREL19"},
	{544, "R_AARCH64_TLSLE_MOVW_TPREL_G2"},
	{545, "R_AARCH64_TLSLE_MOVW_TPREL_G1"},
	{546, "R_AARCH64_TLSLE_MOVW_TPREL_G1_NC"},
	{547, "R_AARCH64_TLSLE_MOVW_TPREL_G0"},
	{548, "R_AARCH64_TLSLE_MOVW_TPREL_G0_NC"},
	{549, "R_AARCH64_TLSLE_ADD_TPREL_HI12"},
	{550, "R_AARCH64_TLSLE_ADD_TPREL_LO12"},
	{551, "R_AARCH64_TLSLE_ADD_TPREL_LO12_NC"},
	{560, "R_AARCH64_TLSDESC_LD_PREL19"},
	{561, "R_AARCH64_TLSDESC_ADR_PREL21"},
	{562, "R_AARCH64_TLSDESC_ADR_PAGE21"},
	{563, "R_AARCH64_TLSDESC_LD64_LO12_NC"},
	{564, "R_AARCH64_TLSDESC_ADD_LO12_NC"},
	{565, "R_AARCH64_TLSDESC_OFF_G1"},
	{566, "R_AARCH64_TLSDESC_OFF_G0_NC"},
	{567, "R_AARCH64_TLSDESC_LDR"},
	{568, "R_AARCH64_TLSDESC_ADD"},
	{569, "R_AARCH64_TLSDESC_CALL"},
	{570, "R_AARCH64_TLSLE_LDST128_TPREL_LO12"},
	{571, "R_AARCH64_TLSLE_LDST128_TPREL_LO12_NC"},
	{572, "R_AARCH64_TLSLD_LDST128_DTPREL_LO12"},
	{573, "R_AARCH64_TLSLD_LDST128_DTPREL_LO12_NC"},
	{1024, "R_AARCH64_COPY"},
	{1025, "R_AARCH64_GLOB_DAT"},
	{1026, "R_AARCH64_JUMP_SLOT"},
	{1027, "R_AARCH64_RELATIVE"},
	{1028, "R_AARCH64_TLS_DTPMOD64"},
	{1029, "R_AARCH64_TLS_DTPREL64"},
	{1030, "R_AARCH64_TLS_TPREL64"},
	{1031, "R_AARCH64_TLSDESC"},
	{1032, "R_AARCH64_IRELATIVE"},
}

func (i R_AARCH64) String() string   { return stringName(uint32(i), raarch64Strings, false) }
func (i R_AARCH64) GoString() string { return stringName(uint32(i), raarch64Strings, true) }

// Relocation types for Alpha.
type R_ALPHA int

const (
	R_ALPHA_NONE           R_ALPHA = 0  /* No reloc */
	R_ALPHA_REFLONG        R_ALPHA = 1  /* Direct 32 bit */
	R_ALPHA_REFQUAD        R_ALPHA = 2  /* Direct 64 bit */
	R_ALPHA_GPREL32        R_ALPHA = 3  /* GP relative 32 bit */
	R_ALPHA_LITERAL        R_ALPHA = 4  /* GP relative 16 bit w/optimization */
	R_ALPHA_LITUSE         R_ALPHA = 5  /* Optimization hint for LITERAL */
	R_ALPHA_GPDISP         R_ALPHA = 6  /* Add displacement to GP */
	R_ALPHA_BRADDR         R_ALPHA = 7  /* PC+4 relative 23 bit shifted */
	R_ALPHA_HINT           R_ALPHA = 8  /* PC+4 relative 16 bit shifted */
	R_ALPHA_SREL16         R_ALPHA = 9  /* PC relative 16 bit */
	R_ALPHA_SREL32         R_ALPHA = 10 /* PC relative 32 bit */
	R_ALPHA_SREL64         R_ALPHA = 11 /* PC relative 64 bit */
	R_ALPHA_OP_PUSH        R_ALPHA = 12 /* OP stack push */
	R_ALPHA_OP_STORE       R_ALPHA = 13 /* OP stack pop and store */
	R_ALPHA_OP_PSUB        R_ALPHA = 14 /* OP stack subtract */
	R_ALPHA_OP_PRSHIFT     R_ALPHA = 15 /* OP stack right shift */
	R_ALPHA_GPVALUE        R_ALPHA = 16
	R_ALPHA_GPRELHIGH      R_ALPHA = 17
	R_ALPHA_GPRELLOW       R_ALPHA = 18
	R_ALPHA_IMMED_GP_16    R_ALPHA = 19
	R_ALPHA_IMMED_GP_HI32  R_ALPHA = 20
	R_ALPHA_IMMED_SCN_HI32 R_ALPHA = 21
	R_ALPHA_IMMED_BR_HI32  R_ALPHA = 22
	R_ALPHA_IMMED_LO32     R_ALPHA = 23
	R_ALPHA_COPY           R_ALPHA = 24 /* Copy symbol at runtime */
	R_ALPHA_GLOB_DAT       R_ALPHA = 25 /* Create GOT entry */
	R_ALPHA_JMP_SLOT       R_ALPHA = 26 /* Create PLT entry */
	R_ALPHA_RELATIVE       R_ALPHA = 27 /* Adjust by program base */
)

var ralphaStrings = []intName{
	{0, "R_ALPHA_NONE"},
	{1, "R_ALPHA_REFLONG"},
	{2, "R_ALPHA_REFQUAD"},
	{3, "R_ALPHA_GPREL32"},
	{4, "R_ALPHA_LITERAL"},
	{5, "R_ALPHA_LITUSE"},
	{6, "R_ALPHA_GPDISP"},
	{7, "R_ALPHA_BRADDR"},
	{8, "R_ALPHA_HINT"},
	{9, "R_ALPHA_SREL16"},
	{10, "R_ALPHA_SREL32"},
	{11, "R_ALPHA_SREL64"},
	{12, "R_ALPHA_OP_PUSH"},
	{13, "R_ALPHA_OP_STORE"},
	{14, "R_ALPHA_OP_PSUB"},
	{15, "R_ALPHA_OP_PRSHIFT"},
	{16, "R_ALPHA_GPVALUE"},
	{17, "R_ALPHA_GPRELHIGH"},
	{18, "R_ALPHA_GPRELLOW"},
	{19, "R_ALPHA_IMMED_GP_16"},
	{20, "R_ALPHA_IMMED_GP_HI32"},
	{21, "R_ALPHA_IMMED_SCN_HI32"},
	{22, "R_ALPHA_IMMED_BR_HI32"},
	{23, "R_ALPHA_IMMED_LO32"},
	{24, "R_ALPHA_COPY"},
	{25, "R_ALPHA_GLOB_DAT"},
	{26, "R_ALPHA_JMP_SLOT"},
	{27, "R_ALPHA_RELATIVE"},
}

func (i R_ALPHA) String() string   { return stringName(uint32(i), ralphaStrings, false) }
func (i R_ALPHA) GoString() string { return stringName(uint32(i), ralphaStrings, true) }

// Relocation types for ARM.
type R_ARM int

const (
	R_ARM_NONE               R_ARM = 0 /* No relocation. */
	R_ARM_PC24               R_ARM = 1
	R_ARM_ABS32              R_ARM = 2
	R_ARM_REL32              R_ARM = 3
	R_ARM_PC13               R_ARM = 4
	R_ARM_ABS16              R_ARM = 5
	R_ARM_ABS12              R_ARM = 6
	R_ARM_THM_ABS5           R_ARM = 7
	R_ARM_ABS8               R_ARM = 8
	R_ARM_SBREL32            R_ARM = 9
	R_ARM_THM_PC22           R_ARM = 10
	R_ARM_THM_PC8            R_ARM = 11
	R_ARM_AMP_VCALL9         R_ARM = 12
	R_ARM_SWI24              R_ARM = 13
	R_ARM_THM_SWI8           R_ARM = 14
	R_ARM_XPC25              R_ARM = 15
	R_ARM_THM_XPC22          R_ARM = 16
	R_ARM_TLS_DTPMOD32       R_ARM = 17
	R_ARM_TLS_DTPOFF32       R_ARM = 18
	R_ARM_TLS_TPOFF32        R_ARM = 19
	R_ARM_COPY               R_ARM = 20 /* Copy data from shared object. */
	R_ARM_GLOB_DAT           R_ARM = 21 /* Set GOT entry to data address. */
	R_ARM_JUMP_SLOT          R_ARM = 22 /* Set GOT entry to code address. */
	R_ARM_RELATIVE           R_ARM = 23 /* Add load address of shared object. */
	R_ARM_GOTOFF             R_ARM = 24 /* Add GOT-relative symbol address. */
	R_ARM_GOTPC              R_ARM = 25 /* Add PC-relative GOT table address. */
	R_ARM_GOT32              R_ARM = 26 /* Add PC-relative GOT offset. */
	R_ARM_PLT32              R_ARM = 27 /* Add PC-relative PLT offset. */
	R_ARM_CALL               R_ARM = 28
	R_ARM_JUMP24             R_ARM = 29
	R_ARM_THM_JUMP24         R_ARM = 30
	R_ARM_BASE_ABS           R_ARM = 31
	R_ARM_ALU_PCREL_7_0      R_ARM = 32
	R_ARM_ALU_PCREL_15_8     R_ARM = 33
	R_ARM_ALU_PCREL_23_15    R_ARM = 34
	R_ARM_LDR_SBREL_11_10_NC R_ARM = 35
	R_ARM_ALU_SBREL_19_12_NC R_ARM = 36
	R_ARM_ALU_SBREL_27_20_CK R_ARM = 37
	R_ARM_TARGET1            R_ARM = 38
	R_ARM_SBREL31            R_ARM = 39
	R_ARM_V4BX               R_ARM = 40
	R_ARM_TARGET2            R_ARM = 41
	R_ARM_PREL31             R_ARM = 42
	R_ARM_MOVW_ABS_NC        R_ARM = 43
	R_ARM_MOVT_ABS           R_ARM = 44
	R_ARM_MOVW_PREL_NC       R_ARM = 45
	R_ARM_MOVT_PREL          R_ARM = 46
	R_ARM_THM_MOVW_ABS_NC    R_ARM = 47
	R_ARM_THM_MOVT_ABS       R_ARM = 48
	R_ARM_THM_MOVW_PREL_NC   R_ARM = 49
	R_ARM_THM_MOVT_PREL      R_ARM = 50
	R_ARM_THM_JUMP19         R_ARM = 51
	R_ARM_THM_JUMP6          R_ARM = 52
	R_ARM_THM_ALU_PREL_11_0  R_ARM = 53
	R_ARM_THM_PC12           R_ARM = 54
	R_ARM_ABS32_NOI          R_ARM = 55
	R_ARM_REL32_NOI          R_ARM = 56
	R_ARM_ALU_PC_G0_NC       R_ARM = 57
	R_ARM_ALU_PC_G0          R_ARM = 58
	R_ARM_ALU_PC_G1_NC       R_ARM = 59
	R_ARM_ALU_PC_G1          R_ARM = 60
	R_ARM_ALU_PC_G2          R_ARM = 61
	R_ARM_LDR_PC_G1          R_ARM = 62
	R_ARM_LDR_PC_G2          R_ARM = 63
	R_ARM_LDRS_PC_G0         R_ARM = 64
	R_ARM_LDRS_PC_G1         R_ARM = 65
	R_ARM_LDRS_PC_G2         R_ARM = 66
	R_ARM_LDC_PC_G0          R_ARM = 67
	R_ARM_LDC_PC_G1          R_ARM = 68
	R_ARM_LDC_PC_G2          R_ARM = 69
	R_ARM_ALU_SB_G0_NC       R_ARM = 70
	R_ARM_ALU_SB_G0          R_ARM = 71
	R_ARM_ALU_SB_G1_NC       R_ARM = 72
	R_ARM_ALU_SB_G1          R_ARM = 73
	R_ARM_ALU_SB_G2          R_ARM = 74
	R_ARM_LDR_SB_G0          R_ARM = 75
	R_ARM_LDR_SB_G1          R_ARM = 76
	R_ARM_LDR_SB_G2          R_ARM = 77
	R_ARM_LDRS_SB_G0         R_ARM = 78
	R_ARM_LDRS_SB_G1         R_ARM = 79
	R_ARM_LDRS_SB_G2         R_ARM = 80
	R_ARM_LDC_SB_G0          R_ARM = 81
	R_ARM_LDC_SB_G1          R_ARM = 82
	R_ARM_LDC_SB_G2          R_ARM = 83
	R_ARM_MOVW_BREL_NC       R_ARM = 84
	R_ARM_MOVT_BREL          R_ARM = 85
	R_ARM_MOVW_BREL          R_ARM = 86
	R_ARM_THM_MOVW_BREL_NC   R_ARM = 87
	R_ARM_THM_MOVT_BREL      R_ARM = 88
	R_ARM_THM_MOVW_BREL      R_ARM = 89
	R_ARM_TLS_GOTDESC        R_ARM = 90
	R_ARM_TLS_CALL           R_ARM = 91
	R_ARM_TLS_DESCSEQ        R_ARM = 92
	R_ARM_THM_TLS_CALL       R_ARM = 93
	R_ARM_PLT32_ABS          R_ARM = 94
	R_ARM_GOT_ABS            R_ARM = 95
	R_ARM_GOT_PREL           R_ARM = 96
	R_ARM_GOT_BREL12         R_ARM = 97
	R_ARM_GOTOFF12           R_ARM = 98
	R_ARM_GOTRELAX           R_ARM = 99
	R_ARM_GNU_VTENTRY        R_ARM = 100
	R_ARM_GNU_VTINHERIT      R_ARM = 101
	R_ARM_THM_JUMP11         R_ARM = 102
	R_ARM_THM_JUMP8          R_ARM = 103
	R_ARM_TLS_GD32           R_ARM = 104
	R_ARM_TLS_LDM32          R_ARM = 105
	R_ARM_TLS_LDO32          R_ARM = 106
	R_ARM_TLS_IE32           R_ARM = 107
	R_ARM_TLS_LE32           R_ARM = 108
	R_ARM_TLS_LDO12          R_ARM = 109
	R_ARM_TLS_LE12           R_ARM = 110
	R_ARM_TLS_IE12GP         R_ARM = 111
	R_ARM_PRIVATE_0          R_ARM = 112
	R_ARM_PRIVATE_1          R_ARM = 113
	R_ARM_PRIVATE_2          R_ARM = 114
	R_ARM_PRIVATE_3          R_ARM = 115
	R_ARM_PRIVATE_4          R_ARM = 116
	R_ARM_PRIVATE_5          R_ARM = 117
	R_ARM_PRIVATE_6          R_ARM = 118
	R_ARM_PRIVATE_7          R_ARM = 119
	R_ARM_PRIVATE_8          R_ARM = 120
	R_ARM_PRIVATE_9          R_ARM = 121
	R_ARM_PRIVATE_10         R_ARM = 122
	R_ARM_PRIVATE_11         R_ARM = 123
	R_ARM_PRIVATE_12         R_ARM = 124
	R_ARM_PRIVATE_13         R_ARM = 125
	R_ARM_PRIVATE_14         R_ARM = 126
	R_ARM_PRIVATE_15         R_ARM = 127
	R_ARM_ME_TOO             R_ARM = 128
	R_ARM_THM_TLS_DESCSEQ16  R_ARM = 129
	R_ARM_THM_TLS_DESCSEQ32  R_ARM = 130
	R_ARM_THM_GOT_BREL12     R_ARM = 131
	R_ARM_THM_ALU_ABS_G0_NC  R_ARM = 132
	R_ARM_THM_ALU_ABS_G1_NC  R_ARM = 133
	R_ARM_THM_ALU_ABS_G2_NC  R_ARM = 134
	R_ARM_THM_ALU_ABS_G3     R_ARM = 135
	R_ARM_IRELATIVE          R_ARM = 160
	R_ARM_RXPC25             R_ARM = 249
	R_ARM_RSBREL32           R_ARM = 250
	R_ARM_THM_RPC22          R_ARM = 251
	R_ARM_RREL32             R_ARM = 252
	R_ARM_RABS32             R_ARM = 253
	R_ARM_RPC24              R_ARM = 254
	R_ARM_RBASE              R_ARM = 255
)

var rarmStrings = []intName{
	{0, "R_ARM_NONE"},
	{1, "R_ARM_PC24"},
	{2, "R_ARM_ABS32"},
	{3, "R_ARM_REL32"},
	{4, "R_ARM_PC13"},
	{5, "R_ARM_ABS16"},
	{6, "R_ARM_ABS12"},
	{7, "R_ARM_THM_ABS5"},
	{8, "R_ARM_ABS8"},
	{9, "R_ARM_SBREL32"},
	{10, "R_ARM_THM_PC22"},
	{11, "R_ARM_THM_PC8"},
	{12, "R_ARM_AMP_VCALL9"},
	{13, "R_ARM_SWI24"},
	{14, "R_ARM_THM_SWI8"},
	{15, "R_ARM_XPC25"},
	{16, "R_ARM_THM_XPC22"},
	{17, "R_ARM_TLS_DTPMOD32"},
	{18, "R_ARM_TLS_DTPOFF32"},
	{19, "R_ARM_TLS_TPOFF32"},
	{20, "R_ARM_COPY"},
	{21, "R_ARM_GLOB_DAT"},
	{22, "R_ARM_JUMP_SLOT"},
	{23, "R_ARM_RELATIVE"},
	{24, "R_ARM_GOTOFF"},
	{25, "R_ARM_GOTPC"},
	{26, "R_ARM_GOT32"},
	{27, "R_ARM_PLT32"},
	{28, "R_ARM_CALL"},
	{29, "R_ARM_JUMP24"},
	{30, "R_ARM_THM_JUMP24"},
	{31, "R_ARM_BASE_ABS"},
	{32, "R_ARM_ALU_PCREL_7_0"},
	{33, "R_ARM_ALU_PCREL_15_8"},
	{34, "R_ARM_ALU_PCREL_23_15"},
	{35, "R_ARM_LDR_SBREL_11_10_NC"},
	{36, "R_ARM_ALU_SBREL_19_12_NC"},
	{37, "R_ARM_ALU_SBREL_27_20_CK"},
	{38, "R_ARM_TARGET1"},
	{39, "R_ARM_SBREL31"},
	{40, "R_ARM_V4BX"},
	{41, "R_ARM_TARGET2"},
	{42, "R_ARM_PREL31"},
	{43, "R_ARM_MOVW_ABS_NC"},
	{44, "R_ARM_MOVT_ABS"},
	{45, "R_ARM_MOVW_PREL_NC"},
	{46, "R_ARM_MOVT_PREL"},
	{47, "R_ARM_THM_MOVW_ABS_NC"},
	{48, "R_ARM_THM_MOVT_ABS"},
	{49, "R_ARM_THM_MOVW_PREL_NC"},
	{50, "R_ARM_THM_MOVT_PREL"},
	{51, "R_ARM_THM_JUMP19"},
	{52, "R_ARM_THM_JUMP6"},
	{53, "R_ARM_THM_ALU_PREL_11_0"},
	{54, "R_ARM_THM_PC12"},
	{55, "R_ARM_ABS32_NOI"},
	{56, "R_ARM_REL32_NOI"},
	{57, "R_ARM_ALU_PC_G0_NC"},
	{58, "R_ARM_ALU_PC_G0"},
	{59, "R_ARM_ALU_PC_G1_NC"},
	{60, "R_ARM_ALU_PC_G1"},
	{61, "R_ARM_ALU_PC_G2"},
	{62, "R_ARM_LDR_PC_G1"},
	{63, "R_ARM_LDR_PC_G2"},
	{64, "R_ARM_LDRS_PC_G0"},
	{65, "R_ARM_LDRS_PC_G1"},
	{66, "R_ARM_LDRS_PC_G2"},
	{67, "R_ARM_LDC_PC_G0"},
	{68, "R_ARM_LDC_PC_G1"},
	{69, "R_ARM_LDC_PC_G2"},
	{70, "R_ARM_ALU_SB_G0_NC"},
	{71, "R_ARM_ALU_SB_G0"},
	{72, "R_ARM_ALU_SB_G1_NC"},
	{73, "R_ARM_ALU_SB_G1"},
	{74, "R_ARM_ALU_SB_G2"},
	{75, "R_ARM_LDR_SB_G0"},
	{76, "R_ARM_LDR_SB_G1"},
	{77, "R_ARM_LDR_SB_G2"},
	{78, "R_ARM_LDRS_SB_G0"},
	{79, "R_ARM_LDRS_SB_G1"},
	{80, "R_ARM_LDRS_SB_G2"},
	{81, "R_ARM_LDC_SB_G0"},
	{82, "R_ARM_LDC_SB_G1"},
	{83, "R_ARM_LDC_SB_G2"},
	{84, "R_ARM_MOVW_BREL_NC"},
	{85, "R_ARM_MOVT_BREL"},
	{86, "R_ARM_MOVW_BREL"},
	{87, "R_ARM_THM_MOVW_BREL_NC"},
	{88, "R_ARM_THM_MOVT_BREL"},
	{89, "R_ARM_THM_MOVW_BREL"},
	{90, "R_ARM_TLS_GOTDESC"},
	{91, "R_ARM_TLS_CALL"},
	{92, "R_ARM_TLS_DESCSEQ"},
	{93, "R_ARM_THM_TLS_CALL"},
	{94, "R_ARM_PLT32_ABS"},
	{95, "R_ARM_GOT_ABS"},
	{96, "R_ARM_GOT_PREL"},
	{97, "R_ARM_GOT_BREL12"},
	{98, "R_ARM_GOTOFF12"},
	{99, "R_ARM_GOTRELAX"},
	{100, "R_ARM_GNU_VTENTRY"},
	{101, "R_ARM_GNU_VTINHERIT"},
	{102, "R_ARM_THM_JUMP11"},
	{103, "R_ARM_THM_JUMP8"},
	{104, "R_ARM_TLS_GD32"},
	{105, "R_ARM_TLS_LDM32"},
	{106, "R_ARM_TLS_LDO32"},
	{107, "R_ARM_TLS_IE32"},
	{108, "R_ARM_TLS_LE32"},
	{109, "R_ARM_TLS_LDO12"},
	{110, "R_ARM_TLS_LE12"},
	{111, "R_ARM_TLS_IE12GP"},
	{112, "R_ARM_PRIVATE_0"},
	{113, "R_ARM_PRIVATE_1"},
	{114, "R_ARM_PRIVATE_2"},
	{115, "R_ARM_PRIVATE_3"},
	{116, "R_ARM_PRIVATE_4"},
	{117, "R_ARM_PRIVATE_5"},
	{118, "R_ARM_PRIVATE_6"},
	{119, "R_ARM_PRIVATE_7"},
	{120, "R_ARM_PRIVATE_8"},
	{121, "R_ARM_PRIVATE_9"},
	{122, "R_ARM_PRIVATE_10"},
	{123, "R_ARM_PRIVATE_11"},
	{124, "R_ARM_PRIVATE_12"},
	{125, "R_ARM_PRIVATE_13"},
	{126, "R_ARM_PRIVATE_14"},
	{127, "R_ARM_PRIVATE_15"},
	{128, "R_ARM_ME_TOO"},
	{129, "R_ARM_THM_TLS_DESCSEQ16"},
	{130, "R_ARM_THM_TLS_DESCSEQ32"},
	{131, "R_ARM_THM_GOT_BREL12"},
	{132, "R_ARM_THM_ALU_ABS_G0_NC"},
	{133, "R_ARM_THM_ALU_ABS_G1_NC"},
	{134, "R_ARM_THM_ALU_ABS_G2_NC"},
	{135, "R_ARM_THM_ALU_ABS_G3"},
	{160, "R_ARM_IRELATIVE"},
	{249, "R_ARM_RXPC25"},
	{250, "R_ARM_RSBREL32"},
	{251, "R_ARM_THM_RPC22"},
	{252, "R_ARM_RREL32"},
	{253, "R_ARM_RABS32"},
	{254, "R_ARM_RPC24"},
	{255, "R_ARM_RBASE"},
}

func (i R_ARM) String() string   { return stringName(uint32(i), rarmStrings, false) }
func (i R_ARM) GoString() string { return stringName(uint32(i), rarmStrings, true) }

// Relocation types for 386.
type R_386 int

const (
	R_386_NONE          R_386 = 0  /* No relocation. */
	R_386_32            R_386 = 1  /* Add symbol value. */
	R_386_PC32          R_386 = 2  /* Add PC-relative symbol value. */
	R_386_GOT32         R_386 = 3  /* Add PC-relative GOT offset. */
	R_386_PLT32         R_386 = 4  /* Add PC-relative PLT offset. */
	R_386_COPY          R_386 = 5  /* Copy data from shared object. */
	R_386_GLOB_DAT      R_386 = 6  /* Set GOT entry to data address. */
	R_386_JMP_SLOT      R_386 = 7  /* Set GOT entry to code address. */
	R_386_RELATIVE      R_386 = 8  /* Add load address of shared object. */
	R_386_GOTOFF        R_386 = 9  /* Add GOT-relative symbol address. */
	R_386_GOTPC         R_386 = 10 /* Add PC-relative GOT table address. */
	R_386_32PLT         R_386 = 11
	R_386_TLS_TPOFF     R_386 = 14 /* Negative offset in static TLS block */
	R_386_TLS_IE        R_386 = 15 /* Absolute address of GOT for -ve static TLS */
	R_386_TLS_GOTIE     R_386 = 16 /* GOT entry for negative static TLS block */
	R_386_TLS_LE        R_386 = 17 /* Negative offset relative to static TLS */
	R_386_TLS_GD        R_386 = 18 /* 32 bit offset to GOT (index,off) pair */
	R_386_TLS_LDM       R_386 = 19 /* 32 bit offset to GOT (index,zero) pair */
	R_386_16            R_386 = 20
	R_386_PC16          R_386 = 21
	R_386_8             R_386 = 22
	R_386_PC8           R_386 = 23
	R_386_TLS_GD_32     R_386 = 24 /* 32 bit offset to GOT (index,off) pair */
	R_386_TLS_GD_PUSH   R_386 = 25 /* pushl instruction for Sun ABI GD sequence */
	R_386_TLS_GD_CALL   R_386 = 26 /* call instruction for Sun ABI GD sequence */
	R_386_TLS_GD_POP    R_386 = 27 /* popl instruction for Sun ABI GD sequence */
	R_386_TLS_LDM_32    R_386 = 28 /* 32 bit offset to GOT (index,zero) pair */
	R_386_TLS_LDM_PUSH  R_386 = 29 /* pushl instruction for Sun ABI LD sequence */
	R_386_TLS_LDM_CALL  R_386 = 30 /* call instruction for Sun ABI LD sequence */
	R_386_TLS_LDM_POP   R_386 = 31 /* popl instruction for Sun ABI LD sequence */
	R_386_TLS_LDO_32    R_386 = 32 /* 32 bit offset from start of TLS block */
	R_386_TLS_IE_32     R_386 = 33 /* 32 bit offset to GOT static TLS offset entry */
	R_386_TLS_LE_32     R_386 = 34 /* 32 bit offset within static TLS block */
	R_386_TLS_DTPMOD32  R_386 = 35 /* GOT entry containing TLS index */
	R_386_TLS_DTPOFF32  R_386 = 36 /* GOT entry containing TLS offset */
	R_386_TLS_TPOFF32   R_386 = 37 /* GOT entry of -ve static TLS offset */
	R_386_SIZE32        R_386 = 38
	R_386_TLS_GOTDESC   R_386 = 39
	R_386_TLS_DESC_CALL R_386 = 40
	R_386_TLS_DESC      R_386 = 41
	R_386_IRELATIVE     R_386 = 42
	R_386_GOT32X        R_386 = 43
)

var r386Strings = []intName{
	{0, "R_386_NONE"},
	{1, "R_386_32"},
	{2, "R_386_PC32"},
	{3, "R_386_GOT32"},
	{4, "R_386_PLT32"},
	{5, "R_386_COPY"},
	{6, "R_386_GLOB_DAT"},
	{7, "R_386_JMP_SLOT"},
	{8, "R_386_RELATIVE"},
	{9, "R_386_GOTOFF"},
	{10, "R_386_GOTPC"},
	{11, "R_386_32PLT"},
	{14, "R_386_TLS_TPOFF"},
	{15, "R_386_TLS_IE"},
	{16, "R_386_TLS_GOTIE"},
	{17, "R_386_TLS_LE"},
	{18, "R_386_TLS_GD"},
	{19, "R_386_TLS_LDM"},
	{20, "R_386_16"},
	{21, "R_386_PC16"},
	{22, "R_386_8"},
	{23, "R_386_PC8"},
	{24, "R_386_TLS_GD_32"},
	{25, "R_386_TLS_GD_PUSH"},
	{26, "R_386_TLS_GD_CALL"},
	{27, "R_386_TLS_GD_POP"},
	{28, "R_386_TLS_LDM_32"},
	{29, "R_386_TLS_LDM_PUSH"},
	{30, "R_386_TLS_LDM_CALL"},
	{31, "R_386_TLS_LDM_POP"},
	{32, "R_386_TLS_LDO_32"},
	{33, "R_386_TLS_IE_32"},
	{34, "R_386_TLS_LE_32"},
	{35, "R_386_TLS_DTPMOD32"},
	{36, "R_386_TLS_DTPOFF32"},
	{37, "R_386_TLS_TPOFF32"},
	{38, "R_386_SIZE32"},
	{39, "R_386_TLS_GOTDESC"},
	{40, "R_386_TLS_DESC_CALL"},
	{41, "R_386_TLS_DESC"},
	{42, "R_386_IRELATIVE"},
	{43, "R_386_GOT32X"},
}

func (i R_386) String() string   { return stringName(uint32(i), r386Strings, false) }
func (i R_386) GoString() string { return stringName(uint32(i), r386Strings, true) }

// Relocation types for MIPS.
type R_MIPS int

const (
	R_MIPS_NONE          R_MIPS = 0
	R_MIPS_16            R_MIPS = 1
	R_MIPS_32            R_MIPS = 2
	R_MIPS_REL32         R_MIPS = 3
	R_MIPS_26            R_MIPS = 4
	R_MIPS_HI16          R_MIPS = 5  /* high 16 bits of symbol value */
	R_MIPS_LO16          R_MIPS = 6  /* low 16 bits of symbol value */
	R_MIPS_GPREL16       R_MIPS = 7  /* GP-relative reference  */
	R_MIPS_LITERAL       R_MIPS = 8  /* Reference to literal section  */
	R_MIPS_GOT16         R_MIPS = 9  /* Reference to global offset table */
	R_MIPS_PC16          R_MIPS = 10 /* 16 bit PC relative reference */
	R_MIPS_CALL16        R_MIPS = 11 /* 16 bit call through glbl offset tbl */
	R_MIPS_GPREL32       R_MIPS = 12
	R_MIPS_SHIFT5        R_MIPS = 16
	R_MIPS_SHIFT6        R_MIPS = 17
	R_MIPS_64            R_MIPS = 18
	R_MIPS_GOT_DISP      R_MIPS = 19
	R_MIPS_GOT_PAGE      R_MIPS = 20
	R_MIPS_GOT_OFST      R_MIPS = 21
	R_MIPS_GOT_HI16      R_MIPS = 22
	R_MIPS_GOT_LO16      R_MIPS = 23
	R_MIPS_SUB           R_MIPS = 24
	R_MIPS_INSERT_A      R_MIPS = 25
	R_MIPS_INSERT_B      R_MIPS = 26
	R_MIPS_DELETE        R_MIPS = 27
	R_MIPS_HIGHER        R_MIPS = 28
	R_MIPS_HIGHEST       R_MIPS = 29
	R_MIPS_CALL_HI16     R_MIPS = 30
	R_MIPS_CALL_LO16     R_MIPS = 31
	R_MIPS_SCN_DISP      R_MIPS = 32
	R_MIPS_REL16         R_MIPS = 33
	R_MIPS_ADD_IMMEDIATE R_MIPS = 34
	R_MIPS_PJUMP         R_MIPS = 35
	R_MIPS_RELGOT        R_MIPS = 36
	R_MIPS_JALR          R_MIPS = 37

	R_MIPS_TLS_DTPMOD32    R_MIPS = 38 /* Module number 32 bit */
	R_MIPS_TLS_DTPREL32    R_MIPS = 39 /* Module-relative offset 32 bit */
	R_MIPS_TLS_DTPMOD64    R_MIPS = 40 /* Module number 64 bit */
	R_MIPS_TLS_DTPREL64    R_MIPS = 41 /* Module-relative offset 64 bit */
	R_MIPS_TLS_GD          R_MIPS = 42 /* 16 bit GOT offset for GD */
	R_MIPS_TLS_LDM         R_MIPS = 43 /* 16 bit GOT offset for LDM */
	R_MIPS_TLS_DTPREL_HI16 R_MIPS = 44 /* Module-relative offset, high 16 bits */
	R_MIPS_TLS_DTPREL_LO16 R_MIPS = 45 /* Module-relative offset, low 16 bits */
	R_MIPS_TLS_GOTTPREL    R_MIPS = 46 /* 16 bit GOT offset for IE */
	R_MIPS_TLS_TPREL32     R_MIPS = 47 /* TP-relative offset, 32 bit */
	R_MIPS_TLS_TPREL64     R_MIPS = 48 /* TP-relative offset, 64 bit */
	R_MIPS_TLS_TPREL_HI16  R_MIPS = 49 /* TP-relative offset, high 16 bits */
	R_MIPS_TLS_TPREL_LO16  R_MIPS = 50 /* TP-relative offset, low 16 bits */

	R_MIPS_PC32 R_MIPS = 248 /* 32 bit PC relative reference */
)

var rmipsStrings = []intName{
	{0, "R_MIPS_NONE"},
	{1, "R_MIPS_16"},
	{2, "R_MIPS_32"},
	{3, "R_MIPS_REL32"},
	{4, "R_MIPS_26"},
	{5, "R_MIPS_HI16"},
	{6, "R_MIPS_LO16"},
	{7, "R_MIPS_GPREL16"},
	{8, "R_MIPS_LITERAL"},
	{9, "R_MIPS_GOT16"},
	{10, "R_MIPS_PC16"},
	{11, "R_MIPS_CALL16"},
	{12, "R_MIPS_GPREL32"},
	{16, "R_MIPS_SHIFT5"},
	{17, "R_MIPS_SHIFT6"},
	{18, "R_MIPS_64"},
	{19, "R_MIPS_GOT_DISP"},
	{20, "R_MIPS_GOT_PAGE"},
	{21, "R_MIPS_GOT_OFST"},
	{22, "R_MIPS_GOT_HI16"},
	{23, "R_MIPS_GOT_LO16"},
	{24, "R_MIPS_SUB"},
	{25, "R_MIPS_INSERT_A"},
	{26, "R_MIPS_INSERT_B"},
	{27, "R_MIPS_DELETE"},
	{28, "R_MIPS_HIGHER"},
	{29, "R_MIPS_HIGHEST"},
	{30, "R_MIPS_CALL_HI16"},
	{31, "R_MIPS_CALL_LO16"},
	{32, "R_MIPS_SCN_DISP"},
	{33, "R_MIPS_REL16"},
	{34, "R_MIPS_ADD_IMMEDIATE"},
	{35, "R_MIPS_PJUMP"},
	{36, "R_MIPS_RELGOT"},
	{37, "R_MIPS_JALR"},
	{38, "R_MIPS_TLS_DTPMOD32"},
	{39, "R_MIPS_TLS_DTPREL32"},
	{40, "R_MIPS_TLS_DTPMOD64"},
	{41, "R_MIPS_TLS_DTPREL64"},
	{42, "R_MIPS_TLS_GD"},
	{43, "R_MIPS_TLS_LDM"},
	{44, "R_MIPS_TLS_DTPREL_HI16"},
	{45, "R_MIPS_TLS_DTPREL_LO16"},
	{46, "R_MIPS_TLS_GOTTPREL"},
	{47, "R_MIPS_TLS_TPREL32"},
	{48, "R_MIPS_TLS_TPREL64"},
	{49, "R_MIPS_TLS_TPREL_HI16"},
	{50, "R_MIPS_TLS_TPREL_LO16"},
	{248, "R_MIPS_PC32"},
}

func (i R_MIPS) String() string   { return stringName(uint32(i), rmipsStrings, false) }
func (i R_MIPS) GoString() string { return stringName(uint32(i), rmipsStrings, true) }

// Relocation types for LoongArch.
type R_LARCH int

const (
	R_LARCH_NONE                       R_LARCH = 0
	R_LARCH_32                         R_LARCH = 1
	R_LARCH_64                         R_LARCH = 2
	R_LARCH_RELATIVE                   R_LARCH = 3
	R_LARCH_COPY                       R_LARCH = 4
	R_LARCH_JUMP_SLOT                  R_LARCH = 5
	R_LARCH_TLS_DTPMOD32               R_LARCH = 6
	R_LARCH_TLS_DTPMOD64               R_LARCH = 7
	R_LARCH_TLS_DTPREL32               R_LARCH = 8
	R_LARCH_TLS_DTPREL64               R_LARCH = 9
	R_LARCH_TLS_TPREL32                R_LARCH = 10
	R_LARCH_TLS_TPREL64                R_LARCH = 11
	R_LARCH_IRELATIVE                  R_LARCH = 12
	R_LARCH_TLS_DESC32                 R_LARCH = 13
	R_LARCH_TLS_DESC64                 R_LARCH = 14
	R_LARCH_MARK_LA                    R_LARCH = 20
	R_LARCH_MARK_PCREL                 R_LARCH = 21
	R_LARCH_SOP_PUSH_PCREL             R_LARCH = 22
	R_LARCH_SOP_PUSH_ABSOLUTE          R_LARCH = 23
	R_LARCH_SOP_PUSH_DUP               R_LARCH = 24
	R_LARCH_SOP_PUSH_GPREL             R_LARCH = 25
	R_LARCH_SOP_PUSH_TLS_TPREL         R_LARCH = 26
	R_LARCH_SOP_PUSH_TLS_GOT           R_LARCH = 27
	R_LARCH_SOP_PUSH_TLS_GD            R_LARCH = 28
	R_LARCH_SOP_PUSH_PLT_PCREL         R_LARCH = 29
	R_LARCH_SOP_ASSERT                 R_LARCH = 30
	R_LARCH_SOP_NOT                    R_LARCH = 31
	R_LARCH_SOP_SUB                    R_LARCH = 32
	R_LARCH_SOP_SL                     R_LARCH = 33
	R_LARCH_SOP_SR                     R_LARCH = 34
	R_LARCH_SOP_ADD                    R_LARCH = 35
	R_LARCH_SOP_AND                    R_LARCH = 36
	R_LARCH_SOP_IF_ELSE                R_LARCH = 37
	R_LARCH_SOP_POP_32_S_10_5          R_LARCH = 38
	R_LARCH_SOP_POP_32_U_10_12         R_LARCH = 39
	R_LARCH_SOP_POP_32_S_10_12         R_LARCH = 40
	R_LARCH_SOP_POP_32_S_10_16         R_LARCH = 41
	R_LARCH_SOP_POP_32_S_10_16_S2      R_LARCH = 42
	R_LARCH_SOP_POP_32_S_5_20          R_LARCH = 43
	R_LARCH_SOP_POP_32_S_0_5_10_16_S2  R_LARCH = 44
	R_LARCH_SOP_POP_32_S_0_10_10_16_S2 R_LARCH = 45
	R_LARCH_SOP_POP_32_U               R_LARCH = 46
	R_LARCH_ADD8                       R_LARCH = 47
	R_LARCH_ADD16                      R_LARCH = 48
	R_LARCH_ADD24                      R_LARCH = 49
	R_LARCH_ADD32                      R_LARCH = 50
	R_LARCH_ADD64                      R_LARCH = 51
	R_LARCH_SUB8                       R_LARCH = 52
	R_LARCH_SUB16                      R_LARCH = 53
	R_LARCH_SUB24                      R_LARCH = 54
	R_LARCH_SUB32                      R_LARCH = 55
	R_LARCH_SUB64                      R_LARCH = 56
	R_LARCH_GNU_VTINHERIT              R_LARCH = 57
	R_LARCH_GNU_VTENTRY                R_LARCH = 58
	R_LARCH_B16                        R_LARCH = 64
	R_LARCH_B21                        R_LARCH = 65
	R_LARCH_B26                        R_LARCH = 66
	R_LARCH_ABS_HI20                   R_LARCH = 67
	R_LARCH_ABS_LO12                   R_LARCH = 68
	R_LARCH_ABS64_LO20                 R_LARCH = 69
	R_LARCH_ABS64_HI12                 R_LARCH = 70
	R_LARCH_PCALA_HI20                 R_LARCH = 71
	R_LARCH_PCALA_LO12                 R_LARCH = 72
	R_LARCH_PCALA64_LO20               R_LARCH = 73
	R_LARCH_PCALA64_HI12               R_LARCH = 74
	R_LARCH_GOT_PC_HI20                R_LARCH = 75
	R_LARCH_GOT_PC_LO12                R_LARCH = 76
	R_LARCH_GOT64_PC_LO20              R_LARCH = 77
	R_LARCH_GOT64_PC_HI12              R_LARCH = 78
	R_LARCH_GOT_HI20                   R_LARCH = 79
	R_LARCH_GOT_LO12                   R_LARCH = 80
	R_LARCH_GOT64_LO20                 R_LARCH = 81
	R_LARCH_GOT64_HI12                 R_LARCH = 82
	R_LARCH_TLS_LE_HI20                R_LARCH = 83
	R_LARCH_TLS_LE_LO12                R_LARCH = 84
	R_LARCH_TLS_LE64_LO20              R_LARCH = 85
	R_LARCH_TLS_LE64_HI12              R_LARCH = 86
	R_LARCH_TLS_IE_PC_HI20             R_LARCH = 87
	R_LARCH_TLS_IE_PC_LO12             R_LARCH = 88
	R_LARCH_TLS_IE64_PC_LO20           R_LARCH = 89
	R_LARCH_TLS_IE64_PC_HI12           R_LARCH = 90
	R_LARCH_TLS_IE_HI20                R_LARCH = 91
	R_LARCH_TLS_IE_LO12                R_LARCH = 92
	R_LARCH_TLS_IE64_LO20              R_LARCH = 93
	R_LARCH_TLS_IE64_HI12              R_LARCH = 94
	R_LARCH_TLS_LD_PC_HI20             R_LARCH = 95
	R_LARCH_TLS_LD_HI20                R_LARCH = 96
	R_LARCH_TLS_GD_PC_HI20             R_LARCH = 97
	R_LARCH_TLS_GD_HI20                R_LARCH = 98
	R_LARCH_32_PCREL                   R_LARCH = 99
	R_LARCH_RELAX                      R_LARCH = 100
	R_LARCH_DELETE                     R_LARCH = 101
	R_LARCH_ALIGN                      R_LARCH = 102
	R_LARCH_PCREL20_S2                 R_LARCH = 103
	R_LARCH_CFA                        R_LARCH = 104
	R_LARCH_ADD6                       R_LARCH = 105
	R_LARCH_SUB6                       R_LARCH = 106
	R_LARCH_ADD_ULEB128                R_LARCH = 107
	R_LARCH_SUB_ULEB128                R_LARCH = 108
	R_LARCH_64_PCREL                   R_LARCH = 109
	R_LARCH_CALL36                     R_LARCH = 110
	R_LARCH_TLS_DESC_PC_HI20           R_LARCH = 111
	R_LARCH_TLS_DESC_PC_LO12           R_LARCH = 112
	R_LARCH_TLS_DESC64_PC_LO20         R_LARCH = 113
	R_LARCH_TLS_DESC64_PC_HI12         R_LARCH = 114
	R_LARCH_TLS_DESC_HI20              R_LARCH = 115
	R_LARCH_TLS_DESC_LO12              R_LARCH = 116
	R_LARCH_TLS_DESC64_LO20            R_LARCH = 117
	R_LARCH_TLS_DESC64_HI12            R_LARCH = 118
	R_LARCH_TLS_DESC_LD                R_LARCH = 119
	R_LARCH_TLS_DESC_CALL              R_LARCH = 120
	R_LARCH_TLS_LE_HI20_R              R_LARCH = 121
	R_LARCH_TLS_LE_ADD_R               R_LARCH = 122
	R_LARCH_TLS_LE_LO12_R              R_LARCH = 123
	R_LARCH_TLS_LD_PCREL20_S2          R_LARCH = 124
	R_LARCH_TLS_GD_PCREL20_S2          R_LARCH = 125
	R_LARCH_TLS_DESC_PCREL20_S2        R_LARCH = 126
)

var rlarchStrings = []intName{
	{0, "R_LARCH_NONE"},
	{1, "R_LARCH_32"},
	{2, "R_LARCH_64"},
	{3, "R_LARCH_RELATIVE"},
	{4, "R_LARCH_COPY"},
	{5, "R_LARCH_JUMP_SLOT"},
	{6, "R_LARCH_TLS_DTPMOD32"},
	{7, "R_LARCH_TLS_DTPMOD64"},
	{8, "R_LARCH_TLS_DTPREL32"},
	{9, "R_LARCH_TLS_DTPREL64"},
	{10, "R_LARCH_TLS_TPREL32"},
	{11, "R_LARCH_TLS_TPREL64"},
	{12, "R_LARCH_IRELATIVE"},
	{13, "R_LARCH_TLS_DESC32"},
	{14, "R_LARCH_TLS_DESC64"},
	{20, "R_LARCH_MARK_LA"},
	{21, "R_LARCH_MARK_PCREL"},
	{22, "R_LARCH_SOP_PUSH_PCREL"},
	{23, "R_LARCH_SOP_PUSH_ABSOLUTE"},
	{24, "R_LARCH_SOP_PUSH_DUP"},
	{25, "R_LARCH_SOP_PUSH_GPREL"},
	{26, "R_LARCH_SOP_PUSH_TLS_TPREL"},
	{27, "R_LARCH_SOP_PUSH_TLS_GOT"},
	{28, "R_LARCH_SOP_PUSH_TLS_GD"},
	{29, "R_LARCH_SOP_PUSH_PLT_PCREL"},
	{30, "R_LARCH_SOP_ASSERT"},
	{31, "R_LARCH_SOP_NOT"},
	{32, "R_LARCH_SOP_SUB"},
	{33, "R_LARCH_SOP_SL"},
	{34, "R_LARCH_SOP_SR"},
	{35, "R_LARCH_SOP_ADD"},
	{36, "R_LARCH_SOP_AND"},
	{37, "R_LARCH_SOP_IF_ELSE"},
	{38, "R_LARCH_SOP_POP_32_S_10_5"},
	{39, "R_LARCH_SOP_POP_32_U_10_12"},
	{40, "R_LARCH_SOP_POP_32_S_10_12"},
	{41, "R_LARCH_SOP_POP_32_S_10_16"},
	{42, "R_LARCH_SOP_POP_32_S_10_16_S2"},
	{43, "R_LARCH_SOP_POP_32_S_5_20"},
	{44, "R_LARCH_SOP_POP_32_S_0_5_10_16_S2"},
	{45, "R_LARCH_SOP_POP_32_S_0_10_10_16_S2"},
	{46, "R_LARCH_SOP_POP_32_U"},
	{47, "R_LARCH_ADD8"},
	{48, "R_LARCH_ADD16"},
	{49, "R_LARCH_ADD24"},
	{50, "R_LARCH_ADD32"},
	{51, "R_LARCH_ADD64"},
	{52, "R_LARCH_SUB8"},
	{53, "R_LARCH_SUB16"},
	{54, "R_LARCH_SUB24"},
	{55, "R_LARCH_SUB32"},
	{56, "R_LARCH_SUB64"},
	{57, "R_LARCH_GNU_VTINHERIT"},
	{58, "R_LARCH_GNU_VTENTRY"},
	{64, "R_LARCH_B16"},
	{65, "R_LARCH_B21"},
	{66, "R_LARCH_B26"},
	{67, "R_LARCH_ABS_HI20"},
	{68, "R_LARCH_ABS_LO12"},
	{69, "R_LARCH_ABS64_LO20"},
	{70, "R_LARCH_ABS64_HI12"},
	{71, "R_LARCH_PCALA_HI20"},
	{72, "R_LARCH_PCALA_LO12"},
	{73, "R_LARCH_PCALA64_LO20"},
	{74, "R_LARCH_PCALA64_HI12"},
	{75, "R_LARCH_GOT_PC_HI20"},
	{76, "R_LARCH_GOT_PC_LO12"},
	{77, "R_LARCH_GOT64_PC_LO20"},
	{78, "R_LARCH_GOT64_PC_HI12"},
	{79, "R_LARCH_GOT_HI20"},
	{80, "R_LARCH_GOT_LO12"},
	{81, "R_LARCH_GOT64_LO20"},
	{82, "R_LARCH_GOT64_HI12"},
	{83, "R_LARCH_TLS_LE_HI20"},
	{84, "R_LARCH_TLS_LE_LO12"},
	{85, "R_LARCH_TLS_LE64_LO20"},
	{86, "R_LARCH_TLS_LE64_HI12"},
	{87, "R_LARCH_TLS_IE_PC_HI20"},
	{88, "R_LARCH_TLS_IE_PC_LO12"},
	{89, "R_LARCH_TLS_IE64_PC_LO20"},
	{90, "R_LARCH_TLS_IE64_PC_HI12"},
	{91, "R_LARCH_TLS_IE_HI20"},
	{92, "R_LARCH_TLS_IE_LO12"},
	{93, "R_LARCH_TLS_IE64_LO20"},
	{94, "R_LARCH_TLS_IE64_HI12"},
	{95, "R_LARCH_TLS_LD_PC_HI20"},
	{96, "R_LARCH_TLS_LD_HI20"},
	{97, "R_LARCH_TLS_GD_PC_HI20"},
	{98, "R_LARCH_TLS_GD_HI20"},
	{99, "R_LARCH_32_PCREL"},
	{100, "R_LARCH_RELAX"},
	{101, "R_LARCH_DELETE"},
	{102, "R_LARCH_ALIGN"},
	{103, "R_LARCH_PCREL20_S2"},
	{104, "R_LARCH_CFA"},
	{105, "R_LARCH_ADD6"},
	{106, "R_LARCH_SUB6"},
	{107, "R_LARCH_ADD_ULEB128"},
	{108, "R_LARCH_SUB_ULEB128"},
	{109, "R_LARCH_64_PCREL"},
	{110, "R_LARCH_CALL36"},
	{111, "R_LARCH_TLS_DESC_PC_HI20"},
	{112, "R_LARCH_TLS_DESC_PC_LO12"},
	{113, "R_LARCH_TLS_DESC64_PC_LO20"},
	{114, "R_LARCH_TLS_DESC64_PC_HI12"},
	{115, "R_LARCH_TLS_DESC_HI20"},
	{116, "R_LARCH_TLS_DESC_LO12"},
	{117, "R_LARCH_TLS_DESC64_LO20"},
	{118, "R_LARCH_TLS_DESC64_HI12"},
	{119, "R_LARCH_TLS_DESC_LD"},
	{120, "R_LARCH_TLS_DESC_CALL"},
	{121, "R_LARCH_TLS_LE_HI20_R"},
	{122, "R_LARCH_TLS_LE_ADD_R"},
	{123, "R_LARCH_TLS_LE_LO12_R"},
	{124, "R_LARCH_TLS_LD_PCREL20_S2"},
	{125, "R_LARCH_TLS_GD_PCREL20_S2"},
	{126, "R_LARCH_TLS_DESC_PCREL20_S2"},
}

func (i R_LARCH) String() string   { return stringName(uint32(i), rlarchStrings, false) }
func (i R_LARCH) GoString() string { return stringName(uint32(i), rlarchStrings, true) }

// Relocation types for PowerPC.
//
// Values that are shared by both R_PPC and R_PPC64 are prefixed with
// R_POWERPC_ in the ELF standard. For the R_PPC type, the relevant
// shared relocations have been renamed with the prefix R_PPC_.
// The original name follows the value in a comment.
type R_PPC int

const (
	R_PPC_NONE            R_PPC = 0  // R_POWERPC_NONE
	R_PPC_ADDR32          R_PPC = 1  // R_POWERPC_ADDR32
	R_PPC_ADDR24          R_PPC = 2  // R_POWERPC_ADDR24
	R_PPC_ADDR16          R_PPC = 3  // R_POWERPC_ADDR16
	R_PPC_ADDR16_LO       R_PPC = 4  // R_POWERPC_ADDR16_LO
	R_PPC_ADDR16_HI       R_PPC = 5  // R_POWERPC_ADDR16_HI
	R_PPC_ADDR16_HA       R_PPC = 6  // R_POWERPC_ADDR16_HA
	R_PPC_ADDR14          R_PPC = 7  // R_POWERPC_ADDR14
	R_PPC_ADDR14_BRTAKEN  R_PPC = 8  // R_POWERPC_ADDR14_BRTAKEN
	R_PPC_ADDR14_BRNTAKEN R_PPC = 9  // R_POWERPC_ADDR14_BRNTAKEN
	R_PPC_REL24           R_PPC = 10 // R_POWERPC_REL24
	R_PPC_REL14           R_PPC = 11 // R_POWERPC_REL14
	R_PPC_REL14_BRTAKEN   R_PPC = 12 // R_POWERPC_REL14_BRTAKEN
	R_PPC_REL14_BRNTAKEN  R_PPC = 13 // R_POWERPC_REL14_BRNTAKEN
	R_PPC_GOT16           R_PPC = 14 // R_POWERPC_GOT16
	R_PPC_GOT16_LO        R_PPC = 15 // R_POWERPC_GOT16_LO
	R_PPC_GOT16_HI        R_PPC = 16 // R_POWERPC_GOT16_HI
	R_PPC_GOT16_HA        R_PPC = 17 // R_POWERPC_GOT16_HA
	R_PPC_PLTREL24        R_PPC = 18
	R_PPC_COPY            R_PPC = 19 // R_POWERPC_COPY
	R_PPC_GLOB_DAT        R_PPC = 20 // R_POWERPC_GLOB_DAT
	R_PPC_JMP_SLOT        R_PPC = 21 // R_POWERPC_JMP_SLOT
	R_PPC_RELATIVE        R_PPC = 22 // R_POWERPC_RELATIVE
	R_PPC_LOCAL24PC       R_PPC = 23
	R_PPC_UADDR32         R_PPC = 24 // R_POWERPC_UADDR32
	R_PPC_UADDR16         R_PPC = 25 // R_POWERPC_UADDR16
	R_PPC_REL32           R_PPC = 26 // R_POWERPC_REL32
	R_PPC_PLT32           R_PPC = 27 // R_POWERPC_PLT32
	R_PPC_PLTREL32        R_PPC = 28 // R_POWERPC_PLTREL32
	R_PPC_PLT16_LO        R_PPC = 29 // R_POWERPC_PLT16_LO
	R_PPC_PLT16_HI        R_PPC = 30 // R_POWERPC_PLT16_HI
	R_PPC_PLT16_HA        R_PPC = 31 // R_POWERPC_PLT16_HA
	R_PPC_SDAREL16        R_PPC = 32
	R_PPC_SECTOFF         R_PPC = 33 // R_POWERPC_SECTOFF
	R_PPC_SECTOFF_LO      R_PPC = 34 // R_POWERPC_SECTOFF_LO
	R_PPC_SECTOFF_HI      R_PPC = 35 // R_POWERPC_SECTOFF_HI
	R_PPC_SECTOFF_HA      R_PPC = 36 // R_POWERPC_SECTOFF_HA
	R_PPC_TLS             R_PPC = 67 // R_POWERPC_TLS
	R_PPC_DTPMOD32        R_PPC = 68 // R_POWERPC_DTPMOD32
	R_PPC_TPREL16         R_PPC = 69 // R_POWERPC_TPREL16
	R_PPC_TPREL16_LO      R_PPC = 70 // R_POWERPC_TPREL16_LO
	R_PPC_TPREL16_HI      R_PPC = 71 // R_POWERPC_TPREL16_HI
	R_PPC_TPREL16_HA      R_PPC = 72 // R_POWERPC_TPREL16_HA
	R_PPC_TPREL32         R_PPC = 73 // R_POWERPC_TPREL32
	R_PPC_DTPREL16        R_PPC = 74 // R_POWERPC_DTPREL16
	R_PPC_DTPREL16_LO     R_PPC = 75 // R_POWERPC_DTPREL16_LO
	R_PPC_DTPREL16_HI     R_PPC = 76 // R_POWERPC_DTPREL16_HI
	R_PPC_DTPREL16_HA     R_PPC = 77 // R_POWERPC_DTPREL16_HA
	R_PPC_DTPREL32        R_PPC = 78 // R_POWERPC_DTPREL32
	R_PPC_GOT_TLSGD16     R_PPC = 79 // R_POWERPC_GOT_TLSGD16
	R_PPC_GOT_TLSGD16_LO  R_PPC = 80 // R_POWERPC_GOT_TLSGD16_LO
	R_PPC_GOT_TLSGD16_HI  R_PPC = 81 // R_POWERPC_GOT_TLSGD16_HI
	R_PPC_GOT_TLSGD16_HA  R_PPC = 82 // R_POWERPC_GOT_TLSGD16_HA
	R_PPC_GOT_TLSLD16     R_PPC = 83 // R_POWERPC_GOT_TLSLD16
	R_PPC_GOT_TLSLD16_LO  R_PPC = 84 // R_POWERPC_GOT_TLSLD16_LO
	R_PPC_GOT_TLSLD16_HI  R_PPC = 85 // R_POWERPC_GOT_TLSLD16_HI
	R_PPC_GOT_TLSLD16_HA  R_PPC = 86 // R_POWERPC_GOT_TLSLD16_HA
	R_PPC_GOT_TPREL16     R_PPC = 87 // R_POWERPC_GOT_TPREL16
	R_PPC_GOT_TPREL16_LO  R_PPC = 88 // R_POWERPC_GOT_TPREL16_LO
	R_PPC_GOT_TPREL16_HI  R_PPC = 89 // R_POWERPC_GOT_TPREL16_HI
	R_PPC_GOT_TPREL16_HA  R_PPC = 90 // R_POWERPC_GOT_TPREL16_HA
	R_PPC_EMB_NADDR32     R_PPC = 101
	R_PPC_EMB_NADDR16     R_PPC = 102
	R_PPC_EMB_NADDR16_LO  R_PPC = 103
	R_PPC_EMB_NADDR16_HI  R_PPC = 104
	R_PPC_EMB_NADDR16_HA  R_PPC = 105
	R_PPC_EMB_SDAI16      R_PPC = 106
	R_PPC_EMB_SDA2I16     R_PPC = 107
	R_PPC_EMB_SDA2REL     R_PPC = 108
	R_PPC_EMB_SDA21       R_PPC = 109
	R_PPC_EMB_MRKREF      R_PPC = 110
	R_PPC_EMB_RELSEC16    R_PPC = 111
	R_PPC_EMB_RELST_LO    R_PPC = 112
	R_PPC_EMB_RELST_HI    R_PPC = 113
	R_PPC_EMB_RELST_HA    R_PPC = 114
	R_PPC_EMB_BIT_FLD     R_PPC = 115
	R_PPC_EMB_RELSDA      R_PPC = 116
)

var rppcStrings = []intName{
	{0, "R_PPC_NONE"},
	{1, "R_PPC_ADDR32"},
	{2, "R_PPC_ADDR24"},
	{3, "R_PPC_ADDR16"},
	{4, "R_PPC_ADDR16_LO"},
	{5, "R_PPC_ADDR16_HI"},
	{6, "R_PPC_ADDR16_HA"},
	{7, "R_PPC_ADDR14"},
	{8, "R_PPC_ADDR14_BRTAKEN"},
	{9, "R_PPC_ADDR14_BRNTAKEN"},
	{10, "R_PPC_REL24"},
	{11, "R_PPC_REL14"},
	{12, "R_PPC_REL14_BRTAKEN"},
	{13, "R_PPC_REL14_BRNTAKEN"},
	{14, "R_PPC_GOT16"},
	{15, "R_PPC_GOT16_LO"},
	{16, "R_PPC_GOT16_HI"},
	{17, "R_PPC_GOT16_HA"},
	{18, "R_PPC_PLTREL24"},
	{19, "R_PPC_COPY"},
	{20, "R_PPC_GLOB_DAT"},
	{21, "R_PPC_JMP_SLOT"},
	{22, "R_PPC_RELATIVE"},
	{23, "R_PPC_LOCAL24PC"},
	{24, "R_PPC_UADDR32"},
	{25, "R_PPC_UADDR16"},
	{26, "R_PPC_REL32"},
	{27, "R_PPC_PLT32"},
	{28, "R_PPC_PLTREL32"},
	{29, "R_PPC_PLT16_LO"},
	{30, "R_PPC_PLT16_HI"},
	{31, "R_PPC_PLT16_HA"},
	{32, "R_PPC_SDAREL16"},
	{33, "R_PPC_SECTOFF"},
	{34, "R_PPC_SECTOFF_LO"},
	{35, "R_PPC_SECTOFF_HI"},
	{36, "R_PPC_SECTOFF_HA"},
	{67, "R_PPC_TLS"},
	{68, "R_PPC_DTPMOD32"},
	{69, "R_PPC_TPREL16"},
	{70, "R_PPC_TPREL16_LO"},
	{71, "R_PPC_TPREL16_HI"},
	{72, "R_PPC_TPREL16_HA"},
	{73, "R_PPC_TPREL32"},
	{74, "R_PPC_DTPREL16"},
	{75, "R_PPC_DTPREL16_LO"},
	{76, "R_PPC_DTPREL16_HI"},
	{77, "R_PPC_DTPREL16_HA"},
	{78, "R_PPC_DTPREL32"},
	{79, "R_PPC_GOT_TLSGD16"},
	{80, "R_PPC_GOT_TLSGD16_LO"},
	{81, "R_PPC_GOT_TLSGD16_HI"},
	{82, "R_PPC_GOT_TLSGD16_HA"},
	{83, "R_PPC_GOT_TLSLD16"},
	{84, "R_PPC_GOT_TLSLD16_LO"},
	{85, "R_PPC_GOT_TLSLD16_HI"},
	{86, "R_PPC_GOT_TLSLD16_HA"},
	{87, "R_PPC_GOT_TPREL16"},
	{88, "R_PPC_GOT_TPREL16_LO"},
	{89, "R_PPC_GOT_TPREL16_HI"},
	{90, "R_PPC_GOT_TPREL16_HA"},
	{101, "R_PPC_EMB_NADDR32"},
	{102, "R_PPC_EMB_NADDR16"},
	{103, "R_PPC_EMB_NADDR16_LO"},
	{104, "R_PPC_EMB_NADDR16_HI"},
	{105, "R_PPC_EMB_NADDR16_HA"},
	{106, "R_PPC_EMB_SDAI16"},
	{107, "R_PPC_EMB_SDA2I16"},
	{108, "R_PPC_EMB_SDA2REL"},
	{109, "R_PPC_EMB_SDA21"},
	{110, "R_PPC_EMB_MRKREF"},
	{111, "R_PPC_EMB_RELSEC16"},
	{112, "R_PPC_EMB_RELST_LO"},
	{113, "R_PPC_EMB_RELST_HI"},
	{114, "R_PPC_EMB_RELST_HA"},
	{115, "R_PPC_EMB_BIT_FLD"},
	{116, "R_PPC_EMB_RELSDA"},
}

func (i R_PPC) String() string   { return stringName(uint32(i), rppcStrings, false) }
func (i R_PPC) GoString() string { return stringName(uint32(i), rppcStrings, true) }

// Relocation types for 64-bit PowerPC or Power Architecture processors.
//
// Values that are shared by both R_PPC and R_PPC64 are prefixed with
// R_POWERPC_ in the ELF standard. For the R_PPC64 type, the relevant
// shared relocations have been renamed with the prefix R_PPC64_.
// The original name follows the value in a comment.
type R_PPC64 int

const (
	R_PPC64_NONE               R_PPC64 = 0  // R_POWERPC_NONE
	R_PPC64_ADDR32             R_PPC64 = 1  // R_POWERPC_ADDR32
	R_PPC64_ADDR24             R_PPC64 = 2  // R_POWERPC_ADDR24
	R_PPC64_ADDR16             R_PPC64 = 3  // R_POWERPC_ADDR16
	R_PPC64_ADDR16_LO          R_PPC64 = 4  // R_POWERPC_ADDR16_LO
	R_PPC64_ADDR16_HI          R_PPC64 = 5  // R_POWERPC_ADDR16_HI
	R_PPC64_ADDR16_HA          R_PPC64 = 6  // R_POWERPC_ADDR16_HA
	R_PPC64_ADDR14             R_PPC64 = 7  // R_POWERPC_ADDR14
	R_PPC64_ADDR14_BRTAKEN     R_PPC64 = 8  // R_POWERPC_ADDR14_BRTAKEN
	R_PPC64_ADDR14_BRNTAKEN    R_PPC64 = 9  // R_POWERPC_ADDR14_BRNTAKEN
	R_PPC64_REL24              R_PPC64 = 10 // R_POWERPC_REL24
	R_PPC64_REL14              R_PPC64 = 11 // R_POWERPC_REL14
	R_PPC64_REL14_BRTAKEN      R_PPC64 = 12 // R_POWERPC_REL14_BRTAKEN
	R_PPC64_REL14_BRNTAKEN     R_PPC64 = 13 // R_POWERPC_REL14_BRNTAKEN
	R_PPC64_GOT16              R_PPC64 = 14 // R_POWERPC_GOT16
	R_PPC64_GOT16_LO           R_PPC64 = 15 // R_POWERPC_GOT16_LO
	R_PPC64_GOT16_HI           R_PPC64 = 16 // R_POWERPC_GOT16_HI
	R_PPC64_GOT16_HA           R_PPC64 = 17 // R_POWERPC_GOT16_HA
	R_PPC64_COPY               R_PPC64 = 19 // R_POWERPC_COPY
	R_PPC64_GLOB_DAT           R_PPC64 = 20 // R_POWERPC_GLOB_DAT
	R_PPC64_JMP_SLOT           R_PPC64 = 21 // R_POWERPC_JMP_SLOT
	R_PPC64_RELATIVE           R_PPC64 = 22 // R_POWERPC_RELATIVE
	R_PPC64_UADDR32            R_PPC64 = 24 // R_POWERPC_UADDR32
	R_PPC64_UADDR16            R_PPC64 = 25 // R_POWERPC_UADDR16
	R_PPC64_REL32              R_PPC64 = 26 // R_POWERPC_REL32
	R_PPC64_PLT32              R_PPC64 = 27 // R_POWERPC_PLT32
	R_PPC64_PLTREL32           R_PPC64 = 28 // R_POWERPC_PLTREL32
	R_PPC64_PLT16_LO           R_PPC64 = 29 // R_POWERPC_PLT16_LO
	R_PPC64_PLT16_HI           R_PPC64 = 30 // R_POWERPC_PLT16_HI
	R_PPC64_PLT16_HA           R_PPC64 = 31 // R_POWERPC_PLT16_HA
	R_PPC64_SECTOFF            R_PPC64 = 33 // R_POWERPC_SECTOFF
	R_PPC64_SECTOFF_LO         R_PPC64 = 34 // R_POWERPC_SECTOFF_LO
	R_PPC64_SECTOFF_HI         R_PPC64 = 35 // R_POWERPC_SECTOFF_HI
	R_PPC64_SECTOFF_HA         R_PPC64 = 36 // R_POWERPC_SECTOFF_HA
	R_PPC64_REL30              R_PPC64 = 37 // R_POWERPC_ADDR30
	R_PPC64_ADDR64             R_PPC64 = 38
	R_PPC64_ADDR16_HIGHER      R_PPC64 = 39
	R_PPC64_ADDR16_HIGHERA     R_PPC64 = 40
	R_PPC64_ADDR16_HIGHEST     R_PPC64 = 41
	R_PPC64_ADDR16_HIGHESTA    R_PPC64 = 42
	R_PPC64_UADDR64            R_PPC64 = 43
	R_PPC64_REL64              R_PPC64 = 44
	R_PPC64_PLT64              R_PPC64 = 45
	R_PPC64_PLTREL64           R_PPC64 = 46
	R_PPC64_TOC16              R_PPC64 = 47
	R_PPC64_TOC16_LO           R_PPC64 = 48
	R_PPC64_TOC16_HI           R_PPC64 = 49
	R_PPC64_TOC16_HA           R_PPC64 = 50
	R_PPC64_TOC                R_PPC64 = 51
	R_PPC64_PLTGOT16           R_PPC64 = 52
	R_PPC64_PLTGOT16_LO        R_PPC64 = 53
	R_PPC64_PLTGOT16_HI        R_PPC64 = 54
	R_PPC64_PLTGOT16_HA        R_PPC64 = 55
	R_PPC64_ADDR16_DS          R_PPC64 = 56
	R_PPC64_ADDR16_LO_DS       R_PPC64 = 57
	R_PPC64_GOT16_DS           R_PPC64 = 58
	R_PPC64_GOT16_LO_DS        R_PPC64 = 59
	R_PPC64_PLT16_LO_DS        R_PPC64 = 60
	R_PPC64_SECTOFF_DS         R_PPC64 = 61
	R_PPC64_SECTOFF_LO_DS      R_PPC64 = 62
	R_PPC64_TOC16_DS           R_PPC64 = 63
	R_PPC64_TOC16_LO_DS        R_PPC64 = 64
	R_PPC64_PLTGOT16_DS        R_PPC64 = 65
	R_PPC64_PLTGOT_LO_DS       R_PPC64 = 66
	R_PPC64_TLS                R_PPC64 = 67 // R_POWERPC_TLS
	R_PPC64_DTPMOD64           R_PPC64 = 68 // R_POWERPC_DTPMOD64
	R_PPC64_TPREL16            R_PPC64 = 69 // R_POWERPC_TPREL16
	R_PPC64_TPREL16_LO         R_PPC64 = 70 // R_POWERPC_TPREL16_LO
	R_PPC64_TPREL16_HI         R_PPC64 = 71 // R_POWERPC_TPREL16_HI
	R_PPC64_TPREL16_HA         R_PPC64 = 72 // R_POWERPC_TPREL16_HA
	R_PPC64_TPREL64            R_PPC64 = 73 // R_POWERPC_TPREL64
	R_PPC64_DTPREL16           R_PPC64 = 74 // R_POWERPC_DTPREL16
	R_PPC64_DTPREL16_LO        R_PPC64 = 75 // R_POWERPC_DTPREL16_LO
	R_PPC64_DTPREL16_HI        R_PPC64 = 76 // R_POWERPC_DTPREL16_HI
	R_PPC64_DTPREL16_HA        R_PPC64 = 77 // R_POWERPC_DTPREL16_HA
	R_PPC64_DTPREL64           R_PPC64 = 78 // R_POWERPC_DTPREL64
	R_PPC64_GOT_TLSGD16        R_PPC64 = 79 // R_POWERPC_GOT_TLSGD16
	R_PPC64_GOT_TLSGD16_LO     R_PPC64 = 80 // R_POWERPC_GOT_TLSGD16_LO
	R_PPC64_GOT_TLSGD16_HI     R_PPC64 = 81 // R_POWERPC_GOT_TLSGD16_HI
	R_PPC64_GOT_TLSGD16_HA     R_PPC64 = 82 // R_POWERPC_GOT_TLSGD16_HA
	R_PPC64_GOT_TLSLD16        R_PPC64 = 83 // R_POWERPC_GOT_TLSLD16
	R_PPC64_GOT_TLSLD16_LO     R_PPC64 = 84 // R_POWERPC_GOT_TLSLD16_LO
	R_PPC64_GOT_TLSLD16_HI     R_PPC64 = 85 // R_POWERPC_GOT_TLSLD16_HI
	R_PPC64_GOT_TLSLD16_HA     R_PPC64 = 86 // R_POWERPC_GOT_TLSLD16_HA
	R_PPC64_GOT_TPREL16_DS     R_PPC64 = 87 // R_POWERPC_GOT_TPREL16_DS
	R_PPC64_GOT_TPREL16_LO_DS  R_PPC64 = 88 // R_POWERPC_GOT_TPREL16_LO_DS
	R_PPC64_GOT_TPREL16_HI     R_PPC64 = 89 // R_POWERPC_GOT_TPREL16_HI
	R_PPC64_GOT_TPREL16_HA     R_PPC64 = 90 // R_POWERPC_GOT_TPREL16_HA
	R_PPC64_GOT_DTPREL16_DS    R_PPC64 = 91 // R_POWERPC_GOT_DTPREL16_DS
	R_PPC64_GOT_DTPREL16_LO_DS R_PPC64 = 92 // R_POWERPC_GOT_DTPREL16_LO_DS
	R_PPC64_GOT_DTPREL16_HI    R_PPC64 = 93 // R_POWERPC_GOT_DTPREL16_HI
	R_PPC64_GOT_DTPREL16_HA    R_PPC64 = 94 // R_POWERPC_GOT_DTPREL16_HA
	R_PPC64_TPREL16_DS         R_PPC64 = 95
	R_PPC64_TPREL16_LO_DS      R_PPC64 = 96
	R_PPC64_TPREL16_HIGHER     R_PPC64 = 97
	R_PPC64_TPREL16_HIGHERA    R_PPC64 = 98
	R_PPC64_TPREL16_HIGHEST    R_PPC64 = 99
	R_PPC64_TPREL16_HIGHESTA   R_PPC64 = 100
	R_PPC64_DTPREL16_DS        R_PPC64 = 101
	R_PPC64_DTPREL16_LO_DS     R_PPC64 = 102
	R_PPC64_DTPREL16_HIGHER    R_PPC64 = 103
	R_PPC64_DTPREL16_HIGHERA   R_PPC64 = 104
	R_PPC64_DTPREL16_HIGHEST   R_PPC64 = 105
	R_PPC64_DTPREL16_HIGHESTA  R_PPC64 = 106
	R_PPC64_TLSGD              R_PPC64 = 107
	R_PPC64_TLSLD              R_PPC64 = 108
	R_PPC64_TOCSAVE            R_PPC64 = 109
	R_PPC64_ADDR16_HIGH        R_PPC64 = 110
	R_PPC64_ADDR16_HIGHA       R_PPC64 = 111
	R_PPC64_TPREL16_HIGH       R_PPC64 = 112
	R_PPC64_TPREL16_HIGHA      R_PPC64 = 113
	R_PPC64_DTPREL16_HIGH      R_PPC64 = 114
	R_PPC64_DTPREL16_HIGHA     R_PPC64 = 115
	R_PPC64_REL24_NOTOC        R_PPC64 = 116
	R_PPC64_ADDR64_LOCAL       R_PPC64 = 117
	R_PPC64_ENTRY              R_PPC64 = 118
	R_PPC64_PLTSEQ             R_PPC64 = 119
	R_PPC64_PLTCALL            R_PPC64 = 120
	R_PPC64_PLTSEQ_NOTOC       R_PPC64 = 121
	R_PPC64_PLTCALL_NOTOC      R_PPC64 = 122
	R_PPC64_PCREL_OPT          R_PPC64 = 123
	R_PPC64_REL24_P9NOTOC      R_PPC64 = 124
	R_PPC64_D34                R_PPC64 = 128
	R_PPC64_D34_LO             R_PPC64 = 129
	R_PPC64_D34_HI30           R_PPC64 = 130
	R_PPC64_D34_HA30           R_PPC64 = 131
	R_PPC64_PCREL34            R_PPC64 = 132
	R_PPC64_GOT_PCREL34        R_PPC64 = 133
	R_PPC64_PLT_PCREL34        R_PPC64 = 134
	R_PPC64_PLT_PCREL34_NOTOC  R_PPC64 = 135
	R_PPC64_ADDR16_HIGHER34    R_PPC64 = 136
	R_PPC64_ADDR16_HIGHERA34   R_PPC64 = 137
	R_PPC64_ADDR16_HIGHEST34   R_PPC64 = 138
	R_PPC64_ADDR16_HIGHESTA34  R_PPC64 = 139
	R_PPC64_REL16_HIGHER34     R_PPC64 = 140
	R_PPC64_REL16_HIGHERA34    R_PPC64 = 141
	R_PPC64_REL16_HIGHEST34    R_PPC64 = 142
	R_PPC64_REL16_HIGHESTA34   R_PPC64 = 143
	R_PPC64_D28                R_PPC64 = 144
	R_PPC64_PCREL28            R_PPC64 = 145
	R_PPC64_TPREL34            R_PPC64 = 146
	R_PPC64_DTPREL34           R_PPC64 = 147
	R_PPC64_GOT_TLSGD_PCREL34  R_PPC64 = 148
	R_PPC64_GOT_TLSLD_PCREL34  R_PPC64 = 149
	R_PPC64_GOT_TPREL_PCREL34  R_PPC64 = 150
	R_PPC64_GOT_DTPREL_PCREL34 R_PPC64 = 151
	R_PPC64_REL16_HIGH         R_PPC64 = 240
	R_PPC64_REL16_HIGHA        R_PPC64 = 241
	R_PPC64_REL16_HIGHER       R_PPC64 = 242
	R_PPC64_REL16_HIGHERA      R_PPC64 = 243
	R_PPC64_REL16_HIGHEST      R_PPC64 = 244
	R_PPC64_REL16_HIGHESTA     R_PPC64 = 245
	R_PPC64_REL16DX_HA         R_PPC64 = 246 // R_POWERPC_REL16DX_HA
	R_PPC64_JMP_IREL           R_PPC64 = 247
	R_PPC64_IRELATIVE          R_PPC64 = 248 // R_POWERPC_IRELATIVE
	R_PPC64_REL16              R_PPC64 = 249 // R_POWERPC_REL16
	R_PPC64_REL16_LO           R_PPC64 = 250 // R_POWERPC_REL16_LO
	R_PPC64_REL16_HI           R_PPC64 = 251 // R_POWERPC_REL16_HI
	R_PPC64_REL16_HA           R_PPC64 = 252 // R_POWERPC_REL16_HA
	R_PPC64_GNU_VTINHERIT      R_PPC64 = 253
	R_PPC64_GNU_VTENTRY        R_PPC64 = 254
)

var rppc64Strings = []intName{
	{0, "R_PPC64_NONE"},
	{1, "R_PPC64_ADDR32"},
	{2, "R_PPC64_ADDR24"},
	{3, "R_PPC64_ADDR16"},
	{4, "R_PPC64_ADDR16_LO"},
	{5, "R_PPC64_ADDR16_HI"},
	{6, "R_PPC64_ADDR16_HA"},
	{7, "R_PPC64_ADDR14"},
	{8, "R_PPC64_ADDR14_BRTAKEN"},
	{9, "R_PPC64_ADDR14_BRNTAKEN"},
	{10, "R_PPC64_REL24"},
	{11, "R_PPC64_REL14"},
	{12, "R_PPC64_REL14_BRTAKEN"},
	{13, "R_PPC64_REL14_BRNTAKEN"},
	{14, "R_PPC64_GOT16"},
	{15, "R_PPC64_GOT16_LO"},
	{16, "R_PPC64_GOT16_HI"},
	{17, "R_PPC64_GOT16_HA"},
	{19, "R_PPC64_COPY"},
	{20, "R_PPC64_GLOB_DAT"},
	{21, "R_PPC64_JMP_SLOT"},
	{22, "R_PPC64_RELATIVE"},
	{24, "R_PPC64_UADDR32"},
	{25, "R_PPC64_UADDR16"},
	{26, "R_PPC64_REL32"},
	{27, "R_PPC64_PLT32"},
	{28, "R_PPC64_PLTREL32"},
	{29, "R_PPC64_PLT16_LO"},
	{30, "R_PPC64_PLT16_HI"},
	{31, "R_PPC64_PLT16_HA"},
	{33, "R_PPC64_SECTOFF"},
	{34, "R_PPC64_SECTOFF_LO"},
	{35, "R_PPC64_SECTOFF_HI"},
	{36, "R_PPC64_SECTOFF_HA"},
	{37, "R_PPC64_REL30"},
	{38, "R_PPC64_ADDR64"},
	{39, "R_PPC64_ADDR16_HIGHER"},
	{40, "R_PPC64_ADDR16_HIGHERA"},
	{41, "R_PPC64_ADDR16_HIGHEST"},
	{42, "R_PPC64_ADDR16_HIGHESTA"},
	{43, "R_PPC64_UADDR64"},
	{44, "R_PPC64_REL64"},
	{45, "R_PPC64_PLT64"},
	{46, "R_PPC64_PLTREL64"},
	{47, "R_PPC64_TOC16"},
	{48, "R_PPC64_TOC16_LO"},
	{49, "R_PPC64_TOC16_HI"},
	{50, "R_PPC64_TOC16_HA"},
	{51, "R_PPC64_TOC"},
	{52, "R_PPC64_PLTGOT16"},
	{53, "R_PPC64_PLTGOT16_LO"},
	{54, "R_PPC64_PLTGOT16_HI"},
	{55, "R_PPC64_PLTGOT16_HA"},
	{56, "R_PPC64_ADDR16_DS"},
	{57, "R_PPC64_ADDR16_LO_DS"},
	{58, "R_PPC64_GOT16_DS"},
	{59, "R_PPC64_GOT16_LO_DS"},
	{60, "R_PPC64_PLT16_LO_DS"},
	{61, "R_PPC64_SECTOFF_DS"},
	{62, "R_PPC64_SECTOFF_LO_DS"},
	{63, "R_PPC64_TOC16_DS"},
	{64, "R_PPC64_TOC16_LO_DS"},
	{65, "R_PPC64_PLTGOT16_DS"},
	{66, "R_PPC64_PLTGOT_LO_DS"},
	{67, "R_PPC64_TLS"},
	{68, "R_PPC64_DTPMOD64"},
	{69, "R_PPC64_TPREL16"},
	{70, "R_PPC64_TPREL16_LO"},
	{71, "R_PPC64_TPREL16_HI"},
	{72, "R_PPC64_TPREL16_HA"},
	{73, "R_PPC64_TPREL64"},
	{74, "R_PPC64_DTPREL16"},
	{75, "R_PPC64_DTPREL16_LO"},
	{76, "R_PPC64_DTPREL16_HI"},
	{77, "R_PPC64_DTPREL16_HA"},
	{78, "R_PPC64_DTPREL64"},
	{79, "R_PPC64_GOT_TLSGD16"},
	{80, "R_PPC64_GOT_TLSGD16_LO"},
	{81, "R_PPC64_GOT_TLSGD16_HI"},
	{82, "R_PPC64_GOT_TLSGD16_HA"},
	{83, "R_PPC64_GOT_TLSLD16"},
	{84, "R_PPC64_GOT_TLSLD16_LO"},
	{85, "R_PPC64_GOT_TLSLD16_HI"},
	{86, "R_PPC64_GOT_TLSLD16_HA"},
	{87, "R_PPC64_GOT_TPREL16_DS"},
	{88, "R_PPC64_GOT_TPREL16_LO_DS"},
	{89, "R_PPC64_GOT_TPREL16_HI"},
	{90, "R_PPC64_GOT_TPREL16_HA"},
	{91, "R_PPC64_GOT_DTPREL16_DS"},
	{92, "R_PPC64_GOT_DTPREL16_LO_DS"},
	{93, "R_PPC64_GOT_DTPREL16_HI"},
	{94, "R_PPC64_GOT_DTPREL16_HA"},
	{95, "R_PPC64_TPREL16_DS"},
	{96, "R_PPC64_TPREL16_LO_DS"},
	{97, "R_PPC64_TPREL16_HIGHER"},
	{98, "R_PPC64_TPREL16_HIGHERA"},
	{99, "R_PPC64_TPREL16_HIGHEST"},
	{100, "R_PPC64_TPREL16_HIGHESTA"},
	{101, "R_PPC64_DTPREL16_DS"},
	{102, "R_PPC64_DTPREL16_LO_DS"},
	{103, "R_PPC64_DTPREL16_HIGHER"},
	{104, "R_PPC64_DTPREL16_HIGHERA"},
	{105, "R_PPC64_DTPREL16_HIGHEST"},
	{106, "R_PPC64_DTPREL16_HIGHESTA"},
	{107, "R_PPC64_TLSGD"},
	{108, "R_PPC64_TLSLD"},
	{109, "R_PPC64_TOCSAVE"},
	{110, "R_PPC64_ADDR16_HIGH"},
	{111, "R_PPC64_ADDR16_HIGHA"},
	{112, "R_PPC64_TPREL16_HIGH"},
	{113, "R_PPC64_TPREL16_HIGHA"},
	{114, "R_PPC64_DTPREL16_HIGH"},
	{115, "R_PPC64_DTPREL16_HIGHA"},
	{116, "R_PPC64_REL24_NOTOC"},
	{117, "R_PPC64_ADDR64_LOCAL"},
	{118, "R_PPC64_ENTRY"},
	{119, "R_PPC64_PLTSEQ"},
	{120, "R_PPC64_PLTCALL"},
	{121, "R_PPC64_PLTSEQ_NOTOC"},
	{122, "R_PPC64_PLTCALL_NOTOC"},
	{123, "R_PPC64_PCREL_OPT"},
	{124, "R_PPC64_REL24_P9NOTOC"},
	{128, "R_PPC64_D34"},
	{129, "R_PPC64_D34_LO"},
	{130, "R_PPC64_D34_HI30"},
	{131, "R_PPC64_D34_HA30"},
	{132, "R_PPC64_PCREL34"},
	{133, "R_PPC64_GOT_PCREL34"},
	{134, "R_PPC64_PLT_PCREL34"},
	{135, "R_PPC64_PLT_PCREL34_NOTOC"},
	{136, "R_PPC64_ADDR16_HIGHER34"},
	{137, "R_PPC64_ADDR16_HIGHERA34"},
	{138, "R_PPC64_ADDR16_HIGHEST34"},
	{139, "R_PPC64_ADDR16_HIGHESTA34"},
	{140, "R_PPC64_REL16_HIGHER34"},
	{141, "R_PPC64_REL16_HIGHERA34"},
	{142, "R_PPC64_REL16_HIGHEST34"},
	{143, "R_PPC64_REL16_HIGHESTA34"},
	{144, "R_PPC64_D28"},
	{145, "R_PPC64_PCREL28"},
	{146, "R_PPC64_TPREL34"},
	{147, "R_PPC64_DTPREL34"},
	{148, "R_PPC64_GOT_TLSGD_PCREL34"},
	{149, "R_PPC64_GOT_TLSLD_PCREL34"},
	{150, "R_PPC64_GOT_TPREL_PCREL34"},
	{151, "R_PPC64_GOT_DTPREL_PCREL34"},
	{240, "R_PPC64_REL16_HIGH"},
	{241, "R_PPC64_REL16_HIGHA"},
	{242, "R_PPC64_REL16_HIGHER"},
	{243, "R_PPC64_REL16_HIGHERA"},
	{244, "R_PPC64_REL16_HIGHEST"},
	{245, "R_PPC64_REL16_HIGHESTA"},
	{246, "R_PPC64_REL16DX_HA"},
	{247, "R_PPC64_JMP_IREL"},
	{248, "R_PPC64_IRELATIVE"},
	{249, "R_PPC64_REL16"},
	{250, "R_PPC64_REL16_LO"},
	{251, "R_PPC64_REL16_HI"},
	{252, "R_PPC64_REL16_HA"},
	{253, "R_PPC64_GNU_VTINHERIT"},
	{254, "R_PPC64_GNU_VTENTRY"},
}

func (i R_PPC64) String() string   { return stringName(uint32(i), rppc64Strings, false) }
func (i R_PPC64) GoString() string { return stringName(uint32(i), rppc64Strings, true) }

// Relocation types for RISC-V processors.
type R_RISCV int

const (
	R_RISCV_NONE          R_RISCV = 0  /* No relocation. */
	R_RISCV_32            R_RISCV = 1  /* Add 32 bit zero extended symbol value */
	R_RISCV_64            R_RISCV = 2  /* Add 64 bit symbol value. */
	R_RISCV_RELATIVE      R_RISCV = 3  /* Add load address of shared object. */
	R_RISCV_COPY          R_RISCV = 4  /* Copy data from shared object. */
	R_RISCV_JUMP_SLOT     R_RISCV = 5  /* Set GOT entry to code address. */
	R_RISCV_TLS_DTPMOD32  R_RISCV = 6  /* 32 bit ID of module containing symbol */
	R_RISCV_TLS_DTPMOD64  R_RISCV = 7  /* ID of module containing symbol */
	R_RISCV_TLS_DTPREL32  R_RISCV = 8  /* 32 bit relative offset in TLS block */
	R_RISCV_TLS_DTPREL64  R_RISCV = 9  /* Relative offset in TLS block */
	R_RISCV_TLS_TPREL32   R_RISCV = 10 /* 32 bit relative offset in static TLS block */
	R_RISCV_TLS_TPREL64   R_RISCV = 11 /* Relative offset in static TLS block */
	R_RISCV_BRANCH        R_RISCV = 16 /* PC-relative branch */
	R_RISCV_JAL           R_RISCV = 17 /* PC-relative jump */
	R_RISCV_CALL          R_RISCV = 18 /* PC-relative call */
	R_RISCV_CALL_PLT      R_RISCV = 19 /* PC-relative call (PLT) */
	R_RISCV_GOT_HI20      R_RISCV = 20 /* PC-relative GOT reference */
	R_RISCV_TLS_GOT_HI20  R_RISCV = 21 /* PC-relative TLS IE GOT offset */
	R_RISCV_TLS_GD_HI20   R_RISCV = 22 /* PC-relative TLS GD reference */
	R_RISCV_PCREL_HI20    R_RISCV = 23 /* PC-relative reference */
	R_RISCV_PCREL_LO12_I  R_RISCV = 24 /* PC-relative reference */
	R_RISCV_PCREL_LO12_S  R_RISCV = 25 /* PC-relative reference */
	R_RISCV_HI20          R_RISCV = 26 /* Absolute address */
	R_RISCV_LO12_I        R_RISCV = 27 /* Absolute address */
	R_RISCV_LO12_S        R_RISCV = 28 /* Absolute address */
	R_RISCV_TPREL_HI20    R_RISCV = 29 /* TLS LE thread offset */
	R_RISCV_TPREL_LO12_I  R_RISCV = 30 /* TLS LE thread offset */
	R_RISCV_TPREL_LO12_S  R_RISCV = 31 /* TLS LE thread offset */
	R_RISCV_TPREL_ADD     R_RISCV = 32 /* TLS LE thread usage */
	R_RISCV_ADD8          R_RISCV = 33 /* 8-bit label addition */
	R_RISCV_ADD16         R_RISCV = 34 /* 16-bit label addition */
	R_RISCV_ADD32         R_RISCV = 35 /* 32-bit label addition */
	R_RISCV_ADD64         R_RISCV = 36 /* 64-bit label addition */
	R_RISCV_SUB8          R_RISCV = 37 /* 8-bit label subtraction */
	R_RISCV_SUB16         R_RISCV = 38 /* 16-bit label subtraction */
	R_RISCV_SUB32         R_RISCV = 39 /* 32-bit label subtraction */
	R_RISCV_SUB64         R_RISCV = 40 /* 64-bit label subtraction */
	R_RISCV_GNU_VTINHERIT R_RISCV = 41 /* GNU C++ vtable hierarchy */
	R_RISCV_GNU_VTENTRY   R_RISCV = 42 /* GNU C++ vtable member usage */
	R_RISCV_ALIGN         R_RISCV = 43 /* Alignment statement */
	R_RISCV_RVC_BRANCH    R_RISCV = 44 /* PC-relative branch offset */
	R_RISCV_RVC_JUMP      R_RISCV = 45 /* PC-relative jump offset */
	R_RISCV_RVC_LUI       R_RISCV = 46 /* Absolute address */
	R_RISCV_GPREL_I       R_RISCV = 47 /* GP-relative reference */
	R_RISCV_GPREL_S       R_RISCV = 48 /* GP-relative reference */
	R_RISCV_TPREL_I       R_RISCV = 49 /* TP-relative TLS LE load */
	R_RISCV_TPREL_S       R_RISCV = 50 /* TP-relative TLS LE store */
	R_RISCV_RELAX         R_RISCV = 51 /* Instruction pair can be relaxed */
	R_RISCV_SUB6          R_RISCV = 52 /* Local label subtraction */
	R_RISCV_SET6          R_RISCV = 53 /* Local label subtraction */
	R_RISCV_SET8          R_RISCV = 54 /* Local label subtraction */
	R_RISCV_SET16         R_RISCV = 55 /* Local label subtraction */
	R_RISCV_SET32         R_RISCV = 56 /* Local label subtraction */
	R_RISCV_32_PCREL      R_RISCV = 57 /* 32-bit PC relative */
)

var rriscvStrings = []intName{
	{0, "R_RISCV_NONE"},
	{1, "R_RISCV_32"},
	{2, "R_RISCV_64"},
	{3, "R_RISCV_RELATIVE"},
	{4, "R_RISCV_COPY"},
	{5, "R_RISCV_JUMP_SLOT"},
	{6, "R_RISCV_TLS_DTPMOD32"},
	{7, "R_RISCV_TLS_DTPMOD64"},
	{8, "R_RISCV_TLS_DTPREL32"},
	{9, "R_RISCV_TLS_DTPREL64"},
	{10, "R_RISCV_TLS_TPREL32"},
	{11, "R_RISCV_TLS_TPREL64"},
	{16, "R_RISCV_BRANCH"},
	{17, "R_RISCV_JAL"},
	{18, "R_RISCV_CALL"},
	{19, "R_RISCV_CALL_PLT"},
	{20, "R_RISCV_GOT_HI20"},
	{21, "R_RISCV_TLS_GOT_HI20"},
	{22, "R_RISCV_TLS_GD_HI20"},
	{23, "R_RISCV_PCREL_HI20"},
	{24, "R_RISCV_PCREL_LO12_I"},
	{25, "R_RISCV_PCREL_LO12_S"},
	{26, "R_RISCV_HI20"},
	{27, "R_RISCV_LO12_I"},
	{28, "R_RISCV_LO12_S"},
	{29, "R_RISCV_TPREL_HI20"},
	{30, "R_RISCV_TPREL_LO12_I"},
	{31, "R_RISCV_TPREL_LO12_S"},
	{32, "R_RISCV_TPREL_ADD"},
	{33, "R_RISCV_ADD8"},
	{34, "R_RISCV_ADD16"},
	{35, "R_RISCV_ADD32"},
	{36, "R_RISCV_ADD64"},
	{37, "R_RISCV_SUB8"},
	{38, "R_RISCV_SUB16"},
	{39, "R_RISCV_SUB32"},
	{40, "R_RISCV_SUB64"},
	{41, "R_RISCV_GNU_VTINHERIT"},
	{42, "R_RISCV_GNU_VTENTRY"},
	{43, "R_RISCV_ALIGN"},
	{44, "R_RISCV_RVC_BRANCH"},
	{45, "R_RISCV_RVC_JUMP"},
	{46, "R_RISCV_RVC_LUI"},
	{47, "R_RISCV_GPREL_I"},
	{48, "R_RISCV_GPREL_S"},
	{49, "R_RISCV_TPREL_I"},
	{50, "R_RISCV_TPREL_S"},
	{51, "R_RISCV_RELAX"},
	{52, "R_RISCV_SUB6"},
	{53, "R_RISCV_SET6"},
	{54, "R_RISCV_SET8"},
	{55, "R_RISCV_SET16"},
	{56, "R_RISCV_SET32"},
	{57, "R_RISCV_32_PCREL"},
}

func (i R_RISCV) String() string   { return stringName(uint32(i), rriscvStrings, false) }
func (i R_RISCV) GoString() string { return stringName(uint32(i), rriscvStrings, true) }

// Relocation types for s390x processors.
type R_390 int

const (
	R_390_NONE        R_390 = 0
	R_390_8           R_390 = 1
	R_390_12          R_390 = 2
	R_390_16          R_390 = 3
	R_390_32          R_390 = 4
	R_390_PC32        R_390 = 5
	R_390_GOT12       R_390 = 6
	R_390_GOT32       R_390 = 7
	R_390_PLT32       R_390 = 8
	R_390_COPY        R_390 = 9
	R_390_GLOB_DAT    R_390 = 10
	R_390_JMP_SLOT    R_390 = 11
	R_390_RELATIVE    R_390 = 12
	R_390_GOTOFF      R_390 = 13
	R_390_GOTPC       R_390 = 14
	R_390_GOT16       R_390 = 15
	R_390_PC16        R_390 = 16
	R_390_PC16DBL     R_390 = 17
	R_390_PLT16DBL    R_390 = 18
	R_390_PC32DBL     R_390 = 19
	R_390_PLT32DBL    R_390 = 20
	R_390_GOTPCDBL    R_390 = 21
	R_390_64          R_390 = 22
	R_390_PC64        R_390 = 23
	R_390_GOT64       R_390 = 24
	R_390_PLT64       R_390 = 25
	R_390_GOTENT      R_390 = 26
	R_390_GOTOFF16    R_390 = 27
	R_390_GOTOFF64    R_390 = 28
	R_390_GOTPLT12    R_390 = 29
	R_390_GOTPLT16    R_390 = 30
	R_390_GOTPLT32    R_390 = 31
	R_390_GOTPLT64    R_390 = 32
	R_390_GOTPLTENT   R_390 = 33
	R_390_GOTPLTOFF16 R_390 = 34
	R_390_GOTPLTOFF32 R_390 = 35
	R_390_GOTPLTOFF64 R_390 = 36
	R_390_TLS_LOAD    R_390 = 37
	R_390_TLS_GDCALL  R_390 = 38
	R_390_TLS_LDCALL  R_390 = 39
	R_390_TLS_GD32    R_390 = 40
	R_390_TLS_GD64    R_390 = 41
	R_390_TLS_GOTIE12 R_390 = 42
	R_390_TLS_GOTIE32 R_390 = 43
	R_390_TLS_GOTIE64 R_390 = 44
	R_390_TLS_LDM32   R_390 = 45
	R_390_TLS_LDM64   R_390 = 46
	R_390_TLS_IE32    R_390 = 47
	R_390_TLS_IE64    R_390 = 48
	R_390_TLS_IEENT   R_390 = 49
	R_390_TLS_LE32    R_390 = 50
	R_390_TLS_LE64    R_390 = 51
	R_390_TLS_LDO32   R_390 = 52
	R_390_TLS_LDO64   R_390 = 53
	R_390_TLS_DTPMOD  R_390 = 54
	R_390_TLS_DTPOFF  R_390 = 55
	R_390_TLS_TPOFF   R_390 = 56
	R_390_20          R_390 = 57
	R_390_GOT20       R_390 = 58
	R_390_GOTPLT20    R_390 = 59
	R_390_TLS_GOTIE20 R_390 = 60
)

var r390Strings = []intName{
	{0, "R_390_NONE"},
	{1, "R_390_8"},
	{2, "R_390_12"},
	{3, "R_390_16"},
	{4, "R_390_32"},
	{5, "R_390_PC32"},
	{6, "R_390_GOT12"},
	{7, "R_390_GOT32"},
	{8, "R_390_PLT32"},
	{9, "R_390_COPY"},
	{10, "R_390_GLOB_DAT"},
	{11, "R_390_JMP_SLOT"},
	{12, "R_390_RELATIVE"},
	{13, "R_390_GOTOFF"},
	{14, "R_390_GOTPC"},
	{15, "R_390_GOT16"},
	{16, "R_390_PC16"},
	{17, "R_390_PC16DBL"},
	{18, "R_390_PLT16DBL"},
	{19, "R_390_PC32DBL"},
	{20, "R_390_PLT32DBL"},
	{21, "R_390_GOTPCDBL"},
	{22, "R_390_64"},
	{23, "R_390_PC64"},
	{24, "R_390_GOT64"},
	{25, "R_390_PLT64"},
	{26, "R_390_GOTENT"},
	{27, "R_390_GOTOFF16"},
	{28, "R_390_GOTOFF64"},
	{29, "R_390_GOTPLT12"},
	{30, "R_390_GOTPLT16"},
	{31, "R_390_GOTPLT32"},
	{32, "R_390_GOTPLT64"},
	{33, "R_390_GOTPLTENT"},
	{34, "R_390_GOTPLTOFF16"},
	{35, "R_390_GOTPLTOFF32"},
	{36, "R_390_GOTPLTOFF64"},
	{37, "R_390_TLS_LOAD"},
	{38, "R_390_TLS_GDCALL"},
	{39, "R_390_TLS_LDCALL"},
	{40, "R_390_TLS_GD32"},
	{41, "R_390_TLS_GD64"},
	{42, "R_390_TLS_GOTIE12"},
	{43, "R_390_TLS_GOTIE32"},
	{44, "R_390_TLS_GOTIE64"},
	{45, "R_390_TLS_LDM32"},
	{46, "R_390_TLS_LDM64"},
	{47, "R_390_TLS_IE32"},
	{48, "R_390_TLS_IE64"},
	{49, "R_390_TLS_IEENT"},
	{50, "R_390_TLS_LE32"},
	{51, "R_390_TLS_LE64"},
	{52, "R_390_TLS_LDO32"},
	{53, "R_390_TLS_LDO64"},
	{54, "R_390_TLS_DTPMOD"},
	{55, "R_390_TLS_DTPOFF"},
	{56, "R_390_TLS_TPOFF"},
	{57, "R_390_20"},
	{58, "R_390_GOT20"},
	{59, "R_390_GOTPLT20"},
	{60, "R_390_TLS_GOTIE20"},
}

func (i R_390) String() string   { return stringName(uint32(i), r390Strings, false) }
func (i R_390) GoString() string { return stringName(uint32(i), r390Strings, true) }

// Relocation types for SPARC.
type R_SPARC int

const (
	R_SPARC_NONE     R_SPARC = 0
	R_SPARC_8        R_SPARC = 1
	R_SPARC_16       R_SPARC = 2
	R_SPARC_32       R_SPARC = 3
	R_SPARC_DISP8    R_SPARC = 4
	R_SPARC_DISP16   R_SPARC = 5
	R_SPARC_DISP32   R_SPARC = 6
	R_SPARC_WDISP30  R_SPARC = 7
	R_SPARC_WDISP22  R_SPARC = 8
	R_SPARC_HI22     R_SPARC = 9
	R_SPARC_22       R_SPARC = 10
	R_SPARC_13       R_SPARC = 11
	R_SPARC_LO10     R_SPARC = 12
	R_SPARC_GOT10    R_SPARC = 13
	R_SPARC_GOT13    R_SPARC = 14
	R_SPARC_GOT22    R_SPARC = 15
	R_SPARC_PC10     R_SPARC = 16
	R_SPARC_PC22     R_SPARC = 17
	R_SPARC_WPLT30   R_SPARC = 18
	R_SPARC_COPY     R_SPARC = 19
	R_SPARC_GLOB_DAT R_SPARC = 20
	R_SPARC_JMP_SLOT R_SPARC = 21
	R_SPARC_RELATIVE R_SPARC = 22
	R_SPARC_UA32     R_SPARC = 23
	R_SPARC_PLT32    R_SPARC = 24
	R_SPARC_HIPLT22  R_SPARC = 25
	R_SPARC_LOPLT10  R_SPARC = 26
	R_SPARC_PCPLT32  R_SPARC = 27
	R_SPARC_PCPLT22  R_SPARC = 28
	R_SPARC_PCPLT10  R_SPARC = 29
	R_SPARC_10       R_SPARC = 30
	R_SPARC_11       R_SPARC = 31
	R_SPARC_64       R_SPARC = 32
	R_SPARC_OLO10    R_SPARC = 33
	R_SPARC_HH22     R_SPARC = 34
	R_SPARC_HM10     R_SPARC = 35
	R_SPARC_LM22     R_SPARC = 36
	R_SPARC_PC_HH22  R_SPARC = 37
	R_SPARC_PC_HM10  R_SPARC = 38
	R_SPARC_PC_LM22  R_SPARC = 39
	R_SPARC_WDISP16  R_SPARC = 40
	R_SPARC_WDISP19  R_SPARC = 41
	R_SPARC_GLOB_JMP R_SPARC = 42
	R_SPARC_7        R_SPARC = 43
	R_SPARC_5        R_SPARC = 44
	R_SPARC_6        R_SPARC = 45
	R_SPARC_DISP64   R_SPARC = 46
	R_SPARC_PLT64    R_SPARC = 47
	R_SPARC_HIX22    R_SPARC = 48
	R_SPARC_LOX10    R_SPARC = 49
	R_SPARC_H44      R_SPARC = 50
	R_SPARC_M44      R_SPARC = 51
	R_SPARC_L44      R_SPARC = 52
	R_SPARC_REGISTER R_SPARC = 53
	R_SPARC_UA64     R_SPARC = 54
	R_SPARC_UA16     R_SPARC = 55
)

var rsparcStrings = []intName{
	{0, "R_SPARC_NONE"},
	{1, "R_SPARC_8"},
	{2, "R_SPARC_16"},
	{3, "R_SPARC_32"},
	{4, "R_SPARC_DISP8"},
	{5, "R_SPARC_DISP16"},
	{6, "R_SPARC_DISP32"},
	{7, "R_SPARC_WDISP30"},
	{8, "R_SPARC_WDISP22"},
	{9, "R_SPARC_HI22"},
	{10, "R_SPARC_22"},
	{11, "R_SPARC_13"},
	{12, "R_SPARC_LO10"},
	{13, "R_SPARC_GOT10"},
	{14, "R_SPARC_GOT13"},
	{15, "R_SPARC_GOT22"},
	{16, "R_SPARC_PC10"},
	{17, "R_SPARC_PC22"},
	{18, "R_SPARC_WPLT30"},
	{19, "R_SPARC_COPY"},
	{20, "R_SPARC_GLOB_DAT"},
	{21, "R_SPARC_JMP_SLOT"},
	{22, "R_SPARC_RELATIVE"},
	{23, "R_SPARC_UA32"},
	{24, "R_SPARC_PLT32"},
	{25, "R_SPARC_HIPLT22"},
	{26, "R_SPARC_LOPLT10"},
	{27, "R_SPARC_PCPLT32"},
	{28, "R_SPARC_PCPLT22"},
	{29, "R_SPARC_PCPLT10"},
	{30, "R_SPARC_10"},
	{31, "R_SPARC_11"},
	{32, "R_SPARC_64"},
	{33, "R_SPARC_OLO10"},
	{34, "R_SPARC_HH22"},
	{35, "R_SPARC_HM10"},
	{36, "R_SPARC_LM22"},
	{37, "R_SPARC_PC_HH22"},
	{38, "R_SPARC_PC_HM10"},
	{39, "R_SPARC_PC_LM22"},
	{40, "R_SPARC_WDISP16"},
	{41, "R_SPARC_WDISP19"},
	{42, "R_SPARC_GLOB_JMP"},
	{43, "R_SPARC_7"},
	{44, "R_SPARC_5"},
	{45, "R_SPARC_6"},
	{46, "R_SPARC_DISP64"},
	{47, "R_SPARC_PLT64"},
	{48, "R_SPARC_HIX22"},
	{49, "R_SPARC_LOX10"},
	{50, "R_SPARC_H44"},
	{51, "R_SPARC_M44"},
	{52, "R_SPARC_L44"},
	{53, "R_SPARC_REGISTER"},
	{54, "R_SPARC_UA64"},
	{55, "R_SPARC_UA16"},
}

func (i R_SPARC) String() string   { return stringName(uint32(i), rsparcStrings, false) }
func (i R_SPARC) GoString() string { return stringName(uint32(i), rsparcStrings, true) }

// Magic number for the elf trampoline, chosen wisely to be an immediate value.
const ARM_MAGIC_TRAMP_NUMBER = 0x5c000003

// ELF32 File header.
type Header32 struct {
	Ident     [EI_NIDENT]byte /* File identification. */
	Type      uint16          /* File type. */
	Machine   uint16          /* Machine architecture. */
	Version   uint32          /* ELF format version. */
	Entry     uint32          /* Entry point. */
	Phoff     uint32          /* Program header file offset. */
	Shoff     uint32          /* Section header file offset. */
	Flags     uint32          /* Architecture-specific flags. */
	Ehsize    uint16          /* Size of ELF header in bytes. */
	Phentsize uint16          /* Size of program header entry. */
	Phnum     uint16          /* Number of program header entries. */
	Shentsize uint16          /* Size of section header entry. */
	Shnum     uint16          /* Number of section header entries. */
	Shstrndx  uint16          /* Section name strings section. */
}

// ELF32 Section header.
type Section32 struct {
	Name      uint32 /* Section name (index into the section header string table). */
	Type      uint32 /* Section type. */
	Flags     uint32 /* Section flags. */
	Addr      uint32 /* Address in memory image. */
	Off       uint32 /* Offset in file. */
	Size      uint32 /* Size in bytes. */
	Link      uint32 /* Index of a related section. */
	Info      uint32 /* Depends on section type. */
	Addralign uint32 /* Alignment in bytes. */
	Entsize   uint32 /* Size of each entry in section. */
}

// ELF32 Program header.
type Prog32 struct {
	Type   uint32 /* Entry type. */
	Off    uint32 /* File offset of contents. */
	Vaddr  uint32 /* Virtual address in memory image. */
	Paddr  uint32 /* Physical address (not used). */
	Filesz uint32 /* Size of contents in file. */
	Memsz  uint32 /* Size of contents in memory. */
	Flags  uint32 /* Access permission flags. */
	Align  uint32 /* Alignment in memory and file. */
}

// ELF32 Dynamic structure. The ".dynamic" section contains an array of them.
type Dyn32 struct {
	Tag int32  /* Entry type. */
	Val uint32 /* Integer/Address value. */
}

// ELF32 Compression header.
type Chdr32 struct {
	Type      uint32
	Size      uint32
	Addralign uint32
}

/*
 * Relocation entries.
 */

// ELF32 Relocations that don't need an addend field.
type Rel32 struct {
	Off  uint32 /* Location to be relocated. */
	Info uint32 /* Relocation type and symbol index. */
}

// ELF32 Relocations that need an addend field.
type Rela32 struct {
	Off    uint32 /* Location to be relocated. */
	Info   uint32 /* Relocation type and symbol index. */
	Addend int32  /* Addend. */
}

func R_SYM32(info uint32) uint32      { return info >> 8 }
func R_TYPE32(info uint32) uint32     { return info & 0xff }
func R_INFO32(sym, typ uint32) uint32 { return sym<<8 | typ }

// ELF32 Symbol.
type Sym32 struct {
	Name  uint32
	Value uint32
	Size  uint32
	Info  uint8
	Other uint8
	Shndx uint16
}

const Sym32Size = 16

func ST_BIND(info uint8) SymBind { return SymBind(info >> 4) }
func ST_TYPE(info uint8) SymType { return SymType(info & 0xF) }
func ST_INFO(bind SymBind, typ SymType) uint8 {
	return uint8(bind)<<4 | uint8(typ)&0xf
}
func ST_VISIBILITY(other uint8) SymVis { return SymVis(other & 3) }

/*
 * ELF64
 */

// ELF64 file header.
type Header64 struct {
	Ident     [EI_NIDENT]byte /* File identification. */
	Type      uint16          /* File type. */
	Machine   uint16          /* Machine architecture. */
	Version   uint32          /* ELF format version. */
	Entry     uint64          /* Entry point. */
	Phoff     uint64          /* Program header file offset. */
	Shoff     uint64          /* Section header file offset. */
	Flags     uint32          /* Architecture-specific flags. */
	Ehsize    uint16          /* Size of ELF header in bytes. */
	Phentsize uint16          /* Size of program header entry. */
	Phnum     uint16          /* Number of program header entries. */
	Shentsize uint16          /* Size of section header entry. */
	Shnum     uint16          /* Number of section header entries. */
	Shstrndx  uint16          /* Section name strings section. */
}

// ELF64 Section header.
type Section64 struct {
	Name      uint32 /* Section name (index into the section header string table). */
	Type      uint32 /* Section type. */
	Flags     uint64 /* Section flags. */
	Addr      uint64 /* Address in memory image. */
	Off       uint64 /* Offset in file. */
	Size      uint64 /* Size in bytes. */
	Link      uint32 /* Index of a related section. */
	Info      uint32 /* Depends on section type. */
	Addralign uint64 /* Alignment in bytes. */
	Entsize   uint64 /* Size of each entry in section. */
}

// ELF64 Program header.
type Prog64 struct {
	Type   uint32 /* Entry type. */
	Flags  uint32 /* Access permission flags. */
	Off    uint64 /* File offset of contents. */
	Vaddr  uint64 /* Virtual address in memory image. */
	Paddr  uint64 /* Physical address (not used). */
	Filesz uint64 /* Size of contents in file. */
	Memsz  uint64 /* Size of contents in memory. */
	Align  uint64 /* Alignment in memory and file. */
}

// ELF64 Dynamic structure. The ".dynamic" section contains an array of them.
type Dyn64 struct {
	Tag int64  /* Entry type. */
	Val uint64 /* Integer/address value */
}

// ELF64 Compression header.
type Chdr64 struct {
	Type      uint32
	_         uint32 /* Reserved. */
	Size      uint64
	Addralign uint64
}

/*
 * Relocation entries.
 */

/* ELF64 relocations that don't need an addend field. */
type Rel64 struct {
	Off  uint64 /* Location to be relocated. */
	Info uint64 /* Relocation type and symbol index. */
}

/* ELF64 relocations that need an addend field. */
type Rela64 struct {
	Off    uint64 /* Location to be relocated. */
	Info   uint64 /* Relocation type and symbol index. */
	Addend int64  /* Addend. */
}

func R_SYM64(info uint64) uint32    { return uint32(info >> 32) }
func R_TYPE64(info uint64) uint32   { return uint32(info) }
func R_INFO(sym, typ uint32) uint64 { return uint64(sym)<<32 | uint64(typ) }

// ELF64 symbol table entries.
type Sym64 struct {
	Name  uint32 /* String table index of name. */
	Info  uint8  /* Type and binding information. */
	Other uint8  /* Reserved (not used). */
	Shndx uint16 /* Section index of symbol. */
	Value uint64 /* Symbol value. */
	Size  uint64 /* Size of associated object. */
}

const Sym64Size = 24

type intName struct {
	i uint32
	s string
}

// Dynamic version flags.
type DynamicVersionFlag uint16

const (
	VER_FLG_BASE DynamicVersionFlag = 0x1 /* Version definition of the file. */
	VER_FLG_WEAK DynamicVersionFlag = 0x2 /* Weak version identifier. */
	VER_FLG_INFO DynamicVersionFlag = 0x4 /* Reference exists for informational purposes. */
)

func stringName(i uint32, names []intName, goSyntax bool) string {
	for _, n := range names {
		if n.i == i {
			if goSyntax {
				return "elf." + n.s
			}
			return n.s
		}
	}

	// second pass - look for smaller to add with.
	// assume sorted already
	for j := len(names) - 1; j >= 0; j-- {
		n := names[j]
		if n.i < i {
			s := n.s
			if goSyntax {
				s = "elf." + s
			}
			return s + "+" + strconv.FormatUint(uint64(i-n.i), 10)
		}
	}

	return strconv.FormatUint(uint64(i), 10)
}

func flagName(i uint32, names []intName, goSyntax bool) string {
	s := ""
	for _, n := range names {
		if n.i&i == n.i {
			if len(s) > 0 {
				s += "+"
			}
			if goSyntax {
				s += "elf."
			}
			s += n.s
			i -= n.i
		}
	}
	if len(s) == 0 {
		return "0x" + strconv.FormatUint(uint64(i), 16)
	}
	if i != 0 {
		s += "+0x" + strconv.FormatUint(uint64(i), 16)
	}
	return s
}

```

// === FILE: references!/go/src/debug/elf/file.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package elf implements access to ELF object files.

# Security

This package is not designed to be hardened against adversarial inputs, and is
outside the scope of https://go.dev/security/policy. In particular, only basic
validation is done when parsing object files. As such, care should be taken when
parsing untrusted inputs, as parsing malformed files may consume significant
resources, or cause panics.
*/
package elf

import (
	"bytes"
	"compress/zlib"
	"debug/dwarf"
	"encoding/binary"
	"errors"
	"fmt"
	"internal/saferio"
	"internal/zstd"
	"io"
	"math"
	"os"
	"strings"
	"unsafe"
)

// TODO: error reporting detail

/*
 * Internal ELF representation
 */

// A FileHeader represents an ELF file header.
type FileHeader struct {
	Class      Class
	Data       Data
	Version    Version
	OSABI      OSABI
	ABIVersion uint8
	ByteOrder  binary.ByteOrder
	Type       Type
	Machine    Machine
	Entry      uint64
}

// A File represents an open ELF file.
type File struct {
	FileHeader
	Sections    []*Section
	Progs       []*Prog
	closer      io.Closer
	dynVers     []DynamicVersion
	dynVerNeeds []DynamicVersionNeed
	gnuVersym   []byte
}

// A SectionHeader represents a single ELF section header.
type SectionHeader struct {
	Name      string
	Type      SectionType
	Flags     SectionFlag
	Addr      uint64
	Offset    uint64
	Size      uint64
	Link      uint32
	Info      uint32
	Addralign uint64
	Entsize   uint64

	// FileSize is the size of this section in the file in bytes.
	// If a section is compressed, FileSize is the size of the
	// compressed data, while Size (above) is the size of the
	// uncompressed data.
	FileSize uint64
}

// A Section represents a single section in an ELF file.
type Section struct {
	SectionHeader

	// Embed ReaderAt for ReadAt method.
	// Do not embed SectionReader directly
	// to avoid having Read and Seek.
	// If a client wants Read and Seek it must use
	// Open() to avoid fighting over the seek offset
	// with other clients.
	//
	// ReaderAt may be nil if the section is not easily available
	// in a random-access form. For example, a compressed section
	// may have a nil ReaderAt.
	io.ReaderAt
	sr *io.SectionReader

	compressionType   CompressionType
	compressionOffset int64
}

// Data reads and returns the contents of the ELF section.
// Even if the section is stored compressed in the ELF file,
// Data returns uncompressed data.
//
// For an [SHT_NOBITS] section, Data always returns a non-nil error.
func (s *Section) Data() ([]byte, error) {
	return saferio.ReadData(s.Open(), s.Size)
}

// stringTable reads and returns the string table given by the
// specified link value.
func (f *File) stringTable(link uint32) ([]byte, error) {
	if link <= 0 || link >= uint32(len(f.Sections)) {
		return nil, errors.New("section has invalid string table link")
	}
	return f.Sections[link].Data()
}

// Open returns a new ReadSeeker reading the ELF section.
// Even if the section is stored compressed in the ELF file,
// the ReadSeeker reads uncompressed data.
//
// For an [SHT_NOBITS] section, all calls to the opened reader
// will return a non-nil error.
func (s *Section) Open() io.ReadSeeker {
	if s.Type == SHT_NOBITS {
		return io.NewSectionReader(&nobitsSectionReader{}, 0, int64(s.Size))
	}

	var zrd func(io.Reader) (io.ReadCloser, error)
	if s.Flags&SHF_COMPRESSED == 0 {

		if !strings.HasPrefix(s.Name, ".zdebug") {
			return io.NewSectionReader(s.sr, 0, 1<<63-1)
		}

		b := make([]byte, 12)
		n, _ := s.sr.ReadAt(b, 0)
		if n != 12 || string(b[:4]) != "ZLIB" {
			return io.NewSectionReader(s.sr, 0, 1<<63-1)
		}

		s.compressionOffset = 12
		s.compressionType = COMPRESS_ZLIB
		s.Size = binary.BigEndian.Uint64(b[4:12])
		zrd = zlib.NewReader

	} else if s.Flags&SHF_ALLOC != 0 {
		return errorReader{&FormatError{int64(s.Offset),
			"SHF_COMPRESSED applies only to non-allocable sections", s.compressionType}}
	}

	switch s.compressionType {
	case COMPRESS_ZLIB:
		zrd = zlib.NewReader
	case COMPRESS_ZSTD:
		zrd = func(r io.Reader) (io.ReadCloser, error) {
			return io.NopCloser(zstd.NewReader(r)), nil
		}
	}

	if zrd == nil {
		return errorReader{&FormatError{int64(s.Offset), "unknown compression type", s.compressionType}}
	}

	return &readSeekerFromReader{
		reset: func() (io.Reader, error) {
			fr := io.NewSectionReader(s.sr, s.compressionOffset, int64(s.FileSize)-s.compressionOffset)
			return zrd(fr)
		},
		size: int64(s.Size),
	}
}

// A ProgHeader represents a single ELF program header.
type ProgHeader struct {
	Type   ProgType
	Flags  ProgFlag
	Off    uint64
	Vaddr  uint64
	Paddr  uint64
	Filesz uint64
	Memsz  uint64
	Align  uint64
}

// A Prog represents a single ELF program header in an ELF binary.
type Prog struct {
	ProgHeader

	// Embed ReaderAt for ReadAt method.
	// Do not embed SectionReader directly
	// to avoid having Read and Seek.
	// If a client wants Read and Seek it must use
	// Open() to avoid fighting over the seek offset
	// with other clients.
	io.ReaderAt
	sr *io.SectionReader
}

// Open returns a new ReadSeeker reading the ELF program body.
func (p *Prog) Open() io.ReadSeeker { return io.NewSectionReader(p.sr, 0, 1<<63-1) }

// A Symbol represents an entry in an ELF symbol table section.
type Symbol struct {
	Name        string
	Info, Other byte

	// HasVersion reports whether the symbol has any version information.
	// This will only be true for the dynamic symbol table.
	HasVersion bool
	// VersionIndex is the symbol's version index.
	// Use the methods of the [VersionIndex] type to access it.
	// This field is only meaningful if HasVersion is true.
	VersionIndex VersionIndex

	Section     SectionIndex
	Value, Size uint64

	// These fields are present only for the dynamic symbol table.
	Version string
	Library string
}

/*
 * ELF reader
 */

type FormatError struct {
	off int64
	msg string
	val any
}

func (e *FormatError) Error() string {
	msg := e.msg
	if e.val != nil {
		msg += fmt.Sprintf(" '%v' ", e.val)
	}
	msg += fmt.Sprintf("in record at byte %#x", e.off)
	return msg
}

// Open opens the named file using [os.Open] and prepares it for use as an ELF binary.
func Open(name string) (*File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	ff, err := NewFile(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	ff.closer = f
	return ff, nil
}

// Close closes the [File].
// If the [File] was created using [NewFile] directly instead of [Open],
// Close has no effect.
func (f *File) Close() error {
	var err error
	if f.closer != nil {
		err = f.closer.Close()
		f.closer = nil
	}
	return err
}

// SectionByType returns the first section in f with the
// given type, or nil if there is no such section.
func (f *File) SectionByType(typ SectionType) *Section {
	for _, s := range f.Sections {
		if s.Type == typ {
			return s
		}
	}
	return nil
}

// NewFile creates a new [File] for accessing an ELF binary in an underlying reader.
// The ELF binary is expected to start at position 0 in the ReaderAt.
func NewFile(r io.ReaderAt) (*File, error) {
	sr := io.NewSectionReader(r, 0, 1<<63-1)
	// Read and decode ELF identifier
	var ident [16]uint8
	if _, err := r.ReadAt(ident[0:], 0); err != nil {
		return nil, &FormatError{0, "cannot read ELF identifier", err}
	}
	if ident[0] != '\x7f' || ident[1] != 'E' || ident[2] != 'L' || ident[3] != 'F' {
		return nil, &FormatError{0, "bad magic number", ident[0:4]}
	}

	f := new(File)
	f.Class = Class(ident[EI_CLASS])
	switch f.Class {
	case ELFCLASS32:
	case ELFCLASS64:
		// ok
	default:
		return nil, &FormatError{0, "unknown ELF class", f.Class}
	}

	f.Data = Data(ident[EI_DATA])
	var bo binary.ByteOrder
	switch f.Data {
	case ELFDATA2LSB:
		bo = binary.LittleEndian
	case ELFDATA2MSB:
		bo = binary.BigEndian
	default:
		return nil, &FormatError{0, "unknown ELF data encoding", f.Data}
	}
	f.ByteOrder = bo

	f.Version = Version(ident[EI_VERSION])
	if f.Version != EV_CURRENT {
		return nil, &FormatError{0, "unknown ELF version", f.Version}
	}

	f.OSABI = OSABI(ident[EI_OSABI])
	f.ABIVersion = ident[EI_ABIVERSION]

	// Read ELF file header
	var phoff int64
	var phentsize, phnum int
	var shoff int64
	var shentsize, shnum, shstrndx int
	switch f.Class {
	case ELFCLASS32:
		var hdr Header32
		data := make([]byte, unsafe.Sizeof(hdr))
		if _, err := sr.ReadAt(data, 0); err != nil {
			return nil, err
		}
		f.Type = Type(bo.Uint16(data[unsafe.Offsetof(hdr.Type):]))
		f.Machine = Machine(bo.Uint16(data[unsafe.Offsetof(hdr.Machine):]))
		f.Entry = uint64(bo.Uint32(data[unsafe.Offsetof(hdr.Entry):]))
		if v := Version(bo.Uint32(data[unsafe.Offsetof(hdr.Version):])); v != f.Version {
			return nil, &FormatError{0, "mismatched ELF version", v}
		}
		phoff = int64(bo.Uint32(data[unsafe.Offsetof(hdr.Phoff):]))
		phentsize = int(bo.Uint16(data[unsafe.Offsetof(hdr.Phentsize):]))
		phnum = int(bo.Uint16(data[unsafe.Offsetof(hdr.Phnum):]))
		shoff = int64(bo.Uint32(data[unsafe.Offsetof(hdr.Shoff):]))
		shentsize = int(bo.Uint16(data[unsafe.Offsetof(hdr.Shentsize):]))
		shnum = int(bo.Uint16(data[unsafe.Offsetof(hdr.Shnum):]))
		shstrndx = int(bo.Uint16(data[unsafe.Offsetof(hdr.Shstrndx):]))
	case ELFCLASS64:
		var hdr Header64
		data := make([]byte, unsafe.Sizeof(hdr))
		if _, err := sr.ReadAt(data, 0); err != nil {
			return nil, err
		}
		f.Type = Type(bo.Uint16(data[unsafe.Offsetof(hdr.Type):]))
		f.Machine = Machine(bo.Uint16(data[unsafe.Offsetof(hdr.Machine):]))
		f.Entry = bo.Uint64(data[unsafe.Offsetof(hdr.Entry):])
		if v := Version(bo.Uint32(data[unsafe.Offsetof(hdr.Version):])); v != f.Version {
			return nil, &FormatError{0, "mismatched ELF version", v}
		}
		phoff = int64(bo.Uint64(data[unsafe.Offsetof(hdr.Phoff):]))
		phentsize = int(bo.Uint16(data[unsafe.Offsetof(hdr.Phentsize):]))
		phnum = int(bo.Uint16(data[unsafe.Offsetof(hdr.Phnum):]))
		shoff = int64(bo.Uint64(data[unsafe.Offsetof(hdr.Shoff):]))
		shentsize = int(bo.Uint16(data[unsafe.Offsetof(hdr.Shentsize):]))
		shnum = int(bo.Uint16(data[unsafe.Offsetof(hdr.Shnum):]))
		shstrndx = int(bo.Uint16(data[unsafe.Offsetof(hdr.Shstrndx):]))
	}

	if shoff < 0 {
		return nil, &FormatError{0, "invalid shoff", shoff}
	}
	if phoff < 0 {
		return nil, &FormatError{0, "invalid phoff", phoff}
	}

	if shoff == 0 && shnum != 0 {
		return nil, &FormatError{0, "invalid ELF shnum for shoff=0", shnum}
	}

	if shnum > 0 && shstrndx >= shnum {
		return nil, &FormatError{0, "invalid ELF shstrndx", shstrndx}
	}

	var wantPhentsize, wantShentsize int
	switch f.Class {
	case ELFCLASS32:
		wantPhentsize = 8 * 4
		wantShentsize = 10 * 4
	case ELFCLASS64:
		wantPhentsize = 2*4 + 6*8
		wantShentsize = 4*4 + 6*8
	}
	if phnum > 0 && phentsize < wantPhentsize {
		return nil, &FormatError{0, "invalid ELF phentsize", phentsize}
	}

	// If the number of sections is greater than or equal to SHN_LORESERVE
	// (0xff00), shnum has the value zero and the actual number of section
	// header table entries is contained in the sh_size field of the section
	// header at index 0.
	//
	// If the number of segments is greater than or equal to 0xffff,
	// phnum has the value 0xffff, and the actual number of segments
	// is contained in the sh_info field of the section header at
	// index 0.
	const pnXnum = 0xffff
	if shoff > 0 && (shnum == 0 || phnum == pnXnum) {
		var typ, link, info uint32
		var size uint64
		sr.Seek(shoff, io.SeekStart)
		switch f.Class {
		case ELFCLASS32:
			sh := new(Section32)
			if err := binary.Read(sr, bo, sh); err != nil {
				return nil, err
			}
			size = uint64(sh.Size)
			typ = sh.Type
			link = sh.Link
			info = sh.Info
		case ELFCLASS64:
			sh := new(Section64)
			if err := binary.Read(sr, bo, sh); err != nil {
				return nil, err
			}
			size = sh.Size
			typ = sh.Type
			link = sh.Link
			info = sh.Info
		}

		if SectionType(typ) != SHT_NULL {
			return nil, &FormatError{shoff, "invalid type of the initial section", SectionType(typ)}
		}

		if shnum == 0 {
			if size < uint64(SHN_LORESERVE) {
				return nil, &FormatError{shoff, "invalid ELF shnum contained in sh_size", shnum}
			}
			shnum = int(size)
		}

		if phnum == pnXnum {
			if info < 0xffff {
				return nil, &FormatError{shoff, "invalid ELF phnum contained in sh_info", info}
			}
			phnum = int(info)
		}

		// If the section name string table section index is greater than or
		// equal to SHN_LORESERVE (0xff00), this member has the value
		// SHN_XINDEX (0xffff) and the actual index of the section name
		// string table section is contained in the sh_link field of the
		// section header at index 0.
		if shstrndx == int(SHN_XINDEX) {
			shstrndx = int(link)
			if shstrndx < int(SHN_LORESERVE) || shstrndx >= shnum {
				return nil, &FormatError{shoff, "invalid ELF shstrndx contained in sh_link", shstrndx}
			}
		}
	}

	// Read program headers
	c := saferio.SliceCap[*Prog](uint64(phnum))
	if c < 0 {
		return nil, &FormatError{0, "too many segments", phnum}
	}
	if phnum > 0 && ((1<<64)-1)/uint64(phnum) < uint64(phentsize) {
		return nil, &FormatError{0, "segment header overflow", phnum}
	}
	f.Progs = make([]*Prog, 0, c)
	phdata, err := saferio.ReadDataAt(sr, uint64(phnum)*uint64(phentsize), phoff)
	if err != nil {
		return nil, err
	}
	for i := 0; i < phnum; i++ {
		off := uintptr(i) * uintptr(phentsize)
		p := new(Prog)
		switch f.Class {
		case ELFCLASS32:
			var ph Prog32
			p.ProgHeader = ProgHeader{
				Type:   ProgType(bo.Uint32(phdata[off+unsafe.Offsetof(ph.Type):])),
				Flags:  ProgFlag(bo.Uint32(phdata[off+unsafe.Offsetof(ph.Flags):])),
				Off:    uint64(bo.Uint32(phdata[off+unsafe.Offsetof(ph.Off):])),
				Vaddr:  uint64(bo.Uint32(phdata[off+unsafe.Offsetof(ph.Vaddr):])),
				Paddr:  uint64(bo.Uint32(phdata[off+unsafe.Offsetof(ph.Paddr):])),
				Filesz: uint64(bo.Uint32(phdata[off+unsafe.Offsetof(ph.Filesz):])),
				Memsz:  uint64(bo.Uint32(phdata[off+unsafe.Offsetof(ph.Memsz):])),
				Align:  uint64(bo.Uint32(phdata[off+unsafe.Offsetof(ph.Align):])),
			}
		case ELFCLASS64:
			var ph Prog64
			p.ProgHeader = ProgHeader{
				Type:   ProgType(bo.Uint32(phdata[off+unsafe.Offsetof(ph.Type):])),
				Flags:  ProgFlag(bo.Uint32(phdata[off+unsafe.Offsetof(ph.Flags):])),
				Off:    bo.Uint64(phdata[off+unsafe.Offsetof(ph.Off):]),
				Vaddr:  bo.Uint64(phdata[off+unsafe.Offsetof(ph.Vaddr):]),
				Paddr:  bo.Uint64(phdata[off+unsafe.Offsetof(ph.Paddr):]),
				Filesz: bo.Uint64(phdata[off+unsafe.Offsetof(ph.Filesz):]),
				Memsz:  bo.Uint64(phdata[off+unsafe.Offsetof(ph.Memsz):]),
				Align:  bo.Uint64(phdata[off+unsafe.Offsetof(ph.Align):]),
			}
		}
		if int64(p.Off) < 0 {
			return nil, &FormatError{phoff + int64(off), "invalid program header offset", p.Off}
		}
		if int64(p.Filesz) < 0 {
			return nil, &FormatError{phoff + int64(off), "invalid program header file size", p.Filesz}
		}
		p.sr = io.NewSectionReader(r, int64(p.Off), int64(p.Filesz))
		p.ReaderAt = p.sr
		f.Progs = append(f.Progs, p)
	}

	if shnum > 0 && shentsize < wantShentsize {
		return nil, &FormatError{0, "invalid ELF shentsize", shentsize}
	}

	// Read section headers
	c = saferio.SliceCap[Section](uint64(shnum))
	if c < 0 {
		return nil, &FormatError{0, "too many sections", shnum}
	}
	if shnum > 0 && ((1<<64)-1)/uint64(shnum) < uint64(shentsize) {
		return nil, &FormatError{0, "section header overflow", shnum}
	}
	f.Sections = make([]*Section, 0, c)
	names := make([]uint32, 0, c)
	shdata, err := saferio.ReadDataAt(sr, uint64(shnum)*uint64(shentsize), shoff)
	if err != nil {
		return nil, err
	}
	for i := 0; i < shnum; i++ {
		off := uintptr(i) * uintptr(shentsize)
		s := new(Section)
		switch f.Class {
		case ELFCLASS32:
			var sh Section32
			names = append(names, bo.Uint32(shdata[off+unsafe.Offsetof(sh.Name):]))
			s.SectionHeader = SectionHeader{
				Type:      SectionType(bo.Uint32(shdata[off+unsafe.Offsetof(sh.Type):])),
				Flags:     SectionFlag(bo.Uint32(shdata[off+unsafe.Offsetof(sh.Flags):])),
				Addr:      uint64(bo.Uint32(shdata[off+unsafe.Offsetof(sh.Addr):])),
				Offset:    uint64(bo.Uint32(shdata[off+unsafe.Offsetof(sh.Off):])),
				FileSize:  uint64(bo.Uint32(shdata[off+unsafe.Offsetof(sh.Size):])),
				Link:      bo.Uint32(shdata[off+unsafe.Offsetof(sh.Link):]),
				Info:      bo.Uint32(shdata[off+unsafe.Offsetof(sh.Info):]),
				Addralign: uint64(bo.Uint32(shdata[off+unsafe.Offsetof(sh.Addralign):])),
				Entsize:   uint64(bo.Uint32(shdata[off+unsafe.Offsetof(sh.Entsize):])),
			}
		case ELFCLASS64:
			var sh Section64
			names = append(names, bo.Uint32(shdata[off+unsafe.Offsetof(sh.Name):]))
			s.SectionHeader = SectionHeader{
				Type:      SectionType(bo.Uint32(shdata[off+unsafe.Offsetof(sh.Type):])),
				Flags:     SectionFlag(bo.Uint64(shdata[off+unsafe.Offsetof(sh.Flags):])),
				Offset:    bo.Uint64(shdata[off+unsafe.Offsetof(sh.Off):]),
				FileSize:  bo.Uint64(shdata[off+unsafe.Offsetof(sh.Size):]),
				Addr:      bo.Uint64(shdata[off+unsafe.Offsetof(sh.Addr):]),
				Link:      bo.Uint32(shdata[off+unsafe.Offsetof(sh.Link):]),
				Info:      bo.Uint32(shdata[off+unsafe.Offsetof(sh.Info):]),
				Addralign: bo.Uint64(shdata[off+unsafe.Offsetof(sh.Addralign):]),
				Entsize:   bo.Uint64(shdata[off+unsafe.Offsetof(sh.Entsize):]),
			}
		}
		if int64(s.Offset) < 0 {
			return nil, &FormatError{shoff + int64(off), "invalid section offset", int64(s.Offset)}
		}
		if int64(s.FileSize) < 0 {
			return nil, &FormatError{shoff + int64(off), "invalid section size", int64(s.FileSize)}
		}
		s.sr = io.NewSectionReader(r, int64(s.Offset), int64(s.FileSize))

		if s.Flags&SHF_COMPRESSED == 0 {
			s.ReaderAt = s.sr
			s.Size = s.FileSize
		} else {
			// Read the compression header.
			switch f.Class {
			case ELFCLASS32:
				var ch Chdr32
				chdata := make([]byte, unsafe.Sizeof(ch))
				if _, err := s.sr.ReadAt(chdata, 0); err != nil {
					return nil, err
				}
				s.compressionType = CompressionType(bo.Uint32(chdata[unsafe.Offsetof(ch.Type):]))
				s.Size = uint64(bo.Uint32(chdata[unsafe.Offsetof(ch.Size):]))
				s.Addralign = uint64(bo.Uint32(chdata[unsafe.Offsetof(ch.Addralign):]))
				s.compressionOffset = int64(unsafe.Sizeof(ch))
			case ELFCLASS64:
				var ch Chdr64
				chdata := make([]byte, unsafe.Sizeof(ch))
				if _, err := s.sr.ReadAt(chdata, 0); err != nil {
					return nil, err
				}
				s.compressionType = CompressionType(bo.Uint32(chdata[unsafe.Offsetof(ch.Type):]))
				s.Size = bo.Uint64(chdata[unsafe.Offsetof(ch.Size):])
				s.Addralign = bo.Uint64(chdata[unsafe.Offsetof(ch.Addralign):])
				s.compressionOffset = int64(unsafe.Sizeof(ch))
			}
		}

		f.Sections = append(f.Sections, s)
	}

	if len(f.Sections) == 0 {
		return f, nil
	}

	// Load section header string table.
	if shstrndx == 0 {
		// If the file has no section name string table,
		// shstrndx holds the value SHN_UNDEF (0).
		return f, nil
	}
	shstr := f.Sections[shstrndx]
	if shstr.Type != SHT_STRTAB {
		return nil, &FormatError{shoff + int64(shstrndx*shentsize), "invalid ELF section name string table type", shstr.Type}
	}
	shstrtab, err := shstr.Data()
	if err != nil {
		return nil, err
	}
	for i, s := range f.Sections {
		var ok bool
		s.Name, ok = getString(shstrtab, int(names[i]))
		if !ok {
			return nil, &FormatError{shoff + int64(i*shentsize), "bad section name index", names[i]}
		}
	}

	return f, nil
}

// getSymbols returns a slice of Symbols from parsing the symbol table
// with the given type, along with the associated string table.
func (f *File) getSymbols(typ SectionType) ([]Symbol, []byte, error) {
	switch f.Class {
	case ELFCLASS64:
		return f.getSymbols64(typ)

	case ELFCLASS32:
		return f.getSymbols32(typ)
	}

	return nil, nil, errors.New("not implemented")
}

// ErrNoSymbols is returned by [File.Symbols] and [File.DynamicSymbols]
// if there is no such section in the File.
var ErrNoSymbols = errors.New("no symbol section")

func (f *File) getSymbols32(typ SectionType) ([]Symbol, []byte, error) {
	symtabSection := f.SectionByType(typ)
	if symtabSection == nil {
		return nil, nil, ErrNoSymbols
	}

	data, err := symtabSection.Data()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot load symbol section: %w", err)
	}
	if len(data) == 0 {
		return nil, nil, ErrNoSymbols
	}
	if len(data)%Sym32Size != 0 {
		return nil, nil, errors.New("length of symbol section is not a multiple of SymSize")
	}

	strdata, err := f.stringTable(symtabSection.Link)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot load string table section: %w", err)
	}

	// The first entry is all zeros.
	data = data[Sym32Size:]

	symbols := make([]Symbol, len(data)/Sym32Size)

	i := 0
	var sym Sym32
	for len(data) > 0 {
		sym.Name = f.ByteOrder.Uint32(data[0:4])
		sym.Value = f.ByteOrder.Uint32(data[4:8])
		sym.Size = f.ByteOrder.Uint32(data[8:12])
		sym.Info = data[12]
		sym.Other = data[13]
		sym.Shndx = f.ByteOrder.Uint16(data[14:16])
		str, _ := getString(strdata, int(sym.Name))
		symbols[i].Name = str
		symbols[i].Info = sym.Info
		symbols[i].Other = sym.Other
		symbols[i].Section = SectionIndex(sym.Shndx)
		symbols[i].Value = uint64(sym.Value)
		symbols[i].Size = uint64(sym.Size)
		i++
		data = data[Sym32Size:]
	}

	return symbols, strdata, nil
}

func (f *File) getSymbols64(typ SectionType) ([]Symbol, []byte, error) {
	symtabSection := f.SectionByType(typ)
	if symtabSection == nil {
		return nil, nil, ErrNoSymbols
	}

	data, err := symtabSection.Data()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot load symbol section: %w", err)
	}
	if len(data) == 0 {
		return nil, nil, ErrNoSymbols
	}
	if len(data)%Sym64Size != 0 {
		return nil, nil, errors.New("length of symbol section is not a multiple of Sym64Size")
	}

	strdata, err := f.stringTable(symtabSection.Link)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot load string table section: %w", err)
	}

	// The first entry is all zeros.
	data = data[Sym64Size:]

	symbols := make([]Symbol, len(data)/Sym64Size)

	i := 0
	var sym Sym64
	for len(data) > 0 {
		sym.Name = f.ByteOrder.Uint32(data[0:4])
		sym.Info = data[4]
		sym.Other = data[5]
		sym.Shndx = f.ByteOrder.Uint16(data[6:8])
		sym.Value = f.ByteOrder.Uint64(data[8:16])
		sym.Size = f.ByteOrder.Uint64(data[16:24])
		str, _ := getString(strdata, int(sym.Name))
		symbols[i].Name = str
		symbols[i].Info = sym.Info
		symbols[i].Other = sym.Other
		symbols[i].Section = SectionIndex(sym.Shndx)
		symbols[i].Value = sym.Value
		symbols[i].Size = sym.Size
		i++
		data = data[Sym64Size:]
	}

	return symbols, strdata, nil
}

// getString extracts a string from an ELF string table.
func getString(section []byte, start int) (string, bool) {
	if start < 0 || start >= len(section) {
		return "", false
	}

	for end := start; end < len(section); end++ {
		if section[end] == 0 {
			return string(section[start:end]), true
		}
	}
	return "", false
}

// Section returns a section with the given name, or nil if no such
// section exists.
func (f *File) Section(name string) *Section {
	for _, s := range f.Sections {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// applyRelocations applies relocations to dst. rels is a relocations section
// in REL or RELA format.
func (f *File) applyRelocations(dst []byte, rels []byte) error {
	switch {
	case f.Class == ELFCLASS64 && f.Machine == EM_X86_64:
		return f.applyRelocationsAMD64(dst, rels)
	case f.Class == ELFCLASS32 && f.Machine == EM_386:
		return f.applyRelocations386(dst, rels)
	case f.Class == ELFCLASS32 && f.Machine == EM_ARM:
		return f.applyRelocationsARM(dst, rels)
	case f.Class == ELFCLASS64 && f.Machine == EM_AARCH64:
		return f.applyRelocationsARM64(dst, rels)
	case f.Class == ELFCLASS32 && f.Machine == EM_PPC:
		return f.applyRelocationsPPC(dst, rels)
	case f.Class == ELFCLASS64 && f.Machine == EM_PPC64:
		return f.applyRelocationsPPC64(dst, rels)
	case f.Class == ELFCLASS32 && f.Machine == EM_MIPS:
		return f.applyRelocationsMIPS(dst, rels)
	case f.Class == ELFCLASS64 && f.Machine == EM_MIPS:
		return f.applyRelocationsMIPS64(dst, rels)
	case f.Class == ELFCLASS64 && f.Machine == EM_LOONGARCH:
		return f.applyRelocationsLOONG64(dst, rels)
	case f.Class == ELFCLASS64 && f.Machine == EM_RISCV:
		return f.applyRelocationsRISCV64(dst, rels)
	case f.Class == ELFCLASS64 && f.Machine == EM_S390:
		return f.applyRelocationss390x(dst, rels)
	case f.Class == ELFCLASS64 && f.Machine == EM_SPARCV9:
		return f.applyRelocationsSPARC64(dst, rels)
	default:
		return errors.New("applyRelocations: not implemented")
	}
}

// canApplyRelocation reports whether we should try to apply a
// relocation to a DWARF data section, given a pointer to the symbol
// targeted by the relocation.
// Most relocations in DWARF data tend to be section-relative, but
// some target non-section symbols (for example, low_PC attrs on
// subprogram or compilation unit DIEs that target function symbols).
func canApplyRelocation(sym *Symbol) bool {
	return sym.Section != SHN_UNDEF && sym.Section < SHN_LORESERVE
}

func (f *File) applyRelocationsAMD64(dst []byte, rels []byte) error {
	// 24 is the size of Rela64.
	if len(rels)%24 != 0 {
		return errors.New("length of relocation section is not a multiple of 24")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rela Rela64

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rela)
		symNo := rela.Info >> 32
		t := R_X86_64(rela.Info & 0xffff)

		if symNo == 0 || symNo > uint64(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]
		if !canApplyRelocation(sym) {
			continue
		}

		// There are relocations, so this must be a normal
		// object file.  The code below handles only basic relocations
		// of the form S + A (symbol plus addend).

		switch t {
		case R_X86_64_64:
			putUint(f.ByteOrder, dst, rela.Off, 8, sym.Value, rela.Addend, false)
		case R_X86_64_32:
			putUint(f.ByteOrder, dst, rela.Off, 4, sym.Value, rela.Addend, false)
		}
	}

	return nil
}

func (f *File) applyRelocations386(dst []byte, rels []byte) error {
	// 8 is the size of Rel32.
	if len(rels)%8 != 0 {
		return errors.New("length of relocation section is not a multiple of 8")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rel Rel32

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rel)
		symNo := rel.Info >> 8
		t := R_386(rel.Info & 0xff)

		if symNo == 0 || symNo > uint32(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]

		if t == R_386_32 {
			putUint(f.ByteOrder, dst, uint64(rel.Off), 4, sym.Value, 0, true)
		}
	}

	return nil
}

func (f *File) applyRelocationsARM(dst []byte, rels []byte) error {
	// 8 is the size of Rel32.
	if len(rels)%8 != 0 {
		return errors.New("length of relocation section is not a multiple of 8")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rel Rel32

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rel)
		symNo := rel.Info >> 8
		t := R_ARM(rel.Info & 0xff)

		if symNo == 0 || symNo > uint32(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]

		switch t {
		case R_ARM_ABS32:
			putUint(f.ByteOrder, dst, uint64(rel.Off), 4, sym.Value, 0, true)
		}
	}

	return nil
}

func (f *File) applyRelocationsARM64(dst []byte, rels []byte) error {
	// 24 is the size of Rela64.
	if len(rels)%24 != 0 {
		return errors.New("length of relocation section is not a multiple of 24")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rela Rela64

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rela)
		symNo := rela.Info >> 32
		t := R_AARCH64(rela.Info & 0xffff)

		if symNo == 0 || symNo > uint64(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]
		if !canApplyRelocation(sym) {
			continue
		}

		// There are relocations, so this must be a normal
		// object file.  The code below handles only basic relocations
		// of the form S + A (symbol plus addend).

		switch t {
		case R_AARCH64_ABS64:
			putUint(f.ByteOrder, dst, rela.Off, 8, sym.Value, rela.Addend, false)
		case R_AARCH64_ABS32:
			putUint(f.ByteOrder, dst, rela.Off, 4, sym.Value, rela.Addend, false)
		}
	}

	return nil
}

func (f *File) applyRelocationsPPC(dst []byte, rels []byte) error {
	// 12 is the size of Rela32.
	if len(rels)%12 != 0 {
		return errors.New("length of relocation section is not a multiple of 12")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rela Rela32

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rela)
		symNo := rela.Info >> 8
		t := R_PPC(rela.Info & 0xff)

		if symNo == 0 || symNo > uint32(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]
		if !canApplyRelocation(sym) {
			continue
		}

		switch t {
		case R_PPC_ADDR32:
			putUint(f.ByteOrder, dst, uint64(rela.Off), 4, sym.Value, 0, false)
		}
	}

	return nil
}

func (f *File) applyRelocationsPPC64(dst []byte, rels []byte) error {
	// 24 is the size of Rela64.
	if len(rels)%24 != 0 {
		return errors.New("length of relocation section is not a multiple of 24")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rela Rela64

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rela)
		symNo := rela.Info >> 32
		t := R_PPC64(rela.Info & 0xffff)

		if symNo == 0 || symNo > uint64(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]
		if !canApplyRelocation(sym) {
			continue
		}

		switch t {
		case R_PPC64_ADDR64:
			putUint(f.ByteOrder, dst, rela.Off, 8, sym.Value, rela.Addend, false)
		case R_PPC64_ADDR32:
			putUint(f.ByteOrder, dst, rela.Off, 4, sym.Value, rela.Addend, false)
		}
	}

	return nil
}

func (f *File) applyRelocationsMIPS(dst []byte, rels []byte) error {
	// 8 is the size of Rel32.
	if len(rels)%8 != 0 {
		return errors.New("length of relocation section is not a multiple of 8")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rel Rel32

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rel)
		symNo := rel.Info >> 8
		t := R_MIPS(rel.Info & 0xff)

		if symNo == 0 || symNo > uint32(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]

		switch t {
		case R_MIPS_32:
			putUint(f.ByteOrder, dst, uint64(rel.Off), 4, sym.Value, 0, true)
		}
	}

	return nil
}

func (f *File) applyRelocationsMIPS64(dst []byte, rels []byte) error {
	// 24 is the size of Rela64.
	if len(rels)%24 != 0 {
		return errors.New("length of relocation section is not a multiple of 24")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rela Rela64

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rela)
		var symNo uint64
		var t R_MIPS
		if f.ByteOrder == binary.BigEndian {
			symNo = rela.Info >> 32
			t = R_MIPS(rela.Info & 0xff)
		} else {
			symNo = rela.Info & 0xffffffff
			t = R_MIPS(rela.Info >> 56)
		}

		if symNo == 0 || symNo > uint64(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]
		if !canApplyRelocation(sym) {
			continue
		}

		switch t {
		case R_MIPS_64:
			putUint(f.ByteOrder, dst, rela.Off, 8, sym.Value, rela.Addend, false)
		case R_MIPS_32:
			putUint(f.ByteOrder, dst, rela.Off, 4, sym.Value, rela.Addend, false)
		}
	}

	return nil
}

func (f *File) applyRelocationsLOONG64(dst []byte, rels []byte) error {
	// 24 is the size of Rela64.
	if len(rels)%24 != 0 {
		return errors.New("length of relocation section is not a multiple of 24")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rela Rela64

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rela)
		var symNo uint64
		var t R_LARCH
		symNo = rela.Info >> 32
		t = R_LARCH(rela.Info & 0xffff)

		if symNo == 0 || symNo > uint64(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]
		if !canApplyRelocation(sym) {
			continue
		}

		switch t {
		case R_LARCH_64:
			putUint(f.ByteOrder, dst, rela.Off, 8, sym.Value, rela.Addend, false)
		case R_LARCH_32:
			putUint(f.ByteOrder, dst, rela.Off, 4, sym.Value, rela.Addend, false)
		}
	}

	return nil
}

func (f *File) applyRelocationsRISCV64(dst []byte, rels []byte) error {
	// 24 is the size of Rela64.
	if len(rels)%24 != 0 {
		return errors.New("length of relocation section is not a multiple of 24")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rela Rela64

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rela)
		symNo := rela.Info >> 32
		t := R_RISCV(rela.Info & 0xffff)

		if symNo == 0 || symNo > uint64(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]
		if !canApplyRelocation(sym) {
			continue
		}

		switch t {
		case R_RISCV_64:
			putUint(f.ByteOrder, dst, rela.Off, 8, sym.Value, rela.Addend, false)
		case R_RISCV_32:
			putUint(f.ByteOrder, dst, rela.Off, 4, sym.Value, rela.Addend, false)
		}
	}

	return nil
}

func (f *File) applyRelocationss390x(dst []byte, rels []byte) error {
	// 24 is the size of Rela64.
	if len(rels)%24 != 0 {
		return errors.New("length of relocation section is not a multiple of 24")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rela Rela64

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rela)
		symNo := rela.Info >> 32
		t := R_390(rela.Info & 0xffff)

		if symNo == 0 || symNo > uint64(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]
		if !canApplyRelocation(sym) {
			continue
		}

		switch t {
		case R_390_64:
			putUint(f.ByteOrder, dst, rela.Off, 8, sym.Value, rela.Addend, false)
		case R_390_32:
			putUint(f.ByteOrder, dst, rela.Off, 4, sym.Value, rela.Addend, false)
		}
	}

	return nil
}

func (f *File) applyRelocationsSPARC64(dst []byte, rels []byte) error {
	// 24 is the size of Rela64.
	if len(rels)%24 != 0 {
		return errors.New("length of relocation section is not a multiple of 24")
	}

	symbols, _, err := f.getSymbols(SHT_SYMTAB)
	if err != nil {
		return err
	}

	b := bytes.NewReader(rels)
	var rela Rela64

	for b.Len() > 0 {
		binary.Read(b, f.ByteOrder, &rela)
		symNo := rela.Info >> 32
		t := R_SPARC(rela.Info & 0xff)

		if symNo == 0 || symNo > uint64(len(symbols)) {
			continue
		}
		sym := &symbols[symNo-1]
		if !canApplyRelocation(sym) {
			continue
		}

		switch t {
		case R_SPARC_64, R_SPARC_UA64:
			putUint(f.ByteOrder, dst, rela.Off, 8, sym.Value, rela.Addend, false)

		case R_SPARC_32, R_SPARC_UA32:
			putUint(f.ByteOrder, dst, rela.Off, 4, sym.Value, rela.Addend, false)
		}
	}

	return nil
}

func (f *File) DWARF() (*dwarf.Data, error) {
	dwarfSuffix := func(s *Section) string {
		switch {
		case strings.HasPrefix(s.Name, ".debug_"):
			return s.Name[7:]
		case strings.HasPrefix(s.Name, ".zdebug_"):
			return s.Name[8:]
		default:
			return ""
		}

	}
	// sectionData gets the data for s, checks its size, and
	// applies any applicable relations.
	sectionData := func(i int, s *Section) ([]byte, error) {
		b, err := s.Data()
		if err != nil && uint64(len(b)) < s.Size {
			return nil, err
		}

		if f.Type == ET_EXEC {
			// Do not apply relocations to DWARF sections for ET_EXEC binaries.
			// Relocations should already be applied, and .rela sections may
			// contain incorrect data.
			return b, nil
		}

		for _, r := range f.Sections {
			if r.Type != SHT_RELA && r.Type != SHT_REL {
				continue
			}
			if int(r.Info) != i {
				continue
			}
			rd, err := r.Data()
			if err != nil {
				return nil, err
			}
			err = f.applyRelocations(b, rd)
			if err != nil {
				return nil, err
			}
		}
		return b, nil
	}

	// There are many DWARF sections, but these are the ones
	// the debug/dwarf package started with.
	var dat = map[string][]byte{"abbrev": nil, "info": nil, "str": nil, "line": nil, "ranges": nil}
	for i, s := range f.Sections {
		suffix := dwarfSuffix(s)
		if suffix == "" {
			continue
		}
		if _, ok := dat[suffix]; !ok {
			continue
		}
		b, err := sectionData(i, s)
		if err != nil {
			return nil, err
		}
		dat[suffix] = b
	}

	d, err := dwarf.New(dat["abbrev"], nil, nil, dat["info"], dat["line"], nil, dat["ranges"], dat["str"])
	if err != nil {
		return nil, err
	}

	// Look for DWARF4 .debug_types sections and DWARF5 sections.
	for i, s := range f.Sections {
		suffix := dwarfSuffix(s)
		if suffix == "" {
			continue
		}
		if _, ok := dat[suffix]; ok {
			// Already handled.
			continue
		}

		b, err := sectionData(i, s)
		if err != nil {
			return nil, err
		}

		if suffix == "types" {
			if err := d.AddTypes(fmt.Sprintf("types-%d", i), b); err != nil {
				return nil, err
			}
		} else {
			if err := d.AddSection(".debug_"+suffix, b); err != nil {
				return nil, err
			}
		}
	}

	return d, nil
}

// Symbols returns the symbol table for f. The symbols will be listed in the order
// they appear in f.
//
// For compatibility with Go 1.0, Symbols omits the null symbol at index 0.
// After retrieving the symbols as symtab, an externally supplied index x
// corresponds to symtab[x-1], not symtab[x].
func (f *File) Symbols() ([]Symbol, error) {
	sym, _, err := f.getSymbols(SHT_SYMTAB)
	return sym, err
}

// DynamicSymbols returns the dynamic symbol table for f. The symbols
// will be listed in the order they appear in f.
//
// If f has a symbol version table, the returned [File.Symbols] will have
// initialized Version and Library fields.
//
// For compatibility with [File.Symbols], [File.DynamicSymbols] omits the null symbol at index 0.
// After retrieving the symbols as symtab, an externally supplied index x
// corresponds to symtab[x-1], not symtab[x].
func (f *File) DynamicSymbols() ([]Symbol, error) {
	sym, str, err := f.getSymbols(SHT_DYNSYM)
	if err != nil {
		return nil, err
	}
	hasVersions, err := f.gnuVersionInit(str)
	if err != nil {
		return nil, err
	}
	if hasVersions {
		for i := range sym {
			sym[i].HasVersion, sym[i].VersionIndex, sym[i].Version, sym[i].Library = f.gnuVersion(i)
		}
	}
	return sym, nil
}

type ImportedSymbol struct {
	Name    string
	Version string
	Library string
}

// ImportedSymbols returns the names of all symbols
// referred to by the binary f that are expected to be
// satisfied by other libraries at dynamic load time.
// It does not return weak symbols.
func (f *File) ImportedSymbols() ([]ImportedSymbol, error) {
	sym, str, err := f.getSymbols(SHT_DYNSYM)
	if err != nil {
		return nil, err
	}
	if _, err := f.gnuVersionInit(str); err != nil {
		return nil, err
	}
	var all []ImportedSymbol
	for i, s := range sym {
		if ST_BIND(s.Info) == STB_GLOBAL && s.Section == SHN_UNDEF {
			all = append(all, ImportedSymbol{Name: s.Name})
			sym := &all[len(all)-1]
			_, _, sym.Version, sym.Library = f.gnuVersion(i)
		}
	}
	return all, nil
}

// VersionIndex is the type of a [Symbol] version index.
type VersionIndex uint16

// IsHidden reports whether the symbol is hidden within the version.
// This means that the symbol can only be seen by specifying the exact version.
func (vi VersionIndex) IsHidden() bool {
	return vi&0x8000 != 0
}

// Index returns the version index.
// If this is the value 0, it means that the symbol is local,
// and is not visible externally.
// If this is the value 1, it means that the symbol is in the base version,
// and has no specific version; it may or may not match a
// [DynamicVersion.Index] in the slice returned by [File.DynamicVersions].
// Other values will match either [DynamicVersion.Index]
// in the slice returned by [File.DynamicVersions],
// or [DynamicVersionDep.Index] in the Needs field
// of the elements of the slice returned by [File.DynamicVersionNeeds].
// In general, a defined symbol will have an index referring
// to DynamicVersions, and an undefined symbol will have an index
// referring to some version in DynamicVersionNeeds.
func (vi VersionIndex) Index() uint16 {
	return uint16(vi & 0x7fff)
}

// DynamicVersion is a version defined by a dynamic object.
// This describes entries in the ELF SHT_GNU_verdef section.
// We assume that the vd_version field is 1.
// Note that the name of the version appears here;
// it is not in the first Deps entry as it is in the ELF file.
type DynamicVersion struct {
	Name  string // Name of version defined by this index.
	Index uint16 // Version index.
	Flags DynamicVersionFlag
	Deps  []string // Names of versions that this version depends upon.
}

// DynamicVersionNeed describes a shared library needed by a dynamic object,
// with a list of the versions needed from that shared library.
// This describes entries in the ELF SHT_GNU_verneed section.
// We assume that the vn_version field is 1.
type DynamicVersionNeed struct {
	Name  string              // Shared library name.
	Needs []DynamicVersionDep // Dependencies.
}

// DynamicVersionDep is a version needed from some shared library.
type DynamicVersionDep struct {
	Flags DynamicVersionFlag
	Index uint16 // Version index.
	Dep   string // Name of required version.
}

// dynamicVersions returns version information for a dynamic object.
func (f *File) dynamicVersions(str []byte) error {
	if f.dynVers != nil {
		// Already initialized.
		return nil
	}

	// Accumulate verdef information.
	vd := f.SectionByType(SHT_GNU_VERDEF)
	if vd == nil {
		return nil
	}
	d, _ := vd.Data()

	var dynVers []DynamicVersion
	i := 0
	for {
		if i+20 > len(d) {
			break
		}
		version := f.ByteOrder.Uint16(d[i : i+2])
		if version != 1 {
			return &FormatError{int64(vd.Offset + uint64(i)), "unexpected dynamic version", version}
		}
		flags := DynamicVersionFlag(f.ByteOrder.Uint16(d[i+2 : i+4]))
		ndx := f.ByteOrder.Uint16(d[i+4 : i+6])
		cnt := f.ByteOrder.Uint16(d[i+6 : i+8])
		aux := f.ByteOrder.Uint32(d[i+12 : i+16])
		next := f.ByteOrder.Uint32(d[i+16 : i+20])

		if cnt == 0 {
			return &FormatError{int64(vd.Offset + uint64(i)), "dynamic version has no name", nil}
		}

		var name string
		var depName string
		var deps []string
		j := i + int(aux)
		for c := 0; c < int(cnt); c++ {
			if j+8 > len(d) {
				break
			}
			vname := f.ByteOrder.Uint32(d[j : j+4])
			vnext := f.ByteOrder.Uint32(d[j+4 : j+8])
			depName, _ = getString(str, int(vname))

			if c == 0 {
				name = depName
			} else {
				deps = append(deps, depName)
			}

			if vnext == 0 {
				break
			}
			j += int(vnext)
		}

		dynVers = append(dynVers, DynamicVersion{
			Name:  name,
			Index: ndx,
			Flags: flags,
			Deps:  deps,
		})

		if next == 0 {
			break
		}
		i += int(next)
	}

	f.dynVers = dynVers

	return nil
}

// DynamicVersions returns version information for a dynamic object.
func (f *File) DynamicVersions() ([]DynamicVersion, error) {
	if f.dynVers == nil {
		_, str, err := f.getSymbols(SHT_DYNSYM)
		if err != nil {
			return nil, err
		}
		hasVersions, err := f.gnuVersionInit(str)
		if err != nil {
			return nil, err
		}
		if !hasVersions {
			return nil, errors.New("DynamicVersions: missing version table")
		}
	}

	return f.dynVers, nil
}

// dynamicVersionNeeds returns version dependencies for a dynamic object.
func (f *File) dynamicVersionNeeds(str []byte) error {
	if f.dynVerNeeds != nil {
		// Already initialized.
		return nil
	}

	// Accumulate verneed information.
	vn := f.SectionByType(SHT_GNU_VERNEED)
	if vn == nil {
		return nil
	}
	d, _ := vn.Data()

	var dynVerNeeds []DynamicVersionNeed
	i := 0
	for {
		if i+16 > len(d) {
			break
		}
		vers := f.ByteOrder.Uint16(d[i : i+2])
		if vers != 1 {
			return &FormatError{int64(vn.Offset + uint64(i)), "unexpected dynamic need version", vers}
		}
		cnt := f.ByteOrder.Uint16(d[i+2 : i+4])
		fileoff := f.ByteOrder.Uint32(d[i+4 : i+8])
		aux := f.ByteOrder.Uint32(d[i+8 : i+12])
		next := f.ByteOrder.Uint32(d[i+12 : i+16])
		file, _ := getString(str, int(fileoff))

		var deps []DynamicVersionDep
		j := i + int(aux)
		for c := 0; c < int(cnt); c++ {
			if j+16 > len(d) {
				break
			}
			flags := DynamicVersionFlag(f.ByteOrder.Uint16(d[j+4 : j+6]))
			index := f.ByteOrder.Uint16(d[j+6 : j+8])
			nameoff := f.ByteOrder.Uint32(d[j+8 : j+12])
			next := f.ByteOrder.Uint32(d[j+12 : j+16])
			depName, _ := getString(str, int(nameoff))

			deps = append(deps, DynamicVersionDep{
				Flags: flags,
				Index: index,
				Dep:   depName,
			})

			if next == 0 {
				break
			}
			j += int(next)
		}

		dynVerNeeds = append(dynVerNeeds, DynamicVersionNeed{
			Name:  file,
			Needs: deps,
		})

		if next == 0 {
			break
		}
		i += int(next)
	}

	f.dynVerNeeds = dynVerNeeds

	return nil
}

// DynamicVersionNeeds returns version dependencies for a dynamic object.
func (f *File) DynamicVersionNeeds() ([]DynamicVersionNeed, error) {
	if f.dynVerNeeds == nil {
		_, str, err := f.getSymbols(SHT_DYNSYM)
		if err != nil {
			return nil, err
		}
		hasVersions, err := f.gnuVersionInit(str)
		if err != nil {
			return nil, err
		}
		if !hasVersions {
			return nil, errors.New("DynamicVersionNeeds: missing version table")
		}
	}

	return f.dynVerNeeds, nil
}

// gnuVersionInit parses the GNU version tables
// for use by calls to gnuVersion.
// It reports whether any version tables were found.
func (f *File) gnuVersionInit(str []byte) (bool, error) {
	// Versym parallels symbol table, indexing into verneed.
	vs := f.SectionByType(SHT_GNU_VERSYM)
	if vs == nil {
		return false, nil
	}
	d, _ := vs.Data()

	f.gnuVersym = d
	if err := f.dynamicVersions(str); err != nil {
		return false, err
	}
	if err := f.dynamicVersionNeeds(str); err != nil {
		return false, err
	}
	return true, nil
}

// gnuVersion adds Library and Version information to sym,
// which came from offset i of the symbol table.
func (f *File) gnuVersion(i int) (hasVersion bool, versionIndex VersionIndex, version string, library string) {
	// Each entry is two bytes; skip undef entry at beginning.
	i = (i + 1) * 2
	if i >= len(f.gnuVersym) {
		return false, 0, "", ""
	}
	s := f.gnuVersym[i:]
	if len(s) < 2 {
		return false, 0, "", ""
	}
	vi := VersionIndex(f.ByteOrder.Uint16(s))
	ndx := vi.Index()

	if ndx == 0 || ndx == 1 {
		return true, vi, "", ""
	}

	for _, v := range f.dynVerNeeds {
		for _, n := range v.Needs {
			if ndx == n.Index {
				return true, vi, n.Dep, v.Name
			}
		}
	}

	for _, v := range f.dynVers {
		if ndx == v.Index {
			return true, vi, v.Name, ""
		}
	}

	return false, 0, "", ""
}

// ImportedLibraries returns the names of all libraries
// referred to by the binary f that are expected to be
// linked with the binary at dynamic link time.
func (f *File) ImportedLibraries() ([]string, error) {
	return f.DynString(DT_NEEDED)
}

// DynString returns the strings listed for the given tag in the file's dynamic
// section.
//
// The tag must be one that takes string values: [DT_NEEDED], [DT_SONAME], [DT_RPATH], or
// [DT_RUNPATH].
func (f *File) DynString(tag DynTag) ([]string, error) {
	switch tag {
	case DT_NEEDED, DT_SONAME, DT_RPATH, DT_RUNPATH:
	default:
		return nil, fmt.Errorf("non-string-valued tag %v", tag)
	}
	ds := f.SectionByType(SHT_DYNAMIC)
	if ds == nil {
		// not dynamic, so no libraries
		return nil, nil
	}
	d, err := ds.Data()
	if err != nil {
		return nil, err
	}

	dynSize := 8
	if f.Class == ELFCLASS64 {
		dynSize = 16
	}
	if len(d)%dynSize != 0 {
		return nil, errors.New("length of dynamic section is not a multiple of dynamic entry size")
	}

	str, err := f.stringTable(ds.Link)
	if err != nil {
		return nil, err
	}
	var all []string
	for len(d) > 0 {
		var t DynTag
		var v uint64
		switch f.Class {
		case ELFCLASS32:
			t = DynTag(f.ByteOrder.Uint32(d[0:4]))
			v = uint64(f.ByteOrder.Uint32(d[4:8]))
			d = d[8:]
		case ELFCLASS64:
			t = DynTag(f.ByteOrder.Uint64(d[0:8]))
			v = f.ByteOrder.Uint64(d[8:16])
			d = d[16:]
		}
		if t == tag {
			s, ok := getString(str, int(v))
			if ok {
				all = append(all, s)
			}
		}
	}
	return all, nil
}

// DynValue returns the values listed for the given tag in the file's dynamic
// section.
func (f *File) DynValue(tag DynTag) ([]uint64, error) {
	ds := f.SectionByType(SHT_DYNAMIC)
	if ds == nil {
		return nil, nil
	}
	d, err := ds.Data()
	if err != nil {
		return nil, err
	}

	dynSize := 8
	if f.Class == ELFCLASS64 {
		dynSize = 16
	}
	if len(d)%dynSize != 0 {
		return nil, errors.New("length of dynamic section is not a multiple of dynamic entry size")
	}

	// Parse the .dynamic section as a string of bytes.
	var vals []uint64
	for len(d) > 0 {
		var t DynTag
		var v uint64
		switch f.Class {
		case ELFCLASS32:
			t = DynTag(f.ByteOrder.Uint32(d[0:4]))
			v = uint64(f.ByteOrder.Uint32(d[4:8]))
			d = d[8:]
		case ELFCLASS64:
			t = DynTag(f.ByteOrder.Uint64(d[0:8]))
			v = f.ByteOrder.Uint64(d[8:16])
			d = d[16:]
		}
		if t == tag {
			vals = append(vals, v)
		}
	}
	return vals, nil
}

type nobitsSectionReader struct{}

func (*nobitsSectionReader) ReadAt(p []byte, off int64) (n int, err error) {
	return 0, errors.New("unexpected read from SHT_NOBITS section")
}

// putUint writes a relocation to slice
// at offset start of length length (4 or 8 bytes),
// adding sym+addend to the existing value if readUint is true,
// or just writing sym+addend if readUint is false.
// If the write would extend beyond the end of slice, putUint does nothing.
// If the addend is negative, putUint does nothing.
// If the addition would overflow, putUint does nothing.
func putUint(byteOrder binary.ByteOrder, slice []byte, start, length, sym uint64, addend int64, readUint bool) {
	if start+length > uint64(len(slice)) || math.MaxUint64-start < length {
		return
	}
	if addend < 0 {
		return
	}

	s := slice[start : start+length]

	switch length {
	case 4:
		ae := uint32(addend)
		if readUint {
			ae += byteOrder.Uint32(s)
		}
		byteOrder.PutUint32(s, uint32(sym)+ae)
	case 8:
		ae := uint64(addend)
		if readUint {
			ae += byteOrder.Uint64(s)
		}
		byteOrder.PutUint64(s, sym+ae)
	default:
		panic("can't happen")
	}
}

```

// === FILE: references!/go/src/debug/elf/reader.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package elf

import (
	"io"
	"os"
)

// errorReader returns error from all operations.
type errorReader struct {
	error
}

func (r errorReader) Read(p []byte) (n int, err error) {
	return 0, r.error
}

func (r errorReader) ReadAt(p []byte, off int64) (n int, err error) {
	return 0, r.error
}

func (r errorReader) Seek(offset int64, whence int) (int64, error) {
	return 0, r.error
}

func (r errorReader) Close() error {
	return r.error
}

// readSeekerFromReader converts an io.Reader into an io.ReadSeeker.
// In general Seek may not be efficient, but it is optimized for
// common cases such as seeking to the end to find the length of the
// data.
type readSeekerFromReader struct {
	reset  func() (io.Reader, error)
	r      io.Reader
	size   int64
	offset int64
}

func (r *readSeekerFromReader) start() {
	x, err := r.reset()
	if err != nil {
		r.r = errorReader{err}
	} else {
		r.r = x
	}
	r.offset = 0
}

func (r *readSeekerFromReader) Read(p []byte) (n int, err error) {
	if r.r == nil {
		r.start()
	}
	n, err = r.r.Read(p)
	r.offset += int64(n)
	return n, err
}

func (r *readSeekerFromReader) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = r.offset + offset
	case io.SeekEnd:
		newOffset = r.size + offset
	default:
		return 0, os.ErrInvalid
	}

	switch {
	case newOffset == r.offset:
		return newOffset, nil

	case newOffset < 0, newOffset > r.size:
		return 0, os.ErrInvalid

	case newOffset == 0:
		r.r = nil

	case newOffset == r.size:
		r.r = errorReader{io.EOF}

	default:
		if newOffset < r.offset {
			// Restart at the beginning.
			r.start()
		}
		// Read until we reach offset.
		var buf [512]byte
		for r.offset < newOffset {
			b := buf[:]
			if newOffset-r.offset < int64(len(buf)) {
				b = buf[:newOffset-r.offset]
			}
			if _, err := r.Read(b); err != nil {
				return 0, err
			}
		}
	}
	r.offset = newOffset
	return r.offset, nil
}

```

// === FILE: references!/go/src/debug/gosym/pclntab.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
 * Line tables
 */

package gosym

import (
	"bytes"
	"encoding/binary"
	"internal/abi"
	"sort"
	"sync"
)

// version of the pclntab
type version int

const (
	verUnknown version = iota
	ver11
	ver12
	ver116
	ver118
	ver120
)

// A LineTable is a data structure mapping program counters to line numbers.
//
// In Go 1.1 and earlier, each function (represented by a [Func]) had its own LineTable,
// and the line number corresponded to a numbering of all source lines in the
// program, across all files. That absolute line number would then have to be
// converted separately to a file name and line number within the file.
//
// In Go 1.2, the format of the data changed so that there is a single LineTable
// for the entire program, shared by all Funcs, and there are no absolute line
// numbers, just line numbers within specific files.
//
// For the most part, LineTable's methods should be treated as an internal
// detail of the package; callers should use the methods on [Table] instead.
type LineTable struct {
	Data []byte
	PC   uint64
	Line int

	// This mutex is used to keep parsing of pclntab synchronous.
	mu sync.Mutex

	// Contains the version of the pclntab section.
	version version

	// Go 1.2/1.16/1.18 state
	binary      binary.ByteOrder
	quantum     uint32
	ptrsize     uint32
	textStart   uint64 // address of runtime.text symbol (1.18+)
	funcnametab []byte
	cutab       []byte
	funcdata    []byte
	functab     []byte
	nfunctab    uint32
	filetab     []byte
	pctab       []byte // points to the pctables.
	nfiletab    uint32
	funcNames   map[uint32]string // cache the function names
	strings     map[uint32]string // interned substrings of Data, keyed by offset
	// fileMap varies depending on the version of the object file.
	// For ver12, it maps the name to the index in the file table.
	// For ver116, it maps the name to the offset in filetab.
	fileMap map[string]uint32
}

// NOTE(rsc): This is wrong for GOARCH=arm, which uses a quantum of 4,
// but we have no idea whether we're using arm or not. This only
// matters in the old (pre-Go 1.2) symbol table format, so it's not worth
// fixing.
const oldQuantum = 1

func (t *LineTable) parse(targetPC uint64, targetLine int) (b []byte, pc uint64, line int) {
	// The PC/line table can be thought of as a sequence of
	//  <pc update>* <line update>
	// batches. Each update batch results in a (pc, line) pair,
	// where line applies to every PC from pc up to but not
	// including the pc of the next pair.
	//
	// Here we process each update individually, which simplifies
	// the code, but makes the corner cases more confusing.
	b, pc, line = t.Data, t.PC, t.Line
	for pc <= targetPC && line != targetLine && len(b) > 0 {
		code := b[0]
		b = b[1:]
		switch {
		case code == 0:
			if len(b) < 4 {
				b = b[0:0]
				break
			}
			val := binary.BigEndian.Uint32(b)
			b = b[4:]
			line += int(val)
		case code <= 64:
			line += int(code)
		case code <= 128:
			line -= int(code - 64)
		default:
			pc += oldQuantum * uint64(code-128)
			continue
		}
		pc += oldQuantum
	}
	return b, pc, line
}

func (t *LineTable) slice(pc uint64) *LineTable {
	data, pc, line := t.parse(pc, -1)
	return &LineTable{Data: data, PC: pc, Line: line}
}

// PCToLine returns the line number for the given program counter.
//
// Deprecated: Use Table's PCToLine method instead.
func (t *LineTable) PCToLine(pc uint64) int {
	if t.isGo12() {
		return t.go12PCToLine(pc)
	}
	_, _, line := t.parse(pc, -1)
	return line
}

// LineToPC returns the program counter for the given line number,
// considering only program counters before maxpc.
//
// Deprecated: Use Table's LineToPC method instead.
func (t *LineTable) LineToPC(line int, maxpc uint64) uint64 {
	if t.isGo12() {
		return 0
	}
	_, pc, line1 := t.parse(maxpc, line)
	if line1 != line {
		return 0
	}
	// Subtract quantum from PC to account for post-line increment
	return pc - oldQuantum
}

// NewLineTable returns a new PC/line table
// corresponding to the encoded data.
// Text must be the start address of the
// corresponding text segment, with the exact
// value stored in the 'runtime.text' symbol.
// This value may differ from the start
// address of the text segment if
// binary was built with cgo enabled.
func NewLineTable(data []byte, text uint64) *LineTable {
	return &LineTable{Data: data, PC: text, Line: 0, funcNames: make(map[uint32]string), strings: make(map[uint32]string)}
}

// Go 1.2 symbol table format.
// See golang.org/s/go12symtab.
//
// A general note about the methods here: rather than try to avoid
// index out of bounds errors, we trust Go to detect them, and then
// we recover from the panics and treat them as indicative of a malformed
// or incomplete table.
//
// The methods called by symtab.go, which begin with "go12" prefixes,
// are expected to have that recovery logic.

// isGo12 reports whether this is a Go 1.2 (or later) symbol table.
func (t *LineTable) isGo12() bool {
	t.parsePclnTab()
	return t.version >= ver12
}

// uintptr returns the pointer-sized value encoded at b.
// The pointer size is dictated by the table being read.
func (t *LineTable) uintptr(b []byte) uint64 {
	if t.ptrsize == 4 {
		return uint64(t.binary.Uint32(b))
	}
	return t.binary.Uint64(b)
}

// parsePclnTab parses the pclntab, setting the version.
func (t *LineTable) parsePclnTab() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.version != verUnknown {
		return
	}

	// Note that during this function, setting the version is the last thing we do.
	// If we set the version too early, and parsing failed (likely as a panic on
	// slice lookups), we'd have a mistaken version.
	//
	// Error paths through this code will default the version to 1.1.
	t.version = ver11

	if !disableRecover {
		defer func() {
			// If we panic parsing, assume it's a Go 1.1 pclntab.
			recover()
		}()
	}

	// Check header: 4-byte magic, two zeros, pc quantum, pointer size.
	if len(t.Data) < 16 || t.Data[4] != 0 || t.Data[5] != 0 ||
		(t.Data[6] != 1 && t.Data[6] != 2 && t.Data[6] != 4) || // pc quantum
		(t.Data[7] != 4 && t.Data[7] != 8) { // pointer size
		return
	}

	var possibleVersion version

	// The magic numbers are chosen such that reading the value with
	// a different endianness does not result in the same value.
	// That lets us the magic number to determine the endianness.
	leMagic := abi.PCLnTabMagic(binary.LittleEndian.Uint32(t.Data))
	beMagic := abi.PCLnTabMagic(binary.BigEndian.Uint32(t.Data))

	switch {
	case leMagic == abi.Go12PCLnTabMagic:
		t.binary, possibleVersion = binary.LittleEndian, ver12
	case beMagic == abi.Go12PCLnTabMagic:
		t.binary, possibleVersion = binary.BigEndian, ver12
	case leMagic == abi.Go116PCLnTabMagic:
		t.binary, possibleVersion = binary.LittleEndian, ver116
	case beMagic == abi.Go116PCLnTabMagic:
		t.binary, possibleVersion = binary.BigEndian, ver116
	case leMagic == abi.Go118PCLnTabMagic:
		t.binary, possibleVersion = binary.LittleEndian, ver118
	case beMagic == abi.Go118PCLnTabMagic:
		t.binary, possibleVersion = binary.BigEndian, ver118
	case leMagic == abi.Go120PCLnTabMagic:
		t.binary, possibleVersion = binary.LittleEndian, ver120
	case beMagic == abi.Go120PCLnTabMagic:
		t.binary, possibleVersion = binary.BigEndian, ver120
	default:
		return
	}
	t.version = possibleVersion

	// quantum and ptrSize are the same between 1.2, 1.16, and 1.18
	t.quantum = uint32(t.Data[6])
	t.ptrsize = uint32(t.Data[7])

	offset := func(word uint32) uint64 {
		return t.uintptr(t.Data[8+word*t.ptrsize:])
	}
	data := func(word uint32) []byte {
		return t.Data[offset(word):]
	}

	switch possibleVersion {
	case ver118, ver120:
		t.nfunctab = uint32(offset(0))
		t.nfiletab = uint32(offset(1))
		t.textStart = t.PC // use the start PC instead of reading from the table, which may be unrelocated
		t.funcnametab = data(3)
		t.cutab = data(4)
		t.filetab = data(5)
		t.pctab = data(6)
		t.funcdata = data(7)
		t.functab = data(7)
		functabsize := (int(t.nfunctab)*2 + 1) * t.functabFieldSize()
		t.functab = t.functab[:functabsize]
	case ver116:
		t.nfunctab = uint32(offset(0))
		t.nfiletab = uint32(offset(1))
		t.funcnametab = data(2)
		t.cutab = data(3)
		t.filetab = data(4)
		t.pctab = data(5)
		t.funcdata = data(6)
		t.functab = data(6)
		functabsize := (int(t.nfunctab)*2 + 1) * t.functabFieldSize()
		t.functab = t.functab[:functabsize]
	case ver12:
		t.nfunctab = uint32(t.uintptr(t.Data[8:]))
		t.funcdata = t.Data
		t.funcnametab = t.Data
		t.functab = t.Data[8+t.ptrsize:]
		t.pctab = t.Data
		functabsize := (int(t.nfunctab)*2 + 1) * t.functabFieldSize()
		fileoff := t.binary.Uint32(t.functab[functabsize:])
		t.functab = t.functab[:functabsize]
		t.filetab = t.Data[fileoff:]
		t.nfiletab = t.binary.Uint32(t.filetab)
		t.filetab = t.filetab[:t.nfiletab*4]
	default:
		panic("unreachable")
	}
}

// go12Funcs returns a slice of Funcs derived from the Go 1.2+ pcln table.
func (t *LineTable) go12Funcs() []Func {
	// Assume it is malformed and return nil on error.
	if !disableRecover {
		defer func() {
			recover()
		}()
	}

	ft := t.funcTab()
	funcs := make([]Func, ft.Count())
	syms := make([]Sym, len(funcs))
	for i := range funcs {
		f := &funcs[i]
		f.Entry = ft.pc(i)
		f.End = ft.pc(i + 1)
		info := t.funcData(uint32(i))
		f.LineTable = t
		f.FrameSize = int(info.deferreturn())
		syms[i] = Sym{
			Value:     f.Entry,
			Type:      'T',
			Name:      t.funcName(info.nameOff()),
			GoType:    0,
			Func:      f,
			goVersion: t.version,
		}
		f.Sym = &syms[i]
	}
	return funcs
}

// findFunc returns the funcData corresponding to the given program counter.
func (t *LineTable) findFunc(pc uint64) funcData {
	ft := t.funcTab()
	if pc < ft.pc(0) || pc >= ft.pc(ft.Count()) {
		return funcData{}
	}
	idx := sort.Search(int(t.nfunctab), func(i int) bool {
		return ft.pc(i) > pc
	})
	idx--
	return t.funcData(uint32(idx))
}

// readvarint reads, removes, and returns a varint from *pp.
func (t *LineTable) readvarint(pp *[]byte) uint32 {
	var v, shift uint32
	p := *pp
	for shift = 0; ; shift += 7 {
		b := p[0]
		p = p[1:]
		v |= (uint32(b) & 0x7F) << shift
		if b&0x80 == 0 {
			break
		}
	}
	*pp = p
	return v
}

// funcName returns the name of the function found at off.
func (t *LineTable) funcName(off uint32) string {
	if s, ok := t.funcNames[off]; ok {
		return s
	}
	i := bytes.IndexByte(t.funcnametab[off:], 0)
	s := string(t.funcnametab[off : off+uint32(i)])
	t.funcNames[off] = s
	return s
}

// stringFrom returns a Go string found at off from a position.
func (t *LineTable) stringFrom(arr []byte, off uint32) string {
	if s, ok := t.strings[off]; ok {
		return s
	}
	i := bytes.IndexByte(arr[off:], 0)
	s := string(arr[off : off+uint32(i)])
	t.strings[off] = s
	return s
}

// string returns a Go string found at off.
func (t *LineTable) string(off uint32) string {
	return t.stringFrom(t.funcdata, off)
}

// functabFieldSize returns the size in bytes of a single functab field.
func (t *LineTable) functabFieldSize() int {
	if t.version >= ver118 {
		return 4
	}
	return int(t.ptrsize)
}

// funcTab returns t's funcTab.
func (t *LineTable) funcTab() funcTab {
	return funcTab{LineTable: t, sz: t.functabFieldSize()}
}

// funcTab is memory corresponding to a slice of functab structs, followed by an invalid PC.
// A functab struct is a PC and a func offset.
type funcTab struct {
	*LineTable
	sz int // cached result of t.functabFieldSize
}

// Count returns the number of func entries in f.
func (f funcTab) Count() int {
	return int(f.nfunctab)
}

// pc returns the PC of the i'th func in f.
func (f funcTab) pc(i int) uint64 {
	u := f.uint(f.functab[2*i*f.sz:])
	if f.version >= ver118 {
		u += f.textStart
	}
	return u
}

// funcOff returns the funcdata offset of the i'th func in f.
func (f funcTab) funcOff(i int) uint64 {
	return f.uint(f.functab[(2*i+1)*f.sz:])
}

// uint returns the uint stored at b.
func (f funcTab) uint(b []byte) uint64 {
	if f.sz == 4 {
		return uint64(f.binary.Uint32(b))
	}
	return f.binary.Uint64(b)
}

// funcData is memory corresponding to an _func struct.
type funcData struct {
	t    *LineTable // LineTable this data is a part of
	data []byte     // raw memory for the function
}

// funcData returns the ith funcData in t.functab.
func (t *LineTable) funcData(i uint32) funcData {
	data := t.funcdata[t.funcTab().funcOff(int(i)):]
	return funcData{t: t, data: data}
}

// IsZero reports whether f is the zero value.
func (f funcData) IsZero() bool {
	return f.t == nil && f.data == nil
}

// entryPC returns the func's entry PC.
func (f *funcData) entryPC() uint64 {
	// In Go 1.18, the first field of _func changed
	// from a uintptr entry PC to a uint32 entry offset.
	if f.t.version >= ver118 {
		// TODO: support multiple text sections.
		// See runtime/symtab.go:(*moduledata).textAddr.
		return uint64(f.t.binary.Uint32(f.data)) + f.t.textStart
	}
	return f.t.uintptr(f.data)
}

func (f funcData) nameOff() uint32     { return f.field(1) }
func (f funcData) deferreturn() uint32 { return f.field(3) }
func (f funcData) pcfile() uint32      { return f.field(5) }
func (f funcData) pcln() uint32        { return f.field(6) }
func (f funcData) cuOffset() uint32    { return f.field(8) }

// field returns the nth field of the _func struct.
// It panics if n == 0 or n > 9; for n == 0, call f.entryPC.
// Most callers should use a named field accessor (just above).
func (f funcData) field(n uint32) uint32 {
	if n == 0 || n > 9 {
		panic("bad funcdata field")
	}
	// In Go 1.18, the first field of _func changed
	// from a uintptr entry PC to a uint32 entry offset.
	sz0 := f.t.ptrsize
	if f.t.version >= ver118 {
		sz0 = 4
	}
	off := sz0 + (n-1)*4 // subsequent fields are 4 bytes each
	data := f.data[off:]
	return f.t.binary.Uint32(data)
}

// step advances to the next pc, value pair in the encoded table.
func (t *LineTable) step(p *[]byte, pc *uint64, val *int32, first bool) bool {
	uvdelta := t.readvarint(p)
	if uvdelta == 0 && !first {
		return false
	}
	if uvdelta&1 != 0 {
		uvdelta = ^(uvdelta >> 1)
	} else {
		uvdelta >>= 1
	}
	vdelta := int32(uvdelta)
	pcdelta := t.readvarint(p) * t.quantum
	*pc += uint64(pcdelta)
	*val += vdelta
	return true
}

// pcvalue reports the value associated with the target pc.
// off is the offset to the beginning of the pc-value table,
// and entry is the start PC for the corresponding function.
func (t *LineTable) pcvalue(off uint32, entry, targetpc uint64) int32 {
	p := t.pctab[off:]

	val := int32(-1)
	pc := entry
	for t.step(&p, &pc, &val, pc == entry) {
		if targetpc < pc {
			return val
		}
	}
	return -1
}

// findFileLine scans one function in the binary looking for a
// program counter in the given file on the given line.
// It does so by running the pc-value tables mapping program counter
// to file number. Since most functions come from a single file, these
// are usually short and quick to scan. If a file match is found, then the
// code goes to the expense of looking for a simultaneous line number match.
func (t *LineTable) findFileLine(entry uint64, filetab, linetab uint32, filenum, line int32, cutab []byte) uint64 {
	if filetab == 0 || linetab == 0 {
		return 0
	}

	fp := t.pctab[filetab:]
	fl := t.pctab[linetab:]
	fileVal := int32(-1)
	filePC := entry
	lineVal := int32(-1)
	linePC := entry
	fileStartPC := filePC
	for t.step(&fp, &filePC, &fileVal, filePC == entry) {
		fileIndex := fileVal
		if t.version == ver116 || t.version == ver118 || t.version == ver120 {
			fileIndex = int32(t.binary.Uint32(cutab[fileVal*4:]))
		}
		if fileIndex == filenum && fileStartPC < filePC {
			// fileIndex is in effect starting at fileStartPC up to
			// but not including filePC, and it's the file we want.
			// Run the PC table looking for a matching line number
			// or until we reach filePC.
			lineStartPC := linePC
			for linePC < filePC && t.step(&fl, &linePC, &lineVal, linePC == entry) {
				// lineVal is in effect until linePC, and lineStartPC < filePC.
				if lineVal == line {
					if fileStartPC <= lineStartPC {
						return lineStartPC
					}
					if fileStartPC < linePC {
						return fileStartPC
					}
				}
				lineStartPC = linePC
			}
		}
		fileStartPC = filePC
	}
	return 0
}

// go12PCToLine maps program counter to line number for the Go 1.2+ pcln table.
func (t *LineTable) go12PCToLine(pc uint64) (line int) {
	defer func() {
		if !disableRecover && recover() != nil {
			line = -1
		}
	}()

	f := t.findFunc(pc)
	if f.IsZero() {
		return -1
	}
	entry := f.entryPC()
	linetab := f.pcln()
	return int(t.pcvalue(linetab, entry, pc))
}

// go12PCToFile maps program counter to file name for the Go 1.2+ pcln table.
func (t *LineTable) go12PCToFile(pc uint64) (file string) {
	defer func() {
		if !disableRecover && recover() != nil {
			file = ""
		}
	}()

	f := t.findFunc(pc)
	if f.IsZero() {
		return ""
	}
	entry := f.entryPC()
	filetab := f.pcfile()
	fno := t.pcvalue(filetab, entry, pc)
	if t.version == ver12 {
		if fno <= 0 {
			return ""
		}
		return t.string(t.binary.Uint32(t.filetab[4*fno:]))
	}
	// Go ≥ 1.16
	if fno < 0 { // 0 is valid for ≥ 1.16
		return ""
	}
	cuoff := f.cuOffset()
	if fnoff := t.binary.Uint32(t.cutab[(cuoff+uint32(fno))*4:]); fnoff != ^uint32(0) {
		return t.stringFrom(t.filetab, fnoff)
	}
	return ""
}

// go12LineToPC maps a (file, line) pair to a program counter for the Go 1.2+ pcln table.
func (t *LineTable) go12LineToPC(file string, line int) (pc uint64) {
	defer func() {
		if !disableRecover && recover() != nil {
			pc = 0
		}
	}()

	t.initFileMap()
	filenum, ok := t.fileMap[file]
	if !ok {
		return 0
	}

	// Scan all functions.
	// If this turns out to be a bottleneck, we could build a map[int32][]int32
	// mapping file number to a list of functions with code from that file.
	var cutab []byte
	for i := uint32(0); i < t.nfunctab; i++ {
		f := t.funcData(i)
		entry := f.entryPC()
		filetab := f.pcfile()
		linetab := f.pcln()
		if t.version == ver116 || t.version == ver118 || t.version == ver120 {
			if f.cuOffset() == ^uint32(0) {
				// skip functions without compilation unit (not real function, or linker generated)
				continue
			}
			cutab = t.cutab[f.cuOffset()*4:]
		}
		pc := t.findFileLine(entry, filetab, linetab, int32(filenum), int32(line), cutab)
		if pc != 0 {
			return pc
		}
	}
	return 0
}

// initFileMap initializes the map from file name to file number.
func (t *LineTable) initFileMap() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.fileMap != nil {
		return
	}
	m := make(map[string]uint32)

	if t.version == ver12 {
		for i := uint32(1); i < t.nfiletab; i++ {
			s := t.string(t.binary.Uint32(t.filetab[4*i:]))
			m[s] = i
		}
	} else {
		var pos uint32
		for i := uint32(0); i < t.nfiletab; i++ {
			s := t.stringFrom(t.filetab, pos)
			m[s] = pos
			pos += uint32(len(s) + 1)
		}
	}
	t.fileMap = m
}

// go12MapFiles adds to m a key for every file in the Go 1.2 LineTable.
// Every key maps to obj. That's not a very interesting map, but it provides
// a way for callers to obtain the list of files in the program.
func (t *LineTable) go12MapFiles(m map[string]*Obj, obj *Obj) {
	if !disableRecover {
		defer func() {
			recover()
		}()
	}

	t.initFileMap()
	for file := range t.fileMap {
		m[file] = obj
	}
}

// disableRecover causes this package not to swallow panics.
// This is useful when making changes.
const disableRecover = false

```

// === FILE: references!/go/src/debug/gosym/symtab.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gosym implements access to the Go symbol
// and line number tables embedded in Go binaries generated
// by the gc compilers.
package gosym

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

/*
 * Symbols
 */

// A Sym represents a single symbol table entry.
type Sym struct {
	Value  uint64
	Type   byte
	Name   string
	GoType uint64
	// If this symbol is a function symbol, the corresponding Func
	Func *Func

	goVersion version
}

// Static reports whether this symbol is static (not visible outside its file).
func (s *Sym) Static() bool { return s.Type >= 'a' }

// nameWithoutInst returns s.Name with all bracketed expressions masked by
// underscores. This is useful to ignore any extra slashes or dots inside the
// brackets from the string searches below, while preserving the byte indices
// of the characters outside the brackets.
//
// s.Name is returned as-is if the brackets are imbalanced.
func (s *Sym) nameWithoutInst() string {
	n := 0
	b := []byte(s.Name)
	for i, r := range b {
		switch r {
		case '[':
			n++
		case ']':
			n--
			if n < 0 {
				return s.Name // malformed
			}
		default:
			if n > 0 {
				b[i] = '_'
			}
		}
	}
	if n > 0 {
		return s.Name // malformed
	}
	return string(b)
}

// PackageName returns the package part of the symbol name,
// or the empty string if there is none.
func (s *Sym) PackageName() string {
	name := s.nameWithoutInst()

	// Since go1.20, a prefix of "type:" and "go:" is a compiler-generated symbol,
	// they do not belong to any package.
	//
	// See cmd/compile/internal/base/link.go:ReservedImports variable.
	if s.goVersion >= ver120 && (strings.HasPrefix(name, "go:") || strings.HasPrefix(name, "type:")) {
		return ""
	}

	// For go1.18 and below, the prefix are "type." and "go." instead.
	if s.goVersion <= ver118 && (strings.HasPrefix(name, "go.") || strings.HasPrefix(name, "type.")) {
		return ""
	}

	pathend := strings.LastIndex(name, "/")
	if pathend < 0 {
		pathend = 0
	}

	if i := strings.Index(name[pathend:], "."); i != -1 {
		return s.Name[:pathend+i]
	}
	return ""
}

// ReceiverName returns the receiver type name of this symbol,
// or the empty string if there is none.  A receiver name is only detected in
// the case that s.Name is fully-specified with a package name.
func (s *Sym) ReceiverName() string {
	name := s.nameWithoutInst()
	pathend := strings.LastIndex(name, "/")
	if pathend < 0 {
		pathend = 0
	}
	// Find the first dot after pathend (or from the beginning, if there was
	// no slash in name).
	l := strings.Index(name[pathend:], ".")
	// Find the last dot after pathend (or the beginning).
	r := strings.LastIndex(name[pathend:], ".")
	if l == -1 || r == -1 || l == r {
		// There is no receiver if we didn't find two distinct dots after pathend.
		return ""
	}
	return s.Name[pathend+l+1 : pathend+r]
}

// BaseName returns the symbol name without the package or receiver name.
func (s *Sym) BaseName() string {
	name := s.nameWithoutInst()
	if i := strings.LastIndex(name, "."); i != -1 {
		return s.Name[i+1:]
	}
	return s.Name
}

// A Func collects information about a single function.
type Func struct {
	Entry uint64
	*Sym
	End       uint64
	Params    []*Sym // nil for Go 1.3 and later binaries
	Locals    []*Sym // nil for Go 1.3 and later binaries
	FrameSize int
	LineTable *LineTable
	Obj       *Obj
}

// An Obj represents a collection of functions in a symbol table.
//
// The exact method of division of a binary into separate Objs is an internal detail
// of the symbol table format.
//
// In early versions of Go each source file became a different Obj.
//
// In Go 1 and Go 1.1, each package produced one Obj for all Go sources
// and one Obj per C source file.
//
// In Go 1.2, there is a single Obj for the entire program.
type Obj struct {
	// Funcs is a list of functions in the Obj.
	Funcs []Func

	// In Go 1.1 and earlier, Paths is a list of symbols corresponding
	// to the source file names that produced the Obj.
	// In Go 1.2, Paths is nil.
	// Use the keys of Table.Files to obtain a list of source files.
	Paths []Sym // meta
}

/*
 * Symbol tables
 */

// Table represents a Go symbol table. It stores all of the
// symbols decoded from the program and provides methods to translate
// between symbols, names, and addresses.
type Table struct {
	Syms  []Sym // nil for Go 1.3 and later binaries
	Funcs []Func
	Files map[string]*Obj // for Go 1.2 and later all files map to one Obj
	Objs  []Obj           // for Go 1.2 and later only one Obj in slice

	go12line *LineTable // Go 1.2 line number table
}

type sym struct {
	value  uint64
	gotype uint64
	typ    byte
	name   []byte
}

var (
	littleEndianSymtab    = []byte{0xFD, 0xFF, 0xFF, 0xFF, 0x00, 0x00, 0x00}
	bigEndianSymtab       = []byte{0xFF, 0xFF, 0xFF, 0xFD, 0x00, 0x00, 0x00}
	oldLittleEndianSymtab = []byte{0xFE, 0xFF, 0xFF, 0xFF, 0x00, 0x00}
)

func walksymtab(data []byte, fn func(sym) error) error {
	if len(data) == 0 { // missing symtab is okay
		return nil
	}
	var order binary.ByteOrder = binary.BigEndian
	newTable := false
	switch {
	case bytes.HasPrefix(data, oldLittleEndianSymtab):
		// Same as Go 1.0, but little endian.
		// Format was used during interim development between Go 1.0 and Go 1.1.
		// Should not be widespread, but easy to support.
		data = data[6:]
		order = binary.LittleEndian
	case bytes.HasPrefix(data, bigEndianSymtab):
		newTable = true
	case bytes.HasPrefix(data, littleEndianSymtab):
		newTable = true
		order = binary.LittleEndian
	}
	var ptrsz int
	if newTable {
		if len(data) < 8 {
			return &DecodingError{len(data), "unexpected EOF", nil}
		}
		ptrsz = int(data[7])
		if ptrsz != 4 && ptrsz != 8 {
			return &DecodingError{7, "invalid pointer size", ptrsz}
		}
		data = data[8:]
	}
	var s sym
	p := data
	for len(p) >= 4 {
		var typ byte
		if newTable {
			// Symbol type, value, Go type.
			typ = p[0] & 0x3F
			wideValue := p[0]&0x40 != 0
			goType := p[0]&0x80 != 0
			if typ < 26 {
				typ += 'A'
			} else {
				typ += 'a' - 26
			}
			s.typ = typ
			p = p[1:]
			if wideValue {
				if len(p) < ptrsz {
					return &DecodingError{len(data), "unexpected EOF", nil}
				}
				// fixed-width value
				if ptrsz == 8 {
					s.value = order.Uint64(p[0:8])
					p = p[8:]
				} else {
					s.value = uint64(order.Uint32(p[0:4]))
					p = p[4:]
				}
			} else {
				// varint value
				s.value = 0
				shift := uint(0)
				for len(p) > 0 && p[0]&0x80 != 0 {
					s.value |= uint64(p[0]&0x7F) << shift
					shift += 7
					p = p[1:]
				}
				if len(p) == 0 {
					return &DecodingError{len(data), "unexpected EOF", nil}
				}
				s.value |= uint64(p[0]) << shift
				p = p[1:]
			}
			if goType {
				if len(p) < ptrsz {
					return &DecodingError{len(data), "unexpected EOF", nil}
				}
				// fixed-width go type
				if ptrsz == 8 {
					s.gotype = order.Uint64(p[0:8])
					p = p[8:]
				} else {
					s.gotype = uint64(order.Uint32(p[0:4]))
					p = p[4:]
				}
			}
		} else {
			// Value, symbol type.
			s.value = uint64(order.Uint32(p[0:4]))
			if len(p) < 5 {
				return &DecodingError{len(data), "unexpected EOF", nil}
			}
			typ = p[4]
			if typ&0x80 == 0 {
				return &DecodingError{len(data) - len(p) + 4, "bad symbol type", typ}
			}
			typ &^= 0x80
			s.typ = typ
			p = p[5:]
		}

		// Name.
		var i int
		var nnul int
		for i = 0; i < len(p); i++ {
			if p[i] == 0 {
				nnul = 1
				break
			}
		}
		switch typ {
		case 'z', 'Z':
			p = p[i+nnul:]
			for i = 0; i+2 <= len(p); i += 2 {
				if p[i] == 0 && p[i+1] == 0 {
					nnul = 2
					break
				}
			}
		}
		if len(p) < i+nnul {
			return &DecodingError{len(data), "unexpected EOF", nil}
		}
		s.name = p[0:i]
		i += nnul
		p = p[i:]

		if !newTable {
			if len(p) < 4 {
				return &DecodingError{len(data), "unexpected EOF", nil}
			}
			// Go type.
			s.gotype = uint64(order.Uint32(p[:4]))
			p = p[4:]
		}
		fn(s)
	}
	return nil
}

// NewTable decodes the Go symbol table (the ".gosymtab" section in ELF),
// returning an in-memory representation.
// Starting with Go 1.3, the Go symbol table no longer includes symbol data;
// callers should pass nil for the symtab parameter.
func NewTable(symtab []byte, pcln *LineTable) (*Table, error) {
	var n int
	err := walksymtab(symtab, func(s sym) error {
		n++
		return nil
	})
	if err != nil {
		return nil, err
	}

	var t Table
	if pcln.isGo12() {
		t.go12line = pcln
	}
	fname := make(map[uint16]string)
	t.Syms = make([]Sym, 0, n)
	nf := 0
	nz := 0
	lasttyp := uint8(0)
	err = walksymtab(symtab, func(s sym) error {
		n := len(t.Syms)
		t.Syms = t.Syms[0 : n+1]
		ts := &t.Syms[n]
		ts.Type = s.typ
		ts.Value = s.value
		ts.GoType = s.gotype
		ts.goVersion = pcln.version
		switch s.typ {
		default:
			// rewrite name to use . instead of · (c2 b7)
			w := 0
			b := s.name
			for i := 0; i < len(b); i++ {
				if b[i] == 0xc2 && i+1 < len(b) && b[i+1] == 0xb7 {
					i++
					b[i] = '.'
				}
				b[w] = b[i]
				w++
			}
			ts.Name = string(s.name[0:w])
		case 'z', 'Z':
			if lasttyp != 'z' && lasttyp != 'Z' {
				nz++
			}
			for i := 0; i < len(s.name); i += 2 {
				eltIdx := binary.BigEndian.Uint16(s.name[i : i+2])
				elt, ok := fname[eltIdx]
				if !ok {
					return &DecodingError{-1, "bad filename code", eltIdx}
				}
				if n := len(ts.Name); n > 0 && ts.Name[n-1] != '/' {
					ts.Name += "/"
				}
				ts.Name += elt
			}
		}
		switch s.typ {
		case 'T', 't', 'L', 'l':
			nf++
		case 'f':
			fname[uint16(s.value)] = ts.Name
		}
		lasttyp = s.typ
		return nil
	})
	if err != nil {
		return nil, err
	}

	t.Funcs = make([]Func, 0, nf)
	t.Files = make(map[string]*Obj)

	var obj *Obj
	if t.go12line != nil {
		// Put all functions into one Obj.
		t.Objs = make([]Obj, 1)
		obj = &t.Objs[0]
		t.go12line.go12MapFiles(t.Files, obj)
	} else {
		t.Objs = make([]Obj, 0, nz)
	}

	// Count text symbols and attach frame sizes, parameters, and
	// locals to them. Also, find object file boundaries.
	lastf := 0
	for i := 0; i < len(t.Syms); i++ {
		sym := &t.Syms[i]
		switch sym.Type {
		case 'Z', 'z': // path symbol
			if t.go12line != nil {
				// Go 1.2 binaries have the file information elsewhere. Ignore.
				break
			}
			// Finish the current object
			if obj != nil {
				obj.Funcs = t.Funcs[lastf:]
			}
			lastf = len(t.Funcs)

			// Start new object
			n := len(t.Objs)
			t.Objs = t.Objs[0 : n+1]
			obj = &t.Objs[n]

			// Count & copy path symbols
			var end int
			for end = i + 1; end < len(t.Syms); end++ {
				if c := t.Syms[end].Type; c != 'Z' && c != 'z' {
					break
				}
			}
			obj.Paths = t.Syms[i:end]
			i = end - 1 // loop will i++

			// Record file names
			depth := 0
			for j := range obj.Paths {
				s := &obj.Paths[j]
				if s.Name == "" {
					depth--
				} else {
					if depth == 0 {
						t.Files[s.Name] = obj
					}
					depth++
				}
			}

		case 'T', 't', 'L', 'l': // text symbol
			if n := len(t.Funcs); n > 0 {
				t.Funcs[n-1].End = sym.Value
			}
			if sym.Name == "runtime.etext" || sym.Name == "etext" {
				continue
			}

			// Count parameter and local (auto) syms
			var np, na int
			var end int
		countloop:
			for end = i + 1; end < len(t.Syms); end++ {
				switch t.Syms[end].Type {
				case 'T', 't', 'L', 'l', 'Z', 'z':
					break countloop
				case 'p':
					np++
				case 'a':
					na++
				}
			}

			// Fill in the function symbol
			n := len(t.Funcs)
			t.Funcs = t.Funcs[0 : n+1]
			fn := &t.Funcs[n]
			sym.Func = fn
			fn.Params = make([]*Sym, 0, np)
			fn.Locals = make([]*Sym, 0, na)
			fn.Sym = sym
			fn.Entry = sym.Value
			fn.Obj = obj
			if t.go12line != nil {
				// All functions share the same line table.
				// It knows how to narrow down to a specific
				// function quickly.
				fn.LineTable = t.go12line
			} else if pcln != nil {
				fn.LineTable = pcln.slice(fn.Entry)
				pcln = fn.LineTable
			}
			for j := i; j < end; j++ {
				s := &t.Syms[j]
				switch s.Type {
				case 'm':
					fn.FrameSize = int(s.Value)
				case 'p':
					n := len(fn.Params)
					fn.Params = fn.Params[0 : n+1]
					fn.Params[n] = s
				case 'a':
					n := len(fn.Locals)
					fn.Locals = fn.Locals[0 : n+1]
					fn.Locals[n] = s
				}
			}
			i = end - 1 // loop will i++
		}
	}

	if t.go12line != nil && nf == 0 {
		t.Funcs = t.go12line.go12Funcs()
	}
	if obj != nil {
		obj.Funcs = t.Funcs[lastf:]
	}
	return &t, nil
}

// PCToFunc returns the function containing the program counter pc,
// or nil if there is no such function.
func (t *Table) PCToFunc(pc uint64) *Func {
	funcs := t.Funcs
	for len(funcs) > 0 {
		m := len(funcs) / 2
		fn := &funcs[m]
		switch {
		case pc < fn.Entry:
			funcs = funcs[0:m]
		case fn.Entry <= pc && pc < fn.End:
			return fn
		default:
			funcs = funcs[m+1:]
		}
	}
	return nil
}

// PCToLine looks up line number information for a program counter.
// If there is no information, it returns fn == nil.
func (t *Table) PCToLine(pc uint64) (file string, line int, fn *Func) {
	if fn = t.PCToFunc(pc); fn == nil {
		return
	}
	if t.go12line != nil {
		file = t.go12line.go12PCToFile(pc)
		line = t.go12line.go12PCToLine(pc)
	} else {
		file, line = fn.Obj.lineFromAline(fn.LineTable.PCToLine(pc))
	}
	return
}

// LineToPC looks up the first program counter on the given line in
// the named file. It returns [UnknownFileError] or [UnknownLineError] if
// there is an error looking up this line.
func (t *Table) LineToPC(file string, line int) (pc uint64, fn *Func, err error) {
	obj, ok := t.Files[file]
	if !ok {
		return 0, nil, UnknownFileError(file)
	}

	if t.go12line != nil {
		pc := t.go12line.go12LineToPC(file, line)
		if pc == 0 {
			return 0, nil, &UnknownLineError{file, line}
		}
		return pc, t.PCToFunc(pc), nil
	}

	abs, err := obj.alineFromLine(file, line)
	if err != nil {
		return
	}
	for i := range obj.Funcs {
		f := &obj.Funcs[i]
		pc := f.LineTable.LineToPC(abs, f.End)
		if pc != 0 {
			return pc, f, nil
		}
	}
	return 0, nil, &UnknownLineError{file, line}
}

// LookupSym returns the text, data, or bss symbol with the given name,
// or nil if no such symbol is found.
func (t *Table) LookupSym(name string) *Sym {
	// TODO(austin) Maybe make a map
	for i := range t.Syms {
		s := &t.Syms[i]
		switch s.Type {
		case 'T', 't', 'L', 'l', 'D', 'd', 'B', 'b':
			if s.Name == name {
				return s
			}
		}
	}
	return nil
}

// LookupFunc returns the text, data, or bss symbol with the given name,
// or nil if no such symbol is found.
func (t *Table) LookupFunc(name string) *Func {
	for i := range t.Funcs {
		f := &t.Funcs[i]
		if f.Sym.Name == name {
			return f
		}
	}
	return nil
}

// SymByAddr returns the text, data, or bss symbol starting at the given address.
func (t *Table) SymByAddr(addr uint64) *Sym {
	for i := range t.Syms {
		s := &t.Syms[i]
		switch s.Type {
		case 'T', 't', 'L', 'l', 'D', 'd', 'B', 'b':
			if s.Value == addr {
				return s
			}
		}
	}
	return nil
}

/*
 * Object files
 */

// This is legacy code for Go 1.1 and earlier, which used the
// Plan 9 format for pc-line tables. This code was never quite
// correct. It's probably very close, and it's usually correct, but
// we never quite found all the corner cases.
//
// Go 1.2 and later use a simpler format, documented at golang.org/s/go12symtab.

func (o *Obj) lineFromAline(aline int) (string, int) {
	type stackEnt struct {
		path   string
		start  int
		offset int
		prev   *stackEnt
	}

	noPath := &stackEnt{"", 0, 0, nil}
	tos := noPath

pathloop:
	for _, s := range o.Paths {
		val := int(s.Value)
		switch {
		case val > aline:
			break pathloop

		case val == 1:
			// Start a new stack
			tos = &stackEnt{s.Name, val, 0, noPath}

		case s.Name == "":
			// Pop
			if tos == noPath {
				return "<malformed symbol table>", 0
			}
			tos.prev.offset += val - tos.start
			tos = tos.prev

		default:
			// Push
			tos = &stackEnt{s.Name, val, 0, tos}
		}
	}

	if tos == noPath {
		return "", 0
	}
	return tos.path, aline - tos.start - tos.offset + 1
}

func (o *Obj) alineFromLine(path string, line int) (int, error) {
	if line < 1 {
		return 0, &UnknownLineError{path, line}
	}

	for i, s := range o.Paths {
		// Find this path
		if s.Name != path {
			continue
		}

		// Find this line at this stack level
		depth := 0
		var incstart int
		line += int(s.Value)
	pathloop:
		for _, s := range o.Paths[i:] {
			val := int(s.Value)
			switch {
			case depth == 1 && val >= line:
				return line - 1, nil

			case s.Name == "":
				depth--
				if depth == 0 {
					break pathloop
				} else if depth == 1 {
					line += val - incstart
				}

			default:
				if depth == 1 {
					incstart = val
				}
				depth++
			}
		}
		return 0, &UnknownLineError{path, line}
	}
	return 0, UnknownFileError(path)
}

/*
 * Errors
 */

// UnknownFileError represents a failure to find the specific file in
// the symbol table.
type UnknownFileError string

func (e UnknownFileError) Error() string { return "unknown file: " + string(e) }

// UnknownLineError represents a failure to map a line to a program
// counter, either because the line is beyond the bounds of the file
// or because there is no code on the given line.
type UnknownLineError struct {
	File string
	Line int
}

func (e *UnknownLineError) Error() string {
	return "no code at " + e.File + ":" + strconv.Itoa(e.Line)
}

// DecodingError represents an error during the decoding of
// the symbol table.
type DecodingError struct {
	off int
	msg string
	val any
}

func (e *DecodingError) Error() string {
	msg := e.msg
	if e.val != nil {
		msg += fmt.Sprintf(" '%v'", e.val)
	}
	msg += fmt.Sprintf(" at byte %#x", e.off)
	return msg
}

```

// === FILE: references!/go/src/debug/macho/fat.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macho

import (
	"encoding/binary"
	"fmt"
	"internal/saferio"
	"io"
	"os"
)

// A FatFile is a Mach-O universal binary that contains at least one architecture.
type FatFile struct {
	Magic  uint32
	Arches []FatArch
	closer io.Closer
}

// A FatArchHeader represents a fat header for a specific image architecture.
type FatArchHeader struct {
	Cpu    Cpu
	SubCpu uint32
	Offset uint32
	Size   uint32
	Align  uint32
}

const fatArchHeaderSize = 5 * 4

// A FatArch is a Mach-O File inside a FatFile.
type FatArch struct {
	FatArchHeader
	*File
}

// ErrNotFat is returned from [NewFatFile] or [OpenFat] when the file is not a
// universal binary but may be a thin binary, based on its magic number.
var ErrNotFat = &FormatError{0, "not a fat Mach-O file", nil}

// NewFatFile creates a new [FatFile] for accessing all the Mach-O images in a
// universal binary. The Mach-O binary is expected to start at position 0 in
// the ReaderAt.
func NewFatFile(r io.ReaderAt) (*FatFile, error) {
	var ff FatFile
	sr := io.NewSectionReader(r, 0, 1<<63-1)

	// Read the fat_header struct, which is always in big endian.
	// Start with the magic number.
	err := binary.Read(sr, binary.BigEndian, &ff.Magic)
	if err != nil {
		return nil, &FormatError{0, "error reading magic number", nil}
	} else if ff.Magic != MagicFat {
		// See if this is a Mach-O file via its magic number. The magic
		// must be converted to little endian first though.
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], ff.Magic)
		leMagic := binary.LittleEndian.Uint32(buf[:])
		if leMagic == Magic32 || leMagic == Magic64 {
			return nil, ErrNotFat
		} else {
			return nil, &FormatError{0, "invalid magic number", nil}
		}
	}
	offset := int64(4)

	// Read the number of FatArchHeaders that come after the fat_header.
	var narch uint32
	err = binary.Read(sr, binary.BigEndian, &narch)
	if err != nil {
		return nil, &FormatError{offset, "invalid fat_header", nil}
	}
	offset += 4

	if narch < 1 {
		return nil, &FormatError{offset, "file contains no images", nil}
	}

	// Combine the Cpu and SubCpu (both uint32) into a uint64 to make sure
	// there are not duplicate architectures.
	seenArches := make(map[uint64]bool)
	// Make sure that all images are for the same MH_ type.
	var machoType Type

	// Following the fat_header comes narch fat_arch structs that index
	// Mach-O images further in the file.
	c := saferio.SliceCap[FatArch](uint64(narch))
	if c < 0 {
		return nil, &FormatError{offset, "too many images", nil}
	}
	ff.Arches = make([]FatArch, 0, c)
	for i := uint32(0); i < narch; i++ {
		var fa FatArch
		err = binary.Read(sr, binary.BigEndian, &fa.FatArchHeader)
		if err != nil {
			return nil, &FormatError{offset, "invalid fat_arch header", nil}
		}
		offset += fatArchHeaderSize

		fr := io.NewSectionReader(r, int64(fa.Offset), int64(fa.Size))
		fa.File, err = NewFile(fr)
		if err != nil {
			return nil, err
		}

		// Make sure the architecture for this image is not duplicate.
		seenArch := (uint64(fa.Cpu) << 32) | uint64(fa.SubCpu)
		if o, k := seenArches[seenArch]; o || k {
			return nil, &FormatError{offset, fmt.Sprintf("duplicate architecture cpu=%v, subcpu=%#x", fa.Cpu, fa.SubCpu), nil}
		}
		seenArches[seenArch] = true

		// Make sure the Mach-O type matches that of the first image.
		if i == 0 {
			machoType = fa.Type
		} else {
			if fa.Type != machoType {
				return nil, &FormatError{offset, fmt.Sprintf("Mach-O type for architecture #%d (type=%#x) does not match first (type=%#x)", i, fa.Type, machoType), nil}
			}
		}

		ff.Arches = append(ff.Arches, fa)
	}

	return &ff, nil
}

// OpenFat opens the named file using [os.Open] and prepares it for use as a Mach-O
// universal binary.
func OpenFat(name string) (*FatFile, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	ff, err := NewFatFile(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	ff.closer = f
	return ff, nil
}

func (ff *FatFile) Close() error {
	var err error
	if ff.closer != nil {
		err = ff.closer.Close()
		ff.closer = nil
	}
	return err
}

```

// === FILE: references!/go/src/debug/macho/file.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package macho implements access to Mach-O object files.

# Security

This package is not designed to be hardened against adversarial inputs, and is
outside the scope of https://go.dev/security/policy. In particular, only basic
validation is done when parsing object files. As such, care should be taken when
parsing untrusted inputs, as parsing malformed files may consume significant
resources, or cause panics.
*/
package macho

// High level access to low level data structures.

import (
	"bytes"
	"compress/zlib"
	"debug/dwarf"
	"encoding/binary"
	"fmt"
	"internal/saferio"
	"io"
	"os"
	"strings"
)

// A File represents an open Mach-O file.
type File struct {
	FileHeader
	ByteOrder binary.ByteOrder
	Loads     []Load
	Sections  []*Section

	Symtab   *Symtab
	Dysymtab *Dysymtab

	closer io.Closer
}

// A Load represents any Mach-O load command.
type Load interface {
	Raw() []byte
}

// A LoadBytes is the uninterpreted bytes of a Mach-O load command.
type LoadBytes []byte

func (b LoadBytes) Raw() []byte { return b }

// A SegmentHeader is the header for a Mach-O 32-bit or 64-bit load segment command.
type SegmentHeader struct {
	Cmd     LoadCmd
	Len     uint32
	Name    string
	Addr    uint64
	Memsz   uint64
	Offset  uint64
	Filesz  uint64
	Maxprot uint32
	Prot    uint32
	Nsect   uint32
	Flag    uint32
}

// A Segment represents a Mach-O 32-bit or 64-bit load segment command.
type Segment struct {
	LoadBytes
	SegmentHeader

	// Embed ReaderAt for ReadAt method.
	// Do not embed SectionReader directly
	// to avoid having Read and Seek.
	// If a client wants Read and Seek it must use
	// Open() to avoid fighting over the seek offset
	// with other clients.
	io.ReaderAt
	sr *io.SectionReader
}

// Data reads and returns the contents of the segment.
func (s *Segment) Data() ([]byte, error) {
	return saferio.ReadDataAt(s.sr, s.Filesz, 0)
}

// Open returns a new ReadSeeker reading the segment.
func (s *Segment) Open() io.ReadSeeker { return io.NewSectionReader(s.sr, 0, 1<<63-1) }

type SectionHeader struct {
	Name   string
	Seg    string
	Addr   uint64
	Size   uint64
	Offset uint32
	Align  uint32
	Reloff uint32
	Nreloc uint32
	Flags  uint32
}

// A Reloc represents a Mach-O relocation.
type Reloc struct {
	Addr  uint32
	Value uint32
	// when Scattered == false && Extern == true, Value is the symbol number.
	// when Scattered == false && Extern == false, Value is the section number.
	// when Scattered == true, Value is the value that this reloc refers to.
	Type      uint8
	Len       uint8 // 0=byte, 1=word, 2=long, 3=quad
	Pcrel     bool
	Extern    bool // valid if Scattered == false
	Scattered bool
}

type Section struct {
	SectionHeader
	Relocs []Reloc

	// Embed ReaderAt for ReadAt method.
	// Do not embed SectionReader directly
	// to avoid having Read and Seek.
	// If a client wants Read and Seek it must use
	// Open() to avoid fighting over the seek offset
	// with other clients.
	io.ReaderAt
	sr *io.SectionReader
}

// Data reads and returns the contents of the Mach-O section.
func (s *Section) Data() ([]byte, error) {
	return saferio.ReadDataAt(s.sr, s.Size, 0)
}

// Open returns a new ReadSeeker reading the Mach-O section.
func (s *Section) Open() io.ReadSeeker { return io.NewSectionReader(s.sr, 0, 1<<63-1) }

// A Dylib represents a Mach-O load dynamic library command.
type Dylib struct {
	LoadBytes
	Name           string
	Time           uint32
	CurrentVersion uint32
	CompatVersion  uint32
}

// A Symtab represents a Mach-O symbol table command.
type Symtab struct {
	LoadBytes
	SymtabCmd
	Syms []Symbol
}

// A Dysymtab represents a Mach-O dynamic symbol table command.
type Dysymtab struct {
	LoadBytes
	DysymtabCmd
	IndirectSyms []uint32 // indices into Symtab.Syms
}

// A Rpath represents a Mach-O rpath command.
type Rpath struct {
	LoadBytes
	Path string
}

// A Symbol is a Mach-O 32-bit or 64-bit symbol table entry.
type Symbol struct {
	Name  string
	Type  uint8
	Sect  uint8
	Desc  uint16
	Value uint64
}

/*
 * Mach-O reader
 */

// FormatError is returned by some operations if the data does
// not have the correct format for an object file.
type FormatError struct {
	off int64
	msg string
	val any
}

func (e *FormatError) Error() string {
	msg := e.msg
	if e.val != nil {
		msg += fmt.Sprintf(" '%v'", e.val)
	}
	msg += fmt.Sprintf(" in record at byte %#x", e.off)
	return msg
}

// Open opens the named file using [os.Open] and prepares it for use as a Mach-O binary.
func Open(name string) (*File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	ff, err := NewFile(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	ff.closer = f
	return ff, nil
}

// Close closes the [File].
// If the [File] was created using [NewFile] directly instead of [Open],
// Close has no effect.
func (f *File) Close() error {
	var err error
	if f.closer != nil {
		err = f.closer.Close()
		f.closer = nil
	}
	return err
}

// NewFile creates a new [File] for accessing a Mach-O binary in an underlying reader.
// The Mach-O binary is expected to start at position 0 in the ReaderAt.
func NewFile(r io.ReaderAt) (*File, error) {
	f := new(File)
	sr := io.NewSectionReader(r, 0, 1<<63-1)

	// Read and decode Mach magic to determine byte order, size.
	// Magic32 and Magic64 differ only in the bottom bit.
	var ident [4]byte
	if _, err := r.ReadAt(ident[0:], 0); err != nil {
		return nil, err
	}
	be := binary.BigEndian.Uint32(ident[0:])
	le := binary.LittleEndian.Uint32(ident[0:])
	switch Magic32 &^ 1 {
	case be &^ 1:
		f.ByteOrder = binary.BigEndian
		f.Magic = be
	case le &^ 1:
		f.ByteOrder = binary.LittleEndian
		f.Magic = le
	default:
		return nil, &FormatError{0, "invalid magic number", nil}
	}

	// Read entire file header.
	if err := binary.Read(sr, f.ByteOrder, &f.FileHeader); err != nil {
		return nil, err
	}

	// Then load commands.
	offset := int64(fileHeaderSize32)
	if f.Magic == Magic64 {
		offset = fileHeaderSize64
	}
	dat, err := saferio.ReadDataAt(r, uint64(f.Cmdsz), offset)
	if err != nil {
		return nil, err
	}
	c := saferio.SliceCap[Load](uint64(f.Ncmd))
	if c < 0 {
		return nil, &FormatError{offset, "too many load commands", nil}
	}
	f.Loads = make([]Load, 0, c)
	bo := f.ByteOrder
	for i := uint32(0); i < f.Ncmd; i++ {
		// Each load command begins with uint32 command and length.
		if len(dat) < 8 {
			return nil, &FormatError{offset, "command block too small", nil}
		}
		cmd, siz := LoadCmd(bo.Uint32(dat[0:4])), bo.Uint32(dat[4:8])
		if siz < 8 || siz > uint32(len(dat)) {
			return nil, &FormatError{offset, "invalid command block size", nil}
		}
		var cmddat []byte
		cmddat, dat = dat[0:siz], dat[siz:]
		offset += int64(siz)
		var s *Segment
		switch cmd {
		default:
			f.Loads = append(f.Loads, LoadBytes(cmddat))

		case LoadCmdRpath:
			var hdr RpathCmd
			b := bytes.NewReader(cmddat)
			if err := binary.Read(b, bo, &hdr); err != nil {
				return nil, err
			}
			l := new(Rpath)
			if hdr.Path >= uint32(len(cmddat)) {
				return nil, &FormatError{offset, "invalid path in rpath command", hdr.Path}
			}
			l.Path = cstring(cmddat[hdr.Path:])
			l.LoadBytes = LoadBytes(cmddat)
			f.Loads = append(f.Loads, l)

		case LoadCmdDylib:
			var hdr DylibCmd
			b := bytes.NewReader(cmddat)
			if err := binary.Read(b, bo, &hdr); err != nil {
				return nil, err
			}
			l := new(Dylib)
			if hdr.Name >= uint32(len(cmddat)) {
				return nil, &FormatError{offset, "invalid name in dynamic library command", hdr.Name}
			}
			l.Name = cstring(cmddat[hdr.Name:])
			l.Time = hdr.Time
			l.CurrentVersion = hdr.CurrentVersion
			l.CompatVersion = hdr.CompatVersion
			l.LoadBytes = LoadBytes(cmddat)
			f.Loads = append(f.Loads, l)

		case LoadCmdSymtab:
			var hdr SymtabCmd
			b := bytes.NewReader(cmddat)
			if err := binary.Read(b, bo, &hdr); err != nil {
				return nil, err
			}
			strtab, err := saferio.ReadDataAt(r, uint64(hdr.Strsize), int64(hdr.Stroff))
			if err != nil {
				return nil, err
			}
			var symsz int
			if f.Magic == Magic64 {
				symsz = 16
			} else {
				symsz = 12
			}
			symdat, err := saferio.ReadDataAt(r, uint64(hdr.Nsyms)*uint64(symsz), int64(hdr.Symoff))
			if err != nil {
				return nil, err
			}
			st, err := f.parseSymtab(symdat, strtab, cmddat, &hdr, offset)
			if err != nil {
				return nil, err
			}
			f.Loads = append(f.Loads, st)
			f.Symtab = st

		case LoadCmdDysymtab:
			var hdr DysymtabCmd
			b := bytes.NewReader(cmddat)
			if err := binary.Read(b, bo, &hdr); err != nil {
				return nil, err
			}
			if f.Symtab == nil {
				return nil, &FormatError{offset, "dynamic symbol table seen before any ordinary symbol table", nil}
			} else if hdr.Iundefsym > uint32(len(f.Symtab.Syms)) {
				return nil, &FormatError{offset, fmt.Sprintf(
					"undefined symbols index in dynamic symbol table command is greater than symbol table length (%d > %d)",
					hdr.Iundefsym, len(f.Symtab.Syms)), nil}
			} else if hdr.Iundefsym+hdr.Nundefsym > uint32(len(f.Symtab.Syms)) {
				return nil, &FormatError{offset, fmt.Sprintf(
					"number of undefined symbols after index in dynamic symbol table command is greater than symbol table length (%d > %d)",
					hdr.Iundefsym+hdr.Nundefsym, len(f.Symtab.Syms)), nil}
			}
			dat, err := saferio.ReadDataAt(r, uint64(hdr.Nindirectsyms)*4, int64(hdr.Indirectsymoff))
			if err != nil {
				return nil, err
			}
			x := make([]uint32, hdr.Nindirectsyms)
			if err := binary.Read(bytes.NewReader(dat), bo, x); err != nil {
				return nil, err
			}
			st := new(Dysymtab)
			st.LoadBytes = LoadBytes(cmddat)
			st.DysymtabCmd = hdr
			st.IndirectSyms = x
			f.Loads = append(f.Loads, st)
			f.Dysymtab = st

		case LoadCmdSegment:
			var seg32 Segment32
			b := bytes.NewReader(cmddat)
			if err := binary.Read(b, bo, &seg32); err != nil {
				return nil, err
			}
			s = new(Segment)
			s.LoadBytes = cmddat
			s.Cmd = cmd
			s.Len = siz
			s.Name = cstring(seg32.Name[0:])
			s.Addr = uint64(seg32.Addr)
			s.Memsz = uint64(seg32.Memsz)
			s.Offset = uint64(seg32.Offset)
			s.Filesz = uint64(seg32.Filesz)
			s.Maxprot = seg32.Maxprot
			s.Prot = seg32.Prot
			s.Nsect = seg32.Nsect
			s.Flag = seg32.Flag
			f.Loads = append(f.Loads, s)
			for i := 0; i < int(s.Nsect); i++ {
				var sh32 Section32
				if err := binary.Read(b, bo, &sh32); err != nil {
					return nil, err
				}
				sh := new(Section)
				sh.Name = cstring(sh32.Name[0:])
				sh.Seg = cstring(sh32.Seg[0:])
				sh.Addr = uint64(sh32.Addr)
				sh.Size = uint64(sh32.Size)
				sh.Offset = sh32.Offset
				sh.Align = sh32.Align
				sh.Reloff = sh32.Reloff
				sh.Nreloc = sh32.Nreloc
				sh.Flags = sh32.Flags
				if err := f.pushSection(sh, r); err != nil {
					return nil, err
				}
			}

		case LoadCmdSegment64:
			var seg64 Segment64
			b := bytes.NewReader(cmddat)
			if err := binary.Read(b, bo, &seg64); err != nil {
				return nil, err
			}
			s = new(Segment)
			s.LoadBytes = cmddat
			s.Cmd = cmd
			s.Len = siz
			s.Name = cstring(seg64.Name[0:])
			s.Addr = seg64.Addr
			s.Memsz = seg64.Memsz
			s.Offset = seg64.Offset
			s.Filesz = seg64.Filesz
			s.Maxprot = seg64.Maxprot
			s.Prot = seg64.Prot
			s.Nsect = seg64.Nsect
			s.Flag = seg64.Flag
			f.Loads = append(f.Loads, s)
			for i := 0; i < int(s.Nsect); i++ {
				var sh64 Section64
				if err := binary.Read(b, bo, &sh64); err != nil {
					return nil, err
				}
				sh := new(Section)
				sh.Name = cstring(sh64.Name[0:])
				sh.Seg = cstring(sh64.Seg[0:])
				sh.Addr = sh64.Addr
				sh.Size = sh64.Size
				sh.Offset = sh64.Offset
				sh.Align = sh64.Align
				sh.Reloff = sh64.Reloff
				sh.Nreloc = sh64.Nreloc
				sh.Flags = sh64.Flags
				if err := f.pushSection(sh, r); err != nil {
					return nil, err
				}
			}
		}
		if s != nil {
			if int64(s.Offset) < 0 {
				return nil, &FormatError{offset, "invalid section offset", s.Offset}
			}
			if int64(s.Filesz) < 0 {
				return nil, &FormatError{offset, "invalid section file size", s.Filesz}
			}
			s.sr = io.NewSectionReader(r, int64(s.Offset), int64(s.Filesz))
			s.ReaderAt = s.sr
		}
	}
	return f, nil
}

func (f *File) parseSymtab(symdat, strtab, cmddat []byte, hdr *SymtabCmd, offset int64) (*Symtab, error) {
	bo := f.ByteOrder
	c := saferio.SliceCap[Symbol](uint64(hdr.Nsyms))
	if c < 0 {
		return nil, &FormatError{offset, "too many symbols", nil}
	}
	symtab := make([]Symbol, 0, c)
	b := bytes.NewReader(symdat)
	for i := 0; i < int(hdr.Nsyms); i++ {
		var n Nlist64
		if f.Magic == Magic64 {
			if err := binary.Read(b, bo, &n); err != nil {
				return nil, err
			}
		} else {
			var n32 Nlist32
			if err := binary.Read(b, bo, &n32); err != nil {
				return nil, err
			}
			n.Name = n32.Name
			n.Type = n32.Type
			n.Sect = n32.Sect
			n.Desc = n32.Desc
			n.Value = uint64(n32.Value)
		}
		if n.Name >= uint32(len(strtab)) {
			return nil, &FormatError{offset, "invalid name in symbol table", n.Name}
		}
		// We add "_" to Go symbols. Strip it here. See issue 33808.
		name := cstring(strtab[n.Name:])
		if strings.Contains(name, ".") && name[0] == '_' {
			name = name[1:]
		}
		symtab = append(symtab, Symbol{
			Name:  name,
			Type:  n.Type,
			Sect:  n.Sect,
			Desc:  n.Desc,
			Value: n.Value,
		})
	}
	st := new(Symtab)
	st.LoadBytes = LoadBytes(cmddat)
	st.Syms = symtab
	return st, nil
}

type relocInfo struct {
	Addr   uint32
	Symnum uint32
}

func (f *File) pushSection(sh *Section, r io.ReaderAt) error {
	f.Sections = append(f.Sections, sh)
	sh.sr = io.NewSectionReader(r, int64(sh.Offset), int64(sh.Size))
	sh.ReaderAt = sh.sr

	if sh.Nreloc > 0 {
		reldat, err := saferio.ReadDataAt(r, uint64(sh.Nreloc)*8, int64(sh.Reloff))
		if err != nil {
			return err
		}
		b := bytes.NewReader(reldat)

		bo := f.ByteOrder

		sh.Relocs = make([]Reloc, sh.Nreloc)
		for i := range sh.Relocs {
			rel := &sh.Relocs[i]

			var ri relocInfo
			if err := binary.Read(b, bo, &ri); err != nil {
				return err
			}

			if ri.Addr&(1<<31) != 0 { // scattered
				rel.Addr = ri.Addr & (1<<24 - 1)
				rel.Type = uint8((ri.Addr >> 24) & (1<<4 - 1))
				rel.Len = uint8((ri.Addr >> 28) & (1<<2 - 1))
				rel.Pcrel = ri.Addr&(1<<30) != 0
				rel.Value = ri.Symnum
				rel.Scattered = true
			} else {
				switch bo {
				case binary.LittleEndian:
					rel.Addr = ri.Addr
					rel.Value = ri.Symnum & (1<<24 - 1)
					rel.Pcrel = ri.Symnum&(1<<24) != 0
					rel.Len = uint8((ri.Symnum >> 25) & (1<<2 - 1))
					rel.Extern = ri.Symnum&(1<<27) != 0
					rel.Type = uint8((ri.Symnum >> 28) & (1<<4 - 1))
				case binary.BigEndian:
					rel.Addr = ri.Addr
					rel.Value = ri.Symnum >> 8
					rel.Pcrel = ri.Symnum&(1<<7) != 0
					rel.Len = uint8((ri.Symnum >> 5) & (1<<2 - 1))
					rel.Extern = ri.Symnum&(1<<4) != 0
					rel.Type = uint8(ri.Symnum & (1<<4 - 1))
				default:
					panic("unreachable")
				}
			}
		}
	}

	return nil
}

func cstring(b []byte) string {
	i := bytes.IndexByte(b, 0)
	if i == -1 {
		i = len(b)
	}
	return string(b[0:i])
}

// Segment returns the first Segment with the given name, or nil if no such segment exists.
func (f *File) Segment(name string) *Segment {
	for _, l := range f.Loads {
		if s, ok := l.(*Segment); ok && s.Name == name {
			return s
		}
	}
	return nil
}

// Section returns the first section with the given name, or nil if no such
// section exists.
func (f *File) Section(name string) *Section {
	for _, s := range f.Sections {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// DWARF returns the DWARF debug information for the Mach-O file.
func (f *File) DWARF() (*dwarf.Data, error) {
	dwarfSuffix := func(s *Section) string {
		sectname := s.Name
		var pfx int
		switch {
		case strings.HasPrefix(sectname, "__debug_"):
			pfx = 8
		case strings.HasPrefix(sectname, "__zdebug_"):
			pfx = 9
		default:
			return ""
		}
		// Mach-O executables truncate section names to 16 characters, mangling some DWARF sections.
		// As of DWARFv5 these are the only problematic section names (see DWARFv5 Appendix G).
		for _, longname := range []string{
			"__debug_str_offsets",
			"__zdebug_line_str",
			"__zdebug_loclists",
			"__zdebug_pubnames",
			"__zdebug_pubtypes",
			"__zdebug_rnglists",
			"__zdebug_str_offsets",
		} {
			if sectname == longname[:16] {
				sectname = longname
				break
			}
		}
		return sectname[pfx:]
	}
	sectionData := func(s *Section) ([]byte, error) {
		b, err := s.Data()
		if err != nil && uint64(len(b)) < s.Size {
			return nil, err
		}

		if len(b) >= 12 && string(b[:4]) == "ZLIB" {
			dlen := binary.BigEndian.Uint64(b[4:12])
			r, err := zlib.NewReader(bytes.NewBuffer(b[12:]))
			if err != nil {
				return nil, err
			}
			dbuf, err := saferio.ReadData(r, dlen)
			if err != nil {
				return nil, err
			}
			if err := r.Close(); err != nil {
				return nil, err
			}
			b = dbuf
		}
		return b, nil
	}

	// There are many other DWARF sections, but these
	// are the ones the debug/dwarf package uses.
	// Don't bother loading others.
	var dat = map[string][]byte{"abbrev": nil, "info": nil, "str": nil, "line": nil, "ranges": nil}
	for _, s := range f.Sections {
		suffix := dwarfSuffix(s)
		if suffix == "" {
			continue
		}
		if _, ok := dat[suffix]; !ok {
			continue
		}
		b, err := sectionData(s)
		if err != nil {
			return nil, err
		}
		dat[suffix] = b
	}

	d, err := dwarf.New(dat["abbrev"], nil, nil, dat["info"], dat["line"], nil, dat["ranges"], dat["str"])
	if err != nil {
		return nil, err
	}

	// Look for DWARF4 .debug_types sections and DWARF5 sections.
	for i, s := range f.Sections {
		suffix := dwarfSuffix(s)
		if suffix == "" {
			continue
		}
		if _, ok := dat[suffix]; ok {
			// Already handled.
			continue
		}

		b, err := sectionData(s)
		if err != nil {
			return nil, err
		}

		if suffix == "types" {
			err = d.AddTypes(fmt.Sprintf("types-%d", i), b)
		} else {
			err = d.AddSection(".debug_"+suffix, b)
		}
		if err != nil {
			return nil, err
		}
	}

	return d, nil
}

// ImportedSymbols returns the names of all symbols
// referred to by the binary f that are expected to be
// satisfied by other libraries at dynamic load time.
func (f *File) ImportedSymbols() ([]string, error) {
	if f.Symtab == nil {
		return nil, &FormatError{0, "missing symbol table", nil}
	}

	st := f.Symtab
	dt := f.Dysymtab
	var all []string
	if dt != nil {
		for _, s := range st.Syms[dt.Iundefsym : dt.Iundefsym+dt.Nundefsym] {
			all = append(all, s.Name)
		}
	} else {
		// From Darwin's include/mach-o/nlist.h
		const (
			N_TYPE = 0x0e
			N_UNDF = 0x0
			N_EXT  = 0x01
		)
		for _, s := range st.Syms {
			if s.Type&N_TYPE == N_UNDF && s.Type&N_EXT != 0 {
				all = append(all, s.Name)
			}
		}
	}
	return all, nil
}

// ImportedLibraries returns the paths of all libraries
// referred to by the binary f that are expected to be
// linked with the binary at dynamic link time.
func (f *File) ImportedLibraries() ([]string, error) {
	var all []string
	for _, l := range f.Loads {
		if lib, ok := l.(*Dylib); ok {
			all = append(all, lib.Name)
		}
	}
	return all, nil
}

```

// === FILE: references!/go/src/debug/macho/macho.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Mach-O header data structures
// Originally at:
// http://developer.apple.com/mac/library/documentation/DeveloperTools/Conceptual/MachORuntime/Reference/reference.html (since deleted by Apple)
// Archived copy at:
// https://web.archive.org/web/20090819232456/http://developer.apple.com/documentation/DeveloperTools/Conceptual/MachORuntime/index.html
// For cloned PDF see:
// https://github.com/aidansteele/osx-abi-macho-file-format-reference

package macho

import "strconv"

// A FileHeader represents a Mach-O file header.
type FileHeader struct {
	Magic  uint32
	Cpu    Cpu
	SubCpu uint32
	Type   Type
	Ncmd   uint32
	Cmdsz  uint32
	Flags  uint32
}

const (
	fileHeaderSize32 = 7 * 4
	fileHeaderSize64 = 8 * 4
)

const (
	Magic32  uint32 = 0xfeedface
	Magic64  uint32 = 0xfeedfacf
	MagicFat uint32 = 0xcafebabe
)

// A Type is the Mach-O file type, e.g. an object file, executable, or dynamic library.
type Type uint32

const (
	TypeObj    Type = 1
	TypeExec   Type = 2
	TypeDylib  Type = 6
	TypeBundle Type = 8
)

var typeStrings = []intName{
	{uint32(TypeObj), "Obj"},
	{uint32(TypeExec), "Exec"},
	{uint32(TypeDylib), "Dylib"},
	{uint32(TypeBundle), "Bundle"},
}

func (t Type) String() string   { return stringName(uint32(t), typeStrings, false) }
func (t Type) GoString() string { return stringName(uint32(t), typeStrings, true) }

// A Cpu is a Mach-O cpu type.
type Cpu uint32

const cpuArch64 = 0x01000000

const (
	Cpu386   Cpu = 7
	CpuAmd64 Cpu = Cpu386 | cpuArch64
	CpuArm   Cpu = 12
	CpuArm64 Cpu = CpuArm | cpuArch64
	CpuPpc   Cpu = 18
	CpuPpc64 Cpu = CpuPpc | cpuArch64
)

var cpuStrings = []intName{
	{uint32(Cpu386), "Cpu386"},
	{uint32(CpuAmd64), "CpuAmd64"},
	{uint32(CpuArm), "CpuArm"},
	{uint32(CpuArm64), "CpuArm64"},
	{uint32(CpuPpc), "CpuPpc"},
	{uint32(CpuPpc64), "CpuPpc64"},
}

func (i Cpu) String() string   { return stringName(uint32(i), cpuStrings, false) }
func (i Cpu) GoString() string { return stringName(uint32(i), cpuStrings, true) }

// A LoadCmd is a Mach-O load command.
type LoadCmd uint32

const (
	LoadCmdSegment    LoadCmd = 0x1
	LoadCmdSymtab     LoadCmd = 0x2
	LoadCmdThread     LoadCmd = 0x4
	LoadCmdUnixThread LoadCmd = 0x5 // thread+stack
	LoadCmdDysymtab   LoadCmd = 0xb
	LoadCmdDylib      LoadCmd = 0xc // load dylib command
	LoadCmdDylinker   LoadCmd = 0xf // id dylinker command (not load dylinker command)
	LoadCmdSegment64  LoadCmd = 0x19
	LoadCmdRpath      LoadCmd = 0x8000001c
)

var cmdStrings = []intName{
	{uint32(LoadCmdSegment), "LoadCmdSegment"},
	{uint32(LoadCmdThread), "LoadCmdThread"},
	{uint32(LoadCmdUnixThread), "LoadCmdUnixThread"},
	{uint32(LoadCmdDylib), "LoadCmdDylib"},
	{uint32(LoadCmdSegment64), "LoadCmdSegment64"},
	{uint32(LoadCmdRpath), "LoadCmdRpath"},
}

func (i LoadCmd) String() string   { return stringName(uint32(i), cmdStrings, false) }
func (i LoadCmd) GoString() string { return stringName(uint32(i), cmdStrings, true) }

type (
	// A Segment32 is a 32-bit Mach-O segment load command.
	Segment32 struct {
		Cmd     LoadCmd
		Len     uint32
		Name    [16]byte
		Addr    uint32
		Memsz   uint32
		Offset  uint32
		Filesz  uint32
		Maxprot uint32
		Prot    uint32
		Nsect   uint32
		Flag    uint32
	}

	// A Segment64 is a 64-bit Mach-O segment load command.
	Segment64 struct {
		Cmd     LoadCmd
		Len     uint32
		Name    [16]byte
		Addr    uint64
		Memsz   uint64
		Offset  uint64
		Filesz  uint64
		Maxprot uint32
		Prot    uint32
		Nsect   uint32
		Flag    uint32
	}

	// A SymtabCmd is a Mach-O symbol table command.
	SymtabCmd struct {
		Cmd     LoadCmd
		Len     uint32
		Symoff  uint32
		Nsyms   uint32
		Stroff  uint32
		Strsize uint32
	}

	// A DysymtabCmd is a Mach-O dynamic symbol table command.
	DysymtabCmd struct {
		Cmd            LoadCmd
		Len            uint32
		Ilocalsym      uint32
		Nlocalsym      uint32
		Iextdefsym     uint32
		Nextdefsym     uint32
		Iundefsym      uint32
		Nundefsym      uint32
		Tocoffset      uint32
		Ntoc           uint32
		Modtaboff      uint32
		Nmodtab        uint32
		Extrefsymoff   uint32
		Nextrefsyms    uint32
		Indirectsymoff uint32
		Nindirectsyms  uint32
		Extreloff      uint32
		Nextrel        uint32
		Locreloff      uint32
		Nlocrel        uint32
	}

	// A DylibCmd is a Mach-O load dynamic library command.
	DylibCmd struct {
		Cmd            LoadCmd
		Len            uint32
		Name           uint32
		Time           uint32
		CurrentVersion uint32
		CompatVersion  uint32
	}

	// A RpathCmd is a Mach-O rpath command.
	RpathCmd struct {
		Cmd  LoadCmd
		Len  uint32
		Path uint32
	}

	// A Thread is a Mach-O thread state command.
	Thread struct {
		Cmd  LoadCmd
		Len  uint32
		Type uint32
		Data []uint32
	}
)

const (
	FlagNoUndefs              uint32 = 0x1
	FlagIncrLink              uint32 = 0x2
	FlagDyldLink              uint32 = 0x4
	FlagBindAtLoad            uint32 = 0x8
	FlagPrebound              uint32 = 0x10
	FlagSplitSegs             uint32 = 0x20
	FlagLazyInit              uint32 = 0x40
	FlagTwoLevel              uint32 = 0x80
	FlagForceFlat             uint32 = 0x100
	FlagNoMultiDefs           uint32 = 0x200
	FlagNoFixPrebinding       uint32 = 0x400
	FlagPrebindable           uint32 = 0x800
	FlagAllModsBound          uint32 = 0x1000
	FlagSubsectionsViaSymbols uint32 = 0x2000
	FlagCanonical             uint32 = 0x4000
	FlagWeakDefines           uint32 = 0x8000
	FlagBindsToWeak           uint32 = 0x10000
	FlagAllowStackExecution   uint32 = 0x20000
	FlagRootSafe              uint32 = 0x40000
	FlagSetuidSafe            uint32 = 0x80000
	FlagNoReexportedDylibs    uint32 = 0x100000
	FlagPIE                   uint32 = 0x200000
	FlagDeadStrippableDylib   uint32 = 0x400000
	FlagHasTLVDescriptors     uint32 = 0x800000
	FlagNoHeapExecution       uint32 = 0x1000000
	FlagAppExtensionSafe      uint32 = 0x2000000
)

// A Section32 is a 32-bit Mach-O section header.
type Section32 struct {
	Name     [16]byte
	Seg      [16]byte
	Addr     uint32
	Size     uint32
	Offset   uint32
	Align    uint32
	Reloff   uint32
	Nreloc   uint32
	Flags    uint32
	Reserve1 uint32
	Reserve2 uint32
}

// A Section64 is a 64-bit Mach-O section header.
type Section64 struct {
	Name     [16]byte
	Seg      [16]byte
	Addr     uint64
	Size     uint64
	Offset   uint32
	Align    uint32
	Reloff   uint32
	Nreloc   uint32
	Flags    uint32
	Reserve1 uint32
	Reserve2 uint32
	Reserve3 uint32
}

// An Nlist32 is a Mach-O 32-bit symbol table entry.
type Nlist32 struct {
	Name  uint32
	Type  uint8
	Sect  uint8
	Desc  uint16
	Value uint32
}

// An Nlist64 is a Mach-O 64-bit symbol table entry.
type Nlist64 struct {
	Name  uint32
	Type  uint8
	Sect  uint8
	Desc  uint16
	Value uint64
}

// Regs386 is the Mach-O 386 register structure.
type Regs386 struct {
	AX    uint32
	BX    uint32
	CX    uint32
	DX    uint32
	DI    uint32
	SI    uint32
	BP    uint32
	SP    uint32
	SS    uint32
	FLAGS uint32
	IP    uint32
	CS    uint32
	DS    uint32
	ES    uint32
	FS    uint32
	GS    uint32
}

// RegsAMD64 is the Mach-O AMD64 register structure.
type RegsAMD64 struct {
	AX    uint64
	BX    uint64
	CX    uint64
	DX    uint64
	DI    uint64
	SI    uint64
	BP    uint64
	SP    uint64
	R8    uint64
	R9    uint64
	R10   uint64
	R11   uint64
	R12   uint64
	R13   uint64
	R14   uint64
	R15   uint64
	IP    uint64
	FLAGS uint64
	CS    uint64
	FS    uint64
	GS    uint64
}

type intName struct {
	i uint32
	s string
}

func stringName(i uint32, names []intName, goSyntax bool) string {
	for _, n := range names {
		if n.i == i {
			if goSyntax {
				return "macho." + n.s
			}
			return n.s
		}
	}
	return strconv.FormatUint(uint64(i), 10)
}

```

// === FILE: references!/go/src/debug/macho/reloctype.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package macho

//go:generate stringer -type=RelocTypeGeneric,RelocTypeX86_64,RelocTypeARM,RelocTypeARM64 -output reloctype_string.go

type RelocTypeGeneric int

const (
	GENERIC_RELOC_VANILLA        RelocTypeGeneric = 0
	GENERIC_RELOC_PAIR           RelocTypeGeneric = 1
	GENERIC_RELOC_SECTDIFF       RelocTypeGeneric = 2
	GENERIC_RELOC_PB_LA_PTR      RelocTypeGeneric = 3
	GENERIC_RELOC_LOCAL_SECTDIFF RelocTypeGeneric = 4
	GENERIC_RELOC_TLV            RelocTypeGeneric = 5
)

func (r RelocTypeGeneric) GoString() string { return "macho." + r.String() }

type RelocTypeX86_64 int

const (
	X86_64_RELOC_UNSIGNED   RelocTypeX86_64 = 0
	X86_64_RELOC_SIGNED     RelocTypeX86_64 = 1
	X86_64_RELOC_BRANCH     RelocTypeX86_64 = 2
	X86_64_RELOC_GOT_LOAD   RelocTypeX86_64 = 3
	X86_64_RELOC_GOT        RelocTypeX86_64 = 4
	X86_64_RELOC_SUBTRACTOR RelocTypeX86_64 = 5
	X86_64_RELOC_SIGNED_1   RelocTypeX86_64 = 6
	X86_64_RELOC_SIGNED_2   RelocTypeX86_64 = 7
	X86_64_RELOC_SIGNED_4   RelocTypeX86_64 = 8
	X86_64_RELOC_TLV        RelocTypeX86_64 = 9
)

func (r RelocTypeX86_64) GoString() string { return "macho." + r.String() }

type RelocTypeARM int

const (
	ARM_RELOC_VANILLA        RelocTypeARM = 0
	ARM_RELOC_PAIR           RelocTypeARM = 1
	ARM_RELOC_SECTDIFF       RelocTypeARM = 2
	ARM_RELOC_LOCAL_SECTDIFF RelocTypeARM = 3
	ARM_RELOC_PB_LA_PTR      RelocTypeARM = 4
	ARM_RELOC_BR24           RelocTypeARM = 5
	ARM_THUMB_RELOC_BR22     RelocTypeARM = 6
	ARM_THUMB_32BIT_BRANCH   RelocTypeARM = 7
	ARM_RELOC_HALF           RelocTypeARM = 8
	ARM_RELOC_HALF_SECTDIFF  RelocTypeARM = 9
)

func (r RelocTypeARM) GoString() string { return "macho." + r.String() }

type RelocTypeARM64 int

const (
	ARM64_RELOC_UNSIGNED            RelocTypeARM64 = 0
	ARM64_RELOC_SUBTRACTOR          RelocTypeARM64 = 1
	ARM64_RELOC_BRANCH26            RelocTypeARM64 = 2
	ARM64_RELOC_PAGE21              RelocTypeARM64 = 3
	ARM64_RELOC_PAGEOFF12           RelocTypeARM64 = 4
	ARM64_RELOC_GOT_LOAD_PAGE21     RelocTypeARM64 = 5
	ARM64_RELOC_GOT_LOAD_PAGEOFF12  RelocTypeARM64 = 6
	ARM64_RELOC_POINTER_TO_GOT      RelocTypeARM64 = 7
	ARM64_RELOC_TLVP_LOAD_PAGE21    RelocTypeARM64 = 8
	ARM64_RELOC_TLVP_LOAD_PAGEOFF12 RelocTypeARM64 = 9
	ARM64_RELOC_ADDEND              RelocTypeARM64 = 10
)

func (r RelocTypeARM64) GoString() string { return "macho." + r.String() }

```

// === FILE: references!/go/src/debug/macho/reloctype_string.go ===
```go
// Code generated by "stringer -type=RelocTypeGeneric,RelocTypeX86_64,RelocTypeARM,RelocTypeARM64 -output reloctype_string.go"; DO NOT EDIT.

package macho

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[GENERIC_RELOC_VANILLA-0]
	_ = x[GENERIC_RELOC_PAIR-1]
	_ = x[GENERIC_RELOC_SECTDIFF-2]
	_ = x[GENERIC_RELOC_PB_LA_PTR-3]
	_ = x[GENERIC_RELOC_LOCAL_SECTDIFF-4]
	_ = x[GENERIC_RELOC_TLV-5]
}

const _RelocTypeGeneric_name = "GENERIC_RELOC_VANILLAGENERIC_RELOC_PAIRGENERIC_RELOC_SECTDIFFGENERIC_RELOC_PB_LA_PTRGENERIC_RELOC_LOCAL_SECTDIFFGENERIC_RELOC_TLV"

var _RelocTypeGeneric_index = [...]uint8{0, 21, 39, 61, 84, 112, 129}

func (i RelocTypeGeneric) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_RelocTypeGeneric_index)-1 {
		return "RelocTypeGeneric(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _RelocTypeGeneric_name[_RelocTypeGeneric_index[idx]:_RelocTypeGeneric_index[idx+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[X86_64_RELOC_UNSIGNED-0]
	_ = x[X86_64_RELOC_SIGNED-1]
	_ = x[X86_64_RELOC_BRANCH-2]
	_ = x[X86_64_RELOC_GOT_LOAD-3]
	_ = x[X86_64_RELOC_GOT-4]
	_ = x[X86_64_RELOC_SUBTRACTOR-5]
	_ = x[X86_64_RELOC_SIGNED_1-6]
	_ = x[X86_64_RELOC_SIGNED_2-7]
	_ = x[X86_64_RELOC_SIGNED_4-8]
	_ = x[X86_64_RELOC_TLV-9]
}

const _RelocTypeX86_64_name = "X86_64_RELOC_UNSIGNEDX86_64_RELOC_SIGNEDX86_64_RELOC_BRANCHX86_64_RELOC_GOT_LOADX86_64_RELOC_GOTX86_64_RELOC_SUBTRACTORX86_64_RELOC_SIGNED_1X86_64_RELOC_SIGNED_2X86_64_RELOC_SIGNED_4X86_64_RELOC_TLV"

var _RelocTypeX86_64_index = [...]uint8{0, 21, 40, 59, 80, 96, 119, 140, 161, 182, 198}

func (i RelocTypeX86_64) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_RelocTypeX86_64_index)-1 {
		return "RelocTypeX86_64(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _RelocTypeX86_64_name[_RelocTypeX86_64_index[idx]:_RelocTypeX86_64_index[idx+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ARM_RELOC_VANILLA-0]
	_ = x[ARM_RELOC_PAIR-1]
	_ = x[ARM_RELOC_SECTDIFF-2]
	_ = x[ARM_RELOC_LOCAL_SECTDIFF-3]
	_ = x[ARM_RELOC_PB_LA_PTR-4]
	_ = x[ARM_RELOC_BR24-5]
	_ = x[ARM_THUMB_RELOC_BR22-6]
	_ = x[ARM_THUMB_32BIT_BRANCH-7]
	_ = x[ARM_RELOC_HALF-8]
	_ = x[ARM_RELOC_HALF_SECTDIFF-9]
}

const _RelocTypeARM_name = "ARM_RELOC_VANILLAARM_RELOC_PAIRARM_RELOC_SECTDIFFARM_RELOC_LOCAL_SECTDIFFARM_RELOC_PB_LA_PTRARM_RELOC_BR24ARM_THUMB_RELOC_BR22ARM_THUMB_32BIT_BRANCHARM_RELOC_HALFARM_RELOC_HALF_SECTDIFF"

var _RelocTypeARM_index = [...]uint8{0, 17, 31, 49, 73, 92, 106, 126, 148, 162, 185}

func (i RelocTypeARM) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_RelocTypeARM_index)-1 {
		return "RelocTypeARM(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _RelocTypeARM_name[_RelocTypeARM_index[idx]:_RelocTypeARM_index[idx+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ARM64_RELOC_UNSIGNED-0]
	_ = x[ARM64_RELOC_SUBTRACTOR-1]
	_ = x[ARM64_RELOC_BRANCH26-2]
	_ = x[ARM64_RELOC_PAGE21-3]
	_ = x[ARM64_RELOC_PAGEOFF12-4]
	_ = x[ARM64_RELOC_GOT_LOAD_PAGE21-5]
	_ = x[ARM64_RELOC_GOT_LOAD_PAGEOFF12-6]
	_ = x[ARM64_RELOC_POINTER_TO_GOT-7]
	_ = x[ARM64_RELOC_TLVP_LOAD_PAGE21-8]
	_ = x[ARM64_RELOC_TLVP_LOAD_PAGEOFF12-9]
	_ = x[ARM64_RELOC_ADDEND-10]
}

const _RelocTypeARM64_name = "ARM64_RELOC_UNSIGNEDARM64_RELOC_SUBTRACTORARM64_RELOC_BRANCH26ARM64_RELOC_PAGE21ARM64_RELOC_PAGEOFF12ARM64_RELOC_GOT_LOAD_PAGE21ARM64_RELOC_GOT_LOAD_PAGEOFF12ARM64_RELOC_POINTER_TO_GOTARM64_RELOC_TLVP_LOAD_PAGE21ARM64_RELOC_TLVP_LOAD_PAGEOFF12ARM64_RELOC_ADDEND"

var _RelocTypeARM64_index = [...]uint16{0, 20, 42, 62, 80, 101, 128, 158, 184, 212, 243, 261}

func (i RelocTypeARM64) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_RelocTypeARM64_index)-1 {
		return "RelocTypeARM64(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _RelocTypeARM64_name[_RelocTypeARM64_index[idx]:_RelocTypeARM64_index[idx+1]]
}

```

// === FILE: references!/go/src/debug/pe/file.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package pe implements access to PE (Microsoft Windows Portable Executable) files.

# Security

This package is not designed to be hardened against adversarial inputs, and is
outside the scope of https://go.dev/security/policy. In particular, only basic
validation is done when parsing object files. As such, care should be taken when
parsing untrusted inputs, as parsing malformed files may consume significant
resources, or cause panics.
*/
package pe

import (
	"bytes"
	"compress/zlib"
	"debug/dwarf"
	"encoding/binary"
	"errors"
	"fmt"
	"internal/saferio"
	"io"
	"os"
	"strings"
)

// A File represents an open PE file.
type File struct {
	FileHeader
	OptionalHeader any // of type *OptionalHeader32 or *OptionalHeader64
	Sections       []*Section
	Symbols        []*Symbol    // COFF symbols with auxiliary symbol records removed
	COFFSymbols    []COFFSymbol // all COFF symbols (including auxiliary symbol records)
	StringTable    StringTable

	closer io.Closer
}

// Open opens the named file using [os.Open] and prepares it for use as a PE binary.
func Open(name string) (*File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	ff, err := NewFile(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	ff.closer = f
	return ff, nil
}

// Close closes the [File].
// If the [File] was created using [NewFile] directly instead of [Open],
// Close has no effect.
func (f *File) Close() error {
	var err error
	if f.closer != nil {
		err = f.closer.Close()
		f.closer = nil
	}
	return err
}

// TODO(brainman): add Load function, as a replacement for NewFile, that does not call removeAuxSymbols (for performance)

// NewFile creates a new [File] for accessing a PE binary in an underlying reader.
func NewFile(r io.ReaderAt) (*File, error) {
	f := new(File)
	sr := io.NewSectionReader(r, 0, 1<<63-1)

	var dosheader [96]byte
	if _, err := r.ReadAt(dosheader[0:], 0); err != nil {
		return nil, err
	}
	var base int64
	if dosheader[0] == 'M' && dosheader[1] == 'Z' {
		signoff := int64(binary.LittleEndian.Uint32(dosheader[0x3c:]))
		var sign [4]byte
		if _, err := r.ReadAt(sign[:], signoff); err != nil {
			return nil, err
		}
		if !(sign[0] == 'P' && sign[1] == 'E' && sign[2] == 0 && sign[3] == 0) {
			return nil, fmt.Errorf("invalid PE file signature: % x", sign)
		}
		base = signoff + 4
	} else {
		base = int64(0)
	}
	sr.Seek(base, io.SeekStart)
	if err := binary.Read(sr, binary.LittleEndian, &f.FileHeader); err != nil {
		return nil, err
	}
	switch f.FileHeader.Machine {
	case IMAGE_FILE_MACHINE_AMD64,
		IMAGE_FILE_MACHINE_ARM64,
		IMAGE_FILE_MACHINE_ARMNT,
		IMAGE_FILE_MACHINE_I386,
		IMAGE_FILE_MACHINE_RISCV32,
		IMAGE_FILE_MACHINE_RISCV64,
		IMAGE_FILE_MACHINE_RISCV128,
		IMAGE_FILE_MACHINE_UNKNOWN:
		// ok
	default:
		return nil, fmt.Errorf("unrecognized PE machine: %#x", f.FileHeader.Machine)
	}

	var err error

	// Read string table.
	f.StringTable, err = readStringTable(&f.FileHeader, sr)
	if err != nil {
		return nil, err
	}

	// Read symbol table.
	f.COFFSymbols, err = readCOFFSymbols(&f.FileHeader, sr)
	if err != nil {
		return nil, err
	}
	f.Symbols, err = removeAuxSymbols(f.COFFSymbols, f.StringTable)
	if err != nil {
		return nil, err
	}

	// Seek past file header.
	_, err = sr.Seek(base+int64(binary.Size(f.FileHeader)), io.SeekStart)
	if err != nil {
		return nil, err
	}

	// Read optional header.
	f.OptionalHeader, err = readOptionalHeader(sr, f.FileHeader.SizeOfOptionalHeader)
	if err != nil {
		return nil, err
	}

	// Process sections.
	f.Sections = make([]*Section, f.FileHeader.NumberOfSections)
	for i := 0; i < int(f.FileHeader.NumberOfSections); i++ {
		sh := new(SectionHeader32)
		if err := binary.Read(sr, binary.LittleEndian, sh); err != nil {
			return nil, err
		}
		name, err := sh.fullName(f.StringTable)
		if err != nil {
			return nil, err
		}
		s := new(Section)
		s.SectionHeader = SectionHeader{
			Name:                 name,
			VirtualSize:          sh.VirtualSize,
			VirtualAddress:       sh.VirtualAddress,
			Size:                 sh.SizeOfRawData,
			Offset:               sh.PointerToRawData,
			PointerToRelocations: sh.PointerToRelocations,
			PointerToLineNumbers: sh.PointerToLineNumbers,
			NumberOfRelocations:  sh.NumberOfRelocations,
			NumberOfLineNumbers:  sh.NumberOfLineNumbers,
			Characteristics:      sh.Characteristics,
		}
		r2 := r
		if sh.PointerToRawData == 0 { // .bss must have all 0s
			r2 = &nobitsSectionReader{}
		}
		s.sr = io.NewSectionReader(r2, int64(s.SectionHeader.Offset), int64(s.SectionHeader.Size))
		s.ReaderAt = s.sr
		f.Sections[i] = s
	}
	for i := range f.Sections {
		var err error
		f.Sections[i].Relocs, err = readRelocs(&f.Sections[i].SectionHeader, sr)
		if err != nil {
			return nil, err
		}
	}

	return f, nil
}

type nobitsSectionReader struct{}

func (*nobitsSectionReader) ReadAt(p []byte, off int64) (n int, err error) {
	return 0, errors.New("unexpected read from section with uninitialized data")
}

// getString extracts a string from symbol string table.
func getString(section []byte, start int) (string, bool) {
	if start < 0 || start >= len(section) {
		return "", false
	}

	for end := start; end < len(section); end++ {
		if section[end] == 0 {
			return string(section[start:end]), true
		}
	}
	return "", false
}

// Section returns the first section with the given name, or nil if no such
// section exists.
func (f *File) Section(name string) *Section {
	for _, s := range f.Sections {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func (f *File) DWARF() (*dwarf.Data, error) {
	dwarfSuffix := func(s *Section) string {
		switch {
		case strings.HasPrefix(s.Name, ".debug_"):
			return s.Name[7:]
		case strings.HasPrefix(s.Name, ".zdebug_"):
			return s.Name[8:]
		default:
			return ""
		}

	}

	// sectionData gets the data for s and checks its size.
	sectionData := func(s *Section) ([]byte, error) {
		b, err := s.Data()
		if err != nil && uint32(len(b)) < s.Size {
			return nil, err
		}

		if 0 < s.VirtualSize && s.VirtualSize < s.Size {
			b = b[:s.VirtualSize]
		}

		if len(b) >= 12 && string(b[:4]) == "ZLIB" {
			dlen := binary.BigEndian.Uint64(b[4:12])
			r, err := zlib.NewReader(bytes.NewBuffer(b[12:]))
			if err != nil {
				return nil, err
			}
			dbuf, err := saferio.ReadData(r, dlen)
			if err != nil {
				return nil, err
			}
			if err := r.Close(); err != nil {
				return nil, err
			}
			b = dbuf
		}
		return b, nil
	}

	// There are many other DWARF sections, but these
	// are the ones the debug/dwarf package uses.
	// Don't bother loading others.
	var dat = map[string][]byte{"abbrev": nil, "info": nil, "str": nil, "line": nil, "ranges": nil}
	for _, s := range f.Sections {
		suffix := dwarfSuffix(s)
		if suffix == "" {
			continue
		}
		if _, ok := dat[suffix]; !ok {
			continue
		}

		b, err := sectionData(s)
		if err != nil {
			return nil, err
		}
		dat[suffix] = b
	}

	d, err := dwarf.New(dat["abbrev"], nil, nil, dat["info"], dat["line"], nil, dat["ranges"], dat["str"])
	if err != nil {
		return nil, err
	}

	// Look for DWARF4 .debug_types sections and DWARF5 sections.
	for i, s := range f.Sections {
		suffix := dwarfSuffix(s)
		if suffix == "" {
			continue
		}
		if _, ok := dat[suffix]; ok {
			// Already handled.
			continue
		}

		b, err := sectionData(s)
		if err != nil {
			return nil, err
		}

		if suffix == "types" {
			err = d.AddTypes(fmt.Sprintf("types-%d", i), b)
		} else {
			err = d.AddSection(".debug_"+suffix, b)
		}
		if err != nil {
			return nil, err
		}
	}

	return d, nil
}

// TODO(brainman): document ImportDirectory once we decide what to do with it.

type ImportDirectory struct {
	OriginalFirstThunk uint32
	TimeDateStamp      uint32
	ForwarderChain     uint32
	Name               uint32
	FirstThunk         uint32

	dll string
}

// ImportedSymbols returns the names of all symbols
// referred to by the binary f that are expected to be
// satisfied by other libraries at dynamic load time.
// It does not return weak symbols.
func (f *File) ImportedSymbols() ([]string, error) {
	if f.OptionalHeader == nil {
		return nil, nil
	}

	_, pe64 := f.OptionalHeader.(*OptionalHeader64)

	// grab the number of data directory entries
	var dd_length uint32
	if pe64 {
		dd_length = f.OptionalHeader.(*OptionalHeader64).NumberOfRvaAndSizes
	} else {
		dd_length = f.OptionalHeader.(*OptionalHeader32).NumberOfRvaAndSizes
	}

	// check that the length of data directory entries is large
	// enough to include the imports directory.
	if dd_length < IMAGE_DIRECTORY_ENTRY_IMPORT+1 {
		return nil, nil
	}

	// grab the import data directory entry
	var idd DataDirectory
	if pe64 {
		idd = f.OptionalHeader.(*OptionalHeader64).DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT]
	} else {
		idd = f.OptionalHeader.(*OptionalHeader32).DataDirectory[IMAGE_DIRECTORY_ENTRY_IMPORT]
	}

	// figure out which section contains the import directory table
	var ds *Section
	ds = nil
	for _, s := range f.Sections {
		if s.Offset == 0 {
			continue
		}
		// We are using distance between s.VirtualAddress and idd.VirtualAddress
		// to avoid potential overflow of uint32 caused by addition of s.VirtualSize
		// to s.VirtualAddress.
		if s.VirtualAddress <= idd.VirtualAddress && idd.VirtualAddress-s.VirtualAddress < s.VirtualSize {
			ds = s
			break
		}
	}

	// didn't find a section, so no import libraries were found
	if ds == nil {
		return nil, nil
	}

	d, err := ds.Data()
	if err != nil {
		return nil, err
	}

	// seek to the virtual address specified in the import data directory
	seek := idd.VirtualAddress - ds.VirtualAddress
	if seek >= uint32(len(d)) {
		return nil, errors.New("optional header data directory virtual size doesn't fit within data seek")
	}
	d = d[seek:]

	// start decoding the import directory
	var ida []ImportDirectory
	for len(d) >= 20 {
		var dt ImportDirectory
		dt.OriginalFirstThunk = binary.LittleEndian.Uint32(d[0:4])
		dt.TimeDateStamp = binary.LittleEndian.Uint32(d[4:8])
		dt.ForwarderChain = binary.LittleEndian.Uint32(d[8:12])
		dt.Name = binary.LittleEndian.Uint32(d[12:16])
		dt.FirstThunk = binary.LittleEndian.Uint32(d[16:20])
		d = d[20:]
		if dt.OriginalFirstThunk == 0 {
			break
		}
		ida = append(ida, dt)
	}
	// TODO(brainman): this needs to be rewritten
	//  ds.Data() returns contents of section containing import table. Why store in variable called "names"?
	//  Why we are retrieving it second time? We already have it in "d", and it is not modified anywhere.
	//  getString does not extracts a string from symbol string table (as getString doco says).
	//  Why ds.Data() called again and again in the loop?
	//  Needs test before rewrite.
	names, _ := ds.Data()
	var all []string
	for _, dt := range ida {
		dt.dll, _ = getString(names, int(dt.Name-ds.VirtualAddress))
		d, _ = ds.Data()
		// seek to OriginalFirstThunk
		seek := dt.OriginalFirstThunk - ds.VirtualAddress
		if seek >= uint32(len(d)) {
			return nil, errors.New("import directory original first thunk doesn't fit within data seek")
		}
		d = d[seek:]
		for len(d) > 0 {
			if pe64 { // 64bit
				if len(d) < 8 {
					return nil, errors.New("thunk parsing needs at least 8-bytes")
				}
				va := binary.LittleEndian.Uint64(d[0:8])
				d = d[8:]
				if va == 0 {
					break
				}
				if va&0x8000000000000000 > 0 { // is Ordinal
					// TODO add dynimport ordinal support.
				} else {
					fn, _ := getString(names, int(uint32(va)-ds.VirtualAddress+2))
					all = append(all, fn+":"+dt.dll)
				}
			} else { // 32bit
				if len(d) <= 4 {
					return nil, errors.New("thunk parsing needs at least 5-bytes")
				}
				va := binary.LittleEndian.Uint32(d[0:4])
				d = d[4:]
				if va == 0 {
					break
				}
				if va&0x80000000 > 0 { // is Ordinal
					// TODO add dynimport ordinal support.
					//ord := va&0x0000FFFF
				} else {
					fn, _ := getString(names, int(va-ds.VirtualAddress+2))
					all = append(all, fn+":"+dt.dll)
				}
			}
		}
	}

	return all, nil
}

// ImportedLibraries returns the names of all libraries
// referred to by the binary f that are expected to be
// linked with the binary at dynamic link time.
func (f *File) ImportedLibraries() ([]string, error) {
	// TODO
	// cgo -dynimport don't use this for windows PE, so just return.
	return nil, nil
}

// FormatError is unused.
// The type is retained for compatibility.
type FormatError struct {
}

func (e *FormatError) Error() string {
	return "unknown error"
}

// readOptionalHeader accepts an io.ReadSeeker pointing to optional header in the PE file
// and its size as seen in the file header.
// It parses the given size of bytes and returns optional header. It infers whether the
// bytes being parsed refer to 32 bit or 64 bit version of optional header.
func readOptionalHeader(r io.ReadSeeker, sz uint16) (any, error) {
	// If optional header size is 0, return empty optional header.
	if sz == 0 {
		return nil, nil
	}

	var (
		// First couple of bytes in option header state its type.
		// We need to read them first to determine the type and
		// validity of optional header.
		ohMagic   uint16
		ohMagicSz = binary.Size(ohMagic)
	)

	// If optional header size is greater than 0 but less than its magic size, return error.
	if sz < uint16(ohMagicSz) {
		return nil, fmt.Errorf("optional header size is less than optional header magic size")
	}

	// read reads from io.ReadSeeke, r, into data.
	var err error
	read := func(data any) bool {
		err = binary.Read(r, binary.LittleEndian, data)
		return err == nil
	}

	if !read(&ohMagic) {
		return nil, fmt.Errorf("failure to read optional header magic: %v", err)

	}

	switch ohMagic {
	case 0x10b: // PE32
		var (
			oh32 OptionalHeader32
			// There can be 0 or more data directories. So the minimum size of optional
			// header is calculated by subtracting oh32.DataDirectory size from oh32 size.
			oh32MinSz = binary.Size(oh32) - binary.Size(oh32.DataDirectory)
		)

		if sz < uint16(oh32MinSz) {
			return nil, fmt.Errorf("optional header size(%d) is less minimum size (%d) of PE32 optional header", sz, oh32MinSz)
		}

		// Init oh32 fields
		oh32.Magic = ohMagic
		if !read(&oh32.MajorLinkerVersion) ||
			!read(&oh32.MinorLinkerVersion) ||
			!read(&oh32.SizeOfCode) ||
			!read(&oh32.SizeOfInitializedData) ||
			!read(&oh32.SizeOfUninitializedData) ||
			!read(&oh32.AddressOfEntryPoint) ||
			!read(&oh32.BaseOfCode) ||
			!read(&oh32.BaseOfData) ||
			!read(&oh32.ImageBase) ||
			!read(&oh32.SectionAlignment) ||
			!read(&oh32.FileAlignment) ||
			!read(&oh32.MajorOperatingSystemVersion) ||
			!read(&oh32.MinorOperatingSystemVersion) ||
			!read(&oh32.MajorImageVersion) ||
			!read(&oh32.MinorImageVersion) ||
			!read(&oh32.MajorSubsystemVersion) ||
			!read(&oh32.MinorSubsystemVersion) ||
			!read(&oh32.Win32VersionValue) ||
			!read(&oh32.SizeOfImage) ||
			!read(&oh32.SizeOfHeaders) ||
			!read(&oh32.CheckSum) ||
			!read(&oh32.Subsystem) ||
			!read(&oh32.DllCharacteristics) ||
			!read(&oh32.SizeOfStackReserve) ||
			!read(&oh32.SizeOfStackCommit) ||
			!read(&oh32.SizeOfHeapReserve) ||
			!read(&oh32.SizeOfHeapCommit) ||
			!read(&oh32.LoaderFlags) ||
			!read(&oh32.NumberOfRvaAndSizes) {
			return nil, fmt.Errorf("failure to read PE32 optional header: %v", err)
		}

		dd, err := readDataDirectories(r, sz-uint16(oh32MinSz), oh32.NumberOfRvaAndSizes)
		if err != nil {
			return nil, err
		}

		copy(oh32.DataDirectory[:], dd)

		return &oh32, nil
	case 0x20b: // PE32+
		var (
			oh64 OptionalHeader64
			// There can be 0 or more data directories. So the minimum size of optional
			// header is calculated by subtracting oh64.DataDirectory size from oh64 size.
			oh64MinSz = binary.Size(oh64) - binary.Size(oh64.DataDirectory)
		)

		if sz < uint16(oh64MinSz) {
			return nil, fmt.Errorf("optional header size(%d) is less minimum size (%d) for PE32+ optional header", sz, oh64MinSz)
		}

		// Init oh64 fields
		oh64.Magic = ohMagic
		if !read(&oh64.MajorLinkerVersion) ||
			!read(&oh64.MinorLinkerVersion) ||
			!read(&oh64.SizeOfCode) ||
			!read(&oh64.SizeOfInitializedData) ||
			!read(&oh64.SizeOfUninitializedData) ||
			!read(&oh64.AddressOfEntryPoint) ||
			!read(&oh64.BaseOfCode) ||
			!read(&oh64.ImageBase) ||
			!read(&oh64.SectionAlignment) ||
			!read(&oh64.FileAlignment) ||
			!read(&oh64.MajorOperatingSystemVersion) ||
			!read(&oh64.MinorOperatingSystemVersion) ||
			!read(&oh64.MajorImageVersion) ||
			!read(&oh64.MinorImageVersion) ||
			!read(&oh64.MajorSubsystemVersion) ||
			!read(&oh64.MinorSubsystemVersion) ||
			!read(&oh64.Win32VersionValue) ||
			!read(&oh64.SizeOfImage) ||
			!read(&oh64.SizeOfHeaders) ||
			!read(&oh64.CheckSum) ||
			!read(&oh64.Subsystem) ||
			!read(&oh64.DllCharacteristics) ||
			!read(&oh64.SizeOfStackReserve) ||
			!read(&oh64.SizeOfStackCommit) ||
			!read(&oh64.SizeOfHeapReserve) ||
			!read(&oh64.SizeOfHeapCommit) ||
			!read(&oh64.LoaderFlags) ||
			!read(&oh64.NumberOfRvaAndSizes) {
			return nil, fmt.Errorf("failure to read PE32+ optional header: %v", err)
		}

		dd, err := readDataDirectories(r, sz-uint16(oh64MinSz), oh64.NumberOfRvaAndSizes)
		if err != nil {
			return nil, err
		}

		copy(oh64.DataDirectory[:], dd)

		return &oh64, nil
	default:
		return nil, fmt.Errorf("optional header has unexpected Magic of 0x%x", ohMagic)
	}
}

// readDataDirectories accepts an io.ReadSeeker pointing to data directories in the PE file,
// its size and number of data directories as seen in optional header.
// It parses the given size of bytes and returns given number of data directories.
func readDataDirectories(r io.ReadSeeker, sz uint16, n uint32) ([]DataDirectory, error) {
	ddSz := uint64(binary.Size(DataDirectory{}))
	if uint64(sz) != uint64(n)*ddSz {
		return nil, fmt.Errorf("size of data directories(%d) is inconsistent with number of data directories(%d)", sz, n)
	}

	dd := make([]DataDirectory, n)
	if err := binary.Read(r, binary.LittleEndian, dd); err != nil {
		return nil, fmt.Errorf("failure to read data directories: %v", err)
	}

	return dd, nil
}

```

// === FILE: references!/go/src/debug/pe/pe.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pe

type FileHeader struct {
	Machine              uint16
	NumberOfSections     uint16
	TimeDateStamp        uint32
	PointerToSymbolTable uint32
	NumberOfSymbols      uint32
	SizeOfOptionalHeader uint16
	Characteristics      uint16
}

type DataDirectory struct {
	VirtualAddress uint32
	Size           uint32
}

type OptionalHeader32 struct {
	Magic                       uint16
	MajorLinkerVersion          uint8
	MinorLinkerVersion          uint8
	SizeOfCode                  uint32
	SizeOfInitializedData       uint32
	SizeOfUninitializedData     uint32
	AddressOfEntryPoint         uint32
	BaseOfCode                  uint32
	BaseOfData                  uint32
	ImageBase                   uint32
	SectionAlignment            uint32
	FileAlignment               uint32
	MajorOperatingSystemVersion uint16
	MinorOperatingSystemVersion uint16
	MajorImageVersion           uint16
	MinorImageVersion           uint16
	MajorSubsystemVersion       uint16
	MinorSubsystemVersion       uint16
	Win32VersionValue           uint32
	SizeOfImage                 uint32
	SizeOfHeaders               uint32
	CheckSum                    uint32
	Subsystem                   uint16
	DllCharacteristics          uint16
	SizeOfStackReserve          uint32
	SizeOfStackCommit           uint32
	SizeOfHeapReserve           uint32
	SizeOfHeapCommit            uint32
	LoaderFlags                 uint32
	NumberOfRvaAndSizes         uint32
	DataDirectory               [16]DataDirectory
}

type OptionalHeader64 struct {
	Magic                       uint16
	MajorLinkerVersion          uint8
	MinorLinkerVersion          uint8
	SizeOfCode                  uint32
	SizeOfInitializedData       uint32
	SizeOfUninitializedData     uint32
	AddressOfEntryPoint         uint32
	BaseOfCode                  uint32
	ImageBase                   uint64
	SectionAlignment            uint32
	FileAlignment               uint32
	MajorOperatingSystemVersion uint16
	MinorOperatingSystemVersion uint16
	MajorImageVersion           uint16
	MinorImageVersion           uint16
	MajorSubsystemVersion       uint16
	MinorSubsystemVersion       uint16
	Win32VersionValue           uint32
	SizeOfImage                 uint32
	SizeOfHeaders               uint32
	CheckSum                    uint32
	Subsystem                   uint16
	DllCharacteristics          uint16
	SizeOfStackReserve          uint64
	SizeOfStackCommit           uint64
	SizeOfHeapReserve           uint64
	SizeOfHeapCommit            uint64
	LoaderFlags                 uint32
	NumberOfRvaAndSizes         uint32
	DataDirectory               [16]DataDirectory
}

const (
	IMAGE_FILE_MACHINE_UNKNOWN     = 0x0
	IMAGE_FILE_MACHINE_AM33        = 0x1d3
	IMAGE_FILE_MACHINE_AMD64       = 0x8664
	IMAGE_FILE_MACHINE_ARM         = 0x1c0
	IMAGE_FILE_MACHINE_ARMNT       = 0x1c4
	IMAGE_FILE_MACHINE_ARM64       = 0xaa64
	IMAGE_FILE_MACHINE_EBC         = 0xebc
	IMAGE_FILE_MACHINE_I386        = 0x14c
	IMAGE_FILE_MACHINE_IA64        = 0x200
	IMAGE_FILE_MACHINE_LOONGARCH32 = 0x6232
	IMAGE_FILE_MACHINE_LOONGARCH64 = 0x6264
	IMAGE_FILE_MACHINE_M32R        = 0x9041
	IMAGE_FILE_MACHINE_MIPS16      = 0x266
	IMAGE_FILE_MACHINE_MIPSFPU     = 0x366
	IMAGE_FILE_MACHINE_MIPSFPU16   = 0x466
	IMAGE_FILE_MACHINE_POWERPC     = 0x1f0
	IMAGE_FILE_MACHINE_POWERPCFP   = 0x1f1
	IMAGE_FILE_MACHINE_R4000       = 0x166
	IMAGE_FILE_MACHINE_SH3         = 0x1a2
	IMAGE_FILE_MACHINE_SH3DSP      = 0x1a3
	IMAGE_FILE_MACHINE_SH4         = 0x1a6
	IMAGE_FILE_MACHINE_SH5         = 0x1a8
	IMAGE_FILE_MACHINE_THUMB       = 0x1c2
	IMAGE_FILE_MACHINE_WCEMIPSV2   = 0x169
	IMAGE_FILE_MACHINE_RISCV32     = 0x5032
	IMAGE_FILE_MACHINE_RISCV64     = 0x5064
	IMAGE_FILE_MACHINE_RISCV128    = 0x5128
)

// IMAGE_DIRECTORY_ENTRY constants
const (
	IMAGE_DIRECTORY_ENTRY_EXPORT         = 0
	IMAGE_DIRECTORY_ENTRY_IMPORT         = 1
	IMAGE_DIRECTORY_ENTRY_RESOURCE       = 2
	IMAGE_DIRECTORY_ENTRY_EXCEPTION      = 3
	IMAGE_DIRECTORY_ENTRY_SECURITY       = 4
	IMAGE_DIRECTORY_ENTRY_BASERELOC      = 5
	IMAGE_DIRECTORY_ENTRY_DEBUG          = 6
	IMAGE_DIRECTORY_ENTRY_ARCHITECTURE   = 7
	IMAGE_DIRECTORY_ENTRY_GLOBALPTR      = 8
	IMAGE_DIRECTORY_ENTRY_TLS            = 9
	IMAGE_DIRECTORY_ENTRY_LOAD_CONFIG    = 10
	IMAGE_DIRECTORY_ENTRY_BOUND_IMPORT   = 11
	IMAGE_DIRECTORY_ENTRY_IAT            = 12
	IMAGE_DIRECTORY_ENTRY_DELAY_IMPORT   = 13
	IMAGE_DIRECTORY_ENTRY_COM_DESCRIPTOR = 14
)

// Values of IMAGE_FILE_HEADER.Characteristics. These can be combined together.
const (
	IMAGE_FILE_RELOCS_STRIPPED         = 0x0001
	IMAGE_FILE_EXECUTABLE_IMAGE        = 0x0002
	IMAGE_FILE_LINE_NUMS_STRIPPED      = 0x0004
	IMAGE_FILE_LOCAL_SYMS_STRIPPED     = 0x0008
	IMAGE_FILE_AGGRESIVE_WS_TRIM       = 0x0010
	IMAGE_FILE_LARGE_ADDRESS_AWARE     = 0x0020
	IMAGE_FILE_BYTES_REVERSED_LO       = 0x0080
	IMAGE_FILE_32BIT_MACHINE           = 0x0100
	IMAGE_FILE_DEBUG_STRIPPED          = 0x0200
	IMAGE_FILE_REMOVABLE_RUN_FROM_SWAP = 0x0400
	IMAGE_FILE_NET_RUN_FROM_SWAP       = 0x0800
	IMAGE_FILE_SYSTEM                  = 0x1000
	IMAGE_FILE_DLL                     = 0x2000
	IMAGE_FILE_UP_SYSTEM_ONLY          = 0x4000
	IMAGE_FILE_BYTES_REVERSED_HI       = 0x8000
)

// OptionalHeader64.Subsystem and OptionalHeader32.Subsystem values.
const (
	IMAGE_SUBSYSTEM_UNKNOWN                  = 0
	IMAGE_SUBSYSTEM_NATIVE                   = 1
	IMAGE_SUBSYSTEM_WINDOWS_GUI              = 2
	IMAGE_SUBSYSTEM_WINDOWS_CUI              = 3
	IMAGE_SUBSYSTEM_OS2_CUI                  = 5
	IMAGE_SUBSYSTEM_POSIX_CUI                = 7
	IMAGE_SUBSYSTEM_NATIVE_WINDOWS           = 8
	IMAGE_SUBSYSTEM_WINDOWS_CE_GUI           = 9
	IMAGE_SUBSYSTEM_EFI_APPLICATION          = 10
	IMAGE_SUBSYSTEM_EFI_BOOT_SERVICE_DRIVER  = 11
	IMAGE_SUBSYSTEM_EFI_RUNTIME_DRIVER       = 12
	IMAGE_SUBSYSTEM_EFI_ROM                  = 13
	IMAGE_SUBSYSTEM_XBOX                     = 14
	IMAGE_SUBSYSTEM_WINDOWS_BOOT_APPLICATION = 16
)

// OptionalHeader64.DllCharacteristics and OptionalHeader32.DllCharacteristics
// values. These can be combined together.
const (
	IMAGE_DLLCHARACTERISTICS_HIGH_ENTROPY_VA       = 0x0020
	IMAGE_DLLCHARACTERISTICS_DYNAMIC_BASE          = 0x0040
	IMAGE_DLLCHARACTERISTICS_FORCE_INTEGRITY       = 0x0080
	IMAGE_DLLCHARACTERISTICS_NX_COMPAT             = 0x0100
	IMAGE_DLLCHARACTERISTICS_NO_ISOLATION          = 0x0200
	IMAGE_DLLCHARACTERISTICS_NO_SEH                = 0x0400
	IMAGE_DLLCHARACTERISTICS_NO_BIND               = 0x0800
	IMAGE_DLLCHARACTERISTICS_APPCONTAINER          = 0x1000
	IMAGE_DLLCHARACTERISTICS_WDM_DRIVER            = 0x2000
	IMAGE_DLLCHARACTERISTICS_GUARD_CF              = 0x4000
	IMAGE_DLLCHARACTERISTICS_TERMINAL_SERVER_AWARE = 0x8000
)

```

// === FILE: references!/go/src/debug/pe/section.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pe

import (
	"encoding/binary"
	"fmt"
	"internal/saferio"
	"io"
	"strconv"
)

// SectionHeader32 represents real PE COFF section header.
type SectionHeader32 struct {
	Name                 [8]uint8
	VirtualSize          uint32
	VirtualAddress       uint32
	SizeOfRawData        uint32
	PointerToRawData     uint32
	PointerToRelocations uint32
	PointerToLineNumbers uint32
	NumberOfRelocations  uint16
	NumberOfLineNumbers  uint16
	Characteristics      uint32
}

// fullName finds real name of section sh. Normally name is stored
// in sh.Name, but if it is longer then 8 characters, it is stored
// in COFF string table st instead.
func (sh *SectionHeader32) fullName(st StringTable) (string, error) {
	if sh.Name[0] != '/' {
		return cstring(sh.Name[:]), nil
	}
	i, err := strconv.Atoi(cstring(sh.Name[1:]))
	if err != nil {
		return "", err
	}
	return st.String(uint32(i))
}

// TODO(brainman): copy all IMAGE_REL_* consts from ldpe.go here

// Reloc represents a PE COFF relocation.
// Each section contains its own relocation list.
type Reloc struct {
	VirtualAddress   uint32
	SymbolTableIndex uint32
	Type             uint16
}

func readRelocs(sh *SectionHeader, r io.ReadSeeker) ([]Reloc, error) {
	if sh.NumberOfRelocations <= 0 {
		return nil, nil
	}
	_, err := r.Seek(int64(sh.PointerToRelocations), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("fail to seek to %q section relocations: %v", sh.Name, err)
	}
	relocs := make([]Reloc, sh.NumberOfRelocations)
	err = binary.Read(r, binary.LittleEndian, relocs)
	if err != nil {
		return nil, fmt.Errorf("fail to read section relocations: %v", err)
	}
	return relocs, nil
}

// SectionHeader is similar to [SectionHeader32] with Name
// field replaced by Go string.
type SectionHeader struct {
	Name                 string
	VirtualSize          uint32
	VirtualAddress       uint32
	Size                 uint32
	Offset               uint32
	PointerToRelocations uint32
	PointerToLineNumbers uint32
	NumberOfRelocations  uint16
	NumberOfLineNumbers  uint16
	Characteristics      uint32
}

// Section provides access to PE COFF section.
type Section struct {
	SectionHeader
	Relocs []Reloc

	// Embed ReaderAt for ReadAt method.
	// Do not embed SectionReader directly
	// to avoid having Read and Seek.
	// If a client wants Read and Seek it must use
	// Open() to avoid fighting over the seek offset
	// with other clients.
	io.ReaderAt
	sr *io.SectionReader
}

// Data reads and returns the contents of the PE section s.
//
// If s.Offset is 0, the section has no contents,
// and Data will always return a non-nil error.
func (s *Section) Data() ([]byte, error) {
	return saferio.ReadDataAt(s.sr, uint64(s.Size), 0)
}

// Open returns a new ReadSeeker reading the PE section s.
//
// If s.Offset is 0, the section has no contents, and all calls
// to the returned reader will return a non-nil error.
func (s *Section) Open() io.ReadSeeker {
	return io.NewSectionReader(s.sr, 0, 1<<63-1)
}

// Section characteristics flags.
const (
	IMAGE_SCN_CNT_CODE               = 0x00000020
	IMAGE_SCN_CNT_INITIALIZED_DATA   = 0x00000040
	IMAGE_SCN_CNT_UNINITIALIZED_DATA = 0x00000080
	IMAGE_SCN_LNK_COMDAT             = 0x00001000
	IMAGE_SCN_MEM_DISCARDABLE        = 0x02000000
	IMAGE_SCN_MEM_EXECUTE            = 0x20000000
	IMAGE_SCN_MEM_READ               = 0x40000000
	IMAGE_SCN_MEM_WRITE              = 0x80000000
)

```

// === FILE: references!/go/src/debug/pe/string.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pe

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"internal/saferio"
	"io"
)

// cstring converts ASCII byte sequence b to string.
// It stops once it finds 0 or reaches end of b.
func cstring(b []byte) string {
	i := bytes.IndexByte(b, 0)
	if i == -1 {
		i = len(b)
	}
	return string(b[:i])
}

// StringTable is a COFF string table.
type StringTable []byte

func readStringTable(fh *FileHeader, r io.ReadSeeker) (StringTable, error) {
	// COFF string table is located right after COFF symbol table.
	if fh.PointerToSymbolTable <= 0 {
		return nil, nil
	}
	offset := fh.PointerToSymbolTable + COFFSymbolSize*fh.NumberOfSymbols
	_, err := r.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("fail to seek to string table: %v", err)
	}
	var l uint32
	err = binary.Read(r, binary.LittleEndian, &l)
	if err != nil {
		return nil, fmt.Errorf("fail to read string table length: %v", err)
	}
	// string table length includes itself
	if l <= 4 {
		return nil, nil
	}
	l -= 4

	buf, err := saferio.ReadData(r, uint64(l))
	if err != nil {
		return nil, fmt.Errorf("fail to read string table: %v", err)
	}
	return StringTable(buf), nil
}

// TODO(brainman): decide if start parameter should be int instead of uint32

// String extracts string from COFF string table st at offset start.
func (st StringTable) String(start uint32) (string, error) {
	// start includes 4 bytes of string table length
	if start < 4 {
		return "", fmt.Errorf("offset %d is before the start of string table", start)
	}
	start -= 4
	if int(start) > len(st) {
		return "", fmt.Errorf("offset %d is beyond the end of string table", start)
	}
	return cstring(st[start:]), nil
}

```

// === FILE: references!/go/src/debug/pe/symbol.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pe

import (
	"encoding/binary"
	"errors"
	"fmt"
	"internal/saferio"
	"io"
	"unsafe"
)

const COFFSymbolSize = 18

// COFFSymbol represents single COFF symbol table record.
type COFFSymbol struct {
	Name               [8]uint8
	Value              uint32
	SectionNumber      int16
	Type               uint16
	StorageClass       uint8
	NumberOfAuxSymbols uint8
}

// readCOFFSymbols reads in the symbol table for a PE file, returning
// a slice of COFFSymbol objects. The PE format includes both primary
// symbols (whose fields are described by COFFSymbol above) and
// auxiliary symbols; all symbols are 18 bytes in size. The auxiliary
// symbols for a given primary symbol are placed following it in the
// array, e.g.
//
//	...
//	k+0:  regular sym k
//	k+1:    1st aux symbol for k
//	k+2:    2nd aux symbol for k
//	k+3:  regular sym k+3
//	k+4:    1st aux symbol for k+3
//	k+5:  regular sym k+5
//	k+6:  regular sym k+6
//
// The PE format allows for several possible aux symbol formats. For
// more info see:
//
//	https://docs.microsoft.com/en-us/windows/win32/debug/pe-format#auxiliary-symbol-records
//
// At the moment this package only provides APIs for looking at
// aux symbols of format 5 (associated with section definition symbols).
func readCOFFSymbols(fh *FileHeader, r io.ReadSeeker) ([]COFFSymbol, error) {
	if fh.PointerToSymbolTable == 0 {
		return nil, nil
	}
	if fh.NumberOfSymbols <= 0 {
		return nil, nil
	}
	_, err := r.Seek(int64(fh.PointerToSymbolTable), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("fail to seek to symbol table: %v", err)
	}
	c := saferio.SliceCap[COFFSymbol](uint64(fh.NumberOfSymbols))
	if c < 0 {
		return nil, errors.New("too many symbols; file may be corrupt")
	}
	syms := make([]COFFSymbol, 0, c)
	naux := 0
	for k := uint32(0); k < fh.NumberOfSymbols; k++ {
		var sym COFFSymbol
		if naux == 0 {
			// Read a primary symbol.
			err = binary.Read(r, binary.LittleEndian, &sym)
			if err != nil {
				return nil, fmt.Errorf("fail to read symbol table: %v", err)
			}
			// Record how many auxiliary symbols it has.
			naux = int(sym.NumberOfAuxSymbols)
		} else {
			// Read an aux symbol. At the moment we assume all
			// aux symbols are format 5 (obviously this doesn't always
			// hold; more cases will be needed below if more aux formats
			// are supported in the future).
			naux--
			aux := (*COFFSymbolAuxFormat5)(unsafe.Pointer(&sym))
			err = binary.Read(r, binary.LittleEndian, aux)
			if err != nil {
				return nil, fmt.Errorf("fail to read symbol table: %v", err)
			}
		}
		syms = append(syms, sym)
	}
	if naux != 0 {
		return nil, fmt.Errorf("fail to read symbol table: %d aux symbols unread", naux)
	}
	return syms, nil
}

// isSymNameOffset checks symbol name if it is encoded as offset into string table.
func isSymNameOffset(name [8]byte) (bool, uint32) {
	if name[0] == 0 && name[1] == 0 && name[2] == 0 && name[3] == 0 {
		offset := binary.LittleEndian.Uint32(name[4:])
		if offset == 0 {
			// symbol has no name
			return false, 0
		}
		return true, offset
	}
	return false, 0
}

// FullName finds real name of symbol sym. Normally name is stored
// in sym.Name, but if it is longer then 8 characters, it is stored
// in COFF string table st instead.
func (sym *COFFSymbol) FullName(st StringTable) (string, error) {
	if ok, offset := isSymNameOffset(sym.Name); ok {
		return st.String(offset)
	}
	return cstring(sym.Name[:]), nil
}

func removeAuxSymbols(allsyms []COFFSymbol, st StringTable) ([]*Symbol, error) {
	if len(allsyms) == 0 {
		return nil, nil
	}
	syms := make([]*Symbol, 0)
	aux := uint8(0)
	for _, sym := range allsyms {
		if aux > 0 {
			aux--
			continue
		}
		name, err := sym.FullName(st)
		if err != nil {
			return nil, err
		}
		aux = sym.NumberOfAuxSymbols
		s := &Symbol{
			Name:          name,
			Value:         sym.Value,
			SectionNumber: sym.SectionNumber,
			Type:          sym.Type,
			StorageClass:  sym.StorageClass,
		}
		syms = append(syms, s)
	}
	return syms, nil
}

// Symbol is similar to [COFFSymbol] with Name field replaced
// by Go string. Symbol also does not have NumberOfAuxSymbols.
type Symbol struct {
	Name          string
	Value         uint32
	SectionNumber int16
	Type          uint16
	StorageClass  uint8
}

// COFFSymbolAuxFormat5 describes the expected form of an aux symbol
// attached to a section definition symbol. The PE format defines a
// number of different aux symbol formats: format 1 for function
// definitions, format 2 for .be and .ef symbols, and so on. Format 5
// holds extra info associated with a section definition, including
// number of relocations + line numbers, as well as COMDAT info. See
// https://docs.microsoft.com/en-us/windows/win32/debug/pe-format#auxiliary-format-5-section-definitions
// for more on what's going on here.
type COFFSymbolAuxFormat5 struct {
	Size           uint32
	NumRelocs      uint16
	NumLineNumbers uint16
	Checksum       uint32
	SecNum         uint16
	Selection      uint8
	_              [3]uint8 // padding
}

// These constants make up the possible values for the 'Selection'
// field in an AuxFormat5.
const (
	IMAGE_COMDAT_SELECT_NODUPLICATES = 1
	IMAGE_COMDAT_SELECT_ANY          = 2
	IMAGE_COMDAT_SELECT_SAME_SIZE    = 3
	IMAGE_COMDAT_SELECT_EXACT_MATCH  = 4
	IMAGE_COMDAT_SELECT_ASSOCIATIVE  = 5
	IMAGE_COMDAT_SELECT_LARGEST      = 6
)

// COFFSymbolReadSectionDefAux returns a blob of auxiliary information
// (including COMDAT info) for a section definition symbol. Here 'idx'
// is the index of a section symbol in the main [COFFSymbol] array for
// the File. Return value is a pointer to the appropriate aux symbol
// struct. For more info, see:
//
// auxiliary symbols: https://docs.microsoft.com/en-us/windows/win32/debug/pe-format#auxiliary-symbol-records
// COMDAT sections: https://docs.microsoft.com/en-us/windows/win32/debug/pe-format#comdat-sections-object-only
// auxiliary info for section definitions: https://docs.microsoft.com/en-us/windows/win32/debug/pe-format#auxiliary-format-5-section-definitions
func (f *File) COFFSymbolReadSectionDefAux(idx int) (*COFFSymbolAuxFormat5, error) {
	var rv *COFFSymbolAuxFormat5
	if idx < 0 || idx >= len(f.COFFSymbols) {
		return rv, fmt.Errorf("invalid symbol index")
	}
	pesym := &f.COFFSymbols[idx]
	const IMAGE_SYM_CLASS_STATIC = 3
	if pesym.StorageClass != uint8(IMAGE_SYM_CLASS_STATIC) {
		return rv, fmt.Errorf("incorrect symbol storage class")
	}
	if pesym.NumberOfAuxSymbols == 0 || idx+1 >= len(f.COFFSymbols) {
		return rv, fmt.Errorf("aux symbol unavailable")
	}
	// Locate and return a pointer to the successor aux symbol.
	pesymn := &f.COFFSymbols[idx+1]
	rv = (*COFFSymbolAuxFormat5)(unsafe.Pointer(pesymn))
	return rv, nil
}

```

// === FILE: references!/go/src/debug/plan9obj/file.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package plan9obj implements access to Plan 9 a.out object files.

# Security

This package is not designed to be hardened against adversarial inputs, and is
outside the scope of https://go.dev/security/policy. In particular, only basic
validation is done when parsing object files. As such, care should be taken when
parsing untrusted inputs, as parsing malformed files may consume significant
resources, or cause panics.
*/
package plan9obj

import (
	"encoding/binary"
	"errors"
	"fmt"
	"internal/saferio"
	"io"
	"os"
)

// A FileHeader represents a Plan 9 a.out file header.
type FileHeader struct {
	Magic       uint32
	Bss         uint32
	Entry       uint64
	PtrSize     int
	LoadAddress uint64
	HdrSize     uint64
}

// A File represents an open Plan 9 a.out file.
type File struct {
	FileHeader
	Sections []*Section
	closer   io.Closer
}

// A SectionHeader represents a single Plan 9 a.out section header.
// This structure doesn't exist on-disk, but eases navigation
// through the object file.
type SectionHeader struct {
	Name   string
	Size   uint32
	Offset uint32
}

// A Section represents a single section in a Plan 9 a.out file.
type Section struct {
	SectionHeader

	// Embed ReaderAt for ReadAt method.
	// Do not embed SectionReader directly
	// to avoid having Read and Seek.
	// If a client wants Read and Seek it must use
	// Open() to avoid fighting over the seek offset
	// with other clients.
	io.ReaderAt
	sr *io.SectionReader
}

// Data reads and returns the contents of the Plan 9 a.out section.
func (s *Section) Data() ([]byte, error) {
	return saferio.ReadDataAt(s.sr, uint64(s.Size), 0)
}

// Open returns a new ReadSeeker reading the Plan 9 a.out section.
func (s *Section) Open() io.ReadSeeker { return io.NewSectionReader(s.sr, 0, 1<<63-1) }

// A Symbol represents an entry in a Plan 9 a.out symbol table section.
type Sym struct {
	Value uint64
	Type  rune
	Name  string
}

/*
 * Plan 9 a.out reader
 */

// formatError is returned by some operations if the data does
// not have the correct format for an object file.
type formatError struct {
	off int
	msg string
	val any
}

func (e *formatError) Error() string {
	msg := e.msg
	if e.val != nil {
		msg += fmt.Sprintf(" '%v'", e.val)
	}
	msg += fmt.Sprintf(" in record at byte %#x", e.off)
	return msg
}

// Open opens the named file using [os.Open] and prepares it for use as a Plan 9 a.out binary.
func Open(name string) (*File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	ff, err := NewFile(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	ff.closer = f
	return ff, nil
}

// Close closes the [File].
// If the [File] was created using [NewFile] directly instead of [Open],
// Close has no effect.
func (f *File) Close() error {
	var err error
	if f.closer != nil {
		err = f.closer.Close()
		f.closer = nil
	}
	return err
}

func parseMagic(magic []byte) (uint32, error) {
	m := binary.BigEndian.Uint32(magic)
	switch m {
	case Magic386, MagicAMD64, MagicARM:
		return m, nil
	}
	return 0, &formatError{0, "bad magic number", magic}
}

// NewFile creates a new [File] for accessing a Plan 9 binary in an underlying reader.
// The Plan 9 binary is expected to start at position 0 in the ReaderAt.
func NewFile(r io.ReaderAt) (*File, error) {
	sr := io.NewSectionReader(r, 0, 1<<63-1)
	// Read and decode Plan 9 magic
	var magic [4]byte
	if _, err := r.ReadAt(magic[:], 0); err != nil {
		return nil, err
	}
	_, err := parseMagic(magic[:])
	if err != nil {
		return nil, err
	}

	ph := new(prog)
	if err := binary.Read(sr, binary.BigEndian, ph); err != nil {
		return nil, err
	}

	f := &File{FileHeader: FileHeader{
		Magic:       ph.Magic,
		Bss:         ph.Bss,
		Entry:       uint64(ph.Entry),
		PtrSize:     4,
		LoadAddress: 0x1000,
		HdrSize:     4 * 8,
	}}

	if ph.Magic&Magic64 != 0 {
		if err := binary.Read(sr, binary.BigEndian, &f.Entry); err != nil {
			return nil, err
		}
		f.PtrSize = 8
		f.LoadAddress = 0x200000
		f.HdrSize += 8
	}

	var sects = []struct {
		name string
		size uint32
	}{
		{"text", ph.Text},
		{"data", ph.Data},
		{"syms", ph.Syms},
		{"spsz", ph.Spsz},
		{"pcsz", ph.Pcsz},
	}

	f.Sections = make([]*Section, 5)

	off := uint32(f.HdrSize)

	for i, sect := range sects {
		s := new(Section)
		s.SectionHeader = SectionHeader{
			Name:   sect.name,
			Size:   sect.size,
			Offset: off,
		}
		off += sect.size
		s.sr = io.NewSectionReader(r, int64(s.Offset), int64(s.Size))
		s.ReaderAt = s.sr
		f.Sections[i] = s
	}

	return f, nil
}

func walksymtab(data []byte, ptrsz int, fn func(sym) error) error {
	var order binary.ByteOrder = binary.BigEndian
	var s sym
	p := data
	for len(p) >= 4 {
		// Symbol type, value.
		if len(p) < ptrsz {
			return &formatError{len(data), "unexpected EOF", nil}
		}
		// fixed-width value
		if ptrsz == 8 {
			s.value = order.Uint64(p[0:8])
			p = p[8:]
		} else {
			s.value = uint64(order.Uint32(p[0:4]))
			p = p[4:]
		}

		if len(p) < 1 {
			return &formatError{len(data), "unexpected EOF", nil}
		}
		typ := p[0] & 0x7F
		s.typ = typ
		p = p[1:]

		// Name.
		var i int
		var nnul int
		for i = 0; i < len(p); i++ {
			if p[i] == 0 {
				nnul = 1
				break
			}
		}
		switch typ {
		case 'z', 'Z':
			p = p[i+nnul:]
			for i = 0; i+2 <= len(p); i += 2 {
				if p[i] == 0 && p[i+1] == 0 {
					nnul = 2
					break
				}
			}
		}
		if len(p) < i+nnul {
			return &formatError{len(data), "unexpected EOF", nil}
		}
		s.name = p[0:i]
		i += nnul
		p = p[i:]

		fn(s)
	}
	return nil
}

// newTable decodes the Go symbol table in data,
// returning an in-memory representation.
func newTable(symtab []byte, ptrsz int) ([]Sym, error) {
	var n int
	err := walksymtab(symtab, ptrsz, func(s sym) error {
		n++
		return nil
	})
	if err != nil {
		return nil, err
	}

	fname := make(map[uint16]string)
	syms := make([]Sym, 0, n)
	err = walksymtab(symtab, ptrsz, func(s sym) error {
		n := len(syms)
		syms = syms[0 : n+1]
		ts := &syms[n]
		ts.Type = rune(s.typ)
		ts.Value = s.value
		switch s.typ {
		default:
			ts.Name = string(s.name)
		case 'z', 'Z':
			for i := 0; i < len(s.name); i += 2 {
				eltIdx := binary.BigEndian.Uint16(s.name[i : i+2])
				elt, ok := fname[eltIdx]
				if !ok {
					return &formatError{-1, "bad filename code", eltIdx}
				}
				if n := len(ts.Name); n > 0 && ts.Name[n-1] != '/' {
					ts.Name += "/"
				}
				ts.Name += elt
			}
		}
		switch s.typ {
		case 'f':
			fname[uint16(s.value)] = ts.Name
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return syms, nil
}

// ErrNoSymbols is returned by [File.Symbols] if there is no such section
// in the File.
var ErrNoSymbols = errors.New("no symbol section")

// Symbols returns the symbol table for f.
func (f *File) Symbols() ([]Sym, error) {
	symtabSection := f.Section("syms")
	if symtabSection == nil {
		return nil, ErrNoSymbols
	}

	symtab, err := symtabSection.Data()
	if err != nil {
		return nil, errors.New("cannot load symbol section")
	}

	return newTable(symtab, f.PtrSize)
}

// Section returns a section with the given name, or nil if no such
// section exists.
func (f *File) Section(name string) *Section {
	for _, s := range f.Sections {
		if s.Name == name {
			return s
		}
	}
	return nil
}

```

// === FILE: references!/go/src/debug/plan9obj/plan9obj.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
 * Plan 9 a.out constants and data structures
 */

package plan9obj

// Plan 9 Program header.
type prog struct {
	Magic uint32 /* magic number */
	Text  uint32 /* size of text segment */
	Data  uint32 /* size of initialized data */
	Bss   uint32 /* size of uninitialized data */
	Syms  uint32 /* size of symbol table */
	Entry uint32 /* entry point */
	Spsz  uint32 /* size of pc/sp offset table */
	Pcsz  uint32 /* size of pc/line number table */
}

// Plan 9 symbol table entries.
type sym struct {
	value uint64
	typ   byte
	name  []byte
}

const (
	Magic64 = 0x8000 // 64-bit expanded header

	Magic386   = (4*11+0)*11 + 7
	MagicAMD64 = (4*26+0)*26 + 7 + Magic64
	MagicARM   = (4*20+0)*20 + 7
)

```

