# Domain Architecture: cmd/compile/internal/base

## Layout Topology
```text
cmd/compile/internal/base/
├── base.go
├── bootstrap_false.go
├── bootstrap_true.go
├── debug.go
├── flag.go
├── hashdebug.go
├── link.go
├── mapfile_mmap.go
├── mapfile_read.go
├── print.go
├── startheap.go
└── timings.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/compile/internal/base/base.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"os"
)

var atExitFuncs []func()

func AtExit(f func()) {
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

// To enable tracing support (-t flag), set EnableTrace to true.
const EnableTrace = false

```

// === FILE: references/go/src/cmd/compile/internal/base/bootstrap_false.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !compiler_bootstrap

package base

// CompilerBootstrap reports whether the current compiler binary was
// built with -tags=compiler_bootstrap.
const CompilerBootstrap = false

```

// === FILE: references/go/src/cmd/compile/internal/base/bootstrap_true.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build compiler_bootstrap

package base

// CompilerBootstrap reports whether the current compiler binary was
// built with -tags=compiler_bootstrap.
const CompilerBootstrap = true

```

// === FILE: references/go/src/cmd/compile/internal/base/debug.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Debug arguments, set by -d flag.

package base

// Debug holds the parsed debugging configuration values.
var Debug DebugFlags

// DebugFlags defines the debugging configuration values (see var Debug).
// Each struct field is a different value, named for the lower-case of the field name.
// Each field must be an int or string and must have a `help` struct tag.
// Don't forget to note that concurrency is "ok" when it is,
// else the compiler will serialize for that flag.
//
// The -d option takes a comma-separated list of settings.
// Each setting is name=value; for ints, name is short for name=1.
type DebugFlags struct {
	AlignHot              int    `help:"enable hot block alignment (currently requires -pgo)" concurrent:"ok"`
	Append                int    `help:"print information about append compilation"`
	AstDump               string `help:"for specified function/method, dump AST/IR at interesting points in compilation, to file pkg.func.ast. Use leading ~ for regular expression match."`
	Checkptr              int    `help:"instrument unsafe pointer conversions\n0: instrumentation disabled\n1: conversions involving unsafe.Pointer are instrumented\n2: conversions to unsafe.Pointer force heap allocation" concurrent:"ok"`
	Closure               int    `help:"print information about closure compilation"`
	CompressInstructions  int    `help:"use compressed instructions when possible (if supported by architecture)" concurrent:"ok"`
	Converthash           string `help:"hash value for use in debugging changes to platform-dependent float-to-[u]int conversion" concurrent:"ok"`
	Defer                 int    `help:"print information about defer compilation"`
	DisableNil            int    `help:"disable nil checks" concurrent:"ok"`
	DumpInlFuncProps      string `help:"dump function properties from inl heuristics to specified file"`
	DumpInlCallSiteScores int    `help:"dump scored callsites during inlining"`
	InlScoreAdj           string `help:"set inliner score adjustments (ex: -d=inlscoreadj=panicPathAdj:10/passConstToNestedIfAdj:-90)" concurrent:"ok"`
	InlBudgetSlack        int    `help:"amount to expand the initial inline budget when new inliner enabled. Defaults to 80 if option not set." concurrent:"ok"`
	DumpPtrs              int    `help:"show Node pointers values in dump output"`
	DwarfInl              int    `help:"print information about DWARF inlined function creation"`
	EscapeMutationsCalls  int    `help:"print extra escape analysis diagnostics about mutations and calls" concurrent:"ok"`
	EscapeDebug           int    `help:"print information about escape analysis and resulting optimizations" concurrent:"ok"`
	EscapeAlias           int    `help:"print information about alias analysis" concurrent:"ok"`
	EscapeAliasCheck      int    `help:"enable additional validation for alias analysis" concurrent:"ok"`
	Export                int    `help:"print export data"`
	FIPSHash              string `help:"hash value for FIPS debugging" concurrent:"ok"`
	Fmahash               string `help:"hash value for use in debugging platform-dependent multiply-add use" concurrent:"ok"`
	FreeAppend            int    `help:"insert frees of append results when proven safe (0 disabled, 1 enabled, 2 enabled + log)" concurrent:"ok"`
	GCAdjust              int    `help:"log adjustments to GOGC" concurrent:"ok"`
	GCCheck               int    `help:"check heap/gc use by compiler" concurrent:"ok"`
	GCProg                int    `help:"print dump of GC programs"`
	GCStart               int    `help:"specify \"starting\" compiler's heap size in MiB" concurrent:"ok"`
	Gossahash             string `help:"hash value for use in debugging the compiler"`
	InlFuncsWithClosures  int    `help:"allow functions with closures to be inlined" concurrent:"ok"`
	InlStaticInit         int    `help:"allow static initialization of inlined calls" concurrent:"ok"`
	InterfaceCycles       int    `help:"allow anonymous interface cycles" concurrent:"ok"`
	Libfuzzer             int    `help:"enable coverage instrumentation for libfuzzer"`
	LiteralAllocHash      string `help:"hash value for use in debugging literal allocation optimizations" concurrent:"ok"`
	LoopVar               int    `help:"shared (0), 1 (private loop variables, default), 2, private + log" concurrent:"ok"`
	LoopVarHash           string `help:"for debugging changes in loop behavior. Overrides experiment and loopvar flag." concurrent:"ok"`
	LocationLists         int    `help:"print information about DWARF location list creation"`
	MaxShapeLen           int    `help:"hash shape names longer than this threshold (default 500)" concurrent:"ok"`
	MergeLocals           int    `help:"merge together non-interfering local stack slots" concurrent:"ok"`
	MergeLocalsDumpFunc   string `help:"dump specified func in merge locals"`
	MergeLocalsHash       string `help:"hash value for debugging stack slot merging of local variables" concurrent:"ok"`
	MergeLocalsTrace      int    `help:"trace debug output for locals merging"`
	MergeLocalsHTrace     int    `help:"hash-selected trace debug output for locals merging"`
	Nil                   int    `help:"print information about nil checks"`
	NoDeadLocals          int    `help:"disable deadlocals pass" concurrent:"ok"`
	NoOpenDefer           int    `help:"disable open-coded defers" concurrent:"ok"`
	NoRefName             int    `help:"do not include referenced symbol names in object file" concurrent:"ok"`
	PCTab                 string `help:"print named pc-value table\nOne of: pctospadj, pctofile, pctoline, pctoinline, pctopcdata"`
	Panic                 int    `help:"show all compiler panics"`
	Reshape               int    `help:"print information about expression reshaping"`
	Shapify               int    `help:"print information about shaping recursive types"`
	Simd                  int    `help:"print information about simd analysis and code transformation" concurrent:"ok"`
	Slice                 int    `help:"print information about slice compilation"`
	SoftFloat             int    `help:"force compiler to emit soft-float code" concurrent:"ok"`
	StaticCopy            int    `help:"print information about missed static copies" concurrent:"ok"`
	SyncFrames            int    `help:"how many writer stack frames to include at sync points in unified export data" concurrent:"ok"`
	TailCall              int    `help:"print information about tail calls"`
	TypeAssert            int    `help:"print information about type assertion inlining"`
	WB                    int    `help:"print information about write barriers"`
	ABIWrap               int    `help:"print information about ABI wrapper generation"`
	MayMoreStack          string `help:"call named function before all stack growth checks" concurrent:"ok"`
	PGODebug              int    `help:"debug profile-guided optimizations"`
	PGOHash               string `help:"hash value for debugging profile-guided optimizations" concurrent:"ok"`
	PGOInline             int    `help:"enable profile-guided inlining" concurrent:"ok"`
	PGOInlineCDFThreshold string `help:"cumulative threshold percentage for determining call sites as hot candidates for inlining" concurrent:"ok"`
	PGOInlineBudget       int    `help:"inline budget for hot functions" concurrent:"ok"`
	PGODevirtualize       int    `help:"enable profile-guided devirtualization; 0 to disable, 1 to enable interface devirtualization, 2 to enable function devirtualization" concurrent:"ok"`
	RangeFuncCheck        int    `help:"insert code to check behavior of range iterator functions" concurrent:"ok"`
	VariableMakeHash      string `help:"hash value for debugging stack allocation of variable-sized make results" concurrent:"ok"`
	VariableMakeThreshold int    `help:"threshold in bytes for possible stack allocation of variable-sized make results" concurrent:"ok"`
	WrapGlobalMapDbg      int    `help:"debug trace output for global map init wrapping"`
	WrapGlobalMapCtl      int    `help:"global map init wrap control (0 => default, 1 => off, 2 => stress mode, no size cutoff)" concurrent:"ok"`
	ZeroCopy              int    `help:"enable zero-copy string->[]byte conversions" concurrent:"ok"`

	ConcurrentOk bool // true if only concurrentOk flags seen
}

// DebugSSA is called to set a -d ssa/... option.
// If nil, those options are reported as invalid options.
// If DebugSSA returns a non-empty string, that text is reported as a compiler error.
var DebugSSA func(phase, flag string, val int, valString string) string

