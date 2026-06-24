# Domain Architecture: cmd/compile/internal/ir

## Layout Topology
```text
cmd/compile/internal/ir/
├── abi.go
├── bitset.go
├── cfg.go
├── check_reassign_no.go
├── check_reassign_yes.go
├── class_string.go
├── const.go
├── copy.go
├── dump.go
├── expr.go
├── fmt.go
├── func.go
├── html.go
├── ir.go
├── mini.go
├── mknode.go
├── name.go
├── node.go
├── op_string.go
├── package.go
├── reassign_consistency_check.go
├── reassignment.go
├── scc.go
├── stmt.go
├── symtab.go
├── type.go
├── val.go
└── visit.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/compile/internal/ir/abi.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"cmd/compile/internal/base"
	"cmd/internal/obj"
)

// InitLSym defines f's obj.LSym and initializes it based on the
// properties of f. This includes setting the symbol flags and ABI and
// creating and initializing related DWARF symbols.
//
// InitLSym must be called exactly once per function and must be
// called for both functions with bodies and functions without bodies.
// For body-less functions, we only create the LSym; for functions
// with bodies call a helper to setup up / populate the LSym.
func InitLSym(f *Func, hasBody bool) {
	if f.LSym != nil {
		base.FatalfAt(f.Pos(), "InitLSym called twice on %v", f)
	}

	if nam := f.Nname; !IsBlank(nam) {
		f.LSym = nam.LinksymABI(f.ABI)
		if f.Pragma&Systemstack != 0 {
			f.LSym.Set(obj.AttrCFunc, true)
		}
	}
	if hasBody {
		setupTextLSym(f, 0)
	}
}

// setupTextLSym initializes the LSym for a with-body text symbol.
func setupTextLSym(f *Func, flag int) {
	if f.Dupok() {
		flag |= obj.DUPOK
	}
	if f.Wrapper() {
		flag |= obj.WRAPPER
	}
	if f.ABIWrapper() {
		flag |= obj.ABIWRAPPER
	}
	if f.Needctxt() {
		flag |= obj.NEEDCTXT
	}
	if f.Pragma&Nosplit != 0 {
		flag |= obj.NOSPLIT
	}
	if f.IsPackageInit() {
		flag |= obj.PKGINIT
	}

	// Clumsy but important.
	// For functions that could be on the path of invoking a deferred
	// function that can recover (runtime.reflectcall, reflect.callReflect,
	// and reflect.callMethod), we want the panic+recover special handling.
	// See test/recover.go for test cases and src/reflect/value.go
	// for the actual functions being considered.
	//
	// runtime.reflectcall is an assembly function which tailcalls
	// WRAPPER functions (runtime.callNN). Its ABI wrapper needs WRAPPER
	// flag as well.
	fnname := f.Sym().Name
	if base.Ctxt.Pkgpath == "runtime" && fnname == "reflectcall" {
		flag |= obj.WRAPPER
	} else if base.Ctxt.Pkgpath == "reflect" {
		switch fnname {
		case "callReflect", "callMethod":
			flag |= obj.WRAPPER
		}
	}

	base.Ctxt.InitTextSym(f.LSym, flag, f.Pos())
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/bitset.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

type bitset8 uint8

func (f *bitset8) set(mask uint8, b bool) {
	if b {
		*(*uint8)(f) |= mask
	} else {
		*(*uint8)(f) &^= mask
	}
}

func (f bitset8) get2(shift uint8) uint8 {
	return uint8(f>>shift) & 3
}

// set2 sets two bits in f using the bottom two bits of b.
func (f *bitset8) set2(shift uint8, b uint8) {
	// Clear old bits.
	*(*uint8)(f) &^= 3 << shift
	// Set new bits.
	*(*uint8)(f) |= (b & 3) << shift
}

type bitset16 uint16

func (f *bitset16) set(mask uint16, b bool) {
	if b {
		*(*uint16)(f) |= mask
	} else {
		*(*uint16)(f) &^= mask
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/cfg.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

var (
	// MaxStackVarSize is the maximum size variable which we will allocate on the stack.
	// This limit is for explicit variable declarations like "var x T" or "x := ...".
	// Note: the flag smallframes can update this value.
	MaxStackVarSize = int64(128 * 1024)

	// MaxImplicitStackVarSize is the maximum size of implicit variables that we will allocate on the stack.
	//   p := new(T)          allocating T on the stack
	//   p := &T{}            allocating T on the stack
	//   s := make([]T, n)    allocating [n]T on the stack
	//   s := []byte("...")   allocating [n]byte on the stack
	// Note: the flag smallframes can update this value.
	MaxImplicitStackVarSize = int64(64 * 1024)

	// MaxSmallArraySize is the maximum size of an array which is considered small.
	// Small arrays will be initialized directly with a sequence of constant stores.
	// Large arrays will be initialized by copying from a static temp.
	// 256 bytes was chosen to minimize generated code + statictmp size.
	MaxSmallArraySize = int64(256)
)

```

// === FILE: references/go/src/cmd/compile/internal/ir/check_reassign_no.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !checknewoldreassignment

package ir

const consistencyCheckEnabled = false

```

// === FILE: references/go/src/cmd/compile/internal/ir/check_reassign_yes.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build checknewoldreassignment

package ir

const consistencyCheckEnabled = true

```

