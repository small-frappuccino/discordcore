# Domain Architecture: cmd/compile/internal/escape

## Layout Topology
```text
cmd/compile/internal/escape/
├── alias.go
├── assign.go
├── call.go
├── escape.go
├── expr.go
├── graph.go
├── leaks.go
├── solve.go
├── stmt.go
└── utils.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/cmd/compile/internal/escape/alias.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/internal/src"
	"fmt"
	"maps"
	"path/filepath"
)

type aliasAnalysis struct {
	// fn is the function being analyzed.
	fn *ir.Func

	// candidateSlices are declared slices that
	// start unaliased and might still be unaliased.
	candidateSlices map[*ir.Name]candidateSlice

	// noAliasAppends are appends that have been
	// proven to use an unaliased slice.
	noAliasAppends []*ir.CallExpr

	// loops is a stack of observed loops,
	// each with a list of candidate appends.
	loops [][]candidateAppend

	// State for optional validation checking (doubleCheck mode):
	processed   map[ir.Node]int // count of times each node was processed, for doubleCheck mode
	doubleCheck bool            // whether to do doubleCheck mode
}

// candidateSlice tracks information about a declared slice
// that might be unaliased.
type candidateSlice struct {
	loopDepth int // depth of loop when slice was declared
}

// candidateAppend tracks information about an OAPPEND that
// might be using an unaliased slice.
type candidateAppend struct {
	s    *ir.Name     // the slice argument in 's = append(s, ...)'
	call *ir.CallExpr // the append call
}

// aliasAnalysis looks for specific patterns of slice usage and proves
// that certain appends are operating on non-aliased slices.
//
// This allows us to emit calls to free the backing arrays for certain
// non-aliased slices at runtime when we know the memory is logically dead.
//
// The analysis is conservative, giving up on any operation we do not
// explicitly understand.
func (aa *aliasAnalysis) analyze(fn *ir.Func) {
	// Walk the function body to discover slice declarations, their uses,
	// and any append that we can prove is using an unaliased slice.
	//
	// An example is:
	//
	//   var s []T
	//   for _, v := range input {
	//      f()
	//      s = append(s, g(v))	   // s cannot be aliased here
	//      h()
	//   }
	//   return s
	//
	// Here, we can prove that the append to s is operating on an unaliased slice,
	// and that conclusion is unaffected by s later being returned and escaping.
	//
	// In contrast, in this example, the aliasing of s in the loop body means the
	// append can be operating on an aliased slice, so we do not record s as unaliased:
	//
	//   var s []T
	//   var alias []T
	//   for _, v := range input {
	//      s = append(s, v)	   // s is aliased on second pass through loop body
	//      alias = s
	//   }
	//
	// Arbitrary uses of s after an append do not affect the aliasing conclusion
	// for that append, but only if the append cannot be revisited at execution time
	// via a loop or goto.
	//
	// We track the loop depth when a slice was declared and verify all uses of a slice
	// are non-aliasing until we return to that depth. In other words, we make sure
	// we have processed any possible execution-time revisiting of the slice prior
	// to making our final determination.
	//
	// This approach helps for example with nested loops, such as:
	//
	//   var s []int
	//   for range 10 {
	//       for range 10 {
	//           s = append(s, 0)  // s is proven as non-aliased here
	//       }
	//   }
	//   alias = s                 // both loops are complete
	//
	// Or in contrast:
	//
	//   var s []int
	//   for range 10 {
	//       for range 10 {
	//           s = append(s, 0)  // s is treated as aliased here
	//       }
	//       alias = s             // aliased, and outermost loop cycles back
	//   }
	//
	// As we walk the function, we look for things like:
	//
	// 1. Slice declarations (currently supporting 'var s []T', 's := make([]T, ...)',
	//    and 's := []T{...}').
	// 2. Appends to a slice of the form 's = append(s, ...)'.
	// 3. Other uses of the slice, which we treat as potential aliasing outside
	//    of a few known safe cases.
	// 4. A start of a loop, which we track in a stack so that
	//    any uses of a slice within a loop body are treated as potential
	//    aliasing, including statements in the loop body after an append.
	//    Candidate appends are stored in the loop stack at the loop depth of their
	//    corresponding slice declaration (rather than the loop depth of the append),
	//    which essentially postpones a decision about the candidate append.
	// 5. An end of a loop, which pops the loop stack and allows us to
	//    conclusively treat candidate appends from the loop body based
	//    on the loop depth of the slice declaration.
	//
	// Note that as we pop a candidate append at the end of a loop, we know
	// its corresponding slice was unaliased throughout the loop being popped
	// if the slice is still in the candidate slice map (without having been
	// removed for potential aliasing), and we know we can make a final decision
	// about a candidate append if we have returned to the loop depth
	// where its slice was declared. In other words, there is no unanalyzed
	// control flow that could take us back at execution-time to the
	// candidate append in the now analyzed loop. This helps for example
	// with nested loops, such as in our examples just above.
	//
	// We give up on a particular candidate slice if we see any use of it
	// that we don't explicitly understand, and we give up on all of
	// our candidate slices if we see any goto or label, which could be
	// unstructured control flow. (TODO(thepudds): we remove the goto/label
	// restriction in a subsequent CL.)
	//
	// Note that the intended use is to indicate that a slice is safe to pass
	// to runtime.freegc, which currently requires that the passed pointer
	// point to the base of its heap object.
	//
	// Therefore, we currently do not allow any re-slicing of the slice, though we could
	// potentially allow s[0:x] or s[:x] or similar. (Slice expressions that alter
	// the capacity might be possible to allow with freegc changes, though they are
	// currently disallowed here like all slice expressions).
	//
	// TODO(thepudds): we could support the slice being used as non-escaping function call parameter
	// but to do that, we need to verify any creation of specials via user code triggers an escape,
	// or mail better runtime.freegc support for specials, or have a temporary compile-time solution
	// for specials. (Currently, this analysis side-steps specials because any use of a slice
	// that might cause a user-created special will cause it to be treated as aliased, and
	// separately, runtime.freegc handles profiling-related specials).

	// Initialize.
	aa.fn = fn
	aa.candidateSlices = make(map[*ir.Name]candidateSlice) // slices that might be unaliased

	// doubleCheck controls whether we do a sanity check of our processing logic
	// by counting each node visited in our main pass, and then comparing those counts
	// against a simple walk at the end. The main intent is to help catch missing
	// any nodes squirreled away in some spot we forgot to examine in our main pass.
	aa.doubleCheck = base.Debug.EscapeAliasCheck > 0
	aa.processed = make(map[ir.Node]int)

	if base.Debug.EscapeAlias >= 2 {
		aa.diag(fn.Pos(), fn, "====== starting func", "======")
	}

	ir.DoChildren(fn, aa.visit)

	for _, call := range aa.noAliasAppends {
		if base.Debug.EscapeAlias >= 1 {
			base.WarnfAt(call.Pos(), "alias analysis: append using non-aliased slice: %v in func %v",
				call, fn)
		}
		if base.Debug.FreeAppend > 0 {
			call.AppendNoAlias = true
		}
	}

	if aa.doubleCheck {
		doubleCheckProcessed(fn, aa.processed)
	}
}

func (aa *aliasAnalysis) visit(n ir.Node) bool {
	if n == nil {
		return false
	}

	if base.Debug.EscapeAlias >= 3 {
		fmt.Printf("%-25s alias analysis: visiting node: %12s  %-18T  %v\n",
			fmtPosShort(n.Pos())+":", n.Op().String(), n, n)
	}

	// As we visit nodes, we want to ensure we handle all children
	// without missing any (through ignorance or future changes).
	// We do this by counting nodes as we visit them or otherwise
	// declare a node to be fully processed.
	//
	// In particular, we want to ensure we don't miss the use
	// of a slice in some expression that might be an aliasing usage.
	//
	// When doubleCheck is enabled, we compare the counts
	// accumulated in our analysis against counts from a trivial walk,
	// failing if there is any mismatch.
	//
	// This call here counts that we have visited this node n
	// via our main visit method. (In contrast, some nodes won't
	// be visited by the main visit method, but instead will be
	// manually marked via countProcessed when we believe we have fully
	// dealt with the node).
	aa.countProcessed(n)

	switch n.Op() {
	case ir.ODCL:
		decl := n.(*ir.Decl)

		if decl.X != nil && decl.X.Type().IsSlice() && decl.X.Class == ir.PAUTO {
			s := decl.X
			if _, ok := aa.candidateSlices[s]; ok {
				base.FatalfAt(n.Pos(), "candidate slice already tracked as candidate: %v", s)
			}
			if base.Debug.EscapeAlias >= 2 {
				aa.diag(n.Pos(), s, "adding candidate slice", "(loop depth: %d)", len(aa.loops))
			}
			aa.candidateSlices[s] = candidateSlice{loopDepth: len(aa.loops)}
		}
		// No children aside from the declared ONAME.
		aa.countProcessed(decl.X)
		return false

	case ir.ONAME:

		// We are seeing a name we have not already handled in another case,
		// so remove any corresponding candidate slice.
		if n.Type().IsSlice() {
			name := n.(*ir.Name)
			_, ok := aa.candidateSlices[name]
			if ok {
				delete(aa.candidateSlices, name)
				if base.Debug.EscapeAlias >= 2 {
					aa.diag(n.Pos(), name, "removing candidate slice", "")
				}
			}
		}
		// No children.
		return false

	case ir.OAS2:
		n := n.(*ir.AssignListStmt)
		aa.analyzeAssign(n, n.Lhs, n.Rhs)
		return false

	case ir.OAS:
		assign := n.(*ir.AssignStmt)
		aa.analyzeAssign(n, []ir.Node{assign.X}, []ir.Node{assign.Y})
		return false

	case ir.OFOR, ir.ORANGE:
		aa.visitList(n.Init())

		if n.Op() == ir.ORANGE {
			// TODO(thepudds): previously we visited this range expression
			// in the switch just below, after pushing the loop. This current placement
			// is more correct, but generate a test or find an example in stdlib or similar
			// where it matters. (Our current tests do not complain.)
			aa.visit(n.(*ir.RangeStmt).X)
		}

		// Push a new loop.
		aa.loops = append(aa.loops, nil)

		// Process the loop.
		switch n.Op() {
		case ir.OFOR:
			forstmt := n.(*ir.ForStmt)
			aa.visit(forstmt.Cond)
			aa.visitList(forstmt.Body)
			aa.visit(forstmt.Post)
		case ir.ORANGE:
			rangestmt := n.(*ir.RangeStmt)
			aa.visit(rangestmt.Key)
			aa.visit(rangestmt.Value)
			aa.visitList(rangestmt.Body)
		default:
			base.Fatalf("loop not OFOR or ORANGE: %v", n)
		}

		// Pop the loop.
		var candidateAppends []candidateAppend
		candidateAppends, aa.loops = aa.loops[len(aa.loops)-1], aa.loops[:len(aa.loops)-1]
		for _, a := range candidateAppends {
			// We are done with the loop, so we can validate any candidate appends
			// that have not had their slice removed yet. We know a slice is unaliased
			// throughout the loop if the slice is still in the candidate slice map.
			if cs, ok := aa.candidateSlices[a.s]; ok {
				if cs.loopDepth == len(aa.loops) {
					// We've returned to the loop depth where the slice was declared and
					// hence made it all the way through any loops that started after
					// that declaration.
					if base.Debug.EscapeAlias >= 2 {
						aa.diag(n.Pos(), a.s, "proved non-aliased append",
							"(completed loop, decl at depth: %d)", cs.loopDepth)
					}
					aa.noAliasAppends = append(aa.noAliasAppends, a.call)
				} else if cs.loopDepth < len(aa.loops) {
					if base.Debug.EscapeAlias >= 2 {
						aa.diag(n.Pos(), a.s, "cannot prove non-aliased append",
							"(completed loop, decl at depth: %d)", cs.loopDepth)
					}
				} else {
					panic("impossible: candidate slice loopDepth > current loop depth")
				}
			}
		}
		return false

	case ir.OLEN, ir.OCAP:
		n := n.(*ir.UnaryExpr)
		if n.X.Op() == ir.ONAME {
			// This does not disqualify a candidate slice.
			aa.visitList(n.Init())
			aa.countProcessed(n.X)
		} else {
			ir.DoChildren(n, aa.visit)
		}
		return false

	case ir.OCLOSURE:
		// Give up on all our in-progress slices.
		closure := n.(*ir.ClosureExpr)
		if base.Debug.EscapeAlias >= 2 {
			aa.diag(n.Pos(), closure.Func, "clearing all in-progress slices due to OCLOSURE",
				"(was %d in-progress slices)", len(aa.candidateSlices))
		}
		clear(aa.candidateSlices)
		return ir.DoChildren(n, aa.visit)

	case ir.OLABEL, ir.OGOTO:
		// Give up on all our in-progress slices.
		if base.Debug.EscapeAlias >= 2 {
			aa.diag(n.Pos(), n, "clearing all in-progress slices due to label or goto",
				"(was %d in-progress slices)", len(aa.candidateSlices))
		}
		clear(aa.candidateSlices)
		return false

	default:
		return ir.DoChildren(n, aa.visit)
	}
}

func (aa *aliasAnalysis) visitList(nodes []ir.Node) {
	for _, n := range nodes {
		aa.visit(n)
	}
}

// analyzeAssign evaluates the assignment dsts... = srcs...
//
// assign is an *ir.AssignStmt or *ir.AssignListStmt.
func (aa *aliasAnalysis) analyzeAssign(assign ir.Node, dsts, srcs []ir.Node) {
	aa.visitList(assign.Init())
	for i := range dsts {
		dst := dsts[i]
		src := srcs[i]

		if dst.Op() != ir.ONAME || !dst.Type().IsSlice() {
			// Nothing for us to do aside from visiting the remaining children.
			aa.visit(dst)
			aa.visit(src)
			continue
		}

		// We have a slice being assigned to an ONAME.

		// Check for simple zero value assignments to an ONAME, which we ignore.
		if src == nil {
			aa.countProcessed(dst)
			continue
		}

		if base.Debug.EscapeAlias >= 4 {
			srcfn := ""
			if src.Op() == ir.ONAME {
				srcfn = fmt.Sprintf("%v.", src.Name().Curfn)
			}
			aa.diag(assign.Pos(), assign, "visiting slice assignment", "%v.%v = %s%v (%s %T = %s %T)",
				dst.Name().Curfn, dst, srcfn, src, dst.Op().String(), dst, src.Op().String(), src)
		}

		// Now check what we have on the RHS.
		switch src.Op() {
		// Cases:

		// Check for s := make([]T, ...) or s := []T{...}, along with the '=' version
		// of those which does not alias s as long as s is not used in the make.
		//
		// TODO(thepudds): we need to be sure that 's := []T{1,2,3}' does not end up backed by a
		// global static. Ad-hoc testing indicates that example and similar seem to be
		// stack allocated, but that was not exhaustive testing. We do have runtime.freegc
		// able to throw if it finds a global static, but should test more.
		//
		// TODO(thepudds): could also possibly allow 's := append([]T(nil), ...)'
		// and 's := append([]T{}, ...)'.
		case ir.OMAKESLICE, ir.OSLICELIT:
			name := dst.(*ir.Name)
			if name.Class == ir.PAUTO {
				if base.Debug.EscapeAlias > 1 {
					aa.diag(assign.Pos(), assign, "assignment from make or slice literal", "")
				}
				// If this is Def=true, the ODCL in the init will causes this to be tracked
				// as a candidate slice. We walk the init and RHS but avoid visiting the name
				// in the LHS, which would remove the slice from the candidate list after it
				// was just added.
				aa.visit(src)
				aa.countProcessed(name)
				continue
			}

		// Check for s = append(s, <...>).
		case ir.OAPPEND:
			s := dst.(*ir.Name)
			call := src.(*ir.CallExpr)
			if call.Args[0] == s {
				// Matches s = append(s, <...>).
				// First visit other arguments in case they use s.
				aa.visitList(call.Args[1:])
				// Mark the call as processed, and s twice.
				aa.countProcessed(s, call, s)

				// We have now examined all non-ONAME children of assign.

				// This is now the heart of the analysis.
				// Check to see if this slice is a live candidate.
				cs, ok := aa.candidateSlices[s]
				if ok {
					if cs.loopDepth == len(aa.loops) {
						// No new loop has started after the declaration of s,
						// so this is definitive.
						if base.Debug.EscapeAlias >= 2 {
							aa.diag(assign.Pos(), assign, "proved non-aliased append",
								"(loop depth: %d, equals decl depth)", len(aa.loops))
						}
						aa.noAliasAppends = append(aa.noAliasAppends, call)
					} else if cs.loopDepth < len(aa.loops) {
						// A new loop has started since the declaration of s,
						// so we can't validate this append yet, but
						// remember it in case we can validate it later when
						// all loops using s are done.
						aa.loops[cs.loopDepth] = append(aa.loops[cs.loopDepth],
							candidateAppend{s: s, call: call})
					} else {
						panic("impossible: candidate slice loopDepth > current loop depth")
					}
				}
				continue
			}
		} // End of switch on src.Op().

		// Reached bottom of the loop over assignments.
		// If we get here, we need to visit the dst and src normally.
		aa.visit(dst)
		aa.visit(src)
	}
}

func (aa *aliasAnalysis) countProcessed(nodes ...ir.Node) {
	if aa.doubleCheck {
		for _, n := range nodes {
			aa.processed[n]++
		}
	}
}

func (aa *aliasAnalysis) diag(pos src.XPos, n ir.Node, what string, format string, args ...any) {
	fmt.Printf("%-25s alias analysis: %-30s  %-20s  %s\n",
		fmtPosShort(pos)+":",
		what+":",
		fmt.Sprintf("%v", n),
		fmt.Sprintf(format, args...))
}

// doubleCheckProcessed does a sanity check for missed nodes in our visit.
func doubleCheckProcessed(fn *ir.Func, processed map[ir.Node]int) {
	// Do a trivial walk while counting the nodes
	// to compare against the counts in processed.

	observed := make(map[ir.Node]int)
	var walk func(n ir.Node) bool
	walk = func(n ir.Node) bool {
		observed[n]++
		return ir.DoChildren(n, walk)
	}
	ir.DoChildren(fn, walk)

	if !maps.Equal(processed, observed) {
		// The most likely mistake might be something was missed while building processed,
		// so print extra details in that direction.
		for n, observedCount := range observed {
			processedCount, ok := processed[n]
			if processedCount != observedCount || !ok {
				base.WarnfAt(n.Pos(),
					"alias analysis: mismatch for %T: %v: processed %d times, observed %d times",
					n, n, processedCount, observedCount)
			}
		}
		base.FatalfAt(fn.Pos(), "alias analysis: mismatch in visited nodes")
	}
}

func fmtPosShort(xpos src.XPos) string {
	// TODO(thepudds): I think I did this a simpler way a while ago? Or maybe add base.FmtPosShort
	// or similar? Or maybe just use base.FmtPos and give up on nicely aligned log messages?
	pos := base.Ctxt.PosTable.Pos(xpos)
	shortLine := filepath.Base(pos.AbsFilename()) + ":" + pos.LineNumber()
	return shortLine
}

```

