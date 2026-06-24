# Domain Architecture: cmd/compile/internal/dwarfgen

## Layout Topology
```text
cmd/compile/internal/dwarfgen/
├── dwarf.go
├── dwinl.go
├── marker.go
└── scope.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/compile/internal/dwarfgen/dwarf.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dwarfgen

import (
	"bytes"
	"flag"
	"fmt"
	"internal/buildcfg"
	"slices"
	"sort"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/ssagen"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/dwarf"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"
)

func Info(ctxt *obj.Link, fnsym *obj.LSym, infosym *obj.LSym, curfn obj.Func) (scopes []dwarf.Scope, inlcalls dwarf.InlCalls) {
	fn := curfn.(*ir.Func)

	if fn.Nname != nil {
		expect := fn.Linksym()
		if fnsym.ABI() == obj.ABI0 {
			expect = fn.LinksymABI(obj.ABI0)
		}
		if fnsym != expect {
			base.Fatalf("unexpected fnsym: %v != %v", fnsym, expect)
		}
	}

	// Back when there were two different *Funcs for a function, this code
	// was not consistent about whether a particular *Node being processed
	// was an ODCLFUNC or ONAME node. Partly this is because inlined function
	// bodies have no ODCLFUNC node, which was it's own inconsistency.
	// In any event, the handling of the two different nodes for DWARF purposes
	// was subtly different, likely in unintended ways. CL 272253 merged the
	// two nodes' Func fields, so that code sees the same *Func whether it is
	// holding the ODCLFUNC or the ONAME. This resulted in changes in the
	// DWARF output. To preserve the existing DWARF output and leave an
	// intentional change for a future CL, this code does the following when
	// fn.Op == ONAME:
	//
	// 1. Disallow use of createComplexVars in createDwarfVars.
	//    It was not possible to reach that code for an ONAME before,
	//    because the DebugInfo was set only on the ODCLFUNC Func.
	//    Calling into it in the ONAME case causes an index out of bounds panic.
	//
	// 2. Do not populate apdecls. fn.Func.Dcl was in the ODCLFUNC Func,
	//    not the ONAME Func. Populating apdecls for the ONAME case results
	//    in selected being populated after createSimpleVars is called in
	//    createDwarfVars, and then that causes the loop to skip all the entries
	//    in dcl, meaning that the RecordAutoType calls don't happen.
	//
	// These two adjustments keep toolstash -cmp working for now.
	// Deciding the right answer is, as they say, future work.
	//
	// We can tell the difference between the old ODCLFUNC and ONAME
	// cases by looking at the infosym.Name. If it's empty, DebugInfo is
	// being called from (*obj.Link).populateDWARF, which used to use
	// the ODCLFUNC. If it's non-empty (the name will end in $abstract),
	// DebugInfo is being called from (*obj.Link).DwarfAbstractFunc,
	// which used to use the ONAME form.
	isODCLFUNC := infosym.Name == ""

	var apdecls []*ir.Name
	// Populate decls for fn.
	if isODCLFUNC {
		for _, n := range fn.Dcl {
			if n.Op() != ir.ONAME { // might be OTYPE or OLITERAL
				continue
			}
			switch n.Class {
			case ir.PAUTO:
				if !n.Used() {
					// Text == nil -> generating abstract function
					if fnsym.Func().Text != nil {
						base.Fatalf("debuginfo unused node (AllocFrame should truncate fn.Func.Dcl)")
					}
					continue
				}
			case ir.PPARAM, ir.PPARAMOUT:
			default:
				continue
			}
			if !shouldEmitDwarfVar(n) {
				continue
			}
			apdecls = append(apdecls, n)
			if n.Type().Kind() == types.TSSA {
				// Can happen for TypeInt128 types. This only happens for
				// spill locations, so not a huge deal.
				continue
			}
			fnsym.Func().RecordAutoType(reflectdata.TypeLinksym(n.Type()))
		}
	}

	var closureVars map[*ir.Name]int64
	if fn.Needctxt() {
		closureVars = make(map[*ir.Name]int64)
		csiter := typecheck.NewClosureStructIter(fn.ClosureVars)
		for {
			n, _, offset := csiter.Next()
			if n == nil {
				break
			}
			closureVars[n] = offset
			if n.Heapaddr != nil {
				closureVars[n.Heapaddr] = offset
			}
		}
	}

	decls, dwarfVars := createDwarfVars(fnsym, isODCLFUNC, fn, apdecls, closureVars)

	// For each type referenced by the functions auto vars but not
	// already referenced by a dwarf var, attach an R_USETYPE relocation to
	// the function symbol to insure that the type included in DWARF
	// processing during linking.
	// Do the same with R_USEIFACE relocations from the function symbol for the
	// same reason.
	// All these R_USETYPE relocations are only looked at if the function
	// survives deadcode elimination in the linker.
	typesyms := []*obj.LSym{}
	for t := range fnsym.Func().Autot {
		typesyms = append(typesyms, t)
	}
	for i := range fnsym.R {
		if fnsym.R[i].Type == objabi.R_USEIFACE && !strings.HasPrefix(fnsym.R[i].Sym.Name, "go:itab.") {
			// Types referenced through itab will be referenced from somewhere else
			typesyms = append(typesyms, fnsym.R[i].Sym)
		}
	}
	slices.SortFunc(typesyms, func(a, b *obj.LSym) int {
		return strings.Compare(a.Name, b.Name)
	})
	var lastsym *obj.LSym
	for _, sym := range typesyms {
		if sym == lastsym {
			continue
		}
		lastsym = sym
		infosym.AddRel(ctxt, obj.Reloc{Type: objabi.R_USETYPE, Sym: sym})
	}
	fnsym.Func().Autot = nil

	var varScopes []ir.ScopeID
	for _, decl := range decls {
		pos := declPos(decl)
		varScopes = append(varScopes, findScope(fn.Marks, pos))
	}

	scopes = assembleScopes(fnsym, fn, dwarfVars, varScopes)
	if base.Flag.GenDwarfInl > 0 {
		inlcalls = assembleInlines(fnsym, dwarfVars)
	}
	return scopes, inlcalls
}

func declPos(decl *ir.Name) src.XPos {
	return decl.Canonical().Pos()
}

// createDwarfVars process fn, returning a list of DWARF variables and the
// Nodes they represent.
func createDwarfVars(fnsym *obj.LSym, complexOK bool, fn *ir.Func, apDecls []*ir.Name, closureVars map[*ir.Name]int64) ([]*ir.Name, []*dwarf.Var) {
	// Collect a raw list of DWARF vars.
	var vars []*dwarf.Var
	var decls []*ir.Name

	// Build a VarID lookup map for SSA debug info if available.
	var debug *ssa.FuncDebug
	var varIDMap map[*ir.Name]ssa.VarID
	if fn.DebugInfo != nil {
		debug = fn.DebugInfo.(*ssa.FuncDebug)
		varIDMap = make(map[*ir.Name]ssa.VarID, len(debug.Vars))
		for i, n := range debug.Vars {
			varIDMap[n] = ssa.VarID(i)
		}
	}
	canUseComplex := complexOK && debug != nil

	// markVarSeen marks a variable and all its associated slot names as seen.
	// This is needed because decomposed variables may have slots whose ir.Name
	// differs from the variable itself (e.g., PAUTO vs PPARAMOUT for the same
	// logical variable). Without this, the dcl loop could create duplicate
	// conservative entries for names that are already covered by a complex var.
	seen := make(map[*ir.Name]bool)
	markVarSeen := func(n *ir.Name, varID ssa.VarID) {
		seen[n] = true
		if debug != nil && int(varID) < len(debug.VarSlots) {
			for _, slot := range debug.VarSlots[varID] {
				seen[debug.Slots[slot].N] = true
			}
		}
	}

	// Unified loop: for each variable in apDecls, try createComplexVar
	// (SSA debug info) first, then fall back to createSimpleVar.
	for _, n := range apDecls {
		if !shouldEmitDwarfVar(n) {
			continue
		}
		if canUseComplex {
			if vid, ok := varIDMap[n]; ok {
				if dvar := createComplexVar(fnsym, fn, vid, closureVars); dvar != nil {
					decls = append(decls, n)
					vars = append(vars, dvar)
					markVarSeen(n, vid)
					continue
				}
			}
		}
		seen[n] = true
		decls = append(decls, n)
		vars = append(vars, createSimpleVar(fnsym, n, closureVars))
	}

	// Add SSA-tracked vars not in apDecls.
	if canUseComplex {
		for i, n := range debug.Vars {
			if seen[n] {
				continue
			}
			if !shouldEmitDwarfVar(n) {
				continue
			}
			if dvar := createComplexVar(fnsym, fn, ssa.VarID(i), closureVars); dvar != nil {
				decls = append(decls, n)
				vars = append(vars, dvar)
				markVarSeen(n, ssa.VarID(i))
			}
		}
	}

	// Recover zero-sized variables eliminated by the stackframe pass.
	if debug != nil {
		for _, n := range debug.OptDcl {
			if seen[n] {
				continue
			}
			if n.Class != ir.PAUTO {
				continue
			}
			types.CalcSize(n.Type())
			if n.Type().Size() == 0 {
				decls = append(decls, n)
				vars = append(vars, createSimpleVar(fnsym, n, closureVars))
				vars[len(vars)-1].StackOffset = 0
				fnsym.Func().RecordAutoType(reflectdata.TypeLinksym(n.Type()))
				seen[n] = true
			}
		}
	}

	// For inlined functions or functions with register output params,
	// collect additional declarations that may not be in apDecls.
	dcl := apDecls
	if fnsym.WasInlined() {
		dcl = preInliningDcls(fnsym)
	} else if debug != nil {
		// The backend's stackframe pass prunes away entries from the
		// fn's Dcl list, including PARAMOUT nodes that correspond to
		// output params passed in registers. Add back in these
		// entries here so that we can process them properly during
		// DWARF-gen. See issue 48573 for more details.
		for _, n := range debug.RegOutputParams {
			if !ssa.IsVarWantedForDebug(n) {
				continue
			}
			if n.Class != ir.PPARAMOUT || !n.IsOutputParamInRegisters() {
				base.Fatalf("invalid ir.Name on debugInfo.RegOutputParams list")
			}
			dcl = append(dcl, n)
		}
	}

	// Process remaining variables not yet handled. For each variable,
	// try createComplexVar first, then fall back to createSimpleVar
	// for non-SSA-able params, or createConservativeVar for the rest.
	for _, n := range dcl {
		if seen[n] {
			continue
		}
		if !shouldEmitDwarfVar(n) {
			continue
		}
		seen[n] = true
		if canUseComplex {
			if vid, ok := varIDMap[n]; ok {
				if dvar := createComplexVar(fnsym, fn, vid, closureVars); dvar != nil {
					decls = append(decls, n)
					vars = append(vars, dvar)
					continue
				}
			}
		}
		if n.Class == ir.PPARAM && !ssa.CanSSA(n.Type()) {
			decls = append(decls, n)
			vars = append(vars, createSimpleVar(fnsym, n, closureVars))
			continue
		}
		decls = append(decls, n)
		vars = append(vars, createConservativeVar(fnsym, fn, n, closureVars))
	}

	// Sort decls and vars.
	sortDeclsAndVars(fn, decls, vars)

	return decls, vars
}

// createConservativeVar creates a DWARF variable with a conservative location
// description. This is used for variables that were optimized away or otherwise
// don't have precise location info. The intent is to communicate that "yes,
// there is a variable named X in this function, but no, I don't have enough
// information to reliably report its contents."
// For heap-escaped variables, a location list is created that describes
// dereferencing the pointer at the stack offset.
func createConservativeVar(fnsym *obj.LSym, fn *ir.Func, n *ir.Name, closureVars map[*ir.Name]int64) *dwarf.Var {
	typename := dwarf.InfoPrefix + types.TypeSymName(n.Type())
	tag := dwarf.DW_TAG_variable
	isReturnValue := (n.Class == ir.PPARAMOUT)
	if n.Class == ir.PPARAM || n.Class == ir.PPARAMOUT {
		tag = dwarf.DW_TAG_formal_parameter
	}
	inlIndex := 0
	if base.Flag.GenDwarfInl > 1 {
		if n.InlFormal() || n.InlLocal() {
			inlIndex = posInlIndex(n.Pos()) + 1
			if n.InlFormal() {
				tag = dwarf.DW_TAG_formal_parameter
			}
		}
	}
	declpos := base.Ctxt.InnermostPos(n.Pos())
	dvar := &dwarf.Var{
		Name:          n.Sym().Name,
		IsReturnValue: isReturnValue,
		Tag:           tag,
		WithLoclist:   true,
		StackOffset:   int32(n.FrameOffset()),
		Type:          base.Ctxt.Lookup(typename),
		DeclFile:      declpos.RelFilename(),
		DeclLine:      declpos.RelLine(),
		DeclCol:       declpos.RelCol(),
		InlIndex:      int32(inlIndex),
		ChildIndex:    -1,
		DictIndex:     n.DictIndex,
		ClosureOffset: closureOffset(n, closureVars),
	}
	if n.Esc() == ir.EscHeap && n.Heapaddr != nil {
		// The variable was promoted to the heap and has a known heap
		// address, so describe its location by dereferencing the pointer
		// stored at its stack offset. A heap-escaped variable may have no
		// Heapaddr if it was declared in unreachable code: escape analysis
		// marks it as heap-allocated, but SSA generation skips the dead
		// declaration and never allocates the address. In that case fall
		// through and emit a conservative variable with no location list.
		debug := fn.DebugInfo.(*ssa.FuncDebug)
		list := createHeapDerefLocationList(n, debug.EntryID)
		dvar.PutLocationList = func(listSym, startPC dwarf.Sym) {
			debug.PutLocationList(list, base.Ctxt, listSym.(*obj.LSym), startPC.(*obj.LSym))
		}
	}
	// Record go type to ensure that it gets emitted by the linker.
	fnsym.Func().RecordAutoType(reflectdata.TypeLinksym(n.Type()))
	return dvar
}

// sortDeclsAndVars sorts the decl and dwarf var lists according to
// parameter declaration order, so as to insure that when a subprogram
// DIE is emitted, its parameter children appear in declaration order.
// Prior to the advent of the register ABI, sorting by frame offset
// would achieve this; with the register we now need to go back to the
// original function signature.
func sortDeclsAndVars(fn *ir.Func, decls []*ir.Name, vars []*dwarf.Var) {
	paramOrder := make(map[*ir.Name]int)
	idx := 1
	for _, f := range fn.Type().RecvParamsResults() {
		if n, ok := f.Nname.(*ir.Name); ok {
			paramOrder[n] = idx
			idx++
		}
	}
	sort.Stable(varsAndDecls{decls, vars, paramOrder})
}

type varsAndDecls struct {
	decls      []*ir.Name
	vars       []*dwarf.Var
	paramOrder map[*ir.Name]int
}

func (v varsAndDecls) Len() int {
	return len(v.decls)
}

func (v varsAndDecls) Less(i, j int) bool {
	nameLT := func(ni, nj *ir.Name) bool {
		oi, foundi := v.paramOrder[ni]
		oj, foundj := v.paramOrder[nj]
		if foundi {
			if foundj {
				return oi < oj
			} else {
				return true
			}
		}
		return false
	}
	return nameLT(v.decls[i], v.decls[j])
}

func (v varsAndDecls) Swap(i, j int) {
	v.vars[i], v.vars[j] = v.vars[j], v.vars[i]
	v.decls[i], v.decls[j] = v.decls[j], v.decls[i]
}

// Given a function that was inlined at some point during the
// compilation, return a sorted list of nodes corresponding to the
// autos/locals in that function prior to inlining. If this is a
// function that is not local to the package being compiled, then the
// names of the variables may have been "versioned" to avoid conflicts
// with local vars; disregard this versioning when sorting.
func preInliningDcls(fnsym *obj.LSym) []*ir.Name {
	fn := base.Ctxt.DwFixups.GetPrecursorFunc(fnsym).(*ir.Func)
	var rdcl []*ir.Name
	for _, n := range fn.Inl.Dcl {
		if n.Sym().Name[0] == '.' || !shouldEmitDwarfVarSafe(n) {
			continue
		}
		rdcl = append(rdcl, n)
	}
	return rdcl
}

func createSimpleVar(fnsym *obj.LSym, n *ir.Name, closureVars map[*ir.Name]int64) *dwarf.Var {
	var tag int
	var offs int64

	localAutoOffset := func() int64 {
		offs = n.FrameOffset()
		if base.Ctxt.Arch.FixedFrameSize == 0 {
			offs -= int64(types.PtrSize)
		}
		if buildcfg.FramePointerEnabled {
			offs -= int64(types.PtrSize)
		}
		return offs
	}

	switch n.Class {
	case ir.PAUTO:
		offs = localAutoOffset()
		tag = dwarf.DW_TAG_variable
	case ir.PPARAM, ir.PPARAMOUT:
		tag = dwarf.DW_TAG_formal_parameter
		if n.IsOutputParamInRegisters() {
			offs = localAutoOffset()
		} else {
			offs = n.FrameOffset() + base.Ctxt.Arch.FixedFrameSize
		}

	default:
		base.Fatalf("createSimpleVar unexpected class %v for node %v", n.Class, n)
	}

	typename := dwarf.InfoPrefix + types.TypeSymName(n.Type())
	delete(fnsym.Func().Autot, reflectdata.TypeLinksym(n.Type()))
	inlIndex := 0
	if base.Flag.GenDwarfInl > 1 {
		if n.InlFormal() || n.InlLocal() {
			inlIndex = posInlIndex(n.Pos()) + 1
			if n.InlFormal() {
				tag = dwarf.DW_TAG_formal_parameter
			}
		}
	}
	declpos := base.Ctxt.InnermostPos(declPos(n))
	return &dwarf.Var{
		Name:          n.Sym().Name,
		IsReturnValue: n.Class == ir.PPARAMOUT,
		IsInlFormal:   n.InlFormal(),
		Tag:           tag,
		StackOffset:   int32(offs),
		Type:          base.Ctxt.Lookup(typename),
		DeclFile:      declpos.RelFilename(),
		DeclLine:      declpos.RelLine(),
		DeclCol:       declpos.RelCol(),
		InlIndex:      int32(inlIndex),
		ChildIndex:    -1,
		DictIndex:     n.DictIndex,
		ClosureOffset: closureOffset(n, closureVars),
	}
}

// createComplexVar builds a single DWARF variable entry and location list.
func createComplexVar(fnsym *obj.LSym, fn *ir.Func, varID ssa.VarID, closureVars map[*ir.Name]int64) *dwarf.Var {
	debug := fn.DebugInfo.(*ssa.FuncDebug)
	n := debug.Vars[varID]

	var tag int
	switch n.Class {
	case ir.PAUTO:
		tag = dwarf.DW_TAG_variable
	case ir.PPARAM, ir.PPARAMOUT:
		tag = dwarf.DW_TAG_formal_parameter
	default:
		return nil
	}

	gotype := reflectdata.TypeLinksym(n.Type())
	delete(fnsym.Func().Autot, gotype)
	typename := dwarf.InfoPrefix + gotype.Name[len("type:"):]
	inlIndex := 0
	if base.Flag.GenDwarfInl > 1 {
		if n.InlFormal() || n.InlLocal() {
			inlIndex = posInlIndex(n.Pos()) + 1
			if n.InlFormal() {
				tag = dwarf.DW_TAG_formal_parameter
			}
		}
	}
	declpos := base.Ctxt.InnermostPos(n.Pos())
	dvar := &dwarf.Var{
		Name:          n.Sym().Name,
		IsReturnValue: n.Class == ir.PPARAMOUT,
		IsInlFormal:   n.InlFormal(),
		Tag:           tag,
		WithLoclist:   true,
		Type:          base.Ctxt.Lookup(typename),
		// The stack offset is used as a sorting key, so for decomposed
		// variables just give it the first one. It's not used otherwise.
		// This won't work well if the first slot hasn't been assigned a stack
		// location, but it's not obvious how to do better.
		StackOffset:   ssagen.StackOffset(debug.Slots[debug.VarSlots[varID][0]]),
		DeclFile:      declpos.RelFilename(),
		DeclLine:      declpos.RelLine(),
		DeclCol:       declpos.RelCol(),
		InlIndex:      int32(inlIndex),
		ChildIndex:    -1,
		DictIndex:     n.DictIndex,
		ClosureOffset: closureOffset(n, closureVars),
	}
	list := debug.LocationLists[varID]
	if len(list) != 0 {
		dvar.PutLocationList = func(listSym, startPC dwarf.Sym) {
			debug.PutLocationList(list, base.Ctxt, listSym.(*obj.LSym), startPC.(*obj.LSym))
		}
	}
	return dvar
}

// createHeapDerefLocationList creates a location list for a heap-escaped variable
// that describes "dereference pointer at stack offset"
func createHeapDerefLocationList(n *ir.Name, entryID ssa.ID) []ssa.LocListEntry {
	// Get the stack offset where the heap pointer is stored
	heapPtrOffset := n.Heapaddr.FrameOffset()
	if base.Ctxt.Arch.FixedFrameSize == 0 {
		heapPtrOffset -= int64(types.PtrSize)
	}
	if buildcfg.FramePointerEnabled {
		heapPtrOffset -= int64(types.PtrSize)
	}

	// Create a location expression: DW_OP_fbreg <offset> DW_OP_deref
	var expr []byte
	expr = append(expr, dwarf.DW_OP_fbreg)
	expr = dwarf.AppendSleb128(expr, heapPtrOffset)
	expr = append(expr, dwarf.DW_OP_deref)

	return []ssa.LocListEntry{{
		StartBlock: entryID,
		StartValue: ssa.BlockStart.ID,
		EndBlock:   entryID,
		EndValue:   ssa.FuncEnd.ID,
		Expr:       expr,
	}}
}

// RecordFlags records the specified command-line flags to be placed
// in the DWARF info.
func RecordFlags(flags ...string) {
	if base.Ctxt.Pkgpath == "" {
		base.Fatalf("missing pkgpath")
	}

	type BoolFlag interface {
		IsBoolFlag() bool
	}
	type CountFlag interface {
		IsCountFlag() bool
	}
	var cmd bytes.Buffer
	for _, name := range flags {
		f := flag.Lookup(name)
		if f == nil {
			continue
		}
		getter := f.Value.(flag.Getter)
		if getter.String() == f.DefValue {
			// Flag has default value, so omit it.
			continue
		}
		if bf, ok := f.Value.(BoolFlag); ok && bf.IsBoolFlag() {
			val, ok := getter.Get().(bool)
			if ok && val {
				fmt.Fprintf(&cmd, " -%s", f.Name)
				continue
			}
		}
		if cf, ok := f.Value.(CountFlag); ok && cf.IsCountFlag() {
			val, ok := getter.Get().(int)
			if ok && val == 1 {
				fmt.Fprintf(&cmd, " -%s", f.Name)
				continue
			}
		}
		fmt.Fprintf(&cmd, " -%s=%v", f.Name, getter.Get())
	}

	// Adds flag to producer string signaling whether regabi is turned on or
	// off.
	// Once regabi is turned on across the board and the relative GOEXPERIMENT
	// knobs no longer exist this code should be removed.
	if buildcfg.Experiment.RegabiArgs {
		cmd.Write([]byte(" regabi"))
	}

	if cmd.Len() == 0 {
		return
	}
	s := base.Ctxt.Lookup(dwarf.CUInfoPrefix + "producer." + base.Ctxt.Pkgpath)
	s.Type = objabi.SDWARFCUINFO
	// Sometimes (for example when building tests) we can link
	// together two package main archives. So allow dups.
	s.Set(obj.AttrDuplicateOK, true)
	base.Ctxt.Data = append(base.Ctxt.Data, s)
	s.P = cmd.Bytes()[1:]
}

// RecordPackageName records the name of the package being
// compiled, so that the linker can save it in the compile unit's DIE.
func RecordPackageName() {
	s := base.Ctxt.Lookup(dwarf.CUInfoPrefix + "packagename." + base.Ctxt.Pkgpath)
	s.Type = objabi.SDWARFCUINFO
	// Sometimes (for example when building tests) we can link
	// together two package main archives. So allow dups.
	s.Set(obj.AttrDuplicateOK, true)
	base.Ctxt.Data = append(base.Ctxt.Data, s)
	s.P = []byte(types.LocalPkg.Name)
}

// shouldEmitDwarfVar reports whether n should have a DWARF variable entry.
// This consolidates filtering that was previously spread across IR (AutoTemp),
// SSA (IsVarWantedForDebug), and dwarfgen (symbol name checks).
func shouldEmitDwarfVar(n *ir.Name) bool {
	if ir.IsAutoTmp(n) {
		return false
	}
	return shouldEmitDwarfVarSafe(n)
}

// shouldEmitDwarfVarSafe is like shouldEmitDwarfVar but omits the ir.IsAutoTmp
// check, making it safe to call during parallel compilation on shared ir.Name
// nodes (e.g., in preInliningDcls). ir.IsAutoTmp reads the mutable flags bitset,
// which can race with other goroutines writing different flags during compilation.
// Auto temps have names starting with "." so callers must filter those separately.
func shouldEmitDwarfVarSafe(n *ir.Name) bool {
	if !ssa.IsVarWantedForDebug(n) {
		return false
	}
	if n.Sym().Name == "_" {
		return false
	}
	if n.Type().IsUntyped() {
		return false
	}
	return true
}

func closureOffset(n *ir.Name, closureVars map[*ir.Name]int64) int64 {
	return closureVars[n]
}

```

