# Domain Architecture: cmd/dist

## Layout Topology
```text
cmd/dist/
├── README
├── build.go
├── buildgo.go
├── buildruntime.go
├── buildtool.go
├── doc.go
├── exec.go
├── imports.go
├── main.go
├── notgo124.go
├── quoted.go
├── sys_default.go
├── sys_windows.go
├── test.go
├── testjson.go
├── util.go
├── util_gc.go
├── util_gccgo.go
├── vfp_arm.s
└── vfp_default.s
```

## Source Stream Aggregation

// === FILE: references!/go/src/cmd/dist/build.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/build/constraint"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Initialization for any invocation.

// The usual variables.
var (
	goarch           string
	gorootBin        string
	gorootBinGo      string
	gohostarch       string
	gohostos         string
	goos             string
	goarm            string
	goarm64          string
	go386            string
	goamd64          string
	gomips           string
	gomips64         string
	goppc64          string
	goriscv64        string
	goroot           string
	goextlinkenabled string
	gogcflags        string // For running built compiler
	goldflags        string
	goexperiment     string
	gofips140        string
	workdir          string
	tooldir          string
	oldgoos          string
	oldgoarch        string
	oldgocache       string
	exe              string
	defaultcc        map[string]string
	defaultcxx       map[string]string
	defaultpkgconfig string
	defaultldso      string

	rebuildall bool
	noOpt      bool
	isRelease  bool

	vflag int // verbosity
)

// The known architectures.
var okgoarch = []string{
	"386",
	"amd64",
	"arm",
	"arm64",
	"loong64",
	"mips",
	"mipsle",
	"mips64",
	"mips64le",
	"ppc64",
	"ppc64le",
	"riscv64",
	"s390x",
	"sparc64",
	"wasm",
}

// The known operating systems.
var okgoos = []string{
	"darwin",
	"dragonfly",
	"illumos",
	"ios",
	"js",
	"wasip1",
	"linux",
	"android",
	"solaris",
	"freebsd",
	"nacl", // keep;
	"netbsd",
	"openbsd",
	"plan9",
	"windows",
	"aix",
}

// xinit handles initialization of the various global state, like goroot and goarch.
func xinit() {
	b := os.Getenv("GOROOT")
	if b == "" {
		fatalf("$GOROOT must be set")
	}
	goroot = filepath.Clean(b)
	gorootBin = pathf("%s/bin", goroot)

	// Don't run just 'go' because the build infrastructure
	// runs cmd/dist inside go/bin often, and on Windows
	// it will be found in the current directory and refuse to exec.
	// All exec calls rewrite "go" into gorootBinGo.
	gorootBinGo = pathf("%s/bin/go", goroot)

	b = os.Getenv("GOOS")
	if b == "" {
		b = gohostos
	}
	goos = b
	if slices.Index(okgoos, goos) < 0 {
		fatalf("unknown $GOOS %s", goos)
	}

	b = os.Getenv("GOARM")
	if b == "" {
		b = xgetgoarm()
	}
	goarm = b

	b = os.Getenv("GOARM64")
	if b == "" {
		b = "v8.0"
	}
	goarm64 = b

	b = os.Getenv("GO386")
	if b == "" {
		b = "sse2"
	}
	go386 = b

	b = os.Getenv("GOAMD64")
	if b == "" {
		b = "v1"
	}
	goamd64 = b

	b = os.Getenv("GOMIPS")
	if b == "" {
		b = "hardfloat"
	}
	gomips = b

	b = os.Getenv("GOMIPS64")
	if b == "" {
		b = "hardfloat"
	}
	gomips64 = b

	b = os.Getenv("GOPPC64")
	if b == "" {
		b = "power8"
	}
	goppc64 = b

	b = os.Getenv("GORISCV64")
	if b == "" {
		b = "rva20u64"
	}
	goriscv64 = b

	b = os.Getenv("GOFIPS140")
	if b == "" {
		b = "off"
	}
	gofips140 = b

	if p := pathf("%s/src/all.bash", goroot); !isfile(p) {
		fatalf("$GOROOT is not set correctly or not exported\n"+
			"\tGOROOT=%s\n"+
			"\t%s does not exist", goroot, p)
	}

	b = os.Getenv("GOHOSTARCH")
	if b != "" {
		gohostarch = b
	}
	if slices.Index(okgoarch, gohostarch) < 0 {
		fatalf("unknown $GOHOSTARCH %s", gohostarch)
	}

	b = os.Getenv("GOARCH")
	if b == "" {
		b = gohostarch
	}
	goarch = b
	if slices.Index(okgoarch, goarch) < 0 {
		fatalf("unknown $GOARCH %s", goarch)
	}

	b = os.Getenv("GO_EXTLINK_ENABLED")
	if b != "" {
		if b != "0" && b != "1" {
			fatalf("unknown $GO_EXTLINK_ENABLED %s", b)
		}
		goextlinkenabled = b
	}

	goexperiment = os.Getenv("GOEXPERIMENT")
	// TODO(mdempsky): Validate known experiments?

	gogcflags = os.Getenv("BOOT_GO_GCFLAGS")
	goldflags = os.Getenv("BOOT_GO_LDFLAGS")

	defaultcc = compilerEnv("CC", "")
	defaultcxx = compilerEnv("CXX", "")

	b = os.Getenv("PKG_CONFIG")
	if b == "" {
		b = "pkg-config"
	}
	defaultpkgconfig = b

	defaultldso = os.Getenv("GO_LDSO")

	// For tools being invoked but also for os.ExpandEnv.
	os.Setenv("GO386", go386)
	os.Setenv("GOAMD64", goamd64)
	os.Setenv("GOARCH", goarch)
	os.Setenv("GOARM", goarm)
	os.Setenv("GOARM64", goarm64)
	os.Setenv("GOHOSTARCH", gohostarch)
	os.Setenv("GOHOSTOS", gohostos)
	os.Setenv("GOOS", goos)
	os.Setenv("GOMIPS", gomips)
	os.Setenv("GOMIPS64", gomips64)
	os.Setenv("GOPPC64", goppc64)
	os.Setenv("GORISCV64", goriscv64)
	os.Setenv("GOROOT", goroot)
	os.Setenv("GOFIPS140", gofips140)

	// Set GOBIN to GOROOT/bin. The meaning of GOBIN has drifted over time
	// (see https://go.dev/issue/3269, https://go.dev/cl/183058,
	// https://go.dev/issue/31576). Since we want binaries installed by 'dist' to
	// always go to GOROOT/bin anyway.
	os.Setenv("GOBIN", gorootBin)

	// Make the environment more predictable.
	os.Setenv("LANG", "C")
	os.Setenv("LANGUAGE", "en_US.UTF8")
	os.Unsetenv("GO111MODULE")
	os.Setenv("GOENV", "off")
	os.Unsetenv("GOFLAGS")
	os.Setenv("GOWORK", "off")

	// Create the go.mod for building toolchain2 and toolchain3. Toolchain1 and go_bootstrap are built with
	// a separate go.mod (with a lower required go version to allow all allowed bootstrap toolchain versions)
	// in bootstrapBuildTools.
	modVer := goModVersion()
	workdir = xworkdir()
	if err := os.WriteFile(pathf("%s/go.mod", workdir), []byte("module bootstrap\n\ngo "+modVer+"\n"), 0666); err != nil {
		fatalf("cannot write stub go.mod: %s", err)
	}
	xatexit(rmworkdir)

	tooldir = pathf("%s/pkg/tool/%s_%s", goroot, gohostos, gohostarch)

	goversion := findgoversion()
	isRelease = (strings.HasPrefix(goversion, "release.") || strings.HasPrefix(goversion, "go")) &&
		!strings.Contains(goversion, "devel")
}

// compilerEnv returns a map from "goos/goarch" to the
// compiler setting to use for that platform.
// The entry for key "" covers any goos/goarch not explicitly set in the map.
// For example, compilerEnv("CC", "gcc") returns the C compiler settings
// read from $CC, defaulting to gcc.
//
// The result is a map because additional environment variables
// can be set to change the compiler based on goos/goarch settings.
// The following applies to all envNames but CC is assumed to simplify
// the presentation.
//
// If no environment variables are set, we use def for all goos/goarch.
// $CC, if set, applies to all goos/goarch but is overridden by the following.
// $CC_FOR_TARGET, if set, applies to all goos/goarch except gohostos/gohostarch,
// but is overridden by the following.
// If gohostos=goos and gohostarch=goarch, then $CC_FOR_TARGET applies even for gohostos/gohostarch.
// $CC_FOR_goos_goarch, if set, applies only to goos/goarch.
func compilerEnv(envName, def string) map[string]string {
	m := map[string]string{"": def}

	if env := os.Getenv(envName); env != "" {
		m[""] = env
	}
	if env := os.Getenv(envName + "_FOR_TARGET"); env != "" {
		if gohostos != goos || gohostarch != goarch {
			m[gohostos+"/"+gohostarch] = m[""]
		}
		m[""] = env
	}

	for _, goos := range okgoos {
		for _, goarch := range okgoarch {
			if env := os.Getenv(envName + "_FOR_" + goos + "_" + goarch); env != "" {
				m[goos+"/"+goarch] = env
			}
		}
	}

	return m
}

// clangos lists the operating systems where we prefer clang to gcc.
var clangos = []string{
	"darwin", "ios", // macOS 10.9 and later require clang
	"freebsd", // FreeBSD 10 and later do not ship gcc
	"openbsd", // OpenBSD ships with GCC 4.2, which is now quite old.
}

// compilerEnvLookup returns the compiler settings for goos/goarch in map m.
// kind is "CC" or "CXX".
func compilerEnvLookup(kind string, m map[string]string, goos, goarch string) string {
	if !needCC() {
		return ""
	}
	if cc := m[goos+"/"+goarch]; cc != "" {
		return cc
	}
	if cc := m[""]; cc != "" {
		return cc
	}
	for _, os := range clangos {
		if goos == os {
			if kind == "CXX" {
				return "clang++"
			}
			return "clang"
		}
	}
	if kind == "CXX" {
		return "g++"
	}
	return "gcc"
}

// rmworkdir deletes the work directory.
func rmworkdir() {
	if vflag > 1 {
		errprintf("rm -rf %s\n", workdir)
	}
	xremoveall(workdir)
}

// Remove trailing spaces.
func chomp(s string) string {
	return strings.TrimRight(s, " \t\r\n")
}

// findgoversion determines the Go version to use in the version string.
// It also parses any other metadata found in the version file.
func findgoversion() string {
	// The $GOROOT/VERSION file takes priority, for distributions
	// without the source repo.
	path := pathf("%s/VERSION", goroot)
	if isfile(path) {
		b := chomp(readfile(path))

		// Starting in Go 1.21 the VERSION file starts with the
		// version on a line by itself but then can contain other
		// metadata about the release, one item per line.
		if i := strings.Index(b, "\n"); i >= 0 {
			rest := b[i+1:]
			b = chomp(b[:i])
			for line := range strings.SplitSeq(rest, "\n") {
				f := strings.Fields(line)
				if len(f) == 0 {
					continue
				}
				switch f[0] {
				default:
					fatalf("VERSION: unexpected line: %s", line)
				case "time":
					if len(f) != 2 {
						fatalf("VERSION: unexpected time line: %s", line)
					}
					_, err := time.Parse(time.RFC3339, f[1])
					if err != nil {
						fatalf("VERSION: bad time: %s", err)
					}
				}
			}
		}

		// Commands such as "dist version > VERSION" will cause
		// the shell to create an empty VERSION file and set dist's
		// stdout to its fd. dist in turn looks at VERSION and uses
		// its content if available, which is empty at this point.
		// Only use the VERSION file if it is non-empty.
		if b != "" {
			return b
		}
	}

	// The $GOROOT/VERSION.cache file is a cache to avoid invoking
	// git every time we run this command. Unlike VERSION, it gets
	// deleted by the clean command.
	path = pathf("%s/VERSION.cache", goroot)
	if isfile(path) {
		return chomp(readfile(path))
	}

	// Otherwise, use Git or jj.
	//
	// Include 1.x base version, hash, and date in the version.
	// Make sure it includes the substring "devel", but otherwise
	// use a format compatible with https://go.dev/doc/toolchain#name
	// so that it's possible to use go/version.Lang, Compare and so on.
	// See go.dev/issue/73372.
	//
	// Note that we lightly parse internal/goversion/goversion.go to
	// obtain the base version. We can't just import the package,
	// because cmd/dist is built with a bootstrap GOROOT which could
	// be an entirely different version of Go. We assume
	// that the file contains "const Version = <Integer>".
	goversionSource := readfile(pathf("%s/src/internal/goversion/goversion.go", goroot))
	m := regexp.MustCompile(`(?m)^const Version = (\d+)`).FindStringSubmatch(goversionSource)
	if m == nil {
		fatalf("internal/goversion/goversion.go does not contain 'const Version = ...'")
	}
	version := fmt.Sprintf("go1.%s-devel_", m[1])
	switch {
	case isGitRepo():
		version += chomp(run(goroot, CheckExit, "git", "log", "-n", "1", "--format=format:%h %cd", "HEAD"))
	case isJJRepo():
		const jjTemplate = `commit_id.short(10) ++ " " ++ committer.timestamp().format("%c %z")`
		version += chomp(run(goroot, CheckExit, "jj", "--no-pager", "--color=never", "log", "--no-graph", "-r", "@", "-T", jjTemplate))
	default:
		// Show a nicer error message if this isn't a Git or jj repo.
		fatalf("FAILED: not a Git or jj repo; must put a VERSION file in $GOROOT")
	}

	// Cache version.
	writefile(version, path, 0)

	return version
}

// goModVersion returns the go version declared in src/go.mod. This is the
// go version to use in the go.mod building go_bootstrap, toolchain2, and toolchain3.
// (toolchain1 must be built with requiredBootstrapVersion(goModVersion))
func goModVersion() string {
	goMod := readfile(pathf("%s/src/go.mod", goroot))
	m := regexp.MustCompile(`(?m)^go (1.\d+)$`).FindStringSubmatch(goMod)
	if m == nil {
		fatalf("std go.mod does not contain go 1.X")
	}
	return m[1]
}

func requiredBootstrapVersion(v string) string {
	minorstr, ok := strings.CutPrefix(v, "1.")
	if !ok {
		fatalf("go version %q in go.mod does not start with %q", v, "1.")
	}
	minor, err := strconv.Atoi(minorstr)
	if err != nil {
		fatalf("invalid go version minor component %q: %v", minorstr, err)
	}
	// Per go.dev/doc/install/source, for N >= 22, Go version 1.N will require a Go 1.M compiler,
	// where M is N-2 rounded down to an even number. Example: Go 1.24 and 1.25 require Go 1.22.
	requiredMinor := minor - 2 - minor%2
	return "1." + strconv.Itoa(requiredMinor)
}

// isGitRepo reports whether the working directory is inside a Git repository.
func isGitRepo() bool {
	// NB: simply checking the exit code of `git rev-parse --git-dir` would
	// suffice here, but that requires deviating from the infrastructure
	// provided by `run`.
	gitDir := chomp(run(goroot, 0, "git", "rev-parse", "--git-dir"))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(goroot, gitDir)
	}
	return isdir(gitDir)
}

// isJJRepo reports whether the working directory is inside a jj repository.
func isJJRepo() bool {
	// Don't check the error from jj, similarly to what we do in isGitRepo.
	jjDir := chomp(run(goroot, 0, "jj", "--no-pager", "--color=never", "root"))
	if !filepath.IsAbs(jjDir) {
		jjDir = filepath.Join(goroot, jjDir)
	}
	return isdir(jjDir)
}

/*
 * Initial tree setup.
 */

// The old tools that no longer live in $GOBIN or $GOROOT/bin.
var oldtool = []string{
	"5a", "5c", "5g", "5l",
	"6a", "6c", "6g", "6l",
	"8a", "8c", "8g", "8l",
	"9a", "9c", "9g", "9l",
	"6cov",
	"6nm",
	"6prof",
	"cgo",
	"ebnflint",
	"goapi",
	"gofix",
	"goinstall",
	"gomake",
	"gopack",
	"gopprof",
	"gotest",
	"gotype",
	"govet",
	"goyacc",
	"quietgcc",
}

// Unreleased directories (relative to $GOROOT) that should
// not be in release branches.
var unreleased = []string{
	"src/cmd/newlink",
	"src/cmd/objwriter",
	"src/debug/goobj",
	"src/old",
}

// setup sets up the tree for the initial build.
func setup() {
	// Create bin directory.
	if p := pathf("%s/bin", goroot); !isdir(p) {
		xmkdir(p)
	}

	// Create package directory.
	if p := pathf("%s/pkg", goroot); !isdir(p) {
		xmkdir(p)
	}

	goosGoarch := pathf("%s/pkg/%s_%s", goroot, gohostos, gohostarch)
	if rebuildall {
		xremoveall(goosGoarch)
	}
	xmkdirall(goosGoarch)
	xatexit(func() {
		if files := xreaddir(goosGoarch); len(files) == 0 {
			xremove(goosGoarch)
		}
	})

	if goos != gohostos || goarch != gohostarch {
		p := pathf("%s/pkg/%s_%s", goroot, goos, goarch)
		if rebuildall {
			xremoveall(p)
		}
		xmkdirall(p)
	}

	// Create object directory.
	// We used to use it for C objects.
	// Now we use it for the build cache, to separate dist's cache
	// from any other cache the user might have, and for the location
	// to build the bootstrap versions of the standard library.
	obj := pathf("%s/pkg/obj", goroot)
	if !isdir(obj) {
		xmkdir(obj)
	}
	xatexit(func() { xremove(obj) })

	// Create build cache directory.
	objGobuild := pathf("%s/pkg/obj/go-build", goroot)
	if rebuildall {
		xremoveall(objGobuild)
	}
	xmkdirall(objGobuild)
	xatexit(func() { xremoveall(objGobuild) })

	// Create directory for bootstrap versions of standard library .a files.
	objGoBootstrap := pathf("%s/pkg/obj/go-bootstrap", goroot)
	if rebuildall {
		xremoveall(objGoBootstrap)
	}
	xmkdirall(objGoBootstrap)
	xatexit(func() { xremoveall(objGoBootstrap) })

	// Create tool directory.
	// We keep it in pkg/, just like the object directory above.
	if rebuildall {
		xremoveall(tooldir)
	}
	xmkdirall(tooldir)

	// Remove tool binaries from before the tool/gohostos_gohostarch
	xremoveall(pathf("%s/bin/tool", goroot))

	// Remove old pre-tool binaries.
	for _, old := range oldtool {
		xremove(pathf("%s/bin/%s", goroot, old))
	}

	// Special release-specific setup.
	if isRelease {
		// Make sure release-excluded things are excluded.
		for _, dir := range unreleased {
			if p := pathf("%s/%s", goroot, dir); isdir(p) {
				fatalf("%s should not exist in release build", p)
			}
		}
	}
}

/*
 * Tool building
 */

// mustLinkExternal is a copy of internal/platform.MustLinkExternal,
// duplicated here to avoid version skew in the MustLinkExternal function
// during bootstrapping.
func mustLinkExternal(goos, goarch string, cgoEnabled bool) bool {
	if cgoEnabled {
		switch goarch {
		case "mips", "mipsle", "mips64", "mips64le":
			// Internally linking cgo is incomplete on some architectures.
			// https://golang.org/issue/14449
			return true
		case "ppc64":
			// Big Endian PPC64 cgo internal linking is not implemented for aix.
			if goos == "aix" {
				return true
			}
		}

		switch goos {
		case "android":
			return true
		case "dragonfly":
			// It seems that on Dragonfly thread local storage is
			// set up by the dynamic linker, so internal cgo linking
			// doesn't work. Test case is "go test runtime/cgo".
			return true
		}
	}

	switch goos {
	case "android":
		if goarch != "arm64" {
			return true
		}
	case "ios":
		if goarch == "arm64" {
			return true
		}
	}
	return false
}

// depsuffix records the allowed suffixes for source files.
var depsuffix = []string{
	".s",
	".go",
}

// gentab records how to generate some trivial files.
// Files listed here should also be listed in ../distpack/pack.go's srcArch.Remove list.
var gentab = []struct {
	pkg  string // Relative to $GOROOT/src
	file string
	gen  func(dir, file string)
}{
	{"cmd/go/internal/cfg", "zdefaultcc.go", mkzdefaultcc},
	{"internal/runtime/sys", "zversion.go", mkzversion},
	{"time/tzdata", "zzipdata.go", mktzdata},
}

// installed maps from a dir name (as given to install) to a chan
// closed when the dir's package is installed.
var installed = make(map[string]chan struct{})
var installedMu sync.Mutex

func install(dir string) {
	<-startInstall(dir)
}

func startInstall(dir string) chan struct{} {
	installedMu.Lock()
	ch := installed[dir]
	if ch == nil {
		ch = make(chan struct{})
		installed[dir] = ch
		go runInstall(dir, ch)
	}
	installedMu.Unlock()
	return ch
}

