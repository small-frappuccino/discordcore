# Domain Architecture: io

## Layout Topology
```text
io/
├── fs
│   ├── format.go
│   ├── fs.go
│   ├── glob.go
│   ├── readdir.go
│   ├── readfile.go
│   ├── readlink.go
│   ├── stat.go
│   ├── sub.go
│   └── walk.go
├── ioutil
│   ├── ioutil.go
│   └── tempfile.go
├── io.go
├── multi.go
└── pipe.go
```

## Source Stream Aggregation

// === FILE: references/go/src/io/fs/format.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"time"
)

// FormatFileInfo returns a formatted version of info for human readability.
// Implementations of [FileInfo] can call this from a String method.
// The output for a file named "hello.go", 100 bytes, mode 0o644, created
// January 1, 1970 at noon is
//
//	-rw-r--r-- 100 1970-01-01 12:00:00 hello.go
func FormatFileInfo(info FileInfo) string {
	name := info.Name()
	b := make([]byte, 0, 40+len(name))
	b = append(b, info.Mode().String()...)
	b = append(b, ' ')

	size := info.Size()
	var usize uint64
	if size >= 0 {
		usize = uint64(size)
	} else {
		b = append(b, '-')
		usize = uint64(-size)
	}
	var buf [20]byte
	i := len(buf) - 1
	for usize >= 10 {
		q := usize / 10
		buf[i] = byte('0' + usize - q*10)
		i--
		usize = q
	}
	buf[i] = byte('0' + usize)
	b = append(b, buf[i:]...)
	b = append(b, ' ')

	b = append(b, info.ModTime().Format(time.DateTime)...)
	b = append(b, ' ')

	b = append(b, name...)
	if info.IsDir() {
		b = append(b, '/')
	}

	return string(b)
}

// FormatDirEntry returns a formatted version of dir for human readability.
// Implementations of [DirEntry] can call this from a String method.
// The outputs for a directory named subdir and a file named hello.go are:
//
//	d subdir/
//	- hello.go
func FormatDirEntry(dir DirEntry) string {
	name := dir.Name()
	b := make([]byte, 0, 5+len(name))

	// The Type method does not return any permission bits,
	// so strip them from the string.
	mode := dir.Type().String()
	mode = mode[:len(mode)-9]

	b = append(b, mode...)
	b = append(b, ' ')
	b = append(b, name...)
	if dir.IsDir() {
		b = append(b, '/')
	}
	return string(b)
}

```

// === FILE: references/go/src/io/fs/fs.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fs defines basic interfaces to a file system.
// A file system can be provided by the host operating system
// but also by other packages.
//
// # Path Names
//
// The interfaces in this package all operate on the same
// path name syntax, regardless of the host operating system.
//
// Path names are UTF-8-encoded,
// unrooted, slash-separated sequences of path elements, like “x/y/z”.
// Path names must not contain an element that is “.” or “..” or the empty string,
// except for the special case that the name "." may be used for the root directory.
// Paths must not start or end with a slash: “/x” and “x/” are invalid.
//
// # Testing
//
// See the [testing/fstest] package for support with testing
// implementations of file systems.
package fs

import (
	"internal/oserror"
	"time"
	"unicode/utf8"
)

// An FS provides access to a hierarchical file system.
//
// The FS interface is the minimum implementation required of the file system.
// A file system may implement additional interfaces,
// such as [ReadFileFS], to provide additional or optimized functionality.
//
// [testing/fstest.TestFS] may be used to test implementations of an FS for
// correctness.
type FS interface {
	// Open opens the named file.
	// [File.Close] must be called to release any associated resources.
	//
	// When Open returns an error, it should be of type *PathError
	// with the Op field set to "open", the Path field set to name,
	// and the Err field describing the problem.
	//
	// Open should reject attempts to open names that do not satisfy
	// ValidPath(name), returning a *PathError with Err set to
	// ErrInvalid or ErrNotExist.
	Open(name string) (File, error)
}

// ValidPath reports whether the given path name
// is valid for use in a call to Open.
//
// Note that paths are slash-separated on all systems, even Windows.
// Paths containing other characters such as backslash and colon
// are accepted as valid, but those characters must never be
// interpreted by an [FS] implementation as path element separators.
// See the [Path Names] section for more details.
//
// [Path Names]: https://pkg.go.dev/io/fs#hdr-Path_Names
func ValidPath(name string) bool {
	if !utf8.ValidString(name) {
		return false
	}

	if name == "." {
		// special case
		return true
	}

	// Iterate over elements in name, checking each.
	for {
		i := 0
		for i < len(name) && name[i] != '/' {
			i++
		}
		elem := name[:i]
		if elem == "" || elem == "." || elem == ".." {
			return false
		}
		if i == len(name) {
			return true // reached clean ending
		}
		name = name[i+1:]
	}
}

// A File provides access to a single file.
// The File interface is the minimum implementation required of the file.
// Directory files should also implement [ReadDirFile].
// A file may implement [io.ReaderAt] or [io.Seeker] as optimizations.
type File interface {
	Stat() (FileInfo, error)
	Read([]byte) (int, error)
	Close() error
}

// A DirEntry is an entry read from a directory
// (using the [ReadDir] function or a [ReadDirFile]'s ReadDir method).
type DirEntry interface {
	// Name returns the name of the file (or subdirectory) described by the entry.
	// This name is only the final element of the path (the base name), not the entire path.
	// For example, Name would return "hello.go" not "home/gopher/hello.go".
	Name() string

	// IsDir reports whether the entry describes a directory.
	IsDir() bool

	// Type returns the type bits for the entry.
	// The type bits are a subset of the usual FileMode bits, those returned by the FileMode.Type method.
	Type() FileMode

	// Info returns the FileInfo for the file or subdirectory described by the entry.
	// The returned FileInfo may be from the time of the original directory read
	// or from the time of the call to Info. If the file has been removed or renamed
	// since the directory read, Info may return an error satisfying errors.Is(err, ErrNotExist).
	// If the entry denotes a symbolic link, Info reports the information about the link itself,
	// not the link's target.
	Info() (FileInfo, error)
}

// A ReadDirFile is a directory file whose entries can be read with the ReadDir method.
// Every directory file should implement this interface.
// (It is permissible for any file to implement this interface,
// but if so ReadDir should return an error for non-directories.)
type ReadDirFile interface {
	File

	// ReadDir reads the contents of the directory and returns
	// a slice of up to n DirEntry values in directory order.
	// Subsequent calls on the same file will yield further DirEntry values.
	//
	// If n > 0, ReadDir returns at most n DirEntry structures.
	// In this case, if ReadDir returns an empty slice, it will return
	// a non-nil error explaining why.
	// At the end of a directory, the error is io.EOF.
	// (ReadDir must return io.EOF itself, not an error wrapping io.EOF.)
	//
	// If n <= 0, ReadDir returns all remaining DirEntry values from the directory
	// in a single slice. In this case, if ReadDir succeeds (reads all the way
	// to the end of the directory), it returns the slice and a nil error.
	// If it encounters an error before the end of the directory,
	// ReadDir returns the DirEntry list read until that point and a non-nil error.
	ReadDir(n int) ([]DirEntry, error)
}

// Generic file system errors.
// Errors returned by file systems can be tested against these errors
// using [errors.Is].
var (
	ErrInvalid    = errInvalid()    // "invalid argument"
	ErrPermission = errPermission() // "permission denied"
	ErrExist      = errExist()      // "file already exists"
	ErrNotExist   = errNotExist()   // "file does not exist"
)

// ErrClosed is returned when operations are attempted on a file that
// has already been closed, including when Close is called more than once.
var ErrClosed = errClosed()

func errInvalid() error    { return oserror.ErrInvalid }
func errPermission() error { return oserror.ErrPermission }
func errExist() error      { return oserror.ErrExist }
func errNotExist() error   { return oserror.ErrNotExist }
func errClosed() error     { return oserror.ErrClosed }

// A FileInfo describes a file and is returned by [Stat].
type FileInfo interface {
	Name() string       // base name of the file
	Size() int64        // length in bytes for regular files; system-dependent for others
	Mode() FileMode     // file mode bits
	ModTime() time.Time // modification time
	IsDir() bool        // abbreviation for Mode().IsDir()
	Sys() any           // underlying data source (can return nil)
}

// A FileMode represents a file's mode and permission bits.
// The bits have the same definition on all systems, so that
// information about files can be moved from one system
// to another portably. Not all bits apply to all systems.
// The only required bit is [ModeDir] for directories.
type FileMode uint32

// The defined file mode bits are the most significant bits of the [FileMode].
// The nine least-significant bits are the standard Unix rwxrwxrwx permissions.
// The values of these bits should be considered part of the public API and
// may be used in wire protocols or disk representations: they must not be
// changed, although new bits might be added.
const (
	// The single letters are the abbreviations
	// used by the String method's formatting.
	ModeDir        FileMode = 1 << (32 - 1 - iota) // d: is a directory
	ModeAppend                                     // a: append-only
	ModeExclusive                                  // l: exclusive use
	ModeTemporary                                  // T: temporary file; Plan 9 only
	ModeSymlink                                    // L: symbolic link
	ModeDevice                                     // D: device file
	ModeNamedPipe                                  // p: named pipe (FIFO)
	ModeSocket                                     // S: Unix domain socket
	ModeSetuid                                     // u: setuid
	ModeSetgid                                     // g: setgid
	ModeCharDevice                                 // c: Unix character device, when ModeDevice is set
	ModeSticky                                     // t: sticky
	ModeIrregular                                  // ?: non-regular file; nothing else is known about this file

	// Mask for the type bits. For regular files, none will be set.
	ModeType = ModeDir | ModeSymlink | ModeNamedPipe | ModeSocket | ModeDevice | ModeCharDevice | ModeIrregular

	ModePerm FileMode = 0777 // Unix permission bits
)

func (m FileMode) String() string {
	const str = "dalTLDpSugct?"
	var buf [32]byte // Mode is uint32.
	w := 0
	for i, c := range str {
		if m&(1<<uint(32-1-i)) != 0 {
			buf[w] = byte(c)
			w++
		}
	}
	if w == 0 {
		buf[w] = '-'
		w++
	}
	const rwx = "rwxrwxrwx"
	for i, c := range rwx {
		if m&(1<<uint(9-1-i)) != 0 {
			buf[w] = byte(c)
		} else {
			buf[w] = '-'
		}
		w++
	}
	return string(buf[:w])
}

// IsDir reports whether m describes a directory.
// That is, it tests for the [ModeDir] bit being set in m.
func (m FileMode) IsDir() bool {
	return m&ModeDir != 0
}

// IsRegular reports whether m describes a regular file.
// That is, it tests that no mode type bits are set.
func (m FileMode) IsRegular() bool {
	return m&ModeType == 0
}

// Perm returns the Unix permission bits in m (m & [ModePerm]).
func (m FileMode) Perm() FileMode {
	return m & ModePerm
}

// Type returns type bits in m (m & [ModeType]).
func (m FileMode) Type() FileMode {
	return m & ModeType
}

// PathError records an error and the operation and file path that caused it.
type PathError struct {
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string { return e.Op + " " + e.Path + ": " + e.Err.Error() }

func (e *PathError) Unwrap() error { return e.Err }

// Timeout reports whether this error represents a timeout.
func (e *PathError) Timeout() bool {
	t, ok := e.Err.(interface{ Timeout() bool })
	return ok && t.Timeout()
}

```