```

// === FILE: references/go/src/cmd/compile/internal/base/flag.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"cmd/internal/cov/covcmd"
	"cmd/internal/telemetry/counter"
	"encoding/json"
	"flag"
	"fmt"
	"internal/buildcfg"
	"internal/platform"
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"

	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/sys"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: compile [options] file.go...\n")
	objabi.Flagprint(os.Stderr)
	Exit(2)
}

// Flag holds the parsed command-line flags.
// See ParseFlag for non-zero defaults.
var Flag CmdFlags

// A CountFlag is a counting integer flag.
// It accepts -name=value to set the value directly,
// but it also accepts -name with no =value to increment the count.
type CountFlag int

// CmdFlags defines the command-line flags (see var Flag).
// Each struct field is a different flag, by default named for the lower-case of the field name.
// If the flag name is a single letter, the default flag name is left upper-case.
// If the flag name is "Lower" followed by a single letter, the default flag name is the lower-case of the last letter.
//
// If this default flag name can't be made right, the `flag` struct tag can be used to replace it,
// but this should be done only in exceptional circumstances: it helps everyone if the flag name
// is obvious from the field name when the flag is used elsewhere in the compiler sources.
// The `flag:"-"` struct tag makes a field invisible to the flag logic and should also be used sparingly.
//
// Each field must have a `help` struct tag giving the flag help message.
//
// The allowed field types are bool, int, string, pointers to those (for values stored elsewhere),
// CountFlag (for a counting flag), and func(string) (for a flag that uses special code for parsing).
type CmdFlags struct {
	// Single letters
	B CountFlag    "help:\"disable bounds checking\""
	C CountFlag    "help:\"disable printing of columns in error messages\""
	D string       "help:\"set relative `path` for local imports\""
	E CountFlag    "help:\"debug symbol export\""
	I func(string) "help:\"add `directory` to import search path\""
	K CountFlag    "help:\"debug missing line numbers\""
	L CountFlag    "help:\"also show actual source file names in error messages for positions affected by //line directives\""
	N CountFlag    "help:\"disable optimizations\""
	S CountFlag    "help:\"print assembly listing\""
	// V is added by objabi.AddVersionFlag
	W CountFlag "help:\"debug parse tree after type checking\""

	LowerC int        "help:\"concurrency during compilation (1 means no concurrency)\""
	LowerD flag.Value "help:\"enable debugging settings; try -d help\""
	LowerE CountFlag  "help:\"no limit on number of errors reported\""
	LowerH CountFlag  "help:\"halt on error\""
	LowerJ CountFlag  "help:\"debug runtime-initialized variables\""
	LowerL CountFlag  "help:\"disable inlining\""
	LowerM CountFlag  "help:\"print optimization decisions\""
	LowerO string     "help:\"write output to `file`\""
	LowerP *string    "help:\"set expected package import `path`\"" // &Ctxt.Pkgpath, set below
	LowerR CountFlag  "help:\"debug generated wrappers\""
	LowerT bool       "help:\"enable tracing for debugging the compiler\""
	LowerW CountFlag  "help:\"debug type checking\""
	LowerU CountFlag  "help:\"emit unsorted warnings/errors\""
	LowerV *bool      "help:\"increase debug verbosity\""

	// Special characters
	Percent          CountFlag "flag:\"%\" help:\"debug non-static initializers\""
	CompilingRuntime bool      "flag:\"+\" help:\"compiling runtime\""

	// Longer names
	AsmHdr             string       "help:\"write assembly header to `file`\""
	ASan               bool         "help:\"build code compatible with C/C++ address sanitizer\""
	Bench              string       "help:\"append benchmark times to `file`\""
	BlockProfile       string       "help:\"write block profile to `file`\""
	BuildID            string       "help:\"record `id` as the build id in the export metadata\""
	CPUProfile         string       "help:\"write cpu profile to `file`\""
	Complete           bool         "help:\"compiling complete package (no C or assembly)\""
	ClobberDead        bool         "help:\"clobber dead stack slots (for debugging)\""
	ClobberDeadReg     bool         "help:\"clobber dead registers (for debugging)\""
	Dwarf              bool         "help:\"generate DWARF symbols\""
	DwarfBASEntries    *bool        "help:\"use base address selection entries in DWARF\""                        // &Ctxt.UseBASEntries, set below
	DwarfLocationLists *bool        "help:\"add location lists to DWARF in optimized mode\""                      // &Ctxt.Flag_locationlists, set below
	Dynlink            *bool        "help:\"support references to Go symbols defined in other shared libraries\"" // &Ctxt.Flag_dynlink, set below
	EmbedCfg           func(string) "help:\"read go:embed configuration from `file`\""
	Env                func(string) "help:\"add `definition` of the form key=value to environment\""
	GenDwarfInl        int          "help:\"generate DWARF inline info records\"" // 0=disabled, 1=funcs, 2=funcs+formals/locals
	GoVersion          string       "help:\"required version of the runtime\""
	ImportCfg          func(string) "help:\"read import configuration from `file`\""
	InstallSuffix      string       "help:\"set pkg directory `suffix`\""
	JSON               string       "help:\"version,file for JSON compiler/optimizer detail output\""
	Lang               string       "help:\"Go language version source code expects\""
	LinkObj            string       "help:\"write linker-specific object to `file`\""
	LinkShared         *bool        "help:\"generate code that will be linked against Go shared libraries\"" // &Ctxt.Flag_linkshared, set below
	Live               CountFlag    "help:\"debug liveness analysis\""
	MSan               bool         "help:\"build code compatible with C/C++ memory sanitizer\""
	MemProfile         string       "help:\"write memory profile to `file`\""
	MemProfileRate     int          "help:\"set runtime.MemProfileRate to `rate`\""
	MutexProfile       string       "help:\"write mutex profile to `file`\""
	NoLocalImports     bool         "help:\"reject local (relative) imports\""
	CoverageCfg        func(string) "help:\"read coverage configuration from `file`\""
	Pack               bool         "help:\"write to file.a instead of file.o\""
	Race               bool         "help:\"enable race detector\""
	Shared             *bool        "help:\"generate code that can be linked into a shared library\"" // &Ctxt.Flag_shared, set below
	SmallFrames        bool         "help:\"reduce the size limit for stack allocated objects\""      // small stacks, to diagnose GC latency; see golang.org/issue/27732
	Spectre            string       "help:\"enable spectre mitigations in `list` (all, index, ret)\""
	Std                bool         "help:\"compiling standard library\""
	SymABIs            string       "help:\"read symbol ABIs from `file`\""
	TraceProfile       string       "help:\"write an execution trace to `file`\""
	TrimPath           string       "help:\"remove `prefix` from recorded source file paths\""
	WB                 bool         "help:\"enable write barrier\"" // TODO: remove
	PgoProfile         string       "help:\"read profile or pre-process profile from `file`\""
	ErrorURL           bool         "help:\"print explanatory URL with error message if applicable\""

	// Configuration derived from flags; not a flag itself.
	Cfg struct {
		Embed struct { // set by -embedcfg
			Patterns map[string][]string
			Files    map[string]string
		}
		ImportDirs   []string                 // appended to by -I
		ImportMap    map[string]string        // set by -importcfg
		PackageFile  map[string]string        // set by -importcfg; nil means not in use
		CoverageInfo *covcmd.CoverFixupConfig // set by -coveragecfg
		SpectreIndex bool                     // set by -spectre=index or -spectre=all
		// Whether we are adding any sort of code instrumentation, such as
		// when the race detector is enabled.
		Instrumenting bool
	}
}

func addEnv(s string) {
	i := strings.Index(s, "=")
	if i < 0 {
		log.Fatal("-env argument must be of the form key=value")
	}
	os.Setenv(s[:i], s[i+1:])
}

// ParseFlags parses the command-line flags into Flag.
func ParseFlags() {
	Flag.I = addImportDir

	Flag.LowerC = runtime.GOMAXPROCS(0)
	Flag.LowerD = objabi.NewDebugFlag(&Debug, DebugSSA)
	Flag.LowerP = &Ctxt.Pkgpath
	Flag.LowerV = &Ctxt.Debugvlog

	Flag.Dwarf = buildcfg.GOARCH != "wasm"
	Flag.DwarfBASEntries = &Ctxt.UseBASEntries
	Flag.DwarfLocationLists = &Ctxt.Flag_locationlists
	*Flag.DwarfLocationLists = true
	Flag.Dynlink = &Ctxt.Flag_dynlink
	Flag.EmbedCfg = readEmbedCfg
	Flag.Env = addEnv
	Flag.GenDwarfInl = 2
	Flag.ImportCfg = readImportCfg
	Flag.CoverageCfg = readCoverageCfg
	Flag.LinkShared = &Ctxt.Flag_linkshared
	Flag.Shared = &Ctxt.Flag_shared
	Flag.WB = true

	Debug.ConcurrentOk = true
	Debug.CompressInstructions = 1
	Debug.MaxShapeLen = 500
	Debug.AlignHot = 1
	Debug.InlFuncsWithClosures = 1
	Debug.InlStaticInit = 1
	Debug.FreeAppend = 1
	Debug.PGOInline = 1
	Debug.PGODevirtualize = 2
	Debug.SyncFrames = -1            // disable sync markers by default
	Debug.VariableMakeThreshold = 32 // 32 byte default for stack allocated make results
	Debug.ZeroCopy = 1
	Debug.RangeFuncCheck = 1
	Debug.MergeLocals = 1

	Debug.Checkptr = -1 // so we can tell whether it is set explicitly

	Flag.Cfg.ImportMap = make(map[string]string)

	objabi.AddVersionFlag() // -V
	registerFlags()
	objabi.Flagparse(usage)
	counter.CountFlags("compile/flag:", *flag.CommandLine)

	if gcd := os.Getenv("GOCOMPILEDEBUG"); gcd != "" {
		// This will only override the flags set in gcd;
		// any others set on the command line remain set.
		Flag.LowerD.Set(gcd)
	}

	if Debug.Gossahash != "" {
		hashDebug = NewHashDebug("gossahash", Debug.Gossahash, nil)
	}
	obj.SetFIPSDebugHash(Debug.FIPSHash)

	// Compute whether we're compiling the runtime from the package path. Test
	// code can also use the flag to set this explicitly.
	if Flag.Std && objabi.LookupPkgSpecial(Ctxt.Pkgpath).Runtime {
		Flag.CompilingRuntime = true
	}

	Ctxt.Std = Flag.Std

	// Three inputs govern loop iteration variable rewriting, hash, experiment, flag.
	// The loop variable rewriting is:
	// IF non-empty hash, then hash determines behavior (function+line match) (*)
	// ELSE IF experiment and flag==0, then experiment (set flag=1)
	// ELSE flag (note that build sets flag per-package), with behaviors:
	//  -1 => no change to behavior.
	//   0 => no change to behavior (unless non-empty hash, see above)
	//   1 => apply change to likely-iteration-variable-escaping loops
	//   2 => apply change, log results
	//   11 => apply change EVERYWHERE, do not log results (for debugging/benchmarking)
	//   12 => apply change EVERYWHERE, log results (for debugging/benchmarking)
	//
	// The expected uses of the these inputs are, in believed most-likely to least likely:
	//  GOEXPERIMENT=loopvar -- apply change to entire application
	//  -gcflags=some_package=-d=loopvar=1 -- apply change to some_package (**)
	//  -gcflags=some_package=-d=loopvar=2 -- apply change to some_package, log it
	//  GOEXPERIMENT=loopvar -gcflags=some_package=-d=loopvar=-1 -- apply change to all but one package
	//  GOCOMPILEDEBUG=loopvarhash=... -- search for failure cause
	//
	//  (*) For debugging purposes, providing loopvar flag >= 11 will expand the hash-eligible set of loops to all.
	// (**) Loop semantics, changed or not, follow code from a package when it is inlined; that is, the behavior
	//      of an application compiled with partially modified loop semantics does not depend on inlining.

	if Debug.LoopVarHash != "" {
		// This first little bit controls the inputs for debug-hash-matching.
		mostInlineOnly := true
		if strings.HasPrefix(Debug.LoopVarHash, "IL") {
			// When hash-searching on a position that is an inline site, default is to use the
			// most-inlined position only.  This makes the hash faster, plus there's no point
			// reporting a problem with all the inlining; there's only one copy of the source.
			// However, if for some reason you wanted it per-site, you can get this.  (The default
			// hash-search behavior for compiler debugging is at an inline site.)
			Debug.LoopVarHash = Debug.LoopVarHash[2:]
			mostInlineOnly = false
		}
		// end of testing trickiness
		LoopVarHash = NewHashDebug("loopvarhash", Debug.LoopVarHash, nil)
		if Debug.LoopVar < 11 { // >= 11 means all loops are rewrite-eligible
			Debug.LoopVar = 1 // 1 means those loops that syntactically escape their dcl vars are eligible.
		}
		LoopVarHash.SetInlineSuffixOnly(mostInlineOnly)
	} else if buildcfg.Experiment.LoopVar && Debug.LoopVar == 0 {
		Debug.LoopVar = 1
	}

	if Debug.Converthash != "" {
		ConvertHash = NewHashDebug("converthash", Debug.Converthash, nil)
	} else {
		// quietly disable the convert hash changes
		ConvertHash = NewHashDebug("converthash", "qn", nil)
	}
	if Debug.Fmahash != "" {
		FmaHash = NewHashDebug("fmahash", Debug.Fmahash, nil)
	}
	if Debug.PGOHash != "" {
		PGOHash = NewHashDebug("pgohash", Debug.PGOHash, nil)
	}
	if Debug.LiteralAllocHash != "" {
		LiteralAllocHash = NewHashDebug("literalalloc", Debug.LiteralAllocHash, nil)
	}

	if Debug.MergeLocalsHash != "" {
		MergeLocalsHash = NewHashDebug("mergelocals", Debug.MergeLocalsHash, nil)
	}
	if Debug.VariableMakeHash != "" {
		VariableMakeHash = NewHashDebug("variablemake", Debug.VariableMakeHash, nil)
	}

	if Flag.MSan && !platform.MSanSupported(buildcfg.GOOS, buildcfg.GOARCH) {
		log.Fatalf("%s/%s does not support -msan", buildcfg.GOOS, buildcfg.GOARCH)
	}
	if Flag.ASan && !platform.ASanSupported(buildcfg.GOOS, buildcfg.GOARCH) {
		log.Fatalf("%s/%s does not support -asan", buildcfg.GOOS, buildcfg.GOARCH)
	}
	if Flag.Race && !platform.RaceDetectorSupported(buildcfg.GOOS, buildcfg.GOARCH) {
		log.Fatalf("%s/%s does not support -race", buildcfg.GOOS, buildcfg.GOARCH)
	}
	if (*Flag.Shared || *Flag.Dynlink || *Flag.LinkShared) && !Ctxt.Arch.InFamily(sys.AMD64, sys.ARM, sys.ARM64, sys.I386, sys.Loong64, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X) {
		log.Fatalf("%s/%s does not support -shared", buildcfg.GOOS, buildcfg.GOARCH)
	}
	parseSpectre(Flag.Spectre) // left as string for RecordFlags

	Ctxt.CompressInstructions = Debug.CompressInstructions != 0
	Ctxt.Flag_shared = Ctxt.Flag_dynlink || Ctxt.Flag_shared
	Ctxt.Flag_optimize = Flag.N == 0
	Ctxt.Debugasm = int(Flag.S)
	Ctxt.Flag_maymorestack = Debug.MayMoreStack
	Ctxt.Flag_noRefName = Debug.NoRefName != 0

	if flag.NArg() < 1 {
		usage()
	}

	if Flag.GoVersion != "" && Flag.GoVersion != runtime.Version() {
		fmt.Printf("compile: version %q does not match go tool version %q\n", runtime.Version(), Flag.GoVersion)
		Exit(2)
	}

	if *Flag.LowerP == "" {
		*Flag.LowerP = obj.UnlinkablePkg
	}

	if Flag.LowerO == "" {
		p := flag.Arg(0)
		if i := strings.LastIndex(p, "/"); i >= 0 {
			p = p[i+1:]
		}
		if runtime.GOOS == "windows" {
			if i := strings.LastIndex(p, `\`); i >= 0 {
				p = p[i+1:]
			}
		}
		if i := strings.LastIndex(p, "."); i >= 0 {
			p = p[:i]
		}
		suffix := ".o"
		if Flag.Pack {
			suffix = ".a"
		}
		Flag.LowerO = p + suffix
	}
	switch {
	case Flag.Race && Flag.MSan:
		log.Fatal("cannot use both -race and -msan")
	case Flag.Race && Flag.ASan:
		log.Fatal("cannot use both -race and -asan")
	case Flag.MSan && Flag.ASan:
		log.Fatal("cannot use both -msan and -asan")
	}
	if Flag.Race || Flag.MSan || Flag.ASan {
		// -race, -msan and -asan imply -d=checkptr for now.
		if Debug.Checkptr == -1 { // if not set explicitly
			Debug.Checkptr = 1
		}
	}

	if Flag.LowerC < 1 {
		log.Fatalf("-c must be at least 1, got %d", Flag.LowerC)
	}
	if !concurrentBackendAllowed() {
		Flag.LowerC = 1
	}

	if Flag.CompilingRuntime {
		// It is not possible to build the runtime with no optimizations,
		// because the compiler cannot eliminate enough write barriers.
		Flag.N = 0
		Ctxt.Flag_optimize = true

		// Runtime can't use -d=checkptr, at least not yet.
		Debug.Checkptr = 0

		// Fuzzing the runtime isn't interesting either.
		Debug.Libfuzzer = 0
	}

	if Debug.Checkptr == -1 { // if not set explicitly
		Debug.Checkptr = 0
	}

	// set via a -d flag
	Ctxt.Debugpcln = Debug.PCTab

	// https://golang.org/issue/67502
	if buildcfg.GOOS == "plan9" && buildcfg.GOARCH == "386" {
		Debug.AlignHot = 0
	}
}

// registerFlags adds flag registrations for all the fields in Flag.
// See the comment on type CmdFlags for the rules.
func registerFlags() {
	var (
		boolType      = reflect.TypeFor[bool]()
		intType       = reflect.TypeFor[int]()
		stringType    = reflect.TypeFor[string]()
		ptrBoolType   = reflect.TypeFor[*bool]()
		ptrIntType    = reflect.TypeFor[*int]()
		ptrStringType = reflect.TypeFor[*string]()
		countType     = reflect.TypeFor[CountFlag]()
		funcType      = reflect.TypeFor[func(string)]()
	)

	v := reflect.ValueOf(&Flag).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == "Cfg" {
			continue
		}

		var name string
		if len(f.Name) == 1 {
			name = f.Name
		} else if len(f.Name) == 6 && f.Name[:5] == "Lower" && 'A' <= f.Name[5] && f.Name[5] <= 'Z' {
			name = string(rune(f.Name[5] + 'a' - 'A'))
		} else {
			name = strings.ToLower(f.Name)
		}
		if tag := f.Tag.Get("flag"); tag != "" {
			name = tag
		}

		help := f.Tag.Get("help")
		if help == "" {
			panic(fmt.Sprintf("base.Flag.%s is missing help text", f.Name))
		}

		if k := f.Type.Kind(); (k == reflect.Ptr || k == reflect.Func) && v.Field(i).IsNil() {
			panic(fmt.Sprintf("base.Flag.%s is uninitialized %v", f.Name, f.Type))
		}

		switch f.Type {
		case boolType:
			p := v.Field(i).Addr().Interface().(*bool)
			flag.BoolVar(p, name, *p, help)
		case intType:
			p := v.Field(i).Addr().Interface().(*int)
			flag.IntVar(p, name, *p, help)
		case stringType:
			p := v.Field(i).Addr().Interface().(*string)
			flag.StringVar(p, name, *p, help)
		case ptrBoolType:
			p := v.Field(i).Interface().(*bool)
			flag.BoolVar(p, name, *p, help)
		case ptrIntType:
			p := v.Field(i).Interface().(*int)
			flag.IntVar(p, name, *p, help)
		case ptrStringType:
			p := v.Field(i).Interface().(*string)
			flag.StringVar(p, name, *p, help)
		case countType:
			p := (*int)(v.Field(i).Addr().Interface().(*CountFlag))
			objabi.Flagcount(name, help, p)
		case funcType:
			f := v.Field(i).Interface().(func(string))
			objabi.Flagfn1(name, help, f)
		default:
			if val, ok := v.Field(i).Interface().(flag.Value); ok {
				flag.Var(val, name, help)
			} else {
				panic(fmt.Sprintf("base.Flag.%s has unexpected type %s", f.Name, f.Type))
			}
		}
	}
}

// concurrentFlagOk reports whether the current compiler flags
// are compatible with concurrent compilation.
func concurrentFlagOk() bool {
	// TODO(rsc): Many of these are fine. Remove them.
	return Flag.Percent == 0 &&
		Flag.E == 0 &&
		Flag.K == 0 &&
		Flag.L == 0 &&
		Flag.LowerJ == 0 &&
		Flag.LowerM == 0 &&
		Flag.LowerR == 0
}

func concurrentBackendAllowed() bool {
	if !concurrentFlagOk() {
		return false
	}

	// Debug.S by itself is ok, because all printing occurs
	// while writing the object file, and that is non-concurrent.
	// Adding Debug_vlog, however, causes Debug.S to also print
	// while flushing the plist, which happens concurrently.
	if Ctxt.Debugvlog || !Debug.ConcurrentOk || Flag.Live > 0 {
		return false
	}
	// TODO: Test and delete this condition.
	if buildcfg.Experiment.FieldTrack {
		return false
	}
	// TODO: fix races and enable the following flags
	if Ctxt.Flag_dynlink || Flag.Race {
		return false
	}
	return true
}

func addImportDir(dir string) {
	if dir != "" {
		Flag.Cfg.ImportDirs = append(Flag.Cfg.ImportDirs, dir)
	}
}

func readImportCfg(file string) {
	if Flag.Cfg.ImportMap == nil {
		Flag.Cfg.ImportMap = make(map[string]string)
	}
	Flag.Cfg.PackageFile = map[string]string{}
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("-importcfg: %v", err)
	}

	for lineNum, line := range strings.Split(string(data), "\n") {
		lineNum++ // 1-based
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		verb, args, found := strings.Cut(line, " ")
		if found {
			args = strings.TrimSpace(args)
		}
		before, after, hasEq := strings.Cut(args, "=")

		switch verb {
		default:
			log.Fatalf("%s:%d: unknown directive %q", file, lineNum, verb)
		case "importmap":
			if !hasEq || before == "" || after == "" {
				log.Fatalf(`%s:%d: invalid importmap: syntax is "importmap old=new"`, file, lineNum)
			}
			Flag.Cfg.ImportMap[before] = after
		case "packagefile":
			if !hasEq || before == "" || after == "" {
				log.Fatalf(`%s:%d: invalid packagefile: syntax is "packagefile path=filename"`, file, lineNum)
			}
			Flag.Cfg.PackageFile[before] = after
		}
	}
}

func readCoverageCfg(file string) {
	var cfg covcmd.CoverFixupConfig
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("-coveragecfg: %v", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("error reading -coveragecfg file %q: %v", file, err)
	}
	Flag.Cfg.CoverageInfo = &cfg
}

func readEmbedCfg(file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("-embedcfg: %v", err)
	}
	if err := json.Unmarshal(data, &Flag.Cfg.Embed); err != nil {
		log.Fatalf("%s: %v", file, err)
	}
	if Flag.Cfg.Embed.Patterns == nil {
		log.Fatalf("%s: invalid embedcfg: missing Patterns", file)
	}
	if Flag.Cfg.Embed.Files == nil {
		log.Fatalf("%s: invalid embedcfg: missing Files", file)
	}
}

// parseSpectre parses the spectre configuration from the string s.
func parseSpectre(s string) {
	for f := range strings.SplitSeq(s, ",") {
		f = strings.TrimSpace(f)
		switch f {
		default:
			log.Fatalf("unknown setting -spectre=%s", f)
		case "":
			// nothing
		case "all":
			Flag.Cfg.SpectreIndex = true
			Ctxt.Retpoline = true
		case "index":
			Flag.Cfg.SpectreIndex = true
		case "ret":
			Ctxt.Retpoline = true
		}
	}

	if Flag.Cfg.SpectreIndex {
		switch buildcfg.GOARCH {
		case "amd64":
			// ok
		default:
			log.Fatalf("GOARCH=%s does not support -spectre=index", buildcfg.GOARCH)
		}
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/base/hashdebug.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"bytes"
	"cmd/internal/obj"
	"cmd/internal/src"
	"fmt"
	"internal/bisect"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type hashAndMask struct {
	// a hash h matches if (h^hash)&mask == 0
	hash uint64
	mask uint64
	name string // base name, or base name + "0", "1", etc.
}

type HashDebug struct {
	mu   sync.Mutex // for logfile, posTmp, bytesTmp
	name string     // base name of the flag/variable.
	// what file (if any) receives the yes/no logging?
	// default is os.Stdout
	logfile          io.Writer
	posTmp           []src.Pos
	bytesTmp         bytes.Buffer
	matches          []hashAndMask // A hash matches if one of these matches.
	excludes         []hashAndMask // explicitly excluded hash suffixes
	bisect           *bisect.Matcher
	fileSuffixOnly   bool // for Pos hashes, remove the directory prefix.
	inlineSuffixOnly bool // for Pos hashes, remove all but the most inline position.
}

// SetInlineSuffixOnly controls whether hashing and reporting use the entire
// inline position, or just the most-inline suffix.  Compiler debugging tends
// to want the whole inlining, debugging user problems (loopvarhash, e.g.)
// typically does not need to see the entire inline tree, there is just one
// copy of the source code.
func (d *HashDebug) SetInlineSuffixOnly(b bool) *HashDebug {
	d.inlineSuffixOnly = b
	return d
}

// The default compiler-debugging HashDebug, for "-d=gossahash=..."
var hashDebug *HashDebug

var ConvertHash *HashDebug      // for debugging float-to-[u]int conversion changes
var FmaHash *HashDebug          // for debugging fused-multiply-add floating point changes
var LoopVarHash *HashDebug      // for debugging shared/private loop variable changes
var PGOHash *HashDebug          // for debugging PGO optimization decisions
var LiteralAllocHash *HashDebug // for debugging literal allocation optimizations
var MergeLocalsHash *HashDebug  // for debugging local stack slot merging changes
var VariableMakeHash *HashDebug // for debugging variable-sized make optimizations

// DebugHashMatchPkgFunc reports whether debug variable Gossahash
//
//  1. is empty (returns true; this is a special more-quickly implemented case of 4 below)
//
//  2. is "y" or "Y" (returns true)
//
//  3. is "n" or "N" (returns false)
//
//  4. does not explicitly exclude the sha1 hash of pkgAndName (see step 6)
//
//  5. is a suffix of the sha1 hash of pkgAndName (returns true)
//
//  6. OR
//     if the (non-empty) value is in the regular language
//     "(-[01]+/)+?([01]+(/[01]+)+?"
//     (exclude..)(....include...)
//     test the [01]+ exclude substrings, if any suffix-match, return false (4 above)
//     test the [01]+ include substrings, if any suffix-match, return true
//     The include substrings AFTER the first slash are numbered 0,1, etc and
//     are named fmt.Sprintf("%s%d", varname, number)
//     As an extra-special case for multiple failure search,
//     an excludes-only string ending in a slash (terminated, not separated)
//     implicitly specifies the include string "0/1", that is, match everything.
//     (Exclude strings are used for automated search for multiple failures.)
//     Clause 6 is not really intended for human use and only
//     matters for failures that require multiple triggers.
//
// Otherwise it returns false.
//
// Unless Flags.Gossahash is empty, when DebugHashMatchPkgFunc returns true the message
//
//	"%s triggered %s\n", varname, pkgAndName
//
// is printed on the file named in environment variable GSHS_LOGFILE,
// or standard out if that is empty.  "Varname" is either the name of
// the variable or the name of the substring, depending on which matched.
//
// Typical use:
//
//  1. you make a change to the compiler, say, adding a new phase
//
//  2. it is broken in some mystifying way, for example, make.bash builds a broken
//     compiler that almost works, but crashes compiling a test in run.bash.
//
//  3. add this guard to the code, which by default leaves it broken, but does not
//     run the broken new code if Flags.Gossahash is non-empty and non-matching:
//
//     if !base.DebugHashMatch(ir.PkgFuncName(fn)) {
//     return nil // early exit, do nothing
//     }
//
//  4. rebuild w/o the bad code,
//     GOCOMPILEDEBUG=gossahash=n ./all.bash
//     to verify that you put the guard in the right place with the right sense of the test.
//
//  5. use github.com/dr2chase/gossahash to search for the error:
//
//     go install github.com/dr2chase/gossahash@latest
//
//     gossahash -- <the thing that fails>
//
//     for example: GOMAXPROCS=1 gossahash -- ./all.bash
//
//  6. gossahash should return a single function whose miscompilation
//     causes the problem, and you can focus on that.
func DebugHashMatchPkgFunc(pkg, fn string) bool {
	return hashDebug.MatchPkgFunc(pkg, fn, nil)
}

func DebugHashMatchPos(pos src.XPos) bool {
	return hashDebug.MatchPos(pos, nil)
}

// HasDebugHash returns true if Flags.Gossahash is non-empty, which
// results in hashDebug being not-nil.  I.e., if !HasDebugHash(),
// there is no need to create the string for hashing and testing.
func HasDebugHash() bool {
	return hashDebug != nil
}

// TODO: Delete when we switch to bisect-only.
func toHashAndMask(s, varname string) hashAndMask {
	l := len(s)
	if l > 64 {
		s = s[l-64:]
		l = 64
	}
	m := ^(^uint64(0) << l)
	h, err := strconv.ParseUint(s, 2, 64)
	if err != nil {
		Fatalf("Could not parse %s (=%s) as a binary number", varname, s)
	}

	return hashAndMask{name: varname, hash: h, mask: m}
}

// NewHashDebug returns a new hash-debug tester for the
// environment variable ev.  If ev is not set, it returns
// nil, allowing a lightweight check for normal-case behavior.
func NewHashDebug(ev, s string, file io.Writer) *HashDebug {
	if s == "" {
		return nil
	}

	hd := &HashDebug{name: ev, logfile: file}
	if !strings.Contains(s, "/") {
		m, err := bisect.New(s)
		if err != nil {
			Fatalf("%s: %v", ev, err)
		}
		hd.bisect = m
		return hd
	}

	// TODO: Delete remainder of function when we switch to bisect-only.
	ss := strings.Split(s, "/")
	// first remove any leading exclusions; these are preceded with "-"
	i := 0
	for len(ss) > 0 {
		s := ss[0]
		if len(s) == 0 || len(s) > 0 && s[0] != '-' {
			break
		}
		ss = ss[1:]
		hd.excludes = append(hd.excludes, toHashAndMask(s[1:], fmt.Sprintf("%s%d", "HASH_EXCLUDE", i)))
		i++
	}
	// hash searches may use additional EVs with 0, 1, 2, ... suffixes.
	i = 0
	for _, s := range ss {
		if s == "" {
			if i != 0 || len(ss) > 1 && ss[1] != "" || len(ss) > 2 {
				Fatalf("Empty hash match string for %s should be first (and only) one", ev)
			}
			// Special case of should match everything.
			hd.matches = append(hd.matches, toHashAndMask("0", fmt.Sprintf("%s0", ev)))
			hd.matches = append(hd.matches, toHashAndMask("1", fmt.Sprintf("%s1", ev)))
			break
		}
		if i == 0 {
			hd.matches = append(hd.matches, toHashAndMask(s, ev))
		} else {
			hd.matches = append(hd.matches, toHashAndMask(s, fmt.Sprintf("%s%d", ev, i-1)))
		}
		i++
	}
	return hd
}

// TODO: Delete when we switch to bisect-only.
func (d *HashDebug) excluded(hash uint64) bool {
	for _, m := range d.excludes {
		if (m.hash^hash)&m.mask == 0 {
			return true
		}
	}
	return false
}

// TODO: Delete when we switch to bisect-only.
func hashString(hash uint64) string {
	hstr := ""
	if hash == 0 {
		hstr = "0"
	} else {
		for ; hash != 0; hash = hash >> 1 {
			hstr = string('0'+byte(hash&1)) + hstr
		}
	}
	if len(hstr) > 24 {
		hstr = hstr[len(hstr)-24:]
	}
	return hstr
}

// TODO: Delete when we switch to bisect-only.
func (d *HashDebug) match(hash uint64) *hashAndMask {
	for i, m := range d.matches {
		if (m.hash^hash)&m.mask == 0 {
			return &d.matches[i]
		}
	}
	return nil
}

// MatchPkgFunc returns true if either the variable used to create d is
// unset, or if its value is y, or if it is a suffix of the base-two
// representation of the hash of pkg and fn.  If the variable is not nil,
// then a true result is accompanied by stylized output to d.logfile, which
// is used for automated bug search.
func (d *HashDebug) MatchPkgFunc(pkg, fn string, note func() string) bool {
	if d == nil {
		return true
	}
	// Written this way to make inlining likely.
	return d.matchPkgFunc(pkg, fn, note)
}

func (d *HashDebug) matchPkgFunc(pkg, fn string, note func() string) bool {
	hash := bisect.Hash(pkg, fn)
	return d.matchAndLog(hash, func() string { return pkg + "." + fn }, note)
}

// MatchPos is similar to MatchPkgFunc, but for hash computation
// it uses the source position including all inlining information instead of
// package name and path.
// Note that the default answer for no environment variable (d == nil)
// is "yes", do the thing.
func (d *HashDebug) MatchPos(pos src.XPos, desc func() string) bool {
	if d == nil {
		return true
	}
	// Written this way to make inlining likely.
	return d.matchPos(Ctxt, pos, desc)
}

func (d *HashDebug) matchPos(ctxt *obj.Link, pos src.XPos, note func() string) bool {
	return d.matchPosWithInfo(ctxt, pos, nil, note)
}

func (d *HashDebug) matchPosWithInfo(ctxt *obj.Link, pos src.XPos, info any, note func() string) bool {
	hash := d.hashPos(ctxt, pos)
	if info != nil {
		hash = bisect.Hash(hash, info)
	}
	return d.matchAndLog(hash,
		func() string {
			r := d.fmtPos(ctxt, pos)
			if info != nil {
				r += fmt.Sprintf(" (%v)", info)
			}
			return r
		},
		note)
}

// MatchPosWithInfo is similar to MatchPos, but with additional information
// that is included for hash computation, so it can distinguish multiple
// matches on the same source location.
// Note that the default answer for no environment variable (d == nil)
// is "yes", do the thing.
func (d *HashDebug) MatchPosWithInfo(pos src.XPos, info any, desc func() string) bool {
	if d == nil {
		return true
	}
	// Written this way to make inlining likely.
	return d.matchPosWithInfo(Ctxt, pos, info, desc)
}

// matchAndLog is the core matcher. It reports whether the hash matches the pattern.
// If a report needs to be printed, match prints that report to the log file.
// The text func must be non-nil and should return a user-readable
// representation of what was hashed. The note func may be nil; if non-nil,
// it should return additional information to display to the user when this
// change is selected.
func (d *HashDebug) matchAndLog(hash uint64, text, note func() string) bool {
	if d.bisect != nil {
		enabled := d.bisect.ShouldEnable(hash)
		if d.bisect.ShouldPrint(hash) {
			disabled := ""
			if !enabled {
				disabled = " [DISABLED]"
			}
			var t string
			if !d.bisect.MarkerOnly() {
				t = text()
				if note != nil {
					if n := note(); n != "" {
						t += ": " + n + disabled
						disabled = ""
					}
				}
			}
			d.log(d.name, hash, strings.TrimSpace(t+disabled))
		}
		return enabled
	}

	// TODO: Delete rest of function body when we switch to bisect-only.
	if d.excluded(hash) {
		return false
	}
	if m := d.match(hash); m != nil {
		d.log(m.name, hash, text())
		return true
	}
	return false
}

// short returns the form of file name to use for d.
// The default is the full path, but fileSuffixOnly selects
// just the final path element.
func (d *HashDebug) short(name string) string {
	if d.fileSuffixOnly {
		return filepath.Base(name)
	}
	return name
}

// hashPos returns a hash of the position pos, including its entire inline stack.
// If d.inlineSuffixOnly is true, hashPos only considers the innermost (leaf) position on the inline stack.
func (d *HashDebug) hashPos(ctxt *obj.Link, pos src.XPos) uint64 {
	if d.inlineSuffixOnly {
		p := ctxt.InnermostPos(pos)
		return bisect.Hash(d.short(p.Filename()), p.Line(), p.Col())
	}
	h := bisect.Hash()
	ctxt.AllPos(pos, func(p src.Pos) {
		h = bisect.Hash(h, d.short(p.Filename()), p.Line(), p.Col())
	})
	return h
}

// fmtPos returns a textual formatting of the position pos, including its entire inline stack.
// If d.inlineSuffixOnly is true, fmtPos only considers the innermost (leaf) position on the inline stack.
func (d *HashDebug) fmtPos(ctxt *obj.Link, pos src.XPos) string {
	format := func(p src.Pos) string {
		return fmt.Sprintf("%s:%d:%d", d.short(p.Filename()), p.Line(), p.Col())
	}
	if d.inlineSuffixOnly {
		return format(ctxt.InnermostPos(pos))
	}
	var stk []string
	ctxt.AllPos(pos, func(p src.Pos) {
		stk = append(stk, format(p))
	})
	return strings.Join(stk, "; ")
}

// log prints a match with the given hash and textual formatting.
// TODO: Delete varname parameter when we switch to bisect-only.
func (d *HashDebug) log(varname string, hash uint64, text string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	file := d.logfile
	if file == nil {
		if tmpfile := os.Getenv("GSHS_LOGFILE"); tmpfile != "" {
			var err error
			file, err = os.OpenFile(tmpfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				Fatalf("could not open hash-testing logfile %s", tmpfile)
				return
			}
		}
		if file == nil {
			file = os.Stdout
		}
		d.logfile = file
	}

	// Bisect output.
	fmt.Fprintf(file, "%s %s\n", text, bisect.Marker(hash))

	// Gossahash output.
	// TODO: Delete rest of function when we switch to bisect-only.
	fmt.Fprintf(file, "%s triggered %s %s\n", varname, text, hashString(hash))
}

```

