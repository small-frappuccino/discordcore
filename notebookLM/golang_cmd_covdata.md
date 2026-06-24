# Domain Architecture: cmd/covdata

## Layout Topology
```text
cmd/covdata/
├── argsmerge.go
├── covdata.go
├── doc.go
├── dump.go
├── merge.go
├── metamerge.go
└── subtractintersect.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/covdata/argsmerge.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"slices"
	"strconv"
)

type argvalues struct {
	osargs []string
	goos   string
	goarch string
}

type argstate struct {
	state       argvalues
	initialized bool
}

func (a *argstate) Merge(state argvalues) {
	if !a.initialized {
		a.state = state
		a.initialized = true
		return
	}
	if !slices.Equal(a.state.osargs, state.osargs) {
		a.state.osargs = nil
	}
	if state.goos != a.state.goos {
		a.state.goos = ""
	}
	if state.goarch != a.state.goarch {
		a.state.goarch = ""
	}
}

func (a *argstate) ArgsSummary() map[string]string {
	m := make(map[string]string)
	if len(a.state.osargs) != 0 {
		m["argc"] = strconv.Itoa(len(a.state.osargs))
		for k, a := range a.state.osargs {
			m[fmt.Sprintf("argv%d", k)] = a
		}
	}
	if a.state.goos != "" {
		m["GOOS"] = a.state.goos
	}
	if a.state.goarch != "" {
		m["GOARCH"] = a.state.goarch
	}
	return m
}

```

// === FILE: references/go/src/cmd/covdata/covdata.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"cmd/internal/cov"
	"cmd/internal/pkgpattern"
	"cmd/internal/telemetry/counter"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
)

var verbflag = flag.Int("v", 0, "Verbose trace output level")
var hflag = flag.Bool("h", false, "Panic on fatal errors (for stack trace)")
var hwflag = flag.Bool("hw", false, "Panic on warnings (for stack trace)")
var indirsflag = flag.String("i", "", "Input dirs to examine (comma separated)")
var pkgpatflag = flag.String("pkg", "", "Restrict output to package(s) matching specified package pattern.")
var cpuprofileflag = flag.String("cpuprofile", "", "Write CPU profile to specified file")
var memprofileflag = flag.String("memprofile", "", "Write memory profile to specified file")
var memprofilerateflag = flag.Int("memprofilerate", 0, "Set memprofile sampling rate to value")

var matchpkg func(name string) bool

var atExitFuncs []func()

func atExit(f func()) {
	atExitFuncs = append(atExitFuncs, f)
}

func Exit(code int) {
	for i := len(atExitFuncs) - 1; i >= 0; i-- {
		f := atExitFuncs[i]
		atExitFuncs = atExitFuncs[:i]
		f()
	}
	os.Exit(code)
}

func dbgtrace(vlevel int, s string, a ...any) {
	if *verbflag >= vlevel {
		fmt.Printf(s, a...)
		fmt.Printf("\n")
	}
}

func warn(s string, a ...any) {
	fmt.Fprintf(os.Stderr, "warning: ")
	fmt.Fprintf(os.Stderr, s, a...)
	fmt.Fprintf(os.Stderr, "\n")
	if *hwflag {
		panic("unexpected warning")
	}
}

func fatal(s string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: ")
	fmt.Fprintf(os.Stderr, s, a...)
	fmt.Fprintf(os.Stderr, "\n")
	if *hflag {
		panic("fatal error")
	}
	Exit(1)
}

func usage(msg string) {
	if len(msg) > 0 {
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	}
	fmt.Fprintf(os.Stderr, "usage: go tool covdata [command]\n")
	fmt.Fprintf(os.Stderr, `
Commands are:

textfmt     convert coverage data to textual format
percent     output total percentage of statements covered
pkglist     output list of package import paths
func        output coverage profile information for each function
merge       merge data files together
subtract    subtract one set of data files from another set
intersect   generate intersection of two sets of data files
debugdump   dump data in human-readable format for debugging purposes
`)
	fmt.Fprintf(os.Stderr, "\nFor help on a specific subcommand, try:\n")
	fmt.Fprintf(os.Stderr, "\ngo tool covdata <cmd> -help\n")
	Exit(2)
}

type covOperation interface {
	cov.CovDataVisitor
	Setup()
	Usage(string)
}

// Modes of operation.
const (
	funcMode      = "func"
	mergeMode     = "merge"
	intersectMode = "intersect"
	subtractMode  = "subtract"
	percentMode   = "percent"
	pkglistMode   = "pkglist"
	textfmtMode   = "textfmt"
	debugDumpMode = "debugdump"
)

