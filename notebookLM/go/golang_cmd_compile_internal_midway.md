# Domain Architecture: cmd/compile/internal/midway

## Layout Topology
```text
cmd/compile/internal/midway/
├── analysis.go
├── check.go
├── deepcopy.go
├── midway.go
└── rewrite.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/cmd/compile/internal/midway/analysis.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package midway

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
)

// Analyzer holds the state for SIMD dependency analysis
type Analyzer struct {
	pkg                *types2.Package
	info               *types2.Info
	isDependentObj     map[types2.Object]bool // does an Object depend on a simd type in some way?
	isDependentMethod  map[types2.Object]bool // is this dependent Object also a method? (methods are not renamed, their types are)
	hasDependentMethod map[types2.Type]bool   // is this a type that has a dependent method?
	visited            map[types2.Type]bool   // if in map, type has been visited, and value is whether type is dependent
	inSimd             bool                   // true if the current package is the simd package (which is a special case)
}

func NewAnalyzer(pkg *types2.Package, info *types2.Info) *Analyzer {
	return &Analyzer{
		pkg:                pkg,
		info:               info,
		isDependentObj:     make(map[types2.Object]bool),
		isDependentMethod:  make(map[types2.Object]bool),
		hasDependentMethod: make(map[types2.Type]bool),
		visited:            make(map[types2.Type]bool),
		inSimd:             pkg.Path() == simdPkg,
	}
}

// Analyze builds the set of SIMD-dependent objects
func (a *Analyzer) Analyze(files []*syntax.File) bool {
	// Phase 1: Seed dependence from types and signatures
	hdmsize := len(a.hasDependentMethod)

	for {
		for _, obj := range a.info.Defs {
			if obj != nil {
				a.markIfDependent(obj)
			}
		}
		for _, obj := range a.info.Uses {
			if obj != nil {
				a.markIfDependent(obj)
			}
		}
		if hdmsize == len(a.hasDependentMethod) {
			break
		}
		if base.Debug.Simd > 0 {
			base.Warn("hasDependentMethod increased from %d to %d", hdmsize, len(a.hasDependentMethod))
		}
		hdmsize = len(a.hasDependentMethod)
		clear(a.visited)
	}

	// Phase 2: Transitive closure via function bodies
	changed := true
	for changed {
		changed = false
		for _, file := range files {
			for _, decl := range file.DeclList {
				if fn, ok := decl.(*syntax.FuncDecl); ok {
					if fn.Name == nil {
						continue
					}
					obj := a.info.Defs[fn.Name]
					if obj == nil || a.isDependentObj[obj] {
						continue
					}

					if a.hasBodyDependency(fn) {
						a.isDependentObj[obj] = true
						changed = true
					}
				}
			}
		}
	}

	return len(a.isDependentObj) > 0
}

func (a *Analyzer) hasBodyDependency(fn *syntax.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}
	// Walk the body and check identifiers
	// This will also note any variable references that are dependent.
	found := false
	syntax.Inspect(fn.Body, func(n syntax.Node) bool {
		if id, ok := n.(*syntax.Name); ok {
			obj := a.info.Uses[id]
			if obj == nil {
				obj = a.info.Defs[id]
			}
			if obj != nil {
				if _, isFunc := obj.(*types2.Func); !isFunc {
					if a.isDependentObj[obj] {
						found = true
						return false
					}
				} else {
					sig := obj.Type().(*types2.Signature)
					if a.HasDependentSignature(sig) {
						found = true
						return false
					}
				}
				if a.isDependentType(obj.Type()) {
					// Whatever this is, it makes the outer object dependent.
					// If this is a package variable with dependent type, mark the
					// variable as dependent, so that references to it become dependent.
					if obj, ok := obj.(*types2.Var); ok && obj.Kind() == types2.PackageVar {
						// everything else is nested within a dependent function/struct/scope
						// and does not need its own renaming
						a.isDependentObj[obj] = true
					}
					found = true
					return false
				}
				if isBaseSimdTypeObj(obj) {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}

func (a *Analyzer) markIfDependent(obj types2.Object) bool {
	if a.isDependentObj[obj] {
		return true
	}

	isDep := false
	isDepMeth := false
	switch obj := obj.(type) {
	case *types2.Var:
		if obj.Pkg() == a.pkg && obj.Parent() == a.pkg.Scope() {
			isDep = a.isDependentType(obj.Type())
		}
	case *types2.TypeName:
		isDep = a.isDependentType(obj.Type())
	case *types2.Func:
		sig := obj.Type().(*types2.Signature)
		if a.HasDependentSignature(sig) {
			// NOT dependent if it is a method of one of the base SIMD types.
			// TODO: what about aliases of base SIMD types?
			if rcv := sig.Recv(); rcv == nil {
				isDep = true
			} else if named, ok := rcv.Type().(*types2.Named); !ok || !isBaseSimdType(named) {
				isDep = true
				t := rcv.Type()
				if !a.isDependentType(t) {
					a.markHasMethod(t)
				}
				isDepMeth = true
			}
		}
	}

	// Also check if obj name is "simd.Type" (base case)
	if isBaseSimdTypeObj(obj) {
		isDep = true
	}

	if isDep {
		if base.Debug.Simd > 0 {
			base.Warn("%s: %v is simd-dependent", obj.Pos().String(), obj)
		}
		a.isDependentObj[obj] = true
	}
	if isDepMeth {
		if base.Debug.Simd > 0 {
			base.Warn("%s: %v is simd-dependent method", obj.Pos().String(), obj)
		}
		a.isDependentMethod[obj] = true
	}
	return isDep
}

func (a *Analyzer) isDependentType(t types2.Type) bool {
	return a.checkTypeRecursive(t)
}

func (a *Analyzer) checkTypeRecursive(t types2.Type) bool {
	if t == nil {
		return false
	}
	if a.hasDependentMethod[t] {
		a.visited[t] = true
	}
	if b, ok := a.visited[t]; ok {
		return b // Break cycles
	}
	a.visited[t] = false

	memo := func(b bool) bool {
		a.visited[t] = b
		return b
	}

	// Unwrap aliases
	if named, ok := t.(*types2.Named); ok {
		if isBaseSimdType(named) {
			return memo(true)
		}
		if a.checkTypeRecursive(named.Underlying()) {
			return memo(true)
		}
	}

	switch t := t.(type) {
	case *types2.Basic:
		return false
	case *types2.Pointer:
		return memo(a.checkTypeRecursive(t.Elem()))
	case *types2.Slice:
		return memo(a.checkTypeRecursive(t.Elem()))
	case *types2.Array:
		return memo(a.checkTypeRecursive(t.Elem()))
	case *types2.Map:
		return memo(a.checkTypeRecursive(t.Key()) ||
			a.checkTypeRecursive(t.Elem()))
	case *types2.Chan:
		return memo(a.checkTypeRecursive(t.Elem()))
	case *types2.Struct:
		for i := 0; i < t.NumFields(); i++ {
			if a.checkTypeRecursive(t.Field(i).Type()) {
				return memo(true)
			}
		}
	case *types2.Signature:
		return memo(a.HasDependentSignature(t))
	case *types2.Tuple:
		for i := 0; i < t.Len(); i++ {
			if a.checkTypeRecursive(t.At(i).Type()) {
				return memo(true)
			}
		}
	case *types2.Alias:
		return memo(a.checkTypeRecursive(types2.Unalias(t)))
	}
	return false
}

// This attempts to mark types that are not otherwise dependent
// as being dependent, if they have a method with a dependent
// signature.
func (a *Analyzer) markHasMethod(t types2.Type) {
	if t == nil {
		return
	}
	if a.hasDependentMethod[t] {
		return
	}

	a.hasDependentMethod[t] = true

	switch t := t.(type) {
	case *types2.Pointer:
		a.markHasMethod(t.Elem())
	case *types2.Alias:
		a.markHasMethod(t.Rhs())
	}
	return
}

func isBaseSimdType(t *types2.Named) bool {
	return isBaseSimdTypeObj(t.Obj())
}

func isBaseSimdTypeObj(obj types2.Object) bool {
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	if obj.Pkg().Path() != simdPkg {
		return false
	}
	return isSimdTypeName(obj.Name())
}

func (a *Analyzer) HasDependentSignature(sig *types2.Signature) bool {
	// TODO what about type parameters?  Need to invent a test that provokes that case.
	return a.isDependentType(sig.Params()) ||
		a.isDependentType(sig.Results()) ||
		(sig.Recv() != nil && a.isDependentType(sig.Recv().Type()))
}

```