// === FILE: references/go/src/io/fs/glob.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"path"
)

// A GlobFS is a file system with a Glob method.
type GlobFS interface {
	FS

	// Glob returns the names of all files matching pattern,
	// providing an implementation of the top-level
	// Glob function.
	Glob(pattern string) ([]string, error)
}

// Glob returns the names of all files matching pattern or nil
// if there is no matching file. The syntax of patterns is the same
// as in [path.Match]. The pattern may describe hierarchical names such as
// usr/*/bin/ed.
//
// Glob ignores file system errors such as I/O errors reading directories.
// The only possible returned error is [path.ErrBadPattern], reporting that
// the pattern is malformed.
//
// If fsys implements [GlobFS], Glob calls fsys.Glob.
// Otherwise, Glob uses [ReadDir] to traverse the directory tree
// and look for matches for the pattern.
func Glob(fsys FS, pattern string) (matches []string, err error) {
	return globWithLimit(fsys, pattern, 0)
}

func globWithLimit(fsys FS, pattern string, depth int) (matches []string, err error) {
	// This limit is added to prevent stack exhaustion issues. See
	// CVE-2022-30630.
	const pathSeparatorsLimit = 10000
	if depth > pathSeparatorsLimit {
		return nil, path.ErrBadPattern
	}
	if fsys, ok := fsys.(GlobFS); ok {
		return fsys.Glob(pattern)
	}

	// Check pattern is well-formed.
	if _, err := path.Match(pattern, ""); err != nil {
		return nil, err
	}
	if !hasMeta(pattern) {
		if _, err = Stat(fsys, pattern); err != nil {
			return nil, nil
		}
		return []string{pattern}, nil
	}

	dir, file := path.Split(pattern)
	dir = cleanGlobPath(dir)

	if !hasMeta(dir) {
		return glob(fsys, dir, file, nil)
	}

	// Prevent infinite recursion. See issue 15879.
	if dir == pattern {
		return nil, path.ErrBadPattern
	}

	var m []string
	m, err = globWithLimit(fsys, dir, depth+1)
	if err != nil {
		return nil, err
	}
	for _, d := range m {
		matches, err = glob(fsys, d, file, matches)
		if err != nil {
			return
		}
	}
	return
}

// cleanGlobPath prepares path for glob matching.
func cleanGlobPath(path string) string {
	switch path {
	case "":
		return "."
	default:
		return path[0 : len(path)-1] // chop off trailing separator
	}
}

// glob searches for files matching pattern in the directory dir
// and appends them to matches, returning the updated slice.
// If the directory cannot be opened, glob returns the existing matches.
// New matches are added in lexicographical order.
func glob(fs FS, dir, pattern string, matches []string) (m []string, e error) {
	m = matches
	infos, err := ReadDir(fs, dir)
	if err != nil {
		return // ignore I/O error
	}

	for _, info := range infos {
		n := info.Name()
		matched, err := path.Match(pattern, n)
		if err != nil {
			return m, err
		}
		if matched {
			m = append(m, path.Join(dir, n))
		}
	}
	return
}

// hasMeta reports whether path contains any of the magic characters
// recognized by path.Match.
func hasMeta(path string) bool {
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '*', '?', '[', '\\':
			return true
		}
	}
	return false
}

```

// === FILE: references/go/src/io/fs/readdir.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"errors"
	"internal/bytealg"
	"slices"
)

// ReadDirFS is the interface implemented by a file system
// that provides an optimized implementation of [ReadDir].
type ReadDirFS interface {
	FS

	// ReadDir reads the named directory
	// and returns a list of directory entries sorted by filename.
	ReadDir(name string) ([]DirEntry, error)
}

// ReadDir reads the named directory
// and returns a list of directory entries sorted by filename.
//
// If fsys implements [ReadDirFS], ReadDir calls fsys.ReadDir.
// Otherwise ReadDir calls fsys.Open and uses ReadDir and Close
// on the returned [ReadDirFile].
func ReadDir(fsys FS, name string) ([]DirEntry, error) {
	if fsys, ok := fsys.(ReadDirFS); ok {
		return fsys.ReadDir(name)
	}

	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dir, ok := file.(ReadDirFile)
	if !ok {
		return nil, &PathError{Op: "readdir", Path: name, Err: errors.New("not implemented")}
	}

	list, err := dir.ReadDir(-1)
	slices.SortFunc(list, func(a, b DirEntry) int {
		return bytealg.CompareString(a.Name(), b.Name())
	})
	return list, err
}

// dirInfo is a DirEntry based on a FileInfo.
type dirInfo struct {
	fileInfo FileInfo
}

func (di dirInfo) IsDir() bool {
	return di.fileInfo.IsDir()
}

func (di dirInfo) Type() FileMode {
	return di.fileInfo.Mode().Type()
}

func (di dirInfo) Info() (FileInfo, error) {
	return di.fileInfo, nil
}

func (di dirInfo) Name() string {
	return di.fileInfo.Name()
}

func (di dirInfo) String() string {
	return FormatDirEntry(di)
}

// FileInfoToDirEntry returns a [DirEntry] that returns information from info.
// If info is nil, FileInfoToDirEntry returns nil.
func FileInfoToDirEntry(info FileInfo) DirEntry {
	if info == nil {
		return nil
	}
	return dirInfo{fileInfo: info}
}

```

