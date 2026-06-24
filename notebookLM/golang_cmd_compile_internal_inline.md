# Domain Architecture: cmd/compile/internal/inline

## Layout Topology
```text
cmd/compile/internal/inline/
├── inlheur
│   ├── actualexprpropbits_string.go
│   ├── analyze.go
│   ├── analyze_func_callsites.go
│   ├── analyze_func_flags.go
│   ├── analyze_func_params.go
│   ├── analyze_func_returns.go
│   ├── callsite.go
│   ├── cspropbits_string.go
│   ├── eclassify.go
│   ├── funcprop_string.go
│   ├── funcpropbits_string.go
│   ├── function_properties.go
│   ├── names.go
│   ├── parampropbits_string.go
│   ├── pstate_string.go
│   ├── resultpropbits_string.go
│   ├── score_callresult_uses.go
│   ├── scoreadjusttyp_string.go
│   ├── scoring.go
│   ├── serialize.go
│   ├── trace_off.go
│   └── trace_on.go
├── interleaved
│   └── interleaved.go
└── inl.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/compile/internal/inline/inl.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// The inlining facility makes 2 passes: first CanInline determines which
// functions are suitable for inlining, and for those that are it
// saves a copy of the body. Then InlineCalls walks each function body to
// expand calls to inlinable functions.
//
// The Debug.l flag controls the aggressiveness. Note that main() swaps level 0 and 1,
// making 1 the default and -l disable. Additional levels (beyond -l) may be buggy and
// are not supported.
//      0: disabled
//      1: 80-nodes leaf functions, oneliners, panic, lazy typechecking (default)
//      2: (unassigned)
//      3: (unassigned)
//      4: allow non-leaf functions
//
// At some point this may get another default and become switch-offable with -N.
//
// The -d typcheckinl flag enables early typechecking of all imported bodies,
// which is useful to flush out bugs.
//
// The Debug.m flag enables diagnostic output.  a single -m is useful for verifying
// which calls get inlined or not, more is for debugging, and may go away at any point.

package inline

import (
	"fmt"
	"go/constant"
	"internal/buildcfg"
	"strconv"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/inline/inlheur"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/logopt"
	"cmd/compile/internal/pgoir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/pgo"
	"cmd/internal/src"
)

// Inlining budget parameters, gathered in one place
const (
	inlineMaxBudget       = 80
	inlineExtraAppendCost = 0
	// default is to inline if there's at most one call. -l=4 overrides this by using 1 instead.
	inlineExtraCallCost  = 57              // 57 was benchmarked to provided most benefit with no bad surprises; see https://github.com/golang/go/issues/19348#issuecomment-439370742
	inlineParamCallCost  = 17              // calling a parameter only costs this much extra (inlining might expose a constant function)
	inlineExtraPanicCost = 1               // do not penalize inlining panics.
	inlineExtraThrowCost = inlineMaxBudget // with current (2018-05/1.11) code, inlining runtime.throw does not help.

	inlineBigFunctionNodes      = 5000                 // Functions with this many nodes are considered "big".
	inlineBigFunctionMaxCost    = 20                   // Max cost of inlinee when inlining into a "big" function.
	inlineClosureCalledOnceCost = 10 * inlineMaxBudget // if a closure is just called once, inline it.
)

var (
	// List of all hot callee nodes.
	// TODO(prattmic): Make this non-global.
	candHotCalleeMap = make(map[*pgoir.IRNode]struct{})

	// Set of functions that contain hot call sites.
	hasHotCall = make(map[*ir.Func]struct{})

	// List of all hot call sites. CallSiteInfo.Callee is always nil.
	// TODO(prattmic): Make this non-global.
	candHotEdgeMap = make(map[pgoir.CallSiteInfo]struct{})

	// Threshold in percentage for hot callsite inlining.
	inlineHotCallSiteThresholdPercent float64

	// Threshold in CDF percentage for hot callsite inlining,
	// that is, for a threshold of X the hottest callsites that
	// make up the top X% of total edge weight will be
	// considered hot for inlining candidates.
	inlineCDFHotCallSiteThresholdPercent = float64(99)

	// Budget increased due to hotness.
	inlineHotMaxBudget int32 = 2000
)

func IsPgoHotFunc(fn *ir.Func, profile *pgoir.Profile) bool {
	if profile == nil {
		return false
	}
	if n, ok := profile.WeightedCG.IRNodes[ir.LinkFuncName(fn)]; ok {
		_, ok := candHotCalleeMap[n]
		return ok
	}
	return false
}

func HasPgoHotInline(fn *ir.Func) bool {
	_, has := hasHotCall[fn]
	return has
}

// PGOInlinePrologue records the hot callsites from ir-graph.
func PGOInlinePrologue(p *pgoir.Profile) {
	if base.Debug.PGOInlineCDFThreshold != "" {
		if s, err := strconv.ParseFloat(base.Debug.PGOInlineCDFThreshold, 64); err == nil && s >= 0 && s <= 100 {
			inlineCDFHotCallSiteThresholdPercent = s
		} else {
			base.Fatalf("invalid PGOInlineCDFThreshold, must be between 0 and 100")
		}
	}
	var hotCallsites []pgo.NamedCallEdge
	inlineHotCallSiteThresholdPercent, hotCallsites = hotNodesFromCDF(p)
	if base.Debug.PGODebug > 0 {
		fmt.Printf("hot-callsite-thres-from-CDF=%v\n", inlineHotCallSiteThresholdPercent)
	}

	if x := base.Debug.PGOInlineBudget; x != 0 {
		inlineHotMaxBudget = int32(x)
	}

	for _, n := range hotCallsites {
		// mark inlineable callees from hot edges
		if callee := p.WeightedCG.IRNodes[n.CalleeName]; callee != nil {
			candHotCalleeMap[callee] = struct{}{}
		}
		// mark hot call sites
		if caller := p.WeightedCG.IRNodes[n.CallerName]; caller != nil && caller.AST != nil {
			csi := pgoir.CallSiteInfo{LineOffset: n.CallSiteOffset, Caller: caller.AST}
			candHotEdgeMap[csi] = struct{}{}
		}
	}

	if base.Debug.PGODebug >= 3 {
		fmt.Printf("hot-cg before inline in dot format:")
		p.PrintWeightedCallGraphDOT(inlineHotCallSiteThresholdPercent)
	}
}

// hotNodesFromCDF computes an edge weight threshold and the list of hot
// nodes that make up the given percentage of the CDF. The threshold, as
// a percent, is the lower bound of weight for nodes to be considered hot
// (currently only used in debug prints) (in case of equal weights,
// comparing with the threshold may not accurately reflect which nodes are
// considered hot).
func hotNodesFromCDF(p *pgoir.Profile) (float64, []pgo.NamedCallEdge) {
	cum := int64(0)
	for i, n := range p.NamedEdgeMap.ByWeight {
		w := p.NamedEdgeMap.Weight[n]
		cum += w
		if pgo.WeightInPercentage(cum, p.TotalWeight) > inlineCDFHotCallSiteThresholdPercent {
			// nodes[:i+1] to include the very last node that makes it to go over the threshold.
			// (Say, if the CDF threshold is 50% and one hot node takes 60% of weight, we want to
			// include that node instead of excluding it.)
			return pgo.WeightInPercentage(w, p.TotalWeight), p.NamedEdgeMap.ByWeight[:i+1]
		}
	}
	return 0, p.NamedEdgeMap.ByWeight
}

// CanInlineFuncs computes whether a batch of functions are inlinable.
func CanInlineFuncs(funcs []*ir.Func, profile *pgoir.Profile) {
	if profile != nil {
		PGOInlinePrologue(profile)
	}

	if base.Flag.LowerL == 0 {
		return
	}

	ir.VisitFuncsBottomUp(funcs, func(funcs []*ir.Func, recursive bool) {
		for _, fn := range funcs {
			CanInline(fn, profile)
			if inlheur.Enabled() {
				analyzeFuncProps(fn, profile)
			}
		}
	})
}

func simdCreditMultiplier(fn *ir.Func) int32 {
	for _, field := range fn.Type().RecvParamsResults() {
		if field.Type.IsSIMD() {
			return 3
		}
	}
	// Sometimes code uses closures, that do not take simd
	// parameters, to perform repetitive SIMD operations.
	// fn.  These really need to be inlined, or the anticipated
	// awesome SIMD performance will be missed.
	for _, v := range fn.ClosureVars {
		if v.Type().IsSIMD() {
			return 16 // <strike>11</strike> 16 ought to be enough.
		}
	}

	return 1
}

// inlineBudget determines the max budget for function 'fn' prior to
// analyzing the hairiness of the body of 'fn'. We pass in the pgo
// profile if available (which can change the budget), also a
// 'relaxed' flag, which expands the budget slightly to allow for the
// possibility that a call to the function might have its score
// adjusted downwards. If 'verbose' is set, then print a remark where
// we boost the budget due to PGO.
// Note that inlineCostOk has the final say on whether an inline will
// happen; changes here merely make inlines possible.
func inlineBudget(fn *ir.Func, profile *pgoir.Profile, relaxed bool, verbose bool) int32 {
	// Update the budget for profile-guided inlining.
	budget := int32(inlineMaxBudget)

	budget *= simdCreditMultiplier(fn)

	if IsPgoHotFunc(fn, profile) {
		budget = inlineHotMaxBudget
		if verbose {
			fmt.Printf("hot-node enabled increased budget=%v for func=%v\n", budget, ir.PkgFuncName(fn))
		}
	}
	if relaxed {
		budget += inlheur.BudgetExpansion(inlineMaxBudget)
	}
	if fn.ClosureParent != nil {
		// be very liberal here, if the closure is only called once, the budget is large
		budget = max(budget, inlineClosureCalledOnceCost)
	}

	return budget
}

// CanInline determines whether fn is inlineable.
// If so, CanInline saves copies of fn.Body and fn.Dcl in fn.Inl.
// fn and fn.Body will already have been typechecked.
func CanInline(fn *ir.Func, profile *pgoir.Profile) {
	if fn.Nname == nil {
		base.Fatalf("CanInline no nname %+v", fn)
	}

	var reason string // reason, if any, that the function was not inlined
	if base.Flag.LowerM > 1 || logopt.Enabled() {
		defer func() {
			if reason != "" {
				if base.Flag.LowerM > 1 {
					fmt.Printf("%v: cannot inline %v: %s\n", ir.Line(fn), fn.Nname, reason)
				}
				if logopt.Enabled() {
					logopt.LogOpt(fn.Pos(), "cannotInlineFunction", "inline", ir.FuncName(fn), reason)
				}
			}
		}()
	}

	reason = InlineImpossible(fn)
	if reason != "" {
		return
	}
	if fn.Typecheck() == 0 {
		base.Fatalf("CanInline on non-typechecked function %v", fn)
	}

	n := fn.Nname
	if n.Func.InlinabilityChecked() {
		return
	}
	defer n.Func.SetInlinabilityChecked(true)

	cc := int32(inlineExtraCallCost)
	if base.Flag.LowerL == 4 {
		cc = 1 // this appears to yield better performance than 0.
	}

	// Used a "relaxed" inline budget if the new inliner is enabled.
	relaxed := inlheur.Enabled()

	// Compute the inline budget for this func.
	budget := inlineBudget(fn, profile, relaxed, base.Debug.PGODebug > 0)

	// At this point in the game the function we're looking at may
	// have "stale" autos, vars that still appear in the Dcl list, but
	// which no longer have any uses in the function body (due to
	// elimination by deadcode). We'd like to exclude these dead vars
	// when creating the "Inline.Dcl" field below; to accomplish this,
	// the hairyVisitor below builds up a map of used/referenced
	// locals, and we use this map to produce a pruned Inline.Dcl
	// list. See issue 25459 for more context.

	dbg := ir.MatchAstDump(fn, "inline")

	visitor := hairyVisitor{
		curFunc:       fn,
		debug:         isDebugFn(fn),
		isBigFunc:     IsBigFunc(fn),
		budget:        budget,
		maxBudget:     budget,
		extraCallCost: cc,
		profile:       profile,
		dbg:           dbg, // Useful for downstream debugging
	}

	if visitor.tooHairy(fn) {
		reason = visitor.reason
		if dbg {
			ir.AstDump(fn, "inline, too hairy because "+visitor.reason+", "+ir.FuncName(fn))
		}
		return
	} else if dbg {
		ir.AstDump(fn, "inline, OK, "+ir.FuncName(fn))
	}

	n.Func.Inl = &ir.Inline{
		Cost:            budget - visitor.budget,
		Dcl:             pruneUnusedAutos(n.Func.Dcl, &visitor),
		HaveDcl:         true,
		CanDelayResults: canDelayResults(fn),
	}
	if base.Flag.LowerM != 0 || logopt.Enabled() {
		noteInlinableFunc(n, fn, budget-visitor.budget)
	}
}

// noteInlinableFunc issues a message to the user that the specified
// function is inlinable.
func noteInlinableFunc(n *ir.Name, fn *ir.Func, cost int32) {
	if base.Flag.LowerM > 1 {
		fmt.Printf("%v: can inline %v with cost %d as: %v { %v }\n", ir.Line(fn), n.DiagName(), cost, fn.Type(), fn.Body)
	} else if base.Flag.LowerM != 0 {
		fmt.Printf("%v: can inline %v\n", ir.Line(fn), n.DiagName())
	}
	// JSON optimization log output.
	if logopt.Enabled() {
		logopt.LogOpt(fn.Pos(), "canInlineFunction", "inline", ir.FuncName(fn), fmt.Sprintf("cost: %d", cost))
	}
}

// InlineImpossible returns a non-empty reason string if fn is impossible to
// inline regardless of cost or contents.
func InlineImpossible(fn *ir.Func) string {
	var reason string // reason, if any, that the function can not be inlined.
	if fn.Nname == nil {
		reason = "no name"
		return reason
	}

	// If marked "go:noinline", don't inline.
	if fn.Pragma&ir.Noinline != 0 {
		reason = "marked go:noinline"
		return reason
	}

	// If marked "go:norace" and -race compilation, don't inline.
	if base.Flag.Race && fn.Pragma&ir.Norace != 0 {
		reason = "marked go:norace with -race compilation"
		return reason
	}

	// If marked "go:nocheckptr" and -d checkptr compilation, don't inline.
	if base.Debug.Checkptr != 0 && fn.Pragma&ir.NoCheckPtr != 0 {
		reason = "marked go:nocheckptr"
		return reason
	}

	// If marked "go:cgo_unsafe_args", don't inline, since the function
	// makes assumptions about its argument frame layout.
	if fn.Pragma&ir.CgoUnsafeArgs != 0 {
		reason = "marked go:cgo_unsafe_args"
		return reason
	}

	// If marked as "go:uintptrkeepalive", don't inline, since the keep
	// alive information is lost during inlining.
	//
	// TODO(prattmic): This is handled on calls during escape analysis,
	// which is after inlining. Move prior to inlining so the keep-alive is
	// maintained after inlining.
	if fn.Pragma&ir.UintptrKeepAlive != 0 {
		reason = "marked as having a keep-alive uintptr argument"
		return reason
	}

	// If marked as "go:uintptrescapes", don't inline, since the escape
	// information is lost during inlining.
	if fn.Pragma&ir.UintptrEscapes != 0 {
		reason = "marked as having an escaping uintptr argument"
		return reason
	}

	// The nowritebarrierrec checker currently works at function
	// granularity, so inlining yeswritebarrierrec functions can confuse it
	// (#22342). As a workaround, disallow inlining them for now.
	if fn.Pragma&ir.Yeswritebarrierrec != 0 {
		reason = "marked go:yeswritebarrierrec"
		return reason
	}

	// If a local function has no fn.Body (is defined outside of Go), cannot inline it.
	// Imported functions don't have fn.Body but might have inline body in fn.Inl.
	if len(fn.Body) == 0 && !typecheck.HaveInlineBody(fn) {
		reason = "no function body"
		return reason
	}

	return ""
}

// canDelayResults reports whether inlined calls to fn can delay
// declaring the result parameter until the "return" statement.
func canDelayResults(fn *ir.Func) bool {
	// We can delay declaring+initializing result parameters if:
	// (1) there's exactly one "return" statement in the inlined function;
	// (2) it's not an empty return statement (#44355); and
	// (3) the result parameters aren't named.

	nreturns := 0
	ir.VisitList(fn.Body, func(n ir.Node) {
		if n, ok := n.(*ir.ReturnStmt); ok {
			nreturns++
			if len(n.Results) == 0 {
				nreturns++ // empty return statement (case 2)
			}
		}
	})

	if nreturns != 1 {
		return false // not exactly one return statement (case 1)
	}

	// temporaries for return values.
	for _, param := range fn.Type().Results() {
		if sym := param.Sym; sym != nil && !sym.IsBlank() {
			return false // found a named result parameter (case 3)
		}
	}

	return true
}

// hairyVisitor visits a function body to determine its inlining
// hairiness and whether or not it can be inlined.
type hairyVisitor struct {
	// This is needed to access the current caller in the doNode function.
	curFunc       *ir.Func
	isBigFunc     bool
	debug         bool
	budget        int32
	maxBudget     int32
	reason        string
	extraCallCost int32
	usedLocals    ir.NameSet
	do            func(ir.Node) bool
	profile       *pgoir.Profile
	dbg           bool
}

func isDebugFn(fn *ir.Func) bool {
	// if n := fn.Nname; n != nil {
	// 	if n.Sym().Name == "Int32x8.Transpose8" && n.Sym().Pkg.Path == "simd/archsimd" {
	// 		fmt.Printf("isDebugFn '%s' DOT '%s'\n", n.Sym().Pkg.Path, n.Sym().Name)
	// 		return true
	// 	}
	// }
	return false
}

func (v *hairyVisitor) tooHairy(fn *ir.Func) bool {
	v.do = v.doNode // cache closure
	if ir.DoChildren(fn, v.do) {
		return true
	}
	if v.budget < 0 {
		v.reason = fmt.Sprintf("function too complex: cost %d exceeds budget %d", v.maxBudget-v.budget, v.maxBudget)
		return true
	}
	return false
}

// doNode visits n and its children, updates the state in v, and returns true if
// n makes the current function too hairy for inlining.
func (v *hairyVisitor) doNode(n ir.Node) bool {
	if n == nil {
		return false
	}
	if v.debug {
		fmt.Printf("%v: doNode %v budget is %d\n", ir.Line(n), n.Op(), v.budget)
	}
opSwitch:
	switch n.Op() {
	// Call is okay if inlinable and we have the budget for the body.
	case ir.OCALLFUNC:
		n := n.(*ir.CallExpr)
		var cheap bool
		if n.Fun.Op() == ir.ONAME {
			name := n.Fun.(*ir.Name)
			if name.Class == ir.PFUNC {
				s := name.Sym()
				fn := s.Name
				switch s.Pkg.Path {
				case "internal/abi":
					switch fn {
					case "NoEscape":
						// Special case for internal/abi.NoEscape. It does just type
						// conversions to appease the escape analysis, and doesn't
						// generate code.
						cheap = true
					}
					if strings.HasPrefix(fn, "EscapeNonString[") {
						// internal/abi.EscapeNonString[T] is a compiler intrinsic
						// implemented in the escape analysis phase.
						cheap = true
					}
				case "internal/runtime/sys":
					switch fn {
					case "GetCallerPC", "GetCallerSP":
						// Functions that call GetCallerPC/SP can not be inlined
						// because users expect the PC/SP of the logical caller,
						// but GetCallerPC/SP returns the physical caller.
						v.reason = "call to " + fn
						return true
					}
				case "go.runtime":
					switch fn {
					case "throw":
						// runtime.throw is a "cheap call" like panic in normal code.
						v.budget -= inlineExtraThrowCost
						break opSwitch
					case "panicrangestate":
						cheap = true
					case "deferrangefunc":
						v.reason = "defer call in range func"
						return true
					}
				}
			}
			// Special case for coverage counter updates; although
			// these correspond to real operations, we treat them as
			// zero cost for the moment. This is due to the existence
			// of tests that are sensitive to inlining-- if the
			// insertion of coverage instrumentation happens to tip a
			// given function over the threshold and move it from
			// "inlinable" to "not-inlinable", this can cause changes
			// in allocation behavior, which can then result in test
			// failures (a good example is the TestAllocations in
			// crypto/ed25519).
			if isAtomicCoverageCounterUpdate(n) {
				return false
			}
		}
		if n.Fun.Op() == ir.OMETHEXPR {
			if meth := ir.MethodExprName(n.Fun); meth != nil {
				if fn := meth.Func; fn != nil {
					s := fn.Sym()
					if types.RuntimeSymName(s) == "heapBits.nextArena" {
						// Special case: explicitly allow mid-stack inlining of
						// runtime.heapBits.next even though it calls slow-path
						// runtime.heapBits.nextArena.
						cheap = true
					}
					// Special case: on architectures that can do unaligned loads,
					// explicitly mark encoding/binary methods as cheap,
					// because in practice they are, even though our inlining
					// budgeting system does not see that. See issue 42958.
					if base.Ctxt.Arch.CanMergeLoads && s.Pkg.Path == "encoding/binary" {
						switch s.Name {
						case "littleEndian.Uint64", "littleEndian.Uint32", "littleEndian.Uint16",
							"bigEndian.Uint64", "bigEndian.Uint32", "bigEndian.Uint16",
							"littleEndian.PutUint64", "littleEndian.PutUint32", "littleEndian.PutUint16",
							"bigEndian.PutUint64", "bigEndian.PutUint32", "bigEndian.PutUint16",
							"littleEndian.AppendUint64", "littleEndian.AppendUint32", "littleEndian.AppendUint16",
							"bigEndian.AppendUint64", "bigEndian.AppendUint32", "bigEndian.AppendUint16":
							cheap = true
						}
					}
				}
			}
		}

		// A call to a parameter is optimistically a cheap call, if it's a constant function
		// perhaps it will inline, it also can simplify escape analysis.
		extraCost := v.extraCallCost

		if n.Fun.Op() == ir.ONAME {
			name := n.Fun.(*ir.Name)
			if name.Class == ir.PFUNC {
				// Special case: on architectures that can do unaligned loads,
				// explicitly mark internal/byteorder methods as cheap,
				// because in practice they are, even though our inlining
				// budgeting system does not see that. See issue 42958.
				if base.Ctxt.Arch.CanMergeLoads && name.Sym().Pkg.Path == "internal/byteorder" {
					switch name.Sym().Name {
					case "LEUint64", "LEUint32", "LEUint16",
						"BEUint64", "BEUint32", "BEUint16",
						"LEPutUint64", "LEPutUint32", "LEPutUint16",
						"BEPutUint64", "BEPutUint32", "BEPutUint16",
						"LEAppendUint64", "LEAppendUint32", "LEAppendUint16",
						"BEAppendUint64", "BEAppendUint32", "BEAppendUint16":
						cheap = true
					}
				}
			}
			if name.Class == ir.PPARAM || name.Class == ir.PAUTOHEAP && name.IsClosureVar() {
				extraCost = min(extraCost, inlineParamCallCost)
			}
		}

		if cheap {
			if v.debug {
				if ir.IsIntrinsicCall(n) {
					fmt.Printf("%v: cheap call is also intrinsic, %v\n", ir.Line(n), n)
				}
			}
			break // treat like any other node, that is, cost of 1
		}

		if ir.IsIntrinsicCall(n) {
			if v.debug {
				fmt.Printf("%v: intrinsic call, %v\n", ir.Line(n), n)
			}
			break // Treat like any other node.
		}

		if callee := inlCallee(v.curFunc, n.Fun, v.profile, false); callee != nil && typecheck.HaveInlineBody(callee) {
			// Check whether we'd actually inline this call. Set
			// log == false since we aren't actually doing inlining
			// yet.
			if ok, _, _ := canInlineCallExpr(v.curFunc, n, callee, v.isBigFunc, false, false); ok {
				// mkinlcall would inline this call [1], so use
				// the cost of the inline body as the cost of
				// the call, as that is what will actually
				// appear in the code.
				//
				// [1] This is almost a perfect match to the
				// mkinlcall logic, except that
				// canInlineCallExpr considers inlining cycles
				// by looking at what has already been inlined.
				// Since we haven't done any inlining yet we
				// will miss those.
				//
				// TODO: in the case of a single-call closure, the inlining budget here is potentially much, much larger.
				//
				v.budget -= callee.Inl.Cost
				break
			}
		}

		if v.debug {
			fmt.Printf("%v: costly OCALLFUNC %v\n", ir.Line(n), n)
		}

		// Call cost for non-leaf inlining.
		v.budget -= extraCost

	case ir.OCALLMETH:
		base.FatalfAt(n.Pos(), "OCALLMETH missed by typecheck")

	// Things that are too hairy, irrespective of the budget
	case ir.OCALL, ir.OCALLINTER:
		// Call cost for non-leaf inlining.
		if v.debug {
			fmt.Printf("%v: costly OCALL %v\n", ir.Line(n), n)
		}
		v.budget -= v.extraCallCost

	case ir.OPANIC:
		n := n.(*ir.UnaryExpr)
		if n.X.Op() == ir.OCONVIFACE && n.X.(*ir.ConvExpr).Implicit() {
			// Hack to keep reflect.flag.mustBe inlinable for TestIntendedInlining.
			// Before CL 284412, these conversions were introduced later in the
			// compiler, so they didn't count against inlining budget.
			v.budget++
		}
		v.budget -= inlineExtraPanicCost

	case ir.ORECOVER:
		// TODO: maybe we could allow inlining of recover() now?
		v.reason = "call to recover"
		return true

	case ir.OCLOSURE:
		if base.Debug.InlFuncsWithClosures == 0 {
			v.reason = "not inlining functions with closures"
			return true
		}

		// TODO(danscales): Maybe make budget proportional to number of closure
		// variables, e.g.:
		//v.budget -= int32(len(n.(*ir.ClosureExpr).Func.ClosureVars) * 3)
		// TODO(austin): However, if we're able to inline this closure into
		// v.curFunc, then we actually pay nothing for the closure captures. We
		// should try to account for that if we're going to account for captures.
		v.budget -= 15

	case ir.OGO, ir.ODEFER, ir.OTAILCALL:
		v.reason = "unhandled op " + n.Op().String()
		return true

	case ir.OAPPEND:
		v.budget -= inlineExtraAppendCost

	case ir.OADDR:
		n := n.(*ir.AddrExpr)
		// Make "&s.f" cost 0 when f's offset is zero.
		if dot, ok := n.X.(*ir.SelectorExpr); ok && (dot.Op() == ir.ODOT || dot.Op() == ir.ODOTPTR) {
			if _, ok := dot.X.(*ir.Name); ok && dot.Selection.Offset == 0 {
				v.budget += 2 // undo ir.OADDR+ir.ODOT/ir.ODOTPTR
			}
		}

	case ir.ODEREF:
		// *(*X)(unsafe.Pointer(&x)) is low-cost
		n := n.(*ir.StarExpr)

		ptr := n.X
		for ptr.Op() == ir.OCONVNOP {
			ptr = ptr.(*ir.ConvExpr).X
		}
		if ptr.Op() == ir.OADDR {
			v.budget += 1 // undo half of default cost of ir.ODEREF+ir.OADDR
		}

	case ir.OCONVNOP:
		// This doesn't produce code, but the children might.
		v.budget++ // undo default cost

	case ir.OFALL, ir.OTYPE:
		// These nodes don't produce code; omit from inlining budget.
		return false

	case ir.OIF:
		n := n.(*ir.IfStmt)
		if ir.IsConst(n.Cond, constant.Bool) {
			// This if and the condition cost nothing.
			if doList(n.Init(), v.do) {
				return true
			}
			if ir.BoolVal(n.Cond) {
				return doList(n.Body, v.do)
			} else {
				return doList(n.Else, v.do)
			}
		}

	case ir.ONAME:
		n := n.(*ir.Name)
		if n.Class == ir.PAUTO {
			v.usedLocals.Add(n)
		}

	case ir.OBLOCK:
		// The only OBLOCK we should see at this point is an empty one.
		// In any event, let the visitList(n.List()) below take care of the statements,
		// and don't charge for the OBLOCK itself. The ++ undoes the -- below.
		v.budget++

	case ir.OMETHVALUE, ir.OSLICELIT:
		v.budget-- // Hack for toolstash -cmp.

	case ir.OMETHEXPR:
		v.budget++ // Hack for toolstash -cmp.

	case ir.OAS2:
		n := n.(*ir.AssignListStmt)

		// Unified IR unconditionally rewrites:
		//
		//	a, b = f()
		//
		// into:
		//
		//	DCL tmp1
		//	DCL tmp2
		//	tmp1, tmp2 = f()
		//	a, b = tmp1, tmp2
		//
		// so that it can insert implicit conversions as necessary. To
		// minimize impact to the existing inlining heuristics (in
		// particular, to avoid breaking the existing inlinability regress
		// tests), we need to compensate for this here.
		//
		// See also identical logic in IsBigFunc.
		if len(n.Rhs) > 0 {
			if init := n.Rhs[0].Init(); len(init) == 1 {
				if _, ok := init[0].(*ir.AssignListStmt); ok {
					// 4 for each value, because each temporary variable now
					// appears 3 times (DCL, LHS, RHS), plus an extra DCL node.
					//
					// 1 for the extra "tmp1, tmp2 = f()" assignment statement.
					v.budget += 4*int32(len(n.Lhs)) + 1
				}
			}
		}

	case ir.OAS:
		// Special case for coverage counter updates and coverage
		// function registrations. Although these correspond to real
		// operations, we treat them as zero cost for the moment. This
		// is primarily due to the existence of tests that are
		// sensitive to inlining-- if the insertion of coverage
		// instrumentation happens to tip a given function over the
		// threshold and move it from "inlinable" to "not-inlinable",
		// this can cause changes in allocation behavior, which can
		// then result in test failures (a good example is the
		// TestAllocations in crypto/ed25519).
		n := n.(*ir.AssignStmt)
		if n.X.Op() == ir.OINDEX && isIndexingCoverageCounter(n.X) {
			return false
		}

	case ir.OSLICE, ir.OSLICEARR, ir.OSLICESTR, ir.OSLICE3, ir.OSLICE3ARR:
		n := n.(*ir.SliceExpr)

		// Ignore superfluous slicing.
		if n.Low != nil && n.Low.Op() == ir.OLITERAL && ir.Int64Val(n.Low) == 0 {
			v.budget++
		}
		if n.High != nil && n.High.Op() == ir.OLEN && n.High.(*ir.UnaryExpr).X == n.X {
			v.budget += 2
		}
	}

	v.budget--

	// When debugging, don't stop early, to get full cost of inlining this function
	if v.budget < 0 && base.Flag.LowerM < 2 && !logopt.Enabled() && !v.debug {
		v.reason = "too expensive"
		return true
	}

	return ir.DoChildren(n, v.do)
}

// IsBigFunc reports whether fn is a "big" function.
//
// Note: The criteria for "big" is heuristic and subject to change.
func IsBigFunc(fn *ir.Func) bool {
	budget := inlineBigFunctionNodes
	return ir.Any(fn, func(n ir.Node) bool {
		// See logic in hairyVisitor.doNode, explaining unified IR's
		// handling of "a, b = f()" assignments.
		if n, ok := n.(*ir.AssignListStmt); ok && n.Op() == ir.OAS2 && len(n.Rhs) > 0 {
			if init := n.Rhs[0].Init(); len(init) == 1 {
				if _, ok := init[0].(*ir.AssignListStmt); ok {
					budget += 4*len(n.Lhs) + 1
				}
			}
		}

		budget--
		return budget <= 0
	})
}

// inlineCallCheck returns whether a call will never be inlineable
// for basic reasons, and whether the call is an intrinisic call.
// The intrinsic result singles out intrinsic calls for debug logging.
func inlineCallCheck(callerfn *ir.Func, call *ir.CallExpr) (bool, bool) {
	if base.Flag.LowerL == 0 {
		return false, false
	}
	if call.Op() != ir.OCALLFUNC {
		return false, false
	}
	if call.GoDefer || call.NoInline {
		return false, false
	}

	// Prevent inlining some reflect.Value methods when using checkptr,
	// even when package reflect was compiled without it (#35073).
	if base.Debug.Checkptr != 0 && call.Fun.Op() == ir.OMETHEXPR {
		if method := ir.MethodExprName(call.Fun); method != nil {
			switch types.ReflectSymName(method.Sym()) {
			case "Value.UnsafeAddr", "Value.Pointer":
				return false, false
			}
		}
	}

	// internal/abi.EscapeNonString[T] is a compiler intrinsic implemented
	// in the escape analysis phase.
	if fn := ir.StaticCalleeName(call.Fun); fn != nil && fn.Sym().Pkg.Path == "internal/abi" &&
		strings.HasPrefix(fn.Sym().Name, "EscapeNonString[") {
		return false, true
	}

	if ir.IsIntrinsicCall(call) {
		return false, true
	}
	return true, false
}

// InlineCallTarget returns the resolved-for-inlining target of a call.
// It does not necessarily guarantee that the target can be inlined, though
// obvious exclusions are applied.
func InlineCallTarget(callerfn *ir.Func, call *ir.CallExpr, profile *pgoir.Profile) *ir.Func {
	if mightInline, _ := inlineCallCheck(callerfn, call); !mightInline {
		return nil
	}
	return inlCallee(callerfn, call.Fun, profile, true)
}

// TryInlineCall returns an inlined call expression for call, or nil
// if inlining is not possible.
func TryInlineCall(callerfn *ir.Func, call *ir.CallExpr, bigCaller bool, profile *pgoir.Profile, closureCalledOnce bool) *ir.InlinedCallExpr {
	mightInline, isIntrinsic := inlineCallCheck(callerfn, call)

	// Preserve old logging behavior
	if (mightInline || isIntrinsic) && base.Flag.LowerM > 3 {
		fmt.Printf("%v:call to func %+v\n", ir.Line(call), call.Fun)
	}
	if !mightInline {
		return nil
	}

	if fn := inlCallee(callerfn, call.Fun, profile, false); fn != nil && typecheck.HaveInlineBody(fn) {
		return mkinlcall(callerfn, call, fn, bigCaller, closureCalledOnce, profile)
	}
	return nil
}

// inlCallee takes a function-typed expression and returns the underlying function ONAME
// that it refers to if statically known. Otherwise, it returns nil.
// resolveOnly skips cost-based inlineability checks for closures; the result may not actually be inlineable.
func inlCallee(caller *ir.Func, fn ir.Node, profile *pgoir.Profile, resolveOnly bool) (res *ir.Func) {
	fn = ir.StaticValue(fn)
	switch fn.Op() {
	case ir.OMETHEXPR:
		fn := fn.(*ir.SelectorExpr)
		n := ir.MethodExprName(fn)
		// Check that receiver type matches fn.X.
		// TODO(mdempsky): Handle implicit dereference
		// of pointer receiver argument?
		if n == nil || !types.Identical(n.Type().Recv().Type, fn.X.Type()) {
			return nil
		}
		return n.Func
	case ir.ONAME:
		fn := fn.(*ir.Name)
		if fn.Class == ir.PFUNC {
			return fn.Func
		}
	case ir.OCLOSURE:
		fn := fn.(*ir.ClosureExpr)
		c := fn.Func
		if len(c.ClosureVars) != 0 && c.ClosureVars[0].Outer.Curfn != caller {
			return nil // inliner doesn't support inlining across closure frames
		}
		if !resolveOnly {
			CanInline(c, profile)
		}
		return c
	}
	return nil
}

var inlgen int

// SSADumpInline gives the SSA back end a chance to dump the function
// when producing output for debugging the compiler itself.
var SSADumpInline = func(*ir.Func) {}

// InlineCall allows the inliner implementation to be overridden.
// If it returns nil, the function will not be inlined.
var InlineCall = func(callerfn *ir.Func, call *ir.CallExpr, fn *ir.Func, inlIndex int, profile *pgoir.Profile) *ir.InlinedCallExpr {
	base.Fatalf("inline.InlineCall not overridden")
	panic("unreachable")
}

// inlineCostOK returns true if call n from caller to callee is cheap enough to
// inline. bigCaller indicates that caller is a big function.
//
// In addition to the "cost OK" boolean, it also returns
//   - the "max cost" limit used to make the decision (which may differ depending on func size)
//   - the score assigned to this specific callsite
//   - whether the inlined function is "hot" according to PGO.
func inlineCostOK(n *ir.CallExpr, caller, callee *ir.Func, bigCaller, closureCalledOnce bool) (bool, int32, int32, bool) {
	maxCost := int32(inlineMaxBudget)

	if bigCaller {
		// We use this to restrict inlining into very big functions.
		// See issue 26546 and 17566.
		maxCost = inlineBigFunctionMaxCost
	}

	simdMaxCost := simdCreditMultiplier(callee) * maxCost

	if callee.ClosureParent != nil {
		maxCost *= 2           // favor inlining closures
		if closureCalledOnce { // really favor inlining the one call to this closure
			maxCost = max(maxCost, inlineClosureCalledOnceCost)
		}
	}

	maxCost = max(maxCost, simdMaxCost)

	metric := callee.Inl.Cost
	if inlheur.Enabled() {
		score, ok := inlheur.GetCallSiteScore(caller, n)
		if ok {
			metric = int32(score)
		}
	}

	lineOffset := pgoir.NodeLineOffset(n, caller)
	csi := pgoir.CallSiteInfo{LineOffset: lineOffset, Caller: caller}
	_, hot := candHotEdgeMap[csi]

	if metric <= maxCost {
		// Simple case. Function is already cheap enough.
		return true, 0, metric, hot
	}

	// We'll also allow inlining of hot functions below inlineHotMaxBudget,
	// but only in small functions.

	if !hot {
		// Cold
		return false, maxCost, metric, false
	}

	// Hot

	if bigCaller {
		if base.Debug.PGODebug > 0 {
			fmt.Printf("hot-big check disallows inlining for call %s (cost %d) at %v in big function %s\n", ir.PkgFuncName(callee), callee.Inl.Cost, ir.Line(n), ir.PkgFuncName(caller))
		}
		return false, maxCost, metric, false
	}

	if metric > inlineHotMaxBudget {
		return false, inlineHotMaxBudget, metric, false
	}

	if !base.PGOHash.MatchPosWithInfo(n.Pos(), "inline", nil) {
		// De-selected by PGO Hash.
		return false, maxCost, metric, false
	}

	if base.Debug.PGODebug > 0 {
		fmt.Printf("hot-budget check allows inlining for call %s (cost %d) at %v in function %s\n", ir.PkgFuncName(callee), callee.Inl.Cost, ir.Line(n), ir.PkgFuncName(caller))
	}

	return true, 0, metric, hot
}

// parsePos returns all the inlining positions and the innermost position.
func parsePos(pos src.XPos, posTmp []src.Pos) ([]src.Pos, src.Pos) {
	ctxt := base.Ctxt
	ctxt.AllPos(pos, func(p src.Pos) {
		posTmp = append(posTmp, p)
	})
	l := len(posTmp) - 1
	return posTmp[:l], posTmp[l]
}

// canInlineCallExpr returns true if the call n from caller to callee
// can be inlined, plus the score computed for the call expr in question,
// and whether the callee is hot according to PGO.
// bigCaller indicates that caller is a big function. log
// indicates that the 'cannot inline' reason should be logged.
//
// Preconditions: CanInline(callee) has already been called.
func canInlineCallExpr(callerfn *ir.Func, n *ir.CallExpr, callee *ir.Func, bigCaller, closureCalledOnce bool, log bool) (bool, int32, bool) {
	if callee.Inl == nil {
		// callee is never inlinable.
		if log && logopt.Enabled() {
			logopt.LogOpt(n.Pos(), "cannotInlineCall", "inline", ir.FuncName(callerfn),
				fmt.Sprintf("%s cannot be inlined", ir.PkgFuncName(callee)))
		}
		return false, 0, false
	}

	ok, maxCost, callSiteScore, hot := inlineCostOK(n, callerfn, callee, bigCaller, closureCalledOnce)
	if !ok {
		// callee cost too high for this call site.
		if log && logopt.Enabled() {
			logopt.LogOpt(n.Pos(), "cannotInlineCall", "inline", ir.FuncName(callerfn),
				fmt.Sprintf("cost %d of %s exceeds max caller cost %d", callee.Inl.Cost, ir.PkgFuncName(callee), maxCost))
		}
		return false, 0, false
	}

	callees, calleeInner := parsePos(n.Pos(), make([]src.Pos, 0, 10))

	for _, p := range callees {
		if p.Line() == calleeInner.Line() && p.Col() == calleeInner.Col() && p.AbsFilename() == calleeInner.AbsFilename() {
			if log && logopt.Enabled() {
				logopt.LogOpt(n.Pos(), "cannotInlineCall", "inline", fmt.Sprintf("recursive call to %s", ir.FuncName(callerfn)))
			}
			return false, 0, false
		}
	}

	if base.Flag.Cfg.Instrumenting && types.IsNoInstrumentPkg(callee.Sym().Pkg) {
		// Runtime package must not be instrumented.
		// Instrument skips runtime package. However, some runtime code can be
		// inlined into other packages and instrumented there. To avoid this,
		// we disable inlining of runtime functions when instrumenting.
		// The example that we observed is inlining of LockOSThread,
		// which lead to false race reports on m contents.
		if log && logopt.Enabled() {
			logopt.LogOpt(n.Pos(), "cannotInlineCall", "inline", ir.FuncName(callerfn),
				fmt.Sprintf("call to runtime function %s in instrumented build", ir.PkgFuncName(callee)))
		}
		return false, 0, false
	}

	if base.Flag.Race && types.IsNoRacePkg(callee.Sym().Pkg) {
		if log && logopt.Enabled() {
			logopt.LogOpt(n.Pos(), "cannotInlineCall", "inline", ir.FuncName(callerfn),
				fmt.Sprintf(`call to into "no-race" package function %s in race build`, ir.PkgFuncName(callee)))
		}
		return false, 0, false
	}

	if base.Debug.Checkptr != 0 && types.IsRuntimePkg(callee.Sym().Pkg) {
		// We don't instrument runtime packages for checkptr (see base/flag.go).
		if log && logopt.Enabled() {
			logopt.LogOpt(n.Pos(), "cannotInlineCall", "inline", ir.FuncName(callerfn),
				fmt.Sprintf(`call to into runtime package function %s in -d=checkptr build`, ir.PkgFuncName(callee)))
		}
		return false, 0, false
	}

	// Check if we've already inlined this function at this particular
	// call site, in order to stop inlining when we reach the beginning
	// of a recursion cycle again. We don't inline immediately recursive
	// functions, but allow inlining if there is a recursion cycle of
	// many functions. Most likely, the inlining will stop before we
	// even hit the beginning of the cycle again, but this catches the
	// unusual case.
	parent := base.Ctxt.PosTable.Pos(n.Pos()).Base().InliningIndex()
	sym := callee.Linksym()
	for inlIndex := parent; inlIndex >= 0; inlIndex = base.Ctxt.InlTree.Parent(inlIndex) {
		if base.Ctxt.InlTree.InlinedFunction(inlIndex) == sym {
			if log {
				if base.Flag.LowerM > 1 {
					fmt.Printf("%v: cannot inline %v into %v: repeated recursive cycle\n", ir.Line(n), callee, ir.FuncName(callerfn))
				}
				if logopt.Enabled() {
					logopt.LogOpt(n.Pos(), "cannotInlineCall", "inline", ir.FuncName(callerfn),
						fmt.Sprintf("repeated recursive cycle to %s", ir.PkgFuncName(callee)))
				}
			}
			return false, 0, false
		}
	}

	return true, callSiteScore, hot
}

// mkinlcall returns an OINLCALL node that can replace OCALLFUNC n, or
// nil if it cannot be inlined. callerfn is the function that contains
// n, and fn is the function being called.
//
// The result of mkinlcall MUST be assigned back to n, e.g.
//
//	n.Left = mkinlcall(n.Left, fn, isddd)
func mkinlcall(callerfn *ir.Func, n *ir.CallExpr, fn *ir.Func, bigCaller, closureCalledOnce bool, profile *pgoir.Profile) *ir.InlinedCallExpr {
	ok, score, hot := canInlineCallExpr(callerfn, n, fn, bigCaller, closureCalledOnce, true)
	if !ok {
		return nil
	}
	if hot {
		hasHotCall[callerfn] = struct{}{}
	}
	typecheck.AssertFixedCall(n)

	parent := base.Ctxt.PosTable.Pos(n.Pos()).Base().InliningIndex()
	sym := fn.Linksym()
	inlIndex := base.Ctxt.InlTree.Add(parent, n.Pos(), sym, ir.FuncName(fn))

	closureInitLSym := func(n *ir.CallExpr, fn *ir.Func) {
		// The linker needs FuncInfo metadata for all inlined
		// functions. This is typically handled by gc.enqueueFunc
		// calling ir.InitLSym for all function declarations in
		// typecheck.Target.Decls (ir.UseClosure adds all closures to
		// Decls).
		//
		// However, closures in Decls are ignored, and are
		// instead enqueued when walk of the calling function
		// discovers them.
		//
		// This presents a problem for direct calls to closures.
		// Inlining will replace the entire closure definition with its
		// body, which hides the closure from walk and thus suppresses
		// symbol creation.
		//
		// Explicitly create a symbol early in this edge case to ensure
		// we keep this metadata.
		//
		// TODO: Refactor to keep a reference so this can all be done
		// by enqueueFunc.

		if n.Op() != ir.OCALLFUNC {
			// Not a standard call.
			return
		}

		var nf = n.Fun
		// Skips ir.OCONVNOPs, see issue #73716.
		for nf.Op() == ir.OCONVNOP {
			nf = nf.(*ir.ConvExpr).X
		}
		if nf.Op() != ir.OCLOSURE {
			// Not a direct closure call or one with type conversion.
			return
		}

		clo := nf.(*ir.ClosureExpr)
		if !clo.Func.IsClosure() {
			// enqueueFunc will handle non closures anyways.
			return
		}

		ir.InitLSym(fn, true)
	}

	closureInitLSym(n, fn)

	if base.Flag.GenDwarfInl > 0 {
		if !sym.WasInlined() {
			base.Ctxt.DwFixups.SetPrecursorFunc(sym, fn)
			sym.Set(obj.AttrWasInlined, true)
		}
	}

	if base.Flag.LowerM != 0 {
		if buildcfg.Experiment.NewInliner {
			fmt.Printf("%v: inlining call to %v with score %d\n",
				ir.Line(n), fn.Nname.DiagName(), score)
		} else {
			fmt.Printf("%v: inlining call to %v\n", ir.Line(n), fn.Nname.DiagName())
		}
	}
	if base.Flag.LowerM > 2 {
		fmt.Printf("%v: Before inlining: %+v\n", ir.Line(n), n)
	}

	res := InlineCall(callerfn, n, fn, inlIndex, profile)

	if res == nil {
		base.FatalfAt(n.Pos(), "inlining call to %v failed", fn.Nname.DiagName())
	}

	if base.Flag.LowerM > 2 {
		fmt.Printf("%v: After inlining %+v\n\n", ir.Line(res), res)
	}

	if inlheur.Enabled() {
		inlheur.UpdateCallsiteTable(callerfn, n, res)
	}

	return res
}

// CalleeEffects appends any side effects from evaluating callee to init.
func CalleeEffects(init *ir.Nodes, callee ir.Node) {
	for {
		init.Append(ir.TakeInit(callee)...)

		switch callee.Op() {
		case ir.ONAME, ir.OCLOSURE, ir.OMETHEXPR:
			return // done

		case ir.OCONVNOP:
			conv := callee.(*ir.ConvExpr)
			callee = conv.X

		case ir.OINLCALL:
			ic := callee.(*ir.InlinedCallExpr)
			init.Append(ic.Body.Take()...)
			callee = ic.SingleResult()

		default:
			base.FatalfAt(callee.Pos(), "unexpected callee expression: %v", callee)
		}
	}
}

func pruneUnusedAutos(ll []*ir.Name, vis *hairyVisitor) []*ir.Name {
	s := make([]*ir.Name, 0, len(ll))
	for _, n := range ll {
		if n.Class == ir.PAUTO {
			if !vis.usedLocals.Has(n) {
				// TODO(mdempsky): Simplify code after confident that this
				// never happens anymore.
				base.FatalfAt(n.Pos(), "unused auto: %v", n)
				continue
			}
		}
		s = append(s, n)
	}
	return s
}

func doList(list []ir.Node, do func(ir.Node) bool) bool {
	for _, x := range list {
		if x != nil {
			if do(x) {
				return true
			}
		}
	}
	return false
}

// isIndexingCoverageCounter returns true if the specified node 'n' is indexing
// into a coverage counter array.
func isIndexingCoverageCounter(n ir.Node) bool {
	if n.Op() != ir.OINDEX {
		return false
	}
	ixn := n.(*ir.IndexExpr)
	if ixn.X.Op() != ir.ONAME || !ixn.X.Type().IsArray() {
		return false
	}
	nn := ixn.X.(*ir.Name)
	// CoverageAuxVar implies either a coverage counter or a package
	// ID; since the cover tool never emits code to index into ID vars
	// this is effectively testing whether nn is a coverage counter.
	return nn.CoverageAuxVar()
}

// isAtomicCoverageCounterUpdate examines the specified node to
// determine whether it represents a call to sync/atomic.AddUint32 to
// increment a coverage counter.
func isAtomicCoverageCounterUpdate(cn *ir.CallExpr) bool {
	if cn.Fun.Op() != ir.ONAME {
		return false
	}
	name := cn.Fun.(*ir.Name)
	if name.Class != ir.PFUNC {
		return false
	}
	fn := name.Sym().Name
	if name.Sym().Pkg.Path != "sync/atomic" ||
		(fn != "AddUint32" && fn != "StoreUint32") {
		return false
	}
	if len(cn.Args) != 2 || cn.Args[0].Op() != ir.OADDR {
		return false
	}
	adn := cn.Args[0].(*ir.AddrExpr)
	v := isIndexingCoverageCounter(adn.X)
	return v
}

func PostProcessCallSites(profile *pgoir.Profile) {
	if base.Debug.DumpInlCallSiteScores != 0 {
		budgetCallback := func(fn *ir.Func, prof *pgoir.Profile) (int32, bool) {
			v := inlineBudget(fn, prof, false, false)
			return v, v == inlineHotMaxBudget
		}
		inlheur.DumpInlCallSiteScores(profile, budgetCallback)
	}
}

func analyzeFuncProps(fn *ir.Func, p *pgoir.Profile) {
	canInline := func(fn *ir.Func) { CanInline(fn, p) }
	budgetForFunc := func(fn *ir.Func) int32 {
		return inlineBudget(fn, p, true, false)
	}
	inlheur.AnalyzeFunc(fn, canInline, budgetForFunc, inlineMaxBudget)
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/actualexprpropbits_string.go ===
```go
// Code generated by "stringer -bitset -type ActualExprPropBits"; DO NOT EDIT.