// runInstall installs the library, package, or binary associated with pkg,
// which is relative to $GOROOT/src.
func runInstall(pkg string, ch chan struct{}) {
	if pkg == "net" || pkg == "os/user" || pkg == "crypto/x509" {
		fatalf("go_bootstrap cannot depend on cgo package %s", pkg)
	}

	defer close(ch)

	if pkg == "unsafe" {
		return
	}

	if vflag > 0 {
		if goos != gohostos || goarch != gohostarch {
			errprintf("%s (%s/%s)\n", pkg, goos, goarch)
		} else {
			errprintf("%s\n", pkg)
		}
	}

	workdir := pathf("%s/%s", workdir, pkg)
	xmkdirall(workdir)

	var clean []string
	defer func() {
		for _, name := range clean {
			xremove(name)
		}
	}()

	// dir = full path to pkg.
	dir := pathf("%s/src/%s", goroot, pkg)
	name := filepath.Base(dir)

	// ispkg predicts whether the package should be linked as a binary, based
	// on the name. There should be no "main" packages in vendor, since
	// 'go mod vendor' will only copy imported packages there.
	ispkg := !strings.HasPrefix(pkg, "cmd/") || strings.Contains(pkg, "/internal/") || strings.Contains(pkg, "/vendor/")

	// Start final link command line.
	// Note: code below knows that link.p[targ] is the target.
	var (
		link      []string
		targ      int
		ispackcmd bool
	)
	if ispkg {
		// Go library (package).
		ispackcmd = true
		link = []string{"pack", packagefile(pkg)}
		targ = len(link) - 1
		xmkdirall(filepath.Dir(link[targ]))
	} else {
		// Go command.
		elem := name
		if elem == "go" {
			elem = "go_bootstrap"
		}
		link = []string{pathf("%s/link", tooldir)}
		if goos == "android" {
			link = append(link, "-buildmode=pie")
		}
		if goldflags != "" {
			link = append(link, goldflags)
		}
		link = append(link, "-extld="+compilerEnvLookup("CC", defaultcc, goos, goarch))
		link = append(link, "-L="+pathf("%s/pkg/obj/go-bootstrap/%s_%s", goroot, goos, goarch))
		link = append(link, "-o", pathf("%s/%s%s", tooldir, elem, exe))
		targ = len(link) - 1
	}
	ttarg := mtime(link[targ])

	// Gather files that are sources for this target.
	// Everything in that directory, and any target-specific
	// additions.
	files := xreaddir(dir)

	// Remove files beginning with . or _,
	// which are likely to be editor temporary files.
	// This is the same heuristic build.ScanDir uses.
	// There do exist real C files beginning with _,
	// so limit that check to just Go files.
	files = filter(files, func(p string) bool {
		return !strings.HasPrefix(p, ".") && (!strings.HasPrefix(p, "_") || !strings.HasSuffix(p, ".go"))
	})

	// Add generated files for this package.
	for _, gt := range gentab {
		if gt.pkg == pkg {
			files = append(files, gt.file)
		}
	}
	files = uniq(files)

	// Convert to absolute paths.
	for i, p := range files {
		if !filepath.IsAbs(p) {
			files[i] = pathf("%s/%s", dir, p)
		}
	}

	// Is the target up-to-date?
	var gofiles, sfiles []string
	stale := rebuildall
	files = filter(files, func(p string) bool {
		for _, suf := range depsuffix {
			if strings.HasSuffix(p, suf) {
				goto ok
			}
		}
		return false
	ok:
		t := mtime(p)
		if !t.IsZero() && !strings.HasSuffix(p, ".a") && !shouldbuild(p, pkg) {
			return false
		}
		if strings.HasSuffix(p, ".go") {
			gofiles = append(gofiles, p)
		} else if strings.HasSuffix(p, ".s") {
			sfiles = append(sfiles, p)
		}
		if t.After(ttarg) {
			stale = true
		}
		return true
	})

	// If there are no files to compile, we're done.
	if len(files) == 0 {
		return
	}

	if !stale {
		return
	}

	// For package runtime, copy some files into the work space.
	if pkg == "runtime" {
		xmkdirall(pathf("%s/pkg/include", goroot))
		// For use by assembly and C files.
		copyfile(pathf("%s/pkg/include/textflag.h", goroot),
			pathf("%s/src/runtime/textflag.h", goroot), 0)
		copyfile(pathf("%s/pkg/include/funcdata.h", goroot),
			pathf("%s/src/runtime/funcdata.h", goroot), 0)
		copyfile(pathf("%s/pkg/include/asm_ppc64x.h", goroot),
			pathf("%s/src/runtime/asm_ppc64x.h", goroot), 0)
		copyfile(pathf("%s/pkg/include/asm_amd64.h", goroot),
			pathf("%s/src/runtime/asm_amd64.h", goroot), 0)
		copyfile(pathf("%s/pkg/include/asm_riscv64.h", goroot),
			pathf("%s/src/runtime/asm_riscv64.h", goroot), 0)
	}

	// Generate any missing files; regenerate existing ones.
	for _, gt := range gentab {
		if gt.pkg != pkg {
			continue
		}
		p := pathf("%s/%s", dir, gt.file)
		if vflag > 1 {
			errprintf("generate %s\n", p)
		}
		gt.gen(dir, p)
		// Do not add generated file to clean list.
		// In runtime, we want to be able to
		// build the package with the go tool,
		// and it assumes these generated files already
		// exist (it does not know how to build them).
		// The 'clean' command can remove
		// the generated files.
	}

	// Resolve imported packages to actual package paths.
	// Make sure they're installed.
	importMap := make(map[string]string)
	for _, p := range gofiles {
		for _, imp := range readimports(p) {
			if imp == "C" {
				fatalf("%s imports C", p)
			}
			importMap[imp] = resolveVendor(imp, dir)
		}
	}
	sortedImports := make([]string, 0, len(importMap))
	for imp := range importMap {
		sortedImports = append(sortedImports, imp)
	}
	sort.Strings(sortedImports)

	for _, dep := range importMap {
		if dep == "C" {
			fatalf("%s imports C", pkg)
		}
		startInstall(dep)
	}
	for _, dep := range importMap {
		install(dep)
	}

	if goos != gohostos || goarch != gohostarch {
		// We've generated the right files; the go command can do the build.
		if vflag > 1 {
			errprintf("skip build for cross-compile %s\n", pkg)
		}
		return
	}

	asmArgs := []string{
		pathf("%s/asm", tooldir),
		"-I", workdir,
		"-I", pathf("%s/pkg/include", goroot),
		"-D", "GOOS_" + goos,
		"-D", "GOARCH_" + goarch,
		"-D", "GOOS_GOARCH_" + goos + "_" + goarch,
		"-p", pkg,
		"-std",
	}
	if goarch == "mips" || goarch == "mipsle" {
		// Define GOMIPS_value from gomips.
		asmArgs = append(asmArgs, "-D", "GOMIPS_"+gomips)
	}
	if goarch == "mips64" || goarch == "mips64le" {
		// Define GOMIPS64_value from gomips64.
		asmArgs = append(asmArgs, "-D", "GOMIPS64_"+gomips64)
	}
	if goarch == "ppc64" || goarch == "ppc64le" {
		// We treat each powerpc version as a superset of functionality.
		switch goppc64 {
		case "power10":
			asmArgs = append(asmArgs, "-D", "GOPPC64_power10")
			fallthrough
		case "power9":
			asmArgs = append(asmArgs, "-D", "GOPPC64_power9")
			fallthrough
		default: // This should always be power8.
			asmArgs = append(asmArgs, "-D", "GOPPC64_power8")
		}
	}
	if goarch == "riscv64" {
		// Define GORISCV64_value from goriscv64
		asmArgs = append(asmArgs, "-D", "GORISCV64_"+goriscv64)
	}
	if goarch == "arm" {
		// Define GOARM_value from goarm, which can be either a version
		// like "6", or a version and a FP mode, like "7,hardfloat".
		switch {
		case strings.Contains(goarm, "7"):
			asmArgs = append(asmArgs, "-D", "GOARM_7")
			fallthrough
		case strings.Contains(goarm, "6"):
			asmArgs = append(asmArgs, "-D", "GOARM_6")
			fallthrough
		default:
			asmArgs = append(asmArgs, "-D", "GOARM_5")
		}
	}
	goasmh := pathf("%s/go_asm.h", workdir)

	// Collect symabis from assembly code.
	var symabis string
	if len(sfiles) > 0 {
		symabis = pathf("%s/symabis", workdir)
		var wg sync.WaitGroup
		asmabis := append(asmArgs[:len(asmArgs):len(asmArgs)], "-gensymabis", "-o", symabis)
		asmabis = append(asmabis, sfiles...)
		if err := os.WriteFile(goasmh, nil, 0666); err != nil {
			fatalf("cannot write empty go_asm.h: %s", err)
		}
		bgrun(&wg, dir, asmabis...)
		bgwait(&wg)
	}

	// Build an importcfg file for the compiler.
	buf := &bytes.Buffer{}
	for _, imp := range sortedImports {
		if imp == "unsafe" {
			continue
		}
		dep := importMap[imp]
		if imp != dep {
			fmt.Fprintf(buf, "importmap %s=%s\n", imp, dep)
		}
		fmt.Fprintf(buf, "packagefile %s=%s\n", dep, packagefile(dep))
	}
	importcfg := pathf("%s/importcfg", workdir)
	if err := os.WriteFile(importcfg, buf.Bytes(), 0666); err != nil {
		fatalf("cannot write importcfg file: %v", err)
	}

	var archive string
	// The next loop will compile individual non-Go files.
	// Hand the Go files to the compiler en masse.
	// For packages containing assembly, this writes go_asm.h, which
	// the assembly files will need.
	pkgName := pkg
	if strings.HasPrefix(pkg, "cmd/") && strings.Count(pkg, "/") == 1 {
		pkgName = "main"
	}
	b := pathf("%s/_go_.a", workdir)
	clean = append(clean, b)
	if !ispackcmd {
		link = append(link, b)
	} else {
		archive = b
	}

	// Compile Go code.
	compile := []string{pathf("%s/compile", tooldir), "-std", "-pack", "-o", b, "-p", pkgName, "-importcfg", importcfg}
	if gogcflags != "" {
		compile = append(compile, strings.Fields(gogcflags)...)
	}
	if len(sfiles) > 0 {
		compile = append(compile, "-asmhdr", goasmh)
	}
	if symabis != "" {
		compile = append(compile, "-symabis", symabis)
	}
	if goos == "android" {
		compile = append(compile, "-shared")
	}

	compile = append(compile, gofiles...)
	var wg sync.WaitGroup
	// We use bgrun and immediately wait for it instead of calling run() synchronously.
	// This executes all jobs through the bgwork channel and allows the process
	// to exit cleanly in case an error occurs.
	bgrun(&wg, dir, compile...)
	bgwait(&wg)

	// Compile the files.
	for _, p := range sfiles {
		// Assembly file for a Go package.
		compile := asmArgs[:len(asmArgs):len(asmArgs)]

		doclean := true
		b := pathf("%s/%s", workdir, filepath.Base(p))

		// Change the last character of the output file (which was c or s).
		b = b[:len(b)-1] + "o"
		compile = append(compile, "-o", b, p)
		bgrun(&wg, dir, compile...)

		link = append(link, b)
		if doclean {
			clean = append(clean, b)
		}
	}
	bgwait(&wg)

	if ispackcmd {
		xremove(link[targ])
		dopack(link[targ], archive, link[targ+1:])
		return
	}

	// Remove target before writing it.
	xremove(link[targ])
	bgrun(&wg, "", link...)
	bgwait(&wg)
}

// packagefile returns the path to a compiled .a file for the given package
// path. Paths may need to be resolved with resolveVendor first.
func packagefile(pkg string) string {
	return pathf("%s/pkg/obj/go-bootstrap/%s_%s/%s.a", goroot, goos, goarch, pkg)
}

// unixOS is the set of GOOS values matched by the "unix" build tag.
// This is the same list as in internal/syslist/syslist.go.
var unixOS = map[string]bool{
	"aix":       true,
	"android":   true,
	"darwin":    true,
	"dragonfly": true,
	"freebsd":   true,
	"hurd":      true,
	"illumos":   true,
	"ios":       true,
	"linux":     true,
	"netbsd":    true,
	"openbsd":   true,
	"solaris":   true,
}

// matchtag reports whether the tag matches this build.
func matchtag(tag string) bool {
	switch tag {
	case "gc", "cmd_go_bootstrap", "go1.1":
		return true
	case "linux":
		return goos == "linux" || goos == "android"
	case "solaris":
		return goos == "solaris" || goos == "illumos"
	case "darwin":
		return goos == "darwin" || goos == "ios"
	case goos, goarch:
		return true
	case "unix":
		return unixOS[goos]
	default:
		return false
	}
}

// shouldbuild reports whether we should build this file.
// It applies the same rules that are used with context tags
// in package go/build, except it's less picky about the order
// of GOOS and GOARCH.
// We also allow the special tag cmd_go_bootstrap.
// See ../go/bootstrap.go and package go/build.
func shouldbuild(file, pkg string) bool {
	// Check file name for GOOS or GOARCH.
	name := filepath.Base(file)
	excluded := func(list []string, ok string) bool {
		for _, x := range list {
			if x == ok || (ok == "android" && x == "linux") || (ok == "illumos" && x == "solaris") || (ok == "ios" && x == "darwin") {
				continue
			}
			i := strings.Index(name, x)
			if i <= 0 || name[i-1] != '_' {
				continue
			}
			i += len(x)
			if i == len(name) || name[i] == '.' || name[i] == '_' {
				return true
			}
		}
		return false
	}
	if excluded(okgoos, goos) || excluded(okgoarch, goarch) {
		return false
	}

	// Omit test files.
	if strings.Contains(name, "_test") {
		return false
	}

	// Check file contents for //go:build lines.
	for p := range strings.SplitSeq(readfile(file), "\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		code := p
		i := strings.Index(code, "//")
		if i > 0 {
			code = strings.TrimSpace(code[:i])
		}
		if code == "package documentation" {
			return false
		}
		if code == "package main" && pkg != "cmd/go" && pkg != "cmd/cgo" {
			return false
		}
		if !strings.HasPrefix(p, "//") {
			break
		}
		if strings.HasPrefix(p, "//go:build ") {
			c, err := constraint.Parse(p)
			if err != nil {
				errprintf("%s: parsing //go:build line: %v", file, err)
				return false
			}
			return c.Eval(matchtag)
		}
	}

	return true
}

// copyfile copies the file src to dst, via memory (so only good for small files).
func copyfile(dst, src string, flag int) {
	if vflag > 1 {
		errprintf("cp %s %s\n", src, dst)
	}
	writefile(readfile(src), dst, flag)
}

// dopack copies the package src to dst,
// appending the files listed in extra.
// The archive format is the traditional Unix ar format.
func dopack(dst, src string, extra []string) {
	bdst := bytes.NewBufferString(readfile(src))
	for _, file := range extra {
		b := readfile(file)
		// find last path element for archive member name
		i := strings.LastIndex(file, "/") + 1
		j := strings.LastIndex(file, `\`) + 1
		if i < j {
			i = j
		}
		fmt.Fprintf(bdst, "%-16.16s%-12d%-6d%-6d%-8o%-10d`\n", file[i:], 0, 0, 0, 0644, len(b))
		bdst.WriteString(b)
		if len(b)&1 != 0 {
			bdst.WriteByte(0)
		}
	}
	writefile(bdst.String(), dst, 0)
}

func clean() {
	generated := []byte(generatedHeader)

	// Remove generated source files.
	filepath.WalkDir(pathf("%s/src", goroot), func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			// ignore
		case d.IsDir() && (d.Name() == "vendor" || d.Name() == "testdata"):
			return filepath.SkipDir
		case d.IsDir() && d.Name() != "dist":
			// Remove generated binary named for directory, but not dist out from under us.
			exe := filepath.Join(path, d.Name())
			if info, err := os.Stat(exe); err == nil && !info.IsDir() {
				xremove(exe)
			}
			xremove(exe + ".exe")
		case !d.IsDir() && strings.HasPrefix(d.Name(), "z"):
			// Remove generated file, identified by marker string.
			head := make([]byte, 512)
			if f, err := os.Open(path); err == nil {
				io.ReadFull(f, head)
				f.Close()
			}
			if bytes.HasPrefix(head, generated) {
				xremove(path)
			}
		}
		return nil
	})

	if rebuildall {
		// Remove object tree.
		xremoveall(pathf("%s/pkg/obj/%s_%s", goroot, gohostos, gohostarch))

		// Remove installed packages and tools.
		xremoveall(pathf("%s/pkg/%s_%s", goroot, gohostos, gohostarch))
		xremoveall(pathf("%s/pkg/%s_%s", goroot, goos, goarch))
		xremoveall(pathf("%s/pkg/%s_%s_race", goroot, gohostos, gohostarch))
		xremoveall(pathf("%s/pkg/%s_%s_race", goroot, goos, goarch))
		xremoveall(tooldir)

		// Remove cached version info.
		xremove(pathf("%s/VERSION.cache", goroot))

		// Remove distribution packages.
		xremoveall(pathf("%s/pkg/distpack", goroot))
	}
}

/*
 * command implementations
 */

// The env command prints the default environment.
func cmdenv() {
	path := flag.Bool("p", false, "emit updated PATH")
	plan9 := flag.Bool("9", gohostos == "plan9", "emit plan 9 syntax")
	windows := flag.Bool("w", gohostos == "windows", "emit windows syntax")
	xflagparse(0)

	format := "%s=\"%s\";\n" // Include ; to separate variables when 'dist env' output is used with eval.
	switch {
	case *plan9:
		format = "%s='%s'\n"
	case *windows:
		format = "set %s=%s\r\n"
	}

	xprintf(format, "GO111MODULE", "")
	xprintf(format, "GOARCH", goarch)
	xprintf(format, "GOBIN", gorootBin)
	xprintf(format, "GODEBUG", os.Getenv("GODEBUG"))
	xprintf(format, "GOENV", "off")
	xprintf(format, "GOFLAGS", "")
	xprintf(format, "GOHOSTARCH", gohostarch)
	xprintf(format, "GOHOSTOS", gohostos)
	xprintf(format, "GOOS", goos)
	xprintf(format, "GOPROXY", os.Getenv("GOPROXY"))
	xprintf(format, "GOROOT", goroot)
	xprintf(format, "GOTMPDIR", os.Getenv("GOTMPDIR"))
	xprintf(format, "GOTOOLDIR", tooldir)
	if goarch == "arm" {
		xprintf(format, "GOARM", goarm)
	}
	if goarch == "arm64" {
		xprintf(format, "GOARM64", goarm64)
	}
	if goarch == "386" {
		xprintf(format, "GO386", go386)
	}
	if goarch == "amd64" {
		xprintf(format, "GOAMD64", goamd64)
	}
	if goarch == "mips" || goarch == "mipsle" {
		xprintf(format, "GOMIPS", gomips)
	}
	if goarch == "mips64" || goarch == "mips64le" {
		xprintf(format, "GOMIPS64", gomips64)
	}
	if goarch == "ppc64" || goarch == "ppc64le" {
		xprintf(format, "GOPPC64", goppc64)
	}
	if goarch == "riscv64" {
		xprintf(format, "GORISCV64", goriscv64)
	}
	xprintf(format, "GOWORK", "off")

	if *path {
		sep := ":"
		if gohostos == "windows" {
			sep = ";"
		}
		xprintf(format, "PATH", fmt.Sprintf("%s%s%s", gorootBin, sep, os.Getenv("PATH")))

		// Also include $DIST_UNMODIFIED_PATH with the original $PATH
		// for the internal needs of "dist banner", along with export
		// so that it reaches the dist process. See its comment below.
		var exportFormat string
		if !*windows && !*plan9 {
			exportFormat = "export " + format
		} else {
			exportFormat = format
		}
		xprintf(exportFormat, "DIST_UNMODIFIED_PATH", os.Getenv("PATH"))
	}
}

var (
	timeLogEnabled = os.Getenv("GOBUILDTIMELOGFILE") != ""
	timeLogMu      sync.Mutex
	timeLogFile    *os.File
	timeLogStart   time.Time
)

func timelog(op, name string) {
	if !timeLogEnabled {
		return
	}
	timeLogMu.Lock()
	defer timeLogMu.Unlock()
	if timeLogFile == nil {
		f, err := os.OpenFile(os.Getenv("GOBUILDTIMELOGFILE"), os.O_RDWR|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal(err)
		}
		buf := make([]byte, 100)
		n, _ := f.Read(buf)
		s := string(buf[:n])
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[:i]
		}
		i := strings.Index(s, " start")
		if i < 0 {
			log.Fatalf("time log %s does not begin with start line", os.Getenv("GOBUILDTIMELOGFILE"))
		}
		t, err := time.Parse(time.UnixDate, s[:i])
		if err != nil {
			log.Fatalf("cannot parse time log line %q: %v", s, err)
		}
		timeLogStart = t
		timeLogFile = f
	}
	t := time.Now()
	fmt.Fprintf(timeLogFile, "%s %+.1fs %s %s\n", t.Format(time.UnixDate), t.Sub(timeLogStart).Seconds(), op, name)
}

// toolenv returns the environment to use when building commands in cmd.
//
// This is a function instead of a variable because the exact toolenv depends
// on the GOOS and GOARCH, and (at least for now) those are modified in place
// to switch between the host and target configurations when cross-compiling.
func toolenv() []string {
	var env []string
	if !mustLinkExternal(goos, goarch, false) {
		// Unless the platform requires external linking,
		// we disable cgo to get static binaries for cmd/go and cmd/pprof,
		// so that they work on systems without the same dynamic libraries
		// as the original build system.
		env = append(env, "CGO_ENABLED=0")
	}
	if isRelease || os.Getenv("GO_BUILDER_NAME") != "" {
		// Add -trimpath for reproducible builds of releases.
		// Include builders so that -trimpath is well-tested ahead of releases.
		// Do not include local development, so that people working in the
		// main branch for day-to-day work on the Go toolchain itself can
		// still have full paths for stack traces for compiler crashes and the like.
		env = append(env, "GOFLAGS=-trimpath -ldflags=-w -gcflags=cmd/...=-dwarf=false")
	}
	return env
}

var (
	toolchain = []string{"cmd/asm", "cmd/cgo", "cmd/compile", "cmd/link", "cmd/preprofile"}

	// Keep in sync with binExes in cmd/distpack/pack.go.
	binExesIncludedInDistpack = []string{"cmd/go", "cmd/gofmt"}

	// Keep in sync with the filter in cmd/distpack/pack.go.
	toolsIncludedInDistpack = []string{"cmd/asm", "cmd/cgo", "cmd/compile", "cmd/cover", "cmd/fix", "cmd/link", "cmd/preprofile", "cmd/vet"}

	// We could install all tools in "cmd", but is unnecessary because we will
	// remove them in distpack, so instead install the tools that will actually
	// be included in distpack, which is a superset of toolchain. Not installing
	// the tools will help us test what happens when the tools aren't present.
	toolsToInstall = slices.Concat(binExesIncludedInDistpack, toolsIncludedInDistpack)
)