// === FILE: references/go/src/cmd/compile/internal/ir/class_string.go ===
```go
// Code generated by "stringer -type=Class name.go"; DO NOT EDIT.

package ir

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Pxxx-0]
	_ = x[PEXTERN-1]
	_ = x[PAUTO-2]
	_ = x[PAUTOHEAP-3]
	_ = x[PPARAM-4]
	_ = x[PPARAMOUT-5]
	_ = x[PTYPEPARAM-6]
	_ = x[PFUNC-7]
}

const _Class_name = "PxxxPEXTERNPAUTOPAUTOHEAPPPARAMPPARAMOUTPTYPEPARAMPFUNC"

var _Class_index = [...]uint8{0, 4, 11, 16, 25, 31, 40, 50, 55}

func (i Class) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_Class_index)-1 {
		return "Class(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Class_name[_Class_index[idx]:_Class_index[idx+1]]
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/const.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"go/constant"
	"math"
	"math/big"

	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// NewBool returns an OLITERAL representing b as an untyped boolean.
func NewBool(pos src.XPos, b bool) Node {
	return NewBasicLit(pos, types.UntypedBool, constant.MakeBool(b))
}

// NewInt returns an OLITERAL representing v as an untyped integer.
func NewInt(pos src.XPos, v int64) Node {
	return NewBasicLit(pos, types.UntypedInt, constant.MakeInt64(v))
}

// NewString returns an OLITERAL representing s as an untyped string.
func NewString(pos src.XPos, s string) Node {
	return NewBasicLit(pos, types.UntypedString, constant.MakeString(s))
}

// NewUintptr returns an OLITERAL representing v as a uintptr.
func NewUintptr(pos src.XPos, v int64) Node {
	return NewBasicLit(pos, types.Types[types.TUINTPTR], constant.MakeInt64(v))
}

// NewZero returns a zero value of the given type.
func NewZero(pos src.XPos, typ *types.Type) Node {
	switch {
	case typ.HasNil():
		return NewNilExpr(pos, typ)
	case typ.IsInteger():
		return NewBasicLit(pos, typ, intZero)
	case typ.IsFloat():
		return NewBasicLit(pos, typ, floatZero)
	case typ.IsComplex():
		return NewBasicLit(pos, typ, complexZero)
	case typ.IsBoolean():
		return NewBasicLit(pos, typ, constant.MakeBool(false))
	case typ.IsString():
		return NewBasicLit(pos, typ, constant.MakeString(""))
	case typ.IsArray() || typ.IsStruct():
		// TODO(mdempsky): Return a typechecked expression instead.
		return NewCompLitExpr(pos, OCOMPLIT, typ, nil)
	}

	base.FatalfAt(pos, "unexpected type: %v", typ)
	panic("unreachable")
}

var (
	intZero     = constant.MakeInt64(0)
	floatZero   = constant.ToFloat(intZero)
	complexZero = constant.ToComplex(intZero)
)

// NewOne returns an OLITERAL representing 1 with the given type.
func NewOne(pos src.XPos, typ *types.Type) Node {
	var val constant.Value
	switch {
	case typ.IsInteger():
		val = intOne
	case typ.IsFloat():
		val = floatOne
	case typ.IsComplex():
		val = complexOne
	default:
		base.FatalfAt(pos, "%v cannot represent 1", typ)
	}

	return NewBasicLit(pos, typ, val)
}

var (
	intOne     = constant.MakeInt64(1)
	floatOne   = constant.ToFloat(intOne)
	complexOne = constant.ToComplex(intOne)
)

const (
	// Maximum size in bits for big.Ints before signaling
	// overflow and also mantissa precision for big.Floats.
	ConstPrec = 512
)

func BigFloat(v constant.Value) *big.Float {
	f := new(big.Float)
	f.SetPrec(ConstPrec)
	switch u := constant.Val(v).(type) {
	case int64:
		f.SetInt64(u)
	case *big.Int:
		f.SetInt(u)
	case *big.Float:
		f.Set(u)
	case *big.Rat:
		f.SetRat(u)
	default:
		base.Fatalf("unexpected: %v", u)
	}
	return f
}

// ConstOverflow reports whether constant value v is too large
// to represent with type t.
func ConstOverflow(v constant.Value, t *types.Type) bool {
	switch {
	case t.IsInteger():
		bits := uint(8 * t.Size())
		if t.IsUnsigned() {
			x, ok := constant.Uint64Val(v)
			return !ok || x>>bits != 0
		}
		x, ok := constant.Int64Val(v)
		if x < 0 {
			x = ^x
		}
		return !ok || x>>(bits-1) != 0
	case t.IsFloat():
		switch t.Size() {
		case 4:
			f, _ := constant.Float32Val(v)
			return math.IsInf(float64(f), 0)
		case 8:
			f, _ := constant.Float64Val(v)
			return math.IsInf(f, 0)
		}
	case t.IsComplex():
		ft := types.FloatForComplex(t)
		return ConstOverflow(constant.Real(v), ft) || ConstOverflow(constant.Imag(v), ft)
	}
	base.Fatalf("ConstOverflow: %v, %v", v, t)
	panic("unreachable")
}

// IsConstNode reports whether n is a Go language constant (as opposed to a
// compile-time constant).
//
// Expressions derived from nil, like string([]byte(nil)), while they
// may be known at compile time, are not Go language constants.
func IsConstNode(n Node) bool {
	return n.Op() == OLITERAL
}

func IsSmallIntConst(n Node) bool {
	if n.Op() == OLITERAL {
		v, ok := constant.Int64Val(n.Val())
		return ok && int64(int32(v)) == v
	}
	return false
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/copy.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"cmd/internal/src"
)

// Copy returns a shallow copy of n.
func Copy(n Node) Node {
	return n.copy()
}

// DeepCopy returns a “deep” copy of n, with its entire structure copied
// (except for shared nodes like ONAME, ONONAME, OLITERAL, and OTYPE).
// If pos.IsKnown(), it sets the source position of newly allocated Nodes to pos.
func DeepCopy(pos src.XPos, n Node) Node {
	var edit func(Node) Node
	edit = func(x Node) Node {
		switch x.Op() {
		case ONAME, ONONAME, OLITERAL, ONIL, OTYPE:
			return x
		}
		x = Copy(x)
		if pos.IsKnown() {
			x.SetPos(pos)
		}
		EditChildren(x, edit)
		return x
	}
	return edit(n)
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/dump.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements textual dumping of arbitrary data structures
// for debugging purposes. The code is customized for Node graphs
// and may be used for an alternative view of the node structure.

package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// DumpAny is like FDumpAny but prints to stderr.
func DumpAny(root any, filter string, depth int) {
	FDumpAny(os.Stderr, root, filter, depth)
}

// FDumpAny prints the structure of a rooted data structure
// to w by depth-first traversal of the data structure.
//
// The filter parameter is a regular expression. If it is
// non-empty, only struct fields whose names match filter
// are printed.
//
// The depth parameter controls how deep traversal recurses
// before it returns (higher value means greater depth).
// If an empty field filter is given, a good depth default value
// is 4. A negative depth means no depth limit, which may be fine
// for small data structures or if there is a non-empty filter.
//
// In the output, Node structs are identified by their Op name
// rather than their type; struct fields with zero values or
// non-matching field names are omitted, and "…" means recursion
// depth has been reached or struct fields have been omitted.
func FDumpAny(w io.Writer, root any, filter string, depth int) {
	if root == nil {
		fmt.Fprintln(w, "nil")
		return
	}

	if filter == "" {
		filter = ".*" // default
	}

	p := dumper{
		output:  w,
		fieldrx: regexp.MustCompile(filter),
		ptrmap:  make(map[uintptr]int),
		last:    '\n', // force printing of line number on first line
	}

	p.dump(reflect.ValueOf(root), depth)
	p.printf("\n")
}

// MatchAstDump returns true if the fn matches the value
// of the astdump debug flag.  Fn matches in the following
// cases:
//
//   - astdump == name(fn)
//   - astdump == pkgname(fn).name(fn)
//   - astdump == afterslash(pkgname(fn)).name(fn)
//   - astdump begins with a "~" and what follows "~" is a
//     regular expression matching pkgname(fn).name(fn)
//
// If MatchAstDump returns true, it also prints to os.Stderr
//
//	\nir.Match(<fn>, <astdump>) for <where>\n
func MatchAstDump(fn *Func, where string) bool {
	if len(base.Debug.AstDump) == 0 {
		return false
	}
	return matchForDump(fn, base.Ctxt.Pkgpath, where)
}

// matchForDump is marked noinline to ensure that the exported
// function MatchAstDump IS inlineable and is also small, because
// common case is AstDump is not set.
//
//go:noinline
func matchForDump(fn *Func, pkgPath, where string) bool {
	return MatchPkgFn(pkgPath, FuncName(fn), base.Debug.AstDump)
}

// MatchPkgFn returns true if pkg and fnName "match" toMatch.
// "~REGEXP" matches REGEXP against pkgName + "." + fnName
// "aFunc" matches "aFunc" (in any package)
// "aPkg.aFunc" matches "aPkg.aFunc"
// "aPkg/subPkg.aFunc" matches "subPkg.aFunc"
func MatchPkgFn(pkgName, fnName, toMatch string) bool {
	if toMatch[0] == '~' {
		dbgRE := regexp.MustCompile(toMatch[1:])
		return dbgRE.MatchString(pkgName + "." + fnName)
	}
	if fnName == toMatch {
		return true
	}
	matchPkgDotName := func(pkg string) bool {
		// Allocation-free equality check for toMatch == base.Ctxt.Pkgpath + "." + fnName
		return len(toMatch) == len(pkg)+1+len(fnName) &&
			strings.HasPrefix(toMatch, pkg) && toMatch[len(pkg)] == '.' && strings.HasSuffix(toMatch, fnName)
	}
	if matchPkgDotName(pkgName) {
		return true
	}
	if l := strings.LastIndexByte(pkgName, '/'); l > 0 && matchPkgDotName(pkgName[l+1:]) {
		return true
	}

	return false
}

// AstDump appends the ast dump for fn to the ast dump file for fn.
// The generated file name is
//
//	url.PathEscape(PkgFuncName(fn)) + ".ast"
//
// It also prints
//
//	Writing ast output to <astfilename>\n
//
// to os.Stderr.
func AstDump(fn *Func, why string) {
	err := withLockAndFile(
		fn,
		func(w io.Writer) {
			FDump(w, why, fn)
		},
	)
	// strip text following comma, for phase names.
	comma := strings.Index(why, ",")
	if comma > 0 {
		why = why[:comma]
	}
	DumpNodeHTML(fn, why, fn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dump returned error %v\n", err)
	}
}

var mu sync.Mutex
var astDumpFiles = make(map[string]bool)

func escapedFileName(fn *Func, suffix string) string {
	return EscapedFileName(PkgFuncName(fn), suffix)
}

// EscapedFileName constructs a file name from fn and suffix,
// url-path-escaping the function part of the name and replacing it
// with a hash if it is too long.  The suffix is neither escaped
// nor including in the length calculation, so an excessively
// creative suffix will result in problems.
func EscapedFileName(fn, suffix string) string {
	name := url.PathEscape(fn)
	if len(name) > 125 { // arbitrary limit on file names, as if anyone types these in by hand
		hash := sha256.Sum256([]byte(name))
		name = hex.EncodeToString(hash[:8])
	}
	return name + suffix
}

// withLockAndFile manages ast dump files for various function names
// and invokes a dumping function to write output, under a lock.
func withLockAndFile(fn *Func, dump func(io.Writer)) (err error) {
	name := escapedFileName(fn, ".ast")

	// Ensure that debugging output is not scrambled and is written promptly
	mu.Lock()
	defer mu.Unlock()
	mode := os.O_APPEND | os.O_RDWR
	if !astDumpFiles[name] {
		astDumpFiles[name] = true
		mode = os.O_CREATE | os.O_TRUNC | os.O_RDWR
		fmt.Fprintf(os.Stderr, "Writing text ast output for %s to %s\n", PkgFuncName(fn), name)
	}

	fi, err := os.OpenFile(name, mode, 0777)
	if err != nil {
		return err
	}
	defer func() { err = fi.Close() }()
	dump(fi)
	return
}

var htmlWriters = make(map[*Func]*HTMLWriter)
var orderedFuncs = []*Func{}

// DumpNodeHTML dumps the node n to the HTML writer for fn.
// It uses the same phase name as the text dump.
func DumpNodeHTML(fn *Func, why string, n Node) {
	mu.Lock()
	defer mu.Unlock()
	w, ok := htmlWriters[fn]
	if !ok {
		name := escapedFileName(fn, ".html")
		w = NewHTMLWriter(name, fn, "")
		htmlWriters[fn] = w
		orderedFuncs = append(orderedFuncs, fn)
	}
	w.WritePhase(why, why)
}

// CloseHTMLWriters closes the HTML writer for fn, if one exists.
func CloseHTMLWriters() {
	mu.Lock()
	defer mu.Unlock()
	for _, fn := range orderedFuncs {
		if w, ok := htmlWriters[fn]; ok {
			w.Close("Writing html ast output for %s to %s\n", PkgFuncName(w.Func), w.path)
			delete(htmlWriters, fn)
		}
	}
	orderedFuncs = nil
}

type dumper struct {
	output  io.Writer
	fieldrx *regexp.Regexp  // field name filter
	ptrmap  map[uintptr]int // ptr -> dump line number
	lastadr string          // last address string printed (for shortening)

	// output
	indent int  // current indentation level
	last   byte // last byte processed by Write
	line   int  // current line number
}

var indentBytes = []byte(".  ")

func (p *dumper) Write(data []byte) (n int, err error) {
	var m int
	for i, b := range data {
		// invariant: data[0:n] has been written
		if b == '\n' {
			m, err = p.output.Write(data[n : i+1])
			n += m
			if err != nil {
				return
			}
		} else if p.last == '\n' {
			p.line++
			_, err = fmt.Fprintf(p.output, "%6d  ", p.line)
			if err != nil {
				return
			}
			for j := p.indent; j > 0; j-- {
				_, err = p.output.Write(indentBytes)
				if err != nil {
					return
				}
			}
		}
		p.last = b
	}
	if len(data) > n {
		m, err = p.output.Write(data[n:])
		n += m
	}
	return
}

// printf is a convenience wrapper.
func (p *dumper) printf(format string, args ...any) {
	if _, err := fmt.Fprintf(p, format, args...); err != nil {
		panic(err)
	}
}

// addr returns the (hexadecimal) address string of the object
// represented by x (or "?" if x is not addressable), with the
// common prefix between this and the prior address replaced by
// "0x…" to make it easier to visually match addresses.
func (p *dumper) addr(x reflect.Value) string {
	if !x.CanAddr() {
		return "?"
	}
	adr := fmt.Sprintf("%p", x.Addr().Interface())
	s := adr
	if i := commonPrefixLen(p.lastadr, adr); i > 0 {
		s = "0x…" + adr[i:]
	}
	p.lastadr = adr
	return s
}

// dump prints the contents of x.
func (p *dumper) dump(x reflect.Value, depth int) {
	if depth == 0 {
		p.printf("…")
		return
	}

	if pos, ok := x.Interface().(src.XPos); ok {
		p.printf("%s", base.FmtPos(pos))
		return
	}

	switch x.Kind() {
	case reflect.String:
		p.printf("%q", x.Interface()) // print strings in quotes

	case reflect.Interface:
		if x.IsNil() {
			p.printf("nil")
			return
		}
		p.dump(x.Elem(), depth-1)

	case reflect.Ptr:
		if x.IsNil() {
			p.printf("nil")
			return
		}

		p.printf("*")
		ptr := x.Pointer()
		if line, exists := p.ptrmap[ptr]; exists {
			p.printf("(@%d)", line)
			return
		}
		p.ptrmap[ptr] = p.line
		p.dump(x.Elem(), depth) // don't count pointer indirection towards depth

	case reflect.Slice:
		if x.IsNil() {
			p.printf("nil")
			return
		}
		p.printf("%s (%d entries) {", x.Type(), x.Len())
		if x.Len() > 0 {
			p.indent++
			p.printf("\n")
			for i, n := 0, x.Len(); i < n; i++ {
				p.printf("%d: ", i)
				p.dump(x.Index(i), depth-1)
				p.printf("\n")
			}
			p.indent--
		}
		p.printf("}")

	case reflect.Struct:
		typ := x.Type()

		isNode := false
		if n, ok := x.Interface().(Node); ok {
			isNode = true
			p.printf("%s %s {", n.Op().String(), p.addr(x))
		} else {
			p.printf("%s {", typ)
		}
		p.indent++

		first := true
		omitted := false
		for i, n := 0, typ.NumField(); i < n; i++ {
			// Exclude non-exported fields because their
			// values cannot be accessed via reflection.
			if name := typ.Field(i).Name; types.IsExported(name) {
				if !p.fieldrx.MatchString(name) {
					omitted = true
					continue // field name not selected by filter
				}

				// special cases
				if isNode && name == "Op" {
					omitted = true
					continue // Op field already printed for Nodes
				}
				x := x.Field(i)
				if x.IsZero() {
					omitted = true
					continue // exclude zero-valued fields
				}
				if n, ok := x.Interface().(Nodes); ok && len(n) == 0 {
					omitted = true
					continue // exclude empty Nodes slices
				}

				if first {
					p.printf("\n")
					first = false
				}
				p.printf("%s: ", name)
				p.dump(x, depth-1)
				p.printf("\n")
			}
		}
		if omitted {
			p.printf("…\n")
		}

		p.indent--
		p.printf("}")

	default:
		p.printf("%v", x.Interface())
	}
}

func commonPrefixLen(a, b string) (i int) {
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/expr.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"bytes"
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/src"
	"fmt"
	"go/constant"
	"go/token"
)

// An Expr is a Node that can appear as an expression.
type Expr interface {
	Node
	isExpr()
}

// A miniExpr is a miniNode with extra fields common to expressions.
// TODO(rsc): Once we are sure about the contents, compact the bools
// into a bit field and leave extra bits available for implementations
// embedding miniExpr. Right now there are ~24 unused bits sitting here.
type miniExpr struct {
	miniNode
	flags bitset8
	typ   *types.Type
	init  Nodes // TODO(rsc): Don't require every Node to have an init
}

const (
	miniExprNonNil = 1 << iota
	miniExprTransient
	miniExprBounded
	miniExprImplicit // for use by implementations; not supported by every Expr
	miniExprCheckPtr
)

func (*miniExpr) isExpr() {}

func (n *miniExpr) Type() *types.Type     { return n.typ }
func (n *miniExpr) SetType(x *types.Type) { n.typ = x }
func (n *miniExpr) NonNil() bool          { return n.flags&miniExprNonNil != 0 }
func (n *miniExpr) MarkNonNil()           { n.flags |= miniExprNonNil }
func (n *miniExpr) Transient() bool       { return n.flags&miniExprTransient != 0 }
func (n *miniExpr) SetTransient(b bool)   { n.flags.set(miniExprTransient, b) }
func (n *miniExpr) Bounded() bool         { return n.flags&miniExprBounded != 0 }
func (n *miniExpr) SetBounded(b bool)     { n.flags.set(miniExprBounded, b) }
func (n *miniExpr) Init() Nodes           { return n.init }
func (n *miniExpr) PtrInit() *Nodes       { return &n.init }
func (n *miniExpr) SetInit(x Nodes)       { n.init = x }

// An AddStringExpr is a string concatenation List[0] + List[1] + ... + List[len(List)-1].
type AddStringExpr struct {
	miniExpr
	List     Nodes
	Prealloc *Name
}

func NewAddStringExpr(pos src.XPos, list []Node) *AddStringExpr {
	n := &AddStringExpr{}
	n.pos = pos
	n.op = OADDSTR
	n.List = list
	return n
}

// An AddrExpr is an address-of expression &X.
// It may end up being a normal address-of or an allocation of a composite literal.
type AddrExpr struct {
	miniExpr
	X        Node
	Prealloc *Name // preallocated storage if any
}

func NewAddrExpr(pos src.XPos, x Node) *AddrExpr {
	if x == nil || x.Typecheck() != 1 {
		base.FatalfAt(pos, "missed typecheck: %L", x)
	}
	n := &AddrExpr{X: x}
	n.pos = pos

	switch x.Op() {
	case OARRAYLIT, OMAPLIT, OSLICELIT, OSTRUCTLIT:
		n.op = OPTRLIT

	default:
		n.op = OADDR
		if r, ok := OuterValue(x).(*Name); ok && r.Op() == ONAME {
			r.SetAddrtaken(true)

			// If r is a closure variable, we need to mark its canonical
			// variable as addrtaken too, so that closure conversion
			// captures it by reference.
			//
			// Exception: if we've already marked the variable as
			// capture-by-value, then that means this variable isn't
			// logically modified, and we must be taking its address to pass
			// to a runtime function that won't mutate it. In that case, we
			// only need to make sure our own copy is addressable.
			if r.IsClosureVar() && !r.Byval() {
				r.Canonical().SetAddrtaken(true)
			}
		}
	}

	n.SetType(types.NewPtr(x.Type()))
	n.SetTypecheck(1)

	return n
}

func (n *AddrExpr) Implicit() bool     { return n.flags&miniExprImplicit != 0 }
func (n *AddrExpr) SetImplicit(b bool) { n.flags.set(miniExprImplicit, b) }

func (n *AddrExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OADDR, OPTRLIT:
		n.op = op
	}
}

// A BasicLit is a literal of basic type.
type BasicLit struct {
	miniExpr
	val constant.Value
}

// NewBasicLit returns an OLITERAL representing val with the given type.
func NewBasicLit(pos src.XPos, typ *types.Type, val constant.Value) Node {
	AssertValidTypeForConst(typ, val)

	n := &BasicLit{val: val}
	n.op = OLITERAL
	n.pos = pos
	n.SetType(typ)
	n.SetTypecheck(1)
	return n
}

func (n *BasicLit) Val() constant.Value       { return n.val }
func (n *BasicLit) SetVal(val constant.Value) { n.val = val }

// NewConstExpr returns an OLITERAL representing val, copying the
// position and type from orig.
func NewConstExpr(val constant.Value, orig Node) Node {
	return NewBasicLit(orig.Pos(), orig.Type(), val)
}

// A BinaryExpr is a binary expression X Op Y,
// or Op(X, Y) for builtin functions that do not become calls.
type BinaryExpr struct {
	miniExpr
	X     Node
	Y     Node
	RType Node `mknode:"-"` // see reflectdata/helpers.go
}

func NewBinaryExpr(pos src.XPos, op Op, x, y Node) *BinaryExpr {
	n := &BinaryExpr{X: x, Y: y}
	n.pos = pos
	n.SetOp(op)
	return n
}

func (n *BinaryExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OADD, OADDSTR, OAND, OANDNOT, ODIV, OEQ, OGE, OGT, OLE,
		OLSH, OLT, OMOD, OMUL, ONE, OOR, ORSH, OSUB, OXOR,
		OCOPY, OCOMPLEX, OUNSAFEADD, OUNSAFESLICE, OUNSAFESTRING,
		OMAKEFACE:
		n.op = op
	}
}

// A CallExpr is a function call Fun(Args).
type CallExpr struct {
	miniExpr
	Fun           Node
	Args          Nodes
	DeferAt       Node
	RType         Node    `mknode:"-"` // see reflectdata/helpers.go
	KeepAlive     []*Name // vars to be kept alive until call returns
	IsDDD         bool
	GoDefer       bool // whether this call is part of a go or defer statement
	NoInline      bool // whether this call must not be inlined
	UseBuf        bool // use stack buffer for backing store (OAPPEND only)
	AppendNoAlias bool // backing store proven to be unaliased (OAPPEND only)
	// whether it's a runtime.KeepAlive call the compiler generates to
	// keep a variable alive. See #73137.
	IsCompilerVarLive bool
	Reshape           bool
}

func NewCallExpr(pos src.XPos, op Op, fun Node, args []Node) *CallExpr {
	n := &CallExpr{Fun: fun}
	n.pos = pos
	n.SetOp(op)
	n.Args = args
	return n
}

func (*CallExpr) isStmt() {}

func (n *CallExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OAPPEND,
		OCALL, OCALLFUNC, OCALLINTER, OCALLMETH,
		ODELETE,
		OGETG, OGETCALLERSP,
		OMAKE, OMAX, OMIN, OPRINT, OPRINTLN,
		ORECOVER:
		n.op = op
	}
}

// A ClosureExpr is a function literal expression.
type ClosureExpr struct {
	miniExpr
	Func     *Func `mknode:"-"`
	Prealloc *Name
	IsGoWrap bool // whether this is wrapper closure of a go statement
}

// A CompLitExpr is a composite literal Type{Vals}.
// Before type-checking, the type is Ntype.
type CompLitExpr struct {
	miniExpr
	List     Nodes // initialized values
	RType    Node  `mknode:"-"` // *runtime._type for OMAPLIT map types
	Prealloc *Name
	// For OSLICELIT, Len is the backing array length.
	// For OMAPLIT, Len is the number of entries that we've removed from List and
	// generated explicit mapassign calls for. This is used to inform the map alloc hint.
	Len int64
}

func NewCompLitExpr(pos src.XPos, op Op, typ *types.Type, list []Node) *CompLitExpr {
	n := &CompLitExpr{List: list}
	n.pos = pos
	n.SetOp(op)
	if typ != nil {
		n.SetType(typ)
	}
	return n
}

func (n *CompLitExpr) Implicit() bool     { return n.flags&miniExprImplicit != 0 }
func (n *CompLitExpr) SetImplicit(b bool) { n.flags.set(miniExprImplicit, b) }

func (n *CompLitExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OARRAYLIT, OCOMPLIT, OMAPLIT, OSTRUCTLIT, OSLICELIT:
		n.op = op
	}
}

// A ConvExpr is a conversion Type(X).
// It may end up being a value or a type.
type ConvExpr struct {
	miniExpr
	X Node

	// For implementing OCONVIFACE expressions.
	//
	// TypeWord is an expression yielding a *runtime._type or
	// *runtime.itab value to go in the type word of the iface/eface
	// result. See reflectdata.ConvIfaceTypeWord for further details.
	//
	// SrcRType is an expression yielding a *runtime._type value for X,
	// if it's not pointer-shaped and needs to be heap allocated.
	TypeWord Node `mknode:"-"`
	SrcRType Node `mknode:"-"`

	// For -d=checkptr instrumentation of conversions from
	// unsafe.Pointer to *Elem or *[Len]Elem.
	//
	// TODO(mdempsky): We only ever need one of these, but currently we
	// don't decide which one until walk. Longer term, it probably makes
	// sense to have a dedicated IR op for `(*[Len]Elem)(ptr)[:n:m]`
	// expressions.
	ElemRType     Node `mknode:"-"`
	ElemElemRType Node `mknode:"-"`
}

func NewConvExpr(pos src.XPos, op Op, typ *types.Type, x Node) *ConvExpr {
	n := &ConvExpr{X: x}
	n.pos = pos
	n.typ = typ
	n.SetOp(op)
	return n
}

func (n *ConvExpr) Implicit() bool     { return n.flags&miniExprImplicit != 0 }
func (n *ConvExpr) SetImplicit(b bool) { n.flags.set(miniExprImplicit, b) }
func (n *ConvExpr) CheckPtr() bool     { return n.flags&miniExprCheckPtr != 0 }
func (n *ConvExpr) SetCheckPtr(b bool) { n.flags.set(miniExprCheckPtr, b) }

func (n *ConvExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OCONV, OCONVIFACE, OCONVNOP, OBYTES2STR, OBYTES2STRTMP, ORUNES2STR, OSTR2BYTES, OSTR2BYTESTMP, OSTR2RUNES, ORUNESTR, OSLICE2ARR, OSLICE2ARRPTR:
		n.op = op
	}
}

// An IndexExpr is an index expression X[Index].
type IndexExpr struct {
	miniExpr
	X        Node
	Index    Node
	RType    Node `mknode:"-"` // see reflectdata/helpers.go
	Assigned bool
}

func NewIndexExpr(pos src.XPos, x, index Node) *IndexExpr {
	n := &IndexExpr{X: x, Index: index}
	n.pos = pos
	n.op = OINDEX
	return n
}

func (n *IndexExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OINDEX, OINDEXMAP:
		n.op = op
	}
}

// A KeyExpr is a Key: Value composite literal key.
type KeyExpr struct {
	miniExpr
	Key   Node
	Value Node
}

func NewKeyExpr(pos src.XPos, key, value Node) *KeyExpr {
	n := &KeyExpr{Key: key, Value: value}
	n.pos = pos
	n.op = OKEY
	return n
}

// A StructKeyExpr is a Field: Value composite literal key.
type StructKeyExpr struct {
	miniExpr
	Field *types.Field
	Value Node
}

func NewStructKeyExpr(pos src.XPos, field *types.Field, value Node) *StructKeyExpr {
	n := &StructKeyExpr{Field: field, Value: value}
	n.pos = pos
	n.op = OSTRUCTKEY
	return n
}

func (n *StructKeyExpr) Sym() *types.Sym { return n.Field.Sym }

// An InlinedCallExpr is an inlined function call.
type InlinedCallExpr struct {
	miniExpr
	Body       Nodes
	ReturnVars Nodes // must be side-effect free
	Reshape    bool
}

func NewInlinedCallExpr(pos src.XPos, body, retvars []Node) *InlinedCallExpr {
	n := &InlinedCallExpr{}
	n.pos = pos
	n.op = OINLCALL
	n.Body = body
	n.ReturnVars = retvars
	return n
}

func (n *InlinedCallExpr) SingleResult() Node {
	if have := len(n.ReturnVars); have != 1 {
		base.FatalfAt(n.Pos(), "inlined call has %v results, expected 1", have)
	}

	// If the type of the call is not a shape, but the type of the return value
	// is a shape, we need to do an implicit conversion, so the real type
	// of n is maintained.
	needImplicitConv := !n.Type().HasShape() && n.ReturnVars[0].Type().HasShape()
	if n.Reshape { // or if the inlined call expr needs reshaping.
		needImplicitConv = true
	}

	if needImplicitConv {
		r := NewConvExpr(n.Pos(), OCONVNOP, n.Type(), n.ReturnVars[0])
		r.SetTypecheck(1)
		return r
	}
	return n.ReturnVars[0]
}

// A LogicalExpr is an expression X Op Y where Op is && or ||.
// It is separate from BinaryExpr to make room for statements
// that must be executed before Y but after X.
type LogicalExpr struct {
	miniExpr
	X Node
	Y Node
}

func NewLogicalExpr(pos src.XPos, op Op, x, y Node) *LogicalExpr {
	n := &LogicalExpr{X: x, Y: y}
	n.pos = pos
	n.SetOp(op)
	return n
}

func (n *LogicalExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OANDAND, OOROR:
		n.op = op
	}
}

// A MakeExpr is a make expression: make(Type[, Len[, Cap]]).
// Op is OMAKECHAN, OMAKEMAP, OMAKESLICE, or OMAKESLICECOPY,
// but *not* OMAKE (that's a pre-typechecking CallExpr).
type MakeExpr struct {
	miniExpr
	RType Node `mknode:"-"` // see reflectdata/helpers.go
	Len   Node
	Cap   Node
}

func NewMakeExpr(pos src.XPos, op Op, len, cap Node) *MakeExpr {
	n := &MakeExpr{Len: len, Cap: cap}
	n.pos = pos
	n.SetOp(op)
	return n
}

func (n *MakeExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OMAKECHAN, OMAKEMAP, OMAKESLICE, OMAKESLICECOPY:
		n.op = op
	}
}

// A NilExpr represents the predefined untyped constant nil.
type NilExpr struct {
	miniExpr
}

func NewNilExpr(pos src.XPos, typ *types.Type) *NilExpr {
	if typ == nil {
		base.FatalfAt(pos, "missing type")
	}
	n := &NilExpr{}
	n.pos = pos
	n.op = ONIL
	n.SetType(typ)
	n.SetTypecheck(1)
	return n
}

// A ParenExpr is a parenthesized expression (X).
// It may end up being a value or a type.
type ParenExpr struct {
	miniExpr
	X Node
}

func NewParenExpr(pos src.XPos, x Node) *ParenExpr {
	n := &ParenExpr{X: x}
	n.op = OPAREN
	n.pos = pos
	return n
}

func (n *ParenExpr) Implicit() bool     { return n.flags&miniExprImplicit != 0 }
func (n *ParenExpr) SetImplicit(b bool) { n.flags.set(miniExprImplicit, b) }

// A ResultExpr represents a direct access to a result.
type ResultExpr struct {
	miniExpr
	Index int64 // index of the result expr.
}

func NewResultExpr(pos src.XPos, typ *types.Type, index int64) *ResultExpr {
	n := &ResultExpr{Index: index}
	n.pos = pos
	n.op = ORESULT
	n.typ = typ
	return n
}

// A LinksymOffsetExpr refers to an offset within a global variable.
// It is like a SelectorExpr but without the field name.
type LinksymOffsetExpr struct {
	miniExpr
	Linksym *obj.LSym
	Offset_ int64
}

func NewLinksymOffsetExpr(pos src.XPos, lsym *obj.LSym, offset int64, typ *types.Type) *LinksymOffsetExpr {
	if typ == nil {
		base.FatalfAt(pos, "nil type")
	}
	n := &LinksymOffsetExpr{Linksym: lsym, Offset_: offset}
	n.typ = typ
	n.op = OLINKSYMOFFSET
	n.SetTypecheck(1)
	return n
}

// NewLinksymExpr is NewLinksymOffsetExpr, but with offset fixed at 0.
func NewLinksymExpr(pos src.XPos, lsym *obj.LSym, typ *types.Type) *LinksymOffsetExpr {
	return NewLinksymOffsetExpr(pos, lsym, 0, typ)
}

// NewNameOffsetExpr is NewLinksymOffsetExpr, but taking a *Name
// representing a global variable instead of an *obj.LSym directly.
func NewNameOffsetExpr(pos src.XPos, name *Name, offset int64, typ *types.Type) *LinksymOffsetExpr {
	if name == nil || IsBlank(name) || !(name.Op() == ONAME && name.Class == PEXTERN) {
		base.FatalfAt(pos, "cannot take offset of nil, blank name or non-global variable: %v", name)
	}
	return NewLinksymOffsetExpr(pos, name.Linksym(), offset, typ)
}

// A SelectorExpr is a selector expression X.Sel.
type SelectorExpr struct {
	miniExpr
	X Node
	// Sel is the name of the field or method being selected, without (in the
	// case of methods) any preceding type specifier. If the field/method is
	// exported, than the Sym uses the local package regardless of the package
	// of the containing type.
	Sel *types.Sym
	// The actual selected field - may not be filled in until typechecking.
	Selection *types.Field
	Prealloc  *Name // preallocated storage for OMETHVALUE, if any
}

func NewSelectorExpr(pos src.XPos, op Op, x Node, sel *types.Sym) *SelectorExpr {
	n := &SelectorExpr{X: x, Sel: sel}
	n.pos = pos
	n.SetOp(op)
	return n
}

func (n *SelectorExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OXDOT, ODOT, ODOTPTR, ODOTMETH, ODOTINTER, OMETHVALUE, OMETHEXPR:
		n.op = op
	}
}

func (n *SelectorExpr) Sym() *types.Sym    { return n.Sel }
func (n *SelectorExpr) Implicit() bool     { return n.flags&miniExprImplicit != 0 }
func (n *SelectorExpr) SetImplicit(b bool) { n.flags.set(miniExprImplicit, b) }
func (n *SelectorExpr) Offset() int64      { return n.Selection.Offset }

func (n *SelectorExpr) FuncName() *Name {
	if n.Op() != OMETHEXPR {
		panic(n.no("FuncName"))
	}
	fn := NewNameAt(n.Selection.Pos, MethodSym(n.X.Type(), n.Sel), n.Type())
	fn.Class = PFUNC
	if n.Selection.Nname != nil {
		// TODO(austin): Nname is nil for interface method
		// expressions (I.M), so we can't attach a Func to
		// those here.
		fn.Func = n.Selection.Nname.(*Name).Func
	}
	return fn
}

// A SliceExpr is a slice expression X[Low:High] or X[Low:High:Max].
type SliceExpr struct {
	miniExpr
	X    Node
	Low  Node
	High Node
	Max  Node
}

func NewSliceExpr(pos src.XPos, op Op, x, low, high, max Node) *SliceExpr {
	n := &SliceExpr{X: x, Low: low, High: high, Max: max}
	n.pos = pos
	n.op = op
	return n
}

func (n *SliceExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OSLICE, OSLICEARR, OSLICESTR, OSLICE3, OSLICE3ARR:
		n.op = op
	}
}

// IsSlice3 reports whether o is a slice3 op (OSLICE3, OSLICE3ARR).
// o must be a slicing op.
func (o Op) IsSlice3() bool {
	switch o {
	case OSLICE, OSLICEARR, OSLICESTR:
		return false
	case OSLICE3, OSLICE3ARR:
		return true
	}
	base.Fatalf("IsSlice3 op %v", o)
	return false
}

// A SliceHeaderExpr constructs a slice header from its parts.
type SliceHeaderExpr struct {
	miniExpr
	Ptr Node
	Len Node
	Cap Node
}

func NewSliceHeaderExpr(pos src.XPos, typ *types.Type, ptr, len, cap Node) *SliceHeaderExpr {
	n := &SliceHeaderExpr{Ptr: ptr, Len: len, Cap: cap}
	n.pos = pos
	n.op = OSLICEHEADER
	n.typ = typ
	return n
}

// A StringHeaderExpr expression constructs a string header from its parts.
type StringHeaderExpr struct {
	miniExpr
	Ptr Node
	Len Node
}

func NewStringHeaderExpr(pos src.XPos, ptr, len Node) *StringHeaderExpr {
	n := &StringHeaderExpr{Ptr: ptr, Len: len}
	n.pos = pos
	n.op = OSTRINGHEADER
	n.typ = types.Types[types.TSTRING]
	return n
}

// A StarExpr is a dereference expression *X.
// It may end up being a value or a type.
type StarExpr struct {
	miniExpr
	X Node
}

func NewStarExpr(pos src.XPos, x Node) *StarExpr {
	n := &StarExpr{X: x}
	n.op = ODEREF
	n.pos = pos
	return n
}

func (n *StarExpr) Implicit() bool     { return n.flags&miniExprImplicit != 0 }
func (n *StarExpr) SetImplicit(b bool) { n.flags.set(miniExprImplicit, b) }

// A TypeAssertExpr is a selector expression X.(Type).
// Before type-checking, the type is Ntype.
type TypeAssertExpr struct {
	miniExpr
	X Node

	// Runtime type information provided by walkDotType for
	// assertions from non-empty interface to concrete type.
	ITab Node `mknode:"-"` // *runtime.itab for Type implementing X's type

	// An internal/abi.TypeAssert descriptor to pass to the runtime.
	Descriptor *obj.LSym

	// When set to true, if this assert would panic, then use a nil pointer panic
	// instead of an interface conversion panic.
	// It must not be set for type assertions using the commaok form.
	UseNilPanic bool
}

func NewTypeAssertExpr(pos src.XPos, x Node, typ *types.Type) *TypeAssertExpr {
	n := &TypeAssertExpr{X: x}
	n.pos = pos
	n.op = ODOTTYPE
	if typ != nil {
		n.SetType(typ)
	}
	return n
}

func (n *TypeAssertExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case ODOTTYPE, ODOTTYPE2:
		n.op = op
	}
}

// A DynamicTypeAssertExpr asserts that X is of dynamic type RType.
type DynamicTypeAssertExpr struct {
	miniExpr
	X Node

	// SrcRType is an expression that yields a *runtime._type value
	// representing X's type. It's used in failed assertion panic
	// messages.
	SrcRType Node

	// RType is an expression that yields a *runtime._type value
	// representing the asserted type.
	//
	// BUG(mdempsky): If ITab is non-nil, RType may be nil.
	RType Node

	// ITab is an expression that yields a *runtime.itab value
	// representing the asserted type within the assertee expression's
	// original interface type.
	//
	// ITab is only used for assertions from non-empty interface type to
	// a concrete (i.e., non-interface) type. For all other assertions,
	// ITab is nil.
	ITab Node
}

func NewDynamicTypeAssertExpr(pos src.XPos, op Op, x, rtype Node) *DynamicTypeAssertExpr {
	n := &DynamicTypeAssertExpr{X: x, RType: rtype}
	n.pos = pos
	n.op = op
	return n
}

func (n *DynamicTypeAssertExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case ODYNAMICDOTTYPE, ODYNAMICDOTTYPE2:
		n.op = op
	}
}

// A UnaryExpr is a unary expression Op X,
// or Op(X) for a builtin function that does not end up being a call.
type UnaryExpr struct {
	miniExpr
	X Node
}

func NewUnaryExpr(pos src.XPos, op Op, x Node) *UnaryExpr {
	n := &UnaryExpr{X: x}
	n.pos = pos
	n.SetOp(op)
	return n
}

func (n *UnaryExpr) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OBITNOT, ONEG, ONOT, OPLUS, ORECV,
		OCAP, OCLEAR, OCLOSE, OIMAG, OLEN, ONEW, OPANIC, OREAL,
		OCHECKNIL, OCFUNC, OIDATA, OITAB, OSPTR,
		OUNSAFESTRINGDATA, OUNSAFESLICEDATA:
		n.op = op
	}
}

func IsZero(n Node) bool {
	switch n.Op() {
	case ONIL:
		return true

	case OLITERAL:
		switch u := n.Val(); u.Kind() {
		case constant.String:
			return constant.StringVal(u) == ""
		case constant.Bool:
			return !constant.BoolVal(u)
		default:
			return constant.Sign(u) == 0
		}

	case OARRAYLIT:
		n := n.(*CompLitExpr)
		for _, n1 := range n.List {
			if n1.Op() == OKEY {
				n1 = n1.(*KeyExpr).Value
			}
			if !IsZero(n1) {
				return false
			}
		}
		return true

	case OSTRUCTLIT:
		n := n.(*CompLitExpr)
		for _, n1 := range n.List {
			n1 := n1.(*StructKeyExpr)
			if !IsZero(n1.Value) {
				return false
			}
		}
		return true
	}

	return false
}

// lvalue etc
func IsAddressable(n Node) bool {
	switch n.Op() {
	case OINDEX:
		n := n.(*IndexExpr)
		if n.X.Type() != nil && n.X.Type().IsArray() {
			return IsAddressable(n.X)
		}
		if n.X.Type() != nil && n.X.Type().IsString() {
			return false
		}
		fallthrough
	case ODEREF, ODOTPTR:
		return true

	case ODOT:
		n := n.(*SelectorExpr)
		return IsAddressable(n.X)

	case ONAME:
		n := n.(*Name)
		if n.Class == PFUNC {
			return false
		}
		return true

	case OLINKSYMOFFSET:
		return true
	}

	return false
}

// StaticValue analyzes n to find the earliest expression that always
// evaluates to the same value as n, which might be from an enclosing
// function.
//
// For example, given:
//
//	var x int = g()
//	func() {
//		y := x
//		*p = int(y)
//	}
//
// calling StaticValue on the "int(y)" expression returns the outer
// "g()" expression.
//
// NOTE: StaticValue can return a result with a different type than
// n's type because it can traverse through OCONVNOP operations.
// TODO: consider reapplying OCONVNOP operations to the result. See https://go.dev/cl/676517.
func StaticValue(n Node) Node {
	for {
		switch n1 := n.(type) {
		case *ConvExpr:
			if n1.Op() == OCONVNOP {
				n = n1.X
				continue
			}
		case *InlinedCallExpr:
			if n1.Op() == OINLCALL {
				n = n1.SingleResult()
				continue
			}
		case *ParenExpr:
			n = n1.X
			continue
		}

		n1 := staticValue1(n)
		if n1 == nil {
			return n
		}
		n = n1
	}
}

func staticValue1(nn Node) Node {
	if nn.Op() != ONAME {
		return nil
	}
	n := nn.(*Name).Canonical()
	if n.Class != PAUTO {
		return nil
	}

	defn := n.Defn
	if defn == nil {
		return nil
	}

	var rhs Node
FindRHS:
	switch defn.Op() {
	case OAS:
		defn := defn.(*AssignStmt)
		rhs = defn.Y
	case OAS2:
		defn := defn.(*AssignListStmt)
		for i, lhs := range defn.Lhs {
			if lhs == n {
				rhs = defn.Rhs[i]
				break FindRHS
			}
		}
		base.FatalfAt(defn.Pos(), "%v missing from LHS of %v", n, defn)
	default:
		return nil
	}
	if rhs == nil {
		base.FatalfAt(defn.Pos(), "RHS is nil: %v", defn)
	}

	if Reassigned(n) {
		return nil
	}

	return rhs
}

// Reassigned takes an ONAME node, walks the function in which it is
// defined, and returns a boolean indicating whether the name has any
// assignments other than its declaration.
// NB: global variables are always considered to be re-assigned.
// TODO: handle initial declaration not including an assignment and
// followed by a single assignment?
// NOTE: any changes made here should also be made in the corresponding
// code in the ReassignOracle.Init method.
func Reassigned(name *Name) bool {
	if name.Op() != ONAME {
		base.Fatalf("reassigned %v", name)
	}
	// no way to reliably check for no-reassignment of globals, assume it can be
	if name.Curfn == nil {
		return true
	}

	if name.Addrtaken() {
		return true // conservatively assume it's reassigned indirectly
	}

	// TODO(mdempsky): This is inefficient and becoming increasingly
	// unwieldy. Figure out a way to generalize escape analysis's
	// reassignment detection for use by inlining and devirtualization.

	// isName reports whether n is a reference to name.
	isName := func(x Node) bool {
		if x == nil {
			return false
		}
		n, ok := OuterValue(x).(*Name)
		return ok && n.Canonical() == name
	}

	var do func(n Node) bool
	do = func(n Node) bool {
		switch n.Op() {
		case OAS:
			n := n.(*AssignStmt)
			if isName(n.X) && n != name.Defn {
				return true
			}
		case OAS2, OAS2FUNC, OAS2MAPR, OAS2DOTTYPE, OAS2RECV, OSELRECV2:
			n := n.(*AssignListStmt)
			for _, p := range n.Lhs {
				if isName(p) && n != name.Defn {
					return true
				}
			}
		case OASOP:
			n := n.(*AssignOpStmt)
			if isName(n.X) {
				return true
			}
		case OADDR:
			n := n.(*AddrExpr)
			if isName(n.X) {
				base.FatalfAt(n.Pos(), "%v not marked addrtaken", name)
			}
		case ORANGE:
			n := n.(*RangeStmt)
			if isName(n.Key) || isName(n.Value) {
				return true
			}
		case OCLOSURE:
			n := n.(*ClosureExpr)
			if Any(n.Func, do) {
				return true
			}
		}
		return false
	}
	return Any(name.Curfn, do)
}

// StaticCalleeName returns the ONAME/PFUNC for n, if known.
func StaticCalleeName(n Node) *Name {
	switch n.Op() {
	case OMETHEXPR:
		n := n.(*SelectorExpr)
		return MethodExprName(n)
	case ONAME:
		n := n.(*Name)
		if n.Class == PFUNC {
			return n
		}
	case OCLOSURE:
		return n.(*ClosureExpr).Func.Nname
	}
	return nil
}

// IsIntrinsicCall reports whether the compiler back end will treat the call as an intrinsic operation.
var IsIntrinsicCall = func(*CallExpr) bool { return false }

// IsIntrinsicSym reports whether the compiler back end will treat a call to this symbol as an intrinsic operation.
var IsIntrinsicSym = func(*types.Sym) bool { return false }

// SameSafeExpr checks whether it is safe to reuse one of l and r
// instead of computing both. SameSafeExpr assumes that l and r are
// used in the same statement or expression. In order for it to be
// safe to reuse l or r, they must:
//   - be the same expression
//   - not have side-effects (no function calls, no channel ops);
//     however, panics are ok
//   - not cause inappropriate aliasing; e.g. two string to []byte
//     conversions, must result in two distinct slices
//
// The handling of OINDEXMAP is subtle. OINDEXMAP can occur both
// as an lvalue (map assignment) and an rvalue (map access). This is
// currently OK, since the only place SameSafeExpr gets used on an
// lvalue expression is for OSLICE and OAPPEND optimizations, and it
// is correct in those settings.
func SameSafeExpr(l Node, r Node) bool {
	for l.Op() == OCONVNOP {
		l = l.(*ConvExpr).X
	}
	for r.Op() == OCONVNOP {
		r = r.(*ConvExpr).X
	}
	if l.Op() != r.Op() || !types.Identical(l.Type(), r.Type()) {
		return false
	}

	switch l.Op() {
	case ONAME:
		return l == r

	case ODOT, ODOTPTR:
		l := l.(*SelectorExpr)
		r := r.(*SelectorExpr)
		return l.Sel != nil && r.Sel != nil && l.Sel == r.Sel && SameSafeExpr(l.X, r.X)

	case ODEREF:
		l := l.(*StarExpr)
		r := r.(*StarExpr)
		return SameSafeExpr(l.X, r.X)

	case ONOT, OBITNOT, OPLUS, ONEG:
		l := l.(*UnaryExpr)
		r := r.(*UnaryExpr)
		return SameSafeExpr(l.X, r.X)

	case OCONV:
		l := l.(*ConvExpr)
		r := r.(*ConvExpr)
		// Some conversions can't be reused, such as []byte(str).
		// Allow only numeric-ish types. This is a bit conservative.
		return types.IsSimple[l.Type().Kind()] && SameSafeExpr(l.X, r.X)

	case OINDEX, OINDEXMAP:
		l := l.(*IndexExpr)
		r := r.(*IndexExpr)
		return SameSafeExpr(l.X, r.X) && SameSafeExpr(l.Index, r.Index)

	case OADD, OSUB, OOR, OXOR, OMUL, OLSH, ORSH, OAND, OANDNOT, ODIV, OMOD:
		l := l.(*BinaryExpr)
		r := r.(*BinaryExpr)
		return SameSafeExpr(l.X, r.X) && SameSafeExpr(l.Y, r.Y)

	case OLITERAL:
		return constant.Compare(l.Val(), token.EQL, r.Val())

	case ONIL:
		return true
	}

	return false
}

// ShouldCheckPtr reports whether pointer checking should be enabled for
// function fn at a given level. See debugHelpFooter for defined
// levels.
func ShouldCheckPtr(fn *Func, level int) bool {
	return base.Debug.Checkptr >= level && fn.Pragma&NoCheckPtr == 0
}

// ShouldAsanCheckPtr reports whether pointer checking should be enabled for
// function fn when -asan is enabled.
func ShouldAsanCheckPtr(fn *Func) bool {
	return base.Flag.ASan && fn.Pragma&NoCheckPtr == 0
}

// IsReflectHeaderDataField reports whether l is an expression p.Data
// where p has type reflect.SliceHeader or reflect.StringHeader.
func IsReflectHeaderDataField(l Node) bool {
	if l.Type() != types.Types[types.TUINTPTR] {
		return false
	}

	var tsym *types.Sym
	switch l.Op() {
	case ODOT:
		l := l.(*SelectorExpr)
		tsym = l.X.Type().Sym()
	case ODOTPTR:
		l := l.(*SelectorExpr)
		tsym = l.X.Type().Elem().Sym()
	default:
		return false
	}

	if tsym == nil || l.Sym().Name != "Data" || tsym.Pkg.Path != "reflect" {
		return false
	}
	return tsym.Name == "SliceHeader" || tsym.Name == "StringHeader"
}

func ParamNames(ft *types.Type) []Node {
	args := make([]Node, ft.NumParams())
	for i, f := range ft.Params() {
		args[i] = f.Nname.(*Name)
	}
	return args
}

func RecvParamNames(ft *types.Type) []Node {
	args := make([]Node, ft.NumRecvs()+ft.NumParams())
	for i, f := range ft.RecvParams() {
		args[i] = f.Nname.(*Name)
	}
	return args
}

// MethodSym returns the method symbol representing a method name
// associated with a specific receiver type.
//
// Method symbols can be used to distinguish the same method appearing
// in different method sets. For example, T.M and (*T).M have distinct
// method symbols.
//
// The returned symbol will be marked as a function.
func MethodSym(recv *types.Type, msym *types.Sym) *types.Sym {
	sym := MethodSymSuffix(recv, msym, "")
	sym.SetFunc(true)
	return sym
}

// MethodSymSuffix is like MethodSym, but allows attaching a
// distinguisher suffix. To avoid collisions, the suffix must not
// start with a letter, number, or period.
func MethodSymSuffix(recv *types.Type, msym *types.Sym, suffix string) *types.Sym {
	if msym.IsBlank() {
		base.Fatalf("blank method name")
	}

	rsym := recv.Sym()
	if recv.IsPtr() {
		if rsym != nil {
			base.Fatalf("declared pointer receiver type: %v", recv)
		}
		rsym = recv.Elem().Sym()
	}

	// Find the package the receiver type appeared in. For
	// anonymous receiver types (i.e., anonymous structs with
	// embedded fields), use the "go" pseudo-package instead.
	rpkg := Pkgs.Go
	if rsym != nil {
		rpkg = rsym.Pkg
	}

	var b bytes.Buffer
	if recv.IsPtr() {
		// The parentheses aren't really necessary, but
		// they're pretty traditional at this point.
		fmt.Fprintf(&b, "(%-S)", recv)
	} else {
		fmt.Fprintf(&b, "%-S", recv)
	}

	// A particular receiver type may have multiple non-exported
	// methods with the same name. To disambiguate them, include a
	// package qualifier for names that came from a different
	// package than the receiver type.
	if !types.IsExported(msym.Name) && msym.Pkg != rpkg {
		b.WriteString(".")
		b.WriteString(msym.Pkg.Prefix)
	}

	b.WriteString(".")
	b.WriteString(msym.Name)
	b.WriteString(suffix)
	return rpkg.LookupBytes(b.Bytes())
}

// LookupMethodSelector returns the types.Sym of the selector for a method
// named in local symbol name, as well as the types.Sym of the receiver.
//
// TODO(prattmic): this does not attempt to handle method suffixes (wrappers).
func LookupMethodSelector(pkg *types.Pkg, name string) (typ, meth *types.Sym, err error) {
	typeName, methName := splitType(name)
	if typeName == "" {
		return nil, nil, fmt.Errorf("%s doesn't contain type split", name)
	}

	if len(typeName) > 3 && typeName[:2] == "(*" && typeName[len(typeName)-1] == ')' {
		// Symbol name is for a pointer receiver method. We just want
		// the base type name.
		typeName = typeName[2 : len(typeName)-1]
	}

	typ = pkg.Lookup(typeName)
	meth = pkg.Selector(methName)
	return typ, meth, nil
}

// splitType splits a local symbol name into type and method (fn). If this a
// free function, typ == "".
//
// N.B. closures and methods can be ambiguous (e.g., bar.func1). These cases
// are returned as methods.
func splitType(name string) (typ, fn string) {
	// Types are split on the first dot, ignoring everything inside
	// brackets (instantiation of type parameter, usually including
	// "go.shape").
	bracket := 0
	for i, r := range name {
		if r == '.' && bracket == 0 {
			return name[:i], name[i+1:]
		}
		if r == '[' {
			bracket++
		}
		if r == ']' {
			bracket--
		}
	}
	return "", name
}

// MethodExprName returns the ONAME representing the method
// referenced by expression n, which must be a method selector,
// method expression, or method value.
func MethodExprName(n Node) *Name {
	name, _ := MethodExprFunc(n).Nname.(*Name)
	return name
}

// MethodExprFunc is like MethodExprName, but returns the types.Field instead.
func MethodExprFunc(n Node) *types.Field {
	switch n.Op() {
	case ODOTMETH, OMETHEXPR, OMETHVALUE:
		return n.(*SelectorExpr).Selection
	}
	base.Fatalf("unexpected node: %v (%v)", n, n.Op())
	panic("unreachable")
}

// A MoveToHeapExpr takes a slice as input and moves it to the
// heap (by copying the backing store if it is not already
// on the heap).
type MoveToHeapExpr struct {
	miniExpr
	Slice Node
	// An expression that evaluates to a *runtime._type
	// that represents the slice element type.
	RType Node
	// If PreserveCapacity is true, the capacity of
	// the resulting slice, and all of the elements in
	// [len:cap], must be preserved.
	// If PreserveCapacity is false, the resulting
	// slice may have any capacity >= len, with any
	// elements in the resulting [len:cap] range zeroed.
	PreserveCapacity bool
}

func NewMoveToHeapExpr(pos src.XPos, slice Node) *MoveToHeapExpr {
	n := &MoveToHeapExpr{Slice: slice}
	n.pos = pos
	n.op = OMOVE2HEAP
	return n
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/fmt.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"bytes"
	"fmt"
	"go/constant"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"unicode/utf8"

	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// Op

var OpNames = []string{
	OADDR:             "&",
	OADD:              "+",
	OADDSTR:           "+",
	OANDAND:           "&&",
	OANDNOT:           "&^",
	OAND:              "&",
	OAPPEND:           "append",
	OAS:               "=",
	OAS2:              "=",
	OBREAK:            "break",
	OCALL:             "function call", // not actual syntax
	OCAP:              "cap",
	OCASE:             "case",
	OCLEAR:            "clear",
	OCLOSE:            "close",
	OCOMPLEX:          "complex",
	OBITNOT:           "^",
	OCONTINUE:         "continue",
	OCOPY:             "copy",
	ODELETE:           "delete",
	ODEFER:            "defer",
	ODIV:              "/",
	OEQ:               "==",
	OFALL:             "fallthrough",
	OFOR:              "for",
	OGE:               ">=",
	OGOTO:             "goto",
	OGT:               ">",
	OIF:               "if",
	OIMAG:             "imag",
	OINLMARK:          "inlmark",
	ODEREF:            "*",
	OLEN:              "len",
	OLE:               "<=",
	OLSH:              "<<",
	OLT:               "<",
	OMAKE:             "make",
	ONEG:              "-",
	OMAX:              "max",
	OMIN:              "min",
	OMOD:              "%",
	OMUL:              "*",
	ONEW:              "new",
	ONE:               "!=",
	ONOT:              "!",
	OOROR:             "||",
	OOR:               "|",
	OPANIC:            "panic",
	OPLUS:             "+",
	OPRINTLN:          "println",
	OPRINT:            "print",
	ORANGE:            "range",
	OREAL:             "real",
	ORECV:             "<-",
	ORECOVER:          "recover",
	ORETURN:           "return",
	ORSH:              ">>",
	OSELECT:           "select",
	OSEND:             "<-",
	OSUB:              "-",
	OSWITCH:           "switch",
	OUNSAFEADD:        "unsafe.Add",
	OUNSAFESLICE:      "unsafe.Slice",
	OUNSAFESLICEDATA:  "unsafe.SliceData",
	OUNSAFESTRING:     "unsafe.String",
	OUNSAFESTRINGDATA: "unsafe.StringData",
	OXOR:              "^",
}

// GoString returns the Go syntax for the Op, or else its name.
func (o Op) GoString() string {
	if int(o) < len(OpNames) && OpNames[o] != "" {
		return OpNames[o]
	}
	return o.String()
}

// Format implements formatting for an Op.
// The valid formats are:
//
//	%v	Go syntax ("+", "<-", "print")
//	%+v	Debug syntax ("ADD", "RECV", "PRINT")
func (o Op) Format(s fmt.State, verb rune) {
	switch verb {
	default:
		fmt.Fprintf(s, "%%!%c(Op=%d)", verb, int(o))
	case 'v':
		if s.Flag('+') {
			// %+v is OMUL instead of "*"
			io.WriteString(s, o.String())
			return
		}
		io.WriteString(s, o.GoString())
	}
}

// Node

// fmtNode implements formatting for a Node n.
// Every Node implementation must define a Format method that calls fmtNode.
// The valid formats are:
//
//	%v	Go syntax
//	%L	Go syntax followed by " (type T)" if type is known.
//	%+v	Debug syntax, as in Dump.
func fmtNode(n Node, s fmt.State, verb rune) {
	// %+v prints Dump.
	// Otherwise we print Go syntax.
	if s.Flag('+') && verb == 'v' {
		dumpNode(s, n, 1)
		return
	}

	if verb != 'v' && verb != 'S' && verb != 'L' {
		fmt.Fprintf(s, "%%!%c(*Node=%p)", verb, n)
		return
	}

	if n == nil {
		fmt.Fprint(s, "<nil>")
		return
	}

	t := n.Type()
	if verb == 'L' && t != nil {
		if t.Kind() == types.TNIL {
			fmt.Fprint(s, "nil")
		} else if n.Op() == ONAME && n.Name().AutoTemp() {
			fmt.Fprintf(s, "%v value", t)
		} else {
			fmt.Fprintf(s, "%v (type %v)", n, t)
		}
		return
	}

	// TODO inlining produces expressions with ninits. we can't print these yet.

	if OpPrec[n.Op()] < 0 {
		stmtFmt(n, s)
		return
	}

	exprFmt(n, s, 0)
}

var OpPrec = []int{
	OAPPEND:           8,
	OBYTES2STR:        8,
	OARRAYLIT:         8,
	OSLICELIT:         8,
	ORUNES2STR:        8,
	OCALLFUNC:         8,
	OCALLINTER:        8,
	OCALLMETH:         8,
	OCALL:             8,
	OCAP:              8,
	OCLEAR:            8,
	OCLOSE:            8,
	OCOMPLIT:          8,
	OCONVIFACE:        8,
	OCONVNOP:          8,
	OCONV:             8,
	OCOPY:             8,
	ODELETE:           8,
	OGETG:             8,
	OLEN:              8,
	OLITERAL:          8,
	OMAKESLICE:        8,
	OMAKESLICECOPY:    8,
	OMAKE:             8,
	OMAPLIT:           8,
	OMAX:              8,
	OMIN:              8,
	ONAME:             8,
	ONEW:              8,
	ONIL:              8,
	ONONAME:           8,
	OPANIC:            8,
	OPAREN:            8,
	OPRINTLN:          8,
	OPRINT:            8,
	ORUNESTR:          8,
	OSLICE2ARR:        8,
	OSLICE2ARRPTR:     8,
	OSTR2BYTES:        8,
	OSTR2RUNES:        8,
	OSTRUCTLIT:        8,
	OTYPE:             8,
	OUNSAFEADD:        8,
	OUNSAFESLICE:      8,
	OUNSAFESLICEDATA:  8,
	OUNSAFESTRING:     8,
	OUNSAFESTRINGDATA: 8,
	OINDEXMAP:         8,
	OINDEX:            8,
	OSLICE:            8,
	OSLICESTR:         8,
	OSLICEARR:         8,
	OSLICE3:           8,
	OSLICE3ARR:        8,
	OSLICEHEADER:      8,
	OSTRINGHEADER:     8,
	ODOTINTER:         8,
	ODOTMETH:          8,
	ODOTPTR:           8,
	ODOTTYPE2:         8,
	ODOTTYPE:          8,
	ODOT:              8,
	OXDOT:             8,
	OMETHVALUE:        8,
	OMETHEXPR:         8,
	OPLUS:             7,
	ONOT:              7,
	OBITNOT:           7,
	ONEG:              7,
	OADDR:             7,
	ODEREF:            7,
	ORECV:             7,
	OMUL:              6,
	ODIV:              6,
	OMOD:              6,
	OLSH:              6,
	ORSH:              6,
	OAND:              6,
	OANDNOT:           6,
	OADD:              5,
	OSUB:              5,
	OOR:               5,
	OXOR:              5,
	OEQ:               4,
	OLT:               4,
	OLE:               4,
	OGE:               4,
	OGT:               4,
	ONE:               4,
	OSEND:             3,
	OANDAND:           2,
	OOROR:             1,

	// Statements handled by stmtfmt
	OAS:         -1,
	OAS2:        -1,
	OAS2DOTTYPE: -1,
	OAS2FUNC:    -1,
	OAS2MAPR:    -1,
	OAS2RECV:    -1,
	OASOP:       -1,
	OBLOCK:      -1,
	OBREAK:      -1,
	OCASE:       -1,
	OCONTINUE:   -1,
	ODCL:        -1,
	ODEFER:      -1,
	OFALL:       -1,
	OFOR:        -1,
	OGOTO:       -1,
	OIF:         -1,
	OLABEL:      -1,
	OGO:         -1,
	ORANGE:      -1,
	ORETURN:     -1,
	OSELECT:     -1,
	OSWITCH:     -1,

	OEND: 0,
}

// StmtWithInit reports whether op is a statement with an explicit init list.
func StmtWithInit(op Op) bool {
	switch op {
	case OIF, OFOR, OSWITCH:
		return true
	}
	return false
}

func stmtFmt(n Node, s fmt.State) {
	// NOTE(rsc): This code used to support the text-based
	// which was more aggressive about printing full Go syntax
	// (for example, an actual loop instead of "for loop").
	// The code is preserved for now in case we want to expand
	// any of those shortenings later. Or maybe we will delete
	// the code. But for now, keep it.
	const exportFormat = false

	// some statements allow for an init, but at most one,
	// but we may have an arbitrary number added, eg by typecheck
	// and inlining. If it doesn't fit the syntax, emit an enclosing
	// block starting with the init statements.

	// if we can just say "for" n->ninit; ... then do so
	simpleinit := len(n.Init()) == 1 && len(n.Init()[0].Init()) == 0 && StmtWithInit(n.Op())

	// otherwise, print the inits as separate statements
	complexinit := len(n.Init()) != 0 && !simpleinit && exportFormat

	// but if it was for if/for/switch, put in an extra surrounding block to limit the scope
	extrablock := complexinit && StmtWithInit(n.Op())

	if extrablock {
		fmt.Fprint(s, "{")
	}

	if complexinit {
		fmt.Fprintf(s, " %v; ", n.Init())
	}

	switch n.Op() {
	case ODCL:
		n := n.(*Decl)
		fmt.Fprintf(s, "var %v %v", n.X.Sym(), n.X.Type())

	// Don't export "v = <N>" initializing statements, hope they're always
	// preceded by the DCL which will be re-parsed and typechecked to reproduce
	// the "v = <N>" again.
	case OAS:
		n := n.(*AssignStmt)
		if n.Def && !complexinit {
			fmt.Fprintf(s, "%v := %v", n.X, n.Y)
		} else {
			fmt.Fprintf(s, "%v = %v", n.X, n.Y)
		}

	case OASOP:
		n := n.(*AssignOpStmt)
		if n.IncDec {
			if n.AsOp == OADD {
				fmt.Fprintf(s, "%v++", n.X)
			} else {
				fmt.Fprintf(s, "%v--", n.X)
			}
			break
		}

		fmt.Fprintf(s, "%v %v= %v", n.X, n.AsOp, n.Y)

	case OAS2, OAS2DOTTYPE, OAS2FUNC, OAS2MAPR, OAS2RECV:
		n := n.(*AssignListStmt)
		if n.Def && !complexinit {
			fmt.Fprintf(s, "%.v := %.v", n.Lhs, n.Rhs)
		} else {
			fmt.Fprintf(s, "%.v = %.v", n.Lhs, n.Rhs)
		}

	case OBLOCK:
		n := n.(*BlockStmt)
		if len(n.List) != 0 {
			fmt.Fprintf(s, "%v", n.List)
		}

	case ORETURN:
		n := n.(*ReturnStmt)
		fmt.Fprintf(s, "return %.v", n.Results)

	case OTAILCALL:
		n := n.(*TailCallStmt)
		fmt.Fprintf(s, "tailcall %v", n.Call)

	case OINLMARK:
		n := n.(*InlineMarkStmt)
		fmt.Fprintf(s, "inlmark %d", n.Index)

	case OGO:
		n := n.(*GoDeferStmt)
		fmt.Fprintf(s, "go %v", n.Call)

	case ODEFER:
		n := n.(*GoDeferStmt)
		fmt.Fprintf(s, "defer %v", n.Call)

	case OIF:
		n := n.(*IfStmt)
		if simpleinit {
			fmt.Fprintf(s, "if %v; %v { %v }", n.Init()[0], n.Cond, n.Body)
		} else {
			fmt.Fprintf(s, "if %v { %v }", n.Cond, n.Body)
		}
		if len(n.Else) != 0 {
			fmt.Fprintf(s, " else { %v }", n.Else)
		}

	case OFOR:
		n := n.(*ForStmt)
		if !exportFormat { // TODO maybe only if FmtShort, same below
			fmt.Fprintf(s, "for loop")
			break
		}

		fmt.Fprint(s, "for")
		if n.DistinctVars {
			fmt.Fprint(s, " /* distinct */")
		}
		if simpleinit {
			fmt.Fprintf(s, " %v;", n.Init()[0])
		} else if n.Post != nil {
			fmt.Fprint(s, " ;")
		}

		if n.Cond != nil {
			fmt.Fprintf(s, " %v", n.Cond)
		}

		if n.Post != nil {
			fmt.Fprintf(s, "; %v", n.Post)
		} else if simpleinit {
			fmt.Fprint(s, ";")
		}

		fmt.Fprintf(s, " { %v }", n.Body)

	case ORANGE:
		n := n.(*RangeStmt)
		if !exportFormat {
			fmt.Fprint(s, "for loop")
			break
		}

		fmt.Fprint(s, "for")
		if n.Key != nil {
			fmt.Fprintf(s, " %v", n.Key)
			if n.Value != nil {
				fmt.Fprintf(s, ", %v", n.Value)
			}
			fmt.Fprint(s, " =")
		}
		fmt.Fprintf(s, " range %v { %v }", n.X, n.Body)
		if n.DistinctVars {
			fmt.Fprint(s, " /* distinct vars */")
		}

	case OSELECT:
		n := n.(*SelectStmt)
		if !exportFormat {
			fmt.Fprintf(s, "%v statement", n.Op())
			break
		}
		fmt.Fprintf(s, "select { %v }", n.Cases)

	case OSWITCH:
		n := n.(*SwitchStmt)
		if !exportFormat {
			fmt.Fprintf(s, "%v statement", n.Op())
			break
		}
		fmt.Fprintf(s, "switch")
		if simpleinit {
			fmt.Fprintf(s, " %v;", n.Init()[0])
		}
		if n.Tag != nil {
			fmt.Fprintf(s, " %v ", n.Tag)
		}
		fmt.Fprintf(s, " { %v }", n.Cases)

	case OCASE:
		n := n.(*CaseClause)
		if len(n.List) != 0 {
			fmt.Fprintf(s, "case %.v", n.List)
		} else {
			fmt.Fprint(s, "default")
		}
		fmt.Fprintf(s, ": %v", n.Body)

	case OBREAK, OCONTINUE, OGOTO, OFALL:
		n := n.(*BranchStmt)
		if n.Label != nil {
			fmt.Fprintf(s, "%v %v", n.Op(), n.Label)
		} else {
			fmt.Fprintf(s, "%v", n.Op())
		}

	case OLABEL:
		n := n.(*LabelStmt)
		fmt.Fprintf(s, "%v: ", n.Label)
	}

	if extrablock {
		fmt.Fprint(s, "}")
	}
}

func exprFmt(n Node, s fmt.State, prec int) {
	// NOTE(rsc): This code used to support the text-based
	// which was more aggressive about printing full Go syntax
	// (for example, an actual loop instead of "for loop").
	// The code is preserved for now in case we want to expand
	// any of those shortenings later. Or maybe we will delete
	// the code. But for now, keep it.
	const exportFormat = false

	for {
		if n == nil {
			fmt.Fprint(s, "<nil>")
			return
		}

		// Skip implicit operations introduced during typechecking.
		switch nn := n; nn.Op() {
		case OADDR:
			nn := nn.(*AddrExpr)
			if nn.Implicit() {
				n = nn.X
				continue
			}
		case ODEREF:
			nn := nn.(*StarExpr)
			if nn.Implicit() {
				n = nn.X
				continue
			}
		case OCONV, OCONVNOP, OCONVIFACE:
			nn := nn.(*ConvExpr)
			if nn.Implicit() {
				n = nn.X
				continue
			}
		}

		break
	}

	nprec := OpPrec[n.Op()]
	if n.Op() == OTYPE && n.Type() != nil && n.Type().IsPtr() {
		nprec = OpPrec[ODEREF]
	}

	if prec > nprec {
		fmt.Fprintf(s, "(%v)", n)
		return
	}

	switch n.Op() {
	case OPAREN:
		n := n.(*ParenExpr)
		fmt.Fprintf(s, "(%v)", n.X)

	case ONIL:
		fmt.Fprint(s, "nil")

	case OLITERAL:
		if n.Sym() != nil {
			fmt.Fprint(s, n.Sym())
			return
		}

		typ := n.Type()
		val := n.Val()

		// Special case for rune constants.
		if typ == types.RuneType || typ == types.UntypedRune {
			if x, ok := constant.Uint64Val(val); ok && x <= utf8.MaxRune {
				fmt.Fprintf(s, "%q", rune(x))
				return
			}
		}

		// Only include typ if it's neither the default nor untyped type
		// for the constant value.
		if k := val.Kind(); typ == types.Types[types.DefaultKinds[k]] || typ == types.UntypedTypes[k] {
			fmt.Fprint(s, val)
		} else {
			fmt.Fprintf(s, "%v(%v)", typ, val)
		}

	case ODCLFUNC:
		n := n.(*Func)
		if sym := n.Sym(); sym != nil {
			fmt.Fprint(s, sym)
			return
		}
		fmt.Fprintf(s, "<unnamed Func>")

	case ONAME:
		n := n.(*Name)
		// Special case: name used as local variable in export.
		// _ becomes ~b%d internally; print as _ for export
		if !exportFormat && n.Sym() != nil && n.Sym().Name[0] == '~' && n.Sym().Name[1] == 'b' {
			fmt.Fprint(s, "_")
			return
		}
		fallthrough
	case ONONAME:
		fmt.Fprint(s, n.Sym())

	case OLINKSYMOFFSET:
		n := n.(*LinksymOffsetExpr)
		fmt.Fprintf(s, "(%v)(%s@%d)", n.Type(), n.Linksym.Name, n.Offset_)

	case OTYPE:
		if n.Type() == nil && n.Sym() != nil {
			fmt.Fprint(s, n.Sym())
			return
		}
		fmt.Fprintf(s, "%v", n.Type())

	case OCLOSURE:
		n := n.(*ClosureExpr)
		if !exportFormat {
			fmt.Fprint(s, "func literal")
			return
		}
		fmt.Fprintf(s, "%v { %v }", n.Type(), n.Func.Body)

	case OPTRLIT:
		n := n.(*AddrExpr)
		fmt.Fprintf(s, "&%v", n.X)

	case OCOMPLIT, OSTRUCTLIT, OARRAYLIT, OSLICELIT, OMAPLIT:
		n := n.(*CompLitExpr)
		if n.Implicit() {
			fmt.Fprintf(s, "... argument")
			return
		}
		fmt.Fprintf(s, "%v{%s}", n.Type(), ellipsisIf(len(n.List) != 0))

	case OKEY:
		n := n.(*KeyExpr)
		if n.Key != nil && n.Value != nil {
			fmt.Fprintf(s, "%v:%v", n.Key, n.Value)
			return
		}

		if n.Key == nil && n.Value != nil {
			fmt.Fprintf(s, ":%v", n.Value)
			return
		}
		if n.Key != nil && n.Value == nil {
			fmt.Fprintf(s, "%v:", n.Key)
			return
		}
		fmt.Fprint(s, ":")

	case OSTRUCTKEY:
		n := n.(*StructKeyExpr)
		fmt.Fprintf(s, "%v:%v", n.Field, n.Value)

	case OXDOT, ODOT, ODOTPTR, ODOTINTER, ODOTMETH, OMETHVALUE, OMETHEXPR:
		n := n.(*SelectorExpr)
		exprFmt(n.X, s, nprec)
		if n.Sel == nil {
			fmt.Fprint(s, ".<nil>")
			return
		}
		fmt.Fprintf(s, ".%s", n.Sel.Name)

	case ODOTTYPE, ODOTTYPE2:
		n := n.(*TypeAssertExpr)
		exprFmt(n.X, s, nprec)
		fmt.Fprintf(s, ".(%v)", n.Type())

	case OINDEX, OINDEXMAP:
		n := n.(*IndexExpr)
		exprFmt(n.X, s, nprec)
		fmt.Fprintf(s, "[%v]", n.Index)

	case OSLICE, OSLICESTR, OSLICEARR, OSLICE3, OSLICE3ARR:
		n := n.(*SliceExpr)
		exprFmt(n.X, s, nprec)
		fmt.Fprint(s, "[")
		if n.Low != nil {
			fmt.Fprint(s, n.Low)
		}
		fmt.Fprint(s, ":")
		if n.High != nil {
			fmt.Fprint(s, n.High)
		}
		if n.Op().IsSlice3() {
			fmt.Fprint(s, ":")
			if n.Max != nil {
				fmt.Fprint(s, n.Max)
			}
		}
		fmt.Fprint(s, "]")

	case OSLICEHEADER:
		n := n.(*SliceHeaderExpr)
		fmt.Fprintf(s, "sliceheader{%v,%v,%v}", n.Ptr, n.Len, n.Cap)

	case OCOMPLEX, OCOPY, OUNSAFEADD, OUNSAFESLICE:
		n := n.(*BinaryExpr)
		fmt.Fprintf(s, "%v(%v, %v)", n.Op(), n.X, n.Y)

	case OCONV,
		OCONVIFACE,
		OCONVNOP,
		OBYTES2STR,
		ORUNES2STR,
		OSTR2BYTES,
		OSTR2RUNES,
		ORUNESTR,
		OSLICE2ARR,
		OSLICE2ARRPTR:
		n := n.(*ConvExpr)
		if n.Type() == nil || n.Type().Sym() == nil {
			fmt.Fprintf(s, "(%v)", n.Type())
		} else {
			fmt.Fprintf(s, "%v", n.Type())
		}
		fmt.Fprintf(s, "(%v)", n.X)

	case OREAL,
		OIMAG,
		OCAP,
		OCLEAR,
		OCLOSE,
		OLEN,
		ONEW,
		OPANIC:
		n := n.(*UnaryExpr)
		fmt.Fprintf(s, "%v(%v)", n.Op(), n.X)

	case OAPPEND,
		ODELETE,
		OMAKE,
		OMAX,
		OMIN,
		ORECOVER,
		OPRINT,
		OPRINTLN:
		n := n.(*CallExpr)
		if n.IsDDD {
			fmt.Fprintf(s, "%v(%.v...)", n.Op(), n.Args)
			return
		}
		fmt.Fprintf(s, "%v(%.v)", n.Op(), n.Args)

	case OCALL, OCALLFUNC, OCALLINTER, OCALLMETH, OGETG:
		n := n.(*CallExpr)
		exprFmt(n.Fun, s, nprec)
		if n.IsDDD {
			fmt.Fprintf(s, "(%.v...)", n.Args)
			return
		}
		fmt.Fprintf(s, "(%.v)", n.Args)

	case OINLCALL:
		n := n.(*InlinedCallExpr)
		// TODO(mdempsky): Print Init and/or Body?
		if len(n.ReturnVars) == 1 {
			fmt.Fprintf(s, "%v", n.ReturnVars[0])
			return
		}
		fmt.Fprintf(s, "(.%v)", n.ReturnVars)

	case OMAKEMAP, OMAKECHAN, OMAKESLICE:
		n := n.(*MakeExpr)
		if n.Cap != nil {
			fmt.Fprintf(s, "make(%v, %v, %v)", n.Type(), n.Len, n.Cap)
			return
		}
		if n.Len != nil && (n.Op() == OMAKESLICE || !n.Len.Type().IsUntyped()) {
			fmt.Fprintf(s, "make(%v, %v)", n.Type(), n.Len)
			return
		}
		fmt.Fprintf(s, "make(%v)", n.Type())

	case OMAKESLICECOPY:
		n := n.(*MakeExpr)
		fmt.Fprintf(s, "makeslicecopy(%v, %v, %v)", n.Type(), n.Len, n.Cap)

	case OPLUS, ONEG, OBITNOT, ONOT, ORECV:
		// Unary
		n := n.(*UnaryExpr)
		fmt.Fprintf(s, "%v", n.Op())
		if n.X != nil && n.X.Op() == n.Op() {
			fmt.Fprint(s, " ")
		}
		exprFmt(n.X, s, nprec+1)

	case OADDR:
		n := n.(*AddrExpr)
		fmt.Fprintf(s, "%v", n.Op())
		if n.X != nil && n.X.Op() == n.Op() {
			fmt.Fprint(s, " ")
		}
		exprFmt(n.X, s, nprec+1)

	case ODEREF:
		n := n.(*StarExpr)
		fmt.Fprintf(s, "%v", n.Op())
		exprFmt(n.X, s, nprec+1)

		// Binary
	case OADD,
		OAND,
		OANDNOT,
		ODIV,
		OEQ,
		OGE,
		OGT,
		OLE,
		OLT,
		OLSH,
		OMOD,
		OMUL,
		ONE,
		OOR,
		ORSH,
		OSUB,
		OXOR:
		n := n.(*BinaryExpr)
		exprFmt(n.X, s, nprec)
		fmt.Fprintf(s, " %v ", n.Op())
		exprFmt(n.Y, s, nprec+1)

	case OANDAND,
		OOROR:
		n := n.(*LogicalExpr)
		exprFmt(n.X, s, nprec)
		fmt.Fprintf(s, " %v ", n.Op())
		exprFmt(n.Y, s, nprec+1)

	case OSEND:
		n := n.(*SendStmt)
		exprFmt(n.Chan, s, nprec)
		fmt.Fprintf(s, " <- ")
		exprFmt(n.Value, s, nprec+1)

	case OADDSTR:
		n := n.(*AddStringExpr)
		for i, n1 := range n.List {
			if i != 0 {
				fmt.Fprint(s, " + ")
			}
			exprFmt(n1, s, nprec)
		}
	default:
		fmt.Fprintf(s, "<node %v>", n.Op())
	}
}

func ellipsisIf(b bool) string {
	if b {
		return "..."
	}
	return ""
}

// Nodes

// Format implements formatting for a Nodes.
// The valid formats are:
//
//	%v	Go syntax, semicolon-separated
//	%.v	Go syntax, comma-separated
//	%+v	Debug syntax, as in DumpList.
func (l Nodes) Format(s fmt.State, verb rune) {
	if s.Flag('+') && verb == 'v' {
		// %+v is DumpList output
		dumpNodes(s, l, 1)
		return
	}

	if verb != 'v' {
		fmt.Fprintf(s, "%%!%c(Nodes)", verb)
		return
	}

	sep := "; "
	if _, ok := s.Precision(); ok { // %.v is expr list
		sep = ", "
	}

	for i, n := range l {
		fmt.Fprint(s, n)
		if i+1 < len(l) {
			fmt.Fprint(s, sep)
		}
	}
}

// Dump

// Dump prints the message s followed by a debug dump of n.
// This includes all the recursive structure under n.
func Dump(s string, n Node) {
	fmt.Printf("%s%+v\n", s, n)
}

// FDump prints to w the message s followed by a debug dump of n.
// This includes all the recursive structure under n.
func FDump(w io.Writer, s string, n Node) {
	fmt.Fprintf(w, "%s%+v\n", s, n)
}

// DumpList prints the message s followed by a debug dump of each node in the list.
// This includes all the recursive structure under each node in the list.
func DumpList(s string, list Nodes) {
	var buf bytes.Buffer
	FDumpList(&buf, s, list)
	os.Stdout.Write(buf.Bytes())
}

// FDumpList prints to w the message s followed by a debug dump of each node in the list.
// This includes all the recursive structure under each node in the list.
func FDumpList(w io.Writer, s string, list Nodes) {
	io.WriteString(w, s)
	dumpNodes(w, list, 1)
	io.WriteString(w, "\n")
}

// indent prints indentation to w.
func indent(w io.Writer, depth int) {
	fmt.Fprint(w, "\n")
	for i := 0; i < depth; i++ {
		fmt.Fprint(w, ".   ")
	}
}

// EscFmt is set by the escape analysis code to add escape analysis details to the node print.
var EscFmt func(n Node) string

// dumpNodeHeader prints the debug-format node header line to w.
func dumpNodeHeader(w io.Writer, n Node) {
	// Useful to see which nodes in an AST printout are actually identical
	if base.Debug.DumpPtrs != 0 {
		fmt.Fprintf(w, " p(%p)", n)
	}

	if base.Debug.DumpPtrs != 0 && n.Name() != nil && n.Name().Defn != nil {
		// Useful to see where Defn is set and what node it points to
		fmt.Fprintf(w, " defn(%p)", n.Name().Defn)
	}

	if base.Debug.DumpPtrs != 0 && n.Name() != nil && n.Name().Curfn != nil {
		// Useful to see where Defn is set and what node it points to
		fmt.Fprintf(w, " curfn(%p)", n.Name().Curfn)
	}
	if base.Debug.DumpPtrs != 0 && n.Name() != nil && n.Name().Outer != nil {
		// Useful to see where Defn is set and what node it points to
		fmt.Fprintf(w, " outer(%p)", n.Name().Outer)
	}

	if EscFmt != nil {
		if esc := EscFmt(n); esc != "" {
			fmt.Fprintf(w, " %s", esc)
		}
	}

	if n.Sym() != nil && n.Op() != ONAME && n.Op() != ONONAME && n.Op() != OTYPE {
		fmt.Fprintf(w, " %+v", n.Sym())
	}

	// Print Node-specific fields of basic type in header line.
	v := reflect.ValueOf(n).Elem()
	t := v.Type()
	nf := t.NumField()
	for i := 0; i < nf; i++ {
		tf := t.Field(i)
		if tf.PkgPath != "" {
			// skip unexported field - Interface will fail
			continue
		}
		k := tf.Type.Kind()
		if reflect.Bool <= k && k <= reflect.Complex128 {
			name := strings.TrimSuffix(tf.Name, "_")
			vf := v.Field(i)
			vfi := vf.Interface()
			if name == "Offset" && vfi == types.BADWIDTH || name != "Offset" && vf.IsZero() {
				continue
			}
			if vfi == true {
				fmt.Fprintf(w, " %s", name)
			} else {
				fmt.Fprintf(w, " %s:%+v", name, vf.Interface())
			}
		}
	}

	// Print Node-specific booleans by looking for methods.
	// Different v, t from above - want *Struct not Struct, for methods.
	v = reflect.ValueOf(n)
	t = v.Type()
	nm := t.NumMethod()
	for i := 0; i < nm; i++ {
		tm := t.Method(i)
		if tm.PkgPath != "" {
			// skip unexported method - call will fail
			continue
		}
		m := v.Method(i)
		mt := m.Type()
		if mt.NumIn() == 0 && mt.NumOut() == 1 && mt.Out(0).Kind() == reflect.Bool {
			// TODO(rsc): Remove the func/defer/recover wrapping,
			// which is guarding against panics in miniExpr,
			// once we get down to the simpler state in which
			// nodes have no getter methods that aren't allowed to be called.
			func() {
				defer func() { recover() }()
				if m.Call(nil)[0].Bool() {
					name := strings.TrimSuffix(tm.Name, "_")
					fmt.Fprintf(w, " %s", name)
				}
			}()
		}
	}

	if n.Op() == OCLOSURE {
		n := n.(*ClosureExpr)
		if fn := n.Func; fn != nil && fn.Nname.Sym() != nil {
			fmt.Fprintf(w, " fnName(%+v)", fn.Nname.Sym())
		}
	}

	if n.Type() != nil {
		if n.Op() == OTYPE {
			fmt.Fprintf(w, " type")
		}
		fmt.Fprintf(w, " %+v", n.Type())
	}
	if n.Typecheck() != 0 {
		fmt.Fprintf(w, " tc(%d)", n.Typecheck())
	}

	if n.Pos().IsKnown() {
		fmt.Fprint(w, " # ")
		switch n.Pos().IsStmt() {
		case src.PosNotStmt:
			fmt.Fprint(w, "_") // "-" would be confusing
		case src.PosIsStmt:
			fmt.Fprint(w, "+")
		}
		sep := ""
		base.Ctxt.AllPos(n.Pos(), func(pos src.Pos) {
			fmt.Fprint(w, sep)
			sep = " "
			// TODO(mdempsky): Print line pragma details too.
			file := filepath.Base(pos.Filename())
			// Note: this output will be parsed by ssa/html.go:(*HTMLWriter).WriteAST. Keep in sync.
			fmt.Fprintf(w, "%s:%d:%d", file, pos.Line(), pos.Col())
		})
	}
}

func dumpNode(w io.Writer, n Node, depth int) {
	indent(w, depth)
	if depth > 40 {
		fmt.Fprint(w, "...")
		return
	}

	if n == nil {
		fmt.Fprint(w, "NilIrNode")
		return
	}

	if len(n.Init()) != 0 {
		fmt.Fprintf(w, "%+v-init", n.Op())
		dumpNodes(w, n.Init(), depth+1)
		indent(w, depth)
	}

	switch n.Op() {
	default:
		fmt.Fprintf(w, "%+v", n.Op())
		dumpNodeHeader(w, n)

	case OLITERAL:
		fmt.Fprintf(w, "%+v-%v", n.Op(), n.Val())
		dumpNodeHeader(w, n)
		return

	case ONAME, ONONAME:
		if n.Sym() != nil {
			fmt.Fprintf(w, "%+v-%+v", n.Op(), n.Sym())
		} else {
			fmt.Fprintf(w, "%+v", n.Op())
		}
		dumpNodeHeader(w, n)
		return

	case OLINKSYMOFFSET:
		n := n.(*LinksymOffsetExpr)
		fmt.Fprintf(w, "%+v-%v", n.Op(), n.Linksym)
		// Offset is almost always 0, so only print when it's interesting.
		if n.Offset_ != 0 {
			fmt.Fprintf(w, "%+v", n.Offset_)
		}
		dumpNodeHeader(w, n)

	case OASOP:
		n := n.(*AssignOpStmt)
		fmt.Fprintf(w, "%+v-%+v", n.Op(), n.AsOp)
		dumpNodeHeader(w, n)

	case OTYPE:
		fmt.Fprintf(w, "%+v %+v", n.Op(), n.Sym())
		dumpNodeHeader(w, n)
		return

	case OCLOSURE:
		fmt.Fprintf(w, "%+v", n.Op())
		dumpNodeHeader(w, n)

	case ODCLFUNC:
		// Func has many fields we don't want to print.
		// Bypass reflection and just print what we want.
		n := n.(*Func)
		fmt.Fprintf(w, "%+v", n.Op())
		dumpNodeHeader(w, n)
		fn := n
		if len(fn.Dcl) > 0 {
			indent(w, depth)
			fmt.Fprintf(w, "%+v-Dcl", n.Op())
			for _, dcl := range n.Dcl {
				dumpNode(w, dcl, depth+1)
			}
		}
		if len(fn.ClosureVars) > 0 {
			indent(w, depth)
			fmt.Fprintf(w, "%+v-ClosureVars", n.Op())
			for _, cv := range fn.ClosureVars {
				dumpNode(w, cv, depth+1)
			}
		}
		if len(fn.Body) > 0 {
			indent(w, depth)
			fmt.Fprintf(w, "%+v-body", n.Op())
			dumpNodes(w, fn.Body, depth+1)
		}
		return
	}

	v := reflect.ValueOf(n).Elem()
	t := reflect.TypeOf(n).Elem()
	nf := t.NumField()
	for i := 0; i < nf; i++ {
		tf := t.Field(i)
		vf := v.Field(i)
		if tf.PkgPath != "" {
			// skip unexported field - Interface will fail
			continue
		}
		switch tf.Type.Kind() {
		case reflect.Interface, reflect.Ptr, reflect.Slice:
			if vf.IsNil() {
				continue
			}
		}
		name := strings.TrimSuffix(tf.Name, "_")
		// Do not bother with field name header lines for the
		// most common positional arguments: unary, binary expr,
		// index expr, send stmt, go and defer call expression.
		switch name {
		case "X", "Y", "Index", "Chan", "Value", "Call":
			name = ""
		}
		switch val := vf.Interface().(type) {
		case Node:
			if name != "" {
				indent(w, depth)
				fmt.Fprintf(w, "%+v-%s", n.Op(), name)
			}
			dumpNode(w, val, depth+1)
		case Nodes:
			if len(val) == 0 {
				continue
			}
			if name != "" {
				indent(w, depth)
				fmt.Fprintf(w, "%+v-%s", n.Op(), name)
			}
			dumpNodes(w, val, depth+1)
		default:
			if vf.Kind() == reflect.Slice && vf.Type().Elem().Implements(nodeType) {
				if vf.Len() == 0 {
					continue
				}
				if name != "" {
					indent(w, depth)
					fmt.Fprintf(w, "%+v-%s", n.Op(), name)
				}
				for i, n := 0, vf.Len(); i < n; i++ {
					dumpNode(w, vf.Index(i).Interface().(Node), depth+1)
				}
			}
		}
	}
}

var nodeType = reflect.TypeFor[Node]()

func dumpNodes(w io.Writer, list Nodes, depth int) {
	if len(list) == 0 {
		fmt.Fprintf(w, " <nil>")
		return
	}

	for _, n := range list {
		dumpNode(w, n, depth)
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/func.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/hash"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"
	"encoding/base64"
	"fmt"
	"unicode/utf8"
)

// A Func corresponds to a single function in a Go program
// (and vice versa: each function is denoted by exactly one *Func).
//
// There are multiple nodes that represent a Func in the IR.
//
// The ONAME node (Func.Nname) is used for plain references to it.
// The ODCLFUNC node (the Func itself) is used for its declaration code.
// The OCLOSURE node (Func.OClosure) is used for a reference to a
// function literal.
//
// An imported function will have an ONAME node which points to a Func
// with an empty body.
// A declared function or method has an ODCLFUNC (the Func itself) and an ONAME.
// A function literal is represented directly by an OCLOSURE, but it also
// has an ODCLFUNC (and a matching ONAME) representing the compiled
// underlying form of the closure, which accesses the captured variables
// using a special data structure passed in a register.
//
// A method declaration is represented like functions, except f.Sym
// will be the qualified method name (e.g., "T.m").
//
// A method expression (T.M) is represented as an OMETHEXPR node,
// in which n.Left and n.Right point to the type and method, respectively.
// Each distinct mention of a method expression in the source code
// constructs a fresh node.
//
// A method value (t.M) is represented by ODOTMETH/ODOTINTER
// when it is called directly and by OMETHVALUE otherwise.
// These are like method expressions, except that for ODOTMETH/ODOTINTER,
// the method name is stored in Sym instead of Right.
// Each OMETHVALUE ends up being implemented as a new
// function, a bit like a closure, with its own ODCLFUNC.
// The OMETHVALUE uses n.Func to record the linkage to
// the generated ODCLFUNC, but there is no
// pointer from the Func back to the OMETHVALUE.
type Func struct {
	// if you add or remove a field, don't forget to update sizeof_test.go

	miniNode
	Body Nodes

	Nname    *Name        // ONAME node
	OClosure *ClosureExpr // OCLOSURE node

	// ONAME nodes for all params/locals for this func/closure, does NOT
	// include closurevars until transforming closures during walk.
	// Names must be listed PPARAMs, PPARAMOUTs, then PAUTOs,
	// with PPARAMs and PPARAMOUTs in order corresponding to the function signature.
	// Anonymous and blank params are declared as ~pNN (for PPARAMs) and ~rNN (for PPARAMOUTs).
	Dcl []*Name

	// ClosureVars lists the free variables that are used within a
	// function literal, but formally declared in an enclosing
	// function. The variables in this slice are the closure function's
	// own copy of the variables, which are used within its function
	// body. They will also each have IsClosureVar set, and will have
	// Byval set if they're captured by value.
	ClosureVars []*Name

	// Enclosed functions that need to be compiled.
	// Populated during walk.
	Closures []*Func

	// Parent of a closure
	ClosureParent *Func

	// Parents records the parent scope of each scope within a
	// function. The root scope (0) has no parent, so the i'th
	// scope's parent is stored at Parents[i-1].
	Parents []ScopeID

	// Marks records scope boundary changes.
	Marks []Mark

	FieldTrack map[*obj.LSym]struct{}
	DebugInfo  any
	LSym       *obj.LSym // Linker object in this function's native ABI (Func.ABI)

	Inl *Inline

	// RangeParent, if non-nil, is the first non-range body function containing
	// the closure for the body of a range function.
	RangeParent *Func

	// funcLitGen, rangeLitGen and goDeferGen track how many closures have been
	// created in this function for function literals, range-over-func loops,
	// and go/defer wrappers, respectively. Used by closureName for creating
	// unique function names.
	// Tracking goDeferGen separately avoids wrappers throwing off
	// function literal numbering (e.g., runtime/trace_test.TestTraceSymbolize.func11).
	funcLitGen  int32
	rangeLitGen int32
	goDeferGen  int32

	Label int32 // largest auto-generated label in this function

	Endlineno src.XPos
	WBPos     src.XPos // position of first write barrier; see SetWBPos

	Pragma PragmaFlag // go:xxx function annotations

	flags bitset16

	// ABI is a function's "definition" ABI. This is the ABI that
	// this function's generated code is expecting to be called by.
	//
	// For most functions, this will be obj.ABIInternal. It may be
	// a different ABI for functions defined in assembly or ABI wrappers.
	//
	// This is included in the export data and tracked across packages.
	ABI obj.ABI
	// ABIRefs is the set of ABIs by which this function is referenced.
	// For ABIs other than this function's definition ABI, the
	// compiler generates ABI wrapper functions. This is only tracked
	// within a package.
	ABIRefs obj.ABISet

	NumDefers  int32 // number of defer calls in the function
	NumReturns int32 // number of explicit returns in the function

	// NWBRCalls records the LSyms of functions called by this
	// function for go:nowritebarrierrec analysis. Only filled in
	// if nowritebarrierrecCheck != nil.
	NWBRCalls *[]SymAndPos

	// For wrapper functions, WrappedFunc point to the original Func.
	// Currently only used for go/defer wrappers.
	WrappedFunc *Func

	// WasmImport is used by the //go:wasmimport directive to store info about
	// a WebAssembly function import.
	WasmImport *WasmImport
	// WasmExport is used by the //go:wasmexport directive to store info about
	// a WebAssembly function export.
	WasmExport *WasmExport
}

// WasmImport stores metadata associated with the //go:wasmimport pragma.
type WasmImport struct {
	Module string
	Name   string
}

// WasmExport stores metadata associated with the //go:wasmexport pragma.
type WasmExport struct {
	Name string
}

// NewFunc returns a new Func with the given name and type.
//
// fpos is the position of the "func" token, and npos is the position
// of the name identifier.
//
// TODO(mdempsky): I suspect there's no need for separate fpos and
// npos.
func NewFunc(fpos, npos src.XPos, sym *types.Sym, typ *types.Type) *Func {
	name := NewNameAt(npos, sym, typ)
	name.Class = PFUNC
	sym.SetFunc(true)

	fn := &Func{Nname: name}
	fn.pos = fpos
	fn.op = ODCLFUNC
	// Most functions are ABIInternal. The importer or symabis
	// pass may override this.
	fn.ABI = obj.ABIInternal
	fn.SetTypecheck(1)

	name.Func = fn

	return fn
}

func (f *Func) isStmt() {}

func (n *Func) copy() Node                                   { panic(n.no("copy")) }
func (n *Func) doChildren(do func(Node) bool) bool           { return doNodes(n.Body, do) }
func (n *Func) doChildrenWithHidden(do func(Node) bool) bool { return doNodes(n.Body, do) }
func (n *Func) editChildren(edit func(Node) Node)            { editNodes(n.Body, edit) }
func (n *Func) editChildrenWithHidden(edit func(Node) Node)  { editNodes(n.Body, edit) }

func (f *Func) Type() *types.Type                { return f.Nname.Type() }
func (f *Func) Sym() *types.Sym                  { return f.Nname.Sym() }
func (f *Func) Linksym() *obj.LSym               { return f.Nname.Linksym() }
func (f *Func) LinksymABI(abi obj.ABI) *obj.LSym { return f.Nname.LinksymABI(abi) }

// An Inline holds fields used for function bodies that can be inlined.
type Inline struct {
	Cost int32 // heuristic cost of inlining this function

	// Copy of Func.Dcl for use during inlining. This copy is needed
	// because the function's Dcl may change from later compiler
	// transformations. This field is also populated when a function
	// from another package is imported and inlined.
	Dcl     []*Name
	HaveDcl bool // whether we've loaded Dcl

	// Function properties, encoded as a string (these are used for
	// making inlining decisions). See cmd/compile/internal/inline/inlheur.
	Properties string

	// CanDelayResults reports whether it's safe for the inliner to delay
	// initializing the result parameters until immediately before the
	// "return" statement.
	CanDelayResults bool
}

// A Mark represents a scope boundary.
type Mark struct {
	// Pos is the position of the token that marks the scope
	// change.
	Pos src.XPos

	// Scope identifies the innermost scope to the right of Pos.
	Scope ScopeID
}

// A ScopeID represents a lexical scope within a function.
type ScopeID int32

const (
	funcDupok                    = 1 << iota // duplicate definitions ok
	funcWrapper                              // hide frame from users (elide in tracebacks, don't count as a frame for recover())
	funcABIWrapper                           // is an ABI wrapper (also set flagWrapper)
	funcNeedctxt                             // function uses context register (has closure variables)
	funcHasDefer                             // contains a defer statement
	funcNilCheckDisabled                     // disable nil checks when compiling this function
	funcInlinabilityChecked                  // inliner has already determined whether the function is inlinable
	funcNeverReturns                         // function never returns (in most cases calls panic(), os.Exit(), or equivalent)
	funcOpenCodedDeferDisallowed             // can't do open-coded defers
	funcClosureResultsLost                   // closure is called indirectly and we lost track of its results; used by escape analysis
	funcPackageInit                          // compiler emitted .init func for package
)

type SymAndPos struct {
	Sym *obj.LSym // LSym of callee
	Pos src.XPos  // line of call
}

func (f *Func) Dupok() bool                    { return f.flags&funcDupok != 0 }
func (f *Func) Wrapper() bool                  { return f.flags&funcWrapper != 0 }
func (f *Func) ABIWrapper() bool               { return f.flags&funcABIWrapper != 0 }
func (f *Func) Needctxt() bool                 { return f.flags&funcNeedctxt != 0 }
func (f *Func) HasDefer() bool                 { return f.flags&funcHasDefer != 0 }
func (f *Func) NilCheckDisabled() bool         { return f.flags&funcNilCheckDisabled != 0 }
func (f *Func) InlinabilityChecked() bool      { return f.flags&funcInlinabilityChecked != 0 }
func (f *Func) NeverReturns() bool             { return f.flags&funcNeverReturns != 0 }
func (f *Func) OpenCodedDeferDisallowed() bool { return f.flags&funcOpenCodedDeferDisallowed != 0 }
func (f *Func) ClosureResultsLost() bool       { return f.flags&funcClosureResultsLost != 0 }
func (f *Func) IsPackageInit() bool            { return f.flags&funcPackageInit != 0 }

func (f *Func) SetDupok(b bool)                    { f.flags.set(funcDupok, b) }
func (f *Func) SetWrapper(b bool)                  { f.flags.set(funcWrapper, b) }
func (f *Func) SetABIWrapper(b bool)               { f.flags.set(funcABIWrapper, b) }
func (f *Func) SetNeedctxt(b bool)                 { f.flags.set(funcNeedctxt, b) }
func (f *Func) SetHasDefer(b bool)                 { f.flags.set(funcHasDefer, b) }
func (f *Func) SetNilCheckDisabled(b bool)         { f.flags.set(funcNilCheckDisabled, b) }
func (f *Func) SetInlinabilityChecked(b bool)      { f.flags.set(funcInlinabilityChecked, b) }
func (f *Func) SetNeverReturns(b bool)             { f.flags.set(funcNeverReturns, b) }
func (f *Func) SetOpenCodedDeferDisallowed(b bool) { f.flags.set(funcOpenCodedDeferDisallowed, b) }
func (f *Func) SetClosureResultsLost(b bool)       { f.flags.set(funcClosureResultsLost, b) }
func (f *Func) SetIsPackageInit(b bool)            { f.flags.set(funcPackageInit, b) }

func (f *Func) SetWBPos(pos src.XPos) {
	if base.Debug.WB != 0 {
		base.WarnfAt(pos, "write barrier")
	}
	if !f.WBPos.IsKnown() {
		f.WBPos = pos
	}
}

// IsClosure reports whether f is a function literal that captures at least one value.
func (f *Func) IsClosure() bool {
	if f.OClosure == nil {
		return false
	}
	return len(f.ClosureVars) > 0
}

// FuncName returns the name (without the package) of the function f.
func FuncName(f *Func) string {
	if f == nil || f.Nname == nil {
		return "<nil>"
	}
	return f.Sym().Name
}

// PkgFuncName returns the name of the function referenced by f, with package
// prepended.
//
// This differs from the compiler's internal convention where local functions
// lack a package. This is primarily useful when the ultimate consumer of this
// is a human looking at message.
func PkgFuncName(f *Func) string {
	if f == nil || f.Nname == nil {
		return "<nil>"
	}
	s := f.Sym()
	pkg := s.Pkg
	if pkg == nil {
		return "<nil>." + s.Name
	}
	return pkg.Path + "." + s.Name
}

// LinkFuncName returns the name of the function f, as it will appear in the
// symbol table of the final linked binary.
func LinkFuncName(f *Func) string {
	if f == nil || f.Nname == nil {
		return "<nil>"
	}
	s := f.Sym()
	pkg := s.Pkg

	return objabi.PathToPrefix(pkg.Path) + "." + s.Name
}

// ParseLinkFuncName parsers a symbol name (as returned from LinkFuncName) back
// to the package path and local symbol name.
func ParseLinkFuncName(name string) (pkg, sym string, err error) {
	pkg, sym = splitPkg(name)
	if pkg == "" {
		return "", "", fmt.Errorf("no package path in name")
	}

	pkg, err = objabi.PrefixToPath(pkg) // unescape
	if err != nil {
		return "", "", fmt.Errorf("malformed package path: %v", err)
	}

	return pkg, sym, nil
}

// Borrowed from x/mod.
func modPathOK(r rune) bool {
	if r < utf8.RuneSelf {
		return r == '-' || r == '.' || r == '_' || r == '~' ||
			'0' <= r && r <= '9' ||
			'A' <= r && r <= 'Z' ||
			'a' <= r && r <= 'z'
	}
	return false
}

func escapedImportPathOK(r rune) bool {
	return modPathOK(r) || r == '+' || r == '/' || r == '%'
}

// splitPkg splits the full linker symbol name into package and local symbol
// name.
func splitPkg(name string) (pkgpath, sym string) {
	// package-sym split is at first dot after last the / that comes before
	// any characters illegal in a package path.

	lastSlashIdx := 0
	for i, r := range name {
		// Catches cases like:
		// * example.foo[sync/atomic.Uint64].
		// * example%2ecom.foo[sync/atomic.Uint64].
		//
		// Note that name is still escaped; unescape occurs after splitPkg.
		if !escapedImportPathOK(r) {
			break
		}
		if r == '/' {
			lastSlashIdx = i
		}
	}
	for i := lastSlashIdx; i < len(name); i++ {
		r := name[i]
		if r == '.' {
			return name[:i], name[i+1:]
		}
	}

	return "", name
}

var CurFunc *Func

// WithFunc invokes do with CurFunc and base.Pos set to curfn and
// curfn.Pos(), respectively, and then restores their previous values
// before returning.
func WithFunc(curfn *Func, do func()) {
	oldfn, oldpos := CurFunc, base.Pos
	defer func() { CurFunc, base.Pos = oldfn, oldpos }()

	CurFunc, base.Pos = curfn, curfn.Pos()
	do()
}

func FuncSymName(s *types.Sym) string {
	return s.Name + "·f"
}

// ClosureDebugRuntimeCheck applies boilerplate checks for debug flags
// and compiling runtime.
func ClosureDebugRuntimeCheck(clo *ClosureExpr) {
	if base.Debug.Closure > 0 {
		if clo.Esc() == EscHeap {
			base.WarnfAt(clo.Pos(), "heap closure, captured vars = %v", clo.Func.ClosureVars)
		} else {
			base.WarnfAt(clo.Pos(), "stack closure, captured vars = %v", clo.Func.ClosureVars)
		}
	}
	if base.Flag.CompilingRuntime && clo.Esc() == EscHeap && !clo.IsGoWrap {
		base.ErrorfAt(clo.Pos(), 0, "heap-allocated closure %s, not allowed in runtime", FuncName(clo.Func))
	}
}

// globClosgen is like Func.Closgen, but for the global scope.
var globClosgen int32

// closureName generates a new unique name for a closure within outerfn at pos.
// gen is an optional counter for the closure name. If it is 0, the counter
// will be computed based on outerfn.
func closureName(outerfn *Func, pos src.XPos, why Op, gen int) *types.Sym {
	pkg := types.LocalPkg
	outer := "glob."
	var suffix string = "."
	switch why {
	default:
		base.FatalfAt(pos, "closureName: bad Op: %v", why)
	case OCLOSURE:
		if outerfn.OClosure == nil {
			suffix = ".func"
		}
	case ORANGE:
		suffix = "-range"
	case OGO:
		suffix = ".gowrap"
	case ODEFER:
		suffix = ".deferwrap"
	}

	// There may be multiple functions named "_". In those
	// cases, we can't use their individual Closgens as it
	// would lead to name clashes.
	if !IsBlank(outerfn.Nname) {
		pkg = outerfn.Sym().Pkg
		outer = FuncName(outerfn)
	}

	// If this closure was created due to inlining, find the original
	// outer function's name for the closure (#60324).
	var inlHash string
	if inlIndex := base.Ctxt.InnermostPos(pos).Base().InliningIndex(); inlIndex >= 0 {
		// The compiler doesn't like multiple symbols with the same
		// name. We make a unique suffix temporarily for the
		// compiler, and strip it during object file writing, so
		// it will not be the linker symbol name. For linking,
		// we use a content hash to disambiguate instead.
		// We choose the suffix as a hash of the inline call stack.
		h := hash.New32()
		fmt.Fprint(h, inlIndex)
		base.Ctxt.InlTree.AllParents(inlIndex, func(call obj.InlinedCall) {
			if call.Parent >= 0 {
				fmt.Fprint(h, " ", call.Parent)
			}
		})
		inlHash = base64.StdEncoding.EncodeToString(h.Sum(nil)[:8])

		outer = base.Ctxt.InlTree.InlinedFuncName(inlIndex)
		if pkgPath := base.Ctxt.InlTree.InlinedFuncPkg(inlIndex); pkgPath != "" {
			pkg = types.NewPkg(pkgPath, "")
		}
	}

	if gen == 0 {
		p := &globClosgen
		if !IsBlank(outerfn.Nname) {
			switch why {
			case OCLOSURE:
				p = &outerfn.funcLitGen
			case ORANGE:
				p = &outerfn.rangeLitGen
			default:
				p = &outerfn.goDeferGen
			}
		}
		*p++
		gen = int(*p)
	}

	name := fmt.Sprintf("%s%s%d", outer, suffix, gen)
	if inlHash != "" {
		// Attach the inline hash (see the comment above).
		// If it already has a hash, trim it, so we don't include
		// two hashes for nested closures. The new hash should be
		// enough to disambiguate.
		name = obj.TrimInlineHash(name) + "#" + inlHash + "#"
	}

	return pkg.Lookup(name)
}

// NewClosureFunc creates a new Func to represent a function literal
// with the given type.
//
// fpos the position used for the underlying ODCLFUNC and ONAME,
// whereas cpos is the position used for the OCLOSURE. They're
// separate because in the presence of inlining, the OCLOSURE node
// should have an inline-adjusted position, whereas the ODCLFUNC and
// ONAME must not.
//
// outerfn is the enclosing function. The returned function is
// appending to pkg.Funcs.
//
// why is the reason we're generating this Func. It can be OCLOSURE
// (for a normal function literal) or OGO or ODEFER (for wrapping a
// call expression that has parameters or results).
//
// gen is an optional counter for the closure name. If it is 0,
// the counter will be computed based on outerfn.
func NewClosureFunc(fpos, cpos src.XPos, why Op, typ *types.Type, outerfn *Func, pkg *Package, gen int) *Func {
	if outerfn == nil {
		base.FatalfAt(fpos, "outerfn is nil")
	}

	fn := NewFunc(fpos, fpos, closureName(outerfn, cpos, why, gen), typ)
	fn.SetDupok(outerfn.Dupok()) // if the outer function is dupok, so is the closure

	fn.Linksym().Set(obj.AttrContentAddressable, true)

	clo := &ClosureExpr{Func: fn}
	clo.op = OCLOSURE
	clo.pos = cpos
	clo.SetType(typ)
	clo.SetTypecheck(1)
	if why == ORANGE {
		clo.Func.RangeParent = outerfn
		if outerfn.OClosure != nil && outerfn.OClosure.Func.RangeParent != nil {
			clo.Func.RangeParent = outerfn.OClosure.Func.RangeParent
		}
	}
	fn.OClosure = clo

	fn.Nname.Defn = fn
	pkg.Funcs = append(pkg.Funcs, fn)
	fn.ClosureParent = outerfn

	return fn
}

// IsFuncPCIntrinsic returns whether n is a direct call of internal/abi.FuncPCABIxxx functions.
func IsFuncPCIntrinsic(n *CallExpr) bool {
	if n.Op() != OCALLFUNC || n.Fun.Op() != ONAME {
		return false
	}
	fn := n.Fun.(*Name).Sym()
	return (fn.Name == "FuncPCABI0" || fn.Name == "FuncPCABIInternal") &&
		fn.Pkg.Path == "internal/abi"
}

// IsIfaceOfFunc inspects whether n is an interface conversion from a direct
// reference of a func. If so, it returns referenced Func; otherwise nil.
//
// This is only usable before walk.walkConvertInterface, which converts to an
// OMAKEFACE.
func IsIfaceOfFunc(n Node) *Func {
	if n, ok := n.(*ConvExpr); ok && n.Op() == OCONVIFACE {
		if name, ok := n.X.(*Name); ok && name.Op() == ONAME && name.Class == PFUNC {
			return name.Func
		}
	}
	return nil
}

// FuncPC returns a uintptr-typed expression that evaluates to the PC of a
// function as uintptr, as returned by internal/abi.FuncPC{ABI0,ABIInternal}.
//
// n should be a Node of an interface type, as is passed to
// internal/abi.FuncPC{ABI0,ABIInternal}.
//
// TODO(prattmic): Since n is simply an interface{} there is no assertion that
// it is actually a function at all. Perhaps we should emit a runtime type
// assertion?
func FuncPC(pos src.XPos, n Node, wantABI obj.ABI) Node {
	if !n.Type().IsInterface() {
		base.ErrorfAt(pos, 0, "internal/abi.FuncPC%s expects an interface value, got %v", wantABI, n.Type())
	}

	if fn := IsIfaceOfFunc(n); fn != nil {
		name := fn.Nname
		abi := fn.ABI
		if abi != wantABI {
			base.ErrorfAt(pos, 0, "internal/abi.FuncPC%s expects an %v function, %s is defined as %v", wantABI, wantABI, name.Sym().Name, abi)
		}
		var e Node = NewLinksymExpr(pos, name.LinksymABI(abi), types.Types[types.TUINTPTR])
		e = NewAddrExpr(pos, e)
		e.SetType(types.Types[types.TUINTPTR].PtrTo())
		e = NewConvExpr(pos, OCONVNOP, types.Types[types.TUINTPTR], e)
		e.SetTypecheck(1)
		return e
	}
	// fn is not a defined function. It must be ABIInternal.
	// Read the address from func value, i.e. *(*uintptr)(idata(fn)).
	if wantABI != obj.ABIInternal {
		base.ErrorfAt(pos, 0, "internal/abi.FuncPC%s does not accept func expression, which is ABIInternal", wantABI)
	}
	var e Node = NewUnaryExpr(pos, OIDATA, n)
	e.SetType(types.Types[types.TUINTPTR].PtrTo())
	e.SetTypecheck(1)
	e = NewStarExpr(pos, e)
	e.SetType(types.Types[types.TUINTPTR])
	e.SetTypecheck(1)
	return e
}

// DeclareParams creates Names for all of the parameters in fn's
// signature and adds them to fn.Dcl.
//
// If setNname is true, then it also sets types.Field.Nname for each
// parameter.
func (fn *Func) DeclareParams(setNname bool) {
	if fn.Dcl != nil {
		base.FatalfAt(fn.Pos(), "%v already has Dcl", fn)
	}

	declareParams := func(params []*types.Field, ctxt Class, prefix string, offset int) {
		for i, param := range params {
			sym := param.Sym
			if sym == nil || sym.IsBlank() {
				sym = fn.Sym().Pkg.LookupNum(prefix, i)
			}

			name := NewNameAt(param.Pos, sym, param.Type)
			name.Class = ctxt
			name.Curfn = fn
			fn.Dcl[offset+i] = name

			if setNname {
				param.Nname = name
			}
		}
	}

	sig := fn.Type()
	params := sig.RecvParams()
	results := sig.Results()

	fn.Dcl = make([]*Name, len(params)+len(results))
	declareParams(params, PPARAM, "~p", 0)
	declareParams(results, PPARAMOUT, "~r", len(params))
}

// ContainsClosure reports whether c is a closure contained within f.
func ContainsClosure(f, c *Func) bool {
	// Common cases.
	if f == c || c.OClosure == nil {
		return false
	}

	for p := c.ClosureParent; p != nil; p = p.ClosureParent {
		if p == f {
			return true
		}
	}
	return false
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/html.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"bufio"
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/src"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

// An HTMLWriter dumps IR to multicolumn HTML, similar to what the
// ssa backend does for GOSSAFUNC.  This is not the format used for
// the ast column in GOSSAFUNC output.
type HTMLWriter struct {
	HTMLWriterBase
	Func *Func
}

type HTMLWriterBase struct {
	w             *BufferedWriterCloser
	canonIdMap    map[any]int
	prevCanonId   int
	path          string
	prevHash      []byte
	pendingPhases []string
	pendingTitles []string
	doDump        func(string) func()
}

func (h *HTMLWriterBase) Init(out io.WriteCloser, reportPath string, doDump func(string) func()) {
	h.w = NewBufferedWriterCloser(out)
	h.canonIdMap = make(map[any]int)
	h.path = reportPath
	h.doDump = doDump
}

// BufferedWriterCloser is here to help avoid pre-buffering the whole
// rendered HTML in memory, which can cause problems for large inputs.
type BufferedWriterCloser struct {
	file io.Closer
	w    *bufio.Writer
}

func (b *BufferedWriterCloser) Write(p []byte) (n int, err error) {
	return b.w.Write(p)
}

func (b *BufferedWriterCloser) Close() error {
	b.w.Flush()
	b.w = nil
	return b.file.Close()
}

func NewBufferedWriterCloser(f io.WriteCloser) *BufferedWriterCloser {
	return &BufferedWriterCloser{file: f, w: bufio.NewWriter(f)}
}

func NewHTMLWriter(path string, f *Func, cfgMask string) *HTMLWriter {
	path = strings.ReplaceAll(path, "/", string(filepath.Separator))
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		base.Fatalf("%v", err)
	}
	reportPath := path
	if !filepath.IsAbs(reportPath) {
		pwd, err := os.Getwd()
		if err != nil {
			base.Fatalf("%v", err)
		}
		reportPath = filepath.Join(pwd, path)
	}
	h := HTMLWriter{
		Func: f,
	}
	h.Init(out, reportPath, h.FuncHTML)
	h.start()
	return &h
}

func (h *HTMLWriterBase) Path() string {
	return h.path
}

// CanonId assigns indices to nodes based on pointer identity.
// this helps ensure that output html files don't gratuitously
// differ from run to run.
func (h *HTMLWriterBase) CanonId(n any) int {
	if id := h.canonIdMap[n]; id > 0 {
		return id
	}
	h.prevCanonId++
	h.canonIdMap[n] = h.prevCanonId
	return h.prevCanonId
}

// Fatalf reports an error and exits.
func (w *HTMLWriterBase) Fatalf(msg string, args ...any) {
	base.FatalfAt(src.NoXPos, msg, args...)
}

const (
	RightArrow = "►" // \u25BA click-to-open (is closed)
	DownArrow  = "▼" // \u25BC click-to-close (is open)
)

func (w *HTMLWriter) start() {
	if w == nil {
		return
	}
	escName := html.EscapeString(PkgFuncName(w.Func))
	w.Print("<!DOCTYPE html>")
	w.Print("<html>")
	w.Printf(`<head>