// === FILE: references/go/src/cmd/compile/internal/dwarfgen/dwinl.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dwarfgen

import (
	"fmt"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/internal/dwarf"
	"cmd/internal/obj"
	"cmd/internal/src"
)

// To identify variables by original source position.
type varPos struct {
	DeclName string
	DeclFile string
	DeclLine uint
	DeclCol  uint
}

// This is the main entry point for collection of raw material to
// drive generation of DWARF "inlined subroutine" DIEs. See proposal
// 22080 for more details and background info.
func assembleInlines(fnsym *obj.LSym, dwVars []*dwarf.Var) dwarf.InlCalls {
	var inlcalls dwarf.InlCalls

	if base.Debug.DwarfInl != 0 {
		base.Ctxt.Logf("assembling DWARF inlined routine info for %v\n", fnsym.Name)
	}

	// This maps inline index (from Ctxt.InlTree) to index in inlcalls.Calls
	imap := make(map[int]int)

	// Walk progs to build up the InlCalls data structure
	var prevpos src.XPos
	for p := fnsym.Func().Text; p != nil; p = p.Link {
		if p.Pos == prevpos {
			continue
		}
		ii := posInlIndex(p.Pos)
		if ii >= 0 {
			insertInlCall(&inlcalls, ii, imap)
		}
		prevpos = p.Pos
	}

	// This is used to partition DWARF vars by inline index. Vars not
	// produced by the inliner will wind up in the vmap[0] entry.
	vmap := make(map[int32][]*dwarf.Var)

	// Now walk the dwarf vars and partition them based on whether they
	// were produced by the inliner (dwv.InlIndex > 0) or were original
	// vars/params from the function (dwv.InlIndex == 0).
	for _, dwv := range dwVars {

		vmap[dwv.InlIndex] = append(vmap[dwv.InlIndex], dwv)

		// Zero index => var was not produced by an inline
		if dwv.InlIndex == 0 {
			continue
		}

		// Look up index in our map, then tack the var in question
		// onto the vars list for the correct inlined call.
		ii := int(dwv.InlIndex) - 1
		idx, ok := imap[ii]
		if !ok {
			// We can occasionally encounter a var produced by the
			// inliner for which there is no remaining prog; add a new
			// entry to the call list in this scenario.
			idx = insertInlCall(&inlcalls, ii, imap)
		}
		inlcalls.Calls[idx].InlVars =
			append(inlcalls.Calls[idx].InlVars, dwv)
	}

	// Post process the map above to assign child indices to vars.
	//
	// A given variable is treated differently depending on whether it
	// is part of the top-level function (ii == 0) or if it was
	// produced as a result of an inline (ii != 0).
	//
	// If a variable was not produced by an inline and its containing
	// function was not inlined, then we just assign an ordering of
	// based on variable name.
	//
	// If a variable was not produced by an inline and its containing
	// function was inlined, then we need to assign a child index
	// based on the order of vars in the abstract function (in
	// addition, those vars that don't appear in the abstract
	// function, such as "~r1", are flagged as such).
	//
	// If a variable was produced by an inline, then we locate it in
	// the pre-inlining decls for the target function and assign child
	// index accordingly.
	for ii, sl := range vmap {
		var m map[varPos]int
		if ii == 0 {
			if !fnsym.WasInlined() {
				for j, v := range sl {
					v.ChildIndex = int32(j)
				}
				continue
			}
			m = makePreinlineDclMap(fnsym)
		} else {
			ifnlsym := base.Ctxt.InlTree.InlinedFunction(int(ii - 1))
			m = makePreinlineDclMap(ifnlsym)
		}

		// Here we assign child indices to variables based on
		// pre-inlined decls, and set the "IsInAbstract" flag
		// appropriately. In addition: parameter and local variable
		// names are given "middle dot" version numbers as part of the
		// writing them out to export data (see issue 4326). If DWARF
		// inlined routine generation is turned on, we want to undo
		// this versioning, since DWARF variables in question will be
		// parented by the inlined routine and not the top-level
		// caller.
		synthCount := len(m)
		for _, v := range sl {
			vp := varPos{
				DeclName: v.Name,
				DeclFile: v.DeclFile,
				DeclLine: v.DeclLine,
				DeclCol:  v.DeclCol,
			}
			synthesized := strings.HasPrefix(v.Name, "~") || v.Name == "_"
			if idx, found := m[vp]; found {
				v.ChildIndex = int32(idx)
				v.IsInAbstract = !synthesized
			} else {
				// Variable can't be found in the pre-inline dcl list.
				// In the top-level case (ii=0) this can happen
				// because a composite variable was split into pieces,
				// and we're looking at a piece. We can also see
				// return temps (~r%d) that were created during
				// lowering, or unnamed params ("_").
				v.ChildIndex = int32(synthCount)
				synthCount++
			}
		}
	}

	// Make a second pass through the progs to compute PC ranges for
	// the various inlined calls.
	start := int64(-1)
	curii := -1
	var prevp *obj.Prog
	for p := fnsym.Func().Text; p != nil; prevp, p = p, p.Link {
		if prevp != nil && p.Pos == prevp.Pos {
			continue
		}
		ii := posInlIndex(p.Pos)
		if ii == curii {
			continue
		}
		// Close out the current range
		if start != -1 {
			addRange(inlcalls.Calls, start, p.Pc, curii, imap)
		}
		// Begin new range
		start = p.Pc
		curii = ii
	}
	if start != -1 {
		addRange(inlcalls.Calls, start, fnsym.Size, curii, imap)
	}

	// Issue 33188: if II foo is a child of II bar, then ensure that
	// bar's ranges include the ranges of foo (the loop above will produce
	// disjoint ranges).
	for k, c := range inlcalls.Calls {
		if c.Root {
			unifyCallRanges(inlcalls, k)
		}
	}

	// Debugging
	if base.Debug.DwarfInl != 0 {
		dumpInlCalls(inlcalls)
		dumpInlVars(dwVars)
	}

	// Perform a consistency check on inlined routine PC ranges
	// produced by unifyCallRanges above. In particular, complain in
	// cases where you have A -> B -> C (e.g. C is inlined into B, and
	// B is inlined into A) and the ranges for B are not enclosed
	// within the ranges for A, or C within B.
	for k, c := range inlcalls.Calls {
		if c.Root {
			checkInlCall(fnsym.Name, inlcalls, fnsym.Size, k, -1)
		}
	}

	return inlcalls
}