// === FILE: references/go/src/cmd/compile/internal/base/link.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"cmd/internal/obj"
)

// ReservedImports are import paths used internally for generated
// symbols by the compiler.
//
// The linker uses the magic symbol prefixes "go:" and "type:".
// Avoid potential confusion between import paths and symbols
// by rejecting these reserved imports for now. Also, people
// "can do weird things in GOPATH and we'd prefer they didn't
// do _that_ weird thing" (per rsc). See also #4257.
var ReservedImports = map[string]bool{
	"go":   true,
	"type": true,
}

var Ctxt *obj.Link

// TODO(mdempsky): These should probably be obj.Link methods.

// PkgLinksym returns the linker symbol for name within the given
// package prefix. For user packages, prefix should be the package
// path encoded with objabi.PathToPrefix.
func PkgLinksym(prefix, name string, abi obj.ABI) *obj.LSym {
	if name == "_" {
		// TODO(mdempsky): Cleanup callers and Fatalf instead.
		return linksym(prefix, "_", abi)
	}
	sep := "."
	if ReservedImports[prefix] {
		sep = ":"
	}
	return linksym(prefix, prefix+sep+name, abi)
}

// Linkname returns the linker symbol for the given name as it might
// appear within a //go:linkname directive.
func Linkname(name string, abi obj.ABI) *obj.LSym {
	return linksym("_", name, abi)
}