<meta name="generator" content="AST display for %s">
<meta http-equiv="Content-Type" content="text/html;charset=UTF-8">
%s
%s
<title>AST display for %s</title>
</head>`, escName, CSS, JS("bloop", "loopvar", "escape", "slice", "walk"), escName)
	w.Print("<body>")
	w.Print("<h1>")
	w.Print(html.EscapeString(w.Func.Sym().Name))
	w.Print("</h1>")
	w.Print(`
<a href="#" onclick="toggle_visibility('help');return false;" id="helplink">help</a>
<div id="help">

<p>
Click anywhere on a node (with "cell" cursor) to outline a node and all of its subtrees.
</p>
<p>
Click on a name (with "crosshair" cursor) to highlight every occurrence of a name.
(Note that all the name nodes are the same node, so those also all outline together).
</p>
<p>
Click on a file, line, or column (with "crosshair" cursor) to highlight positions
in that file, at that file:line, or at that file:line:column, respectively.<br>Inlined
locations are not treated as a single location, but as a sequence of locations that
can be independently highlighted.
</p>
<p>
Click on a ` + DownArrow + ` to collapse a subtree, or on a ` + RightArrow + ` to expand a subtree.
</p>


</div>
<label for="dark-mode-button" style="margin-left: 15px; cursor: pointer;">darkmode</label>
<input type="checkbox" onclick="toggleDarkMode();" id="dark-mode-button" style="cursor: pointer" />
`)
	w.Print("<table>")
	w.Print("<tr>")
}

func (w *HTMLWriterBase) Close(format string, args ...any) {
	if w == nil {
		return
	}
	w.Print("</tr>")
	w.Print("</table>")
	w.Print("</body>")
	w.Print("</html>\n")
	w.w.Close()
	fmt.Fprintf(os.Stderr, format, args...)
}

// WritePhase writes f in a column headed by title.
// phase is used for collapsing columns and should be unique across the table.
func (w *HTMLWriterBase) WritePhase(phase, title string) {
	if w == nil {
		return // avoid generating HTML just to discard it
	}
	w.pendingPhases = append(w.pendingPhases, phase)
	w.pendingTitles = append(w.pendingTitles, title)
	w.flushPhases()
}

// flushPhases collects any pending phases and titles, writes them to the html, and resets the pending slices.
func (w *HTMLWriterBase) flushPhases() {
	phaseLen := len(w.pendingPhases)
	if phaseLen == 0 {
		return
	}
	phases := strings.Join(w.pendingPhases, "  +  ")
	w.WriteMultiTitleColumn(
		phases,
		w.pendingTitles,
		"allow-x-scroll",
		w.doDump(w.pendingPhases[phaseLen-1]),
	)
	w.pendingPhases = w.pendingPhases[:0]
	w.pendingTitles = w.pendingTitles[:0]
}

func (w *HTMLWriterBase) WriteMultiTitleColumn(phase string, titles []string, class string, writeContent func()) {
	if w == nil {
		return
	}
	id := strings.ReplaceAll(phase, " ", "-")
	// collapsed column
	w.Printf("<td id=\"%v-col\" class=\"collapsed\"><div>%v</div></td>", id, phase)

	if class == "" {
		w.Printf("<td id=\"%v-exp\">", id)
	} else {
		w.Printf("<td id=\"%v-exp\" class=\"%v\">", id, class)
	}
	for _, title := range titles {
		w.Print("<h2>" + title + "</h2>")
	}
	writeContent()
	w.Print("<div class=\"resizer\"></div>")
	w.Print("</td>\n")
}

func (w *HTMLWriterBase) Printf(msg string, v ...any) {
	if _, err := fmt.Fprintf(w.w, msg, v...); err != nil {
		w.Fatalf("%v", err)
	}
}

func (w *HTMLWriterBase) Print(s string) {
	if _, err := fmt.Fprint(w.w, s); err != nil {
		w.Fatalf("%v", err)
	}
}

func (w *HTMLWriterBase) indent(n int) {
	indent(w.w, n)
}

func (w *HTMLWriter) FuncHTML(phase string) func() {
	return func() {
		w.Print("<pre>") // use pre for formatting to preserve indentation
		w.dumpNodesHTML(w.Func.Body, 1)
		w.Print("</pre>")
	}
}

func (h *HTMLWriter) dumpNodesHTML(list Nodes, depth int) {
	if len(list) == 0 {
		h.Print(" <nil>")
		return
	}

	for _, n := range list {
		h.dumpNodeHTML(n, depth)
	}
}

const indentString = ".   "

// indent prints indentation to w.
func (h *HTMLWriterBase) indentForToggle(depth int, hasChildren bool) {
	h.Print("\n")
	if depth == 0 {
		return
	}
	for i := 0; i < depth-1; i++ {
		h.Print(indentString)
	}
	if hasChildren {
		// Remove 2 spaces, which have similar rendered width to
		// leading ir.DownArrow and trailing space.
		h.Print(indentString[:len(indentString)-2])
	} else {
		h.Print(indentString)
	}
}

func (h *HTMLWriter) dumpNodeHTML(n Node, depth int) {
	hasChildren := nodeHasChildren(n)
	h.indentForToggle(depth, hasChildren)

	if depth > 40 {
		h.Print("...")
		return
	}

	if n == nil {
		h.Print("NilIrNode")
		return
	}

	// For HTML, we want to wrap the node and its details in a span that can be highlighted
	// across all occurrences of the span in all columns, so it has to be linked to the node ID,
	// which is its address. Canonicalize the address to a counter so that repeated compiler
	// runs yield the same html.
	//
	// JS Equivalence logic:
	//   var c = elem.classList.item(0);
	//   var x = document.getElementsByClassName(c);
	//
	// Tag each class with its canonicalized index.

	h.Printf("<span class=\"n%d outline-node\">", h.CanonId(n))
	defer h.Printf("</span>")

	if hasChildren {
		h.Print(`<span class="toggle" onclick="toggle_node(this)">` + DownArrow + `</span> `) // NOTE TRAILING SPACE after </span>!
	}

	if len(n.Init()) != 0 {
		h.Print(`<span class="node-body">`)
		h.Printf("%+v-init", n.Op())
		h.dumpNodesHTML(n.Init(), depth+1)
		h.indent(depth)
		h.Print(`</span>`)
	}

	switch n.Op() {
	default:
		h.Printf("%+v", n.Op())
		h.dumpNodeHeaderHTML(n)

	case OLITERAL:
		h.Printf("%+v-%v", n.Op(), html.EscapeString(fmt.Sprintf("%v", n.Val())))
		h.dumpNodeHeaderHTML(n)
		return

	case ONAME, ONONAME:
		if n.Sym() != nil {
			// Name highlighting:
			// Create a hash for the symbol name to use as a class
			// We use the same irValueClicked logic which uses the first class as the identifier
			name := fmt.Sprintf("%v", n.Sym())
			hash := sha256.Sum256([]byte(name))
			symID := "sym-" + hex.EncodeToString(hash[:6])
			h.Printf("%+v-<span class=\"%s variable-name\">%+v</span>", n.Op(), symID, html.EscapeString(name))
		} else {
			h.Printf("%+v", n.Op())
		}
		h.dumpNodeHeaderHTML(n)
		return

	case OLINKSYMOFFSET:
		n := n.(*LinksymOffsetExpr)
		h.Printf("%+v-%v", n.Op(), html.EscapeString(fmt.Sprintf("%v", n.Linksym)))
		if n.Offset_ != 0 {
			h.Printf("%+v", n.Offset_)
		}
		h.dumpNodeHeaderHTML(n)

	case OASOP:
		n := n.(*AssignOpStmt)
		h.Printf("%+v-%+v", n.Op(), n.AsOp)
		h.dumpNodeHeaderHTML(n)

	case OTYPE:
		h.Printf("%+v %+v", n.Op(), html.EscapeString(fmt.Sprintf("%v", n.Sym())))
		h.dumpNodeHeaderHTML(n)
		return

	case OCLOSURE:
		h.Printf("%+v", n.Op())
		h.dumpNodeHeaderHTML(n)

	case ODCLFUNC:
		n := n.(*Func)
		h.Printf("%+v", n.Op())
		h.dumpNodeHeaderHTML(n)
		if hasChildren {
			h.Print(`<span class="node-body">`)
			defer h.Print(`</span>`)
		}
		fn := n
		if len(fn.Dcl) > 0 {
			h.indent(depth)
			h.Printf("%+v-Dcl", n.Op())
			for _, dcl := range n.Dcl {
				h.dumpNodeHTML(dcl, depth+1)
			}
		}
		if len(fn.ClosureVars) > 0 {
			h.indent(depth)
			h.Printf("%+v-ClosureVars", n.Op())
			for _, cv := range fn.ClosureVars {
				h.dumpNodeHTML(cv, depth+1)
			}
		}
		if len(fn.Body) > 0 {
			h.indent(depth)
			h.Printf("%+v-body", n.Op())
			h.dumpNodesHTML(fn.Body, depth+1)
		}
		return
	}
	if hasChildren {
		h.Print(`<span class="node-body">`)
		defer h.Print(`</span>`)
	}

	v := reflect.ValueOf(n).Elem()
	t := reflect.TypeOf(n).Elem()
	nf := t.NumField()
	for i := 0; i < nf; i++ {
		tf := t.Field(i)
		vf := v.Field(i)
		if tf.PkgPath != "" {
			continue
		}
		switch tf.Type.Kind() {
		case reflect.Interface, reflect.Ptr, reflect.Slice:
			if vf.IsNil() {
				continue
			}
		}
		name := strings.TrimSuffix(tf.Name, "_")
		switch name {
		case "X", "Y", "Index", "Chan", "Value", "Call":
			name = ""
		}
		switch val := vf.Interface().(type) {
		case Node:
			if name != "" {
				h.indent(depth)
				h.Printf("%+v-%s", n.Op(), name)
			}
			h.dumpNodeHTML(val, depth+1)
		case Nodes:
			if len(val) == 0 {
				continue
			}
			if name != "" {
				h.indent(depth)
				h.Printf("%+v-%s", n.Op(), name)
			}
			h.dumpNodesHTML(val, depth+1)
		default:
			if vf.Kind() == reflect.Slice && vf.Type().Elem().Implements(nodeType) {
				if vf.Len() == 0 {
					continue
				}
				if name != "" {
					h.indent(depth)
					h.Printf("%+v-%s", n.Op(), name)
				}
				for i, n := 0, vf.Len(); i < n; i++ {
					h.dumpNodeHTML(vf.Index(i).Interface().(Node), depth+1)
				}
			}
		}
	}
}

func nodeHasChildren(n Node) bool {
	if n == nil {
		return false
	}
	if len(n.Init()) != 0 {
		return true
	}
	switch n.Op() {
	case OLITERAL, ONAME, ONONAME, OTYPE:
		return false
	case ODCLFUNC:
		n := n.(*Func)
		return len(n.Dcl) > 0 || len(n.ClosureVars) > 0 || len(n.Body) > 0
	}

	v := reflect.ValueOf(n).Elem()
	t := reflect.TypeOf(n).Elem()
	nf := t.NumField()
	for i := 0; i < nf; i++ {
		tf := t.Field(i)
		vf := v.Field(i)
		if tf.PkgPath != "" {
			continue
		}
		switch tf.Type.Kind() {
		case reflect.Interface, reflect.Ptr, reflect.Slice:
			if vf.IsNil() {
				continue
			}
		}
		switch val := vf.Interface().(type) {
		case Node:
			return true
		case Nodes:
			if len(val) > 0 {
				return true
			}
		default:
			if vf.Kind() == reflect.Slice && vf.Type().Elem().Implements(nodeType) {
				if vf.Len() > 0 {
					return true
				}
			}
		}
	}
	return false
}

func (h *HTMLWriter) dumpNodeHeaderHTML(n Node) {
	// print pointer to be able to see identical nodes
	if base.Debug.DumpPtrs != 0 {
		h.Printf(" p(%p)", n)
	}

	if base.Debug.DumpPtrs != 0 && n.Name() != nil && n.Name().Defn != nil {
		h.Printf(" defn(%p)", n.Name().Defn)
	}

	if base.Debug.DumpPtrs != 0 && n.Name() != nil && n.Name().Curfn != nil {
		h.Printf(" curfn(%p)", n.Name().Curfn)
	}
	if base.Debug.DumpPtrs != 0 && n.Name() != nil && n.Name().Outer != nil {
		h.Printf(" outer(%p)", n.Name().Outer)
	}

	if EscFmt != nil {
		if esc := EscFmt(n); esc != "" {
			h.Printf(" %s", html.EscapeString(esc))
		}
	}

	if n.Sym() != nil && n.Op() != ONAME && n.Op() != ONONAME && n.Op() != OTYPE {
		h.Printf(" %+v", html.EscapeString(fmt.Sprintf("%v", n.Sym())))
	}

	v := reflect.ValueOf(n).Elem()
	t := v.Type()
	nf := t.NumField()
	for i := 0; i < nf; i++ {
		tf := t.Field(i)
		if tf.PkgPath != "" {
			continue
		}
		k := tf.Type.Kind()
		if reflect.Bool <= k && k <= reflect.Complex128 {
			name := strings.TrimSuffix(tf.Name, "_")
			vf := v.Field(i)
			vfi := vf.Interface()
			if name == "Offset" && vfi == types.BADWIDTH || name != "Offset" && vf.IsZero() {
				continue
			}
			if vfi == true {
				h.Printf(" %s", name)
			} else {
				h.Printf(" %s:%+v", name, html.EscapeString(fmt.Sprintf("%v", vf.Interface())))
			}
		}
	}

	v = reflect.ValueOf(n)
	t = v.Type()
	nm := t.NumMethod()
	for i := 0; i < nm; i++ {
		tm := t.Method(i)
		if tm.PkgPath != "" {
			continue
		}
		m := v.Method(i)
		mt := m.Type()
		if mt.NumIn() == 0 && mt.NumOut() == 1 && mt.Out(0).Kind() == reflect.Bool {
			func() {
				defer func() { recover() }()
				if m.Call(nil)[0].Bool() {
					name := strings.TrimSuffix(tm.Name, "_")
					h.Printf(" %s", name)
				}
			}()
		}
	}

	if n.Op() == OCLOSURE {
		n := n.(*ClosureExpr)
		if fn := n.Func; fn != nil && fn.Nname.Sym() != nil {
			h.Printf(" fnName(%+v)", html.EscapeString(fmt.Sprintf("%v", fn.Nname.Sym())))
		}
	}

	if n.Type() != nil {
		if n.Op() == OTYPE {
			h.Printf(" type")
		}
		h.Printf(" %+v", html.EscapeString(fmt.Sprintf("%v", n.Type())))
	}
	if n.Typecheck() != 0 {
		h.Printf(" tc(%d)", n.Typecheck())
	}

	if n.Pos().IsKnown() {
		h.Print(" <span class=\"line-number\">")
		switch n.Pos().IsStmt() {
		case src.PosNotStmt:
			h.Print("_")
		case src.PosIsStmt:
			h.Print("+")
		}
		sep := ""
		base.Ctxt.AllPos(n.Pos(), func(pos src.Pos) {
			h.Print(sep)
			sep = " "
			// Hierarchical highlighting:
			// Click file -> highlight all ranges in this file
			// Click line -> highlight all ranges at this line (in this file)
			// Click col  -> highlight this specific range

			file := pos.Filename()
			// Create a hash for the filename to use as a class
			hash := sha256.Sum256([]byte(file))
			fileID := "loc-" + hex.EncodeToString(hash[:6])
			lineID := fmt.Sprintf("%s-L%d", fileID, pos.Line())
			colID := fmt.Sprintf("%s-C%d", lineID, pos.Col())

			// File part: triggers fileID
			h.Printf("<span class=\"%s line-number\">%s</span>:", fileID, html.EscapeString(filepath.Base(file)))
			// Line part: triggers lineID (and fileID via class list)
			h.Printf("<span class=\"%s %s line-number\">%d</span>:", lineID, fileID, pos.Line())
			// Col part: triggers colID (and lineID, fileID)
			h.Printf("<span class=\"%s %s %s line-number\">%d</span>", colID, lineID, fileID, pos.Col())
		})
		h.Print("</span>")
	}
}

const CSS = `<style>