// Secondary hook for DWARF inlined subroutine generation. This is called
// late in the compilation when it is determined that we need an
// abstract function DIE for an inlined routine imported from a
// previously compiled package.
func AbstractFunc(fn *obj.LSym) {
	ifn := base.Ctxt.DwFixups.GetPrecursorFunc(fn)
	if ifn == nil {
		base.Ctxt.Diag("failed to locate precursor fn for %v", fn)
		return
	}
	_ = ifn.(*ir.Func)
	if base.Debug.DwarfInl != 0 {
		base.Ctxt.Logf("DwarfAbstractFunc(%v)\n", fn.Name)
	}
	base.Ctxt.DwarfAbstractFunc(ifn, fn)
}

// Given a function that was inlined as part of the compilation, dig
// up the pre-inlining DCL list for the function and create a map that
// supports lookup of pre-inline dcl index, based on variable
// position/name. NB: the recipe for computing variable pos/file/line
// needs to be kept in sync with the similar code in gc.createSimpleVars
// and related functions.
func makePreinlineDclMap(fnsym *obj.LSym) map[varPos]int {
	dcl := preInliningDcls(fnsym)
	m := make(map[varPos]int)
	for i, n := range dcl {
		pos := base.Ctxt.InnermostPos(n.Pos())
		vp := varPos{
			DeclName: n.Sym().Name,
			DeclFile: pos.RelFilename(),
			DeclLine: pos.RelLine(),
			DeclCol:  pos.RelCol(),
		}
		if _, found := m[vp]; found {
			// We can see collisions (variables with the same name/file/line/col) in obfuscated or machine-generated code -- see issue 44378 for an example. Skip duplicates in such cases, since it is unlikely that a human will be debugging such code.
			continue
		}
		m[vp] = i
	}
	return m
}