// === FILE: references!/go/src/cmd/compile/internal/escape/assign.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
)

// addr evaluates an addressable expression n and returns a hole
// that represents storing into the represented location.
func (e *escape) addr(n ir.Node) hole {
	if n == nil || ir.IsBlank(n) {
		// Can happen in select case, range, maybe others.
		return e.discardHole()
	}

	k := e.heapHole()

	switch n.Op() {
	default:
		base.Fatalf("unexpected addr: %v", n)
	case ir.ONAME:
		n := n.(*ir.Name)
		if n.Class == ir.PEXTERN {
			break
		}
		k = e.oldLoc(n).asHole()
	case ir.OLINKSYMOFFSET:
		break
	case ir.ODOT:
		n := n.(*ir.SelectorExpr)
		k = e.addr(n.X)
	case ir.OINDEX:
		n := n.(*ir.IndexExpr)
		e.discard(n.Index)
		if n.X.Type().IsArray() {
			k = e.addr(n.X)
		} else {
			e.mutate(n.X)
		}
	case ir.ODEREF:
		n := n.(*ir.StarExpr)
		e.mutate(n.X)
	case ir.ODOTPTR:
		n := n.(*ir.SelectorExpr)
		e.mutate(n.X)
	case ir.OINDEXMAP:
		n := n.(*ir.IndexExpr)
		e.discard(n.X)
		e.assignHeap(n.Index, "key of map put", n)
	}

	return k
}

func (e *escape) mutate(n ir.Node) {
	e.expr(e.mutatorHole(), n)
}

func (e *escape) addrs(l ir.Nodes) []hole {
	var ks []hole
	for _, n := range l {
		ks = append(ks, e.addr(n))
	}
	return ks
}

func (e *escape) assignHeap(src ir.Node, why string, where ir.Node) {
	e.expr(e.heapHole().note(where, why), src)
}

// assignList evaluates the assignment dsts... = srcs....
func (e *escape) assignList(dsts, srcs []ir.Node, why string, where ir.Node) {
	ks := e.addrs(dsts)
	for i, k := range ks {
		var src ir.Node
		if i < len(srcs) {
			src = srcs[i]
		}

		if dst := dsts[i]; dst != nil {
			// Detect implicit conversion of uintptr to unsafe.Pointer when
			// storing into reflect.{Slice,String}Header.
			if dst.Op() == ir.ODOTPTR && ir.IsReflectHeaderDataField(dst) {
				e.unsafeValue(e.heapHole().note(where, why), src)
				continue
			}

			// Filter out some no-op assignments for escape analysis.
			if src != nil && isSelfAssign(dst, src) {
				if base.Flag.LowerM != 0 {
					base.WarnfAt(where.Pos(), "%v ignoring self-assignment in %v", e.curfn, where)
				}
				k = e.discardHole()
			}
		}

		e.expr(k.note(where, why), src)
	}

	e.reassigned(ks, where)
}

// reassigned marks the locations associated with the given holes as
// reassigned, unless the location represents a variable declared and
// assigned exactly once by where.
func (e *escape) reassigned(ks []hole, where ir.Node) {
	if as, ok := where.(*ir.AssignStmt); ok && as.Op() == ir.OAS && as.Y == nil {
		if dst, ok := as.X.(*ir.Name); ok && dst.Op() == ir.ONAME && dst.Defn == nil {
			// Zero-value assignment for variable declared without an
			// explicit initial value. Assume this is its initialization
			// statement.
			return
		}
	}

	for _, k := range ks {
		loc := k.dst
		// Variables declared by range statements are assigned on every iteration.
		if n, ok := loc.n.(*ir.Name); ok && n.Defn == where && where.Op() != ir.ORANGE {
			continue
		}
		loc.reassigned = true
	}
}