func main() {
	counter.Open()

	// First argument should be mode/subcommand.
	if len(os.Args) < 2 {
		usage("missing command selector")
	}

	// Select mode
	var op covOperation
	cmd := os.Args[1]
	switch cmd {
	case mergeMode:
		op = makeMergeOp()
	case debugDumpMode:
		op = makeDumpOp(debugDumpMode)
	case textfmtMode:
		op = makeDumpOp(textfmtMode)
	case percentMode:
		op = makeDumpOp(percentMode)
	case funcMode:
		op = makeDumpOp(funcMode)
	case pkglistMode:
		op = makeDumpOp(pkglistMode)
	case subtractMode:
		op = makeSubtractIntersectOp(subtractMode)
	case intersectMode:
		op = makeSubtractIntersectOp(intersectMode)
	default:
		usage(fmt.Sprintf("unknown command selector %q", cmd))
	}

	// Edit out command selector, then parse flags.
	os.Args = append(os.Args[:1], os.Args[2:]...)
	flag.Usage = func() {
		op.Usage("")
	}
	flag.Parse()
	counter.Inc("covdata/invocations")
	counter.CountFlags("covdata/flag:", *flag.CommandLine)

	// Mode-independent flag setup
	dbgtrace(1, "starting mode-independent setup")
	if flag.NArg() != 0 {
		op.Usage("unknown extra arguments")
	}
	if *pkgpatflag != "" {
		pats := strings.Split(*pkgpatflag, ",")
		matchers := []func(name string) bool{}
		for _, p := range pats {
			if p == "" {
				continue
			}
			f := pkgpattern.MatchSimplePattern(p)
			matchers = append(matchers, f)
		}
		matchpkg = func(name string) bool {
			for _, f := range matchers {
				if f(name) {
					return true
				}
			}
			return false
		}
	}
	if *cpuprofileflag != "" {
		f, err := os.Create(*cpuprofileflag)
		if err != nil {
			fatal("%v", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			fatal("%v", err)
		}
		atExit(func() {
			pprof.StopCPUProfile()
			if err = f.Close(); err != nil {
				fatal("error closing cpu profile: %v", err)
			}
		})
	}
	if *memprofileflag != "" {
		if *memprofilerateflag != 0 {
			runtime.MemProfileRate = *memprofilerateflag
		}
		f, err := os.Create(*memprofileflag)
		if err != nil {
			fatal("%v", err)
		}
		atExit(func() {
			runtime.GC()
			const writeLegacyFormat = 1
			if err := pprof.Lookup("heap").WriteTo(f, writeLegacyFormat); err != nil {
				fatal("%v", err)
			}
			if err = f.Close(); err != nil {
				fatal("error closing memory profile: %v", err)
			}
		})
	} else {
		// Not doing memory profiling; disable it entirely.
		runtime.MemProfileRate = 0
	}

	// Mode-dependent setup.
	op.Setup()

	// ... off and running now.
	dbgtrace(1, "starting perform")

	indirs := strings.Split(*indirsflag, ",")
	vis := cov.CovDataVisitor(op)
	var flags cov.CovDataReaderFlags
	if *hflag {
		flags |= cov.PanicOnError
	}
	if *hwflag {
		flags |= cov.PanicOnWarning
	}
	reader := cov.MakeCovDataReader(vis, indirs, *verbflag, flags, matchpkg)
	st := 0
	if err := reader.Visit(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		st = 1
	}
	dbgtrace(1, "leaving main")
	Exit(st)
}

```

// === FILE: references/go/src/cmd/covdata/doc.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Covdata is a program for manipulating and generating reports
from 2nd-generation coverage testing output files, those produced
from running applications or integration tests. E.g.

	$ mkdir ./profiledir
	$ go build -cover -o myapp.exe .
	$ GOCOVERDIR=./profiledir ./myapp.exe <arguments>
	$ ls ./profiledir
	covcounters.cce1b350af34b6d0fb59cc1725f0ee27.821598.1663006712821344241
	covmeta.cce1b350af34b6d0fb59cc1725f0ee27
	$

Run covdata via "go tool covdata <mode>", where 'mode' is a subcommand
selecting a specific reporting, merging, or data manipulation operation.
Descriptions on the various modes (run "go tool cover <mode> -help" for
specifics on usage of a given mode):

1. Report percent of statements covered in each profiled package

	$ go tool covdata percent -i=profiledir
	cov-example/p	coverage: 41.1% of statements
	main	coverage: 87.5% of statements
	$

2. Report import paths of packages profiled

	$ go tool covdata pkglist -i=profiledir
	cov-example/p
	main
	$

3. Report percent statements covered by function:

	$ go tool covdata func -i=profiledir
	cov-example/p/p.go:12:		emptyFn			0.0%
	cov-example/p/p.go:32:		Small			100.0%
	cov-example/p/p.go:47:		Medium			90.9%
	...
	$

4. Convert coverage data to legacy textual format:

	$ go tool covdata textfmt -i=profiledir -o=cov.txt
	$ head cov.txt
	mode: set
	cov-example/p/p.go:12.22,13.2 0 0
	cov-example/p/p.go:15.31,16.2 1 0
	cov-example/p/p.go:16.3,18.3 0 0
	cov-example/p/p.go:19.3,21.3 0 0
	...
	$ go tool cover -html=cov.txt
	$

5. Merge profiles together:

	$ go tool covdata merge -i=indir1,indir2 -o=outdir -modpaths=github.com/go-delve/delve
	$

6. Subtract one profile from another

	$ go tool covdata subtract -i=indir1,indir2 -o=outdir
	$

7. Intersect profiles

	$ go tool covdata intersect -i=indir1,indir2 -o=outdir
	$

8. Dump a profile for debugging purposes.

	$ go tool covdata debugdump -i=indir
	<human readable output>
	$
*/
package main

```

// === FILE: references/go/src/cmd/covdata/dump.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// This file contains functions and apis to support the "go tool
// covdata" sub-commands that relate to dumping text format summaries
// and reports: "pkglist", "func",  "debugdump", "percent", and
// "textfmt".

import (
	"flag"
	"fmt"
	"internal/coverage"
	"internal/coverage/calloc"
	"internal/coverage/cformat"
	"internal/coverage/cmerge"
	"internal/coverage/decodecounter"
	"internal/coverage/decodemeta"
	"internal/coverage/pods"
	"os"
	"sort"
	"strings"
)

var textfmtoutflag *string
var liveflag *bool

func makeDumpOp(cmd string) covOperation {
	if cmd == textfmtMode || cmd == percentMode {
		textfmtoutflag = flag.String("o", "", "Output text format to file")
	}
	if cmd == debugDumpMode {
		liveflag = flag.Bool("live", false, "Select only live (executed) functions for dump output.")
	}
	d := &dstate{
		cmd: cmd,
		cm:  &cmerge.Merger{},
	}
	// For these modes (percent, pkglist, func, etc), use a relaxed
	// policy when it comes to counter mode clashes. For a percent
	// report, for example, we only care whether a given line is
	// executed at least once, so it's ok to (effectively) merge
	// together runs derived from different counter modes.
	if d.cmd == percentMode || d.cmd == funcMode || d.cmd == pkglistMode {
		d.cm.SetModeMergePolicy(cmerge.ModeMergeRelaxed)
	}
	if d.cmd == pkglistMode {
		d.pkgpaths = make(map[string]struct{})
	}
	return d
}

// dstate encapsulates state and provides methods for implementing
// various dump operations. Specifically, dstate implements the
// CovDataVisitor interface, and is designed to be used in
// concert with the CovDataReader utility, which abstracts away most
// of the grubby details of reading coverage data files.
type dstate struct {
	// for batch allocation of counter arrays
	calloc.BatchCounterAlloc

	// counter merging state + methods
	cm *cmerge.Merger

	// counter data formatting helper
	format *cformat.Formatter

	// 'mm' stores values read from a counter data file; the pkfunc key
	// is a pkgid/funcid pair that uniquely identifies a function in
	// instrumented application.
	mm map[pkfunc]decodecounter.FuncPayload

	// pkm maps package ID to the number of functions in the package
	// with that ID. It is used to report inconsistencies in counter
	// data (for example, a counter data entry with pkgid=N funcid=10
	// where package N only has 3 functions).
	pkm map[uint32]uint32

	// pkgpaths records all package import paths encountered while
	// visiting coverage data files (used to implement the "pkglist"
	// subcommand).
	pkgpaths map[string]struct{}

	// Current package name and import path.
	pkgName       string
	pkgImportPath string

	// Module path for current package (may be empty).
	modulePath string

	// Dump subcommand (ex: "textfmt", "debugdump", etc).
	cmd string

	// File to which we will write text format output, if enabled.
	textfmtoutf *os.File

	// Total and covered statements (used by "debugdump" subcommand).
	totalStmts, coveredStmts int

	// Records whether preamble has been emitted for current pkg
	// (used when in "debugdump" mode)
	preambleEmitted bool
}

func (d *dstate) Usage(msg string) {
	if len(msg) > 0 {
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	}
	fmt.Fprintf(os.Stderr, "usage: go tool covdata %s -i=<directories>\n\n", d.cmd)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n\n")
	switch d.cmd {
	case pkglistMode:
		fmt.Fprintf(os.Stderr, "  go tool covdata pkglist -i=dir1,dir2\n\n")
		fmt.Fprintf(os.Stderr, "  \treads coverage data files from dir1+dirs2\n")
		fmt.Fprintf(os.Stderr, "  \tand writes out a list of the import paths\n")
		fmt.Fprintf(os.Stderr, "  \tof all compiled packages.\n")
	case textfmtMode:
		fmt.Fprintf(os.Stderr, "  go tool covdata textfmt -i=dir1,dir2 -o=out.txt\n\n")
		fmt.Fprintf(os.Stderr, "  \tmerges data from input directories dir1+dir2\n")
		fmt.Fprintf(os.Stderr, "  \tand emits text format into file 'out.txt'\n")
	case percentMode:
		fmt.Fprintf(os.Stderr, "  go tool covdata percent -i=dir1,dir2\n\n")
		fmt.Fprintf(os.Stderr, "  \tmerges data from input directories dir1+dir2\n")
		fmt.Fprintf(os.Stderr, "  \tand emits percentage of statements covered\n\n")
	case funcMode:
		fmt.Fprintf(os.Stderr, "  go tool covdata func -i=dir1,dir2\n\n")
		fmt.Fprintf(os.Stderr, "  \treads coverage data files from dir1+dirs2\n")
		fmt.Fprintf(os.Stderr, "  \tand writes out coverage profile data for\n")
		fmt.Fprintf(os.Stderr, "  \teach function.\n")
	case debugDumpMode:
		fmt.Fprintf(os.Stderr, "  go tool covdata debugdump [flags] -i=dir1,dir2\n\n")
		fmt.Fprintf(os.Stderr, "  \treads coverage data from dir1+dir2 and dumps\n")
		fmt.Fprintf(os.Stderr, "  \tcontents in human-readable form to stdout, for\n")
		fmt.Fprintf(os.Stderr, "  \tdebugging purposes.\n")
	default:
		panic("unexpected")
	}
	Exit(2)
}

// Setup is called once at program startup time to vet flag values
// and do any necessary setup operations.
func (d *dstate) Setup() {
	if *indirsflag == "" {
		d.Usage("select input directories with '-i' option")
	}
	if d.cmd == textfmtMode || (d.cmd == percentMode && *textfmtoutflag != "") {
		if *textfmtoutflag == "" {
			d.Usage("select output file name with '-o' option")
		}
		var err error
		d.textfmtoutf, err = os.Create(*textfmtoutflag)
		if err != nil {
			d.Usage(fmt.Sprintf("unable to open textfmt output file %q: %v", *textfmtoutflag, err))
		}
	}
	if d.cmd == debugDumpMode {
		fmt.Printf("/* WARNING: the format of this dump is not stable and is\n")
		fmt.Printf(" * expected to change from one Go release to the next.\n")
		fmt.Printf(" *\n")
		fmt.Printf(" * produced by:\n")
		args := append([]string{os.Args[0]}, debugDumpMode)
		args = append(args, os.Args[1:]...)
		fmt.Printf(" *\t%s\n", strings.Join(args, " "))
		fmt.Printf(" */\n")
	}
}

func (d *dstate) BeginPod(p pods.Pod) {
	d.mm = make(map[pkfunc]decodecounter.FuncPayload)
}

func (d *dstate) EndPod(p pods.Pod) {
	if d.cmd == debugDumpMode {
		d.cm.ResetModeAndGranularity()
	}
}

func (d *dstate) BeginCounterDataFile(cdf string, cdr *decodecounter.CounterDataReader, dirIdx int) {
	dbgtrace(2, "visit counter data file %s dirIdx %d", cdf, dirIdx)
	if d.cmd == debugDumpMode {
		fmt.Printf("data file %s", cdf)
		if cdr.Goos() != "" {
			fmt.Printf(" GOOS=%s", cdr.Goos())
		}
		if cdr.Goarch() != "" {
			fmt.Printf(" GOARCH=%s", cdr.Goarch())
		}
		if len(cdr.OsArgs()) != 0 {
			fmt.Printf("  program args: %+v\n", cdr.OsArgs())
		}
		fmt.Printf("\n")
	}
}

func (d *dstate) EndCounterDataFile(cdf string, cdr *decodecounter.CounterDataReader, dirIdx int) {
}

func (d *dstate) VisitFuncCounterData(data decodecounter.FuncPayload) {
	if nf, ok := d.pkm[data.PkgIdx]; !ok || data.FuncIdx > nf {
		warn("func payload inconsistency: id [p=%d,f=%d] nf=%d len(ctrs)=%d in VisitFuncCounterData, ignored", data.PkgIdx, data.FuncIdx, nf, len(data.Counters))
		return
	}
	key := pkfunc{pk: data.PkgIdx, fcn: data.FuncIdx}
	val, found := d.mm[key]

	dbgtrace(5, "ctr visit pk=%d fid=%d found=%v len(val.ctrs)=%d len(data.ctrs)=%d", data.PkgIdx, data.FuncIdx, found, len(val.Counters), len(data.Counters))

	if len(val.Counters) < len(data.Counters) {
		t := val.Counters
		val.Counters = d.AllocateCounters(len(data.Counters))
		copy(val.Counters, t)
	}
	err, overflow := d.cm.MergeCounters(val.Counters, data.Counters)
	if err != nil {
		fatal("%v", err)
	}
	if overflow {
		warn("uint32 overflow during counter merge")
	}
	d.mm[key] = val
}

func (d *dstate) EndCounters() {
}

func (d *dstate) VisitMetaDataFile(mdf string, mfr *decodemeta.CoverageMetaFileReader) {
	newgran := mfr.CounterGranularity()
	newmode := mfr.CounterMode()
	if err := d.cm.SetModeAndGranularity(mdf, newmode, newgran); err != nil {
		fatal("%v", err)
	}
	if d.cmd == debugDumpMode {
		fmt.Printf("Cover mode: %s\n", newmode.String())
		fmt.Printf("Cover granularity: %s\n", newgran.String())
	}
	if d.format == nil {
		d.format = cformat.NewFormatter(mfr.CounterMode())
	}

	// To provide an additional layer of checking when reading counter
	// data, walk the meta-data file to determine the set of legal
	// package/function combinations. This will help catch bugs in the
	// counter file reader.
	d.pkm = make(map[uint32]uint32)
	np := uint32(mfr.NumPackages())
	payload := []byte{}
	for pkIdx := uint32(0); pkIdx < np; pkIdx++ {
		var pd *decodemeta.CoverageMetaDataDecoder
		var err error
		pd, payload, err = mfr.GetPackageDecoder(pkIdx, payload)
		if err != nil {
			fatal("reading pkg %d from meta-file %s: %s", pkIdx, mdf, err)
		}
		d.pkm[pkIdx] = pd.NumFuncs()
	}
}

func (d *dstate) BeginPackage(pd *decodemeta.CoverageMetaDataDecoder, pkgIdx uint32) {
	d.preambleEmitted = false
	d.pkgImportPath = pd.PackagePath()
	d.pkgName = pd.PackageName()
	d.modulePath = pd.ModulePath()
	if d.cmd == pkglistMode {
		d.pkgpaths[d.pkgImportPath] = struct{}{}
	}
	d.format.SetPackage(pd.PackagePath())
}

func (d *dstate) EndPackage(pd *decodemeta.CoverageMetaDataDecoder, pkgIdx uint32) {
}

func (d *dstate) VisitFunc(pkgIdx uint32, fnIdx uint32, fd *coverage.FuncDesc) {
	var counters []uint32
	key := pkfunc{pk: pkgIdx, fcn: fnIdx}
	v, haveCounters := d.mm[key]

	dbgtrace(5, "meta visit pk=%d fid=%d fname=%s file=%s found=%v len(val.ctrs)=%d", pkgIdx, fnIdx, fd.Funcname, fd.Srcfile, haveCounters, len(v.Counters))

	suppressOutput := false
	if haveCounters {
		counters = v.Counters
	} else if d.cmd == debugDumpMode && *liveflag {
		suppressOutput = true
	}

	if d.cmd == debugDumpMode && !suppressOutput {
		if !d.preambleEmitted {
			fmt.Printf("\nPackage path: %s\n", d.pkgImportPath)
			fmt.Printf("Package name: %s\n", d.pkgName)
			fmt.Printf("Module path: %s\n", d.modulePath)
			d.preambleEmitted = true
		}
		fmt.Printf("\nFunc: %s\n", fd.Funcname)
		fmt.Printf("Srcfile: %s\n", fd.Srcfile)
		fmt.Printf("Literal: %v\n", fd.Lit)
	}
	for i := 0; i < len(fd.Units); i++ {
		u := fd.Units[i]
		var count uint32
		if counters != nil {
			count = counters[i]
		}
		d.format.AddUnit(fd.Srcfile, fd.Funcname, fd.Lit, u, count)
		if d.cmd == debugDumpMode && !suppressOutput {
			fmt.Printf("%d: L%d:C%d -- L%d:C%d ",
				i, u.StLine, u.StCol, u.EnLine, u.EnCol)
			if u.Parent != 0 {
				fmt.Printf("Parent:%d = %d\n", u.Parent, count)
			} else {
				fmt.Printf("NS=%d = %d\n", u.NxStmts, count)
			}
		}
		d.totalStmts += int(u.NxStmts)
		if count != 0 {
			d.coveredStmts += int(u.NxStmts)
		}
	}
}

func (d *dstate) Finish() {
	// d.format maybe nil here if the specified input dir was empty.
	if d.format != nil {
		if d.cmd == percentMode {
			d.format.EmitPercent(os.Stdout, nil, "", false, false)
		}
		if d.cmd == funcMode {
			d.format.EmitFuncs(os.Stdout)
		}
		if d.textfmtoutf != nil {
			if err := d.format.EmitTextual(nil, d.textfmtoutf); err != nil {
				fatal("writing to %s: %v", *textfmtoutflag, err)
			}
		}
	}
	if d.textfmtoutf != nil {
		if err := d.textfmtoutf.Close(); err != nil {
			fatal("closing textfmt output file %s: %v", *textfmtoutflag, err)
		}
	}
	if d.cmd == debugDumpMode {
		fmt.Printf("totalStmts: %d coveredStmts: %d\n", d.totalStmts, d.coveredStmts)
	}
	if d.cmd == pkglistMode {
		pkgs := make([]string, 0, len(d.pkgpaths))
		for p := range d.pkgpaths {
			pkgs = append(pkgs, p)
		}
		sort.Strings(pkgs)
		for _, p := range pkgs {
			fmt.Printf("%s\n", p)
		}
	}
}

```

// === FILE: references/go/src/cmd/covdata/merge.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// This file contains functions and apis to support the "merge"
// subcommand of "go tool covdata".

import (
	"flag"
	"fmt"
	"internal/coverage"
	"internal/coverage/cmerge"
	"internal/coverage/decodecounter"
	"internal/coverage/decodemeta"
	"internal/coverage/pods"
	"os"
)

var outdirflag *string
var pcombineflag *bool

func makeMergeOp() covOperation {
	outdirflag = flag.String("o", "", "Output directory to write")
	pcombineflag = flag.Bool("pcombine", false, "Combine profiles derived from distinct program executables")
	m := &mstate{
		mm: newMetaMerge(),
	}
	return m
}

// mstate encapsulates state and provides methods for implementing the
// merge operation. This type implements the CovDataVisitor interface,
// and is designed to be used in concert with the CovDataReader
// utility, which abstracts away most of the grubby details of reading
// coverage data files. Most of the heavy lifting for merging is done
// using apis from 'metaMerge' (this is mainly a wrapper around that
// functionality).
type mstate struct {
	mm *metaMerge
}

func (m *mstate) Usage(msg string) {
	if len(msg) > 0 {
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	}
	fmt.Fprintf(os.Stderr, "usage: go tool covdata merge -i=<directories> -o=<dir>\n\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n\n")
	fmt.Fprintf(os.Stderr, "  go tool covdata merge -i=dir1,dir2,dir3 -o=outdir\n\n")
	fmt.Fprintf(os.Stderr, "  \tmerges all files in dir1/dir2/dir3\n")
	fmt.Fprintf(os.Stderr, "  \tinto output dir outdir\n")
	Exit(2)
}

func (m *mstate) Setup() {
	if *indirsflag == "" {
		m.Usage("select input directories with '-i' option")
	}
	if *outdirflag == "" {
		m.Usage("select output directory with '-o' option")
	}
	m.mm.SetModeMergePolicy(cmerge.ModeMergeRelaxed)
}

func (m *mstate) BeginPod(p pods.Pod) {
	m.mm.beginPod()
}

func (m *mstate) EndPod(p pods.Pod) {
	m.mm.endPod(*pcombineflag)
}

func (m *mstate) BeginCounterDataFile(cdf string, cdr *decodecounter.CounterDataReader, dirIdx int) {
	dbgtrace(2, "visit counter data file %s dirIdx %d", cdf, dirIdx)
	m.mm.beginCounterDataFile(cdr)
}

func (m *mstate) EndCounterDataFile(cdf string, cdr *decodecounter.CounterDataReader, dirIdx int) {
}

func (m *mstate) VisitFuncCounterData(data decodecounter.FuncPayload) {
	m.mm.visitFuncCounterData(data)
}

func (m *mstate) EndCounters() {
}

func (m *mstate) VisitMetaDataFile(mdf string, mfr *decodemeta.CoverageMetaFileReader) {
	m.mm.visitMetaDataFile(mdf, mfr)
}

func (m *mstate) BeginPackage(pd *decodemeta.CoverageMetaDataDecoder, pkgIdx uint32) {
	dbgtrace(3, "VisitPackage(pk=%d path=%s)", pkgIdx, pd.PackagePath())
	m.mm.visitPackage(pd, pkgIdx, *pcombineflag)
}

func (m *mstate) EndPackage(pd *decodemeta.CoverageMetaDataDecoder, pkgIdx uint32) {
}

func (m *mstate) VisitFunc(pkgIdx uint32, fnIdx uint32, fd *coverage.FuncDesc) {
	m.mm.visitFunc(pkgIdx, fnIdx, fd, mergeMode, *pcombineflag)
}

func (m *mstate) Finish() {
	if *pcombineflag {
		finalHash := m.mm.emitMeta(*outdirflag, true)
		m.mm.emitCounters(*outdirflag, finalHash)
	}
}

```

// === FILE: references/go/src/cmd/covdata/metamerge.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// This file contains functions and apis that support merging of
// meta-data information.  It helps implement the "merge", "subtract",
// and "intersect" subcommands.

import (
	"fmt"
	"hash/fnv"
	"internal/coverage"
	"internal/coverage/calloc"
	"internal/coverage/cmerge"
	"internal/coverage/decodecounter"
	"internal/coverage/decodemeta"
	"internal/coverage/encodecounter"
	"internal/coverage/encodemeta"
	"internal/coverage/slicewriter"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
	"unsafe"
)

// metaMerge provides state and methods to help manage the process
// of selecting or merging meta data files. There are three cases
// of interest here: the "-pcombine" flag provided by merge, the
// "-pkg" option provided by all merge/subtract/intersect, and
// a regular vanilla merge with no package selection
//
// In the -pcombine case, we're essentially glomming together all the
// meta-data for all packages and all functions, meaning that
// everything we see in a given package needs to be added into the
// meta-data file builder; we emit a single meta-data file at the end
// of the run.
//
// In the -pkg case, we will typically emit a single meta-data file
// per input pod, where that new meta-data file contains entries for
// just the selected packages.
//
// In the third case (vanilla merge with no combining or package
// selection) we can carry over meta-data files without touching them
// at all (only counter data files will be merged).
type metaMerge struct {
	calloc.BatchCounterAlloc
	cmerge.Merger
	// maps package import path to package state
	pkm map[string]*pkstate
	// list of packages
	pkgs []*pkstate
	// current package state
	p *pkstate
	// current pod state
	pod *podstate
	// counter data file osargs/goos/goarch state
	astate *argstate
}

// pkstate
type pkstate struct {
	// index of package within meta-data file.
	pkgIdx uint32
	// this maps function index within the package to counter data payload
	ctab map[uint32]decodecounter.FuncPayload
	// pointer to meta-data blob for package
	mdblob []byte
	// filled in only for -pcombine merges
	*pcombinestate
}

type podstate struct {
	pmm      map[pkfunc]decodecounter.FuncPayload
	mdf      string
	mfr      *decodemeta.CoverageMetaFileReader
	fileHash [16]byte
}

type pkfunc struct {
	pk, fcn uint32
}

// pcombinestate
type pcombinestate struct {
	// Meta-data builder for the package.
	cmdb *encodemeta.CoverageMetaDataBuilder
	// Maps function meta-data hash to new function index in the
	// new version of the package we're building.
	ftab map[[16]byte]uint32
}

func newMetaMerge() *metaMerge {
	return &metaMerge{
		pkm:    make(map[string]*pkstate),
		astate: &argstate{},
	}
}

func (mm *metaMerge) visitMetaDataFile(mdf string, mfr *decodemeta.CoverageMetaFileReader) {
	dbgtrace(2, "visitMetaDataFile(mdf=%s)", mdf)

	// Record meta-data file name.
	mm.pod.mdf = mdf
	// Keep a pointer to the file-level reader.
	mm.pod.mfr = mfr
	// Record file hash.
	mm.pod.fileHash = mfr.FileHash()
	// Counter mode and granularity -- detect and record clashes here.
	newgran := mfr.CounterGranularity()
	newmode := mfr.CounterMode()
	if err := mm.SetModeAndGranularity(mdf, newmode, newgran); err != nil {
		fatal("%v", err)
	}
}

func (mm *metaMerge) beginCounterDataFile(cdr *decodecounter.CounterDataReader) {
	state := argvalues{
		osargs: cdr.OsArgs(),
		goos:   cdr.Goos(),
		goarch: cdr.Goarch(),
	}
	mm.astate.Merge(state)
}

func copyMetaDataFile(inpath, outpath string) {
	inf, err := os.Open(inpath)
	if err != nil {
		fatal("opening input meta-data file %s: %v", inpath, err)
	}
	defer inf.Close()

	fi, err := inf.Stat()
	if err != nil {
		fatal("accessing input meta-data file %s: %v", inpath, err)
	}

	outf, err := os.OpenFile(outpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode())
	if err != nil {
		fatal("opening output meta-data file %s: %v", outpath, err)
	}

	_, err = io.Copy(outf, inf)
	outf.Close()
	if err != nil {
		fatal("writing output meta-data file %s: %v", outpath, err)
	}
}

func (mm *metaMerge) beginPod() {
	mm.pod = &podstate{
		pmm: make(map[pkfunc]decodecounter.FuncPayload),
	}
}

// metaEndPod handles actions needed when we're done visiting all of
// the things in a pod -- counter files and meta-data file. There are
// three cases of interest here:
//
// Case 1: in an unconditional merge (we're not selecting a specific set of
// packages using "-pkg", and the "-pcombine" option is not in use),
// we can simply copy over the meta-data file from input to output.
//
// Case 2: if this is a select merge (-pkg is in effect), then at
// this point we write out a new smaller meta-data file that includes
// only the packages of interest. At this point we also emit a merged
// counter data file as well.
//
// Case 3: if "-pcombine" is in effect, we don't write anything at
// this point (all writes will happen at the end of the run).
func (mm *metaMerge) endPod(pcombine bool) {
	if pcombine {
		// Just clear out the pod data, we'll do all the
		// heavy lifting at the end.
		mm.pod = nil
		return
	}

	finalHash := mm.pod.fileHash
	if matchpkg != nil {
		// Emit modified meta-data file for this pod.
		finalHash = mm.emitMeta(*outdirflag, pcombine)
	} else {
		// Copy meta-data file for this pod to the output directory.
		inpath := mm.pod.mdf
		mdfbase := filepath.Base(mm.pod.mdf)
		outpath := filepath.Join(*outdirflag, mdfbase)
		copyMetaDataFile(inpath, outpath)
	}

	// Emit accumulated counter data for this pod.
	mm.emitCounters(*outdirflag, finalHash)

	// Reset package state.
	mm.pkm = make(map[string]*pkstate)
	mm.pkgs = nil
	mm.pod = nil

	// Reset counter mode and granularity
	mm.ResetModeAndGranularity()
}

// emitMeta encodes and writes out a new coverage meta-data file as
// part of a merge operation, specifically a merge with the
// "-pcombine" flag.
func (mm *metaMerge) emitMeta(outdir string, pcombine bool) [16]byte {
	fh := fnv.New128a()
	fhSum := fnv.New128a()
	blobs := [][]byte{}
	tlen := uint64(unsafe.Sizeof(coverage.MetaFileHeader{}))
	for _, p := range mm.pkgs {
		var blob []byte
		if pcombine {
			mdw := &slicewriter.WriteSeeker{}
			p.cmdb.Emit(mdw)
			blob = mdw.BytesWritten()
		} else {
			blob = p.mdblob
		}
		fhSum.Reset()
		fhSum.Write(blob)
		ph := fhSum.Sum(nil)
		blobs = append(blobs, blob)
		if _, err := fh.Write(ph[:]); err != nil {
			panic(fmt.Sprintf("internal error: md5 sum failed: %v", err))
		}
		tlen += uint64(len(blob))
	}
	var finalHash [16]byte
	fhh := fh.Sum(nil)
	copy(finalHash[:], fhh)

	// Open meta-file for writing.
	fn := fmt.Sprintf("%s.%x", coverage.MetaFilePref, finalHash)
	fpath := filepath.Join(outdir, fn)
	mf, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		fatal("unable to open output meta-data file %s: %v", fpath, err)
	}

	defer func() {
		if err := mf.Close(); err != nil {
			fatal("error closing output meta-data file %s: %v", fpath, err)
		}
	}()

	// Encode and write.
	mfw := encodemeta.NewCoverageMetaFileWriter(fpath, mf)
	err = mfw.Write(finalHash, blobs, mm.Mode(), mm.Granularity())
	if err != nil {
		fatal("error writing %s: %v\n", fpath, err)
	}
	return finalHash
}