// The bootstrap command runs a build from scratch,
// stopping at having installed the go_bootstrap command.
//
// WARNING: This command runs after cmd/dist is built with the Go bootstrap toolchain.
// It rebuilds and installs cmd/dist with the new toolchain, so other
// commands (like "go tool dist test" in run.bash) can rely on bug fixes
// made since the Go bootstrap version, but this function cannot.
func cmdbootstrap() {
	timelog("start", "dist bootstrap")
	defer timelog("end", "dist bootstrap")

	var debug, distpack, force, noBanner, noClean bool
	flag.BoolVar(&rebuildall, "a", rebuildall, "rebuild all")
	flag.BoolVar(&debug, "d", debug, "enable debugging of bootstrap process")
	flag.BoolVar(&distpack, "distpack", distpack, "write distribution files to pkg/distpack")
	flag.BoolVar(&force, "force", force, "build even if the port is marked as broken")
	flag.BoolVar(&noBanner, "no-banner", noBanner, "do not print banner")
	flag.BoolVar(&noClean, "no-clean", noClean, "print deprecation warning")

	xflagparse(0)

	if noClean {
		xprintf("warning: --no-clean is deprecated and has no effect; use 'go install std cmd' instead\n")
	}

	// Don't build broken ports by default.
	if broken[goos+"/"+goarch] && !force {
		fatalf("build stopped because the port %s/%s is marked as broken\n\n"+
			"Use the -force flag to build anyway.\n", goos, goarch)
	}

	// Set GOPATH to an internal directory. We shouldn't actually
	// need to store files here, since the toolchain won't
	// depend on modules outside of vendor directories, but if
	// GOPATH points somewhere else (e.g., to GOROOT), the
	// go tool may complain.
	os.Setenv("GOPATH", pathf("%s/pkg/obj/gopath", goroot))

	// Set GOPROXY=off to avoid downloading modules to the modcache in
	// the GOPATH set above to be inside GOROOT. The modcache is read
	// only so if we downloaded to the modcache, we'd create readonly
	// files in GOROOT, which is undesirable. See #67463)
	os.Setenv("GOPROXY", "off")

	// Use a build cache separate from the default user one.
	// Also one that will be wiped out during startup, so that
	// make.bash really does start from a clean slate.
	oldgocache = os.Getenv("GOCACHE")
	os.Setenv("GOCACHE", pathf("%s/pkg/obj/go-build", goroot))

	// Disable GOEXPERIMENT when building toolchain1 and
	// go_bootstrap. We don't need any experiments for the
	// bootstrap toolchain, and this lets us avoid duplicating the
	// GOEXPERIMENT-related build logic from cmd/go here. If the
	// bootstrap toolchain is < Go 1.17, it will ignore this
	// anyway since GOEXPERIMENT is baked in; otherwise it will
	// pick it up from the environment we set here. Once we're
	// using toolchain1 with dist as the build system, we need to
	// override this to keep the experiments assumed by the
	// toolchain and by dist consistent. Once go_bootstrap takes
	// over the build process, we'll set this back to the original
	// GOEXPERIMENT.
	os.Setenv("GOEXPERIMENT", "none")

	if isdir(pathf("%s/src/pkg", goroot)) {
		fatalf("\n\n"+
			"The Go package sources have moved to $GOROOT/src.\n"+
			"*** %s still exists. ***\n"+
			"It probably contains stale files that may confuse the build.\n"+
			"Please (check what's there and) remove it and try again.\n"+
			"See https://golang.org/s/go14nopkg\n",
			pathf("%s/src/pkg", goroot))
	}

	if rebuildall {
		clean()
	}

	setup()

	timelog("build", "toolchain1")
	checkCC()
	bootstrapBuildTools()

	// Remember old content of $GOROOT/bin for comparison below.
	oldBinFiles, err := filepath.Glob(pathf("%s/bin/*", goroot))
	if err != nil {
		fatalf("glob: %v", err)
	}

	// For the main bootstrap, building for host os/arch.
	oldgoos = goos
	oldgoarch = goarch
	goos = gohostos
	goarch = gohostarch
	os.Setenv("GOHOSTARCH", gohostarch)
	os.Setenv("GOHOSTOS", gohostos)
	os.Setenv("GOARCH", goarch)
	os.Setenv("GOOS", goos)

	timelog("build", "go_bootstrap")
	xprintf("Building Go bootstrap cmd/go (go_bootstrap) using Go toolchain1.\n")
	install("runtime")     // dependency not visible in sources; also sets up textflag.h
	install("time/tzdata") // no dependency in sources; creates generated file
	install("cmd/go")
	if vflag > 0 {
		xprintf("\n")
	}

	gogcflags = os.Getenv("GO_GCFLAGS") // we were using $BOOT_GO_GCFLAGS until now
	setNoOpt()
	goldflags = os.Getenv("GO_LDFLAGS") // we were using $BOOT_GO_LDFLAGS until now
	goBootstrap := pathf("%s/go_bootstrap", tooldir)
	if debug {
		run("", ShowOutput|CheckExit, pathf("%s/compile", tooldir), "-V=full")
		copyfile(pathf("%s/compile1", tooldir), pathf("%s/compile", tooldir), writeExec)
	}

	// To recap, so far we have built the new toolchain
	// (cmd/asm, cmd/cgo, cmd/compile, cmd/link, cmd/preprofile)
	// using the Go bootstrap toolchain and go command.
	// Then we built the new go command (as go_bootstrap)
	// using the new toolchain and our own build logic (above).
	//
	//	toolchain1 = mk(new toolchain, go1.17 toolchain, go1.17 cmd/go)
	//	go_bootstrap = mk(new cmd/go, toolchain1, cmd/dist)
	//
	// The toolchain1 we built earlier is built from the new sources,
	// but because it was built using cmd/go it has no build IDs.
	// The eventually installed toolchain needs build IDs, so we need
	// to do another round:
	//
	//	toolchain2 = mk(new toolchain, toolchain1, go_bootstrap)
	//
	timelog("build", "toolchain2")
	if vflag > 0 {
		xprintf("\n")
	}
	xprintf("Building Go toolchain2 using go_bootstrap and Go toolchain1.\n")
	os.Setenv("CC", compilerEnvLookup("CC", defaultcc, goos, goarch))
	// Now that cmd/go is in charge of the build process, enable GOEXPERIMENT.
	os.Setenv("GOEXPERIMENT", goexperiment)
	// No need to enable PGO for toolchain2.
	goInstall(toolenv(), goBootstrap, append([]string{"-pgo=off"}, toolchain...)...)
	if debug {
		run("", ShowOutput|CheckExit, pathf("%s/compile", tooldir), "-V=full")
		copyfile(pathf("%s/compile2", tooldir), pathf("%s/compile", tooldir), writeExec)
	}

	// Toolchain2 should be semantically equivalent to toolchain1,
	// but it was built using the newly built compiler instead of the Go bootstrap compiler,
	// so it should at the least run faster. Also, toolchain1 had no build IDs
	// in the binaries, while toolchain2 does. In non-release builds, the
	// toolchain's build IDs feed into constructing the build IDs of built targets,
	// so in non-release builds, everything now looks out-of-date due to
	// toolchain2 having build IDs - that is, due to the go command seeing
	// that there are new compilers. In release builds, the toolchain's reported
	// version is used in place of the build ID, and the go command does not
	// see that change from toolchain1 to toolchain2, so in release builds,
	// nothing looks out of date.
	// To keep the behavior the same in both non-release and release builds,
	// we force-install everything here.
	//
	//	toolchain3 = mk(new toolchain, toolchain2, go_bootstrap)
	//
	timelog("build", "toolchain3")
	if vflag > 0 {
		xprintf("\n")
	}
	xprintf("Building Go toolchain3 using go_bootstrap and Go toolchain2.\n")
	goInstall(toolenv(), goBootstrap, append([]string{"-a"}, toolchain...)...)
	if debug {
		run("", ShowOutput|CheckExit, pathf("%s/compile", tooldir), "-V=full")
		copyfile(pathf("%s/compile3", tooldir), pathf("%s/compile", tooldir), writeExec)
	}

	// Now that toolchain3 has been built from scratch, its compiler and linker
	// should have accurate build IDs suitable for caching.
	// Now prime the build cache with the rest of the standard library for
	// testing, and so that the user can run 'go install std cmd' to quickly
	// iterate on local changes without waiting for a full rebuild.
	if _, err := os.Stat(pathf("%s/VERSION", goroot)); err == nil {
		// If we have a VERSION file, then we use the Go version
		// instead of build IDs as a cache key, and there is no guarantee
		// that code hasn't changed since the last time we ran a build
		// with this exact VERSION file (especially if someone is working
		// on a release branch). We must not fall back to the shared build cache
		// in this case. Leave $GOCACHE alone.
	} else {
		os.Setenv("GOCACHE", oldgocache)
	}

	if goos == oldgoos && goarch == oldgoarch {
		// Common case - not setting up for cross-compilation.
		timelog("build", "toolchain")
		if vflag > 0 {
			xprintf("\n")
		}
		xprintf("Building packages and commands for %s/%s.\n", goos, goarch)
	} else {
		// GOOS/GOARCH does not match GOHOSTOS/GOHOSTARCH.
		// Finish GOHOSTOS/GOHOSTARCH installation and then
		// run GOOS/GOARCH installation.
		timelog("build", "host toolchain")
		if vflag > 0 {
			xprintf("\n")
		}
		xprintf("Building commands for host, %s/%s.\n", goos, goarch)
		goInstall(toolenv(), goBootstrap, toolsToInstall...)
		checkNotStale(toolenv(), goBootstrap, toolsToInstall...)
		checkNotStale(toolenv(), gorootBinGo, toolsToInstall...)

		timelog("build", "target toolchain")
		if vflag > 0 {
			xprintf("\n")
		}
		goos = oldgoos
		goarch = oldgoarch
		os.Setenv("GOOS", goos)
		os.Setenv("GOARCH", goarch)
		os.Setenv("CC", compilerEnvLookup("CC", defaultcc, goos, goarch))
		xprintf("Building packages and commands for target, %s/%s.\n", goos, goarch)
	}
	goInstall(nil, goBootstrap, "std")
	goInstall(toolenv(), goBootstrap, toolsToInstall...)
	checkNotStale(toolenv(), goBootstrap, toolchain...)
	checkNotStale(nil, goBootstrap, "std")
	checkNotStale(toolenv(), goBootstrap, toolsToInstall...)
	checkNotStale(nil, gorootBinGo, "std")
	checkNotStale(toolenv(), gorootBinGo, toolsToInstall...)
	if debug {
		run("", ShowOutput|CheckExit, pathf("%s/compile", tooldir), "-V=full")
		checkNotStale(toolenv(), goBootstrap, toolchain...)
		copyfile(pathf("%s/compile4", tooldir), pathf("%s/compile", tooldir), writeExec)
	}

	// Check that there are no new files in $GOROOT/bin other than
	// go and gofmt and $GOOS_$GOARCH (target bin when cross-compiling).
	binFiles, err := filepath.Glob(pathf("%s/bin/*", goroot))
	if err != nil {
		fatalf("glob: %v", err)
	}

	ok := map[string]bool{}
	for _, f := range oldBinFiles {
		ok[f] = true
	}
	for _, f := range binFiles {
		if gohostos == "darwin" && filepath.Base(f) == ".DS_Store" {
			continue // unfortunate but not unexpected
		}
		elem := strings.TrimSuffix(filepath.Base(f), ".exe")
		if !ok[f] && elem != "go" && elem != "gofmt" && elem != goos+"_"+goarch {
			fatalf("unexpected new file in $GOROOT/bin: %s", elem)
		}
	}

	// Remove go_bootstrap now that we're done.
	xremove(pathf("%s/go_bootstrap"+exe, tooldir))

	if goos == "android" {
		// Make sure the exec wrapper will sync a fresh $GOROOT to the device.
		xremove(pathf("%s/go_android_exec-adb-sync-status", os.TempDir()))
	}

	if wrapperPath := wrapperPathFor(goos, goarch); wrapperPath != "" {
		oldcc := os.Getenv("CC")
		os.Setenv("GOOS", gohostos)
		os.Setenv("GOARCH", gohostarch)
		os.Setenv("CC", compilerEnvLookup("CC", defaultcc, gohostos, gohostarch))
		goCmd(nil, gorootBinGo, "build", "-o", pathf("%s/go_%s_%s_exec%s", gorootBin, goos, goarch, exe), wrapperPath)
		// Restore environment.
		// TODO(elias.naur): support environment variables in goCmd?
		os.Setenv("GOOS", goos)
		os.Setenv("GOARCH", goarch)
		os.Setenv("CC", oldcc)
	}

	if distpack {
		xprintf("Packaging archives for %s/%s.\n", goos, goarch)
		run("", ShowOutput|CheckExit, gorootBinGo, "tool", "distpack")
	}

	// Print trailing banner unless instructed otherwise.
	if !noBanner {
		banner()
	}
}

func wrapperPathFor(goos, goarch string) string {
	switch {
	case goos == "android":
		if gohostos != "android" {
			return pathf("%s/misc/go_android_exec/main.go", goroot)
		}
	case goos == "ios":
		if gohostos != "ios" {
			return pathf("%s/misc/ios/go_ios_exec.go", goroot)
		}
	}
	return ""
}

func goInstall(env []string, goBinary string, args ...string) {
	goCmd(env, goBinary, "install", args...)
}

func appendCompilerFlags(args []string) []string {
	if gogcflags != "" {
		args = append(args, "-gcflags=all="+gogcflags)
	}
	if goldflags != "" {
		args = append(args, "-ldflags=all="+goldflags)
	}
	return args
}

func goCmd(env []string, goBinary string, cmd string, args ...string) {
	goCmd := []string{goBinary, cmd}
	if noOpt {
		goCmd = append(goCmd, "-tags=noopt")
	}
	goCmd = appendCompilerFlags(goCmd)
	if vflag > 0 {
		goCmd = append(goCmd, "-v")
	}

	// Force only one process at a time on vx32 emulation.
	if gohostos == "plan9" && os.Getenv("sysname") == "vx32" {
		goCmd = append(goCmd, "-p=1")
	}

	runEnv(workdir, ShowOutput|CheckExit, env, append(goCmd, args...)...)
}

func checkNotStale(env []string, goBinary string, targets ...string) {
	goCmd := []string{goBinary, "list"}
	if noOpt {
		goCmd = append(goCmd, "-tags=noopt")
	}
	goCmd = appendCompilerFlags(goCmd)
	goCmd = append(goCmd, "-f={{if .Stale}}\tSTALE {{.ImportPath}}: {{.StaleReason}}{{end}}")

	out := runEnv(workdir, CheckExit, env, append(goCmd, targets...)...)
	if strings.Contains(out, "\tSTALE ") {
		os.Setenv("GODEBUG", "gocachehash=1")
		for _, target := range []string{"internal/runtime/sys", "cmd/dist", "cmd/link"} {
			if strings.Contains(out, "STALE "+target) {
				run(workdir, ShowOutput|CheckExit, goBinary, "list", "-f={{.ImportPath}} {{.Stale}}", target)
				break
			}
		}
		fatalf("unexpected stale targets reported by %s list -gcflags=\"%s\" -ldflags=\"%s\" for %v (consider rerunning with GOMAXPROCS=1 GODEBUG=gocachehash=1):\n%s", goBinary, gogcflags, goldflags, targets, out)
	}
}

// Cannot use go/build directly because cmd/dist for a new release
// builds against an old release's go/build, which may be out of sync.
// To reduce duplication, we generate the list for go/build from this.
//
// We list all supported platforms in this list, so that this is the
// single point of truth for supported platforms. This list is used
// by 'go tool dist list'.
var cgoEnabled = map[string]bool{
	"aix/ppc64":       true,
	"darwin/amd64":    true,
	"darwin/arm64":    true,
	"dragonfly/amd64": true,
	"freebsd/386":     true,
	"freebsd/amd64":   true,
	"freebsd/arm":     true,
	"freebsd/arm64":   true,
	"freebsd/riscv64": true,
	"illumos/amd64":   true,
	"linux/386":       true,
	"linux/amd64":     true,
	"linux/arm":       true,
	"linux/arm64":     true,
	"linux/loong64":   true,
	"linux/ppc64":     true,
	"linux/ppc64le":   true,
	"linux/mips":      true,
	"linux/mipsle":    true,
	"linux/mips64":    true,
	"linux/mips64le":  true,
	"linux/riscv64":   true,
	"linux/s390x":     true,
	"linux/sparc64":   true,
	"android/386":     true,
	"android/amd64":   true,
	"android/arm":     true,
	"android/arm64":   true,
	"ios/arm64":       true,
	"ios/amd64":       true,
	"js/wasm":         false,
	"wasip1/wasm":     false,
	"netbsd/386":      true,
	"netbsd/amd64":    true,
	"netbsd/arm":      true,
	"netbsd/arm64":    true,
	"openbsd/386":     true,
	"openbsd/amd64":   true,
	"openbsd/arm":     true,
	"openbsd/arm64":   true,
	"openbsd/ppc64":   false,
	"openbsd/riscv64": true,
	"plan9/386":       false,
	"plan9/amd64":     false,
	"plan9/arm":       false,
	"solaris/amd64":   true,
	"windows/386":     true,
	"windows/amd64":   true,
	"windows/arm64":   true,
}

// List of platforms that are marked as broken ports.
// These require -force flag to build, and also
// get filtered out of cgoEnabled for 'dist list'.
// See go.dev/issue/56679.
var broken = map[string]bool{
	"freebsd/riscv64": true, // Broken: go.dev/issue/76475.
	"linux/sparc64":   true, // An incomplete port. See CL 132155.
}

// List of platforms which are first class ports. See go.dev/issue/38874.
var firstClass = map[string]bool{
	"darwin/amd64":  true,
	"darwin/arm64":  true,
	"linux/386":     true,
	"linux/amd64":   true,
	"linux/arm":     true,
	"linux/arm64":   true,
	"windows/386":   true,
	"windows/amd64": true,
}

// We only need CC if cgo is forced on, or if the platform requires external linking.
// Otherwise the go command will automatically disable it.
func needCC() bool {
	return os.Getenv("CGO_ENABLED") == "1" || mustLinkExternal(gohostos, gohostarch, false)
}

func checkCC() {
	if !needCC() {
		return
	}
	cc1 := defaultcc[""]
	if cc1 == "" {
		cc1 = "gcc"
		for _, os := range clangos {
			if gohostos == os {
				cc1 = "clang"
				break
			}
		}
	}
	cc, err := quotedSplit(cc1)
	if err != nil {
		fatalf("split CC: %v", err)
	}
	var ccHelp = append(cc, "--help")

	if output, err := exec.Command(ccHelp[0], ccHelp[1:]...).CombinedOutput(); err != nil {
		outputHdr := ""
		if len(output) > 0 {
			outputHdr = "\nCommand output:\n\n"
		}
		fatalf("cannot invoke C compiler %q: %v\n\n"+
			"Go needs a system C compiler for use with cgo.\n"+
			"To set a C compiler, set CC=the-compiler.\n"+
			"To disable cgo, set CGO_ENABLED=0.\n%s%s", cc, err, outputHdr, output)
	}
}

func defaulttarg() string {
	// xgetwd might return a path with symlinks fully resolved, and if
	// there happens to be symlinks in goroot, then the hasprefix test
	// will never succeed. Instead, we use xrealwd to get a canonical
	// goroot/src before the comparison to avoid this problem.
	pwd := xgetwd()
	src := pathf("%s/src/", goroot)
	real_src := xrealwd(src)
	if !strings.HasPrefix(pwd, real_src) {
		fatalf("current directory %s is not under %s", pwd, real_src)
	}
	pwd = pwd[len(real_src):]
	// guard against xrealwd returning the directory without the trailing /
	pwd = strings.TrimPrefix(pwd, "/")

	return pwd
}

// Install installs the list of packages named on the command line.
func cmdinstall() {
	xflagparse(-1)

	if flag.NArg() == 0 {
		install(defaulttarg())
	}

	for _, arg := range flag.Args() {
		install(arg)
	}
}

// Clean deletes temporary objects.
func cmdclean() {
	xflagparse(0)
	clean()
}

// Banner prints the 'now you've installed Go' banner.
func cmdbanner() {
	xflagparse(0)
	banner()
}

func banner() {
	if vflag > 0 {
		xprintf("\n")
	}
	xprintf("---\n")
	xprintf("Installed Go for %s/%s in %s\n", goos, goarch, goroot)
	xprintf("Installed commands in %s\n", gorootBin)

	if gohostos == "plan9" {
		// Check that GOROOT/bin is bound before /bin.
		pid := strings.ReplaceAll(readfile("#c/pid"), " ", "")
		ns := fmt.Sprintf("/proc/%s/ns", pid)
		if !strings.Contains(readfile(ns), fmt.Sprintf("bind -b %s /bin", gorootBin)) {
			xprintf("*** You need to bind %s before /bin.\n", gorootBin)
		}
	} else {
		// Check that GOROOT/bin appears in $PATH.
		pathsep := ":"
		if gohostos == "windows" {
			pathsep = ";"
		}
		path := os.Getenv("PATH")
		if p, ok := os.LookupEnv("DIST_UNMODIFIED_PATH"); ok {
			// Scripts that modify $PATH and then run dist should also provide
			// dist with an unmodified copy of $PATH via $DIST_UNMODIFIED_PATH.
			// Use it here when determining if the user still needs to update
			// their $PATH. See go.dev/issue/42563.
			path = p
		}
		if !strings.Contains(pathsep+path+pathsep, pathsep+gorootBin+pathsep) {
			xprintf("*** You need to add %s to your PATH.\n", gorootBin)
		}
	}
}

// Version prints the Go version.
func cmdversion() {
	xflagparse(0)
	xprintf("%s\n", findgoversion())
}

// cmdlist lists all supported platforms.
func cmdlist() {
	jsonFlag := flag.Bool("json", false, "produce JSON output")
	brokenFlag := flag.Bool("broken", false, "include broken ports")
	xflagparse(0)

	var plats []string
	for p := range cgoEnabled {
		if broken[p] && !*brokenFlag {
			continue
		}
		plats = append(plats, p)
	}
	sort.Strings(plats)

	if !*jsonFlag {
		for _, p := range plats {
			xprintf("%s\n", p)
		}
		return
	}

	type jsonResult struct {
		GOOS         string
		GOARCH       string
		CgoSupported bool
		FirstClass   bool
		Broken       bool `json:",omitempty"`
	}
	var results []jsonResult
	for _, p := range plats {
		fields := strings.Split(p, "/")
		results = append(results, jsonResult{
			GOOS:         fields[0],
			GOARCH:       fields[1],
			CgoSupported: cgoEnabled[p],
			FirstClass:   firstClass[p],
			Broken:       broken[p],
		})
	}
	out, err := json.MarshalIndent(results, "", "\t")
	if err != nil {
		fatalf("json marshal error: %v", err)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		fatalf("write failed: %v", err)
	}
}

func setNoOpt() {
	for gcflag := range strings.SplitSeq(gogcflags, " ") {
		if gcflag == "-N" || gcflag == "-l" {
			noOpt = true
			break
		}
	}
}

```

// === FILE: references!/go/src/cmd/dist/buildgo.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

/*
 * Helpers for building cmd/go and cmd/cgo.
 */

// generatedHeader is the string that all source files generated by dist start with.
//
// DO NOT CHANGE THIS STRING. If this string is changed then during
//
//	./make.bash
//	git checkout other-rev
//	./make.bash
//
// the second make.bash will not find the files generated by the first make.bash
// and will not clean up properly.
const generatedHeader = "// Code generated by go tool dist; DO NOT EDIT.\n\n"

// writeHeader emits the standard "generated by" header for all files generated
// by dist.
func writeHeader(w io.Writer) {
	fmt.Fprint(w, generatedHeader)
}

// mkzdefaultcc writes zdefaultcc.go:
//
//	package main
//	const defaultCC = <defaultcc>
//	const defaultCXX = <defaultcxx>
//	const defaultPkgConfig = <defaultpkgconfig>
//
// It is invoked to write cmd/go/internal/cfg/zdefaultcc.go
// but we also write cmd/cgo/zdefaultcc.go
func mkzdefaultcc(dir, file string) {
	if strings.Contains(file, filepath.FromSlash("go/internal/cfg")) {
		var buf strings.Builder
		writeHeader(&buf)
		fmt.Fprintf(&buf, "package cfg\n")
		fmt.Fprintln(&buf)
		fmt.Fprintf(&buf, "const DefaultPkgConfig = `%s`\n", defaultpkgconfig)
		buf.WriteString(defaultCCFunc("DefaultCC", defaultcc))
		buf.WriteString(defaultCCFunc("DefaultCXX", defaultcxx))
		writefile(buf.String(), file, writeSkipSame)
		return
	}

	var buf strings.Builder
	writeHeader(&buf)
	fmt.Fprintf(&buf, "package main\n")
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "const defaultPkgConfig = `%s`\n", defaultpkgconfig)
	buf.WriteString(defaultCCFunc("defaultCC", defaultcc))
	buf.WriteString(defaultCCFunc("defaultCXX", defaultcxx))
	writefile(buf.String(), file, writeSkipSame)
}

func defaultCCFunc(name string, defaultcc map[string]string) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "func %s(goos, goarch string) string {\n", name)
	fmt.Fprintf(&buf, "\tswitch goos+`/`+goarch {\n")
	var keys []string
	for k := range defaultcc {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&buf, "\tcase %s:\n\t\treturn %s\n", quote(k), quote(defaultcc[k]))
	}
	fmt.Fprintf(&buf, "\t}\n")
	if cc := defaultcc[""]; cc != "" {
		fmt.Fprintf(&buf, "\treturn %s\n", quote(cc))
	} else {
		clang, gcc := "clang", "gcc"
		if strings.HasSuffix(name, "CXX") {
			clang, gcc = "clang++", "g++"
		}
		fmt.Fprintf(&buf, "\tswitch goos {\n")
		fmt.Fprintf(&buf, "\tcase ")
		for i, os := range clangos {
			if i > 0 {
				fmt.Fprintf(&buf, ", ")
			}
			fmt.Fprintf(&buf, "%s", quote(os))
		}
		fmt.Fprintf(&buf, ":\n")
		fmt.Fprintf(&buf, "\t\treturn %s\n", quote(clang))
		fmt.Fprintf(&buf, "\t}\n")
		fmt.Fprintf(&buf, "\treturn %s\n", quote(gcc))
	}
	fmt.Fprintf(&buf, "}\n")

	return buf.String()
}

// mktzdata src/time/tzdata/zzipdata.go:
//
//	package tzdata
//	const zipdata = "PK..."
func mktzdata(dir, file string) {
	zip := readfile(filepath.Join(dir, "../../../lib/time/zoneinfo.zip"))

	var buf strings.Builder
	writeHeader(&buf)
	fmt.Fprintf(&buf, "package tzdata\n")
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "const zipdata = %s\n", quote(zip))

	writefile(buf.String(), file, writeSkipSame)
}

// quote is like strconv.Quote but simpler and has output
// that does not depend on the exact Go bootstrap version.
func quote(s string) string {
	const hex = "0123456789abcdef"
	var out strings.Builder
	out.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 0x20 <= c && c <= 0x7E && c != '"' && c != '\\' {
			out.WriteByte(c)
		} else {
			out.WriteByte('\\')
			out.WriteByte('x')
			out.WriteByte(hex[c>>4])
			out.WriteByte(hex[c&0xf])
		}
	}
	out.WriteByte('"')
	return out.String()
}

```

