# Domain Architecture: runtime/debug

## Layout Topology
```text
runtime/debug/
├── debug.s
├── garbage.go
├── mod.go
├── stack.go
└── stubs.go
```

## Source Stream Aggregation

// === FILE: references/go/src/runtime/debug/debug.s ===
```text
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Nothing to see here.
// This file exists so that the go command knows that parts of the
// package are implemented in C, so that it does not instruct the
// Go compiler to complain about extern declarations.
// The actual implementations are in package runtime.

```

// === FILE: references/go/src/runtime/debug/garbage.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debug

import (
	"runtime"
	"slices"
	"time"
)

// GCStats collect information about recent garbage collections.
type GCStats struct {
	LastGC         time.Time       // time of last collection
	NumGC          int64           // number of garbage collections
	PauseTotal     time.Duration   // total pause for all collections
	Pause          []time.Duration // pause history, most recent first
	PauseEnd       []time.Time     // pause end times history, most recent first
	PauseQuantiles []time.Duration
}

// ReadGCStats reads statistics about garbage collection into stats.
// The number of entries in the pause history is system-dependent;
// stats.Pause slice will be reused if large enough, reallocated otherwise.
// ReadGCStats may use the full capacity of the stats.Pause slice.
// If stats.PauseQuantiles is non-empty, ReadGCStats fills it with quantiles
// summarizing the distribution of pause time. For example, if
// len(stats.PauseQuantiles) is 5, it will be filled with the minimum,
// 25%, 50%, 75%, and maximum pause times.
func ReadGCStats(stats *GCStats) {
	// Create a buffer with space for at least two copies of the
	// pause history tracked by the runtime. One will be returned
	// to the caller and the other will be used as transfer buffer
	// for end times history and as a temporary buffer for
	// computing quantiles.
	const maxPause = len(((*runtime.MemStats)(nil)).PauseNs)
	if cap(stats.Pause) < 2*maxPause+3 {
		stats.Pause = make([]time.Duration, 2*maxPause+3)
	}

	// readGCStats fills in the pause and end times histories (up to
	// maxPause entries) and then three more: Unix ns time of last GC,
	// number of GC, and total pause time in nanoseconds. Here we
	// depend on the fact that time.Duration's native unit is
	// nanoseconds, so the pauses and the total pause time do not need
	// any conversion.
	readGCStats(&stats.Pause)
	n := len(stats.Pause) - 3
	stats.LastGC = time.Unix(0, int64(stats.Pause[n]))
	stats.NumGC = int64(stats.Pause[n+1])
	stats.PauseTotal = stats.Pause[n+2]
	n /= 2 // buffer holds pauses and end times
	stats.Pause = stats.Pause[:n]

	if cap(stats.PauseEnd) < maxPause {
		stats.PauseEnd = make([]time.Time, 0, maxPause)
	}
	stats.PauseEnd = stats.PauseEnd[:0]
	for _, ns := range stats.Pause[n : n+n] {
		stats.PauseEnd = append(stats.PauseEnd, time.Unix(0, int64(ns)))
	}

	if len(stats.PauseQuantiles) > 0 {
		if n == 0 {
			clear(stats.PauseQuantiles)
		} else {
			// There's room for a second copy of the data in stats.Pause.
			// See the allocation at the top of the function.
			sorted := stats.Pause[n : n+n]
			copy(sorted, stats.Pause)
			slices.Sort(sorted)
			nq := len(stats.PauseQuantiles) - 1
			for i := 0; i < nq; i++ {
				stats.PauseQuantiles[i] = sorted[len(sorted)*i/nq]
			}
			stats.PauseQuantiles[nq] = sorted[len(sorted)-1]
		}
	}
}

// SetGCPercent sets the garbage collection target percentage:
// a collection is triggered when the ratio of freshly allocated data
// to live data remaining after the previous collection reaches this percentage.
// SetGCPercent returns the previous setting.
// The initial setting is the value of the GOGC environment variable
// at startup, or 100 if the variable is not set.
// This setting may be effectively reduced in order to maintain a memory
// limit.
// A negative percentage effectively disables garbage collection, unless
// the memory limit is reached.
// See [SetMemoryLimit] for more details.
func SetGCPercent(percent int) int {
	return int(setGCPercent(int32(percent)))
}

// FreeOSMemory forces a garbage collection followed by an
// attempt to return as much memory to the operating system
// as possible. (Even if this is not called, the runtime gradually
// returns memory to the operating system in a background task.)
func FreeOSMemory() {
	freeOSMemory()
}

