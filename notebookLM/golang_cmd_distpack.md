# Domain Architecture: cmd/distpack

## Layout Topology
```text
cmd/distpack/
├── archive.go
├── pack.go
└── test.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/distpack/archive.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// An Archive describes an archive to write: a collection of files.
// Directories are implied by the files and not explicitly listed.
type Archive struct {
	Files []File
}

// A File describes a single file to write to an archive.
type File struct {
	Name string    // name in archive
	Time time.Time // modification time
	Mode fs.FileMode
	Size int64
	Src  string // source file in OS file system
}

// Info returns a FileInfo about the file, for use with tar.FileInfoHeader
// and zip.FileInfoHeader.
func (f *File) Info() fs.FileInfo {
	return fileInfo{f}
}

// A fileInfo is an implementation of fs.FileInfo describing a File.
type fileInfo struct {
	f *File
}

func (i fileInfo) Name() string       { return path.Base(i.f.Name) }
func (i fileInfo) ModTime() time.Time { return i.f.Time }
func (i fileInfo) Mode() fs.FileMode  { return i.f.Mode }
func (i fileInfo) IsDir() bool        { return i.f.Mode&fs.ModeDir != 0 }
func (i fileInfo) Size() int64        { return i.f.Size }
func (i fileInfo) Sys() any           { return nil }

func (i fileInfo) String() string {
	return fs.FormatFileInfo(i)
}

// NewArchive returns a new Archive containing all the files in the directory dir.
// The archive can be amended afterward using methods like Add and Filter.
func NewArchive(dir string) (*Archive, error) {
	a := new(Archive)
	err := fs.WalkDir(os.DirFS(dir), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		a.Add(name, filepath.Join(dir, name), info)
		return nil
	})
	if err != nil {
		return nil, err
	}
	a.Sort()
	return a, nil
}

// Add adds a file with the given name and info to the archive.
// The content of the file comes from the operating system file src.
// After a sequence of one or more calls to Add,
// the caller should invoke Sort to re-sort the archive's files.
func (a *Archive) Add(name, src string, info fs.FileInfo) {
	a.Files = append(a.Files, File{
		Name: name,
		Time: info.ModTime(),
		Mode: info.Mode(),
		Size: info.Size(),
		Src:  src,
	})
}

func nameLess(x, y string) bool {
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] != y[i] {
			// foo/bar/baz before foo/bar.go, because foo/bar is before foo/bar.go
			if x[i] == '/' {
				return true
			}
			if y[i] == '/' {
				return false
			}
			return x[i] < y[i]
		}
	}
	return len(x) < len(y)
}

// Sort sorts the files in the archive.
// It is only necessary to call Sort after calling Add or RenameGoMod.
// NewArchive returns a sorted archive, and the other methods
// preserve the sorting of the archive.
func (a *Archive) Sort() {
	sort.Slice(a.Files, func(i, j int) bool {
		return nameLess(a.Files[i].Name, a.Files[j].Name)
	})
}

// Clone returns a copy of the Archive.
// Method calls like Add and Filter invoked on the copy do not affect the original,
// nor do calls on the original affect the copy.
func (a *Archive) Clone() *Archive {
	b := &Archive{
		Files: make([]File, len(a.Files)),
	}
	copy(b.Files, a.Files)
	return b
}

// AddPrefix adds a prefix to all file names in the archive.
func (a *Archive) AddPrefix(prefix string) {
	for i := range a.Files {
		a.Files[i].Name = path.Join(prefix, a.Files[i].Name)
	}
}

// Filter removes files from the archive for which keep(name) returns false.
func (a *Archive) Filter(keep func(name string) bool) {
	files := a.Files[:0]
	for _, f := range a.Files {
		if keep(f.Name) {
			files = append(files, f)
		}
	}
	a.Files = files
}

// SetMode changes the mode of every file in the archive
// to be mode(name, m), where m is the file's current mode.
func (a *Archive) SetMode(mode func(name string, m fs.FileMode) fs.FileMode) {
	for i := range a.Files {
		a.Files[i].Mode = mode(a.Files[i].Name, a.Files[i].Mode)
	}
}

// Remove removes files matching any of the patterns from the archive.
// The patterns use the syntax of path.Match, with an extension of allowing
// a leading **/ or trailing /**, which match any number of path elements
// (including no path elements) before or after the main match.
func (a *Archive) Remove(patterns ...string) {
	a.Filter(func(name string) bool {
		for _, pattern := range patterns {
			match, err := amatch(pattern, name)
			if err != nil {
				log.Fatalf("archive remove: %v", err)
			}
			if match {
				return false
			}
		}
		return true
	})
}

// SetTime sets the modification time of all files in the archive to t.
func (a *Archive) SetTime(t time.Time) {
	for i := range a.Files {
		a.Files[i].Time = t
	}
}

// RenameGoMod renames the go.mod files in the archive to _go.mod,
// for use with the module form, which cannot contain other go.mod files.
func (a *Archive) RenameGoMod() {
	for i, f := range a.Files {
		if strings.HasSuffix(f.Name, "/go.mod") {
			a.Files[i].Name = strings.TrimSuffix(f.Name, "go.mod") + "_go.mod"
		}
	}
}

func amatch(pattern, name string) (bool, error) {
	// firstN returns the prefix of name corresponding to the first n path elements.
	// If n <= 0, firstN returns the entire name.
	firstN := func(name string, n int) string {
		for i := 0; i < len(name); i++ {
			if name[i] == '/' {
				if n--; n == 0 {
					return name[:i]
				}
			}
		}
		return name
	}

	// lastN returns the suffix of name corresponding to the last n path elements.
	// If n <= 0, lastN returns the entire name.
	lastN := func(name string, n int) string {
		for i := len(name) - 1; i >= 0; i-- {
			if name[i] == '/' {
				if n--; n == 0 {
					return name[i+1:]
				}
			}
		}
		return name
	}

	if p, ok := strings.CutPrefix(pattern, "**/"); ok {
		return path.Match(p, lastN(name, 1+strings.Count(p, "/")))
	}
	if p, ok := strings.CutSuffix(pattern, "/**"); ok {
		return path.Match(p, firstN(name, 1+strings.Count(p, "/")))
	}
	return path.Match(pattern, name)
}

```

