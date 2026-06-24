# Domain Architecture: cmd/compile/internal/gc

## Layout Topology
```text
cmd/compile/internal/gc/
├── compile.go
├── export.go
├── main.go
├── obj.go
└── util.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/compile/internal/gc/compile.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gc

import (
	"cmp"
	"internal/race"
	"math/rand"
	"slices"
	"sync"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/liveness"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/pgoir"
	"cmd/compile/internal/ssagen"
	"cmd/compile/internal/staticinit"
	"cmd/compile/internal/types"
	"cmd/compile/internal/walk"
	"cmd/internal/obj"
)

// "Portable" code generation.

var (
	compilequeue []*ir.Func // functions waiting to be compiled
)

func enqueueFunc(fn *ir.Func, symABIs *ssagen.SymABIs) {
	if ir.CurFunc != nil {
		base.FatalfAt(fn.Pos(), "enqueueFunc %v inside %v", fn, ir.CurFunc)
	}

	if ir.FuncName(fn) == "_" {
		// Skip compiling blank functions.
		// Frontend already reported any spec-mandated errors (#29870).
		return
	}

	if fn.IsClosure() {
		return // we'll get this as part of its enclosing function
	}

	if ssagen.CreateWasmImportWrapper(fn) {
		return
	}

	if len(fn.Body) == 0 {
		if ir.IsIntrinsicSym(fn.Sym()) && fn.Sym().Linkname == "" && !symABIs.HasDef(fn.Sym()) {
			// Generate the function body for a bodyless intrinsic, in case it
			// is used in a non-call context (e.g. as a function pointer).
			// We skip functions defined in assembly, or has a linkname (which
			// could be defined in another package).
			ssagen.GenIntrinsicBody(fn)
		} else {
			// Initialize ABI wrappers if necessary.
			ir.InitLSym(fn, false)
			types.CalcSize(fn.Type())
			a := ssagen.AbiForBodylessFuncStackMap(fn)
			abiInfo := a.ABIAnalyzeFuncType(fn.Type()) // abiInfo has spill/home locations for wrapper
			if fn.ABI == obj.ABI0 {
				// The current args_stackmap generation assumes the function
				// is ABI0, and only ABI0 assembly function can have a FUNCDATA
				// reference to args_stackmap (see cmd/internal/obj/plist.go:Flushplist).
				// So avoid introducing an args_stackmap if the func is not ABI0.
				liveness.WriteFuncMap(fn, abiInfo)

				x := ssagen.EmitArgInfo(fn, abiInfo)
				objw.Global(x, int32(len(x.P)), obj.RODATA|obj.LOCAL)
			}
			return
		}
	}

	errorsBefore := base.Errors()

	todo := []*ir.Func{fn}
	for len(todo) > 0 {
		next := todo[len(todo)-1]
		todo = todo[:len(todo)-1]

		prepareFunc(next)
		todo = append(todo, next.Closures...)
	}

	if base.Errors() > errorsBefore {
		return
	}

	// Enqueue just fn itself. compileFunctions will handle
	// scheduling compilation of its closures after it's done.
	compilequeue = append(compilequeue, fn)
}

// prepareFunc handles any remaining frontend compilation tasks that
// aren't yet safe to perform concurrently.
func prepareFunc(fn *ir.Func) {
	// Set up the function's LSym early to avoid data races with the assemblers.
	// Do this before walk, as walk needs the LSym to set attributes/relocations
	// (e.g. in MarkTypeUsedInInterface).
	ir.InitLSym(fn, true)

	// If this function is a compiler-generated outlined global map
	// initializer function, register its LSym for later processing.
	if staticinit.MapInitToVar != nil {
		if _, ok := staticinit.MapInitToVar[fn]; ok {
			ssagen.RegisterMapInitLsym(fn.Linksym())
		}
	}

	// Calculate parameter offsets.
	types.CalcSize(fn.Type())

	// Generate wrappers between Go ABI and Wasm ABI, for a wasmexport
	// function.
	// Must be done after InitLSym and CalcSize.
	ssagen.GenWasmExportWrapper(fn)

	ir.CurFunc = fn
	walk.Walk(fn)
	if ir.MatchAstDump(fn, "walk") {
		ir.AstDump(fn, "walk, "+ir.FuncName(fn))
	}
	ir.CurFunc = nil // enforce no further uses of CurFunc

	base.Ctxt.DwTextCount++
}

// compileFunctions compiles all functions in compilequeue.
// It fans out nBackendWorkers to do the work
// and waits for them to complete.
func compileFunctions(profile *pgoir.Profile) {
	if race.Enabled {
		// Randomize compilation order to try to shake out races.
		tmp := make([]*ir.Func, len(compilequeue))
		perm := rand.Perm(len(compilequeue))
		for i, v := range perm {
			tmp[v] = compilequeue[i]
		}
		copy(compilequeue, tmp)
	} else {
		// Compile the longest functions first,
		// since they're most likely to be the slowest.
		// This helps avoid stragglers.
		// Since we remove from the end of the slice queue,
		// that means shortest to longest.
		slices.SortFunc(compilequeue, func(a, b *ir.Func) int {
			return cmp.Compare(len(a.Body), len(b.Body))
		})
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	mu.Lock()

	for workerId := range base.Flag.LowerC {
		// TODO: replace with wg.Go when the oldest bootstrap has it.
		// With the current policy, that'd be go1.27.
		wg.Add(1)
		go func() {
			defer wg.Done()
			var closures []*ir.Func
			for {
				mu.Lock()
				compilequeue = append(compilequeue, closures...)
				remaining := len(compilequeue)
				if remaining == 0 {
					mu.Unlock()
					return
				}
				fn := compilequeue[len(compilequeue)-1]
				compilequeue = compilequeue[:len(compilequeue)-1]
				mu.Unlock()
				ssagen.Compile(fn, workerId, profile)
				closures = fn.Closures
			}
		}()
	}

	types.CalcSizeDisabled = true // not safe to calculate sizes concurrently
	base.Ctxt.InParallel = true

	mu.Unlock()
	wg.Wait()
	compilequeue = nil

	base.Ctxt.InParallel = false
	types.CalcSizeDisabled = false
}

```