// === FILE: references/go/src/io/fs/readfile.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import "io"

// ReadFileFS is the interface implemented by a file system
// that provides an optimized implementation of [ReadFile].
type ReadFileFS interface {
	FS

	// ReadFile reads the named file and returns its contents.
	// A successful call returns a nil error, not io.EOF.
	// (Because ReadFile reads the whole file, the expected EOF
	// from the final Read is not treated as an error to be reported.)
	//
	// The caller is permitted to modify the returned byte slice.
	// This method should return a copy of the underlying data.
	ReadFile(name string) ([]byte, error)
}

// ReadFile reads the named file from the file system fsys and returns its contents.
// A successful call returns a nil error, not [io.EOF].
// (Because ReadFile reads the whole file, the expected EOF
// from the final Read is not treated as an error to be reported.)
//
// If fsys implements [ReadFileFS], ReadFile calls fsys.ReadFile.
// Otherwise ReadFile calls fsys.Open and uses Read and Close
// on the returned [File].
func ReadFile(fsys FS, name string) ([]byte, error) {
	if fsys, ok := fsys.(ReadFileFS); ok {
		return fsys.ReadFile(name)
	}

	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var size int
	if info, err := file.Stat(); err == nil {
		size64 := info.Size()
		if int64(int(size64)) == size64 {
			size = int(size64)
		}
	}

	data := make([]byte, 0, size+1)
	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}
		n, err := file.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}
	}
}

```

// === FILE: references/go/src/io/fs/readlink.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

// ReadLinkFS is the interface implemented by a file system
// that supports reading symbolic links.
type ReadLinkFS interface {
	FS

	// ReadLink returns the destination of the named symbolic link.
	// If there is an error, it should be of type [*PathError].
	ReadLink(name string) (string, error)

	// Lstat returns a [FileInfo] describing the named file.
	// If the file is a symbolic link, the returned [FileInfo] describes the symbolic link.
	// Lstat makes no attempt to follow the link.
	// If there is an error, it should be of type [*PathError].
	Lstat(name string) (FileInfo, error)
}

// ReadLink returns the destination of the named symbolic link.
//
// If fsys does not implement [ReadLinkFS], then ReadLink returns an error.
func ReadLink(fsys FS, name string) (string, error) {
	sym, ok := fsys.(ReadLinkFS)
	if !ok {
		return "", &PathError{Op: "readlink", Path: name, Err: ErrInvalid}
	}
	return sym.ReadLink(name)
}

// Lstat returns a [FileInfo] describing the named file.
// If the file is a symbolic link, the returned [FileInfo] describes the symbolic link.
// Lstat makes no attempt to follow the link.
//
// If fsys does not implement [ReadLinkFS], then Lstat is identical to [Stat].
func Lstat(fsys FS, name string) (FileInfo, error) {
	sym, ok := fsys.(ReadLinkFS)
	if !ok {
		return Stat(fsys, name)
	}
	return sym.Lstat(name)
}

```

// === FILE: references/go/src/io/fs/stat.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

// A StatFS is a file system with a Stat method.
type StatFS interface {
	FS

	// Stat returns a FileInfo describing the file.
	// If there is an error, it should be of type *PathError.
	Stat(name string) (FileInfo, error)
}

// Stat returns a [FileInfo] describing the named file from the file system.
//
// If fsys implements [StatFS], Stat calls fsys.Stat.
// Otherwise, Stat opens the [File] to stat it.
func Stat(fsys FS, name string) (FileInfo, error) {
	if fsys, ok := fsys.(StatFS); ok {
		return fsys.Stat(name)
	}

	file, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}

```

// === FILE: references/go/src/io/fs/sub.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"errors"
	"path"
)

// A SubFS is a file system with a Sub method.
type SubFS interface {
	FS

	// Sub returns an FS corresponding to the subtree rooted at dir.
	Sub(dir string) (FS, error)
}

// Sub returns an [FS] corresponding to the subtree rooted at fsys's dir.
//
// If dir is ".", Sub returns fsys unchanged.
// Otherwise, if fsys implements [SubFS], Sub returns fsys.Sub(dir).
// Otherwise, Sub returns a new [FS] implementation sub that,
// in effect, implements sub.Open(name) as fsys.Open(path.Join(dir, name)).
// The implementation also translates calls to ReadDir, ReadFile,
// ReadLink, Lstat, and Glob appropriately. Sub does not check if the
// directory currently exists.
//
// Note that Sub(os.DirFS("/"), "prefix") is equivalent to os.DirFS("/prefix")
// and that neither of them guarantees to avoid operating system
// accesses outside "/prefix", because the implementation of [os.DirFS]
// does not check for symbolic links inside "/prefix" that point to
// other directories. That is, [os.DirFS] is not a general substitute for a
// chroot-style security mechanism, and Sub does not change that fact.
// Use [os.Root] to constrain access to particular directory trees.
func Sub(fsys FS, dir string) (FS, error) {
	if !ValidPath(dir) {
		return nil, &PathError{Op: "sub", Path: dir, Err: ErrInvalid}
	}
	if dir == "." {
		return fsys, nil
	}
	if fsys, ok := fsys.(SubFS); ok {
		return fsys.Sub(dir)
	}
	return &subFS{fsys, dir}, nil
}

var _ FS = (*subFS)(nil)
var _ ReadDirFS = (*subFS)(nil)
var _ ReadFileFS = (*subFS)(nil)
var _ ReadLinkFS = (*subFS)(nil)
var _ GlobFS = (*subFS)(nil)

type subFS struct {
	fsys FS
	dir  string
}

// fullName maps name to the fully-qualified name dir/name.
func (f *subFS) fullName(op string, name string) (string, error) {
	if !ValidPath(name) {
		return "", &PathError{Op: op, Path: name, Err: ErrInvalid}
	}
	return path.Join(f.dir, name), nil
}

// shorten maps name, which should start with f.dir, back to the suffix after f.dir.
func (f *subFS) shorten(name string) (rel string, ok bool) {
	if name == f.dir {
		return ".", true
	}
	if len(name) >= len(f.dir)+2 && name[len(f.dir)] == '/' && name[:len(f.dir)] == f.dir {
		return name[len(f.dir)+1:], true
	}
	return "", false
}

// fixErr shortens any reported names in PathErrors by stripping f.dir.
func (f *subFS) fixErr(err error) error {
	if e, ok := err.(*PathError); ok {
		if short, ok := f.shorten(e.Path); ok {
			e.Path = short
		}
	}
	return err
}

func (f *subFS) Open(name string) (File, error) {
	full, err := f.fullName("open", name)
	if err != nil {
		return nil, err
	}
	file, err := f.fsys.Open(full)
	return file, f.fixErr(err)
}

func (f *subFS) ReadDir(name string) ([]DirEntry, error) {
	full, err := f.fullName("read", name)
	if err != nil {
		return nil, err
	}
	dir, err := ReadDir(f.fsys, full)
	return dir, f.fixErr(err)
}

func (f *subFS) ReadFile(name string) ([]byte, error) {
	full, err := f.fullName("read", name)
	if err != nil {
		return nil, err
	}
	data, err := ReadFile(f.fsys, full)
	return data, f.fixErr(err)
}

func (f *subFS) ReadLink(name string) (string, error) {
	full, err := f.fullName("readlink", name)
	if err != nil {
		return "", err
	}
	target, err := ReadLink(f.fsys, full)
	if err != nil {
		return "", f.fixErr(err)
	}
	return target, nil
}

func (f *subFS) Lstat(name string) (FileInfo, error) {
	full, err := f.fullName("lstat", name)
	if err != nil {
		return nil, err
	}
	info, err := Lstat(f.fsys, full)
	if err != nil {
		return nil, f.fixErr(err)
	}
	return info, nil
}

func (f *subFS) Glob(pattern string) ([]string, error) {
	// Check pattern is well-formed.
	if _, err := path.Match(pattern, ""); err != nil {
		return nil, err
	}
	if pattern == "." {
		return []string{"."}, nil
	}

	full := f.dir + "/" + pattern
	list, err := Glob(f.fsys, full)
	for i, name := range list {
		name, ok := f.shorten(name)
		if !ok {
			return nil, errors.New("invalid result from inner fsys Glob: " + name + " not in " + f.dir) // can't use fmt in this package
		}
		list[i] = name
	}
	return list, f.fixErr(err)
}

func (f *subFS) Sub(dir string) (FS, error) {
	if dir == "." {
		return f, nil
	}
	full, err := f.fullName("sub", dir)
	if err != nil {
		return nil, err
	}
	return &subFS{f.fsys, full}, nil
}

```