// === FILE: references/go/src/cmd/distpack/pack.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Distpack creates the tgz and zip files for a Go distribution.
// It writes into GOROOT/pkg/distpack:
//
//   - a binary distribution (tgz or zip) for the current GOOS and GOARCH
//   - a source distribution that is independent of GOOS/GOARCH
//   - the module mod, info, and zip files for a distribution in module form
//     (as used by GOTOOLCHAIN support in the go command).
//
// Distpack is typically invoked by the -distpack flag to make.bash.
// A cross-compiled distribution for goos/goarch can be built using:
//
//	GOOS=goos GOARCH=goarch ./make.bash -distpack
//
// To test that the module downloads are usable with the go command:
//
//	./make.bash -distpack
//	mkdir -p /tmp/goproxy/golang.org/toolchain/
//	ln -sf $(pwd)/../pkg/distpack /tmp/goproxy/golang.org/toolchain/@v
//	GOPROXY=file:///tmp/goproxy GOTOOLCHAIN=$(sed 1q ../VERSION) gotip version
//
// gotip can be replaced with an older released Go version once there is one.
// It just can't be the one make.bash built, because it knows it is already that
// version and will skip the download.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/flate"
	"compress/gzip"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"cmd/internal/telemetry/counter"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: distpack\n")
	os.Exit(2)
}

const (
	modPath          = "golang.org/toolchain"
	modVersionPrefix = "v0.0.1"
)

var (
	goroot     string
	gohostos   string
	gohostarch string
	goos       string
	goarch     string
)

