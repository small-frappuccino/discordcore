# Domain Architecture: cmd/compile/internal/devirtualize

## Layout Topology
```text
cmd/compile/internal/devirtualize/
├── devirtualize.go
└── pgo.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/cmd/compile/internal/devirtualize/devirtualize.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package devirtualize implements two "devirtualization" optimization passes:
//
//   - "Static" devirtualization which replaces interface method calls with
//     direct concrete-type method calls where possible.
//   - "Profile-guided" devirtualization which replaces indirect calls with a
//     conditional direct call to the hottest concrete callee from a profile, as
//     well as a fallback using the original indirect call.
package devirtualize

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
)

const go126ImprovedConcreteTypeAnalysis = true

// StaticCall devirtualizes the given call if possible when the concrete callee
// is available statically.
func StaticCall(s *State, call *ir.CallExpr) {
	// For promoted methods (including value-receiver methods promoted
	// to pointer-receivers), the interface method wrapper may contain
	// expressions that can panic (e.g., ODEREF, ODOTPTR,
	// ODOTINTER). Devirtualization involves inlining these expressions
	// (and possible panics) to the call site. This normally isn't a
	// problem, but for go/defer statements it can move the panic from
	// when/where the call executes to the go/defer statement itself,
	// which is a visible change in semantics (e.g., #52072). To prevent
	// this, we skip devirtualizing calls within go/defer statements
	// altogether.
	if call.GoDefer {
		return
	}

	if call.Op() != ir.OCALLINTER {
		return
	}

	sel := call.Fun.(*ir.SelectorExpr)
	var typ *types.Type
	if go126ImprovedConcreteTypeAnalysis {
		typ = concreteType(s, sel.X)
		if typ == nil {
			return
		}

		// Don't create type-assertions that would be impossible at compile-time.
		// This can happen in such case: any(0).(interface {A()}).A(), this typechecks without
		// any errors, but will cause a runtime panic. We statically know that int(0) does not
		// implement that interface, thus we skip the devirtualization, as it is not possible
		// to make an assertion: any(0).(interface{A()}).(int) (int does not implement interface{A()}).
		if !typecheck.Implements(typ, sel.X.Type()) {
			return
		}
	} else {
		r := ir.StaticValue(sel.X)
		if r.Op() != ir.OCONVIFACE {
			return
		}
		recv := r.(*ir.ConvExpr)
		typ = recv.X.Type()
		if typ.IsInterface() {
			return
		}
	}

	// If typ is a shape type, then it was a type argument originally
	// and we'd need an indirect call through the dictionary anyway.
	// We're unable to devirtualize this call.
	if typ.IsShape() {
		return
	}

	// If typ *has* a shape type, then it's a shaped, instantiated
	// type like T[go.shape.int], and its methods (may) have an extra
	// dictionary parameter. We could devirtualize this call if we
	// could derive an appropriate dictionary argument.
	//
	// TODO(mdempsky): If typ has a promoted non-generic method,
	// then that method won't require a dictionary argument. We could
	// still devirtualize those calls.
	//
	// TODO(mdempsky): We have the *runtime.itab in recv.TypeWord. It
	// should be possible to compute the represented type's runtime
	// dictionary from this (e.g., by adding a pointer from T[int]'s
	// *runtime._type to .dict.T[int]; or by recognizing static
	// references to go:itab.T[int],iface and constructing a direct
	// reference to .dict.T[int]).
	if typ.HasShape() {
		if base.Flag.LowerM != 0 {
			base.WarnfAt(call.Pos(), "cannot devirtualize %v: shaped receiver %v", call, typ)
		}
		return
	}

	// Further, if sel.X's type has a shape type, then it's a shaped
	// interface type. In this case, the (non-dynamic) TypeAssertExpr
	// we construct below would attempt to create an itab
	// corresponding to this shaped interface type; but the actual
	// itab pointer in the interface value will correspond to the
	// original (non-shaped) interface type instead. These are
	// functionally equivalent, but they have distinct pointer
	// identities, which leads to the type assertion failing.
	//
	// TODO(mdempsky): We know the type assertion here is safe, so we
	// could instead set a flag so that walk skips the itab check. For
	// now, punting is easy and safe.
	if sel.X.Type().HasShape() {
		if base.Flag.LowerM != 0 {
			base.WarnfAt(call.Pos(), "cannot devirtualize %v: shaped interface %v", call, sel.X.Type())
		}
		return
	}

	dt := ir.NewTypeAssertExpr(sel.Pos(), sel.X, typ)

	if go126ImprovedConcreteTypeAnalysis {
		// Consider:
		//
		//	var v Iface
		//	v.A()
		//	v = &Impl{}
		//
		// Here in the devirtualizer, we determine the concrete type of v as being an *Impl,
		// but it can still be a nil interface, we have not detected that. The v.(*Impl)
		// type assertion that we make here would also have failed, but with a different
		// panic "pkg.Iface is nil, not *pkg.Impl", where previously we would get a nil panic.
		// We fix this, by introducing an additional nilcheck on the itab.
		// Calling a method on a nil interface (in most cases) is a bug in a program, so it is fine
		// to devirtualize and further (possibly) inline them, even though we would never reach
		// the called function.
		dt.UseNilPanic = true
		dt.SetPos(call.Pos())
	}

	x := typecheck.XDotMethod(sel.Pos(), dt, sel.Sel, true)
	switch x.Op() {
	case ir.ODOTMETH:
		if base.Flag.LowerM != 0 {
			base.WarnfAt(call.Pos(), "devirtualizing %v to %v", sel, typ)
		}
		call.SetOp(ir.OCALLMETH)
		call.Fun = x
	case ir.ODOTINTER:
		// Promoted method from embedded interface-typed field (#42279).
		if base.Flag.LowerM != 0 {
			base.WarnfAt(call.Pos(), "partially devirtualizing %v to %v", sel, typ)
		}
		call.SetOp(ir.OCALLINTER)
		call.Fun = x
	default:
		base.FatalfAt(call.Pos(), "failed to devirtualize %v (%v)", x, x.Op())
	}

	// Duplicated logic from typecheck for function call return
	// value types.
	//
	// Receiver parameter size may have changed; need to update
	// call.Type to get correct stack offsets for result
	// parameters.
	types.CheckSize(x.Type())
	switch ft := x.Type(); ft.NumResults() {
	case 0:
	case 1:
		call.SetType(ft.Result(0).Type)
	default:
		call.SetType(ft.ResultsTuple())
	}

	// Desugar OCALLMETH, if we created one (#57309).
	typecheck.FixMethodCall(call)
}

const concreteTypeDebug = false

// concreteType determines the concrete type of n, following OCONVIFACEs and type asserts.
// Returns nil when the concrete type could not be determined, or when there are multiple
// (different) types assigned to an interface.
func concreteType(s *State, n ir.Node) (typ *types.Type) {
	if concreteTypeDebug {
		base.Warn("concreteType(%v) - analyzing", n)
		defer func() {
			t := typ.String()
			if typ == nil {
				t = "<nil> (unknown static type)"
			}
			base.Warn("concreteType(%v) -> %v", n, t)
		}()
	}

	typ = concreteType1(s, n, make(map[*ir.Name]struct{}))
	if typ == &noType {
		return nil
	}
	if typ != nil && typ.IsInterface() {
		base.FatalfAt(n.Pos(), "typ.IsInterface() = true; want = false; typ = %v", typ)
	}
	return typ
}

// noType is a sentinel value returned by [concreteType1].
var noType types.Type

// concreteType1 analyzes the node n and returns its concrete type if it is statically known.
// Otherwise, it returns a nil Type, indicating that a concrete type was not determined.
// When n is known to be statically nil or a self-assignment is detected, it returns a sentinel [noType] type instead.
func concreteType1(s *State, n ir.Node, seen map[*ir.Name]struct{}) (outT *types.Type) {
	nn := n // for debug messages

	if concreteTypeDebug {
		defer func() {
			t := "&noType"
			if outT != &noType {
				t = outT.String()
			}
			if outT == nil {
				t = "<nil> (unknown static type)"
			}
			base.Warn("concreteType1(%v) -> %v", nn, t)
		}()
	}

	for {
		if concreteTypeDebug {
			base.Warn("concreteType1(%v): analyzing %v", nn, n)
		}

		if !n.Type().IsInterface() {
			return n.Type()
		}

		switch n1 := n.(type) {
		case *ir.ConvExpr:
			if n1.Op() == ir.OCONVNOP {
				if !n1.Type().IsInterface() || !types.Identical(n1.Type().Underlying(), n1.X.Type().Underlying()) {
					// As we check (directly before this switch) whether n is an interface, thus we should only reach
					// here for iface conversions where both operands are the same.
					base.FatalfAt(n1.Pos(), "not identical/interface types found n1.Type = %v; n1.X.Type = %v", n1.Type(), n1.X.Type())
				}
				n = n1.X
				continue
			}
			if n1.Op() == ir.OCONVIFACE {
				n = n1.X
				continue
			}
		case *ir.InlinedCallExpr:
			if n1.Op() == ir.OINLCALL {
				n = n1.SingleResult()
				continue
			}
		case *ir.ParenExpr:
			n = n1.X
			continue
		case *ir.TypeAssertExpr:
			n = n1.X
			continue
		}
		break
	}

	if n.Op() != ir.ONAME {
		return nil
	}

	name := n.(*ir.Name).Canonical()
	if name.Class != ir.PAUTO {
		return nil
	}

	if name.Op() != ir.ONAME {
		base.FatalfAt(name.Pos(), "name.Op = %v; want = ONAME", n.Op())
	}

	// name.Curfn must be set, as we checked name.Class != ir.PAUTO before.
	if name.Curfn == nil {
		base.FatalfAt(name.Pos(), "name.Curfn = nil; want not nil")
	}

	if name.Addrtaken() {
		return nil // conservatively assume it's reassigned with a different type indirectly
	}

	if _, ok := seen[name]; ok {
		return &noType // Already analyzed assignments to name, no need to do that twice.
	}
	seen[name] = struct{}{}

	if concreteTypeDebug {
		base.Warn("concreteType1(%v): analyzing assignments to %v", nn, name)
	}

	var typ *types.Type
	for _, v := range s.assignments(name) {
		var t *types.Type
		switch v := v.(type) {
		case *types.Type:
			t = v
		case ir.Node:
			t = concreteType1(s, v, seen)
			if t == &noType {
				continue
			}
		}
		if t == nil {
			return nil // unknown concrete type
		}

		// Methods are only declared on named types, and each named type
		// is represented by a unique [*types.Type], thus pointer comparison
		// is fine here.
		//
		// The only scenario where [types.IdenticalStrict] could help here is with
		// unnamed struct types that embed another type (e.g. foo = struct { Impl }{}).
		// However, such patterns are uncommon and not worth the additional complexity
		// in the devirtualizer.
		if typ != nil && typ != t {
			return nil // assigned with a different type
		}

		typ = t
	}

	if typ == nil {
		// Variable either declared with zero value, or only assigned with nil.
		return &noType
	}

	return typ
}

// assignment can be one of:
// - nil - assignment from an interface type.
// - *types.Type - assignment from a concrete type (non-interface).
// - ir.Node - assignment from an ir.Node.
//
// In most cases assignment should be an [ir.Node], but in cases where we
// do not follow the data-flow, we return either a concrete type (*types.Type) or a nil.
// For example in range over a slice, if the slice elem is of an interface type, then we return
// a nil, otherwise the elem's concrete type (We do so because we do not analyze assignment to the
// slice being ranged-over).
type assignment any

// State holds precomputed state for use in [StaticCall].
type State struct {
	// ifaceAssignments maps interface variables to all their assignments
	// defined inside functions stored in the analyzedFuncs set.
	// Note: it does not include direct assignments to nil.
	ifaceAssignments map[*ir.Name][]assignment

	// ifaceCallExprAssigns stores every [*ir.CallExpr], which has an interface
	// result, that is assigned to a variable.
	ifaceCallExprAssigns map[*ir.CallExpr][]ifaceAssignRef

	// analyzedFuncs is a set of Funcs that were analyzed for iface assignments.
	analyzedFuncs map[*ir.Func]struct{}
}

type ifaceAssignRef struct {
	name            *ir.Name // ifaceAssignments[name]
	assignmentIndex int      // ifaceAssignments[name][assignmentIndex]
	returnIndex     int      // (*ir.CallExpr).Result(returnIndex)
}

// InlinedCall updates the [State] to take into account a newly inlined call.
func (s *State) InlinedCall(fun *ir.Func, origCall *ir.CallExpr, inlinedCall *ir.InlinedCallExpr) {
	if _, ok := s.analyzedFuncs[fun]; !ok {
		// Full analyze has not been yet executed for the provided function, so we can skip it for now.
		// When no devirtualization happens in a function, it is unnecessary to analyze it.
		return
	}

	// Analyze assignments in the newly inlined function.
	s.analyze(inlinedCall.Init())
	s.analyze(inlinedCall.Body)

	refs, ok := s.ifaceCallExprAssigns[origCall]
	if !ok {
		return
	}
	delete(s.ifaceCallExprAssigns, origCall)

	// Update assignments to reference the new ReturnVars of the inlined call.
	for _, ref := range refs {
		vt := &s.ifaceAssignments[ref.name][ref.assignmentIndex]
		if *vt != nil {
			base.Fatalf("unexpected non-nil assignment")
		}
		if concreteTypeDebug {
			base.Warn(
				"InlinedCall(%v, %v): replacing interface node in (%v,%v) to %v (typ %v)",
				origCall, inlinedCall, ref.name, ref.assignmentIndex,
				inlinedCall.ReturnVars[ref.returnIndex],
				inlinedCall.ReturnVars[ref.returnIndex].Type(),
			)
		}

		// Update ifaceAssignments with an ir.Node from the inlined function’s ReturnVars.
		// This may enable future devirtualization of calls that reference ref.name.
		// We will get calls to [StaticCall] from the interleaved package,
		// to try devirtualize such calls afterwards.
		*vt = inlinedCall.ReturnVars[ref.returnIndex]
	}
}

// assignments returns all assignments to n.
func (s *State) assignments(n *ir.Name) []assignment {
	fun := n.Curfn
	if fun == nil {
		base.FatalfAt(n.Pos(), "n.Curfn = <nil>")
	}
	if n.Class != ir.PAUTO {
		base.FatalfAt(n.Pos(), "n.Class = %v; want = PAUTO", n.Class)
	}

	if !n.Type().IsInterface() {
		base.FatalfAt(n.Pos(), "name passed to assignments is not of an interface type: %v", n.Type())
	}

	// Analyze assignments in func, if not analyzed before.
	if _, ok := s.analyzedFuncs[fun]; !ok {
		if concreteTypeDebug {
			base.Warn("assignments(): analyzing assignments in %v func", fun)
		}
		if s.analyzedFuncs == nil {
			s.ifaceAssignments = make(map[*ir.Name][]assignment)
			s.ifaceCallExprAssigns = make(map[*ir.CallExpr][]ifaceAssignRef)
			s.analyzedFuncs = make(map[*ir.Func]struct{})
		}
		s.analyzedFuncs[fun] = struct{}{}
		s.analyze(fun.Init())
		s.analyze(fun.Body)
	}

	return s.ifaceAssignments[n]
}

// analyze analyzes every assignment to interface variables in nodes, updating [State].
func (s *State) analyze(nodes ir.Nodes) {
	assign := func(name ir.Node, assignment assignment) (*ir.Name, int) {
		if name == nil || name.Op() != ir.ONAME || ir.IsBlank(name) {
			return nil, -1
		}

		n, ok := ir.OuterValue(name).(*ir.Name)
		if !ok || n.Curfn == nil {
			return nil, -1
		}

		// Do not track variables that are not of interface types.
		// For devirtualization they are unnecessary, we will not even look them up.
		if !n.Type().IsInterface() {
			return nil, -1
		}

		n = n.Canonical()
		if n.Op() != ir.ONAME {
			base.FatalfAt(n.Pos(), "n.Op = %v; want = ONAME", n.Op())
		}
		if n.Class != ir.PAUTO {
			return nil, -1
		}

		switch a := assignment.(type) {
		case nil:
		case *types.Type:
			if a != nil && a.IsInterface() {
				assignment = nil // non-concrete type
			}
		case ir.Node:
			// nil assignment, we can safely ignore them, see [StaticCall].
			if ir.IsNil(a) {
				return nil, -1
			}
		default:
			base.Fatalf("unexpected type: %v", assignment)
		}

		if concreteTypeDebug {
			base.Warn("analyze(): assignment found %v = %v", name, assignment)
		}

		s.ifaceAssignments[n] = append(s.ifaceAssignments[n], assignment)
		return n, len(s.ifaceAssignments[n]) - 1
	}

	var do func(n ir.Node)
	do = func(n ir.Node) {
		switch n.Op() {
		case ir.OAS:
			n := n.(*ir.AssignStmt)
			if rhs := n.Y; rhs != nil {
				for {
					if r, ok := rhs.(*ir.ParenExpr); ok {
						rhs = r.X
						continue
					}
					break
				}
				if call, ok := rhs.(*ir.CallExpr); ok && call.Fun != nil {
					retTyp := call.Fun.Type().Results()[0].Type
					n, idx := assign(n.X, retTyp)
					if n != nil && retTyp.IsInterface() {
						// We have a call expression, that returns an interface, store it for later evaluation.
						// In case this func gets inlined later, we will update the assignment (added before)
						// with a reference to ReturnVars, see [State.InlinedCall], which might allow for future devirtualizing of n.X.
						s.ifaceCallExprAssigns[call] = append(s.ifaceCallExprAssigns[call], ifaceAssignRef{n, idx, 0})
					}
				} else {
					assign(n.X, rhs)
				}
			}
		case ir.OAS2:
			n := n.(*ir.AssignListStmt)
			for i, p := range n.Lhs {
				if n.Rhs[i] != nil {
					assign(p, n.Rhs[i])
				}
			}
		case ir.OAS2DOTTYPE:
			n := n.(*ir.AssignListStmt)
			if n.Rhs[0] == nil {
				base.FatalfAt(n.Pos(), "n.Rhs[0] == nil; n = %v", n)
			}
			assign(n.Lhs[0], n.Rhs[0])
			assign(n.Lhs[1], nil) // boolean does not have methods to devirtualize
		case ir.OAS2MAPR, ir.OAS2RECV, ir.OSELRECV2:
			n := n.(*ir.AssignListStmt)
			if n.Rhs[0] == nil {
				base.FatalfAt(n.Pos(), "n.Rhs[0] == nil; n = %v", n)
			}
			assign(n.Lhs[0], n.Rhs[0].Type())
			assign(n.Lhs[1], nil) // boolean does not have methods to devirtualize
		case ir.OAS2FUNC:
			n := n.(*ir.AssignListStmt)
			rhs := n.Rhs[0]
			for {
				if r, ok := rhs.(*ir.ParenExpr); ok {
					rhs = r.X
					continue
				}
				break
			}
			if call, ok := rhs.(*ir.CallExpr); ok {
				for i, p := range n.Lhs {
					retTyp := call.Fun.Type().Results()[i].Type
					n, idx := assign(p, retTyp)
					if n != nil && retTyp.IsInterface() {
						// We have a call expression, that returns an interface, store it for later evaluation.
						// In case this func gets inlined later, we will update the assignment (added before)
						// with a reference to ReturnVars, see [State.InlinedCall], which might allow for future devirtualizing of n.X.
						s.ifaceCallExprAssigns[call] = append(s.ifaceCallExprAssigns[call], ifaceAssignRef{n, idx, i})
					}
				}
			} else if call, ok := rhs.(*ir.InlinedCallExpr); ok {
				for i, p := range n.Lhs {
					assign(p, call.ReturnVars[i])
				}
			} else {
				base.FatalfAt(n.Pos(), "unexpected type %T in OAS2FUNC Rhs[0]", call)
			}
		case ir.ORANGE:
			n := n.(*ir.RangeStmt)
			xTyp := n.X.Type()

			// Range over an array pointer.
			if xTyp.IsPtr() && xTyp.Elem().IsArray() {
				xTyp = xTyp.Elem()
			}

			if xTyp.IsArray() || xTyp.IsSlice() {
				assign(n.Key, nil) // integer does not have methods to devirtualize
				assign(n.Value, xTyp.Elem())
			} else if xTyp.IsChan() {
				assign(n.Key, xTyp.Elem())
				base.AssertfAt(n.Value == nil, n.Pos(), "n.Value != nil in range over chan")
			} else if xTyp.IsMap() {
				assign(n.Key, xTyp.Key())
				assign(n.Value, xTyp.Elem())
			} else if xTyp.IsInteger() || xTyp.IsString() {
				// Range over int/string, results do not have methods, so nothing to devirtualize.
				assign(n.Key, nil)
				assign(n.Value, nil)
			} else {
				// We will not reach here in case of a range-over-func, as it is
				// rewritten to function calls in the noder package.
				base.FatalfAt(n.Pos(), "range over unexpected type %v", n.X.Type())
			}
		case ir.OSWITCH:
			n := n.(*ir.SwitchStmt)
			if guard, ok := n.Tag.(*ir.TypeSwitchGuard); ok {
				for _, v := range n.Cases {
					if v.Var == nil {
						base.Assert(guard.Tag == nil)
						continue
					}
					assign(v.Var, guard.X)
				}
			}
		case ir.OCLOSURE:
			n := n.(*ir.ClosureExpr)
			if _, ok := s.analyzedFuncs[n.Func]; !ok {
				s.analyzedFuncs[n.Func] = struct{}{}
				ir.Visit(n.Func, do)
			}
		}
	}
	ir.VisitList(nodes, do)
}

```