func insertInlCall(dwcalls *dwarf.InlCalls, inlIdx int, imap map[int]int) int {
	callIdx, found := imap[inlIdx]
	if found {
		return callIdx
	}

	// Haven't seen this inline yet. Visit parent of inline if there
	// is one. We do this first so that parents appear before their
	// children in the resulting table.
	parCallIdx := -1
	parInlIdx := base.Ctxt.InlTree.Parent(inlIdx)
	if parInlIdx >= 0 {
		parCallIdx = insertInlCall(dwcalls, parInlIdx, imap)
	}

	// Create new entry for this inline
	inlinedFn := base.Ctxt.InlTree.InlinedFunction(inlIdx)
	callXPos := base.Ctxt.InlTree.CallPos(inlIdx)
	callPos := base.Ctxt.InnermostPos(callXPos)
	absFnSym := base.Ctxt.DwFixups.AbsFuncDwarfSym(inlinedFn)
	ic := dwarf.InlCall{
		InlIndex:  inlIdx,
		CallPos:   callPos,
		AbsFunSym: absFnSym,
		Root:      parCallIdx == -1,
	}
	dwcalls.Calls = append(dwcalls.Calls, ic)
	callIdx = len(dwcalls.Calls) - 1
	imap[inlIdx] = callIdx

	if parCallIdx != -1 {
		// Add this inline to parent's child list
		dwcalls.Calls[parCallIdx].Children = append(dwcalls.Calls[parCallIdx].Children, callIdx)
	}

	return callIdx
}