func main() {
	log.SetPrefix("distpack: ")
	log.SetFlags(0)
	counter.Open()
	flag.Usage = usage
	flag.Parse()
	counter.Inc("distpack/invocations")
	counter.CountFlags("distpack/flag:", *flag.CommandLine)
	if flag.NArg() != 0 {
		usage()
	}

	// Load context.
	goroot = runtime.GOROOT()
	if goroot == "" {
		log.Fatalf("missing $GOROOT")
	}
	gohostos = runtime.GOOS
	gohostarch = runtime.GOARCH
	goos = os.Getenv("GOOS")
	if goos == "" {
		goos = gohostos
	}
	goarch = os.Getenv("GOARCH")
	if goarch == "" {
		goarch = gohostarch
	}
	goosUnderGoarch := goos + "_" + goarch
	goosDashGoarch := goos + "-" + goarch
	exe := ""
	if goos == "windows" {
		exe = ".exe"
	}
	version, versionTime := readVERSION(goroot)

	// Start with files from GOROOT, filtering out non-distribution files.
	base, err := NewArchive(goroot)
	if err != nil {
		log.Fatal(err)
	}
	base.SetTime(versionTime)
	base.SetMode(mode)
	base.Remove(
		".git/**",
		".gitattributes",
		".github/**",
		".gitignore",
		"VERSION.cache",
		"misc/cgo/*/_obj/**",
		"**/.DS_Store",
		"**/*.exe~", // go.dev/issue/23894
		// Generated during make.bat/make.bash.
		"src/cmd/dist/dist",
		"src/cmd/dist/dist.exe",
	)

	// The source distribution removes files generated during the release build.
	// See ../dist/build.go's deptab.
	srcArch := base.Clone()
	srcArch.Remove(
		"bin/**",
		"pkg/**",

		// Generated during cmd/dist. See ../dist/build.go:/gentab.
		"src/cmd/go/internal/cfg/zdefaultcc.go",
		"src/go/build/zcgo.go",
		"src/internal/runtime/sys/zversion.go",
		"src/time/tzdata/zzipdata.go",

		// Generated during cmd/dist by bootstrapBuildTools.
		"src/cmd/cgo/zdefaultcc.go",
		"src/cmd/internal/objabi/zbootstrap.go",
		"src/internal/buildcfg/zbootstrap.go",

		// Generated by earlier versions of cmd/dist .
		"src/cmd/go/internal/cfg/zosarch.go",
	)
	srcArch.AddPrefix("go")
	testSrc(srcArch)

	// The binary distribution includes only a subset of bin and pkg.
	binArch := base.Clone()
	binArch.Filter(func(name string) bool {
		// Discard bin/ for now, will add back later.
		if strings.HasPrefix(name, "bin/") {
			return false
		}
		// Discard most of pkg.
		if strings.HasPrefix(name, "pkg/") {
			// Keep pkg/include.
			if strings.HasPrefix(name, "pkg/include/") {
				return true
			}
			// Discard other pkg except pkg/tool.
			if !strings.HasPrefix(name, "pkg/tool/") {
				return false
			}
			// Inside pkg/tool, keep only $GOOS_$GOARCH.
			if !strings.HasPrefix(name, "pkg/tool/"+goosUnderGoarch+"/") {
				return false
			}
			// Inside pkg/tool/$GOOS_$GOARCH, keep only tools needed for build actions.
			switch strings.TrimSuffix(path.Base(name), ".exe") {
			default:
				return false
			// Keep in sync with toolsIncludedInDistpack in cmd/dist/build.go.
			case "asm", "cgo", "compile", "cover", "fix", "link", "preprofile", "vet":
			}
		}
		return true
	})

	// Add go and gofmt to bin, using cross-compiled binaries
	// if this is a cross-compiled distribution.
	// Keep in sync with binExesIncludedInDistpack in cmd/dist/build.go.
	binExes := []string{
		"go",
		"gofmt",
	}
	crossBin := "bin"
	if goos != gohostos || goarch != gohostarch {
		crossBin = "bin/" + goosUnderGoarch
	}
	for _, b := range binExes {
		name := "bin/" + b + exe
		src := filepath.Join(goroot, crossBin, b+exe)
		info, err := os.Stat(src)
		if err != nil {
			log.Fatal(err)
		}
		binArch.Add(name, src, info)
	}
	binArch.Sort()
	binArch.SetTime(versionTime) // fix added files
	binArch.SetMode(mode)        // fix added files

	zipArch := binArch.Clone()
	zipArch.AddPrefix("go")
	testZip(zipArch)

	// The module distribution is the binary distribution with unnecessary files removed
	// and file names using the necessary prefix for the module.
	modArch := binArch.Clone()
	modArch.Remove(
		"api/**",
		"doc/**",
		"misc/**",
		"test/**",
	)
	modVers := modVersionPrefix + "-" + version + "." + goosDashGoarch
	modArch.AddPrefix(modPath + "@" + modVers)
	modArch.RenameGoMod()
	modArch.Sort()
	testMod(modArch)

	// distpack returns the full path to name in the distpack directory.
	distpack := func(name string) string {
		return filepath.Join(goroot, "pkg/distpack", name)
	}
	if err := os.MkdirAll(filepath.Join(goroot, "pkg/distpack"), 0777); err != nil {
		log.Fatal(err)
	}

	writeTgz(distpack(version+".src.tar.gz"), srcArch)

	if goos == "windows" {
		writeZip(distpack(version+"."+goos+"-"+goarch+".zip"), zipArch)
	} else {
		writeTgz(distpack(version+"."+goos+"-"+goarch+".tar.gz"), zipArch)
	}

	writeZip(distpack(modVers+".zip"), modArch)
	writeFile(distpack(modVers+".mod"),
		[]byte(fmt.Sprintf("module %s\n", modPath)))
	writeFile(distpack(modVers+".info"),
		[]byte(fmt.Sprintf("{%q:%q, %q:%q}\n",
			"Version", modVers,
			"Time", versionTime.Format(time.RFC3339))))
}

