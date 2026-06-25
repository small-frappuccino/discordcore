# Domain Architecture: os

## Layout Topology
```text
os/
├── exec
│   ├── internal
│   │   └── fdtest
│   │       ├── exists_plan9.go
│   │       ├── exists_unix.go
│   │       └── exists_windows.go
│   ├── exec.go
│   ├── exec_plan9.go
│   ├── exec_unix.go
│   ├── exec_windows.go
│   ├── lookpath.go
│   ├── lp_plan9.go
│   ├── lp_unix.go
│   ├── lp_wasm.go
│   ├── lp_windows.go
│   └── read3.go
├── signal
│   ├── doc.go
│   ├── sig.s
│   ├── signal.go
│   ├── signal_plan9.go
│   └── signal_unix.go
├── user
│   ├── cgo_listgroups_unix.go
│   ├── cgo_lookup_cgo.go
│   ├── cgo_lookup_syscall.go
│   ├── cgo_lookup_unix.go
│   ├── getgrouplist_syscall.go
│   ├── getgrouplist_unix.go
│   ├── listgroups_stub.go
│   ├── listgroups_unix.go
│   ├── lookup.go
│   ├── lookup_android.go
│   ├── lookup_plan9.go
│   ├── lookup_stubs.go
│   ├── lookup_unix.go
│   ├── lookup_windows.go
│   └── user.go
├── dir.go
├── dir_darwin.go
├── dir_plan9.go
├── dir_unix.go
├── dir_windows.go
├── dirent_aix.go
├── dirent_dragonfly.go
├── dirent_freebsd.go
├── dirent_js.go
├── dirent_linux.go
├── dirent_netbsd.go
├── dirent_openbsd.go
├── dirent_solaris.go
├── dirent_wasip1.go
├── eloop_netbsd.go
├── eloop_other.go
├── env.go
├── error.go
├── error_errno.go
├── error_plan9.go
├── exec.go
├── exec_linux.go
├── exec_nohandle.go
├── exec_plan9.go
├── exec_posix.go
├── exec_unix.go
├── exec_windows.go
├── executable.go
├── executable_darwin.go
├── executable_dragonfly.go
├── executable_freebsd.go
├── executable_netbsd.go
├── executable_path.go
├── executable_plan9.go
├── executable_procfs.go
├── executable_solaris.go
├── executable_sysctl.go
├── executable_wasm.go
├── executable_windows.go
├── file.go
├── file_mutex_plan9.go
├── file_open_unix.go
├── file_open_wasip1.go
├── file_plan9.go
├── file_posix.go
├── file_unix.go
├── file_wasip1.go
├── file_windows.go
├── getwd.go
├── path.go
├── path_plan9.go
├── path_unix.go
├── path_windows.go
├── pidfd_linux.go
├── pidfd_other.go
├── pipe2_unix.go
├── pipe_unix.go
├── pipe_wasm.go
├── proc.go
├── rawconn.go
├── removeall_at.go
├── removeall_noat.go
├── removeall_unix.go
├── removeall_windows.go
├── root.go
├── root_js.go
├── root_nonwindows.go
├── root_noopenat.go
├── root_openat.go
├── root_plan9.go
├── root_unix.go
├── root_windows.go
├── stat.go
├── stat_aix.go
├── stat_darwin.go
├── stat_dragonfly.go
├── stat_freebsd.go
├── stat_js.go
├── stat_linux.go
├── stat_netbsd.go
├── stat_openbsd.go
├── stat_plan9.go
├── stat_solaris.go
├── stat_unix.go
├── stat_wasip1.go
├── stat_windows.go
├── statat.go
├── statat_other.go
├── statat_unix.go
├── sticky_bsd.go
├── sticky_notbsd.go
├── sys.go
├── sys_aix.go
├── sys_bsd.go
├── sys_js.go
├── sys_linux.go
├── sys_plan9.go
├── sys_solaris.go
├── sys_unix.go
├── sys_wasip1.go
├── sys_windows.go
├── tempfile.go
├── types.go
├── types_plan9.go
├── types_unix.go
├── types_windows.go
├── wait6_dragonfly.go
├── wait6_freebsd64.go
├── wait6_freebsd_386.go
├── wait6_freebsd_arm.go
├── wait6_netbsd.go
├── wait_unimp.go
├── wait_wait6.go
├── wait_waitid.go
├── zero_copy_freebsd.go
├── zero_copy_linux.go
├── zero_copy_posix.go
├── zero_copy_solaris.go
└── zero_copy_stub.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/os/dir.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/bytealg"
	"internal/filepathlite"
	"io"
	"io/fs"
	"slices"
)

type readdirMode int

const (
	readdirName readdirMode = iota
	readdirDirEntry
	readdirFileInfo
)

// Readdir reads the contents of the directory associated with file and
// returns a slice of up to n [FileInfo] values, as would be returned
// by [Lstat], in directory order. Subsequent calls on the same file will yield
// further FileInfos.
//
// If n > 0, Readdir returns at most n FileInfo structures. In this case, if
// Readdir returns an empty slice, it will return a non-nil error
// explaining why. At the end of a directory, the error is [io.EOF].
//
// If n <= 0, Readdir returns all the FileInfo from the directory in
// a single slice. In this case, if Readdir succeeds (reads all
// the way to the end of the directory), it returns the slice and a
// nil error. If it encounters an error before the end of the
// directory, Readdir returns the FileInfo read until that point
// and a non-nil error.
//
// Most clients are better served by the more efficient ReadDir method.
func (f *File) Readdir(n int) ([]FileInfo, error) {
	if f == nil {
		return nil, ErrInvalid
	}
	_, _, infos, err := f.readdir(n, readdirFileInfo)
	if infos == nil {
		// Readdir has historically always returned a non-nil empty slice, never nil,
		// even on error (except misuse with nil receiver above).
		// Keep it that way to avoid breaking overly sensitive callers.
		infos = []FileInfo{}
	}
	return infos, err
}

// Readdirnames reads the contents of the directory associated with file
// and returns a slice of up to n names of files in the directory,
// in directory order. Subsequent calls on the same file will yield
// further names.
//
// If n > 0, Readdirnames returns at most n names. In this case, if
// Readdirnames returns an empty slice, it will return a non-nil error
// explaining why. At the end of a directory, the error is [io.EOF].
//
// If n <= 0, Readdirnames returns all the names from the directory in
// a single slice. In this case, if Readdirnames succeeds (reads all
// the way to the end of the directory), it returns the slice and a
// nil error. If it encounters an error before the end of the
// directory, Readdirnames returns the names read until that point and
// a non-nil error.
func (f *File) Readdirnames(n int) (names []string, err error) {
	if f == nil {
		return nil, ErrInvalid
	}
	names, _, _, err = f.readdir(n, readdirName)
	if names == nil {
		// Readdirnames has historically always returned a non-nil empty slice, never nil,
		// even on error (except misuse with nil receiver above).
		// Keep it that way to avoid breaking overly sensitive callers.
		names = []string{}
	}
	return names, err
}

// A DirEntry is an entry read from a directory
// (using the [ReadDir] function or a [File.ReadDir] method).
type DirEntry = fs.DirEntry

// ReadDir reads the contents of the directory associated with the file f
// and returns a slice of [DirEntry] values in directory order.
// Subsequent calls on the same file will yield later DirEntry records in the directory.
//
// If n > 0, ReadDir returns at most n DirEntry records.
// In this case, if ReadDir returns an empty slice, it will return an error explaining why.
// At the end of a directory, the error is [io.EOF].
//
// If n <= 0, ReadDir returns all the DirEntry records remaining in the directory.
// When it succeeds, it returns a nil error (not io.EOF).
func (f *File) ReadDir(n int) ([]DirEntry, error) {
	if f == nil {
		return nil, ErrInvalid
	}
	_, dirents, _, err := f.readdir(n, readdirDirEntry)
	if dirents == nil {
		// Match Readdir and Readdirnames: don't return nil slices.
		dirents = []DirEntry{}
	}
	return dirents, err
}

// ReadDir reads the named directory,
// returning all its directory entries sorted by filename.
// If an error occurs reading the directory,
// ReadDir returns the entries it was able to read before the error,
// along with the error.
func ReadDir(name string) ([]DirEntry, error) {
	f, err := openDir(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dirs, err := f.ReadDir(-1)
	slices.SortFunc(dirs, func(a, b DirEntry) int {
		return bytealg.CompareString(a.Name(), b.Name())
	})
	return dirs, err
}

// CopyFS copies the file system fsys into the directory dir,
// creating dir if necessary.
//
// Files are created with mode 0o666 plus any execute permissions
// from the source, and directories are created with mode 0o777
// (before umask).
//
// CopyFS will not overwrite existing files. If a file name in fsys
// already exists in the destination, CopyFS will return an error
// such that errors.Is(err, fs.ErrExist) will be true.
//
// Symbolic links in dir are followed.
//
// New files added to fsys (including if dir is a subdirectory of fsys)
// while CopyFS is running are not guaranteed to be copied.
//
// Copying stops at and returns the first error encountered.
func CopyFS(dir string, fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		fpath, err := filepathlite.Localize(path)
		if err != nil {
			return err
		}
		newPath := joinPath(dir, fpath)

		switch d.Type() {
		case ModeDir:
			return MkdirAll(newPath, 0777)
		case ModeSymlink:
			target, err := fs.ReadLink(fsys, path)
			if err != nil {
				return err
			}
			return Symlink(target, newPath)
		case 0:
			r, err := fsys.Open(path)
			if err != nil {
				return err
			}
			defer r.Close()
			info, err := r.Stat()
			if err != nil {
				return err
			}
			w, err := OpenFile(newPath, O_CREATE|O_EXCL|O_WRONLY, 0666|info.Mode()&0777)
			if err != nil {
				return err
			}

			if _, err := io.Copy(w, r); err != nil {
				w.Close()
				return &PathError{Op: "Copy", Path: newPath, Err: err}
			}
			return w.Close()
		default:
			return &PathError{Op: "CopyFS", Path: path, Err: ErrInvalid}
		}
	})
}

```

// === FILE: references!/go/src/os/dir_darwin.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"io"
	"runtime"
	"syscall"
	"unsafe"
)

// Auxiliary information if the File describes a directory
type dirInfo struct {
	dir uintptr // Pointer to DIR structure from dirent.h
}

func (d *dirInfo) close() {
	if d.dir == 0 {
		return
	}
	closedir(d.dir)
	d.dir = 0
}

func (f *File) readdir(n int, mode readdirMode) (names []string, dirents []DirEntry, infos []FileInfo, err error) {
	// If this file has no dirinfo, create one.
	var d *dirInfo
	for {
		d = f.dirinfo.Load()
		if d != nil {
			break
		}
		dir, call, errno := f.pfd.OpenDir()
		if errno != nil {
			return nil, nil, nil, &PathError{Op: call, Path: f.name, Err: errno}
		}
		d = &dirInfo{dir: dir}
		if f.dirinfo.CompareAndSwap(nil, d) {
			break
		}
		// We lost the race: try again.
		d.close()
	}

	size := n
	if size <= 0 {
		size = 100
		n = -1
	}

	var dirent syscall.Dirent
	var entptr *syscall.Dirent
	for len(names)+len(dirents)+len(infos) < size || n == -1 {
		if errno := readdir_r(d.dir, &dirent, &entptr); errno != 0 {
			if errno == syscall.EINTR {
				continue
			}
			return names, dirents, infos, &PathError{Op: "readdir", Path: f.name, Err: errno}
		}
		if entptr == nil { // EOF
			break
		}
		// Darwin may return a zero inode when a directory entry has been
		// deleted but not yet removed from the directory. The man page for
		// getdirentries(2) states that programs are responsible for skipping
		// those entries:
		//
		//   Users of getdirentries() should skip entries with d_fileno = 0,
		//   as such entries represent files which have been deleted but not
		//   yet removed from the directory entry.
		//
		if dirent.Ino == 0 {
			continue
		}
		name := (*[len(syscall.Dirent{}.Name)]byte)(unsafe.Pointer(&dirent.Name))[:]
		for i, c := range name {
			if c == 0 {
				name = name[:i]
				break
			}
		}
		// Check for useless names before allocating a string.
		if string(name) == "." || string(name) == ".." {
			continue
		}
		if mode == readdirName {
			names = append(names, string(name))
		} else if mode == readdirDirEntry {
			de, err := newUnixDirent(f, string(name), dtToType(dirent.Type))
			if IsNotExist(err) {
				// File disappeared between readdir and stat.
				// Treat as if it didn't exist.
				continue
			}
			if err != nil {
				return nil, dirents, nil, err
			}
			dirents = append(dirents, de)
		} else {
			info, err := f.lstatat(string(name))
			if IsNotExist(err) {
				// File disappeared between readdir + stat.
				// Treat as if it didn't exist.
				continue
			}
			if err != nil {
				return nil, nil, infos, err
			}
			infos = append(infos, info)
		}
		runtime.KeepAlive(f)
	}

	if n > 0 && len(names)+len(dirents)+len(infos) == 0 {
		return nil, nil, nil, io.EOF
	}
	return names, dirents, infos, nil
}

func dtToType(typ uint8) FileMode {
	switch typ {
	case syscall.DT_BLK:
		return ModeDevice
	case syscall.DT_CHR:
		return ModeDevice | ModeCharDevice
	case syscall.DT_DIR:
		return ModeDir
	case syscall.DT_FIFO:
		return ModeNamedPipe
	case syscall.DT_LNK:
		return ModeSymlink
	case syscall.DT_REG:
		return 0
	case syscall.DT_SOCK:
		return ModeSocket
	}
	return ^FileMode(0)
}

// Implemented in syscall/syscall_darwin.go.

//go:linkname closedir syscall.closedir
func closedir(dir uintptr) (err error)

//go:linkname readdir_r syscall.readdir_r
func readdir_r(dir uintptr, entry *syscall.Dirent, result **syscall.Dirent) (res syscall.Errno)

```

// === FILE: references!/go/src/os/dir_plan9.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"io"
	"io/fs"
	"syscall"
)

func (file *File) readdir(n int, mode readdirMode) (names []string, dirents []DirEntry, infos []FileInfo, err error) {
	var d *dirInfo
	for {
		d = file.dirinfo.Load()
		if d != nil {
			break
		}
		newD := new(dirInfo)
		if file.dirinfo.CompareAndSwap(nil, newD) {
			d = newD
			break
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	size := n
	if size <= 0 {
		size = 100
		n = -1
	}
	for n != 0 {
		// Refill the buffer if necessary.
		if d.bufp >= d.nbuf {
			nb, err := file.Read(d.buf[:])

			// Update the buffer state before checking for errors.
			d.bufp, d.nbuf = 0, nb

			if err != nil {
				if err == io.EOF {
					break
				}
				return names, dirents, infos, &PathError{Op: "readdir", Path: file.name, Err: err}
			}
			if nb < syscall.STATFIXLEN {
				return names, dirents, infos, &PathError{Op: "readdir", Path: file.name, Err: syscall.ErrShortStat}
			}
		}

		// Get a record from the buffer.
		b := d.buf[d.bufp:]
		m := int(uint16(b[0])|uint16(b[1])<<8) + 2
		if m < syscall.STATFIXLEN {
			return names, dirents, infos, &PathError{Op: "readdir", Path: file.name, Err: syscall.ErrShortStat}
		}

		dir, err := syscall.UnmarshalDir(b[:m])
		if err != nil {
			return names, dirents, infos, &PathError{Op: "readdir", Path: file.name, Err: err}
		}

		if mode == readdirName {
			names = append(names, dir.Name)
		} else {
			f := fileInfoFromStat(dir)
			if mode == readdirDirEntry {
				dirents = append(dirents, dirEntry{f})
			} else {
				infos = append(infos, f)
			}
		}
		d.bufp += m
		n--
	}

	if n > 0 && len(names)+len(dirents)+len(infos) == 0 {
		return nil, nil, nil, io.EOF
	}
	return names, dirents, infos, nil
}

type dirEntry struct {
	fs *fileStat
}

func (de dirEntry) Name() string            { return de.fs.Name() }
func (de dirEntry) IsDir() bool             { return de.fs.IsDir() }
func (de dirEntry) Type() FileMode          { return de.fs.Mode().Type() }
func (de dirEntry) Info() (FileInfo, error) { return de.fs, nil }

func (de dirEntry) String() string {
	return fs.FormatDirEntry(de)
}

```

// === FILE: references!/go/src/os/dir_unix.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || dragonfly || freebsd || (js && wasm) || wasip1 || linux || netbsd || openbsd || solaris

package os

import (
	"internal/byteorder"
	"internal/goarch"
	"io"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Auxiliary information if the File describes a directory
type dirInfo struct {
	mu   sync.Mutex
	buf  *[]byte // buffer for directory I/O
	nbuf int     // length of buf; return value from Getdirentries
	bufp int     // location of next record in buf.
}

const (
	// More than 5760 to work around https://golang.org/issue/24015.
	blockSize = 8192
)

var dirBufPool = sync.Pool{
	New: func() any {
		// The buffer must be at least a block long.
		buf := make([]byte, blockSize)
		return &buf
	},
}

func (d *dirInfo) close() {
	if d.buf != nil {
		dirBufPool.Put(d.buf)
		d.buf = nil
	}
}

func (f *File) readdir(n int, mode readdirMode) (names []string, dirents []DirEntry, infos []FileInfo, err error) {
	// If this file has no dirInfo, create one.
	var d *dirInfo
	for {
		d = f.dirinfo.Load()
		if d != nil {
			break
		}
		newD := new(dirInfo)
		if f.dirinfo.CompareAndSwap(nil, newD) {
			d = newD
			break
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.buf == nil {
		d.buf = dirBufPool.Get().(*[]byte)
	}

	// Change the meaning of n for the implementation below.
	//
	// The n above was for the public interface of "if n <= 0,
	// Readdir returns all the FileInfo from the directory in a
	// single slice".
	//
	// But below, we use only negative to mean looping until the
	// end and positive to mean bounded, with positive
	// terminating at 0.
	if n == 0 {
		n = -1
	}

	for n != 0 {
		// Refill the buffer if necessary
		if d.bufp >= d.nbuf {
			d.bufp = 0
			var errno error
			d.nbuf, errno = f.pfd.ReadDirent(*d.buf)
			runtime.KeepAlive(f)
			if errno != nil {
				return names, dirents, infos, &PathError{Op: "readdirent", Path: f.name, Err: errno}
			}
			if d.nbuf <= 0 {
				// Optimization: we can return the buffer to the pool, there is nothing else to read.
				dirBufPool.Put(d.buf)
				d.buf = nil
				break // EOF
			}
		}

		// Drain the buffer
		buf := (*d.buf)[d.bufp:d.nbuf]
		reclen, ok := direntReclen(buf)
		if !ok || reclen > uint64(len(buf)) {
			break
		}
		rec := buf[:reclen]
		d.bufp += int(reclen)
		ino, ok := direntIno(rec)
		if !ok {
			break
		}
		// When building to wasip1, the host runtime might be running on Windows
		// or might expose a remote file system which does not have the concept
		// of inodes. Therefore, we cannot make the assumption that it is safe
		// to skip entries with zero inodes.
		// Some Linux filesystems (old XFS, FUSE) can return valid files with zero inodes.
		if ino == 0 && runtime.GOOS != "linux" && runtime.GOOS != "wasip1" {
			continue
		}
		const namoff = uint64(unsafe.Offsetof(syscall.Dirent{}.Name))
		namlen, ok := direntNamlen(rec)
		if !ok || namoff+namlen > uint64(len(rec)) {
			break
		}
		name := rec[namoff : namoff+namlen]
		for i, c := range name {
			if c == 0 {
				name = name[:i]
				break
			}
		}
		// Check for useless names before allocating a string.
		if string(name) == "." || string(name) == ".." {
			continue
		}
		if n > 0 { // see 'n == 0' comment above
			n--
		}
		if mode == readdirName {
			names = append(names, string(name))
		} else if mode == readdirDirEntry {
			de, err := newUnixDirent(f, string(name), direntType(rec))
			if IsNotExist(err) {
				// File disappeared between readdir and stat.
				// Treat as if it didn't exist.
				continue
			}
			if err != nil {
				return nil, dirents, nil, err
			}
			dirents = append(dirents, de)
		} else {
			info, err := f.lstatat(string(name))
			if IsNotExist(err) {
				// File disappeared between readdir + stat.
				// Treat as if it didn't exist.
				continue
			}
			if err != nil {
				return nil, nil, infos, err
			}
			infos = append(infos, info)
		}
	}

	if n > 0 && len(names)+len(dirents)+len(infos) == 0 {
		return nil, nil, nil, io.EOF
	}
	return names, dirents, infos, nil
}

// readInt returns the size-bytes unsigned integer in native byte order at offset off.
func readInt(b []byte, off, size uintptr) (u uint64, ok bool) {
	if len(b) < int(off+size) {
		return 0, false
	}
	if goarch.BigEndian {
		return readIntBE(b[off:], size), true
	}
	return readIntLE(b[off:], size), true
}

func readIntBE(b []byte, size uintptr) uint64 {
	switch size {
	case 1:
		return uint64(b[0])
	case 2:
		return uint64(byteorder.BEUint16(b))
	case 4:
		return uint64(byteorder.BEUint32(b))
	case 8:
		return uint64(byteorder.BEUint64(b))
	default:
		panic("syscall: readInt with unsupported size")
	}
}

func readIntLE(b []byte, size uintptr) uint64 {
	switch size {
	case 1:
		return uint64(b[0])
	case 2:
		return uint64(byteorder.LEUint16(b))
	case 4:
		return uint64(byteorder.LEUint32(b))
	case 8:
		return uint64(byteorder.LEUint64(b))
	default:
		panic("syscall: readInt with unsupported size")
	}
}

```

// === FILE: references!/go/src/os/dir_windows.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/syscall/windows"
	"io"
	"io/fs"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Auxiliary information if the File describes a directory
type dirInfo struct {
	mu sync.Mutex
	// buf is a slice pointer so the slice header
	// does not escape to the heap when returning
	// buf to dirBufPool.
	buf   *[]byte // buffer for directory I/O
	bufp  int     // location of next record in buf
	h     syscall.Handle
	vol   uint32
	class uint32 // type of entries in buf
	path  string // absolute directory path, empty if the file system supports FILE_ID_BOTH_DIR_INFO
}

const (
	// dirBufSize is the size of the dirInfo buffer.
	// The buffer must be big enough to hold at least a single entry.
	// The filename alone can be 512 bytes (MAX_PATH*2), and the fixed part of
	// the FILE_ID_BOTH_DIR_INFO structure is 105 bytes, so dirBufSize
	// should not be set below 1024 bytes (512+105+safety buffer).
	// Windows 8.1 and earlier only works with buffer sizes up to 64 kB.
	dirBufSize = 64 * 1024 // 64kB
)

var dirBufPool = sync.Pool{
	New: func() any {
		// The buffer must be at least a block long.
		buf := make([]byte, dirBufSize)
		return &buf
	},
}

func (d *dirInfo) close() {
	d.h = 0
	if d.buf != nil {
		dirBufPool.Put(d.buf)
		d.buf = nil
	}
}

// allowReadDirFileID indicates whether File.readdir should try to use FILE_ID_BOTH_DIR_INFO
// if the underlying file system supports it.
// Useful for testing purposes.
var allowReadDirFileID = true

func (d *dirInfo) init(h syscall.Handle) {
	d.h = h
	d.class = windows.FileFullDirectoryRestartInfo
	// The previous settings are enough to read the directory entries.
	// The following code is only needed to support os.SameFile.

	// It is safe to query d.vol once and reuse the value.
	// Hard links are not allowed to reference files in other volumes.
	// Junctions and symbolic links can reference files and directories in other volumes,
	// but the reparse point should still live in the parent volume.
	var flags uint32
	err := windows.GetVolumeInformationByHandle(h, nil, 0, &d.vol, nil, &flags, nil, 0)
	if err != nil {
		d.vol = 0 // Set to zero in case Windows writes garbage to it.
		// If we can't get the volume information, we can't use os.SameFile,
		// but we can still read the directory entries.
		return
	}
	if flags&windows.FILE_SUPPORTS_OBJECT_IDS == 0 {
		// The file system does not support object IDs, no need to continue.
		return
	}
	if allowReadDirFileID && flags&windows.FILE_SUPPORTS_OPEN_BY_FILE_ID != 0 {
		// Use FileIdBothDirectoryRestartInfo if available as it returns the file ID
		// without the need to open the file.
		d.class = windows.FileIdBothDirectoryRestartInfo
	} else {
		// If FileIdBothDirectoryRestartInfo is not available but objects IDs are supported,
		// get the directory path so that os.SameFile can use it to open the file
		// and retrieve the file ID.
		d.path, _ = windows.FinalPath(h, windows.FILE_NAME_OPENED)
	}
}

func (file *File) readdir(n int, mode readdirMode) (names []string, dirents []DirEntry, infos []FileInfo, err error) {
	// If this file has no dirInfo, create one.
	var d *dirInfo
	for {
		d = file.dirinfo.Load()
		if d != nil {
			break
		}
		d = new(dirInfo)
		d.init(file.pfd.Sysfd)
		if file.dirinfo.CompareAndSwap(nil, d) {
			break
		}
		// We lost the race: try again.
		d.close()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.buf == nil {
		d.buf = dirBufPool.Get().(*[]byte)
	}

	wantAll := n <= 0
	if wantAll {
		n = -1
	}
	for n != 0 {
		// Refill the buffer if necessary
		if d.bufp == 0 {
			err = windows.GetFileInformationByHandleEx(file.pfd.Sysfd, d.class, (*byte)(unsafe.Pointer(&(*d.buf)[0])), uint32(len(*d.buf)))
			runtime.KeepAlive(file)
			if err != nil {
				if err == syscall.ERROR_NO_MORE_FILES {
					// Optimization: we can return the buffer to the pool, there is nothing else to read.
					dirBufPool.Put(d.buf)
					d.buf = nil
					break
				}
				if err == syscall.ERROR_FILE_NOT_FOUND &&
					(d.class == windows.FileIdBothDirectoryRestartInfo || d.class == windows.FileFullDirectoryRestartInfo) {
					// GetFileInformationByHandleEx doesn't document the return error codes when the info class is FileIdBothDirectoryRestartInfo,
					// but MS-FSA 2.1.5.6.3 [1] specifies that the underlying file system driver should return STATUS_NO_SUCH_FILE when
					// reading an empty root directory, which is mapped to ERROR_FILE_NOT_FOUND by Windows.
					// Note that some file system drivers may never return this error code, as the spec allows to return the "." and ".."
					// entries in such cases, making the directory appear non-empty.
					// The chances of false positive are very low, as we know that the directory exists, else GetVolumeInformationByHandle
					// would have failed, and that the handle is still valid, as we haven't closed it.
					// See go.dev/issue/61159.
					// [1] https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fsa/fa8194e0-53ec-413b-8315-e8fa85396fd8
					break
				}
				if s, _ := file.Stat(); s != nil && !s.IsDir() {
					err = &PathError{Op: "readdir", Path: file.name, Err: syscall.ENOTDIR}
				} else {
					err = &PathError{Op: "GetFileInformationByHandleEx", Path: file.name, Err: err}
				}
				return
			}
			if d.class == windows.FileIdBothDirectoryRestartInfo {
				d.class = windows.FileIdBothDirectoryInfo
			} else if d.class == windows.FileFullDirectoryRestartInfo {
				d.class = windows.FileFullDirectoryInfo
			}
		}
		// Drain the buffer
		var islast bool
		for n != 0 && !islast {
			var nextEntryOffset uint32
			var nameslice []uint16
			entry := unsafe.Pointer(&(*d.buf)[d.bufp])
			if d.class == windows.FileIdBothDirectoryInfo {
				info := (*windows.FILE_ID_BOTH_DIR_INFO)(entry)
				nextEntryOffset = info.NextEntryOffset
				nameslice = unsafe.Slice(&info.FileName[0], info.FileNameLength/2)
			} else {
				info := (*windows.FILE_FULL_DIR_INFO)(entry)
				nextEntryOffset = info.NextEntryOffset
				nameslice = unsafe.Slice(&info.FileName[0], info.FileNameLength/2)
			}
			d.bufp += int(nextEntryOffset)
			islast = nextEntryOffset == 0
			if islast {
				d.bufp = 0
			}
			if (len(nameslice) == 1 && nameslice[0] == '.') ||
				(len(nameslice) == 2 && nameslice[0] == '.' && nameslice[1] == '.') {
				// Ignore "." and ".." and avoid allocating a string for them.
				continue
			}
			name := syscall.UTF16ToString(nameslice)
			if mode == readdirName {
				names = append(names, name)
			} else {
				var f *fileStat
				if d.class == windows.FileIdBothDirectoryInfo {
					f = newFileStatFromFileIDBothDirInfo((*windows.FILE_ID_BOTH_DIR_INFO)(entry))
				} else {
					f = newFileStatFromFileFullDirInfo((*windows.FILE_FULL_DIR_INFO)(entry))
					if d.path != "" {
						// Defer appending the entry name to the parent directory path until
						// it is really needed, to avoid allocating a string that may not be used.
						// It is currently only used in os.SameFile.
						f.appendNameToPath = true
						f.path = d.path
					}
				}
				f.name = name
				f.vol = d.vol
				if mode == readdirDirEntry {
					dirents = append(dirents, dirEntry{f})
				} else {
					infos = append(infos, f)
				}
			}
			n--
		}
	}
	if !wantAll && len(names)+len(dirents)+len(infos) == 0 {
		return nil, nil, nil, io.EOF
	}
	return names, dirents, infos, nil
}

type dirEntry struct {
	fs *fileStat
}

func (de dirEntry) Name() string            { return de.fs.Name() }
func (de dirEntry) IsDir() bool             { return de.fs.IsDir() }
func (de dirEntry) Type() FileMode          { return de.fs.Mode().Type() }
func (de dirEntry) Info() (FileInfo, error) { return de.fs, nil }

func (de dirEntry) String() string {
	return fs.FormatDirEntry(de)
}

```

// === FILE: references!/go/src/os/dirent_aix.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

func direntIno(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Ino), unsafe.Sizeof(syscall.Dirent{}.Ino))
}

func direntReclen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen))
}

func direntNamlen(buf []byte) (uint64, bool) {
	reclen, ok := direntReclen(buf)
	if !ok {
		return 0, false
	}
	return reclen - uint64(unsafe.Offsetof(syscall.Dirent{}.Name)), true
}

func direntType(buf []byte) FileMode {
	return ^FileMode(0) // unknown
}

```

// === FILE: references!/go/src/os/dirent_dragonfly.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

func direntIno(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Fileno), unsafe.Sizeof(syscall.Dirent{}.Fileno))
}

func direntReclen(buf []byte) (uint64, bool) {
	namlen, ok := direntNamlen(buf)
	if !ok {
		return 0, false
	}
	return (16 + namlen + 1 + 7) &^ 7, true
}

func direntNamlen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Namlen), unsafe.Sizeof(syscall.Dirent{}.Namlen))
}

func direntType(buf []byte) FileMode {
	off := unsafe.Offsetof(syscall.Dirent{}.Type)
	if off >= uintptr(len(buf)) {
		return ^FileMode(0) // unknown
	}
	typ := buf[off]
	switch typ {
	case syscall.DT_BLK:
		return ModeDevice
	case syscall.DT_CHR:
		return ModeDevice | ModeCharDevice
	case syscall.DT_DBF:
		// DT_DBF is "database record file".
		// fillFileStatFromSys treats as regular file.
		return 0
	case syscall.DT_DIR:
		return ModeDir
	case syscall.DT_FIFO:
		return ModeNamedPipe
	case syscall.DT_LNK:
		return ModeSymlink
	case syscall.DT_REG:
		return 0
	case syscall.DT_SOCK:
		return ModeSocket
	}
	return ^FileMode(0) // unknown
}

```

// === FILE: references!/go/src/os/dirent_freebsd.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

func direntIno(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Fileno), unsafe.Sizeof(syscall.Dirent{}.Fileno))
}

func direntReclen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen))
}

func direntNamlen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Namlen), unsafe.Sizeof(syscall.Dirent{}.Namlen))
}

func direntType(buf []byte) FileMode {
	off := unsafe.Offsetof(syscall.Dirent{}.Type)
	if off >= uintptr(len(buf)) {
		return ^FileMode(0) // unknown
	}
	typ := buf[off]
	switch typ {
	case syscall.DT_BLK:
		return ModeDevice
	case syscall.DT_CHR:
		return ModeDevice | ModeCharDevice
	case syscall.DT_DIR:
		return ModeDir
	case syscall.DT_FIFO:
		return ModeNamedPipe
	case syscall.DT_LNK:
		return ModeSymlink
	case syscall.DT_REG:
		return 0
	case syscall.DT_SOCK:
		return ModeSocket
	}
	return ^FileMode(0) // unknown
}

```

// === FILE: references!/go/src/os/dirent_js.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

func direntIno(buf []byte) (uint64, bool) {
	return 1, true
}

func direntReclen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen))
}

func direntNamlen(buf []byte) (uint64, bool) {
	reclen, ok := direntReclen(buf)
	if !ok {
		return 0, false
	}
	return reclen - uint64(unsafe.Offsetof(syscall.Dirent{}.Name)), true
}

func direntType(buf []byte) FileMode {
	return ^FileMode(0) // unknown
}

```

// === FILE: references!/go/src/os/dirent_linux.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

func direntIno(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Ino), unsafe.Sizeof(syscall.Dirent{}.Ino))
}

func direntReclen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen))
}

func direntNamlen(buf []byte) (uint64, bool) {
	reclen, ok := direntReclen(buf)
	if !ok {
		return 0, false
	}
	return reclen - uint64(unsafe.Offsetof(syscall.Dirent{}.Name)), true
}

func direntType(buf []byte) FileMode {
	off := unsafe.Offsetof(syscall.Dirent{}.Type)
	if off >= uintptr(len(buf)) {
		return ^FileMode(0) // unknown
	}
	typ := buf[off]
	switch typ {
	case syscall.DT_BLK:
		return ModeDevice
	case syscall.DT_CHR:
		return ModeDevice | ModeCharDevice
	case syscall.DT_DIR:
		return ModeDir
	case syscall.DT_FIFO:
		return ModeNamedPipe
	case syscall.DT_LNK:
		return ModeSymlink
	case syscall.DT_REG:
		return 0
	case syscall.DT_SOCK:
		return ModeSocket
	}
	return ^FileMode(0) // unknown
}

```

// === FILE: references!/go/src/os/dirent_netbsd.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

func direntIno(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Fileno), unsafe.Sizeof(syscall.Dirent{}.Fileno))
}

func direntReclen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen))
}

func direntNamlen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Namlen), unsafe.Sizeof(syscall.Dirent{}.Namlen))
}

func direntType(buf []byte) FileMode {
	off := unsafe.Offsetof(syscall.Dirent{}.Type)
	if off >= uintptr(len(buf)) {
		return ^FileMode(0) // unknown
	}
	typ := buf[off]
	switch typ {
	case syscall.DT_BLK:
		return ModeDevice
	case syscall.DT_CHR:
		return ModeDevice | ModeCharDevice
	case syscall.DT_DIR:
		return ModeDir
	case syscall.DT_FIFO:
		return ModeNamedPipe
	case syscall.DT_LNK:
		return ModeSymlink
	case syscall.DT_REG:
		return 0
	case syscall.DT_SOCK:
		return ModeSocket
	}
	return ^FileMode(0) // unknown
}

```

// === FILE: references!/go/src/os/dirent_openbsd.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

func direntIno(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Fileno), unsafe.Sizeof(syscall.Dirent{}.Fileno))
}

func direntReclen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen))
}

func direntNamlen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Namlen), unsafe.Sizeof(syscall.Dirent{}.Namlen))
}

func direntType(buf []byte) FileMode {
	off := unsafe.Offsetof(syscall.Dirent{}.Type)
	if off >= uintptr(len(buf)) {
		return ^FileMode(0) // unknown
	}
	typ := buf[off]
	switch typ {
	case syscall.DT_BLK:
		return ModeDevice
	case syscall.DT_CHR:
		return ModeDevice | ModeCharDevice
	case syscall.DT_DIR:
		return ModeDir
	case syscall.DT_FIFO:
		return ModeNamedPipe
	case syscall.DT_LNK:
		return ModeSymlink
	case syscall.DT_REG:
		return 0
	case syscall.DT_SOCK:
		return ModeSocket
	}
	return ^FileMode(0) // unknown
}

```

// === FILE: references!/go/src/os/dirent_solaris.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

func direntIno(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Ino), unsafe.Sizeof(syscall.Dirent{}.Ino))
}

func direntReclen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Reclen), unsafe.Sizeof(syscall.Dirent{}.Reclen))
}

func direntNamlen(buf []byte) (uint64, bool) {
	reclen, ok := direntReclen(buf)
	if !ok {
		return 0, false
	}
	return reclen - uint64(unsafe.Offsetof(syscall.Dirent{}.Name)), true
}

func direntType(buf []byte) FileMode {
	return ^FileMode(0) // unknown
}

```

// === FILE: references!/go/src/os/dirent_wasip1.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build wasip1

package os

import (
	"syscall"
	"unsafe"
)

// https://github.com/WebAssembly/WASI/blob/main/legacy/preview1/docs.md#-dirent-record
const sizeOfDirent = 24

func direntIno(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Ino), unsafe.Sizeof(syscall.Dirent{}.Ino))
}

func direntReclen(buf []byte) (uint64, bool) {
	namelen, ok := direntNamlen(buf)
	return sizeOfDirent + namelen, ok
}

func direntNamlen(buf []byte) (uint64, bool) {
	return readInt(buf, unsafe.Offsetof(syscall.Dirent{}.Namlen), unsafe.Sizeof(syscall.Dirent{}.Namlen))
}

func direntType(buf []byte) FileMode {
	off := unsafe.Offsetof(syscall.Dirent{}.Type)
	if off >= uintptr(len(buf)) {
		return ^FileMode(0) // unknown
	}
	switch syscall.Filetype(buf[off]) {
	case syscall.FILETYPE_BLOCK_DEVICE:
		return ModeDevice
	case syscall.FILETYPE_CHARACTER_DEVICE:
		return ModeDevice | ModeCharDevice
	case syscall.FILETYPE_DIRECTORY:
		return ModeDir
	case syscall.FILETYPE_REGULAR_FILE:
		return 0
	case syscall.FILETYPE_SOCKET_DGRAM:
		return ModeSocket
	case syscall.FILETYPE_SOCKET_STREAM:
		return ModeSocket
	case syscall.FILETYPE_SYMBOLIC_LINK:
		return ModeSymlink
	}
	return ^FileMode(0) // unknown
}

```

// === FILE: references!/go/src/os/eloop_netbsd.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build netbsd

package os

import "syscall"

// isNoFollowErr reports whether err may result from O_NOFOLLOW blocking an open operation.
func isNoFollowErr(err error) bool {
	// NetBSD returns EFTYPE, but check the other possibilities as well.
	switch err {
	case syscall.ELOOP, syscall.EMLINK, syscall.EFTYPE:
		return true
	}
	return false
}

```

// === FILE: references!/go/src/os/eloop_other.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || darwin || dragonfly || freebsd || linux || openbsd || solaris || wasip1

package os

import (
	"runtime"
	"syscall"
)

// isNoFollowErr reports whether err may result from O_NOFOLLOW blocking an open operation.
func isNoFollowErr(err error) bool {
	switch err {
	case syscall.ELOOP, syscall.EMLINK:
		return true
	}
	if runtime.GOOS == "dragonfly" {
		// Dragonfly appears to return EINVAL from openat in this case.
		if err == syscall.EINVAL {
			return true
		}
	}
	return false
}

```

// === FILE: references!/go/src/os/env.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// General environment variables.

package os

import (
	"internal/testlog"
	"syscall"
)

// Expand replaces ${var} or $var in the string based on the mapping function.
// For example, [os.ExpandEnv](s) is equivalent to [os.Expand](s, [os.Getenv]).
func Expand(s string, mapping func(string) string) string {
	var buf []byte
	// ${} is all ASCII, so bytes are fine for this operation.
	i := 0
	for j := 0; j < len(s); j++ {
		if s[j] == '$' && j+1 < len(s) {
			if buf == nil {
				buf = make([]byte, 0, 2*len(s))
			}
			buf = append(buf, s[i:j]...)
			name, w := getShellName(s[j+1:])
			if name == "" && w > 0 {
				// Encountered invalid syntax; eat the
				// characters.
			} else if name == "" {
				// Valid syntax, but $ was not followed by a
				// name. Leave the dollar character untouched.
				buf = append(buf, s[j])
			} else {
				buf = append(buf, mapping(name)...)
			}
			j += w
			i = j + 1
		}
	}
	if buf == nil {
		return s
	}
	return string(buf) + s[i:]
}

// ExpandEnv replaces ${var} or $var in the string according to the values
// of the current environment variables. References to undefined
// variables are replaced by the empty string.
func ExpandEnv(s string) string {
	return Expand(s, Getenv)
}

// isShellSpecialVar reports whether the character identifies a special
// shell variable such as $*.
func isShellSpecialVar(c uint8) bool {
	switch c {
	case '*', '#', '$', '@', '!', '?', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	}
	return false
}

// isAlphaNum reports whether the byte is an ASCII letter, number, or underscore.
func isAlphaNum(c uint8) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// getShellName returns the name that begins the string and the number of bytes
// consumed to extract it. If the name is enclosed in {}, it's part of a ${}
// expansion and two more bytes are needed than the length of the name.
func getShellName(s string) (string, int) {
	switch {
	case s[0] == '{':
		if len(s) > 2 && isShellSpecialVar(s[1]) && s[2] == '}' {
			return s[1:2], 3
		}
		// Scan to closing brace
		for i := 1; i < len(s); i++ {
			if s[i] == '}' {
				if i == 1 {
					return "", 2 // Bad syntax; eat "${}"
				}
				return s[1:i], i + 1
			}
		}
		return "", 1 // Bad syntax; eat "${"
	case isShellSpecialVar(s[0]):
		return s[0:1], 1
	}
	// Scan alphanumerics.
	var i int
	for i = 0; i < len(s) && isAlphaNum(s[i]); i++ {
	}
	return s[:i], i
}

// Getenv retrieves the value of the environment variable named by the key.
// It returns the value, which will be empty if the variable is not present.
// To distinguish between an empty value and an unset value, use [LookupEnv].
func Getenv(key string) string {
	testlog.Getenv(key)
	v, _ := syscall.Getenv(key)
	return v
}

// LookupEnv retrieves the value of the environment variable named
// by the key. If the variable is present in the environment the
// value (which may be empty) is returned and the boolean is true.
// Otherwise the returned value will be empty and the boolean will
// be false.
func LookupEnv(key string) (string, bool) {
	testlog.Getenv(key)
	return syscall.Getenv(key)
}

// Setenv sets the value of the environment variable named by the key.
// It returns an error, if any.
func Setenv(key, value string) error {
	err := syscall.Setenv(key, value)
	if err != nil {
		return NewSyscallError("setenv", err)
	}
	return nil
}

// Unsetenv unsets a single environment variable.
func Unsetenv(key string) error {
	return syscall.Unsetenv(key)
}

// Clearenv deletes all environment variables.
func Clearenv() {
	syscall.Clearenv()
}

// Environ returns a copy of strings representing the environment,
// in the form "key=value".
func Environ() []string {
	return syscall.Environ()
}

```

// === FILE: references!/go/src/os/error.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/poll"
	"io/fs"
)

// Portable analogs of some common system call errors.
//
// Errors returned from this package may be tested against these errors
// with [errors.Is].
var (
	// ErrInvalid indicates an invalid argument.
	// Methods on File will return this error when the receiver is nil.
	ErrInvalid = fs.ErrInvalid // "invalid argument"

	ErrPermission = fs.ErrPermission // "permission denied"
	ErrExist      = fs.ErrExist      // "file already exists"
	ErrNotExist   = fs.ErrNotExist   // "file does not exist"
	ErrClosed     = fs.ErrClosed     // "file already closed"

	ErrNoDeadline       = errNoDeadline()       // "file type does not support deadline"
	ErrDeadlineExceeded = errDeadlineExceeded() // "i/o timeout"
)

func errNoDeadline() error { return poll.ErrNoDeadline }

// errDeadlineExceeded returns the value for os.ErrDeadlineExceeded.
// This error comes from the internal/poll package, which is also
// used by package net. Doing it this way ensures that the net
// package will return os.ErrDeadlineExceeded for an exceeded deadline,
// as documented by net.Conn.SetDeadline, without requiring any extra
// work in the net package and without requiring the internal/poll
// package to import os (which it can't, because that would be circular).
func errDeadlineExceeded() error { return poll.ErrDeadlineExceeded }

type timeout interface {
	Timeout() bool
}

// PathError records an error and the operation and file path that caused it.
type PathError = fs.PathError

// SyscallError records an error from a specific system call.
type SyscallError struct {
	Syscall string
	Err     error
}

func (e *SyscallError) Error() string { return e.Syscall + ": " + e.Err.Error() }

func (e *SyscallError) Unwrap() error { return e.Err }

// Timeout reports whether this error represents a timeout.
func (e *SyscallError) Timeout() bool {
	t, ok := e.Err.(timeout)
	return ok && t.Timeout()
}

// NewSyscallError returns, as an error, a new [SyscallError]
// with the given system call name and error details.
// As a convenience, if err is nil, NewSyscallError returns nil.
func NewSyscallError(syscall string, err error) error {
	if err == nil {
		return nil
	}
	return &SyscallError{syscall, err}
}

// IsExist returns a boolean indicating whether its argument is known to report
// that a file or directory already exists. It is satisfied by [ErrExist] as
// well as some syscall errors.
//
// This function predates [errors.Is]. It only supports errors returned by
// the os package. New code should use errors.Is(err, fs.ErrExist).
func IsExist(err error) bool {
	return underlyingErrorIs(err, ErrExist)
}

// IsNotExist returns a boolean indicating whether its argument is known to
// report that a file or directory does not exist. It is satisfied by
// [ErrNotExist] as well as some syscall errors.
//
// This function predates [errors.Is]. It only supports errors returned by
// the os package. New code should use errors.Is(err, fs.ErrNotExist).
func IsNotExist(err error) bool {
	return underlyingErrorIs(err, ErrNotExist)
}

// IsPermission returns a boolean indicating whether its argument is known to
// report that permission is denied. It is satisfied by [ErrPermission] as well
// as some syscall errors.
//
// This function predates [errors.Is]. It only supports errors returned by
// the os package. New code should use errors.Is(err, fs.ErrPermission).
func IsPermission(err error) bool {
	return underlyingErrorIs(err, ErrPermission)
}

// IsTimeout returns a boolean indicating whether its argument is known
// to report that a timeout occurred.
//
// This function predates [errors.Is], and the notion of whether an
// error indicates a timeout can be ambiguous. For example, the Unix
// error EWOULDBLOCK sometimes indicates a timeout and sometimes does not.
// New code should use errors.Is with a value appropriate to the call
// returning the error, such as [os.ErrDeadlineExceeded].
func IsTimeout(err error) bool {
	terr, ok := underlyingError(err).(timeout)
	return ok && terr.Timeout()
}

func underlyingErrorIs(err, target error) bool {
	// Note that this function is not errors.Is:
	// underlyingError only unwraps the specific error-wrapping types
	// that it historically did, not all errors implementing Unwrap().
	err = underlyingError(err)
	if err == target {
		return true
	}
	// To preserve prior behavior, only examine syscall errors.
	e, ok := err.(syscallErrorType)
	return ok && e.Is(target)
}

// underlyingError returns the underlying error for known os error types.
func underlyingError(err error) error {
	switch err := err.(type) {
	case *PathError:
		return err.Err
	case *LinkError:
		return err.Err
	case *SyscallError:
		return err.Err
	}
	return err
}

```

// === FILE: references!/go/src/os/error_errno.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !plan9

package os

import "syscall"

type syscallErrorType = syscall.Errno

const (
	errENOSYS = syscall.ENOSYS
	errERANGE = syscall.ERANGE
	errENOMEM = syscall.ENOMEM
)

```

// === FILE: references!/go/src/os/error_plan9.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import "syscall"

type syscallErrorType = syscall.ErrorString

var errENOSYS = syscall.NewError("function not implemented")
var errERANGE = syscall.NewError("out of range")
var errENOMEM = syscall.NewError("cannot allocate memory")

```

// === FILE: references!/go/src/os/exec/exec.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package exec runs external commands. It wraps os.StartProcess to make it
// easier to remap stdin and stdout, connect I/O with pipes, and do other
// adjustments.
//
// Unlike the "system" library call from C and other languages, the
// os/exec package intentionally does not invoke the system shell and
// does not expand any glob patterns or handle other expansions,
// pipelines, or redirections typically done by shells. The package
// behaves more like C's "exec" family of functions. To expand glob
// patterns, either call the shell directly, taking care to escape any
// dangerous input, or use the [path/filepath] package's Glob function.
// To expand environment variables, use package os's ExpandEnv.
//
// Note that the examples in this package assume a Unix system.
// They may not run on Windows, and they do not run in the Go Playground
// used by go.dev and pkg.go.dev.
//
// # Executables in the current directory
//
// The functions [Command] and [LookPath] look for a program
// in the directories listed in the current path, following the
// conventions of the host operating system.
// Operating systems have for decades included the current
// directory in this search, sometimes implicitly and sometimes
// configured explicitly that way by default.
// Modern practice is that including the current directory
// is usually unexpected and often leads to security problems.
//
// To avoid those security problems, as of Go 1.19, this package will not resolve a program
// using an implicit or explicit path entry relative to the current directory.
// That is, if you run [LookPath]("go"), it will not successfully return
// ./go on Unix nor .\go.exe on Windows, no matter how the path is configured.
// Instead, if the usual path algorithms would result in that answer,
// these functions return an error err satisfying [errors.Is](err, [ErrDot]).
//
// For example, consider these two program snippets:
//
//	path, err := exec.LookPath("prog")
//	if err != nil {
//		log.Fatal(err)
//	}
//	use(path)
//
// and
//
//	cmd := exec.Command("prog")
//	if err := cmd.Run(); err != nil {
//		log.Fatal(err)
//	}
//
// These will not find and run ./prog or .\prog.exe,
// no matter how the current path is configured.
//
// Code that always wants to run a program from the current directory
// can be rewritten to say "./prog" instead of "prog".
//
// Code that insists on including results from relative path entries
// can instead override the error using an errors.Is check:
//
//	path, err := exec.LookPath("prog")
//	if errors.Is(err, exec.ErrDot) {
//		err = nil
//	}
//	if err != nil {
//		log.Fatal(err)
//	}
//	use(path)
//
// and
//
//	cmd := exec.Command("prog")
//	if errors.Is(cmd.Err, exec.ErrDot) {
//		cmd.Err = nil
//	}
//	if err := cmd.Run(); err != nil {
//		log.Fatal(err)
//	}
//
// Setting the environment variable GODEBUG=execerrdot=0
// disables generation of ErrDot entirely, temporarily restoring the pre-Go 1.19
// behavior for programs that are unable to apply more targeted fixes.
// A future version of Go may remove support for this variable.
//
// Before adding such overrides, make sure you understand the
// security implications of doing so.
// See https://go.dev/blog/path-security for more information.
package exec

import (
	"bytes"
	"context"
	"errors"
	"internal/godebug"
	"internal/syscall/execenv"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// Error is returned by [LookPath] when it fails to classify a file as an
// executable.
type Error struct {
	// Name is the file name for which the error occurred.
	Name string
	// Err is the underlying error.
	Err error
}

func (e *Error) Error() string {
	return "exec: " + strconv.Quote(e.Name) + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

// ErrWaitDelay is returned by [Cmd.Wait] if the process exits with a
// successful status code but its output pipes are not closed before the
// command's WaitDelay expires.
var ErrWaitDelay = errors.New("exec: WaitDelay expired before I/O complete")

// wrappedError wraps an error without relying on fmt.Errorf.
type wrappedError struct {
	prefix string
	err    error
}

func (w wrappedError) Error() string {
	return w.prefix + ": " + w.err.Error()
}

func (w wrappedError) Unwrap() error {
	return w.err
}

// Cmd represents an external command being prepared or run.
//
// A Cmd cannot be reused after calling its [Cmd.Start], [Cmd.Run],
// [Cmd.Output], or [Cmd.CombinedOutput] methods.
type Cmd struct {
	// Path is the path of the command to run.
	//
	// This is the only field that must be set to a non-zero
	// value. If Path is relative, it is evaluated relative
	// to Dir.
	Path string

	// Args holds command line arguments, including the command as Args[0].
	// If the Args field is empty or nil, Run uses {Path}.
	//
	// In typical use, both Path and Args are set by calling Command.
	Args []string

	// Env specifies the environment of the process.
	// Each entry is of the form "key=value".
	// If Env is nil, the new process uses the current process's
	// environment.
	// If Env contains duplicate environment keys, only the last
	// value in the slice for each duplicate key is used.
	// As a special case on Windows, SYSTEMROOT is always added if
	// missing and not explicitly set to the empty string.
	//
	// See also the Dir field, which may set PWD in the environment.
	Env []string

	// Dir specifies the working directory of the command.
	// If Dir is the empty string, Run runs the command in the
	// calling process's current directory.
	//
	// On Unix systems, the value of Dir also determines the
	// child process's PWD environment variable if not otherwise
	// specified. A Unix process represents its working directory
	// not by name but as an implicit reference to a node in the
	// file tree. So, if the child process obtains its working
	// directory by calling a function such as C's getcwd, which
	// computes the canonical name by walking up the file tree, it
	// will not recover the original value of Dir if that value
	// was an alias involving symbolic links. However, if the
	// child process calls Go's [os.Getwd] or GNU C's
	// get_current_dir_name, and the value of PWD is an alias for
	// the current directory, those functions will return the
	// value of PWD, which matches the value of Dir.
	Dir string

	// Stdin specifies the process's standard input.
	//
	// If Stdin is nil, the process reads from the null device (os.DevNull).
	//
	// If Stdin is an *os.File, the process's standard input is connected
	// directly to that file.
	//
	// Otherwise, during the execution of the command a separate
	// goroutine reads from Stdin and delivers that data to the command
	// over a pipe. In this case, Wait does not complete until the goroutine
	// stops copying, either because it has reached the end of Stdin
	// (EOF or a read error), or because writing to the pipe returned an error,
	// or because a nonzero WaitDelay was set and expired.
	//
	// Regardless of WaitDelay, Wait can block until a Read from
	// Stdin completes. If you need to use a blocking io.Reader,
	// use the StdinPipe method to get a pipe, copy from the Reader
	// to the pipe, and arrange to close the Reader after Wait returns.
	Stdin io.Reader

	// Stdout and Stderr specify the process's standard output and error.
	//
	// If either is nil, Run connects the corresponding file descriptor
	// to the null device (os.DevNull).
	//
	// If either is an *os.File, the corresponding output from the process
	// is connected directly to that file.
	//
	// Otherwise, during the execution of the command a separate goroutine
	// reads from the process over a pipe and delivers that data to the
	// corresponding Writer. In this case, Wait does not complete until the
	// goroutine reaches EOF or encounters an error or a nonzero WaitDelay
	// expires.
	//
	// Regardless of WaitDelay, Wait can block until a Write to
	// Stdout or Stderr completes. If you need to use a blocking io.Writer,
	// use the StdoutPipe or StderrPipe method to get a pipe,
	// copy from the pipe to the Writer, and arrange to close the
	// Writer after Wait returns.
	//
	// If Stdout and Stderr are the same writer, and have a type that can
	// be compared with ==, at most one goroutine at a time will call Write.
	Stdout io.Writer
	Stderr io.Writer

	// ExtraFiles specifies additional open files to be inherited by the
	// new process. It does not include standard input, standard output, or
	// standard error. If non-nil, entry i becomes file descriptor 3+i.
	//
	// ExtraFiles is not supported on Windows.
	ExtraFiles []*os.File

	// SysProcAttr holds optional, operating system-specific attributes.
	// Run passes it to os.StartProcess as the os.ProcAttr's Sys field.
	SysProcAttr *syscall.SysProcAttr

	// Process is the underlying process, once started.
	Process *os.Process

	// ProcessState contains information about an exited process.
	// If the process was started successfully, Wait or Run will
	// populate its ProcessState when the command completes.
	ProcessState *os.ProcessState

	// ctx is the context passed to CommandContext, if any.
	ctx context.Context

	Err error // LookPath error, if any.

	// If Cancel is non-nil, the command must have been created with
	// CommandContext and Cancel will be called when the command's
	// Context is done. By default, CommandContext sets Cancel to
	// call the Kill method on the command's Process.
	//
	// Typically a custom Cancel will send a signal to the command's
	// Process, but it may instead take other actions to initiate cancellation,
	// such as closing a stdin or stdout pipe or sending a shutdown request on a
	// network socket.
	//
	// If the command exits with a success status after Cancel is
	// called, and Cancel does not return an error equivalent to
	// os.ErrProcessDone, then Wait and similar methods will return a non-nil
	// error: either an error wrapping the one returned by Cancel,
	// or the error from the Context.
	// (If the command exits with a non-success status, or Cancel
	// returns an error that wraps os.ErrProcessDone, Wait and similar methods
	// continue to return the command's usual exit status.)
	//
	// If Cancel is set to nil, nothing will happen immediately when the command's
	// Context is done, but a nonzero WaitDelay will still take effect. That may
	// be useful, for example, to work around deadlocks in commands that do not
	// support shutdown signals but are expected to always finish quickly.
	//
	// Cancel will not be called if Start returns a non-nil error.
	Cancel func() error

	// If WaitDelay is non-zero, it bounds the time spent waiting on two sources
	// of unexpected delay in Wait: a child process that fails to exit after the
	// associated Context is canceled, and a child process that exits but leaves
	// its I/O pipes unclosed.
	//
	// The WaitDelay timer starts when either the associated Context is done or a
	// call to Wait observes that the child process has exited, whichever occurs
	// first. When the delay has elapsed, the command shuts down the child process
	// and/or its I/O pipes.
	//
	// If the child process has failed to exit — perhaps because it ignored or
	// failed to receive a shutdown signal from a Cancel function, or because no
	// Cancel function was set — then it will be terminated using os.Process.Kill.
	//
	// Then, if the I/O pipes communicating with the child process are still open,
	// those pipes are closed in order to unblock any goroutines currently blocked
	// on Read or Write calls.
	//
	// If pipes are closed due to WaitDelay, no Cancel call has occurred,
	// and the command has otherwise exited with a successful status, Wait and
	// similar methods will return ErrWaitDelay instead of nil.
	//
	// If WaitDelay is zero (the default), I/O pipes will be read until EOF,
	// which might not occur until orphaned subprocesses of the command have
	// also closed their descriptors for the pipes.
	WaitDelay time.Duration

	// childIOFiles holds closers for any of the child process's
	// stdin, stdout, and/or stderr files that were opened by the Cmd itself
	// (not supplied by the caller). These should be closed as soon as they
	// are inherited by the child process.
	childIOFiles []io.Closer

	// parentIOPipes holds closers for the parent's end of any pipes
	// connected to the child's stdin, stdout, and/or stderr streams
	// that were opened by the Cmd itself (not supplied by the caller).
	// These should be closed after Wait sees the command and copying
	// goroutines exit, or after WaitDelay has expired.
	parentIOPipes []io.Closer

	// goroutine holds a set of closures to execute to copy data
	// to and/or from the command's I/O pipes.
	goroutine []func() error

	// If goroutineErr is non-nil, it receives the first error from a copying
	// goroutine once all such goroutines have completed.
	// goroutineErr is set to nil once its error has been received.
	goroutineErr <-chan error

	// If ctxResult is non-nil, it receives the result of watchCtx exactly once.
	ctxResult <-chan ctxResult

	// The stack saved when the Command was created, if GODEBUG contains
	// execwait=2. Used for debugging leaks.
	createdByStack []byte

	// For a security release long ago, we created x/sys/execabs,
	// which manipulated the unexported lookPathErr error field
	// in this struct. For Go 1.19 we exported the field as Err error,
	// above, but we have to keep lookPathErr around for use by
	// old programs building against new toolchains.
	// The String and Start methods look for an error in lookPathErr
	// in preference to Err, to preserve the errors that execabs sets.
	//
	// In general we don't guarantee misuse of reflect like this,
	// but the misuse of reflect was by us, the best of various bad
	// options to fix the security problem, and people depend on
	// those old copies of execabs continuing to work.
	// The result is that we have to leave this variable around for the
	// rest of time, a compatibility scar.
	//
	// See https://go.dev/blog/path-security
	// and https://go.dev/issue/43724 for more context.
	lookPathErr error

	// cachedLookExtensions caches the result of calling lookExtensions.
	// It is set when Command is called with an absolute path, letting it do
	// the work of resolving the extension, so Start doesn't need to do it again.
	// This is only used on Windows.
	cachedLookExtensions struct{ in, out string }

	// startCalled records that Start was attempted, regardless of outcome.
	// (Until go.dev/issue/77075 is resolved, we use atomic.SwapInt32,
	// not atomic.Bool.Swap, to avoid triggering the copylocks vet check.)
	startCalled int32
}

// A ctxResult reports the result of watching the Context associated with a
// running command (and sending corresponding signals if needed).
type ctxResult struct {
	err error

	// If timer is non-nil, it expires after WaitDelay has elapsed after
	// the Context is done.
	//
	// (If timer is nil, that means that the Context was not done before the
	// command completed, or no WaitDelay was set, or the WaitDelay already
	// expired and its effect was already applied.)
	timer *time.Timer
}

var execwait = godebug.New("#execwait")
var execerrdot = godebug.New("execerrdot")

// Command returns the [Cmd] struct to execute the named program with
// the given arguments.
//
// It sets only the Path and Args in the returned structure.
//
// If name contains no path separators, Command uses [LookPath] to
// resolve name to a complete path if possible. Otherwise it uses name
// directly as Path.
//
// The returned Cmd's Args field is constructed from the command name
// followed by the elements of arg, so arg should not include the
// command name itself. For example, Command("echo", "hello").
// Args[0] is always name, not the possibly resolved Path.
//
// On Windows, processes receive the whole command line as a single string
// and do their own parsing. Command combines and quotes Args into a command
// line string with an algorithm compatible with applications using
// CommandLineToArgvW (which is the most common way). Notable exceptions are
// msiexec.exe and cmd.exe (and thus, all batch files), which have a different
// unquoting algorithm. In these or other similar cases, you can do the
// quoting yourself and provide the full command line in SysProcAttr.CmdLine,
// leaving Args empty.
func Command(name string, arg ...string) *Cmd {
	cmd := &Cmd{
		Path: name,
		Args: append([]string{name}, arg...),
	}

	if v := execwait.Value(); v != "" {
		if v == "2" {
			// Obtain the caller stack. (This is equivalent to runtime/debug.Stack,
			// copied to avoid importing the whole package.)
			stack := make([]byte, 1024)
			for {
				n := runtime.Stack(stack, false)
				if n < len(stack) {
					stack = stack[:n]
					break
				}
				stack = make([]byte, 2*len(stack))
			}

			if i := bytes.Index(stack, []byte("\nos/exec.Command(")); i >= 0 {
				stack = stack[i+1:]
			}
			cmd.createdByStack = stack
		}

		runtime.SetFinalizer(cmd, func(c *Cmd) {
			if c.Process != nil && c.ProcessState == nil {
				debugHint := ""
				if c.createdByStack == nil {
					debugHint = " (set GODEBUG=execwait=2 to capture stacks for debugging)"
				} else {
					os.Stderr.WriteString("GODEBUG=execwait=2 detected a leaked exec.Cmd created by:\n")
					os.Stderr.Write(c.createdByStack)
					os.Stderr.WriteString("\n")
					debugHint = ""
				}
				panic("exec: Cmd started a Process but leaked without a call to Wait" + debugHint)
			}
		})
	}

	if filepath.Base(name) == name {
		lp, err := LookPath(name)
		if lp != "" {
			// Update cmd.Path even if err is non-nil.
			// If err is ErrDot (especially on Windows), lp may include a resolved
			// extension (like .exe or .bat) that should be preserved.
			cmd.Path = lp
		}
		if err != nil {
			cmd.Err = err
		}
	} else if runtime.GOOS == "windows" && filepath.IsAbs(name) {
		// We may need to add a filename extension from PATHEXT
		// or verify an extension that is already present.
		// Since the path is absolute, its extension should be unambiguous
		// and independent of cmd.Dir, and we can go ahead and cache the lookup now.
		//
		// Note that we don't cache anything here for relative paths, because
		// cmd.Dir may be set after we return from this function and that may
		// cause the command to resolve to a different extension.
		if lp, err := lookExtensions(name, ""); err == nil {
			cmd.cachedLookExtensions.in, cmd.cachedLookExtensions.out = name, lp
		} else {
			cmd.Err = err
		}
	}
	return cmd
}

// CommandContext is like [Command] but includes a context.
//
// The provided context is used to interrupt the process
// (by calling cmd.Cancel or [os.Process.Kill])
// if the context becomes done before the command completes on its own.
//
// CommandContext sets the command's Cancel function to invoke the Kill method
// on its Process, and leaves its WaitDelay unset. The caller may change the
// cancellation behavior by modifying those fields before starting the command.
func CommandContext(ctx context.Context, name string, arg ...string) *Cmd {
	if ctx == nil {
		panic("nil Context")
	}
	cmd := Command(name, arg...)
	cmd.ctx = ctx
	cmd.Cancel = func() error {
		return cmd.Process.Kill()
	}
	return cmd
}

// String returns a human-readable description of c.
// It is intended only for debugging.
// In particular, it is not suitable for use as input to a shell.
// The output of String may vary across Go releases.
func (c *Cmd) String() string {
	if c.Err != nil || c.lookPathErr != nil {
		// failed to resolve path; report the original requested path (plus args)
		return strings.Join(c.Args, " ")
	}
	// report the exact executable path (plus args)
	b := new(strings.Builder)
	b.WriteString(c.Path)
	for _, a := range c.argv()[1:] {
		b.WriteByte(' ')
		b.WriteString(a)
	}
	return b.String()
}

// interfaceEqual protects against panics from doing equality tests on
// two interfaces with non-comparable underlying types.
func interfaceEqual(a, b any) bool {
	defer func() {
		recover()
	}()
	return a == b
}

func (c *Cmd) argv() []string {
	if len(c.Args) > 0 {
		return c.Args
	}
	return []string{c.Path}
}

func (c *Cmd) childStdin() (*os.File, error) {
	if c.Stdin == nil {
		f, err := os.Open(os.DevNull)
		if err != nil {
			return nil, err
		}
		c.childIOFiles = append(c.childIOFiles, f)
		return f, nil
	}

	if f, ok := c.Stdin.(*os.File); ok {
		return f, nil
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	c.childIOFiles = append(c.childIOFiles, pr)
	c.parentIOPipes = append(c.parentIOPipes, pw)
	c.goroutine = append(c.goroutine, func() error {
		_, err := io.Copy(pw, c.Stdin)
		if skipStdinCopyError(err) {
			err = nil
		}
		if err1 := pw.Close(); err == nil {
			err = err1
		}
		return err
	})
	return pr, nil
}

func (c *Cmd) childStdout() (*os.File, error) {
	return c.writerDescriptor(c.Stdout)
}

func (c *Cmd) childStderr(childStdout *os.File) (*os.File, error) {
	if c.Stderr != nil && interfaceEqual(c.Stderr, c.Stdout) {
		return childStdout, nil
	}
	return c.writerDescriptor(c.Stderr)
}

// writerDescriptor returns an os.File to which the child process
// can write to send data to w.
//
// If w is nil, writerDescriptor returns a File that writes to os.DevNull.
func (c *Cmd) writerDescriptor(w io.Writer) (*os.File, error) {
	if w == nil {
		f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			return nil, err
		}
		c.childIOFiles = append(c.childIOFiles, f)
		return f, nil
	}

	if f, ok := w.(*os.File); ok {
		return f, nil
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	c.childIOFiles = append(c.childIOFiles, pw)
	c.parentIOPipes = append(c.parentIOPipes, pr)
	c.goroutine = append(c.goroutine, func() error {
		_, err := io.Copy(w, pr)
		pr.Close() // in case io.Copy stopped due to write error
		return err
	})
	return pw, nil
}

func closeDescriptors(closers []io.Closer) {
	for _, fd := range closers {
		fd.Close()
	}
}

// Run starts the specified command and waits for it to complete.
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the command starts but does not complete successfully, the error is of
// type [*ExitError]. Other error types may be returned for other situations.
//
// If the calling goroutine has locked the operating system thread
// with [runtime.LockOSThread] and modified any inheritable OS-level
// thread state (for example, Linux or Plan 9 name spaces), the new
// process will inherit the caller's thread state.
func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// Start starts the specified command but does not wait for it to complete.
//
// If Start returns successfully, the c.Process field will be set.
//
// After a successful call to Start the [Cmd.Wait] method must be called in
// order to release associated system resources.
func (c *Cmd) Start() error {
	// Check for doubled Start calls before we defer failure cleanup. If the prior
	// call to Start succeeded, we don't want to spuriously close its pipes.
	// It is an error to call Start twice even if the first call did not create a process.
	if atomic.SwapInt32(&c.startCalled, 1) != 0 {
		return errors.New("exec: already started")
	}

	started := false
	defer func() {
		closeDescriptors(c.childIOFiles)
		c.childIOFiles = nil

		if !started {
			closeDescriptors(c.parentIOPipes)
			c.parentIOPipes = nil
			c.goroutine = nil // aid GC, finalization of pipe fds
		}
	}()

	if c.Path == "" && c.Err == nil && c.lookPathErr == nil {
		c.Err = errors.New("exec: no command")
	}
	if c.Err != nil || c.lookPathErr != nil {
		if c.lookPathErr != nil {
			return c.lookPathErr
		}
		return c.Err
	}
	lp := c.Path
	if runtime.GOOS == "windows" {
		if c.Path == c.cachedLookExtensions.in {
			// If Command was called with an absolute path, we already resolved
			// its extension and shouldn't need to do so again (provided c.Path
			// wasn't set to another value between the calls to Command and Start).
			lp = c.cachedLookExtensions.out
		} else {
			// If *Cmd was made without using Command at all, or if Command was
			// called with a relative path, we had to wait until now to resolve
			// it in case c.Dir was changed.
			//
			// Unfortunately, we cannot write the result back to c.Path because programs
			// may assume that they can call Start concurrently with reading the path.
			// (It is safe and non-racy to do so on Unix platforms, and users might not
			// test with the race detector on all platforms;
			// see https://go.dev/issue/62596.)
			//
			// So we will pass the fully resolved path to os.StartProcess, but leave
			// c.Path as is: missing a bit of logging information seems less harmful
			// than triggering a surprising data race, and if the user really cares
			// about that bit of logging they can always use LookPath to resolve it.
			var err error
			lp, err = lookExtensions(c.Path, c.Dir)
			if err != nil {
				return err
			}
		}
	}
	if c.Cancel != nil && c.ctx == nil {
		return errors.New("exec: command with a non-nil Cancel was not created with CommandContext")
	}
	if c.ctx != nil {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		default:
		}
	}

	childFiles := make([]*os.File, 0, 3+len(c.ExtraFiles))
	stdin, err := c.childStdin()
	if err != nil {
		return err
	}
	childFiles = append(childFiles, stdin)
	stdout, err := c.childStdout()
	if err != nil {
		return err
	}
	childFiles = append(childFiles, stdout)
	stderr, err := c.childStderr(stdout)
	if err != nil {
		return err
	}
	childFiles = append(childFiles, stderr)
	childFiles = append(childFiles, c.ExtraFiles...)

	env, err := c.environ()
	if err != nil {
		return err
	}

	c.Process, err = os.StartProcess(lp, c.argv(), &os.ProcAttr{
		Dir:   c.Dir,
		Files: childFiles,
		Env:   env,
		Sys:   c.SysProcAttr,
	})
	if err != nil {
		return err
	}
	started = true

	// Don't allocate the goroutineErr channel unless there are goroutines to start.
	if len(c.goroutine) > 0 {
		goroutineErr := make(chan error, 1)
		c.goroutineErr = goroutineErr

		type goroutineStatus struct {
			running  int
			firstErr error
		}
		statusc := make(chan goroutineStatus, 1)
		statusc <- goroutineStatus{running: len(c.goroutine)}
		for _, fn := range c.goroutine {
			go func(fn func() error) {
				err := fn()

				status := <-statusc
				if status.firstErr == nil {
					status.firstErr = err
				}
				status.running--
				if status.running == 0 {
					goroutineErr <- status.firstErr
				} else {
					statusc <- status
				}
			}(fn)
		}
		c.goroutine = nil // Allow the goroutines' closures to be GC'd when they complete.
	}

	// If we have anything to do when the command's Context expires,
	// start a goroutine to watch for cancellation.
	//
	// (Even if the command was created by CommandContext, a helper library may
	// have explicitly set its Cancel field back to nil, indicating that it should
	// be allowed to continue running after cancellation after all.)
	if (c.Cancel != nil || c.WaitDelay != 0) && c.ctx != nil && c.ctx.Done() != nil {
		resultc := make(chan ctxResult)
		c.ctxResult = resultc
		go c.watchCtx(resultc)
	}

	return nil
}

// watchCtx watches c.ctx until it is able to send a result to resultc.
//
// If c.ctx is done before a result can be sent, watchCtx calls c.Cancel,
// and/or kills cmd.Process it after c.WaitDelay has elapsed.
//
// watchCtx manipulates c.goroutineErr, so its result must be received before
// c.awaitGoroutines is called.
func (c *Cmd) watchCtx(resultc chan<- ctxResult) {
	select {
	case resultc <- ctxResult{}:
		return
	case <-c.ctx.Done():
	}

	var err error
	if c.Cancel != nil {
		if interruptErr := c.Cancel(); interruptErr == nil {
			// We appear to have successfully interrupted the command, so any
			// program behavior from this point may be due to ctx even if the
			// command exits with code 0.
			err = c.ctx.Err()
		} else if errors.Is(interruptErr, os.ErrProcessDone) {
			// The process already finished: we just didn't notice it yet.
			// (Perhaps c.Wait hadn't been called, or perhaps it happened to race with
			// c.ctx being canceled.) Don't inject a needless error.
		} else {
			err = wrappedError{
				prefix: "exec: canceling Cmd",
				err:    interruptErr,
			}
		}
	}
	if c.WaitDelay == 0 {
		resultc <- ctxResult{err: err}
		return
	}

	timer := time.NewTimer(c.WaitDelay)
	select {
	case resultc <- ctxResult{err: err, timer: timer}:
		// c.Process.Wait returned and we've handed the timer off to c.Wait.
		// It will take care of goroutine shutdown from here.
		return
	case <-timer.C:
	}

	killed := false
	if killErr := c.Process.Kill(); killErr == nil {
		// We appear to have killed the process. c.Process.Wait should return a
		// non-nil error to c.Wait unless the Kill signal races with a successful
		// exit, and if that does happen we shouldn't report a spurious error,
		// so don't set err to anything here.
		killed = true
	} else if !errors.Is(killErr, os.ErrProcessDone) {
		err = wrappedError{
			prefix: "exec: killing Cmd",
			err:    killErr,
		}
	}

	if c.goroutineErr != nil {
		select {
		case goroutineErr := <-c.goroutineErr:
			// Forward goroutineErr only if we don't have reason to believe it was
			// caused by a call to Cancel or Kill above.
			if err == nil && !killed {
				err = goroutineErr
			}
		default:
			// Close the child process's I/O pipes, in case it abandoned some
			// subprocess that inherited them and is still holding them open
			// (see https://go.dev/issue/23019).
			//
			// We close the goroutine pipes only after we have sent any signals we're
			// going to send to the process (via Signal or Kill above): if we send
			// SIGKILL to the process, we would prefer for it to die of SIGKILL, not
			// SIGPIPE. (However, this may still cause any orphaned subprocesses to
			// terminate with SIGPIPE.)
			closeDescriptors(c.parentIOPipes)
			// Wait for the copying goroutines to finish, but report ErrWaitDelay for
			// the error: any other error here could result from closing the pipes.
			_ = <-c.goroutineErr
			if err == nil {
				err = ErrWaitDelay
			}
		}

		// Since we have already received the only result from c.goroutineErr,
		// set it to nil to prevent awaitGoroutines from blocking on it.
		c.goroutineErr = nil
	}

	resultc <- ctxResult{err: err}
}

// An ExitError reports an unsuccessful exit by a command.
type ExitError struct {
	*os.ProcessState

	// Stderr holds a subset of the standard error output from the
	// Cmd.Output method if standard error was not otherwise being
	// collected.
	//
	// If the error output is long, Stderr may contain only a prefix
	// and suffix of the output, with the middle replaced with
	// text about the number of omitted bytes.
	//
	// Stderr is provided for debugging, for inclusion in error messages.
	// Users with other needs should redirect Cmd.Stderr as needed.
	Stderr []byte
}

func (e *ExitError) Error() string {
	return e.ProcessState.String()
}

// Wait waits for the command to exit and waits for any copying to
// stdin or copying from stdout or stderr to complete.
//
// The command must have been started by [Cmd.Start].
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the command fails to run or doesn't complete successfully, the
// error is of type [*ExitError]. Other error types may be
// returned for I/O problems.
//
// If any of c.Stdin, c.Stdout or c.Stderr are not an [*os.File], Wait also waits
// for the respective I/O loop copying to or from the process to complete.
//
// Wait must not be called concurrently from multiple goroutines.
// A custom Cmd.Cancel function should not call Wait.
//
// Wait releases any resources associated with the [Cmd].
func (c *Cmd) Wait() error {
	if c.Process == nil {
		return errors.New("exec: not started")
	}
	if c.ProcessState != nil {
		return errors.New("exec: Wait was already called")
	}

	state, err := c.Process.Wait()
	if err == nil && !state.Success() {
		err = &ExitError{ProcessState: state}
	}
	c.ProcessState = state

	var timer *time.Timer
	if c.ctxResult != nil {
		watch := <-c.ctxResult
		timer = watch.timer
		// If c.Process.Wait returned an error, prefer that.
		// Otherwise, report any error from the watchCtx goroutine,
		// such as a Context cancellation or a WaitDelay overrun.
		if err == nil && watch.err != nil {
			err = watch.err
		}
	}

	if goroutineErr := c.awaitGoroutines(timer); err == nil {
		// Report an error from the copying goroutines only if the program otherwise
		// exited normally on its own. Otherwise, the copying error may be due to the
		// abnormal termination.
		err = goroutineErr
	}
	closeDescriptors(c.parentIOPipes)
	c.parentIOPipes = nil

	return err
}

// awaitGoroutines waits for the results of the goroutines copying data to or
// from the command's I/O pipes.
//
// If c.WaitDelay elapses before the goroutines complete, awaitGoroutines
// forcibly closes their pipes and returns ErrWaitDelay.
//
// If timer is non-nil, it must send to timer.C at the end of c.WaitDelay.
func (c *Cmd) awaitGoroutines(timer *time.Timer) error {
	defer func() {
		if timer != nil {
			timer.Stop()
		}
		c.goroutineErr = nil
	}()

	if c.goroutineErr == nil {
		return nil // No running goroutines to await.
	}

	if timer == nil {
		if c.WaitDelay == 0 {
			return <-c.goroutineErr
		}

		select {
		case err := <-c.goroutineErr:
			// Avoid the overhead of starting a timer.
			return err
		default:
		}

		// No existing timer was started: either there is no Context associated with
		// the command, or c.Process.Wait completed before the Context was done.
		timer = time.NewTimer(c.WaitDelay)
	}

	select {
	case <-timer.C:
		closeDescriptors(c.parentIOPipes)
		// Wait for the copying goroutines to finish, but ignore any error
		// (since it was probably caused by closing the pipes).
		_ = <-c.goroutineErr
		return ErrWaitDelay

	case err := <-c.goroutineErr:
		return err
	}
}

// Output runs the command and returns its standard output.
// Any returned error will usually be of type [*ExitError].
// If c.Stderr was nil and the returned error is of type
// [*ExitError], Output populates the Stderr field of the
// returned error.
func (c *Cmd) Output() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	var stdout bytes.Buffer
	c.Stdout = &stdout

	captureErr := c.Stderr == nil
	if captureErr {
		c.Stderr = &prefixSuffixSaver{N: 32 << 10}
	}

	err := c.Run()
	if err != nil && captureErr {
		if ee, ok := err.(*ExitError); ok {
			ee.Stderr = c.Stderr.(*prefixSuffixSaver).Bytes()
		}
	}
	return stdout.Bytes(), err
}

// CombinedOutput runs the command and returns its combined standard
// output and standard error.
func (c *Cmd) CombinedOutput() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = &b
	err := c.Run()
	return b.Bytes(), err
}

// StdinPipe returns a pipe that will be connected to the command's
// standard input when the command starts.
// The pipe will be closed automatically after [Cmd.Wait] sees the command exit.
// A caller need only call Close to force the pipe to close sooner.
// For example, if the command being run will not exit until standard input
// is closed, the caller must close the pipe.
func (c *Cmd) StdinPipe() (io.WriteCloser, error) {
	if c.Stdin != nil {
		return nil, errors.New("exec: Stdin already set")
	}
	if c.Process != nil {
		return nil, errors.New("exec: StdinPipe after process started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stdin = pr
	c.childIOFiles = append(c.childIOFiles, pr)
	c.parentIOPipes = append(c.parentIOPipes, pw)
	return pw, nil
}

// StdoutPipe returns a pipe that will be connected to the command's
// standard output when the command starts.
//
// [Cmd.Wait] will close the pipe after seeing the command exit, so most callers
// need not close the pipe themselves. It is thus incorrect to call Wait
// before all reads from the pipe have completed.
// For the same reason, it is incorrect to call [Cmd.Run] when using StdoutPipe.
// See the example for idiomatic usage.
func (c *Cmd) StdoutPipe() (io.ReadCloser, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Process != nil {
		return nil, errors.New("exec: StdoutPipe after process started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stdout = pw
	c.childIOFiles = append(c.childIOFiles, pw)
	c.parentIOPipes = append(c.parentIOPipes, pr)
	return pr, nil
}

// StderrPipe returns a pipe that will be connected to the command's
// standard error when the command starts.
//
// [Cmd.Wait] will close the pipe after seeing the command exit, so most callers
// need not close the pipe themselves. It is thus incorrect to call Wait
// before all reads from the pipe have completed.
// For the same reason, it is incorrect to use [Cmd.Run] when using StderrPipe.
// See the StdoutPipe example for idiomatic usage.
func (c *Cmd) StderrPipe() (io.ReadCloser, error) {
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	if c.Process != nil {
		return nil, errors.New("exec: StderrPipe after process started")
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	c.Stderr = pw
	c.childIOFiles = append(c.childIOFiles, pw)
	c.parentIOPipes = append(c.parentIOPipes, pr)
	return pr, nil
}

// prefixSuffixSaver is an io.Writer which retains the first N bytes
// and the last N bytes written to it. The Bytes() methods reconstructs
// it with a pretty error message.
type prefixSuffixSaver struct {
	N         int // max size of prefix or suffix
	prefix    []byte
	suffix    []byte // ring buffer once len(suffix) == N
	suffixOff int    // offset to write into suffix
	skipped   int64

	// TODO(bradfitz): we could keep one large []byte and use part of it for
	// the prefix, reserve space for the '... Omitting N bytes ...' message,
	// then the ring buffer suffix, and just rearrange the ring buffer
	// suffix when Bytes() is called, but it doesn't seem worth it for
	// now just for error messages. It's only ~64KB anyway.
}

func (w *prefixSuffixSaver) Write(p []byte) (n int, err error) {
	lenp := len(p)
	p = w.fill(&w.prefix, p)

	// Only keep the last w.N bytes of suffix data.
	if overage := len(p) - w.N; overage > 0 {
		p = p[overage:]
		w.skipped += int64(overage)
	}
	p = w.fill(&w.suffix, p)

	// w.suffix is full now if p is non-empty. Overwrite it in a circle.
	for len(p) > 0 { // 0, 1, or 2 iterations.
		n := copy(w.suffix[w.suffixOff:], p)
		p = p[n:]
		w.skipped += int64(n)
		w.suffixOff += n
		if w.suffixOff == w.N {
			w.suffixOff = 0
		}
	}
	return lenp, nil
}

// fill appends up to len(p) bytes of p to *dst, such that *dst does not
// grow larger than w.N. It returns the un-appended suffix of p.
func (w *prefixSuffixSaver) fill(dst *[]byte, p []byte) (pRemain []byte) {
	if remain := w.N - len(*dst); remain > 0 {
		add := min(len(p), remain)
		*dst = append(*dst, p[:add]...)
		p = p[add:]
	}
	return p
}

func (w *prefixSuffixSaver) Bytes() []byte {
	if w.suffix == nil {
		return w.prefix
	}
	if w.skipped == 0 {
		return append(w.prefix, w.suffix...)
	}
	var buf bytes.Buffer
	buf.Grow(len(w.prefix) + len(w.suffix) + 50)
	buf.Write(w.prefix)
	buf.WriteString("\n... omitting ")
	buf.WriteString(strconv.FormatInt(w.skipped, 10))
	buf.WriteString(" bytes ...\n")
	buf.Write(w.suffix[w.suffixOff:])
	buf.Write(w.suffix[:w.suffixOff])
	return buf.Bytes()
}

// environ returns a best-effort copy of the environment in which the command
// would be run as it is currently configured. If an error occurs in computing
// the environment, it is returned alongside the best-effort copy.
func (c *Cmd) environ() ([]string, error) {
	var err error

	env := c.Env
	if env == nil {
		env, err = execenv.Default(c.SysProcAttr)
		if err != nil {
			env = os.Environ()
			// Note that the non-nil err is preserved despite env being overridden.
		}

		if c.Dir != "" {
			switch runtime.GOOS {
			case "windows", "plan9":
				// Windows and Plan 9 do not use the PWD variable, so we don't need to
				// keep it accurate.
			default:
				// On POSIX platforms, PWD represents “an absolute pathname of the
				// current working directory.” Since we are changing the working
				// directory for the command, we should also update PWD to reflect that.
				//
				// Unfortunately, we didn't always do that, so (as proposed in
				// https://go.dev/issue/50599) to avoid unintended collateral damage we
				// only implicitly update PWD when Env is nil. That way, we're much
				// less likely to override an intentional change to the variable.
				if pwd, absErr := filepath.Abs(c.Dir); absErr == nil {
					env = append(env, "PWD="+pwd)
				} else if err == nil {
					err = absErr
				}
			}
		}
	}

	env, dedupErr := dedupEnv(env)
	if err == nil {
		err = dedupErr
	}
	return addCriticalEnv(env), err
}

// Environ returns a copy of the environment in which the command would be run
// as it is currently configured.
func (c *Cmd) Environ() []string {
	//  Intentionally ignore errors: environ returns a best-effort environment no matter what.
	env, _ := c.environ()
	return env
}

// dedupEnv returns a copy of env with any duplicates removed, in favor of
// later values.
// Items not of the normal environment "key=value" form are preserved unchanged.
// Except on Plan 9, items containing NUL characters are removed, and
// an error is returned along with the remaining values.
func dedupEnv(env []string) ([]string, error) {
	return dedupEnvCase(runtime.GOOS == "windows", runtime.GOOS == "plan9", env)
}

// dedupEnvCase is dedupEnv with a case option for testing.
// If caseInsensitive is true, the case of keys is ignored.
// If nulOK is false, items containing NUL characters are rejected.
func dedupEnvCase(caseInsensitive, nulOK bool, env []string) ([]string, error) {
	// Construct the output in reverse order, to preserve the
	// last occurrence of each key.
	var err error
	out := make([]string, 0, len(env))
	saw := make(map[string]bool, len(env))
	for n := len(env); n > 0; n-- {
		kv := env[n-1]

		// Reject NUL in environment variables to prevent security issues (#56284);
		// except on Plan 9, which uses NUL as os.PathListSeparator (#56544).
		if !nulOK && strings.IndexByte(kv, 0) != -1 {
			err = errors.New("exec: environment variable contains NUL")
			continue
		}

		i := strings.Index(kv, "=")
		if i == 0 {
			// We observe in practice keys with a single leading "=" on Windows.
			// TODO(#49886): Should we consume only the first leading "=" as part
			// of the key, or parse through arbitrarily many of them until a non-"="?
			i = strings.Index(kv[1:], "=") + 1
		}
		if i < 0 {
			if kv != "" {
				// The entry is not of the form "key=value" (as it is required to be).
				// Leave it as-is for now.
				// TODO(#52436): should we strip or reject these bogus entries?
				out = append(out, kv)
			}
			continue
		}
		k := kv[:i]
		if caseInsensitive {
			k = strings.ToLower(k)
		}
		if saw[k] {
			continue
		}

		saw[k] = true
		out = append(out, kv)
	}

	// Now reverse the slice to restore the original order.
	for i := 0; i < len(out)/2; i++ {
		j := len(out) - i - 1
		out[i], out[j] = out[j], out[i]
	}

	return out, err
}

// addCriticalEnv adds any critical environment variables that are required
// (or at least almost always required) on the operating system.
// Currently this is only used for Windows.
func addCriticalEnv(env []string) []string {
	if runtime.GOOS != "windows" {
		return env
	}
	for _, kv := range env {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(k, "SYSTEMROOT") {
			// We already have it.
			return env
		}
	}
	return append(env, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"))
}

// ErrDot indicates that a path lookup resolved to an executable
// in the current directory due to ‘.’ being in the path, either
// implicitly or explicitly. See the package documentation for details.
//
// Note that functions in this package do not return ErrDot directly.
// Code should use errors.Is(err, ErrDot), not err == ErrDot,
// to test whether a returned error err is due to this condition.
var ErrDot = errors.New("cannot run executable found relative to current directory")

// validateLookPath excludes paths that can't be valid
// executable names. See issue #74466 and CVE-2025-47906.
func validateLookPath(s string) error {
	switch s {
	case "", ".", "..":
		return ErrNotFound
	}
	return nil
}

```

// === FILE: references!/go/src/os/exec/exec_plan9.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import "io/fs"

// skipStdinCopyError optionally specifies a function which reports
// whether the provided stdin copy error should be ignored.
func skipStdinCopyError(err error) bool {
	// Ignore hungup errors copying to stdin if the program
	// completed successfully otherwise.
	// See Issue 35753.
	pe, ok := err.(*fs.PathError)
	return ok &&
		pe.Op == "write" && pe.Path == "|1" &&
		pe.Err.Error() == "i/o on hungup channel"
}

```

// === FILE: references!/go/src/os/exec/exec_unix.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !plan9 && !windows

package exec

import (
	"io/fs"
	"syscall"
)

// skipStdinCopyError optionally specifies a function which reports
// whether the provided stdin copy error should be ignored.
func skipStdinCopyError(err error) bool {
	// Ignore EPIPE errors copying to stdin if the program
	// completed successfully otherwise.
	// See Issue 9173.
	pe, ok := err.(*fs.PathError)
	return ok &&
		pe.Op == "write" && pe.Path == "|1" &&
		pe.Err == syscall.EPIPE
}

```

// === FILE: references!/go/src/os/exec/exec_windows.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"io/fs"
	"syscall"
)

// skipStdinCopyError optionally specifies a function which reports
// whether the provided stdin copy error should be ignored.
func skipStdinCopyError(err error) bool {
	// Ignore ERROR_BROKEN_PIPE and ERROR_NO_DATA errors copying
	// to stdin if the program completed successfully otherwise.
	// See Issue 20445.
	const _ERROR_NO_DATA = syscall.Errno(0xe8)
	pe, ok := err.(*fs.PathError)
	return ok &&
		pe.Op == "write" && pe.Path == "|1" &&
		(pe.Err == syscall.ERROR_BROKEN_PIPE || pe.Err == _ERROR_NO_DATA)
}

```

// === FILE: references!/go/src/os/exec/internal/fdtest/exists_plan9.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build plan9

package fdtest

import (
	"syscall"
)

const errBadFd = syscall.ErrorString("fd out of range or not open")

// Exists returns true if fd is a valid file descriptor.
func Exists(fd uintptr) bool {
	var buf [1]byte
	_, err := syscall.Fstat(int(fd), buf[:])
	return err != errBadFd
}

```

// === FILE: references!/go/src/os/exec/internal/fdtest/exists_unix.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || wasm

// Package fdtest provides test helpers for working with file descriptors across exec.
package fdtest

import (
	"syscall"
)

// Exists returns true if fd is a valid file descriptor.
func Exists(fd uintptr) bool {
	var s syscall.Stat_t
	err := syscall.Fstat(int(fd), &s)
	return err != syscall.EBADF
}

```

// === FILE: references!/go/src/os/exec/internal/fdtest/exists_windows.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package fdtest

// Exists is not implemented on windows and panics.
func Exists(fd uintptr) bool {
	panic("unimplemented")
}

```

// === FILE: references!/go/src/os/exec/lookpath.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

// LookPath searches for an executable named file in the current path,
// following the conventions of the host operating system.
// If file contains a slash, it is tried directly and the default path is not consulted.
// Otherwise, on success the result is an absolute path.
//
// LookPath returns an error satisfying [errors.Is](err, [ErrDot])
// if the resolved path is relative to the current directory.
// See the package documentation for more details.
//
// LookPath looks for an executable named file in the
// directories named by the PATH environment variable,
// except as described below.
//
//   - On Windows, the file must have an extension named by
//     the PATHEXT environment variable.
//     When PATHEXT is unset, the file must have
//     a ".com", ".exe", ".bat", or ".cmd" extension.
//   - On Plan 9, LookPath consults the path environment variable.
//     If file begins with "/", "#", "./", or "../", it is tried
//     directly and the path is not consulted.
//   - On Wasm, LookPath always returns an error.
func LookPath(file string) (string, error) {
	return lookPath(file)
}

```

// === FILE: references!/go/src/os/exec/lp_plan9.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is the error resulting if a path search failed to find an executable file.
var ErrNotFound = errors.New("executable file not found in $path")

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
		return nil
	}
	return fs.ErrPermission
}

func lookPath(file string) (string, error) {
	if err := validateLookPath(filepath.Clean(file)); err != nil {
		return "", &Error{file, err}
	}

	// skip the path lookup for these prefixes
	skip := []string{"/", "#", "./", "../"}

	for _, p := range skip {
		if strings.HasPrefix(file, p) {
			err := findExecutable(file)
			if err == nil {
				return file, nil
			}
			return "", &Error{file, err}
		}
	}

	path := os.Getenv("path")
	for _, dir := range filepath.SplitList(path) {
		path := filepath.Join(dir, file)
		if err := findExecutable(path); err == nil {
			if !filepath.IsAbs(path) {
				if execerrdot.Value() != "0" {
					return path, &Error{file, ErrDot}
				}
				execerrdot.IncNonDefault()
			}
			return path, nil
		}
	}
	return "", &Error{file, ErrNotFound}
}

// lookExtensions is a no-op on non-Windows platforms, since
// they do not restrict executables to specific extensions.
func lookExtensions(path, dir string) (string, error) {
	return path, nil
}

```

// === FILE: references!/go/src/os/exec/lp_unix.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package exec

import (
	"errors"
	"internal/syscall/unix"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ErrNotFound is the error resulting if a path search failed to find an executable file.
var ErrNotFound = errors.New("executable file not found in $PATH")

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	m := d.Mode()
	if m.IsDir() {
		return syscall.EISDIR
	}
	err = unix.Eaccess(file, unix.X_OK)
	// ENOSYS means Eaccess is not available or not implemented.
	// EPERM can be returned by Linux containers employing seccomp.
	// In both cases, fall back to checking the permission bits.
	if err == nil || (err != syscall.ENOSYS && err != syscall.EPERM) {
		return err
	}
	if m&0111 != 0 {
		return nil
	}
	return fs.ErrPermission
}

func lookPath(file string) (string, error) {
	// NOTE(rsc): I wish we could use the Plan 9 behavior here
	// (only bypass the path if file begins with / or ./ or ../)
	// but that would not match all the Unix shells.

	if err := validateLookPath(file); err != nil {
		return "", &Error{file, err}
	}

	if strings.Contains(file, "/") {
		err := findExecutable(file)
		if err == nil {
			return file, nil
		}
		return "", &Error{file, err}
	}
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := filepath.Join(dir, file)
		if err := findExecutable(path); err == nil {
			if !filepath.IsAbs(path) {
				if execerrdot.Value() != "0" {
					return path, &Error{file, ErrDot}
				}
				execerrdot.IncNonDefault()
			}
			return path, nil
		}
	}
	return "", &Error{file, ErrNotFound}
}

// lookExtensions is a no-op on non-Windows platforms, since
// they do not restrict executables to specific extensions.
func lookExtensions(path, dir string) (string, error) {
	return path, nil
}

```

// === FILE: references!/go/src/os/exec/lp_wasm.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build wasm

package exec

import (
	"errors"
)

// ErrNotFound is the error resulting if a path search failed to find an executable file.
var ErrNotFound = errors.New("executable file not found in $PATH")

// LookPath searches for an executable named file in the
// directories named by the PATH environment variable.
// If file contains a slash, it is tried directly and the PATH is not consulted.
// The result may be an absolute path or a path relative to the current directory.
func lookPath(file string) (string, error) {
	// Wasm can not execute processes, so act as if there are no executables at all.
	return "", &Error{file, ErrNotFound}
}

// lookExtensions is a no-op on non-Windows platforms, since
// they do not restrict executables to specific extensions.
func lookExtensions(path, dir string) (string, error) {
	return path, nil
}

```

// === FILE: references!/go/src/os/exec/lp_windows.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is the error resulting if a path search failed to find an executable file.
var ErrNotFound = errors.New("executable file not found in %PATH%")

func chkStat(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if d.IsDir() {
		return fs.ErrPermission
	}
	return nil
}

func hasExt(file string) bool {
	i := strings.LastIndex(file, ".")
	if i < 0 {
		return false
	}
	return strings.LastIndexAny(file, `:\/`) < i
}

func findExecutable(file string, exts []string) (string, error) {
	if len(exts) == 0 {
		return file, chkStat(file)
	}
	if hasExt(file) {
		if chkStat(file) == nil {
			return file, nil
		}
		// Keep checking exts below, so that programs with weird names
		// like "foo.bat.exe" will resolve instead of failing.
	}
	for _, e := range exts {
		if f := file + e; chkStat(f) == nil {
			return f, nil
		}
	}
	if hasExt(file) {
		return "", fs.ErrNotExist
	}
	return "", ErrNotFound
}

func lookPath(file string) (string, error) {
	if err := validateLookPath(file); err != nil {
		return "", &Error{file, err}
	}

	return lookPathExts(file, pathExt())
}

// lookExtensions finds windows executable by its dir and path.
// It uses LookPath to try appropriate extensions.
// lookExtensions does not search PATH, instead it converts `prog` into `.\prog`.
//
// If the path already has an extension found in PATHEXT,
// lookExtensions returns it directly without searching
// for additional extensions. For example,
// "C:\foo\example.com" would be returned as-is even if the
// program is actually "C:\foo\example.com.exe".
func lookExtensions(path, dir string) (string, error) {
	if err := validateLookPath(path); err != nil {
		return "", &Error{path, err}
	}

	if filepath.Base(path) == path {
		path = "." + string(filepath.Separator) + path
	}
	exts := pathExt()
	if ext := filepath.Ext(path); ext != "" {
		for _, e := range exts {
			if strings.EqualFold(ext, e) {
				// Assume that path has already been resolved.
				return path, nil
			}
		}
	}
	if dir == "" {
		return lookPathExts(path, exts)
	}
	if filepath.VolumeName(path) != "" {
		return lookPathExts(path, exts)
	}
	if len(path) > 1 && os.IsPathSeparator(path[0]) {
		return lookPathExts(path, exts)
	}
	dirandpath := filepath.Join(dir, path)
	// We assume that LookPath will only add file extension.
	lp, err := lookPathExts(dirandpath, exts)
	if err != nil {
		return "", err
	}
	ext := strings.TrimPrefix(lp, dirandpath)
	return path + ext, nil
}

func pathExt() []string {
	var exts []string
	x := os.Getenv(`PATHEXT`)
	if x != "" {
		for e := range strings.SplitSeq(strings.ToLower(x), `;`) {
			if e == "" {
				continue
			}
			if e[0] != '.' {
				e = "." + e
			}
			exts = append(exts, e)
		}
	} else {
		exts = []string{".com", ".exe", ".bat", ".cmd"}
	}
	return exts
}

// lookPathExts implements LookPath for the given PATHEXT list.
func lookPathExts(file string, exts []string) (string, error) {
	if strings.ContainsAny(file, `:\/`) {
		f, err := findExecutable(file, exts)
		if err == nil {
			return f, nil
		}
		return "", &Error{file, err}
	}

	// On Windows, creating the NoDefaultCurrentDirectoryInExePath
	// environment variable (with any value or no value!) signals that
	// path lookups should skip the current directory.
	// In theory we are supposed to call NeedCurrentDirectoryForExePathW
	// "as the registry location of this environment variable can change"
	// but that seems exceedingly unlikely: it would break all users who
	// have configured their environment this way!
	// https://docs.microsoft.com/en-us/windows/win32/api/processenv/nf-processenv-needcurrentdirectoryforexepathw
	// See also go.dev/issue/43947.
	var (
		dotf   string
		dotErr error
	)
	if _, found := os.LookupEnv("NoDefaultCurrentDirectoryInExePath"); !found {
		if f, err := findExecutable(filepath.Join(".", file), exts); err == nil {
			if execerrdot.Value() == "0" {
				execerrdot.IncNonDefault()
				return f, nil
			}
			dotf, dotErr = f, &Error{file, ErrDot}
		}
	}

	path := os.Getenv("path")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Skip empty entries, consistent with what PowerShell does.
			// (See https://go.dev/issue/61493#issuecomment-1649724826.)
			continue
		}

		if f, err := findExecutable(filepath.Join(dir, file), exts); err == nil {
			if dotErr != nil {
				// https://go.dev/issue/53536: if we resolved a relative path implicitly,
				// and it is the same executable that would be resolved from the explicit %PATH%,
				// prefer the explicit name for the executable (and, likely, no error) instead
				// of the equivalent implicit name with ErrDot.
				//
				// Otherwise, return the ErrDot for the implicit path as soon as we find
				// out that the explicit one doesn't match.
				dotfi, dotfiErr := os.Lstat(dotf)
				fi, fiErr := os.Lstat(f)
				if dotfiErr != nil || fiErr != nil || !os.SameFile(dotfi, fi) {
					return dotf, dotErr
				}
			}

			if !filepath.IsAbs(f) {
				if execerrdot.Value() != "0" {
					// If this is the same relative path that we already found,
					// dotErr is non-nil and we already checked it above.
					// Otherwise, record this path as the one to which we must resolve,
					// with or without a dotErr.
					if dotErr == nil {
						dotf, dotErr = f, &Error{file, ErrDot}
					}
					continue
				}
				execerrdot.IncNonDefault()
			}
			return f, nil
		}
	}

	if dotErr != nil {
		return dotf, dotErr
	}
	return "", &Error{file, ErrNotFound}
}

```

// === FILE: references!/go/src/os/exec/read3.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// This is a test program that verifies that it can read from
// descriptor 3 and that no other descriptors are open.
// This is not done via TestHelperProcess and GO_EXEC_TEST_PID
// because we want to ensure that this program does not use cgo,
// because C libraries can open file descriptors behind our backs
// and confuse the test. See issue 25628.
package main

import (
	"fmt"
	"internal/poll"
	"io"
	"os"
	"os/exec"
	"os/exec/internal/fdtest"
	"runtime"
)

func main() {
	fd3 := os.NewFile(3, "fd3")
	defer fd3.Close()

	bs, err := io.ReadAll(fd3)
	if err != nil {
		fmt.Printf("ReadAll from fd 3: %v\n", err)
		os.Exit(1)
	}

	// Now verify that there are no other open fds.
	// stdin == 0
	// stdout == 1
	// stderr == 2
	// descriptor from parent == 3
	// All descriptors 4 and up should be available,
	// except for any used by the network poller.
	for fd := uintptr(4); fd <= 100; fd++ {
		if poll.IsPollDescriptor(fd) {
			continue
		}

		if !fdtest.Exists(fd) {
			continue
		}

		fmt.Printf("leaked parent file. fdtest.Exists(%d) got true want false\n", fd)

		fdfile := fmt.Sprintf("/proc/self/fd/%d", fd)
		link, err := os.Readlink(fdfile)
		fmt.Printf("readlink(%q) = %q, %v\n", fdfile, link, err)

		var args []string
		switch runtime.GOOS {
		case "plan9":
			args = []string{fmt.Sprintf("/proc/%d/fd", os.Getpid())}
		case "aix", "solaris", "illumos":
			args = []string{fmt.Sprint(os.Getpid())}
		default:
			args = []string{"-p", fmt.Sprint(os.Getpid())}
		}

		// Determine which command to use to display open files.
		ofcmd := "lsof"
		switch runtime.GOOS {
		case "dragonfly", "freebsd", "netbsd", "openbsd":
			ofcmd = "fstat"
		case "plan9":
			ofcmd = "/bin/cat"
		case "aix":
			ofcmd = "procfiles"
		case "solaris", "illumos":
			ofcmd = "pfiles"
		}

		cmd := exec.Command(ofcmd, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%#q failed: %v\n", cmd, err)
		}
		fmt.Printf("%s", out)
		os.Exit(1)
	}

	os.Stdout.Write(bs)
}

```

// === FILE: references!/go/src/os/exec.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"errors"
	"internal/testlog"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	// ErrProcessDone indicates a [Process] has finished.
	ErrProcessDone = errors.New("os: process already finished")
	// errProcessReleased indicates a [Process] has been released.
	errProcessReleased = errors.New("os: process already released")
	// ErrNoHandle indicates a [Process] does not have a handle.
	ErrNoHandle = errors.New("os: process handle unavailable")
)

// processStatus describes the status of a [Process].
type processStatus uint32

const (
	// statusOK means that the Process is ready to use.
	statusOK processStatus = iota

	// statusDone indicates that the PID/handle should not be used because
	// the process is done (has been successfully Wait'd on).
	statusDone

	// statusReleased indicates that the PID/handle should not be used
	// because the process is released.
	statusReleased
)

// Process stores the information about a process created by [StartProcess].
type Process struct {
	Pid int

	// state contains the atomic process state.
	//
	// This consists of the processStatus fields,
	// which indicate if the process is done/released.
	state atomic.Uint32

	// Used only when handle is nil
	sigMu sync.RWMutex // avoid race between wait and signal

	// handle, if not nil, is a pointer to a struct
	// that holds the OS-specific process handle.
	// This pointer is set when Process is created,
	// and never changed afterward.
	// This is a pointer to a separate memory allocation
	// so that we can use runtime.AddCleanup.
	handle *processHandle

	// cleanup is used to clean up the process handle.
	cleanup runtime.Cleanup
}

// processHandle holds an operating system handle to a process.
// This is only used on systems that support that concept,
// currently Linux and Windows.
// This maintains a reference count to the handle,
// and closes the handle when the reference drops to zero.
type processHandle struct {
	// The actual handle. This field should not be used directly.
	// Instead, use the acquire and release methods.
	//
	// On Windows this is a handle returned by OpenProcess.
	// On Linux this is a pidfd.
	handle uintptr

	// Number of active references. When this drops to zero
	// the handle is closed.
	refs atomic.Int32
}

// acquire adds a reference and returns the handle.
// The bool result reports whether acquire succeeded;
// it fails if the handle is already closed.
// Every successful call to acquire should be paired with a call to release.
func (ph *processHandle) acquire() (uintptr, bool) {
	for {
		refs := ph.refs.Load()
		if refs < 0 {
			panic("internal error: negative process handle reference count")
		}
		if refs == 0 {
			return 0, false
		}
		if ph.refs.CompareAndSwap(refs, refs+1) {
			return ph.handle, true
		}
	}
}

// release releases a reference to the handle.
func (ph *processHandle) release() {
	for {
		refs := ph.refs.Load()
		if refs <= 0 {
			panic("internal error: too many releases of process handle")
		}
		if ph.refs.CompareAndSwap(refs, refs-1) {
			if refs == 1 {
				ph.closeHandle()
			}
			return
		}
	}
}

// newPIDProcess returns a [Process] for the given PID.
func newPIDProcess(pid int) *Process {
	p := &Process{
		Pid: pid,
	}
	return p
}

// newHandleProcess returns a [Process] with the given PID and handle.
func newHandleProcess(pid int, handle uintptr) *Process {
	ph := &processHandle{
		handle: handle,
	}

	// Start the reference count as 1,
	// meaning the reference from the returned Process.
	ph.refs.Store(1)

	p := &Process{
		Pid:    pid,
		handle: ph,
	}

	p.cleanup = runtime.AddCleanup(p, (*processHandle).release, ph)

	return p
}

// newDoneProcess returns a [Process] for the given PID
// that is already marked as done. This is used on Unix systems
// if the process is known to not exist.
func newDoneProcess(pid int) *Process {
	p := &Process{
		Pid: pid,
	}
	p.state.Store(uint32(statusDone)) // No persistent reference, as there is no handle.
	return p
}

// handleTransientAcquire returns the process handle or,
// if the process is not ready, the current status.
func (p *Process) handleTransientAcquire() (uintptr, processStatus) {
	if p.handle == nil {
		panic("handleTransientAcquire called in invalid mode")
	}

	status := processStatus(p.state.Load())
	if status != statusOK {
		return 0, status
	}
	h, ok := p.handle.acquire()
	if ok {
		return h, statusOK
	}

	// This case means that the handle has been closed.
	// We always set the status to non-zero before closing the handle.
	// If we get here the status must have been set non-zero after
	// we just checked it above.
	status = processStatus(p.state.Load())
	if status == statusOK {
		panic("inconsistent process status")
	}
	return 0, status
}

// handleTransientRelease releases a handle returned by handleTransientAcquire.
func (p *Process) handleTransientRelease() {
	if p.handle == nil {
		panic("handleTransientRelease called in invalid mode")
	}
	p.handle.release()
}

// pidStatus returns the current process status.
func (p *Process) pidStatus() processStatus {
	if p.handle != nil {
		panic("pidStatus called in invalid mode")
	}

	return processStatus(p.state.Load())
}

// ProcAttr holds the attributes that will be applied to a new process
// started by StartProcess.
type ProcAttr struct {
	// If Dir is non-empty, the child changes into the directory before
	// creating the process.
	Dir string
	// If Env is non-nil, it gives the environment variables for the
	// new process in the form returned by Environ.
	// If it is nil, the result of Environ will be used.
	Env []string
	// Files specifies the open files inherited by the new process. The
	// first three entries correspond to standard input, standard output, and
	// standard error. An implementation may support additional entries,
	// depending on the underlying operating system. A nil entry corresponds
	// to that file being closed when the process starts.
	// On Unix systems, StartProcess will change these File values
	// to blocking mode, which means that SetDeadline will stop working
	// and calling Close will not interrupt a Read or Write.
	Files []*File

	// Operating system-specific process creation attributes.
	// Note that setting this field means that your program
	// may not execute properly or even compile on some
	// operating systems.
	Sys *syscall.SysProcAttr
}

// A Signal represents an operating system signal.
// The usual underlying implementation is operating system-dependent:
// on Unix it is syscall.Signal.
type Signal interface {
	String() string
	Signal() // to distinguish from other Stringers
}

// Getpid returns the process id of the caller.
func Getpid() int { return syscall.Getpid() }

// Getppid returns the process id of the caller's parent.
func Getppid() int { return syscall.Getppid() }

// FindProcess looks for a running process by its pid.
//
// The [Process] it returns can be used to obtain information
// about the underlying operating system process.
//
// On Unix systems, FindProcess always succeeds and returns a Process
// for the given pid, regardless of whether the process exists. To test whether
// the process actually exists, see whether p.Signal(syscall.Signal(0)) reports
// an error.
func FindProcess(pid int) (*Process, error) {
	return findProcess(pid)
}

// StartProcess starts a new process with the program, arguments and attributes
// specified by name, argv and attr. The argv slice will become [os.Args] in the
// new process, so it normally starts with the program name.
//
// If the calling goroutine has locked the operating system thread
// with [runtime.LockOSThread] and modified any inheritable OS-level
// thread state (for example, Linux or Plan 9 name spaces), the new
// process will inherit the caller's thread state.
//
// StartProcess is a low-level interface. The [os/exec] package provides
// higher-level interfaces.
//
// If there is an error, it will be of type [*PathError].
func StartProcess(name string, argv []string, attr *ProcAttr) (*Process, error) {
	testlog.Open(name)
	return startProcess(name, argv, attr)
}

// Release releases any resources associated with the [Process] p,
// rendering it unusable in the future.
// Release only needs to be called if [Process.Wait] is not.
func (p *Process) Release() error {
	// Unfortunately, for historical reasons, on systems other
	// than Windows, Release sets the Pid field to -1.
	// This causes the race detector to report a problem
	// on concurrent calls to Release, but we can't change it now.
	if runtime.GOOS != "windows" {
		p.Pid = -1
	}

	oldStatus := p.doRelease(statusReleased)

	// For backward compatibility, on Windows only,
	// we return EINVAL on a second call to Release.
	if runtime.GOOS == "windows" {
		if oldStatus == statusReleased {
			return syscall.EINVAL
		}
	}

	return nil
}

// doRelease releases a [Process], setting the status to newStatus.
// If the previous status is not statusOK, this does nothing.
// It returns the previous status.
func (p *Process) doRelease(newStatus processStatus) processStatus {
	for {
		state := p.state.Load()
		oldStatus := processStatus(state)
		if oldStatus != statusOK {
			return oldStatus
		}

		if !p.state.CompareAndSwap(state, uint32(newStatus)) {
			continue
		}

		// We have successfully released the Process.
		// If it has a handle, release the reference we
		// created in newHandleProcess.
		if p.handle != nil {
			// No need for more cleanup.
			// We must stop the cleanup before calling release;
			// otherwise the cleanup might run concurrently
			// with the release, which would cause the reference
			// counts to be invalid, causing a panic.
			p.cleanup.Stop()

			p.handle.release()
		}

		return statusOK
	}
}

// Kill causes the [Process] to exit immediately. Kill does not wait until
// the Process has actually exited. This only kills the Process itself,
// not any other processes it may have started.
func (p *Process) Kill() error {
	return p.kill()
}

// Wait waits for the [Process] to exit, and then returns a
// ProcessState describing its status and an error, if any.
// Wait releases any resources associated with the Process.
// On most operating systems, the Process must be a child
// of the current process or an error will be returned.
func (p *Process) Wait() (*ProcessState, error) {
	return p.wait()
}

// Signal sends a signal to the [Process].
// Sending [Interrupt] on Windows is not implemented.
func (p *Process) Signal(sig Signal) error {
	return p.signal(sig)
}

// WithHandle calls a supplied function f with a valid process handle
// as an argument. The handle is guaranteed to refer to process p
// until f returns, even if p terminates. This function cannot be used
// after [Process.Release] or [Process.Wait].
//
// If process handles are not supported or a handle is not available,
// it returns [ErrNoHandle]. Currently, process handles are supported
// on Linux 5.4 or later (pidfd) and Windows.
func (p *Process) WithHandle(f func(handle uintptr)) error {
	return p.withHandle(f)
}

// UserTime returns the user CPU time of the exited process and its children.
func (p *ProcessState) UserTime() time.Duration {
	return p.userTime()
}

// SystemTime returns the system CPU time of the exited process and its children.
func (p *ProcessState) SystemTime() time.Duration {
	return p.systemTime()
}

// Exited reports whether the program has exited.
// On Unix systems this reports true if the program exited due to calling exit,
// but false if the program terminated due to a signal.
func (p *ProcessState) Exited() bool {
	return p.exited()
}

// Success reports whether the program exited successfully,
// such as with exit status 0 on Unix.
func (p *ProcessState) Success() bool {
	return p.success()
}

// Sys returns system-dependent exit information about
// the process. Convert it to the appropriate underlying
// type, such as [syscall.WaitStatus] on Unix, to access its contents.
func (p *ProcessState) Sys() any {
	return p.sys()
}

// SysUsage returns system-dependent resource usage information about
// the exited process. Convert it to the appropriate underlying
// type, such as [*syscall.Rusage] on Unix, to access its contents.
// (On Unix, *syscall.Rusage matches struct rusage as defined in the
// getrusage(2) manual page.)
func (p *ProcessState) SysUsage() any {
	return p.sysUsage()
}

```

// === FILE: references!/go/src/os/exec_linux.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
)

func (ph *processHandle) closeHandle() {
	syscall.Close(int(ph.handle))
}

```

// === FILE: references!/go/src/os/exec_nohandle.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !linux && !windows

package os

func (ph *processHandle) closeHandle() {
	panic("internal error: unexpected call to closeHandle")
}

```

// === FILE: references!/go/src/os/exec_plan9.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/strconv"
	"syscall"
	"time"
)

// The only signal values guaranteed to be present in the os package
// on all systems are Interrupt (send the process an interrupt) and
// Kill (force the process to exit). Interrupt is not implemented on
// Windows; using it with [os.Process.Signal] will return an error.
var (
	Interrupt Signal = syscall.Note("interrupt")
	Kill      Signal = syscall.Note("kill")
)

func startProcess(name string, argv []string, attr *ProcAttr) (p *Process, err error) {
	sysattr := &syscall.ProcAttr{
		Dir: attr.Dir,
		Env: attr.Env,
		Sys: attr.Sys,
	}

	sysattr.Files = make([]uintptr, 0, len(attr.Files))
	for _, f := range attr.Files {
		sysattr.Files = append(sysattr.Files, f.Fd())
	}

	pid, _, e := syscall.StartProcess(name, argv, sysattr)
	if e != nil {
		return nil, &PathError{Op: "fork/exec", Path: name, Err: e}
	}

	return newPIDProcess(pid), nil
}

func (p *Process) writeProcFile(file string, data string) error {
	f, e := OpenFile("/proc/"+strconv.Itoa(p.Pid)+"/"+file, O_WRONLY, 0)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = f.Write([]byte(data))
	return e
}

func (p *Process) signal(sig Signal) error {
	switch p.pidStatus() {
	case statusDone:
		return ErrProcessDone
	case statusReleased:
		return syscall.ENOENT
	}

	if e := p.writeProcFile("note", sig.String()); e != nil {
		return NewSyscallError("signal", e)
	}
	return nil
}

func (p *Process) kill() error {
	return p.signal(Kill)
}

func (p *Process) wait() (ps *ProcessState, err error) {
	var waitmsg syscall.Waitmsg

	switch p.pidStatus() {
	case statusReleased:
		return nil, ErrInvalid
	}

	err = syscall.WaitProcess(p.Pid, &waitmsg)
	if err != nil {
		return nil, NewSyscallError("wait", err)
	}

	p.doRelease(statusDone)
	ps = &ProcessState{
		pid:    waitmsg.Pid,
		status: &waitmsg,
	}
	return ps, nil
}

func (p *Process) withHandle(_ func(handle uintptr)) error {
	return ErrNoHandle
}

func findProcess(pid int) (p *Process, err error) {
	// NOOP for Plan 9.
	return newPIDProcess(pid), nil
}

// ProcessState stores information about a process, as reported by Wait.
type ProcessState struct {
	pid    int              // The process's id.
	status *syscall.Waitmsg // System-dependent status info.
}

// Pid returns the process id of the exited process.
func (p *ProcessState) Pid() int {
	return p.pid
}

func (p *ProcessState) exited() bool {
	return p.status.Exited()
}

func (p *ProcessState) success() bool {
	return p.status.ExitStatus() == 0
}

func (p *ProcessState) sys() any {
	return p.status
}

func (p *ProcessState) sysUsage() any {
	return p.status
}

func (p *ProcessState) userTime() time.Duration {
	return time.Duration(p.status.Time[0]) * time.Millisecond
}

func (p *ProcessState) systemTime() time.Duration {
	return time.Duration(p.status.Time[1]) * time.Millisecond
}

func (p *ProcessState) String() string {
	if p == nil {
		return "<nil>"
	}
	return "exit status: " + p.status.Msg
}

// ExitCode returns the exit code of the exited process, or -1
// if the process hasn't exited or was terminated by a signal.
func (p *ProcessState) ExitCode() int {
	// return -1 if the process hasn't started.
	if p == nil {
		return -1
	}
	return p.status.ExitStatus()
}

```

// === FILE: references!/go/src/os/exec_posix.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || (js && wasm) || wasip1 || windows

package os

import (
	"internal/strconv"
	"internal/syscall/execenv"
	"runtime"
	"syscall"
)

// The only signal values guaranteed to be present in the os package on all
// systems are os.Interrupt (send the process an interrupt) and os.Kill (force
// the process to exit). On Windows, sending os.Interrupt to a process with
// os.Process.Signal is not implemented; it will return an error instead of
// sending a signal.
var (
	Interrupt Signal = syscall.SIGINT
	Kill      Signal = syscall.SIGKILL
)

func startProcess(name string, argv []string, attr *ProcAttr) (p *Process, err error) {
	// If there is no SysProcAttr (ie. no Chroot or changed
	// UID/GID), double-check existence of the directory we want
	// to chdir into. We can make the error clearer this way.
	if attr != nil && attr.Sys == nil && attr.Dir != "" {
		if _, err := Stat(attr.Dir); err != nil {
			pe := err.(*PathError)
			pe.Op = "chdir"
			return nil, pe
		}
	}

	attrSys, shouldDupPidfd := ensurePidfd(attr.Sys)
	sysattr := &syscall.ProcAttr{
		Dir: attr.Dir,
		Env: attr.Env,
		Sys: attrSys,
	}
	if sysattr.Env == nil {
		sysattr.Env, err = execenv.Default(sysattr.Sys)
		if err != nil {
			return nil, err
		}
	}
	sysattr.Files = make([]uintptr, 0, len(attr.Files))
	for _, f := range attr.Files {
		sysattr.Files = append(sysattr.Files, f.Fd())
	}

	pid, h, e := syscall.StartProcess(name, argv, sysattr)

	// Make sure we don't run the finalizers of attr.Files.
	runtime.KeepAlive(attr)

	if e != nil {
		return nil, &PathError{Op: "fork/exec", Path: name, Err: e}
	}

	// For Windows, syscall.StartProcess above already returned a process handle.
	if runtime.GOOS != "windows" {
		var ok bool
		h, ok = getPidfd(sysattr.Sys, shouldDupPidfd)
		if !ok {
			return newPIDProcess(pid), nil
		}
	}

	return newHandleProcess(pid, h), nil
}

func (p *Process) kill() error {
	return p.Signal(Kill)
}

func (p *Process) withHandle(f func(handle uintptr)) error {
	if p.handle == nil {
		return ErrNoHandle
	}
	handle, status := p.handleTransientAcquire()
	switch status {
	case statusDone:
		return ErrProcessDone
	case statusReleased:
		return errProcessReleased
	}
	defer p.handleTransientRelease()
	f(handle)

	return nil
}

// ProcessState stores information about a process, as reported by Wait.
type ProcessState struct {
	pid    int                // The process's id.
	status syscall.WaitStatus // System-dependent status info.
	rusage *syscall.Rusage
}

// Pid returns the process id of the exited process.
func (p *ProcessState) Pid() int {
	return p.pid
}

func (p *ProcessState) exited() bool {
	return p.status.Exited()
}

func (p *ProcessState) success() bool {
	return p.status.ExitStatus() == 0
}

func (p *ProcessState) sys() any {
	return p.status
}

func (p *ProcessState) sysUsage() any {
	return p.rusage
}

func (p *ProcessState) String() string {
	if p == nil {
		return "<nil>"
	}
	status := p.Sys().(syscall.WaitStatus)
	res := ""
	switch {
	case status.Exited():
		code := status.ExitStatus()
		if runtime.GOOS == "windows" && uint(code) >= 1<<16 { // windows uses large hex numbers
			res = "exit status 0x" + strconv.FormatUint(uint64(code), 16)
		} else { // unix systems use small decimal integers
			res = "exit status " + strconv.Itoa(code) // unix
		}
	case status.Signaled():
		res = "signal: " + status.Signal().String()
	case status.Stopped():
		res = "stop signal: " + status.StopSignal().String()
		if status.StopSignal() == syscall.SIGTRAP && status.TrapCause() != 0 {
			res += " (trap " + strconv.Itoa(status.TrapCause()) + ")"
		}
	case status.Continued():
		res = "continued"
	}
	if status.CoreDump() {
		res += " (core dumped)"
	}
	return res
}

// ExitCode returns the exit code of the exited process, or -1
// if the process hasn't exited or was terminated by a signal.
func (p *ProcessState) ExitCode() int {
	// return -1 if the process hasn't started.
	if p == nil {
		return -1
	}
	return p.status.ExitStatus()
}

```

// === FILE: references!/go/src/os/exec_unix.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || (js && wasm) || wasip1

package os

import (
	"errors"
	"syscall"
	"time"
)

const (
	// Special values for Process.Pid.
	pidUnset    = 0
	pidReleased = -1
)

func (p *Process) wait() (ps *ProcessState, err error) {
	// Which type of Process do we have?
	if p.handle != nil {
		// pidfd
		return p.pidfdWait()
	} else {
		// Regular PID
		return p.pidWait()
	}
}

func (p *Process) pidWait() (*ProcessState, error) {
	// TODO(go.dev/issue/67642): When there are concurrent Wait calls, one
	// may wait on the wrong process if the PID is reused after the
	// completes its wait.
	//
	// Checking for statusDone here would not be a complete fix, as the PID
	// could still be waited on and reused prior to blockUntilWaitable.
	switch p.pidStatus() {
	case statusReleased:
		return nil, syscall.EINVAL
	}

	// If we can block until Wait4 will succeed immediately, do so.
	ready, err := p.blockUntilWaitable()
	if err != nil {
		return nil, err
	}
	if ready {
		// Mark the process done now, before the call to Wait4,
		// so that Process.pidSignal will not send a signal.
		p.doRelease(statusDone)
		// Acquire a write lock on sigMu to wait for any
		// active call to the signal method to complete.
		p.sigMu.Lock()
		p.sigMu.Unlock()
	}

	var (
		status syscall.WaitStatus
		rusage syscall.Rusage
	)
	pid1, err := ignoringEINTR2(func() (int, error) {
		return syscall.Wait4(p.Pid, &status, 0, &rusage)
	})
	if err != nil {
		return nil, NewSyscallError("wait", err)
	}
	p.doRelease(statusDone)
	return &ProcessState{
		pid:    pid1,
		status: status,
		rusage: &rusage,
	}, nil
}

func (p *Process) signal(sig Signal) error {
	s, ok := sig.(syscall.Signal)
	if !ok {
		return errors.New("os: unsupported signal type")
	}

	// Which type of Process do we have?
	if p.handle != nil {
		// pidfd
		return p.pidfdSendSignal(s)
	} else {
		// Regular PID
		return p.pidSignal(s)
	}
}

func (p *Process) pidSignal(s syscall.Signal) error {
	if p.Pid == pidReleased {
		return errProcessReleased
	}
	if p.Pid == pidUnset {
		return errors.New("os: process not initialized")
	}

	p.sigMu.RLock()
	defer p.sigMu.RUnlock()

	switch p.pidStatus() {
	case statusDone:
		return ErrProcessDone
	case statusReleased:
		return errProcessReleased
	}

	return convertESRCH(syscall.Kill(p.Pid, s))
}

func convertESRCH(err error) error {
	if err == syscall.ESRCH {
		return ErrProcessDone
	}
	return err
}

func findProcess(pid int) (p *Process, err error) {
	h, err := pidfdFind(pid)
	if err == ErrProcessDone {
		// We can't return an error here since users are not expecting
		// it. Instead, return a process with a "done" state already
		// and let a subsequent Signal or Wait call catch that.
		return newDoneProcess(pid), nil
	} else if err != nil {
		// Ignore other errors from pidfdFind, as the callers
		// do not expect them. Fall back to using the PID.
		return newPIDProcess(pid), nil
	}
	// Use the handle.
	return newHandleProcess(pid, h), nil
}

func (p *ProcessState) userTime() time.Duration {
	return time.Duration(p.rusage.Utime.Nano()) * time.Nanosecond
}

func (p *ProcessState) systemTime() time.Duration {
	return time.Duration(p.rusage.Stime.Nano()) * time.Nanosecond
}

```

// === FILE: references!/go/src/os/exec_windows.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"errors"
	"internal/syscall/windows"
	"runtime"
	"syscall"
	"time"
)

// Note that Process.handle is never nil because Windows always requires
// a handle. A manually-created Process literal is not valid.

func (p *Process) wait() (ps *ProcessState, err error) {
	handle, status := p.handleTransientAcquire()
	switch status {
	case statusDone:
		return nil, ErrProcessDone
	case statusReleased:
		return nil, syscall.EINVAL
	}
	defer p.handleTransientRelease()

	s, e := syscall.WaitForSingleObject(syscall.Handle(handle), syscall.INFINITE)
	switch s {
	case syscall.WAIT_OBJECT_0:
		break
	case syscall.WAIT_FAILED:
		return nil, NewSyscallError("WaitForSingleObject", e)
	default:
		return nil, errors.New("os: unexpected result from WaitForSingleObject")
	}
	var ec uint32
	e = syscall.GetExitCodeProcess(syscall.Handle(handle), &ec)
	if e != nil {
		return nil, NewSyscallError("GetExitCodeProcess", e)
	}
	var u syscall.Rusage
	e = syscall.GetProcessTimes(syscall.Handle(handle), &u.CreationTime, &u.ExitTime, &u.KernelTime, &u.UserTime)
	if e != nil {
		return nil, NewSyscallError("GetProcessTimes", e)
	}

	// For compatibility we use statusReleased here rather
	// than statusDone.
	p.doRelease(statusReleased)

	return &ProcessState{p.Pid, syscall.WaitStatus{ExitCode: ec}, &u}, nil
}

func (p *Process) signal(sig Signal) error {
	handle, status := p.handleTransientAcquire()
	switch status {
	case statusDone:
		return ErrProcessDone
	case statusReleased:
		return syscall.EINVAL
	}
	defer p.handleTransientRelease()

	if sig == Kill {
		var terminationHandle syscall.Handle
		e := syscall.DuplicateHandle(^syscall.Handle(0), syscall.Handle(handle), ^syscall.Handle(0), &terminationHandle, syscall.PROCESS_TERMINATE, false, 0)
		if e != nil {
			return NewSyscallError("DuplicateHandle", e)
		}
		runtime.KeepAlive(p)
		defer syscall.CloseHandle(terminationHandle)
		e = syscall.TerminateProcess(syscall.Handle(terminationHandle), 1)
		return NewSyscallError("TerminateProcess", e)
	}
	// TODO(rsc): Handle Interrupt too?
	return syscall.Errno(syscall.EWINDOWS)
}

func (ph *processHandle) closeHandle() {
	syscall.CloseHandle(syscall.Handle(ph.handle))
}

func findProcess(pid int) (p *Process, err error) {
	const da = syscall.STANDARD_RIGHTS_READ |
		syscall.PROCESS_QUERY_INFORMATION | syscall.SYNCHRONIZE
	h, e := syscall.OpenProcess(da, false, uint32(pid))
	if e != nil {
		return nil, NewSyscallError("OpenProcess", e)
	}
	return newHandleProcess(pid, uintptr(h)), nil
}

func init() {
	cmd := windows.UTF16PtrToString(syscall.GetCommandLine())
	if len(cmd) == 0 {
		arg0, _ := Executable()
		Args = []string{arg0}
	} else {
		Args = commandLineToArgv(cmd)
	}
}

// appendBSBytes appends n '\\' bytes to b and returns the resulting slice.
func appendBSBytes(b []byte, n int) []byte {
	for ; n > 0; n-- {
		b = append(b, '\\')
	}
	return b
}

// readNextArg splits command line string cmd into next
// argument and command line remainder.
func readNextArg(cmd string) (arg []byte, rest string) {
	var b []byte
	var inquote bool
	var nslash int
	for ; len(cmd) > 0; cmd = cmd[1:] {
		c := cmd[0]
		switch c {
		case ' ', '\t':
			if !inquote {
				return appendBSBytes(b, nslash), cmd[1:]
			}
		case '"':
			b = appendBSBytes(b, nslash/2)
			if nslash%2 == 0 {
				// use "Prior to 2008" rule from
				// http://daviddeley.com/autohotkey/parameters/parameters.htm
				// section 5.2 to deal with double double quotes
				if inquote && len(cmd) > 1 && cmd[1] == '"' {
					b = append(b, c)
					cmd = cmd[1:]
				}
				inquote = !inquote
			} else {
				b = append(b, c)
			}
			nslash = 0
			continue
		case '\\':
			nslash++
			continue
		}
		b = appendBSBytes(b, nslash)
		nslash = 0
		b = append(b, c)
	}
	return appendBSBytes(b, nslash), ""
}

// commandLineToArgv splits a command line into individual argument
// strings, following the Windows conventions documented
// at http://daviddeley.com/autohotkey/parameters/parameters.htm#WINARGV
func commandLineToArgv(cmd string) []string {
	var args []string
	for len(cmd) > 0 {
		if cmd[0] == ' ' || cmd[0] == '\t' {
			cmd = cmd[1:]
			continue
		}
		var arg []byte
		arg, cmd = readNextArg(cmd)
		args = append(args, string(arg))
	}
	return args
}

func ftToDuration(ft *syscall.Filetime) time.Duration {
	n := int64(ft.HighDateTime)<<32 + int64(ft.LowDateTime) // in 100-nanosecond intervals
	return time.Duration(n*100) * time.Nanosecond
}

func (p *ProcessState) userTime() time.Duration {
	return ftToDuration(&p.rusage.UserTime)
}

func (p *ProcessState) systemTime() time.Duration {
	return ftToDuration(&p.rusage.KernelTime)
}

```

// === FILE: references!/go/src/os/executable.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

// Executable returns the path name for the executable that started
// the current process. There is no guarantee that the path is still
// pointing to the correct executable. If a symlink was used to start
// the process, depending on the operating system, the result might
// be the symlink or the path it pointed to. If a stable result is
// needed, [path/filepath.EvalSymlinks] might help.
//
// Executable returns an absolute path unless an error occurred.
//
// The main use case is finding resources located relative to an
// executable.
func Executable() (string, error) {
	return executable()
}

```

// === FILE: references!/go/src/os/executable_darwin.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"errors"
	_ "unsafe" // for linkname
)

//go:linkname executablePath
var executablePath string // set by ../runtime/os_darwin.go

var initCwd, initCwdErr = Getwd()

func executable() (string, error) {
	ep := executablePath
	if len(ep) == 0 {
		return ep, errors.New("cannot find executable path")
	}
	if ep[0] != '/' {
		if initCwdErr != nil {
			return ep, initCwdErr
		}
		if len(ep) > 2 && ep[0:2] == "./" {
			// skip "./"
			ep = ep[2:]
		}
		ep = initCwd + "/" + ep
	}
	return ep, nil
}

```

// === FILE: references!/go/src/os/executable_dragonfly.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

// From DragonFly's <sys/sysctl.h>
const (
	_CTL_KERN           = 1
	_KERN_PROC          = 14
	_KERN_PROC_PATHNAME = 9
)

var executableMIB = [4]int32{_CTL_KERN, _KERN_PROC, _KERN_PROC_PATHNAME, -1}

```

// === FILE: references!/go/src/os/executable_freebsd.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

// From FreeBSD's <sys/sysctl.h>
const (
	_CTL_KERN           = 1
	_KERN_PROC          = 14
	_KERN_PROC_PATHNAME = 12
)

var executableMIB = [4]int32{_CTL_KERN, _KERN_PROC, _KERN_PROC_PATHNAME, -1}

```

// === FILE: references!/go/src/os/executable_netbsd.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

// From NetBSD's <sys/sysctl.h>
const (
	_CTL_KERN           = 1
	_KERN_PROC_ARGS     = 48
	_KERN_PROC_PATHNAME = 5
)

var executableMIB = [4]int32{_CTL_KERN, _KERN_PROC_ARGS, -1, _KERN_PROC_PATHNAME}

```

// === FILE: references!/go/src/os/executable_path.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || openbsd

package os

// We query the working directory at init, to use it later to search for the
// executable file
// errWd will be checked later, if we need to use initWd
var initWd, errWd = Getwd()

func executable() (string, error) {
	var exePath string
	if len(Args) == 0 || Args[0] == "" {
		return "", ErrNotExist
	}
	if IsPathSeparator(Args[0][0]) {
		// Args[0] is an absolute path, so it is the executable.
		// Note that we only need to worry about Unix paths here.
		exePath = Args[0]
	} else {
		for i := 1; i < len(Args[0]); i++ {
			if IsPathSeparator(Args[0][i]) {
				// Args[0] is a relative path: prepend the
				// initial working directory.
				if errWd != nil {
					return "", errWd
				}
				exePath = initWd + string(PathSeparator) + Args[0]
				break
			}
		}
	}
	if exePath != "" {
		if err := isExecutable(exePath); err != nil {
			return "", err
		}
		return exePath, nil
	}
	// Search for executable in $PATH.
	for _, dir := range splitPathList(Getenv("PATH")) {
		if len(dir) == 0 {
			dir = "."
		}
		if !IsPathSeparator(dir[0]) {
			if errWd != nil {
				return "", errWd
			}
			dir = initWd + string(PathSeparator) + dir
		}
		exePath = dir + string(PathSeparator) + Args[0]
		switch isExecutable(exePath) {
		case nil:
			return exePath, nil
		case ErrPermission:
			return "", ErrPermission
		}
	}
	return "", ErrNotExist
}

// isExecutable returns an error if a given file is not an executable.
func isExecutable(path string) error {
	stat, err := Stat(path)
	if err != nil {
		return err
	}
	mode := stat.Mode()
	if !mode.IsRegular() {
		return ErrPermission
	}
	if (mode & 0111) == 0 {
		return ErrPermission
	}
	return nil
}

// splitPathList splits a path list.
// This is based on genSplit from strings/strings.go
func splitPathList(pathList string) []string {
	if pathList == "" {
		return nil
	}
	n := 1
	for i := 0; i < len(pathList); i++ {
		if pathList[i] == PathListSeparator {
			n++
		}
	}
	start := 0
	a := make([]string, n)
	na := 0
	for i := 0; i+1 <= len(pathList) && na+1 < n; i++ {
		if pathList[i] == PathListSeparator {
			a[na] = pathList[start:i]
			na++
			start = i + 1
		}
	}
	a[na] = pathList[start:]
	return a[:na+1]
}

```

// === FILE: references!/go/src/os/executable_plan9.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build plan9

package os

import (
	"internal/strconv"
	"syscall"
)

func executable() (string, error) {
	fn := "/proc/" + strconv.Itoa(Getpid()) + "/text"
	f, err := Open(fn)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return syscall.Fd2path(int(f.Fd()))
}

```

// === FILE: references!/go/src/os/executable_procfs.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux

package os

import (
	"errors"
	"internal/stringslite"
	"runtime"
)

func executable() (string, error) {
	var procfn string
	switch runtime.GOOS {
	default:
		return "", errors.New("Executable not implemented for " + runtime.GOOS)
	case "linux", "android":
		procfn = "/proc/self/exe"
	}
	path, err := Readlink(procfn)

	// When the executable has been deleted then Readlink returns a
	// path appended with " (deleted)".
	return stringslite.TrimSuffix(path, " (deleted)"), err
}

```

// === FILE: references!/go/src/os/executable_solaris.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	_ "unsafe" // for linkname
)

//go:linkname executablePath
var executablePath string // set by sysauxv in ../runtime/os3_solaris.go

var initCwd, initCwdErr = Getwd()

func executable() (string, error) {
	path := executablePath
	if len(path) == 0 {
		path, err := syscall.Getexecname()
		if err != nil {
			return path, err
		}
	}
	if len(path) > 0 && path[0] != '/' {
		if initCwdErr != nil {
			return path, initCwdErr
		}
		if len(path) > 2 && path[0:2] == "./" {
			// skip "./"
			path = path[2:]
		}
		return initCwd + "/" + path, nil
	}
	return path, nil
}

```

// === FILE: references!/go/src/os/executable_sysctl.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build freebsd || dragonfly || netbsd

package os

import (
	"syscall"
	"unsafe"
)

func executable() (string, error) {
	n := uintptr(0)
	// get length
	_, _, err := syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&executableMIB[0])), 4, 0, uintptr(unsafe.Pointer(&n)), 0, 0)
	if err != 0 {
		return "", err
	}
	if n == 0 { // shouldn't happen
		return "", nil
	}
	buf := make([]byte, n)
	_, _, err = syscall.Syscall6(syscall.SYS___SYSCTL, uintptr(unsafe.Pointer(&executableMIB[0])), 4, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)), 0, 0)
	if err != 0 {
		return "", err
	}
	if n == 0 { // shouldn't happen
		return "", nil
	}
	return string(buf[:n-1]), nil
}

```

// === FILE: references!/go/src/os/executable_wasm.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build wasm

package os

import (
	"errors"
	"runtime"
)

func executable() (string, error) {
	return "", errors.New("Executable not implemented for " + runtime.GOOS)
}

```

// === FILE: references!/go/src/os/executable_windows.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/syscall/windows"
	"syscall"
)

func getModuleFileName(handle syscall.Handle) (string, error) {
	n := uint32(1024)
	var buf []uint16
	for {
		buf = make([]uint16, n)
		r, err := windows.GetModuleFileName(handle, &buf[0], n)
		if err != nil {
			return "", err
		}
		if r < n {
			break
		}
		// r == n means n not big enough
		n += 1024
	}
	return syscall.UTF16ToString(buf), nil
}

func executable() (string, error) {
	return getModuleFileName(0)
}

```

// === FILE: references!/go/src/os/file.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package os provides a platform-independent interface to operating system
// functionality. The design is Unix-like, although the error handling is
// Go-like; failing calls return values of type error rather than error numbers.
// Often, more information is available within the error. For example,
// if a call that takes a file name fails, such as [Open] or [Stat], the error
// will include the failing file name when printed and will be of type
// [*PathError], which may be unpacked for more information.
//
// The os interface is intended to be uniform across all operating systems.
// Features not generally available appear in the system-specific package syscall.
//
// Here is a simple example, opening a file and reading some of it.
//
//	file, err := os.Open("file.go") // For read access.
//	if err != nil {
//		log.Fatal(err)
//	}
//
// If the open fails, the error string will be self-explanatory, like
//
//	open file.go: no such file or directory
//
// The file's data can then be read into a slice of bytes. Read and
// Write take their byte counts from the length of the argument slice.
//
//	data := make([]byte, 100)
//	count, err := file.Read(data)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("read %d bytes: %q\n", count, data[:count])
//
// # Concurrency
//
// The methods of [File] correspond to file system operations. All are
// safe for concurrent use. The maximum number of concurrent
// operations on a File may be limited by the OS or the system. The
// number should be high, but exceeding it may degrade performance or
// cause other issues.
package os

import (
	"errors"
	"internal/filepathlite"
	"internal/poll"
	"internal/testlog"
	"io"
	"io/fs"
	"runtime"
	"slices"
	"syscall"
	"time"
	"unsafe"
)

// Name returns the name of the file as presented to Open.
//
// It is safe to call Name after [Close].
func (f *File) Name() string { return f.name }

// Stdin, Stdout, and Stderr are open Files pointing to the standard input,
// standard output, and standard error file descriptors.
//
// Note that the Go runtime writes to standard error for panics and crashes;
// closing Stderr may cause those messages to go elsewhere, perhaps
// to a file opened later.
var (
	Stdin  = NewFile(uintptr(syscall.Stdin), "/dev/stdin")
	Stdout = NewFile(uintptr(syscall.Stdout), "/dev/stdout")
	Stderr = NewFile(uintptr(syscall.Stderr), "/dev/stderr")
)

// Flags to OpenFile wrapping those of the underlying system. Not all
// flags may be implemented on a given system.
const (
	// Exactly one of O_RDONLY, O_WRONLY, or O_RDWR must be specified.
	O_RDONLY int = syscall.O_RDONLY // open the file read-only.
	O_WRONLY int = syscall.O_WRONLY // open the file write-only.
	O_RDWR   int = syscall.O_RDWR   // open the file read-write.
	// The remaining values may be or'ed in to control behavior.
	O_APPEND int = syscall.O_APPEND // append data to the file when writing.
	O_CREATE int = syscall.O_CREAT  // create a new file if none exists.
	O_EXCL   int = syscall.O_EXCL   // used with O_CREATE, file must not exist.
	O_SYNC   int = syscall.O_SYNC   // open for synchronous I/O.
	O_TRUNC  int = syscall.O_TRUNC  // truncate regular writable file when opened.
)

// Seek whence values.
//
// Deprecated: Use io.SeekStart, io.SeekCurrent, and io.SeekEnd.
const (
	SEEK_SET int = 0 // seek relative to the origin of the file
	SEEK_CUR int = 1 // seek relative to the current offset
	SEEK_END int = 2 // seek relative to the end
)

// LinkError records an error during a link or symlink or rename
// system call and the paths that caused it.
type LinkError struct {
	Op  string
	Old string
	New string
	Err error
}

func (e *LinkError) Error() string {
	return e.Op + " " + e.Old + " " + e.New + ": " + e.Err.Error()
}

func (e *LinkError) Unwrap() error {
	return e.Err
}

// NewFile returns a new [File] with the given file descriptor and name.
// The returned value will be nil if fd is not a valid file descriptor.
//
// NewFile's behavior differs on some platforms:
//
//   - On Unix, if fd is in non-blocking mode, NewFile will attempt to return a pollable file.
//   - On Windows, if fd is opened for asynchronous I/O (that is, [syscall.FILE_FLAG_OVERLAPPED]
//     has been specified in the [syscall.CreateFile] call), NewFile will attempt to return a pollable
//     file by associating fd with the Go runtime I/O completion port.
//     The I/O operations will be performed synchronously if the association fails.
//
// Only pollable files support [File.SetDeadline], [File.SetReadDeadline], and [File.SetWriteDeadline].
//
// After passing it to NewFile, fd may become invalid under the same conditions described
// in the comments of [File.Fd], and the same constraints apply.
func NewFile(fd uintptr, name string) *File {
	return newFileFromNewFile(fd, name)
}

// Read reads up to len(b) bytes from the File and stores them in b.
// It returns the number of bytes read and any error encountered.
// At end of file, Read returns 0, io.EOF.
func (f *File) Read(b []byte) (n int, err error) {
	if err := f.checkValid("read"); err != nil {
		return 0, err
	}
	n, e := f.read(b)
	return n, f.wrapErr("read", e)
}

// ReadAt reads len(b) bytes from the File starting at byte offset off.
// It returns the number of bytes read and the error, if any.
// ReadAt always returns a non-nil error when n < len(b).
// At end of file, that error is io.EOF.
func (f *File) ReadAt(b []byte, off int64) (n int, err error) {
	if err := f.checkValid("read"); err != nil {
		return 0, err
	}

	if off < 0 {
		return 0, &PathError{Op: "readat", Path: f.name, Err: errors.New("negative offset")}
	}

	for len(b) > 0 {
		m, e := f.pread(b, off)
		if e != nil {
			err = f.wrapErr("read", e)
			break
		}
		n += m
		b = b[m:]
		off += int64(m)
	}
	return
}

// ReadFrom implements io.ReaderFrom.
func (f *File) ReadFrom(r io.Reader) (n int64, err error) {
	if err := f.checkValid("write"); err != nil {
		return 0, err
	}
	n, handled, e := f.readFrom(r)
	if !handled {
		return genericReadFrom(f, r) // without wrapping
	}
	return n, f.wrapErr("write", e)
}

// noReadFrom can be embedded alongside another type to
// hide the ReadFrom method of that other type.
type noReadFrom struct{}

// ReadFrom hides another ReadFrom method.
// It should never be called.
func (noReadFrom) ReadFrom(io.Reader) (int64, error) {
	panic("can't happen")
}

// fileWithoutReadFrom implements all the methods of *File other
// than ReadFrom. This is used to permit ReadFrom to call io.Copy
// without leading to a recursive call to ReadFrom.
type fileWithoutReadFrom struct {
	noReadFrom
	*File
}

func genericReadFrom(f *File, r io.Reader) (int64, error) {
	return io.Copy(fileWithoutReadFrom{File: f}, r)
}

// Write writes len(b) bytes from b to the File.
// It returns the number of bytes written and an error, if any.
// Write returns a non-nil error when n != len(b).
func (f *File) Write(b []byte) (n int, err error) {
	if err := f.checkValid("write"); err != nil {
		return 0, err
	}
	n, e := f.write(b)
	if n < 0 {
		n = 0
	}
	if n != len(b) {
		err = io.ErrShortWrite
	}

	epipecheck(f, e)

	if e != nil {
		err = f.wrapErr("write", e)
	}

	return n, err
}

var errWriteAtInAppendMode = errors.New("os: invalid use of WriteAt on file opened with O_APPEND")

// WriteAt writes len(b) bytes to the File starting at byte offset off.
// It returns the number of bytes written and an error, if any.
// WriteAt returns a non-nil error when n != len(b).
//
// If file was opened with the [O_APPEND] flag, WriteAt returns an error.
func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	if err := f.checkValid("write"); err != nil {
		return 0, err
	}
	if f.appendMode {
		return 0, errWriteAtInAppendMode
	}

	if off < 0 {
		return 0, &PathError{Op: "writeat", Path: f.name, Err: errors.New("negative offset")}
	}

	for len(b) > 0 {
		m, e := f.pwrite(b, off)
		if e != nil {
			err = f.wrapErr("write", e)
			break
		}
		n += m
		b = b[m:]
		off += int64(m)
	}
	return
}

// WriteTo implements io.WriterTo.
func (f *File) WriteTo(w io.Writer) (n int64, err error) {
	if err := f.checkValid("read"); err != nil {
		return 0, err
	}
	n, handled, e := f.writeTo(w)
	if handled {
		return n, f.wrapErr("read", e)
	}
	return genericWriteTo(f, w) // without wrapping
}

// noWriteTo can be embedded alongside another type to
// hide the WriteTo method of that other type.
type noWriteTo struct{}

// WriteTo hides another WriteTo method.
// It should never be called.
func (noWriteTo) WriteTo(io.Writer) (int64, error) {
	panic("can't happen")
}

// fileWithoutWriteTo implements all the methods of *File other
// than WriteTo. This is used to permit WriteTo to call io.Copy
// without leading to a recursive call to WriteTo.
type fileWithoutWriteTo struct {
	noWriteTo
	*File
}

func genericWriteTo(f *File, w io.Writer) (int64, error) {
	return io.Copy(w, fileWithoutWriteTo{File: f})
}

// Seek sets the offset for the next Read or Write on file to offset, interpreted
// according to whence: 0 means relative to the origin of the file, 1 means
// relative to the current offset, and 2 means relative to the end.
// It returns the new offset and an error, if any.
// The behavior of Seek on a file opened with [O_APPEND] is not specified.
func (f *File) Seek(offset int64, whence int) (ret int64, err error) {
	if err := f.checkValid("seek"); err != nil {
		return 0, err
	}
	r, e := f.seek(offset, whence)
	if e == nil && f.dirinfo.Load() != nil && r != 0 {
		e = syscall.EISDIR
	}
	if e != nil {
		return 0, f.wrapErr("seek", e)
	}
	return r, nil
}

// WriteString is like Write, but writes the contents of string s rather than
// a slice of bytes.
func (f *File) WriteString(s string) (n int, err error) {
	b := unsafe.Slice(unsafe.StringData(s), len(s))
	return f.Write(b)
}

// Mkdir creates a new directory with the specified name and permission
// bits (before umask).
// If there is an error, it will be of type [*PathError].
func Mkdir(name string, perm FileMode) error {
	longName := fixLongPath(name)
	e := ignoringEINTR(func() error {
		return syscall.Mkdir(longName, syscallMode(perm))
	})

	if e != nil {
		return &PathError{Op: "mkdir", Path: name, Err: e}
	}

	// mkdir(2) itself won't handle the sticky bit on *BSD and Solaris
	if !supportsCreateWithStickyBit && perm&ModeSticky != 0 {
		e = setStickyBit(name)

		if e != nil {
			Remove(name)
			return e
		}
	}

	return nil
}

// setStickyBit adds ModeSticky to the permission bits of path, non atomic.
func setStickyBit(name string) error {
	fi, err := Stat(name)
	if err != nil {
		return err
	}
	return Chmod(name, fi.Mode()|ModeSticky)
}

// Chdir changes the current working directory to the named directory.
// If there is an error, it will be of type [*PathError].
func Chdir(dir string) error {
	if e := syscall.Chdir(dir); e != nil {
		testlog.Open(dir) // observe likely non-existent directory
		return &PathError{Op: "chdir", Path: dir, Err: e}
	}
	if runtime.GOOS == "windows" {
		abs := filepathlite.IsAbs(dir)
		getwdCache.Lock()
		if abs {
			getwdCache.dir = dir
		} else {
			getwdCache.dir = ""
		}
		getwdCache.Unlock()
	}
	if log := testlog.Logger(); log != nil {
		wd, err := Getwd()
		if err == nil {
			log.Chdir(wd)
		}
	}
	return nil
}

// Open opens the named file for reading. If successful, methods on
// the returned file can be used for reading; the associated file
// descriptor has mode [O_RDONLY].
// If there is an error, it will be of type [*PathError].
func Open(name string) (*File, error) {
	return OpenFile(name, O_RDONLY, 0)
}

// Create creates or truncates the named file. If the file already exists,
// it is truncated. If the file does not exist, it is created with mode 0o666
// (before umask). If successful, methods on the returned File can
// be used for I/O; the associated file descriptor has mode [O_RDWR].
// The directory containing the file must already exist.
// If there is an error, it will be of type [*PathError].
func Create(name string) (*File, error) {
	return OpenFile(name, O_RDWR|O_CREATE|O_TRUNC, 0666)
}

// OpenFile is the generalized open call; most users will use Open
// or Create instead. It opens the named file with specified flag
// ([O_RDONLY] etc.). If the file does not exist, and the [O_CREATE] flag
// is passed, it is created with mode perm (before umask);
// the containing directory must exist. If successful,
// methods on the returned File can be used for I/O.
// If there is an error, it will be of type [*PathError].
func OpenFile(name string, flag int, perm FileMode) (*File, error) {
	testlog.Open(name)
	f, err := openFileNolog(name, flag, perm)
	if err != nil {
		return nil, err
	}
	f.appendMode = flag&O_APPEND != 0

	return f, nil
}

var errPathEscapes = errors.New("path escapes from parent")

// openDir opens a file which is assumed to be a directory. As such, it skips
// the syscalls that make the file descriptor non-blocking as these take time
// and will fail on file descriptors for directories.
func openDir(name string) (*File, error) {
	testlog.Open(name)
	return openDirNolog(name)
}

// Rename renames (moves) oldpath to newpath.
// If newpath already exists and is not a directory, Rename replaces it.
// If newpath already exists and is a directory, Rename returns an error.
// OS-specific restrictions may apply when oldpath and newpath are in different directories.
// Even within the same directory, on non-Unix platforms Rename is not an atomic operation.
// If there is an error, it will be of type *LinkError.
func Rename(oldpath, newpath string) error {
	return rename(oldpath, newpath)
}

// Readlink returns the destination of the named symbolic link.
// If there is an error, it will be of type [*PathError].
//
// If the link destination is relative, Readlink returns the relative path
// without resolving it to an absolute one.
func Readlink(name string) (string, error) {
	return readlink(name)
}

// Many functions in package syscall return a count of -1 instead of 0.
// Using fixCount(call()) instead of call() corrects the count.
func fixCount(n int, err error) (int, error) {
	if n < 0 {
		n = 0
	}
	return n, err
}

// checkWrapErr is the test hook to enable checking unexpected wrapped errors of poll.ErrFileClosing.
// It is set to true in the export_test.go for tests (including fuzz tests).
var checkWrapErr = false

// wrapErr wraps an error that occurred during an operation on an open file.
// It passes io.EOF through unchanged, otherwise converts
// poll.ErrFileClosing to ErrClosed and wraps the error in a PathError.
func (f *File) wrapErr(op string, err error) error {
	if err == nil || err == io.EOF {
		return err
	}
	if err == poll.ErrFileClosing {
		err = ErrClosed
	} else if checkWrapErr && errors.Is(err, poll.ErrFileClosing) {
		panic("unexpected error wrapping poll.ErrFileClosing: " + err.Error())
	}
	return &PathError{Op: op, Path: f.name, Err: err}
}

// TempDir returns the default directory to use for temporary files.
//
// On Unix systems, it returns $TMPDIR if non-empty, else /tmp.
// On Windows, it uses GetTempPath, returning the first non-empty
// value from %TMP%, %TEMP%, %USERPROFILE%, or the Windows directory.
// On Plan 9, it returns /tmp.
//
// The directory is neither guaranteed to exist nor have accessible
// permissions.
func TempDir() string {
	return tempDir()
}

// UserCacheDir returns the default root directory to use for user-specific
// cached data. Users should create their own application-specific subdirectory
// within this one and use that.
//
// On Unix systems, it returns $XDG_CACHE_HOME as specified by
// https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html if
// non-empty, else $HOME/.cache.
// On Darwin, it returns $HOME/Library/Caches.
// On Windows, it returns %LocalAppData%.
// On Plan 9, it returns $home/lib/cache.
//
// If the location cannot be determined (for example, $HOME is not defined) or
// the path in $XDG_CACHE_HOME is relative, then it will return an error.
func UserCacheDir() (string, error) {
	var dir string

	switch runtime.GOOS {
	case "windows":
		dir = Getenv("LocalAppData")
		if dir == "" {
			return "", errors.New("%LocalAppData% is not defined")
		}

	case "darwin", "ios":
		dir = Getenv("HOME")
		if dir == "" {
			return "", errors.New("$HOME is not defined")
		}
		dir += "/Library/Caches"

	case "plan9":
		dir = Getenv("home")
		if dir == "" {
			return "", errors.New("$home is not defined")
		}
		dir += "/lib/cache"

	default: // Unix
		dir = Getenv("XDG_CACHE_HOME")
		if dir == "" {
			dir = Getenv("HOME")
			if dir == "" {
				return "", errors.New("neither $XDG_CACHE_HOME nor $HOME are defined")
			}
			dir += "/.cache"
		} else if !filepathlite.IsAbs(dir) {
			return "", errors.New("path in $XDG_CACHE_HOME is relative")
		}
	}

	return dir, nil
}

// UserConfigDir returns the default root directory to use for user-specific
// configuration data. Users should create their own application-specific
// subdirectory within this one and use that.
//
// On Unix systems, it returns $XDG_CONFIG_HOME as specified by
// https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html if
// non-empty, else $HOME/.config.
// On Darwin, it returns $HOME/Library/Application Support.
// On Windows, it returns %AppData%.
// On Plan 9, it returns $home/lib.
//
// If the location cannot be determined (for example, $HOME is not defined) or
// the path in $XDG_CONFIG_HOME is relative, then it will return an error.
func UserConfigDir() (string, error) {
	var dir string

	switch runtime.GOOS {
	case "windows":
		dir = Getenv("AppData")
		if dir == "" {
			return "", errors.New("%AppData% is not defined")
		}

	case "darwin", "ios":
		dir = Getenv("HOME")
		if dir == "" {
			return "", errors.New("$HOME is not defined")
		}
		dir += "/Library/Application Support"

	case "plan9":
		dir = Getenv("home")
		if dir == "" {
			return "", errors.New("$home is not defined")
		}
		dir += "/lib"

	default: // Unix
		dir = Getenv("XDG_CONFIG_HOME")
		if dir == "" {
			dir = Getenv("HOME")
			if dir == "" {
				return "", errors.New("neither $XDG_CONFIG_HOME nor $HOME are defined")
			}
			dir += "/.config"
		} else if !filepathlite.IsAbs(dir) {
			return "", errors.New("path in $XDG_CONFIG_HOME is relative")
		}
	}

	return dir, nil
}

// UserHomeDir returns the current user's home directory.
//
// On Unix, including macOS, it returns the $HOME environment variable.
// On Windows, it returns %USERPROFILE%.
// On Plan 9, it returns the $home environment variable.
//
// If the expected variable is not set in the environment, UserHomeDir
// returns either a platform-specific default value or a non-nil error.
func UserHomeDir() (string, error) {
	env, enverr := "HOME", "$HOME"
	switch runtime.GOOS {
	case "windows":
		env, enverr = "USERPROFILE", "%userprofile%"
	case "plan9":
		env, enverr = "home", "$home"
	}
	if v := Getenv(env); v != "" {
		return v, nil
	}
	// On some operating systems the home directory is not always defined.
	switch runtime.GOOS {
	case "android":
		return "/sdcard", nil
	case "ios":
		return "/", nil
	}
	return "", errors.New(enverr + " is not defined")
}

// Chmod changes the mode of the named file to mode.
// If the file is a symbolic link, it changes the mode of the link's target.
// If there is an error, it will be of type [*PathError].
//
// A different subset of the mode bits are used, depending on the
// operating system.
//
// On Unix, the mode's permission bits, [ModeSetuid], [ModeSetgid], and
// [ModeSticky] are used.
//
// On Windows, only the 0o200 bit (owner writable) of mode is used; it
// controls whether the file's read-only attribute is set or cleared.
// The other bits are currently unused. For compatibility with Go 1.12
// and earlier, use a non-zero mode. Use mode 0o400 for a read-only
// file and 0o600 for a readable+writable file.
//
// On Plan 9, the mode's permission bits, [ModeAppend], [ModeExclusive],
// and [ModeTemporary] are used.
func Chmod(name string, mode FileMode) error { return chmod(name, mode) }

// Chmod changes the mode of the file to mode.
// If there is an error, it will be of type [*PathError].
func (f *File) Chmod(mode FileMode) error { return f.chmod(mode) }

// SetDeadline sets the read and write deadlines for a File.
// It is equivalent to calling both SetReadDeadline and SetWriteDeadline.
//
// Only some kinds of files support setting a deadline. Calls to SetDeadline
// for files that do not support deadlines will return ErrNoDeadline.
// On most systems ordinary files do not support deadlines, but pipes do.
//
// A deadline is an absolute time after which I/O operations fail with an
// error instead of blocking. The deadline applies to all future and pending
// I/O, not just the immediately following call to Read or Write.
// After a deadline has been exceeded, the connection can be refreshed
// by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other I/O
// methods will return an error that wraps ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// That error implements the Timeout method, and calling the Timeout
// method will return true, but there are other possible errors for which
// the Timeout will return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (f *File) SetDeadline(t time.Time) error {
	return f.setDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls and any
// currently-blocked Read call.
// A zero value for t means Read will not time out.
// Not all files support setting deadlines; see SetDeadline.
func (f *File) SetReadDeadline(t time.Time) error {
	return f.setReadDeadline(t)
}

// SetWriteDeadline sets the deadline for any future Write calls and any
// currently-blocked Write call.
// Even if Write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
// Not all files support setting deadlines; see SetDeadline.
func (f *File) SetWriteDeadline(t time.Time) error {
	return f.setWriteDeadline(t)
}

// SyscallConn returns a raw file.
// This implements the syscall.Conn interface.
func (f *File) SyscallConn() (syscall.RawConn, error) {
	if err := f.checkValid("SyscallConn"); err != nil {
		return nil, err
	}
	return newRawConn(f)
}

// Fd returns the system file descriptor or handle referencing the open file.
// If f is closed, the descriptor becomes invalid.
// If f is garbage collected, a finalizer may close the descriptor,
// making it invalid; see [runtime.SetFinalizer] for more information on when
// a finalizer might be run.
//
// Do not close the returned descriptor; that could cause a later
// close of f to close an unrelated descriptor.
//
// Fd's behavior differs on some platforms:
//
//   - On Unix and Windows, [File.SetDeadline] methods will stop working.
//   - On Windows, the file descriptor will be disassociated from the
//     Go runtime I/O completion port if there are no concurrent I/O
//     operations on the file.
//
// For most uses prefer the f.SyscallConn method.
func (f *File) Fd() uintptr {
	return f.fd()
}

// DirFS returns a file system (an fs.FS) for the tree of files rooted at the directory dir.
//
// Note that DirFS("/prefix") only guarantees that the Open calls it makes to the
// operating system will begin with "/prefix": DirFS("/prefix").Open("file") is the
// same as os.Open("/prefix/file"). So if /prefix/file is a symbolic link pointing outside
// the /prefix tree, then using DirFS does not stop the access any more than using
// os.Open does. Additionally, the root of the fs.FS returned for a relative path,
// DirFS("prefix"), will be affected by later calls to Chdir. DirFS is therefore not
// a general substitute for a chroot-style security mechanism when the directory tree
// contains arbitrary content.
//
// Use [Root.FS] to obtain a fs.FS that prevents escapes from the tree via symbolic links.
//
// The directory dir must not be "".
//
// The result implements [io/fs.StatFS], [io/fs.ReadFileFS], [io/fs.ReadDirFS], and
// [io/fs.ReadLinkFS].
func DirFS(dir string) fs.FS {
	return dirFS(dir)
}

var _ fs.StatFS = dirFS("")
var _ fs.ReadFileFS = dirFS("")
var _ fs.ReadDirFS = dirFS("")
var _ fs.ReadLinkFS = dirFS("")

type dirFS string

func (dir dirFS) Open(name string) (fs.File, error) {
	fullname, err := dir.join(name)
	if err != nil {
		return nil, &PathError{Op: "open", Path: name, Err: err}
	}
	f, err := Open(fullname)
	if err != nil {
		// DirFS takes a string appropriate for GOOS,
		// while the name argument here is always slash separated.
		// dir.join will have mixed the two; undo that for
		// error reporting.
		err.(*PathError).Path = name
		return nil, err
	}
	return f, nil
}

// The ReadFile method calls the [ReadFile] function for the file
// with the given name in the directory. The function provides
// robust handling for small files and special file systems.
// Through this method, dirFS implements [io/fs.ReadFileFS].
func (dir dirFS) ReadFile(name string) ([]byte, error) {
	fullname, err := dir.join(name)
	if err != nil {
		return nil, &PathError{Op: "readfile", Path: name, Err: err}
	}
	b, err := ReadFile(fullname)
	if err != nil {
		if e, ok := err.(*PathError); ok {
			// See comment in dirFS.Open.
			e.Path = name
		}
		return nil, err
	}
	return b, nil
}

// ReadDir reads the named directory, returning all its directory entries sorted
// by filename. Through this method, dirFS implements [io/fs.ReadDirFS].
func (dir dirFS) ReadDir(name string) ([]DirEntry, error) {
	fullname, err := dir.join(name)
	if err != nil {
		return nil, &PathError{Op: "readdir", Path: name, Err: err}
	}
	entries, err := ReadDir(fullname)
	if err != nil {
		if e, ok := err.(*PathError); ok {
			// See comment in dirFS.Open.
			e.Path = name
		}
		return nil, err
	}
	return entries, nil
}

func (dir dirFS) Stat(name string) (fs.FileInfo, error) {
	fullname, err := dir.join(name)
	if err != nil {
		return nil, &PathError{Op: "stat", Path: name, Err: err}
	}
	f, err := Stat(fullname)
	if err != nil {
		// See comment in dirFS.Open.
		err.(*PathError).Path = name
		return nil, err
	}
	return f, nil
}

func (dir dirFS) Lstat(name string) (fs.FileInfo, error) {
	fullname, err := dir.join(name)
	if err != nil {
		return nil, &PathError{Op: "lstat", Path: name, Err: err}
	}
	f, err := Lstat(fullname)
	if err != nil {
		// See comment in dirFS.Open.
		err.(*PathError).Path = name
		return nil, err
	}
	return f, nil
}

func (dir dirFS) ReadLink(name string) (string, error) {
	fullname, err := dir.join(name)
	if err != nil {
		return "", &PathError{Op: "readlink", Path: name, Err: err}
	}
	return Readlink(fullname)
}

// join returns the path for name in dir.
func (dir dirFS) join(name string) (string, error) {
	if dir == "" {
		return "", errors.New("os: DirFS with empty root")
	}
	name, err := filepathlite.Localize(name)
	if err != nil {
		return "", ErrInvalid
	}
	if IsPathSeparator(dir[len(dir)-1]) {
		return string(dir) + name, nil
	}
	return string(dir) + string(PathSeparator) + name, nil
}

// ReadFile reads the named file and returns the contents.
// A successful call returns err == nil, not err == EOF.
// Because ReadFile reads the whole file, it does not treat an EOF from Read
// as an error to be reported.
func ReadFile(name string) ([]byte, error) {
	f, err := Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return readFileContents(statOrZero(f), f.Read)
}

func statOrZero(f *File) int64 {
	if fi, err := f.Stat(); err == nil {
		return fi.Size()
	}
	return 0
}

// readFileContents reads the contents of a file using the provided read function
// (*os.File.Read, except in tests) one or more times, until an error is seen.
//
// The provided size is the stat size of the file, which might be 0 for a
// /proc-like file that doesn't report a size.
func readFileContents(statSize int64, read func([]byte) (int, error)) ([]byte, error) {
	zeroSize := statSize == 0

	// Figure out how big to make the initial slice. For files with known size
	// that fit in memory, use that size + 1. Otherwise, use a small buffer and
	// we'll grow.
	var size int
	if int64(int(statSize)) == statSize {
		size = int(statSize)
	}
	size++ // one byte for final read at EOF

	const minBuf = 512
	// If a file claims a small size, read at least 512 bytes. In particular,
	// files in Linux's /proc claim size 0 but then do not work right if read in
	// small pieces, so an initial read of 1 byte would not work correctly.
	if size < minBuf {
		size = minBuf
	}

	data := make([]byte, 0, size)
	for {
		n, err := read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}

		// If we're either out of capacity or if the file was a /proc-like zero
		// sized file, grow the buffer. Per Issue 72080, we always want to issue
		// Read calls on zero-length files with a non-tiny buffer size.
		capRemain := cap(data) - len(data)
		if capRemain == 0 || (zeroSize && capRemain < minBuf) {
			data = slices.Grow(data, minBuf)
		}
	}
}

// WriteFile writes data to the named file, creating it if necessary.
// If the file does not exist, WriteFile creates it with permissions perm (before umask);
// otherwise WriteFile truncates it before writing, without changing permissions.
// Since WriteFile requires multiple system calls to complete, a failure mid-operation
// can leave the file in a partially written state.
func WriteFile(name string, data []byte, perm FileMode) error {
	f, err := OpenFile(name, O_WRONLY|O_CREATE|O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}

```

// === FILE: references!/go/src/os/file_mutex_plan9.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

// File locking support for Plan 9. This uses fdMutex from the
// internal/poll package.

// incref adds a reference to the file. It returns an error if the file
// is already closed. This method is on File so that we can incorporate
// a nil test.
func (f *File) incref(op string) (err error) {
	if f == nil {
		return ErrInvalid
	}
	if !f.fdmu.Incref() {
		err = ErrClosed
		if op != "" {
			err = &PathError{Op: op, Path: f.name, Err: err}
		}
	}
	return err
}

// decref removes a reference to the file. If this is the last
// remaining reference, and the file has been marked to be closed,
// then actually close it.
func (file *file) decref() error {
	if file.fdmu.Decref() {
		return file.destroy()
	}
	return nil
}

// readLock adds a reference to the file and locks it for reading.
// It returns an error if the file is already closed.
func (file *file) readLock() error {
	if !file.fdmu.ReadLock() {
		return ErrClosed
	}
	return nil
}

// readUnlock removes a reference from the file and unlocks it for reading.
// It also closes the file if it marked as closed and there is no remaining
// reference.
func (file *file) readUnlock() {
	if file.fdmu.ReadUnlock() {
		file.destroy()
	}
}

// writeLock adds a reference to the file and locks it for writing.
// It returns an error if the file is already closed.
func (file *file) writeLock() error {
	if !file.fdmu.WriteLock() {
		return ErrClosed
	}
	return nil
}

// writeUnlock removes a reference from the file and unlocks it for writing.
// It also closes the file if it is marked as closed and there is no remaining
// reference.
func (file *file) writeUnlock() {
	if file.fdmu.WriteUnlock() {
		file.destroy()
	}
}

```

// === FILE: references!/go/src/os/file_open_unix.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || (js && wasm)

package os

import (
	"internal/poll"
	"syscall"
)

func open(path string, flag int, perm uint32) (int, poll.SysFile, error) {
	fd, err := syscall.Open(path, flag, perm)
	return fd, poll.SysFile{}, err
}

```

// === FILE: references!/go/src/os/file_open_wasip1.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build wasip1

package os

import (
	"internal/poll"
	"syscall"
)

func open(filePath string, flag int, perm uint32) (int, poll.SysFile, error) {
	if filePath == "" {
		return -1, poll.SysFile{}, syscall.EINVAL
	}
	absPath := filePath
	// os.(*File).Chdir is emulated by setting the working directory to the
	// absolute path that this file was opened at, which is why we have to
	// resolve and capture it here.
	if filePath[0] != '/' {
		wd, err := syscall.Getwd()
		if err != nil {
			return -1, poll.SysFile{}, err
		}
		absPath = joinPath(wd, filePath)
	}
	fd, err := syscall.Open(absPath, flag, perm)
	return fd, poll.SysFile{Path: absPath}, err
}

```

// === FILE: references!/go/src/os/file_plan9.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/bytealg"
	"internal/poll"
	"internal/stringslite"
	"internal/testlog"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// fixLongPath is a noop on non-Windows platforms.
func fixLongPath(path string) string {
	return path
}

// file is the real representation of *File.
// The extra level of indirection ensures that no clients of os
// can overwrite this data, which could cause the finalizer
// to close the wrong file descriptor.
type file struct {
	fdmu       poll.FDMutex
	sysfd      int
	name       string
	dirinfo    atomic.Pointer[dirInfo] // nil unless directory being read
	appendMode bool                    // whether file is opened for appending
}

// fd is the Plan 9 implementation of Fd.
func (f *File) fd() uintptr {
	if f == nil {
		return ^(uintptr(0))
	}
	return uintptr(f.sysfd)
}

// newFileFromNewFile is called by [NewFile].
func newFileFromNewFile(fd uintptr, name string) *File {
	fdi := int(fd)
	if fdi < 0 {
		return nil
	}
	f := &File{&file{sysfd: fdi, name: name}}
	runtime.SetFinalizer(f.file, (*file).close)
	return f
}

// Auxiliary information if the File describes a directory
type dirInfo struct {
	mu   sync.Mutex
	buf  [syscall.STATMAX]byte // buffer for directory I/O
	nbuf int                   // length of buf; return value from Read
	bufp int                   // location of next record in buf.
}

func epipecheck(file *File, e error) {
}

// DevNull is the name of the operating system's “null device.”
// On Unix-like systems, it is "/dev/null"; on Windows, "NUL".
const DevNull = "/dev/null"

// syscallMode returns the syscall-specific mode bits from Go's portable mode bits.
func syscallMode(i FileMode) (o uint32) {
	o |= uint32(i.Perm())
	if i&ModeAppend != 0 {
		o |= syscall.DMAPPEND
	}
	if i&ModeExclusive != 0 {
		o |= syscall.DMEXCL
	}
	if i&ModeTemporary != 0 {
		o |= syscall.DMTMP
	}
	return
}

// openFileNolog is the Plan 9 implementation of OpenFile.
func openFileNolog(name string, flag int, perm FileMode) (*File, error) {
	var (
		fd     int
		e      error
		create bool
		excl   bool
		trunc  bool
		append bool
	)

	if flag&O_CREATE == O_CREATE {
		flag = flag & ^O_CREATE
		create = true
	}
	if flag&O_EXCL == O_EXCL {
		excl = true
	}
	if flag&O_TRUNC == O_TRUNC {
		trunc = true
	}
	// O_APPEND is emulated on Plan 9
	if flag&O_APPEND == O_APPEND {
		flag = flag &^ O_APPEND
		append = true
	}

	if (create && trunc) || excl {
		fd, e = syscall.Create(name, flag, syscallMode(perm))
	} else {
		fd, e = syscall.Open(name, flag)
		if IsNotExist(e) && create {
			fd, e = syscall.Create(name, flag, syscallMode(perm))
			if e != nil {
				return nil, &PathError{Op: "create", Path: name, Err: e}
			}
		}
	}

	if e != nil {
		return nil, &PathError{Op: "open", Path: name, Err: e}
	}

	if append {
		if _, e = syscall.Seek(fd, 0, io.SeekEnd); e != nil {
			return nil, &PathError{Op: "seek", Path: name, Err: e}
		}
	}

	return NewFile(uintptr(fd), name), nil
}

func openDirNolog(name string) (*File, error) {
	f, e := openFileNolog(name, O_RDONLY, 0)
	if e != nil {
		return nil, e
	}
	d, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, e
	}
	if !d.IsDir() {
		f.Close()
		return nil, &PathError{Op: "open", Path: name, Err: syscall.ENOTDIR}
	}
	return f, nil
}

// Close closes the File, rendering it unusable for I/O.
// On files that support SetDeadline, any pending I/O operations will
// be canceled and return immediately with an ErrClosed error.
// Close will return an error if it has already been called.
func (f *File) Close() error {
	if f == nil {
		return ErrInvalid
	}
	return f.file.close()
}

func (file *file) close() error {
	if !file.fdmu.IncrefAndClose() {
		return &PathError{Op: "close", Path: file.name, Err: ErrClosed}
	}

	// At this point we should cancel any pending I/O.
	// How do we do that on Plan 9?

	err := file.decref()

	// no need for a finalizer anymore
	runtime.SetFinalizer(file, nil)
	return err
}

// destroy actually closes the descriptor. This is called when
// there are no remaining references, by the decref, readUnlock,
// and writeUnlock methods.
func (file *file) destroy() error {
	var err error
	if e := syscall.Close(file.sysfd); e != nil {
		err = &PathError{Op: "close", Path: file.name, Err: e}
	}
	return err
}

// Stat returns the FileInfo structure describing file.
// If there is an error, it will be of type [*PathError].
func (f *File) Stat() (FileInfo, error) {
	if f == nil {
		return nil, ErrInvalid
	}
	d, err := dirstat(f)
	if err != nil {
		return nil, err
	}
	return fileInfoFromStat(d), nil
}

// Truncate changes the size of the file.
// It does not change the I/O offset.
// If there is an error, it will be of type [*PathError].
func (f *File) Truncate(size int64) error {
	if f == nil {
		return ErrInvalid
	}

	var d syscall.Dir
	d.Null()
	d.Length = size

	var buf [syscall.STATFIXLEN]byte
	n, err := d.Marshal(buf[:])
	if err != nil {
		return &PathError{Op: "truncate", Path: f.name, Err: err}
	}

	if err := f.incref("truncate"); err != nil {
		return err
	}
	defer f.decref()

	if err = syscall.Fwstat(f.sysfd, buf[:n]); err != nil {
		return &PathError{Op: "truncate", Path: f.name, Err: err}
	}
	return nil
}

const chmodMask = uint32(syscall.DMAPPEND | syscall.DMEXCL | syscall.DMTMP | ModePerm)

func (f *File) chmod(mode FileMode) error {
	if f == nil {
		return ErrInvalid
	}
	var d syscall.Dir

	odir, e := dirstat(f)
	if e != nil {
		return &PathError{Op: "chmod", Path: f.name, Err: e}
	}
	d.Null()
	d.Mode = odir.Mode&^chmodMask | syscallMode(mode)&chmodMask

	var buf [syscall.STATFIXLEN]byte
	n, err := d.Marshal(buf[:])
	if err != nil {
		return &PathError{Op: "chmod", Path: f.name, Err: err}
	}

	if err := f.incref("chmod"); err != nil {
		return err
	}
	defer f.decref()

	if err = syscall.Fwstat(f.sysfd, buf[:n]); err != nil {
		return &PathError{Op: "chmod", Path: f.name, Err: err}
	}
	return nil
}

// Sync commits the current contents of the file to stable storage.
// Typically, this means flushing the file system's in-memory copy
// of recently written data to disk.
func (f *File) Sync() error {
	if f == nil {
		return ErrInvalid
	}
	var d syscall.Dir
	d.Null()

	var buf [syscall.STATFIXLEN]byte
	n, err := d.Marshal(buf[:])
	if err != nil {
		return &PathError{Op: "sync", Path: f.name, Err: err}
	}

	if err := f.incref("sync"); err != nil {
		return err
	}
	defer f.decref()

	if err = syscall.Fwstat(f.sysfd, buf[:n]); err != nil {
		return &PathError{Op: "sync", Path: f.name, Err: err}
	}
	return nil
}

// read reads up to len(b) bytes from the File.
// It returns the number of bytes read and an error, if any.
func (f *File) read(b []byte) (n int, err error) {
	if err := f.readLock(); err != nil {
		return 0, err
	}
	defer f.readUnlock()
	n, e := fixCount(syscall.Read(f.sysfd, b))
	if n == 0 && len(b) > 0 && e == nil {
		return 0, io.EOF
	}
	return n, e
}

// pread reads len(b) bytes from the File starting at byte offset off.
// It returns the number of bytes read and the error, if any.
// EOF is signaled by a zero count with err set to nil.
func (f *File) pread(b []byte, off int64) (n int, err error) {
	if err := f.readLock(); err != nil {
		return 0, err
	}
	defer f.readUnlock()
	n, e := fixCount(syscall.Pread(f.sysfd, b, off))
	if n == 0 && len(b) > 0 && e == nil {
		return 0, io.EOF
	}
	return n, e
}

// write writes len(b) bytes to the File.
// It returns the number of bytes written and an error, if any.
// Since Plan 9 preserves message boundaries, never allow
// a zero-byte write.
func (f *File) write(b []byte) (n int, err error) {
	if err := f.writeLock(); err != nil {
		return 0, err
	}
	defer f.writeUnlock()
	if len(b) == 0 {
		return 0, nil
	}
	return fixCount(syscall.Write(f.sysfd, b))
}

// pwrite writes len(b) bytes to the File starting at byte offset off.
// It returns the number of bytes written and an error, if any.
// Since Plan 9 preserves message boundaries, never allow
// a zero-byte write.
func (f *File) pwrite(b []byte, off int64) (n int, err error) {
	if err := f.writeLock(); err != nil {
		return 0, err
	}
	defer f.writeUnlock()
	if len(b) == 0 {
		return 0, nil
	}
	return fixCount(syscall.Pwrite(f.sysfd, b, off))
}

// seek sets the offset for the next Read or Write on file to offset, interpreted
// according to whence: 0 means relative to the origin of the file, 1 means
// relative to the current offset, and 2 means relative to the end.
// It returns the new offset and an error, if any.
func (f *File) seek(offset int64, whence int) (ret int64, err error) {
	if err := f.incref(""); err != nil {
		return 0, err
	}
	defer f.decref()
	// Free cached dirinfo, so we allocate a new one if we
	// access this file as a directory again. See #35767 and #37161.
	f.dirinfo.Store(nil)
	return syscall.Seek(f.sysfd, offset, whence)
}

// Truncate changes the size of the named file.
// If the file is a symbolic link, it changes the size of the link's target.
// If there is an error, it will be of type [*PathError].
func Truncate(name string, size int64) error {
	var d syscall.Dir

	d.Null()
	d.Length = size

	var buf [syscall.STATFIXLEN]byte
	n, err := d.Marshal(buf[:])
	if err != nil {
		return &PathError{Op: "truncate", Path: name, Err: err}
	}
	if err = syscall.Wstat(name, buf[:n]); err != nil {
		return &PathError{Op: "truncate", Path: name, Err: err}
	}
	return nil
}

// Remove removes the named file or directory.
// If there is an error, it will be of type [*PathError].
func Remove(name string) error {
	if e := syscall.Remove(name); e != nil {
		return &PathError{Op: "remove", Path: name, Err: e}
	}
	return nil
}

func rename(oldname, newname string) error {
	dirname := oldname[:bytealg.LastIndexByteString(oldname, '/')+1]
	if stringslite.HasPrefix(newname, dirname) {
		newname = newname[len(dirname):]
	} else {
		return &LinkError{"rename", oldname, newname, ErrInvalid}
	}

	// If newname still contains slashes after removing the oldname
	// prefix, the rename is cross-directory and must be rejected.
	if bytealg.LastIndexByteString(newname, '/') >= 0 {
		return &LinkError{"rename", oldname, newname, ErrInvalid}
	}

	var d syscall.Dir

	d.Null()
	d.Name = newname

	buf := make([]byte, syscall.STATFIXLEN+len(d.Name))
	n, err := d.Marshal(buf[:])
	if err != nil {
		return &LinkError{"rename", oldname, newname, err}
	}

	// If newname already exists and is not a directory, rename replaces it.
	f, err := Stat(dirname + newname)
	if err == nil && !f.IsDir() {
		Remove(dirname + newname)
	}

	if err = syscall.Wstat(oldname, buf[:n]); err != nil {
		return &LinkError{"rename", oldname, newname, err}
	}
	return nil
}

// See docs in file.go:Chmod.
func chmod(name string, mode FileMode) error {
	var d syscall.Dir

	odir, e := dirstat(name)
	if e != nil {
		return &PathError{Op: "chmod", Path: name, Err: e}
	}
	d.Null()
	d.Mode = odir.Mode&^chmodMask | syscallMode(mode)&chmodMask

	var buf [syscall.STATFIXLEN]byte
	n, err := d.Marshal(buf[:])
	if err != nil {
		return &PathError{Op: "chmod", Path: name, Err: err}
	}
	if err = syscall.Wstat(name, buf[:n]); err != nil {
		return &PathError{Op: "chmod", Path: name, Err: err}
	}
	return nil
}

// Chtimes changes the access and modification times of the named
// file, similar to the Unix utime() or utimes() functions.
// A zero time.Time value will leave the corresponding file time unchanged.
//
// The underlying filesystem may truncate or round the values to a
// less precise time unit.
// If there is an error, it will be of type [*PathError].
func Chtimes(name string, atime time.Time, mtime time.Time) error {
	var d syscall.Dir

	d.Null()
	d.Atime = uint32(atime.Unix())
	d.Mtime = uint32(mtime.Unix())
	if atime.IsZero() {
		d.Atime = 0xFFFFFFFF
	}
	if mtime.IsZero() {
		d.Mtime = 0xFFFFFFFF
	}

	var buf [syscall.STATFIXLEN]byte
	n, err := d.Marshal(buf[:])
	if err != nil {
		return &PathError{Op: "chtimes", Path: name, Err: err}
	}
	if err = syscall.Wstat(name, buf[:n]); err != nil {
		return &PathError{Op: "chtimes", Path: name, Err: err}
	}
	return nil
}

// Pipe returns a connected pair of Files; reads from r return bytes
// written to w. It returns the files and an error, if any.
func Pipe() (r *File, w *File, err error) {
	var p [2]int

	if e := syscall.Pipe(p[0:]); e != nil {
		return nil, nil, NewSyscallError("pipe", e)
	}

	return NewFile(uintptr(p[0]), "|0"), NewFile(uintptr(p[1]), "|1"), nil
}

// not supported on Plan 9

// Link creates newname as a hard link to the oldname file.
// If there is an error, it will be of type *LinkError.
func Link(oldname, newname string) error {
	return &LinkError{"link", oldname, newname, syscall.EPLAN9}
}

// Symlink creates newname as a symbolic link to oldname.
// On Windows, a symlink to a non-existent oldname creates a file symlink;
// if oldname is later created as a directory the symlink will not work.
// If there is an error, it will be of type *LinkError.
func Symlink(oldname, newname string) error {
	return &LinkError{"symlink", oldname, newname, syscall.EPLAN9}
}

func readlink(name string) (string, error) {
	return "", &PathError{Op: "readlink", Path: name, Err: syscall.EPLAN9}
}

// Chown changes the numeric uid and gid of the named file.
// If the file is a symbolic link, it changes the uid and gid of the link's target.
// A uid or gid of -1 means to not change that value.
// If there is an error, it will be of type [*PathError].
//
// On Windows or Plan 9, Chown always returns the [syscall.EWINDOWS] or
// [syscall.EPLAN9] error, wrapped in [*PathError].
func Chown(name string, uid, gid int) error {
	return &PathError{Op: "chown", Path: name, Err: syscall.EPLAN9}
}

// Lchown changes the numeric uid and gid of the named file.
// If the file is a symbolic link, it changes the uid and gid of the link itself.
// If there is an error, it will be of type [*PathError].
func Lchown(name string, uid, gid int) error {
	return &PathError{Op: "lchown", Path: name, Err: syscall.EPLAN9}
}

// Chown changes the numeric uid and gid of the named file.
// If there is an error, it will be of type [*PathError].
func (f *File) Chown(uid, gid int) error {
	if f == nil {
		return ErrInvalid
	}
	return &PathError{Op: "chown", Path: f.name, Err: syscall.EPLAN9}
}

func tempDir() string {
	dir := Getenv("TMPDIR")
	if dir == "" {
		dir = "/tmp"
	}
	return dir
}

// Chdir changes the current working directory to the file,
// which must be a directory.
// If there is an error, it will be of type [*PathError].
func (f *File) Chdir() error {
	if err := f.incref("chdir"); err != nil {
		return err
	}
	defer f.decref()
	if e := syscall.Fchdir(f.sysfd); e != nil {
		return &PathError{Op: "chdir", Path: f.name, Err: e}
	}
	if log := testlog.Logger(); log != nil {
		wd, err := Getwd()
		if err == nil {
			log.Chdir(wd)
		}
	}
	return nil
}

// setDeadline sets the read and write deadline.
func (f *File) setDeadline(time.Time) error {
	if err := f.checkValid("SetDeadline"); err != nil {
		return err
	}
	return poll.ErrNoDeadline
}

// setReadDeadline sets the read deadline.
func (f *File) setReadDeadline(time.Time) error {
	if err := f.checkValid("SetReadDeadline"); err != nil {
		return err
	}
	return poll.ErrNoDeadline
}

// setWriteDeadline sets the write deadline.
func (f *File) setWriteDeadline(time.Time) error {
	if err := f.checkValid("SetWriteDeadline"); err != nil {
		return err
	}
	return poll.ErrNoDeadline
}

// checkValid checks whether f is valid for use, but does not prepare
// to actually use it. If f is not ready checkValid returns an appropriate
// error, perhaps incorporating the operation name op.
func (f *File) checkValid(op string) error {
	if f == nil {
		return ErrInvalid
	}
	if err := f.incref(op); err != nil {
		return err
	}
	return f.decref()
}

type rawConn struct{}

func (c *rawConn) Control(f func(uintptr)) error {
	return syscall.EPLAN9
}

func (c *rawConn) Read(f func(uintptr) bool) error {
	return syscall.EPLAN9
}

func (c *rawConn) Write(f func(uintptr) bool) error {
	return syscall.EPLAN9
}

func newRawConn(file *File) (*rawConn, error) {
	return nil, syscall.EPLAN9
}

func ignoringEINTR(fn func() error) error {
	return fn()
}

func ignoringEINTR2[T any](fn func() (T, error)) (T, error) {
	return fn()
}

```

// === FILE: references!/go/src/os/file_posix.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || (js && wasm) || wasip1 || windows

package os

import (
	"internal/testlog"
	"runtime"
	"syscall"
	"time"
)

// Close closes the [File], rendering it unusable for I/O.
// On files that support [File.SetDeadline], any pending I/O operations will
// be canceled and return immediately with an [ErrClosed] error.
// Close will return an error if it has already been called.
func (f *File) Close() error {
	if f == nil {
		return ErrInvalid
	}
	return f.file.close()
}

// read reads up to len(b) bytes from the File.
// It returns the number of bytes read and an error, if any.
func (f *File) read(b []byte) (n int, err error) {
	n, err = f.pfd.Read(b)
	runtime.KeepAlive(f)
	return n, err
}

// pread reads len(b) bytes from the File starting at byte offset off.
// It returns the number of bytes read and the error, if any.
// EOF is signaled by a zero count with err set to nil.
func (f *File) pread(b []byte, off int64) (n int, err error) {
	n, err = f.pfd.Pread(b, off)
	runtime.KeepAlive(f)
	return n, err
}

// write writes len(b) bytes to the File.
// It returns the number of bytes written and an error, if any.
func (f *File) write(b []byte) (n int, err error) {
	n, err = f.pfd.Write(b)
	runtime.KeepAlive(f)
	return n, err
}

// pwrite writes len(b) bytes to the File starting at byte offset off.
// It returns the number of bytes written and an error, if any.
func (f *File) pwrite(b []byte, off int64) (n int, err error) {
	n, err = f.pfd.Pwrite(b, off)
	runtime.KeepAlive(f)
	return n, err
}

// syscallMode returns the syscall-specific mode bits from Go's portable mode bits.
func syscallMode(i FileMode) (o uint32) {
	o |= uint32(i.Perm())
	if i&ModeSetuid != 0 {
		o |= syscall.S_ISUID
	}
	if i&ModeSetgid != 0 {
		o |= syscall.S_ISGID
	}
	if i&ModeSticky != 0 {
		o |= syscall.S_ISVTX
	}
	// No mapping for Go's ModeTemporary (plan9 only).
	return
}

// See docs in file.go:Chmod.
func chmod(name string, mode FileMode) error {
	longName := fixLongPath(name)
	e := ignoringEINTR(func() error {
		return syscall.Chmod(longName, syscallMode(mode))
	})
	if e != nil {
		return &PathError{Op: "chmod", Path: name, Err: e}
	}
	return nil
}

// See docs in file.go:(*File).Chmod.
func (f *File) chmod(mode FileMode) error {
	if err := f.checkValid("chmod"); err != nil {
		return err
	}
	if e := f.pfd.Fchmod(syscallMode(mode)); e != nil {
		return f.wrapErr("chmod", e)
	}
	return nil
}

// Chown changes the numeric uid and gid of the named file.
// If the file is a symbolic link, it changes the uid and gid of the link's target.
// A uid or gid of -1 means to not change that value.
// If there is an error, it will be of type [*PathError].
//
// On Windows or Plan 9, Chown always returns the [syscall.EWINDOWS] or
// [syscall.EPLAN9] error, wrapped in [*PathError].
func Chown(name string, uid, gid int) error {
	e := ignoringEINTR(func() error {
		return syscall.Chown(name, uid, gid)
	})
	if e != nil {
		return &PathError{Op: "chown", Path: name, Err: e}
	}
	return nil
}

// Lchown changes the numeric uid and gid of the named file.
// If the file is a symbolic link, it changes the uid and gid of the link itself.
// If there is an error, it will be of type [*PathError].
//
// On Windows, it always returns the [syscall.EWINDOWS] error, wrapped
// in [*PathError].
func Lchown(name string, uid, gid int) error {
	e := ignoringEINTR(func() error {
		return syscall.Lchown(name, uid, gid)
	})
	if e != nil {
		return &PathError{Op: "lchown", Path: name, Err: e}
	}
	return nil
}

// Chown changes the numeric uid and gid of the named file.
// If there is an error, it will be of type [*PathError].
//
// On Windows, it always returns the [syscall.EWINDOWS] error, wrapped
// in [*PathError].
func (f *File) Chown(uid, gid int) error {
	if err := f.checkValid("chown"); err != nil {
		return err
	}
	if e := f.pfd.Fchown(uid, gid); e != nil {
		return f.wrapErr("chown", e)
	}
	return nil
}

// Truncate changes the size of the file.
// It does not change the I/O offset.
// If there is an error, it will be of type [*PathError].
func (f *File) Truncate(size int64) error {
	if err := f.checkValid("truncate"); err != nil {
		return err
	}
	if e := f.pfd.Ftruncate(size); e != nil {
		return f.wrapErr("truncate", e)
	}
	return nil
}

// Sync commits the current contents of the file to stable storage.
// Typically, this means flushing the file system's in-memory copy
// of recently written data to disk.
func (f *File) Sync() error {
	if err := f.checkValid("sync"); err != nil {
		return err
	}
	if e := f.pfd.Fsync(); e != nil {
		return f.wrapErr("sync", e)
	}
	return nil
}

// Chtimes changes the access and modification times of the named
// file, similar to the Unix utime() or utimes() functions.
// A zero [time.Time] value will leave the corresponding file time unchanged.
//
// The underlying filesystem may truncate or round the values to a
// less precise time unit.
// If there is an error, it will be of type [*PathError].
func Chtimes(name string, atime time.Time, mtime time.Time) error {
	utimes := chtimesUtimes(atime, mtime)
	if e := syscall.UtimesNano(fixLongPath(name), utimes[0:]); e != nil {
		return &PathError{Op: "chtimes", Path: name, Err: e}
	}
	return nil
}

func chtimesUtimes(atime, mtime time.Time) [2]syscall.Timespec {
	var utimes [2]syscall.Timespec
	set := func(i int, t time.Time) {
		if t.IsZero() {
			utimes[i] = syscall.Timespec{Sec: _UTIME_OMIT, Nsec: _UTIME_OMIT}
		} else {
			utimes[i] = syscall.NsecToTimespec(t.UnixNano())
		}
	}
	set(0, atime)
	set(1, mtime)
	return utimes
}

// Chdir changes the current working directory to the file,
// which must be a directory.
// If there is an error, it will be of type [*PathError].
func (f *File) Chdir() error {
	if err := f.checkValid("chdir"); err != nil {
		return err
	}
	if e := f.pfd.Fchdir(); e != nil {
		return f.wrapErr("chdir", e)
	}
	if log := testlog.Logger(); log != nil {
		wd, err := Getwd()
		if err == nil {
			log.Chdir(wd)
		}
	}
	return nil
}

// setDeadline sets the read and write deadline.
func (f *File) setDeadline(t time.Time) error {
	if err := f.checkValid("SetDeadline"); err != nil {
		return err
	}
	return f.pfd.SetDeadline(t)
}

// setReadDeadline sets the read deadline.
func (f *File) setReadDeadline(t time.Time) error {
	if err := f.checkValid("SetReadDeadline"); err != nil {
		return err
	}
	return f.pfd.SetReadDeadline(t)
}

// setWriteDeadline sets the write deadline.
func (f *File) setWriteDeadline(t time.Time) error {
	if err := f.checkValid("SetWriteDeadline"); err != nil {
		return err
	}
	return f.pfd.SetWriteDeadline(t)
}

// checkValid checks whether f is valid for use.
// If not, it returns an appropriate error, perhaps incorporating the operation name op.
func (f *File) checkValid(op string) error {
	if f == nil {
		return ErrInvalid
	}
	return nil
}

// ignoringEINTR makes a function call and repeats it if it returns an
// EINTR error. This appears to be required even though we install all
// signal handlers with SA_RESTART: see #22838, #38033, #38836, #40846.
// Also #20400 and #36644 are issues in which a signal handler is
// installed without setting SA_RESTART. None of these are the common case,
// but there are enough of them that it seems that we can't avoid
// an EINTR loop.
func ignoringEINTR(fn func() error) error {
	for {
		err := fn()
		if err != syscall.EINTR {
			return err
		}
	}
}

// ignoringEINTR2 is ignoringEINTR, but returning an additional value.
func ignoringEINTR2[T any](fn func() (T, error)) (T, error) {
	for {
		v, err := fn()
		if err != syscall.EINTR {
			return v, err
		}
	}
}

```

// === FILE: references!/go/src/os/file_unix.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || (js && wasm) || wasip1

package os

import (
	"internal/poll"
	"internal/syscall/unix"
	"io/fs"
	"runtime"
	"sync/atomic"
	"syscall"
	_ "unsafe" // for go:linkname
)

const _UTIME_OMIT = unix.UTIME_OMIT

// fixLongPath is a noop on non-Windows platforms.
func fixLongPath(path string) string {
	return path
}

func rename(oldname, newname string) error {
	fi, err := Lstat(newname)
	if err == nil && fi.IsDir() {
		// There are two independent errors this function can return:
		// one for a bad oldname, and one for a bad newname.
		// At this point we've determined the newname is bad.
		// But just in case oldname is also bad, prioritize returning
		// the oldname error because that's what we did historically.
		// However, if the old name and new name are not the same, yet
		// they refer to the same file, it implies a case-only
		// rename on a case-insensitive filesystem, which is ok.
		if ofi, err := Lstat(oldname); err != nil {
			if pe, ok := err.(*PathError); ok {
				err = pe.Err
			}
			return &LinkError{"rename", oldname, newname, err}
		} else if newname == oldname || !SameFile(fi, ofi) {
			return &LinkError{"rename", oldname, newname, syscall.EEXIST}
		}
	}
	err = ignoringEINTR(func() error {
		return syscall.Rename(oldname, newname)
	})
	if err != nil {
		return &LinkError{"rename", oldname, newname, err}
	}
	return nil
}

// file is the real representation of *File.
// The extra level of indirection ensures that no clients of os
// can overwrite this data, which could cause the finalizer
// to close the wrong file descriptor.
type file struct {
	pfd         poll.FD
	name        string
	dirinfo     atomic.Pointer[dirInfo] // nil unless directory being read
	nonblock    bool                    // whether we set nonblocking mode
	stdoutOrErr bool                    // whether this is stdout or stderr
	appendMode  bool                    // whether file is opened for appending
	inRoot      bool                    // whether file is opened in a Root
}

// fd is the Unix implementation of Fd.
func (f *File) fd() uintptr {
	if f == nil {
		return ^(uintptr(0))
	}

	// If we put the file descriptor into nonblocking mode,
	// then set it to blocking mode before we return it,
	// because historically we have always returned a descriptor
	// opened in blocking mode. The File will continue to work,
	// but any blocking operation will tie up a thread.
	if f.nonblock {
		f.pfd.SetBlocking()
	}

	return uintptr(f.pfd.Sysfd)
}

// newFileFromNewFile is called by [NewFile].
func newFileFromNewFile(fd uintptr, name string) *File {
	fdi := int(fd)
	if fdi < 0 {
		return nil
	}

	flags, err := unix.Fcntl(fdi, syscall.F_GETFL, 0)
	if err != nil {
		flags = 0
	}
	f := newFile(fdi, name, kindNewFile, unix.HasNonblockFlag(flags))
	f.appendMode = flags&syscall.O_APPEND != 0
	return f
}

// net_newUnixFile is a hidden entry point called by net.conn.File.
// This is used so that a nonblocking network connection will become
// blocking if code calls the Fd method. We don't want that for direct
// calls to NewFile: passing a nonblocking descriptor to NewFile should
// remain nonblocking if you get it back using Fd. But for net.conn.File
// the call to NewFile is hidden from the user. Historically in that case
// the Fd method has returned a blocking descriptor, and we want to
// retain that behavior because existing code expects it and depends on it.
//
//go:linkname net_newUnixFile net.newUnixFile
func net_newUnixFile(fd int, name string) *File {
	if fd < 0 {
		panic("invalid FD")
	}

	return newFile(fd, name, kindSock, true)
}

// newFileKind describes the kind of file to newFile.
type newFileKind int

const (
	// kindNewFile means that the descriptor was passed to us via NewFile.
	kindNewFile newFileKind = iota
	// kindOpenFile means that the descriptor was opened using
	// Open, Create, or OpenFile.
	kindOpenFile
	// kindPipe means that the descriptor was opened using Pipe.
	kindPipe
	// kindSock means that the descriptor is a network file descriptor
	// that was created from net package and was opened using net_newUnixFile.
	kindSock
	// kindNoPoll means that we should not put the descriptor into
	// non-blocking mode, because we know it is not a pipe or FIFO.
	// Used by openDirAt and openDirNolog for directories.
	kindNoPoll
)

// newFile is like NewFile, but if called from OpenFile or Pipe
// (as passed in the kind parameter) it tries to add the file to
// the runtime poller.
func newFile(fd int, name string, kind newFileKind, nonBlocking bool) *File {
	f := &File{&file{
		pfd: poll.FD{
			Sysfd:         fd,
			IsStream:      true,
			ZeroReadIsEOF: true,
		},
		name:        name,
		stdoutOrErr: fd == 1 || fd == 2,
	}}

	pollable := kind == kindOpenFile || kind == kindPipe || kind == kindSock || nonBlocking

	// Things like regular files and FIFOs in kqueue on *BSD/Darwin
	// may not work properly (or accurately according to its manual).
	// As a result, we should avoid adding those to the kqueue-based
	// netpoller. Check out #19093, #24164, and #66239 for more contexts.
	//
	// If the fd was passed to us via any path other than OpenFile,
	// we assume those callers know what they were doing, so we won't
	// perform this check and allow it to be added to the kqueue.
	if kind == kindOpenFile {
		switch runtime.GOOS {
		case "darwin", "ios", "dragonfly", "freebsd", "netbsd", "openbsd":
			var st syscall.Stat_t
			err := ignoringEINTR(func() error {
				return syscall.Fstat(fd, &st)
			})
			typ := st.Mode & syscall.S_IFMT
			// Don't try to use kqueue with regular files on *BSDs.
			// On FreeBSD a regular file is always
			// reported as ready for writing.
			// On Dragonfly, NetBSD and OpenBSD the fd is signaled
			// only once as ready (both read and write).
			// Issue 19093.
			// Also don't add directories to the netpoller.
			if err == nil && (typ == syscall.S_IFREG || typ == syscall.S_IFDIR) {
				pollable = false
			}

			// In addition to the behavior described above for regular files,
			// on Darwin, kqueue does not work properly with fifos:
			// closing the last writer does not cause a kqueue event
			// for any readers. See issue #24164.
			if (runtime.GOOS == "darwin" || runtime.GOOS == "ios") && typ == syscall.S_IFIFO {
				pollable = false
			}
		}
	}

	clearNonBlock := false
	if pollable {
		// The descriptor is already in non-blocking mode.
		// We only set f.nonblock if we put the file into
		// non-blocking mode.
		if nonBlocking {
			// See the comments on net_newUnixFile.
			if kind == kindSock {
				f.nonblock = true // tell Fd to return blocking descriptor
			}
		} else if err := syscall.SetNonblock(fd, true); err == nil {
			f.nonblock = true
			clearNonBlock = true
		} else {
			pollable = false
		}
	}

	// An error here indicates a failure to register
	// with the netpoll system. That can happen for
	// a file descriptor that is not supported by
	// epoll/kqueue; for example, disk files on
	// Linux systems. We assume that any real error
	// will show up in later I/O.
	// We do restore the blocking behavior if it was set by us.
	if pollErr := f.pfd.Init("file", pollable); pollErr != nil && clearNonBlock {
		if err := syscall.SetNonblock(fd, false); err == nil {
			f.nonblock = false
		}
	}

	runtime.SetFinalizer(f.file, (*file).close)
	return f
}

func sigpipe() // implemented in package runtime

// epipecheck raises SIGPIPE if we get an EPIPE error on standard
// output or standard error. See the SIGPIPE docs in os/signal, and
// issue 11845.
func epipecheck(file *File, e error) {
	if e == syscall.EPIPE && file.stdoutOrErr {
		sigpipe()
	}
}

// DevNull is the name of the operating system's “null device.”
// On Unix-like systems, it is "/dev/null"; on Windows, "NUL".
const DevNull = "/dev/null"

// openFileNolog is the Unix implementation of OpenFile.
// Changes here should be reflected in openDirAt and openDirNolog, if relevant.
func openFileNolog(name string, flag int, perm FileMode) (*File, error) {
	setSticky := false
	if !supportsCreateWithStickyBit && flag&O_CREATE != 0 && perm&ModeSticky != 0 {
		if _, err := Stat(name); IsNotExist(err) {
			setSticky = true
		}
	}

	var (
		r int
		s poll.SysFile
		e error
	)
	// We have to check EINTR here, per issues 11180 and 39237.
	ignoringEINTR(func() error {
		r, s, e = open(name, flag|syscall.O_CLOEXEC, syscallMode(perm))
		return e
	})
	if e != nil {
		return nil, &PathError{Op: "open", Path: name, Err: e}
	}

	// open(2) itself won't handle the sticky bit on *BSD and Solaris
	if setSticky {
		setStickyBit(name)
	}

	// There's a race here with fork/exec, which we are
	// content to live with. See ../syscall/exec_unix.go.
	if !supportsCloseOnExec {
		syscall.CloseOnExec(r)
	}

	f := newFile(r, name, kindOpenFile, unix.HasNonblockFlag(flag))
	f.pfd.SysFile = s
	return f, nil
}

func openDirNolog(name string) (*File, error) {
	var (
		r int
		s poll.SysFile
		e error
	)
	ignoringEINTR(func() error {
		r, s, e = open(name, O_RDONLY|syscall.O_CLOEXEC|syscall.O_DIRECTORY, 0)
		return e
	})
	if e != nil {
		return nil, &PathError{Op: "open", Path: name, Err: e}
	}

	if !supportsCloseOnExec {
		syscall.CloseOnExec(r)
	}

	f := newFile(r, name, kindNoPoll, false)
	f.pfd.SysFile = s
	return f, nil
}

func (file *file) close() error {
	if file == nil {
		return syscall.EINVAL
	}
	if info := file.dirinfo.Swap(nil); info != nil {
		info.close()
	}
	var err error
	if e := file.pfd.Close(); e != nil {
		if e == poll.ErrFileClosing {
			e = ErrClosed
		}
		err = &PathError{Op: "close", Path: file.name, Err: e}
	}

	// no need for a finalizer anymore
	runtime.SetFinalizer(file, nil)
	return err
}

// seek sets the offset for the next Read or Write on file to offset, interpreted
// according to whence: 0 means relative to the origin of the file, 1 means
// relative to the current offset, and 2 means relative to the end.
// It returns the new offset and an error, if any.
func (f *File) seek(offset int64, whence int) (ret int64, err error) {
	if info := f.dirinfo.Swap(nil); info != nil {
		// Free cached dirinfo, so we allocate a new one if we
		// access this file as a directory again. See #35767 and #37161.
		info.close()
	}
	ret, err = f.pfd.Seek(offset, whence)
	runtime.KeepAlive(f)
	return ret, err
}

// Truncate changes the size of the named file.
// If the file is a symbolic link, it changes the size of the link's target.
// If there is an error, it will be of type [*PathError].
func Truncate(name string, size int64) error {
	e := ignoringEINTR(func() error {
		return syscall.Truncate(name, size)
	})
	if e != nil {
		return &PathError{Op: "truncate", Path: name, Err: e}
	}
	return nil
}

// Remove removes the named file or (empty) directory.
// If there is an error, it will be of type [*PathError].
func Remove(name string) error {
	// System call interface forces us to know
	// whether name is a file or directory.
	// Try both: it is cheaper on average than
	// doing a Stat plus the right one.
	e := ignoringEINTR(func() error {
		return syscall.Unlink(name)
	})
	if e == nil {
		return nil
	}
	e1 := ignoringEINTR(func() error {
		return syscall.Rmdir(name)
	})
	if e1 == nil {
		return nil
	}

	// Both failed: figure out which error to return.
	// OS X and Linux differ on whether unlink(dir)
	// returns EISDIR, so can't use that. However,
	// both agree that rmdir(file) returns ENOTDIR,
	// so we can use that to decide which error is real.
	// Rmdir might also return ENOTDIR if given a bad
	// file path, like /etc/passwd/foo, but in that case,
	// both errors will be ENOTDIR, so it's okay to
	// use the error from unlink.
	if e1 != syscall.ENOTDIR {
		e = e1
	}
	return &PathError{Op: "remove", Path: name, Err: e}
}

func tempDir() string {
	dir := Getenv("TMPDIR")
	if dir == "" {
		if runtime.GOOS == "android" {
			dir = "/data/local/tmp"
		} else {
			dir = "/tmp"
		}
	}
	return dir
}

// Link creates newname as a hard link to the oldname file.
// If there is an error, it will be of type *LinkError.
func Link(oldname, newname string) error {
	e := ignoringEINTR(func() error {
		return syscall.Link(oldname, newname)
	})
	if e != nil {
		return &LinkError{"link", oldname, newname, e}
	}
	return nil
}

// Symlink creates newname as a symbolic link to oldname.
// On Windows, a symlink to a non-existent oldname creates a file symlink;
// if oldname is later created as a directory the symlink will not work.
// If there is an error, it will be of type *LinkError.
func Symlink(oldname, newname string) error {
	e := ignoringEINTR(func() error {
		return syscall.Symlink(oldname, newname)
	})
	if e != nil {
		return &LinkError{"symlink", oldname, newname, e}
	}
	return nil
}

func readlink(name string) (string, error) {
	for len := 128; ; len *= 2 {
		b := make([]byte, len)
		n, err := ignoringEINTR2(func() (int, error) {
			return fixCount(syscall.Readlink(name, b))
		})
		// buffer too small
		if (runtime.GOOS == "aix" || runtime.GOOS == "wasip1") && err == syscall.ERANGE {
			continue
		}
		if err != nil {
			return "", &PathError{Op: "readlink", Path: name, Err: err}
		}
		if n < len {
			return string(b[0:n]), nil
		}
	}
}

type unixDirent struct {
	parent string
	name   string
	typ    FileMode
	info   FileInfo
}

func (d *unixDirent) Name() string   { return d.name }
func (d *unixDirent) IsDir() bool    { return d.typ.IsDir() }
func (d *unixDirent) Type() FileMode { return d.typ }

func (d *unixDirent) Info() (FileInfo, error) {
	if d.info != nil {
		return d.info, nil
	}
	return Lstat(d.parent + "/" + d.name)
}

func (d *unixDirent) String() string {
	return fs.FormatDirEntry(d)
}

func newUnixDirent(parent *File, name string, typ FileMode) (DirEntry, error) {
	ude := &unixDirent{
		parent: parent.name,
		name:   name,
		typ:    typ,
	}
	// When the parent file was opened in a Root,
	// we cannot use a lazy lstat to load the FileInfo.
	// Use lstatat here.
	if typ != ^FileMode(0) && !parent.inRoot {
		return ude, nil
	}

	info, err := parent.lstatat(name)
	if err != nil {
		return nil, err
	}

	ude.typ = info.Mode().Type()
	ude.info = info
	return ude, nil
}

```

// === FILE: references!/go/src/os/file_wasip1.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build wasip1

package os

import "internal/poll"

// PollFD returns the poll.FD of the file.
//
// Other packages in std that also import internal/poll (such as net)
// can use a type assertion to access this extension method so that
// they can pass the *poll.FD to functions like poll.Splice.
//
// There is an equivalent function in net.rawConn.
//
// PollFD is not intended for use outside the standard library.
func (f *file) PollFD() *poll.FD {
	return &f.pfd
}

```

// === FILE: references!/go/src/os/file_windows.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"errors"
	"internal/filepathlite"
	"internal/godebug"
	"internal/poll"
	"internal/syscall/windows"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// This matches the value in syscall/syscall_windows.go.
const _UTIME_OMIT = -1

// file is the real representation of *File.
// The extra level of indirection ensures that no clients of os
// can overwrite this data, which could cause the finalizer
// to close the wrong file descriptor.
type file struct {
	pfd        poll.FD
	name       string
	dirinfo    atomic.Pointer[dirInfo] // nil unless directory being read
	appendMode bool                    // whether file is opened for appending
}

// fd is the Windows implementation of Fd.
func (file *File) fd() uintptr {
	if file == nil {
		return uintptr(syscall.InvalidHandle)
	}
	// Try to disassociate the file from the runtime poller.
	// File.Fd doesn't return an error, so we don't have a way to
	// report it. We just ignore it. It's up to the caller to call
	// it when there are no concurrent IO operations.
	_ = file.pfd.DisassociateIOCP()
	return uintptr(file.pfd.Sysfd)
}

// newFileKind describes the kind of file to newFile.
type newFileKind int

const (
	// kindNewFile means that the descriptor was passed to us via NewFile.
	kindNewFile newFileKind = iota
	// kindOpenFile means that the descriptor was opened using
	// Open, Create, or OpenFile.
	kindOpenFile
	// kindPipe means that the descriptor was opened using Pipe.
	kindPipe
	// kindSock means that the descriptor is a network file descriptor
	// that was created from net package and was opened using net_newUnixFile.
	kindSock
	// kindConsole means that the descriptor is a console handle.
	kindConsole
)

// newFile returns a new File with the given file handle and name.
// Unlike NewFile, it does not check that h is syscall.InvalidHandle.
// If nonBlocking is true, it tries to add the file to the runtime poller.
func newFile(h syscall.Handle, name string, kind newFileKind, nonBlocking bool) *File {
	typ := "file"
	switch kind {
	case kindNewFile, kindOpenFile:
		t, err := syscall.GetFileType(h)
		if err != nil || t == syscall.FILE_TYPE_CHAR {
			var m uint32
			if syscall.GetConsoleMode(h, &m) == nil {
				typ = "console"
				// Console handles are always blocking.
				break
			}
		} else if t == syscall.FILE_TYPE_PIPE {
			typ = "pipe"
		}
		// NewFile doesn't know if a handle is blocking or non-blocking,
		// so we try to detect that here. This call may block/ if the handle
		// is blocking and there is an outstanding I/O operation.
		//
		// Avoid doing this for Stdin, which is almost always blocking and might
		// be in use by other process when the "os" package is initializing.
		// See go.dev/issue/75949 and go.dev/issue/76391.
		if kind == kindNewFile && h != syscall.Stdin {
			nonBlocking, _ = windows.IsNonblock(h)
		}
	case kindPipe:
		typ = "pipe"
	case kindSock:
		typ = "file+net"
	case kindConsole:
		typ = "console"
	default:
		panic("newFile with unknown kind")
	}

	f := &File{&file{
		pfd: poll.FD{
			Sysfd:         h,
			IsStream:      true,
			ZeroReadIsEOF: true,
		},
		name: name,
	}}
	runtime.SetFinalizer(f.file, (*file).close)

	// Ignore initialization errors.
	// Assume any problems will show up in later I/O.
	f.pfd.Init(typ, nonBlocking)
	return f
}

// newConsoleFile creates new File that will be used as console.
func newConsoleFile(h syscall.Handle, name string) *File {
	return newFile(h, name, kindConsole, false)
}

// newFileFromNewFile is called by [NewFile].
func newFileFromNewFile(fd uintptr, name string) *File {
	h := syscall.Handle(fd)
	if h == syscall.InvalidHandle {
		return nil
	}
	return newFile(h, name, kindNewFile, false)
}

// net_newWindowsFile is a hidden entry point called by net.conn.File.
// This is used so that the File.pfd.close method calls [syscall.Closesocket]
// instead of [syscall.CloseHandle].
//
//go:linkname net_newWindowsFile net.newWindowsFile
func net_newWindowsFile(h syscall.Handle, name string) *File {
	if h == syscall.InvalidHandle {
		panic("invalid FD")
	}
	return newFile(h, name, kindSock, true)
}

func epipecheck(file *File, e error) {
}

// DevNull is the name of the operating system's “null device.”
// On Unix-like systems, it is "/dev/null"; on Windows, "NUL".
const DevNull = "NUL"

// openFileNolog is the Windows implementation of OpenFile.
func openFileNolog(name string, flag int, perm FileMode) (*File, error) {
	if name == "" {
		return nil, &PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}
	path := fixLongPath(name)
	r, err := syscall.Open(path, flag|syscall.O_CLOEXEC, syscallMode(perm))
	if err != nil {
		return nil, &PathError{Op: "open", Path: name, Err: err}
	}
	nonblocking := flag&windows.O_FILE_FLAG_OVERLAPPED != 0
	return newFile(r, name, kindOpenFile, nonblocking), nil
}

func openDirNolog(name string) (*File, error) {
	return openFileNolog(name, O_RDONLY|windows.O_DIRECTORY, 0)
}

func (file *file) close() error {
	if file == nil {
		return syscall.EINVAL
	}
	if info := file.dirinfo.Swap(nil); info != nil {
		info.close()
	}
	var err error
	if e := file.pfd.Close(); e != nil {
		if e == poll.ErrFileClosing {
			e = ErrClosed
		}
		err = &PathError{Op: "close", Path: file.name, Err: e}
	}

	// no need for a finalizer anymore
	runtime.SetFinalizer(file, nil)
	return err
}

// seek sets the offset for the next Read or Write on file to offset, interpreted
// according to whence: 0 means relative to the origin of the file, 1 means
// relative to the current offset, and 2 means relative to the end.
// It returns the new offset and an error, if any.
func (f *File) seek(offset int64, whence int) (ret int64, err error) {
	if info := f.dirinfo.Swap(nil); info != nil {
		// Free cached dirinfo, so we allocate a new one if we
		// access this file as a directory again. See #35767 and #37161.
		info.close()
	}
	ret, err = f.pfd.Seek(offset, whence)
	runtime.KeepAlive(f)
	return ret, err
}

// Truncate changes the size of the named file.
// If the file is a symbolic link, it changes the size of the link's target.
func Truncate(name string, size int64) error {
	f, e := OpenFile(name, O_WRONLY, 0666)
	if e != nil {
		return e
	}
	defer f.Close()
	e1 := f.Truncate(size)
	if e1 != nil {
		return e1
	}
	return nil
}

// Remove removes the named file or directory.
// If there is an error, it will be of type [*PathError].
func Remove(name string) error {
	p, e := syscall.UTF16PtrFromString(fixLongPath(name))
	if e != nil {
		return &PathError{Op: "remove", Path: name, Err: e}
	}

	// Go file interface forces us to know whether
	// name is a file or directory. Try both.
	e = syscall.DeleteFile(p)
	if e == nil {
		return nil
	}
	e1 := syscall.RemoveDirectory(p)
	if e1 == nil {
		return nil
	}

	// Both failed: figure out which error to return.
	if e1 != e {
		a, e2 := syscall.GetFileAttributes(p)
		if e2 != nil {
			e = e2
		} else {
			if a&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
				e = e1
			} else if a&syscall.FILE_ATTRIBUTE_READONLY != 0 {
				if e1 = syscall.SetFileAttributes(p, a&^syscall.FILE_ATTRIBUTE_READONLY); e1 == nil {
					if e = syscall.DeleteFile(p); e == nil {
						return nil
					}
				}
			}
		}
	}
	return &PathError{Op: "remove", Path: name, Err: e}
}

func rename(oldname, newname string) error {
	e := windows.Rename(fixLongPath(oldname), fixLongPath(newname))
	if e != nil {
		return &LinkError{"rename", oldname, newname, e}
	}
	return nil
}

// Pipe returns a connected pair of Files; reads from r return bytes written to w.
// It returns the files and an error, if any. The Windows handles underlying
// the returned files are marked as inheritable by child processes.
func Pipe() (r *File, w *File, err error) {
	var p [2]syscall.Handle
	e := syscall.Pipe(p[:])
	if e != nil {
		return nil, nil, NewSyscallError("pipe", e)
	}
	// syscall.Pipe always returns a non-blocking handle.
	return newFile(p[0], "|0", kindPipe, false), newFile(p[1], "|1", kindPipe, false), nil
}

var useGetTempPath2 = sync.OnceValue(func() bool {
	return windows.ErrorLoadingGetTempPath2() == nil
})

func tempDir() string {
	getTempPath := syscall.GetTempPath
	if useGetTempPath2() {
		getTempPath = windows.GetTempPath2
	}
	n := uint32(syscall.MAX_PATH)
	for {
		b := make([]uint16, n)
		n, _ = getTempPath(uint32(len(b)), &b[0])
		if n > uint32(len(b)) {
			continue
		}
		if n == 3 && b[1] == ':' && b[2] == '\\' {
			// Do nothing for path, like C:\.
		} else if n > 0 && b[n-1] == '\\' {
			// Otherwise remove terminating \.
			n--
		}
		return syscall.UTF16ToString(b[:n])
	}
}

// Link creates newname as a hard link to the oldname file.
// If there is an error, it will be of type *LinkError.
func Link(oldname, newname string) error {
	n, err := syscall.UTF16PtrFromString(fixLongPath(newname))
	if err != nil {
		return &LinkError{"link", oldname, newname, err}
	}
	o, err := syscall.UTF16PtrFromString(fixLongPath(oldname))
	if err != nil {
		return &LinkError{"link", oldname, newname, err}
	}
	err = syscall.CreateHardLink(n, o, 0)
	if err != nil {
		return &LinkError{"link", oldname, newname, err}
	}
	return nil
}

// Symlink creates newname as a symbolic link to oldname.
// On Windows, a symlink to a non-existent oldname creates a file symlink;
// if oldname is later created as a directory the symlink will not work.
// If there is an error, it will be of type *LinkError.
func Symlink(oldname, newname string) error {
	// '/' does not work in link's content
	oldname = filepathlite.FromSlash(oldname)

	// need the exact location of the oldname when it's relative to determine if it's a directory
	destpath := oldname
	if v := filepathlite.VolumeName(oldname); v == "" {
		if len(oldname) > 0 && IsPathSeparator(oldname[0]) {
			// oldname is relative to the volume containing newname.
			if v = filepathlite.VolumeName(newname); v != "" {
				// Prepend the volume explicitly, because it may be different from the
				// volume of the current working directory.
				destpath = v + oldname
			}
		} else {
			// oldname is relative to newname.
			destpath = dirname(newname) + `\` + oldname
		}
	}

	fi, err := Stat(destpath)
	isdir := err == nil && fi.IsDir()

	n, err := syscall.UTF16PtrFromString(fixLongPath(newname))
	if err != nil {
		return &LinkError{"symlink", oldname, newname, err}
	}
	var o *uint16
	if filepathlite.IsAbs(oldname) {
		o, err = syscall.UTF16PtrFromString(fixLongPath(oldname))
	} else {
		// Do not use fixLongPath on oldname for relative symlinks,
		// as it would turn the name into an absolute path thus making
		// an absolute symlink instead.
		// Notice that CreateSymbolicLinkW does not fail for relative
		// symlinks beyond MAX_PATH, so this does not prevent the
		// creation of an arbitrary long path name.
		o, err = syscall.UTF16PtrFromString(oldname)
	}
	if err != nil {
		return &LinkError{"symlink", oldname, newname, err}
	}

	var flags uint32 = windows.SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE
	if isdir {
		flags |= syscall.SYMBOLIC_LINK_FLAG_DIRECTORY
	}
	err = syscall.CreateSymbolicLink(n, o, flags)
	if err != nil {
		// the unprivileged create flag is unsupported
		// below Windows 10 (1703, v10.0.14972). retry without it.
		flags &^= windows.SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE
		err = syscall.CreateSymbolicLink(n, o, flags)
		if err != nil {
			return &LinkError{"symlink", oldname, newname, err}
		}
	}
	return nil
}

// openSymlink calls CreateFile Windows API with FILE_FLAG_OPEN_REPARSE_POINT
// parameter, so that Windows does not follow symlink, if path is a symlink.
// openSymlink returns opened file handle.
func openSymlink(path string) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	attrs := uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS)
	// Use FILE_FLAG_OPEN_REPARSE_POINT, otherwise CreateFile will follow symlink.
	// See https://docs.microsoft.com/en-us/windows/desktop/FileIO/symbolic-link-effects-on-file-systems-functions#createfile-and-createfiletransacted
	attrs |= syscall.FILE_FLAG_OPEN_REPARSE_POINT
	h, err := syscall.CreateFile(p, 0, 0, nil, syscall.OPEN_EXISTING, attrs, 0)
	if err != nil {
		return 0, err
	}
	return h, nil
}

var winreadlinkvolume = godebug.New("winreadlinkvolume")

// normaliseLinkPath converts absolute paths returned by
// DeviceIoControl(h, FSCTL_GET_REPARSE_POINT, ...)
// into paths acceptable by all Windows APIs.
// For example, it converts
//
//	\??\C:\foo\bar into C:\foo\bar
//	\??\UNC\foo\bar into \\foo\bar
//	\??\Volume{abc}\ into \\?\Volume{abc}\
func normaliseLinkPath(path string) (string, error) {
	if len(path) < 4 || path[:4] != `\??\` {
		// unexpected path, return it as is
		return path, nil
	}
	// we have path that start with \??\
	s := path[4:]
	switch {
	case len(s) >= 2 && s[1] == ':': // \??\C:\foo\bar
		return s, nil
	case len(s) >= 4 && s[:4] == `UNC\`: // \??\UNC\foo\bar
		return `\\` + s[4:], nil
	}

	// \??\Volume{abc}\
	if winreadlinkvolume.Value() != "0" {
		return `\\?\` + path[4:], nil
	}
	winreadlinkvolume.IncNonDefault()

	h, err := openSymlink(path)
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(h)

	buf := make([]uint16, 100)
	for {
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), windows.VOLUME_NAME_DOS)
		if err != nil {
			return "", err
		}
		if n < uint32(len(buf)) {
			break
		}
		buf = make([]uint16, n)
	}
	s = syscall.UTF16ToString(buf)
	if len(s) > 4 && s[:4] == `\\?\` {
		s = s[4:]
		if len(s) > 3 && s[:3] == `UNC` {
			// return path like \\server\share\...
			return `\` + s[3:], nil
		}
		return s, nil
	}
	return "", errors.New("GetFinalPathNameByHandle returned unexpected path: " + s)
}

func readReparseLink(path string) (string, error) {
	h, err := openSymlink(path)
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(h)
	return readReparseLinkHandle(h)
}

func readReparseLinkHandle(h syscall.Handle) (string, error) {
	rdbbuf := make([]byte, syscall.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)
	var bytesReturned uint32
	err := syscall.DeviceIoControl(h, syscall.FSCTL_GET_REPARSE_POINT, nil, 0, &rdbbuf[0], uint32(len(rdbbuf)), &bytesReturned, nil)
	if err != nil {
		return "", err
	}

	rdb := (*windows.REPARSE_DATA_BUFFER)(unsafe.Pointer(&rdbbuf[0]))
	switch rdb.ReparseTag {
	case syscall.IO_REPARSE_TAG_SYMLINK:
		rb := (*windows.SymbolicLinkReparseBuffer)(unsafe.Pointer(&rdb.DUMMYUNIONNAME))
		s := rb.Path()
		if rb.Flags&windows.SYMLINK_FLAG_RELATIVE != 0 {
			return s, nil
		}
		return normaliseLinkPath(s)
	case windows.IO_REPARSE_TAG_MOUNT_POINT:
		return normaliseLinkPath((*windows.MountPointReparseBuffer)(unsafe.Pointer(&rdb.DUMMYUNIONNAME)).Path())
	default:
		// the path is not a symlink or junction but another type of reparse
		// point
		return "", syscall.ENOENT
	}
}

func readlink(name string) (string, error) {
	s, err := readReparseLink(fixLongPath(name))
	if err != nil {
		return "", &PathError{Op: "readlink", Path: name, Err: err}
	}
	return s, nil
}

```

// === FILE: references!/go/src/os/getwd.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"runtime"
	"sync"
	"syscall"
)

var getwdCache struct {
	sync.Mutex
	dir string
}

// Getwd returns an absolute path name corresponding to the
// current directory. If the current directory can be
// reached via multiple paths (due to symbolic links),
// Getwd may return any one of them.
//
// On Unix platforms, if the environment variable PWD
// provides an absolute name, and it is a name of the
// current directory, it is returned.
func Getwd() (dir string, err error) {
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		// Use syscall.Getwd directly for
		//   - plan9: see reasons in CL 89575;
		//   - windows: syscall implementation is sufficient,
		//     and we should not rely on $PWD.
		dir, err = syscall.Getwd()
		return dir, NewSyscallError("getwd", err)
	}

	// Clumsy but widespread kludge:
	// if $PWD is set and matches ".", use it.
	var dot FileInfo
	dir = Getenv("PWD")
	if len(dir) > 0 && dir[0] == '/' {
		dot, err = statNolog(".")
		if err != nil {
			return "", err
		}
		d, err := statNolog(dir)
		if err == nil && SameFile(dot, d) {
			return dir, nil
		}
		// If err is ENAMETOOLONG here, the syscall.Getwd below will
		// fail with the same error, too, but let's give it a try
		// anyway as the fallback code is much slower.
	}

	// If the operating system provides a Getwd call, use it.
	if syscall.ImplementsGetwd {
		dir, err = ignoringEINTR2(syscall.Getwd)
		// Linux returns ENAMETOOLONG if the result is too long.
		// Some BSD systems appear to return EINVAL.
		// FreeBSD systems appear to use ENOMEM
		// Solaris appears to use ERANGE.
		if err != syscall.ENAMETOOLONG && err != syscall.EINVAL && err != errERANGE && err != errENOMEM {
			return dir, NewSyscallError("getwd", err)
		}
	}

	// We're trying to find our way back to ".".
	if dot == nil {
		dot, err = statNolog(".")
		if err != nil {
			return "", err
		}
	}
	// Apply same kludge but to cached dir instead of $PWD.
	getwdCache.Lock()
	dir = getwdCache.dir
	getwdCache.Unlock()
	if len(dir) > 0 {
		d, err := statNolog(dir)
		if err == nil && SameFile(dot, d) {
			return dir, nil
		}
	}

	// Root is a special case because it has no parent
	// and ends in a slash.
	root, err := statNolog("/")
	if err != nil {
		// Can't stat root - no hope.
		return "", err
	}
	if SameFile(root, dot) {
		return "/", nil
	}

	// General algorithm: find name in parent
	// and then find name of parent. Each iteration
	// adds /name to the beginning of dir.
	dir = ""
	for parent := ".."; ; parent = "../" + parent {
		if len(parent) >= 1024 { // Sanity check
			return "", NewSyscallError("getwd", syscall.ENAMETOOLONG)
		}
		fd, err := openDirNolog(parent)
		if err != nil {
			return "", err
		}

		for {
			names, err := fd.Readdirnames(100)
			if err != nil {
				fd.Close()
				// Readdirnames can return io.EOF or other error.
				// In any case, we're here because syscall.Getwd
				// is not implemented or failed with ENAMETOOLONG,
				// so return the most sensible error.
				if syscall.ImplementsGetwd {
					return "", NewSyscallError("getwd", syscall.ENAMETOOLONG)
				}
				return "", NewSyscallError("getwd", errENOSYS)
			}
			for _, name := range names {
				d, _ := lstatNolog(parent + "/" + name)
				if SameFile(d, dot) {
					dir = "/" + name + dir
					goto Found
				}
			}
		}

	Found:
		pd, err := fd.Stat()
		fd.Close()
		if err != nil {
			return "", err
		}
		if SameFile(pd, root) {
			break
		}
		// Set up for next round.
		dot = pd
	}

	// Save answer as hint to avoid the expensive path next time.
	getwdCache.Lock()
	getwdCache.dir = dir
	getwdCache.Unlock()

	return dir, nil
}

```

// === FILE: references!/go/src/os/path.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/filepathlite"
	"syscall"
)

// MkdirAll creates a directory named path,
// along with any necessary parents, and returns nil,
// or else returns an error.
// The permission bits perm (before umask) are used for all
// directories that MkdirAll creates.
// If path is already a directory, MkdirAll does nothing
// and returns nil.
func MkdirAll(path string, perm FileMode) error {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.

	// Extract the parent folder from path by first removing any trailing
	// path separator and then scanning backward until finding a path
	// separator or reaching the beginning of the string.
	i := len(path) - 1
	for i >= 0 && IsPathSeparator(path[i]) {
		i--
	}
	for i >= 0 && !IsPathSeparator(path[i]) {
		i--
	}
	if i < 0 {
		i = 0
	}

	// If there is a parent directory, and it is not the volume name,
	// recurse to ensure parent directory exists.
	if parent := path[:i]; len(parent) > len(filepathlite.VolumeName(path)) {
		err = MkdirAll(parent, perm)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}
	return nil
}

// RemoveAll removes path and any children it contains.
// It removes everything it can but returns the first error
// it encounters. If the path does not exist, RemoveAll
// returns nil (no error).
// If there is an error, it will be of type [*PathError].
func RemoveAll(path string) error {
	return removeAll(path)
}

// endsWithDot reports whether the final component of path is ".".
func endsWithDot(path string) bool {
	if path == "." {
		return true
	}
	if len(path) >= 2 && path[len(path)-1] == '.' && IsPathSeparator(path[len(path)-2]) {
		return true
	}
	return false
}

```

// === FILE: references!/go/src/os/path_plan9.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

const (
	PathSeparator     = '/'    // OS-specific path separator
	PathListSeparator = '\000' // OS-specific path list separator
)

// IsPathSeparator reports whether c is a directory separator character.
func IsPathSeparator(c uint8) bool {
	return PathSeparator == c
}

```

// === FILE: references!/go/src/os/path_unix.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || (js && wasm) || wasip1

package os

const (
	PathSeparator     = '/' // OS-specific path separator
	PathListSeparator = ':' // OS-specific path list separator
)

// IsPathSeparator reports whether c is a directory separator character.
func IsPathSeparator(c uint8) bool {
	return PathSeparator == c
}

// splitPath returns the base name and parent directory.
func splitPath(path string) (string, string) {
	// if no better parent is found, the path is relative from "here"
	dirname := "."

	// Remove all but one leading slash.
	for len(path) > 1 && path[0] == '/' && path[1] == '/' {
		path = path[1:]
	}

	i := len(path) - 1

	// Remove trailing slashes.
	for ; i > 0 && path[i] == '/'; i-- {
		path = path[:i]
	}

	// if no slashes in path, base is path
	basename := path

	// Remove leading directory path
	for i--; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				dirname = path[:1]
			} else {
				dirname = path[:i]
			}
			basename = path[i+1:]
			break
		}
	}

	return dirname, basename
}

```

// === FILE: references!/go/src/os/path_windows.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/filepathlite"
	"internal/syscall/windows"
	"syscall"
)

const (
	PathSeparator     = '\\' // OS-specific path separator
	PathListSeparator = ';'  // OS-specific path list separator
)

// IsPathSeparator reports whether c is a directory separator character.
func IsPathSeparator(c uint8) bool {
	// NOTE: Windows accepts / as path separator.
	return c == '\\' || c == '/'
}

// splitPath returns the base name and parent directory.
func splitPath(path string) (string, string) {
	if path == "" {
		return ".", "."
	}

	// The first prefixlen bytes are part of the parent directory.
	// The prefix consists of the volume name (if any) and the first \ (if significant).
	prefixlen := filepathlite.VolumeNameLen(path)
	if len(path) > prefixlen && IsPathSeparator(path[prefixlen]) {
		if prefixlen == 0 {
			// This is a path relative to the current volume, like \foo.
			// Include the initial \ in the prefix.
			prefixlen = 1
		} else if path[prefixlen-1] == ':' {
			// This is an absolute path on a named drive, like c:\foo.
			// Include the initial \ in the prefix.
			prefixlen++
		}
	}

	i := len(path) - 1

	// Remove trailing slashes.
	for i >= prefixlen && IsPathSeparator(path[i]) {
		i--
	}
	path = path[:i+1]

	// Find the last path separator. The basename is what follows.
	for i >= prefixlen && !IsPathSeparator(path[i]) {
		i--
	}
	basename := path[i+1:]
	if basename == "" {
		basename = "."
	}

	// Remove trailing slashes. The remainder is dirname.
	for i >= prefixlen && IsPathSeparator(path[i]) {
		i--
	}
	dirname := path[:i+1]
	if dirname == "" {
		dirname = "."
	}

	return dirname, basename
}

func dirname(path string) string {
	vol := filepathlite.VolumeName(path)
	i := len(path) - 1
	for i >= len(vol) && !IsPathSeparator(path[i]) {
		i--
	}
	dir := path[len(vol) : i+1]
	last := len(dir) - 1
	if last > 0 && IsPathSeparator(dir[last]) {
		dir = dir[:last]
	}
	if dir == "" {
		dir = "."
	}
	return vol + dir
}

// fixLongPath returns the extended-length (\\?\-prefixed) form of
// path when needed, in order to avoid the default 260 character file
// path limit imposed by Windows. If the path is short enough or already
// has the extended-length prefix, fixLongPath returns path unmodified.
// If the path is relative and joining it with the current working
// directory results in a path that is too long, fixLongPath returns
// the absolute path with the extended-length prefix.
//
// See https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file#maximum-path-length-limitation
func fixLongPath(path string) string {
	if windows.CanUseLongPaths {
		return path
	}
	return addExtendedPrefix(path)
}

// addExtendedPrefix adds the extended path prefix (\\?\) to path.
func addExtendedPrefix(path string) string {
	if len(path) >= 4 {
		if path[:4] == `\??\` {
			// Already extended with \??\
			return path
		}
		if IsPathSeparator(path[0]) && IsPathSeparator(path[1]) && path[2] == '?' && IsPathSeparator(path[3]) {
			// Already extended with \\?\ or any combination of directory separators.
			return path
		}
	}

	// Do nothing (and don't allocate) if the path is "short".
	// Empirically (at least on the Windows Server 2013 builder),
	// the kernel is arbitrarily okay with < 248 bytes. That
	// matches what the docs above say:
	// "When using an API to create a directory, the specified
	// path cannot be so long that you cannot append an 8.3 file
	// name (that is, the directory name cannot exceed MAX_PATH
	// minus 12)." Since MAX_PATH is 260, 260 - 12 = 248.
	//
	// The MSDN docs appear to say that a normal path that is 248 bytes long
	// will work; empirically the path must be less then 248 bytes long.
	pathLength := len(path)
	if !filepathlite.IsAbs(path) {
		// If the path is relative, we need to prepend the working directory
		// plus a separator to the path before we can determine if it's too long.
		// We don't want to call syscall.Getwd here, as that call is expensive to do
		// every time fixLongPath is called with a relative path, so we use a cache.
		// Note that getwdCache might be outdated if the working directory has been
		// changed without using os.Chdir, i.e. using syscall.Chdir directly or cgo.
		// This is fine, as the worst that can happen is that we fail to fix the path.
		getwdCache.Lock()
		if getwdCache.dir == "" {
			// Init the working directory cache.
			getwdCache.dir, _ = syscall.Getwd()
		}
		pathLength += len(getwdCache.dir) + 1
		getwdCache.Unlock()
	}

	if pathLength < 248 {
		// Don't fix. (This is how Go 1.7 and earlier worked,
		// not automatically generating the \\?\ form)
		return path
	}

	var isUNC, isDevice bool
	if len(path) >= 2 && IsPathSeparator(path[0]) && IsPathSeparator(path[1]) {
		if len(path) >= 4 && path[2] == '.' && IsPathSeparator(path[3]) {
			// Starts with //./
			isDevice = true
		} else {
			// Starts with //
			isUNC = true
		}
	}
	var prefix []uint16
	if isUNC {
		// UNC path, prepend the \\?\UNC\ prefix.
		prefix = []uint16{'\\', '\\', '?', '\\', 'U', 'N', 'C', '\\'}
	} else if isDevice {
		// Don't add the extended prefix to device paths, as it would
		// change its meaning.
	} else {
		prefix = []uint16{'\\', '\\', '?', '\\'}
	}

	p, err := syscall.UTF16FromString(path)
	if err != nil {
		return path
	}
	// Estimate the required buffer size using the path length plus the null terminator.
	// pathLength includes the working directory. This should be accurate unless
	// the working directory has changed without using os.Chdir.
	n := uint32(pathLength) + 1
	var buf []uint16
	for {
		buf = make([]uint16, n+uint32(len(prefix)))
		n, err = syscall.GetFullPathName(&p[0], n, &buf[len(prefix)], nil)
		if err != nil {
			return path
		}
		if n <= uint32(len(buf)-len(prefix)) {
			buf = buf[:n+uint32(len(prefix))]
			break
		}
	}
	if isUNC {
		// Remove leading \\.
		buf = buf[2:]
	}
	copy(buf, prefix)
	return syscall.UTF16ToString(buf)
}

```

// === FILE: references!/go/src/os/pidfd_linux.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Support for pidfd was added during the course of a few Linux releases:
//  v5.1: pidfd_send_signal syscall;
//  v5.2: CLONE_PIDFD flag for clone syscall;
//  v5.3: pidfd_open syscall, clone3 syscall;
//  v5.4: P_PIDFD idtype support for waitid syscall;
//  v5.6: pidfd_getfd syscall.
//
// N.B. Alternative Linux implementations may not follow this ordering. e.g.,
// QEMU user mode 7.2 added pidfd_open, but CLONE_PIDFD was not added until
// 8.0.

package os

import (
	"internal/syscall/unix"
	"runtime"
	"sync"
	"syscall"
	_ "unsafe" // for linkname
)

// ensurePidfd initializes the PidFD field in sysAttr if it is not already set.
// It returns the original or modified SysProcAttr struct and a flag indicating
// whether the PidFD should be duplicated before using.
func ensurePidfd(sysAttr *syscall.SysProcAttr) (*syscall.SysProcAttr, bool) {
	if !pidfdWorks() {
		return sysAttr, false
	}

	var pidfd int

	if sysAttr == nil {
		return &syscall.SysProcAttr{
			PidFD: &pidfd,
		}, false
	}
	if sysAttr.PidFD == nil {
		newSys := *sysAttr // copy
		newSys.PidFD = &pidfd
		return &newSys, false
	}

	return sysAttr, true
}

// getPidfd returns the value of sysAttr.PidFD (or its duplicate if needDup is
// set) and a flag indicating whether the value can be used.
func getPidfd(sysAttr *syscall.SysProcAttr, needDup bool) (uintptr, bool) {
	if !pidfdWorks() {
		return 0, false
	}

	h := *sysAttr.PidFD
	if needDup {
		dupH, e := unix.Fcntl(h, syscall.F_DUPFD_CLOEXEC, 0)
		if e != nil {
			return 0, false
		}
		h = dupH
	}
	return uintptr(h), true
}

// pidfdFind returns the process handle for pid.
func pidfdFind(pid int) (uintptr, error) {
	if !pidfdWorks() {
		return 0, syscall.ENOSYS
	}

	h, err := unix.PidFDOpen(pid, 0)
	if err != nil {
		return 0, convertESRCH(err)
	}
	return h, nil
}

// pidfdWait waits for the process to complete,
// and updates the process status to done.
func (p *Process) pidfdWait() (*ProcessState, error) {
	// When pidfd is used, there is no wait/kill race (described in CL 23967)
	// because the PID recycle issue doesn't exist (IOW, pidfd, unlike PID,
	// is guaranteed to refer to one particular process). Thus, there is no
	// need for the workaround (blockUntilWaitable + sigMu) from pidWait.
	//
	// We _do_ need to be careful about reuse of the pidfd FD number when
	// closing the pidfd. See handle for more details.
	handle, status := p.handleTransientAcquire()
	switch status {
	case statusDone:
		// Process already completed Wait, or was not found by
		// pidfdFind. Return ECHILD for consistency with what the wait
		// syscall would return.
		return nil, NewSyscallError("wait", syscall.ECHILD)
	case statusReleased:
		return nil, syscall.EINVAL
	}
	defer p.handleTransientRelease()

	var (
		info   unix.SiginfoChild
		rusage syscall.Rusage
	)
	err := ignoringEINTR(func() error {
		return unix.Waitid(unix.P_PIDFD, int(handle), &info, syscall.WEXITED, &rusage)
	})
	if err != nil {
		return nil, NewSyscallError("waitid", err)
	}

	// Update the Process status to statusDone.
	// This also releases a reference to the handle.
	p.doRelease(statusDone)

	return &ProcessState{
		pid:    int(info.Pid),
		status: info.WaitStatus(),
		rusage: &rusage,
	}, nil
}

// pidfdSendSignal sends a signal to the process.
func (p *Process) pidfdSendSignal(s syscall.Signal) error {
	handle, status := p.handleTransientAcquire()
	switch status {
	case statusDone:
		return ErrProcessDone
	case statusReleased:
		return errProcessReleased
	}
	defer p.handleTransientRelease()

	return convertESRCH(unix.PidFDSendSignal(handle, s))
}

// pidfdWorks returns whether we can use pidfd on this system.
func pidfdWorks() bool {
	return checkPidfdOnce() == nil
}

// checkPidfdOnce is used to only check whether pidfd works once.
var checkPidfdOnce = sync.OnceValue(checkPidfd)

// checkPidfd checks whether all required pidfd-related syscalls work. This
// consists of pidfd_open and pidfd_send_signal syscalls, waitid syscall with
// idtype of P_PIDFD, and clone(CLONE_PIDFD).
//
// Reasons for non-working pidfd syscalls include an older kernel and an
// execution environment in which the above system calls are restricted by
// seccomp or a similar technology.
func checkPidfd() error {
	// In Android version < 12, pidfd-related system calls are not allowed
	// by seccomp and trigger the SIGSYS signal. See issue #69065.
	if runtime.GOOS == "android" {
		ignoreSIGSYS()
		defer restoreSIGSYS()
	}

	// Get a pidfd of the current process (opening of "/proc/self" won't
	// work for waitid).
	fd, err := unix.PidFDOpen(syscall.Getpid(), 0)
	if err != nil {
		return NewSyscallError("pidfd_open", err)
	}
	defer syscall.Close(int(fd))

	// Check waitid(P_PIDFD) works.
	err = ignoringEINTR(func() error {
		var info unix.SiginfoChild
		// We don't actually care about the info, but passing a nil pointer
		// makes valgrind complain because 0x0 is unaddressable.
		return unix.Waitid(unix.P_PIDFD, int(fd), &info, syscall.WEXITED, nil)
	})
	// Expect ECHILD from waitid since we're not our own parent.
	if err != syscall.ECHILD {
		return NewSyscallError("pidfd_wait", err)
	}

	// Check pidfd_send_signal works (should be able to send 0 to itself).
	if err := unix.PidFDSendSignal(fd, 0); err != nil {
		return NewSyscallError("pidfd_send_signal", err)
	}

	// Verify that clone(CLONE_PIDFD) works.
	//
	// This shouldn't be necessary since pidfd_open was added in Linux 5.3,
	// after CLONE_PIDFD in Linux 5.2, but some alternative Linux
	// implementations may not adhere to this ordering.
	if err := checkClonePidfd(); err != nil {
		return err
	}

	return nil
}

// Provided by syscall.
//
//go:linkname checkClonePidfd
func checkClonePidfd() error

// Provided by runtime.
//
//go:linkname ignoreSIGSYS
func ignoreSIGSYS()

//go:linkname restoreSIGSYS
func restoreSIGSYS()

```

// === FILE: references!/go/src/os/pidfd_other.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (unix && !linux) || (js && wasm) || wasip1 || windows

package os

import "syscall"

func ensurePidfd(sysAttr *syscall.SysProcAttr) (*syscall.SysProcAttr, bool) {
	return sysAttr, false
}

func getPidfd(_ *syscall.SysProcAttr, _ bool) (uintptr, bool) {
	return 0, false
}

func pidfdFind(_ int) (uintptr, error) {
	return 0, syscall.ENOSYS
}

func (_ *Process) pidfdWait() (*ProcessState, error) {
	panic("unreachable")
}

func (_ *Process) pidfdSendSignal(_ syscall.Signal) error {
	panic("unreachable")
}

```

// === FILE: references!/go/src/os/pipe2_unix.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build dragonfly || freebsd || linux || netbsd || openbsd || solaris

package os

import "syscall"

// Pipe returns a connected pair of Files; reads from r return bytes written to w.
// It returns the files and an error, if any.
func Pipe() (r *File, w *File, err error) {
	var p [2]int

	e := syscall.Pipe2(p[0:], syscall.O_CLOEXEC)
	if e != nil {
		return nil, nil, NewSyscallError("pipe2", e)
	}

	return newFile(p[0], "|0", kindPipe, false), newFile(p[1], "|1", kindPipe, false), nil
}

```

// === FILE: references!/go/src/os/pipe_unix.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || darwin

package os

import "syscall"

// Pipe returns a connected pair of Files; reads from r return bytes written to w.
// It returns the files and an error, if any.
func Pipe() (r *File, w *File, err error) {
	var p [2]int

	// See ../syscall/exec.go for description of lock.
	syscall.ForkLock.RLock()
	e := syscall.Pipe(p[0:])
	if e != nil {
		syscall.ForkLock.RUnlock()
		return nil, nil, NewSyscallError("pipe", e)
	}
	syscall.CloseOnExec(p[0])
	syscall.CloseOnExec(p[1])
	syscall.ForkLock.RUnlock()

	return newFile(p[0], "|0", kindPipe, false), newFile(p[1], "|1", kindPipe, false), nil
}

```

// === FILE: references!/go/src/os/pipe_wasm.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build wasm

package os

import "syscall"

// Pipe returns a connected pair of Files; reads from r return bytes written to w.
// It returns the files and an error, if any.
func Pipe() (r *File, w *File, err error) {
	// Neither GOOS=js nor GOOS=wasip1 have pipes.
	return nil, nil, NewSyscallError("pipe", syscall.ENOSYS)
}

```

// === FILE: references!/go/src/os/proc.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Process etc.

package os

import (
	"internal/testlog"
	"runtime"
	"syscall"
)

// Args hold the command-line arguments, starting with the program name.
var Args []string

func init() {
	if runtime.GOOS == "windows" {
		// Initialized in exec_windows.go.
		return
	}
	Args = runtime_args()
}

func runtime_args() []string // in package runtime

// Getuid returns the numeric user id of the caller.
//
// On Windows, it returns -1.
func Getuid() int { return syscall.Getuid() }

// Geteuid returns the numeric effective user id of the caller.
//
// On Windows, it returns -1.
func Geteuid() int { return syscall.Geteuid() }

// Getgid returns the numeric group id of the caller.
//
// On Windows, it returns -1.
func Getgid() int { return syscall.Getgid() }

// Getegid returns the numeric effective group id of the caller.
//
// On Windows, it returns -1.
func Getegid() int { return syscall.Getegid() }

// Getgroups returns a list of the numeric ids of groups that the caller belongs to.
//
// On Windows, it returns [syscall.EWINDOWS]. See the [os/user] package
// for a possible alternative.
func Getgroups() ([]int, error) {
	gids, e := syscall.Getgroups()
	return gids, NewSyscallError("getgroups", e)
}

// Exit causes the current program to exit with the given status code.
// Conventionally, code zero indicates success, non-zero an error.
// The program terminates immediately; deferred functions are not run.
//
// For portability, the status code should be in the range [0, 125].
func Exit(code int) {
	if code == 0 && testlog.PanicOnExit0() {
		// We were told to panic on calls to os.Exit(0).
		// This is used to fail tests that make an early
		// unexpected call to os.Exit(0).
		panic("unexpected call to os.Exit(0) during test")
	}

	// Inform the runtime that os.Exit is being called. If -race is
	// enabled, this will give race detector a chance to fail the
	// program (racy programs do not have the right to finish
	// successfully). If coverage is enabled, then this call will
	// enable us to write out a coverage data file.
	runtime_beforeExit(code)

	syscall.Exit(code)
}

func runtime_beforeExit(exitCode int) // implemented in runtime

```

// === FILE: references!/go/src/os/rawconn.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !plan9

package os

import (
	"runtime"
)

// rawConn implements syscall.RawConn.
type rawConn struct {
	file *File
}

func (c *rawConn) Control(f func(uintptr)) error {
	if err := c.file.checkValid("SyscallConn.Control"); err != nil {
		return err
	}
	err := c.file.pfd.RawControl(f)
	runtime.KeepAlive(c.file)
	return err
}

func (c *rawConn) Read(f func(uintptr) bool) error {
	if err := c.file.checkValid("SyscallConn.Read"); err != nil {
		return err
	}
	err := c.file.pfd.RawRead(f)
	runtime.KeepAlive(c.file)
	return err
}

func (c *rawConn) Write(f func(uintptr) bool) error {
	if err := c.file.checkValid("SyscallConn.Write"); err != nil {
		return err
	}
	err := c.file.pfd.RawWrite(f)
	runtime.KeepAlive(c.file)
	return err
}

func newRawConn(file *File) (*rawConn, error) {
	return &rawConn{file: file}, nil
}

```

// === FILE: references!/go/src/os/removeall_at.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || wasip1 || windows

package os

import (
	"io"
	"runtime"
	"syscall"
)

func removeAll(path string) error {
	if path == "" {
		// fail silently to retain compatibility with previous behavior
		// of RemoveAll. See issue 28830.
		return nil
	}

	// The rmdir system call does not permit removing ".",
	// so we don't permit it either.
	if endsWithDot(path) {
		return &PathError{Op: "RemoveAll", Path: path, Err: syscall.EINVAL}
	}

	// Simple case: if Remove works, we're done.
	err := Remove(path)
	if err == nil || IsNotExist(err) {
		return nil
	}

	// RemoveAll recurses by deleting the path base from
	// its parent directory
	parentDir, base := splitPath(path)

	flag := O_RDONLY
	if runtime.GOOS == "windows" {
		// On Windows, the process might not have read permission on the parent directory,
		// but still can delete files in it. See https://go.dev/issue/74134.
		// We can open a file even if we don't have read permission by passing the
		// O_WRONLY | O_RDWR flag, which is mapped to FILE_READ_ATTRIBUTES.
		flag = O_WRONLY | O_RDWR
	}
	parent, err := OpenFile(parentDir, flag, 0)
	if IsNotExist(err) {
		// If parent does not exist, base cannot exist. Fail silently
		return nil
	}
	if err != nil {
		return err
	}
	defer parent.Close()

	if err := removeAllFrom(sysfdType(parent.Fd()), base); err != nil {
		if pathErr, ok := err.(*PathError); ok {
			pathErr.Path = parentDir + string(PathSeparator) + pathErr.Path
			err = pathErr
		}
		return err
	}
	return nil
}

func removeAllFrom(parentFd sysfdType, base string) error {
	// Simple case: if Unlink (aka remove) works, we're done.
	err := removefileat(parentFd, base)
	if err == nil || IsNotExist(err) {
		return nil
	}

	// EISDIR means that we have a directory, and we need to
	// remove its contents.
	// EPERM or EACCES means that we don't have write permission on
	// the parent directory, but this entry might still be a directory
	// whose contents need to be removed.
	// Otherwise just return the error.
	if err != syscall.EISDIR && err != syscall.EPERM && err != syscall.EACCES {
		return &PathError{Op: "unlinkat", Path: base, Err: err}
	}
	uErr := err

	// Remove the directory's entries.
	var recurseErr error
	for {
		const reqSize = 1024
		var respSize int

		// Open the directory to recurse into.
		file, err := openDirAt(parentFd, base)
		if err != nil {
			if IsNotExist(err) {
				return nil
			}
			if err == syscall.ENOTDIR {
				// Not a directory; return the error from the removefileat.
				return &PathError{Op: "unlinkat", Path: base, Err: uErr}
			}
			if _, ok := err.(errSymlink); ok {
				// Not a user-visible error.
				err = uErr
			}
			recurseErr = &PathError{Op: "openfdat", Path: base, Err: err}
			break
		}

		for {
			numErr := 0

			names, readErr := file.Readdirnames(reqSize)
			// Errors other than EOF should stop us from continuing.
			if readErr != nil && readErr != io.EOF {
				file.Close()
				if IsNotExist(readErr) {
					return nil
				}
				return &PathError{Op: "readdirnames", Path: base, Err: readErr}
			}

			respSize = len(names)
			for _, name := range names {
				err := removeAllFrom(sysfdType(file.Fd()), name)
				if err != nil {
					if pathErr, ok := err.(*PathError); ok {
						pathErr.Path = base + string(PathSeparator) + pathErr.Path
					}
					numErr++
					if recurseErr == nil {
						recurseErr = err
					}
				}
			}

			// If we can delete any entry, break to start new iteration.
			// Otherwise, we discard current names, get next entries and try deleting them.
			if numErr != reqSize {
				break
			}
		}

		// Removing files from the directory may have caused
		// the OS to reshuffle it. Simply calling Readdirnames
		// again may skip some entries. The only reliable way
		// to avoid this is to close and re-open the
		// directory. See issue 20841.
		file.Close()

		// Finish when the end of the directory is reached
		if respSize < reqSize {
			break
		}
	}

	// Remove the directory itself.
	unlinkError := removedirat(parentFd, base)
	if unlinkError == nil || IsNotExist(unlinkError) {
		return nil
	}

	if recurseErr != nil {
		return recurseErr
	}
	return &PathError{Op: "unlinkat", Path: base, Err: unlinkError}
}

// openDirAt opens a directory name relative to the directory referred to by
// the file descriptor dirfd. If name is anything but a directory (this
// includes a symlink to one), it should return an error. Other than that this
// should act like openFileNolog.
//
// This acts like openFileNolog rather than OpenFile because
// we are going to (try to) remove the file.
// The contents of this file are not relevant for test caching.
func openDirAt(dirfd sysfdType, name string) (*File, error) {
	fd, err := rootOpenDir(dirfd, name)
	if err != nil {
		return nil, err
	}
	return newDirFile(fd, name)
}

```

// === FILE: references!/go/src/os/removeall_noat.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (js && wasm) || plan9

package os

import (
	"io"
	"runtime"
	"syscall"
)

func removeAll(path string) error {
	if path == "" {
		// fail silently to retain compatibility with previous behavior
		// of RemoveAll. See issue 28830.
		return nil
	}

	// The rmdir system call permits removing "." on Plan 9,
	// so we don't permit it to remain consistent with the
	// "at" implementation of RemoveAll.
	if endsWithDot(path) {
		return &PathError{Op: "RemoveAll", Path: path, Err: syscall.EINVAL}
	}

	// Simple case: if Remove works, we're done.
	err := Remove(path)
	if err == nil || IsNotExist(err) {
		return nil
	}

	// Otherwise, is this a directory we need to recurse into?
	dir, serr := Lstat(path)
	if serr != nil {
		if serr, ok := serr.(*PathError); ok && (IsNotExist(serr.Err) || serr.Err == syscall.ENOTDIR) {
			return nil
		}
		return serr
	}
	if !dir.IsDir() {
		// Not a directory; return the error from Remove.
		return err
	}

	// Remove contents & return first error.
	err = nil
	for {
		fd, err := Open(path)
		if err != nil {
			if IsNotExist(err) {
				// Already deleted by someone else.
				return nil
			}
			return err
		}

		const reqSize = 1024
		var names []string
		var readErr error

		for {
			numErr := 0
			names, readErr = fd.Readdirnames(reqSize)

			for _, name := range names {
				err1 := RemoveAll(path + string(PathSeparator) + name)
				if err == nil {
					err = err1
				}
				if err1 != nil {
					numErr++
				}
			}

			// If we can delete any entry, break to start new iteration.
			// Otherwise, we discard current names, get next entries and try deleting them.
			if numErr != reqSize {
				break
			}
		}

		// Removing files from the directory may have caused
		// the OS to reshuffle it. Simply calling Readdirnames
		// again may skip some entries. The only reliable way
		// to avoid this is to close and re-open the
		// directory. See issue 20841.
		fd.Close()

		if readErr == io.EOF {
			break
		}
		// If Readdirnames returned an error, use it.
		if err == nil {
			err = readErr
		}
		if len(names) == 0 {
			break
		}

		// We don't want to re-open unnecessarily, so if we
		// got fewer than request names from Readdirnames, try
		// simply removing the directory now. If that
		// succeeds, we are done.
		if len(names) < reqSize {
			err1 := Remove(path)
			if err1 == nil || IsNotExist(err1) {
				return nil
			}

			if err != nil {
				// We got some error removing the
				// directory contents, and since we
				// read fewer names than we requested
				// there probably aren't more files to
				// remove. Don't loop around to read
				// the directory again. We'll probably
				// just get the same error.
				return err
			}
		}
	}

	// Remove directory.
	err1 := Remove(path)
	if err1 == nil || IsNotExist(err1) {
		return nil
	}
	if runtime.GOOS == "windows" && IsPermission(err1) {
		if fs, err := Stat(path); err == nil {
			if err = Chmod(path, FileMode(0200|int(fs.Mode()))); err == nil {
				err1 = Remove(path)
			}
		}
	}
	if err == nil {
		err = err1
	}
	return err
}

```

// === FILE: references!/go/src/os/removeall_unix.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || wasip1

package os

func newDirFile(fd int, name string) (*File, error) {
	// We use kindNoPoll because we know that this is a directory.
	return newFile(fd, name, kindNoPoll, false), nil
}

```

// === FILE: references!/go/src/os/removeall_windows.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package os

import "syscall"

func newDirFile(fd syscall.Handle, name string) (*File, error) {
	return newFile(fd, name, kindOpenFile, false), nil
}

```

// === FILE: references!/go/src/os/root.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"errors"
	"internal/bytealg"
	"internal/stringslite"
	"internal/testlog"
	"io/fs"
	"runtime"
	"slices"
	"time"
)

// OpenInRoot opens the file name in the directory dir.
// It is equivalent to OpenRoot(dir) followed by opening the file in the root.
//
// OpenInRoot returns an error if any component of the name
// references a location outside of dir.
//
// See [Root] for details and limitations.
func OpenInRoot(dir, name string) (*File, error) {
	r, err := OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return r.Open(name)
}

// Root may be used to only access files within a single directory tree.
//
// Methods on Root can only access files and directories beneath a root directory.
// If any component of a file name passed to a method of Root references a location
// outside the root, the method returns an error.
// File names may reference the directory itself (.).
//
// Methods on Root will follow symbolic links, but symbolic links may not
// reference a location outside the root.
// Symbolic links must not be absolute.
//
// Methods on Root do not prohibit traversal of filesystem boundaries,
// Linux bind mounts, /proc special files, or access to Unix device files.
//
// Methods on Root are safe to be used from multiple goroutines simultaneously.
//
// On most platforms, creating a Root opens a file descriptor or handle referencing
// the directory. If the directory is moved, methods on Root reference the original
// directory in its new location.
//
// Root's behavior differs on some platforms:
//
//   - When GOOS=windows, file names may not reference Windows reserved device names
//     such as NUL and COM1.
//   - On Unix, [Root.Chmod], [Root.Chown], and [Root.Chtimes] are vulnerable to a race condition.
//     If the target of the operation is changed from a regular file to a symlink
//     while the operation is in progress, the operation may be performed on the link
//     rather than the link target.
//   - When GOOS=js, Root is vulnerable to TOCTOU (time-of-check-time-of-use)
//     attacks in symlink validation, and cannot ensure that operations will not
//     escape the root.
//   - When GOOS=plan9 or GOOS=js, Root does not track directories across renames.
//     On these platforms, a Root references a directory name, not a file descriptor.
//   - WASI preview 1 (GOOS=wasip1) does not support [Root.Chmod].
type Root struct {
	root *root
}

const (
	// Maximum number of symbolic links we will follow when resolving a file in a root.
	// 8 is __POSIX_SYMLOOP_MAX (the minimum allowed value for SYMLOOP_MAX),
	// and a common limit.
	rootMaxSymlinks = 8
)

// OpenRoot opens the named directory.
// It follows symbolic links in the directory name.
// If there is an error, it will be of type [*PathError].
func OpenRoot(name string) (*Root, error) {
	testlog.Open(name)
	return openRootNolog(name)
}

// Name returns the name of the directory presented to OpenRoot.
//
// It is safe to call Name after [Close].
func (r *Root) Name() string {
	return r.root.Name()
}

// Close closes the Root.
// After Close is called, methods on Root return errors.
func (r *Root) Close() error {
	return r.root.Close()
}

// Open opens the named file in the root for reading.
// See [Open] for more details.
func (r *Root) Open(name string) (*File, error) {
	return r.OpenFile(name, O_RDONLY, 0)
}

// Create creates or truncates the named file in the root.
// See [Create] for more details.
func (r *Root) Create(name string) (*File, error) {
	return r.OpenFile(name, O_RDWR|O_CREATE|O_TRUNC, 0666)
}

// OpenFile opens the named file in the root.
// See [OpenFile] for more details.
//
// If perm contains bits other than the nine least-significant bits (0o777),
// OpenFile returns an error.
func (r *Root) OpenFile(name string, flag int, perm FileMode) (*File, error) {
	if perm&0o777 != perm {
		return nil, &PathError{Op: "openat", Path: name, Err: errors.New("unsupported file mode")}
	}
	r.logOpen(name)
	rf, err := rootOpenFileNolog(r, name, flag, perm)
	if err != nil {
		return nil, err
	}
	rf.appendMode = flag&O_APPEND != 0
	return rf, nil
}

// OpenRoot opens the named directory in the root.
// If there is an error, it will be of type [*PathError].
func (r *Root) OpenRoot(name string) (*Root, error) {
	r.logOpen(name)
	return openRootInRoot(r, name)
}

// Chmod changes the mode of the named file in the root to mode.
// See [Chmod] for more details.
func (r *Root) Chmod(name string, mode FileMode) error {
	return rootChmod(r, name, mode)
}

// Mkdir creates a new directory in the root
// with the specified name and permission bits (before umask).
// See [Mkdir] for more details.
//
// If perm contains bits other than the nine least-significant bits (0o777),
// Mkdir returns an error.
func (r *Root) Mkdir(name string, perm FileMode) error {
	if perm&0o777 != perm {
		return &PathError{Op: "mkdirat", Path: name, Err: errors.New("unsupported file mode")}
	}
	return rootMkdir(r, name, perm)
}

// MkdirAll creates a new directory in the root, along with any necessary parents.
// See [MkdirAll] for more details.
//
// If perm contains bits other than the nine least-significant bits (0o777),
// MkdirAll returns an error.
func (r *Root) MkdirAll(name string, perm FileMode) error {
	if perm&0o777 != perm {
		return &PathError{Op: "mkdirat", Path: name, Err: errors.New("unsupported file mode")}
	}
	return rootMkdirAll(r, name, perm)
}

// Chown changes the numeric uid and gid of the named file in the root.
// See [Chown] for more details.
func (r *Root) Chown(name string, uid, gid int) error {
	return rootChown(r, name, uid, gid)
}

// Lchown changes the numeric uid and gid of the named file in the root.
// See [Lchown] for more details.
func (r *Root) Lchown(name string, uid, gid int) error {
	return rootLchown(r, name, uid, gid)
}

// Chtimes changes the access and modification times of the named file in the root.
// See [Chtimes] for more details.
func (r *Root) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return rootChtimes(r, name, atime, mtime)
}

// Remove removes the named file or (empty) directory in the root.
// See [Remove] for more details.
func (r *Root) Remove(name string) error {
	return rootRemove(r, name)
}

// RemoveAll removes the named file or directory and any children that it contains.
// See [RemoveAll] for more details.
func (r *Root) RemoveAll(name string) error {
	return rootRemoveAll(r, name)
}

// Stat returns a [FileInfo] describing the named file in the root.
// See [Stat] for more details.
func (r *Root) Stat(name string) (FileInfo, error) {
	r.logStat(name)
	return rootStat(r, name, false)
}

// Lstat returns a [FileInfo] describing the named file in the root.
// If the file is a symbolic link, the returned FileInfo
// describes the symbolic link.
// See [Lstat] for more details.
func (r *Root) Lstat(name string) (FileInfo, error) {
	r.logStat(name)
	return rootStat(r, name, true)
}

// Readlink returns the destination of the named symbolic link in the root.
// See [Readlink] for more details.
func (r *Root) Readlink(name string) (string, error) {
	return rootReadlink(r, name)
}

// Rename renames (moves) oldname to newname.
// Both paths are relative to the root.
// See [Rename] for more details.
func (r *Root) Rename(oldname, newname string) error {
	return rootRename(r, oldname, newname)
}

// Link creates newname as a hard link to the oldname file.
// Both paths are relative to the root.
// See [Link] for more details.
//
// If oldname is a symbolic link, Link creates new link to oldname and not its target.
// This behavior may differ from that of [Link] on some platforms.
//
// When GOOS=js, Link returns an error if oldname is a symbolic link.
func (r *Root) Link(oldname, newname string) error {
	return rootLink(r, oldname, newname)
}

// Symlink creates newname as a symbolic link to oldname.
// See [Symlink] for more details.
//
// Symlink does not validate oldname,
// which may reference a location outside the root.
//
// On Windows, a directory link is created if oldname references
// a directory within the root. Otherwise a file link is created.
func (r *Root) Symlink(oldname, newname string) error {
	return rootSymlink(r, oldname, newname)
}

// ReadFile reads the named file in the root and returns its contents.
// See [ReadFile] for more details.
func (r *Root) ReadFile(name string) ([]byte, error) {
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readFileContents(statOrZero(f), f.Read)
}

// WriteFile writes data to the named file in the root, creating it if necessary.
// See [WriteFile] for more details.
func (r *Root) WriteFile(name string, data []byte, perm FileMode) error {
	f, err := r.OpenFile(name, O_WRONLY|O_CREATE|O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}

func (r *Root) logOpen(name string) {
	if log := testlog.Logger(); log != nil {
		// This won't be right if r's name has changed since it was opened,
		// but it's the best we can do.
		log.Open(joinPath(r.Name(), name))
	}
}

func (r *Root) logStat(name string) {
	if log := testlog.Logger(); log != nil {
		// This won't be right if r's name has changed since it was opened,
		// but it's the best we can do.
		log.Stat(joinPath(r.Name(), name))
	}
}

// splitPathInRoot splits a path into components
// and joins it with the given prefix and suffix.
//
// The path is relative to a Root, and must not be
// absolute, volume-relative, or "".
//
// "." components are removed, except in the last component.
//
// Path separators following the last component are returned in suffixSep.
func splitPathInRoot(s string, prefix, suffix []string) (_ []string, suffixSep string, err error) {
	if len(s) == 0 {
		return nil, "", errors.New("empty path")
	}
	if IsPathSeparator(s[0]) {
		return nil, "", errPathEscapes
	}

	if runtime.GOOS == "windows" {
		// Windows cleans paths before opening them.
		s, err = rootCleanPath(s, prefix, suffix)
		if err != nil {
			return nil, "", err
		}
		prefix = nil
		suffix = nil
	}

	parts := slices.Clone(prefix)
	i, j := 0, 1
	for {
		if j < len(s) && !IsPathSeparator(s[j]) {
			// Keep looking for the end of this component.
			j++
			continue
		}
		parts = append(parts, s[i:j])
		// Advance to the next component, or end of the path.
		partEnd := j
		for j < len(s) && IsPathSeparator(s[j]) {
			j++
		}
		if j == len(s) {
			// If this is the last path component,
			// preserve any trailing path separators.
			suffixSep = s[partEnd:]
			break
		}
		if parts[len(parts)-1] == "." {
			// Remove "." components, except at the end.
			parts = parts[:len(parts)-1]
		}
		i = j
	}
	if len(suffix) > 0 && len(parts) > 0 && parts[len(parts)-1] == "." {
		// Remove a trailing "." component if we're joining to a suffix.
		parts = parts[:len(parts)-1]
	}
	parts = append(parts, suffix...)
	return parts, suffixSep, nil
}

// FS returns a file system (an fs.FS) for the tree of files in the root.
//
// The result implements [io/fs.StatFS], [io/fs.ReadFileFS],
// [io/fs.ReadDirFS], and [io/fs.ReadLinkFS].
func (r *Root) FS() fs.FS {
	return (*rootFS)(r)
}

type rootFS Root

func (rfs *rootFS) Open(name string) (fs.File, error) {
	r := (*Root)(rfs)
	if !isValidRootFSPath(name) {
		return nil, &PathError{Op: "open", Path: name, Err: ErrInvalid}
	}
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (rfs *rootFS) ReadDir(name string) ([]DirEntry, error) {
	r := (*Root)(rfs)
	if !isValidRootFSPath(name) {
		return nil, &PathError{Op: "readdir", Path: name, Err: ErrInvalid}
	}

	// This isn't efficient: We just open a regular file and ReadDir it.
	// Ideally, we would skip creating a *File entirely and operate directly
	// on the file descriptor, but that will require some extensive reworking
	// of directory reading in general.
	//
	// This suffices for the moment.
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dirs, err := f.ReadDir(-1)
	slices.SortFunc(dirs, func(a, b DirEntry) int {
		return bytealg.CompareString(a.Name(), b.Name())
	})
	return dirs, err
}

func (rfs *rootFS) ReadFile(name string) ([]byte, error) {
	r := (*Root)(rfs)
	if !isValidRootFSPath(name) {
		return nil, &PathError{Op: "readfile", Path: name, Err: ErrInvalid}
	}
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readFileContents(statOrZero(f), f.Read)
}

func (rfs *rootFS) ReadLink(name string) (string, error) {
	r := (*Root)(rfs)
	if !isValidRootFSPath(name) {
		return "", &PathError{Op: "readlink", Path: name, Err: ErrInvalid}
	}
	return r.Readlink(name)
}

func (rfs *rootFS) Stat(name string) (FileInfo, error) {
	r := (*Root)(rfs)
	if !isValidRootFSPath(name) {
		return nil, &PathError{Op: "stat", Path: name, Err: ErrInvalid}
	}
	return r.Stat(name)
}

func (rfs *rootFS) Lstat(name string) (FileInfo, error) {
	r := (*Root)(rfs)
	if !isValidRootFSPath(name) {
		return nil, &PathError{Op: "lstat", Path: name, Err: ErrInvalid}
	}
	return r.Lstat(name)
}

// isValidRootFSPath reports whether name is a valid filename to pass a Root.FS method.
func isValidRootFSPath(name string) bool {
	if !fs.ValidPath(name) {
		return false
	}
	if runtime.GOOS == "windows" {
		// fs.FS paths are /-separated.
		// On Windows, reject the path if it contains any \ separators.
		// Other forms of invalid path (for example, "NUL") are handled by
		// Root's usual file lookup mechanisms.
		if stringslite.IndexByte(name, '\\') >= 0 {
			return false
		}
	}
	return true
}

```

// === FILE: references!/go/src/os/root_js.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build js && wasm

package os

import (
	"errors"
	"slices"
	"syscall"
)

// checkPathEscapes reports whether name escapes the root.
//
// Due to the lack of openat, checkPathEscapes is subject to TOCTOU races
// when symlinks change during the resolution process.
func checkPathEscapes(r *Root, name string) error {
	return checkPathEscapesInternal(r, name, false)
}

// checkPathEscapesLstat reports whether name escapes the root.
// It does not resolve symlinks in the final path component.
//
// Due to the lack of openat, checkPathEscapes is subject to TOCTOU races
// when symlinks change during the resolution process.
func checkPathEscapesLstat(r *Root, name string) error {
	return checkPathEscapesInternal(r, name, true)
}

func checkPathEscapesInternal(r *Root, name string, lstat bool) error {
	if r.root.closed.Load() {
		return ErrClosed
	}
	parts, suffixSep, err := splitPathInRoot(name, nil, nil)
	if err != nil {
		return err
	}

	i := 0
	symlinks := 0
	base := r.root.name
	for i < len(parts) {
		if parts[i] == ".." {
			// Resolve one or more parent ("..") path components.
			end := i + 1
			for end < len(parts) && parts[end] == ".." {
				end++
			}
			count := end - i
			if count > i {
				return errPathEscapes
			}
			parts = slices.Delete(parts, i-count, end)
			i -= count
			base = r.root.name
			for j := range i {
				base = joinPath(base, parts[j])
			}
			continue
		}

		part := parts[i]
		if i == len(parts)-1 {
			if lstat {
				break
			}
			part += suffixSep
		}

		next := joinPath(base, part)
		fi, err := Lstat(next)
		if err != nil {
			if IsNotExist(err) {
				return nil
			}
			return underlyingError(err)
		}
		if fi.Mode()&ModeSymlink != 0 {
			link, err := Readlink(next)
			if err != nil {
				return errPathEscapes
			}
			symlinks++
			if symlinks > rootMaxSymlinks {
				return errors.New("too many symlinks")
			}
			newparts, newSuffixSep, err := splitPathInRoot(link, parts[:i], parts[i+1:])
			if err != nil {
				return err
			}
			if i == len(parts) {
				// suffixSep contains any trailing path separator characters
				// in the link target.
				// If we are replacing the remainder of the path, retain these.
				// If we're replacing some intermediate component of the path,
				// ignore them, since intermediate components must always be
				// directories.
				suffixSep = newSuffixSep
			}
			parts = newparts
			continue
		}
		if !fi.IsDir() && i < len(parts)-1 {
			return syscall.ENOTDIR
		}

		base = next
		i++
	}
	return nil
}

```

// === FILE: references!/go/src/os/root_nonwindows.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows

package os

func rootCleanPath(s string, prefix, suffix []string) (string, error) {
	return s, nil
}

```

// === FILE: references!/go/src/os/root_noopenat.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (js && wasm) || plan9

package os

import (
	"errors"
	"internal/filepathlite"
	"internal/stringslite"
	"sync/atomic"
	"syscall"
	"time"
)

// root implementation for platforms with no openat.
// Currently plan9 and js.
type root struct {
	name   string
	closed atomic.Bool
}

// openRootNolog is OpenRoot.
func openRootNolog(name string) (*Root, error) {
	r, err := newRoot(name)
	if err != nil {
		return nil, &PathError{Op: "open", Path: name, Err: err}
	}
	return r, nil
}

// openRootInRoot is Root.OpenRoot.
func openRootInRoot(r *Root, name string) (*Root, error) {
	if err := checkPathEscapes(r, name); err != nil {
		return nil, &PathError{Op: "openat", Path: name, Err: err}
	}
	r, err := newRoot(joinPath(r.root.name, name))
	if err != nil {
		return nil, &PathError{Op: "openat", Path: name, Err: err}
	}
	return r, nil
}

// newRoot returns a new Root.
// If fd is not a directory, it closes it and returns an error.
func newRoot(name string) (*Root, error) {
	fi, err := Stat(name)
	if err != nil {
		return nil, err.(*PathError).Err
	}
	if !fi.IsDir() {
		return nil, errors.New("not a directory")
	}
	return &Root{&root{name: name}}, nil
}

func (r *root) Close() error {
	// For consistency with platforms where Root.Close closes a handle,
	// mark the Root as closed and return errors from future calls.
	r.closed.Store(true)
	return nil
}

func (r *root) Name() string {
	return r.name
}

// rootOpenFileNolog is Root.OpenFile.
func rootOpenFileNolog(r *Root, name string, flag int, perm FileMode) (*File, error) {
	if err := checkPathEscapes(r, name); err != nil {
		return nil, &PathError{Op: "openat", Path: name, Err: err}
	}
	f, err := openFileNolog(joinPath(r.root.name, name), flag, perm)
	if err != nil {
		return nil, &PathError{Op: "openat", Path: name, Err: underlyingError(err)}
	}
	return f, nil
}

func rootStat(r *Root, name string, lstat bool) (FileInfo, error) {
	var fi FileInfo
	var err error
	if lstat {
		err = checkPathEscapesLstat(r, name)
		if err == nil {
			fi, err = Lstat(joinPath(r.root.name, name))
		}
	} else {
		err = checkPathEscapes(r, name)
		if err == nil {
			fi, err = Stat(joinPath(r.root.name, name))
		}
	}
	if err != nil {
		return nil, &PathError{Op: "statat", Path: name, Err: underlyingError(err)}
	}
	return fi, nil
}

func rootChmod(r *Root, name string, mode FileMode) error {
	if err := checkPathEscapes(r, name); err != nil {
		return &PathError{Op: "chmodat", Path: name, Err: err}
	}
	if err := Chmod(joinPath(r.root.name, name), mode); err != nil {
		return &PathError{Op: "chmodat", Path: name, Err: underlyingError(err)}
	}
	return nil
}

func rootChown(r *Root, name string, uid, gid int) error {
	if err := checkPathEscapes(r, name); err != nil {
		return &PathError{Op: "chownat", Path: name, Err: err}
	}
	if err := Chown(joinPath(r.root.name, name), uid, gid); err != nil {
		return &PathError{Op: "chownat", Path: name, Err: underlyingError(err)}
	}
	return nil
}

func rootLchown(r *Root, name string, uid, gid int) error {
	if err := checkPathEscapesLstat(r, name); err != nil {
		return &PathError{Op: "lchownat", Path: name, Err: err}
	}
	if err := Lchown(joinPath(r.root.name, name), uid, gid); err != nil {
		return &PathError{Op: "lchownat", Path: name, Err: underlyingError(err)}
	}
	return nil
}

func rootChtimes(r *Root, name string, atime time.Time, mtime time.Time) error {
	if err := checkPathEscapes(r, name); err != nil {
		return &PathError{Op: "chtimesat", Path: name, Err: err}
	}
	if err := Chtimes(joinPath(r.root.name, name), atime, mtime); err != nil {
		return &PathError{Op: "chtimesat", Path: name, Err: underlyingError(err)}
	}
	return nil
}

func rootMkdir(r *Root, name string, perm FileMode) error {
	if err := checkPathEscapes(r, name); err != nil {
		return &PathError{Op: "mkdirat", Path: name, Err: err}
	}
	if err := Mkdir(joinPath(r.root.name, name), perm); err != nil {
		return &PathError{Op: "mkdirat", Path: name, Err: underlyingError(err)}
	}
	return nil
}

func rootMkdirAll(r *Root, name string, perm FileMode) error {
	// We only check for errPathEscapes here.
	// For errors such as ENOTDIR (a non-directory file appeared somewhere along the path),
	// we let MkdirAll generate the error.
	// MkdirAll will return a PathError referencing the exact location of the error,
	// and we want to preserve that property.
	if err := checkPathEscapes(r, name); err == errPathEscapes {
		return &PathError{Op: "mkdirat", Path: name, Err: err}
	}
	prefix := r.root.name + string(PathSeparator)
	if err := MkdirAll(prefix+name, perm); err != nil {
		if pe, ok := err.(*PathError); ok {
			pe.Op = "mkdirat"
			pe.Path = stringslite.TrimPrefix(pe.Path, prefix)
			return pe
		}
		return &PathError{Op: "mkdirat", Path: name, Err: underlyingError(err)}
	}
	return nil
}

func rootRemove(r *Root, name string) error {
	if err := checkPathEscapesLstat(r, name); err != nil {
		return &PathError{Op: "removeat", Path: name, Err: err}
	}
	if endsWithDot(name) {
		// We don't want to permit removing the root itself, so check for that.
		if filepathlite.Clean(name) == "." {
			return &PathError{Op: "removeat", Path: name, Err: errPathEscapes}
		}
	}
	if err := Remove(joinPath(r.root.name, name)); err != nil {
		return &PathError{Op: "removeat", Path: name, Err: underlyingError(err)}
	}
	return nil
}

func rootRemoveAll(r *Root, name string) error {
	if endsWithDot(name) {
		// Consistency with os.RemoveAll: Return EINVAL when trying to remove .
		return &PathError{Op: "RemoveAll", Path: name, Err: syscall.EINVAL}
	}
	if err := checkPathEscapesLstat(r, name); err != nil {
		if err == syscall.ENOTDIR {
			// Some intermediate path component is not a directory.
			// RemoveAll treats this as success (since the target doesn't exist).
			return nil
		}
		return &PathError{Op: "RemoveAll", Path: name, Err: err}
	}
	if err := RemoveAll(joinPath(r.root.name, name)); err != nil {
		return &PathError{Op: "RemoveAll", Path: name, Err: underlyingError(err)}
	}
	return nil
}

func rootReadlink(r *Root, name string) (string, error) {
	if err := checkPathEscapesLstat(r, name); err != nil {
		return "", &PathError{Op: "readlinkat", Path: name, Err: err}
	}
	name, err := Readlink(joinPath(r.root.name, name))
	if err != nil {
		return "", &PathError{Op: "readlinkat", Path: name, Err: underlyingError(err)}
	}
	return name, nil
}

func rootRename(r *Root, oldname, newname string) error {
	if err := checkPathEscapesLstat(r, oldname); err != nil {
		return &PathError{Op: "renameat", Path: oldname, Err: err}
	}
	if err := checkPathEscapesLstat(r, newname); err != nil {
		return &PathError{Op: "renameat", Path: newname, Err: err}
	}
	err := Rename(joinPath(r.root.name, oldname), joinPath(r.root.name, newname))
	if err != nil {
		return &LinkError{"renameat", oldname, newname, underlyingError(err)}
	}
	return nil
}

func rootLink(r *Root, oldname, newname string) error {
	if err := checkPathEscapesLstat(r, oldname); err != nil {
		return &PathError{Op: "linkat", Path: oldname, Err: err}
	}
	fullOldName := joinPath(r.root.name, oldname)
	if fs, err := Lstat(fullOldName); err == nil && fs.Mode()&ModeSymlink != 0 {
		return &PathError{Op: "linkat", Path: oldname, Err: errors.New("cannot create a hard link to a symlink")}
	}
	if err := checkPathEscapesLstat(r, newname); err != nil {
		return &PathError{Op: "linkat", Path: newname, Err: err}
	}
	err := Link(fullOldName, joinPath(r.root.name, newname))
	if err != nil {
		return &LinkError{"linkat", oldname, newname, underlyingError(err)}
	}
	return nil
}

func rootSymlink(r *Root, oldname, newname string) error {
	if err := checkPathEscapesLstat(r, newname); err != nil {
		return &PathError{Op: "symlinkat", Path: newname, Err: err}
	}
	err := Symlink(oldname, joinPath(r.root.name, newname))
	if err != nil {
		return &LinkError{"symlinkat", oldname, newname, underlyingError(err)}
	}
	return nil
}

```

// === FILE: references!/go/src/os/root_openat.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || windows || wasip1

package os

import (
	"runtime"
	"slices"
	"sync"
	"syscall"
	"time"
)

// root implementation for platforms with a function to open a file
// relative to a directory.
type root struct {
	name string

	// refs is incremented while an operation is using fd.
	// closed is set when Close is called.
	// fd is closed when closed is true and refs is 0.
	mu     sync.Mutex
	fd     sysfdType
	refs   int  // number of active operations
	closed bool // set when closed
}

func (r *root) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed && r.refs == 0 {
		syscall.Close(r.fd)
	}
	r.closed = true
	runtime.SetFinalizer(r, nil) // no need for a finalizer any more
	return nil
}

func (r *root) incref() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrClosed
	}
	r.refs++
	return nil
}

func (r *root) decref() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.refs <= 0 {
		panic("bad Root refcount")
	}
	r.refs--
	if r.closed && r.refs == 0 {
		syscall.Close(r.fd)
	}
}

func (r *root) Name() string {
	return r.name
}

func rootChmod(r *Root, name string, mode FileMode) error {
	_, err := doInRoot(r, name, nil, func(parent sysfdType, name string) (struct{}, error) {
		return struct{}{}, chmodat(parent, name, mode)
	})
	if err != nil {
		return &PathError{Op: "chmodat", Path: name, Err: err}
	}
	return nil
}

func rootChown(r *Root, name string, uid, gid int) error {
	_, err := doInRoot(r, name, nil, func(parent sysfdType, name string) (struct{}, error) {
		return struct{}{}, chownat(parent, name, uid, gid)
	})
	if err != nil {
		return &PathError{Op: "chownat", Path: name, Err: err}
	}
	return nil
}

func rootLchown(r *Root, name string, uid, gid int) error {
	_, err := doInRoot(r, name, nil, func(parent sysfdType, name string) (struct{}, error) {
		return struct{}{}, lchownat(parent, name, uid, gid)
	})
	if err != nil {
		return &PathError{Op: "lchownat", Path: name, Err: err}
	}
	return err
}

func rootChtimes(r *Root, name string, atime time.Time, mtime time.Time) error {
	_, err := doInRoot(r, name, nil, func(parent sysfdType, name string) (struct{}, error) {
		return struct{}{}, chtimesat(parent, name, atime, mtime)
	})
	if err != nil {
		return &PathError{Op: "chtimesat", Path: name, Err: err}
	}
	return err
}

func rootMkdir(r *Root, name string, perm FileMode) error {
	_, err := doInRoot(r, name, nil, func(parent sysfdType, name string) (struct{}, error) {
		return struct{}{}, mkdirat(parent, name, perm)
	})
	if err != nil {
		return &PathError{Op: "mkdirat", Path: name, Err: err}
	}
	return nil
}

func rootMkdirAll(r *Root, fullname string, perm FileMode) error {
	// doInRoot opens each path element in turn.
	//
	// openDirFunc opens all but the last path component.
	// The usual default openDirFunc just opens directories with O_DIRECTORY.
	// We replace it here with one that creates missing directories along the way.
	openDirFunc := func(parent sysfdType, name string) (sysfdType, error) {
		for try := range 2 {
			fd, err := rootOpenDir(parent, name)
			switch err.(type) {
			case nil, errSymlink:
				return fd, err
			}
			if try > 0 || !IsNotExist(err) {
				return 0, &PathError{Op: "openat", Err: err}
			}
			// Try again on EEXIST, because the directory may have been created
			// by another process or thread between the rootOpenDir and mkdirat calls.
			if err := mkdirat(parent, name, perm); err != nil && err != syscall.EEXIST {
				return 0, &PathError{Op: "mkdirat", Err: err}
			}
		}
		panic("unreachable")
	}
	// openLastComponentFunc opens the last path component.
	openLastComponentFunc := func(parent sysfdType, name string) (struct{}, error) {
		err := mkdirat(parent, name, perm)
		if err == syscall.EEXIST {
			mode, e := modeAt(parent, name)
			if e == nil {
				if mode.IsDir() {
					// The target of MkdirAll is an existing directory.
					err = nil
				} else if mode&ModeSymlink != 0 {
					// The target of MkdirAll is a symlink.
					// For consistency with os.MkdirAll,
					// succeed if the link resolves to a directory.
					// We don't return errSymlink here, because we don't
					// want to create the link target if it doesn't exist.
					fi, e := r.Stat(fullname)
					if e == nil && fi.Mode().IsDir() {
						err = nil
					}
				}
			}
		}
		switch err.(type) {
		case nil, errSymlink:
			return struct{}{}, err
		}
		return struct{}{}, &PathError{Op: "mkdirat", Err: err}
	}
	_, err := doInRoot(r, fullname, openDirFunc, openLastComponentFunc)
	if err != nil {
		if _, ok := err.(*PathError); !ok {
			err = &PathError{Op: "mkdirat", Path: fullname, Err: err}
		}
	}
	return err
}

func rootReadlink(r *Root, name string) (string, error) {
	target, err := doInRoot(r, name, nil, func(parent sysfdType, name string) (string, error) {
		return readlinkat(parent, name)
	})
	if err != nil {
		return "", &PathError{Op: "readlinkat", Path: name, Err: err}
	}
	return target, nil
}

func rootRemove(r *Root, name string) error {
	_, err := doInRoot(r, name, nil, func(parent sysfdType, name string) (struct{}, error) {
		return struct{}{}, removeat(parent, name)
	})
	if err != nil {
		return &PathError{Op: "removeat", Path: name, Err: err}
	}
	return nil
}

func rootRemoveAll(r *Root, name string) error {
	// Consistency with os.RemoveAll: Strip trailing /s from the name,
	// so RemoveAll("not_a_directory/") succeeds.
	for len(name) > 0 && IsPathSeparator(name[len(name)-1]) {
		name = name[:len(name)-1]
	}
	if endsWithDot(name) {
		// Consistency with os.RemoveAll: Return EINVAL when trying to remove .
		return &PathError{Op: "RemoveAll", Path: name, Err: syscall.EINVAL}
	}
	_, err := doInRoot(r, name, nil, func(parent sysfdType, name string) (struct{}, error) {
		return struct{}{}, removeAllFrom(parent, name)
	})
	if IsNotExist(err) {
		return nil
	}
	if err != nil {
		return &PathError{Op: "RemoveAll", Path: name, Err: underlyingError(err)}
	}
	return err
}

func rootRename(r *Root, oldname, newname string) error {
	_, err := doInRoot(r, oldname, nil, func(oldparent sysfdType, oldname string) (struct{}, error) {
		_, err := doInRoot(r, newname, nil, func(newparent sysfdType, newname string) (struct{}, error) {
			return struct{}{}, renameat(oldparent, oldname, newparent, newname)
		})
		return struct{}{}, err
	})
	if err != nil {
		return &LinkError{"renameat", oldname, newname, err}
	}
	return err
}

func rootLink(r *Root, oldname, newname string) error {
	_, err := doInRoot(r, oldname, nil, func(oldparent sysfdType, oldname string) (struct{}, error) {
		_, err := doInRoot(r, newname, nil, func(newparent sysfdType, newname string) (struct{}, error) {
			return struct{}{}, linkat(oldparent, oldname, newparent, newname)
		})
		return struct{}{}, err
	})
	if err != nil {
		return &LinkError{"linkat", oldname, newname, err}
	}
	return err
}

// doInRoot performs an operation on a path in a Root.
//
// It calls f with the FD or handle for the directory containing the last
// path element, and the name of the last path element.
//
// For example, given the path a/b/c it calls f with the FD for a/b and the name "c".
//
// If openDirFunc is non-nil, it is called to open intermediate path elements.
// For example, given the path a/b/c openDirFunc will be called to open a and a/b in turn.
//
// f or openDirFunc may return errSymlink to indicate that the path element is a symlink
// which should be followed. Note that this can result in f being called multiple times
// with different names. For example, give the path "link" which is a symlink to "target",
// f is called with the path "link", returns errSymlink("target"), and is called again with
// the path "target".
//
// If f or openDirFunc return a *PathError, doInRoot will set PathError.Path to the
// full path which caused the error.
func doInRoot[T any](r *Root, name string, openDirFunc func(parent sysfdType, name string) (sysfdType, error), f func(parent sysfdType, name string) (T, error)) (ret T, err error) {
	if err := r.root.incref(); err != nil {
		return ret, err
	}
	defer r.root.decref()

	parts, suffixSep, err := splitPathInRoot(name, nil, nil)
	if err != nil {
		return ret, err
	}
	if openDirFunc == nil {
		openDirFunc = rootOpenDir
	}

	rootfd := r.root.fd
	dirfd := rootfd
	defer func() {
		if dirfd != rootfd {
			syscall.Close(dirfd)
		}
	}()

	// When resolving .. path components, we restart path resolution from the root.
	// (We can't openat(dir, "..") to move up to the parent directory,
	// because dir may have moved since we opened it.)
	// To limit how many opens a malicious path can cause us to perform, we set
	// a limit on the total number of path steps and the total number of restarts
	// caused by .. components. If *both* limits are exceeded, we halt the operation.
	const maxSteps = 255
	const maxRestarts = 8

	i := 0
	steps := 0
	restarts := 0
	symlinks := 0
Loop:
	for {
		steps++
		if steps > maxSteps && restarts > maxRestarts {
			return ret, syscall.ENAMETOOLONG
		}

		if parts[i] == ".." {
			// Resolve one or more parent ("..") path components.
			//
			// Rewrite the original path,
			// removing the elements eliminated by ".." components,
			// and start over from the beginning.
			restarts++
			end := i + 1
			for end < len(parts) && parts[end] == ".." {
				end++
			}
			count := end - i
			if count > i {
				return ret, errPathEscapes
			}
			parts = slices.Delete(parts, i-count, end)
			if len(parts) == 0 {
				parts = []string{"."}
			}
			i = 0
			if dirfd != rootfd {
				syscall.Close(dirfd)
			}
			dirfd = rootfd
			continue
		}

		if i == len(parts)-1 {
			// This is the last path element.
			// Call f to decide what to do with it.
			// If f returns errSymlink, this element is a symlink
			// which should be followed.
			// suffixSep contains any trailing separator characters
			// which we rejoin to the final part at this time.
			ret, err = f(dirfd, parts[i]+suffixSep)
			if err == nil {
				return
			}
		} else {
			var fd sysfdType
			fd, err = openDirFunc(dirfd, parts[i])
			if err == nil {
				if dirfd != rootfd {
					syscall.Close(dirfd)
				}
				dirfd = fd
			}
		}

		switch e := err.(type) {
		case nil:
		case errSymlink:
			symlinks++
			if symlinks > rootMaxSymlinks {
				return ret, syscall.ELOOP
			}
			newparts, newSuffixSep, err := splitPathInRoot(string(e), parts[:i], parts[i+1:])
			if err != nil {
				return ret, err
			}
			if i == len(parts)-1 {
				// suffixSep contains any trailing path separator characters
				// in the link target.
				// If we are replacing the remainder of the path, retain these.
				// If we're replacing some intermediate component of the path,
				// ignore them, since intermediate components must always be
				// directories.
				suffixSep = newSuffixSep
			}
			if len(newparts) < i || !slices.Equal(parts[:i], newparts[:i]) {
				// Some component in the path which we have already traversed
				// has changed. We need to restart parsing from the root.
				i = 0
				if dirfd != rootfd {
					syscall.Close(dirfd)
				}
				dirfd = rootfd
			}
			parts = newparts
			continue Loop
		case *PathError:
			// This is strings.Join(parts[:i+1], PathSeparator).
			e.Path = parts[0]
			for _, part := range parts[1 : i+1] {
				e.Path += string(PathSeparator) + part
			}
			return ret, e
		default:
			return ret, err
		}

		i++
	}
}

// errSymlink reports that a file being operated on is actually a symlink,
// and the target of that symlink.
type errSymlink string

func (errSymlink) Error() string { panic("errSymlink is not user-visible") }

```

// === FILE: references!/go/src/os/root_plan9.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build plan9

package os

import (
	"internal/filepathlite"
)

func checkPathEscapes(r *Root, name string) error {
	if r.root.closed.Load() {
		return ErrClosed
	}
	if !filepathlite.IsLocal(name) {
		return errPathEscapes
	}
	return nil
}

func checkPathEscapesLstat(r *Root, name string) error {
	return checkPathEscapes(r, name)
}

```

// === FILE: references!/go/src/os/root_unix.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || wasip1

package os

import (
	"errors"
	"internal/syscall/unix"
	"runtime"
	"syscall"
	"time"
)

// sysfdType is the native type of a file handle
// (int on Unix, syscall.Handle on Windows),
// permitting helper functions to be written portably.
type sysfdType = int

// openRootNolog is OpenRoot.
func openRootNolog(name string) (*Root, error) {
	var fd int
	err := ignoringEINTR(func() error {
		var err error
		fd, _, err = open(name, syscall.O_CLOEXEC, 0)
		return err
	})
	if err != nil {
		return nil, &PathError{Op: "open", Path: name, Err: err}
	}
	return newRoot(fd, name)
}

// newRoot returns a new Root.
// If fd is not a directory, it closes it and returns an error.
func newRoot(fd int, name string) (*Root, error) {
	var fs fileStat
	err := ignoringEINTR(func() error {
		return syscall.Fstat(fd, &fs.sys)
	})
	fillFileStatFromSys(&fs, name)
	if err == nil && !fs.IsDir() {
		syscall.Close(fd)
		return nil, &PathError{Op: "open", Path: name, Err: errors.New("not a directory")}
	}

	// There's a race here with fork/exec, which we are
	// content to live with. See ../syscall/exec_unix.go.
	if !supportsCloseOnExec {
		syscall.CloseOnExec(fd)
	}

	r := &Root{&root{
		fd:   fd,
		name: name,
	}}
	runtime.SetFinalizer(r.root, (*root).Close)
	return r, nil
}

// openRootInRoot is Root.OpenRoot.
func openRootInRoot(r *Root, name string) (*Root, error) {
	fd, err := doInRoot(r, name, nil, func(parent int, name string) (fd int, err error) {
		ignoringEINTR(func() error {
			fd, err = unix.Openat(parent, name, syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
			if isNoFollowErr(err) {
				err = checkSymlink(parent, name, err)
			}
			return err
		})
		return fd, err
	})
	if err != nil {
		return nil, &PathError{Op: "openat", Path: name, Err: err}
	}
	return newRoot(fd, joinPath(r.Name(), name))
}

// rootOpenFileNolog is Root.OpenFile.
func rootOpenFileNolog(root *Root, name string, flag int, perm FileMode) (*File, error) {
	fd, err := doInRoot(root, name, nil, func(parent int, name string) (fd int, err error) {
		ignoringEINTR(func() error {
			fd, err = unix.Openat(parent, name, syscall.O_NOFOLLOW|syscall.O_CLOEXEC|flag, uint32(perm))
			if err != nil {
				// Never follow symlinks when O_CREATE|O_EXCL, no matter
				// what error the OS returns.
				isCreateExcl := flag&(O_CREATE|O_EXCL) == (O_CREATE | O_EXCL)
				if !isCreateExcl && (isNoFollowErr(err) || err == syscall.ENOTDIR) {
					err = checkSymlink(parent, name, err)
				}
				// AIX returns ELOOP instead of EEXIST for a dangling symlink.
				// Convert this to EEXIST so it matches ErrExists.
				if isCreateExcl && err == syscall.ELOOP {
					err = syscall.EEXIST
				}
			}
			return err
		})
		return fd, err
	})
	if err != nil {
		return nil, &PathError{Op: "openat", Path: name, Err: err}
	}
	f := newFile(fd, joinPath(root.Name(), name), kindOpenFile, unix.HasNonblockFlag(flag))
	f.inRoot = true
	return f, nil
}

func rootOpenDir(parent int, name string) (int, error) {
	var (
		fd  int
		err error
	)
	ignoringEINTR(func() error {
		fd, err = unix.Openat(parent, name, syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_DIRECTORY, 0)
		if isNoFollowErr(err) || err == syscall.ENOTDIR {
			err = checkSymlink(parent, name, err)
		} else if err == syscall.ENOTSUP || err == syscall.EOPNOTSUPP {
			// ENOTSUP and EOPNOTSUPP are often, but not always, the same errno.
			// Translate both to ENOTDIR, since this indicates a non-terminal
			// path component was not a directory.
			err = syscall.ENOTDIR
		}
		return err
	})
	return fd, err
}

func rootStat(r *Root, name string, lstat bool) (FileInfo, error) {
	fi, err := doInRoot(r, name, nil, func(parent sysfdType, n string) (FileInfo, error) {
		var fs fileStat
		if err := unix.Fstatat(parent, n, &fs.sys, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return nil, err
		}
		fillFileStatFromSys(&fs, name)
		if !lstat && fs.Mode()&ModeSymlink != 0 {
			return nil, checkSymlink(parent, n, syscall.ELOOP)
		}
		return &fs, nil
	})
	if err != nil {
		return nil, &PathError{Op: "statat", Path: name, Err: err}
	}
	return fi, nil
}

func rootSymlink(r *Root, oldname, newname string) error {
	_, err := doInRoot(r, newname, nil, func(parent sysfdType, name string) (struct{}, error) {
		return struct{}{}, symlinkat(oldname, parent, name)
	})
	if err != nil {
		return &LinkError{"symlinkat", oldname, newname, err}
	}
	return nil
}

// On systems which use fchmodat, fchownat, etc., we have a race condition:
// When "name" is a symlink, Root.Chmod("name") should act on the target of that link.
// However, fchmodat doesn't allow us to chmod a file only if it is not a symlink;
// the AT_SYMLINK_NOFOLLOW parameter causes the operation to act on the symlink itself.
//
// We do the best we can by first checking to see if the target of the operation is a symlink,
// and only attempting the fchmodat if it is not. If the target is replaced between the check
// and the fchmodat, we will chmod the symlink rather than following it.
//
// This race condition is unfortunate, but does not permit escaping a root:
// We may act on the wrong file, but that file will be contained within the root.
func afterResolvingSymlink(parent int, name string, f func() error) error {
	if err := checkSymlink(parent, name, nil); err != nil {
		return err
	}
	return f()
}

func chmodat(parent int, name string, mode FileMode) error {
	return afterResolvingSymlink(parent, name, func() error {
		return ignoringEINTR(func() error {
			return unix.Fchmodat(parent, name, syscallMode(mode), unix.AT_SYMLINK_NOFOLLOW)
		})
	})
}

func chownat(parent int, name string, uid, gid int) error {
	return afterResolvingSymlink(parent, name, func() error {
		return ignoringEINTR(func() error {
			return unix.Fchownat(parent, name, uid, gid, unix.AT_SYMLINK_NOFOLLOW)
		})
	})
}

func lchownat(parent int, name string, uid, gid int) error {
	return ignoringEINTR(func() error {
		return unix.Fchownat(parent, name, uid, gid, unix.AT_SYMLINK_NOFOLLOW)
	})
}

func chtimesat(parent int, name string, atime time.Time, mtime time.Time) error {
	return afterResolvingSymlink(parent, name, func() error {
		return ignoringEINTR(func() error {
			utimes := chtimesUtimes(atime, mtime)
			return unix.Utimensat(parent, name, &utimes, unix.AT_SYMLINK_NOFOLLOW)
		})
	})
}

func mkdirat(fd int, name string, perm FileMode) error {
	return ignoringEINTR(func() error {
		return unix.Mkdirat(fd, name, syscallMode(perm))
	})
}

func removeat(fd int, name string) error {
	// The system call interface forces us to know whether
	// we are removing a file or directory. Try both.
	e := ignoringEINTR(func() error {
		return unix.Unlinkat(fd, name, 0)
	})
	if e == nil {
		return nil
	}
	e1 := ignoringEINTR(func() error {
		return unix.Unlinkat(fd, name, unix.AT_REMOVEDIR)
	})
	if e1 == nil {
		return nil
	}
	// Both failed. See comment in Remove for how we decide which error to return.
	if e1 != syscall.ENOTDIR {
		return e1
	}
	return e
}

func removefileat(fd int, name string) error {
	return ignoringEINTR(func() error {
		return unix.Unlinkat(fd, name, 0)
	})
}

func removedirat(fd int, name string) error {
	return ignoringEINTR(func() error {
		return unix.Unlinkat(fd, name, unix.AT_REMOVEDIR)
	})
}

func renameat(oldfd int, oldname string, newfd int, newname string) error {
	return unix.Renameat(oldfd, oldname, newfd, newname)
}

func linkat(oldfd int, oldname string, newfd int, newname string) error {
	return unix.Linkat(oldfd, oldname, newfd, newname, 0)
}

func symlinkat(oldname string, newfd int, newname string) error {
	return unix.Symlinkat(oldname, newfd, newname)
}

func modeAt(parent int, name string) (FileMode, error) {
	var fs fileStat
	if err := unix.Fstatat(parent, name, &fs.sys, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return 0, err
	}
	fillFileStatFromSys(&fs, name)
	return fs.mode, nil
}

// checkSymlink resolves the symlink name in parent,
// and returns errSymlink with the link contents.
//
// If name is not a symlink, return origError.
func checkSymlink(parent int, name string, origError error) error {
	link, err := readlinkat(parent, name)
	if err != nil {
		return origError
	}
	return errSymlink(link)
}

func readlinkat(fd int, name string) (string, error) {
	for len := 128; ; len *= 2 {
		b := make([]byte, len)
		var (
			n int
			e error
		)
		ignoringEINTR(func() error {
			n, e = unix.Readlinkat(fd, name, b)
			return e
		})
		if e == syscall.ERANGE {
			continue
		}
		if e != nil {
			return "", e
		}
		n = max(n, 0)
		if n < len {
			return string(b[0:n]), nil
		}
	}
}

```

// === FILE: references!/go/src/os/root_windows.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package os

import (
	"errors"
	"internal/filepathlite"
	"internal/stringslite"
	"internal/syscall/windows"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// rootCleanPath uses GetFullPathName to perform lexical path cleaning.
//
// On Windows, file names are lexically cleaned at the start of a file operation.
// For example, on Windows the path `a\..\b` is exactly equivalent to `b` alone,
// even if `a` does not exist or is not a directory.
//
// We use the Windows API function GetFullPathName to perform this cleaning.
// We could do this ourselves, but there are a number of subtle behaviors here,
// and deferring to the OS maintains consistency.
// (For example, `a\.\` cleans to `a\`.)
//
// GetFullPathName operates on absolute paths, and our input path is relative.
// We make the path absolute by prepending a fixed prefix of \\?\?\.
//
// We want to detect paths which use .. components to escape the root.
// We do this by ensuring the cleaned path still begins with \\?\?\.
// We catch the corner case of a path which includes a ..\?\. component
// by rejecting any input paths which contain a ?, which is not a valid character
// in a Windows filename.
func rootCleanPath(s string, prefix, suffix []string) (string, error) {
	// Reject paths which include a ? component (see above).
	if stringslite.IndexByte(s, '?') >= 0 {
		return "", windows.ERROR_INVALID_NAME
	}

	const fixedPrefix = `\\?\?`
	buf := []byte(fixedPrefix)
	for _, p := range prefix {
		buf = append(buf, '\\')
		buf = append(buf, []byte(p)...)
	}
	buf = append(buf, '\\')
	buf = append(buf, []byte(s)...)
	for _, p := range suffix {
		buf = append(buf, '\\')
		buf = append(buf, []byte(p)...)
	}
	s = string(buf)

	s, err := syscall.FullPath(s)
	if err != nil {
		return "", err
	}

	s, ok := stringslite.CutPrefix(s, fixedPrefix)
	if !ok {
		return "", errPathEscapes
	}
	s = stringslite.TrimPrefix(s, `\`)
	if s == "" {
		s = "."
	}

	if !filepathlite.IsLocal(s) {
		return "", errPathEscapes
	}

	return s, nil
}

// sysfdType is the native type of a file handle
// (int on Unix, syscall.Handle on Windows),
// permitting helper functions to be written portably.
type sysfdType = syscall.Handle

// openRootNolog is OpenRoot.
func openRootNolog(name string) (*Root, error) {
	if name == "" {
		return nil, &PathError{Op: "open", Path: name, Err: syscall.ENOENT}
	}
	path := fixLongPath(name)
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, &PathError{Op: "open", Path: name, Err: err}
	}
	return newRoot(fd, name)
}

// newRoot returns a new Root.
// If fd is not a directory, it closes it and returns an error.
func newRoot(fd syscall.Handle, name string) (*Root, error) {
	// Check that this is a directory.
	//
	// If we get any errors here, ignore them; worst case we create a Root
	// which returns errors when you try to use it.
	var fi syscall.ByHandleFileInformation
	err := syscall.GetFileInformationByHandle(fd, &fi)
	if err == nil && fi.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 {
		syscall.CloseHandle(fd)
		return nil, &PathError{Op: "open", Path: name, Err: errors.New("not a directory")}
	}

	r := &Root{&root{
		fd:   fd,
		name: name,
	}}
	runtime.SetFinalizer(r.root, (*root).Close)
	return r, nil
}

// openRootInRoot is Root.OpenRoot.
func openRootInRoot(r *Root, name string) (*Root, error) {
	fd, err := doInRoot(r, name, nil, rootOpenDir)
	if err != nil {
		return nil, &PathError{Op: "openat", Path: name, Err: err}
	}
	return newRoot(fd, joinPath(r.Name(), name))
}

// rootOpenFileNolog is Root.OpenFile.
func rootOpenFileNolog(root *Root, name string, flag int, perm FileMode) (*File, error) {
	fd, err := doInRoot(root, name, nil, func(parent syscall.Handle, name string) (syscall.Handle, error) {
		return openat(parent, name, uint64(flag), perm)
	})
	if err != nil {
		return nil, &PathError{Op: "openat", Path: name, Err: err}
	}
	nonblocking := flag&windows.O_FILE_FLAG_OVERLAPPED != 0
	return newFile(fd, joinPath(root.Name(), name), kindOpenFile, nonblocking), nil
}

func openat(dirfd syscall.Handle, name string, flag uint64, perm FileMode) (syscall.Handle, error) {
	h, err := windows.Openat(dirfd, name, flag|syscall.O_CLOEXEC|windows.O_NOFOLLOW_ANY, syscallMode(perm))
	if err == syscall.ELOOP || err == syscall.ENOTDIR {
		if link, err := readReparseLinkAt(dirfd, name); err == nil {
			return syscall.InvalidHandle, errSymlink(link)
		}
	}
	return h, err
}

func readReparseLinkAt(dirfd syscall.Handle, name string) (string, error) {
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return "", err
	}
	objAttrs := &windows.OBJECT_ATTRIBUTES{
		ObjectName: objectName,
	}
	if dirfd != syscall.InvalidHandle {
		objAttrs.RootDirectory = dirfd
	}
	objAttrs.Length = uint32(unsafe.Sizeof(*objAttrs))
	var h syscall.Handle
	err = windows.NtCreateFile(
		&h,
		windows.FILE_GENERIC_READ,
		objAttrs,
		&windows.IO_STATUS_BLOCK{},
		nil,
		uint32(syscall.FILE_ATTRIBUTE_NORMAL),
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_REPARSE_POINT,
		nil,
		0,
	)
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(h)
	return readReparseLinkHandle(h)
}

func rootOpenDir(parent syscall.Handle, name string) (syscall.Handle, error) {
	h, err := openat(parent, name, syscall.O_RDONLY|syscall.O_CLOEXEC|windows.O_DIRECTORY, 0)
	if err == syscall.ERROR_FILE_NOT_FOUND {
		// Windows returns:
		//   - ERROR_PATH_NOT_FOUND if any path component before the leaf
		//     does not exist or is not a directory.
		//   - ERROR_FILE_NOT_FOUND if the leaf does not exist.
		//
		// This differs from Unix behavior, which is:
		//   - ENOENT if any path component does not exist, including the leaf.
		//   - ENOTDIR if any path component before the leaf is not a directory.
		//
		// We map syscall.ENOENT to ERROR_FILE_NOT_FOUND and syscall.ENOTDIR
		// to ERROR_PATH_NOT_FOUND, but the Windows errors don't quite match.
		//
		// For consistency with os.Open, convert ERROR_FILE_NOT_FOUND here into
		// ERROR_PATH_NOT_FOUND, since we're opening a non-leaf path component.
		err = syscall.ERROR_PATH_NOT_FOUND
	}
	return h, err
}

func rootStat(r *Root, name string, lstat bool) (FileInfo, error) {
	if len(name) > 0 && IsPathSeparator(name[len(name)-1]) {
		// When a filename ends with a path separator,
		// Lstat behaves like Stat.
		//
		// This behavior is not based on a principled decision here,
		// merely the empirical evidence that Lstat behaves this way.
		lstat = false
	}
	fi, err := doInRoot(r, name, nil, func(parent syscall.Handle, n string) (FileInfo, error) {
		fd, err := openat(parent, n, windows.O_FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err != nil {
			return nil, err
		}
		defer syscall.CloseHandle(fd)
		fi, err := statHandle(name, fd)
		if err != nil {
			return nil, err
		}
		if !lstat && fi.(*fileStat).isReparseTagNameSurrogate() {
			link, err := readReparseLinkHandle(fd)
			if err != nil {
				return nil, err
			}
			return nil, errSymlink(link)
		}
		return fi, nil
	})
	if err != nil {
		return nil, &PathError{Op: "statat", Path: name, Err: err}
	}
	return fi, nil
}

func rootSymlink(r *Root, oldname, newname string) error {
	if oldname == "" {
		return syscall.EINVAL
	}

	// Windows treats / and \ as equivalent almost everywhere, but not in symlink targets.
	// Match os.Symlink behavior and convert / to \.
	oldname = filepathlite.FromSlash(oldname)

	// CreateSymbolicLinkW converts volume-relative paths into absolute ones.
	// Do the same.
	if filepathlite.VolumeNameLen(oldname) > 0 && !filepathlite.IsAbs(oldname) {
		p, err := syscall.FullPath(oldname)
		if err == nil {
			oldname = p
		}
	}

	// If oldname can be resolved to a directory in the root, create a directory link.
	// Otherwise, create a file link.
	var flags windows.SymlinkatFlags
	if filepathlite.VolumeNameLen(oldname) == 0 && !IsPathSeparator(oldname[0]) {
		// oldname is a path relative to the directory containing newname.
		// Prepend newname's directory to it to make a path relative to the root.
		// For example, if oldname=old and newname=a\new, destPath=a\old.
		destPath := oldname
		if dir := dirname(newname); dir != "." {
			destPath = dir + `\` + oldname
		}
		fi, err := r.Stat(destPath)
		if err == nil && fi.IsDir() {
			flags |= windows.SYMLINKAT_DIRECTORY
		}
	}

	// Empirically, CreateSymbolicLinkW appears to set the relative flag iff
	// the target does not contain a volume name.
	if filepathlite.VolumeNameLen(oldname) == 0 {
		flags |= windows.SYMLINKAT_RELATIVE
	}

	_, err := doInRoot(r, newname, nil, func(parent sysfdType, name string) (struct{}, error) {
		return struct{}{}, windows.Symlinkat(oldname, parent, name, flags)
	})
	if err != nil {
		return &LinkError{"symlinkat", oldname, newname, err}
	}
	return nil
}

func chmodat(parent syscall.Handle, name string, mode FileMode) error {
	// Currently, on Windows os.Chmod("symlink") will act on "symlink",
	// not on any file it points to.
	//
	// This may or may not be the desired behavior: https://go.dev/issue/71492
	//
	// For now, be consistent with os.Symlink.
	// Passing O_FILE_FLAG_OPEN_REPARSE_POINT causes us to open the named file itself,
	// not any file that it links to.
	//
	// If we want to change this in the future, pass O_NOFOLLOW_ANY instead
	// and return errSymlink when encountering a symlink:
	//
	//     if err == syscall.ELOOP || err == syscall.ENOTDIR {
	//         if link, err := readReparseLinkAt(parent, name); err == nil {
	//                 return errSymlink(link)
	//         }
	//     }
	h, err := windows.Openat(parent, name, syscall.O_CLOEXEC|windows.O_FILE_FLAG_OPEN_REPARSE_POINT|windows.O_WRITE_ATTRS, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)

	var d syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &d); err != nil {
		return err
	}
	attrs := d.FileAttributes

	if mode&syscall.S_IWRITE != 0 {
		attrs &^= syscall.FILE_ATTRIBUTE_READONLY
	} else {
		attrs |= syscall.FILE_ATTRIBUTE_READONLY
	}
	if attrs == d.FileAttributes {
		return nil
	}

	var fbi windows.FILE_BASIC_INFO
	fbi.FileAttributes = attrs
	return windows.SetFileInformationByHandle(h, windows.FileBasicInfo, unsafe.Pointer(&fbi), uint32(unsafe.Sizeof(fbi)))
}

func chownat(parent syscall.Handle, name string, uid, gid int) error {
	return syscall.EWINDOWS // matches syscall.Chown
}

func lchownat(parent syscall.Handle, name string, uid, gid int) error {
	return syscall.EWINDOWS // matches syscall.Lchown
}

func mkdirat(dirfd syscall.Handle, name string, perm FileMode) error {
	return windows.Mkdirat(dirfd, name, syscallMode(perm))
}

func removeat(dirfd syscall.Handle, name string) error {
	return windows.Deleteat(dirfd, name, 0)
}

func removefileat(dirfd syscall.Handle, name string) error {
	return windows.Deleteat(dirfd, name, windows.FILE_NON_DIRECTORY_FILE)
}

func removedirat(dirfd syscall.Handle, name string) error {
	return windows.Deleteat(dirfd, name, windows.FILE_DIRECTORY_FILE)
}

func chtimesat(dirfd syscall.Handle, name string, atime time.Time, mtime time.Time) error {
	h, err := windows.Openat(dirfd, name, syscall.O_CLOEXEC|windows.O_NOFOLLOW_ANY|windows.O_WRITE_ATTRS, 0)
	if err == syscall.ELOOP || err == syscall.ENOTDIR {
		if link, err := readReparseLinkAt(dirfd, name); err == nil {
			return errSymlink(link)
		}
	}
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)
	a := syscall.Filetime{}
	w := syscall.Filetime{}
	if !atime.IsZero() {
		a = syscall.NsecToFiletime(atime.UnixNano())
	}
	if !mtime.IsZero() {
		w = syscall.NsecToFiletime(mtime.UnixNano())
	}
	return syscall.SetFileTime(h, nil, &a, &w)
}

func renameat(oldfd syscall.Handle, oldname string, newfd syscall.Handle, newname string) error {
	return windows.Renameat(oldfd, oldname, newfd, newname)
}

func linkat(oldfd syscall.Handle, oldname string, newfd syscall.Handle, newname string) error {
	return windows.Linkat(oldfd, oldname, newfd, newname)
}

func readlinkat(dirfd syscall.Handle, name string) (string, error) {
	fd, err := openat(dirfd, name, windows.O_FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(fd)
	return readReparseLinkHandle(fd)
}

func modeAt(parent syscall.Handle, name string) (FileMode, error) {
	fd, err := openat(parent, name, windows.O_FILE_FLAG_OPEN_REPARSE_POINT|windows.O_DIRECTORY, 0)
	if err != nil {
		return 0, err
	}
	defer syscall.CloseHandle(fd)
	fi, err := statHandle(name, fd)
	if err != nil {
		return 0, err
	}
	return fi.Mode(), nil
}

```

// === FILE: references!/go/src/os/signal/doc.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package signal implements access to incoming signals.

Signals are primarily used on Unix-like systems. For the use of this
package on Windows and Plan 9, see below.

# Types of signals

The signals SIGKILL and SIGSTOP may not be caught by a program, and
therefore cannot be affected by this package.

Synchronous signals are signals triggered by errors in program
execution: SIGBUS, SIGFPE, and SIGSEGV. These are only considered
synchronous when caused by program execution, not when sent using
[os.Process.Kill] or the kill program or some similar mechanism. In
general, except as discussed below, Go programs will convert a
synchronous signal into a run-time panic.

The remaining signals are asynchronous signals. They are not
triggered by program errors, but are instead sent from the kernel or
from some other program.

Of the asynchronous signals, the SIGHUP signal is sent when a program
loses its controlling terminal. The SIGINT signal is sent when the
user at the controlling terminal presses the interrupt character,
which by default is ^C (Control-C). The SIGQUIT signal is sent when
the user at the controlling terminal presses the quit character, which
by default is ^\ (Control-Backslash). In general you can cause a
program to simply exit by pressing ^C, and you can cause it to exit
with a stack dump by pressing ^\.

# Default behavior of signals in Go programs

By default, a synchronous signal is converted into a run-time panic. A
SIGHUP, SIGINT, or SIGTERM signal causes the program to exit. A
SIGQUIT, SIGILL, SIGTRAP, SIGABRT, SIGSTKFLT, SIGEMT, or SIGSYS signal
causes the program to exit with a stack dump. A SIGTSTP, SIGTTIN, or
SIGTTOU signal gets the system default behavior (these signals are
used by the shell for job control). The SIGPROF signal is handled
directly by the Go runtime to implement runtime.CPUProfile. Other
signals will be caught but no action will be taken.

If the Go program is started with either SIGHUP or SIGINT ignored
(signal handler set to SIG_IGN), they will remain ignored.

If the Go program is started with a non-empty signal mask, that will
generally be honored. However, some signals are explicitly unblocked:
the synchronous signals, SIGILL, SIGTRAP, SIGSTKFLT, SIGCHLD, SIGPROF,
and, on Linux, signals 32 (SIGCANCEL) and 33 (SIGSETXID)
(SIGCANCEL and SIGSETXID are used internally by glibc). Subprocesses
started by [os.Exec], or by [os/exec], will inherit the
modified signal mask.

# Changing the behavior of signals in Go programs

The functions in this package allow a program to change the way Go
programs handle signals.

Notify disables the default behavior for a given set of asynchronous
signals and instead delivers them over one or more registered
channels. Specifically, it applies to the signals SIGHUP, SIGINT,
SIGQUIT, SIGABRT, and SIGTERM. It also applies to the job control
signals SIGTSTP, SIGTTIN, and SIGTTOU, in which case the system
default behavior does not occur. It also applies to some signals that
otherwise cause no action: SIGUSR1, SIGUSR2, SIGPIPE, SIGALRM,
SIGCHLD, SIGCONT, SIGURG, SIGXCPU, SIGXFSZ, SIGVTALRM, SIGWINCH,
SIGIO, SIGPWR, SIGINFO, SIGTHR, SIGWAITING, SIGLWP, SIGFREEZE,
SIGTHAW, SIGLOST, SIGXRES, SIGJVM1, SIGJVM2, and any real time signals
used on the system. Note that not all of these signals are available
on all systems.

If the program was started with SIGHUP or SIGINT ignored, and [Notify]
is called for either signal, a signal handler will be installed for
that signal and it will no longer be ignored. If, later, [Reset] or
[Ignore] is called for that signal, or [Stop] is called on all channels
passed to Notify for that signal, the signal will once again be
ignored. Reset will restore the system default behavior for the
signal, while Ignore will cause the system to ignore the signal
entirely.

If the program is started with a non-empty signal mask, some signals
will be explicitly unblocked as described above. If Notify is called
for a blocked signal, it will be unblocked. If, later, Reset is
called for that signal, or Stop is called on all channels passed to
Notify for that signal, the signal will once again be blocked.

# SIGPIPE

When a Go program writes to a broken pipe, the kernel will raise a
SIGPIPE signal.

If the program has not called Notify to receive SIGPIPE signals, then
the behavior depends on the file descriptor number. A write to a
broken pipe on file descriptors 1 or 2 (standard output or standard
error) will cause the program to exit with a SIGPIPE signal. A write
to a broken pipe on some other file descriptor will take no action on
the SIGPIPE signal, and the write will fail with a [syscall.EPIPE]
error.

If the program has called Notify to receive SIGPIPE signals, the file
descriptor number does not matter. The SIGPIPE signal will be
delivered to the Notify channel, and the write will fail with a
[syscall.EPIPE] error.

This means that, by default, command line programs will behave like
typical Unix command line programs, while other programs will not
crash with SIGPIPE when writing to a closed network connection.

# Go programs that use cgo or SWIG

In a Go program that includes non-Go code, typically C/C++ code
accessed using cgo or SWIG, Go's startup code normally runs first. It
configures the signal handlers as expected by the Go runtime, before
the non-Go startup code runs. If the non-Go startup code wishes to
install its own signal handlers, it must take certain steps to keep Go
working well. This section documents those steps and the overall
effect changes to signal handler settings by the non-Go code can have
on Go programs. In rare cases, the non-Go code may run before the Go
code, in which case the next section also applies.

If the non-Go code called by the Go program does not change any signal
handlers or masks, then the behavior is the same as for a pure Go
program.

If the non-Go code installs any signal handlers, it must use the
SA_ONSTACK flag with sigaction. Failing to do so is likely to cause
the program to crash if the signal is received. Go programs routinely
run with a limited stack, and therefore set up an alternate signal
stack.

If the non-Go code installs a signal handler for any of the
synchronous signals (SIGBUS, SIGFPE, SIGSEGV), then it should record
the existing Go signal handler. If those signals occur while
executing Go code, it should invoke the Go signal handler (whether the
signal occurs while executing Go code can be determined by looking at
the PC passed to the signal handler). Otherwise some Go run-time
panics will not occur as expected.

If the non-Go code installs a signal handler for any of the
asynchronous signals, it may invoke the Go signal handler or not as it
chooses. Naturally, if it does not invoke the Go signal handler, the
Go behavior described above will not occur. This can be an issue with
the SIGPROF signal in particular.

The non-Go code should not change the signal mask on any threads
created by the Go runtime. If the non-Go code starts new threads
itself, those threads may set the signal mask as they please.

If the non-Go code starts a new thread, changes the signal mask, and
then invokes a Go function in that thread, the Go runtime will
automatically unblock certain signals: the synchronous signals,
SIGILL, SIGTRAP, SIGSTKFLT, SIGCHLD, SIGPROF, SIGCANCEL, and
SIGSETXID. When the Go function returns, the non-Go signal mask will
be restored.

If the Go signal handler is invoked on a non-Go thread not running Go
code, the handler generally forwards the signal to the non-Go code, as
follows. If the signal is SIGPROF, the Go handler does
nothing. Otherwise, the Go handler removes itself, unblocks the
signal, and raises it again, to invoke any non-Go handler or default
system handler. If the program does not exit, the Go handler then
reinstalls itself and continues execution of the program.

If a SIGPIPE signal is received, the Go program will invoke the
special handling described above if the SIGPIPE is received on a Go
thread.  If the SIGPIPE is received on a non-Go thread the signal will
be forwarded to the non-Go handler, if any; if there is none the
default system handler will cause the program to terminate.

# Non-Go programs that call Go code

When Go code is built with options like -buildmode=c-shared, it will
be run as part of an existing non-Go program. The non-Go code may
have already installed signal handlers when the Go code starts (that
may also happen in unusual cases when using cgo or SWIG; in that case,
the discussion here applies).  For -buildmode=c-archive the Go runtime
will initialize signals at global constructor time.  For
-buildmode=c-shared the Go runtime will initialize signals when the
shared library is loaded.

If the Go runtime sees an existing signal handler for the SIGCANCEL or
SIGSETXID signals (which are used only on Linux), it will turn on
the SA_ONSTACK flag and otherwise keep the signal handler.

For the synchronous signals and SIGPIPE, the Go runtime will install a
signal handler. It will save any existing signal handler. If a
synchronous signal arrives while executing non-Go code, the Go runtime
will invoke the existing signal handler instead of the Go signal
handler.

Go code built with -buildmode=c-archive or -buildmode=c-shared will
not install any other signal handlers by default. If there is an
existing signal handler, the Go runtime will turn on the SA_ONSTACK
flag and otherwise keep the signal handler. If Notify is called for an
asynchronous signal, a Go signal handler will be installed for that
signal. If, later, Reset is called for that signal, the original
handling for that signal will be reinstalled, restoring the non-Go
signal handler if any.

Go code built without -buildmode=c-archive or -buildmode=c-shared will
install a signal handler for the asynchronous signals listed above,
and save any existing signal handler. If a signal is delivered to a
non-Go thread, it will act as described above, except that if there is
an existing non-Go signal handler, that handler will be installed
before raising the signal.

# Windows

On Windows a ^C (Control-C) or ^BREAK (Control-Break) normally cause
the program to exit. If Notify is called for [os.Interrupt], ^C or ^BREAK
will cause [os.Interrupt] to be sent on the channel, and the program will
not exit. [os.Interrupt] is the only signal that can be used on Windows.
If Reset is called, or Stop is called on all channels passed to Notify,
then the default behavior will be restored.

Additionally, if Notify is called, and Windows sends CTRL_CLOSE_EVENT,
CTRL_LOGOFF_EVENT or CTRL_SHUTDOWN_EVENT to the process, Notify will
return syscall.SIGTERM. Unlike Control-C and Control-Break, Notify does
not change process behavior when either CTRL_CLOSE_EVENT,
CTRL_LOGOFF_EVENT or CTRL_SHUTDOWN_EVENT is received - the process will
still get terminated unless it exits. But receiving syscall.SIGTERM will
give the process an opportunity to clean up before termination.

# Plan 9

On Plan 9, signals have type syscall.Note, which is a string. Calling
Notify with a syscall.Note will cause that value to be sent on the
channel when that string is posted as a note.
*/
package signal

```

// === FILE: references!/go/src/os/signal/sig.s ===
```text
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The runtime package uses //go:linkname to push a few functions into this
// package but we still need a .s file so the Go tool does not pass -complete
// to the go tool compile so the latter does not complain about Go functions
// with no bodies.

```

// === FILE: references!/go/src/os/signal/signal.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package signal

import (
	"context"
	"os"
	"slices"
	"sync"
)

var handlers struct {
	sync.Mutex
	// Map a channel to the signals that should be sent to it.
	m map[chan<- os.Signal]*handler
	// Map a signal to the number of channels receiving it.
	ref [numSig]int64
	// Map channels to signals while the channel is being stopped.
	// Not a map because entries live here only very briefly.
	// We need a separate container because we need m to correspond to ref
	// at all times, and we also need to keep track of the *handler
	// value for a channel being stopped. See the Stop function.
	stopping []stopping
}

type stopping struct {
	c chan<- os.Signal
	h *handler
}

type handler struct {
	mask [(numSig + 31) / 32]uint32
}

func (h *handler) want(sig int) bool {
	return (h.mask[sig/32]>>uint(sig&31))&1 != 0
}

func (h *handler) set(sig int) {
	h.mask[sig/32] |= 1 << uint(sig&31)
}

func (h *handler) clear(sig int) {
	h.mask[sig/32] &^= 1 << uint(sig&31)
}

// Stop relaying the signals, sigs, to any channels previously registered to
// receive them and either reset the signal handlers to their original values
// (action=disableSignal) or ignore the signals (action=ignoreSignal).
func cancel(sigs []os.Signal, action func(int)) {
	handlers.Lock()
	defer handlers.Unlock()

	remove := func(n int) {
		if n < 0 {
			return
		}

		var zerohandler handler

		for c, h := range handlers.m {
			if h.want(n) {
				handlers.ref[n]--
				h.clear(n)
				if h.mask == zerohandler.mask {
					delete(handlers.m, c)
				}
			}
		}

		action(n)
	}

	if len(sigs) == 0 {
		for n := 0; n < numSig; n++ {
			remove(n)
		}
	} else {
		for _, s := range sigs {
			remove(signum(s))
		}
	}
}

// Ignore causes the provided signals to be ignored. If they are received by
// the program, nothing will happen. Ignore undoes the effect of any prior
// calls to [Notify] for the provided signals.
// If no signals are provided, all incoming signals will be ignored.
func Ignore(sig ...os.Signal) {
	cancel(sig, ignoreSignal)
}

// Ignored reports whether sig is currently ignored.
func Ignored(sig os.Signal) bool {
	sn := signum(sig)
	return sn >= 0 && signalIgnored(sn)
}

var (
	// watchSignalLoopOnce guards calling the conditionally
	// initialized watchSignalLoop. If watchSignalLoop is non-nil,
	// it will be run in a goroutine lazily once Notify is invoked.
	// See Issue 21576.
	watchSignalLoopOnce sync.Once
	watchSignalLoop     func()
)

// Notify causes package signal to relay incoming signals to c.
// If no signals are provided, all incoming signals will be relayed to c.
// Otherwise, just the provided signals will.
//
// Package signal will not block sending to c: the caller must ensure
// that c has sufficient buffer space to keep up with the expected
// signal rate. For a channel used for notification of just one signal value,
// a buffer of size 1 is sufficient.
//
// It is allowed to call Notify multiple times with the same channel:
// each call expands the set of signals sent to that channel.
// The only way to remove signals from the set is to call [Stop].
//
// It is allowed to call Notify multiple times with different channels
// and the same signals: each channel receives copies of incoming
// signals independently.
func Notify(c chan<- os.Signal, sig ...os.Signal) {
	if c == nil {
		panic("os/signal: Notify using nil channel")
	}

	handlers.Lock()
	defer handlers.Unlock()

	// Lazily create the handler. If all of the signals are bogus there is
	// no need to install a handler at all.
	getHandler := func() *handler {
		h := handlers.m[c]
		if h == nil {
			if handlers.m == nil {
				handlers.m = make(map[chan<- os.Signal]*handler)
			}
			h = new(handler)
			handlers.m[c] = h
		}

		return h
	}

	add := func(n int) {
		if n < 0 {
			return
		}

		h := getHandler()
		if !h.want(n) {
			h.set(n)
			if handlers.ref[n] == 0 {
				enableSignal(n)

				// The runtime requires that we enable a
				// signal before starting the watcher.
				watchSignalLoopOnce.Do(func() {
					if watchSignalLoop != nil {
						go watchSignalLoop()
					}
				})
			}
			handlers.ref[n]++
		}
	}

	if len(sig) == 0 {
		for n := 0; n < numSig; n++ {
			add(n)
		}
	} else {
		for _, s := range sig {
			add(signum(s))
		}
	}
}

// Reset undoes the effect of any prior calls to [Notify] for the provided
// signals.
// If no signals are provided, all signal handlers will be reset.
func Reset(sig ...os.Signal) {
	cancel(sig, disableSignal)
}

// Stop causes package signal to stop relaying incoming signals to c.
// It undoes the effect of all prior calls to [Notify] using c.
// When Stop returns, it is guaranteed that c will receive no more signals.
func Stop(c chan<- os.Signal) {
	handlers.Lock()

	h := handlers.m[c]
	if h == nil {
		handlers.Unlock()
		return
	}
	delete(handlers.m, c)

	for n := 0; n < numSig; n++ {
		if h.want(n) {
			handlers.ref[n]--
			if handlers.ref[n] == 0 {
				disableSignal(n)
			}
		}
	}

	// Signals will no longer be delivered to the channel.
	// We want to avoid a race for a signal such as SIGINT:
	// it should be either delivered to the channel,
	// or the program should take the default action (that is, exit).
	// To avoid the possibility that the signal is delivered,
	// and the signal handler invoked, and then Stop deregisters
	// the channel before the process function below has a chance
	// to send it on the channel, put the channel on a list of
	// channels being stopped and wait for signal delivery to
	// quiesce before fully removing it.

	handlers.stopping = append(handlers.stopping, stopping{c, h})

	handlers.Unlock()

	signalWaitUntilIdle()

	handlers.Lock()

	for i, s := range handlers.stopping {
		if s.c == c {
			handlers.stopping = slices.Delete(handlers.stopping, i, i+1)
			break
		}
	}

	handlers.Unlock()
}

// Wait until there are no more signals waiting to be delivered.
// Defined by the runtime package.
func signalWaitUntilIdle()

func process(sig os.Signal) {
	n := signum(sig)
	if n < 0 {
		return
	}

	handlers.Lock()
	defer handlers.Unlock()

	for c, h := range handlers.m {
		if h.want(n) {
			// send but do not block for it
			select {
			case c <- sig:
			default:
			}
		}
	}

	// Avoid the race mentioned in Stop.
	for _, d := range handlers.stopping {
		if d.h.want(n) {
			select {
			case d.c <- sig:
			default:
			}
		}
	}
}

// NotifyContext returns a copy of the parent context that is marked done
// (its Done channel is closed) when one of the listed signals arrives,
// when the returned stop function is called, or when the parent context's
// Done channel is closed, whichever happens first.
//
// The stop function unregisters the signal behavior, which, like [signal.Reset],
// may restore the default behavior for a given signal. For example, the default
// behavior of a Go program receiving [os.Interrupt] is to exit. Calling
// NotifyContext(parent, os.Interrupt) will change the behavior to cancel
// the returned context. Future interrupts received will not trigger the default
// (exit) behavior until the returned stop function is called.
//
// If a signal causes the returned context to be canceled, calling
// [context.Cause] on it will return an error describing the signal.
//
// The stop function releases resources associated with it, so code should
// call stop as soon as the operations running in this Context complete and
// signals no longer need to be diverted to the context.
func NotifyContext(parent context.Context, signals ...os.Signal) (ctx context.Context, stop context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	c := &signalCtx{
		Context: ctx,
		cancel:  cancel,
		signals: signals,
	}
	c.ch = make(chan os.Signal, 1)
	Notify(c.ch, c.signals...)
	if ctx.Err() == nil {
		go func() {
			select {
			case s := <-c.ch:
				c.cancel(signalError(s.String() + " signal received"))
			case <-c.Done():
			}
		}()
	}
	return c, c.stop
}

type signalCtx struct {
	context.Context

	cancel  context.CancelCauseFunc
	signals []os.Signal
	ch      chan os.Signal
}

func (c *signalCtx) stop() {
	c.cancel(nil)
	Stop(c.ch)
}

type stringer interface {
	String() string
}

func (c *signalCtx) String() string {
	var buf []byte
	// We know that the type of c.Context is context.cancelCtx, and we know that the
	// String method of cancelCtx returns a string that ends with ".WithCancel".
	name := c.Context.(stringer).String()
	name = name[:len(name)-len(".WithCancel")]
	buf = append(buf, "signal.NotifyContext("+name...)
	if len(c.signals) != 0 {
		buf = append(buf, ", ["...)
		for i, s := range c.signals {
			buf = append(buf, s.String()...)
			if i != len(c.signals)-1 {
				buf = append(buf, ' ')
			}
		}
		buf = append(buf, ']')
	}
	buf = append(buf, ')')
	return string(buf)
}

type signalError string

func (s signalError) Error() string {
	return string(s)
}

func (s signalError) Is(target error) bool {
	if target == context.Canceled {
		return true
	}
	return false
}

```

// === FILE: references!/go/src/os/signal/signal_plan9.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package signal

import (
	"os"
	"syscall"
)

var sigtab = make(map[os.Signal]int)

// Defined by the runtime package.
func signal_disable(uint32)
func signal_enable(uint32)
func signal_ignore(uint32)
func signal_ignored(uint32) bool
func signal_recv() string

func init() {
	watchSignalLoop = loop
}

func loop() {
	for {
		process(syscall.Note(signal_recv()))
	}
}

const numSig = 256

func signum(sig os.Signal) int {
	switch sig := sig.(type) {
	case syscall.Note:
		n, ok := sigtab[sig]
		if !ok {
			n = len(sigtab) + 1
			if n > numSig {
				return -1
			}
			sigtab[sig] = n
		}
		return n
	default:
		return -1
	}
}

func enableSignal(sig int) {
	signal_enable(uint32(sig))
}

func disableSignal(sig int) {
	signal_disable(uint32(sig))
}

func ignoreSignal(sig int) {
	signal_ignore(uint32(sig))
}

func signalIgnored(sig int) bool {
	return signal_ignored(uint32(sig))
}

```

// === FILE: references!/go/src/os/signal/signal_unix.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || (js && wasm) || wasip1 || windows

package signal

import (
	"os"
	"syscall"
)

// Defined by the runtime package.
func signal_disable(uint32)
func signal_enable(uint32)
func signal_ignore(uint32)
func signal_ignored(uint32) bool
func signal_recv() uint32

func loop() {
	for {
		process(syscall.Signal(signal_recv()))
	}
}

func init() {
	watchSignalLoop = loop
}

const (
	numSig = 65 // max across all systems
)

func signum(sig os.Signal) int {
	switch sig := sig.(type) {
	case syscall.Signal:
		i := int(sig)
		if i < 0 || i >= numSig {
			return -1
		}
		return i
	default:
		return -1
	}
}

func enableSignal(sig int) {
	signal_enable(uint32(sig))
}

func disableSignal(sig int) {
	signal_disable(uint32(sig))
}

func ignoreSignal(sig int) {
	signal_ignore(uint32(sig))
}

func signalIgnored(sig int) bool {
	return signal_ignored(uint32(sig))
}

```

// === FILE: references!/go/src/os/stat.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import "internal/testlog"

// Stat returns a [FileInfo] describing the named file.
// If there is an error, it will be of type [*PathError].
func Stat(name string) (FileInfo, error) {
	testlog.Stat(name)
	return statNolog(name)
}

// Lstat returns a [FileInfo] describing the named file.
// If the file is a symbolic link, the returned FileInfo
// describes the symbolic link. Lstat makes no attempt to follow the link.
// If there is an error, it will be of type [*PathError].
//
// On Windows, if the file is a reparse point that is a surrogate for another
// named entity (such as a symbolic link or mounted folder), the returned
// FileInfo describes the reparse point, and makes no attempt to resolve it.
func Lstat(name string) (FileInfo, error) {
	testlog.Stat(name)
	return lstatNolog(name)
}

// stathook is set in tests
var stathook func(f *File, name string) (FileInfo, error)

```

// === FILE: references!/go/src/os/stat_aix.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/filepathlite"
	"syscall"
	"time"
)

func fillFileStatFromSys(fs *fileStat, name string) {
	fs.name = filepathlite.Base(name)
	fs.size = int64(fs.sys.Size)
	fs.modTime = stTimespecToTime(fs.sys.Mtim)
	fs.mode = FileMode(fs.sys.Mode & 0777)
	switch fs.sys.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		fs.mode |= ModeDevice
	case syscall.S_IFCHR:
		fs.mode |= ModeDevice | ModeCharDevice
	case syscall.S_IFDIR:
		fs.mode |= ModeDir
	case syscall.S_IFIFO:
		fs.mode |= ModeNamedPipe
	case syscall.S_IFLNK:
		fs.mode |= ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fs.mode |= ModeSocket
	}
	if fs.sys.Mode&syscall.S_ISGID != 0 {
		fs.mode |= ModeSetgid
	}
	if fs.sys.Mode&syscall.S_ISUID != 0 {
		fs.mode |= ModeSetuid
	}
	if fs.sys.Mode&syscall.S_ISVTX != 0 {
		fs.mode |= ModeSticky
	}
}

func stTimespecToTime(ts syscall.StTimespec_t) time.Time {
	return time.Unix(int64(ts.Sec), int64(ts.Nsec))
}

// For testing.
func atime(fi FileInfo) time.Time {
	return stTimespecToTime(fi.Sys().(*syscall.Stat_t).Atim)
}

```

// === FILE: references!/go/src/os/stat_darwin.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/filepathlite"
	"syscall"
	"time"
)

func fillFileStatFromSys(fs *fileStat, name string) {
	fs.name = filepathlite.Base(name)
	fs.size = fs.sys.Size
	fs.modTime = time.Unix(fs.sys.Mtimespec.Unix())
	fs.mode = FileMode(fs.sys.Mode & 0777)
	switch fs.sys.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK, syscall.S_IFWHT:
		fs.mode |= ModeDevice
	case syscall.S_IFCHR:
		fs.mode |= ModeDevice | ModeCharDevice
	case syscall.S_IFDIR:
		fs.mode |= ModeDir
	case syscall.S_IFIFO:
		fs.mode |= ModeNamedPipe
	case syscall.S_IFLNK:
		fs.mode |= ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fs.mode |= ModeSocket
	}
	if fs.sys.Mode&syscall.S_ISGID != 0 {
		fs.mode |= ModeSetgid
	}
	if fs.sys.Mode&syscall.S_ISUID != 0 {
		fs.mode |= ModeSetuid
	}
	if fs.sys.Mode&syscall.S_ISVTX != 0 {
		fs.mode |= ModeSticky
	}
}

// For testing.
func atime(fi FileInfo) time.Time {
	return time.Unix(fi.Sys().(*syscall.Stat_t).Atimespec.Unix())
}

```

// === FILE: references!/go/src/os/stat_dragonfly.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/filepathlite"
	"syscall"
	"time"
)

func fillFileStatFromSys(fs *fileStat, name string) {
	fs.name = filepathlite.Base(name)
	fs.size = fs.sys.Size
	fs.modTime = time.Unix(fs.sys.Mtim.Unix())
	fs.mode = FileMode(fs.sys.Mode & 0777)
	switch fs.sys.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		fs.mode |= ModeDevice
	case syscall.S_IFCHR:
		fs.mode |= ModeDevice | ModeCharDevice
	case syscall.S_IFDIR:
		fs.mode |= ModeDir
	case syscall.S_IFIFO:
		fs.mode |= ModeNamedPipe
	case syscall.S_IFLNK:
		fs.mode |= ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fs.mode |= ModeSocket
	}
	if fs.sys.Mode&syscall.S_ISGID != 0 {
		fs.mode |= ModeSetgid
	}
	if fs.sys.Mode&syscall.S_ISUID != 0 {
		fs.mode |= ModeSetuid
	}
	if fs.sys.Mode&syscall.S_ISVTX != 0 {
		fs.mode |= ModeSticky
	}
}

// For testing.
func atime(fi FileInfo) time.Time {
	return time.Unix(fi.Sys().(*syscall.Stat_t).Atim.Unix())
}

```

// === FILE: references!/go/src/os/stat_freebsd.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/filepathlite"
	"syscall"
	"time"
)

func fillFileStatFromSys(fs *fileStat, name string) {
	fs.name = filepathlite.Base(name)
	fs.size = fs.sys.Size
	fs.modTime = time.Unix(fs.sys.Mtimespec.Unix())
	fs.mode = FileMode(fs.sys.Mode & 0777)
	switch fs.sys.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		fs.mode |= ModeDevice
	case syscall.S_IFCHR:
		fs.mode |= ModeDevice | ModeCharDevice
	case syscall.S_IFDIR:
		fs.mode |= ModeDir
	case syscall.S_IFIFO:
		fs.mode |= ModeNamedPipe
	case syscall.S_IFLNK:
		fs.mode |= ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fs.mode |= ModeSocket
	}
	if fs.sys.Mode&syscall.S_ISGID != 0 {
		fs.mode |= ModeSetgid
	}
	if fs.sys.Mode&syscall.S_ISUID != 0 {
		fs.mode |= ModeSetuid
	}
	if fs.sys.Mode&syscall.S_ISVTX != 0 {
		fs.mode |= ModeSticky
	}
}

// For testing.
func atime(fi FileInfo) time.Time {
	return time.Unix(fi.Sys().(*syscall.Stat_t).Atimespec.Unix())
}

```

// === FILE: references!/go/src/os/stat_js.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build js && wasm

package os

import (
	"internal/filepathlite"
	"syscall"
	"time"
)

func fillFileStatFromSys(fs *fileStat, name string) {
	fs.name = filepathlite.Base(name)
	fs.size = fs.sys.Size
	fs.modTime = time.Unix(fs.sys.Mtime, fs.sys.MtimeNsec)
	fs.mode = FileMode(fs.sys.Mode & 0777)
	switch fs.sys.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		fs.mode |= ModeDevice
	case syscall.S_IFCHR:
		fs.mode |= ModeDevice | ModeCharDevice
	case syscall.S_IFDIR:
		fs.mode |= ModeDir
	case syscall.S_IFIFO:
		fs.mode |= ModeNamedPipe
	case syscall.S_IFLNK:
		fs.mode |= ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fs.mode |= ModeSocket
	}
	if fs.sys.Mode&syscall.S_ISGID != 0 {
		fs.mode |= ModeSetgid
	}
	if fs.sys.Mode&syscall.S_ISUID != 0 {
		fs.mode |= ModeSetuid
	}
	if fs.sys.Mode&syscall.S_ISVTX != 0 {
		fs.mode |= ModeSticky
	}
}

// For testing.
func atime(fi FileInfo) time.Time {
	st := fi.Sys().(*syscall.Stat_t)
	return time.Unix(st.Atime, st.AtimeNsec)
}

```

// === FILE: references!/go/src/os/stat_linux.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/filepathlite"
	"syscall"
	"time"
)

func fillFileStatFromSys(fs *fileStat, name string) {
	fs.name = filepathlite.Base(name)
	fs.size = fs.sys.Size
	fs.modTime = time.Unix(fs.sys.Mtim.Unix())
	fs.mode = FileMode(fs.sys.Mode & 0777)
	switch fs.sys.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		fs.mode |= ModeDevice
	case syscall.S_IFCHR:
		fs.mode |= ModeDevice | ModeCharDevice
	case syscall.S_IFDIR:
		fs.mode |= ModeDir
	case syscall.S_IFIFO:
		fs.mode |= ModeNamedPipe
	case syscall.S_IFLNK:
		fs.mode |= ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fs.mode |= ModeSocket
	}
	if fs.sys.Mode&syscall.S_ISGID != 0 {
		fs.mode |= ModeSetgid
	}
	if fs.sys.Mode&syscall.S_ISUID != 0 {
		fs.mode |= ModeSetuid
	}
	if fs.sys.Mode&syscall.S_ISVTX != 0 {
		fs.mode |= ModeSticky
	}
}

// For testing.
func atime(fi FileInfo) time.Time {
	return time.Unix(fi.Sys().(*syscall.Stat_t).Atim.Unix())
}

```

// === FILE: references!/go/src/os/stat_netbsd.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/filepathlite"
	"syscall"
	"time"
)

func fillFileStatFromSys(fs *fileStat, name string) {
	fs.name = filepathlite.Base(name)
	fs.size = fs.sys.Size
	fs.modTime = time.Unix(fs.sys.Mtimespec.Unix())
	fs.mode = FileMode(fs.sys.Mode & 0777)
	switch fs.sys.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		fs.mode |= ModeDevice
	case syscall.S_IFCHR:
		fs.mode |= ModeDevice | ModeCharDevice
	case syscall.S_IFDIR:
		fs.mode |= ModeDir
	case syscall.S_IFIFO:
		fs.mode |= ModeNamedPipe
	case syscall.S_IFLNK:
		fs.mode |= ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fs.mode |= ModeSocket
	}
	if fs.sys.Mode&syscall.S_ISGID != 0 {
		fs.mode |= ModeSetgid
	}
	if fs.sys.Mode&syscall.S_ISUID != 0 {
		fs.mode |= ModeSetuid
	}
	if fs.sys.Mode&syscall.S_ISVTX != 0 {
		fs.mode |= ModeSticky
	}
}

// For testing.
func atime(fi FileInfo) time.Time {
	return time.Unix(fi.Sys().(*syscall.Stat_t).Atimespec.Unix())
}

```

// === FILE: references!/go/src/os/stat_openbsd.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/filepathlite"
	"syscall"
	"time"
)

func fillFileStatFromSys(fs *fileStat, name string) {
	fs.name = filepathlite.Base(name)
	fs.size = fs.sys.Size
	fs.modTime = time.Unix(fs.sys.Mtim.Unix())
	fs.mode = FileMode(fs.sys.Mode & 0777)
	switch fs.sys.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		fs.mode |= ModeDevice
	case syscall.S_IFCHR:
		fs.mode |= ModeDevice | ModeCharDevice
	case syscall.S_IFDIR:
		fs.mode |= ModeDir
	case syscall.S_IFIFO:
		fs.mode |= ModeNamedPipe
	case syscall.S_IFLNK:
		fs.mode |= ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fs.mode |= ModeSocket
	}
	if fs.sys.Mode&syscall.S_ISGID != 0 {
		fs.mode |= ModeSetgid
	}
	if fs.sys.Mode&syscall.S_ISUID != 0 {
		fs.mode |= ModeSetuid
	}
	if fs.sys.Mode&syscall.S_ISVTX != 0 {
		fs.mode |= ModeSticky
	}
}

// For testing.
func atime(fi FileInfo) time.Time {
	return time.Unix(fi.Sys().(*syscall.Stat_t).Atim.Unix())
}

```

// === FILE: references!/go/src/os/stat_plan9.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"time"
)

const bitSize16 = 2

func fileInfoFromStat(d *syscall.Dir) *fileStat {
	fs := &fileStat{
		name:    d.Name,
		size:    d.Length,
		modTime: time.Unix(int64(d.Mtime), 0),
		sys:     d,
	}
	fs.mode = FileMode(d.Mode & 0777)
	if d.Mode&syscall.DMDIR != 0 {
		fs.mode |= ModeDir
	}
	if d.Mode&syscall.DMAPPEND != 0 {
		fs.mode |= ModeAppend
	}
	if d.Mode&syscall.DMEXCL != 0 {
		fs.mode |= ModeExclusive
	}
	if d.Mode&syscall.DMTMP != 0 {
		fs.mode |= ModeTemporary
	}
	// Consider all files not served by #M as device files.
	if d.Type != 'M' {
		fs.mode |= ModeDevice
	}
	// Consider all files served by #c as character device files.
	if d.Type == 'c' {
		fs.mode |= ModeCharDevice
	}
	return fs
}

// arg is an open *File or a path string.
func dirstat(arg any) (*syscall.Dir, error) {
	var name string
	var err error

	size := syscall.STATFIXLEN + 16*4

	for i := 0; i < 2; i++ {
		buf := make([]byte, bitSize16+size)

		var n int
		switch a := arg.(type) {
		case *File:
			name = a.name
			if err := a.incref("fstat"); err != nil {
				return nil, err
			}
			n, err = syscall.Fstat(a.sysfd, buf)
			a.decref()
		case string:
			name = a
			n, err = syscall.Stat(a, buf)
		default:
			panic("phase error in dirstat")
		}

		if n < bitSize16 {
			return nil, &PathError{Op: "stat", Path: name, Err: err}
		}

		// Pull the real size out of the stat message.
		size = int(uint16(buf[0]) | uint16(buf[1])<<8)

		// If the stat message is larger than our buffer we will
		// go around the loop and allocate one that is big enough.
		if size <= n {
			d, err := syscall.UnmarshalDir(buf[:n])
			if err != nil {
				return nil, &PathError{Op: "stat", Path: name, Err: err}
			}
			return d, nil
		}

	}

	if err == nil {
		err = syscall.ErrBadStat
	}

	return nil, &PathError{Op: "stat", Path: name, Err: err}
}

// statNolog implements Stat for Plan 9.
func statNolog(name string) (FileInfo, error) {
	d, err := dirstat(name)
	if err != nil {
		return nil, err
	}
	return fileInfoFromStat(d), nil
}

// lstatNolog implements Lstat for Plan 9.
func lstatNolog(name string) (FileInfo, error) {
	return statNolog(name)
}

// For testing.
func atime(fi FileInfo) time.Time {
	return time.Unix(int64(fi.Sys().(*syscall.Dir).Atime), 0)
}

```

// === FILE: references!/go/src/os/stat_solaris.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/filepathlite"
	"syscall"
	"time"
)

// These constants aren't in the syscall package, which is frozen.
// Values taken from golang.org/x/sys/unix.
const (
	_S_IFNAM  = 0x5000
	_S_IFDOOR = 0xd000
	_S_IFPORT = 0xe000
)

func fillFileStatFromSys(fs *fileStat, name string) {
	fs.name = filepathlite.Base(name)
	fs.size = fs.sys.Size
	fs.modTime = time.Unix(fs.sys.Mtim.Unix())
	fs.mode = FileMode(fs.sys.Mode & 0777)
	switch fs.sys.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		fs.mode |= ModeDevice
	case syscall.S_IFCHR:
		fs.mode |= ModeDevice | ModeCharDevice
	case syscall.S_IFDIR:
		fs.mode |= ModeDir
	case syscall.S_IFIFO:
		fs.mode |= ModeNamedPipe
	case syscall.S_IFLNK:
		fs.mode |= ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fs.mode |= ModeSocket
	case _S_IFNAM, _S_IFDOOR, _S_IFPORT:
		fs.mode |= ModeIrregular
	}
	if fs.sys.Mode&syscall.S_ISGID != 0 {
		fs.mode |= ModeSetgid
	}
	if fs.sys.Mode&syscall.S_ISUID != 0 {
		fs.mode |= ModeSetuid
	}
	if fs.sys.Mode&syscall.S_ISVTX != 0 {
		fs.mode |= ModeSticky
	}
}

// For testing.
func atime(fi FileInfo) time.Time {
	return time.Unix(fi.Sys().(*syscall.Stat_t).Atim.Unix())
}

```

// === FILE: references!/go/src/os/stat_unix.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || (js && wasm) || wasip1

package os

import (
	"syscall"
)

// Stat returns the [FileInfo] structure describing file.
// If there is an error, it will be of type [*PathError].
func (f *File) Stat() (FileInfo, error) {
	if f == nil {
		return nil, ErrInvalid
	}
	var fs fileStat
	err := f.pfd.Fstat(&fs.sys)
	if err != nil {
		return nil, f.wrapErr("stat", err)
	}
	fillFileStatFromSys(&fs, f.name)
	return &fs, nil
}

// statNolog stats a file with no test logging.
func statNolog(name string) (FileInfo, error) {
	var fs fileStat
	err := ignoringEINTR(func() error {
		return syscall.Stat(name, &fs.sys)
	})
	if err != nil {
		return nil, &PathError{Op: "stat", Path: name, Err: err}
	}
	fillFileStatFromSys(&fs, name)
	return &fs, nil
}

// lstatNolog lstats a file with no test logging.
func lstatNolog(name string) (FileInfo, error) {
	var fs fileStat
	err := ignoringEINTR(func() error {
		return syscall.Lstat(name, &fs.sys)
	})
	if err != nil {
		return nil, &PathError{Op: "lstat", Path: name, Err: err}
	}
	fillFileStatFromSys(&fs, name)
	return &fs, nil
}

```

// === FILE: references!/go/src/os/stat_wasip1.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build wasip1

package os

import (
	"internal/filepathlite"
	"syscall"
	"time"
)

func fillFileStatFromSys(fs *fileStat, name string) {
	fs.name = filepathlite.Base(name)
	fs.size = int64(fs.sys.Size)
	fs.modTime = time.Unix(0, int64(fs.sys.Mtime))

	switch fs.sys.Filetype {
	case syscall.FILETYPE_BLOCK_DEVICE:
		fs.mode |= ModeDevice
	case syscall.FILETYPE_CHARACTER_DEVICE:
		fs.mode |= ModeDevice | ModeCharDevice
	case syscall.FILETYPE_DIRECTORY:
		fs.mode |= ModeDir
	case syscall.FILETYPE_SOCKET_DGRAM:
		fs.mode |= ModeSocket
	case syscall.FILETYPE_SOCKET_STREAM:
		fs.mode |= ModeSocket
	case syscall.FILETYPE_SYMBOLIC_LINK:
		fs.mode |= ModeSymlink
	}

	// WASI does not support unix-like permissions, but Go programs are likely
	// to expect the permission bits to not be zero so we set defaults to help
	// avoid breaking applications that are migrating to WASM.
	if fs.sys.Filetype == syscall.FILETYPE_DIRECTORY {
		fs.mode |= 0700
	} else {
		fs.mode |= 0600
	}
}

// For testing.
func atime(fi FileInfo) time.Time {
	st := fi.Sys().(*syscall.Stat_t)
	return time.Unix(0, int64(st.Atime))
}

```

// === FILE: references!/go/src/os/stat_windows.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"errors"
	"internal/filepathlite"
	"internal/syscall/windows"
	"syscall"
	"unsafe"
)

// Stat returns the [FileInfo] structure describing file.
// If there is an error, it will be of type [*PathError].
func (file *File) Stat() (FileInfo, error) {
	if file == nil {
		return nil, ErrInvalid
	}
	return statHandle(file.name, file.pfd.Sysfd)
}

// stat implements both Stat and Lstat of a file.
func stat(funcname, name string, followSurrogates bool) (FileInfo, error) {
	if len(name) == 0 {
		return nil, &PathError{Op: funcname, Path: name, Err: syscall.Errno(syscall.ERROR_PATH_NOT_FOUND)}
	}
	namep, err := syscall.UTF16PtrFromString(fixLongPath(name))
	if err != nil {
		return nil, &PathError{Op: funcname, Path: name, Err: err}
	}

	// Try GetFileAttributesEx first, because it is faster than CreateFile.
	// See https://golang.org/issues/19922#issuecomment-300031421 for details.
	var fa syscall.Win32FileAttributeData
	err = syscall.GetFileAttributesEx(namep, syscall.GetFileExInfoStandard, (*byte)(unsafe.Pointer(&fa)))
	if errors.Is(err, ErrNotExist) {
		return nil, &PathError{Op: "GetFileAttributesEx", Path: name, Err: err}
	}
	if err == nil && fa.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		// Not a surrogate for another named entity, because it isn't any kind of reparse point.
		// The information we got from GetFileAttributesEx is good enough for now.
		fs := newFileStatFromWin32FileAttributeData(&fa)
		if err := fs.saveInfoFromPath(name); err != nil {
			return nil, err
		}
		return fs, nil
	}

	// GetFileAttributesEx fails with ERROR_SHARING_VIOLATION error for
	// files like c:\pagefile.sys. Use FindFirstFile for such files.
	if err == windows.ERROR_SHARING_VIOLATION {
		var fd syscall.Win32finddata
		sh, err := syscall.FindFirstFile(namep, &fd)
		if err != nil {
			return nil, &PathError{Op: "FindFirstFile", Path: name, Err: err}
		}
		syscall.FindClose(sh)
		if fd.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
			// Not a surrogate for another named entity. FindFirstFile is good enough.
			fs := newFileStatFromWin32finddata(&fd)
			if err := fs.saveInfoFromPath(name); err != nil {
				return nil, err
			}
			return fs, nil
		}
	}

	// Use CreateFile to determine whether the file is a name surrogate and, if so,
	// save information about the link target.
	// Set FILE_FLAG_BACKUP_SEMANTICS so that CreateFile will create the handle
	// even if name refers to a directory.
	var flags uint32 = syscall.FILE_FLAG_BACKUP_SEMANTICS | syscall.FILE_FLAG_OPEN_REPARSE_POINT
	h, err := syscall.CreateFile(namep, 0, 0, nil, syscall.OPEN_EXISTING, flags, 0)

	if err == windows.ERROR_INVALID_PARAMETER {
		// Console handles, like "\\.\con", require generic read access. See
		// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew#consoles.
		// We haven't set it previously because it is normally not required
		// to read attributes and some files may not allow it.
		h, err = syscall.CreateFile(namep, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, flags, 0)
	}
	if err != nil {
		// Since CreateFile failed, we can't determine whether name refers to a
		// name surrogate, or some other kind of reparse point. Since we can't return a
		// FileInfo with a known-accurate Mode, we must return an error.
		return nil, &PathError{Op: "CreateFile", Path: name, Err: err}
	}

	fi, err := statHandle(name, h)
	syscall.CloseHandle(h)
	if err == nil && followSurrogates && fi.(*fileStat).isReparseTagNameSurrogate() {
		// To obtain information about the link target, we reopen the file without
		// FILE_FLAG_OPEN_REPARSE_POINT and examine the resulting handle.
		// (See https://devblogs.microsoft.com/oldnewthing/20100212-00/?p=14963.)
		h, err = syscall.CreateFile(namep, 0, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if err != nil {
			// name refers to a symlink, but we couldn't resolve the symlink target.
			return nil, &PathError{Op: "CreateFile", Path: name, Err: err}
		}
		defer syscall.CloseHandle(h)
		return statHandle(name, h)
	}
	return fi, err
}

func statHandle(name string, h syscall.Handle) (FileInfo, error) {
	ft, err := syscall.GetFileType(h)
	if err != nil {
		return nil, &PathError{Op: "GetFileType", Path: name, Err: err}
	}
	switch ft {
	case syscall.FILE_TYPE_PIPE, syscall.FILE_TYPE_CHAR:
		return &fileStat{name: filepathlite.Base(name), filetype: ft}, nil
	}
	fs, err := newFileStatFromGetFileInformationByHandle(name, h)
	if err != nil {
		return nil, err
	}
	fs.filetype = ft
	return fs, nil
}

// statNolog implements Stat for Windows.
func statNolog(name string) (FileInfo, error) {
	return stat("Stat", name, true)
}

// lstatNolog implements Lstat for Windows.
func lstatNolog(name string) (FileInfo, error) {
	followSurrogates := false
	if name != "" && IsPathSeparator(name[len(name)-1]) {
		// We try to implement POSIX semantics for Lstat path resolution
		// (per https://pubs.opengroup.org/onlinepubs/9699919799.2013edition/basedefs/V1_chap04.html#tag_04_12):
		// symlinks before the last separator in the path must be resolved. Since
		// the last separator in this case follows the last path element, we should
		// follow symlinks in the last path element.
		followSurrogates = true
	}
	return stat("Lstat", name, followSurrogates)
}

```

// === FILE: references!/go/src/os/statat.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows

package os

import (
	"internal/testlog"
)

func (f *File) lstatat(name string) (FileInfo, error) {
	if stathook != nil {
		fi, err := stathook(f, name)
		if fi != nil || err != nil {
			return fi, err
		}
	}
	if log := testlog.Logger(); log != nil {
		log.Stat(joinPath(f.Name(), name))
	}
	return f.lstatatNolog(name)
}

```

// === FILE: references!/go/src/os/statat_other.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (js && wasm) || plan9

package os

func (f *File) lstatatNolog(name string) (FileInfo, error) {
	// These platforms don't have fstatat, so use stat instead.
	return Lstat(f.name + "/" + name)
}

```

// === FILE: references!/go/src/os/statat_unix.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || darwin || dragonfly || freebsd || wasip1 || linux || netbsd || openbsd || solaris

package os

import (
	"internal/syscall/unix"
)

func (f *File) lstatatNolog(name string) (FileInfo, error) {
	var fs fileStat
	if err := f.pfd.Fstatat(name, &fs.sys, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return nil, f.wrapErr("fstatat", err)
	}
	fillFileStatFromSys(&fs, name)
	return &fs, nil
}

```

// === FILE: references!/go/src/os/sticky_bsd.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || netbsd || openbsd || solaris || wasip1

package os

// According to sticky(8), neither open(2) nor mkdir(2) will create
// a file with the sticky bit set.
const supportsCreateWithStickyBit = false

```

// === FILE: references!/go/src/os/sticky_notbsd.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !aix && !darwin && !dragonfly && !freebsd && !js && !netbsd && !openbsd && !solaris && !wasip1

package os

const supportsCreateWithStickyBit = true

```

// === FILE: references!/go/src/os/sys.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

// Hostname returns the host name reported by the kernel.
func Hostname() (name string, err error) {
	return hostname()
}

```

// === FILE: references!/go/src/os/sys_aix.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import "syscall"

// gethostname syscall cannot be used because it also returns the domain.
// Therefore, hostname is retrieve with uname syscall and the Nodename field.

func hostname() (name string, err error) {
	var u syscall.Utsname
	if errno := syscall.Uname(&u); errno != nil {
		return "", NewSyscallError("uname", errno)
	}
	b := make([]byte, len(u.Nodename))
	i := 0
	for ; i < len(u.Nodename); i++ {
		if u.Nodename[i] == 0 {
			break
		}
		b[i] = byte(u.Nodename[i])
	}
	return string(b[:i]), nil
}

```

// === FILE: references!/go/src/os/sys_bsd.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin || dragonfly || freebsd || (js && wasm) || netbsd || openbsd || wasip1

package os

import "syscall"

func hostname() (name string, err error) {
	name, err = syscall.Sysctl("kern.hostname")
	if err != nil {
		return "", NewSyscallError("sysctl kern.hostname", err)
	}
	return name, nil
}

```

// === FILE: references!/go/src/os/sys_js.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build js && wasm

package os

// supportsCloseOnExec reports whether the platform supports the
// O_CLOEXEC flag.
const supportsCloseOnExec = false

```

// === FILE: references!/go/src/os/sys_linux.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"runtime"
	"syscall"
)

func hostname() (name string, err error) {
	// Try uname first, as it's only one system call and reading
	// from /proc is not allowed on Android.
	var un syscall.Utsname
	err = syscall.Uname(&un)

	var buf [512]byte // Enough for a DNS name.
	for i, b := range un.Nodename[:] {
		buf[i] = uint8(b)
		if b == 0 {
			name = string(buf[:i])
			break
		}
	}
	// If we got a name and it's not potentially truncated
	// (Nodename is 65 bytes), return it.
	if err == nil && len(name) > 0 && len(name) < 64 {
		return name, nil
	}
	if runtime.GOOS == "android" {
		if name != "" {
			return name, nil
		}
		return "localhost", nil
	}

	f, err := Open("/proc/sys/kernel/hostname")
	if err != nil {
		return "", err
	}
	defer f.Close()

	n, err := f.Read(buf[:])
	if err != nil {
		return "", err
	}

	if n > 0 && buf[n-1] == '\n' {
		n--
	}
	return string(buf[:n]), nil
}

```

// === FILE: references!/go/src/os/sys_plan9.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

func hostname() (name string, err error) {
	f, err := Open("#c/sysname")
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf [128]byte
	n, err := f.Read(buf[:len(buf)-1])

	if err != nil {
		return "", err
	}
	if n > 0 {
		buf[n] = 0
	}
	return string(buf[0:n]), nil
}

```

// === FILE: references!/go/src/os/sys_solaris.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import "syscall"

func hostname() (name string, err error) {
	return syscall.Gethostname()
}

```

// === FILE: references!/go/src/os/sys_unix.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package os

// supportsCloseOnExec reports whether the platform supports the
// O_CLOEXEC flag.
// On Darwin, the O_CLOEXEC flag was introduced in OS X 10.7 (Darwin 11.0.0).
// See https://support.apple.com/kb/HT1633.
// On FreeBSD, the O_CLOEXEC flag was introduced in version 8.3.
const supportsCloseOnExec = true

```

// === FILE: references!/go/src/os/sys_wasip1.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build wasip1

package os

// supportsCloseOnExec reports whether the platform supports the
// O_CLOEXEC flag.
const supportsCloseOnExec = false

```

// === FILE: references!/go/src/os/sys_windows.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/syscall/windows"
	"syscall"
)

func hostname() (name string, err error) {
	// Use PhysicalDnsHostname to uniquely identify host in a cluster
	const format = windows.ComputerNamePhysicalDnsHostname

	n := uint32(64)
	for {
		b := make([]uint16, n)
		err := windows.GetComputerNameEx(format, &b[0], &n)
		if err == nil {
			return syscall.UTF16ToString(b[:n]), nil
		}
		if err != syscall.ERROR_MORE_DATA {
			return "", NewSyscallError("ComputerNameEx", err)
		}

		// If we received an ERROR_MORE_DATA, but n doesn't get larger,
		// something has gone wrong and we may be in an infinite loop
		if n <= uint32(len(b)) {
			return "", NewSyscallError("ComputerNameEx", err)
		}
	}
}

```

// === FILE: references!/go/src/os/tempfile.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"errors"
	"internal/bytealg"
	"internal/strconv"
	_ "unsafe" // for go:linkname
)

// random number source provided by runtime.
// We generate random temporary file names so that there's a good
// chance the file doesn't exist yet - keeps the number of tries in
// TempFile to a minimum.
//
//go:linkname runtime_rand runtime.rand
func runtime_rand() uint64

func nextRandom() string {
	return strconv.FormatUint(uint64(uint32(runtime_rand())), 10)
}

// CreateTemp creates a new temporary file in the directory dir,
// opens the file for reading and writing, and returns the resulting file.
// The filename is generated by taking pattern and adding a random string to the end.
// If pattern includes a "*", the random string replaces the last "*".
// The file is created with mode 0o600 (before umask).
// If dir is the empty string, CreateTemp uses the default directory for temporary files, as returned by [TempDir].
// Multiple programs or goroutines calling CreateTemp simultaneously will not choose the same file.
// The caller can use the file's Name method to find the pathname of the file.
// It is the caller's responsibility to remove the file when it is no longer needed.
func CreateTemp(dir, pattern string) (*File, error) {
	if dir == "" {
		dir = TempDir()
	}

	prefix, suffix, err := prefixAndSuffix(pattern)
	if err != nil {
		return nil, &PathError{Op: "createtemp", Path: pattern, Err: err}
	}
	prefix = joinPath(dir, prefix)

	try := 0
	for {
		name := prefix + nextRandom() + suffix
		f, err := OpenFile(name, O_RDWR|O_CREATE|O_EXCL, 0600)
		if IsExist(err) {
			if try++; try < 10000 {
				continue
			}
			return nil, &PathError{Op: "createtemp", Path: prefix + "*" + suffix, Err: ErrExist}
		}
		return f, err
	}
}

var errPatternHasSeparator = errors.New("pattern contains path separator")

// prefixAndSuffix splits pattern by the last wildcard "*", if applicable,
// returning prefix as the part before "*" and suffix as the part after "*".
func prefixAndSuffix(pattern string) (prefix, suffix string, err error) {
	for i := 0; i < len(pattern); i++ {
		if IsPathSeparator(pattern[i]) {
			return "", "", errPatternHasSeparator
		}
	}
	if pos := bytealg.LastIndexByteString(pattern, '*'); pos != -1 {
		prefix, suffix = pattern[:pos], pattern[pos+1:]
	} else {
		prefix = pattern
	}
	return prefix, suffix, nil
}

// MkdirTemp creates a new temporary directory in the directory dir
// and returns the pathname of the new directory.
// The new directory's name is generated by adding a random string to the end of pattern.
// If pattern includes a "*", the random string replaces the last "*" instead.
// The directory is created with mode 0o700 (before umask).
// If dir is the empty string, MkdirTemp uses the default directory for temporary files, as returned by TempDir.
// Multiple programs or goroutines calling MkdirTemp simultaneously will not choose the same directory.
// It is the caller's responsibility to remove the directory when it is no longer needed.
func MkdirTemp(dir, pattern string) (string, error) {
	if dir == "" {
		dir = TempDir()
	}

	prefix, suffix, err := prefixAndSuffix(pattern)
	if err != nil {
		return "", &PathError{Op: "mkdirtemp", Path: pattern, Err: err}
	}
	prefix = joinPath(dir, prefix)

	try := 0
	for {
		name := prefix + nextRandom() + suffix
		err := Mkdir(name, 0700)
		if err == nil {
			return name, nil
		}
		if IsExist(err) {
			if try++; try < 10000 {
				continue
			}
			return "", &PathError{Op: "mkdirtemp", Path: prefix + "*" + suffix, Err: ErrExist}
		}
		if IsNotExist(err) {
			if _, err := Stat(dir); IsNotExist(err) {
				return "", err
			}
		}
		return "", err
	}
}

func joinPath(dir, name string) string {
	if len(dir) > 0 && IsPathSeparator(dir[len(dir)-1]) {
		return dir + name
	}
	return dir + string(PathSeparator) + name
}

```

// === FILE: references!/go/src/os/types.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"io/fs"
	"syscall"
)

// Getpagesize returns the underlying system's memory page size.
func Getpagesize() int { return syscall.Getpagesize() }

// File represents an open file descriptor.
//
// The methods of File are safe for concurrent use.
type File struct {
	*file // os specific
}

// A FileInfo describes a file and is returned by [Stat] and [Lstat].
type FileInfo = fs.FileInfo

// A FileMode represents a file's mode and permission bits.
// The bits have the same definition on all systems, so that
// information about files can be moved from one system
// to another portably. Not all bits apply to all systems.
// The only required bit is [ModeDir] for directories.
type FileMode = fs.FileMode

// The defined file mode bits are the most significant bits of the [FileMode].
// The nine least-significant bits are the standard Unix rwxrwxrwx permissions.
// The values of these bits should be considered part of the public API and
// may be used in wire protocols or disk representations: they must not be
// changed, although new bits might be added.
const (
	// The single letters are the abbreviations
	// used by the String method's formatting.
	ModeDir        = fs.ModeDir        // d: is a directory
	ModeAppend     = fs.ModeAppend     // a: append-only
	ModeExclusive  = fs.ModeExclusive  // l: exclusive use
	ModeTemporary  = fs.ModeTemporary  // T: temporary file; Plan 9 only
	ModeSymlink    = fs.ModeSymlink    // L: symbolic link
	ModeDevice     = fs.ModeDevice     // D: device file
	ModeNamedPipe  = fs.ModeNamedPipe  // p: named pipe (FIFO)
	ModeSocket     = fs.ModeSocket     // S: Unix domain socket
	ModeSetuid     = fs.ModeSetuid     // u: setuid
	ModeSetgid     = fs.ModeSetgid     // g: setgid
	ModeCharDevice = fs.ModeCharDevice // c: Unix character device, when ModeDevice is set
	ModeSticky     = fs.ModeSticky     // t: sticky
	ModeIrregular  = fs.ModeIrregular  // ?: non-regular file; nothing else is known about this file

	// Mask for the type bits. For regular files, none will be set.
	ModeType = fs.ModeType

	ModePerm = fs.ModePerm // Unix permission bits, 0o777
)

func (fs *fileStat) Name() string { return fs.name }
func (fs *fileStat) IsDir() bool  { return fs.Mode().IsDir() }

// SameFile reports whether fi1 and fi2 describe the same file.
// For example, on Unix this means that the device and inode fields
// of the two underlying structures are identical; on other systems
// the decision may be based on the path names.
// SameFile only applies to results returned by this package's [Stat].
// It returns false in other cases.
func SameFile(fi1, fi2 FileInfo) bool {
	fs1, ok1 := fi1.(*fileStat)
	fs2, ok2 := fi2.(*fileStat)
	if !ok1 || !ok2 {
		return false
	}
	return sameFile(fs1, fs2)
}

```

// === FILE: references!/go/src/os/types_plan9.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"time"
)

// A fileStat is the implementation of FileInfo returned by Stat and Lstat.
type fileStat struct {
	name    string
	size    int64
	mode    FileMode
	modTime time.Time
	sys     any
}

func (fs *fileStat) Size() int64        { return fs.size }
func (fs *fileStat) Mode() FileMode     { return fs.mode }
func (fs *fileStat) ModTime() time.Time { return fs.modTime }
func (fs *fileStat) Sys() any           { return fs.sys }

func sameFile(fs1, fs2 *fileStat) bool {
	a := fs1.sys.(*syscall.Dir)
	b := fs2.sys.(*syscall.Dir)
	return a.Qid.Path == b.Qid.Path && a.Type == b.Type && a.Dev == b.Dev
}

```

// === FILE: references!/go/src/os/types_unix.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows && !plan9

package os

import (
	"syscall"
	"time"
)

// A fileStat is the implementation of FileInfo returned by Stat and Lstat.
type fileStat struct {
	name    string
	size    int64
	mode    FileMode
	modTime time.Time
	sys     syscall.Stat_t
}

func (fs *fileStat) Size() int64        { return fs.size }
func (fs *fileStat) Mode() FileMode     { return fs.mode }
func (fs *fileStat) ModTime() time.Time { return fs.modTime }
func (fs *fileStat) Sys() any           { return &fs.sys }

func sameFile(fs1, fs2 *fileStat) bool {
	return fs1.sys.Dev == fs2.sys.Dev && fs1.sys.Ino == fs2.sys.Ino
}

```

// === FILE: references!/go/src/os/types_windows.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/filepathlite"
	"internal/godebug"
	"internal/syscall/windows"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// A fileStat is the implementation of FileInfo returned by Stat and Lstat.
type fileStat struct {
	name string

	// from ByHandleFileInformation, Win32FileAttributeData, Win32finddata, and GetFileInformationByHandleEx
	FileAttributes uint32
	CreationTime   syscall.Filetime
	LastAccessTime syscall.Filetime
	LastWriteTime  syscall.Filetime
	FileSizeHigh   uint32
	FileSizeLow    uint32

	// from Win32finddata and GetFileInformationByHandleEx
	ReparseTag uint32

	// what syscall.GetFileType returns
	filetype uint32

	// used to implement SameFile
	sync.Mutex
	path             string
	vol              uint32
	idxhi            uint32
	idxlo            uint32
	appendNameToPath bool
}

// newFileStatFromGetFileInformationByHandle calls GetFileInformationByHandle
// to gather all required information about the file handle h.
func newFileStatFromGetFileInformationByHandle(path string, h syscall.Handle) (fs *fileStat, err error) {
	var d syscall.ByHandleFileInformation
	err = syscall.GetFileInformationByHandle(h, &d)
	if err != nil {
		return nil, &PathError{Op: "GetFileInformationByHandle", Path: path, Err: err}
	}

	var reparseTag uint32
	if d.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		var ti windows.FILE_ATTRIBUTE_TAG_INFO
		err = windows.GetFileInformationByHandleEx(h, windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&ti)), uint32(unsafe.Sizeof(ti)))
		if err != nil {
			return nil, &PathError{Op: "GetFileInformationByHandleEx", Path: path, Err: err}
		}
		reparseTag = ti.ReparseTag
	}

	return &fileStat{
		name:           filepathlite.Base(path),
		FileAttributes: d.FileAttributes,
		CreationTime:   d.CreationTime,
		LastAccessTime: d.LastAccessTime,
		LastWriteTime:  d.LastWriteTime,
		FileSizeHigh:   d.FileSizeHigh,
		FileSizeLow:    d.FileSizeLow,
		vol:            d.VolumeSerialNumber,
		idxhi:          d.FileIndexHigh,
		idxlo:          d.FileIndexLow,
		ReparseTag:     reparseTag,
		// fileStat.path is used by os.SameFile to decide if it needs
		// to fetch vol, idxhi and idxlo. But these are already set,
		// so set fileStat.path to "" to prevent os.SameFile doing it again.
	}, nil
}

// newFileStatFromWin32FileAttributeData copies all required information
// from syscall.Win32FileAttributeData d into the newly created fileStat.
func newFileStatFromWin32FileAttributeData(d *syscall.Win32FileAttributeData) *fileStat {
	return &fileStat{
		FileAttributes: d.FileAttributes,
		CreationTime:   d.CreationTime,
		LastAccessTime: d.LastAccessTime,
		LastWriteTime:  d.LastWriteTime,
		FileSizeHigh:   d.FileSizeHigh,
		FileSizeLow:    d.FileSizeLow,
	}
}

// newFileStatFromFileIDBothDirInfo copies all required information
// from windows.FILE_ID_BOTH_DIR_INFO d into the newly created fileStat.
func newFileStatFromFileIDBothDirInfo(d *windows.FILE_ID_BOTH_DIR_INFO) *fileStat {
	// The FILE_ID_BOTH_DIR_INFO MSDN documentations isn't completely correct.
	// FileAttributes can contain any file attributes that is currently set on the file,
	// not just the ones documented.
	// EaSize contains the reparse tag if the file is a reparse point.
	return &fileStat{
		FileAttributes: d.FileAttributes,
		CreationTime:   d.CreationTime,
		LastAccessTime: d.LastAccessTime,
		LastWriteTime:  d.LastWriteTime,
		FileSizeHigh:   uint32(d.EndOfFile >> 32),
		FileSizeLow:    uint32(d.EndOfFile),
		ReparseTag:     d.EaSize,
		idxhi:          uint32(d.FileID >> 32),
		idxlo:          uint32(d.FileID),
	}
}

// newFileStatFromFileFullDirInfo copies all required information
// from windows.FILE_FULL_DIR_INFO d into the newly created fileStat.
func newFileStatFromFileFullDirInfo(d *windows.FILE_FULL_DIR_INFO) *fileStat {
	return &fileStat{
		FileAttributes: d.FileAttributes,
		CreationTime:   d.CreationTime,
		LastAccessTime: d.LastAccessTime,
		LastWriteTime:  d.LastWriteTime,
		FileSizeHigh:   uint32(d.EndOfFile >> 32),
		FileSizeLow:    uint32(d.EndOfFile),
		ReparseTag:     d.EaSize,
	}
}

// newFileStatFromWin32finddata copies all required information
// from syscall.Win32finddata d into the newly created fileStat.
func newFileStatFromWin32finddata(d *syscall.Win32finddata) *fileStat {
	fs := &fileStat{
		FileAttributes: d.FileAttributes,
		CreationTime:   d.CreationTime,
		LastAccessTime: d.LastAccessTime,
		LastWriteTime:  d.LastWriteTime,
		FileSizeHigh:   d.FileSizeHigh,
		FileSizeLow:    d.FileSizeLow,
	}
	if d.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		// Per https://learn.microsoft.com/en-us/windows/win32/api/minwinbase/ns-minwinbase-win32_find_dataw:
		// “If the dwFileAttributes member includes the FILE_ATTRIBUTE_REPARSE_POINT
		// attribute, this member specifies the reparse point tag. Otherwise, this
		// value is undefined and should not be used.”
		fs.ReparseTag = d.Reserved0
	}
	return fs
}

// isReparseTagNameSurrogate determines whether a tag's associated
// reparse point is a surrogate for another named entity (for example, a mounted folder).
//
// See https://learn.microsoft.com/en-us/windows/win32/api/winnt/nf-winnt-isreparsetagnamesurrogate
// and https://learn.microsoft.com/en-us/windows/win32/fileio/reparse-point-tags.
func (fs *fileStat) isReparseTagNameSurrogate() bool {
	// True for IO_REPARSE_TAG_SYMLINK and IO_REPARSE_TAG_MOUNT_POINT.
	return fs.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 && fs.ReparseTag&0x20000000 != 0
}

func (fs *fileStat) Size() int64 {
	return int64(fs.FileSizeHigh)<<32 + int64(fs.FileSizeLow)
}

var winsymlink = godebug.New("winsymlink")

func (fs *fileStat) Mode() FileMode {
	m := fs.mode()
	if winsymlink.Value() == "0" {
		old := fs.modePreGo1_23()
		if old != m {
			winsymlink.IncNonDefault()
			m = old
		}
	}
	return m
}

func (fs *fileStat) mode() (m FileMode) {
	if fs.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		m |= 0444
	} else {
		m |= 0666
	}

	// Windows reports the FILE_ATTRIBUTE_DIRECTORY bit for reparse points
	// that refer to directories, such as symlinks and mount points.
	// However, we follow symlink POSIX semantics and do not set the mode bits.
	// This allows users to walk directories without following links
	// by just calling "fi, err := os.Lstat(name); err == nil && fi.IsDir()".
	// Note that POSIX only defines the semantics for symlinks, not for
	// mount points or other surrogate reparse points, but we treat them
	// the same way for consistency. Also, mount points can contain infinite
	// loops, so it is not safe to walk them without special handling.
	if !fs.isReparseTagNameSurrogate() {
		if fs.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
			m |= ModeDir | 0111
		}

		switch fs.filetype {
		case syscall.FILE_TYPE_PIPE:
			m |= ModeNamedPipe
		case syscall.FILE_TYPE_CHAR:
			m |= ModeDevice | ModeCharDevice
		}
	}

	if fs.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		switch fs.ReparseTag {
		case syscall.IO_REPARSE_TAG_SYMLINK:
			m |= ModeSymlink
		case windows.IO_REPARSE_TAG_AF_UNIX:
			m |= ModeSocket
		case windows.IO_REPARSE_TAG_DEDUP:
			// If the Data Deduplication service is enabled on Windows Server, its
			// Optimization job may convert regular files to IO_REPARSE_TAG_DEDUP
			// whenever that job runs.
			//
			// However, DEDUP reparse points remain similar in most respects to
			// regular files: they continue to support random-access reads and writes
			// of persistent data, and they shouldn't add unexpected latency or
			// unavailability in the way that a network filesystem might.
			//
			// Go programs may use ModeIrregular to filter out unusual files (such as
			// raw device files on Linux, POSIX FIFO special files, and so on), so
			// to avoid files changing unpredictably from regular to irregular we will
			// consider DEDUP files to be close enough to regular to treat as such.
		default:
			m |= ModeIrregular
		}
	}
	return
}

// modePreGo1_23 returns the FileMode for the fileStat, using the pre-Go 1.23
// logic for determining the file mode.
// The logic is subtle and not well-documented, so it is better to keep it
// separate from the new logic.
func (fs *fileStat) modePreGo1_23() (m FileMode) {
	if fs.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		m |= 0444
	} else {
		m |= 0666
	}
	if fs.ReparseTag == syscall.IO_REPARSE_TAG_SYMLINK ||
		fs.ReparseTag == windows.IO_REPARSE_TAG_MOUNT_POINT {
		return m | ModeSymlink
	}
	if fs.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		m |= ModeDir | 0111
	}
	switch fs.filetype {
	case syscall.FILE_TYPE_PIPE:
		m |= ModeNamedPipe
	case syscall.FILE_TYPE_CHAR:
		m |= ModeDevice | ModeCharDevice
	}
	if fs.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		if fs.ReparseTag == windows.IO_REPARSE_TAG_AF_UNIX {
			m |= ModeSocket
		}
		if m&ModeType == 0 {
			if fs.ReparseTag == windows.IO_REPARSE_TAG_DEDUP {
				// See comment in fs.Mode.
			} else {
				m |= ModeIrregular
			}
		}
	}
	return m
}

func (fs *fileStat) ModTime() time.Time {
	return time.Unix(0, fs.LastWriteTime.Nanoseconds())
}

// Sys returns syscall.Win32FileAttributeData for file fs.
func (fs *fileStat) Sys() any {
	return &syscall.Win32FileAttributeData{
		FileAttributes: fs.FileAttributes,
		CreationTime:   fs.CreationTime,
		LastAccessTime: fs.LastAccessTime,
		LastWriteTime:  fs.LastWriteTime,
		FileSizeHigh:   fs.FileSizeHigh,
		FileSizeLow:    fs.FileSizeLow,
	}
}

func (fs *fileStat) loadFileId() error {
	fs.Lock()
	defer fs.Unlock()
	if fs.path == "" {
		// already done
		return nil
	}
	var path string
	if fs.appendNameToPath {
		path = fixLongPath(fs.path + `\` + fs.name)
	} else {
		path = fs.path
	}
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	// Per https://learn.microsoft.com/en-us/windows/win32/fileio/reparse-points-and-file-operations,
	// “Applications that use the CreateFile function should specify the
	// FILE_FLAG_OPEN_REPARSE_POINT flag when opening the file if it is a reparse
	// point.”
	//
	// And per https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew,
	// “If the file is not a reparse point, then this flag is ignored.”
	//
	// So we set FILE_FLAG_OPEN_REPARSE_POINT unconditionally, since we want
	// information about the reparse point itself.
	//
	// If the file is a symlink, the symlink target should have already been
	// resolved when the fileStat was created, so we don't need to worry about
	// resolving symlink reparse points again here.
	attrs := uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS | syscall.FILE_FLAG_OPEN_REPARSE_POINT)

	h, err := syscall.CreateFile(pathp, 0, 0, nil, syscall.OPEN_EXISTING, attrs, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)
	var i syscall.ByHandleFileInformation
	err = syscall.GetFileInformationByHandle(h, &i)
	if err != nil {
		return err
	}
	fs.path = ""
	fs.vol = i.VolumeSerialNumber
	fs.idxhi = i.FileIndexHigh
	fs.idxlo = i.FileIndexLow
	return nil
}

// saveInfoFromPath saves full path of the file to be used by os.SameFile later,
// and set name from path.
func (fs *fileStat) saveInfoFromPath(path string) error {
	fs.path = path
	if !filepathlite.IsAbs(fs.path) {
		var err error
		fs.path, err = syscall.FullPath(fs.path)
		if err != nil {
			return &PathError{Op: "FullPath", Path: path, Err: err}
		}
	}
	fs.name = filepathlite.Base(path)
	return nil
}

func sameFile(fs1, fs2 *fileStat) bool {
	e := fs1.loadFileId()
	if e != nil {
		return false
	}
	e = fs2.loadFileId()
	if e != nil {
		return false
	}
	return fs1.vol == fs2.vol && fs1.idxhi == fs2.idxhi && fs1.idxlo == fs2.idxlo
}

// For testing.
func atime(fi FileInfo) time.Time {
	return time.Unix(0, fi.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds())
}

```

// === FILE: references!/go/src/os/user/cgo_listgroups_unix.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (cgo || darwin) && !osusergo && unix && !android && !aix

package user

import (
	"fmt"
	"strconv"
	"unsafe"
)

const maxGroups = 2048

func listGroups(u *User) ([]string, error) {
	ug, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, fmt.Errorf("user: list groups for %s: invalid gid %q", u.Username, u.Gid)
	}
	userGID := _C_gid_t(ug)
	nameC := make([]byte, len(u.Username)+1)
	copy(nameC, u.Username)

	n := _C_int(256)
	gidsC := make([]_C_gid_t, n)
	rv := getGroupList((*_C_char)(unsafe.Pointer(&nameC[0])), userGID, &gidsC[0], &n)
	if rv == -1 {
		// Mac is the only Unix that does not set n properly when rv == -1, so
		// we need to use different logic for Mac vs. the other OS's.
		if err := groupRetry(u.Username, nameC, userGID, &gidsC, &n); err != nil {
			return nil, err
		}
	}
	gidsC = gidsC[:n]
	gids := make([]string, 0, n)
	for _, g := range gidsC[:n] {
		gids = append(gids, strconv.Itoa(int(g)))
	}
	return gids, nil
}

// groupRetry retries getGroupList with much larger size for n. The result is
// stored in gids.
func groupRetry(username string, name []byte, userGID _C_gid_t, gids *[]_C_gid_t, n *_C_int) error {
	// More than initial buffer, but now n contains the correct size.
	if *n > maxGroups {
		return fmt.Errorf("user: %q is a member of more than %d groups", username, maxGroups)
	}
	*gids = make([]_C_gid_t, *n)
	rv := getGroupList((*_C_char)(unsafe.Pointer(&name[0])), userGID, &(*gids)[0], n)
	if rv == -1 {
		return fmt.Errorf("user: list groups for %s failed", username)
	}
	return nil
}

```

// === FILE: references!/go/src/os/user/cgo_lookup_cgo.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build cgo && !osusergo && unix && !android && !darwin

package user

import (
	"syscall"
)

/*
#cgo solaris CFLAGS: -D_POSIX_PTHREAD_SEMANTICS
#cgo CFLAGS: -fno-stack-protector
#include <unistd.h>
#include <sys/types.h>
#include <pwd.h>
#include <grp.h>
#include <stdlib.h>
#include <string.h>

static struct passwd mygetpwuid_r(int uid, char *buf, size_t buflen, int *found, int *perr) {
	struct passwd pwd;
	struct passwd *result;
	memset (&pwd, 0, sizeof(pwd));
	*perr = getpwuid_r(uid, &pwd, buf, buflen, &result);
	*found = result != NULL;
	return pwd;
}

static struct passwd mygetpwnam_r(const char *name, char *buf, size_t buflen, int *found, int *perr) {
	struct passwd pwd;
	struct passwd *result;
	memset(&pwd, 0, sizeof(pwd));
	*perr = getpwnam_r(name, &pwd, buf, buflen, &result);
	*found = result != NULL;
	return pwd;
}

static struct group mygetgrgid_r(int gid, char *buf, size_t buflen, int *found, int *perr) {
	struct group grp;
	struct group *result;
	memset(&grp, 0, sizeof(grp));
	*perr = getgrgid_r(gid, &grp, buf, buflen, &result);
	*found = result != NULL;
	return grp;
}

static struct group mygetgrnam_r(const char *name, char *buf, size_t buflen, int *found, int *perr) {
	struct group grp;
	struct group *result;
	memset(&grp, 0, sizeof(grp));
	*perr = getgrnam_r(name, &grp, buf, buflen, &result);
	*found = result != NULL;
	return grp;
}
*/
import "C"

type _C_char = C.char
type _C_int = C.int
type _C_gid_t = C.gid_t
type _C_uid_t = C.uid_t
type _C_size_t = C.size_t
type _C_struct_group = C.struct_group
type _C_struct_passwd = C.struct_passwd
type _C_long = C.long

func _C_pw_uid(p *_C_struct_passwd) _C_uid_t   { return p.pw_uid }
func _C_pw_uidp(p *_C_struct_passwd) *_C_uid_t { return &p.pw_uid }
func _C_pw_gid(p *_C_struct_passwd) _C_gid_t   { return p.pw_gid }
func _C_pw_gidp(p *_C_struct_passwd) *_C_gid_t { return &p.pw_gid }
func _C_pw_name(p *_C_struct_passwd) *_C_char  { return p.pw_name }
func _C_pw_gecos(p *_C_struct_passwd) *_C_char { return p.pw_gecos }
func _C_pw_dir(p *_C_struct_passwd) *_C_char   { return p.pw_dir }

func _C_gr_gid(g *_C_struct_group) _C_gid_t  { return g.gr_gid }
func _C_gr_name(g *_C_struct_group) *_C_char { return g.gr_name }

func _C_GoString(p *_C_char) string { return C.GoString(p) }

func _C_getpwnam_r(name *_C_char, buf *_C_char, size _C_size_t) (pwd _C_struct_passwd, found bool, errno syscall.Errno) {
	var f, e _C_int
	pwd = C.mygetpwnam_r(name, buf, size, &f, &e)
	return pwd, f != 0, syscall.Errno(e)
}

func _C_getpwuid_r(uid _C_uid_t, buf *_C_char, size _C_size_t) (pwd _C_struct_passwd, found bool, errno syscall.Errno) {
	var f, e _C_int
	pwd = C.mygetpwuid_r(_C_int(uid), buf, size, &f, &e)
	return pwd, f != 0, syscall.Errno(e)
}

func _C_getgrnam_r(name *_C_char, buf *_C_char, size _C_size_t) (grp _C_struct_group, found bool, errno syscall.Errno) {
	var f, e _C_int
	grp = C.mygetgrnam_r(name, buf, size, &f, &e)
	return grp, f != 0, syscall.Errno(e)
}

func _C_getgrgid_r(gid _C_gid_t, buf *_C_char, size _C_size_t) (grp _C_struct_group, found bool, errno syscall.Errno) {
	var f, e _C_int
	grp = C.mygetgrgid_r(_C_int(gid), buf, size, &f, &e)
	return grp, f != 0, syscall.Errno(e)
}

const (
	_C__SC_GETPW_R_SIZE_MAX = C._SC_GETPW_R_SIZE_MAX
	_C__SC_GETGR_R_SIZE_MAX = C._SC_GETGR_R_SIZE_MAX
)

func _C_sysconf(key _C_int) _C_long { return C.sysconf(key) }

```

// === FILE: references!/go/src/os/user/cgo_lookup_syscall.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !osusergo && darwin

package user

import (
	"internal/syscall/unix"
	"syscall"
)

type _C_char = byte
type _C_int = int32
type _C_gid_t = uint32
type _C_uid_t = uint32
type _C_size_t = uintptr
type _C_struct_group = unix.Group
type _C_struct_passwd = unix.Passwd
type _C_long = int64

func _C_pw_uid(p *_C_struct_passwd) _C_uid_t   { return p.Uid }
func _C_pw_uidp(p *_C_struct_passwd) *_C_uid_t { return &p.Uid }
func _C_pw_gid(p *_C_struct_passwd) _C_gid_t   { return p.Gid }
func _C_pw_gidp(p *_C_struct_passwd) *_C_gid_t { return &p.Gid }
func _C_pw_name(p *_C_struct_passwd) *_C_char  { return p.Name }
func _C_pw_gecos(p *_C_struct_passwd) *_C_char { return p.Gecos }
func _C_pw_dir(p *_C_struct_passwd) *_C_char   { return p.Dir }

func _C_gr_gid(g *_C_struct_group) _C_gid_t  { return g.Gid }
func _C_gr_name(g *_C_struct_group) *_C_char { return g.Name }

func _C_GoString(p *_C_char) string { return unix.GoString(p) }

func _C_getpwnam_r(name *_C_char, buf *_C_char, size _C_size_t) (pwd _C_struct_passwd, found bool, errno syscall.Errno) {
	var result *_C_struct_passwd
	errno = unix.Getpwnam(name, &pwd, buf, size, &result)
	return pwd, result != nil, errno
}

func _C_getpwuid_r(uid _C_uid_t, buf *_C_char, size _C_size_t) (pwd _C_struct_passwd, found bool, errno syscall.Errno) {
	var result *_C_struct_passwd
	errno = unix.Getpwuid(uid, &pwd, buf, size, &result)
	return pwd, result != nil, errno
}

func _C_getgrnam_r(name *_C_char, buf *_C_char, size _C_size_t) (grp _C_struct_group, found bool, errno syscall.Errno) {
	var result *_C_struct_group
	errno = unix.Getgrnam(name, &grp, buf, size, &result)
	return grp, result != nil, errno
}

func _C_getgrgid_r(gid _C_gid_t, buf *_C_char, size _C_size_t) (grp _C_struct_group, found bool, errno syscall.Errno) {
	var result *_C_struct_group
	errno = unix.Getgrgid(gid, &grp, buf, size, &result)
	return grp, result != nil, errno
}

const (
	_C__SC_GETPW_R_SIZE_MAX = unix.SC_GETPW_R_SIZE_MAX
	_C__SC_GETGR_R_SIZE_MAX = unix.SC_GETGR_R_SIZE_MAX
)

func _C_sysconf(key _C_int) _C_long { return unix.Sysconf(key) }

```

// === FILE: references!/go/src/os/user/cgo_lookup_unix.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (cgo || darwin) && !osusergo && unix && !android

package user

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

func current() (*User, error) {
	return lookupUnixUid(syscall.Getuid())
}

func lookupUser(username string) (*User, error) {
	var pwd _C_struct_passwd
	var found bool
	nameC := make([]byte, len(username)+1)
	copy(nameC, username)

	err := retryWithBuffer(userBuffer, func(buf []byte) syscall.Errno {
		var errno syscall.Errno
		pwd, found, errno = _C_getpwnam_r((*_C_char)(unsafe.Pointer(&nameC[0])),
			(*_C_char)(unsafe.Pointer(&buf[0])), _C_size_t(len(buf)))
		return errno
	})
	if err == syscall.ENOENT || (err == nil && !found) {
		return nil, UnknownUserError(username)
	}
	if err != nil {
		return nil, fmt.Errorf("user: lookup username %s: %v", username, err)
	}
	return buildUser(&pwd), nil
}

func lookupUserId(uid string) (*User, error) {
	i, e := strconv.Atoi(uid)
	if e != nil {
		return nil, e
	}
	return lookupUnixUid(i)
}

func lookupUnixUid(uid int) (*User, error) {
	var pwd _C_struct_passwd
	var found bool

	err := retryWithBuffer(userBuffer, func(buf []byte) syscall.Errno {
		var errno syscall.Errno
		pwd, found, errno = _C_getpwuid_r(_C_uid_t(uid),
			(*_C_char)(unsafe.Pointer(&buf[0])), _C_size_t(len(buf)))
		return errno
	})
	if err == syscall.ENOENT || (err == nil && !found) {
		return nil, UnknownUserIdError(uid)
	}
	if err != nil {
		return nil, fmt.Errorf("user: lookup userid %d: %v", uid, err)
	}
	return buildUser(&pwd), nil
}

func buildUser(pwd *_C_struct_passwd) *User {
	u := &User{
		Uid:      strconv.FormatUint(uint64(_C_pw_uid(pwd)), 10),
		Gid:      strconv.FormatUint(uint64(_C_pw_gid(pwd)), 10),
		Username: _C_GoString(_C_pw_name(pwd)),
		Name:     _C_GoString(_C_pw_gecos(pwd)),
		HomeDir:  _C_GoString(_C_pw_dir(pwd)),
	}
	// The pw_gecos field isn't quite standardized. Some docs
	// say: "It is expected to be a comma separated list of
	// personal data where the first item is the full name of the
	// user."
	u.Name, _, _ = strings.Cut(u.Name, ",")
	return u
}

func lookupGroup(groupname string) (*Group, error) {
	var grp _C_struct_group
	var found bool

	cname := make([]byte, len(groupname)+1)
	copy(cname, groupname)

	err := retryWithBuffer(groupBuffer, func(buf []byte) syscall.Errno {
		var errno syscall.Errno
		grp, found, errno = _C_getgrnam_r((*_C_char)(unsafe.Pointer(&cname[0])),
			(*_C_char)(unsafe.Pointer(&buf[0])), _C_size_t(len(buf)))
		return errno
	})
	if err == syscall.ENOENT || (err == nil && !found) {
		return nil, UnknownGroupError(groupname)
	}
	if err != nil {
		return nil, fmt.Errorf("user: lookup groupname %s: %v", groupname, err)
	}
	return buildGroup(&grp), nil
}

func lookupGroupId(gid string) (*Group, error) {
	i, e := strconv.Atoi(gid)
	if e != nil {
		return nil, e
	}
	return lookupUnixGid(i)
}

func lookupUnixGid(gid int) (*Group, error) {
	var grp _C_struct_group
	var found bool

	err := retryWithBuffer(groupBuffer, func(buf []byte) syscall.Errno {
		var errno syscall.Errno
		grp, found, errno = _C_getgrgid_r(_C_gid_t(gid),
			(*_C_char)(unsafe.Pointer(&buf[0])), _C_size_t(len(buf)))
		return syscall.Errno(errno)
	})
	if err == syscall.ENOENT || (err == nil && !found) {
		return nil, UnknownGroupIdError(strconv.Itoa(gid))
	}
	if err != nil {
		return nil, fmt.Errorf("user: lookup groupid %d: %v", gid, err)
	}
	return buildGroup(&grp), nil
}

func buildGroup(grp *_C_struct_group) *Group {
	g := &Group{
		Gid:  strconv.Itoa(int(_C_gr_gid(grp))),
		Name: _C_GoString(_C_gr_name(grp)),
	}
	return g
}

type bufferKind _C_int

var (
	userBuffer  = bufferKind(_C__SC_GETPW_R_SIZE_MAX)
	groupBuffer = bufferKind(_C__SC_GETGR_R_SIZE_MAX)
)

func (k bufferKind) initialSize() _C_size_t {
	sz := _C_sysconf(_C_int(k))
	if sz == -1 {
		// DragonFly and FreeBSD do not have _SC_GETPW_R_SIZE_MAX.
		// Additionally, not all Linux systems have it, either. For
		// example, the musl libc returns -1.
		return 1024
	}
	if !isSizeReasonable(int64(sz)) {
		// Truncate.  If this truly isn't enough, retryWithBuffer will error on the first run.
		return maxBufferSize
	}
	return _C_size_t(sz)
}

// retryWithBuffer repeatedly calls f(), increasing the size of the
// buffer each time, until f succeeds, fails with a non-ERANGE error,
// or the buffer exceeds a reasonable limit.
func retryWithBuffer(kind bufferKind, f func([]byte) syscall.Errno) error {
	buf := make([]byte, kind.initialSize())
	for {
		errno := f(buf)
		if errno == 0 {
			return nil
		} else if runtime.GOOS == "aix" && errno+1 == 0 {
			// On AIX getpwuid_r appears to return -1,
			// not ERANGE, on buffer overflow.
		} else if errno != syscall.ERANGE {
			return errno
		}
		newSize := len(buf) * 2
		if !isSizeReasonable(int64(newSize)) {
			return fmt.Errorf("internal buffer exceeds %d bytes", maxBufferSize)
		}
		buf = make([]byte, newSize)
	}
}

const maxBufferSize = 1 << 20

func isSizeReasonable(sz int64) bool {
	return sz > 0 && sz <= maxBufferSize
}

// Because we can't use cgo in tests:
func structPasswdForNegativeTest() _C_struct_passwd {
	sp := _C_struct_passwd{}
	*_C_pw_uidp(&sp) = 1<<32 - 2
	*_C_pw_gidp(&sp) = 1<<32 - 3
	return sp
}

```

// === FILE: references!/go/src/os/user/getgrouplist_syscall.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !osusergo && darwin

package user

import (
	"internal/syscall/unix"
)

func getGroupList(name *_C_char, userGID _C_gid_t, gids *_C_gid_t, n *_C_int) _C_int {
	err := unix.Getgrouplist(name, userGID, gids, n)
	if err != nil {
		return -1
	}
	return 0
}

```

// === FILE: references!/go/src/os/user/getgrouplist_unix.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build cgo && !osusergo && unix && !android && !aix && !darwin

package user

/*
#include <unistd.h>
#include <sys/types.h>
#include <grp.h>

static int mygetgrouplist(const char* user, gid_t group, gid_t* groups, int* ngroups) {
	return getgrouplist(user, group, groups, ngroups);
}
*/
import "C"

func getGroupList(name *_C_char, userGID _C_gid_t, gids *_C_gid_t, n *_C_int) _C_int {
	return C.mygetgrouplist(name, userGID, gids, n)
}

```

// === FILE: references!/go/src/os/user/listgroups_stub.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build android

package user

import (
	"errors"
)

func init() {
	groupListImplemented = false
}

func listGroups(*User) ([]string, error) {
	return nil, errors.New("user: list groups not implemented")
}

```

// === FILE: references!/go/src/os/user/listgroups_unix.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (((unix && !android) || (js && wasm) || wasip1) && ((!cgo && !darwin) || osusergo)) || aix

package user

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

func listGroupsFromReader(u *User, r io.Reader) ([]string, error) {
	if u.Username == "" {
		return nil, errors.New("user: list groups: empty username")
	}
	primaryGid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, fmt.Errorf("user: list groups for %s: invalid gid %q", u.Username, u.Gid)
	}

	userCommas := []byte("," + u.Username + ",")  // ,john,
	userFirst := userCommas[1:]                   // john,
	userLast := userCommas[:len(userCommas)-1]    // ,john
	userOnly := userCommas[1 : len(userCommas)-1] // john

	// Add primary Gid first.
	groups := []string{u.Gid}

	rd := bufio.NewReader(r)
	done := false
	for !done {
		line, err := rd.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				done = true
			} else {
				return groups, err
			}
		}

		// Look for username in the list of users. If user is found,
		// append the GID to the groups slice.

		// There's no spec for /etc/passwd or /etc/group, but we try to follow
		// the same rules as the glibc parser, which allows comments and blank
		// space at the beginning of a line.
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' ||
			// If you search for a gid in a row where the group
			// name (the first field) starts with "+" or "-",
			// glibc fails to find the record, and so should we.
			line[0] == '+' || line[0] == '-' {
			continue
		}

		// Format of /etc/group is
		// 	groupname:password:GID:user_list
		// for example
		// 	wheel:x:10:john,paul,jack
		//	tcpdump:x:72:
		listIdx := bytes.LastIndexByte(line, ':')
		if listIdx == -1 || listIdx == len(line)-1 {
			// No commas, or empty group list.
			continue
		}
		if bytes.Count(line[:listIdx], colon) != 2 {
			// Incorrect number of colons.
			continue
		}
		list := line[listIdx+1:]
		// Check the list for user without splitting or copying.
		if !(bytes.Equal(list, userOnly) || bytes.HasPrefix(list, userFirst) || bytes.HasSuffix(list, userLast) || bytes.Contains(list, userCommas)) {
			continue
		}

		// groupname:password:GID
		parts := bytes.Split(line[:listIdx], colon)
		if len(parts) != 3 || len(parts[0]) == 0 {
			continue
		}
		gid := string(parts[2])
		// Make sure it's numeric and not the same as primary GID.
		numGid, err := strconv.Atoi(gid)
		if err != nil || numGid == primaryGid {
			continue
		}

		groups = append(groups, gid)
	}

	return groups, nil
}

func listGroups(u *User) ([]string, error) {
	f, err := os.Open(groupFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return listGroupsFromReader(u, f)
}

```

// === FILE: references!/go/src/os/user/lookup.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package user

import "sync"

const (
	userFile  = "/etc/passwd"
	groupFile = "/etc/group"
)

var colon = []byte{':'}

// Current returns the current user.
//
// The first call will cache the current user information.
// Subsequent calls will return the cached value and will not reflect
// changes to the current user.
func Current() (*User, error) {
	cache.Do(func() { cache.u, cache.err = current() })
	if cache.err != nil {
		return nil, cache.err
	}
	u := *cache.u // copy
	return &u, nil
}

// cache of the current user
var cache struct {
	sync.Once
	u   *User
	err error
}

// Lookup looks up a user by username. If the user cannot be found, the
// returned error is of type [UnknownUserError].
func Lookup(username string) (*User, error) {
	if u, err := Current(); err == nil && u.Username == username {
		return u, err
	}
	return lookupUser(username)
}

// LookupId looks up a user by userid. If the user cannot be found, the
// returned error is of type [UnknownUserIdError].
func LookupId(uid string) (*User, error) {
	if u, err := Current(); err == nil && u.Uid == uid {
		return u, err
	}
	return lookupUserId(uid)
}

// LookupGroup looks up a group by name. If the group cannot be found, the
// returned error is of type [UnknownGroupError].
func LookupGroup(name string) (*Group, error) {
	return lookupGroup(name)
}

// LookupGroupId looks up a group by groupid. If the group cannot be found, the
// returned error is of type [UnknownGroupIdError].
func LookupGroupId(gid string) (*Group, error) {
	return lookupGroupId(gid)
}

// GroupIds returns the list of group IDs that the user is a member of.
func (u *User) GroupIds() ([]string, error) {
	return listGroups(u)
}

```

// === FILE: references!/go/src/os/user/lookup_android.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build android

package user

import "errors"

func lookupUser(string) (*User, error) {
	return nil, errors.New("user: Lookup not implemented on android")
}

func lookupUserId(string) (*User, error) {
	return nil, errors.New("user: LookupId not implemented on android")
}

func lookupGroup(string) (*Group, error) {
	return nil, errors.New("user: LookupGroup not implemented on android")
}

func lookupGroupId(string) (*Group, error) {
	return nil, errors.New("user: LookupGroupId not implemented on android")
}

```

// === FILE: references!/go/src/os/user/lookup_plan9.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"
	"os"
	"syscall"
)

// Partial os/user support on Plan 9.
// Supports Current(), but not Lookup()/LookupId().
// The latter two would require parsing /adm/users.

func init() {
	userImplemented = false
	groupImplemented = false
	groupListImplemented = false
}

var (
	// unused variables (in this implementation)
	// modified during test to exercise code paths in the cgo implementation.
	userBuffer  = 0
	groupBuffer = 0
)

func current() (*User, error) {
	ubytes, err := os.ReadFile("/dev/user")
	if err != nil {
		return nil, fmt.Errorf("user: %s", err)
	}

	uname := string(ubytes)

	u := &User{
		Uid:      uname,
		Gid:      uname,
		Username: uname,
		Name:     uname,
		HomeDir:  os.Getenv("home"),
	}

	return u, nil
}

func lookupUser(username string) (*User, error) {
	return nil, syscall.EPLAN9
}

func lookupUserId(uid string) (*User, error) {
	return nil, syscall.EPLAN9
}

func lookupGroup(groupname string) (*Group, error) {
	return nil, syscall.EPLAN9
}

func lookupGroupId(string) (*Group, error) {
	return nil, syscall.EPLAN9
}

func listGroups(*User) ([]string, error) {
	return nil, syscall.EPLAN9
}

```

// === FILE: references!/go/src/os/user/lookup_stubs.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (!cgo && !darwin && !windows && !plan9) || android || (osusergo && !windows && !plan9)

package user

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
)

var (
	// unused variables (in this implementation)
	// modified during test to exercise code paths in the cgo implementation.
	userBuffer  = 0
	groupBuffer = 0
)

func current() (*User, error) {
	uid := currentUID()
	// $USER and /etc/passwd may disagree; prefer the latter if we can get it.
	// See issue 27524 for more information.
	u, err := lookupUserId(uid)
	if err == nil {
		return u, nil
	}

	homeDir, _ := os.UserHomeDir()
	u = &User{
		Uid:      uid,
		Gid:      currentGID(),
		Username: os.Getenv("USER"),
		Name:     "", // ignored
		HomeDir:  homeDir,
	}
	// On Android, return a dummy user instead of failing.
	switch runtime.GOOS {
	case "android":
		if u.Uid == "" {
			u.Uid = "1"
		}
		if u.Username == "" {
			u.Username = "android"
		}
	}
	// cgo isn't available, but if we found the minimum information
	// without it, use it:
	if u.Uid != "" && u.Username != "" && u.HomeDir != "" {
		return u, nil
	}
	var missing string
	if u.Username == "" {
		missing = "$USER"
	}
	if u.HomeDir == "" {
		if missing != "" {
			missing += ", "
		}
		missing += "$HOME"
	}
	return u, fmt.Errorf("user: Current requires cgo or %s set in environment", missing)
}

func currentUID() string {
	if id := os.Getuid(); id >= 0 {
		return strconv.Itoa(id)
	}
	// Note: Windows returns -1, but this file isn't used on
	// Windows anyway, so this empty return path shouldn't be
	// used.
	return ""
}

func currentGID() string {
	if id := os.Getgid(); id >= 0 {
		return strconv.Itoa(id)
	}
	return ""
}

```

// === FILE: references!/go/src/os/user/lookup_unix.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ((unix && !android) || (js && wasm) || wasip1) && ((!cgo && !darwin) || osusergo)

package user

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
)

// lineFunc returns a value, an error, or (nil, nil) to skip the row.
type lineFunc func(line []byte) (v any, err error)

// readColonFile parses r as an /etc/group or /etc/passwd style file, running
// fn for each row. readColonFile returns a value, an error, or (nil, nil) if
// the end of the file is reached without a match.
//
// readCols is the minimum number of colon-separated fields that will be passed
// to fn; in a long line additional fields may be silently discarded.
func readColonFile(r io.Reader, fn lineFunc, readCols int) (v any, err error) {
	rd := bufio.NewReader(r)

	// Read the file line-by-line.
	for {
		var isPrefix bool
		var wholeLine []byte

		// Read the next line. We do so in chunks (as much as reader's
		// buffer is able to keep), check if we read enough columns
		// already on each step and store final result in wholeLine.
		for {
			var line []byte
			line, isPrefix, err = rd.ReadLine()

			if err != nil {
				// We should return (nil, nil) if EOF is reached
				// without a match.
				if err == io.EOF {
					err = nil
				}
				return nil, err
			}

			// Simple common case: line is short enough to fit in a
			// single reader's buffer.
			if !isPrefix && len(wholeLine) == 0 {
				wholeLine = line
				break
			}

			wholeLine = append(wholeLine, line...)

			// Check if we read the whole line (or enough columns)
			// already.
			if !isPrefix || bytes.Count(wholeLine, []byte{':'}) >= readCols {
				break
			}
		}

		// There's no spec for /etc/passwd or /etc/group, but we try to follow
		// the same rules as the glibc parser, which allows comments and blank
		// space at the beginning of a line.
		wholeLine = bytes.TrimSpace(wholeLine)
		if len(wholeLine) == 0 || wholeLine[0] == '#' {
			continue
		}
		v, err = fn(wholeLine)
		if v != nil || err != nil {
			return
		}

		// If necessary, skip the rest of the line
		for ; isPrefix; _, isPrefix, err = rd.ReadLine() {
			if err != nil {
				// We should return (nil, nil) if EOF is reached without a match.
				if err == io.EOF {
					err = nil
				}
				return nil, err
			}
		}
	}
}

func matchGroupIndexValue(value string, idx int) lineFunc {
	var leadColon string
	if idx > 0 {
		leadColon = ":"
	}
	substr := []byte(leadColon + value + ":")
	return func(line []byte) (v any, err error) {
		if !bytes.Contains(line, substr) || bytes.Count(line, colon) < 3 {
			return
		}
		// wheel:*:0:root
		parts := strings.SplitN(string(line), ":", 4)
		if len(parts) < 4 || parts[0] == "" || parts[idx] != value ||
			// If the file contains +foo and you search for "foo", glibc
			// returns an "invalid argument" error. Similarly, if you search
			// for a gid for a row where the group name starts with "+" or "-",
			// glibc fails to find the record.
			parts[0][0] == '+' || parts[0][0] == '-' {
			return
		}
		if _, err := strconv.Atoi(parts[2]); err != nil {
			return nil, nil
		}
		return &Group{Name: parts[0], Gid: parts[2]}, nil
	}
}

func findGroupId(id string, r io.Reader) (*Group, error) {
	if v, err := readColonFile(r, matchGroupIndexValue(id, 2), 3); err != nil {
		return nil, err
	} else if v != nil {
		return v.(*Group), nil
	}
	return nil, UnknownGroupIdError(id)
}

func findGroupName(name string, r io.Reader) (*Group, error) {
	if v, err := readColonFile(r, matchGroupIndexValue(name, 0), 3); err != nil {
		return nil, err
	} else if v != nil {
		return v.(*Group), nil
	}
	return nil, UnknownGroupError(name)
}

// returns a *User for a row if that row's has the given value at the
// given index.
func matchUserIndexValue(value string, idx int) lineFunc {
	var leadColon string
	if idx > 0 {
		leadColon = ":"
	}
	substr := []byte(leadColon + value + ":")
	return func(line []byte) (v any, err error) {
		if !bytes.Contains(line, substr) || bytes.Count(line, colon) < 6 {
			return
		}
		// kevin:x:1005:1006::/home/kevin:/usr/bin/zsh
		parts := strings.SplitN(string(line), ":", 7)
		if len(parts) < 6 || parts[idx] != value || parts[0] == "" ||
			parts[0][0] == '+' || parts[0][0] == '-' {
			return
		}
		if _, err := strconv.Atoi(parts[2]); err != nil {
			return nil, nil
		}
		if _, err := strconv.Atoi(parts[3]); err != nil {
			return nil, nil
		}
		u := &User{
			Username: parts[0],
			Uid:      parts[2],
			Gid:      parts[3],
			Name:     parts[4],
			HomeDir:  parts[5],
		}
		// The pw_gecos field isn't quite standardized. Some docs
		// say: "It is expected to be a comma separated list of
		// personal data where the first item is the full name of the
		// user."
		u.Name, _, _ = strings.Cut(u.Name, ",")
		return u, nil
	}
}

func findUserId(uid string, r io.Reader) (*User, error) {
	i, e := strconv.Atoi(uid)
	if e != nil {
		return nil, errors.New("user: invalid userid " + uid)
	}
	if v, err := readColonFile(r, matchUserIndexValue(uid, 2), 6); err != nil {
		return nil, err
	} else if v != nil {
		return v.(*User), nil
	}
	return nil, UnknownUserIdError(i)
}

func findUsername(name string, r io.Reader) (*User, error) {
	if v, err := readColonFile(r, matchUserIndexValue(name, 0), 6); err != nil {
		return nil, err
	} else if v != nil {
		return v.(*User), nil
	}
	return nil, UnknownUserError(name)
}

func lookupGroup(groupname string) (*Group, error) {
	f, err := os.Open(groupFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return findGroupName(groupname, f)
}

func lookupGroupId(id string) (*Group, error) {
	f, err := os.Open(groupFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return findGroupId(id, f)
}

func lookupUser(username string) (*User, error) {
	f, err := os.Open(userFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return findUsername(username, f)
}

func lookupUserId(uid string) (*User, error) {
	f, err := os.Open(userFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return findUserId(uid, f)
}

```

// === FILE: references!/go/src/os/user/lookup_windows.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package user

import (
	"errors"
	"fmt"
	"internal/syscall/windows"
	"internal/syscall/windows/registry"
	"runtime"
	"syscall"
	"unsafe"
)

func isDomainJoined() (bool, error) {
	var domain *uint16
	var status uint32
	err := syscall.NetGetJoinInformation(nil, &domain, &status)
	if err != nil {
		return false, err
	}
	syscall.NetApiBufferFree((*byte)(unsafe.Pointer(domain)))
	return status == syscall.NetSetupDomainName, nil
}

func lookupFullNameDomain(domainAndUser string) (string, error) {
	return syscall.TranslateAccountName(domainAndUser,
		syscall.NameSamCompatible, syscall.NameDisplay, 50)
}

func lookupFullNameServer(servername, username string) (string, error) {
	s, e := syscall.UTF16PtrFromString(servername)
	if e != nil {
		return "", e
	}
	u, e := syscall.UTF16PtrFromString(username)
	if e != nil {
		return "", e
	}
	var p *byte
	e = syscall.NetUserGetInfo(s, u, 10, &p)
	if e != nil {
		return "", e
	}
	defer syscall.NetApiBufferFree(p)
	i := (*syscall.UserInfo10)(unsafe.Pointer(p))
	return windows.UTF16PtrToString(i.FullName), nil
}

func lookupFullName(domain, username, domainAndUser string) (string, error) {
	joined, err := isDomainJoined()
	if err == nil && joined {
		name, err := lookupFullNameDomain(domainAndUser)
		if err == nil {
			return name, nil
		}
	}
	name, err := lookupFullNameServer(domain, username)
	if err == nil {
		return name, nil
	}
	// domain worked neither as a domain nor as a server
	// could be domain server unavailable
	// pretend username is fullname
	return username, nil
}

// getProfilesDirectory retrieves the path to the root directory
// where user profiles are stored.
func getProfilesDirectory() (string, error) {
	n := uint32(100)
	for {
		b := make([]uint16, n)
		e := windows.GetProfilesDirectory(&b[0], &n)
		if e == nil {
			return syscall.UTF16ToString(b), nil
		}
		if e != syscall.ERROR_INSUFFICIENT_BUFFER {
			return "", e
		}
		if n <= uint32(len(b)) {
			return "", e
		}
	}
}

func isServiceAccount(sid *syscall.SID) bool {
	if !windows.IsValidSid(sid) {
		// We don't accept SIDs from the public API, so this should never happen.
		// Better be on the safe side and validate anyway.
		return false
	}
	// The following RIDs are considered service user accounts as per
	// https://learn.microsoft.com/en-us/windows/win32/secauthz/well-known-sids and
	// https://learn.microsoft.com/en-us/windows/win32/services/service-user-accounts:
	// - "S-1-5-18": LocalSystem
	// - "S-1-5-19": LocalService
	// - "S-1-5-20": NetworkService
	if windows.GetSidSubAuthorityCount(sid) != windows.SID_REVISION ||
		windows.GetSidIdentifierAuthority(sid) != windows.SECURITY_NT_AUTHORITY {
		return false
	}
	switch windows.GetSidSubAuthority(sid, 0) {
	case windows.SECURITY_LOCAL_SYSTEM_RID,
		windows.SECURITY_LOCAL_SERVICE_RID,
		windows.SECURITY_NETWORK_SERVICE_RID:
		return true
	}
	return false
}

func isValidUserAccountType(sid *syscall.SID, sidType uint32) bool {
	switch sidType {
	case syscall.SidTypeUser:
		return true
	case syscall.SidTypeWellKnownGroup:
		return isServiceAccount(sid)
	}
	return false
}

func isValidGroupAccountType(sidType uint32) bool {
	switch sidType {
	case syscall.SidTypeGroup:
		return true
	case syscall.SidTypeWellKnownGroup:
		// Some well-known groups are also considered service accounts,
		// so isValidUserAccountType would return true for them.
		// We have historically allowed them in LookupGroup and LookupGroupId,
		// so don't treat them as invalid here.
		return true
	case syscall.SidTypeAlias:
		// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-samr/7b2aeb27-92fc-41f6-8437-deb65d950921#gt_0387e636-5654-4910-9519-1f8326cf5ec0
		// SidTypeAlias should also be treated as a group type next to SidTypeGroup
		// and SidTypeWellKnownGroup:
		// "alias object -> resource group: A group object..."
		//
		// Tests show that "Administrators" can be considered of type SidTypeAlias.
		return true
	}
	return false
}

// lookupUsernameAndDomain obtains the username and domain for usid.
func lookupUsernameAndDomain(usid *syscall.SID) (username, domain string, sidType uint32, e error) {
	username, domain, sidType, e = usid.LookupAccount("")
	if e != nil {
		return "", "", 0, e
	}
	if !isValidUserAccountType(usid, sidType) {
		return "", "", 0, fmt.Errorf("user: should be user account type, not %d", sidType)
	}
	return username, domain, sidType, nil
}

// findHomeDirInRegistry finds the user home path based on the uid.
func findHomeDirInRegistry(uid string) (dir string, e error) {
	k, e := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList\`+uid, registry.QUERY_VALUE)
	if e != nil {
		return "", e
	}
	defer k.Close()
	dir, _, e = k.GetStringValue("ProfileImagePath")
	if e != nil {
		return "", e
	}
	return dir, nil
}

// lookupGroupName accepts the name of a group and retrieves the group SID.
func lookupGroupName(groupname string) (string, error) {
	sid, _, t, e := syscall.LookupSID("", groupname)
	if e != nil {
		return "", e
	}
	if !isValidGroupAccountType(t) {
		return "", fmt.Errorf("lookupGroupName: should be group account type, not %d", t)
	}
	return sid.String()
}

// listGroupsForUsernameAndDomain accepts username and domain and retrieves
// a SID list of the local groups where this user is a member.
func listGroupsForUsernameAndDomain(username, domain string) ([]string, error) {
	// Check if both the domain name and user should be used.
	var query string
	joined, err := isDomainJoined()
	if err == nil && joined && len(domain) != 0 {
		query = domain + `\` + username
	} else {
		query = username
	}
	q, err := syscall.UTF16PtrFromString(query)
	if err != nil {
		return nil, err
	}
	var p0 *byte
	var entriesRead, totalEntries uint32
	// https://learn.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netusergetlocalgroups
	// NetUserGetLocalGroups() would return a list of LocalGroupUserInfo0
	// elements which hold the names of local groups where the user participates.
	// The list does not follow any sorting order.
	err = windows.NetUserGetLocalGroups(nil, q, 0, windows.LG_INCLUDE_INDIRECT, &p0, windows.MAX_PREFERRED_LENGTH, &entriesRead, &totalEntries)
	if err != nil {
		return nil, err
	}
	defer syscall.NetApiBufferFree(p0)
	if entriesRead == 0 {
		return nil, nil
	}
	entries := (*[1024]windows.LocalGroupUserInfo0)(unsafe.Pointer(p0))[:entriesRead:entriesRead]
	var sids []string
	for _, entry := range entries {
		if entry.Name == nil {
			continue
		}
		sid, err := lookupGroupName(windows.UTF16PtrToString(entry.Name))
		if err != nil {
			return nil, err
		}
		sids = append(sids, sid)
	}
	return sids, nil
}

func newUser(uid, gid, dir, username, domain string) (*User, error) {
	domainAndUser := domain + `\` + username
	name, e := lookupFullName(domain, username, domainAndUser)
	if e != nil {
		return nil, e
	}
	u := &User{
		Uid:      uid,
		Gid:      gid,
		Username: domainAndUser,
		Name:     name,
		HomeDir:  dir,
	}
	return u, nil
}

var (
	// unused variables (in this implementation)
	// modified during test to exercise code paths in the cgo implementation.
	userBuffer  = 0
	groupBuffer = 0
)

func current() (*User, error) {
	// Use runAsProcessOwner to ensure that we can access the process token
	// when calling syscall.OpenCurrentProcessToken if the current thread
	// is impersonating a different user. See https://go.dev/issue/68647.
	var usr *User
	err := runAsProcessOwner(func() error {
		t, e := syscall.OpenCurrentProcessToken()
		if e != nil {
			return e
		}
		defer t.Close()
		u, e := t.GetTokenUser()
		if e != nil {
			return e
		}
		pg, e := t.GetTokenPrimaryGroup()
		if e != nil {
			return e
		}
		uid, e := u.User.Sid.String()
		if e != nil {
			return e
		}
		gid, e := pg.PrimaryGroup.String()
		if e != nil {
			return e
		}
		dir, e := t.GetUserProfileDirectory()
		if e != nil {
			return e
		}
		username, e := windows.GetUserName(syscall.NameSamCompatible)
		if e != nil {
			return e
		}
		displayName, e := windows.GetUserName(syscall.NameDisplay)
		if e != nil {
			// Historically, the username is used as fallback
			// when the display name can't be retrieved.
			displayName = username
		}
		usr = &User{
			Uid:      uid,
			Gid:      gid,
			Username: username,
			Name:     displayName,
			HomeDir:  dir,
		}
		return nil
	})
	return usr, err
}

// runAsProcessOwner runs f in the context of the current process owner,
// that is, removing any impersonation that may be in effect before calling f,
// and restoring the impersonation afterwards.
func runAsProcessOwner(f func() error) error {
	var impersonationRollbackErr error
	runtime.LockOSThread()
	defer func() {
		// If impersonation failed, the thread is running with the wrong token,
		// so it's better to terminate it.
		// This is achieved by not calling runtime.UnlockOSThread.
		if impersonationRollbackErr != nil {
			println("os/user: failed to revert to previous token:", impersonationRollbackErr.Error())
			runtime.Goexit()
		} else {
			runtime.UnlockOSThread()
		}
	}()
	prevToken, isProcessToken, err := getCurrentToken()
	if err != nil {
		return fmt.Errorf("os/user: failed to get current token: %w", err)
	}
	defer prevToken.Close()
	if !isProcessToken {
		if err = windows.RevertToSelf(); err != nil {
			return fmt.Errorf("os/user: failed to revert to self: %w", err)
		}
		defer func() {
			impersonationRollbackErr = windows.ImpersonateLoggedOnUser(prevToken)
		}()
	}
	return f()
}

// getCurrentToken returns the current thread token, or
// the process token if the thread doesn't have a token.
func getCurrentToken() (t syscall.Token, isProcessToken bool, err error) {
	thread, _ := windows.GetCurrentThread()
	// Need TOKEN_DUPLICATE and TOKEN_IMPERSONATE to use the token in ImpersonateLoggedOnUser.
	err = windows.OpenThreadToken(thread, syscall.TOKEN_QUERY|syscall.TOKEN_DUPLICATE|syscall.TOKEN_IMPERSONATE, true, &t)
	if errors.Is(err, windows.ERROR_NO_TOKEN) {
		// Not impersonating, use the process token.
		isProcessToken = true
		t, err = syscall.OpenCurrentProcessToken()
	}
	return t, isProcessToken, err
}

// lookupUserPrimaryGroup obtains the primary group SID for a user using this method:
// https://support.microsoft.com/en-us/help/297951/how-to-use-the-primarygroupid-attribute-to-find-the-primary-group-for
// The method follows this formula: domainRID + "-" + primaryGroupRID
func lookupUserPrimaryGroup(username, domain string) (string, error) {
	// get the domain RID
	sid, _, t, e := syscall.LookupSID("", domain)
	if e != nil {
		return "", e
	}
	if t != syscall.SidTypeDomain {
		return "", fmt.Errorf("lookupUserPrimaryGroup: should be domain account type, not %d", t)
	}
	domainRID, e := sid.String()
	if e != nil {
		return "", e
	}
	// If the user has joined a domain use the RID of the default primary group
	// called "Domain Users":
	// https://support.microsoft.com/en-us/help/243330/well-known-security-identifiers-in-windows-operating-systems
	// SID: S-1-5-21domain-513
	//
	// The correct way to obtain the primary group of a domain user is
	// probing the user primaryGroupID attribute in the server Active Directory:
	// https://learn.microsoft.com/en-us/windows/win32/adschema/a-primarygroupid
	//
	// Note that the primary group of domain users should not be modified
	// on Windows for performance reasons, even if it's possible to do that.
	// The .NET Developer's Guide to Directory Services Programming - Page 409
	// https://books.google.bg/books?id=kGApqjobEfsC&lpg=PA410&ots=p7oo-eOQL7&dq=primary%20group%20RID&hl=bg&pg=PA409#v=onepage&q&f=false
	joined, err := isDomainJoined()
	if err == nil && joined {
		return domainRID + "-513", nil
	}
	// For non-domain users call NetUserGetInfo() with level 4, which
	// in this case would not have any network overhead.
	// The primary group should not change from RID 513 here either
	// but the group will be called "None" instead:
	// https://www.adampalmer.me/iodigitalsec/2013/08/10/windows-null-session-enumeration/
	// "Group 'None' (RID: 513)"
	u, e := syscall.UTF16PtrFromString(username)
	if e != nil {
		return "", e
	}
	d, e := syscall.UTF16PtrFromString(domain)
	if e != nil {
		return "", e
	}
	var p *byte
	e = syscall.NetUserGetInfo(d, u, 4, &p)
	if e != nil {
		return "", e
	}
	defer syscall.NetApiBufferFree(p)
	i := (*windows.UserInfo4)(unsafe.Pointer(p))
	return fmt.Sprintf("%s-%d", domainRID, i.PrimaryGroupID), nil
}

func newUserFromSid(usid *syscall.SID) (*User, error) {
	username, domain, sidType, e := lookupUsernameAndDomain(usid)
	if e != nil {
		return nil, e
	}
	uid, e := usid.String()
	if e != nil {
		return nil, e
	}
	var gid string
	if sidType == syscall.SidTypeWellKnownGroup {
		// The SID does not contain a domain; this function's domain variable has
		// been populated with the SID's identifier authority. This happens with
		// special service user accounts such as "NT AUTHORITY\LocalSystem".
		// In this case, gid is the same as the user SID.
		gid = uid
	} else {
		gid, e = lookupUserPrimaryGroup(username, domain)
		if e != nil {
			return nil, e
		}
	}
	// If this user has logged in at least once their home path should be stored
	// in the registry under the specified SID. References:
	// https://social.technet.microsoft.com/wiki/contents/articles/13895.how-to-remove-a-corrupted-user-profile-from-the-registry.aspx
	// https://support.asperasoft.com/hc/en-us/articles/216127438-How-to-delete-Windows-user-profiles
	//
	// The registry is the most reliable way to find the home path as the user
	// might have decided to move it outside of the default location,
	// (e.g. C:\users). Reference:
	// https://answers.microsoft.com/en-us/windows/forum/windows_7-security/how-do-i-set-a-home-directory-outside-cusers-for-a/aed68262-1bf4-4a4d-93dc-7495193a440f
	dir, e := findHomeDirInRegistry(uid)
	if e != nil {
		// If the home path does not exist in the registry, the user might
		// have not logged in yet; fall back to using getProfilesDirectory().
		// Find the username based on a SID and append that to the result of
		// getProfilesDirectory(). The domain is not relevant here.
		dir, e = getProfilesDirectory()
		if e != nil {
			return nil, e
		}
		dir += `\` + username
	}
	return newUser(uid, gid, dir, username, domain)
}

func lookupUser(username string) (*User, error) {
	sid, _, t, e := syscall.LookupSID("", username)
	if e != nil {
		return nil, e
	}
	if !isValidUserAccountType(sid, t) {
		return nil, fmt.Errorf("user: should be user account type, not %d", t)
	}
	return newUserFromSid(sid)
}

func lookupUserId(uid string) (*User, error) {
	sid, e := syscall.StringToSid(uid)
	if e != nil {
		return nil, e
	}
	return newUserFromSid(sid)
}

func lookupGroup(groupname string) (*Group, error) {
	sid, err := lookupGroupName(groupname)
	if err != nil {
		return nil, err
	}
	return &Group{Name: groupname, Gid: sid}, nil
}

func lookupGroupId(gid string) (*Group, error) {
	sid, err := syscall.StringToSid(gid)
	if err != nil {
		return nil, err
	}
	groupname, _, t, err := sid.LookupAccount("")
	if err != nil {
		return nil, err
	}
	if !isValidGroupAccountType(t) {
		return nil, fmt.Errorf("lookupGroupId: should be group account type, not %d", t)
	}
	return &Group{Name: groupname, Gid: gid}, nil
}

func listGroups(user *User) ([]string, error) {
	var sids []string
	if u, err := Current(); err == nil && u.Uid == user.Uid {
		// It is faster and more reliable to get the groups
		// of the current user from the current process token.
		err := runAsProcessOwner(func() error {
			t, err := syscall.OpenCurrentProcessToken()
			if err != nil {
				return err
			}
			defer t.Close()
			groups, err := windows.GetTokenGroups(t)
			if err != nil {
				return err
			}
			for _, g := range groups.AllGroups() {
				sid, err := g.Sid.String()
				if err != nil {
					return err
				}
				sids = append(sids, sid)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		sid, err := syscall.StringToSid(user.Uid)
		if err != nil {
			return nil, err
		}
		username, domain, _, err := lookupUsernameAndDomain(sid)
		if err != nil {
			return nil, err
		}
		sids, err = listGroupsForUsernameAndDomain(username, domain)
		if err != nil {
			return nil, err
		}
	}
	// Add the primary group of the user to the list if it is not already there.
	// This is done only to comply with the POSIX concept of a primary group.
	for _, sid := range sids {
		if sid == user.Gid {
			return sids, nil
		}
	}
	return append(sids, user.Gid), nil
}

```

// === FILE: references!/go/src/os/user/user.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package user allows user account lookups by name or id.

For most Unix systems, this package has two internal implementations of
resolving user and group ids to names, and listing supplementary group IDs.
One is written in pure Go and parses /etc/passwd and /etc/group. The other
is cgo-based and relies on the standard C library (libc) routines such as
getpwuid_r, getgrnam_r, and getgrouplist.

When cgo is available, and the required routines are implemented in libc
for a particular platform, cgo-based (libc-backed) code is used.
This can be overridden by using osusergo build tag, which enforces
the pure Go implementation.
*/
package user

import (
	"strconv"
)

// These may be set to false in init() for a particular platform and/or
// build flags to let the tests know to skip tests of some features.
var (
	userImplemented      = true
	groupImplemented     = true
	groupListImplemented = true
)

// User represents a user account.
type User struct {
	// Uid is the user ID.
	// On POSIX systems, this is a decimal number representing the uid.
	// On Windows, this is a security identifier (SID) in a string format.
	// On Plan 9, this is the contents of /dev/user.
	Uid string
	// Gid is the primary group ID.
	// On POSIX systems, this is a decimal number representing the gid.
	// On Windows, this is a SID in a string format.
	// On Plan 9, this is the contents of /dev/user.
	Gid string
	// Username is the login name.
	Username string
	// Name is the user's real or display name.
	// It might be blank.
	// On POSIX systems, this is the first (or only) entry in the GECOS field
	// list.
	// On Windows, this is the user's display name.
	// On Plan 9, this is the contents of /dev/user.
	Name string
	// HomeDir is the path to the user's home directory (if they have one).
	HomeDir string
}

// Group represents a grouping of users.
//
// On POSIX systems Gid contains a decimal number representing the group ID.
type Group struct {
	Gid  string // group ID
	Name string // group name
}

// UnknownUserIdError is returned by [LookupId] when a user cannot be found.
type UnknownUserIdError int

func (e UnknownUserIdError) Error() string {
	return "user: unknown userid " + strconv.Itoa(int(e))
}

// UnknownUserError is returned by [Lookup] when
// a user cannot be found.
type UnknownUserError string

func (e UnknownUserError) Error() string {
	return "user: unknown user " + string(e)
}

// UnknownGroupIdError is returned by [LookupGroupId] when
// a group cannot be found.
type UnknownGroupIdError string

func (e UnknownGroupIdError) Error() string {
	return "group: unknown groupid " + string(e)
}

// UnknownGroupError is returned by [LookupGroup] when
// a group cannot be found.
type UnknownGroupError string

func (e UnknownGroupError) Error() string {
	return "group: unknown group " + string(e)
}

```

// === FILE: references!/go/src/os/wait6_dragonfly.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

const _P_PID = 0

func wait6(idtype, id, options int) (status int, errno syscall.Errno) {
	var status32 int32 // C.int
	_, _, errno = syscall.Syscall6(syscall.SYS_WAIT6, uintptr(idtype), uintptr(id), uintptr(unsafe.Pointer(&status32)), uintptr(options), 0, 0)
	return int(status32), errno
}

```

// === FILE: references!/go/src/os/wait6_freebsd64.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build freebsd && (amd64 || arm64 || riscv64)

package os

import (
	"syscall"
	"unsafe"
)

const _P_PID = 0

func wait6(idtype, id, options int) (status int, errno syscall.Errno) {
	var status32 int32 // C.int
	_, _, errno = syscall.Syscall6(syscall.SYS_WAIT6, uintptr(idtype), uintptr(id), uintptr(unsafe.Pointer(&status32)), uintptr(options), 0, 0)
	return int(status32), errno
}

```

// === FILE: references!/go/src/os/wait6_freebsd_386.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

const _P_PID = 0

func wait6(idtype, id, options int) (status int, errno syscall.Errno) {
	// freebsd32_wait6_args{ idtype, id1, id2, status, options, wrusage, info }
	_, _, errno = syscall.Syscall9(syscall.SYS_WAIT6, uintptr(idtype), uintptr(id), 0, uintptr(unsafe.Pointer(&status)), uintptr(options), 0, 0, 0, 0)
	return status, errno
}

```

// === FILE: references!/go/src/os/wait6_freebsd_arm.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

const _P_PID = 0

func wait6(idtype, id, options int) (status int, errno syscall.Errno) {
	// freebsd32_wait6_args{ idtype, pad, id1, id2, status, options, wrusage, info }
	_, _, errno = syscall.Syscall9(syscall.SYS_WAIT6, uintptr(idtype), 0, uintptr(id), 0, uintptr(unsafe.Pointer(&status)), uintptr(options), 0, 0, 0)
	return status, errno
}

```

// === FILE: references!/go/src/os/wait6_netbsd.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"syscall"
	"unsafe"
)

const _P_PID = 1 // not 0 as on FreeBSD and Dragonfly!

func wait6(idtype, id, options int) (status int, errno syscall.Errno) {
	var status32 int32 // C.int
	_, _, errno = syscall.Syscall6(syscall.SYS_WAIT6, uintptr(idtype), uintptr(id), uintptr(unsafe.Pointer(&status32)), uintptr(options), 0, 0)
	return int(status32), errno
}

```

// === FILE: references!/go/src/os/wait_unimp.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// aix, darwin, js/wasm, openbsd, solaris and wasip1/wasm don't implement
// waitid/wait6.

//go:build aix || darwin || (js && wasm) || openbsd || solaris || wasip1

package os

// blockUntilWaitable attempts to block until a call to p.Wait will
// succeed immediately, and reports whether it has done so.
// It does not actually call p.Wait.
// This version is used on systems that do not implement waitid,
// or where we have not implemented it yet. Note that this is racy:
// a call to Process.Signal can in an extremely unlikely case send a
// signal to the wrong process, see issue #13987.
func (p *Process) blockUntilWaitable() (bool, error) {
	return false, nil
}

```

// === FILE: references!/go/src/os/wait_wait6.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build dragonfly || freebsd || netbsd

package os

import (
	"runtime"
	"syscall"
)

// blockUntilWaitable attempts to block until a call to p.Wait will
// succeed immediately, and reports whether it has done so.
// It does not actually call p.Wait.
func (p *Process) blockUntilWaitable() (bool, error) {
	err := ignoringEINTR(func() error {
		_, errno := wait6(_P_PID, p.Pid, syscall.WEXITED|syscall.WNOWAIT)
		if errno != 0 {
			return errno
		}
		return nil
	})
	runtime.KeepAlive(p)
	if err == syscall.ENOSYS {
		return false, nil
	} else if err != nil {
		return false, NewSyscallError("wait6", err)
	}
	return true, nil
}

```

// === FILE: references!/go/src/os/wait_waitid.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// We used to use this code for Darwin, but according to issue #19314
// waitid returns if the process is stopped, even when using WEXITED.

//go:build linux

package os

import (
	"internal/syscall/unix"
	"runtime"
	"syscall"
)

// blockUntilWaitable attempts to block until a call to p.Wait will
// succeed immediately, and reports whether it has done so.
// It does not actually call p.Wait.
func (p *Process) blockUntilWaitable() (bool, error) {
	var info unix.SiginfoChild
	err := ignoringEINTR(func() error {
		return unix.Waitid(unix.P_PID, p.Pid, &info, syscall.WEXITED|syscall.WNOWAIT, nil)
	})
	runtime.KeepAlive(p)
	if err != nil {
		// waitid has been available since Linux 2.6.9, but
		// reportedly is not available in Ubuntu on Windows.
		// See issue 16610.
		if err == syscall.ENOSYS {
			return false, nil
		}
		return false, NewSyscallError("waitid", err)
	}
	return true, nil
}

```

// === FILE: references!/go/src/os/zero_copy_freebsd.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/poll"
	"io"
)

var pollCopyFileRange = poll.CopyFileRange

func (f *File) writeTo(w io.Writer) (written int64, handled bool, err error) {
	return 0, false, nil
}

func (f *File) readFrom(r io.Reader) (written int64, handled bool, err error) {
	// copy_file_range(2) doesn't support destinations opened with
	// O_APPEND, so don't bother to try zero-copy with these system calls.
	//
	// Visit https://man.freebsd.org/cgi/man.cgi?copy_file_range(2)#ERRORS for details.
	if f.appendMode {
		return 0, false, nil
	}

	var (
		remain int64
		lr     *io.LimitedReader
	)
	if lr, r, remain = tryLimitedReader(r); remain <= 0 {
		return 0, true, nil
	}

	var src *File
	switch v := r.(type) {
	case *File:
		src = v
	case fileWithoutWriteTo:
		src = v.File
	default:
		return 0, false, nil
	}

	if src.checkValid("ReadFrom") != nil {
		// Avoid returning the error as we report handled as false,
		// leave further error handling as the responsibility of the caller.
		return 0, false, nil
	}

	written, handled, err = pollCopyFileRange(&f.pfd, &src.pfd, remain)
	if lr != nil {
		lr.N -= written
	}

	return written, handled, wrapSyscallError("copy_file_range", err)
}

```

// === FILE: references!/go/src/os/zero_copy_linux.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/poll"
	"io"
	"syscall"
)

var (
	pollCopyFileRange = poll.CopyFileRange
	pollSplice        = poll.Splice
)

func (f *File) writeTo(w io.Writer) (written int64, handled bool, err error) {
	pfd, network := getPollFDAndNetwork(w)
	// TODO(panjf2000): same as File.spliceToFile.
	if pfd == nil || !pfd.IsStream || !isUnixOrTCP(string(network)) {
		return
	}

	sc, err := f.SyscallConn()
	if err != nil {
		return
	}

	rerr := sc.Read(func(fd uintptr) (done bool) {
		written, err, handled = poll.SendFile(pfd, fd, 0)
		return true
	})

	if err == nil {
		err = rerr
	}

	return written, handled, wrapSyscallError("sendfile", err)
}

func (f *File) readFrom(r io.Reader) (written int64, handled bool, err error) {
	// Neither copy_file_range(2) nor splice(2) supports destinations opened with
	// O_APPEND, so don't bother to try zero-copy with these system calls.
	//
	// Visit https://man7.org/linux/man-pages/man2/copy_file_range.2.html#ERRORS and
	// https://man7.org/linux/man-pages/man2/splice.2.html#ERRORS for details.
	if f.appendMode {
		return 0, false, nil
	}

	written, handled, err = f.copyFileRange(r)
	if handled {
		return
	}
	return f.spliceToFile(r)
}

func (f *File) spliceToFile(r io.Reader) (written int64, handled bool, err error) {
	var (
		remain int64
		lr     *io.LimitedReader
	)
	if lr, r, remain = tryLimitedReader(r); remain <= 0 {
		return 0, true, nil
	}

	pfd, _ := getPollFDAndNetwork(r)
	// TODO(panjf2000): run some tests to see if we should unlock the non-streams for splice.
	// Streams benefit the most from the splice(2), non-streams are not even supported in old kernels
	// where splice(2) will just return EINVAL; newer kernels support non-streams like UDP, but I really
	// doubt that splice(2) could help non-streams, cuz they usually send small frames respectively
	// and one splice call would result in one frame.
	// splice(2) is suitable for large data but the generation of fragments defeats its edge here.
	// Therefore, don't bother to try splice if the r is not a streaming descriptor.
	if pfd == nil || !pfd.IsStream {
		return
	}

	written, handled, err = pollSplice(&f.pfd, pfd, remain)

	if lr != nil {
		lr.N = remain - written
	}

	return written, handled, wrapSyscallError("splice", err)
}

func (f *File) copyFileRange(r io.Reader) (written int64, handled bool, err error) {
	var (
		remain int64
		lr     *io.LimitedReader
	)
	if lr, r, remain = tryLimitedReader(r); remain <= 0 {
		return 0, true, nil
	}

	var src *File
	switch v := r.(type) {
	case *File:
		src = v
	case fileWithoutWriteTo:
		src = v.File
	default:
		return 0, false, nil
	}

	if src.checkValid("ReadFrom") != nil {
		// Avoid returning the error as we report handled as false,
		// leave further error handling as the responsibility of the caller.
		return 0, false, nil
	}

	written, handled, err = pollCopyFileRange(&f.pfd, &src.pfd, remain)
	if lr != nil {
		lr.N -= written
	}
	return written, handled, wrapSyscallError("copy_file_range", err)
}

// getPollFDAndNetwork tries to get the poll.FD and network type from the given interface
// by expecting the underlying type of i to be the implementation of syscall.Conn
// that contains a *net.rawConn.
func getPollFDAndNetwork(i any) (*poll.FD, poll.String) {
	sc, ok := i.(syscall.Conn)
	if !ok {
		return nil, ""
	}
	rc, err := sc.SyscallConn()
	if err != nil {
		return nil, ""
	}
	irc, ok := rc.(interface {
		PollFD() *poll.FD
		Network() poll.String
	})
	if !ok {
		return nil, ""
	}
	return irc.PollFD(), irc.Network()
}

func isUnixOrTCP(network string) bool {
	switch network {
	case "tcp", "tcp4", "tcp6", "unix":
		return true
	default:
		return false
	}
}

```

// === FILE: references!/go/src/os/zero_copy_posix.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || js || wasip1 || windows

package os

import (
	"io"
	"syscall"
)

// wrapSyscallError takes an error and a syscall name. If the error is
// a syscall.Errno, it wraps it in an os.SyscallError using the syscall name.
func wrapSyscallError(name string, err error) error {
	if _, ok := err.(syscall.Errno); ok {
		err = NewSyscallError(name, err)
	}
	return err
}

// tryLimitedReader tries to assert the io.Reader to io.LimitedReader, it returns the io.LimitedReader,
// the underlying io.Reader and the remaining amount of bytes if the assertion succeeds,
// otherwise it just returns the original io.Reader and the theoretical unlimited remaining amount of bytes.
func tryLimitedReader(r io.Reader) (*io.LimitedReader, io.Reader, int64) {
	var remain int64 = 1<<63 - 1 // by default, copy until EOF

	lr, ok := r.(*io.LimitedReader)
	if !ok {
		return nil, r, remain
	}

	remain = lr.N
	return lr, lr.R, remain
}

```

// === FILE: references!/go/src/os/zero_copy_solaris.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"internal/poll"
	"io"
	"runtime"
	"syscall"
)

func (f *File) writeTo(w io.Writer) (written int64, handled bool, err error) {
	return 0, false, nil
}

// readFrom is basically a refactor of net.sendFile, but adapted to work for the target of *File.
func (f *File) readFrom(r io.Reader) (written int64, handled bool, err error) {
	var remain int64 = 0 // 0 indicates sending until EOF
	lr, ok := r.(*io.LimitedReader)
	if ok {
		remain, r = lr.N, lr.R
		if remain <= 0 {
			return 0, true, nil
		}
	}

	var src *File
	switch v := r.(type) {
	case *File:
		src = v
	case fileWithoutWriteTo:
		src = v.File
	default:
		return 0, false, nil
	}

	if src.checkValid("ReadFrom") != nil {
		// Avoid returning the error as we report handled as false,
		// leave further error handling as the responsibility of the caller.
		return 0, false, nil
	}

	// If fd_in and fd_out refer to the same file and the source and target ranges overlap,
	// sendfile(2) on SunOS will allow this kind of overlapping and work like a memmove,
	// in this case the file content remains the same after copying, which is not what we want.
	// Thus, we just bail out here and leave it to generic copy when it's a file copying itself.
	if f.pfd.Sysfd == src.pfd.Sysfd {
		return 0, false, nil
	}

	// sendfile() on illumos seems to incur intermittent failures when the
	// target file is a standard stream (stdout/stderr), we hereby skip any
	// anything other than regular files conservatively and leave them to generic copy.
	// Check out https://go.dev/issue/68863 for more details.
	if runtime.GOOS == "illumos" {
		fi, err := f.Stat()
		if err != nil {
			return 0, false, nil
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return 0, false, nil
		}
		if typ := st.Mode & syscall.S_IFMT; typ != syscall.S_IFREG {
			return 0, false, nil
		}
	}

	sc, err := src.SyscallConn()
	if err != nil {
		return
	}

	// System call sendfile()s on Solaris and illumos support file-to-file copying.
	// Check out https://docs.oracle.com/cd/E86824_01/html/E54768/sendfile-3ext.html and
	// https://docs.oracle.com/cd/E88353_01/html/E37843/sendfile-3c.html and
	// https://illumos.org/man/3EXT/sendfile for more details.
	rerr := sc.Read(func(fd uintptr) bool {
		written, err, handled = poll.SendFile(&f.pfd, fd, remain)
		return true
	})
	if lr != nil {
		lr.N = remain - written
	}
	if err == nil {
		err = rerr
	}

	return written, handled, wrapSyscallError("sendfile", err)
}

```

// === FILE: references!/go/src/os/zero_copy_stub.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !freebsd && !linux && !solaris

package os

import "io"

func (f *File) writeTo(w io.Writer) (written int64, handled bool, err error) {
	return 0, false, nil
}

func (f *File) readFrom(r io.Reader) (n int64, handled bool, err error) {
	return 0, false, nil
}

```