// SetMaxStack sets the maximum amount of memory that
// can be used by a single goroutine stack.
// If any goroutine exceeds this limit while growing its stack,
// the program crashes.
// SetMaxStack returns the previous setting.
// The initial setting is 1 GB on 64-bit systems, 250 MB on 32-bit systems.
// There may be a system-imposed maximum stack limit regardless
// of the value provided to SetMaxStack.
//
// SetMaxStack is useful mainly for limiting the damage done by
// goroutines that enter an infinite recursion. It only limits future
// stack growth.
func SetMaxStack(bytes int) int {
	return setMaxStack(bytes)
}

// SetMaxThreads sets the maximum number of operating system
// threads that the Go program can use. If it attempts to use more than
// this many, the program crashes.
// SetMaxThreads returns the previous setting.
// The initial setting is 10,000 threads.
//
// The limit controls the number of operating system threads, not the number
// of goroutines. A Go program creates a new thread only when a goroutine
// is ready to run but all the existing threads are blocked in system calls, cgo calls,
// or are locked to other goroutines due to use of [runtime.LockOSThread].
//
// SetMaxThreads is useful mainly for limiting the damage done by
// programs that create an unbounded number of threads. The idea is
// to take down the program before it takes down the operating system.
func SetMaxThreads(threads int) int {
	return setMaxThreads(threads)
}

// SetPanicOnFault controls the runtime's behavior when a program faults
// at an unexpected (non-nil) address. Such faults are typically caused by
// bugs such as runtime memory corruption, so the default response is to crash
// the program. Programs working with memory-mapped files or unsafe
// manipulation of memory may cause faults at non-nil addresses in less
// dramatic situations; SetPanicOnFault allows such programs to request
// that the runtime trigger only a panic, not a crash.
// The [runtime.Error] that the runtime panics with may have an additional method:
//
//	Addr() uintptr
//
// If that method exists, it returns the memory address which triggered the fault.
// The results of Addr are best-effort and the veracity of the result
// may depend on the platform.
// SetPanicOnFault applies only to the current goroutine.
// It returns the previous setting.
func SetPanicOnFault(enabled bool) bool {
	return setPanicOnFault(enabled)
}

// WriteHeapDump writes a description of the heap and the objects in
// it to the given file descriptor.
//
// WriteHeapDump suspends the execution of all goroutines until the heap
// dump is completely written.  Thus, the file descriptor must not be
// connected to a pipe or socket whose other end is in the same Go
// process; instead, use a temporary file or network socket.
//
// The heap dump format is defined at https://golang.org/s/go15heapdump.
func WriteHeapDump(fd uintptr)

// SetTraceback sets the amount of detail printed by the runtime in
// the traceback it prints before exiting due to an unrecovered panic
// or an internal runtime error.
// The level argument takes the same values as the GOTRACEBACK
// environment variable. For example, SetTraceback("all") ensure
// that the program prints all goroutines when it crashes.
// See the package runtime documentation for details.
// If SetTraceback is called with a level lower than that of the
// environment variable, the call is ignored.
func SetTraceback(level string)

// SetMemoryLimit provides the runtime with a soft memory limit.
//
// The runtime undertakes several processes to try to respect this
// memory limit, including adjustments to the frequency of garbage
// collections and returning memory to the underlying system more
// aggressively. This limit will be respected even if GOGC=off (or,
// if [SetGCPercent](-1) is executed).
//
// The input limit is provided as bytes, and includes all memory
// mapped, managed, and not released by the Go runtime. Notably, it
// does not account for space used by the Go binary and memory
// external to Go, such as memory managed by the underlying system
// on behalf of the process, or memory managed by non-Go code inside
// the same process. Examples of excluded memory sources include: OS
// kernel memory held on behalf of the process, memory allocated by
// C code, and memory mapped by syscall.Mmap (because it is not
// managed by the Go runtime).
//
// More specifically, the following expression accurately reflects
// the value the runtime attempts to maintain as the limit:
//
//	runtime.MemStats.Sys - runtime.MemStats.HeapReleased
//
// or in terms of the [runtime/metrics] package:
//
//	/memory/classes/total:bytes - /memory/classes/heap/released:bytes
//
// A zero limit or a limit that's lower than the amount of memory
// used by the Go runtime may cause the garbage collector to run
// nearly continuously. However, the application may still make
// progress.
//
// The memory limit is always respected by the Go runtime, so to
// effectively disable this behavior, set the limit very high.
// [math.MaxInt64] is the canonical value for disabling the limit,
// but values much greater than the available memory on the underlying
// system work just as well.
//
// See https://go.dev/doc/gc-guide for a detailed guide explaining
// the soft memory limit in more detail, as well as a variety of common
// use-cases and scenarios.
//
// The initial setting is [math.MaxInt64] unless the GOMEMLIMIT
// environment variable is set, in which case it provides the initial
// setting. GOMEMLIMIT is a numeric value in bytes with an optional
// unit suffix. The supported suffixes include B, KiB, MiB, GiB, and
// TiB. These suffixes represent quantities of bytes as defined by
// the IEC 80000-13 standard. That is, they are based on powers of
// two: KiB means 2^10 bytes, MiB means 2^20 bytes, and so on.
//
// SetMemoryLimit returns the previously set memory limit.
// A negative input does not adjust the limit, and allows for
// retrieval of the currently set memory limit.
func SetMemoryLimit(limit int64) int64 {
	return setMemoryLimit(limit)
}