// === FILE: references!/go/src/cmd/dist/buildruntime.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"strings"
)

/*
 * Helpers for building runtime.
 */

// mkzversion writes zversion.go:
//
//	package sys
//
// (Nothing right now!)
func mkzversion(dir, file string) {
	var buf strings.Builder
	writeHeader(&buf)
	fmt.Fprintf(&buf, "package sys\n")
	writefile(buf.String(), file, writeSkipSame)
}

// mkbuildcfg writes internal/buildcfg/zbootstrap.go:
//
//	package buildcfg
//
//	const defaultGOROOT = <goroot>
//	const defaultGO386 = <go386>
//	...
//	const defaultGOOS = runtime.GOOS
//	const defaultGOARCH = runtime.GOARCH
//
// The use of runtime.GOOS and runtime.GOARCH makes sure that
// a cross-compiled compiler expects to compile for its own target
// system. That is, if on a Mac you do:
//
//	GOOS=linux GOARCH=ppc64 go build cmd/compile
//
// the resulting compiler will default to generating linux/ppc64 object files.
// This is more useful than having it default to generating objects for the
// original target (in this example, a Mac).
func mkbuildcfg(file string) {
	var buf strings.Builder
	writeHeader(&buf)
	fmt.Fprintf(&buf, "package buildcfg\n")
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "import \"runtime\"\n")
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "const DefaultGO386 = `%s`\n", go386)
	fmt.Fprintf(&buf, "const DefaultGOAMD64 = `%s`\n", goamd64)
	fmt.Fprintf(&buf, "const DefaultGOARM = `%s`\n", goarm)
	fmt.Fprintf(&buf, "const DefaultGOARM64 = `%s`\n", goarm64)
	fmt.Fprintf(&buf, "const DefaultGOMIPS = `%s`\n", gomips)
	fmt.Fprintf(&buf, "const DefaultGOMIPS64 = `%s`\n", gomips64)
	fmt.Fprintf(&buf, "const DefaultGOPPC64 = `%s`\n", goppc64)
	fmt.Fprintf(&buf, "const DefaultGORISCV64 = `%s`\n", goriscv64)
	fmt.Fprintf(&buf, "const defaultGOEXPERIMENT = `%s`\n", goexperiment)
	fmt.Fprintf(&buf, "const defaultGO_EXTLINK_ENABLED = `%s`\n", goextlinkenabled)
	fmt.Fprintf(&buf, "const defaultGO_LDSO = `%s`\n", defaultldso)
	fmt.Fprintf(&buf, "const version = `%s`\n", findgoversion())
	fmt.Fprintf(&buf, "const defaultGOOS = runtime.GOOS\n")
	fmt.Fprintf(&buf, "const defaultGOARCH = runtime.GOARCH\n")
	fmt.Fprintf(&buf, "const DefaultGOFIPS140 = `%s`\n", gofips140)
	fmt.Fprintf(&buf, "const DefaultCGO_ENABLED = %s\n", quote(os.Getenv("CGO_ENABLED")))

	writefile(buf.String(), file, writeSkipSame)
}

// mkobjabi writes cmd/internal/objabi/zbootstrap.go:
//
//	package objabi
//
// (Nothing right now!)
func mkobjabi(file string) {
	var buf strings.Builder
	writeHeader(&buf)
	fmt.Fprintf(&buf, "package objabi\n")

	writefile(buf.String(), file, writeSkipSame)
}

```

// === FILE: references!/go/src/cmd/dist/buildtool.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Build toolchain using Go bootstrap version.
//
// The general strategy is to copy the source files we need into
// a new GOPATH workspace, adjust import paths appropriately,
// invoke the Go bootstrap toolchains go command to build those sources,
// and then copy the binaries back.

package main

import (
	"fmt"
	"go/version"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// bootstrapDirs is a list of directories holding code that must be
// compiled with the Go bootstrap toolchain to produce the bootstrapTargets.
// All directories in this list are relative to and must be below $GOROOT/src.
//
// The list has two kinds of entries: names beginning with cmd/ with
// no other slashes, which are commands, and other paths, which are packages
// supporting the commands. Packages in the standard library can be listed
// if a newer copy needs to be substituted for the Go bootstrap copy when used
// by the command packages. Paths ending with /... automatically
// include all packages within subdirectories as well.
// These will be imported during bootstrap as bootstrap/name, like bootstrap/math/big.
var bootstrapDirs = []string{
	"cmp",
	"cmd/asm",
	"cmd/asm/internal/...",
	"cmd/cgo",
	"cmd/compile",
	"cmd/compile/internal/...",
	"cmd/internal/archive",
	"cmd/internal/bio",
	"cmd/internal/codesign",
	"cmd/internal/dwarf",
	"cmd/internal/edit",
	"cmd/internal/gcprog",
	"cmd/internal/goobj",
	"cmd/internal/hash",
	"cmd/internal/macho",
	"cmd/internal/obj/...",
	"cmd/internal/objabi",
	"cmd/internal/par",
	"cmd/internal/pgo",
	"cmd/internal/pkgpath",
	"cmd/internal/quoted",
	"cmd/internal/src",
	"cmd/internal/sys",
	"cmd/internal/telemetry",
	"cmd/internal/telemetry/counter",
	"cmd/link",
	"cmd/link/internal/...",
	"compress/flate",
	"compress/zlib",
	"container/heap",
	"debug/dwarf",
	"debug/elf",
	"debug/macho",
	"debug/pe",
	"go/build/constraint",
	"go/constant",
	"go/version",
	"internal/abi",
	"internal/coverage",
	"cmd/internal/cov/covcmd",
	"internal/bisect",
	"internal/buildcfg",
	"internal/exportdata",
	"internal/goarch",
	"internal/godebugs",
	"internal/goexperiment",
	"internal/goroot",
	"internal/gover",
	"internal/goversion",
	// internal/lazyregexp is provided by Go 1.17, which permits it to
	// be imported by other packages in this list, but is not provided
	// by the Go 1.17 version of gccgo. It's on this list only to
	// support gccgo, and can be removed if we require gccgo 14 or later.
	"internal/lazyregexp",
	"internal/pkgbits",
	"internal/platform",
	"internal/profile",
	"internal/race",
	"internal/runtime/gc",
	"internal/saferio",
	"internal/strconv",
	"internal/syscall/unix",
	"internal/types/errors",
	"internal/unsafeheader",
	"internal/xcoff",
	"internal/zstd",
	"math/bits",
	"sort",
}

// File prefixes that are ignored by go/build anyway, and cause
// problems with editor generated temporary files (#18931).
var ignorePrefixes = []string{
	".",
	"_",
	"#",
}

// File suffixes that use build tags introduced since Go 1.17.
// These must not be copied into the bootstrap build directory.
// Also ignore test files.
var ignoreSuffixes = []string{
	"_test.s",
	"_test.go",
	// Skip PGO profile. No need to build toolchain1 compiler
	// with PGO. And as it is not a text file the import path
	// rewrite will break it.
	".pgo",
	// Skip editor backup files.
	"~",
}

const minBootstrap = "go1.24.6"

var tryDirs = []string{
	"sdk/" + minBootstrap,
	minBootstrap,
}

func bootstrapBuildTools() {
	goroot_bootstrap := os.Getenv("GOROOT_BOOTSTRAP")
	if goroot_bootstrap == "" {
		home := os.Getenv("HOME")
		goroot_bootstrap = pathf("%s/go1.4", home)
		for _, d := range tryDirs {
			if p := pathf("%s/%s", home, d); isdir(p) {
				goroot_bootstrap = p
			}
		}
	}

	// check bootstrap version.
	ver := run(pathf("%s/bin", goroot_bootstrap), CheckExit, pathf("%s/bin/go", goroot_bootstrap), "env", "GOVERSION")
	// go env GOVERSION output like "go1.22.6\n" or "devel go1.24-ffb3e574 Thu Aug 29 20:16:26 2024 +0000\n".
	ver = ver[:len(ver)-1]
	if version.Compare(ver, version.Lang(minBootstrap)) > 0 && version.Compare(ver, minBootstrap) < 0 {
		fatalf("%s does not meet the minimum bootstrap requirement of %s or later", ver, minBootstrap)
	}

	xprintf("Building Go toolchain1 using %s.\n", goroot_bootstrap)

	mkbuildcfg(pathf("%s/src/internal/buildcfg/zbootstrap.go", goroot))
	mkobjabi(pathf("%s/src/cmd/internal/objabi/zbootstrap.go", goroot))

	// Use $GOROOT/pkg/bootstrap as the bootstrap workspace root.
	// We use a subdirectory of $GOROOT/pkg because that's the
	// space within $GOROOT where we store all generated objects.
	// We could use a temporary directory outside $GOROOT instead,
	// but it is easier to debug on failure if the files are in a known location.
	workspace := pathf("%s/pkg/bootstrap", goroot)
	xremoveall(workspace)
	xatexit(func() { xremoveall(workspace) })
	base := pathf("%s/src/bootstrap", workspace)
	xmkdirall(base)

	// Copy source code into $GOROOT/pkg/bootstrap and rewrite import paths.
	minBootstrapVers := requiredBootstrapVersion(goModVersion()) // require the minimum required go version to build this go version in the go.mod file
	writefile("module bootstrap\ngo "+minBootstrapVers+"\n", pathf("%s/%s", base, "go.mod"), 0)
	for _, dir := range bootstrapDirs {
		recurse := strings.HasSuffix(dir, "/...")
		dir = strings.TrimSuffix(dir, "/...")
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fatalf("walking bootstrap dirs failed: %v: %v", path, err)
			}

			name := filepath.Base(path)
			src := pathf("%s/src/%s", goroot, path)
			dst := pathf("%s/%s", base, path)

			if info.IsDir() {
				if !recurse && path != dir || name == "testdata" {
					return filepath.SkipDir
				}

				xmkdirall(dst)
				if path == "cmd/cgo" {
					// Write to src because we need the file both for bootstrap
					// and for later in the main build.
					mkzdefaultcc("", pathf("%s/zdefaultcc.go", src))
					mkzdefaultcc("", pathf("%s/zdefaultcc.go", dst))
				}
				return nil
			}

			for _, pre := range ignorePrefixes {
				if strings.HasPrefix(name, pre) {
					return nil
				}
			}
			for _, suf := range ignoreSuffixes {
				if strings.HasSuffix(name, suf) {
					return nil
				}
			}

			text := bootstrapRewriteFile(src)
			writefile(text, dst, 0)
			return nil
		})
	}

	// Set up environment for invoking Go bootstrap toolchains go command.
	// GOROOT points at Go bootstrap GOROOT,
	// GOPATH points at our bootstrap workspace,
	// GOBIN is empty, so that binaries are installed to GOPATH/bin,
	// and GOOS, GOHOSTOS, GOARCH, and GOHOSTOS are empty,
	// so that Go bootstrap toolchain builds whatever kind of binary it knows how to build.
	// Restore GOROOT, GOPATH, and GOBIN when done.
	// Don't bother with GOOS, GOHOSTOS, GOARCH, and GOHOSTARCH,
	// because setup will take care of those when bootstrapBuildTools returns.

	defer os.Setenv("GOROOT", os.Getenv("GOROOT"))
	os.Setenv("GOROOT", goroot_bootstrap)

	defer os.Setenv("GOPATH", os.Getenv("GOPATH"))
	os.Setenv("GOPATH", workspace)

	defer os.Setenv("GOBIN", os.Getenv("GOBIN"))
	os.Setenv("GOBIN", "")

	os.Setenv("GOOS", "")
	os.Setenv("GOHOSTOS", "")
	os.Setenv("GOARCH", "")
	os.Setenv("GOHOSTARCH", "")

	// Run Go bootstrap to build binaries.
	// Use the math_big_pure_go build tag to disable the assembly in math/big
	// which may contain unsupported instructions.
	// Use the purego build tag to disable other assembly code.
	cmd := []string{
		pathf("%s/bin/go", goroot_bootstrap),
		"install",
		"-tags=math_big_pure_go compiler_bootstrap purego",
	}
	if vflag > 0 {
		cmd = append(cmd, "-v")
	}
	if tool := os.Getenv("GOBOOTSTRAP_TOOLEXEC"); tool != "" {
		cmd = append(cmd, "-toolexec="+tool)
	}
	cmd = append(cmd, "bootstrap/cmd/...")
	run(base, ShowOutput|CheckExit, cmd...)

	// Copy binaries into tool binary directory.
	for _, name := range bootstrapDirs {
		if !strings.HasPrefix(name, "cmd/") {
			continue
		}
		name = name[len("cmd/"):]
		if !strings.Contains(name, "/") {
			copyfile(pathf("%s/%s%s", tooldir, name, exe), pathf("%s/bin/%s%s", workspace, name, exe), writeExec)
		}
	}

	if vflag > 0 {
		xprintf("\n")
	}
}

var ssaRewriteFileSubstring = filepath.FromSlash("src/cmd/compile/internal/ssa/rewrite")

// isUnneededSSARewriteFile reports whether srcFile is a
// src/cmd/compile/internal/ssa/rewriteARCHNAME.go file for an
// architecture that isn't for the given GOARCH.
//
// When unneeded is true archCaps is the rewrite base filename without
// the "rewrite" prefix or ".go" suffix: AMD64, 386, ARM, ARM64, etc.
func isUnneededSSARewriteFile(srcFile, goArch string) (archCaps string, unneeded bool) {
	if !strings.Contains(srcFile, ssaRewriteFileSubstring) {
		return "", false
	}
	fileArch := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(srcFile), "rewrite"), ".go")
	if fileArch == "" {
		return "", false
	}
	b := fileArch[0]
	if b == '_' || ('a' <= b && b <= 'z') {
		return "", false
	}
	archCaps = fileArch
	fileArch = strings.ToLower(fileArch)
	fileArch = strings.TrimSuffix(fileArch, "splitload")
	fileArch = strings.TrimSuffix(fileArch, "latelower")
	if fileArch == goArch {
		return "", false
	}
	if fileArch == strings.TrimSuffix(goArch, "le") {
		return "", false
	}
	return archCaps, true
}

func bootstrapRewriteFile(srcFile string) string {
	// During bootstrap, generate dummy rewrite files for
	// irrelevant architectures. We only need to build a bootstrap
	// binary that works for the current gohostarch.
	// This saves 6+ seconds of bootstrap.
	if archCaps, ok := isUnneededSSARewriteFile(srcFile, gohostarch); ok {
		return fmt.Sprintf(`%spackage ssa

func rewriteValue%s(v *Value) bool { panic("unused during bootstrap") }
func rewriteBlock%s(b *Block) bool { panic("unused during bootstrap") }
`, generatedHeader, archCaps, archCaps)
	}

	return bootstrapFixImports(srcFile)
}

var (
	importRE      = regexp.MustCompile(`\Aimport\s+(\.|[A-Za-z0-9_]+)?\s*"([^"]+)"\s*(//.*)?\n\z`)
	importBlockRE = regexp.MustCompile(`\A\s*(?:(\.|[A-Za-z0-9_]+)?\s*"([^"]+)")?\s*(//.*)?\n\z`)
)

func bootstrapFixImports(srcFile string) string {
	text := readfile(srcFile)
	lines := strings.SplitAfter(text, "\n")
	inBlock := false
	inComment := false
	for i, line := range lines {
		if strings.HasSuffix(line, "*/\n") {
			inComment = false
		}
		if strings.HasSuffix(line, "/*\n") {
			inComment = true
		}
		if inComment {
			continue
		}
		if strings.HasPrefix(line, "import (") {
			inBlock = true
			continue
		}
		if inBlock && strings.HasPrefix(line, ")") {
			inBlock = false
			continue
		}

		var m []string
		if !inBlock {
			if !strings.HasPrefix(line, "import ") {
				continue
			}
			m = importRE.FindStringSubmatch(line)
			if m == nil {
				fatalf("%s:%d: invalid import declaration: %q", srcFile, i+1, line)
			}
		} else {
			m = importBlockRE.FindStringSubmatch(line)
			if m == nil {
				fatalf("%s:%d: invalid import block line", srcFile, i+1)
			}
			if m[2] == "" {
				continue
			}
		}

		path := m[2]
		if strings.HasPrefix(path, "cmd/") {
			path = "bootstrap/" + path
		} else {
			for _, dir := range bootstrapDirs {
				if path == dir {
					path = "bootstrap/" + dir
					break
				}
			}
		}

		// Rewrite use of internal/reflectlite to be plain reflect.
		if path == "internal/reflectlite" {
			lines[i] = strings.ReplaceAll(line, `"reflect"`, `reflectlite "reflect"`)
			continue
		}

		// Otherwise, reject direct imports of internal packages,
		// since that implies knowledge of internal details that might
		// change from one bootstrap toolchain to the next.
		// There are many internal packages that are listed in
		// bootstrapDirs and made into bootstrap copies based on the
		// current repo's source code. Those are fine; this is catching
		// references to internal packages in the older bootstrap toolchain.
		if strings.HasPrefix(path, "internal/") {
			fatalf("%s:%d: bootstrap-copied source file cannot import %s", srcFile, i+1, path)
		}
		if path != m[2] {
			lines[i] = strings.ReplaceAll(line, `"`+m[2]+`"`, `"`+path+`"`)
		}
	}

	lines[0] = generatedHeader + "// This is a bootstrap copy of " + srcFile + "\n\n//line " + srcFile + ":1\n" + lines[0]

	return strings.Join(lines, "")
}

```

// === FILE: references!/go/src/cmd/dist/doc.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Dist helps bootstrap, build, and test the Go distribution.
//
// Usage:
//
//	go tool dist [command]
//
// The commands are:
//
//	banner         print installation banner
//	bootstrap      rebuild everything
//	clean          deletes all built files
//	env [-p]       print environment (-p: include $PATH)
//	install [dir]  install individual directory
//	list [-json]   list all supported platforms
//	test [-h]      run Go test(s)
//	version        print Go version
package main

```

// === FILE: references!/go/src/cmd/dist/exec.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os/exec"
	"strings"
)

// setDir sets cmd.Dir to dir, and also adds PWD=dir to cmd's environment.
func setDir(cmd *exec.Cmd, dir string) {
	cmd.Dir = dir
	if cmd.Env != nil {
		// os/exec won't set PWD automatically.
		setEnv(cmd, "PWD", dir)
	}
}

// setEnv sets cmd.Env so that key = value.
func setEnv(cmd *exec.Cmd, key, value string) {
	cmd.Env = append(cmd.Environ(), key+"="+value)
}

// unsetEnv sets cmd.Env so that key is not present in the environment.
func unsetEnv(cmd *exec.Cmd, key string) {
	cmd.Env = cmd.Environ()

	prefix := key + "="
	newEnv := []string{}
	for _, entry := range cmd.Env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		newEnv = append(newEnv, entry)
		// key may appear multiple times, so keep going.
	}
	cmd.Env = newEnv
}

```

// === FILE: references!/go/src/cmd/dist/imports.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is forked from go/build/read.go.
// (cmd/dist must not import go/build because we do not want it to be
// sensitive to the specific version of go/build present in $GOROOT_BOOTSTRAP.)

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

type importReader struct {
	b    *bufio.Reader
	buf  []byte
	peek byte
	err  error
	eof  bool
	nerr int
}

func isIdent(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '_' || c >= utf8.RuneSelf
}

var (
	errSyntax = errors.New("syntax error")
	errNUL    = errors.New("unexpected NUL in input")
)

// syntaxError records a syntax error, but only if an I/O error has not already been recorded.
func (r *importReader) syntaxError() {
	if r.err == nil {
		r.err = errSyntax
	}
}

// readByte reads the next byte from the input, saves it in buf, and returns it.
// If an error occurs, readByte records the error in r.err and returns 0.
func (r *importReader) readByte() byte {
	c, err := r.b.ReadByte()
	if err == nil {
		r.buf = append(r.buf, c)
		if c == 0 {
			err = errNUL
		}
	}
	if err != nil {
		if err == io.EOF {
			r.eof = true
		} else if r.err == nil {
			r.err = err
		}
		c = 0
	}
	return c
}

// peekByte returns the next byte from the input reader but does not advance beyond it.
// If skipSpace is set, peekByte skips leading spaces and comments.
func (r *importReader) peekByte(skipSpace bool) byte {
	if r.err != nil {
		if r.nerr++; r.nerr > 10000 {
			panic("go/build: import reader looping")
		}
		return 0
	}

	// Use r.peek as first input byte.
	// Don't just return r.peek here: it might have been left by peekByte(false)
	// and this might be peekByte(true).
	c := r.peek
	if c == 0 {
		c = r.readByte()
	}
	for r.err == nil && !r.eof {
		if skipSpace {
			// For the purposes of this reader, semicolons are never necessary to
			// understand the input and are treated as spaces.
			switch c {
			case ' ', '\f', '\t', '\r', '\n', ';':
				c = r.readByte()
				continue

			case '/':
				c = r.readByte()
				if c == '/' {
					for c != '\n' && r.err == nil && !r.eof {
						c = r.readByte()
					}
				} else if c == '*' {
					var c1 byte
					for (c != '*' || c1 != '/') && r.err == nil {
						if r.eof {
							r.syntaxError()
						}
						c, c1 = c1, r.readByte()
					}
				} else {
					r.syntaxError()
				}
				c = r.readByte()
				continue
			}
		}
		break
	}
	r.peek = c
	return r.peek
}

// nextByte is like peekByte but advances beyond the returned byte.
func (r *importReader) nextByte(skipSpace bool) byte {
	c := r.peekByte(skipSpace)
	r.peek = 0
	return c
}

// readKeyword reads the given keyword from the input.
// If the keyword is not present, readKeyword records a syntax error.
func (r *importReader) readKeyword(kw string) {
	r.peekByte(true)
	for i := 0; i < len(kw); i++ {
		if r.nextByte(false) != kw[i] {
			r.syntaxError()
			return
		}
	}
	if isIdent(r.peekByte(false)) {
		r.syntaxError()
	}
}

// readIdent reads an identifier from the input.
// If an identifier is not present, readIdent records a syntax error.
func (r *importReader) readIdent() {
	c := r.peekByte(true)
	if !isIdent(c) {
		r.syntaxError()
		return
	}
	for isIdent(r.peekByte(false)) {
		r.peek = 0
	}
}

// readString reads a quoted string literal from the input.
// If an identifier is not present, readString records a syntax error.
func (r *importReader) readString(save *[]string) {
	switch r.nextByte(true) {
	case '`':
		start := len(r.buf) - 1
		for r.err == nil {
			if r.nextByte(false) == '`' {
				if save != nil {
					*save = append(*save, string(r.buf[start:]))
				}
				break
			}
			if r.eof {
				r.syntaxError()
			}
		}
	case '"':
		start := len(r.buf) - 1
		for r.err == nil {
			c := r.nextByte(false)
			if c == '"' {
				if save != nil {
					*save = append(*save, string(r.buf[start:]))
				}
				break
			}
			if r.eof || c == '\n' {
				r.syntaxError()
			}
			if c == '\\' {
				r.nextByte(false)
			}
		}
	default:
		r.syntaxError()
	}
}