// mode computes the mode for the given file name.
func mode(name string, _ fs.FileMode) fs.FileMode {
	if strings.HasPrefix(name, "bin/") ||
		strings.HasPrefix(name, "pkg/tool/") ||
		strings.HasSuffix(name, ".bash") ||
		strings.HasSuffix(name, ".sh") ||
		strings.HasSuffix(name, ".pl") ||
		strings.HasSuffix(name, ".rc") {
		return 0o755
	} else if ok, _ := amatch("**/go_?*_?*_exec", name); ok {
		return 0o755
	}
	return 0o644
}

// readVERSION reads the VERSION file.
// The first line of the file is the Go version.
// Additional lines are 'key value' pairs setting other data.
// The only valid key at the moment is 'time', which sets the modification time for file archives.
func readVERSION(goroot string) (version string, t time.Time) {
	data, err := os.ReadFile(filepath.Join(goroot, "VERSION"))
	if err != nil {
		log.Fatal(err)
	}
	version, rest, _ := strings.Cut(string(data), "\n")
	for line := range strings.SplitSeq(rest, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		default:
			log.Fatalf("VERSION: unexpected line: %s", line)
		case "time":
			if len(f) != 2 {
				log.Fatalf("VERSION: unexpected time line: %s", line)
			}
			t, err = time.ParseInLocation(time.RFC3339, f[1], time.UTC)
			if err != nil {
				log.Fatalf("VERSION: bad time: %s", err)
			}
		}
	}
	return version, t
}

// writeFile writes a file with the given name and data or fatals.
func writeFile(name string, data []byte) {
	if err := os.WriteFile(name, data, 0666); err != nil {
		log.Fatal(err)
	}
	reportHash(name)
}

// check panics if err is not nil. Otherwise it returns x.
// It is only meant to be used in a function that has deferred
// a function to recover appropriately from the panic.
func check[T any](x T, err error) T {
	check1(err)
	return x
}

// check1 panics if err is not nil.
// It is only meant to be used in a function that has deferred
// a function to recover appropriately from the panic.
func check1(err error) {
	if err != nil {
		panic(err)
	}
}

// writeTgz writes the archive in tgz form to the file named name.
func writeTgz(name string, a *Archive) {
	out, err := os.Create(name)
	if err != nil {
		log.Fatal(err)
	}

	var f File
	defer func() {
		if err := recover(); err != nil {
			extra := ""
			if f.Name != "" {
				extra = " " + f.Name
			}
			log.Fatalf("writing %s%s: %v", name, extra, err)
		}
	}()

	zw := check(gzip.NewWriterLevel(out, gzip.BestCompression))
	tw := tar.NewWriter(zw)

	// Find the mode and mtime to use for directory entries,
	// based on the mode and mtime of the first file we see.
	// We know that modes and mtimes are uniform across the archive.
	var dirMode fs.FileMode
	var mtime time.Time
	for _, f := range a.Files {
		dirMode = fs.ModeDir | f.Mode | (f.Mode&0444)>>2 // copy r bits down to x bits
		mtime = f.Time
		break
	}

	// mkdirAll ensures that the tar file contains directory
	// entries for dir and all its parents. Some programs reading
	// these tar files expect that. See go.dev/issue/61862.
	haveDir := map[string]bool{".": true}
	var mkdirAll func(string)
	mkdirAll = func(dir string) {
		if dir == "/" {
			panic("mkdirAll /")
		}
		if haveDir[dir] {
			return
		}
		haveDir[dir] = true
		mkdirAll(path.Dir(dir))
		df := &File{
			Name: dir + "/",
			Time: mtime,
			Mode: dirMode,
		}
		h := check(tar.FileInfoHeader(df.Info(), ""))
		h.Name = dir + "/"
		if err := tw.WriteHeader(h); err != nil {
			panic(err)
		}
	}

	for _, f = range a.Files {
		h := check(tar.FileInfoHeader(f.Info(), ""))
		mkdirAll(path.Dir(f.Name))
		h.Name = f.Name
		if err := tw.WriteHeader(h); err != nil {
			panic(err)
		}
		r := check(os.Open(f.Src))
		check(io.Copy(tw, r))
		check1(r.Close())
	}
	f.Name = ""
	check1(tw.Close())
	check1(zw.Close())
	check1(out.Close())
	reportHash(name)
}