// Given a src.XPos, return its associated inlining index if it
// corresponds to something created as a result of an inline, or -1 if
// there is no inline info. Note that the index returned will refer to
// the deepest call in the inlined stack, e.g. if you have "A calls B
// calls C calls D" and all three callees are inlined (B, C, and D),
// the index for a node from the inlined body of D will refer to the
// call to D from C. Whew.
func posInlIndex(xpos src.XPos) int {
	pos := base.Ctxt.PosTable.Pos(xpos)
	if b := pos.Base(); b != nil {
		ii := b.InliningIndex()
		if ii >= 0 {
			return ii
		}
	}
	return -1
}

func addRange(calls []dwarf.InlCall, start, end int64, ii int, imap map[int]int) {
	if start == -1 {
		panic("bad range start")
	}
	if end == -1 {
		panic("bad range end")
	}
	if ii == -1 {
		return
	}
	if start == end {
		return
	}
	// Append range to correct inlined call
	callIdx, found := imap[ii]
	if !found {
		base.Fatalf("can't find inlIndex %d in imap for prog at %d\n", ii, start)
	}
	call := &calls[callIdx]
	call.Ranges = append(call.Ranges, dwarf.Range{Start: start, End: end})
}

func dumpInlCall(inlcalls dwarf.InlCalls, idx, ilevel int) {
	for i := 0; i < ilevel; i++ {
		base.Ctxt.Logf("  ")
	}
	ic := inlcalls.Calls[idx]
	callee := base.Ctxt.InlTree.InlinedFunction(ic.InlIndex)
	base.Ctxt.Logf("  %d: II:%d (%s) V: (", idx, ic.InlIndex, callee.Name)
	for _, f := range ic.InlVars {
		base.Ctxt.Logf(" %v", f.Name)
	}
	base.Ctxt.Logf(" ) C: (")
	for _, k := range ic.Children {
		base.Ctxt.Logf(" %v", k)
	}
	base.Ctxt.Logf(" ) R:")
	for _, r := range ic.Ranges {
		base.Ctxt.Logf(" [%d,%d)", r.Start, r.End)
	}
	base.Ctxt.Logf("\n")
	for _, k := range ic.Children {
		dumpInlCall(inlcalls, k, ilevel+1)
	}

}