body {
    font-size: 14px;
    font-family: Arial, sans-serif;
}

h1 {
    font-size: 18px;
    display: inline-block;
    margin: 0 1em .5em 0;
}

#helplink {
    display: inline-block;
}

#help {
    display: none;
}

table {
    border: 1px solid black;
    table-layout: fixed;
    width: 300px;
}

th, td {
    border: 1px solid black;
    overflow: hidden;
    width: 400px;
    vertical-align: top;
    padding: 5px;
    position: relative;
}

.resizer {
    display: inline-block;
    background: transparent;
    width: 10px;
    height: 100%;
    position: absolute;
    right: 0;
    top: 0;
    cursor: col-resize;
    z-index: 100;
}

td > h2 {
    cursor: pointer;
    font-size: 120%;
    margin: 5px 0px 5px 0px;
}

td.collapsed {
    font-size: 12px;
    width: 12px;
    border: 1px solid white;
    padding: 2px;
    cursor: pointer;
    background: #fafafa;
}

td.collapsed div {
    text-align: right;
    transform: rotate(180deg);
    writing-mode: vertical-lr;
    white-space: pre;
}

pre {
    font-family: Menlo, monospace;
    font-size: 12px;
}

pre {
    -moz-tab-size: 4;
    -o-tab-size:   4;
    tab-size:      4;
}