```

// === FILE: references!/go/src/cmd/compile/internal/escape/call.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/src"
	"strings"
)

// call evaluates a call expressions, including builtin calls. ks
// should contain the holes representing where the function callee's
// results flows.
func (e *escape) call(ks []hole, call ir.Node) {
	argument := func(k hole, arg ir.Node) {
		// TODO(mdempsky): Should be "call argument".
		e.expr(k.note(call, "call parameter"), arg)
	}

	switch call.Op() {
	default:
		ir.Dump("esc", call)
		base.Fatalf("unexpected call op: %v", call.Op())

	case ir.OCALLFUNC, ir.OCALLINTER:
		call := call.(*ir.CallExpr)
		typecheck.AssertFixedCall(call)

		// Pick out the function callee(s), if statically known.
		// fns collects all known callees; for a single static callee
		// it has one element. For unknown callees fns is nil.
		//
		// TODO(mdempsky): Change fns from []*ir.Name to []*ir.Func,
		// but some functions (e.g., runtime builtins, method wrappers,
		// generated eq/hash functions) don't have it set. Investigate
		// whether that's a concern.
		var fns []*ir.Name
		switch call.Op() {
		case ir.OCALLFUNC:
			ro := e.reassignOracle(e.curfn)
			v := ro.StaticValue(call.Fun)
			if fn := ir.StaticCalleeName(v); fn != nil {
				fns = []*ir.Name{fn}
			} else if name, ok := v.(*ir.Name); ok {
				fns = resolveAssignedCallees(ro.FuncAssignments(name.Canonical()))
			}
		}

		fntype := call.Fun.Type()
		if len(fns) == 1 {
			fntype = fns[0].Type()
		}

		// Wire result flows for in-batch callees.
		if ks != nil {
			for _, f := range fns {
				if e.inMutualBatch(f) {
					for i, result := range f.Type().Results() {
						e.expr(ks[i], result.Nname.(*ir.Name))
					}
				}
			}
		}

		var recvArg ir.Node
		if call.Op() == ir.OCALLFUNC {
			// Evaluate callee function expression.
			calleeK := e.discardHole()
			if len(fns) == 0 { // unknown callee
				for _, k := range ks {
					if k.dst != &e.blankLoc {
						// The results flow somewhere, but we don't statically
						// know the callee function. If a closure flows here, we
						// need to conservatively assume its results might flow to
						// the heap.
						calleeK = e.calleeHole().note(call, "callee operand")
						break
					}
				}
			}
			e.expr(calleeK, call.Fun)
		} else {
			recvArg = call.Fun.(*ir.SelectorExpr).X
		}

		args := call.Args
		if fntype.Recv() != nil {
			if recvArg == nil {
				// Function call using method expression. Receiver argument is
				// at the front of the regular arguments list.
				recvArg, args = args[0], args[1:]
			}
		}

		if call.IsCompilerVarLive {
			// Don't escape compiler-inserted KeepAlive.
			if recvArg != nil {
				argument(e.discardHole(), recvArg)
			}
			for _, arg := range args {
				argument(e.discardHole(), arg)
			}
		} else if isEscapeNonString(fns, fntype) {
			// internal/abi.EscapeNonString forces its argument to
			// the heap if it contains a non-string pointer. This is
			// used in hash/maphash.Comparable (where we cannot hash
			// pointers to locals whose address may change on stack
			// growth) and unique.clone (to model the data flow edge
			// with strings excluded, because strings are cloned by
			// content). The actual call we match is:
			//   internal/abi.EscapeNonString[go.shape.T](dict, go.shape.T)
			k := e.heapHole()
			if !hasNonStringPointers(fntype.Params()[1].Type) {
				k = e.discardHole()
			}
			for _, arg := range args {
				argument(k, arg)
			}
		} else {
			if recvArg != nil {
				e.rewriteArgument(recvArg, call, fns)
				argument(e.mergedTagHole(ks, fns, -1, len(fntype.Params())), recvArg)
			}
			for i := range fntype.Params() {
				e.rewriteArgument(args[i], call, fns)
				argument(e.mergedTagHole(ks, fns, i, len(fntype.Params())), args[i])
			}
		}

	case ir.OINLCALL:
		call := call.(*ir.InlinedCallExpr)
		e.stmts(call.Body)
		for i, result := range call.ReturnVars {
			k := e.discardHole()
			if ks != nil {
				k = ks[i]
			}
			e.expr(k, result)
		}

	case ir.OAPPEND:
		call := call.(*ir.CallExpr)
		args := call.Args

		// Appendee slice may flow directly to the result, if
		// it has enough capacity. Alternatively, a new heap
		// slice might be allocated, and all slice elements
		// might flow to heap.
		appendeeK := e.teeHole(ks[0], e.mutatorHole())
		if args[0].Type().Elem().HasPointers() {
			appendeeK = e.teeHole(appendeeK, e.heapHole().deref(call, "appendee slice"))
		}
		argument(appendeeK, args[0])

		if call.IsDDD {
			appendedK := e.discardHole()
			if args[1].Type().IsSlice() && args[1].Type().Elem().HasPointers() {
				appendedK = e.heapHole().deref(call, "appended slice...")
			}
			argument(appendedK, args[1])
		} else {
			for i := 1; i < len(args); i++ {
				argument(e.heapHole(), args[i])
			}
		}
		e.discard(call.RType)

		// Model the new backing store that might be allocated by append.
		// Its address flows to the result.
		// Users of escape analysis can look at the escape information for OAPPEND
		// and use that to decide where to allocate the backing store.
		backingStore := e.spill(ks[0], call)
		// As we have a boolean to prevent reuse, we can treat these allocations as outside any loops.
		backingStore.dst.loopDepth = 0

	case ir.OCOPY:
		call := call.(*ir.BinaryExpr)
		argument(e.mutatorHole(), call.X)

		copiedK := e.discardHole()
		if call.Y.Type().IsSlice() && call.Y.Type().Elem().HasPointers() {
			copiedK = e.heapHole().deref(call, "copied slice")
		}
		argument(copiedK, call.Y)
		e.discard(call.RType)

	case ir.OPANIC:
		call := call.(*ir.UnaryExpr)
		argument(e.heapHole(), call.X)

	case ir.OCOMPLEX:
		call := call.(*ir.BinaryExpr)
		e.discard(call.X)
		e.discard(call.Y)

	case ir.ODELETE, ir.OPRINT, ir.OPRINTLN, ir.ORECOVER:
		call := call.(*ir.CallExpr)
		for _, arg := range call.Args {
			e.discard(arg)
		}
		e.discard(call.RType)

	case ir.OMIN, ir.OMAX:
		call := call.(*ir.CallExpr)
		for _, arg := range call.Args {
			argument(ks[0], arg)
		}
		e.discard(call.RType)

	case ir.OLEN, ir.OCAP, ir.OREAL, ir.OIMAG, ir.OCLOSE:
		call := call.(*ir.UnaryExpr)
		e.discard(call.X)

	case ir.OCLEAR:
		call := call.(*ir.UnaryExpr)
		argument(e.mutatorHole(), call.X)

	case ir.OUNSAFESTRINGDATA, ir.OUNSAFESLICEDATA:
		call := call.(*ir.UnaryExpr)
		argument(ks[0], call.X)

	case ir.OUNSAFEADD, ir.OUNSAFESLICE, ir.OUNSAFESTRING:
		call := call.(*ir.BinaryExpr)
		argument(ks[0], call.X)
		e.discard(call.Y)
		e.discard(call.RType)
	}
}

// goDeferStmt analyzes a "go" or "defer" statement.
func (e *escape) goDeferStmt(n *ir.GoDeferStmt) {
	k := e.heapHole()
	if n.Op() == ir.ODEFER && e.loopDepth == 1 && n.DeferAt == nil {
		// Top-level defer arguments don't escape to the heap,
		// but they do need to last until they're invoked.
		k = e.later(e.discardHole())

		// force stack allocation of defer record, unless
		// open-coded defers are used (see ssa.go)
		n.SetEsc(ir.EscNever)
	}

	// If the function is already a zero argument/result function call,
	// just escape analyze it normally.
	//
	// Note that the runtime is aware of this optimization for
	// "go" statements that start in reflect.makeFuncStub or
	// reflect.methodValueCall.

	call, ok := n.Call.(*ir.CallExpr)
	if !ok || call.Op() != ir.OCALLFUNC {
		base.FatalfAt(n.Pos(), "expected function call: %v", n.Call)
	}
	if sig := call.Fun.Type(); sig.NumParams()+sig.NumResults() != 0 {
		base.FatalfAt(n.Pos(), "expected signature without parameters or results: %v", sig)
	}

	if clo, ok := call.Fun.(*ir.ClosureExpr); ok && n.Op() == ir.OGO {
		clo.IsGoWrap = true
	}

	e.expr(k, call.Fun)
}

// rewriteArgument rewrites the argument arg of the given call expression.
// fns is the list of statically known callees, if any.
func (e *escape) rewriteArgument(arg ir.Node, call *ir.CallExpr, fns []*ir.Name) {
	var pragma ir.PragmaFlag
	for _, fn := range fns {
		if fn.Func != nil {
			pragma |= fn.Func.Pragma
		}
	}
	if pragma&(ir.UintptrKeepAlive|ir.UintptrEscapes) == 0 {
		return
	}

	// unsafeUintptr rewrites "uintptr(ptr)" arguments to syscall-like
	// functions, so that ptr is kept alive and/or escaped as
	// appropriate. unsafeUintptr also reports whether it modified arg0.
	unsafeUintptr := func(arg ir.Node) {
		// If the argument is really a pointer being converted to uintptr,
		// arrange for the pointer to be kept alive until the call
		// returns, by copying it into a temp and marking that temp still
		// alive when we pop the temp stack.
		conv, ok := arg.(*ir.ConvExpr)
		if !ok || conv.Op() != ir.OCONVNOP {
			return // not a conversion
		}
		if !conv.X.Type().IsUnsafePtr() || !conv.Type().IsUintptr() {
			return // not an unsafe.Pointer->uintptr conversion
		}

		// Create and declare a new pointer-typed temp variable.
		//
		// TODO(mdempsky): This potentially violates the Go spec's order
		// of evaluations, by evaluating arg.X before any other
		// operands.
		tmp := e.copyExpr(conv.Pos(), conv.X, call.PtrInit())
		conv.X = tmp

		k := e.mutatorHole()
		if pragma&ir.UintptrEscapes != 0 {
			k = e.heapHole().note(conv, "//go:uintptrescapes")
		}
		e.flow(k, e.oldLoc(tmp))

		if pragma&ir.UintptrKeepAlive != 0 {
			tmp.SetAddrtaken(true) // ensure SSA keeps the tmp variable
			call.KeepAlive = append(call.KeepAlive, tmp)
		}
	}

	// For variadic functions, the compiler has already rewritten:
	//
	//     f(a, b, c)
	//
	// to:
	//
	//     f([]T{a, b, c}...)
	//
	// So we need to look into slice elements to handle uintptr(ptr)
	// arguments to variadic syscall-like functions correctly.
	if arg.Op() == ir.OSLICELIT {
		list := arg.(*ir.CompLitExpr).List
		for _, el := range list {
			if el.Op() == ir.OKEY {
				el = el.(*ir.KeyExpr).Value
			}
			unsafeUintptr(el)
		}
	} else {
		unsafeUintptr(arg)
	}
}

// copyExpr creates and returns a new temporary variable within fn;
// appends statements to init to declare and initialize it to expr;
// and escape analyzes the data flow.
func (e *escape) copyExpr(pos src.XPos, expr ir.Node, init *ir.Nodes) *ir.Name {
	if ir.HasUniquePos(expr) {
		pos = expr.Pos()
	}

	tmp := typecheck.TempAt(pos, e.curfn, expr.Type())

	stmts := []ir.Node{
		ir.NewDecl(pos, ir.ODCL, tmp),
		ir.NewAssignStmt(pos, tmp, expr),
	}
	typecheck.Stmts(stmts)
	init.Append(stmts...)

	e.newLoc(tmp, true)
	e.stmts(stmts)

	return tmp
}

func (e *escape) mergedTagHole(ks []hole, fns []*ir.Name, paramIdx int, nParams int) hole {
	if len(fns) == 0 {
		return e.heapHole()
	}
	holes := make([]hole, 0, len(fns))
	for _, f := range fns {
		offset := nParams - len(f.Type().Params())
		j := paramIdx - offset
		var p *types.Field
		if j >= 0 {
			p = f.Type().Params()[j]
		} else {
			p = f.Type().Recv()
		}
		holes = append(holes, e.tagHole(ks, f, p))
	}
	return e.teeHole(holes...)
}

// tagHole returns a hole for evaluating an argument passed to param.
// ks should contain the holes representing where the function
// callee's results flows. fn is the statically-known callee function,
// if any.
func (e *escape) tagHole(ks []hole, fn *ir.Name, param *types.Field) hole {
	// If this is a dynamic call, we can't rely on param.Note.
	if fn == nil {
		return e.heapHole()
	}

	if e.inMutualBatch(fn) {
		if param.Nname == nil {
			return e.discardHole()
		}
		return e.addr(param.Nname.(*ir.Name))
	}

	// Call to previously tagged function.

	var tagKs []hole
	esc := parseLeaks(param.Note)

	if x := esc.Heap(); x >= 0 {
		tagKs = append(tagKs, e.heapHole().shift(x))
	}
	if x := esc.Mutator(); x >= 0 {
		tagKs = append(tagKs, e.mutatorHole().shift(x))
	}
	if x := esc.Callee(); x >= 0 {
		tagKs = append(tagKs, e.calleeHole().shift(x))
	}

	if ks != nil {
		for i := 0; i < numEscResults; i++ {
			if x := esc.Result(i); x >= 0 {
				tagKs = append(tagKs, ks[i].shift(x))
			}
		}
	}

	return e.teeHole(tagKs...)
}

// resolveAssignedCallees resolves all assignment RHS values to static
// callee names, skipping zero-value assignments since nil panics on
// call and can't cause escape.
func resolveAssignedCallees(assigns []*ir.AssignStmt) []*ir.Name {
	fns := make([]*ir.Name, 0, len(assigns))
	for _, as := range assigns {
		if ir.IsZero(as.Y) {
			continue // zero value panics on call; skip
		}
		callee := ir.StaticCalleeName(as.Y)
		if callee == nil {
			return nil
		}
		if callee.Func != nil && callee.Func.Pragma&(ir.UintptrKeepAlive|ir.UintptrEscapes) != 0 {
			return nil
		}
		fns = append(fns, callee)
	}
	return fns
}

func isEscapeNonString(fns []*ir.Name, fntype *types.Type) bool {
	return len(fns) == 1 &&
		fns[0].Sym().Pkg.Path == "internal/abi" &&
		strings.HasPrefix(fns[0].Sym().Name, "EscapeNonString[") &&
		len(fntype.Params()) == 2 && fntype.Params()[1].Type.IsShape()
}

func hasNonStringPointers(t *types.Type) bool {
	if !t.HasPointers() {
		return false
	}
	switch t.Kind() {
	case types.TSTRING:
		return false
	case types.TSTRUCT:
		for _, f := range t.Fields() {
			if hasNonStringPointers(f.Type) {
				return true
			}
		}
		return false
	case types.TARRAY:
		return hasNonStringPointers(t.Elem())
	}
	return true
}

```

// === FILE: references!/go/src/cmd/compile/internal/escape/escape.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"fmt"
	"go/constant"
	"go/token"
	"internal/goexperiment"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/logopt"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// Escape analysis.
//
// Here we analyze functions to determine which Go variables
// (including implicit allocations such as calls to "new" or "make",
// composite literals, etc.) can be allocated on the stack. The two
// key invariants we have to ensure are: (1) pointers to stack objects
// cannot be stored in the heap, and (2) pointers to a stack object
// cannot outlive that object (e.g., because the declaring function
// returned and destroyed the object's stack frame, or its space is
// reused across loop iterations for logically distinct variables).
//
// We implement this with a static data-flow analysis of the AST.
// First, we construct a directed weighted graph where vertices
// (termed "locations") represent variables allocated by statements
// and expressions, and edges represent assignments between variables
// (with weights representing addressing/dereference counts).
//
// Next we walk the graph looking for assignment paths that might
// violate the invariants stated above. If a variable v's address is
// stored in the heap or elsewhere that may outlive it, then v is
// marked as requiring heap allocation.
//
// To support interprocedural analysis, we also record data-flow from
// each function's parameters to the heap and to its result
// parameters. This information is summarized as "parameter tags",
// which are used at static call sites to improve escape analysis of
// function arguments.

// Constructing the location graph.
//
// Every allocating statement (e.g., variable declaration) or
// expression (e.g., "new" or "make") is first mapped to a unique
// "location."
//
// We also model every Go assignment as a directed edges between
// locations. The number of dereference operations minus the number of
// addressing operations is recorded as the edge's weight (termed
// "derefs"). For example:
//
//     p = &q    // -1
//     p = q     //  0
//     p = *q    //  1
//     p = **q   //  2
//
//     p = **&**&q  // 2
//
// Note that the & operator can only be applied to addressable
// expressions, and the expression &x itself is not addressable, so
// derefs cannot go below -1.
//
// Every Go language construct is lowered into this representation,
// generally without sensitivity to flow, path, or context; and
// without distinguishing elements within a compound variable. For
// example:
//
//     var x struct { f, g *int }
//     var u []*int
//
//     x.f = u[0]
//
// is modeled simply as
//
//     x = *u
//
// That is, we don't distinguish x.f from x.g, or u[0] from u[1],
// u[2], etc. However, we do record the implicit dereference involved
// in indexing a slice.

// A batch holds escape analysis state that's shared across an entire
// batch of functions being analyzed at once.
type batch struct {
	allLocs         []*location
	closures        []closure
	reassignOracles map[*ir.Func]*ir.ReassignOracle

	heapLoc    location
	mutatorLoc location
	calleeLoc  location
	blankLoc   location
}

// A closure holds a closure expression and its spill hole (i.e.,
// where the hole representing storing into its closure record).
type closure struct {
	k   hole
	clo *ir.ClosureExpr
}

// An escape holds state specific to a single function being analyzed
// within a batch.
type escape struct {
	*batch

	curfn *ir.Func // function being analyzed

	labels map[*types.Sym]labelState // known labels

	// loopDepth counts the current loop nesting depth within
	// curfn. It increments within each "for" loop and at each
	// label with a corresponding backwards "goto" (i.e.,
	// unstructured loop).
	loopDepth int
}

func Funcs(all []*ir.Func) {
	// Make a cache of ir.ReassignOracles. The cache is lazily populated.
	// TODO(thepudds): consider adding a field on ir.Func instead. We might also be able
	// to use that field elsewhere, like in walk. See discussion in https://go.dev/cl/688075.
	reassignOracles := make(map[*ir.Func]*ir.ReassignOracle)

	ir.VisitFuncsBottomUp(all, func(list []*ir.Func, recursive bool) {
		Batch(list, reassignOracles)
	})
}

// Batch performs escape analysis on a minimal batch of
// functions.
func Batch(fns []*ir.Func, reassignOracles map[*ir.Func]*ir.ReassignOracle) {
	var b batch
	b.heapLoc.attrs = attrEscapes | attrPersists | attrMutates | attrCalls
	b.mutatorLoc.attrs = attrMutates
	b.calleeLoc.attrs = attrCalls
	b.reassignOracles = reassignOracles

	// Construct data-flow graph from syntax trees.
	for _, fn := range fns {
		if base.Flag.W > 1 {
			s := fmt.Sprintf("\nbefore escape %v", fn)
			ir.Dump(s, fn)
		}
		b.initFunc(fn)
	}
	for _, fn := range fns {
		if !fn.IsClosure() {
			b.walkFunc(fn)
		}
	}

	// We've walked the function bodies, so we've seen everywhere a
	// variable might be reassigned or have its address taken. Now we
	// can decide whether closures should capture their free variables
	// by value or reference.
	for _, closure := range b.closures {
		b.flowClosure(closure.k, closure.clo)
	}
	b.closures = nil

	for _, loc := range b.allLocs {
		// Try to replace some non-constant expressions with literals.
		b.rewriteWithLiterals(loc.n, loc.curfn)

		// Check if the node must be heap allocated for certain reasons
		// such as OMAKESLICE for a large slice.
		if why := HeapAllocReason(loc.n); why != "" {
			b.flow(b.heapHole().addr(loc.n, why), loc)
		}
	}

	b.walkAll()
	b.finish(fns)
}