func (mm *metaMerge) emitCounters(outdir string, metaHash [16]byte) {
	// Open output file. The file naming scheme is intended to mimic
	// that used when running a coverage-instrumented binary, for
	// consistency (however the process ID is not meaningful here, so
	// use a value of zero).
	var dummyPID int
	fn := fmt.Sprintf(coverage.CounterFileTempl, coverage.CounterFilePref, metaHash, dummyPID, time.Now().UnixNano())
	fpath := filepath.Join(outdir, fn)
	cf, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		fatal("opening counter data file %s: %v", fpath, err)
	}
	defer func() {
		if err := cf.Close(); err != nil {
			fatal("error closing output meta-data file %s: %v", fpath, err)
		}
	}()

	args := mm.astate.ArgsSummary()
	cfw := encodecounter.NewCoverageDataWriter(cf, coverage.CtrULeb128)
	if err := cfw.Write(metaHash, args, mm); err != nil {
		fatal("counter file write failed: %v", err)
	}
	mm.astate = &argstate{}
}

// VisitFuncs is used while writing the counter data files; it
// implements the 'VisitFuncs' method required by the interface
// internal/coverage/encodecounter/CounterVisitor.
func (mm *metaMerge) VisitFuncs(f encodecounter.CounterVisitorFn) error {
	if *verbflag >= 4 {
		fmt.Printf("counterVisitor invoked\n")
	}
	// For each package, for each function, construct counter
	// array and then call "f" on it.
	for pidx, p := range mm.pkgs {
		fids := make([]int, 0, len(p.ctab))
		for fid := range p.ctab {
			fids = append(fids, int(fid))
		}
		sort.Ints(fids)
		if *verbflag >= 4 {
			fmt.Printf("fids for pk=%d: %+v\n", pidx, fids)
		}
		for _, fid := range fids {
			fp := p.ctab[uint32(fid)]
			if *verbflag >= 4 {
				fmt.Printf("counter write for pk=%d fid=%d len(ctrs)=%d\n", pidx, fid, len(fp.Counters))
			}
			if err := f(uint32(pidx), uint32(fid), fp.Counters); err != nil {
				return err
			}
		}
	}
	return nil
}