// === FILE: references!/go/src/cmd/compile/internal/devirtualize/pgo.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devirtualize

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/inline"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/logopt"
	"cmd/compile/internal/pgoir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/src"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// CallStat summarizes a single call site.
//
// This is used only for debug logging.
type CallStat struct {
	Pkg string // base.Ctxt.Pkgpath
	Pos string // file:line:col of call.

	Caller string // Linker symbol name of calling function.

	// Direct or indirect call.
	Direct bool

	// For indirect calls, interface call or other indirect function call.
	Interface bool

	// Total edge weight from this call site.
	Weight int64

	// Hottest callee from this call site, regardless of type
	// compatibility.
	Hottest       string
	HottestWeight int64

	// Devirtualized callee if != "".
	//
	// Note that this may be different than Hottest because we apply
	// type-check restrictions, which helps distinguish multiple calls on
	// the same line.
	Devirtualized       string
	DevirtualizedWeight int64
}

// ProfileGuided performs call devirtualization of indirect calls based on
// profile information.
//
// Specifically, it performs conditional devirtualization of interface calls or
// function value calls for the hottest callee.
//
// That is, for interface calls it performs a transformation like:
//
//	type Iface interface {
//		Foo()
//	}
//
//	type Concrete struct{}
//
//	func (Concrete) Foo() {}
//
//	func foo(i Iface) {
//		i.Foo()
//	}
//
// to:
//
//	func foo(i Iface) {
//		if c, ok := i.(Concrete); ok {
//			c.Foo()
//		} else {
//			i.Foo()
//		}
//	}
//
// For function value calls it performs a transformation like:
//
//	func Concrete() {}
//
//	func foo(fn func()) {
//		fn()
//	}
//
// to:
//
//	func foo(fn func()) {
//		if internal/abi.FuncPCABIInternal(fn) == internal/abi.FuncPCABIInternal(Concrete) {
//			Concrete()
//		} else {
//			fn()
//		}
//	}
//
// The primary benefit of this transformation is enabling inlining of the
// direct call.
func ProfileGuided(fn *ir.Func, p *pgoir.Profile) {
	ir.CurFunc = fn

	name := ir.LinkFuncName(fn)

	var jsonW *json.Encoder
	if base.Debug.PGODebug >= 3 {
		jsonW = json.NewEncoder(os.Stdout)
	}

	var edit func(n ir.Node) ir.Node
	edit = func(n ir.Node) ir.Node {
		if n == nil {
			return n
		}

		ir.EditChildren(n, edit)

		call, ok := n.(*ir.CallExpr)
		if !ok {
			return n
		}

		var stat *CallStat
		if base.Debug.PGODebug >= 3 {
			// Statistics about every single call. Handy for external data analysis.
			//
			// TODO(prattmic): Log via logopt?
			stat = constructCallStat(p, fn, name, call)
			if stat != nil {
				defer func() {
					jsonW.Encode(&stat)
				}()
			}
		}

		op := call.Op()
		if op != ir.OCALLFUNC && op != ir.OCALLINTER {
			return n
		}

		if base.Debug.PGODebug >= 2 {
			fmt.Printf("%v: PGO devirtualize considering call %v\n", ir.Line(call), call)
		}

		if call.GoDefer {
			if base.Debug.PGODebug >= 2 {
				fmt.Printf("%v: can't PGO devirtualize go/defer call %v\n", ir.Line(call), call)
			}
			return n
		}

		var newNode ir.Node
		var callee *ir.Func
		var weight int64
		switch op {
		case ir.OCALLFUNC:
			newNode, callee, weight = maybeDevirtualizeFunctionCall(p, fn, call)
		case ir.OCALLINTER:
			newNode, callee, weight = maybeDevirtualizeInterfaceCall(p, fn, call)
		default:
			panic("unreachable")
		}

		if newNode == nil {
			return n
		}

		if stat != nil {
			stat.Devirtualized = ir.LinkFuncName(callee)
			stat.DevirtualizedWeight = weight
		}

		return newNode
	}

	ir.EditChildren(fn, edit)
}