```

// === FILE: references/go/src/runtime/debug/mod.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debug

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// exported from runtime.
func modinfo() string

// ReadBuildInfo returns the build information embedded
// in the running binary. The information is available only
// in binaries built with module support.
func ReadBuildInfo() (info *BuildInfo, ok bool) {
	data := modinfo()
	if len(data) < 32 {
		return nil, false
	}
	data = data[16 : len(data)-16]
	bi, err := ParseBuildInfo(data)
	if err != nil {
		return nil, false
	}

	// The go version is stored separately from other build info, mostly for
	// historical reasons. It is not part of the modinfo() string, and
	// ParseBuildInfo does not recognize it. We inject it here to hide this
	// awkwardness from the user.
	bi.GoVersion = runtime.Version()

	return bi, true
}

// BuildInfo represents the build information read from a Go binary.
type BuildInfo struct {
	// GoVersion is the version of the Go toolchain that built the binary
	// (for example, "go1.19.2").
	GoVersion string `json:",omitempty"`

	// Path is the package path of the main package for the binary
	// (for example, "golang.org/x/tools/cmd/stringer").
	Path string `json:",omitempty"`

	// Main describes the module that contains the main package for the binary.
	Main Module `json:""`

	// Deps describes all the dependency modules, both direct and indirect,
	// that contributed packages to the build of this binary.
	Deps []*Module `json:",omitempty"`

	// Settings describes the build settings used to build the binary.
	Settings []BuildSetting `json:",omitempty"`
}

// A Module describes a single module included in a build.
type Module struct {
	Path    string  `json:",omitempty"` // module path
	Version string  `json:",omitempty"` // module version
	Sum     string  `json:",omitempty"` // checksum
	Replace *Module `json:",omitempty"` // replaced by this module
}

// A BuildSetting is a key-value pair describing one setting that influenced a build.
//
// Defined keys include:
//
//   - -buildmode: the buildmode flag used (typically "exe")
//   - -compiler: the compiler toolchain flag used (typically "gc")
//   - CGO_ENABLED: the effective CGO_ENABLED environment variable
//   - CGO_CFLAGS: the effective CGO_CFLAGS environment variable
//   - CGO_CPPFLAGS: the effective CGO_CPPFLAGS environment variable
//   - CGO_CXXFLAGS:  the effective CGO_CXXFLAGS environment variable
//   - CGO_LDFLAGS: the effective CGO_LDFLAGS environment variable
//   - DefaultGODEBUG: the effective GODEBUG settings
//   - GOARCH: the architecture target
//   - GOAMD64/GOARM/GO386/etc: the architecture feature level for GOARCH
//   - GOOS: the operating system target
//   - GOFIPS140: the frozen FIPS 140-3 module version, if any
//   - vcs: the version control system for the source tree where the build ran
//   - vcs.revision: the revision identifier for the current commit or checkout
//   - vcs.time: the modification time associated with vcs.revision, in RFC3339 format
//   - vcs.modified: true or false indicating whether the source tree had local modifications
type BuildSetting struct {
	// Key and Value describe the build setting.
	// Key must not contain an equals sign, space, tab, or newline.
	Key string `json:",omitempty"`
	// Value must not contain newlines ('\n').
	Value string `json:",omitempty"`
}

// quoteKey reports whether key is required to be quoted.
func quoteKey(key string) bool {
	return len(key) == 0 || strings.ContainsAny(key, "= \t\r\n\"`")
}