func (b *batch) with(fn *ir.Func) *escape {
	return &escape{
		batch:     b,
		curfn:     fn,
		loopDepth: 1,
	}
}

func (b *batch) initFunc(fn *ir.Func) {
	e := b.with(fn)
	if fn.Esc() != escFuncUnknown {
		base.Fatalf("unexpected node: %v", fn)
	}
	fn.SetEsc(escFuncPlanned)
	if base.Flag.LowerM > 3 {
		ir.Dump("escAnalyze", fn)
	}

	// Allocate locations for local variables.
	for _, n := range fn.Dcl {
		e.newLoc(n, true)
	}

	// Also for hidden parameters (e.g., the ".this" parameter to a
	// method value wrapper).
	if fn.OClosure == nil {
		for _, n := range fn.ClosureVars {
			e.newLoc(n.Canonical(), true)
		}
	}

	// Initialize resultIndex for result parameters.
	for i, f := range fn.Type().Results() {
		e.oldLoc(f.Nname.(*ir.Name)).resultIndex = 1 + i
	}
}

func (b *batch) walkFunc(fn *ir.Func) {
	e := b.with(fn)
	fn.SetEsc(escFuncStarted)

	// Identify labels that mark the head of an unstructured loop.
	ir.Visit(fn, func(n ir.Node) {
		switch n.Op() {
		case ir.OLABEL:
			n := n.(*ir.LabelStmt)
			if n.Label.IsBlank() {
				break
			}
			if e.labels == nil {
				e.labels = make(map[*types.Sym]labelState)
			}
			e.labels[n.Label] = nonlooping

		case ir.OGOTO:
			// If we visited the label before the goto,
			// then this is a looping label.
			n := n.(*ir.BranchStmt)
			if e.labels[n.Label] == nonlooping {
				e.labels[n.Label] = looping
			}
		}
	})

	e.block(fn.Body)

	if len(e.labels) != 0 {
		base.FatalfAt(fn.Pos(), "leftover labels after walkFunc")
	}
}

func (b *batch) flowClosure(k hole, clo *ir.ClosureExpr) {
	for _, cv := range clo.Func.ClosureVars {
		n := cv.Canonical()
		loc := b.oldLoc(cv)
		if !loc.captured {
			base.FatalfAt(cv.Pos(), "closure variable never captured: %v", cv)
		}

		// Capture by value for variables <= 128 bytes that are never reassigned.
		n.SetByval(!loc.addrtaken && !loc.reassigned && n.Type().Size() <= 128)
		if !n.Byval() {
			n.SetAddrtaken(true)
			if n.Sym().Name == typecheck.LocalDictName {
				base.FatalfAt(n.Pos(), "dictionary variable not captured by value")
			}
		}

		if base.Flag.LowerM > 1 {
			how := "ref"
			if n.Byval() {
				how = "value"
			}
			base.WarnfAt(n.Pos(), "%v capturing by %s: %v (addr=%v assign=%v width=%d)", n.Curfn, how, n, loc.addrtaken, loc.reassigned, n.Type().Size())
		}

		// Flow captured variables to closure.
		k := k
		if !cv.Byval() {
			k = k.addr(cv, "reference")
		}
		b.flow(k.note(cv, "captured by a closure"), loc)
	}
}

func (b *batch) finish(fns []*ir.Func) {
	// Record parameter tags for package export data.
	for _, fn := range fns {
		fn.SetEsc(escFuncTagged)

		for i, param := range fn.Type().RecvParams() {
			param.Note = b.paramTag(fn, 1+i, param)
		}
	}

	for _, loc := range b.allLocs {
		n := loc.n
		if n == nil {
			continue
		}

		if n.Op() == ir.ONAME {
			n := n.(*ir.Name)
			n.Opt = nil
		}

		// Update n.Esc based on escape analysis results.

		// Omit escape diagnostics for go/defer wrappers, at least for now.
		// Historically, we haven't printed them, and test cases don't expect them.
		// TODO(mdempsky): Update tests to expect this.
		goDeferWrapper := n.Op() == ir.OCLOSURE && n.(*ir.ClosureExpr).Func.Wrapper()

		if loc.hasAttr(attrEscapes) {
			if n.Op() == ir.ONAME {
				if base.Flag.CompilingRuntime {
					base.ErrorfAt(n.Pos(), 0, "%v escapes to heap, not allowed in runtime", n)
				}
				if base.Flag.LowerM != 0 {
					base.WarnfAt(n.Pos(), "moved to heap: %v", n)
				}
			} else {
				if base.Flag.LowerM != 0 && !goDeferWrapper {
					if n.Op() == ir.OAPPEND {
						base.WarnfAt(n.Pos(), "append escapes to heap")
					} else {
						base.WarnfAt(n.Pos(), "%v escapes to heap", n)
					}
				}
				if logopt.Enabled() {
					var e_curfn *ir.Func // TODO(mdempsky): Fix.
					logopt.LogOpt(n.Pos(), "escape", "escape", ir.FuncName(e_curfn))
				}
			}
			n.SetEsc(ir.EscHeap)
		} else {
			if base.Flag.LowerM != 0 && n.Op() != ir.ONAME && !goDeferWrapper {
				if n.Op() == ir.OAPPEND {
					base.WarnfAt(n.Pos(), "append does not escape")
				} else {
					base.WarnfAt(n.Pos(), "%v does not escape", n)
				}
			}
			n.SetEsc(ir.EscNone)
			if !loc.hasAttr(attrPersists) {
				switch n.Op() {
				case ir.OCLOSURE:
					n := n.(*ir.ClosureExpr)
					n.SetTransient(true)
				case ir.OMETHVALUE:
					n := n.(*ir.SelectorExpr)
					n.SetTransient(true)
				case ir.OSLICELIT:
					n := n.(*ir.CompLitExpr)
					n.SetTransient(true)
				}
			}
		}

		// If the result of a string->[]byte conversion is never mutated,
		// then it can simply reuse the string's memory directly.
		if base.Debug.ZeroCopy != 0 {
			if n, ok := n.(*ir.ConvExpr); ok && n.Op() == ir.OSTR2BYTES && !loc.hasAttr(attrMutates) {
				if base.Flag.LowerM >= 1 {
					base.WarnfAt(n.Pos(), "zero-copy string->[]byte conversion")
				}
				n.SetOp(ir.OSTR2BYTESTMP)
			}
		}
	}

	if goexperiment.RuntimeFreegc {
		// Look for specific patterns of usage, such as appends
		// to slices that we can prove are not aliased.
		for _, fn := range fns {
			a := aliasAnalysis{}
			a.analyze(fn)
		}
	}

	for _, fn := range fns {
		if ir.MatchAstDump(fn, "escape") {
			ir.AstDump(fn, "escape, "+ir.FuncName(fn))
		}
	}
}

// inMutualBatch reports whether function fn is in the batch of
// mutually recursive functions being analyzed. When this is true,
// fn has not yet been analyzed, so its parameters and results
// should be incorporated directly into the flow graph instead of
// relying on its escape analysis tagging.
func (b *batch) inMutualBatch(fn *ir.Name) bool {
	if fn.Defn != nil && fn.Defn.Esc() < escFuncTagged {
		if fn.Defn.Esc() == escFuncUnknown {
			base.FatalfAt(fn.Pos(), "graph inconsistency: %v", fn)
		}
		return true
	}
	return false
}

const (
	escFuncUnknown = 0 + iota
	escFuncPlanned
	escFuncStarted
	escFuncTagged
)

// Mark labels that have no backjumps to them as not increasing e.loopdepth.
type labelState int

const (
	looping labelState = 1 + iota
	nonlooping
)

func (b *batch) paramTag(fn *ir.Func, narg int, f *types.Field) string {
	name := func() string {
		if f.Nname != nil {
			return f.Nname.Sym().Name
		}
		return fmt.Sprintf("arg#%d", narg)
	}

	// Only report diagnostics for user code;
	// not for wrappers generated around them.
	// TODO(mdempsky): Generalize this.
	diagnose := base.Flag.LowerM != 0 && !(fn.Wrapper() || fn.Dupok())

	if len(fn.Body) == 0 {
		// Assume that uintptr arguments must be held live across the call.
		// This is most important for syscall.Syscall.
		// See golang.org/issue/13372.
		// This really doesn't have much to do with escape analysis per se,
		// but we are reusing the ability to annotate an individual function
		// argument and pass those annotations along to importing code.
		fn.Pragma |= ir.UintptrKeepAlive

		if f.Type.IsUintptr() {
			if diagnose {
				base.WarnfAt(f.Pos, "assuming %v is unsafe uintptr", name())
			}
			return ""
		}

		if !f.Type.HasPointers() { // don't bother tagging for scalars
			return ""
		}

		var esc leaks

		// External functions are assumed unsafe, unless
		// //go:noescape is given before the declaration.
		if fn.Pragma&ir.Noescape != 0 {
			if diagnose && f.Sym != nil {
				base.WarnfAt(f.Pos, "%v does not escape", name())
			}
			esc.AddMutator(0)
			esc.AddCallee(0)
		} else {
			if diagnose && f.Sym != nil {
				base.WarnfAt(f.Pos, "leaking param: %v", name())
			}
			esc.AddHeap(0)
		}

		return esc.Encode()
	}

	if fn.Pragma&ir.UintptrEscapes != 0 {
		if f.Type.IsUintptr() {
			if diagnose {
				base.WarnfAt(f.Pos, "marking %v as escaping uintptr", name())
			}
			return ""
		}
		if f.IsDDD() && f.Type.Elem().IsUintptr() {
			// final argument is ...uintptr.
			if diagnose {
				base.WarnfAt(f.Pos, "marking %v as escaping ...uintptr", name())
			}
			return ""
		}
	}

	if !f.Type.HasPointers() { // don't bother tagging for scalars
		return ""
	}

	// Unnamed parameters are unused and therefore do not escape.
	if f.Sym == nil || f.Sym.IsBlank() {
		var esc leaks
		return esc.Encode()
	}

	n := f.Nname.(*ir.Name)
	loc := b.oldLoc(n)
	esc := loc.paramEsc
	esc.Optimize()

	if diagnose && !loc.hasAttr(attrEscapes) {
		b.reportLeaks(f.Pos, name(), esc, fn.Type())
	}

	return esc.Encode()
}

func (b *batch) reportLeaks(pos src.XPos, name string, esc leaks, sig *types.Type) {
	warned := false
	if x := esc.Heap(); x >= 0 {
		if x == 0 {
			base.WarnfAt(pos, "leaking param: %v", name)
		} else {
			// TODO(mdempsky): Mention level=x like below?
			base.WarnfAt(pos, "leaking param content: %v", name)
		}
		warned = true
	}
	for i := 0; i < numEscResults; i++ {
		if x := esc.Result(i); x >= 0 {
			res := sig.Result(i).Nname.Sym().Name
			base.WarnfAt(pos, "leaking param: %v to result %v level=%d", name, res, x)
			warned = true
		}
	}

	if base.Debug.EscapeMutationsCalls <= 0 {
		if !warned {
			base.WarnfAt(pos, "%v does not escape", name)
		}
		return
	}

	if x := esc.Mutator(); x >= 0 {
		base.WarnfAt(pos, "mutates param: %v derefs=%v", name, x)
		warned = true
	}
	if x := esc.Callee(); x >= 0 {
		base.WarnfAt(pos, "calls param: %v derefs=%v", name, x)
		warned = true
	}

	if !warned {
		base.WarnfAt(pos, "%v does not escape, mutate, or call", name)
	}
}

// rewriteWithLiterals attempts to replace certain non-constant expressions
// within n with a literal if possible.
func (b *batch) rewriteWithLiterals(n ir.Node, fn *ir.Func) {
	if n == nil || fn == nil {
		return
	}

	assignTemp := func(pos src.XPos, n ir.Node, init *ir.Nodes) {
		// Preserve any side effects of n by assigning it to an otherwise unused temp.
		tmp := typecheck.TempAt(pos, fn, n.Type())
		init.Append(typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, tmp)))
		init.Append(typecheck.Stmt(ir.NewAssignStmt(pos, tmp, n)))
	}

	switch n.Op() {
	case ir.OMAKESLICE:
		// Check if we can replace a non-constant argument to make with
		// a literal to allow for this slice to be stack allocated if otherwise allowed.
		n := n.(*ir.MakeExpr)

		r := &n.Cap
		if n.Cap == nil {
			r = &n.Len
		}

		if (*r).Op() != ir.OLITERAL {
			// Look up a cached ReassignOracle for the function, lazily computing one if needed.
			ro := b.reassignOracle(fn)
			if ro == nil {
				base.Fatalf("no ReassignOracle for function %v with closure parent %v", fn, fn.ClosureParent)
			}

			s := ro.StaticValue(*r)
			switch s.Op() {
			case ir.OLITERAL:
				lit, ok := s.(*ir.BasicLit)
				if !ok || lit.Val().Kind() != constant.Int {
					base.Fatalf("unexpected BasicLit Kind")
				}
				if constant.Compare(lit.Val(), token.GEQ, constant.MakeInt64(0)) {
					if !base.LiteralAllocHash.MatchPos(n.Pos(), nil) {
						// De-selected by literal alloc optimizations debug hash.
						return
					}
					// Preserve any side effects of the original expression, then replace it.
					assignTemp(n.Pos(), *r, n.PtrInit())
					*r = ir.NewBasicLit(n.Pos(), (*r).Type(), lit.Val())
				}
			case ir.OLEN:
				x := ro.StaticValue(s.(*ir.UnaryExpr).X)
				if x.Op() == ir.OSLICELIT {
					x := x.(*ir.CompLitExpr)
					// Preserve any side effects of the original expression, then update the value.
					assignTemp(n.Pos(), *r, n.PtrInit())
					*r = ir.NewBasicLit(n.Pos(), types.Types[types.TINT], constant.MakeInt64(x.Len))
				}
			}
		}
	case ir.OCONVIFACE:
		// Check if we can replace a non-constant expression in an interface conversion with
		// a literal to avoid heap allocating the underlying interface value.
		conv := n.(*ir.ConvExpr)
		if conv.X.Op() != ir.OLITERAL && !conv.X.Type().IsInterface() {
			// TODO(thepudds): likely could avoid some work by tightening the check of conv.X's type.
			// Look up a cached ReassignOracle for the function, lazily computing one if needed.
			ro := b.reassignOracle(fn)
			if ro == nil {
				base.Fatalf("no ReassignOracle for function %v with closure parent %v", fn, fn.ClosureParent)
			}
			v := ro.StaticValue(conv.X)
			if v != nil && v.Op() == ir.OLITERAL && ir.ValidTypeForConst(conv.X.Type(), v.Val()) {
				if !base.LiteralAllocHash.MatchPos(n.Pos(), nil) {
					// De-selected by literal alloc optimizations debug hash.
					return
				}
				if base.Debug.EscapeDebug >= 3 {
					base.WarnfAt(n.Pos(), "rewriting OCONVIFACE value from %v (%v) to %v (%v)", conv.X, conv.X.Type(), v, v.Type())
				}
				// Preserve any side effects of the original expression, then replace it.
				assignTemp(conv.Pos(), conv.X, conv.PtrInit())
				v := v.(*ir.BasicLit)
				conv.X = ir.NewBasicLit(conv.Pos(), conv.X.Type(), v.Val())
				typecheck.Expr(conv)
			}
		}
	}
}