// Devirtualize interface call if possible and eligible. Returns the new
// ir.Node if call was devirtualized, and if so also the callee and weight of
// the devirtualized edge.
func maybeDevirtualizeInterfaceCall(p *pgoir.Profile, fn *ir.Func, call *ir.CallExpr) (ir.Node, *ir.Func, int64) {
	if base.Debug.PGODevirtualize < 1 {
		return nil, nil, 0
	}

	// Bail if we do not have a hot callee.
	callee, weight := findHotConcreteInterfaceCallee(p, fn, call)
	if callee == nil {
		return nil, nil, 0
	}
	// Bail if we do not have a Type node for the hot callee.
	ctyp := methodRecvType(callee)
	if ctyp == nil {
		return nil, nil, 0
	}
	// Bail if we know for sure it won't inline.
	if !shouldPGODevirt(callee) {
		return nil, nil, 0
	}
	// Bail if de-selected by PGO Hash.
	if !base.PGOHash.MatchPosWithInfo(call.Pos(), "devirt", nil) {
		return nil, nil, 0
	}

	return rewriteInterfaceCall(call, fn, callee, ctyp), callee, weight
}

// Devirtualize an indirect function call if possible and eligible. Returns the new
// ir.Node if call was devirtualized, and if so also the callee and weight of
// the devirtualized edge.
func maybeDevirtualizeFunctionCall(p *pgoir.Profile, fn *ir.Func, call *ir.CallExpr) (ir.Node, *ir.Func, int64) {
	if base.Debug.PGODevirtualize < 2 {
		return nil, nil, 0
	}

	// Bail if this is a direct call; no devirtualization necessary.
	callee := pgoir.DirectCallee(call.Fun)
	if callee != nil {
		return nil, nil, 0
	}

	// Bail if we do not have a hot callee.
	callee, weight := findHotConcreteFunctionCallee(p, fn, call)
	if callee == nil {
		return nil, nil, 0
	}

	// TODO(go.dev/issue/61577): Closures need the closure context passed
	// via the context register. That requires extra plumbing that we
	// haven't done yet.
	if callee.OClosure != nil {
		if base.Debug.PGODebug >= 3 {
			fmt.Printf("callee %s is a closure, skipping\n", ir.FuncName(callee))
		}
		return nil, nil, 0
	}
	// runtime.memhash_varlen does not look like a closure, but it uses
	// internal/runtime/sys.GetClosurePtr to access data encoded by
	// callers, which are generated by
	// cmd/compile/internal/reflectdata.genhash.
	if callee.Sym().Pkg.Path == "runtime" && callee.Sym().Name == "memhash_varlen" {
		if base.Debug.PGODebug >= 3 {
			fmt.Printf("callee %s is a closure (runtime.memhash_varlen), skipping\n", ir.FuncName(callee))
		}
		return nil, nil, 0
	}
	// TODO(prattmic): We don't properly handle methods as callees in two
	// different dimensions:
	//
	// 1. Method expressions. e.g.,
	//
	//      var fn func(*os.File, []byte) (int, error) = (*os.File).Read
	//
	// In this case, typ will report *os.File as the receiver while
	// ctyp reports it as the first argument. types.Identical ignores
	// receiver parameters, so it treats these as different, even though
	// they are still call compatible.
	//
	// 2. Method values. e.g.,
	//
	//      var f *os.File
	//      var fn func([]byte) (int, error) = f.Read
	//
	// types.Identical will treat these as compatible (since receiver
	// parameters are ignored). However, in this case, we do not call
	// (*os.File).Read directly. Instead, f is stored in closure context
	// and we call the wrapper (*os.File).Read-fm. However, runtime/pprof
	// hides wrappers from profiles, making it appear that there is a call
	// directly to the method. We could recognize this pattern return the
	// wrapper rather than the method.
	//
	// N.B. perf profiles will report wrapper symbols directly, so
	// ideally we should support direct wrapper references as well.
	if callee.Type().Recv() != nil {
		if base.Debug.PGODebug >= 3 {
			fmt.Printf("callee %s is a method, skipping\n", ir.FuncName(callee))
		}
		return nil, nil, 0
	}

	// Bail if we know for sure it won't inline.
	if !shouldPGODevirt(callee) {
		return nil, nil, 0
	}
	// Bail if de-selected by PGO Hash.
	if !base.PGOHash.MatchPosWithInfo(call.Pos(), "devirt", nil) {
		return nil, nil, 0
	}

	return rewriteFunctionCall(call, fn, callee), callee, weight
}

