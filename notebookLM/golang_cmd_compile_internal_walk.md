# Domain Architecture: cmd/compile/internal/walk

## Layout Topology
```text
cmd/compile/internal/walk/
├── assign.go
├── builtin.go
├── closure.go
├── compare.go
├── complit.go
├── convert.go
├── expr.go
├── order.go
├── range.go
├── select.go
├── stmt.go
├── switch.go
├── temp.go
└── walk.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/compile/internal/walk/assign.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"go/constant"
	"internal/abi"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// walkAssign walks an OAS (AssignExpr) or OASOP (AssignOpExpr) node.
func walkAssign(init *ir.Nodes, n ir.Node) ir.Node {
	init.Append(ir.TakeInit(n)...)

	var left, right ir.Node
	switch n.Op() {
	case ir.OAS:
		n := n.(*ir.AssignStmt)
		left, right = n.X, n.Y
	case ir.OASOP:
		n := n.(*ir.AssignOpStmt)
		left, right = n.X, n.Y
	}

	// Recognize m[k] = append(m[k], ...) so we can reuse
	// the mapassign call.
	var mapAppend *ir.CallExpr
	if left.Op() == ir.OINDEXMAP && right.Op() == ir.OAPPEND {
		left := left.(*ir.IndexExpr)
		mapAppend = right.(*ir.CallExpr)
		if !ir.SameSafeExpr(left, mapAppend.Args[0]) {
			base.Fatalf("not same expressions: %v != %v", left, mapAppend.Args[0])
		}
	}

	left = walkExpr(left, init)
	left = safeExpr(left, init)
	if mapAppend != nil {
		mapAppend.Args[0] = left
	}

	if n.Op() == ir.OASOP {
		// Rewrite x op= y into x = x op y.
		n = ir.NewAssignStmt(base.Pos, left, typecheck.Expr(ir.NewBinaryExpr(base.Pos, n.(*ir.AssignOpStmt).AsOp, left, right)))
	} else {
		n.(*ir.AssignStmt).X = left
	}
	as := n.(*ir.AssignStmt)

	if oaslit(as, init) {
		return ir.NewBlockStmt(as.Pos(), nil)
	}

	if as.Y == nil {
		// TODO(austin): Check all "implicit zeroing"
		return as
	}

	if !base.Flag.Cfg.Instrumenting && ir.IsZero(as.Y) {
		return as
	}

	switch as.Y.Op() {
	default:
		as.Y = walkExpr(as.Y, init)

	case ir.ORECV:
		// x = <-c; as.Left is x, as.Right.Left is c.
		// order.stmt made sure x is addressable.
		recv := as.Y.(*ir.UnaryExpr)
		recv.X = walkExpr(recv.X, init)

		n1 := typecheck.NodAddr(as.X)
		r := recv.X // the channel
		return mkcall1(chanfn("chanrecv1", 2, r.Type()), nil, init, r, n1)

	case ir.OAPPEND:
		// x = append(...)
		call := as.Y.(*ir.CallExpr)
		if call.Type().Elem().NotInHeap() {
			base.Errorf("%v can't be allocated in Go; it is incomplete (or unallocatable)", call.Type().Elem())
		}
		var r ir.Node
		switch {
		case isAppendOfMake(call):
			// x = append(y, make([]T, y)...)
			r = extendSlice(call, init)
		case call.IsDDD:
			r = appendSlice(call, init) // also works for append(slice, string).
		default:
			r = walkAppend(call, init, as)
		}
		as.Y = r
		if r.Op() == ir.OAPPEND {
			r := r.(*ir.CallExpr)
			// Left in place for back end.
			// Do not add a new write barrier.
			// Set up address of type for back end.
			r.Fun = reflectdata.AppendElemRType(base.Pos, r)
			return as
		}
		// Otherwise, lowered for race detector.
		// Treat as ordinary assignment.
	}

	if as.X != nil && as.Y != nil {
		return convas(as, init)
	}
	return as
}

// walkAssignDotType walks an OAS2DOTTYPE node.
func walkAssignDotType(n *ir.AssignListStmt, init *ir.Nodes) ir.Node {
	walkExprListSafe(n.Lhs, init)

	if r, ok := n.Rhs[0].(*ir.TypeAssertExpr); ok && r.Op() == ir.ODOTTYPE2 && !r.Type().IsInterface() {
		if shapeTypeAssertImpossible(r.X, r.Type()) {
			init.Append(typecheck.Stmt(ir.NewAssignStmt(base.Pos, ir.BlankNode, walkExpr(r.X, init))))
			init.Append(typecheck.Stmt(ir.NewAssignStmt(base.Pos, n.Lhs[0], ir.NewZero(base.Pos, r.Type()))))
			init.Append(typecheck.Stmt(ir.NewAssignStmt(base.Pos, n.Lhs[1], ir.NewBool(base.Pos, false))))
			return ir.NewBlockStmt(base.Pos, nil)
		}
	}

	n.Rhs[0] = walkExpr(n.Rhs[0], init)
	return n
}

// walkAssignFunc walks an OAS2FUNC node.
func walkAssignFunc(init *ir.Nodes, n *ir.AssignListStmt) ir.Node {
	init.Append(ir.TakeInit(n)...)

	r := n.Rhs[0]
	walkExprListSafe(n.Lhs, init)
	r = walkExpr(r, init)

	if ir.IsIntrinsicCall(r.(*ir.CallExpr)) {
		n.Rhs = []ir.Node{r}
		return n
	}
	init.Append(r)

	ll := ascompatet(n.Lhs, r.Type())
	return ir.NewBlockStmt(src.NoXPos, ll)
}

// walkAssignList walks an OAS2 node.
func walkAssignList(init *ir.Nodes, n *ir.AssignListStmt) ir.Node {
	init.Append(ir.TakeInit(n)...)
	return ir.NewBlockStmt(src.NoXPos, ascompatee(ir.OAS, n.Lhs, n.Rhs))
}

// walkAssignMapRead walks an OAS2MAPR node.
func walkAssignMapRead(init *ir.Nodes, n *ir.AssignListStmt) ir.Node {
	init.Append(ir.TakeInit(n)...)

	r := n.Rhs[0].(*ir.IndexExpr)
	walkExprListSafe(n.Lhs, init)

	r.X = walkExpr(r.X, init)
	r.Index = walkExpr(r.Index, init)
	t := r.X.Type()
	fast := mapfast(t)
	key := mapKeyArg(fast, r, r.Index, false)

	// from:
	//   a,b = m[i]
	// to:
	//   var,b = mapaccess2*(t, m, i)
	//   a = *var
	a := n.Lhs[0]

	var call *ir.CallExpr
	if w := t.Elem().Size(); w <= abi.ZeroValSize {
		fn := mapfn(mapaccess2[fast], t, false)
		call = mkcall1(fn, fn.Type().ResultsTuple(), init, reflectdata.IndexMapRType(base.Pos, r), r.X, key)
	} else {
		fn := mapfn("mapaccess2_fat", t, true)
		z := reflectdata.ZeroAddr(w)
		call = mkcall1(fn, fn.Type().ResultsTuple(), init, reflectdata.IndexMapRType(base.Pos, r), r.X, key, z)
	}

	// mapaccess2* returns a typed bool, but due to spec changes,
	// the boolean result of i.(T) is now untyped so we make it the
	// same type as the variable on the lhs.
	if ok := n.Lhs[1]; !ir.IsBlank(ok) && ok.Type().IsBoolean() {
		call.Type().Field(1).Type = ok.Type()
	}
	n.Rhs = []ir.Node{call}
	n.SetOp(ir.OAS2FUNC)

	// don't generate a = *var if a is _
	if ir.IsBlank(a) {
		return walkExpr(typecheck.Stmt(n), init)
	}

	var_ := typecheck.TempAt(base.Pos, ir.CurFunc, types.NewPtr(t.Elem()))
	var_.SetTypecheck(1)
	var_.MarkNonNil() // mapaccess always returns a non-nil pointer

	n.Lhs[0] = var_
	init.Append(walkExpr(n, init))

	as := ir.NewAssignStmt(base.Pos, a, ir.NewStarExpr(base.Pos, var_))
	return walkExpr(typecheck.Stmt(as), init)
}

// walkAssignRecv walks an OAS2RECV node.
func walkAssignRecv(init *ir.Nodes, n *ir.AssignListStmt) ir.Node {
	init.Append(ir.TakeInit(n)...)

	r := n.Rhs[0].(*ir.UnaryExpr) // recv
	walkExprListSafe(n.Lhs, init)
	r.X = walkExpr(r.X, init)
	var n1 ir.Node
	if ir.IsBlank(n.Lhs[0]) {
		n1 = typecheck.NodNil()
	} else {
		n1 = typecheck.NodAddr(n.Lhs[0])
	}
	fn := chanfn("chanrecv2", 2, r.X.Type())
	ok := n.Lhs[1]
	call := mkcall1(fn, types.Types[types.TBOOL], init, r.X, n1)
	return walkAssign(init, typecheck.Stmt(ir.NewAssignStmt(base.Pos, ok, call)))
}

// walkReturn walks an ORETURN node.
func walkReturn(n *ir.ReturnStmt) ir.Node {
	fn := ir.CurFunc

	fn.NumReturns++
	if len(n.Results) == 0 {
		return n
	}

	results := fn.Type().Results()
	dsts := make([]ir.Node, len(results))
	for i, v := range results {
		// TODO(mdempsky): typecheck should have already checked the result variables.
		dsts[i] = typecheck.AssignExpr(v.Nname.(*ir.Name))
	}

	n.Results = ascompatee(n.Op(), dsts, n.Results)
	return n
}

// check assign type list to
// an expression list. called in
//
//	expr-list = func()
func ascompatet(nl ir.Nodes, nr *types.Type) []ir.Node {
	if len(nl) != nr.NumFields() {
		base.Fatalf("ascompatet: assignment count mismatch: %d = %d", len(nl), nr.NumFields())
	}

	var nn ir.Nodes
	for i, l := range nl {
		if ir.IsBlank(l) {
			continue
		}
		r := nr.Field(i)

		// Order should have created autotemps of the appropriate type for
		// us to store results into.
		if tmp, ok := l.(*ir.Name); !ok || !tmp.AutoTemp() || !types.Identical(tmp.Type(), r.Type) {
			base.FatalfAt(l.Pos(), "assigning %v to %+v", r.Type, l)
		}

		res := ir.NewResultExpr(base.Pos, nil, types.BADWIDTH)
		res.Index = int64(i)
		res.SetType(r.Type)
		res.SetTypecheck(1)

		nn.Append(ir.NewAssignStmt(base.Pos, l, res))
	}
	return nn
}

// check assign expression list to
// an expression list. called in
//
//	expr-list = expr-list
func ascompatee(op ir.Op, nl, nr []ir.Node) []ir.Node {
	// cannot happen: should have been rejected during type checking
	if len(nl) != len(nr) {
		base.Fatalf("assignment operands mismatch: %+v / %+v", ir.Nodes(nl), ir.Nodes(nr))
	}

	var assigned ir.NameSet
	var memWrite, deferResultWrite bool

	// affected reports whether expression n could be affected by
	// the assignments applied so far.
	affected := func(n ir.Node) bool {
		if deferResultWrite {
			return true
		}
		return ir.Any(n, func(n ir.Node) bool {
			if n.Op() == ir.ONAME && assigned.Has(n.(*ir.Name)) {
				return true
			}
			if memWrite && readsMemory(n) {
				return true
			}
			return false
		})
	}

	// If a needed expression may be affected by an
	// earlier assignment, make an early copy of that
	// expression and use the copy instead.
	var early ir.Nodes
	save := func(np *ir.Node) {
		if n := *np; affected(n) {
			*np = copyExpr(n, n.Type(), &early)
		}
	}

	var late ir.Nodes
	for i, lorig := range nl {
		l, r := lorig, nr[i]

		// Do not generate 'x = x' during return. See issue 4014.
		if op == ir.ORETURN && ir.SameSafeExpr(l, r) {
			continue
		}

		// Save subexpressions needed on left side.
		// Drill through non-dereferences.
		for {
			// If an expression has init statements, they must be evaluated
			// before any of its saved sub-operands (#45706).
			// TODO(mdempsky): Disallow init statements on lvalues.
			init := ir.TakeInit(l)
			walkStmtList(init)
			early.Append(init...)

			switch ll := l.(type) {
			case *ir.IndexExpr:
				if ll.X.Type().IsArray() {
					save(&ll.Index)
					l = ll.X
					continue
				}
			case *ir.ParenExpr:
				l = ll.X
				continue
			case *ir.SelectorExpr:
				if ll.Op() == ir.ODOT {
					l = ll.X
					continue
				}
			}
			break
		}

		var name *ir.Name
		switch l.Op() {
		default:
			base.Fatalf("unexpected lvalue %v", l.Op())
		case ir.ONAME:
			name = l.(*ir.Name)
		case ir.OINDEX, ir.OINDEXMAP:
			l := l.(*ir.IndexExpr)
			save(&l.X)
			save(&l.Index)
		case ir.ODEREF:
			l := l.(*ir.StarExpr)
			save(&l.X)
		case ir.ODOTPTR:
			l := l.(*ir.SelectorExpr)
			save(&l.X)
		}

		// Save expression on right side.
		save(&r)

		appendWalkStmt(&late, convas(ir.NewAssignStmt(base.Pos, lorig, r), &late))

		// Check for reasons why we may need to compute later expressions
		// before this assignment happens.

		if name == nil {
			// Not a direct assignment to a declared variable.
			// Conservatively assume any memory access might alias.
			memWrite = true
			continue
		}

		if name.Class == ir.PPARAMOUT && ir.CurFunc.HasDefer() {
			// Assignments to a result parameter in a function with defers
			// becomes visible early if evaluation of any later expression
			// panics (#43835).
			deferResultWrite = true
			continue
		}

		if ir.IsBlank(name) {
			// We can ignore assignments to blank or anonymous result parameters.
			// These can't appear in expressions anyway.
			continue
		}

		if name.Addrtaken() || !name.OnStack() {
			// Global variable, heap escaped, or just addrtaken.
			// Conservatively assume any memory access might alias.
			memWrite = true
			continue
		}

		// Local, non-addrtaken variable.
		// Assignments can only alias with direct uses of this variable.
		assigned.Add(name)
	}

	early.Append(late.Take()...)
	return early
}

// readsMemory reports whether the evaluation n directly reads from
// memory that might be written to indirectly.
func readsMemory(n ir.Node) bool {
	switch n.Op() {
	case ir.ONAME:
		n := n.(*ir.Name)
		if n.Class == ir.PFUNC {
			return false
		}
		return n.Addrtaken() || !n.OnStack()

	case ir.OADD,
		ir.OAND,
		ir.OANDAND,
		ir.OANDNOT,
		ir.OBITNOT,
		ir.OCONV,
		ir.OCONVIFACE,
		ir.OCONVNOP,
		ir.ODIV,
		ir.ODOT,
		ir.ODOTTYPE,
		ir.OLITERAL,
		ir.OLSH,
		ir.OMOD,
		ir.OMUL,
		ir.ONEG,
		ir.ONIL,
		ir.OOR,
		ir.OOROR,
		ir.OPAREN,
		ir.OPLUS,
		ir.ORSH,
		ir.OSUB,
		ir.OXOR:
		return false
	}

	// Be conservative.
	return true
}

// expand append(l1, l2...) to
//
//	init {
//	  s := l1
//	  newLen := s.len + l2.len
//	  // Compare as uint so growslice can panic on overflow.
//	  if uint(newLen) <= uint(s.cap) {
//	    s = s[:newLen]
//	  } else {
//	    s = growslice(s.ptr, s.len, s.cap, l2.len, T)
//	  }
//	  memmove(&s[s.len-l2.len], &l2[0], l2.len*sizeof(T))
//	}
//	s
//
// l2 is allowed to be a string.
func appendSlice(n *ir.CallExpr, init *ir.Nodes) ir.Node {
	walkAppendArgs(n, init)

	l1 := n.Args[0]
	l2 := n.Args[1]
	l2 = cheapExpr(l2, init)
	n.Args[1] = l2

	var nodes ir.Nodes

	// var s []T
	s := typecheck.TempAt(base.Pos, ir.CurFunc, l1.Type())
	nodes.Append(ir.NewAssignStmt(base.Pos, s, l1)) // s = l1

	elemtype := s.Type().Elem()

	// Decompose slice.
	oldPtr := ir.NewUnaryExpr(base.Pos, ir.OSPTR, s)
	oldLen := ir.NewUnaryExpr(base.Pos, ir.OLEN, s)
	oldCap := ir.NewUnaryExpr(base.Pos, ir.OCAP, s)

	// Number of elements we are adding
	num := ir.NewUnaryExpr(base.Pos, ir.OLEN, l2)

	// newLen := oldLen + num
	newLen := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])
	nodes.Append(ir.NewAssignStmt(base.Pos, newLen, ir.NewBinaryExpr(base.Pos, ir.OADD, oldLen, num)))

	// if uint(newLen) <= uint(oldCap)
	nif := ir.NewIfStmt(base.Pos, nil, nil, nil)
	nuint := typecheck.Conv(newLen, types.Types[types.TUINT])
	scapuint := typecheck.Conv(oldCap, types.Types[types.TUINT])
	nif.Cond = ir.NewBinaryExpr(base.Pos, ir.OLE, nuint, scapuint)
	nif.Likely = true

	// then { s = s[:newLen] }
	slice := ir.NewSliceExpr(base.Pos, ir.OSLICE, s, nil, newLen, nil)
	slice.SetBounded(true)
	nif.Body = []ir.Node{ir.NewAssignStmt(base.Pos, s, slice)}

	// else { s = growslice(oldPtr, newLen, oldCap, num, T) }
	call := walkGrowslice(s, nif.PtrInit(), oldPtr, newLen, oldCap, num)
	nif.Else = []ir.Node{ir.NewAssignStmt(base.Pos, s, call)}

	nodes.Append(nif)

	// Index to start copying into s.
	//   idx = newLen - len(l2)
	// We use this expression instead of oldLen because it avoids
	// a spill/restore of oldLen.
	// Note: this doesn't work optimally currently because
	// the compiler optimizer undoes this arithmetic.
	idx := ir.NewBinaryExpr(base.Pos, ir.OSUB, newLen, ir.NewUnaryExpr(base.Pos, ir.OLEN, l2))

	var ncopy ir.Node
	if elemtype.HasPointers() {
		// copy(s[idx:], l2)
		slice := ir.NewSliceExpr(base.Pos, ir.OSLICE, s, idx, nil, nil)
		slice.SetType(s.Type())
		slice.SetBounded(true)

		ir.CurFunc.SetWBPos(n.Pos())

		// instantiate typedslicecopy(typ *type, dstPtr *any, dstLen int, srcPtr *any, srcLen int) int
		fn := typecheck.LookupRuntime("typedslicecopy", l1.Type().Elem(), l2.Type().Elem())
		ptr1, len1 := backingArrayPtrLen(cheapExpr(slice, &nodes))
		ptr2, len2 := backingArrayPtrLen(l2)
		ncopy = mkcall1(fn, types.Types[types.TINT], &nodes, reflectdata.AppendElemRType(base.Pos, n), ptr1, len1, ptr2, len2)
	} else if base.Flag.Cfg.Instrumenting && !base.Flag.CompilingRuntime {
		// rely on runtime to instrument:
		//  copy(s[idx:], l2)
		// l2 can be a slice or string.
		slice := ir.NewSliceExpr(base.Pos, ir.OSLICE, s, idx, nil, nil)
		slice.SetType(s.Type())
		slice.SetBounded(true)

		ptr1, len1 := backingArrayPtrLen(cheapExpr(slice, &nodes))
		ptr2, len2 := backingArrayPtrLen(l2)

		fn := typecheck.LookupRuntime("slicecopy", ptr1.Type().Elem(), ptr2.Type().Elem())
		ncopy = mkcall1(fn, types.Types[types.TINT], &nodes, ptr1, len1, ptr2, len2, ir.NewInt(base.Pos, elemtype.Size()))
	} else {
		// memmove(&s[idx], &l2[0], len(l2)*sizeof(T))
		ix := ir.NewIndexExpr(base.Pos, s, idx)
		ix.SetBounded(true)
		addr := typecheck.NodAddr(ix)

		sptr := ir.NewUnaryExpr(base.Pos, ir.OSPTR, l2)

		nwid := cheapExpr(typecheck.Conv(ir.NewUnaryExpr(base.Pos, ir.OLEN, l2), types.Types[types.TUINTPTR]), &nodes)
		nwid = ir.NewBinaryExpr(base.Pos, ir.OMUL, nwid, ir.NewInt(base.Pos, elemtype.Size()))

		// instantiate func memmove(to *any, frm *any, length uintptr)
		fn := typecheck.LookupRuntime("memmove", elemtype, elemtype)
		ncopy = mkcall1(fn, nil, &nodes, addr, sptr, nwid)
	}
	ln := append(nodes, ncopy)

	typecheck.Stmts(ln)
	walkStmtList(ln)
	init.Append(ln...)
	return s
}

// isAppendOfMake reports whether n is of the form append(x, make([]T, y)...).
// isAppendOfMake assumes n has already been typechecked.
func isAppendOfMake(n ir.Node) bool {
	if base.Flag.N != 0 || base.Flag.Cfg.Instrumenting {
		return false
	}

	if n.Typecheck() == 0 {
		base.Fatalf("missing typecheck: %+v", n)
	}

	if n.Op() != ir.OAPPEND {
		return false
	}
	call := n.(*ir.CallExpr)
	if !call.IsDDD || len(call.Args) != 2 || call.Args[1].Op() != ir.OMAKESLICE {
		return false
	}

	mk := call.Args[1].(*ir.MakeExpr)
	if mk.Cap != nil {
		return false
	}

	// y must be either an integer constant or the largest possible positive value
	// of variable y needs to fit into a uint.

	// typecheck made sure that constant arguments to make are not negative and fit into an int.

	// The care of overflow of the len argument to make will be handled by an explicit check of int(len) < 0 during runtime.
	y := mk.Len
	if !ir.IsConst(y, constant.Int) && y.Type().Size() > types.Types[types.TUINT].Size() {
		return false
	}

	return true
}

// extendSlice rewrites append(l1, make([]T, l2)...) to
//
//	init {
//	  if l2 >= 0 { // Empty if block here for more meaningful node.SetLikely(true)
//	  } else {
//	    panicmakeslicelen()
//	  }
//	  s := l1
//	  if l2 != 0 {
//	    n := len(s) + l2
//	    // Compare n and s as uint so growslice can panic on overflow of len(s) + l2.
//	    // cap is a positive int and n can become negative when len(s) + l2
//	    // overflows int. Interpreting n when negative as uint makes it larger
//	    // than cap(s). growslice will check the int n arg and panic if n is
//	    // negative. This prevents the overflow from being undetected.
//	    if uint(n) <= uint(cap(s)) {
//	      s = s[:n]
//	    } else {
//	      s = growslice(T, s.ptr, n, s.cap, l2, T)
//	    }
//	    // clear the new portion of the underlying array.
//	    hp := &s[len(s)-l2]
//	    hn := l2 * sizeof(T)
//	    memclr(hp, hn)
//	  }
//	}
//	s
//
//	if T has pointers, the final memclr can go inside the "then" branch, as
//	growslice will have done the clearing for us.

func extendSlice(n *ir.CallExpr, init *ir.Nodes) ir.Node {
	// isAppendOfMake made sure all possible positive values of l2 fit into a uint.
	// The case of l2 overflow when converting from e.g. uint to int is handled by an explicit
	// check of l2 < 0 at runtime which is generated below.
	l2 := typecheck.Conv(n.Args[1].(*ir.MakeExpr).Len, types.Types[types.TINT])
	l2 = typecheck.Expr(l2)
	n.Args[1] = l2 // walkAppendArgs expects l2 in n.List.Second().

	walkAppendArgs(n, init)

	l1 := n.Args[0]
	l2 = n.Args[1] // re-read l2, as it may have been updated by walkAppendArgs

	var nodes []ir.Node

	// if l2 >= 0 (likely happens), do nothing
	nifneg := ir.NewIfStmt(base.Pos, ir.NewBinaryExpr(base.Pos, ir.OGE, l2, ir.NewInt(base.Pos, 0)), nil, nil)
	nifneg.Likely = true

	// else panicmakeslicelen()
	nifneg.Else = []ir.Node{mkcall("panicmakeslicelen", nil, init)}
	nodes = append(nodes, nifneg)

	// s := l1
	s := typecheck.TempAt(base.Pos, ir.CurFunc, l1.Type())
	nodes = append(nodes, ir.NewAssignStmt(base.Pos, s, l1))

	// if l2 != 0 {
	// Avoid work if we're not appending anything. But more importantly,
	// avoid allowing hp to be a past-the-end pointer when clearing. See issue 67255.
	nifnz := ir.NewIfStmt(base.Pos, ir.NewBinaryExpr(base.Pos, ir.ONE, l2, ir.NewInt(base.Pos, 0)), nil, nil)
	nifnz.Likely = true
	nodes = append(nodes, nifnz)

	elemtype := s.Type().Elem()

	// n := s.len + l2
	nn := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])
	nifnz.Body = append(nifnz.Body, ir.NewAssignStmt(base.Pos, nn, ir.NewBinaryExpr(base.Pos, ir.OADD, ir.NewUnaryExpr(base.Pos, ir.OLEN, s), l2)))

	// if uint(n) <= uint(s.cap)
	nuint := typecheck.Conv(nn, types.Types[types.TUINT])
	capuint := typecheck.Conv(ir.NewUnaryExpr(base.Pos, ir.OCAP, s), types.Types[types.TUINT])
	nif := ir.NewIfStmt(base.Pos, ir.NewBinaryExpr(base.Pos, ir.OLE, nuint, capuint), nil, nil)
	nif.Likely = true

	// then { s = s[:n] }
	nt := ir.NewSliceExpr(base.Pos, ir.OSLICE, s, nil, nn, nil)
	nt.SetBounded(true)
	nif.Body = []ir.Node{ir.NewAssignStmt(base.Pos, s, nt)}

	// else { s = growslice(s.ptr, n, s.cap, l2, T) }
	nif.Else = []ir.Node{
		ir.NewAssignStmt(base.Pos, s, walkGrowslice(s, nif.PtrInit(),
			ir.NewUnaryExpr(base.Pos, ir.OSPTR, s),
			nn,
			ir.NewUnaryExpr(base.Pos, ir.OCAP, s),
			l2)),
	}

	nifnz.Body = append(nifnz.Body, nif)

	// hp := &s[s.len - l2]
	// TODO: &s[s.len] - hn?
	ix := ir.NewIndexExpr(base.Pos, s, ir.NewBinaryExpr(base.Pos, ir.OSUB, ir.NewUnaryExpr(base.Pos, ir.OLEN, s), l2))
	ix.SetBounded(true)
	hp := typecheck.ConvNop(typecheck.NodAddr(ix), types.Types[types.TUNSAFEPTR])

	// hn := l2 * sizeof(elem(s))
	hn := typecheck.Conv(ir.NewBinaryExpr(base.Pos, ir.OMUL, l2, ir.NewInt(base.Pos, elemtype.Size())), types.Types[types.TUINTPTR])

	clrname := "memclrNoHeapPointers"
	hasPointers := elemtype.HasPointers()
	if hasPointers {
		clrname = "memclrHasPointers"
		ir.CurFunc.SetWBPos(n.Pos())
	}

	var clr ir.Nodes
	clrfn := mkcall(clrname, nil, &clr, hp, hn)
	clr.Append(clrfn)
	if hasPointers {
		// growslice will have cleared the new entries, so only
		// if growslice isn't called do we need to do the zeroing ourselves.
		nif.Body = append(nif.Body, clr...)
	} else {
		nifnz.Body = append(nifnz.Body, clr...)
	}

	typecheck.Stmts(nodes)
	walkStmtList(nodes)
	init.Append(nodes...)
	return s
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/builtin.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.walk/bui
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"fmt"
	"go/constant"
	"go/token"
	"internal/abi"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/escape"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
)

// Rewrite append(src, x, y, z) so that any side effects in
// x, y, z (including runtime panics) are evaluated in
// initialization statements before the append.
// For normal code generation, stop there and leave the
// rest to ssagen.
//
// For race detector, expand append(src, a [, b]* ) to
//
//	init {
//	  s := src
//	  const argc = len(args) - 1
//	  newLen := s.len + argc
//	  if uint(newLen) <= uint(s.cap) {
//	    s = s[:newLen]
//	  } else {
//	    s = growslice(s.ptr, newLen, s.cap, argc, elemType)
//	  }
//	  s[s.len - argc] = a
//	  s[s.len - argc + 1] = b
//	  ...
//	}
//	s
func walkAppend(n *ir.CallExpr, init *ir.Nodes, dst ir.Node) ir.Node {
	if !ir.SameSafeExpr(dst, n.Args[0]) {
		n.Args[0] = safeExpr(n.Args[0], init)
		n.Args[0] = walkExpr(n.Args[0], init)
	}
	walkExprListSafe(n.Args[1:], init)

	nsrc := n.Args[0]

	// walkExprListSafe will leave OINDEX (s[n]) alone if both s
	// and n are name or literal, but those may index the slice we're
	// modifying here. Fix explicitly.
	// Using cheapExpr also makes sure that the evaluation
	// of all arguments (and especially any panics) happen
	// before we begin to modify the slice in a visible way.
	ls := n.Args[1:]
	for i, n := range ls {
		n = cheapExpr(n, init)
		if !types.Identical(n.Type(), nsrc.Type().Elem()) {
			n = typecheck.AssignConv(n, nsrc.Type().Elem(), "append")
			n = walkExpr(n, init)
		}
		ls[i] = n
	}

	argc := len(n.Args) - 1
	if argc < 1 {
		return nsrc
	}

	// General case, with no function calls left as arguments.
	// Leave for ssagen, except that instrumentation requires the old form.
	if !base.Flag.Cfg.Instrumenting || base.Flag.CompilingRuntime {
		return n
	}

	var l []ir.Node

	// s = slice to append to
	s := typecheck.TempAt(base.Pos, ir.CurFunc, nsrc.Type())
	l = append(l, ir.NewAssignStmt(base.Pos, s, nsrc))

	// num = number of things to append
	num := ir.NewInt(base.Pos, int64(argc))

	// newLen := s.len + num
	newLen := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])
	l = append(l, ir.NewAssignStmt(base.Pos, newLen, ir.NewBinaryExpr(base.Pos, ir.OADD, ir.NewUnaryExpr(base.Pos, ir.OLEN, s), num)))

	// if uint(newLen) <= uint(s.cap)
	nif := ir.NewIfStmt(base.Pos, nil, nil, nil)
	nif.Cond = ir.NewBinaryExpr(base.Pos, ir.OLE, typecheck.Conv(newLen, types.Types[types.TUINT]), typecheck.Conv(ir.NewUnaryExpr(base.Pos, ir.OCAP, s), types.Types[types.TUINT]))
	nif.Likely = true

	// then { s = s[:n] }
	slice := ir.NewSliceExpr(base.Pos, ir.OSLICE, s, nil, newLen, nil)
	slice.SetBounded(true)
	nif.Body = []ir.Node{
		ir.NewAssignStmt(base.Pos, s, slice),
	}

	// else { s = growslice(s.ptr, n, s.cap, a, T) }
	nif.Else = []ir.Node{
		ir.NewAssignStmt(base.Pos, s, walkGrowslice(s, nif.PtrInit(),
			ir.NewUnaryExpr(base.Pos, ir.OSPTR, s),
			newLen,
			ir.NewUnaryExpr(base.Pos, ir.OCAP, s),
			num)),
	}

	l = append(l, nif)

	ls = n.Args[1:]
	for i, n := range ls {
		// s[s.len-argc+i] = arg
		ix := ir.NewIndexExpr(base.Pos, s, ir.NewBinaryExpr(base.Pos, ir.OSUB, newLen, ir.NewInt(base.Pos, int64(argc-i))))
		ix.SetBounded(true)
		l = append(l, ir.NewAssignStmt(base.Pos, ix, n))
	}

	typecheck.Stmts(l)
	walkStmtList(l)
	init.Append(l...)
	return s
}

// growslice(ptr *T, newLen, oldCap, num int, <type>) (ret []T)
func walkGrowslice(slice *ir.Name, init *ir.Nodes, oldPtr, newLen, oldCap, num ir.Node) *ir.CallExpr {
	elemtype := slice.Type().Elem()
	fn := typecheck.LookupRuntime("growslice", elemtype, elemtype)
	elemtypeptr := reflectdata.TypePtrAt(base.Pos, elemtype)
	return mkcall1(fn, slice.Type(), init, oldPtr, newLen, oldCap, num, elemtypeptr)
}

// walkClear walks an OCLEAR node.
func walkClear(n *ir.UnaryExpr, init *ir.Nodes) ir.Node {
	x := walkExpr(n.X, init)
	typ := n.X.Type()
	switch {
	case typ.IsSlice():
		if n := arrayClear(x.Pos(), x, nil); n != nil {
			return n
		}
		// If n == nil, we are clearing an array which takes zero memory, do nothing.
		return ir.NewBlockStmt(n.Pos(), nil)
	case typ.IsMap():
		return mapClear(x, reflectdata.TypePtrAt(x.Pos(), typ))
	}
	panic("unreachable")
}

// walkClose walks an OCLOSE node.
func walkClose(n *ir.UnaryExpr, init *ir.Nodes) ir.Node {
	return mkcall1(chanfn("closechan", 1, n.X.Type()), nil, init, n.X)
}

// Lower copy(a, b) to a memmove call or a runtime call.
//
//	init {
//	  n := len(a)
//	  if n > len(b) { n = len(b) }
//	  if a.ptr != b.ptr { memmove(a.ptr, b.ptr, n*sizeof(elem(a))) }
//	}
//	n;
//
// Also works if b is a string.
func walkCopy(n *ir.BinaryExpr, init *ir.Nodes, runtimecall bool) ir.Node {
	if n.X.Type().Elem().HasPointers() {
		ir.CurFunc.SetWBPos(n.Pos())
		fn := writebarrierfn("typedslicecopy", n.X.Type().Elem(), n.Y.Type().Elem())
		n.X = cheapExpr(n.X, init)
		ptrL, lenL := backingArrayPtrLen(n.X)
		n.Y = cheapExpr(n.Y, init)
		ptrR, lenR := backingArrayPtrLen(n.Y)
		return mkcall1(fn, n.Type(), init, reflectdata.CopyElemRType(base.Pos, n), ptrL, lenL, ptrR, lenR)
	}

	if runtimecall {
		// rely on runtime to instrument:
		//  copy(n.Left, n.Right)
		// n.Right can be a slice or string.

		n.X = cheapExpr(n.X, init)
		ptrL, lenL := backingArrayPtrLen(n.X)
		n.Y = cheapExpr(n.Y, init)
		ptrR, lenR := backingArrayPtrLen(n.Y)

		fn := typecheck.LookupRuntime("slicecopy", ptrL.Type().Elem(), ptrR.Type().Elem())

		return mkcall1(fn, n.Type(), init, ptrL, lenL, ptrR, lenR, ir.NewInt(base.Pos, n.X.Type().Elem().Size()))
	}

	n.X = walkExpr(n.X, init)
	n.Y = walkExpr(n.Y, init)
	nl := typecheck.TempAt(base.Pos, ir.CurFunc, n.X.Type())
	nr := typecheck.TempAt(base.Pos, ir.CurFunc, n.Y.Type())
	var l []ir.Node
	l = append(l, ir.NewAssignStmt(base.Pos, nl, n.X))
	l = append(l, ir.NewAssignStmt(base.Pos, nr, n.Y))

	nfrm := ir.NewUnaryExpr(base.Pos, ir.OSPTR, nr)
	nto := ir.NewUnaryExpr(base.Pos, ir.OSPTR, nl)

	nlen := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])

	// n = len(to)
	l = append(l, ir.NewAssignStmt(base.Pos, nlen, ir.NewUnaryExpr(base.Pos, ir.OLEN, nl)))

	// if n > len(frm) { n = len(frm) }
	nif := ir.NewIfStmt(base.Pos, nil, nil, nil)

	nif.Cond = ir.NewBinaryExpr(base.Pos, ir.OGT, nlen, ir.NewUnaryExpr(base.Pos, ir.OLEN, nr))
	nif.Body.Append(ir.NewAssignStmt(base.Pos, nlen, ir.NewUnaryExpr(base.Pos, ir.OLEN, nr)))
	l = append(l, nif)

	// if to.ptr != frm.ptr { memmove( ... ) }
	ne := ir.NewIfStmt(base.Pos, ir.NewBinaryExpr(base.Pos, ir.ONE, nto, nfrm), nil, nil)
	ne.Likely = true
	l = append(l, ne)

	fn := typecheck.LookupRuntime("memmove", nl.Type().Elem(), nl.Type().Elem())
	nwid := ir.Node(typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TUINTPTR]))
	setwid := ir.NewAssignStmt(base.Pos, nwid, typecheck.Conv(nlen, types.Types[types.TUINTPTR]))
	ne.Body.Append(setwid)
	nwid = ir.NewBinaryExpr(base.Pos, ir.OMUL, nwid, ir.NewInt(base.Pos, nl.Type().Elem().Size()))
	call := mkcall1(fn, nil, init, nto, nfrm, nwid)
	ne.Body.Append(call)

	typecheck.Stmts(l)
	walkStmtList(l)
	init.Append(l...)
	return nlen
}

// walkDelete walks an ODELETE node.
func walkDelete(init *ir.Nodes, n *ir.CallExpr) ir.Node {
	init.Append(ir.TakeInit(n)...)
	map_ := n.Args[0]
	key := n.Args[1]
	map_ = walkExpr(map_, init)
	key = walkExpr(key, init)

	t := map_.Type()
	fast := mapfast(t)
	key = mapKeyArg(fast, n, key, false)
	return mkcall1(mapfndel(mapdelete[fast], t), nil, init, reflectdata.DeleteMapRType(base.Pos, n), map_, key)
}

// walkLenCap walks an OLEN or OCAP node.
func walkLenCap(n *ir.UnaryExpr, init *ir.Nodes) ir.Node {
	if isRuneCount(n) {
		// Replace len([]rune(string)) with runtime.countrunes(string).
		return mkcall("countrunes", n.Type(), init, typecheck.Conv(n.X.(*ir.ConvExpr).X, types.Types[types.TSTRING]))
	}
	if isByteCount(n) {
		conv := n.X.(*ir.ConvExpr)
		walkStmtList(conv.Init())
		init.Append(ir.TakeInit(conv)...)
		_, len := backingArrayPtrLen(cheapExpr(conv.X, init))
		return len
	}
	if isChanLenCap(n) {
		name := "chanlen"
		if n.Op() == ir.OCAP {
			name = "chancap"
		}
		// cannot use chanfn - closechan takes any, not chan any,
		// because it accepts both send-only and recv-only channels.
		fn := typecheck.LookupRuntime(name, n.X.Type())
		return mkcall1(fn, n.Type(), init, n.X)
	}

	n.X = walkExpr(n.X, init)

	// replace len(*[10]int) with 10.
	// delayed until now to preserve side effects.
	t := n.X.Type()
	if t.IsPtr() {
		t = t.Elem()
	}
	if t.IsArray() {
		// evaluate any side effects in n.X. See issue 72844.
		appendWalkStmt(init, ir.NewAssignStmt(base.Pos, ir.BlankNode, n.X))

		con := ir.NewConstExpr(constant.MakeInt64(t.NumElem()), n)
		con.SetTypecheck(1)
		return con
	}
	return n
}

// walkMakeChan walks an OMAKECHAN node.
func walkMakeChan(n *ir.MakeExpr, init *ir.Nodes) ir.Node {
	// When size fits into int, use makechan instead of
	// makechan64, which is faster and shorter on 32 bit platforms.
	size := n.Len
	fnname := "makechan64"
	argtype := types.Types[types.TINT64]

	// Type checking guarantees that TIDEAL size is positive and fits in an int.
	// The case of size overflow when converting TUINT or TUINTPTR to TINT
	// will be handled by the negative range checks in makechan during runtime.
	if size.Type().IsKind(types.TIDEAL) || size.Type().Size() <= types.Types[types.TUINT].Size() {
		fnname = "makechan"
		argtype = types.Types[types.TINT]
	}

	return mkcall1(chanfn(fnname, 1, n.Type()), n.Type(), init, reflectdata.MakeChanRType(base.Pos, n), typecheck.Conv(size, argtype))
}

// walkMakeMap walks an OMAKEMAP node.
func walkMakeMap(n *ir.MakeExpr, init *ir.Nodes) ir.Node {
	t := n.Type()
	mapType := reflectdata.MapType()
	hint := n.Len

	// var m *Map
	var m ir.Node
	if n.Esc() == ir.EscNone {
		// Allocate hmap on stack.

		// var mv Map
		// m = &mv
		m = stackTempAddr(init, mapType)

		// Allocate one group pointed to by m.dirPtr on stack if hint
		// is not larger than MapGroupSlots. In case hint is
		// larger, runtime.makemap will allocate on the heap.
		// Maximum key and elem size is 128 bytes, larger objects
		// are stored with an indirection. So max bucket size is 2048+eps.
		if !ir.IsConst(hint, constant.Int) ||
			constant.Compare(hint.Val(), token.LEQ, constant.MakeInt64(abi.MapGroupSlots)) {

			// In case hint is larger than MapGroupSlots
			// runtime.makemap will allocate on the heap, see
			// #20184
			//
			// if hint <= abi.MapGroupSlots {
			//     var gv group
			//     g = &gv
			//     g.ctrl = abi.MapCtrlEmpty
			//     m.dirPtr = g
			// }

			nif := ir.NewIfStmt(base.Pos, ir.NewBinaryExpr(base.Pos, ir.OLE, hint, ir.NewInt(base.Pos, abi.MapGroupSlots)), nil, nil)
			nif.Likely = true

			groupType := reflectdata.MapGroupType(t)

			// var gv group
			// g = &gv
			g := stackTempAddr(&nif.Body, groupType)

			// Can't use ir.NewInt because bit 63 is set, which
			// makes conversion to uint64 upset.
			empty := ir.NewBasicLit(base.Pos, types.UntypedInt, constant.MakeUint64(abi.MapCtrlEmpty))

			// g.ctrl = abi.MapCtrlEmpty
			csym := groupType.Field(0).Sym // g.ctrl see reflectdata/map.go
			ca := ir.NewAssignStmt(base.Pos, ir.NewSelectorExpr(base.Pos, ir.ODOT, g, csym), empty)
			nif.Body.Append(ca)

			// m.dirPtr = g
			dsym := mapType.Field(2).Sym // m.dirPtr see reflectdata/map.go
			na := ir.NewAssignStmt(base.Pos, ir.NewSelectorExpr(base.Pos, ir.ODOT, m, dsym), typecheck.ConvNop(g, types.Types[types.TUNSAFEPTR]))
			nif.Body.Append(na)
			appendWalkStmt(init, nif)
		}
	}

	if ir.IsConst(hint, constant.Int) && constant.Compare(hint.Val(), token.LEQ, constant.MakeInt64(abi.MapGroupSlots)) {
		// Handling make(map[any]any) and
		// make(map[any]any, hint) where hint <= abi.MapGroupSlots
		// specially allows for faster map initialization and
		// improves binary size by using calls with fewer arguments.
		// For hint <= abi.MapGroupSlots no groups will be
		// allocated by makemap. Therefore, no groups need to be
		// allocated in this code path.
		if n.Esc() == ir.EscNone {
			// Only need to initialize m.seed since
			// m map has been allocated on the stack already.
			// m.seed = uintptr(rand())
			rand := mkcall("rand", types.Types[types.TUINT64], init)
			seedSym := mapType.Field(1).Sym // m.seed see reflectdata/map.go
			appendWalkStmt(init, ir.NewAssignStmt(base.Pos, ir.NewSelectorExpr(base.Pos, ir.ODOT, m, seedSym), typecheck.Conv(rand, types.Types[types.TUINTPTR])))
			return typecheck.ConvNop(m, t)
		}
		// Call runtime.makemap_small to allocate a
		// map on the heap and initialize the map's seed field.
		fn := typecheck.LookupRuntime("makemap_small", t.Key(), t.Elem())
		return mkcall1(fn, n.Type(), init)
	}

	if n.Esc() != ir.EscNone {
		m = typecheck.NodNil()
	}

	// Map initialization with a variable or large hint is
	// more complicated. We therefore generate a call to
	// runtime.makemap to initialize hmap and allocate the
	// map buckets.

	// When hint fits into int, use makemap instead of
	// makemap64, which is faster and shorter on 32 bit platforms.
	fnname := "makemap64"
	argtype := types.Types[types.TINT64]

	// Type checking guarantees that TIDEAL hint is positive and fits in an int.
	// See checkmake call in TMAP case of OMAKE case in OpSwitch in typecheck1 function.
	// The case of hint overflow when converting TUINT or TUINTPTR to TINT
	// will be handled by the negative range checks in makemap during runtime.
	if hint.Type().IsKind(types.TIDEAL) || hint.Type().Size() <= types.Types[types.TUINT].Size() {
		fnname = "makemap"
		argtype = types.Types[types.TINT]
	}

	fn := typecheck.LookupRuntime(fnname, mapType, t.Key(), t.Elem())
	return mkcall1(fn, n.Type(), init, reflectdata.MakeMapRType(base.Pos, n), typecheck.Conv(hint, argtype), m)
}

// walkMakeSlice walks an OMAKESLICE node.
func walkMakeSlice(n *ir.MakeExpr, init *ir.Nodes) ir.Node {
	len := n.Len
	cap := n.Cap
	len = safeExpr(len, init)
	if cap != nil {
		cap = safeExpr(cap, init)
	} else {
		cap = len
	}
	t := n.Type()
	if t.Elem().NotInHeap() {
		base.Errorf("%v can't be allocated in Go; it is incomplete (or unallocatable)", t.Elem())
	}

	tryStack := false
	if n.Esc() == ir.EscNone {
		if why := escape.HeapAllocReason(n); why != "" {
			base.Fatalf("%v has EscNone, but %v", n, why)
		}
		if ir.IsSmallIntConst(cap) {
			// Constant backing array - allocate it and slice it.
			cap := typecheck.IndexConst(cap)
			// Note that len might not be constant. If it isn't, check for panics.
			// cap is constrained to [0,2^31) or [0,2^63) depending on whether
			// we're in 32-bit or 64-bit systems. So it's safe to do:
			//
			// if uint64(len) > cap {
			//     if len < 0 { panicmakeslicelen() }
			//     panicmakeslicecap()
			// }
			nif := ir.NewIfStmt(base.Pos, ir.NewBinaryExpr(base.Pos, ir.OGT, typecheck.Conv(len, types.Types[types.TUINT64]), ir.NewInt(base.Pos, cap)), nil, nil)
			niflen := ir.NewIfStmt(base.Pos, ir.NewBinaryExpr(base.Pos, ir.OLT, len, ir.NewInt(base.Pos, 0)), nil, nil)
			niflen.Body = []ir.Node{mkcall("panicmakeslicelen", nil, init)}
			nif.Body.Append(niflen, mkcall("panicmakeslicecap", nil, init))
			appendWalkStmt(init, nif)

			// var arr [cap]E
			// s = arr[:len]
			t := types.NewArray(t.Elem(), cap) // [cap]E
			arr := typecheck.TempAt(base.Pos, ir.CurFunc, t)
			appendWalkStmt(init, ir.NewAssignStmt(base.Pos, arr, nil))    // zero temp
			s := ir.NewSliceExpr(base.Pos, ir.OSLICE, arr, nil, len, nil) // arr[:len]
			// The conv is necessary in case n.Type is named.
			return walkExpr(typecheck.Expr(typecheck.Conv(s, n.Type())), init)
		}
		// Check that this optimization is enabled in general and for this node.
		tryStack = base.Flag.N == 0 && base.VariableMakeHash.MatchPos(n.Pos(), nil)
	}

	// The final result is assigned to this variable.
	slice := typecheck.TempAt(base.Pos, ir.CurFunc, n.Type()) // []E result (possibly named)

	if tryStack {
		// K := maxStackSize/sizeof(E)
		// if cap <= K {
		//     var arr [K]E
		//     slice = arr[:len:cap]
		// } else {
		//     slice = makeslice(elemType, len, cap)
		// }
		maxStackSize := int64(base.Debug.VariableMakeThreshold)
		K := maxStackSize / t.Elem().Size() // rounds down
		if K > 0 {                          // skip if elem size is too big.
			nif := ir.NewIfStmt(base.Pos, ir.NewBinaryExpr(base.Pos, ir.OLE, typecheck.Conv(cap, types.Types[types.TUINT64]), ir.NewInt(base.Pos, K)), nil, nil)

			// cap is in bounds after the K check, but len might not be.
			// (Note that the slicing below would generate a panic for
			// the same bad cases, but we want makeslice panics, not
			// regular slicing panics.)
			lenCap := ir.NewIfStmt(base.Pos, ir.NewBinaryExpr(base.Pos, ir.OGT, typecheck.Conv(len, types.Types[types.TUINT64]), typecheck.Conv(cap, types.Types[types.TUINT64])), nil, nil)
			lenZero := ir.NewIfStmt(base.Pos, ir.NewBinaryExpr(base.Pos, ir.OLT, len, ir.NewInt(base.Pos, 0)), nil, nil)
			lenZero.Body.Append(mkcall("panicmakeslicelen", nil, &lenZero.Body))
			lenCap.Body.Append(lenZero)
			lenCap.Body.Append(mkcall("panicmakeslicecap", nil, &lenCap.Body))
			nif.Body.Append(lenCap)

			t := types.NewArray(t.Elem(), K) // [K]E
			// Wrap in a struct containing a [0]uintptr field to force
			// pointer alignment. Some user code expects higher alignment
			// than what is guaranteed by the element type, because that's
			// the behavior they observed of mallocgc, and then relied upon.
			// See issue 73199.
			field := typecheck.Lookup("arr")
			t = types.NewStruct([]*types.Field{
				{Sym: types.BlankSym, Type: types.NewArray(types.Types[types.TUINTPTR], 0)},
				{Sym: field, Type: t},
			})
			t.SetNoalg(true)
			store := typecheck.TempAt(base.Pos, ir.CurFunc, t)            // var store struct{_ uintptr[0]; arr [K]E}
			nif.Body.Append(ir.NewAssignStmt(base.Pos, store, nil))       // store = {} (zero it)
			arr := ir.NewSelectorExpr(base.Pos, ir.ODOT, store, field)    // arr = store.arr
			s := ir.NewSliceExpr(base.Pos, ir.OSLICE, arr, nil, len, cap) // store.arr[:len:cap]
			nif.Body.Append(ir.NewAssignStmt(base.Pos, slice, s))         // slice = store.arr[:len:cap]

			appendWalkStmt(init, typecheck.Stmt(nif))

			// Put makeslice call below in the else branch.
			init = &nif.Else
		}
	}

	// Set up a call to makeslice.
	// When len and cap can fit into int, use makeslice instead of
	// makeslice64, which is faster and shorter on 32 bit platforms.
	fnname := "makeslice64"
	argtype := types.Types[types.TINT64]

	// Type checking guarantees that TIDEAL len/cap are positive and fit in an int.
	// The case of len or cap overflow when converting TUINT or TUINTPTR to TINT
	// will be handled by the negative range checks in makeslice during runtime.
	if (len.Type().IsKind(types.TIDEAL) || len.Type().Size() <= types.Types[types.TUINT].Size()) &&
		(cap.Type().IsKind(types.TIDEAL) || cap.Type().Size() <= types.Types[types.TUINT].Size()) {
		fnname = "makeslice"
		argtype = types.Types[types.TINT]
	}
	fn := typecheck.LookupRuntime(fnname)
	ptr := mkcall1(fn, types.Types[types.TUNSAFEPTR], init, reflectdata.MakeSliceElemRType(base.Pos, n), typecheck.Conv(len, argtype), typecheck.Conv(cap, argtype))
	ptr.MarkNonNil()
	len = typecheck.Conv(len, types.Types[types.TINT])
	cap = typecheck.Conv(cap, types.Types[types.TINT])
	s := ir.NewSliceHeaderExpr(base.Pos, t, ptr, len, cap)
	appendWalkStmt(init, ir.NewAssignStmt(base.Pos, slice, s))

	return slice
}

// walkMakeSliceCopy walks an OMAKESLICECOPY node.
func walkMakeSliceCopy(n *ir.MakeExpr, init *ir.Nodes) ir.Node {
	if n.Esc() == ir.EscNone {
		base.Fatalf("OMAKESLICECOPY with EscNone: %v", n)
	}

	t := n.Type()
	if t.Elem().NotInHeap() {
		base.Errorf("%v can't be allocated in Go; it is incomplete (or unallocatable)", t.Elem())
	}

	length := typecheck.Conv(n.Len, types.Types[types.TINT])
	copylen := ir.NewUnaryExpr(base.Pos, ir.OLEN, n.Cap)
	copyptr := ir.NewUnaryExpr(base.Pos, ir.OSPTR, n.Cap)

	if !t.Elem().HasPointers() && n.Bounded() {
		// When len(to)==len(from) and elements have no pointers:
		// replace make+copy with runtime.mallocgc+runtime.memmove.

		// We do not check for overflow of len(to)*elem.Width here
		// since len(from) is an existing checked slice capacity
		// with same elem.Width for the from slice.
		size := ir.NewBinaryExpr(base.Pos, ir.OMUL, typecheck.Conv(length, types.Types[types.TUINTPTR]), typecheck.Conv(ir.NewInt(base.Pos, t.Elem().Size()), types.Types[types.TUINTPTR]))

		// instantiate mallocgc(size uintptr, typ *byte, needszero bool) unsafe.Pointer
		fn := typecheck.LookupRuntime("mallocgc")
		ptr := mkcall1(fn, types.Types[types.TUNSAFEPTR], init, size, typecheck.NodNil(), ir.NewBool(base.Pos, false))
		ptr.MarkNonNil()
		sh := ir.NewSliceHeaderExpr(base.Pos, t, ptr, length, length)

		s := typecheck.TempAt(base.Pos, ir.CurFunc, t)
		r := typecheck.Stmt(ir.NewAssignStmt(base.Pos, s, sh))
		r = walkExpr(r, init)
		init.Append(r)

		// instantiate memmove(to *any, frm *any, size uintptr)
		fn = typecheck.LookupRuntime("memmove", t.Elem(), t.Elem())
		ncopy := mkcall1(fn, nil, init, ir.NewUnaryExpr(base.Pos, ir.OSPTR, s), copyptr, size)
		init.Append(walkExpr(typecheck.Stmt(ncopy), init))

		return s
	}
	// Replace make+copy with runtime.makeslicecopy.
	// instantiate makeslicecopy(typ *byte, tolen int, fromlen int, from unsafe.Pointer) unsafe.Pointer
	fn := typecheck.LookupRuntime("makeslicecopy")
	ptr := mkcall1(fn, types.Types[types.TUNSAFEPTR], init, reflectdata.MakeSliceElemRType(base.Pos, n), length, copylen, typecheck.Conv(copyptr, types.Types[types.TUNSAFEPTR]))
	ptr.MarkNonNil()
	sh := ir.NewSliceHeaderExpr(base.Pos, t, ptr, length, length)
	return walkExpr(typecheck.Expr(sh), init)
}

// walkNew walks an ONEW node.
func walkNew(n *ir.UnaryExpr, init *ir.Nodes) ir.Node {
	t := n.Type().Elem()
	if t.NotInHeap() {
		base.Errorf("%v can't be allocated in Go; it is incomplete (or unallocatable)", n.Type().Elem())
	}
	if n.Esc() == ir.EscNone {
		if t.Size() > ir.MaxImplicitStackVarSize {
			base.Fatalf("large ONEW with EscNone: %v", n)
		}
		return stackTempAddr(init, t)
	}
	types.CalcSize(t)
	n.MarkNonNil()
	return n
}

func walkMinMax(n *ir.CallExpr, init *ir.Nodes) ir.Node {
	init.Append(ir.TakeInit(n)...)
	walkExprList(n.Args, init)
	return n
}

// generate code for print.
func walkPrint(nn *ir.CallExpr, init *ir.Nodes) ir.Node {
	// Hoist all the argument evaluation up before the lock.
	walkExprListCheap(nn.Args, init)

	// For println, add " " between elements and "\n" at the end.
	if nn.Op() == ir.OPRINTLN {
		s := nn.Args
		t := make([]ir.Node, 0, len(s)*2)
		for i, n := range s {
			if i != 0 {
				t = append(t, ir.NewString(base.Pos, " "))
			}
			t = append(t, n)
		}
		t = append(t, ir.NewString(base.Pos, "\n"))
		nn.Args = t
	}

	// Collapse runs of constant strings.
	s := nn.Args
	t := make([]ir.Node, 0, len(s))
	for i := 0; i < len(s); {
		var strs []string
		for i < len(s) && ir.IsConst(s[i], constant.String) {
			strs = append(strs, ir.StringVal(s[i]))
			i++
		}
		if len(strs) > 0 {
			t = append(t, ir.NewString(base.Pos, strings.Join(strs, "")))
		}
		if i < len(s) {
			t = append(t, s[i])
			i++
		}
	}
	nn.Args = t

	calls := []ir.Node{mkcall("printlock", nil, init)}
	for i, n := range nn.Args {
		if n.Op() == ir.OLITERAL {
			if n.Type() == types.UntypedRune {
				n = typecheck.DefaultLit(n, types.RuneType)
			}

			switch n.Val().Kind() {
			case constant.Int:
				n = typecheck.DefaultLit(n, types.Types[types.TINT64])

			case constant.Float:
				n = typecheck.DefaultLit(n, types.Types[types.TFLOAT64])
			}
		}

		if n.Op() != ir.OLITERAL && n.Type() != nil && n.Type().Kind() == types.TIDEAL {
			n = typecheck.DefaultLit(n, types.Types[types.TINT64])
		}
		n = typecheck.DefaultLit(n, nil)
		nn.Args[i] = n
		if n.Type() == nil || n.Type().Kind() == types.TFORW {
			continue
		}

		var on *ir.Name
		switch n.Type().Kind() {
		case types.TINTER:
			if n.Type().IsEmptyInterface() {
				on = typecheck.LookupRuntime("printeface", n.Type())
			} else {
				on = typecheck.LookupRuntime("printiface", n.Type())
			}
		case types.TPTR:
			if n.Type().Elem().NotInHeap() {
				on = typecheck.LookupRuntime("printuintptr")
				n = ir.NewConvExpr(base.Pos, ir.OCONV, nil, n)
				n.SetType(types.Types[types.TUNSAFEPTR])
				n = ir.NewConvExpr(base.Pos, ir.OCONV, nil, n)
				n.SetType(types.Types[types.TUINTPTR])
				break
			}
			fallthrough
		case types.TCHAN, types.TMAP, types.TFUNC, types.TUNSAFEPTR:
			on = typecheck.LookupRuntime("printpointer", n.Type())
		case types.TSLICE:
			on = typecheck.LookupRuntime("printslice", n.Type())
		case types.TUINT, types.TUINT8, types.TUINT16, types.TUINT32, types.TUINT64, types.TUINTPTR:
			if types.RuntimeSymName(n.Type().Sym()) == "hex" {
				on = typecheck.LookupRuntime("printhex")
			} else {
				on = typecheck.LookupRuntime("printuint")
			}
		case types.TINT, types.TINT8, types.TINT16, types.TINT32, types.TINT64:
			on = typecheck.LookupRuntime("printint")
		case types.TFLOAT32:
			on = typecheck.LookupRuntime("printfloat32")
		case types.TFLOAT64:
			on = typecheck.LookupRuntime("printfloat64")
		case types.TCOMPLEX64:
			on = typecheck.LookupRuntime("printcomplex64")
		case types.TCOMPLEX128:
			on = typecheck.LookupRuntime("printcomplex128")
		case types.TBOOL:
			on = typecheck.LookupRuntime("printbool")
		case types.TSTRING:
			cs := ""
			if ir.IsConst(n, constant.String) {
				cs = ir.StringVal(n)
			}
			// Print values of the named type `quoted` using printquoted.
			if types.RuntimeSymName(n.Type().Sym()) == "quoted" {
				on = typecheck.LookupRuntime("printquoted")
			} else {
				switch cs {
				case " ":
					on = typecheck.LookupRuntime("printsp")
				case "\n":
					on = typecheck.LookupRuntime("printnl")
				default:
					on = typecheck.LookupRuntime("printstring")
				}
			}
		default:
			badtype(nn.Op(), n.Type(), nil)
			continue
		}

		r := ir.NewCallExpr(base.Pos, ir.OCALL, on, nil)
		if params := on.Type().Params(); len(params) > 0 {
			t := params[0].Type
			n = typecheck.Conv(n, t)
			r.Args.Append(n)
		}
		calls = append(calls, r)
	}

	calls = append(calls, mkcall("printunlock", nil, init))

	typecheck.Stmts(calls)
	walkExprList(calls, init)

	r := ir.NewBlockStmt(base.Pos, nil)
	r.List = calls
	return walkStmt(typecheck.Stmt(r))
}

// walkRecover walks an ORECOVER node.
func walkRecover(nn *ir.CallExpr, init *ir.Nodes) ir.Node {
	return mkcall("gorecover", nn.Type(), init)
}

// walkUnsafeData walks an OUNSAFESLICEDATA or OUNSAFESTRINGDATA expression.
func walkUnsafeData(n *ir.UnaryExpr, init *ir.Nodes) ir.Node {
	slice := walkExpr(n.X, init)
	res := typecheck.Expr(ir.NewUnaryExpr(n.Pos(), ir.OSPTR, slice))
	res.SetType(n.Type())
	return walkExpr(res, init)
}

func walkUnsafeSlice(n *ir.BinaryExpr, init *ir.Nodes) ir.Node {
	ptr := safeExpr(n.X, init)
	len := safeExpr(n.Y, init)
	sliceType := n.Type()

	lenType := types.Types[types.TINT64]
	unsafePtr := typecheck.Conv(ptr, types.Types[types.TUNSAFEPTR])

	// If checkptr enabled, call runtime.unsafeslicecheckptr to check ptr and len.
	// for simplicity, unsafeslicecheckptr always uses int64.
	// Type checking guarantees that TIDEAL len/cap are positive and fit in an int.
	// The case of len or cap overflow when converting TUINT or TUINTPTR to TINT
	// will be handled by the negative range checks in unsafeslice during runtime.
	if ir.ShouldCheckPtr(ir.CurFunc, 1) {
		fnname := "unsafeslicecheckptr"
		fn := typecheck.LookupRuntime(fnname)
		init.Append(mkcall1(fn, nil, init, reflectdata.UnsafeSliceElemRType(base.Pos, n), unsafePtr, typecheck.Conv(len, lenType)))
	} else {
		// Otherwise, open code unsafe.Slice to prevent runtime call overhead.
		// Keep this code in sync with runtime.unsafeslice{,64}
		if len.Type().IsKind(types.TIDEAL) || len.Type().Size() <= types.Types[types.TUINT].Size() {
			lenType = types.Types[types.TINT]
		} else {
			// len64 := int64(len)
			// if int64(int(len64)) != len64 {
			//     panicunsafeslicelen()
			// }
			len64 := typecheck.Conv(len, lenType)
			nif := ir.NewIfStmt(base.Pos, nil, nil, nil)
			nif.Cond = ir.NewBinaryExpr(base.Pos, ir.ONE, typecheck.Conv(typecheck.Conv(len64, types.Types[types.TINT]), lenType), len64)
			nif.Body.Append(mkcall("panicunsafeslicelen", nil, &nif.Body))
			appendWalkStmt(init, nif)
		}

		// if len < 0 { panicunsafeslicelen() }
		nif := ir.NewIfStmt(base.Pos, nil, nil, nil)
		nif.Cond = ir.NewBinaryExpr(base.Pos, ir.OLT, typecheck.Conv(len, lenType), ir.NewInt(base.Pos, 0))
		nif.Body.Append(mkcall("panicunsafeslicelen", nil, &nif.Body))
		appendWalkStmt(init, nif)

		if sliceType.Elem().Size() == 0 {
			// if ptr == nil && len > 0  {
			//      panicunsafesliceptrnil()
			// }
			nifPtr := ir.NewIfStmt(base.Pos, nil, nil, nil)
			isNil := ir.NewBinaryExpr(base.Pos, ir.OEQ, unsafePtr, typecheck.NodNil())
			gtZero := ir.NewBinaryExpr(base.Pos, ir.OGT, typecheck.Conv(len, lenType), ir.NewInt(base.Pos, 0))
			nifPtr.Cond =
				ir.NewLogicalExpr(base.Pos, ir.OANDAND, isNil, gtZero)
			nifPtr.Body.Append(mkcall("panicunsafeslicenilptr", nil, &nifPtr.Body))
			appendWalkStmt(init, nifPtr)

			h := ir.NewSliceHeaderExpr(n.Pos(), sliceType,
				typecheck.Conv(ptr, types.Types[types.TUNSAFEPTR]),
				typecheck.Conv(len, types.Types[types.TINT]),
				typecheck.Conv(len, types.Types[types.TINT]))
			return walkExpr(typecheck.Expr(h), init)
		}

		// mem, overflow := math.mulUintptr(et.size, len)
		mem := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TUINTPTR])
		overflow := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TBOOL])

		decl := types.NewSignature(nil,
			[]*types.Field{
				types.NewField(base.Pos, nil, types.Types[types.TUINTPTR]),
				types.NewField(base.Pos, nil, types.Types[types.TUINTPTR]),
			},
			[]*types.Field{
				types.NewField(base.Pos, nil, types.Types[types.TUINTPTR]),
				types.NewField(base.Pos, nil, types.Types[types.TBOOL]),
			})

		fn := ir.NewFunc(n.Pos(), n.Pos(), math_MulUintptr, decl)

		call := mkcall1(fn.Nname, fn.Type().ResultsTuple(), init, ir.NewInt(base.Pos, sliceType.Elem().Size()), typecheck.Conv(typecheck.Conv(len, lenType), types.Types[types.TUINTPTR]))
		appendWalkStmt(init, ir.NewAssignListStmt(base.Pos, ir.OAS2, []ir.Node{mem, overflow}, []ir.Node{call}))

		// if overflow || mem > -uintptr(ptr) {
		//     if ptr == nil {
		//         panicunsafesliceptrnil()
		//     }
		//     panicunsafeslicelen()
		// }
		nif = ir.NewIfStmt(base.Pos, nil, nil, nil)
		memCond := ir.NewBinaryExpr(base.Pos, ir.OGT, mem, ir.NewUnaryExpr(base.Pos, ir.ONEG, typecheck.Conv(unsafePtr, types.Types[types.TUINTPTR])))
		nif.Cond = ir.NewLogicalExpr(base.Pos, ir.OOROR, overflow, memCond)
		nifPtr := ir.NewIfStmt(base.Pos, nil, nil, nil)
		nifPtr.Cond = ir.NewBinaryExpr(base.Pos, ir.OEQ, unsafePtr, typecheck.NodNil())
		nifPtr.Body.Append(mkcall("panicunsafeslicenilptr", nil, &nifPtr.Body))
		nif.Body.Append(nifPtr, mkcall("panicunsafeslicelen", nil, &nif.Body))
		appendWalkStmt(init, nif)
	}

	h := ir.NewSliceHeaderExpr(n.Pos(), sliceType,
		typecheck.Conv(ptr, types.Types[types.TUNSAFEPTR]),
		typecheck.Conv(len, types.Types[types.TINT]),
		typecheck.Conv(len, types.Types[types.TINT]))
	return walkExpr(typecheck.Expr(h), init)
}

var math_MulUintptr = &types.Sym{Pkg: types.NewPkg("internal/runtime/math", "math"), Name: "MulUintptr"}

func walkUnsafeString(n *ir.BinaryExpr, init *ir.Nodes) ir.Node {
	ptr := safeExpr(n.X, init)
	len := safeExpr(n.Y, init)

	lenType := types.Types[types.TINT64]
	unsafePtr := typecheck.Conv(ptr, types.Types[types.TUNSAFEPTR])

	// If checkptr enabled, call runtime.unsafestringcheckptr to check ptr and len.
	// for simplicity, unsafestringcheckptr always uses int64.
	// Type checking guarantees that TIDEAL len are positive and fit in an int.
	if ir.ShouldCheckPtr(ir.CurFunc, 1) {
		fnname := "unsafestringcheckptr"
		fn := typecheck.LookupRuntime(fnname)
		init.Append(mkcall1(fn, nil, init, unsafePtr, typecheck.Conv(len, lenType)))
	} else {
		// Otherwise, open code unsafe.String to prevent runtime call overhead.
		// Keep this code in sync with runtime.unsafestring{,64}
		if len.Type().IsKind(types.TIDEAL) || len.Type().Size() <= types.Types[types.TUINT].Size() {
			lenType = types.Types[types.TINT]
		} else {
			// len64 := int64(len)
			// if int64(int(len64)) != len64 {
			//     panicunsafestringlen()
			// }
			len64 := typecheck.Conv(len, lenType)
			nif := ir.NewIfStmt(base.Pos, nil, nil, nil)
			nif.Cond = ir.NewBinaryExpr(base.Pos, ir.ONE, typecheck.Conv(typecheck.Conv(len64, types.Types[types.TINT]), lenType), len64)
			nif.Body.Append(mkcall("panicunsafestringlen", nil, &nif.Body))
			appendWalkStmt(init, nif)
		}

		// if len < 0 { panicunsafestringlen() }
		nif := ir.NewIfStmt(base.Pos, nil, nil, nil)
		nif.Cond = ir.NewBinaryExpr(base.Pos, ir.OLT, typecheck.Conv(len, lenType), ir.NewInt(base.Pos, 0))
		nif.Body.Append(mkcall("panicunsafestringlen", nil, &nif.Body))
		appendWalkStmt(init, nif)

		// if uintpr(len) > -uintptr(ptr) {
		//    if ptr == nil {
		//       panicunsafestringnilptr()
		//    }
		//    panicunsafeslicelen()
		// }
		nifLen := ir.NewIfStmt(base.Pos, nil, nil, nil)
		nifLen.Cond = ir.NewBinaryExpr(base.Pos, ir.OGT, typecheck.Conv(len, types.Types[types.TUINTPTR]), ir.NewUnaryExpr(base.Pos, ir.ONEG, typecheck.Conv(unsafePtr, types.Types[types.TUINTPTR])))
		nifPtr := ir.NewIfStmt(base.Pos, nil, nil, nil)
		nifPtr.Cond = ir.NewBinaryExpr(base.Pos, ir.OEQ, unsafePtr, typecheck.NodNil())
		nifPtr.Body.Append(mkcall("panicunsafestringnilptr", nil, &nifPtr.Body))
		nifLen.Body.Append(nifPtr, mkcall("panicunsafestringlen", nil, &nifLen.Body))
		appendWalkStmt(init, nifLen)
	}
	h := ir.NewStringHeaderExpr(n.Pos(),
		typecheck.Conv(ptr, types.Types[types.TUNSAFEPTR]),
		typecheck.Conv(len, types.Types[types.TINT]),
	)
	return walkExpr(typecheck.Expr(h), init)
}

func badtype(op ir.Op, tl, tr *types.Type) {
	var s string
	if tl != nil {
		s += fmt.Sprintf("\n\t%v", tl)
	}
	if tr != nil {
		s += fmt.Sprintf("\n\t%v", tr)
	}

	// common mistake: *struct and *interface.
	if tl != nil && tr != nil && tl.IsPtr() && tr.IsPtr() {
		if tl.Elem().IsStruct() && tr.Elem().IsInterface() {
			s += "\n\t(*struct vs *interface)"
		} else if tl.Elem().IsInterface() && tr.Elem().IsStruct() {
			s += "\n\t(*interface vs *struct)"
		}
	}

	base.Errorf("illegal types for operand: %v%s", op, s)
}

func writebarrierfn(name string, l *types.Type, r *types.Type) ir.Node {
	return typecheck.LookupRuntime(name, l, r)
}

// isRuneCount reports whether n is of the form len([]rune(string)).
// These are optimized into a call to runtime.countrunes.
func isRuneCount(n ir.Node) bool {
	return base.Flag.N == 0 && !base.Flag.Cfg.Instrumenting && n.Op() == ir.OLEN && n.(*ir.UnaryExpr).X.Op() == ir.OSTR2RUNES
}

// isByteCount reports whether n is of the form len(string([]byte)).
func isByteCount(n ir.Node) bool {
	return base.Flag.N == 0 && !base.Flag.Cfg.Instrumenting && n.Op() == ir.OLEN &&
		(n.(*ir.UnaryExpr).X.Op() == ir.OBYTES2STR || n.(*ir.UnaryExpr).X.Op() == ir.OBYTES2STRTMP)
}

// isChanLenCap reports whether n is of the form len(c) or cap(c) for a channel c.
// Note that this does not check for -n or instrumenting because this
// is a correctness rewrite, not an optimization.
func isChanLenCap(n ir.Node) bool {
	return (n.Op() == ir.OLEN || n.Op() == ir.OCAP) && n.(*ir.UnaryExpr).X.Type().IsChan()
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/closure.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// directClosureCall rewrites a direct call of a function literal into
// a normal function call with closure variables passed as arguments.
// This avoids allocation of a closure object.
//
// For illustration, the following call:
//
//	func(a int) {
//		println(byval)
//		byref++
//	}(42)
//
// becomes:
//
//	func(byval int, &byref *int, a int) {
//		println(byval)
//		(*&byref)++
//	}(byval, &byref, 42)
func directClosureCall(n *ir.CallExpr) {
	clo := n.Fun.(*ir.ClosureExpr)
	clofn := clo.Func

	if !clofn.IsClosure() {
		return // leave for walkClosure to handle
	}

	// We are going to insert captured variables before input args.
	var params []*types.Field
	var decls []*ir.Name
	for _, v := range clofn.ClosureVars {
		if !v.Byval() {
			// If v of type T is captured by reference,
			// we introduce function param &v *T
			// and v remains PAUTOHEAP with &v heapaddr
			// (accesses will implicitly deref &v).

			addr := ir.NewNameAt(clofn.Pos(), typecheck.Lookup("&"+v.Sym().Name), types.NewPtr(v.Type()))
			addr.Curfn = clofn
			v.Heapaddr = addr
			v = addr
		}

		v.Class = ir.PPARAM
		decls = append(decls, v)

		fld := types.NewField(src.NoXPos, v.Sym(), v.Type())
		fld.Nname = v
		params = append(params, fld)
	}

	// f is ONAME of the actual function.
	f := clofn.Nname
	typ := f.Type()

	// Create new function type with parameters prepended, and
	// then update type and declarations.
	typ = types.NewSignature(nil, append(params, typ.Params()...), typ.Results())
	f.SetType(typ)
	clofn.Dcl = append(decls, clofn.Dcl...)

	// Rewrite call.
	n.Fun = f
	n.Args.Prepend(closureArgs(clo)...)

	// Update the call expression's type. We need to do this
	// because typecheck gave it the result type of the OCLOSURE
	// node, but we only rewrote the ONAME node's type. Logically,
	// they're the same, but the stack offsets probably changed.
	if typ.NumResults() == 1 {
		n.SetType(typ.Result(0).Type)
	} else {
		n.SetType(typ.ResultsTuple())
	}

	// Add to Closures for enqueueFunc. It's no longer a proper
	// closure, but we may have already skipped over it in the
	// functions list, so this just ensures it's compiled.
	ir.CurFunc.Closures = append(ir.CurFunc.Closures, clofn)
}

func walkClosure(clo *ir.ClosureExpr, init *ir.Nodes) ir.Node {
	clofn := clo.Func

	// If not a closure, don't bother wrapping.
	if !clofn.IsClosure() {
		if base.Debug.Closure > 0 {
			base.WarnfAt(clo.Pos(), "closure converted to global")
		}
		return clofn.Nname
	}

	// The closure is not trivial or directly called, so it's going to stay a closure.
	ir.ClosureDebugRuntimeCheck(clo)
	clofn.SetNeedctxt(true)

	// The closure expression may be walked more than once if it appeared in composite
	// literal initialization (e.g, see issue #49029).
	//
	// Don't add the closure function to compilation queue more than once, since when
	// compiling a function twice would lead to an ICE.
	if !clofn.Walked() {
		clofn.SetWalked(true)
		ir.CurFunc.Closures = append(ir.CurFunc.Closures, clofn)
	}

	typ := typecheck.ClosureType(clo)

	clos := ir.NewCompLitExpr(base.Pos, ir.OCOMPLIT, typ, nil)
	clos.SetEsc(clo.Esc())
	clos.List = append([]ir.Node{ir.NewUnaryExpr(base.Pos, ir.OCFUNC, clofn.Nname)}, closureArgs(clo)...)
	for i, value := range clos.List {
		clos.List[i] = ir.NewStructKeyExpr(base.Pos, typ.Field(i), value)
	}

	addr := typecheck.NodAddr(clos)
	addr.SetEsc(clo.Esc())

	// Force type conversion from *struct to the func type.
	cfn := typecheck.ConvNop(addr, clo.Type())

	// non-escaping temp to use, if any.
	if x := clo.Prealloc; x != nil {
		if !types.Identical(typ, x.Type()) {
			panic("closure type does not match order's assigned type")
		}
		addr.Prealloc = x
		clo.Prealloc = nil
	}

	return walkExpr(cfn, init)
}

// closureArgs returns a slice of expressions that can be used to
// initialize the given closure's free variables. These correspond
// one-to-one with the variables in clo.Func.ClosureVars, and will be
// either an ONAME node (if the variable is captured by value) or an
// OADDR-of-ONAME node (if not).
func closureArgs(clo *ir.ClosureExpr) []ir.Node {
	fn := clo.Func

	args := make([]ir.Node, len(fn.ClosureVars))
	for i, v := range fn.ClosureVars {
		var outer ir.Node
		outer = v.Outer
		if !v.Byval() {
			outer = typecheck.NodAddrAt(fn.Pos(), outer)
		}
		args[i] = typecheck.Expr(outer)
	}
	return args
}

func walkMethodValue(n *ir.SelectorExpr, init *ir.Nodes) ir.Node {
	// Create closure in the form of a composite literal.
	// For x.M with receiver (x) type T, the generated code looks like:
	//
	//	clos = &struct{F uintptr; R T}{T.M·f, x}
	//
	// Like walkClosure above.

	if n.X.Type().IsInterface() {
		// Trigger panic for method on nil interface now.
		// Otherwise it happens in the wrapper and is confusing.
		n.X = cheapExpr(n.X, init)
		n.X = walkExpr(n.X, nil)

		tab := ir.NewUnaryExpr(base.Pos, ir.OITAB, n.X)
		check := ir.NewUnaryExpr(base.Pos, ir.OCHECKNIL, tab)
		init.Append(typecheck.Stmt(check))
	}

	typ := typecheck.MethodValueType(n)

	clos := ir.NewCompLitExpr(base.Pos, ir.OCOMPLIT, typ, nil)
	clos.SetEsc(n.Esc())
	clos.List = []ir.Node{ir.NewUnaryExpr(base.Pos, ir.OCFUNC, methodValueWrapper(n)), n.X}

	addr := typecheck.NodAddr(clos)
	addr.SetEsc(n.Esc())

	// Force type conversion from *struct to the func type.
	cfn := typecheck.ConvNop(addr, n.Type())

	// non-escaping temp to use, if any.
	if x := n.Prealloc; x != nil {
		if !types.Identical(typ, x.Type()) {
			panic("partial call type does not match order's assigned type")
		}
		addr.Prealloc = x
		n.Prealloc = nil
	}

	return walkExpr(cfn, init)
}

// methodValueWrapper returns the ONAME node representing the
// wrapper function (*-fm) needed for the given method value. If the
// wrapper function hasn't already been created yet, it's created and
// added to typecheck.Target.Decls.
func methodValueWrapper(dot *ir.SelectorExpr) *ir.Name {
	if dot.Op() != ir.OMETHVALUE {
		base.Fatalf("methodValueWrapper: unexpected %v (%v)", dot, dot.Op())
	}

	meth := dot.Sel
	rcvrtype := dot.X.Type()
	sym := ir.MethodSymSuffix(rcvrtype, meth, "-fm")

	if sym.Uniq() {
		return sym.Def.(*ir.Name)
	}
	sym.SetUniq(true)

	base.FatalfAt(dot.Pos(), "missing wrapper for %v", meth)
	panic("unreachable")
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/compare.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"encoding/binary"
	"fmt"
	"go/constant"
	"hash/fnv"
	"io"

	"cmd/compile/internal/base"
	"cmd/compile/internal/compare"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/ssagen"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
)

func fakePC(n ir.Node) ir.Node {
	// In order to get deterministic IDs, we include the package path, absolute filename, line number, column number
	// in the calculation of the fakePC for the IR node.
	hash := fnv.New32()
	// We ignore the errors here because the `io.Writer` in the `hash.Hash` interface never returns an error.
	io.WriteString(hash, base.Ctxt.Pkgpath)
	io.WriteString(hash, base.Ctxt.PosTable.Pos(n.Pos()).AbsFilename())
	binary.Write(hash, binary.LittleEndian, int64(n.Pos().Line()))
	binary.Write(hash, binary.LittleEndian, int64(n.Pos().Col()))
	// We also include the string representation of the node to distinguish autogenerated expression since
	// those get the same `src.XPos`
	io.WriteString(hash, fmt.Sprintf("%v", n))

	return ir.NewInt(base.Pos, int64(hash.Sum32()))
}

// The result of walkCompare MUST be assigned back to n, e.g.
//
//	n.Left = walkCompare(n.Left, init)
func walkCompare(n *ir.BinaryExpr, init *ir.Nodes) ir.Node {
	if n.X.Type().IsInterface() && n.Y.Type().IsInterface() && n.X.Op() != ir.ONIL && n.Y.Op() != ir.ONIL {
		return walkCompareInterface(n, init)
	}

	if n.X.Type().IsString() && n.Y.Type().IsString() {
		return walkCompareString(n, init)
	}

	n.X = walkExpr(n.X, init)
	n.Y = walkExpr(n.Y, init)

	// Given mixed interface/concrete comparison,
	// rewrite into types-equal && data-equal.
	// This is efficient, avoids allocations, and avoids runtime calls.
	//
	// TODO(mdempsky): It would be more general and probably overall
	// simpler to just extend walkCompareInterface to optimize when one
	// operand is an OCONVIFACE.
	if n.X.Type().IsInterface() != n.Y.Type().IsInterface() {
		// Preserve side-effects in case of short-circuiting; see #32187.
		l := cheapExpr(n.X, init)
		r := cheapExpr(n.Y, init)
		// Swap so that l is the interface value and r is the concrete value.
		if n.Y.Type().IsInterface() {
			l, r = r, l
		}

		// Handle both == and !=.
		eq := n.Op()
		andor := ir.OOROR
		if eq == ir.OEQ {
			andor = ir.OANDAND
		}
		// Check for types equal.
		// For empty interface, this is:
		//   l.tab == type(r)
		// For non-empty interface, this is:
		//   l.tab != nil && l.tab._type == type(r)
		//
		// TODO(mdempsky): For non-empty interface comparisons, just
		// compare against the itab address directly?
		var eqtype ir.Node
		tab := ir.NewUnaryExpr(base.Pos, ir.OITAB, l)
		rtyp := reflectdata.CompareRType(base.Pos, n)
		if l.Type().IsEmptyInterface() {
			tab.SetType(types.NewPtr(types.Types[types.TUINT8]))
			tab.SetTypecheck(1)
			eqtype = ir.NewBinaryExpr(base.Pos, eq, tab, rtyp)
		} else {
			nonnil := ir.NewBinaryExpr(base.Pos, brcom(eq), typecheck.NodNil(), tab)
			match := ir.NewBinaryExpr(base.Pos, eq, itabType(tab), rtyp)
			eqtype = ir.NewLogicalExpr(base.Pos, andor, nonnil, match)
		}
		// Check for data equal.
		eqdata := ir.NewBinaryExpr(base.Pos, eq, ifaceData(n.Pos(), l, r.Type()), r)
		// Put it all together.
		expr := ir.NewLogicalExpr(base.Pos, andor, eqtype, eqdata)
		return finishCompare(n, expr, init)
	}

	// Must be comparison of array or struct.
	// Otherwise back end handles it.
	// While we're here, decide whether to
	// inline or call an eq alg.
	t := n.X.Type()
	var inline bool

	maxcmpsize := int64(4)
	unalignedLoad := ssagen.Arch.LinkArch.CanMergeLoads
	if unalignedLoad {
		// Keep this low enough to generate less code than a function call.
		maxcmpsize = 2 * int64(ssagen.Arch.LinkArch.RegSize)
	}

	switch t.Kind() {
	default:
		if base.Debug.Libfuzzer != 0 && t.IsInteger() && (n.X.Name() == nil || !n.X.Name().Libfuzzer8BitCounter()) {
			n.X = cheapExpr(n.X, init)
			n.Y = cheapExpr(n.Y, init)

			// If exactly one comparison operand is
			// constant, invoke the constcmp functions
			// instead, and arrange for the constant
			// operand to be the first argument.
			l, r := n.X, n.Y
			if r.Op() == ir.OLITERAL {
				l, r = r, l
			}
			constcmp := l.Op() == ir.OLITERAL && r.Op() != ir.OLITERAL

			var fn string
			var paramType *types.Type
			switch t.Size() {
			case 1:
				fn = "libfuzzerTraceCmp1"
				if constcmp {
					fn = "libfuzzerTraceConstCmp1"
				}
				paramType = types.Types[types.TUINT8]
			case 2:
				fn = "libfuzzerTraceCmp2"
				if constcmp {
					fn = "libfuzzerTraceConstCmp2"
				}
				paramType = types.Types[types.TUINT16]
			case 4:
				fn = "libfuzzerTraceCmp4"
				if constcmp {
					fn = "libfuzzerTraceConstCmp4"
				}
				paramType = types.Types[types.TUINT32]
			case 8:
				fn = "libfuzzerTraceCmp8"
				if constcmp {
					fn = "libfuzzerTraceConstCmp8"
				}
				paramType = types.Types[types.TUINT64]
			default:
				base.Fatalf("unexpected integer size %d for %v", t.Size(), t)
			}
			init.Append(mkcall(fn, nil, init, tracecmpArg(l, paramType, init), tracecmpArg(r, paramType, init), fakePC(n)))
		}
		return n
	case types.TARRAY:
		// We can compare several elements at once with 2/4/8 byte integer compares
		inline = t.NumElem() <= 1 || (types.IsSimple[t.Elem().Kind()] && (t.NumElem() <= 4 || t.Elem().Size()*t.NumElem() <= maxcmpsize))
	case types.TSTRUCT:
		inline = compare.EqStructCost(t) <= 4
	}

	cmpl := n.X
	for cmpl != nil && cmpl.Op() == ir.OCONVNOP {
		cmpl = cmpl.(*ir.ConvExpr).X
	}
	cmpr := n.Y
	for cmpr != nil && cmpr.Op() == ir.OCONVNOP {
		cmpr = cmpr.(*ir.ConvExpr).X
	}

	// Chose not to inline. Call equality function directly.
	if !inline {
		// eq algs take pointers; cmpl and cmpr must be addressable
		if !ir.IsAddressable(cmpl) || !ir.IsAddressable(cmpr) {
			base.Fatalf("arguments of comparison must be lvalues - %v %v", cmpl, cmpr)
		}

		// Should only arrive here with large memory or
		// a struct/array containing a non-memory field/element.
		// Small memory is handled inline, and single non-memory
		// is handled by walkCompare.
		fn, needsLength := reflectdata.EqFor(t)
		call := ir.NewCallExpr(base.Pos, ir.OCALL, fn, nil)
		addrCmpL := typecheck.NodAddr(cmpl)
		addrCmpR := typecheck.NodAddr(cmpr)
		if !types.IsNoRacePkg(types.LocalPkg) && base.Flag.Race {
			ptrL := typecheck.Conv(typecheck.Conv(addrCmpL, types.Types[types.TUNSAFEPTR]), types.Types[types.TUINTPTR])
			ptrR := typecheck.Conv(typecheck.Conv(addrCmpR, types.Types[types.TUNSAFEPTR]), types.Types[types.TUINTPTR])
			raceFn := typecheck.LookupRuntime("racereadrange")
			size := ir.NewInt(base.Pos, t.Size())
			call.PtrInit().Append(mkcall1(raceFn, nil, init, ptrL, size))
			call.PtrInit().Append(mkcall1(raceFn, nil, init, ptrR, size))
		}
		call.Args.Append(typecheck.Conv(addrCmpL, types.Types[types.TUNSAFEPTR]))
		call.Args.Append(typecheck.Conv(addrCmpR, types.Types[types.TUNSAFEPTR]))
		if needsLength {
			call.Args.Append(ir.NewInt(base.Pos, t.Size()))
		}
		res := ir.Node(call)
		if n.Op() != ir.OEQ {
			res = ir.NewUnaryExpr(base.Pos, ir.ONOT, res)
		}
		return finishCompare(n, res, init)
	}

	// inline: build boolean expression comparing element by element
	andor := ir.OANDAND
	if n.Op() == ir.ONE {
		andor = ir.OOROR
	}
	var expr ir.Node
	comp := func(el, er ir.Node) {
		a := ir.NewBinaryExpr(base.Pos, n.Op(), el, er)
		if expr == nil {
			expr = a
		} else {
			expr = ir.NewLogicalExpr(base.Pos, andor, expr, a)
		}
	}
	and := func(cond ir.Node) {
		if expr == nil {
			expr = cond
		} else {
			expr = ir.NewLogicalExpr(base.Pos, andor, expr, cond)
		}
	}
	cmpl = safeExpr(cmpl, init)
	cmpr = safeExpr(cmpr, init)
	if t.IsStruct() {
		conds, _ := compare.EqStruct(t, cmpl, cmpr)
		if n.Op() == ir.OEQ {
			for _, cond := range conds {
				and(cond)
			}
		} else {
			for _, cond := range conds {
				notCond := ir.NewUnaryExpr(base.Pos, ir.ONOT, cond)
				and(notCond)
			}
		}
	} else {
		step := int64(1)
		remains := t.NumElem() * t.Elem().Size()
		combine64bit := unalignedLoad && types.RegSize == 8 && t.Elem().Size() <= 4 && t.Elem().IsInteger()
		combine32bit := unalignedLoad && t.Elem().Size() <= 2 && t.Elem().IsInteger()
		combine16bit := unalignedLoad && t.Elem().Size() == 1 && t.Elem().IsInteger()
		for i := int64(0); remains > 0; {
			var convType *types.Type
			switch {
			case remains >= 8 && combine64bit:
				convType = types.Types[types.TINT64]
				step = 8 / t.Elem().Size()
			case remains >= 4 && combine32bit:
				convType = types.Types[types.TUINT32]
				step = 4 / t.Elem().Size()
			case remains >= 2 && combine16bit:
				convType = types.Types[types.TUINT16]
				step = 2 / t.Elem().Size()
			default:
				step = 1
			}
			if step == 1 {
				comp(
					ir.NewIndexExpr(base.Pos, cmpl, ir.NewInt(base.Pos, i)),
					ir.NewIndexExpr(base.Pos, cmpr, ir.NewInt(base.Pos, i)),
				)
				i++
				remains -= t.Elem().Size()
			} else {
				elemType := t.Elem().ToUnsigned()
				cmplw := ir.Node(ir.NewIndexExpr(base.Pos, cmpl, ir.NewInt(base.Pos, i)))
				cmplw = typecheck.Conv(cmplw, elemType) // convert to unsigned
				cmplw = typecheck.Conv(cmplw, convType) // widen
				cmprw := ir.Node(ir.NewIndexExpr(base.Pos, cmpr, ir.NewInt(base.Pos, i)))
				cmprw = typecheck.Conv(cmprw, elemType)
				cmprw = typecheck.Conv(cmprw, convType)
				// For code like this:  uint32(s[0]) | uint32(s[1])<<8 | uint32(s[2])<<16 ...
				// ssa will generate a single large load.
				for offset := int64(1); offset < step; offset++ {
					lb := ir.Node(ir.NewIndexExpr(base.Pos, cmpl, ir.NewInt(base.Pos, i+offset)))
					lb = typecheck.Conv(lb, elemType)
					lb = typecheck.Conv(lb, convType)
					lb = ir.NewBinaryExpr(base.Pos, ir.OLSH, lb, ir.NewInt(base.Pos, 8*t.Elem().Size()*offset))
					cmplw = ir.NewBinaryExpr(base.Pos, ir.OOR, cmplw, lb)
					rb := ir.Node(ir.NewIndexExpr(base.Pos, cmpr, ir.NewInt(base.Pos, i+offset)))
					rb = typecheck.Conv(rb, elemType)
					rb = typecheck.Conv(rb, convType)
					rb = ir.NewBinaryExpr(base.Pos, ir.OLSH, rb, ir.NewInt(base.Pos, 8*t.Elem().Size()*offset))
					cmprw = ir.NewBinaryExpr(base.Pos, ir.OOR, cmprw, rb)
				}
				comp(cmplw, cmprw)
				i += step
				remains -= step * t.Elem().Size()
			}
		}
	}
	if expr == nil {
		expr = ir.NewBool(base.Pos, n.Op() == ir.OEQ)
		// We still need to use cmpl and cmpr, in case they contain
		// an expression which might panic. See issue 23837.
		a1 := typecheck.Stmt(ir.NewAssignStmt(base.Pos, ir.BlankNode, cmpl))
		a2 := typecheck.Stmt(ir.NewAssignStmt(base.Pos, ir.BlankNode, cmpr))
		init.Append(a1, a2)
	}
	return finishCompare(n, expr, init)
}

func walkCompareInterface(n *ir.BinaryExpr, init *ir.Nodes) ir.Node {
	swap := n.X.Op() != ir.OCONVIFACE && n.Y.Op() == ir.OCONVIFACE
	n.Y = cheapExpr(n.Y, init)
	n.X = cheapExpr(n.X, init)
	if swap {
		// Put the concrete type first in the comparison.
		// This passes a constant type (itab) to efaceeq (ifaceeq)
		// which is easier to match against in rewrite rules.
		// See issue 70738.
		n.X, n.Y = n.Y, n.X
	}

	eqtab, eqdata := compare.EqInterface(n.X, n.Y)
	var cmp ir.Node
	if n.Op() == ir.OEQ {
		cmp = ir.NewLogicalExpr(base.Pos, ir.OANDAND, eqtab, eqdata)
	} else {
		eqtab.SetOp(ir.ONE)
		cmp = ir.NewLogicalExpr(base.Pos, ir.OOROR, eqtab, ir.NewUnaryExpr(base.Pos, ir.ONOT, eqdata))
	}
	return finishCompare(n, cmp, init)
}

func walkCompareString(n *ir.BinaryExpr, init *ir.Nodes) ir.Node {
	if base.Debug.Libfuzzer != 0 {
		if !ir.IsConst(n.X, constant.String) || !ir.IsConst(n.Y, constant.String) {
			fn := "libfuzzerHookStrCmp"
			n.X = cheapExpr(n.X, init)
			n.Y = cheapExpr(n.Y, init)
			paramType := types.Types[types.TSTRING]
			init.Append(mkcall(fn, nil, init, tracecmpArg(n.X, paramType, init), tracecmpArg(n.Y, paramType, init), fakePC(n)))
		}
	}
	// Rewrite comparisons to short constant strings as length+byte-wise comparisons.
	var cs, ncs ir.Node // const string, non-const string
	switch {
	case ir.IsConst(n.X, constant.String) && ir.IsConst(n.Y, constant.String):
		// ignore; will be constant evaluated
	case ir.IsConst(n.X, constant.String):
		cs = n.X
		ncs = n.Y
	case ir.IsConst(n.Y, constant.String):
		cs = n.Y
		ncs = n.X
	}
	if cs != nil {
		cmp := n.Op()
		// Our comparison below assumes that the non-constant string
		// is on the left hand side, so rewrite "" cmp x to x cmp "".
		// See issue 24817.
		if ir.IsConst(n.X, constant.String) {
			cmp = brrev(cmp)
		}

		// maxRewriteLen was chosen empirically.
		// It is the value that minimizes cmd/go file size
		// across most architectures.
		// See the commit description for CL 26758 for details.
		maxRewriteLen := 6
		// Some architectures can load unaligned byte sequence as 1 word.
		// So we can cover longer strings with the same amount of code.
		canCombineLoads := ssagen.Arch.LinkArch.CanMergeLoads
		combine64bit := false
		if canCombineLoads {
			// Keep this low enough to generate less code than a function call.
			maxRewriteLen = 2 * ssagen.Arch.LinkArch.RegSize
			combine64bit = ssagen.Arch.LinkArch.RegSize >= 8
		}

		var and ir.Op
		switch cmp {
		case ir.OEQ:
			and = ir.OANDAND
		case ir.ONE:
			and = ir.OOROR
		default:
			// Don't do byte-wise comparisons for <, <=, etc.
			// They're fairly complicated.
			// Length-only checks are ok, though.
			maxRewriteLen = 0
		}
		if s := ir.StringVal(cs); len(s) <= maxRewriteLen {
			if len(s) > 0 {
				ncs = safeExpr(ncs, init)
			}
			r := ir.Node(ir.NewBinaryExpr(base.Pos, cmp, ir.NewUnaryExpr(base.Pos, ir.OLEN, ncs), ir.NewInt(base.Pos, int64(len(s)))))
			remains := len(s)
			for i := 0; remains > 0; {
				if remains == 1 || !canCombineLoads {
					cb := ir.NewInt(base.Pos, int64(s[i]))
					ncb := ir.NewIndexExpr(base.Pos, ncs, ir.NewInt(base.Pos, int64(i)))
					r = ir.NewLogicalExpr(base.Pos, and, r, ir.NewBinaryExpr(base.Pos, cmp, ncb, cb))
					remains--
					i++
					continue
				}
				var step int
				var convType *types.Type
				switch {
				case remains >= 8 && combine64bit:
					convType = types.Types[types.TINT64]
					step = 8
				case remains >= 4:
					convType = types.Types[types.TUINT32]
					step = 4
				case remains >= 2:
					convType = types.Types[types.TUINT16]
					step = 2
				}
				ncsubstr := typecheck.Conv(ir.NewIndexExpr(base.Pos, ncs, ir.NewInt(base.Pos, int64(i))), convType)
				csubstr := int64(s[i])
				// Calculate large constant from bytes as sequence of shifts and ors.
				// Like this:  uint32(s[0]) | uint32(s[1])<<8 | uint32(s[2])<<16 ...
				// ssa will combine this into a single large load.
				for offset := 1; offset < step; offset++ {
					b := typecheck.Conv(ir.NewIndexExpr(base.Pos, ncs, ir.NewInt(base.Pos, int64(i+offset))), convType)
					b = ir.NewBinaryExpr(base.Pos, ir.OLSH, b, ir.NewInt(base.Pos, int64(8*offset)))
					ncsubstr = ir.NewBinaryExpr(base.Pos, ir.OOR, ncsubstr, b)
					csubstr |= int64(s[i+offset]) << uint8(8*offset)
				}
				csubstrPart := ir.NewInt(base.Pos, csubstr)
				// Compare "step" bytes as once
				r = ir.NewLogicalExpr(base.Pos, and, r, ir.NewBinaryExpr(base.Pos, cmp, csubstrPart, ncsubstr))
				remains -= step
				i += step
			}
			return finishCompare(n, r, init)
		}
	}

	var r ir.Node
	if n.Op() == ir.OEQ || n.Op() == ir.ONE {
		// prepare for rewrite below
		n.X = cheapExpr(n.X, init)
		n.Y = cheapExpr(n.Y, init)
		eqlen, eqmem := compare.EqString(n.X, n.Y)
		// quick check of len before full compare for == or !=.
		// memequal then tests equality up to length len.
		if n.Op() == ir.OEQ {
			// len(left) == len(right) && memequal(left, right, len)
			r = ir.NewLogicalExpr(base.Pos, ir.OANDAND, eqlen, eqmem)
		} else {
			// len(left) != len(right) || !memequal(left, right, len)
			eqlen.SetOp(ir.ONE)
			r = ir.NewLogicalExpr(base.Pos, ir.OOROR, eqlen, ir.NewUnaryExpr(base.Pos, ir.ONOT, eqmem))
		}
	} else {
		// sys_cmpstring(s1, s2) :: 0
		r = mkcall("cmpstring", types.Types[types.TINT], init, typecheck.Conv(n.X, types.Types[types.TSTRING]), typecheck.Conv(n.Y, types.Types[types.TSTRING]))
		r = ir.NewBinaryExpr(base.Pos, n.Op(), r, ir.NewInt(base.Pos, 0))
	}

	return finishCompare(n, r, init)
}

// The result of finishCompare MUST be assigned back to n, e.g.
//
//	n.Left = finishCompare(n.Left, x, r, init)
func finishCompare(n *ir.BinaryExpr, r ir.Node, init *ir.Nodes) ir.Node {
	r = typecheck.Expr(r)
	r = typecheck.Conv(r, n.Type())
	r = walkExpr(r, init)
	return r
}

// brcom returns !(op).
// For example, brcom(==) is !=.
func brcom(op ir.Op) ir.Op {
	switch op {
	case ir.OEQ:
		return ir.ONE
	case ir.ONE:
		return ir.OEQ
	case ir.OLT:
		return ir.OGE
	case ir.OGT:
		return ir.OLE
	case ir.OLE:
		return ir.OGT
	case ir.OGE:
		return ir.OLT
	}
	base.Fatalf("brcom: no com for %v\n", op)
	return op
}

// brrev returns reverse(op).
// For example, Brrev(<) is >.
func brrev(op ir.Op) ir.Op {
	switch op {
	case ir.OEQ:
		return ir.OEQ
	case ir.ONE:
		return ir.ONE
	case ir.OLT:
		return ir.OGT
	case ir.OGT:
		return ir.OLT
	case ir.OLE:
		return ir.OGE
	case ir.OGE:
		return ir.OLE
	}
	base.Fatalf("brrev: no rev for %v\n", op)
	return op
}

func tracecmpArg(n ir.Node, t *types.Type, init *ir.Nodes) ir.Node {
	// Ugly hack to avoid "constant -1 overflows uintptr" errors, etc.
	if n.Op() == ir.OLITERAL && n.Type().IsSigned() && ir.Int64Val(n) < 0 {
		n = copyExpr(n, n.Type(), init)
	}

	return typecheck.Conv(n, t)
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/complit.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/staticdata"
	"cmd/compile/internal/staticinit"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
)

// walkCompLit walks a composite literal node:
// OARRAYLIT, OSLICELIT, OMAPLIT, OSTRUCTLIT (all CompLitExpr), or OPTRLIT (AddrExpr).
func walkCompLit(n ir.Node, init *ir.Nodes) ir.Node {
	if isStaticCompositeLiteral(n) && !ssa.CanSSA(n.Type()) {
		n := n.(*ir.CompLitExpr) // not OPTRLIT
		// n can be directly represented in the read-only data section.
		// Make direct reference to the static data. See issue 12841.
		vstat := readonlystaticname(n.Type())
		fixedlit(initKindStatic, n, vstat, init)
		return typecheck.Expr(vstat)
	}
	var_ := typecheck.TempAt(base.Pos, ir.CurFunc, n.Type())
	anylit(n, var_, init)
	return var_
}

// readonlystaticname returns a name backed by a read-only static data symbol.
func readonlystaticname(t *types.Type) *ir.Name {
	n := staticinit.StaticName(t)
	n.MarkReadonly()
	n.Linksym().Set(obj.AttrContentAddressable, true)
	n.Linksym().Set(obj.AttrLocal, true)
	return n
}

func isSimpleName(nn ir.Node) bool {
	if nn.Op() != ir.ONAME || ir.IsBlank(nn) {
		return false
	}
	n := nn.(*ir.Name)
	return n.OnStack()
}

// initGenType is a bitmap indicating the types of generation that will occur for a static value.
type initGenType uint8

const (
	initDynamic initGenType = 1 << iota // contains some dynamic values, for which init code will be generated
	initConst                           // contains some constant values, which may be written into data symbols
)

// getdyn calculates the initGenType for n.
// If top is false, getdyn is recursing.
func getdyn(n ir.Node, top bool) initGenType {
	switch n.Op() {
	default:
		if isStaticLiteral(n) {
			return initConst
		}
		return initDynamic

	case ir.OSLICELIT:
		n := n.(*ir.CompLitExpr)
		if !top {
			return initDynamic
		}
		if n.Len/4 > int64(len(n.List)) {
			// <25% of entries have explicit values.
			// Very rough estimation, it takes 4 bytes of instructions
			// to initialize 1 byte of result. So don't use a static
			// initializer if the dynamic initialization code would be
			// smaller than the static value.
			// See issue 23780.
			return initDynamic
		}

	case ir.OARRAYLIT, ir.OSTRUCTLIT:
	}
	lit := n.(*ir.CompLitExpr)

	var mode initGenType
	for _, n1 := range lit.List {
		switch n1.Op() {
		case ir.OKEY:
			n1 = n1.(*ir.KeyExpr).Value
		case ir.OSTRUCTKEY:
			n1 = n1.(*ir.StructKeyExpr).Value
		}
		mode |= getdyn(n1, false)
		if mode == initDynamic|initConst {
			break
		}
	}
	return mode
}

// isStaticLiteral reports whether n is a compile-time (non-composite)
// constant, which can be represented in the read-only data section.
func isStaticLiteral(n ir.Node) bool {
	// A string reference requires a relocation, not allowed
	// in static data in FIPS mode.
	return ir.IsConstNode(n) && !(base.Ctxt.IsFIPS() && n.Type().IsString())
}

// isStaticCompositeLiteral reports whether a composite literal n
// is a compile-time constant, which can be represented in the
// read-only data section.
func isStaticCompositeLiteral(n ir.Node) bool {
	switch n.Op() {
	case ir.OSLICELIT:
		return false
	case ir.OARRAYLIT:
		n := n.(*ir.CompLitExpr)
		for _, r := range n.List {
			if r.Op() == ir.OKEY {
				r = r.(*ir.KeyExpr).Value
			}
			if !isStaticCompositeLiteral(r) {
				return false
			}
		}
		return true
	case ir.OSTRUCTLIT:
		n := n.(*ir.CompLitExpr)
		for _, r := range n.List {
			r := r.(*ir.StructKeyExpr)
			if !isStaticCompositeLiteral(r.Value) {
				return false
			}
		}
		return true
	case ir.ONIL:
		return true
	case ir.OLITERAL:
		return isStaticLiteral(n)
	case ir.OCONVIFACE:
		// See staticinit.Schedule.StaticAssign's OCONVIFACE case for comments.
		if base.Ctxt.IsFIPS() && base.Ctxt.Flag_shared {
			return false
		}
		n := n.(*ir.ConvExpr)
		val := ir.Node(n)
		for val.Op() == ir.OCONVIFACE {
			val = val.(*ir.ConvExpr).X
		}
		if val.Type().IsInterface() {
			return val.Op() == ir.ONIL
		}
		if types.IsDirectIface(val.Type()) && val.Op() == ir.ONIL {
			return true
		}
		return isStaticCompositeLiteral(val)
	}
	return false
}

// initKind is a kind of static initialization: static, dynamic, or local.
// Static initialization represents literals and
// literal components of composite literals.
// Dynamic initialization represents non-literals and
// non-literal components of composite literals.
// LocalCode initialization represents initialization
// that occurs purely in generated code local to the function of use.
// Initialization code is sometimes generated in passes,
// first static then dynamic.
type initKind uint8

const (
	initKindStatic initKind = iota + 1
	initKindDynamic
	initKindLocalCode
)

// fixedlit handles struct, array, and slice literals.
// TODO: expand documentation.
func fixedlit(kind initKind, n *ir.CompLitExpr, var_ ir.Node, init *ir.Nodes) {
	isBlank := var_ == ir.BlankNode
	var splitnode func(ir.Node) (a ir.Node, value ir.Node)
	switch n.Op() {
	case ir.OARRAYLIT, ir.OSLICELIT:
		var k int64
		splitnode = func(r ir.Node) (ir.Node, ir.Node) {
			if r.Op() == ir.OKEY {
				kv := r.(*ir.KeyExpr)
				k = typecheck.IndexConst(kv.Key)
				r = kv.Value
			}
			a := ir.NewIndexExpr(base.Pos, var_, ir.NewInt(base.Pos, k))
			k++
			if isBlank {
				return ir.BlankNode, r
			}
			return a, r
		}
	case ir.OSTRUCTLIT:
		splitnode = func(rn ir.Node) (ir.Node, ir.Node) {
			r := rn.(*ir.StructKeyExpr)
			if r.Sym().IsBlank() || isBlank {
				return ir.BlankNode, r.Value
			}
			ir.SetPos(r)
			return ir.NewSelectorExpr(base.Pos, ir.OXDOT, var_, r.Sym()), r.Value
		}
	default:
		base.Fatalf("fixedlit bad op: %v", n.Op())
	}

	for _, r := range n.List {
		a, value := splitnode(r)
		if a == ir.BlankNode && !staticinit.AnySideEffects(value) {
			// Discard.
			continue
		}

		switch value.Op() {
		case ir.OSLICELIT:
			value := value.(*ir.CompLitExpr)
			if kind == initKindDynamic {
				slicelit(value, a, init)
				continue
			}

		case ir.OARRAYLIT, ir.OSTRUCTLIT:
			value := value.(*ir.CompLitExpr)
			fixedlit(kind, value, a, init)
			continue
		}

		islit := isStaticLiteral(value)
		if (kind == initKindStatic && !islit) || (kind == initKindDynamic && islit) {
			continue
		}

		// build list of assignments: var[index] = expr
		ir.SetPos(a)
		as := ir.NewAssignStmt(base.Pos, a, value)
		as = typecheck.Stmt(as).(*ir.AssignStmt)
		switch kind {
		case initKindStatic:
			genAsStatic(as)
		case initKindDynamic, initKindLocalCode:
			appendWalkStmt(init, orderStmtInPlace(as, map[string][]*ir.Name{}))
		default:
			base.Fatalf("fixedlit: bad kind %d", kind)
		}

	}
}

func isSmallSliceLit(n *ir.CompLitExpr) bool {
	if n.Op() != ir.OSLICELIT {
		return false
	}

	return n.Type().Elem().Size() == 0 || n.Len <= ir.MaxSmallArraySize/n.Type().Elem().Size()
}

func slicelit(n *ir.CompLitExpr, var_ ir.Node, init *ir.Nodes) {
	// make an array type corresponding the number of elements we have
	t := types.NewArray(n.Type().Elem(), n.Len)
	types.CalcSize(t)

	// recipe for var = []t{...}
	// 1. make a static array
	//	var vstat [...]t
	// 2. assign (data statements) the constant part
	//	vstat = constpart{}
	// 3. make an auto pointer to array and allocate heap to it
	//	var vauto *[...]t = new([...]t)
	// 4. copy the static array to the auto array
	//	*vauto = vstat
	// 5. for each dynamic part assign to the array
	//	vauto[i] = dynamic part
	// 6. assign slice of allocated heap to var
	//	var = vauto[:]
	//
	// an optimization is done if there is no constant part
	//	3. var vauto *[...]t = new([...]t)
	//	5. vauto[i] = dynamic part
	//	6. var = vauto[:]

	// if the literal contains constants,
	// make static initialized array (1),(2)
	var vstat ir.Node

	mode := getdyn(n, true)
	if mode&initConst != 0 && !isSmallSliceLit(n) {
		vstat = readonlystaticname(t)
		fixedlit(initKindStatic, n, vstat, init)
	}

	// make new auto *array (3 declare)
	vauto := typecheck.TempAt(base.Pos, ir.CurFunc, types.NewPtr(t))

	// set auto to point at new temp or heap (3 assign)
	var a ir.Node
	if x := n.Prealloc; x != nil {
		// temp allocated during order.go for dddarg
		if !types.Identical(t, x.Type()) {
			panic("dotdotdot base type does not match order's assigned type")
		}
		a = initStackTemp(init, x, vstat)
	} else if n.Esc() == ir.EscNone {
		a = initStackTemp(init, typecheck.TempAt(base.Pos, ir.CurFunc, t), vstat)
	} else {
		a = ir.NewUnaryExpr(base.Pos, ir.ONEW, ir.TypeNode(t))
	}
	appendWalkStmt(init, ir.NewAssignStmt(base.Pos, vauto, a))

	if vstat != nil && n.Prealloc == nil && n.Esc() != ir.EscNone {
		// If we allocated on the heap with ONEW, copy the static to the
		// heap (4). We skip this for stack temporaries, because
		// initStackTemp already handled the copy.
		a = ir.NewStarExpr(base.Pos, vauto)
		appendWalkStmt(init, ir.NewAssignStmt(base.Pos, a, vstat))
	}

	// put dynamics into array (5)
	var index int64
	for _, value := range n.List {
		if value.Op() == ir.OKEY {
			kv := value.(*ir.KeyExpr)
			index = typecheck.IndexConst(kv.Key)
			value = kv.Value
		}
		a := ir.NewIndexExpr(base.Pos, vauto, ir.NewInt(base.Pos, index))
		a.SetBounded(true)
		index++

		// TODO need to check bounds?

		switch value.Op() {
		case ir.OSLICELIT:
			break

		case ir.OARRAYLIT, ir.OSTRUCTLIT:
			value := value.(*ir.CompLitExpr)
			k := initKindDynamic
			if vstat == nil {
				// Generate both static and dynamic initializations.
				// See issue #31987.
				k = initKindLocalCode
			}
			fixedlit(k, value, a, init)
			continue
		}

		if vstat != nil && isStaticLiteral(value) { // already set by copy from static value
			continue
		}

		// build list of vauto[c] = expr
		ir.SetPos(value)
		as := ir.NewAssignStmt(base.Pos, a, value)
		appendWalkStmt(init, orderStmtInPlace(typecheck.Stmt(as), map[string][]*ir.Name{}))
	}

	// make slice out of heap (6)
	a = ir.NewAssignStmt(base.Pos, var_, ir.NewSliceExpr(base.Pos, ir.OSLICE, vauto, nil, nil, nil))
	appendWalkStmt(init, orderStmtInPlace(typecheck.Stmt(a), map[string][]*ir.Name{}))
}

func maplit(n *ir.CompLitExpr, m ir.Node, init *ir.Nodes) {
	// make the map var
	args := []ir.Node{ir.TypeNode(n.Type()), ir.NewInt(base.Pos, n.Len+int64(len(n.List)))}
	a := typecheck.Expr(ir.NewCallExpr(base.Pos, ir.OMAKE, nil, args)).(*ir.MakeExpr)
	a.RType = n.RType
	a.SetEsc(n.Esc())
	appendWalkStmt(init, ir.NewAssignStmt(base.Pos, m, a))

	entries := n.List

	// The order pass already removed any dynamic (runtime-computed) entries.
	// All remaining entries are static. Double-check that.
	for _, r := range entries {
		r := r.(*ir.KeyExpr)
		if !isStaticCompositeLiteral(r.Key) || !isStaticCompositeLiteral(r.Value) {
			base.Fatalf("maplit: entry is not a literal: %v", r)
		}
	}

	if len(entries) > 25 {
		// For a large number of entries, put them in an array and loop.

		// build types [count]Tindex and [count]Tvalue
		tk := types.NewArray(n.Type().Key(), int64(len(entries)))
		te := types.NewArray(n.Type().Elem(), int64(len(entries)))

		// TODO(#47904): mark tk and te NoAlg here once the
		// compiler/linker can handle NoAlg types correctly.

		types.CalcSize(tk)
		types.CalcSize(te)

		// make and initialize static arrays
		vstatk := readonlystaticname(tk)
		vstate := readonlystaticname(te)

		datak := ir.NewCompLitExpr(base.Pos, ir.OARRAYLIT, nil, nil)
		datae := ir.NewCompLitExpr(base.Pos, ir.OARRAYLIT, nil, nil)
		for _, r := range entries {
			r := r.(*ir.KeyExpr)
			datak.List.Append(r.Key)
			datae.List.Append(r.Value)
		}
		fixedlit(initKindStatic, datak, vstatk, init)
		fixedlit(initKindStatic, datae, vstate, init)

		// loop adding structure elements to map
		// for i = 0; i < len(vstatk); i++ {
		//	map[vstatk[i]] = vstate[i]
		// }
		i := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])
		rhs := ir.NewIndexExpr(base.Pos, vstate, i)
		rhs.SetBounded(true)

		kidx := ir.NewIndexExpr(base.Pos, vstatk, i)
		kidx.SetBounded(true)

		// typechecker rewrites OINDEX to OINDEXMAP
		lhs := typecheck.AssignExpr(ir.NewIndexExpr(base.Pos, m, kidx)).(*ir.IndexExpr)
		base.AssertfAt(lhs.Op() == ir.OINDEXMAP, lhs.Pos(), "want OINDEXMAP, have %+v", lhs)
		lhs.RType = n.RType

		zero := ir.NewAssignStmt(base.Pos, i, ir.NewInt(base.Pos, 0))
		cond := ir.NewBinaryExpr(base.Pos, ir.OLT, i, ir.NewInt(base.Pos, tk.NumElem()))
		incr := ir.NewAssignStmt(base.Pos, i, ir.NewBinaryExpr(base.Pos, ir.OADD, i, ir.NewInt(base.Pos, 1)))

		var body ir.Node = ir.NewAssignStmt(base.Pos, lhs, rhs)
		body = typecheck.Stmt(body)
		body = orderStmtInPlace(body, map[string][]*ir.Name{})

		loop := ir.NewForStmt(base.Pos, nil, cond, incr, nil, false)
		loop.Body = []ir.Node{body}
		loop.SetInit([]ir.Node{zero})

		appendWalkStmt(init, loop)
		return
	}
	// For a small number of entries, just add them directly.

	// Build list of var[c] = expr.
	// Use temporaries so that mapassign1 can have addressable key, elem.
	// TODO(josharian): avoid map key temporaries for mapfast_* assignments with literal keys.
	// TODO(khr): assign these temps in order phase so we can reuse them across multiple maplits?
	tmpkey := typecheck.TempAt(base.Pos, ir.CurFunc, m.Type().Key())
	tmpelem := typecheck.TempAt(base.Pos, ir.CurFunc, m.Type().Elem())

	for _, r := range entries {
		r := r.(*ir.KeyExpr)
		index, elem := r.Key, r.Value

		ir.SetPos(index)
		appendWalkStmt(init, ir.NewAssignStmt(base.Pos, tmpkey, index))

		ir.SetPos(elem)
		appendWalkStmt(init, ir.NewAssignStmt(base.Pos, tmpelem, elem))

		ir.SetPos(tmpelem)

		// typechecker rewrites OINDEX to OINDEXMAP
		lhs := typecheck.AssignExpr(ir.NewIndexExpr(base.Pos, m, tmpkey)).(*ir.IndexExpr)
		base.AssertfAt(lhs.Op() == ir.OINDEXMAP, lhs.Pos(), "want OINDEXMAP, have %+v", lhs)
		lhs.RType = n.RType

		var a ir.Node = ir.NewAssignStmt(base.Pos, lhs, tmpelem)
		a = typecheck.Stmt(a)
		a = orderStmtInPlace(a, map[string][]*ir.Name{})
		appendWalkStmt(init, a)
	}
}

func anylit(n ir.Node, var_ ir.Node, init *ir.Nodes) {
	t := n.Type()
	switch n.Op() {
	default:
		base.Fatalf("anylit: not lit, op=%v node=%v", n.Op(), n)

	case ir.ONAME:
		n := n.(*ir.Name)
		appendWalkStmt(init, ir.NewAssignStmt(base.Pos, var_, n))

	case ir.OMETHEXPR:
		n := n.(*ir.SelectorExpr)
		anylit(n.FuncName(), var_, init)

	case ir.OPTRLIT:
		n := n.(*ir.AddrExpr)
		if !t.IsPtr() {
			base.Fatalf("anylit: not ptr")
		}

		var r ir.Node
		if n.Prealloc != nil {
			// n.Prealloc is stack temporary used as backing store.
			r = initStackTemp(init, n.Prealloc, nil)
		} else {
			r = ir.NewUnaryExpr(base.Pos, ir.ONEW, ir.TypeNode(n.X.Type()))
			r.SetEsc(n.Esc())
		}
		appendWalkStmt(init, ir.NewAssignStmt(base.Pos, var_, r))

		var_ = ir.NewStarExpr(base.Pos, var_)
		var_ = typecheck.AssignExpr(var_)
		anylit(n.X, var_, init)

	case ir.OSTRUCTLIT, ir.OARRAYLIT:
		n := n.(*ir.CompLitExpr)
		if !t.IsStruct() && !t.IsArray() {
			base.Fatalf("anylit: not struct/array")
		}

		if isSimpleName(var_) && len(n.List) > 4 {
			// lay out static data
			vstat := readonlystaticname(t)

			fixedlit(initKindStatic, n, vstat, init)

			// copy static to var
			appendWalkStmt(init, ir.NewAssignStmt(base.Pos, var_, vstat))

			// add expressions to automatic
			fixedlit(initKindDynamic, n, var_, init)
			break
		}

		var components int64
		if n.Op() == ir.OARRAYLIT {
			components = t.NumElem()
		} else {
			components = int64(t.NumFields())
		}
		// initialization of an array or struct with unspecified components (missing fields or arrays)
		if isSimpleName(var_) || int64(len(n.List)) < components {
			appendWalkStmt(init, ir.NewAssignStmt(base.Pos, var_, nil))
		}

		fixedlit(initKindLocalCode, n, var_, init)

	case ir.OSLICELIT:
		n := n.(*ir.CompLitExpr)
		slicelit(n, var_, init)

	case ir.OMAPLIT:
		n := n.(*ir.CompLitExpr)
		if !t.IsMap() {
			base.Fatalf("anylit: not map")
		}
		maplit(n, var_, init)
	}
}

// oaslit handles special composite literal assignments.
// It returns true if n's effects have been added to init,
// in which case n should be dropped from the program by the caller.
func oaslit(n *ir.AssignStmt, init *ir.Nodes) bool {
	if n.X == nil || n.Y == nil {
		// not a special composite literal assignment
		return false
	}
	if n.X.Type() == nil || n.Y.Type() == nil {
		// not a special composite literal assignment
		return false
	}
	if !isSimpleName(n.X) {
		// not a special composite literal assignment
		return false
	}
	x := n.X.(*ir.Name)
	if !types.Identical(n.X.Type(), n.Y.Type()) {
		// not a special composite literal assignment
		return false
	}
	if x.Addrtaken() {
		// If x is address-taken, the RHS may (implicitly) uses LHS.
		// Not safe to do a special composite literal assignment
		// (which may expand to multiple assignments).
		return false
	}

	switch n.Y.Op() {
	default:
		// not a special composite literal assignment
		return false

	case ir.OSTRUCTLIT, ir.OARRAYLIT, ir.OSLICELIT, ir.OMAPLIT:
		if ir.Any(n.Y, func(y ir.Node) bool { return ir.Uses(y, x) }) {
			// not safe to do a special composite literal assignment if RHS uses LHS.
			return false
		}
		anylit(n.Y, n.X, init)
	}

	return true
}

func genAsStatic(as *ir.AssignStmt) {
	if as.X.Type() == nil {
		base.Fatalf("genAsStatic as.Left not typechecked")
	}

	name, offset, ok := staticinit.StaticLoc(as.X)
	if !ok || (name.Class != ir.PEXTERN && as.X != ir.BlankNode) {
		base.Fatalf("genAsStatic: lhs %v", as.X)
	}

	switch r := as.Y; r.Op() {
	case ir.OLITERAL:
		staticdata.InitConst(name, offset, r, int(r.Type().Size()))
		return
	case ir.OMETHEXPR:
		r := r.(*ir.SelectorExpr)
		staticdata.InitAddr(name, offset, staticdata.FuncLinksym(r.FuncName()))
		return
	case ir.ONAME:
		r := r.(*ir.Name)
		if r.Offset_ != 0 {
			base.Fatalf("genAsStatic %+v", as)
		}
		if r.Class == ir.PFUNC {
			staticdata.InitAddr(name, offset, staticdata.FuncLinksym(r))
			return
		}
	}
	base.Fatalf("genAsStatic: rhs %v", as.Y)
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/convert.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"encoding/binary"
	"go/constant"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/ssagen"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/sys"
)

// walkConv walks an OCONV or OCONVNOP (but not OCONVIFACE) node.
func walkConv(n *ir.ConvExpr, init *ir.Nodes) ir.Node {
	n.X = walkExpr(n.X, init)
	if n.Op() == ir.OCONVNOP && n.Type() == n.X.Type() {
		return n.X
	}
	if n.Op() == ir.OCONVNOP && ir.ShouldCheckPtr(ir.CurFunc, 1) {
		if n.Type().IsUnsafePtr() && n.X.Type().IsUintptr() { // uintptr to unsafe.Pointer
			return walkCheckPtrArithmetic(n, init)
		}
	}
	param, result := rtconvfn(n.X.Type(), n.Type())
	if param == types.Txxx {
		return n
	}
	fn := types.BasicTypeNames[param] + "to" + types.BasicTypeNames[result]
	return typecheck.Conv(mkcall(fn, types.Types[result], init, typecheck.Conv(n.X, types.Types[param])), n.Type())
}

// walkConvInterface walks an OCONVIFACE node.
func walkConvInterface(n *ir.ConvExpr, init *ir.Nodes) ir.Node {

	n.X = walkExpr(n.X, init)

	fromType := n.X.Type()
	toType := n.Type()
	if !fromType.IsInterface() && !ir.IsBlank(ir.CurFunc.Nname) {
		// skip unnamed functions (func _())
		if fromType.HasShape() {
			// Unified IR uses OCONVIFACE for converting all derived types
			// to interface type. Avoid assertion failure in
			// MarkTypeUsedInInterface, because we've marked used types
			// separately anyway.
		} else {
			reflectdata.MarkTypeUsedInInterface(fromType, ir.CurFunc.LSym)
		}
	}

	if !fromType.IsInterface() {
		typeWord := reflectdata.ConvIfaceTypeWord(base.Pos, n)
		l := ir.NewBinaryExpr(base.Pos, ir.OMAKEFACE, typeWord, dataWord(n, init))
		l.SetType(toType)
		l.SetTypecheck(n.Typecheck())
		return l
	}
	if fromType.IsEmptyInterface() {
		base.Fatalf("OCONVIFACE can't operate on an empty interface")
	}

	// Evaluate the input interface.
	c := typecheck.TempAt(base.Pos, ir.CurFunc, fromType)
	init.Append(ir.NewAssignStmt(base.Pos, c, n.X))

	if toType.IsEmptyInterface() {
		// Implement interface to empty interface conversion:
		//
		// var res *uint8
		// res = (*uint8)(unsafe.Pointer(itab))
		// if res != nil {
		//    res = res.type
		// }

		// Grab its parts.
		itab := ir.NewUnaryExpr(base.Pos, ir.OITAB, c)
		itab.SetType(types.Types[types.TUINTPTR].PtrTo())
		itab.SetTypecheck(1)
		data := ir.NewUnaryExpr(n.Pos(), ir.OIDATA, c)
		data.SetType(types.Types[types.TUINT8].PtrTo()) // Type is generic pointer - we're just passing it through.
		data.SetTypecheck(1)

		typeWord := typecheck.TempAt(base.Pos, ir.CurFunc, types.NewPtr(types.Types[types.TUINT8]))
		init.Append(ir.NewAssignStmt(base.Pos, typeWord, typecheck.Conv(typecheck.Conv(itab, types.Types[types.TUNSAFEPTR]), typeWord.Type())))
		nif := ir.NewIfStmt(base.Pos, typecheck.Expr(ir.NewBinaryExpr(base.Pos, ir.ONE, typeWord, typecheck.NodNil())), nil, nil)
		nif.Body = []ir.Node{ir.NewAssignStmt(base.Pos, typeWord, itabType(typeWord))}
		init.Append(nif)

		// Build the result.
		// e = iface{typeWord, data}
		e := ir.NewBinaryExpr(base.Pos, ir.OMAKEFACE, typeWord, data)
		e.SetType(toType) // assign type manually, typecheck doesn't understand OEFACE.
		e.SetTypecheck(1)
		return e
	}

	// Must be converting I2I (more specific to less specific interface).
	// Use the same code as e, _ = c.(T).
	var rhs ir.Node
	if n.TypeWord == nil || n.TypeWord.Op() == ir.OADDR && n.TypeWord.(*ir.AddrExpr).X.Op() == ir.OLINKSYMOFFSET {
		// Fixed (not loaded from a dictionary) type.
		ta := ir.NewTypeAssertExpr(base.Pos, c, toType)
		ta.SetOp(ir.ODOTTYPE2)
		// Allocate a descriptor for this conversion to pass to the runtime.
		ta.Descriptor = makeTypeAssertDescriptor(toType, true)
		rhs = ta
	} else {
		ta := ir.NewDynamicTypeAssertExpr(base.Pos, ir.ODYNAMICDOTTYPE2, c, n.TypeWord)
		rhs = ta
	}
	rhs.SetType(toType)
	rhs.SetTypecheck(1)

	res := typecheck.TempAt(base.Pos, ir.CurFunc, toType)
	as := ir.NewAssignListStmt(base.Pos, ir.OAS2DOTTYPE, []ir.Node{res, ir.BlankNode}, []ir.Node{rhs})
	init.Append(as)
	return res
}

// Returns the data word (the second word) used to represent conv.X in
// an interface.
func dataWord(conv *ir.ConvExpr, init *ir.Nodes) ir.Node {
	pos, n := conv.Pos(), conv.X
	fromType := n.Type()

	// If it's a pointer, it is its own representation.
	if types.IsDirectIface(fromType) {
		return n
	}

	isInteger := fromType.IsInteger()
	isBool := fromType.IsBoolean()
	if sc := fromType.SoleComponent(); sc != nil {
		isInteger = sc.IsInteger()
		isBool = sc.IsBoolean()
	}

	diagnose := func(msg string, n ir.Node) {
		if base.Debug.EscapeDebug > 0 {
			// This output is most useful with -gcflags=-W=2 or similar because
			// it often prints a temp variable name.
			base.WarnfAt(n.Pos(), "convert: %s: %v", msg, n)
		}
	}

	// Try a bunch of cases to avoid an allocation.
	var value ir.Node
	switch {
	case fromType.Size() == 0:
		// n is zero-sized. Use zerobase.
		diagnose("using global for zero-sized interface value", n)
		cheapExpr(n, init) // Evaluate n for side-effects. See issue 19246.
		value = ir.NewLinksymExpr(base.Pos, ir.Syms.Zerobase, types.Types[types.TUINTPTR])
	case isBool || fromType.Size() == 1 && isInteger:
		// n is a bool/byte. Use staticuint64s[n * 8] on little-endian
		// and staticuint64s[n * 8 + 7] on big-endian.
		diagnose("using global for single-byte interface value", n)
		n = cheapExpr(n, init)
		n = soleComponent(init, n)
		// byteindex widens n so that the multiplication doesn't overflow.
		index := ir.NewBinaryExpr(base.Pos, ir.OLSH, byteindex(n), ir.NewInt(base.Pos, 3))
		if ssagen.Arch.LinkArch.ByteOrder == binary.BigEndian {
			index = ir.NewBinaryExpr(base.Pos, ir.OADD, index, ir.NewInt(base.Pos, 7))
		}
		// The actual type is [256]uint64, but we use [256*8]uint8 so we can address
		// individual bytes.
		staticuint64s := ir.NewLinksymExpr(base.Pos, ir.Syms.Staticuint64s, types.NewArray(types.Types[types.TUINT8], 256*8))
		xe := ir.NewIndexExpr(base.Pos, staticuint64s, index)
		xe.SetBounded(true)
		value = xe
	case n.Op() == ir.OLINKSYMOFFSET && n.(*ir.LinksymOffsetExpr).Linksym == ir.Syms.ZeroVal && n.(*ir.LinksymOffsetExpr).Offset_ == 0:
		// n is using zeroVal, so we can use n directly.
		// (Note that n does not have a proper pos in this case, so using conv for the diagnostic instead.)
		diagnose("using global for zero value interface value", conv)
		value = n
	case n.Op() == ir.ONAME && n.(*ir.Name).Class == ir.PEXTERN && n.(*ir.Name).Readonly():
		// n is a readonly global; use it directly.
		diagnose("using global for interface value", n)
		value = n
	case conv.Esc() == ir.EscNone && fromType.Size() <= 1024:
		// n does not escape. Use a stack temporary initialized to n.
		diagnose("using stack temporary for interface value", n)
		value = typecheck.TempAt(base.Pos, ir.CurFunc, fromType)
		init.Append(typecheck.Stmt(ir.NewAssignStmt(base.Pos, value, n)))
	}
	if value != nil {
		// The interface data word is &value.
		return typecheck.Expr(typecheck.NodAddr(value))
	}

	// Time to do an allocation. We'll call into the runtime for that.
	fnname, argType, needsaddr := dataWordFuncName(fromType)
	var fn *ir.Name

	var args []ir.Node
	if needsaddr {
		// Types of large or unknown size are passed by reference.
		// Orderexpr arranged for n to be a temporary for all
		// the conversions it could see. Comparison of an interface
		// with a non-interface, especially in a switch on interface value
		// with non-interface cases, is not visible to order.stmt, so we
		// have to fall back on allocating a temp here.
		if !ir.IsAddressable(n) {
			n = copyExpr(n, fromType, init)
		}
		fn = typecheck.LookupRuntime(fnname, fromType)
		args = []ir.Node{reflectdata.ConvIfaceSrcRType(base.Pos, conv), typecheck.NodAddr(n)}
	} else {
		// Use a specialized conversion routine that takes the type being
		// converted by value, not by pointer.
		fn = typecheck.LookupRuntime(fnname)
		var arg ir.Node
		switch {
		case fromType == argType:
			// already in the right type, nothing to do
			arg = n
		case fromType.Kind() == argType.Kind(),
			fromType.IsPtrShaped() && argType.IsPtrShaped():
			// can directly convert (e.g. named type to underlying type, or one pointer to another)
			// TODO: never happens because pointers are directIface?
			arg = ir.NewConvExpr(pos, ir.OCONVNOP, argType, n)
		case fromType.IsInteger() && argType.IsInteger():
			// can directly convert (e.g. int32 to uint32)
			arg = ir.NewConvExpr(pos, ir.OCONV, argType, n)
		default:
			// unsafe cast through memory
			arg = copyExpr(n, fromType, init)
			var addr ir.Node = typecheck.NodAddr(arg)
			addr = ir.NewConvExpr(pos, ir.OCONVNOP, argType.PtrTo(), addr)
			arg = ir.NewStarExpr(pos, addr)
			arg.SetType(argType)
		}
		args = []ir.Node{arg}
	}
	call := ir.NewCallExpr(base.Pos, ir.OCALL, fn, nil)
	call.Args = args
	return safeExpr(walkExpr(typecheck.Expr(call), init), init)
}

// walkBytesRunesToString walks an OBYTES2STR or ORUNES2STR node.
func walkBytesRunesToString(n *ir.ConvExpr, init *ir.Nodes) ir.Node {
	a := typecheck.NodNil()
	if n.Esc() == ir.EscNone {
		// Create temporary buffer for string on stack.
		a = stackBufAddr(tmpstringbufsize, types.Types[types.TUINT8])
	}
	if n.Op() == ir.ORUNES2STR {
		// slicerunetostring(*[32]byte, []rune) string
		return mkcall("slicerunetostring", n.Type(), init, a, n.X)
	}
	// slicebytetostring(*[32]byte, ptr *byte, n int) string
	n.X = cheapExpr(n.X, init)
	ptr, len := backingArrayPtrLen(n.X)
	return mkcall("slicebytetostring", n.Type(), init, a, ptr, len)
}

// walkBytesToStringTemp walks an OBYTES2STRTMP node.
func walkBytesToStringTemp(n *ir.ConvExpr, init *ir.Nodes) ir.Node {
	n.X = walkExpr(n.X, init)
	if !base.Flag.Cfg.Instrumenting {
		// Let the backend handle OBYTES2STRTMP directly
		// to avoid a function call to slicebytetostringtmp.
		return n
	}
	// slicebytetostringtmp(ptr *byte, n int) string
	n.X = cheapExpr(n.X, init)
	ptr, len := backingArrayPtrLen(n.X)
	return mkcall("slicebytetostringtmp", n.Type(), init, ptr, len)
}

// walkRuneToString walks an ORUNESTR node.
func walkRuneToString(n *ir.ConvExpr, init *ir.Nodes) ir.Node {
	a := typecheck.NodNil()
	if n.Esc() == ir.EscNone {
		a = stackBufAddr(4, types.Types[types.TUINT8])
	}
	// intstring(*[4]byte, rune)
	return mkcall("intstring", n.Type(), init, a, typecheck.Conv(n.X, types.Types[types.TINT64]))
}

// walkStringToBytes walks an OSTR2BYTES node.
func walkStringToBytes(n *ir.ConvExpr, init *ir.Nodes) ir.Node {
	s := n.X

	if expr, ok := s.(*ir.AddStringExpr); ok {
		return walkAddString(expr, init, n)
	}

	if ir.IsConst(s, constant.String) {
		sc := ir.StringVal(s)

		// Allocate a [n]byte of the right size.
		t := types.NewArray(types.Types[types.TUINT8], int64(len(sc)))
		var a ir.Node
		if n.Esc() == ir.EscNone && len(sc) <= int(ir.MaxImplicitStackVarSize) {
			a = stackBufAddr(t.NumElem(), t.Elem())
		} else {
			types.CalcSize(t)
			a = ir.NewUnaryExpr(base.Pos, ir.ONEW, nil)
			a.SetType(types.NewPtr(t))
			a.SetTypecheck(1)
			a.MarkNonNil()
		}
		p := typecheck.TempAt(base.Pos, ir.CurFunc, t.PtrTo()) // *[n]byte
		init.Append(typecheck.Stmt(ir.NewAssignStmt(base.Pos, p, a)))

		// Copy from the static string data to the [n]byte.
		if len(sc) > 0 {
			sptr := ir.NewUnaryExpr(base.Pos, ir.OSPTR, s)
			sptr.SetBounded(true)
			as := ir.NewAssignStmt(base.Pos, ir.NewStarExpr(base.Pos, p), ir.NewStarExpr(base.Pos, typecheck.ConvNop(sptr, t.PtrTo())))
			appendWalkStmt(init, as)
		}

		// Slice the [n]byte to a []byte.
		slice := ir.NewSliceExpr(n.Pos(), ir.OSLICEARR, p, nil, nil, nil)
		slice.SetType(n.Type())
		slice.SetTypecheck(1)
		return walkExpr(slice, init)
	}

	a := typecheck.NodNil()
	if n.Esc() == ir.EscNone {
		// Create temporary buffer for slice on stack.
		a = stackBufAddr(tmpstringbufsize, types.Types[types.TUINT8])
	}
	// stringtoslicebyte(*32[byte], string) []byte
	return mkcall("stringtoslicebyte", n.Type(), init, a, typecheck.Conv(s, types.Types[types.TSTRING]))
}

// walkStringToBytesTemp walks an OSTR2BYTESTMP node.
func walkStringToBytesTemp(n *ir.ConvExpr, init *ir.Nodes) ir.Node {
	// []byte(string) conversion that creates a slice
	// referring to the actual string bytes.
	// This conversion is handled later by the backend and
	// is only for use by internal compiler optimizations
	// that know that the slice won't be mutated.
	// The only such case today is:
	// for i, c := range []byte(string)
	n.X = walkExpr(n.X, init)
	return n
}

// walkStringToRunes walks an OSTR2RUNES node.
func walkStringToRunes(n *ir.ConvExpr, init *ir.Nodes) ir.Node {
	a := typecheck.NodNil()
	if n.Esc() == ir.EscNone {
		// Create temporary buffer for slice on stack.
		a = stackBufAddr(tmpstringbufsize, types.Types[types.TINT32])
	}
	// stringtoslicerune(*[32]rune, string) []rune
	return mkcall("stringtoslicerune", n.Type(), init, a, typecheck.Conv(n.X, types.Types[types.TSTRING]))
}

// dataWordFuncName returns the name of the function used to convert a value of type "from"
// to the data word of an interface.
// argType is the type the argument needs to be coerced to.
// needsaddr reports whether the value should be passed (needaddr==false) or its address (needsaddr==true).
func dataWordFuncName(from *types.Type) (fnname string, argType *types.Type, needsaddr bool) {
	if from.IsInterface() {
		base.Fatalf("can only handle non-interfaces")
	}
	switch {
	case from.Size() == 2 && uint8(from.Alignment()) == 2:
		return "convT16", types.Types[types.TUINT16], false
	case from.Size() == 4 && uint8(from.Alignment()) == 4 && !from.HasPointers():
		return "convT32", types.Types[types.TUINT32], false
	case from.Size() == 8 && uint8(from.Alignment()) == uint8(types.Types[types.TUINT64].Alignment()) && !from.HasPointers():
		return "convT64", types.Types[types.TUINT64], false
	}
	if sc := from.SoleComponent(); sc != nil {
		switch {
		case sc.IsString():
			return "convTstring", types.Types[types.TSTRING], false
		case sc.IsSlice():
			return "convTslice", types.NewSlice(types.Types[types.TUINT8]), false // the element type doesn't matter
		}
	}

	if from.HasPointers() {
		return "convT", types.Types[types.TUNSAFEPTR], true
	}
	return "convTnoptr", types.Types[types.TUNSAFEPTR], true
}

// rtconvfn returns the parameter and result types that will be used by a
// runtime function to convert from type src to type dst. The runtime function
// name can be derived from the names of the returned types.
//
// If no such function is necessary, it returns (Txxx, Txxx).
func rtconvfn(src, dst *types.Type) (param, result types.Kind) {
	if ssagen.Arch.SoftFloat {
		return types.Txxx, types.Txxx
	}

	switch ssagen.Arch.LinkArch.Family {
	case sys.ARM, sys.MIPS:
		if src.IsFloat() {
			switch dst.Kind() {
			case types.TINT64, types.TUINT64:
				return types.TFLOAT64, dst.Kind()
			}
		}
		if dst.IsFloat() {
			switch src.Kind() {
			case types.TINT64, types.TUINT64:
				return src.Kind(), dst.Kind()
			}
		}

	case sys.I386:
		if src.IsFloat() {
			switch dst.Kind() {
			case types.TINT64, types.TUINT64:
				return types.TFLOAT64, dst.Kind()
			case types.TUINT32, types.TUINT, types.TUINTPTR:
				return types.TFLOAT64, types.TUINT32
			}
		}
		if dst.IsFloat() {
			switch src.Kind() {
			case types.TINT64, types.TUINT64:
				return src.Kind(), dst.Kind()
			case types.TUINT32, types.TUINT, types.TUINTPTR:
				return types.TUINT32, types.TFLOAT64
			}
		}
	}
	return types.Txxx, types.Txxx
}

func soleComponent(init *ir.Nodes, n ir.Node) ir.Node {
	if n.Type().SoleComponent() == nil {
		return n
	}
	// Keep in sync with cmd/compile/internal/types/type.go:Type.SoleComponent.
	for {
		switch {
		case n.Type().IsStruct():
			if n.Type().Field(0).Sym.IsBlank() {
				// Treat blank fields as the zero value as the Go language requires.
				n = typecheck.TempAt(base.Pos, ir.CurFunc, n.Type().Field(0).Type)
				appendWalkStmt(init, ir.NewAssignStmt(base.Pos, n, nil))
				continue
			}
			n = typecheck.DotField(n.Pos(), n, 0)
		case n.Type().IsArray():
			n = typecheck.Expr(ir.NewIndexExpr(n.Pos(), n, ir.NewInt(base.Pos, 0)))
		default:
			return n
		}
	}
}

// byteindex converts n, which is byte-sized, to an int used to index into an array.
// We cannot use conv, because we allow converting bool to int here,
// which is forbidden in user code.
func byteindex(n ir.Node) ir.Node {
	// We cannot convert from bool to int directly.
	// While converting from int8 to int is possible, it would yield
	// the wrong result for negative values.
	// Reinterpreting the value as an unsigned byte solves both cases.
	if !types.Identical(n.Type(), types.Types[types.TUINT8]) {
		n = ir.NewConvExpr(base.Pos, ir.OCONV, nil, n)
		n.SetType(types.Types[types.TUINT8])
		n.SetTypecheck(1)
	}
	n = ir.NewConvExpr(base.Pos, ir.OCONV, nil, n)
	n.SetType(types.Types[types.TINT])
	n.SetTypecheck(1)
	return n
}

func walkCheckPtrArithmetic(n *ir.ConvExpr, init *ir.Nodes) ir.Node {
	// Calling cheapExpr(n, init) below leads to a recursive call to
	// walkExpr, which leads us back here again. Use n.Checkptr to
	// prevent infinite loops.
	if n.CheckPtr() {
		return n
	}
	n.SetCheckPtr(true)
	defer n.SetCheckPtr(false)

	// TODO(mdempsky): Make stricter. We only need to exempt
	// reflect.Value.Pointer and reflect.Value.UnsafeAddr.
	switch n.X.Op() {
	case ir.OCALLMETH:
		base.FatalfAt(n.X.Pos(), "OCALLMETH missed by typecheck")
	case ir.OCALLFUNC, ir.OCALLINTER:
		return n
	}

	if n.X.Op() == ir.ODOTPTR && ir.IsReflectHeaderDataField(n.X) {
		return n
	}

	// Find original unsafe.Pointer operands involved in this
	// arithmetic expression.
	//
	// "It is valid both to add and to subtract offsets from a
	// pointer in this way. It is also valid to use &^ to round
	// pointers, usually for alignment."
	var originals []ir.Node
	var walk func(n ir.Node)
	walk = func(n ir.Node) {
		switch n.Op() {
		case ir.OADD:
			n := n.(*ir.BinaryExpr)
			walk(n.X)
			walk(n.Y)
		case ir.OSUB, ir.OANDNOT:
			n := n.(*ir.BinaryExpr)
			walk(n.X)
		case ir.OCONVNOP:
			n := n.(*ir.ConvExpr)
			if n.X.Type().IsUnsafePtr() {
				n.X = cheapExpr(n.X, init)
				originals = append(originals, typecheck.ConvNop(n.X, types.Types[types.TUNSAFEPTR]))
			}
		}
	}
	walk(n.X)

	cheap := cheapExpr(n, init)

	slice := typecheck.MakeDotArgs(base.Pos, types.NewSlice(types.Types[types.TUNSAFEPTR]), originals)
	slice.SetEsc(ir.EscNone)

	init.Append(mkcall("checkptrArithmetic", nil, init, typecheck.ConvNop(cheap, types.Types[types.TUNSAFEPTR]), slice))
	// TODO(khr): Mark backing store of slice as dead. This will allow us to reuse
	// the backing store for multiple calls to checkptrArithmetic.

	return cheap
}

// walkSliceToArray walks an OSLICE2ARR expression.
func walkSliceToArray(n *ir.ConvExpr, init *ir.Nodes) ir.Node {
	// Replace T(x) with *(*T)(x).
	conv := typecheck.Expr(ir.NewConvExpr(base.Pos, ir.OCONV, types.NewPtr(n.Type()), n.X)).(*ir.ConvExpr)
	deref := typecheck.Expr(ir.NewStarExpr(base.Pos, conv)).(*ir.StarExpr)

	// The OSLICE2ARRPTR conversion handles checking the slice length,
	// so the dereference can't fail.
	//
	// However, this is more than just an optimization: if T is a
	// zero-length array, then x (and thus (*T)(x)) can be nil, but T(x)
	// should *not* panic. So suppressing the nil check here is
	// necessary for correctness in that case.
	deref.SetBounded(true)

	return walkExpr(deref, init)
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/expr.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"fmt"
	"go/constant"
	"internal/abi"
	"internal/buildcfg"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/noder"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/rttype"
	"cmd/compile/internal/staticdata"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/objabi"
)

// The result of walkExpr MUST be assigned back to n, e.g.
//
//	n.Left = walkExpr(n.Left, init)
func walkExpr(n ir.Node, init *ir.Nodes) ir.Node {
	if n == nil {
		return n
	}

	if n, ok := n.(ir.InitNode); ok && init == n.PtrInit() {
		// not okay to use n->ninit when walking n,
		// because we might replace n with some other node
		// and would lose the init list.
		base.Fatalf("walkExpr init == &n->ninit")
	}

	if len(n.Init()) != 0 {
		walkStmtList(n.Init())
		init.Append(ir.TakeInit(n)...)
	}

	lno := ir.SetPos(n)

	if base.Flag.LowerW > 1 {
		ir.Dump("before walk expr", n)
	}

	if n.Typecheck() != 1 {
		base.Fatalf("missed typecheck: %+v", n)
	}

	if n.Type().IsUntyped() {
		base.Fatalf("expression has untyped type: %+v", n)
	}

	n = walkExpr1(n, init)

	// Eagerly compute sizes of all expressions for the back end.
	if typ := n.Type(); typ != nil && typ.Kind() != types.TBLANK && !typ.IsFuncArgStruct() {
		types.CheckSize(typ)
	}
	if n, ok := n.(*ir.Name); ok && n.Heapaddr != nil {
		types.CheckSize(n.Heapaddr.Type())
	}
	if ir.IsConst(n, constant.String) {
		// Emit string symbol now to avoid emitting
		// any concurrently during the backend.
		_ = staticdata.StringSym(n.Pos(), constant.StringVal(n.Val()))
	}

	if base.Flag.LowerW != 0 && n != nil {
		ir.Dump("after walk expr", n)
	}

	base.Pos = lno
	return n
}

func walkExpr1(n ir.Node, init *ir.Nodes) ir.Node {
	switch n.Op() {
	default:
		ir.Dump("walk", n)
		base.Fatalf("walkExpr: switch 1 unknown op %+v", n.Op())
		panic("unreachable")

	case ir.OGETG, ir.OGETCALLERSP:
		return n

	case ir.OTYPE, ir.ONAME, ir.OLITERAL, ir.ONIL, ir.OLINKSYMOFFSET:
		// TODO(mdempsky): Just return n; see discussion on CL 38655.
		// Perhaps refactor to use Node.mayBeShared for these instead.
		// If these return early, make sure to still call
		// StringSym for constant strings.
		return n

	case ir.OMETHEXPR:
		// TODO(mdempsky): Do this right after type checking.
		n := n.(*ir.SelectorExpr)
		return n.FuncName()

	case ir.OMIN, ir.OMAX:
		n := n.(*ir.CallExpr)
		return walkMinMax(n, init)

	case ir.ONOT, ir.ONEG, ir.OPLUS, ir.OBITNOT, ir.OREAL, ir.OIMAG, ir.OSPTR, ir.OITAB, ir.OIDATA:
		n := n.(*ir.UnaryExpr)
		n.X = walkExpr(n.X, init)
		return n

	case ir.ODOTMETH, ir.ODOTINTER:
		n := n.(*ir.SelectorExpr)
		n.X = walkExpr(n.X, init)
		return n

	case ir.OADDR:
		n := n.(*ir.AddrExpr)
		n.X = walkExpr(n.X, init)
		return n

	case ir.ODEREF:
		n := n.(*ir.StarExpr)
		n.X = walkExpr(n.X, init)
		return n

	case ir.OMAKEFACE, ir.OAND, ir.OANDNOT, ir.OSUB, ir.OMUL, ir.OADD, ir.OOR, ir.OXOR, ir.OLSH, ir.ORSH,
		ir.OUNSAFEADD:
		n := n.(*ir.BinaryExpr)
		n.X = walkExpr(n.X, init)
		n.Y = walkExpr(n.Y, init)
		if n.Op() == ir.OUNSAFEADD && ir.ShouldCheckPtr(ir.CurFunc, 1) {
			// For unsafe.Add(p, n), just walk "unsafe.Pointer(uintptr(p)+uintptr(n))"
			// for the side effects of validating unsafe.Pointer rules.
			x := typecheck.ConvNop(n.X, types.Types[types.TUINTPTR])
			y := typecheck.Conv(n.Y, types.Types[types.TUINTPTR])
			conv := typecheck.ConvNop(ir.NewBinaryExpr(n.Pos(), ir.OADD, x, y), types.Types[types.TUNSAFEPTR])
			walkExpr(conv, init)
		}
		return n

	case ir.OUNSAFESLICE:
		n := n.(*ir.BinaryExpr)
		return walkUnsafeSlice(n, init)

	case ir.OUNSAFESTRING:
		n := n.(*ir.BinaryExpr)
		return walkUnsafeString(n, init)

	case ir.OUNSAFESTRINGDATA, ir.OUNSAFESLICEDATA:
		n := n.(*ir.UnaryExpr)
		return walkUnsafeData(n, init)

	case ir.ODOT, ir.ODOTPTR:
		n := n.(*ir.SelectorExpr)
		return walkDot(n, init)

	case ir.ODOTTYPE, ir.ODOTTYPE2:
		n := n.(*ir.TypeAssertExpr)
		return walkDotType(n, init)

	case ir.ODYNAMICDOTTYPE, ir.ODYNAMICDOTTYPE2:
		n := n.(*ir.DynamicTypeAssertExpr)
		return walkDynamicDotType(n, init)

	case ir.OLEN, ir.OCAP:
		n := n.(*ir.UnaryExpr)
		return walkLenCap(n, init)

	case ir.OCOMPLEX:
		n := n.(*ir.BinaryExpr)
		n.X = walkExpr(n.X, init)
		n.Y = walkExpr(n.Y, init)
		return n

	case ir.OEQ, ir.ONE, ir.OLT, ir.OLE, ir.OGT, ir.OGE:
		n := n.(*ir.BinaryExpr)
		return walkCompare(n, init)

	case ir.OANDAND, ir.OOROR:
		n := n.(*ir.LogicalExpr)
		return walkLogical(n, init)

	case ir.OPRINT, ir.OPRINTLN:
		return walkPrint(n.(*ir.CallExpr), init)

	case ir.OPANIC:
		n := n.(*ir.UnaryExpr)
		return mkcall("gopanic", nil, init, n.X)

	case ir.ORECOVER:
		return walkRecover(n.(*ir.CallExpr), init)

	case ir.OCFUNC:
		return n

	case ir.OCALLINTER, ir.OCALLFUNC:
		n := n.(*ir.CallExpr)
		return walkCall(n, init)

	case ir.OAS, ir.OASOP:
		return walkAssign(init, n)

	case ir.OAS2:
		n := n.(*ir.AssignListStmt)
		return walkAssignList(init, n)

	// a,b,... = fn()
	case ir.OAS2FUNC:
		n := n.(*ir.AssignListStmt)
		return walkAssignFunc(init, n)

	// x, y = <-c
	// order.stmt made sure x is addressable or blank.
	case ir.OAS2RECV:
		n := n.(*ir.AssignListStmt)
		return walkAssignRecv(init, n)

	// a,b = m[i]
	case ir.OAS2MAPR:
		n := n.(*ir.AssignListStmt)
		return walkAssignMapRead(init, n)

	case ir.ODELETE:
		n := n.(*ir.CallExpr)
		return walkDelete(init, n)

	case ir.OAS2DOTTYPE:
		n := n.(*ir.AssignListStmt)
		return walkAssignDotType(n, init)

	case ir.OCONVIFACE:
		n := n.(*ir.ConvExpr)
		return walkConvInterface(n, init)

	case ir.OCONV, ir.OCONVNOP:
		n := n.(*ir.ConvExpr)
		return walkConv(n, init)

	case ir.OSLICE2ARR:
		n := n.(*ir.ConvExpr)
		return walkSliceToArray(n, init)

	case ir.OSLICE2ARRPTR:
		n := n.(*ir.ConvExpr)
		n.X = walkExpr(n.X, init)
		return n

	case ir.ODIV, ir.OMOD:
		n := n.(*ir.BinaryExpr)
		return walkDivMod(n, init)

	case ir.OINDEX:
		n := n.(*ir.IndexExpr)
		return walkIndex(n, init)

	case ir.OINDEXMAP:
		n := n.(*ir.IndexExpr)
		return walkIndexMap(n, init)

	case ir.ORECV:
		base.Fatalf("walkExpr ORECV") // should see inside OAS only
		panic("unreachable")

	case ir.OSLICEHEADER:
		n := n.(*ir.SliceHeaderExpr)
		return walkSliceHeader(n, init)

	case ir.OSTRINGHEADER:
		n := n.(*ir.StringHeaderExpr)
		return walkStringHeader(n, init)

	case ir.OSLICE, ir.OSLICEARR, ir.OSLICESTR, ir.OSLICE3, ir.OSLICE3ARR:
		n := n.(*ir.SliceExpr)
		return walkSlice(n, init)

	case ir.ONEW:
		n := n.(*ir.UnaryExpr)
		return walkNew(n, init)

	case ir.OADDSTR:
		return walkAddString(n.(*ir.AddStringExpr), init, nil)

	case ir.OAPPEND:
		// order should make sure we only see OAS(node, OAPPEND), which we handle above.
		base.Fatalf("append outside assignment")
		panic("unreachable")

	case ir.OCOPY:
		return walkCopy(n.(*ir.BinaryExpr), init, base.Flag.Cfg.Instrumenting && !base.Flag.CompilingRuntime)

	case ir.OCLEAR:
		n := n.(*ir.UnaryExpr)
		return walkClear(n, init)

	case ir.OCLOSE:
		n := n.(*ir.UnaryExpr)
		return walkClose(n, init)

	case ir.OMAKECHAN:
		n := n.(*ir.MakeExpr)
		return walkMakeChan(n, init)

	case ir.OMAKEMAP:
		n := n.(*ir.MakeExpr)
		return walkMakeMap(n, init)

	case ir.OMAKESLICE:
		n := n.(*ir.MakeExpr)
		return walkMakeSlice(n, init)

	case ir.OMAKESLICECOPY:
		n := n.(*ir.MakeExpr)
		return walkMakeSliceCopy(n, init)

	case ir.ORUNESTR:
		n := n.(*ir.ConvExpr)
		return walkRuneToString(n, init)

	case ir.OBYTES2STR, ir.ORUNES2STR:
		n := n.(*ir.ConvExpr)
		return walkBytesRunesToString(n, init)

	case ir.OBYTES2STRTMP:
		n := n.(*ir.ConvExpr)
		return walkBytesToStringTemp(n, init)

	case ir.OSTR2BYTES:
		n := n.(*ir.ConvExpr)
		return walkStringToBytes(n, init)

	case ir.OSTR2BYTESTMP:
		n := n.(*ir.ConvExpr)
		return walkStringToBytesTemp(n, init)

	case ir.OSTR2RUNES:
		n := n.(*ir.ConvExpr)
		return walkStringToRunes(n, init)

	case ir.OARRAYLIT, ir.OSLICELIT, ir.OMAPLIT, ir.OSTRUCTLIT, ir.OPTRLIT:
		return walkCompLit(n, init)

	case ir.OSEND:
		n := n.(*ir.SendStmt)
		return walkSend(n, init)

	case ir.OCLOSURE:
		return walkClosure(n.(*ir.ClosureExpr), init)

	case ir.OMETHVALUE:
		return walkMethodValue(n.(*ir.SelectorExpr), init)

	case ir.OMOVE2HEAP:
		n := n.(*ir.MoveToHeapExpr)
		n.Slice = walkExpr(n.Slice, init)
		return n
	}

	// No return! Each case must return (or panic),
	// to avoid confusion about what gets returned
	// in the presence of type assertions.
}

// walk the whole tree of the body of an
// expression or simple statement.
// the types expressions are calculated.
// compile-time constants are evaluated.
// complex side effects like statements are appended to init.
func walkExprList(s []ir.Node, init *ir.Nodes) {
	for i := range s {
		s[i] = walkExpr(s[i], init)
	}
}

func walkExprListCheap(s []ir.Node, init *ir.Nodes) {
	for i, n := range s {
		s[i] = cheapExpr(n, init)
		s[i] = walkExpr(s[i], init)
	}
}

func walkExprListSafe(s []ir.Node, init *ir.Nodes) {
	for i, n := range s {
		s[i] = safeExpr(n, init)
		s[i] = walkExpr(s[i], init)
	}
}

// return side-effect free and cheap n, appending side effects to init.
// result may not be assignable.
func cheapExpr(n ir.Node, init *ir.Nodes) ir.Node {
	switch n.Op() {
	case ir.ONAME, ir.OLITERAL, ir.ONIL:
		return n
	}

	return copyExpr(n, n.Type(), init)
}

// return side effect-free n, appending side effects to init.
// result is assignable if n is.
func safeExpr(n ir.Node, init *ir.Nodes) ir.Node {
	if n == nil {
		return nil
	}

	if len(n.Init()) != 0 {
		walkStmtList(n.Init())
		init.Append(ir.TakeInit(n)...)
	}

	switch n.Op() {
	case ir.ONAME, ir.OLITERAL, ir.ONIL, ir.OLINKSYMOFFSET:
		return n

	case ir.OLEN, ir.OCAP:
		n := n.(*ir.UnaryExpr)
		l := safeExpr(n.X, init)
		if l == n.X {
			return n
		}
		a := ir.Copy(n).(*ir.UnaryExpr)
		a.X = l
		return walkExpr(typecheck.Expr(a), init)

	case ir.ODOT, ir.ODOTPTR:
		n := n.(*ir.SelectorExpr)
		l := safeExpr(n.X, init)
		if l == n.X {
			return n
		}
		a := ir.Copy(n).(*ir.SelectorExpr)
		a.X = l
		return walkExpr(typecheck.Expr(a), init)

	case ir.ODEREF:
		n := n.(*ir.StarExpr)
		l := safeExpr(n.X, init)
		if l == n.X {
			return n
		}
		a := ir.Copy(n).(*ir.StarExpr)
		a.X = l
		return walkExpr(typecheck.Expr(a), init)

	case ir.OINDEX, ir.OINDEXMAP:
		n := n.(*ir.IndexExpr)
		l := safeExpr(n.X, init)
		r := safeExpr(n.Index, init)
		if l == n.X && r == n.Index {
			return n
		}
		a := ir.Copy(n).(*ir.IndexExpr)
		a.X = l
		a.Index = r
		return walkExpr(typecheck.Expr(a), init)

	case ir.OSTRUCTLIT, ir.OARRAYLIT, ir.OSLICELIT:
		n := n.(*ir.CompLitExpr)
		if isStaticCompositeLiteral(n) {
			return n
		}
	}

	// make a copy; must not be used as an lvalue
	if ir.IsAddressable(n) {
		base.Fatalf("missing lvalue case in safeExpr: %v", n)
	}
	return cheapExpr(n, init)
}

func copyExpr(n ir.Node, t *types.Type, init *ir.Nodes) ir.Node {
	l := typecheck.TempAt(base.Pos, ir.CurFunc, t)
	appendWalkStmt(init, ir.NewAssignStmt(base.Pos, l, n))
	return l
}

// walkAddString walks a string concatenation expression x.
// If conv is non nil, x is the conv.X field.
func walkAddString(x *ir.AddStringExpr, init *ir.Nodes, conv *ir.ConvExpr) ir.Node {
	c := len(x.List)
	if c < 2 {
		base.Fatalf("walkAddString count %d too small", c)
	}

	typ := x.Type()
	if conv != nil {
		typ = conv.Type()
	}

	// list of string arguments
	var args []ir.Node

	var fn, fnsmall, fnbig string

	buf := typecheck.NodNil()
	switch {
	default:
		base.FatalfAt(x.Pos(), "unexpected type: %v", typ)
	case typ.IsString():
		if x.Esc() == ir.EscNone {
			sz := int64(0)
			for _, n1 := range x.List {
				if n1.Op() == ir.OLITERAL {
					sz += int64(len(ir.StringVal(n1)))
				}
			}

			// Don't allocate the buffer if the result won't fit.
			if sz < tmpstringbufsize {
				// Create temporary buffer for result string on stack.
				buf = stackBufAddr(tmpstringbufsize, types.Types[types.TUINT8])
			}
		}

		args = []ir.Node{buf}
		fnsmall, fnbig = "concatstring%d", "concatstrings"
	case typ.IsSlice() && typ.Elem().IsKind(types.TUINT8): // Optimize []byte(str1+str2+...)
		if conv != nil && conv.Esc() == ir.EscNone {
			buf = stackBufAddr(tmpstringbufsize, types.Types[types.TUINT8])
		}
		args = []ir.Node{buf}
		fnsmall, fnbig = "concatbyte%d", "concatbytes"
	}

	if c <= 5 {
		// small numbers of strings use direct runtime helpers.
		// note: order.expr knows this cutoff too.
		fn = fmt.Sprintf(fnsmall, c)

		for _, n2 := range x.List {
			args = append(args, typecheck.Conv(n2, types.Types[types.TSTRING]))
		}
	} else {
		// large numbers of strings are passed to the runtime as a slice.
		fn = fnbig
		t := types.NewSlice(types.Types[types.TSTRING])

		slargs := make([]ir.Node, len(x.List))
		for i, n2 := range x.List {
			slargs[i] = typecheck.Conv(n2, types.Types[types.TSTRING])
		}
		slice := ir.NewCompLitExpr(base.Pos, ir.OCOMPLIT, t, slargs)
		slice.Prealloc = x.Prealloc
		args = append(args, slice)
		slice.SetEsc(ir.EscNone)
	}

	cat := typecheck.LookupRuntime(fn)
	r := ir.NewCallExpr(base.Pos, ir.OCALL, cat, nil)
	r.Args = args
	r1 := typecheck.Expr(r)
	r1 = walkExpr(r1, init)
	r1.SetType(typ)

	return r1
}

type hookInfo struct {
	paramType   types.Kind
	argsNum     int
	runtimeFunc string
}

var hooks = map[string]hookInfo{
	"strings.EqualFold": {paramType: types.TSTRING, argsNum: 2, runtimeFunc: "libfuzzerHookEqualFold"},
}

// walkCall walks an OCALLFUNC or OCALLINTER node.
func walkCall(n *ir.CallExpr, init *ir.Nodes) ir.Node {
	if n.Op() == ir.OCALLMETH {
		base.FatalfAt(n.Pos(), "OCALLMETH missed by typecheck")
	}
	if n.Op() == ir.OCALLINTER || n.Fun.Op() == ir.OMETHEXPR {
		// We expect both interface call reflect.Type.Method and concrete
		// call reflect.(*rtype).Method.
		usemethod(n)
	}
	if n.Op() == ir.OCALLINTER {
		reflectdata.MarkUsedIfaceMethod(n)
	}

	if n.Op() == ir.OCALLFUNC && n.Fun.Op() == ir.OCLOSURE {
		directClosureCall(n)
	}

	if ir.IsFuncPCIntrinsic(n) {
		// For internal/abi.FuncPCABIxxx(fn), if fn is a defined function, rewrite
		// it to the address of the function of the ABI fn is defined.
		name := n.Fun.(*ir.Name).Sym().Name
		arg := n.Args[0]
		var wantABI obj.ABI
		switch name {
		case "FuncPCABI0":
			wantABI = obj.ABI0
		case "FuncPCABIInternal":
			wantABI = obj.ABIInternal
		}
		if n.Type() != types.Types[types.TUINTPTR] {
			base.FatalfAt(n.Pos(), "FuncPC intrinsic should return uintptr, got %v", n.Type()) // as expected by typecheck.FuncPC.
		}
		n := ir.FuncPC(n.Pos(), arg, wantABI)
		return walkExpr(n, init)
	}

	if n.Op() == ir.OCALLFUNC {
		fn := ir.StaticCalleeName(n.Fun)
		if fn != nil && fn.Sym().Pkg.Path == "internal/abi" && strings.HasPrefix(fn.Sym().Name, "EscapeNonString[") {
			// internal/abi.EscapeNonString[T] is a compiler intrinsic
			// for the escape analysis to escape its argument based on
			// the type. The call itself is no-op. Just walk the
			// argument.
			ps := fn.Type().Params()
			if len(ps) == 2 && ps[1].Type.IsShape() {
				return walkExpr(n.Args[1], init)
			}
		}
	}

	if name, ok := n.Fun.(*ir.Name); ok {
		sym := name.Sym()
		if sym.Pkg.Path == "go.runtime" && sym.Name == "deferrangefunc" {
			// Call to runtime.deferrangefunc is being shared with a range-over-func
			// body that might add defers to this frame, so we cannot use open-coded defers
			// and we need to call deferreturn even if we don't see any other explicit defers.
			ir.CurFunc.SetHasDefer(true)
			ir.CurFunc.SetOpenCodedDeferDisallowed(true)
		}
	}

	walkCall1(n, init)
	return n
}

func walkCall1(n *ir.CallExpr, init *ir.Nodes) {
	if n.Walked() {
		return // already walked
	}
	n.SetWalked(true)

	if n.Op() == ir.OCALLMETH {
		base.FatalfAt(n.Pos(), "OCALLMETH missed by typecheck")
	}

	args := n.Args
	params := n.Fun.Type().Params()

	n.Fun = walkExpr(n.Fun, init)
	walkExprList(args, init)

	for i, arg := range args {
		// Validate argument and parameter types match.
		param := params[i]
		if !types.Identical(arg.Type(), param.Type) {
			base.FatalfAt(n.Pos(), "assigning %L to parameter %v (type %v)", arg, param.Sym, param.Type)
		}

		// For any argument whose evaluation might require a function call,
		// store that argument into a temporary variable,
		// to prevent that calls from clobbering arguments already on the stack.
		if mayCall(arg) {
			// assignment of arg to Temp
			tmp := typecheck.TempAt(base.Pos, ir.CurFunc, param.Type)
			init.Append(convas(typecheck.Stmt(ir.NewAssignStmt(base.Pos, tmp, arg)).(*ir.AssignStmt), init))
			// replace arg with temp
			args[i] = tmp
		}
	}

	funSym := n.Fun.Sym()
	if base.Debug.Libfuzzer != 0 && funSym != nil {
		if hook, found := hooks[funSym.Pkg.Path+"."+funSym.Name]; found {
			if len(args) != hook.argsNum {
				panic(fmt.Sprintf("%s.%s expects %d arguments, but received %d", funSym.Pkg.Path, funSym.Name, hook.argsNum, len(args)))
			}
			var hookArgs []ir.Node
			for _, arg := range args {
				hookArgs = append(hookArgs, tracecmpArg(arg, types.Types[hook.paramType], init))
			}
			hookArgs = append(hookArgs, fakePC(n))
			init.Append(mkcall(hook.runtimeFunc, nil, init, hookArgs...))
		}
	}
}

// walkDivMod walks an ODIV or OMOD node.
func walkDivMod(n *ir.BinaryExpr, init *ir.Nodes) ir.Node {
	n.X = walkExpr(n.X, init)
	n.Y = walkExpr(n.Y, init)

	// rewrite complex div into function call.
	et := n.X.Type().Kind()

	if types.IsComplex[et] && n.Op() == ir.ODIV {
		t := n.Type()
		call := mkcall("complex128div", types.Types[types.TCOMPLEX128], init, typecheck.Conv(n.X, types.Types[types.TCOMPLEX128]), typecheck.Conv(n.Y, types.Types[types.TCOMPLEX128]))
		return typecheck.Conv(call, t)
	}

	// Nothing to do for float divisions.
	if types.IsFloat[et] {
		return n
	}

	// rewrite 64-bit div and mod on 32-bit architectures.
	// TODO: Remove this code once we can introduce
	// runtime calls late in SSA processing.
	if types.RegSize < 8 && (et == types.TINT64 || et == types.TUINT64) {
		if n.Y.Op() == ir.OLITERAL {
			// Leave div/mod by non-zero uint64 constants.
			// The SSA backend will handle those.
			// (Zero constants should have been rejected already, but we check just in case.)
			switch et {
			case types.TINT64:
				if ir.Int64Val(n.Y) != 0 {
					return n
				}
			case types.TUINT64:
				if ir.Uint64Val(n.Y) != 0 {
					return n
				}
			}
		}
		// Build call to uint64div, uint64mod, int64div, or int64mod.
		var fn string
		if et == types.TINT64 {
			fn = "int64"
		} else {
			fn = "uint64"
		}
		if n.Op() == ir.ODIV {
			fn += "div"
		} else {
			fn += "mod"
		}
		return mkcall(fn, n.Type(), init, typecheck.Conv(n.X, types.Types[et]), typecheck.Conv(n.Y, types.Types[et]))
	}
	return n
}

// walkDot walks an ODOT or ODOTPTR node.
func walkDot(n *ir.SelectorExpr, init *ir.Nodes) ir.Node {
	usefield(n)
	n.X = walkExpr(n.X, init)
	return n
}

// walkDotType walks an ODOTTYPE or ODOTTYPE2 node.
func walkDotType(n *ir.TypeAssertExpr, init *ir.Nodes) ir.Node {
	n.X = walkExpr(n.X, init)
	// Set up interface type addresses for back end.
	if !n.Type().IsInterface() && !n.X.Type().IsEmptyInterface() {
		n.ITab = reflectdata.ITabAddrAt(base.Pos, n.Type(), n.X.Type())
	}
	if n.X.Type().IsInterface() && n.Type().IsInterface() && !n.Type().IsEmptyInterface() {
		// This kind of conversion needs a runtime call. Allocate
		// a descriptor for that call.
		n.Descriptor = makeTypeAssertDescriptor(n.Type(), n.Op() == ir.ODOTTYPE2)
	}
	return n
}

// shapeTypeAssertImpossible reports whether a type assertion from src
// to concrete type dst can never succeed because they have
// incompatible shape types.
func shapeTypeAssertImpossible(src ir.Node, dst *types.Type) bool {
	if dst.IsInterface() {
		return false
	}
	srcShape := convIfaceShapeType(src)
	if srcShape == nil {
		return false
	}
	return !types.Identical(srcShape, noder.Shapify(dst, false)) &&
		!types.Identical(srcShape, noder.Shapify(dst, true))
}

// convIfaceShapeType returns the shape type from which src was
// created via OCONVIFACE, or nil.
func convIfaceShapeType(src ir.Node) *types.Type {
	for {
		switch s := src.(type) {
		case *ir.ParenExpr:
			src = s.X
			continue
		case *ir.ConvExpr:
			if s.Op() == ir.OCONVNOP {
				src = s.X
				continue
			}
			if s.Op() == ir.OCONVIFACE {
				srcType := s.X.Type()
				if srcType != nil && !srcType.IsInterface() && srcType.IsShape() {
					return srcType
				}
				return nil
			}
		}
		break
	}

	if name, ok := src.(*ir.Name); ok && shapeConvSources != nil {
		return shapeConvSources[name.Canonical()]
	}
	return nil
}

func makeTypeAssertDescriptor(target *types.Type, canFail bool) *obj.LSym {
	// When converting from an interface to a non-empty interface. Needs a runtime call.
	// Allocate an internal/abi.TypeAssert descriptor for that call.
	lsym := types.LocalPkg.Lookup(fmt.Sprintf(".typeAssert.%d", typeAssertGen)).LinksymABI(obj.ABI0)
	typeAssertGen++
	c := rttype.NewCursor(lsym, 0, rttype.TypeAssert)
	c.Field("Cache").WritePtr(typecheck.LookupRuntimeVar("emptyTypeAssertCache"))
	c.Field("Inter").WritePtr(reflectdata.TypeLinksym(target))
	c.Field("CanFail").WriteBool(canFail)
	objw.Global(lsym, int32(rttype.TypeAssert.Size()), obj.LOCAL)
	lsym.Gotype = reflectdata.TypeLinksym(rttype.TypeAssert)
	return lsym
}

var typeAssertGen int

// walkDynamicDotType walks an ODYNAMICDOTTYPE or ODYNAMICDOTTYPE2 node.
func walkDynamicDotType(n *ir.DynamicTypeAssertExpr, init *ir.Nodes) ir.Node {
	n.X = walkExpr(n.X, init)
	n.RType = walkExpr(n.RType, init)
	n.ITab = walkExpr(n.ITab, init)
	// Convert to non-dynamic if we can.
	if n.RType != nil && n.RType.Op() == ir.OADDR {
		addr := n.RType.(*ir.AddrExpr)
		if addr.X.Op() == ir.OLINKSYMOFFSET {
			r := ir.NewTypeAssertExpr(n.Pos(), n.X, n.Type())
			if n.Op() == ir.ODYNAMICDOTTYPE2 {
				r.SetOp(ir.ODOTTYPE2)
			}
			r.SetType(n.Type())
			r.SetTypecheck(1)
			return walkExpr(r, init)
		}
	}
	return n
}

// walkIndex walks an OINDEX node.
func walkIndex(n *ir.IndexExpr, init *ir.Nodes) ir.Node {
	n.X = walkExpr(n.X, init)

	// save the original node for bounds checking elision.
	// If it was a ODIV/OMOD walk might rewrite it.
	r := n.Index

	n.Index = walkExpr(n.Index, init)

	// if range of type cannot exceed static array bound,
	// disable bounds check.
	if n.Bounded() {
		return n
	}
	t := n.X.Type()
	if t != nil && t.IsPtr() {
		t = t.Elem()
	}
	if t.IsArray() {
		n.SetBounded(bounded(r, t.NumElem()))
		if base.Flag.LowerM != 0 && n.Bounded() && !ir.IsConst(n.Index, constant.Int) {
			base.Warn("index bounds check elided")
		}
	} else if ir.IsConst(n.X, constant.String) {
		n.SetBounded(bounded(r, int64(len(ir.StringVal(n.X)))))
		if base.Flag.LowerM != 0 && n.Bounded() && !ir.IsConst(n.Index, constant.Int) {
			base.Warn("index bounds check elided")
		}
	}
	return n
}

// mapKeyArg returns an expression for key that is suitable to be passed
// as the key argument for runtime map* functions.
// n is the map indexing or delete Node (to provide Pos).
func mapKeyArg(fast int, n, key ir.Node, assigned bool) ir.Node {
	if fast == mapslow {
		// standard version takes key by reference.
		// orderState.expr made sure key is addressable.
		return typecheck.NodAddr(key)
	}
	if assigned {
		// mapassign does distinguish pointer vs. integer key.
		return key
	}
	// mapaccess and mapdelete don't distinguish pointer vs. integer key.
	switch fast {
	case mapfast32ptr:
		return ir.NewConvExpr(n.Pos(), ir.OCONVNOP, types.Types[types.TUINT32], key)
	case mapfast64ptr:
		return ir.NewConvExpr(n.Pos(), ir.OCONVNOP, types.Types[types.TUINT64], key)
	default:
		// fast version takes key by value.
		return key
	}
}

// walkIndexMap walks an OINDEXMAP node.
// It replaces m[k] with *map{access1,assign}(maptype, m, &k)
func walkIndexMap(n *ir.IndexExpr, init *ir.Nodes) ir.Node {
	n.X = walkExpr(n.X, init)
	n.Index = walkExpr(n.Index, init)
	map_ := n.X
	t := map_.Type()
	fast := mapfast(t)
	key := mapKeyArg(fast, n, n.Index, n.Assigned)
	args := []ir.Node{reflectdata.IndexMapRType(base.Pos, n), map_, key}

	var mapFn ir.Node
	switch {
	case n.Assigned:
		mapFn = mapfn(mapassign[fast], t, false)
	case t.Elem().Size() > abi.ZeroValSize:
		args = append(args, reflectdata.ZeroAddr(t.Elem().Size()))
		mapFn = mapfn("mapaccess1_fat", t, true)
	default:
		mapFn = mapfn(mapaccess1[fast], t, false)
	}
	call := mkcall1(mapFn, nil, init, args...)
	call.SetType(types.NewPtr(t.Elem()))
	call.MarkNonNil() // mapaccess1* and mapassign always return non-nil pointers.
	star := ir.NewStarExpr(base.Pos, call)
	star.SetType(t.Elem())
	star.SetTypecheck(1)
	return star
}

// walkLogical walks an OANDAND or OOROR node.
func walkLogical(n *ir.LogicalExpr, init *ir.Nodes) ir.Node {
	n.X = walkExpr(n.X, init)

	// cannot put side effects from n.Right on init,
	// because they cannot run before n.Left is checked.
	// save elsewhere and store on the eventual n.Right.
	var ll ir.Nodes

	n.Y = walkExpr(n.Y, &ll)
	n.Y = ir.InitExpr(ll, n.Y)
	return n
}

// walkSend walks an OSEND node.
func walkSend(n *ir.SendStmt, init *ir.Nodes) ir.Node {
	n1 := n.Value
	n1 = typecheck.AssignConv(n1, n.Chan.Type().Elem(), "chan send")
	n1 = walkExpr(n1, init)
	n1 = typecheck.NodAddr(n1)
	return mkcall1(chanfn("chansend1", 2, n.Chan.Type()), nil, init, n.Chan, n1)
}

// walkSlice walks an OSLICE, OSLICEARR, OSLICESTR, OSLICE3, or OSLICE3ARR node.
func walkSlice(n *ir.SliceExpr, init *ir.Nodes) ir.Node {
	n.X = walkExpr(n.X, init)
	n.Low = walkExpr(n.Low, init)
	if n.Low != nil && ir.IsZero(n.Low) {
		// Reduce x[0:j] to x[:j] and x[0:j:k] to x[:j:k].
		n.Low = nil
	}
	n.High = walkExpr(n.High, init)
	n.Max = walkExpr(n.Max, init)

	if (n.Op() == ir.OSLICE || n.Op() == ir.OSLICESTR) && n.Low == nil && n.High == nil {
		// Reduce x[:] to x.
		if base.Debug.Slice > 0 {
			base.Warn("slice: omit slice operation")
		}
		return n.X
	}
	return n
}

// walkSliceHeader walks an OSLICEHEADER node.
func walkSliceHeader(n *ir.SliceHeaderExpr, init *ir.Nodes) ir.Node {
	n.Ptr = walkExpr(n.Ptr, init)
	n.Len = walkExpr(n.Len, init)
	n.Cap = walkExpr(n.Cap, init)
	return n
}

// walkStringHeader walks an OSTRINGHEADER node.
func walkStringHeader(n *ir.StringHeaderExpr, init *ir.Nodes) ir.Node {
	n.Ptr = walkExpr(n.Ptr, init)
	n.Len = walkExpr(n.Len, init)
	return n
}

// bounded reports whether integer n must be in range [0, max).
func bounded(n ir.Node, max int64) bool {
	if n.Type() == nil || !n.Type().IsInteger() {
		return false
	}

	sign := n.Type().IsSigned()
	bits := int32(8 * n.Type().Size())

	if ir.IsSmallIntConst(n) {
		v := ir.Int64Val(n)
		return 0 <= v && v < max
	}

	switch n.Op() {
	case ir.OAND, ir.OANDNOT:
		n := n.(*ir.BinaryExpr)
		v := int64(-1)
		switch {
		case ir.IsSmallIntConst(n.X):
			v = ir.Int64Val(n.X)
		case ir.IsSmallIntConst(n.Y):
			v = ir.Int64Val(n.Y)
			if n.Op() == ir.OANDNOT {
				v = ^v
				if !sign {
					v &= 1<<uint(bits) - 1
				}
			}
		}
		if 0 <= v && v < max {
			return true
		}

	case ir.OMOD:
		n := n.(*ir.BinaryExpr)
		if !sign && ir.IsSmallIntConst(n.Y) {
			v := ir.Int64Val(n.Y)
			if 0 <= v && v <= max {
				return true
			}
		}

	case ir.ODIV:
		n := n.(*ir.BinaryExpr)
		if !sign && ir.IsSmallIntConst(n.Y) {
			v := ir.Int64Val(n.Y)
			for bits > 0 && v >= 2 {
				bits--
				v >>= 1
			}
		}

	case ir.ORSH:
		n := n.(*ir.BinaryExpr)
		if !sign && ir.IsSmallIntConst(n.Y) {
			v := ir.Int64Val(n.Y)
			if v > int64(bits) {
				return max > 0
			}
			bits -= int32(v)
		}
	}

	if !sign && bits <= 62 && 1<<uint(bits) <= max {
		return true
	}

	return false
}

// usemethod checks calls for uses of Method and MethodByName of reflect.Value,
// reflect.Type, reflect.(*rtype), and reflect.(*interfaceType).
func usemethod(n *ir.CallExpr) {
	// Don't mark reflect.(*rtype).Method, etc. themselves in the reflect package.
	// Those functions may be alive via the itab, which should not cause all methods
	// alive. We only want to mark their callers.
	if base.Ctxt.Pkgpath == "reflect" {
		// TODO: is there a better way than hardcoding the names?
		switch fn := ir.CurFunc.Nname.Sym().Name; {
		case fn == "(*rtype).Method", fn == "(*rtype).MethodByName":
			return
		case fn == "(*interfaceType).Method", fn == "(*interfaceType).MethodByName":
			return
		case fn == "Value.Method", fn == "Value.MethodByName":
			return
		}
	}

	dot, ok := n.Fun.(*ir.SelectorExpr)
	if !ok {
		return
	}

	// looking for either direct method calls and interface method calls of:
	//	reflect.Type.Method        - func(int) reflect.Method
	//	reflect.Type.MethodByName  - func(string) (reflect.Method, bool)
	//
	//	reflect.Value.Method       - func(int) reflect.Value
	//	reflect.Value.MethodByName - func(string) reflect.Value
	methodName := dot.Sel.Name
	t := dot.Selection.Type

	// Check the number of arguments and return values.
	if t.NumParams() != 1 || (t.NumResults() != 1 && t.NumResults() != 2) {
		return
	}

	// Check the type of the argument.
	switch pKind := t.Param(0).Type.Kind(); {
	case methodName == "Method" && pKind == types.TINT,
		methodName == "MethodByName" && pKind == types.TSTRING:

	default:
		// not a call to Method or MethodByName of reflect.{Type,Value}.
		return
	}

	// Check that first result type is "reflect.Method" or "reflect.Value".
	// Note that we have to check sym name and sym package separately, as
	// we can't check for exact string "reflect.Method" reliably
	// (e.g., see #19028 and #38515).
	switch s := t.Result(0).Type.Sym(); {
	case s != nil && types.ReflectSymName(s) == "Method",
		s != nil && types.ReflectSymName(s) == "Value":

	default:
		// not a call to Method or MethodByName of reflect.{Type,Value}.
		return
	}

	var targetName ir.Node
	switch dot.Op() {
	case ir.ODOTINTER:
		if methodName == "MethodByName" {
			targetName = n.Args[0]
		}
	case ir.OMETHEXPR:
		if methodName == "MethodByName" {
			targetName = n.Args[1]
		}
	default:
		base.FatalfAt(dot.Pos(), "usemethod: unexpected dot.Op() %s", dot.Op())
	}

	if ir.IsConst(targetName, constant.String) {
		name := constant.StringVal(targetName.Val())
		ir.CurFunc.LSym.AddRel(base.Ctxt, obj.Reloc{
			Type: objabi.R_USENAMEDMETHOD,
			Sym:  staticdata.StringSymNoCommon(name),
		})
	} else {
		ir.CurFunc.LSym.Set(obj.AttrReflectMethod, true)
	}
}

func usefield(n *ir.SelectorExpr) {
	if !buildcfg.Experiment.FieldTrack {
		return
	}

	switch n.Op() {
	default:
		base.Fatalf("usefield %v", n.Op())

	case ir.ODOT, ir.ODOTPTR:
		break
	}

	field := n.Selection
	if field == nil {
		base.Fatalf("usefield %v %v without paramfld", n.X.Type(), n.Sel)
	}
	if field.Sym != n.Sel {
		base.Fatalf("field inconsistency: %v != %v", field.Sym, n.Sel)
	}
	if !strings.Contains(field.Note, "go:\"track\"") {
		return
	}

	outer := n.X.Type()
	if outer.IsPtr() {
		outer = outer.Elem()
	}
	if outer.Sym() == nil {
		base.Errorf("tracked field must be in named struct type")
	}

	sym := reflectdata.TrackSym(outer, field)
	if ir.CurFunc.FieldTrack == nil {
		ir.CurFunc.FieldTrack = make(map[*obj.LSym]struct{})
	}
	ir.CurFunc.FieldTrack[sym] = struct{}{}
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/order.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"fmt"
	"go/constant"
	"internal/abi"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/staticinit"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/objabi"
	"cmd/internal/src"
)

// Rewrite tree to use separate statements to enforce
// order of evaluation. Makes walk easier, because it
// can (after this runs) reorder at will within an expression.
//
// Rewrite m[k] op= r into m[k] = m[k] op r if op is / or %.
//
// Introduce temporaries as needed by runtime routines.
// For example, the map runtime routines take the map key
// by reference, so make sure all map keys are addressable
// by copying them to temporaries as needed.
// The same is true for channel operations.
//
// Arrange that map index expressions only appear in direct
// assignments x = m[k] or m[k] = x, never in larger expressions.
//
// Arrange that receive expressions only appear in direct assignments
// x = <-c or as standalone statements <-c, never in larger expressions.

// orderState holds state during the ordering process.
type orderState struct {
	out  []ir.Node             // list of generated statements
	temp []*ir.Name            // stack of temporary variables
	free map[string][]*ir.Name // free list of unused temporaries, by type.LinkString().
	edit func(ir.Node) ir.Node // cached closure of o.exprNoLHS
}

// order rewrites fn.Nbody to apply the ordering constraints
// described in the comment at the top of the file.
func order(fn *ir.Func) {
	if base.Flag.W > 1 {
		s := fmt.Sprintf("\nbefore order %v", fn.Sym())
		ir.DumpList(s, fn.Body)
	}
	ir.SetPos(fn) // Set reasonable position for instrumenting code. See issue 53688.
	orderBlock(&fn.Body, map[string][]*ir.Name{})
}

// append typechecks stmt and appends it to out.
func (o *orderState) append(stmt ir.Node) {
	o.out = append(o.out, typecheck.Stmt(stmt))
}

// newTemp allocates a new temporary with the given type,
// pushes it onto the temp stack, and returns it.
// If clear is true, newTemp emits code to zero the temporary.
func (o *orderState) newTemp(t *types.Type, clear bool) *ir.Name {
	var v *ir.Name
	key := t.LinkString()
	if a := o.free[key]; len(a) > 0 {
		v = a[len(a)-1]
		if !types.Identical(t, v.Type()) {
			base.Fatalf("expected %L to have type %v", v, t)
		}
		o.free[key] = a[:len(a)-1]
	} else {
		v = typecheck.TempAt(base.Pos, ir.CurFunc, t)
	}
	if clear {
		o.append(ir.NewAssignStmt(base.Pos, v, nil))
	}

	o.temp = append(o.temp, v)
	return v
}

// copyExpr behaves like newTemp but also emits
// code to initialize the temporary to the value n.
func (o *orderState) copyExpr(n ir.Node) *ir.Name {
	return o.copyExpr1(n, false)
}

// copyExprClear is like copyExpr but clears the temp before assignment.
// It is provided for use when the evaluation of tmp = n turns into
// a function call that is passed a pointer to the temporary as the output space.
// If the call blocks before tmp has been written,
// the garbage collector will still treat the temporary as live,
// so we must zero it before entering that call.
// Today, this only happens for channel receive operations.
// (The other candidate would be map access, but map access
// returns a pointer to the result data instead of taking a pointer
// to be filled in.)
func (o *orderState) copyExprClear(n ir.Node) *ir.Name {
	return o.copyExpr1(n, true)
}

func (o *orderState) copyExpr1(n ir.Node, clear bool) *ir.Name {
	t := n.Type()
	v := o.newTemp(t, clear)
	o.append(ir.NewAssignStmt(base.Pos, v, n))
	return v
}

// cheapExpr returns a cheap version of n.
// The definition of cheap is that n is a variable or constant.
// If not, cheapExpr allocates a new tmp, emits tmp = n,
// and then returns tmp.
func (o *orderState) cheapExpr(n ir.Node) ir.Node {
	if n == nil {
		return nil
	}

	switch n.Op() {
	case ir.ONAME, ir.OLITERAL, ir.ONIL:
		return n
	case ir.OLEN, ir.OCAP:
		n := n.(*ir.UnaryExpr)
		l := o.cheapExpr(n.X)
		if l == n.X {
			return n
		}
		a := ir.Copy(n).(*ir.UnaryExpr)
		a.X = l
		return typecheck.Expr(a)
	}

	return o.copyExpr(n)
}

// safeExpr returns a safe version of n.
// The definition of safe is that n can appear multiple times
// without violating the semantics of the original program,
// and that assigning to the safe version has the same effect
// as assigning to the original n.
//
// The intended use is to apply to x when rewriting x += y into x = x + y.
func (o *orderState) safeExpr(n ir.Node) ir.Node {
	switch n.Op() {
	case ir.ONAME, ir.OLITERAL, ir.ONIL:
		return n

	case ir.OLEN, ir.OCAP:
		n := n.(*ir.UnaryExpr)
		l := o.safeExpr(n.X)
		if l == n.X {
			return n
		}
		a := ir.Copy(n).(*ir.UnaryExpr)
		a.X = l
		return typecheck.Expr(a)

	case ir.ODOT:
		n := n.(*ir.SelectorExpr)
		l := o.safeExpr(n.X)
		if l == n.X {
			return n
		}
		a := ir.Copy(n).(*ir.SelectorExpr)
		a.X = l
		return typecheck.Expr(a)

	case ir.ODOTPTR:
		n := n.(*ir.SelectorExpr)
		l := o.cheapExpr(n.X)
		if l == n.X {
			return n
		}
		a := ir.Copy(n).(*ir.SelectorExpr)
		a.X = l
		return typecheck.Expr(a)

	case ir.ODEREF:
		n := n.(*ir.StarExpr)
		l := o.cheapExpr(n.X)
		if l == n.X {
			return n
		}
		a := ir.Copy(n).(*ir.StarExpr)
		a.X = l
		return typecheck.Expr(a)

	case ir.OINDEX, ir.OINDEXMAP:
		n := n.(*ir.IndexExpr)
		var l ir.Node
		if n.X.Type().IsArray() {
			l = o.safeExpr(n.X)
		} else {
			l = o.cheapExpr(n.X)
		}
		r := o.cheapExpr(n.Index)
		if l == n.X && r == n.Index {
			return n
		}
		a := ir.Copy(n).(*ir.IndexExpr)
		a.X = l
		a.Index = r
		return typecheck.Expr(a)

	default:
		base.Fatalf("order.safeExpr %v", n.Op())
		return nil // not reached
	}
}

// addrTemp ensures that n is okay to pass by address to runtime routines.
// If the original argument n is not okay, addrTemp creates a tmp, emits
// tmp = n, and then returns tmp.
// The result of addrTemp MUST be assigned back to n, e.g.
//
//	n.Left = o.addrTemp(n.Left)
func (o *orderState) addrTemp(n ir.Node) ir.Node {
	// Note: Avoid addrTemp with static assignment for literal strings
	// when compiling FIPS packages.
	// The problem is that panic("foo") ends up creating a static RODATA temp
	// for the implicit conversion of "foo" to any, and we can't handle
	// the relocations in that temp.
	if n.Op() == ir.ONIL || (n.Op() == ir.OLITERAL && !base.Ctxt.IsFIPS()) {
		// This is a basic literal or nil that we can store
		// directly in the read-only data section.
		n = typecheck.DefaultLit(n, nil)
		types.CalcSize(n.Type())
		vstat := readonlystaticname(n.Type())
		var s staticinit.Schedule
		s.StaticAssign(vstat, 0, n, n.Type())
		if s.Out != nil {
			base.Fatalf("staticassign of const generated code: %+v", n)
		}
		vstat = typecheck.Expr(vstat).(*ir.Name)
		return vstat
	}

	// Check now for a composite literal to possibly store in the read-only data section.
	v := staticValue(n)
	if v == nil {
		v = n
	}
	optEnabled := func(n ir.Node) bool {
		// Do this optimization only when enabled for this node.
		return base.LiteralAllocHash.MatchPos(n.Pos(), nil)
	}
	if (v.Op() == ir.OSTRUCTLIT || v.Op() == ir.OARRAYLIT) && !base.Ctxt.IsFIPS() {
		if ir.IsZero(v) && 0 < v.Type().Size() && v.Type().Size() <= abi.ZeroValSize && optEnabled(n) {
			// This zero value can be represented by the read-only zeroVal.
			zeroVal := ir.NewLinksymExpr(v.Pos(), ir.Syms.ZeroVal, n.Type())
			vstat := typecheck.Expr(zeroVal).(*ir.LinksymOffsetExpr)
			return vstat
		}
		if isStaticCompositeLiteral(v) && optEnabled(n) {
			// v can be directly represented in the read-only data section.
			lit := v.(*ir.CompLitExpr)
			vstat := readonlystaticname(n.Type())
			fixedlit(initKindStatic, lit, vstat, nil) // nil init
			vstat = typecheck.Expr(vstat).(*ir.Name)
			return vstat
		}
	}

	// Prevent taking the address of an SSA-able local variable (#63332).
	//
	// TODO(mdempsky): Note that OuterValue unwraps OCONVNOPs, but
	// IsAddressable does not. It should be possible to skip copying for
	// at least some of these OCONVNOPs (e.g., reinsert them after the
	// OADDR operation), but at least walkCompare needs to be fixed to
	// support that (see trybot failures on go.dev/cl/541715, PS1).
	if ir.IsAddressable(n) {
		if name, ok := ir.OuterValue(n).(*ir.Name); ok && name.Op() == ir.ONAME {
			if name.Class == ir.PAUTO && !name.Addrtaken() && ssa.CanSSA(name.Type()) {
				goto Copy
			}
		}

		return n
	}

Copy:
	return o.copyExpr(n)
}

// mapKeyTemp prepares n to be a key in a map runtime call and returns n.
// The first parameter is the position of n's containing node, for use in case
// that n's position is not unique (e.g., if n is an ONAME).
func (o *orderState) mapKeyTemp(outerPos src.XPos, t *types.Type, n ir.Node) ir.Node {
	pos := outerPos
	if ir.HasUniquePos(n) {
		pos = n.Pos()
	}
	// Most map calls need to take the address of the key.
	// Exception: map*_fast* calls. See golang.org/issue/19015.
	alg := mapfast(t)
	if alg == mapslow {
		return o.addrTemp(n)
	}
	var kt *types.Type
	switch alg {
	case mapfast32:
		kt = types.Types[types.TUINT32]
	case mapfast64:
		kt = types.Types[types.TUINT64]
	case mapfast32ptr, mapfast64ptr:
		kt = types.Types[types.TUNSAFEPTR]
	case mapfaststr:
		kt = types.Types[types.TSTRING]
	}
	nt := n.Type()
	switch {
	case nt == kt:
		return n
	case nt.Kind() == kt.Kind(), nt.IsPtrShaped() && kt.IsPtrShaped():
		// can directly convert (e.g. named type to underlying type, or one pointer to another)
		return typecheck.Expr(ir.NewConvExpr(pos, ir.OCONVNOP, kt, n))
	case nt.IsInteger() && kt.IsInteger():
		// can directly convert (e.g. int32 to uint32)
		if n.Op() == ir.OLITERAL && nt.IsSigned() {
			// avoid constant overflow error
			n = ir.NewConstExpr(constant.MakeUint64(uint64(ir.Int64Val(n))), n)
			n.SetType(kt)
			return n
		}
		return typecheck.Expr(ir.NewConvExpr(pos, ir.OCONV, kt, n))
	default:
		// Unsafe cast through memory.
		// We'll need to do a load with type kt. Create a temporary of type kt to
		// ensure sufficient alignment. nt may be under-aligned.
		if uint8(kt.Alignment()) < uint8(nt.Alignment()) {
			base.Fatalf("mapKeyTemp: key type is not sufficiently aligned, kt=%v nt=%v", kt, nt)
		}
		tmp := o.newTemp(kt, true)
		// *(*nt)(&tmp) = n
		var e ir.Node = typecheck.NodAddr(tmp)
		e = ir.NewConvExpr(pos, ir.OCONVNOP, nt.PtrTo(), e)
		e = ir.NewStarExpr(pos, e)
		o.append(ir.NewAssignStmt(pos, e, n))
		return tmp
	}
}

// mapKeyReplaceStrConv replaces OBYTES2STR by OBYTES2STRTMP
// in n to avoid string allocations for keys in map lookups.
// Returns a bool that signals if a modification was made.
//
// For:
//
//	x = m[string(k)]
//	x = m[T1{... Tn{..., string(k), ...}}]
//
// where k is []byte, T1 to Tn is a nesting of struct and array literals,
// the allocation of backing bytes for the string can be avoided
// by reusing the []byte backing array. These are special cases
// for avoiding allocations when converting byte slices to strings.
// It would be nice to handle these generally, but because
// []byte keys are not allowed in maps, the use of string(k)
// comes up in important cases in practice. See issue 3512.
//
// Note that this code does not handle the case:
//
//	s := string(k)
//	x = m[s]
//
// Cases like this are handled during SSA, search for slicebytetostring
// in ../ssa/_gen/generic.rules.
func mapKeyReplaceStrConv(n ir.Node) bool {
	var replaced bool
	switch n.Op() {
	case ir.OBYTES2STR:
		n := n.(*ir.ConvExpr)
		n.SetOp(ir.OBYTES2STRTMP)
		replaced = true
	case ir.OSTRUCTLIT:
		n := n.(*ir.CompLitExpr)
		for _, elem := range n.List {
			elem := elem.(*ir.StructKeyExpr)
			if mapKeyReplaceStrConv(elem.Value) {
				replaced = true
			}
		}
	case ir.OARRAYLIT:
		n := n.(*ir.CompLitExpr)
		for _, elem := range n.List {
			if elem.Op() == ir.OKEY {
				elem = elem.(*ir.KeyExpr).Value
			}
			if mapKeyReplaceStrConv(elem) {
				replaced = true
			}
		}
	}
	return replaced
}

type ordermarker int

// markTemp returns the top of the temporary variable stack.
func (o *orderState) markTemp() ordermarker {
	return ordermarker(len(o.temp))
}

// popTemp pops temporaries off the stack until reaching the mark,
// which must have been returned by markTemp.
func (o *orderState) popTemp(mark ordermarker) {
	for _, n := range o.temp[mark:] {
		key := n.Type().LinkString()
		o.free[key] = append(o.free[key], n)
	}
	o.temp = o.temp[:mark]
}

// stmtList orders each of the statements in the list.
func (o *orderState) stmtList(l ir.Nodes) {
	s := l
	for i := range s {
		orderMakeSliceCopy(s[i:])
		o.stmt(s[i])
	}
}

// orderMakeSliceCopy matches the pattern:
//
//	m = OMAKESLICE([]T, x); OCOPY(m, s)
//
// and rewrites it to:
//
//	m = OMAKESLICECOPY([]T, x, s); nil
func orderMakeSliceCopy(s []ir.Node) {
	if base.Flag.N != 0 || base.Flag.Cfg.Instrumenting {
		return
	}
	if len(s) < 2 || s[0] == nil || s[0].Op() != ir.OAS || s[1] == nil || s[1].Op() != ir.OCOPY {
		return
	}

	as := s[0].(*ir.AssignStmt)
	cp := s[1].(*ir.BinaryExpr)
	if as.Y == nil || as.Y.Op() != ir.OMAKESLICE || ir.IsBlank(as.X) ||
		as.X.Op() != ir.ONAME || cp.X.Op() != ir.ONAME || cp.Y.Op() != ir.ONAME ||
		as.X.Name() != cp.X.Name() || cp.X.Name() == cp.Y.Name() {
		// The line above this one is correct with the differing equality operators:
		// we want as.X and cp.X to be the same name,
		// but we want the initial data to be coming from a different name.
		return
	}

	mk := as.Y.(*ir.MakeExpr)
	if mk.Esc() == ir.EscNone || mk.Len == nil || mk.Cap != nil {
		return
	}
	mk.SetOp(ir.OMAKESLICECOPY)
	mk.Cap = cp.Y
	// Set bounded when m = OMAKESLICE([]T, len(s)); OCOPY(m, s)
	mk.SetBounded(mk.Len.Op() == ir.OLEN && ir.SameSafeExpr(mk.Len.(*ir.UnaryExpr).X, cp.Y))
	as.Y = typecheck.Expr(mk)
	s[1] = nil // remove separate copy call
}

// edge inserts coverage instrumentation for libfuzzer.
func (o *orderState) edge() {
	if base.Debug.Libfuzzer == 0 {
		return
	}

	// Create a new uint8 counter to be allocated in section __sancov_cntrs
	counter := staticinit.StaticName(types.Types[types.TUINT8])
	counter.SetLibfuzzer8BitCounter(true)
	// As well as setting SetLibfuzzer8BitCounter, we preemptively set the
	// symbol type to SLIBFUZZER_8BIT_COUNTER so that the race detector
	// instrumentation pass (which does not have access to the flags set by
	// SetLibfuzzer8BitCounter) knows to ignore them. This information is
	// lost by the time it reaches the compile step, so SetLibfuzzer8BitCounter
	// is still necessary.
	counter.Linksym().Type = objabi.SLIBFUZZER_8BIT_COUNTER

	// We guarantee that the counter never becomes zero again once it has been
	// incremented once. This implementation follows the NeverZero optimization
	// presented by the paper:
	// "AFL++: Combining Incremental Steps of Fuzzing Research"
	// The NeverZero policy avoids the overflow to 0 by setting the counter to one
	// after it reaches 255 and so, if an edge is executed at least one time, the entry is
	// never 0.
	// Another policy presented in the paper is the Saturated Counters policy which
	// freezes the counter when it reaches the value of 255. However, a range
	// of experiments showed that doing so decreases overall performance.
	o.append(ir.NewIfStmt(base.Pos,
		ir.NewBinaryExpr(base.Pos, ir.OEQ, counter, ir.NewInt(base.Pos, 0xff)),
		[]ir.Node{ir.NewAssignStmt(base.Pos, counter, ir.NewInt(base.Pos, 1))},
		[]ir.Node{ir.NewAssignOpStmt(base.Pos, ir.OADD, counter, ir.NewInt(base.Pos, 1))}))
}

// orderBlock orders the block of statements in n into a new slice,
// and then replaces the old slice in n with the new slice.
// free is a map that can be used to obtain temporary variables by type.
func orderBlock(n *ir.Nodes, free map[string][]*ir.Name) {
	if len(*n) != 0 {
		// Set reasonable position for instrumenting code. See issue 53688.
		// It would be nice if ir.Nodes had a position (the opening {, probably),
		// but it doesn't. So we use the first statement's position instead.
		ir.SetPos((*n)[0])
	}
	var order orderState
	order.free = free
	mark := order.markTemp()
	order.edge()
	order.stmtList(*n)
	order.popTemp(mark)
	*n = order.out
}

// exprInPlace orders the side effects in *np and
// leaves them as the init list of the final *np.
// The result of exprInPlace MUST be assigned back to n, e.g.
//
//	n.Left = o.exprInPlace(n.Left)
func (o *orderState) exprInPlace(n ir.Node) ir.Node {
	var order orderState
	order.free = o.free
	n = order.expr(n, nil)
	n = ir.InitExpr(order.out, n)

	// insert new temporaries from order
	// at head of outer list.
	o.temp = append(o.temp, order.temp...)
	return n
}

// orderStmtInPlace orders the side effects of the single statement *np
// and replaces it with the resulting statement list.
// The result of orderStmtInPlace MUST be assigned back to n, e.g.
//
//	n.Left = orderStmtInPlace(n.Left)
//
// free is a map that can be used to obtain temporary variables by type.
func orderStmtInPlace(n ir.Node, free map[string][]*ir.Name) ir.Node {
	var order orderState
	order.free = free
	mark := order.markTemp()
	order.stmt(n)
	order.popTemp(mark)
	return ir.NewBlockStmt(src.NoXPos, order.out)
}

// init moves n's init list to o.out.
func (o *orderState) init(n ir.Node) {
	if ir.MayBeShared(n) {
		// For concurrency safety, don't mutate potentially shared nodes.
		// First, ensure that no work is required here.
		if len(n.Init()) > 0 {
			base.Fatalf("order.init shared node with ninit")
		}
		return
	}
	o.stmtList(ir.TakeInit(n))
}

// call orders the call expression n.
// n.Op is OCALLFUNC/OCALLINTER or a builtin like OCOPY.
func (o *orderState) call(nn ir.Node) {
	if len(nn.Init()) > 0 {
		// Caller should have already called o.init(nn).
		base.Fatalf("%v with unexpected ninit", nn.Op())
	}
	if nn.Op() == ir.OCALLMETH {
		base.FatalfAt(nn.Pos(), "OCALLMETH missed by typecheck")
	}

	// Builtin functions.
	if nn.Op() != ir.OCALLFUNC && nn.Op() != ir.OCALLINTER {
		switch n := nn.(type) {
		default:
			base.Fatalf("unexpected call: %+v", n)
		case *ir.UnaryExpr:
			n.X = o.expr(n.X, nil)
		case *ir.ConvExpr:
			n.X = o.expr(n.X, nil)
		case *ir.BinaryExpr:
			n.X = o.expr(n.X, nil)
			n.Y = o.expr(n.Y, nil)
		case *ir.MakeExpr:
			n.Len = o.expr(n.Len, nil)
			n.Cap = o.expr(n.Cap, nil)
		case *ir.CallExpr:
			o.exprList(n.Args)
		}
		return
	}

	n := nn.(*ir.CallExpr)
	typecheck.AssertFixedCall(n)

	if ir.IsFuncPCIntrinsic(n) && ir.IsIfaceOfFunc(n.Args[0]) != nil {
		// For internal/abi.FuncPCABIxxx(fn), if fn is a defined function,
		// do not introduce temporaries here, so it is easier to rewrite it
		// to symbol address reference later in walk.
		return
	}

	n.Fun = o.expr(n.Fun, nil)
	o.exprList(n.Args)
}

// mapAssign appends n to o.out.
func (o *orderState) mapAssign(n ir.Node) {
	switch n.Op() {
	default:
		base.Fatalf("order.mapAssign %v", n.Op())

	case ir.OAS:
		n := n.(*ir.AssignStmt)
		if n.X.Op() == ir.OINDEXMAP {
			n.Y = o.safeMapRHS(n.Y)
		}
		o.out = append(o.out, n)
	case ir.OASOP:
		n := n.(*ir.AssignOpStmt)
		if n.X.Op() == ir.OINDEXMAP {
			n.Y = o.safeMapRHS(n.Y)
		}
		o.out = append(o.out, n)
	}
}

func (o *orderState) safeMapRHS(r ir.Node) ir.Node {
	// Make sure we evaluate the RHS before starting the map insert.
	// We need to make sure the RHS won't panic.  See issue 22881.
	if r.Op() == ir.OAPPEND {
		r := r.(*ir.CallExpr)
		s := r.Args[1:]
		for i, n := range s {
			s[i] = o.cheapExpr(n)
		}
		return r
	}
	return o.cheapExpr(r)
}

// stmt orders the statement n, appending to o.out.
func (o *orderState) stmt(n ir.Node) {
	if n == nil {
		return
	}

	lno := ir.SetPos(n)
	o.init(n)

	switch n.Op() {
	default:
		base.Fatalf("order.stmt %v", n.Op())

	case ir.OINLMARK:
		o.out = append(o.out, n)

	case ir.OAS:
		n := n.(*ir.AssignStmt)
		t := o.markTemp()

		// There's a delicate interaction here between two OINDEXMAP
		// optimizations.
		//
		// First, we want to handle m[k] = append(m[k], ...) with a single
		// runtime call to mapassign. This requires the m[k] expressions to
		// satisfy ir.SameSafeExpr in walkAssign.
		//
		// But if k is a slow map key type that's passed by reference (e.g.,
		// byte), then we want to avoid marking user variables as addrtaken,
		// if that might prevent the compiler from keeping k in a register.
		//
		// TODO(mdempsky): It would be better if walk was responsible for
		// inserting temporaries as needed.
		mapAppend := n.X.Op() == ir.OINDEXMAP && n.Y.Op() == ir.OAPPEND &&
			ir.SameSafeExpr(n.X, n.Y.(*ir.CallExpr).Args[0])

		n.X = o.expr(n.X, nil)
		if mapAppend {
			indexLHS := n.X.(*ir.IndexExpr)
			indexLHS.X = o.cheapExpr(indexLHS.X)
			indexLHS.Index = o.cheapExpr(indexLHS.Index)

			call := n.Y.(*ir.CallExpr)
			arg0 := call.Args[0]
			// ir.SameSafeExpr skips OCONVNOPs, so we must do the same here (#66096).
			for arg0.Op() == ir.OCONVNOP {
				arg0 = arg0.(*ir.ConvExpr).X
			}
			indexRHS := arg0.(*ir.IndexExpr)
			indexRHS.X = indexLHS.X
			indexRHS.Index = indexLHS.Index

			o.exprList(call.Args[1:])
		} else {
			n.Y = o.expr(n.Y, n.X)
		}
		o.mapAssign(n)
		o.popTemp(t)

	case ir.OASOP:
		n := n.(*ir.AssignOpStmt)
		t := o.markTemp()
		n.X = o.expr(n.X, nil)
		n.Y = o.expr(n.Y, nil)

		if base.Flag.Cfg.Instrumenting || n.X.Op() == ir.OINDEXMAP && (n.AsOp == ir.ODIV || n.AsOp == ir.OMOD) {
			// Rewrite m[k] op= r into m[k] = m[k] op r so
			// that we can ensure that if op panics
			// because r is zero, the panic happens before
			// the map assignment.
			// DeepCopy is a big hammer here, but safeExpr
			// makes sure there is nothing too deep being copied.
			l1 := o.safeExpr(n.X)
			l2 := ir.DeepCopy(src.NoXPos, l1)
			if l2.Op() == ir.OINDEXMAP {
				l2 := l2.(*ir.IndexExpr)
				l2.Assigned = false
			}
			l2 = o.copyExpr(l2)
			r := o.expr(typecheck.Expr(ir.NewBinaryExpr(n.Pos(), n.AsOp, l2, n.Y)), nil)
			as := typecheck.Stmt(ir.NewAssignStmt(n.Pos(), l1, r))
			o.mapAssign(as)
			o.popTemp(t)
			return
		}

		o.mapAssign(n)
		o.popTemp(t)

	case ir.OAS2:
		n := n.(*ir.AssignListStmt)
		t := o.markTemp()
		o.exprList(n.Lhs)
		o.exprList(n.Rhs)
		o.out = append(o.out, n)
		o.popTemp(t)

	// Special: avoid copy of func call n.Right
	case ir.OAS2FUNC:
		n := n.(*ir.AssignListStmt)
		t := o.markTemp()
		o.exprList(n.Lhs)
		call := n.Rhs[0]
		o.init(call)
		if ic, ok := call.(*ir.InlinedCallExpr); ok {
			o.stmtList(ic.Body)

			n.SetOp(ir.OAS2)
			n.Rhs = ic.ReturnVars

			o.exprList(n.Rhs)
			o.out = append(o.out, n)
		} else {
			o.call(call)
			o.as2func(n)
		}
		o.popTemp(t)

	// Special: use temporary variables to hold result,
	// so that runtime can take address of temporary.
	// No temporary for blank assignment.
	//
	// OAS2MAPR: make sure key is addressable if needed,
	//           and make sure OINDEXMAP is not copied out.
	case ir.OAS2DOTTYPE, ir.OAS2RECV, ir.OAS2MAPR:
		n := n.(*ir.AssignListStmt)
		t := o.markTemp()
		o.exprList(n.Lhs)

		switch r := n.Rhs[0]; r.Op() {
		case ir.ODOTTYPE2:
			r := r.(*ir.TypeAssertExpr)
			r.X = o.expr(r.X, nil)
		case ir.ODYNAMICDOTTYPE2:
			r := r.(*ir.DynamicTypeAssertExpr)
			r.X = o.expr(r.X, nil)
			r.RType = o.expr(r.RType, nil)
			r.ITab = o.expr(r.ITab, nil)
		case ir.ORECV:
			r := r.(*ir.UnaryExpr)
			r.X = o.expr(r.X, nil)
		case ir.OINDEXMAP:
			r := r.(*ir.IndexExpr)
			r.X = o.expr(r.X, nil)
			r.Index = o.expr(r.Index, nil)
			// See similar conversion for OINDEXMAP below.
			_ = mapKeyReplaceStrConv(r.Index)
			r.Index = o.mapKeyTemp(r.Pos(), r.X.Type(), r.Index)
		default:
			base.Fatalf("order.stmt: %v", r.Op())
		}

		o.as2ok(n)
		o.popTemp(t)

	// Special: does not save n onto out.
	case ir.OBLOCK:
		n := n.(*ir.BlockStmt)
		o.stmtList(n.List)

	// Special: n->left is not an expression; save as is.
	case ir.OBREAK,
		ir.OCONTINUE,
		ir.ODCL,
		ir.OFALL,
		ir.OGOTO,
		ir.OLABEL,
		ir.OTAILCALL:
		o.out = append(o.out, n)

	// Special: handle call arguments.
	case ir.OCALLFUNC, ir.OCALLINTER:
		n := n.(*ir.CallExpr)
		t := o.markTemp()
		o.call(n)
		o.out = append(o.out, n)
		o.popTemp(t)

	case ir.OINLCALL:
		n := n.(*ir.InlinedCallExpr)
		o.stmtList(n.Body)

		// discard results; double-check for no side effects
		for _, result := range n.ReturnVars {
			if staticinit.AnySideEffects(result) {
				base.FatalfAt(result.Pos(), "inlined call result has side effects: %v", result)
			}
		}

	case ir.OCHECKNIL, ir.OCLEAR, ir.OCLOSE, ir.OPANIC, ir.ORECV:
		n := n.(*ir.UnaryExpr)
		t := o.markTemp()
		n.X = o.expr(n.X, nil)
		o.out = append(o.out, n)
		o.popTemp(t)

	case ir.OCOPY:
		n := n.(*ir.BinaryExpr)
		t := o.markTemp()
		n.X = o.expr(n.X, nil)
		n.Y = o.expr(n.Y, nil)
		o.out = append(o.out, n)
		o.popTemp(t)

	case ir.OPRINT, ir.OPRINTLN, ir.ORECOVER:
		n := n.(*ir.CallExpr)
		t := o.markTemp()
		o.call(n)
		o.out = append(o.out, n)
		o.popTemp(t)

	// Special: order arguments to inner call but not call itself.
	case ir.ODEFER, ir.OGO:
		n := n.(*ir.GoDeferStmt)
		t := o.markTemp()
		o.init(n.Call)
		o.call(n.Call)
		o.out = append(o.out, n)
		o.popTemp(t)

	case ir.ODELETE:
		n := n.(*ir.CallExpr)
		t := o.markTemp()
		n.Args[0] = o.expr(n.Args[0], nil)
		n.Args[1] = o.expr(n.Args[1], nil)
		n.Args[1] = o.mapKeyTemp(n.Pos(), n.Args[0].Type(), n.Args[1])
		o.out = append(o.out, n)
		o.popTemp(t)

	// Clean temporaries from condition evaluation at
	// beginning of loop body and after for statement.
	case ir.OFOR:
		n := n.(*ir.ForStmt)
		t := o.markTemp()
		n.Cond = o.exprInPlace(n.Cond)
		orderBlock(&n.Body, o.free)
		n.Post = orderStmtInPlace(n.Post, o.free)
		o.out = append(o.out, n)
		o.popTemp(t)

	// Clean temporaries from condition at
	// beginning of both branches.
	case ir.OIF:
		n := n.(*ir.IfStmt)
		t := o.markTemp()
		n.Cond = o.exprInPlace(n.Cond)
		o.popTemp(t)
		orderBlock(&n.Body, o.free)
		orderBlock(&n.Else, o.free)
		o.out = append(o.out, n)

	case ir.ORANGE:
		// n.Right is the expression being ranged over.
		// order it, and then make a copy if we need one.
		// We almost always do, to ensure that we don't
		// see any value changes made during the loop.
		// Usually the copy is cheap (e.g., array pointer,
		// chan, slice, string are all tiny).
		// The exception is ranging over an array value
		// (not a slice, not a pointer to array),
		// which must make a copy to avoid seeing updates made during
		// the range body. Ranging over an array value is uncommon though.

		// Mark []byte(str) range expression to reuse string backing storage.
		// It is safe because the storage cannot be mutated.
		n := n.(*ir.RangeStmt)
		if x, ok := n.X.(*ir.ConvExpr); ok {
			switch x.Op() {
			case ir.OSTR2BYTES:
				x.SetOp(ir.OSTR2BYTESTMP)
				fallthrough
			case ir.OSTR2BYTESTMP:
				x.MarkNonNil() // "range []byte(nil)" is fine
			}
		}

		t := o.markTemp()
		n.X = o.expr(n.X, nil)

		orderBody := true
		xt := typecheck.RangeExprType(n.X.Type())
		switch k := xt.Kind(); {
		default:
			base.Fatalf("order.stmt range %v", n.Type())

		case types.IsInt[k]:
			// Used only once, no need to copy.

		case k == types.TARRAY, k == types.TSLICE:
			if n.Value == nil || ir.IsBlank(n.Value) {
				// for i := range x will only use x once, to compute len(x).
				// No need to copy it.
				break
			}
			fallthrough

		case k == types.TCHAN, k == types.TSTRING:
			// chan, string, slice, array ranges use value multiple times.
			// make copy.
			r := n.X

			if r.Type().IsString() && r.Type() != types.Types[types.TSTRING] {
				r = ir.NewConvExpr(base.Pos, ir.OCONV, nil, r)
				r.SetType(types.Types[types.TSTRING])
				r = typecheck.Expr(r)
			}

			n.X = o.copyExpr(r)

		case k == types.TMAP:
			if isMapClear(n) {
				// Preserve the body of the map clear pattern so it can
				// be detected during walk. The loop body will not be used
				// when optimizing away the range loop to a runtime call.
				orderBody = false
				break
			}

			// copy the map value in case it is a map literal.
			// TODO(rsc): Make tmp = literal expressions reuse tmp.
			// For maps tmp is just one word so it hardly matters.
			r := n.X
			n.X = o.copyExpr(r)

			// n.Prealloc is the temp for the iterator.
			// MapIterType contains pointers and needs to be zeroed.
			n.Prealloc = o.newTemp(reflectdata.MapIterType(), true)
		}
		n.Key = o.exprInPlace(n.Key)
		n.Value = o.exprInPlace(n.Value)
		if orderBody {
			orderBlock(&n.Body, o.free)
		}
		o.out = append(o.out, n)
		o.popTemp(t)

	case ir.ORETURN:
		n := n.(*ir.ReturnStmt)
		o.exprList(n.Results)
		o.out = append(o.out, n)

	// Special: clean case temporaries in each block entry.
	// Select must enter one of its blocks, so there is no
	// need for a cleaning at the end.
	// Doubly special: evaluation order for select is stricter
	// than ordinary expressions. Even something like p.c
	// has to be hoisted into a temporary, so that it cannot be
	// reordered after the channel evaluation for a different
	// case (if p were nil, then the timing of the fault would
	// give this away).
	case ir.OSELECT:
		n := n.(*ir.SelectStmt)
		t := o.markTemp()
		for _, ncas := range n.Cases {
			r := ncas.Comm
			ir.SetPos(ncas)

			// Append any new body prologue to ninit.
			// The next loop will insert ninit into nbody.
			if len(ncas.Init()) != 0 {
				base.Fatalf("order select ninit")
			}
			if r == nil {
				continue
			}
			switch r.Op() {
			default:
				ir.Dump("select case", r)
				base.Fatalf("unknown op in select %v", r.Op())

			case ir.OSELRECV2:
				// case x, ok = <-c
				r := r.(*ir.AssignListStmt)
				recv := r.Rhs[0].(*ir.UnaryExpr)
				recv.X = o.expr(recv.X, nil)
				if !ir.IsAutoTmp(recv.X) {
					recv.X = o.copyExpr(recv.X)
				}
				init := ir.TakeInit(r)

				colas := r.Def
				do := func(i int, t *types.Type) {
					n := r.Lhs[i]
					if ir.IsBlank(n) {
						return
					}
					// If this is case x := <-ch or case x, y := <-ch, the case has
					// the ODCL nodes to declare x and y. We want to delay that
					// declaration (and possible allocation) until inside the case body.
					// Delete the ODCL nodes here and recreate them inside the body below.
					if colas {
						if len(init) > 0 && init[0].Op() == ir.ODCL && init[0].(*ir.Decl).X == n {
							init = init[1:]

							// iimport may have added a default initialization assignment,
							// due to how it handles ODCL statements.
							if len(init) > 0 && init[0].Op() == ir.OAS && init[0].(*ir.AssignStmt).X == n {
								init = init[1:]
							}
						}
						dcl := typecheck.Stmt(ir.NewDecl(base.Pos, ir.ODCL, n.(*ir.Name)))
						ncas.PtrInit().Append(dcl)
					}
					tmp := o.newTemp(t, t.HasPointers())
					as := typecheck.Stmt(ir.NewAssignStmt(base.Pos, n, typecheck.Conv(tmp, n.Type())))
					ncas.PtrInit().Append(as)
					r.Lhs[i] = tmp
				}
				do(0, recv.X.Type().Elem())
				do(1, types.Types[types.TBOOL])
				if len(init) != 0 {
					ir.DumpList("ninit", init)
					base.Fatalf("ninit on select recv")
				}
				orderBlock(ncas.PtrInit(), o.free)

			case ir.OSEND:
				r := r.(*ir.SendStmt)
				if len(r.Init()) != 0 {
					ir.DumpList("ninit", r.Init())
					base.Fatalf("ninit on select send")
				}

				// case c <- x
				// r->left is c, r->right is x, both are always evaluated.
				r.Chan = o.expr(r.Chan, nil)

				if !ir.IsAutoTmp(r.Chan) {
					r.Chan = o.copyExpr(r.Chan)
				}
				r.Value = o.expr(r.Value, nil)
				if !ir.IsAutoTmp(r.Value) {
					r.Value = o.copyExpr(r.Value)
				}
			}
		}
		// Now that we have accumulated all the temporaries, clean them.
		// Also insert any ninit queued during the previous loop.
		// (The temporary cleaning must follow that ninit work.)
		for _, cas := range n.Cases {
			orderBlock(&cas.Body, o.free)

			// TODO(mdempsky): Is this actually necessary?
			// walkSelect appears to walk Ninit.
			cas.Body.Prepend(ir.TakeInit(cas)...)
		}

		o.out = append(o.out, n)
		o.popTemp(t)

	// Special: value being sent is passed as a pointer; make it addressable.
	case ir.OSEND:
		n := n.(*ir.SendStmt)
		t := o.markTemp()
		n.Chan = o.expr(n.Chan, nil)
		n.Value = o.expr(n.Value, nil)
		if base.Flag.Cfg.Instrumenting {
			// Force copying to the stack so that (chan T)(nil) <- x
			// is still instrumented as a read of x.
			n.Value = o.copyExpr(n.Value)
		} else {
			n.Value = o.addrTemp(n.Value)
		}
		o.out = append(o.out, n)
		o.popTemp(t)

	// TODO(rsc): Clean temporaries more aggressively.
	// Note that because walkSwitch will rewrite some of the
	// switch into a binary search, this is not as easy as it looks.
	// (If we ran that code here we could invoke order.stmt on
	// the if-else chain instead.)
	// For now just clean all the temporaries at the end.
	// In practice that's fine.
	case ir.OSWITCH:
		n := n.(*ir.SwitchStmt)
		if base.Debug.Libfuzzer != 0 && !hasDefaultCase(n) {
			// Add empty "default:" case for instrumentation.
			n.Cases = append(n.Cases, ir.NewCaseStmt(base.Pos, nil, nil))
		}

		t := o.markTemp()
		n.Tag = o.expr(n.Tag, nil)
		for _, ncas := range n.Cases {
			o.exprListInPlace(ncas.List)
			orderBlock(&ncas.Body, o.free)
		}

		o.out = append(o.out, n)
		o.popTemp(t)
	}

	base.Pos = lno
}

func hasDefaultCase(n *ir.SwitchStmt) bool {
	for _, ncas := range n.Cases {
		if len(ncas.List) == 0 {
			return true
		}
	}
	return false
}

// exprList orders the expression list l into o.
func (o *orderState) exprList(l ir.Nodes) {
	s := l
	for i := range s {
		s[i] = o.expr(s[i], nil)
	}
}

// exprListInPlace orders the expression list l but saves
// the side effects on the individual expression ninit lists.
func (o *orderState) exprListInPlace(l ir.Nodes) {
	s := l
	for i := range s {
		s[i] = o.exprInPlace(s[i])
	}
}

func (o *orderState) exprNoLHS(n ir.Node) ir.Node {
	return o.expr(n, nil)
}

// expr orders a single expression, appending side
// effects to o.out as needed.
// If this is part of an assignment lhs = *np, lhs is given.
// Otherwise lhs == nil. (When lhs != nil it may be possible
// to avoid copying the result of the expression to a temporary.)
// The result of expr MUST be assigned back to n, e.g.
//
//	n.Left = o.expr(n.Left, lhs)
func (o *orderState) expr(n, lhs ir.Node) ir.Node {
	if n == nil {
		return n
	}
	lno := ir.SetPos(n)
	n = o.expr1(n, lhs)
	base.Pos = lno
	return n
}

func (o *orderState) expr1(n, lhs ir.Node) ir.Node {
	o.init(n)

	switch n.Op() {
	default:
		if o.edit == nil {
			o.edit = o.exprNoLHS // create closure once
		}
		ir.EditChildren(n, o.edit)
		return n

	// Addition of strings turns into a function call.
	// Allocate a temporary to hold the strings.
	// Fewer than 5 strings use direct runtime helpers.
	case ir.OADDSTR:
		n := n.(*ir.AddStringExpr)
		o.exprList(n.List)

		if len(n.List) > 5 {
			t := types.NewArray(types.Types[types.TSTRING], int64(len(n.List)))
			n.Prealloc = o.newTemp(t, false)
		}

		// Mark string(byteSlice) arguments to reuse byteSlice backing
		// buffer during conversion. String concatenation does not
		// memorize the strings for later use, so it is safe.
		// However, we can do it only if there is at least one non-empty string literal.
		// Otherwise if all other arguments are empty strings,
		// concatstrings will return the reference to the temp string
		// to the caller.
		hasbyte := false

		haslit := false
		for _, n1 := range n.List {
			hasbyte = hasbyte || n1.Op() == ir.OBYTES2STR
			haslit = haslit || n1.Op() == ir.OLITERAL && len(ir.StringVal(n1)) != 0
		}

		if haslit && hasbyte {
			for _, n2 := range n.List {
				if n2.Op() == ir.OBYTES2STR {
					n2 := n2.(*ir.ConvExpr)
					n2.SetOp(ir.OBYTES2STRTMP)
				}
			}
		}
		return n

	case ir.OINDEXMAP:
		n := n.(*ir.IndexExpr)
		n.X = o.expr(n.X, nil)
		n.Index = o.expr(n.Index, nil)
		needCopy := false

		if !n.Assigned {
			// Enforce that any []byte slices we are not copying
			// can not be changed before the map index by forcing
			// the map index to happen immediately following the
			// conversions. See copyExpr a few lines below.
			needCopy = mapKeyReplaceStrConv(n.Index)

			if base.Flag.Cfg.Instrumenting {
				// Race detector needs the copy.
				needCopy = true
			}
		}

		// key may need to be addressable
		n.Index = o.mapKeyTemp(n.Pos(), n.X.Type(), n.Index)
		if needCopy {
			return o.copyExpr(n)
		}
		return n

	// concrete type (not interface) argument might need an addressable
	// temporary to pass to the runtime conversion routine.
	case ir.OCONVIFACE:
		n := n.(*ir.ConvExpr)
		n.X = o.expr(n.X, nil)
		if n.X.Type().IsInterface() {
			return n
		}
		if _, _, needsaddr := dataWordFuncName(n.X.Type()); needsaddr || isStaticCompositeLiteral(n.X) {
			// Need a temp if we need to pass the address to the conversion function.
			// We also process static composite literal node here, making a named static global
			// whose address we can put directly in an interface (see OCONVIFACE case in walk).
			n.X = o.addrTemp(n.X)
		}
		return n

	case ir.OCONVNOP:
		n := n.(*ir.ConvExpr)
		if n.X.Op() == ir.OCALLMETH {
			base.FatalfAt(n.X.Pos(), "OCALLMETH missed by typecheck")
		}
		if n.Type().IsKind(types.TUNSAFEPTR) && n.X.Type().IsKind(types.TUINTPTR) && (n.X.Op() == ir.OCALLFUNC || n.X.Op() == ir.OCALLINTER) {
			call := n.X.(*ir.CallExpr)
			// When reordering unsafe.Pointer(f()) into a separate
			// statement, the conversion and function call must stay
			// together. See golang.org/issue/15329.
			o.init(call)
			o.call(call)
			if lhs == nil || lhs.Op() != ir.ONAME || base.Flag.Cfg.Instrumenting {
				return o.copyExpr(n)
			}
		} else {
			n.X = o.expr(n.X, nil)
		}
		return n

	case ir.OANDAND, ir.OOROR:
		// ... = LHS && RHS
		//
		// var r bool
		// r = LHS
		// if r {       // or !r, for OROR
		//     r = RHS
		// }
		// ... = r

		n := n.(*ir.LogicalExpr)
		r := o.newTemp(n.Type(), false)

		// Evaluate left-hand side.
		lhs := o.expr(n.X, nil)
		o.out = append(o.out, typecheck.Stmt(ir.NewAssignStmt(base.Pos, r, lhs)))

		// Evaluate right-hand side, save generated code.
		saveout := o.out
		o.out = nil
		t := o.markTemp()
		o.edge()
		rhs := o.expr(n.Y, nil)
		o.out = append(o.out, typecheck.Stmt(ir.NewAssignStmt(base.Pos, r, rhs)))
		o.popTemp(t)
		gen := o.out
		o.out = saveout

		// If left-hand side doesn't cause a short-circuit, issue right-hand side.
		nif := ir.NewIfStmt(base.Pos, r, nil, nil)
		if n.Op() == ir.OANDAND {
			nif.Body = gen
		} else {
			nif.Else = gen
		}
		o.out = append(o.out, nif)
		return r

	case ir.OCALLMETH:
		base.FatalfAt(n.Pos(), "OCALLMETH missed by typecheck")
		panic("unreachable")

	case ir.OCALLFUNC,
		ir.OCALLINTER,
		ir.OCAP,
		ir.OCOMPLEX,
		ir.OCOPY,
		ir.OIMAG,
		ir.OLEN,
		ir.OMAKECHAN,
		ir.OMAKEMAP,
		ir.OMAKESLICE,
		ir.OMAKESLICECOPY,
		ir.OMAX,
		ir.OMIN,
		ir.ONEW,
		ir.OREAL,
		ir.ORECOVER,
		ir.OSTR2BYTES,
		ir.OSTR2BYTESTMP,
		ir.OSTR2RUNES:

		if isRuneCount(n) {
			// len([]rune(s)) is rewritten to runtime.countrunes(s) later.
			conv := n.(*ir.UnaryExpr).X.(*ir.ConvExpr)
			conv.X = o.expr(conv.X, nil)
		} else {
			o.call(n)
		}

		if lhs == nil || lhs.Op() != ir.ONAME || base.Flag.Cfg.Instrumenting {
			return o.copyExpr(n)
		}
		return n

	case ir.OINLCALL:
		n := n.(*ir.InlinedCallExpr)
		o.stmtList(n.Body)
		return n.SingleResult()

	case ir.OAPPEND:
		// Check for append(x, make([]T, y)...) .
		n := n.(*ir.CallExpr)
		if isAppendOfMake(n) {
			n.Args[0] = o.expr(n.Args[0], nil) // order x
			mk := n.Args[1].(*ir.MakeExpr)
			mk.Len = o.expr(mk.Len, nil) // order y
		} else {
			o.exprList(n.Args)
		}

		if lhs == nil || lhs.Op() != ir.ONAME && !ir.SameSafeExpr(lhs, n.Args[0]) {
			return o.copyExpr(n)
		}
		return n

	case ir.OSLICE, ir.OSLICEARR, ir.OSLICESTR, ir.OSLICE3, ir.OSLICE3ARR:
		n := n.(*ir.SliceExpr)
		n.X = o.expr(n.X, nil)
		n.Low = o.cheapExpr(o.expr(n.Low, nil))
		n.High = o.cheapExpr(o.expr(n.High, nil))
		n.Max = o.cheapExpr(o.expr(n.Max, nil))
		if lhs == nil || lhs.Op() != ir.ONAME && !ir.SameSafeExpr(lhs, n.X) {
			return o.copyExpr(n)
		}
		return n

	case ir.OCLOSURE:
		n := n.(*ir.ClosureExpr)
		if n.Transient() && len(n.Func.ClosureVars) > 0 {
			n.Prealloc = o.newTemp(typecheck.ClosureType(n), false)
		}
		return n

	case ir.OMETHVALUE:
		n := n.(*ir.SelectorExpr)
		n.X = o.expr(n.X, nil)
		if n.Transient() {
			t := typecheck.MethodValueType(n)
			n.Prealloc = o.newTemp(t, false)
		}
		return n

	case ir.OSLICELIT:
		n := n.(*ir.CompLitExpr)
		o.exprList(n.List)
		if n.Transient() {
			t := types.NewArray(n.Type().Elem(), n.Len)
			n.Prealloc = o.newTemp(t, false)
		}
		return n

	case ir.ODOTTYPE, ir.ODOTTYPE2:
		n := n.(*ir.TypeAssertExpr)
		n.X = o.expr(n.X, nil)
		if !types.IsDirectIface(n.Type()) || base.Flag.Cfg.Instrumenting {
			return o.copyExprClear(n)
		}
		return n

	case ir.ORECV:
		n := n.(*ir.UnaryExpr)
		n.X = o.expr(n.X, nil)
		return o.copyExprClear(n)

	case ir.OEQ, ir.ONE, ir.OLT, ir.OLE, ir.OGT, ir.OGE:
		n := n.(*ir.BinaryExpr)
		n.X = o.expr(n.X, nil)
		n.Y = o.expr(n.Y, nil)

		t := n.X.Type()
		switch {
		case t.IsString():
			// Mark string(byteSlice) arguments to reuse byteSlice backing
			// buffer during conversion. String comparison does not
			// memorize the strings for later use, so it is safe.
			if n.X.Op() == ir.OBYTES2STR {
				n.X.(*ir.ConvExpr).SetOp(ir.OBYTES2STRTMP)
			}
			if n.Y.Op() == ir.OBYTES2STR {
				n.Y.(*ir.ConvExpr).SetOp(ir.OBYTES2STRTMP)
			}

		case t.IsStruct() || t.IsArray():
			// for complex comparisons, we need both args to be
			// addressable so we can pass them to the runtime.
			n.X = o.addrTemp(n.X)
			n.Y = o.addrTemp(n.Y)
		}
		return n

	case ir.OMAPLIT:
		// Order map by converting:
		//   map[int]int{
		//     a(): b(),
		//     c(): d(),
		//     e(): f(),
		//   }
		// to
		//   m := map[int]int{}
		//   m[a()] = b()
		//   m[c()] = d()
		//   m[e()] = f()
		// Then order the result.
		// Without this special case, order would otherwise compute all
		// the keys and values before storing any of them to the map.
		// See issue 26552.
		n := n.(*ir.CompLitExpr)
		entries := n.List
		statics := entries[:0]
		var dynamics []*ir.KeyExpr
		for _, r := range entries {
			r := r.(*ir.KeyExpr)

			if !isStaticCompositeLiteral(r.Key) || !isStaticCompositeLiteral(r.Value) {
				dynamics = append(dynamics, r)
				continue
			}

			// Recursively ordering some static entries can change them to dynamic;
			// e.g., OCONVIFACE nodes. See #31777.
			r = o.expr(r, nil).(*ir.KeyExpr)
			if !isStaticCompositeLiteral(r.Key) || !isStaticCompositeLiteral(r.Value) {
				dynamics = append(dynamics, r)
				continue
			}

			statics = append(statics, r)
		}
		n.List = statics

		if len(dynamics) == 0 {
			return n
		}

		// Emit the creation of the map (with all its static entries).
		m := o.newTemp(n.Type(), false)
		as := ir.NewAssignStmt(base.Pos, m, n)
		typecheck.Stmt(as)
		o.stmt(as)

		// Emit eval+insert of dynamic entries, one at a time.
		for _, r := range dynamics {
			lhs := typecheck.AssignExpr(ir.NewIndexExpr(base.Pos, m, r.Key)).(*ir.IndexExpr)
			base.AssertfAt(lhs.Op() == ir.OINDEXMAP, lhs.Pos(), "want OINDEXMAP, have %+v", lhs)
			lhs.RType = n.RType

			as := ir.NewAssignStmt(base.Pos, lhs, r.Value)
			typecheck.Stmt(as)
			o.stmt(as)
		}

		// Remember that we issued these assignments so we can include that count
		// in the map alloc hint.
		// We're assuming here that all the keys in the map literal are distinct.
		// If any are equal, this will be an overcount. Probably not worth accounting
		// for that, as equal keys in map literals are rare, and at worst we waste
		// a bit of space.
		n.Len += int64(len(dynamics))

		return m
	}

	// No return - type-assertions above. Each case must return for itself.
}

// as2func orders OAS2FUNC nodes. It creates temporaries to ensure left-to-right assignment.
// The caller should order the right-hand side of the assignment before calling order.as2func.
// It rewrites,
//
//	a, b, a = ...
//
// as
//
//	tmp1, tmp2, tmp3 = ...
//	a, b, a = tmp1, tmp2, tmp3
//
// This is necessary to ensure left to right assignment order.
func (o *orderState) as2func(n *ir.AssignListStmt) {
	results := n.Rhs[0].Type()
	as := ir.NewAssignListStmt(n.Pos(), ir.OAS2, nil, nil)
	for i, nl := range n.Lhs {
		if !ir.IsBlank(nl) {
			typ := results.Field(i).Type
			tmp := o.newTemp(typ, typ.HasPointers())
			n.Lhs[i] = tmp
			as.Lhs = append(as.Lhs, nl)
			as.Rhs = append(as.Rhs, tmp)
		}
	}

	o.out = append(o.out, n)
	o.stmt(typecheck.Stmt(as))
}

// as2ok orders OAS2XXX with ok.
// Just like as2func, this also adds temporaries to ensure left-to-right assignment.
func (o *orderState) as2ok(n *ir.AssignListStmt) {
	as := ir.NewAssignListStmt(n.Pos(), ir.OAS2, nil, nil)

	do := func(i int, typ *types.Type) {
		if nl := n.Lhs[i]; !ir.IsBlank(nl) {
			var tmp ir.Node = o.newTemp(typ, typ.HasPointers())
			n.Lhs[i] = tmp
			as.Lhs = append(as.Lhs, nl)
			if i == 1 {
				// The "ok" result is an untyped boolean according to the Go
				// spec. We need to explicitly convert it to the LHS type in
				// case the latter is a defined boolean type (#8475).
				tmp = typecheck.Conv(tmp, nl.Type())
			}
			as.Rhs = append(as.Rhs, tmp)
		}
	}

	do(0, n.Rhs[0].Type())
	do(1, types.Types[types.TBOOL])

	o.out = append(o.out, n)
	o.stmt(typecheck.Stmt(as))
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/range.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"go/constant"
	"unicode/utf8"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/ssagen"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/src"
	"cmd/internal/sys"
)

func cheapComputableIndex(width int64) bool {
	switch ssagen.Arch.LinkArch.Family {
	// MIPS does not have R+R addressing
	// Arm64 may lack ability to generate this code in our assembler,
	// but the architecture supports it.
	case sys.Loong64, sys.PPC64, sys.S390X:
		return width == 1
	case sys.AMD64, sys.I386, sys.ARM64, sys.ARM:
		switch width {
		case 1, 2, 4, 8:
			return true
		}
	}
	return false
}

// walkRange transforms various forms of ORANGE into
// simpler forms.  The result must be assigned back to n.
// Node n may also be modified in place, and may also be
// the returned node.
func walkRange(nrange *ir.RangeStmt) ir.Node {
	base.Assert(!nrange.DistinctVars) // Should all be rewritten before escape analysis
	if isMapClear(nrange) {
		return mapRangeClear(nrange)
	}

	nfor := ir.NewForStmt(nrange.Pos(), nil, nil, nil, nil, nrange.DistinctVars)
	nfor.SetInit(nrange.Init())
	nfor.Label = nrange.Label

	// variable name conventions:
	//	ohv1, hv1, hv2: hidden (old) val 1, 2
	//	ha, hit: hidden aggregate, iterator
	//	hn, hp: hidden len, pointer
	//	hb: hidden bool
	//	a, v1, v2: not hidden aggregate, val 1, 2

	a := nrange.X
	t := a.Type()
	lno := ir.SetPos(a)

	v1, v2 := nrange.Key, nrange.Value

	if ir.IsBlank(v2) {
		v2 = nil
	}

	if ir.IsBlank(v1) && v2 == nil {
		v1 = nil
	}

	if v1 == nil && v2 != nil {
		base.Fatalf("walkRange: v2 != nil while v1 == nil")
	}

	var body []ir.Node
	var init []ir.Node
	switch k := t.Kind(); {
	default:
		base.Fatalf("walkRange")

	case types.IsInt[k]:
		if nn := arrayRangeClear(nrange, v1, v2, a); nn != nil {
			base.Pos = lno
			return nn
		}
		hv1 := typecheck.TempAt(base.Pos, ir.CurFunc, t)
		hn := typecheck.TempAt(base.Pos, ir.CurFunc, t)

		init = append(init, ir.NewAssignStmt(base.Pos, hv1, nil))
		init = append(init, ir.NewAssignStmt(base.Pos, hn, a))

		nfor.Cond = ir.NewBinaryExpr(base.Pos, ir.OLT, hv1, hn)
		nfor.Post = ir.NewAssignStmt(base.Pos, hv1, ir.NewBinaryExpr(base.Pos, ir.OADD, hv1, ir.NewInt(base.Pos, 1)))

		if v1 != nil {
			body = []ir.Node{rangeAssign(nrange, hv1)}
		}

	case k == types.TARRAY, k == types.TSLICE, k == types.TPTR: // TPTR is pointer-to-array
		if nn := arrayRangeClear(nrange, v1, v2, a); nn != nil {
			base.Pos = lno
			return nn
		}

		// Element type of the iteration
		var elem *types.Type
		switch t.Kind() {
		case types.TSLICE, types.TARRAY:
			elem = t.Elem()
		case types.TPTR:
			elem = t.Elem().Elem()
		}

		// order.stmt arranged for a copy of the array/slice variable if needed.
		ha := a

		hv1 := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])
		hn := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])

		init = append(init, ir.NewAssignStmt(base.Pos, hv1, nil))
		init = append(init, ir.NewAssignStmt(base.Pos, hn, ir.NewUnaryExpr(base.Pos, ir.OLEN, ha)))

		nfor.Cond = ir.NewBinaryExpr(base.Pos, ir.OLT, hv1, hn)
		nfor.Post = ir.NewAssignStmt(base.Pos, hv1, ir.NewBinaryExpr(base.Pos, ir.OADD, hv1, ir.NewInt(base.Pos, 1)))

		// for range ha { body }
		if v1 == nil {
			break
		}

		// for v1 := range ha { body }
		if v2 == nil {
			body = []ir.Node{rangeAssign(nrange, hv1)}
			break
		}

		// for v1, v2 := range ha { body }
		if cheapComputableIndex(elem.Size()) {
			// v1, v2 = hv1, ha[hv1]
			tmp := ir.NewIndexExpr(base.Pos, ha, hv1)
			tmp.SetBounded(true)
			body = []ir.Node{rangeAssign2(nrange, hv1, tmp)}
			break
		}

		// Slice to iterate over
		var hs ir.Node
		if t.IsSlice() {
			hs = ha
		} else {
			var arr ir.Node
			if t.IsPtr() {
				arr = ha
			} else {
				arr = typecheck.NodAddr(ha)
				arr.SetType(t.PtrTo())
				arr.SetTypecheck(1)
			}
			hs = ir.NewSliceExpr(base.Pos, ir.OSLICEARR, arr, nil, nil, nil)
			// old typechecker doesn't know OSLICEARR, so we set types explicitly
			hs.SetType(types.NewSlice(elem))
			hs.SetTypecheck(1)
		}

		// We use a "pointer" to keep track of where we are in the backing array
		// of the slice hs. This pointer starts at hs.ptr and gets incremented
		// by the element size each time through the loop.
		//
		// It's tricky, though, as on the last iteration this pointer gets
		// incremented to point past the end of the backing array. We can't
		// let the garbage collector see that final out-of-bounds pointer.
		//
		// To avoid this, we keep the "pointer" alternately in 2 variables, one
		// pointer typed and one uintptr typed. Most of the time it lives in the
		// regular pointer variable, but when it might be out of bounds (after it
		// has been incremented, but before the loop condition has been checked)
		// it lives briefly in the uintptr variable.
		//
		// hp contains the pointer version (of type *T, where T is the element type).
		// It is guaranteed to always be in range, keeps the backing store alive,
		// and is updated on stack copies. If a GC occurs when this function is
		// suspended at any safepoint, this variable ensures correct operation.
		//
		// hu contains the equivalent uintptr version. It may point past the
		// end, but doesn't keep the backing store alive and doesn't get updated
		// on a stack copy. If a GC occurs while this function is on the top of
		// the stack, then the last frame is scanned conservatively and hu will
		// act as a reference to the backing array to ensure it is not collected.
		//
		// The "pointer" we're moving across the backing array lives in one
		// or the other of hp and hu as the loop proceeds.
		//
		// hp is live during most of the body of the loop. But it isn't live
		// at the very top of the loop, when we haven't checked i<n yet, and
		// it could point off the end of the backing store.
		// hu is live only at the very top and very bottom of the loop.
		// In particular, only when it cannot possibly be live across a call.
		//
		// So we do
		//   hu = uintptr(unsafe.Pointer(hs.ptr))
		//   for i := 0; i < hs.len; i++ {
		//     hp = (*T)(unsafe.Pointer(hu))
		//     v1, v2 = i, *hp
		//     ... body of loop ...
		//     hu = uintptr(unsafe.Pointer(hp)) + elemsize
		//   }
		//
		// Between the assignments to hu and the assignment back to hp, there
		// must not be any calls.

		// Pointer to current iteration position. Start on entry to the loop
		// with the pointer in hu.
		ptr := ir.NewUnaryExpr(base.Pos, ir.OSPTR, hs)
		ptr.SetBounded(true)
		huVal := ir.NewConvExpr(base.Pos, ir.OCONVNOP, types.Types[types.TUNSAFEPTR], ptr)
		huVal = ir.NewConvExpr(base.Pos, ir.OCONVNOP, types.Types[types.TUINTPTR], huVal)
		hu := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TUINTPTR])
		init = append(init, ir.NewAssignStmt(base.Pos, hu, huVal))

		// Convert hu to hp at the top of the loop (after the condition has been checked).
		hpVal := ir.NewConvExpr(base.Pos, ir.OCONVNOP, types.Types[types.TUNSAFEPTR], hu)
		hpVal.SetCheckPtr(true) // disable checkptr on this conversion
		hpVal = ir.NewConvExpr(base.Pos, ir.OCONVNOP, elem.PtrTo(), hpVal)
		hp := typecheck.TempAt(base.Pos, ir.CurFunc, elem.PtrTo())
		body = append(body, ir.NewAssignStmt(base.Pos, hp, hpVal))

		// Assign variables on the LHS of the range statement. Use *hp to get the element.
		e := ir.NewStarExpr(base.Pos, hp)
		e.SetBounded(true)
		a := rangeAssign2(nrange, hv1, e)
		body = append(body, a)

		// Advance pointer for next iteration of the loop.
		// This reads from hp and writes to hu.
		huVal = ir.NewConvExpr(base.Pos, ir.OCONVNOP, types.Types[types.TUNSAFEPTR], hp)
		huVal = ir.NewConvExpr(base.Pos, ir.OCONVNOP, types.Types[types.TUINTPTR], huVal)
		as := ir.NewAssignStmt(base.Pos, hu, ir.NewBinaryExpr(base.Pos, ir.OADD, huVal, ir.NewInt(base.Pos, elem.Size())))
		nfor.Post = ir.NewBlockStmt(base.Pos, []ir.Node{nfor.Post, as})

	case k == types.TMAP:
		// order.stmt allocated the iterator for us.
		// we only use a once, so no copy needed.
		ha := a

		hit := nrange.Prealloc
		th := hit.Type()
		// depends on layout of iterator struct.
		// See cmd/compile/internal/reflectdata/map.go:MapIterType
		keysym := th.Field(0).Sym
		elemsym := th.Field(1).Sym // ditto
		iterInit := "mapIterStart"
		iterNext := "mapIterNext"

		fn := typecheck.LookupRuntime(iterInit, t.Key(), t.Elem(), th)
		init = append(init, mkcallstmt1(fn, reflectdata.RangeMapRType(base.Pos, nrange), ha, typecheck.NodAddr(hit)))
		nfor.Cond = ir.NewBinaryExpr(base.Pos, ir.ONE, ir.NewSelectorExpr(base.Pos, ir.ODOT, hit, keysym), typecheck.NodNil())

		fn = typecheck.LookupRuntime(iterNext, th)
		nfor.Post = mkcallstmt1(fn, typecheck.NodAddr(hit))

		key := ir.NewStarExpr(base.Pos, typecheck.ConvNop(ir.NewSelectorExpr(base.Pos, ir.ODOT, hit, keysym), types.NewPtr(t.Key())))
		if v1 == nil {
			body = nil
		} else if v2 == nil {
			body = []ir.Node{rangeAssign(nrange, key)}
		} else {
			elem := ir.NewStarExpr(base.Pos, typecheck.ConvNop(ir.NewSelectorExpr(base.Pos, ir.ODOT, hit, elemsym), types.NewPtr(t.Elem())))
			body = []ir.Node{rangeAssign2(nrange, key, elem)}
		}

	case k == types.TCHAN:
		// order.stmt arranged for a copy of the channel variable.
		ha := a

		hv1 := typecheck.TempAt(base.Pos, ir.CurFunc, t.Elem())
		hv1.SetTypecheck(1)
		if t.Elem().HasPointers() {
			init = append(init, ir.NewAssignStmt(base.Pos, hv1, nil))
		}
		hb := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TBOOL])

		nfor.Cond = ir.NewBinaryExpr(base.Pos, ir.ONE, hb, ir.NewBool(base.Pos, false))
		lhs := []ir.Node{hv1, hb}
		rhs := []ir.Node{ir.NewUnaryExpr(base.Pos, ir.ORECV, ha)}
		a := ir.NewAssignListStmt(base.Pos, ir.OAS2RECV, lhs, rhs)
		a.SetTypecheck(1)
		nfor.Cond = ir.InitExpr([]ir.Node{a}, nfor.Cond)
		if v1 == nil {
			body = nil
		} else {
			body = []ir.Node{rangeAssign(nrange, hv1)}
		}
		// Zero hv1. This prevents hv1 from being the sole, inaccessible
		// reference to an otherwise GC-able value during the next channel receive.
		// See issue 15281.
		body = append(body, ir.NewAssignStmt(base.Pos, hv1, nil))

	case k == types.TSTRING:
		// Transform string range statements like "for v1, v2 = range a" into
		//
		// ha := a
		// for hv1 := 0; hv1 < len(ha); {
		//   hv1t := hv1
		//   hv2 := rune(ha[hv1])
		//   if hv2 < utf8.RuneSelf {
		//      hv1++
		//   } else {
		//      hv2, hv1 = decoderune(ha, hv1)
		//   }
		//   v1, v2 = hv1t, hv2
		//   // original body
		// }

		// order.stmt arranged for a copy of the string variable.
		ha := a

		hv1 := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])
		hv1t := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])
		hv2 := typecheck.TempAt(base.Pos, ir.CurFunc, types.RuneType)

		// hv1 := 0
		init = append(init, ir.NewAssignStmt(base.Pos, hv1, nil))

		// hv1 < len(ha)
		nfor.Cond = ir.NewBinaryExpr(base.Pos, ir.OLT, hv1, ir.NewUnaryExpr(base.Pos, ir.OLEN, ha))

		if v1 != nil {
			// hv1t = hv1
			body = append(body, ir.NewAssignStmt(base.Pos, hv1t, hv1))
		}

		// hv2 := rune(ha[hv1])
		nind := ir.NewIndexExpr(base.Pos, ha, hv1)
		nind.SetBounded(true)
		body = append(body, ir.NewAssignStmt(base.Pos, hv2, typecheck.Conv(nind, types.RuneType)))

		// if hv2 < utf8.RuneSelf
		nif := ir.NewIfStmt(base.Pos, nil, nil, nil)

		// On x86, hv2 <= 127 is shorter to encode than hv2 < 128
		// Doesn't hurt other archs.
		nif.Cond = ir.NewBinaryExpr(base.Pos, ir.OLE, hv2, ir.NewInt(base.Pos, utf8.RuneSelf-1))

		// hv1++
		nif.Body = []ir.Node{ir.NewAssignStmt(base.Pos, hv1, ir.NewBinaryExpr(base.Pos, ir.OADD, hv1, ir.NewInt(base.Pos, 1)))}

		// } else {
		// hv2, hv1 = decoderune(ha, hv1)
		fn := typecheck.LookupRuntime("decoderune")
		// decoderune expects a uint, but hv1 is an int.
		// This is safe because hv1 is always >= 0.
		call := mkcall1(fn, fn.Type().ResultsTuple(), &nif.Else, ha, hv1)
		a := ir.NewAssignListStmt(base.Pos, ir.OAS2, []ir.Node{hv2, hv1}, []ir.Node{call})
		nif.Else.Append(a)

		body = append(body, nif)

		if v1 != nil {
			if v2 != nil {
				// v1, v2 = hv1t, hv2
				body = append(body, rangeAssign2(nrange, hv1t, hv2))
			} else {
				// v1 = hv1t
				body = append(body, rangeAssign(nrange, hv1t))
			}
		}
	}

	typecheck.Stmts(init)

	nfor.PtrInit().Append(init...)

	typecheck.Stmts(nfor.Cond.Init())

	nfor.Cond = typecheck.Expr(nfor.Cond)
	nfor.Cond = typecheck.DefaultLit(nfor.Cond, nil)
	nfor.Post = typecheck.Stmt(nfor.Post)
	typecheck.Stmts(body)
	nfor.Body.Append(body...)
	nfor.Body.Append(nrange.Body...)

	var n ir.Node = nfor

	n = walkStmt(n)

	base.Pos = lno
	return n
}

// rangeAssign returns "n.Key = key".
func rangeAssign(n *ir.RangeStmt, key ir.Node) ir.Node {
	key = rangeConvert(n, n.Key.Type(), key, n.KeyTypeWord, n.KeySrcRType)
	return ir.NewAssignStmt(n.Pos(), n.Key, key)
}

// rangeAssign2 returns "n.Key, n.Value = key, value".
func rangeAssign2(n *ir.RangeStmt, key, value ir.Node) ir.Node {
	// Use OAS2 to correctly handle assignments
	// of the form "v1, a[v1] = range".
	key = rangeConvert(n, n.Key.Type(), key, n.KeyTypeWord, n.KeySrcRType)
	value = rangeConvert(n, n.Value.Type(), value, n.ValueTypeWord, n.ValueSrcRType)
	return ir.NewAssignListStmt(n.Pos(), ir.OAS2, []ir.Node{n.Key, n.Value}, []ir.Node{key, value})
}

// rangeConvert returns src, converted to dst if necessary. If a
// conversion is necessary, then typeWord and srcRType are copied to
// their respective ConvExpr fields.
func rangeConvert(nrange *ir.RangeStmt, dst *types.Type, src, typeWord, srcRType ir.Node) ir.Node {
	src = typecheck.Expr(src)
	if dst.Kind() == types.TBLANK || types.Identical(dst, src.Type()) {
		return src
	}

	n := ir.NewConvExpr(nrange.Pos(), ir.OCONV, dst, src)
	n.TypeWord = typeWord
	n.SrcRType = srcRType
	return typecheck.Expr(n)
}

// isMapClear checks if n is of the form:
//
//	for k := range m {
//		delete(m, k)
//	}
//
// where == for keys of map m is reflexive.
func isMapClear(n *ir.RangeStmt) bool {
	if base.Flag.N != 0 || base.Flag.Cfg.Instrumenting {
		return false
	}

	t := n.X.Type()
	if n.Op() != ir.ORANGE || t.Kind() != types.TMAP || n.Key == nil || n.Value != nil {
		return false
	}

	k := n.Key
	// Require k to be a new variable name.
	if !ir.DeclaredBy(k, n) {
		return false
	}

	if len(n.Body) != 1 {
		return false
	}

	stmt := n.Body[0] // only stmt in body
	if stmt == nil || stmt.Op() != ir.ODELETE {
		return false
	}

	m := n.X
	if delete := stmt.(*ir.CallExpr); !ir.SameSafeExpr(delete.Args[0], m) || !ir.SameSafeExpr(delete.Args[1], k) {
		return false
	}

	// Keys where equality is not reflexive can not be deleted from maps.
	if !types.IsReflexive(t.Key()) {
		return false
	}

	return true
}

// mapRangeClear constructs a call to runtime.mapclear for the map range idiom.
func mapRangeClear(nrange *ir.RangeStmt) ir.Node {
	m := nrange.X
	origPos := ir.SetPos(m)
	defer func() { base.Pos = origPos }()

	return mapClear(m, reflectdata.RangeMapRType(base.Pos, nrange))
}

// mapClear constructs a call to runtime.mapclear for the map m.
func mapClear(m, rtyp ir.Node) ir.Node {
	t := m.Type()

	// instantiate mapclear(typ *type, hmap map[any]any)
	fn := typecheck.LookupRuntime("mapclear", t.Key(), t.Elem())
	n := mkcallstmt1(fn, rtyp, m)
	return typecheck.Stmt(n)
}

// Lower n into runtime·memclr if possible, for
// fast zeroing of slices and arrays (issue 5373).
// Look for instances of
//
//	for i := range a {
//		a[i] = zero
//	}
//
// in which the evaluation of a is side-effect-free.
//
// Parameters are as in walkRange: "for v1, v2 = range a".
func arrayRangeClear(loop *ir.RangeStmt, v1, v2, a ir.Node) ir.Node {
	if base.Flag.N != 0 || base.Flag.Cfg.Instrumenting {
		return nil
	}

	if v1 == nil || v2 != nil {
		return nil
	}

	if len(loop.Body) != 1 || loop.Body[0] == nil {
		return nil
	}

	stmt1 := loop.Body[0] // only stmt in body
	if stmt1.Op() != ir.OAS {
		return nil
	}
	stmt := stmt1.(*ir.AssignStmt)
	if stmt.X.Op() != ir.OINDEX {
		return nil
	}
	lhs := stmt.X.(*ir.IndexExpr)
	x := lhs.X

	// Get constant number of iterations for int and array cases.
	n := int64(-1)
	if ir.IsConst(a, constant.Int) {
		n = ir.Int64Val(a)
	} else if a.Type().IsArray() {
		n = a.Type().NumElem()
	} else if a.Type().IsPtr() && a.Type().Elem().IsArray() {
		n = a.Type().Elem().NumElem()
	}

	if n >= 0 {
		// Int/Array case.
		if !x.Type().IsArray() {
			return nil
		}
		if x.Type().NumElem() != n {
			return nil
		}
	} else {
		// Slice case.
		if !ir.SameSafeExpr(x, a) {
			return nil
		}
	}

	if !ir.SameSafeExpr(lhs.Index, v1) {
		return nil
	}

	if !ir.IsZero(stmt.Y) {
		return nil
	}

	return arrayClear(stmt.Pos(), x, loop)
}

// arrayClear constructs a call to runtime.memclr for fast zeroing of slices and arrays.
func arrayClear(wbPos src.XPos, a ir.Node, nrange *ir.RangeStmt) ir.Node {
	elemsize := typecheck.RangeExprType(a.Type()).Elem().Size()
	if elemsize <= 0 {
		return nil
	}

	// Convert to
	// if ln := len(a); ln != 0 {
	// 	hp = &a[0]
	// 	hn = len(a)*sizeof(elem(a))
	// 	memclr{NoHeap,Has}Pointers(hp, hn)
	// 	i = ln - 1
	// }
	n := ir.NewIfStmt(base.Pos, nil, nil, nil)
	ln := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])
	as := ir.NewAssignStmt(base.Pos, ln, ir.NewUnaryExpr(base.Pos, ir.OLEN, a))
	n.PtrInit().Append(typecheck.Stmt(as))
	n.Cond = ir.NewBinaryExpr(base.Pos, ir.ONE, ln, ir.NewInt(base.Pos, 0))

	// hp = &a[0]
	hp := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TUNSAFEPTR])

	ix := ir.NewIndexExpr(base.Pos, a, ir.NewInt(base.Pos, 0))
	ix.SetBounded(true)
	addr := typecheck.ConvNop(typecheck.NodAddr(ix), types.Types[types.TUNSAFEPTR])
	n.Body.Append(ir.NewAssignStmt(base.Pos, hp, addr))

	// hn = len(a) * sizeof(elem(a))
	hn := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TUINTPTR])
	mul := typecheck.Conv(ir.NewBinaryExpr(base.Pos, ir.OMUL, ln, ir.NewInt(base.Pos, elemsize)), types.Types[types.TUINTPTR])
	n.Body.Append(ir.NewAssignStmt(base.Pos, hn, mul))

	var fn ir.Node
	if a.Type().Elem().HasPointers() {
		// memclrHasPointers(hp, hn)
		ir.CurFunc.SetWBPos(wbPos)
		fn = mkcallstmt("memclrHasPointers", hp, hn)
	} else {
		// memclrNoHeapPointers(hp, hn)
		fn = mkcallstmt("memclrNoHeapPointers", hp, hn)
	}

	n.Body.Append(fn)

	// For array range clear, also set "i = len(a) - 1"
	if nrange != nil {
		idx := ir.NewAssignStmt(base.Pos, nrange.Key, typecheck.Conv(ir.NewBinaryExpr(base.Pos, ir.OSUB, ln, ir.NewInt(base.Pos, 1)), nrange.Key.Type()))
		n.Body.Append(idx)
	}

	n.Cond = typecheck.Expr(n.Cond)
	n.Cond = typecheck.DefaultLit(n.Cond, nil)
	typecheck.Stmts(n.Body)
	return walkStmt(n)
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/select.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

func walkSelect(sel *ir.SelectStmt) {
	lno := ir.SetPos(sel)
	if sel.Walked() {
		base.Fatalf("double walkSelect")
	}
	sel.SetWalked(true)

	init := ir.TakeInit(sel)

	init = append(init, walkSelectCases(sel.Cases)...)
	sel.Cases = nil

	sel.Compiled = init
	walkStmtList(sel.Compiled)

	base.Pos = lno
}

func walkSelectCases(cases []*ir.CommClause) []ir.Node {
	ncas := len(cases)
	sellineno := base.Pos

	// optimization: zero-case select
	if ncas == 0 {
		return []ir.Node{mkcallstmt("block")}
	}

	// optimization: one-case select: single op.
	if ncas == 1 {
		cas := cases[0]
		ir.SetPos(cas)
		l := cas.Init()
		if cas.Comm != nil { // not default:
			n := cas.Comm
			l = append(l, ir.TakeInit(n)...)
			switch n.Op() {
			default:
				base.Fatalf("select %v", n.Op())

			case ir.OSEND:
				// already ok

			case ir.OSELRECV2:
				r := n.(*ir.AssignListStmt)
				if ir.IsBlank(r.Lhs[0]) && ir.IsBlank(r.Lhs[1]) {
					n = r.Rhs[0]
					break
				}
				r.SetOp(ir.OAS2RECV)
			}

			l = append(l, n)
		}

		l = append(l, cas.Body...)
		l = append(l, ir.NewBranchStmt(base.Pos, ir.OBREAK, nil))
		return l
	}

	// convert case value arguments to addresses.
	// this rewrite is used by both the general code and the next optimization.
	var dflt *ir.CommClause
	for _, cas := range cases {
		ir.SetPos(cas)
		n := cas.Comm
		if n == nil {
			dflt = cas
			continue
		}
		switch n.Op() {
		case ir.OSEND:
			n := n.(*ir.SendStmt)
			n.Value = typecheck.NodAddr(n.Value)
			n.Value = typecheck.Expr(n.Value)

		case ir.OSELRECV2:
			n := n.(*ir.AssignListStmt)
			if !ir.IsBlank(n.Lhs[0]) {
				n.Lhs[0] = typecheck.NodAddr(n.Lhs[0])
				n.Lhs[0] = typecheck.Expr(n.Lhs[0])
			}
		}
	}

	// optimization: two-case select but one is default: single non-blocking op.
	if ncas == 2 && dflt != nil {
		cas := cases[0]
		if cas == dflt {
			cas = cases[1]
		}

		n := cas.Comm
		ir.SetPos(n)
		r := ir.NewIfStmt(base.Pos, nil, nil, nil)
		r.SetInit(cas.Init())
		var cond ir.Node
		switch n.Op() {
		default:
			base.Fatalf("select %v", n.Op())

		case ir.OSEND:
			// if selectnbsend(c, v) { body } else { default body }
			n := n.(*ir.SendStmt)
			ch := n.Chan
			cond = mkcall1(chanfn("selectnbsend", 2, ch.Type()), types.Types[types.TBOOL], r.PtrInit(), ch, n.Value)

		case ir.OSELRECV2:
			n := n.(*ir.AssignListStmt)
			recv := n.Rhs[0].(*ir.UnaryExpr)
			ch := recv.X
			elem := n.Lhs[0]
			if ir.IsBlank(elem) {
				elem = typecheck.NodNil()
			}
			cond = typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TBOOL])
			fn := chanfn("selectnbrecv", 2, ch.Type())
			call := mkcall1(fn, fn.Type().ResultsTuple(), r.PtrInit(), elem, ch)
			as := ir.NewAssignListStmt(r.Pos(), ir.OAS2, []ir.Node{cond, n.Lhs[1]}, []ir.Node{call})
			r.PtrInit().Append(typecheck.Stmt(as))
		}

		r.Cond = typecheck.Expr(cond)
		r.Body = cas.Body
		r.Else = append(dflt.Init(), dflt.Body...)
		return []ir.Node{r, ir.NewBranchStmt(base.Pos, ir.OBREAK, nil)}
	}

	if dflt != nil {
		ncas--
	}
	casorder := make([]*ir.CommClause, ncas)
	nsends, nrecvs := 0, 0

	var init []ir.Node

	// generate sel-struct
	base.Pos = sellineno
	selv := typecheck.TempAt(base.Pos, ir.CurFunc, types.NewArray(scasetype(), int64(ncas)))
	init = append(init, typecheck.Stmt(ir.NewAssignStmt(base.Pos, selv, nil)))

	// No initialization for order; runtime.selectgo is responsible for that.
	order := typecheck.TempAt(base.Pos, ir.CurFunc, types.NewArray(types.Types[types.TUINT16], 2*int64(ncas)))

	var pc0, pcs ir.Node
	if base.Flag.Race {
		pcs = typecheck.TempAt(base.Pos, ir.CurFunc, types.NewArray(types.Types[types.TUINTPTR], int64(ncas)))
		pc0 = typecheck.Expr(typecheck.NodAddr(ir.NewIndexExpr(base.Pos, pcs, ir.NewInt(base.Pos, 0))))
	} else {
		pc0 = typecheck.NodNil()
	}

	// register cases
	for _, cas := range cases {
		ir.SetPos(cas)

		init = append(init, ir.TakeInit(cas)...)

		n := cas.Comm
		if n == nil { // default:
			continue
		}

		var i int
		var c, elem ir.Node
		switch n.Op() {
		default:
			base.Fatalf("select %v", n.Op())
		case ir.OSEND:
			n := n.(*ir.SendStmt)
			i = nsends
			nsends++
			c = n.Chan
			elem = n.Value
		case ir.OSELRECV2:
			n := n.(*ir.AssignListStmt)
			nrecvs++
			i = ncas - nrecvs
			recv := n.Rhs[0].(*ir.UnaryExpr)
			c = recv.X
			elem = n.Lhs[0]
		}

		casorder[i] = cas

		setField := func(f string, val ir.Node) {
			r := ir.NewAssignStmt(base.Pos, ir.NewSelectorExpr(base.Pos, ir.ODOT, ir.NewIndexExpr(base.Pos, selv, ir.NewInt(base.Pos, int64(i))), typecheck.Lookup(f)), val)
			init = append(init, typecheck.Stmt(r))
		}

		c = typecheck.ConvNop(c, types.Types[types.TUNSAFEPTR])
		setField("c", c)
		if !ir.IsBlank(elem) {
			elem = typecheck.ConvNop(elem, types.Types[types.TUNSAFEPTR])
			setField("elem", elem)
		}

		// TODO(mdempsky): There should be a cleaner way to
		// handle this.
		if base.Flag.Race {
			r := mkcallstmt("selectsetpc", typecheck.NodAddr(ir.NewIndexExpr(base.Pos, pcs, ir.NewInt(base.Pos, int64(i)))))
			init = append(init, r)
		}
	}
	if nsends+nrecvs != ncas {
		base.Fatalf("walkSelectCases: miscount: %v + %v != %v", nsends, nrecvs, ncas)
	}

	// run the select
	base.Pos = sellineno
	chosen := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])
	recvOK := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TBOOL])
	r := ir.NewAssignListStmt(base.Pos, ir.OAS2, nil, nil)
	r.Lhs = []ir.Node{chosen, recvOK}
	fn := typecheck.LookupRuntime("selectgo")
	var fnInit ir.Nodes
	r.Rhs = []ir.Node{mkcall1(fn, fn.Type().ResultsTuple(), &fnInit, bytePtrToIndex(selv, 0), bytePtrToIndex(order, 0), pc0, ir.NewInt(base.Pos, int64(nsends)), ir.NewInt(base.Pos, int64(nrecvs)), ir.NewBool(base.Pos, dflt == nil))}
	init = append(init, fnInit...)
	init = append(init, typecheck.Stmt(r))

	// selv, order, and pcs (if race) are no longer alive after selectgo.

	// dispatch cases
	dispatch := func(cond ir.Node, cas *ir.CommClause) {
		var list ir.Nodes

		if n := cas.Comm; n != nil && n.Op() == ir.OSELRECV2 {
			n := n.(*ir.AssignListStmt)
			if !ir.IsBlank(n.Lhs[1]) {
				x := ir.NewAssignStmt(base.Pos, n.Lhs[1], recvOK)
				list.Append(typecheck.Stmt(x))
			}
		}

		list.Append(cas.Body.Take()...)
		list.Append(ir.NewBranchStmt(base.Pos, ir.OBREAK, nil))

		var r ir.Node
		if cond != nil {
			cond = typecheck.Expr(cond)
			cond = typecheck.DefaultLit(cond, nil)
			r = ir.NewIfStmt(base.Pos, cond, list, nil)
		} else {
			r = ir.NewBlockStmt(base.Pos, list)
		}

		init = append(init, r)
	}

	if dflt != nil {
		ir.SetPos(dflt)
		dispatch(ir.NewBinaryExpr(base.Pos, ir.OLT, chosen, ir.NewInt(base.Pos, 0)), dflt)
	}
	for i, cas := range casorder {
		ir.SetPos(cas)
		if i == len(casorder)-1 {
			dispatch(nil, cas)
			break
		}
		dispatch(ir.NewBinaryExpr(base.Pos, ir.OEQ, chosen, ir.NewInt(base.Pos, int64(i))), cas)
	}

	return init
}

// bytePtrToIndex returns a Node representing "(*byte)(&n[i])".
func bytePtrToIndex(n ir.Node, i int64) ir.Node {
	s := typecheck.NodAddr(ir.NewIndexExpr(base.Pos, n, ir.NewInt(base.Pos, i)))
	t := types.NewPtr(types.Types[types.TUINT8])
	return typecheck.ConvNop(s, t)
}

var scase *types.Type

// Keep in sync with src/runtime/select.go.
func scasetype() *types.Type {
	if scase == nil {
		n := ir.NewDeclNameAt(src.NoXPos, ir.OTYPE, ir.Pkgs.Runtime.Lookup("scase"))
		scase = types.NewNamed(n)
		n.SetType(scase)
		n.SetTypecheck(1)

		scase.SetUnderlying(types.NewStruct([]*types.Field{
			types.NewField(base.Pos, typecheck.Lookup("c"), types.Types[types.TUNSAFEPTR]),
			types.NewField(base.Pos, typecheck.Lookup("elem"), types.Types[types.TUNSAFEPTR]),
		}))
	}
	return scase
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/stmt.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
)

// The result of walkStmt MUST be assigned back to n, e.g.
//
//	n.Left = walkStmt(n.Left)
func walkStmt(n ir.Node) ir.Node {
	if n == nil {
		return n
	}

	ir.SetPos(n)

	walkStmtList(n.Init())

	switch n.Op() {
	default:
		if n.Op() == ir.ONAME {
			n := n.(*ir.Name)
			base.Errorf("%v is not a top level statement", n.Sym())
		} else {
			base.Errorf("%v is not a top level statement", n.Op())
		}
		ir.Dump("nottop", n)
		return n

	case ir.OAS,
		ir.OASOP,
		ir.OAS2,
		ir.OAS2DOTTYPE,
		ir.OAS2RECV,
		ir.OAS2FUNC,
		ir.OAS2MAPR,
		ir.OCLEAR,
		ir.OCLOSE,
		ir.OCOPY,
		ir.OCALLINTER,
		ir.OCALL,
		ir.OCALLFUNC,
		ir.ODELETE,
		ir.OSEND,
		ir.OPRINT,
		ir.OPRINTLN,
		ir.OPANIC,
		ir.ORECOVER,
		ir.OGETG:
		if n.Typecheck() == 0 {
			base.Fatalf("missing typecheck: %+v", n)
		}

		init := ir.TakeInit(n)
		n = walkExpr(n, &init)
		if n.Op() == ir.ONAME {
			// copy rewrote to a statement list and a temp for the length.
			// Throw away the temp to avoid plain values as statements.
			n = ir.NewBlockStmt(n.Pos(), init)
			init = nil
		}
		if len(init) > 0 {
			switch n.Op() {
			case ir.OAS, ir.OAS2, ir.OBLOCK:
				n.(ir.InitNode).PtrInit().Prepend(init...)

			default:
				init.Append(n)
				n = ir.NewBlockStmt(n.Pos(), init)
			}
		}
		return n

	// special case for a receive where we throw away
	// the value received.
	case ir.ORECV:
		n := n.(*ir.UnaryExpr)
		return walkRecv(n)

	case ir.OBREAK,
		ir.OCONTINUE,
		ir.OFALL,
		ir.OGOTO,
		ir.OLABEL,
		ir.OJUMPTABLE,
		ir.OINTERFACESWITCH,
		ir.ODCL,
		ir.OCHECKNIL:
		return n

	case ir.OBLOCK:
		n := n.(*ir.BlockStmt)
		walkStmtList(n.List)
		return n

	case ir.OCASE:
		base.Errorf("case statement out of place")
		panic("unreachable")

	case ir.ODEFER:
		n := n.(*ir.GoDeferStmt)
		ir.CurFunc.SetHasDefer(true)
		ir.CurFunc.NumDefers++
		if ir.CurFunc.NumDefers > maxOpenDefers || n.DeferAt != nil {
			// Don't allow open-coded defers if there are more than
			// 8 defers in the function, since we use a single
			// byte to record active defers.
			// Also don't allow if we need to use deferprocat.
			ir.CurFunc.SetOpenCodedDeferDisallowed(true)
		}
		if n.Esc() != ir.EscNever {
			// If n.Esc is not EscNever, then this defer occurs in a loop,
			// so open-coded defers cannot be used in this function.
			ir.CurFunc.SetOpenCodedDeferDisallowed(true)
		}
		fallthrough
	case ir.OGO:
		n := n.(*ir.GoDeferStmt)
		return walkGoDefer(n)

	case ir.OFOR:
		n := n.(*ir.ForStmt)
		return walkFor(n)

	case ir.OIF:
		n := n.(*ir.IfStmt)
		return walkIf(n)

	case ir.ORETURN:
		n := n.(*ir.ReturnStmt)
		return walkReturn(n)

	case ir.OTAILCALL:
		n := n.(*ir.TailCallStmt)

		var init ir.Nodes
		n.Call.Fun = walkExpr(n.Call.Fun, &init)

		if len(init) > 0 {
			init.Append(n)
			return ir.NewBlockStmt(n.Pos(), init)
		}
		return n

	case ir.OINLMARK:
		n := n.(*ir.InlineMarkStmt)
		return n

	case ir.OSELECT:
		n := n.(*ir.SelectStmt)
		walkSelect(n)
		return n

	case ir.OSWITCH:
		n := n.(*ir.SwitchStmt)
		walkSwitch(n)
		return n

	case ir.ORANGE:
		n := n.(*ir.RangeStmt)
		return walkRange(n)
	}

	// No return! Each case must return (or panic),
	// to avoid confusion about what gets returned
	// in the presence of type assertions.
}

func walkStmtList(s []ir.Node) {
	for i := range s {
		s[i] = walkStmt(s[i])
	}
}

// walkFor walks an OFOR node.
func walkFor(n *ir.ForStmt) ir.Node {
	if n.Cond != nil {
		init := ir.TakeInit(n.Cond)
		walkStmtList(init)
		n.Cond = walkExpr(n.Cond, &init)
		n.Cond = ir.InitExpr(init, n.Cond)
	}

	n.Post = walkStmt(n.Post)
	walkStmtList(n.Body)
	return n
}

// validGoDeferCall reports whether call is a valid call to appear in
// a go or defer statement; that is, whether it's a regular function
// call without arguments or results.
func validGoDeferCall(call ir.Node) bool {
	if call, ok := call.(*ir.CallExpr); ok && call.Op() == ir.OCALLFUNC && len(call.KeepAlive) == 0 {
		sig := call.Fun.Type()
		return sig.NumParams()+sig.NumResults() == 0
	}
	return false
}

// walkGoDefer walks an OGO or ODEFER node.
func walkGoDefer(n *ir.GoDeferStmt) ir.Node {
	if !validGoDeferCall(n.Call) {
		base.FatalfAt(n.Pos(), "invalid %v call: %v", n.Op(), n.Call)
	}

	var init ir.Nodes

	call := n.Call.(*ir.CallExpr)
	call.Fun = walkExpr(call.Fun, &init)

	if len(init) > 0 {
		init.Append(n)
		return ir.NewBlockStmt(n.Pos(), init)
	}
	return n
}

// walkIf walks an OIF node.
func walkIf(n *ir.IfStmt) ir.Node {
	n.Cond = walkExpr(n.Cond, n.PtrInit())
	walkStmtList(n.Body)
	walkStmtList(n.Else)
	return n
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/switch.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"cmp"
	"fmt"
	"go/constant"
	"go/token"
	"math"
	"math/bits"
	"slices"
	"sort"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/rttype"
	"cmd/compile/internal/ssagen"
	"cmd/compile/internal/staticdata"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/src"
)

// walkSwitch walks a switch statement.
func walkSwitch(sw *ir.SwitchStmt) {
	// Guard against double walk, see #25776.
	if sw.Walked() {
		return // Was fatal, but eliminating every possible source of double-walking is hard
	}
	sw.SetWalked(true)

	if sw.Tag != nil && sw.Tag.Op() == ir.OTYPESW {
		walkSwitchType(sw)
	} else {
		walkSwitchExpr(sw)
	}
}

// walkSwitchExpr generates an AST implementing sw.  sw is an
// expression switch.
func walkSwitchExpr(sw *ir.SwitchStmt) {
	lno := ir.SetPos(sw)

	cond := sw.Tag
	sw.Tag = nil

	// convert switch {...} to switch true {...}
	if cond == nil {
		cond = ir.NewBool(base.Pos, true)
		cond = typecheck.Expr(cond)
		cond = typecheck.DefaultLit(cond, nil)
	}

	// Given "switch string(byteslice)",
	// with all cases being side-effect free,
	// use a zero-cost alias of the byte slice.
	// Do this before calling walkExpr on cond,
	// because walkExpr will lower the string
	// conversion into a runtime call.
	// See issue 24937 for more discussion.
	if cond.Op() == ir.OBYTES2STR && allCaseExprsAreSideEffectFree(sw) {
		cond := cond.(*ir.ConvExpr)
		cond.SetOp(ir.OBYTES2STRTMP)
	}

	cond = walkExpr(cond, sw.PtrInit())
	if cond.Op() != ir.OLITERAL && cond.Op() != ir.ONIL {
		cond = copyExpr(cond, cond.Type(), &sw.Compiled)
	}

	base.Pos = lno

	tryLookupTable(sw, cond)

	s := exprSwitch{
		pos:      lno,
		exprname: cond,
	}

	var defaultGoto ir.Node
	var body ir.Nodes
	for _, ncase := range sw.Cases {
		label := typecheck.AutoLabel(".s")
		jmp := ir.NewBranchStmt(ncase.Pos(), ir.OGOTO, label)

		// Process case dispatch.
		if len(ncase.List) == 0 {
			if defaultGoto != nil {
				base.Fatalf("duplicate default case not detected during typechecking")
			}
			defaultGoto = jmp
		}

		for i, n1 := range ncase.List {
			var rtype ir.Node
			if i < len(ncase.RTypes) {
				rtype = ncase.RTypes[i]
			}
			s.Add(ncase.Pos(), n1, rtype, jmp)
		}

		// Process body.
		body.Append(ir.NewLabelStmt(ncase.Pos(), label))
		body.Append(ncase.Body...)
		if fall, pos := endsInFallthrough(ncase.Body); !fall {
			br := ir.NewBranchStmt(base.Pos, ir.OBREAK, nil)
			br.SetPos(pos)
			body.Append(br)
		}
	}
	sw.Cases = nil

	if defaultGoto == nil {
		br := ir.NewBranchStmt(base.Pos, ir.OBREAK, nil)
		br.SetPos(br.Pos().WithNotStmt())
		defaultGoto = br
	}

	s.Emit(&sw.Compiled)
	sw.Compiled.Append(defaultGoto)
	sw.Compiled.Append(body.Take()...)
	walkStmtList(sw.Compiled)
}

// An exprSwitch walks an expression switch.
type exprSwitch struct {
	pos      src.XPos
	exprname ir.Node // value being switched on

	done    ir.Nodes
	clauses []exprClause
}

type exprClause struct {
	pos    src.XPos
	lo, hi ir.Node
	rtype  ir.Node // *runtime._type for OEQ node
	jmp    ir.Node
}

func (s *exprSwitch) Add(pos src.XPos, expr, rtype, jmp ir.Node) {
	c := exprClause{pos: pos, lo: expr, hi: expr, rtype: rtype, jmp: jmp}
	if types.IsOrdered[s.exprname.Type().Kind()] && expr.Op() == ir.OLITERAL {
		s.clauses = append(s.clauses, c)
		return
	}

	s.flush()
	s.clauses = append(s.clauses, c)
	s.flush()
}

func (s *exprSwitch) Emit(out *ir.Nodes) {
	s.flush()
	out.Append(s.done.Take()...)
}

func (s *exprSwitch) flush() {
	cc := s.clauses
	s.clauses = nil
	if len(cc) == 0 {
		return
	}

	// Caution: If len(cc) == 1, then cc[0] might not an OLITERAL.
	// The code below is structured to implicitly handle this case
	// (e.g., sort.Slice doesn't need to invoke the less function
	// when there's only a single slice element).

	if s.exprname.Type().IsString() && len(cc) >= 2 {
		// Sort strings by length and then by value. It is
		// much cheaper to compare lengths than values, and
		// all we need here is consistency. We respect this
		// sorting below.
		slices.SortFunc(cc, func(a, b exprClause) int {
			si := ir.StringVal(a.lo)
			sj := ir.StringVal(b.lo)
			if len(si) != len(sj) {
				return cmp.Compare(len(si), len(sj))
			}
			return strings.Compare(si, sj)
		})

		// runLen returns the string length associated with a
		// particular run of exprClauses.
		runLen := func(run []exprClause) int64 { return int64(len(ir.StringVal(run[0].lo))) }

		// Collapse runs of consecutive strings with the same length.
		var runs [][]exprClause
		start := 0
		for i := 1; i < len(cc); i++ {
			if runLen(cc[start:]) != runLen(cc[i:]) {
				runs = append(runs, cc[start:i])
				start = i
			}
		}
		runs = append(runs, cc[start:])

		// We have strings of more than one length. Generate an
		// outer switch which switches on the length of the string
		// and an inner switch in each case which resolves all the
		// strings of the same length. The code looks something like this:

		// goto outerLabel
		// len5:
		//   ... search among length 5 strings ...
		//   goto endLabel
		// len8:
		//   ... search among length 8 strings ...
		//   goto endLabel
		// ... other lengths ...
		// outerLabel:
		// switch len(s) {
		//   case 5: goto len5
		//   case 8: goto len8
		//   ... other lengths ...
		// }
		// endLabel:

		outerLabel := typecheck.AutoLabel(".s")
		endLabel := typecheck.AutoLabel(".s")

		// Jump around all the individual switches for each length.
		s.done.Append(ir.NewBranchStmt(s.pos, ir.OGOTO, outerLabel))

		var outer exprSwitch
		outer.exprname = ir.NewUnaryExpr(s.pos, ir.OLEN, s.exprname)
		outer.exprname.SetType(types.Types[types.TINT])

		for _, run := range runs {
			// Target label to jump to when we match this length.
			label := typecheck.AutoLabel(".s")

			// Search within this run of same-length strings.
			pos := run[0].pos
			s.done.Append(ir.NewLabelStmt(pos, label))
			stringSearch(s.exprname, run, &s.done)
			s.done.Append(ir.NewBranchStmt(pos, ir.OGOTO, endLabel))

			// Add length case to outer switch.
			cas := ir.NewInt(pos, runLen(run))
			jmp := ir.NewBranchStmt(pos, ir.OGOTO, label)
			outer.Add(pos, cas, nil, jmp)
		}
		s.done.Append(ir.NewLabelStmt(s.pos, outerLabel))
		outer.Emit(&s.done)
		s.done.Append(ir.NewLabelStmt(s.pos, endLabel))
		return
	}

	sort.Slice(cc, func(i, j int) bool {
		return constant.Compare(cc[i].lo.Val(), token.LSS, cc[j].lo.Val())
	})

	// Merge consecutive integer cases.
	if s.exprname.Type().IsInteger() {
		consecutive := func(last, next constant.Value) bool {
			delta := constant.BinaryOp(next, token.SUB, last)
			return constant.Compare(delta, token.EQL, constant.MakeInt64(1))
		}

		merged := cc[:1]
		for _, c := range cc[1:] {
			last := &merged[len(merged)-1]
			if last.jmp == c.jmp && consecutive(last.hi.Val(), c.lo.Val()) {
				last.hi = c.lo
			} else {
				merged = append(merged, c)
			}
		}
		cc = merged
	}

	s.search(cc, &s.done)
}

func (s *exprSwitch) search(cc []exprClause, out *ir.Nodes) {
	if s.tryJumpTable(cc, out) {
		return
	}
	binarySearch(len(cc), out,
		func(i int) ir.Node {
			return ir.NewBinaryExpr(base.Pos, ir.OLE, s.exprname, cc[i-1].hi)
		},
		func(i int, nif *ir.IfStmt) {
			c := &cc[i]
			nif.Cond = c.test(s.exprname)
			nif.Body = []ir.Node{c.jmp}
		},
	)
}

// Try to implement the clauses with a jump table. Returns true if successful.
func (s *exprSwitch) tryJumpTable(cc []exprClause, out *ir.Nodes) bool {
	const minCases = 8   // have at least minCases cases in the switch
	const minDensity = 4 // use at least 1 out of every minDensity entries

	if base.Flag.N != 0 || !ssagen.Arch.LinkArch.CanJumpTable || base.Ctxt.Retpoline {
		return false
	}
	if len(cc) < minCases {
		return false // not enough cases for it to be worth it
	}
	if cc[0].lo.Val().Kind() != constant.Int {
		return false // e.g. float
	}
	if s.exprname.Type().Size() > int64(types.PtrSize) {
		return false // 64-bit switches on 32-bit archs
	}
	min := cc[0].lo.Val()
	max := cc[len(cc)-1].hi.Val()
	width := constant.BinaryOp(constant.BinaryOp(max, token.SUB, min), token.ADD, constant.MakeInt64(1))
	limit := constant.MakeInt64(int64(len(cc)) * minDensity)
	if constant.Compare(width, token.GTR, limit) {
		// We disable jump tables if we use less than a minimum fraction of the entries.
		// i.e. for switch x {case 0: case 1000: case 2000:} we don't want to use a jump table.
		return false
	}
	jt := ir.NewJumpTableStmt(base.Pos, s.exprname)
	for _, c := range cc {
		jmp := c.jmp.(*ir.BranchStmt)
		if jmp.Op() != ir.OGOTO || jmp.Label == nil {
			panic("bad switch case body")
		}
		for i := c.lo.Val(); constant.Compare(i, token.LEQ, c.hi.Val()); i = constant.BinaryOp(i, token.ADD, constant.MakeInt64(1)) {
			jt.Cases = append(jt.Cases, i)
			jt.Targets = append(jt.Targets, jmp.Label)
		}
	}
	out.Append(jt)
	return true
}

// tryLookupTable attempts to replace constant-returning cases of an integer
// switch with a static lookup table. Cases whose bodies are a single "return
// <int constant>" are served from a read-only array, eliminating branching.
// Remaining cases (non-constant bodies, default) are left in sw.Cases for
// normal switch compilation.
//
// For example:
//
//	switch x {
//	case 0: return 10
//	case 1: return 20
//	case 2, 3: return 30
//	default: return -1
//	}
//
// Becomes:
//
//	var table = [4]int{10, 20, 30, 30}
//	if uint(x) < 4 { return table[x] }
//	// remaining switch for default
//
// Partial optimization also works when some cases have non-constant bodies:
//
//	switch x {
//	case 1: return 1
//	case 2: return 4
//	case 3: sideEffect(); return 9
//	...
//	default: return x * x
//	}
//
// Becomes:
//
//	var table = [8]int{1, 4, 0, ...}
//	var mask  = [8]uint8{1, 1, 0, ...}
//	if uint(x-1) <= 7 && mask[x-1] != 0 { return table[x-1] }
//	// remaining switch for case 3 + default
func tryLookupTable(sw *ir.SwitchStmt, cond ir.Node) {
	const minCases = 4 // need enough cases to justify a table

	if base.Flag.N != 0 {
		return // optimizations disabled
	}
	if !cond.Type().IsInteger() {
		return
	}
	if cond.Type().Size() > int64(types.PtrSize) {
		return // 64-bit switches on 32-bit archs
	}

	fn := ir.CurFunc
	if fn == nil || fn.Type().NumResults() != 1 {
		return // only handle single return value
	}
	resultType := fn.Type().Results()[0].Type

	// Classify each case as const-returning or not.
	// TODO: support more complex bodies, like local variable assignments.
	// For example:
	//
	//   var n int
	//   switch x {
	//   case 1: n = 1
	//   case 2: n = 4
	//   case 3: n = 9
	//   case 4: n = 16
	//   }
	//   return n
	//
	// Could be optimized to:
	//
	//   var table = [4]int{1, 4, 9, 16}
	//   var n int
	//   if uint(x-1) < 4 { n = table[x-1] }
	//   return n
	constSet := make(map[int64]ir.Node) // case value → return constant literal
	constCaseSet := make(map[int]bool)  // indices of const-returning non-default cases
	excludeSet := make(map[int64]bool)  // case values with non-const bodies
	var defaultVal ir.Node              // non-nil if default returns a constant
	minVal, maxVal := int64(math.MaxInt64), int64(math.MinInt64)
	var excludeNextCase bool // true if the previous case ends in fallthrough

	for i, ncase := range sw.Cases {
		// A case that is the target of a fallthrough must be excluded,
		// since removing it would break the fallthrough chain.
		isFallthroughTarget := excludeNextCase
		excludeNextCase, _ = endsInFallthrough(ncase.Body)

		if len(ncase.List) == 0 {
			// Default case: check if it returns a constant (for gap filling).
			if isConstReturn(ncase) && !isFallthroughTarget {
				defaultVal = ncase.Body[0].(*ir.ReturnStmt).Results[0]
			}
			continue
		}

		vals, ok := constIntCaseVals(ncase)
		if !ok {
			// Case has a non-constant case expression (e.g. a variable).
			// Bail out: we can't determine overlap with the table range.
			// For example:
			//   case 1: return 1  // const → would go to table
			//   case c: return 9  // c is a variable, not a constant
			//   case 3: return 3  // const → would go to table
			// At runtime, if c==3 then Go evaluates case c before case 3,
			// returning 9. But if we put cases 1 and 3 in a table, n==3
			// would return 3 from the table, skipping the case c check.
			return
		}

		if !isConstReturn(ncase) || isFallthroughTarget || excludeNextCase {
			// Non-const body, fallthrough source, or fallthrough target:
			// exclude these values from the table so the mask redirects
			// them to the normal switch, preserving Go's top-to-bottom
			// case evaluation order.
			for _, v := range vals {
				excludeSet[v] = true
			}
			continue // will be handled by normal switch
		}

		retVal := ncase.Body[0].(*ir.ReturnStmt).Results[0]
		for _, v := range vals {
			constSet[v] = retVal
			minVal = min(minVal, v)
			maxVal = max(maxVal, v)
		}
		constCaseSet[i] = true
	}

	if len(constSet) < minCases {
		return
	}

	tableSize := maxVal - minVal + 1
	if tableSize <= 0 || !isSwitchDense(int64(len(constSet)), tableSize) {
		return // too sparse
	}

	// Build static lookup table and determine which slots are valid.
	// Also build the bitmask inline if the table is small enough.
	tabType := types.NewArray(resultType, tableSize)
	tabName := readonlystaticname(tabType)
	elemSize := int(resultType.Size())
	maxBitmaskSize := int64(types.PtrSize * 8) // 32 or 64

	var needMask bool
	var bitmask uint64
	validSlots := make([]bool, tableSize)
	// Pre-size the symbol to cover the full table.
	tabName.Linksym().WriteInt(base.Ctxt, tableSize*int64(elemSize)-1, 1, 0)
	for i := range tableSize {
		caseVal := minVal + i
		if excludeSet[caseVal] {
			// Non-const case in range: must fall through to normal switch.
			needMask = true
		} else {
			val := constSet[caseVal]
			if val == nil {
				if defaultVal == nil {
					// Gap with no const default: must fall through to normal switch.
					needMask = true
					continue
				}
				val = defaultVal // gap filled with default constant value
			}
			staticdata.InitConst(tabName, i*int64(elemSize), val, elemSize)
			validSlots[i] = true
			bitmask |= 1 << uint(i)
		}
	}

	// Build mask if some slots must fall through to normal switch.
	// When the table fits in a register-width bitmask (≤32 entries on 32-bit,
	// ≤64 on 64-bit), use the bitmask computed above. For larger tables,
	// fall back to a byte array.
	var maskName *ir.Name
	useBitmask := needMask && tableSize <= maxBitmaskSize
	if needMask && !useBitmask {
		maskType := types.NewArray(types.Types[types.TUINT8], tableSize)
		maskName = readonlystaticname(maskType)
		maskSym := maskName.Linksym()
		for i := range tableSize {
			var v uint8
			if validSlots[i] {
				v = 1
			}
			maskSym.WriteInt(base.Ctxt, i, 1, int64(v))
		}
	}

	// Generate code:
	//   idx := uint(int(cond) - minVal)
	//   if idx <= uint(maxVal-minVal) [&& bitmask>>idx&1 != 0] { return table[idx] }
	pos := sw.Pos()

	// Widen cond to int to avoid overflow in small integer types.
	intType := types.Types[types.TINT]
	wideCond := typecheck.Conv(cond, intType)

	// Compute idx = int(cond) - minVal.
	var idx ir.Node
	if minVal != 0 {
		minLit := ir.NewBasicLit(pos, intType, constant.MakeInt64(minVal))
		idx = typecheck.Expr(ir.NewBinaryExpr(pos, ir.OSUB, wideCond, minLit))
	} else {
		idx = wideCond
	}

	// Convert to uint for the one-sided bounds check and store in a temp
	// so the index can be shared across the bounds check, table, and mask.
	uintType := types.Types[types.TUINT]
	uidx := typecheck.Conv(idx, uintType)
	uidx = copyExpr(uidx, uintType, &sw.Compiled)

	// Bounds check: uint(idx) <= uint(maxVal - minVal).
	rangeLit := ir.NewBasicLit(pos, uintType, constant.MakeUint64(uint64(maxVal-minVal)))
	boundsCheck := typecheck.Expr(ir.NewBinaryExpr(pos, ir.OLE, uidx, rangeLit))
	boundsCheck = typecheck.DefaultLit(boundsCheck, nil)

	// Table lookup: table[idx] with bounds elided (already checked).
	lookup := ir.NewIndexExpr(pos, tabName, uidx)
	lookup.SetBounded(true)
	lookup = typecheck.Expr(lookup).(*ir.IndexExpr)

	retStmt := ir.NewReturnStmt(pos, []ir.Node{lookup})

	var ifBody []ir.Node
	if needMask {
		var maskCheck ir.Node
		if useBitmask {
			// Bitmask check: (bitmask >> idx) & 1 != 0.
			// Use uintptr so the operation is register-width on all architectures.
			bitmaskType := types.Types[types.TUINTPTR]
			bitmaskLit := ir.NewBasicLit(pos, bitmaskType, constant.MakeUint64(bitmask))
			shifted := typecheck.Expr(ir.NewBinaryExpr(pos, ir.ORSH, bitmaskLit, uidx))
			one := ir.NewBasicLit(pos, bitmaskType, constant.MakeUint64(1))
			masked := typecheck.Expr(ir.NewBinaryExpr(pos, ir.OAND, shifted, one))
			zero := ir.NewBasicLit(pos, bitmaskType, constant.MakeUint64(0))
			maskCheck = typecheck.Expr(ir.NewBinaryExpr(pos, ir.ONE, masked, zero))
		} else {
			// Mask array check: mask[idx] != 0.
			maskLookup := ir.NewIndexExpr(pos, maskName, uidx)
			maskLookup.SetBounded(true)
			maskLookup = typecheck.Expr(maskLookup).(*ir.IndexExpr)
			zero := ir.NewBasicLit(pos, types.Types[types.TUINT8], constant.MakeInt64(0))
			maskCheck = typecheck.Expr(ir.NewBinaryExpr(pos, ir.ONE, maskLookup, zero))
		}
		maskCheck = typecheck.DefaultLit(maskCheck, nil)

		innerIf := ir.NewIfStmt(pos, maskCheck, []ir.Node{retStmt}, nil)
		ifBody = []ir.Node{innerIf}
	} else {
		ifBody = []ir.Node{retStmt}
	}

	outerIf := ir.NewIfStmt(pos, boundsCheck, ifBody, nil)
	sw.Compiled.Append(outerIf)

	// Remove handled const cases from sw.Cases.
	// Keep default and non-const cases for normal switch processing.
	newCases := make([]*ir.CaseClause, 0, len(sw.Cases)-len(constCaseSet))
	for i, ncase := range sw.Cases {
		if !constCaseSet[i] {
			newCases = append(newCases, ncase)
		}
	}
	sw.Cases = newCases
}

// isSwitchDense reports whether a lookup table with tableSize entries
// for numCases cases is dense enough to be worth building.
// It requires at least 40% of table slots to be used, matching the
// density threshold used by LLVM's SimplifyCFG.
func isSwitchDense(numCases, tableSize int64) bool {
	const minDensity = 40
	if tableSize >= math.MaxInt64/100 {
		return false // avoid multiplication overflow below
	}
	return numCases*100 >= tableSize*minDensity
}

// isConstReturn reports whether ncase has a body that is a single
// return statement returning one constant.
func isConstReturn(ncase *ir.CaseClause) bool {
	if len(ncase.Body) != 1 {
		return false
	}
	ret, ok := ncase.Body[0].(*ir.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	return ret.Results[0].Op() == ir.OLITERAL
}

// constIntCaseVals returns the int64 values of all case expressions in
// ncase, if they are all integer constants. Returns ok=false if any
// case expression is not a constant integer.
func constIntCaseVals(ncase *ir.CaseClause) (vals []int64, ok bool) {
	for _, n1 := range ncase.List {
		if n1.Op() != ir.OLITERAL || n1.Val().Kind() != constant.Int {
			return nil, false
		}
		v, fit := constant.Int64Val(n1.Val())
		if !fit {
			return nil, false
		}
		vals = append(vals, v)
	}
	return vals, true
}

func (c *exprClause) test(exprname ir.Node) ir.Node {
	// Integer range.
	if c.hi != c.lo {
		low := ir.NewBinaryExpr(c.pos, ir.OGE, exprname, c.lo)
		high := ir.NewBinaryExpr(c.pos, ir.OLE, exprname, c.hi)
		return ir.NewLogicalExpr(c.pos, ir.OANDAND, low, high)
	}

	// Optimize "switch true { ...}" and "switch false { ... }".
	if ir.IsConst(exprname, constant.Bool) && !c.lo.Type().IsInterface() {
		if ir.BoolVal(exprname) {
			return c.lo
		} else {
			return ir.NewUnaryExpr(c.pos, ir.ONOT, c.lo)
		}
	}

	n := ir.NewBinaryExpr(c.pos, ir.OEQ, exprname, c.lo)
	n.RType = c.rtype
	return n
}

func allCaseExprsAreSideEffectFree(sw *ir.SwitchStmt) bool {
	// In theory, we could be more aggressive, allowing any
	// side-effect-free expressions in cases, but it's a bit
	// tricky because some of that information is unavailable due
	// to the introduction of temporaries during order.
	// Restricting to constants is simple and probably powerful
	// enough.

	for _, ncase := range sw.Cases {
		for _, v := range ncase.List {
			if v.Op() != ir.OLITERAL {
				return false
			}
		}
	}
	return true
}

// endsInFallthrough reports whether stmts ends with a "fallthrough" statement.
func endsInFallthrough(stmts []ir.Node) (bool, src.XPos) {
	if len(stmts) == 0 {
		return false, src.NoXPos
	}
	i := len(stmts) - 1
	return stmts[i].Op() == ir.OFALL, stmts[i].Pos()
}

// walkSwitchType generates an AST that implements sw, where sw is a
// type switch.
func walkSwitchType(sw *ir.SwitchStmt) {
	var s typeSwitch
	origSrc := sw.Tag.(*ir.TypeSwitchGuard).X
	s.srcName = origSrc
	s.srcName = walkExpr(s.srcName, sw.PtrInit())
	s.srcName = copyExpr(s.srcName, s.srcName.Type(), &sw.Compiled)
	s.okName = typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TBOOL])
	s.itabName = typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TUINT8].PtrTo())

	// Get interface descriptor word.
	// For empty interfaces this will be the type.
	// For non-empty interfaces this will be the itab.
	srcItab := ir.NewUnaryExpr(base.Pos, ir.OITAB, s.srcName)
	srcData := ir.NewUnaryExpr(base.Pos, ir.OIDATA, s.srcName)
	srcData.SetType(types.Types[types.TUINT8].PtrTo())
	srcData.SetTypecheck(1)

	// For empty interfaces, do:
	//     if e._type == nil {
	//         do nil case if it exists, otherwise default
	//     }
	//     h := e._type.hash
	// Use a similar strategy for non-empty interfaces.
	ifNil := ir.NewIfStmt(base.Pos, nil, nil, nil)
	ifNil.Cond = ir.NewBinaryExpr(base.Pos, ir.OEQ, srcItab, typecheck.NodNil())
	base.Pos = base.Pos.WithNotStmt() // disable statement marks after the first check.
	ifNil.Cond = typecheck.Expr(ifNil.Cond)
	ifNil.Cond = typecheck.DefaultLit(ifNil.Cond, nil)
	// ifNil.Nbody assigned later.
	sw.Compiled.Append(ifNil)

	// Load hash from type or itab.
	dotHash := typeHashFieldOf(base.Pos, srcItab)
	s.hashName = copyExpr(dotHash, dotHash.Type(), &sw.Compiled)

	// Make a label for each case body.
	labels := make([]*types.Sym, len(sw.Cases))
	for i := range sw.Cases {
		labels[i] = typecheck.AutoLabel(".s")
	}

	// "jump" to execute if no case matches.
	br := ir.NewBranchStmt(base.Pos, ir.OBREAK, nil)

	// Assemble a list of all the types we're looking for.
	// This pass flattens the case lists, as well as handles
	// some unusual cases, like default and nil cases.
	type oneCase struct {
		pos src.XPos
		jmp ir.Node // jump to body of selected case

		// The case we're matching. Normally the type we're looking for
		// is typ.Type(), but when typ is ODYNAMICTYPE the actual type
		// we're looking for is not a compile-time constant (typ.Type()
		// will be its shape).
		typ ir.Node

		// For a single runtime known type with a case var, create a
		// temporary variable to hold the value returned by the dynamic
		// type assert expr, so that we do not need one more dynamic
		// type assert expr later.
		val ir.Node
		idx int // index of the single runtime known type in sw.Cases
	}
	var cases []oneCase
	var defaultGoto, nilGoto ir.Node
	for i, ncase := range sw.Cases {
		jmp := ir.NewBranchStmt(ncase.Pos(), ir.OGOTO, labels[i])
		if len(ncase.List) == 0 { // default:
			if defaultGoto != nil {
				base.Fatalf("duplicate default case not detected during typechecking")
			}
			defaultGoto = jmp
		}
		for _, n1 := range ncase.List {
			if ir.IsNil(n1) { // case nil:
				if nilGoto != nil {
					base.Fatalf("duplicate nil case not detected during typechecking")
				}
				nilGoto = jmp
				continue
			}
			idx := -1
			var val ir.Node
			// for a single runtime known type with a case var, create the tmpVar
			if len(ncase.List) == 1 && ncase.List[0].Op() == ir.ODYNAMICTYPE && ncase.Var != nil {
				val = typecheck.TempAt(ncase.Pos(), ir.CurFunc, ncase.Var.Type())
				idx = i
			}
			cases = append(cases, oneCase{
				pos: ncase.Pos(),
				typ: n1,
				jmp: jmp,
				val: val,
				idx: idx,
			})
		}
	}
	if defaultGoto == nil {
		defaultGoto = br
	}
	if nilGoto == nil {
		nilGoto = defaultGoto
	}
	ifNil.Body = []ir.Node{nilGoto}

	// Now go through the list of cases, processing groups as we find them.
	var concreteCases []oneCase
	var interfaceCases []oneCase
	flush := func() {
		// Process all the concrete types first. Because we handle shadowing
		// below, it is correct to do all the concrete types before all of
		// the interface types.
		// The concrete cases can all be handled without a runtime call.
		if len(concreteCases) > 0 {
			var clauses []typeClause
			for _, c := range concreteCases {
				as := ir.NewAssignListStmt(c.pos, ir.OAS2,
					[]ir.Node{ir.BlankNode, s.okName},                               // _, ok =
					[]ir.Node{ir.NewTypeAssertExpr(c.pos, s.srcName, c.typ.Type())}) // iface.(type)
				nif := ir.NewIfStmt(c.pos, s.okName, []ir.Node{c.jmp}, nil)
				clauses = append(clauses, typeClause{
					hash: types.TypeHash(c.typ.Type()),
					body: []ir.Node{typecheck.Stmt(as), typecheck.Stmt(nif)},
				})
			}
			s.flush(clauses, &sw.Compiled)
			concreteCases = concreteCases[:0]
		}

		// The "any" case, if it exists, must be the last interface case, because
		// it would shadow all subsequent cases. Strip it off here so the runtime
		// call only needs to handle non-empty interfaces.
		var anyGoto ir.Node
		if len(interfaceCases) > 0 && interfaceCases[len(interfaceCases)-1].typ.Type().IsEmptyInterface() {
			anyGoto = interfaceCases[len(interfaceCases)-1].jmp
			interfaceCases = interfaceCases[:len(interfaceCases)-1]
		}

		// Next, process all the interface types with a single call to the runtime.
		if len(interfaceCases) > 0 {

			// Build an internal/abi.InterfaceSwitch descriptor to pass to the runtime.
			lsym := types.LocalPkg.Lookup(fmt.Sprintf(".interfaceSwitch.%d", interfaceSwitchGen)).LinksymABI(obj.ABI0)
			interfaceSwitchGen++
			c := rttype.NewCursor(lsym, 0, rttype.InterfaceSwitch)
			c.Field("Cache").WritePtr(typecheck.LookupRuntimeVar("emptyInterfaceSwitchCache"))
			c.Field("NCases").WriteInt(int64(len(interfaceCases)))
			array, sizeDelta := c.Field("Cases").ModifyArray(len(interfaceCases))
			for i, c := range interfaceCases {
				array.Elem(i).WritePtr(reflectdata.TypeLinksym(c.typ.Type()))
			}
			objw.Global(lsym, int32(rttype.InterfaceSwitch.Size()+sizeDelta), obj.LOCAL)
			// The GC only needs to see the first pointer in the structure (all the others
			// are to static locations). So the InterfaceSwitch type itself is fine, even
			// though it might not cover the whole array we wrote above.
			lsym.Gotype = reflectdata.TypeLinksym(rttype.InterfaceSwitch)

			// Call runtime to do switch
			// case, itab = runtime.interfaceSwitch(&descriptor, typeof(arg))
			var typeArg ir.Node
			if s.srcName.Type().IsEmptyInterface() {
				typeArg = ir.NewConvExpr(base.Pos, ir.OCONVNOP, types.Types[types.TUINT8].PtrTo(), srcItab)
			} else {
				typeArg = itabType(srcItab)
			}
			caseVar := typecheck.TempAt(base.Pos, ir.CurFunc, types.Types[types.TINT])
			isw := ir.NewInterfaceSwitchStmt(base.Pos, caseVar, s.itabName, typeArg, dotHash, lsym)
			sw.Compiled.Append(isw)

			// Switch on the result of the call (or cache lookup).
			var newCases []*ir.CaseClause
			for i, c := range interfaceCases {
				newCases = append(newCases, &ir.CaseClause{
					List: []ir.Node{ir.NewInt(base.Pos, int64(i))},
					Body: []ir.Node{c.jmp},
				})
			}
			// TODO: add len(newCases) case, mark switch as bounded
			sw2 := ir.NewSwitchStmt(base.Pos, caseVar, newCases)
			sw.Compiled.Append(typecheck.Stmt(sw2))
			interfaceCases = interfaceCases[:0]
		}

		if anyGoto != nil {
			// We've already handled the nil case, so everything
			// that reaches here matches the "any" case.
			sw.Compiled.Append(anyGoto)
		}
	}
caseLoop:
	for _, c := range cases {
		if c.typ.Op() == ir.ODYNAMICTYPE {
			flush() // process all previous cases
			dt := c.typ.(*ir.DynamicType)
			dot := ir.NewDynamicTypeAssertExpr(c.pos, ir.ODYNAMICDOTTYPE, s.srcName, dt.RType)
			dot.ITab = dt.ITab
			dot.SetType(c.typ.Type())
			dot.SetTypecheck(1)

			as := ir.NewAssignListStmt(c.pos, ir.OAS2, nil, nil)
			as.Lhs = []ir.Node{ir.BlankNode, s.okName} // _, ok =
			if c.val != nil {
				as.Lhs[0] = c.val // tmpVar, ok =
			}
			as.Rhs = []ir.Node{dot}
			typecheck.Stmt(as)

			nif := ir.NewIfStmt(c.pos, s.okName, []ir.Node{c.jmp}, nil)
			sw.Compiled.Append(as, nif)
			continue
		}

		// Check for shadowing (a case that will never fire because
		// a previous case would have always fired first). This check
		// allows us to reorder concrete and interface cases.
		// (TODO: these should be vet failures, maybe?)
		for _, ic := range interfaceCases {
			// An interface type case will shadow all
			// subsequent types that implement that interface.
			if typecheck.Implements(c.typ.Type(), ic.typ.Type()) {
				continue caseLoop
			}
			// Note that we don't need to worry about:
			// 1. Two concrete types shadowing each other. That's
			//    disallowed by the spec.
			// 2. A concrete type shadowing an interface type.
			//    That can never happen, as interface types can
			//    be satisfied by an infinite set of concrete types.
			// The correctness of this step also depends on handling
			// the dynamic type cases separately, as we do above.
		}

		if shapeTypeAssertImpossible(origSrc, c.typ.Type()) {
			continue
		}

		if c.typ.Type().IsInterface() {
			interfaceCases = append(interfaceCases, c)
		} else {
			concreteCases = append(concreteCases, c)
		}
	}
	flush()

	sw.Compiled.Append(defaultGoto) // if none of the cases matched

	// Now generate all the case bodies
	for i, ncase := range sw.Cases {
		sw.Compiled.Append(ir.NewLabelStmt(ncase.Pos(), labels[i]))
		if caseVar := ncase.Var; caseVar != nil {
			val := s.srcName
			if len(ncase.List) == 1 {
				// single type. We have to downcast the input value to the target type.
				if ncase.List[0].Op() == ir.OTYPE { // single compile-time known type
					t := ncase.List[0].Type()
					if t.IsInterface() {
						// This case is an interface. Build case value from input interface.
						// The data word will always be the same, but the itab/type changes.
						if t.IsEmptyInterface() {
							var typ ir.Node
							if s.srcName.Type().IsEmptyInterface() {
								// E->E, nothing to do, type is already correct.
								typ = srcItab
							} else {
								// I->E, load type out of itab
								typ = itabType(srcItab)
								typ.SetPos(ncase.Pos())
							}
							val = ir.NewBinaryExpr(ncase.Pos(), ir.OMAKEFACE, typ, srcData)
						} else {
							// The itab we need was returned by a runtime.interfaceSwitch call.
							val = ir.NewBinaryExpr(ncase.Pos(), ir.OMAKEFACE, s.itabName, srcData)
						}
					} else {
						// This case is a concrete type, just read its value out of the interface.
						val = ifaceData(ncase.Pos(), s.srcName, t)
					}
				} else if ncase.List[0].Op() == ir.ODYNAMICTYPE { // single runtime known type
					var found bool
					for _, c := range cases {
						if c.idx == i {
							val = c.val
							found = val != nil
							break
						}
					}
					// the tmpVar must always be found
					if !found {
						base.Fatalf("an error occurred when processing type switch case %v", ncase.List[0])
					}
				} else if ir.IsNil(ncase.List[0]) {
				} else {
					base.Fatalf("unhandled type switch case %v", ncase.List[0])
				}
				val.SetType(caseVar.Type())
				val.SetTypecheck(1)
			}
			l := []ir.Node{
				ir.NewDecl(ncase.Pos(), ir.ODCL, caseVar),
				ir.NewAssignStmt(ncase.Pos(), caseVar, val),
			}
			typecheck.Stmts(l)
			sw.Compiled.Append(l...)
		}
		sw.Compiled.Append(ncase.Body...)
		sw.Compiled.Append(br)
	}

	walkStmtList(sw.Compiled)
	sw.Tag = nil
	sw.Cases = nil
}

var interfaceSwitchGen int

// typeHashFieldOf returns an expression to select the type hash field
// from an interface's descriptor word (whether a *runtime._type or
// *runtime.itab pointer).
func typeHashFieldOf(pos src.XPos, itab *ir.UnaryExpr) *ir.SelectorExpr {
	if itab.Op() != ir.OITAB {
		base.Fatalf("expected OITAB, got %v", itab.Op())
	}
	var hashField *types.Field
	if itab.X.Type().IsEmptyInterface() {
		// runtime._type's hash field
		if rtypeHashField == nil {
			rtypeHashField = runtimeField("hash", rttype.Type.OffsetOf("Hash"), types.Types[types.TUINT32])
		}
		hashField = rtypeHashField
	} else {
		// runtime.itab's hash field
		if itabHashField == nil {
			itabHashField = runtimeField("hash", rttype.ITab.OffsetOf("Hash"), types.Types[types.TUINT32])
		}
		hashField = itabHashField
	}
	return boundedDotPtr(pos, itab, hashField)
}

var rtypeHashField, itabHashField *types.Field

// A typeSwitch walks a type switch.
type typeSwitch struct {
	// Temporary variables (i.e., ONAMEs) used by type switch dispatch logic:
	srcName  ir.Node // value being type-switched on
	hashName ir.Node // type hash of the value being type-switched on
	okName   ir.Node // boolean used for comma-ok type assertions
	itabName ir.Node // itab value to use for first word of non-empty interface
}

type typeClause struct {
	hash uint32
	body ir.Nodes
}

func (s *typeSwitch) flush(cc []typeClause, compiled *ir.Nodes) {
	if len(cc) == 0 {
		return
	}

	slices.SortFunc(cc, func(a, b typeClause) int { return cmp.Compare(a.hash, b.hash) })

	// Combine adjacent cases with the same hash.
	merged := cc[:1]
	for _, c := range cc[1:] {
		last := &merged[len(merged)-1]
		if last.hash == c.hash {
			last.body.Append(c.body.Take()...)
		} else {
			merged = append(merged, c)
		}
	}
	cc = merged

	if s.tryJumpTable(cc, compiled) {
		return
	}
	binarySearch(len(cc), compiled,
		func(i int) ir.Node {
			return ir.NewBinaryExpr(base.Pos, ir.OLE, s.hashName, ir.NewInt(base.Pos, int64(cc[i-1].hash)))
		},
		func(i int, nif *ir.IfStmt) {
			// TODO(mdempsky): Omit hash equality check if
			// there's only one type.
			c := cc[i]
			nif.Cond = ir.NewBinaryExpr(base.Pos, ir.OEQ, s.hashName, ir.NewInt(base.Pos, int64(c.hash)))
			nif.Body.Append(c.body.Take()...)
		},
	)
}

// Try to implement the clauses with a jump table. Returns true if successful.
func (s *typeSwitch) tryJumpTable(cc []typeClause, out *ir.Nodes) bool {
	const minCases = 5 // have at least minCases cases in the switch
	if base.Flag.N != 0 || !ssagen.Arch.LinkArch.CanJumpTable || base.Ctxt.Retpoline {
		return false
	}
	if len(cc) < minCases {
		return false // not enough cases for it to be worth it
	}
	hashes := make([]uint32, len(cc))
	// b = # of bits to use. Start with the minimum number of
	// bits possible, but try a few larger sizes if needed.
	b0 := bits.Len(uint(len(cc) - 1))
	for b := b0; b < b0+3; b++ {
	pickI:
		for i := 0; i <= 32-b; i++ { // starting bit position
			// Compute the hash we'd get from all the cases,
			// selecting b bits starting at bit i.
			hashes = hashes[:0]
			for _, c := range cc {
				h := c.hash >> i & (1<<b - 1)
				hashes = append(hashes, h)
			}
			// Order by increasing hash.
			slices.Sort(hashes)
			for j := 1; j < len(hashes); j++ {
				if hashes[j] == hashes[j-1] {
					// There is a duplicate hash; try a different b/i pair.
					continue pickI
				}
			}

			// All hashes are distinct. Use these values of b and i.
			h := s.hashName
			if i != 0 {
				h = ir.NewBinaryExpr(base.Pos, ir.ORSH, h, ir.NewInt(base.Pos, int64(i)))
			}
			h = ir.NewBinaryExpr(base.Pos, ir.OAND, h, ir.NewInt(base.Pos, int64(1<<b-1)))
			h = typecheck.Expr(h)

			// Build jump table.
			jt := ir.NewJumpTableStmt(base.Pos, h)
			jt.Cases = make([]constant.Value, 1<<b)
			jt.Targets = make([]*types.Sym, 1<<b)
			out.Append(jt)

			// Start with all hashes going to the didn't-match target.
			noMatch := typecheck.AutoLabel(".s")
			for j := 0; j < 1<<b; j++ {
				jt.Cases[j] = constant.MakeInt64(int64(j))
				jt.Targets[j] = noMatch
			}
			// This statement is not reachable, but it will make it obvious that we don't
			// fall through to the first case.
			out.Append(ir.NewBranchStmt(base.Pos, ir.OGOTO, noMatch))

			// Emit each of the actual cases.
			for _, c := range cc {
				h := c.hash >> i & (1<<b - 1)
				label := typecheck.AutoLabel(".s")
				jt.Targets[h] = label
				out.Append(ir.NewLabelStmt(base.Pos, label))
				out.Append(c.body...)
				// We reach here if the hash matches but the type equality test fails.
				out.Append(ir.NewBranchStmt(base.Pos, ir.OGOTO, noMatch))
			}
			// Emit point to go to if type doesn't match any case.
			out.Append(ir.NewLabelStmt(base.Pos, noMatch))
			return true
		}
	}
	// Couldn't find a perfect hash. Fall back to binary search.
	return false
}

// binarySearch constructs a binary search tree for handling n cases,
// and appends it to out. It's used for efficiently implementing
// switch statements.
//
// less(i) should return a boolean expression. If it evaluates true,
// then cases before i will be tested; otherwise, cases i and later.
//
// leaf(i, nif) should setup nif (an OIF node) to test case i. In
// particular, it should set nif.Cond and nif.Body.
func binarySearch(n int, out *ir.Nodes, less func(i int) ir.Node, leaf func(i int, nif *ir.IfStmt)) {
	const binarySearchMin = 4 // minimum number of cases for binary search

	var do func(lo, hi int, out *ir.Nodes)
	do = func(lo, hi int, out *ir.Nodes) {
		n := hi - lo
		if n < binarySearchMin {
			for i := lo; i < hi; i++ {
				nif := ir.NewIfStmt(base.Pos, nil, nil, nil)
				leaf(i, nif)
				base.Pos = base.Pos.WithNotStmt()
				nif.Cond = typecheck.Expr(nif.Cond)
				nif.Cond = typecheck.DefaultLit(nif.Cond, nil)
				out.Append(nif)
				out = &nif.Else
			}
			return
		}

		half := lo + n/2
		nif := ir.NewIfStmt(base.Pos, nil, nil, nil)
		nif.Cond = less(half)
		base.Pos = base.Pos.WithNotStmt()
		nif.Cond = typecheck.Expr(nif.Cond)
		nif.Cond = typecheck.DefaultLit(nif.Cond, nil)
		do(lo, half, &nif.Body)
		do(half, hi, &nif.Else)
		out.Append(nif)
	}

	do(0, n, out)
}

func stringSearch(expr ir.Node, cc []exprClause, out *ir.Nodes) {
	if len(cc) < 4 {
		// Short list, just do brute force equality checks.
		for _, c := range cc {
			nif := ir.NewIfStmt(base.Pos.WithNotStmt(), typecheck.DefaultLit(typecheck.Expr(c.test(expr)), nil), []ir.Node{c.jmp}, nil)
			out.Append(nif)
			out = &nif.Else
		}
		return
	}

	// The strategy here is to find a simple test to divide the set of possible strings
	// that might match expr approximately in half.
	// The test we're going to use is to do an ordered comparison of a single byte
	// of expr to a constant. We will pick the index of that byte and the value we're
	// comparing against to make the split as even as possible.
	//   if expr[3] <= 'd' { ... search strings with expr[3] at 'd' or lower  ... }
	//   else              { ... search strings with expr[3] at 'e' or higher ... }
	//
	// To add complication, we will do the ordered comparison in the signed domain.
	// The reason for this is to prevent CSE from merging the load used for the
	// ordered comparison with the load used for the later equality check.
	//   if expr[3] <= 'd' { ... if expr[0] == 'f' && expr[1] == 'o' && expr[2] == 'o' && expr[3] == 'd' { ... } }
	// If we did both expr[3] loads in the unsigned domain, they would be CSEd, and that
	// would in turn defeat the combining of expr[0]...expr[3] into a single 4-byte load.
	// See issue 48222.
	// By using signed loads for the ordered comparison and unsigned loads for the
	// equality comparison, they don't get CSEd and the equality comparisons will be
	// done using wider loads.

	n := len(ir.StringVal(cc[0].lo)) // Length of the constant strings.
	bestScore := int64(0)            // measure of how good the split is.
	bestIdx := 0                     // split using expr[bestIdx]
	bestByte := int8(0)              // compare expr[bestIdx] against bestByte
	for idx := 0; idx < n; idx++ {
		for b := int8(-128); b < 127; b++ {
			le := 0
			for _, c := range cc {
				s := ir.StringVal(c.lo)
				if int8(s[idx]) <= b {
					le++
				}
			}
			score := int64(le) * int64(len(cc)-le)
			if score > bestScore {
				bestScore = score
				bestIdx = idx
				bestByte = b
			}
		}
	}

	// The split must be at least 1:n-1 because we have at least 2 distinct strings; they
	// have to be different somewhere.
	// TODO: what if the best split is still pretty bad?
	if bestScore == 0 {
		base.Fatalf("unable to split string set")
	}

	// Convert expr to a []int8
	slice := ir.NewConvExpr(base.Pos, ir.OSTR2BYTESTMP, types.NewSlice(types.Types[types.TINT8]), expr)
	slice.SetTypecheck(1) // legacy typechecker doesn't handle this op
	slice.MarkNonNil()
	// Load the byte we're splitting on.
	load := ir.NewIndexExpr(base.Pos, slice, ir.NewInt(base.Pos, int64(bestIdx)))
	// Compare with the value we're splitting on.
	cmp := ir.Node(ir.NewBinaryExpr(base.Pos, ir.OLE, load, ir.NewInt(base.Pos, int64(bestByte))))
	cmp = typecheck.DefaultLit(typecheck.Expr(cmp), nil)
	nif := ir.NewIfStmt(base.Pos, cmp, nil, nil)

	var le []exprClause
	var gt []exprClause
	for _, c := range cc {
		s := ir.StringVal(c.lo)
		if int8(s[bestIdx]) <= bestByte {
			le = append(le, c)
		} else {
			gt = append(gt, c)
		}
	}
	stringSearch(expr, le, &nif.Body)
	stringSearch(expr, gt, &nif.Else)
	out.Append(nif)

	// TODO: if expr[bestIdx] has enough different possible values, use a jump table.
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/temp.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
)

// initStackTemp appends statements to init to initialize the given
// temporary variable to val, and then returns the expression &tmp.
func initStackTemp(init *ir.Nodes, tmp *ir.Name, val ir.Node) *ir.AddrExpr {
	if val != nil && !types.Identical(tmp.Type(), val.Type()) {
		base.Fatalf("bad initial value for %L: %L", tmp, val)
	}
	appendWalkStmt(init, ir.NewAssignStmt(base.Pos, tmp, val))
	return typecheck.Expr(typecheck.NodAddr(tmp)).(*ir.AddrExpr)
}

// stackTempAddr returns the expression &tmp, where tmp is a newly
// allocated temporary variable of the given type. Statements to
// zero-initialize tmp are appended to init.
func stackTempAddr(init *ir.Nodes, typ *types.Type) *ir.AddrExpr {
	n := typecheck.TempAt(base.Pos, ir.CurFunc, typ)
	n.SetNonMergeable(true)
	return initStackTemp(init, n, nil)
}

// stackBufAddr returns the expression &tmp, where tmp is a newly
// allocated temporary variable of type [len]elem. This variable is
// initialized, and elem must not contain pointers.
func stackBufAddr(len int64, elem *types.Type) *ir.AddrExpr {
	if elem.HasPointers() {
		base.FatalfAt(base.Pos, "%v has pointers", elem)
	}
	tmp := typecheck.TempAt(base.Pos, ir.CurFunc, types.NewArray(elem, len))
	return typecheck.Expr(typecheck.NodAddr(tmp)).(*ir.AddrExpr)
}

```

// === FILE: references/go/src/cmd/compile/internal/walk/walk.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import (
	"fmt"
	"internal/abi"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/rttype"
	"cmd/compile/internal/ssagen"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// The constant is known to runtime.
const tmpstringbufsize = 32

func Walk(fn *ir.Func) {
	ir.CurFunc = fn

	// Build pre-walk analysis caches with a single AST traversal.
	// (At some point, it might be worthwhile to have a walkState structure
	// that gets passed everywhere where things like this can go.)
	analyzePreWalk(fn)
	defer func() { staticValues = nil; shapeConvSources = nil }()

	errorsBefore := base.Errors()
	order(fn)
	if base.Errors() > errorsBefore {
		return
	}

	if base.Flag.W != 0 {
		s := fmt.Sprintf("\nbefore walk %v", ir.CurFunc.Sym())
		ir.DumpList(s, ir.CurFunc.Body)
	}

	walkStmtList(ir.CurFunc.Body)
	if base.Flag.W != 0 {
		s := fmt.Sprintf("after walk %v", ir.CurFunc.Sym())
		ir.DumpList(s, ir.CurFunc.Body)
	}

	// Eagerly compute sizes of all variables for SSA.
	for _, n := range fn.Dcl {
		types.CalcSize(n.Type())
	}
}

// walkRecv walks an ORECV node.
func walkRecv(n *ir.UnaryExpr) ir.Node {
	if n.Typecheck() == 0 {
		base.Fatalf("missing typecheck: %+v", n)
	}
	init := ir.TakeInit(n)

	n.X = walkExpr(n.X, &init)
	call := walkExpr(mkcall1(chanfn("chanrecv1", 2, n.X.Type()), nil, &init, n.X, typecheck.NodNil()), &init)
	return ir.InitExpr(init, call)
}

func convas(n *ir.AssignStmt, init *ir.Nodes) *ir.AssignStmt {
	if n.Op() != ir.OAS {
		base.Fatalf("convas: not OAS %v", n.Op())
	}
	n.SetTypecheck(1)

	if n.X == nil || n.Y == nil {
		return n
	}

	lt := n.X.Type()
	rt := n.Y.Type()
	if lt == nil || rt == nil {
		return n
	}

	if ir.IsBlank(n.X) {
		n.Y = typecheck.DefaultLit(n.Y, nil)
		return n
	}

	if !types.Identical(lt, rt) {
		n.Y = typecheck.AssignConv(n.Y, lt, "assignment")
		n.Y = walkExpr(n.Y, init)
	}
	types.CalcSize(n.Y.Type())

	return n
}

func vmkcall(fn ir.Node, t *types.Type, init *ir.Nodes, va []ir.Node) *ir.CallExpr {
	if init == nil {
		base.Fatalf("mkcall with nil init: %v", fn)
	}
	if fn.Type() == nil || fn.Type().Kind() != types.TFUNC {
		base.Fatalf("mkcall %v %v", fn, fn.Type())
	}

	n := fn.Type().NumParams()
	if n != len(va) {
		base.Fatalf("vmkcall %v needs %v args got %v", fn, n, len(va))
	}

	call := typecheck.Call(base.Pos, fn, va, false).(*ir.CallExpr)
	call.SetType(t)
	return walkExpr(call, init).(*ir.CallExpr)
}

func mkcall(name string, t *types.Type, init *ir.Nodes, args ...ir.Node) *ir.CallExpr {
	return vmkcall(typecheck.LookupRuntime(name), t, init, args)
}

func mkcallstmt(name string, args ...ir.Node) ir.Node {
	return mkcallstmt1(typecheck.LookupRuntime(name), args...)
}

func mkcall1(fn ir.Node, t *types.Type, init *ir.Nodes, args ...ir.Node) *ir.CallExpr {
	return vmkcall(fn, t, init, args)
}

func mkcallstmt1(fn ir.Node, args ...ir.Node) ir.Node {
	var init ir.Nodes
	n := vmkcall(fn, nil, &init, args)
	if len(init) == 0 {
		return n
	}
	init.Append(n)
	return ir.NewBlockStmt(n.Pos(), init)
}

func chanfn(name string, n int, t *types.Type) ir.Node {
	if !t.IsChan() {
		base.Fatalf("chanfn %v", t)
	}
	switch n {
	case 1:
		return typecheck.LookupRuntime(name, t.Elem())
	case 2:
		return typecheck.LookupRuntime(name, t.Elem(), t.Elem())
	}
	base.Fatalf("chanfn %d", n)
	return nil
}

func mapfn(name string, t *types.Type, isfat bool) ir.Node {
	if !t.IsMap() {
		base.Fatalf("mapfn %v", t)
	}
	if mapfast(t) == mapslow || isfat {
		return typecheck.LookupRuntime(name, t.Key(), t.Elem(), t.Key(), t.Elem())
	}
	return typecheck.LookupRuntime(name, t.Key(), t.Elem(), t.Elem())
}

func mapfndel(name string, t *types.Type) ir.Node {
	if !t.IsMap() {
		base.Fatalf("mapfn %v", t)
	}
	if mapfast(t) == mapslow {
		return typecheck.LookupRuntime(name, t.Key(), t.Elem(), t.Key())
	}
	return typecheck.LookupRuntime(name, t.Key(), t.Elem())
}

const (
	mapslow = iota
	mapfast32
	mapfast32ptr
	mapfast64
	mapfast64ptr
	mapfaststr
	nmapfast
)

type mapnames [nmapfast]string

func mkmapnames(base string, ptr string) mapnames {
	return mapnames{base, base + "_fast32", base + "_fast32" + ptr, base + "_fast64", base + "_fast64" + ptr, base + "_faststr"}
}

var mapaccess1 = mkmapnames("mapaccess1", "")
var mapaccess2 = mkmapnames("mapaccess2", "")
var mapassign = mkmapnames("mapassign", "ptr")
var mapdelete = mkmapnames("mapdelete", "")

func mapfast(t *types.Type) int {
	if t.Elem().Size() > abi.MapMaxElemBytes {
		return mapslow
	}
	switch algType(t.Key()) {
	case types.AMEM32:
		if !t.Key().HasPointers() {
			return mapfast32
		}
		if types.PtrSize == 4 {
			return mapfast32ptr
		}
		base.Fatalf("small pointer %v", t.Key())
	case types.AMEM64:
		if !t.Key().HasPointers() {
			return mapfast64
		}
		if types.PtrSize == 8 {
			return mapfast64ptr
		}
		// Two-word object, at least one of which is a pointer.
		// Use the slow path.
	case types.ASTRING:
		return mapfaststr
	}
	return mapslow
}

// algType returns the fixed-width AMEMxx variants instead of the general
// AMEM kind when possible.
func algType(t *types.Type) types.AlgKind {
	a := types.AlgType(t)
	if a == types.AMEM {
		if t.Alignment() < int64(base.Ctxt.Arch.Alignment) && t.Alignment() < t.Size() {
			// For example, we can't treat [2]int16 as an int32 if int32s require
			// 4-byte alignment. See issue 46283.
			return a
		}
		switch t.Size() {
		case 0:
			return types.AMEM0
		case 1:
			return types.AMEM8
		case 2:
			return types.AMEM16
		case 4:
			return types.AMEM32
		case 8:
			return types.AMEM64
		case 16:
			return types.AMEM128
		}
	}

	return a
}

func walkAppendArgs(n *ir.CallExpr, init *ir.Nodes) {
	walkExprListSafe(n.Args, init)

	// walkExprListSafe will leave OINDEX (s[n]) alone if both s
	// and n are name or literal, but those may index the slice we're
	// modifying here. Fix explicitly.
	ls := n.Args
	for i1, n1 := range ls {
		ls[i1] = cheapExpr(n1, init)
	}
}

// appendWalkStmt typechecks and walks stmt and then appends it to init.
func appendWalkStmt(init *ir.Nodes, stmt ir.Node) {
	op := stmt.Op()
	n := typecheck.Stmt(stmt)
	if op == ir.OAS || op == ir.OAS2 {
		// If the assignment has side effects, walkExpr will append them
		// directly to init for us, while walkStmt will wrap it in an OBLOCK.
		// We need to append them directly.
		// TODO(rsc): Clean this up.
		n = walkExpr(n, init)
	} else {
		n = walkStmt(n)
	}
	init.Append(n)
}

// The max number of defers in a function using open-coded defers. We enforce this
// limit because the deferBits bitmask is currently a single byte (to minimize code size)
const maxOpenDefers = 8

// backingArrayPtrLen extracts the pointer and length from a slice or string.
// This constructs two nodes referring to n, so n must be a cheapExpr.
func backingArrayPtrLen(n ir.Node) (ptr, length ir.Node) {
	var init ir.Nodes
	c := cheapExpr(n, &init)
	if c != n || len(init) != 0 {
		base.Fatalf("backingArrayPtrLen not cheap: %v", n)
	}
	ptr = ir.NewUnaryExpr(base.Pos, ir.OSPTR, n)
	if n.Type().IsString() {
		ptr.SetType(types.Types[types.TUINT8].PtrTo())
	} else {
		ptr.SetType(n.Type().Elem().PtrTo())
	}
	ptr.SetTypecheck(1)
	length = ir.NewUnaryExpr(base.Pos, ir.OLEN, n)
	length.SetType(types.Types[types.TINT])
	length.SetTypecheck(1)
	return ptr, length
}

// mayCall reports whether evaluating expression n may require
// function calls, which could clobber function call arguments/results
// currently on the stack.
func mayCall(n ir.Node) bool {
	// This is intended to avoid putting constants
	// into temporaries with the race detector (or other
	// instrumentation) which interferes with simple
	// "this is a constant" tests in ssagen.
	// Also, it will generally lead to better code.
	if n.Op() == ir.OLITERAL {
		return false
	}

	// When instrumenting, any expression might require function calls.
	if base.Flag.Cfg.Instrumenting {
		return true
	}

	isSoftFloat := func(typ *types.Type) bool {
		return types.IsFloat[typ.Kind()] || types.IsComplex[typ.Kind()]
	}

	return ir.Any(n, func(n ir.Node) bool {
		// walk should have already moved any Init blocks off of
		// expressions.
		if len(n.Init()) != 0 {
			base.FatalfAt(n.Pos(), "mayCall %+v", n)
		}

		switch n.Op() {
		default:
			base.FatalfAt(n.Pos(), "mayCall %+v", n)

		case ir.OCALLFUNC, ir.OCALLINTER,
			ir.OUNSAFEADD, ir.OUNSAFESLICE:
			return true

		case ir.OINDEX, ir.OSLICE, ir.OSLICEARR, ir.OSLICE3, ir.OSLICE3ARR, ir.OSLICESTR,
			ir.ODEREF, ir.ODOTPTR, ir.ODOTTYPE, ir.ODYNAMICDOTTYPE, ir.ODIV, ir.OMOD,
			ir.OSLICE2ARR, ir.OSLICE2ARRPTR:
			// These ops might panic, make sure they are done
			// before we start marshaling args for a call. See issue 16760.
			return true

		case ir.OANDAND, ir.OOROR:
			n := n.(*ir.LogicalExpr)
			// The RHS expression may have init statements that
			// should only execute conditionally, and so cannot be
			// pulled out to the top-level init list. We could try
			// to be more precise here.
			return len(n.Y.Init()) != 0

		// When using soft-float, these ops might be rewritten to function calls
		// so we ensure they are evaluated first.
		case ir.OADD, ir.OSUB, ir.OMUL, ir.ONEG:
			return ssagen.Arch.SoftFloat && isSoftFloat(n.Type())
		case ir.OLT, ir.OEQ, ir.ONE, ir.OLE, ir.OGE, ir.OGT:
			n := n.(*ir.BinaryExpr)
			return ssagen.Arch.SoftFloat && isSoftFloat(n.X.Type())
		case ir.OCONV:
			n := n.(*ir.ConvExpr)
			return ssagen.Arch.SoftFloat && (isSoftFloat(n.Type()) || isSoftFloat(n.X.Type()))

		case ir.OMIN, ir.OMAX:
			// string or float requires runtime call, see (*ssagen.state).minmax method.
			return n.Type().IsString() || n.Type().IsFloat()

		case ir.OLITERAL, ir.ONIL, ir.ONAME, ir.OLINKSYMOFFSET, ir.OMETHEXPR,
			ir.OAND, ir.OANDNOT, ir.OLSH, ir.OOR, ir.ORSH, ir.OXOR, ir.OCOMPLEX, ir.OMAKEFACE,
			ir.OADDR, ir.OBITNOT, ir.ONOT, ir.OPLUS,
			ir.OCAP, ir.OIMAG, ir.OLEN, ir.OREAL,
			ir.OCONVNOP, ir.ODOT,
			ir.OCFUNC, ir.OIDATA, ir.OITAB, ir.OSPTR,
			ir.OBYTES2STRTMP, ir.OGETG, ir.OGETCALLERSP, ir.OSLICEHEADER, ir.OSTRINGHEADER:
			// ok: operations that don't require function calls.
			// Expand as needed.
		}

		return false
	})
}

// itabType loads the _type field from a runtime.itab struct.
func itabType(itab ir.Node) ir.Node {
	if itabTypeField == nil {
		// internal/abi.ITab's Type field
		itabTypeField = runtimeField("Type", rttype.ITab.OffsetOf("Type"), types.NewPtr(types.Types[types.TUINT8]))
	}
	return boundedDotPtr(base.Pos, itab, itabTypeField)
}

var itabTypeField *types.Field

// boundedDotPtr returns a selector expression representing ptr.field
// and omits nil-pointer checks for ptr.
func boundedDotPtr(pos src.XPos, ptr ir.Node, field *types.Field) *ir.SelectorExpr {
	sel := ir.NewSelectorExpr(pos, ir.ODOTPTR, ptr, field.Sym)
	sel.Selection = field
	sel.SetType(field.Type)
	sel.SetTypecheck(1)
	sel.SetBounded(true) // guaranteed not to fault
	return sel
}

func runtimeField(name string, offset int64, typ *types.Type) *types.Field {
	f := types.NewField(src.NoXPos, ir.Pkgs.Runtime.Lookup(name), typ)
	f.Offset = offset
	return f
}

// ifaceData loads the data field from an interface.
// The concrete type must be known to have type t.
// It follows the pointer if !IsDirectIface(t).
func ifaceData(pos src.XPos, n ir.Node, t *types.Type) ir.Node {
	if t.IsInterface() {
		base.Fatalf("ifaceData interface: %v", t)
	}
	ptr := ir.NewUnaryExpr(pos, ir.OIDATA, n)
	if types.IsDirectIface(t) {
		ptr.SetType(t)
		ptr.SetTypecheck(1)
		return ptr
	}
	ptr.SetType(types.NewPtr(t))
	ptr.SetTypecheck(1)
	ind := ir.NewStarExpr(pos, ptr)
	ind.SetType(t)
	ind.SetTypecheck(1)
	ind.SetBounded(true)
	return ind
}

// staticValue returns the earliest expression it can find that always
// evaluates to n, with similar semantics to [ir.StaticValue].
//
// It only returns results for the ir.CurFunc being processed in [Walk],
// including its closures, and uses a cache to reduce duplicative work.
// It can return n or nil if it does not find an earlier expression.
//
// The current use case is reducing OCONVIFACE allocations, and hence
// staticValue is currently only useful when given an *ir.ConvExpr.X as n.
func staticValue(n ir.Node) ir.Node {
	if staticValues == nil {
		base.Fatalf("staticValues is nil. staticValue called outside of walk.Walk?")
	}
	return staticValues[n]
}

// staticValues is a cache of static values for use by staticValue.
var staticValues map[ir.Node]ir.Node

// shapeConvSources maps an *ir.Name (a PAUTO interface variable) to
// the shape type of the OCONVIFACE expression that is its single
// static value, if any.
var shapeConvSources map[*ir.Name]*types.Type

// analyzePreWalk populates staticValues and shapeConvSources using a
// single AST traversal. We can't use an ir.ReassignOracle or
// ir.StaticValue in the middle of walk because they don't currently
// handle transformed assignments (e.g., will complain about
// 'RHS == nil'). So we build these maps before walk begins.
func analyzePreWalk(fn *ir.Func) {
	ro := &ir.ReassignOracle{}
	ro.Init(fn)
	sv := make(map[ir.Node]ir.Node)
	scs := make(map[*ir.Name]*types.Type)
	ir.Visit(fn, func(n ir.Node) {
		switch n.Op() {
		case ir.OCONVIFACE:
			x := n.(*ir.ConvExpr).X
			v := ro.StaticValue(x)
			if v != nil && v != x {
				sv[x] = v
			}
		case ir.ONAME:
			name := n.(*ir.Name).Canonical()
			if name.Class != ir.PAUTO || name.Type() == nil || !name.Type().IsInterface() {
				return
			}
			val := ro.StaticValue(name)
			if val == nil || val.Op() != ir.OCONVIFACE {
				return
			}
			srcType := val.(*ir.ConvExpr).X.Type()
			if srcType != nil && !srcType.IsInterface() && srcType.IsShape() {
				scs[name] = srcType
			}
		}
	})
	staticValues = sv
	shapeConvSources = scs
}

```