package inlheur

import "strconv"
import "bytes"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ActualExprConstant-1]
	_ = x[ActualExprIsConcreteConvIface-2]
	_ = x[ActualExprIsFunc-4]
	_ = x[ActualExprIsInlinableFunc-8]
}

var _ActualExprPropBits_value = [...]uint64{
	0x1, /* ActualExprConstant */
	0x2, /* ActualExprIsConcreteConvIface */
	0x4, /* ActualExprIsFunc */
	0x8, /* ActualExprIsInlinableFunc */
}

const _ActualExprPropBits_name = "ActualExprConstantActualExprIsConcreteConvIfaceActualExprIsFuncActualExprIsInlinableFunc"

var _ActualExprPropBits_index = [...]uint8{0, 18, 47, 63, 88}

func (i ActualExprPropBits) String() string {
	var b bytes.Buffer

	remain := uint64(i)
	seen := false

	for k, v := range _ActualExprPropBits_value {
		x := _ActualExprPropBits_name[_ActualExprPropBits_index[k]:_ActualExprPropBits_index[k+1]]
		if v == 0 {
			if i == 0 {
				b.WriteString(x)
				return b.String()
			}
			continue
		}
		if (v & remain) == v {
			remain &^= v
			x := _ActualExprPropBits_name[_ActualExprPropBits_index[k]:_ActualExprPropBits_index[k+1]]
			if seen {
				b.WriteString("|")
			}
			seen = true
			b.WriteString(x)
		}
	}
	if remain == 0 {
		return b.String()
	}
	return "ActualExprPropBits(0x" + strconv.FormatInt(int64(i), 16) + ")"
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/analyze.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmp"
	"encoding/json"
	"fmt"
	"internal/buildcfg"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	debugTraceFuncs = 1 << iota
	debugTraceFuncFlags
	debugTraceResults
	debugTraceParams
	debugTraceExprClassify
	debugTraceCalls
	debugTraceScoring
)