// linksym is an internal helper function for implementing the above
// exported APIs.
func linksym(pkg, name string, abi obj.ABI) *obj.LSym {
	return Ctxt.LookupABIInit(name, abi, func(r *obj.LSym) { r.Pkg = pkg })
}

```

// === FILE: references/go/src/cmd/compile/internal/base/mapfile_mmap.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package base

import (
	"internal/unsafeheader"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// TODO(mdempsky): Is there a higher-level abstraction that still
// works well for iimport?

// MapFile returns length bytes from the file starting at the
// specified offset as a string.
func MapFile(f *os.File, offset, length int64) (string, error) {
	// POSIX mmap: "The implementation may require that off is a
	// multiple of the page size."
	x := offset & int64(os.Getpagesize()-1)
	offset -= x
	length += x

	buf, err := syscall.Mmap(int(f.Fd()), offset, int(length), syscall.PROT_READ, syscall.MAP_SHARED)
	runtime.KeepAlive(f)
	if err != nil {
		return "", err
	}

	buf = buf[x:]
	pSlice := (*unsafeheader.Slice)(unsafe.Pointer(&buf))

	var res string
	pString := (*unsafeheader.String)(unsafe.Pointer(&res))

	pString.Data = pSlice.Data
	pString.Len = pSlice.Len

	return res, nil
}

```