// readImport reads an import clause - optional identifier followed by quoted string -
// from the input.
func (r *importReader) readImport(imports *[]string) {
	c := r.peekByte(true)
	if c == '.' {
		r.peek = 0
	} else if isIdent(c) {
		r.readIdent()
	}
	r.readString(imports)
}

// readimports returns the imports found in the named file.
func readimports(file string) []string {
	var imports []string
	r := &importReader{b: bufio.NewReader(strings.NewReader(readfile(file)))}
	r.readKeyword("package")
	r.readIdent()
	for r.peekByte(true) == 'i' {
		r.readKeyword("import")
		if r.peekByte(true) == '(' {
			r.nextByte(false)
			for r.peekByte(true) != ')' && r.err == nil {
				r.readImport(&imports)
			}
			r.nextByte(false)
		} else {
			r.readImport(&imports)
		}
	}

	for i := range imports {
		unquoted, err := strconv.Unquote(imports[i])
		if err != nil {
			fatalf("reading imports from %s: %v", file, err)
		}
		imports[i] = unquoted
	}

	return imports
}

// resolveVendor returns a unique package path imported with the given import
// path from srcDir.
//
// resolveVendor assumes that a package is vendored if and only if its first
// path component contains a dot. If a package is vendored, its import path
// is returned with a "vendor" or "cmd/vendor" prefix, depending on srcDir.
// Otherwise, the import path is returned verbatim.
func resolveVendor(imp, srcDir string) string {
	var first string
	if i := strings.Index(imp, "/"); i < 0 {
		first = imp
	} else {
		first = imp[:i]
	}
	isStandard := !strings.Contains(first, ".")
	if isStandard {
		return imp
	}

	if strings.HasPrefix(srcDir, filepath.Join(goroot, "src", "cmd")) {
		return path.Join("cmd", "vendor", imp)
	} else if strings.HasPrefix(srcDir, filepath.Join(goroot, "src")) {
		return path.Join("vendor", imp)
	} else {
		panic(fmt.Sprintf("srcDir %q not in GOROOT/src", srcDir))
	}
}

```

// === FILE: references!/go/src/cmd/dist/main.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
)

func usage() {
	xprintf(`usage: go tool dist [command]
Commands are:

banner                  print installation banner
bootstrap               rebuild everything
clean                   deletes all built files
env [-p]                print environment (-p: include $PATH)
install [dir]           install individual directory
list [-json] [-broken]  list all supported platforms
test [-h]               run Go test(s)
version                 print Go version

All commands take -v flags to emit extra information.
`)
	xexit(2)
}

// commands records the available commands.
var commands = map[string]func(){
	"banner":    cmdbanner,
	"bootstrap": cmdbootstrap,
	"clean":     cmdclean,
	"env":       cmdenv,
	"install":   cmdinstall,
	"list":      cmdlist,
	"test":      cmdtest,
	"version":   cmdversion,
}

// main takes care of OS-specific startup and dispatches to xmain.
func main() {
	os.Setenv("TERM", "dumb") // disable escape codes in clang errors

	// provide -check-armv6k first, before checking for $GOROOT so that
	// it is possible to run this check without having $GOROOT available.
	if len(os.Args) > 1 && os.Args[1] == "-check-armv6k" {
		useARMv6K() // might fail with SIGILL
		println("ARMv6K supported.")
		os.Exit(0)
	}

	gohostos = runtime.GOOS
	switch gohostos {
	case "aix":
		// uname -m doesn't work under AIX
		gohostarch = "ppc64"
	case "plan9":
		gohostarch = os.Getenv("objtype")
		if gohostarch == "" {
			fatalf("$objtype is unset")
		}
	case "solaris", "illumos":
		// Solaris and illumos systems have multi-arch userlands, and
		// "uname -m" reports the machine hardware name; e.g.,
		// "i86pc" on both 32- and 64-bit x86 systems.  Check for the
		// native (widest) instruction set on the running kernel:
		out := run("", CheckExit, "isainfo", "-n")
		if strings.Contains(out, "amd64") {
			gohostarch = "amd64"
		}
		if strings.Contains(out, "i386") {
			gohostarch = "386"
		}
	case "windows":
		exe = ".exe"
	}

	sysinit()

	if gohostarch == "" {
		// Default Unix system.
		out := run("", CheckExit, "uname", "-m")
		outAll := run("", CheckExit, "uname", "-a")
		switch {
		case strings.Contains(outAll, "RELEASE_ARM64"):
			// MacOS prints
			// Darwin p1.local 21.1.0 Darwin Kernel Version 21.1.0: Wed Oct 13 17:33:01 PDT 2021; root:xnu-8019.41.5~1/RELEASE_ARM64_T6000 x86_64
			// on ARM64 laptops when there is an x86 parent in the
			// process tree. Look for the RELEASE_ARM64 to avoid being
			// confused into building an x86 toolchain.
			gohostarch = "arm64"
		case strings.Contains(out, "x86_64"), strings.Contains(out, "amd64"):
			gohostarch = "amd64"
		case strings.Contains(out, "86"):
			gohostarch = "386"
			if gohostos == "darwin" {
				// Even on 64-bit platform, some versions of macOS uname -m prints i386.
				// We don't support any of the OS X versions that run on 32-bit-only hardware anymore.
				gohostarch = "amd64"
			}
		case strings.Contains(out, "aarch64"), strings.Contains(out, "arm64"):
			gohostarch = "arm64"
		case strings.Contains(out, "arm"):
			gohostarch = "arm"
			if gohostos == "netbsd" && strings.Contains(run("", CheckExit, "uname", "-p"), "aarch64") {
				gohostarch = "arm64"
			}
		case strings.Contains(out, "ppc64le"):
			gohostarch = "ppc64le"
		case strings.Contains(out, "ppc64"):
			gohostarch = "ppc64"
		case strings.Contains(out, "mips64"):
			gohostarch = "mips64"
			if elfIsLittleEndian(os.Args[0]) {
				gohostarch = "mips64le"
			}
		case strings.Contains(out, "mips"):
			gohostarch = "mips"
			if elfIsLittleEndian(os.Args[0]) {
				gohostarch = "mipsle"
			}
		case strings.Contains(out, "loongarch64"):
			gohostarch = "loong64"
		case strings.Contains(out, "riscv64"):
			gohostarch = "riscv64"
		case strings.Contains(out, "s390x"):
			gohostarch = "s390x"
		case gohostos == "darwin", gohostos == "ios":
			if strings.Contains(run("", CheckExit, "uname", "-v"), "RELEASE_ARM64_") {
				gohostarch = "arm64"
			}
		case gohostos == "freebsd":
			if strings.Contains(run("", CheckExit, "uname", "-p"), "riscv64") {
				gohostarch = "riscv64"
			}
		case gohostos == "openbsd" && strings.Contains(out, "powerpc64"):
			gohostarch = "ppc64"
		default:
			fatalf("unknown architecture: %s", out)
		}
	}

	if gohostarch == "arm" || gohostarch == "mips64" || gohostarch == "mips64le" {
		maxbg = min(maxbg, runtime.NumCPU())
	}
	// For deterministic make.bash debugging and for smallest-possible footprint,
	// pay attention to GOMAXPROCS=1.  This was a bad idea for 1.4 bootstrap, but
	// the bootstrap version is now 1.17+ and thus this is fine.
	if runtime.GOMAXPROCS(0) == 1 {
		maxbg = 1
	}
	bginit()

	if len(os.Args) > 1 && os.Args[1] == "-check-goarm" {
		useVFPv1() // might fail with SIGILL
		println("VFPv1 OK.")
		useVFPv3() // might fail with SIGILL
		println("VFPv3 OK.")
		os.Exit(0)
	}

	xinit()
	xmain()
	xexit(0)
}

// The OS-specific main calls into the portable code here.
func xmain() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	os.Args = os.Args[1:] // for flag parsing during cmd
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: go tool dist %s [options]\n", cmd)
		flag.PrintDefaults()
		os.Exit(2)
	}
	if f, ok := commands[cmd]; ok {
		f()
	} else {
		xprintf("unknown command %s\n", cmd)
		usage()
	}
}

```

// === FILE: references!/go/src/cmd/dist/notgo124.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Go 1.26 and later requires Go 1.24.6 as the minimum bootstrap toolchain.
// If cmd/dist is built using an earlier Go version, this file will be
// included in the build and cause an error like:
//
// % GOROOT_BOOTSTRAP=$HOME/sdk/go1.16 ./make.bash
// Building Go cmd/dist using /Users/rsc/sdk/go1.16. (go1.16 darwin/amd64)
// found packages main (build.go) and building_Go_requires_Go_1_24_6_or_later (notgo124.go) in /Users/rsc/go/src/cmd/dist
// %
//
// which is the best we can do under the circumstances.
//
// See go.dev/issue/44505 and go.dev/issue/54265 for more
// background on why Go moved on from Go 1.4 for bootstrap.

//go:build !go1.24

package building_Go_requires_Go_1_24_6_or_later

```

// === FILE: references!/go/src/cmd/dist/quoted.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "fmt"

// quotedSplit is a verbatim copy from cmd/internal/quoted.go:Split and its
// dependencies (isSpaceByte). Since this package is built using the host's
// Go compiler, it cannot use `cmd/internal/...`. We also don't want to export
// it to all Go users.
//
// Please keep those in sync.
func quotedSplit(s string) ([]string, error) {
	// Split fields allowing '' or "" around elements.
	// Quotes further inside the string do not count.
	var f []string
	for len(s) > 0 {
		for len(s) > 0 && isSpaceByte(s[0]) {
			s = s[1:]
		}
		if len(s) == 0 {
			break
		}
		// Accepted quoted string. No unescaping inside.
		if s[0] == '"' || s[0] == '\'' {
			quote := s[0]
			s = s[1:]
			i := 0
			for i < len(s) && s[i] != quote {
				i++
			}
			if i >= len(s) {
				return nil, fmt.Errorf("unterminated %c string", quote)
			}
			f = append(f, s[:i])
			s = s[i+1:]
			continue
		}
		i := 0
		for i < len(s) && !isSpaceByte(s[i]) {
			i++
		}
		f = append(f, s[:i])
		s = s[i:]
	}
	return f, nil
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

```

// === FILE: references!/go/src/cmd/dist/README ===
```text
This program, dist, is the bootstrapping tool for the Go distribution.

As of Go 1.5, dist and other parts of the compiler toolchain are written
in Go, making bootstrapping a little more involved than in the past.
The approach is to build the current release of Go with an earlier one.

The process to install Go 1.x, for x ≥ 26, is:

1. Build cmd/dist with Go 1.24.6.
2. Using dist, build Go 1.x compiler toolchain with Go 1.24.6.
3. Using dist, rebuild Go 1.x compiler toolchain with itself.
4. Using dist, build Go 1.x cmd/go (as go_bootstrap) with Go 1.x compiler toolchain.
5. Using go_bootstrap, build the remaining Go 1.x standard library and commands.

Because of backward compatibility, although the steps above say Go 1.24.6,
in practice any release ≥ Go 1.24.6 but < Go 1.x will work as the bootstrap base.
Releases ≥ Go 1.x are very likely to work as well.

See go.dev/s/go15bootstrap for more details about the original bootstrap
and go.dev/issue/54265 for details about later bootstrap version bumps.

```

// === FILE: references!/go/src/cmd/dist/sys_default.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows

package main

func sysinit() {
}

```

// === FILE: references!/go/src/cmd/dist/sys_windows.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"syscall"
	"unsafe"
)

var (
	modkernel32       = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemInfo = modkernel32.NewProc("GetSystemInfo")
)

// see https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/ns-sysinfoapi-system_info
type systeminfo struct {
	wProcessorArchitecture      uint16
	wReserved                   uint16
	dwPageSize                  uint32
	lpMinimumApplicationAddress uintptr
	lpMaximumApplicationAddress uintptr
	dwActiveProcessorMask       uintptr
	dwNumberOfProcessors        uint32
	dwProcessorType             uint32
	dwAllocationGranularity     uint32
	wProcessorLevel             uint16
	wProcessorRevision          uint16
}

// See https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/ns-sysinfoapi-system_info
const (
	PROCESSOR_ARCHITECTURE_AMD64 = 9
	PROCESSOR_ARCHITECTURE_INTEL = 0
	PROCESSOR_ARCHITECTURE_ARM64 = 12
	PROCESSOR_ARCHITECTURE_IA64  = 6
)

var sysinfo systeminfo

func sysinit() {
	syscall.Syscall(procGetSystemInfo.Addr(), 1, uintptr(unsafe.Pointer(&sysinfo)), 0, 0)
	switch sysinfo.wProcessorArchitecture {
	case PROCESSOR_ARCHITECTURE_AMD64:
		gohostarch = "amd64"
	case PROCESSOR_ARCHITECTURE_INTEL:
		gohostarch = "386"
	case PROCESSOR_ARCHITECTURE_ARM64:
		gohostarch = "arm64"
	default:
		fatalf("unknown processor architecture")
	}
}

```

// === FILE: references!/go/src/cmd/dist/test.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

func cmdtest() {
	gogcflags = os.Getenv("GO_GCFLAGS")
	setNoOpt()

	var t tester

	t.asmflags = os.Getenv("GO_TEST_ASMFLAGS")

	var noRebuild bool
	flag.BoolVar(&t.listMode, "list", false, "list available tests")
	flag.BoolVar(&t.rebuild, "rebuild", false, "rebuild everything first")
	flag.BoolVar(&noRebuild, "no-rebuild", false, "overrides -rebuild (historical dreg)")
	flag.BoolVar(&t.keepGoing, "k", false, "keep going even when error occurred")
	flag.BoolVar(&t.race, "race", false, "run in race builder mode (different set of tests)")
	flag.BoolVar(&t.compileOnly, "compile-only", false, "compile tests, but don't run them")
	flag.StringVar(&t.banner, "banner", "##### ", "banner prefix; blank means no section banners")
	flag.StringVar(&t.runRxStr, "run", "",
		"run only those tests matching the regular expression; empty means to run all. "+
			"Special exception: if the string begins with '!', the match is inverted.")
	flag.BoolVar(&t.msan, "msan", false, "run in memory sanitizer builder mode")
	flag.BoolVar(&t.asan, "asan", false, "run in address sanitizer builder mode")
	flag.BoolVar(&t.json, "json", false, "report test results in JSON")

	xflagparse(-1) // any number of args
	if noRebuild {
		t.rebuild = false
	}

	t.run()
}

// tester executes cmdtest.
type tester struct {
	race        bool
	msan        bool
	asan        bool
	listMode    bool
	rebuild     bool
	failed      bool
	keepGoing   bool
	compileOnly bool // just try to compile all tests, but no need to run
	short       bool
	cgoEnabled  bool
	asmflags    string
	json        bool
	runRxStr    string
	runRx       *regexp.Regexp
	runRxWant   bool     // want runRx to match (true) or not match (false)
	runNames    []string // tests to run, exclusive with runRx; empty means all
	banner      string   // prefix, or "" for none
	lastHeading string   // last dir heading printed

	tests        []distTest // use addTest to extend
	testNames    map[string]bool
	timeoutScale int // a non-negative integer factor to scale test timeout by; defaults to 1

	worklist []*work
}

// work tracks command execution for a test.
type work struct {
	dt    *distTest     // unique test name, etc.
	cmd   *exec.Cmd     // must write stdout/stderr to out
	flush func()        // if non-nil, called after cmd.Run
	start chan bool     // a true means to start, a false means to skip
	out   bytes.Buffer  // combined stdout/stderr from cmd
	err   error         // work result
	end   chan struct{} // a value means cmd ended (or was skipped)
}

// printSkip prints a skip message for all of work.
func (w *work) printSkip(t *tester, msg string) {
	if t.json {
		synthesizeSkipEvent(json.NewEncoder(&w.out), w.dt.name, msg)
		return
	}
	fmt.Fprintln(&w.out, msg)
}

// A distTest is a test run by dist test.
// Each test has a unique name and belongs to a group (heading)
type distTest struct {
	name    string // unique test name; may be filtered with -run flag
	heading string // group section; this header is printed before the test is run.
	fn      func(*distTest) error
}

func (t *tester) run() {
	timelog("start", "dist test")

	os.Setenv("PATH", fmt.Sprintf("%s%c%s", gorootBin, os.PathListSeparator, os.Getenv("PATH")))

	t.short = true
	if v := os.Getenv("GO_TEST_SHORT"); v != "" {
		short, err := strconv.ParseBool(v)
		if err != nil {
			fatalf("invalid GO_TEST_SHORT %q: %v", v, err)
		}
		t.short = short
	}

	cmd := exec.Command(gorootBinGo, "env", "CGO_ENABLED")
	cmd.Stderr = new(bytes.Buffer)
	slurp, err := cmd.Output()
	if err != nil {
		fatalf("Error running %s: %v\n%s", cmd, err, cmd.Stderr)
	}
	parts := strings.Split(string(slurp), "\n")
	if nlines := len(parts) - 1; nlines < 1 {
		fatalf("Error running %s: output contains <1 lines\n%s", cmd, cmd.Stderr)
	}
	t.cgoEnabled, _ = strconv.ParseBool(parts[0])

	if flag.NArg() > 0 && t.runRxStr != "" {
		fatalf("the -run regular expression flag is mutually exclusive with test name arguments")
	}

	t.runNames = flag.Args()

	// Set GOTRACEBACK to system if the user didn't set a level explicitly.
	// Since we're running tests for Go, we want as much detail as possible
	// if something goes wrong.
	//
	// Set it before running any commands just in case something goes wrong.
	if ok := isEnvSet("GOTRACEBACK"); !ok {
		if err := os.Setenv("GOTRACEBACK", "system"); err != nil {
			if t.keepGoing {
				log.Printf("Failed to set GOTRACEBACK: %v", err)
			} else {
				fatalf("Failed to set GOTRACEBACK: %v", err)
			}
		}
	}

	if t.rebuild {
		t.out("Building packages and commands.")
		// Force rebuild the whole toolchain.
		goInstall(toolenv(), gorootBinGo, append([]string{"-a"}, toolchain...)...)
	}

	if !t.listMode {
		if builder := os.Getenv("GO_BUILDER_NAME"); builder == "" {
			// Ensure that installed commands are up to date, even with -no-rebuild,
			// so that tests that run commands end up testing what's actually on disk.
			// If everything is up-to-date, this is a no-op.
			// We first build the toolchain twice to allow it to converge,
			// as when we first bootstrap.
			// See cmdbootstrap for a description of the overall process.
			//
			// On the builders, we skip this step: we assume that 'dist test' is
			// already using the result of a clean build, and because of test sharding
			// and virtualization we usually start with a clean GOCACHE, so we would
			// end up rebuilding large parts of the standard library that aren't
			// otherwise relevant to the actual set of packages under test.
			goInstall(toolenv(), gorootBinGo, toolchain...)
			goInstall(toolenv(), gorootBinGo, toolchain...)
			goInstall(toolenv(), gorootBinGo, toolsToInstall...)
		}
	}

	t.timeoutScale = 1
	if s := os.Getenv("GO_TEST_TIMEOUT_SCALE"); s != "" {
		t.timeoutScale, err = strconv.Atoi(s)
		if err != nil {
			fatalf("failed to parse $GO_TEST_TIMEOUT_SCALE = %q as integer: %v", s, err)
		}
	}

	if t.runRxStr != "" {
		if t.runRxStr[0] == '!' {
			t.runRxWant = false
			t.runRxStr = t.runRxStr[1:]
		} else {
			t.runRxWant = true
		}
		t.runRx = regexp.MustCompile(t.runRxStr)
	}

	t.registerTests()
	if t.listMode {
		for _, tt := range t.tests {
			fmt.Println(tt.name)
		}
		return
	}

	for _, name := range t.runNames {
		if !t.testNames[name] {
			fatalf("unknown test %q", name)
		}
	}

	// On a few builders, make GOROOT unwritable to catch tests writing to it.
	if strings.HasPrefix(os.Getenv("GO_BUILDER_NAME"), "linux-") {
		if os.Getuid() == 0 {
			// Don't bother making GOROOT unwritable:
			// we're running as root, so permissions would have no effect.
		} else {
			xatexit(t.makeGOROOTUnwritable())
		}
	}

	if !t.json {
		if err := t.maybeLogMetadata(); err != nil {
			t.failed = true
			if t.keepGoing {
				log.Printf("Failed logging metadata: %v", err)
			} else {
				fatalf("Failed logging metadata: %v", err)
			}
		}
	}

	var anyIncluded, someExcluded bool
	for _, dt := range t.tests {
		if !t.shouldRunTest(dt.name) {
			someExcluded = true
			continue
		}
		anyIncluded = true
		dt := dt // dt used in background after this iteration
		if err := dt.fn(&dt); err != nil {
			t.runPending(&dt) // in case that hasn't been done yet
			t.failed = true
			if t.keepGoing {
				log.Printf("Failed: %v", err)
			} else {
				fatalf("Failed: %v", err)
			}
		}
	}
	t.runPending(nil)
	timelog("end", "dist test")

	if !t.json {
		if t.failed {
			fmt.Println("\nFAILED")
		} else if !anyIncluded {
			fmt.Println()
			errprintf("go tool dist: warning: %q matched no tests; use the -list flag to list available tests\n", t.runRxStr)
			fmt.Println("NO TESTS TO RUN")
		} else if someExcluded {
			fmt.Println("\nALL TESTS PASSED (some were excluded)")
		} else {
			fmt.Println("\nALL TESTS PASSED")
		}
	}
	if t.failed {
		xexit(1)
	}
}

func (t *tester) shouldRunTest(name string) bool {
	if t.runRx != nil {
		return t.runRx.MatchString(name) == t.runRxWant
	}
	if len(t.runNames) == 0 {
		return true
	}
	return slices.Contains(t.runNames, name)
}

func (t *tester) maybeLogMetadata() error {
	if t.compileOnly {
		// We need to run a subprocess to log metadata. Don't do that
		// on compile-only runs.
		return nil
	}
	t.out("Test execution environment.")
	// Helper binary to print system metadata (CPU model, etc). This is a
	// separate binary from dist so it need not build with the bootstrap
	// toolchain.
	//
	// TODO(prattmic): If we split dist bootstrap and dist test then this
	// could be simplified to directly use internal/sysinfo here.
	return t.dirCmd(filepath.Join(goroot, "src/cmd/internal/metadata"), gorootBinGo, []string{"run", "main.go"}).Run()
}

// testName returns the dist test name for a given package and variant.
func testName(pkg, variant string) string {
	name := pkg
	if variant != "" {
		name += ":" + variant
	}
	return name
}

// goTest represents all options to a "go test" command. The final command will
// combine configuration from goTest and tester flags.
type goTest struct {
	timeout  time.Duration // If non-zero, override timeout
	short    bool          // If true, force -short
	tags     []string      // Build tags
	race     bool          // Force -race
	bench    bool          // Run benchmarks (briefly), not tests.
	runTests string        // Regexp of tests to run
	cpu      string        // If non-empty, -cpu flag
	skip     string        // If non-empty, -skip flag

	gcflags   string // If non-empty, build with -gcflags=all=X
	ldflags   string // If non-empty, build with -ldflags=X
	buildmode string // If non-empty, -buildmode flag

	env []string // Environment variables to add, as KEY=VAL. KEY= unsets a variable

	runOnHost bool // When cross-compiling, run this test on the host instead of guest

	// variant, if non-empty, is a name used to distinguish different
	// configurations of the same test package(s). If set and omitVariant is false,
	// the Package field in test2json output is rewritten to pkg:variant.
	variant string
	// omitVariant indicates that variant is used solely for the dist test name and
	// that the set of test names run by each variant (including empty) of a package
	// is non-overlapping.
	//
	// TODO(mknyszek): Consider removing omitVariant as it is no longer set to true
	// by any test. It's too valuable to have timing information in ResultDB that
	// corresponds directly with dist names for tests.
	omitVariant bool

	// We have both pkg and pkgs as a convenience. Both may be set, in which
	// case they will be combined. At least one must be set.
	pkgs []string // Multiple packages to test
	pkg  string   // A single package to test

	testFlags []string // Additional flags accepted by this test
}