// reassignOracle returns an initialized *ir.ReassignOracle for fn.
// If fn is a closure, it returns the ReassignOracle for the ultimate parent.
//
// A new ReassignOracle is initialized lazily if needed, and the result
// is cached to reduce duplicative work of preparing a ReassignOracle.
func (b *batch) reassignOracle(fn *ir.Func) *ir.ReassignOracle {
	if ro, ok := b.reassignOracles[fn]; ok {
		return ro // Hit.
	}

	// For closures, we want the ultimate parent's ReassignOracle,
	// so walk up the parent chain, if any.
	f := fn
	for f.ClosureParent != nil && !f.ClosureParent.IsPackageInit() {
		f = f.ClosureParent
	}

	if f != fn {
		// We found a parent.
		ro := b.reassignOracles[f]
		if ro != nil {
			// Hit, via a parent. Before returning, store this ro for the original fn as well.
			b.reassignOracles[fn] = ro
			return ro
		}
	}

	// Miss. We did not find a ReassignOracle for fn or a parent, so lazily create one.
	ro := &ir.ReassignOracle{}
	ro.Init(f)

	// Cache the answer for the original fn.
	b.reassignOracles[fn] = ro
	if f != fn {
		// Cache for the parent as well.
		b.reassignOracles[f] = ro
	}
	return ro
}

```

// === FILE: references!/go/src/cmd/compile/internal/escape/expr.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
)

// expr models evaluating an expression n and flowing the result into
// hole k.
func (e *escape) expr(k hole, n ir.Node) {
	if n == nil {
		return
	}
	e.stmts(n.Init())
	e.exprSkipInit(k, n)
}

func (e *escape) exprSkipInit(k hole, n ir.Node) {
	if n == nil {
		return
	}

	lno := ir.SetPos(n)
	defer func() {
		base.Pos = lno
	}()

	if k.derefs >= 0 && !n.Type().IsUntyped() && !n.Type().HasPointers() {
		k.dst = &e.blankLoc
	}

	switch n.Op() {
	default:
		base.Fatalf("unexpected expr: %s %v", n.Op().String(), n)

	case ir.OLITERAL, ir.ONIL, ir.OGETG, ir.OGETCALLERSP, ir.OTYPE, ir.OMETHEXPR, ir.OLINKSYMOFFSET:
		// nop

	case ir.ONAME:
		n := n.(*ir.Name)
		if n.Class == ir.PFUNC || n.Class == ir.PEXTERN {
			return
		}
		e.flow(k, e.oldLoc(n))

	case ir.OPLUS, ir.ONEG, ir.OBITNOT, ir.ONOT:
		n := n.(*ir.UnaryExpr)
		e.discard(n.X)
	case ir.OADD, ir.OSUB, ir.OOR, ir.OXOR, ir.OMUL, ir.ODIV, ir.OMOD, ir.OLSH, ir.ORSH, ir.OAND, ir.OANDNOT, ir.OEQ, ir.ONE, ir.OLT, ir.OLE, ir.OGT, ir.OGE:
		n := n.(*ir.BinaryExpr)
		e.discard(n.X)
		e.discard(n.Y)
	case ir.OANDAND, ir.OOROR:
		n := n.(*ir.LogicalExpr)
		e.discard(n.X)
		e.discard(n.Y)
	case ir.OADDR:
		n := n.(*ir.AddrExpr)
		e.expr(k.addr(n, "address-of"), n.X) // "address-of"
	case ir.ODEREF:
		n := n.(*ir.StarExpr)
		e.expr(k.deref(n, "indirection"), n.X) // "indirection"
	case ir.ODOT, ir.ODOTMETH, ir.ODOTINTER:
		n := n.(*ir.SelectorExpr)
		e.expr(k.note(n, "dot"), n.X)
	case ir.ODOTPTR:
		n := n.(*ir.SelectorExpr)
		e.expr(k.deref(n, "dot of pointer"), n.X) // "dot of pointer"
	case ir.ODOTTYPE, ir.ODOTTYPE2:
		n := n.(*ir.TypeAssertExpr)
		e.expr(k.dotType(n.Type(), n, "dot"), n.X)
	case ir.ODYNAMICDOTTYPE, ir.ODYNAMICDOTTYPE2:
		n := n.(*ir.DynamicTypeAssertExpr)
		e.expr(k.dotType(n.Type(), n, "dot"), n.X)
		// n.T doesn't need to be tracked; it always points to read-only storage.
	case ir.OINDEX:
		n := n.(*ir.IndexExpr)
		if n.X.Type().IsArray() {
			e.expr(k.note(n, "fixed-array-index-of"), n.X)
		} else {
			// TODO(mdempsky): Fix why reason text.
			e.expr(k.deref(n, "dot of pointer"), n.X)
		}
		e.discard(n.Index)
	case ir.OINDEXMAP:
		n := n.(*ir.IndexExpr)
		e.discard(n.X)
		e.discard(n.Index)
	case ir.OSLICE, ir.OSLICEARR, ir.OSLICE3, ir.OSLICE3ARR, ir.OSLICESTR:
		n := n.(*ir.SliceExpr)
		e.expr(k.note(n, "slice"), n.X)
		e.discard(n.Low)
		e.discard(n.High)
		e.discard(n.Max)

	case ir.OCONV, ir.OCONVNOP:
		n := n.(*ir.ConvExpr)
		if (ir.ShouldCheckPtr(e.curfn, 2) || ir.ShouldAsanCheckPtr(e.curfn)) && n.Type().IsUnsafePtr() && n.X.Type().IsPtr() {
			// When -d=checkptr=2 or -asan is enabled,
			// treat conversions to unsafe.Pointer as an
			// escaping operation. This allows better
			// runtime instrumentation, since we can more
			// easily detect object boundaries on the heap
			// than the stack.
			e.assignHeap(n.X, "conversion to unsafe.Pointer", n)
		} else if n.Type().IsUnsafePtr() && n.X.Type().IsUintptr() {
			e.unsafeValue(k, n.X)
		} else {
			e.expr(k, n.X)
		}
	case ir.OCONVIFACE:
		n := n.(*ir.ConvExpr)
		if !n.X.Type().IsInterface() && !types.IsDirectIface(n.X.Type()) {
			k = e.spill(k, n)
		}
		e.expr(k.note(n, "interface-converted"), n.X)
	case ir.OMAKEFACE:
		n := n.(*ir.BinaryExpr)
		// Note: n.X is not needed because it can never point to memory that might escape.
		e.expr(k, n.Y)
	case ir.OITAB, ir.OIDATA, ir.OSPTR:
		n := n.(*ir.UnaryExpr)
		e.expr(k, n.X)
	case ir.OSLICE2ARR:
		// Converting a slice to array is effectively a deref.
		n := n.(*ir.ConvExpr)
		e.expr(k.deref(n, "slice-to-array"), n.X)
	case ir.OSLICE2ARRPTR:
		// the slice pointer flows directly to the result
		n := n.(*ir.ConvExpr)
		e.expr(k, n.X)
	case ir.ORECV:
		n := n.(*ir.UnaryExpr)
		e.discard(n.X)

	case ir.OCALLMETH, ir.OCALLFUNC, ir.OCALLINTER, ir.OINLCALL,
		ir.OLEN, ir.OCAP, ir.OMIN, ir.OMAX, ir.OCOMPLEX, ir.OREAL, ir.OIMAG, ir.OAPPEND, ir.OCOPY, ir.ORECOVER,
		ir.OUNSAFEADD, ir.OUNSAFESLICE, ir.OUNSAFESTRING, ir.OUNSAFESTRINGDATA, ir.OUNSAFESLICEDATA:
		e.call([]hole{k}, n)

	case ir.ONEW:
		n := n.(*ir.UnaryExpr)
		e.spill(k, n)

	case ir.OMAKESLICE:
		n := n.(*ir.MakeExpr)
		e.spill(k, n)
		e.discard(n.Len)
		e.discard(n.Cap)
	case ir.OMAKECHAN:
		n := n.(*ir.MakeExpr)
		e.discard(n.Len)
	case ir.OMAKEMAP:
		n := n.(*ir.MakeExpr)
		e.spill(k, n)
		e.discard(n.Len)

	case ir.OMETHVALUE:
		// Flow the receiver argument to both the closure and
		// to the receiver parameter.

		n := n.(*ir.SelectorExpr)
		closureK := e.spill(k, n)

		m := n.Selection

		// We don't know how the method value will be called
		// later, so conservatively assume the result
		// parameters all flow to the heap.
		//
		// TODO(mdempsky): Change ks into a callback, so that
		// we don't have to create this slice?
		var ks []hole
		for i := m.Type.NumResults(); i > 0; i-- {
			ks = append(ks, e.heapHole())
		}
		name, _ := m.Nname.(*ir.Name)
		paramK := e.tagHole(ks, name, m.Type.Recv())

		e.expr(e.teeHole(paramK, closureK), n.X)

	case ir.OPTRLIT:
		n := n.(*ir.AddrExpr)
		e.expr(e.spill(k, n), n.X)

	case ir.OARRAYLIT:
		n := n.(*ir.CompLitExpr)
		for _, elt := range n.List {
			if elt.Op() == ir.OKEY {
				elt = elt.(*ir.KeyExpr).Value
			}
			e.expr(k.note(n, "array literal element"), elt)
		}

	case ir.OSLICELIT:
		n := n.(*ir.CompLitExpr)
		k = e.spill(k, n)

		for _, elt := range n.List {
			if elt.Op() == ir.OKEY {
				elt = elt.(*ir.KeyExpr).Value
			}
			e.expr(k.note(n, "slice-literal-element"), elt)
		}

	case ir.OSTRUCTLIT:
		n := n.(*ir.CompLitExpr)
		for _, elt := range n.List {
			e.expr(k.note(n, "struct literal element"), elt.(*ir.StructKeyExpr).Value)
		}

	case ir.OMAPLIT:
		n := n.(*ir.CompLitExpr)
		e.spill(k, n)

		// Map keys and values are always stored in the heap.
		for _, elt := range n.List {
			elt := elt.(*ir.KeyExpr)
			e.assignHeap(elt.Key, "map literal key", n)
			e.assignHeap(elt.Value, "map literal value", n)
		}

	case ir.OCLOSURE:
		n := n.(*ir.ClosureExpr)
		k = e.spill(k, n)
		e.closures = append(e.closures, closure{k, n})

		if fn := n.Func; fn.IsClosure() {
			for _, cv := range fn.ClosureVars {
				if loc := e.oldLoc(cv); !loc.captured {
					loc.captured = true

					// Ignore reassignments to the variable in straightline code
					// preceding the first capture by a closure.
					if loc.loopDepth == e.loopDepth {
						loc.reassigned = false
					}
				}
			}

			for _, n := range fn.Dcl {
				// Add locations for local variables of the
				// closure, if needed, in case we're not including
				// the closure func in the batch for escape
				// analysis (happens for escape analysis called
				// from reflectdata.methodWrapper)
				if n.Op() == ir.ONAME && n.Opt == nil {
					e.with(fn).newLoc(n, true)
				}
			}
			e.walkFunc(fn)
		}

	case ir.ORUNES2STR, ir.OBYTES2STR, ir.OSTR2RUNES, ir.OSTR2BYTES, ir.ORUNESTR:
		n := n.(*ir.ConvExpr)
		e.spill(k, n)
		e.discard(n.X)

	case ir.OADDSTR:
		n := n.(*ir.AddStringExpr)
		e.spill(k, n)

		// Arguments of OADDSTR never escape;
		// runtime.concatstrings makes sure of that.
		e.discards(n.List)

	case ir.ODYNAMICTYPE:
		// Nothing to do - argument is a *runtime._type (+ maybe a *runtime.itab) pointing to static data section
	}
}

// unsafeValue evaluates a uintptr-typed arithmetic expression looking
// for conversions from an unsafe.Pointer.
func (e *escape) unsafeValue(k hole, n ir.Node) {
	if n.Type().Kind() != types.TUINTPTR {
		base.Fatalf("unexpected type %v for %v", n.Type(), n)
	}
	if k.addrtaken {
		base.Fatalf("unexpected addrtaken")
	}

	e.stmts(n.Init())

	switch n.Op() {
	case ir.OCONV, ir.OCONVNOP:
		n := n.(*ir.ConvExpr)
		if n.X.Type().IsUnsafePtr() {
			e.expr(k, n.X)
		} else {
			e.discard(n.X)
		}
	case ir.ODOTPTR:
		n := n.(*ir.SelectorExpr)
		if ir.IsReflectHeaderDataField(n) {
			e.expr(k.deref(n, "reflect.Header.Data"), n.X)
		} else {
			e.discard(n.X)
		}
	case ir.OPLUS, ir.ONEG, ir.OBITNOT:
		n := n.(*ir.UnaryExpr)
		e.unsafeValue(k, n.X)
	case ir.OADD, ir.OSUB, ir.OOR, ir.OXOR, ir.OMUL, ir.ODIV, ir.OMOD, ir.OAND, ir.OANDNOT:
		n := n.(*ir.BinaryExpr)
		e.unsafeValue(k, n.X)
		e.unsafeValue(k, n.Y)
	case ir.OLSH, ir.ORSH:
		n := n.(*ir.BinaryExpr)
		e.unsafeValue(k, n.X)
		// RHS need not be uintptr-typed (#32959) and can't meaningfully
		// flow pointers anyway.
		e.discard(n.Y)
	default:
		e.exprSkipInit(e.discardHole(), n)
	}
}

// discard evaluates an expression n for side-effects, but discards
// its value.
func (e *escape) discard(n ir.Node) {
	e.expr(e.discardHole(), n)
}

func (e *escape) discards(l ir.Nodes) {
	for _, n := range l {
		e.discard(n)
	}
}

// spill allocates a new location associated with expression n, flows
// its address to k, and returns a hole that flows values to it. It's
// intended for use with most expressions that allocate storage.
func (e *escape) spill(k hole, n ir.Node) hole {
	loc := e.newLoc(n, false)
	e.flow(k.addr(n, "spill"), loc)
	return loc.asHole()
}

```

