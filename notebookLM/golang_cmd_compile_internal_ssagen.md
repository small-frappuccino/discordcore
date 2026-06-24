# Domain Architecture: cmd/compile/internal/ssagen

## Layout Topology
```text
cmd/compile/internal/ssagen/
├── abi.go
├── arch.go
├── intrinsics.go
├── nowb.go
├── pgen.go
├── phi.go
├── simdAMD64intrinsics.go
├── simdARM64intrinsics.go
├── simdWasmintrinsics.go
└── ssa.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/compile/internal/ssagen/abi.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssagen

import (
	"fmt"
	"internal/buildcfg"
	"log"
	"os"
	"strings"

	"cmd/compile/internal/abi"
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/obj/wasm"

	rtabi "internal/abi"
)

// SymABIs records information provided by the assembler about symbol
// definition ABIs and reference ABIs.
type SymABIs struct {
	defs map[string]obj.ABI
	refs map[string]obj.ABISet
}

func NewSymABIs() *SymABIs {
	return &SymABIs{
		defs: make(map[string]obj.ABI),
		refs: make(map[string]obj.ABISet),
	}
}

// canonicalize returns the canonical name used for a linker symbol in
// s's maps. Symbols in this package may be written either as "".X or
// with the package's import path already in the symbol. This rewrites
// both to use the full path, which matches compiler-generated linker
// symbol names.
func (s *SymABIs) canonicalize(linksym string) string {
	if strings.HasPrefix(linksym, `"".`) {
		panic("non-canonical symbol name: " + linksym)
	}
	return linksym
}

// ReadSymABIs reads a symabis file that specifies definitions and
// references of text symbols by ABI.
//
// The symabis format is a set of lines, where each line is a sequence
// of whitespace-separated fields. The first field is a verb and is
// either "def" for defining a symbol ABI or "ref" for referencing a
// symbol using an ABI. For both "def" and "ref", the second field is
// the symbol name and the third field is the ABI name, as one of the
// named cmd/internal/obj.ABI constants.
func (s *SymABIs) ReadSymABIs(file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("-symabis: %v", err)
	}

	for lineNum, line := range strings.Split(string(data), "\n") {
		lineNum++ // 1-based
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		switch parts[0] {
		case "def", "ref":
			// Parse line.
			if len(parts) != 3 {
				log.Fatalf(`%s:%d: invalid symabi: syntax is "%s sym abi"`, file, lineNum, parts[0])
			}
			sym, abistr := parts[1], parts[2]
			abi, valid := obj.ParseABI(abistr)
			if !valid {
				log.Fatalf(`%s:%d: invalid symabi: unknown abi "%s"`, file, lineNum, abistr)
			}

			sym = s.canonicalize(sym)

			// Record for later.
			if parts[0] == "def" {
				s.defs[sym] = abi
				base.Ctxt.DwTextCount++
			} else {
				s.refs[sym] |= obj.ABISetOf(abi)
			}
		default:
			log.Fatalf(`%s:%d: invalid symabi type "%s"`, file, lineNum, parts[0])
		}
	}
}

// HasDef returns whether the given symbol has an assembly definition.
func (s *SymABIs) HasDef(sym *types.Sym) bool {
	symName := sym.Linkname
	if symName == "" {
		symName = sym.Pkg.Prefix + "." + sym.Name
	}
	symName = s.canonicalize(symName)

	_, hasDefABI := s.defs[symName]
	return hasDefABI
}

// GenABIWrappers applies ABI information to Funcs and generates ABI
// wrapper functions where necessary.
func (s *SymABIs) GenABIWrappers() {
	// For cgo exported symbols, we tell the linker to export the
	// definition ABI to C. That also means that we don't want to
	// create ABI wrappers even if there's a linkname.
	//
	// TODO(austin): Maybe we want to create the ABI wrappers, but
	// ensure the linker exports the right ABI definition under
	// the unmangled name?
	cgoExports := make(map[string][]*[]string)
	for i, prag := range typecheck.Target.CgoPragmas {
		switch prag[0] {
		case "cgo_export_static", "cgo_export_dynamic":
			symName := s.canonicalize(prag[1])
			pprag := &typecheck.Target.CgoPragmas[i]
			cgoExports[symName] = append(cgoExports[symName], pprag)
		}
	}

	// Apply ABI defs and refs to Funcs and generate wrappers.
	//
	// This may generate new decls for the wrappers, but we
	// specifically *don't* want to visit those, lest we create
	// wrappers for wrappers.
	for _, fn := range typecheck.Target.Funcs {
		nam := fn.Nname
		if ir.IsBlank(nam) {
			continue
		}
		sym := nam.Sym()

		symName := sym.Linkname
		if symName == "" {
			symName = sym.Pkg.Prefix + "." + sym.Name
		}
		symName = s.canonicalize(symName)

		// Apply definitions.
		defABI, hasDefABI := s.defs[symName]
		if hasDefABI {
			if len(fn.Body) != 0 {
				base.ErrorfAt(fn.Pos(), 0, "%v defined in both Go and assembly", fn)
			}
			fn.ABI = defABI
		}

		if fn.Pragma&ir.CgoUnsafeArgs != 0 {
			// CgoUnsafeArgs indicates the function (or its callee) uses
			// offsets to dispatch arguments, which currently using ABI0
			// frame layout. Pin it to ABI0.
			fn.ABI = obj.ABI0
			// Propagate linkname attribute, which was set on the ABIInternal
			// symbol.
			if sym.Linksym().IsLinkname() {
				sym.LinksymABI(fn.ABI).Set(obj.AttrLinkname, true)
			}
			if sym.Linksym().IsLinknameStd() {
				sym.LinksymABI(fn.ABI).Set(obj.AttrLinknameStd, true)
			}
		}

		// If cgo-exported, add the definition ABI to the cgo
		// pragmas.
		cgoExport := cgoExports[symName]
		for _, pprag := range cgoExport {
			// The export pragmas have the form:
			//
			//   cgo_export_* <local> [<remote>]
			//
			// If <remote> is omitted, it's the same as
			// <local>.
			//
			// Expand to
			//
			//   cgo_export_* <local> <remote> <ABI>
			if len(*pprag) == 2 {
				*pprag = append(*pprag, (*pprag)[1])
			}
			// Add the ABI argument.
			*pprag = append(*pprag, fn.ABI.String())
		}

		// Apply references.
		if abis, ok := s.refs[symName]; ok {
			fn.ABIRefs |= abis
		}
		// Assume all functions are referenced at least as
		// ABIInternal, since they may be referenced from
		// other packages.
		fn.ABIRefs.Set(obj.ABIInternal, true)

		// If a symbol is defined in this package (either in
		// Go or assembly) and given a linkname, it may be
		// referenced from another package, so make it
		// callable via any ABI. It's important that we know
		// it's defined in this package since other packages
		// may "pull" symbols using linkname and we don't want
		// to create duplicate ABI wrappers.
		//
		// However, if it's given a linkname for exporting to
		// C, then we don't make ABI wrappers because the cgo
		// tool wants the original definition.
		hasBody := len(fn.Body) != 0
		if sym.Linkname != "" && (hasBody || hasDefABI) && len(cgoExport) == 0 {
			fn.ABIRefs |= obj.ABISetCallable
		}

		// Double check that cgo-exported symbols don't get
		// any wrappers.
		if len(cgoExport) > 0 && fn.ABIRefs&^obj.ABISetOf(fn.ABI) != 0 {
			base.Fatalf("cgo exported function %v cannot have ABI wrappers", fn)
		}

		if !buildcfg.Experiment.RegabiWrappers {
			continue
		}

		forEachWrapperABI(fn, makeABIWrapper)
	}
}

func forEachWrapperABI(fn *ir.Func, cb func(fn *ir.Func, wrapperABI obj.ABI)) {
	need := fn.ABIRefs &^ obj.ABISetOf(fn.ABI)
	if need == 0 {
		return
	}

	for wrapperABI := obj.ABI(0); wrapperABI < obj.ABICount; wrapperABI++ {
		if !need.Get(wrapperABI) {
			continue
		}
		cb(fn, wrapperABI)
	}
}

// makeABIWrapper creates a new function that will be called with
// wrapperABI and calls "f" using f.ABI.
func makeABIWrapper(f *ir.Func, wrapperABI obj.ABI) {
	if base.Debug.ABIWrap != 0 {
		fmt.Fprintf(os.Stderr, "=-= %v to %v wrapper for %v\n", wrapperABI, f.ABI, f)
	}

	// Q: is this needed?
	savepos := base.Pos
	savedcurfn := ir.CurFunc

	pos := base.AutogeneratedPos
	base.Pos = pos

	// At the moment we don't support wrapping a method, we'd need machinery
	// below to handle the receiver. Panic if we see this scenario.
	ft := f.Nname.Type()
	if ft.NumRecvs() != 0 {
		base.ErrorfAt(f.Pos(), 0, "makeABIWrapper support for wrapping methods not implemented")
		return
	}

	// Reuse f's types.Sym to create a new ODCLFUNC/function.
	// TODO(mdempsky): Means we can't set sym.Def in Declfunc, ugh.
	fn := ir.NewFunc(pos, pos, f.Sym(), types.NewSignature(nil,
		typecheck.NewFuncParams(ft.Params()),
		typecheck.NewFuncParams(ft.Results())))
	fn.ABI = wrapperABI
	typecheck.DeclFunc(fn)

	fn.SetABIWrapper(true)
	fn.SetDupok(true)

	// Propagate linkname attribute.
	fn.LinksymABI(fn.ABI).Set(obj.AttrLinkname, f.Linksym().IsLinkname())
	fn.LinksymABI(fn.ABI).Set(obj.AttrLinknameStd, f.Linksym().IsLinknameStd())

	// ABI0-to-ABIInternal wrappers will be mainly loading params from
	// stack into registers (and/or storing stack locations back to
	// registers after the wrapped call); in most cases they won't
	// need to allocate stack space, so it should be OK to mark them
	// as NOSPLIT in these cases. In addition, my assumption is that
	// functions written in assembly are NOSPLIT in most (but not all)
	// cases. In the case of an ABIInternal target that has too many
	// parameters to fit into registers, the wrapper would need to
	// allocate stack space, but this seems like an unlikely scenario.
	// Hence: mark these wrappers NOSPLIT.
	//
	// ABIInternal-to-ABI0 wrappers on the other hand will be taking
	// things in registers and pushing them onto the stack prior to
	// the ABI0 call, meaning that they will always need to allocate
	// stack space. If the compiler marks them as NOSPLIT this seems
	// as though it could lead to situations where the linker's
	// nosplit-overflow analysis would trigger a link failure. On the
	// other hand if they not tagged NOSPLIT then this could cause
	// problems when building the runtime (since there may be calls to
	// asm routine in cases where it's not safe to grow the stack). In
	// most cases the wrapper would be (in effect) inlined, but are
	// there (perhaps) indirect calls from the runtime that could run
	// into trouble here.
	// FIXME: at the moment all.bash does not pass when I leave out
	// NOSPLIT for these wrappers, so all are currently tagged with NOSPLIT.
	fn.Pragma |= ir.Nosplit

	// Generate call. Use tail call if no params and no returns,
	// but a regular call otherwise.
	//
	// Note: ideally we would be using a tail call in cases where
	// there are params but no returns for ABI0->ABIInternal wrappers,
	// provided that all params fit into registers (e.g. we don't have
	// to allocate any stack space). Doing this will require some
	// extra work in typecheck/walk/ssa, might want to add a new node
	// OTAILCALL or something to this effect.
	tailcall := fn.Type().NumResults() == 0 && fn.Type().NumParams() == 0 && fn.Type().NumRecvs() == 0
	if (base.Ctxt.Arch.Name == "ppc64le" || base.Ctxt.Arch.Name == "ppc64") && base.Ctxt.Flag_dynlink {
		// cannot tailcall on PPC64 with dynamic linking, as we need
		// to restore R2 after call.
		tailcall = false
	}
	if base.Ctxt.Arch.Name == "amd64" && wrapperABI == obj.ABIInternal {
		// cannot tailcall from ABIInternal to ABI0 on AMD64, as we need
		// to special registers (X15) when returning to ABIInternal.
		tailcall = false
	}

	var tail ir.Node
	call := ir.NewCallExpr(base.Pos, ir.OCALL, f.Nname, nil)
	call.Args = ir.ParamNames(fn.Type())
	call.IsDDD = fn.Type().IsVariadic()
	tail = call
	if tailcall {
		tail = ir.NewTailCallStmt(base.Pos, call)
	} else if fn.Type().NumResults() > 0 {
		n := ir.NewReturnStmt(base.Pos, nil)
		n.Results = []ir.Node{call}
		tail = n
	}
	fn.Body.Append(tail)

	typecheck.FinishFuncBody()

	ir.CurFunc = fn
	typecheck.Stmts(fn.Body)

	// Restore previous context.
	base.Pos = savepos
	ir.CurFunc = savedcurfn
}

// CreateWasmImportWrapper creates a wrapper for imported WASM functions to
// adapt them to the Go calling convention. The body for this function is
// generated in cmd/internal/obj/wasm/wasmobj.go
func CreateWasmImportWrapper(fn *ir.Func) bool {
	if fn.WasmImport == nil {
		return false
	}
	if buildcfg.GOARCH != "wasm" {
		base.FatalfAt(fn.Pos(), "CreateWasmImportWrapper call not supported on %s: func was %v", buildcfg.GOARCH, fn)
	}

	ir.InitLSym(fn, true)

	setupWasmImport(fn)

	pp := objw.NewProgs(fn, 0)
	defer pp.Free()
	pp.Text.To.Type = obj.TYPE_TEXTSIZE
	pp.Text.To.Val = int32(types.RoundUp(fn.Type().ArgWidth(), int64(types.RegSize)))
	// Wrapper functions never need their own stack frame
	pp.Text.To.Offset = 0
	pp.Flush()

	return true
}

func GenWasmExportWrapper(wrapped *ir.Func) {
	if wrapped.WasmExport == nil {
		return
	}
	if buildcfg.GOARCH != "wasm" {
		base.FatalfAt(wrapped.Pos(), "GenWasmExportWrapper call not supported on %s: func was %v", buildcfg.GOARCH, wrapped)
	}

	pos := base.AutogeneratedPos
	sym := &types.Sym{
		Name:     wrapped.WasmExport.Name,
		Linkname: wrapped.WasmExport.Name,
	}
	ft := wrapped.Nname.Type()
	fn := ir.NewFunc(pos, pos, sym, types.NewSignature(nil,
		typecheck.NewFuncParams(ft.Params()),
		typecheck.NewFuncParams(ft.Results())))
	fn.ABI = obj.ABI0 // actually wasm ABI
	// The wrapper function has a special calling convention that
	// morestack currently doesn't handle. For now we require that
	// the argument size fits in StackSmall, which we know we have
	// on stack, so we don't need to split stack.
	// cmd/internal/obj/wasm supports only 16 argument "registers"
	// anyway.
	if ft.ArgWidth() > rtabi.StackSmall {
		base.ErrorfAt(wrapped.Pos(), 0, "wasmexport function argument too large")
	}
	fn.Pragma |= ir.Nosplit

	ir.InitLSym(fn, true)

	setupWasmExport(fn, wrapped)

	pp := objw.NewProgs(fn, 0)
	defer pp.Free()
	// TEXT. Has a frame to pass args on stack to the Go function.
	pp.Text.To.Type = obj.TYPE_TEXTSIZE
	pp.Text.To.Val = int32(0)
	pp.Text.To.Offset = types.RoundUp(ft.ArgWidth(), int64(types.RegSize))
	// No locals. (Callee's args are covered in the callee's stackmap.)
	p := pp.Prog(obj.AFUNCDATA)
	p.From.SetConst(rtabi.FUNCDATA_LocalsPointerMaps)
	p.To.Type = obj.TYPE_MEM
	p.To.Name = obj.NAME_EXTERN
	p.To.Sym = base.Ctxt.Lookup("no_pointers_stackmap")
	pp.Flush()
	// Actual code geneneration is in cmd/internal/obj/wasm.
}

func paramsToWasmFields(f *ir.Func, pragma string, result *abi.ABIParamResultInfo, abiParams []abi.ABIParamAssignment) []obj.WasmField {
	wfs := make([]obj.WasmField, 0, len(abiParams))
	for _, p := range abiParams {
		t := p.Type
		var wt obj.WasmFieldType
		if t.IsSIMD() {
			wt = obj.WasmV128
		} else {
			switch t.Kind() {
			case types.TINT32, types.TUINT32:
				wt = obj.WasmI32
			case types.TINT64, types.TUINT64:
				wt = obj.WasmI64
			case types.TFLOAT32:
				wt = obj.WasmF32
			case types.TFLOAT64:
				wt = obj.WasmF64
			case types.TUNSAFEPTR, types.TUINTPTR:
				wt = obj.WasmPtr
			case types.TBOOL:
				wt = obj.WasmBool
			case types.TSTRING:
				// Two parts, (ptr, len)
				wt = obj.WasmPtr
				wfs = append(wfs, obj.WasmField{Type: wt, Offset: p.FrameOffset(result)})
				wfs = append(wfs, obj.WasmField{Type: wt, Offset: p.FrameOffset(result) + int64(types.PtrSize)})
				continue
			case types.TPTR:
				if wasmElemTypeAllowed(t.Elem()) {
					wt = obj.WasmPtr
					break
				}
				fallthrough
			default:
				base.ErrorfAt(f.Pos(), 0, "%s: unsupported parameter type %s", pragma, t.String())
			}
		}
		wfs = append(wfs, obj.WasmField{Type: wt, Offset: p.FrameOffset(result)})
	}
	return wfs
}

func resultsToWasmFields(f *ir.Func, pragma string, result *abi.ABIParamResultInfo, abiParams []abi.ABIParamAssignment) []obj.WasmField {
	if len(abiParams) > 1 {
		base.ErrorfAt(f.Pos(), 0, "%s: too many return values", pragma)
		return nil
	}
	wfs := make([]obj.WasmField, len(abiParams))
	for i, p := range abiParams {
		t := p.Type
		if t.IsSIMD() {
			wfs[i].Type = obj.WasmV128
		} else {
			switch t.Kind() {
			case types.TINT32, types.TUINT32:
				wfs[i].Type = obj.WasmI32
			case types.TINT64, types.TUINT64:
				wfs[i].Type = obj.WasmI64
			case types.TFLOAT32:
				wfs[i].Type = obj.WasmF32
			case types.TFLOAT64:
				wfs[i].Type = obj.WasmF64
			case types.TUNSAFEPTR, types.TUINTPTR:
				wfs[i].Type = obj.WasmPtr
			case types.TBOOL:
				wfs[i].Type = obj.WasmBool
			case types.TPTR:
				if wasmElemTypeAllowed(t.Elem()) {
					wfs[i].Type = obj.WasmPtr
					break
				}
				fallthrough
			default:
				base.ErrorfAt(f.Pos(), 0, "%s: unsupported result type %s", pragma, t.String())
			}
		}
		wfs[i].Offset = p.FrameOffset(result)
	}
	return wfs
}

// wasmElemTypeAllowed reports whether t is allowed to be passed in memory
// (as a pointer's element type, a field of it, etc.) between the Go wasm
// module and the host.
func wasmElemTypeAllowed(t *types.Type) bool {
	switch t.Kind() {
	case types.TINT8, types.TUINT8, types.TINT16, types.TUINT16,
		types.TINT32, types.TUINT32, types.TINT64, types.TUINT64,
		types.TFLOAT32, types.TFLOAT64, types.TBOOL:
		return true
	case types.TARRAY:
		return wasmElemTypeAllowed(t.Elem())
	case types.TSTRUCT:
		if len(t.Fields()) == 0 {
			return true
		}
		seenHostLayout := false
		for _, f := range t.Fields() {
			sym := f.Type.Sym()
			if sym != nil && sym.Name == "HostLayout" && sym.Pkg.Path == "structs" {
				seenHostLayout = true
				continue
			}
			if !wasmElemTypeAllowed(f.Type) {
				return false
			}
		}
		return seenHostLayout
	}
	// Pointer, and all pointerful types are not allowed, as pointers have
	// different width on the Go side and the host side. (It will be allowed
	// on GOARCH=wasm32.)
	return false
}

// setupWasmImport calculates the params and results in terms of WebAssembly values for the given function,
// and sets up the wasmimport metadata.
func setupWasmImport(f *ir.Func) {
	wi := obj.WasmImport{
		Module: f.WasmImport.Module,
		Name:   f.WasmImport.Name,
	}
	if wi.Module == wasm.GojsModule {
		// Functions that are imported from the "gojs" module use a special
		// ABI that just accepts the stack pointer.
		// Example:
		//
		// 	//go:wasmimport gojs add
		// 	func importedAdd(a, b uint) uint
		//
		// will roughly become
		//
		// 	(import "gojs" "add" (func (param i32)))
		wi.Params = []obj.WasmField{{Type: obj.WasmI32}}
	} else {
		// All other imported functions use the normal WASM ABI.
		// Example:
		//
		// 	//go:wasmimport a_module add
		// 	func importedAdd(a, b uint) uint
		//
		// will roughly become
		//
		// 	(import "a_module" "add" (func (param i32 i32) (result i32)))
		abiConfig := AbiForBodylessFuncStackMap(f)
		abiInfo := abiConfig.ABIAnalyzeFuncType(f.Type())
		wi.Params = paramsToWasmFields(f, "go:wasmimport", abiInfo, abiInfo.InParams())
		wi.Results = resultsToWasmFields(f, "go:wasmimport", abiInfo, abiInfo.OutParams())
	}
	f.LSym.Func().WasmImport = &wi
}

// setupWasmExport calculates the params and results in terms of WebAssembly values for the given function,
// and sets up the wasmexport metadata.
func setupWasmExport(f, wrapped *ir.Func) {
	we := obj.WasmExport{
		WrappedSym: wrapped.LSym,
	}
	abiConfig := AbiForBodylessFuncStackMap(wrapped)
	abiInfo := abiConfig.ABIAnalyzeFuncType(wrapped.Type())
	we.Params = paramsToWasmFields(wrapped, "go:wasmexport", abiInfo, abiInfo.InParams())
	we.Results = resultsToWasmFields(wrapped, "go:wasmexport", abiInfo, abiInfo.OutParams())
	f.LSym.Func().WasmExport = &we
}

```

// === FILE: references/go/src/cmd/compile/internal/ssagen/arch.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssagen

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
)

var Arch ArchInfo

// interface to back end

type ArchInfo struct {
	LinkArch *obj.LinkArch

	REGSP     int
	MAXWIDTH  int64
	SoftFloat bool

	PadFrame func(int64) int64

	// ZeroRange zeroes a range of memory the on stack.
	//  - it is only called at function entry
	//  - it is ok to clobber (non-arg) registers.
	//  - currently used only for small things, so it can be simple.
	//    - pointers to heap-allocated return values
	//    - open-coded deferred functions
	// (Max size in make.bash is 40 bytes.)
	ZeroRange func(*objw.Progs, *obj.Prog, int64, int64, *uint32) *obj.Prog

	Ginsnop func(*objw.Progs) *obj.Prog

	// SSAMarkMoves marks any MOVXconst ops that need to avoid clobbering flags.
	SSAMarkMoves func(*State, *ssa.Block)

	// SSAGenValue emits Prog(s) for the Value.
	SSAGenValue func(*State, *ssa.Value)

	// SSAGenBlock emits end-of-block Progs. SSAGenValue should be called
	// for all values in the block before SSAGenBlock.
	SSAGenBlock func(s *State, b, next *ssa.Block)

	// LoadRegResult emits instructions that loads register-assigned result
	// at n+off (n is PPARAMOUT) to register reg. The result is already in
	// memory. Used in open-coded defer return path.
	LoadRegResult func(s *State, f *ssa.Func, t *types.Type, reg int16, n *ir.Name, off int64) *obj.Prog

	// SpillArgReg emits instructions that spill reg to n+off.
	SpillArgReg func(pp *objw.Progs, p *obj.Prog, f *ssa.Func, t *types.Type, reg int16, n *ir.Name, off int64) *obj.Prog
}

```

// === FILE: references/go/src/cmd/compile/internal/ssagen/intrinsics.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssagen

import (
	"fmt"
	"internal/abi"
	"internal/buildcfg"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/sys"
)

var intrinsics intrinsicBuilders

// An intrinsicBuilder converts a call node n into an ssa value that
// implements that call as an intrinsic. args is a list of arguments to the func.
type intrinsicBuilder func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value

type intrinsicKey struct {
	arch *sys.Arch
	pkg  string
	fn   string
}

// intrinsicBuildConfig specifies the config to use for intrinsic building.
type intrinsicBuildConfig struct {
	instrumenting bool

	go386     string
	goamd64   int
	goarm     buildcfg.GoarmFeatures
	goarm64   buildcfg.Goarm64Features
	gomips    string
	gomips64  string
	goppc64   int
	goriscv64 int
}

type intrinsicBuilders map[intrinsicKey]intrinsicBuilder

// add adds the intrinsic builder b for pkg.fn for the given architecture.
func (ib intrinsicBuilders) add(arch *sys.Arch, pkg, fn string, b intrinsicBuilder) {
	if _, found := ib[intrinsicKey{arch, pkg, fn}]; found {
		panic(fmt.Sprintf("intrinsic already exists for %v.%v on %v", pkg, fn, arch.Name))
	}
	ib[intrinsicKey{arch, pkg, fn}] = b
}

// addForArchs adds the intrinsic builder b for pkg.fn for the given architectures.
func (ib intrinsicBuilders) addForArchs(pkg, fn string, b intrinsicBuilder, archs ...*sys.Arch) {
	for _, arch := range archs {
		ib.add(arch, pkg, fn, b)
	}
}

// addForFamilies does the same as addForArchs but operates on architecture families.
func (ib intrinsicBuilders) addForFamilies(pkg, fn string, b intrinsicBuilder, archFamilies ...sys.ArchFamily) {
	for _, arch := range sys.Archs {
		if arch.InFamily(archFamilies...) {
			intrinsics.add(arch, pkg, fn, b)
		}
	}
}

// alias aliases pkg.fn to targetPkg.targetFn for all architectures in archs
// for which targetPkg.targetFn already exists.
func (ib intrinsicBuilders) alias(pkg, fn, targetPkg, targetFn string, archs ...*sys.Arch) {
	// TODO(jsing): Consider making this work even if the alias is added
	// before the intrinsic.
	aliased := false
	for _, arch := range archs {
		if b := intrinsics.lookup(arch, targetPkg, targetFn); b != nil {
			intrinsics.add(arch, pkg, fn, b)
			aliased = true
		}
	}
	if !aliased {
		panic(fmt.Sprintf("attempted to alias undefined intrinsic: %s.%s", pkg, fn))
	}
}

// lookup looks up the intrinsic for a pkg.fn on the specified architecture.
func (ib intrinsicBuilders) lookup(arch *sys.Arch, pkg, fn string) intrinsicBuilder {
	return intrinsics[intrinsicKey{arch, pkg, fn}]
}

func initIntrinsics(cfg *intrinsicBuildConfig) {
	if cfg == nil {
		cfg = &intrinsicBuildConfig{
			instrumenting: base.Flag.Cfg.Instrumenting,
			go386:         buildcfg.GO386,
			goamd64:       buildcfg.GOAMD64,
			goarm:         buildcfg.GOARM,
			goarm64:       buildcfg.GOARM64,
			gomips:        buildcfg.GOMIPS,
			gomips64:      buildcfg.GOMIPS64,
			goppc64:       buildcfg.GOPPC64,
			goriscv64:     buildcfg.GORISCV64,
		}
	}
	intrinsics = intrinsicBuilders{}

	var p4 []*sys.Arch
	var p8 []*sys.Arch
	var lwatomics []*sys.Arch
	for _, a := range sys.Archs {
		if a.PtrSize == 4 {
			p4 = append(p4, a)
		} else {
			p8 = append(p8, a)
		}
		if a.Family != sys.PPC64 {
			lwatomics = append(lwatomics, a)
		}
	}
	all := sys.Archs[:]

	add := func(pkg, fn string, b intrinsicBuilder, archs ...*sys.Arch) {
		intrinsics.addForArchs(pkg, fn, b, archs...)
	}
	addF := func(pkg, fn string, b intrinsicBuilder, archFamilies ...sys.ArchFamily) {
		intrinsics.addForFamilies(pkg, fn, b, archFamilies...)
	}
	alias := func(pkg, fn, pkg2, fn2 string, archs ...*sys.Arch) {
		intrinsics.alias(pkg, fn, pkg2, fn2, archs...)
	}

	/******** runtime ********/
	if !cfg.instrumenting {
		add("runtime", "slicebytetostringtmp",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				// Compiler frontend optimizations emit OBYTES2STRTMP nodes
				// for the backend instead of slicebytetostringtmp calls
				// when not instrumenting.
				return s.newValue2(ssa.OpStringMake, n.Type(), args[0], args[1])
			},
			all...)
	}
	addF("internal/runtime/math", "MulUintptr",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			if s.config.PtrSize == 4 {
				return s.newValue2(ssa.OpMul32uover, types.NewTuple(types.Types[types.TUINT], types.Types[types.TUINT]), args[0], args[1])
			}
			return s.newValue2(ssa.OpMul64uover, types.NewTuple(types.Types[types.TUINT], types.Types[types.TUINT]), args[0], args[1])
		},
		sys.AMD64, sys.I386, sys.Loong64, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.ARM64)
	add("runtime", "KeepAlive",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			data := s.newValue1(ssa.OpIData, s.f.Config.Types.BytePtr, args[0])
			s.vars[memVar] = s.newValue2(ssa.OpKeepAlive, types.TypeMem, data, s.mem())
			return nil
		},
		all...)

	addF("runtime", "publicationBarrier",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue1(ssa.OpPubBarrier, types.TypeMem, s.mem())
			return nil
		},
		sys.ARM64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64)

	/******** internal/runtime/sys ********/
	add("internal/runtime/sys", "GetCallerPC",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue0(ssa.OpGetCallerPC, s.f.Config.Types.Uintptr)
		},
		all...)

	add("internal/runtime/sys", "GetCallerSP",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpGetCallerSP, s.f.Config.Types.Uintptr, s.mem())
		},
		all...)

	add("internal/runtime/sys", "GetClosurePtr",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue0(ssa.OpGetClosurePtr, s.f.Config.Types.Uintptr)
		},
		all...)

	addF("internal/runtime/sys", "Bswap32",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBswap32, types.Types[types.TUINT32], args[0])
		},
		sys.AMD64, sys.I386, sys.ARM64, sys.ARM, sys.Loong64, sys.S390X)
	addF("internal/runtime/sys", "Bswap64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBswap64, types.Types[types.TUINT64], args[0])
		},
		sys.AMD64, sys.I386, sys.ARM64, sys.ARM, sys.Loong64, sys.S390X)

	addF("runtime", "memequal",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue4(ssa.OpMemEq, s.f.Config.Types.Bool, args[0], args[1], args[2], s.mem())
		},
		sys.ARM64)

	if cfg.goppc64 >= 10 {
		// Use only on Power10 as the new byte reverse instructions that Power10 provide
		// make it worthwhile as an intrinsic
		addF("internal/runtime/sys", "Bswap32",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpBswap32, types.Types[types.TUINT32], args[0])
			},
			sys.PPC64)
		addF("internal/runtime/sys", "Bswap64",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpBswap64, types.Types[types.TUINT64], args[0])
			},
			sys.PPC64)
	}

	if cfg.goriscv64 >= 22 {
		addF("internal/runtime/sys", "Bswap32",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpBswap32, types.Types[types.TUINT32], args[0])
			},
			sys.RISCV64)
		addF("internal/runtime/sys", "Bswap64",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpBswap64, types.Types[types.TUINT64], args[0])
			},
			sys.RISCV64)
	}

	/****** Prefetch ******/
	makePrefetchFunc := func(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue2(op, types.TypeMem, args[0], s.mem())
			return nil
		}
	}

	// Make Prefetch intrinsics for supported platforms
	// On the unsupported platforms stub function will be eliminated
	addF("internal/runtime/sys", "Prefetch", makePrefetchFunc(ssa.OpPrefetchCache),
		sys.AMD64, sys.ARM64, sys.Loong64, sys.PPC64)
	addF("internal/runtime/sys", "PrefetchStreamed", makePrefetchFunc(ssa.OpPrefetchCacheStreamed),
		sys.AMD64, sys.ARM64, sys.Loong64, sys.PPC64)

	/******** internal/runtime/atomic ********/
	type atomicOpEmitter func(s *state, n *ir.CallExpr, args []*ssa.Value, op ssa.Op, typ types.Kind, needReturn bool)

	addF("internal/runtime/atomic", "Load",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue2(ssa.OpAtomicLoad32, types.NewTuple(types.Types[types.TUINT32], types.TypeMem), args[0], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TUINT32], v)
		},
		sys.AMD64, sys.ARM64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "Load8",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue2(ssa.OpAtomicLoad8, types.NewTuple(types.Types[types.TUINT8], types.TypeMem), args[0], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TUINT8], v)
		},
		sys.AMD64, sys.ARM64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "Load64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue2(ssa.OpAtomicLoad64, types.NewTuple(types.Types[types.TUINT64], types.TypeMem), args[0], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TUINT64], v)
		},
		sys.AMD64, sys.ARM64, sys.Loong64, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "LoadAcq",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue2(ssa.OpAtomicLoadAcq32, types.NewTuple(types.Types[types.TUINT32], types.TypeMem), args[0], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TUINT32], v)
		},
		sys.PPC64)
	addF("internal/runtime/atomic", "LoadAcq64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue2(ssa.OpAtomicLoadAcq64, types.NewTuple(types.Types[types.TUINT64], types.TypeMem), args[0], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TUINT64], v)
		},
		sys.PPC64)
	addF("internal/runtime/atomic", "Loadp",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue2(ssa.OpAtomicLoadPtr, types.NewTuple(s.f.Config.Types.BytePtr, types.TypeMem), args[0], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, s.f.Config.Types.BytePtr, v)
		},
		sys.AMD64, sys.ARM64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)

	addF("internal/runtime/atomic", "Store",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue3(ssa.OpAtomicStore32, types.TypeMem, args[0], args[1], s.mem())
			return nil
		},
		sys.AMD64, sys.ARM64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "Store8",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue3(ssa.OpAtomicStore8, types.TypeMem, args[0], args[1], s.mem())
			return nil
		},
		sys.AMD64, sys.ARM64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "Store64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue3(ssa.OpAtomicStore64, types.TypeMem, args[0], args[1], s.mem())
			return nil
		},
		sys.AMD64, sys.ARM64, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "StorepNoWB",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue3(ssa.OpAtomicStorePtrNoWB, types.TypeMem, args[0], args[1], s.mem())
			return nil
		},
		sys.AMD64, sys.ARM64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "StoreRel",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue3(ssa.OpAtomicStoreRel32, types.TypeMem, args[0], args[1], s.mem())
			return nil
		},
		sys.PPC64)
	addF("internal/runtime/atomic", "StoreRel64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue3(ssa.OpAtomicStoreRel64, types.TypeMem, args[0], args[1], s.mem())
			return nil
		},
		sys.PPC64)

	makeAtomicStoreGuardedIntrinsicLoong64 := func(op0, op1 ssa.Op, typ types.Kind, emit atomicOpEmitter) intrinsicBuilder {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			// Target Atomic feature is identified by dynamic detection
			addr := s.entryNewValue1A(ssa.OpAddr, types.Types[types.TBOOL].PtrTo(), ir.Syms.Loong64HasDBAR_HINTS, s.sb)
			v := s.load(types.Types[types.TBOOL], addr)
			b := s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(v)
			bTrue := s.f.NewBlock(ssa.BlockPlain)
			bFalse := s.f.NewBlock(ssa.BlockPlain)
			bEnd := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(bTrue)
			b.AddEdgeTo(bFalse)
			b.Likely = ssa.BranchLikely

			// most loong64 machines support the finer-grained DBAR hints
			s.startBlock(bTrue)
			emit(s, n, args, op0, typ, false)
			s.endBlock().AddEdgeTo(bEnd)

			// Use original instruction sequence.
			s.startBlock(bFalse)
			emit(s, n, args, op1, typ, false)
			s.endBlock().AddEdgeTo(bEnd)

			// Merge results.
			s.startBlock(bEnd)

			return nil
		}
	}

	atomicStoreEmitterLoong64 := func(s *state, n *ir.CallExpr, args []*ssa.Value, op ssa.Op, typ types.Kind, needReturn bool) {
		v := s.newValue3(op, types.NewTuple(types.Types[typ], types.TypeMem), args[0], args[1], s.mem())
		s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
		if needReturn {
			s.vars[n] = s.newValue1(ssa.OpSelect0, types.Types[typ], v)
		}
	}

	addF("internal/runtime/atomic", "Store",
		makeAtomicStoreGuardedIntrinsicLoong64(ssa.OpAtomicStore32, ssa.OpAtomicStore32Variant, types.TUINT8, atomicStoreEmitterLoong64),
		sys.Loong64)
	addF("internal/runtime/atomic", "Store64",
		makeAtomicStoreGuardedIntrinsicLoong64(ssa.OpAtomicStore64, ssa.OpAtomicStore64Variant, types.TUINT8, atomicStoreEmitterLoong64),
		sys.Loong64)

	addF("internal/runtime/atomic", "Xchg8",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue3(ssa.OpAtomicExchange8, types.NewTuple(types.Types[types.TUINT8], types.TypeMem), args[0], args[1], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TUINT8], v)
		},
		sys.AMD64, sys.PPC64)
	addF("internal/runtime/atomic", "Xchg",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue3(ssa.OpAtomicExchange32, types.NewTuple(types.Types[types.TUINT32], types.TypeMem), args[0], args[1], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TUINT32], v)
		},
		sys.AMD64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "Xchg64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue3(ssa.OpAtomicExchange64, types.NewTuple(types.Types[types.TUINT64], types.TypeMem), args[0], args[1], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TUINT64], v)
		},
		sys.AMD64, sys.Loong64, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)

	makeAtomicGuardedIntrinsicARM64common := func(op0, op1 ssa.Op, typ types.Kind, emit atomicOpEmitter, needReturn bool) intrinsicBuilder {

		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			if cfg.goarm64.LSE {
				emit(s, n, args, op1, typ, needReturn)
			} else {
				// Target Atomic feature is identified by dynamic detection
				addr := s.entryNewValue1A(ssa.OpAddr, types.Types[types.TBOOL].PtrTo(), ir.Syms.ARM64HasATOMICS, s.sb)
				v := s.load(types.Types[types.TBOOL], addr)
				b := s.endBlock()
				b.Kind = ssa.BlockIf
				b.SetControl(v)
				bTrue := s.f.NewBlock(ssa.BlockPlain)
				bFalse := s.f.NewBlock(ssa.BlockPlain)
				bEnd := s.f.NewBlock(ssa.BlockPlain)
				b.AddEdgeTo(bTrue)
				b.AddEdgeTo(bFalse)
				b.Likely = ssa.BranchLikely

				// We have atomic instructions - use it directly.
				s.startBlock(bTrue)
				emit(s, n, args, op1, typ, needReturn)
				s.endBlock().AddEdgeTo(bEnd)

				// Use original instruction sequence.
				s.startBlock(bFalse)
				emit(s, n, args, op0, typ, needReturn)
				s.endBlock().AddEdgeTo(bEnd)

				// Merge results.
				s.startBlock(bEnd)
			}
			if needReturn {
				return s.variable(n, types.Types[typ])
			} else {
				return nil
			}
		}
	}
	makeAtomicGuardedIntrinsicARM64 := func(op0, op1 ssa.Op, typ types.Kind, emit atomicOpEmitter) intrinsicBuilder {
		return makeAtomicGuardedIntrinsicARM64common(op0, op1, typ, emit, true)
	}
	makeAtomicGuardedIntrinsicARM64old := func(op0, op1 ssa.Op, typ types.Kind, emit atomicOpEmitter) intrinsicBuilder {
		return makeAtomicGuardedIntrinsicARM64common(op0, op1, typ, emit, false)
	}

	atomicEmitterARM64 := func(s *state, n *ir.CallExpr, args []*ssa.Value, op ssa.Op, typ types.Kind, needReturn bool) {
		v := s.newValue3(op, types.NewTuple(types.Types[typ], types.TypeMem), args[0], args[1], s.mem())
		s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
		if needReturn {
			s.vars[n] = s.newValue1(ssa.OpSelect0, types.Types[typ], v)
		}
	}
	addF("internal/runtime/atomic", "Xchg8",
		makeAtomicGuardedIntrinsicARM64(ssa.OpAtomicExchange8, ssa.OpAtomicExchange8Variant, types.TUINT8, atomicEmitterARM64),
		sys.ARM64)
	addF("internal/runtime/atomic", "Xchg",
		makeAtomicGuardedIntrinsicARM64(ssa.OpAtomicExchange32, ssa.OpAtomicExchange32Variant, types.TUINT32, atomicEmitterARM64),
		sys.ARM64)
	addF("internal/runtime/atomic", "Xchg64",
		makeAtomicGuardedIntrinsicARM64(ssa.OpAtomicExchange64, ssa.OpAtomicExchange64Variant, types.TUINT64, atomicEmitterARM64),
		sys.ARM64)

	makeAtomicXchg8GuardedIntrinsicLoong64 := func(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			addr := s.entryNewValue1A(ssa.OpAddr, types.Types[types.TBOOL].PtrTo(), ir.Syms.Loong64HasLAM_BH, s.sb)
			v := s.load(types.Types[types.TBOOL], addr)
			b := s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(v)
			bTrue := s.f.NewBlock(ssa.BlockPlain)
			bFalse := s.f.NewBlock(ssa.BlockPlain)
			bEnd := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(bTrue)
			b.AddEdgeTo(bFalse)
			b.Likely = ssa.BranchLikely // most loong64 machines support the amswapdb.b

			// We have the intrinsic - use it directly.
			s.startBlock(bTrue)
			s.vars[n] = s.newValue3(op, types.NewTuple(types.Types[types.TUINT8], types.TypeMem), args[0], args[1], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, s.vars[n])
			s.vars[n] = s.newValue1(ssa.OpSelect0, types.Types[types.TUINT8], s.vars[n])
			s.endBlock().AddEdgeTo(bEnd)

			// Call the pure Go version.
			s.startBlock(bFalse)
			s.vars[n] = s.callResult(n, callNormal) // types.Types[TUINT8]
			s.endBlock().AddEdgeTo(bEnd)

			// Merge results.
			s.startBlock(bEnd)
			return s.variable(n, types.Types[types.TUINT8])
		}
	}
	addF("internal/runtime/atomic", "Xchg8",
		makeAtomicXchg8GuardedIntrinsicLoong64(ssa.OpAtomicExchange8Variant),
		sys.Loong64)

	addF("internal/runtime/atomic", "Xadd",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue3(ssa.OpAtomicAdd32, types.NewTuple(types.Types[types.TUINT32], types.TypeMem), args[0], args[1], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TUINT32], v)
		},
		sys.AMD64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "Xadd64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue3(ssa.OpAtomicAdd64, types.NewTuple(types.Types[types.TUINT64], types.TypeMem), args[0], args[1], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TUINT64], v)
		},
		sys.AMD64, sys.Loong64, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)

	addF("internal/runtime/atomic", "Xadd",
		makeAtomicGuardedIntrinsicARM64(ssa.OpAtomicAdd32, ssa.OpAtomicAdd32Variant, types.TUINT32, atomicEmitterARM64),
		sys.ARM64)
	addF("internal/runtime/atomic", "Xadd64",
		makeAtomicGuardedIntrinsicARM64(ssa.OpAtomicAdd64, ssa.OpAtomicAdd64Variant, types.TUINT64, atomicEmitterARM64),
		sys.ARM64)

	addF("internal/runtime/atomic", "Cas",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue4(ssa.OpAtomicCompareAndSwap32, types.NewTuple(types.Types[types.TBOOL], types.TypeMem), args[0], args[1], args[2], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TBOOL], v)
		},
		sys.AMD64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "Cas64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue4(ssa.OpAtomicCompareAndSwap64, types.NewTuple(types.Types[types.TBOOL], types.TypeMem), args[0], args[1], args[2], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TBOOL], v)
		},
		sys.AMD64, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "CasRel",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue4(ssa.OpAtomicCompareAndSwap32, types.NewTuple(types.Types[types.TBOOL], types.TypeMem), args[0], args[1], args[2], s.mem())
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
			return s.newValue1(ssa.OpSelect0, types.Types[types.TBOOL], v)
		},
		sys.PPC64)

	atomicCasEmitterARM64 := func(s *state, n *ir.CallExpr, args []*ssa.Value, op ssa.Op, typ types.Kind, needReturn bool) {
		v := s.newValue4(op, types.NewTuple(types.Types[types.TBOOL], types.TypeMem), args[0], args[1], args[2], s.mem())
		s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
		if needReturn {
			s.vars[n] = s.newValue1(ssa.OpSelect0, types.Types[typ], v)
		}
	}

	addF("internal/runtime/atomic", "Cas",
		makeAtomicGuardedIntrinsicARM64(ssa.OpAtomicCompareAndSwap32, ssa.OpAtomicCompareAndSwap32Variant, types.TBOOL, atomicCasEmitterARM64),
		sys.ARM64)
	addF("internal/runtime/atomic", "Cas64",
		makeAtomicGuardedIntrinsicARM64(ssa.OpAtomicCompareAndSwap64, ssa.OpAtomicCompareAndSwap64Variant, types.TBOOL, atomicCasEmitterARM64),
		sys.ARM64)

	atomicCasEmitterLoong64 := func(s *state, n *ir.CallExpr, args []*ssa.Value, op ssa.Op, typ types.Kind, needReturn bool) {
		v := s.newValue4(op, types.NewTuple(types.Types[types.TBOOL], types.TypeMem), args[0], args[1], args[2], s.mem())
		s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, v)
		if needReturn {
			s.vars[n] = s.newValue1(ssa.OpSelect0, types.Types[typ], v)
		}
	}

	makeAtomicCasGuardedIntrinsicLoong64 := func(op0, op1 ssa.Op, emit atomicOpEmitter) intrinsicBuilder {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			// Target Atomic feature is identified by dynamic detection
			addr := s.entryNewValue1A(ssa.OpAddr, types.Types[types.TBOOL].PtrTo(), ir.Syms.Loong64HasLAMCAS, s.sb)
			v := s.load(types.Types[types.TBOOL], addr)
			b := s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(v)
			bTrue := s.f.NewBlock(ssa.BlockPlain)
			bFalse := s.f.NewBlock(ssa.BlockPlain)
			bEnd := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(bTrue)
			b.AddEdgeTo(bFalse)
			b.Likely = ssa.BranchLikely

			// We have atomic instructions - use it directly.
			s.startBlock(bTrue)
			emit(s, n, args, op1, types.TBOOL, true)
			s.endBlock().AddEdgeTo(bEnd)

			// Use original instruction sequence.
			s.startBlock(bFalse)
			emit(s, n, args, op0, types.TBOOL, true)
			s.endBlock().AddEdgeTo(bEnd)

			// Merge results.
			s.startBlock(bEnd)

			return s.variable(n, types.Types[types.TBOOL])
		}
	}

	addF("internal/runtime/atomic", "Cas",
		makeAtomicCasGuardedIntrinsicLoong64(ssa.OpAtomicCompareAndSwap32, ssa.OpAtomicCompareAndSwap32Variant, atomicCasEmitterLoong64),
		sys.Loong64)
	addF("internal/runtime/atomic", "Cas64",
		makeAtomicCasGuardedIntrinsicLoong64(ssa.OpAtomicCompareAndSwap64, ssa.OpAtomicCompareAndSwap64Variant, atomicCasEmitterLoong64),
		sys.Loong64)

	// Old-style atomic logical operation API (all supported archs except arm64).
	addF("internal/runtime/atomic", "And8",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue3(ssa.OpAtomicAnd8, types.TypeMem, args[0], args[1], s.mem())
			return nil
		},
		sys.AMD64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "And",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue3(ssa.OpAtomicAnd32, types.TypeMem, args[0], args[1], s.mem())
			return nil
		},
		sys.AMD64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "Or8",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue3(ssa.OpAtomicOr8, types.TypeMem, args[0], args[1], s.mem())
			return nil
		},
		sys.AMD64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("internal/runtime/atomic", "Or",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			s.vars[memVar] = s.newValue3(ssa.OpAtomicOr32, types.TypeMem, args[0], args[1], s.mem())
			return nil
		},
		sys.AMD64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X)

	// arm64 always uses the new-style atomic logical operations, for both the
	// old and new style API.
	addF("internal/runtime/atomic", "And8",
		makeAtomicGuardedIntrinsicARM64old(ssa.OpAtomicAnd8value, ssa.OpAtomicAnd8valueVariant, types.TUINT8, atomicEmitterARM64),
		sys.ARM64)
	addF("internal/runtime/atomic", "Or8",
		makeAtomicGuardedIntrinsicARM64old(ssa.OpAtomicOr8value, ssa.OpAtomicOr8valueVariant, types.TUINT8, atomicEmitterARM64),
		sys.ARM64)
	addF("internal/runtime/atomic", "And64",
		makeAtomicGuardedIntrinsicARM64(ssa.OpAtomicAnd64value, ssa.OpAtomicAnd64valueVariant, types.TUINT64, atomicEmitterARM64),
		sys.ARM64)
	addF("internal/runtime/atomic", "And32",
		makeAtomicGuardedIntrinsicARM64(ssa.OpAtomicAnd32value, ssa.OpAtomicAnd32valueVariant, types.TUINT32, atomicEmitterARM64),
		sys.ARM64)
	addF("internal/runtime/atomic", "And",
		makeAtomicGuardedIntrinsicARM64old(ssa.OpAtomicAnd32value, ssa.OpAtomicAnd32valueVariant, types.TUINT32, atomicEmitterARM64),
		sys.ARM64)
	addF("internal/runtime/atomic", "Or64",
		makeAtomicGuardedIntrinsicARM64(ssa.OpAtomicOr64value, ssa.OpAtomicOr64valueVariant, types.TUINT64, atomicEmitterARM64),
		sys.ARM64)
	addF("internal/runtime/atomic", "Or32",
		makeAtomicGuardedIntrinsicARM64(ssa.OpAtomicOr32value, ssa.OpAtomicOr32valueVariant, types.TUINT32, atomicEmitterARM64),
		sys.ARM64)
	addF("internal/runtime/atomic", "Or",
		makeAtomicGuardedIntrinsicARM64old(ssa.OpAtomicOr32value, ssa.OpAtomicOr32valueVariant, types.TUINT32, atomicEmitterARM64),
		sys.ARM64)

	// New-style atomic logical operations, which return the old memory value.
	addF("internal/runtime/atomic", "And64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue3(ssa.OpAtomicAnd64value, types.NewTuple(types.Types[types.TUINT64], types.TypeMem), args[0], args[1], s.mem())
			p0, p1 := s.split(v)
			s.vars[memVar] = p1
			return p0
		},
		sys.AMD64, sys.Loong64)
	addF("internal/runtime/atomic", "And32",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue3(ssa.OpAtomicAnd32value, types.NewTuple(types.Types[types.TUINT32], types.TypeMem), args[0], args[1], s.mem())
			p0, p1 := s.split(v)
			s.vars[memVar] = p1
			return p0
		},
		sys.AMD64, sys.Loong64)
	addF("internal/runtime/atomic", "Or64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue3(ssa.OpAtomicOr64value, types.NewTuple(types.Types[types.TUINT64], types.TypeMem), args[0], args[1], s.mem())
			p0, p1 := s.split(v)
			s.vars[memVar] = p1
			return p0
		},
		sys.AMD64, sys.Loong64)
	addF("internal/runtime/atomic", "Or32",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v := s.newValue3(ssa.OpAtomicOr32value, types.NewTuple(types.Types[types.TUINT32], types.TypeMem), args[0], args[1], s.mem())
			p0, p1 := s.split(v)
			s.vars[memVar] = p1
			return p0
		},
		sys.AMD64, sys.Loong64)

	// Aliases for atomic load operations
	alias("internal/runtime/atomic", "Loadint32", "internal/runtime/atomic", "Load", all...)
	alias("internal/runtime/atomic", "Loadint64", "internal/runtime/atomic", "Load64", all...)
	alias("internal/runtime/atomic", "Loaduintptr", "internal/runtime/atomic", "Load", p4...)
	alias("internal/runtime/atomic", "Loaduintptr", "internal/runtime/atomic", "Load64", p8...)
	alias("internal/runtime/atomic", "Loaduint", "internal/runtime/atomic", "Load", p4...)
	alias("internal/runtime/atomic", "Loaduint", "internal/runtime/atomic", "Load64", p8...)
	alias("internal/runtime/atomic", "LoadAcq", "internal/runtime/atomic", "Load", lwatomics...)
	alias("internal/runtime/atomic", "LoadAcq64", "internal/runtime/atomic", "Load64", lwatomics...)
	alias("internal/runtime/atomic", "LoadAcquintptr", "internal/runtime/atomic", "LoadAcq", p4...)
	alias("internal/runtime/atomic", "LoadAcquintptr", "internal/runtime/atomic", "LoadAcq64", p8...)

	// Aliases for atomic store operations
	alias("internal/runtime/atomic", "Storeint32", "internal/runtime/atomic", "Store", all...)
	alias("internal/runtime/atomic", "Storeint64", "internal/runtime/atomic", "Store64", all...)
	alias("internal/runtime/atomic", "Storeuintptr", "internal/runtime/atomic", "Store", p4...)
	alias("internal/runtime/atomic", "Storeuintptr", "internal/runtime/atomic", "Store64", p8...)
	alias("internal/runtime/atomic", "StoreRel", "internal/runtime/atomic", "Store", lwatomics...)
	alias("internal/runtime/atomic", "StoreRel64", "internal/runtime/atomic", "Store64", lwatomics...)
	alias("internal/runtime/atomic", "StoreReluintptr", "internal/runtime/atomic", "StoreRel", p4...)
	alias("internal/runtime/atomic", "StoreReluintptr", "internal/runtime/atomic", "StoreRel64", p8...)

	// Aliases for atomic swap operations
	alias("internal/runtime/atomic", "Xchgint32", "internal/runtime/atomic", "Xchg", all...)
	alias("internal/runtime/atomic", "Xchgint64", "internal/runtime/atomic", "Xchg64", all...)
	alias("internal/runtime/atomic", "Xchguintptr", "internal/runtime/atomic", "Xchg", p4...)
	alias("internal/runtime/atomic", "Xchguintptr", "internal/runtime/atomic", "Xchg64", p8...)

	// Aliases for atomic add operations
	alias("internal/runtime/atomic", "Xaddint32", "internal/runtime/atomic", "Xadd", all...)
	alias("internal/runtime/atomic", "Xaddint64", "internal/runtime/atomic", "Xadd64", all...)
	alias("internal/runtime/atomic", "Xadduintptr", "internal/runtime/atomic", "Xadd", p4...)
	alias("internal/runtime/atomic", "Xadduintptr", "internal/runtime/atomic", "Xadd64", p8...)

	// Aliases for atomic CAS operations
	alias("internal/runtime/atomic", "Casint32", "internal/runtime/atomic", "Cas", all...)
	alias("internal/runtime/atomic", "Casint64", "internal/runtime/atomic", "Cas64", all...)
	alias("internal/runtime/atomic", "Casuintptr", "internal/runtime/atomic", "Cas", p4...)
	alias("internal/runtime/atomic", "Casuintptr", "internal/runtime/atomic", "Cas64", p8...)
	alias("internal/runtime/atomic", "Casp1", "internal/runtime/atomic", "Cas", p4...)
	alias("internal/runtime/atomic", "Casp1", "internal/runtime/atomic", "Cas64", p8...)
	alias("internal/runtime/atomic", "CasRel", "internal/runtime/atomic", "Cas", lwatomics...)

	// Aliases for atomic And/Or operations
	alias("internal/runtime/atomic", "Anduintptr", "internal/runtime/atomic", "And64", sys.ArchARM64, sys.ArchLoong64)
	alias("internal/runtime/atomic", "Oruintptr", "internal/runtime/atomic", "Or64", sys.ArchARM64, sys.ArchLoong64)

	/******** math ********/
	addF("math", "sqrt",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpSqrt, types.Types[types.TFLOAT64], args[0])
		},
		sys.I386, sys.AMD64, sys.ARM, sys.ARM64, sys.Loong64, sys.MIPS, sys.MIPS64, sys.PPC64, sys.RISCV64, sys.S390X, sys.Wasm)
	addF("math", "Trunc",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpTrunc, types.Types[types.TFLOAT64], args[0])
		},
		sys.ARM64, sys.PPC64, sys.S390X, sys.Wasm)
	addF("math", "Ceil",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpCeil, types.Types[types.TFLOAT64], args[0])
		},
		sys.ARM64, sys.PPC64, sys.S390X, sys.Wasm)
	addF("math", "Floor",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpFloor, types.Types[types.TFLOAT64], args[0])
		},
		sys.ARM64, sys.PPC64, sys.S390X, sys.Wasm)
	addF("math", "Round",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpRound, types.Types[types.TFLOAT64], args[0])
		},
		sys.ARM64, sys.PPC64, sys.S390X)
	addF("math", "RoundToEven",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpRoundToEven, types.Types[types.TFLOAT64], args[0])
		},
		sys.ARM64, sys.S390X, sys.Wasm)
	addF("math", "Abs",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpAbs, types.Types[types.TFLOAT64], args[0])
		},
		sys.ARM64, sys.ARM, sys.Loong64, sys.PPC64, sys.RISCV64, sys.Wasm, sys.MIPS, sys.MIPS64)
	addF("math", "Copysign",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue2(ssa.OpCopysign, types.Types[types.TFLOAT64], args[0], args[1])
		},
		sys.Loong64, sys.PPC64, sys.RISCV64, sys.Wasm)
	addF("math", "FMA",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue3(ssa.OpFMA, types.Types[types.TFLOAT64], args[0], args[1], args[2])
		},
		sys.ARM64, sys.Loong64, sys.PPC64, sys.RISCV64, sys.S390X)
	addF("math", "FMA",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			if cfg.goamd64 >= 3 {
				return s.newValue3(ssa.OpFMA, types.Types[types.TFLOAT64], args[0], args[1], args[2])
			}

			v := s.entryNewValue0A(ssa.OpHasCPUFeature, types.Types[types.TBOOL], ir.Syms.X86HasFMA)
			b := s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(v)
			bTrue := s.f.NewBlock(ssa.BlockPlain)
			bFalse := s.f.NewBlock(ssa.BlockPlain)
			bEnd := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(bTrue)
			b.AddEdgeTo(bFalse)
			b.Likely = ssa.BranchLikely // >= haswell cpus are common

			// We have the intrinsic - use it directly.
			s.startBlock(bTrue)
			s.vars[n] = s.newValue3(ssa.OpFMA, types.Types[types.TFLOAT64], args[0], args[1], args[2])
			s.endBlock().AddEdgeTo(bEnd)

			// Call the pure Go version.
			s.startBlock(bFalse)
			s.vars[n] = s.callResult(n, callNormal) // types.Types[TFLOAT64]
			s.endBlock().AddEdgeTo(bEnd)

			// Merge results.
			s.startBlock(bEnd)
			return s.variable(n, types.Types[types.TFLOAT64])
		},
		sys.AMD64)
	addF("math", "FMA",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			addr := s.entryNewValue1A(ssa.OpAddr, types.Types[types.TBOOL].PtrTo(), ir.Syms.ARMHasVFPv4, s.sb)
			v := s.load(types.Types[types.TBOOL], addr)
			b := s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(v)
			bTrue := s.f.NewBlock(ssa.BlockPlain)
			bFalse := s.f.NewBlock(ssa.BlockPlain)
			bEnd := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(bTrue)
			b.AddEdgeTo(bFalse)
			b.Likely = ssa.BranchLikely

			// We have the intrinsic - use it directly.
			s.startBlock(bTrue)
			s.vars[n] = s.newValue3(ssa.OpFMA, types.Types[types.TFLOAT64], args[0], args[1], args[2])
			s.endBlock().AddEdgeTo(bEnd)

			// Call the pure Go version.
			s.startBlock(bFalse)
			s.vars[n] = s.callResult(n, callNormal) // types.Types[TFLOAT64]
			s.endBlock().AddEdgeTo(bEnd)

			// Merge results.
			s.startBlock(bEnd)
			return s.variable(n, types.Types[types.TFLOAT64])
		},
		sys.ARM)

	makeRoundAMD64 := func(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			if cfg.goamd64 >= 2 {
				return s.newValue1(op, types.Types[types.TFLOAT64], args[0])
			}

			v := s.entryNewValue0A(ssa.OpHasCPUFeature, types.Types[types.TBOOL], ir.Syms.X86HasSSE41)
			b := s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(v)
			bTrue := s.f.NewBlock(ssa.BlockPlain)
			bFalse := s.f.NewBlock(ssa.BlockPlain)
			bEnd := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(bTrue)
			b.AddEdgeTo(bFalse)
			b.Likely = ssa.BranchLikely // most machines have sse4.1 nowadays

			// We have the intrinsic - use it directly.
			s.startBlock(bTrue)
			s.vars[n] = s.newValue1(op, types.Types[types.TFLOAT64], args[0])
			s.endBlock().AddEdgeTo(bEnd)

			// Call the pure Go version.
			s.startBlock(bFalse)
			s.vars[n] = s.callResult(n, callNormal) // types.Types[TFLOAT64]
			s.endBlock().AddEdgeTo(bEnd)

			// Merge results.
			s.startBlock(bEnd)
			return s.variable(n, types.Types[types.TFLOAT64])
		}
	}
	addF("math", "RoundToEven",
		makeRoundAMD64(ssa.OpRoundToEven),
		sys.AMD64)
	addF("math", "Floor",
		makeRoundAMD64(ssa.OpFloor),
		sys.AMD64)
	addF("math", "Ceil",
		makeRoundAMD64(ssa.OpCeil),
		sys.AMD64)
	addF("math", "Trunc",
		makeRoundAMD64(ssa.OpTrunc),
		sys.AMD64)

	makeRoundLoong64 := func(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			addr := s.entryNewValue1A(ssa.OpAddr, types.Types[types.TBOOL].PtrTo(), ir.Syms.Loong64HasLSX, s.sb)
			v := s.load(types.Types[types.TBOOL], addr)
			b := s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(v)
			bTrue := s.f.NewBlock(ssa.BlockPlain)
			bFalse := s.f.NewBlock(ssa.BlockPlain)
			bEnd := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(bTrue)
			b.AddEdgeTo(bFalse)
			b.Likely = ssa.BranchLikely // most loong64 machines support the LSX

			// We have the intrinsic - use it directly.
			s.startBlock(bTrue)
			s.vars[n] = s.newValue1(op, types.Types[types.TFLOAT64], args[0])
			s.endBlock().AddEdgeTo(bEnd)

			// Call the pure Go version.
			s.startBlock(bFalse)
			s.vars[n] = s.callResult(n, callNormal) // types.Types[TFLOAT64]
			s.endBlock().AddEdgeTo(bEnd)

			// Merge results.
			s.startBlock(bEnd)
			return s.variable(n, types.Types[types.TFLOAT64])
		}
	}
	addF("math", "RoundToEven",
		makeRoundLoong64(ssa.OpRoundToEven),
		sys.Loong64)
	addF("math", "Floor",
		makeRoundLoong64(ssa.OpFloor),
		sys.Loong64)
	addF("math", "Ceil",
		makeRoundLoong64(ssa.OpCeil),
		sys.Loong64)
	addF("math", "Trunc",
		makeRoundLoong64(ssa.OpTrunc),
		sys.Loong64)

	/******** math/bits ********/
	addF("math/bits", "TrailingZeros64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpCtz64, types.Types[types.TINT], args[0])
		},
		sys.AMD64, sys.ARM64, sys.ARM, sys.Loong64, sys.S390X, sys.MIPS, sys.PPC64, sys.Wasm)
	addF("math/bits", "TrailingZeros64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			lo := s.newValue1(ssa.OpInt64Lo, types.Types[types.TUINT32], args[0])
			hi := s.newValue1(ssa.OpInt64Hi, types.Types[types.TUINT32], args[0])
			return s.newValue2(ssa.OpCtz64On32, types.Types[types.TINT], lo, hi)
		},
		sys.I386)
	addF("math/bits", "TrailingZeros32",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpCtz32, types.Types[types.TINT], args[0])
		},
		sys.AMD64, sys.I386, sys.ARM64, sys.ARM, sys.Loong64, sys.S390X, sys.MIPS, sys.PPC64, sys.Wasm)
	addF("math/bits", "TrailingZeros16",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpCtz16, types.Types[types.TINT], args[0])
		},
		sys.AMD64, sys.ARM, sys.ARM64, sys.I386, sys.MIPS, sys.Loong64, sys.PPC64, sys.S390X, sys.Wasm)
	addF("math/bits", "TrailingZeros8",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpCtz8, types.Types[types.TINT], args[0])
		},
		sys.AMD64, sys.ARM, sys.ARM64, sys.I386, sys.MIPS, sys.Loong64, sys.PPC64, sys.S390X, sys.Wasm)

	if cfg.goriscv64 >= 22 {
		addF("math/bits", "TrailingZeros64",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpCtz64, types.Types[types.TINT], args[0])
			},
			sys.RISCV64)
		addF("math/bits", "TrailingZeros32",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpCtz32, types.Types[types.TINT], args[0])
			},
			sys.RISCV64)
		addF("math/bits", "TrailingZeros16",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpCtz16, types.Types[types.TINT], args[0])
			},
			sys.RISCV64)
		addF("math/bits", "TrailingZeros8",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpCtz8, types.Types[types.TINT], args[0])
			},
			sys.RISCV64)
	}

	// ReverseBytes inlines correctly, no need to intrinsify it.
	alias("math/bits", "ReverseBytes64", "internal/runtime/sys", "Bswap64", all...)
	alias("math/bits", "ReverseBytes32", "internal/runtime/sys", "Bswap32", all...)
	// Nothing special is needed for targets where ReverseBytes16 lowers to a rotate
	addF("math/bits", "ReverseBytes16",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBswap16, types.Types[types.TUINT16], args[0])
		},
		sys.Loong64)
	if cfg.goppc64 >= 10 {
		// On Power10, 16-bit rotate is not available so use BRH instruction
		addF("math/bits", "ReverseBytes16",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpBswap16, types.Types[types.TUINT], args[0])
			},
			sys.PPC64)
	}
	if cfg.goriscv64 >= 22 {
		addF("math/bits", "ReverseBytes16",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpBswap16, types.Types[types.TUINT16], args[0])
			},
			sys.RISCV64)
	}

	addF("math/bits", "Len64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBitLen64, types.Types[types.TINT], args[0])
		},
		sys.AMD64, sys.ARM, sys.ARM64, sys.Loong64, sys.MIPS, sys.PPC64, sys.S390X, sys.Wasm)
	addF("math/bits", "Len32",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBitLen32, types.Types[types.TINT], args[0])
		},
		sys.AMD64, sys.ARM, sys.ARM64, sys.Loong64, sys.MIPS, sys.PPC64, sys.S390X, sys.Wasm)
	addF("math/bits", "Len16",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBitLen16, types.Types[types.TINT], args[0])
		},
		sys.AMD64, sys.ARM, sys.ARM64, sys.Loong64, sys.MIPS, sys.PPC64, sys.S390X, sys.Wasm)
	addF("math/bits", "Len8",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBitLen8, types.Types[types.TINT], args[0])
		},
		sys.AMD64, sys.ARM, sys.ARM64, sys.Loong64, sys.MIPS, sys.PPC64, sys.S390X, sys.Wasm)

	if cfg.goriscv64 >= 22 {
		addF("math/bits", "Len64",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpBitLen64, types.Types[types.TINT], args[0])
			},
			sys.RISCV64)
		addF("math/bits", "Len32",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpBitLen32, types.Types[types.TINT], args[0])
			},
			sys.RISCV64)
		addF("math/bits", "Len16",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpBitLen16, types.Types[types.TINT], args[0])
			},
			sys.RISCV64)
		addF("math/bits", "Len8",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				return s.newValue1(ssa.OpBitLen8, types.Types[types.TINT], args[0])
			},
			sys.RISCV64)
	}

	alias("math/bits", "Len", "math/bits", "Len64", p8...)
	alias("math/bits", "Len", "math/bits", "Len32", p4...)

	// LeadingZeros is handled because it trivially calls Len.
	addF("math/bits", "Reverse64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBitRev64, types.Types[types.TUINT64], args[0])
		},
		sys.ARM64, sys.Loong64)
	addF("math/bits", "Reverse32",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBitRev32, types.Types[types.TUINT32], args[0])
		},
		sys.ARM64, sys.Loong64)
	addF("math/bits", "Reverse16",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBitRev16, types.Types[types.TUINT16], args[0])
		},
		sys.ARM64, sys.Loong64)
	addF("math/bits", "Reverse8",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBitRev8, types.Types[types.TUINT8], args[0])
		},
		sys.ARM64, sys.Loong64)
	addF("math/bits", "Reverse",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpBitRev64, types.Types[types.TUINT], args[0])
		},
		sys.ARM64, sys.Loong64)
	addF("math/bits", "RotateLeft8",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue2(ssa.OpRotateLeft8, types.Types[types.TUINT8], args[0], args[1])
		},
		sys.AMD64, sys.RISCV64)
	addF("math/bits", "RotateLeft16",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue2(ssa.OpRotateLeft16, types.Types[types.TUINT16], args[0], args[1])
		},
		sys.AMD64, sys.RISCV64)
	addF("math/bits", "RotateLeft32",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue2(ssa.OpRotateLeft32, types.Types[types.TUINT32], args[0], args[1])
		},
		sys.AMD64, sys.ARM, sys.ARM64, sys.Loong64, sys.PPC64, sys.RISCV64, sys.S390X, sys.Wasm)
	addF("math/bits", "RotateLeft64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue2(ssa.OpRotateLeft64, types.Types[types.TUINT64], args[0], args[1])
		},
		sys.AMD64, sys.ARM64, sys.Loong64, sys.PPC64, sys.RISCV64, sys.S390X, sys.Wasm)
	alias("math/bits", "RotateLeft", "math/bits", "RotateLeft64", p8...)

	makeOnesCountAMD64 := func(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			if cfg.goamd64 >= 2 {
				return s.newValue1(op, types.Types[types.TINT], args[0])
			}

			v := s.entryNewValue0A(ssa.OpHasCPUFeature, types.Types[types.TBOOL], ir.Syms.X86HasPOPCNT)
			b := s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(v)
			bTrue := s.f.NewBlock(ssa.BlockPlain)
			bFalse := s.f.NewBlock(ssa.BlockPlain)
			bEnd := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(bTrue)
			b.AddEdgeTo(bFalse)
			b.Likely = ssa.BranchLikely // most machines have popcnt nowadays

			// We have the intrinsic - use it directly.
			s.startBlock(bTrue)
			s.vars[n] = s.newValue1(op, types.Types[types.TINT], args[0])
			s.endBlock().AddEdgeTo(bEnd)

			// Call the pure Go version.
			s.startBlock(bFalse)
			s.vars[n] = s.callResult(n, callNormal) // types.Types[TINT]
			s.endBlock().AddEdgeTo(bEnd)

			// Merge results.
			s.startBlock(bEnd)
			return s.variable(n, types.Types[types.TINT])
		}
	}

	makeOnesCountLoong64 := func(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			addr := s.entryNewValue1A(ssa.OpAddr, types.Types[types.TBOOL].PtrTo(), ir.Syms.Loong64HasLSX, s.sb)
			v := s.load(types.Types[types.TBOOL], addr)
			b := s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(v)
			bTrue := s.f.NewBlock(ssa.BlockPlain)
			bFalse := s.f.NewBlock(ssa.BlockPlain)
			bEnd := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(bTrue)
			b.AddEdgeTo(bFalse)
			b.Likely = ssa.BranchLikely // most loong64 machines support the LSX

			// We have the intrinsic - use it directly.
			s.startBlock(bTrue)
			s.vars[n] = s.newValue1(op, types.Types[types.TINT], args[0])
			s.endBlock().AddEdgeTo(bEnd)

			// Call the pure Go version.
			s.startBlock(bFalse)
			s.vars[n] = s.callResult(n, callNormal) // types.Types[TINT]
			s.endBlock().AddEdgeTo(bEnd)

			// Merge results.
			s.startBlock(bEnd)
			return s.variable(n, types.Types[types.TINT])
		}
	}

	makeOnesCountRISCV64 := func(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			if cfg.goriscv64 >= 22 {
				return s.newValue1(op, types.Types[types.TINT], args[0])
			}

			addr := s.entryNewValue1A(ssa.OpAddr, types.Types[types.TBOOL].PtrTo(), ir.Syms.RISCV64HasZbb, s.sb)
			v := s.load(types.Types[types.TBOOL], addr)
			b := s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(v)
			bTrue := s.f.NewBlock(ssa.BlockPlain)
			bFalse := s.f.NewBlock(ssa.BlockPlain)
			bEnd := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(bTrue)
			b.AddEdgeTo(bFalse)
			b.Likely = ssa.BranchLikely // Majority of RISC-V support Zbb.

			// We have the intrinsic - use it directly.
			s.startBlock(bTrue)
			s.vars[n] = s.newValue1(op, types.Types[types.TINT], args[0])
			s.endBlock().AddEdgeTo(bEnd)

			// Call the pure Go version.
			s.startBlock(bFalse)
			s.vars[n] = s.callResult(n, callNormal) // types.Types[TINT]
			s.endBlock().AddEdgeTo(bEnd)

			// Merge results.
			s.startBlock(bEnd)
			return s.variable(n, types.Types[types.TINT])
		}
	}

	addF("math/bits", "OnesCount64",
		makeOnesCountAMD64(ssa.OpPopCount64),
		sys.AMD64)
	addF("math/bits", "OnesCount64",
		makeOnesCountLoong64(ssa.OpPopCount64),
		sys.Loong64)
	addF("math/bits", "OnesCount64",
		makeOnesCountRISCV64(ssa.OpPopCount64),
		sys.RISCV64)
	addF("math/bits", "OnesCount64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpPopCount64, types.Types[types.TINT], args[0])
		},
		sys.PPC64, sys.ARM64, sys.S390X, sys.Wasm)
	addF("math/bits", "OnesCount32",
		makeOnesCountAMD64(ssa.OpPopCount32),
		sys.AMD64)
	addF("math/bits", "OnesCount32",
		makeOnesCountLoong64(ssa.OpPopCount32),
		sys.Loong64)
	addF("math/bits", "OnesCount32",
		makeOnesCountRISCV64(ssa.OpPopCount32),
		sys.RISCV64)
	addF("math/bits", "OnesCount32",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpPopCount32, types.Types[types.TINT], args[0])
		},
		sys.PPC64, sys.ARM64, sys.S390X, sys.Wasm)
	addF("math/bits", "OnesCount16",
		makeOnesCountAMD64(ssa.OpPopCount16),
		sys.AMD64)
	addF("math/bits", "OnesCount16",
		makeOnesCountLoong64(ssa.OpPopCount16),
		sys.Loong64)
	addF("math/bits", "OnesCount16",
		makeOnesCountRISCV64(ssa.OpPopCount16),
		sys.RISCV64)
	addF("math/bits", "OnesCount16",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpPopCount16, types.Types[types.TINT], args[0])
		},
		sys.ARM64, sys.S390X, sys.PPC64, sys.Wasm)
	addF("math/bits", "OnesCount8",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpPopCount8, types.Types[types.TINT], args[0])
		},
		sys.S390X, sys.PPC64, sys.Wasm)

	if cfg.goriscv64 >= 22 {
		addF("math/bits", "OnesCount8",
			makeOnesCountRISCV64(ssa.OpPopCount8),
			sys.RISCV64)
	}

	alias("math/bits", "OnesCount", "math/bits", "OnesCount64", p8...)

	add("math/bits", "Mul64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue2(ssa.OpMul64uhilo, types.NewTuple(types.Types[types.TUINT64], types.Types[types.TUINT64]), args[0], args[1])
		},
		all...)
	alias("math/bits", "Mul", "math/bits", "Mul64", p8...)
	addF("math/bits", "Add64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue3(ssa.OpAdd64carry, types.NewTuple(types.Types[types.TUINT64], types.Types[types.TUINT64]), args[0], args[1], args[2])
		},
		sys.AMD64, sys.ARM64, sys.PPC64, sys.S390X, sys.RISCV64, sys.Loong64, sys.MIPS64)
	alias("math/bits", "Add", "math/bits", "Add64", p8...)
	alias("internal/runtime/math", "Add64", "math/bits", "Add64", all...)
	addF("math/bits", "Sub64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue3(ssa.OpSub64borrow, types.NewTuple(types.Types[types.TUINT64], types.Types[types.TUINT64]), args[0], args[1], args[2])
		},
		sys.AMD64, sys.ARM64, sys.PPC64, sys.S390X, sys.RISCV64, sys.Loong64, sys.MIPS64)
	alias("math/bits", "Sub", "math/bits", "Sub64", p8...)
	addF("math/bits", "Div64",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			// check for divide-by-zero/overflow and panic with appropriate message
			cmpZero := s.newValue2(s.ssaOp(ir.ONE, types.Types[types.TUINT64]), types.Types[types.TBOOL], args[2], s.zeroVal(types.Types[types.TUINT64]))
			s.check(cmpZero, ir.Syms.Panicdivide)
			cmpOverflow := s.newValue2(s.ssaOp(ir.OLT, types.Types[types.TUINT64]), types.Types[types.TBOOL], args[0], args[2])
			s.check(cmpOverflow, ir.Syms.Panicoverflow)
			return s.newValue3(ssa.OpDiv128u, types.NewTuple(types.Types[types.TUINT64], types.Types[types.TUINT64]), args[0], args[1], args[2])
		},
		sys.AMD64)
	alias("math/bits", "Div", "math/bits", "Div64", sys.ArchAMD64)

	alias("internal/runtime/sys", "TrailingZeros8", "math/bits", "TrailingZeros8", all...)
	alias("internal/runtime/sys", "TrailingZeros32", "math/bits", "TrailingZeros32", all...)
	alias("internal/runtime/sys", "TrailingZeros64", "math/bits", "TrailingZeros64", all...)
	alias("internal/runtime/sys", "Len8", "math/bits", "Len8", all...)
	alias("internal/runtime/sys", "Len64", "math/bits", "Len64", all...)
	alias("internal/runtime/sys", "OnesCount64", "math/bits", "OnesCount64", all...)

	/******** sync/atomic ********/

	// Note: these are disabled by flag_race in findIntrinsic below.
	alias("sync/atomic", "LoadInt32", "internal/runtime/atomic", "Load", all...)
	alias("sync/atomic", "LoadInt64", "internal/runtime/atomic", "Load64", all...)
	alias("sync/atomic", "LoadPointer", "internal/runtime/atomic", "Loadp", all...)
	alias("sync/atomic", "LoadUint32", "internal/runtime/atomic", "Load", all...)
	alias("sync/atomic", "LoadUint64", "internal/runtime/atomic", "Load64", all...)
	alias("sync/atomic", "LoadUintptr", "internal/runtime/atomic", "Load", p4...)
	alias("sync/atomic", "LoadUintptr", "internal/runtime/atomic", "Load64", p8...)

	alias("sync/atomic", "StoreInt32", "internal/runtime/atomic", "Store", all...)
	alias("sync/atomic", "StoreInt64", "internal/runtime/atomic", "Store64", all...)
	// Note: not StorePointer, that needs a write barrier.  Same below for {CompareAnd}Swap.
	alias("sync/atomic", "StoreUint32", "internal/runtime/atomic", "Store", all...)
	alias("sync/atomic", "StoreUint64", "internal/runtime/atomic", "Store64", all...)
	alias("sync/atomic", "StoreUintptr", "internal/runtime/atomic", "Store", p4...)
	alias("sync/atomic", "StoreUintptr", "internal/runtime/atomic", "Store64", p8...)

	alias("sync/atomic", "SwapInt32", "internal/runtime/atomic", "Xchg", all...)
	alias("sync/atomic", "SwapInt64", "internal/runtime/atomic", "Xchg64", all...)
	alias("sync/atomic", "SwapUint32", "internal/runtime/atomic", "Xchg", all...)
	alias("sync/atomic", "SwapUint64", "internal/runtime/atomic", "Xchg64", all...)
	alias("sync/atomic", "SwapUintptr", "internal/runtime/atomic", "Xchg", p4...)
	alias("sync/atomic", "SwapUintptr", "internal/runtime/atomic", "Xchg64", p8...)

	alias("sync/atomic", "CompareAndSwapInt32", "internal/runtime/atomic", "Cas", all...)
	alias("sync/atomic", "CompareAndSwapInt64", "internal/runtime/atomic", "Cas64", all...)
	alias("sync/atomic", "CompareAndSwapUint32", "internal/runtime/atomic", "Cas", all...)
	alias("sync/atomic", "CompareAndSwapUint64", "internal/runtime/atomic", "Cas64", all...)
	alias("sync/atomic", "CompareAndSwapUintptr", "internal/runtime/atomic", "Cas", p4...)
	alias("sync/atomic", "CompareAndSwapUintptr", "internal/runtime/atomic", "Cas64", p8...)

	alias("sync/atomic", "AddInt32", "internal/runtime/atomic", "Xadd", all...)
	alias("sync/atomic", "AddInt64", "internal/runtime/atomic", "Xadd64", all...)
	alias("sync/atomic", "AddUint32", "internal/runtime/atomic", "Xadd", all...)
	alias("sync/atomic", "AddUint64", "internal/runtime/atomic", "Xadd64", all...)
	alias("sync/atomic", "AddUintptr", "internal/runtime/atomic", "Xadd", p4...)
	alias("sync/atomic", "AddUintptr", "internal/runtime/atomic", "Xadd64", p8...)

	alias("sync/atomic", "AndInt32", "internal/runtime/atomic", "And32", sys.ArchARM64, sys.ArchAMD64, sys.ArchLoong64)
	alias("sync/atomic", "AndUint32", "internal/runtime/atomic", "And32", sys.ArchARM64, sys.ArchAMD64, sys.ArchLoong64)
	alias("sync/atomic", "AndInt64", "internal/runtime/atomic", "And64", sys.ArchARM64, sys.ArchAMD64, sys.ArchLoong64)
	alias("sync/atomic", "AndUint64", "internal/runtime/atomic", "And64", sys.ArchARM64, sys.ArchAMD64, sys.ArchLoong64)
	alias("sync/atomic", "AndUintptr", "internal/runtime/atomic", "And64", sys.ArchARM64, sys.ArchAMD64, sys.ArchLoong64)
	alias("sync/atomic", "OrInt32", "internal/runtime/atomic", "Or32", sys.ArchARM64, sys.ArchAMD64, sys.ArchLoong64)
	alias("sync/atomic", "OrUint32", "internal/runtime/atomic", "Or32", sys.ArchARM64, sys.ArchAMD64, sys.ArchLoong64)
	alias("sync/atomic", "OrInt64", "internal/runtime/atomic", "Or64", sys.ArchARM64, sys.ArchAMD64, sys.ArchLoong64)
	alias("sync/atomic", "OrUint64", "internal/runtime/atomic", "Or64", sys.ArchARM64, sys.ArchAMD64, sys.ArchLoong64)
	alias("sync/atomic", "OrUintptr", "internal/runtime/atomic", "Or64", sys.ArchARM64, sys.ArchAMD64, sys.ArchLoong64)

	/******** math/big ********/
	alias("math/big", "mulWW", "math/bits", "Mul64", p8...)

	/******** internal/runtime/maps ********/

	// Important: The intrinsic implementations below return a packed
	// bitset, while the portable Go implementation uses an unpacked
	// representation (one bit set in each byte).
	//
	// Thus we must replace most bitset methods with implementations that
	// work with the packed representation.
	//
	// TODO(prattmic): The bitset implementations don't use SIMD, so they
	// could be handled with build tags (though that would break
	// -d=ssa/intrinsics/off=1).

	// With a packed representation we no longer need to shift the result
	// of TrailingZeros64.
	alias("internal/runtime/maps", "bitsetFirst", "internal/runtime/sys", "TrailingZeros64", sys.ArchAMD64)

	addF("internal/runtime/maps", "bitsetRemoveBelow",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			b := args[0]
			i := args[1]

			// Clear the lower i bits in b.
			//
			// out = b &^ ((1 << i) - 1)

			one := s.constInt64(types.Types[types.TUINT64], 1)

			mask := s.newValue2(ssa.OpLsh8x8, types.Types[types.TUINT64], one, i)
			mask = s.newValue2(ssa.OpSub64, types.Types[types.TUINT64], mask, one)
			mask = s.newValue1(ssa.OpCom64, types.Types[types.TUINT64], mask)

			return s.newValue2(ssa.OpAnd64, types.Types[types.TUINT64], b, mask)
		},
		sys.AMD64)

	addF("internal/runtime/maps", "bitsetLowestSet",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			b := args[0]

			// Test the lowest bit in b.
			//
			// out = (b & 1) == 1

			one := s.constInt64(types.Types[types.TUINT64], 1)
			and := s.newValue2(ssa.OpAnd64, types.Types[types.TUINT64], b, one)
			return s.newValue2(ssa.OpEq64, types.Types[types.TBOOL], and, one)
		},
		sys.AMD64)

	addF("internal/runtime/maps", "bitsetShiftOutLowest",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			b := args[0]

			// Right shift out the lowest bit in b.
			//
			// out = b >> 1

			one := s.constInt64(types.Types[types.TUINT64], 1)
			return s.newValue2(ssa.OpRsh64Ux64, types.Types[types.TUINT64], b, one)
		},
		sys.AMD64)

	addF("internal/runtime/maps", "ctrlGroupMatchH2",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			g := args[0]
			h := args[1]

			// Explicit copies to fp registers. See
			// https://go.dev/issue/70451.
			gfp := s.newValue1(ssa.OpAMD64MOVQi2f, types.TypeInt128, g)
			hfp := s.newValue1(ssa.OpAMD64MOVQi2f, types.TypeInt128, h)

			// Broadcast h2 into each byte of a word.
			var broadcast *ssa.Value
			if buildcfg.GOAMD64 >= 4 {
				// VPBROADCASTB saves 1 instruction vs PSHUFB
				// because the input can come from a GP
				// register, while PSHUFB requires moving into
				// an FP register first.
				//
				// Nominally PSHUFB would require a second
				// additional instruction to load the control
				// mask into a FP register. But broadcast uses
				// a control mask of 0, and the register ABI
				// already defines X15 as a zero register.
				broadcast = s.newValue1(ssa.OpAMD64VPBROADCASTB, types.TypeInt128, h) // use gp copy of h
			} else if buildcfg.GOAMD64 >= 2 {
				// PSHUFB performs a byte broadcast when given
				// a control input of 0.
				broadcast = s.newValue1(ssa.OpAMD64PSHUFBbroadcast, types.TypeInt128, hfp)
			} else {
				// No direct byte broadcast. First we must
				// duplicate the lower byte and then do a
				// 16-bit broadcast.

				// "Unpack" h2 with itself. This duplicates the
				// input, resulting in h2 in the lower two
				// bytes.
				unpack := s.newValue2(ssa.OpAMD64PUNPCKLBW, types.TypeInt128, hfp, hfp)

				// Copy the lower 16-bits of unpack into every
				// 16-bit slot in the lower 64-bits of the
				// output register. Note that immediate 0
				// selects the low word as the source for every
				// destination slot.
				broadcast = s.newValue1I(ssa.OpAMD64PSHUFLW, types.TypeInt128, 0, unpack)

				// No need to broadcast into the upper 64-bits,
				// as we don't use those.
			}

			// Compare each byte of the control word with h2. Each
			// matching byte has every bit set.
			eq := s.newValue2(ssa.OpAMD64PCMPEQB, types.TypeInt128, broadcast, gfp)

			// Construct a "byte mask": each output bit is equal to
			// the sign bit each input byte.
			//
			// This results in a packed output (bit N set means
			// byte N matched).
			//
			// NOTE: See comment above on bitsetFirst.
			out := s.newValue1(ssa.OpAMD64PMOVMSKB, types.Types[types.TUINT8], eq)

			// g is only 64-bits so the upper 64-bits of the
			// 128-bit register will be zero. If h2 is also zero,
			// then we'll get matches on those bytes. Truncate the
			// upper bits to ignore such matches.
			ret := s.newValue1(ssa.OpZeroExt8to64, types.Types[types.TUINT64], out)

			return ret
		},
		sys.AMD64)

	addF("internal/runtime/maps", "ctrlGroupMatchEmpty",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			// An empty slot is   1000 0000
			// A deleted slot is  1111 1110
			// A full slot is     0??? ????

			g := args[0]

			// Explicit copy to fp register. See
			// https://go.dev/issue/70451.
			gfp := s.newValue1(ssa.OpAMD64MOVQi2f, types.TypeInt128, g)

			if buildcfg.GOAMD64 >= 2 {
				// "PSIGNB negates each data element of the
				// destination operand (the first operand) if
				// the signed integer value of the
				// corresponding data element in the source
				// operand (the second operand) is less than
				// zero. If the signed integer value of a data
				// element in the source operand is positive,
				// the corresponding data element in the
				// destination operand is unchanged. If a data
				// element in the source operand is zero, the
				// corresponding data element in the
				// destination operand is set to zero" - Intel SDM
				//
				// If we pass the group control word as both
				// arguments:
				// - Full slots are unchanged.
				// - Deleted slots are negated, becoming
				//   0000 0010.
				// - Empty slots are negated, becoming
				//   1000 0000 (unchanged!).
				//
				// The result is that only empty slots have the
				// sign bit set. We then use PMOVMSKB to
				// extract the sign bits.
				sign := s.newValue2(ssa.OpAMD64PSIGNB, types.TypeInt128, gfp, gfp)

				// Construct a "byte mask": each output bit is
				// equal to the sign bit each input byte. The
				// sign bit is only set for empty or deleted
				// slots.
				//
				// This results in a packed output (bit N set
				// means byte N matched).
				//
				// NOTE: See comment above on bitsetFirst.
				ret := s.newValue1(ssa.OpAMD64PMOVMSKB, types.Types[types.TUINT64], sign)

				// g is only 64-bits so the upper 64-bits of
				// the 128-bit register will be zero. PSIGNB
				// will keep all of these bytes zero, so no
				// need to truncate.

				return ret
			}

			// No PSIGNB, simply do byte equality with ctrlEmpty.

			// Load ctrlEmpty into each byte of a control word.
			var ctrlsEmpty uint64 = abi.MapCtrlEmpty
			e := s.constInt64(types.Types[types.TUINT64], int64(ctrlsEmpty))
			// Explicit copy to fp register. See
			// https://go.dev/issue/70451.
			efp := s.newValue1(ssa.OpAMD64MOVQi2f, types.TypeInt128, e)

			// Compare each byte of the control word with ctrlEmpty. Each
			// matching byte has every bit set.
			eq := s.newValue2(ssa.OpAMD64PCMPEQB, types.TypeInt128, efp, gfp)

			// Construct a "byte mask": each output bit is equal to
			// the sign bit each input byte.
			//
			// This results in a packed output (bit N set means
			// byte N matched).
			//
			// NOTE: See comment above on bitsetFirst.
			out := s.newValue1(ssa.OpAMD64PMOVMSKB, types.Types[types.TUINT8], eq)

			// g is only 64-bits so the upper 64-bits of the
			// 128-bit register will be zero. The upper 64-bits of
			// efp are also zero, so we'll get matches on those
			// bytes. Truncate the upper bits to ignore such
			// matches.
			return s.newValue1(ssa.OpZeroExt8to64, types.Types[types.TUINT64], out)
		},
		sys.AMD64)

	addF("internal/runtime/maps", "ctrlGroupMatchEmptyOrDeleted",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			// An empty slot is   1000 0000
			// A deleted slot is  1111 1110
			// A full slot is     0??? ????
			//
			// A slot is empty or deleted iff bit 7 (sign bit) is
			// set.

			g := args[0]

			// Explicit copy to fp register. See
			// https://go.dev/issue/70451.
			gfp := s.newValue1(ssa.OpAMD64MOVQi2f, types.TypeInt128, g)

			// Construct a "byte mask": each output bit is equal to
			// the sign bit each input byte. The sign bit is only
			// set for empty or deleted slots.
			//
			// This results in a packed output (bit N set means
			// byte N matched).
			//
			// NOTE: See comment above on bitsetFirst.
			ret := s.newValue1(ssa.OpAMD64PMOVMSKB, types.Types[types.TUINT64], gfp)

			// g is only 64-bits so the upper 64-bits of the
			// 128-bit register will be zero. Zero will never match
			// ctrlEmpty or ctrlDeleted, so no need to truncate.

			return ret
		},
		sys.AMD64)

	addF("internal/runtime/maps", "ctrlGroupMatchFull",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			// An empty slot is   1000 0000
			// A deleted slot is  1111 1110
			// A full slot is     0??? ????
			//
			// A slot is full iff bit 7 (sign bit) is unset.

			g := args[0]

			// Explicit copy to fp register. See
			// https://go.dev/issue/70451.
			gfp := s.newValue1(ssa.OpAMD64MOVQi2f, types.TypeInt128, g)

			// Construct a "byte mask": each output bit is equal to
			// the sign bit each input byte. The sign bit is only
			// set for empty or deleted slots.
			//
			// This results in a packed output (bit N set means
			// byte N matched).
			//
			// NOTE: See comment above on bitsetFirst.
			mask := s.newValue1(ssa.OpAMD64PMOVMSKB, types.Types[types.TUINT8], gfp)

			// Invert the mask to set the bits for the full slots.
			out := s.newValue1(ssa.OpCom8, types.Types[types.TUINT8], mask)

			// g is only 64-bits so the upper 64-bits of the
			// 128-bit register will be zero, with bit 7 unset.
			// Truncate the upper bits to ignore these.
			return s.newValue1(ssa.OpZeroExt8to64, types.Types[types.TUINT64], out)
		},
		sys.AMD64)

	/******** crypto/internal/constanttime ********/
	// We implement a superset of the Select promise:
	// Select returns x if v != 0 and y if v == 0.
	hasCMOV := []*sys.Arch{sys.ArchAMD64, sys.ArchARM64, sys.ArchLoong64, sys.ArchPPC64, sys.ArchPPC64LE, sys.ArchWasm}
	if cfg.goriscv64 >= 23 {
		hasCMOV = append(hasCMOV, sys.ArchRISCV64)
	}
	add("crypto/internal/constanttime", "Select",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			v, x, y := args[0], args[1], args[2]

			var checkOp ssa.Op
			var zero *ssa.Value
			switch s.config.PtrSize {
			case 8:
				checkOp = ssa.OpNeq64
				zero = s.constInt64(types.Types[types.TINT], 0)
			case 4:
				checkOp = ssa.OpNeq32
				zero = s.constInt32(types.Types[types.TINT], 0)
			default:
				panic("unreachable")
			}
			check := s.newValue2(checkOp, types.Types[types.TBOOL], zero, v)

			return s.newValue3(ssa.OpCondSelect, types.Types[types.TINT], x, y, check)
		}, hasCMOV...) // all with CMOV support.
	add("crypto/internal/constanttime", "boolToUint8",
		func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(ssa.OpCvtBoolToUint8, types.Types[types.TUINT8], args[0])
		},
		all...)

	if buildcfg.Experiment.SIMD {
		// Only enable intrinsics, if SIMD experiment.
		simdAMD64Intrinsics(addF)
		simdARM64Intrinsics(addF)
		initWasmSIMD()

		addF(simdPackage, "ClearAVXUpperBits",
			func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
				s.vars[memVar] = s.newValue1(ssa.OpAMD64VZEROUPPER, types.TypeMem, s.mem())
				return nil
			},
			sys.AMD64)

		addF(simdPackage, "Int8x16.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Int16x8.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Int32x4.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Int64x2.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Uint8x16.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Uint16x8.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Uint32x4.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Uint64x2.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Int8x32.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Int16x16.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Int32x8.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Int64x4.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Uint8x32.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Uint16x16.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Uint32x8.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Uint64x4.IsZero", opLen1(ssa.OpIsZeroVec, types.Types[types.TBOOL]), sys.AMD64)
		addF(simdPackage, "Float32x4.IsNaN", opLen1(ssa.OpIsNaNFloat32x4, types.TypeVec128), sys.AMD64)
		addF(simdPackage, "Float32x8.IsNaN", opLen1(ssa.OpIsNaNFloat32x8, types.TypeVec256), sys.AMD64)
		addF(simdPackage, "Float32x16.IsNaN", opLen1(ssa.OpIsNaNFloat32x16, types.TypeVec512), sys.AMD64)
		addF(simdPackage, "Float64x2.IsNaN", opLen1(ssa.OpIsNaNFloat64x2, types.TypeVec128), sys.AMD64)
		addF(simdPackage, "Float64x4.IsNaN", opLen1(ssa.OpIsNaNFloat64x4, types.TypeVec256), sys.AMD64)
		addF(simdPackage, "Float64x8.IsNaN", opLen1(ssa.OpIsNaNFloat64x8, types.TypeVec512), sys.AMD64)

		// sfp4 is intrinsic-if-constant, but otherwise it's complicated enough to just implement in Go.
		sfp4 := func(method string, hwop ssa.Op, vectype *types.Type) {
			addF(simdPackage, method,
				func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
					x, a, b, c, d, y := args[0], args[1], args[2], args[3], args[4], args[5]
					if a.Op == ssa.OpConst8 && b.Op == ssa.OpConst8 && c.Op == ssa.OpConst8 && d.Op == ssa.OpConst8 {
						z := select4FromPair(x, a, b, c, d, y, s, hwop, vectype)
						if z != nil {
							return z
						}
					}
					return s.callResult(n, callNormal)
				},
				sys.AMD64)
		}

		sfp4("Int32x4.ConcatPermuteScalars", ssa.OpconcatSelectedConstantInt32x4, types.TypeVec128)
		sfp4("Uint32x4.ConcatPermuteScalars", ssa.OpconcatSelectedConstantUint32x4, types.TypeVec128)
		sfp4("Float32x4.ConcatPermuteScalars", ssa.OpconcatSelectedConstantFloat32x4, types.TypeVec128)

		sfp4("Int32x8.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedInt32x8, types.TypeVec256)
		sfp4("Uint32x8.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedUint32x8, types.TypeVec256)
		sfp4("Float32x8.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedFloat32x8, types.TypeVec256)

		sfp4("Int32x16.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedInt32x16, types.TypeVec512)
		sfp4("Uint32x16.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedUint32x16, types.TypeVec512)
		sfp4("Float32x16.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedFloat32x16, types.TypeVec512)

		// sfp2 is intrinsic-if-constant, but otherwise it's complicated enough to just implement in Go.
		sfp2 := func(method string, hwop ssa.Op, vectype *types.Type, cscimm func(i, j uint8) int64) {
			addF(simdPackage, method,
				func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
					x, a, b, y := args[0], args[1], args[2], args[3]
					if a.Op == ssa.OpConst8 && b.Op == ssa.OpConst8 {
						z := select2FromPair(x, a, b, y, s, hwop, vectype, cscimm)
						if z != nil {
							return z
						}
					}
					return s.callResult(n, callNormal)
				},
				sys.AMD64)
		}

		sfp2("Uint64x2.ConcatPermuteScalars", ssa.OpconcatSelectedConstantUint64x2, types.TypeVec128, cscimm2)
		sfp2("Int64x2.ConcatPermuteScalars", ssa.OpconcatSelectedConstantInt64x2, types.TypeVec128, cscimm2)
		sfp2("Float64x2.ConcatPermuteScalars", ssa.OpconcatSelectedConstantFloat64x2, types.TypeVec128, cscimm2)

		sfp2("Uint64x4.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedUint64x4, types.TypeVec256, cscimm2g2)
		sfp2("Int64x4.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedInt64x4, types.TypeVec256, cscimm2g2)
		sfp2("Float64x4.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedFloat64x4, types.TypeVec256, cscimm2g2)

		sfp2("Uint64x8.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedUint64x8, types.TypeVec512, cscimm2g4)
		sfp2("Int64x8.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedInt64x8, types.TypeVec512, cscimm2g4)
		sfp2("Float64x8.ConcatPermuteScalarsGrouped", ssa.OpconcatSelectedConstantGroupedFloat64x8, types.TypeVec512, cscimm2g4)

	}
}

const simdPackage = "simd/archsimd"

func cscimm4(a, b, c, d uint8) int64 {
	return se(a + b<<2 + c<<4 + d<<6)
}

func cscimm2(a, b uint8) int64 {
	return se(a + b<<1)
}

func cscimm2g2(a, b uint8) int64 {
	g := cscimm2(a, b)
	return int64(int8(g + g<<2))
}

func cscimm2g4(a, b uint8) int64 {
	g := cscimm2g2(a, b)
	return int64(int8(g + g<<4))
}

const (
	_LLLL = iota
	_HLLL
	_LHLL
	_HHLL
	_LLHL
	_HLHL
	_LHHL
	_HHHL
	_LLLH
	_HLLH
	_LHLH
	_HHLH
	_LLHH
	_HLHH
	_LHHH
	_HHHH
)

const (
	_LL = iota
	_HL
	_LH
	_HH
)

func select2FromPair(x, _a, _b, y *ssa.Value, s *state, op ssa.Op, t *types.Type, csc func(a, b uint8) int64) *ssa.Value {
	a, b := uint8(_a.AuxInt8()), uint8(_b.AuxInt8())
	if a > 3 || b > 3 {
		return nil
	}
	pattern := (a&2)>>1 + (b & 2)
	a, b = a&1, b&1

	switch pattern {
	case _LL:
		return s.newValue2I(op, t, csc(a, b), x, x)
	case _HH:
		return s.newValue2I(op, t, csc(a, b), y, y)
	case _LH:
		return s.newValue2I(op, t, csc(a, b), x, y)
	case _HL:
		return s.newValue2I(op, t, csc(a, b), y, x)
	}
	panic("The preceding switch should have been exhaustive")
}

func select4FromPair(x, _a, _b, _c, _d, y *ssa.Value, s *state, op ssa.Op, t *types.Type) *ssa.Value {
	a, b, c, d := uint8(_a.AuxInt8()), uint8(_b.AuxInt8()), uint8(_c.AuxInt8()), uint8(_d.AuxInt8())
	if a > 7 || b > 7 || c > 7 || d > 7 {
		return nil
	}
	pattern := a>>2 + (b&4)>>1 + (c & 4) + (d&4)<<1

	a, b, c, d = a&3, b&3, c&3, d&3

	switch pattern {
	case _LLLL:
		// TODO DETECT 0,1,2,3, 0,0,0,0
		return s.newValue2I(op, t, cscimm4(a, b, c, d), x, x)
	case _HHHH:
		// TODO DETECT 0,1,2,3, 0,0,0,0
		return s.newValue2I(op, t, cscimm4(a, b, c, d), y, y)
	case _LLHH:
		return s.newValue2I(op, t, cscimm4(a, b, c, d), x, y)
	case _HHLL:
		return s.newValue2I(op, t, cscimm4(a, b, c, d), y, x)

	case _HLLL:
		z := s.newValue2I(op, t, cscimm4(a, a, b, b), y, x)
		return s.newValue2I(op, t, cscimm4(0, 2, c, d), z, x)
	case _LHLL:
		z := s.newValue2I(op, t, cscimm4(a, a, b, b), x, y)
		return s.newValue2I(op, t, cscimm4(0, 2, c, d), z, x)
	case _HLHH:
		z := s.newValue2I(op, t, cscimm4(a, a, b, b), y, x)
		return s.newValue2I(op, t, cscimm4(0, 2, c, d), z, y)
	case _LHHH:
		z := s.newValue2I(op, t, cscimm4(a, a, b, b), x, y)
		return s.newValue2I(op, t, cscimm4(0, 2, c, d), z, y)

	case _LLLH:
		z := s.newValue2I(op, t, cscimm4(c, c, d, d), x, y)
		return s.newValue2I(op, t, cscimm4(a, b, 0, 2), x, z)
	case _LLHL:
		z := s.newValue2I(op, t, cscimm4(c, c, d, d), y, x)
		return s.newValue2I(op, t, cscimm4(a, b, 0, 2), x, z)

	case _HHLH:
		z := s.newValue2I(op, t, cscimm4(c, c, d, d), x, y)
		return s.newValue2I(op, t, cscimm4(a, b, 0, 2), y, z)

	case _HHHL:
		z := s.newValue2I(op, t, cscimm4(c, c, d, d), y, x)
		return s.newValue2I(op, t, cscimm4(a, b, 0, 2), y, z)

	case _LHLH:
		z := s.newValue2I(op, t, cscimm4(a, c, b, d), x, y)
		return s.newValue2I(op, t, se(0b11_01_10_00), z, z)
	case _HLHL:
		z := s.newValue2I(op, t, cscimm4(b, d, a, c), x, y)
		return s.newValue2I(op, t, se(0b01_11_00_10), z, z)
	case _HLLH:
		z := s.newValue2I(op, t, cscimm4(b, c, a, d), x, y)
		return s.newValue2I(op, t, se(0b11_01_00_10), z, z)
	case _LHHL:
		z := s.newValue2I(op, t, cscimm4(a, d, b, c), x, y)
		return s.newValue2I(op, t, se(0b01_11_10_00), z, z)
	}
	panic("The preceding switch should have been exhaustive")
}

// se smears the not-really-a-sign bit of a uint8 to conform to the conventions
// for representing AuxInt in ssa.
func se(x uint8) int64 {
	return int64(int8(x))
}

func opLen1(op ssa.Op, t *types.Type) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue1(op, t, args[0])
	}
}

func opLen2(op ssa.Op, t *types.Type) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue2(op, t, args[0], args[1])
	}
}

func opLen2_21(op ssa.Op, t *types.Type) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue2(op, t, args[1], args[0])
	}
}

func opLen3(op ssa.Op, t *types.Type) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue3(op, t, args[0], args[1], args[2])
	}
}

var ssaVecBySize = map[int64]*types.Type{
	16: types.TypeVec128,
	32: types.TypeVec256,
	64: types.TypeVec512,
}

func opLen3_31Zero3(op ssa.Op, t *types.Type) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if t, ok := ssaVecBySize[args[1].Type.Size()]; !ok {
			panic("unknown simd vector size")
		} else {
			return s.newValue3(op, t, s.newValue0(ssa.OpZeroSIMD, t), args[1], args[0])
		}
	}
}

func opLen3_21(op ssa.Op, t *types.Type) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue3(op, t, args[1], args[0], args[2])
	}
}

func opLen3_231(op ssa.Op, t *types.Type) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue3(op, t, args[2], args[0], args[1])
	}
}

func opLen4(op ssa.Op, t *types.Type) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue4(op, t, args[0], args[1], args[2], args[3])
	}
}

func opLen4_231(op ssa.Op, t *types.Type) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue4(op, t, args[2], args[0], args[1], args[3])
	}
}

func opLen4_31(op ssa.Op, t *types.Type) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue4(op, t, args[2], args[1], args[0], args[3])
	}
}

func immJumpTable(s *state, idx *ssa.Value, intrinsicCall *ir.CallExpr, genOp func(*state, int)) *ssa.Value {
	if base.Ctxt.Retpoline {
		// Note spectre=all implies retpoline which requires binary search instead of table switch.
		return branchTableImm8(s, idx, intrinsicCall, genOp)
	}

	// Make blocks we'll need.
	bEnd := s.f.NewBlock(ssa.BlockPlain)

	if !idx.Type.IsKind(types.TUINT8) && !idx.Type.IsKind(types.TUINT64) {
		panic("immJumpTable expects uint8 or uint64 value")
	}

	// We will exhaust 0-255, so no need to check the bounds.
	t := types.Types[types.TUINTPTR]
	idx = s.conv(nil, idx, idx.Type, t)

	b := s.curBlock
	b.Kind = ssa.BlockJumpTable
	b.Pos = intrinsicCall.Pos()

	b.SetControl(idx)
	targets := [256]*ssa.Block{}
	for i := range 256 {
		t := s.f.NewBlock(ssa.BlockPlain)
		targets[i] = t
		b.AddEdgeTo(t)
	}
	s.endBlock()

	for i, t := range targets {
		s.startBlock(t)
		genOp(s, i)
		if t.Kind != ssa.BlockExit {
			t.AddEdgeTo(bEnd)
		}
		s.endBlock()
	}

	s.startBlock(bEnd)
	ret := s.variable(intrinsicCall, intrinsicCall.Type())
	return ret
}

func branchTableImm8(s *state, idx *ssa.Value, intrinsicCall *ir.CallExpr, genOp func(*state, int)) *ssa.Value {
	return branchTableN(s, idx, intrinsicCall, genOp, 256, true)
}

func branchTableN(s *state, idx *ssa.Value, intrinsicCall *ir.CallExpr, genOp func(*state, int), immLimit uint64, preChecked bool) *ssa.Value {
	// Make blocks we'll need.
	bEnd := s.f.NewBlock(ssa.BlockPlain)
	bPanic := s.f.NewBlock(ssa.BlockPlain)

	jt := s.f.NewBlock(ssa.BlockPlain)

	t := types.Types[types.TUINTPTR]
	idx = s.conv(nil, idx, idx.Type, t)

	if !preChecked {
		// Begin with a bounds check
		width := s.uintptrConstant(immLimit)
		cmp := s.newValue2(s.ssaOp(ir.OLT, t), types.Types[types.TBOOL], idx, width)
		bb := s.endBlock()
		bb.Kind = ssa.BlockIf
		bb.SetControl(cmp)
		bb.AddEdgeTo(jt)             // in range - use jump table
		bb.AddEdgeTo(bPanic)         // out of range - panic
		bb.Likely = ssa.BranchLikely // panic is unlikely

		s.startBlock(bPanic)
		s.rtcall(ir.Syms.PanicSimdImm, false, nil)
	}
	if s.curBlock != nil {
		bb := s.endBlock()
		bb.AddEdgeTo(jt)
	}

	s.startBlock(jt)
	jt.Kind = ssa.BlockPlain
	jt.Pos = intrinsicCall.Pos()

	branchTableNInner(s, idx, 0, immLimit, genOp, bEnd)

	s.startBlock(bEnd)
	ret := s.variable(intrinsicCall, intrinsicCall.Type())
	return ret
}

func branchTableNInner(s *state, idx *ssa.Value, lowInclusive, len uint64, genOp func(*state, int), bEnd *ssa.Block) {
	t := types.Types[types.TUINTPTR]
	if len == 0 {
		panic("empty branch table")
	}
	if len == 1 {
		genOp(s, int(lowInclusive+len-1))
		if s.curBlock != nil { // if genOp was "panic" then curBlock is already ended and nil
			if s.curBlock.Kind != ssa.BlockExit {
				s.curBlock.AddEdgeTo(bEnd)
			}
			s.endBlock()
		}
		return
	}

	s.curBlock.Kind = ssa.BlockIf
	cmp := s.newValue2(s.ssaOp(ir.OLT, t), types.Types[types.TBOOL], idx, s.uintptrConstant(lowInclusive+len/2))
	bb := s.endBlock()
	bb.Kind = ssa.BlockIf
	bb.SetControl(cmp)
	bMatch := s.f.NewBlock(ssa.BlockPlain)
	bNext := s.f.NewBlock(ssa.BlockPlain)
	bb.AddEdgeTo(bMatch)
	bb.AddEdgeTo(bNext)
	s.startBlock(bMatch)
	branchTableNInner(s, idx, lowInclusive, len/2, genOp, bEnd)
	s.startBlock(bNext)
	branchTableNInner(s, idx, lowInclusive+len/2, len-len/2, genOp, bEnd)
}

// immJumpTableN emits a jump table to one of a number of indexed cases, from zero to n-1.
// an index of n or larger will panic
func immJumpTableN(s *state, idx *ssa.Value, intrinsicCall *ir.CallExpr, immLimit uint64, genOp func(*state, int)) *ssa.Value {

	if !idx.Type.IsKind(types.TUINT8) && !idx.Type.IsKind(types.TUINT64) {
		s.Fatalf("immJumpTable expects uint8 or uint64 value, saw %v instead, val=%s", idx.Type.String(), idx.LongString())
	}

	if base.Flag.N != 0 || !Arch.LinkArch.CanJumpTable || base.Ctxt.Retpoline {
		return branchTableN(s, idx, intrinsicCall, genOp, immLimit, false)
	}

	// Make blocks we'll need.
	bEnd := s.f.NewBlock(ssa.BlockPlain)
	bPanic := s.f.NewBlock(ssa.BlockPlain)

	jt := s.f.NewBlock(ssa.BlockJumpTable)

	t := types.Types[types.TUINTPTR]
	idx = s.conv(nil, idx, idx.Type, t)
	width := s.uintptrConstant(immLimit)

	// Begin with a bounds check
	cmp := s.newValue2(s.ssaOp(ir.OLT, t), types.Types[types.TBOOL], idx, width)
	bb := s.endBlock()
	bb.Kind = ssa.BlockIf
	bb.SetControl(cmp)
	bb.AddEdgeTo(jt)             // in range - use jump table
	bb.AddEdgeTo(bPanic)         // out of range - panic
	bb.Likely = ssa.BranchLikely // panic is unlikely

	s.startBlock(bPanic)
	s.rtcall(ir.Syms.PanicSimdImm, false, nil)
	s.endBlock()

	s.startBlock(jt)
	jt.Kind = ssa.BlockJumpTable
	jt.Pos = intrinsicCall.Pos()
	if base.Flag.Cfg.SpectreIndex {
		// Potential Spectre vulnerability hardening?
		idx = s.newValue2(ssa.OpSpectreSliceIndex, t, idx, s.uintptrConstant(immLimit-1))
	}
	jt.SetControl(idx)
	targets := make([]*ssa.Block, immLimit, immLimit)
	for i := range immLimit {
		t := s.f.NewBlock(ssa.BlockPlain)
		targets[i] = t
		jt.AddEdgeTo(t)
	}
	s.endBlock()

	for i, t := range targets {
		s.startBlock(t)
		genOp(s, i)
		if t.Kind != ssa.BlockExit {
			t.AddEdgeTo(bEnd)
		}
		s.endBlock()
	}

	s.startBlock(bEnd)
	ret := s.variable(intrinsicCall, intrinsicCall.Type())
	return ret
}

func opLen1Imm8(op ssa.Op, t *types.Type, offset int) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if args[1].Op == ssa.OpConst8 || args[1].Op == ssa.OpConst64 {
			return s.newValue1I(op, t, int64(int8(args[1].AuxInt<<int64(offset))), args[0])
		}
		return immJumpTable(s, args[1], n, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			s.vars[n] = sNew.newValue1I(op, t, int64(int8(idx<<offset)), args[0])
		})
	}
}

func opLen2Imm8(op ssa.Op, t *types.Type, offset int) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if args[1].Op == ssa.OpConst8 || args[1].Op == ssa.OpConst64 {
			return s.newValue2I(op, t, int64(int8(args[1].AuxInt<<int64(offset))), args[0], args[2])
		}
		return immJumpTable(s, args[1], n, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			s.vars[n] = sNew.newValue2I(op, t, int64(int8(idx<<offset)), args[0], args[2])
		})
	}
}

func opLen3Imm8(op ssa.Op, t *types.Type, offset int) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if args[1].Op == ssa.OpConst8 || args[1].Op == ssa.OpConst64 {
			return s.newValue3I(op, t, int64(int8(args[1].AuxInt<<int64(offset))), args[0], args[2], args[3])
		}
		return immJumpTable(s, args[1], n, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			s.vars[n] = sNew.newValue3I(op, t, int64(int8(idx<<offset)), args[0], args[2], args[3])
		})
	}
}

func opLen2Imm8_2I(op ssa.Op, t *types.Type, offset int) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if args[2].Op == ssa.OpConst8 || args[2].Op == ssa.OpConst64 {
			return s.newValue2I(op, t, int64(int8(args[2].AuxInt<<int64(offset))), args[0], args[1])
		}
		return immJumpTable(s, args[2], n, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			s.vars[n] = sNew.newValue2I(op, t, int64(int8(idx<<offset)), args[0], args[1])
		})
	}
}

// Two immediates instead of just 1.  Offset is ignored, so it is a _ parameter instead.
func opLen2Imm8_II(op ssa.Op, t *types.Type, _ int) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if (args[1].Op == ssa.OpConst8 || args[1].Op == ssa.OpConst64) && (args[2].Op == ssa.OpConst8 || args[2].Op == ssa.OpConst64) && args[1].AuxInt & ^3 == 0 && args[2].AuxInt & ^3 == 0 {
			i1, i2 := args[1].AuxInt, args[2].AuxInt
			return s.newValue2I(op, t, int64(int8(i1+i2<<4)), args[0], args[3])
		}
		four := s.constInt64(types.Types[types.TUINT8], 4)
		shifted := s.newValue2(ssa.OpLsh8x8, types.Types[types.TUINT8], args[2], four)
		combined := s.newValue2(ssa.OpAdd8, types.Types[types.TUINT8], args[1], shifted)
		return immJumpTable(s, combined, n, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			// TODO for "zeroing" values, panic instead.
			if idx & ^(3+3<<4) == 0 {
				s.vars[n] = sNew.newValue2I(op, t, int64(int8(idx)), args[0], args[3])
			} else {
				sNew.rtcall(ir.Syms.PanicSimdImm, false, nil)
			}
		})
	}
}

// The assembler requires the imm value of a SHA1RNDS4 instruction to be one of 0,1,2,3...
func opLen2Imm8_SHA1RNDS4(op ssa.Op, t *types.Type, offset int) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if args[1].Op == ssa.OpConst8 || args[1].Op == ssa.OpConst64 {
			return s.newValue2I(op, t, int64(int8((args[1].AuxInt<<int64(offset))&0b11)), args[0], args[2])
		}
		return immJumpTable(s, args[1], n, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			s.vars[n] = sNew.newValue2I(op, t, int64(int8(idx<<offset))&0b11, args[0], args[2])
		})
	}
}

func opLen1Imm(op ssa.Op, t *types.Type, offset int, immMax uint64) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if (args[1].Op == ssa.OpConst8 || args[1].Op == ssa.OpConst64) && uint64(args[1].AuxInt) <= immMax {
			return s.newValue1I(op, t, int64(int8(args[1].AuxInt<<int64(offset))), args[0])
		}
		return immJumpTableN(s, args[1], n, immMax+1, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			s.vars[n] = sNew.newValue1I(op, t, int64(int8(idx<<offset)), args[0])
		})
	}
}

func opLen2Imm(op ssa.Op, t *types.Type, offset int, immMax uint64) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if (args[1].Op == ssa.OpConst8 || args[1].Op == ssa.OpConst64) && uint64(args[1].AuxInt) <= immMax {
			return s.newValue2I(op, t, int64(int8(args[1].AuxInt<<int64(offset))), args[0], args[2])
		}
		return immJumpTableN(s, args[1], n, immMax+1, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			s.vars[n] = sNew.newValue2I(op, t, int64(int8(idx<<offset)), args[0], args[2])
		})
	}
}

func opLen3Imm(op ssa.Op, t *types.Type, offset int, immMax uint64) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if (args[1].Op == ssa.OpConst8 || args[1].Op == ssa.OpConst64) && uint64(args[1].AuxInt) <= immMax {
			return s.newValue3I(op, t, int64(int8(args[1].AuxInt<<int64(offset))), args[0], args[2], args[3])
		}
		return immJumpTableN(s, args[1], n, immMax+1, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			s.vars[n] = sNew.newValue3I(op, t, int64(int8(idx<<offset)), args[0], args[2], args[3])
		})
	}
}

func opLen2Imm_2I(op ssa.Op, t *types.Type, offset int, immMax uint64) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if (args[2].Op == ssa.OpConst8 || args[2].Op == ssa.OpConst64) && uint64(args[2].AuxInt) <= immMax {
			return s.newValue2I(op, t, int64(int8(args[2].AuxInt<<int64(offset))), args[0], args[1])
		}
		return immJumpTableN(s, args[2], n, immMax+1, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			s.vars[n] = sNew.newValue2I(op, t, int64(int8(idx<<offset)), args[0], args[1])
		})
	}
}

func opLen3Imm8_2I(op ssa.Op, t *types.Type, offset int) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if args[2].Op == ssa.OpConst8 || args[2].Op == ssa.OpConst64 {
			return s.newValue3I(op, t, int64(int8(args[2].AuxInt<<int64(offset))), args[0], args[1], args[3])
		}
		return immJumpTable(s, args[2], n, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			s.vars[n] = sNew.newValue3I(op, t, int64(int8(idx<<offset)), args[0], args[1], args[3])
		})
	}
}

func opLen4Imm8(op ssa.Op, t *types.Type, offset int) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		if args[1].Op == ssa.OpConst8 || args[1].Op == ssa.OpConst64 {
			return s.newValue4I(op, t, int64(int8(args[1].AuxInt<<int64(offset))), args[0], args[2], args[3], args[4])
		}
		return immJumpTable(s, args[1], n, func(sNew *state, idx int) {
			// Encode as int8 due to requirement of AuxInt, check its comment for details.
			s.vars[n] = sNew.newValue4I(op, t, int64(int8(idx<<offset)), args[0], args[2], args[3], args[4])
		})
	}
}

func simdBroadcast(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue2(op, n.Type(), args[0], s.mem())
	}
}

func simdLoad() func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue2(ssa.OpLoad, n.Type(), args[0], s.mem())
	}
}

func simdStore() func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		s.store(args[0].Type, args[1], args[0])
		return nil
	}
}

var cvtVToMaskOpcodes = map[int]map[int]ssa.Op{
	8:  {16: ssa.OpCvt16toMask8x16, 32: ssa.OpCvt32toMask8x32, 64: ssa.OpCvt64toMask8x64},
	16: {8: ssa.OpCvt8toMask16x8, 16: ssa.OpCvt16toMask16x16, 32: ssa.OpCvt32toMask16x32},
	32: {4: ssa.OpCvt8toMask32x4, 8: ssa.OpCvt8toMask32x8, 16: ssa.OpCvt16toMask32x16},
	64: {2: ssa.OpCvt8toMask64x2, 4: ssa.OpCvt8toMask64x4, 8: ssa.OpCvt8toMask64x8},
}

var cvtMaskToVOpcodes = map[int]map[int]ssa.Op{
	8:  {16: ssa.OpCvtMask8x16to16, 32: ssa.OpCvtMask8x32to32, 64: ssa.OpCvtMask8x64to64},
	16: {8: ssa.OpCvtMask16x8to8, 16: ssa.OpCvtMask16x16to16, 32: ssa.OpCvtMask16x32to32},
	32: {4: ssa.OpCvtMask32x4to8, 8: ssa.OpCvtMask32x8to8, 16: ssa.OpCvtMask32x16to16},
	64: {2: ssa.OpCvtMask64x2to8, 4: ssa.OpCvtMask64x4to8, 8: ssa.OpCvtMask64x8to8},
}

func simdCvtVToMask(elemBits, lanes int) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		op := cvtVToMaskOpcodes[elemBits][lanes]
		if op == 0 {
			panic(fmt.Sprintf("Unknown mask shape: Mask%dx%d", elemBits, lanes))
		}
		return s.newValue1(op, types.TypeMask, args[0])
	}
}

func simdCvtMaskToV(elemBits, lanes int) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		op := cvtMaskToVOpcodes[elemBits][lanes]
		if op == 0 {
			panic(fmt.Sprintf("Unknown mask shape: Mask%dx%d", elemBits, lanes))
		}
		return s.newValue1(op, n.Type(), args[0])
	}
}

func simdMaskedLoad(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return s.newValue3(op, n.Type(), args[0], args[1], s.mem())
	}
}

func simdMaskedStore(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
	return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		s.vars[memVar] = s.newValue4A(op, types.TypeMem, args[0].Type, args[1], args[2], args[0], s.mem())
		return nil
	}
}

// findIntrinsic returns a function which builds the SSA equivalent of the
// function identified by the symbol sym.  If sym is not an intrinsic call, returns nil.
func findIntrinsic(sym *types.Sym) intrinsicBuilder {
	if sym == nil || sym.Pkg == nil {
		return nil
	}
	pkg := sym.Pkg.Path
	if sym.Pkg == ir.Pkgs.Runtime {
		pkg = "runtime"
	}
	if base.Flag.Race && pkg == "sync/atomic" {
		// The race detector needs to be able to intercept these calls.
		// We can't intrinsify them.
		return nil
	}
	// Skip intrinsifying math functions (which may contain hard-float
	// instructions) when soft-float
	if Arch.SoftFloat && pkg == "math" {
		return nil
	}

	fn := sym.Name
	if ssa.IntrinsicsDisable {
		if pkg == "internal/runtime/sys" && (fn == "GetCallerPC" || fn == "GetCallerSP" || fn == "GetClosurePtr") ||
			pkg == simdPackage {
			// These runtime functions don't have definitions, must be intrinsics.
		} else {
			return nil
		}
	}
	return intrinsics.lookup(Arch.LinkArch.Arch, pkg, fn)
}

func IsIntrinsicCall(n *ir.CallExpr) bool {
	if n == nil {
		return false
	}
	name, ok := n.Fun.(*ir.Name)
	if !ok {
		if n.Fun.Op() == ir.OMETHEXPR {
			if meth := ir.MethodExprName(n.Fun); meth != nil {
				if fn := meth.Func; fn != nil {
					return IsIntrinsicSym(fn.Sym())
				}
			}
		}
		return false
	}
	return IsIntrinsicSym(name.Sym())
}

func IsIntrinsicSym(sym *types.Sym) bool {
	return findIntrinsic(sym) != nil
}

// GenIntrinsicBody generates the function body for a bodyless intrinsic.
// This is used when the intrinsic is used in a non-call context, e.g.
// as a function pointer, or (for a method) being referenced from the type
// descriptor.
//
// The compiler already recognizes a call to fn as an intrinsic and can
// directly generate code for it. So we just fill in the body with a call
// to fn.
func GenIntrinsicBody(fn *ir.Func) {
	if ir.CurFunc != nil {
		base.FatalfAt(fn.Pos(), "enqueueFunc %v inside %v", fn, ir.CurFunc)
	}

	if base.Flag.LowerR != 0 {
		fmt.Println("generate intrinsic for", ir.FuncName(fn))
	}

	pos := fn.Pos()
	ft := fn.Type()
	var ret ir.Node

	// For a method, it usually starts with an ODOTMETH (pre-typecheck) or
	// OMETHEXPR (post-typecheck) referencing the method symbol without the
	// receiver type, and Walk rewrites it to a call directly to the
	// type-qualified method symbol, moving the receiver to an argument.
	// Here fn has already the type-qualified method symbol, and it is hard
	// to get the unqualified symbol. So we just generate the post-Walk form
	// and mark it typechecked and Walked.
	call := ir.NewCallExpr(pos, ir.OCALLFUNC, fn.Nname, nil)
	call.Args = ir.RecvParamNames(ft)
	call.IsDDD = ft.IsVariadic()
	typecheck.Exprs(call.Args)
	call.SetTypecheck(1)
	call.SetWalked(true)
	ret = call
	if ft.NumResults() > 0 {
		if ft.NumResults() == 1 {
			call.SetType(ft.Result(0).Type)
		} else {
			call.SetType(ft.ResultsTuple())
		}
		n := ir.NewReturnStmt(base.Pos, nil)
		n.Results = []ir.Node{call}
		ret = n
	}
	fn.Body.Append(ret)

	if base.Flag.LowerR != 0 {
		ir.DumpList("generate intrinsic body", fn.Body)
	}

	ir.CurFunc = fn
	typecheck.Stmts(fn.Body)
	ir.CurFunc = nil // we know CurFunc is nil at entry
}

```

// === FILE: references/go/src/cmd/compile/internal/ssagen/nowb.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssagen

import (
	"fmt"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/src"
)

func EnableNoWriteBarrierRecCheck() {
	nowritebarrierrecCheck = newNowritebarrierrecChecker()
}

func NoWriteBarrierRecCheck() {
	// Write barriers are now known. Check the
	// call graph.
	nowritebarrierrecCheck.check()
	nowritebarrierrecCheck = nil
}

var nowritebarrierrecCheck *nowritebarrierrecChecker

type nowritebarrierrecChecker struct {
	// extraCalls contains extra function calls that may not be
	// visible during later analysis. It maps from the ODCLFUNC of
	// the caller to a list of callees.
	extraCalls map[*ir.Func][]nowritebarrierrecCall

	// curfn is the current function during AST walks.
	curfn *ir.Func
}

type nowritebarrierrecCall struct {
	target *ir.Func // caller or callee
	lineno src.XPos // line of call
}

// newNowritebarrierrecChecker creates a nowritebarrierrecChecker. It
// must be called before walk.
func newNowritebarrierrecChecker() *nowritebarrierrecChecker {
	c := &nowritebarrierrecChecker{
		extraCalls: make(map[*ir.Func][]nowritebarrierrecCall),
	}

	// Find all systemstack calls and record their targets. In
	// general, flow analysis can't see into systemstack, but it's
	// important to handle it for this check, so we model it
	// directly. This has to happen before transforming closures in walk since
	// it's a lot harder to work out the argument after.
	for _, n := range typecheck.Target.Funcs {
		c.curfn = n
		if c.curfn.ABIWrapper() {
			// We only want "real" calls to these
			// functions, not the generated ones within
			// their own ABI wrappers.
			continue
		}
		ir.Visit(n, c.findExtraCalls)
	}
	c.curfn = nil
	return c
}

func (c *nowritebarrierrecChecker) findExtraCalls(nn ir.Node) {
	if nn.Op() != ir.OCALLFUNC {
		return
	}
	n := nn.(*ir.CallExpr)
	if n.Fun == nil || n.Fun.Op() != ir.ONAME {
		return
	}
	fn := n.Fun.(*ir.Name)
	if fn.Class != ir.PFUNC || fn.Defn == nil {
		return
	}
	if types.RuntimeSymName(fn.Sym()) != "systemstack" {
		return
	}

	var callee *ir.Func
	arg := n.Args[0]
	switch arg.Op() {
	case ir.ONAME:
		arg := arg.(*ir.Name)
		callee = arg.Defn.(*ir.Func)
	case ir.OCLOSURE:
		arg := arg.(*ir.ClosureExpr)
		callee = arg.Func
	default:
		base.Fatalf("expected ONAME or OCLOSURE node, got %+v", arg)
	}
	c.extraCalls[c.curfn] = append(c.extraCalls[c.curfn], nowritebarrierrecCall{callee, n.Pos()})
}

// recordCall records a call from ODCLFUNC node "from", to function
// symbol "to" at position pos.
//
// This should be done as late as possible during compilation to
// capture precise call graphs. The target of the call is an LSym
// because that's all we know after we start SSA.
//
// This can be called concurrently for different from Nodes.
func (c *nowritebarrierrecChecker) recordCall(fn *ir.Func, to *obj.LSym, pos src.XPos) {
	// We record this information on the *Func so this is concurrent-safe.
	if fn.NWBRCalls == nil {
		fn.NWBRCalls = new([]ir.SymAndPos)
	}
	*fn.NWBRCalls = append(*fn.NWBRCalls, ir.SymAndPos{Sym: to, Pos: pos})
}

func (c *nowritebarrierrecChecker) check() {
	// We walk the call graph as late as possible so we can
	// capture all calls created by lowering, but this means we
	// only get to see the obj.LSyms of calls. symToFunc lets us
	// get back to the ODCLFUNCs.
	symToFunc := make(map[*obj.LSym]*ir.Func)
	// funcs records the back-edges of the BFS call graph walk. It
	// maps from the ODCLFUNC of each function that must not have
	// write barriers to the call that inhibits them. Functions
	// that are directly marked go:nowritebarrierrec are in this
	// map with a zero-valued nowritebarrierrecCall. This also
	// acts as the set of marks for the BFS of the call graph.
	funcs := make(map[*ir.Func]nowritebarrierrecCall)
	// q is the queue of ODCLFUNC Nodes to visit in BFS order.
	var q ir.NameQueue

	for _, fn := range typecheck.Target.Funcs {
		symToFunc[fn.LSym] = fn

		// Make nowritebarrierrec functions BFS roots.
		if fn.Pragma&ir.Nowritebarrierrec != 0 {
			funcs[fn] = nowritebarrierrecCall{}
			q.PushRight(fn.Nname)
		}
		// Check go:nowritebarrier functions.
		if fn.Pragma&ir.Nowritebarrier != 0 && fn.WBPos.IsKnown() {
			base.ErrorfAt(fn.WBPos, 0, "write barrier prohibited")
		}
	}

	// Perform a BFS of the call graph from all
	// go:nowritebarrierrec functions.
	enqueue := func(src, target *ir.Func, pos src.XPos) {
		if target.Pragma&ir.Yeswritebarrierrec != 0 {
			// Don't flow into this function.
			return
		}
		if _, ok := funcs[target]; ok {
			// Already found a path to target.
			return
		}

		// Record the path.
		funcs[target] = nowritebarrierrecCall{target: src, lineno: pos}
		q.PushRight(target.Nname)
	}
	for !q.Empty() {
		fn := q.PopLeft().Func

		// Check fn.
		if fn.WBPos.IsKnown() {
			var err strings.Builder
			call := funcs[fn]
			for call.target != nil {
				fmt.Fprintf(&err, "\n\t%v: called by %v", base.FmtPos(call.lineno), call.target.Nname)
				call = funcs[call.target]
			}
			// Seeing this error in a failed CI run? It indicates that
			// a function in the runtime package marked nowritebarrierrec
			// (the outermost stack element) was found, by a static
			// reachability analysis over the fully lowered optimized code,
			// to call a function (fn) that involves a write barrier.
			//
			// Even if the call path is infeasable,
			// you will need to reorganize the code to avoid it.
			base.ErrorfAt(fn.WBPos, 0, "write barrier prohibited by caller; %v%s", fn.Nname, err.String())
			continue
		}

		// Enqueue fn's calls.
		for _, callee := range c.extraCalls[fn] {
			enqueue(fn, callee.target, callee.lineno)
		}
		if fn.NWBRCalls == nil {
			continue
		}
		for _, callee := range *fn.NWBRCalls {
			target := symToFunc[callee.Sym]
			if target != nil {
				enqueue(fn, target, callee.Pos)
			}
		}
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/ssagen/pgen.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssagen

import (
	"fmt"
	"internal/buildcfg"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"

	"cmd/compile/internal/base"
	"cmd/compile/internal/inline"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/liveness"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/pgoir"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"
)

// cmpstackvarlt reports whether the stack variable a sorts before b.
func cmpstackvarlt(a, b *ir.Name, mls *liveness.MergeLocalsState) bool {
	// Sort non-autos before autos.
	if needAlloc(a) != needAlloc(b) {
		return needAlloc(b)
	}

	// If both are non-auto (e.g., parameters, results), then sort by
	// frame offset (defined by ABI).
	if !needAlloc(a) {
		return a.FrameOffset() < b.FrameOffset()
	}

	// From here on, a and b are both autos (i.e., local variables).

	// Sort followers after leaders, if mls != nil
	if mls != nil {
		aFollow := mls.Subsumed(a)
		bFollow := mls.Subsumed(b)
		if aFollow != bFollow {
			return bFollow
		}
	}

	// Sort used before unused (so AllocFrame can truncate unused
	// variables).
	if a.Used() != b.Used() {
		return a.Used()
	}

	// Sort pointer-typed before non-pointer types.
	// Keeps the stack's GC bitmap compact.
	ap := a.Type().HasPointers()
	bp := b.Type().HasPointers()
	if ap != bp {
		return ap
	}

	// Group variables that need zeroing, so we can efficiently zero
	// them altogether.
	ap = a.Needzero()
	bp = b.Needzero()
	if ap != bp {
		return ap
	}

	// Sort variables in descending alignment order, so we can optimally
	// pack variables into the frame.
	if a.Type().Alignment() != b.Type().Alignment() {
		return a.Type().Alignment() > b.Type().Alignment()
	}

	// Sort normal variables before open-coded-defer slots, so that the
	// latter are grouped together and near the top of the frame (to
	// minimize varint encoding of their varp offset).
	if a.OpenDeferSlot() != b.OpenDeferSlot() {
		return a.OpenDeferSlot()
	}

	// If a and b are both open-coded defer slots, then order them by
	// index in descending order, so they'll be laid out in the frame in
	// ascending order.
	//
	// Their index was saved in FrameOffset in state.openDeferSave.
	if a.OpenDeferSlot() {
		return a.FrameOffset() > b.FrameOffset()
	}

	// Tie breaker for stable results.
	return a.Sym().Name < b.Sym().Name
}

// needAlloc reports whether n is within the current frame, for which we need to
// allocate space. In particular, it excludes arguments and results, which are in
// the callers frame.
func needAlloc(n *ir.Name) bool {
	if n.Op() != ir.ONAME {
		base.FatalfAt(n.Pos(), "%v has unexpected Op %v", n, n.Op())
	}

	switch n.Class {
	case ir.PAUTO:
		return true
	case ir.PPARAM:
		return false
	case ir.PPARAMOUT:
		return n.IsOutputParamInRegisters()

	default:
		base.FatalfAt(n.Pos(), "%v has unexpected Class %v", n, n.Class)
		return false
	}
}

func (s *ssafn) AllocFrame(f *ssa.Func) {
	s.stksize = 0
	s.stkptrsize = 0
	s.stkalign = int64(types.RegSize)
	fn := s.curfn

	// Mark the PAUTO's unused.
	for _, ln := range fn.Dcl {
		if ln.OpenDeferSlot() {
			// Open-coded defer slots have indices that were assigned
			// upfront during SSA construction, but the defer statement can
			// later get removed during deadcode elimination (#61895). To
			// keep their relative offsets correct, treat them all as used.
			continue
		}

		if needAlloc(ln) {
			ln.SetUsed(false)
		}
	}

	for _, l := range f.RegAlloc {
		if ls, ok := l.(ssa.LocalSlot); ok {
			ls.N.SetUsed(true)
		}
	}

	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if n, ok := v.Aux.(*ir.Name); ok {
				switch n.Class {
				case ir.PPARAMOUT:
					if n.IsOutputParamInRegisters() && v.Op == ssa.OpVarDef {
						// ignore VarDef, look for "real" uses.
						// TODO: maybe do this for PAUTO as well?
						continue
					}
					fallthrough
				case ir.PPARAM, ir.PAUTO:
					n.SetUsed(true)
				}
			}
		}
	}

	var mls *liveness.MergeLocalsState
	var leaders map[*ir.Name]int64
	if base.Debug.MergeLocals != 0 {
		mls = liveness.MergeLocals(fn, f)
		if base.Debug.MergeLocalsTrace > 0 && mls != nil {
			savedNP, savedP := mls.EstSavings()
			fmt.Fprintf(os.Stderr, "%s: %d bytes of stack space saved via stack slot merging (%d nonpointer %d pointer)\n", ir.FuncName(fn), savedNP+savedP, savedNP, savedP)
			if base.Debug.MergeLocalsTrace > 1 {
				fmt.Fprintf(os.Stderr, "=-= merge locals state for %v:\n%v",
					fn, mls)
			}
		}
		leaders = make(map[*ir.Name]int64)
	}

	// Use sort.SliceStable instead of sort.Slice so stack layout (and thus
	// compiler output) is less sensitive to frontend changes that
	// introduce or remove unused variables.
	sort.SliceStable(fn.Dcl, func(i, j int) bool {
		return cmpstackvarlt(fn.Dcl[i], fn.Dcl[j], mls)
	})

	if mls != nil {
		// Rewrite fn.Dcl to reposition followers (subsumed vars) to
		// be immediately following the leader var in their partition.
		followers := []*ir.Name{}
		newdcl := make([]*ir.Name, 0, len(fn.Dcl))
		for i := 0; i < len(fn.Dcl); i++ {
			n := fn.Dcl[i]
			if mls.Subsumed(n) {
				continue
			}
			newdcl = append(newdcl, n)
			if mls.IsLeader(n) {
				followers = mls.Followers(n, followers)
				// position followers immediately after leader
				newdcl = append(newdcl, followers...)
			}
		}
		fn.Dcl = newdcl
	}

	if base.Debug.MergeLocalsTrace > 1 && mls != nil {
		fmt.Fprintf(os.Stderr, "=-= sorted DCL for %v:\n", fn)
		for i, v := range fn.Dcl {
			if !ssa.IsMergeCandidate(v) {
				continue
			}
			fmt.Fprintf(os.Stderr, " %d: %q isleader=%v subsumed=%v used=%v sz=%d align=%d t=%s\n", i, v.Sym().Name, mls.IsLeader(v), mls.Subsumed(v), v.Used(), v.Type().Size(), v.Type().Alignment(), v.Type().String())
		}
	}

	// Reassign stack offsets of the locals that are used.
	lastHasPtr := false
	for i, n := range fn.Dcl {
		if n.Op() != ir.ONAME || n.Class != ir.PAUTO && !(n.Class == ir.PPARAMOUT && n.IsOutputParamInRegisters()) {
			// i.e., stack assign if AUTO, or if PARAMOUT in registers (which has no predefined spill locations)
			continue
		}
		if mls != nil && mls.Subsumed(n) {
			continue
		}
		if !n.Used() {
			fn.DebugInfo.(*ssa.FuncDebug).OptDcl = fn.Dcl[i:]
			fn.Dcl = fn.Dcl[:i]
			break
		}
		types.CalcSize(n.Type())
		w := n.Type().Size()
		if w >= types.MaxWidth || w < 0 {
			base.Fatalf("bad width")
		}
		if w == 0 && lastHasPtr {
			// Pad between a pointer-containing object and a zero-sized object.
			// This prevents a pointer to the zero-sized object from being interpreted
			// as a pointer to the pointer-containing object (and causing it
			// to be scanned when it shouldn't be). See issue 24993.
			w = 1
		}
		s.stksize += w
		s.stksize = types.RoundUp(s.stksize, n.Type().Alignment())
		if n.Type().Alignment() > int64(types.RegSize) {
			s.stkalign = n.Type().Alignment()
		}
		if n.Type().HasPointers() {
			s.stkptrsize = s.stksize
			lastHasPtr = true
		} else {
			lastHasPtr = false
		}
		n.SetFrameOffset(-s.stksize)
		if mls != nil && mls.IsLeader(n) {
			leaders[n] = -s.stksize
		}
	}

	if mls != nil {
		// Update offsets of followers (subsumed vars) to be the
		// same as the leader var in their partition.
		for i := 0; i < len(fn.Dcl); i++ {
			n := fn.Dcl[i]
			if !mls.Subsumed(n) {
				continue
			}
			leader := mls.Leader(n)
			off, ok := leaders[leader]
			if !ok {
				panic("internal error missing leader")
			}
			// Set the stack offset this subsumed (followed) var
			// to be the same as the leader.
			n.SetFrameOffset(off)
		}

		if base.Debug.MergeLocalsTrace > 1 {
			fmt.Fprintf(os.Stderr, "=-= stack layout for %v:\n", fn)
			for i, v := range fn.Dcl {
				if v.Op() != ir.ONAME || (v.Class != ir.PAUTO && !(v.Class == ir.PPARAMOUT && v.IsOutputParamInRegisters())) {
					continue
				}
				fmt.Fprintf(os.Stderr, " %d: %q frameoff %d isleader=%v subsumed=%v sz=%d align=%d t=%s\n", i, v.Sym().Name, v.FrameOffset(), mls.IsLeader(v), mls.Subsumed(v), v.Type().Size(), v.Type().Alignment(), v.Type().String())
			}
		}
	}

	s.stksize = types.RoundUp(s.stksize, s.stkalign)
	s.stkptrsize = types.RoundUp(s.stkptrsize, s.stkalign)
}

const maxStackSize = 1 << 30

// Compile builds an SSA backend function,
// uses it to generate a plist,
// and flushes that plist to machine code.
// worker indicates which of the backend workers is doing the processing.
func Compile(fn *ir.Func, worker int, profile *pgoir.Profile) {
	f := buildssa(fn, worker, inline.IsPgoHotFunc(fn, profile) || inline.HasPgoHotInline(fn))
	// Note: check arg size to fix issue 25507.
	if f.Frontend().(*ssafn).stksize >= maxStackSize || f.OwnAux.ArgWidth() >= maxStackSize {
		largeStackFramesMu.Lock()
		largeStackFrames = append(largeStackFrames, largeStack{locals: f.Frontend().(*ssafn).stksize, args: f.OwnAux.ArgWidth(), pos: fn.Pos()})
		largeStackFramesMu.Unlock()
		return
	}
	pp := objw.NewProgs(fn, worker)
	defer pp.Free()
	genssa(f, pp)
	// Check frame size again.
	// The check above included only the space needed for local variables.
	// After genssa, the space needed includes local variables and the callee arg region.
	// We must do this check prior to calling pp.Flush.
	// If there are any oversized stack frames,
	// the assembler may emit inscrutable complaints about invalid instructions.
	if pp.Text.To.Offset >= maxStackSize {
		largeStackFramesMu.Lock()
		locals := f.Frontend().(*ssafn).stksize
		largeStackFrames = append(largeStackFrames, largeStack{locals: locals, args: f.OwnAux.ArgWidth(), callee: pp.Text.To.Offset - locals, pos: fn.Pos()})
		largeStackFramesMu.Unlock()
		return
	}

	pp.Flush() // assemble, fill in boilerplate, etc.

	// If we're compiling the package init function, search for any
	// relocations that target global map init outline functions and
	// turn them into weak relocs.
	if fn.IsPackageInit() && base.Debug.WrapGlobalMapCtl != 1 {
		weakenGlobalMapInitRelocs(fn)
	}

	// fieldtrack must be called after pp.Flush. See issue 20014.
	fieldtrack(pp.Text.From.Sym, fn.FieldTrack)
}

// globalMapInitLsyms records the LSym of each map.init.NNN outlined
// map initializer function created by the compiler.
var globalMapInitLsyms map[*obj.LSym]struct{}

// RegisterMapInitLsym records "s" in the set of outlined map initializer
// functions.
func RegisterMapInitLsym(s *obj.LSym) {
	if globalMapInitLsyms == nil {
		globalMapInitLsyms = make(map[*obj.LSym]struct{})
	}
	globalMapInitLsyms[s] = struct{}{}
}

// weakenGlobalMapInitRelocs walks through all of the relocations on a
// given a package init function "fn" and looks for relocs that target
// outlined global map initializer functions; if it finds any such
// relocs, it flags them as R_WEAK.
func weakenGlobalMapInitRelocs(fn *ir.Func) {
	if globalMapInitLsyms == nil {
		return
	}
	for i := range fn.LSym.R {
		tgt := fn.LSym.R[i].Sym
		if tgt == nil {
			continue
		}
		if _, ok := globalMapInitLsyms[tgt]; !ok {
			continue
		}
		if base.Debug.WrapGlobalMapDbg > 1 {
			fmt.Fprintf(os.Stderr, "=-= weakify fn %v reloc %d %+v\n", fn, i,
				fn.LSym.R[i])
		}
		// set the R_WEAK bit, leave rest of reloc type intact
		fn.LSym.R[i].Type |= objabi.R_WEAK
	}
}

// StackOffset returns the stack location of a LocalSlot relative to the
// stack pointer, suitable for use in a DWARF location entry. This has nothing
// to do with its offset in the user variable.
func StackOffset(slot ssa.LocalSlot) int32 {
	n := slot.N
	var off int64
	switch n.Class {
	case ir.PPARAM, ir.PPARAMOUT:
		if !n.IsOutputParamInRegisters() {
			off = n.FrameOffset() + base.Ctxt.Arch.FixedFrameSize
			break
		}
		fallthrough // PPARAMOUT in registers allocates like an AUTO
	case ir.PAUTO:
		off = n.FrameOffset()
		if base.Ctxt.Arch.FixedFrameSize == 0 {
			off -= int64(types.PtrSize)
		}
		if buildcfg.FramePointerEnabled {
			off -= int64(types.PtrSize)
		}
	}
	return int32(off + slot.Off)
}

// fieldtrack adds R_USEFIELD relocations to fnsym to record any
// struct fields that it used.
func fieldtrack(fnsym *obj.LSym, tracked map[*obj.LSym]struct{}) {
	if fnsym == nil {
		return
	}
	if !buildcfg.Experiment.FieldTrack || len(tracked) == 0 {
		return
	}

	trackSyms := make([]*obj.LSym, 0, len(tracked))
	for sym := range tracked {
		trackSyms = append(trackSyms, sym)
	}
	slices.SortFunc(trackSyms, func(a, b *obj.LSym) int { return strings.Compare(a.Name, b.Name) })
	for _, sym := range trackSyms {
		fnsym.AddRel(base.Ctxt, obj.Reloc{Type: objabi.R_USEFIELD, Sym: sym})
	}
}

// largeStack is info about a function whose stack frame is too large (rare).
type largeStack struct {
	locals int64
	args   int64
	callee int64
	pos    src.XPos
}

var (
	largeStackFramesMu sync.Mutex // protects largeStackFrames
	largeStackFrames   []largeStack
)

func CheckLargeStacks() {
	// Check whether any of the functions we have compiled have gigantic stack frames.
	sort.Slice(largeStackFrames, func(i, j int) bool {
		return largeStackFrames[i].pos.Before(largeStackFrames[j].pos)
	})
	for _, large := range largeStackFrames {
		if large.callee != 0 {
			base.ErrorfAt(large.pos, 0, "stack frame too large (>1GB): %d MB locals + %d MB args + %d MB callee", large.locals>>20, large.args>>20, large.callee>>20)
		} else {
			base.ErrorfAt(large.pos, 0, "stack frame too large (>1GB): %d MB locals + %d MB args", large.locals>>20, large.args>>20)
		}
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/ssagen/phi.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssagen

import (
	"container/heap"
	"fmt"

	"cmd/compile/internal/ir"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// This file contains the algorithm to place phi nodes in a function.
// For small functions, we use Braun, Buchwald, Hack, Leißa, Mallon, and Zwinkau.
// https://pp.info.uni-karlsruhe.de/uploads/publikationen/braun13cc.pdf
// For large functions, we use Sreedhar & Gao: A Linear Time Algorithm for Placing Φ-Nodes.
// http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.8.1979&rep=rep1&type=pdf

const smallBlocks = 500

const debugPhi = false

// fwdRefAux wraps an arbitrary ir.Node as an ssa.Aux for use with OpFwdref.
type fwdRefAux struct {
	_ [0]func() // ensure ir.Node isn't compared for equality
	N ir.Node
}

func (fwdRefAux) CanBeAnSSAAux() {}

// insertPhis finds all the places in the function where a phi is
// necessary and inserts them.
// Uses FwdRef ops to find all uses of variables, and s.defvars to find
// all definitions.
// Phi values are inserted, and all FwdRefs are changed to a Copy
// of the appropriate phi or definition.
// TODO: make this part of cmd/compile/internal/ssa somehow?
func (s *state) insertPhis() {
	if len(s.f.Blocks) <= smallBlocks {
		sps := simplePhiState{s: s, f: s.f, defvars: s.defvars}
		sps.insertPhis()
		return
	}
	ps := phiState{s: s, f: s.f, defvars: s.defvars}
	ps.insertPhis()
}

type phiState struct {
	s       *state                   // SSA state
	f       *ssa.Func                // function to work on
	defvars []map[ir.Node]*ssa.Value // defined variables at end of each block

	varnum map[ir.Node]int32 // variable numbering

	// properties of the dominator tree
	idom  []*ssa.Block // dominator parents
	tree  []domBlock   // dominator child+sibling
	level []int32      // level in dominator tree (0 = root or unreachable, 1 = children of root, ...)

	// scratch locations
	priq   blockHeap    // priority queue of blocks, higher level (toward leaves) = higher priority
	q      []*ssa.Block // inner loop queue
	queued *sparseSet   // has been put in q
	hasPhi *sparseSet   // has a phi
	hasDef *sparseSet   // has a write of the variable we're processing

	// miscellaneous
	placeholder *ssa.Value // value to use as a "not set yet" placeholder.
}

func (s *phiState) insertPhis() {
	if debugPhi {
		fmt.Println(s.f.String())
	}

	// Find all the variables for which we need to match up reads & writes.
	// This step prunes any basic-block-only variables from consideration.
	// Generate a numbering for these variables.
	s.varnum = map[ir.Node]int32{}
	var vars []ir.Node
	var vartypes []*types.Type
	for _, b := range s.f.Blocks {
		for _, v := range b.Values {
			if v.Op != ssa.OpFwdRef {
				continue
			}
			var_ := v.Aux.(fwdRefAux).N

			// Optimization: look back 1 block for the definition.
			if len(b.Preds) == 1 {
				c := b.Preds[0].Block()
				if w := s.defvars[c.ID][var_]; w != nil {
					v.Op = ssa.OpCopy
					v.Aux = nil
					v.AddArg(w)
					continue
				}
			}

			if _, ok := s.varnum[var_]; ok {
				continue
			}
			s.varnum[var_] = int32(len(vartypes))
			if debugPhi {
				fmt.Printf("var%d = %v\n", len(vartypes), var_)
			}
			vars = append(vars, var_)
			vartypes = append(vartypes, v.Type)
		}
	}

	if len(vartypes) == 0 {
		return
	}

	// Find all definitions of the variables we need to process.
	// defs[n] contains all the blocks in which variable number n is assigned.
	defs := make([][]*ssa.Block, len(vartypes))
	for _, b := range s.f.Blocks {
		for var_ := range s.defvars[b.ID] { // TODO: encode defvars some other way (explicit ops)? make defvars[n] a slice instead of a map.
			if n, ok := s.varnum[var_]; ok {
				defs[n] = append(defs[n], b)
			}
		}
	}

	// Make dominator tree.
	s.idom = s.f.Idom()
	s.tree = make([]domBlock, s.f.NumBlocks())
	for _, b := range s.f.Blocks {
		p := s.idom[b.ID]
		if p != nil {
			s.tree[b.ID].sibling = s.tree[p.ID].firstChild
			s.tree[p.ID].firstChild = b
		}
	}
	// Compute levels in dominator tree.
	// With parent pointers we can do a depth-first walk without
	// any auxiliary storage.
	s.level = make([]int32, s.f.NumBlocks())
	b := s.f.Entry
levels:
	for {
		if p := s.idom[b.ID]; p != nil {
			s.level[b.ID] = s.level[p.ID] + 1
			if debugPhi {
				fmt.Printf("level %s = %d\n", b, s.level[b.ID])
			}
		}
		if c := s.tree[b.ID].firstChild; c != nil {
			b = c
			continue
		}
		for {
			if c := s.tree[b.ID].sibling; c != nil {
				b = c
				continue levels
			}
			b = s.idom[b.ID]
			if b == nil {
				break levels
			}
		}
	}

	// Allocate scratch locations.
	s.priq.level = s.level
	s.q = make([]*ssa.Block, 0, s.f.NumBlocks())
	s.queued = newSparseSet(s.f.NumBlocks())
	s.hasPhi = newSparseSet(s.f.NumBlocks())
	s.hasDef = newSparseSet(s.f.NumBlocks())
	s.placeholder = s.s.entryNewValue0(ssa.OpUnknown, types.TypeInvalid)

	// Generate phi ops for each variable.
	for n := range vartypes {
		s.insertVarPhis(n, vars[n], defs[n], vartypes[n])
	}

	// Resolve FwdRefs to the correct write or phi.
	s.resolveFwdRefs()

	// Erase variable numbers stored in AuxInt fields of phi ops. They are no longer needed.
	for _, b := range s.f.Blocks {
		for _, v := range b.Values {
			if v.Op == ssa.OpPhi {
				v.AuxInt = 0
			}
			// Any remaining FwdRefs are dead code.
			if v.Op == ssa.OpFwdRef {
				v.Op = ssa.OpUnknown
				v.Aux = nil
			}
		}
	}
}

func (s *phiState) insertVarPhis(n int, var_ ir.Node, defs []*ssa.Block, typ *types.Type) {
	priq := &s.priq
	q := s.q
	queued := s.queued
	queued.clear()
	hasPhi := s.hasPhi
	hasPhi.clear()
	hasDef := s.hasDef
	hasDef.clear()

	// Add defining blocks to priority queue.
	for _, b := range defs {
		priq.a = append(priq.a, b)
		hasDef.add(b.ID)
		if debugPhi {
			fmt.Printf("def of var%d in %s\n", n, b)
		}
	}
	heap.Init(priq)

	// Visit blocks defining variable n, from deepest to shallowest.
	for len(priq.a) > 0 {
		currentRoot := heap.Pop(priq).(*ssa.Block)
		if debugPhi {
			fmt.Printf("currentRoot %s\n", currentRoot)
		}
		// Walk subtree below definition.
		// Skip subtrees we've done in previous iterations.
		// Find edges exiting tree dominated by definition (the dominance frontier).
		// Insert phis at target blocks.
		if queued.contains(currentRoot.ID) {
			s.s.Fatalf("root already in queue")
		}
		q = append(q, currentRoot)
		queued.add(currentRoot.ID)
		for len(q) > 0 {
			b := q[len(q)-1]
			q = q[:len(q)-1]
			if debugPhi {
				fmt.Printf("  processing %s\n", b)
			}

			currentRootLevel := s.level[currentRoot.ID]
			for _, e := range b.Succs {
				c := e.Block()
				// TODO: if the variable is dead at c, skip it.
				if s.level[c.ID] > currentRootLevel {
					// a D-edge, or an edge whose target is in currentRoot's subtree.
					continue
				}
				if hasPhi.contains(c.ID) {
					continue
				}
				// Add a phi to block c for variable n.
				hasPhi.add(c.ID)
				v := c.NewValue0I(s.s.blockStarts[b.ID], ssa.OpPhi, typ, int64(n))
				// Note: we store the variable number in the phi's AuxInt field. Used temporarily by phi building.
				if var_.Op() == ir.ONAME {
					s.s.addNamedValue(var_.(*ir.Name), v)
				}
				for range c.Preds {
					v.AddArg(s.placeholder) // Actual args will be filled in by resolveFwdRefs.
				}
				if debugPhi {
					fmt.Printf("new phi for var%d in %s: %s\n", n, c, v)
				}
				if !hasDef.contains(c.ID) {
					// There's now a new definition of this variable in block c.
					// Add it to the priority queue to explore.
					heap.Push(priq, c)
					hasDef.add(c.ID)
				}
			}

			// Visit children if they have not been visited yet.
			for c := s.tree[b.ID].firstChild; c != nil; c = s.tree[c.ID].sibling {
				if !queued.contains(c.ID) {
					q = append(q, c)
					queued.add(c.ID)
				}
			}
		}
	}
}

// resolveFwdRefs links all FwdRef uses up to their nearest dominating definition.
func (s *phiState) resolveFwdRefs() {
	// Do a depth-first walk of the dominator tree, keeping track
	// of the most-recently-seen value for each variable.

	// Map from variable ID to SSA value at the current point of the walk.
	values := make([]*ssa.Value, len(s.varnum))
	for i := range values {
		values[i] = s.placeholder
	}

	// Stack of work to do.
	type stackEntry struct {
		b *ssa.Block // block to explore

		// variable/value pair to reinstate on exit
		n int32 // variable ID
		v *ssa.Value

		// Note: only one of b or n,v will be set.
	}
	var stk []stackEntry

	stk = append(stk, stackEntry{b: s.f.Entry})
	for len(stk) > 0 {
		work := stk[len(stk)-1]
		stk = stk[:len(stk)-1]

		b := work.b
		if b == nil {
			// On exit from a block, this case will undo any assignments done below.
			values[work.n] = work.v
			continue
		}

		// Process phis as new defs. They come before FwdRefs in this block.
		for _, v := range b.Values {
			if v.Op != ssa.OpPhi {
				continue
			}
			n := int32(v.AuxInt)
			// Remember the old assignment so we can undo it when we exit b.
			stk = append(stk, stackEntry{n: n, v: values[n]})
			// Record the new assignment.
			values[n] = v
		}

		// Replace a FwdRef op with the current incoming value for its variable.
		for _, v := range b.Values {
			if v.Op != ssa.OpFwdRef {
				continue
			}
			n := s.varnum[v.Aux.(fwdRefAux).N]
			v.Op = ssa.OpCopy
			v.Aux = nil
			v.AddArg(values[n])
		}

		// Establish values for variables defined in b.
		for var_, v := range s.defvars[b.ID] {
			n, ok := s.varnum[var_]
			if !ok {
				// some variable not live across a basic block boundary.
				continue
			}
			// Remember the old assignment so we can undo it when we exit b.
			stk = append(stk, stackEntry{n: n, v: values[n]})
			// Record the new assignment.
			values[n] = v
		}

		// Replace phi args in successors with the current incoming value.
		for _, e := range b.Succs {
			c, i := e.Block(), e.Index()
			for j := len(c.Values) - 1; j >= 0; j-- {
				v := c.Values[j]
				if v.Op != ssa.OpPhi {
					break // All phis will be at the end of the block during phi building.
				}
				// Only set arguments that have been resolved.
				// For very wide CFGs, this significantly speeds up phi resolution.
				// See golang.org/issue/8225.
				if w := values[v.AuxInt]; w.Op != ssa.OpUnknown {
					v.SetArg(i, w)
				}
			}
		}

		// Walk children in dominator tree.
		for c := s.tree[b.ID].firstChild; c != nil; c = s.tree[c.ID].sibling {
			stk = append(stk, stackEntry{b: c})
		}
	}
}

// domBlock contains extra per-block information to record the dominator tree.
type domBlock struct {
	firstChild *ssa.Block // first child of block in dominator tree
	sibling    *ssa.Block // next child of parent in dominator tree
}

// A block heap is used as a priority queue to implement the PiggyBank
// from Sreedhar and Gao.  That paper uses an array which is better
// asymptotically but worse in the common case when the PiggyBank
// holds a sparse set of blocks.
type blockHeap struct {
	a     []*ssa.Block // block IDs in heap
	level []int32      // depth in dominator tree (static, used for determining priority)
}

func (h *blockHeap) Len() int      { return len(h.a) }
func (h *blockHeap) Swap(i, j int) { a := h.a; a[i], a[j] = a[j], a[i] }

func (h *blockHeap) Push(x any) {
	v := x.(*ssa.Block)
	h.a = append(h.a, v)
}
func (h *blockHeap) Pop() any {
	old := h.a
	n := len(old)
	x := old[n-1]
	h.a = old[:n-1]
	return x
}
func (h *blockHeap) Less(i, j int) bool {
	return h.level[h.a[i].ID] > h.level[h.a[j].ID]
}

// TODO: stop walking the iterated domininance frontier when
// the variable is dead. Maybe detect that by checking if the
// node we're on is reverse dominated by all the reads?
// Reverse dominated by the highest common successor of all the reads?

// copy of ../ssa/sparseset.go
// TODO: move this file to ../ssa, then use sparseSet there.
type sparseSet struct {
	dense  []ssa.ID
	sparse []int32
}

// newSparseSet returns a sparseSet that can represent
// integers between 0 and n-1.
func newSparseSet(n int) *sparseSet {
	return &sparseSet{dense: nil, sparse: make([]int32, n)}
}

func (s *sparseSet) contains(x ssa.ID) bool {
	i := s.sparse[x]
	return i < int32(len(s.dense)) && s.dense[i] == x
}

func (s *sparseSet) add(x ssa.ID) {
	i := s.sparse[x]
	if i < int32(len(s.dense)) && s.dense[i] == x {
		return
	}
	s.dense = append(s.dense, x)
	s.sparse[x] = int32(len(s.dense)) - 1
}

func (s *sparseSet) clear() {
	s.dense = s.dense[:0]
}

// Variant to use for small functions.
type simplePhiState struct {
	s         *state                   // SSA state
	f         *ssa.Func                // function to work on
	fwdrefs   []*ssa.Value             // list of FwdRefs to be processed
	defvars   []map[ir.Node]*ssa.Value // defined variables at end of each block
	reachable []bool                   // which blocks are reachable
}

func (s *simplePhiState) insertPhis() {
	s.reachable = ssa.ReachableBlocks(s.f)

	// Find FwdRef ops.
	for _, b := range s.f.Blocks {
		for _, v := range b.Values {
			if v.Op != ssa.OpFwdRef {
				continue
			}
			s.fwdrefs = append(s.fwdrefs, v)
			var_ := v.Aux.(fwdRefAux).N
			if _, ok := s.defvars[b.ID][var_]; !ok {
				s.defvars[b.ID][var_] = v // treat FwdDefs as definitions.
			}
		}
	}

	var args []*ssa.Value

loop:
	for len(s.fwdrefs) > 0 {
		v := s.fwdrefs[len(s.fwdrefs)-1]
		s.fwdrefs = s.fwdrefs[:len(s.fwdrefs)-1]
		b := v.Block
		var_ := v.Aux.(fwdRefAux).N
		if b == s.f.Entry {
			// No variable should be live at entry.
			s.s.Fatalf("value %v (%v) incorrectly live at entry", var_, v)
		}
		if !s.reachable[b.ID] {
			// This block is dead.
			// It doesn't matter what we use here as long as it is well-formed.
			v.Op = ssa.OpUnknown
			v.Aux = nil
			continue
		}
		// Find variable value on each predecessor.
		args = args[:0]
		for _, e := range b.Preds {
			args = append(args, s.lookupVarOutgoing(e.Block(), v.Type, var_, v.Pos))
		}

		// Decide if we need a phi or not. We need a phi if there
		// are two different args (which are both not v).
		var w *ssa.Value
		for _, a := range args {
			if a == v {
				continue // self-reference
			}
			if a == w {
				continue // already have this witness
			}
			if w != nil {
				// two witnesses, need a phi value
				v.Op = ssa.OpPhi
				v.AddArgs(args...)
				v.Aux = nil
				v.Pos = s.s.blockStarts[b.ID]
				continue loop
			}
			w = a // save witness
		}
		if w == nil {
			s.s.Fatalf("no witness for reachable phi %s", v)
		}
		// One witness. Make v a copy of w.
		v.Op = ssa.OpCopy
		v.Aux = nil
		v.AddArg(w)
	}
}

// lookupVarOutgoing finds the variable's value at the end of block b.
func (s *simplePhiState) lookupVarOutgoing(b *ssa.Block, t *types.Type, var_ ir.Node, line src.XPos) *ssa.Value {
	for {
		if v := s.defvars[b.ID][var_]; v != nil {
			return v
		}
		// The variable is not defined by b and we haven't looked it up yet.
		// If b has exactly one predecessor, loop to look it up there.
		// Otherwise, give up and insert a new FwdRef and resolve it later.
		if len(b.Preds) != 1 {
			break
		}
		b = b.Preds[0].Block()
		if !s.reachable[b.ID] {
			// This is rare; it happens with oddly interleaved infinite loops in dead code.
			// See issue 19783.
			break
		}
	}
	// Generate a FwdRef for the variable and return that.
	v := b.NewValue0A(line, ssa.OpFwdRef, t, fwdRefAux{N: var_})
	s.defvars[b.ID][var_] = v
	if var_.Op() == ir.ONAME {
		s.s.addNamedValue(var_.(*ir.Name), v)
	}
	s.fwdrefs = append(s.fwdrefs, v)
	return v
}

```

// === FILE: references/go/src/cmd/compile/internal/ssagen/simdAMD64intrinsics.go ===
```go
// Code generated by 'simdgen -o godefs -goroot $GOROOT -arch amd64 -xedPath $XED_PATH go_amd64.yaml types.yaml categories.yaml'; DO NOT EDIT.
package ssagen

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/types"
	"cmd/internal/sys"
)

func simdAMD64Intrinsics(addF func(pkg, fn string, b intrinsicBuilder, archFamilies ...sys.ArchFamily)) {

	addF(simdPackage, "Uint8x16.AESDecryptLastRound", opLen2(ssa.OpAESDecryptLastRoundUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.AESDecryptLastRound", opLen2(ssa.OpAESDecryptLastRoundUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.AESDecryptLastRound", opLen2(ssa.OpAESDecryptLastRoundUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.AESDecryptOneRound", opLen2(ssa.OpAESDecryptOneRoundUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.AESDecryptOneRound", opLen2(ssa.OpAESDecryptOneRoundUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.AESDecryptOneRound", opLen2(ssa.OpAESDecryptOneRoundUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.AESEncryptLastRound", opLen2(ssa.OpAESEncryptLastRoundUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.AESEncryptLastRound", opLen2(ssa.OpAESEncryptLastRoundUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.AESEncryptLastRound", opLen2(ssa.OpAESEncryptLastRoundUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.AESEncryptOneRound", opLen2(ssa.OpAESEncryptOneRoundUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.AESEncryptOneRound", opLen2(ssa.OpAESEncryptOneRoundUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.AESEncryptOneRound", opLen2(ssa.OpAESEncryptOneRoundUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.AESInvMixColumns", opLen1(ssa.OpAESInvMixColumnsUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.AESRoundKeyGenAssist", opLen1Imm8(ssa.OpAESRoundKeyGenAssistUint32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int8x16.Abs", opLen1(ssa.OpAbsInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Abs", opLen1(ssa.OpAbsInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Abs", opLen1(ssa.OpAbsInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Abs", opLen1(ssa.OpAbsInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Abs", opLen1(ssa.OpAbsInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Abs", opLen1(ssa.OpAbsInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Abs", opLen1(ssa.OpAbsInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Abs", opLen1(ssa.OpAbsInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Abs", opLen1(ssa.OpAbsInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Abs", opLen1(ssa.OpAbsInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Abs", opLen1(ssa.OpAbsInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Abs", opLen1(ssa.OpAbsInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Add", opLen2(ssa.OpAddFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Add", opLen2(ssa.OpAddFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Add", opLen2(ssa.OpAddFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Add", opLen2(ssa.OpAddFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Add", opLen2(ssa.OpAddFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Add", opLen2(ssa.OpAddFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.Add", opLen2(ssa.OpAddInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Add", opLen2(ssa.OpAddInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Add", opLen2(ssa.OpAddInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Add", opLen2(ssa.OpAddInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Add", opLen2(ssa.OpAddInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Add", opLen2(ssa.OpAddInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Add", opLen2(ssa.OpAddInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Add", opLen2(ssa.OpAddInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Add", opLen2(ssa.OpAddInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Add", opLen2(ssa.OpAddInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Add", opLen2(ssa.OpAddInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Add", opLen2(ssa.OpAddInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.Add", opLen2(ssa.OpAddUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.Add", opLen2(ssa.OpAddUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.Add", opLen2(ssa.OpAddUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.Add", opLen2(ssa.OpAddUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.Add", opLen2(ssa.OpAddUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.Add", opLen2(ssa.OpAddUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.Add", opLen2(ssa.OpAddUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.Add", opLen2(ssa.OpAddUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.Add", opLen2(ssa.OpAddUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.Add", opLen2(ssa.OpAddUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.Add", opLen2(ssa.OpAddUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.Add", opLen2(ssa.OpAddUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.AddOddSubEven", opLen2(ssa.OpAddOddSubEvenFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.AddOddSubEven", opLen2(ssa.OpAddOddSubEvenFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x2.AddOddSubEven", opLen2(ssa.OpAddOddSubEvenFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.AddOddSubEven", opLen2(ssa.OpAddOddSubEvenFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x16.AddSaturated", opLen2(ssa.OpAddSaturatedInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.AddSaturated", opLen2(ssa.OpAddSaturatedInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.AddSaturated", opLen2(ssa.OpAddSaturatedInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.AddSaturated", opLen2(ssa.OpAddSaturatedInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.AddSaturated", opLen2(ssa.OpAddSaturatedInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.AddSaturated", opLen2(ssa.OpAddSaturatedInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.AddSaturated", opLen2(ssa.OpAddSaturatedUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.AddSaturated", opLen2(ssa.OpAddSaturatedUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.AddSaturated", opLen2(ssa.OpAddSaturatedUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.AddSaturated", opLen2(ssa.OpAddSaturatedUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.AddSaturated", opLen2(ssa.OpAddSaturatedUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.AddSaturated", opLen2(ssa.OpAddSaturatedUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.And", opLen2(ssa.OpAndInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.And", opLen2(ssa.OpAndInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.And", opLen2(ssa.OpAndInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.And", opLen2(ssa.OpAndInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.And", opLen2(ssa.OpAndInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.And", opLen2(ssa.OpAndInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.And", opLen2(ssa.OpAndInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.And", opLen2(ssa.OpAndInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.And", opLen2(ssa.OpAndInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.And", opLen2(ssa.OpAndInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.And", opLen2(ssa.OpAndInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.And", opLen2(ssa.OpAndInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.And", opLen2(ssa.OpAndUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.And", opLen2(ssa.OpAndUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.And", opLen2(ssa.OpAndUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.And", opLen2(ssa.OpAndUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.And", opLen2(ssa.OpAndUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.And", opLen2(ssa.OpAndUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.And", opLen2(ssa.OpAndUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.And", opLen2(ssa.OpAndUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.And", opLen2(ssa.OpAndUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x4.And", opLen2(ssa.OpAndUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.And", opLen2(ssa.OpAndUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.And", opLen2(ssa.OpAndUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x16.AndNot", opLen2_21(ssa.OpAndNotInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.AndNot", opLen2_21(ssa.OpAndNotInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.AndNot", opLen2_21(ssa.OpAndNotInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.AndNot", opLen2_21(ssa.OpAndNotInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.AndNot", opLen2_21(ssa.OpAndNotInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.AndNot", opLen2_21(ssa.OpAndNotInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.AndNot", opLen2_21(ssa.OpAndNotInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.AndNot", opLen2_21(ssa.OpAndNotInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.AndNot", opLen2_21(ssa.OpAndNotInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.AndNot", opLen2_21(ssa.OpAndNotInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.AndNot", opLen2_21(ssa.OpAndNotInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.AndNot", opLen2_21(ssa.OpAndNotInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.AndNot", opLen2_21(ssa.OpAndNotUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.AndNot", opLen2_21(ssa.OpAndNotUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.AndNot", opLen2_21(ssa.OpAndNotUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.AndNot", opLen2_21(ssa.OpAndNotUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.AndNot", opLen2_21(ssa.OpAndNotUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.AndNot", opLen2_21(ssa.OpAndNotUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.AndNot", opLen2_21(ssa.OpAndNotUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.AndNot", opLen2_21(ssa.OpAndNotUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.AndNot", opLen2_21(ssa.OpAndNotUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x4.AndNot", opLen2_21(ssa.OpAndNotUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.AndNot", opLen2_21(ssa.OpAndNotUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.AndNot", opLen2_21(ssa.OpAndNotUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x16.Average", opLen2(ssa.OpAverageUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.Average", opLen2(ssa.OpAverageUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.Average", opLen2(ssa.OpAverageUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.Average", opLen2(ssa.OpAverageUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.Average", opLen2(ssa.OpAverageUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.Average", opLen2(ssa.OpAverageUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Ceil", opLen1(ssa.OpCeilFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Ceil", opLen1(ssa.OpCeilFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x2.Ceil", opLen1(ssa.OpCeilFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Ceil", opLen1(ssa.OpCeilFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x4.CeilScaled", opLen1Imm8(ssa.OpCeilScaledFloat32x4, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float32x8.CeilScaled", opLen1Imm8(ssa.OpCeilScaledFloat32x8, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float32x16.CeilScaled", opLen1Imm8(ssa.OpCeilScaledFloat32x16, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float64x2.CeilScaled", opLen1Imm8(ssa.OpCeilScaledFloat64x2, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float64x4.CeilScaled", opLen1Imm8(ssa.OpCeilScaledFloat64x4, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float64x8.CeilScaled", opLen1Imm8(ssa.OpCeilScaledFloat64x8, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float32x4.CeilScaledResidue", opLen1Imm8(ssa.OpCeilScaledResidueFloat32x4, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float32x8.CeilScaledResidue", opLen1Imm8(ssa.OpCeilScaledResidueFloat32x8, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float32x16.CeilScaledResidue", opLen1Imm8(ssa.OpCeilScaledResidueFloat32x16, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float64x2.CeilScaledResidue", opLen1Imm8(ssa.OpCeilScaledResidueFloat64x2, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float64x4.CeilScaledResidue", opLen1Imm8(ssa.OpCeilScaledResidueFloat64x4, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float64x8.CeilScaledResidue", opLen1Imm8(ssa.OpCeilScaledResidueFloat64x8, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float32x4.Compress", opLen2(ssa.OpCompressFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Compress", opLen2(ssa.OpCompressFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Compress", opLen2(ssa.OpCompressFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Compress", opLen2(ssa.OpCompressFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Compress", opLen2(ssa.OpCompressFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Compress", opLen2(ssa.OpCompressFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.Compress", opLen2(ssa.OpCompressInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Compress", opLen2(ssa.OpCompressInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Compress", opLen2(ssa.OpCompressInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Compress", opLen2(ssa.OpCompressInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Compress", opLen2(ssa.OpCompressInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Compress", opLen2(ssa.OpCompressInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Compress", opLen2(ssa.OpCompressInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Compress", opLen2(ssa.OpCompressInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Compress", opLen2(ssa.OpCompressInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Compress", opLen2(ssa.OpCompressInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Compress", opLen2(ssa.OpCompressInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Compress", opLen2(ssa.OpCompressInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.Compress", opLen2(ssa.OpCompressUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.Compress", opLen2(ssa.OpCompressUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.Compress", opLen2(ssa.OpCompressUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.Compress", opLen2(ssa.OpCompressUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.Compress", opLen2(ssa.OpCompressUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.Compress", opLen2(ssa.OpCompressUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.Compress", opLen2(ssa.OpCompressUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.Compress", opLen2(ssa.OpCompressUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.Compress", opLen2(ssa.OpCompressUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.Compress", opLen2(ssa.OpCompressUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.Compress", opLen2(ssa.OpCompressUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.Compress", opLen2(ssa.OpCompressUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x2.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x8.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x8.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.ConcatAddPairsGrouped", opLen2(ssa.OpConcatAddPairsGroupedFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x4.ConcatAddPairsGrouped", opLen2(ssa.OpConcatAddPairsGroupedFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x16.ConcatAddPairsGrouped", opLen2(ssa.OpConcatAddPairsGroupedInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x8.ConcatAddPairsGrouped", opLen2(ssa.OpConcatAddPairsGroupedInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x16.ConcatAddPairsGrouped", opLen2(ssa.OpConcatAddPairsGroupedUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x8.ConcatAddPairsGrouped", opLen2(ssa.OpConcatAddPairsGroupedUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x8.ConcatAddPairsSaturated", opLen2(ssa.OpConcatAddPairsSaturatedInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.ConcatAddPairsSaturatedGrouped", opLen2(ssa.OpConcatAddPairsSaturatedGroupedInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x16.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x16.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x32.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x64.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x8.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x16.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x32.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.ConcatPermute", opLen3_231(ssa.OpConcatPermuteFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.ConcatPermute", opLen3_231(ssa.OpConcatPermuteFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x8.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x8.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.ConcatPermute", opLen3_231(ssa.OpConcatPermuteFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x16.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x16.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.ConcatPermute", opLen3_231(ssa.OpConcatPermuteFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x2.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x2.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.ConcatPermute", opLen3_231(ssa.OpConcatPermuteFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x4.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x4.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.ConcatPermute", opLen3_231(ssa.OpConcatPermuteFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x8.ConcatPermute", opLen3_231(ssa.OpConcatPermuteInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x8.ConcatPermute", opLen3_231(ssa.OpConcatPermuteUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x8.ConcatPermute128Scalars", opLen2Imm8_II(ssa.OpConcatPermute128ScalarsFloat32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Float64x4.ConcatPermute128Scalars", opLen2Imm8_II(ssa.OpConcatPermute128ScalarsFloat64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int8x32.ConcatPermute128Scalars", opLen2Imm8_II(ssa.OpConcatPermute128ScalarsInt8x32, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int16x16.ConcatPermute128Scalars", opLen2Imm8_II(ssa.OpConcatPermute128ScalarsInt16x16, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int32x8.ConcatPermute128Scalars", opLen2Imm8_II(ssa.OpConcatPermute128ScalarsInt32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int64x4.ConcatPermute128Scalars", opLen2Imm8_II(ssa.OpConcatPermute128ScalarsInt64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint8x32.ConcatPermute128Scalars", opLen2Imm8_II(ssa.OpConcatPermute128ScalarsUint8x32, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint16x16.ConcatPermute128Scalars", opLen2Imm8_II(ssa.OpConcatPermute128ScalarsUint16x16, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint32x8.ConcatPermute128Scalars", opLen2Imm8_II(ssa.OpConcatPermute128ScalarsUint32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint64x4.ConcatPermute128Scalars", opLen2Imm8_II(ssa.OpConcatPermute128ScalarsUint64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint8x16.ConcatShiftBytesRight", opLen2Imm8_2I(ssa.OpConcatShiftBytesRightUint8x16, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint8x32.ConcatShiftBytesRightGrouped", opLen2Imm8_2I(ssa.OpConcatShiftBytesRightGroupedUint8x32, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint8x64.ConcatShiftBytesRightGrouped", opLen2Imm8_2I(ssa.OpConcatShiftBytesRightGroupedUint8x64, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Float32x4.ConcatSubPairs", opLen2(ssa.OpConcatSubPairsFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x2.ConcatSubPairs", opLen2(ssa.OpConcatSubPairsFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x8.ConcatSubPairs", opLen2(ssa.OpConcatSubPairsInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.ConcatSubPairs", opLen2(ssa.OpConcatSubPairsInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x8.ConcatSubPairs", opLen2(ssa.OpConcatSubPairsUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.ConcatSubPairs", opLen2(ssa.OpConcatSubPairsUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.ConcatSubPairsGrouped", opLen2(ssa.OpConcatSubPairsGroupedFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x4.ConcatSubPairsGrouped", opLen2(ssa.OpConcatSubPairsGroupedFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x16.ConcatSubPairsGrouped", opLen2(ssa.OpConcatSubPairsGroupedInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x8.ConcatSubPairsGrouped", opLen2(ssa.OpConcatSubPairsGroupedInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x16.ConcatSubPairsGrouped", opLen2(ssa.OpConcatSubPairsGroupedUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x8.ConcatSubPairsGrouped", opLen2(ssa.OpConcatSubPairsGroupedUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x8.ConcatSubPairsSaturated", opLen2(ssa.OpConcatSubPairsSaturatedInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.ConcatSubPairsSaturatedGrouped", opLen2(ssa.OpConcatSubPairsSaturatedGroupedInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x2.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Float64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Float64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x8.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Float64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x4.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Int32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Int32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Int32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Int64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Int64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x8.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Int64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x4.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Uint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Uint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Uint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Uint64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x8.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Uint64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x4.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Float32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x8.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Float32x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Int32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x8.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Int32x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Int64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Int64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Int64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Uint32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x8.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Uint32x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Uint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Uint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Uint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.ConvertToInt32", opLen1(ssa.OpConvertToInt32Float32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.ConvertToInt32", opLen1(ssa.OpConvertToInt32Float32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.ConvertToInt32", opLen1(ssa.OpConvertToInt32Float32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.ConvertToInt32", opLen1(ssa.OpConvertToInt32Float64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.ConvertToInt32", opLen1(ssa.OpConvertToInt32Float64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x8.ConvertToInt32", opLen1(ssa.OpConvertToInt32Float64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x4.ConvertToInt64", opLen1(ssa.OpConvertToInt64Float32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x8.ConvertToInt64", opLen1(ssa.OpConvertToInt64Float32x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.ConvertToInt64", opLen1(ssa.OpConvertToInt64Float64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.ConvertToInt64", opLen1(ssa.OpConvertToInt64Float64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.ConvertToInt64", opLen1(ssa.OpConvertToInt64Float64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.ConvertToUint32", opLen1(ssa.OpConvertToUint32Float32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.ConvertToUint32", opLen1(ssa.OpConvertToUint32Float32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.ConvertToUint32", opLen1(ssa.OpConvertToUint32Float32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.ConvertToUint32", opLen1(ssa.OpConvertToUint32Float64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.ConvertToUint32", opLen1(ssa.OpConvertToUint32Float64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x8.ConvertToUint32", opLen1(ssa.OpConvertToUint32Float64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x4.ConvertToUint64", opLen1(ssa.OpConvertToUint64Float32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x8.ConvertToUint64", opLen1(ssa.OpConvertToUint64Float32x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.ConvertToUint64", opLen1(ssa.OpConvertToUint64Float64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.ConvertToUint64", opLen1(ssa.OpConvertToUint64Float64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.ConvertToUint64", opLen1(ssa.OpConvertToUint64Float64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Div", opLen2(ssa.OpDivFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Div", opLen2(ssa.OpDivFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Div", opLen2(ssa.OpDivFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Div", opLen2(ssa.OpDivFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Div", opLen2(ssa.OpDivFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Div", opLen2(ssa.OpDivFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.DotProductPairs", opLen2(ssa.OpDotProductPairsInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.DotProductPairs", opLen2(ssa.OpDotProductPairsInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.DotProductPairs", opLen2(ssa.OpDotProductPairsInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.DotProductPairsSaturated", opLen2(ssa.OpDotProductPairsSaturatedUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.DotProductPairsSaturated", opLen2(ssa.OpDotProductPairsSaturatedUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.DotProductPairsSaturated", opLen2(ssa.OpDotProductPairsSaturatedUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.Equal", opLen2(ssa.OpEqualInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Equal", opLen2(ssa.OpEqualInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Equal", opLen2(ssa.OpEqualInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Equal", opLen2(ssa.OpEqualInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Equal", opLen2(ssa.OpEqualInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Equal", opLen2(ssa.OpEqualInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Equal", opLen2(ssa.OpEqualInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Equal", opLen2(ssa.OpEqualInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Equal", opLen2(ssa.OpEqualInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Equal", opLen2(ssa.OpEqualInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Equal", opLen2(ssa.OpEqualInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Equal", opLen2(ssa.OpEqualInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.Equal", opLen2(ssa.OpEqualUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.Equal", opLen2(ssa.OpEqualUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.Equal", opLen2(ssa.OpEqualUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.Equal", opLen2(ssa.OpEqualUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.Equal", opLen2(ssa.OpEqualUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.Equal", opLen2(ssa.OpEqualUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.Equal", opLen2(ssa.OpEqualUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.Equal", opLen2(ssa.OpEqualUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.Equal", opLen2(ssa.OpEqualUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.Equal", opLen2(ssa.OpEqualUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.Equal", opLen2(ssa.OpEqualUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.Equal", opLen2(ssa.OpEqualUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Equal", opLen2(ssa.OpEqualFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Equal", opLen2(ssa.OpEqualFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Equal", opLen2(ssa.OpEqualFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Equal", opLen2(ssa.OpEqualFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Equal", opLen2(ssa.OpEqualFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Equal", opLen2(ssa.OpEqualFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Expand", opLen2(ssa.OpExpandFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Expand", opLen2(ssa.OpExpandFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Expand", opLen2(ssa.OpExpandFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Expand", opLen2(ssa.OpExpandFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Expand", opLen2(ssa.OpExpandFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Expand", opLen2(ssa.OpExpandFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.Expand", opLen2(ssa.OpExpandInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Expand", opLen2(ssa.OpExpandInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Expand", opLen2(ssa.OpExpandInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Expand", opLen2(ssa.OpExpandInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Expand", opLen2(ssa.OpExpandInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Expand", opLen2(ssa.OpExpandInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Expand", opLen2(ssa.OpExpandInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Expand", opLen2(ssa.OpExpandInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Expand", opLen2(ssa.OpExpandInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Expand", opLen2(ssa.OpExpandInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Expand", opLen2(ssa.OpExpandInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Expand", opLen2(ssa.OpExpandInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.Expand", opLen2(ssa.OpExpandUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.Expand", opLen2(ssa.OpExpandUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.Expand", opLen2(ssa.OpExpandUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.Expand", opLen2(ssa.OpExpandUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.Expand", opLen2(ssa.OpExpandUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.Expand", opLen2(ssa.OpExpandUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.Expand", opLen2(ssa.OpExpandUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.Expand", opLen2(ssa.OpExpandUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.Expand", opLen2(ssa.OpExpandUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.Expand", opLen2(ssa.OpExpandUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.Expand", opLen2(ssa.OpExpandUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.Expand", opLen2(ssa.OpExpandUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.ExtendLo2ToInt64", opLen1(ssa.OpExtendLo2ToInt64Int8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x8.ExtendLo2ToInt64", opLen1(ssa.OpExtendLo2ToInt64Int16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.ExtendLo2ToInt64", opLen1(ssa.OpExtendLo2ToInt64Int32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x16.ExtendLo2ToUint64", opLen1(ssa.OpExtendLo2ToUint64Uint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x8.ExtendLo2ToUint64", opLen1(ssa.OpExtendLo2ToUint64Uint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.ExtendLo2ToUint64", opLen1(ssa.OpExtendLo2ToUint64Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x16.ExtendLo4ToInt32", opLen1(ssa.OpExtendLo4ToInt32Int8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x8.ExtendLo4ToInt32", opLen1(ssa.OpExtendLo4ToInt32Int16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x16.ExtendLo4ToInt64", opLen1(ssa.OpExtendLo4ToInt64Int8x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x8.ExtendLo4ToInt64", opLen1(ssa.OpExtendLo4ToInt64Int16x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x16.ExtendLo4ToUint32", opLen1(ssa.OpExtendLo4ToUint32Uint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x8.ExtendLo4ToUint32", opLen1(ssa.OpExtendLo4ToUint32Uint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x16.ExtendLo4ToUint64", opLen1(ssa.OpExtendLo4ToUint64Uint8x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x8.ExtendLo4ToUint64", opLen1(ssa.OpExtendLo4ToUint64Uint16x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x16.ExtendLo8ToInt16", opLen1(ssa.OpExtendLo8ToInt16Int8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x16.ExtendLo8ToInt32", opLen1(ssa.OpExtendLo8ToInt32Int8x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x16.ExtendLo8ToInt64", opLen1(ssa.OpExtendLo8ToInt64Int8x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.ExtendLo8ToUint16", opLen1(ssa.OpExtendLo8ToUint16Uint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x16.ExtendLo8ToUint32", opLen1(ssa.OpExtendLo8ToUint32Uint8x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x16.ExtendLo8ToUint64", opLen1(ssa.OpExtendLo8ToUint64Uint8x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.ExtendToInt16", opLen1(ssa.OpExtendToInt16Int8x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x32.ExtendToInt16", opLen1(ssa.OpExtendToInt16Int8x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.ExtendToInt32", opLen1(ssa.OpExtendToInt32Int8x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.ExtendToInt32", opLen1(ssa.OpExtendToInt32Int16x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x16.ExtendToInt32", opLen1(ssa.OpExtendToInt32Int16x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.ExtendToInt64", opLen1(ssa.OpExtendToInt64Int16x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.ExtendToInt64", opLen1(ssa.OpExtendToInt64Int32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x8.ExtendToInt64", opLen1(ssa.OpExtendToInt64Int32x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.ExtendToUint16", opLen1(ssa.OpExtendToUint16Uint8x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x32.ExtendToUint16", opLen1(ssa.OpExtendToUint16Uint8x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.ExtendToUint32", opLen1(ssa.OpExtendToUint32Uint8x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.ExtendToUint32", opLen1(ssa.OpExtendToUint32Uint16x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x16.ExtendToUint32", opLen1(ssa.OpExtendToUint32Uint16x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.ExtendToUint64", opLen1(ssa.OpExtendToUint64Uint16x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.ExtendToUint64", opLen1(ssa.OpExtendToUint64Uint32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x8.ExtendToUint64", opLen1(ssa.OpExtendToUint64Uint32x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Floor", opLen1(ssa.OpFloorFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Floor", opLen1(ssa.OpFloorFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x2.Floor", opLen1(ssa.OpFloorFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Floor", opLen1(ssa.OpFloorFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x4.FloorScaled", opLen1Imm8(ssa.OpFloorScaledFloat32x4, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float32x8.FloorScaled", opLen1Imm8(ssa.OpFloorScaledFloat32x8, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float32x16.FloorScaled", opLen1Imm8(ssa.OpFloorScaledFloat32x16, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float64x2.FloorScaled", opLen1Imm8(ssa.OpFloorScaledFloat64x2, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float64x4.FloorScaled", opLen1Imm8(ssa.OpFloorScaledFloat64x4, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float64x8.FloorScaled", opLen1Imm8(ssa.OpFloorScaledFloat64x8, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float32x4.FloorScaledResidue", opLen1Imm8(ssa.OpFloorScaledResidueFloat32x4, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float32x8.FloorScaledResidue", opLen1Imm8(ssa.OpFloorScaledResidueFloat32x8, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float32x16.FloorScaledResidue", opLen1Imm8(ssa.OpFloorScaledResidueFloat32x16, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float64x2.FloorScaledResidue", opLen1Imm8(ssa.OpFloorScaledResidueFloat64x2, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float64x4.FloorScaledResidue", opLen1Imm8(ssa.OpFloorScaledResidueFloat64x4, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float64x8.FloorScaledResidue", opLen1Imm8(ssa.OpFloorScaledResidueFloat64x8, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Uint8x16.GaloisFieldAffineTransform", opLen2Imm8_2I(ssa.OpGaloisFieldAffineTransformUint8x16, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint8x32.GaloisFieldAffineTransform", opLen2Imm8_2I(ssa.OpGaloisFieldAffineTransformUint8x32, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint8x64.GaloisFieldAffineTransform", opLen2Imm8_2I(ssa.OpGaloisFieldAffineTransformUint8x64, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint8x16.GaloisFieldAffineTransformInverse", opLen2Imm8_2I(ssa.OpGaloisFieldAffineTransformInverseUint8x16, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint8x32.GaloisFieldAffineTransformInverse", opLen2Imm8_2I(ssa.OpGaloisFieldAffineTransformInverseUint8x32, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint8x64.GaloisFieldAffineTransformInverse", opLen2Imm8_2I(ssa.OpGaloisFieldAffineTransformInverseUint8x64, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint8x16.GaloisFieldMul", opLen2(ssa.OpGaloisFieldMulUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.GaloisFieldMul", opLen2(ssa.OpGaloisFieldMulUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.GaloisFieldMul", opLen2(ssa.OpGaloisFieldMulUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.GetElem", opLen1Imm(ssa.OpGetElemFloat32x4, types.Types[types.TFLOAT32], 0, 3), sys.AMD64)
	addF(simdPackage, "Float64x2.GetElem", opLen1Imm(ssa.OpGetElemFloat64x2, types.Types[types.TFLOAT64], 0, 1), sys.AMD64)
	addF(simdPackage, "Int8x16.GetElem", opLen1Imm(ssa.OpGetElemInt8x16, types.Types[types.TINT8], 0, 15), sys.AMD64)
	addF(simdPackage, "Int16x8.GetElem", opLen1Imm(ssa.OpGetElemInt16x8, types.Types[types.TINT16], 0, 7), sys.AMD64)
	addF(simdPackage, "Int32x4.GetElem", opLen1Imm(ssa.OpGetElemInt32x4, types.Types[types.TINT32], 0, 3), sys.AMD64)
	addF(simdPackage, "Int64x2.GetElem", opLen1Imm(ssa.OpGetElemInt64x2, types.Types[types.TINT64], 0, 1), sys.AMD64)
	addF(simdPackage, "Uint8x16.GetElem", opLen1Imm(ssa.OpGetElemUint8x16, types.Types[types.TUINT8], 0, 15), sys.AMD64)
	addF(simdPackage, "Uint16x8.GetElem", opLen1Imm(ssa.OpGetElemUint16x8, types.Types[types.TUINT16], 0, 7), sys.AMD64)
	addF(simdPackage, "Uint32x4.GetElem", opLen1Imm(ssa.OpGetElemUint32x4, types.Types[types.TUINT32], 0, 3), sys.AMD64)
	addF(simdPackage, "Uint64x2.GetElem", opLen1Imm(ssa.OpGetElemUint64x2, types.Types[types.TUINT64], 0, 1), sys.AMD64)
	addF(simdPackage, "Float32x8.GetHi", opLen1(ssa.OpGetHiFloat32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x16.GetHi", opLen1(ssa.OpGetHiFloat32x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x4.GetHi", opLen1(ssa.OpGetHiFloat64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x8.GetHi", opLen1(ssa.OpGetHiFloat64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x32.GetHi", opLen1(ssa.OpGetHiInt8x32, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x64.GetHi", opLen1(ssa.OpGetHiInt8x64, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x16.GetHi", opLen1(ssa.OpGetHiInt16x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x32.GetHi", opLen1(ssa.OpGetHiInt16x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x8.GetHi", opLen1(ssa.OpGetHiInt32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x16.GetHi", opLen1(ssa.OpGetHiInt32x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x4.GetHi", opLen1(ssa.OpGetHiInt64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x8.GetHi", opLen1(ssa.OpGetHiInt64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x32.GetHi", opLen1(ssa.OpGetHiUint8x32, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x64.GetHi", opLen1(ssa.OpGetHiUint8x64, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x16.GetHi", opLen1(ssa.OpGetHiUint16x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x32.GetHi", opLen1(ssa.OpGetHiUint16x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x8.GetHi", opLen1(ssa.OpGetHiUint32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x16.GetHi", opLen1(ssa.OpGetHiUint32x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x4.GetHi", opLen1(ssa.OpGetHiUint64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x8.GetHi", opLen1(ssa.OpGetHiUint64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x8.GetLo", opLen1(ssa.OpGetLoFloat32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x16.GetLo", opLen1(ssa.OpGetLoFloat32x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x4.GetLo", opLen1(ssa.OpGetLoFloat64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x8.GetLo", opLen1(ssa.OpGetLoFloat64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x32.GetLo", opLen1(ssa.OpGetLoInt8x32, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x64.GetLo", opLen1(ssa.OpGetLoInt8x64, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x16.GetLo", opLen1(ssa.OpGetLoInt16x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x32.GetLo", opLen1(ssa.OpGetLoInt16x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x8.GetLo", opLen1(ssa.OpGetLoInt32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x16.GetLo", opLen1(ssa.OpGetLoInt32x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x4.GetLo", opLen1(ssa.OpGetLoInt64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x8.GetLo", opLen1(ssa.OpGetLoInt64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x32.GetLo", opLen1(ssa.OpGetLoUint8x32, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x64.GetLo", opLen1(ssa.OpGetLoUint8x64, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x16.GetLo", opLen1(ssa.OpGetLoUint16x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x32.GetLo", opLen1(ssa.OpGetLoUint16x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x8.GetLo", opLen1(ssa.OpGetLoUint32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x16.GetLo", opLen1(ssa.OpGetLoUint32x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x4.GetLo", opLen1(ssa.OpGetLoUint64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x8.GetLo", opLen1(ssa.OpGetLoUint64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x16.Greater", opLen2(ssa.OpGreaterInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Greater", opLen2(ssa.OpGreaterInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Greater", opLen2(ssa.OpGreaterInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Greater", opLen2(ssa.OpGreaterInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Greater", opLen2(ssa.OpGreaterInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Greater", opLen2(ssa.OpGreaterInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Greater", opLen2(ssa.OpGreaterInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Greater", opLen2(ssa.OpGreaterInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Greater", opLen2(ssa.OpGreaterInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Greater", opLen2(ssa.OpGreaterInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Greater", opLen2(ssa.OpGreaterInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Greater", opLen2(ssa.OpGreaterInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Greater", opLen2(ssa.OpGreaterFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Greater", opLen2(ssa.OpGreaterFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Greater", opLen2(ssa.OpGreaterFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Greater", opLen2(ssa.OpGreaterFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Greater", opLen2(ssa.OpGreaterFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Greater", opLen2(ssa.OpGreaterFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x64.Greater", opLen2(ssa.OpGreaterUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x32.Greater", opLen2(ssa.OpGreaterUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x16.Greater", opLen2(ssa.OpGreaterUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x8.Greater", opLen2(ssa.OpGreaterUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.GreaterEqual", opLen2(ssa.OpGreaterEqualFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.GreaterEqual", opLen2(ssa.OpGreaterEqualFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.GreaterEqual", opLen2(ssa.OpGreaterEqualFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.GreaterEqual", opLen2(ssa.OpGreaterEqualFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.GreaterEqual", opLen2(ssa.OpGreaterEqualFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.GreaterEqual", opLen2(ssa.OpGreaterEqualFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x64.GreaterEqual", opLen2(ssa.OpGreaterEqualInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x32.GreaterEqual", opLen2(ssa.OpGreaterEqualInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x16.GreaterEqual", opLen2(ssa.OpGreaterEqualInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x8.GreaterEqual", opLen2(ssa.OpGreaterEqualInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x64.GreaterEqual", opLen2(ssa.OpGreaterEqualUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x32.GreaterEqual", opLen2(ssa.OpGreaterEqualUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x16.GreaterEqual", opLen2(ssa.OpGreaterEqualUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x8.GreaterEqual", opLen2(ssa.OpGreaterEqualUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.InterleaveHi", opLen2(ssa.OpInterleaveHiInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.InterleaveHi", opLen2(ssa.OpInterleaveHiInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x2.InterleaveHi", opLen2(ssa.OpInterleaveHiInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x8.InterleaveHi", opLen2(ssa.OpInterleaveHiUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.InterleaveHi", opLen2(ssa.OpInterleaveHiUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x2.InterleaveHi", opLen2(ssa.OpInterleaveHiUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x8.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x4.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x16.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x8.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x4.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.InterleaveHiGrouped", opLen2(ssa.OpInterleaveHiGroupedUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.InterleaveLo", opLen2(ssa.OpInterleaveLoInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.InterleaveLo", opLen2(ssa.OpInterleaveLoInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x2.InterleaveLo", opLen2(ssa.OpInterleaveLoInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x8.InterleaveLo", opLen2(ssa.OpInterleaveLoUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.InterleaveLo", opLen2(ssa.OpInterleaveLoUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x2.InterleaveLo", opLen2(ssa.OpInterleaveLoUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x8.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x4.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x16.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x8.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x4.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.InterleaveLoGrouped", opLen2(ssa.OpInterleaveLoGroupedUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.LeadingZeros", opLen1(ssa.OpLeadingZerosInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.LeadingZeros", opLen1(ssa.OpLeadingZerosInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.LeadingZeros", opLen1(ssa.OpLeadingZerosInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.LeadingZeros", opLen1(ssa.OpLeadingZerosInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.LeadingZeros", opLen1(ssa.OpLeadingZerosInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.LeadingZeros", opLen1(ssa.OpLeadingZerosInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.LeadingZeros", opLen1(ssa.OpLeadingZerosUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.LeadingZeros", opLen1(ssa.OpLeadingZerosUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.LeadingZeros", opLen1(ssa.OpLeadingZerosUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.LeadingZeros", opLen1(ssa.OpLeadingZerosUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.LeadingZeros", opLen1(ssa.OpLeadingZerosUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.LeadingZeros", opLen1(ssa.OpLeadingZerosUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Less", opLen2(ssa.OpLessFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Less", opLen2(ssa.OpLessFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Less", opLen2(ssa.OpLessFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Less", opLen2(ssa.OpLessFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Less", opLen2(ssa.OpLessFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Less", opLen2(ssa.OpLessFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x64.Less", opLen2(ssa.OpLessInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x32.Less", opLen2(ssa.OpLessInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x16.Less", opLen2(ssa.OpLessInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x8.Less", opLen2(ssa.OpLessInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x64.Less", opLen2(ssa.OpLessUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x32.Less", opLen2(ssa.OpLessUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x16.Less", opLen2(ssa.OpLessUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x8.Less", opLen2(ssa.OpLessUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.LessEqual", opLen2(ssa.OpLessEqualFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.LessEqual", opLen2(ssa.OpLessEqualFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.LessEqual", opLen2(ssa.OpLessEqualFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.LessEqual", opLen2(ssa.OpLessEqualFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.LessEqual", opLen2(ssa.OpLessEqualFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.LessEqual", opLen2(ssa.OpLessEqualFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x64.LessEqual", opLen2(ssa.OpLessEqualInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x32.LessEqual", opLen2(ssa.OpLessEqualInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x16.LessEqual", opLen2(ssa.OpLessEqualInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x8.LessEqual", opLen2(ssa.OpLessEqualInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x64.LessEqual", opLen2(ssa.OpLessEqualUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x32.LessEqual", opLen2(ssa.OpLessEqualUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x16.LessEqual", opLen2(ssa.OpLessEqualUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x8.LessEqual", opLen2(ssa.OpLessEqualUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Max", opLen2(ssa.OpMaxFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Max", opLen2(ssa.OpMaxFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Max", opLen2(ssa.OpMaxFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Max", opLen2(ssa.OpMaxFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Max", opLen2(ssa.OpMaxFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Max", opLen2(ssa.OpMaxFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.Max", opLen2(ssa.OpMaxInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Max", opLen2(ssa.OpMaxInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Max", opLen2(ssa.OpMaxInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Max", opLen2(ssa.OpMaxInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Max", opLen2(ssa.OpMaxInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Max", opLen2(ssa.OpMaxInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Max", opLen2(ssa.OpMaxInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Max", opLen2(ssa.OpMaxInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Max", opLen2(ssa.OpMaxInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Max", opLen2(ssa.OpMaxInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Max", opLen2(ssa.OpMaxInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Max", opLen2(ssa.OpMaxInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.Max", opLen2(ssa.OpMaxUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.Max", opLen2(ssa.OpMaxUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.Max", opLen2(ssa.OpMaxUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.Max", opLen2(ssa.OpMaxUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.Max", opLen2(ssa.OpMaxUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.Max", opLen2(ssa.OpMaxUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.Max", opLen2(ssa.OpMaxUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.Max", opLen2(ssa.OpMaxUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.Max", opLen2(ssa.OpMaxUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.Max", opLen2(ssa.OpMaxUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.Max", opLen2(ssa.OpMaxUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.Max", opLen2(ssa.OpMaxUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Min", opLen2(ssa.OpMinFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Min", opLen2(ssa.OpMinFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Min", opLen2(ssa.OpMinFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Min", opLen2(ssa.OpMinFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Min", opLen2(ssa.OpMinFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Min", opLen2(ssa.OpMinFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.Min", opLen2(ssa.OpMinInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Min", opLen2(ssa.OpMinInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Min", opLen2(ssa.OpMinInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Min", opLen2(ssa.OpMinInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Min", opLen2(ssa.OpMinInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Min", opLen2(ssa.OpMinInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Min", opLen2(ssa.OpMinInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Min", opLen2(ssa.OpMinInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Min", opLen2(ssa.OpMinInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Min", opLen2(ssa.OpMinInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Min", opLen2(ssa.OpMinInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Min", opLen2(ssa.OpMinInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.Min", opLen2(ssa.OpMinUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.Min", opLen2(ssa.OpMinUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.Min", opLen2(ssa.OpMinUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.Min", opLen2(ssa.OpMinUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.Min", opLen2(ssa.OpMinUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.Min", opLen2(ssa.OpMinUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.Min", opLen2(ssa.OpMinUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.Min", opLen2(ssa.OpMinUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.Min", opLen2(ssa.OpMinUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.Min", opLen2(ssa.OpMinUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.Min", opLen2(ssa.OpMinUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.Min", opLen2(ssa.OpMinUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Mul", opLen2(ssa.OpMulFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Mul", opLen2(ssa.OpMulFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Mul", opLen2(ssa.OpMulFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Mul", opLen2(ssa.OpMulFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Mul", opLen2(ssa.OpMulFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Mul", opLen2(ssa.OpMulFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Mul", opLen2(ssa.OpMulInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Mul", opLen2(ssa.OpMulInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Mul", opLen2(ssa.OpMulInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Mul", opLen2(ssa.OpMulInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Mul", opLen2(ssa.OpMulInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Mul", opLen2(ssa.OpMulInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Mul", opLen2(ssa.OpMulInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Mul", opLen2(ssa.OpMulInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Mul", opLen2(ssa.OpMulInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.Mul", opLen2(ssa.OpMulUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.Mul", opLen2(ssa.OpMulUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.Mul", opLen2(ssa.OpMulUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.Mul", opLen2(ssa.OpMulUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.Mul", opLen2(ssa.OpMulUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.Mul", opLen2(ssa.OpMulUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.Mul", opLen2(ssa.OpMulUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.Mul", opLen2(ssa.OpMulUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.Mul", opLen2(ssa.OpMulUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.MulAdd", opLen3(ssa.OpMulAddFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.MulAdd", opLen3(ssa.OpMulAddFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.MulAdd", opLen3(ssa.OpMulAddFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.MulAdd", opLen3(ssa.OpMulAddFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.MulAdd", opLen3(ssa.OpMulAddFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.MulAdd", opLen3(ssa.OpMulAddFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.MulAddEvenSubOdd", opLen3(ssa.OpMulAddEvenSubOddFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.MulAddEvenSubOdd", opLen3(ssa.OpMulAddEvenSubOddFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.MulAddEvenSubOdd", opLen3(ssa.OpMulAddEvenSubOddFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.MulAddEvenSubOdd", opLen3(ssa.OpMulAddEvenSubOddFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.MulAddEvenSubOdd", opLen3(ssa.OpMulAddEvenSubOddFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.MulAddEvenSubOdd", opLen3(ssa.OpMulAddEvenSubOddFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.MulAddOddSubEven", opLen3(ssa.OpMulAddOddSubEvenFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.MulAddOddSubEven", opLen3(ssa.OpMulAddOddSubEvenFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.MulAddOddSubEven", opLen3(ssa.OpMulAddOddSubEvenFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.MulAddOddSubEven", opLen3(ssa.OpMulAddOddSubEvenFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.MulAddOddSubEven", opLen3(ssa.OpMulAddOddSubEvenFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.MulAddOddSubEven", opLen3(ssa.OpMulAddOddSubEvenFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.MulHigh", opLen2(ssa.OpMulHighInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.MulHigh", opLen2(ssa.OpMulHighInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.MulHigh", opLen2(ssa.OpMulHighInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.MulHigh", opLen2(ssa.OpMulHighUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.MulHigh", opLen2(ssa.OpMulHighUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.MulHigh", opLen2(ssa.OpMulHighUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.MulSign", opLen2(ssa.OpMulSignInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.MulSign", opLen2(ssa.OpMulSignInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x8.MulSign", opLen2(ssa.OpMulSignInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.MulSign", opLen2(ssa.OpMulSignInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x4.MulSign", opLen2(ssa.OpMulSignInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.MulSign", opLen2(ssa.OpMulSignInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x4.MulWidenEven", opLen2(ssa.OpMulWidenEvenInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.MulWidenEven", opLen2(ssa.OpMulWidenEvenInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x4.MulWidenEven", opLen2(ssa.OpMulWidenEvenUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.MulWidenEven", opLen2(ssa.OpMulWidenEvenUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x4.NotEqual", opLen2(ssa.OpNotEqualFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.NotEqual", opLen2(ssa.OpNotEqualFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.NotEqual", opLen2(ssa.OpNotEqualFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.NotEqual", opLen2(ssa.OpNotEqualFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.NotEqual", opLen2(ssa.OpNotEqualFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.NotEqual", opLen2(ssa.OpNotEqualFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x64.NotEqual", opLen2(ssa.OpNotEqualInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x32.NotEqual", opLen2(ssa.OpNotEqualInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x16.NotEqual", opLen2(ssa.OpNotEqualInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x8.NotEqual", opLen2(ssa.OpNotEqualInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x64.NotEqual", opLen2(ssa.OpNotEqualUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x32.NotEqual", opLen2(ssa.OpNotEqualUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x16.NotEqual", opLen2(ssa.OpNotEqualUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x8.NotEqual", opLen2(ssa.OpNotEqualUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.OnesCount", opLen1(ssa.OpOnesCountInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.OnesCount", opLen1(ssa.OpOnesCountInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.OnesCount", opLen1(ssa.OpOnesCountInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.OnesCount", opLen1(ssa.OpOnesCountInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.OnesCount", opLen1(ssa.OpOnesCountInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.OnesCount", opLen1(ssa.OpOnesCountInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.OnesCount", opLen1(ssa.OpOnesCountInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.OnesCount", opLen1(ssa.OpOnesCountInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.OnesCount", opLen1(ssa.OpOnesCountInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.OnesCount", opLen1(ssa.OpOnesCountInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.OnesCount", opLen1(ssa.OpOnesCountInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.OnesCount", opLen1(ssa.OpOnesCountInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.OnesCount", opLen1(ssa.OpOnesCountUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.OnesCount", opLen1(ssa.OpOnesCountUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.OnesCount", opLen1(ssa.OpOnesCountUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.OnesCount", opLen1(ssa.OpOnesCountUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.OnesCount", opLen1(ssa.OpOnesCountUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.OnesCount", opLen1(ssa.OpOnesCountUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.OnesCount", opLen1(ssa.OpOnesCountUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.OnesCount", opLen1(ssa.OpOnesCountUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.OnesCount", opLen1(ssa.OpOnesCountUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.OnesCount", opLen1(ssa.OpOnesCountUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.OnesCount", opLen1(ssa.OpOnesCountUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.OnesCount", opLen1(ssa.OpOnesCountUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.Or", opLen2(ssa.OpOrInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Or", opLen2(ssa.OpOrInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Or", opLen2(ssa.OpOrInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Or", opLen2(ssa.OpOrInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Or", opLen2(ssa.OpOrInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Or", opLen2(ssa.OpOrInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Or", opLen2(ssa.OpOrInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Or", opLen2(ssa.OpOrInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Or", opLen2(ssa.OpOrInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Or", opLen2(ssa.OpOrInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Or", opLen2(ssa.OpOrInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Or", opLen2(ssa.OpOrInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.Or", opLen2(ssa.OpOrUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.Or", opLen2(ssa.OpOrUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.Or", opLen2(ssa.OpOrUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.Or", opLen2(ssa.OpOrUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.Or", opLen2(ssa.OpOrUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.Or", opLen2(ssa.OpOrUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.Or", opLen2(ssa.OpOrUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.Or", opLen2(ssa.OpOrUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.Or", opLen2(ssa.OpOrUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x4.Or", opLen2(ssa.OpOrUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.Or", opLen2(ssa.OpOrUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.Or", opLen2(ssa.OpOrUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x16.Permute", opLen2_21(ssa.OpPermuteInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x16.Permute", opLen2_21(ssa.OpPermuteUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Permute", opLen2_21(ssa.OpPermuteInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x32.Permute", opLen2_21(ssa.OpPermuteUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Permute", opLen2_21(ssa.OpPermuteInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x64.Permute", opLen2_21(ssa.OpPermuteUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Permute", opLen2_21(ssa.OpPermuteInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x8.Permute", opLen2_21(ssa.OpPermuteUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Permute", opLen2_21(ssa.OpPermuteInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x16.Permute", opLen2_21(ssa.OpPermuteUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Permute", opLen2_21(ssa.OpPermuteInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x32.Permute", opLen2_21(ssa.OpPermuteUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x8.Permute", opLen2_21(ssa.OpPermuteFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x8.Permute", opLen2_21(ssa.OpPermuteInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x8.Permute", opLen2_21(ssa.OpPermuteUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Permute", opLen2_21(ssa.OpPermuteFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x16.Permute", opLen2_21(ssa.OpPermuteInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x16.Permute", opLen2_21(ssa.OpPermuteUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x4.Permute", opLen2_21(ssa.OpPermuteFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x4.Permute", opLen2_21(ssa.OpPermuteInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x4.Permute", opLen2_21(ssa.OpPermuteUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Permute", opLen2_21(ssa.OpPermuteFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x8.Permute", opLen2_21(ssa.OpPermuteInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x8.Permute", opLen2_21(ssa.OpPermuteUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.PermuteOrZero", opLen2(ssa.OpPermuteOrZeroInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x16.PermuteOrZero", opLen2(ssa.OpPermuteOrZeroUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.PermuteOrZeroGrouped", opLen2(ssa.OpPermuteOrZeroGroupedInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.PermuteOrZeroGrouped", opLen2(ssa.OpPermuteOrZeroGroupedInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x32.PermuteOrZeroGrouped", opLen2(ssa.OpPermuteOrZeroGroupedUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.PermuteOrZeroGrouped", opLen2(ssa.OpPermuteOrZeroGroupedUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Reciprocal", opLen1(ssa.OpReciprocalFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Reciprocal", opLen1(ssa.OpReciprocalFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Reciprocal", opLen1(ssa.OpReciprocalFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Reciprocal", opLen1(ssa.OpReciprocalFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Reciprocal", opLen1(ssa.OpReciprocalFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Reciprocal", opLen1(ssa.OpReciprocalFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.ReciprocalSqrt", opLen1(ssa.OpReciprocalSqrtFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.ReciprocalSqrt", opLen1(ssa.OpReciprocalSqrtFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.ReciprocalSqrt", opLen1(ssa.OpReciprocalSqrtFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.ReciprocalSqrt", opLen1(ssa.OpReciprocalSqrtFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.ReciprocalSqrt", opLen1(ssa.OpReciprocalSqrtFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.ReciprocalSqrt", opLen1(ssa.OpReciprocalSqrtFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.RotateLeft", opLen2(ssa.OpRotateLeftInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.RotateLeft", opLen2(ssa.OpRotateLeftInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.RotateLeft", opLen2(ssa.OpRotateLeftInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.RotateLeft", opLen2(ssa.OpRotateLeftInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.RotateLeft", opLen2(ssa.OpRotateLeftInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.RotateLeft", opLen2(ssa.OpRotateLeftInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.RotateLeft", opLen2(ssa.OpRotateLeftUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.RotateLeft", opLen2(ssa.OpRotateLeftUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.RotateLeft", opLen2(ssa.OpRotateLeftUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.RotateLeft", opLen2(ssa.OpRotateLeftUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.RotateLeft", opLen2(ssa.OpRotateLeftUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.RotateLeft", opLen2(ssa.OpRotateLeftUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.RotateRight", opLen2(ssa.OpRotateRightInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.RotateRight", opLen2(ssa.OpRotateRightInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.RotateRight", opLen2(ssa.OpRotateRightInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.RotateRight", opLen2(ssa.OpRotateRightInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.RotateRight", opLen2(ssa.OpRotateRightInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.RotateRight", opLen2(ssa.OpRotateRightInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.RotateRight", opLen2(ssa.OpRotateRightUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.RotateRight", opLen2(ssa.OpRotateRightUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.RotateRight", opLen2(ssa.OpRotateRightUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.RotateRight", opLen2(ssa.OpRotateRightUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.RotateRight", opLen2(ssa.OpRotateRightUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.RotateRight", opLen2(ssa.OpRotateRightUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Round", opLen1(ssa.OpRoundFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Round", opLen1(ssa.OpRoundFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x2.Round", opLen1(ssa.OpRoundFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Round", opLen1(ssa.OpRoundFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x4.RoundScaled", opLen1Imm8(ssa.OpRoundScaledFloat32x4, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float32x8.RoundScaled", opLen1Imm8(ssa.OpRoundScaledFloat32x8, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float32x16.RoundScaled", opLen1Imm8(ssa.OpRoundScaledFloat32x16, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float64x2.RoundScaled", opLen1Imm8(ssa.OpRoundScaledFloat64x2, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float64x4.RoundScaled", opLen1Imm8(ssa.OpRoundScaledFloat64x4, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float64x8.RoundScaled", opLen1Imm8(ssa.OpRoundScaledFloat64x8, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float32x4.RoundScaledResidue", opLen1Imm8(ssa.OpRoundScaledResidueFloat32x4, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float32x8.RoundScaledResidue", opLen1Imm8(ssa.OpRoundScaledResidueFloat32x8, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float32x16.RoundScaledResidue", opLen1Imm8(ssa.OpRoundScaledResidueFloat32x16, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float64x2.RoundScaledResidue", opLen1Imm8(ssa.OpRoundScaledResidueFloat64x2, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float64x4.RoundScaledResidue", opLen1Imm8(ssa.OpRoundScaledResidueFloat64x4, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float64x8.RoundScaledResidue", opLen1Imm8(ssa.OpRoundScaledResidueFloat64x8, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Uint32x4.SHA1FourRounds", opLen2Imm8_SHA1RNDS4(ssa.OpSHA1FourRoundsUint32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint32x4.SHA1Message1", opLen2(ssa.OpSHA1Message1Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.SHA1Message2", opLen2(ssa.OpSHA1Message2Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.SHA1NextE", opLen2(ssa.OpSHA1NextEUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.SHA256Message1", opLen2(ssa.OpSHA256Message1Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.SHA256Message2", opLen2(ssa.OpSHA256Message2Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.SHA256TwoRounds", opLen3(ssa.OpSHA256TwoRoundsUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x8.SaturateToInt8", opLen1(ssa.OpSaturateToInt8Int16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.SaturateToInt8", opLen1(ssa.OpSaturateToInt8Int16x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x32.SaturateToInt8", opLen1(ssa.OpSaturateToInt8Int16x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x4.SaturateToInt8", opLen1(ssa.OpSaturateToInt8Int32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.SaturateToInt8", opLen1(ssa.OpSaturateToInt8Int32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x16.SaturateToInt8", opLen1(ssa.OpSaturateToInt8Int32x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x2.SaturateToInt8", opLen1(ssa.OpSaturateToInt8Int64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.SaturateToInt8", opLen1(ssa.OpSaturateToInt8Int64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x8.SaturateToInt8", opLen1(ssa.OpSaturateToInt8Int64x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.SaturateToInt16", opLen1(ssa.OpSaturateToInt16Int32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.SaturateToInt16", opLen1(ssa.OpSaturateToInt16Int32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x16.SaturateToInt16", opLen1(ssa.OpSaturateToInt16Int32x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x2.SaturateToInt16", opLen1(ssa.OpSaturateToInt16Int64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.SaturateToInt16", opLen1(ssa.OpSaturateToInt16Int64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x8.SaturateToInt16", opLen1(ssa.OpSaturateToInt16Int64x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.SaturateToInt16Concat", opLen2(ssa.OpSaturateToInt16ConcatInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.SaturateToInt16ConcatGrouped", opLen2(ssa.OpSaturateToInt16ConcatGroupedInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.SaturateToInt16ConcatGrouped", opLen2(ssa.OpSaturateToInt16ConcatGroupedInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.SaturateToInt32", opLen1(ssa.OpSaturateToInt32Int64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.SaturateToInt32", opLen1(ssa.OpSaturateToInt32Int64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x8.SaturateToInt32", opLen1(ssa.OpSaturateToInt32Int64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x8.SaturateToUint8", opLen1(ssa.OpSaturateToUint8Uint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.SaturateToUint8", opLen1(ssa.OpSaturateToUint8Uint16x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x32.SaturateToUint8", opLen1(ssa.OpSaturateToUint8Uint16x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x4.SaturateToUint8", opLen1(ssa.OpSaturateToUint8Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.SaturateToUint8", opLen1(ssa.OpSaturateToUint8Uint32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x16.SaturateToUint8", opLen1(ssa.OpSaturateToUint8Uint32x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x2.SaturateToUint8", opLen1(ssa.OpSaturateToUint8Uint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.SaturateToUint8", opLen1(ssa.OpSaturateToUint8Uint64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x8.SaturateToUint8", opLen1(ssa.OpSaturateToUint8Uint64x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.SaturateToUint16", opLen1(ssa.OpSaturateToUint16Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.SaturateToUint16", opLen1(ssa.OpSaturateToUint16Uint32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x16.SaturateToUint16", opLen1(ssa.OpSaturateToUint16Uint32x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x2.SaturateToUint16", opLen1(ssa.OpSaturateToUint16Uint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.SaturateToUint16", opLen1(ssa.OpSaturateToUint16Uint64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x8.SaturateToUint16", opLen1(ssa.OpSaturateToUint16Uint64x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.SaturateToUint16Concat", opLen2(ssa.OpSaturateToUint16ConcatInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.SaturateToUint16ConcatGrouped", opLen2(ssa.OpSaturateToUint16ConcatGroupedInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.SaturateToUint16ConcatGrouped", opLen2(ssa.OpSaturateToUint16ConcatGroupedInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.SaturateToUint32", opLen1(ssa.OpSaturateToUint32Uint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.SaturateToUint32", opLen1(ssa.OpSaturateToUint32Uint64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x8.SaturateToUint32", opLen1(ssa.OpSaturateToUint32Uint64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x4.Scale", opLen2(ssa.OpScaleFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Scale", opLen2(ssa.OpScaleFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Scale", opLen2(ssa.OpScaleFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Scale", opLen2(ssa.OpScaleFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Scale", opLen2(ssa.OpScaleFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Scale", opLen2(ssa.OpScaleFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.SetElem", opLen2Imm(ssa.OpSetElemFloat32x4, types.TypeVec128, 0, 3), sys.AMD64)
	addF(simdPackage, "Float64x2.SetElem", opLen2Imm(ssa.OpSetElemFloat64x2, types.TypeVec128, 0, 1), sys.AMD64)
	addF(simdPackage, "Int8x16.SetElem", opLen2Imm(ssa.OpSetElemInt8x16, types.TypeVec128, 0, 15), sys.AMD64)
	addF(simdPackage, "Int16x8.SetElem", opLen2Imm(ssa.OpSetElemInt16x8, types.TypeVec128, 0, 7), sys.AMD64)
	addF(simdPackage, "Int32x4.SetElem", opLen2Imm(ssa.OpSetElemInt32x4, types.TypeVec128, 0, 3), sys.AMD64)
	addF(simdPackage, "Int64x2.SetElem", opLen2Imm(ssa.OpSetElemInt64x2, types.TypeVec128, 0, 1), sys.AMD64)
	addF(simdPackage, "Uint8x16.SetElem", opLen2Imm(ssa.OpSetElemUint8x16, types.TypeVec128, 0, 15), sys.AMD64)
	addF(simdPackage, "Uint16x8.SetElem", opLen2Imm(ssa.OpSetElemUint16x8, types.TypeVec128, 0, 7), sys.AMD64)
	addF(simdPackage, "Uint32x4.SetElem", opLen2Imm(ssa.OpSetElemUint32x4, types.TypeVec128, 0, 3), sys.AMD64)
	addF(simdPackage, "Uint64x2.SetElem", opLen2Imm(ssa.OpSetElemUint64x2, types.TypeVec128, 0, 1), sys.AMD64)
	addF(simdPackage, "Float32x8.SetHi", opLen2(ssa.OpSetHiFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.SetHi", opLen2(ssa.OpSetHiFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x4.SetHi", opLen2(ssa.OpSetHiFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.SetHi", opLen2(ssa.OpSetHiFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x32.SetHi", opLen2(ssa.OpSetHiInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.SetHi", opLen2(ssa.OpSetHiInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x16.SetHi", opLen2(ssa.OpSetHiInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.SetHi", opLen2(ssa.OpSetHiInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x8.SetHi", opLen2(ssa.OpSetHiInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.SetHi", opLen2(ssa.OpSetHiInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x4.SetHi", opLen2(ssa.OpSetHiInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.SetHi", opLen2(ssa.OpSetHiInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x32.SetHi", opLen2(ssa.OpSetHiUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.SetHi", opLen2(ssa.OpSetHiUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x16.SetHi", opLen2(ssa.OpSetHiUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.SetHi", opLen2(ssa.OpSetHiUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x8.SetHi", opLen2(ssa.OpSetHiUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.SetHi", opLen2(ssa.OpSetHiUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x4.SetHi", opLen2(ssa.OpSetHiUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.SetHi", opLen2(ssa.OpSetHiUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x8.SetLo", opLen2(ssa.OpSetLoFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.SetLo", opLen2(ssa.OpSetLoFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x4.SetLo", opLen2(ssa.OpSetLoFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.SetLo", opLen2(ssa.OpSetLoFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x32.SetLo", opLen2(ssa.OpSetLoInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.SetLo", opLen2(ssa.OpSetLoInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x16.SetLo", opLen2(ssa.OpSetLoInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.SetLo", opLen2(ssa.OpSetLoInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x8.SetLo", opLen2(ssa.OpSetLoInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.SetLo", opLen2(ssa.OpSetLoInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x4.SetLo", opLen2(ssa.OpSetLoInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.SetLo", opLen2(ssa.OpSetLoInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x32.SetLo", opLen2(ssa.OpSetLoUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.SetLo", opLen2(ssa.OpSetLoUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x16.SetLo", opLen2(ssa.OpSetLoUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.SetLo", opLen2(ssa.OpSetLoUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x8.SetLo", opLen2(ssa.OpSetLoUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.SetLo", opLen2(ssa.OpSetLoUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x4.SetLo", opLen2(ssa.OpSetLoUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.SetLo", opLen2(ssa.OpSetLoUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.ShiftAllLeftConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod16Int16x8, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int16x16.ShiftAllLeftConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod16Int16x16, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int16x32.ShiftAllLeftConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod16Int16x32, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint16x8.ShiftAllLeftConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod16Uint16x8, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint16x16.ShiftAllLeftConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod16Uint16x16, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint16x32.ShiftAllLeftConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod16Uint16x32, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int32x4.ShiftAllLeftConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod32Int32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int32x8.ShiftAllLeftConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod32Int32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int32x16.ShiftAllLeftConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod32Int32x16, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint32x4.ShiftAllLeftConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod32Uint32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint32x8.ShiftAllLeftConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod32Uint32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint32x16.ShiftAllLeftConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod32Uint32x16, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int64x2.ShiftAllLeftConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod64Int64x2, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int64x4.ShiftAllLeftConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod64Int64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int64x8.ShiftAllLeftConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod64Int64x8, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint64x2.ShiftAllLeftConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod64Uint64x2, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint64x4.ShiftAllLeftConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod64Uint64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint64x8.ShiftAllLeftConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllLeftConcatMod64Uint64x8, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int16x8.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.ShiftAllRightConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod16Int16x8, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int16x16.ShiftAllRightConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod16Int16x16, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int16x32.ShiftAllRightConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod16Int16x32, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint16x8.ShiftAllRightConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod16Uint16x8, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint16x16.ShiftAllRightConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod16Uint16x16, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint16x32.ShiftAllRightConcatMod16", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod16Uint16x32, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int32x4.ShiftAllRightConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod32Int32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int32x8.ShiftAllRightConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod32Int32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int32x16.ShiftAllRightConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod32Int32x16, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint32x4.ShiftAllRightConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod32Uint32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint32x8.ShiftAllRightConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod32Uint32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint32x16.ShiftAllRightConcatMod32", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod32Uint32x16, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int64x2.ShiftAllRightConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod64Int64x2, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int64x4.ShiftAllRightConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod64Int64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int64x8.ShiftAllRightConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod64Int64x8, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint64x2.ShiftAllRightConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod64Uint64x2, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint64x4.ShiftAllRightConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod64Uint64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint64x8.ShiftAllRightConcatMod64", opLen2Imm8_2I(ssa.OpShiftAllRightConcatMod64Uint64x8, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int16x8.ShiftLeft", opLen2(ssa.OpShiftLeftInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.ShiftLeft", opLen2(ssa.OpShiftLeftInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.ShiftLeft", opLen2(ssa.OpShiftLeftInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.ShiftLeft", opLen2(ssa.OpShiftLeftInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.ShiftLeft", opLen2(ssa.OpShiftLeftInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.ShiftLeft", opLen2(ssa.OpShiftLeftInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.ShiftLeft", opLen2(ssa.OpShiftLeftInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.ShiftLeft", opLen2(ssa.OpShiftLeftInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.ShiftLeft", opLen2(ssa.OpShiftLeftInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.ShiftLeft", opLen2(ssa.OpShiftLeftUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.ShiftLeft", opLen2(ssa.OpShiftLeftUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.ShiftLeft", opLen2(ssa.OpShiftLeftUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.ShiftLeft", opLen2(ssa.OpShiftLeftUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.ShiftLeft", opLen2(ssa.OpShiftLeftUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.ShiftLeft", opLen2(ssa.OpShiftLeftUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.ShiftLeft", opLen2(ssa.OpShiftLeftUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.ShiftLeft", opLen2(ssa.OpShiftLeftUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.ShiftLeft", opLen2(ssa.OpShiftLeftUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.ShiftLeftConcatMod16", opLen3(ssa.OpShiftLeftConcatMod16Int16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.ShiftLeftConcatMod16", opLen3(ssa.OpShiftLeftConcatMod16Int16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.ShiftLeftConcatMod16", opLen3(ssa.OpShiftLeftConcatMod16Int16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.ShiftLeftConcatMod16", opLen3(ssa.OpShiftLeftConcatMod16Uint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.ShiftLeftConcatMod16", opLen3(ssa.OpShiftLeftConcatMod16Uint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.ShiftLeftConcatMod16", opLen3(ssa.OpShiftLeftConcatMod16Uint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.ShiftLeftConcatMod32", opLen3(ssa.OpShiftLeftConcatMod32Int32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.ShiftLeftConcatMod32", opLen3(ssa.OpShiftLeftConcatMod32Int32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.ShiftLeftConcatMod32", opLen3(ssa.OpShiftLeftConcatMod32Int32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.ShiftLeftConcatMod32", opLen3(ssa.OpShiftLeftConcatMod32Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.ShiftLeftConcatMod32", opLen3(ssa.OpShiftLeftConcatMod32Uint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.ShiftLeftConcatMod32", opLen3(ssa.OpShiftLeftConcatMod32Uint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.ShiftLeftConcatMod64", opLen3(ssa.OpShiftLeftConcatMod64Int64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.ShiftLeftConcatMod64", opLen3(ssa.OpShiftLeftConcatMod64Int64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.ShiftLeftConcatMod64", opLen3(ssa.OpShiftLeftConcatMod64Int64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.ShiftLeftConcatMod64", opLen3(ssa.OpShiftLeftConcatMod64Uint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.ShiftLeftConcatMod64", opLen3(ssa.OpShiftLeftConcatMod64Uint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.ShiftLeftConcatMod64", opLen3(ssa.OpShiftLeftConcatMod64Uint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.ShiftRight", opLen2(ssa.OpShiftRightInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.ShiftRight", opLen2(ssa.OpShiftRightInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.ShiftRight", opLen2(ssa.OpShiftRightInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.ShiftRight", opLen2(ssa.OpShiftRightInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.ShiftRight", opLen2(ssa.OpShiftRightInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.ShiftRight", opLen2(ssa.OpShiftRightInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.ShiftRight", opLen2(ssa.OpShiftRightInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.ShiftRight", opLen2(ssa.OpShiftRightInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.ShiftRight", opLen2(ssa.OpShiftRightInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.ShiftRight", opLen2(ssa.OpShiftRightUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.ShiftRight", opLen2(ssa.OpShiftRightUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.ShiftRight", opLen2(ssa.OpShiftRightUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.ShiftRight", opLen2(ssa.OpShiftRightUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.ShiftRight", opLen2(ssa.OpShiftRightUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.ShiftRight", opLen2(ssa.OpShiftRightUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.ShiftRight", opLen2(ssa.OpShiftRightUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.ShiftRight", opLen2(ssa.OpShiftRightUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.ShiftRight", opLen2(ssa.OpShiftRightUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.ShiftRightConcatMod16", opLen3(ssa.OpShiftRightConcatMod16Int16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.ShiftRightConcatMod16", opLen3(ssa.OpShiftRightConcatMod16Int16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.ShiftRightConcatMod16", opLen3(ssa.OpShiftRightConcatMod16Int16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.ShiftRightConcatMod16", opLen3(ssa.OpShiftRightConcatMod16Uint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.ShiftRightConcatMod16", opLen3(ssa.OpShiftRightConcatMod16Uint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.ShiftRightConcatMod16", opLen3(ssa.OpShiftRightConcatMod16Uint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.ShiftRightConcatMod32", opLen3(ssa.OpShiftRightConcatMod32Int32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.ShiftRightConcatMod32", opLen3(ssa.OpShiftRightConcatMod32Int32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.ShiftRightConcatMod32", opLen3(ssa.OpShiftRightConcatMod32Int32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.ShiftRightConcatMod32", opLen3(ssa.OpShiftRightConcatMod32Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.ShiftRightConcatMod32", opLen3(ssa.OpShiftRightConcatMod32Uint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.ShiftRightConcatMod32", opLen3(ssa.OpShiftRightConcatMod32Uint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.ShiftRightConcatMod64", opLen3(ssa.OpShiftRightConcatMod64Int64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.ShiftRightConcatMod64", opLen3(ssa.OpShiftRightConcatMod64Int64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.ShiftRightConcatMod64", opLen3(ssa.OpShiftRightConcatMod64Int64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.ShiftRightConcatMod64", opLen3(ssa.OpShiftRightConcatMod64Uint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.ShiftRightConcatMod64", opLen3(ssa.OpShiftRightConcatMod64Uint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.ShiftRightConcatMod64", opLen3(ssa.OpShiftRightConcatMod64Uint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Sqrt", opLen1(ssa.OpSqrtFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Sqrt", opLen1(ssa.OpSqrtFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Sqrt", opLen1(ssa.OpSqrtFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Sqrt", opLen1(ssa.OpSqrtFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Sqrt", opLen1(ssa.OpSqrtFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Sqrt", opLen1(ssa.OpSqrtFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Sub", opLen2(ssa.OpSubFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Sub", opLen2(ssa.OpSubFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x16.Sub", opLen2(ssa.OpSubFloat32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.Sub", opLen2(ssa.OpSubFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Sub", opLen2(ssa.OpSubFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x8.Sub", opLen2(ssa.OpSubFloat64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.Sub", opLen2(ssa.OpSubInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Sub", opLen2(ssa.OpSubInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Sub", opLen2(ssa.OpSubInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Sub", opLen2(ssa.OpSubInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Sub", opLen2(ssa.OpSubInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Sub", opLen2(ssa.OpSubInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Sub", opLen2(ssa.OpSubInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Sub", opLen2(ssa.OpSubInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Sub", opLen2(ssa.OpSubInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Sub", opLen2(ssa.OpSubInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Sub", opLen2(ssa.OpSubInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Sub", opLen2(ssa.OpSubInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.Sub", opLen2(ssa.OpSubUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.Sub", opLen2(ssa.OpSubUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.Sub", opLen2(ssa.OpSubUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.Sub", opLen2(ssa.OpSubUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.Sub", opLen2(ssa.OpSubUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.Sub", opLen2(ssa.OpSubUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.Sub", opLen2(ssa.OpSubUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.Sub", opLen2(ssa.OpSubUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.Sub", opLen2(ssa.OpSubUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.Sub", opLen2(ssa.OpSubUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.Sub", opLen2(ssa.OpSubUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.Sub", opLen2(ssa.OpSubUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.SubSaturated", opLen2(ssa.OpSubSaturatedInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.SubSaturated", opLen2(ssa.OpSubSaturatedInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.SubSaturated", opLen2(ssa.OpSubSaturatedInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.SubSaturated", opLen2(ssa.OpSubSaturatedInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.SubSaturated", opLen2(ssa.OpSubSaturatedInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.SubSaturated", opLen2(ssa.OpSubSaturatedInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.SubSaturated", opLen2(ssa.OpSubSaturatedUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.SubSaturated", opLen2(ssa.OpSubSaturatedUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.SubSaturated", opLen2(ssa.OpSubSaturatedUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.SubSaturated", opLen2(ssa.OpSubSaturatedUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.SubSaturated", opLen2(ssa.OpSubSaturatedUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.SubSaturated", opLen2(ssa.OpSubSaturatedUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.SumOf8AbsDiff", opLen2(ssa.OpSumOf8AbsDiffUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.SumOf8AbsDiff", opLen2(ssa.OpSumOf8AbsDiffUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.SumOf8AbsDiff", opLen2(ssa.OpSumOf8AbsDiffUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.Trunc", opLen1(ssa.OpTruncFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x8.Trunc", opLen1(ssa.OpTruncFloat32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x2.Trunc", opLen1(ssa.OpTruncFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x4.Trunc", opLen1(ssa.OpTruncFloat64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x4.TruncScaled", opLen1Imm8(ssa.OpTruncScaledFloat32x4, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float32x8.TruncScaled", opLen1Imm8(ssa.OpTruncScaledFloat32x8, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float32x16.TruncScaled", opLen1Imm8(ssa.OpTruncScaledFloat32x16, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float64x2.TruncScaled", opLen1Imm8(ssa.OpTruncScaledFloat64x2, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float64x4.TruncScaled", opLen1Imm8(ssa.OpTruncScaledFloat64x4, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float64x8.TruncScaled", opLen1Imm8(ssa.OpTruncScaledFloat64x8, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float32x4.TruncScaledResidue", opLen1Imm8(ssa.OpTruncScaledResidueFloat32x4, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float32x8.TruncScaledResidue", opLen1Imm8(ssa.OpTruncScaledResidueFloat32x8, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float32x16.TruncScaledResidue", opLen1Imm8(ssa.OpTruncScaledResidueFloat32x16, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Float64x2.TruncScaledResidue", opLen1Imm8(ssa.OpTruncScaledResidueFloat64x2, types.TypeVec128, 4), sys.AMD64)
	addF(simdPackage, "Float64x4.TruncScaledResidue", opLen1Imm8(ssa.OpTruncScaledResidueFloat64x4, types.TypeVec256, 4), sys.AMD64)
	addF(simdPackage, "Float64x8.TruncScaledResidue", opLen1Imm8(ssa.OpTruncScaledResidueFloat64x8, types.TypeVec512, 4), sys.AMD64)
	addF(simdPackage, "Int16x8.TruncToInt8", opLen1(ssa.OpTruncToInt8Int16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.TruncToInt8", opLen1(ssa.OpTruncToInt8Int16x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x32.TruncToInt8", opLen1(ssa.OpTruncToInt8Int16x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x4.TruncToInt8", opLen1(ssa.OpTruncToInt8Int32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.TruncToInt8", opLen1(ssa.OpTruncToInt8Int32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x16.TruncToInt8", opLen1(ssa.OpTruncToInt8Int32x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x2.TruncToInt8", opLen1(ssa.OpTruncToInt8Int64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.TruncToInt8", opLen1(ssa.OpTruncToInt8Int64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x8.TruncToInt8", opLen1(ssa.OpTruncToInt8Int64x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.TruncToInt16", opLen1(ssa.OpTruncToInt16Int32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.TruncToInt16", opLen1(ssa.OpTruncToInt16Int32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x16.TruncToInt16", opLen1(ssa.OpTruncToInt16Int32x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x2.TruncToInt16", opLen1(ssa.OpTruncToInt16Int64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.TruncToInt16", opLen1(ssa.OpTruncToInt16Int64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x8.TruncToInt16", opLen1(ssa.OpTruncToInt16Int64x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x2.TruncToInt32", opLen1(ssa.OpTruncToInt32Int64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.TruncToInt32", opLen1(ssa.OpTruncToInt32Int64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x8.TruncToInt32", opLen1(ssa.OpTruncToInt32Int64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x8.TruncToUint8", opLen1(ssa.OpTruncToUint8Uint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.TruncToUint8", opLen1(ssa.OpTruncToUint8Uint16x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x32.TruncToUint8", opLen1(ssa.OpTruncToUint8Uint16x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x4.TruncToUint8", opLen1(ssa.OpTruncToUint8Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.TruncToUint8", opLen1(ssa.OpTruncToUint8Uint32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x16.TruncToUint8", opLen1(ssa.OpTruncToUint8Uint32x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x2.TruncToUint8", opLen1(ssa.OpTruncToUint8Uint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.TruncToUint8", opLen1(ssa.OpTruncToUint8Uint64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x8.TruncToUint8", opLen1(ssa.OpTruncToUint8Uint64x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.TruncToUint16", opLen1(ssa.OpTruncToUint16Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.TruncToUint16", opLen1(ssa.OpTruncToUint16Uint32x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x16.TruncToUint16", opLen1(ssa.OpTruncToUint16Uint32x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x2.TruncToUint16", opLen1(ssa.OpTruncToUint16Uint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.TruncToUint16", opLen1(ssa.OpTruncToUint16Uint64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x8.TruncToUint16", opLen1(ssa.OpTruncToUint16Uint64x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x2.TruncToUint32", opLen1(ssa.OpTruncToUint32Uint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x4.TruncToUint32", opLen1(ssa.OpTruncToUint32Uint64x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x8.TruncToUint32", opLen1(ssa.OpTruncToUint32Uint64x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x16.Xor", opLen2(ssa.OpXorInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.Xor", opLen2(ssa.OpXorInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.Xor", opLen2(ssa.OpXorInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.Xor", opLen2(ssa.OpXorInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x16.Xor", opLen2(ssa.OpXorInt16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x32.Xor", opLen2(ssa.OpXorInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x4.Xor", opLen2(ssa.OpXorInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x8.Xor", opLen2(ssa.OpXorInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x16.Xor", opLen2(ssa.OpXorInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x2.Xor", opLen2(ssa.OpXorInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x4.Xor", opLen2(ssa.OpXorInt64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x8.Xor", opLen2(ssa.OpXorInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.Xor", opLen2(ssa.OpXorUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint8x32.Xor", opLen2(ssa.OpXorUint8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint8x64.Xor", opLen2(ssa.OpXorUint8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.Xor", opLen2(ssa.OpXorUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x16.Xor", opLen2(ssa.OpXorUint16x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x32.Xor", opLen2(ssa.OpXorUint16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint32x4.Xor", opLen2(ssa.OpXorUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x8.Xor", opLen2(ssa.OpXorUint32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x16.Xor", opLen2(ssa.OpXorUint32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x4.Xor", opLen2(ssa.OpXorUint64x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x8.Xor", opLen2(ssa.OpXorUint64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.Xor", opLen2(ssa.OpXorUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x16.blend", opLen3(ssa.OpblendInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int8x32.blend", opLen3(ssa.OpblendInt8x32, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int8x64.blendMasked", opLen3(ssa.OpblendMaskedInt8x64, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x32.blendMasked", opLen3(ssa.OpblendMaskedInt16x32, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int32x16.blendMasked", opLen3(ssa.OpblendMaskedInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int64x8.blendMasked", opLen3(ssa.OpblendMaskedInt64x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float64x2.broadcast1To2", opLen1(ssa.Opbroadcast1To2Float64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x2.broadcast1To2", opLen1(ssa.Opbroadcast1To2Int64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x2.broadcast1To2", opLen1(ssa.Opbroadcast1To2Uint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x2.broadcast1To2Masked", opLen2(ssa.Opbroadcast1To2MaskedFloat64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x2.broadcast1To2Masked", opLen2(ssa.Opbroadcast1To2MaskedInt64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x2.broadcast1To2Masked", opLen2(ssa.Opbroadcast1To2MaskedUint64x2, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float32x4.broadcast1To4", opLen1(ssa.Opbroadcast1To4Float32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x2.broadcast1To4", opLen1(ssa.Opbroadcast1To4Float64x2, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x4.broadcast1To4", opLen1(ssa.Opbroadcast1To4Int32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x2.broadcast1To4", opLen1(ssa.Opbroadcast1To4Int64x2, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x4.broadcast1To4", opLen1(ssa.Opbroadcast1To4Uint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x2.broadcast1To4", opLen1(ssa.Opbroadcast1To4Uint64x2, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x4.broadcast1To4Masked", opLen2(ssa.Opbroadcast1To4MaskedFloat32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Float64x2.broadcast1To4Masked", opLen2(ssa.Opbroadcast1To4MaskedFloat64x2, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x4.broadcast1To4Masked", opLen2(ssa.Opbroadcast1To4MaskedInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int64x2.broadcast1To4Masked", opLen2(ssa.Opbroadcast1To4MaskedInt64x2, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x4.broadcast1To4Masked", opLen2(ssa.Opbroadcast1To4MaskedUint32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint64x2.broadcast1To4Masked", opLen2(ssa.Opbroadcast1To4MaskedUint64x2, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float32x4.broadcast1To8", opLen1(ssa.Opbroadcast1To8Float32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x2.broadcast1To8", opLen1(ssa.Opbroadcast1To8Float64x2, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.broadcast1To8", opLen1(ssa.Opbroadcast1To8Int16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.broadcast1To8", opLen1(ssa.Opbroadcast1To8Int32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x2.broadcast1To8", opLen1(ssa.Opbroadcast1To8Int64x2, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.broadcast1To8", opLen1(ssa.Opbroadcast1To8Uint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.broadcast1To8", opLen1(ssa.Opbroadcast1To8Uint32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x2.broadcast1To8", opLen1(ssa.Opbroadcast1To8Uint64x2, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.broadcast1To8Masked", opLen2(ssa.Opbroadcast1To8MaskedFloat32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Float64x2.broadcast1To8Masked", opLen2(ssa.Opbroadcast1To8MaskedFloat64x2, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int16x8.broadcast1To8Masked", opLen2(ssa.Opbroadcast1To8MaskedInt16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int32x4.broadcast1To8Masked", opLen2(ssa.Opbroadcast1To8MaskedInt32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int64x2.broadcast1To8Masked", opLen2(ssa.Opbroadcast1To8MaskedInt64x2, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint16x8.broadcast1To8Masked", opLen2(ssa.Opbroadcast1To8MaskedUint16x8, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint32x4.broadcast1To8Masked", opLen2(ssa.Opbroadcast1To8MaskedUint32x4, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint64x2.broadcast1To8Masked", opLen2(ssa.Opbroadcast1To8MaskedUint64x2, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.broadcast1To16", opLen1(ssa.Opbroadcast1To16Float32x4, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.broadcast1To16", opLen1(ssa.Opbroadcast1To16Int8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x8.broadcast1To16", opLen1(ssa.Opbroadcast1To16Int16x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x4.broadcast1To16", opLen1(ssa.Opbroadcast1To16Int32x4, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.broadcast1To16", opLen1(ssa.Opbroadcast1To16Uint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x8.broadcast1To16", opLen1(ssa.Opbroadcast1To16Uint16x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x4.broadcast1To16", opLen1(ssa.Opbroadcast1To16Uint32x4, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Float32x4.broadcast1To16Masked", opLen2(ssa.Opbroadcast1To16MaskedFloat32x4, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.broadcast1To16Masked", opLen2(ssa.Opbroadcast1To16MaskedInt8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Int16x8.broadcast1To16Masked", opLen2(ssa.Opbroadcast1To16MaskedInt16x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int32x4.broadcast1To16Masked", opLen2(ssa.Opbroadcast1To16MaskedInt32x4, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.broadcast1To16Masked", opLen2(ssa.Opbroadcast1To16MaskedUint8x16, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Uint16x8.broadcast1To16Masked", opLen2(ssa.Opbroadcast1To16MaskedUint16x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint32x4.broadcast1To16Masked", opLen2(ssa.Opbroadcast1To16MaskedUint32x4, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.broadcast1To32", opLen1(ssa.Opbroadcast1To32Int8x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x8.broadcast1To32", opLen1(ssa.Opbroadcast1To32Int16x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.broadcast1To32", opLen1(ssa.Opbroadcast1To32Uint8x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x8.broadcast1To32", opLen1(ssa.Opbroadcast1To32Uint16x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.broadcast1To32Masked", opLen2(ssa.Opbroadcast1To32MaskedInt8x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Int16x8.broadcast1To32Masked", opLen2(ssa.Opbroadcast1To32MaskedInt16x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.broadcast1To32Masked", opLen2(ssa.Opbroadcast1To32MaskedUint8x16, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Uint16x8.broadcast1To32Masked", opLen2(ssa.Opbroadcast1To32MaskedUint16x8, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.broadcast1To64", opLen1(ssa.Opbroadcast1To64Int8x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.broadcast1To64", opLen1(ssa.Opbroadcast1To64Uint8x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Int8x16.broadcast1To64Masked", opLen2(ssa.Opbroadcast1To64MaskedInt8x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint8x16.broadcast1To64Masked", opLen2(ssa.Opbroadcast1To64MaskedUint8x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Uint64x2.carrylessMultiply", opLen2Imm8(ssa.OpcarrylessMultiplyUint64x2, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint64x4.carrylessMultiply", opLen2Imm8(ssa.OpcarrylessMultiplyUint64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint64x8.carrylessMultiply", opLen2Imm8(ssa.OpcarrylessMultiplyUint64x8, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Float32x4.concatSelectedConstant", opLen2Imm8(ssa.OpconcatSelectedConstantFloat32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Float64x2.concatSelectedConstant", opLen2Imm8(ssa.OpconcatSelectedConstantFloat64x2, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int32x4.concatSelectedConstant", opLen2Imm8(ssa.OpconcatSelectedConstantInt32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int64x2.concatSelectedConstant", opLen2Imm8(ssa.OpconcatSelectedConstantInt64x2, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint32x4.concatSelectedConstant", opLen2Imm8(ssa.OpconcatSelectedConstantUint32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint64x2.concatSelectedConstant", opLen2Imm8(ssa.OpconcatSelectedConstantUint64x2, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Float32x8.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedFloat32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Float32x16.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedFloat32x16, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Float64x4.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedFloat64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Float64x8.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedFloat64x8, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int32x8.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedInt32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int32x16.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedInt32x16, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int64x4.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedInt64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int64x8.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedInt64x8, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint32x8.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedUint32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint32x16.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedUint32x16, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint64x4.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedUint64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint64x8.concatSelectedConstantGrouped", opLen2Imm8(ssa.OpconcatSelectedConstantGroupedUint64x8, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int32x4.permuteScalars", opLen1Imm8(ssa.OppermuteScalarsInt32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint32x4.permuteScalars", opLen1Imm8(ssa.OppermuteScalarsUint32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int32x8.permuteScalarsGrouped", opLen1Imm8(ssa.OppermuteScalarsGroupedInt32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int32x16.permuteScalarsGrouped", opLen1Imm8(ssa.OppermuteScalarsGroupedInt32x16, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint32x8.permuteScalarsGrouped", opLen1Imm8(ssa.OppermuteScalarsGroupedUint32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint32x16.permuteScalarsGrouped", opLen1Imm8(ssa.OppermuteScalarsGroupedUint32x16, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int16x8.permuteScalarsHi", opLen1Imm8(ssa.OppermuteScalarsHiInt16x8, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint16x8.permuteScalarsHi", opLen1Imm8(ssa.OppermuteScalarsHiUint16x8, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int16x16.permuteScalarsHiGrouped", opLen1Imm8(ssa.OppermuteScalarsHiGroupedInt16x16, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int16x32.permuteScalarsHiGrouped", opLen1Imm8(ssa.OppermuteScalarsHiGroupedInt16x32, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint16x16.permuteScalarsHiGrouped", opLen1Imm8(ssa.OppermuteScalarsHiGroupedUint16x16, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint16x32.permuteScalarsHiGrouped", opLen1Imm8(ssa.OppermuteScalarsHiGroupedUint16x32, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int16x8.permuteScalarsLo", opLen1Imm8(ssa.OppermuteScalarsLoInt16x8, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint16x8.permuteScalarsLo", opLen1Imm8(ssa.OppermuteScalarsLoUint16x8, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int16x16.permuteScalarsLoGrouped", opLen1Imm8(ssa.OppermuteScalarsLoGroupedInt16x16, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int16x32.permuteScalarsLoGrouped", opLen1Imm8(ssa.OppermuteScalarsLoGroupedInt16x32, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint16x16.permuteScalarsLoGrouped", opLen1Imm8(ssa.OppermuteScalarsLoGroupedUint16x16, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint16x32.permuteScalarsLoGrouped", opLen1Imm8(ssa.OppermuteScalarsLoGroupedUint16x32, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int32x4.tern", opLen3Imm8(ssa.OpternInt32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int32x8.tern", opLen3Imm8(ssa.OpternInt32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int32x16.tern", opLen3Imm8(ssa.OpternInt32x16, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Int64x2.tern", opLen3Imm8(ssa.OpternInt64x2, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Int64x4.tern", opLen3Imm8(ssa.OpternInt64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Int64x8.tern", opLen3Imm8(ssa.OpternInt64x8, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint32x4.tern", opLen3Imm8(ssa.OpternUint32x4, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint32x8.tern", opLen3Imm8(ssa.OpternUint32x8, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint32x16.tern", opLen3Imm8(ssa.OpternUint32x16, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Uint64x2.tern", opLen3Imm8(ssa.OpternUint64x2, types.TypeVec128, 0), sys.AMD64)
	addF(simdPackage, "Uint64x4.tern", opLen3Imm8(ssa.OpternUint64x4, types.TypeVec256, 0), sys.AMD64)
	addF(simdPackage, "Uint64x8.tern", opLen3Imm8(ssa.OpternUint64x8, types.TypeVec512, 0), sys.AMD64)
	addF(simdPackage, "Float32x4.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x4.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x4.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x4.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x4.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x4.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x4.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x4.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x4.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x8.AsFloat64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x8.AsInt8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x8.AsInt16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x8.AsInt32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x8.AsInt64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x8.AsUint8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x8.AsUint16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x8.AsUint32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x8.AsUint64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x16.AsFloat64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x16.AsInt8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x16.AsInt16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x16.AsInt32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x16.AsInt64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x16.AsUint8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x16.AsUint16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x16.AsUint32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x16.AsUint64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x2.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x2.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x2.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x2.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x2.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x2.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x2.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x2.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x2.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x4.AsFloat32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x4.AsInt8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x4.AsInt16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x4.AsInt32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x4.AsInt64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x4.AsUint8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x4.AsUint16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x4.AsUint32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x4.AsUint64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x8.AsFloat32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x8.AsInt8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x8.AsInt16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x8.AsInt32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x8.AsInt64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x8.AsUint8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x8.AsUint16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x8.AsUint32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x8.AsUint64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.AsFloat32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.AsFloat64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.AsInt16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.AsInt32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.AsInt64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.AsUint8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.AsUint16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.AsUint32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.AsUint64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.AsFloat32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.AsFloat64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.AsInt16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.AsInt32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.AsInt64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.AsUint8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.AsUint16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.AsUint32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.AsUint64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.AsFloat32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.AsFloat64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.AsInt8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.AsInt32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.AsInt64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.AsUint8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.AsUint16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.AsUint32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.AsUint64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.AsFloat32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.AsFloat64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.AsInt8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.AsInt32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.AsInt64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.AsUint8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.AsUint16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.AsUint32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.AsUint64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.AsFloat32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.AsFloat64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.AsInt8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.AsInt16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.AsInt64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.AsUint8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.AsUint16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.AsUint32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.AsUint64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.AsFloat32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.AsFloat64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.AsInt8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.AsInt16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.AsInt64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.AsUint8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.AsUint16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.AsUint32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.AsUint64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.AsFloat32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.AsFloat64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.AsInt8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.AsInt16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.AsInt32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.AsUint8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.AsUint16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.AsUint32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.AsUint64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.AsFloat32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.AsFloat64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.AsInt8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.AsInt16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.AsInt32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.AsUint8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.AsUint16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.AsUint32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.AsUint64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.BitsToInt8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.ConvertToInt8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.ConvertToUint8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x16.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.AsFloat32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.AsFloat64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.AsInt8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.BitsToInt8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.ConvertToInt8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.ConvertToUint8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.AsInt16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.AsInt32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.AsInt64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.AsUint16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.AsUint32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.AsUint64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x32.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.AsFloat32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.AsFloat64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.AsInt8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.BitsToInt8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.ConvertToInt8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.ConvertToUint8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.AsInt16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.AsInt32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.AsInt64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.AsUint16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.AsUint32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.AsUint64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint8x64.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.BitsToInt16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.ConvertToInt16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.ConvertToUint16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x8.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.AsFloat32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.AsFloat64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.AsInt8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.AsInt16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.BitsToInt16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.ConvertToInt16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.ConvertToUint16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.AsInt32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.AsInt64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.AsUint8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.AsUint32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.AsUint64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x16.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.AsFloat32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.AsFloat64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.AsInt8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.AsInt16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.BitsToInt16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.ConvertToInt16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.ConvertToUint16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.AsInt32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.AsInt64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.AsUint8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.AsUint32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.AsUint64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint16x32.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.BitsToFloat32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x4.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.BitsToInt32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.ConvertToInt32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.ConvertToUint32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x4.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.AsFloat32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.BitsToFloat32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x8.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.AsFloat64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.AsInt8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.AsInt16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.AsInt32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.BitsToInt32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.ConvertToInt32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.ConvertToUint32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.AsInt64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.AsUint8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.AsUint16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.AsUint64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x8.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.AsFloat32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.BitsToFloat32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float32x16.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.AsFloat64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.AsInt8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.AsInt16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.AsInt32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.BitsToInt32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.ConvertToInt32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.ConvertToUint32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.AsInt64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.AsUint8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.AsUint16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.AsUint64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint32x16.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.BitsToFloat64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x2.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.BitsToInt64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.ConvertToInt64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.ConvertToUint64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x2.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.AsFloat32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.AsFloat64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.BitsToFloat64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x4.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.AsInt8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.AsInt16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.AsInt32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.AsInt64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.BitsToInt64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.ConvertToInt64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.ConvertToUint64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.AsUint8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.AsUint16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.AsUint32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x4.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.AsFloat32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.AsFloat64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.BitsToFloat64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Float64x8.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.AsInt8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.AsInt16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.AsInt32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.AsInt64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.BitsToInt64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.ConvertToInt64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.ConvertToUint64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.AsUint8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.AsUint16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.AsUint32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Uint64x8.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "LoadFloat32x4Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Float32x4.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadFloat32x8Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Float32x8.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadFloat32x16Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Float32x16.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadFloat64x2Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Float64x2.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadFloat64x4Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Float64x4.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadFloat64x8Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Float64x8.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt8x16Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int8x16.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt8x32Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int8x32.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt8x64Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int8x64.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt16x8Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int16x8.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt16x16Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int16x16.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt16x32Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int16x32.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt32x4Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int32x4.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt32x8Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int32x8.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt32x16Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int32x16.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt64x2Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int64x2.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt64x4Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int64x4.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadInt64x8Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Int64x8.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint8x16Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint8x16.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint8x32Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint8x32.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint8x64Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint8x64.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint16x8Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint16x8.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint16x16Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint16x16.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint16x32Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint16x32.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint32x4Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint32x4.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint32x8Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint32x8.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint32x16Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint32x16.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint64x2Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint64x2.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint64x4Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint64x4.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "LoadUint64x8Array", simdLoad(), sys.AMD64)
	addF(simdPackage, "Uint64x8.StoreArray", simdStore(), sys.AMD64)
	addF(simdPackage, "Float32x4.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked32), sys.AMD64)
	addF(simdPackage, "Float32x8.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked32), sys.AMD64)
	addF(simdPackage, "Float32x16.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked32), sys.AMD64)
	addF(simdPackage, "Float64x2.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked64), sys.AMD64)
	addF(simdPackage, "Float64x4.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked64), sys.AMD64)
	addF(simdPackage, "Float64x8.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked64), sys.AMD64)
	addF(simdPackage, "Int8x64.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked8), sys.AMD64)
	addF(simdPackage, "Int16x32.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked16), sys.AMD64)
	addF(simdPackage, "Int32x4.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked32), sys.AMD64)
	addF(simdPackage, "Int32x8.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked32), sys.AMD64)
	addF(simdPackage, "Int32x16.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked32), sys.AMD64)
	addF(simdPackage, "Int64x2.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked64), sys.AMD64)
	addF(simdPackage, "Int64x4.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked64), sys.AMD64)
	addF(simdPackage, "Int64x8.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked64), sys.AMD64)
	addF(simdPackage, "Uint8x64.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked8), sys.AMD64)
	addF(simdPackage, "Uint16x32.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked16), sys.AMD64)
	addF(simdPackage, "Uint32x4.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked32), sys.AMD64)
	addF(simdPackage, "Uint32x8.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked32), sys.AMD64)
	addF(simdPackage, "Uint32x16.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked32), sys.AMD64)
	addF(simdPackage, "Uint64x2.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked64), sys.AMD64)
	addF(simdPackage, "Uint64x4.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked64), sys.AMD64)
	addF(simdPackage, "Uint64x8.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked64), sys.AMD64)
	addF(simdPackage, "Mask8x64.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked8), sys.AMD64)
	addF(simdPackage, "Mask16x32.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked16), sys.AMD64)
	addF(simdPackage, "Mask32x16.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked32), sys.AMD64)
	addF(simdPackage, "Mask64x8.StoreArrayMasked", simdMaskedStore(ssa.OpStoreMasked64), sys.AMD64)
	addF(simdPackage, "Mask8x16.ToInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x16.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask8x16.And", opLen2(ssa.OpAndInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Mask8x16.Or", opLen2(ssa.OpOrInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Mask8x16FromBits", simdCvtVToMask(8, 16), sys.AMD64)
	addF(simdPackage, "Mask8x16.ToBits", simdCvtMaskToV(8, 16), sys.AMD64)
	addF(simdPackage, "Mask8x32.ToInt8x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x32.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask8x32.And", opLen2(ssa.OpAndInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Mask8x32.Or", opLen2(ssa.OpOrInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Mask8x32FromBits", simdCvtVToMask(8, 32), sys.AMD64)
	addF(simdPackage, "Mask8x32.ToBits", simdCvtMaskToV(8, 32), sys.AMD64)
	addF(simdPackage, "Mask8x64.ToInt8x64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int8x64.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask8x64.And", opLen2(ssa.OpAndInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Mask8x64.Or", opLen2(ssa.OpOrInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Mask8x64FromBits", simdCvtVToMask(8, 64), sys.AMD64)
	addF(simdPackage, "Mask8x64.ToBits", simdCvtMaskToV(8, 64), sys.AMD64)
	addF(simdPackage, "Mask16x8.ToInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x8.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask16x8.And", opLen2(ssa.OpAndInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Mask16x8.Or", opLen2(ssa.OpOrInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Mask16x8FromBits", simdCvtVToMask(16, 8), sys.AMD64)
	addF(simdPackage, "Mask16x8.ToBits", simdCvtMaskToV(16, 8), sys.AMD64)
	addF(simdPackage, "Mask16x16.ToInt16x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x16.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask16x16.And", opLen2(ssa.OpAndInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Mask16x16.Or", opLen2(ssa.OpOrInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Mask16x16FromBits", simdCvtVToMask(16, 16), sys.AMD64)
	addF(simdPackage, "Mask16x16.ToBits", simdCvtMaskToV(16, 16), sys.AMD64)
	addF(simdPackage, "Mask16x32.ToInt16x32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int16x32.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask16x32.And", opLen2(ssa.OpAndInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Mask16x32.Or", opLen2(ssa.OpOrInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Mask16x32FromBits", simdCvtVToMask(16, 32), sys.AMD64)
	addF(simdPackage, "Mask16x32.ToBits", simdCvtMaskToV(16, 32), sys.AMD64)
	addF(simdPackage, "Mask32x4.ToInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x4.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask32x4.And", opLen2(ssa.OpAndInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Mask32x4.Or", opLen2(ssa.OpOrInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Mask32x4FromBits", simdCvtVToMask(32, 4), sys.AMD64)
	addF(simdPackage, "Mask32x4.ToBits", simdCvtMaskToV(32, 4), sys.AMD64)
	addF(simdPackage, "Mask32x8.ToInt32x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x8.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask32x8.And", opLen2(ssa.OpAndInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Mask32x8.Or", opLen2(ssa.OpOrInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Mask32x8FromBits", simdCvtVToMask(32, 8), sys.AMD64)
	addF(simdPackage, "Mask32x8.ToBits", simdCvtMaskToV(32, 8), sys.AMD64)
	addF(simdPackage, "Mask32x16.ToInt32x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int32x16.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask32x16.And", opLen2(ssa.OpAndInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Mask32x16.Or", opLen2(ssa.OpOrInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Mask32x16FromBits", simdCvtVToMask(32, 16), sys.AMD64)
	addF(simdPackage, "Mask32x16.ToBits", simdCvtMaskToV(32, 16), sys.AMD64)
	addF(simdPackage, "Mask64x2.ToInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x2.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask64x2.And", opLen2(ssa.OpAndInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Mask64x2.Or", opLen2(ssa.OpOrInt32x4, types.TypeVec128), sys.AMD64)
	addF(simdPackage, "Mask64x2FromBits", simdCvtVToMask(64, 2), sys.AMD64)
	addF(simdPackage, "Mask64x2.ToBits", simdCvtMaskToV(64, 2), sys.AMD64)
	addF(simdPackage, "Mask64x4.ToInt64x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x4.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask64x4.And", opLen2(ssa.OpAndInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Mask64x4.Or", opLen2(ssa.OpOrInt32x8, types.TypeVec256), sys.AMD64)
	addF(simdPackage, "Mask64x4FromBits", simdCvtVToMask(64, 4), sys.AMD64)
	addF(simdPackage, "Mask64x4.ToBits", simdCvtMaskToV(64, 4), sys.AMD64)
	addF(simdPackage, "Mask64x8.ToInt64x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Int64x8.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.AMD64)
	addF(simdPackage, "Mask64x8.And", opLen2(ssa.OpAndInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Mask64x8.Or", opLen2(ssa.OpOrInt32x16, types.TypeVec512), sys.AMD64)
	addF(simdPackage, "Mask64x8FromBits", simdCvtVToMask(64, 8), sys.AMD64)
	addF(simdPackage, "Mask64x8.ToBits", simdCvtMaskToV(64, 8), sys.AMD64)
}

```

// === FILE: references/go/src/cmd/compile/internal/ssagen/simdARM64intrinsics.go ===
```go
// Code generated by 'simdgen -o godefs -goroot $GOROOT -arch arm64 -arm64Path $ARM64_ISA_PATH go_arm64.yaml types.yaml categories.yaml'; DO NOT EDIT.
package ssagen

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/types"
	"cmd/internal/sys"
)

func simdARM64Intrinsics(addF func(pkg, fn string, b intrinsicBuilder, archFamilies ...sys.ArchFamily)) {

	addF(simdPackage, "Float32x4.Abs", opLen1(ssa.OpAbsFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Abs", opLen1(ssa.OpAbsFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Abs", opLen1(ssa.OpAbsInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Abs", opLen1(ssa.OpAbsInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Abs", opLen1(ssa.OpAbsInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.Abs", opLen1(ssa.OpAbsInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Add", opLen2(ssa.OpAddFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Add", opLen2(ssa.OpAddFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Add", opLen2(ssa.OpAddInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Add", opLen2(ssa.OpAddInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Add", opLen2(ssa.OpAddInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.Add", opLen2(ssa.OpAddInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Add", opLen2(ssa.OpAddUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Add", opLen2(ssa.OpAddUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Add", opLen2(ssa.OpAddUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.Add", opLen2(ssa.OpAddUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.AddSaturated", opLen2(ssa.OpAddSaturatedInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.AddSaturated", opLen2(ssa.OpAddSaturatedInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.AddSaturated", opLen2(ssa.OpAddSaturatedInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.AddSaturated", opLen2(ssa.OpAddSaturatedInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.AddSaturated", opLen2(ssa.OpAddSaturatedUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.AddSaturated", opLen2(ssa.OpAddSaturatedUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.AddSaturated", opLen2(ssa.OpAddSaturatedUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.AddSaturated", opLen2(ssa.OpAddSaturatedUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.And", opLen2(ssa.OpAndInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.And", opLen2(ssa.OpAndInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.And", opLen2(ssa.OpAndInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.And", opLen2(ssa.OpAndInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.And", opLen2(ssa.OpAndUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.And", opLen2(ssa.OpAndUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.And", opLen2(ssa.OpAndUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.And", opLen2(ssa.OpAndUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.AndNot", opLen2(ssa.OpAndNotInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.AndNot", opLen2(ssa.OpAndNotInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.AndNot", opLen2(ssa.OpAndNotInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.AndNot", opLen2(ssa.OpAndNotInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.AndNot", opLen2(ssa.OpAndNotUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.AndNot", opLen2(ssa.OpAndNotUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.AndNot", opLen2(ssa.OpAndNotUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.AndNot", opLen2(ssa.OpAndNotUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Average", opLen2(ssa.OpAverageInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Average", opLen2(ssa.OpAverageInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Average", opLen2(ssa.OpAverageInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Average", opLen2(ssa.OpAverageUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Average", opLen2(ssa.OpAverageUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Average", opLen2(ssa.OpAverageUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Ceil", opLen1(ssa.OpCeilFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Ceil", opLen1(ssa.OpCeilFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.ConcatAddPairs", opLen2(ssa.OpConcatAddPairsUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.ConcatEven", opLen2(ssa.OpConcatEvenInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.ConcatEven", opLen2(ssa.OpConcatEvenInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.ConcatEven", opLen2(ssa.OpConcatEvenInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.ConcatEven", opLen2(ssa.OpConcatEvenInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.ConcatEven", opLen2(ssa.OpConcatEvenUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.ConcatEven", opLen2(ssa.OpConcatEvenUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.ConcatEven", opLen2(ssa.OpConcatEvenUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.ConcatEven", opLen2(ssa.OpConcatEvenUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.ConcatOdd", opLen2(ssa.OpConcatOddInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.ConcatOdd", opLen2(ssa.OpConcatOddInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.ConcatOdd", opLen2(ssa.OpConcatOddInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.ConcatOdd", opLen2(ssa.OpConcatOddInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.ConcatOdd", opLen2(ssa.OpConcatOddUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.ConcatOdd", opLen2(ssa.OpConcatOddUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.ConcatOdd", opLen2(ssa.OpConcatOddUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.ConcatOdd", opLen2(ssa.OpConcatOddUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.ConcatShiftBytesRight", opLen2Imm_2I(ssa.OpConcatShiftBytesRightUint8x16, types.TypeVec128, 0, 15), sys.ARM64)
	addF(simdPackage, "Float32x4.ConvertLo2ToFloat64", opLen1(ssa.OpConvertLo2ToFloat64Float32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Float64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Int32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.ConvertToFloat32", opLen1(ssa.OpConvertToFloat32Uint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Int64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.ConvertToFloat64", opLen1(ssa.OpConvertToFloat64Uint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.ConvertToInt32", opLen1(ssa.OpConvertToInt32Float32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.ConvertToInt64", opLen1(ssa.OpConvertToInt64Float64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.ConvertToUint32", opLen1(ssa.OpConvertToUint32Float32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.ConvertToUint64", opLen1(ssa.OpConvertToUint64Float64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Div", opLen2(ssa.OpDivFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Div", opLen2(ssa.OpDivFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Equal", opLen2(ssa.OpEqualFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Equal", opLen2(ssa.OpEqualFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Equal", opLen2(ssa.OpEqualInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Equal", opLen2(ssa.OpEqualInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Equal", opLen2(ssa.OpEqualInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.Equal", opLen2(ssa.OpEqualInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Equal", opLen2(ssa.OpEqualUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Equal", opLen2(ssa.OpEqualUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Equal", opLen2(ssa.OpEqualUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.Equal", opLen2(ssa.OpEqualUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.ExtendLo2ToInt64", opLen1(ssa.OpExtendLo2ToInt64Int32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.ExtendLo2ToUint64", opLen1(ssa.OpExtendLo2ToUint64Uint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.ExtendLo4ToInt32", opLen1(ssa.OpExtendLo4ToInt32Int16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.ExtendLo4ToUint32", opLen1(ssa.OpExtendLo4ToUint32Uint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.ExtendLo8ToInt16", opLen1(ssa.OpExtendLo8ToInt16Int8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.ExtendLo8ToUint16", opLen1(ssa.OpExtendLo8ToUint16Uint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Floor", opLen1(ssa.OpFloorFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Floor", opLen1(ssa.OpFloorFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.GetElem", opLen1Imm(ssa.OpGetElemFloat32x4, types.Types[types.TFLOAT32], 0, 3), sys.ARM64)
	addF(simdPackage, "Float64x2.GetElem", opLen1Imm(ssa.OpGetElemFloat64x2, types.Types[types.TFLOAT64], 0, 1), sys.ARM64)
	addF(simdPackage, "Int8x16.GetElem", opLen1Imm(ssa.OpGetElemInt8x16, types.Types[types.TINT8], 0, 15), sys.ARM64)
	addF(simdPackage, "Int16x8.GetElem", opLen1Imm(ssa.OpGetElemInt16x8, types.Types[types.TINT16], 0, 7), sys.ARM64)
	addF(simdPackage, "Int32x4.GetElem", opLen1Imm(ssa.OpGetElemInt32x4, types.Types[types.TINT32], 0, 3), sys.ARM64)
	addF(simdPackage, "Int64x2.GetElem", opLen1Imm(ssa.OpGetElemInt64x2, types.Types[types.TINT64], 0, 1), sys.ARM64)
	addF(simdPackage, "Uint8x16.GetElem", opLen1Imm(ssa.OpGetElemUint8x16, types.Types[types.TUINT8], 0, 15), sys.ARM64)
	addF(simdPackage, "Uint16x8.GetElem", opLen1Imm(ssa.OpGetElemUint16x8, types.Types[types.TUINT16], 0, 7), sys.ARM64)
	addF(simdPackage, "Uint32x4.GetElem", opLen1Imm(ssa.OpGetElemUint32x4, types.Types[types.TUINT32], 0, 3), sys.ARM64)
	addF(simdPackage, "Uint64x2.GetElem", opLen1Imm(ssa.OpGetElemUint64x2, types.Types[types.TUINT64], 0, 1), sys.ARM64)
	addF(simdPackage, "Float32x4.Greater", opLen2(ssa.OpGreaterFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Greater", opLen2(ssa.OpGreaterFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Greater", opLen2(ssa.OpGreaterInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Greater", opLen2(ssa.OpGreaterInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Greater", opLen2(ssa.OpGreaterInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.Greater", opLen2(ssa.OpGreaterInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Greater", opLen2(ssa.OpGreaterUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Greater", opLen2(ssa.OpGreaterUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Greater", opLen2(ssa.OpGreaterUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.Greater", opLen2(ssa.OpGreaterUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.GreaterEqual", opLen2(ssa.OpGreaterEqualFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.GreaterEqual", opLen2(ssa.OpGreaterEqualFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.GreaterEqual", opLen2(ssa.OpGreaterEqualInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.GreaterEqual", opLen2(ssa.OpGreaterEqualInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.GreaterEqual", opLen2(ssa.OpGreaterEqualInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.GreaterEqual", opLen2(ssa.OpGreaterEqualInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.GreaterEqual", opLen2(ssa.OpGreaterEqualUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.GreaterEqual", opLen2(ssa.OpGreaterEqualUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.GreaterEqual", opLen2(ssa.OpGreaterEqualUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.GreaterEqual", opLen2(ssa.OpGreaterEqualUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.InterleaveEven", opLen2(ssa.OpInterleaveEvenInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.InterleaveEven", opLen2(ssa.OpInterleaveEvenInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.InterleaveEven", opLen2(ssa.OpInterleaveEvenInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.InterleaveEven", opLen2(ssa.OpInterleaveEvenInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.InterleaveEven", opLen2(ssa.OpInterleaveEvenUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.InterleaveEven", opLen2(ssa.OpInterleaveEvenUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.InterleaveEven", opLen2(ssa.OpInterleaveEvenUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.InterleaveEven", opLen2(ssa.OpInterleaveEvenUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.InterleaveHi", opLen2(ssa.OpInterleaveHiInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.InterleaveHi", opLen2(ssa.OpInterleaveHiInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.InterleaveHi", opLen2(ssa.OpInterleaveHiInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.InterleaveHi", opLen2(ssa.OpInterleaveHiInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.InterleaveHi", opLen2(ssa.OpInterleaveHiUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.InterleaveHi", opLen2(ssa.OpInterleaveHiUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.InterleaveHi", opLen2(ssa.OpInterleaveHiUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.InterleaveHi", opLen2(ssa.OpInterleaveHiUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.InterleaveLo", opLen2(ssa.OpInterleaveLoInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.InterleaveLo", opLen2(ssa.OpInterleaveLoInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.InterleaveLo", opLen2(ssa.OpInterleaveLoInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.InterleaveLo", opLen2(ssa.OpInterleaveLoInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.InterleaveLo", opLen2(ssa.OpInterleaveLoUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.InterleaveLo", opLen2(ssa.OpInterleaveLoUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.InterleaveLo", opLen2(ssa.OpInterleaveLoUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.InterleaveLo", opLen2(ssa.OpInterleaveLoUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.InterleaveOdd", opLen2(ssa.OpInterleaveOddInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.InterleaveOdd", opLen2(ssa.OpInterleaveOddInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.InterleaveOdd", opLen2(ssa.OpInterleaveOddInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.InterleaveOdd", opLen2(ssa.OpInterleaveOddInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.InterleaveOdd", opLen2(ssa.OpInterleaveOddUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.InterleaveOdd", opLen2(ssa.OpInterleaveOddUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.InterleaveOdd", opLen2(ssa.OpInterleaveOddUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.InterleaveOdd", opLen2(ssa.OpInterleaveOddUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.LeadingSignBits", opLen1(ssa.OpLeadingSignBitsInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.LeadingSignBits", opLen1(ssa.OpLeadingSignBitsInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.LeadingSignBits", opLen1(ssa.OpLeadingSignBitsInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.LeadingSignBits", opLen1(ssa.OpLeadingSignBitsUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.LeadingSignBits", opLen1(ssa.OpLeadingSignBitsUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.LeadingSignBits", opLen1(ssa.OpLeadingSignBitsUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.LeadingZeros", opLen1(ssa.OpLeadingZerosInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.LeadingZeros", opLen1(ssa.OpLeadingZerosInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.LeadingZeros", opLen1(ssa.OpLeadingZerosInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.LeadingZeros", opLen1(ssa.OpLeadingZerosUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.LeadingZeros", opLen1(ssa.OpLeadingZerosUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.LeadingZeros", opLen1(ssa.OpLeadingZerosUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.LookupOrZero", opLen2(ssa.OpLookupOrZeroInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.LookupOrZero", opLen2(ssa.OpLookupOrZeroUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Max", opLen2(ssa.OpMaxFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Max", opLen2(ssa.OpMaxFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Max", opLen2(ssa.OpMaxInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Max", opLen2(ssa.OpMaxInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Max", opLen2(ssa.OpMaxInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Max", opLen2(ssa.OpMaxUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Max", opLen2(ssa.OpMaxUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Max", opLen2(ssa.OpMaxUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Min", opLen2(ssa.OpMinFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Min", opLen2(ssa.OpMinFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Min", opLen2(ssa.OpMinInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Min", opLen2(ssa.OpMinInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Min", opLen2(ssa.OpMinInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Min", opLen2(ssa.OpMinUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Min", opLen2(ssa.OpMinUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Min", opLen2(ssa.OpMinUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Mul", opLen2(ssa.OpMulFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Mul", opLen2(ssa.OpMulFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Mul", opLen2(ssa.OpMulInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Mul", opLen2(ssa.OpMulInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Mul", opLen2(ssa.OpMulInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Mul", opLen2(ssa.OpMulUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Mul", opLen2(ssa.OpMulUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Mul", opLen2(ssa.OpMulUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.MulAdd", opLen3(ssa.OpMulAddFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.MulAdd", opLen3(ssa.OpMulAddFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.MulAdd", opLen3(ssa.OpMulAddInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.MulAdd", opLen3(ssa.OpMulAddInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.MulAdd", opLen3(ssa.OpMulAddInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.MulAdd", opLen3(ssa.OpMulAddUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.MulAdd", opLen3(ssa.OpMulAddUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.MulAdd", opLen3(ssa.OpMulAddUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.MulWidenLo", opLen2(ssa.OpMulWidenLoInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.MulWidenLo", opLen2(ssa.OpMulWidenLoInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.MulWidenLo", opLen2(ssa.OpMulWidenLoInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.MulWidenLo", opLen2(ssa.OpMulWidenLoUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.MulWidenLo", opLen2(ssa.OpMulWidenLoUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.MulWidenLo", opLen2(ssa.OpMulWidenLoUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Neg", opLen1(ssa.OpNegFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Neg", opLen1(ssa.OpNegFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Neg", opLen1(ssa.OpNegInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Neg", opLen1(ssa.OpNegInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Neg", opLen1(ssa.OpNegInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.Neg", opLen1(ssa.OpNegInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Not", opLen1(ssa.OpNotInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Not", opLen1(ssa.OpNotInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Not", opLen1(ssa.OpNotInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.Not", opLen1(ssa.OpNotInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Not", opLen1(ssa.OpNotUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Not", opLen1(ssa.OpNotUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Not", opLen1(ssa.OpNotUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.Not", opLen1(ssa.OpNotUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.OnesCount", opLen1(ssa.OpOnesCountInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.OnesCount", opLen1(ssa.OpOnesCountUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Or", opLen2(ssa.OpOrInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Or", opLen2(ssa.OpOrInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Or", opLen2(ssa.OpOrInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.Or", opLen2(ssa.OpOrInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Or", opLen2(ssa.OpOrUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Or", opLen2(ssa.OpOrUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Or", opLen2(ssa.OpOrUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.Or", opLen2(ssa.OpOrUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.OrNot", opLen2(ssa.OpOrNotInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.OrNot", opLen2(ssa.OpOrNotInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.OrNot", opLen2(ssa.OpOrNotInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.OrNot", opLen2(ssa.OpOrNotInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.OrNot", opLen2(ssa.OpOrNotUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.OrNot", opLen2(ssa.OpOrNotUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.OrNot", opLen2(ssa.OpOrNotUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.OrNot", opLen2(ssa.OpOrNotUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Round", opLen1(ssa.OpRoundFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Round", opLen1(ssa.OpRoundFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.SaturateToInt8", opLen1(ssa.OpSaturateToInt8Int16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.SaturateToInt16", opLen1(ssa.OpSaturateToInt16Int32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.SaturateToInt32", opLen1(ssa.OpSaturateToInt32Int64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.SaturateToUint8", opLen1(ssa.OpSaturateToUint8Int16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.SaturateToUint8", opLen1(ssa.OpSaturateToUint8Uint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.SaturateToUint16", opLen1(ssa.OpSaturateToUint16Int32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.SaturateToUint16", opLen1(ssa.OpSaturateToUint16Uint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.SaturateToUint32", opLen1(ssa.OpSaturateToUint32Int64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.SaturateToUint32", opLen1(ssa.OpSaturateToUint32Uint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.SetElem", opLen2Imm(ssa.OpSetElemInt8x16, types.TypeVec128, 0, 15), sys.ARM64)
	addF(simdPackage, "Int16x8.SetElem", opLen2Imm(ssa.OpSetElemInt16x8, types.TypeVec128, 0, 7), sys.ARM64)
	addF(simdPackage, "Int32x4.SetElem", opLen2Imm(ssa.OpSetElemInt32x4, types.TypeVec128, 0, 3), sys.ARM64)
	addF(simdPackage, "Int64x2.SetElem", opLen2Imm(ssa.OpSetElemInt64x2, types.TypeVec128, 0, 1), sys.ARM64)
	addF(simdPackage, "Uint8x16.SetElem", opLen2Imm(ssa.OpSetElemUint8x16, types.TypeVec128, 0, 15), sys.ARM64)
	addF(simdPackage, "Uint16x8.SetElem", opLen2Imm(ssa.OpSetElemUint16x8, types.TypeVec128, 0, 7), sys.ARM64)
	addF(simdPackage, "Uint32x4.SetElem", opLen2Imm(ssa.OpSetElemUint32x4, types.TypeVec128, 0, 3), sys.ARM64)
	addF(simdPackage, "Uint64x2.SetElem", opLen2Imm(ssa.OpSetElemUint64x2, types.TypeVec128, 0, 1), sys.ARM64)
	addF(simdPackage, "Float32x4.SetElem", opLen2Imm(ssa.OpSetElemFloat32x4, types.TypeVec128, 0, 3), sys.ARM64)
	addF(simdPackage, "Float64x2.SetElem", opLen2Imm(ssa.OpSetElemFloat64x2, types.TypeVec128, 0, 1), sys.ARM64)
	addF(simdPackage, "Int8x16.Shift", opLen2(ssa.OpShiftInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Shift", opLen2(ssa.OpShiftInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Shift", opLen2(ssa.OpShiftInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.Shift", opLen2(ssa.OpShiftInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Shift", opLen2(ssa.OpShiftUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Shift", opLen2(ssa.OpShiftUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Shift", opLen2(ssa.OpShiftUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.Shift", opLen2(ssa.OpShiftUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.ShiftAllLeft", opLen2(ssa.OpShiftAllLeftUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.ShiftAllRight", opLen2(ssa.OpShiftAllRightInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.ShiftAllRight", opLen2(ssa.OpShiftAllRightUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.ShiftSaturated", opLen2(ssa.OpShiftSaturatedInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.ShiftSaturated", opLen2(ssa.OpShiftSaturatedInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.ShiftSaturated", opLen2(ssa.OpShiftSaturatedInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.ShiftSaturated", opLen2(ssa.OpShiftSaturatedInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.ShiftSaturated", opLen2(ssa.OpShiftSaturatedUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.ShiftSaturated", opLen2(ssa.OpShiftSaturatedUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.ShiftSaturated", opLen2(ssa.OpShiftSaturatedUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.ShiftSaturated", opLen2(ssa.OpShiftSaturatedUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Sqrt", opLen1(ssa.OpSqrtFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Sqrt", opLen1(ssa.OpSqrtFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Sub", opLen2(ssa.OpSubFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Sub", opLen2(ssa.OpSubFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Sub", opLen2(ssa.OpSubInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Sub", opLen2(ssa.OpSubInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Sub", opLen2(ssa.OpSubInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.Sub", opLen2(ssa.OpSubInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Sub", opLen2(ssa.OpSubUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Sub", opLen2(ssa.OpSubUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Sub", opLen2(ssa.OpSubUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.Sub", opLen2(ssa.OpSubUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.SubSaturated", opLen2(ssa.OpSubSaturatedInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.SubSaturated", opLen2(ssa.OpSubSaturatedInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.SubSaturated", opLen2(ssa.OpSubSaturatedInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.SubSaturated", opLen2(ssa.OpSubSaturatedInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.SubSaturated", opLen2(ssa.OpSubSaturatedUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.SubSaturated", opLen2(ssa.OpSubSaturatedUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.SubSaturated", opLen2(ssa.OpSubSaturatedUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.SubSaturated", opLen2(ssa.OpSubSaturatedUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.Trunc", opLen1(ssa.OpTruncFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.Trunc", opLen1(ssa.OpTruncFloat64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.TruncToInt8", opLen1(ssa.OpTruncToInt8Int16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.TruncToInt16", opLen1(ssa.OpTruncToInt16Int32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.TruncToInt32", opLen1(ssa.OpTruncToInt32Int64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.TruncToUint8", opLen1(ssa.OpTruncToUint8Uint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.TruncToUint16", opLen1(ssa.OpTruncToUint16Uint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.TruncToUint32", opLen1(ssa.OpTruncToUint32Uint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.Xor", opLen2(ssa.OpXorInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.Xor", opLen2(ssa.OpXorInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.Xor", opLen2(ssa.OpXorInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.Xor", opLen2(ssa.OpXorInt64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.Xor", opLen2(ssa.OpXorUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.Xor", opLen2(ssa.OpXorUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.Xor", opLen2(ssa.OpXorUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.Xor", opLen2(ssa.OpXorUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.bitSelect", opLen3(ssa.OpbitSelectInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.bitSelectNot", opLen3(ssa.OpbitSelectNotInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float64x2.broadcast1To2", opLen1(ssa.Opbroadcast1To2Float64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int64x2.broadcast1To2", opLen1(ssa.Opbroadcast1To2Int64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.broadcast1To2", opLen1(ssa.Opbroadcast1To2Uint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.broadcast1To4", opLen1(ssa.Opbroadcast1To4Float32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.broadcast1To4", opLen1(ssa.Opbroadcast1To4Int32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.broadcast1To4", opLen1(ssa.Opbroadcast1To4Uint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.broadcast1To8", opLen1(ssa.Opbroadcast1To8Int16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.broadcast1To8", opLen1(ssa.Opbroadcast1To8Uint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.broadcast1To16", opLen1(ssa.Opbroadcast1To16Int8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.broadcast1To16", opLen1(ssa.Opbroadcast1To16Uint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint64x2.carrylessMultiplyWidenLo", opLen2(ssa.OpcarrylessMultiplyWidenLoUint64x2, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.reduceMax", opLen1(ssa.OpreduceMaxFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.reduceMax", opLen1(ssa.OpreduceMaxInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.reduceMax", opLen1(ssa.OpreduceMaxInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.reduceMax", opLen1(ssa.OpreduceMaxInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.reduceMax", opLen1(ssa.OpreduceMaxUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.reduceMax", opLen1(ssa.OpreduceMaxUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.reduceMax", opLen1(ssa.OpreduceMaxUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.reduceMin", opLen1(ssa.OpreduceMinFloat32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.reduceMin", opLen1(ssa.OpreduceMinInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.reduceMin", opLen1(ssa.OpreduceMinInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.reduceMin", opLen1(ssa.OpreduceMinInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.reduceMin", opLen1(ssa.OpreduceMinUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.reduceMin", opLen1(ssa.OpreduceMinUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.reduceMin", opLen1(ssa.OpreduceMinUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int8x16.reduceSum", opLen1(ssa.OpreduceSumInt8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int16x8.reduceSum", opLen1(ssa.OpreduceSumInt16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Int32x4.reduceSum", opLen1(ssa.OpreduceSumInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint8x16.reduceSum", opLen1(ssa.OpreduceSumUint8x16, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint16x8.reduceSum", opLen1(ssa.OpreduceSumUint16x8, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Uint32x4.reduceSum", opLen1(ssa.OpreduceSumUint32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Float32x4.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float32x4.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float32x4.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float32x4.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float32x4.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float32x4.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float32x4.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float32x4.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float32x4.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float64x2.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float64x2.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float64x2.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float64x2.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float64x2.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float64x2.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float64x2.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float64x2.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float64x2.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.BitsToInt8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.ConvertToInt8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.ConvertToUint8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint8x16.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.BitsToInt16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.ConvertToInt16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.ConvertToUint16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint16x8.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.BitsToFloat32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float32x4.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.BitsToInt32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.ConvertToInt32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.ConvertToUint32", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.AsUint64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint32x4.ReshapeToUint64s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.AsFloat32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.AsFloat64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.BitsToFloat64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Float64x2.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.AsInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.AsInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.AsInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.AsInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.BitsToInt64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.ConvertToInt64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.ConvertToUint64", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.ToBits", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.AsUint8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.ReshapeToUint8s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.AsUint16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.ReshapeToUint16s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.AsUint32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Uint64x2.ReshapeToUint32s", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "LoadFloat32x4Array", simdLoad(), sys.ARM64)
	addF(simdPackage, "Float32x4.StoreArray", simdStore(), sys.ARM64)
	addF(simdPackage, "LoadFloat64x2Array", simdLoad(), sys.ARM64)
	addF(simdPackage, "Float64x2.StoreArray", simdStore(), sys.ARM64)
	addF(simdPackage, "LoadInt8x16Array", simdLoad(), sys.ARM64)
	addF(simdPackage, "Int8x16.StoreArray", simdStore(), sys.ARM64)
	addF(simdPackage, "LoadInt16x8Array", simdLoad(), sys.ARM64)
	addF(simdPackage, "Int16x8.StoreArray", simdStore(), sys.ARM64)
	addF(simdPackage, "LoadInt32x4Array", simdLoad(), sys.ARM64)
	addF(simdPackage, "Int32x4.StoreArray", simdStore(), sys.ARM64)
	addF(simdPackage, "LoadInt64x2Array", simdLoad(), sys.ARM64)
	addF(simdPackage, "Int64x2.StoreArray", simdStore(), sys.ARM64)
	addF(simdPackage, "LoadUint8x16Array", simdLoad(), sys.ARM64)
	addF(simdPackage, "Uint8x16.StoreArray", simdStore(), sys.ARM64)
	addF(simdPackage, "LoadUint16x8Array", simdLoad(), sys.ARM64)
	addF(simdPackage, "Uint16x8.StoreArray", simdStore(), sys.ARM64)
	addF(simdPackage, "LoadUint32x4Array", simdLoad(), sys.ARM64)
	addF(simdPackage, "Uint32x4.StoreArray", simdStore(), sys.ARM64)
	addF(simdPackage, "LoadUint64x2Array", simdLoad(), sys.ARM64)
	addF(simdPackage, "Uint64x2.StoreArray", simdStore(), sys.ARM64)
	addF(simdPackage, "Mask8x16.ToInt8x16", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int8x16.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Mask8x16.And", opLen2(ssa.OpAndInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Mask8x16.Or", opLen2(ssa.OpOrInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Mask8x16.Not", opLen1(ssa.OpNotInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Mask16x8.ToInt16x8", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int16x8.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Mask16x8.And", opLen2(ssa.OpAndInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Mask16x8.Or", opLen2(ssa.OpOrInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Mask16x8.Not", opLen1(ssa.OpNotInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Mask32x4.ToInt32x4", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int32x4.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Mask32x4.And", opLen2(ssa.OpAndInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Mask32x4.Or", opLen2(ssa.OpOrInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Mask32x4.Not", opLen1(ssa.OpNotInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Mask64x2.ToInt64x2", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Int64x2.asMask", func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value { return args[0] }, sys.ARM64)
	addF(simdPackage, "Mask64x2.And", opLen2(ssa.OpAndInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Mask64x2.Or", opLen2(ssa.OpOrInt32x4, types.TypeVec128), sys.ARM64)
	addF(simdPackage, "Mask64x2.Not", opLen1(ssa.OpNotInt32x4, types.TypeVec128), sys.ARM64)
}

```

// === FILE: references/go/src/cmd/compile/internal/ssagen/simdWasmintrinsics.go ===
```go
// Code generated by 'wasmgen'; DO NOT EDIT.

package ssagen

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/types"
	"cmd/internal/sys"
)

func initWasmSIMD() {
	makeSimdOp1 := func(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue1(op, types.TypeVec128, args[0])
		}
	}
	makeSimdOp2 := func(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue2(op, types.TypeVec128, args[0], args[1])
		}
	}
	makeSimdOp3 := func(op ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue3(op, types.TypeVec128, args[0], args[1], args[2])
		}
	}

	// "As" is a type pun, just return the bits
	makeAsOp := func() func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return args[0]
		}
	}

	// converting to a mask is an not-equals comparison with zero, zero obtained by x XOR x.
	makeToMask := func(op, xor ssa.Op) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			return s.newValue2(op, types.TypeVec128, args[0], s.newValue2(xor, n.Type(), args[0], args[0]))
		}
	}

	makeSimdOp1Imm8 := func(op ssa.Op, immLimit uint64) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			t := n.Type()
			if args[1].Op == ssa.OpConst8 {
				return s.newValue1I(op, t, args[1].AuxInt, args[0])
			}
			return immJumpTableN(s, args[1], n, immLimit, func(sNew *state, idx int) {
				// Encode as int8 due to requirement of AuxInt, check its comment for details.
				s.vars[n] = sNew.newValue1I(op, t, int64(int8(idx)), args[0])
			})
		}
	}

	makeSimdOp2Imm8 := func(op ssa.Op, immLimit uint64) func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
		return func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value {
			t := types.TypeVec128
			if args[1].Op == ssa.OpConst8 {
				return s.newValue2I(op, t, args[1].AuxInt, args[0], args[2])
			}
			return immJumpTableN(s, args[1], n, immLimit, func(sNew *state, idx int) {
				// Encode as int8 due to requirement of AuxInt, check its comment for details.
				s.vars[n] = sNew.newValue2I(op, t, int64(int8(idx)), args[0], args[2])
			})
		}
	}

	addWasmSIMD := func(pkg, fn string, builder func(s *state, n *ir.CallExpr, args []*ssa.Value) *ssa.Value) {
		intrinsics.add(sys.ArchWasm, pkg, fn, builder)
	}

	addWasmSIMD("simd/archsimd", "Int8x16.Abs", makeSimdOp1(ssa.OpAbsInt8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Abs", makeSimdOp1(ssa.OpAbsInt16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Abs", makeSimdOp1(ssa.OpAbsInt32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.Abs", makeSimdOp1(ssa.OpAbsFloat32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.Abs", makeSimdOp1(ssa.OpAbsInt64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.Abs", makeSimdOp1(ssa.OpAbsFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.Add", makeSimdOp2(ssa.OpAddInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.Add", makeSimdOp2(ssa.OpAddInt8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Add", makeSimdOp2(ssa.OpAddInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.Add", makeSimdOp2(ssa.OpAddInt16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Add", makeSimdOp2(ssa.OpAddInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.Add", makeSimdOp2(ssa.OpAddInt32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.Add", makeSimdOp2(ssa.OpAddFloat32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.Add", makeSimdOp2(ssa.OpAddInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.Add", makeSimdOp2(ssa.OpAddInt64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.Add", makeSimdOp2(ssa.OpAddFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.AddSaturated", makeSimdOp2(ssa.OpAddSaturatedInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.AddSaturated", makeSimdOp2(ssa.OpAddSaturatedUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.AddSaturated", makeSimdOp2(ssa.OpAddSaturatedInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.AddSaturated", makeSimdOp2(ssa.OpAddSaturatedUint16x8))
	addWasmSIMD("simd/archsimd", "Int8x16.And", makeSimdOp2(ssa.OpAndInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.And", makeSimdOp2(ssa.OpAndUint8x16))
	addWasmSIMD("simd/archsimd", "Mask8x16.And", makeSimdOp2(ssa.OpAndInt8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.And", makeSimdOp2(ssa.OpAndInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.And", makeSimdOp2(ssa.OpAndUint16x8))
	addWasmSIMD("simd/archsimd", "Mask16x8.And", makeSimdOp2(ssa.OpAndInt16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.And", makeSimdOp2(ssa.OpAndInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.And", makeSimdOp2(ssa.OpAndUint32x4))
	addWasmSIMD("simd/archsimd", "Mask32x4.And", makeSimdOp2(ssa.OpAndInt32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.And", makeSimdOp2(ssa.OpAndInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.And", makeSimdOp2(ssa.OpAndUint64x2))
	addWasmSIMD("simd/archsimd", "Mask64x2.And", makeSimdOp2(ssa.OpAndInt64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.AndNot", makeSimdOp2(ssa.OpAndNotInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.AndNot", makeSimdOp2(ssa.OpAndNotUint8x16))
	addWasmSIMD("simd/archsimd", "Mask8x16.AndNot", makeSimdOp2(ssa.OpAndNotInt8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.AndNot", makeSimdOp2(ssa.OpAndNotInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.AndNot", makeSimdOp2(ssa.OpAndNotUint16x8))
	addWasmSIMD("simd/archsimd", "Mask16x8.AndNot", makeSimdOp2(ssa.OpAndNotInt16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.AndNot", makeSimdOp2(ssa.OpAndNotInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.AndNot", makeSimdOp2(ssa.OpAndNotUint32x4))
	addWasmSIMD("simd/archsimd", "Mask32x4.AndNot", makeSimdOp2(ssa.OpAndNotInt32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.AndNot", makeSimdOp2(ssa.OpAndNotInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.AndNot", makeSimdOp2(ssa.OpAndNotUint64x2))
	addWasmSIMD("simd/archsimd", "Mask64x2.AndNot", makeSimdOp2(ssa.OpAndNotInt64x2))
	addWasmSIMD("simd/archsimd", "Uint8x16.Average", makeSimdOp2(ssa.OpAverageUint8x16))
	addWasmSIMD("simd/archsimd", "Uint16x8.Average", makeSimdOp2(ssa.OpAverageUint16x8))
	addWasmSIMD("simd/archsimd", "Int8x16.BitSelect", makeSimdOp3(ssa.OpBitSelectInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.BitSelect", makeSimdOp3(ssa.OpBitSelectUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.BitSelect", makeSimdOp3(ssa.OpBitSelectInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.BitSelect", makeSimdOp3(ssa.OpBitSelectUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.BitSelect", makeSimdOp3(ssa.OpBitSelectInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.BitSelect", makeSimdOp3(ssa.OpBitSelectUint32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.BitSelect", makeSimdOp3(ssa.OpBitSelectInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.BitSelect", makeSimdOp3(ssa.OpBitSelectUint64x2))
	addWasmSIMD("simd/archsimd", "Float32x4.Ceil", makeSimdOp1(ssa.OpCeilFloat32x4))
	addWasmSIMD("simd/archsimd", "Float64x2.Ceil", makeSimdOp1(ssa.OpCeilFloat64x2))
	addWasmSIMD("simd/archsimd", "Int32x4.ConvertLo2ToFloat64", makeSimdOp1(ssa.OpConvertLo2ToFloat64Int32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.ConvertLo2ToFloat64", makeSimdOp1(ssa.OpConvertLo2ToFloat64Uint32x4))
	addWasmSIMD("simd/archsimd", "Int32x4.ConvertToFloat32", makeSimdOp1(ssa.OpConvertToFloat32Int32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.ConvertToFloat32", makeSimdOp1(ssa.OpConvertToFloat32Uint32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.ConvertToInt32", makeSimdOp1(ssa.OpConvertToInt32Float32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.ConvertToUint32", makeSimdOp1(ssa.OpConvertToUint32Float32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.Div", makeSimdOp2(ssa.OpDivFloat32x4))
	addWasmSIMD("simd/archsimd", "Float64x2.Div", makeSimdOp2(ssa.OpDivFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.Equal", makeSimdOp2(ssa.OpEqualInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.Equal", makeSimdOp2(ssa.OpEqualUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Equal", makeSimdOp2(ssa.OpEqualInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.Equal", makeSimdOp2(ssa.OpEqualUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Equal", makeSimdOp2(ssa.OpEqualInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.Equal", makeSimdOp2(ssa.OpEqualUint32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.Equal", makeSimdOp2(ssa.OpEqualFloat32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.Equal", makeSimdOp2(ssa.OpEqualInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.Equal", makeSimdOp2(ssa.OpEqualUint64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.Equal", makeSimdOp2(ssa.OpEqualFloat64x2))
	addWasmSIMD("simd/archsimd", "Int32x4.ExtendHi2ToInt64", makeSimdOp1(ssa.OpExtendHi2ToInt64Int32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.ExtendHi2ToUint64", makeSimdOp1(ssa.OpExtendHi2ToUint64Uint32x4))
	addWasmSIMD("simd/archsimd", "Int16x8.ExtendHi4ToInt32", makeSimdOp1(ssa.OpExtendHi4ToInt32Int16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.ExtendHi4ToUint32", makeSimdOp1(ssa.OpExtendHi4ToUint32Uint16x8))
	addWasmSIMD("simd/archsimd", "Int8x16.ExtendHi8ToInt16", makeSimdOp1(ssa.OpExtendHi8ToInt16Int8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.ExtendHi8ToUint16", makeSimdOp1(ssa.OpExtendHi8ToUint16Uint8x16))
	addWasmSIMD("simd/archsimd", "Int32x4.ExtendLo2ToInt64", makeSimdOp1(ssa.OpExtendLo2ToInt64Int32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.ExtendLo2ToUint64", makeSimdOp1(ssa.OpExtendLo2ToUint64Uint32x4))
	addWasmSIMD("simd/archsimd", "Int16x8.ExtendLo4ToInt32", makeSimdOp1(ssa.OpExtendLo4ToInt32Int16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.ExtendLo4ToUint32", makeSimdOp1(ssa.OpExtendLo4ToUint32Uint16x8))
	addWasmSIMD("simd/archsimd", "Int8x16.ExtendLo8ToInt16", makeSimdOp1(ssa.OpExtendLo8ToInt16Int8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.ExtendLo8ToUint16", makeSimdOp1(ssa.OpExtendLo8ToUint16Uint8x16))
	addWasmSIMD("simd/archsimd", "Float32x4.Floor", makeSimdOp1(ssa.OpFloorFloat32x4))
	addWasmSIMD("simd/archsimd", "Float64x2.Floor", makeSimdOp1(ssa.OpFloorFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.GetElem", makeSimdOp1Imm8(ssa.OpGetElemInt8x16, 16))
	addWasmSIMD("simd/archsimd", "Uint8x16.GetElem", makeSimdOp1Imm8(ssa.OpGetElemUint8x16, 16))
	addWasmSIMD("simd/archsimd", "Int16x8.GetElem", makeSimdOp1Imm8(ssa.OpGetElemInt16x8, 8))
	addWasmSIMD("simd/archsimd", "Uint16x8.GetElem", makeSimdOp1Imm8(ssa.OpGetElemUint16x8, 8))
	addWasmSIMD("simd/archsimd", "Int32x4.GetElem", makeSimdOp1Imm8(ssa.OpGetElemInt32x4, 4))
	addWasmSIMD("simd/archsimd", "Uint32x4.GetElem", makeSimdOp1Imm8(ssa.OpGetElemUint32x4, 4))
	addWasmSIMD("simd/archsimd", "Float32x4.GetElem", makeSimdOp1Imm8(ssa.OpGetElemFloat32x4, 4))
	addWasmSIMD("simd/archsimd", "Int64x2.GetElem", makeSimdOp1Imm8(ssa.OpGetElemInt64x2, 2))
	addWasmSIMD("simd/archsimd", "Uint64x2.GetElem", makeSimdOp1Imm8(ssa.OpGetElemUint64x2, 2))
	addWasmSIMD("simd/archsimd", "Float64x2.GetElem", makeSimdOp1Imm8(ssa.OpGetElemFloat64x2, 2))
	addWasmSIMD("simd/archsimd", "Int8x16.Greater", makeSimdOp2(ssa.OpGreaterInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.Greater", makeSimdOp2(ssa.OpGreaterUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Greater", makeSimdOp2(ssa.OpGreaterInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.Greater", makeSimdOp2(ssa.OpGreaterUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Greater", makeSimdOp2(ssa.OpGreaterInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.Greater", makeSimdOp2(ssa.OpGreaterUint32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.Greater", makeSimdOp2(ssa.OpGreaterFloat32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.Greater", makeSimdOp2(ssa.OpGreaterInt64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.Greater", makeSimdOp2(ssa.OpGreaterFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.GreaterEqual", makeSimdOp2(ssa.OpGreaterEqualInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.GreaterEqual", makeSimdOp2(ssa.OpGreaterEqualUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.GreaterEqual", makeSimdOp2(ssa.OpGreaterEqualInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.GreaterEqual", makeSimdOp2(ssa.OpGreaterEqualUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.GreaterEqual", makeSimdOp2(ssa.OpGreaterEqualInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.GreaterEqual", makeSimdOp2(ssa.OpGreaterEqualUint32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.GreaterEqual", makeSimdOp2(ssa.OpGreaterEqualFloat32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.GreaterEqual", makeSimdOp2(ssa.OpGreaterEqualInt64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.GreaterEqual", makeSimdOp2(ssa.OpGreaterEqualFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.Less", makeSimdOp2(ssa.OpLessInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.Less", makeSimdOp2(ssa.OpLessUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Less", makeSimdOp2(ssa.OpLessInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.Less", makeSimdOp2(ssa.OpLessUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Less", makeSimdOp2(ssa.OpLessInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.Less", makeSimdOp2(ssa.OpLessUint32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.Less", makeSimdOp2(ssa.OpLessFloat32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.Less", makeSimdOp2(ssa.OpLessInt64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.Less", makeSimdOp2(ssa.OpLessFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.LessEqual", makeSimdOp2(ssa.OpLessEqualInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.LessEqual", makeSimdOp2(ssa.OpLessEqualUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.LessEqual", makeSimdOp2(ssa.OpLessEqualInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.LessEqual", makeSimdOp2(ssa.OpLessEqualUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.LessEqual", makeSimdOp2(ssa.OpLessEqualInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.LessEqual", makeSimdOp2(ssa.OpLessEqualUint32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.LessEqual", makeSimdOp2(ssa.OpLessEqualFloat32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.LessEqual", makeSimdOp2(ssa.OpLessEqualInt64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.LessEqual", makeSimdOp2(ssa.OpLessEqualFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.LookupOrZero", makeSimdOp2(ssa.OpLookupOrZeroInt8x16))
	addWasmSIMD("simd/archsimd", "Int8x16.Max", makeSimdOp2(ssa.OpMaxInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.Max", makeSimdOp2(ssa.OpMaxUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Max", makeSimdOp2(ssa.OpMaxInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.Max", makeSimdOp2(ssa.OpMaxUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Max", makeSimdOp2(ssa.OpMaxInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.Max", makeSimdOp2(ssa.OpMaxUint32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.Max", makeSimdOp2(ssa.OpMaxFloat32x4))
	addWasmSIMD("simd/archsimd", "Float64x2.Max", makeSimdOp2(ssa.OpMaxFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.Min", makeSimdOp2(ssa.OpMinInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.Min", makeSimdOp2(ssa.OpMinUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Min", makeSimdOp2(ssa.OpMinInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.Min", makeSimdOp2(ssa.OpMinUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Min", makeSimdOp2(ssa.OpMinInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.Min", makeSimdOp2(ssa.OpMinUint32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.Min", makeSimdOp2(ssa.OpMinFloat32x4))
	addWasmSIMD("simd/archsimd", "Float64x2.Min", makeSimdOp2(ssa.OpMinFloat64x2))
	addWasmSIMD("simd/archsimd", "Int16x8.Mul", makeSimdOp2(ssa.OpMulInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.Mul", makeSimdOp2(ssa.OpMulUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Mul", makeSimdOp2(ssa.OpMulInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.Mul", makeSimdOp2(ssa.OpMulUint32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.Mul", makeSimdOp2(ssa.OpMulFloat32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.Mul", makeSimdOp2(ssa.OpMulInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.Mul", makeSimdOp2(ssa.OpMulUint64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.Mul", makeSimdOp2(ssa.OpMulFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.MulWidenHi", makeSimdOp2(ssa.OpMulWidenHiInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.MulWidenHi", makeSimdOp2(ssa.OpMulWidenHiUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.MulWidenHi", makeSimdOp2(ssa.OpMulWidenHiInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.MulWidenHi", makeSimdOp2(ssa.OpMulWidenHiUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.MulWidenHi", makeSimdOp2(ssa.OpMulWidenHiInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.MulWidenHi", makeSimdOp2(ssa.OpMulWidenHiUint32x4))
	addWasmSIMD("simd/archsimd", "Int8x16.MulWidenLo", makeSimdOp2(ssa.OpMulWidenLoInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.MulWidenLo", makeSimdOp2(ssa.OpMulWidenLoUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.MulWidenLo", makeSimdOp2(ssa.OpMulWidenLoInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.MulWidenLo", makeSimdOp2(ssa.OpMulWidenLoUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.MulWidenLo", makeSimdOp2(ssa.OpMulWidenLoInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.MulWidenLo", makeSimdOp2(ssa.OpMulWidenLoUint32x4))
	addWasmSIMD("simd/archsimd", "Int8x16.Neg", makeSimdOp1(ssa.OpNegInt8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Neg", makeSimdOp1(ssa.OpNegInt16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Neg", makeSimdOp1(ssa.OpNegInt32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.Neg", makeSimdOp1(ssa.OpNegFloat32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.Neg", makeSimdOp1(ssa.OpNegInt64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.Neg", makeSimdOp1(ssa.OpNegFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.Not", makeSimdOp1(ssa.OpNotInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.Not", makeSimdOp1(ssa.OpNotUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Not", makeSimdOp1(ssa.OpNotInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.Not", makeSimdOp1(ssa.OpNotUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Not", makeSimdOp1(ssa.OpNotInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.Not", makeSimdOp1(ssa.OpNotUint32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.Not", makeSimdOp1(ssa.OpNotInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.Not", makeSimdOp1(ssa.OpNotUint64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.NotEqual", makeSimdOp2(ssa.OpNotEqualInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.NotEqual", makeSimdOp2(ssa.OpNotEqualUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.NotEqual", makeSimdOp2(ssa.OpNotEqualInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.NotEqual", makeSimdOp2(ssa.OpNotEqualUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.NotEqual", makeSimdOp2(ssa.OpNotEqualInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.NotEqual", makeSimdOp2(ssa.OpNotEqualUint32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.NotEqual", makeSimdOp2(ssa.OpNotEqualFloat32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.NotEqual", makeSimdOp2(ssa.OpNotEqualInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.NotEqual", makeSimdOp2(ssa.OpNotEqualUint64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.NotEqual", makeSimdOp2(ssa.OpNotEqualFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.OnesCount", makeSimdOp1(ssa.OpOnesCountInt8x16))
	addWasmSIMD("simd/archsimd", "Int8x16.Or", makeSimdOp2(ssa.OpOrInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.Or", makeSimdOp2(ssa.OpOrUint8x16))
	addWasmSIMD("simd/archsimd", "Mask8x16.Or", makeSimdOp2(ssa.OpOrInt8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Or", makeSimdOp2(ssa.OpOrInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.Or", makeSimdOp2(ssa.OpOrUint16x8))
	addWasmSIMD("simd/archsimd", "Mask16x8.Or", makeSimdOp2(ssa.OpOrInt16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Or", makeSimdOp2(ssa.OpOrInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.Or", makeSimdOp2(ssa.OpOrUint32x4))
	addWasmSIMD("simd/archsimd", "Mask32x4.Or", makeSimdOp2(ssa.OpOrInt32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.Or", makeSimdOp2(ssa.OpOrInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.Or", makeSimdOp2(ssa.OpOrUint64x2))
	addWasmSIMD("simd/archsimd", "Mask64x2.Or", makeSimdOp2(ssa.OpOrInt64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.RotateAllLeft", makeSimdOp2(ssa.OpRotateAllLeftVarInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.RotateAllLeft", makeSimdOp2(ssa.OpRotateAllLeftVarUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.RotateAllLeft", makeSimdOp2(ssa.OpRotateAllLeftVarInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.RotateAllLeft", makeSimdOp2(ssa.OpRotateAllLeftVarUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.RotateAllLeft", makeSimdOp2(ssa.OpRotateAllLeftVarInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.RotateAllLeft", makeSimdOp2(ssa.OpRotateAllLeftVarUint32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.RotateAllLeft", makeSimdOp2(ssa.OpRotateAllLeftVarInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.RotateAllLeft", makeSimdOp2(ssa.OpRotateAllLeftVarUint64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.RotateAllRight", makeSimdOp2(ssa.OpRotateAllRightVarInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.RotateAllRight", makeSimdOp2(ssa.OpRotateAllRightVarUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.RotateAllRight", makeSimdOp2(ssa.OpRotateAllRightVarInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.RotateAllRight", makeSimdOp2(ssa.OpRotateAllRightVarUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.RotateAllRight", makeSimdOp2(ssa.OpRotateAllRightVarInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.RotateAllRight", makeSimdOp2(ssa.OpRotateAllRightVarUint32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.RotateAllRight", makeSimdOp2(ssa.OpRotateAllRightVarInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.RotateAllRight", makeSimdOp2(ssa.OpRotateAllRightVarUint64x2))
	addWasmSIMD("simd/archsimd", "Float32x4.Round", makeSimdOp1(ssa.OpRoundFloat32x4))
	addWasmSIMD("simd/archsimd", "Float64x2.Round", makeSimdOp1(ssa.OpRoundFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.SetElem", makeSimdOp2Imm8(ssa.OpSetElemInt8x16, 16))
	addWasmSIMD("simd/archsimd", "Uint8x16.SetElem", makeSimdOp2Imm8(ssa.OpSetElemUint8x16, 16))
	addWasmSIMD("simd/archsimd", "Int16x8.SetElem", makeSimdOp2Imm8(ssa.OpSetElemInt16x8, 8))
	addWasmSIMD("simd/archsimd", "Uint16x8.SetElem", makeSimdOp2Imm8(ssa.OpSetElemUint16x8, 8))
	addWasmSIMD("simd/archsimd", "Int32x4.SetElem", makeSimdOp2Imm8(ssa.OpSetElemInt32x4, 4))
	addWasmSIMD("simd/archsimd", "Uint32x4.SetElem", makeSimdOp2Imm8(ssa.OpSetElemUint32x4, 4))
	addWasmSIMD("simd/archsimd", "Float32x4.SetElem", makeSimdOp2Imm8(ssa.OpSetElemFloat32x4, 4))
	addWasmSIMD("simd/archsimd", "Int64x2.SetElem", makeSimdOp2Imm8(ssa.OpSetElemInt64x2, 2))
	addWasmSIMD("simd/archsimd", "Uint64x2.SetElem", makeSimdOp2Imm8(ssa.OpSetElemUint64x2, 2))
	addWasmSIMD("simd/archsimd", "Float64x2.SetElem", makeSimdOp2Imm8(ssa.OpSetElemFloat64x2, 2))
	addWasmSIMD("simd/archsimd", "Int8x16.ShiftAllLeft", makeSimdOp2(ssa.OpShiftAllLeftInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.ShiftAllLeft", makeSimdOp2(ssa.OpShiftAllLeftUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.ShiftAllLeft", makeSimdOp2(ssa.OpShiftAllLeftInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.ShiftAllLeft", makeSimdOp2(ssa.OpShiftAllLeftUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.ShiftAllLeft", makeSimdOp2(ssa.OpShiftAllLeftInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.ShiftAllLeft", makeSimdOp2(ssa.OpShiftAllLeftUint32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.ShiftAllLeft", makeSimdOp2(ssa.OpShiftAllLeftInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.ShiftAllLeft", makeSimdOp2(ssa.OpShiftAllLeftUint64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.ShiftAllRight", makeSimdOp2(ssa.OpShiftAllRightInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.ShiftAllRight", makeSimdOp2(ssa.OpShiftAllRightUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.ShiftAllRight", makeSimdOp2(ssa.OpShiftAllRightInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.ShiftAllRight", makeSimdOp2(ssa.OpShiftAllRightUint16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.ShiftAllRight", makeSimdOp2(ssa.OpShiftAllRightInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.ShiftAllRight", makeSimdOp2(ssa.OpShiftAllRightUint32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.ShiftAllRight", makeSimdOp2(ssa.OpShiftAllRightInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.ShiftAllRight", makeSimdOp2(ssa.OpShiftAllRightUint64x2))
	addWasmSIMD("simd/archsimd", "Float32x4.Sqrt", makeSimdOp1(ssa.OpSqrtFloat32x4))
	addWasmSIMD("simd/archsimd", "Float64x2.Sqrt", makeSimdOp1(ssa.OpSqrtFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.Sub", makeSimdOp2(ssa.OpSubInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.Sub", makeSimdOp2(ssa.OpSubInt8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Sub", makeSimdOp2(ssa.OpSubInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.Sub", makeSimdOp2(ssa.OpSubInt16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Sub", makeSimdOp2(ssa.OpSubInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.Sub", makeSimdOp2(ssa.OpSubInt32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.Sub", makeSimdOp2(ssa.OpSubFloat32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.Sub", makeSimdOp2(ssa.OpSubInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.Sub", makeSimdOp2(ssa.OpSubInt64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.Sub", makeSimdOp2(ssa.OpSubFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.SubSaturated", makeSimdOp2(ssa.OpSubSaturatedInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.SubSaturated", makeSimdOp2(ssa.OpSubSaturatedUint8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.SubSaturated", makeSimdOp2(ssa.OpSubSaturatedInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.SubSaturated", makeSimdOp2(ssa.OpSubSaturatedUint16x8))
	addWasmSIMD("simd/archsimd", "Float32x4.Trunc", makeSimdOp1(ssa.OpTruncFloat32x4))
	addWasmSIMD("simd/archsimd", "Float64x2.Trunc", makeSimdOp1(ssa.OpTruncFloat64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.Xor", makeSimdOp2(ssa.OpXorInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.Xor", makeSimdOp2(ssa.OpXorUint8x16))
	addWasmSIMD("simd/archsimd", "Mask8x16.Xor", makeSimdOp2(ssa.OpXorInt8x16))
	addWasmSIMD("simd/archsimd", "Int16x8.Xor", makeSimdOp2(ssa.OpXorInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.Xor", makeSimdOp2(ssa.OpXorUint16x8))
	addWasmSIMD("simd/archsimd", "Mask16x8.Xor", makeSimdOp2(ssa.OpXorInt16x8))
	addWasmSIMD("simd/archsimd", "Int32x4.Xor", makeSimdOp2(ssa.OpXorInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.Xor", makeSimdOp2(ssa.OpXorUint32x4))
	addWasmSIMD("simd/archsimd", "Mask32x4.Xor", makeSimdOp2(ssa.OpXorInt32x4))
	addWasmSIMD("simd/archsimd", "Int64x2.Xor", makeSimdOp2(ssa.OpXorInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.Xor", makeSimdOp2(ssa.OpXorUint64x2))
	addWasmSIMD("simd/archsimd", "Mask64x2.Xor", makeSimdOp2(ssa.OpXorInt64x2))
	addWasmSIMD("simd/archsimd", "Int8x16.ToMask", makeToMask(ssa.OpNotEqualInt8x16, ssa.OpXorInt8x16))
	addWasmSIMD("simd/archsimd", "Mask8x16.ToInt8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.ToMask", makeToMask(ssa.OpNotEqualInt16x8, ssa.OpXorInt16x8))
	addWasmSIMD("simd/archsimd", "Mask16x8.ToInt16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.ToMask", makeToMask(ssa.OpNotEqualInt32x4, ssa.OpXorInt32x4))
	addWasmSIMD("simd/archsimd", "Mask32x4.ToInt32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.ToMask", makeToMask(ssa.OpNotEqualInt64x2, ssa.OpXorInt64x2))
	addWasmSIMD("simd/archsimd", "Mask64x2.ToInt64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "LoadInt8x16Array", simdLoad())
	addWasmSIMD("simd/archsimd", "BroadcastInt8x16", simdBroadcast(ssa.OpBroadcastInt8x16))
	addWasmSIMD("simd/archsimd", "Int8x16.StoreArray", simdStore())
	addWasmSIMD("simd/archsimd", "LoadInt16x8Array", simdLoad())
	addWasmSIMD("simd/archsimd", "BroadcastInt16x8", simdBroadcast(ssa.OpBroadcastInt16x8))
	addWasmSIMD("simd/archsimd", "Int16x8.StoreArray", simdStore())
	addWasmSIMD("simd/archsimd", "LoadInt32x4Array", simdLoad())
	addWasmSIMD("simd/archsimd", "BroadcastInt32x4", simdBroadcast(ssa.OpBroadcastInt32x4))
	addWasmSIMD("simd/archsimd", "Int32x4.StoreArray", simdStore())
	addWasmSIMD("simd/archsimd", "LoadInt64x2Array", simdLoad())
	addWasmSIMD("simd/archsimd", "BroadcastInt64x2", simdBroadcast(ssa.OpBroadcastInt64x2))
	addWasmSIMD("simd/archsimd", "Int64x2.StoreArray", simdStore())
	addWasmSIMD("simd/archsimd", "LoadUint8x16Array", simdLoad())
	addWasmSIMD("simd/archsimd", "BroadcastUint8x16", simdBroadcast(ssa.OpBroadcastInt8x16))
	addWasmSIMD("simd/archsimd", "Uint8x16.StoreArray", simdStore())
	addWasmSIMD("simd/archsimd", "LoadUint16x8Array", simdLoad())
	addWasmSIMD("simd/archsimd", "BroadcastUint16x8", simdBroadcast(ssa.OpBroadcastInt16x8))
	addWasmSIMD("simd/archsimd", "Uint16x8.StoreArray", simdStore())
	addWasmSIMD("simd/archsimd", "LoadUint32x4Array", simdLoad())
	addWasmSIMD("simd/archsimd", "BroadcastUint32x4", simdBroadcast(ssa.OpBroadcastInt32x4))
	addWasmSIMD("simd/archsimd", "Uint32x4.StoreArray", simdStore())
	addWasmSIMD("simd/archsimd", "LoadUint64x2Array", simdLoad())
	addWasmSIMD("simd/archsimd", "BroadcastUint64x2", simdBroadcast(ssa.OpBroadcastInt64x2))
	addWasmSIMD("simd/archsimd", "Uint64x2.StoreArray", simdStore())
	addWasmSIMD("simd/archsimd", "LoadFloat32x4Array", simdLoad())
	addWasmSIMD("simd/archsimd", "BroadcastFloat32x4", simdBroadcast(ssa.OpBroadcastFloat32x4))
	addWasmSIMD("simd/archsimd", "Float32x4.StoreArray", simdStore())
	addWasmSIMD("simd/archsimd", "LoadFloat64x2Array", simdLoad())
	addWasmSIMD("simd/archsimd", "BroadcastFloat64x2", simdBroadcast(ssa.OpBroadcastFloat64x2))
	addWasmSIMD("simd/archsimd", "Float64x2.StoreArray", simdStore())
	addWasmSIMD("simd/archsimd", "Int8x16.AsInt16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int8x16.AsInt32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int8x16.AsInt64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int8x16.AsUint8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int8x16.AsUint16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int8x16.AsUint32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int8x16.AsUint64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int8x16.AsFloat32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int8x16.AsFloat64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.AsInt8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.AsInt32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.AsInt64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.AsUint8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.AsUint16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.AsUint32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.AsUint64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.AsFloat32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.AsFloat64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.AsInt8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.AsInt16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.AsInt64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.AsUint8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.AsUint16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.AsUint32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.AsUint64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.AsFloat32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.AsFloat64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.AsInt8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.AsInt16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.AsInt32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.AsUint8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.AsUint16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.AsUint32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.AsUint64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.AsFloat32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.AsFloat64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.AsInt8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.AsInt16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.AsInt32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.AsInt64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.AsUint16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.AsUint32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.AsUint64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.AsFloat32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.AsFloat64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.AsInt8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.AsInt16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.AsInt32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.AsInt64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.AsUint8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.AsUint32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.AsUint64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.AsFloat32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.AsFloat64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.AsInt8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.AsInt16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.AsInt32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.AsInt64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.AsUint8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.AsUint16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.AsUint64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.AsFloat32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.AsFloat64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.AsInt8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.AsInt16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.AsInt32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.AsInt64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.AsUint8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.AsUint16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.AsUint32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.AsFloat32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.AsFloat64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float32x4.AsInt8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float32x4.AsInt16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float32x4.AsInt32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float32x4.AsInt64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float32x4.AsUint8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float32x4.AsUint16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float32x4.AsUint32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float32x4.AsUint64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float32x4.AsFloat64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float64x2.AsInt8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float64x2.AsInt16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float64x2.AsInt32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float64x2.AsInt64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float64x2.AsUint8x16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float64x2.AsUint16x8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float64x2.AsUint32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float64x2.AsUint64x2", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float64x2.AsFloat32x4", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.BitsToFloat32", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float32x4.ToBits", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.BitsToFloat64", makeAsOp())
	addWasmSIMD("simd/archsimd", "Float64x2.ToBits", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.BitsToInt8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.ConvertToInt8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int8x16.ConvertToUint8", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int8x16.ToBits", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.BitsToInt16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.ConvertToInt16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.ConvertToUint16", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int16x8.ToBits", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.BitsToInt32", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.ConvertToInt32", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.ConvertToUint32", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int32x4.ToBits", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.BitsToInt64", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.ConvertToInt64", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.ConvertToUint64", makeAsOp())
	addWasmSIMD("simd/archsimd", "Int64x2.ToBits", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.ReshapeToUint16s", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.ReshapeToUint32s", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint8x16.ReshapeToUint64s", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.ReshapeToUint8s", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.ReshapeToUint32s", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint16x8.ReshapeToUint64s", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.ReshapeToUint8s", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.ReshapeToUint16s", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint32x4.ReshapeToUint64s", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.ReshapeToUint8s", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.ReshapeToUint16s", makeAsOp())
	addWasmSIMD("simd/archsimd", "Uint64x2.ReshapeToUint32s", makeAsOp())
}

```

// === FILE: references/go/src/cmd/compile/internal/ssagen/ssa.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssagen

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"go/constant"
	"html"
	"internal/buildcfg"
	"internal/goexperiment"
	"internal/runtime/gc"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"cmd/compile/internal/abi"
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/liveness"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/rttype"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/staticdata"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"
	"cmd/internal/sys"

	rtabi "internal/abi"
)

var ssaConfig *ssa.Config
var ssaCaches []ssa.Cache

var ssaDump string     // early copy of $GOSSAFUNC; the func name to dump output for
var ssaDir string      // optional destination for ssa dump file
var ssaDumpStdout bool // whether to dump to stdout
var ssaDumpCFG string  // generate CFGs for these phases
const ssaDumpFile = "ssa.html"

// ssaDumpInlined holds all inlined functions when ssaDump contains a function name.
var ssaDumpInlined []*ir.Func

// Maximum size we will aggregate heap allocations of scalar locals.
// Almost certainly can't hurt to be as big as the tiny allocator.
// Might help to be a bit bigger.
const maxAggregatedHeapAllocation = 16

func DumpInline(fn *ir.Func) {
	if ssaDump != "" && ssaDump == ir.FuncName(fn) {
		ssaDumpInlined = append(ssaDumpInlined, fn)
	}
}

func InitEnv() {
	ssaDump = os.Getenv("GOSSAFUNC")
	ssaDir = os.Getenv("GOSSADIR")
	if ssaDump != "" {
		if strings.HasSuffix(ssaDump, "+") {
			ssaDump = ssaDump[:len(ssaDump)-1]
			ssaDumpStdout = true
		}
		spl := strings.Split(ssaDump, ":")
		if len(spl) > 1 {
			ssaDump = spl[0]
			ssaDumpCFG = spl[1]
		}
	}
}

func InitConfig() {
	types_ := ssa.NewTypes()

	if Arch.SoftFloat {
		softfloatInit()
	}

	// Generate a few pointer types that are uncommon in the frontend but common in the backend.
	// Caching is disabled in the backend, so generating these here avoids allocations.
	_ = types.NewPtr(types.Types[types.TINTER])                             // *interface{}
	_ = types.NewPtr(types.NewPtr(types.Types[types.TSTRING]))              // **string
	_ = types.NewPtr(types.NewSlice(types.Types[types.TINTER]))             // *[]interface{}
	_ = types.NewPtr(types.NewPtr(types.ByteType))                          // **byte
	_ = types.NewPtr(types.NewSlice(types.ByteType))                        // *[]byte
	_ = types.NewPtr(types.NewSlice(types.Types[types.TSTRING]))            // *[]string
	_ = types.NewPtr(types.NewPtr(types.NewPtr(types.Types[types.TUINT8]))) // ***uint8
	_ = types.NewPtr(types.Types[types.TINT16])                             // *int16
	_ = types.NewPtr(types.Types[types.TINT64])                             // *int64
	_ = types.NewPtr(types.ErrorType)                                       // *error
	_ = types.NewPtr(reflectdata.MapType())                                 // *internal/runtime/maps.Map
	_ = types.NewPtr(deferstruct())                                         // *runtime._defer
	types.NewPtrCacheEnabled = false
	ssaConfig = ssa.NewConfig(base.Ctxt.Arch.Name, *types_, base.Ctxt, base.Flag.N == 0, Arch.SoftFloat)
	ssaConfig.Race = base.Flag.Race
	ssaCaches = make([]ssa.Cache, base.Flag.LowerC)

	// Set up some runtime functions we'll need to call.
	ir.Syms.AssertE2I = typecheck.LookupRuntimeFunc("assertE2I")
	ir.Syms.AssertE2I2 = typecheck.LookupRuntimeFunc("assertE2I2")
	ir.Syms.CgoCheckMemmove = typecheck.LookupRuntimeFunc("cgoCheckMemmove")
	ir.Syms.CgoCheckPtrWrite = typecheck.LookupRuntimeFunc("cgoCheckPtrWrite")
	ir.Syms.CheckPtrAlignment = typecheck.LookupRuntimeFunc("checkptrAlignment")
	ir.Syms.Deferproc = typecheck.LookupRuntimeFunc("deferproc")
	ir.Syms.Deferprocat = typecheck.LookupRuntimeFunc("deferprocat")
	ir.Syms.DeferprocStack = typecheck.LookupRuntimeFunc("deferprocStack")
	ir.Syms.Deferreturn = typecheck.LookupRuntimeFunc("deferreturn")
	ir.Syms.Duffcopy = typecheck.LookupRuntimeFunc("duffcopy")
	ir.Syms.Duffzero = typecheck.LookupRuntimeFunc("duffzero")
	ir.Syms.GCWriteBarrier[0] = typecheck.LookupRuntimeFunc("gcWriteBarrier1")
	ir.Syms.GCWriteBarrier[1] = typecheck.LookupRuntimeFunc("gcWriteBarrier2")
	ir.Syms.GCWriteBarrier[2] = typecheck.LookupRuntimeFunc("gcWriteBarrier3")
	ir.Syms.GCWriteBarrier[3] = typecheck.LookupRuntimeFunc("gcWriteBarrier4")
	ir.Syms.GCWriteBarrier[4] = typecheck.LookupRuntimeFunc("gcWriteBarrier5")
	ir.Syms.GCWriteBarrier[5] = typecheck.LookupRuntimeFunc("gcWriteBarrier6")
	ir.Syms.GCWriteBarrier[6] = typecheck.LookupRuntimeFunc("gcWriteBarrier7")
	ir.Syms.GCWriteBarrier[7] = typecheck.LookupRuntimeFunc("gcWriteBarrier8")
	ir.Syms.Goschedguarded = typecheck.LookupRuntimeFunc("goschedguarded")
	ir.Syms.Growslice = typecheck.LookupRuntimeFunc("growslice")
	ir.Syms.GrowsliceBuf = typecheck.LookupRuntimeFunc("growsliceBuf")
	ir.Syms.GrowsliceBufNoAlias = typecheck.LookupRuntimeFunc("growsliceBufNoAlias")
	ir.Syms.GrowsliceNoAlias = typecheck.LookupRuntimeFunc("growsliceNoAlias")
	ir.Syms.MoveSlice = typecheck.LookupRuntimeFunc("moveSlice")
	ir.Syms.MoveSliceNoScan = typecheck.LookupRuntimeFunc("moveSliceNoScan")
	ir.Syms.MoveSliceNoCap = typecheck.LookupRuntimeFunc("moveSliceNoCap")
	ir.Syms.MoveSliceNoCapNoScan = typecheck.LookupRuntimeFunc("moveSliceNoCapNoScan")
	ir.Syms.InterfaceSwitch = typecheck.LookupRuntimeFunc("interfaceSwitch")
	for i := 1; i < len(ir.Syms.MallocGCSmallNoScan); i++ {
		ir.Syms.MallocGCSmallNoScan[i] = typecheck.LookupRuntimeFunc(fmt.Sprintf("mallocgcSmallNoScanSC%d", i))
	}
	for i := 1; i < len(ir.Syms.MallocGCSmallScanNoHeader); i++ {
		ir.Syms.MallocGCSmallScanNoHeader[i] = typecheck.LookupRuntimeFunc(fmt.Sprintf("mallocgcSmallScanNoHeaderSC%d", i))
	}
	ir.Syms.MallocGCTiny = typecheck.LookupRuntimeFunc("mallocgcTinySC2")
	ir.Syms.MallocGC = typecheck.LookupRuntimeFunc("mallocgc")
	ir.Syms.Memmove = typecheck.LookupRuntimeFunc("memmove")
	ir.Syms.Memequal = typecheck.LookupRuntimeFunc("memequal")
	ir.Syms.Msanread = typecheck.LookupRuntimeFunc("msanread")
	ir.Syms.Msanwrite = typecheck.LookupRuntimeFunc("msanwrite")
	ir.Syms.Msanmove = typecheck.LookupRuntimeFunc("msanmove")
	ir.Syms.Asanread = typecheck.LookupRuntimeFunc("asanread")
	ir.Syms.Asanwrite = typecheck.LookupRuntimeFunc("asanwrite")
	ir.Syms.Newobject = typecheck.LookupRuntimeFunc("newobject")
	ir.Syms.Newproc = typecheck.LookupRuntimeFunc("newproc")
	ir.Syms.PanicBounds = typecheck.LookupRuntimeFunc("panicBounds")
	ir.Syms.PanicExtend = typecheck.LookupRuntimeFunc("panicExtend")
	ir.Syms.Panicdivide = typecheck.LookupRuntimeFunc("panicdivide")
	ir.Syms.PanicdottypeE = typecheck.LookupRuntimeFunc("panicdottypeE")
	ir.Syms.PanicdottypeI = typecheck.LookupRuntimeFunc("panicdottypeI")
	ir.Syms.Panicnildottype = typecheck.LookupRuntimeFunc("panicnildottype")
	ir.Syms.Panicoverflow = typecheck.LookupRuntimeFunc("panicoverflow")
	ir.Syms.Panicshift = typecheck.LookupRuntimeFunc("panicshift")
	ir.Syms.PanicSimdImm = typecheck.LookupRuntimeFunc("panicSimdImm")
	ir.Syms.Racefuncenter = typecheck.LookupRuntimeFunc("racefuncenter")
	ir.Syms.Racefuncexit = typecheck.LookupRuntimeFunc("racefuncexit")
	ir.Syms.Raceread = typecheck.LookupRuntimeFunc("raceread")
	ir.Syms.Racereadrange = typecheck.LookupRuntimeFunc("racereadrange")
	ir.Syms.Racewrite = typecheck.LookupRuntimeFunc("racewrite")
	ir.Syms.Racewriterange = typecheck.LookupRuntimeFunc("racewriterange")
	ir.Syms.TypeAssert = typecheck.LookupRuntimeFunc("typeAssert")
	ir.Syms.WBZero = typecheck.LookupRuntimeFunc("wbZero")
	ir.Syms.WBMove = typecheck.LookupRuntimeFunc("wbMove")
	ir.Syms.X86HasAVX = typecheck.LookupRuntimeVar("x86HasAVX")                       // bool
	ir.Syms.X86HasFMA = typecheck.LookupRuntimeVar("x86HasFMA")                       // bool
	ir.Syms.X86HasPOPCNT = typecheck.LookupRuntimeVar("x86HasPOPCNT")                 // bool
	ir.Syms.X86HasSSE41 = typecheck.LookupRuntimeVar("x86HasSSE41")                   // bool
	ir.Syms.ARMHasVFPv4 = typecheck.LookupRuntimeVar("armHasVFPv4")                   // bool
	ir.Syms.ARM64HasATOMICS = typecheck.LookupRuntimeVar("arm64HasATOMICS")           // bool
	ir.Syms.Loong64HasLAMCAS = typecheck.LookupRuntimeVar("loong64HasLAMCAS")         // bool
	ir.Syms.Loong64HasLAM_BH = typecheck.LookupRuntimeVar("loong64HasLAM_BH")         // bool
	ir.Syms.Loong64HasDBAR_HINTS = typecheck.LookupRuntimeVar("loong64HasDBAR_HINTS") // bool
	ir.Syms.Loong64HasLSX = typecheck.LookupRuntimeVar("loong64HasLSX")               // bool
	ir.Syms.RISCV64HasZbb = typecheck.LookupRuntimeVar("riscv64HasZbb")               // bool
	ir.Syms.Staticuint64s = typecheck.LookupRuntimeVar("staticuint64s")
	ir.Syms.Typedmemmove = typecheck.LookupRuntimeFunc("typedmemmove")
	ir.Syms.Udiv = typecheck.LookupRuntimeVar("udiv")                 // asm func with special ABI
	ir.Syms.WriteBarrier = typecheck.LookupRuntimeVar("writeBarrier") // struct { bool; ... }
	ir.Syms.Zerobase = typecheck.LookupRuntimeVar("zerobase")
	ir.Syms.ZeroVal = typecheck.LookupRuntimeVar("zeroVal")

	if Arch.LinkArch.Family == sys.Wasm {
		BoundsCheckFunc[ssa.BoundsIndex] = typecheck.LookupRuntimeFunc("goPanicIndex")
		BoundsCheckFunc[ssa.BoundsIndexU] = typecheck.LookupRuntimeFunc("goPanicIndexU")
		BoundsCheckFunc[ssa.BoundsSliceAlen] = typecheck.LookupRuntimeFunc("goPanicSliceAlen")
		BoundsCheckFunc[ssa.BoundsSliceAlenU] = typecheck.LookupRuntimeFunc("goPanicSliceAlenU")
		BoundsCheckFunc[ssa.BoundsSliceAcap] = typecheck.LookupRuntimeFunc("goPanicSliceAcap")
		BoundsCheckFunc[ssa.BoundsSliceAcapU] = typecheck.LookupRuntimeFunc("goPanicSliceAcapU")
		BoundsCheckFunc[ssa.BoundsSliceB] = typecheck.LookupRuntimeFunc("goPanicSliceB")
		BoundsCheckFunc[ssa.BoundsSliceBU] = typecheck.LookupRuntimeFunc("goPanicSliceBU")
		BoundsCheckFunc[ssa.BoundsSlice3Alen] = typecheck.LookupRuntimeFunc("goPanicSlice3Alen")
		BoundsCheckFunc[ssa.BoundsSlice3AlenU] = typecheck.LookupRuntimeFunc("goPanicSlice3AlenU")
		BoundsCheckFunc[ssa.BoundsSlice3Acap] = typecheck.LookupRuntimeFunc("goPanicSlice3Acap")
		BoundsCheckFunc[ssa.BoundsSlice3AcapU] = typecheck.LookupRuntimeFunc("goPanicSlice3AcapU")
		BoundsCheckFunc[ssa.BoundsSlice3B] = typecheck.LookupRuntimeFunc("goPanicSlice3B")
		BoundsCheckFunc[ssa.BoundsSlice3BU] = typecheck.LookupRuntimeFunc("goPanicSlice3BU")
		BoundsCheckFunc[ssa.BoundsSlice3C] = typecheck.LookupRuntimeFunc("goPanicSlice3C")
		BoundsCheckFunc[ssa.BoundsSlice3CU] = typecheck.LookupRuntimeFunc("goPanicSlice3CU")
		BoundsCheckFunc[ssa.BoundsConvert] = typecheck.LookupRuntimeFunc("goPanicSliceConvert")
	}

	// Wasm (all asm funcs with special ABIs)
	ir.Syms.WasmDiv = typecheck.LookupRuntimeVar("wasmDiv")
	ir.Syms.WasmTruncS = typecheck.LookupRuntimeVar("wasmTruncS")
	ir.Syms.WasmTruncU = typecheck.LookupRuntimeVar("wasmTruncU")
	ir.Syms.SigPanic = typecheck.LookupRuntimeFunc("sigpanic")
}

func InitTables() {
	initIntrinsics(nil)
}

// AbiForBodylessFuncStackMap returns the ABI for a bodyless function's stack map.
// This is not necessarily the ABI used to call it.
// Currently (1.17 dev) such a stack map is always ABI0;
// any ABI wrapper that is present is nosplit, hence a precise
// stack map is not needed there (the parameters survive only long
// enough to call the wrapped assembly function).
// This always returns a freshly copied ABI.
func AbiForBodylessFuncStackMap(fn *ir.Func) *abi.ABIConfig {
	return ssaConfig.ABI0.Copy() // No idea what races will result, be safe
}

// abiForFunc implements ABI policy for a function, but does not return a copy of the ABI.
// Passing a nil function returns the default ABI based on experiment configuration.
func abiForFunc(fn *ir.Func, abi0, abi1 *abi.ABIConfig) *abi.ABIConfig {
	if buildcfg.Experiment.RegabiArgs {
		// Select the ABI based on the function's defining ABI.
		if fn == nil {
			return abi1
		}
		switch fn.ABI {
		case obj.ABI0:
			return abi0
		case obj.ABIInternal:
			// TODO(austin): Clean up the nomenclature here.
			// It's not clear that "abi1" is ABIInternal.
			return abi1
		}
		base.Fatalf("function %v has unknown ABI %v", fn, fn.ABI)
		panic("not reachable")
	}

	a := abi0
	if fn != nil {
		if fn.Pragma&ir.RegisterParams != 0 { // TODO(register args) remove after register abi is working
			a = abi1
		}
	}
	return a
}

// emitOpenDeferInfo emits FUNCDATA information about the defers in a function
// that is using open-coded defers.  This funcdata is used to determine the active
// defers in a function and execute those defers during panic processing.
//
// The funcdata is all encoded in varints (since values will almost always be less than
// 128, but stack offsets could potentially be up to 2Gbyte). All "locations" (offsets)
// for stack variables are specified as the number of bytes below varp (pointer to the
// top of the local variables) for their starting address. The format is:
//
//   - Offset of the deferBits variable
//   - Offset of the first closure slot (the rest are laid out consecutively).
func (s *state) emitOpenDeferInfo() {
	firstOffset := s.openDefers[0].closureNode.FrameOffset()

	// Verify that cmpstackvarlt laid out the slots in order.
	for i, r := range s.openDefers {
		have := r.closureNode.FrameOffset()
		want := firstOffset + int64(i)*int64(types.PtrSize)
		if have != want {
			base.FatalfAt(s.curfn.Pos(), "unexpected frame offset for open-coded defer slot #%v: have %v, want %v", i, have, want)
		}
	}

	x := base.Ctxt.Lookup(s.curfn.LSym.Name + ".opendefer")
	x.Set(obj.AttrContentAddressable, true)
	x.Align = 1
	s.curfn.LSym.Func().OpenCodedDeferInfo = x

	off := 0
	off = objw.Uvarint(x, off, uint64(-s.deferBitsTemp.FrameOffset()))
	off = objw.Uvarint(x, off, uint64(-firstOffset))
}

// buildssa builds an SSA function for fn.
// worker indicates which of the backend workers is doing the processing.
func buildssa(fn *ir.Func, worker int, isPgoHot bool) *ssa.Func {
	name := ir.FuncName(fn)

	abiSelf := abiForFunc(fn, ssaConfig.ABI0, ssaConfig.ABI1)

	printssa := false
	// match either a simple name e.g. "(*Reader).Reset", package.name e.g. "compress/gzip.(*Reader).Reset", or subpackage name "gzip.(*Reader).Reset"
	// optionally allows an ABI suffix specification in the GOSSAHASH, e.g. "(*Reader).Reset<0>" etc
	if strings.Contains(ssaDump, name) { // in all the cases the function name is entirely contained within the GOSSAFUNC string.
		nameOptABI := name
		if l := len(ssaDump); l > 1 && ssaDump[l-2] == ',' { // ABI specification
			nameOptABI = ssa.FuncNameABI(name, abiSelf.Which())
		} else if strings.HasSuffix(ssaDump, ">") { // if they use the linker syntax instead....
			l := len(ssaDump)
			if l >= 3 && ssaDump[l-3] == '<' {
				nameOptABI = ssa.FuncNameABI(name, abiSelf.Which())
				ssaDump = ssaDump[:l-3] + "," + ssaDump[l-2:l-1]
			}
		}
		pkgDotName := base.Ctxt.Pkgpath + "." + nameOptABI
		printssa = nameOptABI == ssaDump || // "(*Reader).Reset"
			pkgDotName == ssaDump || // "compress/gzip.(*Reader).Reset"
			strings.HasSuffix(pkgDotName, ssaDump) && strings.HasSuffix(pkgDotName, "/"+ssaDump) // "gzip.(*Reader).Reset"
	}

	var astBuf *bytes.Buffer
	if printssa {
		astBuf = &bytes.Buffer{}
		ir.FDumpList(astBuf, "buildssa-body", fn.Body)
		if ssaDumpStdout {
			fmt.Println("generating SSA for", name)
			fmt.Print(astBuf.String())
		}
	}

	var s state
	s.pushLine(fn.Pos())
	defer s.popLine()

	s.hasdefer = fn.HasDefer()
	if fn.Pragma&ir.CgoUnsafeArgs != 0 {
		s.cgoUnsafeArgs = true
	}
	s.checkPtrEnabled = ir.ShouldCheckPtr(fn, 1)

	if base.Flag.Cfg.Instrumenting && fn.Pragma&ir.Norace == 0 && !fn.Linksym().ABIWrapper() {
		if !base.Flag.Race || !objabi.LookupPkgSpecial(fn.Sym().Pkg.Path).NoRaceFunc {
			s.instrumentMemory = true
			if base.Flag.Race {
				s.instrumentEnterExit = true
			}
		}
	}

	fe := ssafn{
		curfn: fn,
		log:   printssa && ssaDumpStdout,
	}
	s.curfn = fn

	cache := &ssaCaches[worker]
	cache.Reset()

	s.f = ssaConfig.NewFunc(&fe, cache)
	s.config = ssaConfig
	s.f.Type = fn.Type()
	s.f.Name = name
	s.f.PrintOrHtmlSSA = printssa
	if fn.Pragma&ir.Nosplit != 0 {
		s.f.NoSplit = true
	}
	s.f.ABI0 = ssaConfig.ABI0
	s.f.ABI1 = ssaConfig.ABI1
	s.f.ABIDefault = abiForFunc(nil, ssaConfig.ABI0, ssaConfig.ABI1)
	s.f.ABISelf = abiSelf

	s.panics = map[funcLine]*ssa.Block{}
	s.softFloat = s.config.SoftFloat

	// Allocate starting block
	s.f.Entry = s.f.NewBlock(ssa.BlockPlain)
	s.f.Entry.Pos = fn.Pos()
	s.f.IsPgoHot = isPgoHot

	if printssa {
		ssaDF := ssaDumpFile
		if ssaDir != "" {
			ssaDF = filepath.Join(ssaDir, base.Ctxt.Pkgpath+"."+s.f.NameABI()+".html")
			ssaD := filepath.Dir(ssaDF)
			os.MkdirAll(ssaD, 0755)
		}
		s.f.HTMLWriter = ssa.NewHTMLWriter(ssaDF, s.f, ssaDumpCFG)
		// TODO: generate and print a mapping from nodes to values and blocks
		dumpSourcesColumn(s.f.HTMLWriter, fn)
		s.f.HTMLWriter.WriteAST("AST", astBuf)
	}

	// Allocate starting values
	s.labels = map[string]*ssaLabel{}
	s.fwdVars = map[ir.Node]*ssa.Value{}
	s.startmem = s.entryNewValue0(ssa.OpInitMem, types.TypeMem)

	s.hasOpenDefers = base.Flag.N == 0 && s.hasdefer && !s.curfn.OpenCodedDeferDisallowed()
	switch {
	case base.Debug.NoOpenDefer != 0:
		s.hasOpenDefers = false
	case s.hasOpenDefers && (base.Ctxt.Flag_shared || base.Ctxt.Flag_dynlink) && base.Ctxt.Arch.Name == "386":
		// Don't support open-coded defers for 386 ONLY when using shared
		// libraries, because there is extra code (added by rewriteToUseGot())
		// preceding the deferreturn/ret code that we don't track correctly.
		//
		// TODO this restriction can be removed given adjusted offset in computeDeferReturn in cmd/link/internal/ld/pcln.go
		s.hasOpenDefers = false
	}
	if s.hasOpenDefers && s.instrumentEnterExit {
		// Skip doing open defers if we need to instrument function
		// returns for the race detector, since we will not generate that
		// code in the case of the extra deferreturn/ret segment.
		s.hasOpenDefers = false
	}
	if s.hasOpenDefers {
		// Similarly, skip if there are any heap-allocated result
		// parameters that need to be copied back to their stack slots.
		for _, f := range s.curfn.Type().Results() {
			if !f.Nname.(*ir.Name).OnStack() {
				s.hasOpenDefers = false
				break
			}
		}
	}
	if s.hasOpenDefers &&
		s.curfn.NumReturns*s.curfn.NumDefers > 15 {
		// Since we are generating defer calls at every exit for
		// open-coded defers, skip doing open-coded defers if there are
		// too many returns (especially if there are multiple defers).
		// Open-coded defers are most important for improving performance
		// for smaller functions (which don't have many returns).
		s.hasOpenDefers = false
	}

	s.sp = s.entryNewValue0(ssa.OpSP, types.Types[types.TUINTPTR]) // TODO: use generic pointer type (unsafe.Pointer?) instead
	s.sb = s.entryNewValue0(ssa.OpSB, types.Types[types.TUINTPTR])

	s.startBlock(s.f.Entry)
	s.vars[memVar] = s.startmem
	if s.hasOpenDefers {
		// Create the deferBits variable and stack slot.  deferBits is a
		// bitmask showing which of the open-coded defers in this function
		// have been activated.
		deferBitsTemp := typecheck.TempAt(src.NoXPos, s.curfn, types.Types[types.TUINT8])
		deferBitsTemp.SetAddrtaken(true)
		s.deferBitsTemp = deferBitsTemp
		// For this value, AuxInt is initialized to zero by default
		startDeferBits := s.entryNewValue0(ssa.OpConst8, types.Types[types.TUINT8])
		s.vars[deferBitsVar] = startDeferBits
		s.deferBitsAddr = s.addr(deferBitsTemp)
		s.store(types.Types[types.TUINT8], s.deferBitsAddr, startDeferBits)
		// Make sure that the deferBits stack slot is kept alive (for use
		// by panics) and stores to deferBits are not eliminated, even if
		// all checking code on deferBits in the function exit can be
		// eliminated, because the defer statements were all
		// unconditional.
		s.vars[memVar] = s.newValue1Apos(ssa.OpVarLive, types.TypeMem, deferBitsTemp, s.mem(), false)
	}

	var params *abi.ABIParamResultInfo
	params = s.f.ABISelf.ABIAnalyze(fn.Type(), true)

	// The backend's stackframe pass prunes away entries from the fn's
	// Dcl list, including PARAMOUT nodes that correspond to output
	// params passed in registers. Walk the Dcl list and capture these
	// nodes to a side list, so that we'll have them available during
	// DWARF-gen later on. See issue 48573 for more details.
	var debugInfo ssa.FuncDebug
	for _, n := range fn.Dcl {
		if n.Class == ir.PPARAMOUT && n.IsOutputParamInRegisters() {
			debugInfo.RegOutputParams = append(debugInfo.RegOutputParams, n)
		}
	}
	fn.DebugInfo = &debugInfo

	// Generate addresses of local declarations
	s.decladdrs = map[*ir.Name]*ssa.Value{}
	for _, n := range fn.Dcl {
		switch n.Class {
		case ir.PPARAM:
			// Be aware that blank and unnamed input parameters will not appear here, but do appear in the type
			s.decladdrs[n] = s.entryNewValue2A(ssa.OpLocalAddr, types.NewPtr(n.Type()), n, s.sp, s.startmem)
		case ir.PPARAMOUT:
			s.decladdrs[n] = s.entryNewValue2A(ssa.OpLocalAddr, types.NewPtr(n.Type()), n, s.sp, s.startmem)
		case ir.PAUTO:
			// processed at each use, to prevent Addr coming
			// before the decl.
		default:
			s.Fatalf("local variable with class %v unimplemented", n.Class)
		}
	}

	s.f.OwnAux = ssa.OwnAuxCall(fn.LSym, params)

	// Populate SSAable arguments.
	for _, n := range fn.Dcl {
		if n.Class == ir.PPARAM {
			if s.canSSA(n) {
				v := s.newValue0A(ssa.OpArg, n.Type(), n)
				s.vars[n] = v
				s.addNamedValue(n, v) // This helps with debugging information, not needed for compilation itself.
			} else { // address was taken AND/OR too large for SSA
				paramAssignment := ssa.ParamAssignmentForArgName(s.f, n)
				if len(paramAssignment.Registers) > 0 {
					if ssa.CanSSA(n.Type()) { // SSA-able type, so address was taken -- receive value in OpArg, DO NOT bind to var, store immediately to memory.
						v := s.newValue0A(ssa.OpArg, n.Type(), n)
						s.store(n.Type(), s.decladdrs[n], v)
					} else { // Too big for SSA.
						// Brute force, and early, do a bunch of stores from registers
						// Note that expand calls knows about this and doesn't trouble itself with larger-than-SSA-able Args in registers.
						s.storeParameterRegsToStack(s.f.ABISelf, paramAssignment, n, s.decladdrs[n], false)
					}
				}
			}
		}
	}

	// Populate closure variables.
	if fn.Needctxt() {
		clo := s.entryNewValue0(ssa.OpGetClosurePtr, s.f.Config.Types.BytePtr)
		if fn.RangeParent != nil && base.Flag.N != 0 {
			// For a range body closure, keep its closure pointer live on the
			// stack with a special name, so the debugger can look for it and
			// find the parent frame.
			sym := &types.Sym{Name: ".closureptr", Pkg: types.LocalPkg}
			cloSlot := s.curfn.NewLocal(src.NoXPos, sym, s.f.Config.Types.BytePtr)
			cloSlot.SetUsed(true)
			cloSlot.SetEsc(ir.EscNever)
			cloSlot.SetAddrtaken(true)
			s.f.CloSlot = cloSlot
			s.vars[memVar] = s.newValue1Apos(ssa.OpVarDef, types.TypeMem, cloSlot, s.mem(), false)
			addr := s.addr(cloSlot)
			s.store(s.f.Config.Types.BytePtr, addr, clo)
			// Keep it from being dead-store eliminated.
			s.vars[memVar] = s.newValue1Apos(ssa.OpVarLive, types.TypeMem, cloSlot, s.mem(), false)
		}
		csiter := typecheck.NewClosureStructIter(fn.ClosureVars)
		for {
			n, typ, offset := csiter.Next()
			if n == nil {
				break
			}

			ptr := s.newValue1I(ssa.OpOffPtr, types.NewPtr(typ), offset, clo)

			// If n is a small variable captured by value, promote
			// it to PAUTO so it can be converted to SSA.
			//
			// Note: While we never capture a variable by value if
			// the user took its address, we may have generated
			// runtime calls that did (#43701). Since we don't
			// convert Addrtaken variables to SSA anyway, no point
			// in promoting them either.
			if n.Byval() && !n.Addrtaken() && ssa.CanSSA(n.Type()) {
				n.Class = ir.PAUTO
				fn.Dcl = append(fn.Dcl, n)
				s.assign(n, s.load(n.Type(), ptr), false, 0)
				continue
			}

			if !n.Byval() {
				ptr = s.load(typ, ptr)
			}
			s.setHeapaddr(fn.Pos(), n, ptr)
		}
	}

	// Convert the AST-based IR to the SSA-based IR
	if s.instrumentEnterExit {
		s.rtcall(ir.Syms.Racefuncenter, true, nil, s.newValue0(ssa.OpGetCallerPC, types.Types[types.TUINTPTR]))
	}
	s.zeroResults()
	s.paramsToHeap()
	s.stmtList(fn.Body)

	// fallthrough to exit
	if s.curBlock != nil {
		s.pushLine(fn.Endlineno)
		s.exit()
		s.popLine()
	}

	for _, b := range s.f.Blocks {
		if b.Pos != src.NoXPos {
			s.updateUnsetPredPos(b)
		}
	}

	s.f.HTMLWriter.WritePhase("before insert phis", "before insert phis")

	s.insertPhis()

	// Main call to ssa package to compile function
	ssa.Compile(s.f)

	fe.AllocFrame(s.f)

	if len(s.openDefers) != 0 {
		s.emitOpenDeferInfo()
	}

	// Record incoming parameter spill information for morestack calls emitted in the assembler.
	// This is done here, using all the parameters (used, partially used, and unused) because
	// it mimics the behavior of the former ABI (everything stored) and because it's not 100%
	// clear if naming conventions are respected in autogenerated code.
	// TODO figure out exactly what's unused, don't spill it. Make liveness fine-grained, also.
	for _, p := range params.InParams() {
		typs, offs := p.RegisterTypesAndOffsets()
		if len(offs) < len(typs) {
			s.Fatalf("len(offs)=%d < len(typs)=%d, params=\n%s", len(offs), len(typs), params)
		}
		for i, t := range typs {
			o := offs[i]                // offset within parameter
			fo := p.FrameOffset(params) // offset of parameter in frame
			reg := ssa.ObjRegForAbiReg(p.Registers[i], s.f.Config)
			s.f.RegArgs = append(s.f.RegArgs, ssa.Spill{Reg: reg, Offset: fo + o, Type: t})
		}
	}

	return s.f
}

func (s *state) storeParameterRegsToStack(abi *abi.ABIConfig, paramAssignment *abi.ABIParamAssignment, n *ir.Name, addr *ssa.Value, pointersOnly bool) {
	typs, offs := paramAssignment.RegisterTypesAndOffsets()
	for i, t := range typs {
		if pointersOnly && !t.IsPtrShaped() {
			continue
		}
		r := paramAssignment.Registers[i]
		o := offs[i]
		op, reg := ssa.ArgOpAndRegisterFor(r, abi)
		aux := &ssa.AuxNameOffset{Name: n, Offset: o}
		v := s.newValue0I(op, t, reg)
		v.Aux = aux
		p := s.newValue1I(ssa.OpOffPtr, types.NewPtr(t), o, addr)
		s.store(t, p, v)
	}
}

// zeroResults zeros the return values at the start of the function.
// We need to do this very early in the function.  Defer might stop a
// panic and show the return values as they exist at the time of
// panic.  For precise stacks, the garbage collector assumes results
// are always live, so we need to zero them before any allocations,
// even allocations to move params/results to the heap.
func (s *state) zeroResults() {
	for _, f := range s.curfn.Type().Results() {
		n := f.Nname.(*ir.Name)
		if !n.OnStack() {
			// The local which points to the return value is the
			// thing that needs zeroing. This is already handled
			// by a Needzero annotation in plive.go:(*liveness).epilogue.
			continue
		}
		// Zero the stack location containing f.
		if typ := n.Type(); ssa.CanSSA(typ) {
			s.assign(n, s.zeroVal(typ), false, 0)
		} else {
			if typ.HasPointers() || ssa.IsMergeCandidate(n) {
				s.vars[memVar] = s.newValue1A(ssa.OpVarDef, types.TypeMem, n, s.mem())
			}
			s.zero(n.Type(), s.decladdrs[n])
		}
	}
}

// paramsToHeap produces code to allocate memory for heap-escaped parameters
// and to copy non-result parameters' values from the stack.
func (s *state) paramsToHeap() {
	do := func(params []*types.Field) {
		for _, f := range params {
			if f.Nname == nil {
				continue // anonymous or blank parameter
			}
			n := f.Nname.(*ir.Name)
			if ir.IsBlank(n) || n.OnStack() {
				continue
			}
			s.newHeapaddr(n)
			if n.Class == ir.PPARAM {
				s.move(n.Type(), s.expr(n.Heapaddr), s.decladdrs[n])
			}
		}
	}

	typ := s.curfn.Type()
	do(typ.Recvs())
	do(typ.Params())
	do(typ.Results())
}

// allocSizeAndAlign returns the size and alignment of t.
// Normally just t.Size() and t.Alignment(), but there
// is a special case to handle 64-bit atomics on 32-bit systems.
func allocSizeAndAlign(t *types.Type) (int64, int64) {
	size, align := t.Size(), t.Alignment()
	if types.PtrSize == 4 && align == 4 && size >= 8 {
		// For 64-bit atomics on 32-bit systems.
		size = types.RoundUp(size, 8)
		align = 8
	}
	return size, align
}
func allocSize(t *types.Type) int64 {
	size, _ := allocSizeAndAlign(t)
	return size
}
func allocAlign(t *types.Type) int64 {
	_, align := allocSizeAndAlign(t)
	return align
}

// newHeapaddr allocates heap memory for n and sets its heap address.
func (s *state) newHeapaddr(n *ir.Name) {
	size := allocSize(n.Type())
	if n.Type().HasPointers() || size >= maxAggregatedHeapAllocation || size == 0 {
		s.setHeapaddr(n.Pos(), n, s.newObject(n.Type()))
		return
	}

	// Do we have room together with our pending allocations?
	// If not, flush all the current ones.
	var used int64
	for _, v := range s.pendingHeapAllocations {
		used += allocSize(v.Type.Elem())
	}
	if used+size > maxAggregatedHeapAllocation {
		s.flushPendingHeapAllocations()
	}

	var allocCall *ssa.Value // (SelectN [0] (call of runtime.newobject))
	if len(s.pendingHeapAllocations) == 0 {
		// Make an allocation, but the type being allocated is just
		// the first pending object. We will come back and update it
		// later if needed.
		allocCall = s.newObjectNonSpecialized(n.Type(), nil)
	} else {
		allocCall = s.pendingHeapAllocations[0].Args[0]
	}
	// v is an offset to the shared allocation. Offsets are dummy 0s for now.
	v := s.newValue1I(ssa.OpOffPtr, n.Type().PtrTo(), 0, allocCall)

	// Add to list of pending allocations.
	s.pendingHeapAllocations = append(s.pendingHeapAllocations, v)

	// Finally, record for posterity.
	s.setHeapaddr(n.Pos(), n, v)
}

func (s *state) flushPendingHeapAllocations() {
	pending := s.pendingHeapAllocations
	if len(pending) == 0 {
		return // nothing to do
	}
	s.pendingHeapAllocations = nil // reset state
	ptr := pending[0].Args[0]      // The SelectN [0] op
	call := ptr.Args[0]            // The runtime.newobject call

	if len(pending) == 1 {
		// Just a single object, do a standard allocation.
		v := pending[0]
		v.Op = ssa.OpCopy // instead of OffPtr [0]
		return
	}

	// Sort in decreasing alignment.
	// This way we never have to worry about padding.
	// (Stable not required; just cleaner to keep program order among equal alignments.)
	slices.SortStableFunc(pending, func(x, y *ssa.Value) int {
		return cmp.Compare(allocAlign(y.Type.Elem()), allocAlign(x.Type.Elem()))
	})

	// Figure out how much data we need allocate.
	var size int64
	for _, v := range pending {
		v.AuxInt = size // Adjust OffPtr to the right value while we are here.
		size += allocSize(v.Type.Elem())
	}
	align := allocAlign(pending[0].Type.Elem())
	size = types.RoundUp(size, align)

	// Convert newObject call to a mallocgc call.
	args := []*ssa.Value{
		s.constInt(types.Types[types.TUINTPTR], size),
		s.constNil(call.Args[0].Type), // a nil *runtime._type
		s.constBool(true),             // needZero TODO: false is ok?
		call.Args[1],                  // memory
	}
	mallocSym := ir.Syms.MallocGC
	if specialMallocSym := s.specializedMallocSym(size, false); specialMallocSym != nil {
		mallocSym = specialMallocSym
	}
	call.Aux = ssa.StaticAuxCall(mallocSym, s.f.ABIDefault.ABIAnalyzeTypes(
		[]*types.Type{args[0].Type, args[1].Type, args[2].Type},
		[]*types.Type{types.Types[types.TUNSAFEPTR]},
	))
	call.AuxInt = 4 * s.config.PtrSize // arg+results size, uintptr/ptr/bool/ptr
	call.SetArgs4(args[0], args[1], args[2], args[3])
	// TODO: figure out how to pass alignment to runtime

	call.Type = types.NewTuple(types.Types[types.TUNSAFEPTR], types.TypeMem)
	ptr.Type = types.Types[types.TUNSAFEPTR]
}

func (s *state) specializedMallocSym(size int64, hasPointers bool) *obj.LSym {
	if !s.sizeSpecializedMallocEnabled() {
		return nil
	}
	const specializedMallocMax = 80 // This must match the constant in mkmalloc.
	if size > specializedMallocMax {
		return nil
	}
	divRoundUp := func(n, a uintptr) uintptr { return (n + a - 1) / a }
	sizeClass := gc.SizeToSizeClass8[divRoundUp(uintptr(size), gc.SmallSizeDiv)]
	if hasPointers {
		return ir.Syms.MallocGCSmallScanNoHeader[sizeClass]
	}
	if size < gc.TinySize {
		return ir.Syms.MallocGCTiny
	}
	return ir.Syms.MallocGCSmallNoScan[sizeClass]
}

func (s *state) sizeSpecializedMallocEnabled() bool {
	if base.Flag.CompilingRuntime {
		// The compiler forces the values of the asan, msan, and race flags to false if
		// we're compiling the runtime, so we lose the information about whether we're
		// building in asan, msan, or race mode. Because the specialized functions don't
		// work in that mode, just turn if off in that case.
		// TODO(matloob): Save the information about whether the flags were passed in
		// originally so we can turn off size specialized malloc in that case instead
		// using Instrumenting below. Then we can remove this condition.
		return false
	}

	return buildcfg.Experiment.SizeSpecializedMalloc && !base.Flag.Cfg.Instrumenting
}

// setHeapaddr allocates a new PAUTO variable to store ptr (which must be non-nil)
// and then sets it as n's heap address.
func (s *state) setHeapaddr(pos src.XPos, n *ir.Name, ptr *ssa.Value) {
	if !ptr.Type.IsPtr() || !types.Identical(n.Type(), ptr.Type.Elem()) {
		base.FatalfAt(n.Pos(), "setHeapaddr %L with type %v", n, ptr.Type)
	}

	// Declare variable to hold address.
	sym := &types.Sym{Name: "&" + n.Sym().Name, Pkg: types.LocalPkg}
	addr := s.curfn.NewLocal(pos, sym, types.NewPtr(n.Type()))
	addr.SetUsed(true)
	types.CalcSize(addr.Type())

	if n.Class == ir.PPARAMOUT {
		addr.SetIsOutputParamHeapAddr(true)
	}

	n.Heapaddr = addr
	s.assign(addr, ptr, false, 0)
}

// newObject returns an SSA value denoting new(typ).
func (s *state) newObject(typ *types.Type) *ssa.Value {
	if typ.Size() == 0 {
		return s.newValue1A(ssa.OpAddr, types.NewPtr(typ), ir.Syms.Zerobase, s.sb)
	}
	rtype := s.reflectType(typ)
	if specialMallocSym := s.specializedMallocSym(typ.Size(), typ.HasPointers()); specialMallocSym != nil {
		return s.rtcall(specialMallocSym, true, []*types.Type{types.NewPtr(typ)},
			s.constInt(types.Types[types.TUINTPTR], typ.Size()),
			rtype,
			s.constBool(true),
		)[0]
	}
	return s.rtcall(ir.Syms.Newobject, true, []*types.Type{types.NewPtr(typ)}, rtype)[0]
}

// newObjectNonSpecialized returns an SSA value denoting new(typ). It does
// not produce size-specialized malloc functions.
func (s *state) newObjectNonSpecialized(typ *types.Type, rtype *ssa.Value) *ssa.Value {
	if typ.Size() == 0 {
		return s.newValue1A(ssa.OpAddr, types.NewPtr(typ), ir.Syms.Zerobase, s.sb)
	}
	if rtype == nil {
		rtype = s.reflectType(typ)
	}
	return s.rtcall(ir.Syms.Newobject, true, []*types.Type{types.NewPtr(typ)}, rtype)[0]
}

func (s *state) checkPtrAlignment(n *ir.ConvExpr, v *ssa.Value, count *ssa.Value) {
	if !n.Type().IsPtr() {
		s.Fatalf("expected pointer type: %v", n.Type())
	}
	elem, rtypeExpr := n.Type().Elem(), n.ElemRType
	if count != nil {
		if !elem.IsArray() {
			s.Fatalf("expected array type: %v", elem)
		}
		elem, rtypeExpr = elem.Elem(), n.ElemElemRType
	}
	size := elem.Size()
	// Casting from larger type to smaller one is ok, so for smallest type, do nothing.
	if elem.Alignment() == 1 && (size == 0 || size == 1 || count == nil) {
		return
	}
	if count == nil {
		count = s.constInt(types.Types[types.TUINTPTR], 1)
	}
	if count.Type.Size() != s.config.PtrSize {
		s.Fatalf("expected count fit to a uintptr size, have: %d, want: %d", count.Type.Size(), s.config.PtrSize)
	}
	var rtype *ssa.Value
	if rtypeExpr != nil {
		rtype = s.expr(rtypeExpr)
	} else {
		rtype = s.reflectType(elem)
	}
	s.rtcall(ir.Syms.CheckPtrAlignment, true, nil, v, rtype, count)
}

// reflectType returns an SSA value representing a pointer to typ's
// reflection type descriptor.
func (s *state) reflectType(typ *types.Type) *ssa.Value {
	// TODO(mdempsky): Make this Fatalf under Unified IR; frontend needs
	// to supply RType expressions.
	lsym := reflectdata.TypeLinksym(typ)
	return s.entryNewValue1A(ssa.OpAddr, types.NewPtr(types.Types[types.TUINT8]), lsym, s.sb)
}

func dumpSourcesColumn(writer *ssa.HTMLWriter, fn *ir.Func) {
	// Read sources of target function fn.
	fname := base.Ctxt.PosTable.Pos(fn.Pos()).Filename()
	targetFn, err := readFuncLines(fname, fn.Pos().Line(), fn.Endlineno.Line())
	if err != nil {
		writer.Logf("cannot read sources for function %v: %v", fn, err)
	}

	// Read sources of inlined functions.
	var inlFns []*ssa.FuncLines
	for _, fi := range ssaDumpInlined {
		elno := fi.Endlineno
		fname := base.Ctxt.PosTable.Pos(fi.Pos()).Filename()
		fnLines, err := readFuncLines(fname, fi.Pos().Line(), elno.Line())
		if err != nil {
			writer.Logf("cannot read sources for inlined function %v: %v", fi, err)
			continue
		}
		inlFns = append(inlFns, fnLines)
	}

	slices.SortFunc(inlFns, ssa.ByTopoCmp)
	if targetFn != nil {
		inlFns = append([]*ssa.FuncLines{targetFn}, inlFns...)
	}

	writer.WriteSources("sources", inlFns)
}

func readFuncLines(file string, start, end uint) (*ssa.FuncLines, error) {
	f, err := os.Open(os.ExpandEnv(file))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	ln := uint(1)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() && ln <= end {
		if ln >= start {
			lines = append(lines, scanner.Text())
		}
		ln++
	}
	return &ssa.FuncLines{Filename: file, StartLineno: start, Lines: lines}, nil
}

// updateUnsetPredPos propagates the earliest-value position information for b
// towards all of b's predecessors that need a position, and recurs on that
// predecessor if its position is updated. B should have a non-empty position.
func (s *state) updateUnsetPredPos(b *ssa.Block) {
	if b.Pos == src.NoXPos {
		s.Fatalf("Block %s should have a position", b)
	}
	bestPos := src.NoXPos
	for _, e := range b.Preds {
		p := e.Block()
		if !p.LackingPos() {
			continue
		}
		if bestPos == src.NoXPos {
			bestPos = b.Pos
			for _, v := range b.Values {
				if v.LackingPos() {
					continue
				}
				if v.Pos != src.NoXPos {
					// Assume values are still in roughly textual order;
					// TODO: could also seek minimum position?
					bestPos = v.Pos
					break
				}
			}
		}
		p.Pos = bestPos
		s.updateUnsetPredPos(p) // We do not expect long chains of these, thus recursion is okay.
	}
}

// Information about each open-coded defer.
type openDeferInfo struct {
	// The node representing the call of the defer
	n *ir.CallExpr
	// If defer call is closure call, the address of the argtmp where the
	// closure is stored.
	closure *ssa.Value
	// The node representing the argtmp where the closure is stored - used for
	// function, method, or interface call, to store a closure that panic
	// processing can use for this defer.
	closureNode *ir.Name
}

type state struct {
	// configuration (arch) information
	config *ssa.Config

	// function we're building
	f *ssa.Func

	// Node for function
	curfn *ir.Func

	// labels in f
	labels map[string]*ssaLabel

	// unlabeled break and continue statement tracking
	breakTo    *ssa.Block // current target for plain break statement
	continueTo *ssa.Block // current target for plain continue statement

	// current location where we're interpreting the AST
	curBlock *ssa.Block

	// variable assignments in the current block (map from variable symbol to ssa value)
	// *Node is the unique identifier (an ONAME Node) for the variable.
	// TODO: keep a single varnum map, then make all of these maps slices instead?
	vars map[ir.Node]*ssa.Value

	// fwdVars are variables that are used before they are defined in the current block.
	// This map exists just to coalesce multiple references into a single FwdRef op.
	// *Node is the unique identifier (an ONAME Node) for the variable.
	fwdVars map[ir.Node]*ssa.Value

	// all defined variables at the end of each block. Indexed by block ID.
	defvars []map[ir.Node]*ssa.Value

	// addresses of PPARAM and PPARAMOUT variables on the stack.
	decladdrs map[*ir.Name]*ssa.Value

	// starting values. Memory, stack pointer, and globals pointer
	startmem *ssa.Value
	sp       *ssa.Value
	sb       *ssa.Value
	// value representing address of where deferBits autotmp is stored
	deferBitsAddr *ssa.Value
	deferBitsTemp *ir.Name

	// line number stack. The current line number is top of stack
	line []src.XPos
	// the last line number processed; it may have been popped
	lastPos src.XPos

	// list of panic calls by function name and line number.
	// Used to deduplicate panic calls.
	panics map[funcLine]*ssa.Block

	cgoUnsafeArgs       bool
	hasdefer            bool // whether the function contains a defer statement
	softFloat           bool
	hasOpenDefers       bool // whether we are doing open-coded defers
	checkPtrEnabled     bool // whether to insert checkptr instrumentation
	instrumentEnterExit bool // whether to instrument function enter/exit
	instrumentMemory    bool // whether to instrument memory operations

	// If doing open-coded defers, list of info about the defer calls in
	// scanning order. Hence, at exit we should run these defers in reverse
	// order of this list
	openDefers []*openDeferInfo
	// For open-coded defers, this is the beginning and end blocks of the last
	// defer exit code that we have generated so far. We use these to share
	// code between exits if the shareDeferExits option (disabled by default)
	// is on.
	lastDeferExit       *ssa.Block // Entry block of last defer exit code we generated
	lastDeferFinalBlock *ssa.Block // Final block of last defer exit code we generated
	lastDeferCount      int        // Number of defers encountered at that point

	prevCall *ssa.Value // the previous call; use this to tie results to the call op.

	// List of allocations in the current block that are still pending.
	// They are all (OffPtr (Select0 (runtime call))) and have the correct types,
	// but the offsets are not set yet, and the type of the runtime call is also not final.
	pendingHeapAllocations []*ssa.Value

	// First argument of append calls that could be stack allocated.
	appendTargets map[ir.Node]bool

	// Block starting position, indexed by block id.
	blockStarts []src.XPos

	// Information for stack allocation. Indexed by the first argument
	// to an append call. Normally a slice-typed variable, but not always.
	backingStores map[ir.Node]*backingStoreInfo
}

type backingStoreInfo struct {
	// Size of backing store array (in elements)
	K int64
	// Stack-allocated backing store variable.
	store *ir.Name
	// Dynamic boolean variable marking the fact that we used this backing store.
	used *ir.Name
	// Have we used this variable statically yet? This is just a hint
	// to avoid checking the dynamic variable if the answer is obvious.
	// (usedStatic == true implies used == true)
	usedStatic bool
}

type funcLine struct {
	f    *obj.LSym
	base *src.PosBase
	line uint
}

type ssaLabel struct {
	target         *ssa.Block // block identified by this label
	breakTarget    *ssa.Block // block to break to in control flow node identified by this label
	continueTarget *ssa.Block // block to continue to in control flow node identified by this label
}

// label returns the label associated with sym, creating it if necessary.
func (s *state) label(sym *types.Sym) *ssaLabel {
	lab := s.labels[sym.Name]
	if lab == nil {
		lab = new(ssaLabel)
		s.labels[sym.Name] = lab
	}
	return lab
}

func (s *state) Logf(msg string, args ...any) { s.f.Logf(msg, args...) }
func (s *state) Log() bool                    { return s.f.Log() }
func (s *state) Fatalf(msg string, args ...any) {
	s.f.Frontend().Fatalf(s.peekPos(), msg, args...)
}
func (s *state) Warnl(pos src.XPos, msg string, args ...any) { s.f.Warnl(pos, msg, args...) }
func (s *state) Debug_checknil() bool                        { return s.f.Frontend().Debug_checknil() }

func ssaMarker(name string) *ir.Name {
	return ir.NewNameAt(base.Pos, &types.Sym{Name: name}, nil)
}

var (
	// marker node for the memory variable
	memVar = ssaMarker("mem")

	// marker nodes for temporary variables
	ptrVar       = ssaMarker("ptr")
	lenVar       = ssaMarker("len")
	capVar       = ssaMarker("cap")
	typVar       = ssaMarker("typ")
	okVar        = ssaMarker("ok")
	deferBitsVar = ssaMarker("deferBits")
	hashVar      = ssaMarker("hash")
)

// startBlock sets the current block we're generating code in to b.
func (s *state) startBlock(b *ssa.Block) {
	if s.curBlock != nil {
		s.Fatalf("starting block %v when block %v has not ended", b, s.curBlock)
	}
	s.curBlock = b
	s.vars = map[ir.Node]*ssa.Value{}
	clear(s.fwdVars)
	for len(s.blockStarts) <= int(b.ID) {
		s.blockStarts = append(s.blockStarts, src.NoXPos)
	}
}

// endBlock marks the end of generating code for the current block.
// Returns the (former) current block. Returns nil if there is no current
// block, i.e. if no code flows to the current execution point.
func (s *state) endBlock() *ssa.Block {
	b := s.curBlock
	if b == nil {
		return nil
	}

	s.flushPendingHeapAllocations()

	for len(s.defvars) <= int(b.ID) {
		s.defvars = append(s.defvars, nil)
	}
	s.defvars[b.ID] = s.vars
	s.curBlock = nil
	s.vars = nil
	if b.LackingPos() {
		// Empty plain blocks get the line of their successor (handled after all blocks created),
		// except for increment blocks in For statements (handled in ssa conversion of OFOR),
		// and for blocks ending in GOTO/BREAK/CONTINUE.
		b.Pos = src.NoXPos
	} else {
		b.Pos = s.lastPos
		if s.blockStarts[b.ID] == src.NoXPos {
			s.blockStarts[b.ID] = s.lastPos
		}
	}
	return b
}

// pushLine pushes a line number on the line number stack.
func (s *state) pushLine(line src.XPos) {
	if !line.IsKnown() {
		// the frontend may emit node with line number missing,
		// use the parent line number in this case.
		line = s.peekPos()
		if base.Flag.K != 0 {
			base.Warn("buildssa: unknown position (line 0)")
		}
	} else {
		s.lastPos = line
	}
	// The first position we see for a new block is its starting position
	// (the line number for its phis, if any).
	if b := s.curBlock; b != nil && s.blockStarts[b.ID] == src.NoXPos {
		s.blockStarts[b.ID] = line
	}

	s.line = append(s.line, line)
}

// popLine pops the top of the line number stack.
func (s *state) popLine() {
	s.line = s.line[:len(s.line)-1]
}

// peekPos peeks the top of the line number stack.
func (s *state) peekPos() src.XPos {
	return s.line[len(s.line)-1]
}

// newValue0 adds a new value with no arguments to the current block.
func (s *state) newValue0(op ssa.Op, t *types.Type) *ssa.Value {
	return s.curBlock.NewValue0(s.peekPos(), op, t)
}

// newValue0A adds a new value with no arguments and an aux value to the current block.
func (s *state) newValue0A(op ssa.Op, t *types.Type, aux ssa.Aux) *ssa.Value {
	return s.curBlock.NewValue0A(s.peekPos(), op, t, aux)
}

// newValue0I adds a new value with no arguments and an auxint value to the current block.
func (s *state) newValue0I(op ssa.Op, t *types.Type, auxint int64) *ssa.Value {
	return s.curBlock.NewValue0I(s.peekPos(), op, t, auxint)
}

// newValue1 adds a new value with one argument to the current block.
func (s *state) newValue1(op ssa.Op, t *types.Type, arg *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue1(s.peekPos(), op, t, arg)
}

// newValue1A adds a new value with one argument and an aux value to the current block.
func (s *state) newValue1A(op ssa.Op, t *types.Type, aux ssa.Aux, arg *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue1A(s.peekPos(), op, t, aux, arg)
}

// newValue1Apos adds a new value with one argument and an aux value to the current block.
// isStmt determines whether the created values may be a statement or not
// (i.e., false means never, yes means maybe).
func (s *state) newValue1Apos(op ssa.Op, t *types.Type, aux ssa.Aux, arg *ssa.Value, isStmt bool) *ssa.Value {
	if isStmt {
		return s.curBlock.NewValue1A(s.peekPos(), op, t, aux, arg)
	}
	return s.curBlock.NewValue1A(s.peekPos().WithNotStmt(), op, t, aux, arg)
}

// newValue1I adds a new value with one argument and an auxint value to the current block.
func (s *state) newValue1I(op ssa.Op, t *types.Type, aux int64, arg *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue1I(s.peekPos(), op, t, aux, arg)
}

// newValue2 adds a new value with two arguments to the current block.
func (s *state) newValue2(op ssa.Op, t *types.Type, arg0, arg1 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue2(s.peekPos(), op, t, arg0, arg1)
}

// newValue2A adds a new value with two arguments and an aux value to the current block.
func (s *state) newValue2A(op ssa.Op, t *types.Type, aux ssa.Aux, arg0, arg1 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue2A(s.peekPos(), op, t, aux, arg0, arg1)
}

// newValue2Apos adds a new value with two arguments and an aux value to the current block.
// isStmt determines whether the created values may be a statement or not
// (i.e., false means never, yes means maybe).
func (s *state) newValue2Apos(op ssa.Op, t *types.Type, aux ssa.Aux, arg0, arg1 *ssa.Value, isStmt bool) *ssa.Value {
	if isStmt {
		return s.curBlock.NewValue2A(s.peekPos(), op, t, aux, arg0, arg1)
	}
	return s.curBlock.NewValue2A(s.peekPos().WithNotStmt(), op, t, aux, arg0, arg1)
}

// newValue2I adds a new value with two arguments and an auxint value to the current block.
func (s *state) newValue2I(op ssa.Op, t *types.Type, aux int64, arg0, arg1 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue2I(s.peekPos(), op, t, aux, arg0, arg1)
}

// newValue3 adds a new value with three arguments to the current block.
func (s *state) newValue3(op ssa.Op, t *types.Type, arg0, arg1, arg2 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue3(s.peekPos(), op, t, arg0, arg1, arg2)
}

// newValue3I adds a new value with three arguments and an auxint value to the current block.
func (s *state) newValue3I(op ssa.Op, t *types.Type, aux int64, arg0, arg1, arg2 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue3I(s.peekPos(), op, t, aux, arg0, arg1, arg2)
}

// newValue3A adds a new value with three arguments and an aux value to the current block.
func (s *state) newValue3A(op ssa.Op, t *types.Type, aux ssa.Aux, arg0, arg1, arg2 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue3A(s.peekPos(), op, t, aux, arg0, arg1, arg2)
}

// newValue3Apos adds a new value with three arguments and an aux value to the current block.
// isStmt determines whether the created values may be a statement or not
// (i.e., false means never, yes means maybe).
func (s *state) newValue3Apos(op ssa.Op, t *types.Type, aux ssa.Aux, arg0, arg1, arg2 *ssa.Value, isStmt bool) *ssa.Value {
	if isStmt {
		return s.curBlock.NewValue3A(s.peekPos(), op, t, aux, arg0, arg1, arg2)
	}
	return s.curBlock.NewValue3A(s.peekPos().WithNotStmt(), op, t, aux, arg0, arg1, arg2)
}

// newValue4 adds a new value with four arguments to the current block.
func (s *state) newValue4(op ssa.Op, t *types.Type, arg0, arg1, arg2, arg3 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue4(s.peekPos(), op, t, arg0, arg1, arg2, arg3)
}

// newValue4A adds a new value with four arguments and an aux value to the current block.
func (s *state) newValue4A(op ssa.Op, t *types.Type, aux ssa.Aux, arg0, arg1, arg2, arg3 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue4A(s.peekPos(), op, t, aux, arg0, arg1, arg2, arg3)
}

// newValue4I adds a new value with four arguments and an auxint value to the current block.
func (s *state) newValue4I(op ssa.Op, t *types.Type, aux int64, arg0, arg1, arg2, arg3 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue4I(s.peekPos(), op, t, aux, arg0, arg1, arg2, arg3)
}

func (s *state) entryBlock() *ssa.Block {
	b := s.f.Entry
	if base.Flag.N > 0 && s.curBlock != nil {
		// If optimizations are off, allocate in current block instead. Since with -N
		// we're not doing the CSE or tighten passes, putting lots of stuff in the
		// entry block leads to O(n^2) entries in the live value map during regalloc.
		// See issue 45897.
		b = s.curBlock
	}
	return b
}

// entryNewValue0 adds a new value with no arguments to the entry block.
func (s *state) entryNewValue0(op ssa.Op, t *types.Type) *ssa.Value {
	return s.entryBlock().NewValue0(src.NoXPos, op, t)
}

// entryNewValue0A adds a new value with no arguments and an aux value to the entry block.
func (s *state) entryNewValue0A(op ssa.Op, t *types.Type, aux ssa.Aux) *ssa.Value {
	return s.entryBlock().NewValue0A(src.NoXPos, op, t, aux)
}

// entryNewValue1 adds a new value with one argument to the entry block.
func (s *state) entryNewValue1(op ssa.Op, t *types.Type, arg *ssa.Value) *ssa.Value {
	return s.entryBlock().NewValue1(src.NoXPos, op, t, arg)
}

// entryNewValue1I adds a new value with one argument and an auxint value to the entry block.
func (s *state) entryNewValue1I(op ssa.Op, t *types.Type, auxint int64, arg *ssa.Value) *ssa.Value {
	return s.entryBlock().NewValue1I(src.NoXPos, op, t, auxint, arg)
}

// entryNewValue1A adds a new value with one argument and an aux value to the entry block.
func (s *state) entryNewValue1A(op ssa.Op, t *types.Type, aux ssa.Aux, arg *ssa.Value) *ssa.Value {
	return s.entryBlock().NewValue1A(src.NoXPos, op, t, aux, arg)
}

// entryNewValue2 adds a new value with two arguments to the entry block.
func (s *state) entryNewValue2(op ssa.Op, t *types.Type, arg0, arg1 *ssa.Value) *ssa.Value {
	return s.entryBlock().NewValue2(src.NoXPos, op, t, arg0, arg1)
}

// entryNewValue2A adds a new value with two arguments and an aux value to the entry block.
func (s *state) entryNewValue2A(op ssa.Op, t *types.Type, aux ssa.Aux, arg0, arg1 *ssa.Value) *ssa.Value {
	return s.entryBlock().NewValue2A(src.NoXPos, op, t, aux, arg0, arg1)
}

// const* routines add a new const value to the entry block.
func (s *state) constSlice(t *types.Type) *ssa.Value {
	return s.f.ConstSlice(t)
}
func (s *state) constInterface(t *types.Type) *ssa.Value {
	return s.f.ConstInterface(t)
}
func (s *state) constNil(t *types.Type) *ssa.Value { return s.f.ConstNil(t) }
func (s *state) constEmptyString(t *types.Type) *ssa.Value {
	return s.f.ConstEmptyString(t)
}
func (s *state) constBool(c bool) *ssa.Value {
	return s.f.ConstBool(types.Types[types.TBOOL], c)
}
func (s *state) constInt8(t *types.Type, c int8) *ssa.Value {
	return s.f.ConstInt8(t, c)
}
func (s *state) constInt16(t *types.Type, c int16) *ssa.Value {
	return s.f.ConstInt16(t, c)
}
func (s *state) constInt32(t *types.Type, c int32) *ssa.Value {
	return s.f.ConstInt32(t, c)
}
func (s *state) constInt64(t *types.Type, c int64) *ssa.Value {
	return s.f.ConstInt64(t, c)
}
func (s *state) constFloat32(t *types.Type, c float64) *ssa.Value {
	return s.f.ConstFloat32(t, c)
}
func (s *state) constFloat64(t *types.Type, c float64) *ssa.Value {
	return s.f.ConstFloat64(t, c)
}
func (s *state) constInt(t *types.Type, c int64) *ssa.Value {
	if s.config.PtrSize == 8 {
		return s.constInt64(t, c)
	}
	if int64(int32(c)) != c {
		s.Fatalf("integer constant too big %d", c)
	}
	return s.constInt32(t, int32(c))
}

// newValueOrSfCall* are wrappers around newValue*, which may create a call to a
// soft-float runtime function instead (when emitting soft-float code).
func (s *state) newValueOrSfCall1(op ssa.Op, t *types.Type, arg *ssa.Value) *ssa.Value {
	if s.softFloat {
		if c, ok := s.sfcall(op, arg); ok {
			return c
		}
	}
	return s.newValue1(op, t, arg)
}
func (s *state) newValueOrSfCall2(op ssa.Op, t *types.Type, arg0, arg1 *ssa.Value) *ssa.Value {
	if s.softFloat {
		if c, ok := s.sfcall(op, arg0, arg1); ok {
			return c
		}
	}
	return s.newValue2(op, t, arg0, arg1)
}

type instrumentKind uint8

const (
	instrumentRead = iota
	instrumentWrite
	instrumentMove
)

func (s *state) instrument(t *types.Type, addr *ssa.Value, kind instrumentKind) {
	s.instrument2(t, addr, nil, kind)
}

// instrumentFields instruments a read/write operation on addr.
// If it is instrumenting for MSAN or ASAN and t is a struct type, it instruments
// operation for each field, instead of for the whole struct.
func (s *state) instrumentFields(t *types.Type, addr *ssa.Value, kind instrumentKind) {
	if !(base.Flag.MSan || base.Flag.ASan) || !isStructNotSIMD(t) {
		s.instrument(t, addr, kind)
		return
	}
	for _, f := range t.Fields() {
		if f.Sym.IsBlank() {
			continue
		}
		offptr := s.newValue1I(ssa.OpOffPtr, types.NewPtr(f.Type), f.Offset, addr)
		s.instrumentFields(f.Type, offptr, kind)
	}
}

func (s *state) instrumentMove(t *types.Type, dst, src *ssa.Value) {
	if base.Flag.MSan {
		s.instrument2(t, dst, src, instrumentMove)
	} else {
		s.instrument(t, src, instrumentRead)
		s.instrument(t, dst, instrumentWrite)
	}
}

func (s *state) instrument2(t *types.Type, addr, addr2 *ssa.Value, kind instrumentKind) {
	if !s.instrumentMemory {
		return
	}

	w := t.Size()
	if w == 0 {
		return // can't race on zero-sized things
	}

	if ssa.IsSanitizerSafeAddr(addr) {
		return
	}

	var fn *obj.LSym
	needWidth := false

	if addr2 != nil && kind != instrumentMove {
		panic("instrument2: non-nil addr2 for non-move instrumentation")
	}

	if base.Flag.MSan {
		switch kind {
		case instrumentRead:
			fn = ir.Syms.Msanread
		case instrumentWrite:
			fn = ir.Syms.Msanwrite
		case instrumentMove:
			fn = ir.Syms.Msanmove
		default:
			panic("unreachable")
		}
		needWidth = true
	} else if base.Flag.Race && t.NumComponents(types.CountBlankFields) > 1 {
		// for composite objects we have to write every address
		// because a write might happen to any subobject.
		// composites with only one element don't have subobjects, though.
		switch kind {
		case instrumentRead:
			fn = ir.Syms.Racereadrange
		case instrumentWrite:
			fn = ir.Syms.Racewriterange
		default:
			panic("unreachable")
		}
		needWidth = true
	} else if base.Flag.Race {
		// for non-composite objects we can write just the start
		// address, as any write must write the first byte.
		switch kind {
		case instrumentRead:
			fn = ir.Syms.Raceread
		case instrumentWrite:
			fn = ir.Syms.Racewrite
		default:
			panic("unreachable")
		}
	} else if base.Flag.ASan {
		switch kind {
		case instrumentRead:
			fn = ir.Syms.Asanread
		case instrumentWrite:
			fn = ir.Syms.Asanwrite
		default:
			panic("unreachable")
		}
		needWidth = true
	} else {
		panic("unreachable")
	}

	args := []*ssa.Value{addr}
	if addr2 != nil {
		args = append(args, addr2)
	}
	if needWidth {
		args = append(args, s.constInt(types.Types[types.TUINTPTR], w))
	}
	s.rtcall(fn, true, nil, args...)
}

func (s *state) load(t *types.Type, src *ssa.Value) *ssa.Value {
	s.instrumentFields(t, src, instrumentRead)
	return s.rawLoad(t, src)
}

func (s *state) rawLoad(t *types.Type, src *ssa.Value) *ssa.Value {
	return s.newValue2(ssa.OpLoad, t, src, s.mem())
}

func (s *state) store(t *types.Type, dst, val *ssa.Value) {
	s.vars[memVar] = s.newValue3A(ssa.OpStore, types.TypeMem, t, dst, val, s.mem())
}

func (s *state) zero(t *types.Type, dst *ssa.Value) {
	s.instrument(t, dst, instrumentWrite)
	store := s.newValue2I(ssa.OpZero, types.TypeMem, t.Size(), dst, s.mem())
	store.Aux = t
	s.vars[memVar] = store
}

func (s *state) move(t *types.Type, dst, src *ssa.Value) {
	s.moveWhichMayOverlap(t, dst, src, false)
}
func (s *state) moveWhichMayOverlap(t *types.Type, dst, src *ssa.Value, mayOverlap bool) {
	s.instrumentMove(t, dst, src)
	if mayOverlap && t.IsArray() && t.NumElem() > 1 && !ssa.IsInlinableMemmove(dst, src, t.Size(), s.f.Config) {
		// Normally, when moving Go values of type T from one location to another,
		// we don't need to worry about partial overlaps. The two Ts must either be
		// in disjoint (nonoverlapping) memory or in exactly the same location.
		// There are 2 cases where this isn't true:
		//  1) Using unsafe you can arrange partial overlaps.
		//  2) Since Go 1.17, you can use a cast from a slice to a ptr-to-array.
		//     https://go.dev/ref/spec#Conversions_from_slice_to_array_pointer
		//     This feature can be used to construct partial overlaps of array types.
		//       var a [3]int
		//       p := (*[2]int)(a[:])
		//       q := (*[2]int)(a[1:])
		//       *p = *q
		// We don't care about solving 1. Or at least, we haven't historically
		// and no one has complained.
		// For 2, we need to ensure that if there might be partial overlap,
		// then we can't use OpMove; we must use memmove instead.
		// (memmove handles partial overlap by copying in the correct
		// direction. OpMove does not.)
		//
		// Note that we have to be careful here not to introduce a call when
		// we're marshaling arguments to a call or unmarshaling results from a call.
		// Cases where this is happening must pass mayOverlap to false.
		// (Currently this only happens when unmarshaling results of a call.)
		if t.HasPointers() {
			s.rtcall(ir.Syms.Typedmemmove, true, nil, s.reflectType(t), dst, src)
			// We would have otherwise implemented this move with straightline code,
			// including a write barrier. Pretend we issue a write barrier here,
			// so that the write barrier tests work. (Otherwise they'd need to know
			// the details of IsInlineableMemmove.)
			s.curfn.SetWBPos(s.peekPos())
		} else {
			s.rtcall(ir.Syms.Memmove, true, nil, dst, src, s.constInt(types.Types[types.TUINTPTR], t.Size()))
		}
		ssa.LogLargeCopy(s.f.Name, s.peekPos(), t.Size())
		return
	}
	store := s.newValue3I(ssa.OpMove, types.TypeMem, t.Size(), dst, src, s.mem())
	store.Aux = t
	s.vars[memVar] = store
}

// stmtList converts the statement list n to SSA and adds it to s.
func (s *state) stmtList(l ir.Nodes) {
	for _, n := range l {
		s.stmt(n)
	}
}

func peelConvNop(n ir.Node) ir.Node {
	if n == nil {
		return n
	}
	for n.Op() == ir.OCONVNOP {
		n = n.(*ir.ConvExpr).X
	}
	return n
}

// stmt converts the statement n to SSA and adds it to s.
func (s *state) stmt(n ir.Node) {
	s.pushLine(n.Pos())
	defer s.popLine()

	// If s.curBlock is nil, and n isn't a label (which might have an associated goto somewhere),
	// then this code is dead. Stop here.
	if s.curBlock == nil && n.Op() != ir.OLABEL {
		return
	}

	s.stmtList(n.Init())
	switch n.Op() {

	case ir.OBLOCK:
		n := n.(*ir.BlockStmt)
		s.stmtList(n.List)

	case ir.OFALL: // no-op

	// Expression statements
	case ir.OCALLFUNC:
		n := n.(*ir.CallExpr)
		if ir.IsIntrinsicCall(n) {
			s.intrinsicCall(n)
			return
		}
		fallthrough

	case ir.OCALLINTER:
		n := n.(*ir.CallExpr)
		s.callResult(n, callNormal)
		if n.Op() == ir.OCALLFUNC && n.Fun.Op() == ir.ONAME && n.Fun.(*ir.Name).Class == ir.PFUNC {
			if fn := n.Fun.Sym().Name; base.Flag.CompilingRuntime && fn == "throw" ||
				n.Fun.Sym().Pkg == ir.Pkgs.Runtime &&
					(fn == "throwinit" || fn == "gopanic" || fn == "panicwrap" || fn == "block" ||
						fn == "panicmakeslicelen" || fn == "panicmakeslicecap" || fn == "panicunsafeslicelen" ||
						fn == "panicunsafeslicenilptr" || fn == "panicunsafestringlen" || fn == "panicunsafestringnilptr" ||
						fn == "panicrangestate") {
				m := s.mem()
				b := s.endBlock()
				b.Kind = ssa.BlockExit
				b.SetControl(m)
				// TODO: never rewrite OPANIC to OCALLFUNC in the
				// first place. Need to wait until all backends
				// go through SSA.
			}
		}
	case ir.ODEFER:
		n := n.(*ir.GoDeferStmt)
		if base.Debug.Defer > 0 {
			var defertype string
			if s.hasOpenDefers {
				defertype = "open-coded"
			} else if n.Esc() == ir.EscNever {
				defertype = "stack-allocated"
			} else {
				defertype = "heap-allocated"
			}
			base.WarnfAt(n.Pos(), "%s defer", defertype)
		}
		if s.hasOpenDefers {
			s.openDeferRecord(n.Call.(*ir.CallExpr))
		} else {
			d := callDefer
			if n.Esc() == ir.EscNever && n.DeferAt == nil {
				d = callDeferStack
			}
			s.call(n.Call.(*ir.CallExpr), d, false, n.DeferAt)
		}
	case ir.OGO:
		n := n.(*ir.GoDeferStmt)
		s.callResult(n.Call.(*ir.CallExpr), callGo)

	case ir.OAS2DOTTYPE:
		n := n.(*ir.AssignListStmt)
		var res, resok *ssa.Value
		if n.Rhs[0].Op() == ir.ODOTTYPE2 {
			res, resok = s.dottype(n.Rhs[0].(*ir.TypeAssertExpr), true)
		} else {
			res, resok = s.dynamicDottype(n.Rhs[0].(*ir.DynamicTypeAssertExpr), true)
		}
		deref := false
		if !ssa.CanSSA(n.Rhs[0].Type()) {
			if res.Op != ssa.OpLoad {
				s.Fatalf("dottype of non-load")
			}
			mem := s.mem()
			if res.Args[1] != mem {
				s.Fatalf("memory no longer live from 2-result dottype load")
			}
			deref = true
			res = res.Args[0]
		}
		s.assign(n.Lhs[0], res, deref, 0)
		s.assign(n.Lhs[1], resok, false, 0)
		return

	case ir.OAS2FUNC:
		// We come here only when it is an intrinsic call returning two values.
		n := n.(*ir.AssignListStmt)
		call := n.Rhs[0].(*ir.CallExpr)
		if !ir.IsIntrinsicCall(call) {
			s.Fatalf("non-intrinsic AS2FUNC not expanded %v", call)
		}
		v := s.intrinsicCall(call)
		v1 := s.newValue1(ssa.OpSelect0, n.Lhs[0].Type(), v)
		v2 := s.newValue1(ssa.OpSelect1, n.Lhs[1].Type(), v)
		s.assign(n.Lhs[0], v1, false, 0)
		s.assign(n.Lhs[1], v2, false, 0)
		return

	case ir.ODCL:
		n := n.(*ir.Decl)
		if v := n.X; v.Esc() == ir.EscHeap {
			s.newHeapaddr(v)
		}

	case ir.OLABEL:
		n := n.(*ir.LabelStmt)
		sym := n.Label
		if sym.IsBlank() {
			// Nothing to do because the label isn't targetable. See issue 52278.
			break
		}
		lab := s.label(sym)

		// The label might already have a target block via a goto.
		if lab.target == nil {
			lab.target = s.f.NewBlock(ssa.BlockPlain)
		}

		// Go to that label.
		// (We pretend "label:" is preceded by "goto label", unless the predecessor is unreachable.)
		if s.curBlock != nil {
			b := s.endBlock()
			b.AddEdgeTo(lab.target)
		}
		s.startBlock(lab.target)

	case ir.OGOTO:
		n := n.(*ir.BranchStmt)
		sym := n.Label

		lab := s.label(sym)
		if lab.target == nil {
			lab.target = s.f.NewBlock(ssa.BlockPlain)
		}

		b := s.endBlock()
		b.Pos = s.lastPos.WithIsStmt() // Do this even if b is an empty block.
		b.AddEdgeTo(lab.target)

	case ir.OAS:
		n := n.(*ir.AssignStmt)
		if n.X == n.Y && n.X.Op() == ir.ONAME {
			// An x=x assignment. No point in doing anything
			// here. In addition, skipping this assignment
			// prevents generating:
			//   VARDEF x
			//   COPY x -> x
			// which is bad because x is incorrectly considered
			// dead before the vardef. See issue #14904.
			return
		}

		// mayOverlap keeps track of whether the LHS and RHS might
		// refer to partially overlapping memory. Partial overlapping can
		// only happen for arrays, see the comment in moveWhichMayOverlap.
		//
		// If both sides of the assignment are not dereferences, then partial
		// overlap can't happen. Partial overlap can only occur only when the
		// arrays referenced are strictly smaller parts of the same base array.
		// If one side of the assignment is a full array, then partial overlap
		// can't happen. (The arrays are either disjoint or identical.)
		ny := peelConvNop(n.Y)
		mayOverlap := n.X.Op() == ir.ODEREF && (n.Y != nil && ny.Op() == ir.ODEREF)
		if ny != nil && ny.Op() == ir.ODEREF {
			p := peelConvNop(ny.(*ir.StarExpr).X)
			if p.Op() == ir.OSPTR && p.(*ir.UnaryExpr).X.Type().IsString() {
				// Pointer fields of strings point to unmodifiable memory.
				// That memory can't overlap with the memory being written.
				mayOverlap = false
			}
		}

		// Evaluate RHS.
		rhs := n.Y
		if rhs != nil {
			switch rhs.Op() {
			case ir.OSTRUCTLIT, ir.OARRAYLIT, ir.OSLICELIT:
				// All literals with nonzero fields have already been
				// rewritten during walk. Any that remain are just T{}
				// or equivalents. Use the zero value.
				if !ir.IsZero(rhs) {
					s.Fatalf("literal with nonzero value in SSA: %v", rhs)
				}
				rhs = nil
			case ir.OAPPEND:
				rhs := rhs.(*ir.CallExpr)
				// Check whether we're writing the result of an append back to the same slice.
				// If so, we handle it specially to avoid write barriers on the fast
				// (non-growth) path.
				if !ir.SameSafeExpr(n.X, rhs.Args[0]) || base.Flag.N != 0 {
					break
				}
				// If the slice can be SSA'd, it'll be on the stack,
				// so there will be no write barriers,
				// so there's no need to attempt to prevent them.
				if s.canSSA(n.X) {
					if base.Debug.Append > 0 { // replicating old diagnostic message
						base.WarnfAt(n.Pos(), "append: len-only update (in local slice)")
					}
					break
				}
				if base.Debug.Append > 0 {
					base.WarnfAt(n.Pos(), "append: len-only update")
				}
				s.append(rhs, true)
				return
			}
		}

		if ir.IsBlank(n.X) {
			// _ = rhs
			// Just evaluate rhs for side-effects.
			if rhs != nil {
				s.expr(rhs)
			}
			return
		}

		var t *types.Type
		if n.Y != nil {
			t = n.Y.Type()
		} else {
			t = n.X.Type()
		}

		var r *ssa.Value
		deref := !ssa.CanSSA(t)
		if deref {
			if rhs == nil {
				r = nil // Signal assign to use OpZero.
			} else {
				r = s.addr(rhs)
			}
		} else {
			if rhs == nil {
				r = s.zeroVal(t)
			} else {
				r = s.expr(rhs)
			}
		}

		var skip skipMask
		if rhs != nil && (rhs.Op() == ir.OSLICE || rhs.Op() == ir.OSLICE3 || rhs.Op() == ir.OSLICESTR) && ir.SameSafeExpr(rhs.(*ir.SliceExpr).X, n.X) {
			// We're assigning a slicing operation back to its source.
			// Don't write back fields we aren't changing. See issue #14855.
			rhs := rhs.(*ir.SliceExpr)
			i, j, k := rhs.Low, rhs.High, rhs.Max
			if i != nil && (i.Op() == ir.OLITERAL && i.Val().Kind() == constant.Int && ir.Int64Val(i) == 0) {
				// [0:...] is the same as [:...]
				i = nil
			}
			// TODO: detect defaults for len/cap also.
			// Currently doesn't really work because (*p)[:len(*p)] appears here as:
			//    tmp = len(*p)
			//    (*p)[:tmp]
			// if j != nil && (j.Op == OLEN && SameSafeExpr(j.Left, n.Left)) {
			//      j = nil
			// }
			// if k != nil && (k.Op == OCAP && SameSafeExpr(k.Left, n.Left)) {
			//      k = nil
			// }
			if i == nil {
				skip |= skipPtr
				if j == nil {
					skip |= skipLen
				}
				if k == nil {
					skip |= skipCap
				}
			}
		}

		s.assignWhichMayOverlap(n.X, r, deref, skip, mayOverlap)

	case ir.OIF:
		n := n.(*ir.IfStmt)
		if ir.IsConst(n.Cond, constant.Bool) {
			s.stmtList(n.Cond.Init())
			if ir.BoolVal(n.Cond) {
				s.stmtList(n.Body)
			} else {
				s.stmtList(n.Else)
			}
			break
		}

		bEnd := s.f.NewBlock(ssa.BlockPlain)
		var likely int8
		if n.Likely {
			likely = 1
		}
		var bThen *ssa.Block
		if len(n.Body) != 0 {
			bThen = s.f.NewBlock(ssa.BlockPlain)
		} else {
			bThen = bEnd
		}
		var bElse *ssa.Block
		if len(n.Else) != 0 {
			bElse = s.f.NewBlock(ssa.BlockPlain)
		} else {
			bElse = bEnd
		}
		s.condBranch(n.Cond, bThen, bElse, likely)

		if len(n.Body) != 0 {
			s.startBlock(bThen)
			s.stmtList(n.Body)
			if b := s.endBlock(); b != nil {
				b.AddEdgeTo(bEnd)
			}
		}
		if len(n.Else) != 0 {
			s.startBlock(bElse)
			s.stmtList(n.Else)
			if b := s.endBlock(); b != nil {
				b.AddEdgeTo(bEnd)
			}
		}
		s.startBlock(bEnd)

	case ir.ORETURN:
		n := n.(*ir.ReturnStmt)
		s.stmtList(n.Results)
		b := s.exit()
		b.Pos = s.lastPos.WithIsStmt()

	case ir.OTAILCALL:
		n := n.(*ir.TailCallStmt)
		s.callResult(n.Call, callTail)
		call := s.mem()
		b := s.endBlock()
		b.Kind = ssa.BlockRetJmp // could use BlockExit. BlockRetJmp is mostly for clarity.
		b.SetControl(call)

	case ir.OCONTINUE, ir.OBREAK:
		n := n.(*ir.BranchStmt)
		var to *ssa.Block
		if n.Label == nil {
			// plain break/continue
			switch n.Op() {
			case ir.OCONTINUE:
				to = s.continueTo
			case ir.OBREAK:
				to = s.breakTo
			}
		} else {
			// labeled break/continue; look up the target
			sym := n.Label
			lab := s.label(sym)
			switch n.Op() {
			case ir.OCONTINUE:
				to = lab.continueTarget
			case ir.OBREAK:
				to = lab.breakTarget
			}
		}

		b := s.endBlock()
		b.Pos = s.lastPos.WithIsStmt() // Do this even if b is an empty block.
		b.AddEdgeTo(to)

	case ir.OFOR:
		// OFOR: for Ninit; Left; Right { Nbody }
		// cond (Left); body (Nbody); incr (Right)
		n := n.(*ir.ForStmt)
		base.Assert(!n.DistinctVars) // Should all be rewritten before escape analysis
		bCond := s.f.NewBlock(ssa.BlockPlain)
		bBody := s.f.NewBlock(ssa.BlockPlain)
		bIncr := s.f.NewBlock(ssa.BlockPlain)
		bEnd := s.f.NewBlock(ssa.BlockPlain)

		// ensure empty for loops have correct position; issue #30167
		bBody.Pos = n.Pos()

		// first, jump to condition test
		b := s.endBlock()
		b.AddEdgeTo(bCond)

		// generate code to test condition
		s.startBlock(bCond)
		if n.Cond != nil {
			s.condBranch(n.Cond, bBody, bEnd, 1)
		} else {
			b := s.endBlock()
			b.Kind = ssa.BlockPlain
			b.AddEdgeTo(bBody)
		}

		// set up for continue/break in body
		prevContinue := s.continueTo
		prevBreak := s.breakTo
		s.continueTo = bIncr
		s.breakTo = bEnd
		var lab *ssaLabel
		if sym := n.Label; sym != nil {
			// labeled for loop
			lab = s.label(sym)
			lab.continueTarget = bIncr
			lab.breakTarget = bEnd
		}

		// generate body
		s.startBlock(bBody)
		s.stmtList(n.Body)

		// tear down continue/break
		s.continueTo = prevContinue
		s.breakTo = prevBreak
		if lab != nil {
			lab.continueTarget = nil
			lab.breakTarget = nil
		}

		// done with body, goto incr
		if b := s.endBlock(); b != nil {
			b.AddEdgeTo(bIncr)
		}

		// generate incr
		s.startBlock(bIncr)
		if n.Post != nil {
			s.stmt(n.Post)
		}
		if b := s.endBlock(); b != nil {
			b.AddEdgeTo(bCond)
			// It can happen that bIncr ends in a block containing only VARKILL,
			// and that muddles the debugging experience.
			if b.Pos == src.NoXPos {
				b.Pos = bCond.Pos
			}
		}

		s.startBlock(bEnd)

	case ir.OSWITCH, ir.OSELECT:
		// These have been mostly rewritten by the front end into their Nbody fields.
		// Our main task is to correctly hook up any break statements.
		bEnd := s.f.NewBlock(ssa.BlockPlain)

		prevBreak := s.breakTo
		s.breakTo = bEnd
		var sym *types.Sym
		var body ir.Nodes
		if n.Op() == ir.OSWITCH {
			n := n.(*ir.SwitchStmt)
			sym = n.Label
			body = n.Compiled
		} else {
			n := n.(*ir.SelectStmt)
			sym = n.Label
			body = n.Compiled
		}

		var lab *ssaLabel
		if sym != nil {
			// labeled
			lab = s.label(sym)
			lab.breakTarget = bEnd
		}

		// generate body code
		s.stmtList(body)

		s.breakTo = prevBreak
		if lab != nil {
			lab.breakTarget = nil
		}

		// walk adds explicit OBREAK nodes to the end of all reachable code paths.
		// If we still have a current block here, then mark it unreachable.
		if s.curBlock != nil {
			m := s.mem()
			b := s.endBlock()
			b.Kind = ssa.BlockExit
			b.SetControl(m)
		}
		s.startBlock(bEnd)

	case ir.OJUMPTABLE:
		n := n.(*ir.JumpTableStmt)

		// Make blocks we'll need.
		jt := s.f.NewBlock(ssa.BlockJumpTable)
		bEnd := s.f.NewBlock(ssa.BlockPlain)

		// The only thing that needs evaluating is the index we're looking up.
		idx := s.expr(n.Idx)
		unsigned := idx.Type.IsUnsigned()

		// Extend so we can do everything in uintptr arithmetic.
		t := types.Types[types.TUINTPTR]
		idx = s.conv(nil, idx, idx.Type, t)

		// The ending condition for the current block decides whether we'll use
		// the jump table at all.
		// We check that min <= idx <= max and jump around the jump table
		// if that test fails.
		// We implement min <= idx <= max with 0 <= idx-min <= max-min, because
		// we'll need idx-min anyway as the control value for the jump table.
		var min, max uint64
		if unsigned {
			min, _ = constant.Uint64Val(n.Cases[0])
			max, _ = constant.Uint64Val(n.Cases[len(n.Cases)-1])
		} else {
			mn, _ := constant.Int64Val(n.Cases[0])
			mx, _ := constant.Int64Val(n.Cases[len(n.Cases)-1])
			min = uint64(mn)
			max = uint64(mx)
		}
		// Compare idx-min with max-min, to see if we can use the jump table.
		idx = s.newValue2(s.ssaOp(ir.OSUB, t), t, idx, s.uintptrConstant(min))
		width := s.uintptrConstant(max - min)
		cmp := s.newValue2(s.ssaOp(ir.OLE, t), types.Types[types.TBOOL], idx, width)
		b := s.endBlock()
		b.Kind = ssa.BlockIf
		b.SetControl(cmp)
		b.AddEdgeTo(jt)             // in range - use jump table
		b.AddEdgeTo(bEnd)           // out of range - no case in the jump table will trigger
		b.Likely = ssa.BranchLikely // TODO: assumes missing the table entirely is unlikely. True?

		// Build jump table block.
		s.startBlock(jt)
		jt.Pos = n.Pos()
		if base.Flag.Cfg.SpectreIndex {
			idx = s.newValue2(ssa.OpSpectreSliceIndex, t, idx, width)
		}
		jt.SetControl(idx)

		// Figure out where we should go for each index in the table.
		table := make([]*ssa.Block, max-min+1)
		for i := range table {
			table[i] = bEnd // default target
		}
		for i := range n.Targets {
			c := n.Cases[i]
			lab := s.label(n.Targets[i])
			if lab.target == nil {
				lab.target = s.f.NewBlock(ssa.BlockPlain)
			}
			var val uint64
			if unsigned {
				val, _ = constant.Uint64Val(c)
			} else {
				vl, _ := constant.Int64Val(c)
				val = uint64(vl)
			}
			// Overwrite the default target.
			table[val-min] = lab.target
		}
		for _, t := range table {
			jt.AddEdgeTo(t)
		}
		s.endBlock()

		s.startBlock(bEnd)

	case ir.OINTERFACESWITCH:
		n := n.(*ir.InterfaceSwitchStmt)
		typs := s.f.Config.Types

		t := s.expr(n.RuntimeType)
		h := s.expr(n.Hash)
		d := s.newValue1A(ssa.OpAddr, typs.BytePtr, n.Descriptor, s.sb)

		// Check the cache first.
		var merge *ssa.Block
		if base.Flag.N == 0 && rtabi.UseInterfaceSwitchCache(Arch.LinkArch.Family) {
			// Note: we can only use the cache if we have the right atomic load instruction.
			// Double-check that here.
			if intrinsics.lookup(Arch.LinkArch.Arch, "internal/runtime/atomic", "Loadp") == nil {
				s.Fatalf("atomic load not available")
			}
			merge = s.f.NewBlock(ssa.BlockPlain)
			cacheHit := s.f.NewBlock(ssa.BlockPlain)
			cacheMiss := s.f.NewBlock(ssa.BlockPlain)
			loopHead := s.f.NewBlock(ssa.BlockPlain)
			loopBody := s.f.NewBlock(ssa.BlockPlain)

			// Pick right size ops.
			var mul, and, add, zext ssa.Op
			if s.config.PtrSize == 4 {
				mul = ssa.OpMul32
				and = ssa.OpAnd32
				add = ssa.OpAdd32
				zext = ssa.OpCopy
			} else {
				mul = ssa.OpMul64
				and = ssa.OpAnd64
				add = ssa.OpAdd64
				zext = ssa.OpZeroExt32to64
			}

			// Load cache pointer out of descriptor, with an atomic load so
			// we ensure that we see a fully written cache.
			atomicLoad := s.newValue2(ssa.OpAtomicLoadPtr, types.NewTuple(typs.BytePtr, types.TypeMem), d, s.mem())
			cache := s.newValue1(ssa.OpSelect0, typs.BytePtr, atomicLoad)
			s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, atomicLoad)

			// Initialize hash variable.
			s.vars[hashVar] = s.newValue1(zext, typs.Uintptr, h)

			// Load mask from cache.
			mask := s.newValue2(ssa.OpLoad, typs.Uintptr, cache, s.mem())
			// Jump to loop head.
			b := s.endBlock()
			b.AddEdgeTo(loopHead)

			// At loop head, get pointer to the cache entry.
			//   e := &cache.Entries[hash&mask]
			s.startBlock(loopHead)
			entries := s.newValue2(ssa.OpAddPtr, typs.UintptrPtr, cache, s.uintptrConstant(uint64(s.config.PtrSize)))
			idx := s.newValue2(and, typs.Uintptr, s.variable(hashVar, typs.Uintptr), mask)
			idx = s.newValue2(mul, typs.Uintptr, idx, s.uintptrConstant(uint64(3*s.config.PtrSize)))
			e := s.newValue2(ssa.OpAddPtr, typs.UintptrPtr, entries, idx)
			//   hash++
			s.vars[hashVar] = s.newValue2(add, typs.Uintptr, s.variable(hashVar, typs.Uintptr), s.uintptrConstant(1))

			// Look for a cache hit.
			//   if e.Typ == t { goto hit }
			eTyp := s.newValue2(ssa.OpLoad, typs.Uintptr, e, s.mem())
			cmp1 := s.newValue2(ssa.OpEqPtr, typs.Bool, t, eTyp)
			b = s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(cmp1)
			b.AddEdgeTo(cacheHit)
			b.AddEdgeTo(loopBody)

			// Look for an empty entry, the tombstone for this hash table.
			//   if e.Typ == nil { goto miss }
			s.startBlock(loopBody)
			cmp2 := s.newValue2(ssa.OpEqPtr, typs.Bool, eTyp, s.constNil(typs.BytePtr))
			b = s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(cmp2)
			b.AddEdgeTo(cacheMiss)
			b.AddEdgeTo(loopHead)

			// On a hit, load the data fields of the cache entry.
			//   Case = e.Case
			//   Itab = e.Itab
			s.startBlock(cacheHit)
			eCase := s.newValue2(ssa.OpLoad, typs.Int, s.newValue1I(ssa.OpOffPtr, typs.IntPtr, s.config.PtrSize, e), s.mem())
			eItab := s.newValue2(ssa.OpLoad, typs.BytePtr, s.newValue1I(ssa.OpOffPtr, typs.BytePtrPtr, 2*s.config.PtrSize, e), s.mem())
			s.assign(n.Case, eCase, false, 0)
			s.assign(n.Itab, eItab, false, 0)
			b = s.endBlock()
			b.AddEdgeTo(merge)

			// On a miss, call into the runtime to get the answer.
			s.startBlock(cacheMiss)
		}

		r := s.rtcall(ir.Syms.InterfaceSwitch, true, []*types.Type{typs.Int, typs.BytePtr}, d, t)
		s.assign(n.Case, r[0], false, 0)
		s.assign(n.Itab, r[1], false, 0)

		if merge != nil {
			// Cache hits merge in here.
			b := s.endBlock()
			b.Kind = ssa.BlockPlain
			b.AddEdgeTo(merge)
			s.startBlock(merge)
		}

	case ir.OCHECKNIL:
		n := n.(*ir.UnaryExpr)
		p := s.expr(n.X)
		_ = s.nilCheck(p)
		// TODO: check that throwing away the nilcheck result is ok.

	case ir.OINLMARK:
		n := n.(*ir.InlineMarkStmt)
		s.newValue1I(ssa.OpInlMark, types.TypeVoid, n.Index, s.mem())

	default:
		s.Fatalf("unhandled stmt %v", n.Op())
	}
}

// If true, share as many open-coded defer exits as possible (with the downside of
// worse line-number information)
const shareDeferExits = false

// exit processes any code that needs to be generated just before returning.
// It returns a BlockRet block that ends the control flow. Its control value
// will be set to the final memory state.
func (s *state) exit() *ssa.Block {
	if s.hasdefer {
		if s.hasOpenDefers {
			if shareDeferExits && s.lastDeferExit != nil && len(s.openDefers) == s.lastDeferCount {
				if s.curBlock.Kind != ssa.BlockPlain {
					panic("Block for an exit should be BlockPlain")
				}
				s.curBlock.AddEdgeTo(s.lastDeferExit)
				s.endBlock()
				return s.lastDeferFinalBlock
			}
			s.openDeferExit()
		} else {
			// Shared deferreturn is assigned the "last" position in the function.
			// The linker picks the first deferreturn call it sees, so this is
			// the only sensible "shared" place.
			// To not-share deferreturn, the protocol would need to be changed
			// so that the call to deferproc-etc would receive the PC offset from
			// the return PC, and the runtime would need to use that instead of
			// the deferreturn retrieved from the pcln information.
			// opendefers would remain a problem, however.
			s.pushLine(s.curfn.Endlineno)
			s.rtcall(ir.Syms.Deferreturn, true, nil)
			s.popLine()
		}
	}

	// Do actual return.
	// These currently turn into self-copies (in many cases).
	resultFields := s.curfn.Type().Results()
	results := make([]*ssa.Value, len(resultFields)+1, len(resultFields)+1)
	// Store SSAable and heap-escaped PPARAMOUT variables back to stack locations.
	for i, f := range resultFields {
		n := f.Nname.(*ir.Name)
		if s.canSSA(n) { // result is in some SSA variable
			if !n.IsOutputParamInRegisters() && n.Type().HasPointers() {
				// We are about to store to the result slot.
				s.vars[memVar] = s.newValue1A(ssa.OpVarDef, types.TypeMem, n, s.mem())
			}
			results[i] = s.variable(n, n.Type())
		} else if !n.OnStack() { // result is actually heap allocated
			// We are about to copy the in-heap result to the result slot.
			if n.Type().HasPointers() {
				s.vars[memVar] = s.newValue1A(ssa.OpVarDef, types.TypeMem, n, s.mem())
			}
			ha := s.expr(n.Heapaddr)
			s.instrumentFields(n.Type(), ha, instrumentRead)
			results[i] = s.newValue2(ssa.OpDereference, n.Type(), ha, s.mem())
		} else { // result is not SSA-able; not escaped, so not on heap, but too large for SSA.
			// Before register ABI this ought to be a self-move, home=dest,
			// With register ABI, it's still a self-move if parameter is on stack (i.e., too big or overflowed)
			// No VarDef, as the result slot is already holding live value.
			results[i] = s.newValue2(ssa.OpDereference, n.Type(), s.addr(n), s.mem())
		}
	}

	// In -race mode, we need to call racefuncexit.
	// Note: This has to happen after we load any heap-allocated results,
	// otherwise races will be attributed to the caller instead.
	if s.instrumentEnterExit {
		s.rtcall(ir.Syms.Racefuncexit, true, nil)
	}

	results[len(results)-1] = s.mem()
	m := s.newValue0(ssa.OpMakeResult, s.f.OwnAux.LateExpansionResultType())
	m.AddArgs(results...)

	b := s.endBlock()
	b.Kind = ssa.BlockRet
	b.SetControl(m)
	if s.hasdefer && s.hasOpenDefers {
		s.lastDeferFinalBlock = b
	}
	return b
}

type opAndType struct {
	op    ir.Op
	etype types.Kind
}

var opToSSA = map[opAndType]ssa.Op{
	{ir.OADD, types.TINT8}:    ssa.OpAdd8,
	{ir.OADD, types.TUINT8}:   ssa.OpAdd8,
	{ir.OADD, types.TINT16}:   ssa.OpAdd16,
	{ir.OADD, types.TUINT16}:  ssa.OpAdd16,
	{ir.OADD, types.TINT32}:   ssa.OpAdd32,
	{ir.OADD, types.TUINT32}:  ssa.OpAdd32,
	{ir.OADD, types.TINT64}:   ssa.OpAdd64,
	{ir.OADD, types.TUINT64}:  ssa.OpAdd64,
	{ir.OADD, types.TFLOAT32}: ssa.OpAdd32F,
	{ir.OADD, types.TFLOAT64}: ssa.OpAdd64F,

	{ir.OSUB, types.TINT8}:    ssa.OpSub8,
	{ir.OSUB, types.TUINT8}:   ssa.OpSub8,
	{ir.OSUB, types.TINT16}:   ssa.OpSub16,
	{ir.OSUB, types.TUINT16}:  ssa.OpSub16,
	{ir.OSUB, types.TINT32}:   ssa.OpSub32,
	{ir.OSUB, types.TUINT32}:  ssa.OpSub32,
	{ir.OSUB, types.TINT64}:   ssa.OpSub64,
	{ir.OSUB, types.TUINT64}:  ssa.OpSub64,
	{ir.OSUB, types.TFLOAT32}: ssa.OpSub32F,
	{ir.OSUB, types.TFLOAT64}: ssa.OpSub64F,

	{ir.ONOT, types.TBOOL}: ssa.OpNot,

	{ir.ONEG, types.TINT8}:    ssa.OpNeg8,
	{ir.ONEG, types.TUINT8}:   ssa.OpNeg8,
	{ir.ONEG, types.TINT16}:   ssa.OpNeg16,
	{ir.ONEG, types.TUINT16}:  ssa.OpNeg16,
	{ir.ONEG, types.TINT32}:   ssa.OpNeg32,
	{ir.ONEG, types.TUINT32}:  ssa.OpNeg32,
	{ir.ONEG, types.TINT64}:   ssa.OpNeg64,
	{ir.ONEG, types.TUINT64}:  ssa.OpNeg64,
	{ir.ONEG, types.TFLOAT32}: ssa.OpNeg32F,
	{ir.ONEG, types.TFLOAT64}: ssa.OpNeg64F,

	{ir.OBITNOT, types.TINT8}:   ssa.OpCom8,
	{ir.OBITNOT, types.TUINT8}:  ssa.OpCom8,
	{ir.OBITNOT, types.TINT16}:  ssa.OpCom16,
	{ir.OBITNOT, types.TUINT16}: ssa.OpCom16,
	{ir.OBITNOT, types.TINT32}:  ssa.OpCom32,
	{ir.OBITNOT, types.TUINT32}: ssa.OpCom32,
	{ir.OBITNOT, types.TINT64}:  ssa.OpCom64,
	{ir.OBITNOT, types.TUINT64}: ssa.OpCom64,

	{ir.OIMAG, types.TCOMPLEX64}:  ssa.OpComplexImag,
	{ir.OIMAG, types.TCOMPLEX128}: ssa.OpComplexImag,
	{ir.OREAL, types.TCOMPLEX64}:  ssa.OpComplexReal,
	{ir.OREAL, types.TCOMPLEX128}: ssa.OpComplexReal,

	{ir.OMUL, types.TINT8}:    ssa.OpMul8,
	{ir.OMUL, types.TUINT8}:   ssa.OpMul8,
	{ir.OMUL, types.TINT16}:   ssa.OpMul16,
	{ir.OMUL, types.TUINT16}:  ssa.OpMul16,
	{ir.OMUL, types.TINT32}:   ssa.OpMul32,
	{ir.OMUL, types.TUINT32}:  ssa.OpMul32,
	{ir.OMUL, types.TINT64}:   ssa.OpMul64,
	{ir.OMUL, types.TUINT64}:  ssa.OpMul64,
	{ir.OMUL, types.TFLOAT32}: ssa.OpMul32F,
	{ir.OMUL, types.TFLOAT64}: ssa.OpMul64F,

	{ir.ODIV, types.TFLOAT32}: ssa.OpDiv32F,
	{ir.ODIV, types.TFLOAT64}: ssa.OpDiv64F,

	{ir.ODIV, types.TINT8}:   ssa.OpDiv8,
	{ir.ODIV, types.TUINT8}:  ssa.OpDiv8u,
	{ir.ODIV, types.TINT16}:  ssa.OpDiv16,
	{ir.ODIV, types.TUINT16}: ssa.OpDiv16u,
	{ir.ODIV, types.TINT32}:  ssa.OpDiv32,
	{ir.ODIV, types.TUINT32}: ssa.OpDiv32u,
	{ir.ODIV, types.TINT64}:  ssa.OpDiv64,
	{ir.ODIV, types.TUINT64}: ssa.OpDiv64u,

	{ir.OMOD, types.TINT8}:   ssa.OpMod8,
	{ir.OMOD, types.TUINT8}:  ssa.OpMod8u,
	{ir.OMOD, types.TINT16}:  ssa.OpMod16,
	{ir.OMOD, types.TUINT16}: ssa.OpMod16u,
	{ir.OMOD, types.TINT32}:  ssa.OpMod32,
	{ir.OMOD, types.TUINT32}: ssa.OpMod32u,
	{ir.OMOD, types.TINT64}:  ssa.OpMod64,
	{ir.OMOD, types.TUINT64}: ssa.OpMod64u,

	{ir.OAND, types.TINT8}:   ssa.OpAnd8,
	{ir.OAND, types.TUINT8}:  ssa.OpAnd8,
	{ir.OAND, types.TINT16}:  ssa.OpAnd16,
	{ir.OAND, types.TUINT16}: ssa.OpAnd16,
	{ir.OAND, types.TINT32}:  ssa.OpAnd32,
	{ir.OAND, types.TUINT32}: ssa.OpAnd32,
	{ir.OAND, types.TINT64}:  ssa.OpAnd64,
	{ir.OAND, types.TUINT64}: ssa.OpAnd64,

	{ir.OOR, types.TINT8}:   ssa.OpOr8,
	{ir.OOR, types.TUINT8}:  ssa.OpOr8,
	{ir.OOR, types.TINT16}:  ssa.OpOr16,
	{ir.OOR, types.TUINT16}: ssa.OpOr16,
	{ir.OOR, types.TINT32}:  ssa.OpOr32,
	{ir.OOR, types.TUINT32}: ssa.OpOr32,
	{ir.OOR, types.TINT64}:  ssa.OpOr64,
	{ir.OOR, types.TUINT64}: ssa.OpOr64,

	{ir.OXOR, types.TINT8}:   ssa.OpXor8,
	{ir.OXOR, types.TUINT8}:  ssa.OpXor8,
	{ir.OXOR, types.TINT16}:  ssa.OpXor16,
	{ir.OXOR, types.TUINT16}: ssa.OpXor16,
	{ir.OXOR, types.TINT32}:  ssa.OpXor32,
	{ir.OXOR, types.TUINT32}: ssa.OpXor32,
	{ir.OXOR, types.TINT64}:  ssa.OpXor64,
	{ir.OXOR, types.TUINT64}: ssa.OpXor64,

	{ir.OEQ, types.TBOOL}:      ssa.OpEqB,
	{ir.OEQ, types.TINT8}:      ssa.OpEq8,
	{ir.OEQ, types.TUINT8}:     ssa.OpEq8,
	{ir.OEQ, types.TINT16}:     ssa.OpEq16,
	{ir.OEQ, types.TUINT16}:    ssa.OpEq16,
	{ir.OEQ, types.TINT32}:     ssa.OpEq32,
	{ir.OEQ, types.TUINT32}:    ssa.OpEq32,
	{ir.OEQ, types.TINT64}:     ssa.OpEq64,
	{ir.OEQ, types.TUINT64}:    ssa.OpEq64,
	{ir.OEQ, types.TINTER}:     ssa.OpEqInter,
	{ir.OEQ, types.TSLICE}:     ssa.OpEqSlice,
	{ir.OEQ, types.TFUNC}:      ssa.OpEqPtr,
	{ir.OEQ, types.TMAP}:       ssa.OpEqPtr,
	{ir.OEQ, types.TCHAN}:      ssa.OpEqPtr,
	{ir.OEQ, types.TPTR}:       ssa.OpEqPtr,
	{ir.OEQ, types.TUINTPTR}:   ssa.OpEqPtr,
	{ir.OEQ, types.TUNSAFEPTR}: ssa.OpEqPtr,
	{ir.OEQ, types.TFLOAT64}:   ssa.OpEq64F,
	{ir.OEQ, types.TFLOAT32}:   ssa.OpEq32F,

	{ir.ONE, types.TBOOL}:      ssa.OpNeqB,
	{ir.ONE, types.TINT8}:      ssa.OpNeq8,
	{ir.ONE, types.TUINT8}:     ssa.OpNeq8,
	{ir.ONE, types.TINT16}:     ssa.OpNeq16,
	{ir.ONE, types.TUINT16}:    ssa.OpNeq16,
	{ir.ONE, types.TINT32}:     ssa.OpNeq32,
	{ir.ONE, types.TUINT32}:    ssa.OpNeq32,
	{ir.ONE, types.TINT64}:     ssa.OpNeq64,
	{ir.ONE, types.TUINT64}:    ssa.OpNeq64,
	{ir.ONE, types.TINTER}:     ssa.OpNeqInter,
	{ir.ONE, types.TSLICE}:     ssa.OpNeqSlice,
	{ir.ONE, types.TFUNC}:      ssa.OpNeqPtr,
	{ir.ONE, types.TMAP}:       ssa.OpNeqPtr,
	{ir.ONE, types.TCHAN}:      ssa.OpNeqPtr,
	{ir.ONE, types.TPTR}:       ssa.OpNeqPtr,
	{ir.ONE, types.TUINTPTR}:   ssa.OpNeqPtr,
	{ir.ONE, types.TUNSAFEPTR}: ssa.OpNeqPtr,
	{ir.ONE, types.TFLOAT64}:   ssa.OpNeq64F,
	{ir.ONE, types.TFLOAT32}:   ssa.OpNeq32F,

	{ir.OLT, types.TINT8}:    ssa.OpLess8,
	{ir.OLT, types.TUINT8}:   ssa.OpLess8U,
	{ir.OLT, types.TINT16}:   ssa.OpLess16,
	{ir.OLT, types.TUINT16}:  ssa.OpLess16U,
	{ir.OLT, types.TINT32}:   ssa.OpLess32,
	{ir.OLT, types.TUINT32}:  ssa.OpLess32U,
	{ir.OLT, types.TINT64}:   ssa.OpLess64,
	{ir.OLT, types.TUINT64}:  ssa.OpLess64U,
	{ir.OLT, types.TFLOAT64}: ssa.OpLess64F,
	{ir.OLT, types.TFLOAT32}: ssa.OpLess32F,

	{ir.OLE, types.TINT8}:    ssa.OpLeq8,
	{ir.OLE, types.TUINT8}:   ssa.OpLeq8U,
	{ir.OLE, types.TINT16}:   ssa.OpLeq16,
	{ir.OLE, types.TUINT16}:  ssa.OpLeq16U,
	{ir.OLE, types.TINT32}:   ssa.OpLeq32,
	{ir.OLE, types.TUINT32}:  ssa.OpLeq32U,
	{ir.OLE, types.TINT64}:   ssa.OpLeq64,
	{ir.OLE, types.TUINT64}:  ssa.OpLeq64U,
	{ir.OLE, types.TFLOAT64}: ssa.OpLeq64F,
	{ir.OLE, types.TFLOAT32}: ssa.OpLeq32F,
}

func (s *state) concreteEtype(t *types.Type) types.Kind {
	e := t.Kind()
	switch e {
	default:
		return e
	case types.TINT:
		if s.config.PtrSize == 8 {
			return types.TINT64
		}
		return types.TINT32
	case types.TUINT:
		if s.config.PtrSize == 8 {
			return types.TUINT64
		}
		return types.TUINT32
	case types.TUINTPTR:
		if s.config.PtrSize == 8 {
			return types.TUINT64
		}
		return types.TUINT32
	}
}

func (s *state) ssaOp(op ir.Op, t *types.Type) ssa.Op {
	etype := s.concreteEtype(t)
	x, ok := opToSSA[opAndType{op, etype}]
	if !ok {
		s.Fatalf("unhandled binary op %v %s", op, etype)
	}
	return x
}

type opAndTwoTypes struct {
	op     ir.Op
	etype1 types.Kind
	etype2 types.Kind
}

type twoTypes struct {
	etype1 types.Kind
	etype2 types.Kind
}

type twoOpsAndType struct {
	op1              ssa.Op
	op2              ssa.Op
	intermediateType types.Kind
}

var fpConvOpToSSA = map[twoTypes]twoOpsAndType{

	{types.TINT8, types.TFLOAT32}:  {ssa.OpSignExt8to32, ssa.OpCvt32to32F, types.TINT32},
	{types.TINT16, types.TFLOAT32}: {ssa.OpSignExt16to32, ssa.OpCvt32to32F, types.TINT32},
	{types.TINT32, types.TFLOAT32}: {ssa.OpCopy, ssa.OpCvt32to32F, types.TINT32},
	{types.TINT64, types.TFLOAT32}: {ssa.OpCopy, ssa.OpCvt64to32F, types.TINT64},

	{types.TINT8, types.TFLOAT64}:  {ssa.OpSignExt8to32, ssa.OpCvt32to64F, types.TINT32},
	{types.TINT16, types.TFLOAT64}: {ssa.OpSignExt16to32, ssa.OpCvt32to64F, types.TINT32},
	{types.TINT32, types.TFLOAT64}: {ssa.OpCopy, ssa.OpCvt32to64F, types.TINT32},
	{types.TINT64, types.TFLOAT64}: {ssa.OpCopy, ssa.OpCvt64to64F, types.TINT64},

	{types.TFLOAT32, types.TINT8}:  {ssa.OpCvt32Fto32, ssa.OpTrunc32to8, types.TINT32},
	{types.TFLOAT32, types.TINT16}: {ssa.OpCvt32Fto32, ssa.OpTrunc32to16, types.TINT32},
	{types.TFLOAT32, types.TINT32}: {ssa.OpCvt32Fto32, ssa.OpCopy, types.TINT32},
	{types.TFLOAT32, types.TINT64}: {ssa.OpCvt32Fto64, ssa.OpCopy, types.TINT64},

	{types.TFLOAT64, types.TINT8}:  {ssa.OpCvt64Fto32, ssa.OpTrunc32to8, types.TINT32},
	{types.TFLOAT64, types.TINT16}: {ssa.OpCvt64Fto32, ssa.OpTrunc32to16, types.TINT32},
	{types.TFLOAT64, types.TINT32}: {ssa.OpCvt64Fto32, ssa.OpCopy, types.TINT32},
	{types.TFLOAT64, types.TINT64}: {ssa.OpCvt64Fto64, ssa.OpCopy, types.TINT64},
	// unsigned
	{types.TUINT8, types.TFLOAT32}:  {ssa.OpZeroExt8to32, ssa.OpCvt32to32F, types.TINT32},
	{types.TUINT16, types.TFLOAT32}: {ssa.OpZeroExt16to32, ssa.OpCvt32to32F, types.TINT32},
	{types.TUINT32, types.TFLOAT32}: {ssa.OpZeroExt32to64, ssa.OpCvt64to32F, types.TINT64}, // go wide to dodge unsigned
	{types.TUINT64, types.TFLOAT32}: {ssa.OpCopy, ssa.OpInvalid, types.TUINT64},            // Cvt64Uto32F, branchy code expansion instead

	{types.TUINT8, types.TFLOAT64}:  {ssa.OpZeroExt8to32, ssa.OpCvt32to64F, types.TINT32},
	{types.TUINT16, types.TFLOAT64}: {ssa.OpZeroExt16to32, ssa.OpCvt32to64F, types.TINT32},
	{types.TUINT32, types.TFLOAT64}: {ssa.OpZeroExt32to64, ssa.OpCvt64to64F, types.TINT64}, // go wide to dodge unsigned
	{types.TUINT64, types.TFLOAT64}: {ssa.OpCopy, ssa.OpInvalid, types.TUINT64},            // Cvt64Uto64F, branchy code expansion instead

	{types.TFLOAT32, types.TUINT8}:  {ssa.OpCvt32Fto32, ssa.OpTrunc32to8, types.TINT32},
	{types.TFLOAT32, types.TUINT16}: {ssa.OpCvt32Fto32, ssa.OpTrunc32to16, types.TINT32},
	{types.TFLOAT32, types.TUINT32}: {ssa.OpInvalid, ssa.OpCopy, types.TINT64},  // Cvt64Fto32U, branchy code expansion instead
	{types.TFLOAT32, types.TUINT64}: {ssa.OpInvalid, ssa.OpCopy, types.TUINT64}, // Cvt32Fto64U, branchy code expansion instead

	{types.TFLOAT64, types.TUINT8}:  {ssa.OpCvt64Fto32, ssa.OpTrunc32to8, types.TINT32},
	{types.TFLOAT64, types.TUINT16}: {ssa.OpCvt64Fto32, ssa.OpTrunc32to16, types.TINT32},
	{types.TFLOAT64, types.TUINT32}: {ssa.OpInvalid, ssa.OpCopy, types.TINT64},  // Cvt64Fto32U, branchy code expansion instead
	{types.TFLOAT64, types.TUINT64}: {ssa.OpInvalid, ssa.OpCopy, types.TUINT64}, // Cvt64Fto64U, branchy code expansion instead

	// float
	{types.TFLOAT64, types.TFLOAT32}: {ssa.OpCvt64Fto32F, ssa.OpCopy, types.TFLOAT32},
	{types.TFLOAT64, types.TFLOAT64}: {ssa.OpRound64F, ssa.OpCopy, types.TFLOAT64},
	{types.TFLOAT32, types.TFLOAT32}: {ssa.OpRound32F, ssa.OpCopy, types.TFLOAT32},
	{types.TFLOAT32, types.TFLOAT64}: {ssa.OpCvt32Fto64F, ssa.OpCopy, types.TFLOAT64},
}

// this map is used only for 32-bit arch, and only includes the difference
// on 32-bit arch, don't use int64<->float conversion for uint32
var fpConvOpToSSA32 = map[twoTypes]twoOpsAndType{
	{types.TUINT32, types.TFLOAT32}: {ssa.OpCopy, ssa.OpCvt32Uto32F, types.TUINT32},
	{types.TUINT32, types.TFLOAT64}: {ssa.OpCopy, ssa.OpCvt32Uto64F, types.TUINT32},
	{types.TFLOAT32, types.TUINT32}: {ssa.OpCvt32Fto32U, ssa.OpCopy, types.TUINT32},
	{types.TFLOAT64, types.TUINT32}: {ssa.OpCvt64Fto32U, ssa.OpCopy, types.TUINT32},
}

// uint64<->float conversions, only on machines that have instructions for that
var uint64fpConvOpToSSA = map[twoTypes]twoOpsAndType{
	{types.TUINT64, types.TFLOAT32}: {ssa.OpCopy, ssa.OpCvt64Uto32F, types.TUINT64},
	{types.TUINT64, types.TFLOAT64}: {ssa.OpCopy, ssa.OpCvt64Uto64F, types.TUINT64},
	{types.TFLOAT32, types.TUINT64}: {ssa.OpCvt32Fto64U, ssa.OpCopy, types.TUINT64},
	{types.TFLOAT64, types.TUINT64}: {ssa.OpCvt64Fto64U, ssa.OpCopy, types.TUINT64},
}

var shiftOpToSSA = map[opAndTwoTypes]ssa.Op{
	{ir.OLSH, types.TINT8, types.TUINT8}:   ssa.OpLsh8x8,
	{ir.OLSH, types.TUINT8, types.TUINT8}:  ssa.OpLsh8x8,
	{ir.OLSH, types.TINT8, types.TUINT16}:  ssa.OpLsh8x16,
	{ir.OLSH, types.TUINT8, types.TUINT16}: ssa.OpLsh8x16,
	{ir.OLSH, types.TINT8, types.TUINT32}:  ssa.OpLsh8x32,
	{ir.OLSH, types.TUINT8, types.TUINT32}: ssa.OpLsh8x32,
	{ir.OLSH, types.TINT8, types.TUINT64}:  ssa.OpLsh8x64,
	{ir.OLSH, types.TUINT8, types.TUINT64}: ssa.OpLsh8x64,

	{ir.OLSH, types.TINT16, types.TUINT8}:   ssa.OpLsh16x8,
	{ir.OLSH, types.TUINT16, types.TUINT8}:  ssa.OpLsh16x8,
	{ir.OLSH, types.TINT16, types.TUINT16}:  ssa.OpLsh16x16,
	{ir.OLSH, types.TUINT16, types.TUINT16}: ssa.OpLsh16x16,
	{ir.OLSH, types.TINT16, types.TUINT32}:  ssa.OpLsh16x32,
	{ir.OLSH, types.TUINT16, types.TUINT32}: ssa.OpLsh16x32,
	{ir.OLSH, types.TINT16, types.TUINT64}:  ssa.OpLsh16x64,
	{ir.OLSH, types.TUINT16, types.TUINT64}: ssa.OpLsh16x64,

	{ir.OLSH, types.TINT32, types.TUINT8}:   ssa.OpLsh32x8,
	{ir.OLSH, types.TUINT32, types.TUINT8}:  ssa.OpLsh32x8,
	{ir.OLSH, types.TINT32, types.TUINT16}:  ssa.OpLsh32x16,
	{ir.OLSH, types.TUINT32, types.TUINT16}: ssa.OpLsh32x16,
	{ir.OLSH, types.TINT32, types.TUINT32}:  ssa.OpLsh32x32,
	{ir.OLSH, types.TUINT32, types.TUINT32}: ssa.OpLsh32x32,
	{ir.OLSH, types.TINT32, types.TUINT64}:  ssa.OpLsh32x64,
	{ir.OLSH, types.TUINT32, types.TUINT64}: ssa.OpLsh32x64,

	{ir.OLSH, types.TINT64, types.TUINT8}:   ssa.OpLsh64x8,
	{ir.OLSH, types.TUINT64, types.TUINT8}:  ssa.OpLsh64x8,
	{ir.OLSH, types.TINT64, types.TUINT16}:  ssa.OpLsh64x16,
	{ir.OLSH, types.TUINT64, types.TUINT16}: ssa.OpLsh64x16,
	{ir.OLSH, types.TINT64, types.TUINT32}:  ssa.OpLsh64x32,
	{ir.OLSH, types.TUINT64, types.TUINT32}: ssa.OpLsh64x32,
	{ir.OLSH, types.TINT64, types.TUINT64}:  ssa.OpLsh64x64,
	{ir.OLSH, types.TUINT64, types.TUINT64}: ssa.OpLsh64x64,

	{ir.ORSH, types.TINT8, types.TUINT8}:   ssa.OpRsh8x8,
	{ir.ORSH, types.TUINT8, types.TUINT8}:  ssa.OpRsh8Ux8,
	{ir.ORSH, types.TINT8, types.TUINT16}:  ssa.OpRsh8x16,
	{ir.ORSH, types.TUINT8, types.TUINT16}: ssa.OpRsh8Ux16,
	{ir.ORSH, types.TINT8, types.TUINT32}:  ssa.OpRsh8x32,
	{ir.ORSH, types.TUINT8, types.TUINT32}: ssa.OpRsh8Ux32,
	{ir.ORSH, types.TINT8, types.TUINT64}:  ssa.OpRsh8x64,
	{ir.ORSH, types.TUINT8, types.TUINT64}: ssa.OpRsh8Ux64,

	{ir.ORSH, types.TINT16, types.TUINT8}:   ssa.OpRsh16x8,
	{ir.ORSH, types.TUINT16, types.TUINT8}:  ssa.OpRsh16Ux8,
	{ir.ORSH, types.TINT16, types.TUINT16}:  ssa.OpRsh16x16,
	{ir.ORSH, types.TUINT16, types.TUINT16}: ssa.OpRsh16Ux16,
	{ir.ORSH, types.TINT16, types.TUINT32}:  ssa.OpRsh16x32,
	{ir.ORSH, types.TUINT16, types.TUINT32}: ssa.OpRsh16Ux32,
	{ir.ORSH, types.TINT16, types.TUINT64}:  ssa.OpRsh16x64,
	{ir.ORSH, types.TUINT16, types.TUINT64}: ssa.OpRsh16Ux64,

	{ir.ORSH, types.TINT32, types.TUINT8}:   ssa.OpRsh32x8,
	{ir.ORSH, types.TUINT32, types.TUINT8}:  ssa.OpRsh32Ux8,
	{ir.ORSH, types.TINT32, types.TUINT16}:  ssa.OpRsh32x16,
	{ir.ORSH, types.TUINT32, types.TUINT16}: ssa.OpRsh32Ux16,
	{ir.ORSH, types.TINT32, types.TUINT32}:  ssa.OpRsh32x32,
	{ir.ORSH, types.TUINT32, types.TUINT32}: ssa.OpRsh32Ux32,
	{ir.ORSH, types.TINT32, types.TUINT64}:  ssa.OpRsh32x64,
	{ir.ORSH, types.TUINT32, types.TUINT64}: ssa.OpRsh32Ux64,

	{ir.ORSH, types.TINT64, types.TUINT8}:   ssa.OpRsh64x8,
	{ir.ORSH, types.TUINT64, types.TUINT8}:  ssa.OpRsh64Ux8,
	{ir.ORSH, types.TINT64, types.TUINT16}:  ssa.OpRsh64x16,
	{ir.ORSH, types.TUINT64, types.TUINT16}: ssa.OpRsh64Ux16,
	{ir.ORSH, types.TINT64, types.TUINT32}:  ssa.OpRsh64x32,
	{ir.ORSH, types.TUINT64, types.TUINT32}: ssa.OpRsh64Ux32,
	{ir.ORSH, types.TINT64, types.TUINT64}:  ssa.OpRsh64x64,
	{ir.ORSH, types.TUINT64, types.TUINT64}: ssa.OpRsh64Ux64,
}

func (s *state) ssaShiftOp(op ir.Op, t *types.Type, u *types.Type) ssa.Op {
	etype1 := s.concreteEtype(t)
	etype2 := s.concreteEtype(u)
	x, ok := shiftOpToSSA[opAndTwoTypes{op, etype1, etype2}]
	if !ok {
		s.Fatalf("unhandled shift op %v etype=%s/%s", op, etype1, etype2)
	}
	return x
}

func (s *state) uintptrConstant(v uint64) *ssa.Value {
	if s.config.PtrSize == 4 {
		return s.newValue0I(ssa.OpConst32, types.Types[types.TUINTPTR], int64(v))
	}
	return s.newValue0I(ssa.OpConst64, types.Types[types.TUINTPTR], int64(v))
}

func (s *state) conv(n ir.Node, v *ssa.Value, ft, tt *types.Type) *ssa.Value {
	if ft.IsBoolean() && tt.IsKind(types.TUINT8) {
		// Bool -> uint8 is generated internally when indexing into runtime.staticbyte.
		return s.newValue1(ssa.OpCvtBoolToUint8, tt, v)
	}
	if ft.IsInteger() && tt.IsInteger() {
		var op ssa.Op
		if tt.Size() == ft.Size() {
			op = ssa.OpCopy
		} else if tt.Size() < ft.Size() {
			// truncation
			switch 10*ft.Size() + tt.Size() {
			case 21:
				op = ssa.OpTrunc16to8
			case 41:
				op = ssa.OpTrunc32to8
			case 42:
				op = ssa.OpTrunc32to16
			case 81:
				op = ssa.OpTrunc64to8
			case 82:
				op = ssa.OpTrunc64to16
			case 84:
				op = ssa.OpTrunc64to32
			default:
				s.Fatalf("weird integer truncation %v -> %v", ft, tt)
			}
		} else if ft.IsSigned() {
			// sign extension
			switch 10*ft.Size() + tt.Size() {
			case 12:
				op = ssa.OpSignExt8to16
			case 14:
				op = ssa.OpSignExt8to32
			case 18:
				op = ssa.OpSignExt8to64
			case 24:
				op = ssa.OpSignExt16to32
			case 28:
				op = ssa.OpSignExt16to64
			case 48:
				op = ssa.OpSignExt32to64
			default:
				s.Fatalf("bad integer sign extension %v -> %v", ft, tt)
			}
		} else {
			// zero extension
			switch 10*ft.Size() + tt.Size() {
			case 12:
				op = ssa.OpZeroExt8to16
			case 14:
				op = ssa.OpZeroExt8to32
			case 18:
				op = ssa.OpZeroExt8to64
			case 24:
				op = ssa.OpZeroExt16to32
			case 28:
				op = ssa.OpZeroExt16to64
			case 48:
				op = ssa.OpZeroExt32to64
			default:
				s.Fatalf("weird integer sign extension %v -> %v", ft, tt)
			}
		}
		return s.newValue1(op, tt, v)
	}

	if ft.IsComplex() && tt.IsComplex() {
		var op ssa.Op
		if ft.Size() == tt.Size() {
			switch ft.Size() {
			case 8:
				op = ssa.OpRound32F
			case 16:
				op = ssa.OpRound64F
			default:
				s.Fatalf("weird complex conversion %v -> %v", ft, tt)
			}
		} else if ft.Size() == 8 && tt.Size() == 16 {
			op = ssa.OpCvt32Fto64F
		} else if ft.Size() == 16 && tt.Size() == 8 {
			op = ssa.OpCvt64Fto32F
		} else {
			s.Fatalf("weird complex conversion %v -> %v", ft, tt)
		}
		ftp := types.FloatForComplex(ft)
		ttp := types.FloatForComplex(tt)
		return s.newValue2(ssa.OpComplexMake, tt,
			s.newValueOrSfCall1(op, ttp, s.newValue1(ssa.OpComplexReal, ftp, v)),
			s.newValueOrSfCall1(op, ttp, s.newValue1(ssa.OpComplexImag, ftp, v)))
	}

	if tt.IsComplex() { // and ft is not complex
		// Needed for generics support - can't happen in normal Go code.
		et := types.FloatForComplex(tt)
		v = s.conv(n, v, ft, et)
		return s.newValue2(ssa.OpComplexMake, tt, v, s.zeroVal(et))
	}

	if ft.IsFloat() || tt.IsFloat() {
		cft, ctt := s.concreteEtype(ft), s.concreteEtype(tt)
		conv, ok := fpConvOpToSSA[twoTypes{cft, ctt}]
		// there's a change to a conversion-op table, this restores the old behavior if ConvertHash is false.
		// use salted hash to distinguish unsigned convert at a Pos from signed convert at a Pos
		if ctt == types.TUINT32 && ft.IsFloat() && !base.ConvertHash.MatchPosWithInfo(n.Pos(), "U", nil) {
			// revert to old behavior
			conv.op1 = ssa.OpCvt64Fto64
			if cft == types.TFLOAT32 {
				conv.op1 = ssa.OpCvt32Fto64
			}
			conv.op2 = ssa.OpTrunc64to32

		}
		if s.config.RegSize == 4 && Arch.LinkArch.Family != sys.MIPS && !s.softFloat {
			if conv1, ok1 := fpConvOpToSSA32[twoTypes{s.concreteEtype(ft), s.concreteEtype(tt)}]; ok1 {
				conv = conv1
			}
		}
		if Arch.LinkArch.Family == sys.ARM64 || Arch.LinkArch.Family == sys.Wasm || Arch.LinkArch.Family == sys.S390X || s.softFloat {
			if conv1, ok1 := uint64fpConvOpToSSA[twoTypes{s.concreteEtype(ft), s.concreteEtype(tt)}]; ok1 {
				conv = conv1
			}
		}

		if Arch.LinkArch.Family == sys.MIPS && !s.softFloat {
			if ft.Size() == 4 && ft.IsInteger() && !ft.IsSigned() {
				// tt is float32 or float64, and ft is also unsigned
				if tt.Size() == 4 {
					return s.uint32Tofloat32(n, v, ft, tt)
				}
				if tt.Size() == 8 {
					return s.uint32Tofloat64(n, v, ft, tt)
				}
			} else if tt.Size() == 4 && tt.IsInteger() && !tt.IsSigned() {
				// ft is float32 or float64, and tt is unsigned integer
				if ft.Size() == 4 {
					return s.float32ToUint32(n, v, ft, tt)
				}
				if ft.Size() == 8 {
					return s.float64ToUint32(n, v, ft, tt)
				}
			}
		}

		if !ok {
			s.Fatalf("weird float conversion %v -> %v", ft, tt)
		}
		op1, op2, it := conv.op1, conv.op2, conv.intermediateType

		if op1 != ssa.OpInvalid && op2 != ssa.OpInvalid {
			// normal case, not tripping over unsigned 64
			if op1 == ssa.OpCopy {
				if op2 == ssa.OpCopy {
					return v
				}
				return s.newValueOrSfCall1(op2, tt, v)
			}
			if op2 == ssa.OpCopy {
				return s.newValueOrSfCall1(op1, tt, v)
			}
			return s.newValueOrSfCall1(op2, tt, s.newValueOrSfCall1(op1, types.Types[it], v))
		}
		// Tricky 64-bit unsigned cases.
		if ft.IsInteger() {
			// tt is float32 or float64, and ft is also unsigned
			if tt.Size() == 4 {
				return s.uint64Tofloat32(n, v, ft, tt)
			}
			if tt.Size() == 8 {
				return s.uint64Tofloat64(n, v, ft, tt)
			}
			s.Fatalf("weird unsigned integer to float conversion %v -> %v", ft, tt)
		}
		// ft is float32 or float64, and tt is unsigned integer
		if ft.Size() == 4 {
			switch tt.Size() {
			case 8:
				return s.float32ToUint64(n, v, ft, tt)
			case 4, 2, 1:
				// TODO should 2 and 1 saturate or truncate?
				return s.float32ToUint32(n, v, ft, tt)
			}
		}
		if ft.Size() == 8 {
			switch tt.Size() {
			case 8:
				return s.float64ToUint64(n, v, ft, tt)
			case 4, 2, 1:
				// TODO should 2 and 1 saturate or truncate?
				return s.float64ToUint32(n, v, ft, tt)
			}

		}
		s.Fatalf("weird float to unsigned integer conversion %v -> %v", ft, tt)
		return nil
	}

	s.Fatalf("unhandled OCONV %s -> %s", ft.Kind(), tt.Kind())
	return nil
}

// expr converts the expression n to ssa, adds it to s and returns the ssa result.
func (s *state) expr(n ir.Node) *ssa.Value {
	return s.exprCheckPtr(n, true)
}

func (s *state) exprCheckPtr(n ir.Node, checkPtrOK bool) *ssa.Value {
	if ir.HasUniquePos(n) {
		// ONAMEs and named OLITERALs have the line number
		// of the decl, not the use. See issue 14742.
		s.pushLine(n.Pos())
		defer s.popLine()
	}

	s.stmtList(n.Init())
	switch n.Op() {
	case ir.OBYTES2STRTMP:
		n := n.(*ir.ConvExpr)
		slice := s.expr(n.X)
		ptr := s.newValue1(ssa.OpSlicePtr, s.f.Config.Types.BytePtr, slice)
		len := s.newValue1(ssa.OpSliceLen, types.Types[types.TINT], slice)
		return s.newValue2(ssa.OpStringMake, n.Type(), ptr, len)
	case ir.OSTR2BYTESTMP:
		n := n.(*ir.ConvExpr)
		str := s.expr(n.X)
		ptr := s.newValue1(ssa.OpStringPtr, s.f.Config.Types.BytePtr, str)
		if !n.NonNil() {
			// We need to ensure []byte("") evaluates to []byte{}, and not []byte(nil).
			//
			// TODO(mdempsky): Investigate using "len != 0" instead of "ptr != nil".
			cond := s.newValue2(ssa.OpNeqPtr, types.Types[types.TBOOL], ptr, s.constNil(ptr.Type))
			zerobase := s.newValue1A(ssa.OpAddr, ptr.Type, ir.Syms.Zerobase, s.sb)
			ptr = s.ternary(cond, ptr, zerobase)
		}
		len := s.newValue1(ssa.OpStringLen, types.Types[types.TINT], str)
		return s.newValue3(ssa.OpSliceMake, n.Type(), ptr, len, len)
	case ir.OCFUNC:
		n := n.(*ir.UnaryExpr)
		aux := n.X.(*ir.Name).Linksym()
		// OCFUNC is used to build function values, which must
		// always reference ABIInternal entry points.
		if aux.ABI() != obj.ABIInternal {
			s.Fatalf("expected ABIInternal: %v", aux.ABI())
		}
		return s.entryNewValue1A(ssa.OpAddr, n.Type(), aux, s.sb)
	case ir.ONAME:
		n := n.(*ir.Name)
		if n.Class == ir.PFUNC {
			// "value" of a function is the address of the function's closure
			sym := staticdata.FuncLinksym(n)
			return s.entryNewValue1A(ssa.OpAddr, types.NewPtr(n.Type()), sym, s.sb)
		}
		if s.canSSA(n) {
			return s.variable(n, n.Type())
		}
		return s.load(n.Type(), s.addr(n))
	case ir.OLINKSYMOFFSET:
		n := n.(*ir.LinksymOffsetExpr)
		return s.load(n.Type(), s.addr(n))
	case ir.ONIL:
		n := n.(*ir.NilExpr)
		t := n.Type()
		switch {
		case t.IsSlice():
			return s.constSlice(t)
		case t.IsInterface():
			return s.constInterface(t)
		default:
			return s.constNil(t)
		}
	case ir.OLITERAL:
		switch u := n.Val(); u.Kind() {
		case constant.Int:
			i := ir.IntVal(n.Type(), u)
			switch n.Type().Size() {
			case 1:
				return s.constInt8(n.Type(), int8(i))
			case 2:
				return s.constInt16(n.Type(), int16(i))
			case 4:
				return s.constInt32(n.Type(), int32(i))
			case 8:
				return s.constInt64(n.Type(), i)
			default:
				s.Fatalf("bad integer size %d", n.Type().Size())
				return nil
			}
		case constant.String:
			i := constant.StringVal(u)
			if i == "" {
				return s.constEmptyString(n.Type())
			}
			return s.entryNewValue0A(ssa.OpConstString, n.Type(), ssa.StringToAux(i))
		case constant.Bool:
			return s.constBool(constant.BoolVal(u))
		case constant.Float:
			f, _ := constant.Float64Val(u)
			switch n.Type().Size() {
			case 4:
				return s.constFloat32(n.Type(), f)
			case 8:
				return s.constFloat64(n.Type(), f)
			default:
				s.Fatalf("bad float size %d", n.Type().Size())
				return nil
			}
		case constant.Complex:
			re, _ := constant.Float64Val(constant.Real(u))
			im, _ := constant.Float64Val(constant.Imag(u))
			switch n.Type().Size() {
			case 8:
				pt := types.Types[types.TFLOAT32]
				return s.newValue2(ssa.OpComplexMake, n.Type(),
					s.constFloat32(pt, re),
					s.constFloat32(pt, im))
			case 16:
				pt := types.Types[types.TFLOAT64]
				return s.newValue2(ssa.OpComplexMake, n.Type(),
					s.constFloat64(pt, re),
					s.constFloat64(pt, im))
			default:
				s.Fatalf("bad complex size %d", n.Type().Size())
				return nil
			}
		default:
			s.Fatalf("unhandled OLITERAL %v", u.Kind())
			return nil
		}
	case ir.OCONVNOP:
		n := n.(*ir.ConvExpr)
		to := n.Type()
		from := n.X.Type()

		// Assume everything will work out, so set up our return value.
		// Anything interesting that happens from here is a fatal.
		x := s.expr(n.X)
		if to == from {
			return x
		}

		// Special case for not confusing GC and liveness.
		// We don't want pointers accidentally classified
		// as not-pointers or vice-versa because of copy
		// elision.
		if to.IsPtrShaped() != from.IsPtrShaped() {
			return s.newValue2(ssa.OpConvert, to, x, s.mem())
		}

		v := s.newValue1(ssa.OpCopy, to, x) // ensure that v has the right type

		// CONVNOP closure
		if to.Kind() == types.TFUNC && from.IsPtrShaped() {
			return v
		}

		// named <--> unnamed type or typed <--> untyped const
		if from.Kind() == to.Kind() {
			return v
		}

		// unsafe.Pointer <--> *T
		if to.IsUnsafePtr() && from.IsPtrShaped() || from.IsUnsafePtr() && to.IsPtrShaped() {
			if s.checkPtrEnabled && checkPtrOK && to.IsPtr() && from.IsUnsafePtr() {
				s.checkPtrAlignment(n, v, nil)
			}
			return v
		}

		// map <--> *internal/runtime/maps.Map
		mt := types.NewPtr(reflectdata.MapType())
		if to.Kind() == types.TMAP && from == mt {
			return v
		}

		types.CalcSize(from)
		types.CalcSize(to)
		if from.Size() != to.Size() {
			s.Fatalf("CONVNOP width mismatch %v (%d) -> %v (%d)\n", from, from.Size(), to, to.Size())
			return nil
		}
		if etypesign(from.Kind()) != etypesign(to.Kind()) {
			s.Fatalf("CONVNOP sign mismatch %v (%s) -> %v (%s)\n", from, from.Kind(), to, to.Kind())
			return nil
		}

		if base.Flag.Cfg.Instrumenting {
			// These appear to be fine, but they fail the
			// integer constraint below, so okay them here.
			// Sample non-integer conversion: map[string]string -> *uint8
			return v
		}

		if etypesign(from.Kind()) == 0 {
			s.Fatalf("CONVNOP unrecognized non-integer %v -> %v\n", from, to)
			return nil
		}

		// integer, same width, same sign
		return v

	case ir.OCONV:
		n := n.(*ir.ConvExpr)
		x := s.expr(n.X)
		return s.conv(n, x, n.X.Type(), n.Type())

	case ir.ODOTTYPE:
		n := n.(*ir.TypeAssertExpr)
		res, _ := s.dottype(n, false)
		return res

	case ir.ODYNAMICDOTTYPE:
		n := n.(*ir.DynamicTypeAssertExpr)
		res, _ := s.dynamicDottype(n, false)
		return res

	// binary ops
	case ir.OLT, ir.OEQ, ir.ONE, ir.OLE, ir.OGE, ir.OGT:
		n := n.(*ir.BinaryExpr)
		a := s.expr(n.X)
		b := s.expr(n.Y)
		if n.X.Type().IsComplex() {
			pt := types.FloatForComplex(n.X.Type())
			op := s.ssaOp(ir.OEQ, pt)
			r := s.newValueOrSfCall2(op, types.Types[types.TBOOL], s.newValue1(ssa.OpComplexReal, pt, a), s.newValue1(ssa.OpComplexReal, pt, b))
			i := s.newValueOrSfCall2(op, types.Types[types.TBOOL], s.newValue1(ssa.OpComplexImag, pt, a), s.newValue1(ssa.OpComplexImag, pt, b))
			c := s.newValue2(ssa.OpAndB, types.Types[types.TBOOL], r, i)
			switch n.Op() {
			case ir.OEQ:
				return c
			case ir.ONE:
				return s.newValue1(ssa.OpNot, types.Types[types.TBOOL], c)
			default:
				s.Fatalf("ordered complex compare %v", n.Op())
			}
		}

		// Convert OGE and OGT into OLE and OLT.
		op := n.Op()
		switch op {
		case ir.OGE:
			op, a, b = ir.OLE, b, a
		case ir.OGT:
			op, a, b = ir.OLT, b, a
		}
		if n.X.Type().IsFloat() {
			// float comparison
			return s.newValueOrSfCall2(s.ssaOp(op, n.X.Type()), types.Types[types.TBOOL], a, b)
		}
		// integer comparison
		return s.newValue2(s.ssaOp(op, n.X.Type()), types.Types[types.TBOOL], a, b)
	case ir.OMUL:
		n := n.(*ir.BinaryExpr)
		a := s.expr(n.X)
		b := s.expr(n.Y)
		if n.Type().IsComplex() {
			mulop := ssa.OpMul64F
			addop := ssa.OpAdd64F
			subop := ssa.OpSub64F
			pt := types.FloatForComplex(n.Type()) // Could be Float32 or Float64
			wt := types.Types[types.TFLOAT64]     // Compute in Float64 to minimize cancellation error

			areal := s.newValue1(ssa.OpComplexReal, pt, a)
			breal := s.newValue1(ssa.OpComplexReal, pt, b)
			aimag := s.newValue1(ssa.OpComplexImag, pt, a)
			bimag := s.newValue1(ssa.OpComplexImag, pt, b)

			if pt != wt { // Widen for calculation
				areal = s.newValueOrSfCall1(ssa.OpCvt32Fto64F, wt, areal)
				breal = s.newValueOrSfCall1(ssa.OpCvt32Fto64F, wt, breal)
				aimag = s.newValueOrSfCall1(ssa.OpCvt32Fto64F, wt, aimag)
				bimag = s.newValueOrSfCall1(ssa.OpCvt32Fto64F, wt, bimag)
			}

			xreal := s.newValueOrSfCall2(subop, wt, s.newValueOrSfCall2(mulop, wt, areal, breal), s.newValueOrSfCall2(mulop, wt, aimag, bimag))
			ximag := s.newValueOrSfCall2(addop, wt, s.newValueOrSfCall2(mulop, wt, areal, bimag), s.newValueOrSfCall2(mulop, wt, aimag, breal))

			if pt != wt { // Narrow to store back
				xreal = s.newValueOrSfCall1(ssa.OpCvt64Fto32F, pt, xreal)
				ximag = s.newValueOrSfCall1(ssa.OpCvt64Fto32F, pt, ximag)
			}

			return s.newValue2(ssa.OpComplexMake, n.Type(), xreal, ximag)
		}

		if n.Type().IsFloat() {
			return s.newValueOrSfCall2(s.ssaOp(n.Op(), n.Type()), a.Type, a, b)
		}

		return s.newValue2(s.ssaOp(n.Op(), n.Type()), a.Type, a, b)

	case ir.ODIV:
		n := n.(*ir.BinaryExpr)
		a := s.expr(n.X)
		b := s.expr(n.Y)
		if n.Type().IsComplex() {
			// TODO this is not executed because the front-end substitutes a runtime call.
			// That probably ought to change; with modest optimization the widen/narrow
			// conversions could all be elided in larger expression trees.
			mulop := ssa.OpMul64F
			addop := ssa.OpAdd64F
			subop := ssa.OpSub64F
			divop := ssa.OpDiv64F
			pt := types.FloatForComplex(n.Type()) // Could be Float32 or Float64
			wt := types.Types[types.TFLOAT64]     // Compute in Float64 to minimize cancellation error

			areal := s.newValue1(ssa.OpComplexReal, pt, a)
			breal := s.newValue1(ssa.OpComplexReal, pt, b)
			aimag := s.newValue1(ssa.OpComplexImag, pt, a)
			bimag := s.newValue1(ssa.OpComplexImag, pt, b)

			if pt != wt { // Widen for calculation
				areal = s.newValueOrSfCall1(ssa.OpCvt32Fto64F, wt, areal)
				breal = s.newValueOrSfCall1(ssa.OpCvt32Fto64F, wt, breal)
				aimag = s.newValueOrSfCall1(ssa.OpCvt32Fto64F, wt, aimag)
				bimag = s.newValueOrSfCall1(ssa.OpCvt32Fto64F, wt, bimag)
			}

			denom := s.newValueOrSfCall2(addop, wt, s.newValueOrSfCall2(mulop, wt, breal, breal), s.newValueOrSfCall2(mulop, wt, bimag, bimag))
			xreal := s.newValueOrSfCall2(addop, wt, s.newValueOrSfCall2(mulop, wt, areal, breal), s.newValueOrSfCall2(mulop, wt, aimag, bimag))
			ximag := s.newValueOrSfCall2(subop, wt, s.newValueOrSfCall2(mulop, wt, aimag, breal), s.newValueOrSfCall2(mulop, wt, areal, bimag))

			// TODO not sure if this is best done in wide precision or narrow
			// Double-rounding might be an issue.
			// Note that the pre-SSA implementation does the entire calculation
			// in wide format, so wide is compatible.
			xreal = s.newValueOrSfCall2(divop, wt, xreal, denom)
			ximag = s.newValueOrSfCall2(divop, wt, ximag, denom)

			if pt != wt { // Narrow to store back
				xreal = s.newValueOrSfCall1(ssa.OpCvt64Fto32F, pt, xreal)
				ximag = s.newValueOrSfCall1(ssa.OpCvt64Fto32F, pt, ximag)
			}
			return s.newValue2(ssa.OpComplexMake, n.Type(), xreal, ximag)
		}
		if n.Type().IsFloat() {
			return s.newValueOrSfCall2(s.ssaOp(n.Op(), n.Type()), a.Type, a, b)
		}
		return s.intDivide(n, a, b)
	case ir.OMOD:
		n := n.(*ir.BinaryExpr)
		a := s.expr(n.X)
		b := s.expr(n.Y)
		return s.intDivide(n, a, b)
	case ir.OADD, ir.OSUB:
		n := n.(*ir.BinaryExpr)
		a := s.expr(n.X)
		b := s.expr(n.Y)
		if n.Type().IsComplex() {
			pt := types.FloatForComplex(n.Type())
			op := s.ssaOp(n.Op(), pt)
			return s.newValue2(ssa.OpComplexMake, n.Type(),
				s.newValueOrSfCall2(op, pt, s.newValue1(ssa.OpComplexReal, pt, a), s.newValue1(ssa.OpComplexReal, pt, b)),
				s.newValueOrSfCall2(op, pt, s.newValue1(ssa.OpComplexImag, pt, a), s.newValue1(ssa.OpComplexImag, pt, b)))
		}
		if n.Type().IsFloat() {
			return s.newValueOrSfCall2(s.ssaOp(n.Op(), n.Type()), a.Type, a, b)
		}
		return s.newValue2(s.ssaOp(n.Op(), n.Type()), a.Type, a, b)
	case ir.OAND, ir.OOR, ir.OXOR:
		n := n.(*ir.BinaryExpr)
		a := s.expr(n.X)
		b := s.expr(n.Y)
		return s.newValue2(s.ssaOp(n.Op(), n.Type()), a.Type, a, b)
	case ir.OANDNOT:
		n := n.(*ir.BinaryExpr)
		a := s.expr(n.X)
		b := s.expr(n.Y)
		b = s.newValue1(s.ssaOp(ir.OBITNOT, b.Type), b.Type, b)
		return s.newValue2(s.ssaOp(ir.OAND, n.Type()), a.Type, a, b)
	case ir.OLSH, ir.ORSH:
		n := n.(*ir.BinaryExpr)
		a := s.expr(n.X)
		b := s.expr(n.Y)
		bt := b.Type
		if bt.IsSigned() {
			cmp := s.newValue2(s.ssaOp(ir.OLE, bt), types.Types[types.TBOOL], s.zeroVal(bt), b)
			s.check(cmp, ir.Syms.Panicshift)
			bt = bt.ToUnsigned()
		}
		return s.newValue2(s.ssaShiftOp(n.Op(), n.Type(), bt), a.Type, a, b)
	case ir.OANDAND, ir.OOROR:
		// To implement OANDAND (and OOROR), we introduce a
		// new temporary variable to hold the result. The
		// variable is associated with the OANDAND node in the
		// s.vars table (normally variables are only
		// associated with ONAME nodes). We convert
		//     A && B
		// to
		//     var = A
		//     if var {
		//         var = B
		//     }
		// Using var in the subsequent block introduces the
		// necessary phi variable.
		n := n.(*ir.LogicalExpr)
		el := s.expr(n.X)
		s.vars[n] = el

		b := s.endBlock()
		b.Kind = ssa.BlockIf
		b.SetControl(el)
		// In theory, we should set b.Likely here based on context.
		// However, gc only gives us likeliness hints
		// in a single place, for plain OIF statements,
		// and passing around context is finicky, so don't bother for now.

		bRight := s.f.NewBlock(ssa.BlockPlain)
		bResult := s.f.NewBlock(ssa.BlockPlain)
		if n.Op() == ir.OANDAND {
			b.AddEdgeTo(bRight)
			b.AddEdgeTo(bResult)
		} else if n.Op() == ir.OOROR {
			b.AddEdgeTo(bResult)
			b.AddEdgeTo(bRight)
		}

		s.startBlock(bRight)
		er := s.expr(n.Y)
		s.vars[n] = er

		b = s.endBlock()
		b.AddEdgeTo(bResult)

		s.startBlock(bResult)
		return s.variable(n, types.Types[types.TBOOL])
	case ir.OCOMPLEX:
		n := n.(*ir.BinaryExpr)
		r := s.expr(n.X)
		i := s.expr(n.Y)
		return s.newValue2(ssa.OpComplexMake, n.Type(), r, i)

	// unary ops
	case ir.ONEG:
		n := n.(*ir.UnaryExpr)
		a := s.expr(n.X)
		if n.Type().IsComplex() {
			tp := types.FloatForComplex(n.Type())
			negop := s.ssaOp(n.Op(), tp)
			return s.newValue2(ssa.OpComplexMake, n.Type(),
				s.newValue1(negop, tp, s.newValue1(ssa.OpComplexReal, tp, a)),
				s.newValue1(negop, tp, s.newValue1(ssa.OpComplexImag, tp, a)))
		}
		return s.newValue1(s.ssaOp(n.Op(), n.Type()), a.Type, a)
	case ir.ONOT, ir.OBITNOT:
		n := n.(*ir.UnaryExpr)
		a := s.expr(n.X)
		return s.newValue1(s.ssaOp(n.Op(), n.Type()), a.Type, a)
	case ir.OIMAG, ir.OREAL:
		n := n.(*ir.UnaryExpr)
		a := s.expr(n.X)
		return s.newValue1(s.ssaOp(n.Op(), n.X.Type()), n.Type(), a)
	case ir.OPLUS:
		n := n.(*ir.UnaryExpr)
		return s.expr(n.X)

	case ir.OADDR:
		n := n.(*ir.AddrExpr)
		return s.addr(n.X)

	case ir.ORESULT:
		n := n.(*ir.ResultExpr)
		if s.prevCall == nil || s.prevCall.Op != ssa.OpStaticLECall && s.prevCall.Op != ssa.OpInterLECall && s.prevCall.Op != ssa.OpClosureLECall {
			panic("Expected to see a previous call")
		}
		which := n.Index
		if which == -1 {
			panic(fmt.Errorf("ORESULT %v does not match call %s", n, s.prevCall))
		}
		return s.resultOfCall(s.prevCall, which, n.Type())

	case ir.ODEREF:
		n := n.(*ir.StarExpr)
		p := s.exprPtr(n.X, n.Bounded(), n.Pos())
		return s.load(n.Type(), p)

	case ir.ODOT:
		n := n.(*ir.SelectorExpr)
		if n.X.Op() == ir.OSTRUCTLIT {
			// All literals with nonzero fields have already been
			// rewritten during walk. Any that remain are just T{}
			// or equivalents. Use the zero value.
			if !ir.IsZero(n.X) {
				s.Fatalf("literal with nonzero value in SSA: %v", n.X)
			}
			return s.zeroVal(n.Type())
		}
		// If n is addressable and can't be represented in
		// SSA, then load just the selected field. This
		// prevents false memory dependencies in race/msan/asan
		// instrumentation.
		if ir.IsAddressable(n) && !s.canSSA(n) {
			p := s.addr(n)
			return s.load(n.Type(), p)
		}
		v := s.expr(n.X)
		return s.newValue1I(ssa.OpStructSelect, n.Type(), int64(fieldIdx(n)), v)

	case ir.ODOTPTR:
		n := n.(*ir.SelectorExpr)
		p := s.exprPtr(n.X, n.Bounded(), n.Pos())
		p = s.newValue1I(ssa.OpOffPtr, types.NewPtr(n.Type()), n.Offset(), p)
		return s.load(n.Type(), p)

	case ir.OINDEX:
		n := n.(*ir.IndexExpr)
		switch {
		case n.X.Type().IsString():
			if n.Bounded() && ir.IsConst(n.X, constant.String) && ir.IsConst(n.Index, constant.Int) {
				// Replace "abc"[1] with 'b'.
				// Delayed until now because "abc"[1] is not an ideal constant.
				// See test/fixedbugs/issue11370.go.
				return s.newValue0I(ssa.OpConst8, types.Types[types.TUINT8], int64(int8(ir.StringVal(n.X)[ir.Int64Val(n.Index)])))
			}
			a := s.expr(n.X)
			i := s.expr(n.Index)
			len := s.newValue1(ssa.OpStringLen, types.Types[types.TINT], a)
			i = s.boundsCheck(i, len, ssa.BoundsIndex, n.Bounded())
			ptrtyp := s.f.Config.Types.BytePtr
			ptr := s.newValue1(ssa.OpStringPtr, ptrtyp, a)
			if ir.IsConst(n.Index, constant.Int) {
				ptr = s.newValue1I(ssa.OpOffPtr, ptrtyp, ir.Int64Val(n.Index), ptr)
			} else {
				ptr = s.newValue2(ssa.OpAddPtr, ptrtyp, ptr, i)
			}
			return s.load(types.Types[types.TUINT8], ptr)
		case n.X.Type().IsSlice():
			p := s.addr(n)
			return s.load(n.X.Type().Elem(), p)
		case n.X.Type().IsArray():
			if ssa.CanSSA(n.X.Type()) {
				// SSA can handle arrays of length at most 1.
				bound := n.X.Type().NumElem()
				a := s.expr(n.X)
				i := s.expr(n.Index)
				len := s.constInt(types.Types[types.TINT], bound)
				if bound == 0 {
					// Bounds check will never succeed.
					s.boundsCheck(i, len, ssa.BoundsIndex, false)
					// The return value won't be live. In case bounds checks
					// are turned off, load from (*T)(nil) to cause a segfault.
					return s.load(n.Type(), s.constNil(n.Type().PtrTo()))
				}
				s.boundsCheck(i, len, ssa.BoundsIndex, n.Bounded()) // checks i == 0
				return s.newValue1I(ssa.OpArraySelect, n.Type(), 0, a)
			}
			p := s.addr(n)
			return s.load(n.X.Type().Elem(), p)
		default:
			s.Fatalf("bad type for index %v", n.X.Type())
			return nil
		}

	case ir.OLEN, ir.OCAP:
		n := n.(*ir.UnaryExpr)
		// Note: all constant cases are handled by the frontend. If len or cap
		// makes it here, we want the side effects of the argument. See issue 72844.
		a := s.expr(n.X)
		t := n.X.Type()
		switch {
		case t.IsSlice():
			op := ssa.OpSliceLen
			if n.Op() == ir.OCAP {
				op = ssa.OpSliceCap
			}
			return s.newValue1(op, types.Types[types.TINT], a)
		case t.IsString(): // string; not reachable for OCAP
			return s.newValue1(ssa.OpStringLen, types.Types[types.TINT], a)
		case t.IsMap(), t.IsChan():
			return s.referenceTypeBuiltin(n, a)
		case t.IsArray():
			return s.constInt(types.Types[types.TINT], t.NumElem())
		case t.IsPtr() && t.Elem().IsArray():
			return s.constInt(types.Types[types.TINT], t.Elem().NumElem())
		default:
			s.Fatalf("bad type in len/cap: %v", t)
			return nil
		}

	case ir.OSPTR:
		n := n.(*ir.UnaryExpr)
		a := s.expr(n.X)
		if n.X.Type().IsSlice() {
			if n.Bounded() {
				return s.newValue1(ssa.OpSlicePtr, n.Type(), a)
			}
			return s.newValue1(ssa.OpSlicePtrUnchecked, n.Type(), a)
		} else {
			return s.newValue1(ssa.OpStringPtr, n.Type(), a)
		}

	case ir.OITAB:
		n := n.(*ir.UnaryExpr)
		a := s.expr(n.X)
		return s.newValue1(ssa.OpITab, n.Type(), a)

	case ir.OIDATA:
		n := n.(*ir.UnaryExpr)
		a := s.expr(n.X)
		return s.newValue1(ssa.OpIData, n.Type(), a)

	case ir.OMAKEFACE:
		n := n.(*ir.BinaryExpr)
		tab := s.expr(n.X)
		data := s.expr(n.Y)
		return s.newValue2(ssa.OpIMake, n.Type(), tab, data)

	case ir.OSLICEHEADER:
		n := n.(*ir.SliceHeaderExpr)
		p := s.expr(n.Ptr)
		l := s.expr(n.Len)
		c := s.expr(n.Cap)
		return s.newValue3(ssa.OpSliceMake, n.Type(), p, l, c)

	case ir.OSTRINGHEADER:
		n := n.(*ir.StringHeaderExpr)
		p := s.expr(n.Ptr)
		l := s.expr(n.Len)
		return s.newValue2(ssa.OpStringMake, n.Type(), p, l)

	case ir.OSLICE, ir.OSLICEARR, ir.OSLICE3, ir.OSLICE3ARR:
		n := n.(*ir.SliceExpr)
		check := s.checkPtrEnabled && n.Op() == ir.OSLICE3ARR && n.X.Op() == ir.OCONVNOP && n.X.(*ir.ConvExpr).X.Type().IsUnsafePtr()
		v := s.exprCheckPtr(n.X, !check)
		var i, j, k *ssa.Value
		if n.Low != nil {
			i = s.expr(n.Low)
		}
		if n.High != nil {
			j = s.expr(n.High)
		}
		if n.Max != nil {
			k = s.expr(n.Max)
		}
		p, l, c := s.slice(v, i, j, k, n.Bounded())
		if check {
			// Emit checkptr instrumentation after bound check to prevent false positive, see #46938.
			s.checkPtrAlignment(n.X.(*ir.ConvExpr), v, s.conv(n.Max, k, k.Type, types.Types[types.TUINTPTR]))
		}
		return s.newValue3(ssa.OpSliceMake, n.Type(), p, l, c)

	case ir.OSLICESTR:
		n := n.(*ir.SliceExpr)
		v := s.expr(n.X)
		var i, j *ssa.Value
		if n.Low != nil {
			i = s.expr(n.Low)
		}
		if n.High != nil {
			j = s.expr(n.High)
		}
		p, l, _ := s.slice(v, i, j, nil, n.Bounded())
		return s.newValue2(ssa.OpStringMake, n.Type(), p, l)

	case ir.OSLICE2ARRPTR:
		// if arrlen > slice.len {
		//   panic(...)
		// }
		// slice.ptr
		n := n.(*ir.ConvExpr)
		v := s.expr(n.X)
		nelem := n.Type().Elem().NumElem()
		arrlen := s.constInt(types.Types[types.TINT], nelem)
		cap := s.newValue1(ssa.OpSliceLen, types.Types[types.TINT], v)
		s.boundsCheck(arrlen, cap, ssa.BoundsConvert, false)
		op := ssa.OpSlicePtr
		if nelem == 0 {
			op = ssa.OpSlicePtrUnchecked
		}
		return s.newValue1(op, n.Type(), v)

	case ir.OCALLFUNC:
		n := n.(*ir.CallExpr)
		if ir.IsIntrinsicCall(n) {
			return s.intrinsicCall(n)
		}
		fallthrough

	case ir.OCALLINTER:
		n := n.(*ir.CallExpr)
		return s.callResult(n, callNormal)

	case ir.OGETG:
		n := n.(*ir.CallExpr)
		return s.newValue1(ssa.OpGetG, n.Type(), s.mem())

	case ir.OGETCALLERSP:
		n := n.(*ir.CallExpr)
		return s.newValue1(ssa.OpGetCallerSP, n.Type(), s.mem())

	case ir.OAPPEND:
		return s.append(n.(*ir.CallExpr), false)

	case ir.OMOVE2HEAP:
		return s.move2heap(n.(*ir.MoveToHeapExpr))

	case ir.OMIN, ir.OMAX:
		return s.minMax(n.(*ir.CallExpr))

	case ir.OSTRUCTLIT, ir.OARRAYLIT:
		// All literals with nonzero fields have already been
		// rewritten during walk. Any that remain are just T{}
		// or equivalents. Use the zero value.
		n := n.(*ir.CompLitExpr)
		if !ir.IsZero(n) {
			s.Fatalf("literal with nonzero value in SSA: %v", n)
		}
		return s.zeroVal(n.Type())

	case ir.ONEW:
		n := n.(*ir.UnaryExpr)
		if x, ok := n.X.(*ir.DynamicType); ok && x.Op() == ir.ODYNAMICTYPE {
			return s.newObjectNonSpecialized(n.Type().Elem(), s.expr(x.RType))
		}
		return s.newObject(n.Type().Elem())

	case ir.OUNSAFEADD:
		n := n.(*ir.BinaryExpr)
		ptr := s.expr(n.X)
		len := s.expr(n.Y)

		// Force len to uintptr to prevent misuse of garbage bits in the
		// upper part of the register (#48536).
		len = s.conv(n, len, len.Type, types.Types[types.TUINTPTR])

		return s.newValue2(ssa.OpAddPtr, n.Type(), ptr, len)

	default:
		s.Fatalf("unhandled expr %v", n.Op())
		return nil
	}
}

func (s *state) resultOfCall(c *ssa.Value, which int64, t *types.Type) *ssa.Value {
	aux := c.Aux.(*ssa.AuxCall)
	pa := aux.ParamAssignmentForResult(which)
	// TODO(register args) determine if in-memory TypeOK is better loaded early from SelectNAddr or later when SelectN is expanded.
	// SelectN is better for pattern-matching and possible call-aware analysis we might want to do in the future.
	if len(pa.Registers) == 0 && !ssa.CanSSA(t) {
		addr := s.newValue1I(ssa.OpSelectNAddr, types.NewPtr(t), which, c)
		return s.rawLoad(t, addr)
	}
	return s.newValue1I(ssa.OpSelectN, t, which, c)
}

func (s *state) resultAddrOfCall(c *ssa.Value, which int64, t *types.Type) *ssa.Value {
	aux := c.Aux.(*ssa.AuxCall)
	pa := aux.ParamAssignmentForResult(which)
	if len(pa.Registers) == 0 {
		return s.newValue1I(ssa.OpSelectNAddr, types.NewPtr(t), which, c)
	}
	_, addr := s.temp(c.Pos, t)
	rval := s.newValue1I(ssa.OpSelectN, t, which, c)
	s.vars[memVar] = s.newValue3Apos(ssa.OpStore, types.TypeMem, t, addr, rval, s.mem(), false)
	return addr
}

// Get backing store information for an append call.
func (s *state) getBackingStoreInfoForAppend(n *ir.CallExpr) *backingStoreInfo {
	if n.Esc() != ir.EscNone {
		return nil
	}
	return s.getBackingStoreInfo(n.Args[0])
}
func (s *state) getBackingStoreInfo(n ir.Node) *backingStoreInfo {
	t := n.Type()
	et := t.Elem()
	maxStackSize := int64(base.Debug.VariableMakeThreshold)
	if et.Size() == 0 || et.Size() > maxStackSize {
		return nil
	}
	if base.Flag.N != 0 {
		return nil
	}
	if !base.VariableMakeHash.MatchPos(n.Pos(), nil) {
		return nil
	}
	i := s.backingStores[n]
	if i != nil {
		return i
	}

	// Build type of backing store.
	K := maxStackSize / et.Size() // rounds down
	KT := types.NewArray(et, K)
	KT.SetNoalg(true)
	types.CalcArraySize(KT)
	// Align more than naturally for the type KT. See issue 73199.
	align := types.NewArray(types.Types[types.TUINTPTR], 0)
	types.CalcArraySize(align)
	storeTyp := types.NewStruct([]*types.Field{
		{Sym: types.BlankSym, Type: align},
		{Sym: types.BlankSym, Type: KT},
	})
	storeTyp.SetNoalg(true)
	types.CalcStructSize(storeTyp)

	// Make backing store variable.
	backingStore := typecheck.TempAt(n.Pos(), s.curfn, storeTyp)
	backingStore.SetAddrtaken(true)

	// Make "used" boolean.
	used := typecheck.TempAt(n.Pos(), s.curfn, types.Types[types.TBOOL])
	if s.curBlock == s.f.Entry {
		s.vars[used] = s.constBool(false)
	} else {
		// initialize this variable at end of entry block
		s.defvars[s.f.Entry.ID][used] = s.constBool(false)
	}

	// Initialize an info structure.
	if s.backingStores == nil {
		s.backingStores = map[ir.Node]*backingStoreInfo{}
	}
	i = &backingStoreInfo{K: K, store: backingStore, used: used, usedStatic: false}
	s.backingStores[n] = i
	return i
}

// append converts an OAPPEND node to SSA.
// If inplace is false, it converts the OAPPEND expression n to an ssa.Value,
// adds it to s, and returns the Value.
// If inplace is true, it writes the result of the OAPPEND expression n
// back to the slice being appended to, and returns nil.
// inplace MUST be set to false if the slice can be SSA'd.
// Note: this code only handles fixed-count appends. Dotdotdot appends
// have already been rewritten at this point (by walk).
func (s *state) append(n *ir.CallExpr, inplace bool) *ssa.Value {
	// If inplace is false, process as expression "append(s, e1, e2, e3)":
	//
	// ptr, len, cap := s
	// len += 3
	// if uint(len) > uint(cap) {
	//     ptr, len, cap = growslice(ptr, len, cap, 3, typ)
	//     Note that len is unmodified by growslice.
	// }
	// // with write barriers, if needed:
	// *(ptr+(len-3)) = e1
	// *(ptr+(len-2)) = e2
	// *(ptr+(len-1)) = e3
	// return makeslice(ptr, len, cap)
	//
	//
	// If inplace is true, process as statement "s = append(s, e1, e2, e3)":
	//
	// a := &s
	// ptr, len, cap := s
	// len += 3
	// if uint(len) > uint(cap) {
	//    ptr, len, cap = growslice(ptr, len, cap, 3, typ)
	//    vardef(a)    // if necessary, advise liveness we are writing a new a
	//    *a.cap = cap // write before ptr to avoid a spill
	//    *a.ptr = ptr // with write barrier
	// }
	// *a.len = len
	// // with write barriers, if needed:
	// *(ptr+(len-3)) = e1
	// *(ptr+(len-2)) = e2
	// *(ptr+(len-1)) = e3

	et := n.Type().Elem()
	pt := types.NewPtr(et)

	// Evaluate slice
	sn := n.Args[0] // the slice node is the first in the list
	var slice, addr *ssa.Value
	if inplace {
		addr = s.addr(sn)
		slice = s.load(n.Type(), addr)
	} else {
		slice = s.expr(sn)
	}

	// Allocate new blocks
	grow := s.f.NewBlock(ssa.BlockPlain)
	assign := s.f.NewBlock(ssa.BlockPlain)

	// Decomposse input slice.
	p := s.newValue1(ssa.OpSlicePtr, pt, slice)
	l := s.newValue1(ssa.OpSliceLen, types.Types[types.TINT], slice)
	c := s.newValue1(ssa.OpSliceCap, types.Types[types.TINT], slice)

	// Add number of new elements to length.
	nargs := s.constInt(types.Types[types.TINT], int64(len(n.Args)-1))
	oldLen := l
	l = s.newValue2(s.ssaOp(ir.OADD, types.Types[types.TINT]), types.Types[types.TINT], l, nargs)

	// Decide if we need to grow
	cmp := s.newValue2(s.ssaOp(ir.OLT, types.Types[types.TUINT]), types.Types[types.TBOOL], c, l)

	// Record values of ptr/len/cap before branch.
	s.vars[ptrVar] = p
	s.vars[lenVar] = l
	if !inplace {
		s.vars[capVar] = c
	}

	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.Likely = ssa.BranchUnlikely
	b.SetControl(cmp)
	b.AddEdgeTo(grow)
	b.AddEdgeTo(assign)

	// If the result of the append does not escape, we can use
	// a stack-allocated backing store if len is small enough.
	// A stack-allocated backing store could be used at every
	// append that qualifies, but we limit it in some cases to
	// avoid wasted code and stack space.
	//
	// Note that we have two different strategies.
	// 1. The standard strategy is just to allocate the full
	//    backing store at the first append.
	// 2. An alternate strategy is used when
	//        a. The backing store eventually escapes via move2heap
	//    and b. The capacity is used somehow
	//    In this case, we don't want to just allocate
	//    the full buffer at the first append, because when
	//    we move2heap the buffer to the heap when it escapes,
	//    we might end up wasting memory because we can't
	//    change the capacity.
	//    So in this case we use growsliceBuf to reuse the buffer
	//    and walk one step up the size class ladder each time.
	//
	// TODO: handle ... append case? Currently we handle only
	// a fixed number of appended elements.
	var info *backingStoreInfo
	if !inplace {
		info = s.getBackingStoreInfoForAppend(n)
	}

	if !inplace && info != nil && !n.UseBuf && !info.usedStatic {
		// if l <= K {
		//   if !used {
		//     if oldLen == 0 {
		//       var store [K]T
		//       s = store[:l:K]
		//       used = true
		//     }
		//   }
		// }
		// ... if we didn't use the stack backing store, call growslice ...
		//
		// oldLen==0 is not strictly necessary, but requiring it means
		// we don't have to worry about copying existing elements.
		// Allowing oldLen>0 would add complication. Worth it? I would guess not.
		//
		// TODO: instead of the used boolean, we could insist that this only applies
		// to monotonic slices, those which once they have >0 entries never go back
		// to 0 entries. Then oldLen==0 is enough.
		//
		// We also do this for append(x, ...) once for every x.
		// It is ok to do it more often, but it is probably helpful only for
		// the first instance. TODO: this could use more tuning. Using ir.Node
		// as the key works for *ir.Name instances but probably nothing else.
		info.usedStatic = true
		// TODO: unset usedStatic somehow?

		usedTestBlock := s.f.NewBlock(ssa.BlockPlain)
		oldLenTestBlock := s.f.NewBlock(ssa.BlockPlain)
		bodyBlock := s.f.NewBlock(ssa.BlockPlain)
		growSlice := s.f.NewBlock(ssa.BlockPlain)
		tInt := types.Types[types.TINT]
		tBool := types.Types[types.TBOOL]

		// if l <= K
		s.startBlock(grow)
		kTest := s.newValue2(s.ssaOp(ir.OLE, tInt), tBool, l, s.constInt(tInt, info.K))
		b := s.endBlock()
		b.Kind = ssa.BlockIf
		b.SetControl(kTest)
		b.AddEdgeTo(usedTestBlock)
		b.AddEdgeTo(growSlice)
		b.Likely = ssa.BranchLikely

		// if !used
		s.startBlock(usedTestBlock)
		usedTest := s.newValue1(ssa.OpNot, tBool, s.expr(info.used))
		b = s.endBlock()
		b.Kind = ssa.BlockIf
		b.SetControl(usedTest)
		b.AddEdgeTo(oldLenTestBlock)
		b.AddEdgeTo(growSlice)
		b.Likely = ssa.BranchLikely

		// if oldLen == 0
		s.startBlock(oldLenTestBlock)
		oldLenTest := s.newValue2(s.ssaOp(ir.OEQ, tInt), tBool, oldLen, s.constInt(tInt, 0))
		b = s.endBlock()
		b.Kind = ssa.BlockIf
		b.SetControl(oldLenTest)
		b.AddEdgeTo(bodyBlock)
		b.AddEdgeTo(growSlice)
		b.Likely = ssa.BranchLikely

		// var store struct { _ [0]uintptr; arr [K]T }
		s.startBlock(bodyBlock)
		if et.HasPointers() {
			s.vars[memVar] = s.newValue1A(ssa.OpVarDef, types.TypeMem, info.store, s.mem())
		}
		addr := s.addr(info.store)
		s.zero(info.store.Type(), addr)

		// s = store.arr[:l:K]
		s.vars[ptrVar] = addr
		s.vars[lenVar] = l // nargs would also be ok because of the oldLen==0 test.
		s.vars[capVar] = s.constInt(tInt, info.K)

		// used = true
		s.assign(info.used, s.constBool(true), false, 0)
		b = s.endBlock()
		b.AddEdgeTo(assign)

		// New block to use for growslice call.
		grow = growSlice
	}

	// Call growslice
	s.startBlock(grow)
	taddr := s.expr(n.Fun)
	var r []*ssa.Value
	if info != nil && n.UseBuf {
		// Use stack-allocated buffer as backing store, if we can.
		if et.HasPointers() && !info.usedStatic {
			// Initialize in the function header. Not the best place,
			// but it makes sure we don't scan this area before it is
			// initialized.
			mem := s.defvars[s.f.Entry.ID][memVar]
			mem = s.f.Entry.NewValue1A(n.Pos(), ssa.OpVarDef, types.TypeMem, info.store, mem)
			addr := s.f.Entry.NewValue2A(n.Pos(), ssa.OpLocalAddr, types.NewPtr(info.store.Type()), info.store, s.sp, mem)
			mem = s.f.Entry.NewValue2I(n.Pos(), ssa.OpZero, types.TypeMem, info.store.Type().Size(), addr, mem)
			mem.Aux = info.store.Type()
			s.defvars[s.f.Entry.ID][memVar] = mem
			info.usedStatic = true
		}
		fn := ir.Syms.GrowsliceBuf
		if goexperiment.RuntimeFreegc && n.AppendNoAlias && !et.HasPointers() {
			// The append is for a non-aliased slice where the runtime knows how to free
			// the old logically dead backing store after growth.
			// TODO(thepudds): for now, we only use the NoAlias version for element types
			// without pointers while waiting on additional runtime support (CL 698515).
			fn = ir.Syms.GrowsliceBufNoAlias
		}
		r = s.rtcall(fn, true, []*types.Type{n.Type()}, p, l, c, nargs, taddr, s.addr(info.store), s.constInt(types.Types[types.TINT], info.K))
	} else {
		fn := ir.Syms.Growslice
		if goexperiment.RuntimeFreegc && n.AppendNoAlias && !et.HasPointers() {
			// The append is for a non-aliased slice where the runtime knows how to free
			// the old logically dead backing store after growth.
			// TODO(thepudds): for now, we only use the NoAlias version for element types
			// without pointers while waiting on additional runtime support (CL 698515).
			fn = ir.Syms.GrowsliceNoAlias
		}
		r = s.rtcall(fn, true, []*types.Type{n.Type()}, p, l, c, nargs, taddr)
	}

	// Decompose output slice
	p = s.newValue1(ssa.OpSlicePtr, pt, r[0])
	l = s.newValue1(ssa.OpSliceLen, types.Types[types.TINT], r[0])
	c = s.newValue1(ssa.OpSliceCap, types.Types[types.TINT], r[0])

	s.vars[ptrVar] = p
	s.vars[lenVar] = l
	s.vars[capVar] = c
	if inplace {
		if sn.Op() == ir.ONAME {
			sn := sn.(*ir.Name)
			if sn.Class != ir.PEXTERN {
				// Tell liveness we're about to build a new slice
				s.vars[memVar] = s.newValue1A(ssa.OpVarDef, types.TypeMem, sn, s.mem())
			}
		}
		capaddr := s.newValue1I(ssa.OpOffPtr, s.f.Config.Types.IntPtr, types.SliceCapOffset, addr)
		s.store(types.Types[types.TINT], capaddr, c)
		s.store(pt, addr, p)
	}

	b = s.endBlock()
	b.AddEdgeTo(assign)

	// assign new elements to slots
	s.startBlock(assign)
	p = s.variable(ptrVar, pt)                      // generates phi for ptr
	l = s.variable(lenVar, types.Types[types.TINT]) // generates phi for len
	if !inplace {
		c = s.variable(capVar, types.Types[types.TINT]) // generates phi for cap
	}

	if inplace {
		// Update length in place.
		// We have to wait until here to make sure growslice succeeded.
		lenaddr := s.newValue1I(ssa.OpOffPtr, s.f.Config.Types.IntPtr, types.SliceLenOffset, addr)
		s.store(types.Types[types.TINT], lenaddr, l)
	}

	// Evaluate args
	type argRec struct {
		// if store is true, we're appending the value v.  If false, we're appending the
		// value at *v.
		v     *ssa.Value
		store bool
	}
	args := make([]argRec, 0, len(n.Args[1:]))
	for _, n := range n.Args[1:] {
		if ssa.CanSSA(n.Type()) {
			args = append(args, argRec{v: s.expr(n), store: true})
		} else {
			v := s.addr(n)
			args = append(args, argRec{v: v})
		}
	}

	// Write args into slice.
	oldLen = s.newValue2(s.ssaOp(ir.OSUB, types.Types[types.TINT]), types.Types[types.TINT], l, nargs)
	p2 := s.newValue2(ssa.OpPtrIndex, pt, p, oldLen)
	for i, arg := range args {
		addr := s.newValue2(ssa.OpPtrIndex, pt, p2, s.constInt(types.Types[types.TINT], int64(i)))
		if arg.store {
			s.storeType(et, addr, arg.v, 0, true)
		} else {
			s.move(et, addr, arg.v)
		}
	}

	// The following deletions have no practical effect at this time
	// because state.vars has been reset by the preceding state.startBlock.
	// They only enforce the fact that these variables are no longer need in
	// the current scope.
	delete(s.vars, ptrVar)
	delete(s.vars, lenVar)
	if !inplace {
		delete(s.vars, capVar)
	}

	// make result
	if inplace {
		return nil
	}
	return s.newValue3(ssa.OpSliceMake, n.Type(), p, l, c)
}

func (s *state) move2heap(n *ir.MoveToHeapExpr) *ssa.Value {
	// s := n.Slice
	// if s.ptr points to current stack frame {
	//     s2 := make([]T, s.len, s.cap)
	//     copy(s2[:cap], s[:cap])
	//     s = s2
	// }
	// return s

	slice := s.expr(n.Slice)
	et := slice.Type.Elem()
	pt := types.NewPtr(et)

	info := s.getBackingStoreInfo(n)
	if info == nil {
		// Backing store will never be stack allocated, so
		// move2heap is a no-op.
		return slice
	}

	// Decomposse input slice.
	p := s.newValue1(ssa.OpSlicePtr, pt, slice)
	l := s.newValue1(ssa.OpSliceLen, types.Types[types.TINT], slice)
	c := s.newValue1(ssa.OpSliceCap, types.Types[types.TINT], slice)

	moveBlock := s.f.NewBlock(ssa.BlockPlain)
	mergeBlock := s.f.NewBlock(ssa.BlockPlain)

	s.vars[ptrVar] = p
	s.vars[lenVar] = l
	s.vars[capVar] = c

	// Decide if we need to move the slice backing store.
	// It needs to be moved if it is currently on the stack.
	sub := ssa.OpSub64
	less := ssa.OpLess64U
	if s.config.PtrSize == 4 {
		sub = ssa.OpSub32
		less = ssa.OpLess32U
	}
	callerSP := s.newValue1(ssa.OpGetCallerSP, types.Types[types.TUINTPTR], s.mem())
	frameSize := s.newValue2(sub, types.Types[types.TUINTPTR], callerSP, s.sp)
	pInt := s.newValue2(ssa.OpConvert, types.Types[types.TUINTPTR], p, s.mem())
	off := s.newValue2(sub, types.Types[types.TUINTPTR], pInt, s.sp)
	cond := s.newValue2(less, types.Types[types.TBOOL], off, frameSize)

	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.Likely = ssa.BranchUnlikely // fast path is to not have to call into runtime
	b.SetControl(cond)
	b.AddEdgeTo(moveBlock)
	b.AddEdgeTo(mergeBlock)

	// Move the slice to heap
	s.startBlock(moveBlock)
	var newSlice *ssa.Value
	if et.HasPointers() {
		typ := s.expr(n.RType)
		if n.PreserveCapacity {
			newSlice = s.rtcall(ir.Syms.MoveSlice, true, []*types.Type{slice.Type}, typ, p, l, c)[0]
		} else {
			newSlice = s.rtcall(ir.Syms.MoveSliceNoCap, true, []*types.Type{slice.Type}, typ, p, l)[0]
		}
	} else {
		elemSize := s.constInt(types.Types[types.TUINTPTR], et.Size())
		if n.PreserveCapacity {
			newSlice = s.rtcall(ir.Syms.MoveSliceNoScan, true, []*types.Type{slice.Type}, elemSize, p, l, c)[0]
		} else {
			newSlice = s.rtcall(ir.Syms.MoveSliceNoCapNoScan, true, []*types.Type{slice.Type}, elemSize, p, l)[0]
		}
	}
	// Decompose output slice
	s.vars[ptrVar] = s.newValue1(ssa.OpSlicePtr, pt, newSlice)
	s.vars[lenVar] = s.newValue1(ssa.OpSliceLen, types.Types[types.TINT], newSlice)
	s.vars[capVar] = s.newValue1(ssa.OpSliceCap, types.Types[types.TINT], newSlice)
	b = s.endBlock()
	b.AddEdgeTo(mergeBlock)

	// Merge fast path (no moving) and slow path (moved)
	s.startBlock(mergeBlock)
	p = s.variable(ptrVar, pt)                      // generates phi for ptr
	l = s.variable(lenVar, types.Types[types.TINT]) // generates phi for len
	c = s.variable(capVar, types.Types[types.TINT]) // generates phi for cap
	delete(s.vars, ptrVar)
	delete(s.vars, lenVar)
	delete(s.vars, capVar)
	return s.newValue3(ssa.OpSliceMake, slice.Type, p, l, c)
}

// minMax converts an OMIN/OMAX builtin call into SSA.
func (s *state) minMax(n *ir.CallExpr) *ssa.Value {
	// The OMIN/OMAX builtin is variadic, but its semantics are
	// equivalent to left-folding a binary min/max operation across the
	// arguments list.
	fold := func(op func(x, a *ssa.Value) *ssa.Value) *ssa.Value {
		x := s.expr(n.Args[0])
		for _, arg := range n.Args[1:] {
			x = op(x, s.expr(arg))
		}
		return x
	}

	typ := n.Type()

	if typ.IsFloat() || typ.IsString() {
		// min/max semantics for floats are tricky because of NaNs and
		// negative zero. Some architectures have instructions which
		// we can use to generate the right result. For others we must
		// call into the runtime instead.
		//
		// Strings are conceptually simpler, but we currently desugar
		// string comparisons during walk, not ssagen.

		if typ.IsFloat() {
			hasIntrinsic := false
			switch Arch.LinkArch.Family {
			case sys.AMD64, sys.ARM64, sys.Loong64, sys.RISCV64, sys.S390X:
				hasIntrinsic = true
			case sys.PPC64:
				hasIntrinsic = buildcfg.GOPPC64 >= 9
			}

			if hasIntrinsic {
				var op ssa.Op
				switch {
				case typ.Kind() == types.TFLOAT64 && n.Op() == ir.OMIN:
					op = ssa.OpMin64F
				case typ.Kind() == types.TFLOAT64 && n.Op() == ir.OMAX:
					op = ssa.OpMax64F
				case typ.Kind() == types.TFLOAT32 && n.Op() == ir.OMIN:
					op = ssa.OpMin32F
				case typ.Kind() == types.TFLOAT32 && n.Op() == ir.OMAX:
					op = ssa.OpMax32F
				}
				return fold(func(x, a *ssa.Value) *ssa.Value {
					return s.newValue2(op, typ, x, a)
				})
			}
		}
		var name string
		switch typ.Kind() {
		case types.TFLOAT32:
			switch n.Op() {
			case ir.OMIN:
				name = "fmin32"
			case ir.OMAX:
				name = "fmax32"
			}
		case types.TFLOAT64:
			switch n.Op() {
			case ir.OMIN:
				name = "fmin64"
			case ir.OMAX:
				name = "fmax64"
			}
		case types.TSTRING:
			switch n.Op() {
			case ir.OMIN:
				name = "strmin"
			case ir.OMAX:
				name = "strmax"
			}
		}
		fn := typecheck.LookupRuntimeFunc(name)

		return fold(func(x, a *ssa.Value) *ssa.Value {
			return s.rtcall(fn, true, []*types.Type{typ}, x, a)[0]
		})
	}

	if typ.IsInteger() {
		if Arch.LinkArch.Family == sys.RISCV64 && buildcfg.GORISCV64 >= 22 && typ.Size() == 8 {
			var op ssa.Op
			switch {
			case typ.IsSigned() && n.Op() == ir.OMIN:
				op = ssa.OpMin64
			case typ.IsSigned() && n.Op() == ir.OMAX:
				op = ssa.OpMax64
			case typ.IsUnsigned() && n.Op() == ir.OMIN:
				op = ssa.OpMin64u
			case typ.IsUnsigned() && n.Op() == ir.OMAX:
				op = ssa.OpMax64u
			}
			return fold(func(x, a *ssa.Value) *ssa.Value {
				return s.newValue2(op, typ, x, a)
			})
		}
	}

	lt := s.ssaOp(ir.OLT, typ)

	return fold(func(x, a *ssa.Value) *ssa.Value {
		switch n.Op() {
		case ir.OMIN:
			// a < x ? a : x
			return s.ternary(s.newValue2(lt, types.Types[types.TBOOL], a, x), a, x)
		case ir.OMAX:
			// x < a ? a : x
			return s.ternary(s.newValue2(lt, types.Types[types.TBOOL], x, a), a, x)
		}
		panic("unreachable")
	})
}

// ternary emits code to evaluate cond ? x : y.
func (s *state) ternary(cond, x, y *ssa.Value) *ssa.Value {
	// Note that we need a new ternaryVar each time (unlike okVar where we can
	// reuse the variable) because it might have a different type every time.
	ternaryVar := ssaMarker("ternary")

	bThen := s.f.NewBlock(ssa.BlockPlain)
	bElse := s.f.NewBlock(ssa.BlockPlain)
	bEnd := s.f.NewBlock(ssa.BlockPlain)

	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.SetControl(cond)
	b.AddEdgeTo(bThen)
	b.AddEdgeTo(bElse)

	s.startBlock(bThen)
	s.vars[ternaryVar] = x
	s.endBlock().AddEdgeTo(bEnd)

	s.startBlock(bElse)
	s.vars[ternaryVar] = y
	s.endBlock().AddEdgeTo(bEnd)

	s.startBlock(bEnd)
	r := s.variable(ternaryVar, x.Type)
	delete(s.vars, ternaryVar)
	return r
}

// condBranch evaluates the boolean expression cond and branches to yes
// if cond is true and no if cond is false.
// This function is intended to handle && and || better than just calling
// s.expr(cond) and branching on the result.
func (s *state) condBranch(cond ir.Node, yes, no *ssa.Block, likely int8) {
	switch cond.Op() {
	case ir.OANDAND:
		cond := cond.(*ir.LogicalExpr)
		mid := s.f.NewBlock(ssa.BlockPlain)
		s.stmtList(cond.Init())
		s.condBranch(cond.X, mid, no, max(likely, 0))
		s.startBlock(mid)
		s.condBranch(cond.Y, yes, no, likely)
		return
		// Note: if likely==1, then both recursive calls pass 1.
		// If likely==-1, then we don't have enough information to decide
		// whether the first branch is likely or not. So we pass 0 for
		// the likeliness of the first branch.
		// TODO: have the frontend give us branch prediction hints for
		// OANDAND and OOROR nodes (if it ever has such info).
	case ir.OOROR:
		cond := cond.(*ir.LogicalExpr)
		mid := s.f.NewBlock(ssa.BlockPlain)
		s.stmtList(cond.Init())
		s.condBranch(cond.X, yes, mid, min(likely, 0))
		s.startBlock(mid)
		s.condBranch(cond.Y, yes, no, likely)
		return
		// Note: if likely==-1, then both recursive calls pass -1.
		// If likely==1, then we don't have enough info to decide
		// the likelihood of the first branch.
	case ir.ONOT:
		cond := cond.(*ir.UnaryExpr)
		s.stmtList(cond.Init())
		s.condBranch(cond.X, no, yes, -likely)
		return
	case ir.OCONVNOP:
		cond := cond.(*ir.ConvExpr)
		s.stmtList(cond.Init())
		s.condBranch(cond.X, yes, no, likely)
		return
	}
	c := s.expr(cond)
	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.SetControl(c)
	b.Likely = ssa.BranchPrediction(likely) // gc and ssa both use -1/0/+1 for likeliness
	b.AddEdgeTo(yes)
	b.AddEdgeTo(no)
}

type skipMask uint8

const (
	skipPtr skipMask = 1 << iota
	skipLen
	skipCap
)

// assign does left = right.
// Right has already been evaluated to ssa, left has not.
// If deref is true, then we do left = *right instead (and right has already been nil-checked).
// If deref is true and right == nil, just do left = 0.
// skip indicates assignments (at the top level) that can be avoided.
// mayOverlap indicates whether left&right might partially overlap in memory. Default is false.
func (s *state) assign(left ir.Node, right *ssa.Value, deref bool, skip skipMask) {
	s.assignWhichMayOverlap(left, right, deref, skip, false)
}
func (s *state) assignWhichMayOverlap(left ir.Node, right *ssa.Value, deref bool, skip skipMask, mayOverlap bool) {
	if left.Op() == ir.ONAME && ir.IsBlank(left) {
		return
	}
	t := left.Type()
	types.CalcSize(t)
	if s.canSSA(left) {
		if deref {
			s.Fatalf("can SSA LHS %v but not RHS %s", left, right)
		}
		if left.Op() == ir.ODOT {
			// We're assigning to a field of an ssa-able value.
			// We need to build a new structure with the new value for the
			// field we're assigning and the old values for the other fields.
			// For instance:
			//   type T struct {a, b, c int}
			//   var T x
			//   x.b = 5
			// For the x.b = 5 assignment we want to generate x = T{x.a, 5, x.c}

			// Grab information about the structure type.
			left := left.(*ir.SelectorExpr)
			t := left.X.Type()
			nf := t.NumFields()
			idx := fieldIdx(left)

			// Grab old value of structure.
			old := s.expr(left.X)

			if left.Type().Size() == 0 {
				// Nothing to do when assigning zero-sized things.
				return
			}

			// Make new structure.
			new := s.newValue0(ssa.OpStructMake, t)

			// Add fields as args.
			for i := 0; i < nf; i++ {
				if i == idx {
					new.AddArg(right)
				} else {
					new.AddArg(s.newValue1I(ssa.OpStructSelect, t.FieldType(i), int64(i), old))
				}
			}

			// Recursively assign the new value we've made to the base of the dot op.
			s.assign(left.X, new, false, 0)
			// TODO: do we need to update named values here?
			return
		}
		if left.Op() == ir.OINDEX && left.(*ir.IndexExpr).X.Type().IsArray() {
			left := left.(*ir.IndexExpr)
			s.pushLine(left.Pos())
			defer s.popLine()
			// We're assigning to an element of an ssa-able array.
			// a[i] = v
			t := left.X.Type()
			n := t.NumElem()

			i := s.expr(left.Index) // index
			if n == 0 {
				_ = s.expr(left.X) // Evaluating left.X for any side-effects.
				// The bounds check must fail.  Might as well
				// ignore the actual index and just use zeros.
				z := s.constInt(types.Types[types.TINT], 0)
				s.boundsCheck(z, z, ssa.BoundsIndex, false)
				return
			}
			if t.Size() == 0 {
				_ = s.expr(left.X) // Evaluating left.X for any side-effects.
				// Generate bounds check for left, since this can happen
				// for 0-size assignment case, see issue #79236.
				len := s.constInt(types.Types[types.TINT], n)
				s.boundsCheck(i, len, ssa.BoundsIndex, false)
				return
			}
			if n != 1 {
				// This can happen in weird, always-panics cases, like:
				//     var x [0][2]int
				//     x[i][j] = 5
				// We know it always panics because the LHS is ssa-able,
				// and arrays of length > 1 can't be ssa-able unless
				// they are somewhere inside an outer [0].
				// We can ignore the actual assignment, it is dynamically
				// unreachable. See issue 77635.
				// Still, evaluating left.X for any side-effects.
				_ = s.expr(left.X)
				return
			}

			// Rewrite to a = [1]{v}
			len := s.constInt(types.Types[types.TINT], 1)
			s.boundsCheck(i, len, ssa.BoundsIndex, false) // checks i == 0
			v := s.newValue1(ssa.OpArrayMake1, t, right)
			s.assign(left.X, v, false, 0)
			return
		}
		left := left.(*ir.Name)
		// Update variable assignment.
		s.vars[left] = right
		s.addNamedValue(left, right)
		return
	}

	// If this assignment clobbers an entire local variable, then emit
	// OpVarDef so liveness analysis knows the variable is redefined.
	if base, ok := clobberBase(left).(*ir.Name); ok && base.OnStack() && skip == 0 && (t.HasPointers() || ssa.IsMergeCandidate(base)) {
		s.vars[memVar] = s.newValue1Apos(ssa.OpVarDef, types.TypeMem, base, s.mem(), !ir.IsAutoTmp(base))
	}

	// Left is not ssa-able. Compute its address.
	addr := s.addr(left)
	if ir.IsReflectHeaderDataField(left) {
		// Package unsafe's documentation says storing pointers into
		// reflect.SliceHeader and reflect.StringHeader's Data fields
		// is valid, even though they have type uintptr (#19168).
		// Mark it pointer type to signal the writebarrier pass to
		// insert a write barrier.
		t = types.Types[types.TUNSAFEPTR]
	}
	if deref {
		// Treat as a mem->mem move.
		if right == nil {
			s.zero(t, addr)
		} else {
			s.moveWhichMayOverlap(t, addr, right, mayOverlap)
		}
		return
	}
	// Treat as a store.
	s.storeType(t, addr, right, skip, !ir.IsAutoTmp(left))
}

// zeroVal returns the zero value for type t.
func (s *state) zeroVal(t *types.Type) *ssa.Value {
	if t.Size() == 0 {
		return s.entryNewValue0(ssa.OpEmpty, t)
	}
	switch {
	case t.IsInteger():
		switch t.Size() {
		case 1:
			return s.constInt8(t, 0)
		case 2:
			return s.constInt16(t, 0)
		case 4:
			return s.constInt32(t, 0)
		case 8:
			return s.constInt64(t, 0)
		default:
			s.Fatalf("bad sized integer type %v", t)
		}
	case t.IsFloat():
		switch t.Size() {
		case 4:
			return s.constFloat32(t, 0)
		case 8:
			return s.constFloat64(t, 0)
		default:
			s.Fatalf("bad sized float type %v", t)
		}
	case t.IsComplex():
		switch t.Size() {
		case 8:
			z := s.constFloat32(types.Types[types.TFLOAT32], 0)
			return s.entryNewValue2(ssa.OpComplexMake, t, z, z)
		case 16:
			z := s.constFloat64(types.Types[types.TFLOAT64], 0)
			return s.entryNewValue2(ssa.OpComplexMake, t, z, z)
		default:
			s.Fatalf("bad sized complex type %v", t)
		}

	case t.IsString():
		return s.constEmptyString(t)
	case t.IsPtrShaped():
		return s.constNil(t)
	case t.IsBoolean():
		return s.constBool(false)
	case t.IsInterface():
		return s.constInterface(t)
	case t.IsSlice():
		return s.constSlice(t)
	case isStructNotSIMD(t):
		n := t.NumFields()
		v := s.entryNewValue0(ssa.OpStructMake, t)
		for i := 0; i < n; i++ {
			v.AddArg(s.zeroVal(t.FieldType(i)))
		}
		return v
	case t.IsArray() && t.NumElem() == 1:
		return s.entryNewValue1(ssa.OpArrayMake1, t, s.zeroVal(t.Elem()))
	case t.IsSIMD():
		return s.newValue0(ssa.OpZeroSIMD, t)
	}
	s.Fatalf("zero for type %v not implemented", t)
	return nil
}

type callKind int8

const (
	callNormal callKind = iota
	callDefer
	callDeferStack
	callGo
	callTail
)

type sfRtCallDef struct {
	rtfn  *obj.LSym
	rtype types.Kind
}

var softFloatOps map[ssa.Op]sfRtCallDef

func softfloatInit() {
	// Some of these operations get transformed by sfcall.
	softFloatOps = map[ssa.Op]sfRtCallDef{
		ssa.OpAdd32F: {typecheck.LookupRuntimeFunc("fadd32"), types.TFLOAT32},
		ssa.OpAdd64F: {typecheck.LookupRuntimeFunc("fadd64"), types.TFLOAT64},
		ssa.OpSub32F: {typecheck.LookupRuntimeFunc("fadd32"), types.TFLOAT32},
		ssa.OpSub64F: {typecheck.LookupRuntimeFunc("fadd64"), types.TFLOAT64},
		ssa.OpMul32F: {typecheck.LookupRuntimeFunc("fmul32"), types.TFLOAT32},
		ssa.OpMul64F: {typecheck.LookupRuntimeFunc("fmul64"), types.TFLOAT64},
		ssa.OpDiv32F: {typecheck.LookupRuntimeFunc("fdiv32"), types.TFLOAT32},
		ssa.OpDiv64F: {typecheck.LookupRuntimeFunc("fdiv64"), types.TFLOAT64},

		ssa.OpEq64F:   {typecheck.LookupRuntimeFunc("feq64"), types.TBOOL},
		ssa.OpEq32F:   {typecheck.LookupRuntimeFunc("feq32"), types.TBOOL},
		ssa.OpNeq64F:  {typecheck.LookupRuntimeFunc("feq64"), types.TBOOL},
		ssa.OpNeq32F:  {typecheck.LookupRuntimeFunc("feq32"), types.TBOOL},
		ssa.OpLess64F: {typecheck.LookupRuntimeFunc("fgt64"), types.TBOOL},
		ssa.OpLess32F: {typecheck.LookupRuntimeFunc("fgt32"), types.TBOOL},
		ssa.OpLeq64F:  {typecheck.LookupRuntimeFunc("fge64"), types.TBOOL},
		ssa.OpLeq32F:  {typecheck.LookupRuntimeFunc("fge32"), types.TBOOL},

		ssa.OpCvt32to32F:  {typecheck.LookupRuntimeFunc("fint32to32"), types.TFLOAT32},
		ssa.OpCvt32Fto32:  {typecheck.LookupRuntimeFunc("f32toint32"), types.TINT32},
		ssa.OpCvt64to32F:  {typecheck.LookupRuntimeFunc("fint64to32"), types.TFLOAT32},
		ssa.OpCvt32Fto64:  {typecheck.LookupRuntimeFunc("f32toint64"), types.TINT64},
		ssa.OpCvt64Uto32F: {typecheck.LookupRuntimeFunc("fuint64to32"), types.TFLOAT32},
		ssa.OpCvt32Fto64U: {typecheck.LookupRuntimeFunc("f32touint64"), types.TUINT64},
		ssa.OpCvt32to64F:  {typecheck.LookupRuntimeFunc("fint32to64"), types.TFLOAT64},
		ssa.OpCvt64Fto32:  {typecheck.LookupRuntimeFunc("f64toint32"), types.TINT32},
		ssa.OpCvt64to64F:  {typecheck.LookupRuntimeFunc("fint64to64"), types.TFLOAT64},
		ssa.OpCvt64Fto64:  {typecheck.LookupRuntimeFunc("f64toint64"), types.TINT64},
		ssa.OpCvt64Uto64F: {typecheck.LookupRuntimeFunc("fuint64to64"), types.TFLOAT64},
		ssa.OpCvt64Fto64U: {typecheck.LookupRuntimeFunc("f64touint64"), types.TUINT64},
		ssa.OpCvt32Fto64F: {typecheck.LookupRuntimeFunc("f32to64"), types.TFLOAT64},
		ssa.OpCvt64Fto32F: {typecheck.LookupRuntimeFunc("f64to32"), types.TFLOAT32},
	}
}

// TODO: do not emit sfcall if operation can be optimized to constant in later
// opt phase
func (s *state) sfcall(op ssa.Op, args ...*ssa.Value) (*ssa.Value, bool) {
	f2i := func(t *types.Type) *types.Type {
		switch t.Kind() {
		case types.TFLOAT32:
			return types.Types[types.TUINT32]
		case types.TFLOAT64:
			return types.Types[types.TUINT64]
		}
		return t
	}

	if callDef, ok := softFloatOps[op]; ok {
		switch op {
		case ssa.OpLess32F,
			ssa.OpLess64F,
			ssa.OpLeq32F,
			ssa.OpLeq64F:
			args[0], args[1] = args[1], args[0]
		case ssa.OpSub32F,
			ssa.OpSub64F:
			args[1] = s.newValue1(s.ssaOp(ir.ONEG, types.Types[callDef.rtype]), args[1].Type, args[1])
		}

		// runtime functions take uints for floats and returns uints.
		// Convert to uints so we use the right calling convention.
		for i, a := range args {
			if a.Type.IsFloat() {
				args[i] = s.newValue1(ssa.OpCopy, f2i(a.Type), a)
			}
		}

		rt := types.Types[callDef.rtype]
		result := s.rtcall(callDef.rtfn, true, []*types.Type{f2i(rt)}, args...)[0]
		if rt.IsFloat() {
			result = s.newValue1(ssa.OpCopy, rt, result)
		}
		if op == ssa.OpNeq32F || op == ssa.OpNeq64F {
			result = s.newValue1(ssa.OpNot, result.Type, result)
		}
		return result, true
	}
	return nil, false
}

// split breaks up a tuple-typed value into its 2 parts.
func (s *state) split(v *ssa.Value) (*ssa.Value, *ssa.Value) {
	p0 := s.newValue1(ssa.OpSelect0, v.Type.FieldType(0), v)
	p1 := s.newValue1(ssa.OpSelect1, v.Type.FieldType(1), v)
	return p0, p1
}

// intrinsicCall converts a call to a recognized intrinsic function into the intrinsic SSA operation.
func (s *state) intrinsicCall(n *ir.CallExpr) *ssa.Value {
	v := findIntrinsic(n.Fun.Sym())(s, n, s.intrinsicArgs(n))
	if ssa.IntrinsicsDebug > 0 {
		x := v
		if x == nil {
			x = s.mem()
		}
		if x.Op == ssa.OpSelect0 || x.Op == ssa.OpSelect1 {
			x = x.Args[0]
		}
		base.WarnfAt(n.Pos(), "intrinsic substitution for %v with %s", n.Fun.Sym().Name, x.LongString())
	}
	return v
}

// intrinsicArgs extracts args from n, evaluates them to SSA values, and returns them.
func (s *state) intrinsicArgs(n *ir.CallExpr) []*ssa.Value {
	args := make([]*ssa.Value, len(n.Args))
	for i, n := range n.Args {
		args[i] = s.expr(n)
	}
	return args
}

// openDeferRecord adds code to evaluate and store the function for an open-code defer
// call, and records info about the defer, so we can generate proper code on the
// exit paths. n is the sub-node of the defer node that is the actual function
// call. We will also record funcdata information on where the function is stored
// (as well as the deferBits variable), and this will enable us to run the proper
// defer calls during panics.
func (s *state) openDeferRecord(n *ir.CallExpr) {
	if len(n.Args) != 0 || n.Op() != ir.OCALLFUNC || n.Fun.Type().NumResults() != 0 {
		s.Fatalf("defer call with arguments or results: %v", n)
	}

	opendefer := &openDeferInfo{
		n: n,
	}
	fn := n.Fun
	// We must always store the function value in a stack slot for the
	// runtime panic code to use. But in the defer exit code, we will
	// call the function directly if it is a static function.
	closureVal := s.expr(fn)
	closure := s.openDeferSave(fn.Type(), closureVal)
	opendefer.closureNode = closure.Aux.(*ir.Name)
	if !(fn.Op() == ir.ONAME && fn.(*ir.Name).Class == ir.PFUNC) {
		opendefer.closure = closure
	}
	index := len(s.openDefers)
	s.openDefers = append(s.openDefers, opendefer)

	// Update deferBits only after evaluation and storage to stack of
	// the function is successful.
	bitvalue := s.constInt8(types.Types[types.TUINT8], 1<<uint(index))
	newDeferBits := s.newValue2(ssa.OpOr8, types.Types[types.TUINT8], s.variable(deferBitsVar, types.Types[types.TUINT8]), bitvalue)
	s.vars[deferBitsVar] = newDeferBits
	s.store(types.Types[types.TUINT8], s.deferBitsAddr, newDeferBits)
}

// openDeferSave generates SSA nodes to store a value (with type t) for an
// open-coded defer at an explicit autotmp location on the stack, so it can be
// reloaded and used for the appropriate call on exit. Type t must be a function type
// (therefore SSAable). val is the value to be stored. The function returns an SSA
// value representing a pointer to the autotmp location.
func (s *state) openDeferSave(t *types.Type, val *ssa.Value) *ssa.Value {
	if !ssa.CanSSA(t) {
		s.Fatalf("openDeferSave of non-SSA-able type %v val=%v", t, val)
	}
	if !t.HasPointers() {
		s.Fatalf("openDeferSave of pointerless type %v val=%v", t, val)
	}
	pos := val.Pos
	temp := typecheck.TempAt(pos.WithNotStmt(), s.curfn, t)
	temp.SetOpenDeferSlot(true)
	temp.SetFrameOffset(int64(len(s.openDefers))) // so cmpstackvarlt can order them
	var addrTemp *ssa.Value
	// Use OpVarLive to make sure stack slot for the closure is not removed by
	// dead-store elimination
	if s.curBlock.ID != s.f.Entry.ID {
		// Force the tmp storing this defer function to be declared in the entry
		// block, so that it will be live for the defer exit code (which will
		// actually access it only if the associated defer call has been activated).
		if t.HasPointers() {
			s.defvars[s.f.Entry.ID][memVar] = s.f.Entry.NewValue1A(src.NoXPos, ssa.OpVarDef, types.TypeMem, temp, s.defvars[s.f.Entry.ID][memVar])
		}
		s.defvars[s.f.Entry.ID][memVar] = s.f.Entry.NewValue1A(src.NoXPos, ssa.OpVarLive, types.TypeMem, temp, s.defvars[s.f.Entry.ID][memVar])
		addrTemp = s.f.Entry.NewValue2A(src.NoXPos, ssa.OpLocalAddr, types.NewPtr(temp.Type()), temp, s.sp, s.defvars[s.f.Entry.ID][memVar])
	} else {
		// Special case if we're still in the entry block. We can't use
		// the above code, since s.defvars[s.f.Entry.ID] isn't defined
		// until we end the entry block with s.endBlock().
		if t.HasPointers() {
			s.vars[memVar] = s.newValue1Apos(ssa.OpVarDef, types.TypeMem, temp, s.mem(), false)
		}
		s.vars[memVar] = s.newValue1Apos(ssa.OpVarLive, types.TypeMem, temp, s.mem(), false)
		addrTemp = s.newValue2Apos(ssa.OpLocalAddr, types.NewPtr(temp.Type()), temp, s.sp, s.mem(), false)
	}
	// Since we may use this temp during exit depending on the
	// deferBits, we must define it unconditionally on entry.
	// Therefore, we must make sure it is zeroed out in the entry
	// block if it contains pointers, else GC may wrongly follow an
	// uninitialized pointer value.
	temp.SetNeedzero(true)
	// We are storing to the stack, hence we can avoid the full checks in
	// storeType() (no write barrier) and do a simple store().
	s.store(t, addrTemp, val)
	return addrTemp
}

// openDeferExit generates SSA for processing all the open coded defers at exit.
// The code involves loading deferBits, and checking each of the bits to see if
// the corresponding defer statement was executed. For each bit that is turned
// on, the associated defer call is made.
func (s *state) openDeferExit() {
	deferExit := s.f.NewBlock(ssa.BlockPlain)
	s.endBlock().AddEdgeTo(deferExit)
	s.startBlock(deferExit)
	s.lastDeferExit = deferExit
	s.lastDeferCount = len(s.openDefers)
	zeroval := s.constInt8(types.Types[types.TUINT8], 0)
	// Test for and run defers in reverse order
	for i := len(s.openDefers) - 1; i >= 0; i-- {
		r := s.openDefers[i]
		bCond := s.f.NewBlock(ssa.BlockPlain)
		bEnd := s.f.NewBlock(ssa.BlockPlain)

		deferBits := s.variable(deferBitsVar, types.Types[types.TUINT8])
		// Generate code to check if the bit associated with the current
		// defer is set.
		bitval := s.constInt8(types.Types[types.TUINT8], 1<<uint(i))
		andval := s.newValue2(ssa.OpAnd8, types.Types[types.TUINT8], deferBits, bitval)
		eqVal := s.newValue2(ssa.OpEq8, types.Types[types.TBOOL], andval, zeroval)
		b := s.endBlock()
		b.Kind = ssa.BlockIf
		b.SetControl(eqVal)
		b.AddEdgeTo(bEnd)
		b.AddEdgeTo(bCond)
		bCond.AddEdgeTo(bEnd)
		s.startBlock(bCond)

		// Clear this bit in deferBits and force store back to stack, so
		// we will not try to re-run this defer call if this defer call panics.
		nbitval := s.newValue1(ssa.OpCom8, types.Types[types.TUINT8], bitval)
		maskedval := s.newValue2(ssa.OpAnd8, types.Types[types.TUINT8], deferBits, nbitval)
		s.store(types.Types[types.TUINT8], s.deferBitsAddr, maskedval)
		// Use this value for following tests, so we keep previous
		// bits cleared.
		s.vars[deferBitsVar] = maskedval

		// Generate code to call the function call of the defer, using the
		// closure that were stored in argtmps at the point of the defer
		// statement.
		fn := r.n.Fun
		stksize := fn.Type().ArgWidth()
		var callArgs []*ssa.Value
		var call *ssa.Value
		if r.closure != nil {
			v := s.load(r.closure.Type.Elem(), r.closure)
			s.maybeNilCheckClosure(v, callDefer)
			codeptr := s.rawLoad(types.Types[types.TUINTPTR], v)
			aux := ssa.ClosureAuxCall(s.f.ABIDefault.ABIAnalyzeTypes(nil, nil))
			call = s.newValue2A(ssa.OpClosureLECall, aux.LateExpansionResultType(), aux, codeptr, v)
		} else {
			aux := ssa.StaticAuxCall(fn.(*ir.Name).Linksym(), s.f.ABIDefault.ABIAnalyzeTypes(nil, nil))
			call = s.newValue0A(ssa.OpStaticLECall, aux.LateExpansionResultType(), aux)
		}
		callArgs = append(callArgs, s.mem())
		call.AddArgs(callArgs...)
		call.AuxInt = stksize
		s.vars[memVar] = s.newValue1I(ssa.OpSelectN, types.TypeMem, 0, call)
		// Make sure that the stack slots with pointers are kept live
		// through the call (which is a pre-emption point). Also, we will
		// use the first call of the last defer exit to compute liveness
		// for the deferreturn, so we want all stack slots to be live.
		if r.closureNode != nil {
			s.vars[memVar] = s.newValue1Apos(ssa.OpVarLive, types.TypeMem, r.closureNode, s.mem(), false)
		}

		s.endBlock()
		s.startBlock(bEnd)
	}
}

func (s *state) callResult(n *ir.CallExpr, k callKind) *ssa.Value {
	return s.call(n, k, false, nil)
}

func (s *state) callAddr(n *ir.CallExpr, k callKind) *ssa.Value {
	return s.call(n, k, true, nil)
}

// Calls the function n using the specified call type.
// Returns the address of the return value (or nil if none).
func (s *state) call(n *ir.CallExpr, k callKind, returnResultAddr bool, deferExtra ir.Expr) *ssa.Value {
	s.prevCall = nil
	var calleeLSym *obj.LSym // target function (if static)
	var closure *ssa.Value   // ptr to closure to run (if dynamic)
	var codeptr *ssa.Value   // ptr to target code (if dynamic)
	var dextra *ssa.Value    // defer extra arg
	var rcvr *ssa.Value      // receiver to set
	fn := n.Fun
	var ACArgs []*types.Type    // AuxCall args
	var ACResults []*types.Type // AuxCall results
	var callArgs []*ssa.Value   // For late-expansion, the args themselves (not stored, args to the call instead).

	callABI := s.f.ABIDefault

	if k != callNormal && k != callTail && (len(n.Args) != 0 || n.Op() == ir.OCALLINTER || n.Fun.Type().NumResults() != 0) {
		s.Fatalf("go/defer call with arguments: %v", n)
	}

	isCallDeferRangeFunc := false

	switch n.Op() {
	case ir.OCALLFUNC:
		if (k == callNormal || k == callTail) && fn.Op() == ir.ONAME && fn.(*ir.Name).Class == ir.PFUNC {
			fn := fn.(*ir.Name)
			calleeLSym = callTargetLSym(fn)
			if buildcfg.Experiment.RegabiArgs {
				// This is a static call, so it may be
				// a direct call to a non-ABIInternal
				// function. fn.Func may be nil for
				// some compiler-generated functions,
				// but those are all ABIInternal.
				if fn.Func != nil {
					callABI = abiForFunc(fn.Func, s.f.ABI0, s.f.ABI1)
				}
			} else {
				// TODO(register args) remove after register abi is working
				inRegistersImported := fn.Pragma()&ir.RegisterParams != 0
				inRegistersSamePackage := fn.Func != nil && fn.Func.Pragma&ir.RegisterParams != 0
				if inRegistersImported || inRegistersSamePackage {
					callABI = s.f.ABI1
				}
			}
			if fn := n.Fun.Sym().Name; n.Fun.Sym().Pkg == ir.Pkgs.Runtime && fn == "deferrangefunc" {
				isCallDeferRangeFunc = true
			}
			break
		}
		closure = s.expr(fn)
		if k != callDefer && k != callDeferStack {
			// Deferred nil function needs to panic when the function is invoked,
			// not the point of defer statement.
			s.maybeNilCheckClosure(closure, k)
		}
	case ir.OCALLINTER:
		if fn.Op() != ir.ODOTINTER {
			s.Fatalf("OCALLINTER: n.Left not an ODOTINTER: %v", fn.Op())
		}
		fn := fn.(*ir.SelectorExpr)
		var iclosure *ssa.Value
		iclosure, rcvr = s.getClosureAndRcvr(fn)
		if k == callNormal || k == callTail {
			codeptr = s.load(types.Types[types.TUINTPTR], iclosure)
		} else {
			closure = iclosure
		}
	}
	if deferExtra != nil {
		dextra = s.expr(deferExtra)
	}

	params := callABI.ABIAnalyze(n.Fun.Type(), false /* Do not set (register) nNames from caller side -- can cause races. */)
	types.CalcSize(fn.Type())
	stksize := params.ArgWidth() // includes receiver, args, and results

	res := n.Fun.Type().Results()
	if k == callNormal || k == callTail {
		for _, p := range params.OutParams() {
			ACResults = append(ACResults, p.Type)
		}
	}

	var call *ssa.Value
	if k == callDeferStack {
		if stksize != 0 {
			s.Fatalf("deferprocStack with non-zero stack size %d: %v", stksize, n)
		}
		// Make a defer struct on the stack.
		t := deferstruct()
		n, addr := s.temp(n.Pos(), t)
		n.SetNonMergeable(true)
		s.store(closure.Type,
			s.newValue1I(ssa.OpOffPtr, closure.Type.PtrTo(), t.FieldOff(deferStructFnField), addr),
			closure)

		// Call runtime.deferprocStack with pointer to _defer record.
		ACArgs = append(ACArgs, types.Types[types.TUINTPTR])
		aux := ssa.StaticAuxCall(ir.Syms.DeferprocStack, s.f.ABIDefault.ABIAnalyzeTypes(ACArgs, ACResults))
		callArgs = append(callArgs, addr, s.mem())
		call = s.newValue0A(ssa.OpStaticLECall, aux.LateExpansionResultType(), aux)
		call.AddArgs(callArgs...)
		call.AuxInt = int64(types.PtrSize) // deferprocStack takes a *_defer arg
	} else {
		// Store arguments to stack, including defer/go arguments and receiver for method calls.
		// These are written in SP-offset order.
		argStart := base.Ctxt.Arch.FixedFrameSize
		// Defer/go args.
		if k != callNormal && k != callTail {
			// Write closure (arg to newproc/deferproc).
			ACArgs = append(ACArgs, types.Types[types.TUINTPTR]) // not argExtra
			callArgs = append(callArgs, closure)
			stksize += int64(types.PtrSize)
			argStart += int64(types.PtrSize)
			if dextra != nil {
				// Extra token of type any for deferproc
				ACArgs = append(ACArgs, types.Types[types.TINTER])
				callArgs = append(callArgs, dextra)
				stksize += 2 * int64(types.PtrSize)
				argStart += 2 * int64(types.PtrSize)
			}
		}

		// Set receiver (for interface calls).
		if rcvr != nil {
			callArgs = append(callArgs, rcvr)
		}

		// Write args.
		t := n.Fun.Type()
		args := n.Args

		for _, p := range params.InParams() { // includes receiver for interface calls
			ACArgs = append(ACArgs, p.Type)
		}

		// Split the entry block if there are open defers, because later calls to
		// openDeferSave may cause a mismatch between the mem for an OpDereference
		// and the call site which uses it. See #49282.
		if s.curBlock.ID == s.f.Entry.ID && s.hasOpenDefers {
			b := s.endBlock()
			b.Kind = ssa.BlockPlain
			curb := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(curb)
			s.startBlock(curb)
		}

		for i, n := range args {
			callArgs = append(callArgs, s.putArg(n, t.Param(i).Type))
		}

		callArgs = append(callArgs, s.mem())

		// call target
		switch {
		case k == callDefer:
			sym := ir.Syms.Deferproc
			if dextra != nil {
				sym = ir.Syms.Deferprocat
			}
			aux := ssa.StaticAuxCall(sym, s.f.ABIDefault.ABIAnalyzeTypes(ACArgs, ACResults)) // TODO paramResultInfo for Deferproc(at)
			call = s.newValue0A(ssa.OpStaticLECall, aux.LateExpansionResultType(), aux)
		case k == callGo:
			aux := ssa.StaticAuxCall(ir.Syms.Newproc, s.f.ABIDefault.ABIAnalyzeTypes(ACArgs, ACResults))
			call = s.newValue0A(ssa.OpStaticLECall, aux.LateExpansionResultType(), aux) // TODO paramResultInfo for Newproc
		case closure != nil:
			// rawLoad because loading the code pointer from a
			// closure is always safe, but IsSanitizerSafeAddr
			// can't always figure that out currently, and it's
			// critical that we not clobber any arguments already
			// stored onto the stack.
			codeptr = s.rawLoad(types.Types[types.TUINTPTR], closure)
			aux := ssa.ClosureAuxCall(callABI.ABIAnalyzeTypes(ACArgs, ACResults))
			call = s.newValue2A(ssa.OpClosureLECall, aux.LateExpansionResultType(), aux, codeptr, closure)
		case codeptr != nil:
			// Note that the "receiver" parameter is nil because the actual receiver is the first input parameter.
			aux := ssa.InterfaceAuxCall(params)
			call = s.newValue1A(ssa.OpInterLECall, aux.LateExpansionResultType(), aux, codeptr)
			if k == callTail {
				call.Op = ssa.OpTailLECallInter
				stksize = 0 // Tail call does not use stack. We reuse caller's frame.
			}
		case calleeLSym != nil:
			aux := ssa.StaticAuxCall(calleeLSym, params)
			call = s.newValue0A(ssa.OpStaticLECall, aux.LateExpansionResultType(), aux)
			if k == callTail {
				call.Op = ssa.OpTailLECall
				stksize = 0 // Tail call does not use stack. We reuse caller's frame.
			}
		default:
			s.Fatalf("bad call type %v %v", n.Op(), n)
		}
		call.AddArgs(callArgs...)
		call.AuxInt = stksize // Call operations carry the argsize of the callee along with them
	}
	s.prevCall = call
	s.vars[memVar] = s.newValue1I(ssa.OpSelectN, types.TypeMem, int64(len(ACResults)), call)
	// Insert VarLive opcodes.
	for _, v := range n.KeepAlive {
		if !v.Addrtaken() {
			s.Fatalf("KeepAlive variable %v must have Addrtaken set", v)
		}
		switch v.Class {
		case ir.PAUTO, ir.PPARAM, ir.PPARAMOUT:
		default:
			s.Fatalf("KeepAlive variable %v must be Auto or Arg", v)
		}
		s.vars[memVar] = s.newValue1A(ssa.OpVarLive, types.TypeMem, v, s.mem())
	}

	// Build result value (before we might end the defer block, below).
	var result *ssa.Value
	if len(res) == 0 || k != callNormal {
		result = nil
	} else {
		fp := res[0]
		if returnResultAddr {
			result = s.resultAddrOfCall(call, 0, fp.Type)
		} else {
			result = s.newValue1I(ssa.OpSelectN, fp.Type, 0, call)
		}
		if n.Reshape {
			result = s.newValue1(ssa.OpCopy, n.Type(), result)
		}
	}

	// Finish block for defers
	if k == callDefer || k == callDeferStack || isCallDeferRangeFunc {
		b := s.endBlock()
		b.Kind = ssa.BlockDefer
		b.SetControl(call)
		bNext := s.f.NewBlock(ssa.BlockPlain)
		b.AddEdgeTo(bNext)
		r := s.f.DeferReturn // Share a single deferreturn among all defers
		if r == nil {
			r = s.f.NewBlock(ssa.BlockPlain)
			s.startBlock(r)
			s.exit()
			s.f.DeferReturn = r
		}
		b.AddEdgeTo(r) // Add recover edge to exit code.  This is a fake edge to keep the block live.
		b.Likely = ssa.BranchLikely
		s.startBlock(bNext)
	}

	return result
}

// maybeNilCheckClosure checks if a nil check of a closure is needed in some
// architecture-dependent situations and, if so, emits the nil check.
func (s *state) maybeNilCheckClosure(closure *ssa.Value, k callKind) {
	if Arch.LinkArch.Family == sys.Wasm || buildcfg.GOOS == "aix" && k != callGo {
		// On AIX, the closure needs to be verified as fn can be nil, except if it's a call go. This needs to be handled by the runtime to have the "go of nil func value" error.
		// TODO(neelance): On other architectures this should be eliminated by the optimization steps
		s.nilCheck(closure)
	}
}

// getClosureAndRcvr returns values for the appropriate closure and receiver of an
// interface call
func (s *state) getClosureAndRcvr(fn *ir.SelectorExpr) (*ssa.Value, *ssa.Value) {
	i := s.expr(fn.X)
	itab := s.newValue1(ssa.OpITab, types.Types[types.TUINTPTR], i)
	s.nilCheck(itab)
	itabidx := fn.Offset() + rttype.ITab.OffsetOf("Fun")
	closure := s.newValue1I(ssa.OpOffPtr, s.f.Config.Types.UintptrPtr, itabidx, itab)
	rcvr := s.newValue1(ssa.OpIData, s.f.Config.Types.BytePtr, i)
	return closure, rcvr
}

// etypesign returns the signed-ness of e, for integer/pointer etypes.
// -1 means signed, +1 means unsigned, 0 means non-integer/non-pointer.
func etypesign(e types.Kind) int8 {
	switch e {
	case types.TINT8, types.TINT16, types.TINT32, types.TINT64, types.TINT:
		return -1
	case types.TUINT8, types.TUINT16, types.TUINT32, types.TUINT64, types.TUINT, types.TUINTPTR, types.TUNSAFEPTR:
		return +1
	}
	return 0
}

// addr converts the address of the expression n to SSA, adds it to s and returns the SSA result.
// The value that the returned Value represents is guaranteed to be non-nil.
func (s *state) addr(n ir.Node) *ssa.Value {
	if n.Op() != ir.ONAME {
		s.pushLine(n.Pos())
		defer s.popLine()
	}

	if s.canSSA(n) {
		// This happens in weird, always-panics cases, like:
		//     var x [0][2]int
		//     x[i][j] = 5
		// The outer assignment, ...[j] = 5, is a fine
		// assignment to do, but requires computing the address
		// &x[i], which will always panic when evaluated.
		// We just return something reasonable in this case.
		// It will be dynamically unreachable. See issue 77635.
		s.boundsCheckArrayIndex(n)
		return s.newValue1A(ssa.OpAddr, n.Type().PtrTo(), ir.Syms.Zerobase, s.sb)
	}

	t := types.NewPtr(n.Type())
	linksymOffset := func(lsym *obj.LSym, offset int64) *ssa.Value {
		v := s.entryNewValue1A(ssa.OpAddr, t, lsym, s.sb)
		// TODO: Make OpAddr use AuxInt as well as Aux.
		if offset != 0 {
			v = s.entryNewValue1I(ssa.OpOffPtr, v.Type, offset, v)
		}
		return v
	}
	switch n.Op() {
	case ir.OLINKSYMOFFSET:
		no := n.(*ir.LinksymOffsetExpr)
		return linksymOffset(no.Linksym, no.Offset_)
	case ir.ONAME:
		n := n.(*ir.Name)
		if n.Heapaddr != nil {
			return s.expr(n.Heapaddr)
		}
		switch n.Class {
		case ir.PEXTERN:
			// global variable
			return linksymOffset(n.Linksym(), 0)
		case ir.PPARAM:
			// parameter slot
			v := s.decladdrs[n]
			if v != nil {
				return v
			}
			s.Fatalf("addr of undeclared ONAME %v. declared: %v", n, s.decladdrs)
			return nil
		case ir.PAUTO:
			return s.newValue2Apos(ssa.OpLocalAddr, t, n, s.sp, s.mem(), !ir.IsAutoTmp(n))

		case ir.PPARAMOUT: // Same as PAUTO -- cannot generate LEA early.
			// ensure that we reuse symbols for out parameters so
			// that cse works on their addresses
			return s.newValue2Apos(ssa.OpLocalAddr, t, n, s.sp, s.mem(), true)
		default:
			s.Fatalf("variable address class %v not implemented", n.Class)
			return nil
		}
	case ir.ORESULT:
		// load return from callee
		n := n.(*ir.ResultExpr)
		return s.resultAddrOfCall(s.prevCall, n.Index, n.Type())
	case ir.OINDEX:
		n := n.(*ir.IndexExpr)
		if n.X.Type().IsSlice() {
			a := s.expr(n.X)
			i := s.expr(n.Index)
			len := s.newValue1(ssa.OpSliceLen, types.Types[types.TINT], a)
			i = s.boundsCheck(i, len, ssa.BoundsIndex, n.Bounded())
			p := s.newValue1(ssa.OpSlicePtr, t, a)
			return s.newValue2(ssa.OpPtrIndex, t, p, i)
		} else { // array
			a := s.addr(n.X)
			i := s.expr(n.Index)
			len := s.constInt(types.Types[types.TINT], n.X.Type().NumElem())
			i = s.boundsCheck(i, len, ssa.BoundsIndex, n.Bounded())
			return s.newValue2(ssa.OpPtrIndex, types.NewPtr(n.X.Type().Elem()), a, i)
		}
	case ir.ODEREF:
		n := n.(*ir.StarExpr)
		return s.exprPtr(n.X, n.Bounded(), n.Pos())
	case ir.ODOT:
		n := n.(*ir.SelectorExpr)
		p := s.addr(n.X)
		return s.newValue1I(ssa.OpOffPtr, t, n.Offset(), p)
	case ir.ODOTPTR:
		n := n.(*ir.SelectorExpr)
		p := s.exprPtr(n.X, n.Bounded(), n.Pos())
		return s.newValue1I(ssa.OpOffPtr, t, n.Offset(), p)
	case ir.OCONVNOP:
		n := n.(*ir.ConvExpr)
		if n.Type() == n.X.Type() {
			return s.addr(n.X)
		}
		addr := s.addr(n.X)
		return s.newValue1(ssa.OpCopy, t, addr) // ensure that addr has the right type
	case ir.OCALLFUNC, ir.OCALLINTER:
		n := n.(*ir.CallExpr)
		return s.callAddr(n, callNormal)
	case ir.ODOTTYPE, ir.ODYNAMICDOTTYPE:
		var v *ssa.Value
		if n.Op() == ir.ODOTTYPE {
			v, _ = s.dottype(n.(*ir.TypeAssertExpr), false)
		} else {
			v, _ = s.dynamicDottype(n.(*ir.DynamicTypeAssertExpr), false)
		}
		if v.Op != ssa.OpLoad {
			s.Fatalf("dottype of non-load")
		}
		if v.Args[1] != s.mem() {
			s.Fatalf("memory no longer live from dottype load")
		}
		return v.Args[0]
	default:
		s.Fatalf("unhandled addr %v", n.Op())
		return nil
	}
}

// canSSA reports whether n is SSA-able.
// n must be an ONAME (or an ODOT sequence with an ONAME base).
func (s *state) canSSA(n ir.Node) bool {
	if base.Flag.N != 0 {
		return false
	}
	for {
		nn := n
		if nn.Op() == ir.ODOT {
			nn := nn.(*ir.SelectorExpr)
			n = nn.X
			continue
		}
		if nn.Op() == ir.OINDEX {
			nn := nn.(*ir.IndexExpr)
			if nn.X.Type().IsArray() {
				n = nn.X
				continue
			}
		}
		break
	}
	if n.Op() != ir.ONAME {
		return false
	}
	return s.canSSAName(n.(*ir.Name)) && ssa.CanSSA(n.Type())
}

func (s *state) canSSAName(name *ir.Name) bool {
	if name.Addrtaken() || !name.OnStack() {
		return false
	}
	switch name.Class {
	case ir.PPARAMOUT:
		if s.hasdefer {
			// TODO: handle this case? Named return values must be
			// in memory so that the deferred function can see them.
			// Maybe do: if !strings.HasPrefix(n.String(), "~") { return false }
			// Or maybe not, see issue 18860.  Even unnamed return values
			// must be written back so if a defer recovers, the caller can see them.
			return false
		}
		if s.cgoUnsafeArgs {
			// Cgo effectively takes the address of all result args,
			// but the compiler can't see that.
			return false
		}
	}
	return true
	// TODO: try to make more variables SSAable?
}

// exprPtr evaluates n to a pointer and nil-checks it.
func (s *state) exprPtr(n ir.Node, bounded bool, lineno src.XPos) *ssa.Value {
	p := s.expr(n)
	if bounded || n.NonNil() {
		if s.f.Frontend().Debug_checknil() && lineno.Line() > 1 {
			s.f.Warnl(lineno, "removed nil check")
		}
		return p
	}
	p = s.nilCheck(p)
	return p
}

// nilCheck generates nil pointer checking code.
// Used only for automatically inserted nil checks,
// not for user code like 'x != nil'.
// Returns a "definitely not nil" copy of x to ensure proper ordering
// of the uses of the post-nilcheck pointer.
func (s *state) nilCheck(ptr *ssa.Value) *ssa.Value {
	if base.Debug.DisableNil != 0 || s.curfn.NilCheckDisabled() {
		return ptr
	}
	return s.newValue2(ssa.OpNilCheck, ptr.Type, ptr, s.mem())
}

// boundsCheckArrayIndex generates bounds checking code for array indexing operations.
func (s *state) boundsCheckArrayIndex(n ir.Node) {
	if n.Op() != ir.OINDEX {
		return
	}
	nn := n.(*ir.IndexExpr)
	typ := nn.X.Type()
	if typ.IsArray() {
		_ = s.expr(nn.X) // for side effects
		idx := s.expr(nn.Index)
		len := s.constInt(types.Types[types.TINT], typ.NumElem())
		s.boundsCheck(idx, len, ssa.BoundsIndex, nn.Bounded())
	}
}

// boundsCheck generates bounds checking code. Checks if 0 <= idx <[=] len, branches to exit if not.
// Starts a new block on return.
// On input, len must be converted to full int width and be nonnegative.
// Returns idx converted to full int width.
// If bounded is true then caller guarantees the index is not out of bounds
// (but boundsCheck will still extend the index to full int width).
func (s *state) boundsCheck(idx, len *ssa.Value, kind ssa.BoundsKind, bounded bool) *ssa.Value {
	idx = s.extendIndex(idx, len, kind, bounded)

	if bounded || base.Flag.B != 0 {
		// If bounded or bounds checking is flag-disabled, then no check necessary,
		// just return the extended index.
		//
		// Here, bounded == true if the compiler generated the index itself,
		// such as in the expansion of a slice initializer. These indexes are
		// compiler-generated, not Go program variables, so they cannot be
		// attacker-controlled, so we can omit Spectre masking as well.
		//
		// Note that we do not want to omit Spectre masking in code like:
		//
		//	if 0 <= i && i < len(x) {
		//		use(x[i])
		//	}
		//
		// Lucky for us, bounded==false for that code.
		// In that case (handled below), we emit a bound check (and Spectre mask)
		// and then the prove pass will remove the bounds check.
		// In theory the prove pass could potentially remove certain
		// Spectre masks, but it's very delicate and probably better
		// to be conservative and leave them all in.
		return idx
	}

	bNext := s.f.NewBlock(ssa.BlockPlain)
	bPanic := s.f.NewBlock(ssa.BlockExit)

	if !idx.Type.IsSigned() {
		switch kind {
		case ssa.BoundsIndex:
			kind = ssa.BoundsIndexU
		case ssa.BoundsSliceAlen:
			kind = ssa.BoundsSliceAlenU
		case ssa.BoundsSliceAcap:
			kind = ssa.BoundsSliceAcapU
		case ssa.BoundsSliceB:
			kind = ssa.BoundsSliceBU
		case ssa.BoundsSlice3Alen:
			kind = ssa.BoundsSlice3AlenU
		case ssa.BoundsSlice3Acap:
			kind = ssa.BoundsSlice3AcapU
		case ssa.BoundsSlice3B:
			kind = ssa.BoundsSlice3BU
		case ssa.BoundsSlice3C:
			kind = ssa.BoundsSlice3CU
		}
	}

	var cmp *ssa.Value
	if kind == ssa.BoundsIndex || kind == ssa.BoundsIndexU {
		cmp = s.newValue2(ssa.OpIsInBounds, types.Types[types.TBOOL], idx, len)
	} else {
		cmp = s.newValue2(ssa.OpIsSliceInBounds, types.Types[types.TBOOL], idx, len)
	}
	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.SetControl(cmp)
	b.Likely = ssa.BranchLikely
	b.AddEdgeTo(bNext)
	b.AddEdgeTo(bPanic)

	s.startBlock(bPanic)
	if Arch.LinkArch.Family == sys.Wasm {
		// TODO(khr): figure out how to do "register" based calling convention for bounds checks.
		// Should be similar to gcWriteBarrier, but I can't make it work.
		s.rtcall(BoundsCheckFunc[kind], false, nil, idx, len)
	} else {
		mem := s.newValue3I(ssa.OpPanicBounds, types.TypeMem, int64(kind), idx, len, s.mem())
		s.endBlock().SetControl(mem)
	}
	s.startBlock(bNext)

	// In Spectre index mode, apply an appropriate mask to avoid speculative out-of-bounds accesses.
	if base.Flag.Cfg.SpectreIndex {
		op := ssa.OpSpectreIndex
		if kind != ssa.BoundsIndex && kind != ssa.BoundsIndexU {
			op = ssa.OpSpectreSliceIndex
		}
		idx = s.newValue2(op, types.Types[types.TINT], idx, len)
	}

	return idx
}

// If cmp (a bool) is false, panic using the given function.
func (s *state) check(cmp *ssa.Value, fn *obj.LSym) {
	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.SetControl(cmp)
	b.Likely = ssa.BranchLikely
	bNext := s.f.NewBlock(ssa.BlockPlain)
	line := s.peekPos()
	pos := base.Ctxt.PosTable.Pos(line)
	fl := funcLine{f: fn, base: pos.Base(), line: pos.Line()}
	bPanic := s.panics[fl]
	if bPanic == nil {
		bPanic = s.f.NewBlock(ssa.BlockPlain)
		s.panics[fl] = bPanic
		s.startBlock(bPanic)
		// The panic call takes/returns memory to ensure that the right
		// memory state is observed if the panic happens.
		s.rtcall(fn, false, nil)
	}
	b.AddEdgeTo(bNext)
	b.AddEdgeTo(bPanic)
	s.startBlock(bNext)
}

func (s *state) intDivide(n ir.Node, a, b *ssa.Value) *ssa.Value {
	needcheck := true
	switch b.Op {
	case ssa.OpConst8, ssa.OpConst16, ssa.OpConst32, ssa.OpConst64:
		if b.AuxInt != 0 {
			needcheck = false
		}
	}
	if needcheck {
		// do a size-appropriate check for zero
		cmp := s.newValue2(s.ssaOp(ir.ONE, n.Type()), types.Types[types.TBOOL], b, s.zeroVal(n.Type()))
		s.check(cmp, ir.Syms.Panicdivide)
	}
	return s.newValue2(s.ssaOp(n.Op(), n.Type()), a.Type, a, b)
}

// rtcall issues a call to the given runtime function fn with the listed args.
// Returns a slice of results of the given result types.
// The call is added to the end of the current block.
// If returns is false, the block is marked as an exit block.
func (s *state) rtcall(fn *obj.LSym, returns bool, results []*types.Type, args ...*ssa.Value) []*ssa.Value {
	s.prevCall = nil
	// Write args to the stack
	off := base.Ctxt.Arch.FixedFrameSize
	var callArgs []*ssa.Value
	var callArgTypes []*types.Type

	for _, arg := range args {
		t := arg.Type
		off = types.RoundUp(off, t.Alignment())
		size := t.Size()
		callArgs = append(callArgs, arg)
		callArgTypes = append(callArgTypes, t)
		off += size
	}
	off = types.RoundUp(off, int64(types.RegSize))

	// Issue call
	var call *ssa.Value
	aux := ssa.StaticAuxCall(fn, s.f.ABIDefault.ABIAnalyzeTypes(callArgTypes, results))
	callArgs = append(callArgs, s.mem())
	call = s.newValue0A(ssa.OpStaticLECall, aux.LateExpansionResultType(), aux)
	call.AddArgs(callArgs...)
	s.vars[memVar] = s.newValue1I(ssa.OpSelectN, types.TypeMem, int64(len(results)), call)

	if !returns {
		// Finish block
		b := s.endBlock()
		b.Kind = ssa.BlockExit
		b.SetControl(call)
		call.AuxInt = off - base.Ctxt.Arch.FixedFrameSize
		if len(results) > 0 {
			s.Fatalf("panic call can't have results")
		}
		return nil
	}

	// Load results
	res := make([]*ssa.Value, len(results))
	for i, t := range results {
		off = types.RoundUp(off, t.Alignment())
		res[i] = s.resultOfCall(call, int64(i), t)
		off += t.Size()
	}
	off = types.RoundUp(off, int64(types.PtrSize))

	// Remember how much callee stack space we needed.
	call.AuxInt = off

	return res
}

// do *left = right for type t.
func (s *state) storeType(t *types.Type, left, right *ssa.Value, skip skipMask, leftIsStmt bool) {
	s.instrument(t, left, instrumentWrite)

	if skip == 0 && (!t.HasPointers() || ssa.IsStackAddr(left)) {
		// Known to not have write barrier. Store the whole type.
		s.vars[memVar] = s.newValue3Apos(ssa.OpStore, types.TypeMem, t, left, right, s.mem(), leftIsStmt)
		return
	}

	// store scalar fields first, so write barrier stores for
	// pointer fields can be grouped together, and scalar values
	// don't need to be live across the write barrier call.
	// TODO: if the writebarrier pass knows how to reorder stores,
	// we can do a single store here as long as skip==0.
	s.storeTypeScalars(t, left, right, skip)
	if skip&skipPtr == 0 && t.HasPointers() {
		s.storeTypePtrs(t, left, right)
	}
}

// do *left = right for all scalar (non-pointer) parts of t.
func (s *state) storeTypeScalars(t *types.Type, left, right *ssa.Value, skip skipMask) {
	switch {
	case t.IsBoolean() || t.IsInteger() || t.IsFloat() || t.IsComplex() || t.IsSIMD():
		s.store(t, left, right)
	case t.IsPtrShaped():
		if t.IsPtr() && t.Elem().NotInHeap() {
			s.store(t, left, right) // see issue 42032
		}
		// otherwise, no scalar fields.
	case t.IsString():
		if skip&skipLen != 0 {
			return
		}
		len := s.newValue1(ssa.OpStringLen, types.Types[types.TINT], right)
		lenAddr := s.newValue1I(ssa.OpOffPtr, s.f.Config.Types.IntPtr, s.config.PtrSize, left)
		s.store(types.Types[types.TINT], lenAddr, len)
	case t.IsSlice():
		if skip&skipLen == 0 {
			len := s.newValue1(ssa.OpSliceLen, types.Types[types.TINT], right)
			lenAddr := s.newValue1I(ssa.OpOffPtr, s.f.Config.Types.IntPtr, s.config.PtrSize, left)
			s.store(types.Types[types.TINT], lenAddr, len)
		}
		if skip&skipCap == 0 {
			cap := s.newValue1(ssa.OpSliceCap, types.Types[types.TINT], right)
			capAddr := s.newValue1I(ssa.OpOffPtr, s.f.Config.Types.IntPtr, 2*s.config.PtrSize, left)
			s.store(types.Types[types.TINT], capAddr, cap)
		}
	case t.IsInterface():
		// itab field doesn't need a write barrier (even though it is a pointer).
		itab := s.newValue1(ssa.OpITab, s.f.Config.Types.BytePtr, right)
		s.store(types.Types[types.TUINTPTR], left, itab)
	case isStructNotSIMD(t):
		n := t.NumFields()
		for i := 0; i < n; i++ {
			ft := t.FieldType(i)
			addr := s.newValue1I(ssa.OpOffPtr, ft.PtrTo(), t.FieldOff(i), left)
			val := s.newValue1I(ssa.OpStructSelect, ft, int64(i), right)
			s.storeTypeScalars(ft, addr, val, 0)
		}
	case t.IsArray() && t.Size() == 0:
		// nothing
	case t.IsArray() && t.NumElem() == 1:
		s.storeTypeScalars(t.Elem(), left, s.newValue1I(ssa.OpArraySelect, t.Elem(), 0, right), 0)
	default:
		s.Fatalf("bad write barrier type %v", t)
	}
}

// do *left = right for all pointer parts of t.
func (s *state) storeTypePtrs(t *types.Type, left, right *ssa.Value) {
	switch {
	case t.IsPtrShaped():
		if t.IsPtr() && t.Elem().NotInHeap() {
			break // see issue 42032
		}
		s.store(t, left, right)
	case t.IsString():
		ptr := s.newValue1(ssa.OpStringPtr, s.f.Config.Types.BytePtr, right)
		s.store(s.f.Config.Types.BytePtr, left, ptr)
	case t.IsSlice():
		elType := types.NewPtr(t.Elem())
		ptr := s.newValue1(ssa.OpSlicePtr, elType, right)
		s.store(elType, left, ptr)
	case t.IsInterface():
		// itab field is treated as a scalar.
		idata := s.newValue1(ssa.OpIData, s.f.Config.Types.BytePtr, right)
		idataAddr := s.newValue1I(ssa.OpOffPtr, s.f.Config.Types.BytePtrPtr, s.config.PtrSize, left)
		s.store(s.f.Config.Types.BytePtr, idataAddr, idata)
	case isStructNotSIMD(t):
		n := t.NumFields()
		for i := 0; i < n; i++ {
			ft := t.FieldType(i)
			if !ft.HasPointers() {
				continue
			}
			addr := s.newValue1I(ssa.OpOffPtr, ft.PtrTo(), t.FieldOff(i), left)
			val := s.newValue1I(ssa.OpStructSelect, ft, int64(i), right)
			s.storeTypePtrs(ft, addr, val)
		}
	case t.IsArray() && t.Size() == 0:
		// nothing
	case t.IsArray() && t.NumElem() == 1:
		s.storeTypePtrs(t.Elem(), left, s.newValue1I(ssa.OpArraySelect, t.Elem(), 0, right))
	default:
		s.Fatalf("bad write barrier type %v", t)
	}
}

// putArg evaluates n for the purpose of passing it as an argument to a function and returns the value for the call.
func (s *state) putArg(n ir.Node, t *types.Type) *ssa.Value {
	var a *ssa.Value
	if !ssa.CanSSA(t) {
		a = s.newValue2(ssa.OpDereference, t, s.addr(n), s.mem())
	} else {
		a = s.expr(n)
	}
	return a
}

// slice computes the slice v[i:j:k] and returns ptr, len, and cap of result.
// i,j,k may be nil, in which case they are set to their default value.
// v may be a slice, string or pointer to an array.
func (s *state) slice(v, i, j, k *ssa.Value, bounded bool) (p, l, c *ssa.Value) {
	t := v.Type
	var ptr, len, cap *ssa.Value
	switch {
	case t.IsSlice():
		ptr = s.newValue1(ssa.OpSlicePtr, types.NewPtr(t.Elem()), v)
		len = s.newValue1(ssa.OpSliceLen, types.Types[types.TINT], v)
		cap = s.newValue1(ssa.OpSliceCap, types.Types[types.TINT], v)
	case t.IsString():
		ptr = s.newValue1(ssa.OpStringPtr, types.NewPtr(types.Types[types.TUINT8]), v)
		len = s.newValue1(ssa.OpStringLen, types.Types[types.TINT], v)
		cap = len
	case t.IsPtr():
		if !t.Elem().IsArray() {
			s.Fatalf("bad ptr to array in slice %v\n", t)
		}
		nv := s.nilCheck(v)
		ptr = s.newValue1(ssa.OpCopy, types.NewPtr(t.Elem().Elem()), nv)
		len = s.constInt(types.Types[types.TINT], t.Elem().NumElem())
		cap = len
	default:
		s.Fatalf("bad type in slice %v\n", t)
	}

	// Set default values
	if i == nil {
		i = s.constInt(types.Types[types.TINT], 0)
	}
	if j == nil {
		j = len
	}
	three := true
	if k == nil {
		three = false
		k = cap
	}

	// Panic if slice indices are not in bounds.
	// Make sure we check these in reverse order so that we're always
	// comparing against a value known to be nonnegative. See issue 28797.
	if three {
		if k != cap {
			kind := ssa.BoundsSlice3Alen
			if t.IsSlice() {
				kind = ssa.BoundsSlice3Acap
			}
			k = s.boundsCheck(k, cap, kind, bounded)
		}
		if j != k {
			j = s.boundsCheck(j, k, ssa.BoundsSlice3B, bounded)
		}
		i = s.boundsCheck(i, j, ssa.BoundsSlice3C, bounded)
	} else {
		if j != k {
			kind := ssa.BoundsSliceAlen
			if t.IsSlice() {
				kind = ssa.BoundsSliceAcap
			}
			j = s.boundsCheck(j, k, kind, bounded)
		}
		i = s.boundsCheck(i, j, ssa.BoundsSliceB, bounded)
	}

	// Word-sized integer operations.
	subOp := s.ssaOp(ir.OSUB, types.Types[types.TINT])
	mulOp := s.ssaOp(ir.OMUL, types.Types[types.TINT])
	andOp := s.ssaOp(ir.OAND, types.Types[types.TINT])

	// Calculate the length (rlen) and capacity (rcap) of the new slice.
	// For strings the capacity of the result is unimportant. However,
	// we use rcap to test if we've generated a zero-length slice.
	// Use length of strings for that.
	rlen := s.newValue2(subOp, types.Types[types.TINT], j, i)
	rcap := rlen
	if j != k && !t.IsString() {
		rcap = s.newValue2(subOp, types.Types[types.TINT], k, i)
	}

	if (i.Op == ssa.OpConst64 || i.Op == ssa.OpConst32) && i.AuxInt == 0 {
		// No pointer arithmetic necessary.
		return ptr, rlen, rcap
	}

	// Calculate the base pointer (rptr) for the new slice.
	//
	// Generate the following code assuming that indexes are in bounds.
	// The masking is to make sure that we don't generate a slice
	// that points to the next object in memory. We cannot just set
	// the pointer to nil because then we would create a nil slice or
	// string.
	//
	//     rcap = k - i
	//     rlen = j - i
	//     rptr = ptr + (mask(rcap) & (i * stride))
	//
	// Where mask(x) is 0 if x==0 and -1 if x>0 and stride is the width
	// of the element type.
	stride := s.constInt(types.Types[types.TINT], ptr.Type.Elem().Size())

	// The delta is the number of bytes to offset ptr by.
	delta := s.newValue2(mulOp, types.Types[types.TINT], i, stride)

	// If we're slicing to the point where the capacity is zero,
	// zero out the delta.
	mask := s.newValue1(ssa.OpSlicemask, types.Types[types.TINT], rcap)
	delta = s.newValue2(andOp, types.Types[types.TINT], delta, mask)

	// Compute rptr = ptr + delta.
	rptr := s.newValue2(ssa.OpAddPtr, ptr.Type, ptr, delta)

	return rptr, rlen, rcap
}

type u642fcvtTab struct {
	leq, cvt2F, and, rsh, or, add ssa.Op
	one                           func(*state, *types.Type, int64) *ssa.Value
}

var u64_f64 = u642fcvtTab{
	leq:   ssa.OpLeq64,
	cvt2F: ssa.OpCvt64to64F,
	and:   ssa.OpAnd64,
	rsh:   ssa.OpRsh64Ux64,
	or:    ssa.OpOr64,
	add:   ssa.OpAdd64F,
	one:   (*state).constInt64,
}

var u64_f32 = u642fcvtTab{
	leq:   ssa.OpLeq64,
	cvt2F: ssa.OpCvt64to32F,
	and:   ssa.OpAnd64,
	rsh:   ssa.OpRsh64Ux64,
	or:    ssa.OpOr64,
	add:   ssa.OpAdd32F,
	one:   (*state).constInt64,
}

func (s *state) uint64Tofloat64(n ir.Node, x *ssa.Value, ft, tt *types.Type) *ssa.Value {
	return s.uint64Tofloat(&u64_f64, n, x, ft, tt)
}

func (s *state) uint64Tofloat32(n ir.Node, x *ssa.Value, ft, tt *types.Type) *ssa.Value {
	return s.uint64Tofloat(&u64_f32, n, x, ft, tt)
}

func (s *state) uint64Tofloat(cvttab *u642fcvtTab, n ir.Node, x *ssa.Value, ft, tt *types.Type) *ssa.Value {
	// if x >= 0 {
	//    result = (floatY) x
	// } else {
	// 	  y = uintX(x) ; y = x & 1
	// 	  z = uintX(x) ; z = z >> 1
	// 	  z = z | y
	// 	  result = floatY(z)
	// 	  result = result + result
	// }
	//
	// Code borrowed from old code generator.
	// What's going on: large 64-bit "unsigned" looks like
	// negative number to hardware's integer-to-float
	// conversion. However, because the mantissa is only
	// 63 bits, we don't need the LSB, so instead we do an
	// unsigned right shift (divide by two), convert, and
	// double. However, before we do that, we need to be
	// sure that we do not lose a "1" if that made the
	// difference in the resulting rounding. Therefore, we
	// preserve it, and OR (not ADD) it back in. The case
	// that matters is when the eleven discarded bits are
	// equal to 10000000001; that rounds up, and the 1 cannot
	// be lost else it would round down if the LSB of the
	// candidate mantissa is 0.

	cmp := s.newValue2(cvttab.leq, types.Types[types.TBOOL], s.zeroVal(ft), x)

	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.SetControl(cmp)
	b.Likely = ssa.BranchLikely

	bThen := s.f.NewBlock(ssa.BlockPlain)
	bElse := s.f.NewBlock(ssa.BlockPlain)
	bAfter := s.f.NewBlock(ssa.BlockPlain)

	b.AddEdgeTo(bThen)
	s.startBlock(bThen)
	a0 := s.newValue1(cvttab.cvt2F, tt, x)
	s.vars[n] = a0
	s.endBlock()
	bThen.AddEdgeTo(bAfter)

	b.AddEdgeTo(bElse)
	s.startBlock(bElse)
	one := cvttab.one(s, ft, 1)
	y := s.newValue2(cvttab.and, ft, x, one)
	z := s.newValue2(cvttab.rsh, ft, x, one)
	z = s.newValue2(cvttab.or, ft, z, y)
	a := s.newValue1(cvttab.cvt2F, tt, z)
	a1 := s.newValue2(cvttab.add, tt, a, a)
	s.vars[n] = a1
	s.endBlock()
	bElse.AddEdgeTo(bAfter)

	s.startBlock(bAfter)
	return s.variable(n, n.Type())
}

type u322fcvtTab struct {
	cvtI2F, cvtF2F ssa.Op
}

var u32_f64 = u322fcvtTab{
	cvtI2F: ssa.OpCvt32to64F,
	cvtF2F: ssa.OpCopy,
}

var u32_f32 = u322fcvtTab{
	cvtI2F: ssa.OpCvt32to32F,
	cvtF2F: ssa.OpCvt64Fto32F,
}

func (s *state) uint32Tofloat64(n ir.Node, x *ssa.Value, ft, tt *types.Type) *ssa.Value {
	return s.uint32Tofloat(&u32_f64, n, x, ft, tt)
}

func (s *state) uint32Tofloat32(n ir.Node, x *ssa.Value, ft, tt *types.Type) *ssa.Value {
	return s.uint32Tofloat(&u32_f32, n, x, ft, tt)
}

func (s *state) uint32Tofloat(cvttab *u322fcvtTab, n ir.Node, x *ssa.Value, ft, tt *types.Type) *ssa.Value {
	// if x >= 0 {
	// 	result = floatY(x)
	// } else {
	// 	result = floatY(float64(x) + (1<<32))
	// }
	cmp := s.newValue2(ssa.OpLeq32, types.Types[types.TBOOL], s.zeroVal(ft), x)
	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.SetControl(cmp)
	b.Likely = ssa.BranchLikely

	bThen := s.f.NewBlock(ssa.BlockPlain)
	bElse := s.f.NewBlock(ssa.BlockPlain)
	bAfter := s.f.NewBlock(ssa.BlockPlain)

	b.AddEdgeTo(bThen)
	s.startBlock(bThen)
	a0 := s.newValue1(cvttab.cvtI2F, tt, x)
	s.vars[n] = a0
	s.endBlock()
	bThen.AddEdgeTo(bAfter)

	b.AddEdgeTo(bElse)
	s.startBlock(bElse)
	a1 := s.newValue1(ssa.OpCvt32to64F, types.Types[types.TFLOAT64], x)
	twoToThe32 := s.constFloat64(types.Types[types.TFLOAT64], float64(1<<32))
	a2 := s.newValue2(ssa.OpAdd64F, types.Types[types.TFLOAT64], a1, twoToThe32)
	a3 := s.newValue1(cvttab.cvtF2F, tt, a2)

	s.vars[n] = a3
	s.endBlock()
	bElse.AddEdgeTo(bAfter)

	s.startBlock(bAfter)
	return s.variable(n, n.Type())
}

// referenceTypeBuiltin generates code for the len/cap builtins for maps and channels.
func (s *state) referenceTypeBuiltin(n *ir.UnaryExpr, x *ssa.Value) *ssa.Value {
	if !n.X.Type().IsMap() && !n.X.Type().IsChan() {
		s.Fatalf("node must be a map or a channel")
	}
	if n.X.Type().IsChan() && n.Op() == ir.OLEN {
		s.Fatalf("cannot inline len(chan)") // must use runtime.chanlen now
	}
	if n.X.Type().IsChan() && n.Op() == ir.OCAP {
		s.Fatalf("cannot inline cap(chan)") // must use runtime.chancap now
	}
	if n.X.Type().IsMap() && n.Op() == ir.OCAP {
		s.Fatalf("cannot inline cap(map)") // cap(map) does not exist
	}
	// if n == nil {
	//   return 0
	// } else {
	//   // len, the actual loadType depends
	//   return int(*((*loadType)n))
	//   // cap (chan only, not used for now)
	//   return *(((*int)n)+1)
	// }
	lenType := n.Type()
	nilValue := s.constNil(types.Types[types.TUINTPTR])
	cmp := s.newValue2(ssa.OpEqPtr, types.Types[types.TBOOL], x, nilValue)
	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.SetControl(cmp)
	b.Likely = ssa.BranchUnlikely

	bThen := s.f.NewBlock(ssa.BlockPlain)
	bElse := s.f.NewBlock(ssa.BlockPlain)
	bAfter := s.f.NewBlock(ssa.BlockPlain)

	// length/capacity of a nil map/chan is zero
	b.AddEdgeTo(bThen)
	s.startBlock(bThen)
	s.vars[n] = s.zeroVal(lenType)
	s.endBlock()
	bThen.AddEdgeTo(bAfter)

	b.AddEdgeTo(bElse)
	s.startBlock(bElse)
	switch n.Op() {
	case ir.OLEN:
		if n.X.Type().IsMap() {
			// length is stored in the first word, but needs conversion to int.
			loadType := reflectdata.MapType().Field(0).Type // uint64
			load := s.load(loadType, x)
			s.vars[n] = s.conv(nil, load, loadType, lenType) // integer conversion doesn't need Node
		} else {
			// length is stored in the first word for chan, no conversion needed.
			s.vars[n] = s.load(lenType, x)
		}
	case ir.OCAP:
		// capacity is stored in the second word for chan
		sw := s.newValue1I(ssa.OpOffPtr, lenType.PtrTo(), lenType.Size(), x)
		s.vars[n] = s.load(lenType, sw)
	default:
		s.Fatalf("op must be OLEN or OCAP")
	}
	s.endBlock()
	bElse.AddEdgeTo(bAfter)

	s.startBlock(bAfter)
	return s.variable(n, lenType)
}

type f2uCvtTab struct {
	ltf, cvt2U, subf, or ssa.Op
	floatValue           func(*state, *types.Type, float64) *ssa.Value
	intValue             func(*state, *types.Type, int64) *ssa.Value
	cutoff               uint64
}

var f32_u64 = f2uCvtTab{
	ltf:        ssa.OpLess32F,
	cvt2U:      ssa.OpCvt32Fto64,
	subf:       ssa.OpSub32F,
	or:         ssa.OpOr64,
	floatValue: (*state).constFloat32,
	intValue:   (*state).constInt64,
	cutoff:     1 << 63,
}

var f64_u64 = f2uCvtTab{
	ltf:        ssa.OpLess64F,
	cvt2U:      ssa.OpCvt64Fto64,
	subf:       ssa.OpSub64F,
	or:         ssa.OpOr64,
	floatValue: (*state).constFloat64,
	intValue:   (*state).constInt64,
	cutoff:     1 << 63,
}

var f32_u32 = f2uCvtTab{
	ltf:        ssa.OpLess32F,
	cvt2U:      ssa.OpCvt32Fto32,
	subf:       ssa.OpSub32F,
	or:         ssa.OpOr32,
	floatValue: (*state).constFloat32,
	intValue:   func(s *state, t *types.Type, v int64) *ssa.Value { return s.constInt32(t, int32(v)) },
	cutoff:     1 << 31,
}

var f64_u32 = f2uCvtTab{
	ltf:        ssa.OpLess64F,
	cvt2U:      ssa.OpCvt64Fto32,
	subf:       ssa.OpSub64F,
	or:         ssa.OpOr32,
	floatValue: (*state).constFloat64,
	intValue:   func(s *state, t *types.Type, v int64) *ssa.Value { return s.constInt32(t, int32(v)) },
	cutoff:     1 << 31,
}

func (s *state) float32ToUint64(n ir.Node, x *ssa.Value, ft, tt *types.Type) *ssa.Value {
	return s.floatToUint(&f32_u64, n, x, ft, tt)
}
func (s *state) float64ToUint64(n ir.Node, x *ssa.Value, ft, tt *types.Type) *ssa.Value {
	return s.floatToUint(&f64_u64, n, x, ft, tt)
}

func (s *state) float32ToUint32(n ir.Node, x *ssa.Value, ft, tt *types.Type) *ssa.Value {
	return s.floatToUint(&f32_u32, n, x, ft, tt)
}

func (s *state) float64ToUint32(n ir.Node, x *ssa.Value, ft, tt *types.Type) *ssa.Value {
	return s.floatToUint(&f64_u32, n, x, ft, tt)
}

func (s *state) floatToUint(cvttab *f2uCvtTab, n ir.Node, x *ssa.Value, ft, tt *types.Type) *ssa.Value {
	// cutoff:=1<<(intY_Size-1)
	// if x < floatX(cutoff) {
	// 	result = uintY(x) // bThen
	//  // gated by ConvertHash, clamp negative inputs to zero
	// 	if x < 0 { // unlikely
	// 		result = 0 // bZero
	// 	}
	// } else {
	// 	y = x - floatX(cutoff) // bElse
	// 	z = uintY(y)
	// 	result = z | -(cutoff)
	// }

	cutoff := cvttab.floatValue(s, ft, float64(cvttab.cutoff))
	cmp := s.newValueOrSfCall2(cvttab.ltf, types.Types[types.TBOOL], x, cutoff)
	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.SetControl(cmp)
	b.Likely = ssa.BranchLikely

	var bThen, bZero *ssa.Block
	// use salted hash to distinguish unsigned convert at a Pos from signed convert at a Pos
	newConversion := base.ConvertHash.MatchPosWithInfo(n.Pos(), "U", nil)
	if newConversion {
		bZero = s.f.NewBlock(ssa.BlockPlain)
		bThen = s.f.NewBlock(ssa.BlockIf)
	} else {
		bThen = s.f.NewBlock(ssa.BlockPlain)
	}

	bElse := s.f.NewBlock(ssa.BlockPlain)
	bAfter := s.f.NewBlock(ssa.BlockPlain)

	b.AddEdgeTo(bThen)
	s.startBlock(bThen)
	a0 := s.newValueOrSfCall1(cvttab.cvt2U, tt, x)
	s.vars[n] = a0

	if newConversion {
		cmpz := s.newValueOrSfCall2(cvttab.ltf, types.Types[types.TBOOL], x, cvttab.floatValue(s, ft, 0.0))
		s.endBlock()
		bThen.SetControl(cmpz)
		bThen.AddEdgeTo(bZero)
		bThen.Likely = ssa.BranchUnlikely
		bThen.AddEdgeTo(bAfter)

		s.startBlock(bZero)
		s.vars[n] = cvttab.intValue(s, tt, 0)
		s.endBlock()
		bZero.AddEdgeTo(bAfter)
	} else {
		s.endBlock()
		bThen.AddEdgeTo(bAfter)
	}

	b.AddEdgeTo(bElse)
	s.startBlock(bElse)
	y := s.newValueOrSfCall2(cvttab.subf, ft, x, cutoff)
	y = s.newValueOrSfCall1(cvttab.cvt2U, tt, y)
	z := cvttab.intValue(s, tt, int64(-cvttab.cutoff))
	a1 := s.newValue2(cvttab.or, tt, y, z)
	s.vars[n] = a1
	s.endBlock()
	bElse.AddEdgeTo(bAfter)

	s.startBlock(bAfter)
	return s.variable(n, n.Type())
}

// dottype generates SSA for a type assertion node.
// commaok indicates whether to panic or return a bool.
// If commaok is false, resok will be nil.
func (s *state) dottype(n *ir.TypeAssertExpr, commaok bool) (res, resok *ssa.Value) {
	iface := s.expr(n.X)              // input interface
	target := s.reflectType(n.Type()) // target type
	var targetItab *ssa.Value
	if n.ITab != nil {
		targetItab = s.expr(n.ITab)
	}

	if n.UseNilPanic {
		if commaok {
			base.Fatalf("unexpected *ir.TypeAssertExpr with UseNilPanic == true && commaok == true")
		}
		if n.Type().IsInterface() {
			// Currently we do not expect the compiler to emit type assertions with UseNilPanic, that asserts to an interface type.
			// If needed, this can be relaxed in the future, but for now we can't assert that.
			base.Fatalf("unexpected *ir.TypeAssertExpr with UseNilPanic == true && Type().IsInterface() == true")
		}
		typs := s.f.Config.Types
		iface = s.newValue2(
			ssa.OpIMake,
			iface.Type,
			s.nilCheck(s.newValue1(ssa.OpITab, typs.BytePtr, iface)),
			s.newValue1(ssa.OpIData, typs.BytePtr, iface),
		)
	}

	return s.dottype1(n.Pos(), n.X.Type(), n.Type(), iface, nil, target, targetItab, commaok, n.Descriptor)
}

func (s *state) dynamicDottype(n *ir.DynamicTypeAssertExpr, commaok bool) (res, resok *ssa.Value) {
	iface := s.expr(n.X)
	var source, target, targetItab *ssa.Value
	if n.SrcRType != nil {
		source = s.expr(n.SrcRType)
	}
	if !n.X.Type().IsEmptyInterface() && !n.Type().IsInterface() {
		byteptr := s.f.Config.Types.BytePtr
		targetItab = s.expr(n.ITab)
		// TODO(mdempsky): Investigate whether compiling n.RType could be
		// better than loading itab.typ.
		target = s.load(byteptr, s.newValue1I(ssa.OpOffPtr, byteptr, rttype.ITab.OffsetOf("Type"), targetItab))
	} else {
		target = s.expr(n.RType)
	}
	return s.dottype1(n.Pos(), n.X.Type(), n.Type(), iface, source, target, targetItab, commaok, nil)
}

// dottype1 implements a x.(T) operation. iface is the argument (x), dst is the type we're asserting to (T)
// and src is the type we're asserting from.
// source is the *runtime._type of src
// target is the *runtime._type of dst.
// If src is a nonempty interface and dst is not an interface, targetItab is an itab representing (dst, src). Otherwise it is nil.
// commaok is true if the caller wants a boolean success value. Otherwise, the generated code panics if the conversion fails.
// descriptor is a compiler-allocated internal/abi.TypeAssert whose address is passed to runtime.typeAssert when
// the target type is a compile-time-known non-empty interface. It may be nil.
func (s *state) dottype1(pos src.XPos, src, dst *types.Type, iface, source, target, targetItab *ssa.Value, commaok bool, descriptor *obj.LSym) (res, resok *ssa.Value) {
	typs := s.f.Config.Types
	byteptr := typs.BytePtr
	if dst.IsInterface() {
		if dst.IsEmptyInterface() {
			// Converting to an empty interface.
			// Input could be an empty or nonempty interface.
			if base.Debug.TypeAssert > 0 {
				base.WarnfAt(pos, "type assertion inlined")
			}

			// Get itab/type field from input.
			itab := s.newValue1(ssa.OpITab, byteptr, iface)
			// Conversion succeeds iff that field is not nil.
			cond := s.newValue2(ssa.OpNeqPtr, types.Types[types.TBOOL], itab, s.constNil(byteptr))

			if src.IsEmptyInterface() && commaok {
				// Converting empty interface to empty interface with ,ok is just a nil check.
				return iface, cond
			}

			// Branch on nilness.
			b := s.endBlock()
			b.Kind = ssa.BlockIf
			b.SetControl(cond)
			b.Likely = ssa.BranchLikely
			bOk := s.f.NewBlock(ssa.BlockPlain)
			bFail := s.f.NewBlock(ssa.BlockPlain)
			b.AddEdgeTo(bOk)
			b.AddEdgeTo(bFail)

			if !commaok {
				// On failure, panic by calling panicnildottype.
				s.startBlock(bFail)
				s.rtcall(ir.Syms.Panicnildottype, false, nil, target)

				// On success, return (perhaps modified) input interface.
				s.startBlock(bOk)
				if src.IsEmptyInterface() {
					res = iface // Use input interface unchanged.
					return
				}
				// Load type out of itab, build interface with existing idata.
				off := s.newValue1I(ssa.OpOffPtr, byteptr, rttype.ITab.OffsetOf("Type"), itab)
				typ := s.load(byteptr, off)
				idata := s.newValue1(ssa.OpIData, byteptr, iface)
				res = s.newValue2(ssa.OpIMake, dst, typ, idata)
				return
			}

			s.startBlock(bOk)
			// nonempty -> empty
			// Need to load type from itab
			off := s.newValue1I(ssa.OpOffPtr, byteptr, rttype.ITab.OffsetOf("Type"), itab)
			s.vars[typVar] = s.load(byteptr, off)
			s.endBlock()

			// itab is nil, might as well use that as the nil result.
			s.startBlock(bFail)
			s.vars[typVar] = itab
			s.endBlock()

			// Merge point.
			bEnd := s.f.NewBlock(ssa.BlockPlain)
			bOk.AddEdgeTo(bEnd)
			bFail.AddEdgeTo(bEnd)
			s.startBlock(bEnd)
			idata := s.newValue1(ssa.OpIData, byteptr, iface)
			res = s.newValue2(ssa.OpIMake, dst, s.variable(typVar, byteptr), idata)
			resok = cond
			delete(s.vars, typVar) // no practical effect, just to indicate typVar is no longer live.
			return
		}
		// converting to a nonempty interface needs a runtime call.
		if base.Debug.TypeAssert > 0 {
			base.WarnfAt(pos, "type assertion not inlined")
		}

		itab := s.newValue1(ssa.OpITab, byteptr, iface)
		data := s.newValue1(ssa.OpIData, types.Types[types.TUNSAFEPTR], iface)

		// First, check for nil.
		bNil := s.f.NewBlock(ssa.BlockPlain)
		bNonNil := s.f.NewBlock(ssa.BlockPlain)
		bMerge := s.f.NewBlock(ssa.BlockPlain)
		cond := s.newValue2(ssa.OpNeqPtr, types.Types[types.TBOOL], itab, s.constNil(byteptr))
		b := s.endBlock()
		b.Kind = ssa.BlockIf
		b.SetControl(cond)
		b.Likely = ssa.BranchLikely
		b.AddEdgeTo(bNonNil)
		b.AddEdgeTo(bNil)

		s.startBlock(bNil)
		if commaok {
			s.vars[typVar] = itab // which will be nil
			b := s.endBlock()
			b.AddEdgeTo(bMerge)
		} else {
			// Panic if input is nil.
			s.rtcall(ir.Syms.Panicnildottype, false, nil, target)
		}

		// Get typ, possibly by loading out of itab.
		s.startBlock(bNonNil)
		typ := itab
		if !src.IsEmptyInterface() {
			typ = s.load(byteptr, s.newValue1I(ssa.OpOffPtr, byteptr, rttype.ITab.OffsetOf("Type"), itab))
		}

		// Check the cache first.
		var d *ssa.Value
		if descriptor != nil {
			d = s.newValue1A(ssa.OpAddr, byteptr, descriptor, s.sb)
			if base.Flag.N == 0 && rtabi.UseInterfaceSwitchCache(Arch.LinkArch.Family) {
				// Note: we can only use the cache if we have the right atomic load instruction.
				// Double-check that here.
				if intrinsics.lookup(Arch.LinkArch.Arch, "internal/runtime/atomic", "Loadp") == nil {
					s.Fatalf("atomic load not available")
				}
				// Pick right size ops.
				var mul, and, add, zext ssa.Op
				if s.config.PtrSize == 4 {
					mul = ssa.OpMul32
					and = ssa.OpAnd32
					add = ssa.OpAdd32
					zext = ssa.OpCopy
				} else {
					mul = ssa.OpMul64
					and = ssa.OpAnd64
					add = ssa.OpAdd64
					zext = ssa.OpZeroExt32to64
				}

				loopHead := s.f.NewBlock(ssa.BlockPlain)
				loopBody := s.f.NewBlock(ssa.BlockPlain)
				cacheHit := s.f.NewBlock(ssa.BlockPlain)
				cacheMiss := s.f.NewBlock(ssa.BlockPlain)

				// Load cache pointer out of descriptor, with an atomic load so
				// we ensure that we see a fully written cache.
				atomicLoad := s.newValue2(ssa.OpAtomicLoadPtr, types.NewTuple(typs.BytePtr, types.TypeMem), d, s.mem())
				cache := s.newValue1(ssa.OpSelect0, typs.BytePtr, atomicLoad)
				s.vars[memVar] = s.newValue1(ssa.OpSelect1, types.TypeMem, atomicLoad)

				// Load hash from type or itab.
				var hash *ssa.Value
				if src.IsEmptyInterface() {
					hash = s.newValue2(ssa.OpLoad, typs.UInt32, s.newValue1I(ssa.OpOffPtr, typs.UInt32Ptr, rttype.Type.OffsetOf("Hash"), typ), s.mem())
				} else {
					hash = s.newValue2(ssa.OpLoad, typs.UInt32, s.newValue1I(ssa.OpOffPtr, typs.UInt32Ptr, rttype.ITab.OffsetOf("Hash"), itab), s.mem())
				}
				hash = s.newValue1(zext, typs.Uintptr, hash)
				s.vars[hashVar] = hash
				// Load mask from cache.
				mask := s.newValue2(ssa.OpLoad, typs.Uintptr, cache, s.mem())
				// Jump to loop head.
				b := s.endBlock()
				b.AddEdgeTo(loopHead)

				// At loop head, get pointer to the cache entry.
				//   e := &cache.Entries[hash&mask]
				s.startBlock(loopHead)
				idx := s.newValue2(and, typs.Uintptr, s.variable(hashVar, typs.Uintptr), mask)
				idx = s.newValue2(mul, typs.Uintptr, idx, s.uintptrConstant(uint64(2*s.config.PtrSize)))
				idx = s.newValue2(add, typs.Uintptr, idx, s.uintptrConstant(uint64(s.config.PtrSize)))
				e := s.newValue2(ssa.OpAddPtr, typs.UintptrPtr, cache, idx)
				//   hash++
				s.vars[hashVar] = s.newValue2(add, typs.Uintptr, s.variable(hashVar, typs.Uintptr), s.uintptrConstant(1))

				// Look for a cache hit.
				//   if e.Typ == typ { goto hit }
				eTyp := s.newValue2(ssa.OpLoad, typs.Uintptr, e, s.mem())
				cmp1 := s.newValue2(ssa.OpEqPtr, typs.Bool, typ, eTyp)
				b = s.endBlock()
				b.Kind = ssa.BlockIf
				b.SetControl(cmp1)
				b.AddEdgeTo(cacheHit)
				b.AddEdgeTo(loopBody)

				// Look for an empty entry, the tombstone for this hash table.
				//   if e.Typ == nil { goto miss }
				s.startBlock(loopBody)
				cmp2 := s.newValue2(ssa.OpEqPtr, typs.Bool, eTyp, s.constNil(typs.BytePtr))
				b = s.endBlock()
				b.Kind = ssa.BlockIf
				b.SetControl(cmp2)
				b.AddEdgeTo(cacheMiss)
				b.AddEdgeTo(loopHead)

				// On a hit, load the data fields of the cache entry.
				//   Itab = e.Itab
				s.startBlock(cacheHit)
				eItab := s.newValue2(ssa.OpLoad, typs.BytePtr, s.newValue1I(ssa.OpOffPtr, typs.BytePtrPtr, s.config.PtrSize, e), s.mem())
				s.vars[typVar] = eItab
				b = s.endBlock()
				b.AddEdgeTo(bMerge)

				// On a miss, call into the runtime to get the answer.
				s.startBlock(cacheMiss)
			}
		}

		// Call into runtime to get itab for result.
		if descriptor != nil {
			itab = s.rtcall(ir.Syms.TypeAssert, true, []*types.Type{byteptr}, d, typ)[0]
		} else {
			var fn *obj.LSym
			if commaok {
				fn = ir.Syms.AssertE2I2
			} else {
				fn = ir.Syms.AssertE2I
			}
			itab = s.rtcall(fn, true, []*types.Type{byteptr}, target, typ)[0]
		}
		s.vars[typVar] = itab
		b = s.endBlock()
		b.AddEdgeTo(bMerge)

		// Build resulting interface.
		s.startBlock(bMerge)
		itab = s.variable(typVar, byteptr)
		var ok *ssa.Value
		if commaok {
			ok = s.newValue2(ssa.OpNeqPtr, types.Types[types.TBOOL], itab, s.constNil(byteptr))
		}
		return s.newValue2(ssa.OpIMake, dst, itab, data), ok
	}

	if base.Debug.TypeAssert > 0 {
		base.WarnfAt(pos, "type assertion inlined")
	}

	// Converting to a concrete type.
	direct := types.IsDirectIface(dst)
	itab := s.newValue1(ssa.OpITab, byteptr, iface) // type word of interface
	if base.Debug.TypeAssert > 0 {
		base.WarnfAt(pos, "type assertion inlined")
	}
	var wantedFirstWord *ssa.Value
	if src.IsEmptyInterface() {
		// Looking for pointer to target type.
		wantedFirstWord = target
	} else {
		// Looking for pointer to itab for target type and source interface.
		wantedFirstWord = targetItab
	}

	var tmp ir.Node     // temporary for use with large types
	var addr *ssa.Value // address of tmp
	if commaok && !ssa.CanSSA(dst) {
		// unSSAable type, use temporary.
		// TODO: get rid of some of these temporaries.
		tmp, addr = s.temp(pos, dst)
	}

	cond := s.newValue2(ssa.OpEqPtr, types.Types[types.TBOOL], itab, wantedFirstWord)
	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.SetControl(cond)
	b.Likely = ssa.BranchLikely

	bOk := s.f.NewBlock(ssa.BlockPlain)
	bFail := s.f.NewBlock(ssa.BlockPlain)
	b.AddEdgeTo(bOk)
	b.AddEdgeTo(bFail)

	if !commaok {
		// on failure, panic by calling panicdottype
		s.startBlock(bFail)
		taddr := source
		if taddr == nil {
			taddr = s.reflectType(src)
		}
		if src.IsEmptyInterface() {
			s.rtcall(ir.Syms.PanicdottypeE, false, nil, itab, target, taddr)
		} else {
			s.rtcall(ir.Syms.PanicdottypeI, false, nil, itab, target, taddr)
		}

		// on success, return data from interface
		s.startBlock(bOk)
		if direct {
			return s.newValue1(ssa.OpIData, dst, iface), nil
		}
		p := s.newValue1(ssa.OpIData, types.NewPtr(dst), iface)
		return s.load(dst, p), nil
	}

	// commaok is the more complicated case because we have
	// a control flow merge point.
	bEnd := s.f.NewBlock(ssa.BlockPlain)
	// Note that we need a new valVar each time (unlike okVar where we can
	// reuse the variable) because it might have a different type every time.
	valVar := ssaMarker("val")

	// type assertion succeeded
	s.startBlock(bOk)
	if tmp == nil {
		if direct {
			s.vars[valVar] = s.newValue1(ssa.OpIData, dst, iface)
		} else {
			p := s.newValue1(ssa.OpIData, types.NewPtr(dst), iface)
			s.vars[valVar] = s.load(dst, p)
		}
	} else {
		p := s.newValue1(ssa.OpIData, types.NewPtr(dst), iface)
		s.move(dst, addr, p)
	}
	s.vars[okVar] = s.constBool(true)
	s.endBlock()
	bOk.AddEdgeTo(bEnd)

	// type assertion failed
	s.startBlock(bFail)
	if tmp == nil {
		s.vars[valVar] = s.zeroVal(dst)
	} else {
		s.zero(dst, addr)
	}
	s.vars[okVar] = s.constBool(false)
	s.endBlock()
	bFail.AddEdgeTo(bEnd)

	// merge point
	s.startBlock(bEnd)
	if tmp == nil {
		res = s.variable(valVar, dst)
		delete(s.vars, valVar) // no practical effect, just to indicate typVar is no longer live.
	} else {
		res = s.load(dst, addr)
	}
	resok = s.variable(okVar, types.Types[types.TBOOL])
	delete(s.vars, okVar) // ditto
	return res, resok
}

// temp allocates a temp of type t at position pos
func (s *state) temp(pos src.XPos, t *types.Type) (*ir.Name, *ssa.Value) {
	tmp := typecheck.TempAt(pos, s.curfn, t)
	if t.HasPointers() || (ssa.IsMergeCandidate(tmp) && t != deferstruct()) {
		s.vars[memVar] = s.newValue1A(ssa.OpVarDef, types.TypeMem, tmp, s.mem())
	}
	addr := s.addr(tmp)
	return tmp, addr
}

// variable returns the value of a variable at the current location.
func (s *state) variable(n ir.Node, t *types.Type) *ssa.Value {
	v := s.vars[n]
	if v != nil {
		return v
	}
	v = s.fwdVars[n]
	if v != nil {
		return v
	}

	if s.curBlock == s.f.Entry {
		// No variable should be live at entry.
		s.f.Fatalf("value %v (%v) incorrectly live at entry", n, v)
	}
	// Make a FwdRef, which records a value that's live on block input.
	// We'll find the matching definition as part of insertPhis.
	v = s.newValue0A(ssa.OpFwdRef, t, fwdRefAux{N: n})
	s.fwdVars[n] = v
	if n.Op() == ir.ONAME {
		s.addNamedValue(n.(*ir.Name), v)
	}
	return v
}

func (s *state) mem() *ssa.Value {
	return s.variable(memVar, types.TypeMem)
}

func (s *state) addNamedValue(n *ir.Name, v *ssa.Value) {
	if n.Class == ir.Pxxx {
		// Don't track our marker nodes (memVar etc.).
		return
	}
	if ir.IsAutoTmp(n) {
		// Don't track temporary variables.
		return
	}
	if n.Class == ir.PPARAMOUT {
		// Don't track named output values.  This prevents return values
		// from being assigned too early. See #14591 and #14762. TODO: allow this.
		return
	}
	loc := ssa.LocalSlot{N: n, Type: n.Type(), Off: 0}
	values, ok := s.f.NamedValues[loc]
	if !ok {
		s.f.Names = append(s.f.Names, &loc)
		s.f.CanonicalLocalSlots[loc] = &loc
	}
	s.f.NamedValues[loc] = append(values, v)
}

// Branch is an unresolved branch.
type Branch struct {
	P *obj.Prog  // branch instruction
	B *ssa.Block // target
}

// State contains state needed during Prog generation.
type State struct {
	ABI obj.ABI

	pp *objw.Progs

	// Branches remembers all the branch instructions we've seen
	// and where they would like to go.
	Branches []Branch

	// JumpTables remembers all the jump tables we've seen.
	JumpTables []*ssa.Block

	// bstart remembers where each block starts (indexed by block ID)
	bstart []*obj.Prog

	maxarg int64 // largest frame size for arguments to calls made by the function

	// Map from GC safe points to liveness index, generated by
	// liveness analysis.
	livenessMap liveness.Map

	// partLiveArgs includes arguments that may be partially live, for which we
	// need to generate instructions that spill the argument registers.
	partLiveArgs map[*ir.Name]bool

	// lineRunStart records the beginning of the current run of instructions
	// within a single block sharing the same line number
	// Used to move statement marks to the beginning of such runs.
	lineRunStart *obj.Prog

	// wasm: The number of values on the WebAssembly stack. This is only used as a safeguard.
	OnWasmStackSkipped int
}

func (s *State) FuncInfo() *obj.FuncInfo {
	return s.pp.CurFunc.LSym.Func()
}

// Prog appends a new Prog.
func (s *State) Prog(as obj.As) *obj.Prog {
	p := s.pp.Prog(as)
	if objw.LosesStmtMark(as) {
		return p
	}
	// Float a statement start to the beginning of any same-line run.
	// lineRunStart is reset at block boundaries, which appears to work well.
	if s.lineRunStart == nil || s.lineRunStart.Pos.Line() != p.Pos.Line() {
		s.lineRunStart = p
	} else if p.Pos.IsStmt() == src.PosIsStmt {
		s.lineRunStart.Pos = s.lineRunStart.Pos.WithIsStmt()
		p.Pos = p.Pos.WithNotStmt()
	}
	return p
}

// Pc returns the current Prog.
func (s *State) Pc() *obj.Prog {
	return s.pp.Next
}

// SetPos sets the current source position.
func (s *State) SetPos(pos src.XPos) {
	s.pp.Pos = pos
}

// Br emits a single branch instruction and returns the instruction.
// Not all architectures need the returned instruction, but otherwise
// the boilerplate is common to all.
func (s *State) Br(op obj.As, target *ssa.Block) *obj.Prog {
	p := s.Prog(op)
	p.To.Type = obj.TYPE_BRANCH
	s.Branches = append(s.Branches, Branch{P: p, B: target})
	return p
}

// DebugFriendlySetPosFrom adjusts Pos.IsStmt subject to heuristics
// that reduce "jumpy" line number churn when debugging.
// Spill/fill/copy instructions from the register allocator,
// phi functions, and instructions with a no-pos position
// are examples of instructions that can cause churn.
func (s *State) DebugFriendlySetPosFrom(v *ssa.Value) {
	switch v.Op {
	case ssa.OpPhi, ssa.OpCopy, ssa.OpLoadReg, ssa.OpStoreReg:
		// These are not statements
		s.SetPos(v.Pos.WithNotStmt())
	default:
		p := v.Pos
		if p != src.NoXPos {
			// If the position is defined, update the position.
			// Also convert default IsStmt to NotStmt; only
			// explicit statement boundaries should appear
			// in the generated code.
			if p.IsStmt() != src.PosIsStmt {
				if s.pp.Pos.IsStmt() == src.PosIsStmt && s.pp.Pos.SameFileAndLine(p) {
					// If s.pp.Pos already has a statement mark, then it was set here (below) for
					// the previous value.  If an actual instruction had been emitted for that
					// value, then the statement mark would have been reset.  Since the statement
					// mark of s.pp.Pos was not reset, this position (file/line) still needs a
					// statement mark on an instruction.  If file and line for this value are
					// the same as the previous value, then the first instruction for this
					// value will work to take the statement mark.  Return early to avoid
					// resetting the statement mark.
					//
					// The reset of s.pp.Pos occurs in (*Progs).Prog() -- if it emits
					// an instruction, and the instruction's statement mark was set,
					// and it is not one of the LosesStmtMark instructions,
					// then Prog() resets the statement mark on the (*Progs).Pos.
					return
				}
				p = p.WithNotStmt()
				// Calls use the pos attached to v, but copy the statement mark from State
			}
			s.SetPos(p)
		} else {
			s.SetPos(s.pp.Pos.WithNotStmt())
		}
	}
}

// emit argument info (locations on stack) for traceback.
func emitArgInfo(e *ssafn, f *ssa.Func, pp *objw.Progs) {
	ft := e.curfn.Type()
	if ft.NumRecvs() == 0 && ft.NumParams() == 0 {
		return
	}

	x := EmitArgInfo(e.curfn, f.OwnAux.ABIInfo())
	x.Set(obj.AttrContentAddressable, true)
	e.curfn.LSym.Func().ArgInfo = x

	// Emit a funcdata pointing at the arg info data.
	p := pp.Prog(obj.AFUNCDATA)
	p.From.SetConst(rtabi.FUNCDATA_ArgInfo)
	p.To.Type = obj.TYPE_MEM
	p.To.Name = obj.NAME_EXTERN
	p.To.Sym = x
}

// emit argument info (locations on stack) of f for traceback.
func EmitArgInfo(f *ir.Func, abiInfo *abi.ABIParamResultInfo) *obj.LSym {
	x := base.Ctxt.Lookup(fmt.Sprintf("%s.arginfo%d", f.LSym.Name, f.ABI))
	x.Align = 1
	// NOTE: do not set ContentAddressable here. This may be referenced from
	// assembly code by name (in this case f is a declaration).
	// Instead, set it in emitArgInfo above.

	PtrSize := int64(types.PtrSize)
	uintptrTyp := types.Types[types.TUINTPTR]

	isAggregate := func(t *types.Type) bool {
		return isStructNotSIMD(t) || t.IsArray() || t.IsComplex() || t.IsInterface() || t.IsString() || t.IsSlice()
	}

	wOff := 0
	n := 0
	writebyte := func(o uint8) { wOff = objw.Uint8(x, wOff, o) }

	// Write one non-aggregate arg/field/element.
	write1 := func(sz, offset int64) {
		if offset >= rtabi.TraceArgsSpecial {
			writebyte(rtabi.TraceArgsOffsetTooLarge)
		} else {
			writebyte(uint8(offset))
			writebyte(uint8(sz))
		}
		n++
	}

	// Visit t recursively and write it out.
	// Returns whether to continue visiting.
	var visitType func(baseOffset int64, t *types.Type, depth int) bool
	visitType = func(baseOffset int64, t *types.Type, depth int) bool {
		if n >= rtabi.TraceArgsLimit {
			writebyte(rtabi.TraceArgsDotdotdot)
			return false
		}
		if !isAggregate(t) {
			write1(t.Size(), baseOffset)
			return true
		}
		writebyte(rtabi.TraceArgsStartAgg)
		depth++
		if depth >= rtabi.TraceArgsMaxDepth {
			writebyte(rtabi.TraceArgsDotdotdot)
			writebyte(rtabi.TraceArgsEndAgg)
			n++
			return true
		}
		switch {
		case t.IsInterface(), t.IsString():
			_ = visitType(baseOffset, uintptrTyp, depth) &&
				visitType(baseOffset+PtrSize, uintptrTyp, depth)
		case t.IsSlice():
			_ = visitType(baseOffset, uintptrTyp, depth) &&
				visitType(baseOffset+PtrSize, uintptrTyp, depth) &&
				visitType(baseOffset+PtrSize*2, uintptrTyp, depth)
		case t.IsComplex():
			_ = visitType(baseOffset, types.FloatForComplex(t), depth) &&
				visitType(baseOffset+t.Size()/2, types.FloatForComplex(t), depth)
		case t.IsArray():
			if t.NumElem() == 0 {
				n++ // {} counts as a component
				break
			}
			for i := int64(0); i < t.NumElem(); i++ {
				if !visitType(baseOffset, t.Elem(), depth) {
					break
				}
				baseOffset += t.Elem().Size()
			}
		case isStructNotSIMD(t):
			if t.NumFields() == 0 {
				n++ // {} counts as a component
				break
			}
			for _, field := range t.Fields() {
				if !visitType(baseOffset+field.Offset, field.Type, depth) {
					break
				}
			}
		}
		writebyte(rtabi.TraceArgsEndAgg)
		return true
	}

	start := 0
	if strings.Contains(f.LSym.Name, "[") {
		// Skip the dictionary argument - it is implicit and the user doesn't need to see it.
		start = 1
	}

	for _, a := range abiInfo.InParams()[start:] {
		if !visitType(a.FrameOffset(abiInfo), a.Type, 0) {
			break
		}
	}
	writebyte(rtabi.TraceArgsEndSeq)
	if wOff > rtabi.TraceArgsMaxLen {
		base.Fatalf("ArgInfo too large")
	}

	return x
}

// for wrapper, emit info of wrapped function.
func emitWrappedFuncInfo(e *ssafn, pp *objw.Progs) {
	if base.Ctxt.Flag_linkshared {
		// Relative reference (SymPtrOff) to another shared object doesn't work.
		// Unfortunate.
		return
	}

	wfn := e.curfn.WrappedFunc
	if wfn == nil {
		return
	}

	wsym := wfn.Linksym()
	x := base.Ctxt.LookupInit(fmt.Sprintf("%s.wrapinfo", wsym.Name), func(x *obj.LSym) {
		objw.SymPtrOff(x, 0, wsym)
		x.Set(obj.AttrContentAddressable, true)
		x.Align = 4
	})
	e.curfn.LSym.Func().WrapInfo = x

	// Emit a funcdata pointing at the wrap info data.
	p := pp.Prog(obj.AFUNCDATA)
	p.From.SetConst(rtabi.FUNCDATA_WrapInfo)
	p.To.Type = obj.TYPE_MEM
	p.To.Name = obj.NAME_EXTERN
	p.To.Sym = x
}

// genssa appends entries to pp for each instruction in f.
func genssa(f *ssa.Func, pp *objw.Progs) {
	var s State
	s.ABI = f.OwnAux.Fn.ABI()

	e := f.Frontend().(*ssafn)

	gatherPrintInfo := f.PrintOrHtmlSSA || ssa.GenssaDump[f.Name]

	var lv *liveness.Liveness
	s.livenessMap, s.partLiveArgs, lv = liveness.Compute(e.curfn, f, e.stkptrsize, pp, gatherPrintInfo)
	emitArgInfo(e, f, pp)
	argLiveBlockMap, argLiveValueMap := liveness.ArgLiveness(e.curfn, f, pp)

	openDeferInfo := e.curfn.LSym.Func().OpenCodedDeferInfo
	if openDeferInfo != nil {
		// This function uses open-coded defers -- write out the funcdata
		// info that we computed at the end of genssa.
		p := pp.Prog(obj.AFUNCDATA)
		p.From.SetConst(rtabi.FUNCDATA_OpenCodedDeferInfo)
		p.To.Type = obj.TYPE_MEM
		p.To.Name = obj.NAME_EXTERN
		p.To.Sym = openDeferInfo
	}

	emitWrappedFuncInfo(e, pp)

	// Remember where each block starts.
	s.bstart = make([]*obj.Prog, f.NumBlocks())
	s.pp = pp
	var progToValue map[*obj.Prog]*ssa.Value
	var progToBlock map[*obj.Prog]*ssa.Block
	var valueToProgAfter []*obj.Prog // The first Prog following computation of a value v; v is visible at this point.
	if gatherPrintInfo {
		progToValue = make(map[*obj.Prog]*ssa.Value, f.NumValues())
		progToBlock = make(map[*obj.Prog]*ssa.Block, f.NumBlocks())
		f.Logf("genssa %s\n", f.Name)
		progToBlock[s.pp.Next] = f.Blocks[0]
	}

	if base.Ctxt.Flag_locationlists {
		if cap(f.Cache.ValueToProgAfter) < f.NumValues() {
			f.Cache.ValueToProgAfter = make([]*obj.Prog, f.NumValues())
		}
		valueToProgAfter = f.Cache.ValueToProgAfter[:f.NumValues()]
		clear(valueToProgAfter)
	}

	// If the very first instruction is not tagged as a statement,
	// debuggers may attribute it to previous function in program.
	firstPos := src.NoXPos
	for _, v := range f.Entry.Values {
		if v.Pos.IsStmt() == src.PosIsStmt && v.Op != ssa.OpArg && v.Op != ssa.OpArgIntReg && v.Op != ssa.OpArgFloatReg && v.Op != ssa.OpLoadReg && v.Op != ssa.OpStoreReg {
			firstPos = v.Pos
			v.Pos = firstPos.WithDefaultStmt()
			break
		}
	}

	// inlMarks has an entry for each Prog that implements an inline mark.
	// It maps from that Prog to the global inlining id of the inlined body
	// which should unwind to this Prog's location.
	var inlMarks map[*obj.Prog]int32
	var inlMarkList []*obj.Prog

	// inlMarksByPos maps from a (column 1) source position to the set of
	// Progs that are in the set above and have that source position.
	var inlMarksByPos map[src.XPos][]*obj.Prog

	var argLiveIdx int = -1 // argument liveness info index

	// These control cache line alignment; if the required portion of
	// a cache line is not available, then pad to obtain cache line
	// alignment.  Not implemented on all architectures, may not be
	// useful on all architectures.
	var hotAlign, hotRequire int64

	if base.Debug.AlignHot > 0 {
		switch base.Ctxt.Arch.Name {
		// enable this on a case-by-case basis, with benchmarking.
		// currently shown:
		//   good for amd64
		//   not helpful for Apple Silicon
		//
		case "amd64", "386":
			// Align to 64 if 31 or fewer bytes remain in a cache line
			// benchmarks a little better than always aligning, and also
			// adds slightly less to the (PGO-compiled) binary size.
			hotAlign = 64
			hotRequire = 31
		}
	}

	// Emit basic blocks
	for i, b := range f.Blocks {

		s.lineRunStart = nil
		s.SetPos(s.pp.Pos.WithNotStmt()) // It needs a non-empty Pos, but cannot be a statement boundary (yet).

		if hotAlign > 0 && b.Hotness&ssa.HotPgoInitial == ssa.HotPgoInitial {
			// So far this has only been shown profitable for PGO-hot loop headers.
			// The Hotness values allows distinctions between initial blocks that are "hot" or not, and "flow-in" or not.
			// Currently only the initial blocks of loops are tagged in this way;
			// there are no blocks tagged "pgo-hot" that are not also tagged "initial".
			// TODO more heuristics, more architectures.
			p := s.pp.Prog(obj.APCALIGNMAX)
			p.From.SetConst(hotAlign)
			p.To.SetConst(hotRequire)
		}

		s.bstart[b.ID] = s.pp.Next

		if idx, ok := argLiveBlockMap[b.ID]; ok && idx != argLiveIdx {
			argLiveIdx = idx
			p := s.pp.Prog(obj.APCDATA)
			p.From.SetConst(rtabi.PCDATA_ArgLiveIndex)
			p.To.SetConst(int64(idx))
		}

		// Emit values in block
		Arch.SSAMarkMoves(&s, b)
		for _, v := range b.Values {
			x := s.pp.Next
			s.DebugFriendlySetPosFrom(v)

			if v.Op.ResultInArg0() && v.ResultReg() != v.Args[0].Reg() {
				v.Fatalf("input[0] and output not in same register %s", v.LongString())
			}

			switch v.Op {
			case ssa.OpInitMem:
				// memory arg needs no code
			case ssa.OpArg:
				// input args need no code
			case ssa.OpSP, ssa.OpSB:
				// nothing to do
			case ssa.OpSelect0, ssa.OpSelect1, ssa.OpSelectN, ssa.OpMakeResult:
				// nothing to do
			case ssa.OpGetG:
				// nothing to do when there's a g register,
				// and checkLower complains if there's not
			case ssa.OpVarDef, ssa.OpVarLive, ssa.OpKeepAlive, ssa.OpWBend:
				// nothing to do; already used by liveness
			case ssa.OpPhi:
				CheckLoweredPhi(v)
			case ssa.OpConvert:
				// nothing to do; no-op conversion for liveness
				if v.Args[0].Reg() != v.Reg() {
					v.Fatalf("OpConvert should be a no-op: %s; %s", v.Args[0].LongString(), v.LongString())
				}
			case ssa.OpInlMark:
				p := Arch.Ginsnop(s.pp)
				if inlMarks == nil {
					inlMarks = map[*obj.Prog]int32{}
					inlMarksByPos = map[src.XPos][]*obj.Prog{}
				}
				inlMarks[p] = v.AuxInt32()
				inlMarkList = append(inlMarkList, p)
				pos := v.Pos.AtColumn1()
				inlMarksByPos[pos] = append(inlMarksByPos[pos], p)
				firstPos = src.NoXPos

			default:
				// Special case for first line in function; move it to the start (which cannot be a register-valued instruction)
				if firstPos != src.NoXPos && v.Op != ssa.OpArgIntReg && v.Op != ssa.OpArgFloatReg && v.Op != ssa.OpLoadReg && v.Op != ssa.OpStoreReg {
					s.SetPos(firstPos)
					firstPos = src.NoXPos
				}
				// Attach this safe point to the next
				// instruction.
				s.pp.NextLive = s.livenessMap.Get(v)
				s.pp.NextUnsafe = s.livenessMap.GetUnsafe(v)

				// let the backend handle it
				Arch.SSAGenValue(&s, v)
			}

			if idx, ok := argLiveValueMap[v.ID]; ok && idx != argLiveIdx {
				argLiveIdx = idx
				p := s.pp.Prog(obj.APCDATA)
				p.From.SetConst(rtabi.PCDATA_ArgLiveIndex)
				p.To.SetConst(int64(idx))
			}

			if base.Ctxt.Flag_locationlists {
				valueToProgAfter[v.ID] = s.pp.Next
			}

			if gatherPrintInfo {
				for ; x != s.pp.Next; x = x.Link {
					progToValue[x] = v
				}
			}
		}
		// If this is an empty infinite loop, stick a hardware NOP in there so that debuggers are less confused.
		if s.bstart[b.ID] == s.pp.Next && len(b.Succs) == 1 && b.Succs[0].Block() == b {
			p := Arch.Ginsnop(s.pp)
			p.Pos = p.Pos.WithIsStmt()
			if b.Pos == src.NoXPos {
				b.Pos = p.Pos // It needs a file, otherwise a no-file non-zero line causes confusion.  See #35652.
				if b.Pos == src.NoXPos {
					b.Pos = s.pp.Text.Pos // Sometimes p.Pos is empty.  See #35695.
				}
			}
			b.Pos = b.Pos.WithBogusLine() // Debuggers are not good about infinite loops, force a change in line number
		}

		// Set unsafe mark for any end-of-block generated instructions
		// (normally, conditional or unconditional branches).
		// This is particularly important for empty blocks, as there
		// are no values to inherit the unsafe mark from.
		s.pp.NextUnsafe = s.livenessMap.GetUnsafeBlock(b)

		// Emit control flow instructions for block
		var next *ssa.Block
		if i < len(f.Blocks)-1 && base.Flag.N == 0 {
			// If -N, leave next==nil so every block with successors
			// ends in a JMP (except call blocks - plive doesn't like
			// select{send,recv} followed by a JMP call).  Helps keep
			// line numbers for otherwise empty blocks.
			next = f.Blocks[i+1]
		}
		x := s.pp.Next
		s.SetPos(b.Pos)
		Arch.SSAGenBlock(&s, b, next)
		if gatherPrintInfo {
			for ; x != s.pp.Next; x = x.Link {
				progToBlock[x] = b
			}
		}
	}
	if f.Blocks[len(f.Blocks)-1].Kind == ssa.BlockExit {
		// We need the return address of a panic call to
		// still be inside the function in question. So if
		// it ends in a call which doesn't return, add a
		// nop (which will never execute) after the call.
		Arch.Ginsnop(s.pp)
	}
	if openDeferInfo != nil {
		// When doing open-coded defers, generate a disconnected call to
		// deferreturn and a return. This will be used to during panic
		// recovery to unwind the stack and return back to the runtime.

		// Note that this exit code doesn't work if a return parameter
		// is heap-allocated, but open defers aren't enabled in that case.

		// TODO either make this handle heap-allocated return parameters or reuse the other-defers general-purpose code path.
		s.pp.NextLive = s.livenessMap.DeferReturn
		p := s.pp.Prog(obj.ACALL)
		p.To.Type = obj.TYPE_MEM
		p.To.Name = obj.NAME_EXTERN
		p.To.Sym = ir.Syms.Deferreturn

		// Load results into registers. So when a deferred function
		// recovers a panic, it will return to caller with right results.
		// The results are already in memory, because they are not SSA'd
		// when the function has defers (see canSSAName).
		for _, o := range f.OwnAux.ABIInfo().OutParams() {
			n := o.Name
			rts, offs := o.RegisterTypesAndOffsets()
			for i := range o.Registers {
				Arch.LoadRegResult(&s, f, rts[i], ssa.ObjRegForAbiReg(o.Registers[i], f.Config), n, offs[i])
			}
		}

		s.pp.Prog(obj.ARET)
	}

	if inlMarks != nil {
		hasCall := false

		// We have some inline marks. Try to find other instructions we're
		// going to emit anyway, and use those instructions instead of the
		// inline marks.
		for p := s.pp.Text; p != nil; p = p.Link {
			if p.As == obj.ANOP || p.As == obj.AFUNCDATA || p.As == obj.APCDATA || p.As == obj.ATEXT ||
				p.As == obj.APCALIGN || p.As == obj.APCALIGNMAX || Arch.LinkArch.Family == sys.Wasm {
				// Don't use 0-sized instructions as inline marks, because we need
				// to identify inline mark instructions by pc offset.
				// (Some of these instructions are sometimes zero-sized, sometimes not.
				// We must not use anything that even might be zero-sized.)
				// TODO: are there others?
				continue
			}
			if _, ok := inlMarks[p]; ok {
				// Don't use inline marks themselves. We don't know
				// whether they will be zero-sized or not yet.
				continue
			}
			if p.As == obj.ACALL || p.As == obj.ADUFFCOPY || p.As == obj.ADUFFZERO {
				hasCall = true
			}
			pos := p.Pos.AtColumn1()
			marks := inlMarksByPos[pos]
			if len(marks) == 0 {
				continue
			}
			for _, m := range marks {
				// We found an instruction with the same source position as
				// some of the inline marks.
				// Use this instruction instead.
				p.Pos = p.Pos.WithIsStmt() // promote position to a statement
				s.pp.CurFunc.LSym.Func().AddInlMark(p, inlMarks[m])
				// Make the inline mark a real nop, so it doesn't generate any code.
				m.As = obj.ANOP
				m.Pos = src.NoXPos
				m.From = obj.Addr{}
				m.To = obj.Addr{}
			}
			delete(inlMarksByPos, pos)
		}
		// Any unmatched inline marks now need to be added to the inlining tree (and will generate a nop instruction).
		for _, p := range inlMarkList {
			if p.As != obj.ANOP {
				s.pp.CurFunc.LSym.Func().AddInlMark(p, inlMarks[p])
			}
		}

		if e.stksize == 0 && !hasCall {
			// Frameless leaf function. It doesn't need any preamble,
			// so make sure its first instruction isn't from an inlined callee.
			// If it is, add a nop at the start of the function with a position
			// equal to the start of the function.
			// This ensures that runtime.FuncForPC(uintptr(reflect.ValueOf(fn).Pointer())).Name()
			// returns the right answer. See issue 58300.
			for p := s.pp.Text; p != nil; p = p.Link {
				if p.As == obj.AFUNCDATA || p.As == obj.APCDATA || p.As == obj.ATEXT || p.As == obj.ANOP {
					continue
				}
				if base.Ctxt.PosTable.Pos(p.Pos).Base().InliningIndex() >= 0 {
					// Make a real (not 0-sized) nop.
					nop := Arch.Ginsnop(s.pp)
					nop.Pos = e.curfn.Pos().WithIsStmt()

					// Unfortunately, Ginsnop puts the instruction at the
					// end of the list. Move it up to just before p.

					// Unlink from the current list.
					for x := s.pp.Text; x != nil; x = x.Link {
						if x.Link == nop {
							x.Link = nop.Link
							break
						}
					}
					// Splice in right before p.
					for x := s.pp.Text; x != nil; x = x.Link {
						if x.Link == p {
							nop.Link = p
							x.Link = nop
							break
						}
					}
				}
				break
			}
		}
	}

	if base.Ctxt.Flag_locationlists {
		var debugInfo *ssa.FuncDebug
		debugInfo = e.curfn.DebugInfo.(*ssa.FuncDebug)
		// Save off entry ID in case we need it later for DWARF generation
		// for return values promoted to the heap.
		debugInfo.EntryID = f.Entry.ID
		if e.curfn.ABI == obj.ABIInternal && base.Flag.N != 0 {
			ssa.BuildFuncDebugNoOptimized(base.Ctxt, f, base.Debug.LocationLists > 1, StackOffset, debugInfo)
		} else {
			ssa.BuildFuncDebug(base.Ctxt, f, base.Debug.LocationLists, StackOffset, debugInfo)
		}
		bstart := s.bstart
		idToIdx := make([]int, f.NumBlocks())
		for i, b := range f.Blocks {
			idToIdx[b.ID] = i
		}
		// Register a callback that will be used later to fill in PCs into location
		// lists. At the moment, Prog.Pc is a sequence number; it's not a real PC
		// until after assembly, so the translation needs to be deferred.
		debugInfo.GetPC = func(b, v ssa.ID) int64 {
			switch v {
			case ssa.BlockStart.ID:
				if b == f.Entry.ID {
					return 0 // Start at the very beginning, at the assembler-generated prologue.
					// this should only happen for function args (ssa.OpArg)
				}
				return bstart[b].Pc
			case ssa.BlockEnd.ID:
				blk := f.Blocks[idToIdx[b]]
				nv := len(blk.Values)
				return valueToProgAfter[blk.Values[nv-1].ID].Pc
			case ssa.FuncEnd.ID:
				return e.curfn.LSym.Size
			default:
				return valueToProgAfter[v].Pc
			}
		}
	}

	// Resolve branches, and relax DefaultStmt into NotStmt
	for _, br := range s.Branches {
		br.P.To.SetTarget(s.bstart[br.B.ID])
		if br.P.Pos.IsStmt() != src.PosIsStmt {
			br.P.Pos = br.P.Pos.WithNotStmt()
		} else if v0 := br.B.FirstPossibleStmtValue(); v0 != nil && v0.Pos.Line() == br.P.Pos.Line() && v0.Pos.IsStmt() == src.PosIsStmt {
			br.P.Pos = br.P.Pos.WithNotStmt()
		}

	}

	// Resolve jump table destinations.
	for _, jt := range s.JumpTables {
		// Convert from *Block targets to *Prog targets.
		targets := make([]*obj.Prog, len(jt.Succs))
		for i, e := range jt.Succs {
			targets[i] = s.bstart[e.Block().ID]
		}
		// Add to list of jump tables to be resolved at assembly time.
		// The assembler converts from *Prog entries to absolute addresses
		// once it knows instruction byte offsets.
		fi := s.pp.CurFunc.LSym.Func()
		fi.JumpTables = append(fi.JumpTables, obj.JumpTable{Sym: jt.Aux.(*obj.LSym), Targets: targets})
	}

	if e.log { // spew to stdout
		filename := ""
		for p := s.pp.Text; p != nil; p = p.Link {
			if p.Pos.IsKnown() && p.InnermostFilename() != filename {
				filename = p.InnermostFilename()
				f.Logf("# %s\n", filename)
			}

			var s string
			if v, ok := progToValue[p]; ok {
				s = v.String()
			} else if b, ok := progToBlock[p]; ok {
				s = b.String()
			} else {
				s = "   " // most value and branch strings are 2-3 characters long
			}
			f.Logf(" %-6s\t%.5d (%s)\t%s\n", s, p.Pc, p.InnermostLineNumber(), p.InstructionString())
		}
	}
	if f.HTMLWriter != nil { // spew to ssa.html
		var buf strings.Builder
		buf.WriteString("<code>")
		buf.WriteString("<dl class=\"ssa-gen\">")
		filename := ""

		liveness := lv.Format(nil)
		if liveness != "" {
			buf.WriteString("<dt class=\"ssa-prog-src\"></dt><dd class=\"ssa-prog\">")
			buf.WriteString(html.EscapeString("# " + liveness))
			buf.WriteString("</dd>")
		}

		for p := s.pp.Text; p != nil; p = p.Link {
			// Don't spam every line with the file name, which is often huge.
			// Only print changes, and "unknown" is not a change.
			if p.Pos.IsKnown() && p.InnermostFilename() != filename {
				filename = p.InnermostFilename()
				buf.WriteString("<dt class=\"ssa-prog-src\"></dt><dd class=\"ssa-prog\">")
				buf.WriteString(html.EscapeString("# " + filename))
				buf.WriteString("</dd>")
			}

			buf.WriteString("<dt class=\"ssa-prog-src\">")
			if v, ok := progToValue[p]; ok {

				// Prefix calls with their liveness, if any
				if p.As != obj.APCDATA {
					if liveness := lv.Format(v); liveness != "" {
						// Steal this line, and restart a line
						buf.WriteString("</dt><dd class=\"ssa-prog\">")
						buf.WriteString(html.EscapeString("# " + liveness))
						buf.WriteString("</dd>")
						// restarting a line
						buf.WriteString("<dt class=\"ssa-prog-src\">")
					}
				}

				buf.WriteString(v.HTML())
			} else if b, ok := progToBlock[p]; ok {
				buf.WriteString("<b>" + b.HTML() + "</b>")
			}
			buf.WriteString("</dt>")
			buf.WriteString("<dd class=\"ssa-prog\">")
			fmt.Fprintf(&buf, "%.5d <span class=\"l%v line-number\">(%s)</span> %s", p.Pc, p.InnermostLineNumber(), p.InnermostLineNumberHTML(), html.EscapeString(p.InstructionString()))
			buf.WriteString("</dd>")
		}
		buf.WriteString("</dl>")
		buf.WriteString("</code>")
		f.HTMLWriter.WriteColumn("genssa", "genssa", "ssa-prog", buf.String())
	}
	if ssa.GenssaDump[f.Name] {
		fi := f.DumpFileForPhase("genssa")
		if fi != nil {

			// inliningDiffers if any filename changes or if any line number except the innermost (last index) changes.
			inliningDiffers := func(a, b []src.Pos) bool {
				if len(a) != len(b) {
					return true
				}
				for i := range a {
					if a[i].Filename() != b[i].Filename() {
						return true
					}
					if i != len(a)-1 && a[i].Line() != b[i].Line() {
						return true
					}
				}
				return false
			}

			var allPosOld []src.Pos
			var allPos []src.Pos

			for p := s.pp.Text; p != nil; p = p.Link {
				if p.Pos.IsKnown() {
					allPos = allPos[:0]
					p.Ctxt.AllPos(p.Pos, func(pos src.Pos) { allPos = append(allPos, pos) })
					if inliningDiffers(allPos, allPosOld) {
						for _, pos := range allPos {
							fmt.Fprintf(fi, "# %s:%d\n", pos.Filename(), pos.Line())
						}
						allPos, allPosOld = allPosOld, allPos // swap, not copy, so that they do not share slice storage.
					}
				}

				var s string
				if v, ok := progToValue[p]; ok {
					s = v.String()
				} else if b, ok := progToBlock[p]; ok {
					s = b.String()
				} else {
					s = "   " // most value and branch strings are 2-3 characters long
				}
				fmt.Fprintf(fi, " %-6s\t%.5d %s\t%s\n", s, p.Pc, ssa.StmtString(p.Pos), p.InstructionString())
			}
			fi.Close()
		}
	}

	defframe(&s, e, f)

	f.HTMLWriter.Close()
	f.HTMLWriter = nil
}

func defframe(s *State, e *ssafn, f *ssa.Func) {
	pp := s.pp

	s.maxarg = types.RoundUp(s.maxarg, e.stkalign)
	frame := s.maxarg + e.stksize
	if Arch.PadFrame != nil {
		frame = Arch.PadFrame(frame)
	}

	// Fill in argument and frame size.
	pp.Text.To.Type = obj.TYPE_TEXTSIZE
	pp.Text.To.Val = int32(types.RoundUp(f.OwnAux.ArgWidth(), int64(types.RegSize)))
	pp.Text.To.Offset = frame

	p := pp.Text

	// Insert code to spill argument registers if the named slot may be partially
	// live. That is, the named slot is considered live by liveness analysis,
	// (because a part of it is live), but we may not spill all parts into the
	// slot. This can only happen with aggregate-typed arguments that are SSA-able
	// and not address-taken (for non-SSA-able or address-taken arguments we always
	// spill upfront).
	// Note: spilling is unnecessary in the -N/no-optimize case, since all values
	// will be considered non-SSAable and spilled up front.
	// TODO(register args) Make liveness more fine-grained to that partial spilling is okay.
	if f.OwnAux.ABIInfo().InRegistersUsed() != 0 && base.Flag.N == 0 {
		// First, see if it is already spilled before it may be live. Look for a spill
		// in the entry block up to the first safepoint.
		type nameOff struct {
			n   *ir.Name
			off int64
		}
		partLiveArgsSpilled := make(map[nameOff]bool)
		for _, v := range f.Entry.Values {
			if v.Op.IsCall() {
				break
			}
			if v.Op != ssa.OpStoreReg || v.Args[0].Op != ssa.OpArgIntReg {
				continue
			}
			n, off := ssa.AutoVar(v)
			if n.Class != ir.PPARAM || n.Addrtaken() || !ssa.CanSSA(n.Type()) || !s.partLiveArgs[n] {
				continue
			}
			partLiveArgsSpilled[nameOff{n, off}] = true
		}

		// Then, insert code to spill registers if not already.
		for _, a := range f.OwnAux.ABIInfo().InParams() {
			n := a.Name
			if n == nil || n.Addrtaken() || !ssa.CanSSA(n.Type()) || !s.partLiveArgs[n] || len(a.Registers) <= 1 {
				continue
			}
			rts, offs := a.RegisterTypesAndOffsets()
			for i := range a.Registers {
				if !rts[i].HasPointers() {
					continue
				}
				if partLiveArgsSpilled[nameOff{n, offs[i]}] {
					continue // already spilled
				}
				reg := ssa.ObjRegForAbiReg(a.Registers[i], f.Config)
				p = Arch.SpillArgReg(pp, p, f, rts[i], reg, n, offs[i])
			}
		}
	}

	// Insert code to zero ambiguously live variables so that the
	// garbage collector only sees initialized values when it
	// looks for pointers.
	var lo, hi int64

	// Opaque state for backend to use. Current backends use it to
	// keep track of which helper registers have been zeroed.
	var state uint32

	// Iterate through declarations. Autos are sorted in decreasing
	// frame offset order.
	for _, n := range e.curfn.Dcl {
		if !n.Needzero() {
			continue
		}
		if n.Class != ir.PAUTO {
			e.Fatalf(n.Pos(), "needzero class %d", n.Class)
		}
		if n.Type().Size()%int64(types.PtrSize) != 0 || n.FrameOffset()%int64(types.PtrSize) != 0 || n.Type().Size() == 0 {
			e.Fatalf(n.Pos(), "var %L has size %d offset %d", n, n.Type().Size(), n.Offset_)
		}

		if lo != hi && n.FrameOffset()+n.Type().Size() >= lo-int64(2*types.RegSize) {
			// Merge with range we already have.
			lo = n.FrameOffset()
			continue
		}

		// Zero old range
		p = Arch.ZeroRange(pp, p, frame+lo, hi-lo, &state)

		// Set new range.
		lo = n.FrameOffset()
		hi = lo + n.Type().Size()
	}

	// Zero final range.
	Arch.ZeroRange(pp, p, frame+lo, hi-lo, &state)
}

// For generating consecutive jump instructions to model a specific branching
type IndexJump struct {
	Jump  obj.As
	Index int
}

func (s *State) oneJump(b *ssa.Block, jump *IndexJump) {
	p := s.Br(jump.Jump, b.Succs[jump.Index].Block())
	p.Pos = b.Pos
}

// CombJump generates combinational instructions (2 at present) for a block jump,
// thereby the behaviour of non-standard condition codes could be simulated
func (s *State) CombJump(b, next *ssa.Block, jumps *[2][2]IndexJump) {
	switch next {
	case b.Succs[0].Block():
		s.oneJump(b, &jumps[0][0])
		s.oneJump(b, &jumps[0][1])
	case b.Succs[1].Block():
		s.oneJump(b, &jumps[1][0])
		s.oneJump(b, &jumps[1][1])
	default:
		var q *obj.Prog
		if b.Likely != ssa.BranchUnlikely {
			s.oneJump(b, &jumps[1][0])
			s.oneJump(b, &jumps[1][1])
			q = s.Br(obj.AJMP, b.Succs[1].Block())
		} else {
			s.oneJump(b, &jumps[0][0])
			s.oneJump(b, &jumps[0][1])
			q = s.Br(obj.AJMP, b.Succs[0].Block())
		}
		q.Pos = b.Pos
	}
}

// AddAux adds the offset in the aux fields (AuxInt and Aux) of v to a.
func AddAux(a *obj.Addr, v *ssa.Value) {
	AddAux2(a, v, v.AuxInt)
}
func AddAux2(a *obj.Addr, v *ssa.Value, offset int64) {
	if a.Type != obj.TYPE_MEM && a.Type != obj.TYPE_ADDR {
		v.Fatalf("bad AddAux addr %v", a)
	}
	// add integer offset
	a.Offset += offset

	// If no additional symbol offset, we're done.
	if v.Aux == nil {
		return
	}
	// Add symbol's offset from its base register.
	switch n := v.Aux.(type) {
	case *ssa.AuxCall:
		a.Name = obj.NAME_EXTERN
		a.Sym = n.Fn
	case *obj.LSym:
		a.Name = obj.NAME_EXTERN
		a.Sym = n
	case *ir.Name:
		if n.Class == ir.PPARAM || (n.Class == ir.PPARAMOUT && !n.IsOutputParamInRegisters()) {
			a.Name = obj.NAME_PARAM
		} else {
			a.Name = obj.NAME_AUTO
		}
		a.Sym = n.Linksym()
		a.Offset += n.FrameOffset()
	default:
		v.Fatalf("aux in %s not implemented %#v", v, v.Aux)
	}
}

// extendIndex extends v to a full int width.
// panic with the given kind if v does not fit in an int (only on 32-bit archs).
func (s *state) extendIndex(idx, len *ssa.Value, kind ssa.BoundsKind, bounded bool) *ssa.Value {
	size := idx.Type.Size()
	if size == s.config.PtrSize {
		return idx
	}
	if size > s.config.PtrSize {
		// truncate 64-bit indexes on 32-bit pointer archs. Test the
		// high word and branch to out-of-bounds failure if it is not 0.
		var lo *ssa.Value
		if idx.Type.IsSigned() {
			lo = s.newValue1(ssa.OpInt64Lo, types.Types[types.TINT], idx)
		} else {
			lo = s.newValue1(ssa.OpInt64Lo, types.Types[types.TUINT], idx)
		}
		if bounded || base.Flag.B != 0 {
			return lo
		}
		bNext := s.f.NewBlock(ssa.BlockPlain)
		bPanic := s.f.NewBlock(ssa.BlockExit)
		hi := s.newValue1(ssa.OpInt64Hi, types.Types[types.TUINT32], idx)
		cmp := s.newValue2(ssa.OpEq32, types.Types[types.TBOOL], hi, s.constInt32(types.Types[types.TUINT32], 0))
		if !idx.Type.IsSigned() {
			switch kind {
			case ssa.BoundsIndex:
				kind = ssa.BoundsIndexU
			case ssa.BoundsSliceAlen:
				kind = ssa.BoundsSliceAlenU
			case ssa.BoundsSliceAcap:
				kind = ssa.BoundsSliceAcapU
			case ssa.BoundsSliceB:
				kind = ssa.BoundsSliceBU
			case ssa.BoundsSlice3Alen:
				kind = ssa.BoundsSlice3AlenU
			case ssa.BoundsSlice3Acap:
				kind = ssa.BoundsSlice3AcapU
			case ssa.BoundsSlice3B:
				kind = ssa.BoundsSlice3BU
			case ssa.BoundsSlice3C:
				kind = ssa.BoundsSlice3CU
			}
		}
		b := s.endBlock()
		b.Kind = ssa.BlockIf
		b.SetControl(cmp)
		b.Likely = ssa.BranchLikely
		b.AddEdgeTo(bNext)
		b.AddEdgeTo(bPanic)

		s.startBlock(bPanic)
		mem := s.newValue4I(ssa.OpPanicExtend, types.TypeMem, int64(kind), hi, lo, len, s.mem())
		s.endBlock().SetControl(mem)
		s.startBlock(bNext)

		return lo
	}

	// Extend value to the required size
	var op ssa.Op
	if idx.Type.IsSigned() {
		switch 10*size + s.config.PtrSize {
		case 14:
			op = ssa.OpSignExt8to32
		case 18:
			op = ssa.OpSignExt8to64
		case 24:
			op = ssa.OpSignExt16to32
		case 28:
			op = ssa.OpSignExt16to64
		case 48:
			op = ssa.OpSignExt32to64
		default:
			s.Fatalf("bad signed index extension %s", idx.Type)
		}
	} else {
		switch 10*size + s.config.PtrSize {
		case 14:
			op = ssa.OpZeroExt8to32
		case 18:
			op = ssa.OpZeroExt8to64
		case 24:
			op = ssa.OpZeroExt16to32
		case 28:
			op = ssa.OpZeroExt16to64
		case 48:
			op = ssa.OpZeroExt32to64
		default:
			s.Fatalf("bad unsigned index extension %s", idx.Type)
		}
	}
	return s.newValue1(op, types.Types[types.TINT], idx)
}

// CheckLoweredPhi checks that regalloc and stackalloc correctly handled phi values.
// Called during ssaGenValue.
func CheckLoweredPhi(v *ssa.Value) {
	if v.Op != ssa.OpPhi {
		v.Fatalf("CheckLoweredPhi called with non-phi value: %v", v.LongString())
	}
	if v.Type.IsMemory() {
		return
	}
	f := v.Block.Func
	loc := f.RegAlloc[v.ID]
	for _, a := range v.Args {
		if aloc := f.RegAlloc[a.ID]; aloc != loc { // TODO: .Equal() instead?
			v.Fatalf("phi arg at different location than phi: %v @ %s, but arg %v @ %s\n%s\n", v, loc, a, aloc, v.Block.Func)
		}
	}
}

// CheckLoweredGetClosurePtr checks that v is the first instruction in the function's entry block,
// except for incoming in-register arguments.
// The output of LoweredGetClosurePtr is generally hardwired to the correct register.
// That register contains the closure pointer on closure entry.
func CheckLoweredGetClosurePtr(v *ssa.Value) {
	entry := v.Block.Func.Entry
	if entry != v.Block {
		base.Fatalf("in %s, badly placed LoweredGetClosurePtr: %v %v", v.Block.Func.Name, v.Block, v)
	}
	for _, w := range entry.Values {
		if w == v {
			break
		}
		switch w.Op {
		case ssa.OpArgIntReg, ssa.OpArgFloatReg:
			// okay
		default:
			base.Fatalf("in %s, badly placed LoweredGetClosurePtr: %v %v", v.Block.Func.Name, v.Block, v)
		}
	}
}

// CheckArgReg ensures that v is in the function's entry block.
func CheckArgReg(v *ssa.Value) {
	entry := v.Block.Func.Entry
	if entry != v.Block {
		base.Fatalf("in %s, badly placed ArgIReg or ArgFReg: %v %v", v.Block.Func.Name, v.Block, v)
	}
}

func AddrAuto(a *obj.Addr, v *ssa.Value) {
	n, off := ssa.AutoVar(v)
	a.Type = obj.TYPE_MEM
	a.Sym = n.Linksym()
	a.Reg = int16(Arch.REGSP)
	a.Offset = n.FrameOffset() + off
	if n.Class == ir.PPARAM || (n.Class == ir.PPARAMOUT && !n.IsOutputParamInRegisters()) {
		a.Name = obj.NAME_PARAM
	} else {
		a.Name = obj.NAME_AUTO
	}
}

// Call returns a new CALL instruction for the SSA value v.
// It uses PrepareCall to prepare the call.
func (s *State) Call(v *ssa.Value) *obj.Prog {
	pPosIsStmt := s.pp.Pos.IsStmt() // The statement-ness of the call comes from ssaGenState
	s.PrepareCall(v)

	p := s.Prog(obj.ACALL)
	if pPosIsStmt == src.PosIsStmt {
		p.Pos = v.Pos.WithIsStmt()
	} else {
		p.Pos = v.Pos.WithNotStmt()
	}
	if sym, ok := v.Aux.(*ssa.AuxCall); ok && sym.Fn != nil {
		p.To.Type = obj.TYPE_MEM
		p.To.Name = obj.NAME_EXTERN
		p.To.Sym = sym.Fn
	} else {
		// TODO(mdempsky): Can these differences be eliminated?
		switch Arch.LinkArch.Family {
		case sys.AMD64, sys.I386, sys.PPC64, sys.RISCV64, sys.S390X, sys.Wasm:
			p.To.Type = obj.TYPE_REG
		case sys.ARM, sys.ARM64, sys.Loong64, sys.MIPS, sys.MIPS64:
			p.To.Type = obj.TYPE_MEM
		default:
			base.Fatalf("unknown indirect call family")
		}
		p.To.Reg = v.Args[0].Reg()
	}
	return p
}

// TailCall returns a new tail call instruction for the SSA value v.
// It is like Call, but for a tail call.
func (s *State) TailCall(v *ssa.Value) *obj.Prog {
	p := s.Call(v)
	p.As = obj.ARET
	return p
}

// PrepareCall prepares to emit a CALL instruction for v and does call-related bookkeeping.
// It must be called immediately before emitting the actual CALL instruction,
// since it emits PCDATA for the stack map at the call (calls are safe points).
func (s *State) PrepareCall(v *ssa.Value) {
	idx := s.livenessMap.Get(v)
	if !idx.StackMapValid() {
		// See Liveness.hasStackMap.
		if sym, ok := v.Aux.(*ssa.AuxCall); !ok || !(sym.Fn == ir.Syms.WBZero || sym.Fn == ir.Syms.WBMove) {
			base.Fatalf("missing stack map index for %v", v.LongString())
		}
	}

	call, ok := v.Aux.(*ssa.AuxCall)

	if ok {
		// Record call graph information for nowritebarrierrec
		// analysis.
		if nowritebarrierrecCheck != nil {
			nowritebarrierrecCheck.recordCall(s.pp.CurFunc, call.Fn, v.Pos)
		}
	}

	if s.maxarg < v.AuxInt {
		s.maxarg = v.AuxInt
	}
}

// UseArgs records the fact that an instruction needs a certain amount of
// callee args space for its use.
func (s *State) UseArgs(n int64) {
	if s.maxarg < n {
		s.maxarg = n
	}
}

// fieldIdx finds the index of the field referred to by the ODOT node n.
func fieldIdx(n *ir.SelectorExpr) int {
	t := n.X.Type()
	if !isStructNotSIMD(t) {
		panic("ODOT's LHS is not a struct")
	}

	for i, f := range t.Fields() {
		if f.Sym == n.Sel {
			if f.Offset != n.Offset() {
				panic("field offset doesn't match")
			}
			return i
		}
	}
	panic(fmt.Sprintf("can't find field in expr %v\n", n))

	// TODO: keep the result of this function somewhere in the ODOT Node
	// so we don't have to recompute it each time we need it.
}

// ssafn holds frontend information about a function that the backend is processing.
// It also exports a bunch of compiler services for the ssa backend.
type ssafn struct {
	curfn      *ir.Func
	strings    map[string]*obj.LSym // map from constant string to data symbols
	stksize    int64                // stack size for current frame
	stkptrsize int64                // prefix of stack containing pointers

	// alignment for current frame.
	// NOTE: when stkalign > PtrSize, currently this only ensures the offsets of
	// objects in the stack frame are aligned. The stack pointer is still aligned
	// only PtrSize.
	stkalign int64

	log bool // print ssa debug to the stdout
}

// StringData returns a symbol which
// is the data component of a global string constant containing s.
func (e *ssafn) StringData(s string) *obj.LSym {
	if aux, ok := e.strings[s]; ok {
		return aux
	}
	if e.strings == nil {
		e.strings = make(map[string]*obj.LSym)
	}
	data := staticdata.StringSym(e.curfn.Pos(), s)
	e.strings[s] = data
	return data
}

// SplitSlot returns a slot representing the data of parent starting at offset.
func (e *ssafn) SplitSlot(parent *ssa.LocalSlot, suffix string, offset int64, t *types.Type) ssa.LocalSlot {
	node := parent.N

	if node.Class != ir.PAUTO || node.Addrtaken() {
		// addressed things and non-autos retain their parents (i.e., cannot truly be split)
		return ssa.LocalSlot{N: node, Type: t, Off: parent.Off + offset}
	}

	sym := &types.Sym{Name: node.Sym().Name + suffix, Pkg: types.LocalPkg}
	n := e.curfn.NewLocal(parent.N.Pos(), sym, t)
	n.SetUsed(true)
	n.SetEsc(ir.EscNever)
	types.CalcSize(t)
	return ssa.LocalSlot{N: n, Type: t, Off: 0, SplitOf: parent, SplitOffset: offset}
}

// Logf logs a message from the compiler.
func (e *ssafn) Logf(msg string, args ...any) {
	if e.log {
		fmt.Printf(msg, args...)
	}
}

func (e *ssafn) Log() bool {
	return e.log
}

// Fatalf reports a compiler error and exits.
func (e *ssafn) Fatalf(pos src.XPos, msg string, args ...any) {
	base.Pos = pos
	nargs := append([]any{ir.FuncName(e.curfn)}, args...)
	base.Fatalf("'%s': "+msg, nargs...)
}

// Warnl reports a "warning", which is usually flag-triggered
// logging output for the benefit of tests.
func (e *ssafn) Warnl(pos src.XPos, fmt_ string, args ...any) {
	base.WarnfAt(pos, fmt_, args...)
}

func (e *ssafn) Debug_checknil() bool {
	return base.Debug.Nil != 0
}

func (e *ssafn) UseWriteBarrier() bool {
	return base.Flag.WB
}

func (e *ssafn) Syslook(name string) *obj.LSym {
	switch name {
	case "goschedguarded":
		return ir.Syms.Goschedguarded
	case "writeBarrier":
		return ir.Syms.WriteBarrier
	case "wbZero":
		return ir.Syms.WBZero
	case "wbMove":
		return ir.Syms.WBMove
	case "cgoCheckMemmove":
		return ir.Syms.CgoCheckMemmove
	case "cgoCheckPtrWrite":
		return ir.Syms.CgoCheckPtrWrite
	}
	e.Fatalf(src.NoXPos, "unknown Syslook func %v", name)
	return nil
}

func (e *ssafn) Func() *ir.Func {
	return e.curfn
}

func clobberBase(n ir.Node) ir.Node {
	if n.Op() == ir.ODOT {
		n := n.(*ir.SelectorExpr)
		if n.X.Type().NumFields() == 1 {
			return clobberBase(n.X)
		}
	}
	if n.Op() == ir.OINDEX {
		n := n.(*ir.IndexExpr)
		if n.X.Type().IsArray() && n.X.Type().NumElem() == 1 {
			return clobberBase(n.X)
		}
	}
	return n
}

// callTargetLSym returns the correct LSym to call 'callee' using its ABI.
func callTargetLSym(callee *ir.Name) *obj.LSym {
	if callee.Func == nil {
		// TODO(austin): This happens in case of interface method I.M from imported package.
		// It's ABIInternal, and would be better if callee.Func was never nil and we didn't
		// need this case.
		return callee.Linksym()
	}

	return callee.LinksymABI(callee.Func.ABI)
}

// deferStructFnField is the field index of _defer.fn.
const deferStructFnField = 4

var deferType *types.Type

// deferstruct returns a type interchangeable with runtime._defer.
// Make sure this stays in sync with runtime/runtime2.go:_defer.
func deferstruct() *types.Type {
	if deferType != nil {
		return deferType
	}

	makefield := func(name string, t *types.Type) *types.Field {
		sym := (*types.Pkg)(nil).Lookup(name)
		return types.NewField(src.NoXPos, sym, t)
	}

	fields := []*types.Field{
		makefield("heap", types.Types[types.TBOOL]),
		makefield("rangefunc", types.Types[types.TBOOL]),
		makefield("sp", types.Types[types.TUINTPTR]),
		makefield("pc", types.Types[types.TUINTPTR]),
		// Note: the types here don't really matter. Defer structures
		// are always scanned explicitly during stack copying and GC,
		// so we make them uintptr type even though they are real pointers.
		makefield("fn", types.Types[types.TUINTPTR]),
		makefield("link", types.Types[types.TUINTPTR]),
		makefield("head", types.Types[types.TUINTPTR]),
	}
	if name := fields[deferStructFnField].Sym.Name; name != "fn" {
		base.Fatalf("deferStructFnField is %q, not fn", name)
	}

	n := ir.NewDeclNameAt(src.NoXPos, ir.OTYPE, ir.Pkgs.Runtime.Lookup("_defer"))
	typ := types.NewNamed(n)
	n.SetType(typ)
	n.SetTypecheck(1)

	// build struct holding the above fields
	typ.SetUnderlying(types.NewStruct(fields))
	types.CalcStructSize(typ)

	deferType = typ
	return typ
}

// SpillSlotAddr uses LocalSlot information to initialize an obj.Addr
// The resulting addr is used in a non-standard context -- in the prologue
// of a function, before the frame has been constructed, so the standard
// addressing for the parameters will be wrong.
func SpillSlotAddr(spill ssa.Spill, baseReg int16, extraOffset int64) obj.Addr {
	return obj.Addr{
		Name:   obj.NAME_NONE,
		Type:   obj.TYPE_MEM,
		Reg:    baseReg,
		Offset: spill.Offset + extraOffset,
	}
}

func isStructNotSIMD(t *types.Type) bool {
	return t.IsStruct() && !t.IsSIMD()
}

var BoundsCheckFunc [ssa.BoundsKindCount]*obj.LSym

```