// shouldPGODevirt checks if we should perform PGO devirtualization to the
// target function.
//
// PGO devirtualization is most valuable when the callee is inlined, so if it
// won't inline we can skip devirtualizing.
func shouldPGODevirt(fn *ir.Func) bool {
	var reason string
	if base.Flag.LowerM > 1 || logopt.Enabled() {
		defer func() {
			if reason != "" {
				if base.Flag.LowerM > 1 {
					fmt.Printf("%v: should not PGO devirtualize %v: %s\n", ir.Line(fn), ir.FuncName(fn), reason)
				}
				if logopt.Enabled() {
					logopt.LogOpt(fn.Pos(), ": should not PGO devirtualize function", "pgoir-devirtualize", ir.FuncName(fn), reason)
				}
			}
		}()
	}

	reason = inline.InlineImpossible(fn)
	if reason != "" {
		return false
	}

	// TODO(prattmic): checking only InlineImpossible is very conservative,
	// primarily excluding only functions with pragmas. We probably want to
	// move in either direction. Either:
	//
	// 1. Don't even bother to check InlineImpossible, as it affects so few
	// functions.
	//
	// 2. Or consider the function body (notably cost) to better determine
	// if the function will actually inline.

	return true
}

// constructCallStat builds an initial CallStat describing this call, for
// logging. If the call is devirtualized, the devirtualization fields should be
// updated.
func constructCallStat(p *pgoir.Profile, fn *ir.Func, name string, call *ir.CallExpr) *CallStat {
	switch call.Op() {
	case ir.OCALLFUNC, ir.OCALLINTER, ir.OCALLMETH:
	default:
		// We don't care about logging builtin functions.
		return nil
	}

	stat := CallStat{
		Pkg:    base.Ctxt.Pkgpath,
		Pos:    ir.Line(call),
		Caller: name,
	}

	offset := pgoir.NodeLineOffset(call, fn)

	hotter := func(e *pgoir.IREdge) bool {
		if stat.Hottest == "" {
			return true
		}
		if e.Weight != stat.HottestWeight {
			return e.Weight > stat.HottestWeight
		}
		// If weight is the same, arbitrarily sort lexicographally, as
		// findHotConcreteCallee does.
		return e.Dst.Name() < stat.Hottest
	}

	callerNode := p.WeightedCG.IRNodes[name]
	if callerNode == nil {
		return nil
	}

	// Sum of all edges from this callsite, regardless of callee.
	// For direct calls, this should be the same as the single edge
	// weight (except for multiple calls on one line, which we
	// can't distinguish).
	for _, edge := range callerNode.OutEdges {
		if edge.CallSiteOffset != offset {
			continue
		}
		stat.Weight += edge.Weight
		if hotter(edge) {
			stat.HottestWeight = edge.Weight
			stat.Hottest = edge.Dst.Name()
		}
	}

	switch call.Op() {
	case ir.OCALLFUNC:
		stat.Interface = false

		callee := pgoir.DirectCallee(call.Fun)
		if callee != nil {
			stat.Direct = true
			if stat.Hottest == "" {
				stat.Hottest = ir.LinkFuncName(callee)
			}
		} else {
			stat.Direct = false
		}
	case ir.OCALLINTER:
		stat.Direct = false
		stat.Interface = true
	case ir.OCALLMETH:
		base.FatalfAt(call.Pos(), "OCALLMETH missed by typecheck")
	}

	return &stat
}