func dumpInlCalls(inlcalls dwarf.InlCalls) {
	for k, c := range inlcalls.Calls {
		if c.Root {
			dumpInlCall(inlcalls, k, 0)
		}
	}
}

func dumpInlVars(dwvars []*dwarf.Var) {
	for i, dwv := range dwvars {
		typ := "local"
		if dwv.Tag == dwarf.DW_TAG_formal_parameter {
			typ = "param"
		}
		ia := 0
		if dwv.IsInAbstract {
			ia = 1
		}
		base.Ctxt.Logf("V%d: %s CI:%d II:%d IA:%d %s\n", i, dwv.Name, dwv.ChildIndex, dwv.InlIndex-1, ia, typ)
	}
}

func rangesContains(par []dwarf.Range, rng dwarf.Range) (bool, string) {
	for _, r := range par {
		if rng.Start >= r.Start && rng.End <= r.End {
			return true, ""
		}
	}
	msg := fmt.Sprintf("range [%d,%d) not contained in {", rng.Start, rng.End)
	for _, r := range par {
		msg += fmt.Sprintf(" [%d,%d)", r.Start, r.End)
	}
	msg += " }"
	return false, msg
}

func rangesContainsAll(parent, child []dwarf.Range) (bool, string) {
	for _, r := range child {
		c, m := rangesContains(parent, r)
		if !c {
			return false, m
		}
	}
	return true, ""
}