// propAnalyzer interface is used for defining one or more analyzer
// helper objects, each tasked with computing some specific subset of
// the properties we're interested in. The assumption is that
// properties are independent, so each new analyzer that implements
// this interface can operate entirely on its own. For a given analyzer
// there will be a sequence of calls to nodeVisitPre and nodeVisitPost
// as the nodes within a function are visited, then a followup call to
// setResults so that the analyzer can transfer its results into the
// final properties object.
type propAnalyzer interface {
	nodeVisitPre(n ir.Node)
	nodeVisitPost(n ir.Node)
	setResults(funcProps *FuncProps)
}

// fnInlHeur contains inline heuristics state information about a
// specific Go function being analyzed/considered by the inliner. Note
// that in addition to constructing a fnInlHeur object by analyzing a
// specific *ir.Func, there is also code in the test harness
// (funcprops_test.go) that builds up fnInlHeur's by reading in and
// parsing a dump. This is the reason why we have file/fname/line
// fields below instead of just an *ir.Func field.
type fnInlHeur struct {
	props *FuncProps
	cstab CallSiteTab
	fname string
	file  string
	line  uint
}

var fpmap = map[*ir.Func]fnInlHeur{}

// AnalyzeFunc computes function properties for fn and its contained
// closures, updating the global 'fpmap' table. It is assumed that
// "CanInline" has been run on fn and on the closures that feed
// directly into calls; other closures not directly called will also
// be checked inlinability for inlinability here in case they are
// returned as a result.
func AnalyzeFunc(fn *ir.Func, canInline func(*ir.Func), budgetForFunc func(*ir.Func) int32, inlineMaxBudget int) {
	if fpmap == nil {
		// If fpmap is nil this indicates that the main inliner pass is
		// complete and we're doing inlining of wrappers (no heuristics
		// used here).
		return
	}
	if fn.OClosure != nil {
		// closures will be processed along with their outer enclosing func.
		return
	}
	enableDebugTraceIfEnv()
	if debugTrace&debugTraceFuncs != 0 {
		fmt.Fprintf(os.Stderr, "=-= AnalyzeFunc(%v)\n", fn)
	}
	// Build up a list containing 'fn' and any closures it contains. Along
	// the way, test to see whether each closure is inlinable in case
	// we might be returning it.
	funcs := []*ir.Func{fn}
	ir.VisitFuncAndClosures(fn, func(n ir.Node) {
		if clo, ok := n.(*ir.ClosureExpr); ok {
			funcs = append(funcs, clo.Func)
		}
	})

	// Analyze the list of functions. We want to visit a given func
	// only after the closures it contains have been processed, so
	// iterate through the list in reverse order. Once a function has
	// been analyzed, revisit the question of whether it should be
	// inlinable; if it is over the default hairiness limit and it
	// doesn't have any interesting properties, then we don't want
	// the overhead of writing out its inline body.
	nameFinder := newNameFinder(fn)
	for i := len(funcs) - 1; i >= 0; i-- {
		f := funcs[i]
		if f.OClosure != nil && !f.InlinabilityChecked() {
			canInline(f)
		}
		funcProps := analyzeFunc(f, inlineMaxBudget, nameFinder)
		revisitInlinability(f, funcProps, budgetForFunc)
		if f.Inl != nil {
			f.Inl.Properties = funcProps.SerializeToString()
		}
	}
	disableDebugTrace()
}

// TearDown is invoked at the end of the main inlining pass; doing
// function analysis and call site scoring is unlikely to help a lot
// after this point, so nil out fpmap and other globals to reclaim
// storage.
func TearDown() {
	fpmap = nil
	scoreCallsCache.tab = nil
	scoreCallsCache.csl = nil
}

func analyzeFunc(fn *ir.Func, inlineMaxBudget int, nf *nameFinder) *FuncProps {
	if funcInlHeur, ok := fpmap[fn]; ok {
		return funcInlHeur.props
	}
	funcProps, fcstab := computeFuncProps(fn, inlineMaxBudget, nf)
	file, line := fnFileLine(fn)
	entry := fnInlHeur{
		fname: fn.Sym().Name,
		file:  file,
		line:  line,
		props: funcProps,
		cstab: fcstab,
	}
	fn.SetNeverReturns(entry.props.Flags&FuncPropNeverReturns != 0)
	fpmap[fn] = entry
	if fn.Inl != nil && fn.Inl.Properties == "" {
		fn.Inl.Properties = entry.props.SerializeToString()
	}
	return funcProps
}

// revisitInlinability revisits the question of whether to continue to
// treat function 'fn' as an inline candidate based on the set of
// properties we've computed for it. If (for example) it has an
// initial size score of 150 and no interesting properties to speak
// of, then there isn't really any point to moving ahead with it as an
// inline candidate.
func revisitInlinability(fn *ir.Func, funcProps *FuncProps, budgetForFunc func(*ir.Func) int32) {
	if fn.Inl == nil {
		return
	}
	maxAdj := int32(LargestNegativeScoreAdjustment(fn, funcProps))
	budget := budgetForFunc(fn)
	if fn.Inl.Cost+maxAdj > budget {
		fn.Inl = nil
	}
}

// computeFuncProps examines the Go function 'fn' and computes for it
// a function "properties" object, to be used to drive inlining
// heuristics. See comments on the FuncProps type for more info.
func computeFuncProps(fn *ir.Func, inlineMaxBudget int, nf *nameFinder) (*FuncProps, CallSiteTab) {
	if debugTrace&debugTraceFuncs != 0 {
		fmt.Fprintf(os.Stderr, "=-= starting analysis of func %v:\n%+v\n",
			fn, fn)
	}
	funcProps := new(FuncProps)
	ffa := makeFuncFlagsAnalyzer(fn)
	analyzers := []propAnalyzer{ffa}
	analyzers = addResultsAnalyzer(fn, analyzers, funcProps, inlineMaxBudget, nf)
	analyzers = addParamsAnalyzer(fn, analyzers, funcProps, nf)
	runAnalyzersOnFunction(fn, analyzers)
	for _, a := range analyzers {
		a.setResults(funcProps)
	}
	cstab := computeCallSiteTable(fn, fn.Body, nil, ffa.panicPathTable(), 0, nf)
	return funcProps, cstab
}

func runAnalyzersOnFunction(fn *ir.Func, analyzers []propAnalyzer) {
	var doNode func(ir.Node) bool
	doNode = func(n ir.Node) bool {
		for _, a := range analyzers {
			a.nodeVisitPre(n)
		}
		ir.DoChildren(n, doNode)
		for _, a := range analyzers {
			a.nodeVisitPost(n)
		}
		return false
	}
	doNode(fn)
}

func propsForFunc(fn *ir.Func) *FuncProps {
	if funcInlHeur, ok := fpmap[fn]; ok {
		return funcInlHeur.props
	} else if fn.Inl != nil && fn.Inl.Properties != "" {
		// FIXME: considering adding some sort of cache or table
		// for deserialized properties of imported functions.
		return DeserializeFromString(fn.Inl.Properties)
	}
	return nil
}

func fnFileLine(fn *ir.Func) (string, uint) {
	p := base.Ctxt.InnermostPos(fn.Pos())
	return filepath.Base(p.Filename()), p.Line()
}

func Enabled() bool {
	return buildcfg.Experiment.NewInliner || UnitTesting()
}

func UnitTesting() bool {
	return base.Debug.DumpInlFuncProps != "" ||
		base.Debug.DumpInlCallSiteScores != 0
}

// DumpFuncProps computes and caches function properties for the func
// 'fn', writing out a description of the previously computed set of
// properties to the file given in 'dumpfile'. Used for the
// "-d=dumpinlfuncprops=..." command line flag, intended for use
// primarily in unit testing.
func DumpFuncProps(fn *ir.Func, dumpfile string) {
	if fn != nil {
		if fn.OClosure != nil {
			// closures will be processed along with their outer enclosing func.
			return
		}
		captureFuncDumpEntry(fn)
		ir.VisitFuncAndClosures(fn, func(n ir.Node) {
			if clo, ok := n.(*ir.ClosureExpr); ok {
				captureFuncDumpEntry(clo.Func)
			}
		})
	} else {
		emitDumpToFile(dumpfile)
	}
}

// emitDumpToFile writes out the buffer function property dump entries
// to a file, for unit testing. Dump entries need to be sorted by
// definition line, and due to generics we need to account for the
// possibility that several ir.Func's will have the same def line.
func emitDumpToFile(dumpfile string) {
	mode := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if dumpfile[0] == '+' {
		dumpfile = dumpfile[1:]
		mode = os.O_WRONLY | os.O_APPEND | os.O_CREATE
	}
	if dumpfile[0] == '%' {
		dumpfile = dumpfile[1:]
		d, b := filepath.Dir(dumpfile), filepath.Base(dumpfile)
		ptag := strings.ReplaceAll(types.LocalPkg.Path, "/", ":")
		dumpfile = d + "/" + ptag + "." + b
	}
	outf, err := os.OpenFile(dumpfile, mode, 0644)
	if err != nil {
		base.Fatalf("opening function props dump file %q: %v\n", dumpfile, err)
	}
	defer outf.Close()
	dumpFilePreamble(outf)

	atline := map[uint]uint{}
	sl := make([]fnInlHeur, 0, len(dumpBuffer))
	for _, e := range dumpBuffer {
		sl = append(sl, e)
		atline[e.line] = atline[e.line] + 1
	}
	sl = sortFnInlHeurSlice(sl)

	prevline := uint(0)
	for _, entry := range sl {
		idx := uint(0)
		if prevline == entry.line {
			idx++
		}
		prevline = entry.line
		atl := atline[entry.line]
		if err := dumpFnPreamble(outf, &entry, nil, idx, atl); err != nil {
			base.Fatalf("function props dump: %v\n", err)
		}
	}
	dumpBuffer = nil
}

// captureFuncDumpEntry grabs the function properties object for 'fn'
// and enqueues it for later dumping. Used for the
// "-d=dumpinlfuncprops=..." command line flag, intended for use
// primarily in unit testing.
func captureFuncDumpEntry(fn *ir.Func) {
	// avoid capturing compiler-generated equality funcs.
	if strings.HasPrefix(fn.Sym().Name, ".eq.") {
		return
	}
	funcInlHeur, ok := fpmap[fn]
	if !ok {
		// Missing entry is expected for functions that are too large
		// to inline. We still want to write out call site scores in
		// this case however.
		funcInlHeur = fnInlHeur{cstab: callSiteTab}
	}
	if dumpBuffer == nil {
		dumpBuffer = make(map[*ir.Func]fnInlHeur)
	}
	if _, ok := dumpBuffer[fn]; ok {
		return
	}
	if debugTrace&debugTraceFuncs != 0 {
		fmt.Fprintf(os.Stderr, "=-= capturing dump for %v:\n", fn)
	}
	dumpBuffer[fn] = funcInlHeur
}

// dumpFilePreamble writes out a file-level preamble for a given
// Go function as part of a function properties dump.
func dumpFilePreamble(w io.Writer) {
	fmt.Fprintf(w, "// DO NOT EDIT (use 'go test -v -update-expected' instead.)\n")
	fmt.Fprintf(w, "// See cmd/compile/internal/inline/inlheur/testdata/props/README.txt\n")
	fmt.Fprintf(w, "// for more information on the format of this file.\n")
	fmt.Fprintf(w, "// %s\n", preambleDelimiter)
}

// dumpFnPreamble writes out a function-level preamble for a given
// Go function as part of a function properties dump. See the
// README.txt file in testdata/props for more on the format of
// this preamble.
func dumpFnPreamble(w io.Writer, funcInlHeur *fnInlHeur, ecst encodedCallSiteTab, idx, atl uint) error {
	fmt.Fprintf(w, "// %s %s %d %d %d\n",
		funcInlHeur.file, funcInlHeur.fname, funcInlHeur.line, idx, atl)
	// emit props as comments, followed by delimiter
	fmt.Fprintf(w, "%s// %s\n", funcInlHeur.props.ToString("// "), comDelimiter)
	data, err := json.Marshal(funcInlHeur.props)
	if err != nil {
		return fmt.Errorf("marshal error %v\n", err)
	}
	fmt.Fprintf(w, "// %s\n", string(data))
	dumpCallSiteComments(w, funcInlHeur.cstab, ecst)
	fmt.Fprintf(w, "// %s\n", fnDelimiter)
	return nil
}

// sortFnInlHeurSlice sorts a slice of fnInlHeur based on
// the starting line of the function definition, then by name.
func sortFnInlHeurSlice(sl []fnInlHeur) []fnInlHeur {
	slices.SortStableFunc(sl, func(a, b fnInlHeur) int {
		if a.line != b.line {
			return cmp.Compare(a.line, b.line)
		}
		return strings.Compare(a.fname, b.fname)
	})
	return sl
}

// delimiters written to various preambles to make parsing of
// dumps easier.
const preambleDelimiter = "<endfilepreamble>"
const fnDelimiter = "<endfuncpreamble>"
const comDelimiter = "<endpropsdump>"
const csDelimiter = "<endcallsites>"

// dumpBuffer stores up function properties dumps when
// "-d=dumpinlfuncprops=..." is in effect.
var dumpBuffer map[*ir.Func]fnInlHeur

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/analyze_func_callsites.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/pgoir"
	"cmd/compile/internal/typecheck"
	"fmt"
	"os"
	"strings"
)

type callSiteAnalyzer struct {
	fn *ir.Func
	*nameFinder
}

type callSiteTableBuilder struct {
	fn *ir.Func
	*nameFinder
	cstab    CallSiteTab
	ptab     map[ir.Node]pstate
	nstack   []ir.Node
	loopNest int
	isInit   bool
}

func makeCallSiteAnalyzer(fn *ir.Func) *callSiteAnalyzer {
	return &callSiteAnalyzer{
		fn:         fn,
		nameFinder: newNameFinder(fn),
	}
}

func makeCallSiteTableBuilder(fn *ir.Func, cstab CallSiteTab, ptab map[ir.Node]pstate, loopNestingLevel int, nf *nameFinder) *callSiteTableBuilder {
	isInit := fn.IsPackageInit() || strings.HasPrefix(fn.Sym().Name, "init.")
	return &callSiteTableBuilder{
		fn:         fn,
		cstab:      cstab,
		ptab:       ptab,
		isInit:     isInit,
		loopNest:   loopNestingLevel,
		nstack:     []ir.Node{fn},
		nameFinder: nf,
	}
}

// computeCallSiteTable builds and returns a table of call sites for
// the specified region in function fn. A region here corresponds to a
// specific subtree within the AST for a function. The main intended
// use cases are for 'region' to be either A) an entire function body,
// or B) an inlined call expression.
func computeCallSiteTable(fn *ir.Func, region ir.Nodes, cstab CallSiteTab, ptab map[ir.Node]pstate, loopNestingLevel int, nf *nameFinder) CallSiteTab {
	cstb := makeCallSiteTableBuilder(fn, cstab, ptab, loopNestingLevel, nf)
	var doNode func(ir.Node) bool
	doNode = func(n ir.Node) bool {
		cstb.nodeVisitPre(n)
		ir.DoChildren(n, doNode)
		cstb.nodeVisitPost(n)
		return false
	}
	for _, n := range region {
		doNode(n)
	}
	return cstb.cstab
}

func (cstb *callSiteTableBuilder) flagsForNode(call *ir.CallExpr) CSPropBits {
	var r CSPropBits

	if debugTrace&debugTraceCalls != 0 {
		fmt.Fprintf(os.Stderr, "=-= analyzing call at %s\n",
			fmtFullPos(call.Pos()))
	}

	// Set a bit if this call is within a loop.
	if cstb.loopNest > 0 {
		r |= CallSiteInLoop
	}

	// Set a bit if the call is within an init function (either
	// compiler-generated or user-written).
	if cstb.isInit {
		r |= CallSiteInInitFunc
	}

	// Decide whether to apply the panic path heuristic. Hack: don't
	// apply this heuristic in the function "main.main" (mostly just
	// to avoid annoying users).
	if !isMainMain(cstb.fn) {
		r = cstb.determinePanicPathBits(call, r)
	}

	return r
}

// determinePanicPathBits updates the CallSiteOnPanicPath bit within
// "r" if we think this call is on an unconditional path to
// panic/exit. Do this by walking back up the node stack to see if we
// can find either A) an enclosing panic, or B) a statement node that
// we've determined leads to a panic/exit.
func (cstb *callSiteTableBuilder) determinePanicPathBits(call ir.Node, r CSPropBits) CSPropBits {
	cstb.nstack = append(cstb.nstack, call)
	defer func() {
		cstb.nstack = cstb.nstack[:len(cstb.nstack)-1]
	}()

	for ri := range cstb.nstack[:len(cstb.nstack)-1] {
		i := len(cstb.nstack) - ri - 1
		n := cstb.nstack[i]
		_, isCallExpr := n.(*ir.CallExpr)
		_, isStmt := n.(ir.Stmt)
		if isCallExpr {
			isStmt = false
		}

		if debugTrace&debugTraceCalls != 0 {
			ps, inps := cstb.ptab[n]
			fmt.Fprintf(os.Stderr, "=-= callpar %d op=%s ps=%s inptab=%v stmt=%v\n", i, n.Op().String(), ps.String(), inps, isStmt)
		}

		if n.Op() == ir.OPANIC {
			r |= CallSiteOnPanicPath
			break
		}
		if v, ok := cstb.ptab[n]; ok {
			if v == psCallsPanic {
				r |= CallSiteOnPanicPath
				break
			}
			if isStmt {
				break
			}
		}
	}
	return r
}

// propsForArg returns property bits for a given call argument expression arg.
func (cstb *callSiteTableBuilder) propsForArg(arg ir.Node) ActualExprPropBits {
	if cval := cstb.constValue(arg); cval != nil {
		return ActualExprConstant
	}
	if cstb.isConcreteConvIface(arg) {
		return ActualExprIsConcreteConvIface
	}
	fname := cstb.funcName(arg)
	if fname != nil {
		if fn := fname.Func; fn != nil && typecheck.HaveInlineBody(fn) {
			return ActualExprIsInlinableFunc
		}
		return ActualExprIsFunc
	}
	return 0
}

// argPropsForCall returns a slice of argument properties for the
// expressions being passed to the callee in the specific call
// expression; these will be stored in the CallSite object for a given
// call and then consulted when scoring. If no arg has any interesting
// properties we try to save some space and return a nil slice.
func (cstb *callSiteTableBuilder) argPropsForCall(ce *ir.CallExpr) []ActualExprPropBits {
	rv := make([]ActualExprPropBits, len(ce.Args))
	somethingInteresting := false
	for idx := range ce.Args {
		argProp := cstb.propsForArg(ce.Args[idx])
		somethingInteresting = somethingInteresting || (argProp != 0)
		rv[idx] = argProp
	}
	if !somethingInteresting {
		return nil
	}
	return rv
}

func (cstb *callSiteTableBuilder) addCallSite(callee *ir.Func, call *ir.CallExpr) {
	flags := cstb.flagsForNode(call)
	argProps := cstb.argPropsForCall(call)
	if debugTrace&debugTraceCalls != 0 {
		fmt.Fprintf(os.Stderr, "=-= props %+v for call %v\n", argProps, call)
	}
	// FIXME: maybe bulk-allocate these?
	cs := &CallSite{
		Call:     call,
		Callee:   callee,
		Assign:   cstb.containingAssignment(call),
		ArgProps: argProps,
		Flags:    flags,
		ID:       uint(len(cstb.cstab)),
	}
	if _, ok := cstb.cstab[call]; ok {
		fmt.Fprintf(os.Stderr, "*** cstab duplicate entry at: %s\n",
			fmtFullPos(call.Pos()))
		fmt.Fprintf(os.Stderr, "*** call: %+v\n", call)
		panic("bad")
	}
	// Set initial score for callsite to the cost computed
	// by CanInline; this score will be refined later based
	// on heuristics.
	cs.Score = int(callee.Inl.Cost)

	if cstb.cstab == nil {
		cstb.cstab = make(CallSiteTab)
	}
	cstb.cstab[call] = cs
	if debugTrace&debugTraceCalls != 0 {
		fmt.Fprintf(os.Stderr, "=-= added callsite: caller=%v callee=%v n=%s\n",
			cstb.fn, callee, fmtFullPos(call.Pos()))
	}
}

func (cstb *callSiteTableBuilder) nodeVisitPre(n ir.Node) {
	switch n.Op() {
	case ir.ORANGE, ir.OFOR:
		if !hasTopLevelLoopBodyReturnOrBreak(loopBody(n)) {
			cstb.loopNest++
		}
	case ir.OCALLFUNC:
		ce := n.(*ir.CallExpr)
		callee := pgoir.DirectCallee(ce.Fun)
		if callee != nil && callee.Inl != nil {
			cstb.addCallSite(callee, ce)
		}
	}
	cstb.nstack = append(cstb.nstack, n)
}

func (cstb *callSiteTableBuilder) nodeVisitPost(n ir.Node) {
	cstb.nstack = cstb.nstack[:len(cstb.nstack)-1]
	switch n.Op() {
	case ir.ORANGE, ir.OFOR:
		if !hasTopLevelLoopBodyReturnOrBreak(loopBody(n)) {
			cstb.loopNest--
		}
	}
}