// === FILE: references/go/src/io/fs/walk.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"errors"
	"path"
)

// SkipDir is used as a return value from [WalkDirFunc] to indicate that
// the directory named in the call is to be skipped. It is not returned
// as an error by any function.
var SkipDir = errors.New("skip this directory")

// SkipAll is used as a return value from [WalkDirFunc] to indicate that
// all remaining files and directories are to be skipped. It is not returned
// as an error by any function.
var SkipAll = errors.New("skip everything and stop the walk")

// WalkDirFunc is the type of the function called by [WalkDir] to visit
// each file or directory.
//
// The path argument contains the argument to [WalkDir] as a prefix.
// That is, if WalkDir is called with root argument "dir" and finds a file
// named "a" in that directory, the walk function will be called with
// argument "dir/a".
//
// The d argument is the [DirEntry] for the named path.
//
// The error result returned by the function controls how [WalkDir]
// continues. If the function returns the special value [SkipDir], WalkDir
// skips the current directory (path if d.IsDir() is true, otherwise
// path's parent directory). If the function returns the special value
// [SkipAll], WalkDir skips all remaining files and directories. Otherwise,
// if the function returns a non-nil error, WalkDir stops entirely and
// returns that error.
//
// The err argument reports an error related to path, signaling that
// [WalkDir] will not walk into that directory. The function can decide how
// to handle that error; as described earlier, returning the error will
// cause WalkDir to stop walking the entire tree.
//
// [WalkDir] calls the function with a non-nil err argument in two cases.
//
// First, if the initial [Stat] on the root directory fails, WalkDir
// calls the function with path set to root, d set to nil, and err set to
// the error from [fs.Stat].
//
// Second, if a directory's ReadDir method (see [ReadDirFile]) fails, WalkDir calls the
// function with path set to the directory's path, d set to an
// [DirEntry] describing the directory, and err set to the error from
// ReadDir. In this second case, the function is called twice with the
// path of the directory: the first call is before the directory read is
// attempted and has err set to nil, giving the function a chance to
// return [SkipDir] or [SkipAll] and avoid the ReadDir entirely. The second call
// is after a failed ReadDir and reports the error from ReadDir.
// (If ReadDir succeeds, there is no second call.)
//
// The differences between WalkDirFunc compared to [path/filepath.WalkFunc] are:
//
//   - The second argument has type [DirEntry] instead of [FileInfo].
//   - The function is called before reading a directory, to allow [SkipDir]
//     or [SkipAll] to bypass the directory read entirely or skip all remaining
//     files and directories respectively.
//   - If a directory read fails, the function is called a second time
//     for that directory to report the error.
type WalkDirFunc func(path string, d DirEntry, err error) error

// walkDir recursively descends path, calling walkDirFn.
func walkDir(fsys FS, name string, d DirEntry, walkDirFn WalkDirFunc) error {
	if err := walkDirFn(name, d, nil); err != nil || !d.IsDir() {
		if err == SkipDir && d.IsDir() {
			// Successfully skipped directory.
			err = nil
		}
		return err
	}

	dirs, err := ReadDir(fsys, name)
	if err != nil {
		// Second call, to report ReadDir error.
		err = walkDirFn(name, d, err)
		if err != nil {
			if err == SkipDir && d.IsDir() {
				err = nil
			}
			return err
		}
	}

	for _, d1 := range dirs {
		name1 := path.Join(name, d1.Name())
		if err := walkDir(fsys, name1, d1, walkDirFn); err != nil {
			if err == SkipDir {
				break
			}
			return err
		}
	}
	return nil
}

// WalkDir walks the file tree rooted at root, calling fn for each file or
// directory in the tree, including root.
//
// All errors that arise visiting files and directories are filtered by fn:
// see the [fs.WalkDirFunc] documentation for details.
//
// The files are walked in lexical order, which makes the output deterministic
// but requires WalkDir to read an entire directory into memory before proceeding
// to walk that directory.
//
// WalkDir does not follow symbolic links found in directories,
// but if root itself is a symbolic link, its target will be walked.
func WalkDir(fsys FS, root string, fn WalkDirFunc) error {
	info, err := Stat(fsys, root)
	if err != nil {
		err = fn(root, nil, err)
	} else {
		err = walkDir(fsys, root, FileInfoToDirEntry(info), fn)
	}
	if err == SkipDir || err == SkipAll {
		return nil
	}
	return err
}