// quoteValue reports whether value is required to be quoted.
func quoteValue(value string) bool {
	return strings.ContainsAny(value, " \t\r\n\"`")
}

// String returns a string representation of a [BuildInfo].
func (bi *BuildInfo) String() string {
	buf := new(strings.Builder)
	if bi.GoVersion != "" {
		fmt.Fprintf(buf, "go\t%s\n", bi.GoVersion)
	}
	if bi.Path != "" {
		fmt.Fprintf(buf, "path\t%s\n", bi.Path)
	}
	var formatMod func(string, Module)
	formatMod = func(word string, m Module) {
		buf.WriteString(word)
		buf.WriteByte('\t')
		buf.WriteString(m.Path)
		buf.WriteByte('\t')
		buf.WriteString(m.Version)
		if m.Replace == nil {
			buf.WriteByte('\t')
			buf.WriteString(m.Sum)
		} else {
			buf.WriteByte('\n')
			formatMod("=>", *m.Replace)
		}
		buf.WriteByte('\n')
	}
	if bi.Main != (Module{}) {
		formatMod("mod", bi.Main)
	}
	for _, dep := range bi.Deps {
		formatMod("dep", *dep)
	}
	for _, s := range bi.Settings {
		key := s.Key
		if quoteKey(key) {
			key = strconv.Quote(key)
		}
		value := s.Value
		if quoteValue(value) {
			value = strconv.Quote(value)
		}
		fmt.Fprintf(buf, "build\t%s=%s\n", key, value)
	}

	return buf.String()
}

// ParseBuildInfo parses the string returned by [*BuildInfo.String],
// restoring the original [BuildInfo],
// except that the GoVersion field is not set.
// Programs should normally not call this function,
// but instead call [ReadBuildInfo], [debug/buildinfo.ReadFile],
// or [debug/buildinfo.Read].
func ParseBuildInfo(data string) (bi *BuildInfo, err error) {
	lineNum := 1
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not parse Go build info: line %d: %w", lineNum, err)
		}
	}()

	const (
		pathLine  = "path\t"
		modLine   = "mod\t"
		depLine   = "dep\t"
		repLine   = "=>\t"
		buildLine = "build\t"
		newline   = "\n"
		tab       = "\t"
	)

	readModuleLine := func(elem []string) (Module, error) {
		if len(elem) != 2 && len(elem) != 3 {
			return Module{}, fmt.Errorf("expected 2 or 3 columns; got %d", len(elem))
		}
		version := elem[1]
		sum := ""
		if len(elem) == 3 {
			sum = elem[2]
		}
		return Module{
			Path:    elem[0],
			Version: version,
			Sum:     sum,
		}, nil
	}

	bi = new(BuildInfo)
	var (
		last *Module
		line string
		ok   bool
	)
	// Reverse of BuildInfo.String(), except for go version.
	for len(data) > 0 {
		line, data, ok = strings.Cut(data, newline)
		if !ok {
			break
		}
		switch {
		case strings.HasPrefix(line, pathLine):
			elem := line[len(pathLine):]
			bi.Path = elem
		case strings.HasPrefix(line, modLine):
			elem := strings.Split(line[len(modLine):], tab)
			last = &bi.Main
			*last, err = readModuleLine(elem)
			if err != nil {
				return nil, err
			}
		case strings.HasPrefix(line, depLine):
			elem := strings.Split(line[len(depLine):], tab)
			last = new(Module)
			bi.Deps = append(bi.Deps, last)
			*last, err = readModuleLine(elem)
			if err != nil {
				return nil, err
			}
		case strings.HasPrefix(line, repLine):
			elem := strings.Split(line[len(repLine):], tab)
			if len(elem) != 3 {
				return nil, fmt.Errorf("expected 3 columns for replacement; got %d", len(elem))
			}
			if last == nil {
				return nil, fmt.Errorf("replacement with no module on previous line")
			}
			last.Replace = &Module{
				Path:    elem[0],
				Version: elem[1],
				Sum:     elem[2],
			}
			last = nil
		case strings.HasPrefix(line, buildLine):
			kv := line[len(buildLine):]
			if len(kv) < 1 {
				return nil, fmt.Errorf("build line missing '='")
			}

			var key, rawValue string
			switch kv[0] {
			case '=':
				return nil, fmt.Errorf("build line with missing key")

			case '`', '"':
				rawKey, err := strconv.QuotedPrefix(kv)
				if err != nil {
					return nil, fmt.Errorf("invalid quoted key in build line")
				}
				if len(kv) == len(rawKey) {
					return nil, fmt.Errorf("build line missing '=' after quoted key")
				}
				if c := kv[len(rawKey)]; c != '=' {
					return nil, fmt.Errorf("unexpected character after quoted key: %q", c)
				}
				key, _ = strconv.Unquote(rawKey)
				rawValue = kv[len(rawKey)+1:]

			default:
				var ok bool
				key, rawValue, ok = strings.Cut(kv, "=")
				if !ok {
					return nil, fmt.Errorf("build line missing '=' after key")
				}
				if quoteKey(key) {
					return nil, fmt.Errorf("unquoted key %q must be quoted", key)
				}
			}

			var value string
			if len(rawValue) > 0 {
				switch rawValue[0] {
				case '`', '"':
					var err error
					value, err = strconv.Unquote(rawValue)
					if err != nil {
						return nil, fmt.Errorf("invalid quoted value in build line")
					}

				default:
					value = rawValue
					if quoteValue(value) {
						return nil, fmt.Errorf("unquoted value %q must be quoted", value)
					}
				}
			}

			bi.Settings = append(bi.Settings, BuildSetting{Key: key, Value: value})
		}
		lineNum++
	}
	return bi, nil
}