func loopBody(n ir.Node) ir.Nodes {
	if forst, ok := n.(*ir.ForStmt); ok {
		return forst.Body
	}
	if rst, ok := n.(*ir.RangeStmt); ok {
		return rst.Body
	}
	return nil
}

// hasTopLevelLoopBodyReturnOrBreak examines the body of a "for" or
// "range" loop to try to verify that it is a real loop, as opposed to
// a construct that is syntactically loopy but doesn't actually iterate
// multiple times, like:
//
//	for {
//	  blah()
//	  return 1
//	}
//
// [Remark: the pattern above crops up quite a bit in the source code
// for the compiler itself, e.g. the auto-generated rewrite code]
//
// Note that we don't look for GOTO statements here, so it's possible
// we'll get the wrong result for a loop with complicated control
// jumps via gotos.
func hasTopLevelLoopBodyReturnOrBreak(loopBody ir.Nodes) bool {
	for _, n := range loopBody {
		if n.Op() == ir.ORETURN || n.Op() == ir.OBREAK {
			return true
		}
	}
	return false
}

// containingAssignment returns the top-level assignment statement
// for a statement level function call "n". Examples:
//
//	x := foo()
//	x, y := bar(z, baz())
//	if blah() { ...
//
// Here the top-level assignment statement for the foo() call is the
// statement assigning to "x"; the top-level assignment for "bar()"
// call is the assignment to x,y. For the baz() and blah() calls,
// there is no top level assignment statement.
//
// The unstated goal here is that we want to use the containing
// assignment to establish a connection between a given call and the
// variables to which its results/returns are being assigned.
//
// Note that for the "bar" command above, the front end sometimes
// decomposes this into two assignments, the first one assigning the
// call to a pair of auto-temps, then the second one assigning the
// auto-temps to the user-visible vars. This helper will return the
// second (outer) of these two.
func (cstb *callSiteTableBuilder) containingAssignment(n ir.Node) ir.Node {
	parent := cstb.nstack[len(cstb.nstack)-1]

	// assignsOnlyAutoTemps returns TRUE of the specified OAS2FUNC
	// node assigns only auto-temps.
	assignsOnlyAutoTemps := func(x ir.Node) bool {
		alst := x.(*ir.AssignListStmt)
		oa2init := alst.Init()
		if len(oa2init) == 0 {
			return false
		}
		for _, v := range oa2init {
			d := v.(*ir.Decl)
			if !ir.IsAutoTmp(d.X) {
				return false
			}
		}
		return true
	}

	// Simple case: x := foo()
	if parent.Op() == ir.OAS {
		return parent
	}

	// Multi-return case: x, y := bar()
	if parent.Op() == ir.OAS2FUNC {
		// Hack city: if the result vars are auto-temps, try looking
		// for an outer assignment in the tree. The code shape we're
		// looking for here is:
		//
		// OAS1({x,y},OCONVNOP(OAS2FUNC({auto1,auto2},OCALLFUNC(bar))))
		//
		if assignsOnlyAutoTemps(parent) {
			par2 := cstb.nstack[len(cstb.nstack)-2]
			if par2.Op() == ir.OAS2 {
				return par2
			}
			if par2.Op() == ir.OCONVNOP {
				par3 := cstb.nstack[len(cstb.nstack)-3]
				if par3.Op() == ir.OAS2 {
					return par3
				}
			}
		}
	}

	return nil
}

// UpdateCallsiteTable handles updating of callerfn's call site table
// after an inlined has been carried out, e.g. the call at 'n' as been
// turned into the inlined call expression 'ic' within function
// callerfn. The chief thing of interest here is to make sure that any
// call nodes within 'ic' are added to the call site table for
// 'callerfn' and scored appropriately.
func UpdateCallsiteTable(callerfn *ir.Func, n *ir.CallExpr, ic *ir.InlinedCallExpr) {
	enableDebugTraceIfEnv()
	defer disableDebugTrace()

	funcInlHeur, ok := fpmap[callerfn]
	if !ok {
		// This can happen for compiler-generated wrappers.
		if debugTrace&debugTraceCalls != 0 {
			fmt.Fprintf(os.Stderr, "=-= early exit, no entry for caller fn %v\n", callerfn)
		}
		return
	}

	if debugTrace&debugTraceCalls != 0 {
		fmt.Fprintf(os.Stderr, "=-= UpdateCallsiteTable(caller=%v, cs=%s)\n",
			callerfn, fmtFullPos(n.Pos()))
	}

	// Mark the call in question as inlined.
	oldcs, ok := funcInlHeur.cstab[n]
	if !ok {
		// This can happen for compiler-generated wrappers.
		return
	}
	oldcs.aux |= csAuxInlined

	if debugTrace&debugTraceCalls != 0 {
		fmt.Fprintf(os.Stderr, "=-= marked as inlined: callee=%v %s\n",
			oldcs.Callee, EncodeCallSiteKey(oldcs))
	}

	// Walk the inlined call region to collect new callsites.
	var icp pstate
	if oldcs.Flags&CallSiteOnPanicPath != 0 {
		icp = psCallsPanic
	}
	var loopNestLevel int
	if oldcs.Flags&CallSiteInLoop != 0 {
		loopNestLevel = 1
	}
	ptab := map[ir.Node]pstate{ic: icp}
	nf := newNameFinder(nil)
	icstab := computeCallSiteTable(callerfn, ic.Body, nil, ptab, loopNestLevel, nf)

	// Record parent callsite. This is primarily for debug output.
	for _, cs := range icstab {
		cs.parent = oldcs
	}

	// Score the calls in the inlined body. Note the setting of
	// "doCallResults" to false here: at the moment there isn't any
	// easy way to localize or region-ize the work done by
	// "rescoreBasedOnCallResultUses", which currently does a walk
	// over the entire function to look for uses of a given set of
	// results. Similarly we're passing nil to makeCallSiteAnalyzer,
	// so as to run name finding without the use of static value &
	// friends.
	csa := makeCallSiteAnalyzer(nil)
	const doCallResults = false
	csa.scoreCallsRegion(callerfn, ic.Body, icstab, doCallResults, ic)
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/analyze_func_flags.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"fmt"
	"os"
)

// funcFlagsAnalyzer computes the "Flags" value for the FuncProps
// object we're computing. The main item of interest here is "nstate",
// which stores the disposition of a given ir Node with respect to the
// flags/properties we're trying to compute.
type funcFlagsAnalyzer struct {
	fn     *ir.Func
	nstate map[ir.Node]pstate
	noInfo bool // set if we see something inscrutable/un-analyzable
}

// pstate keeps track of the disposition of a given node and its
// children with respect to panic/exit calls.
type pstate int

const (
	psNoInfo     pstate = iota // nothing interesting about this node
	psCallsPanic               // node causes call to panic or os.Exit
	psMayReturn                // executing node may trigger a "return" stmt
	psTop                      // dataflow lattice "top" element
)

func makeFuncFlagsAnalyzer(fn *ir.Func) *funcFlagsAnalyzer {
	return &funcFlagsAnalyzer{
		fn:     fn,
		nstate: make(map[ir.Node]pstate),
	}
}

// setResults transfers func flag results to 'funcProps'.
func (ffa *funcFlagsAnalyzer) setResults(funcProps *FuncProps) {
	var rv FuncPropBits
	if !ffa.noInfo && ffa.stateForList(ffa.fn.Body) == psCallsPanic {
		rv = FuncPropNeverReturns
	}
	// This is slightly hacky and not at all required, but include a
	// special case for main.main, which often ends in a call to
	// os.Exit. People who write code like this (very common I
	// imagine)
	//
	//   func main() {
	//     rc = perform()
	//     ...
	//     foo()
	//     os.Exit(rc)
	//   }
	//
	// will be constantly surprised when foo() is inlined in many
	// other spots in the program but not in main().
	if isMainMain(ffa.fn) {
		rv &^= FuncPropNeverReturns
	}
	funcProps.Flags = rv
}

func (ffa *funcFlagsAnalyzer) getState(n ir.Node) pstate {
	return ffa.nstate[n]
}

func (ffa *funcFlagsAnalyzer) setState(n ir.Node, st pstate) {
	if st != psNoInfo {
		ffa.nstate[n] = st
	}
}

func (ffa *funcFlagsAnalyzer) updateState(n ir.Node, st pstate) {
	if st == psNoInfo {
		delete(ffa.nstate, n)
	} else {
		ffa.nstate[n] = st
	}
}

func (ffa *funcFlagsAnalyzer) panicPathTable() map[ir.Node]pstate {
	return ffa.nstate
}

// blockCombine merges together states as part of a linear sequence of
// statements, where 'pred' and 'succ' are analysis results for a pair
// of consecutive statements. Examples:
//
//	case 1:             case 2:
//	    panic("foo")      if q { return x }        <-pred
//	    return x          panic("boo")             <-succ
//
// In case 1, since the pred state is "always panic" it doesn't matter
// what the succ state is, hence the state for the combination of the
// two blocks is "always panics". In case 2, because there is a path
// to return that avoids the panic in succ, the state for the
// combination of the two statements is "may return".
func blockCombine(pred, succ pstate) pstate {
	switch succ {
	case psTop:
		return pred
	case psMayReturn:
		if pred == psCallsPanic {
			return psCallsPanic
		}
		return psMayReturn
	case psNoInfo:
		return pred
	case psCallsPanic:
		if pred == psMayReturn {
			return psMayReturn
		}
		return psCallsPanic
	}
	panic("should never execute")
}

// branchCombine combines two states at a control flow branch point where
// either p1 or p2 executes (as in an "if" statement).
func branchCombine(p1, p2 pstate) pstate {
	if p1 == psCallsPanic && p2 == psCallsPanic {
		return psCallsPanic
	}
	if p1 == psMayReturn || p2 == psMayReturn {
		return psMayReturn
	}
	return psNoInfo
}

// stateForList walks through a list of statements and computes the
// state/disposition for the entire list as a whole, as well
// as updating disposition of intermediate nodes.
func (ffa *funcFlagsAnalyzer) stateForList(list ir.Nodes) pstate {
	st := psTop
	// Walk the list backwards so that we can update the state for
	// earlier list elements based on what we find out about their
	// successors. Example:
	//
	//        if ... {
	//  L10:    foo()
	//  L11:    <stmt>
	//  L12:    panic(...)
	//        }
	//
	// After combining the dispositions for line 11 and 12, we want to
	// update the state for the call at line 10 based on that combined
	// disposition (if L11 has no path to "return", then the call at
	// line 10 will be on a panic path).
	for i := len(list) - 1; i >= 0; i-- {
		n := list[i]
		psi := ffa.getState(n)
		if debugTrace&debugTraceFuncFlags != 0 {
			fmt.Fprintf(os.Stderr, "=-= %v: stateForList n=%s ps=%s\n",
				ir.Line(n), n.Op().String(), psi.String())
		}
		st = blockCombine(psi, st)
		ffa.updateState(n, st)
	}
	if st == psTop {
		st = psNoInfo
	}
	return st
}

func isMainMain(fn *ir.Func) bool {
	s := fn.Sym()
	return (s.Pkg.Name == "main" && s.Name == "main")
}

func isWellKnownFunc(s *types.Sym, pkg, name string) bool {
	return s.Pkg.Path == pkg && s.Name == name
}

// isExitCall reports TRUE if the node itself is an unconditional
// call to os.Exit(), a panic, or a function that does likewise.
func isExitCall(n ir.Node) bool {
	if n.Op() != ir.OCALLFUNC {
		return false
	}
	cx := n.(*ir.CallExpr)
	name := ir.StaticCalleeName(cx.Fun)
	if name == nil {
		return false
	}
	s := name.Sym()
	if isWellKnownFunc(s, "os", "Exit") ||
		isWellKnownFunc(s, "runtime", "throw") {
		return true
	}
	if funcProps := propsForFunc(name.Func); funcProps != nil {
		if funcProps.Flags&FuncPropNeverReturns != 0 {
			return true
		}
	}
	return name.Func.NeverReturns()
}

// pessimize is called to record the fact that we saw something in the
// function that renders it entirely impossible to analyze.
func (ffa *funcFlagsAnalyzer) pessimize() {
	ffa.noInfo = true
}

// shouldVisit reports TRUE if this is an interesting node from the
// perspective of computing function flags. NB: due to the fact that
// ir.CallExpr implements the Stmt interface, we wind up visiting
// a lot of nodes that we don't really need to, but these can
// simply be screened out as part of the visit.
func shouldVisit(n ir.Node) bool {
	_, isStmt := n.(ir.Stmt)
	return n.Op() != ir.ODCL &&
		(isStmt || n.Op() == ir.OCALLFUNC || n.Op() == ir.OPANIC)
}

// nodeVisitPost helps implement the propAnalyzer interface; when
// called on a given node, it decides the disposition of that node
// based on the state(s) of the node's children.
func (ffa *funcFlagsAnalyzer) nodeVisitPost(n ir.Node) {
	if debugTrace&debugTraceFuncFlags != 0 {
		fmt.Fprintf(os.Stderr, "=+= nodevis %v %s should=%v\n",
			ir.Line(n), n.Op().String(), shouldVisit(n))
	}
	if !shouldVisit(n) {
		return
	}
	var st pstate
	switch n.Op() {
	case ir.OCALLFUNC:
		if isExitCall(n) {
			st = psCallsPanic
		}
	case ir.OPANIC:
		st = psCallsPanic
	case ir.ORETURN:
		st = psMayReturn
	case ir.OBREAK, ir.OCONTINUE:
		// FIXME: this handling of break/continue is sub-optimal; we
		// have them as "mayReturn" in order to help with this case:
		//
		//   for {
		//     if q() { break }
		//     panic(...)
		//   }
		//
		// where the effect of the 'break' is to cause the subsequent
		// panic to be skipped. One possible improvement would be to
		// track whether the currently enclosing loop is a "for {" or
		// a for/range with condition, then use mayReturn only for the
		// former. Note also that "break X" or "continue X" is treated
		// the same as "goto", since we don't have a good way to track
		// the target of the branch.
		st = psMayReturn
		n := n.(*ir.BranchStmt)
		if n.Label != nil {
			ffa.pessimize()
		}
	case ir.OBLOCK:
		n := n.(*ir.BlockStmt)
		st = ffa.stateForList(n.List)
	case ir.OCASE:
		if ccst, ok := n.(*ir.CaseClause); ok {
			st = ffa.stateForList(ccst.Body)
		} else if ccst, ok := n.(*ir.CommClause); ok {
			st = ffa.stateForList(ccst.Body)
		} else {
			panic("unexpected")
		}
	case ir.OIF:
		n := n.(*ir.IfStmt)
		st = branchCombine(ffa.stateForList(n.Body), ffa.stateForList(n.Else))
	case ir.OFOR:
		// Treat for { XXX } like a block.
		// Treat for <cond> { XXX } like an if statement with no else.
		n := n.(*ir.ForStmt)
		bst := ffa.stateForList(n.Body)
		if n.Cond == nil {
			st = bst
		} else {
			if bst == psMayReturn {
				st = psMayReturn
			}
		}
	case ir.ORANGE:
		// Treat for range { XXX } like an if statement with no else.
		n := n.(*ir.RangeStmt)
		if ffa.stateForList(n.Body) == psMayReturn {
			st = psMayReturn
		}
	case ir.OGOTO:
		// punt if we see even one goto. if we built a control
		// flow graph we could do more, but this is just a tree walk.
		ffa.pessimize()
	case ir.OSELECT:
		// process selects for "may return" but not "always panics",
		// the latter case seems very improbable.
		n := n.(*ir.SelectStmt)
		if len(n.Cases) != 0 {
			st = psTop
			for _, c := range n.Cases {
				st = branchCombine(ffa.stateForList(c.Body), st)
			}
		}
	case ir.OSWITCH:
		n := n.(*ir.SwitchStmt)
		if len(n.Cases) != 0 {
			st = psTop
			for _, c := range n.Cases {
				st = branchCombine(ffa.stateForList(c.Body), st)
			}
		}

		st, fall := psTop, psNoInfo
		for i := len(n.Cases) - 1; i >= 0; i-- {
			cas := n.Cases[i]
			cst := ffa.stateForList(cas.Body)
			endsInFallthrough := false
			if len(cas.Body) != 0 {
				endsInFallthrough = cas.Body[0].Op() == ir.OFALL
			}
			if endsInFallthrough {
				cst = blockCombine(cst, fall)
			}
			st = branchCombine(st, cst)
			fall = cst
		}
	case ir.OFALL:
		// Not important.
	case ir.ODCLFUNC, ir.ORECOVER, ir.OAS, ir.OAS2, ir.OAS2FUNC, ir.OASOP,
		ir.OPRINTLN, ir.OPRINT, ir.OLABEL, ir.OCALLINTER, ir.ODEFER,
		ir.OSEND, ir.ORECV, ir.OSELRECV2, ir.OGO, ir.OAPPEND, ir.OAS2DOTTYPE,
		ir.OAS2MAPR, ir.OGETG, ir.ODELETE, ir.OINLMARK, ir.OAS2RECV,
		ir.OMIN, ir.OMAX, ir.OMAKE, ir.OGETCALLERSP:
		// these should all be benign/uninteresting
	case ir.OTAILCALL, ir.OJUMPTABLE, ir.OTYPESW:
		// don't expect to see these at all.
		base.Fatalf("unexpected op %s in func %s",
			n.Op().String(), ir.FuncName(ffa.fn))
	default:
		base.Fatalf("%v: unhandled op %s in func %v",
			ir.Line(n), n.Op().String(), ir.FuncName(ffa.fn))
	}
	if debugTrace&debugTraceFuncFlags != 0 {
		fmt.Fprintf(os.Stderr, "=-= %v: visit n=%s returns %s\n",
			ir.Line(n), n.Op().String(), st.String())
	}
	ffa.setState(n, st)
}