.allow-x-scroll {
    overflow-x: scroll;
}

.outline-node {
    cursor: cell;
}

.variable-name {
    cursor: crosshair;
}

.line-number {
    font-size: 11px;
    cursor: crosshair;
}

body.darkmode {
    background-color: rgb(21, 21, 21);
    color: rgb(230, 255, 255);
    opacity: 100%;
}

td.darkmode {
    background-color: rgb(21, 21, 21);
    border: 1px solid gray;
}

body.darkmode table, th {
    border: 1px solid gray;
}

body.darkmode text {
    fill: white;
}

.highlight-aquamarine     { background-color: aquamarine; color: black; }
.highlight-coral          { background-color: coral; color: black; }
.highlight-lightpink      { background-color: lightpink; color: black; }
.highlight-lightsteelblue { background-color: lightsteelblue; color: black; }
.highlight-palegreen      { background-color: palegreen; color: black; }
.highlight-skyblue        { background-color: skyblue; color: black; }
.highlight-lightgray      { background-color: lightgray; color: black; }
.highlight-yellow         { background-color: yellow; color: black; }
.highlight-lime           { background-color: lime; color: black; }
.highlight-khaki          { background-color: khaki; color: black; }
.highlight-aqua           { background-color: aqua; color: black; }
.highlight-salmon         { background-color: salmon; color: black; }


.outline-blue           { outline: #2893ff solid 2px; }
.outline-red            { outline: red solid 2px; }
.outline-blueviolet     { outline: blueviolet solid 2px; }
.outline-darkolivegreen { outline: darkolivegreen solid 2px; }
.outline-fuchsia        { outline: fuchsia solid 2px; }
.outline-sienna         { outline: sienna solid 2px; }
.outline-gold           { outline: gold solid 2px; }
.outline-orangered      { outline: orangered solid 2px; }
.outline-teal           { outline: teal solid 2px; }
.outline-maroon         { outline: maroon solid 2px; }
.outline-black          { outline: black solid 2px; }

/* Capture alternative for outline-black and ellipse.outline-black when in dark mode */
body.darkmode .outline-black        { outline: gray solid 2px; }

.toggle {
    cursor: pointer;
    display: inline-block;
    text-align: center;
    user-select: none;
    font-size: 12px; // hand-tweaked
}

</style>
`

// safePhaseNameString is a very conservative limit on phase names
// that can safely be encoded as JavaScript strings by wrapping with
// double-quotes.
var safePhaseNameString = regexp.MustCompile("^[a-zA-Z0-9_ .]+$")

func JS(opened ...string) string {
	var middle strings.Builder

	// "bloop",
	// "loopvar",
	// "escape",
	// "slice",
	// "walk",

	// This is only for default-display purposes, and the expected strings
	// are the names of compiler phases. If a wonky name is rejected, the
	// "harm" is that a pane in the debugging display is not pre-opened.
	for _, s := range opened {
		if !safePhaseNameString.MatchString(s) {
			continue
		}
		middle.WriteString("\t\"")
		middle.WriteString(s)
		middle.WriteString("\",\n")
	}
	return JS1 + middle.String() + JS2
}

const (
	JS1 = `<script type="text/javascript">

// Contains phase names which are expanded by default. Other columns are collapsed.
let expandedDefault = [
`
	JS2 = `];
if (history.state === null) {
    history.pushState({expandedDefault}, "", location.href);
}

// ordered list of all available highlight colors
var highlights = [
    "highlight-aquamarine",
    "highlight-coral",
    "highlight-lightpink",
    "highlight-lightsteelblue",
    "highlight-palegreen",
    "highlight-skyblue",
    "highlight-lightgray",
    "highlight-yellow",
    "highlight-lime",
    "highlight-khaki",
    "highlight-aqua",
    "highlight-salmon"
];

// state: which value is highlighted this color?
var highlighted = {};
for (var i = 0; i < highlights.length; i++) {
    highlighted[highlights[i]] = "";
}

// ordered list of all available outline colors
var outlines = [
    "outline-blue",
    "outline-red",
    "outline-blueviolet",
    "outline-darkolivegreen",
    "outline-fuchsia",
    "outline-sienna",
    "outline-gold",
    "outline-orangered",
    "outline-teal",
    "outline-maroon",
    "outline-black"
];

// state: which value is outlined this color?
var outlined = {};
for (var i = 0; i < outlines.length; i++) {
    outlined[outlines[i]] = "";
}

window.onload = function() {
    if (history.state !== null) {
        expandedDefault = history.state.expandedDefault;
    }
    if (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) {
        toggleDarkMode();
        document.getElementById("dark-mode-button").checked = true;
    }

    var irElemClicked = function(elem, event, selections, selected) {
        event.stopPropagation();

        // find all values with the same name
        var c = elem.classList.item(0);
        var x = document.getElementsByClassName(c);

        // if selected, remove selections from all of them
        // otherwise, attempt to add

        var remove = "";
        for (var i = 0; i < selections.length; i++) {
            var color = selections[i];
            if (selected[color] == c) {
                remove = color;
                break;
            }
        }

        if (remove != "") {
            for (var i = 0; i < x.length; i++) {
                x[i].classList.remove(remove);
            }
            selected[remove] = "";
            return;
        }

        // we're adding a selection
        // find first available color
        var avail = "";
        for (var i = 0; i < selections.length; i++) {
            var color = selections[i];
            if (selected[color] == "") {
                avail = color;
                break;
            }
        }
        if (avail == "") {
            alert("out of selection colors; go add more");
            return;
        }

        // set that as the selection
        for (var i = 0; i < x.length; i++) {
            x[i].classList.add(avail);
        }
        selected[avail] = c;
    };

    var irValueClicked = function(event) {
        irElemClicked(this, event, highlights, highlighted);
    };

    var irTreeClicked = function(event) {
        irElemClicked(this, event, outlines, outlined);
    };

    var irValues = document.getElementsByClassName("outline-node");
    for (var i = 0; i < irValues.length; i++) {
        irValues[i].addEventListener('click', irTreeClicked);
    }

    var lines = document.getElementsByClassName("line-number");
    for (var i = 0; i < lines.length; i++) {
        lines[i].addEventListener('click', irValueClicked);
    }

    var variableNames = document.getElementsByClassName("variable-name");
    for (var i = 0; i < variableNames.length; i++) {
        variableNames[i].addEventListener('click', irValueClicked);
    }

    function toggler(phase) {
        return function() {
            toggle_cell(phase+'-col');
            toggle_cell(phase+'-exp');
            const i = expandedDefault.indexOf(phase);
            if (i !== -1) {
                expandedDefault.splice(i, 1);
            } else {
                expandedDefault.push(phase);
            }
            history.pushState({expandedDefault}, "", location.href);
        };
    }

    function toggle_cell(id) {
        var e = document.getElementById(id);
        if (e.style.display == 'table-cell') {
            e.style.display = 'none';
        } else {
            e.style.display = 'table-cell';
        }
    }

    // Go through all columns and collapse needed phases.
    const td = document.getElementsByTagName("td");
    for (let i = 0; i < td.length; i++) {
        const id = td[i].id;
        const phase = id.substr(0, id.length-4);
        let show = expandedDefault.indexOf(phase) !== -1

        // If show == false, check to see if this is a combined column (multiple phases).
        // If combined, check each of the phases to see if they are in our expandedDefaults.
        // If any are found, that entire combined column gets shown.
        if (!show) {
            const combined = phase.split('--+--');
            const len = combined.length;
            if (len > 1) {
                for (let i = 0; i < len; i++) {
                    const num = expandedDefault.indexOf(combined[i]);
                    if (num !== -1) {
                        expandedDefault.splice(num, 1);
                        if (expandedDefault.indexOf(phase) === -1) {
                            expandedDefault.push(phase);
                            show = true;
                        }
                    }
                }
            }
        }
        if (id.endsWith("-exp")) {
            const h2Els = td[i].getElementsByTagName("h2");
            const len = h2Els.length;
            if (len > 0) {
                for (let i = 0; i < len; i++) {
                    h2Els[i].addEventListener('click', toggler(phase));
                }
            }
        } else {
            td[i].addEventListener('click', toggler(phase));
        }
        if (id.endsWith("-col") && show || id.endsWith("-exp") && !show) {
            td[i].style.display = 'none';
            continue;
        }
        td[i].style.display = 'table-cell';
    }

    var resizers = document.getElementsByClassName("resizer");
    for (var i = 0; i < resizers.length; i++) {
        var resizer = resizers[i];
        resizer.addEventListener('mousedown', initDrag, false);
    }
};

var startX, startWidth, resizableCol;

function initDrag(e) {
    resizableCol = this.parentElement;
    startX = e.clientX;
    startWidth = parseInt(document.defaultView.getComputedStyle(resizableCol).width, 10);
    document.documentElement.addEventListener('mousemove', doDrag, false);
    document.documentElement.addEventListener('mouseup', stopDrag, false);
}

function doDrag(e) {
    resizableCol.style.width = (startWidth + e.clientX - startX) + 'px';
}

function stopDrag(e) {
    document.documentElement.removeEventListener('mousemove', doDrag, false);
    document.documentElement.removeEventListener('mouseup', stopDrag, false);
}

function toggle_visibility(id) {
    var e = document.getElementById(id);
    if (e.style.display == 'block') {
        e.style.display = 'none';
    } else {
        e.style.display = 'block';
    }
}

function toggleDarkMode() {
    document.body.classList.toggle('darkmode');

    // Collect all of the "collapsed" elements and apply dark mode on each collapsed column
    const collapsedEls = document.getElementsByClassName('collapsed');
    const len = collapsedEls.length;

    for (let i = 0; i < len; i++) {
        collapsedEls[i].classList.toggle('darkmode');
    }
}

function toggle_node(e) {
    event.stopPropagation();
    var parent = e.parentNode;
    var children = parent.children;
    for (var i = 0; i < children.length; i++) {
        if (children[i].classList.contains("node-body")) {
            if (children[i].style.display == "none") {
                children[i].style.display = "";
            } else {
                children[i].style.display = "none";
            }
        }
    }
    if (e.innerText == "` + RightArrow + `") {
        e.innerText = "` + DownArrow + `";
    } else {
        e.innerText = "` + RightArrow + `";
    }
}

</script>
`
)

```

// === FILE: references/go/src/cmd/compile/internal/ir/ir.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

```

// === FILE: references/go/src/cmd/compile/internal/ir/mini.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run mknode.go

package ir

import (
	"cmd/compile/internal/types"
	"cmd/internal/src"
	"fmt"
	"go/constant"
)

// A miniNode is a minimal node implementation,
// meant to be embedded as the first field in a larger node implementation,
// at a cost of 12 bytes.
//
// A miniNode is NOT a valid Node by itself: the embedding struct
// must at the least provide:
//
//	func (n *MyNode) String() string { return fmt.Sprint(n) }
//	func (n *MyNode) rawCopy() Node { c := *n; return &c }
//	func (n *MyNode) Format(s fmt.State, verb rune) { FmtNode(n, s, verb) }
//
// The embedding struct should also fill in n.op in its constructor,
// for more useful panic messages when invalid methods are called,
// instead of implementing Op itself.
type miniNode struct {
	pos  src.XPos
	op   Op
	bits bitset8
	esc  uint16
}

// op can be read, but not written.
// An embedding implementation can provide a SetOp if desired.
// (The panicking SetOp is with the other panics below.)
func (n *miniNode) Op() Op            { return n.op }
func (n *miniNode) Pos() src.XPos     { return n.pos }
func (n *miniNode) SetPos(x src.XPos) { n.pos = x }
func (n *miniNode) Esc() uint16       { return n.esc }
func (n *miniNode) SetEsc(x uint16)   { n.esc = x }

const (
	miniTypecheckShift = 0
	miniWalked         = 1 << 2 // to prevent/catch re-walking
)

func (n *miniNode) Typecheck() uint8 { return n.bits.get2(miniTypecheckShift) }
func (n *miniNode) SetTypecheck(x uint8) {
	if x > 2 {
		panic(fmt.Sprintf("cannot SetTypecheck %d", x))
	}
	n.bits.set2(miniTypecheckShift, x)
}

func (n *miniNode) Walked() bool     { return n.bits&miniWalked != 0 }
func (n *miniNode) SetWalked(x bool) { n.bits.set(miniWalked, x) }

// Empty, immutable graph structure.

func (n *miniNode) Init() Nodes { return Nodes{} }

// Additional functionality unavailable.

func (n *miniNode) no(name string) string { return "cannot " + name + " on " + n.op.String() }

func (n *miniNode) Type() *types.Type       { return nil }
func (n *miniNode) SetType(*types.Type)     { panic(n.no("SetType")) }
func (n *miniNode) Name() *Name             { return nil }
func (n *miniNode) Sym() *types.Sym         { return nil }
func (n *miniNode) Val() constant.Value     { panic(n.no("Val")) }
func (n *miniNode) SetVal(v constant.Value) { panic(n.no("SetVal")) }
func (n *miniNode) NonNil() bool            { return false }
func (n *miniNode) MarkNonNil()             { panic(n.no("MarkNonNil")) }

```

// === FILE: references/go/src/cmd/compile/internal/ir/mknode.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// Note: this program must be run in this directory.
//   go run mknode.go

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"slices"
	"strings"
)

var fset = token.NewFileSet()

var buf bytes.Buffer

// concreteNodes contains all concrete types in the package that implement Node
// (except for the mini* types).
var concreteNodes []*ast.TypeSpec

// interfaceNodes contains all interface types in the package that implement Node.
var interfaceNodes []*ast.TypeSpec

// mini contains the embeddable mini types (miniNode, miniExpr, and miniStmt).
var mini = map[string]*ast.TypeSpec{}

// implementsNode reports whether the type t is one which represents a Node
// in the AST.
func implementsNode(t ast.Expr) bool {
	id, ok := t.(*ast.Ident)
	if !ok {
		return false // only named types
	}
	for _, ts := range interfaceNodes {
		if ts.Name.Name == id.Name {
			return true
		}
	}
	for _, ts := range concreteNodes {
		if ts.Name.Name == id.Name {
			return true
		}
	}
	return false
}

func isMini(t ast.Expr) bool {
	id, ok := t.(*ast.Ident)
	return ok && mini[id.Name] != nil
}

func isNamedType(t ast.Expr, name string) bool {
	if id, ok := t.(*ast.Ident); ok {
		if id.Name == name {
			return true
		}
	}
	return false
}

func main() {
	fmt.Fprintln(&buf, "// Code generated by mknode.go. DO NOT EDIT.")
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, "package ir")
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, `import "fmt"`)

	filter := func(file fs.FileInfo) bool {
		return !strings.HasPrefix(file.Name(), "mknode")
	}
	pkgs, err := parser.ParseDir(fset, ".", filter, 0)
	if err != nil {
		panic(err)
	}
	pkg := pkgs["ir"]

	// Find all the mini types. These let us determine which
	// concrete types implement Node, so we need to find them first.
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, s := range g.Specs {
				t, ok := s.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if strings.HasPrefix(t.Name.Name, "mini") {
					mini[t.Name.Name] = t
					// Double-check that it is or embeds miniNode.
					if t.Name.Name != "miniNode" {
						s := t.Type.(*ast.StructType)
						if !isNamedType(s.Fields.List[0].Type, "miniNode") {
							panic(fmt.Sprintf("can't find miniNode in %s", t.Name.Name))
						}
					}
				}
			}
		}
	}

	// Find all the declarations of concrete types that implement Node.
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, s := range g.Specs {
				t, ok := s.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if strings.HasPrefix(t.Name.Name, "mini") {
					// We don't treat the mini types as
					// concrete implementations of Node
					// (even though they are) because
					// we only use them by embedding them.
					continue
				}
				if isConcreteNode(t) {
					concreteNodes = append(concreteNodes, t)
				}
				if isInterfaceNode(t) {
					interfaceNodes = append(interfaceNodes, t)
				}
			}
		}
	}
	// Sort for deterministic output.
	slices.SortFunc(concreteNodes, func(a, b *ast.TypeSpec) int {
		return strings.Compare(a.Name.Name, b.Name.Name)
	})
	// Generate code for each concrete type.
	for _, t := range concreteNodes {
		processType(t)
	}
	// Add some helpers.
	generateHelpers()

	// Format and write output.
	out, err := format.Source(buf.Bytes())
	if err != nil {
		// write out mangled source so we can see the bug.
		out = buf.Bytes()
	}
	err = os.WriteFile("node_gen.go", out, 0666)
	if err != nil {
		log.Fatal(err)
	}
}