// === FILE: references!/go/src/cmd/compile/internal/escape/graph.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/logopt"
	"cmd/compile/internal/types"
	"fmt"
)

// Below we implement the methods for walking the AST and recording
// data flow edges. Note that because a sub-expression might have
// side-effects, it's important to always visit the entire AST.
//
// For example, write either:
//
//     if x {
//         e.discard(n.Left)
//     } else {
//         e.value(k, n.Left)
//     }
//
// or
//
//     if x {
//         k = e.discardHole()
//     }
//     e.value(k, n.Left)
//
// Do NOT write:
//
//    // BAD: possibly loses side-effects within n.Left
//    if !x {
//        e.value(k, n.Left)
//    }

// A location represents an abstract location that stores a Go
// variable.
type location struct {
	n         ir.Node  // represented variable or expression, if any
	curfn     *ir.Func // enclosing function
	edges     []edge   // incoming edges
	loopDepth int      // loopDepth at declaration

	// resultIndex records the tuple index (starting at 1) for
	// PPARAMOUT variables within their function's result type.
	// For non-PPARAMOUT variables it's 0.
	resultIndex int

	// derefs and walkgen are used during walkOne to track the
	// minimal dereferences from the walk root.
	derefs  int // >= -1
	walkgen uint32

	// dst and dstEdgeindex track the next immediate assignment
	// destination location during walkone, along with the index
	// of the edge pointing back to this location.
	dst        *location
	dstEdgeIdx int

	// queuedWalkAll is used by walkAll to track whether this location is
	// in its work queue.
	queuedWalkAll bool

	// queuedWalkOne is used by walkOne to track whether this location is
	// in its work queue. The value is the walkgen when this location was
	// last queued for walkOne, or 0 if it's not currently queued.
	queuedWalkOne uint32

	// attrs is a bitset of location attributes.
	attrs locAttr

	// paramEsc records the represented parameter's leak set.
	paramEsc leaks

	captured   bool // has a closure captured this variable?
	reassigned bool // has this variable been reassigned?
	addrtaken  bool // has this variable's address been taken?
	param      bool // is this variable a parameter (ONAME of class ir.PPARAM)?
	paramOut   bool // is this variable an out parameter (ONAME of class ir.PPARAMOUT)?
}

type locAttr uint8

const (
	// attrEscapes indicates whether the represented variable's address
	// escapes; that is, whether the variable must be heap allocated.
	attrEscapes locAttr = 1 << iota

	// attrPersists indicates whether the represented expression's
	// address outlives the statement; that is, whether its storage
	// cannot be immediately reused.
	attrPersists

	// attrMutates indicates whether pointers that are reachable from
	// this location may have their addressed memory mutated. This is
	// used to detect string->[]byte conversions that can be safely
	// optimized away.
	attrMutates

	// attrCalls indicates whether closures that are reachable from this
	// location may be called without tracking their results. This is
	// used to better optimize indirect closure calls.
	attrCalls
)

func (l *location) hasAttr(attr locAttr) bool { return l.attrs&attr != 0 }

// An edge represents an assignment edge between two Go variables.
type edge struct {
	src    *location
	derefs int // >= -1
	notes  *note
}

func (l *location) asHole() hole {
	return hole{dst: l}
}

// leak records that parameter l leaks to sink.
func (l *location) leakTo(sink *location, derefs int) {
	// If sink is a result parameter that doesn't escape (#44614)
	// and we can fit return bits into the escape analysis tag,
	// then record as a result leak.
	if !sink.hasAttr(attrEscapes) && sink.isName(ir.PPARAMOUT) && sink.curfn == l.curfn {
		ri := sink.resultIndex - 1
		if ri < numEscResults {
			// Leak to result parameter.
			l.paramEsc.AddResult(ri, derefs)
			return
		}
	}

	// Otherwise, record as heap leak.
	l.paramEsc.AddHeap(derefs)
}

func (l *location) isName(c ir.Class) bool {
	return l.n != nil && l.n.Op() == ir.ONAME && l.n.(*ir.Name).Class == c
}

// A hole represents a context for evaluation of a Go
// expression. E.g., when evaluating p in "x = **p", we'd have a hole
// with dst==x and derefs==2.
type hole struct {
	dst    *location
	derefs int // >= -1
	notes  *note

	// addrtaken indicates whether this context is taking the address of
	// the expression, independent of whether the address will actually
	// be stored into a variable.
	addrtaken bool
}

type note struct {
	next  *note
	where ir.Node
	why   string
}

func (k hole) note(where ir.Node, why string) hole {
	if where == nil || why == "" {
		base.Fatalf("note: missing where/why")
	}
	if base.Flag.LowerM >= 2 || logopt.Enabled() {
		k.notes = &note{
			next:  k.notes,
			where: where,
			why:   why,
		}
	}
	return k
}

func (k hole) shift(delta int) hole {
	k.derefs += delta
	if k.derefs < -1 {
		base.Fatalf("derefs underflow: %v", k.derefs)
	}
	k.addrtaken = delta < 0
	return k
}

func (k hole) deref(where ir.Node, why string) hole { return k.shift(1).note(where, why) }
func (k hole) addr(where ir.Node, why string) hole  { return k.shift(-1).note(where, why) }

func (k hole) dotType(t *types.Type, where ir.Node, why string) hole {
	if !t.IsInterface() && !types.IsDirectIface(t) {
		k = k.shift(1)
	}
	return k.note(where, why)
}

func (b *batch) flow(k hole, src *location) {
	if k.addrtaken {
		src.addrtaken = true
	}

	dst := k.dst
	if dst == &b.blankLoc {
		return
	}
	if dst == src && k.derefs >= 0 { // dst = dst, dst = *dst, ...
		return
	}
	if dst.hasAttr(attrEscapes) && k.derefs < 0 { // dst = &src
		if base.Flag.LowerM >= 2 || logopt.Enabled() {
			pos := base.FmtPos(src.n.Pos())
			if base.Flag.LowerM >= 2 {
				fmt.Printf("%s: %v escapes to heap in %v:\n", pos, src.n, ir.FuncName(src.curfn))
			}
			explanation := b.explainFlow(pos, dst, src, k.derefs, k.notes, []*logopt.LoggedOpt{})
			if logopt.Enabled() {
				var e_curfn *ir.Func // TODO(mdempsky): Fix.
				logopt.LogOpt(src.n.Pos(), "escapes", "escape", ir.FuncName(e_curfn), fmt.Sprintf("%v escapes to heap", src.n), explanation)
			}

		}
		src.attrs |= attrEscapes | attrPersists | attrMutates | attrCalls
		return
	}

	// TODO(mdempsky): Deduplicate edges?
	dst.edges = append(dst.edges, edge{src: src, derefs: k.derefs, notes: k.notes})
}

func (b *batch) heapHole() hole    { return b.heapLoc.asHole() }
func (b *batch) mutatorHole() hole { return b.mutatorLoc.asHole() }
func (b *batch) calleeHole() hole  { return b.calleeLoc.asHole() }
func (b *batch) discardHole() hole { return b.blankLoc.asHole() }

func (b *batch) oldLoc(n *ir.Name) *location {
	if n.Canonical().Opt == nil {
		base.FatalfAt(n.Pos(), "%v has no location", n)
	}
	return n.Canonical().Opt.(*location)
}

func (e *escape) newLoc(n ir.Node, persists bool) *location {
	if e.curfn == nil {
		base.Fatalf("e.curfn isn't set")
	}
	if n != nil && n.Type() != nil && n.Type().NotInHeap() {
		base.ErrorfAt(n.Pos(), 0, "%v is incomplete (or unallocatable); stack allocation disallowed", n.Type())
	}

	if n != nil && n.Op() == ir.ONAME {
		if canon := n.(*ir.Name).Canonical(); n != canon {
			base.FatalfAt(n.Pos(), "newLoc on non-canonical %v (canonical is %v)", n, canon)
		}
	}
	loc := &location{
		n:         n,
		curfn:     e.curfn,
		loopDepth: e.loopDepth,
	}
	if loc.isName(ir.PPARAM) {
		loc.param = true
	} else if loc.isName(ir.PPARAMOUT) {
		loc.paramOut = true
	}

	if persists {
		loc.attrs |= attrPersists
	}
	e.allLocs = append(e.allLocs, loc)
	if n != nil {
		if n.Op() == ir.ONAME {
			n := n.(*ir.Name)
			if n.Class == ir.PPARAM && n.Curfn == nil {
				// ok; hidden parameter
			} else if n.Curfn != e.curfn {
				base.FatalfAt(n.Pos(), "curfn mismatch: %v != %v for %v", n.Curfn, e.curfn, n)
			}

			if n.Opt != nil {
				base.FatalfAt(n.Pos(), "%v already has a location", n)
			}
			n.Opt = loc
		}
	}
	return loc
}

// teeHole returns a new hole that flows into each hole of ks,
// similar to the Unix tee(1) command.
func (e *escape) teeHole(ks ...hole) hole {
	if len(ks) == 0 {
		return e.discardHole()
	}
	if len(ks) == 1 {
		return ks[0]
	}
	// TODO(mdempsky): Optimize if there's only one non-discard hole?

	// Given holes "l1 = _", "l2 = **_", "l3 = *_", ..., create a
	// new temporary location ltmp, wire it into place, and return
	// a hole for "ltmp = _".
	loc := e.newLoc(nil, false)
	for _, k := range ks {
		// N.B., "p = &q" and "p = &tmp; tmp = q" are not
		// semantically equivalent. To combine holes like "l1
		// = _" and "l2 = &_", we'd need to wire them as "l1 =
		// *ltmp" and "l2 = ltmp" and return "ltmp = &_"
		// instead.
		if k.derefs < 0 {
			base.Fatalf("teeHole: negative derefs")
		}

		e.flow(k, loc)
	}
	return loc.asHole()
}

// later returns a new hole that flows into k, but some time later.
// Its main effect is to prevent immediate reuse of temporary
// variables introduced during Order.
func (e *escape) later(k hole) hole {
	loc := e.newLoc(nil, true)
	e.flow(k, loc)
	return loc.asHole()
}

// Fmt is called from node printing to print information about escape analysis results.
func Fmt(n ir.Node) string {
	text := ""
	switch n.Esc() {
	case ir.EscUnknown:
		break

	case ir.EscHeap:
		text = "esc(h)"

	case ir.EscNone:
		text = "esc(no)"

	case ir.EscNever:
		text = "esc(N)"

	default:
		text = fmt.Sprintf("esc(%d)", n.Esc())
	}

	if n.Op() == ir.ONAME {
		n := n.(*ir.Name)
		if loc, ok := n.Opt.(*location); ok && loc.loopDepth != 0 {
			if text != "" {
				text += " "
			}
			text += fmt.Sprintf("ld(%d)", loc.loopDepth)
		}
	}

	return text
}

```

// === FILE: references!/go/src/cmd/compile/internal/escape/leaks.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"cmd/compile/internal/base"
	"math"
	"strings"
)

// A leaks represents a set of assignment flows from a parameter to
// the heap, mutator, callee, or to any of its function's (first
// numEscResults) result parameters.
type leaks [8]uint8

const (
	leakHeap = iota
	leakMutator
	leakCallee
	leakResult0
)

const numEscResults = len(leaks{}) - leakResult0

// Heap returns the minimum deref count of any assignment flow from l
// to the heap. If no such flows exist, Heap returns -1.
func (l leaks) Heap() int { return l.get(leakHeap) }

// Mutator returns the minimum deref count of any assignment flow from
// l to the pointer operand of an indirect assignment statement. If no
// such flows exist, Mutator returns -1.
func (l leaks) Mutator() int { return l.get(leakMutator) }

// Callee returns the minimum deref count of any assignment flow from
// l to the callee operand of call expression. If no such flows exist,
// Callee returns -1.
func (l leaks) Callee() int { return l.get(leakCallee) }

// Result returns the minimum deref count of any assignment flow from
// l to its function's i'th result parameter. If no such flows exist,
// Result returns -1.
func (l leaks) Result(i int) int { return l.get(leakResult0 + i) }

// AddHeap adds an assignment flow from l to the heap.
func (l *leaks) AddHeap(derefs int) { l.add(leakHeap, derefs) }

// AddMutator adds a flow from l to the mutator (i.e., a pointer
// operand of an indirect assignment statement).
func (l *leaks) AddMutator(derefs int) { l.add(leakMutator, derefs) }

// AddCallee adds an assignment flow from l to the callee operand of a
// call expression.
func (l *leaks) AddCallee(derefs int) { l.add(leakCallee, derefs) }

// AddResult adds an assignment flow from l to its function's i'th
// result parameter.
func (l *leaks) AddResult(i, derefs int) { l.add(leakResult0+i, derefs) }

func (l leaks) get(i int) int { return int(l[i]) - 1 }

func (l *leaks) add(i, derefs int) {
	if old := l.get(i); old < 0 || derefs < old {
		l.set(i, derefs)
	}
}

func (l *leaks) set(i, derefs int) {
	v := derefs + 1
	if v < 0 {
		base.Fatalf("invalid derefs count: %v", derefs)
	}
	if v > math.MaxUint8 {
		v = math.MaxUint8
	}

	l[i] = uint8(v)
}

// Optimize removes result flow paths that are equal in length or
// longer than the shortest heap flow path.
func (l *leaks) Optimize() {
	// If we have a path to the heap, then there's no use in
	// keeping equal or longer paths elsewhere.
	if x := l.Heap(); x >= 0 {
		for i := 1; i < len(*l); i++ {
			if l.get(i) >= x {
				l.set(i, -1)
			}
		}
	}
}

var leakTagCache = map[leaks]string{}

// Encode converts l into a binary string for export data.
func (l leaks) Encode() string {
	if l.Heap() == 0 {
		// Space optimization: empty string encodes more
		// efficiently in export data.
		return ""
	}
	if s, ok := leakTagCache[l]; ok {
		return s
	}

	n := len(l)
	for n > 0 && l[n-1] == 0 {
		n--
	}
	s := "esc:" + string(l[:n])
	leakTagCache[l] = s
	return s
}

// parseLeaks parses a binary string representing a leaks.
func parseLeaks(s string) leaks {
	var l leaks
	if !strings.HasPrefix(s, "esc:") {
		l.AddHeap(0)
		return l
	}
	copy(l[:], s[4:])
	return l
}

func ParseLeaks(s string) leaks {
	return parseLeaks(s)
}

// Any reports whether the value flows anywhere at all.
func (l leaks) Any() bool {
	// TODO: do mutator/callee matter?
	if l.Heap() >= 0 || l.Mutator() >= 0 || l.Callee() >= 0 {
		return true
	}
	for i := range numEscResults {
		if l.Result(i) >= 0 {
			return true
		}
	}
	return false
}

```