```

// === FILE: references/go/src/runtime/debug/stack.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package debug contains facilities for programs to debug themselves while
// they are running.
package debug

import (
	"internal/poll"
	"os"
	"runtime"
	_ "unsafe" // for linkname
)

// PrintStack prints to standard error the stack trace returned by [runtime.Stack].
func PrintStack() {
	os.Stderr.Write(Stack())
}

// Stack returns a formatted stack trace of the goroutine that calls it.
// It calls [runtime.Stack] with a large enough buffer to capture the entire trace.
func Stack() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}

// CrashOptions provides options that control the formatting of the
// fatal crash message.
type CrashOptions struct {
	/* for future expansion */
}

// SetCrashOutput configures a single additional file where unhandled
// panics and other fatal errors are printed, in addition to standard error.
// There is only one additional file: calling SetCrashOutput again overrides
// any earlier call.
// SetCrashOutput duplicates f's file descriptor, so the caller may safely
// close f as soon as SetCrashOutput returns.
// To disable this additional crash output, call SetCrashOutput(nil).
// If called concurrently with a crash, some in-progress output may be written
// to the old file even after an overriding SetCrashOutput returns.
func SetCrashOutput(f *os.File, opts CrashOptions) error {
	fd := ^uintptr(0)
	if f != nil {
		// The runtime will write to this file descriptor from
		// low-level routines during a panic, possibly without
		// a G, so we must call f.Fd() eagerly. This creates a
		// danger that the file descriptor is no longer
		// valid at the time of the write, because the caller
		// (incorrectly) called f.Close() and the kernel
		// reissued the fd in a later call to open(2), leading
		// to crashes being written to the wrong file.
		//
		// So, we duplicate the fd to obtain a private one
		// that cannot be closed by the user.
		// This also alleviates us from concerns about the
		// lifetime and finalization of f.
		// (DupCloseOnExec returns an fd, not a *File, so
		// there is no finalizer, and we are responsible for
		// closing it.)
		//
		// The new fd must be close-on-exec, otherwise if the
		// crash monitor is a child process, it may inherit
		// it, so it will never see EOF from the pipe even
		// when this process crashes.
		//
		// A side effect of Fd() is that it calls SetBlocking,
		// which is important so that writes of a crash report
		// to a full pipe buffer don't get lost.
		fd2, _, err := poll.DupCloseOnExec(int(f.Fd()))
		if err != nil {
			return err
		}
		runtime.KeepAlive(f) // prevent finalization before dup
		fd = uintptr(fd2)
	}
	if prev := runtime_setCrashFD(fd); prev != ^uintptr(0) {
		// We use NewFile+Close because it is portable
		// unlike syscall.Close, whose parameter type varies.
		os.NewFile(prev, "").Close() // ignore error
	}
	return nil
}

//go:linkname runtime_setCrashFD runtime.setCrashFD
func runtime_setCrashFD(uintptr) uintptr

```

// === FILE: references/go/src/runtime/debug/stubs.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debug

import (
	"time"
)

// Implemented in package runtime.
func readGCStats(*[]time.Duration)
func freeOSMemory()
func setMaxStack(int) int
func setGCPercent(int32) int32
func setPanicOnFault(bool) bool
func setMaxThreads(int) int
func setMemoryLimit(int64) int64

```