// writeZip writes the archive in zip form to the file named name.
func writeZip(name string, a *Archive) {
	out, err := os.Create(name)
	if err != nil {
		log.Fatal(err)
	}

	var f File
	defer func() {
		if err := recover(); err != nil {
			extra := ""
			if f.Name != "" {
				extra = " " + f.Name
			}
			log.Fatalf("writing %s%s: %v", name, extra, err)
		}
	}()

	zw := zip.NewWriter(out)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})
	for _, f = range a.Files {
		h := check(zip.FileInfoHeader(f.Info()))
		h.Name = f.Name
		h.Method = zip.Deflate
		w := check(zw.CreateHeader(h))
		r := check(os.Open(f.Src))
		check(io.Copy(w, r))
		check1(r.Close())
	}
	f.Name = ""
	check1(zw.Close())
	check1(out.Close())
	reportHash(name)
}

func reportHash(name string) {
	f, err := os.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	h := sha256.New()
	io.Copy(h, f)
	f.Close()
	fmt.Printf("distpack: %x %s\n", h.Sum(nil)[:8], filepath.Base(name))
}

```

// === FILE: references/go/src/cmd/distpack/test.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains tests applied to the archives before they are written.

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type testRule struct {
	name    string
	goos    string
	exclude bool
}

var srcRules = []testRule{
	{name: "go/VERSION"},
	{name: "go/src/cmd/go/main.go"},
	{name: "go/src/bytes/bytes.go"},
	{name: "**/.DS_Store", exclude: true},
	{name: "go/.git", exclude: true},
	{name: "go/.gitattributes", exclude: true},
	{name: "go/.github", exclude: true},
	{name: "go/VERSION.cache", exclude: true},
	{name: "go/bin/**", exclude: true},
	{name: "go/pkg/**", exclude: true},
	{name: "go/src/cmd/dist/dist", exclude: true},
	{name: "go/src/cmd/dist/dist.exe", exclude: true},
	{name: "go/src/internal/runtime/sys/zversion.go", exclude: true},
	{name: "go/src/time/tzdata/zzipdata.go", exclude: true},
}

var zipRules = []testRule{
	{name: "go/VERSION"},
	{name: "go/src/cmd/go/main.go"},
	{name: "go/src/bytes/bytes.go"},

	{name: "**/.DS_Store", exclude: true},
	{name: "go/.git", exclude: true},
	{name: "go/.gitattributes", exclude: true},
	{name: "go/.github", exclude: true},
	{name: "go/VERSION.cache", exclude: true},
	{name: "go/bin", exclude: true},
	{name: "go/pkg", exclude: true},
	{name: "go/src/cmd/dist/dist", exclude: true},
	{name: "go/src/cmd/dist/dist.exe", exclude: true},

	{name: "go/bin/go", goos: "linux"},
	{name: "go/bin/go", goos: "darwin"},
	{name: "go/bin/go", goos: "windows", exclude: true},
	{name: "go/bin/go.exe", goos: "windows"},
	{name: "go/bin/gofmt", goos: "linux"},
	{name: "go/bin/gofmt", goos: "darwin"},
	{name: "go/bin/gofmt", goos: "windows", exclude: true},
	{name: "go/bin/gofmt.exe", goos: "windows"},
	{name: "go/pkg/tool/*/compile", goos: "linux"},
	{name: "go/pkg/tool/*/compile", goos: "darwin"},
	{name: "go/pkg/tool/*/compile", goos: "windows", exclude: true},
	{name: "go/pkg/tool/*/compile.exe", goos: "windows"},
	{name: "go/pkg/tool/*/pack", exclude: true},
	{name: "go/pkg/tool/*/pack.exe", exclude: true},
}

var modRules = []testRule{
	{name: "golang.org/toolchain@*/VERSION"},
	{name: "golang.org/toolchain@*/src/cmd/go/main.go"},
	{name: "golang.org/toolchain@*/src/bytes/bytes.go"},

	{name: "golang.org/toolchain@*/lib/wasm/go_js_wasm_exec"},
	{name: "golang.org/toolchain@*/lib/wasm/go_wasip1_wasm_exec"},
	{name: "golang.org/toolchain@*/lib/wasm/wasm_exec.js"},
	{name: "golang.org/toolchain@*/lib/wasm/wasm_exec_node.js"},

	{name: "**/.DS_Store", exclude: true},
	{name: "golang.org/toolchain@*/.git", exclude: true},
	{name: "golang.org/toolchain@*/.gitattributes", exclude: true},
	{name: "golang.org/toolchain@*/.github", exclude: true},
	{name: "golang.org/toolchain@*/VERSION.cache", exclude: true},
	{name: "golang.org/toolchain@*/bin", exclude: true},
	{name: "golang.org/toolchain@*/pkg", exclude: true},
	{name: "golang.org/toolchain@*/src/cmd/dist/dist", exclude: true},
	{name: "golang.org/toolchain@*/src/cmd/dist/dist.exe", exclude: true},

	{name: "golang.org/toolchain@*/bin/go", goos: "linux"},
	{name: "golang.org/toolchain@*/bin/go", goos: "darwin"},
	{name: "golang.org/toolchain@*/bin/go", goos: "windows", exclude: true},
	{name: "golang.org/toolchain@*/bin/go.exe", goos: "windows"},
	{name: "golang.org/toolchain@*/bin/gofmt", goos: "linux"},
	{name: "golang.org/toolchain@*/bin/gofmt", goos: "darwin"},
	{name: "golang.org/toolchain@*/bin/gofmt", goos: "windows", exclude: true},
	{name: "golang.org/toolchain@*/bin/gofmt.exe", goos: "windows"},
	{name: "golang.org/toolchain@*/pkg/tool/*/compile", goos: "linux"},
	{name: "golang.org/toolchain@*/pkg/tool/*/compile", goos: "darwin"},
	{name: "golang.org/toolchain@*/pkg/tool/*/compile", goos: "windows", exclude: true},
	{name: "golang.org/toolchain@*/pkg/tool/*/compile.exe", goos: "windows"},
	{name: "golang.org/toolchain@*/pkg/tool/*/pack", exclude: true},
	{name: "golang.org/toolchain@*/pkg/tool/*/pack.exe", exclude: true},

	// go.mod are renamed to _go.mod.
	{name: "**/go.mod", exclude: true},
	{name: "**/_go.mod"},
}

func testSrc(a *Archive) {
	test("source", a, srcRules)

	// Check that no generated files slip in, even if new ones are added.
	for _, f := range a.Files {
		if strings.HasPrefix(path.Base(f.Name), "z") {
			data, err := os.ReadFile(filepath.Join(goroot, strings.TrimPrefix(f.Name, "go/")))
			if err != nil {
				log.Fatalf("checking source archive: %v", err)
			}
			if strings.Contains(string(data), "generated by go tool dist; DO NOT EDIT") {
				log.Fatalf("unexpected source archive file: %s (generated by dist)", f.Name)
			}
		}
	}
}

func testZip(a *Archive) { test("binary", a, zipRules) }
func testMod(a *Archive) { test("module", a, modRules) }

func test(kind string, a *Archive, rules []testRule) {
	ok := true
	have := make([]bool, len(rules))
	for _, f := range a.Files {
		for i, r := range rules {
			if r.goos != "" && r.goos != goos {
				continue
			}
			match, err := amatch(r.name, f.Name)
			if err != nil {
				log.Fatal(err)
			}
			if match {
				if r.exclude {
					ok = false
					if !have[i] {
						log.Printf("unexpected %s archive file: %s", kind, f.Name)
						have[i] = true // silence future prints for excluded directory
					}
				} else {
					have[i] = true
				}
			}
		}
	}
	missing := false
	for i, r := range rules {
		if r.goos != "" && r.goos != goos {
			continue
		}
		if !r.exclude && !have[i] {
			missing = true
			log.Printf("missing %s archive file: %s", kind, r.name)
		}
	}
	if missing {
		ok = false
		var buf bytes.Buffer
		for _, f := range a.Files {
			fmt.Fprintf(&buf, "\n\t%s", f.Name)
		}
		log.Printf("archive contents: %d files%s", len(a.Files), buf.Bytes())
	}
	if !ok {
		log.Fatalf("bad archive file")
	}
}

```