```

// === FILE: references/go/src/io/io.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package io provides basic interfaces to I/O primitives.
// Its primary job is to wrap existing implementations of such primitives,
// such as those in package os, into shared public interfaces that
// abstract the functionality, plus some other related primitives.
//
// Because these interfaces and primitives wrap lower-level operations with
// various implementations, unless otherwise informed clients should not
// assume they are safe for parallel execution.
package io

import (
	"errors"
	"sync"
)

// Seek whence values.
const (
	SeekStart   = 0 // seek relative to the origin of the file
	SeekCurrent = 1 // seek relative to the current offset
	SeekEnd     = 2 // seek relative to the end
)

// ErrShortWrite means that a write accepted fewer bytes than requested
// but failed to return an explicit error.
var ErrShortWrite = errors.New("short write")

// errInvalidWrite means that a write returned an impossible count.
var errInvalidWrite = errors.New("invalid write result")

// ErrShortBuffer means that a read required a longer buffer than was provided.
var ErrShortBuffer = errors.New("short buffer")

// EOF is the error returned by Read when no more input is available.
// (Read must return EOF itself, not an error wrapping EOF,
// because callers will test for EOF using ==.)
// Functions should return EOF only to signal a graceful end of input.
// If the EOF occurs unexpectedly in a structured data stream,
// the appropriate error is either [ErrUnexpectedEOF] or some other error
// giving more detail.
var EOF = errors.New("EOF")

// ErrUnexpectedEOF means that EOF was encountered in the
// middle of reading a fixed-size block or data structure.
var ErrUnexpectedEOF = errors.New("unexpected EOF")

// ErrNoProgress is returned by some clients of a [Reader] when
// many calls to Read have failed to return any data or error,
// usually the sign of a broken [Reader] implementation.
var ErrNoProgress = errors.New("multiple Read calls return no data or error")

// Reader is the interface that wraps the basic Read method.
//
// Read reads up to len(p) bytes into p. It returns the number of bytes
// read (0 <= n <= len(p)) and any error encountered. Even if Read
// returns n < len(p), it may use all of p as scratch space during the call.
// If some data is available but not len(p) bytes, Read conventionally
// returns what is available instead of waiting for more.
//
// When Read encounters an error or end-of-file condition after
// successfully reading n > 0 bytes, it returns the number of
// bytes read. It may return the (non-nil) error from the same call
// or return the error (and n == 0) from a subsequent call.
// An instance of this general case is that a Reader returning
// a non-zero number of bytes at the end of the input stream may
// return either err == EOF or err == nil. The next Read should
// return 0, EOF.
//
// Callers should always process the n > 0 bytes returned before
// considering the error err. Doing so correctly handles I/O errors
// that happen after reading some bytes and also both of the
// allowed EOF behaviors.
//
// If len(p) == 0, Read should always return n == 0. It may return a
// non-nil error if some error condition is known, such as EOF.
//
// Implementations of Read are discouraged from returning a
// zero byte count with a nil error, except when len(p) == 0.
// Callers should treat a return of 0 and nil as indicating that
// nothing happened; in particular it does not indicate EOF.
//
// Implementations must not retain p.
type Reader interface {
	Read(p []byte) (n int, err error)
}

// Writer is the interface that wraps the basic Write method.
//
// Write writes len(p) bytes from p to the underlying data stream.
// It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
// Write must return a non-nil error if it returns n < len(p).
// Write must not modify the slice data, even temporarily.
//
// Implementations must not retain p.
type Writer interface {
	Write(p []byte) (n int, err error)
}

// Closer is the interface that wraps the basic Close method.
//
// The behavior of Close after the first call is undefined.
// Specific implementations may document their own behavior.
type Closer interface {
	Close() error
}

// Seeker is the interface that wraps the basic Seek method.
//
// Seek sets the offset for the next Read or Write to offset,
// interpreted according to whence:
// [SeekStart] means relative to the start of the file,
// [SeekCurrent] means relative to the current offset, and
// [SeekEnd] means relative to the end
// (for example, offset = -2 specifies the penultimate byte of the file).
// Seek returns the new offset relative to the start of the
// file or an error, if any.
//
// Seeking to an offset before the start of the file is an error.
// Seeking to any positive offset may be allowed, but if the new offset exceeds
// the size of the underlying object the behavior of subsequent I/O operations
// is implementation-dependent.
type Seeker interface {
	Seek(offset int64, whence int) (int64, error)
}

// ReadWriter is the interface that groups the basic Read and Write methods.
type ReadWriter interface {
	Reader
	Writer
}

// ReadCloser is the interface that groups the basic Read and Close methods.
type ReadCloser interface {
	Reader
	Closer
}

// WriteCloser is the interface that groups the basic Write and Close methods.
type WriteCloser interface {
	Writer
	Closer
}

// ReadWriteCloser is the interface that groups the basic Read, Write and Close methods.
type ReadWriteCloser interface {
	Reader
	Writer
	Closer
}

// ReadSeeker is the interface that groups the basic Read and Seek methods.
type ReadSeeker interface {
	Reader
	Seeker
}

// ReadSeekCloser is the interface that groups the basic Read, Seek and Close
// methods.
type ReadSeekCloser interface {
	Reader
	Seeker
	Closer
}

// WriteSeeker is the interface that groups the basic Write and Seek methods.
type WriteSeeker interface {
	Writer
	Seeker
}

// ReadWriteSeeker is the interface that groups the basic Read, Write and Seek methods.
type ReadWriteSeeker interface {
	Reader
	Writer
	Seeker
}

// ReaderFrom is the interface that wraps the ReadFrom method.
//
// ReadFrom reads data from r until EOF or error.
// The return value n is the number of bytes read.
// Any error except EOF encountered during the read is also returned.
//
// The [Copy] function uses [ReaderFrom] if available.
type ReaderFrom interface {
	ReadFrom(r Reader) (n int64, err error)
}

// WriterTo is the interface that wraps the WriteTo method.
//
// WriteTo writes data to w until there's no more data to write or
// when an error occurs. The return value n is the number of bytes
// written. Any error encountered during the write is also returned.
//
// The Copy function uses WriterTo if available.
type WriterTo interface {
	WriteTo(w Writer) (n int64, err error)
}

// ReaderAt is the interface that wraps the basic ReadAt method.
//
// ReadAt reads len(p) bytes into p starting at offset off in the
// underlying input source. It returns the number of bytes
// read (0 <= n <= len(p)) and any error encountered.
//
// When ReadAt returns n < len(p), it returns a non-nil error
// explaining why more bytes were not returned. In this respect,
// ReadAt is stricter than Read.
//
// Even if ReadAt returns n < len(p), it may use all of p as scratch
// space during the call. If some data is available but not len(p) bytes,
// ReadAt blocks until either all the data is available or an error occurs.
// In this respect ReadAt is different from Read.
//
// If the n = len(p) bytes returned by ReadAt are at the end of the
// input source, ReadAt may return either err == EOF or err == nil.
//
// If ReadAt is reading from an input source with a seek offset,
// ReadAt should not affect nor be affected by the underlying
// seek offset.
//
// Clients of ReadAt can execute parallel ReadAt calls on the
// same input source.
//
// Implementations must not retain p.
type ReaderAt interface {
	ReadAt(p []byte, off int64) (n int, err error)
}

// WriterAt is the interface that wraps the basic WriteAt method.
//
// WriteAt writes len(p) bytes from p to the underlying data stream
// at offset off. It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
// WriteAt must return a non-nil error if it returns n < len(p).
//
// If WriteAt is writing to a destination with a seek offset,
// WriteAt should not affect nor be affected by the underlying
// seek offset.
//
// Clients of WriteAt can execute parallel WriteAt calls on the same
// destination if the ranges do not overlap.
//
// Implementations must not retain p.
type WriterAt interface {
	WriteAt(p []byte, off int64) (n int, err error)
}

// ByteReader is the interface that wraps the ReadByte method.
//
// ReadByte reads and returns the next byte from the input or
// any error encountered. If ReadByte returns an error, no input
// byte was consumed, and the returned byte value is undefined.
//
// ReadByte provides an efficient interface for byte-at-time
// processing. A [Reader] that does not implement  ByteReader
// can be wrapped using bufio.NewReader to add this method.
type ByteReader interface {
	ReadByte() (byte, error)
}

// ByteScanner is the interface that adds the UnreadByte method to the
// basic ReadByte method.
//
// UnreadByte causes the next call to ReadByte to return the last byte read.
// If the last operation was not a successful call to ReadByte, UnreadByte may
// return an error, unread the last byte read (or the byte prior to the
// last-unread byte), or (in implementations that support the [Seeker] interface)
// seek to one byte before the current offset.
type ByteScanner interface {
	ByteReader
	UnreadByte() error
}

// ByteWriter is the interface that wraps the WriteByte method.
type ByteWriter interface {
	WriteByte(c byte) error
}

// RuneReader is the interface that wraps the ReadRune method.
//
// ReadRune reads a single encoded Unicode character
// and returns the rune and its size in bytes. If no character is
// available, err will be set.
type RuneReader interface {
	ReadRune() (r rune, size int, err error)
}

// RuneScanner is the interface that adds the UnreadRune method to the
// basic ReadRune method.
//
// UnreadRune causes the next call to ReadRune to return the last rune read.
// If the last operation was not a successful call to ReadRune, UnreadRune may
// return an error, unread the last rune read (or the rune prior to the
// last-unread rune), or (in implementations that support the [Seeker] interface)
// seek to the start of the rune before the current offset.
type RuneScanner interface {
	RuneReader
	UnreadRune() error
}

// StringWriter is the interface that wraps the WriteString method.
type StringWriter interface {
	WriteString(s string) (n int, err error)
}

// WriteString writes the contents of the string s to w, which accepts a slice of bytes.
// If w implements [StringWriter], [StringWriter.WriteString] is invoked directly.
// Otherwise, [Writer.Write] is called exactly once.
func WriteString(w Writer, s string) (n int, err error) {
	if sw, ok := w.(StringWriter); ok {
		return sw.WriteString(s)
	}
	return w.Write([]byte(s))
}

// ReadAtLeast reads from r into buf until it has read at least min bytes.
// It returns the number of bytes copied and an error if fewer bytes were read.
// The error is EOF only if no bytes were read.
// If an EOF happens after reading fewer than min bytes,
// ReadAtLeast returns [ErrUnexpectedEOF].
// If min is greater than the length of buf, ReadAtLeast returns [ErrShortBuffer].
// On return, n >= min if and only if err == nil.
// If r returns an error having read at least min bytes, the error is dropped.
func ReadAtLeast(r Reader, buf []byte, min int) (n int, err error) {
	if len(buf) < min {
		return 0, ErrShortBuffer
	}
	for n < min && err == nil {
		var nn int
		nn, err = r.Read(buf[n:])
		n += nn
	}
	if n >= min {
		err = nil
	} else if n > 0 && err == EOF {
		err = ErrUnexpectedEOF
	}
	return
}

// ReadFull reads exactly len(buf) bytes from r into buf.
// It returns the number of bytes copied and an error if fewer bytes were read.
// The error is EOF only if no bytes were read.
// If an EOF happens after reading some but not all the bytes,
// ReadFull returns [ErrUnexpectedEOF].
// On return, n == len(buf) if and only if err == nil.
// If r returns an error having read at least len(buf) bytes, the error is dropped.
func ReadFull(r Reader, buf []byte) (n int, err error) {
	return ReadAtLeast(r, buf, len(buf))
}

// CopyN copies n bytes (or until an error) from src to dst.
// It returns the number of bytes copied and the earliest
// error encountered while copying.
// On return, written == n if and only if err == nil.
//
// If dst implements [ReaderFrom], the copy is implemented using it.
func CopyN(dst Writer, src Reader, n int64) (written int64, err error) {
	written, err = Copy(dst, LimitReader(src, n))
	if written == n {
		return n, nil
	}
	if written < n && err == nil {
		// src stopped early; must have been EOF.
		err = EOF
	}
	return
}

// Copy copies from src to dst until either EOF is reached
// on src or an error occurs. It returns the number of bytes
// copied and the first error encountered while copying, if any.
//
// A successful Copy returns err == nil, not err == EOF.
// Because Copy is defined to read from src until EOF, it does
// not treat an EOF from Read as an error to be reported.
//
// If src implements [WriterTo],
// the copy is implemented by calling src.WriteTo(dst).
// Otherwise, if dst implements [ReaderFrom],
// the copy is implemented by calling dst.ReadFrom(src).
func Copy(dst Writer, src Reader) (written int64, err error) {
	return copyBuffer(dst, src, nil)
}

// CopyBuffer is identical to Copy except that it stages through the
// provided buffer (if one is required) rather than allocating a
// temporary one. If buf is nil, one is allocated; otherwise if it has
// zero length, CopyBuffer panics.
//
// If either src implements [WriterTo] or dst implements [ReaderFrom],
// buf will not be used to perform the copy.
func CopyBuffer(dst Writer, src Reader, buf []byte) (written int64, err error) {
	if buf != nil && len(buf) == 0 {
		panic("empty buffer in CopyBuffer")
	}
	return copyBuffer(dst, src, buf)
}

// copyBuffer is the actual implementation of Copy and CopyBuffer.
// if buf is nil, one is allocated.
func copyBuffer(dst Writer, src Reader, buf []byte) (written int64, err error) {
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	if wt, ok := src.(WriterTo); ok {
		return wt.WriteTo(dst)
	}
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	if rf, ok := dst.(ReaderFrom); ok {
		return rf.ReadFrom(src)
	}
	if buf == nil {
		size := 32 * 1024
		if l, ok := src.(*LimitedReader); ok && int64(size) > l.N {
			if l.N < 1 {
				size = 1
			} else {
				size = int(l.N)
			}
		}
		buf = make([]byte, size)
	}
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errInvalidWrite
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

// LimitReader returns a Reader that reads from r
// but stops with EOF after n bytes.
// The underlying implementation is a *LimitedReader.
func LimitReader(r Reader, n int64) Reader { return &LimitedReader{r, n} }

// A LimitedReader reads from R but limits the amount of
// data returned to just N bytes. Each call to Read
// updates N to reflect the new amount remaining.
// Read returns EOF when N <= 0 or when the underlying R returns EOF.
type LimitedReader struct {
	R Reader // underlying reader
	N int64  // max bytes remaining
}

func (l *LimitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, EOF
	}
	if int64(len(p)) > l.N {
		p = p[0:l.N]
	}
	n, err = l.R.Read(p)
	l.N -= int64(n)
	return
}

// NewSectionReader returns a [SectionReader] that reads from r
// starting at offset off and stops with EOF after n bytes.
func NewSectionReader(r ReaderAt, off int64, n int64) *SectionReader {
	var remaining int64
	const maxint64 = 1<<63 - 1
	if off <= maxint64-n {
		remaining = n + off
	} else {
		// Overflow, with no way to return error.
		// Assume we can read up to an offset of 1<<63 - 1.
		remaining = maxint64
	}
	return &SectionReader{r, off, off, remaining, n}
}

// SectionReader implements Read, Seek, and ReadAt on a section
// of an underlying [ReaderAt].
type SectionReader struct {
	r     ReaderAt // constant after creation
	base  int64    // constant after creation
	off   int64
	limit int64 // constant after creation
	n     int64 // constant after creation
}

func (s *SectionReader) Read(p []byte) (n int, err error) {
	if s.off >= s.limit {
		return 0, EOF
	}
	if max := s.limit - s.off; int64(len(p)) > max {
		p = p[0:max]
	}
	n, err = s.r.ReadAt(p, s.off)
	s.off += int64(n)
	return
}

var errWhence = errors.New("Seek: invalid whence")
var errOffset = errors.New("Seek: invalid offset")

func (s *SectionReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	default:
		return 0, errWhence
	case SeekStart:
		offset += s.base
	case SeekCurrent:
		offset += s.off
	case SeekEnd:
		offset += s.limit
	}
	if offset < s.base {
		return 0, errOffset
	}
	s.off = offset
	return offset - s.base, nil
}

func (s *SectionReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 || off >= s.Size() {
		return 0, EOF
	}
	off += s.base
	if max := s.limit - off; int64(len(p)) > max {
		p = p[0:max]
		n, err = s.r.ReadAt(p, off)
		if err == nil {
			err = EOF
		}
		return n, err
	}
	return s.r.ReadAt(p, off)
}

// Size returns the size of the section in bytes.
func (s *SectionReader) Size() int64 { return s.limit - s.base }

// Outer returns the underlying [ReaderAt] and offsets for the section.
//
// The returned values are the same that were passed to [NewSectionReader]
// when the [SectionReader] was created.
func (s *SectionReader) Outer() (r ReaderAt, off int64, n int64) {
	return s.r, s.base, s.n
}

// An OffsetWriter maps writes at offset base to offset base+off in the underlying writer.
type OffsetWriter struct {
	w    WriterAt
	base int64 // the original offset
	off  int64 // the current offset
}

// NewOffsetWriter returns an [OffsetWriter] that writes to w
// starting at offset off.
func NewOffsetWriter(w WriterAt, off int64) *OffsetWriter {
	return &OffsetWriter{w, off, off}
}

func (o *OffsetWriter) Write(p []byte) (n int, err error) {
	n, err = o.w.WriteAt(p, o.off)
	o.off += int64(n)
	return
}

func (o *OffsetWriter) WriteAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errOffset
	}

	off += o.base
	return o.w.WriteAt(p, off)
}

func (o *OffsetWriter) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	default:
		return 0, errWhence
	case SeekStart:
		offset += o.base
	case SeekCurrent:
		offset += o.off
	}
	if offset < o.base {
		return 0, errOffset
	}
	o.off = offset
	return offset - o.base, nil
}

// TeeReader returns a [Reader] that writes to w what it reads from r.
// All reads from r performed through it are matched with
// corresponding writes to w. There is no internal buffering -
// the write must complete before the read completes.
// Any error encountered while writing is reported as a read error.
func TeeReader(r Reader, w Writer) Reader {
	return &teeReader{r, w}
}

type teeReader struct {
	r Reader
	w Writer
}

func (t *teeReader) Read(p []byte) (n int, err error) {
	n, err = t.r.Read(p)
	if n > 0 {
		if n, err := t.w.Write(p[:n]); err != nil {
			return n, err
		}
	}
	return
}

// Discard is a [Writer] on which all Write calls succeed
// without doing anything.
var Discard Writer = discard{}

type discard struct{}

// discard implements ReaderFrom as an optimization so Copy to
// io.Discard can avoid doing unnecessary work.
var _ ReaderFrom = discard{}

func (discard) Write(p []byte) (int, error) {
	return len(p), nil
}

func (discard) WriteString(s string) (int, error) {
	return len(s), nil
}

var blackHolePool = sync.Pool{
	New: func() any {
		b := make([]byte, 8192)
		return &b
	},
}

func (discard) ReadFrom(r Reader) (n int64, err error) {
	bufp := blackHolePool.Get().(*[]byte)
	readSize := 0
	for {
		readSize, err = r.Read(*bufp)
		n += int64(readSize)
		if err != nil {
			blackHolePool.Put(bufp)
			if err == EOF {
				return n, nil
			}
			return
		}
	}
}

// NopCloser returns a [ReadCloser] with a no-op Close method wrapping
// the provided [Reader] r.
// If r implements [WriterTo], the returned [ReadCloser] will implement [WriterTo]
// by forwarding calls to r.
func NopCloser(r Reader) ReadCloser {
	if _, ok := r.(WriterTo); ok {
		return nopCloserWriterTo{r}
	}
	return nopCloser{r}
}

type nopCloser struct {
	Reader
}

func (nopCloser) Close() error { return nil }

type nopCloserWriterTo struct {
	Reader
}

func (nopCloserWriterTo) Close() error { return nil }

func (c nopCloserWriterTo) WriteTo(w Writer) (n int64, err error) {
	return c.Reader.(WriterTo).WriteTo(w)
}

// ReadAll reads from r until an error or EOF and returns the data it read.
// A successful call returns err == nil, not err == EOF. Because ReadAll is
// defined to read from src until EOF, it does not treat an EOF from Read
// as an error to be reported.
func ReadAll(r Reader) ([]byte, error) {
	// Build slices of exponentially growing size,
	// then copy into a perfectly-sized slice at the end.
	b := make([]byte, 0, 512)
	// Starting with next equal to 256 (instead of say 512 or 1024)
	// allows less memory usage for small inputs that finish in the
	// early growth stages, but we grow the read sizes quickly such that
	// it does not materially impact medium or large inputs.
	next := 256
	chunks := make([][]byte, 0, 4)
	// Invariant: finalSize = sum(len(c) for c in chunks)
	var finalSize int
	for {
		n, err := r.Read(b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err == EOF {
				err = nil
			}
			if len(chunks) == 0 {
				return b, err
			}

			// Build our final right-sized slice.
			finalSize += len(b)
			final := append([]byte(nil), make([]byte, finalSize)...)[:0]
			for _, chunk := range chunks {
				final = append(final, chunk...)
			}
			final = append(final, b...)
			return final, err
		}

		if cap(b)-len(b) < cap(b)/16 {
			// Move to the next intermediate slice.
			chunks = append(chunks, b)
			finalSize += len(b)
			b = append([]byte(nil), make([]byte, next)...)[:0]
			next += next / 2
		}
	}
}

```