// copyInputs copies the inputs to a call: the receiver (for interface calls)
// or function value (for function value calls) and the arguments. These
// expressions are evaluated once and assigned to temporaries.
//
// The assignment statement is added to init and the copied receiver/fn
// expression and copied arguments expressions are returned.
func copyInputs(curfn *ir.Func, pos src.XPos, recvOrFn ir.Node, args []ir.Node, init *ir.Nodes) (ir.Node, []ir.Node) {
	// Evaluate receiver/fn and argument expressions. The receiver/fn is
	// used twice but we don't want to cause side effects twice. The
	// arguments are used in two different calls and we can't trivially
	// copy them.
	//
	// recvOrFn must be first in the assignment list as its side effects
	// must be ordered before argument side effects.
	var lhs, rhs []ir.Node
	newRecvOrFn := typecheck.TempAt(pos, curfn, recvOrFn.Type())
	lhs = append(lhs, newRecvOrFn)
	rhs = append(rhs, recvOrFn)

	for _, arg := range args {
		argvar := typecheck.TempAt(pos, curfn, arg.Type())

		lhs = append(lhs, argvar)
		rhs = append(rhs, arg)
	}

	asList := ir.NewAssignListStmt(pos, ir.OAS2, lhs, rhs)
	init.Append(typecheck.Stmt(asList))

	return newRecvOrFn, lhs[1:]
}