// compileOnly reports whether this test is only for compiling,
// indicated by runTests being set to '^$' and bench being false.
func (opts *goTest) compileOnly() bool {
	return opts.runTests == "^$" && !opts.bench
}

// bgCommand returns a go test Cmd and a post-Run flush function. The result
// will write its output to stdout and stderr. If stdout==stderr, bgCommand
// ensures Writes are serialized. The caller should call flush() after Cmd exits.
func (opts *goTest) bgCommand(t *tester, stdout, stderr io.Writer) (cmd *exec.Cmd, flush func()) {
	build, run, pkgs, testFlags, setupCmd := opts.buildArgs(t)

	// Combine the flags.
	args := append([]string{"test"}, build...)
	if t.compileOnly || opts.compileOnly() {
		args = append(args, "-c", "-o", os.DevNull)
	} else {
		args = append(args, run...)
	}
	args = append(args, pkgs...)
	if !t.compileOnly && !opts.compileOnly() {
		args = append(args, testFlags...)
	}

	cmd = exec.Command(gorootBinGo, args...)
	setupCmd(cmd)
	if t.json && opts.variant != "" && !opts.omitVariant {
		// Rewrite Package in the JSON output to be pkg:variant. When omitVariant
		// is true, pkg.TestName is already unambiguous, so we don't need to
		// rewrite the Package field.
		//
		// We only want to process JSON on the child's stdout. Ideally if
		// stdout==stderr, we would also use the same testJSONFilter for
		// cmd.Stdout and cmd.Stderr in order to keep the underlying
		// interleaving of writes, but then it would see even partial writes
		// interleaved, which would corrupt the JSON. So, we only process
		// cmd.Stdout. This has another consequence though: if stdout==stderr,
		// we have to serialize Writes in case the Writer is not concurrent
		// safe. If we were just passing stdout/stderr through to exec, it would
		// do this for us, but since we're wrapping stdout, we have to do it
		// ourselves.
		if stdout == stderr {
			stdout = &lockedWriter{w: stdout}
			stderr = stdout
		}
		f := &testJSONFilter{w: stdout, variant: opts.variant}
		cmd.Stdout = f
		flush = f.Flush
	} else {
		cmd.Stdout = stdout
		flush = func() {}
	}
	cmd.Stderr = stderr

	return cmd, flush
}

// run runs a go test and returns an error if it does not succeed.
func (opts *goTest) run(t *tester) error {
	cmd, flush := opts.bgCommand(t, os.Stdout, os.Stderr)
	err := cmd.Run()
	flush()
	return err
}

// buildArgs is in internal helper for goTest that constructs the elements of
// the "go test" command line. build is the flags for building the test. run is
// the flags for running the test. pkgs is the list of packages to build and
// run. testFlags is the list of flags to pass to the test package.
//
// The caller must call setupCmd on the resulting exec.Cmd to set its directory
// and environment.
func (opts *goTest) buildArgs(t *tester) (build, run, pkgs, testFlags []string, setupCmd func(*exec.Cmd)) {
	run = append(run, "-count=1") // Disallow caching
	if opts.timeout != 0 {
		d := opts.timeout * time.Duration(t.timeoutScale)
		run = append(run, "-timeout="+d.String())
	} else if t.timeoutScale != 1 {
		const goTestDefaultTimeout = 10 * time.Minute // Default value of go test -timeout flag.
		run = append(run, "-timeout="+(goTestDefaultTimeout*time.Duration(t.timeoutScale)).String())
	}
	if opts.short || t.short {
		run = append(run, "-short")
	}
	var tags []string
	if noOpt {
		tags = append(tags, "noopt")
	}
	tags = append(tags, opts.tags...)
	if len(tags) > 0 {
		build = append(build, "-tags="+strings.Join(tags, ","))
	}
	if t.race || opts.race {
		build = append(build, "-race")
	}
	if t.msan {
		build = append(build, "-msan")
	}
	if t.asan {
		build = append(build, "-asan")
	}
	if opts.bench {
		// Run no tests.
		run = append(run, "-run=^$")
		// Run benchmarks briefly as a smoke test.
		run = append(run, "-bench=.*", "-benchtime=.1s")
	} else if opts.runTests != "" {
		run = append(run, "-run="+opts.runTests)
	}
	if opts.cpu != "" {
		run = append(run, "-cpu="+opts.cpu)
	}
	if opts.skip != "" {
		run = append(run, "-skip="+opts.skip)
	}
	if t.json {
		run = append(run, "-json")
	}

	if opts.gcflags != "" {
		build = append(build, "-gcflags=all="+opts.gcflags)
	}
	if opts.ldflags != "" {
		build = append(build, "-ldflags="+opts.ldflags)
	}
	if t.asmflags != "" {
		build = append(build, "-asmflags="+t.asmflags)
	}
	if opts.buildmode != "" {
		build = append(build, "-buildmode="+opts.buildmode)
	}

	pkgs = opts.packages()

	runOnHost := opts.runOnHost && (goarch != gohostarch || goos != gohostos)
	needTestFlags := len(opts.testFlags) > 0 || runOnHost
	if needTestFlags {
		testFlags = append([]string{"-args"}, opts.testFlags...)
	}
	if runOnHost {
		// -target is a special flag understood by tests that can run on the host
		testFlags = append(testFlags, "-target="+goos+"/"+goarch)
	}

	setupCmd = func(cmd *exec.Cmd) {
		setDir(cmd, filepath.Join(goroot, "src"))
		if len(opts.env) != 0 {
			for _, kv := range opts.env {
				if i := strings.Index(kv, "="); i < 0 {
					unsetEnv(cmd, kv[:len(kv)-1])
				} else {
					setEnv(cmd, kv[:i], kv[i+1:])
				}
			}
		}
		if runOnHost {
			setEnv(cmd, "GOARCH", gohostarch)
			setEnv(cmd, "GOOS", gohostos)
		}
	}

	return
}

// packages returns the full list of packages to be run by this goTest. This
// will always include at least one package.
func (opts *goTest) packages() []string {
	pkgs := opts.pkgs
	if opts.pkg != "" {
		pkgs = append(pkgs[:len(pkgs):len(pkgs)], opts.pkg)
	}
	if len(pkgs) == 0 {
		panic("no packages")
	}
	return pkgs
}

// printSkip prints a skip message for all of goTest.
func (opts *goTest) printSkip(t *tester, msg string) {
	if t.json {
		enc := json.NewEncoder(os.Stdout)
		for _, pkg := range opts.packages() {
			synthesizeSkipEvent(enc, pkg, msg)
		}
		return
	}
	fmt.Println(msg)
}

// ranGoTest and stdMatches are state closed over by the stdlib
// testing func in registerStdTest below. The tests are run
// sequentially, so there's no need for locks.
//
// ranGoBench and benchMatches are the same, but are only used
// in -race mode.
var (
	ranGoTest  bool
	stdMatches []string

	ranGoBench   bool
	benchMatches []string
)

func (t *tester) registerStdTest(pkg string) {
	const stdTestHeading = "Testing packages." // known to addTest for a safety check
	gcflags := gogcflags
	name := testName(pkg, "")
	if t.runRx == nil || t.runRx.MatchString(name) == t.runRxWant {
		stdMatches = append(stdMatches, pkg)
	}
	t.addTest(name, stdTestHeading, func(dt *distTest) error {
		if ranGoTest {
			return nil
		}
		t.runPending(dt)
		timelog("start", dt.name)
		defer timelog("end", dt.name)
		ranGoTest = true

		timeoutSec := 180 * time.Second
		for _, pkg := range stdMatches {
			switch pkg {
			case "cmd/go":
				timeoutSec *= 3
			case "cmd/cgo/internal/testshared":
				// This package can take 2-3 minutes to test, so 3 min timeout causes
				// flaky failures, like https://ci.chromium.org/b/8679277370961616529.
				// Use the default timeout for it rather than the custom 3 minute one.
				timeoutSec = 0
			case "internal/godebugs":
				// This package can take 5-6 minutes to test when the asan mode is on.
				// The asan modifier scales the timeout by 2, but even 3*2 minutes is
				// sometimes not enough. See go.dev/issue/78392.
				// Use the default timeout for it rather than the custom 3 minute one.
				timeoutSec = 0
			}
		}
		return (&goTest{
			timeout: timeoutSec,
			gcflags: gcflags,
			pkgs:    stdMatches,
		}).run(t)
	})
}

func (t *tester) registerRaceBenchTest(pkg string) {
	const raceBenchHeading = "Running benchmarks briefly." // known to addTest for a safety check
	name := testName(pkg, "racebench")
	if t.runRx == nil || t.runRx.MatchString(name) == t.runRxWant {
		benchMatches = append(benchMatches, pkg)
	}
	t.addTest(name, raceBenchHeading, func(dt *distTest) error {
		if ranGoBench {
			return nil
		}
		t.runPending(dt)
		timelog("start", dt.name)
		defer timelog("end", dt.name)
		ranGoBench = true
		return (&goTest{
			variant: "racebench",
			// Include the variant even though there's no overlap in test names.
			// This makes the test targets distinct, allowing our build system to record
			// elapsed time for each one, which is useful for load-balancing test shards.
			omitVariant: false,
			timeout:     1200 * time.Second, // longer timeout for race with benchmarks
			race:        true,
			bench:       true,
			cpu:         "4",
			pkgs:        benchMatches,
		}).run(t)
	})
}

func (t *tester) registerTests() {
	// registerStdTestSpecially tracks import paths in the standard library
	// whose test registration happens in a special way.
	//
	// These tests *must* be able to run normally as part of "go test std cmd",
	// even if they are also registered separately by dist, because users often
	// run go test directly. Use skips or build tags in preference to expanding
	// this list.
	registerStdTestSpecially := map[string]bool{
		// testdir can run normally as part of "go test std cmd", but because
		// it's a very large test, we register is specially as several shards to
		// enable better load balancing on sharded builders. Ideally the build
		// system would know how to shard any large test package.
		"cmd/internal/testdir": true,
	}

	// Fast path to avoid the ~1 second of `go list std cmd` when
	// the caller lists specific tests to run. (as the continuous
	// build coordinator does).
	if len(t.runNames) > 0 {
		for _, name := range t.runNames {
			if !strings.Contains(name, ":") {
				t.registerStdTest(name)
			} else if strings.HasSuffix(name, ":racebench") {
				t.registerRaceBenchTest(strings.TrimSuffix(name, ":racebench"))
			}
		}
	} else {
		// Use 'go list std cmd' to get a list of all Go packages
		// that running 'go test std cmd' could find problems in.
		// (In race test mode, also set -tags=race.)
		// This includes vendored packages and other
		// packages without tests so that 'dist test' finds if any of
		// them don't build, have a problem reported by high-confidence
		// vet checks that come with 'go test', and anything else it
		// may check in the future. See go.dev/issue/60463.
		// Most packages have tests, so there is not much saved
		// by skipping non-test packages.
		// For the packages without any test files,
		// 'go test' knows not to actually build a test binary,
		// so the only cost is the vet, and we still want to run vet.
		cmd := exec.Command(gorootBinGo, "list")
		if t.race {
			cmd.Args = append(cmd.Args, "-tags=race")
		}
		cmd.Args = append(cmd.Args, "std", "cmd")
		cmd.Stderr = new(bytes.Buffer)
		all, err := cmd.Output()
		if err != nil {
			fatalf("Error running go list std cmd: %v:\n%s", err, cmd.Stderr)
		}
		pkgs := strings.Fields(string(all))
		for _, pkg := range pkgs {
			if registerStdTestSpecially[pkg] {
				continue
			}
			if t.short && (strings.HasPrefix(pkg, "vendor/") || strings.HasPrefix(pkg, "cmd/vendor/")) {
				// Vendored code has no tests, and we don't care too much about vet errors
				// since we can't modify the code, so skip the tests in short mode.
				// We still let the longtest builders vet them.
				continue
			}
			t.registerStdTest(pkg)
		}
		if t.race && !t.short {
			for _, pkg := range pkgs {
				if t.packageHasBenchmarks(pkg) {
					t.registerRaceBenchTest(pkg)
				}
			}
		}
	}

	if t.race {
		return
	}

	// Test the os/user package in the pure-Go mode too.
	if !t.compileOnly {
		t.registerTest("os/user with tag osusergo",
			&goTest{
				variant: "osusergo",
				timeout: 300 * time.Second,
				tags:    []string{"osusergo"},
				pkg:     "os/user",
			})
	}

	// Tests that the nethttpomithttp2 build tag doesn't rot too much,
	// even if there's not a regular builder on it.
	t.registerTest("net/http with tag nethttpomithttp2", &goTest{
		variant: "nethttpomithttp2",
		tags:    []string{"nethttpomithttp2"},
		pkg:     "net/http",
	})

	// Check that all crypto packages compile with the purego build tag.
	t.registerTest("crypto with tag purego (build and vet only)", &goTest{
		variant:  "purego",
		tags:     []string{"purego"},
		pkg:      "crypto/...",
		runTests: "^$", // only ensure they compile
	})

	// Check that all crypto packages compile (and test correctly, in longmode) with fips.
	if t.fipsSupported() {
		// Test standard crypto packages with fips140=on.
		t.registerTest("GOFIPS140=latest go test crypto/...", &goTest{
			variant: "gofips140",
			env:     []string{"GOFIPS140=latest"},
			pkg:     "crypto/...",
		})

		// Test that earlier FIPS snapshots build.
		// In long mode, test that they work too.
		for _, version := range fipsVersions() {
			suffix := " # (build and vet only)"
			run := "^$" // only ensure they compile
			if !t.short {
				suffix = ""
				run = ""
			}
			t.registerTest("GOFIPS140="+version+" go test crypto/..."+suffix, &goTest{
				variant:  "gofips140-" + version,
				pkg:      "crypto/...",
				runTests: run,
				env:      []string{"GOFIPS140=" + version, "GOMODCACHE=" + filepath.Join(workdir, "fips-"+version)},
			})
		}
	}

	// Test GOEXPERIMENT=nojsonv2.
	if !strings.Contains(goexperiment, "nojsonv2") {
		t.registerTest("GOEXPERIMENT=nojsonv2 go test encoding/json/...", &goTest{
			variant: "nojsonv2",
			env:     []string{"GOEXPERIMENT=" + goexperiments("nojsonv2")},
			pkg:     "encoding/json/...",
		})
	}

	// Test GOEXPERIMENT=runtimesecret.
	if !strings.Contains(goexperiment, "runtimesecret") {
		t.registerTest("GOEXPERIMENT=runtimesecret go test runtime/secret/...", &goTest{
			variant: "runtimesecret",
			env:     []string{"GOEXPERIMENT=" + goexperiments("runtimesecret")},
			pkg:     "runtime/secret/...",
		})
	}

	// Test GOEXPERIMENT=simd.
	if !strings.Contains(goexperiment, "simd") {
		// simd package is portable.
		t.registerTest("GOEXPERIMENT=simd go test simd", &goTest{
			variant: "simd",
			env:     []string{"GOEXPERIMENT=" + goexperiments("simd")},
			pkg:     "simd",
		})
		// simd/archsimd supports amd64, arm64, and wasm.
		archsimdSupported := goarch == "amd64" || goarch == "arm64" || goarch == "wasm"
		if archsimdSupported {
			t.registerTest("GOEXPERIMENT=simd go test simd/archsimd/...", &goTest{
				variant: "simd",
				env:     []string{"GOEXPERIMENT=" + goexperiments("simd")},
				pkg:     "simd/archsimd/...",
			})
		}
	}

	// Test ios/amd64 for the iOS simulator.
	if goos == "darwin" && goarch == "amd64" && t.cgoEnabled {
		t.registerTest("GOOS=ios on darwin/amd64",
			&goTest{
				variant:  "amd64ios",
				timeout:  300 * time.Second,
				runTests: "SystemRoots",
				env:      []string{"GOOS=ios", "CGO_ENABLED=1"},
				pkg:      "crypto/x509",
			})
	}

	// GC debug mode tests. We only run these in long-test mode
	// (with GO_TEST_SHORT=0) because this is just testing a
	// non-critical debug setting.
	if !t.compileOnly && !t.short {
		t.registerTest("GODEBUG=gcstoptheworld=2 archive/zip",
			&goTest{
				variant: "gcstoptheworld2",
				timeout: 300 * time.Second,
				short:   true,
				env:     []string{"GODEBUG=gcstoptheworld=2"},
				pkg:     "archive/zip",
			})
		t.registerTest("GODEBUG=gccheckmark=1 runtime",
			&goTest{
				variant: "gccheckmark",
				timeout: 300 * time.Second,
				short:   true,
				env:     []string{"GODEBUG=gccheckmark=1"},
				pkg:     "runtime",
			})
	}

	// Spectre mitigation smoke test.
	if goos == "linux" && goarch == "amd64" && !(gogcflags == "-spectre=all" && t.asmflags == "all=-spectre=all") {
		// Pick a bunch of packages known to have some assembly.
		pkgs := []string{"internal/runtime/...", "reflect", "crypto/..."}
		if !t.short {
			pkgs = append(pkgs, "runtime")
		}
		t.registerTest("spectre",
			&goTest{
				variant: "spectre",
				short:   true,
				env:     []string{"GOFLAGS=-gcflags=all=-spectre=all -asmflags=all=-spectre=all"},
				pkgs:    pkgs,
			})
	}

	// morestack tests. We only run these in long-test mode
	// (with GO_TEST_SHORT=0) because the runtime test is
	// already quite long and mayMoreStackMove makes it about
	// twice as slow.
	if !t.compileOnly && !t.short {
		// hooks is the set of maymorestack hooks to test with.
		hooks := []string{"mayMoreStackPreempt", "mayMoreStackMove"}
		// hookPkgs is the set of package patterns to apply
		// the maymorestack hook to.
		hookPkgs := []string{"runtime/...", "reflect", "sync"}
		// unhookPkgs is the set of package patterns to
		// exclude from hookPkgs.
		unhookPkgs := []string{"runtime/testdata/..."}
		for _, hook := range hooks {
			// Construct the build flags to use the
			// maymorestack hook in the compiler and
			// assembler. We pass this via the GOFLAGS
			// environment variable so that it applies to
			// both the test itself and to binaries built
			// by the test.
			goFlagsList := []string{}
			for _, flag := range []string{"-gcflags", "-asmflags"} {
				for _, hookPkg := range hookPkgs {
					goFlagsList = append(goFlagsList, flag+"="+hookPkg+"=-d=maymorestack=runtime."+hook)
				}
				for _, unhookPkg := range unhookPkgs {
					goFlagsList = append(goFlagsList, flag+"="+unhookPkg+"=")
				}
			}
			goFlags := strings.Join(goFlagsList, " ")

			t.registerTest("maymorestack="+hook,
				&goTest{
					variant: hook,
					timeout: 600 * time.Second,
					short:   true,
					env:     []string{"GOFLAGS=" + goFlags},
					pkgs:    []string{"runtime", "reflect", "sync"},
				})
		}
	}

	// Test that internal linking of standard packages does not
	// require libgcc. This ensures that we can install a Go
	// release on a system that does not have a C compiler
	// installed and still build Go programs (that don't use cgo).
	for _, pkg := range cgoPackages {
		if !t.internalLink() {
			break
		}

		// ARM libgcc may be Thumb, which internal linking does not support.
		if goarch == "arm" {
			break
		}

		// What matters is that the tests build and start up.
		// Skip expensive tests, especially x509 TestSystemRoots.
		run := "^Test[^CS]"
		if pkg == "net" {
			run = "TestTCPStress"
		}
		t.registerTest("Testing without libgcc.",
			&goTest{
				variant:  "nolibgcc",
				ldflags:  "-linkmode=internal -libgcc=none",
				runTests: run,
				pkg:      pkg,
			})
	}

	// Stub out following test on alpine until 54354 resolved.
	builderName := os.Getenv("GO_BUILDER_NAME")
	disablePIE := strings.HasSuffix(builderName, "-alpine")

	// Test internal linking of PIE binaries where it is supported.
	if t.internalLinkPIE() && !disablePIE {
		t.registerTest("internal linking, -buildmode=pie",
			&goTest{
				variant:   "pie_internal",
				timeout:   60 * time.Second,
				buildmode: "pie",
				ldflags:   "-linkmode=internal",
				env:       []string{"CGO_ENABLED=0"},
				pkg:       "reflect",
			})
		t.registerTest("internal linking, -buildmode=pie",
			&goTest{
				variant:   "pie_internal",
				timeout:   60 * time.Second,
				buildmode: "pie",
				ldflags:   "-linkmode=internal",
				env:       []string{"CGO_ENABLED=0"},
				pkg:       "crypto/internal/fips140test",
				runTests:  "TestFIPSCheck",
			})
		// Also test a cgo package.
		if t.cgoEnabled && t.internalLink() && !disablePIE {
			t.registerTest("internal linking, -buildmode=pie",
				&goTest{
					variant:   "pie_internal",
					timeout:   60 * time.Second,
					buildmode: "pie",
					ldflags:   "-linkmode=internal",
					pkg:       "os/user",
				})
		}
	}

	if t.extLink() && !t.compileOnly {
		if goos != "android" { // Android does not support non-PIE linking
			t.registerTest("external linking, -buildmode=exe",
				&goTest{
					variant:   "exe_external",
					timeout:   60 * time.Second,
					buildmode: "exe",
					ldflags:   "-linkmode=external",
					env:       []string{"CGO_ENABLED=1"},
					pkg:       "crypto/internal/fips140test",
					runTests:  "TestFIPSCheck",
				})
		}
		if t.externalLinkPIE() && !disablePIE {
			t.registerTest("external linking, -buildmode=pie",
				&goTest{
					variant:   "pie_external",
					timeout:   60 * time.Second,
					buildmode: "pie",
					ldflags:   "-linkmode=external",
					env:       []string{"CGO_ENABLED=1"},
					pkg:       "crypto/internal/fips140test",
					runTests:  "TestFIPSCheck",
				})
		}
	}

	// sync tests
	if t.hasParallelism() {
		t.registerTest("sync -cpu=10",
			&goTest{
				variant: "cpu10",
				timeout: 120 * time.Second,
				cpu:     "10",
				pkg:     "sync",
			})
	}

	const cgoHeading = "Testing cgo"
	if t.cgoEnabled {
		t.registerCgoTests(cgoHeading)
	}

	if goos == "wasip1" {
		t.registerTest("wasip1 host tests",
			&goTest{
				variant:   "host",
				pkg:       "internal/runtime/wasitest",
				timeout:   1 * time.Minute,
				runOnHost: true,
			})
	}

	// Only run the API check on fast development platforms.
	// Every platform checks the API on every GOOS/GOARCH/CGO_ENABLED combination anyway,
	// so we really only need to run this check once anywhere to get adequate coverage.
	// To help developers avoid trybot-only failures, we try to run on typical developer machines
	// which is darwin,linux,windows/amd64 and darwin/arm64.
	//
	// The same logic applies to the release notes that correspond to each api/next file.
	//
	// TODO: remove the exclusion of goexperiment simd right before dev.simd branch is merged to master.
	if goos == "darwin" || ((goos == "linux" || goos == "windows") && (goarch == "amd64" && !strings.Contains(goexperiment, "simd"))) {
		t.registerTest("API release note check", &goTest{variant: "check", pkg: "cmd/relnote", testFlags: []string{"-check"}})
		t.registerTest("API check", &goTest{variant: "check", pkg: "cmd/api", timeout: 5 * time.Minute, testFlags: []string{"-check"}})
	}

	// Runtime CPU tests.
	if !t.compileOnly && t.hasParallelism() {
		for i := 1; i <= 4; i *= 2 {
			t.registerTest(fmt.Sprintf("GOMAXPROCS=2 runtime -cpu=%d -quick", i),
				&goTest{
					variant:   "cpu" + strconv.Itoa(i),
					timeout:   300 * time.Second,
					cpu:       strconv.Itoa(i),
					gcflags:   gogcflags,
					short:     true,
					testFlags: []string{"-quick"},
					// We set GOMAXPROCS=2 in addition to -cpu=1,2,4 in order to test runtime bootstrap code,
					// creation of first goroutines and first garbage collections in the parallel setting.
					env: []string{"GOMAXPROCS=2"},
					pkg: "runtime",
				})
		}
	}

	if t.raceDetectorSupported() && !t.msan && !t.asan {
		// N.B. -race is incompatible with -msan and -asan.
		t.registerRaceTests()
	}

	if goos != "android" && !t.iOS() {
		// Only start multiple test dir shards on builders,
		// where they get distributed to multiple machines.
		// See issues 20141 and 31834.
		nShards := 1
		if os.Getenv("GO_BUILDER_NAME") != "" {
			nShards = 10
		}
		if n, err := strconv.Atoi(os.Getenv("GO_TEST_SHARDS")); err == nil {
			nShards = n
		}
		for shard := 0; shard < nShards; shard++ {
			id := fmt.Sprintf("%d_%d", shard, nShards)
			t.registerTest("../test",
				&goTest{
					variant: id,
					// Include the variant even though there's no overlap in test names.
					// This makes the test target more clearly distinct in our build
					// results and is important for load-balancing test shards.
					omitVariant: false,
					pkg:         "cmd/internal/testdir",
					testFlags:   []string{fmt.Sprintf("-shard=%d", shard), fmt.Sprintf("-shards=%d", nShards)},
					runOnHost:   true,
				},
			)
		}
	}
}