// === FILE: references!/go/src/cmd/compile/internal/escape/solve.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/logopt"
	"cmd/internal/src"
	"fmt"
	"math/bits"
	"strings"
)

// walkAll computes the minimal dereferences between all pairs of
// locations.
func (b *batch) walkAll() {
	// We use a work queue to keep track of locations that we need
	// to visit, and repeatedly walk until we reach a fixed point.
	//
	// We walk once from each location (including the heap), and
	// then re-enqueue each location on its transition from
	// !persists->persists and !escapes->escapes, which can each
	// happen at most once. So we take Θ(len(e.allLocs)) walks.

	// Queue of locations to walk. Has enough room for b.allLocs
	// plus b.heapLoc, b.mutatorLoc, b.calleeLoc.
	todo := newQueue(len(b.allLocs) + 3)

	enqueue := func(loc *location) {
		if !loc.queuedWalkAll {
			loc.queuedWalkAll = true
			if loc.hasAttr(attrEscapes) {
				// Favor locations that escape to the heap,
				// which in some cases allows attrEscape to
				// propagate faster.
				todo.pushFront(loc)
			} else {
				todo.pushBack(loc)
			}
		}
	}

	for _, loc := range b.allLocs {
		todo.pushFront(loc)
		// TODO(thepudds): clean up setting queuedWalkAll.
		loc.queuedWalkAll = true
	}
	todo.pushFront(&b.mutatorLoc)
	todo.pushFront(&b.calleeLoc)
	todo.pushFront(&b.heapLoc)

	b.mutatorLoc.queuedWalkAll = true
	b.calleeLoc.queuedWalkAll = true
	b.heapLoc.queuedWalkAll = true

	var walkgen uint32
	for todo.len() > 0 {
		root := todo.popFront()
		root.queuedWalkAll = false
		walkgen++
		b.walkOne(root, walkgen, enqueue)
	}
}

// walkOne computes the minimal number of dereferences from root to
// all other locations.
func (b *batch) walkOne(root *location, walkgen uint32, enqueue func(*location)) {
	// The data flow graph has negative edges (from addressing
	// operations), so we use the Bellman-Ford algorithm. However,
	// we don't have to worry about infinite negative cycles since
	// we bound intermediate dereference counts to 0.

	root.walkgen = walkgen
	root.derefs = 0
	root.dst = nil

	if root.hasAttr(attrCalls) {
		if clo, ok := root.n.(*ir.ClosureExpr); ok {
			if fn := clo.Func; b.inMutualBatch(fn.Nname) && !fn.ClosureResultsLost() {
				fn.SetClosureResultsLost(true)

				// Re-flow from the closure's results, now that we're aware
				// we lost track of them.
				for _, result := range fn.Type().Results() {
					enqueue(b.oldLoc(result.Nname.(*ir.Name)))
				}
			}
		}
	}

	todo := newQueue(1)
	todo.pushFront(root)

	for todo.len() > 0 {
		l := todo.popFront()
		l.queuedWalkOne = 0 // no longer queued for walkOne

		derefs := l.derefs
		var newAttrs locAttr

		// If l.derefs < 0, then l's address flows to root.
		addressOf := derefs < 0
		if addressOf {
			// For a flow path like "root = &l; l = x",
			// l's address flows to root, but x's does
			// not. We recognize this by lower bounding
			// derefs at 0.
			derefs = 0

			// If l's address flows somewhere that
			// outlives it, then l needs to be heap
			// allocated.
			if b.outlives(root, l) {
				if !l.hasAttr(attrEscapes) && (logopt.Enabled() || base.Flag.LowerM >= 2) {
					if base.Flag.LowerM >= 2 {
						fmt.Printf("%s: %v escapes to heap in %v:\n", base.FmtPos(l.n.Pos()), l.n, ir.FuncName(l.curfn))
					}
					explanation := b.explainPath(root, l)
					if logopt.Enabled() {
						var e_curfn *ir.Func // TODO(mdempsky): Fix.
						logopt.LogOpt(l.n.Pos(), "escape", "escape", ir.FuncName(e_curfn), fmt.Sprintf("%v escapes to heap", l.n), explanation)
					}
				}
				newAttrs |= attrEscapes | attrPersists | attrMutates | attrCalls
			} else
			// If l's address flows to a persistent location, then l needs
			// to persist too.
			if root.hasAttr(attrPersists) {
				newAttrs |= attrPersists
			}
		}

		if derefs == 0 {
			newAttrs |= root.attrs & (attrMutates | attrCalls)
		}

		// l's value flows to root. If l is a function
		// parameter and root is the heap or a
		// corresponding result parameter, then record
		// that value flow for tagging the function
		// later.
		if l.param {
			if b.outlives(root, l) {
				if !l.hasAttr(attrEscapes) && (logopt.Enabled() || base.Flag.LowerM >= 2) {
					if base.Flag.LowerM >= 2 {
						fmt.Printf("%s: parameter %v leaks to %s for %v with derefs=%d:\n", base.FmtPos(l.n.Pos()), l.n, b.explainLoc(root), ir.FuncName(l.curfn), derefs)
					}
					explanation := b.explainPath(root, l)
					if logopt.Enabled() {
						var e_curfn *ir.Func // TODO(mdempsky): Fix.
						logopt.LogOpt(l.n.Pos(), "leak", "escape", ir.FuncName(e_curfn),
							fmt.Sprintf("parameter %v leaks to %s with derefs=%d", l.n, b.explainLoc(root), derefs), explanation)
					}
				}
				l.leakTo(root, derefs)
			}
			if root.hasAttr(attrMutates) {
				l.paramEsc.AddMutator(derefs)
			}
			if root.hasAttr(attrCalls) {
				l.paramEsc.AddCallee(derefs)
			}
		}

		if newAttrs&^l.attrs != 0 {
			l.attrs |= newAttrs
			enqueue(l)
			if l.attrs&attrEscapes != 0 {
				continue
			}
		}

		for i, edge := range l.edges {
			if edge.src.hasAttr(attrEscapes) {
				continue
			}
			d := derefs + edge.derefs
			if edge.src.walkgen != walkgen || edge.src.derefs > d {
				edge.src.walkgen = walkgen
				edge.src.derefs = d
				edge.src.dst = l
				edge.src.dstEdgeIdx = i
				// Check if already queued in todo.
				if edge.src.queuedWalkOne != walkgen {
					edge.src.queuedWalkOne = walkgen // Mark queued for this walkgen.

					// Place at the back to possibly give time for
					// other possible attribute changes to src.
					todo.pushBack(edge.src)
				}
			}
		}
	}
}

// explainPath prints an explanation of how src flows to the walk root.
func (b *batch) explainPath(root, src *location) []*logopt.LoggedOpt {
	visited := make(map[*location]bool)
	pos := base.FmtPos(src.n.Pos())
	var explanation []*logopt.LoggedOpt
	for {
		// Prevent infinite loop.
		if visited[src] {
			if base.Flag.LowerM >= 2 {
				fmt.Printf("%s:   warning: truncated explanation due to assignment cycle; see golang.org/issue/35518\n", pos)
			}
			break
		}
		visited[src] = true
		dst := src.dst
		edge := &dst.edges[src.dstEdgeIdx]
		if edge.src != src {
			base.Fatalf("path inconsistency: %v != %v", edge.src, src)
		}

		explanation = b.explainFlow(pos, dst, src, edge.derefs, edge.notes, explanation)

		if dst == root {
			break
		}
		src = dst
	}

	return explanation
}

func (b *batch) explainFlow(pos string, dst, srcloc *location, derefs int, notes *note, explanation []*logopt.LoggedOpt) []*logopt.LoggedOpt {
	ops := "&"
	if derefs >= 0 {
		ops = strings.Repeat("*", derefs)
	}
	print := base.Flag.LowerM >= 2

	flow := fmt.Sprintf("   flow: %s ← %s%v:", b.explainLoc(dst), ops, b.explainLoc(srcloc))
	if print {
		fmt.Printf("%s:%s\n", pos, flow)
	}
	if logopt.Enabled() {
		var epos src.XPos
		if notes != nil {
			epos = notes.where.Pos()
		} else if srcloc != nil && srcloc.n != nil {
			epos = srcloc.n.Pos()
		}
		var e_curfn *ir.Func // TODO(mdempsky): Fix.
		explanation = append(explanation, logopt.NewLoggedOpt(epos, epos, "escflow", "escape", ir.FuncName(e_curfn), flow))
	}

	for note := notes; note != nil; note = note.next {
		if print {
			fmt.Printf("%s:     from %v (%v) at %s\n", pos, note.where, note.why, base.FmtPos(note.where.Pos()))
		}
		if logopt.Enabled() {
			var e_curfn *ir.Func // TODO(mdempsky): Fix.
			notePos := note.where.Pos()
			explanation = append(explanation, logopt.NewLoggedOpt(notePos, notePos, "escflow", "escape", ir.FuncName(e_curfn),
				fmt.Sprintf("     from %v (%v)", note.where, note.why)))
		}
	}
	return explanation
}

func (b *batch) explainLoc(l *location) string {
	if l == &b.heapLoc {
		return "{heap}"
	}
	if l.n == nil {
		// TODO(mdempsky): Omit entirely.
		return "{temp}"
	}
	if l.n.Op() == ir.ONAME {
		return fmt.Sprintf("%v", l.n)
	}
	return fmt.Sprintf("{storage for %v}", l.n)
}

// outlives reports whether values stored in l may survive beyond
// other's lifetime if stack allocated.
func (b *batch) outlives(l, other *location) bool {
	// The heap outlives everything.
	if l.hasAttr(attrEscapes) {
		return true
	}

	// Pseudo-locations that don't really exist.
	if l == &b.mutatorLoc || l == &b.calleeLoc {
		return false
	}

	// We don't know what callers do with returned values, so
	// pessimistically we need to assume they flow to the heap and
	// outlive everything too.
	if l.paramOut {
		// Exception: Closures can return locations allocated outside of
		// them without forcing them to the heap, if we can statically
		// identify all call sites. For example:
		//
		//	var u int  // okay to stack allocate
		//	fn := func() *int { return &u }()
		//	*fn() = 42
		if ir.ContainsClosure(other.curfn, l.curfn) && !l.curfn.ClosureResultsLost() {
			return false
		}

		return true
	}

	// If l and other are within the same function, then l
	// outlives other if it was declared outside other's loop
	// scope. For example:
	//
	//	var l *int
	//	for {
	//		l = new(int) // must heap allocate: outlives for loop
	//	}
	if l.curfn == other.curfn && l.loopDepth < other.loopDepth {
		return true
	}

	// If other is declared within a child closure of where l is
	// declared, then l outlives it. For example:
	//
	//	var l *int
	//	func() {
	//		l = new(int) // must heap allocate: outlives call frame (if not inlined)
	//	}()
	if ir.ContainsClosure(l.curfn, other.curfn) {
		return true
	}

	return false
}

// queue implements a queue of locations for use in WalkAll and WalkOne.
// It supports pushing to front & back, and popping from front.
// TODO(thepudds): does cmd/compile have a deque or similar somewhere?
type queue struct {
	locs  []*location
	head  int // index of front element
	tail  int // next back element
	elems int
}

func newQueue(capacity int) *queue {
	capacity = max(capacity, 2)
	capacity = 1 << bits.Len64(uint64(capacity-1)) // round up to a power of 2
	return &queue{locs: make([]*location, capacity)}
}

// pushFront adds an element to the front of the queue.
func (q *queue) pushFront(loc *location) {
	if q.elems == len(q.locs) {
		q.grow()
	}
	q.head = q.wrap(q.head - 1)
	q.locs[q.head] = loc
	q.elems++
}

// pushBack adds an element to the back of the queue.
func (q *queue) pushBack(loc *location) {
	if q.elems == len(q.locs) {
		q.grow()
	}
	q.locs[q.tail] = loc
	q.tail = q.wrap(q.tail + 1)
	q.elems++
}

// popFront removes the front of the queue.
func (q *queue) popFront() *location {
	if q.elems == 0 {
		return nil
	}
	loc := q.locs[q.head]
	q.head = q.wrap(q.head + 1)
	q.elems--
	return loc
}

// grow doubles the capacity.
func (q *queue) grow() {
	newLocs := make([]*location, len(q.locs)*2)
	for i := range q.elems {
		// Copy over our elements in order.
		newLocs[i] = q.locs[q.wrap(q.head+i)]
	}
	q.locs = newLocs
	q.head = 0
	q.tail = q.elems
}

func (q *queue) len() int       { return q.elems }
func (q *queue) wrap(i int) int { return i & (len(q.locs) - 1) }