// retTemps returns a slice of temporaries to be used for storing result values from call.
func retTemps(curfn *ir.Func, pos src.XPos, call *ir.CallExpr) []ir.Node {
	sig := call.Fun.Type()
	var retvars []ir.Node
	for _, ret := range sig.Results() {
		retvars = append(retvars, typecheck.TempAt(pos, curfn, ret.Type))
	}
	return retvars
}

// condCall returns an ir.InlinedCallExpr that performs a call to thenCall if
// cond is true and elseCall if cond is false. The return variables of the
// InlinedCallExpr evaluate to the return values from the call.
func condCall(curfn *ir.Func, pos src.XPos, cond ir.Node, thenCall, elseCall *ir.CallExpr, init ir.Nodes) *ir.InlinedCallExpr {
	// Doesn't matter whether we use thenCall or elseCall, they must have
	// the same return types.
	retvars := retTemps(curfn, pos, thenCall)

	var thenBlock, elseBlock ir.Nodes
	if len(retvars) == 0 {
		thenBlock.Append(thenCall)
		elseBlock.Append(elseCall)
	} else {
		// Copy slice so edits in one location don't affect another.
		thenRet := append([]ir.Node(nil), retvars...)
		thenAsList := ir.NewAssignListStmt(pos, ir.OAS2, thenRet, []ir.Node{thenCall})
		thenBlock.Append(typecheck.Stmt(thenAsList))

		elseRet := append([]ir.Node(nil), retvars...)
		elseAsList := ir.NewAssignListStmt(pos, ir.OAS2, elseRet, []ir.Node{elseCall})
		elseBlock.Append(typecheck.Stmt(elseAsList))
	}

	nif := ir.NewIfStmt(pos, cond, thenBlock, elseBlock)
	nif.SetInit(init)
	nif.Likely = true

	body := []ir.Node{typecheck.Stmt(nif)}

	// This isn't really an inlined call of course, but InlinedCallExpr
	// makes handling reassignment of return values easier.
	res := ir.NewInlinedCallExpr(pos, body, retvars)
	res.SetType(thenCall.Type())
	res.SetTypecheck(1)
	res.Reshape = thenCall.Reshape
	return res
}