// checkInlCall verifies that the PC ranges for inline info 'idx' are
// enclosed/contained within the ranges of its parent inline (or if
// this is a root/toplevel inline, checks that the ranges fall within
// the extent of the top level function). A panic is issued if a
// malformed range is found.
func checkInlCall(funcName string, inlCalls dwarf.InlCalls, funcSize int64, idx, parentIdx int) {

	// Callee
	ic := inlCalls.Calls[idx]
	callee := base.Ctxt.InlTree.InlinedFunction(ic.InlIndex).Name
	calleeRanges := ic.Ranges

	// Caller
	caller := funcName
	parentRanges := []dwarf.Range{dwarf.Range{Start: int64(0), End: funcSize}}
	if parentIdx != -1 {
		pic := inlCalls.Calls[parentIdx]
		caller = base.Ctxt.InlTree.InlinedFunction(pic.InlIndex).Name
		parentRanges = pic.Ranges
	}

	// Callee ranges contained in caller ranges?
	c, m := rangesContainsAll(parentRanges, calleeRanges)
	if !c {
		base.Fatalf("** malformed inlined routine range in %s: caller %s callee %s II=%d %s\n", funcName, caller, callee, idx, m)
	}

	// Now visit kids
	for _, k := range ic.Children {
		checkInlCall(funcName, inlCalls, funcSize, k, idx)
	}
}