// isConcreteNode reports whether the type t is a concrete type
// implementing Node.
func isConcreteNode(t *ast.TypeSpec) bool {
	s, ok := t.Type.(*ast.StructType)
	if !ok {
		return false
	}
	for _, f := range s.Fields.List {
		if isMini(f.Type) {
			return true
		}
	}
	return false
}

// isInterfaceNode reports whether the type t is an interface type
// implementing Node (including Node itself).
func isInterfaceNode(t *ast.TypeSpec) bool {
	s, ok := t.Type.(*ast.InterfaceType)
	if !ok {
		return false
	}
	if t.Name.Name == "Node" {
		return true
	}
	if t.Name.Name == "OrigNode" || t.Name.Name == "InitNode" {
		// These we exempt from consideration (fields of
		// this type don't need to be walked or copied).
		return false
	}

	// Look for embedded Node type.
	// Note that this doesn't handle multi-level embedding, but
	// we have none of that at the moment.
	for _, f := range s.Methods.List {
		if len(f.Names) != 0 {
			continue
		}
		if isNamedType(f.Type, "Node") {
			return true
		}
	}
	return false
}

func processType(t *ast.TypeSpec) {
	name := t.Name.Name
	fmt.Fprintf(&buf, "\n")
	fmt.Fprintf(&buf, "func (n *%s) Format(s fmt.State, verb rune) { fmtNode(n, s, verb) }\n", name)

	switch name {
	case "Name", "Func":
		// Too specialized to automate.
		return
	}

	s := t.Type.(*ast.StructType)
	fields := s.Fields.List

	// Expand any embedded fields.
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f.Names) != 0 {
			continue // not embedded
		}
		if isMini(f.Type) {
			// Insert the fields of the embedded type into the main type.
			// (It would be easier just to append, but inserting in place
			// matches the old mknode behavior.)
			ss := mini[f.Type.(*ast.Ident).Name].Type.(*ast.StructType)
			var f2 []*ast.Field
			f2 = append(f2, fields[:i]...)
			f2 = append(f2, ss.Fields.List...)
			f2 = append(f2, fields[i+1:]...)
			fields = f2
			i--
			continue
		} else if isNamedType(f.Type, "origNode") {
			// Ignore this field
			copy(fields[i:], fields[i+1:])
			fields = fields[:len(fields)-1]
			i--
			continue
		} else {
			panic("unknown embedded field " + fmt.Sprintf("%v", f.Type))
		}
	}
	// Process fields.
	var copyBody strings.Builder
	var doChildrenBody strings.Builder
	var doChildrenWithHiddenBody strings.Builder
	var editChildrenBody strings.Builder
	var editChildrenWithHiddenBody strings.Builder
	var hasHidden bool
	for _, f := range fields {
		names := f.Names
		ft := f.Type
		hidden := false
		if f.Tag != nil {
			tag := f.Tag.Value[1 : len(f.Tag.Value)-1]
			if strings.HasPrefix(tag, "mknode:") {
				if tag[7:] == "\"-\"" {
					if !isNamedType(ft, "Node") {
						continue
					}
					hidden = true
				} else {
					panic(fmt.Sprintf("unexpected tag value: %s", tag))
				}
			}
		}
		if isNamedType(ft, "Nodes") {
			// Nodes == []Node
			ft = &ast.ArrayType{Elt: &ast.Ident{Name: "Node"}}
		}
		isSlice := false
		if a, ok := ft.(*ast.ArrayType); ok && a.Len == nil {
			isSlice = true
			ft = a.Elt
		}
		isPtr := false
		if p, ok := ft.(*ast.StarExpr); ok {
			isPtr = true
			ft = p.X
		}
		if !implementsNode(ft) {
			continue
		}
		for _, name := range names {
			ptr := ""
			if isPtr {
				ptr = "*"
			}
			if isSlice {
				fmt.Fprintf(&doChildrenWithHiddenBody,
					"if do%ss(n.%s, do) {\nreturn true\n}\n", ft, name)
				fmt.Fprintf(&editChildrenWithHiddenBody,
					"edit%ss(n.%s, edit)\n", ft, name)
			} else {
				fmt.Fprintf(&doChildrenWithHiddenBody,
					"if n.%s != nil && do(n.%s) {\nreturn true\n}\n", name, name)
				fmt.Fprintf(&editChildrenWithHiddenBody,
					"if n.%s != nil {\nn.%s = edit(n.%s).(%s%s)\n}\n", name, name, name, ptr, ft)
			}
			if hidden {
				hasHidden = true
				continue
			}
			if isSlice {
				fmt.Fprintf(&copyBody, "c.%s = copy%ss(c.%s)\n", name, ft, name)
				fmt.Fprintf(&doChildrenBody,
					"if do%ss(n.%s, do) {\nreturn true\n}\n", ft, name)
				fmt.Fprintf(&editChildrenBody,
					"edit%ss(n.%s, edit)\n", ft, name)
			} else {
				fmt.Fprintf(&doChildrenBody,
					"if n.%s != nil && do(n.%s) {\nreturn true\n}\n", name, name)
				fmt.Fprintf(&editChildrenBody,
					"if n.%s != nil {\nn.%s = edit(n.%s).(%s%s)\n}\n", name, name, name, ptr, ft)
			}
		}
	}
	fmt.Fprintf(&buf, "func (n *%s) copy() Node {\nc := *n\n", name)
	buf.WriteString(copyBody.String())
	buf.WriteString("return &c\n}\n")
	fmt.Fprintf(&buf, "func (n *%s) doChildren(do func(Node) bool) bool {\n", name)
	buf.WriteString(doChildrenBody.String())
	buf.WriteString("return false\n}\n")
	fmt.Fprintf(&buf, "func (n *%s) doChildrenWithHidden(do func(Node) bool) bool {\n", name)
	if hasHidden {
		buf.WriteString(doChildrenWithHiddenBody.String())
		buf.WriteString("return false\n}\n")
	} else {
		buf.WriteString("return n.doChildren(do)\n}\n")
	}
	fmt.Fprintf(&buf, "func (n *%s) editChildren(edit func(Node) Node) {\n", name)
	buf.WriteString(editChildrenBody.String())
	buf.WriteString("}\n")
	fmt.Fprintf(&buf, "func (n *%s) editChildrenWithHidden(edit func(Node) Node) {\n", name)
	if hasHidden {
		buf.WriteString(editChildrenWithHiddenBody.String())
	} else {
		buf.WriteString("n.editChildren(edit)\n")
	}
	buf.WriteString("}\n")
}