// rewriteInterfaceCall devirtualizes the given interface call using a direct
// method call to concretetyp.
func rewriteInterfaceCall(call *ir.CallExpr, curfn, callee *ir.Func, concretetyp *types.Type) ir.Node {
	if base.Flag.LowerM != 0 {
		fmt.Printf("%v: PGO devirtualizing interface call %v to %v\n", ir.Line(call), call.Fun, callee)
	}

	// We generate an OINCALL of:
	//
	// var recv Iface
	//
	// var arg1 A1
	// var argN AN
	//
	// var ret1 R1
	// var retN RN
	//
	// recv, arg1, argN = recv expr, arg1 expr, argN expr
	//
	// t, ok := recv.(Concrete)
	// if ok {
	//   ret1, retN = t.Method(arg1, ... argN)
	// } else {
	//   ret1, retN = recv.Method(arg1, ... argN)
	// }
	//
	// OINCALL retvars: ret1, ... retN
	//
	// This isn't really an inlined call of course, but InlinedCallExpr
	// makes handling reassignment of return values easier.
	//
	// TODO(prattmic): This increases the size of the AST in the caller,
	// making it less like to inline. We may want to compensate for this
	// somehow.

	sel := call.Fun.(*ir.SelectorExpr)
	method := sel.Sel
	pos := call.Pos()
	init := ir.TakeInit(call)

	recv, args := copyInputs(curfn, pos, sel.X, call.Args.Take(), &init)

	// Copy slice so edits in one location don't affect another.
	argvars := append([]ir.Node(nil), args...)
	call.Args = argvars

	tmpnode := typecheck.TempAt(base.Pos, curfn, concretetyp)
	tmpok := typecheck.TempAt(base.Pos, curfn, types.Types[types.TBOOL])

	assert := ir.NewTypeAssertExpr(pos, recv, concretetyp)

	assertAsList := ir.NewAssignListStmt(pos, ir.OAS2, []ir.Node{tmpnode, tmpok}, []ir.Node{typecheck.Expr(assert)})
	init.Append(typecheck.Stmt(assertAsList))

	concreteCallee := typecheck.XDotMethod(pos, tmpnode, method, true)
	// Copy slice so edits in one location don't affect another.
	argvars = append([]ir.Node(nil), argvars...)
	concreteCall := typecheck.Call(pos, concreteCallee, argvars, call.IsDDD).(*ir.CallExpr)

	res := condCall(curfn, pos, tmpok, concreteCall, call, init)

	if base.Debug.PGODebug >= 3 {
		fmt.Printf("PGO devirtualizing interface call to %+v. After: %+v\n", concretetyp, res)
	}

	return res
}

// rewriteFunctionCall devirtualizes the given OCALLFUNC using a direct
// function call to callee.
func rewriteFunctionCall(call *ir.CallExpr, curfn, callee *ir.Func) ir.Node {
	if base.Flag.LowerM != 0 {
		fmt.Printf("%v: PGO devirtualizing function call %v to %v\n", ir.Line(call), call.Fun, callee)
	}

	// We generate an OINCALL of:
	//
	// var fn FuncType
	//
	// var arg1 A1
	// var argN AN
	//
	// var ret1 R1
	// var retN RN
	//
	// fn, arg1, argN = fn expr, arg1 expr, argN expr
	//
	// fnPC := internal/abi.FuncPCABIInternal(fn)
	// concretePC := internal/abi.FuncPCABIInternal(concrete)
	//
	// if fnPC == concretePC {
	//   ret1, retN = concrete(arg1, ... argN) // Same closure context passed (TODO)
	// } else {
	//   ret1, retN = fn(arg1, ... argN)
	// }
	//
	// OINCALL retvars: ret1, ... retN
	//
	// This isn't really an inlined call of course, but InlinedCallExpr
	// makes handling reassignment of return values easier.

	pos := call.Pos()
	init := ir.TakeInit(call)

	fn, args := copyInputs(curfn, pos, call.Fun, call.Args.Take(), &init)

	// Copy slice so edits in one location don't affect another.
	argvars := append([]ir.Node(nil), args...)
	call.Args = argvars

	// FuncPCABIInternal takes an interface{}, emulate that. This is needed
	// for to ensure we get the MAKEFACE we need for SSA.
	fnIface := typecheck.Expr(ir.NewConvExpr(pos, ir.OCONV, types.Types[types.TINTER], fn))
	calleeIface := typecheck.Expr(ir.NewConvExpr(pos, ir.OCONV, types.Types[types.TINTER], callee.Nname))

	fnPC := ir.FuncPC(pos, fnIface, obj.ABIInternal)
	concretePC := ir.FuncPC(pos, calleeIface, obj.ABIInternal)

	pcEq := typecheck.Expr(ir.NewBinaryExpr(base.Pos, ir.OEQ, fnPC, concretePC))

	// TODO(go.dev/issue/61577): Handle callees that a closures and need a
	// copy of the closure context from call. For now, we skip callees that
	// are closures in maybeDevirtualizeFunctionCall.
	if callee.OClosure != nil {
		base.Fatalf("Callee is a closure: %+v", callee)
	}

	// Copy slice so edits in one location don't affect another.
	argvars = append([]ir.Node(nil), argvars...)
	concreteCall := typecheck.Call(pos, callee.Nname, argvars, call.IsDDD).(*ir.CallExpr)

	res := condCall(curfn, pos, pcEq, concreteCall, call, init)

	if base.Debug.PGODebug >= 3 {
		fmt.Printf("PGO devirtualizing function call to %+v. After: %+v\n", ir.FuncName(callee), res)
	}

	return res
}

// methodRecvType returns the type containing method fn. Returns nil if fn
// is not a method.
func methodRecvType(fn *ir.Func) *types.Type {
	recv := fn.Nname.Type().Recv()
	if recv == nil {
		return nil
	}
	return recv.Type
}

// interfaceCallRecvTypeAndMethod returns the type and the method of the interface
// used in an interface call.
func interfaceCallRecvTypeAndMethod(call *ir.CallExpr) (*types.Type, *types.Sym) {
	if call.Op() != ir.OCALLINTER {
		base.Fatalf("Call isn't OCALLINTER: %+v", call)
	}

	sel, ok := call.Fun.(*ir.SelectorExpr)
	if !ok {
		base.Fatalf("OCALLINTER doesn't contain SelectorExpr: %+v", call)
	}

	return sel.X.Type(), sel.Sel
}