func (ffa *funcFlagsAnalyzer) nodeVisitPre(n ir.Node) {
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/analyze_func_params.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import (
	"cmd/compile/internal/ir"
	"fmt"
	"os"
)

// paramsAnalyzer holds state information for the phase that computes
// flags for a Go functions parameters, for use in inline heuristics.
// Note that the params slice below includes entries for blanks.
type paramsAnalyzer struct {
	fname  string
	values []ParamPropBits
	params []*ir.Name
	top    []bool
	*condLevelTracker
	*nameFinder
}

// getParams returns an *ir.Name slice containing all params for the
// function (plus rcvr as well if applicable).
func getParams(fn *ir.Func) []*ir.Name {
	sig := fn.Type()
	numParams := sig.NumRecvs() + sig.NumParams()
	return fn.Dcl[:numParams]
}

// addParamsAnalyzer creates a new paramsAnalyzer helper object for
// the function fn, appends it to the analyzers list, and returns the
// new list. If the function in question doesn't have any interesting
// parameters then the analyzer list is returned unchanged, and the
// params flags in "fp" are updated accordingly.
func addParamsAnalyzer(fn *ir.Func, analyzers []propAnalyzer, fp *FuncProps, nf *nameFinder) []propAnalyzer {
	pa, props := makeParamsAnalyzer(fn, nf)
	if pa != nil {
		analyzers = append(analyzers, pa)
	} else {
		fp.ParamFlags = props
	}
	return analyzers
}

// makeParamsAnalyzer creates a new helper object to analyze parameters
// of function fn. If the function doesn't have any interesting
// params, a nil helper is returned along with a set of default param
// flags for the func.
func makeParamsAnalyzer(fn *ir.Func, nf *nameFinder) (*paramsAnalyzer, []ParamPropBits) {
	params := getParams(fn) // includes receiver if applicable
	if len(params) == 0 {
		return nil, nil
	}
	vals := make([]ParamPropBits, len(params))
	if fn.Inl == nil {
		return nil, vals
	}
	top := make([]bool, len(params))
	interestingToAnalyze := false
	for i, pn := range params {
		if pn == nil {
			continue
		}
		pt := pn.Type()
		if !pt.IsScalar() && !pt.HasNil() {
			// existing properties not applicable here (for things
			// like structs, arrays, slices, etc).
			continue
		}
		// If param is reassigned, skip it.
		if ir.Reassigned(pn) {
			continue
		}
		top[i] = true
		interestingToAnalyze = true
	}
	if !interestingToAnalyze {
		return nil, vals
	}

	if debugTrace&debugTraceParams != 0 {
		fmt.Fprintf(os.Stderr, "=-= param analysis of func %v:\n",
			fn.Sym().Name)
		for i := range vals {
			n := "_"
			if params[i] != nil {
				n = params[i].Sym().String()
			}
			fmt.Fprintf(os.Stderr, "=-=  %d: %q %s top=%v\n",
				i, n, vals[i].String(), top[i])
		}
	}
	pa := &paramsAnalyzer{
		fname:            fn.Sym().Name,
		values:           vals,
		params:           params,
		top:              top,
		condLevelTracker: new(condLevelTracker),
		nameFinder:       nf,
	}
	return pa, nil
}

func (pa *paramsAnalyzer) setResults(funcProps *FuncProps) {
	funcProps.ParamFlags = pa.values
}

func (pa *paramsAnalyzer) findParamIdx(n *ir.Name) int {
	if n == nil {
		panic("bad")
	}
	for i := range pa.params {
		if pa.params[i] == n {
			return i
		}
	}
	return -1
}

type testfType func(x ir.Node, param *ir.Name, idx int) (bool, bool)

// checkParams invokes function 'testf' on the specified expression 'x'
// for each parameter. If the result is TRUE, it OR's either 'flag' or 'mayflag'
// into the flags for that param, depending on whether we are in a conditional context.
func (pa *paramsAnalyzer) checkParams(x ir.Node, flag ParamPropBits, mayflag ParamPropBits, testf testfType) {
	for idx, p := range pa.params {
		if !pa.top[idx] && pa.values[idx] == ParamNoInfo {
			continue
		}
		result, may := testf(x, p, idx)
		if debugTrace&debugTraceParams != 0 {
			fmt.Fprintf(os.Stderr, "=-= test expr %v param %s result=%v flag=%s\n", x, p.Sym().Name, result, flag.String())
		}
		if result {
			v := flag
			if pa.condLevel != 0 || may {
				v = mayflag
			}
			pa.values[idx] |= v
			pa.top[idx] = false
		}
	}
}

// foldCheckParams checks expression 'x' (an 'if' condition or
// 'switch' stmt expr) to see if the expr would fold away if a
// specific parameter had a constant value.
func (pa *paramsAnalyzer) foldCheckParams(x ir.Node) {
	pa.checkParams(x, ParamFeedsIfOrSwitch, ParamMayFeedIfOrSwitch,
		func(x ir.Node, p *ir.Name, idx int) (bool, bool) {
			return ShouldFoldIfNameConstant(x, []*ir.Name{p}), false
		})
}

// callCheckParams examines the target of call expression 'ce' to see
// if it is making a call to the value passed in for some parameter.
func (pa *paramsAnalyzer) callCheckParams(ce *ir.CallExpr) {
	switch ce.Op() {
	case ir.OCALLINTER:
		if ce.Op() != ir.OCALLINTER {
			return
		}
		sel := ce.Fun.(*ir.SelectorExpr)
		r := pa.staticValue(sel.X)
		if r.Op() != ir.ONAME {
			return
		}
		name := r.(*ir.Name)
		if name.Class != ir.PPARAM {
			return
		}
		pa.checkParams(r, ParamFeedsInterfaceMethodCall,
			ParamMayFeedInterfaceMethodCall,
			func(x ir.Node, p *ir.Name, idx int) (bool, bool) {
				name := x.(*ir.Name)
				return name == p, false
			})
	case ir.OCALLFUNC:
		if ce.Fun.Op() != ir.ONAME {
			return
		}
		called := ir.StaticValue(ce.Fun)
		if called.Op() != ir.ONAME {
			return
		}
		name := called.(*ir.Name)
		if name.Class == ir.PPARAM {
			pa.checkParams(called, ParamFeedsIndirectCall,
				ParamMayFeedIndirectCall,
				func(x ir.Node, p *ir.Name, idx int) (bool, bool) {
					name := x.(*ir.Name)
					return name == p, false
				})
		} else {
			cname := pa.funcName(called)
			if cname != nil {
				pa.deriveFlagsFromCallee(ce, cname.Func)
			}
		}
	}
}

// deriveFlagsFromCallee tries to derive flags for the current
// function based on a call this function makes to some other
// function. Example:
//
//	/* Simple */                /* Derived from callee */
//	func foo(f func(int)) {     func foo(f func(int)) {
//	  f(2)                        bar(32, f)
//	}                           }
//	                            func bar(x int, f func()) {
//	                              f(x)
//	                            }
//
// Here we can set the "param feeds indirect call" flag for
// foo's param 'f' since we know that bar has that flag set for
// its second param, and we're passing that param a function.
func (pa *paramsAnalyzer) deriveFlagsFromCallee(ce *ir.CallExpr, callee *ir.Func) {
	calleeProps := propsForFunc(callee)
	if calleeProps == nil {
		return
	}
	if debugTrace&debugTraceParams != 0 {
		fmt.Fprintf(os.Stderr, "=-= callee props for %v:\n%s",
			callee.Sym().Name, calleeProps.String())
	}

	must := []ParamPropBits{ParamFeedsInterfaceMethodCall, ParamFeedsIndirectCall, ParamFeedsIfOrSwitch}
	may := []ParamPropBits{ParamMayFeedInterfaceMethodCall, ParamMayFeedIndirectCall, ParamMayFeedIfOrSwitch}

	for pidx, arg := range ce.Args {
		// Does the callee param have any interesting properties?
		// If not we can skip this one.
		pflag := calleeProps.ParamFlags[pidx]
		if pflag == 0 {
			continue
		}
		// See if one of the caller's parameters is flowing unmodified
		// into this actual expression.
		r := pa.staticValue(arg)
		if r.Op() != ir.ONAME {
			return
		}
		name := r.(*ir.Name)
		if name.Class != ir.PPARAM {
			return
		}
		callerParamIdx := pa.findParamIdx(name)
		// note that callerParamIdx may return -1 in the case where
		// the param belongs not to the current closure func we're
		// analyzing but to an outer enclosing func.
		if callerParamIdx == -1 {
			return
		}
		if pa.params[callerParamIdx] == nil {
			panic("something went wrong")
		}
		if !pa.top[callerParamIdx] &&
			pa.values[callerParamIdx] == ParamNoInfo {
			continue
		}
		if debugTrace&debugTraceParams != 0 {
			fmt.Fprintf(os.Stderr, "=-= pflag for arg %d is %s\n",
				pidx, pflag.String())
		}
		for i := range must {
			mayv := may[i]
			mustv := must[i]
			if pflag&mustv != 0 && pa.condLevel == 0 {
				pa.values[callerParamIdx] |= mustv
			} else if pflag&(mustv|mayv) != 0 {
				pa.values[callerParamIdx] |= mayv
			}
		}
		pa.top[callerParamIdx] = false
	}
}

func (pa *paramsAnalyzer) nodeVisitPost(n ir.Node) {
	if len(pa.values) == 0 {
		return
	}
	pa.condLevelTracker.post(n)
	switch n.Op() {
	case ir.OCALLFUNC:
		ce := n.(*ir.CallExpr)
		pa.callCheckParams(ce)
	case ir.OCALLINTER:
		ce := n.(*ir.CallExpr)
		pa.callCheckParams(ce)
	case ir.OIF:
		ifst := n.(*ir.IfStmt)
		pa.foldCheckParams(ifst.Cond)
	case ir.OSWITCH:
		swst := n.(*ir.SwitchStmt)
		if swst.Tag != nil {
			pa.foldCheckParams(swst.Tag)
		}
	}
}

func (pa *paramsAnalyzer) nodeVisitPre(n ir.Node) {
	if len(pa.values) == 0 {
		return
	}
	pa.condLevelTracker.pre(n)
}

// condLevelTracker helps keeps track very roughly of "level of conditional
// nesting", e.g. how many "if" statements you have to go through to
// get to the point where a given stmt executes. Example:
//
//	                      cond nesting level
//	func foo() {
//	 G = 1                   0
//	 if x < 10 {             0
//	  if y < 10 {            1
//	   G = 0                 2
//	  }
//	 }
//	}
//
// The intent here is to provide some sort of very abstract relative
// hotness metric, e.g. "G = 1" above is expected to be executed more
// often than "G = 0" (in the aggregate, across large numbers of
// functions).
type condLevelTracker struct {
	condLevel int
}

func (c *condLevelTracker) pre(n ir.Node) {
	// Increment level of "conditional testing" if we see
	// an "if" or switch statement, and decrement if in
	// a loop.
	switch n.Op() {
	case ir.OIF, ir.OSWITCH:
		c.condLevel++
	case ir.OFOR, ir.ORANGE:
		c.condLevel--
	}
}

func (c *condLevelTracker) post(n ir.Node) {
	switch n.Op() {
	case ir.OFOR, ir.ORANGE:
		c.condLevel++
	case ir.OIF:
		c.condLevel--
	case ir.OSWITCH:
		c.condLevel--
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/analyze_func_returns.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import (
	"cmd/compile/internal/ir"
	"fmt"
	"go/constant"
	"go/token"
	"os"
)

// resultsAnalyzer stores state information for the process of
// computing flags/properties for the return values of a specific Go
// function, as part of inline heuristics synthesis.
type resultsAnalyzer struct {
	fname           string
	props           []ResultPropBits
	values          []resultVal
	inlineMaxBudget int
	*nameFinder
}

// resultVal captures information about a specific result returned from
// the function we're analyzing; we are interested in cases where
// the func always returns the same constant, or always returns
// the same function, etc. This container stores info on a the specific
// scenarios we're looking for.
type resultVal struct {
	cval    constant.Value
	fn      *ir.Name
	fnClo   bool
	top     bool
	derived bool // see deriveReturnFlagsFromCallee below
}

// addResultsAnalyzer creates a new resultsAnalyzer helper object for
// the function fn, appends it to the analyzers list, and returns the
// new list. If the function in question doesn't have any returns (or
// any interesting returns) then the analyzer list is left as is, and
// the result flags in "fp" are updated accordingly.
func addResultsAnalyzer(fn *ir.Func, analyzers []propAnalyzer, fp *FuncProps, inlineMaxBudget int, nf *nameFinder) []propAnalyzer {
	ra, props := makeResultsAnalyzer(fn, inlineMaxBudget, nf)
	if ra != nil {
		analyzers = append(analyzers, ra)
	} else {
		fp.ResultFlags = props
	}
	return analyzers
}

// makeResultsAnalyzer creates a new helper object to analyze results
// in function fn. If the function doesn't have any interesting
// results, a nil helper is returned along with a set of default
// result flags for the func.
func makeResultsAnalyzer(fn *ir.Func, inlineMaxBudget int, nf *nameFinder) (*resultsAnalyzer, []ResultPropBits) {
	results := fn.Type().Results()
	if len(results) == 0 {
		return nil, nil
	}
	props := make([]ResultPropBits, len(results))
	if fn.Inl == nil {
		return nil, props
	}
	vals := make([]resultVal, len(results))
	interestingToAnalyze := false
	for i := range results {
		rt := results[i].Type
		if !rt.IsScalar() && !rt.HasNil() {
			// existing properties not applicable here (for things
			// like structs, arrays, slices, etc).
			continue
		}
		// set the "top" flag (as in "top element of data flow lattice")
		// meaning "we have no info yet, but we might later on".
		vals[i].top = true
		interestingToAnalyze = true
	}
	if !interestingToAnalyze {
		return nil, props
	}
	ra := &resultsAnalyzer{
		props:           props,
		values:          vals,
		inlineMaxBudget: inlineMaxBudget,
		nameFinder:      nf,
	}
	return ra, nil
}

// setResults transfers the calculated result properties for this
// function to 'funcProps'.
func (ra *resultsAnalyzer) setResults(funcProps *FuncProps) {
	// Promote ResultAlwaysSameFunc to ResultAlwaysSameInlinableFunc
	for i := range ra.values {
		if ra.props[i] == ResultAlwaysSameFunc && !ra.values[i].derived {
			f := ra.values[i].fn.Func
			// HACK: in order to allow for call site score
			// adjustments, we used a relaxed inline budget in
			// determining inlinability. For the check below, however,
			// we want to know is whether the func in question is
			// likely to be inlined, as opposed to whether it might
			// possibly be inlined if all the right score adjustments
			// happened, so do a simple check based on the cost.
			if f.Inl != nil && f.Inl.Cost <= int32(ra.inlineMaxBudget) {
				ra.props[i] = ResultAlwaysSameInlinableFunc
			}
		}
	}
	funcProps.ResultFlags = ra.props
}

func (ra *resultsAnalyzer) pessimize() {
	for i := range ra.props {
		ra.props[i] = ResultNoInfo
	}
}

func (ra *resultsAnalyzer) nodeVisitPre(n ir.Node) {
}

func (ra *resultsAnalyzer) nodeVisitPost(n ir.Node) {
	if len(ra.values) == 0 {
		return
	}
	if n.Op() != ir.ORETURN {
		return
	}
	if debugTrace&debugTraceResults != 0 {
		fmt.Fprintf(os.Stderr, "=+= returns nodevis %v %s\n",
			ir.Line(n), n.Op().String())
	}

	// No support currently for named results, so if we see an empty
	// "return" stmt, be conservative.
	rs := n.(*ir.ReturnStmt)
	if len(rs.Results) != len(ra.values) {
		ra.pessimize()
		return
	}
	for i, r := range rs.Results {
		ra.analyzeResult(i, r)
	}
}

// analyzeResult examines the expression 'n' being returned as the
// 'ii'th argument in some return statement to see whether has
// interesting characteristics (for example, returns a constant), then
// applies a dataflow "meet" operation to combine this result with any
// previous result (for the given return slot) that we've already
// processed.
func (ra *resultsAnalyzer) analyzeResult(ii int, n ir.Node) {
	isAllocMem := ra.isAllocatedMem(n)
	isConcConvItf := ra.isConcreteConvIface(n)
	constVal := ra.constValue(n)
	isConst := (constVal != nil)
	isNil := ra.isNil(n)
	rfunc := ra.funcName(n)
	isFunc := (rfunc != nil)
	isClo := (rfunc != nil && rfunc.Func.OClosure != nil)
	curp := ra.props[ii]
	dprops, isDerivedFromCall := ra.deriveReturnFlagsFromCallee(n)
	newp := ResultNoInfo
	var newcval constant.Value
	var newfunc *ir.Name

	if debugTrace&debugTraceResults != 0 {
		fmt.Fprintf(os.Stderr, "=-= %v: analyzeResult n=%s ismem=%v isconcconv=%v isconst=%v isnil=%v isfunc=%v isclo=%v\n", ir.Line(n), n.Op().String(), isAllocMem, isConcConvItf, isConst, isNil, isFunc, isClo)
	}

	if ra.values[ii].top {
		ra.values[ii].top = false
		// this is the first return we've seen; record
		// whatever properties it has.
		switch {
		case isAllocMem:
			newp = ResultIsAllocatedMem
		case isConcConvItf:
			newp = ResultIsConcreteTypeConvertedToInterface
		case isFunc:
			newp = ResultAlwaysSameFunc
			newfunc = rfunc
		case isConst:
			newp = ResultAlwaysSameConstant
			newcval = constVal
		case isNil:
			newp = ResultAlwaysSameConstant
			newcval = nil
		case isDerivedFromCall:
			newp = dprops
			ra.values[ii].derived = true
		}
	} else {
		if !ra.values[ii].derived {
			// this is not the first return we've seen; apply
			// what amounts of a "meet" operator to combine
			// the properties we see here with what we saw on
			// the previous returns.
			switch curp {
			case ResultIsAllocatedMem:
				if isAllocMem {
					newp = ResultIsAllocatedMem
				}
			case ResultIsConcreteTypeConvertedToInterface:
				if isConcConvItf {
					newp = ResultIsConcreteTypeConvertedToInterface
				}
			case ResultAlwaysSameConstant:
				if isNil && ra.values[ii].cval == nil {
					newp = ResultAlwaysSameConstant
					newcval = nil
				} else if isConst && constant.Compare(constVal, token.EQL, ra.values[ii].cval) {
					newp = ResultAlwaysSameConstant
					newcval = constVal
				}
			case ResultAlwaysSameFunc:
				if isFunc && isSameFuncName(rfunc, ra.values[ii].fn) {
					newp = ResultAlwaysSameFunc
					newfunc = rfunc
				}
			}
		}
	}
	ra.values[ii].fn = newfunc
	ra.values[ii].fnClo = isClo
	ra.values[ii].cval = newcval
	ra.props[ii] = newp

	if debugTrace&debugTraceResults != 0 {
		fmt.Fprintf(os.Stderr, "=-= %v: analyzeResult newp=%s\n",
			ir.Line(n), newp)
	}
}

// deriveReturnFlagsFromCallee tries to set properties for a given
// return result where we're returning call expression; return value
// is a return property value and a boolean indicating whether the
// prop is valid. Examples:
//
//	func foo() int { return bar() }
//	func bar() int { return 42 }
//	func blix() int { return 43 }
//	func two(y int) int {
//	  if y < 0 { return bar() } else { return blix() }
//	}
//
// Since "foo" always returns the result of a call to "bar", we can
// set foo's return property to that of bar. In the case of "two", however,
// even though each return path returns a constant, we don't know
// whether the constants are identical, hence we need to be conservative.
func (ra *resultsAnalyzer) deriveReturnFlagsFromCallee(n ir.Node) (ResultPropBits, bool) {
	if n.Op() != ir.OCALLFUNC {
		return 0, false
	}
	ce := n.(*ir.CallExpr)
	if ce.Fun.Op() != ir.ONAME {
		return 0, false
	}
	called := ir.StaticValue(ce.Fun)
	if called.Op() != ir.ONAME {
		return 0, false
	}
	cname := ra.funcName(called)
	if cname == nil {
		return 0, false
	}
	calleeProps := propsForFunc(cname.Func)
	if calleeProps == nil {
		return 0, false
	}
	if len(calleeProps.ResultFlags) != 1 {
		return 0, false
	}
	return calleeProps.ResultFlags[0], true
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/callsite.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/internal/src"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// CallSite records useful information about a potentially inlinable
// (direct) function call. "Callee" is the target of the call, "Call"
// is the ir node corresponding to the call itself, "Assign" is
// the top-level assignment statement containing the call (if the call
// appears in the form of a top-level statement, e.g. "x := foo()"),
// "Flags" contains properties of the call that might be useful for
// making inlining decisions, "Score" is the final score assigned to
// the site, and "ID" is a numeric ID for the site within its
// containing function.
type CallSite struct {
	Callee *ir.Func
	Call   *ir.CallExpr
	parent *CallSite
	Assign ir.Node
	Flags  CSPropBits

	ArgProps  []ActualExprPropBits
	Score     int
	ScoreMask scoreAdjustTyp
	ID        uint
	aux       uint8
}

// CallSiteTab is a table of call sites, keyed by call expr.
// Ideally it would be nice to key the table by src.XPos, but
// this results in collisions for calls on very long lines (the
// front end saturates column numbers at 255). We also wind up
// with many calls that share the same auto-generated pos.
type CallSiteTab map[*ir.CallExpr]*CallSite

// ActualExprPropBits describes a property of an actual expression (value
// passed to some specific func argument at a call site).
type ActualExprPropBits uint8

const (
	ActualExprConstant ActualExprPropBits = 1 << iota
	ActualExprIsConcreteConvIface
	ActualExprIsFunc
	ActualExprIsInlinableFunc
)

type CSPropBits uint32

const (
	CallSiteInLoop CSPropBits = 1 << iota
	CallSiteOnPanicPath
	CallSiteInInitFunc
)

type csAuxBits uint8

const (
	csAuxInlined = 1 << iota
)

// encodedCallSiteTab is a table keyed by "encoded" callsite
// (stringified src.XPos plus call site ID) mapping to a value of call
// property bits and score.
type encodedCallSiteTab map[string]propsAndScore

type propsAndScore struct {
	props CSPropBits
	score int
	mask  scoreAdjustTyp
}

func (pas propsAndScore) String() string {
	return fmt.Sprintf("P=%s|S=%d|M=%s", pas.props.String(),
		pas.score, pas.mask.String())
}

func (cst CallSiteTab) merge(other CallSiteTab) error {
	for k, v := range other {
		if prev, ok := cst[k]; ok {
			return fmt.Errorf("internal error: collision during call site table merge, fn=%s callsite=%s", prev.Callee.Sym().Name, fmtFullPos(prev.Call.Pos()))
		}
		cst[k] = v
	}
	return nil
}

func fmtFullPos(p src.XPos) string {
	var sb strings.Builder
	sep := ""
	base.Ctxt.AllPos(p, func(pos src.Pos) {
		sb.WriteString(sep)
		sep = "|"
		file := filepath.Base(pos.Filename())
		fmt.Fprintf(&sb, "%s:%d:%d", file, pos.Line(), pos.Col())
	})
	return sb.String()
}

func EncodeCallSiteKey(cs *CallSite) string {
	var sb strings.Builder
	// FIXME: maybe rewrite line offsets relative to function start?
	sb.WriteString(fmtFullPos(cs.Call.Pos()))
	fmt.Fprintf(&sb, "|%d", cs.ID)
	return sb.String()
}

func buildEncodedCallSiteTab(tab CallSiteTab) encodedCallSiteTab {
	r := make(encodedCallSiteTab)
	for _, cs := range tab {
		k := EncodeCallSiteKey(cs)
		r[k] = propsAndScore{
			props: cs.Flags,
			score: cs.Score,
			mask:  cs.ScoreMask,
		}
	}
	return r
}

// dumpCallSiteComments emits comments into the dump file for the
// callsites in the function of interest. If "ecst" is non-nil, we use
// that, otherwise generated a fresh encodedCallSiteTab from "tab".
func dumpCallSiteComments(w io.Writer, tab CallSiteTab, ecst encodedCallSiteTab) {
	if ecst == nil {
		ecst = buildEncodedCallSiteTab(tab)
	}
	tags := make([]string, 0, len(ecst))
	for k := range ecst {
		tags = append(tags, k)
	}
	sort.Strings(tags)
	for _, s := range tags {
		v := ecst[s]
		fmt.Fprintf(w, "// callsite: %s flagstr %q flagval %d score %d mask %d maskstr %q\n", s, v.props.String(), v.props, v.score, v.mask, v.mask.String())
	}
	fmt.Fprintf(w, "// %s\n", csDelimiter)
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/cspropbits_string.go ===
```go
// Code generated by "stringer -bitset -type CSPropBits"; DO NOT EDIT.

package inlheur

import "strconv"
import "bytes"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[CallSiteInLoop-1]
	_ = x[CallSiteOnPanicPath-2]
	_ = x[CallSiteInInitFunc-4]
}

var _CSPropBits_value = [...]uint64{
	0x1, /* CallSiteInLoop */
	0x2, /* CallSiteOnPanicPath */
	0x4, /* CallSiteInInitFunc */
}

const _CSPropBits_name = "CallSiteInLoopCallSiteOnPanicPathCallSiteInInitFunc"

var _CSPropBits_index = [...]uint8{0, 14, 33, 51}

func (i CSPropBits) String() string {
	var b bytes.Buffer

	remain := uint64(i)
	seen := false

	for k, v := range _CSPropBits_value {
		x := _CSPropBits_name[_CSPropBits_index[k]:_CSPropBits_index[k+1]]
		if v == 0 {
			if i == 0 {
				b.WriteString(x)
				return b.String()
			}
			continue
		}
		if (v & remain) == v {
			remain &^= v
			x := _CSPropBits_name[_CSPropBits_index[k]:_CSPropBits_index[k+1]]
			if seen {
				b.WriteString("|")
			}
			seen = true
			b.WriteString(x)
		}
	}
	if remain == 0 {
		return b.String()
	}
	return "CSPropBits(0x" + strconv.FormatInt(int64(i), 16) + ")"
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/eclassify.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import (
	"cmd/compile/internal/ir"
	"fmt"
	"os"
)

// ShouldFoldIfNameConstant analyzes expression tree 'e' to see
// whether it contains only combinations of simple references to all
// of the names in 'names' with selected constants + operators. The
// intent is to identify expression that could be folded away to a
// constant if the value of 'n' were available. Return value is TRUE
// if 'e' does look foldable given the value of 'n', and given that
// 'e' actually makes reference to 'n'. Some examples where the type
// of "n" is int64, type of "s" is string, and type of "p" is *byte:
//
//	Simple?		Expr
//	yes			n<10
//	yes			n*n-100
//	yes			(n < 10 || n > 100) && (n >= 12 || n <= 99 || n != 101)
//	yes			s == "foo"
//	yes			p == nil
//	no			n<foo()
//	no			n<1 || n>m
//	no			float32(n)<1.0
//	no			*p == 1
//	no			1 + 100
//	no			1 / n
//	no			1 + unsafe.Sizeof(n)
//
// To avoid complexities (e.g. nan, inf) we stay way from folding and
// floating point or complex operations (integers, bools, and strings
// only). We also try to be conservative about avoiding any operation
// that might result in a panic at runtime, e.g. for "n" with type
// int64:
//
//	1<<(n-9) < 100/(n<<9999)
//
// we would return FALSE due to the negative shift count and/or
// potential divide by zero.
func ShouldFoldIfNameConstant(n ir.Node, names []*ir.Name) bool {
	cl := makeExprClassifier(names)
	var doNode func(ir.Node) bool
	doNode = func(n ir.Node) bool {
		ir.DoChildren(n, doNode)
		cl.Visit(n)
		return false
	}
	doNode(n)
	if cl.getdisp(n) != exprSimple {
		return false
	}
	for _, v := range cl.names {
		if !v {
			return false
		}
	}
	return true
}

// exprClassifier holds intermediate state about nodes within an
// expression tree being analyzed by ShouldFoldIfNameConstant. Here
// "name" is the name node passed in, and "disposition" stores the
// result of classifying a given IR node.
type exprClassifier struct {
	names       map[*ir.Name]bool
	disposition map[ir.Node]disp
}

type disp int

const (
	// no info on this expr
	exprNoInfo disp = iota

	// expr contains only literals
	exprLiterals

	// expr is legal combination of literals and specified names
	exprSimple
)

func (d disp) String() string {
	switch d {
	case exprNoInfo:
		return "noinfo"
	case exprSimple:
		return "simple"
	case exprLiterals:
		return "literals"
	default:
		return fmt.Sprintf("unknown<%d>", d)
	}
}

func makeExprClassifier(names []*ir.Name) *exprClassifier {
	m := make(map[*ir.Name]bool, len(names))
	for _, n := range names {
		m[n] = false
	}
	return &exprClassifier{
		names:       m,
		disposition: make(map[ir.Node]disp),
	}
}

// Visit sets the classification for 'n' based on the previously
// calculated classifications for n's children, as part of a bottom-up
// walk over an expression tree.
func (ec *exprClassifier) Visit(n ir.Node) {

	ndisp := exprNoInfo

	binparts := func(n ir.Node) (ir.Node, ir.Node) {
		if lex, ok := n.(*ir.LogicalExpr); ok {
			return lex.X, lex.Y
		} else if bex, ok := n.(*ir.BinaryExpr); ok {
			return bex.X, bex.Y
		} else {
			panic("bad")
		}
	}

	t := n.Type()
	if t == nil {
		if debugTrace&debugTraceExprClassify != 0 {
			fmt.Fprintf(os.Stderr, "=-= *** untyped op=%s\n",
				n.Op().String())
		}
	} else if t.IsInteger() || t.IsString() || t.IsBoolean() || t.HasNil() {
		switch n.Op() {
		// FIXME: maybe add support for OADDSTR?
		case ir.ONIL:
			ndisp = exprLiterals

		case ir.OLITERAL:
			if _, ok := n.(*ir.BasicLit); ok {
			} else {
				panic("unexpected")
			}
			ndisp = exprLiterals

		case ir.ONAME:
			nn := n.(*ir.Name)
			if _, ok := ec.names[nn]; ok {
				ndisp = exprSimple
				ec.names[nn] = true
			} else {
				sv := ir.StaticValue(n)
				if sv.Op() == ir.ONAME {
					nn = sv.(*ir.Name)
				}
				if _, ok := ec.names[nn]; ok {
					ndisp = exprSimple
					ec.names[nn] = true
				}
			}

		case ir.ONOT,
			ir.OPLUS,
			ir.ONEG:
			uex := n.(*ir.UnaryExpr)
			ndisp = ec.getdisp(uex.X)

		case ir.OEQ,
			ir.ONE,
			ir.OLT,
			ir.OGT,
			ir.OGE,
			ir.OLE:
			// compare ops
			x, y := binparts(n)
			ndisp = ec.dispmeet(x, y)
			if debugTrace&debugTraceExprClassify != 0 {
				fmt.Fprintf(os.Stderr, "=-= meet(%s,%s) = %s for op=%s\n",
					ec.getdisp(x), ec.getdisp(y), ec.dispmeet(x, y),
					n.Op().String())
			}
		case ir.OLSH,
			ir.ORSH,
			ir.ODIV,
			ir.OMOD:
			x, y := binparts(n)
			if ec.getdisp(y) == exprLiterals {
				ndisp = ec.dispmeet(x, y)
			}

		case ir.OADD,
			ir.OSUB,
			ir.OOR,
			ir.OXOR,
			ir.OMUL,
			ir.OAND,
			ir.OANDNOT,
			ir.OANDAND,
			ir.OOROR:
			x, y := binparts(n)
			if debugTrace&debugTraceExprClassify != 0 {
				fmt.Fprintf(os.Stderr, "=-= meet(%s,%s) = %s for op=%s\n",
					ec.getdisp(x), ec.getdisp(y), ec.dispmeet(x, y),
					n.Op().String())
			}
			ndisp = ec.dispmeet(x, y)
		}
	}

	if debugTrace&debugTraceExprClassify != 0 {
		fmt.Fprintf(os.Stderr, "=-= op=%s disp=%v\n", n.Op().String(),
			ndisp.String())
	}

	ec.disposition[n] = ndisp
}

func (ec *exprClassifier) getdisp(x ir.Node) disp {
	if d, ok := ec.disposition[x]; ok {
		return d
	} else {
		panic("missing node from disp table")
	}
}

// dispmeet performs a "meet" operation on the data flow states of
// node x and y (where the term "meet" is being drawn from traditional
// lattice-theoretical data flow analysis terminology).
func (ec *exprClassifier) dispmeet(x, y ir.Node) disp {
	xd := ec.getdisp(x)
	if xd == exprNoInfo {
		return exprNoInfo
	}
	yd := ec.getdisp(y)
	if yd == exprNoInfo {
		return exprNoInfo
	}
	if xd == exprSimple || yd == exprSimple {
		return exprSimple
	}
	if xd != exprLiterals || yd != exprLiterals {
		panic("unexpected")
	}
	return exprLiterals
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/funcprop_string.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import (
	"fmt"
	"strings"
)

func (fp *FuncProps) String() string {
	return fp.ToString("")
}

func (fp *FuncProps) ToString(prefix string) string {
	var sb strings.Builder
	if fp.Flags != 0 {
		fmt.Fprintf(&sb, "%sFlags %s\n", prefix, fp.Flags)
	}
	flagSliceToSB[ParamPropBits](&sb, fp.ParamFlags,
		prefix, "ParamFlags")
	flagSliceToSB[ResultPropBits](&sb, fp.ResultFlags,
		prefix, "ResultFlags")
	return sb.String()
}

func flagSliceToSB[T interface {
	~uint32
	String() string
}](sb *strings.Builder, sl []T, prefix string, tag string) {
	var sb2 strings.Builder
	foundnz := false
	fmt.Fprintf(&sb2, "%s%s\n", prefix, tag)
	for i, e := range sl {
		if e != 0 {
			foundnz = true
		}
		fmt.Fprintf(&sb2, "%s  %d %s\n", prefix, i, e.String())
	}
	if foundnz {
		sb.WriteString(sb2.String())
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/funcpropbits_string.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by "stringer -bitset -type FuncPropBits"; DO NOT EDIT.

package inlheur

import (
	"bytes"
	"strconv"
)

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[FuncPropNeverReturns-1]
}

var _FuncPropBits_value = [...]uint64{
	0x1, /* FuncPropNeverReturns */
}

const _FuncPropBits_name = "FuncPropNeverReturns"

var _FuncPropBits_index = [...]uint8{0, 20}

func (i FuncPropBits) String() string {
	var b bytes.Buffer

	remain := uint64(i)
	seen := false

	for k, v := range _FuncPropBits_value {
		x := _FuncPropBits_name[_FuncPropBits_index[k]:_FuncPropBits_index[k+1]]
		if v == 0 {
			if i == 0 {
				b.WriteString(x)
				return b.String()
			}
			continue
		}
		if (v & remain) == v {
			remain &^= v
			x := _FuncPropBits_name[_FuncPropBits_index[k]:_FuncPropBits_index[k+1]]
			if seen {
				b.WriteString("|")
			}
			seen = true
			b.WriteString(x)
		}
	}
	if remain == 0 {
		return b.String()
	}
	return "FuncPropBits(0x" + strconv.FormatInt(int64(i), 16) + ")"
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/function_properties.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

// This file defines a set of Go function "properties" intended to
// guide inlining heuristics; these properties may apply to the
// function as a whole, or to one or more function return values or
// parameters.
//
// IMPORTANT: function properties are produced on a "best effort"
// basis, meaning that the code that computes them doesn't verify that
// the properties are guaranteed to be true in 100% of cases. For this
// reason, properties should only be used to drive always-safe
// optimization decisions (e.g. "should I inline this call", or
// "should I unroll this loop") as opposed to potentially unsafe IR
// alterations that could change program semantics (e.g. "can I delete
// this variable" or "can I move this statement to a new location").
//
//----------------------------------------------------------------

// FuncProps describes a set of function or method properties that may
// be useful for inlining heuristics. Here 'Flags' are properties that
// we think apply to the entire function; 'RecvrParamFlags' are
// properties of specific function params (or the receiver), and
// 'ResultFlags' are things properties we think will apply to values
// of specific results. Note that 'ParamFlags' includes and entry for
// the receiver if applicable, and does include etries for blank
// params; for a function such as "func foo(_ int, b byte, _ float32)"
// the length of ParamFlags will be 3.
type FuncProps struct {
	Flags       FuncPropBits
	ParamFlags  []ParamPropBits // slot 0 receiver if applicable
	ResultFlags []ResultPropBits
}

type FuncPropBits uint32

const (
	// Function always panics or invokes os.Exit() or a func that does
	// likewise.
	FuncPropNeverReturns FuncPropBits = 1 << iota
)

type ParamPropBits uint32

const (
	// No info about this param
	ParamNoInfo ParamPropBits = 0

	// Parameter value feeds unmodified into a top-level interface
	// call (this assumes the parameter is of interface type).
	ParamFeedsInterfaceMethodCall ParamPropBits = 1 << iota

	// Parameter value feeds unmodified into an interface call that
	// may be conditional/nested and not always executed (this assumes
	// the parameter is of interface type).
	ParamMayFeedInterfaceMethodCall ParamPropBits = 1 << iota

	// Parameter value feeds unmodified into a top level indirect
	// function call (assumes parameter is of function type).
	ParamFeedsIndirectCall

	// Parameter value feeds unmodified into an indirect function call
	// that is conditional/nested (not guaranteed to execute). Assumes
	// parameter is of function type.
	ParamMayFeedIndirectCall

	// Parameter value feeds unmodified into a top level "switch"
	// statement or "if" statement simple expressions (see more on
	// "simple" expression classification below).
	ParamFeedsIfOrSwitch

	// Parameter value feeds unmodified into a "switch" or "if"
	// statement simple expressions (see more on "simple" expression
	// classification below), where the if/switch is
	// conditional/nested.
	ParamMayFeedIfOrSwitch
)

type ResultPropBits uint32

const (
	// No info about this result
	ResultNoInfo ResultPropBits = 0
	// This result always contains allocated memory.
	ResultIsAllocatedMem ResultPropBits = 1 << iota
	// This result is always a single concrete type that is
	// implicitly converted to interface.
	ResultIsConcreteTypeConvertedToInterface
	// Result is always the same non-composite compile time constant.
	ResultAlwaysSameConstant
	// Result is always the same function or closure.
	ResultAlwaysSameFunc
	// Result is always the same (potentially) inlinable function or closure.
	ResultAlwaysSameInlinableFunc
)

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/names.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import (
	"cmd/compile/internal/ir"
	"go/constant"
)

// nameFinder provides a set of "isXXX" query methods for clients to
// ask whether a given AST node corresponds to a function, a constant
// value, and so on. These methods use an underlying ir.ReassignOracle
// to return more precise results in cases where an "interesting"
// value is assigned to a singly-defined local temp. Example:
//
//	const q = 101
//	fq := func() int { return q }
//	copyOfConstant := q
//	copyOfFunc := f
//	interestingCall(copyOfConstant, copyOfFunc)
//
// A name finder query method invoked on the arguments being passed to
// "interestingCall" will be able detect that 'copyOfConstant' always
// evaluates to a constant (even though it is in fact a PAUTO local
// variable). A given nameFinder can also operate without using
// ir.ReassignOracle (in cases where it is not practical to look
// at the entire function); in such cases queries will still work
// for explicit constant values and functions.
type nameFinder struct {
	ro *ir.ReassignOracle
}

// newNameFinder returns a new nameFinder object with a reassignment
// oracle initialized based on the function fn, or if fn is nil,
// without an underlying ReassignOracle.
func newNameFinder(fn *ir.Func) *nameFinder {
	var ro *ir.ReassignOracle
	if fn != nil {
		ro = &ir.ReassignOracle{}
		ro.Init(fn)
	}
	return &nameFinder{ro: ro}
}

// funcName returns the *ir.Name for the func or method
// corresponding to node 'n', or nil if n can't be proven
// to contain a function value.
func (nf *nameFinder) funcName(n ir.Node) *ir.Name {
	sv := n
	if nf.ro != nil {
		sv = nf.ro.StaticValue(n)
	}
	if name := ir.StaticCalleeName(sv); name != nil {
		return name
	}
	return nil
}

// isAllocatedMem returns true if node n corresponds to a memory
// allocation expression (make, new, or equivalent).
func (nf *nameFinder) isAllocatedMem(n ir.Node) bool {
	sv := n
	if nf.ro != nil {
		sv = nf.ro.StaticValue(n)
	}
	switch sv.Op() {
	case ir.OMAKESLICE, ir.ONEW, ir.OPTRLIT, ir.OSLICELIT:
		return true
	}
	return false
}

// constValue returns the underlying constant.Value for an AST node n
// if n is itself a constant value/expr, or if n is a singly assigned
// local containing constant expr/value (or nil not constant).
func (nf *nameFinder) constValue(n ir.Node) constant.Value {
	sv := n
	if nf.ro != nil {
		sv = nf.ro.StaticValue(n)
	}
	if sv.Op() == ir.OLITERAL {
		return sv.Val()
	}
	return nil
}

// isNil returns whether n is nil (or singly
// assigned local containing nil).
func (nf *nameFinder) isNil(n ir.Node) bool {
	sv := n
	if nf.ro != nil {
		sv = nf.ro.StaticValue(n)
	}
	return sv.Op() == ir.ONIL
}

func (nf *nameFinder) staticValue(n ir.Node) ir.Node {
	if nf.ro == nil {
		return n
	}
	return nf.ro.StaticValue(n)
}

func (nf *nameFinder) reassigned(n *ir.Name) bool {
	if nf.ro == nil {
		return true
	}
	return nf.ro.Reassigned(n)
}

func (nf *nameFinder) isConcreteConvIface(n ir.Node) bool {
	sv := n
	if nf.ro != nil {
		sv = nf.ro.StaticValue(n)
	}
	if sv.Op() != ir.OCONVIFACE {
		return false
	}
	return !sv.(*ir.ConvExpr).X.Type().IsInterface()
}

func isSameFuncName(v1, v2 *ir.Name) bool {
	// NB: there are a few corner cases where pointer equality
	// doesn't work here, but this should be good enough for
	// our purposes here.
	return v1 == v2
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/parampropbits_string.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by "stringer -bitset -type ParamPropBits"; DO NOT EDIT.

package inlheur

import (
	"bytes"
	"strconv"
)

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ParamNoInfo-0]
	_ = x[ParamFeedsInterfaceMethodCall-2]
	_ = x[ParamMayFeedInterfaceMethodCall-4]
	_ = x[ParamFeedsIndirectCall-8]
	_ = x[ParamMayFeedIndirectCall-16]
	_ = x[ParamFeedsIfOrSwitch-32]
	_ = x[ParamMayFeedIfOrSwitch-64]
}

var _ParamPropBits_value = [...]uint64{
	0x0,  /* ParamNoInfo */
	0x2,  /* ParamFeedsInterfaceMethodCall */
	0x4,  /* ParamMayFeedInterfaceMethodCall */
	0x8,  /* ParamFeedsIndirectCall */
	0x10, /* ParamMayFeedIndirectCall */
	0x20, /* ParamFeedsIfOrSwitch */
	0x40, /* ParamMayFeedIfOrSwitch */
}

const _ParamPropBits_name = "ParamNoInfoParamFeedsInterfaceMethodCallParamMayFeedInterfaceMethodCallParamFeedsIndirectCallParamMayFeedIndirectCallParamFeedsIfOrSwitchParamMayFeedIfOrSwitch"

var _ParamPropBits_index = [...]uint8{0, 11, 40, 71, 93, 117, 137, 159}

func (i ParamPropBits) String() string {
	var b bytes.Buffer

	remain := uint64(i)
	seen := false

	for k, v := range _ParamPropBits_value {
		x := _ParamPropBits_name[_ParamPropBits_index[k]:_ParamPropBits_index[k+1]]
		if v == 0 {
			if i == 0 {
				b.WriteString(x)
				return b.String()
			}
			continue
		}
		if (v & remain) == v {
			remain &^= v
			x := _ParamPropBits_name[_ParamPropBits_index[k]:_ParamPropBits_index[k+1]]
			if seen {
				b.WriteString("|")
			}
			seen = true
			b.WriteString(x)
		}
	}
	if remain == 0 {
		return b.String()
	}
	return "ParamPropBits(0x" + strconv.FormatInt(int64(i), 16) + ")"
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/pstate_string.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by "stringer -type pstate"; DO NOT EDIT.

package inlheur

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[psNoInfo-0]
	_ = x[psCallsPanic-1]
	_ = x[psMayReturn-2]
	_ = x[psTop-3]
}

const _pstate_name = "psNoInfopsCallsPanicpsMayReturnpsTop"

var _pstate_index = [...]uint8{0, 8, 20, 31, 36}

func (i pstate) String() string {
	if i < 0 || i >= pstate(len(_pstate_index)-1) {
		return "pstate(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _pstate_name[_pstate_index[i]:_pstate_index[i+1]]
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/resultpropbits_string.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by "stringer -bitset -type ResultPropBits"; DO NOT EDIT.

package inlheur

import (
	"bytes"
	"strconv"
)

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ResultNoInfo-0]
	_ = x[ResultIsAllocatedMem-2]
	_ = x[ResultIsConcreteTypeConvertedToInterface-4]
	_ = x[ResultAlwaysSameConstant-8]
	_ = x[ResultAlwaysSameFunc-16]
	_ = x[ResultAlwaysSameInlinableFunc-32]
}

var _ResultPropBits_value = [...]uint64{
	0x0,  /* ResultNoInfo */
	0x2,  /* ResultIsAllocatedMem */
	0x4,  /* ResultIsConcreteTypeConvertedToInterface */
	0x8,  /* ResultAlwaysSameConstant */
	0x10, /* ResultAlwaysSameFunc */
	0x20, /* ResultAlwaysSameInlinableFunc */
}

const _ResultPropBits_name = "ResultNoInfoResultIsAllocatedMemResultIsConcreteTypeConvertedToInterfaceResultAlwaysSameConstantResultAlwaysSameFuncResultAlwaysSameInlinableFunc"

var _ResultPropBits_index = [...]uint8{0, 12, 32, 72, 96, 116, 145}

func (i ResultPropBits) String() string {
	var b bytes.Buffer

	remain := uint64(i)
	seen := false

	for k, v := range _ResultPropBits_value {
		x := _ResultPropBits_name[_ResultPropBits_index[k]:_ResultPropBits_index[k+1]]
		if v == 0 {
			if i == 0 {
				b.WriteString(x)
				return b.String()
			}
			continue
		}
		if (v & remain) == v {
			remain &^= v
			x := _ResultPropBits_name[_ResultPropBits_index[k]:_ResultPropBits_index[k+1]]
			if seen {
				b.WriteString("|")
			}
			seen = true
			b.WriteString(x)
		}
	}
	if remain == 0 {
		return b.String()
	}
	return "ResultPropBits(0x" + strconv.FormatInt(int64(i), 16) + ")"
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/score_callresult_uses.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import (
	"cmd/compile/internal/ir"
	"fmt"
	"os"
)

// This file contains code to re-score callsites based on how the
// results of the call were used.  Example:
//
//    func foo() {
//       x, fptr := bar()
//       switch x {
//         case 10: fptr = baz()
//         default: blix()
//       }
//       fptr(100)
//     }
//
// The initial scoring pass will assign a score to "bar()" based on
// various criteria, however once the first pass of scoring is done,
// we look at the flags on the result from bar, and check to see
// how those results are used. If bar() always returns the same constant
// for its first result, and if the variable receiving that result
// isn't redefined, and if that variable feeds into an if/switch
// condition, then we will try to adjust the score for "bar" (on the
// theory that if we inlined, we can constant fold / deadcode).

type resultPropAndCS struct {
	defcs *CallSite
	props ResultPropBits
}

type resultUseAnalyzer struct {
	resultNameTab map[*ir.Name]resultPropAndCS
	fn            *ir.Func
	cstab         CallSiteTab
	*condLevelTracker
}

// rescoreBasedOnCallResultUses examines how call results are used,
// and tries to update the scores of calls based on how their results
// are used in the function.
func (csa *callSiteAnalyzer) rescoreBasedOnCallResultUses(fn *ir.Func, resultNameTab map[*ir.Name]resultPropAndCS, cstab CallSiteTab) {
	enableDebugTraceIfEnv()
	rua := &resultUseAnalyzer{
		resultNameTab:    resultNameTab,
		fn:               fn,
		cstab:            cstab,
		condLevelTracker: new(condLevelTracker),
	}
	var doNode func(ir.Node) bool
	doNode = func(n ir.Node) bool {
		rua.nodeVisitPre(n)
		ir.DoChildren(n, doNode)
		rua.nodeVisitPost(n)
		return false
	}
	doNode(fn)
	disableDebugTrace()
}

func (csa *callSiteAnalyzer) examineCallResults(cs *CallSite, resultNameTab map[*ir.Name]resultPropAndCS) map[*ir.Name]resultPropAndCS {
	if debugTrace&debugTraceScoring != 0 {
		fmt.Fprintf(os.Stderr, "=-= examining call results for %q\n",
			EncodeCallSiteKey(cs))
	}

	// Invoke a helper to pick out the specific ir.Name's the results
	// from this call are assigned into, e.g. "x, y := fooBar()". If
	// the call is not part of an assignment statement, or if the
	// variables in question are not newly defined, then we'll receive
	// an empty list here.
	//
	names, autoTemps, props := namesDefined(cs)
	if len(names) == 0 {
		return resultNameTab
	}

	if debugTrace&debugTraceScoring != 0 {
		fmt.Fprintf(os.Stderr, "=-= %d names defined\n", len(names))
	}

	// For each returned value, if the value has interesting
	// properties (ex: always returns the same constant), and the name
	// in question is never redefined, then make an entry in the
	// result table for it.
	const interesting = (ResultIsConcreteTypeConvertedToInterface |
		ResultAlwaysSameConstant | ResultAlwaysSameInlinableFunc | ResultAlwaysSameFunc)
	for idx, n := range names {
		rprop := props.ResultFlags[idx]

		if debugTrace&debugTraceScoring != 0 {
			fmt.Fprintf(os.Stderr, "=-= props for ret %d %q: %s\n",
				idx, n.Sym().Name, rprop.String())
		}

		if rprop&interesting == 0 {
			continue
		}
		if csa.nameFinder.reassigned(n) {
			continue
		}
		if resultNameTab == nil {
			resultNameTab = make(map[*ir.Name]resultPropAndCS)
		} else if _, ok := resultNameTab[n]; ok {
			panic("should never happen")
		}
		entry := resultPropAndCS{
			defcs: cs,
			props: rprop,
		}
		resultNameTab[n] = entry
		if autoTemps[idx] != nil {
			resultNameTab[autoTemps[idx]] = entry
		}
		if debugTrace&debugTraceScoring != 0 {
			fmt.Fprintf(os.Stderr, "=-= add resultNameTab table entry n=%v autotemp=%v props=%s\n", n, autoTemps[idx], rprop.String())
		}
	}
	return resultNameTab
}

// namesDefined returns a list of ir.Name's corresponding to locals
// that receive the results from the call at site 'cs', plus the
// properties object for the called function. If a given result
// isn't cleanly assigned to a newly defined local, the
// slot for that result in the returned list will be nil. Example:
//
//	call                             returned name list
//
//	x := foo()                       [ x ]
//	z, y := bar()                    [ nil, nil ]
//	_, q := baz()                    [ nil, q ]
//
// In the case of a multi-return call, such as "x, y := foo()",
// the pattern we see from the front end will be a call op
// assigning to auto-temps, and then an assignment of the auto-temps
// to the user-level variables. In such cases we return
// first the user-level variable (in the first func result)
// and then the auto-temp name in the second result.
func namesDefined(cs *CallSite) ([]*ir.Name, []*ir.Name, *FuncProps) {
	// If this call doesn't feed into an assignment (and of course not
	// all calls do), then we don't have anything to work with here.
	if cs.Assign == nil {
		return nil, nil, nil
	}
	funcInlHeur, ok := fpmap[cs.Callee]
	if !ok {
		// TODO: add an assert/panic here.
		return nil, nil, nil
	}
	if len(funcInlHeur.props.ResultFlags) == 0 {
		return nil, nil, nil
	}

	// Single return case.
	if len(funcInlHeur.props.ResultFlags) == 1 {
		asgn, ok := cs.Assign.(*ir.AssignStmt)
		if !ok {
			return nil, nil, nil
		}
		// locate name being assigned
		aname, ok := asgn.X.(*ir.Name)
		if !ok {
			return nil, nil, nil
		}
		return []*ir.Name{aname}, []*ir.Name{nil}, funcInlHeur.props
	}

	// Multi-return case
	asgn, ok := cs.Assign.(*ir.AssignListStmt)
	if !ok || !asgn.Def {
		return nil, nil, nil
	}
	userVars := make([]*ir.Name, len(funcInlHeur.props.ResultFlags))
	autoTemps := make([]*ir.Name, len(funcInlHeur.props.ResultFlags))
	for idx, x := range asgn.Lhs {
		if n, ok := x.(*ir.Name); ok {
			userVars[idx] = n
			r := asgn.Rhs[idx]
			if r.Op() == ir.OCONVNOP {
				r = r.(*ir.ConvExpr).X
			}
			if ir.IsAutoTmp(r) {
				autoTemps[idx] = r.(*ir.Name)
			}
			if debugTrace&debugTraceScoring != 0 {
				fmt.Fprintf(os.Stderr, "=-= multi-ret namedef uv=%v at=%v\n",
					x, autoTemps[idx])
			}
		} else {
			return nil, nil, nil
		}
	}
	return userVars, autoTemps, funcInlHeur.props
}

func (rua *resultUseAnalyzer) nodeVisitPost(n ir.Node) {
	rua.condLevelTracker.post(n)
}

func (rua *resultUseAnalyzer) nodeVisitPre(n ir.Node) {
	rua.condLevelTracker.pre(n)
	switch n.Op() {
	case ir.OCALLINTER:
		if debugTrace&debugTraceScoring != 0 {
			fmt.Fprintf(os.Stderr, "=-= rescore examine iface call %v:\n", n)
		}
		rua.callTargetCheckResults(n)
	case ir.OCALLFUNC:
		if debugTrace&debugTraceScoring != 0 {
			fmt.Fprintf(os.Stderr, "=-= rescore examine call %v:\n", n)
		}
		rua.callTargetCheckResults(n)
	case ir.OIF:
		ifst := n.(*ir.IfStmt)
		rua.foldCheckResults(ifst.Cond)
	case ir.OSWITCH:
		swst := n.(*ir.SwitchStmt)
		if swst.Tag != nil {
			rua.foldCheckResults(swst.Tag)
		}

	}
}

// callTargetCheckResults examines a given call to see whether the
// callee expression is potentially an inlinable function returned
// from a potentially inlinable call. Examples:
//
//	Scenario 1: named intermediate
//
//	   fn1 := foo()         conc := bar()
//	   fn1("blah")          conc.MyMethod()
//
//	Scenario 2: returned func or concrete object feeds directly to call
//
//	   foo()("blah")        bar().MyMethod()
//
// In the second case although at the source level the result of the
// direct call feeds right into the method call or indirect call,
// we're relying on the front end having inserted an auto-temp to
// capture the value.
func (rua *resultUseAnalyzer) callTargetCheckResults(call ir.Node) {
	ce := call.(*ir.CallExpr)
	rname := rua.getCallResultName(ce)
	if rname == nil {
		return
	}
	if debugTrace&debugTraceScoring != 0 {
		fmt.Fprintf(os.Stderr, "=-= staticvalue returns %v:\n",
			rname)
	}
	if rname.Class != ir.PAUTO {
		return
	}
	switch call.Op() {
	case ir.OCALLINTER:
		if debugTrace&debugTraceScoring != 0 {
			fmt.Fprintf(os.Stderr, "=-= in %s checking %v for cci prop:\n",
				rua.fn.Sym().Name, rname)
		}
		if cs := rua.returnHasProp(rname, ResultIsConcreteTypeConvertedToInterface); cs != nil {

			adj := returnFeedsConcreteToInterfaceCallAdj
			cs.Score, cs.ScoreMask = adjustScore(adj, cs.Score, cs.ScoreMask)
		}
	case ir.OCALLFUNC:
		if debugTrace&debugTraceScoring != 0 {
			fmt.Fprintf(os.Stderr, "=-= in %s checking %v for samefunc props:\n",
				rua.fn.Sym().Name, rname)
			v, ok := rua.resultNameTab[rname]
			if !ok {
				fmt.Fprintf(os.Stderr, "=-= no entry for %v in rt\n", rname)
			} else {
				fmt.Fprintf(os.Stderr, "=-= props for %v: %q\n", rname, v.props.String())
			}
		}
		if cs := rua.returnHasProp(rname, ResultAlwaysSameInlinableFunc); cs != nil {
			adj := returnFeedsInlinableFuncToIndCallAdj
			cs.Score, cs.ScoreMask = adjustScore(adj, cs.Score, cs.ScoreMask)
		} else if cs := rua.returnHasProp(rname, ResultAlwaysSameFunc); cs != nil {
			adj := returnFeedsFuncToIndCallAdj
			cs.Score, cs.ScoreMask = adjustScore(adj, cs.Score, cs.ScoreMask)

		}
	}
}

// foldCheckResults examines the specified if/switch condition 'cond'
// to see if it refers to locals defined by a (potentially inlinable)
// function call at call site C, and if so, whether 'cond' contains
// only combinations of simple references to all of the names in
// 'names' with selected constants + operators. If these criteria are
// met, then we adjust the score for call site C to reflect the
// fact that inlining will enable deadcode and/or constant propagation.
// Note: for this heuristic to kick in, the names in question have to
// be all from the same callsite. Examples:
//
//	  q, r := baz()	    x, y := foo()
//	  switch q+r {		a, b, c := bar()
//		...			    if x && y && a && b && c {
//	  }					   ...
//					    }
//
// For the call to "baz" above we apply a score adjustment, but not
// for the calls to "foo" or "bar".
func (rua *resultUseAnalyzer) foldCheckResults(cond ir.Node) {
	namesUsed := collectNamesUsed(cond)
	if len(namesUsed) == 0 {
		return
	}
	var cs *CallSite
	for _, n := range namesUsed {
		rpcs, found := rua.resultNameTab[n]
		if !found {
			return
		}
		if cs != nil && rpcs.defcs != cs {
			return
		}
		cs = rpcs.defcs
		if rpcs.props&ResultAlwaysSameConstant == 0 {
			return
		}
	}
	if debugTrace&debugTraceScoring != 0 {
		nls := func(nl []*ir.Name) string {
			r := ""
			for _, n := range nl {
				r += " " + n.Sym().Name
			}
			return r
		}
		fmt.Fprintf(os.Stderr, "=-= calling ShouldFoldIfNameConstant on names={%s} cond=%v\n", nls(namesUsed), cond)
	}

	if !ShouldFoldIfNameConstant(cond, namesUsed) {
		return
	}
	adj := returnFeedsConstToIfAdj
	cs.Score, cs.ScoreMask = adjustScore(adj, cs.Score, cs.ScoreMask)
}

func collectNamesUsed(expr ir.Node) []*ir.Name {
	res := []*ir.Name{}
	ir.Visit(expr, func(n ir.Node) {
		if n.Op() != ir.ONAME {
			return
		}
		nn := n.(*ir.Name)
		if nn.Class != ir.PAUTO {
			return
		}
		res = append(res, nn)
	})
	return res
}

func (rua *resultUseAnalyzer) returnHasProp(name *ir.Name, prop ResultPropBits) *CallSite {
	v, ok := rua.resultNameTab[name]
	if !ok {
		return nil
	}
	if v.props&prop == 0 {
		return nil
	}
	return v.defcs
}

func (rua *resultUseAnalyzer) getCallResultName(ce *ir.CallExpr) *ir.Name {
	var callTarg ir.Node
	if sel, ok := ce.Fun.(*ir.SelectorExpr); ok {
		// method call
		callTarg = sel.X
	} else if ctarg, ok := ce.Fun.(*ir.Name); ok {
		// regular call
		callTarg = ctarg
	} else {
		return nil
	}
	r := ir.StaticValue(callTarg)
	if debugTrace&debugTraceScoring != 0 {
		fmt.Fprintf(os.Stderr, "=-= staticname on %v returns %v:\n",
			callTarg, r)
	}
	if r.Op() == ir.OCALLFUNC {
		// This corresponds to the "x := foo()" case; here
		// ir.StaticValue has brought us all the way back to
		// the call expression itself. We need to back off to
		// the name defined by the call; do this by looking up
		// the callsite.
		ce := r.(*ir.CallExpr)
		cs, ok := rua.cstab[ce]
		if !ok {
			return nil
		}
		names, _, _ := namesDefined(cs)
		if len(names) == 0 {
			return nil
		}
		return names[0]
	} else if r.Op() == ir.ONAME {
		return r.(*ir.Name)
	}
	return nil
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/scoreadjusttyp_string.go ===
```go
// Code generated by "stringer -bitset -type scoreAdjustTyp"; DO NOT EDIT.

package inlheur

import "strconv"
import "bytes"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[panicPathAdj-1]
	_ = x[initFuncAdj-2]
	_ = x[inLoopAdj-4]
	_ = x[passConstToIfAdj-8]
	_ = x[passConstToNestedIfAdj-16]
	_ = x[passConcreteToItfCallAdj-32]
	_ = x[passConcreteToNestedItfCallAdj-64]
	_ = x[passFuncToIndCallAdj-128]
	_ = x[passFuncToNestedIndCallAdj-256]
	_ = x[passInlinableFuncToIndCallAdj-512]
	_ = x[passInlinableFuncToNestedIndCallAdj-1024]
	_ = x[returnFeedsConstToIfAdj-2048]
	_ = x[returnFeedsFuncToIndCallAdj-4096]
	_ = x[returnFeedsInlinableFuncToIndCallAdj-8192]
	_ = x[returnFeedsConcreteToInterfaceCallAdj-16384]
}

var _scoreAdjustTyp_value = [...]uint64{
	0x1,    /* panicPathAdj */
	0x2,    /* initFuncAdj */
	0x4,    /* inLoopAdj */
	0x8,    /* passConstToIfAdj */
	0x10,   /* passConstToNestedIfAdj */
	0x20,   /* passConcreteToItfCallAdj */
	0x40,   /* passConcreteToNestedItfCallAdj */
	0x80,   /* passFuncToIndCallAdj */
	0x100,  /* passFuncToNestedIndCallAdj */
	0x200,  /* passInlinableFuncToIndCallAdj */
	0x400,  /* passInlinableFuncToNestedIndCallAdj */
	0x800,  /* returnFeedsConstToIfAdj */
	0x1000, /* returnFeedsFuncToIndCallAdj */
	0x2000, /* returnFeedsInlinableFuncToIndCallAdj */
	0x4000, /* returnFeedsConcreteToInterfaceCallAdj */
}

const _scoreAdjustTyp_name = "panicPathAdjinitFuncAdjinLoopAdjpassConstToIfAdjpassConstToNestedIfAdjpassConcreteToItfCallAdjpassConcreteToNestedItfCallAdjpassFuncToIndCallAdjpassFuncToNestedIndCallAdjpassInlinableFuncToIndCallAdjpassInlinableFuncToNestedIndCallAdjreturnFeedsConstToIfAdjreturnFeedsFuncToIndCallAdjreturnFeedsInlinableFuncToIndCallAdjreturnFeedsConcreteToInterfaceCallAdj"

var _scoreAdjustTyp_index = [...]uint16{0, 12, 23, 32, 48, 70, 94, 124, 144, 170, 199, 234, 257, 284, 320, 357}

func (i scoreAdjustTyp) String() string {
	var b bytes.Buffer

	remain := uint64(i)
	seen := false

	for k, v := range _scoreAdjustTyp_value {
		x := _scoreAdjustTyp_name[_scoreAdjustTyp_index[k]:_scoreAdjustTyp_index[k+1]]
		if v == 0 {
			if i == 0 {
				b.WriteString(x)
				return b.String()
			}
			continue
		}
		if (v & remain) == v {
			remain &^= v
			x := _scoreAdjustTyp_name[_scoreAdjustTyp_index[k]:_scoreAdjustTyp_index[k+1]]
			if seen {
				b.WriteString("|")
			}
			seen = true
			b.WriteString(x)
		}
	}
	if remain == 0 {
		return b.String()
	}
	return "scoreAdjustTyp(0x" + strconv.FormatInt(int64(i), 16) + ")"
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/scoring.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/pgoir"
	"cmd/compile/internal/types"
	"cmp"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

// These constants enumerate the set of possible ways/scenarios
// in which we'll adjust the score of a given callsite.
type scoreAdjustTyp uint

// These constants capture the various ways in which the inliner's
// scoring phase can adjust a callsite score based on heuristics. They
// fall broadly into three categories:
//
// 1) adjustments based solely on the callsite context (ex: call
// appears on panic path)
//
// 2) adjustments that take into account specific interesting values
// passed at a call site (ex: passing a constant that could result in
// cprop/deadcode in the caller)
//
// 3) adjustments that take into account values returned from the call
// at a callsite (ex: call always returns the same inlinable function,
// and return value flows unmodified into an indirect call)
//
// For categories 2 and 3 above, each adjustment can have either a
// "must" version and a "may" version (but not both). Here the idea is
// that in the "must" version the value flow is unconditional: if the
// callsite executes, then the condition we're interested in (ex:
// param feeding call) is guaranteed to happen. For the "may" version,
// there may be control flow that could cause the benefit to be
// bypassed.
const (
	// Category 1 adjustments (see above)
	panicPathAdj scoreAdjustTyp = (1 << iota)
	initFuncAdj
	inLoopAdj

	// Category 2 adjustments (see above).
	passConstToIfAdj
	passConstToNestedIfAdj
	passConcreteToItfCallAdj
	passConcreteToNestedItfCallAdj
	passFuncToIndCallAdj
	passFuncToNestedIndCallAdj
	passInlinableFuncToIndCallAdj
	passInlinableFuncToNestedIndCallAdj

	// Category 3 adjustments.
	returnFeedsConstToIfAdj
	returnFeedsFuncToIndCallAdj
	returnFeedsInlinableFuncToIndCallAdj
	returnFeedsConcreteToInterfaceCallAdj

	sentinelScoreAdj // sentinel; not a real adjustment
)

// This table records the specific values we use to adjust call
// site scores in a given scenario.
// NOTE: these numbers are chosen very arbitrarily; ideally
// we will go through some sort of turning process to decide
// what value for each one produces the best performance.

var adjValues = map[scoreAdjustTyp]int{
	panicPathAdj:                          40,
	initFuncAdj:                           20,
	inLoopAdj:                             -5,
	passConstToIfAdj:                      -20,
	passConstToNestedIfAdj:                -15,
	passConcreteToItfCallAdj:              -30,
	passConcreteToNestedItfCallAdj:        -25,
	passFuncToIndCallAdj:                  -25,
	passFuncToNestedIndCallAdj:            -20,
	passInlinableFuncToIndCallAdj:         -45,
	passInlinableFuncToNestedIndCallAdj:   -40,
	returnFeedsConstToIfAdj:               -15,
	returnFeedsFuncToIndCallAdj:           -25,
	returnFeedsInlinableFuncToIndCallAdj:  -40,
	returnFeedsConcreteToInterfaceCallAdj: -25,
}

// SetupScoreAdjustments interprets the value of the -d=inlscoreadj
// debugging option, if set. The value of this flag is expected to be
// a series of "/"-separated clauses of the form adj1:value1. Example:
// -d=inlscoreadj=inLoopAdj=0/passConstToIfAdj=-99
func SetupScoreAdjustments() {
	if base.Debug.InlScoreAdj == "" {
		return
	}
	if err := parseScoreAdj(base.Debug.InlScoreAdj); err != nil {
		base.Fatalf("malformed -d=inlscoreadj argument %q: %v",
			base.Debug.InlScoreAdj, err)
	}
}

func adjStringToVal(s string) (scoreAdjustTyp, bool) {
	for adj := scoreAdjustTyp(1); adj < sentinelScoreAdj; adj <<= 1 {
		if adj.String() == s {
			return adj, true
		}
	}
	return 0, false
}

func parseScoreAdj(val string) error {
	clauses := strings.Split(val, "/")
	if len(clauses) == 0 {
		return fmt.Errorf("no clauses")
	}
	for _, clause := range clauses {
		elems := strings.Split(clause, ":")
		if len(elems) < 2 {
			return fmt.Errorf("clause %q: expected colon", clause)
		}
		if len(elems) != 2 {
			return fmt.Errorf("clause %q has %d elements, wanted 2", clause,
				len(elems))
		}
		adj, ok := adjStringToVal(elems[0])
		if !ok {
			return fmt.Errorf("clause %q: unknown adjustment", clause)
		}
		val, err := strconv.Atoi(elems[1])
		if err != nil {
			return fmt.Errorf("clause %q: malformed value: %v", clause, err)
		}
		adjValues[adj] = val
	}
	return nil
}

func adjValue(x scoreAdjustTyp) int {
	if val, ok := adjValues[x]; ok {
		return val
	} else {
		panic("internal error unregistered adjustment type")
	}
}

var mayMustAdj = [...]struct{ may, must scoreAdjustTyp }{
	{may: passConstToNestedIfAdj, must: passConstToIfAdj},
	{may: passConcreteToNestedItfCallAdj, must: passConcreteToItfCallAdj},
	{may: passFuncToNestedIndCallAdj, must: passFuncToNestedIndCallAdj},
	{may: passInlinableFuncToNestedIndCallAdj, must: passInlinableFuncToIndCallAdj},
}

func isMay(x scoreAdjustTyp) bool {
	return mayToMust(x) != 0
}

func isMust(x scoreAdjustTyp) bool {
	return mustToMay(x) != 0
}

func mayToMust(x scoreAdjustTyp) scoreAdjustTyp {
	for _, v := range mayMustAdj {
		if x == v.may {
			return v.must
		}
	}
	return 0
}

func mustToMay(x scoreAdjustTyp) scoreAdjustTyp {
	for _, v := range mayMustAdj {
		if x == v.must {
			return v.may
		}
	}
	return 0
}

// computeCallSiteScore takes a given call site whose ir node is
// 'call' and callee function is 'callee' and with previously computed
// call site properties 'csflags', then computes a score for the
// callsite that combines the size cost of the callee with heuristics
// based on previously computed argument and function properties,
// then stores the score and the adjustment mask in the appropriate
// fields in 'cs'
func (cs *CallSite) computeCallSiteScore(csa *callSiteAnalyzer, calleeProps *FuncProps) {
	callee := cs.Callee
	csflags := cs.Flags
	call := cs.Call

	// Start with the size-based score for the callee.
	score := int(callee.Inl.Cost)
	var tmask scoreAdjustTyp

	if debugTrace&debugTraceScoring != 0 {
		fmt.Fprintf(os.Stderr, "=-= scoring call to %s at %s , initial=%d\n",
			callee.Sym().Name, fmtFullPos(call.Pos()), score)
	}

	// First some score adjustments to discourage inlining in selected cases.
	if csflags&CallSiteOnPanicPath != 0 {
		score, tmask = adjustScore(panicPathAdj, score, tmask)
	}
	if csflags&CallSiteInInitFunc != 0 {
		score, tmask = adjustScore(initFuncAdj, score, tmask)
	}

	// Then adjustments to encourage inlining in selected cases.
	if csflags&CallSiteInLoop != 0 {
		score, tmask = adjustScore(inLoopAdj, score, tmask)
	}

	// Stop here if no callee props.
	if calleeProps == nil {
		cs.Score, cs.ScoreMask = score, tmask
		return
	}

	// Walk through the actual expressions being passed at the call.
	calleeRecvrParms := callee.Type().RecvParams()
	for idx := range call.Args {
		// ignore blanks
		if calleeRecvrParms[idx].Sym == nil ||
			calleeRecvrParms[idx].Sym.IsBlank() {
			continue
		}
		arg := call.Args[idx]
		pflag := calleeProps.ParamFlags[idx]
		if debugTrace&debugTraceScoring != 0 {
			fmt.Fprintf(os.Stderr, "=-= arg %d of %d: val %v flags=%s\n",
				idx, len(call.Args), arg, pflag.String())
		}

		if len(cs.ArgProps) == 0 {
			continue
		}
		argProps := cs.ArgProps[idx]

		if debugTrace&debugTraceScoring != 0 {
			fmt.Fprintf(os.Stderr, "=-= arg %d props %s value %v\n",
				idx, argProps.String(), arg)
		}

		if argProps&ActualExprConstant != 0 {
			if pflag&ParamMayFeedIfOrSwitch != 0 {
				score, tmask = adjustScore(passConstToNestedIfAdj, score, tmask)
			}
			if pflag&ParamFeedsIfOrSwitch != 0 {
				score, tmask = adjustScore(passConstToIfAdj, score, tmask)
			}
		}

		if argProps&ActualExprIsConcreteConvIface != 0 {
			// FIXME: ideally here it would be nice to make a
			// distinction between the inlinable case and the
			// non-inlinable case, but this is hard to do. Example:
			//
			//    type I interface { Tiny() int; Giant() }
			//    type Conc struct { x int }
			//    func (c *Conc) Tiny() int { return 42 }
			//    func (c *Conc) Giant() { <huge amounts of code> }
			//
			//    func passConcToItf(c *Conc) {
			//        makesItfMethodCall(c)
			//    }
			//
			// In the code above, function properties will only tell
			// us that 'makesItfMethodCall' invokes a method on its
			// interface parameter, but we don't know whether it calls
			// "Tiny" or "Giant". If we knew if called "Tiny", then in
			// theory in addition to converting the interface call to
			// a direct call, we could also inline (in which case
			// we'd want to decrease the score even more).
			//
			// One thing we could do (not yet implemented) is iterate
			// through all of the methods of "*Conc" that allow it to
			// satisfy I, and if all are inlinable, then exploit that.
			if pflag&ParamMayFeedInterfaceMethodCall != 0 {
				score, tmask = adjustScore(passConcreteToNestedItfCallAdj, score, tmask)
			}
			if pflag&ParamFeedsInterfaceMethodCall != 0 {
				score, tmask = adjustScore(passConcreteToItfCallAdj, score, tmask)
			}
		}

		if argProps&(ActualExprIsFunc|ActualExprIsInlinableFunc) != 0 {
			mayadj := passFuncToNestedIndCallAdj
			mustadj := passFuncToIndCallAdj
			if argProps&ActualExprIsInlinableFunc != 0 {
				mayadj = passInlinableFuncToNestedIndCallAdj
				mustadj = passInlinableFuncToIndCallAdj
			}
			if pflag&ParamMayFeedIndirectCall != 0 {
				score, tmask = adjustScore(mayadj, score, tmask)
			}
			if pflag&ParamFeedsIndirectCall != 0 {
				score, tmask = adjustScore(mustadj, score, tmask)
			}
		}
	}

	cs.Score, cs.ScoreMask = score, tmask
}

func adjustScore(typ scoreAdjustTyp, score int, mask scoreAdjustTyp) (int, scoreAdjustTyp) {

	if isMust(typ) {
		if mask&typ != 0 {
			return score, mask
		}
		may := mustToMay(typ)
		if mask&may != 0 {
			// promote may to must, so undo may
			score -= adjValue(may)
			mask &^= may
		}
	} else if isMay(typ) {
		must := mayToMust(typ)
		if mask&(must|typ) != 0 {
			return score, mask
		}
	}
	if mask&typ == 0 {
		if debugTrace&debugTraceScoring != 0 {
			fmt.Fprintf(os.Stderr, "=-= applying adj %d for %s\n",
				adjValue(typ), typ.String())
		}
		score += adjValue(typ)
		mask |= typ
	}
	return score, mask
}

var resultFlagToPositiveAdj map[ResultPropBits]scoreAdjustTyp
var paramFlagToPositiveAdj map[ParamPropBits]scoreAdjustTyp

func setupFlagToAdjMaps() {
	resultFlagToPositiveAdj = map[ResultPropBits]scoreAdjustTyp{
		ResultIsAllocatedMem:     returnFeedsConcreteToInterfaceCallAdj,
		ResultAlwaysSameFunc:     returnFeedsFuncToIndCallAdj,
		ResultAlwaysSameConstant: returnFeedsConstToIfAdj,
	}
	paramFlagToPositiveAdj = map[ParamPropBits]scoreAdjustTyp{
		ParamMayFeedInterfaceMethodCall: passConcreteToNestedItfCallAdj,
		ParamFeedsInterfaceMethodCall:   passConcreteToItfCallAdj,
		ParamMayFeedIndirectCall:        passInlinableFuncToNestedIndCallAdj,
		ParamFeedsIndirectCall:          passInlinableFuncToIndCallAdj,
	}
}

// LargestNegativeScoreAdjustment tries to estimate the largest possible
// negative score adjustment that could be applied to a call of the
// function with the specified props. Example:
//
//	func foo() {                  func bar(x int, p *int) int {
//	   ...                          if x < 0 { *p = x }
//	}                               return 99
//	                              }
//
// Function 'foo' above on the left has no interesting properties,
// thus as a result the most we'll adjust any call to is the value for
// "call in loop". If the calculated cost of the function is 150, and
// the in-loop adjustment is 5 (for example), then there is not much
// point treating it as inlinable. On the other hand "bar" has a param
// property (parameter "x" feeds unmodified to an "if" statement) and
// a return property (always returns same constant) meaning that a
// given call _could_ be rescored down as much as -35 points-- thus if
// the size of "bar" is 100 (for example) then there is at least a
// chance that scoring will enable inlining.
func LargestNegativeScoreAdjustment(fn *ir.Func, props *FuncProps) int {
	if resultFlagToPositiveAdj == nil {
		setupFlagToAdjMaps()
	}
	var tmask scoreAdjustTyp
	score := adjValues[inLoopAdj] // any call can be in a loop
	for _, pf := range props.ParamFlags {
		if adj, ok := paramFlagToPositiveAdj[pf]; ok {
			score, tmask = adjustScore(adj, score, tmask)
		}
	}
	for _, rf := range props.ResultFlags {
		if adj, ok := resultFlagToPositiveAdj[rf]; ok {
			score, tmask = adjustScore(adj, score, tmask)
		}
	}

	if debugTrace&debugTraceScoring != 0 {
		fmt.Fprintf(os.Stderr, "=-= largestScore(%v) is %d\n",
			fn, score)
	}

	return score
}

// callSiteTab contains entries for each call in the function
// currently being processed by InlineCalls; this variable will either
// be set to 'cstabCache' below (for non-inlinable routines) or to the
// local 'cstab' entry in the fnInlHeur object for inlinable routines.
//
// NOTE: this assumes that inlining operations are happening in a serial,
// single-threaded fashion,f which is true today but probably won't hold
// in the future (for example, we might want to score the callsites
// in multiple functions in parallel); if the inliner evolves in this
// direction we'll need to come up with a different approach here.
var callSiteTab CallSiteTab

// scoreCallsCache caches a call site table and call site list between
// invocations of ScoreCalls so that we can reuse previously allocated
// storage.
var scoreCallsCache scoreCallsCacheType

type scoreCallsCacheType struct {
	tab CallSiteTab
	csl []*CallSite
}

// ScoreCalls assigns numeric scores to each of the callsites in
// function 'fn'; the lower the score, the more helpful we think it
// will be to inline.
//
// Unlike a lot of the other inline heuristics machinery, callsite
// scoring can't be done as part of the CanInline call for a function,
// due to fact that we may be working on a non-trivial SCC. So for
// example with this SCC:
//
//	func foo(x int) {           func bar(x int, f func()) {
//	  if x != 0 {                  f()
//	    bar(x, func(){})           foo(x-1)
//	  }                         }
//	}
//
// We don't want to perform scoring for the 'foo' call in "bar" until
// after foo has been analyzed, but it's conceivable that CanInline
// might visit bar before foo for this SCC.
func ScoreCalls(fn *ir.Func) {
	if len(fn.Body) == 0 {
		return
	}
	enableDebugTraceIfEnv()

	nameFinder := newNameFinder(fn)

	if debugTrace&debugTraceScoring != 0 {
		fmt.Fprintf(os.Stderr, "=-= ScoreCalls(%v)\n", ir.FuncName(fn))
	}

	// If this is an inlinable function, use the precomputed
	// call site table for it. If the function wasn't an inline
	// candidate, collect a callsite table for it now.
	var cstab CallSiteTab
	if funcInlHeur, ok := fpmap[fn]; ok {
		cstab = funcInlHeur.cstab
	} else {
		if len(scoreCallsCache.tab) != 0 {
			panic("missing call to ScoreCallsCleanup")
		}
		if scoreCallsCache.tab == nil {
			scoreCallsCache.tab = make(CallSiteTab)
		}
		if debugTrace&debugTraceScoring != 0 {
			fmt.Fprintf(os.Stderr, "=-= building cstab for non-inl func %s\n",
				ir.FuncName(fn))
		}
		cstab = computeCallSiteTable(fn, fn.Body, scoreCallsCache.tab, nil, 0,
			nameFinder)
	}

	csa := makeCallSiteAnalyzer(fn)
	const doCallResults = true
	csa.scoreCallsRegion(fn, fn.Body, cstab, doCallResults, nil)

	disableDebugTrace()
}

// scoreCallsRegion assigns numeric scores to each of the callsites in
// region 'region' within function 'fn'. This can be called on
// an entire function, or with 'region' set to a chunk of
// code corresponding to an inlined call.
func (csa *callSiteAnalyzer) scoreCallsRegion(fn *ir.Func, region ir.Nodes, cstab CallSiteTab, doCallResults bool, ic *ir.InlinedCallExpr) {
	if debugTrace&debugTraceScoring != 0 {
		fmt.Fprintf(os.Stderr, "=-= scoreCallsRegion(%v, %s) len(cstab)=%d\n",
			ir.FuncName(fn), region[0].Op().String(), len(cstab))
	}

	// Sort callsites to avoid any surprises with non deterministic
	// map iteration order (this is probably not needed, but here just
	// in case).
	csl := scoreCallsCache.csl[:0]
	for _, cs := range cstab {
		csl = append(csl, cs)
	}
	scoreCallsCache.csl = csl[:0]
	slices.SortFunc(csl, func(a, b *CallSite) int {
		return cmp.Compare(a.ID, b.ID)
	})

	// Score each call site.
	var resultNameTab map[*ir.Name]resultPropAndCS
	for _, cs := range csl {
		var cprops *FuncProps
		fihcprops := false
		desercprops := false
		if funcInlHeur, ok := fpmap[cs.Callee]; ok {
			cprops = funcInlHeur.props
			fihcprops = true
		} else if cs.Callee.Inl != nil {
			cprops = DeserializeFromString(cs.Callee.Inl.Properties)
			desercprops = true
		} else {
			if base.Debug.DumpInlFuncProps != "" {
				fmt.Fprintf(os.Stderr, "=-= *** unable to score call to %s from %s\n", cs.Callee.Sym().Name, fmtFullPos(cs.Call.Pos()))
				panic("should never happen")
			} else {
				continue
			}
		}
		cs.computeCallSiteScore(csa, cprops)

		if doCallResults {
			if debugTrace&debugTraceScoring != 0 {
				fmt.Fprintf(os.Stderr, "=-= examineCallResults at %s: flags=%d score=%d funcInlHeur=%v deser=%v\n", fmtFullPos(cs.Call.Pos()), cs.Flags, cs.Score, fihcprops, desercprops)
			}
			resultNameTab = csa.examineCallResults(cs, resultNameTab)
		}

		if debugTrace&debugTraceScoring != 0 {
			fmt.Fprintf(os.Stderr, "=-= scoring call at %s: flags=%d score=%d funcInlHeur=%v deser=%v\n", fmtFullPos(cs.Call.Pos()), cs.Flags, cs.Score, fihcprops, desercprops)
		}
	}

	if resultNameTab != nil {
		csa.rescoreBasedOnCallResultUses(fn, resultNameTab, cstab)
	}

	disableDebugTrace()

	if ic != nil && callSiteTab != nil {
		// Integrate the calls from this cstab into the table for the caller.
		if err := callSiteTab.merge(cstab); err != nil {
			base.FatalfAt(ic.Pos(), "%v", err)
		}
	} else {
		callSiteTab = cstab
	}
}

// ScoreCallsCleanup resets the state of the callsite cache
// once ScoreCalls is done with a function.
func ScoreCallsCleanup() {
	if base.Debug.DumpInlCallSiteScores != 0 {
		if allCallSites == nil {
			allCallSites = make(CallSiteTab)
		}
		for call, cs := range callSiteTab {
			allCallSites[call] = cs
		}
	}
	clear(scoreCallsCache.tab)
}

// GetCallSiteScore returns the previously calculated score for call
// within fn.
func GetCallSiteScore(fn *ir.Func, call *ir.CallExpr) (int, bool) {
	if funcInlHeur, ok := fpmap[fn]; ok {
		if cs, ok := funcInlHeur.cstab[call]; ok {
			return cs.Score, true
		}
	}
	if cs, ok := callSiteTab[call]; ok {
		return cs.Score, true
	}
	return 0, false
}

// BudgetExpansion returns the amount to relax/expand the base
// inlining budget when the new inliner is turned on; the inliner
// will add the returned value to the hairiness budget.
//
// Background: with the new inliner, the score for a given callsite
// can be adjusted down by some amount due to heuristics, however we
// won't know whether this is going to happen until much later after
// the CanInline call. This function returns the amount to relax the
// budget initially (to allow for a large score adjustment); later on
// in RevisitInlinability we'll look at each individual function to
// demote it if needed.
func BudgetExpansion(maxBudget int32) int32 {
	if base.Debug.InlBudgetSlack != 0 {
		return int32(base.Debug.InlBudgetSlack)
	}
	// In the default case, return maxBudget, which will effectively
	// double the budget from 80 to 160; this should be good enough
	// for most cases.
	return maxBudget
}

var allCallSites CallSiteTab

// DumpInlCallSiteScores is invoked by the inliner if the debug flag
// "-d=dumpinlcallsitescores" is set; it dumps out a human-readable
// summary of all (potentially) inlinable callsites in the package,
// along with info on call site scoring and the adjustments made to a
// given score. Here profile is the PGO profile in use (may be
// nil), budgetCallback is a callback that can be invoked to find out
// the original pre-adjustment hairiness limit for the function, and
// inlineHotMaxBudget is the constant of the same name used in the
// inliner. Sample output lines:
//
// Score  Adjustment  Status  Callee  CallerPos ScoreFlags
// 115    40          DEMOTED cmd/compile/internal/abi.(*ABIParamAssignment).Offset     expand_calls.go:1679:14|6       panicPathAdj
// 76     -5n         PROMOTED runtime.persistentalloc   mcheckmark.go:48:45|3   inLoopAdj
// 201    0           --- PGO  unicode.DecodeRuneInString        utf8.go:312:30|1
// 7      -5          --- PGO  internal/abi.Name.DataChecked     type.go:625:22|0        inLoopAdj
//
// In the dump above, "Score" is the final score calculated for the
// callsite, "Adjustment" is the amount added to or subtracted from
// the original hairiness estimate to form the score. "Status" shows
// whether anything changed with the site -- did the adjustment bump
// it down just below the threshold ("PROMOTED") or instead bump it
// above the threshold ("DEMOTED"); this will be blank ("---") if no
// threshold was crossed as a result of the heuristics. Note that
// "Status" also shows whether PGO was involved. "Callee" is the name
// of the function called, "CallerPos" is the position of the
// callsite, and "ScoreFlags" is a digest of the specific properties
// we used to make adjustments to callsite score via heuristics.
func DumpInlCallSiteScores(profile *pgoir.Profile, budgetCallback func(fn *ir.Func, profile *pgoir.Profile) (int32, bool)) {

	var indirectlyDueToPromotion func(cs *CallSite) bool
	indirectlyDueToPromotion = func(cs *CallSite) bool {
		bud, _ := budgetCallback(cs.Callee, profile)
		hairyval := cs.Callee.Inl.Cost
		score := int32(cs.Score)
		if hairyval > bud && score <= bud {
			return true
		}
		if cs.parent != nil {
			return indirectlyDueToPromotion(cs.parent)
		}
		return false
	}

	genstatus := func(cs *CallSite) string {
		hairyval := cs.Callee.Inl.Cost
		bud, isPGO := budgetCallback(cs.Callee, profile)
		score := int32(cs.Score)
		st := "---"
		expinl := false
		switch {
		case hairyval <= bud && score <= bud:
			// "Normal" inlined case: hairy val sufficiently low that
			// it would have been inlined anyway without heuristics.
			expinl = true
		case hairyval > bud && score > bud:
			// "Normal" not inlined case: hairy val sufficiently high
			// and scoring didn't lower it.
		case hairyval > bud && score <= bud:
			// Promoted: we would not have inlined it before, but
			// after score adjustment we decided to inline.
			st = "PROMOTED"
			expinl = true
		case hairyval <= bud && score > bud:
			// Demoted: we would have inlined it before, but after
			// score adjustment we decided not to inline.
			st = "DEMOTED"
		}
		inlined := cs.aux&csAuxInlined != 0
		indprom := false
		if cs.parent != nil {
			indprom = indirectlyDueToPromotion(cs.parent)
		}
		if inlined && indprom {
			st += "|INDPROM"
		}
		if inlined && !expinl {
			st += "|[NI?]"
		} else if !inlined && expinl {
			st += "|[IN?]"
		}
		if isPGO {
			st += "|PGO"
		}
		return st
	}

	if base.Debug.DumpInlCallSiteScores != 0 {
		var sl []*CallSite
		for _, cs := range allCallSites {
			sl = append(sl, cs)
		}
		slices.SortFunc(sl, func(a, b *CallSite) int {
			if a.Score != b.Score {
				return cmp.Compare(a.Score, b.Score)
			}
			fni := ir.PkgFuncName(a.Callee)
			fnj := ir.PkgFuncName(b.Callee)
			if fni != fnj {
				return cmp.Compare(fni, fnj)
			}
			ecsi := EncodeCallSiteKey(a)
			ecsj := EncodeCallSiteKey(b)
			return cmp.Compare(ecsi, ecsj)
		})

		mkname := func(fn *ir.Func) string {
			var n string
			if fn == nil || fn.Nname == nil {
				return "<nil>"
			}
			if fn.Sym().Pkg == types.LocalPkg {
				n = "·" + fn.Sym().Name
			} else {
				n = ir.PkgFuncName(fn)
			}
			// don't try to print super-long names
			if len(n) <= 64 {
				return n
			}
			return n[:32] + "..." + n[len(n)-32:len(n)]
		}

		if len(sl) != 0 {
			fmt.Fprintf(os.Stdout, "# scores for package %s\n", types.LocalPkg.Path)
			fmt.Fprintf(os.Stdout, "# Score  Adjustment  Status  Callee  CallerPos Flags ScoreFlags\n")
		}
		for _, cs := range sl {
			hairyval := cs.Callee.Inl.Cost
			adj := int32(cs.Score) - hairyval
			nm := mkname(cs.Callee)
			ecc := EncodeCallSiteKey(cs)
			fmt.Fprintf(os.Stdout, "%d  %d\t%s\t%s\t%s\t%s\n",
				cs.Score, adj, genstatus(cs),
				nm, ecc,
				cs.ScoreMask.String())
		}
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/serialize.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inlheur

import "strings"

func (funcProps *FuncProps) SerializeToString() string {
	if funcProps == nil {
		return ""
	}
	var sb strings.Builder
	writeUleb128(&sb, uint64(funcProps.Flags))
	writeUleb128(&sb, uint64(len(funcProps.ParamFlags)))
	for _, pf := range funcProps.ParamFlags {
		writeUleb128(&sb, uint64(pf))
	}
	writeUleb128(&sb, uint64(len(funcProps.ResultFlags)))
	for _, rf := range funcProps.ResultFlags {
		writeUleb128(&sb, uint64(rf))
	}
	return sb.String()
}

func DeserializeFromString(s string) *FuncProps {
	if len(s) == 0 {
		return nil
	}
	var funcProps FuncProps
	var v uint64
	sl := []byte(s)
	v, sl = readULEB128(sl)
	funcProps.Flags = FuncPropBits(v)
	v, sl = readULEB128(sl)
	funcProps.ParamFlags = make([]ParamPropBits, v)
	for i := range funcProps.ParamFlags {
		v, sl = readULEB128(sl)
		funcProps.ParamFlags[i] = ParamPropBits(v)
	}
	v, sl = readULEB128(sl)
	funcProps.ResultFlags = make([]ResultPropBits, v)
	for i := range funcProps.ResultFlags {
		v, sl = readULEB128(sl)
		funcProps.ResultFlags[i] = ResultPropBits(v)
	}
	return &funcProps
}

func readULEB128(sl []byte) (value uint64, rsl []byte) {
	var shift uint

	for {
		b := sl[0]
		sl = sl[1:]
		value |= (uint64(b&0x7F) << shift)
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return value, sl
}

func writeUleb128(sb *strings.Builder, v uint64) {
	if v < 128 {
		sb.WriteByte(uint8(v))
		return
	}
	more := true
	for more {
		c := uint8(v & 0x7f)
		v >>= 7
		more = v != 0
		if more {
			c |= 0x80
		}
		sb.WriteByte(c)
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/trace_off.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !debugtrace

package inlheur

const debugTrace = 0

func enableDebugTrace(x int) {
}

func enableDebugTraceIfEnv() {
}

func disableDebugTrace() {
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/inlheur/trace_on.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build debugtrace

package inlheur

import (
	"os"
	"strconv"
)

var debugTrace = 0

func enableDebugTrace(x int) {
	debugTrace = x
}

func enableDebugTraceIfEnv() {
	v := os.Getenv("DEBUG_TRACE_INLHEUR")
	if v == "" {
		return
	}
	if v[0] == '*' {
		if !UnitTesting() {
			return
		}
		v = v[1:]
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return
	}
	debugTrace = i
}

func disableDebugTrace() {
	debugTrace = 0
}

```

// === FILE: references/go/src/cmd/compile/internal/inline/interleaved/interleaved.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package interleaved implements the interleaved devirtualization and
// inlining pass.
package interleaved

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/devirtualize"
	"cmd/compile/internal/inline"
	"cmd/compile/internal/inline/inlheur"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/pgoir"
	"cmd/compile/internal/typecheck"
	"fmt"
)

// DevirtualizeAndInlinePackage interleaves devirtualization and inlining on
// all functions within pkg.
func DevirtualizeAndInlinePackage(pkg *ir.Package, profile *pgoir.Profile) {
	if base.Flag.W > 1 {
		for _, fn := range typecheck.Target.Funcs {
			s := fmt.Sprintf("\nbefore devirtualize-and-inline %v", fn.Sym())
			ir.DumpList(s, fn.Body)
		}
	}

	if profile != nil && base.Debug.PGODevirtualize > 0 {
		// TODO(mdempsky): Integrate into DevirtualizeAndInlineFunc below.
		ir.VisitFuncsBottomUp(typecheck.Target.Funcs, func(list []*ir.Func, recursive bool) {
			for _, fn := range list {
				devirtualize.ProfileGuided(fn, profile)
			}
		})
		ir.CurFunc = nil
	}

	if base.Flag.LowerL != 0 {
		inlheur.SetupScoreAdjustments()
	}

	var inlProfile *pgoir.Profile // copy of profile for inlining
	if base.Debug.PGOInline != 0 {
		inlProfile = profile
	}

	// First compute inlinability of all functions in the package.
	inline.CanInlineFuncs(pkg.Funcs, inlProfile)

	inlState := make(map[*ir.Func]*inlClosureState)
	calleeUseCounts := make(map[*ir.Func]int)

	var state devirtualize.State

	// Pre-process all the functions, adding parentheses around call sites and starting their "inl state".
	for _, fn := range typecheck.Target.Funcs {
		bigCaller := base.Flag.LowerL != 0 && inline.IsBigFunc(fn)
		if bigCaller && base.Flag.LowerM > 1 {
			fmt.Printf("%v: function %v considered 'big'; reducing max cost of inlinees\n", ir.Line(fn), fn)
		}

		s := &inlClosureState{bigCaller: bigCaller, profile: profile, fn: fn, callSites: make(map[*ir.ParenExpr]bool), useCounts: calleeUseCounts}
		s.parenthesize()
		inlState[fn] = s

		// Do a first pass at counting call sites.
		for i := range s.parens {
			s.resolve(&state, i)
		}
	}

	ir.VisitFuncsBottomUp(typecheck.Target.Funcs, func(list []*ir.Func, recursive bool) {

		anyInlineHeuristics := false

		// inline heuristics, placed here because they have static state and that's what seems to work.
		for _, fn := range list {
			if base.Flag.LowerL != 0 {
				if inlheur.Enabled() && !fn.Wrapper() {
					inlheur.ScoreCalls(fn)
					anyInlineHeuristics = true
				}
				if base.Debug.DumpInlFuncProps != "" && !fn.Wrapper() {
					inlheur.DumpFuncProps(fn, base.Debug.DumpInlFuncProps)
				}
			}
		}

		if anyInlineHeuristics {
			defer inlheur.ScoreCallsCleanup()
		}

		// Iterate to a fixed point over all the functions.
		done := false
		for !done {
			done = true
			for _, fn := range list {
				s := inlState[fn]

				ir.WithFunc(fn, func() {
					l1 := len(s.parens)
					l0 := 0

					// Batch iterations so that newly discovered call sites are
					// resolved in a batch before inlining attempts.
					// Do this to avoid discovering new closure calls 1 at a time
					// which might cause first call to be seen as a single (high-budget)
					// call before the second is observed.
					for {
						for i := l0; i < l1; i++ { // can't use "range parens" here
							paren := s.parens[i]
							if origCall, inlinedCall := s.edit(&state, i); inlinedCall != nil {
								// Update AST and recursively mark nodes.
								paren.X = inlinedCall
								ir.EditChildren(inlinedCall, s.mark) // mark may append to parens
								state.InlinedCall(s.fn, origCall, inlinedCall)
								done = false
							}
						}
						l0, l1 = l1, len(s.parens)
						if l0 == l1 {
							break
						}
						for i := l0; i < l1; i++ {
							s.resolve(&state, i)
						}

					}

				}) // WithFunc

			}
		}
	})

	ir.CurFunc = nil

	if base.Flag.LowerL != 0 {
		if base.Debug.DumpInlFuncProps != "" {
			inlheur.DumpFuncProps(nil, base.Debug.DumpInlFuncProps)
		}
		if inlheur.Enabled() {
			inline.PostProcessCallSites(inlProfile)
			inlheur.TearDown()
		}
	}

	// remove parentheses
	for _, fn := range typecheck.Target.Funcs {
		inlState[fn].unparenthesize()
	}

}

// DevirtualizeAndInlineFunc interleaves devirtualization and inlining
// on a single function.
func DevirtualizeAndInlineFunc(fn *ir.Func, profile *pgoir.Profile) {
	ir.WithFunc(fn, func() {
		if base.Flag.LowerL != 0 {
			if inlheur.Enabled() && !fn.Wrapper() {
				inlheur.ScoreCalls(fn)
				defer inlheur.ScoreCallsCleanup()
			}
			if base.Debug.DumpInlFuncProps != "" && !fn.Wrapper() {
				inlheur.DumpFuncProps(fn, base.Debug.DumpInlFuncProps)
			}
		}

		bigCaller := base.Flag.LowerL != 0 && inline.IsBigFunc(fn)
		if bigCaller && base.Flag.LowerM > 1 {
			fmt.Printf("%v: function %v considered 'big'; reducing max cost of inlinees\n", ir.Line(fn), fn)
		}

		s := &inlClosureState{bigCaller: bigCaller, profile: profile, fn: fn, callSites: make(map[*ir.ParenExpr]bool), useCounts: make(map[*ir.Func]int)}
		s.parenthesize()
		s.fixpoint()
		s.unparenthesize()
	})
}

type callSite struct {
	fn         *ir.Func
	whichParen int
}

type inlClosureState struct {
	fn        *ir.Func
	profile   *pgoir.Profile
	callSites map[*ir.ParenExpr]bool // callSites[p] == "p appears in parens" (do not append again)
	resolved  []*ir.Func             // for each call in parens, the resolved target of the call
	useCounts map[*ir.Func]int       // shared among all InlClosureStates
	parens    []*ir.ParenExpr
	bigCaller bool
}

// resolve attempts to resolve a call to a potentially inlineable callee
// and updates use counts on the callees.  Returns the call site count
// for that callee.
func (s *inlClosureState) resolve(state *devirtualize.State, i int) (*ir.Func, int) {
	p := s.parens[i]
	if i < len(s.resolved) {
		if callee := s.resolved[i]; callee != nil {
			return callee, s.useCounts[callee]
		}
	}
	n := p.X
	call, ok := n.(*ir.CallExpr)
	if !ok { // previously inlined
		return nil, -1
	}
	devirtualize.StaticCall(state, call)
	if callee := inline.InlineCallTarget(s.fn, call, s.profile); callee != nil {
		for len(s.resolved) <= i {
			s.resolved = append(s.resolved, nil)
		}
		s.resolved[i] = callee
		c := s.useCounts[callee] + 1
		s.useCounts[callee] = c
		return callee, c
	}
	return nil, 0
}

func (s *inlClosureState) edit(state *devirtualize.State, i int) (*ir.CallExpr, *ir.InlinedCallExpr) {
	n := s.parens[i].X
	call, ok := n.(*ir.CallExpr)
	if !ok {
		return nil, nil
	}
	// This is redundant with earlier calls to
	// resolve, but because things can change it
	// must be re-checked.
	callee, count := s.resolve(state, i)
	if count <= 0 {
		return nil, nil
	}
	if inlCall := inline.TryInlineCall(s.fn, call, s.bigCaller, s.profile, count == 1 && callee.ClosureParent != nil); inlCall != nil {
		return call, inlCall
	}
	return nil, nil
}

// Mark inserts parentheses, and is called repeatedly.
// These inserted parentheses mark the call sites where
// inlining will be attempted.
func (s *inlClosureState) mark(n ir.Node) ir.Node {
	// Consider the expression "f(g())". We want to be able to replace
	// "g()" in-place with its inlined representation. But if we first
	// replace "f(...)" with its inlined representation, then "g()" will
	// instead appear somewhere within this new AST.
	//
	// To mitigate this, each matched node n is wrapped in a ParenExpr,
	// so we can reliably replace n in-place by assigning ParenExpr.X.
	// It's safe to use ParenExpr here, because typecheck already
	// removed them all.

	p, _ := n.(*ir.ParenExpr)
	if p != nil && s.callSites[p] {
		return n // already visited n.X before wrapping
	}

	if p != nil {
		n = p.X // in this case p was copied in from a (marked) inlined function, this is a new unvisited node.
	}

	ok := match(n)

	// can't wrap TailCall's child into ParenExpr
	if t, ok := n.(*ir.TailCallStmt); ok {
		ir.EditChildren(t.Call, s.mark)
	} else {
		ir.EditChildren(n, s.mark)
	}

	if ok {
		if p == nil {
			p = ir.NewParenExpr(n.Pos(), n)
			p.SetType(n.Type())
			p.SetTypecheck(n.Typecheck())
			s.callSites[p] = true
		}

		s.parens = append(s.parens, p)
		n = p
	} else if p != nil {
		n = p // didn't change anything, restore n
	}
	return n
}

// parenthesize applies s.mark to all the nodes within
// s.fn to mark calls and simplify rewriting them in place.
func (s *inlClosureState) parenthesize() {
	ir.EditChildren(s.fn, s.mark)
}

func (s *inlClosureState) unparenthesize() {
	if s == nil {
		return
	}
	if len(s.parens) == 0 {
		return // short circuit
	}

	var unparen func(ir.Node) ir.Node
	unparen = func(n ir.Node) ir.Node {
		if paren, ok := n.(*ir.ParenExpr); ok {
			n = paren.X
		}
		ir.EditChildren(n, unparen)
		return n
	}
	ir.EditChildren(s.fn, unparen)
}

// fixpoint repeatedly edits a function until it stabilizes, returning
// whether anything changed in any of the fixpoint iterations.
//
// It applies s.edit(n) to each node n within the parentheses in s.parens.
// If s.edit(n) returns nil, no change is made. Otherwise, the result
// replaces n in fn's body, and fixpoint iterates at least once more.
//
// After an iteration where all edit calls return nil, fixpoint
// returns.
func (s *inlClosureState) fixpoint() bool {
	changed := false
	var state devirtualize.State
	ir.WithFunc(s.fn, func() {
		done := false
		for !done {
			done = true
			for i := 0; i < len(s.parens); i++ { // can't use "range parens" here
				paren := s.parens[i]
				if origCall, inlinedCall := s.edit(&state, i); inlinedCall != nil {
					// Update AST and recursively mark nodes.
					paren.X = inlinedCall
					ir.EditChildren(inlinedCall, s.mark) // mark may append to parens
					state.InlinedCall(s.fn, origCall, inlinedCall)
					done = false
					changed = true
				}
			}
		}
	})
	return changed
}

func match(n ir.Node) bool {
	switch n := n.(type) {
	case *ir.CallExpr:
		return true
	case *ir.TailCallStmt:
		n.Call.NoInline = true // can't inline yet
	}
	return false
}

```