// === FILE: references/go/src/cmd/compile/internal/base/mapfile_read.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !unix

package base

import (
	"io"
	"os"
)

func MapFile(f *os.File, offset, length int64) (string, error) {
	buf := make([]byte, length)
	_, err := io.ReadFull(io.NewSectionReader(f, offset, length), buf)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

```

// === FILE: references/go/src/cmd/compile/internal/base/print.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"fmt"
	"internal/buildcfg"
	"internal/types/errors"
	"os"
	"runtime/debug"
	"sort"
	"strings"

	"cmd/internal/src"
	"cmd/internal/telemetry/counter"
)

// An errorMsg is a queued error message, waiting to be printed.
type errorMsg struct {
	pos  src.XPos
	msg  string
	code errors.Code
}

// Pos is the current source position being processed,
// printed by Errorf, ErrorfLang, Fatalf, and Warnf.
var Pos src.XPos

var (
	errorMsgs       []errorMsg
	numErrors       int // number of entries in errorMsgs that are errors (as opposed to warnings)
	numSyntaxErrors int
)

// Errors returns the number of errors reported.
func Errors() int {
	return numErrors
}

// SyntaxErrors returns the number of syntax errors reported.
func SyntaxErrors() int {
	return numSyntaxErrors
}

// addErrorMsg adds a new errorMsg (which may be a warning) to errorMsgs.
func addErrorMsg(pos src.XPos, code errors.Code, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	// Only add the position if know the position.
	// See issue golang.org/issue/11361.
	if pos.IsKnown() {
		msg = fmt.Sprintf("%v: %s", FmtPos(pos), msg)
	}
	errorMsgs = append(errorMsgs, errorMsg{
		pos:  pos,
		msg:  msg + "\n",
		code: code,
	})
}

// FmtPos formats pos as a file:line string.
func FmtPos(pos src.XPos) string {
	if Ctxt == nil {
		return "???"
	}
	return Ctxt.OutermostPos(pos).Format(Flag.C == 0, Flag.L == 1)
}

// byPos sorts errors by source position.
type byPos []errorMsg

func (x byPos) Len() int           { return len(x) }
func (x byPos) Less(i, j int) bool { return x[i].pos.Before(x[j].pos) }
func (x byPos) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// FlushErrors sorts errors seen so far by line number, prints them to stdout,
// and empties the errors array.
func FlushErrors() {
	if Ctxt != nil && Ctxt.Bso != nil {
		Ctxt.Bso.Flush()
	}
	if len(errorMsgs) == 0 {
		return
	}
	if Flag.LowerU == 0 {
		sort.Stable(byPos(errorMsgs))
	}
	for i, err := range errorMsgs {
		if i == 0 || err.msg != errorMsgs[i-1].msg {
			fmt.Print(err.msg)
		}
	}
	errorMsgs = errorMsgs[:0]
}

// lasterror keeps track of the most recently issued error,
// to avoid printing multiple error messages on the same line.
var lasterror struct {
	syntax src.XPos // source position of last syntax error
	other  src.XPos // source position of last non-syntax error
	msg    string   // error message of last non-syntax error
}

// sameline reports whether two positions a, b are on the same line.
func sameline(a, b src.XPos) bool {
	p := Ctxt.PosTable.Pos(a)
	q := Ctxt.PosTable.Pos(b)
	return p.Base() == q.Base() && p.Line() == q.Line()
}

// Errorf reports a formatted error at the current line.
func Errorf(format string, args ...any) {
	ErrorfAt(Pos, 0, format, args...)
}

// ErrorfAt reports a formatted error message at pos.
func ErrorfAt(pos src.XPos, code errors.Code, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)

	if strings.HasPrefix(msg, "syntax error") {
		numSyntaxErrors++
		// only one syntax error per line, no matter what error
		if sameline(lasterror.syntax, pos) {
			return
		}
		lasterror.syntax = pos
	} else {
		// only one of multiple equal non-syntax errors per line
		// (FlushErrors shows only one of them, so we filter them
		// here as best as we can (they may not appear in order)
		// so that we don't count them here and exit early, and
		// then have nothing to show for.)
		if sameline(lasterror.other, pos) && lasterror.msg == msg {
			return
		}
		lasterror.other = pos
		lasterror.msg = msg
	}

	addErrorMsg(pos, code, "%s", msg)
	numErrors++

	hcrash()
	if numErrors >= 10 && Flag.LowerE == 0 {
		FlushErrors()
		fmt.Printf("%v: too many errors\n", FmtPos(pos))
		ErrorExit()
	}
}

// UpdateErrorDot is a clumsy hack that rewrites the last error,
// if it was "LINE: undefined: NAME", to be "LINE: undefined: NAME in EXPR".
// It is used to give better error messages for dot (selector) expressions.
func UpdateErrorDot(line string, name, expr string) {
	if len(errorMsgs) == 0 {
		return
	}
	e := &errorMsgs[len(errorMsgs)-1]
	if strings.HasPrefix(e.msg, line) && e.msg == fmt.Sprintf("%v: undefined: %v\n", line, name) {
		e.msg = fmt.Sprintf("%v: undefined: %v in %v\n", line, name, expr)
	}
}

// Warn reports a formatted warning at the current line.
// In general the Go compiler does NOT generate warnings,
// so this should be used only when the user has opted in
// to additional output by setting a particular flag.
func Warn(format string, args ...any) {
	WarnfAt(Pos, format, args...)
}

// WarnfAt reports a formatted warning at pos.
// In general the Go compiler does NOT generate warnings,
// so this should be used only when the user has opted in
// to additional output by setting a particular flag.
func WarnfAt(pos src.XPos, format string, args ...any) {
	addErrorMsg(pos, 0, format, args...)
	if Flag.LowerM != 0 {
		FlushErrors()
	}
}

// Fatalf reports a fatal error - an internal problem - at the current line and exits.
// If other errors have already been printed, then Fatalf just quietly exits.
// (The internal problem may have been caused by incomplete information
// after the already-reported errors, so best to let users fix those and
// try again without being bothered about a spurious internal error.)
//
// But if no errors have been printed, or if -d panic has been specified,
// Fatalf prints the error as an "internal compiler error". In a released build,
// it prints an error asking to file a bug report. In development builds, it
// prints a stack trace.
//
// If -h has been specified, Fatalf panics to force the usual runtime info dump.
func Fatalf(format string, args ...any) {
	FatalfAt(Pos, format, args...)
}

var bugStack = counter.NewStack("compile/bug", 16) // 16 is arbitrary; used by gopls and crashmonitor

// FatalfAt reports a fatal error - an internal problem - at pos and exits.
// If other errors have already been printed, then FatalfAt just quietly exits.
// (The internal problem may have been caused by incomplete information
// after the already-reported errors, so best to let users fix those and
// try again without being bothered about a spurious internal error.)
//
// But if no errors have been printed, or if -d panic has been specified,
// FatalfAt prints the error as an "internal compiler error". In a released build,
// it prints an error asking to file a bug report. In development builds, it
// prints a stack trace.
//
// If -h has been specified, FatalfAt panics to force the usual runtime info dump.
func FatalfAt(pos src.XPos, format string, args ...any) {
	FlushErrors()

	bugStack.Inc()

	if Debug.Panic != 0 || numErrors == 0 {
		fmt.Printf("%v: internal compiler error: ", FmtPos(pos))
		fmt.Printf(format, args...)
		fmt.Printf("\n")

		// If this is a released compiler version, ask for a bug report.
		if Debug.Panic == 0 && strings.HasPrefix(buildcfg.Version, "go") && !strings.Contains(buildcfg.Version, "devel") {
			fmt.Printf("\n")
			fmt.Printf("Please file a bug report including a short program that triggers the error.\n")
			fmt.Printf("https://go.dev/issue/new\n")
		} else {
			// Not a release; dump a stack trace, too.
			fmt.Println()
			os.Stdout.Write(debug.Stack())
			fmt.Println()
		}
	}

	hcrash()
	ErrorExit()
}

// Assert reports "assertion failed" with Fatalf, unless b is true.
func Assert(b bool) {
	if !b {
		Fatalf("assertion failed")
	}
}

// Assertf reports a fatal error with Fatalf, unless b is true.
func Assertf(b bool, format string, args ...any) {
	if !b {
		Fatalf(format, args...)
	}
}

// AssertfAt reports a fatal error with FatalfAt, unless b is true.
func AssertfAt(b bool, pos src.XPos, format string, args ...any) {
	if !b {
		FatalfAt(pos, format, args...)
	}
}

// hcrash crashes the compiler when -h is set, to find out where a message is generated.
func hcrash() {
	if Flag.LowerH != 0 {
		FlushErrors()
		if Flag.LowerO != "" {
			os.Remove(Flag.LowerO)
		}
		panic("-h")
	}
}

// ErrorExit handles an error-status exit.
// It flushes any pending errors, removes the output file, and exits.
func ErrorExit() {
	FlushErrors()
	if Flag.LowerO != "" {
		os.Remove(Flag.LowerO)
	}
	os.Exit(2)
}

// ExitIfErrors calls ErrorExit if any errors have been reported.
func ExitIfErrors() {
	if Errors() > 0 {
		ErrorExit()
	}
}

var AutogeneratedPos src.XPos

```

// === FILE: references/go/src/cmd/compile/internal/base/startheap.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"sync"
)