// unifyCallRanges ensures that the ranges for a given inline
// transitively include all of the ranges for its child inlines.
func unifyCallRanges(inlcalls dwarf.InlCalls, idx int) {
	ic := &inlcalls.Calls[idx]
	for _, childIdx := range ic.Children {
		// First make sure child ranges are unified.
		unifyCallRanges(inlcalls, childIdx)

		// Then merge child ranges into ranges for this inline.
		cic := inlcalls.Calls[childIdx]
		ic.Ranges = dwarf.MergeRanges(ic.Ranges, cic.Ranges)
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/dwarfgen/marker.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dwarfgen

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/internal/src"
)

// A ScopeMarker tracks scope nesting and boundaries for later use
// during DWARF generation.
type ScopeMarker struct {
	parents []ir.ScopeID
	marks   []ir.Mark
}

// checkPos validates the given position and returns the current scope.
func (m *ScopeMarker) checkPos(pos src.XPos) ir.ScopeID {
	if !pos.IsKnown() {
		base.Fatalf("unknown scope position, pos=%v", pos)
	}

	if len(m.marks) == 0 {
		return 0
	}

	last := &m.marks[len(m.marks)-1]
	if xposBefore(pos, last.Pos) {
		base.FatalfAt(pos, "non-monotonic scope positions\n\t%v: previous scope position", base.FmtPos(last.Pos))
	}
	return last.Scope
}

// Push records a transition to a new child scope of the current scope.
func (m *ScopeMarker) Push(pos src.XPos) {
	current := m.checkPos(pos)

	m.parents = append(m.parents, current)
	child := ir.ScopeID(len(m.parents))

	m.marks = append(m.marks, ir.Mark{Pos: pos, Scope: child})
}

// Pop records a transition back to the current scope's parent.
func (m *ScopeMarker) Pop(pos src.XPos) {
	current := m.checkPos(pos)

	parent := m.parents[current-1]

	m.marks = append(m.marks, ir.Mark{Pos: pos, Scope: parent})
}

// Unpush removes the current scope, which must be empty.
func (m *ScopeMarker) Unpush() {
	i := len(m.marks) - 1
	current := m.marks[i].Scope

	if current != ir.ScopeID(len(m.parents)) {
		base.FatalfAt(m.marks[i].Pos, "current scope is not empty")
	}

	m.parents = m.parents[:current-1]
	m.marks = m.marks[:i]
}

// WriteTo writes the recorded scope marks to the given function,
// and resets the marker for reuse.
func (m *ScopeMarker) WriteTo(fn *ir.Func) {
	m.compactMarks()

	fn.Parents = make([]ir.ScopeID, len(m.parents))
	copy(fn.Parents, m.parents)
	m.parents = m.parents[:0]

	fn.Marks = make([]ir.Mark, len(m.marks))
	copy(fn.Marks, m.marks)
	m.marks = m.marks[:0]
}

func (m *ScopeMarker) compactMarks() {
	n := 0
	for _, next := range m.marks {
		if n > 0 && next.Pos == m.marks[n-1].Pos {
			m.marks[n-1].Scope = next.Scope
			continue
		}
		m.marks[n] = next
		n++
	}
	m.marks = m.marks[:n]
}

```

// === FILE: references/go/src/cmd/compile/internal/dwarfgen/scope.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dwarfgen

import (
	"sort"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/internal/dwarf"
	"cmd/internal/obj"
	"cmd/internal/src"
)

// See golang.org/issue/20390.
func xposBefore(p, q src.XPos) bool {
	return base.Ctxt.PosTable.Pos(p).Before(base.Ctxt.PosTable.Pos(q))
}

func findScope(marks []ir.Mark, pos src.XPos) ir.ScopeID {
	i := sort.Search(len(marks), func(i int) bool {
		return xposBefore(pos, marks[i].Pos)
	})
	if i == 0 {
		return 0
	}
	return marks[i-1].Scope
}

func assembleScopes(fnsym *obj.LSym, fn *ir.Func, dwarfVars []*dwarf.Var, varScopes []ir.ScopeID) []dwarf.Scope {
	// Initialize the DWARF scope tree based on lexical scopes.
	dwarfScopes := make([]dwarf.Scope, 1+len(fn.Parents))
	for i, parent := range fn.Parents {
		dwarfScopes[i+1].Parent = int32(parent)
	}

	scopeVariables(dwarfVars, varScopes, dwarfScopes, fnsym.ABI() != obj.ABI0)
	if fnsym.Func().Text != nil {
		scopePCs(fnsym, fn.Marks, dwarfScopes)
	}
	return compactScopes(dwarfScopes)
}

// scopeVariables assigns DWARF variable records to their scopes.
func scopeVariables(dwarfVars []*dwarf.Var, varScopes []ir.ScopeID, dwarfScopes []dwarf.Scope, regabi bool) {
	if regabi {
		sort.Stable(varsByScope{dwarfVars, varScopes})
	} else {
		sort.Stable(varsByScopeAndOffset{dwarfVars, varScopes})
	}

	i0 := 0
	for i := range dwarfVars {
		if varScopes[i] == varScopes[i0] {
			continue
		}
		dwarfScopes[varScopes[i0]].Vars = dwarfVars[i0:i]
		i0 = i
	}
	if i0 < len(dwarfVars) {
		dwarfScopes[varScopes[i0]].Vars = dwarfVars[i0:]
	}
}

// scopePCs assigns PC ranges to their scopes.
func scopePCs(fnsym *obj.LSym, marks []ir.Mark, dwarfScopes []dwarf.Scope) {
	// If there aren't any child scopes (in particular, when scope
	// tracking is disabled), we can skip a whole lot of work.
	if len(marks) == 0 {
		return
	}
	p0 := fnsym.Func().Text
	scope := findScope(marks, p0.Pos)
	for p := p0; p != nil; p = p.Link {
		if p.Pos == p0.Pos {
			continue
		}
		dwarfScopes[scope].AppendRange(dwarf.Range{Start: p0.Pc, End: p.Pc})
		p0 = p
		scope = findScope(marks, p0.Pos)
	}
	if p0.Pc < fnsym.Size {
		dwarfScopes[scope].AppendRange(dwarf.Range{Start: p0.Pc, End: fnsym.Size})
	}
}

func compactScopes(dwarfScopes []dwarf.Scope) []dwarf.Scope {
	// Reverse pass to propagate PC ranges to parent scopes.
	for i := len(dwarfScopes) - 1; i > 0; i-- {
		s := &dwarfScopes[i]
		dwarfScopes[s.Parent].UnifyRanges(s)
	}

	return dwarfScopes
}

type varsByScopeAndOffset struct {
	vars   []*dwarf.Var
	scopes []ir.ScopeID
}

func (v varsByScopeAndOffset) Len() int {
	return len(v.vars)
}

func (v varsByScopeAndOffset) Less(i, j int) bool {
	if v.scopes[i] != v.scopes[j] {
		return v.scopes[i] < v.scopes[j]
	}
	return v.vars[i].StackOffset < v.vars[j].StackOffset
}

func (v varsByScopeAndOffset) Swap(i, j int) {
	v.vars[i], v.vars[j] = v.vars[j], v.vars[i]
	v.scopes[i], v.scopes[j] = v.scopes[j], v.scopes[i]
}

type varsByScope struct {
	vars   []*dwarf.Var
	scopes []ir.ScopeID
}

func (v varsByScope) Len() int {
	return len(v.vars)
}

func (v varsByScope) Less(i, j int) bool {
	return v.scopes[i] < v.scopes[j]
}

func (v varsByScope) Swap(i, j int) {
	v.vars[i], v.vars[j] = v.vars[j], v.vars[i]
	v.scopes[i], v.scopes[j] = v.scopes[j], v.scopes[i]
}

```