func (mm *metaMerge) visitPackage(pd *decodemeta.CoverageMetaDataDecoder, pkgIdx uint32, pcombine bool) {
	p, ok := mm.pkm[pd.PackagePath()]
	if !ok {
		p = &pkstate{
			pkgIdx: uint32(len(mm.pkgs)),
		}
		mm.pkgs = append(mm.pkgs, p)
		mm.pkm[pd.PackagePath()] = p
		if pcombine {
			p.pcombinestate = new(pcombinestate)
			cmdb, err := encodemeta.NewCoverageMetaDataBuilder(pd.PackagePath(), pd.PackageName(), pd.ModulePath())
			if err != nil {
				fatal("fatal error creating meta-data builder: %v", err)
			}
			dbgtrace(2, "install new pkm entry for package %s pk=%d", pd.PackagePath(), pkgIdx)
			p.cmdb = cmdb
			p.ftab = make(map[[16]byte]uint32)
		} else {
			var err error
			p.mdblob, err = mm.pod.mfr.GetPackagePayload(pkgIdx, nil)
			if err != nil {
				fatal("error extracting package %d payload from %s: %v",
					pkgIdx, mm.pod.mdf, err)
			}
		}
		p.ctab = make(map[uint32]decodecounter.FuncPayload)
	}
	mm.p = p
}