// === FILE: references!/go/src/cmd/compile/internal/midway/check.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package midway

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/syntax"
)

// CheckPositions checks that all nodes in the files have known positions.
// This converts lack-of-Pos into an early fatal error instead of a later
// weird downstream error (e.g., in the linker, in debugging information).
func CheckPositions(files []*syntax.File, phase string) {
	for _, file := range files {
		syntax.Inspect(file, func(n syntax.Node) bool {
			if n == nil {
				return true
			}
			if !n.Pos().IsKnown() {
				base.Fatalf("Phase %s, Node without known position: %T\n", phase, n)
			}
			return true
		})
	}
}

```

// === FILE: references!/go/src/cmd/compile/internal/midway/deepcopy.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package midway

import (
	"fmt"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
)

// DeepCopier clones syntax nodes and maintains types2.Info mappings.
type DeepCopier struct {
	VecLen   int
	info     *types2.Info
	pkg      *types2.Package
	analyzer *Analyzer
	suffix   string

	vars map[*types2.Var]*types2.Var
}

func NewDeepCopier(pkg *types2.Package, info *types2.Info, vecLen int, analyzer *Analyzer, suffix string) *DeepCopier {
	return &DeepCopier{
		VecLen:   vecLen,
		info:     info,
		pkg:      pkg,
		analyzer: analyzer,
		suffix:   suffix,
		vars:     make(map[*types2.Var]*types2.Var),
	}
}

func (c *DeepCopier) registerDef(newName *syntax.Name, oldName *syntax.Name) {
	if oldName == nil || newName == nil {
		return
	}
	if oldObj := c.info.Defs[oldName]; oldObj != nil {
		if val, isVar := oldObj.(*types2.Var); isVar {
			newObj := types2.NewVar(newName.Pos(), c.pkg, newName.Value, val.Type())
			c.vars[val] = newObj
			c.info.Defs[newName] = newObj
		} else {
			c.info.Defs[newName] = oldObj
		}
	}
}

func (c *DeepCopier) mapUse(newName *syntax.Name, oldName *syntax.Name) {
	if oldName == nil || newName == nil {
		return
	}
	if oldObj := c.info.Uses[oldName]; oldObj != nil {
		if val, isVar := oldObj.(*types2.Var); isVar && c.vars[val] != nil {
			c.info.Uses[newName] = c.vars[val]
		} else {
			c.info.Uses[newName] = oldObj
		}
	}
}

// OnName rewrites "dependent" and SIMD names to their architecture-specific version.
func (c *DeepCopier) OnName(id *syntax.Name) *syntax.Name {
	obj := c.info.Uses[id]
	if obj == nil {
		obj = c.info.Defs[id]
	}
	if obj == nil {
		return nil
	}
	// Don't rename methods of dependent types
	if c.analyzer.isDependentMethod[obj] {
		return nil
	}

	if c.analyzer.isDependentObj[obj] || isBaseSimdTypeObj(obj) {
		newId := syntax.NewName(id.Pos(), id.Value+c.suffix)
		// Object link will be handled manually in deepcopier Use/Def mapper
		if base.Debug.Simd > 0 {
			base.Warn("%s: rewriting name %s to %s", id.Pos().String(), id.Value, newId.Value)
		}
		return newId
	}
	return nil
}

// OnNameExpr rewrites references to simd.<simd type> into
// <bridge package>.<size-dependent-type>.
func (c *DeepCopier) OnNameExpr(id *syntax.Name) syntax.Expr {
	obj := c.info.Uses[id]
	if obj == nil {
		obj = c.info.Defs[id]
	}
	if obj == nil {
		return nil
	}

	if isBaseSimdTypeObj(obj) {
		// if it is a name, that means that this is in the simd package,
		// and the name must be replaced with a selector referencing
		// the architecture-dependent packages.
		name := id.Value
		width := nameToElemBitWidth(name)
		if width > 0 {
			archsimdId := syntax.NewName(id.Pos(), archPkg)
			if c.VecLen == 0 {
				// special case for emulation
				newSel := &syntax.SelectorExpr{
					X:   archsimdId,
					Sel: id, // name is unchanged for emulation
				}
				newSel.SetPos(id.Pos())
				return newSel
			}

			count := c.VecLen / width
			base := name[:len(name)-1]
			newName := fmt.Sprintf("%sx%d", base, count)
			newSelId := syntax.NewName(id.Pos(), newName)
			newSel := &syntax.SelectorExpr{
				X:   archsimdId,
				Sel: newSelId,
			}
			newSel.SetPos(id.Pos())
			return newSel
		}
	}

	if c.analyzer.isDependentObj[obj] {
		newId := syntax.NewName(id.Pos(), id.Value+c.suffix)
		// Object link will be handled manually in deepcopier Use/Def mapper
		if base.Debug.Simd > 0 {
			base.Warn("%s: rewriting name %s to %s", id.Pos().String(), id.Value, newId.Value)
		}
		return newId
	}
	return nil
}

// OnSelector is looking for simd.Something, to be rewritten into
// appropriately.  Note that this will not work properly within the simd
// package because there is no "simd." selection there.
func (c *DeepCopier) OnSelector(se *syntax.SelectorExpr) syntax.Expr {
	if x, ok := se.X.(*syntax.Name); ok {
		obj := c.info.Uses[x]
		if pkgName, isPkg := obj.(*types2.PkgName); isPkg && pkgName.Imported().Path() == simdPkg {
			// This first little bit detects name = Load-Type-Width-s-{,Part}
			// and converts the name to Type-Width-s (for nameToWidth), sets isLoad,
			// and initializes the suffix appropriately.
			prefix := ""
			nameSuffix := ""
			name := se.Sel.Value
			end := len(name)
			if strings.HasPrefix(name, "Load") {
				prefix = "Load"
				if strings.HasSuffix(name, "Part") {
					end = strings.Index(name, "Part")
					nameSuffix = "Part"
				}
				name = name[len("Load"):end]
			}
			if strings.HasPrefix(name, "Broadcast") {
				prefix = "Broadcast"
				name = name[len("Broadcast"):end]
			}

			width := nameToElemBitWidth(name)
			if width > 0 {
				archsimdId := syntax.NewName(se.Pos(), archPkg)
				if c.VecLen == 0 {
					// emulated instead, name is unchanged
					newSel := &syntax.SelectorExpr{
						X:   archsimdId,
						Sel: se.Sel,
					}
					newSel.SetPos(se.Pos())
					return newSel
				}

				count := c.VecLen / width
				base := name[:len(name)-1]
				newName := fmt.Sprintf("%sx%d", base, count)
				newName = prefix + newName + nameSuffix

				newSelId := syntax.NewName(se.Sel.Pos(), newName)

				newSel := &syntax.SelectorExpr{
					X:   archsimdId,
					Sel: newSelId,
				}
				newSel.SetPos(se.Pos())
				return newSel
			}
		}
	}
	return nil
}

func (c *DeepCopier) CopyDecl(d syntax.Decl) syntax.Decl {
	if d == nil {
		return nil
	}
	switch d := d.(type) {
	case *syntax.FuncDecl:
		return c.CopyFuncDecl(d)
	case *syntax.VarDecl:
		return c.CopyVarDecl(d)
	case *syntax.TypeDecl:
		return c.CopyTypeDecl(d)
	case *syntax.ConstDecl:
		return c.CopyConstDecl(d)
	case *syntax.ImportDecl:
		newD := &syntax.ImportDecl{
			Group:        d.Group,
			Pragma:       d.Pragma,
			LocalPkgName: c.CopyName(d.LocalPkgName, false),
			Path:         c.CopyExpr(d.Path).(*syntax.BasicLit),
		}
		newD.SetPos(d.Pos())
		return newD
	default:
		return d
	}
}

func (c *DeepCopier) CopyVarDecl(d *syntax.VarDecl) *syntax.VarDecl {
	newD := &syntax.VarDecl{
		Group:  d.Group,
		Pragma: d.Pragma,
		Type:   c.CopyExpr(d.Type),
		Values: c.CopyExpr(d.Values),
	}
	newD.SetPos(d.Pos())
	for _, n := range d.NameList {
		newN := c.CopyName(n, true)
		newD.NameList = append(newD.NameList, newN)
	}
	return newD
}

func (c *DeepCopier) CopyTypeDecl(d *syntax.TypeDecl) *syntax.TypeDecl {
	newD := &syntax.TypeDecl{
		Group:      d.Group,
		Pragma:     d.Pragma,
		Name:       c.CopyName(d.Name, true),
		TParamList: c.CopyFieldList(d.TParamList),
		Alias:      d.Alias,
		Type:       c.CopyExpr(d.Type),
	}
	newD.SetPos(d.Pos())
	return newD
}

func (c *DeepCopier) CopyConstDecl(d *syntax.ConstDecl) *syntax.ConstDecl {
	newD := &syntax.ConstDecl{
		Group:  d.Group,
		Pragma: d.Pragma,
		Type:   c.CopyExpr(d.Type),
		Values: c.CopyExpr(d.Values),
	}
	newD.SetPos(d.Pos())
	for _, n := range d.NameList {
		newD.NameList = append(newD.NameList, c.CopyName(n, true))
	}
	return newD
}

func (c *DeepCopier) CopyFuncDecl(d *syntax.FuncDecl) *syntax.FuncDecl {
	newD := &syntax.FuncDecl{
		Pragma:     d.Pragma,
		Recv:       c.CopyField(d.Recv),
		Name:       c.CopyName(d.Name, true),
		TParamList: c.CopyFieldList(d.TParamList),
		Type:       c.CopyExpr(d.Type).(*syntax.FuncType),
	}
	newD.SetPos(d.Pos())

	// Create and register new types2.Func
	if oldFuncObj, ok := c.info.Defs[d.Name].(*types2.Func); ok {
		newFuncObj := types2.NewFunc(newD.Name.Pos(), c.pkg, newD.Name.Value, oldFuncObj.Type().(*types2.Signature))
		c.info.Defs[newD.Name] = newFuncObj
	}

	newD.Body = c.CopyBlockStmt(d.Body)
	return newD
}

func (c *DeepCopier) CopyName(id *syntax.Name, isDef bool) *syntax.Name {
	if id == nil {
		return nil
	}
	if match := c.OnName(id); match != nil {
		match.SetPos(id.Pos())
		if isDef {
			c.registerDef(match, id)
		} else {
			c.mapUse(match, id)
		}
		return match
	}
	newId := syntax.NewName(id.Pos(), id.Value)
	if isDef {
		c.registerDef(newId, id)
	} else {
		c.mapUse(newId, id)
	}
	return newId
}

func (c *DeepCopier) CopyNameExpr(id *syntax.Name) syntax.Expr {
	if !c.analyzer.inSimd {
		return c.CopyName(id, false)
	}
	if id == nil {
		return nil
	}

	if match := c.OnNameExpr(id); match != nil {
		match.SetPos(id.Pos())
		if n, ok := match.(*syntax.Name); ok {
			c.mapUse(n, id)
		}
		return match
	}

	newId := syntax.NewName(id.Pos(), id.Value)
	c.mapUse(newId, id)
	return newId
}

func (c *DeepCopier) CopyExpr(e syntax.Expr) syntax.Expr {
	if e == nil {
		return nil
	}
	var newE syntax.Expr
	switch e := e.(type) {
	case *syntax.Name:
		return c.CopyNameExpr(e)
	case *syntax.BasicLit:
		newLit := &syntax.BasicLit{Value: e.Value, Kind: e.Kind, Bad: e.Bad}
		newE = newLit
	case *syntax.CompositeLit:
		newLit := &syntax.CompositeLit{
			Type:   c.CopyExpr(e.Type),
			NKeys:  e.NKeys,
			Rbrace: e.Rbrace,
		}
		for _, el := range e.ElemList {
			newLit.ElemList = append(newLit.ElemList, c.CopyExpr(el))
		}
		newE = newLit
	case *syntax.KeyValueExpr:
		newE = &syntax.KeyValueExpr{Key: c.CopyExpr(e.Key), Value: c.CopyExpr(e.Value)}
	case *syntax.FuncLit:
		newE = &syntax.FuncLit{Type: c.CopyExpr(e.Type).(*syntax.FuncType), Body: c.CopyBlockStmt(e.Body)}
	case *syntax.ParenExpr:
		newE = &syntax.ParenExpr{X: c.CopyExpr(e.X)}
	case *syntax.SelectorExpr:
		if sub := c.OnSelector(e); sub != nil {
			sub.SetPos(e.Pos())
			if sel := c.info.Selections[e]; sel != nil {
				c.info.Selections[sub.(*syntax.SelectorExpr)] = sel
			}
			return sub
		}
		newSel := &syntax.SelectorExpr{X: c.CopyExpr(e.X), Sel: c.CopyName(e.Sel, false)}
		if sel := c.info.Selections[e]; sel != nil {
			c.info.Selections[newSel] = sel
		}
		newE = newSel
	case *syntax.IndexExpr:
		newE = &syntax.IndexExpr{X: c.CopyExpr(e.X), Index: c.CopyExpr(e.Index)}
	case *syntax.SliceExpr:
		newE = &syntax.SliceExpr{
			X:     c.CopyExpr(e.X),
			Index: [3]syntax.Expr{c.CopyExpr(e.Index[0]), c.CopyExpr(e.Index[1]), c.CopyExpr(e.Index[2])},
			Full:  e.Full,
		}
	case *syntax.AssertExpr:
		newE = &syntax.AssertExpr{X: c.CopyExpr(e.X), Type: c.CopyExpr(e.Type)}
	case *syntax.TypeSwitchGuard:
		newE = &syntax.TypeSwitchGuard{Lhs: c.CopyName(e.Lhs, true), X: c.CopyExpr(e.X)}
	case *syntax.Operation:
		newE = &syntax.Operation{Op: e.Op, X: c.CopyExpr(e.X), Y: c.CopyExpr(e.Y)}
	case *syntax.CallExpr:
		newCall := &syntax.CallExpr{
			Fun:     c.CopyExpr(e.Fun),
			HasDots: e.HasDots,
		}
		for _, a := range e.ArgList {
			newCall.ArgList = append(newCall.ArgList, c.CopyExpr(a))
		}
		newE = newCall
	case *syntax.ListExpr:
		newList := &syntax.ListExpr{}
		for _, el := range e.ElemList {
			newList.ElemList = append(newList.ElemList, c.CopyExpr(el))
		}
		newE = newList
	case *syntax.ArrayType:
		newE = &syntax.ArrayType{Len: c.CopyExpr(e.Len), Elem: c.CopyExpr(e.Elem)}
	case *syntax.SliceType:
		newE = &syntax.SliceType{Elem: c.CopyExpr(e.Elem)}
	case *syntax.DotsType:
		newE = &syntax.DotsType{Elem: c.CopyExpr(e.Elem)}
	case *syntax.StructType:
		newE = &syntax.StructType{
			FieldList: c.CopyFieldList(e.FieldList),
			TagList:   e.TagList, // Shallow copy for tags is fine usually
		}
	case *syntax.InterfaceType:
		newE = &syntax.InterfaceType{MethodList: c.CopyFieldList(e.MethodList)}
	case *syntax.FuncType:
		newE = &syntax.FuncType{
			ParamList:  c.CopyFieldList(e.ParamList),
			ResultList: c.CopyFieldList(e.ResultList),
		}
	case *syntax.MapType:
		newE = &syntax.MapType{Key: c.CopyExpr(e.Key), Value: c.CopyExpr(e.Value)}
	case *syntax.ChanType:
		newE = &syntax.ChanType{Dir: e.Dir, Elem: c.CopyExpr(e.Elem)}
	case *syntax.BadExpr:
		newE = &syntax.BadExpr{}
	default:
		newE = e
	}
	newE.SetPos(e.Pos())
	return newE
}

func (c *DeepCopier) CopyStmt(s syntax.Stmt) syntax.Stmt {
	if s == nil {
		return nil
	}
	var newS syntax.Stmt
	switch s := s.(type) {
	case *syntax.DeclStmt:
		newDeclList := make([]syntax.Decl, len(s.DeclList))
		for i, v := range s.DeclList {
			newDeclList[i] = c.CopyDecl(v)
		}
		newS = &syntax.DeclStmt{DeclList: newDeclList}
	case *syntax.ExprStmt:
		newS = &syntax.ExprStmt{X: c.CopyExpr(s.X)}
	case *syntax.SendStmt:
		newS = &syntax.SendStmt{Chan: c.CopyExpr(s.Chan), Value: c.CopyExpr(s.Value)}
	case *syntax.AssignStmt:
		newS = &syntax.AssignStmt{Op: s.Op, Lhs: c.CopyExpr(s.Lhs), Rhs: c.CopyExpr(s.Rhs)}
	case *syntax.ReturnStmt:
		newS = &syntax.ReturnStmt{Results: c.CopyExpr(s.Results)}
	case *syntax.BranchStmt:
		// TODO this is broken
		newS = &syntax.BranchStmt{Tok: s.Tok, Label: c.CopyName(s.Label, false), Target: nil} // Targets need fix-up
	case *syntax.CallStmt:
		newS = &syntax.CallStmt{Tok: s.Tok, Call: c.CopyExpr(s.Call), DeferAt: c.CopyExpr(s.DeferAt)}
	case *syntax.IfStmt:
		newS = &syntax.IfStmt{
			Init: c.CopySimpleStmt(s.Init),
			Cond: c.CopyExpr(s.Cond),
			Then: c.CopyBlockStmt(s.Then),
			Else: c.CopyStmt(s.Else),
		}
	case *syntax.ForStmt:
		newS = &syntax.ForStmt{
			Init: c.CopySimpleStmt(s.Init),
			Cond: c.CopyExpr(s.Cond),
			Post: c.CopySimpleStmt(s.Post),
			Body: c.CopyBlockStmt(s.Body),
		}
	case *syntax.SwitchStmt:
		newS = &syntax.SwitchStmt{
			Init:   c.CopySimpleStmt(s.Init),
			Tag:    c.CopyExpr(s.Tag),
			Body:   c.CopyCaseClauses(s.Body),
			Rbrace: s.Rbrace,
		}
	case *syntax.SelectStmt:
		newS = &syntax.SelectStmt{
			Body:   c.CopyCommClauses(s.Body),
			Rbrace: s.Rbrace,
		}
	case *syntax.EmptyStmt:
		newS = &syntax.EmptyStmt{}
	case *syntax.LabeledStmt:
		newS = &syntax.LabeledStmt{Label: c.CopyName(s.Label, true), Stmt: c.CopyStmt(s.Stmt)} // Labels are defs
	case *syntax.BlockStmt:
		return c.CopyBlockStmt(s)
	default:
		newS = s
	}
	newS.SetPos(s.Pos())
	return newS
}

func (c *DeepCopier) CopySimpleStmt(s syntax.SimpleStmt) syntax.SimpleStmt {
	if s == nil {
		return nil
	}
	switch s := s.(type) {
	case *syntax.RangeClause:
		newS := &syntax.RangeClause{
			Def: s.Def,
			X:   c.CopyExpr(s.X),
		}
		// In a range clause, Lhs may contain definitions if Def is true.
		if list, ok := s.Lhs.(*syntax.ListExpr); ok && s.Def {
			newList := &syntax.ListExpr{}
			for _, el := range list.ElemList {
				if id, ok := el.(*syntax.Name); ok {
					newList.ElemList = append(newList.ElemList, c.CopyName(id, true))
				} else {
					newList.ElemList = append(newList.ElemList, c.CopyExpr(el))
				}
			}
			newS.Lhs = newList
		} else if id, ok := s.Lhs.(*syntax.Name); ok && s.Def {
			newS.Lhs = c.CopyName(id, true)
		} else {
			newS.Lhs = c.CopyExpr(s.Lhs)
		}
		newS.Lhs.SetPos(s.Lhs.Pos())
		newS.SetPos(s.Pos())
		return newS
	case *syntax.AssignStmt:
		// Check for :=
		isDef := false
		if list, ok := s.Lhs.(*syntax.ListExpr); ok {
			for _, el := range list.ElemList {
				if id, ok := el.(*syntax.Name); ok && c.info.Defs[id] != nil {
					isDef = true
					break
				}
			}
		} else if id, ok := s.Lhs.(*syntax.Name); ok && c.info.Defs[id] != nil {
			isDef = true
		}

		newS := &syntax.AssignStmt{Op: s.Op, Rhs: c.CopyExpr(s.Rhs)}
		if isDef {
			if list, ok := s.Lhs.(*syntax.ListExpr); ok {
				newList := &syntax.ListExpr{}
				for _, el := range list.ElemList {
					if id, ok := el.(*syntax.Name); ok && c.info.Defs[id] != nil {
						newList.ElemList = append(newList.ElemList, c.CopyName(id, true))
					} else {
						newList.ElemList = append(newList.ElemList, c.CopyExpr(el))
					}
				}
				newS.Lhs = newList
			} else if id, ok := s.Lhs.(*syntax.Name); ok {
				newS.Lhs = c.CopyName(id, true)
			}
		} else {
			newS.Lhs = c.CopyExpr(s.Lhs)
		}
		newS.Lhs.SetPos(s.Lhs.Pos())
		newS.SetPos(s.Pos())
		return newS
	default:
		return c.CopyStmt(s).(syntax.SimpleStmt)
	}
}

func (c *DeepCopier) CopyCaseClauses(list []*syntax.CaseClause) []*syntax.CaseClause {
	var newList []*syntax.CaseClause
	for _, cc := range list {
		newC := &syntax.CaseClause{Cases: c.CopyExpr(cc.Cases), Colon: cc.Colon}
		for _, b := range cc.Body {
			newC.Body = append(newC.Body, c.CopyStmt(b))
		}
		newC.SetPos(cc.Pos())
		newList = append(newList, newC)
	}
	return newList
}

func (c *DeepCopier) CopyCommClauses(list []*syntax.CommClause) []*syntax.CommClause {
	var newList []*syntax.CommClause
	for _, cc := range list {
		newC := &syntax.CommClause{Comm: c.CopySimpleStmt(cc.Comm), Colon: cc.Colon}
		for _, b := range cc.Body {
			newC.Body = append(newC.Body, c.CopyStmt(b))
		}
		newC.SetPos(cc.Pos())
		newList = append(newList, newC)
	}
	return newList
}

func (c *DeepCopier) CopyBlockStmt(b *syntax.BlockStmt) *syntax.BlockStmt {
	if b == nil {
		return nil
	}
	newB := &syntax.BlockStmt{Rbrace: b.Rbrace}
	for _, s := range b.List {
		newB.List = append(newB.List, c.CopyStmt(s))
	}
	newB.SetPos(b.Pos())
	return newB
}

func (c *DeepCopier) CopyFieldList(f []*syntax.Field) []*syntax.Field {
	if f == nil {
		return nil
	}
	var newF []*syntax.Field
	for _, field := range f {
		newF = append(newF, c.CopyField(field))
	}
	return newF
}

func (c *DeepCopier) CopyField(f *syntax.Field) *syntax.Field {
	if f == nil {
		return nil
	}
	newF := &syntax.Field{
		Name: c.CopyName(f.Name, true),
		Type: c.CopyExpr(f.Type),
	}
	newF.SetPos(f.Pos())
	return newF
}

```