// === FILE: references/go/src/cmd/compile/internal/gc/export.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gc

import (
	"fmt"
	"go/constant"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/bio"
)

func dumpasmhdr() {
	b, err := bio.Create(base.Flag.AsmHdr)
	if err != nil {
		base.Fatalf("%v", err)
	}
	fmt.Fprintf(b, "// generated by compile -asmhdr from package %s\n\n", types.LocalPkg.Name)
	for _, n := range typecheck.Target.AsmHdrDecls {
		if n.Sym().IsBlank() {
			continue
		}
		switch n.Op() {
		case ir.OLITERAL:
			t := n.Val().Kind()
			if t == constant.Float || t == constant.Complex {
				break
			}
			fmt.Fprintf(b, "#define const_%s %v\n", n.Sym().Name, n.Val().ExactString())

		case ir.OTYPE:
			t := n.Type()
			if !t.IsStruct() || t.StructType().Map != nil || t.IsFuncArgStruct() {
				break
			}
			fmt.Fprintf(b, "#define %s__size %d\n", n.Sym().Name, int(t.Size()))
			for _, f := range t.Fields() {
				if !f.Sym.IsBlank() {
					fmt.Fprintf(b, "#define %s_%s %d\n", n.Sym().Name, f.Sym.Name, int(f.Offset))
				}
			}
		}
	}

	if err := b.Close(); err != nil {
		base.Fatalf("%v", err)
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/gc/main.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gc

import (
	"bufio"
	"bytes"
	"cmd/compile/internal/base"
	"cmd/compile/internal/bloop"
	"cmd/compile/internal/coverage"
	"cmd/compile/internal/deadlocals"
	"cmd/compile/internal/dwarfgen"
	"cmd/compile/internal/escape"
	"cmd/compile/internal/inline"
	"cmd/compile/internal/inline/interleaved"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/logopt"
	"cmd/compile/internal/loopvar"
	"cmd/compile/internal/noder"
	"cmd/compile/internal/pgoir"
	"cmd/compile/internal/pkginit"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/rttype"
	"cmd/compile/internal/slice"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/ssagen"
	"cmd/compile/internal/staticinit"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/dwarf"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"
	"cmd/internal/telemetry/counter"
	"flag"
	"fmt"
	"internal/buildcfg"
	"log"
	"os"
	"runtime"
)

// handlePanic ensures that we print out an "internal compiler error" for any panic
// or runtime exception during front-end compiler processing (unless there have
// already been some compiler errors). It may also be invoked from the explicit panic in
// hcrash(), in which case, we pass the panic on through.
func handlePanic() {
	ir.CloseHTMLWriters()
	noder.CloseHTMLWriters()
	if err := recover(); err != nil {
		if err == "-h" {
			// Force real panic now with -h option (hcrash) - the error
			// information will have already been printed.
			panic(err)
		}
		base.Fatalf("panic: %v", err)
	}
}

// Main parses flags and Go source files specified in the command-line
// arguments, type-checks the parsed Go package, compiles functions to machine
// code, and finally writes the compiled package definition to disk.
func Main(archInit func(*ssagen.ArchInfo)) {
	base.Timer.Start("fe", "init")
	counter.Open()
	counter.Inc("compile/invocations")

	defer handlePanic()

	archInit(&ssagen.Arch)

	base.Ctxt = obj.Linknew(ssagen.Arch.LinkArch)
	base.Ctxt.DiagFunc = base.Errorf
	base.Ctxt.DiagFlush = base.FlushErrors
	base.Ctxt.Bso = bufio.NewWriter(os.Stdout)

	// UseBASEntries is preferred because it shaves about 2% off build time, but LLDB, dsymutil, and dwarfdump
	// on Darwin don't support it properly, especially since macOS 10.14 (Mojave).  This is exposed as a flag
	// to allow testing with LLVM tools on Linux, and to help with reporting this bug to the LLVM project.
	// See bugs 31188 and 21945 (CLs 170638, 98075, 72371).
	base.Ctxt.UseBASEntries = base.Ctxt.Headtype != objabi.Hdarwin

	base.DebugSSA = ssa.PhaseOption
	base.ParseFlags()

	if flagGCStart := base.Debug.GCStart; flagGCStart > 0 || // explicit flags overrides environment variable disable of GC boost
		os.Getenv("GOGC") == "" && os.Getenv("GOMEMLIMIT") == "" && base.Flag.LowerC != 1 { // explicit GC knobs or no concurrency implies default heap
		startHeapMB := int64(128)
		if flagGCStart > 0 {
			startHeapMB = int64(flagGCStart)
		}
		base.AdjustStartingHeap(uint64(startHeapMB)<<20, 0, 0, 0, base.Debug.GCAdjust == 1)
	}

	types.LocalPkg = types.NewPkg(base.Ctxt.Pkgpath, "")

	// pseudo-package, for scoping
	types.BuiltinPkg = types.NewPkg("go.builtin", "") // TODO(gri) name this package go.builtin?
	types.BuiltinPkg.Prefix = "go:builtin"

	// pseudo-package, accessed by import "unsafe"
	types.UnsafePkg = types.NewPkg("unsafe", "unsafe")

	// Pseudo-package that contains the compiler's builtin
	// declarations for package runtime. These are declared in a
	// separate package to avoid conflicts with package runtime's
	// actual declarations, which may differ intentionally but
	// insignificantly.
	ir.Pkgs.Runtime = types.NewPkg("go.runtime", "runtime")
	ir.Pkgs.Runtime.Prefix = "runtime"

	// Pseudo-package that contains the compiler's builtin
	// declarations for maps.
	ir.Pkgs.InternalMaps = types.NewPkg("go.internal/runtime/maps", "internal/runtime/maps")
	ir.Pkgs.InternalMaps.Prefix = "internal/runtime/maps"

	// pseudo-packages used in symbol tables
	ir.Pkgs.Itab = types.NewPkg("go.itab", "go.itab")
	ir.Pkgs.Itab.Prefix = "go:itab"

	// pseudo-package used for methods with anonymous receivers
	ir.Pkgs.Go = types.NewPkg("go", "")

	// pseudo-package for use with code coverage instrumentation.
	ir.Pkgs.Coverage = types.NewPkg("go.coverage", "runtime/coverage")
	ir.Pkgs.Coverage.Prefix = "runtime/coverage"

	// Record flags that affect the build result. (And don't
	// record flags that don't, since that would cause spurious
	// changes in the binary.)
	dwarfgen.RecordFlags("B", "N", "l", "msan", "race", "asan", "shared", "dynlink", "dwarf", "dwarflocationlists", "dwarfbasentries", "smallframes", "spectre")

	if !base.EnableTrace && base.Flag.LowerT {
		log.Fatalf("compiler not built with support for -t")
	}

	// Enable inlining (after RecordFlags, to avoid recording the rewritten -l).  For now:
	//	default: inlining on.  (Flag.LowerL == 1)
	//	-l: inlining off  (Flag.LowerL == 0)
	//	-l=2, -l=3: inlining on again, with extra debugging (Flag.LowerL > 1)
	if base.Flag.LowerL <= 1 {
		base.Flag.LowerL = 1 - base.Flag.LowerL
	}

	if base.Flag.SmallFrames {
		ir.MaxStackVarSize = 64 * 1024
		ir.MaxImplicitStackVarSize = 16 * 1024
	}

	if base.Flag.Dwarf {
		base.Ctxt.DebugInfo = dwarfgen.Info
		base.Ctxt.GenAbstractFunc = dwarfgen.AbstractFunc
		base.Ctxt.DwFixups = obj.NewDwarfFixupTable(base.Ctxt)
	} else {
		// turn off inline generation if no dwarf at all
		base.Flag.GenDwarfInl = 0
		base.Ctxt.Flag_locationlists = false
	}
	if base.Ctxt.Flag_locationlists && len(base.Ctxt.Arch.DWARFRegisters) == 0 {
		log.Fatalf("location lists requested but register mapping not available on %v", base.Ctxt.Arch.Name)
	}

	types.ParseLangFlag()

	symABIs := ssagen.NewSymABIs()
	if base.Flag.SymABIs != "" {
		symABIs.ReadSymABIs(base.Flag.SymABIs)
	}

	if objabi.LookupPkgSpecial(base.Ctxt.Pkgpath).NoInstrument {
		base.Flag.Race = false
		base.Flag.MSan = false
		base.Flag.ASan = false
	}

	ssagen.Arch.LinkArch.Init(base.Ctxt)
	startProfile()
	if base.Flag.Race || base.Flag.MSan || base.Flag.ASan {
		base.Flag.Cfg.Instrumenting = true
	}
	if base.Flag.Dwarf {
		dwarf.EnableLogging(base.Debug.DwarfInl != 0)
	}
	if base.Debug.SoftFloat != 0 {
		ssagen.Arch.SoftFloat = true
	}

	if base.Flag.JSON != "" { // parse version,destination from json logging optimization.
		logopt.LogJsonOption(base.Flag.JSON)
	}

	ir.EscFmt = escape.Fmt
	ir.IsIntrinsicCall = ssagen.IsIntrinsicCall
	ir.IsIntrinsicSym = ssagen.IsIntrinsicSym
	inline.SSADumpInline = ssagen.DumpInline
	ssagen.InitEnv()

	types.PtrSize = ssagen.Arch.LinkArch.PtrSize
	types.RegSize = ssagen.Arch.LinkArch.RegSize
	types.MaxWidth = ssagen.Arch.MAXWIDTH

	typecheck.Target = new(ir.Package)

	base.AutogeneratedPos = makePos(src.NewFileBase("<autogenerated>", "<autogenerated>"), 1, 0)

	typecheck.InitUniverse()
	typecheck.InitRuntime()
	rttype.Init()

	// Some intrinsics (notably, the simd intrinsics) mention
	// types "eagerly", thus ssagen must be initialized AFTER
	// the type system is ready.
	ssagen.InitTables()

	// Parse and typecheck input.
	noder.LoadPackage(flag.Args())

	// As a convenience to users (toolchain maintainers, in particular),
	// when compiling a package named "main", we default the package
	// path to "main" if the -p flag was not specified.
	if base.Ctxt.Pkgpath == obj.UnlinkablePkg && types.LocalPkg.Name == "main" {
		base.Ctxt.Pkgpath = "main"
		types.LocalPkg.Path = "main"
		types.LocalPkg.Prefix = "main"
	}

	dwarfgen.RecordPackageName()

	// Prepare for backend processing.
	ssagen.InitConfig()

	// Apply coverage fixups, if applicable.
	coverage.Fixup()

	// Read profile file and build profile-graph and weighted-call-graph.
	base.Timer.Start("fe", "pgo-load-profile")
	var profile *pgoir.Profile
	if base.Flag.PgoProfile != "" {
		var err error
		profile, err = pgoir.New(base.Flag.PgoProfile)
		if err != nil {
			log.Fatalf("%s: PGO error: %v", base.Flag.PgoProfile, err)
		}
	}

	for _, fn := range typecheck.Target.Funcs {
		if ir.MatchAstDump(fn, "start") {
			ir.AstDump(fn, "start, "+ir.FuncName(fn))
		}
	}

	// Apply bloop markings.
	bloop.Walk(typecheck.Target)

	// Interleaved devirtualization and inlining.
	base.Timer.Start("fe", "devirtualize-and-inline")
	interleaved.DevirtualizeAndInlinePackage(typecheck.Target, profile)

	for _, fn := range typecheck.Target.Funcs {
		if ir.MatchAstDump(fn, "devirtualize-and-inline") {
			ir.AstDump(fn, "devirtualize-and-inline, "+ir.FuncName(fn))
		}
	}

	noder.MakeWrappers(typecheck.Target) // must happen after inlining

	// Get variable capture right in for loops.
	var transformed []loopvar.VarAndLoop
	for _, fn := range typecheck.Target.Funcs {
		transformed = append(transformed, loopvar.ForCapture(fn)...)
	}
	ir.CurFunc = nil

	// Build init task, if needed.
	pkginit.MakeTask()

	// Generate ABI wrappers. Must happen before escape analysis
	// and doesn't benefit from dead-coding or inlining.
	symABIs.GenABIWrappers()

	deadlocals.Funcs(typecheck.Target.Funcs)

	// Escape analysis.
	// Required for moving heap allocations onto stack,
	// which in turn is required by the closure implementation,
	// which stores the addresses of stack variables into the closure.
	// If the closure does not escape, it needs to be on the stack
	// or else the stack copier will not update it.
	// Large values are also moved off stack in escape analysis;
	// because large values may contain pointers, it must happen early.
	base.Timer.Start("fe", "escapes")
	escape.Funcs(typecheck.Target.Funcs)

	slice.Funcs(typecheck.Target.Funcs)

	loopvar.LogTransformations(transformed)

	// Collect information for go:nowritebarrierrec
	// checking. This must happen before transforming closures during Walk
	// We'll do the final check after write barriers are
	// inserted.
	if base.Flag.CompilingRuntime {
		ssagen.EnableNoWriteBarrierRecCheck()
	}

	ir.CurFunc = nil

	reflectdata.WriteBasicTypes()

	// Compile top-level declarations.
	//
	// There are cyclic dependencies between all of these phases, so we
	// need to iterate all of them until we reach a fixed point.
	base.Timer.Start("be", "compilefuncs")
	for nextFunc, nextExtern := 0, 0; ; {
		reflectdata.WriteRuntimeTypes()

		if nextExtern < len(typecheck.Target.Externs) {
			switch n := typecheck.Target.Externs[nextExtern]; n.Op() {
			case ir.ONAME:
				dumpGlobal(n)
			case ir.OLITERAL:
				dumpGlobalConst(n)
			case ir.OTYPE:
				reflectdata.NeedRuntimeType(n.Type())
			}
			nextExtern++
			continue
		}

		if nextFunc < len(typecheck.Target.Funcs) {
			enqueueFunc(typecheck.Target.Funcs[nextFunc], symABIs)
			nextFunc++
			continue
		}

		// The SSA backend supports using multiple goroutines, so keep it
		// as late as possible to maximize how much work we can batch and
		// process concurrently.
		if len(compilequeue) != 0 {
			compileFunctions(profile)
			continue
		}

		// Finalize DWARF inline routine DIEs, then explicitly turn off
		// further DWARF inlining generation to avoid problems with
		// generated method wrappers.
		//
		// Note: The DWARF fixup code for inlined calls currently doesn't
		// allow multiple invocations, so we intentionally run it just
		// once after everything else. Worst case, some generated
		// functions have slightly larger DWARF DIEs.
		if base.Ctxt.DwFixups != nil {
			base.Ctxt.DwFixups.Finalize(base.Ctxt.Pkgpath, base.Debug.DwarfInl != 0)
			base.Ctxt.DwFixups = nil
			base.Flag.GenDwarfInl = 0
			continue // may have called reflectdata.TypeLinksym (#62156)
		}

		break
	}

	base.Timer.AddEvent(int64(len(typecheck.Target.Funcs)), "funcs")

	if base.Flag.CompilingRuntime {
		// Write barriers are now known. Check the call graph.
		ssagen.NoWriteBarrierRecCheck()
	}

	// Add keep relocations for global maps.
	if base.Debug.WrapGlobalMapCtl != 1 {
		staticinit.AddKeepRelocations()
	}

	// Write object data to disk.
	base.Timer.Start("be", "dumpobj")
	dumpdata()
	base.Ctxt.NumberSyms()
	dumpobj()
	if base.Flag.AsmHdr != "" {
		dumpasmhdr()
	}

	ssagen.CheckLargeStacks()
	typecheck.CheckFuncStack()

	if len(compilequeue) != 0 {
		base.Fatalf("%d uncompiled functions", len(compilequeue))
	}

	logopt.FlushLoggedOpts(base.Ctxt, base.Ctxt.Pkgpath)
	base.ExitIfErrors()

	base.FlushErrors()
	base.Timer.Stop()

	if base.Flag.Bench != "" {
		if err := writebench(base.Flag.Bench); err != nil {
			log.Fatalf("cannot write benchmark data: %v", err)
		}
	}
}

func writebench(filename string) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	fmt.Fprintln(&buf, "commit:", buildcfg.Version)
	fmt.Fprintln(&buf, "goos:", runtime.GOOS)
	fmt.Fprintln(&buf, "goarch:", runtime.GOARCH)
	base.Timer.Write(&buf, "BenchmarkCompile:"+base.Ctxt.Pkgpath+":")

	n, err := f.Write(buf.Bytes())
	if err != nil {
		return err
	}
	if n != buf.Len() {
		panic("bad writer")
	}

	return f.Close()
}

func makePos(b *src.PosBase, line, col uint) src.XPos {
	return base.Ctxt.PosTable.XPos(src.MakePos(b, line, col))
}

```

// === FILE: references/go/src/cmd/compile/internal/gc/obj.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gc

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/noder"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/pkginit"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/staticdata"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/archive"
	"cmd/internal/bio"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"encoding/json"
	"fmt"
	"strings"
)

// These modes say which kind of object file to generate.
// The default use of the toolchain is to set both bits,
// generating a combined compiler+linker object, one that
// serves to describe the package to both the compiler and the linker.
// In fact the compiler and linker read nearly disjoint sections of
// that file, though, so in a distributed build setting it can be more
// efficient to split the output into two files, supplying the compiler
// object only to future compilations and the linker object only to
// future links.
//
// By default a combined object is written, but if -linkobj is specified
// on the command line then the default -o output is a compiler object
// and the -linkobj output is a linker object.
const (
	modeCompilerObj = 1 << iota
	modeLinkerObj
)

func dumpobj() {
	if base.Flag.LinkObj == "" {
		dumpobj1(base.Flag.LowerO, modeCompilerObj|modeLinkerObj)
		return
	}
	dumpobj1(base.Flag.LowerO, modeCompilerObj)
	dumpobj1(base.Flag.LinkObj, modeLinkerObj)
}

func dumpobj1(outfile string, mode int) {
	bout, err := bio.Create(outfile)
	if err != nil {
		base.FlushErrors()
		fmt.Printf("can't create %s: %v\n", outfile, err)
		base.ErrorExit()
	}

	bout.WriteString("!<arch>\n")

	if mode&modeCompilerObj != 0 {
		start := startArchiveEntry(bout)
		dumpCompilerObj(bout)
		finishArchiveEntry(bout, start, "__.PKGDEF")
	}
	if mode&modeLinkerObj != 0 {
		start := startArchiveEntry(bout)
		dumpLinkerObj(bout)
		finishArchiveEntry(bout, start, "_go_.o")
	}

	if err := bout.Close(); err != nil {
		base.FlushErrors()
		fmt.Printf("error while writing to file %s: %v\n", outfile, err)
		base.ErrorExit()
	}
}

func printObjHeader(bout *bio.Writer) {
	bout.WriteString(objabi.HeaderString())
	if base.Flag.BuildID != "" {
		fmt.Fprintf(bout, "build id %q\n", base.Flag.BuildID)
	}
	if types.LocalPkg.Name == "main" {
		fmt.Fprintf(bout, "main\n")
	}
	fmt.Fprintf(bout, "\n") // header ends with blank line
}

func startArchiveEntry(bout *bio.Writer) int64 {
	var arhdr [archive.HeaderSize]byte
	bout.Write(arhdr[:])
	return bout.Offset()
}

func finishArchiveEntry(bout *bio.Writer, start int64, name string) {
	bout.Flush()
	size := bout.Offset() - start
	if size&1 != 0 {
		bout.WriteByte(0)
	}
	bout.MustSeek(start-archive.HeaderSize, 0)

	var arhdr [archive.HeaderSize]byte
	archive.FormatHeader(arhdr[:], name, size)
	bout.Write(arhdr[:])
	bout.Flush()
	bout.MustSeek(start+size+(size&1), 0)
}

func dumpCompilerObj(bout *bio.Writer) {
	printObjHeader(bout)
	noder.WriteExports(bout)
}

func dumpdata() {
	reflectdata.WriteGCSymbols()
	reflectdata.WritePluginTable()
	dumpembeds()

	if reflectdata.ZeroSize > 0 {
		zero := base.PkgLinksym("go:map", "zero", obj.ABI0)
		objw.Global(zero, int32(reflectdata.ZeroSize), obj.DUPOK|obj.RODATA)
		zero.Set(obj.AttrStatic, true)
	}

	staticdata.WriteFuncSyms()
	addGCLocals()
}

func dumpLinkerObj(bout *bio.Writer) {
	printObjHeader(bout)

	if len(typecheck.Target.CgoPragmas) != 0 {
		// write empty export section; must be before cgo section
		fmt.Fprintf(bout, "\n$$\n\n$$\n\n")
		fmt.Fprintf(bout, "\n$$  // cgo\n")
		if err := json.NewEncoder(bout).Encode(typecheck.Target.CgoPragmas); err != nil {
			base.Fatalf("serializing pragcgobuf: %v", err)
		}
		fmt.Fprintf(bout, "\n$$\n\n")
	}

	fmt.Fprintf(bout, "\n!\n")

	obj.WriteObjFile(base.Ctxt, bout)
}

func dumpGlobal(n *ir.Name) {
	if n.Type() == nil {
		base.Fatalf("external %v nil type\n", n)
	}
	if n.Class == ir.PFUNC {
		return
	}
	if n.Sym().Pkg != types.LocalPkg {
		return
	}
	types.CalcSize(n.Type())
	ggloblnod(n)
	if n.CoverageAuxVar() || n.Linksym().Static() {
		return
	}
	base.Ctxt.DwarfGlobal(types.TypeSymName(n.Type()), n.Linksym())
}

func dumpGlobalConst(n *ir.Name) {
	// only export typed constants
	t := n.Type()
	if t == nil {
		return
	}
	if n.Sym().Pkg != types.LocalPkg {
		return
	}
	// only export integer constants for now
	if !t.IsInteger() {
		return
	}
	v := n.Val()
	if t.IsUntyped() {
		// Export untyped integers as int (if they fit).
		t = types.Types[types.TINT]
		if ir.ConstOverflow(v, t) {
			return
		}
	} else {
		// If the type of the constant is an instantiated generic, we need to emit
		// that type so the linker knows about it. See issue 51245.
		_ = reflectdata.TypeLinksym(t)
	}
	base.Ctxt.DwarfIntConst(n.Sym().Name, types.TypeSymName(t), ir.IntVal(t, v))
}

// addGCLocals adds gcargs, gclocals, gcregs, and stack object symbols to Ctxt.Data.
//
// This is done during the sequential phase after compilation, since
// global symbols can't be declared during parallel compilation.
func addGCLocals() {
	for _, s := range base.Ctxt.Text {
		fn := s.Func()
		if fn == nil {
			continue
		}
		for _, gcsym := range []*obj.LSym{fn.GCArgs, fn.GCLocals} {
			if gcsym != nil && !gcsym.OnList() {
				objw.Global(gcsym, int32(len(gcsym.P)), obj.RODATA|obj.DUPOK)
			}
		}
		if x := fn.StackObjects; x != nil {
			objw.Global(x, int32(len(x.P)), obj.RODATA)
			x.Set(obj.AttrStatic, true)
		}
		if x := fn.OpenCodedDeferInfo; x != nil {
			objw.Global(x, int32(len(x.P)), obj.RODATA|obj.DUPOK)
		}
		if x := fn.ArgInfo; x != nil {
			objw.Global(x, int32(len(x.P)), obj.RODATA|obj.DUPOK)
			x.Set(obj.AttrStatic, true)
		}
		if x := fn.ArgLiveInfo; x != nil {
			objw.Global(x, int32(len(x.P)), obj.RODATA|obj.DUPOK)
			x.Set(obj.AttrStatic, true)
		}
		if x := fn.WrapInfo; x != nil && !x.OnList() {
			objw.Global(x, int32(len(x.P)), obj.RODATA|obj.DUPOK)
			x.Set(obj.AttrStatic, true)
		}
		for _, jt := range fn.JumpTables {
			objw.Global(jt.Sym, int32(len(jt.Targets)*base.Ctxt.Arch.PtrSize), obj.RODATA)
		}
	}
}

func ggloblnod(nam *ir.Name) {
	s := nam.Linksym()

	// main_inittask and runtime_inittask in package runtime (and in
	// test/initempty.go) aren't real variable declarations, but
	// linknamed variables pointing to the compiler's generated
	// .inittask symbol. The real symbol was already written out in
	// pkginit.Task, so we need to avoid writing them out a second time
	// here, otherwise base.Ctxt.Globl will fail.
	if strings.HasSuffix(s.Name, "..inittask") && s.OnList() {
		return
	}

	s.Gotype = reflectdata.TypeLinksym(nam.Type())
	flags := 0
	if nam.Readonly() {
		flags = obj.RODATA
	}
	if nam.Type() != nil && !nam.Type().HasPointers() {
		flags |= obj.NOPTR
	}
	size := nam.Type().Size()
	linkname := nam.Sym().Linkname
	name := nam.Sym().Name

	var saveType objabi.SymKind
	if nam.CoverageAuxVar() {
		saveType = s.Type
	}

	// We've skipped linkname'd globals's instrument, so we can skip them here as well.
	if base.Flag.ASan && linkname == "" && pkginit.InstrumentGlobalsMap[name] != nil {
		// Write the new size of instrumented global variables that have
		// trailing redzones into object file.
		rzSize := pkginit.GetRedzoneSizeForGlobal(size)
		sizeWithRZ := rzSize + size
		base.Ctxt.Globl(s, sizeWithRZ, flags)
	} else {
		base.Ctxt.Globl(s, size, flags)
	}
	if nam.Libfuzzer8BitCounter() {
		s.Type = objabi.SLIBFUZZER_8BIT_COUNTER
	}
	if nam.CoverageAuxVar() && saveType == objabi.SCOVERAGE_COUNTER {
		// restore specialized counter type (which Globl call above overwrote)
		s.Type = saveType
	}
	if nam.Sym().Linkname != "" {
		// Make sure linkname'd symbol is non-package. When a symbol is
		// both imported and linkname'd, s.Pkg may not set to "_" in
		// types.Sym.Linksym because LSym already exists. Set it here.
		s.Pkg = "_"
	}
}

func dumpembeds() {
	for _, v := range typecheck.Target.Embeds {
		staticdata.WriteEmbed(v)
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/gc/util.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gc

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	tracepkg "runtime/trace"
	"strings"

	"cmd/compile/internal/base"
)

func profileName(fn, suffix string) string {
	if strings.HasSuffix(fn, string(os.PathSeparator)) {
		err := os.MkdirAll(fn, 0755)
		if err != nil {
			base.Fatalf("%v", err)
		}
	}
	if fi, statErr := os.Stat(fn); statErr == nil && fi.IsDir() {
		fn = filepath.Join(fn, url.PathEscape(base.Ctxt.Pkgpath)+suffix)
	}
	return fn
}

func startProfile() {
	if base.Flag.CPUProfile != "" {
		fn := profileName(base.Flag.CPUProfile, ".cpuprof")
		f, err := os.Create(fn)
		if err != nil {
			base.Fatalf("%v", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			base.Fatalf("%v", err)
		}
		base.AtExit(func() {
			pprof.StopCPUProfile()
			if err = f.Close(); err != nil {
				base.Fatalf("error closing cpu profile: %v", err)
			}
		})
	}
	if base.Flag.MemProfile != "" {
		if base.Flag.MemProfileRate != 0 {
			runtime.MemProfileRate = base.Flag.MemProfileRate
		}
		const (
			gzipFormat = 0
			textFormat = 1
		)
		// compilebench parses the memory profile to extract memstats,
		// which are only written in the legacy (text) pprof format.
		// See golang.org/issue/18641 and runtime/pprof/pprof.go:writeHeap.
		// gzipFormat is what most people want, otherwise
		var format = textFormat
		fn := base.Flag.MemProfile
		if strings.HasSuffix(fn, string(os.PathSeparator)) {
			err := os.MkdirAll(fn, 0755)
			if err != nil {
				base.Fatalf("%v", err)
			}
		}
		if fi, statErr := os.Stat(fn); statErr == nil && fi.IsDir() {
			fn = filepath.Join(fn, url.PathEscape(base.Ctxt.Pkgpath)+".memprof")
			format = gzipFormat
		}

		f, err := os.Create(fn)

		if err != nil {
			base.Fatalf("%v", err)
		}
		base.AtExit(func() {
			// Profile all outstanding allocations.
			runtime.GC()
			if err := pprof.Lookup("heap").WriteTo(f, format); err != nil {
				base.Fatalf("%v", err)
			}
			if err = f.Close(); err != nil {
				base.Fatalf("error closing memory profile: %v", err)
			}
		})
	} else {
		// Not doing memory profiling; disable it entirely.
		runtime.MemProfileRate = 0
	}
	if base.Flag.BlockProfile != "" {
		f, err := os.Create(profileName(base.Flag.BlockProfile, ".blockprof"))
		if err != nil {
			base.Fatalf("%v", err)
		}
		runtime.SetBlockProfileRate(1)
		base.AtExit(func() {
			pprof.Lookup("block").WriteTo(f, 0)
			f.Close()
		})
	}
	if base.Flag.MutexProfile != "" {
		f, err := os.Create(profileName(base.Flag.MutexProfile, ".mutexprof"))
		if err != nil {
			base.Fatalf("%v", err)
		}
		runtime.SetMutexProfileFraction(1)
		base.AtExit(func() {
			pprof.Lookup("mutex").WriteTo(f, 0)
			f.Close()
		})
	}
	if base.Flag.TraceProfile != "" {
		f, err := os.Create(profileName(base.Flag.TraceProfile, ".trace"))
		if err != nil {
			base.Fatalf("%v", err)
		}
		if err := tracepkg.Start(f); err != nil {
			base.Fatalf("%v", err)
		}
		base.AtExit(func() {
			tracepkg.Stop()
			if err = f.Close(); err != nil {
				base.Fatalf("error closing trace profile: %v", err)
			}
		})
	}
}

```