func (mm *metaMerge) visitFuncCounterData(data decodecounter.FuncPayload) {
	key := pkfunc{pk: data.PkgIdx, fcn: data.FuncIdx}
	val := mm.pod.pmm[key]
	// FIXME: in theory either A) len(val.Counters) is zero, or B)
	// the two lengths are equal. Assert if not? Of course, we could
	// see odd stuff if there is source file skew.
	if *verbflag > 4 {
		fmt.Printf("visit pk=%d fid=%d len(counters)=%d\n", data.PkgIdx, data.FuncIdx, len(data.Counters))
	}
	if len(val.Counters) < len(data.Counters) {
		t := val.Counters
		val.Counters = mm.AllocateCounters(len(data.Counters))
		copy(val.Counters, t)
	}
	err, overflow := mm.MergeCounters(val.Counters, data.Counters)
	if err != nil {
		fatal("%v", err)
	}
	if overflow {
		warn("uint32 overflow during counter merge")
	}
	mm.pod.pmm[key] = val
}

func (mm *metaMerge) visitFunc(pkgIdx uint32, fnIdx uint32, fd *coverage.FuncDesc, verb string, pcombine bool) {
	if *verbflag >= 3 {
		fmt.Printf("visit pk=%d fid=%d func %s\n", pkgIdx, fnIdx, fd.Funcname)
	}

	var counters []uint32
	key := pkfunc{pk: pkgIdx, fcn: fnIdx}
	v, haveCounters := mm.pod.pmm[key]
	if haveCounters {
		counters = v.Counters
	}

	if pcombine {
		// If the merge is running in "combine programs" mode, then hash
		// the function and look it up in the package ftab to see if we've
		// encountered it before. If we haven't, then register it with the
		// meta-data builder.
		fnhash := encodemeta.HashFuncDesc(fd)
		gfidx, ok := mm.p.ftab[fnhash]
		if !ok {
			// We haven't seen this function before, need to add it to
			// the meta data.
			gfidx = uint32(mm.p.cmdb.AddFunc(*fd))
			mm.p.ftab[fnhash] = gfidx
			if *verbflag >= 3 {
				fmt.Printf("new meta entry for fn %s fid=%d\n", fd.Funcname, gfidx)
			}
		}
		fnIdx = gfidx
	}
	if !haveCounters {
		return
	}

	// Install counters in package ctab.
	gfp, ok := mm.p.ctab[fnIdx]
	if ok {
		if verb == "subtract" || verb == "intersect" {
			panic("should never see this for intersect/subtract")
		}
		if *verbflag >= 3 {
			fmt.Printf("counter merge for %s fidx=%d\n", fd.Funcname, fnIdx)
		}
		// Merge.
		err, overflow := mm.MergeCounters(gfp.Counters, counters)
		if err != nil {
			fatal("%v", err)
		}
		if overflow {
			warn("uint32 overflow during counter merge")
		}
		mm.p.ctab[fnIdx] = gfp
	} else {
		if *verbflag >= 3 {
			fmt.Printf("null merge for %s fidx %d\n", fd.Funcname, fnIdx)
		}
		gfp := v
		gfp.PkgIdx = mm.p.pkgIdx
		gfp.FuncIdx = fnIdx
		mm.p.ctab[fnIdx] = gfp
	}
}