// === FILE: references/go/src/io/ioutil/ioutil.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ioutil implements some I/O utility functions.
//
// Deprecated: As of Go 1.16, the same functionality is now provided
// by package [io] or package [os], and those implementations
// should be preferred in new code.
// See the specific function documentation for details.
package ioutil

import (
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// ReadAll reads from r until an error or EOF and returns the data it read.
// A successful call returns err == nil, not err == EOF. Because ReadAll is
// defined to read from src until EOF, it does not treat an EOF from Read
// as an error to be reported.
//
// Deprecated: As of Go 1.16, this function simply calls [io.ReadAll].
//
//go:fix inline
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// ReadFile reads the file named by filename and returns the contents.
// A successful call returns err == nil, not err == EOF. Because ReadFile
// reads the whole file, it does not treat an EOF from Read as an error
// to be reported.
//
// Deprecated: As of Go 1.16, this function simply calls [os.ReadFile].
//
//go:fix inline
func ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// WriteFile writes data to a file named by filename.
// If the file does not exist, WriteFile creates it with permissions perm
// (before umask); otherwise WriteFile truncates it before writing, without changing permissions.
//
// Deprecated: As of Go 1.16, this function simply calls [os.WriteFile].
//
//go:fix inline
func WriteFile(filename string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// ReadDir reads the directory named by dirname and returns
// a list of fs.FileInfo for the directory's contents,
// sorted by filename. If an error occurs reading the directory,
// ReadDir returns no directory entries along with the error.
//
// Deprecated: As of Go 1.16, [os.ReadDir] is a more efficient and correct choice:
// it returns a list of [fs.DirEntry] instead of [fs.FileInfo],
// and it returns partial results in the case of an error
// midway through reading a directory.
//
// If you must continue obtaining a list of [fs.FileInfo], you still can:
//
//	entries, err := os.ReadDir(dirname)
//	if err != nil { ... }
//	infos := make([]fs.FileInfo, 0, len(entries))
//	for _, entry := range entries {
//		info, err := entry.Info()
//		if err != nil { ... }
//		infos = append(infos, info)
//	}
func ReadDir(dirname string) ([]fs.FileInfo, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	slices.SortFunc(list, func(a, b os.FileInfo) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return list, nil
}

// NopCloser returns a ReadCloser with a no-op Close method wrapping
// the provided Reader r.
//
// Deprecated: As of Go 1.16, this function simply calls [io.NopCloser].
//
//go:fix inline
func NopCloser(r io.Reader) io.ReadCloser {
	return io.NopCloser(r)
}

// Discard is an io.Writer on which all Write calls succeed
// without doing anything.
//
// Deprecated: As of Go 1.16, this value is simply [io.Discard].
var Discard io.Writer = io.Discard

```

// === FILE: references/go/src/io/ioutil/tempfile.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ioutil

import (
	"os"
)

// TempFile creates a new temporary file in the directory dir,
// opens the file for reading and writing, and returns the resulting *[os.File].
// The filename is generated by taking pattern and adding a random
// string to the end. If pattern includes a "*", the random string
// replaces the last "*".
// If dir is the empty string, TempFile uses the default directory
// for temporary files (see [os.TempDir]).
// Multiple programs calling TempFile simultaneously
// will not choose the same file. The caller can use f.Name()
// to find the pathname of the file. It is the caller's responsibility
// to remove the file when no longer needed.
//
// Deprecated: As of Go 1.17, this function simply calls [os.CreateTemp].
//
//go:fix inline
func TempFile(dir, pattern string) (f *os.File, err error) {
	return os.CreateTemp(dir, pattern)
}

// TempDir creates a new temporary directory in the directory dir.
// The directory name is generated by taking pattern and applying a
// random string to the end. If pattern includes a "*", the random string
// replaces the last "*". TempDir returns the name of the new directory.
// If dir is the empty string, TempDir uses the
// default directory for temporary files (see [os.TempDir]).
// Multiple programs calling TempDir simultaneously
// will not choose the same directory. It is the caller's responsibility
// to remove the directory when no longer needed.
//
// Deprecated: As of Go 1.17, this function simply calls [os.MkdirTemp].
//
//go:fix inline
func TempDir(dir, pattern string) (name string, err error) {
	return os.MkdirTemp(dir, pattern)
}

```

// === FILE: references/go/src/io/multi.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io

type eofReader struct{}

func (eofReader) Read([]byte) (int, error) {
	return 0, EOF
}

type multiReader struct {
	readers []Reader
}

func (mr *multiReader) Read(p []byte) (n int, err error) {
	for len(mr.readers) > 0 {
		// Optimization to flatten nested multiReaders (Issue 13558).
		if len(mr.readers) == 1 {
			if r, ok := mr.readers[0].(*multiReader); ok {
				mr.readers = r.readers
				continue
			}
		}
		n, err = mr.readers[0].Read(p)
		if err == EOF {
			// Use eofReader instead of nil to avoid nil panic
			// after performing flatten (Issue 18232).
			mr.readers[0] = eofReader{} // permit earlier GC
			mr.readers = mr.readers[1:]
		}
		if n > 0 || err != EOF {
			if err == EOF && len(mr.readers) > 0 {
				// Don't return EOF yet. More readers remain.
				err = nil
			}
			return
		}
	}
	return 0, EOF
}

func (mr *multiReader) WriteTo(w Writer) (sum int64, err error) {
	return mr.writeToWithBuffer(w, make([]byte, 1024*32))
}

func (mr *multiReader) writeToWithBuffer(w Writer, buf []byte) (sum int64, err error) {
	for i, r := range mr.readers {
		var n int64
		if subMr, ok := r.(*multiReader); ok { // reuse buffer with nested multiReaders
			n, err = subMr.writeToWithBuffer(w, buf)
		} else {
			n, err = copyBuffer(w, r, buf)
		}
		sum += n
		if err != nil {
			mr.readers = mr.readers[i:] // permit resume / retry after error
			return sum, err
		}
		mr.readers[i] = nil // permit early GC
	}
	mr.readers = nil
	return sum, nil
}

var _ WriterTo = (*multiReader)(nil)

// MultiReader returns a Reader that's the logical concatenation of
// the provided input readers. They're read sequentially. Once all
// inputs have returned EOF, Read will return EOF.  If any of the readers
// return a non-nil, non-EOF error, Read will return that error.
func MultiReader(readers ...Reader) Reader {
	r := make([]Reader, len(readers))
	copy(r, readers)
	return &multiReader{r}
}

type multiWriter struct {
	writers []Writer
}

func (t *multiWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = ErrShortWrite
			return
		}
	}
	return len(p), nil
}

var _ StringWriter = (*multiWriter)(nil)

func (t *multiWriter) WriteString(s string) (n int, err error) {
	var p []byte // lazily initialized if/when needed
	for _, w := range t.writers {
		if sw, ok := w.(StringWriter); ok {
			n, err = sw.WriteString(s)
		} else {
			if p == nil {
				p = []byte(s)
			}
			n, err = w.Write(p)
		}
		if err != nil {
			return
		}
		if n != len(s) {
			err = ErrShortWrite
			return
		}
	}
	return len(s), nil
}

// MultiWriter creates a writer that duplicates its writes to all the
// provided writers, similar to the Unix tee(1) command.
//
// Each write is written to each listed writer, one at a time.
// If a listed writer returns an error, that overall write operation
// stops and returns the error; it does not continue down the list.
func MultiWriter(writers ...Writer) Writer {
	allWriters := make([]Writer, 0, len(writers))
	for _, w := range writers {
		if mw, ok := w.(*multiWriter); ok {
			allWriters = append(allWriters, mw.writers...)
		} else {
			allWriters = append(allWriters, w)
		}
	}
	return &multiWriter{allWriters}
}

```

// === FILE: references/go/src/io/pipe.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Pipe adapter to connect code expecting an io.Reader
// with code expecting an io.Writer.

package io

import (
	"errors"
	"sync"
)

// onceError is an object that will only store an error once.
type onceError struct {
	sync.Mutex // guards following
	err        error
}

func (a *onceError) Store(err error) {
	a.Lock()
	defer a.Unlock()
	if a.err != nil {
		return
	}
	a.err = err
}
func (a *onceError) Load() error {
	a.Lock()
	defer a.Unlock()
	return a.err
}

// ErrClosedPipe is the error used for read or write operations on a closed pipe.
var ErrClosedPipe = errors.New("io: read/write on closed pipe")

// A pipe is the shared pipe structure underlying PipeReader and PipeWriter.
type pipe struct {
	wrMu sync.Mutex // Serializes Write operations
	wrCh chan []byte
	rdCh chan int

	once sync.Once // Protects closing done
	done chan struct{}
	rerr onceError
	werr onceError
}

func (p *pipe) read(b []byte) (n int, err error) {
	select {
	case <-p.done:
		return 0, p.readCloseError()
	default:
	}

	select {
	case bw := <-p.wrCh:
		nr := copy(b, bw)
		p.rdCh <- nr
		return nr, nil
	case <-p.done:
		return 0, p.readCloseError()
	}
}

func (p *pipe) closeRead(err error) error {
	if err == nil {
		err = ErrClosedPipe
	}
	p.rerr.Store(err)
	p.once.Do(func() { close(p.done) })
	return nil
}

func (p *pipe) write(b []byte) (n int, err error) {
	select {
	case <-p.done:
		return 0, p.writeCloseError()
	default:
		p.wrMu.Lock()
		defer p.wrMu.Unlock()
	}

	for once := true; once || len(b) > 0; once = false {
		select {
		case p.wrCh <- b:
			nw := <-p.rdCh
			b = b[nw:]
			n += nw
		case <-p.done:
			return n, p.writeCloseError()
		}
	}
	return n, nil
}

func (p *pipe) closeWrite(err error) error {
	if err == nil {
		err = EOF
	}
	p.werr.Store(err)
	p.once.Do(func() { close(p.done) })
	return nil
}

// readCloseError is considered internal to the pipe type.
func (p *pipe) readCloseError() error {
	rerr := p.rerr.Load()
	if werr := p.werr.Load(); rerr == nil && werr != nil {
		return werr
	}
	return ErrClosedPipe
}

// writeCloseError is considered internal to the pipe type.
func (p *pipe) writeCloseError() error {
	werr := p.werr.Load()
	if rerr := p.rerr.Load(); werr == nil && rerr != nil {
		return rerr
	}
	return ErrClosedPipe
}

// A PipeReader is the read half of a pipe.
type PipeReader struct{ pipe }

// Read implements the standard Read interface:
// it reads data from the pipe, blocking until a writer
// arrives or the write end is closed.
// If the write end is closed with an error, that error is
// returned as err; otherwise err is EOF.
func (r *PipeReader) Read(data []byte) (n int, err error) {
	return r.pipe.read(data)
}

// Close closes the reader; subsequent writes to the
// write half of the pipe will return the error [ErrClosedPipe].
func (r *PipeReader) Close() error {
	return r.CloseWithError(nil)
}

// CloseWithError closes the reader; subsequent writes
// to the write half of the pipe will return the error err.
//
// CloseWithError never overwrites the previous error if it exists
// and always returns nil.
func (r *PipeReader) CloseWithError(err error) error {
	return r.pipe.closeRead(err)
}

// A PipeWriter is the write half of a pipe.
type PipeWriter struct{ r PipeReader }

// Write implements the standard Write interface:
// it writes data to the pipe, blocking until one or more readers
// have consumed all the data or the read end is closed.
// If the read end is closed with an error, that err is
// returned as err; otherwise err is [ErrClosedPipe].
func (w *PipeWriter) Write(data []byte) (n int, err error) {
	return w.r.pipe.write(data)
}

// Close closes the writer; subsequent reads from the
// read half of the pipe will return no bytes and EOF.
func (w *PipeWriter) Close() error {
	return w.CloseWithError(nil)
}

// CloseWithError closes the writer; subsequent reads from the
// read half of the pipe will return no bytes and the error err,
// or EOF if err is nil.
//
// CloseWithError never overwrites the previous error if it exists
// and always returns nil.
func (w *PipeWriter) CloseWithError(err error) error {
	return w.r.pipe.closeWrite(err)
}

// Pipe creates a synchronous in-memory pipe.
// It can be used to connect code expecting an [io.Reader]
// with code expecting an [io.Writer].
//
// Reads and Writes on the pipe are matched one to one
// except when multiple Reads are needed to consume a single Write.
// That is, each Write to the [PipeWriter] blocks until it has satisfied
// one or more Reads from the [PipeReader] that fully consume
// the written data.
// The data is copied directly from the Write to the corresponding
// Read (or Reads); there is no internal buffering.
//
// It is safe to call Read and Write in parallel with each other or with Close.
// Parallel calls to Read and parallel calls to Write are also safe:
// the individual calls will be gated sequentially.
func Pipe() (*PipeReader, *PipeWriter) {
	pw := &PipeWriter{r: PipeReader{pipe: pipe{
		wrCh: make(chan []byte),
		rdCh: make(chan int),
		done: make(chan struct{}),
	}}}
	return &pw.r, pw
}

```