```

// === FILE: references!/go/src/cmd/compile/internal/escape/stmt.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"fmt"
)

// stmt evaluates a single Go statement.
func (e *escape) stmt(n ir.Node) {
	if n == nil {
		return
	}

	lno := ir.SetPos(n)
	defer func() {
		base.Pos = lno
	}()

	if base.Flag.LowerM > 2 {
		fmt.Printf("%v:[%d] %v stmt: %v\n", base.FmtPos(base.Pos), e.loopDepth, e.curfn, n)
	}

	e.stmts(n.Init())

	switch n.Op() {
	default:
		base.Fatalf("unexpected stmt: %v", n)

	case ir.OFALL, ir.OINLMARK:
		// nop

	case ir.OBREAK, ir.OCONTINUE, ir.OGOTO:
		// TODO(mdempsky): Handle dead code?

	case ir.OBLOCK:
		n := n.(*ir.BlockStmt)
		e.stmts(n.List)

	case ir.ODCL:
		// Record loop depth at declaration.
		n := n.(*ir.Decl)
		if !ir.IsBlank(n.X) {
			e.dcl(n.X)
		}

	case ir.OLABEL:
		n := n.(*ir.LabelStmt)
		if n.Label.IsBlank() {
			break
		}
		switch e.labels[n.Label] {
		case nonlooping:
			if base.Flag.LowerM > 2 {
				fmt.Printf("%v:%v non-looping label\n", base.FmtPos(base.Pos), n)
			}
		case looping:
			if base.Flag.LowerM > 2 {
				fmt.Printf("%v: %v looping label\n", base.FmtPos(base.Pos), n)
			}
			e.loopDepth++
		default:
			base.Fatalf("label %v missing tag", n.Label)
		}
		delete(e.labels, n.Label)

	case ir.OIF:
		n := n.(*ir.IfStmt)
		e.discard(n.Cond)
		e.block(n.Body)
		e.block(n.Else)

	case ir.OCHECKNIL:
		n := n.(*ir.UnaryExpr)
		e.discard(n.X)

	case ir.OFOR:
		n := n.(*ir.ForStmt)
		base.Assert(!n.DistinctVars) // Should all be rewritten before escape analysis
		e.loopDepth++
		e.discard(n.Cond)
		e.stmt(n.Post)
		e.block(n.Body)
		e.loopDepth--

	case ir.ORANGE:
		// for Key, Value = range X { Body }
		n := n.(*ir.RangeStmt)
		base.Assert(!n.DistinctVars) // Should all be rewritten before escape analysis

		// X is evaluated outside the loop and persists until the loop
		// terminates.
		tmp := e.newLoc(nil, true)
		e.expr(tmp.asHole(), n.X)

		e.loopDepth++
		ks := e.addrs([]ir.Node{n.Key, n.Value})
		if n.X.Type().IsArray() {
			e.flow(ks[1].note(n, "range"), tmp)
		} else {
			e.flow(ks[1].deref(n, "range-deref"), tmp)
		}
		e.reassigned(ks, n)

		e.block(n.Body)
		e.loopDepth--

	case ir.OSWITCH:
		n := n.(*ir.SwitchStmt)

		if guard, ok := n.Tag.(*ir.TypeSwitchGuard); ok {
			var ks []hole
			if guard.Tag != nil {
				for _, cas := range n.Cases {
					cv := cas.Var
					k := e.dcl(cv) // type switch variables have no ODCL.
					if cv.Type().HasPointers() {
						ks = append(ks, k.dotType(cv.Type(), cas, "switch case"))
					}
				}
			}
			e.expr(e.teeHole(ks...), n.Tag.(*ir.TypeSwitchGuard).X)
		} else {
			e.discard(n.Tag)
		}

		for _, cas := range n.Cases {
			e.discards(cas.List)
			e.block(cas.Body)
		}

	case ir.OSELECT:
		n := n.(*ir.SelectStmt)
		for _, cas := range n.Cases {
			e.stmt(cas.Comm)
			e.block(cas.Body)
		}
	case ir.ORECV:
		// TODO(mdempsky): Consider e.discard(n.Left).
		n := n.(*ir.UnaryExpr)
		e.exprSkipInit(e.discardHole(), n) // already visited n.Ninit
	case ir.OSEND:
		n := n.(*ir.SendStmt)
		e.discard(n.Chan)
		e.assignHeap(n.Value, "send", n)

	case ir.OAS:
		n := n.(*ir.AssignStmt)
		e.assignList([]ir.Node{n.X}, []ir.Node{n.Y}, "assign", n)
	case ir.OASOP:
		n := n.(*ir.AssignOpStmt)
		// TODO(mdempsky): Worry about OLSH/ORSH?
		e.assignList([]ir.Node{n.X}, []ir.Node{n.Y}, "assign", n)
	case ir.OAS2:
		n := n.(*ir.AssignListStmt)
		e.assignList(n.Lhs, n.Rhs, "assign-pair", n)

	case ir.OAS2DOTTYPE: // v, ok = x.(type)
		n := n.(*ir.AssignListStmt)
		e.assignList(n.Lhs, n.Rhs, "assign-pair-dot-type", n)
	case ir.OAS2MAPR: // v, ok = m[k]
		n := n.(*ir.AssignListStmt)
		e.assignList(n.Lhs, n.Rhs, "assign-pair-mapr", n)
	case ir.OAS2RECV, ir.OSELRECV2: // v, ok = <-ch
		n := n.(*ir.AssignListStmt)
		e.assignList(n.Lhs, n.Rhs, "assign-pair-receive", n)

	case ir.OAS2FUNC:
		n := n.(*ir.AssignListStmt)
		e.stmts(n.Rhs[0].Init())
		ks := e.addrs(n.Lhs)
		e.call(ks, n.Rhs[0])
		e.reassigned(ks, n)
	case ir.ORETURN:
		n := n.(*ir.ReturnStmt)
		results := e.curfn.Type().Results()
		dsts := make([]ir.Node, len(results))
		for i, res := range results {
			dsts[i] = res.Nname.(*ir.Name)
		}
		e.assignList(dsts, n.Results, "return", n)
	case ir.OCALLFUNC, ir.OCALLMETH, ir.OCALLINTER, ir.OINLCALL, ir.OCLEAR, ir.OCLOSE, ir.OCOPY, ir.ODELETE, ir.OPANIC, ir.OPRINT, ir.OPRINTLN, ir.ORECOVER:
		e.call(nil, n)
	case ir.OGO, ir.ODEFER:
		n := n.(*ir.GoDeferStmt)
		e.goDeferStmt(n)

	case ir.OTAILCALL:
		n := n.(*ir.TailCallStmt)
		e.call(nil, n.Call)
	}
}

func (e *escape) stmts(l ir.Nodes) {
	for _, n := range l {
		e.stmt(n)
	}
}

// block is like stmts, but preserves loopDepth.
func (e *escape) block(l ir.Nodes) {
	old := e.loopDepth
	e.stmts(l)
	e.loopDepth = old
}

func (e *escape) dcl(n *ir.Name) hole {
	if n.Curfn != e.curfn || n.IsClosureVar() {
		base.Fatalf("bad declaration of %v", n)
	}
	loc := e.oldLoc(n)
	loc.loopDepth = e.loopDepth
	return loc.asHole()
}

```

// === FILE: references!/go/src/cmd/compile/internal/escape/utils.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package escape

import (
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
)

func isSliceSelfAssign(dst, src ir.Node) bool {
	// Detect the following special case.
	//
	//	func (b *Buffer) Foo() {
	//		n, m := ...
	//		b.buf = b.buf[n:m]
	//	}
	//
	// This assignment is a no-op for escape analysis,
	// it does not store any new pointers into b that were not already there.
	// However, without this special case b will escape, because we assign to OIND/ODOTPTR.
	// Here we assume that the statement will not contain calls,
	// that is, that order will move any calls to init.
	// Otherwise base ONAME value could change between the moments
	// when we evaluate it for dst and for src.

	// dst is ONAME dereference.
	var dstX ir.Node
	switch dst.Op() {
	default:
		return false
	case ir.ODEREF:
		dst := dst.(*ir.StarExpr)
		dstX = dst.X
	case ir.ODOTPTR:
		dst := dst.(*ir.SelectorExpr)
		dstX = dst.X
	}
	if dstX.Op() != ir.ONAME {
		return false
	}
	// src is a slice operation.
	switch src.Op() {
	case ir.OSLICE, ir.OSLICE3, ir.OSLICESTR:
		// OK.
	case ir.OSLICEARR, ir.OSLICE3ARR:
		// Since arrays are embedded into containing object,
		// slice of non-pointer array will introduce a new pointer into b that was not already there
		// (pointer to b itself). After such assignment, if b contents escape,
		// b escapes as well. If we ignore such OSLICEARR, we will conclude
		// that b does not escape when b contents do.
		//
		// Pointer to an array is OK since it's not stored inside b directly.
		// For slicing an array (not pointer to array), there is an implicit OADDR.
		// We check that to determine non-pointer array slicing.
		src := src.(*ir.SliceExpr)
		if src.X.Op() == ir.OADDR {
			return false
		}
	default:
		return false
	}
	// slice is applied to ONAME dereference.
	var baseX ir.Node
	switch base := src.(*ir.SliceExpr).X; base.Op() {
	default:
		return false
	case ir.ODEREF:
		base := base.(*ir.StarExpr)
		baseX = base.X
	case ir.ODOTPTR:
		base := base.(*ir.SelectorExpr)
		baseX = base.X
	}
	if baseX.Op() != ir.ONAME {
		return false
	}
	// dst and src reference the same base ONAME.
	return dstX.(*ir.Name) == baseX.(*ir.Name)
}

// isSelfAssign reports whether assignment from src to dst can
// be ignored by the escape analysis as it's effectively a self-assignment.
func isSelfAssign(dst, src ir.Node) bool {
	if isSliceSelfAssign(dst, src) {
		return true
	}

	// Detect trivial assignments that assign back to the same object.
	//
	// It covers these cases:
	//	val.x = val.y
	//	val.x[i] = val.y[j]
	//	val.x1.x2 = val.x1.y2
	//	... etc
	//
	// These assignments do not change assigned object lifetime.

	if dst == nil || src == nil || dst.Op() != src.Op() {
		return false
	}

	// The expression prefix must be both "safe" and identical.
	switch dst.Op() {
	case ir.ODOT, ir.ODOTPTR:
		// Safe trailing accessors that are permitted to differ.
		dst := dst.(*ir.SelectorExpr)
		src := src.(*ir.SelectorExpr)
		return ir.SameSafeExpr(dst.X, src.X)
	case ir.OINDEX:
		dst := dst.(*ir.IndexExpr)
		src := src.(*ir.IndexExpr)
		if mayAffectMemory(dst.Index) || mayAffectMemory(src.Index) {
			return false
		}
		return ir.SameSafeExpr(dst.X, src.X)
	default:
		return false
	}
}

// mayAffectMemory reports whether evaluation of n may affect the program's
// memory state. If the expression can't affect memory state, then it can be
// safely ignored by the escape analysis.
func mayAffectMemory(n ir.Node) bool {
	// We may want to use a list of "memory safe" ops instead of generally
	// "side-effect free", which would include all calls and other ops that can
	// allocate or change global state. For now, it's safer to start with the latter.
	//
	// We're ignoring things like division by zero, index out of range,
	// and nil pointer dereference here.

	// TODO(rsc): It seems like it should be possible to replace this with
	// an ir.Any looking for any op that's not the ones in the case statement.
	// But that produces changes in the compiled output detected by buildall.
	switch n.Op() {
	case ir.ONAME, ir.OLITERAL, ir.ONIL:
		return false

	case ir.OADD, ir.OSUB, ir.OOR, ir.OXOR, ir.OMUL, ir.OLSH, ir.ORSH, ir.OAND, ir.OANDNOT, ir.ODIV, ir.OMOD:
		n := n.(*ir.BinaryExpr)
		return mayAffectMemory(n.X) || mayAffectMemory(n.Y)

	case ir.OINDEX:
		n := n.(*ir.IndexExpr)
		return mayAffectMemory(n.X) || mayAffectMemory(n.Index)

	case ir.OCONVNOP, ir.OCONV:
		n := n.(*ir.ConvExpr)
		return mayAffectMemory(n.X)

	case ir.OLEN, ir.OCAP, ir.ONOT, ir.OBITNOT, ir.OPLUS, ir.ONEG:
		n := n.(*ir.UnaryExpr)
		return mayAffectMemory(n.X)

	case ir.ODOT, ir.ODOTPTR:
		n := n.(*ir.SelectorExpr)
		return mayAffectMemory(n.X)

	case ir.ODEREF:
		n := n.(*ir.StarExpr)
		return mayAffectMemory(n.X)

	default:
		return true
	}
}

// HeapAllocReason returns the reason the given Node must be heap
// allocated, or the empty string if it doesn't.
func HeapAllocReason(n ir.Node) string {
	if n == nil || n.Type() == nil {
		return ""
	}

	// Parameters are always passed via the stack.
	if n.Op() == ir.ONAME {
		n := n.(*ir.Name)
		if n.Class == ir.PPARAM || n.Class == ir.PPARAMOUT {
			return ""
		}
	}

	if n.Type().Size() > ir.MaxStackVarSize {
		return "too large for stack"
	}
	if n.Type().Alignment() > int64(types.PtrSize) {
		return "too aligned for stack"
	}

	if (n.Op() == ir.ONEW || n.Op() == ir.OPTRLIT) && n.Type().Elem().Size() > ir.MaxImplicitStackVarSize {
		return "too large for stack"
	}
	if (n.Op() == ir.ONEW || n.Op() == ir.OPTRLIT) && n.Type().Elem().Alignment() > int64(types.PtrSize) {
		return "too aligned for stack"
	}

	if n.Op() == ir.OCLOSURE && typecheck.ClosureType(n.(*ir.ClosureExpr)).Size() > ir.MaxImplicitStackVarSize {
		return "too large for stack"
	}
	if n.Op() == ir.OMETHVALUE && typecheck.MethodValueType(n.(*ir.SelectorExpr)).Size() > ir.MaxImplicitStackVarSize {
		return "too large for stack"
	}

	if n.Op() == ir.OMAKESLICE {
		n := n.(*ir.MakeExpr)

		r := n.Cap
		if n.Cap == nil {
			r = n.Len
		}

		elem := n.Type().Elem()
		if elem.Size() == 0 {
			// TODO: stack allocate these? See #65685.
			return "zero-sized element"
		}
		if !ir.IsSmallIntConst(r) {
			// For non-constant sizes, we do a hybrid approach:
			//
			// if cap <= K {
			//     var backing [K]E
			//     s = backing[:len:cap]
			// } else {
			//     s = makeslice(E, len, cap)
			// }
			//
			// It costs a constant amount of stack space, but may
			// avoid a heap allocation.
			// Note we have to be careful that assigning s[i] = v
			// still escapes v, because we forbid heap->stack pointers.
			// Implementation is in ../walk/builtin.go:walkMakeSlice.
			return ""
		}
		if ir.Int64Val(r) > ir.MaxImplicitStackVarSize/elem.Size() {
			return "too large for stack"
		}
	}

	return ""
}

```