// addTest adds an arbitrary test callback to the test list.
//
// name must uniquely identify the test and heading must be non-empty.
func (t *tester) addTest(name, heading string, fn func(*distTest) error) {
	if t.testNames[name] {
		panic("duplicate registered test name " + name)
	}
	if heading == "" {
		panic("empty heading")
	}
	// Two simple checks for cases that would conflict with the fast path in registerTests.
	if !strings.Contains(name, ":") && heading != "Testing packages." {
		panic("empty variant is reserved exclusively for registerStdTest")
	} else if strings.HasSuffix(name, ":racebench") && heading != "Running benchmarks briefly." {
		panic("racebench variant is reserved exclusively for registerRaceBenchTest")
	}
	if t.testNames == nil {
		t.testNames = make(map[string]bool)
	}
	t.testNames[name] = true
	t.tests = append(t.tests, distTest{
		name:    name,
		heading: heading,
		fn:      fn,
	})
}

type registerTestOpt interface {
	isRegisterTestOpt()
}

// rtSkipFunc is a registerTest option that runs a skip check function before
// running the test.
type rtSkipFunc struct {
	skip func(*distTest) (string, bool) // Return message, true to skip the test
}

func (rtSkipFunc) isRegisterTestOpt() {}

// registerTest registers a test that runs the given goTest.
//
// Each Go package in goTest will have a corresponding test
// "<pkg>:<variant>", which must uniquely identify the test.
//
// heading and test.variant must be non-empty.
func (t *tester) registerTest(heading string, test *goTest, opts ...registerTestOpt) {
	var skipFunc func(*distTest) (string, bool)
	for _, opt := range opts {
		switch opt := opt.(type) {
		case rtSkipFunc:
			skipFunc = opt.skip
		}
	}
	// Register each test package as a separate test.
	register1 := func(test *goTest) {
		if test.variant == "" {
			panic("empty variant")
		}
		name := testName(test.pkg, test.variant)
		t.addTest(name, heading, func(dt *distTest) error {
			if skipFunc != nil {
				msg, skip := skipFunc(dt)
				if skip {
					test.printSkip(t, msg)
					return nil
				}
			}
			w := &work{dt: dt}
			w.cmd, w.flush = test.bgCommand(t, &w.out, &w.out)
			t.worklist = append(t.worklist, w)
			return nil
		})
	}
	if test.pkg != "" && len(test.pkgs) == 0 {
		// Common case. Avoid copying.
		register1(test)
		return
	}
	// TODO(dmitshur,austin): It might be better to unify the execution of 'go test pkg'
	// invocations for the same variant to be done with a single 'go test pkg1 pkg2 pkg3'
	// command, just like it's already done in registerStdTest and registerRaceBenchTest.
	// Those methods accumulate matched packages in stdMatches and benchMatches slices,
	// and we can extend that mechanism to work for all other equal variant registrations.
	// Do the simple thing to start with.
	for _, pkg := range test.packages() {
		test1 := *test
		test1.pkg, test1.pkgs = pkg, nil
		register1(&test1)
	}
}

// dirCmd constructs a Cmd intended to be run in the foreground.
// The command will be run in dir, and Stdout and Stderr will go to os.Stdout
// and os.Stderr.
func (t *tester) dirCmd(dir string, cmdline ...any) *exec.Cmd {
	bin, args := flattenCmdline(cmdline)
	cmd := exec.Command(bin, args...)
	if filepath.IsAbs(dir) {
		setDir(cmd, dir)
	} else {
		setDir(cmd, filepath.Join(goroot, dir))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if vflag > 1 {
		errprintf("%#q\n", cmd)
	}
	return cmd
}

// flattenCmdline flattens a mixture of string and []string as single list
// and then interprets it as a command line: first element is binary, then args.
func flattenCmdline(cmdline []any) (bin string, args []string) {
	var list []string
	for _, x := range cmdline {
		switch x := x.(type) {
		case string:
			list = append(list, x)
		case []string:
			list = append(list, x...)
		default:
			panic("invalid dirCmd argument type: " + reflect.TypeOf(x).String())
		}
	}

	bin = list[0]
	if !filepath.IsAbs(bin) {
		panic("command is not absolute: " + bin)
	}
	return bin, list[1:]
}

func (t *tester) iOS() bool {
	return goos == "ios"
}

func (t *tester) out(v string) {
	if t.json {
		return
	}
	if t.banner == "" {
		return
	}
	fmt.Println("\n" + t.banner + v)
}

// extLink reports whether the current goos/goarch supports
// external linking.
func (t *tester) extLink() bool {
	if !cgoEnabled[goos+"/"+goarch] {
		return false
	}
	if goarch == "ppc64" && goos != "aix" && goos != "linux" {
		return false
	}
	return true
}

func (t *tester) internalLink() bool {
	if gohostos == "dragonfly" {
		// linkmode=internal fails on dragonfly since errno is a TLS relocation.
		return false
	}
	if goos == "android" {
		return false
	}
	if goos == "ios" {
		return false
	}
	// Internally linking cgo is incomplete on some architectures.
	// https://golang.org/issue/10373
	// https://golang.org/issue/14449
	if goarch == "mips64" || goarch == "mips64le" || goarch == "mips" || goarch == "mipsle" || goarch == "riscv64" {
		return false
	}
	if goos == "aix" {
		// linkmode=internal isn't supported.
		return false
	}
	if t.msan || t.asan {
		// linkmode=internal isn't supported by msan or asan.
		return false
	}
	return true
}

func (t *tester) internalLinkPIE() bool {
	if t.msan || t.asan {
		// linkmode=internal isn't supported by msan or asan.
		return false
	}
	switch goos + "-" + goarch {
	case "darwin-amd64", "darwin-arm64",
		"linux-amd64", "linux-arm64", "linux-loong64", "linux-ppc64", "linux-ppc64le", "linux-s390x",
		"android-arm64",
		"windows-amd64", "windows-386", "windows-arm64":
		return true
	}
	return false
}

func (t *tester) externalLinkPIE() bool {
	// General rule is if -buildmode=pie and -linkmode=external both work, then they work together.
	return t.internalLinkPIE() && t.extLink()
}

// supportedBuildmode reports whether the given build mode is supported.
func (t *tester) supportedBuildmode(mode string) bool {
	switch mode {
	case "c-archive", "c-shared", "shared", "plugin", "pie":
	default:
		fatalf("internal error: unknown buildmode %s", mode)
		return false
	}

	return buildModeSupported("gc", mode, goos, goarch)
}

func (t *tester) registerCgoTests(heading string) {
	cgoTest := func(variant string, subdir, linkmode, buildmode string, opts ...registerTestOpt) *goTest {
		gt := &goTest{
			variant:   variant,
			pkg:       "cmd/cgo/internal/" + subdir,
			buildmode: buildmode,
		}
		var ldflags []string
		if linkmode != "auto" {
			// "auto" is the default, so avoid cluttering the command line for "auto"
			ldflags = append(ldflags, "-linkmode="+linkmode)
		}

		if linkmode == "internal" {
			gt.tags = append(gt.tags, "internal")
			if buildmode == "pie" {
				gt.tags = append(gt.tags, "internal_pie")
			}
		}
		if buildmode == "static" {
			// This isn't actually a Go buildmode, just a convenient way to tell
			// cgoTest we want static linking.
			gt.buildmode = ""
			switch linkmode {
			case "external":
				ldflags = append(ldflags, `-extldflags "-static -pthread"`)
			case "auto":
				gt.env = append(gt.env, "CGO_LDFLAGS=-static -pthread")
			default:
				panic("unknown linkmode with static build: " + linkmode)
			}
			gt.tags = append(gt.tags, "static")
		}
		gt.ldflags = strings.Join(ldflags, " ")

		t.registerTest(heading, gt, opts...)
		return gt
	}

	// test, testtls, and testnocgo are run with linkmode="auto", buildmode=""
	// as part of go test cmd. Here we only have to register the non-default
	// build modes of these tests.

	// Stub out various buildmode=pie tests  on alpine until 54354 resolved.
	builderName := os.Getenv("GO_BUILDER_NAME")
	disablePIE := strings.HasSuffix(builderName, "-alpine")

	if t.internalLink() {
		cgoTest("internal", "test", "internal", "")
	}

	os := gohostos
	p := gohostos + "/" + goarch
	switch os {
	case "darwin", "windows":
		if !t.extLink() {
			break
		}
		// test linkmode=external, but __thread not supported, so skip testtls.
		cgoTest("external", "test", "external", "")

		gt := cgoTest("external-s", "test", "external", "")
		gt.ldflags += " -s"

		if t.supportedBuildmode("pie") && !disablePIE {
			cgoTest("auto-pie", "test", "auto", "pie")
			if t.internalLink() && t.internalLinkPIE() {
				cgoTest("internal-pie", "test", "internal", "pie")
			}
		}

	case "aix", "android", "dragonfly", "freebsd", "linux", "netbsd", "openbsd":
		gt := cgoTest("external-g0", "test", "external", "")
		gt.env = append(gt.env, "CGO_CFLAGS=-g0 -fdiagnostics-color")

		cgoTest("external", "testtls", "external", "")
		switch {
		case os == "aix":
			// no static linking
		case p == "freebsd/arm":
			// -fPIC compiled tls code will use __tls_get_addr instead
			// of __aeabi_read_tp, however, on FreeBSD/ARM, __tls_get_addr
			// is implemented in rtld-elf, so -fPIC isn't compatible with
			// static linking on FreeBSD/ARM with clang. (cgo depends on
			// -fPIC fundamentally.)
		default:
			// Check for static linking support
			var staticCheck rtSkipFunc
			ccName := compilerEnvLookup("CC", defaultcc, goos, goarch)
			cc, err := exec.LookPath(ccName)
			if err != nil {
				staticCheck.skip = func(*distTest) (string, bool) {
					return fmt.Sprintf("$CC (%q) not found, skip cgo static linking test.", ccName), true
				}
			} else {
				cmd := t.dirCmd("src/cmd/cgo/internal/test", cc, "-xc", "-o", "/dev/null", "-static", "-")
				cmd.Stdin = strings.NewReader("int main() {}")
				cmd.Stdout, cmd.Stderr = nil, nil // Discard output
				if err := cmd.Run(); err != nil {
					// Skip these tests
					staticCheck.skip = func(*distTest) (string, bool) {
						return "No support for static linking found (lacks libc.a?), skip cgo static linking test.", true
					}
				}
			}

			// Doing a static link with boringcrypto gets
			// a C linker warning on Linux.
			// in function `bio_ip_and_port_to_socket_and_addr':
			// warning: Using 'getaddrinfo' in statically linked applications requires at runtime the shared libraries from the glibc version used for linking
			if staticCheck.skip == nil && goos == "linux" && strings.Contains(goexperiment, "boringcrypto") {
				staticCheck.skip = func(*distTest) (string, bool) {
					return "skipping static linking check on Linux when using boringcrypto to avoid C linker warning about getaddrinfo", true
				}
			}

			// Static linking tests
			if goos != "android" && p != "netbsd/arm" && !t.msan && !t.asan {
				// TODO(#56629): Why does this fail on netbsd-arm?
				// TODO(#70080): Why does this fail with msan?
				// asan doesn't support static linking (this is an explicit build error on the C side).
				cgoTest("static", "testtls", "external", "static", staticCheck)
			}
			cgoTest("external", "testnocgo", "external", "", staticCheck)
			if goos != "android" && !t.msan && !t.asan {
				// TODO(#70080): Why does this fail with msan?
				// asan doesn't support static linking (this is an explicit build error on the C side).
				cgoTest("static", "testnocgo", "external", "static", staticCheck)
				cgoTest("static", "test", "external", "static", staticCheck)
				// -static in CGO_LDFLAGS triggers a different code path
				// than -static in -extldflags, so test both.
				// See issue #16651.
				if goarch != "loong64" && !t.msan && !t.asan {
					// TODO(#56623): Why does this fail on loong64?
					cgoTest("auto-static", "test", "auto", "static", staticCheck)
				}
			}

			// PIE linking tests
			if t.supportedBuildmode("pie") && !disablePIE {
				cgoTest("auto-pie", "test", "auto", "pie")
				if t.internalLink() && t.internalLinkPIE() {
					cgoTest("internal-pie", "test", "internal", "pie")
				}
				cgoTest("auto-pie", "testtls", "auto", "pie")
				cgoTest("auto-pie", "testnocgo", "auto", "pie")
			}
		}
	}
}

// runPending runs pending test commands, in parallel, emitting headers as appropriate.
// When finished, it emits header for nextTest, which is going to run after the
// pending commands are done (and runPending returns).
// A test should call runPending if it wants to make sure that it is not
// running in parallel with earlier tests, or if it has some other reason
// for needing the earlier tests to be done.
func (t *tester) runPending(nextTest *distTest) {
	worklist := t.worklist
	t.worklist = nil
	for _, w := range worklist {
		w.start = make(chan bool)
		w.end = make(chan struct{})
		// w.cmd must be set up to write to w.out. We can't check that, but we
		// can check for easy mistakes.
		if w.cmd.Stdout == nil || w.cmd.Stdout == os.Stdout || w.cmd.Stderr == nil || w.cmd.Stderr == os.Stderr {
			panic("work.cmd.Stdout/Stderr must be redirected")
		}
		go func(w *work) {
			if !<-w.start {
				timelog("skip", w.dt.name)
				w.printSkip(t, "skipped due to earlier error")
			} else {
				timelog("start", w.dt.name)
				w.err = w.cmd.Run()
				if w.flush != nil {
					w.flush()
				}
				if w.err != nil {
					if isUnsupportedVMASize(w) {
						timelog("skip", w.dt.name)
						w.out.Reset()
						w.printSkip(t, "skipped due to unsupported VMA")
						w.err = nil
					}
				}
			}
			timelog("end", w.dt.name)
			w.end <- struct{}{}
		}(w)
	}

	maxbg := maxbg
	// for runtime.NumCPU() < 4 ||  runtime.GOMAXPROCS(0) == 1, do not change maxbg.
	// Because there is not enough CPU to parallel the testing of multiple packages.
	if runtime.NumCPU() > 4 && runtime.GOMAXPROCS(0) != 1 {
		for _, w := range worklist {
			// See go.dev/issue/65164
			// because GOMAXPROCS=2 runtime CPU usage is low,
			// so increase maxbg to avoid slowing down execution with low CPU usage.
			// This makes testing a single package slower,
			// but testing multiple packages together faster.
			if strings.Contains(w.dt.heading, "GOMAXPROCS=2 runtime") {
				maxbg = runtime.NumCPU()
				break
			}
		}
	}

	started := 0
	ended := 0
	var last *distTest
	for ended < len(worklist) {
		for started < len(worklist) && started-ended < maxbg {
			w := worklist[started]
			started++
			w.start <- !t.failed || t.keepGoing
		}
		w := worklist[ended]
		dt := w.dt
		if t.lastHeading != dt.heading {
			t.lastHeading = dt.heading
			t.out(dt.heading)
		}
		if dt != last {
			// Assumes all the entries for a single dt are in one worklist.
			last = w.dt
			if vflag > 0 {
				fmt.Printf("# go tool dist test -run=^%s$\n", dt.name)
			}
		}
		if vflag > 1 {
			errprintf("%#q\n", w.cmd)
		}
		ended++
		<-w.end
		os.Stdout.Write(w.out.Bytes())
		// We no longer need the output, so drop the buffer.
		w.out = bytes.Buffer{}
		if w.err != nil {
			log.Printf("Failed: %v", w.err)
			t.failed = true
		}
	}
	if t.failed && !t.keepGoing {
		fatalf("FAILED")
	}

	if dt := nextTest; dt != nil {
		if t.lastHeading != dt.heading {
			t.lastHeading = dt.heading
			t.out(dt.heading)
		}
		if vflag > 0 {
			fmt.Printf("# go tool dist test -run=^%s$\n", dt.name)
		}
	}
}

// hasParallelism is a copy of the function
// internal/testenv.HasParallelism, which can't be used here
// because cmd/dist can not import internal packages during bootstrap.
func (t *tester) hasParallelism() bool {
	switch goos {
	case "js", "wasip1":
		return false
	}
	return true
}

func (t *tester) raceDetectorSupported() bool {
	if gohostos != goos {
		return false
	}
	if !t.cgoEnabled {
		return false
	}
	if !raceDetectorSupported(goos, goarch) {
		return false
	}
	// The race detector doesn't work on Alpine Linux:
	// golang.org/issue/14481
	if isAlpineLinux() {
		return false
	}
	// NetBSD support is unfinished.
	// golang.org/issue/26403
	if goos == "netbsd" {
		return false
	}
	return true
}

func isAlpineLinux() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	fi, err := os.Lstat("/etc/alpine-release")
	return err == nil && fi.Mode().IsRegular()
}

func (t *tester) registerRaceTests() {
	hdr := "Testing race detector"
	t.registerTest(hdr,
		&goTest{
			variant:  "race",
			race:     true,
			runTests: "Output",
			pkg:      "runtime/race",
		})
	t.registerTest(hdr,
		&goTest{
			variant:  "race",
			race:     true,
			runTests: "TestParse|TestEcho|TestStdinCloseRace|TestClosedPipeRace|TestTypeRace|TestFdRace|TestFdReadRace|TestFileCloseRace",
			pkgs:     []string{"flag", "net", "os", "os/exec", "encoding/gob"},
		})
	// We don't want the following line, because it
	// slows down all.bash (by 10 seconds on my laptop).
	// The race builder should catch any error here, but doesn't.
	// TODO(iant): Figure out how to catch this.
	// t.registerTest(hdr, &goTest{variant: "race", race: true, runTests: "TestParallelTest", pkg: "cmd/go"})
	if t.cgoEnabled {
		// Building cmd/cgo/internal/test takes a long time.
		// There are already cgo-enabled packages being tested with the race detector.
		// We shouldn't need to redo all of cmd/cgo/internal/test too.
		// The race builder will take care of this.
		// t.registerTest(hdr, &goTest{variant: "race", race: true, env: []string{"GOTRACEBACK=2"}, pkg: "cmd/cgo/internal/test"})
	}
	if t.extLink() {
		// Test with external linking; see issue 9133.
		t.registerTest(hdr,
			&goTest{
				variant:  "race-external",
				race:     true,
				ldflags:  "-linkmode=external",
				runTests: "TestParse|TestEcho|TestStdinCloseRace",
				pkgs:     []string{"flag", "os/exec"},
			})
	}
}

// cgoPackages is the standard packages that use cgo.
var cgoPackages = []string{
	"net",
	"os/user",
}

var funcBenchmark = []byte("\nfunc Benchmark")

// packageHasBenchmarks reports whether pkg has benchmarks.
// On any error, it conservatively returns true.
//
// This exists just to eliminate work on the builders, since compiling
// a test in race mode just to discover it has no benchmarks costs a
// second or two per package, and this function returns false for
// about 100 packages.
func (t *tester) packageHasBenchmarks(pkg string) bool {
	pkgDir := filepath.Join(goroot, "src", pkg)
	d, err := os.Open(pkgDir)
	if err != nil {
		return true // conservatively
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return true // conservatively
	}
	for _, name := range names {
		if !strings.HasSuffix(name, "_test.go") {
			continue
		}
		slurp, err := os.ReadFile(filepath.Join(pkgDir, name))
		if err != nil {
			return true // conservatively
		}
		if bytes.Contains(slurp, funcBenchmark) {
			return true
		}
	}
	return false
}

// makeGOROOTUnwritable makes all $GOROOT files & directories non-writable to
// check that no tests accidentally write to $GOROOT.
func (t *tester) makeGOROOTUnwritable() (undo func()) {
	dir := os.Getenv("GOROOT")
	if dir == "" {
		panic("GOROOT not set")
	}

	type pathMode struct {
		path string
		mode os.FileMode
	}
	var dirs []pathMode // in lexical order

	undo = func() {
		for i := range dirs {
			os.Chmod(dirs[i].path, dirs[i].mode) // best effort
		}
	}

	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if suffix := strings.TrimPrefix(path, dir+string(filepath.Separator)); suffix != "" {
			if suffix == ".git" {
				// Leave Git metadata in whatever state it was in. It may contain a lot
				// of files, and it is highly unlikely that a test will try to modify
				// anything within that directory.
				return filepath.SkipDir
			}
		}
		if err != nil {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		mode := info.Mode()
		if mode&0222 != 0 && (mode.IsDir() || mode.IsRegular()) {
			dirs = append(dirs, pathMode{path, mode})
		}
		return nil
	})

	// Run over list backward to chmod children before parents.
	for i := len(dirs) - 1; i >= 0; i-- {
		err := os.Chmod(dirs[i].path, dirs[i].mode&^0222)
		if err != nil {
			dirs = dirs[i:] // Only undo what we did so far.
			undo()
			fatalf("failed to make GOROOT read-only: %v", err)
		}
	}

	return undo
}