```

// === FILE: references/go/src/cmd/covdata/subtractintersect.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// This file contains functions and apis to support the "subtract" and
// "intersect" subcommands of "go tool covdata".

import (
	"flag"
	"fmt"
	"internal/coverage"
	"internal/coverage/decodecounter"
	"internal/coverage/decodemeta"
	"internal/coverage/pods"
	"os"
	"strings"
)

// makeSubtractIntersectOp creates a subtract or intersect operation.
// 'mode' here must be either "subtract" or "intersect".
func makeSubtractIntersectOp(mode string) covOperation {
	outdirflag = flag.String("o", "", "Output directory to write")
	s := &sstate{
		mode:  mode,
		mm:    newMetaMerge(),
		inidx: -1,
	}
	return s
}

// sstate holds state needed to implement subtraction and intersection
// operations on code coverage data files. This type provides methods
// to implement the CovDataVisitor interface, and is designed to be
// used in concert with the CovDataReader utility, which abstracts
// away most of the grubby details of reading coverage data files.
type sstate struct {
	mm    *metaMerge
	inidx int
	mode  string
	// Used only for intersection; keyed by pkg/fn ID, it keeps track of
	// just the set of functions for which we have data in the current
	// input directory.
	imm map[pkfunc]struct{}
}

func (s *sstate) Usage(msg string) {
	if len(msg) > 0 {
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	}
	fmt.Fprintf(os.Stderr, "usage: go tool covdata %s -i=dir1,dir2 -o=<dir>\n\n", s.mode)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n\n")
	op := "from"
	if s.mode == intersectMode {
		op = "with"
	}
	fmt.Fprintf(os.Stderr, "  go tool covdata %s -i=dir1,dir2 -o=outdir\n\n", s.mode)
	fmt.Fprintf(os.Stderr, "  \t%ss dir2 %s dir1, writing result\n", s.mode, op)
	fmt.Fprintf(os.Stderr, "  \tinto output dir outdir.\n")
	os.Exit(2)
}

func (s *sstate) Setup() {
	if *indirsflag == "" {
		usage("select input directories with '-i' option")
	}
	indirs := strings.Split(*indirsflag, ",")
	if s.mode == subtractMode && len(indirs) != 2 {
		usage("supply exactly two input dirs for subtract operation")
	}
	if *outdirflag == "" {
		usage("select output directory with '-o' option")
	}
}

func (s *sstate) BeginPod(p pods.Pod) {
	s.mm.beginPod()
}

func (s *sstate) EndPod(p pods.Pod) {
	const pcombine = false
	s.mm.endPod(pcombine)
}

func (s *sstate) EndCounters() {
	if s.imm != nil {
		s.pruneCounters()
	}
}

// pruneCounters performs a function-level partial intersection using the
// current POD counter data (s.mm.pod.pmm) and the intersected data from
// PODs in previous dirs (s.imm).
func (s *sstate) pruneCounters() {
	pkeys := make([]pkfunc, 0, len(s.mm.pod.pmm))
	for k := range s.mm.pod.pmm {
		pkeys = append(pkeys, k)
	}
	// Remove anything from pmm not found in imm. We don't need to
	// go the other way (removing things from imm not found in pmm)
	// since we don't add anything to imm if there is no pmm entry.
	for _, k := range pkeys {
		if _, found := s.imm[k]; !found {
			delete(s.mm.pod.pmm, k)
		}
	}
	s.imm = nil
}

func (s *sstate) BeginCounterDataFile(cdf string, cdr *decodecounter.CounterDataReader, dirIdx int) {
	dbgtrace(2, "visiting counter data file %s diridx %d", cdf, dirIdx)
	if s.inidx != dirIdx {
		if s.inidx > dirIdx {
			// We're relying on having data files presented in
			// the order they appear in the inputs (e.g. first all
			// data files from input dir 0, then dir 1, etc).
			panic("decreasing dir index, internal error")
		}
		if dirIdx == 0 {
			// No need to keep track of the functions in the first
			// directory, since that info will be replicated in
			// s.mm.pod.pmm.
			s.imm = nil
		} else {
			// We're now starting to visit the Nth directory, N != 0.
			if s.mode == intersectMode {
				if s.imm != nil {
					s.pruneCounters()
				}
				s.imm = make(map[pkfunc]struct{})
			}
		}
		s.inidx = dirIdx
	}
}

func (s *sstate) EndCounterDataFile(cdf string, cdr *decodecounter.CounterDataReader, dirIdx int) {
}

func (s *sstate) VisitFuncCounterData(data decodecounter.FuncPayload) {
	key := pkfunc{pk: data.PkgIdx, fcn: data.FuncIdx}

	if *verbflag >= 5 {
		fmt.Printf("ctr visit fid=%d pk=%d inidx=%d data.Counters=%+v\n", data.FuncIdx, data.PkgIdx, s.inidx, data.Counters)
	}

	// If we're processing counter data from the initial (first) input
	// directory, then just install it into the counter data map
	// as usual.
	if s.inidx == 0 {
		s.mm.visitFuncCounterData(data)
		return
	}

	// If we're looking at counter data from a dir other than
	// the first, then perform the intersect/subtract.
	if val, ok := s.mm.pod.pmm[key]; ok {
		if s.mode == subtractMode {
			for i := 0; i < len(data.Counters); i++ {
				if data.Counters[i] != 0 {
					val.Counters[i] = 0
				}
			}
		} else if s.mode == intersectMode {
			s.imm[key] = struct{}{}
			for i := 0; i < len(data.Counters); i++ {
				if data.Counters[i] == 0 {
					val.Counters[i] = 0
				}
			}
		}
	}
}

func (s *sstate) VisitMetaDataFile(mdf string, mfr *decodemeta.CoverageMetaFileReader) {
	if s.mode == intersectMode {
		s.imm = make(map[pkfunc]struct{})
	}
	s.mm.visitMetaDataFile(mdf, mfr)
}

func (s *sstate) BeginPackage(pd *decodemeta.CoverageMetaDataDecoder, pkgIdx uint32) {
	s.mm.visitPackage(pd, pkgIdx, false)
}

func (s *sstate) EndPackage(pd *decodemeta.CoverageMetaDataDecoder, pkgIdx uint32) {
}

func (s *sstate) VisitFunc(pkgIdx uint32, fnIdx uint32, fd *coverage.FuncDesc) {
	s.mm.visitFunc(pkgIdx, fnIdx, fd, s.mode, false)
}

func (s *sstate) Finish() {
}

```