// forEachGC calls fn each GC cycle until it returns false.
func forEachGC(fn func() bool) {
	type T [32]byte // large enough to avoid runtime's tiny object allocator
	var finalizer func(*T)
	finalizer = func(p *T) {

		if fn() {
			runtime.SetFinalizer(p, finalizer)
		}
	}

	finalizer(new(T))
}

// AdjustStartingHeap modifies GOGC so that GC should not occur until the heap
// grows to the requested size.  This is intended but not promised, though it
// is true-mostly, depending on when the adjustment occurs and on the
// compiler's input and behavior.  Once the live heap is approximately half
// this size, GOGC is reset to its value when AdjustStartingHeap was called;
// subsequent GCs may reduce the heap below the requested size, but this
// function does not affect that.
//
// logHeapTweaks (-d=gcadjust=1) enables logging of GOGC adjustment events.
//
// The temporarily requested GOGC is derated from what would be the "obvious"
// value necessary to hit the starting heap goal because the obvious
// (goal/live-1)*100 value seems to grow RSS a little more than it "should"
// (compared to GOMEMLIMIT, e.g.) and the assumption is that the GC's control
// algorithms are tuned for GOGC near 100, and not tuned for huge values of
// GOGC.  Different derating factors apply for "lo" and "hi" values of GOGC;
// lo is below derateBreak, hi is above derateBreak.  The derating factors,
// expressed as integer percentages, are derateLoPct and derateHiPct.
// 60-75 is an okay value for derateLoPct, 30-65 seems like a good value for
// derateHiPct, and 600 seems like a good value for derateBreak.  If these
// are zero, defaults are used instead.
//
// NOTE: If you think this code would help startup time in your own
// application and you decide to use it, please benchmark first to see if it
// actually works for you (it may not: the Go compiler is not typical), and
// whatever the outcome, please leave a comment on bug #56546.  This code
// uses supported interfaces, but depends more than we like on
// current+observed behavior of the garbage collector, so if many people need
// this feature, we should consider/propose a better way to accomplish it.
func AdjustStartingHeap(requestedHeapGoal, derateBreak, derateLoPct, derateHiPct uint64, logHeapTweaks bool) {
	mp := runtime.GOMAXPROCS(0)

	const (
		SHgoal   = "/gc/heap/goal:bytes"
		SHcount  = "/gc/cycles/total:gc-cycles"
		SHallocs = "/gc/heap/allocs:bytes"
		SHfrees  = "/gc/heap/frees:bytes"
	)

	var sample = []metrics.Sample{{Name: SHgoal}, {Name: SHcount}, {Name: SHallocs}, {Name: SHfrees}}

	const (
		SH_GOAL   = 0
		SH_COUNT  = 1
		SH_ALLOCS = 2
		SH_FREES  = 3

		MB = 1_000_000
	)

	// These particular magic numbers are designed to make the RSS footprint of -d=-gcstart=2000
	// resemble that of GOMEMLIMIT=2000MiB GOGC=10000 when building large projects
	// (e.g. the Go compiler itself, and the microsoft's typescript AST package),
	// with the further restriction that these magic numbers did a good job of reducing user-cpu
	// for builds at either gcstart=2000 or gcstart=128.
	//
	// The benchmarking to obtain this was (a version of):
	//
	// for i in {1..50} ; do
	//     for what in std cmd/compile cmd/fix cmd/go github.com/microsoft/typescript-go/internal/ast ; do
	//       whatbase=`basename ${what}`
	//       for sh in 128 2000 ; do
	//         for br in 500 600 ; do
	//           for shlo in 65 70; do
	//             for shhi in 55 60 ; do
	//               benchcmd -n=2 ${whatbase} go build -a \
	//               -gcflags=all=-d=gcstart=${sh},gcstartloderate=${shlo},gcstarthiderate=${shhi},gcstartbreak=${br} \
	//               ${what} | tee -a startheap${sh}_${br}_${shhi}_${shlo}.bench
	//             done
	//           done
	//         done
	//       done
	//     done
	// done
	//
	// benchcmd is "go install github.com/aclements/go-misc/benchcmd@latest"

	if derateBreak == 0 {
		derateBreak = 600
	}
	if derateLoPct == 0 {
		derateLoPct = 70
	}
	if derateHiPct == 0 {
		derateHiPct = 55
	}

	gogcDerate := func(myGogc uint64) uint64 {
		if myGogc < derateBreak {
			return (myGogc * derateLoPct) / 100
		}
		return (myGogc * derateHiPct) / 100
	}

	// Assumptions and observations of Go's garbage collector, as of Go 1.17-1.20:

	// - the initial heap goal is 4MiB, by fiat.  It is possible for Go to start
	//   with a heap as small as 512k, so this may change in the future.

	// - except for the first heap goal, heap goal is a function of
	//   observed-live at the previous GC and current GOGC.  After the first
	//   GC, adjusting GOGC immediately updates GOGC; before the first GC,
	//   adjusting GOGC does not modify goal (but the change takes effect after
	//   the first GC).

	// - the before/after first GC behavior is not guaranteed anywhere, it's
	//   just behavior, and it's a bad idea to rely on it.

	// - we don't know exactly when GC will run, even after we adjust GOGC; the
	//   first GC may not have happened yet, may have already happened, or may
	//   be currently in progress, and GCs can start for several reasons.

	// - forEachGC above will run the provided function at some delay after each
	//   GC's mark phase terminates; finalizers are run after marking as the
	//   spans containing finalizable objects are swept, driven by GC
	//   background activity and allocation demand.

	// - "live at last GC" is not available through the current metrics
	//    interface. Instead, live is estimated by knowing the adjusted value of
	//    GOGC and the new heap goal following a GC (this requires knowing that
	//    at least one GC has occurred):
	//		  estLive = 100 * newGoal / (100 + currentGogc)
	//    this new value of GOGC
	//		  newGogc = 100*requestedHeapGoal/estLive - 100
	//    will result in the desired goal. The logging code checks that the
	//    resulting goal is correct.

	// There's a small risk that the finalizer will be slow to run after a GC
	// that expands the goal to a huge value, and that this will lead to
	// out-of-memory.  This doesn't seem to happen; in experiments on a variety
	// of machines with a variety of extra loads to disrupt scheduling, the
	// worst overshoot observed was 50% past requestedHeapGoal.

	metrics.Read(sample)
	for _, s := range sample {
		if s.Value.Kind() == metrics.KindBad {
			// Just return, a slightly slower compilation is a tolerable outcome.
			if logHeapTweaks {
				fmt.Fprintf(os.Stderr, "GCAdjust: Regret unexpected KindBad for metric %s\n", s.Name)
			}
			return
		}
	}

	// Tinker with GOGC to make the heap grow rapidly at first.
	currentGoal := sample[SH_GOAL].Value.Uint64() // Believe this will be 4MByte or less, perhaps 512k
	myGogc := 100 * requestedHeapGoal / currentGoal
	myGogc = gogcDerate(myGogc)
	if myGogc <= 125 {
		return
	}

	if logHeapTweaks {
		sample := append([]metrics.Sample(nil), sample...) // avoid races with GC callback
		AtExit(func() {
			metrics.Read(sample)
			goal := sample[SH_GOAL].Value.Uint64()
			count := sample[SH_COUNT].Value.Uint64()
			oldGogc := debug.SetGCPercent(100)
			if oldGogc == 100 {
				fmt.Fprintf(os.Stderr, "GCAdjust: AtExit goal %dMB gogc %d count %d maxprocs %d\n",
					goal/MB, oldGogc, count, mp)
			} else {
				inUse := sample[SH_ALLOCS].Value.Uint64() - sample[SH_FREES].Value.Uint64()
				overPct := 100 * (int(inUse) - int(requestedHeapGoal)) / int(requestedHeapGoal)
				fmt.Fprintf(os.Stderr, "GCAdjust: AtExit goal %dMB gogc %d count %d maxprocs %d overPct %d\n",
					goal/MB, oldGogc, count, mp, overPct)

			}
		})
	}

	originalGOGC := debug.SetGCPercent(int(myGogc))

	// forEachGC finalizers ought not overlap, but they could run in separate threads.
	// This ought not matter, but just in case it bothers the/a race detector,
	// use this mutex.
	var forEachGCLock sync.Mutex

	adjustFunc := func() bool {

		forEachGCLock.Lock()
		defer forEachGCLock.Unlock()

		metrics.Read(sample)
		goal := sample[SH_GOAL].Value.Uint64()
		count := sample[SH_COUNT].Value.Uint64()

		if goal <= requestedHeapGoal { // Stay the course
			if logHeapTweaks {
				fmt.Fprintf(os.Stderr, "GCAdjust: Reuse GOGC adjust, current goal %dMB, count is %d, current gogc %d\n",
					goal/MB, count, myGogc)
			}
			return true
		}

		// Believe goal has been adjusted upwards, else it would be less-than-or-equal to requestedHeapGoal
		calcLive := 100 * goal / (100 + myGogc)

		if 2*calcLive < requestedHeapGoal { // calcLive can exceed requestedHeapGoal!
			myGogc = 100*requestedHeapGoal/calcLive - 100
			myGogc = gogcDerate(myGogc)

			if myGogc > 125 {
				// Not done growing the heap.
				oldGogc := debug.SetGCPercent(int(myGogc))

				if logHeapTweaks {
					// Check that the new goal looks right
					inUse := sample[SH_ALLOCS].Value.Uint64() - sample[SH_FREES].Value.Uint64()
					metrics.Read(sample)
					newGoal := sample[SH_GOAL].Value.Uint64()
					pctOff := 100 * (int64(newGoal) - int64(requestedHeapGoal)) / int64(requestedHeapGoal)
					// Check that the new goal is close to requested.  3% of make.bash fails this test.  Why, TBD.
					if pctOff < 2 {
						fmt.Fprintf(os.Stderr, "GCAdjust: Retry GOGC adjust, current goal %dMB, count is %d, gogc was %d, is now %d, calcLive %dMB pctOff %d\n",
							goal/MB, count, oldGogc, myGogc, calcLive/MB, pctOff)
					} else {
						// The GC is being annoying and not giving us the goal that we requested, say more to help understand when/why.
						fmt.Fprintf(os.Stderr, "GCAdjust: Retry GOGC adjust, current goal %dMB, count is %d, gogc was %d, is now %d, calcLive %dMB pctOff %d inUse %dMB\n",
							goal/MB, count, oldGogc, myGogc, calcLive/MB, pctOff, inUse/MB)
					}
				}
				return true
			}
		}

		// In this case we're done boosting GOGC, set it to its original value and don't set a new finalizer.
		oldGogc := debug.SetGCPercent(originalGOGC)
		// inUse helps estimate how late the finalizer ran; at the instant the previous GC ended,
		// it was (in theory) equal to the previous GC's heap goal.  In a growing heap it is
		// expected to grow to the new heap goal.
		if logHeapTweaks {
			inUse := sample[SH_ALLOCS].Value.Uint64() - sample[SH_FREES].Value.Uint64()
			overPct := 100 * (int(inUse) - int(requestedHeapGoal)) / int(requestedHeapGoal)
			fmt.Fprintf(os.Stderr, "GCAdjust: Reset GOGC adjust, old goal %dMB, count is %d, gogc was %d, gogc is now %d, calcLive %dMB inUse %dMB overPct %d\n",
				goal/MB, count, oldGogc, originalGOGC, calcLive/MB, inUse/MB, overPct)
		}
		return false
	}

	forEachGC(adjustFunc)
}