// raceDetectorSupported is a copy of the function
// internal/platform.RaceDetectorSupported, which can't be used here
// because cmd/dist can not import internal packages during bootstrap.
// The race detector only supports 48-bit VMA on arm64. But we don't have
// a good solution to check VMA size (see https://go.dev/issue/29948).
// raceDetectorSupported will always return true for arm64. But race
// detector tests may abort on non 48-bit VMA configuration, the tests
// will be marked as "skipped" in this case.
func raceDetectorSupported(goos, goarch string) bool {
	switch goos {
	case "linux":
		return goarch == "amd64" || goarch == "arm64" || goarch == "loong64" || goarch == "ppc64le" || goarch == "riscv64" || goarch == "s390x"
	case "darwin":
		return goarch == "amd64" || goarch == "arm64"
	case "freebsd", "netbsd", "windows":
		return goarch == "amd64"
	default:
		return false
	}
}

// buildModeSupported is a copy of the function
// internal/platform.BuildModeSupported, which can't be used here
// because cmd/dist can not import internal packages during bootstrap.
func buildModeSupported(compiler, buildmode, goos, goarch string) bool {
	if compiler == "gccgo" {
		return true
	}

	platform := goos + "/" + goarch

	switch buildmode {
	case "archive":
		return true

	case "c-archive":
		switch goos {
		case "aix", "darwin", "ios", "windows":
			return true
		case "linux":
			switch goarch {
			case "386", "amd64", "arm", "armbe", "arm64", "arm64be", "loong64", "ppc64", "ppc64le", "riscv64", "s390x":
				return true
			default:
				// Other targets do not support -shared,
				// per ParseFlags in
				// cmd/compile/internal/base/flag.go.
				// For c-archive the Go tool passes -shared,
				// so that the result is suitable for inclusion
				// in a PIE or shared library.
				return false
			}
		case "freebsd":
			return goarch == "amd64"
		}
		return false

	case "c-shared":
		switch platform {
		case "linux/amd64", "linux/arm", "linux/arm64", "linux/loong64", "linux/386", "linux/ppc64", "linux/ppc64le", "linux/riscv64", "linux/s390x",
			"android/amd64", "android/arm", "android/arm64", "android/386",
			"freebsd/amd64",
			"darwin/amd64", "darwin/arm64",
			"windows/amd64", "windows/386", "windows/arm64",
			"wasip1/wasm":
			return true
		}
		return false

	case "default":
		return true

	case "exe":
		return true

	case "pie":
		switch platform {
		case "linux/386", "linux/amd64", "linux/arm", "linux/arm64", "linux/loong64", "linux/ppc64", "linux/ppc64le", "linux/riscv64", "linux/s390x",
			"android/amd64", "android/arm", "android/arm64", "android/386",
			"freebsd/amd64",
			"darwin/amd64", "darwin/arm64",
			"ios/amd64", "ios/arm64",
			"aix/ppc64",
			"openbsd/arm64",
			"windows/386", "windows/amd64", "windows/arm64":
			return true
		}
		return false

	case "shared":
		switch platform {
		case "linux/386", "linux/amd64", "linux/arm", "linux/arm64", "linux/ppc64", "linux/ppc64le", "linux/s390x":
			return true
		}
		return false

	case "plugin":
		switch platform {
		case "linux/amd64", "linux/arm", "linux/arm64", "linux/386", "linux/loong64", "linux/riscv64", "linux/s390x", "linux/ppc64", "linux/ppc64le",
			"android/amd64", "android/386",
			"darwin/amd64", "darwin/arm64",
			"freebsd/amd64":
			return true
		}
		return false

	default:
		return false
	}
}

// isUnsupportedVMASize reports whether the failure is caused by an unsupported
// VMA for the race detector (for example, running the race detector on an
// arm64 machine configured with 39-bit VMA).
func isUnsupportedVMASize(w *work) bool {
	unsupportedVMA := []byte("unsupported VMA range")
	return strings.Contains(w.dt.name, ":race") && bytes.Contains(w.out.Bytes(), unsupportedVMA)
}

// isEnvSet reports whether the environment variable evar is
// set in the environment.
func isEnvSet(evar string) bool {
	evarEq := evar + "="
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, evarEq) {
			return true
		}
	}
	return false
}

func (t *tester) fipsSupported() bool {
	// Keep this in sync with [crypto/internal/fips140.Supported].

	// We don't test with the purego tag, so no need to check it.

	// Use GOFIPS140 or GOEXPERIMENT=boringcrypto, but not both.
	if strings.Contains(goexperiment, "boringcrypto") {
		return false
	}

	// If this goos/goarch does not support FIPS at all, return no versions.
	// The logic here matches crypto/internal/fips140/check.Supported for now.
	// In the future, if some snapshots add support for these, we will have
	// to make a decision on a per-version basis.
	switch {
	case goarch == "wasm",
		goos == "windows" && goarch == "386",
		goos == "openbsd",
		goos == "aix":
		return false
	}

	// For now, FIPS+ASAN doesn't need to work.
	// If this is made to work, also re-enable the test in check_test.go.
	if t.asan {
		return false
	}

	return true
}

// fipsVersions returns the list of versions available in lib/fips140.
func fipsVersions() []string {
	var versions []string
	zips, err := filepath.Glob(filepath.Join(goroot, "lib/fips140/*.zip"))
	if err != nil {
		fatalf("%v", err)
	}
	for _, zip := range zips {
		versions = append(versions, strings.TrimSuffix(filepath.Base(zip), ".zip"))
	}
	txts, err := filepath.Glob(filepath.Join(goroot, "lib/fips140/*.txt"))
	if err != nil {
		fatalf("%v", err)
	}
	for _, txt := range txts {
		versions = append(versions, strings.TrimSuffix(filepath.Base(txt), ".txt"))
	}
	return versions
}

// goexperiments returns the GOEXPERIMENT value to use
// when running a test with the given experiments enabled.
//
// It preserves any existing GOEXPERIMENTs.
func goexperiments(exps ...string) string {
	if len(exps) == 0 {
		return goexperiment
	}
	existing := goexperiment
	if existing != "" {
		existing += ","
	}
	return existing + strings.Join(exps, ",")

}

```

// === FILE: references!/go/src/cmd/dist/testjson.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// lockedWriter serializes Write calls to an underlying Writer.
type lockedWriter struct {
	lock sync.Mutex
	w    io.Writer
}

func (w *lockedWriter) Write(b []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	return w.w.Write(b)
}

// testJSONFilter is an io.Writer filter that replaces the Package field in
// test2json output.
type testJSONFilter struct {
	w       io.Writer // Underlying writer
	variant string    // Add ":variant" to Package field

	lineBuf bytes.Buffer // Buffer for incomplete lines
}

func (f *testJSONFilter) Write(b []byte) (int, error) {
	bn := len(b)

	// Process complete lines, and buffer any incomplete lines.
	for len(b) > 0 {
		nl := bytes.IndexByte(b, '\n')
		if nl < 0 {
			f.lineBuf.Write(b)
			break
		}
		var line []byte
		if f.lineBuf.Len() > 0 {
			// We have buffered data. Add the rest of the line from b and
			// process the complete line.
			f.lineBuf.Write(b[:nl+1])
			line = f.lineBuf.Bytes()
		} else {
			// Process a complete line from b.
			line = b[:nl+1]
		}
		b = b[nl+1:]
		f.process(line)
		f.lineBuf.Reset()
	}

	return bn, nil
}

func (f *testJSONFilter) Flush() {
	// Write any remaining partial line to the underlying writer.
	if f.lineBuf.Len() > 0 {
		f.w.Write(f.lineBuf.Bytes())
		f.lineBuf.Reset()
	}
}

func (f *testJSONFilter) process(line []byte) {
	if len(line) > 0 && line[0] == '{' {
		// Plausible test2json output. Parse it generically.
		//
		// We go to some effort here to preserve key order while doing this
		// generically. This will stay robust to changes in the test2json
		// struct, or other additions outside of it. If humans are ever looking
		// at the output, it's really nice to keep field order because it
		// preserves a lot of regularity in the output.
		lineBuf := bytes.NewBuffer(line)
		dec := json.NewDecoder(lineBuf)
		dec.UseNumber()
		val, err := decodeJSONValue(dec)
		if err == nil && val.atom == json.Delim('{') {
			// Rewrite the Package field.
			found := false
			for i := 0; i < len(val.seq); i += 2 {
				if val.seq[i].atom == "Package" {
					if pkg, ok := val.seq[i+1].atom.(string); ok {
						val.seq[i+1].atom = pkg + ":" + f.variant
						found = true
						break
					}
				}
			}
			if found {
				data, err := json.Marshal(val)
				if err != nil {
					// Should never happen.
					panic(fmt.Sprintf("failed to round-trip JSON %q: %s", line, err))
				}
				f.w.Write(data)
				// Copy any trailing text. We expect at most a "\n" here, but
				// there could be other text and we want to feed that through.
				io.Copy(f.w, dec.Buffered())
				io.Copy(f.w, lineBuf)
				return
			}
		}
	}

	// Something went wrong. Just pass the line through.
	f.w.Write(line)
}

type jsonValue struct {
	atom json.Token  // If json.Delim, then seq will be filled
	seq  []jsonValue // If atom == json.Delim('{'), alternating pairs
}

var jsonPop = errors.New("end of JSON sequence")

func decodeJSONValue(dec *json.Decoder) (jsonValue, error) {
	t, err := dec.Token()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return jsonValue{}, err
	}

	switch t := t.(type) {
	case json.Delim:
		if t == '}' || t == ']' {
			return jsonValue{}, jsonPop
		}

		var seq []jsonValue
		for {
			val, err := decodeJSONValue(dec)
			if err == jsonPop {
				break
			} else if err != nil {
				return jsonValue{}, err
			}
			seq = append(seq, val)
		}
		return jsonValue{t, seq}, nil
	default:
		return jsonValue{t, nil}, nil
	}
}

func (v jsonValue) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	var marshal1 func(v jsonValue) error
	marshal1 = func(v jsonValue) error {
		if t, ok := v.atom.(json.Delim); ok {
			buf.WriteRune(rune(t))
			for i, v2 := range v.seq {
				if t == '{' && i%2 == 1 {
					buf.WriteByte(':')
				} else if i > 0 {
					buf.WriteByte(',')
				}
				if err := marshal1(v2); err != nil {
					return err
				}
			}
			if t == '{' {
				buf.WriteByte('}')
			} else {
				buf.WriteByte(']')
			}
			return nil
		}
		bytes, err := json.Marshal(v.atom)
		if err != nil {
			return err
		}
		buf.Write(bytes)
		return nil
	}
	err := marshal1(v)
	return buf.Bytes(), err
}

func synthesizeSkipEvent(enc *json.Encoder, pkg, msg string) {
	type event struct {
		Time    time.Time
		Action  string
		Package string
		Output  string `json:",omitempty"`
	}
	ev := event{Time: time.Now(), Package: pkg, Action: "start"}
	enc.Encode(ev)
	ev.Action = "output"
	ev.Output = msg
	enc.Encode(ev)
	ev.Action = "skip"
	ev.Output = ""
	enc.Encode(ev)
}

```

// === FILE: references!/go/src/cmd/dist/util.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// pathf is fmt.Sprintf for generating paths
// (on windows it turns / into \ after the printf).
func pathf(format string, args ...any) string {
	return filepath.Clean(fmt.Sprintf(format, args...))
}

// filter returns a slice containing the elements x from list for which f(x) == true.
func filter(list []string, f func(string) bool) []string {
	var out []string
	for _, x := range list {
		if f(x) {
			out = append(out, x)
		}
	}
	return out
}

// uniq returns a sorted slice containing the unique elements of list.
func uniq(list []string) []string {
	out := make([]string, len(list))
	copy(out, list)
	sort.Strings(out)
	keep := out[:0]
	for _, x := range out {
		if len(keep) == 0 || keep[len(keep)-1] != x {
			keep = append(keep, x)
		}
	}
	return keep
}

const (
	CheckExit = 1 << iota
	ShowOutput
	Background
)

var outputLock sync.Mutex

// run is like runEnv with no additional environment.
func run(dir string, mode int, cmd ...string) string {
	return runEnv(dir, mode, nil, cmd...)
}

// runEnv runs the command line cmd in dir with additional environment env.
// If mode has ShowOutput set and Background unset, run passes cmd's output to
// stdout/stderr directly. Otherwise, run returns cmd's output as a string.
// If mode has CheckExit set and the command fails, run calls fatalf.
// If mode has Background set, this command is being run as a
// Background job. Only bgrun should use the Background mode,
// not other callers.
func runEnv(dir string, mode int, env []string, cmd ...string) string {
	if vflag > 1 {
		errprintf("run: %s\n", strings.Join(cmd, " "))
	}

	xcmd := exec.Command(cmd[0], cmd[1:]...)
	if env != nil {
		xcmd.Env = append(os.Environ(), env...)
	}
	setDir(xcmd, dir)
	var data []byte
	var err error

	// If we want to show command output and this is not
	// a background command, assume it's the only thing
	// running, so we can just let it write directly stdout/stderr
	// as it runs without fear of mixing the output with some
	// other command's output. Not buffering lets the output
	// appear as it is printed instead of once the command exits.
	// This is most important for the invocation of 'go build -v bootstrap/...'.
	if mode&(Background|ShowOutput) == ShowOutput {
		xcmd.Stdout = os.Stdout
		xcmd.Stderr = os.Stderr
		err = xcmd.Run()
	} else {
		data, err = xcmd.CombinedOutput()
	}
	if err != nil && mode&CheckExit != 0 {
		outputLock.Lock()
		if len(data) > 0 {
			xprintf("%s\n", data)
		}
		outputLock.Unlock()
		if mode&Background != 0 {
			// Prevent fatalf from waiting on our own goroutine's
			// bghelper to exit:
			bghelpers.Done()
		}
		fatalf("FAILED: %v: %v", strings.Join(cmd, " "), err)
	}
	if mode&ShowOutput != 0 {
		outputLock.Lock()
		os.Stdout.Write(data)
		outputLock.Unlock()
	}
	if vflag > 2 {
		errprintf("run: %s DONE\n", strings.Join(cmd, " "))
	}
	return string(data)
}

var maxbg = 4 /* maximum number of jobs to run at once */

var (
	bgwork = make(chan func(), 1e5)

	bghelpers sync.WaitGroup

	dieOnce sync.Once // guards close of dying
	dying   = make(chan struct{})
)

func bginit() {
	bghelpers.Add(maxbg)
	for i := 0; i < maxbg; i++ {
		go bghelper()
	}
}

func bghelper() {
	defer bghelpers.Done()
	for {
		select {
		case <-dying:
			return
		case w := <-bgwork:
			// Dying takes precedence over doing more work.
			select {
			case <-dying:
				return
			default:
				w()
			}
		}
	}
}

// bgrun is like run but runs the command in the background.
// CheckExit|ShowOutput mode is implied (since output cannot be returned).
// bgrun adds 1 to wg immediately, and calls Done when the work completes.
func bgrun(wg *sync.WaitGroup, dir string, cmd ...string) {
	wg.Add(1)
	bgwork <- func() {
		defer wg.Done()
		run(dir, CheckExit|ShowOutput|Background, cmd...)
	}
}

// bgwait waits for pending bgruns to finish.
// bgwait must be called from only a single goroutine at a time.
func bgwait(wg *sync.WaitGroup) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-dying:
		// Don't return to the caller, to avoid reporting additional errors
		// to the user.
		select {}
	}
}

// xgetwd returns the current directory.
func xgetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		fatalf("%s", err)
	}
	return wd
}

// xrealwd returns the 'real' name for the given path.
// real is defined as what xgetwd returns in that directory.
func xrealwd(path string) string {
	old := xgetwd()
	if err := os.Chdir(path); err != nil {
		fatalf("chdir %s: %v", path, err)
	}
	real := xgetwd()
	if err := os.Chdir(old); err != nil {
		fatalf("chdir %s: %v", old, err)
	}
	return real
}

// isdir reports whether p names an existing directory.
func isdir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// isfile reports whether p names an existing file.
func isfile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// mtime returns the modification time of the file p.
func mtime(p string) time.Time {
	fi, err := os.Stat(p)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

// readfile returns the content of the named file.
func readfile(file string) string {
	data, err := os.ReadFile(file)
	if err != nil {
		fatalf("%v", err)
	}
	return string(data)
}

const (
	writeExec = 1 << iota
	writeSkipSame
)

// writefile writes text to the named file, creating it if needed.
// if exec is non-zero, marks the file as executable.
// If the file already exists and has the expected content,
// it is not rewritten, to avoid changing the time stamp.
func writefile(text, file string, flag int) {
	new := []byte(text)
	if flag&writeSkipSame != 0 {
		old, err := os.ReadFile(file)
		if err == nil && bytes.Equal(old, new) {
			return
		}
	}
	mode := os.FileMode(0666)
	if flag&writeExec != 0 {
		mode = 0777
	}
	xremove(file) // in case of symlink tricks by misc/reboot test
	err := os.WriteFile(file, new, mode)
	if err != nil {
		fatalf("%v", err)
	}
}

// xmkdir creates the directory p.
func xmkdir(p string) {
	err := os.Mkdir(p, 0777)
	if err != nil {
		fatalf("%v", err)
	}
}

// xmkdirall creates the directory p and its parents, as needed.
func xmkdirall(p string) {
	err := os.MkdirAll(p, 0777)
	if err != nil {
		fatalf("%v", err)
	}
}

// xremove removes the file p.
func xremove(p string) {
	if vflag > 2 {
		errprintf("rm %s\n", p)
	}
	os.Remove(p)
}

// xremoveall removes the file or directory tree rooted at p.
func xremoveall(p string) {
	if vflag > 2 {
		errprintf("rm -r %s\n", p)
	}
	os.RemoveAll(p)
}

// xreaddir replaces dst with a list of the names of the files and subdirectories in dir.
// The names are relative to dir; they are not full paths.
func xreaddir(dir string) []string {
	f, err := os.Open(dir)
	if err != nil {
		fatalf("%v", err)
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		fatalf("reading %s: %v", dir, err)
	}
	return names
}

// xworkdir creates a new temporary directory to hold object files
// and returns the name of that directory.
func xworkdir() string {
	name, err := os.MkdirTemp(os.Getenv("GOTMPDIR"), "go-tool-dist-")
	if err != nil {
		fatalf("%v", err)
	}
	return name
}

// fatalf prints an error message to standard error and exits.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "go tool dist: %s\n", fmt.Sprintf(format, args...))

	dieOnce.Do(func() { close(dying) })

	// Wait for background goroutines to finish,
	// so that exit handler that removes the work directory
	// is not fighting with active writes or open files.
	bghelpers.Wait()

	xexit(2)
}

var atexits []func()

// xexit exits the process with return code n.
func xexit(n int) {
	for i := len(atexits) - 1; i >= 0; i-- {
		atexits[i]()
	}
	os.Exit(n)
}

// xatexit schedules the exit-handler f to be run when the program exits.
func xatexit(f func()) {
	atexits = append(atexits, f)
}

// xprintf prints a message to standard output.
func xprintf(format string, args ...any) {
	fmt.Printf(format, args...)
}

// errprintf prints a message to standard output.
func errprintf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}

func xgetgoarm() string {
	// If we're building on an actual arm system, and not building
	// a cross-compiling toolchain, try to exec ourselves
	// to detect whether VFP is supported and set the default GOARM.
	// We've always assumed Android is ARMv7.
	if gohostarch == "arm" && goarch == "arm" && goos == gohostos && goos != "android" {
		// Try to exec ourselves in a mode to detect VFP support.
		// Seeing how far it gets determines which instructions failed.
		// The test is OS-agnostic.
		out := run("", 0, os.Args[0], "-check-goarm")
		v1ok := strings.Contains(out, "VFPv1 OK.")
		v3ok := strings.Contains(out, "VFPv3 OK.")
		if v1ok && v3ok {
			return "7"
		}
		if v1ok {
			return "6"
		}
		return "5"
	}

	// Otherwise, in the absence of local information, assume GOARM=7.
	//
	// We used to assume GOARM=5 in certain contexts but not others,
	// which produced inconsistent results. For example if you cross-compiled
	// for linux/arm from a windows/amd64 machine, you got GOARM=7 binaries,
	// but if you cross-compiled for linux/arm from a linux/amd64 machine,
	// you got GOARM=5 binaries. Now the default is independent of the
	// host operating system, for better reproducibility of builds.
	return "7"
}

// elfIsLittleEndian detects if the ELF file is little endian.
func elfIsLittleEndian(fn string) bool {
	// read the ELF file header to determine the endianness without using the
	// debug/elf package.
	file, err := os.Open(fn)
	if err != nil {
		fatalf("failed to open file to determine endianness: %v", err)
	}
	defer file.Close()
	var hdr [16]byte
	if _, err := io.ReadFull(file, hdr[:]); err != nil {
		fatalf("failed to read ELF header to determine endianness: %v", err)
	}
	// hdr[5] is EI_DATA byte, 1 is ELFDATA2LSB and 2 is ELFDATA2MSB
	switch hdr[5] {
	default:
		fatalf("unknown ELF endianness of %s: EI_DATA = %d", fn, hdr[5])
	case 1:
		return true
	case 2:
		return false
	}
	panic("unreachable")
}

// count is a flag.Value that is like a flag.Bool and a flag.Int.
// If used as -name, it increments the count, but -name=x sets the count.
// Used for verbose flag -v.
type count int

func (c *count) String() string {
	return fmt.Sprint(int(*c))
}

func (c *count) Set(s string) error {
	switch s {
	case "true":
		*c++
	case "false":
		*c = 0
	default:
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("invalid count %q", s)
		}
		*c = count(n)
	}
	return nil
}

func (c *count) IsBoolFlag() bool {
	return true
}

func xflagparse(maxargs int) {
	flag.Var((*count)(&vflag), "v", "verbosity")
	flag.Parse()
	if maxargs >= 0 && flag.NArg() > maxargs {
		flag.Usage()
	}
}

```

// === FILE: references!/go/src/cmd/dist/util_gc.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build gc

package main

// useVFPv1 tries to execute one VFPv1 instruction on ARM.
// It will crash the current process if VFPv1 is missing.
func useVFPv1()

// useVFPv3 tries to execute one VFPv3 instruction on ARM.
// It will crash the current process if VFPv3 is missing.
func useVFPv3()

// useARMv6K tries to run ARMv6K instructions on ARM.
// It will crash the current process if it doesn't implement
// ARMv6K or above.
func useARMv6K()

```

// === FILE: references!/go/src/cmd/dist/util_gccgo.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build gccgo

package main

func useVFPv1() {}

func useVFPv3() {}

func useARMv6K() {}

```

// === FILE: references!/go/src/cmd/dist/vfp_arm.s ===
```text
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build gc

#include "textflag.h"

// try to run "vmov.f64 d0, d0" instruction
TEXT ·useVFPv1(SB),NOSPLIT,$0
	WORD $0xeeb00b40	// vmov.f64 d0, d0
	RET

// try to run VFPv3-only "vmov.f64 d0, #112" instruction
TEXT ·useVFPv3(SB),NOSPLIT,$0
	WORD $0xeeb70b00	// vmov.f64 d0, #112
	RET

// try to run ARMv6K (or above) "ldrexd" instruction
TEXT ·useARMv6K(SB),NOSPLIT,$32
	MOVW R13, R2
	BIC  $15, R13
	WORD $0xe1bd0f9f	// ldrexd r0, r1, [sp]
	WORD $0xf57ff01f	// clrex
	MOVW R2, R13
	RET

```

// === FILE: references!/go/src/cmd/dist/vfp_default.s ===
```text
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build gc && !arm

#include "textflag.h"

TEXT ·useVFPv1(SB),NOSPLIT,$0
	RET

TEXT ·useVFPv3(SB),NOSPLIT,$0
	RET

TEXT ·useARMv6K(SB),NOSPLIT,$0
	RET

```