// findHotConcreteCallee returns the *ir.Func of the hottest callee of a call,
// if available, and its edge weight. extraFn can perform additional
// applicability checks on each candidate edge. If extraFn returns false,
// candidate will not be considered a valid callee candidate.
func findHotConcreteCallee(p *pgoir.Profile, caller *ir.Func, call *ir.CallExpr, extraFn func(callerName string, callOffset int, candidate *pgoir.IREdge) bool) (*ir.Func, int64) {
	callerName := ir.LinkFuncName(caller)
	callerNode := p.WeightedCG.IRNodes[callerName]
	callOffset := pgoir.NodeLineOffset(call, caller)

	if callerNode == nil {
		return nil, 0
	}

	var hottest *pgoir.IREdge

	// Returns true if e is hotter than hottest.
	//
	// Naively this is just e.Weight > hottest.Weight, but because OutEdges
	// has arbitrary iteration order, we need to apply additional sort
	// criteria when e.Weight == hottest.Weight to ensure we have stable
	// selection.
	hotter := func(e *pgoir.IREdge) bool {
		if hottest == nil {
			return true
		}
		if e.Weight != hottest.Weight {
			return e.Weight > hottest.Weight
		}

		// Now e.Weight == hottest.Weight, we must select on other
		// criteria.

		// If only one edge has IR, prefer that one.
		if (hottest.Dst.AST == nil) != (e.Dst.AST == nil) {
			if e.Dst.AST != nil {
				return true
			}
			return false
		}

		// Arbitrary, but the callee names will always differ. Select
		// the lexicographically first callee.
		return e.Dst.Name() < hottest.Dst.Name()
	}

	for _, e := range callerNode.OutEdges {
		if e.CallSiteOffset != callOffset {
			continue
		}

		if !hotter(e) {
			// TODO(prattmic): consider total caller weight? i.e.,
			// if the hottest callee is only 10% of the weight,
			// maybe don't devirtualize? Similarly, if this is call
			// is globally very cold, there is not much value in
			// devirtualizing.
			if base.Debug.PGODebug >= 2 {
				fmt.Printf("%v: edge %s:%d -> %s (weight %d): too cold (hottest %d)\n", ir.Line(call), callerName, callOffset, e.Dst.Name(), e.Weight, hottest.Weight)
			}
			continue
		}

		if e.Dst.AST == nil {
			// Destination isn't visible from this package
			// compilation.
			//
			// We must assume it implements the interface.
			//
			// We still record this as the hottest callee so far
			// because we only want to return the #1 hottest
			// callee. If we skip this then we'd return the #2
			// hottest callee.
			if base.Debug.PGODebug >= 2 {
				fmt.Printf("%v: edge %s:%d -> %s (weight %d) (missing IR): hottest so far\n", ir.Line(call), callerName, callOffset, e.Dst.Name(), e.Weight)
			}
			hottest = e
			continue
		}

		if extraFn != nil && !extraFn(callerName, callOffset, e) {
			continue
		}

		if base.Debug.PGODebug >= 2 {
			fmt.Printf("%v: edge %s:%d -> %s (weight %d): hottest so far\n", ir.Line(call), callerName, callOffset, e.Dst.Name(), e.Weight)
		}
		hottest = e
	}

	if hottest == nil || hottest.Weight == 0 {
		if base.Debug.PGODebug >= 2 {
			fmt.Printf("%v: call %s:%d: no hot callee\n", ir.Line(call), callerName, callOffset)
		}
		return nil, 0
	}

	if base.Debug.PGODebug >= 2 {
		fmt.Printf("%v: call %s:%d: hottest callee %s (weight %d)\n", ir.Line(call), callerName, callOffset, hottest.Dst.Name(), hottest.Weight)
	}
	return hottest.Dst.AST, hottest.Weight
}

// findHotConcreteInterfaceCallee returns the *ir.Func of the hottest callee of an
// interface call, if available, and its edge weight.
func findHotConcreteInterfaceCallee(p *pgoir.Profile, caller *ir.Func, call *ir.CallExpr) (*ir.Func, int64) {
	inter, method := interfaceCallRecvTypeAndMethod(call)

	return findHotConcreteCallee(p, caller, call, func(callerName string, callOffset int, e *pgoir.IREdge) bool {
		ctyp := methodRecvType(e.Dst.AST)
		if ctyp == nil {
			// Not a method.
			// TODO(prattmic): Support non-interface indirect calls.
			if base.Debug.PGODebug >= 2 {
				fmt.Printf("%v: edge %s:%d -> %s (weight %d): callee not a method\n", ir.Line(call), callerName, callOffset, e.Dst.Name(), e.Weight)
			}
			return false
		}

		// If ctyp doesn't implement inter it is most likely from a
		// different call on the same line
		if !typecheck.Implements(ctyp, inter) {
			// TODO(prattmic): this is overly strict. Consider if
			// ctyp is a partial implementation of an interface
			// that gets embedded in types that complete the
			// interface. It would still be OK to devirtualize a
			// call to this method.
			//
			// What we'd need to do is check that the function
			// pointer in the itab matches the method we want,
			// rather than doing a full type assertion.
			if base.Debug.PGODebug >= 2 {
				why := typecheck.ImplementsExplain(ctyp, inter)
				fmt.Printf("%v: edge %s:%d -> %s (weight %d): %v doesn't implement %v (%s)\n", ir.Line(call), callerName, callOffset, e.Dst.Name(), e.Weight, ctyp, inter, why)
			}
			return false
		}

		// If the method name is different it is most likely from a
		// different call on the same line
		if !strings.HasSuffix(e.Dst.Name(), "."+method.Name) {
			if base.Debug.PGODebug >= 2 {
				fmt.Printf("%v: edge %s:%d -> %s (weight %d): callee is a different method\n", ir.Line(call), callerName, callOffset, e.Dst.Name(), e.Weight)
			}
			return false
		}

		return true
	})
}

// findHotConcreteFunctionCallee returns the *ir.Func of the hottest callee of an
// indirect function call, if available, and its edge weight.
func findHotConcreteFunctionCallee(p *pgoir.Profile, caller *ir.Func, call *ir.CallExpr) (*ir.Func, int64) {
	typ := call.Fun.Type().Underlying()

	return findHotConcreteCallee(p, caller, call, func(callerName string, callOffset int, e *pgoir.IREdge) bool {
		ctyp := e.Dst.AST.Type().Underlying()

		// If ctyp doesn't match typ it is most likely from a different
		// call on the same line.
		//
		// Note that we are comparing underlying types, as different
		// defined types are OK. e.g., a call to a value of type
		// net/http.HandlerFunc can be devirtualized to a function with
		// the same underlying type.
		if !types.Identical(typ, ctyp) {
			if base.Debug.PGODebug >= 2 {
				fmt.Printf("%v: edge %s:%d -> %s (weight %d): %v doesn't match %v\n", ir.Line(call), callerName, callOffset, e.Dst.Name(), e.Weight, ctyp, typ)
			}
			return false
		}

		return true
	})
}

```