```

// === FILE: references/go/src/cmd/compile/internal/base/timings.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"fmt"
	"io"
	"strings"
	"time"
)

var Timer Timings

// Timings collects the execution times of labeled phases
// which are added through a sequence of Start/Stop calls.
// Events may be associated with each phase via AddEvent.
type Timings struct {
	list   []timestamp
	events map[int][]*event // lazily allocated
}

type timestamp struct {
	time  time.Time
	label string
	start bool
}

type event struct {
	size int64  // count or amount of data processed (allocations, data size, lines, funcs, ...)
	unit string // unit of size measure (count, MB, lines, funcs, ...)
}

func (t *Timings) append(labels []string, start bool) {
	t.list = append(t.list, timestamp{time.Now(), strings.Join(labels, ":"), start})
}

// Start marks the beginning of a new phase and implicitly stops the previous phase.
// The phase name is the colon-separated concatenation of the labels.
func (t *Timings) Start(labels ...string) {
	t.append(labels, true)
}

// Stop marks the end of a phase and implicitly starts a new phase.
// The labels are added to the labels of the ended phase.
func (t *Timings) Stop(labels ...string) {
	t.append(labels, false)
}

// AddEvent associates an event, i.e., a count, or an amount of data,
// with the most recently started or stopped phase; or the very first
// phase if Start or Stop hasn't been called yet. The unit specifies
// the unit of measurement (e.g., MB, lines, no. of funcs, etc.).
func (t *Timings) AddEvent(size int64, unit string) {
	m := t.events
	if m == nil {
		m = make(map[int][]*event)
		t.events = m
	}
	i := len(t.list)
	if i > 0 {
		i--
	}
	m[i] = append(m[i], &event{size, unit})
}

// Write prints the phase times to w.
// The prefix is printed at the start of each line.
func (t *Timings) Write(w io.Writer, prefix string) {
	if len(t.list) > 0 {
		var lines lines

		// group of phases with shared non-empty label prefix
		var group struct {
			label string        // label prefix
			tot   time.Duration // accumulated phase time
			size  int           // number of phases collected in group
		}

		// accumulated time between Stop/Start timestamps
		var unaccounted time.Duration

		// process Start/Stop timestamps
		pt := &t.list[0] // previous timestamp
		tot := t.list[len(t.list)-1].time.Sub(pt.time)
		for i := 1; i < len(t.list); i++ {
			qt := &t.list[i] // current timestamp
			dt := qt.time.Sub(pt.time)

			var label string
			var events []*event
			if pt.start {
				// previous phase started
				label = pt.label
				events = t.events[i-1]
				if qt.start {
					// start implicitly ended previous phase; nothing to do
				} else {
					// stop ended previous phase; append stop labels, if any
					if qt.label != "" {
						label += ":" + qt.label
					}
					// events associated with stop replace prior events
					if e := t.events[i]; e != nil {
						events = e
					}
				}
			} else {
				// previous phase stopped
				if qt.start {
					// between a stopped and started phase; unaccounted time
					unaccounted += dt
				} else {
					// previous stop implicitly started current phase
					label = qt.label
					events = t.events[i]
				}
			}
			if label != "" {
				// add phase to existing group, or start a new group
				l := commonPrefix(group.label, label)
				if group.size == 1 && l != "" || group.size > 1 && l == group.label {
					// add to existing group
					group.label = l
					group.tot += dt
					group.size++
				} else {
					// start a new group
					if group.size > 1 {
						lines.add(prefix+group.label+"subtotal", 1, group.tot, tot, nil)
					}
					group.label = label
					group.tot = dt
					group.size = 1
				}

				// write phase
				lines.add(prefix+label, 1, dt, tot, events)
			}

			pt = qt
		}

		if group.size > 1 {
			lines.add(prefix+group.label+"subtotal", 1, group.tot, tot, nil)
		}

		if unaccounted != 0 {
			lines.add(prefix+"unaccounted", 1, unaccounted, tot, nil)
		}

		lines.add(prefix+"total", 1, tot, tot, nil)

		lines.write(w)
	}
}

func commonPrefix(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return a[:i]
}

type lines [][]string

func (lines *lines) add(label string, n int, dt, tot time.Duration, events []*event) {
	var line []string
	add := func(format string, args ...any) {
		line = append(line, fmt.Sprintf(format, args...))
	}

	add("%s", label)
	add("    %d", n)
	add("    %d ns/op", dt)
	add("    %.2f %%", float64(dt)/float64(tot)*100)

	for _, e := range events {
		add("    %d", e.size)
		add(" %s", e.unit)
		add("    %d", int64(float64(e.size)/dt.Seconds()+0.5))
		add(" %s/s", e.unit)
	}

	*lines = append(*lines, line)
}

func (lines lines) write(w io.Writer) {
	// determine column widths and contents
	var widths []int
	var number []bool
	for _, line := range lines {
		for i, col := range line {
			if i < len(widths) {
				if len(col) > widths[i] {
					widths[i] = len(col)
				}
			} else {
				widths = append(widths, len(col))
				number = append(number, isnumber(col)) // first line determines column contents
			}
		}
	}

	// make column widths a multiple of align for more stable output
	const align = 1 // set to a value > 1 to enable
	if align > 1 {
		for i, w := range widths {
			w += align - 1
			widths[i] = w - w%align
		}
	}

	// print lines taking column widths and contents into account
	for _, line := range lines {
		for i, col := range line {
			format := "%-*s"
			if number[i] {
				format = "%*s" // numbers are right-aligned
			}
			fmt.Fprintf(w, format, widths[i], col)
		}
		fmt.Fprintln(w)
	}
}

func isnumber(s string) bool {
	for _, ch := range s {
		if ch <= ' ' {
			continue // ignore leading whitespace
		}
		return '0' <= ch && ch <= '9' || ch == '.' || ch == '-' || ch == '+'
	}
	return false
}

```