func generateHelpers() {
	for _, typ := range []string{"CaseClause", "CommClause", "Name", "Node"} {
		ptr := "*"
		if typ == "Node" {
			ptr = "" // interfaces don't need *
		}
		fmt.Fprintf(&buf, "\n")
		fmt.Fprintf(&buf, "func copy%ss(list []%s%s) []%s%s {\n", typ, ptr, typ, ptr, typ)
		fmt.Fprintf(&buf, "if list == nil { return nil }\n")
		fmt.Fprintf(&buf, "c := make([]%s%s, len(list))\n", ptr, typ)
		fmt.Fprintf(&buf, "copy(c, list)\n")
		fmt.Fprintf(&buf, "return c\n")
		fmt.Fprintf(&buf, "}\n")
		fmt.Fprintf(&buf, "func do%ss(list []%s%s, do func(Node) bool) bool {\n", typ, ptr, typ)
		fmt.Fprintf(&buf, "for _, x := range list {\n")
		fmt.Fprintf(&buf, "if x != nil && do(x) {\n")
		fmt.Fprintf(&buf, "return true\n")
		fmt.Fprintf(&buf, "}\n")
		fmt.Fprintf(&buf, "}\n")
		fmt.Fprintf(&buf, "return false\n")
		fmt.Fprintf(&buf, "}\n")
		fmt.Fprintf(&buf, "func edit%ss(list []%s%s, edit func(Node) Node) {\n", typ, ptr, typ)
		fmt.Fprintf(&buf, "for i, x := range list {\n")
		fmt.Fprintf(&buf, "if x != nil {\n")
		fmt.Fprintf(&buf, "list[i] = edit(x).(%s%s)\n", ptr, typ)
		fmt.Fprintf(&buf, "}\n")
		fmt.Fprintf(&buf, "}\n")
		fmt.Fprintf(&buf, "}\n")
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/name.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"
	"fmt"

	"go/constant"
)

// An Ident is an identifier, possibly qualified.
type Ident struct {
	miniExpr
	sym *types.Sym
}

func NewIdent(pos src.XPos, sym *types.Sym) *Ident {
	n := new(Ident)
	n.op = ONONAME
	n.pos = pos
	n.sym = sym
	return n
}

func (n *Ident) Sym() *types.Sym { return n.sym }

// Name holds Node fields used only by named nodes (ONAME, OTYPE, some OLITERAL).
type Name struct {
	miniExpr
	BuiltinOp Op         // uint8
	Class     Class      // uint8
	pragma    PragmaFlag // int16
	flags     bitset16
	DictIndex uint16 // index of the dictionary entry describing the type of this variable declaration plus 1
	sym       *types.Sym
	Func      *Func // TODO(austin): nil for I.M
	Offset_   int64
	val       constant.Value
	Opt       any      // for use by escape or slice analysis
	Embed     *[]Embed // list of embedded files, for ONAME var

	// For a local variable (not param) or extern, the initializing assignment (OAS or OAS2).
	// For a closure var, the ONAME node of the original (outermost) captured variable.
	// For the case-local variables of a type switch, the type switch guard (OTYPESW).
	// For a range variable, the range statement (ORANGE)
	// For a recv variable in a case of a select statement, the receive assignment (OSELRECV2)
	// For the name of a function, points to corresponding Func node.
	Defn Node

	// The function, method, or closure in which local variable or param is declared.
	Curfn *Func

	Heapaddr *Name // temp holding heap address of param

	// Outer points to the immediately enclosing function's copy of this
	// closure variable. If not a closure variable, then Outer is nil.
	Outer *Name
}

func (n *Name) isExpr() {}

func (n *Name) copy() Node                                   { panic(n.no("copy")) }
func (n *Name) doChildren(do func(Node) bool) bool           { return false }
func (n *Name) doChildrenWithHidden(do func(Node) bool) bool { return false }
func (n *Name) editChildren(edit func(Node) Node)            {}
func (n *Name) editChildrenWithHidden(edit func(Node) Node)  {}

// RecordFrameOffset records the frame offset for the name.
// It is used by package types when laying out function arguments.
func (n *Name) RecordFrameOffset(offset int64) {
	n.SetFrameOffset(offset)
}

// NewNameAt returns a new ONAME Node associated with symbol s at position pos.
// The caller is responsible for setting Curfn.
func NewNameAt(pos src.XPos, sym *types.Sym, typ *types.Type) *Name {
	if sym == nil {
		base.Fatalf("NewNameAt nil")
	}
	n := newNameAt(pos, ONAME, sym)
	if typ != nil {
		n.SetType(typ)
		n.SetTypecheck(1)
	}
	return n
}

// NewBuiltin returns a new Name representing a builtin function,
// either predeclared or from package unsafe.
func NewBuiltin(sym *types.Sym, op Op) *Name {
	n := newNameAt(src.NoXPos, ONAME, sym)
	n.BuiltinOp = op
	n.SetTypecheck(1)
	sym.Def = n
	return n
}

// NewLocal returns a new function-local variable with the given name and type.
func (fn *Func) NewLocal(pos src.XPos, sym *types.Sym, typ *types.Type) *Name {
	if fn.Dcl == nil {
		base.FatalfAt(pos, "must call DeclParams on %v first", fn)
	}

	n := NewNameAt(pos, sym, typ)
	n.Class = PAUTO
	n.Curfn = fn
	fn.Dcl = append(fn.Dcl, n)
	return n
}

// NewDeclNameAt returns a new Name associated with symbol s at position pos.
// The caller is responsible for setting Curfn.
func NewDeclNameAt(pos src.XPos, op Op, sym *types.Sym) *Name {
	if sym == nil {
		base.Fatalf("NewDeclNameAt nil")
	}
	switch op {
	case ONAME, OTYPE, OLITERAL:
		// ok
	default:
		base.Fatalf("NewDeclNameAt op %v", op)
	}
	return newNameAt(pos, op, sym)
}

// NewConstAt returns a new OLITERAL Node associated with symbol s at position pos.
func NewConstAt(pos src.XPos, sym *types.Sym, typ *types.Type, val constant.Value) *Name {
	if sym == nil {
		base.Fatalf("NewConstAt nil")
	}
	n := newNameAt(pos, OLITERAL, sym)
	n.SetType(typ)
	n.SetTypecheck(1)
	n.SetVal(val)
	return n
}

// newNameAt is like NewNameAt but allows sym == nil.
func newNameAt(pos src.XPos, op Op, sym *types.Sym) *Name {
	n := new(Name)
	n.op = op
	n.pos = pos
	n.sym = sym
	return n
}

func (n *Name) Name() *Name            { return n }
func (n *Name) Sym() *types.Sym        { return n.sym }
func (n *Name) SetSym(x *types.Sym)    { n.sym = x }
func (n *Name) SubOp() Op              { return n.BuiltinOp }
func (n *Name) SetSubOp(x Op)          { n.BuiltinOp = x }
func (n *Name) SetFunc(x *Func)        { n.Func = x }
func (n *Name) FrameOffset() int64     { return n.Offset_ }
func (n *Name) SetFrameOffset(x int64) { n.Offset_ = x }

func (n *Name) Linksym() *obj.LSym               { return n.sym.Linksym() }
func (n *Name) LinksymABI(abi obj.ABI) *obj.LSym { return n.sym.LinksymABI(abi) }

func (*Name) CanBeNtype()    {}
func (*Name) CanBeAnSSASym() {}
func (*Name) CanBeAnSSAAux() {}

// DiagName returns the symbol name for diagnostics.
// XXX should it be part of the formatter?
func (n *Name) DiagName() string { return obj.TrimInlineHash(fmt.Sprint(n.Sym())) }

// Pragma returns the PragmaFlag for p, which must be for an OTYPE.
func (n *Name) Pragma() PragmaFlag { return n.pragma }

// SetPragma sets the PragmaFlag for p, which must be for an OTYPE.
func (n *Name) SetPragma(flag PragmaFlag) { n.pragma = flag }

// Alias reports whether p, which must be for an OTYPE, is a type alias.
func (n *Name) Alias() bool { return n.flags&nameAlias != 0 }

// SetAlias sets whether p, which must be for an OTYPE, is a type alias.
func (n *Name) SetAlias(alias bool) { n.flags.set(nameAlias, alias) }

const (
	nameReadonly                 = 1 << iota
	nameByval                    // is the variable captured by value or by reference
	nameNeedzero                 // if it contains pointers, needs to be zeroed on function entry
	nameAutoTemp                 // is the variable a temporary (implies no dwarf info. reset if escapes to heap)
	nameUsed                     // for variable declared and not used error
	nameIsClosureVar             // PAUTOHEAP closure pseudo-variable; original (if any) at n.Defn
	nameIsOutputParamHeapAddr    // pointer to a result parameter's heap copy
	nameIsOutputParamInRegisters // output parameter in registers spills as an auto
	nameAddrtaken                // address taken, even if not moved to heap
	nameInlFormal                // PAUTO created by inliner, derived from callee formal
	nameInlLocal                 // PAUTO created by inliner, derived from callee local
	nameOpenDeferSlot            // if temporary var storing info for open-coded defers
	nameLibfuzzer8BitCounter     // if PEXTERN should be assigned to __sancov_cntrs section
	nameCoverageAuxVar           // instrumentation counter var or pkg ID for cmd/cover
	nameAlias                    // is type name an alias
	nameNonMergeable             // not a candidate for stack slot merging
)

func (n *Name) Readonly() bool                 { return n.flags&nameReadonly != 0 }
func (n *Name) Needzero() bool                 { return n.flags&nameNeedzero != 0 }
func (n *Name) AutoTemp() bool                 { return n.flags&nameAutoTemp != 0 }
func (n *Name) Used() bool                     { return n.flags&nameUsed != 0 }
func (n *Name) IsClosureVar() bool             { return n.flags&nameIsClosureVar != 0 }
func (n *Name) IsOutputParamHeapAddr() bool    { return n.flags&nameIsOutputParamHeapAddr != 0 }
func (n *Name) IsOutputParamInRegisters() bool { return n.flags&nameIsOutputParamInRegisters != 0 }
func (n *Name) Addrtaken() bool                { return n.flags&nameAddrtaken != 0 }
func (n *Name) InlFormal() bool                { return n.flags&nameInlFormal != 0 }
func (n *Name) InlLocal() bool                 { return n.flags&nameInlLocal != 0 }
func (n *Name) OpenDeferSlot() bool            { return n.flags&nameOpenDeferSlot != 0 }
func (n *Name) Libfuzzer8BitCounter() bool     { return n.flags&nameLibfuzzer8BitCounter != 0 }
func (n *Name) CoverageAuxVar() bool           { return n.flags&nameCoverageAuxVar != 0 }
func (n *Name) NonMergeable() bool             { return n.flags&nameNonMergeable != 0 }

func (n *Name) setReadonly(b bool)                 { n.flags.set(nameReadonly, b) }
func (n *Name) SetNeedzero(b bool)                 { n.flags.set(nameNeedzero, b) }
func (n *Name) SetAutoTemp(b bool)                 { n.flags.set(nameAutoTemp, b) }
func (n *Name) SetUsed(b bool)                     { n.flags.set(nameUsed, b) }
func (n *Name) SetIsClosureVar(b bool)             { n.flags.set(nameIsClosureVar, b) }
func (n *Name) SetIsOutputParamHeapAddr(b bool)    { n.flags.set(nameIsOutputParamHeapAddr, b) }
func (n *Name) SetIsOutputParamInRegisters(b bool) { n.flags.set(nameIsOutputParamInRegisters, b) }
func (n *Name) SetAddrtaken(b bool)                { n.flags.set(nameAddrtaken, b) }
func (n *Name) SetInlFormal(b bool)                { n.flags.set(nameInlFormal, b) }
func (n *Name) SetInlLocal(b bool)                 { n.flags.set(nameInlLocal, b) }
func (n *Name) SetOpenDeferSlot(b bool)            { n.flags.set(nameOpenDeferSlot, b) }
func (n *Name) SetLibfuzzer8BitCounter(b bool)     { n.flags.set(nameLibfuzzer8BitCounter, b) }
func (n *Name) SetCoverageAuxVar(b bool)           { n.flags.set(nameCoverageAuxVar, b) }
func (n *Name) SetNonMergeable(b bool)             { n.flags.set(nameNonMergeable, b) }

// OnStack reports whether variable n may reside on the stack.
func (n *Name) OnStack() bool {
	if n.Op() == ONAME {
		switch n.Class {
		case PPARAM, PPARAMOUT, PAUTO:
			return n.Esc() != EscHeap
		case PEXTERN, PAUTOHEAP:
			return false
		}
	}
	// Note: fmt.go:dumpNodeHeader calls all "func() bool"-typed
	// methods, but it can only recover from panics, not Fatalf.
	panic(fmt.Sprintf("%v: not a variable: %v", base.FmtPos(n.Pos()), n))
}

// MarkReadonly indicates that n is an ONAME with readonly contents.
func (n *Name) MarkReadonly() {
	if n.Op() != ONAME {
		base.Fatalf("Node.MarkReadonly %v", n.Op())
	}
	n.setReadonly(true)
	// Mark the linksym as readonly immediately
	// so that the SSA backend can use this information.
	// It will be overridden later during dumpglobls.
	n.Linksym().Type = objabi.SRODATA
}

// Val returns the constant.Value for the node.
func (n *Name) Val() constant.Value {
	if n.val == nil {
		return constant.MakeUnknown()
	}
	return n.val
}

// SetVal sets the constant.Value for the node.
func (n *Name) SetVal(v constant.Value) {
	if n.op != OLITERAL {
		panic(n.no("SetVal"))
	}
	AssertValidTypeForConst(n.Type(), v)
	n.val = v
}

// Canonical returns the logical declaration that n represents. If n
// is a closure variable, then Canonical returns the original Name as
// it appears in the function that immediately contains the
// declaration. Otherwise, Canonical simply returns n itself.
func (n *Name) Canonical() *Name {
	if n.IsClosureVar() && n.Defn != nil {
		n = n.Defn.(*Name)
	}
	return n
}

func (n *Name) SetByval(b bool) {
	if n.Canonical() != n {
		base.Fatalf("SetByval called on non-canonical variable: %v", n)
	}
	n.flags.set(nameByval, b)
}

func (n *Name) Byval() bool {
	// We require byval to be set on the canonical variable, but we
	// allow it to be accessed from any instance.
	return n.Canonical().flags&nameByval != 0
}

// NewClosureVar returns a new closure variable for fn to refer to
// outer variable n.
func NewClosureVar(pos src.XPos, fn *Func, n *Name) *Name {
	switch n.Class {
	case PAUTO, PPARAM, PPARAMOUT, PAUTOHEAP:
		// ok
	default:
		// Prevent mistaken capture of global variables.
		base.Fatalf("NewClosureVar: %+v", n)
	}

	c := NewNameAt(pos, n.Sym(), n.Type())
	c.Curfn = fn
	c.Class = PAUTOHEAP
	c.SetIsClosureVar(true)
	c.Defn = n.Canonical()
	c.Outer = n

	fn.ClosureVars = append(fn.ClosureVars, c)

	return c
}

// NewHiddenParam returns a new hidden parameter for fn with the given
// name and type.
func NewHiddenParam(pos src.XPos, fn *Func, sym *types.Sym, typ *types.Type) *Name {
	if fn.OClosure != nil {
		base.FatalfAt(fn.Pos(), "cannot add hidden parameters to closures")
	}

	fn.SetNeedctxt(true)

	// Create a fake parameter, disassociated from any real function, to
	// pretend to capture.
	fake := NewNameAt(pos, sym, typ)
	fake.Class = PPARAM
	fake.SetByval(true)

	return NewClosureVar(pos, fn, fake)
}

// SameSource reports whether two nodes refer to the same source
// element.
//
// It exists to help incrementally migrate the compiler towards
// allowing the introduction of IdentExpr (#42990). Once we have
// IdentExpr, it will no longer be safe to directly compare Node
// values to tell if they refer to the same Name. Instead, code will
// need to explicitly get references to the underlying Name object(s),
// and compare those instead.
//
// It will still be safe to compare Nodes directly for checking if two
// nodes are syntactically the same. The SameSource function exists to
// indicate code that intentionally compares Nodes for syntactic
// equality as opposed to code that has yet to be updated in
// preparation for IdentExpr.
func SameSource(n1, n2 Node) bool {
	return n1 == n2
}

// Uses reports whether expression x is a (direct) use of the given
// variable.
func Uses(x Node, v *Name) bool {
	if v == nil || v.Op() != ONAME {
		base.Fatalf("RefersTo bad Name: %v", v)
	}
	return x.Op() == ONAME && x.Name() == v
}

// DeclaredBy reports whether expression x refers (directly) to a
// variable that was declared by the given statement.
func DeclaredBy(x, stmt Node) bool {
	if stmt == nil {
		base.Fatalf("DeclaredBy nil")
	}
	return x.Op() == ONAME && SameSource(x.Name().Defn, stmt)
}

// The Class of a variable/function describes the "storage class"
// of a variable or function. During parsing, storage classes are
// called declaration contexts.
type Class uint8

//go:generate stringer -type=Class name.go
const (
	Pxxx       Class = iota // no class; used during ssa conversion to indicate pseudo-variables
	PEXTERN                 // global variables
	PAUTO                   // local variables
	PAUTOHEAP               // local variables or parameters moved to heap
	PPARAM                  // input arguments
	PPARAMOUT               // output results
	PTYPEPARAM              // type params
	PFUNC                   // global functions

	// Careful: Class is stored in three bits in Node.flags.
	_ = uint((1 << 3) - iota) // static assert for iota <= (1 << 3)
)

type Embed struct {
	Pos      src.XPos
	Patterns []string
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/node.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// “Abstract” syntax representation.

package ir

import (
	"fmt"
	"go/constant"

	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// A Node is the abstract interface to an IR node.
type Node interface {
	// Formatting
	// For debugging output, use one of
	//  Dump/FDump/DumpList/FDumplist (in fmt.go)
	//  DumpAny/FDumpAny (in dump.go)
	Format(s fmt.State, verb rune)

	// Source position.
	Pos() src.XPos
	SetPos(x src.XPos)

	// For making copies. For Copy and SepCopy.
	copy() Node

	doChildren(func(Node) bool) bool
	doChildrenWithHidden(func(Node) bool) bool
	editChildren(func(Node) Node)
	editChildrenWithHidden(func(Node) Node)

	// Abstract graph structure, for generic traversals.
	Op() Op
	Init() Nodes

	// Fields specific to certain Ops only.
	Type() *types.Type
	SetType(t *types.Type)
	Name() *Name
	Sym() *types.Sym
	Val() constant.Value
	SetVal(v constant.Value)

	// Storage for analysis passes.
	Esc() uint16
	SetEsc(x uint16)

	// Typecheck values:
	//  0 means the node is not typechecked
	//  1 means the node is completely typechecked
	//  2 means typechecking of the node is in progress
	Typecheck() uint8
	SetTypecheck(x uint8)
	NonNil() bool
	MarkNonNil()
}

// Line returns n's position as a string. If n has been inlined,
// it uses the outermost position where n has been inlined.
func Line(n Node) string {
	return base.FmtPos(n.Pos())
}

func IsSynthetic(n Node) bool {
	name := n.Sym().Name
	return name[0] == '.' || name[0] == '~'
}

// IsAutoTmp indicates if n was created by the compiler as a temporary,
// based on the setting of the .AutoTemp flag in n's Name.
func IsAutoTmp(n Node) bool {
	if n == nil || n.Op() != ONAME {
		return false
	}
	return n.Name().AutoTemp()
}

// MayBeShared reports whether n may occur in multiple places in the AST.
// Extra care must be taken when mutating such a node.
func MayBeShared(n Node) bool {
	switch n.Op() {
	case ONAME, OLITERAL, ONIL, OTYPE:
		return true
	}
	return false
}

type InitNode interface {
	Node
	PtrInit() *Nodes
	SetInit(x Nodes)
}

func TakeInit(n Node) Nodes {
	init := n.Init()
	if len(init) != 0 {
		n.(InitNode).SetInit(nil)
	}
	return init
}

//go:generate stringer -type=Op -trimprefix=O node.go

type Op uint8

// Node ops.
const (
	OXXX Op = iota

	// names
	ONAME // var or func name
	// Unnamed arg or return value: f(int, string) (int, error) { etc }
	// Also used for a qualified package identifier that hasn't been resolved yet.
	ONONAME
	OTYPE    // type name
	OLITERAL // literal
	ONIL     // nil

	// expressions
	OADD          // X + Y
	OSUB          // X - Y
	OOR           // X | Y
	OXOR          // X ^ Y
	OADDSTR       // +{List} (string addition, list elements are strings)
	OADDR         // &X
	OANDAND       // X && Y
	OAPPEND       // append(Args); after walk, X may contain elem type descriptor
	OBYTES2STR    // Type(X) (Type is string, X is a []byte)
	OBYTES2STRTMP // Type(X) (Type is string, X is a []byte, ephemeral)
	ORUNES2STR    // Type(X) (Type is string, X is a []rune)
	OSTR2BYTES    // Type(X) (Type is []byte, X is a string)
	OSTR2BYTESTMP // Type(X) (Type is []byte, X is a string, ephemeral)
	OSTR2RUNES    // Type(X) (Type is []rune, X is a string)
	OSLICE2ARR    // Type(X) (Type is [N]T, X is a []T)
	OSLICE2ARRPTR // Type(X) (Type is *[N]T, X is a []T)
	// X = Y or (if Def=true) X := Y
	// If Def, then Init includes a DCL node for X.
	OAS
	// Lhs = Rhs (x, y, z = a, b, c) or (if Def=true) Lhs := Rhs
	// If Def, then Init includes DCL nodes for Lhs
	OAS2
	OAS2DOTTYPE // Lhs = Rhs (x, ok = I.(int))
	OAS2FUNC    // Lhs = Rhs (x, y = f())
	OAS2MAPR    // Lhs = Rhs (x, ok = m["foo"])
	OAS2RECV    // Lhs = Rhs (x, ok = <-c)
	OASOP       // X AsOp= Y (x += y)
	OCALL       // X(Args) (function call, method call or type conversion)

	// OCALLFUNC, OCALLMETH, and OCALLINTER have the same structure.
	// Prior to walk, they are: X(Args), where Args is all regular arguments.
	// After walk, if any argument whose evaluation might requires temporary variable,
	// that temporary variable will be pushed to Init, Args will contain an updated
	// set of arguments.
	OCALLFUNC  // X(Args) (function call f(args))
	OCALLMETH  // X(Args) (direct method call x.Method(args))
	OCALLINTER // X(Args) (interface method call x.Method(args))
	OCAP       // cap(X)
	OCLEAR     // clear(X)
	OCLOSE     // close(X)
	OCLOSURE   // func Type { Func.Closure.Body } (func literal)
	OCOMPLIT   // Type{List} (composite literal, not yet lowered to specific form)
	OMAPLIT    // Type{List} (composite literal, Type is map)
	OSTRUCTLIT // Type{List} (composite literal, Type is struct)
	OARRAYLIT  // Type{List} (composite literal, Type is array)
	OSLICELIT  // Type{List} (composite literal, Type is slice), Len is slice length.
	OPTRLIT    // &X (X is composite literal)
	OCONV      // Type(X) (type conversion)
	OCONVIFACE // Type(X) (type conversion, to interface)
	OCONVNOP   // Type(X) (type conversion, no effect)
	OCOPY      // copy(X, Y)
	ODCL       // var X (declares X of type X.Type)

	// Used during parsing but don't last.
	ODCLFUNC // func f() or func (r) f()

	ODELETE        // delete(Args)
	ODOT           // X.Sel (X is of struct type)
	ODOTPTR        // X.Sel (X is of pointer to struct type)
	ODOTMETH       // X.Sel (X is non-interface, Sel is method name)
	ODOTINTER      // X.Sel (X is interface, Sel is method name)
	OXDOT          // X.Sel (before rewrite to one of the preceding)
	ODOTTYPE       // X.Ntype or X.Type (.Ntype during parsing, .Type once resolved); after walk, Itab contains address of interface type descriptor and Itab.X contains address of concrete type descriptor
	ODOTTYPE2      // X.Ntype or X.Type (.Ntype during parsing, .Type once resolved; on rhs of OAS2DOTTYPE); after walk, Itab contains address of interface type descriptor
	OEQ            // X == Y
	ONE            // X != Y
	OLT            // X < Y
	OLE            // X <= Y
	OGE            // X >= Y
	OGT            // X > Y
	ODEREF         // *X
	OINDEX         // X[Index] (index of array or slice)
	OINDEXMAP      // X[Index] (index of map)
	OKEY           // Key:Value (key:value in struct/array/map literal)
	OSTRUCTKEY     // Field:Value (key:value in struct literal, after type checking)
	OLEN           // len(X)
	OMAKE          // make(Args) (before type checking converts to one of the following)
	OMAKECHAN      // make(Type[, Len]) (type is chan)
	OMAKEMAP       // make(Type[, Len]) (type is map)
	OMAKESLICE     // make(Type[, Len[, Cap]]) (type is slice)
	OMAKESLICECOPY // makeslicecopy(Type, Len, Cap) (type is slice; Len is length and Cap is the copied from slice)
	// OMAKESLICECOPY is created by the order pass and corresponds to:
	//  s = make(Type, Len); copy(s, Cap)
	//
	// Bounded can be set on the node when Len == len(Cap) is known at compile time.
	//
	// This node is created so the walk pass can optimize this pattern which would
	// otherwise be hard to detect after the order pass.
	OMUL              // X * Y
	ODIV              // X / Y
	OMOD              // X % Y
	OLSH              // X << Y
	ORSH              // X >> Y
	OAND              // X & Y
	OANDNOT           // X &^ Y
	ONEW              // new(X); corresponds to calls to new(T) in source code
	ONOT              // !X
	OBITNOT           // ^X
	OPLUS             // +X
	ONEG              // -X
	OOROR             // X || Y
	OPANIC            // panic(X)
	OPRINT            // print(List)
	OPRINTLN          // println(List)
	OPAREN            // (X)
	OSEND             // Chan <- Value
	OSLICE            // X[Low : High] (X is untypechecked or slice)
	OSLICEARR         // X[Low : High] (X is pointer to array)
	OSLICESTR         // X[Low : High] (X is string)
	OSLICE3           // X[Low : High : Max] (X is untypedchecked or slice)
	OSLICE3ARR        // X[Low : High : Max] (X is pointer to array)
	OSLICEHEADER      // sliceheader{Ptr, Len, Cap} (Ptr is unsafe.Pointer, Len is length, Cap is capacity)
	OSTRINGHEADER     // stringheader{Ptr, Len} (Ptr is unsafe.Pointer, Len is length)
	ORECOVER          // recover()
	ORECV             // <-X
	ORUNESTR          // Type(X) (Type is string, X is rune)
	OSELRECV2         // like OAS2: Lhs = Rhs where len(Lhs)=2, len(Rhs)=1, Rhs[0].Op = ORECV (appears as .Var of OCASE)
	OMIN              // min(List)
	OMAX              // max(List)
	OREAL             // real(X)
	OIMAG             // imag(X)
	OCOMPLEX          // complex(X, Y)
	OUNSAFEADD        // unsafe.Add(X, Y)
	OUNSAFESLICE      // unsafe.Slice(X, Y)
	OUNSAFESLICEDATA  // unsafe.SliceData(X)
	OUNSAFESTRING     // unsafe.String(X, Y)
	OUNSAFESTRINGDATA // unsafe.StringData(X)
	OMETHEXPR         // X(Args) (method expression T.Method(args), first argument is the method receiver)
	OMETHVALUE        // X.Sel   (method expression t.Method, not called)

	// statements
	OBLOCK // { List } (block of code)
	OBREAK // break [Label]
	// OCASE:  case List: Body (List==nil means default)
	//   For OTYPESW, List is a OTYPE node for the specified type (or OLITERAL
	//   for nil) or an ODYNAMICTYPE indicating a runtime type for generics.
	//   If a type-switch variable is specified, Var is an
	//   ONAME for the version of the type-switch variable with the specified
	//   type.
	OCASE
	OCONTINUE // continue [Label]
	ODEFER    // defer Call
	OFALL     // fallthrough
	OFOR      // for Init; Cond; Post { Body }
	OGOTO     // goto Label
	OIF       // if Init; Cond { Then } else { Else }
	OLABEL    // Label:
	OGO       // go Call
	ORANGE    // for Key, Value = range X { Body }
	ORETURN   // return Results
	OSELECT   // select { Cases }
	OSWITCH   // switch Init; Expr { Cases }
	// OTYPESW:  X := Y.(type) (appears as .Tag of OSWITCH)
	//   X is nil if there is no type-switch variable
	OTYPESW

	// misc
	// intermediate representation of an inlined call.  Uses Init (assignments
	// for the captured variables, parameters, retvars, & INLMARK op),
	// Body (body of the inlined function), and ReturnVars (list of
	// return values)
	OINLCALL         // intermediary representation of an inlined call.
	OMAKEFACE        // construct an interface value from rtype/itab and data pointers
	OITAB            // rtype/itab pointer of an interface value
	OIDATA           // data pointer of an interface value
	OSPTR            // base pointer of a slice or string. Bounded==1 means known non-nil.
	OCFUNC           // reference to c function pointer (not go func value)
	OCHECKNIL        // emit code to ensure pointer/interface not nil
	ORESULT          // result of a function call; Xoffset is stack offset
	OINLMARK         // start of an inlined body, with file/line of caller. Xoffset is an index into the inline tree.
	OLINKSYMOFFSET   // offset within a name
	OJUMPTABLE       // A jump table structure for implementing dense expression switches
	OINTERFACESWITCH // A type switch with interface cases
	OMOVE2HEAP       // Promote a stack-backed slice to heap

	// opcodes for generics
	ODYNAMICDOTTYPE  // x = i.(T) where T is a type parameter (or derived from a type parameter)
	ODYNAMICDOTTYPE2 // x, ok = i.(T) where T is a type parameter (or derived from a type parameter)
	ODYNAMICTYPE     // a type node for type switches (represents a dynamic target type for a type switch)

	// arch-specific opcodes
	OTAILCALL    // tail call to another function
	OGETG        // runtime.getg() (read g pointer)
	OGETCALLERSP // internal/runtime/sys.GetCallerSP() (stack pointer in caller frame)

	OEND
)

// IsCmp reports whether op is a comparison operation (==, !=, <, <=,
// >, or >=).
func (op Op) IsCmp() bool {
	switch op {
	case OEQ, ONE, OLT, OLE, OGT, OGE:
		return true
	}
	return false
}

// Nodes is a slice of Node.
type Nodes []Node

// ToNodes returns s as a slice of Nodes.
func ToNodes[T Node](s []T) Nodes {
	res := make(Nodes, len(s))
	for i, n := range s {
		res[i] = n
	}
	return res
}

// Append appends entries to Nodes.
func (n *Nodes) Append(a ...Node) {
	if len(a) == 0 {
		return
	}
	*n = append(*n, a...)
}

// Prepend prepends entries to Nodes.
// If a slice is passed in, this will take ownership of it.
func (n *Nodes) Prepend(a ...Node) {
	if len(a) == 0 {
		return
	}
	*n = append(a, *n...)
}

// Take clears n, returning its former contents.
func (n *Nodes) Take() []Node {
	ret := *n
	*n = nil
	return ret
}

// Copy returns a copy of the content of the slice.
func (n Nodes) Copy() Nodes {
	if n == nil {
		return nil
	}
	c := make(Nodes, len(n))
	copy(c, n)
	return c
}

// NameQueue is a FIFO queue of *Name. The zero value of NameQueue is
// a ready-to-use empty queue.
type NameQueue struct {
	ring       []*Name
	head, tail int
}

// Empty reports whether q contains no Names.
func (q *NameQueue) Empty() bool {
	return q.head == q.tail
}

// PushRight appends n to the right of the queue.
func (q *NameQueue) PushRight(n *Name) {
	if len(q.ring) == 0 {
		q.ring = make([]*Name, 16)
	} else if q.head+len(q.ring) == q.tail {
		// Grow the ring.
		nring := make([]*Name, len(q.ring)*2)
		// Copy the old elements.
		part := q.ring[q.head%len(q.ring):]
		if q.tail-q.head <= len(part) {
			part = part[:q.tail-q.head]
			copy(nring, part)
		} else {
			pos := copy(nring, part)
			copy(nring[pos:], q.ring[:q.tail%len(q.ring)])
		}
		q.ring, q.head, q.tail = nring, 0, q.tail-q.head
	}

	q.ring[q.tail%len(q.ring)] = n
	q.tail++
}

// PopLeft pops a Name from the left of the queue. It panics if q is
// empty.
func (q *NameQueue) PopLeft() *Name {
	if q.Empty() {
		panic("dequeue empty")
	}
	n := q.ring[q.head%len(q.ring)]
	q.head++
	return n
}

// NameSet is a set of Names.
type NameSet map[*Name]struct{}

// Has reports whether s contains n.
func (s NameSet) Has(n *Name) bool {
	_, isPresent := s[n]
	return isPresent
}

// Add adds n to s.
func (s *NameSet) Add(n *Name) {
	if *s == nil {
		*s = make(map[*Name]struct{})
	}
	(*s)[n] = struct{}{}
}

type PragmaFlag uint16

const (
	// Func pragmas.
	Nointerface      PragmaFlag = 1 << iota
	Noescape                    // func parameters don't escape
	Norace                      // func must not have race detector annotations
	Nosplit                     // func should not execute on separate stack
	Noinline                    // func should not be inlined
	NoCheckPtr                  // func should not be instrumented by checkptr
	CgoUnsafeArgs               // treat a pointer to one arg as a pointer to them all
	UintptrKeepAlive            // pointers converted to uintptr must be kept alive
	UintptrEscapes              // pointers converted to uintptr escape

	// Runtime-only func pragmas.
	// See ../../../../runtime/HACKING.md for detailed descriptions.
	Systemstack        // func must run on system stack
	Nowritebarrier     // emit compiler error instead of write barrier
	Nowritebarrierrec  // error on write barrier in this or recursive callees
	Yeswritebarrierrec // cancels Nowritebarrierrec in this function and callees

	// Go command pragmas
	GoBuildPragma

	RegisterParams // TODO(register args) remove after register abi is working

)

var BlankNode *Name

func IsConst(n Node, ct constant.Kind) bool {
	return ConstType(n) == ct
}

// IsNil reports whether n represents the universal untyped zero value "nil".
func IsNil(n Node) bool {
	return n != nil && n.Op() == ONIL
}

func IsBlank(n Node) bool {
	if n == nil {
		return false
	}
	return n.Sym().IsBlank()
}

// IsMethod reports whether n is a method.
// n must be a function or a method.
func IsMethod(n Node) bool {
	return n.Type().Recv() != nil
}

// HasUniquePos reports whether n has a unique position that can be
// used for reporting error messages.
//
// It's primarily used to distinguish references to named objects,
// whose Pos will point back to their declaration position rather than
// their usage position.
func HasUniquePos(n Node) bool {
	switch n.Op() {
	case ONAME:
		return false
	case OLITERAL, ONIL, OTYPE:
		if n.Sym() != nil {
			return false
		}
	}

	if !n.Pos().IsKnown() {
		if base.Flag.K != 0 {
			base.Warn("setlineno: unknown position (line 0)")
		}
		return false
	}

	return true
}

func SetPos(n Node) src.XPos {
	lno := base.Pos
	if n != nil && HasUniquePos(n) {
		base.Pos = n.Pos()
	}
	return lno
}

// The result of InitExpr MUST be assigned back to n, e.g.
//
//	n.X = InitExpr(init, n.X)
func InitExpr(init []Node, expr Node) Node {
	if len(init) == 0 {
		return expr
	}

	n, ok := expr.(InitNode)
	if !ok || MayBeShared(n) {
		// Introduce OCONVNOP to hold init list.
		n = NewConvExpr(base.Pos, OCONVNOP, nil, expr)
		n.SetType(expr.Type())
		n.SetTypecheck(1)
	}

	n.PtrInit().Prepend(init...)
	return n
}

// what's the outer value that a write to n affects?
// outer value means containing struct or array.
func OuterValue(n Node) Node {
	for {
		switch nn := n; nn.Op() {
		case OXDOT:
			base.FatalfAt(n.Pos(), "OXDOT in OuterValue: %v", n)
		case ODOT:
			nn := nn.(*SelectorExpr)
			n = nn.X
			continue
		case OPAREN:
			nn := nn.(*ParenExpr)
			n = nn.X
			continue
		case OCONVNOP:
			nn := nn.(*ConvExpr)
			n = nn.X
			continue
		case OINDEX:
			nn := nn.(*IndexExpr)
			if nn.X.Type() == nil {
				base.Fatalf("OuterValue needs type for %v", nn.X)
			}
			if nn.X.Type().IsArray() {
				n = nn.X
				continue
			}
		}

		return n
	}
}

const (
	EscUnknown = iota
	EscNone    // Does not escape to heap, result, or parameters.
	EscHeap    // Reachable from the heap
	EscNever   // By construction will not escape.
)

```

// === FILE: references/go/src/cmd/compile/internal/ir/op_string.go ===
```go
// Code generated by "stringer -type=Op -trimprefix=O node.go"; DO NOT EDIT.

package ir

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[OXXX-0]
	_ = x[ONAME-1]
	_ = x[ONONAME-2]
	_ = x[OTYPE-3]
	_ = x[OLITERAL-4]
	_ = x[ONIL-5]
	_ = x[OADD-6]
	_ = x[OSUB-7]
	_ = x[OOR-8]
	_ = x[OXOR-9]
	_ = x[OADDSTR-10]
	_ = x[OADDR-11]
	_ = x[OANDAND-12]
	_ = x[OAPPEND-13]
	_ = x[OBYTES2STR-14]
	_ = x[OBYTES2STRTMP-15]
	_ = x[ORUNES2STR-16]
	_ = x[OSTR2BYTES-17]
	_ = x[OSTR2BYTESTMP-18]
	_ = x[OSTR2RUNES-19]
	_ = x[OSLICE2ARR-20]
	_ = x[OSLICE2ARRPTR-21]
	_ = x[OAS-22]
	_ = x[OAS2-23]
	_ = x[OAS2DOTTYPE-24]
	_ = x[OAS2FUNC-25]
	_ = x[OAS2MAPR-26]
	_ = x[OAS2RECV-27]
	_ = x[OASOP-28]
	_ = x[OCALL-29]
	_ = x[OCALLFUNC-30]
	_ = x[OCALLMETH-31]
	_ = x[OCALLINTER-32]
	_ = x[OCAP-33]
	_ = x[OCLEAR-34]
	_ = x[OCLOSE-35]
	_ = x[OCLOSURE-36]
	_ = x[OCOMPLIT-37]
	_ = x[OMAPLIT-38]
	_ = x[OSTRUCTLIT-39]
	_ = x[OARRAYLIT-40]
	_ = x[OSLICELIT-41]
	_ = x[OPTRLIT-42]
	_ = x[OCONV-43]
	_ = x[OCONVIFACE-44]
	_ = x[OCONVNOP-45]
	_ = x[OCOPY-46]
	_ = x[ODCL-47]
	_ = x[ODCLFUNC-48]
	_ = x[ODELETE-49]
	_ = x[ODOT-50]
	_ = x[ODOTPTR-51]
	_ = x[ODOTMETH-52]
	_ = x[ODOTINTER-53]
	_ = x[OXDOT-54]
	_ = x[ODOTTYPE-55]
	_ = x[ODOTTYPE2-56]
	_ = x[OEQ-57]
	_ = x[ONE-58]
	_ = x[OLT-59]
	_ = x[OLE-60]
	_ = x[OGE-61]
	_ = x[OGT-62]
	_ = x[ODEREF-63]
	_ = x[OINDEX-64]
	_ = x[OINDEXMAP-65]
	_ = x[OKEY-66]
	_ = x[OSTRUCTKEY-67]
	_ = x[OLEN-68]
	_ = x[OMAKE-69]
	_ = x[OMAKECHAN-70]
	_ = x[OMAKEMAP-71]
	_ = x[OMAKESLICE-72]
	_ = x[OMAKESLICECOPY-73]
	_ = x[OMUL-74]
	_ = x[ODIV-75]
	_ = x[OMOD-76]
	_ = x[OLSH-77]
	_ = x[ORSH-78]
	_ = x[OAND-79]
	_ = x[OANDNOT-80]
	_ = x[ONEW-81]
	_ = x[ONOT-82]
	_ = x[OBITNOT-83]
	_ = x[OPLUS-84]
	_ = x[ONEG-85]
	_ = x[OOROR-86]
	_ = x[OPANIC-87]
	_ = x[OPRINT-88]
	_ = x[OPRINTLN-89]
	_ = x[OPAREN-90]
	_ = x[OSEND-91]
	_ = x[OSLICE-92]
	_ = x[OSLICEARR-93]
	_ = x[OSLICESTR-94]
	_ = x[OSLICE3-95]
	_ = x[OSLICE3ARR-96]
	_ = x[OSLICEHEADER-97]
	_ = x[OSTRINGHEADER-98]
	_ = x[ORECOVER-99]
	_ = x[ORECV-100]
	_ = x[ORUNESTR-101]
	_ = x[OSELRECV2-102]
	_ = x[OMIN-103]
	_ = x[OMAX-104]
	_ = x[OREAL-105]
	_ = x[OIMAG-106]
	_ = x[OCOMPLEX-107]
	_ = x[OUNSAFEADD-108]
	_ = x[OUNSAFESLICE-109]
	_ = x[OUNSAFESLICEDATA-110]
	_ = x[OUNSAFESTRING-111]
	_ = x[OUNSAFESTRINGDATA-112]
	_ = x[OMETHEXPR-113]
	_ = x[OMETHVALUE-114]
	_ = x[OBLOCK-115]
	_ = x[OBREAK-116]
	_ = x[OCASE-117]
	_ = x[OCONTINUE-118]
	_ = x[ODEFER-119]
	_ = x[OFALL-120]
	_ = x[OFOR-121]
	_ = x[OGOTO-122]
	_ = x[OIF-123]
	_ = x[OLABEL-124]
	_ = x[OGO-125]
	_ = x[ORANGE-126]
	_ = x[ORETURN-127]
	_ = x[OSELECT-128]
	_ = x[OSWITCH-129]
	_ = x[OTYPESW-130]
	_ = x[OINLCALL-131]
	_ = x[OMAKEFACE-132]
	_ = x[OITAB-133]
	_ = x[OIDATA-134]
	_ = x[OSPTR-135]
	_ = x[OCFUNC-136]
	_ = x[OCHECKNIL-137]
	_ = x[ORESULT-138]
	_ = x[OINLMARK-139]
	_ = x[OLINKSYMOFFSET-140]
	_ = x[OJUMPTABLE-141]
	_ = x[OINTERFACESWITCH-142]
	_ = x[OMOVE2HEAP-143]
	_ = x[ODYNAMICDOTTYPE-144]
	_ = x[ODYNAMICDOTTYPE2-145]
	_ = x[ODYNAMICTYPE-146]
	_ = x[OTAILCALL-147]
	_ = x[OGETG-148]
	_ = x[OGETCALLERSP-149]
	_ = x[OEND-150]
}

const _Op_name = "XXXNAMENONAMETYPELITERALNILADDSUBORXORADDSTRADDRANDANDAPPENDBYTES2STRBYTES2STRTMPRUNES2STRSTR2BYTESSTR2BYTESTMPSTR2RUNESSLICE2ARRSLICE2ARRPTRASAS2AS2DOTTYPEAS2FUNCAS2MAPRAS2RECVASOPCALLCALLFUNCCALLMETHCALLINTERCAPCLEARCLOSECLOSURECOMPLITMAPLITSTRUCTLITARRAYLITSLICELITPTRLITCONVCONVIFACECONVNOPCOPYDCLDCLFUNCDELETEDOTDOTPTRDOTMETHDOTINTERXDOTDOTTYPEDOTTYPE2EQNELTLEGEGTDEREFINDEXINDEXMAPKEYSTRUCTKEYLENMAKEMAKECHANMAKEMAPMAKESLICEMAKESLICECOPYMULDIVMODLSHRSHANDANDNOTNEWNOTBITNOTPLUSNEGORORPANICPRINTPRINTLNPARENSENDSLICESLICEARRSLICESTRSLICE3SLICE3ARRSLICEHEADERSTRINGHEADERRECOVERRECVRUNESTRSELRECV2MINMAXREALIMAGCOMPLEXUNSAFEADDUNSAFESLICEUNSAFESLICEDATAUNSAFESTRINGUNSAFESTRINGDATAMETHEXPRMETHVALUEBLOCKBREAKCASECONTINUEDEFERFALLFORGOTOIFLABELGORANGERETURNSELECTSWITCHTYPESWINLCALLMAKEFACEITABIDATASPTRCFUNCCHECKNILRESULTINLMARKLINKSYMOFFSETJUMPTABLEINTERFACESWITCHMOVE2HEAPDYNAMICDOTTYPEDYNAMICDOTTYPE2DYNAMICTYPETAILCALLGETGGETCALLERSPEND"

var _Op_index = [...]uint16{0, 3, 7, 13, 17, 24, 27, 30, 33, 35, 38, 44, 48, 54, 60, 69, 81, 90, 99, 111, 120, 129, 141, 143, 146, 156, 163, 170, 177, 181, 185, 193, 201, 210, 213, 218, 223, 230, 237, 243, 252, 260, 268, 274, 278, 287, 294, 298, 301, 308, 314, 317, 323, 330, 338, 342, 349, 357, 359, 361, 363, 365, 367, 369, 374, 379, 387, 390, 399, 402, 406, 414, 421, 430, 443, 446, 449, 452, 455, 458, 461, 467, 470, 473, 479, 483, 486, 490, 495, 500, 507, 512, 516, 521, 529, 537, 543, 552, 563, 575, 582, 586, 593, 601, 604, 607, 611, 615, 622, 631, 642, 657, 669, 685, 693, 702, 707, 712, 716, 724, 729, 733, 736, 740, 742, 747, 749, 754, 760, 766, 772, 778, 785, 793, 797, 802, 806, 811, 819, 825, 832, 845, 854, 869, 878, 892, 907, 918, 926, 930, 941, 944}

func (i Op) String() string {
	idx := int(i) - 0
	if i < 0 || idx >= len(_Op_index)-1 {
		return "Op(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Op_name[_Op_index[idx]:_Op_index[idx+1]]
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/package.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import "cmd/compile/internal/types"

// A Package holds information about the package being compiled.
type Package struct {
	// Imports, listed in source order.
	// See golang.org/issue/31636.
	Imports []*types.Pkg

	// Init functions, listed in source order.
	Inits []*Func

	// Funcs contains all (instantiated) functions, methods, and
	// function literals to be compiled.
	Funcs []*Func

	// Externs holds constants, (non-generic) types, and variables
	// declared at package scope.
	Externs []*Name

	// AsmHdrDecls holds declared constants and struct types that should
	// be included in -asmhdr output. It's only populated when -asmhdr
	// is set.
	AsmHdrDecls []*Name

	// Cgo directives.
	CgoPragmas [][]string

	// Variables with //go:embed lines.
	Embeds []*Name

	// PluginExports holds exported functions and variables that are
	// accessible through the package plugin API. It's only populated
	// for -buildmode=plugin (i.e., compiling package main and -dynlink
	// is set).
	PluginExports []*Name
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/reassign_consistency_check.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"cmd/compile/internal/base"
	"cmd/internal/src"
	"fmt"
	"path/filepath"
	"strings"
)

// checkStaticValueResult compares the result from ReassignOracle.StaticValue
// with the corresponding result from ir.StaticValue to make sure they agree.
// This method is called only when turned on via build tag.
func checkStaticValueResult(n Node, newres Node) {
	oldres := StaticValue(n)
	if oldres != newres {
		base.Fatalf("%s: new/old static value disagreement on %v:\nnew=%v\nold=%v", fmtFullPos(n.Pos()), n, newres, oldres)
	}
}

// checkReassignedResult compares the result from ReassignOracle.Reassigned
// with the corresponding result from ir.Reassigned to make sure they agree.
// This method is called only when turned on via build tag.
func checkReassignedResult(n *Name, newres bool) {
	origres := Reassigned(n)
	if newres != origres {
		base.Fatalf("%s: new/old reassigned disagreement on %v (class %s) newres=%v oldres=%v", fmtFullPos(n.Pos()), n, n.Class.String(), newres, origres)
	}
}

// fmtFullPos returns a verbose dump for pos p, including inlines.
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

```

// === FILE: references/go/src/cmd/compile/internal/ir/reassignment.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
)

// A ReassignOracle efficiently answers queries about whether local
// variables are reassigned. This helper works by looking for function
// params and short variable declarations (e.g.
// https://go.dev/ref/spec#Short_variable_declarations) that are
// neither address taken nor subsequently re-assigned. It is intended
// to operate much like "ir.StaticValue" and "ir.Reassigned", but in a
// way that does just a single walk of the containing function (as
// opposed to a new walk on every call).
type ReassignOracle struct {
	fn *Func
	// maps candidate name to its defining assignment (or
	// for params, defining func).
	singleDef map[*Name]Node

	// funcAssigns tracks all known simple assignments (OAS) to
	// func-typed PAUTO variables. Only func-typed variables are
	// tracked because this data is used exclusively for callee
	// resolution in escape analysis. Deletion means the candidate was
	// invalidated (e.g., addr-taken, non-simple assignment form, or too
	// many assignments). Assignments inside nested closures are accepted
	// because the only alternative value is nil, which panics on call.
	funcAssigns map[*Name][]*AssignStmt
}

// Init initializes the oracle based on the IR in function fn, laying
// the groundwork for future calls to the StaticValue and Reassigned
// methods. If the fn's IR is subsequently modified, Init must be
// called again.
func (ro *ReassignOracle) Init(fn *Func) {
	ro.fn = fn

	// Collect candidate map. Start by adding function parameters
	// explicitly.
	ro.singleDef = make(map[*Name]Node)
	ro.funcAssigns = make(map[*Name][]*AssignStmt)
	sig := fn.Type()
	numParams := sig.NumRecvs() + sig.NumParams()
	for _, param := range fn.Dcl[:numParams] {
		if IsBlank(param) {
			continue
		}
		// For params, use func itself as defining node.
		ro.singleDef[param] = fn
	}

	// Walk the function body to discover any locals assigned
	// via ":=" syntax (e.g. "a := <expr>").
	var findLocals func(n Node) bool
	findLocals = func(n Node) bool {
		if nn, ok := n.(*Name); ok {
			if nn.Class == PAUTO && !nn.Addrtaken() {
				isFunc := nn.Type().Kind() == types.TFUNC
				if nn.Defn == nil {
					// Bare declaration (e.g., "var f func()").
					if isFunc {
						ro.funcAssigns[nn] = nil
					}
				} else if _, ok := nn.Defn.(*AssignStmt); ok {
					ro.singleDef[nn] = nn.Defn
					if isFunc {
						ro.funcAssigns[nn] = nil
					}
				} else {
					ro.singleDef[nn] = nn.Defn
				}
			}
		} else if nn, ok := n.(*ClosureExpr); ok {
			Any(nn.Func, findLocals)
		}
		return false
	}
	Any(fn, findLocals)

	outerName := func(x Node) *Name {
		if x == nil {
			return nil
		}
		n, ok := OuterValue(x).(*Name)
		if ok {
			return n.Canonical()
		}
		return nil
	}

	// pruneIfNeeded examines node nn appearing on the left hand side
	// of assignment statement asn to see if it contains a reassignment
	// to any nodes in our candidate maps; if a reassignment is found,
	// the corresponding name is deleted.
	pruneIfNeeded := func(nn Node, asn Node) {
		oname := outerName(nn)
		if oname == nil {
			return
		}
		if defn, ok := ro.singleDef[oname]; ok {
			// any assignment to a param invalidates the entry.
			paramAssigned := oname.Class == PPARAM
			// assignment to local ok iff assignment is its orig def.
			localAssigned := (oname.Class == PAUTO && asn != defn)
			if paramAssigned || localAssigned {
				// We found an assignment to name N that doesn't
				// correspond to its original definition; remove
				// from candidates.
				delete(ro.singleDef, oname)
			}
		}
		if _, ok := ro.funcAssigns[oname]; ok {
			as, isOAS := asn.(*AssignStmt)
			if isOAS && isNilAssign(as) {
				// Zero-value assignment (nil, bare decl), skip.
			} else if !isOAS {
				// Not a simple assignment: invalidate.
				delete(ro.funcAssigns, oname)
			} else {
				ro.funcAssigns[oname] = append(ro.funcAssigns[oname], as)
			}
		}
	}

	// Prune away anything that looks assigned. This code modeled after
	// similar code in ir.Reassigned; any changes there should be made
	// here as well.
	var do func(n Node) bool
	do = func(n Node) bool {
		switch n.Op() {
		case OAS:
			asn := n.(*AssignStmt)
			pruneIfNeeded(asn.X, n)
		case OAS2, OAS2FUNC, OAS2MAPR, OAS2DOTTYPE, OAS2RECV, OSELRECV2:
			asn := n.(*AssignListStmt)
			for _, p := range asn.Lhs {
				pruneIfNeeded(p, n)
			}
		case OASOP:
			asn := n.(*AssignOpStmt)
			pruneIfNeeded(asn.X, n)
		case ORANGE:
			rs := n.(*RangeStmt)
			pruneIfNeeded(rs.Key, n)
			pruneIfNeeded(rs.Value, n)
		case OCLOSURE:
			n := n.(*ClosureExpr)
			Any(n.Func, do)
		}
		return false
	}
	Any(fn, do)
}

// StaticValue method has the same semantics as the ir package function
// of the same name; see comments on [StaticValue].
func (ro *ReassignOracle) StaticValue(n Node) Node {
	arg := n
	for {
		if n.Op() == OCONVNOP {
			n = n.(*ConvExpr).X
			continue
		}

		if n.Op() == OINLCALL {
			n = n.(*InlinedCallExpr).SingleResult()
			continue
		}

		if n.Op() == OPAREN {
			n = n.(*ParenExpr).X
			continue
		}

		n1 := ro.staticValue1(n)
		if n1 == nil {
			if consistencyCheckEnabled {
				checkStaticValueResult(arg, n)
			}
			return n
		}
		n = n1
	}
}

func (ro *ReassignOracle) staticValue1(nn Node) Node {
	if nn.Op() != ONAME {
		return nil
	}
	n := nn.(*Name).Canonical()
	if n.Class != PAUTO {
		return nil
	}

	defn := n.Defn
	if defn == nil {
		return nil
	}

	var rhs Node
FindRHS:
	switch defn.Op() {
	case OAS:
		defn := defn.(*AssignStmt)
		rhs = defn.Y
	case OAS2:
		defn := defn.(*AssignListStmt)
		for i, lhs := range defn.Lhs {
			if lhs == n {
				rhs = defn.Rhs[i]
				break FindRHS
			}
		}
		base.FatalfAt(defn.Pos(), "%v missing from LHS of %v", n, defn)
	default:
		return nil
	}
	if rhs == nil {
		base.FatalfAt(defn.Pos(), "RHS is nil: %v", defn)
	}

	if _, ok := ro.singleDef[n]; !ok {
		return nil
	}

	return rhs
}

// Reassigned method has the same semantics as the ir package function
// of the same name; see comments on [Reassigned] for more info.
func (ro *ReassignOracle) Reassigned(n *Name) bool {
	_, ok := ro.singleDef[n]
	result := !ok
	if consistencyCheckEnabled {
		checkReassignedResult(n, result)
	}
	return result
}

// FuncAssignments returns all known simple assignments to a func-typed
// variable. For variables defined with := and a non-zero value, the
// defining assignment is included. Returns nil if the variable is not
// func-typed, was invalidated (addr-taken, non-simple assignment,
// too many assignments), or has no tracked assignments. Assignments
// inside nested closures are accepted because the only alternative
// value is nil, which panics on call.
func (ro *ReassignOracle) FuncAssignments(name *Name) []*AssignStmt {
	return ro.funcAssigns[name.Canonical()]
}

// isNilAssign reports whether as has a nil or absent RHS.
func isNilAssign(as *AssignStmt) bool {
	if as.Y == nil {
		return true
	}
	y := as.Y
	for y.Op() == OCONVNOP {
		y = y.(*ConvExpr).X
	}
	return IsNil(y)
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/scc.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

// Strongly connected components.
//
// Run analysis on minimal sets of mutually recursive functions
// or single non-recursive functions, bottom up.
//
// Finding these sets is finding strongly connected components
// by reverse topological order in the static call graph.
// The algorithm (known as Tarjan's algorithm) for doing that is taken from
// Sedgewick, Algorithms, Second Edition, p. 482, with two adaptations.
//
// First, a closure function (fn.IsClosure()) cannot be
// the root of a connected component. Refusing to use it as a root forces
// it into the component of the function in which it appears.  This is
// more convenient for escape analysis.
//
// Second, each function becomes two virtual nodes in the graph,
// with numbers n and n+1. We record the function's node number as n
// but search from node n+1. If the search tells us that the component
// number (minVisitGen) is n+1, we know that this is a trivial component: one function
// plus its closures. If the search tells us that the component number is
// n, then there was a path from node n+1 back to node n, meaning that
// the function set is mutually recursive. The escape analysis can be
// more precise when analyzing a single non-recursive function than
// when analyzing a set of mutually recursive functions.

type bottomUpVisitor struct {
	analyze  func([]*Func, bool)
	visitgen uint32
	nodeID   map[*Func]uint32
	stack    []*Func
}

// VisitFuncsBottomUp invokes analyze on the ODCLFUNC nodes listed in list.
// It calls analyze with successive groups of functions, working from
// the bottom of the call graph upward. Each time analyze is called with
// a list of functions, every function on that list only calls other functions
// on the list or functions that have been passed in previous invocations of
// analyze. Closures appear in the same list as their outer functions.
// The lists are as short as possible while preserving those requirements.
// (In a typical program, many invocations of analyze will be passed just
// a single function.) The boolean argument 'recursive' passed to analyze
// specifies whether the functions on the list are mutually recursive.
// If recursive is false, the list consists of only a single function and its closures.
// If recursive is true, the list may still contain only a single function,
// if that function is itself recursive.
func VisitFuncsBottomUp(list []*Func, analyze func(list []*Func, recursive bool)) {
	var v bottomUpVisitor
	v.analyze = analyze
	v.nodeID = make(map[*Func]uint32)
	for _, n := range list {
		if !n.IsClosure() {
			v.visit(n)
		}
	}
}

func (v *bottomUpVisitor) visit(n *Func) uint32 {
	if id := v.nodeID[n]; id > 0 {
		// already visited
		return id
	}

	v.visitgen++
	id := v.visitgen
	v.nodeID[n] = id
	v.visitgen++
	minVisitGen := v.visitgen
	v.stack = append(v.stack, n)

	do := func(defn Node) {
		if defn != nil {
			if m := v.visit(defn.(*Func)); m < minVisitGen {
				minVisitGen = m
			}
		}
	}

	Visit(n, func(n Node) {
		switch n.Op() {
		case ONAME:
			if n := n.(*Name); n.Class == PFUNC {
				do(n.Defn)
			}
		case ODOTMETH, OMETHVALUE, OMETHEXPR:
			if fn := MethodExprName(n); fn != nil {
				do(fn.Defn)
			}
		case OCLOSURE:
			n := n.(*ClosureExpr)
			do(n.Func)
		}
	})

	if (minVisitGen == id || minVisitGen == id+1) && !n.IsClosure() {
		// This node is the root of a strongly connected component.

		// The original minVisitGen was id+1. If the bottomUpVisitor found its way
		// back to id, then this block is a set of mutually recursive functions.
		// Otherwise, it's just a lone function that does not recurse.
		recursive := minVisitGen == id

		// Remove connected component from stack and mark v.nodeID so that future
		// visits return a large number, which will not affect the caller's min.
		var i int
		for i = len(v.stack) - 1; i >= 0; i-- {
			x := v.stack[i]
			v.nodeID[x] = ^uint32(0)
			if x == n {
				break
			}
		}
		block := v.stack[i:]
		// Call analyze on this set of functions.
		v.stack = v.stack[:i]
		v.analyze(block, recursive)
	}

	return minVisitGen
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/stmt.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/src"
	"go/constant"
)

// A Decl is a declaration of a const, type, or var. (A declared func is a Func.)
type Decl struct {
	miniNode
	X *Name // the thing being declared
}

func NewDecl(pos src.XPos, op Op, x *Name) *Decl {
	n := &Decl{X: x}
	n.pos = pos
	switch op {
	default:
		panic("invalid Decl op " + op.String())
	case ODCL:
		n.op = op
	}
	return n
}

func (*Decl) isStmt() {}

// A Stmt is a Node that can appear as a statement.
// This includes statement-like expressions such as f().
//
// (It's possible it should include <-c, but that would require
// splitting ORECV out of UnaryExpr, which hasn't yet been
// necessary. Maybe instead we will introduce ExprStmt at
// some point.)
type Stmt interface {
	Node
	isStmt()
	PtrInit() *Nodes
}

// A miniStmt is a miniNode with extra fields common to statements.
type miniStmt struct {
	miniNode
	init Nodes
}

func (*miniStmt) isStmt() {}

func (n *miniStmt) Init() Nodes     { return n.init }
func (n *miniStmt) SetInit(x Nodes) { n.init = x }
func (n *miniStmt) PtrInit() *Nodes { return &n.init }

// An AssignListStmt is an assignment statement with
// more than one item on at least one side: Lhs = Rhs.
// If Def is true, the assignment is a :=.
type AssignListStmt struct {
	miniStmt
	Lhs Nodes
	Def bool
	Rhs Nodes
}

func NewAssignListStmt(pos src.XPos, op Op, lhs, rhs []Node) *AssignListStmt {
	n := &AssignListStmt{}
	n.pos = pos
	n.SetOp(op)
	n.Lhs = lhs
	n.Rhs = rhs
	return n
}

func (n *AssignListStmt) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OAS2, OAS2DOTTYPE, OAS2FUNC, OAS2MAPR, OAS2RECV, OSELRECV2:
		n.op = op
	}
}

// An AssignStmt is a simple assignment statement: X = Y.
// If Def is true, the assignment is a :=.
type AssignStmt struct {
	miniStmt
	X   Node
	Def bool
	Y   Node
}

func NewAssignStmt(pos src.XPos, x, y Node) *AssignStmt {
	n := &AssignStmt{X: x, Y: y}
	n.pos = pos
	n.op = OAS
	return n
}

func (n *AssignStmt) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OAS:
		n.op = op
	}
}

// An AssignOpStmt is an AsOp= assignment statement: X AsOp= Y.
type AssignOpStmt struct {
	miniStmt
	X      Node
	AsOp   Op // OADD etc
	Y      Node
	IncDec bool // actually ++ or --
}

func NewAssignOpStmt(pos src.XPos, asOp Op, x, y Node) *AssignOpStmt {
	n := &AssignOpStmt{AsOp: asOp, X: x, Y: y}
	n.pos = pos
	n.op = OASOP
	return n
}

// A BlockStmt is a block: { List }.
type BlockStmt struct {
	miniStmt
	List Nodes
}

func NewBlockStmt(pos src.XPos, list []Node) *BlockStmt {
	n := &BlockStmt{}
	n.pos = pos
	if !pos.IsKnown() {
		n.pos = base.Pos
		if len(list) > 0 {
			n.pos = list[0].Pos()
		}
	}
	n.op = OBLOCK
	n.List = list
	return n
}

// A BranchStmt is a break, continue, fallthrough, or goto statement.
type BranchStmt struct {
	miniStmt
	Label *types.Sym // label if present
}

func NewBranchStmt(pos src.XPos, op Op, label *types.Sym) *BranchStmt {
	switch op {
	case OBREAK, OCONTINUE, OFALL, OGOTO:
		// ok
	default:
		panic("NewBranch " + op.String())
	}
	n := &BranchStmt{Label: label}
	n.pos = pos
	n.op = op
	return n
}

func (n *BranchStmt) SetOp(op Op) {
	switch op {
	default:
		panic(n.no("SetOp " + op.String()))
	case OBREAK, OCONTINUE, OFALL, OGOTO:
		n.op = op
	}
}

func (n *BranchStmt) Sym() *types.Sym { return n.Label }

// A CaseClause is a case statement in a switch or select: case List: Body.
type CaseClause struct {
	miniStmt
	Var  *Name // declared variable for this case in type switch
	List Nodes // list of expressions for switch, early select

	// RTypes is a list of RType expressions, which are copied to the
	// corresponding OEQ nodes that are emitted when switch statements
	// are desugared. RTypes[i] must be non-nil if the emitted
	// comparison for List[i] will be a mixed interface/concrete
	// comparison; see reflectdata.CompareRType for details.
	//
	// Because mixed interface/concrete switch cases are rare, we allow
	// len(RTypes) < len(List). Missing entries are implicitly nil.
	RTypes Nodes

	Body Nodes
}

func NewCaseStmt(pos src.XPos, list, body []Node) *CaseClause {
	n := &CaseClause{List: list, Body: body}
	n.pos = pos
	n.op = OCASE
	return n
}

type CommClause struct {
	miniStmt
	Comm Node // communication case
	Body Nodes
}

func NewCommStmt(pos src.XPos, comm Node, body []Node) *CommClause {
	n := &CommClause{Comm: comm, Body: body}
	n.pos = pos
	n.op = OCASE
	return n
}

// A ForStmt is a non-range for loop: for Init; Cond; Post { Body }
type ForStmt struct {
	miniStmt
	Label        *types.Sym
	Cond         Node
	Post         Node
	Body         Nodes
	DistinctVars bool
}

func NewForStmt(pos src.XPos, init Node, cond, post Node, body []Node, distinctVars bool) *ForStmt {
	n := &ForStmt{Cond: cond, Post: post}
	n.pos = pos
	n.op = OFOR
	if init != nil {
		n.init = []Node{init}
	}
	n.Body = body
	n.DistinctVars = distinctVars
	return n
}

// A GoDeferStmt is a go or defer statement: go Call / defer Call.
//
// The two opcodes use a single syntax because the implementations
// are very similar: both are concerned with saving Call and running it
// in a different context (a separate goroutine or a later time).
type GoDeferStmt struct {
	miniStmt
	Call    Node
	DeferAt Expr
}

func NewGoDeferStmt(pos src.XPos, op Op, call Node) *GoDeferStmt {
	n := &GoDeferStmt{Call: call}
	n.pos = pos
	switch op {
	case ODEFER, OGO:
		n.op = op
	default:
		panic("NewGoDeferStmt " + op.String())
	}
	return n
}

// An IfStmt is a return statement: if Init; Cond { Body } else { Else }.
type IfStmt struct {
	miniStmt
	Cond   Node
	Body   Nodes
	Else   Nodes
	Likely bool // code layout hint
}

func NewIfStmt(pos src.XPos, cond Node, body, els []Node) *IfStmt {
	n := &IfStmt{Cond: cond}
	n.pos = pos
	n.op = OIF
	n.Body = body
	n.Else = els
	return n
}

// A JumpTableStmt is used to implement switches. Its semantics are:
//
//	tmp := jt.Idx
//	if tmp == Cases[0] goto Targets[0]
//	if tmp == Cases[1] goto Targets[1]
//	...
//	if tmp == Cases[n] goto Targets[n]
//
// Note that a JumpTableStmt is more like a multiway-goto than
// a multiway-if. In particular, the case bodies are just
// labels to jump to, not full Nodes lists.
type JumpTableStmt struct {
	miniStmt

	// Value used to index the jump table.
	// We support only integer types that
	// are at most the size of a uintptr.
	Idx Node

	// If Idx is equal to Cases[i], jump to Targets[i].
	// Cases entries must be distinct and in increasing order.
	// The length of Cases and Targets must be equal.
	Cases   []constant.Value
	Targets []*types.Sym
}

func NewJumpTableStmt(pos src.XPos, idx Node) *JumpTableStmt {
	n := &JumpTableStmt{Idx: idx}
	n.pos = pos
	n.op = OJUMPTABLE
	return n
}

// An InterfaceSwitchStmt is used to implement type switches.
// Its semantics are:
//
//	if RuntimeType implements Descriptor.Cases[0] {
//	    Case, Itab = 0, itab<RuntimeType, Descriptor.Cases[0]>
//	} else if RuntimeType implements Descriptor.Cases[1] {
//	    Case, Itab = 1, itab<RuntimeType, Descriptor.Cases[1]>
//	...
//	} else if RuntimeType implements Descriptor.Cases[N-1] {
//	    Case, Itab = N-1, itab<RuntimeType, Descriptor.Cases[N-1]>
//	} else {
//	    Case, Itab = len(cases), nil
//	}
//
// RuntimeType must be a non-nil *runtime._type.
// Hash must be the hash field of RuntimeType (or its copy loaded from an itab).
// Descriptor must represent an abi.InterfaceSwitch global variable.
type InterfaceSwitchStmt struct {
	miniStmt

	Case        Node
	Itab        Node
	RuntimeType Node
	Hash        Node
	Descriptor  *obj.LSym
}

func NewInterfaceSwitchStmt(pos src.XPos, case_, itab, runtimeType, hash Node, descriptor *obj.LSym) *InterfaceSwitchStmt {
	n := &InterfaceSwitchStmt{
		Case:        case_,
		Itab:        itab,
		RuntimeType: runtimeType,
		Hash:        hash,
		Descriptor:  descriptor,
	}
	n.pos = pos
	n.op = OINTERFACESWITCH
	return n
}

// An InlineMarkStmt is a marker placed just before an inlined body.
type InlineMarkStmt struct {
	miniStmt
	Index int64
}

func NewInlineMarkStmt(pos src.XPos, index int64) *InlineMarkStmt {
	n := &InlineMarkStmt{Index: index}
	n.pos = pos
	n.op = OINLMARK
	return n
}

func (n *InlineMarkStmt) Offset() int64     { return n.Index }
func (n *InlineMarkStmt) SetOffset(x int64) { n.Index = x }

// A LabelStmt is a label statement (just the label, not including the statement it labels).
type LabelStmt struct {
	miniStmt
	Label *types.Sym // "Label:"
}

func NewLabelStmt(pos src.XPos, label *types.Sym) *LabelStmt {
	n := &LabelStmt{Label: label}
	n.pos = pos
	n.op = OLABEL
	return n
}

func (n *LabelStmt) Sym() *types.Sym { return n.Label }

// A RangeStmt is a range loop: for Key, Value = range X { Body }
type RangeStmt struct {
	miniStmt
	Label        *types.Sym
	Def          bool
	X            Node
	RType        Node `mknode:"-"` // see reflectdata/helpers.go
	Key          Node
	Value        Node
	Body         Nodes
	DistinctVars bool
	Prealloc     *Name

	// When desugaring the RangeStmt during walk, the assignments to Key
	// and Value may require OCONVIFACE operations. If so, these fields
	// will be copied to their respective ConvExpr fields.
	KeyTypeWord   Node `mknode:"-"`
	KeySrcRType   Node `mknode:"-"`
	ValueTypeWord Node `mknode:"-"`
	ValueSrcRType Node `mknode:"-"`
}

func NewRangeStmt(pos src.XPos, key, value, x Node, body []Node, distinctVars bool) *RangeStmt {
	n := &RangeStmt{X: x, Key: key, Value: value}
	n.pos = pos
	n.op = ORANGE
	n.Body = body
	n.DistinctVars = distinctVars
	return n
}

// A ReturnStmt is a return statement.
type ReturnStmt struct {
	miniStmt
	Results Nodes // return list
}

func NewReturnStmt(pos src.XPos, results []Node) *ReturnStmt {
	n := &ReturnStmt{}
	n.pos = pos
	n.op = ORETURN
	n.Results = results
	return n
}

// A SelectStmt is a block: { Cases }.
type SelectStmt struct {
	miniStmt
	Label *types.Sym
	Cases []*CommClause

	// TODO(rsc): Instead of recording here, replace with a block?
	Compiled Nodes // compiled form, after walkSelect
}

func NewSelectStmt(pos src.XPos, cases []*CommClause) *SelectStmt {
	n := &SelectStmt{Cases: cases}
	n.pos = pos
	n.op = OSELECT
	return n
}

// A SendStmt is a send statement: X <- Y.
type SendStmt struct {
	miniStmt
	Chan  Node
	Value Node
}

func NewSendStmt(pos src.XPos, ch, value Node) *SendStmt {
	n := &SendStmt{Chan: ch, Value: value}
	n.pos = pos
	n.op = OSEND
	return n
}

// A SwitchStmt is a switch statement: switch Init; Tag { Cases }.
type SwitchStmt struct {
	miniStmt
	Tag   Node
	Cases []*CaseClause
	Label *types.Sym

	// TODO(rsc): Instead of recording here, replace with a block?
	Compiled Nodes // compiled form, after walkSwitch
}

func NewSwitchStmt(pos src.XPos, tag Node, cases []*CaseClause) *SwitchStmt {
	n := &SwitchStmt{Tag: tag, Cases: cases}
	n.pos = pos
	n.op = OSWITCH
	return n
}

// A TailCallStmt is a tail call statement, which is used for back-end
// code generation to jump directly to another function entirely.
type TailCallStmt struct {
	miniStmt
	Call *CallExpr // the underlying call
}

func NewTailCallStmt(pos src.XPos, call *CallExpr) *TailCallStmt {
	n := &TailCallStmt{Call: call}
	n.pos = pos
	n.op = OTAILCALL
	return n
}

// A TypeSwitchGuard is the [Name :=] X.(type) in a type switch.
type TypeSwitchGuard struct {
	miniNode
	Tag  *Ident
	X    Node
	Used bool
}

func NewTypeSwitchGuard(pos src.XPos, tag *Ident, x Node) *TypeSwitchGuard {
	n := &TypeSwitchGuard{Tag: tag, X: x}
	n.pos = pos
	n.op = OTYPESW
	return n
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/symtab.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"cmd/compile/internal/types"
	"cmd/internal/obj"
)

// Syms holds known symbols.
var Syms symsStruct

type symsStruct struct {
	AssertE2I                 *obj.LSym
	AssertE2I2                *obj.LSym
	Asanread                  *obj.LSym
	Asanwrite                 *obj.LSym
	CgoCheckMemmove           *obj.LSym
	CgoCheckPtrWrite          *obj.LSym
	CheckPtrAlignment         *obj.LSym
	Deferproc                 *obj.LSym
	Deferprocat               *obj.LSym
	DeferprocStack            *obj.LSym
	Deferreturn               *obj.LSym
	Duffcopy                  *obj.LSym
	Duffzero                  *obj.LSym
	GCWriteBarrier            [8]*obj.LSym
	Goschedguarded            *obj.LSym
	Growslice                 *obj.LSym
	GrowsliceBuf              *obj.LSym
	GrowsliceBufNoAlias       *obj.LSym
	GrowsliceNoAlias          *obj.LSym
	MoveSlice                 *obj.LSym
	MoveSliceNoScan           *obj.LSym
	MoveSliceNoCap            *obj.LSym
	MoveSliceNoCapNoScan      *obj.LSym
	InterfaceSwitch           *obj.LSym
	MallocGC                  *obj.LSym
	MallocGCTiny              *obj.LSym
	MallocGCSmallNoScan       [8]*obj.LSym
	MallocGCSmallScanNoHeader [8]*obj.LSym
	Memmove                   *obj.LSym
	Memequal                  *obj.LSym
	Msanread                  *obj.LSym
	Msanwrite                 *obj.LSym
	Msanmove                  *obj.LSym
	Newobject                 *obj.LSym
	Newproc                   *obj.LSym
	PanicBounds               *obj.LSym
	PanicExtend               *obj.LSym
	Panicdivide               *obj.LSym
	Panicshift                *obj.LSym
	PanicdottypeE             *obj.LSym
	PanicdottypeI             *obj.LSym
	Panicnildottype           *obj.LSym
	Panicoverflow             *obj.LSym
	PanicSimdImm              *obj.LSym
	Racefuncenter             *obj.LSym
	Racefuncexit              *obj.LSym
	Raceread                  *obj.LSym
	Racereadrange             *obj.LSym
	Racewrite                 *obj.LSym
	Racewriterange            *obj.LSym
	TypeAssert                *obj.LSym
	WBZero                    *obj.LSym
	WBMove                    *obj.LSym
	// Wasm
	SigPanic             *obj.LSym
	Staticuint64s        *obj.LSym
	Typedmemmove         *obj.LSym
	Udiv                 *obj.LSym
	WriteBarrier         *obj.LSym
	Zerobase             *obj.LSym
	ZeroVal              *obj.LSym
	ARM64HasATOMICS      *obj.LSym
	ARMHasVFPv4          *obj.LSym
	Loong64HasLAMCAS     *obj.LSym
	Loong64HasLAM_BH     *obj.LSym
	Loong64HasDBAR_HINTS *obj.LSym
	Loong64HasLSX        *obj.LSym
	RISCV64HasZbb        *obj.LSym
	X86HasAVX            *obj.LSym
	X86HasFMA            *obj.LSym
	X86HasPOPCNT         *obj.LSym
	X86HasSSE41          *obj.LSym
	// Wasm
	WasmDiv *obj.LSym
	// Wasm
	WasmTruncS *obj.LSym
	// Wasm
	WasmTruncU *obj.LSym
}

// Pkgs holds known packages.
var Pkgs struct {
	Go           *types.Pkg
	Itab         *types.Pkg
	Runtime      *types.Pkg
	InternalMaps *types.Pkg
	Coverage     *types.Pkg
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/type.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// Calling TypeNode converts a *types.Type to a Node shell.

// A typeNode is a Node wrapper for type t.
type typeNode struct {
	miniNode
	typ *types.Type
}

func newTypeNode(typ *types.Type) *typeNode {
	n := &typeNode{typ: typ}
	n.pos = src.NoXPos
	n.op = OTYPE
	n.SetTypecheck(1)
	return n
}

func (n *typeNode) Type() *types.Type { return n.typ }
func (n *typeNode) Sym() *types.Sym   { return n.typ.Sym() }

// TypeNode returns the Node representing the type t.
func TypeNode(t *types.Type) Node {
	if n := t.Obj(); n != nil {
		if n.Type() != t {
			base.Fatalf("type skew: %v has type %v, but expected %v", n, n.Type(), t)
		}
		return n.(*Name)
	}
	return newTypeNode(t)
}

// A DynamicType represents a type expression whose exact type must be
// computed dynamically.
//
// TODO(adonovan): I think "dynamic" is a misnomer here; it's really a
// type with free type parameters that needs to be instantiated to obtain
// a ground type for which an rtype can exist.
type DynamicType struct {
	miniExpr

	// RType is an expression that yields a *runtime._type value
	// representing the asserted type.
	//
	// BUG(mdempsky): If ITab is non-nil, RType may be nil.
	RType Node

	// ITab is an expression that yields a *runtime.itab value
	// representing the asserted type within the assertee expression's
	// original interface type.
	//
	// ITab is only used for assertions (including type switches) from
	// non-empty interface type to a concrete (i.e., non-interface)
	// type. For all other assertions, ITab is nil.
	ITab Node
}

func NewDynamicType(pos src.XPos, rtype Node) *DynamicType {
	n := &DynamicType{RType: rtype}
	n.pos = pos
	n.op = ODYNAMICTYPE
	return n
}

// ToStatic returns static type of dt if it is actually static.
func (dt *DynamicType) ToStatic() Node {
	if dt.Typecheck() == 0 {
		base.Fatalf("missing typecheck: %v", dt)
	}
	if dt.RType != nil && dt.RType.Op() == OADDR {
		addr := dt.RType.(*AddrExpr)
		if addr.X.Op() == OLINKSYMOFFSET {
			return TypeNode(dt.Type())
		}
	}
	if dt.ITab != nil && dt.ITab.Op() == OADDR {
		addr := dt.ITab.(*AddrExpr)
		if addr.X.Op() == OLINKSYMOFFSET {
			return TypeNode(dt.Type())
		}
	}
	return nil
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/val.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

import (
	"go/constant"

	"cmd/compile/internal/base"
	"cmd/compile/internal/types"
)

func ConstType(n Node) constant.Kind {
	if n == nil || n.Op() != OLITERAL {
		return constant.Unknown
	}
	return n.Val().Kind()
}

// IntVal returns v converted to int64.
// Note: if t is uint64, very large values will be converted to negative int64.
func IntVal(t *types.Type, v constant.Value) int64 {
	if t.IsUnsigned() {
		if x, ok := constant.Uint64Val(v); ok {
			return int64(x)
		}
	} else {
		if x, ok := constant.Int64Val(v); ok {
			return x
		}
	}
	base.Fatalf("%v out of range for %v", v, t)
	panic("unreachable")
}

func AssertValidTypeForConst(t *types.Type, v constant.Value) {
	if !ValidTypeForConst(t, v) {
		base.Fatalf("%v (%v) does not represent %v (%v)", t, t.Kind(), v, v.Kind())
	}
}

func ValidTypeForConst(t *types.Type, v constant.Value) bool {
	switch v.Kind() {
	case constant.Unknown:
		return OKForConst[t.Kind()]
	case constant.Bool:
		return t.IsBoolean()
	case constant.String:
		return t.IsString()
	case constant.Int:
		return t.IsInteger()
	case constant.Float:
		return t.IsFloat()
	case constant.Complex:
		return t.IsComplex()
	}

	base.Fatalf("unexpected constant kind: %v", v)
	panic("unreachable")
}

var OKForConst [types.NTYPE]bool

// Int64Val returns n as an int64.
// n must be an integer or rune constant.
func Int64Val(n Node) int64 {
	if !IsConst(n, constant.Int) {
		base.Fatalf("Int64Val(%v)", n)
	}
	x, ok := constant.Int64Val(n.Val())
	if !ok {
		base.Fatalf("Int64Val(%v)", n)
	}
	return x
}

// Uint64Val returns n as a uint64.
// n must be an integer or rune constant.
func Uint64Val(n Node) uint64 {
	if !IsConst(n, constant.Int) {
		base.Fatalf("Uint64Val(%v)", n)
	}
	x, ok := constant.Uint64Val(n.Val())
	if !ok {
		base.Fatalf("Uint64Val(%v)", n)
	}
	return x
}

// BoolVal returns n as a bool.
// n must be a boolean constant.
func BoolVal(n Node) bool {
	if !IsConst(n, constant.Bool) {
		base.Fatalf("BoolVal(%v)", n)
	}
	return constant.BoolVal(n.Val())
}

// StringVal returns the value of a literal string Node as a string.
// n must be a string constant.
func StringVal(n Node) string {
	if !IsConst(n, constant.String) {
		base.Fatalf("StringVal(%v)", n)
	}
	return constant.StringVal(n.Val())
}

```

// === FILE: references/go/src/cmd/compile/internal/ir/visit.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// IR visitors for walking the IR tree.
//
// The lowest level helpers are DoChildren and EditChildren, which
// nodes help implement and provide control over whether and when
// recursion happens during the walk of the IR.
//
// Although these are both useful directly, two simpler patterns
// are fairly common and also provided: Visit and Any.

package ir

// DoChildren calls do(x) on each of n's non-nil child nodes x.
// If any call returns true, DoChildren stops and returns true.
// Otherwise, DoChildren returns false.
//
// Note that DoChildren(n, do) only calls do(x) for n's immediate children.
// If x's children should be processed, then do(x) must call DoChildren(x, do).
//
// DoChildren allows constructing general traversals of the IR graph
// that can stop early if needed. The most general usage is:
//
//	var do func(ir.Node) bool
//	do = func(x ir.Node) bool {
//		... processing BEFORE visiting children ...
//		if ... should visit children ... {
//			ir.DoChildren(x, do)
//			... processing AFTER visiting children ...
//		}
//		if ... should stop parent DoChildren call from visiting siblings ... {
//			return true
//		}
//		return false
//	}
//	do(root)
//
// Since DoChildren does not return true itself, if the do function
// never wants to stop the traversal, it can assume that DoChildren
// itself will always return false, simplifying to:
//
//	var do func(ir.Node) bool
//	do = func(x ir.Node) bool {
//		... processing BEFORE visiting children ...
//		if ... should visit children ... {
//			ir.DoChildren(x, do)
//		}
//		... processing AFTER visiting children ...
//		return false
//	}
//	do(root)
//
// The Visit function illustrates a further simplification of the pattern,
// only processing before visiting children and never stopping:
//
//	func Visit(n ir.Node, visit func(ir.Node)) {
//		if n == nil {
//			return
//		}
//		var do func(ir.Node) bool
//		do = func(x ir.Node) bool {
//			visit(x)
//			return ir.DoChildren(x, do)
//		}
//		do(n)
//	}
//
// The Any function illustrates a different simplification of the pattern,
// visiting each node and then its children, recursively, until finding
// a node x for which cond(x) returns true, at which point the entire
// traversal stops and returns true.
//
//	func Any(n ir.Node, cond(ir.Node) bool) bool {
//		if n == nil {
//			return false
//		}
//		var do func(ir.Node) bool
//		do = func(x ir.Node) bool {
//			return cond(x) || ir.DoChildren(x, do)
//		}
//		return do(n)
//	}
//
// Visit and Any are presented above as examples of how to use
// DoChildren effectively, but of course, usage that fits within the
// simplifications captured by Visit or Any will be best served
// by directly calling the ones provided by this package.
func DoChildren(n Node, do func(Node) bool) bool {
	if n == nil {
		return false
	}
	return n.doChildren(do)
}

// DoChildrenWithHidden is like DoChildren, but also visits
// Node-typed fields tagged with `mknode:"-"`.
//
// TODO(mdempsky): Remove the `mknode:"-"` tags so this function can
// go away.
func DoChildrenWithHidden(n Node, do func(Node) bool) bool {
	if n == nil {
		return false
	}
	return n.doChildrenWithHidden(do)
}

// Visit visits each non-nil node x in the IR tree rooted at n
// in a depth-first preorder traversal, calling visit on each node visited.
func Visit(n Node, visit func(Node)) {
	if n == nil {
		return
	}
	var do func(Node) bool
	do = func(x Node) bool {
		visit(x)
		return DoChildren(x, do)
	}
	do(n)
}

// VisitList calls Visit(x, visit) for each node x in the list.
func VisitList(list Nodes, visit func(Node)) {
	for _, x := range list {
		Visit(x, visit)
	}
}

// VisitFuncAndClosures calls visit on each non-nil node in fn.Body,
// including any nested closure bodies.
func VisitFuncAndClosures(fn *Func, visit func(n Node)) {
	VisitList(fn.Body, func(n Node) {
		visit(n)
		if n, ok := n.(*ClosureExpr); ok && n.Op() == OCLOSURE {
			VisitFuncAndClosures(n.Func, visit)
		}
	})
}

// Any looks for a non-nil node x in the IR tree rooted at n
// for which cond(x) returns true.
// Any considers nodes in a depth-first, preorder traversal.
// When Any finds a node x such that cond(x) is true,
// Any ends the traversal and returns true immediately.
// Otherwise Any returns false after completing the entire traversal.
func Any(n Node, cond func(Node) bool) bool {
	if n == nil {
		return false
	}
	var do func(Node) bool
	do = func(x Node) bool {
		return cond(x) || DoChildren(x, do)
	}
	return do(n)
}

// EditChildren edits the child nodes of n, replacing each child x with edit(x).
//
// Note that EditChildren(n, edit) only calls edit(x) for n's immediate children.
// If x's children should be processed, then edit(x) must call EditChildren(x, edit).
//
// EditChildren allows constructing general editing passes of the IR graph.
// The most general usage is:
//
//	var edit func(ir.Node) ir.Node
//	edit = func(x ir.Node) ir.Node {
//		... processing BEFORE editing children ...
//		if ... should edit children ... {
//			EditChildren(x, edit)
//			... processing AFTER editing children ...
//		}
//		... return x ...
//	}
//	n = edit(n)
//
// EditChildren edits the node in place. To edit a copy, call Copy first.
// As an example, a simple deep copy implementation would be:
//
//	func deepCopy(n ir.Node) ir.Node {
//		var edit func(ir.Node) ir.Node
//		edit = func(x ir.Node) ir.Node {
//			x = ir.Copy(x)
//			ir.EditChildren(x, edit)
//			return x
//		}
//		return edit(n)
//	}
//
// Of course, in this case it is better to call ir.DeepCopy than to build one anew.
func EditChildren(n Node, edit func(Node) Node) {
	if n == nil {
		return
	}
	n.editChildren(edit)
}

// EditChildrenWithHidden is like EditChildren, but also edits
// Node-typed fields tagged with `mknode:"-"`.
//
// TODO(mdempsky): Remove the `mknode:"-"` tags so this function can
// go away.
func EditChildrenWithHidden(n Node, edit func(Node) Node) {
	if n == nil {
		return
	}
	n.editChildrenWithHidden(edit)
}

```