// === FILE: references!/go/src/cmd/compile/internal/midway/midway.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package midway

import (
	"internal/buildcfg"
)

func rewriteSizes() []int {
	switch buildcfg.GOARCH {
	case "wasm":
		return []int{0, 128}
	case "amd64":
		return []int{0, 128, 256, 512}
	case "arm64":
		return []int{0, 128} // this will change for SVE and cannot just be a size-based choice.
	}
	return nil
}

const simdPkg = "simd"
const archFullPkg = "simd/internal/bridge"
const archPkg = "bridge"
const vectorSizeFn = "VectorBitSize"
const emulatedFn = "Emulated"

func isSimdTypeName(s string) bool {
	switch s {
	case "Int8s", "Int16s", "Int32s", "Int64s",
		"Uint8s", "Uint16s", "Uint32s", "Uint64s",
		"Mask8s", "Mask16s", "Mask32s", "Mask64s",
		"Float32s", "Float64s":
		return true
	}
	return false
}

```

// === FILE: references!/go/src/cmd/compile/internal/midway/rewrite.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package midway

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
	"fmt"
	"internal/buildcfg"
	"strings"
)

// "Midway" rewriting
//
// Go attempts to provide a package similar to the the "Highway" library
// for C++ (https://google.github.io/highway).  The library package is "simd"
// and defines vector types with unspecified widths that are bound to particular
// machine dependent types as late as program execution.  This is accomplished
// by rewriting code that depends on these types into code that references
// architecture-specific types, perhaps more than once, and if necessary
// dynamically choosing which version to execute based on hardware attributes.
//
// The rewriting takes place early in the compiler, after type checking but
// before conversion to "unified" IR.  To ensure that types are correctly set
// on the modified version of the code, type checking information is reset and
// the type checking phase is re-run.  The places some limits on the shape of
// the rewrites, but it also ensures that the rewritten code is well-formed.
//
// Rewritten code does not reference "archsimd" types directly, but instead
// references types in a "bridge" package that filters the available methods
// and adds a few more.  The package used relies on a builder/compiler hack;
// the compiler's type checker enforces export naming conventions, but the
// build system limits visibility to unrelated "internal" packages and can be
// modified to allow access in special cases (like this one).  This allows the
// rewritten code to reference types, functions, and methods that are not
// accessible otherwise.
//
// The rewrite works in phases.  The first is "analysis", to discover functions,
// types, methods, and variables that depend on "simd" types.  "Depend on" means
// any mention of a simd type, and for types, also includes types that have a
// simd-dependent method.  Dependent functions are split into two categories;
// those whose dependence includes their signature, and those that do not.
// The second category forms the boundary between code that depends on simd and
// code that does not.  Notice that there cannot be a boundary method, because
// (by design) the receiver type is simd-dependent and thus a dependent method
// also has a dependent type in its signature.
//
// The second phase rewrites such "boundary" functions into a "dispatch" version
// and (later, third phase) "specialized" versions.  The dispatch function
// will choose which specialized version to call based on which simd implementation
// has been chosen, and forward parameters and results to/from that specialized version
// of the function.  The dispatch version shares the same name as the original function.
// Note that this applies to functions only, and not methods.

// The third phase specializes dependent functions (both kinds), methods,
// global variables, and types into size/emulation/feature-specific variants.
// Except for methods, this is done by adding a suffix beginning with "@" to
// the name.  Because "@" cannot appear in legal Go identifiers this removes
// the risk of a naming overlap.  Methods are specialized, but not renamed,
// because their receiver type is renamed instead.  Not changing method names
// preserves interface satisfaction, for example in the case of generic interfaces.
//
// Non-boundary dependent function and methods are not rewritten into dispatch
// functions/methods, but remain in the generated code because they must be
// present in the export data so that other packages that import them will still
// compile before rewriting.  Their bodies are replaced with panic(...) to allow
// compilation while preventing even worse chaos in the event of a bug either in
// the compiler or through ambitious use of reflection or assembly language.
//

/* Example rewrites

// Type alias, global variable, and init function:

// before:
type MyInt8s = simd.Int8s
func Generic[T haslen](x int) int {
    var v T
    return x + v.Len()
}
var VL int
func init() {
    VL = Generic[MyInt8s](1)
}
// dispatch:
func init() {
    switch simd.VectorBitSize() {
    case
        128:
            init@simd128()
            return
    case 256:
            init@simd256()
            return
    case 512:
            init@simd512()
            return
    default:
        panic("unsupported vector size")
    }
}
// specialized (128)
type MyInt8s@simd128 = archsimd.Int8x16
func init@simd128() {
        VL = Generic[MyInt8s@simd128](1)
}


// structure containing simd fields, and with simd methods

// before
// A struct dependent on SIMD
type VectorC struct {
    Field simd.Float32s
}
func (v *VectorC) MethodOfSimd() bool {
    return false
}
func (v VectorC) Data() simd.Float32s {
    return v.Field
}
func (v VectorC) Foo(x VectorC) VectorC {
    return VectorC{Field: v.Field.Add(x.Field)}
}

// dispatch
// technically there is none, but functions with panicking bodies
// remain because code must pass type checking before rewriting.
type VectorC struct {
    Field simd.Float32s
}
func (v *VectorC) MethodOfSimd() bool {
    panic(...)
}
func (v VectorC) Data() simd.Float32s {
    panic(...)
}
func (v VectorC) Foo(x VectorC) VectorC {
    panic(...)
}

// specialized (128)

// A struct dependent on SIMD
type VectorC@simd128 struct {
    Field bridge.Float32x4
}
func (v *VectorC@simd128) MethodOfSimd() bool {
    return false
}
func (v VectorC@simd128) Data() bridge.Float32x4 {
    return v.Field
}
func (v VectorC@simd128) Foo(x VectorC@simd128) VectorC@simd128 {
    return VectorC@simd128{Field: v.Field.Add(x.Field)}
}

*/

type Rewriter struct {
	pkg      *types2.Package
	analyzer *Analyzer
	info     *types2.Info
	sizes    []int
}

func NewRewriter(pkg *types2.Package, info *types2.Info, analyzer *Analyzer, sizes []int) *Rewriter {
	return &Rewriter{
		pkg:      pkg,
		info:     info,
		analyzer: analyzer,
		sizes:    sizes,
	}
}

func (r *Rewriter) Rewrite(files []*syntax.File) {

	// First duplicate and specialize all dependent functions and variables.
	for _, fileAST := range files {

		var newDecls []syntax.Decl
		for _, k := range r.sizes {
			newDecls = r.generateForSize(fileAST, k, newDecls)
		}

		// Then replace original functions with dispatchers.
		// This also edits the DeclList of fileAST.
		r.generateDispatchers(fileAST)

		fileAST.DeclList = append(fileAST.DeclList, newDecls...)
	}
}

func (r *Rewriter) generateDispatchers(fileAST *syntax.File) {
	var newDecls []syntax.Decl

	change := false

	for _, decl := range fileAST.DeclList {
		switch d := decl.(type) {
		case *syntax.FuncDecl:
			if d.Name == nil {
				newDecls = append(newDecls, d)
				continue
			}
			obj := r.info.Defs[d.Name]
			if !r.analyzer.isDependentObj[obj] || r.analyzer.inSimd {
				newDecls = append(newDecls, d)
				continue
			}

			sig, ok := obj.Type().(*types2.Signature)
			if !ok {
				newDecls = append(newDecls, d)
				continue
			}

			change = true
			if r.analyzer.HasDependentSignature(sig) {
				if base.Debug.Simd > 0 {
					base.Warn("%s: removing body of dependent-sig original function %v", d.Pos().String(), d.Name.Value)
				}
				d.Body = r.blockOf(d.Pos(), r.panicStmt(d.Pos(),
					"unexpected call of original function rewritten to specialized SIMD"))
				newDecls = append(newDecls, d)
				continue
			}

			// Clean signature -> Replace body with dispatcher
			d.Body = r.createDispatcherBody(d, sig)
			newDecls = append(newDecls, d)

		case *syntax.VarDecl:
			// Keep var decls even if rewritten, so that pre-rewrite code parses correctly.
			// TODO figure out how to deal with side-effects in initializers.
			newDecls = append(newDecls, d)

		case *syntax.TypeDecl:
			// Keep all types; we need the untranslated copy if a method referencing it
			// needs to typecheck pre-translation.
			newDecls = append(newDecls, d)
		default:
			newDecls = append(newDecls, decl)
		}
	}

	if !change {
		return
	}

	fileAST.DeclList = newDecls

	if !r.analyzer.inSimd {
		// Inject an import to the bridge package (if not exists)
		hasArchSimd := false
		var simdImport *syntax.ImportDecl
		p := fileAST.Pos()
		for _, decl := range fileAST.DeclList {
			if imp, ok := decl.(*syntax.ImportDecl); ok {
				if imp.Path.Value == `"`+archFullPkg+`"` {
					hasArchSimd = true
					if simdImport == nil {
						p = imp.Pos()
					}
				}
				if imp.Path.Value == `"`+simdPkg+`"` {
					simdImport = imp
					p = imp.Pos()
				}
			}
		}

		if !hasArchSimd {
			r.injectImport(fileAST, archFullPkg, p)
		}

		// Ensure at least one use of "simd"
		// var _ = simd.VectorBitLen()
		fun := &syntax.SelectorExpr{
			X:   syntax.NewName(p, simdPkg), // Assume this is resolvable
			Sel: syntax.NewName(p, vectorSizeFn),
		}
		fun.SetPos(p)
		call := &syntax.CallExpr{Fun: fun}
		call.SetPos(p)

		name := syntax.NewName(p, "_")

		varDecl := &syntax.VarDecl{NameList: []*syntax.Name{name}, Values: call}
		varDecl.SetPos(p)
		fileAST.DeclList = append(fileAST.DeclList, varDecl)
	}
}

func (r *Rewriter) injectImport(fileAST *syntax.File, toImport string, simdImportPos syntax.Pos) {
	importDecl := &syntax.ImportDecl{
		Path: &syntax.BasicLit{Value: `"` + toImport + `"`, Kind: syntax.StringLit},
	}
	importDecl.Path.SetPos(simdImportPos)
	importDecl.SetPos(simdImportPos)
	fileAST.DeclList = append([]syntax.Decl{importDecl}, fileAST.DeclList...)
}

func (r *Rewriter) createDispatcherBody(d *syntax.FuncDecl, sig *types2.Signature) *syntax.BlockStmt {

	// Build call arguments from the function parameters
	args := func() []syntax.Expr {
		var args []syntax.Expr
		if d.Type.ParamList != nil {
			for _, field := range d.Type.ParamList {
				if field.Name != nil {
					paramName := syntax.NewName(field.Pos(), field.Name.Value)
					args = append(args, paramName)
				}
			}
		}
		return args
	}

	// Slap a pos on an expression
	pe := func(e syntax.Expr) syntax.Expr {
		e.SetPos(d.Pos())
		return e
	}
	// Slap a pos on a statement
	ps := func(e syntax.Stmt) syntax.Stmt {
		e.SetPos(d.Pos())
		return e
	}

	// switch ast node.
	// the goal is something like (for now, till there are finer-grained choices)
	// switch simd.VectorSize() {
	//   case 128: if simd.Emulated() { call the specialize-for-emulation-code(args) }
	//             else { call the specialize-for-128-code(args) }
	//   case 256: call the specialize-for-256-code(args)
	//   etc
	// }
	//
	// the cases above deal with the usual `return call(...)` vs `call(...); return`
	switchStmt := &syntax.SwitchStmt{
		Tag: pe(&syntax.CallExpr{
			Fun: pe(&syntax.SelectorExpr{
				X:   syntax.NewName(d.Pos(), simdPkg), // Assume this is resolvable
				Sel: syntax.NewName(d.Pos(), vectorSizeFn),
			}),
		}),
		Body: []*syntax.CaseClause{},
	}

	var emulation syntax.Stmt

	for _, k := range r.sizes {
		fnName := fmt.Sprintf("%s@simd%d", d.Name.Value, k)
		fnIdent := syntax.NewName(d.Pos(), fnName)

		callExpr := pe(&syntax.CallExpr{
			Fun:     pe(fnIdent),
			ArgList: args(),
		})

		// callReturnStmt is either `return call(...)` or `call(...); return`
		var callReturnStmt syntax.Stmt
		if d.Type.ResultList != nil && len(d.Type.ResultList) > 0 {
			callReturnStmt = &syntax.ReturnStmt{Results: callExpr}
		} else {
			callReturnStmt = &syntax.BlockStmt{
				List: []syntax.Stmt{
					ps(&syntax.ExprStmt{X: callExpr}),
					ps(&syntax.ReturnStmt{}),
				},
				Rbrace: d.Pos(),
			}
		}
		callReturnStmt.SetPos(d.Pos())

		if k == 0 {
			// emulation == `if simd.Emulated() { callReturnStmt }`
			// save it for the first part of the 128 case.
			cond := pe(&syntax.CallExpr{
				Fun: pe(&syntax.SelectorExpr{
					X:   syntax.NewName(d.Pos(), simdPkg), // Assume this is resolvable
					Sel: syntax.NewName(d.Pos(), emulatedFn),
				})})

			blockStmt, ok := callReturnStmt.(*syntax.BlockStmt)
			if !ok {
				blockStmt = &syntax.BlockStmt{
					List:   []syntax.Stmt{callReturnStmt},
					Rbrace: d.Pos(),
				}
				blockStmt.SetPos(d.Pos())
			}

			emulation = ps(&syntax.IfStmt{
				Cond: cond,
				Then: blockStmt,
			})
			continue
		}

		var caseBody []syntax.Stmt
		// assume that 128 is a case; when we do scalable simd, this may change.
		// For now, if there is emulation, it is 128-bit (only).
		if emulation != nil && k == 128 {
			caseBody = append(caseBody, emulation)
			emulation = nil
		}

		caseClause := &syntax.CaseClause{
			Cases: pe(&syntax.BasicLit{Kind: syntax.IntLit, Value: fmt.Sprintf("%d", k)}),
			Body:  append(caseBody, callReturnStmt),
		}
		caseClause.SetPos(d.Pos())
		switchStmt.Body = append(switchStmt.Body, caseClause)
	}

	panicStmt := r.panicStmt(d.Pos(), "unsupported vector size in simd-rewritten code")
	return r.blockOf(d.Pos(), switchStmt, panicStmt)
}

func (r *Rewriter) blockOf(p syntax.Pos, stmts ...syntax.Stmt) *syntax.BlockStmt {
	for _, s := range stmts {
		s.SetPos(p)
	}
	blockStmt := &syntax.BlockStmt{List: stmts}
	blockStmt.SetPos(p)
	return blockStmt
}

func (r *Rewriter) panicStmt(p syntax.Pos, unquotedMessage string) *syntax.ExprStmt {
	pe := func(e syntax.Expr) syntax.Expr {
		e.SetPos(p)
		return e
	}
	fnName := "panic"
	fnIdent := pe(syntax.NewName(p, fnName))
	callExpr := pe(&syntax.CallExpr{
		Fun:     fnIdent,
		ArgList: []syntax.Expr{pe(&syntax.BasicLit{Value: `"` + unquotedMessage + `"`, Kind: syntax.StringLit})},
	})
	panicStmt := &syntax.ExprStmt{X: callExpr}
	panicStmt.SetPos(p)
	return panicStmt
}

func (r *Rewriter) generateForSize(fileAST *syntax.File, k int, newDecls []syntax.Decl) []syntax.Decl {
	copier := NewDeepCopier(r.pkg, r.info, k, r.analyzer, fmt.Sprintf("@simd%d", k))
	for _, decl := range fileAST.DeclList {
		if r.shouldIncludeDecl(decl) {
			newDecl := copier.CopyDecl(decl)
			newDecls = append(newDecls, newDecl)
		}
	}
	return newDecls
}

func nameToElemBitWidth(name string) int {
	var width int
	switch name {
	case "Int8s", "Uint8s", "Mask8s":
		width = 8
	case "Int16s", "Uint16s", "Mask16s":
		width = 16
	case "Int32s", "Uint32s", "Float32s", "Mask32s":
		width = 32
	case "Int64s", "Uint64s", "Float64s", "Mask64s":
		width = 64
	}
	return width
}

func (r *Rewriter) shouldIncludeDecl(decl syntax.Decl) bool {
	// Files (and declarations) in the simd package are excluded
	// from processing, except for those that whose name begins
	// with "tofrom_".
	if r.analyzer.inSimd {
		theFile := decl.Pos().Base().Filename()

		lastSlash := strings.LastIndex(theFile, simdPkg+"/")
		lastBackslash := strings.LastIndex(theFile, simdPkg+"\\")

		// Windows paths can be chaos, all we care, is whether the very last part
		// of the path is any-path-separator + "tofrom_" + anything-else, given that
		// we already know that we are in the simd package.
		maxSlash := max(lastSlash, lastBackslash)
		if maxSlash == -1 {
			return false
		}
		if !strings.HasPrefix(theFile[maxSlash:], simdPkg+"/tofrom_") &&
			!strings.HasPrefix(theFile[maxSlash:], simdPkg+"\\tofrom_") {
			return false
		}
	}

	switch d := decl.(type) {
	case *syntax.FuncDecl:
		if d.Name != nil {
			return r.analyzer.isDependentObj[r.info.Defs[d.Name]]
		}
	case *syntax.TypeDecl:
		return r.analyzer.isDependentObj[r.info.Defs[d.Name]]
	case *syntax.VarDecl:
		for _, name := range d.NameList {
			if r.analyzer.isDependentObj[r.info.Defs[name]] {
				return true
			}
		}
	}
	return false
}

// Generate an API matching the standalone compilation call
func RewriteWrapper(pkg *types2.Package, info *types2.Info, files []*syntax.File) bool {
	if !buildcfg.Experiment.SIMD {
		return false
	}

	switch buildcfg.GOARCH {
	case "wasm", "amd64", "arm64":
	default:
		return false
	}

	sizes := rewriteSizes()
	if len(sizes) == 0 {
		return false
	}
	analyzer := NewAnalyzer(pkg, info)
	if !analyzer.Analyze(files) {
		return false
	}

	CheckPositions(files, "before midway")

	rewriter := NewRewriter(pkg, info, analyzer, sizes)
	rewriter.Rewrite(files)

	CheckPositions(files, "after midway")

	return true
}

```

