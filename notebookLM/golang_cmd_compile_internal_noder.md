# Domain Architecture: cmd/compile/internal/noder

## Layout Topology
```text
cmd/compile/internal/noder/
├── README.md
├── codes.go
├── doc.go
├── dump.go
├── export.go
├── helpers.go
├── html.go
├── import.go
├── irgen.go
├── lex.go
├── linker.go
├── noder.go
├── posmap.go
├── quirks.go
├── reader.go
├── types.go
├── unified.go
└── writer.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/compile/internal/noder/codes.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import "internal/pkgbits"

// A codeStmt distinguishes among statement encodings.
type codeStmt int

func (c codeStmt) Marker() pkgbits.SyncMarker { return pkgbits.SyncStmt1 }
func (c codeStmt) Value() int                 { return int(c) }

const (
	stmtEnd codeStmt = iota
	stmtLabel
	stmtBlock
	stmtExpr
	stmtSend
	stmtAssign
	stmtAssignOp
	stmtIncDec
	stmtBranch
	stmtCall
	stmtReturn
	stmtIf
	stmtFor
	stmtSwitch
	stmtSelect
)

// A codeExpr distinguishes among expression encodings.
type codeExpr int

func (c codeExpr) Marker() pkgbits.SyncMarker { return pkgbits.SyncExpr }
func (c codeExpr) Value() int                 { return int(c) }

// TODO(mdempsky): Split expr into addr, for lvalues.
const (
	exprConst  codeExpr = iota
	exprLocal           // local variable
	exprGlobal          // global variable or function
	exprCompLit
	exprFuncLit
	exprFieldVal
	exprMethodVal
	exprMethodExpr
	exprIndex
	exprSlice
	exprAssert
	exprUnaryOp
	exprBinaryOp
	exprCall
	exprConvert
	exprNew
	exprMake
	exprSizeof
	exprAlignof
	exprOffsetof
	exprZero
	exprFuncInst
	exprRecv
	exprReshape
	exprRuntimeBuiltin // a reference to a runtime function from transformed syntax. Followed by string name, e.g., "panicrangeexit"
)

type codeAssign int

func (c codeAssign) Marker() pkgbits.SyncMarker { return pkgbits.SyncAssign }
func (c codeAssign) Value() int                 { return int(c) }

const (
	assignBlank codeAssign = iota
	assignDef
	assignExpr
)

// A codeDecl distinguishes among declaration encodings.
type codeDecl int

func (c codeDecl) Marker() pkgbits.SyncMarker { return pkgbits.SyncDecl }
func (c codeDecl) Value() int                 { return int(c) }

const (
	declEnd codeDecl = iota
	declFunc
	declMethod
	declVar
	declOther
)

```

// === FILE: references/go/src/cmd/compile/internal/noder/doc.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
The Unified IR (UIR) format is implicitly defined by the package noder.

At the highest level, a package encoded in UIR follows the grammar
below.

    File        = Header Payload fingerprint .
    Header      = version [ flags ] sectionEnds elementEnds .

    version     = uint32 .     // used for backward compatibility
    flags       = uint32 .     // feature flags used across versions
    sectionEnds = [10]uint32 . // defines section boundaries
    elementEnds = []uint32 .   // defines element boundaries
    fingerprint = [8]byte .    // sha256 fingerprint

The payload is a series of sections. Each section has a kind which
determines its index in the series.

    SectionKind = Uint64 .
    Payload     = SectionString
                  SectionMeta
                  SectionPosBase
                  SectionPkg
                  SectionName
                  SectionType
                  SectionObj
                  SectionObjExt  // TODO(markfreeman) Define.
                  SectionObjDict // TODO(markfreeman) Define.
                  SectionBody    // TODO(markfreeman) Define.
                  .

# Sections
A section is a series of elements of a type determined by the section's
kind. Go constructs are mapped onto one or more elements with possibly
different types; in that case, the elements are in different sections.

Elements are accessed using an element index relative to the start of
the section.

    RelElemIdx = Uint64 .

## String Section
String values are stored as elements in the string section. Elements
outside the string section access string values by reference.

    SectionString = { String } .

Note that despite being an element, a string does not begin with a
reference table.

## Meta Section
The meta section provides fundamental information for a package. It
contains exactly two elements — a public root and a private root.

    SectionMeta = PublicRoot
                  PrivateRoot     // TODO(markfreeman): Define.
                  .

The public root element identifies the package and provides references
for all exported objects it contains.

    PublicRoot  = RefTable
                  [ Sync ]
                  PkgRef
                  [ HasInit ]
                  ObjectRefCount // TODO(markfreeman): Define.
                  { ObjectRef }  // TODO(markfreeman): Define.
                  .
    HasInit     = Bool .         // Whether the package uses any
                                 // initialization functions.

## PosBase Section
This section provides position information. It is a series of PosBase
elements.

    SectionPosBase = { PosBase } .

A base is either a file base or line base (produced by a line
directive). Every base has a position, line, and column; these are
constant for file bases and hence not encoded.

    PosBase = RefTable
              [ Sync ]
              StringRef       // the (absolute) file name for the base
              Bool            // true if a file base, else a line base
              // The below is omitted for file bases.
              [ Pos
                Uint64        // line
                Uint64 ]      // column
              .

A source position Pos represents a file-absolute (line, column) pair
and a PosBase indicating the position Pos is relative to. Positions
without a PosBase have no line or column.

    Pos     = [ Sync ]
              Bool             // true if the position has a base
              // The below is omitted if the position has no base.
              [ Ref[PosBase]
                Uint64         // line
                Uint64 ]       // column
              .

## Package Section
The package section holds package information. It is a series of Pkg
elements.

    SectionPkg = { Pkg } .

A Pkg element contains a (path, name) pair and a series of imported
packages. The below package paths have special meaning.

    +--------------+-----------------------------------+
    | package path |             indicates             |
    +--------------+-----------------------------------+
    | ""           | the current package               |
    | "builtin"    | the fake builtin package          |
    | "unsafe"     | the compiler-known unsafe package |
    +--------------+-----------------------------------+

    Pkg        = RefTable
                 [ Sync ]
                 StringRef      // path
                 // The below is omitted for the special package paths
                 // "builtin" and "unsafe".
                 [ StringRef    // name
                   Imports ]
                 .
    Imports    = Uint64         // the number of declared imports
                 { PkgRef }     // references to declared imports
                 .

Note, a PkgRef is *not* equivalent to Ref[Pkg] due to an extra marker.

    PkgRef     = [ Sync ]
                 Ref[Pkg]
                 .

## Type Section
The type section is a series of type definition elements.

    SectionType = { TypeDef } .

A type definition can be in one of several formats, which are identified
by their TypeSpec code.

    TypeDef     = RefTable
                  [ Sync ]
                  [ Sync ]
                  Uint64            // denotes which TypeSpec to use
                  TypeSpec
                  .

    TypeSpec    = TypeSpecBasic     // TODO(markfreeman): Define.
                | TypeSpecNamed     // TODO(markfreeman): Define.
                | TypeSpecPointer   // TODO(markfreeman): Define.
                | TypeSpecSlice     // TODO(markfreeman): Define.
                | TypeSpecArray     // TODO(markfreeman): Define.
                | TypeSpecChan      // TODO(markfreeman): Define.
                | TypeSpecMap       // TODO(markfreeman): Define.
                | TypeSpecSignature // TODO(markfreeman): Define.
                | TypeSpecStruct    // TODO(markfreeman): Define.
                | TypeSpecInterface // TODO(markfreeman): Define.
                | TypeSpecUnion     // TODO(markfreeman): Define.
                | TypeSpecTypeParam // TODO(markfreeman): Define.
                  .

// TODO(markfreeman): Document the reader dictionary once we understand it more.
To use a type elsewhere, a TypeUse is encoded.

    TypeUse     = [ Sync ]
                  Bool              // whether it is a derived type
                  [ Uint64 ]        // if derived, an index into the reader dictionary
                  [ Ref[TypeDef] ]  // else, a reference to the type
                  .

## Object Sections
Information about an object (e.g. variable, function, type name, etc.)
is split into multiple elements in different sections. Those elements
have the same section-relative element index.

### Name Section
The name section holds a series of names.

    SectionName = { Name } .

Names are elements holding qualified identifiers and type information
for objects.

    Name        = RefTable
                  [ Sync ]
                  [ Sync ]
                  PkgRef    // the object's package
                  StringRef // the object's package-local name
                  [ Sync ]
                  Uint64    // the object's type (e.g. Var, Func, etc.)
                  .

### Definition Section
The definition section holds definitions for objects defined by the target
package; it does not contain definitions for imported objects.

    SectionObj = { ObjectDef } .

Object definitions can be in one of several formats. To determine the correct
format, the name section must be referenced; it contains a code indicating
the object's type.

    ObjectDef = RefTable
                [ Sync ]
                ObjectSpec
                .

    ObjectSpec = ObjectSpecConst     // TODO(markfreeman) Define.
               | ObjectSpecFunc      // TODO(markfreeman) Define.
               | ObjectSpecAlias     // TODO(markfreeman) Define.
               | ObjectSpecNamedType // TODO(markfreeman) Define.
               | ObjectSpecVar       // TODO(markfreeman) Define.
                 .

To use an object definition elsewhere, an ObjectUse is encoded.

    ObjectUse  = [ Sync ]
                 [ Bool ]
                 Ref[ObjectDef]
                 Uint64              // the number of type arguments
                 { TypeUse }         // references to the type arguments
                 .

# References
A reference table precedes every element. Each entry in the table
contains a (section, index) pair denoting the location of the
referenced element.

    RefTable      = [ Sync ]
                    Uint64            // the number of table entries
                    { RefTableEntry }
                    .
    RefTableEntry = [ Sync ]
                    SectionKind
                    RelElemIdx
                    .

Elements encode references to other elements as an index in the
reference table — not the location of the referenced element directly.

    RefTableIdx   = Uint64 .

To do this, the Ref[T] primitive is used as below; note that this is
the same shape as provided by package pkgbits, just with new
interpretation applied.

    Ref[T]        = [ Sync ]
                    RefTableIdx       // the Uint64
                    .

# Primitives
Primitive encoding is handled separately by the pkgbits package. Check
there for definitions of the below productions.

    * Bool
    * Int64
    * Uint64
    * String
    * Ref[T]
    * Sync
*/

package noder

```

// === FILE: references/go/src/cmd/compile/internal/noder/dump.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"sync"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
)

// MatchASTDump returns true if the fn matches the value
// of the astdump debug flag.
func MatchASTDump(fn *syntax.FuncDecl) bool {
	if len(base.Debug.AstDump) == 0 {
		return false
	}
	if fn.Name == nil {
		return false
	}
	return matchForDump(fn, base.Ctxt.Pkgpath)
}

// matchForDump is marked noinline to ensure that the exported
// function MatchAstDump IS inlineable and is also small, because
// common case is AstDump is not set.
//
//go:noinline
func matchForDump(fn *syntax.FuncDecl, pkgPath string) bool {
	return ir.MatchPkgFn(pkgPath, fn.Name.Value, base.Debug.AstDump)
}

func escapedFileName(fn *syntax.FuncDecl, suffix string) string {
	return ir.EscapedFileName(base.Ctxt.Pkgpath+"."+fn.Name.Value, suffix)
}

var mu sync.Mutex
var htmlWriters = make(map[*syntax.FuncDecl]*HTMLWriter)
var orderedFuncs = []*syntax.FuncDecl{}

// DumpNodeHTML dumps the node n to the HTML writer for fn.
func DumpNodeHTML(pkg *types2.Package, file *syntax.File, info *types2.Info, fn *syntax.FuncDecl, why string, n syntax.Node) {
	mu.Lock()
	defer mu.Unlock()
	w, ok := htmlWriters[fn]
	if !ok {
		name := escapedFileName(fn, ".syntax.html")
		w = NewHTMLWriter(pkg, file, info, name, fn, "")
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
			w.Close("Writing html syntax output for %s to %s\n", w.pkgFuncName(), w.Path())
			delete(htmlWriters, fn)
		}
	}
	orderedFuncs = nil
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/export.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"bytes"
	"fmt"
	"io"

	"cmd/compile/internal/base"
	"cmd/internal/bio"
)

func WriteExports(out *bio.Writer) {
	var data bytes.Buffer

	data.WriteByte('u')
	writeUnifiedExport(&data)

	// The linker also looks for the $$ marker - use char after $$ to distinguish format.
	out.WriteString("\n$$B\n") // indicate binary export format
	io.Copy(out, &data)
	out.WriteString("\n$$\n")

	if base.Debug.Export != 0 {
		fmt.Printf("BenchmarkExportSize:%s 1 %d bytes\n", base.Ctxt.Pkgpath, data.Len())
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/helpers.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"go/constant"

	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/compile/internal/types2"
	"cmd/internal/src"
)

// Helpers for constructing typed IR nodes.
//
// TODO(mdempsky): Move into their own package so they can be easily
// reused by iimport and frontend optimizations.

type ImplicitNode interface {
	ir.Node
	SetImplicit(x bool)
}

// Implicit returns n after marking it as Implicit.
func Implicit(n ImplicitNode) ImplicitNode {
	n.SetImplicit(true)
	return n
}

// typed returns n after setting its type to typ.
func typed(typ *types.Type, n ir.Node) ir.Node {
	n.SetType(typ)
	n.SetTypecheck(1)
	return n
}

// Values

// FixValue returns val after converting and truncating it as
// appropriate for typ.
func FixValue(typ *types.Type, val constant.Value) constant.Value {
	assert(typ.Kind() != types.TFORW)
	switch {
	case typ.IsInteger():
		val = constant.ToInt(val)
	case typ.IsFloat():
		val = constant.ToFloat(val)
	case typ.IsComplex():
		val = constant.ToComplex(val)
	}
	if !typ.IsUntyped() {
		val = typecheck.ConvertVal(val, typ, false)
	}
	ir.AssertValidTypeForConst(typ, val)
	return val
}

// Expressions

func Addr(pos src.XPos, x ir.Node) *ir.AddrExpr {
	n := typecheck.NodAddrAt(pos, x)
	typed(types.NewPtr(x.Type()), n)
	return n
}

func Deref(pos src.XPos, typ *types.Type, x ir.Node) *ir.StarExpr {
	n := ir.NewStarExpr(pos, x)
	typed(typ, n)
	return n
}

// Statements

func idealType(tv syntax.TypeAndValue) types2.Type {
	// The gc backend expects all expressions to have a concrete type, and
	// types2 mostly satisfies this expectation already. But there are a few
	// cases where the Go spec doesn't require converting to concrete type,
	// and so types2 leaves them untyped. So we need to fix those up here.
	typ := types2.Unalias(tv.Type)
	if basic, ok := typ.(*types2.Basic); ok && basic.Info()&types2.IsUntyped != 0 {
		switch basic.Kind() {
		case types2.UntypedNil:
			// ok; can appear in type switch case clauses
			// TODO(mdempsky): Handle as part of type switches instead?
		case types2.UntypedInt, types2.UntypedFloat, types2.UntypedComplex:
			typ = types2.Typ[types2.Uint]
			if tv.Value != nil {
				s := constant.ToInt(tv.Value)
				assert(s.Kind() == constant.Int)
				if constant.Sign(s) < 0 {
					typ = types2.Typ[types2.Int]
				}
			}
		case types2.UntypedBool:
			typ = types2.Typ[types2.Bool] // expression in "if" or "for" condition
		case types2.UntypedString:
			typ = types2.Typ[types2.String] // argument to "append" or "copy" calls
		case types2.UntypedRune:
			typ = types2.Typ[types2.Int32] // range over rune
		default:
			return nil
		}
	}
	return typ
}

func isTypeParam(t types2.Type) bool {
	_, ok := types2.Unalias(t).(*types2.TypeParam)
	return ok
}

// isNotInHeap reports whether typ is or contains an element of type
// internal/runtime/sys.NotInHeap.
func isNotInHeap(typ types2.Type) bool {
	typ = types2.Unalias(typ)
	if named, ok := typ.(*types2.Named); ok {
		if obj := named.Obj(); obj.Name() == "nih" && obj.Pkg().Path() == "internal/runtime/sys" {
			return true
		}
		typ = named.Underlying()
	}

	switch typ := typ.(type) {
	case *types2.Array:
		return isNotInHeap(typ.Elem())
	case *types2.Struct:
		for i := 0; i < typ.NumFields(); i++ {
			if isNotInHeap(typ.Field(i).Type()) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/html.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// An HTMLWriter dumps syntax nodes to multicolumn HTML, similar to what the
// ssa backend does for GOSSAFUNC.
type HTMLWriter struct {
	ir.HTMLWriterBase

	Decl *syntax.FuncDecl
	pkg  *types2.Package
	file *syntax.File
	info *types2.Info
}

func NewHTMLWriter(pkg *types2.Package, file *syntax.File, info *types2.Info, path string, decl *syntax.FuncDecl, cfgMask string) *HTMLWriter {
	path = strings.ReplaceAll(path, "/", string(filepath.Separator))
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	reportPath := path
	if !filepath.IsAbs(reportPath) {
		pwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		reportPath = filepath.Join(pwd, path)
	}
	h := HTMLWriter{
		pkg:  pkg,
		file: file,
		info: info,
		Decl: decl,
	}
	h.Init(out, reportPath, h.DeclHTML)
	h.start()
	return &h
}

func (w *HTMLWriter) pkgFuncName() string {
	p := w.pkg.Path()
	if p == "" {
		p = base.Ctxt.Pkgpath
	}
	return p + "." + w.Decl.Name.Value
}

func (w *HTMLWriter) start() {
	if w == nil {
		return
	}
	escName := html.EscapeString(w.pkgFuncName())
	w.Print("<!DOCTYPE html>")
	w.Print("<html>")
	w.Printf(`<head>
<meta name="generator" content="AST display for %s">
<meta http-equiv="Content-Type" content="text/html;charset=UTF-8">
%s
%s
<title>AST display for %s</title>
</head>`, escName, ir.CSS, ir.JS("checked", "rangefunc"), escName)
	w.Print("<body>")
	w.Print("<h1>")
	w.Print(escName)
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
Click on a ` + ir.DownArrow + ` to collapse a subtree, or on a ` + ir.RightArrow + ` to expand a subtree.
</p>
<p>
Non-tree attributes, like scope and type lookups, are displayed in italics.  Those may
also be clicked to highlight identity relationships within and between phases.
</p>

</div>
<label for="dark-mode-button" style="margin-left: 15px; cursor: pointer;">darkmode</label>
<input type="checkbox" onclick="toggleDarkMode();" id="dark-mode-button" style="cursor: pointer" />
`)
	w.Print("<table>")
	w.Print("<tr>")
}

func (w *HTMLWriter) DeclHTML(phase string) func() {
	return func() {
		w.Print("<pre>") // use pre for formatting to preserve indentation
		w.dumpScopeHTML(w.pkg.Scope(), 1, false)
		w.dumpScopeHTML(w.info.Scopes[w.file], 1, false)
		w.dumpNodeHTML(w.Decl, 1, "")
		w.Print("</pre>")
	}
}

func (h *HTMLWriter) dumpNodesHTML(list []syntax.Node, depth int) {
	if len(list) == 0 {
		h.Print(" <nil>")
		return
	}

	for _, n := range list {
		h.dumpNodeHTML(n, depth, "")
	}
}

func isValid(t types2.Type) bool {
	return t != nil && types2.Unalias(t) != types2.Typ[types2.Invalid]
}

const indentString = ".  "

func (w *HTMLWriter) indent(n int) {
	w.Print("\n")
	for range n {
		w.Print(indentString)
	}
}

// indentForToggle prints indentation to w.
func (h *HTMLWriter) indentForToggle(depth int, hasChildren bool) {
	h.Print("\n")
	if depth == 0 {
		return
	}
	for range depth - 1 {
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

// dumpScopeHTML writes a string representation of the scope to w,
// with the scope elements sorted by name.
// The level of indentation is controlled by n >= 0, with
// n == 0 for no indentation.
// If recurse is set, it also writes nested (children) scopes.
func (h *HTMLWriter) dumpScopeHTML(s *types2.Scope, depth int, recur bool) {
	hasChildren := true // TODO detect empty scopes
	h.indentForToggle(depth, hasChildren)
	if hasChildren {
		h.Printf("<span class=\"n%d scope\">", h.CanonId(s))
		defer h.Printf("</span>")
		// NOTE TRAILING SPACE after </span>! See indentForToggle above.
		h.Print(`<span class="toggle" onclick="toggle_node(this)">` + ir.DownArrow + `</span> `)
	}
	h.Printf("scope %s %p", html.EscapeString(s.Comment()), s)

	if hasChildren {
		h.Print(`<span class="node-body">`)
		defer h.Print(`</span>`)

		for _, name := range s.Names() {
			obj := s.Lookup(name)
			h.dumpOutlineNodeHTML(depth+1, "", obj)
		}

		if recur {
			for i := range s.NumChildren() {
				c := s.Child(i)
				h.dumpScopeHTML(c, depth+1, recur)
			}
		}
	}
}

func (h *HTMLWriter) dumpOutlineNodeHTML(depth int, pfx string, obj fmt.Stringer) {
	h.indentForToggle(depth, false)
	h.Printf("<span class=\"n%d outline-node\" style=\"font-style: italic;\">%s%s</span>",
		h.CanonId(obj), pfx, html.EscapeString(obj.String()))
}

func (h *HTMLWriter) dumpNodeHTML(n syntax.Node, depth int, prefix string) {
	hasChildren := h.nodeHasChildren(n)
	h.indentForToggle(depth, hasChildren)

	if depth > 40 {
		h.Print("...")
		return
	}

	if n == nil {
		h.Print("NilSyntaxNode")
		return
	}

	h.Printf("<span class=\"n%d outline-node\">", h.CanonId(n))
	defer h.Printf("</span>")

	if hasChildren {
		// NOTE TRAILING SPACE after </span>! See indentForToggle above.
		h.Print(`<span class="toggle" onclick="toggle_node(this)">` + ir.DownArrow + `</span> `) // NOTE TRAILING SPACE after </span>!
	}

	opName := strings.TrimPrefix(fmt.Sprintf("%T", n), "*syntax.")

	if prefix != "" {
		h.Printf("%s", html.EscapeString(prefix))
	}

	switch n := n.(type) {
	case *syntax.BasicLit:
		h.Printf("%s-%v", opName, html.EscapeString(n.Value))
		h.dumpNodeHeaderHTML(n)
		return

	case *syntax.Name:
		name := n.Value
		hash := sha256.Sum256([]byte(name))
		symID := "sym-" + hex.EncodeToString(hash[:6])
		h.Printf("%s-<span class=\"%s variable-name\">%s</span>", opName, symID, html.EscapeString(name))
		h.dumpNodeHeaderHTML(n)
		if hasChildren {
			h.Print(`<span class="node-body">`)
			defer h.Print(`</span>`)

			if obj := h.info.ObjectOf(n); obj != nil {
				h.dumpOutlineNodeHTML(depth+1, "objectOf=", obj)
			}
			if typ := h.info.TypeOf(n); isValid(typ) {
				h.dumpOutlineNodeHTML(depth+1, "typeOf=", typ)
			}
		}
		return

	case syntax.Expr:
		h.Printf("%s", opName)
		h.dumpNodeHeaderHTML(n)
		if hasChildren {
			h.Print(`<span class="node-body">`)
			defer h.Print(`</span>`)

			if typ := h.info.TypeOf(n); isValid(typ) {
				h.dumpOutlineNodeHTML(depth+1, "typeOf=", typ)
			}
		}

	default:
		h.Printf("%s", opName)
		h.dumpNodeHeaderHTML(n)
		if hasChildren {
			h.Print(`<span class="node-body">`)
			defer h.Print(`</span>`)
		}
	}

	if s := h.info.Scopes[n]; s != nil && s.Len() > 0 {
		h.dumpScopeHTML(s, depth+1, false)
	}

	v := reflect.ValueOf(n).Elem()
	t := v.Type()
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

		switch val := vf.Interface().(type) {
		case syntax.Node:
			if name != "" {
				h.dumpNodeHTML(val, depth+1, name+": ")
			} else {
				h.dumpNodeHTML(val, depth+1, "")
			}
		default:
			if vf.Kind() == reflect.Slice && vf.Type().Elem().Implements(nodeType) {
				if vf.Len() == 0 {
					continue
				}
				if name != "" {
					for i := range vf.Len() {
						h.dumpNodeHTML(vf.Index(i).Interface().(syntax.Node), depth+1,
							fmt.Sprintf("%s[%d]: ", name, i))
					}
				} else {
					for i := range vf.Len() {
						h.dumpNodeHTML(vf.Index(i).Interface().(syntax.Node), depth+1, "")
					}
				}
			}
		}
	}
}

var nodeType = reflect.TypeFor[syntax.Node]()

func (h *HTMLWriter) nodeHasChildren(n syntax.Node) bool {
	if n == nil {
		return false
	}
	switch x := n.(type) {
	case *syntax.BasicLit:
		return false
	case *syntax.Name:
		return h.info.ObjectOf(x) != nil || isValid(h.info.TypeOf(x))
	case syntax.Expr:
		if isValid(h.info.TypeOf(x)) {
			return true
		}
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
		switch vf.Interface().(type) {
		case syntax.Node:
			return true
		default:
			if vf.Kind() == reflect.Slice && vf.Type().Elem().Implements(nodeType) && vf.Len() > 0 {
				return true
			}
		}
	}
	return false
}

func (h *HTMLWriter) dumpNodeHeaderHTML(n syntax.Node) {
	v := reflect.ValueOf(n).Elem()
	t := v.Type()
	nf := t.NumField()
	for i := 0; i < nf; i++ {
		tf := t.Field(i)
		if tf.PkgPath != "" {
			continue
		}
		k := tf.Type.Kind()
		if reflect.Bool <= k && k <= reflect.Complex128 || k == reflect.String {
			name := strings.TrimSuffix(tf.Name, "_")
			if name == "Value" {
				continue
			}
			vf := v.Field(i)
			vfi := vf.Interface()
			if vf.IsZero() {
				continue
			}
			if vfi == true {
				h.Printf(" %s", name)
			} else {
				h.Printf(" %s:%+v", name, html.EscapeString(fmt.Sprint(vf.Interface())))
			}
		}
	}

	if n.Pos().IsKnown() {
		h.Print(" <span class=\"line-number\">")
		file := n.Pos().Base().Filename()
		if file != "" {
			hash := sha256.Sum256([]byte(file))
			fileID := "loc-" + hex.EncodeToString(hash[:6])
			lineID := fmt.Sprintf("%s-L%d", fileID, n.Pos().Line())
			colID := fmt.Sprintf("%s-C%d", lineID, n.Pos().Col())

			h.Printf("<span class=\"%s line-number\">%s</span>:", fileID, html.EscapeString(filepath.Base(file)))
			h.Printf("<span class=\"%s %s line-number\">%d</span>:", lineID, fileID, n.Pos().Line())
			h.Printf("<span class=\"%s %s %s line-number\">%d</span>", colID, lineID, fileID, n.Pos().Col())
		} else {
			h.Printf("%v", html.EscapeString(n.Pos().String()))
		}
		h.Print("</span>")
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/import.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"errors"
	"fmt"
	"internal/buildcfg"
	"internal/exportdata"
	"internal/pkgbits"
	"os"
	pathpkg "path"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"

	"cmd/compile/internal/base"
	"cmd/compile/internal/importer"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/compile/internal/types2"
	"cmd/internal/bio"
	"cmd/internal/goobj"
	"cmd/internal/objabi"
)

type gcimports struct {
	ctxt     *types2.Context
	packages map[string]*types2.Package
}

func (m *gcimports) Import(path string) (*types2.Package, error) {
	return m.ImportFrom(path, "" /* no vendoring */, 0)
}

func (m *gcimports) ImportFrom(path, srcDir string, mode types2.ImportMode) (*types2.Package, error) {
	if mode != 0 {
		panic("mode must be 0")
	}

	_, pkg, err := readImportFile(path, typecheck.Target, m.ctxt, m.packages)
	return pkg, err
}

func isDriveLetter(b byte) bool {
	return 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z'
}

// is this path a local name? begins with ./ or ../ or /
func islocalname(name string) bool {
	return strings.HasPrefix(name, "/") ||
		runtime.GOOS == "windows" && len(name) >= 3 && isDriveLetter(name[0]) && name[1] == ':' && name[2] == '/' ||
		strings.HasPrefix(name, "./") || name == "." ||
		strings.HasPrefix(name, "../") || name == ".."
}

func openPackage(path string) (*os.File, error) {
	if islocalname(path) {
		if base.Flag.NoLocalImports {
			return nil, errors.New("local imports disallowed")
		}

		if base.Flag.Cfg.PackageFile != nil {
			return os.Open(base.Flag.Cfg.PackageFile[path])
		}

		// try .a before .o.  important for building libraries:
		// if there is an array.o in the array.a library,
		// want to find all of array.a, not just array.o.
		if file, err := os.Open(fmt.Sprintf("%s.a", path)); err == nil {
			return file, nil
		}
		if file, err := os.Open(fmt.Sprintf("%s.o", path)); err == nil {
			return file, nil
		}
		return nil, errors.New("file not found")
	}

	// local imports should be canonicalized already.
	// don't want to see "encoding/../encoding/base64"
	// as different from "encoding/base64".
	if q := pathpkg.Clean(path); q != path {
		return nil, fmt.Errorf("non-canonical import path %q (should be %q)", path, q)
	}

	if base.Flag.Cfg.PackageFile != nil {
		return os.Open(base.Flag.Cfg.PackageFile[path])
	}

	for _, dir := range base.Flag.Cfg.ImportDirs {
		if file, err := os.Open(fmt.Sprintf("%s/%s.a", dir, path)); err == nil {
			return file, nil
		}
		if file, err := os.Open(fmt.Sprintf("%s/%s.o", dir, path)); err == nil {
			return file, nil
		}
	}

	if buildcfg.GOROOT != "" {
		suffix := ""
		if base.Flag.InstallSuffix != "" {
			suffix = "_" + base.Flag.InstallSuffix
		} else if base.Flag.Race {
			suffix = "_race"
		} else if base.Flag.MSan {
			suffix = "_msan"
		} else if base.Flag.ASan {
			suffix = "_asan"
		}

		if file, err := os.Open(fmt.Sprintf("%s/pkg/%s_%s%s/%s.a", buildcfg.GOROOT, buildcfg.GOOS, buildcfg.GOARCH, suffix, path)); err == nil {
			return file, nil
		}
		if file, err := os.Open(fmt.Sprintf("%s/pkg/%s_%s%s/%s.o", buildcfg.GOROOT, buildcfg.GOOS, buildcfg.GOARCH, suffix, path)); err == nil {
			return file, nil
		}
	}
	return nil, errors.New("file not found")
}

// resolveImportPath resolves an import path as it appears in a Go
// source file to the package's full path.
func resolveImportPath(path string) (string, error) {
	// The package name main is no longer reserved,
	// but we reserve the import path "main" to identify
	// the main package, just as we reserve the import
	// path "math" to identify the standard math package.
	if path == "main" {
		return "", errors.New("cannot import \"main\"")
	}

	if base.Ctxt.Pkgpath == "" {
		panic("missing pkgpath")
	}
	if path == base.Ctxt.Pkgpath {
		return "", fmt.Errorf("import %q while compiling that package (import cycle)", path)
	}

	if mapped, ok := base.Flag.Cfg.ImportMap[path]; ok {
		path = mapped
	}

	if islocalname(path) {
		if path[0] == '/' {
			return "", errors.New("import path cannot be absolute path")
		}

		prefix := base.Flag.D
		if prefix == "" {
			// Questionable, but when -D isn't specified, historically we
			// resolve local import paths relative to the directory the
			// compiler's current directory, not the respective source
			// file's directory.
			prefix = base.Ctxt.Pathname
		}
		path = pathpkg.Join(prefix, path)

		if err := checkImportPath(path, true); err != nil {
			return "", err
		}
	}

	return path, nil
}

// readImportFile reads the import file for the given package path and
// returns its types.Pkg representation. If packages is non-nil, the
// types2.Package representation is also returned.
func readImportFile(path string, target *ir.Package, env *types2.Context, packages map[string]*types2.Package) (pkg1 *types.Pkg, pkg2 *types2.Package, err error) {
	path, err = resolveImportPath(path)
	if err != nil {
		return
	}

	if path == "unsafe" {
		pkg1, pkg2 = types.UnsafePkg, types2.Unsafe

		// TODO(mdempsky): Investigate if this actually matters. Why would
		// the linker or runtime care whether a package imported unsafe?
		if !pkg1.Direct {
			pkg1.Direct = true
			target.Imports = append(target.Imports, pkg1)
		}

		return
	}

	pkg1 = types.NewPkg(path, "")
	if packages != nil {
		pkg2 = packages[path]
		if !(pkg1.Direct == (pkg2 != nil && pkg2.Complete())) {

			base.Fatalf("pkg1.Direct == (pkg2 != nil && pkg2.Complete()), path=%s\npackages=%v", path, packages)
		}
	}

	if pkg1.Direct {
		return
	}
	pkg1.Direct = true
	target.Imports = append(target.Imports, pkg1)

	f, err := openPackage(path)
	if err != nil {
		return
	}
	defer f.Close()

	data, err := readExportData(f)
	if err != nil {
		return
	}

	if base.Debug.Export != 0 {
		fmt.Printf("importing %s (%s)\n", path, f.Name())
	}

	pr := pkgbits.NewPkgDecoder(pkg1.Path, data)

	// Read package descriptors for both types2 and compiler backend.
	readPackage(newPkgReader(pr), pkg1, false)
	pkg2 = importer.ReadPackage(env, packages, pr)

	err = addFingerprint(path, data)
	return
}

// readExportData returns the contents of GC-created unified export data.
func readExportData(f *os.File) (data string, err error) {
	r := bio.NewReader(f)

	sz, err := exportdata.FindPackageDefinition(r.Reader)
	if err != nil {
		return
	}
	end := r.Offset() + int64(sz)

	abihdr, _, err := exportdata.ReadObjectHeaders(r.Reader)
	if err != nil {
		return
	}

	if expect := objabi.HeaderString(); abihdr != expect {
		err = fmt.Errorf("object is [%s] expected [%s]", abihdr, expect)
		return
	}

	_, err = exportdata.ReadExportDataHeader(r.Reader)
	if err != nil {
		return
	}

	pos := r.Offset()

	// Map export data section (+ end-of-section marker) into memory
	// as a single large string. This reduces heap fragmentation and
	// allows returning individual substrings very efficiently.
	var mapped string
	mapped, err = base.MapFile(r.File(), pos, end-pos)
	if err != nil {
		return
	}

	// check for end-of-section marker "\n$$\n" and remove it
	const marker = "\n$$\n"

	var ok bool
	data, ok = strings.CutSuffix(mapped, marker)
	if !ok {
		cutoff := data // include last 10 bytes in error message
		if len(cutoff) >= 10 {
			cutoff = cutoff[len(cutoff)-10:]
		}
		err = fmt.Errorf("expected $$ marker, but found %q (recompile package)", cutoff)
		return
	}

	return
}

// addFingerprint reads the linker fingerprint included at the end of
// the exportdata.
func addFingerprint(path string, data string) error {
	var fingerprint goobj.FingerprintType

	pos := len(data) - len(fingerprint)
	if pos < 0 {
		return fmt.Errorf("missing linker fingerprint in exportdata, but found %q", data)
	}
	buf := []byte(data[pos:])

	copy(fingerprint[:], buf)
	base.Ctxt.AddImport(path, fingerprint)

	return nil
}

func checkImportPath(path string, allowSpace bool) error {
	if path == "" {
		return errors.New("import path is empty")
	}

	if strings.Contains(path, "\x00") {
		return errors.New("import path contains NUL")
	}

	for ri := range base.ReservedImports {
		if path == ri {
			return fmt.Errorf("import path %q is reserved and cannot be used", path)
		}
	}

	for _, r := range path {
		switch {
		case r == utf8.RuneError:
			return fmt.Errorf("import path contains invalid UTF-8 sequence: %q", path)
		case r < 0x20 || r == 0x7f:
			return fmt.Errorf("import path contains control character: %q", path)
		case r == '\\':
			return fmt.Errorf("import path contains backslash; use slash: %q", path)
		case !allowSpace && unicode.IsSpace(r):
			return fmt.Errorf("import path contains space character: %q", path)
		case strings.ContainsRune("!\"#$%&'()*,:;<=>?[]^`{|}", r):
			return fmt.Errorf("import path contains invalid character '%c': %q", r, path)
		}
	}

	return nil
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/irgen.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"fmt"
	"internal/buildcfg"
	"internal/types/errors"
	"regexp"
	"sort"

	"cmd/compile/internal/base"
	"cmd/compile/internal/midway"
	"cmd/compile/internal/rangefunc"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/compile/internal/types2"
	"cmd/internal/src"
)

var versionErrorRx = regexp.MustCompile(`requires go[0-9]+\.[0-9]+ or later`)

// checkFiles configures and runs the types2 checker on the given
// parsed source files and then returns the result.
// The map result value indicates which closures are generated from the bodies of range function loops.
func checkFiles(m posMap, noders []*noder) (*types2.Package, *types2.Info, map[*syntax.FuncLit]bool) {
	if base.SyntaxErrors() != 0 {
		base.ErrorExit()
	}

	// setup and syntax error reporting
	files := make([]*syntax.File, len(noders))
	// fileBaseMap maps all file pos bases back to *syntax.File
	// for checking Go version mismatched.
	fileBaseMap := make(map[*syntax.PosBase]*syntax.File)
	for i, p := range noders {
		files[i] = p.file
		// The file.Pos() is the position of the package clause.
		// If there's a //line directive before that, file.Pos().Base()
		// refers to that directive, not the file itself.
		// Make sure to consistently map back to file base, here and
		// when we look for a file in the conf.Error handler below,
		// otherwise the file may not be found (was go.dev/issue/67141).
		fileBaseMap[p.file.Pos().FileBase()] = p.file
	}

	didMidway := false

recheck:
	// typechecking
	ctxt := types2.NewContext()
	importer := gcimports{
		ctxt:     ctxt,
		packages: make(map[string]*types2.Package),
	}
	conf := types2.Config{
		Context:            ctxt,
		GoVersion:          base.Flag.Lang,
		IgnoreBranchErrors: true, // parser already checked via syntax.CheckBranches mode
		Importer:           &importer,
		Sizes:              types2.SizesFor("gc", buildcfg.GOARCH),
	}
	if base.Flag.ErrorURL {
		conf.ErrorURL = " [go.dev/e/%s]"
	}
	info := &types2.Info{
		StoreTypesInSyntax: true,
		Defs:               make(map[*syntax.Name]types2.Object),
		Uses:               make(map[*syntax.Name]types2.Object),
		Selections:         make(map[*syntax.SelectorExpr]*types2.Selection),
		Implicits:          make(map[syntax.Node]types2.Object),
		Scopes:             make(map[syntax.Node]*types2.Scope),
		Instances:          make(map[*syntax.Name]types2.Instance),
		FileVersions:       make(map[*syntax.PosBase]string),
		// expand as needed
	}
	conf.Error = func(err error) {
		terr := err.(types2.Error)
		msg := terr.Msg
		if versionErrorRx.MatchString(msg) {
			fileBase := terr.Pos.FileBase()
			fileVersion := info.FileVersions[fileBase]
			file := fileBaseMap[fileBase]
			if file == nil {
				// This should never happen, but be careful and don't crash.
			} else if file.GoVersion == fileVersion {
				// If we have a version error caused by //go:build, report it.
				msg = fmt.Sprintf("%s (file declares //go:build %s)", msg, fileVersion)
			} else {
				// Otherwise, hint at the -lang setting.
				msg = fmt.Sprintf("%s (-lang was set to %s; check go.mod)", msg, base.Flag.Lang)
			}
		}
		base.ErrorfAt(m.makeXPos(terr.Pos), terr.Code, "%s", msg)
	}

	pkg, err := conf.Check(base.Ctxt.Pkgpath, files, info)
	base.ExitIfErrors()
	if err != nil {
		base.FatalfAt(src.NoXPos, "conf.Check error: %v", err)
	}

	// Check for anonymous interface cycles (#56103).
	// TODO(gri) move this code into the type checkers (types2 and go/types)
	var f cycleFinder
	for _, file := range files {
		syntax.Inspect(file, func(n syntax.Node) bool {
			if n, ok := n.(*syntax.InterfaceType); ok {
				if f.hasCycle(types2.Unalias(n.GetTypeInfo().Type).(*types2.Interface)) {
					base.ErrorfAt(m.makeXPos(n.Pos()), errors.InvalidTypeCycle, "invalid recursive type: anonymous interface refers to itself (see https://go.dev/issue/56103)")

					for typ := range f.cyclic {
						f.cyclic[typ] = false // suppress duplicate errors
					}
				}
				return false
			}
			return true
		})
	}
	base.ExitIfErrors()

	// Implementation restriction: we don't allow not-in-heap types to
	// be used as type arguments (#54765).
	{
		type nihTarg struct {
			pos src.XPos
			typ types2.Type
		}
		var nihTargs []nihTarg

		for name, inst := range info.Instances {
			for i := 0; i < inst.TypeArgs.Len(); i++ {
				if targ := inst.TypeArgs.At(i); isNotInHeap(targ) {
					nihTargs = append(nihTargs, nihTarg{m.makeXPos(name.Pos()), targ})
				}
			}
		}
		sort.Slice(nihTargs, func(i, j int) bool {
			ti, tj := nihTargs[i], nihTargs[j]
			return ti.pos.Before(tj.pos)
		})
		for _, targ := range nihTargs {
			base.ErrorfAt(targ.pos, 0, "cannot use incomplete (or unallocatable) type as a type argument: %v", targ.typ)
		}
	}
	base.ExitIfErrors()

	// Implementation restriction: we don't allow not-in-heap types to
	// be used as map keys/values, or channel.
	{
		for _, file := range files {
			syntax.Inspect(file, func(n syntax.Node) bool {
				if n, ok := n.(*syntax.TypeDecl); ok {
					switch n := n.Type.(type) {
					case *syntax.MapType:
						typ := n.GetTypeInfo().Type.Underlying().(*types2.Map)
						if isNotInHeap(typ.Key()) {
							base.ErrorfAt(m.makeXPos(n.Pos()), 0, "incomplete (or unallocatable) map key not allowed")
						}
						if isNotInHeap(typ.Elem()) {
							base.ErrorfAt(m.makeXPos(n.Pos()), 0, "incomplete (or unallocatable) map value not allowed")
						}
					case *syntax.ChanType:
						typ := n.GetTypeInfo().Type.Underlying().(*types2.Chan)
						if isNotInHeap(typ.Elem()) {
							base.ErrorfAt(m.makeXPos(n.Pos()), 0, "chan of incomplete (or unallocatable) type not allowed")
						}
					}
				}
				return true
			})
		}
	}
	base.ExitIfErrors()

	if len(base.Debug.AstDump) > 0 {
		dumpSyntax(pkg, info, files, "checked")
	}

	if buildcfg.Experiment.SIMD && !didMidway {
		didMidway = true
		// Perform midway transformation on AST directly
		if midway.RewriteWrapper(pkg, info, files) {
			// midway made changes; type checking must be repeated.
			if len(base.Debug.AstDump) > 0 {
				// TODO how should this interact with -W and textual dumps
				dumpSyntax(pkg, info, files, "midway before recheck")
			}
			// necessary to reset type checking
			for _, p := range types.PkgMap() {
				p.Direct = false
			}
			// necessary to reset type checking
			typecheck.Target.Imports = nil
			goto recheck
		}
	}

	if len(base.Debug.AstDump) > 0 {
		dumpSyntax(pkg, info, files, "midway after recheck")
	}

	// Rewrite range over function to explicit function calls
	// with the loop bodies converted into new implicit closures.
	// We do this now, before serialization to unified IR, so that if the
	// implicit closures are inlined, we will have the unified IR form.
	// If we do the rewrite in the back end, like between typecheck and walk,
	// then the new implicit closure will not have a unified IR inline body,
	// and bodyReaderFor will fail.
	rangeInfo := rangefunc.Rewrite(pkg, info, files)

	if len(base.Debug.AstDump) > 0 {
		dumpSyntax(pkg, info, files, "rangefunc")
	}

	return pkg, info, rangeInfo
}

func dumpSyntax(pkg *types2.Package, info *types2.Info, files []*syntax.File, phase string) {
	for _, file := range files {
		for _, decl := range file.DeclList {
			if fn, ok := decl.(*syntax.FuncDecl); ok {
				if MatchASTDump(fn) {
					DumpNodeHTML(pkg, file, info, fn, phase, fn)
				}
			}
		}
	}
}

// A cycleFinder detects anonymous interface cycles (go.dev/issue/56103).
type cycleFinder struct {
	cyclic map[*types2.Interface]bool
}

// hasCycle reports whether typ is part of an anonymous interface cycle.
func (f *cycleFinder) hasCycle(typ *types2.Interface) bool {
	// We use Method instead of ExplicitMethod to implicitly expand any
	// embedded interfaces. Then we just need to walk any anonymous
	// types, keeping track of *types2.Interface types we visit along
	// the way.
	for i := 0; i < typ.NumMethods(); i++ {
		if f.visit(typ.Method(i).Type()) {
			return true
		}
	}
	return false
}

// visit recursively walks typ0 to check any referenced interface types.
func (f *cycleFinder) visit(typ0 types2.Type) bool {
	for { // loop for tail recursion
		switch typ := types2.Unalias(typ0).(type) {
		default:
			base.Fatalf("unexpected type: %T", typ)

		case *types2.Basic, *types2.Named, *types2.TypeParam:
			return false // named types cannot be part of an anonymous cycle
		case *types2.Pointer:
			typ0 = typ.Elem()
		case *types2.Array:
			typ0 = typ.Elem()
		case *types2.Chan:
			typ0 = typ.Elem()
		case *types2.Map:
			if f.visit(typ.Key()) {
				return true
			}
			typ0 = typ.Elem()
		case *types2.Slice:
			typ0 = typ.Elem()

		case *types2.Struct:
			for i := 0; i < typ.NumFields(); i++ {
				if f.visit(typ.Field(i).Type()) {
					return true
				}
			}
			return false

		case *types2.Interface:
			// The empty interface (e.g., "any") cannot be part of a cycle.
			if typ.NumExplicitMethods() == 0 && typ.NumEmbeddeds() == 0 {
				return false
			}

			// As an optimization, we wait to allocate cyclic here, after
			// we've found at least one other (non-empty) anonymous
			// interface. This means when a cycle is present, we need to
			// make an extra recursive call to actually detect it. But for
			// most packages, it allows skipping the map allocation
			// entirely.
			if x, ok := f.cyclic[typ]; ok {
				return x
			}
			if f.cyclic == nil {
				f.cyclic = make(map[*types2.Interface]bool)
			}
			f.cyclic[typ] = true
			if f.hasCycle(typ) {
				return true
			}
			f.cyclic[typ] = false
			return false

		case *types2.Signature:
			return f.visit(typ.Params()) || f.visit(typ.Results())
		case *types2.Tuple:
			for i := 0; i < typ.Len(); i++ {
				if f.visit(typ.At(i).Type()) {
					return true
				}
			}
			return false
		}
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/lex.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"fmt"
	"internal/buildcfg"
	"strings"

	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
)

func isSpace(c rune) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isQuoted(s string) bool {
	return len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"'
}

const (
	funcPragmas = ir.Nointerface |
		ir.Noescape |
		ir.Norace |
		ir.Nosplit |
		ir.Noinline |
		ir.NoCheckPtr |
		ir.RegisterParams | // TODO(register args) remove after register abi is working
		ir.CgoUnsafeArgs |
		ir.UintptrKeepAlive |
		ir.UintptrEscapes |
		ir.Systemstack |
		ir.Nowritebarrier |
		ir.Nowritebarrierrec |
		ir.Yeswritebarrierrec
)

func pragmaFlag(verb string) ir.PragmaFlag {
	switch verb {
	case "go:build":
		return ir.GoBuildPragma
	case "go:nointerface":
		if buildcfg.Experiment.FieldTrack {
			return ir.Nointerface
		}
	case "go:noescape":
		return ir.Noescape
	case "go:norace":
		return ir.Norace
	case "go:nosplit":
		return ir.Nosplit | ir.NoCheckPtr // implies NoCheckPtr (see #34972)
	case "go:noinline":
		return ir.Noinline
	case "go:nocheckptr":
		return ir.NoCheckPtr
	case "go:systemstack":
		return ir.Systemstack
	case "go:nowritebarrier":
		return ir.Nowritebarrier
	case "go:nowritebarrierrec":
		return ir.Nowritebarrierrec | ir.Nowritebarrier // implies Nowritebarrier
	case "go:yeswritebarrierrec":
		return ir.Yeswritebarrierrec
	case "go:cgo_unsafe_args":
		return ir.CgoUnsafeArgs | ir.NoCheckPtr // implies NoCheckPtr (see #34968)
	case "go:uintptrkeepalive":
		return ir.UintptrKeepAlive
	case "go:uintptrescapes":
		// This directive extends //go:uintptrkeepalive by forcing
		// uintptr arguments to escape to the heap, which makes stack
		// growth safe.
		return ir.UintptrEscapes | ir.UintptrKeepAlive // implies UintptrKeepAlive
	case "go:registerparams": // TODO(register args) remove after register abi is working
		return ir.RegisterParams
	}
	return 0
}

// pragcgo is called concurrently if files are parsed concurrently.
func (p *noder) pragcgo(pos syntax.Pos, text string) {
	f := pragmaFields(text)

	verb := strings.TrimPrefix(f[0], "go:")
	f[0] = verb

	switch verb {
	case "cgo_export_static", "cgo_export_dynamic":
		switch {
		case len(f) == 2 && !isQuoted(f[1]):
		case len(f) == 3 && !isQuoted(f[1]) && !isQuoted(f[2]):
		default:
			p.error(syntax.Error{Pos: pos, Msg: fmt.Sprintf(`usage: //go:%s local [remote]`, verb)})
			return
		}
	case "cgo_import_dynamic":
		switch {
		case len(f) == 2 && !isQuoted(f[1]):
		case len(f) == 3 && !isQuoted(f[1]) && !isQuoted(f[2]):
		case len(f) == 4 && !isQuoted(f[1]) && !isQuoted(f[2]) && isQuoted(f[3]):
			f[3] = strings.Trim(f[3], `"`)
			if buildcfg.GOOS == "aix" && f[3] != "" {
				// On Aix, library pattern must be "lib.a/object.o"
				// or "lib.a/libname.so.X"
				n := strings.Split(f[3], "/")
				if len(n) != 2 || !strings.HasSuffix(n[0], ".a") || (!strings.HasSuffix(n[1], ".o") && !strings.Contains(n[1], ".so.")) {
					p.error(syntax.Error{Pos: pos, Msg: `usage: //go:cgo_import_dynamic local [remote ["lib.a/object.o"]]`})
					return
				}
			}
		default:
			p.error(syntax.Error{Pos: pos, Msg: `usage: //go:cgo_import_dynamic local [remote ["library"]]`})
			return
		}
	case "cgo_import_static":
		switch {
		case len(f) == 2 && !isQuoted(f[1]):
		default:
			p.error(syntax.Error{Pos: pos, Msg: `usage: //go:cgo_import_static local`})
			return
		}
	case "cgo_dynamic_linker":
		switch {
		case len(f) == 2 && isQuoted(f[1]):
			f[1] = strings.Trim(f[1], `"`)
		default:
			p.error(syntax.Error{Pos: pos, Msg: `usage: //go:cgo_dynamic_linker "path"`})
			return
		}
	case "cgo_ldflag":
		switch {
		case len(f) == 2 && isQuoted(f[1]):
			f[1] = strings.Trim(f[1], `"`)
		default:
			p.error(syntax.Error{Pos: pos, Msg: `usage: //go:cgo_ldflag "arg"`})
			return
		}
	default:
		return
	}
	p.pragcgobuf = append(p.pragcgobuf, f)
}

// pragmaFields is similar to strings.FieldsFunc(s, isSpace)
// but does not split when inside double quoted regions and always
// splits before the start and after the end of a double quoted region.
// pragmaFields does not recognize escaped quotes. If a quote in s is not
// closed the part after the opening quote will not be returned as a field.
func pragmaFields(s string) []string {
	var a []string
	inQuote := false
	fieldStart := -1 // Set to -1 when looking for start of field.
	for i, c := range s {
		switch {
		case c == '"':
			if inQuote {
				inQuote = false
				a = append(a, s[fieldStart:i+1])
				fieldStart = -1
			} else {
				inQuote = true
				if fieldStart >= 0 {
					a = append(a, s[fieldStart:i])
				}
				fieldStart = i
			}
		case !inQuote && isSpace(c):
			if fieldStart >= 0 {
				a = append(a, s[fieldStart:i])
				fieldStart = -1
			}
		default:
			if fieldStart == -1 {
				fieldStart = i
			}
		}
	}
	if !inQuote && fieldStart >= 0 { // Last field might end at the end of the string.
		a = append(a, s[fieldStart:])
	}
	return a
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/linker.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"internal/buildcfg"
	"internal/pkgbits"
	"io"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/types"
	"cmd/internal/goobj"
	"cmd/internal/obj"
)

// This file implements the unified IR linker, which combines the
// local package's stub data with imported package data to produce a
// complete export data file. It also rewrites the compiler's
// extension data sections based on the results of compilation (e.g.,
// the function inlining cost and linker symbol index assignments).
//
// TODO(mdempsky): Using the name "linker" here is confusing, because
// readers are likely to mistake references to it for cmd/link. But
// there's a shortage of good names for "something that combines
// multiple parts into a cohesive whole"... e.g., "assembler" and
// "compiler" are also already taken.

// TODO(mdempsky): Should linker go into pkgbits? Probably the
// low-level linking details can be moved there, but the logic for
// handling extension data needs to stay in the compiler.

// A linker combines a package's stub export data with any referenced
// elements from imported packages into a single, self-contained
// export data file.
type linker struct {
	pw pkgbits.PkgEncoder

	pkgs   map[string]index
	decls  map[*types.Sym]index
	bodies map[*types.Sym]index
}

// relocAll ensures that all elements specified by pr and relocs are
// copied into the output export data file, and returns the
// corresponding indices in the output.
func (l *linker) relocAll(pr *pkgReader, relocs []pkgbits.RefTableEntry) []pkgbits.RefTableEntry {
	res := make([]pkgbits.RefTableEntry, len(relocs))
	for i, rent := range relocs {
		rent.Idx = l.relocIdx(pr, rent.Kind, rent.Idx)
		res[i] = rent
	}
	return res
}

// relocIdx ensures a single element is copied into the output export
// data file, and returns the corresponding index in the output.
func (l *linker) relocIdx(pr *pkgReader, k pkgbits.SectionKind, idx index) index {
	assert(pr != nil)

	absIdx := pr.AbsIdx(k, idx)

	if newidx := pr.newindex[absIdx]; newidx != 0 {
		return ^newidx
	}

	var newidx index
	switch k {
	case pkgbits.SectionString:
		newidx = l.relocString(pr, idx)
	case pkgbits.SectionPkg:
		newidx = l.relocPkg(pr, idx)
	case pkgbits.SectionObj:
		newidx = l.relocObj(pr, idx)

	default:
		// Generic relocations.
		//
		// TODO(mdempsky): Deduplicate more sections? In fact, I think
		// every section could be deduplicated. This would also be easier
		// if we do external relocations.

		w := l.pw.NewEncoderRaw(k)
		l.relocCommon(pr, w, k, idx)
		newidx = w.Idx
	}

	pr.newindex[absIdx] = ^newidx

	return newidx
}

// relocString copies the specified string from pr into the output
// export data file, deduplicating it against other strings.
func (l *linker) relocString(pr *pkgReader, idx index) index {
	return l.pw.StringIdx(pr.StringIdx(idx))
}

// relocPkg copies the specified package from pr into the output
// export data file, rewriting its import path to match how it was
// imported.
//
// TODO(mdempsky): Since CL 391014, we already have the compilation
// unit's import path, so there should be no need to rewrite packages
// anymore.
func (l *linker) relocPkg(pr *pkgReader, idx index) index {
	path := pr.PeekPkgPath(idx)

	if newidx, ok := l.pkgs[path]; ok {
		return newidx
	}

	r := pr.NewDecoder(pkgbits.SectionPkg, idx, pkgbits.SyncPkgDef)
	w := l.pw.NewEncoder(pkgbits.SectionPkg, pkgbits.SyncPkgDef)
	l.pkgs[path] = w.Idx

	// TODO(mdempsky): We end up leaving an empty string reference here
	// from when the package was originally written as "". Probably not
	// a big deal, but a little annoying. Maybe relocating
	// cross-references in place is the way to go after all.
	w.Relocs = l.relocAll(pr, r.Relocs)

	_ = r.String() // original path
	w.String(path)

	io.Copy(&w.Data, &r.Data)

	return w.Flush()
}

// relocObj copies the specified object from pr into the output export
// data file, rewriting its compiler-private extension data (e.g.,
// adding inlining cost and escape analysis results for functions).
func (l *linker) relocObj(pr *pkgReader, idx index) index {
	path, name, tag := pr.PeekObj(idx)
	sym := types.NewPkg(path, "").Lookup(name)

	if newidx, ok := l.decls[sym]; ok {
		return newidx
	}

	if tag == pkgbits.ObjStub && path != "builtin" && path != "unsafe" {
		pri, ok := objReader[sym]
		if !ok {
			base.Fatalf("missing reader for %q.%v", path, name)
		}
		assert(ok)

		pr = pri.pr
		idx = pri.idx

		path2, name2, tag2 := pr.PeekObj(idx)
		sym2 := types.NewPkg(path2, "").Lookup(name2)
		assert(sym == sym2)
		assert(tag2 != pkgbits.ObjStub)
	}

	w := l.pw.NewEncoderRaw(pkgbits.SectionObj)
	wext := l.pw.NewEncoderRaw(pkgbits.SectionObjExt)
	wname := l.pw.NewEncoderRaw(pkgbits.SectionName)
	wdict := l.pw.NewEncoderRaw(pkgbits.SectionObjDict)

	l.decls[sym] = w.Idx
	assert(wext.Idx == w.Idx)
	assert(wname.Idx == w.Idx)
	assert(wdict.Idx == w.Idx)

	l.relocCommon(pr, w, pkgbits.SectionObj, idx)
	l.relocCommon(pr, wname, pkgbits.SectionName, idx)
	l.relocCommon(pr, wdict, pkgbits.SectionObjDict, idx)

	// Generic types and functions won't have definitions, and imported
	// objects may not either.
	obj, _ := sym.Def.(*ir.Name)
	local := sym.Pkg == types.LocalPkg

	if local && obj != nil {
		wext.Sync(pkgbits.SyncObject1)
		switch tag {
		case pkgbits.ObjFunc:
			l.relocFuncExt(wext, obj)
		case pkgbits.ObjType:
			l.relocTypeExt(wext, obj)
		case pkgbits.ObjVar:
			l.relocVarExt(wext, obj)
		}
		wext.Flush()
	} else {
		l.relocCommon(pr, wext, pkgbits.SectionObjExt, idx)
	}

	// Check if we need to export the inline bodies for functions and
	// methods.
	if obj != nil {
		if obj.Op() == ir.ONAME && obj.Class == ir.PFUNC {
			l.exportBody(obj, local)
		}

		if obj.Op() == ir.OTYPE && !obj.Alias() {
			if typ := obj.Type(); !typ.IsInterface() {
				for _, method := range typ.Methods() {
					l.exportBody(method.Nname.(*ir.Name), local)
				}
			}
		}
	}

	return w.Idx
}

// exportBody exports the given function or method's body, if
// appropriate. local indicates whether it's a local function or
// method available on a locally declared type. (Due to cross-package
// type aliases, a method may be imported, but still available on a
// locally declared type.)
func (l *linker) exportBody(obj *ir.Name, local bool) {
	assert(obj.Op() == ir.ONAME && obj.Class == ir.PFUNC)

	fn := obj.Func
	if fn.Inl == nil {
		return // not inlinable anyway
	}

	// As a simple heuristic, if the function was declared in this
	// package or we inlined it somewhere in this package, then we'll
	// (re)export the function body. This isn't perfect, but seems
	// reasonable in practice. In particular, it has the nice property
	// that in the worst case, adding a blank import ensures the
	// function body is available for inlining.
	//
	// TODO(mdempsky): Reimplement the reachable method crawling logic
	// from typecheck/crawler.go.
	exportBody := local || fn.Inl.HaveDcl
	if !exportBody {
		return
	}

	sym := obj.Sym()
	if _, ok := l.bodies[sym]; ok {
		// Due to type aliases, we might visit methods multiple times.
		base.AssertfAt(obj.Type().Recv() != nil, obj.Pos(), "expected method: %v", obj)
		return
	}

	pri, ok := bodyReaderFor(fn)
	assert(ok)
	l.bodies[sym] = l.relocIdx(pri.pr, pkgbits.SectionBody, pri.idx)
}

// relocCommon copies the specified element from pr into w,
// recursively relocating any referenced elements as well.
func (l *linker) relocCommon(pr *pkgReader, w *pkgbits.Encoder, k pkgbits.SectionKind, idx index) {
	r := pr.NewDecoderRaw(k, idx)
	w.Relocs = l.relocAll(pr, r.Relocs)
	io.Copy(&w.Data, &r.Data)
	w.Flush()
}

func (l *linker) pragmaFlag(w *pkgbits.Encoder, pragma ir.PragmaFlag) {
	w.Sync(pkgbits.SyncPragma)
	w.Int(int(pragma))
}

func (l *linker) relocFuncExt(w *pkgbits.Encoder, name *ir.Name) {
	w.Sync(pkgbits.SyncFuncExt)

	l.pragmaFlag(w, name.Func.Pragma)
	l.linkname(w, name)

	if buildcfg.GOARCH == "wasm" {
		if name.Func.WasmImport != nil {
			w.String(name.Func.WasmImport.Module)
			w.String(name.Func.WasmImport.Name)
		} else {
			w.String("")
			w.String("")
		}
		if name.Func.WasmExport != nil {
			w.String(name.Func.WasmExport.Name)
		} else {
			w.String("")
		}
	}

	// Relocated extension data.
	w.Bool(true)

	// Record definition ABI so cross-ABI calls can be direct.
	// This is important for the performance of calling some
	// common functions implemented in assembly (e.g., bytealg).
	w.Uint64(uint64(name.Func.ABI))

	// Escape analysis.
	for _, f := range name.Type().RecvParams() {
		w.String(f.Note)
	}

	if inl := name.Func.Inl; w.Bool(inl != nil) {
		w.Len(int(inl.Cost))
		w.Bool(inl.CanDelayResults)
		if buildcfg.Experiment.NewInliner {
			w.String(inl.Properties)
		}
	}

	w.Sync(pkgbits.SyncEOF)
}

func (l *linker) relocTypeExt(w *pkgbits.Encoder, name *ir.Name) {
	w.Sync(pkgbits.SyncTypeExt)

	typ := name.Type()

	l.pragmaFlag(w, name.Pragma())

	// For type T, export the index of type descriptor symbols of T and *T.
	l.lsymIdx(w, "", reflectdata.TypeLinksym(typ))
	l.lsymIdx(w, "", reflectdata.TypeLinksym(typ.PtrTo()))

	if typ.Kind() != types.TINTER {
		for _, method := range typ.Methods() {
			l.relocFuncExt(w, method.Nname.(*ir.Name))
		}
	}
}

func (l *linker) relocVarExt(w *pkgbits.Encoder, name *ir.Name) {
	w.Sync(pkgbits.SyncVarExt)
	l.linkname(w, name)
}

func (l *linker) linkname(w *pkgbits.Encoder, name *ir.Name) {
	w.Sync(pkgbits.SyncLinkname)

	linkname := name.Sym().Linkname
	if !l.lsymIdx(w, linkname, name.Linksym()) {
		w.String(linkname)
		w.Bool(name.Linksym().IsLinknameStd())
	}
}

func (l *linker) lsymIdx(w *pkgbits.Encoder, linkname string, lsym *obj.LSym) bool {
	if lsym.PkgIdx > goobj.PkgIdxSelf || (lsym.PkgIdx == goobj.PkgIdxInvalid && !lsym.Indexed()) || linkname != "" {
		w.Int64(-1)
		return false
	}

	// For a defined symbol, export its index.
	// For re-exporting an imported symbol, pass its index through.
	w.Int64(int64(lsym.SymIdx))
	return true
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/noder.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"errors"
	"fmt"
	"internal/buildcfg"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/objabi"
)

func LoadPackage(filenames []string) {
	base.Timer.Start("fe", "parse")

	// Limit the number of simultaneously open files.
	sem := make(chan struct{}, runtime.GOMAXPROCS(0)+10)

	noders := make([]*noder, len(filenames))
	for i := range noders {
		p := noder{
			err: make(chan syntax.Error),
		}
		noders[i] = &p
	}

	// Move the entire syntax processing logic into a separate goroutine to avoid blocking on the "sem".
	go func() {
		for i, filename := range filenames {
			p := noders[i]
			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()
				defer close(p.err)
				fbase := syntax.NewFileBase(filename)

				f, err := os.Open(filename)
				if err != nil {
					p.error(syntax.Error{Msg: err.Error()})
					return
				}
				defer f.Close()

				p.file, _ = syntax.Parse(fbase, f, p.error, p.pragma, syntax.CheckBranches) // errors are tracked via p.error
			}()
		}
	}()

	var lines uint
	var m posMap
	for _, p := range noders {
		for e := range p.err {
			base.ErrorfAt(m.makeXPos(e.Pos), 0, "%s", e.Msg)
		}
		if p.file == nil {
			base.ErrorExit()
		}
		lines += p.file.EOF.Line()
	}
	base.Timer.AddEvent(int64(lines), "lines")

	unified(m, noders)
}

// trimFilename returns the "trimmed" filename of b, which is the
// absolute filename after applying -trimpath processing. This
// filename form is suitable for use in object files and export data.
//
// If b's filename has already been trimmed (i.e., because it was read
// in from an imported package's export data), then the filename is
// returned unchanged.
func trimFilename(b *syntax.PosBase) string {
	filename := b.Filename()
	if !b.Trimmed() {
		dir := ""
		if b.IsFileBase() {
			dir = base.Ctxt.Pathname
		}
		filename = objabi.AbsFile(dir, filename, base.Flag.TrimPath)
	}
	return filename
}

// noder transforms package syntax's AST into a Node tree.
type noder struct {
	file       *syntax.File
	linknames  []linkname
	pragcgobuf [][]string
	err        chan syntax.Error
}

// linkname records a //go:linkname or //go:linknamestd directive.
type linkname struct {
	pos    syntax.Pos
	std    bool
	local  string
	remote string
}

var unOps = [...]ir.Op{
	syntax.Recv: ir.ORECV,
	syntax.Mul:  ir.ODEREF,
	syntax.And:  ir.OADDR,

	syntax.Not: ir.ONOT,
	syntax.Xor: ir.OBITNOT,
	syntax.Add: ir.OPLUS,
	syntax.Sub: ir.ONEG,
}

var binOps = [...]ir.Op{
	syntax.OrOr:   ir.OOROR,
	syntax.AndAnd: ir.OANDAND,

	syntax.Eql: ir.OEQ,
	syntax.Neq: ir.ONE,
	syntax.Lss: ir.OLT,
	syntax.Leq: ir.OLE,
	syntax.Gtr: ir.OGT,
	syntax.Geq: ir.OGE,

	syntax.Add: ir.OADD,
	syntax.Sub: ir.OSUB,
	syntax.Or:  ir.OOR,
	syntax.Xor: ir.OXOR,

	syntax.Mul:    ir.OMUL,
	syntax.Div:    ir.ODIV,
	syntax.Rem:    ir.OMOD,
	syntax.And:    ir.OAND,
	syntax.AndNot: ir.OANDNOT,
	syntax.Shl:    ir.OLSH,
	syntax.Shr:    ir.ORSH,
}

// error is called concurrently if files are parsed concurrently.
func (p *noder) error(err error) {
	p.err <- err.(syntax.Error)
}

// pragmas that are allowed in the std lib, but don't have
// a syntax.Pragma value (see lex.go) associated with them.
var allowedStdPragmas = map[string]bool{
	"go:cgo_export_static":  true,
	"go:cgo_export_dynamic": true,
	"go:cgo_import_static":  true,
	"go:cgo_import_dynamic": true,
	"go:cgo_ldflag":         true,
	"go:cgo_dynamic_linker": true,
	"go:embed":              true,
	"go:fix":                true,
	"go:generate":           true,
}

// *pragmas is the value stored in a syntax.pragmas during parsing.
type pragmas struct {
	Flag       ir.PragmaFlag // collected bits
	Pos        []pragmaPos   // position of each individual flag
	Embeds     []pragmaEmbed
	WasmImport *WasmImport
	WasmExport *WasmExport
}

func (p *pragmas) Nointerface() bool {
	return p.Flag&ir.Nointerface != 0
}

// WasmImport stores metadata associated with the //go:wasmimport pragma
type WasmImport struct {
	Pos    syntax.Pos
	Module string
	Name   string
}

// WasmExport stores metadata associated with the //go:wasmexport pragma
type WasmExport struct {
	Pos  syntax.Pos
	Name string
}

type pragmaPos struct {
	Flag ir.PragmaFlag
	Pos  syntax.Pos
}

type pragmaEmbed struct {
	Pos      syntax.Pos
	Patterns []string
}

func (p *noder) checkUnusedDuringParse(pragma *pragmas) {
	for _, pos := range pragma.Pos {
		if pos.Flag&pragma.Flag != 0 {
			p.error(syntax.Error{Pos: pos.Pos, Msg: "misplaced compiler directive"})
		}
	}
	if len(pragma.Embeds) > 0 {
		for _, e := range pragma.Embeds {
			p.error(syntax.Error{Pos: e.Pos, Msg: "misplaced go:embed directive"})
		}
	}
	if pragma.WasmImport != nil {
		p.error(syntax.Error{Pos: pragma.WasmImport.Pos, Msg: "misplaced go:wasmimport directive"})
	}
	if pragma.WasmExport != nil {
		p.error(syntax.Error{Pos: pragma.WasmExport.Pos, Msg: "misplaced go:wasmexport directive"})
	}
}

// pragma is called concurrently if files are parsed concurrently.
func (p *noder) pragma(pos syntax.Pos, blankLine bool, text string, old syntax.Pragma) syntax.Pragma {
	pragma, _ := old.(*pragmas)
	if pragma == nil {
		pragma = new(pragmas)
	}

	if text == "" {
		// unused pragma; only called with old != nil.
		p.checkUnusedDuringParse(pragma)
		return nil
	}

	if strings.HasPrefix(text, "line ") {
		// line directives are handled by syntax package
		panic("unreachable")
	}

	if !blankLine {
		// directive must be on line by itself
		p.error(syntax.Error{Pos: pos, Msg: "misplaced compiler directive"})
		return pragma
	}

	switch {
	case strings.HasPrefix(text, "go:wasmimport "):
		f := strings.Fields(text)
		if len(f) != 3 {
			p.error(syntax.Error{Pos: pos, Msg: "usage: //go:wasmimport importmodule importname"})
			break
		}

		if buildcfg.GOARCH == "wasm" {
			// Only actually use them if we're compiling to WASM though.
			pragma.WasmImport = &WasmImport{
				Pos:    pos,
				Module: f[1],
				Name:   f[2],
			}
		}

	case strings.HasPrefix(text, "go:wasmexport "):
		f := strings.Fields(text)
		if len(f) != 2 {
			// TODO: maybe make the name optional? It was once mentioned on proposal 65199.
			p.error(syntax.Error{Pos: pos, Msg: "usage: //go:wasmexport exportname"})
			break
		}

		if buildcfg.GOARCH == "wasm" {
			// Only actually use them if we're compiling to WASM though.
			pragma.WasmExport = &WasmExport{
				Pos:  pos,
				Name: f[1],
			}
		}

	case strings.HasPrefix(text, "go:linkname "), strings.HasPrefix(text, "go:linknamestd "):
		f := strings.Fields(text)
		if !(2 <= len(f) && len(f) <= 3) {
			p.error(syntax.Error{Pos: pos, Msg: fmt.Sprintf("usage: //%s localname [linkname]", f[0])})
			break
		}
		// The second argument is optional. If omitted, we use
		// the default object symbol name for this and
		// linkname only serves to mark this symbol as
		// something that may be referenced via the object
		// symbol name from another package.
		var target string
		if len(f) == 3 {
			target = f[2]
		} else if base.Ctxt.Pkgpath != "" {
			// Use the default object symbol name if the
			// user didn't provide one.
			target = objabi.PathToPrefix(base.Ctxt.Pkgpath) + "." + f[1]
		} else {
			panic("missing pkgpath")
		}
		p.linknames = append(p.linknames, linkname{pos, f[0] == "go:linknamestd", f[1], target})

	case text == "go:embed", strings.HasPrefix(text, "go:embed "):
		args, err := parseGoEmbed(text[len("go:embed"):])
		if err != nil {
			p.error(syntax.Error{Pos: pos, Msg: err.Error()})
		}
		if len(args) == 0 {
			p.error(syntax.Error{Pos: pos, Msg: "usage: //go:embed pattern..."})
			break
		}
		pragma.Embeds = append(pragma.Embeds, pragmaEmbed{pos, args})

	case strings.HasPrefix(text, "go:cgo_import_dynamic "):
		// This is permitted for general use because Solaris
		// code relies on it in golang.org/x/sys/unix and others.
		fields := pragmaFields(text)
		if len(fields) >= 4 {
			lib := strings.Trim(fields[3], `"`)
			if lib != "" && !safeArg(lib) && !isCgoGeneratedFile(pos) {
				p.error(syntax.Error{Pos: pos, Msg: fmt.Sprintf("invalid library name %q in cgo_import_dynamic directive", lib)})
			}
			p.pragcgo(pos, text)
			pragma.Flag |= pragmaFlag("go:cgo_import_dynamic")
			break
		}
		fallthrough
	case strings.HasPrefix(text, "go:cgo_"):
		// For security, we disallow //go:cgo_* directives other
		// than cgo_import_dynamic outside cgo-generated files.
		// Exception: they are allowed in the standard library, for runtime and syscall.
		if !isCgoGeneratedFile(pos) && !base.Flag.Std {
			p.error(syntax.Error{Pos: pos, Msg: fmt.Sprintf("//%s only allowed in cgo-generated code", text)})
		}
		p.pragcgo(pos, text)
		fallthrough // because of //go:cgo_unsafe_args
	default:
		verb := text
		if i := strings.Index(text, " "); i >= 0 {
			verb = verb[:i]
		}
		flag := pragmaFlag(verb)
		const runtimePragmas = ir.Systemstack | ir.Nowritebarrier | ir.Nowritebarrierrec | ir.Yeswritebarrierrec
		if !base.Flag.CompilingRuntime && flag&runtimePragmas != 0 {
			p.error(syntax.Error{Pos: pos, Msg: fmt.Sprintf("//%s only allowed in runtime", verb)})
		}
		if flag == ir.UintptrKeepAlive && !base.Flag.Std {
			p.error(syntax.Error{Pos: pos, Msg: fmt.Sprintf("//%s is only allowed in the standard library", verb)})
		}
		if flag == 0 && !allowedStdPragmas[verb] && base.Flag.Std {
			p.error(syntax.Error{Pos: pos, Msg: fmt.Sprintf("//%s is not allowed in the standard library", verb)})
		}
		pragma.Flag |= flag
		pragma.Pos = append(pragma.Pos, pragmaPos{flag, pos})
	}

	return pragma
}

// isCgoGeneratedFile reports whether pos is in a file
// generated by cgo, which is to say a file with name
// beginning with "_cgo_". Such files are allowed to
// contain cgo directives, and for security reasons
// (primarily misuse of linker flags), other files are not.
// See golang.org/issue/23672.
// Note that cmd/go ignores files whose names start with underscore,
// so the only _cgo_ files we will see from cmd/go are generated by cgo.
// It's easy to bypass this check by calling the compiler directly;
// we only protect against uses by cmd/go.
func isCgoGeneratedFile(pos syntax.Pos) bool {
	// We need the absolute file, independent of //line directives,
	// so we call pos.Base().Pos().
	return strings.HasPrefix(filepath.Base(trimFilename(pos.Base().Pos().Base())), "_cgo_")
}

// safeArg reports whether arg is a "safe" command-line argument,
// meaning that when it appears in a command-line, it probably
// doesn't have some special meaning other than its own name.
// This is copied from SafeArg in cmd/go/internal/load/pkg.go.
func safeArg(name string) bool {
	if name == "" {
		return false
	}
	c := name[0]
	return '0' <= c && c <= '9' || 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || c == '.' || c == '_' || c == '/' || c >= utf8.RuneSelf
}

// parseGoEmbed parses the text following "//go:embed" to extract the glob patterns.
// It accepts unquoted space-separated patterns as well as double-quoted and back-quoted Go strings.
// go/build/read.go also processes these strings and contains similar logic.
func parseGoEmbed(args string) ([]string, error) {
	var list []string
	for args = strings.TrimSpace(args); args != ""; args = strings.TrimSpace(args) {
		var path string
	Switch:
		switch args[0] {
		default:
			i := len(args)
			for j, c := range args {
				if unicode.IsSpace(c) {
					i = j
					break
				}
			}
			path = args[:i]
			args = args[i:]

		case '`':
			i := strings.Index(args[1:], "`")
			if i < 0 {
				return nil, fmt.Errorf("invalid quoted string in //go:embed: %s", args)
			}
			path = args[1 : 1+i]
			args = args[1+i+1:]

		case '"':
			i := 1
			for ; i < len(args); i++ {
				if args[i] == '\\' {
					i++
					continue
				}
				if args[i] == '"' {
					q, err := strconv.Unquote(args[:i+1])
					if err != nil {
						return nil, fmt.Errorf("invalid quoted string in //go:embed: %s", args[:i+1])
					}
					path = q
					args = args[i+1:]
					break Switch
				}
			}
			if i >= len(args) {
				return nil, fmt.Errorf("invalid quoted string in //go:embed: %s", args)
			}
		}

		if args != "" {
			r, _ := utf8.DecodeRuneInString(args)
			if !unicode.IsSpace(r) {
				return nil, fmt.Errorf("invalid quoted string in //go:embed: %s", args)
			}
		}
		list = append(list, path)
	}
	return list, nil
}

// A function named init is a special case.
// It is called by the initialization before main is run.
// To make it unique within a package and also uncallable,
// the name, normally "pkg.init", is altered to "pkg.init.0".
var renameinitgen int

func Renameinit() *types.Sym {
	s := typecheck.LookupNum("init.", renameinitgen)
	renameinitgen++
	return s
}

func checkEmbed(decl *syntax.VarDecl, haveEmbed, withinFunc bool) error {
	switch {
	case !haveEmbed:
		return errors.New("go:embed requires import \"embed\" (or import _ \"embed\", if package is not used)")
	case len(decl.NameList) > 1:
		return errors.New("go:embed cannot apply to multiple vars")
	case decl.Values != nil:
		return errors.New("go:embed cannot apply to var with initializer")
	case decl.Type == nil:
		// Should not happen, since Values == nil now.
		return errors.New("go:embed cannot apply to var without type")
	case withinFunc:
		return errors.New("go:embed cannot apply to var inside func")
	case !types.AllowsGoVersion(1, 16):
		return fmt.Errorf("go:embed requires go1.16 or later (-lang was set to %s; check go.mod)", base.Flag.Lang)

	default:
		return nil
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/posmap.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/syntax"
	"cmd/internal/src"
)

// A posMap handles mapping from syntax.Pos to src.XPos.
type posMap struct {
	bases map[*syntax.PosBase]*src.PosBase
	cache struct {
		last *syntax.PosBase
		base *src.PosBase
	}
}

type poser interface{ Pos() syntax.Pos }
type ender interface{ End() syntax.Pos }

func (m *posMap) pos(p poser) src.XPos { return m.makeXPos(p.Pos()) }

func (m *posMap) makeXPos(pos syntax.Pos) src.XPos {
	// Predeclared objects (e.g., the result parameter for error.Error)
	// do not have a position.
	if !pos.IsKnown() {
		return src.NoXPos
	}

	posBase := m.makeSrcPosBase(pos.Base())
	return base.Ctxt.PosTable.XPos(src.MakePos(posBase, pos.Line(), pos.Col()))
}

// makeSrcPosBase translates from a *syntax.PosBase to a *src.PosBase.
func (m *posMap) makeSrcPosBase(b0 *syntax.PosBase) *src.PosBase {
	// fast path: most likely PosBase hasn't changed
	if m.cache.last == b0 {
		return m.cache.base
	}

	b1, ok := m.bases[b0]
	if !ok {
		fn := b0.Filename()
		absfn := trimFilename(b0)

		if b0.IsFileBase() {
			b1 = src.NewFileBase(fn, absfn)
		} else {
			// line directive base
			p0 := b0.Pos()
			p0b := p0.Base()
			if p0b == b0 {
				panic("infinite recursion in makeSrcPosBase")
			}
			p1 := src.MakePos(m.makeSrcPosBase(p0b), p0.Line(), p0.Col())
			b1 = src.NewLinePragmaBase(p1, fn, absfn, b0.Line(), b0.Col())
		}
		if m.bases == nil {
			m.bases = make(map[*syntax.PosBase]*src.PosBase)
		}
		m.bases[b0] = b1
	}

	// update cache
	m.cache.last = b0
	m.cache.base = b1

	return b1
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/quirks.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"fmt"

	"cmd/compile/internal/syntax"
)

// typeExprEndPos returns the position that noder would leave base.Pos
// after parsing the given type expression.
//
// Deprecated: This function exists to emulate position semantics from
// Go 1.17, necessary for compatibility with the backend DWARF
// generation logic that assigns variables to their appropriate scope.
func typeExprEndPos(expr0 syntax.Expr) syntax.Pos {
	for {
		switch expr := expr0.(type) {
		case *syntax.Name:
			return expr.Pos()
		case *syntax.SelectorExpr:
			return expr.X.Pos()

		case *syntax.ParenExpr:
			expr0 = expr.X

		case *syntax.Operation:
			assert(expr.Op == syntax.Mul)
			assert(expr.Y == nil)
			expr0 = expr.X

		case *syntax.ArrayType:
			expr0 = expr.Elem
		case *syntax.ChanType:
			expr0 = expr.Elem
		case *syntax.DotsType:
			expr0 = expr.Elem
		case *syntax.MapType:
			expr0 = expr.Value
		case *syntax.SliceType:
			expr0 = expr.Elem

		case *syntax.StructType:
			return expr.Pos()

		case *syntax.InterfaceType:
			expr0 = lastFieldType(expr.MethodList)
			if expr0 == nil {
				return expr.Pos()
			}

		case *syntax.FuncType:
			expr0 = lastFieldType(expr.ResultList)
			if expr0 == nil {
				expr0 = lastFieldType(expr.ParamList)
				if expr0 == nil {
					return expr.Pos()
				}
			}

		case *syntax.IndexExpr: // explicit type instantiation
			targs := syntax.UnpackListExpr(expr.Index)
			expr0 = targs[len(targs)-1]

		default:
			panic(fmt.Sprintf("%s: unexpected type expression %v", expr.Pos(), syntax.String(expr)))
		}
	}
}

func lastFieldType(fields []*syntax.Field) syntax.Expr {
	if len(fields) == 0 {
		return nil
	}
	return fields[len(fields)-1].Type
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/reader.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"encoding/hex"
	"fmt"
	"go/constant"
	"internal/buildcfg"
	"internal/pkgbits"
	"path/filepath"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/dwarfgen"
	"cmd/compile/internal/inline"
	"cmd/compile/internal/inline/interleaved"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/pgoir"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/staticinit"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/internal/hash"
	"cmd/internal/obj"
	"cmd/internal/objabi"
	"cmd/internal/src"
)

// This file implements cmd/compile backend's reader for the Unified
// IR export data.

// A pkgReader reads Unified IR export data.
type pkgReader struct {
	pkgbits.PkgDecoder

	// Indices for encoded things; lazily populated as needed.
	//
	// Note: Objects (i.e., ir.Names) are lazily instantiated by
	// populating their types.Sym.Def; see objReader below.

	posBases []*src.PosBase
	pkgs     []*types.Pkg
	typs     []*types.Type

	// offset for rewriting the given (absolute!) index into the output,
	// but bitwise inverted so we can detect if we're missing the entry
	// or not.
	newindex []index
}

func newPkgReader(pr pkgbits.PkgDecoder) *pkgReader {
	return &pkgReader{
		PkgDecoder: pr,

		posBases: make([]*src.PosBase, pr.NumElems(pkgbits.SectionPosBase)),
		pkgs:     make([]*types.Pkg, pr.NumElems(pkgbits.SectionPkg)),
		typs:     make([]*types.Type, pr.NumElems(pkgbits.SectionType)),

		newindex: make([]index, pr.TotalElems()),
	}
}

// A pkgReaderIndex compactly identifies an index (and its
// corresponding dictionary) within a package's export data.
type pkgReaderIndex struct {
	pr        *pkgReader
	idx       index
	dict      *readerDict
	methodSym *types.Sym

	synthetic func(pos src.XPos, r *reader)
}

func (pri pkgReaderIndex) asReader(k pkgbits.SectionKind, marker pkgbits.SyncMarker) *reader {
	if pri.synthetic != nil {
		return &reader{synthetic: pri.synthetic}
	}

	r := pri.pr.newReader(k, pri.idx, marker)
	r.dict = pri.dict
	r.methodSym = pri.methodSym
	return r
}

func (pr *pkgReader) newReader(k pkgbits.SectionKind, idx index, marker pkgbits.SyncMarker) *reader {
	return &reader{
		Decoder: pr.NewDecoder(k, idx, marker),
		p:       pr,
	}
}

// A reader provides APIs for reading an individual element.
type reader struct {
	pkgbits.Decoder

	p *pkgReader

	dict *readerDict

	// funcLitGen is a counter for closure names.
	funcLitGen int
	// rangeLitGen is a counter for range func closure names.
	rangeLitGen int

	// TODO(mdempsky): The state below is all specific to reading
	// function bodies. It probably makes sense to split it out
	// separately so that it doesn't take up space in every reader
	// instance.

	curfn       *ir.Func
	locals      []*ir.Name
	closureVars []*ir.Name

	// funarghack is used during inlining to suppress setting
	// Field.Nname to the inlined copies of the parameters. This is
	// necessary because we reuse the same types.Type as the original
	// function, and most of the compiler still relies on field.Nname to
	// find parameters/results.
	funarghack bool

	// methodSym is the name of method's name, if reading a method.
	// It's nil if reading a normal function or closure body.
	methodSym *types.Sym

	// dictParam is the .dict param, if any.
	dictParam *ir.Name

	// synthetic is a callback function to construct a synthetic
	// function body. It's used for creating the bodies of function
	// literals used to curry arguments to shaped functions.
	synthetic func(pos src.XPos, r *reader)

	// scopeVars is a stack tracking the number of variables declared in
	// the current function at the moment each open scope was opened.
	scopeVars         []int
	marker            dwarfgen.ScopeMarker
	lastCloseScopePos src.XPos

	// === details for handling inline body expansion ===

	// If we're reading in a function body because of inlining, this is
	// the call that we're inlining for.
	inlCaller    *ir.Func
	inlCall      *ir.CallExpr
	inlFunc      *ir.Func
	inlTreeIndex int
	inlPosBases  map[*src.PosBase]*src.PosBase

	// suppressInlPos tracks whether position base rewriting for
	// inlining should be suppressed. See funcLit.
	suppressInlPos int

	delayResults bool

	// Label to return to.
	retlabel *types.Sym
}

// A readerDict represents an instantiated "compile-time dictionary,"
// used for resolving any derived types needed for instantiating a
// generic object.
//
// A compile-time dictionary can either be "shaped" or "non-shaped."
// Shaped compile-time dictionaries are only used for instantiating
// shaped type definitions and function bodies, while non-shaped
// compile-time dictionaries are used for instantiating runtime
// dictionaries.
type readerDict struct {
	shaped bool // whether this is a shaped dictionary

	// baseSym is the symbol for the object this dictionary belongs to.
	// If the object is an instantiated function or defined type, then
	// baseSym is the mangled symbol, including any type arguments.
	baseSym *types.Sym

	// For non-shaped dictionaries, shapedObj is a reference to the
	// corresponding shaped object (always a function or defined type).
	shapedObj *ir.Name

	// targs holds the implicit and explicit type arguments in use for
	// reading the current object. For example:
	//
	//	func F[T any]() {
	//		type X[U any] struct { t T; u U }
	//		var _ X[string]
	//	}
	//
	//	var _ = F[int]
	//
	// While instantiating F[int], we need to in turn instantiate
	// X[string]. [int] and [string] are explicit type arguments for F
	// and X, respectively; but [int] is also the implicit type
	// arguments for X.
	//
	// (As an analogy to function literals, explicits are the function
	// literal's formal parameters, while implicits are variables
	// captured by the function literal.)
	targs []*types.Type

	// implicits counts how many of types within targs are implicit type
	// arguments; the rest are explicit.
	implicits int
	// receivers counts how many of types within targs are receiver type
	// arguments; they are explicit.
	receivers int

	derived      []derivedInfo // reloc index of the derived type's descriptor
	derivedTypes []*types.Type // slice of previously computed derived types

	// These slices correspond to entries in the runtime dictionary.
	typeParamMethodExprs []readerMethodExprInfo
	subdicts             []objInfo
	rtypes               []typeInfo
	itabs                []itabInfo
}

type readerMethodExprInfo struct {
	typeParamIdx int
	method       *types.Sym
}

func setType(n ir.Node, typ *types.Type) {
	n.SetType(typ)
	n.SetTypecheck(1)
}

func setValue(name *ir.Name, val constant.Value) {
	name.SetVal(val)
	name.Defn = nil
}

// @@@ Positions

// pos reads a position from the bitstream.
func (r *reader) pos() src.XPos {
	return base.Ctxt.PosTable.XPos(r.pos0())
}

// origPos reads a position from the bitstream, and returns both the
// original raw position and an inlining-adjusted position.
func (r *reader) origPos() (origPos, inlPos src.XPos) {
	r.suppressInlPos++
	origPos = r.pos()
	r.suppressInlPos--
	inlPos = r.inlPos(origPos)
	return
}

func (r *reader) pos0() src.Pos {
	r.Sync(pkgbits.SyncPos)
	if !r.Bool() {
		return src.NoPos
	}

	posBase := r.posBase()
	line := r.Uint()
	col := r.Uint()
	return src.MakePos(posBase, line, col)
}

// posBase reads a position base from the bitstream.
func (r *reader) posBase() *src.PosBase {
	return r.inlPosBase(r.p.posBaseIdx(r.Reloc(pkgbits.SectionPosBase)))
}

// posBaseIdx returns the specified position base, reading it first if
// needed.
func (pr *pkgReader) posBaseIdx(idx index) *src.PosBase {
	if b := pr.posBases[idx]; b != nil {
		return b
	}

	r := pr.newReader(pkgbits.SectionPosBase, idx, pkgbits.SyncPosBase)
	var b *src.PosBase

	absFilename := r.String()
	filename := absFilename

	// For build artifact stability, the export data format only
	// contains the "absolute" filename as returned by objabi.AbsFile.
	// However, some tests (e.g., test/run.go's asmcheck tests) expect
	// to see the full, original filename printed out. Re-expanding
	// "$GOROOT" to buildcfg.GOROOT is a close-enough approximation to
	// satisfy this.
	//
	// The export data format only ever uses slash paths
	// (for cross-operating-system reproducible builds),
	// but error messages need to use native paths (backslash on Windows)
	// as if they had been specified on the command line.
	// (The go command always passes native paths to the compiler.)
	const dollarGOROOT = "$GOROOT"
	if buildcfg.GOROOT != "" && strings.HasPrefix(filename, dollarGOROOT) {
		filename = filepath.FromSlash(buildcfg.GOROOT + filename[len(dollarGOROOT):])
	}

	if r.Bool() {
		b = src.NewFileBase(filename, absFilename)
	} else {
		pos := r.pos0()
		line := r.Uint()
		col := r.Uint()
		b = src.NewLinePragmaBase(pos, filename, absFilename, line, col)
	}

	pr.posBases[idx] = b
	return b
}

// inlPosBase returns the inlining-adjusted src.PosBase corresponding
// to oldBase, which must be a non-inlined position. When not
// inlining, this is just oldBase.
func (r *reader) inlPosBase(oldBase *src.PosBase) *src.PosBase {
	if index := oldBase.InliningIndex(); index >= 0 {
		base.Fatalf("oldBase %v already has inlining index %v", oldBase, index)
	}

	if r.inlCall == nil || r.suppressInlPos != 0 {
		return oldBase
	}

	if newBase, ok := r.inlPosBases[oldBase]; ok {
		return newBase
	}

	newBase := src.NewInliningBase(oldBase, r.inlTreeIndex)
	r.inlPosBases[oldBase] = newBase
	return newBase
}

// inlPos returns the inlining-adjusted src.XPos corresponding to
// xpos, which must be a non-inlined position. When not inlining, this
// is just xpos.
func (r *reader) inlPos(xpos src.XPos) src.XPos {
	pos := base.Ctxt.PosTable.Pos(xpos)
	pos.SetBase(r.inlPosBase(pos.Base()))
	return base.Ctxt.PosTable.XPos(pos)
}

// @@@ Packages

// pkg reads a package reference from the bitstream.
func (r *reader) pkg() *types.Pkg {
	r.Sync(pkgbits.SyncPkg)
	return r.p.pkgIdx(r.Reloc(pkgbits.SectionPkg))
}

// pkgIdx returns the specified package from the export data, reading
// it first if needed.
func (pr *pkgReader) pkgIdx(idx index) *types.Pkg {
	if pkg := pr.pkgs[idx]; pkg != nil {
		return pkg
	}

	pkg := pr.newReader(pkgbits.SectionPkg, idx, pkgbits.SyncPkgDef).doPkg()
	pr.pkgs[idx] = pkg
	return pkg
}

// doPkg reads a package definition from the bitstream.
func (r *reader) doPkg() *types.Pkg {
	path := r.String()
	switch path {
	case "":
		path = r.p.PkgPath()
	case "builtin":
		return types.BuiltinPkg
	case "unsafe":
		return types.UnsafePkg
	}

	name := r.String()

	pkg := types.NewPkg(path, "")

	if pkg.Name == "" {
		pkg.Name = name
	} else {
		base.Assertf(pkg.Name == name, "package %q has name %q, but want %q", pkg.Path, pkg.Name, name)
	}

	return pkg
}

// @@@ Types

func (r *reader) typ() *types.Type {
	return r.typWrapped(true)
}

// typWrapped is like typ, but allows suppressing generation of
// unnecessary wrappers as a compile-time optimization.
func (r *reader) typWrapped(wrapped bool) *types.Type {
	return r.p.typIdx(r.typInfo(), r.dict, wrapped)
}

func (r *reader) typInfo() typeInfo {
	r.Sync(pkgbits.SyncType)
	if r.Bool() {
		return typeInfo{idx: index(r.Len()), derived: true}
	}
	return typeInfo{idx: r.Reloc(pkgbits.SectionType), derived: false}
}

// typListIdx returns a list of the specified types, resolving derived
// types within the given dictionary.
func (pr *pkgReader) typListIdx(infos []typeInfo, dict *readerDict) []*types.Type {
	typs := make([]*types.Type, len(infos))
	for i, info := range infos {
		typs[i] = pr.typIdx(info, dict, true)
	}
	return typs
}

// typIdx returns the specified type. If info specifies a derived
// type, it's resolved within the given dictionary. If wrapped is
// true, then method wrappers will be generated, if appropriate.
func (pr *pkgReader) typIdx(info typeInfo, dict *readerDict, wrapped bool) *types.Type {
	idx := info.idx
	var where **types.Type
	if info.derived {
		where = &dict.derivedTypes[idx]
		idx = dict.derived[idx].idx
	} else {
		where = &pr.typs[idx]
	}

	if typ := *where; typ != nil {
		return typ
	}

	r := pr.newReader(pkgbits.SectionType, idx, pkgbits.SyncTypeIdx)
	r.dict = dict

	typ := r.doTyp()
	if typ == nil {
		base.Fatalf("doTyp returned nil for info=%v", info)
	}

	// For recursive type declarations involving interfaces and aliases,
	// above r.doTyp() call may have already set pr.typs[idx], so just
	// double check and return the type.
	//
	// Example:
	//
	//     type F = func(I)
	//
	//     type I interface {
	//         m(F)
	//     }
	//
	// The writer writes data types in following index order:
	//
	//     0: func(I)
	//     1: I
	//     2: interface{m(func(I))}
	//
	// The reader resolves it in following index order:
	//
	//     0 -> 1 -> 2 -> 0 -> 1
	//
	// and can divide in logically 2 steps:
	//
	//  - 0 -> 1     : first time the reader reach type I,
	//                 it creates new named type with symbol I.
	//
	//  - 2 -> 0 -> 1: the reader ends up reaching symbol I again,
	//                 now the symbol I was setup in above step, so
	//                 the reader just return the named type.
	//
	// Now, the functions called return, the pr.typs looks like below:
	//
	//  - 0 -> 1 -> 2 -> 0 : [<T> I <T>]
	//  - 0 -> 1 -> 2      : [func(I) I <T>]
	//  - 0 -> 1           : [func(I) I interface { "".m(func("".I)) }]
	//
	// The idx 1, corresponding with type I was resolved successfully
	// after r.doTyp() call.

	if prev := *where; prev != nil {
		return prev
	}

	if wrapped {
		// Only cache if we're adding wrappers, so that other callers that
		// find a cached type know it was wrapped.
		*where = typ

		r.needWrapper(typ)
	}

	if !typ.IsUntyped() {
		types.CheckSize(typ)
	}

	return typ
}

func (r *reader) doTyp() *types.Type {
	switch tag := pkgbits.CodeType(r.Code(pkgbits.SyncType)); tag {
	default:
		panic(fmt.Sprintf("unexpected type: %v", tag))

	case pkgbits.TypeBasic:
		return *basics[r.Len()]

	case pkgbits.TypeNamed:
		obj := r.obj()
		assert(obj.Op() == ir.OTYPE)
		return obj.Type()

	case pkgbits.TypeTypeParam:
		return r.dict.targs[r.Len()]

	case pkgbits.TypeArray:
		len := int64(r.Uint64())
		return types.NewArray(r.typ(), len)
	case pkgbits.TypeChan:
		dir := dirs[r.Len()]
		return types.NewChan(r.typ(), dir)
	case pkgbits.TypeMap:
		return types.NewMap(r.typ(), r.typ())
	case pkgbits.TypePointer:
		return types.NewPtr(r.typ())
	case pkgbits.TypeSignature:
		return r.signature(nil)
	case pkgbits.TypeSlice:
		return types.NewSlice(r.typ())
	case pkgbits.TypeStruct:
		return r.structType()
	case pkgbits.TypeInterface:
		return r.interfaceType()
	case pkgbits.TypeUnion:
		return r.unionType()
	}
}

func (r *reader) unionType() *types.Type {
	// In the types1 universe, we only need to handle value types.
	// Impure interfaces (i.e., interfaces with non-trivial type sets
	// like "int | string") can only appear as type parameter bounds,
	// and this is enforced by the types2 type checker.
	//
	// However, type unions can still appear in pure interfaces if the
	// type union is equivalent to "any". E.g., typeparam/issue52124.go
	// declares variables with the type "interface { any | int }".
	//
	// To avoid needing to represent type unions in types1 (since we
	// don't have any uses for that today anyway), we simply fold them
	// to "any".

	// TODO(mdempsky): Restore consistency check to make sure folding to
	// "any" is safe. This is unfortunately tricky, because a pure
	// interface can reference impure interfaces too, including
	// cyclically (#60117).
	if false {
		pure := false
		for i, n := 0, r.Len(); i < n; i++ {
			_ = r.Bool() // tilde
			term := r.typ()
			if term.IsEmptyInterface() {
				pure = true
			}
		}
		if !pure {
			base.Fatalf("impure type set used in value type")
		}
	}

	return types.Types[types.TINTER]
}

func (r *reader) interfaceType() *types.Type {
	nmethods, nembeddeds := r.Len(), r.Len()
	implicit := nmethods == 0 && nembeddeds == 1 && r.Bool()
	assert(!implicit) // implicit interfaces only appear in constraints

	fields := make([]*types.Field, nmethods+nembeddeds)
	methods, embeddeds := fields[:nmethods], fields[nmethods:]

	for i := range methods {
		methods[i] = types.NewField(r.pos(), r.selector(), r.signature(types.FakeRecv()))
	}
	for i := range embeddeds {
		embeddeds[i] = types.NewField(src.NoXPos, nil, r.typ())
	}

	if len(fields) == 0 {
		return types.Types[types.TINTER] // empty interface
	}
	return types.NewInterface(fields)
}

func (r *reader) structType() *types.Type {
	fields := make([]*types.Field, r.Len())
	for i := range fields {
		field := types.NewField(r.pos(), r.selector(), r.typ())
		field.Note = r.String()
		if r.Bool() {
			field.Embedded = 1
		}
		fields[i] = field
	}
	return types.NewStruct(fields)
}

func (r *reader) signature(recv *types.Field) *types.Type {
	r.Sync(pkgbits.SyncSignature)

	params := r.params()
	results := r.params()
	if r.Bool() { // variadic
		params[len(params)-1].SetIsDDD(true)
	}

	return types.NewSignature(recv, params, results)
}

func (r *reader) params() []*types.Field {
	r.Sync(pkgbits.SyncParams)
	params := make([]*types.Field, r.Len())
	for i := range params {
		params[i] = r.param()
	}
	return params
}

func (r *reader) param() *types.Field {
	r.Sync(pkgbits.SyncParam)
	return types.NewField(r.pos(), r.localIdent(), r.typ())
}

// @@@ Objects

// objReader maps qualified identifiers (represented as *types.Sym) to
// a pkgReader and corresponding index that can be used for reading
// that object's definition.
var objReader = map[*types.Sym]pkgReaderIndex{}

// obj reads an instantiated object reference from the bitstream.
func (r *reader) obj() ir.Node {
	return r.p.objInstIdx(r.objInfo(), r.dict, false)
}

// objInfo reads an instantiated object reference from the bitstream
// and returns the encoded reference to it, without instantiating it.
func (r *reader) objInfo() objInfo {
	r.Sync(pkgbits.SyncObject)
	if r.Version().Has(pkgbits.DerivedFuncInstance) {
		assert(!r.Bool())
	}
	idx := r.Reloc(pkgbits.SectionObj)

	explicits := make([]typeInfo, r.Len())
	for i := range explicits {
		explicits[i] = r.typInfo()
	}

	return objInfo{idx, explicits}
}

// objInstIdx returns the encoded, instantiated object. If shaped is
// true, then the shaped variant of the object is returned instead.
func (pr *pkgReader) objInstIdx(info objInfo, dict *readerDict, shaped bool) ir.Node {
	explicits := pr.typListIdx(info.explicits, dict)

	var implicits []*types.Type
	if dict != nil {
		implicits = dict.targs
	}

	return pr.objIdx(info.idx, implicits, explicits, shaped)
}

// objIdx returns the specified object, instantiated with the given
// type arguments, if any.
// If shaped is true, then the shaped variant of the object is returned
// instead.
func (pr *pkgReader) objIdx(idx index, implicits, explicits []*types.Type, shaped bool) ir.Node {
	n, err := pr.objIdxMayFail(idx, implicits, explicits, shaped)
	if err != nil {
		base.Fatalf("%v", err)
	}
	return n
}

// objIdxMayFail is equivalent to objIdx, but returns an error rather than
// failing the build if this object requires type arguments and the incorrect
// number of type arguments were passed.
//
// Other sources of internal failure (such as duplicate definitions) still fail
// the build.
func (pr *pkgReader) objIdxMayFail(idx index, implicits, explicits []*types.Type, shaped bool) (ir.Node, error) {
	rname := pr.newReader(pkgbits.SectionName, idx, pkgbits.SyncObject1)
	_, sym := rname.qualifiedIdent()
	tag := pkgbits.CodeObj(rname.Code(pkgbits.SyncCodeObj))

	if tag == pkgbits.ObjStub {
		assert(!sym.IsBlank())
		switch sym.Pkg {
		case types.BuiltinPkg, types.UnsafePkg:
			return sym.Def.(ir.Node), nil
		}
		if pri, ok := objReader[sym]; ok {
			return pri.pr.objIdxMayFail(pri.idx, nil, explicits, shaped)
		}
		if sym.Pkg.Path == "runtime" {
			return typecheck.LookupRuntime(sym.Name), nil
		}
		base.Fatalf("unresolved stub: %v", sym)
	}

	dict, err := pr.objDictIdx(sym, idx, implicits, explicits, shaped)
	if err != nil {
		return nil, err
	}

	sym = dict.baseSym
	if !sym.IsBlank() && sym.Def != nil {
		return sym.Def.(*ir.Name), nil
	}

	r := pr.newReader(pkgbits.SectionObj, idx, pkgbits.SyncObject1)
	rext := pr.newReader(pkgbits.SectionObjExt, idx, pkgbits.SyncObject1)

	r.dict = dict
	rext.dict = dict

	do := func(op ir.Op, hasTParams bool) *ir.Name {
		pos := r.pos()
		setBasePos(pos)
		if hasTParams {
			r.typeParamNames()
		}

		name := ir.NewDeclNameAt(pos, op, sym)
		name.Class = ir.PEXTERN // may be overridden later
		if !sym.IsBlank() {
			if sym.Def != nil {
				base.FatalfAt(name.Pos(), "already have a definition for %v", name)
			}
			assert(sym.Def == nil)
			sym.Def = name
		}
		return name
	}

	switch tag {
	default:
		panic("unexpected object")

	case pkgbits.ObjAlias:
		name := do(ir.OTYPE, false)

		if r.Version().Has(pkgbits.AliasTypeParamNames) {
			r.typeParamNames()
		}

		// Clumsy dance: the r.typ() call here might recursively find this
		// type alias name, before we've set its type (#66873). So we
		// temporarily clear sym.Def and then restore it later, if still
		// unset.
		hack := sym.Def == name
		if hack {
			sym.Def = nil
		}
		typ := r.typ()
		if hack {
			if sym.Def != nil {
				name = sym.Def.(*ir.Name)
				assert(types.IdenticalStrict(name.Type(), typ))
				return name, nil
			}
			sym.Def = name
		}

		setType(name, typ)
		name.SetAlias(true)
		return name, nil

	case pkgbits.ObjConst:
		name := do(ir.OLITERAL, false)
		typ := r.typ()
		val := FixValue(typ, r.Value())
		setType(name, typ)
		setValue(name, val)
		return name, nil

	case pkgbits.ObjFunc:
		npos := r.pos()
		setBasePos(npos)

		var sel *types.Sym
		var recv *types.Field
		if r.Version().Has(pkgbits.GenericMethods) && r.Bool() {
			sel = r.selector()
			r.recvTypeParamNames()
			recv = r.param()
		} else {
			if sym.Name == "init" {
				sym = Renameinit()
			}
		}
		r.typeParamNames()
		typ := r.signature(recv)
		fpos := r.pos()

		fn := ir.NewFunc(fpos, npos, sym, typ)
		if r.hasTypeParams() && r.dict.shaped {
			typ.SetHasShape(true)
		}

		name := fn.Nname
		if !sym.IsBlank() {
			if sym.Def != nil {
				base.FatalfAt(name.Pos(), "already have a definition for %v", name)
			}
			assert(sym.Def == nil)
			sym.Def = name
		}

		if r.hasTypeParams() {
			name.Func.SetDupok(true)
			if r.dict.shaped {
				setType(name, shapeSig(name.Func, r.dict))
			} else {
				todoDicts = append(todoDicts, func() {
					r.dict.shapedObj = pr.objIdx(idx, implicits, explicits, true).(*ir.Name)
				})
			}
		}

		rext.funcExt(name, sel)
		return name, nil

	case pkgbits.ObjType:
		name := do(ir.OTYPE, true)
		typ := types.NewNamed(name)
		setType(name, typ)
		if r.hasTypeParams() && r.dict.shaped {
			typ.SetHasShape(true)
		}

		// Important: We need to do this before SetUnderlying.
		rext.typeExt(name)

		// We need to defer CheckSize until we've called SetUnderlying to
		// handle recursive types.
		types.DeferCheckSize()
		typ.SetUnderlying(r.typWrapped(false))
		types.ResumeCheckSize()

		if r.hasTypeParams() && !r.dict.shaped {
			todoDicts = append(todoDicts, func() {
				r.dict.shapedObj = pr.objIdx(idx, implicits, explicits, true).(*ir.Name)
			})
		}

		methods := make([]*types.Field, r.Len())
		for i := range methods {
			methods[i] = r.method(rext)
		}
		if len(methods) != 0 {
			typ.SetMethods(methods)
		}

		if !r.dict.shaped {
			r.needWrapper(typ)
		}

		return name, nil

	case pkgbits.ObjVar:
		name := do(ir.ONAME, false)
		setType(name, r.typ())
		rext.varExt(name)
		return name, nil
	}
}

// mangle shapes the non-shaped symbol sym under the current dictionary.
func (dict *readerDict) mangle(sym *types.Sym) *types.Sym {
	if !dict.hasTypeParams() {
		return sym
	}

	var buf strings.Builder
	// If sym is a locally defined generic type, we need the suffix to
	// stay at the end after mangling so that types/fmt.go can strip it
	// out again when writing the type's runtime descriptor (#54456).
	n0, vsuff := types.SplitVargenSuffix(sym.Name)
	n1, msuff := types.SplitMethSuffix(sym.Name)

	// Methods are never locally defined.
	var n string
	assert(vsuff == "" || msuff == "")
	if vsuff != "" {
		n = n0
	} else {
		n = n1
	}

	var j int
	assert(dict.implicits == 0 || dict.receivers == 0)
	if msuff != "" {
		j = dict.receivers // consume receiver type arguments
	} else {
		j = len(dict.targs) // consume all type arguments
	}

	// put type arguments inside parenthesis; (*T)[int] -> (*T[int])
	n, ok := strings.CutSuffix(n, ")")

	// type arguments, if any
	buf.WriteString(n)
	if j > 0 {
		buf.WriteByte('[')
		for i := 0; i < j; i++ {
			if i > 0 {
				if i == dict.implicits {
					buf.WriteByte(';')
				} else {
					buf.WriteByte(',')
				}
			}
			buf.WriteString(dict.targs[i].LinkString())
		}
		buf.WriteByte(']')
	}

	if ok {
		buf.WriteString(")")
	}

	buf.WriteString(vsuff)
	buf.WriteString(msuff)

	// method arguments, if any
	if msuff != "" {
		buf.WriteByte('[')
		for i := j; i < len(dict.targs); i++ {
			if i > j {
				buf.WriteByte(',')
			}
			buf.WriteString(dict.targs[i].LinkString())
		}
		buf.WriteByte(']')
	}

	return sym.Pkg.Lookup(buf.String())
}

// Shapify returns the shape type for targ.
//
// If basic is true, then the type argument is used to instantiate a
// type parameter whose constraint is a basic interface.
func Shapify(targ *types.Type, basic bool) *types.Type {
	if targ.Kind() == types.TFORW {
		if targ.IsFullyInstantiated() {
			// For recursive instantiated type argument, it may  still be a TFORW
			// when shapifying happens. If we don't have targ's underlying type,
			// shapify won't work. The worst case is we end up not reusing code
			// optimally in some tricky cases.
			if base.Debug.Shapify != 0 {
				base.Warn("skipping shaping of recursive type %v", targ)
			}
			if targ.HasShape() {
				return targ
			}
		} else {
			base.Fatalf("%v is missing its underlying type", targ)
		}
	}
	// For fully instantiated shape interface type, use it as-is. Otherwise, the instantiation
	// involved recursive generic interface may cause mismatching in function signature, see issue #65362.
	if targ.Kind() == types.TINTER && targ.IsFullyInstantiated() && targ.HasShape() {
		return targ
	}

	// When a pointer type is used to instantiate a type parameter
	// constrained by a basic interface, we know the pointer's element
	// type can't matter to the generated code. In this case, we can use
	// an arbitrary pointer type as the shape type. (To match the
	// non-unified frontend, we use `*byte`.)
	//
	// Otherwise, we simply use the type's underlying type as its shape.
	//
	// TODO(mdempsky): It should be possible to do much more aggressive
	// shaping still; e.g., collapsing all pointer-shaped types into a
	// common type, collapsing scalars of the same size/alignment into a
	// common type, recursively shaping the element types of composite
	// types, and discarding struct field names and tags. However, we'll
	// need to start tracking how type parameters are actually used to
	// implement some of these optimizations.
	under := targ.Underlying()
	if basic && targ.IsPtr() && !targ.Elem().NotInHeap() {
		under = types.NewPtr(types.Types[types.TUINT8])
	}

	// Hash long type names to bound symbol name length seen by users,
	// particularly for large protobuf structs (#65030).
	uls := under.LinkString()
	if base.Debug.MaxShapeLen != 0 &&
		len(uls) > base.Debug.MaxShapeLen {
		h := hash.Sum32([]byte(uls))
		uls = hex.EncodeToString(h[:])
	}

	sym := types.ShapePkg.Lookup(uls)
	if sym.Def == nil {
		name := ir.NewDeclNameAt(under.Pos(), ir.OTYPE, sym)
		typ := types.NewNamed(name)
		typ.SetUnderlying(under)
		sym.Def = typed(typ, name)
	}
	res := sym.Def.Type()
	assert(res.IsShape())
	assert(res.HasShape())
	return res
}

// objDictIdx reads and returns the specified object dictionary.
func (pr *pkgReader) objDictIdx(sym *types.Sym, idx index, implicits, explicits []*types.Type, shaped bool) (*readerDict, error) {
	r := pr.newReader(pkgbits.SectionObjDict, idx, pkgbits.SyncObject1)

	dict := readerDict{
		shaped: shaped,
	}

	nimplicits := r.Len()
	nreceivers := 0
	if r.Version().Has(pkgbits.GenericMethods) {
		nreceivers = r.Len()
	}
	nexplicits := r.Len() + nreceivers

	if nimplicits > len(implicits) || nexplicits != len(explicits) {
		return nil, fmt.Errorf("%v has %v+%v params, but instantiated with %v+%v args", sym, nimplicits, nexplicits, len(implicits), len(explicits))
	}

	dict.targs = append(implicits[:nimplicits:nimplicits], explicits...)
	dict.implicits = nimplicits
	dict.receivers = nreceivers

	// Within the compiler, we can just skip over the type parameters.
	for range dict.targs[dict.implicits:] {
		// Skip past bounds without actually evaluating them.
		r.typInfo()
	}

	dict.derived = make([]derivedInfo, r.Len())
	dict.derivedTypes = make([]*types.Type, len(dict.derived))
	for i := range dict.derived {
		dict.derived[i] = derivedInfo{idx: r.Reloc(pkgbits.SectionType)}
		if r.Version().Has(pkgbits.DerivedInfoNeeded) {
			assert(!r.Bool())
		}
	}

	// Runtime dictionary information; private to the compiler.

	// If any type argument is already shaped, then we're constructing a
	// shaped object, even if not explicitly requested (i.e., calling
	// objIdx with shaped==true). This can happen with instantiating
	// types that are referenced within a function body.
	for _, targ := range dict.targs {
		if targ.HasShape() {
			dict.shaped = true
			break
		}
	}

	// And if we're constructing a shaped object, then shapify all type
	// arguments.
	for i, targ := range dict.targs {
		basic := r.Bool()
		if dict.shaped {
			dict.targs[i] = Shapify(targ, basic)
		}
	}

	dict.baseSym = dict.mangle(sym)

	dict.typeParamMethodExprs = make([]readerMethodExprInfo, r.Len())
	for i := range dict.typeParamMethodExprs {
		typeParamIdx := r.Len()
		method := r.selector()

		dict.typeParamMethodExprs[i] = readerMethodExprInfo{typeParamIdx, method}
	}

	dict.subdicts = make([]objInfo, r.Len())
	for i := range dict.subdicts {
		dict.subdicts[i] = r.objInfo()
	}

	dict.rtypes = make([]typeInfo, r.Len())
	for i := range dict.rtypes {
		dict.rtypes[i] = r.typInfo()
	}

	dict.itabs = make([]itabInfo, r.Len())
	for i := range dict.itabs {
		dict.itabs[i] = itabInfo{typ: r.typInfo(), iface: r.typInfo()}
	}

	return &dict, nil
}

func (r *reader) recvTypeParamNames() {
	r.Sync(pkgbits.SyncTypeParamNames)

	for range r.dict.targs[r.dict.implicits : r.dict.implicits+r.dict.receivers] {
		r.pos()
		r.localIdent()
	}
}

func (r *reader) typeParamNames() {
	r.Sync(pkgbits.SyncTypeParamNames)

	for range r.dict.targs[r.dict.implicits+r.dict.receivers:] {
		r.pos()
		r.localIdent()
	}
}

func (r *reader) method(rext *reader) *types.Field {
	r.Sync(pkgbits.SyncMethod)
	npos := r.pos()
	sym := r.selector()
	r.typeParamNames()
	recv := r.param()
	typ := r.signature(recv)

	fpos := r.pos()
	fn := ir.NewFunc(fpos, npos, ir.MethodSym(recv.Type, sym), typ)
	name := fn.Nname

	if r.hasTypeParams() {
		name.Func.SetDupok(true)
		if r.dict.shaped {
			typ = shapeSig(name.Func, r.dict)
			setType(name, typ)
		}
	}

	rext.funcExt(name, sym)

	meth := types.NewField(name.Func.Pos(), sym, typ)
	meth.Nname = name
	meth.SetNointerface(name.Func.Pragma&ir.Nointerface != 0)

	return meth
}

func (r *reader) qualifiedIdent() (pkg *types.Pkg, sym *types.Sym) {
	r.Sync(pkgbits.SyncSym)
	pkg = r.pkg()
	if name := r.String(); name != "" {
		sym = pkg.Lookup(name)
	}
	return
}

func (r *reader) localIdent() *types.Sym {
	r.Sync(pkgbits.SyncLocalIdent)
	pkg := r.pkg()
	if name := r.String(); name != "" {
		return pkg.Lookup(name)
	}
	return nil
}

func (r *reader) selector() *types.Sym {
	r.Sync(pkgbits.SyncSelector)
	pkg := r.pkg()
	name := r.String()
	if types.IsExported(name) {
		pkg = types.LocalPkg
	}
	return pkg.Lookup(name)
}

func (r *reader) hasTypeParams() bool {
	return r.dict.hasTypeParams()
}

func (dict *readerDict) hasTypeParams() bool {
	return dict != nil && len(dict.targs) != 0
}

// @@@ Compiler extensions

func (r *reader) funcExt(name *ir.Name, method *types.Sym) {
	r.Sync(pkgbits.SyncFuncExt)

	fn := name.Func

	// XXX: Workaround because linker doesn't know how to copy Pos.
	if !fn.Pos().IsKnown() {
		fn.SetPos(name.Pos())
	}

	// Normally, we only compile local functions, which saves redundant compilation work.
	// n.Defn is not nil for local functions, and is nil for imported function. But for
	// generic functions, we might have an instantiation that no other package has seen before.
	// So we need to be conservative and compile it again.
	//
	// That's why name.Defn is set here, so ir.VisitFuncsBottomUp can analyze function.
	// TODO(mdempsky,cuonglm): find a cleaner way to handle this.
	if name.Sym().Pkg == types.LocalPkg || r.hasTypeParams() {
		name.Defn = fn
	}

	fn.Pragma = r.pragmaFlag()
	r.linkname(name)

	if buildcfg.GOARCH == "wasm" {
		importmod := r.String()
		importname := r.String()
		exportname := r.String()

		if importmod != "" && importname != "" {
			fn.WasmImport = &ir.WasmImport{
				Module: importmod,
				Name:   importname,
			}
		}
		if exportname != "" {
			if method != nil {
				base.ErrorfAt(fn.Pos(), 0, "cannot use //go:wasmexport on a method")
			}
			fn.WasmExport = &ir.WasmExport{Name: exportname}
		}
	}

	if r.Bool() {
		assert(name.Defn == nil)

		fn.ABI = obj.ABI(r.Uint64())

		// Escape analysis.
		for _, f := range name.Type().RecvParams() {
			f.Note = r.String()
		}

		if r.Bool() {
			fn.Inl = &ir.Inline{
				Cost:            int32(r.Len()),
				CanDelayResults: r.Bool(),
			}
			if buildcfg.Experiment.NewInliner {
				fn.Inl.Properties = r.String()
			}
		}
	} else {
		r.addBody(name.Func, method)
	}
	r.Sync(pkgbits.SyncEOF)
}

func (r *reader) typeExt(name *ir.Name) {
	r.Sync(pkgbits.SyncTypeExt)

	typ := name.Type()

	if r.hasTypeParams() {
		// Mark type as fully instantiated to ensure the type descriptor is written
		// out as DUPOK and method wrappers are generated even for imported types.
		typ.SetIsFullyInstantiated(true)
		// HasShape should be set if any type argument is or has a shape type.
		for _, targ := range r.dict.targs {
			if targ.HasShape() {
				typ.SetHasShape(true)
				break
			}
		}
	}

	name.SetPragma(r.pragmaFlag())

	typecheck.SetBaseTypeIndex(typ, r.Int64(), r.Int64())
}

func (r *reader) varExt(name *ir.Name) {
	r.Sync(pkgbits.SyncVarExt)
	r.linkname(name)
}

func (r *reader) linkname(name *ir.Name) {
	assert(name.Op() == ir.ONAME)
	r.Sync(pkgbits.SyncLinkname)

	if idx := r.Int64(); idx >= 0 {
		lsym := name.Linksym()
		lsym.SymIdx = int32(idx)
		lsym.Set(obj.AttrIndexed, true)
	} else {
		linkname := r.String()
		std := r.Bool()
		sym := name.Sym()
		sym.Linkname = linkname
		if sym.Pkg == types.LocalPkg && linkname != "" {
			// Mark linkname in the current package. We don't mark the
			// ones that are imported and propagated (e.g. through
			// inlining or instantiation, which are marked in their
			// corresponding packages). So we can tell in which package
			// the linkname is used (pulled), and the linker can
			// make a decision for allowing or disallowing it.
			if std {
				sym.Linksym().Set(obj.AttrLinknameStd, true)
			} else {
				sym.Linksym().Set(obj.AttrLinkname, true)
			}
		}
	}
}

func (r *reader) pragmaFlag() ir.PragmaFlag {
	r.Sync(pkgbits.SyncPragma)
	return ir.PragmaFlag(r.Int())
}

// @@@ Function bodies

// bodyReader tracks where the serialized IR for a local or imported,
// generic function's body can be found.
var bodyReader = map[*ir.Func]pkgReaderIndex{}

// importBodyReader tracks where the serialized IR for an imported,
// static (i.e., non-generic) function body can be read.
var importBodyReader = map[*types.Sym]pkgReaderIndex{}

// bodyReaderFor returns the pkgReaderIndex for reading fn's
// serialized IR, and whether one was found.
func bodyReaderFor(fn *ir.Func) (pri pkgReaderIndex, ok bool) {
	if fn.Nname.Defn != nil {
		pri, ok = bodyReader[fn]
		base.AssertfAt(ok, base.Pos, "must have bodyReader for %v", fn) // must always be available
	} else {
		pri, ok = importBodyReader[fn.Sym()]
	}
	return
}

// todoDicts holds the list of dictionaries that still need their
// runtime dictionary objects constructed.
var todoDicts []func()

// todoBodies holds the list of function bodies that still need to be
// constructed.
var todoBodies []*ir.Func

// addBody reads a function body reference from the element bitstream,
// and associates it with fn.
func (r *reader) addBody(fn *ir.Func, method *types.Sym) {
	// addBody should only be called for local functions or imported
	// generic functions; see comment in funcExt.
	assert(fn.Nname.Defn != nil)

	idx := r.Reloc(pkgbits.SectionBody)

	pri := pkgReaderIndex{r.p, idx, r.dict, method, nil}
	bodyReader[fn] = pri

	if r.curfn == nil {
		todoBodies = append(todoBodies, fn)
		return
	}

	pri.funcBody(fn)
}

func (pri pkgReaderIndex) funcBody(fn *ir.Func) {
	r := pri.asReader(pkgbits.SectionBody, pkgbits.SyncFuncBody)
	panicking := true
	defer func() {
		if panicking {
			// TODO not sure what the best way to print in this context is.
			// If code panics in unified IR reading, you want *something* like this.
			// Whoever ends up debugging the next unified IR failure, please
			// improve this (base.Warnf?) if you can figure out how.
			fmt.Printf("****** panic traversed funcBody of %v\n", fn)
		}
	}()
	r.funcBody(fn)
	panicking = false

}

// funcBody reads a function body definition from the element
// bitstream, and populates fn with it.
func (r *reader) funcBody(fn *ir.Func) {
	r.curfn = fn
	r.closureVars = fn.ClosureVars
	if len(r.closureVars) != 0 && r.hasTypeParams() {
		r.dictParam = r.closureVars[len(r.closureVars)-1] // dictParam is last; see reader.funcLit
	}

	ir.WithFunc(fn, func() {
		r.declareParams()

		if r.syntheticBody(fn.Pos()) {
			return
		}

		if !r.Bool() {
			return
		}

		body := r.stmts()
		if body == nil {
			body = []ir.Node{typecheck.Stmt(ir.NewBlockStmt(src.NoXPos, nil))}
		}
		fn.Body = body
		fn.Endlineno = r.pos()
	})

	r.marker.WriteTo(fn)
}

// syntheticBody adds a synthetic body to r.curfn if appropriate, and
// reports whether it did.
func (r *reader) syntheticBody(pos src.XPos) bool {
	if r.synthetic != nil {
		r.synthetic(pos, r)
		return true
	}

	// If this function has type parameters and isn't shaped, then we
	// just tail call its corresponding shaped variant.
	if r.hasTypeParams() && !r.dict.shaped {
		r.callShaped(pos)
		return true
	}

	return false
}

// callShaped emits a tail call to r.shapedFn, passing along the
// arguments to the current function.
func (r *reader) callShaped(pos src.XPos) {
	shapedObj := r.dict.shapedObj
	assert(shapedObj != nil)

	var shapedFn ir.Node
	if r.methodSym == nil {
		// Instantiating a generic function; shapedObj is the shaped function itself.
		assert(shapedObj.Op() == ir.ONAME && shapedObj.Class == ir.PFUNC)
		shapedFn = shapedObj
	} else {
		// Instantiating a generic type's method; shapedObj is the shaped method itself
		// if the method is generic — else, it is the shaped type declaring the method.
		shapedFn = shapedMethodExpr(pos, shapedObj, r.methodSym)
	}

	params := r.syntheticArgs()

	// Construct the arguments list: receiver (if any), then runtime
	// dictionary, and finally normal parameters.
	//
	// Note: For simplicity, shaped methods are added as normal methods
	// on their shaped types. So existing code (e.g., packages ir and
	// typecheck) expects the shaped type to appear as the receiver
	// parameter (or first parameter, as a method expression). Hence
	// putting the dictionary parameter after that is the least invasive
	// solution at the moment.
	var args ir.Nodes
	if r.methodSym != nil {
		args.Append(params[0])
		params = params[1:]
	}
	args.Append(typecheck.Expr(ir.NewAddrExpr(pos, r.p.dictNameOf(r.dict))))
	args.Append(params...)

	r.syntheticTailCall(pos, shapedFn, args)
}

// syntheticArgs returns the recvs and params arguments passed to the
// current function.
func (r *reader) syntheticArgs() ir.Nodes {
	sig := r.curfn.Nname.Type()
	return ir.ToNodes(r.curfn.Dcl[:sig.NumRecvs()+sig.NumParams()])
}

// syntheticTailCall emits a tail call to fn, passing the given
// arguments list.
func (r *reader) syntheticTailCall(pos src.XPos, fn ir.Node, args ir.Nodes) {
	// Mark the function as a wrapper so it doesn't show up in stack
	// traces.
	r.curfn.SetWrapper(true)

	call := typecheck.Call(pos, fn, args, fn.Type().IsVariadic()).(*ir.CallExpr)

	var stmt ir.Node
	if fn.Type().NumResults() != 0 {
		stmt = typecheck.Stmt(ir.NewReturnStmt(pos, []ir.Node{call}))
	} else {
		stmt = call
	}
	r.curfn.Body.Append(stmt)
}

// dictNameOf returns the runtime dictionary corresponding to dict.
func (pr *pkgReader) dictNameOf(dict *readerDict) *ir.Name {
	pos := base.AutogeneratedPos

	// Check that we only instantiate runtime dictionaries with real types.
	base.AssertfAt(!dict.shaped, pos, "runtime dictionary of shaped object %v", dict.baseSym)

	sym := dict.baseSym.Pkg.Lookup(objabi.GlobalDictPrefix + "." + dict.baseSym.Name)
	if sym.Def != nil {
		return sym.Def.(*ir.Name)
	}

	name := ir.NewNameAt(pos, sym, dict.varType())
	name.Class = ir.PEXTERN
	sym.Def = name // break cycles with mutual subdictionaries

	lsym := name.Linksym()
	ot := 0

	assertOffset := func(section string, offset int) {
		base.AssertfAt(ot == offset*types.PtrSize, pos, "writing section %v at offset %v, but it should be at %v*%v", section, ot, offset, types.PtrSize)
	}

	assertOffset("type param method exprs", dict.typeParamMethodExprsOffset())
	for _, info := range dict.typeParamMethodExprs {
		typeParam := dict.targs[info.typeParamIdx]
		method := typecheck.NewMethodExpr(pos, typeParam, info.method)

		rsym := method.FuncName().Linksym()
		assert(rsym.ABI() == obj.ABIInternal) // must be ABIInternal; see ir.OCFUNC in ssagen/ssa.go

		ot = objw.SymPtr(lsym, ot, rsym, 0)
	}

	assertOffset("subdictionaries", dict.subdictsOffset())
	for _, info := range dict.subdicts {
		explicits := pr.typListIdx(info.explicits, dict)

		// Careful: Due to subdictionary cycles, name may not be fully
		// initialized yet.
		name := pr.objDictName(info.idx, dict.targs, explicits)

		ot = objw.SymPtr(lsym, ot, name.Linksym(), 0)
	}

	assertOffset("rtypes", dict.rtypesOffset())
	for _, info := range dict.rtypes {
		typ := pr.typIdx(info, dict, true)
		ot = objw.SymPtr(lsym, ot, reflectdata.TypeLinksym(typ), 0)

		// TODO(mdempsky): Double check this.
		reflectdata.MarkTypeUsedInInterface(typ, lsym)
	}

	// For each (typ, iface) pair, we write the *runtime.itab pointer
	// for the pair. For pairs that don't actually require an itab
	// (i.e., typ is an interface, or iface is an empty interface), we
	// write a nil pointer instead. This is wasteful, but rare in
	// practice (e.g., instantiating a type parameter with an interface
	// type).
	assertOffset("itabs", dict.itabsOffset())
	for _, info := range dict.itabs {
		typ := pr.typIdx(info.typ, dict, true)
		iface := pr.typIdx(info.iface, dict, true)

		if !typ.IsInterface() && iface.IsInterface() && !iface.IsEmptyInterface() {
			ot = objw.SymPtr(lsym, ot, reflectdata.ITabLsym(typ, iface), 0)
		} else {
			ot += types.PtrSize
		}

		// TODO(mdempsky): Double check this.
		reflectdata.MarkTypeUsedInInterface(typ, lsym)
		reflectdata.MarkTypeUsedInInterface(iface, lsym)
	}

	objw.Global(lsym, int32(ot), obj.DUPOK|obj.RODATA)

	return name
}

// typeParamMethodExprsOffset returns the offset of the runtime
// dictionary's type parameter method expressions section, in words.
func (dict *readerDict) typeParamMethodExprsOffset() int {
	return 0
}

// subdictsOffset returns the offset of the runtime dictionary's
// subdictionary section, in words.
func (dict *readerDict) subdictsOffset() int {
	return dict.typeParamMethodExprsOffset() + len(dict.typeParamMethodExprs)
}

// rtypesOffset returns the offset of the runtime dictionary's rtypes
// section, in words.
func (dict *readerDict) rtypesOffset() int {
	return dict.subdictsOffset() + len(dict.subdicts)
}

// itabsOffset returns the offset of the runtime dictionary's itabs
// section, in words.
func (dict *readerDict) itabsOffset() int {
	return dict.rtypesOffset() + len(dict.rtypes)
}

// numWords returns the total number of words that comprise dict's
// runtime dictionary variable.
func (dict *readerDict) numWords() int64 {
	return int64(dict.itabsOffset() + len(dict.itabs))
}

// varType returns the type of dict's runtime dictionary variable.
func (dict *readerDict) varType() *types.Type {
	return types.NewArray(types.Types[types.TUINTPTR], dict.numWords())
}

func (r *reader) declareParams() {
	r.curfn.DeclareParams(!r.funarghack)

	for _, name := range r.curfn.Dcl {
		if name.Sym().Name == dictParamName {
			r.dictParam = name
			continue
		}

		r.addLocal(name)
	}
}

func (r *reader) addLocal(name *ir.Name) {
	if r.synthetic == nil {
		r.Sync(pkgbits.SyncAddLocal)
		if r.p.SyncMarkers() {
			want := r.Int()
			if have := len(r.locals); have != want {
				base.FatalfAt(name.Pos(), "locals table has desynced")
			}
		}
		r.varDictIndex(name)
	}

	r.locals = append(r.locals, name)
}

func (r *reader) useLocal() *ir.Name {
	r.Sync(pkgbits.SyncUseObjLocal)
	if r.Bool() {
		return r.locals[r.Len()]
	}
	return r.closureVars[r.Len()]
}

func (r *reader) openScope() {
	r.Sync(pkgbits.SyncOpenScope)
	pos := r.pos()

	if base.Flag.Dwarf {
		r.scopeVars = append(r.scopeVars, len(r.curfn.Dcl))
		r.marker.Push(pos)
	}
}

func (r *reader) closeScope() {
	r.Sync(pkgbits.SyncCloseScope)
	r.lastCloseScopePos = r.pos()

	r.closeAnotherScope()
}

// closeAnotherScope is like closeScope, but it reuses the same mark
// position as the last closeScope call. This is useful for "for" and
// "if" statements, as their implicit blocks always end at the same
// position as an explicit block.
func (r *reader) closeAnotherScope() {
	r.Sync(pkgbits.SyncCloseAnotherScope)

	if base.Flag.Dwarf {
		scopeVars := r.scopeVars[len(r.scopeVars)-1]
		r.scopeVars = r.scopeVars[:len(r.scopeVars)-1]

		// Quirkish: noder decides which scopes to keep before
		// typechecking, whereas incremental typechecking during IR
		// construction can result in new autotemps being allocated. To
		// produce identical output, we ignore autotemps here for the
		// purpose of deciding whether to retract the scope.
		//
		// This is important for net/http/fcgi, because it contains:
		//
		//	var body io.ReadCloser
		//	if len(content) > 0 {
		//		body, req.pw = io.Pipe()
		//	} else { … }
		//
		// Notably, io.Pipe is inlinable, and inlining it introduces a ~R0
		// variable at the call site.
		//
		// Noder does not preserve the scope where the io.Pipe() call
		// resides, because it doesn't contain any declared variables in
		// source. So the ~R0 variable ends up being assigned to the
		// enclosing scope instead.
		//
		// However, typechecking this assignment also introduces
		// autotemps, because io.Pipe's results need conversion before
		// they can be assigned to their respective destination variables.
		//
		// TODO(mdempsky): We should probably just keep all scopes, and
		// let dwarfgen take care of pruning them instead.
		retract := true
		for _, n := range r.curfn.Dcl[scopeVars:] {
			if !n.AutoTemp() {
				retract = false
				break
			}
		}

		if retract {
			// no variables were declared in this scope, so we can retract it.
			r.marker.Unpush()
		} else {
			r.marker.Pop(r.lastCloseScopePos)
		}
	}
}

// @@@ Statements

func (r *reader) stmt() ir.Node {
	return block(r.stmts())
}

func block(stmts []ir.Node) ir.Node {
	switch len(stmts) {
	case 0:
		return nil
	case 1:
		return stmts[0]
	default:
		return ir.NewBlockStmt(stmts[0].Pos(), stmts)
	}
}

func (r *reader) stmts() ir.Nodes {
	assert(ir.CurFunc == r.curfn)
	var res ir.Nodes

	r.Sync(pkgbits.SyncStmts)
	for {
		tag := codeStmt(r.Code(pkgbits.SyncStmt1))
		if tag == stmtEnd {
			r.Sync(pkgbits.SyncStmtsEnd)
			return res
		}

		if n := r.stmt1(tag, &res); n != nil {
			res.Append(typecheck.Stmt(n))
		}
	}
}

func (r *reader) stmt1(tag codeStmt, out *ir.Nodes) ir.Node {
	var label *types.Sym
	if n := len(*out); n > 0 {
		if ls, ok := (*out)[n-1].(*ir.LabelStmt); ok {
			label = ls.Label
		}
	}

	switch tag {
	default:
		panic("unexpected statement")

	case stmtAssign:
		pos := r.pos()
		names, lhs := r.assignList()
		rhs := r.multiExpr()

		if len(rhs) == 0 {
			for _, name := range names {
				as := ir.NewAssignStmt(pos, name, nil)
				as.PtrInit().Append(ir.NewDecl(pos, ir.ODCL, name))
				out.Append(typecheck.Stmt(as))
			}
			return nil
		}

		if len(lhs) == 1 && len(rhs) == 1 {
			n := ir.NewAssignStmt(pos, lhs[0], rhs[0])
			n.Def = r.initDefn(n, names)
			return n
		}

		n := ir.NewAssignListStmt(pos, ir.OAS2, lhs, rhs)
		n.Def = r.initDefn(n, names)
		return n

	case stmtAssignOp:
		op := r.op()
		lhs := r.expr()
		pos := r.pos()
		rhs := r.expr()
		return ir.NewAssignOpStmt(pos, op, lhs, rhs)

	case stmtIncDec:
		op := r.op()
		lhs := r.expr()
		pos := r.pos()
		n := ir.NewAssignOpStmt(pos, op, lhs, ir.NewOne(pos, lhs.Type()))
		n.IncDec = true
		return n

	case stmtBlock:
		out.Append(r.blockStmt()...)
		return nil

	case stmtBranch:
		pos := r.pos()
		op := r.op()
		sym := r.optLabel()
		return ir.NewBranchStmt(pos, op, sym)

	case stmtCall:
		pos := r.pos()
		op := r.op()
		call := r.expr()
		stmt := ir.NewGoDeferStmt(pos, op, call)
		if op == ir.ODEFER {
			x := r.optExpr()
			if x != nil {
				stmt.DeferAt = x.(ir.Expr)
			}
		}
		return stmt

	case stmtExpr:
		return r.expr()

	case stmtFor:
		return r.forStmt(label)

	case stmtIf:
		return r.ifStmt()

	case stmtLabel:
		pos := r.pos()
		sym := r.label()
		return ir.NewLabelStmt(pos, sym)

	case stmtReturn:
		pos := r.pos()
		results := r.multiExpr()
		return ir.NewReturnStmt(pos, results)

	case stmtSelect:
		return r.selectStmt(label)

	case stmtSend:
		pos := r.pos()
		ch := r.expr()
		value := r.expr()
		return ir.NewSendStmt(pos, ch, value)

	case stmtSwitch:
		return r.switchStmt(label)
	}
}

func (r *reader) assignList() ([]*ir.Name, []ir.Node) {
	lhs := make([]ir.Node, r.Len())
	var names []*ir.Name

	for i := range lhs {
		expr, def := r.assign()
		lhs[i] = expr
		if def {
			names = append(names, expr.(*ir.Name))
		}
	}

	return names, lhs
}

// assign returns an assignee expression. It also reports whether the
// returned expression is a newly declared variable.
func (r *reader) assign() (ir.Node, bool) {
	switch tag := codeAssign(r.Code(pkgbits.SyncAssign)); tag {
	default:
		panic("unhandled assignee expression")

	case assignBlank:
		return typecheck.AssignExpr(ir.BlankNode), false

	case assignDef:
		pos := r.pos()
		setBasePos(pos) // test/fixedbugs/issue49767.go depends on base.Pos being set for the r.typ() call here, ugh
		name := r.curfn.NewLocal(pos, r.localIdent(), r.typ())
		r.addLocal(name)
		return name, true

	case assignExpr:
		return r.expr(), false
	}
}

func (r *reader) blockStmt() []ir.Node {
	r.Sync(pkgbits.SyncBlockStmt)
	r.openScope()
	stmts := r.stmts()
	r.closeScope()
	return stmts
}

func (r *reader) forStmt(label *types.Sym) ir.Node {
	r.Sync(pkgbits.SyncForStmt)

	r.openScope()

	if r.Bool() {
		pos := r.pos()
		rang := ir.NewRangeStmt(pos, nil, nil, nil, nil, false)
		rang.Label = label

		names, lhs := r.assignList()
		if len(lhs) >= 1 {
			rang.Key = lhs[0]
			if len(lhs) >= 2 {
				rang.Value = lhs[1]
			}
		}
		rang.Def = r.initDefn(rang, names)

		rang.X = r.expr()
		if rang.X.Type().IsMap() {
			rang.RType = r.rtype(pos)
		}
		if rang.Key != nil && !ir.IsBlank(rang.Key) {
			rang.KeyTypeWord, rang.KeySrcRType = r.convRTTI(pos)
		}
		if rang.Value != nil && !ir.IsBlank(rang.Value) {
			rang.ValueTypeWord, rang.ValueSrcRType = r.convRTTI(pos)
		}

		rang.Body = r.blockStmt()
		rang.DistinctVars = r.Bool()
		r.closeAnotherScope()

		return rang
	}

	pos := r.pos()
	init := r.stmt()
	cond := r.optExpr()
	post := r.stmt()
	body := r.blockStmt()
	perLoopVars := r.Bool()
	r.closeAnotherScope()

	if ir.IsConst(cond, constant.Bool) && !ir.BoolVal(cond) {
		return init // simplify "for init; false; post { ... }" into "init"
	}

	stmt := ir.NewForStmt(pos, init, cond, post, body, perLoopVars)
	stmt.Label = label
	return stmt
}

func (r *reader) ifStmt() ir.Node {
	r.Sync(pkgbits.SyncIfStmt)
	r.openScope()
	pos := r.pos()
	init := r.stmts()
	cond := r.expr()
	staticCond := r.Int()
	var then, els []ir.Node
	if staticCond >= 0 {
		then = r.blockStmt()
	} else {
		r.lastCloseScopePos = r.pos()
	}
	if staticCond <= 0 {
		els = r.stmts()
	}
	r.closeAnotherScope()

	if staticCond != 0 {
		// We may have removed a dead return statement, which can trip up
		// later passes (#62211). To avoid confusion, we instead flatten
		// the if statement into a block.

		if cond.Op() != ir.OLITERAL {
			init.Append(typecheck.Stmt(ir.NewAssignStmt(pos, ir.BlankNode, cond))) // for side effects
		}
		init.Append(then...)
		init.Append(els...)
		return block(init)
	}

	n := ir.NewIfStmt(pos, cond, then, els)
	n.SetInit(init)
	return n
}

func (r *reader) selectStmt(label *types.Sym) ir.Node {
	r.Sync(pkgbits.SyncSelectStmt)

	pos := r.pos()
	clauses := make([]*ir.CommClause, r.Len())
	for i := range clauses {
		if i > 0 {
			r.closeScope()
		}
		r.openScope()

		pos := r.pos()
		comm := r.stmt()
		body := r.stmts()

		// "case i = <-c: ..." may require an implicit conversion (e.g.,
		// see fixedbugs/bug312.go). Currently, typecheck throws away the
		// implicit conversion and relies on it being reinserted later,
		// but that would lose any explicit RTTI operands too. To preserve
		// RTTI, we rewrite this as "case tmp := <-c: i = tmp; ...".
		if as, ok := comm.(*ir.AssignStmt); ok && as.Op() == ir.OAS && !as.Def {
			if conv, ok := as.Y.(*ir.ConvExpr); ok && conv.Op() == ir.OCONVIFACE {
				base.AssertfAt(conv.Implicit(), conv.Pos(), "expected implicit conversion: %v", conv)

				recv := conv.X
				base.AssertfAt(recv.Op() == ir.ORECV, recv.Pos(), "expected receive expression: %v", recv)

				tmp := r.temp(pos, recv.Type())

				// Replace comm with `tmp := <-c`.
				tmpAs := ir.NewAssignStmt(pos, tmp, recv)
				tmpAs.Def = true
				tmpAs.PtrInit().Append(ir.NewDecl(pos, ir.ODCL, tmp))
				comm = tmpAs

				// Change original assignment to `i = tmp`, and prepend to body.
				conv.X = tmp
				body = append([]ir.Node{as}, body...)
			}
		}

		// multiExpr will have desugared a comma-ok receive expression
		// into a separate statement. However, the rest of the compiler
		// expects comm to be the OAS2RECV statement itself, so we need to
		// shuffle things around to fit that pattern.
		if as2, ok := comm.(*ir.AssignListStmt); ok && as2.Op() == ir.OAS2 {
			init := ir.TakeInit(as2.Rhs[0])
			base.AssertfAt(len(init) == 1 && init[0].Op() == ir.OAS2RECV, as2.Pos(), "unexpected assignment: %+v", as2)

			comm = init[0]
			body = append([]ir.Node{as2}, body...)
		}

		clauses[i] = ir.NewCommStmt(pos, comm, body)
	}
	if len(clauses) > 0 {
		r.closeScope()
	}
	n := ir.NewSelectStmt(pos, clauses)
	n.Label = label
	return n
}

func (r *reader) switchStmt(label *types.Sym) ir.Node {
	r.Sync(pkgbits.SyncSwitchStmt)

	r.openScope()
	pos := r.pos()
	init := r.stmt()

	var tag ir.Node
	var ident *ir.Ident
	var iface *types.Type
	if r.Bool() {
		pos := r.pos()
		if r.Bool() {
			ident = ir.NewIdent(r.pos(), r.localIdent())
		}
		x := r.expr()
		iface = x.Type()
		tag = ir.NewTypeSwitchGuard(pos, ident, x)
	} else {
		tag = r.optExpr()
	}

	clauses := make([]*ir.CaseClause, r.Len())
	for i := range clauses {
		if i > 0 {
			r.closeScope()
		}
		r.openScope()

		pos := r.pos()
		var cases, rtypes []ir.Node
		if iface != nil {
			cases = make([]ir.Node, r.Len())
			if len(cases) == 0 {
				cases = nil // TODO(mdempsky): Unclear if this matters.
			}
			for i := range cases {
				if r.Bool() { // case nil
					cases[i] = typecheck.Expr(types.BuiltinPkg.Lookup("nil").Def.(*ir.NilExpr))
				} else {
					cases[i] = r.exprType()
				}
			}
		} else {
			cases = r.exprList()

			// For `switch { case any(true): }` (e.g., issue 3980 in
			// test/switch.go), the backend still creates a mixed bool/any
			// comparison, and we need to explicitly supply the RTTI for the
			// comparison.
			//
			// TODO(mdempsky): Change writer.go to desugar "switch {" into
			// "switch true {", which we already handle correctly.
			if tag == nil {
				for i, cas := range cases {
					if cas.Type().IsEmptyInterface() {
						for len(rtypes) < i {
							rtypes = append(rtypes, nil)
						}
						rtypes = append(rtypes, reflectdata.TypePtrAt(cas.Pos(), types.Types[types.TBOOL]))
					}
				}
			}
		}

		clause := ir.NewCaseStmt(pos, cases, nil)
		clause.RTypes = rtypes

		if ident != nil {
			name := r.curfn.NewLocal(r.pos(), ident.Sym(), r.typ())
			r.addLocal(name)
			clause.Var = name
			name.Defn = tag
		}

		clause.Body = r.stmts()
		clauses[i] = clause
	}
	if len(clauses) > 0 {
		r.closeScope()
	}
	r.closeScope()

	n := ir.NewSwitchStmt(pos, tag, clauses)
	n.Label = label
	if init != nil {
		n.SetInit([]ir.Node{init})
	}
	return n
}

func (r *reader) label() *types.Sym {
	r.Sync(pkgbits.SyncLabel)
	name := r.String()
	if r.inlCall != nil && name != "_" {
		name = fmt.Sprintf("~%s·%d", name, inlgen)
	}
	return typecheck.Lookup(name)
}

func (r *reader) optLabel() *types.Sym {
	r.Sync(pkgbits.SyncOptLabel)
	if r.Bool() {
		return r.label()
	}
	return nil
}

// initDefn marks the given names as declared by defn and populates
// its Init field with ODCL nodes. It then reports whether any names
// were so declared, which can be used to initialize defn.Def.
func (r *reader) initDefn(defn ir.InitNode, names []*ir.Name) bool {
	if len(names) == 0 {
		return false
	}

	init := make([]ir.Node, len(names))
	for i, name := range names {
		name.Defn = defn
		init[i] = ir.NewDecl(name.Pos(), ir.ODCL, name)
	}
	defn.SetInit(init)
	return true
}

// @@@ Expressions

// expr reads and returns a typechecked expression.
func (r *reader) expr() (res ir.Node) {
	defer func() {
		if res != nil && res.Typecheck() == 0 {
			base.FatalfAt(res.Pos(), "%v missed typecheck", res)
		}
	}()

	switch tag := codeExpr(r.Code(pkgbits.SyncExpr)); tag {
	default:
		panic("unhandled expression")

	case exprLocal:
		return typecheck.Expr(r.useLocal())

	case exprGlobal:
		// Callee instead of Expr allows builtins
		// TODO(mdempsky): Handle builtins directly in exprCall, like method calls?
		return typecheck.Callee(r.obj())

	case exprFuncInst:
		origPos, pos := r.origPos()
		wrapperFn, baseFn, dictPtr := r.funcInst(pos)
		if wrapperFn != nil {
			return wrapperFn
		}
		return r.curry(origPos, false, baseFn, dictPtr, nil)

	case exprConst:
		pos := r.pos()
		typ := r.typ()
		val := FixValue(typ, r.Value())
		return ir.NewBasicLit(pos, typ, val)

	case exprZero:
		pos := r.pos()
		typ := r.typ()
		return ir.NewZero(pos, typ)

	case exprCompLit:
		return r.compLit()

	case exprFuncLit:
		return r.funcLit()

	case exprFieldVal:
		x := r.expr()
		pos := r.pos()
		sym := r.selector()

		return typecheck.XDotField(pos, x, sym)

	case exprMethodVal:
		recv := r.expr()
		origPos, pos := r.origPos()
		wrapperFn, baseFn, dictPtr := r.methodExpr()

		// For simple wrapperFn values, the existing machinery for creating
		// and deduplicating wrapperFn value wrappers still works fine.
		if wrapperFn, ok := wrapperFn.(*ir.SelectorExpr); ok && wrapperFn.Op() == ir.OMETHEXPR {
			// The receiver expression we constructed may have a shape type.
			// For example, in fixedbugs/issue54343.go, `New[int]()` is
			// constructed as `New[go.shape.int](&.dict.New[int])`, which
			// has type `*T[go.shape.int]`, not `*T[int]`.
			//
			// However, the method we want to select here is `(*T[int]).M`,
			// not `(*T[go.shape.int]).M`, so we need to manually convert
			// the type back so that the OXDOT resolves correctly.
			//
			// TODO(mdempsky): Logically it might make more sense for
			// exprCall to take responsibility for setting a non-shaped
			// result type, but this is the only place where we care
			// currently. And only because existing ir.OMETHVALUE backend
			// code relies on n.X.Type() instead of n.Selection.Recv().Type
			// (because the latter is types.FakeRecvType() in the case of
			// interface method values).
			//
			if recv.Type().HasShape() {
				typ := wrapperFn.Type().Param(0).Type
				if !types.Identical(typ, recv.Type()) {
					base.FatalfAt(wrapperFn.Pos(), "receiver %L does not match %L", recv, wrapperFn)
				}
				recv = typecheck.Expr(ir.NewConvExpr(recv.Pos(), ir.OCONVNOP, typ, recv))
			}

			n := typecheck.XDotMethod(pos, recv, wrapperFn.Sel, false)

			// As a consistency check here, we make sure "n" selected the
			// same method (represented by a types.Field) that wrapperFn
			// selected. However, for anonymous receiver types, there can be
			// multiple such types.Field instances (#58563). So we may need
			// to fallback to making sure Sym and Type (including the
			// receiver parameter's type) match.
			if n.Selection != wrapperFn.Selection {
				assert(n.Selection.Sym == wrapperFn.Selection.Sym)
				assert(types.Identical(n.Selection.Type, wrapperFn.Selection.Type))
				assert(types.Identical(n.Selection.Type.Recv().Type, wrapperFn.Selection.Type.Recv().Type))
			}

			wrapper := methodValueWrapper{
				rcvr:   n.X.Type(),
				method: n.Selection,
			}

			if r.importedDef() {
				haveMethodValueWrappers = append(haveMethodValueWrappers, wrapper)
			} else {
				needMethodValueWrappers = append(needMethodValueWrappers, wrapper)
			}
			return n
		}

		// For more complicated method expressions, we construct a
		// function literal wrapper.
		return r.curry(origPos, true, baseFn, recv, dictPtr)

	case exprMethodExpr:
		recv := r.typ()

		implicits := make([]int, r.Len())
		for i := range implicits {
			implicits[i] = r.Len()
		}
		var deref, addr bool
		if r.Bool() {
			deref = true
		} else if r.Bool() {
			addr = true
		}

		origPos, pos := r.origPos()
		wrapperFn, baseFn, dictPtr := r.methodExpr()

		// If we already have a wrapper and don't need to do anything with
		// it, we can just return the wrapper directly.
		//
		// N.B., we use implicits/deref/addr here as the source of truth
		// rather than types.Identical, because the latter can be confused
		// by tricky promoted methods (e.g., typeparam/mdempsky/21.go).
		if wrapperFn != nil && len(implicits) == 0 && !deref && !addr {
			if !types.Identical(recv, wrapperFn.Type().Param(0).Type) {
				base.FatalfAt(pos, "want receiver type %v, but have method %L", recv, wrapperFn)
			}
			return wrapperFn
		}

		// Otherwise, if the wrapper function is a static method
		// expression (OMETHEXPR) and the receiver type is unshaped, then
		// we can rely on a statically generated wrapper being available.
		if method, ok := wrapperFn.(*ir.SelectorExpr); ok && method.Op() == ir.OMETHEXPR && !recv.HasShape() {
			return typecheck.NewMethodExpr(pos, recv, method.Sel)
		}

		return r.methodExprWrap(origPos, recv, implicits, deref, addr, baseFn, dictPtr)

	case exprIndex:
		x := r.expr()
		pos := r.pos()
		index := r.expr()
		n := typecheck.Expr(ir.NewIndexExpr(pos, x, index))
		switch n.Op() {
		case ir.OINDEXMAP:
			n := n.(*ir.IndexExpr)
			n.RType = r.rtype(pos)
		}
		return n

	case exprSlice:
		x := r.expr()
		pos := r.pos()
		var index [3]ir.Node
		for i := range index {
			index[i] = r.optExpr()
		}
		op := ir.OSLICE
		if index[2] != nil {
			op = ir.OSLICE3
		}
		return typecheck.Expr(ir.NewSliceExpr(pos, op, x, index[0], index[1], index[2]))

	case exprAssert:
		x := r.expr()
		pos := r.pos()
		typ := r.exprType()
		srcRType := r.rtype(pos)

		// TODO(mdempsky): Always emit ODYNAMICDOTTYPE for uniformity?
		if typ, ok := typ.(*ir.DynamicType); ok && typ.Op() == ir.ODYNAMICTYPE {
			assert := ir.NewDynamicTypeAssertExpr(pos, ir.ODYNAMICDOTTYPE, x, typ.RType)
			assert.SrcRType = srcRType
			assert.ITab = typ.ITab
			return typed(typ.Type(), assert)
		}
		return typecheck.Expr(ir.NewTypeAssertExpr(pos, x, typ.Type()))

	case exprUnaryOp:
		op := r.op()
		pos := r.pos()
		x := r.expr()

		switch op {
		case ir.OADDR:
			return typecheck.Expr(typecheck.NodAddrAt(pos, x))
		case ir.ODEREF:
			return typecheck.Expr(ir.NewStarExpr(pos, x))
		}
		return typecheck.Expr(ir.NewUnaryExpr(pos, op, x))

	case exprBinaryOp:
		op := r.op()
		x := r.expr()
		pos := r.pos()
		y := r.expr()

		switch op {
		case ir.OANDAND, ir.OOROR:
			return typecheck.Expr(ir.NewLogicalExpr(pos, op, x, y))
		case ir.OLSH, ir.ORSH:
			// Untyped rhs of non-constant shift, e.g. x << 1.0.
			// If we have a constant value, it must be an int >= 0.
			if ir.IsConstNode(y) {
				val := constant.ToInt(y.Val())
				assert(val.Kind() == constant.Int && constant.Sign(val) >= 0)
			}
		}
		return typecheck.Expr(ir.NewBinaryExpr(pos, op, x, y))

	case exprRecv:
		x := r.expr()
		pos := r.pos()
		for i, n := 0, r.Len(); i < n; i++ {
			x = Implicit(typecheck.DotField(pos, x, r.Len()))
		}
		if r.Bool() { // needs deref
			x = Implicit(Deref(pos, x.Type().Elem(), x))
		} else if r.Bool() { // needs addr
			x = Implicit(Addr(pos, x))
		}
		return x

	case exprCall:
		var fun ir.Node
		var args ir.Nodes
		if r.Bool() { // method call
			recv := r.expr()
			_, method, dictPtr := r.methodExpr()

			if recv.Type().IsInterface() && method.Op() == ir.OMETHEXPR {
				method := method.(*ir.SelectorExpr)

				// The compiler backend (e.g., devirtualization) handle
				// OCALLINTER/ODOTINTER better than OCALLFUNC/OMETHEXPR for
				// interface calls, so we prefer to continue constructing
				// calls that way where possible.
				//
				// There are also corner cases where semantically it's perhaps
				// significant; e.g., fixedbugs/issue15975.go, #38634, #52025.

				fun = typecheck.XDotMethod(method.Pos(), recv, method.Sel, true)
			} else {
				if recv.Type().IsInterface() {
					// N.B., this happens currently for typeparam/issue51521.go
					// and typeparam/typeswitch3.go.
					if base.Flag.LowerM != 0 {
						base.WarnfAt(method.Pos(), "imprecise interface call")
					}
				}

				fun = method
				args.Append(recv)
			}
			if dictPtr != nil {
				args.Append(dictPtr)
			}
		} else if r.Bool() { // call to instanced function
			pos := r.pos()
			_, shapedFn, dictPtr := r.funcInst(pos)
			fun = shapedFn
			args.Append(dictPtr)
		} else {
			fun = r.expr()
		}
		pos := r.pos()
		args.Append(r.multiExpr()...)
		dots := r.Bool()
		n := typecheck.Call(pos, fun, args, dots)
		switch n.Op() {
		case ir.OAPPEND:
			n := n.(*ir.CallExpr)
			n.RType = r.rtype(pos)
			// For append(a, b...), we don't need the implicit conversion. The typechecker already
			// ensured that a and b are both slices with the same base type, or []byte and string.
			if n.IsDDD {
				if conv, ok := n.Args[1].(*ir.ConvExpr); ok && conv.Op() == ir.OCONVNOP && conv.Implicit() {
					n.Args[1] = conv.X
				}
			}
		case ir.OCOPY:
			n := n.(*ir.BinaryExpr)
			n.RType = r.rtype(pos)
		case ir.ODELETE:
			n := n.(*ir.CallExpr)
			n.RType = r.rtype(pos)
		case ir.OUNSAFESLICE:
			n := n.(*ir.BinaryExpr)
			n.RType = r.rtype(pos)
		}
		return n

	case exprMake:
		pos := r.pos()
		typ := r.exprType()
		extra := r.exprs()
		n := typecheck.Expr(ir.NewCallExpr(pos, ir.OMAKE, nil, append([]ir.Node{typ}, extra...))).(*ir.MakeExpr)
		n.RType = r.rtype(pos)
		return n

	case exprNew:
		pos := r.pos()
		if r.Bool() {
			// new(expr) -> tmp := expr; &tmp
			x := r.expr()
			x = typecheck.DefaultLit(x, nil) // See TODO in exprConvert case.
			var init ir.Nodes
			addr := ir.NewAddrExpr(pos, r.tempCopy(pos, x, &init))
			addr.SetInit(init)
			return typecheck.Expr(addr)
		}
		// new(T)
		return typecheck.Expr(ir.NewUnaryExpr(pos, ir.ONEW, r.exprType()))

	case exprSizeof:
		return ir.NewUintptr(r.pos(), r.typ().Size())

	case exprAlignof:
		return ir.NewUintptr(r.pos(), r.typ().Alignment())

	case exprOffsetof:
		pos := r.pos()
		typ := r.typ()
		types.CalcSize(typ)

		var offset int64
		for i := r.Len(); i >= 0; i-- {
			field := typ.Field(r.Len())
			offset += field.Offset
			typ = field.Type
		}

		return ir.NewUintptr(pos, offset)

	case exprReshape:
		typ := r.typ()
		x := r.expr()

		if types.IdenticalStrict(x.Type(), typ) {
			return x
		}

		// Comparison expressions are constructed as "untyped bool" still.
		//
		// TODO(mdempsky): It should be safe to reshape them here too, but
		// maybe it's better to construct them with the proper type
		// instead.
		if x.Type() == types.UntypedBool && typ.IsBoolean() {
			return x
		}

		base.AssertfAt(x.Type().HasShape() || typ.HasShape(), x.Pos(), "%L and %v are not shape types", x, typ)
		base.AssertfAt(types.Identical(x.Type(), typ), x.Pos(), "%L is not shape-identical to %v", x, typ)

		// We use ir.HasUniquePos here as a check that x only appears once
		// in the AST, so it's okay for us to call SetType without
		// breaking any other uses of it.
		//
		// Notably, any ONAMEs should already have the exactly right shape
		// type and been caught by types.IdenticalStrict above.
		base.AssertfAt(ir.HasUniquePos(x), x.Pos(), "cannot call SetType(%v) on %L", typ, x)

		if base.Debug.Reshape != 0 {
			base.WarnfAt(x.Pos(), "reshaping %L to %v", x, typ)
		}

		x.SetType(typ)

		if call, ok := x.(*ir.CallExpr); ok {
			call.Reshape = true
		}

		return x

	case exprConvert:
		implicit := r.Bool()
		typ := r.typ()
		pos := r.pos()
		typeWord, srcRType := r.convRTTI(pos)
		dstTypeParam := r.Bool()
		identical := r.Bool()
		x := r.expr()

		// spec: "If the type is a type parameter, the constant is converted
		// into a non-constant value of the type parameter."
		if dstTypeParam && ir.IsConstNode(x) {
			// ConvertVal only handles conversions to constant types.
			if v := typecheck.ConvertVal(x.Val(), typ, false); v.Kind() != constant.Unknown {
				x = ir.NewBasicLit(x.Pos(), typ, v)
				// Wrap in an OCONVNOP node to ensure result is non-constant.
				n := Implicit(ir.NewConvExpr(pos, ir.OCONVNOP, typ, x))
				n.SetTypecheck(1)
				return n
			}
			// A Go language constant could be converted to a non-constant value,
			// like converting string to []byte/[]rune. In this case, just construct
			// the conversion expression as usual, see #79960.
		}

		// TODO(mdempsky): Stop constructing expressions of untyped type.
		x = typecheck.DefaultLit(x, typ)

		ce := ir.NewConvExpr(pos, ir.OCONV, typ, x)
		ce.TypeWord, ce.SrcRType = typeWord, srcRType
		if implicit {
			ce.SetImplicit(true)
		}
		n := typecheck.Expr(ce)

		// Conversions between non-identical, non-empty interfaces always
		// requires a runtime call, even if they have identical underlying
		// interfaces. This is because we create separate itab instances
		// for each unique interface type, not merely each unique
		// interface shape.
		//
		// However, due to shape types, typecheck.Expr might mistakenly
		// think a conversion between two non-empty interfaces are
		// identical and set ir.OCONVNOP, instead of ir.OCONVIFACE. To
		// ensure we update the itab field appropriately, we force it to
		// ir.OCONVIFACE instead when shape types are involved.
		//
		// TODO(mdempsky): Are there other places we might get this wrong?
		// Should this be moved down into typecheck.{Assign,Convert}op?
		// This would be a non-issue if itabs were unique for each
		// *underlying* interface type instead.
		if !identical {
			if n, ok := n.(*ir.ConvExpr); ok && n.Op() == ir.OCONVNOP && n.Type().IsInterface() && !n.Type().IsEmptyInterface() && (n.Type().HasShape() || n.X.Type().HasShape()) {
				n.SetOp(ir.OCONVIFACE)
			}
		}

		return n

	case exprRuntimeBuiltin:
		builtin := typecheck.LookupRuntime(r.String())
		return builtin
	}
}

// funcInst reads an instantiated function reference, and returns
// three (possibly nil) expressions related to it:
//
// baseFn is always non-nil: it's either a function of the appropriate
// type already, or it has an extra dictionary parameter as the first
// parameter.
//
// If dictPtr is non-nil, then it's a dictionary argument that must be
// passed as the first argument to baseFn.
//
// If wrapperFn is non-nil, then it's either the same as baseFn (if
// dictPtr is nil), or it's semantically equivalent to currying baseFn
// to pass dictPtr. (wrapperFn is nil when dictPtr is an expression
// that needs to be computed dynamically.)
//
// For callers that are creating a call to the returned function, it's
// best to emit a call to baseFn, and include dictPtr in the arguments
// list as appropriate.
//
// For callers that want to return the function without invoking it,
// they may return wrapperFn if it's non-nil; but otherwise, they need
// to create their own wrapper.
func (r *reader) funcInst(pos src.XPos) (wrapperFn, baseFn, dictPtr ir.Node) {
	// Like in methodExpr, I'm pretty sure this isn't needed.
	var implicits []*types.Type
	if r.dict != nil {
		implicits = r.dict.targs
	}

	if r.Bool() { // dynamic subdictionary
		idx := r.Len()
		info := r.dict.subdicts[idx]
		explicits := r.p.typListIdx(info.explicits, r.dict)

		baseFn = r.p.objIdx(info.idx, implicits, explicits, true).(*ir.Name)

		// TODO(mdempsky): Is there a more robust way to get the
		// dictionary pointer type here?
		dictPtrType := baseFn.Type().Param(0).Type
		dictPtr = typecheck.Expr(ir.NewConvExpr(pos, ir.OCONVNOP, dictPtrType, r.dictWord(pos, r.dict.subdictsOffset()+idx)))

		return
	}

	info := r.objInfo()
	explicits := r.p.typListIdx(info.explicits, r.dict)

	wrapperFn = r.p.objIdx(info.idx, implicits, explicits, false).(*ir.Name)
	baseFn = r.p.objIdx(info.idx, implicits, explicits, true).(*ir.Name)

	dictName := r.p.objDictName(info.idx, implicits, explicits)
	dictPtr = typecheck.Expr(ir.NewAddrExpr(pos, dictName))

	return
}

func (pr *pkgReader) objDictName(idx index, implicits, explicits []*types.Type) *ir.Name {
	rname := pr.newReader(pkgbits.SectionName, idx, pkgbits.SyncObject1)
	_, sym := rname.qualifiedIdent()
	tag := pkgbits.CodeObj(rname.Code(pkgbits.SyncCodeObj))

	if tag == pkgbits.ObjStub {
		assert(!sym.IsBlank())
		if pri, ok := objReader[sym]; ok {
			return pri.pr.objDictName(pri.idx, nil, explicits)
		}
		base.Fatalf("unresolved stub: %v", sym)
	}

	dict, err := pr.objDictIdx(sym, idx, implicits, explicits, false)
	if err != nil {
		base.Fatalf("%v", err)
	}

	return pr.dictNameOf(dict)
}

// curry returns a function literal that calls fun with arg0 and
// (optionally) arg1, accepting additional arguments to the function
// literal as necessary to satisfy fun's signature.
//
// If nilCheck is true and arg0 is an interface value, then it's
// checked to be non-nil as an initial step at the point of evaluating
// the function literal itself.
func (r *reader) curry(origPos src.XPos, ifaceHack bool, fun ir.Node, arg0, arg1 ir.Node) ir.Node {
	var captured ir.Nodes
	captured.Append(fun, arg0)
	if arg1 != nil {
		captured.Append(arg1)
	}

	params, results := syntheticSig(fun.Type())
	params = params[len(captured)-1:] // skip curried parameters
	typ := types.NewSignature(nil, params, results)

	addBody := func(pos src.XPos, r *reader, captured []ir.Node) {
		fun := captured[0]

		var args ir.Nodes
		args.Append(captured[1:]...)
		args.Append(r.syntheticArgs()...)

		r.syntheticTailCall(pos, fun, args)
	}

	return r.syntheticClosure(origPos, typ, ifaceHack, captured, addBody)
}

// methodExprWrap returns a function literal that changes method's
// first parameter's type to recv, and uses implicits/deref/addr to
// select the appropriate receiver parameter to pass to method.
func (r *reader) methodExprWrap(origPos src.XPos, recv *types.Type, implicits []int, deref, addr bool, method, dictPtr ir.Node) ir.Node {
	var captured ir.Nodes
	captured.Append(method)

	params, results := syntheticSig(method.Type())

	// Change first parameter to recv.
	params[0].Type = recv

	// If we have a dictionary pointer argument to pass, then omit the
	// underlying method expression's dictionary parameter from the
	// returned signature too.
	if dictPtr != nil {
		captured.Append(dictPtr)
		params = append(params[:1], params[2:]...)
	}

	typ := types.NewSignature(nil, params, results)

	addBody := func(pos src.XPos, r *reader, captured []ir.Node) {
		fn := captured[0]
		args := r.syntheticArgs()

		// Rewrite first argument based on implicits/deref/addr.
		{
			arg := args[0]
			for _, ix := range implicits {
				arg = Implicit(typecheck.DotField(pos, arg, ix))
			}
			if deref {
				arg = Implicit(Deref(pos, arg.Type().Elem(), arg))
			} else if addr {
				arg = Implicit(Addr(pos, arg))
			}
			args[0] = arg
		}

		// Insert dictionary argument, if provided.
		if dictPtr != nil {
			newArgs := make([]ir.Node, len(args)+1)
			newArgs[0] = args[0]
			newArgs[1] = captured[1]
			copy(newArgs[2:], args[1:])
			args = newArgs
		}

		r.syntheticTailCall(pos, fn, args)
	}

	return r.syntheticClosure(origPos, typ, false, captured, addBody)
}

// syntheticClosure constructs a synthetic function literal for
// currying dictionary arguments. origPos is the position used for the
// closure, which must be a non-inlined position. typ is the function
// literal's signature type.
//
// captures is a list of expressions that need to be evaluated at the
// point of function literal evaluation and captured by the function
// literal. If ifaceHack is true and captures[1] is an interface type,
// it's checked to be non-nil after evaluation.
//
// addBody is a callback function to populate the function body. The
// list of captured values passed back has the captured variables for
// use within the function literal, corresponding to the expressions
// in captures.
func (r *reader) syntheticClosure(origPos src.XPos, typ *types.Type, ifaceHack bool, captures ir.Nodes, addBody func(pos src.XPos, r *reader, captured []ir.Node)) ir.Node {
	// isSafe reports whether n is an expression that we can safely
	// defer to evaluating inside the closure instead, to avoid storing
	// them into the closure.
	//
	// In practice this is always (and only) the wrappee function.
	isSafe := func(n ir.Node) bool {
		if n.Op() == ir.ONAME && n.(*ir.Name).Class == ir.PFUNC {
			return true
		}
		if n.Op() == ir.OMETHEXPR {
			return true
		}

		return false
	}

	fn := r.inlClosureFunc(origPos, typ, ir.OCLOSURE)
	fn.SetWrapper(true)

	clo := fn.OClosure
	inlPos := clo.Pos()

	var init ir.Nodes
	for i, n := range captures {
		if isSafe(n) {
			continue // skip capture; can reference directly
		}

		tmp := r.tempCopy(inlPos, n, &init)
		ir.NewClosureVar(origPos, fn, tmp)

		// We need to nil check interface receivers at the point of method
		// value evaluation, ugh.
		if ifaceHack && i == 1 && n.Type().IsInterface() {
			check := ir.NewUnaryExpr(inlPos, ir.OCHECKNIL, ir.NewUnaryExpr(inlPos, ir.OITAB, tmp))
			init.Append(typecheck.Stmt(check))
		}
	}

	pri := pkgReaderIndex{synthetic: func(pos src.XPos, r *reader) {
		captured := make([]ir.Node, len(captures))
		next := 0
		for i, n := range captures {
			if isSafe(n) {
				captured[i] = n
			} else {
				captured[i] = r.closureVars[next]
				next++
			}
		}
		assert(next == len(r.closureVars))

		addBody(origPos, r, captured)
	}}
	bodyReader[fn] = pri
	pri.funcBody(fn)

	return ir.InitExpr(init, clo)
}

// syntheticSig duplicates and returns the params and results lists
// for sig, but renaming anonymous parameters so they can be assigned
// ir.Names.
func syntheticSig(sig *types.Type) (params, results []*types.Field) {
	clone := func(params []*types.Field) []*types.Field {
		res := make([]*types.Field, len(params))
		for i, param := range params {
			// TODO(mdempsky): It would be nice to preserve the original
			// parameter positions here instead, but at least
			// typecheck.NewMethodType replaces them with base.Pos, making
			// them useless. Worse, the positions copied from base.Pos may
			// have inlining contexts, which we definitely don't want here
			// (e.g., #54625).
			res[i] = types.NewField(base.AutogeneratedPos, param.Sym, param.Type)
			res[i].SetIsDDD(param.IsDDD())
		}
		return res
	}

	return clone(sig.Params()), clone(sig.Results())
}

func (r *reader) optExpr() ir.Node {
	if r.Bool() {
		return r.expr()
	}
	return nil
}

// methodExpr reads a method expression reference, and returns three
// (possibly nil) expressions related to it:
//
// baseFn is always non-nil: it's either a function of the appropriate
// type already, or it has an extra dictionary parameter as the second
// parameter (i.e., immediately after the promoted receiver
// parameter).
//
// If dictPtr is non-nil, then it's a dictionary argument that must be
// passed as the second argument to baseFn.
//
// If wrapperFn is non-nil, then it's either the same as baseFn (if
// dictPtr is nil), or it's semantically equivalent to currying baseFn
// to pass dictPtr. (wrapperFn is nil when dictPtr is an expression
// that needs to be computed dynamically.)
//
// For callers that are creating a call to the returned method, it's
// best to emit a call to baseFn, and include dictPtr in the arguments
// list as appropriate.
//
// For callers that want to return a method expression without
// invoking it, they may return wrapperFn if it's non-nil; but
// otherwise, they need to create their own wrapper.
func (r *reader) methodExpr() (wrapperFn, baseFn, dictPtr ir.Node) {
	recv := r.typ()

	var sig *types.Type
	generic := r.Version().Has(pkgbits.GenericMethods) && r.Bool()
	if !generic {
		// Signature type to return (i.e., recv prepended to the method's
		// normal parameters list).
		sig = typecheck.NewMethodType(r.typ(), recv)
	}

	pos := r.pos()
	sym := r.selector()

	if r.Bool() { // type parameter method expression
		idx := r.Len()
		word := r.dictWord(pos, r.dict.typeParamMethodExprsOffset()+idx)

		// TODO(mdempsky): If the type parameter was instantiated with an
		// interface type (i.e., embed.IsInterface()), then we could
		// return the OMETHEXPR instead and save an indirection.

		// We wrote the method expression's entry point PC into the
		// dictionary, but for Go `func` values we need to return a
		// closure (i.e., pointer to a structure with the PC as the first
		// field). Because method expressions don't have any closure
		// variables, we pun the dictionary entry as the closure struct.
		fn := typecheck.Expr(ir.NewConvExpr(pos, ir.OCONVNOP, sig, ir.NewAddrExpr(pos, word)))
		return fn, fn, nil
	}

	if r.Bool() { // dynamic subdictionary
		idx := r.Len()
		info := r.dict.subdicts[idx]
		explicits := r.p.typListIdx(info.explicits, r.dict)

		shapedObj := r.p.objIdx(info.idx, nil, explicits, true).(*ir.Name)
		shapedFn := shapedMethodExpr(pos, shapedObj, sym)

		// TODO(mdempsky): Is there a more robust way to get the
		// dictionary pointer type here?
		dictPtrType := shapedFn.Type().Param(1).Type
		dictPtr := typecheck.Expr(ir.NewConvExpr(pos, ir.OCONVNOP, dictPtrType, r.dictWord(pos, r.dict.subdictsOffset()+idx)))

		return nil, shapedFn, dictPtr
	}

	if r.Bool() { // static dictionary
		info := r.objInfo()
		explicits := r.p.typListIdx(info.explicits, r.dict)

		shapedObj := r.p.objIdx(info.idx, nil, explicits, true).(*ir.Name)
		shapedFn := shapedMethodExpr(pos, shapedObj, sym)

		dict := r.p.objDictName(info.idx, nil, explicits)
		dictPtr := typecheck.Expr(ir.NewAddrExpr(pos, dict))

		// Check that dictPtr matches shapedFn's dictionary parameter.
		if !types.Identical(dictPtr.Type(), shapedFn.Type().Param(1).Type) {
			base.FatalfAt(pos, "dict %L, but shaped method %L", dict, shapedFn)
		}

		if !generic {
			// For statically known instantiations, we can take advantage of
			// the stenciled wrapper.
			base.AssertfAt(!recv.HasShape(), pos, "shaped receiver %v", recv)
			wrapperFn := typecheck.NewMethodExpr(pos, recv, sym)
			base.AssertfAt(types.Identical(sig, wrapperFn.Type()), pos, "wrapper %L does not have type %v", wrapperFn, sig)
			return wrapperFn, shapedFn, dictPtr
		} else {
			// Also statically known, but there is a good amount of existing
			// machinery downstream which makes assumptions about method
			// wrapper functions. It's safest not to emit them for now.
			// TODO(mark): Emit wrapper functions for generic methods.
			return nil, shapedFn, dictPtr
		}
	}

	// Simple method expression; no dictionary needed.
	base.AssertfAt(!recv.HasShape() || recv.IsInterface(), pos, "shaped receiver %v", recv)
	fn := typecheck.NewMethodExpr(pos, recv, sym)
	return fn, fn, nil
}

// shapedMethodExpr creates an OMETHEXPR for obj using sym.
//
// If obj is an OTYPE, it must refer to a generic type. If obj is an ONAME,
// it must refer to a generic method. In either case, sym.Name must be the
// unqualified name of the method.
//
// For example, given:
//
//	package p
//
//	type T[P any] struct {}
//
//	func (T[P]) m() {}
//	func (T[P]) n[Q any]() {}
//
// then, using S as go.shape.int:
//   - in T[int].m,      obj is T[S]      and sym.Name is "m".
//   - in T[int].n[int], obj is T[S].n[S] and sym.Name is "n".
//
// Note that we could have pushed dictionaries down to methods in every case,
// but since non-generic methods will always share the same "type environment"
// as their defining type, we can optimize by reusing the type's dictionary.
func shapedMethodExpr(pos src.XPos, obj *ir.Name, sym *types.Sym) ir.Node {
	if obj.Op() == ir.OTYPE {
		// non-generic method on generic type
		typ := obj.Type()
		assert(typ.HasShape())

		method := func() *types.Field {
			for _, m := range typ.Methods() {
				if m.Sym == sym {
					return m
				}
			}

			base.FatalfAt(pos, "failed to find method %v in shaped type %v", sym, typ)
			panic("unreachable")
		}()

		return typecheck.NewMethodExpr(pos, method.Type.Recv().Type, sym)
	} else {
		// generic method on possibly generic type
		assert(obj.Op() == ir.ONAME && obj.Class == ir.PFUNC)
		typ := obj.Type()
		assert(typ.HasShape())

		// OMETHEXPR assumes that the linker symbol to call looks like "<type sym>.<method sym>".
		// This works because non-generic method symbols are relative to their type. But generic
		// methods use fully-qualified names, so this won't work.
		//
		// To use OMETHEXPR for generic methods, we craft a dummy field on the type by removing
		// the qualifier; OMETHEXPR will put it back later.
		lsym := obj.Linksym().Name
		// Since the method is generic, we know the method name must be followed by a bracket.
		// TODO(mark): It's not ideal to rely on string naming here. Find a more robust solution.
		msym := sym.Pkg.Lookup(lsym[strings.LastIndex(lsym, sym.Name+"["):])

		// Note that the field name here includes the type arguments; while also not ideal, the
		// types package does not seem to complain.
		m := types.NewField(obj.Pos(), msym, typ)
		m.Nname = obj

		n := ir.NewSelectorExpr(pos, ir.OMETHEXPR, ir.TypeNode(typ.Recv().Type), msym)
		n.Selection = m
		n.SetType(typecheck.NewMethodType(typ, typ.Recv().Type))
		n.SetTypecheck(1)

		return n
	}
}

func (r *reader) multiExpr() []ir.Node {
	r.Sync(pkgbits.SyncMultiExpr)

	if r.Bool() { // N:1
		pos := r.pos()
		expr := r.expr()

		results := make([]ir.Node, r.Len())
		as := ir.NewAssignListStmt(pos, ir.OAS2, nil, []ir.Node{expr})
		as.Def = true
		for i := range results {
			tmp := r.temp(pos, r.typ())
			tmp.Defn = as
			as.PtrInit().Append(ir.NewDecl(pos, ir.ODCL, tmp))
			as.Lhs.Append(tmp)

			res := ir.Node(tmp)
			if r.Bool() {
				n := ir.NewConvExpr(pos, ir.OCONV, r.typ(), res)
				n.TypeWord, n.SrcRType = r.convRTTI(pos)
				n.SetImplicit(true)
				res = typecheck.Expr(n)
			}
			results[i] = res
		}

		// TODO(mdempsky): Could use ir.InlinedCallExpr instead?
		results[0] = ir.InitExpr([]ir.Node{typecheck.Stmt(as)}, results[0])
		return results
	}

	// N:N
	exprs := make([]ir.Node, r.Len())
	if len(exprs) == 0 {
		return nil
	}
	for i := range exprs {
		exprs[i] = r.expr()
	}
	return exprs
}

// temp returns a new autotemp of the specified type.
func (r *reader) temp(pos src.XPos, typ *types.Type) *ir.Name {
	return typecheck.TempAt(pos, r.curfn, typ)
}

// tempCopy declares and returns a new autotemp initialized to the
// value of expr.
func (r *reader) tempCopy(pos src.XPos, expr ir.Node, init *ir.Nodes) *ir.Name {
	tmp := r.temp(pos, expr.Type())

	init.Append(typecheck.Stmt(ir.NewDecl(pos, ir.ODCL, tmp)))

	assign := ir.NewAssignStmt(pos, tmp, expr)
	assign.Def = true
	init.Append(typecheck.Stmt(assign))

	tmp.Defn = assign

	return tmp
}

func (r *reader) compLit() ir.Node {
	r.Sync(pkgbits.SyncCompLit)
	pos := r.pos()
	typ0 := r.typ()

	typ := typ0
	if typ.IsPtr() {
		typ = typ.Elem()
	}
	if typ.Kind() == types.TFORW {
		base.FatalfAt(pos, "unresolved composite literal type: %v", typ)
	}
	var rtype ir.Node
	if typ.IsMap() {
		rtype = r.rtype(pos)
	}

	var elems []ir.Node
	if r.Version().Has(pkgbits.CompactCompLiterals) {
		n := r.Int()
		elems = make([]ir.Node, max(n, -n) /* abs(n) */)
		switch typ.Kind() {
		default:
			base.FatalfAt(pos, "unexpected composite literal type: %v", typ)
		case types.TARRAY:
			r.arrayElems(n >= 0, elems)
		case types.TMAP:
			r.mapElems(elems)
		case types.TSLICE:
			r.arrayElems(n >= 0, elems)
		case types.TSTRUCT:
			r.structElems(typ, n >= 0, elems)
		}
	} else {
		elems = make([]ir.Node, r.Len())
		isStruct := typ.Kind() == types.TSTRUCT
		for i := range elems {
			elemp := &elems[i]
			if isStruct {
				sk := ir.NewStructKeyExpr(r.pos(), typ.Field(r.Len()), nil)
				*elemp, elemp = sk, &sk.Value
			} else if r.Bool() {
				kv := ir.NewKeyExpr(r.pos(), r.expr(), nil)
				*elemp, elemp = kv, &kv.Value
			}
			*elemp = r.expr()
		}
	}

	lit := typecheck.Expr(ir.NewCompLitExpr(pos, ir.OCOMPLIT, typ, elems))
	if rtype != nil {
		lit := lit.(*ir.CompLitExpr)
		lit.RType = rtype
	}
	if typ0.IsPtr() {
		lit = typecheck.Expr(typecheck.NodAddrAt(pos, lit))
		lit.SetType(typ0)
	}
	return lit
}

func (r *reader) arrayElems(valuesOnly bool, elems []ir.Node) {
	if valuesOnly {
		for i := range elems {
			elems[i] = r.expr()
		}
		return
	}
	// some elements may have a key
	for i := range elems {
		if r.Bool() {
			kv := ir.NewKeyExpr(r.pos(), r.expr(), nil)
			kv.Value = r.expr()
			elems[i] = kv
		} else {
			elems[i] = r.expr()
		}
	}
}

func (r *reader) mapElems(elems []ir.Node) {
	// all elements have a key
	for i := range elems {
		kv := ir.NewKeyExpr(r.pos(), r.expr(), nil)
		kv.Value = r.expr()
		elems[i] = kv
	}
}

func (r *reader) structElems(typ *types.Type, valuesOnly bool, elems []ir.Node) {
	if valuesOnly {
		for i := range elems {
			sk := ir.NewStructKeyExpr(r.pos(), typ.Field(i), nil)
			sk.Value = r.expr()
			elems[i] = sk
		}
		return
	}

	// all elements have a key
	for i := range elems {
		pos := r.pos()
		var fld *types.Field
		if n := r.Int(); n < 0 {
			// embedded field
			typ := typ // don't modify the original typ
			for range -n {
				fld = typ.Field(r.Int())
				typ = fld.Type
			}
		} else { // n >= 0
			fld = typ.Field(n)
		}
		sk := ir.NewStructKeyExpr(pos, fld, nil)
		sk.Value = r.expr()
		elems[i] = sk
	}
}

func (r *reader) funcLit() ir.Node {
	r.Sync(pkgbits.SyncFuncLit)

	// The underlying function declaration (including its parameters'
	// positions, if any) need to remain the original, uninlined
	// positions. This is because we track inlining-context on nodes so
	// we can synthesize the extra implied stack frames dynamically when
	// generating tracebacks, whereas those stack frames don't make
	// sense *within* the function literal. (Any necessary inlining
	// adjustments will have been applied to the call expression
	// instead.)
	//
	// This is subtle, and getting it wrong leads to cycles in the
	// inlining tree, which lead to infinite loops during stack
	// unwinding (#46234, #54625).
	//
	// Note that we *do* want the inline-adjusted position for the
	// OCLOSURE node, because that position represents where any heap
	// allocation of the closure is credited (#49171).
	r.suppressInlPos++
	origPos := r.pos()
	sig := r.signature(nil)
	r.suppressInlPos--
	why := ir.OCLOSURE
	if r.Bool() {
		why = ir.ORANGE
	}

	fn := r.inlClosureFunc(origPos, sig, why)

	fn.ClosureVars = make([]*ir.Name, 0, r.Len())
	for len(fn.ClosureVars) < cap(fn.ClosureVars) {
		// TODO(mdempsky): I think these should be original positions too
		// (i.e., not inline-adjusted).
		ir.NewClosureVar(r.pos(), fn, r.useLocal())
	}
	if param := r.dictParam; param != nil {
		// If we have a dictionary parameter, capture it too. For
		// simplicity, we capture it last and unconditionally.
		ir.NewClosureVar(param.Pos(), fn, param)
	}

	r.addBody(fn, nil)

	return fn.OClosure
}

// inlClosureFunc constructs a new closure function, but correctly
// handles inlining.
func (r *reader) inlClosureFunc(origPos src.XPos, sig *types.Type, why ir.Op) *ir.Func {
	curfn := r.inlCaller
	if curfn == nil {
		curfn = r.curfn
	}

	var gen int
	if why == ir.ORANGE {
		r.rangeLitGen++
		gen = r.rangeLitGen
	} else {
		r.funcLitGen++
		gen = r.funcLitGen
	}

	// TODO(mdempsky): Remove hard-coding of typecheck.Target.
	return ir.NewClosureFunc(origPos, r.inlPos(origPos), why, sig, curfn, typecheck.Target, gen)
}

func (r *reader) exprList() []ir.Node {
	r.Sync(pkgbits.SyncExprList)
	return r.exprs()
}

func (r *reader) exprs() []ir.Node {
	r.Sync(pkgbits.SyncExprs)
	nodes := make([]ir.Node, r.Len())
	if len(nodes) == 0 {
		return nil // TODO(mdempsky): Unclear if this matters.
	}
	for i := range nodes {
		nodes[i] = r.expr()
	}
	return nodes
}

// dictWord returns an expression to return the specified
// uintptr-typed word from the dictionary parameter.
func (r *reader) dictWord(pos src.XPos, idx int) ir.Node {
	base.AssertfAt(r.dictParam != nil, pos, "expected dictParam in %v", r.curfn)
	return typecheck.Expr(ir.NewIndexExpr(pos, r.dictParam, ir.NewInt(pos, int64(idx))))
}

// rttiWord is like dictWord, but converts it to *byte (the type used
// internally to represent *runtime._type and *runtime.itab).
func (r *reader) rttiWord(pos src.XPos, idx int) ir.Node {
	return typecheck.Expr(ir.NewConvExpr(pos, ir.OCONVNOP, types.NewPtr(types.Types[types.TUINT8]), r.dictWord(pos, idx)))
}

// rtype reads a type reference from the element bitstream, and
// returns an expression of type *runtime._type representing that
// type.
func (r *reader) rtype(pos src.XPos) ir.Node {
	_, rtype := r.rtype0(pos)
	return rtype
}

func (r *reader) rtype0(pos src.XPos) (typ *types.Type, rtype ir.Node) {
	r.Sync(pkgbits.SyncRType)
	if r.Bool() { // derived type
		idx := r.Len()
		info := r.dict.rtypes[idx]
		typ = r.p.typIdx(info, r.dict, true)
		rtype = r.rttiWord(pos, r.dict.rtypesOffset()+idx)
		return
	}

	typ = r.typ()
	rtype = reflectdata.TypePtrAt(pos, typ)
	return
}

// varDictIndex populates name.DictIndex if name is a derived type.
func (r *reader) varDictIndex(name *ir.Name) {
	if r.Bool() {
		idx := 1 + r.dict.rtypesOffset() + r.Len()
		if int(uint16(idx)) != idx {
			base.FatalfAt(name.Pos(), "DictIndex overflow for %v: %v", name, idx)
		}
		name.DictIndex = uint16(idx)
	}
}

// itab returns a (typ, iface) pair of types.
//
// typRType and ifaceRType are expressions that evaluate to the
// *runtime._type for typ and iface, respectively.
//
// If typ is a concrete type and iface is a non-empty interface type,
// then itab is an expression that evaluates to the *runtime.itab for
// the pair. Otherwise, itab is nil.
func (r *reader) itab(pos src.XPos) (typ *types.Type, typRType ir.Node, iface *types.Type, ifaceRType ir.Node, itab ir.Node) {
	typ, typRType = r.rtype0(pos)
	iface, ifaceRType = r.rtype0(pos)

	idx := -1
	if r.Bool() {
		idx = r.Len()
	}

	if !typ.IsInterface() && iface.IsInterface() && !iface.IsEmptyInterface() {
		if idx >= 0 {
			itab = r.rttiWord(pos, r.dict.itabsOffset()+idx)
		} else {
			base.AssertfAt(!typ.HasShape(), pos, "%v is a shape type", typ)
			base.AssertfAt(!iface.HasShape(), pos, "%v is a shape type", iface)

			lsym := reflectdata.ITabLsym(typ, iface)
			itab = typecheck.LinksymAddr(pos, lsym, types.Types[types.TUINT8])
		}
	}

	return
}

// convRTTI returns expressions appropriate for populating an
// ir.ConvExpr's TypeWord and SrcRType fields, respectively.
func (r *reader) convRTTI(pos src.XPos) (typeWord, srcRType ir.Node) {
	r.Sync(pkgbits.SyncConvRTTI)
	src, srcRType0, dst, dstRType, itab := r.itab(pos)
	if !dst.IsInterface() {
		return
	}

	// See reflectdata.ConvIfaceTypeWord.
	switch {
	case dst.IsEmptyInterface():
		if !src.IsInterface() {
			typeWord = srcRType0 // direct eface construction
		}
	case !src.IsInterface():
		typeWord = itab // direct iface construction
	default:
		typeWord = dstRType // convI2I
	}

	// See reflectdata.ConvIfaceSrcRType.
	if !src.IsInterface() {
		srcRType = srcRType0
	}

	return
}

func (r *reader) exprType() ir.Node {
	r.Sync(pkgbits.SyncExprType)
	pos := r.pos()

	var typ *types.Type
	var rtype, itab ir.Node

	if r.Bool() {
		// non-empty interface
		typ, rtype, _, _, itab = r.itab(pos)
		if !typ.IsInterface() {
			rtype = nil // TODO(mdempsky): Leave set?
		}
	} else {
		typ, rtype = r.rtype0(pos)

		if !r.Bool() { // not derived
			return ir.TypeNode(typ)
		}
	}

	dt := ir.NewDynamicType(pos, rtype)
	dt.ITab = itab
	dt = typed(typ, dt).(*ir.DynamicType)
	if st := dt.ToStatic(); st != nil {
		return st
	}
	return dt
}

func (r *reader) op() ir.Op {
	r.Sync(pkgbits.SyncOp)
	return ir.Op(r.Len())
}

// @@@ Package initialization

func (r *reader) pkgInit(self *types.Pkg, target *ir.Package) {
	cgoPragmas := make([][]string, r.Len())
	for i := range cgoPragmas {
		cgoPragmas[i] = r.Strings()
	}
	target.CgoPragmas = cgoPragmas

	r.pkgInitOrder(target)

	r.pkgDecls(target)

	r.Sync(pkgbits.SyncEOF)
}

// pkgInitOrder creates a synthetic init function to handle any
// package-scope initialization statements.
func (r *reader) pkgInitOrder(target *ir.Package) {
	initOrder := make([]ir.Node, r.Len())
	if len(initOrder) == 0 {
		return
	}

	// Make a function that contains all the initialization statements.
	pos := base.AutogeneratedPos
	base.Pos = pos

	fn := ir.NewFunc(pos, pos, typecheck.Lookup("init"), types.NewSignature(nil, nil, nil))
	fn.SetIsPackageInit(true)
	fn.SetInlinabilityChecked(true) // suppress useless "can inline" diagnostics

	typecheck.DeclFunc(fn)
	r.curfn = fn

	for i := range initOrder {
		lhs := make([]ir.Node, r.Len())
		for j := range lhs {
			lhs[j] = r.obj()
		}
		rhs := r.expr()
		pos := lhs[0].Pos()

		var as ir.Node
		if len(lhs) == 1 {
			as = typecheck.Stmt(ir.NewAssignStmt(pos, lhs[0], rhs))
		} else {
			as = typecheck.Stmt(ir.NewAssignListStmt(pos, ir.OAS2, lhs, []ir.Node{rhs}))
		}

		for _, v := range lhs {
			v.(*ir.Name).Defn = as
		}

		initOrder[i] = as
	}

	fn.Body = initOrder

	typecheck.FinishFuncBody()
	r.curfn = nil
	r.locals = nil

	// Outline (if legal/profitable) global map inits.
	staticinit.OutlineMapInits(fn)

	// Split large init function.
	staticinit.SplitLargeInit(fn)

	target.Inits = append(target.Inits, fn)
}

func (r *reader) pkgDecls(target *ir.Package) {
	r.Sync(pkgbits.SyncDecls)
	for {
		switch code := codeDecl(r.Code(pkgbits.SyncDecl)); code {
		default:
			panic(fmt.Sprintf("unhandled decl: %v", code))

		case declEnd:
			return

		case declFunc:
			names := r.pkgObjs(target)
			assert(len(names) == 1)
			target.Funcs = append(target.Funcs, names[0].Func)

		case declMethod:
			typ := r.typ()
			sym := r.selector()

			method := typecheck.Lookdot1(nil, sym, typ, typ.Methods(), 0)
			target.Funcs = append(target.Funcs, method.Nname.(*ir.Name).Func)

		case declVar:
			names := r.pkgObjs(target)

			if n := r.Len(); n > 0 {
				assert(len(names) == 1)
				embeds := make([]ir.Embed, n)
				for i := range embeds {
					embeds[i] = ir.Embed{Pos: r.pos(), Patterns: r.Strings()}
				}
				names[0].Embed = &embeds
				target.Embeds = append(target.Embeds, names[0])
			}

		case declOther:
			r.pkgObjs(target)
		}
	}
}

func (r *reader) pkgObjs(target *ir.Package) []*ir.Name {
	r.Sync(pkgbits.SyncDeclNames)
	nodes := make([]*ir.Name, r.Len())
	for i := range nodes {
		r.Sync(pkgbits.SyncDeclName)

		name := r.obj().(*ir.Name)
		nodes[i] = name

		sym := name.Sym()
		if sym.IsBlank() {
			continue
		}

		switch name.Class {
		default:
			base.FatalfAt(name.Pos(), "unexpected class: %v", name.Class)

		case ir.PEXTERN:
			target.Externs = append(target.Externs, name)

		case ir.PFUNC:
			assert(name.Type().Recv() == nil)

			// TODO(mdempsky): Cleaner way to recognize init?
			if strings.HasPrefix(sym.Name, "init.") {
				target.Inits = append(target.Inits, name.Func)
			}
		}

		if base.Ctxt.Flag_dynlink && types.LocalPkg.Name == "main" && types.IsExported(sym.Name) && name.Op() == ir.ONAME {
			assert(!sym.OnExportList())
			target.PluginExports = append(target.PluginExports, name)
			sym.SetOnExportList(true)
		}

		if base.Flag.AsmHdr != "" && (name.Op() == ir.OLITERAL || name.Op() == ir.OTYPE) {
			assert(!sym.Asm())
			target.AsmHdrDecls = append(target.AsmHdrDecls, name)
			sym.SetAsm(true)
		}
	}

	return nodes
}

// @@@ Inlining

// unifiedHaveInlineBody reports whether we have the function body for
// fn, so we can inline it.
func unifiedHaveInlineBody(fn *ir.Func) bool {
	if fn.Inl == nil {
		return false
	}

	_, ok := bodyReaderFor(fn)
	return ok
}

var inlgen = 0

// unifiedInlineCall implements inline.NewInline by re-reading the function
// body from its Unified IR export data.
func unifiedInlineCall(callerfn *ir.Func, call *ir.CallExpr, fn *ir.Func, inlIndex int, profile *pgoir.Profile) *ir.InlinedCallExpr {
	pri, ok := bodyReaderFor(fn)
	if !ok {
		base.FatalfAt(call.Pos(), "cannot inline call to %v: missing inline body", fn)
	}

	if !fn.Inl.HaveDcl {
		expandInline(fn, pri)
	}

	r := pri.asReader(pkgbits.SectionBody, pkgbits.SyncFuncBody)

	tmpfn := ir.NewFunc(fn.Pos(), fn.Nname.Pos(), callerfn.Sym(), fn.Type())

	r.curfn = tmpfn

	r.inlCaller = callerfn
	r.inlCall = call
	r.inlFunc = fn
	r.inlTreeIndex = inlIndex
	r.inlPosBases = make(map[*src.PosBase]*src.PosBase)
	r.funarghack = true

	r.closureVars = make([]*ir.Name, len(r.inlFunc.ClosureVars))
	for i, cv := range r.inlFunc.ClosureVars {
		// TODO(mdempsky): It should be possible to support this case, but
		// for now we rely on the inliner avoiding it.
		if cv.Outer.Curfn != callerfn {
			base.FatalfAt(call.Pos(), "inlining closure call across frames")
		}
		r.closureVars[i] = cv.Outer
	}
	if len(r.closureVars) != 0 && r.hasTypeParams() {
		r.dictParam = r.closureVars[len(r.closureVars)-1] // dictParam is last; see reader.funcLit
	}

	r.declareParams()

	var inlvars, retvars []*ir.Name
	{
		sig := r.curfn.Type()
		endParams := sig.NumRecvs() + sig.NumParams()
		endResults := endParams + sig.NumResults()

		inlvars = r.curfn.Dcl[:endParams]
		retvars = r.curfn.Dcl[endParams:endResults]
	}

	r.delayResults = fn.Inl.CanDelayResults

	r.retlabel = typecheck.AutoLabel(".i")
	inlgen++

	init := ir.TakeInit(call)

	// For normal function calls, the function callee expression
	// may contain side effects. Make sure to preserve these,
	// if necessary (#42703).
	if call.Op() == ir.OCALLFUNC {
		inline.CalleeEffects(&init, call.Fun)
	}

	var args ir.Nodes
	if call.Op() == ir.OCALLMETH {
		base.FatalfAt(call.Pos(), "OCALLMETH missed by typecheck")
	}
	args.Append(call.Args...)

	// Create assignment to declare and initialize inlvars.
	as2 := ir.NewAssignListStmt(call.Pos(), ir.OAS2, ir.ToNodes(inlvars), args)
	as2.Def = true
	var as2init ir.Nodes
	for _, name := range inlvars {
		if ir.IsBlank(name) {
			continue
		}
		// TODO(mdempsky): Use inlined position of name.Pos() instead?
		as2init.Append(ir.NewDecl(call.Pos(), ir.ODCL, name))
		name.Defn = as2
	}
	as2.SetInit(as2init)
	init.Append(typecheck.Stmt(as2))

	if !r.delayResults {
		// If not delaying retvars, declare and zero initialize the
		// result variables now.
		for _, name := range retvars {
			// TODO(mdempsky): Use inlined position of name.Pos() instead?
			init.Append(ir.NewDecl(call.Pos(), ir.ODCL, name))
			ras := ir.NewAssignStmt(call.Pos(), name, nil)
			init.Append(typecheck.Stmt(ras))
		}
	}

	// Add an inline mark just before the inlined body.
	// This mark is inline in the code so that it's a reasonable spot
	// to put a breakpoint. Not sure if that's really necessary or not
	// (in which case it could go at the end of the function instead).
	// Note issue 28603.
	init.Append(ir.NewInlineMarkStmt(call.Pos().WithIsStmt(), int64(r.inlTreeIndex)))

	ir.WithFunc(r.curfn, func() {
		if !r.syntheticBody(call.Pos()) {
			assert(r.Bool()) // have body

			r.curfn.Body = r.stmts()
			r.curfn.Endlineno = r.pos()
		}

		// TODO(mdempsky): This shouldn't be necessary. Inlining might
		// read in new function/method declarations, which could
		// potentially be recursively inlined themselves; but we shouldn't
		// need to read in the non-inlined bodies for the declarations
		// themselves. But currently it's an easy fix to #50552.
		readBodies(typecheck.Target, true, profile)

		// Replace any "return" statements within the function body.
		var edit func(ir.Node) ir.Node
		edit = func(n ir.Node) ir.Node {
			if ret, ok := n.(*ir.ReturnStmt); ok {
				n = typecheck.Stmt(r.inlReturn(ret, retvars))
			}
			ir.EditChildren(n, edit)
			return n
		}
		edit(r.curfn)
	})

	body := r.curfn.Body

	// Reparent any declarations into the caller function.
	for _, name := range r.curfn.Dcl {
		name.Curfn = callerfn

		if name.Class != ir.PAUTO {
			name.SetPos(r.inlPos(name.Pos()))
			name.SetInlFormal(true)
			name.Class = ir.PAUTO
		} else {
			name.SetInlLocal(true)
		}
	}
	callerfn.Dcl = append(callerfn.Dcl, r.curfn.Dcl...)

	body.Append(ir.NewLabelStmt(call.Pos(), r.retlabel))

	res := ir.NewInlinedCallExpr(call.Pos(), body, ir.ToNodes(retvars))
	res.SetInit(init)
	res.SetType(call.Type())
	res.SetTypecheck(1)
	res.Reshape = call.Reshape

	// Inlining shouldn't add any functions to todoBodies.
	assert(len(todoBodies) == 0)

	return res
}

// inlReturn returns a statement that can substitute for the given
// return statement when inlining.
func (r *reader) inlReturn(ret *ir.ReturnStmt, retvars []*ir.Name) *ir.BlockStmt {
	pos := r.inlCall.Pos()

	block := ir.TakeInit(ret)

	if results := ret.Results; len(results) != 0 {
		assert(len(retvars) == len(results))

		as2 := ir.NewAssignListStmt(pos, ir.OAS2, ir.ToNodes(retvars), ret.Results)

		if r.delayResults {
			for _, name := range retvars {
				// TODO(mdempsky): Use inlined position of name.Pos() instead?
				block.Append(ir.NewDecl(pos, ir.ODCL, name))
				name.Defn = as2
			}
		}

		block.Append(as2)
	}

	block.Append(ir.NewBranchStmt(pos, ir.OGOTO, r.retlabel))
	return ir.NewBlockStmt(pos, block)
}

// expandInline reads in an extra copy of IR to populate
// fn.Inl.Dcl.
func expandInline(fn *ir.Func, pri pkgReaderIndex) {
	// TODO(mdempsky): Remove this function. It's currently needed by
	// dwarfgen/dwarf.go:preInliningDcls, which requires fn.Inl.Dcl to
	// create abstract function DIEs. But we should be able to provide it
	// with the same information some other way.

	fndcls := len(fn.Dcl)
	topdcls := len(typecheck.Target.Funcs)

	tmpfn := ir.NewFunc(fn.Pos(), fn.Nname.Pos(), fn.Sym(), fn.Type())
	tmpfn.ClosureVars = fn.ClosureVars

	{
		r := pri.asReader(pkgbits.SectionBody, pkgbits.SyncFuncBody)

		// Don't change parameter's Sym/Nname fields.
		r.funarghack = true

		r.funcBody(tmpfn)
	}

	// Move tmpfn's params to fn.Inl.Dcl, and reparent under fn.
	for _, name := range tmpfn.Dcl {
		name.Curfn = fn
	}
	fn.Inl.Dcl = tmpfn.Dcl
	fn.Inl.HaveDcl = true

	// Double check that we didn't change fn.Dcl by accident.
	assert(fndcls == len(fn.Dcl))

	// typecheck.Stmts may have added function literals to
	// typecheck.Target.Decls. Remove them again so we don't risk trying
	// to compile them multiple times.
	typecheck.Target.Funcs = typecheck.Target.Funcs[:topdcls]
}

// @@@ Method wrappers
//
// Here we handle constructing "method wrappers," alternative entry
// points that adapt methods to different calling conventions. Given a
// user-declared method "func (T) M(i int) bool { ... }", there are a
// few wrappers we may need to construct:
//
//	- Implicit dereferencing. Methods declared with a value receiver T
//	  are also included in the method set of the pointer type *T, so
//	  we need to construct a wrapper like "func (recv *T) M(i int)
//	  bool { return (*recv).M(i) }".
//
//	- Promoted methods. If struct type U contains an embedded field of
//	  type T or *T, we need to construct a wrapper like "func (recv U)
//	  M(i int) bool { return recv.T.M(i) }".
//
//	- Method values. If x is an expression of type T, then "x.M" is
//	  roughly "tmp := x; func(i int) bool { return tmp.M(i) }".
//
// At call sites, we always prefer to call the user-declared method
// directly, if known, so wrappers are only needed for indirect calls
// (for example, interface method calls that can't be devirtualized).
// Consequently, we can save some compile time by skipping
// construction of wrappers that are never needed.
//
// Alternatively, because the linker doesn't care which compilation
// unit constructed a particular wrapper, we can instead construct
// them as needed. However, if a wrapper is needed in multiple
// downstream packages, we may end up needing to compile it multiple
// times, costing us more compile time and object file size. (We mark
// the wrappers as DUPOK, so the linker doesn't complain about the
// duplicate symbols.)
//
// The current heuristics we use to balance these trade offs are:
//
//	- For a (non-parameterized) defined type T, we construct wrappers
//	  for *T and any promoted methods on T (and *T) in the same
//	  compilation unit as the type declaration.
//
//	- For a parameterized defined type, we construct wrappers in the
//	  compilation units in which the type is instantiated. We
//	  similarly handle wrappers for anonymous types with methods and
//	  compilation units where their type literals appear in source.
//
//	- Method value expressions are relatively uncommon, so we
//	  construct their wrappers in the compilation units that they
//	  appear in.
//
// Finally, as an opportunistic compile-time optimization, if we know
// a wrapper was constructed in any imported package's compilation
// unit, then we skip constructing a duplicate one. However, currently
// this is only done on a best-effort basis.

// needWrapperTypes lists types for which we may need to generate
// method wrappers.
var needWrapperTypes []*types.Type

// haveWrapperTypes lists types for which we know we already have
// method wrappers, because we found the type in an imported package.
var haveWrapperTypes []*types.Type

// needMethodValueWrappers lists methods for which we may need to
// generate method value wrappers.
var needMethodValueWrappers []methodValueWrapper

// haveMethodValueWrappers lists methods for which we know we already
// have method value wrappers, because we found it in an imported
// package.
var haveMethodValueWrappers []methodValueWrapper

type methodValueWrapper struct {
	rcvr   *types.Type
	method *types.Field
}

// needWrapper records that wrapper methods may be needed at link
// time.
func (r *reader) needWrapper(typ *types.Type) {
	if typ.IsPtr() || typ.IsKind(types.TFORW) {
		return
	}

	// Special case: runtime must define error even if imported packages mention it (#29304).
	forceNeed := typ == types.ErrorType && base.Ctxt.Pkgpath == "runtime"

	// If a type was found in an imported package, then we can assume
	// that package (or one of its transitive dependencies) already
	// generated method wrappers for it.
	if r.importedDef() && !forceNeed {
		haveWrapperTypes = append(haveWrapperTypes, typ)
	} else {
		needWrapperTypes = append(needWrapperTypes, typ)
	}
}

// importedDef reports whether r is reading from an imported and
// non-generic element.
//
// If a type was found in an imported package, then we can assume that
// package (or one of its transitive dependencies) already generated
// method wrappers for it.
//
// Exception: If we're instantiating an imported generic type or
// function, we might be instantiating it with type arguments not
// previously seen before.
//
// TODO(mdempsky): Distinguish when a generic function or type was
// instantiated in an imported package so that we can add types to
// haveWrapperTypes instead.
func (r *reader) importedDef() bool {
	return r.p != localPkgReader && !r.hasTypeParams()
}

// MakeWrappers constructs all wrapper methods needed for the target
// compilation unit.
func MakeWrappers(target *ir.Package) {
	// always generate a wrapper for error.Error (#29304)
	needWrapperTypes = append(needWrapperTypes, types.ErrorType)

	seen := make(map[string]*types.Type)

	for _, typ := range haveWrapperTypes {
		wrapType(typ, target, seen, false)
	}
	haveWrapperTypes = nil

	for _, typ := range needWrapperTypes {
		wrapType(typ, target, seen, true)
	}
	needWrapperTypes = nil

	for _, wrapper := range haveMethodValueWrappers {
		wrapMethodValue(wrapper.rcvr, wrapper.method, target, false)
	}
	haveMethodValueWrappers = nil

	for _, wrapper := range needMethodValueWrappers {
		wrapMethodValue(wrapper.rcvr, wrapper.method, target, true)
	}
	needMethodValueWrappers = nil
}

func wrapType(typ *types.Type, target *ir.Package, seen map[string]*types.Type, needed bool) {
	key := typ.LinkString()
	if prev := seen[key]; prev != nil {
		if !types.Identical(typ, prev) {
			base.Fatalf("collision: types %v and %v have link string %q", typ, prev, key)
		}
		return
	}
	seen[key] = typ

	if !needed {
		// Only called to add to 'seen'.
		return
	}

	if !typ.IsInterface() {
		typecheck.CalcMethods(typ)
	}
	for _, meth := range typ.AllMethods() {
		if meth.Sym.IsBlank() || !meth.IsMethod() {
			base.FatalfAt(meth.Pos, "invalid method: %v", meth)
		}

		methodWrapper(0, typ, meth, target)

		// For non-interface types, we also want *T wrappers.
		if !typ.IsInterface() {
			methodWrapper(1, typ, meth, target)

			// For not-in-heap types, *T is a scalar, not pointer shaped,
			// so the interface wrappers use **T.
			if typ.NotInHeap() {
				methodWrapper(2, typ, meth, target)
			}
		}
	}
}

func methodWrapper(derefs int, tbase *types.Type, method *types.Field, target *ir.Package) {
	wrapper := tbase
	for i := 0; i < derefs; i++ {
		wrapper = types.NewPtr(wrapper)
	}

	sym := ir.MethodSym(wrapper, method.Sym)
	base.Assertf(!sym.Siggen(), "already generated wrapper %v", sym)
	sym.SetSiggen(true)

	wrappee := method.Type.Recv().Type
	if types.Identical(wrapper, wrappee) ||
		!types.IsMethodApplicable(wrapper, method) ||
		!reflectdata.NeedEmit(tbase) {
		return
	}

	// TODO(mdempsky): Use method.Pos instead?
	pos := base.AutogeneratedPos

	fn := newWrapperFunc(pos, sym, wrapper, method)

	var recv ir.Node = fn.Nname.Type().Recv().Nname.(*ir.Name)

	// For simple *T wrappers around T methods, panicwrap produces a
	// nicer panic message.
	if wrapper.IsPtr() && types.Identical(wrapper.Elem(), wrappee) {
		cond := ir.NewBinaryExpr(pos, ir.OEQ, recv, types.BuiltinPkg.Lookup("nil").Def.(ir.Node))
		then := []ir.Node{ir.NewCallExpr(pos, ir.OCALL, typecheck.LookupRuntime("panicwrap"), nil)}
		fn.Body.Append(ir.NewIfStmt(pos, cond, then, nil))
	}

	// typecheck will add one implicit deref, if necessary,
	// but not-in-heap types require more for their **T wrappers.
	for i := 1; i < derefs; i++ {
		recv = Implicit(ir.NewStarExpr(pos, recv))
	}

	addTailCall(pos, fn, recv, method)

	finishWrapperFunc(fn, target)
}

func wrapMethodValue(recvType *types.Type, method *types.Field, target *ir.Package, needed bool) {
	sym := ir.MethodSymSuffix(recvType, method.Sym, "-fm")
	if sym.Uniq() {
		return
	}
	sym.SetUniq(true)

	// TODO(mdempsky): Use method.Pos instead?
	pos := base.AutogeneratedPos

	fn := newWrapperFunc(pos, sym, nil, method)
	sym.Def = fn.Nname

	// Declare and initialize variable holding receiver.
	recv := ir.NewHiddenParam(pos, fn, typecheck.Lookup(".this"), recvType)

	if !needed {
		return
	}

	addTailCall(pos, fn, recv, method)

	finishWrapperFunc(fn, target)
}

func newWrapperFunc(pos src.XPos, sym *types.Sym, wrapper *types.Type, method *types.Field) *ir.Func {
	sig := newWrapperType(wrapper, method)
	fn := ir.NewFunc(pos, pos, sym, sig)
	fn.DeclareParams(true)
	fn.SetDupok(true) // TODO(mdempsky): Leave unset for local, non-generic wrappers?

	return fn
}

func finishWrapperFunc(fn *ir.Func, target *ir.Package) {
	ir.WithFunc(fn, func() {
		typecheck.Stmts(fn.Body)
	})

	// We generate wrappers after the global inlining pass,
	// so we're responsible for applying inlining ourselves here.
	// TODO(prattmic): plumb PGO.
	interleaved.DevirtualizeAndInlineFunc(fn, nil)

	// The body of wrapper function after inlining may reveal new ir.OMETHVALUE node,
	// we don't know whether wrapper function has been generated for it or not, so
	// generate one immediately here.
	//
	// Further, after CL 492017, function that construct closures is allowed to be inlined,
	// even though the closure itself can't be inline. So we also need to visit body of any
	// closure that we see when visiting body of the wrapper function.
	ir.VisitFuncAndClosures(fn, func(n ir.Node) {
		if n, ok := n.(*ir.SelectorExpr); ok && n.Op() == ir.OMETHVALUE {
			wrapMethodValue(n.X.Type(), n.Selection, target, true)
		}
	})

	fn.Nname.Defn = fn
	target.Funcs = append(target.Funcs, fn)
}

// newWrapperType returns a copy of the given signature type, but with
// the receiver parameter type substituted with recvType.
// If recvType is nil, newWrapperType returns a signature
// without a receiver parameter.
func newWrapperType(recvType *types.Type, method *types.Field) *types.Type {
	clone := func(params []*types.Field) []*types.Field {
		res := make([]*types.Field, len(params))
		for i, param := range params {
			res[i] = types.NewField(param.Pos, param.Sym, param.Type)
			res[i].SetIsDDD(param.IsDDD())
		}
		return res
	}

	sig := method.Type

	var recv *types.Field
	if recvType != nil {
		recv = types.NewField(sig.Recv().Pos, sig.Recv().Sym, recvType)
	}
	params := clone(sig.Params())
	results := clone(sig.Results())

	return types.NewSignature(recv, params, results)
}

func addTailCall(pos src.XPos, fn *ir.Func, recv ir.Node, method *types.Field) {
	sig := fn.Nname.Type()
	args := make([]ir.Node, sig.NumParams())
	for i, param := range sig.Params() {
		args[i] = param.Nname.(*ir.Name)
	}

	dot := typecheck.XDotMethod(pos, recv, method.Sym, true)
	call := typecheck.Call(pos, dot, args, method.Type.IsVariadic()).(*ir.CallExpr)

	if recv.Type() != nil && recv.Type().IsPtr() && method.Type.Recv().Type.IsPtr() &&
		method.Embedded != 0 &&
		(types.IsInterfaceMethod(method.Type) && base.Ctxt.Arch.Name != "wasm" ||
			!types.IsInterfaceMethod(method.Type) && !unifiedHaveInlineBody(ir.MethodExprName(dot).Func)) &&
		// TODO: implement wasm indirect tail calls
		// TODO: do we need the ppc64le/dynlink restriction for interface tail calls?
		!((base.Ctxt.Arch.Name == "ppc64le" || base.Ctxt.Arch.Name == "ppc64") && base.Ctxt.Flag_dynlink) {
		if base.Debug.TailCall != 0 {
			base.WarnfAt(fn.Nname.Type().Recv().Type.Elem().Pos(), "tail call emitted for the method %v wrapper", method.Nname)
		}
		// Prefer OTAILCALL to reduce code size (except the case when the called method can be inlined).
		fn.Body.Append(ir.NewTailCallStmt(pos, call))
		return
	}

	fn.SetWrapper(true)

	if method.Type.NumResults() == 0 {
		fn.Body.Append(call)
		return
	}

	ret := ir.NewReturnStmt(pos, nil)
	ret.Results = []ir.Node{call}
	fn.Body.Append(ret)
}

func setBasePos(pos src.XPos) {
	// Set the position for any error messages we might print (e.g. too large types).
	base.Pos = pos
}

// dictParamName is the name of the synthetic dictionary parameter
// added to shaped functions.
//
// N.B., this variable name is known to Delve:
// https://github.com/go-delve/delve/blob/cb91509630529e6055be845688fd21eb89ae8714/pkg/proc/eval.go#L28
const dictParamName = typecheck.LocalDictName

// shapeSig returns a copy of fn's signature, except adding a
// dictionary parameter and promoting the receiver parameter (if any)
// to a normal parameter.
//
// The parameter types.Fields are all copied too, so their Nname
// fields can be initialized for use by the shape function.
//
// All signatures returned by shapeSig are marked as shaped.
func shapeSig(fn *ir.Func, dict *readerDict) *types.Type {
	sig := fn.Nname.Type()
	oldRecv := sig.Recv()

	var recv *types.Field
	if oldRecv != nil {
		recv = types.NewField(oldRecv.Pos, oldRecv.Sym, oldRecv.Type)
	}

	params := make([]*types.Field, 1+sig.NumParams())
	params[0] = types.NewField(fn.Pos(), fn.Sym().Pkg.Lookup(dictParamName), types.NewPtr(dict.varType()))
	for i, param := range sig.Params() {
		d := types.NewField(param.Pos, param.Sym, param.Type)
		d.SetIsDDD(param.IsDDD())
		params[1+i] = d
	}

	results := make([]*types.Field, sig.NumResults())
	for i, result := range sig.Results() {
		results[i] = types.NewField(result.Pos, result.Sym, result.Type)
	}

	typ := types.NewSignature(recv, params, results)
	typ.SetHasShape(true)
	return typ
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/README.md ===
```markdown
# Making Unified Intermediate Representation (UIR) Changes

For general information on export data, see [here](../../README.md).

UIR is the serial form of the compiler's intermediate representation, used to
propagate bodies of generic and/or inlined functions from one compilation unit
to another.

The Go compiler has a single, canonical UIR writer implementation in
`src/cmd/compile/internal/noder/writer.go`. When we update the byte stream that
the UIR writer writes, *all* of the UIR readers need to be reviewed and
potentially updated; a change might not be backward compatible for them. These
instructions outline the steps required to keep all UIR readers up-to-date.

## The Writer

The UIR version written by the compiler is controlled by
`src/cmd/compile/internal/noder/unified.go`. Do not change this yet. Instead:

1. Add a version flag N+1 for `MyChange` in `internal/pkgbits/version.go`.
2. Update the UIR writer in `src/cmd/compile/internal/noder/writer.go` to guard
   the writing of any fields added in N+1. Note: readers still on version N
   *must* be oblivious to this change to avoid breaking the readers on
   submission.

## The Readers

Besides the compiler itself, there are other readers in go, x/tools, and
externally. Those in x/tools and the general public exist because
`go list -export` produces export data files in this format and we support the
ability of applications to decode it.

> Note that there is an upcoming plan to decouple the compiler's IR from x/tools
> by changing the format encoded by `go list -export`; this would make UIR a
> private detail of the compiler, free to break at any time. For now, these
> instructions must still be followed.

We assume that external readers will update on their own. The necessary reader
updates in go and x/tools are detailed below.

### go

3. Update the compiler's own UIR reader in
   `src/cmd/compile/internal/noder/reader.go` to guard the reading of any fields
   added in N+1.
4. Repeat this change for the readers in `src/go/internal/gcimporter/ureader.go`
   and `src/cmd/compile/internal/importer/ureader.go`. Note that these readers
   only read data needed for type checking (in `src/go/types` and
   `src/cmd/compile/internal/types2` respectively). For instance, they do not
   read exported function bodies. Thus, it's possible that a change to UIR (such
   as the encoding of function bodies) would require no change to these readers.

### x/tools

5. Add a version flag for `MyChange` in `internal/pkgbits/version.go`. Note:
   x/tools has its own pkgbits implementation, which is intended to be an exact
   copy of the [one in go](#the-writer). Any change made to one must be
   reflected in the other.
6. Update the x/tools UIR reader in `internal/gcimporter/ureader.go` to guard
   the reading of any fields added in N+1. Call this commit C.
7. In go, take the commit hash for C and update `src/cmd/go.mod` to use x/tools@C
   per the [vendoring instructions](https://go.dev/wiki/MinorReleases#cherry-pick-cls-for-vendored-golangorgx-packages).

## Finalizing

> If this UIR change will be tested, check the [following section](#testing) and
> consider when it makes to finalize.

Only after reviewing *all* of the readers, bump the UIR version written by the
writer to N+1 in `src/cmd/compile/internal/noder/unified.go`. Because the
readers have already been updated to handle version N+1, this change is
compatible.

## Testing

If making changes related to some new feature requiring extensive testing, it's
best to postpone bumping the UIR version until *all* of the tests are in. To
commit tests incrementally, develop them with a locally-incremented UIR version
and commit *skipped* tests; don't yet bump the remote UIR version.

Once all of the required tests are in, bump the remote UIR version while turning
on all of the previously skipped tests. This minimizes churn on the UIR version
as testing uncovers any discrepancies.

```

// === FILE: references/go/src/cmd/compile/internal/noder/types.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"cmd/compile/internal/types"
	"cmd/compile/internal/types2"
)

var basics = [...]**types.Type{
	types2.Invalid:        new(*types.Type),
	types2.Bool:           &types.Types[types.TBOOL],
	types2.Int:            &types.Types[types.TINT],
	types2.Int8:           &types.Types[types.TINT8],
	types2.Int16:          &types.Types[types.TINT16],
	types2.Int32:          &types.Types[types.TINT32],
	types2.Int64:          &types.Types[types.TINT64],
	types2.Uint:           &types.Types[types.TUINT],
	types2.Uint8:          &types.Types[types.TUINT8],
	types2.Uint16:         &types.Types[types.TUINT16],
	types2.Uint32:         &types.Types[types.TUINT32],
	types2.Uint64:         &types.Types[types.TUINT64],
	types2.Uintptr:        &types.Types[types.TUINTPTR],
	types2.Float32:        &types.Types[types.TFLOAT32],
	types2.Float64:        &types.Types[types.TFLOAT64],
	types2.Complex64:      &types.Types[types.TCOMPLEX64],
	types2.Complex128:     &types.Types[types.TCOMPLEX128],
	types2.String:         &types.Types[types.TSTRING],
	types2.UnsafePointer:  &types.Types[types.TUNSAFEPTR],
	types2.UntypedBool:    &types.UntypedBool,
	types2.UntypedInt:     &types.UntypedInt,
	types2.UntypedRune:    &types.UntypedRune,
	types2.UntypedFloat:   &types.UntypedFloat,
	types2.UntypedComplex: &types.UntypedComplex,
	types2.UntypedString:  &types.UntypedString,
	types2.UntypedNil:     &types.Types[types.TNIL],
}

var dirs = [...]types.ChanDir{
	types2.SendRecv: types.Cboth,
	types2.SendOnly: types.Csend,
	types2.RecvOnly: types.Crecv,
}

// deref2 does a single deref of types2 type t, if it is a pointer type.
func deref2(t types2.Type) types2.Type {
	if ptr := types2.AsPointer(t); ptr != nil {
		t = ptr.Elem()
	}
	return t
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/unified.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"cmp"
	"fmt"
	"internal/pkgbits"
	"internal/types/errors"
	"io"
	"runtime"
	"slices"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/inline"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/pgoir"
	"cmd/compile/internal/typecheck"
	"cmd/compile/internal/types"
	"cmd/compile/internal/types2"
	"cmd/internal/src"
)

// uirVersion is the unified IR version to use for encoding/decoding.
// Use V4 for generic methods.
const uirVersion = pkgbits.V4

// localPkgReader holds the package reader used for reading the local
// package. It exists so the unified IR linker can refer back to it
// later.
var localPkgReader *pkgReader

// LookupFunc returns the ir.Func for an arbitrary full symbol name if
// that function exists in the set of available export data.
//
// This allows lookup of arbitrary functions and methods that aren't otherwise
// referenced by the local package and thus haven't been read yet.
//
// TODO(prattmic): Does not handle instantiation of generic types. Currently
// profiles don't contain the original type arguments, so we won't be able to
// create the runtime dictionaries.
//
// TODO(prattmic): Hit rate of this function is usually fairly low, and errors
// are only used when debug logging is enabled. Consider constructing cheaper
// errors by default.
func LookupFunc(fullName string) (*ir.Func, error) {
	pkgPath, symName, err := ir.ParseLinkFuncName(fullName)
	if err != nil {
		return nil, fmt.Errorf("error parsing symbol name %q: %v", fullName, err)
	}

	pkg, ok := types.PkgMap()[pkgPath]
	if !ok {
		return nil, fmt.Errorf("pkg %s doesn't exist in %v", pkgPath, types.PkgMap())
	}

	// Symbol naming is ambiguous. We can't necessarily distinguish between
	// a method and a closure. e.g., is foo.Bar.func1 a closure defined in
	// function Bar, or a method on type Bar? Thus we must simply attempt
	// to lookup both.

	fn, err := lookupFunction(pkg, symName)
	if err == nil {
		return fn, nil
	}

	fn, mErr := lookupMethod(pkg, symName)
	if mErr == nil {
		return fn, nil
	}

	return nil, fmt.Errorf("%s is not a function (%v) or method (%v)", fullName, err, mErr)
}

// PostLookupCleanup performs cleanup operations needed
// after a series of calls to LookupFunc, specifically invoking
// readBodies to post-process any funcs on the "todoBodies" list
// that were added as a result of the lookup operations.
func PostLookupCleanup() {
	readBodies(typecheck.Target, false, nil)
}

func lookupFunction(pkg *types.Pkg, symName string) (*ir.Func, error) {
	sym := pkg.Lookup(symName)

	// TODO(prattmic): Enclosed functions (e.g., foo.Bar.func1) are not
	// present in objReader, only as OCLOSURE nodes in the enclosing
	// function.
	pri, ok := objReader[sym]
	if !ok {
		return nil, fmt.Errorf("func sym %v missing objReader", sym)
	}

	node, err := pri.pr.objIdxMayFail(pri.idx, nil, nil, false)
	if err != nil {
		return nil, fmt.Errorf("func sym %v lookup error: %w", sym, err)
	}
	name := node.(*ir.Name)
	if name.Op() != ir.ONAME || name.Class != ir.PFUNC {
		return nil, fmt.Errorf("func sym %v refers to non-function name: %v", sym, name)
	}
	return name.Func, nil
}

func lookupMethod(pkg *types.Pkg, symName string) (*ir.Func, error) {
	// N.B. readPackage creates a Sym for every object in the package to
	// initialize objReader and importBodyReader, even if the object isn't
	// read.
	//
	// However, objReader is only initialized for top-level objects, so we
	// must first lookup the type and use that to find the method rather
	// than looking for the method directly.
	typ, meth, err := ir.LookupMethodSelector(pkg, symName)
	if err != nil {
		return nil, fmt.Errorf("error looking up method symbol %q: %v", symName, err)
	}

	pri, ok := objReader[typ]
	if !ok {
		return nil, fmt.Errorf("type sym %v missing objReader", typ)
	}

	node, err := pri.pr.objIdxMayFail(pri.idx, nil, nil, false)
	if err != nil {
		return nil, fmt.Errorf("func sym %v lookup error: %w", typ, err)
	}
	name := node.(*ir.Name)
	if name.Op() != ir.OTYPE {
		return nil, fmt.Errorf("type sym %v refers to non-type name: %v", typ, name)
	}
	if name.Alias() {
		return nil, fmt.Errorf("type sym %v refers to alias", typ)
	}
	if name.Type().IsInterface() {
		return nil, fmt.Errorf("type sym %v refers to interface type", typ)
	}

	for _, m := range name.Type().Methods() {
		if m.Sym == meth {
			fn := m.Nname.(*ir.Name).Func
			return fn, nil
		}
	}

	return nil, fmt.Errorf("method %s missing from method set of %v", symName, typ)
}

// unified constructs the local package's Internal Representation (IR)
// from its syntax tree (AST).
//
// The pipeline contains 2 steps:
//
//  1. Generate the export data "stub".
//
//  2. Generate the IR from the export data above.
//
// The package data "stub" at step (1) contains everything from the local package,
// but nothing that has been imported. When we're actually writing out export data
// to the output files (see writeNewExport), we run the "linker", which:
//
//   - Updates compiler extensions data (e.g. inlining cost, escape analysis results).
//
//   - Handles re-exporting any transitive dependencies.
//
//   - Prunes out any unnecessary details (e.g. non-inlineable functions, because any
//     downstream importers only care about inlinable functions).
//
// The source files are typechecked twice: once before writing the export data
// using types2, and again after reading the export data using gc/typecheck.
// The duplication of work will go away once we only use the types2 type checker,
// removing the gc/typecheck step. For now, it is kept because:
//
//   - It reduces the engineering costs in maintaining a fork of typecheck
//     (e.g. no need to backport fixes like CL 327651).
//
//   - It makes it easier to pass toolstash -cmp.
//
//   - Historically, we would always re-run the typechecker after importing a package,
//     even though we know the imported data is valid. It's not ideal, but it's
//     not causing any problems either.
//
//   - gc/typecheck is still in charge of some transformations, such as rewriting
//     multi-valued function calls or transforming ir.OINDEX to ir.OINDEXMAP.
//
// Using the syntax tree with types2, which has a complete representation of generics,
// the unified IR has the full typed AST needed for introspection during step (1).
// In other words, we have all the necessary information to build the generic IR form
// (see writer.captureVars for an example).
func unified(m posMap, noders []*noder) {
	inline.InlineCall = unifiedInlineCall
	typecheck.HaveInlineBody = unifiedHaveInlineBody
	pgoir.LookupFunc = LookupFunc
	pgoir.PostLookupCleanup = PostLookupCleanup

	data := writePkgStub(m, noders)

	target := typecheck.Target

	localPkgReader = newPkgReader(pkgbits.NewPkgDecoder(types.LocalPkg.Path, data))
	readPackage(localPkgReader, types.LocalPkg, true)

	r := localPkgReader.newReader(pkgbits.SectionMeta, pkgbits.PrivateRootIdx, pkgbits.SyncPrivate)
	r.pkgInit(types.LocalPkg, target)

	readBodies(target, false, nil)

	// Check that nothing snuck past typechecking.
	for _, fn := range target.Funcs {
		if fn.Typecheck() == 0 {
			base.FatalfAt(fn.Pos(), "missed typecheck: %v", fn)
		}

		// For functions, check that at least their first statement (if
		// any) was typechecked too.
		if len(fn.Body) != 0 {
			if stmt := fn.Body[0]; stmt.Typecheck() == 0 {
				base.FatalfAt(stmt.Pos(), "missed typecheck: %v", stmt)
			}
		}
	}

	// For functions originally came from package runtime,
	// mark as norace to prevent instrumenting, see issue #60439.
	for _, fn := range target.Funcs {
		if !base.Flag.CompilingRuntime && types.RuntimeSymName(fn.Sym()) != "" {
			fn.Pragma |= ir.Norace
		}
	}

	base.ExitIfErrors() // just in case
}

// readBodies iteratively expands all pending dictionaries and
// function bodies.
//
// If duringInlining is true, then the inline.InlineDecls is called as
// necessary on instantiations of imported generic functions, so their
// inlining costs can be computed.
func readBodies(target *ir.Package, duringInlining bool, profile *pgoir.Profile) {
	var inlDecls []*ir.Func

	// Don't use range--bodyIdx can add closures to todoBodies.
	for {
		// The order we expand dictionaries and bodies doesn't matter, so
		// pop from the end to reduce todoBodies reallocations if it grows
		// further.
		//
		// However, we do at least need to flush any pending dictionaries
		// before reading bodies, because bodies might reference the
		// dictionaries.

		if len(todoDicts) > 0 {
			fn := todoDicts[len(todoDicts)-1]
			todoDicts = todoDicts[:len(todoDicts)-1]
			fn()
			continue
		}

		if len(todoBodies) > 0 {
			fn := todoBodies[len(todoBodies)-1]
			todoBodies = todoBodies[:len(todoBodies)-1]

			pri, ok := bodyReader[fn]
			assert(ok)
			pri.funcBody(fn)

			// Instantiated generic function: add to Decls for typechecking
			// and compilation.
			if fn.OClosure == nil && len(pri.dict.targs) != 0 {
				// cmd/link does not support a type symbol referencing a method symbol
				// across DSO boundary, so force re-compiling methods on a generic type
				// even it was seen from imported package in linkshared mode, see #58966.
				canSkipNonGenericMethod := !(base.Ctxt.Flag_linkshared && ir.IsMethod(fn))
				if duringInlining && canSkipNonGenericMethod {
					inlDecls = append(inlDecls, fn)
				} else {
					target.Funcs = append(target.Funcs, fn)
				}
			}

			continue
		}

		break
	}

	todoDicts = nil
	todoBodies = nil

	if len(inlDecls) != 0 {
		// If we instantiated any generic functions during inlining, we need
		// to call CanInline on them so they'll be transitively inlined
		// correctly (#56280).
		//
		// We know these functions were already compiled in an imported
		// package though, so we don't need to actually apply InlineCalls or
		// save the function bodies any further than this.
		//
		// We can also lower the -m flag to 0, to suppress duplicate "can
		// inline" diagnostics reported against the imported package. Again,
		// we already reported those diagnostics in the original package, so
		// it's pointless repeating them here.

		oldLowerM := base.Flag.LowerM
		base.Flag.LowerM = 0
		inline.CanInlineFuncs(inlDecls, profile)
		base.Flag.LowerM = oldLowerM

		for _, fn := range inlDecls {
			fn.Body = nil // free memory
		}
	}
}

// writePkgStub type checks the given parsed source files,
// writes an export data package stub representing them,
// and returns the result.
func writePkgStub(m posMap, noders []*noder) string {
	pkg, info, otherInfo := checkFiles(m, noders)

	pw := newPkgWriter(m, pkg, info, otherInfo)

	pw.collectDecls(noders)

	publicRootWriter := pw.newWriter(pkgbits.SectionMeta, pkgbits.SyncPublic)
	privateRootWriter := pw.newWriter(pkgbits.SectionMeta, pkgbits.SyncPrivate)

	assert(publicRootWriter.Idx == pkgbits.PublicRootIdx)
	assert(privateRootWriter.Idx == pkgbits.PrivateRootIdx)

	{
		w := publicRootWriter
		w.pkg(pkg)

		if w.Version().Has(pkgbits.HasInit) {
			w.Bool(false)
		}

		scope := pkg.Scope()
		names := scope.Names()
		w.Len(len(names))
		for _, name := range names {
			w.obj(scope.Lookup(name), nil)
		}

		w.Sync(pkgbits.SyncEOF)
		w.Flush()
	}

	{
		w := privateRootWriter
		w.pkgInit(noders)
		w.Flush()
	}

	var sb strings.Builder
	pw.DumpTo(&sb)

	// At this point, we're done with types2. Make sure the package is
	// garbage collected.
	freePackage(pkg)

	return sb.String()
}

// freePackage ensures the given package is garbage collected.
func freePackage(pkg *types2.Package) {
	// The GC test below relies on a precise GC that runs finalizers as
	// soon as objects are unreachable. Our implementation provides
	// this, but other/older implementations may not (e.g., Go 1.4 does
	// not because of #22350). To avoid imposing unnecessary
	// restrictions on the GOROOT_BOOTSTRAP toolchain, we skip the test
	// during bootstrapping.
	if base.CompilerBootstrap || base.Debug.GCCheck == 0 {
		*pkg = types2.Package{}
		return
	}

	// Set a finalizer on pkg so we can detect if/when it's collected.
	done := make(chan struct{})
	runtime.SetFinalizer(pkg, func(*types2.Package) { close(done) })

	// Important: objects involved in cycles are not finalized, so zero
	// out pkg to break its cycles and allow the finalizer to run.
	*pkg = types2.Package{}

	// It typically takes just 1 or 2 cycles to release pkg, but it
	// doesn't hurt to try a few more times.
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			return
		default:
			runtime.GC()
		}
	}

	base.Fatalf("package never finalized")
}

// readPackage reads package export data from pr to populate
// importpkg.
//
// localStub indicates whether pr is reading the stub export data for
// the local package, as opposed to relocated export data for an
// import.
func readPackage(pr *pkgReader, importpkg *types.Pkg, localStub bool) {
	{
		r := pr.newReader(pkgbits.SectionMeta, pkgbits.PublicRootIdx, pkgbits.SyncPublic)

		pkg := r.pkg()
		// This error can happen if "go tool compile" is called with wrong "-p" flag, see issue #54542.
		if pkg != importpkg {
			base.ErrorfAt(base.AutogeneratedPos, errors.BadImportPath, "mismatched import path, have %q (%p), want %q (%p)", pkg.Path, pkg, importpkg.Path, importpkg)
			base.ErrorExit()
		}

		if r.Version().Has(pkgbits.HasInit) {
			r.Bool()
		}

		for i, n := 0, r.Len(); i < n; i++ {
			r.Sync(pkgbits.SyncObject)
			if r.Version().Has(pkgbits.DerivedFuncInstance) {
				assert(!r.Bool())
			}
			idx := r.Reloc(pkgbits.SectionObj)
			assert(r.Len() == 0)

			path, name, code := r.p.PeekObj(idx)
			if code != pkgbits.ObjStub {
				objReader[types.NewPkg(path, "").Lookup(name)] = pkgReaderIndex{pr, idx, nil, nil, nil}
			}
		}

		r.Sync(pkgbits.SyncEOF)
	}

	if !localStub {
		r := pr.newReader(pkgbits.SectionMeta, pkgbits.PrivateRootIdx, pkgbits.SyncPrivate)

		if r.Bool() {
			sym := importpkg.Lookup(".inittask")
			task := ir.NewNameAt(src.NoXPos, sym, nil)
			task.Class = ir.PEXTERN
			sym.Def = task
		}

		for i, n := 0, r.Len(); i < n; i++ {
			path := r.String()
			name := r.String()
			idx := r.Reloc(pkgbits.SectionBody)

			sym := types.NewPkg(path, "").Lookup(name)
			if _, ok := importBodyReader[sym]; !ok {
				importBodyReader[sym] = pkgReaderIndex{pr, idx, nil, nil, nil}
			}
		}

		r.Sync(pkgbits.SyncEOF)
	}
}

// writeUnifiedExport writes to `out` the finalized, self-contained
// Unified IR export data file for the current compilation unit.
func writeUnifiedExport(out io.Writer) {
	l := linker{
		pw: pkgbits.NewPkgEncoder(uirVersion, base.Debug.SyncFrames),

		pkgs:   make(map[string]index),
		decls:  make(map[*types.Sym]index),
		bodies: make(map[*types.Sym]index),
	}

	publicRootWriter := l.pw.NewEncoder(pkgbits.SectionMeta, pkgbits.SyncPublic)
	privateRootWriter := l.pw.NewEncoder(pkgbits.SectionMeta, pkgbits.SyncPrivate)
	assert(publicRootWriter.Idx == pkgbits.PublicRootIdx)
	assert(privateRootWriter.Idx == pkgbits.PrivateRootIdx)

	var selfPkgIdx index

	{
		pr := localPkgReader
		r := pr.NewDecoder(pkgbits.SectionMeta, pkgbits.PublicRootIdx, pkgbits.SyncPublic)

		r.Sync(pkgbits.SyncPkg)
		selfPkgIdx = l.relocIdx(pr, pkgbits.SectionPkg, r.Reloc(pkgbits.SectionPkg))

		// Versions must match.
		// TODO: It seems that we should be able to use r.Version() for NewPkgEncoder
		// instead of passing uirVersion, but NewPkgEncoder is created before r.
		// If that is correct, we should make that happen.
		assert(r.Version() == uirVersion)

		if r.Version().Has(pkgbits.HasInit) {
			r.Bool()
		}

		for i, n := 0, r.Len(); i < n; i++ {
			r.Sync(pkgbits.SyncObject)
			if r.Version().Has(pkgbits.DerivedFuncInstance) {
				assert(!r.Bool())
			}
			idx := r.Reloc(pkgbits.SectionObj)
			assert(r.Len() == 0)

			xpath, xname, xtag := pr.PeekObj(idx)
			assert(xpath == pr.PkgPath())
			assert(xtag != pkgbits.ObjStub)

			if types.IsExported(xname) {
				l.relocIdx(pr, pkgbits.SectionObj, idx)
			}
		}

		r.Sync(pkgbits.SyncEOF)
	}

	{
		var idxs []index
		for _, idx := range l.decls {
			idxs = append(idxs, idx)
		}
		slices.Sort(idxs)

		w := publicRootWriter

		w.Sync(pkgbits.SyncPkg)
		w.Reloc(pkgbits.SectionPkg, selfPkgIdx)

		if w.Version().Has(pkgbits.HasInit) {
			w.Bool(false)
		}

		w.Len(len(idxs))
		for _, idx := range idxs {
			w.Sync(pkgbits.SyncObject)
			if w.Version().Has(pkgbits.DerivedFuncInstance) {
				w.Bool(false)
			}
			w.Reloc(pkgbits.SectionObj, idx)
			w.Len(0)
		}

		w.Sync(pkgbits.SyncEOF)
		w.Flush()
	}

	{
		type symIdx struct {
			sym *types.Sym
			idx index
		}
		var bodies []symIdx
		for sym, idx := range l.bodies {
			bodies = append(bodies, symIdx{sym, idx})
		}
		slices.SortFunc(bodies, func(a, b symIdx) int { return cmp.Compare(a.idx, b.idx) })

		w := privateRootWriter

		w.Bool(typecheck.Lookup(".inittask").Def != nil)

		w.Len(len(bodies))
		for _, body := range bodies {
			w.String(body.sym.Pkg.Path)
			w.String(body.sym.Name)
			w.Reloc(pkgbits.SectionBody, body.idx)
		}

		w.Sync(pkgbits.SyncEOF)
		w.Flush()
	}

	base.Ctxt.Fingerprint = l.pw.DumpTo(out)
}

```

// === FILE: references/go/src/cmd/compile/internal/noder/writer.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package noder

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/version"
	"internal/buildcfg"
	"internal/pkgbits"
	"os"
	"slices"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types"
	"cmd/compile/internal/types2"
)

// This file implements the Unified IR package writer and defines the
// Unified IR export data format.
//
// Low-level coding details (e.g., byte-encoding of individual
// primitive values, or handling element bitstreams and
// cross-references) are handled by internal/pkgbits, so here we only
// concern ourselves with higher-level worries like mapping Go
// language constructs into elements.

// There are two central types in the writing process: the "writer"
// type handles writing out individual elements, while the "pkgWriter"
// type keeps track of which elements have already been created.
//
// For each sort of "thing" (e.g., position, package, object, type)
// that can be written into the export data, there are generally
// several methods that work together:
//
// - writer.thing handles writing out a *use* of a thing, which often
//   means writing a relocation to that thing's encoded index.
//
// - pkgWriter.thingIdx handles reserving an index for a thing, and
//   writing out any elements needed for the thing.
//
// - writer.doThing handles writing out the *definition* of a thing,
//   which in general is a mix of low-level coding primitives (e.g.,
//   ints and strings) or uses of other things.
//
// A design goal of Unified IR is to have a single, canonical writer
// implementation, but multiple reader implementations each tailored
// to their respective needs. For example, within cmd/compile's own
// backend, inlining is implemented largely by just re-running the
// function body reading code.

// TODO(mdempsky): Add an importer for Unified IR to the x/tools repo,
// and better document the file format boundary between public and
// private data.

type index = pkgbits.Index

func assert(p bool) { base.Assert(p) }

// A pkgWriter constructs Unified IR export data from the results of
// running the types2 type checker on a Go compilation unit.
type pkgWriter struct {
	pkgbits.PkgEncoder

	m                     posMap
	curpkg                *types2.Package
	info                  *types2.Info
	rangeFuncBodyClosures map[*syntax.FuncLit]bool // non-public information, e.g., which functions are closures range function bodies?

	// Indices for previously written syntax and types2 things.

	posBasesIdx map[*syntax.PosBase]index
	pkgsIdx     map[*types2.Package]index
	typsIdx     map[types2.Type]index
	objsIdx     map[types2.Object]index

	// Maps from types2.Objects back to their syntax.Decl.

	funDecls map[*types2.Func]*syntax.FuncDecl
	typDecls map[*types2.TypeName]typeDeclGen

	// linknames maps package-scope objects to their linker symbol name,
	// if specified by a //go:linkname or //go:linknamestd directive.
	linknames map[types2.Object]struct {
		remote string
		std    bool
	}

	// cgoPragmas accumulates any //go:cgo_* pragmas that need to be
	// passed through to cmd/link.
	cgoPragmas [][]string
}

// newPkgWriter returns an initialized pkgWriter for the specified
// package.
func newPkgWriter(m posMap, pkg *types2.Package, info *types2.Info, otherInfo map[*syntax.FuncLit]bool) *pkgWriter {
	return &pkgWriter{
		PkgEncoder: pkgbits.NewPkgEncoder(uirVersion, base.Debug.SyncFrames),

		m:                     m,
		curpkg:                pkg,
		info:                  info,
		rangeFuncBodyClosures: otherInfo,

		pkgsIdx: make(map[*types2.Package]index),
		objsIdx: make(map[types2.Object]index),
		typsIdx: make(map[types2.Type]index),

		posBasesIdx: make(map[*syntax.PosBase]index),

		funDecls: make(map[*types2.Func]*syntax.FuncDecl),
		typDecls: make(map[*types2.TypeName]typeDeclGen),

		linknames: make(map[types2.Object]struct {
			remote string
			std    bool
		}),
	}
}

// errorf reports a user error about thing p.
func (pw *pkgWriter) errorf(p poser, msg string, args ...any) {
	base.ErrorfAt(pw.m.pos(p), 0, msg, args...)
}

// fatalf reports an internal compiler error about thing p.
func (pw *pkgWriter) fatalf(p poser, msg string, args ...any) {
	base.FatalfAt(pw.m.pos(p), msg, args...)
}

// unexpected reports a fatal error about a thing of unexpected
// dynamic type.
func (pw *pkgWriter) unexpected(what string, p poser) {
	pw.fatalf(p, "unexpected %s: %v (%T)", what, p, p)
}

func (pw *pkgWriter) typeAndValue(x syntax.Expr) syntax.TypeAndValue {
	tv, ok := pw.maybeTypeAndValue(x)
	if !ok {
		pw.fatalf(x, "missing Types entry: %v", syntax.String(x))
	}
	return tv
}

func (pw *pkgWriter) maybeTypeAndValue(x syntax.Expr) (syntax.TypeAndValue, bool) {
	tv := x.GetTypeInfo()

	// If x is a generic function whose type arguments are inferred
	// from assignment context, then we need to find its inferred type
	// in Info.Instances instead.
	if name, ok := x.(*syntax.Name); ok {
		if inst, ok := pw.info.Instances[name]; ok {
			tv.Type = inst.Type
		}
	}

	return tv, tv.Type != nil
}

// typeOf returns the Type of the given value expression.
func (pw *pkgWriter) typeOf(expr syntax.Expr) types2.Type {
	tv := pw.typeAndValue(expr)
	if !tv.IsValue() {
		pw.fatalf(expr, "expected value: %v", syntax.String(expr))
	}
	return tv.Type
}

// A writer provides APIs for writing out an individual element.
type writer struct {
	p *pkgWriter

	*pkgbits.Encoder

	// sig holds the signature for the current function body, if any.
	sig *types2.Signature

	// TODO(mdempsky): We should be able to prune localsIdx whenever a
	// scope closes, and then maybe we can just use the same map for
	// storing the TypeParams too (as their TypeName instead).

	// localsIdx tracks any local variables declared within this
	// function body. It's unused for writing out non-body things.
	localsIdx map[*types2.Var]int

	// closureVars tracks any free variables that are referenced by this
	// function body. It's unused for writing out non-body things.
	closureVars    []posVar
	closureVarsIdx map[*types2.Var]int // index of previously seen free variables

	dict *writerDict

	// derived tracks whether the type being written out references any
	// type parameters. It's unused for writing non-type things.
	derived bool
}

// A writerDict tracks types and objects that are used by a declaration.
type writerDict struct {
	// implicits contains type parameters from enclosing declarations.
	implicits []*types2.TypeParam
	// receivers contains receiver type parameters of the declaration.
	receivers []*types2.TypeParam

	// derived is a slice of type indices for computing derived types
	// (i.e., types that depend on the declaration's type parameters).
	derived []derivedInfo

	// derivedIdx maps a Type to its corresponding index within the
	// derived slice, if present.
	derivedIdx map[types2.Type]index

	// These slices correspond to entries in the runtime dictionary.
	typeParamMethodExprs []writerMethodExprInfo
	subdicts             []objInfo
	rtypes               []typeInfo
	itabs                []itabInfo
}

type itabInfo struct {
	typ   typeInfo
	iface typeInfo
}

// typeParamIndex returns the index of the given type parameter within
// the dictionary. This may differ from typ.Index() when there are
// implicit or receiver type parameters.
func (dict *writerDict) typeParamIndex(typ *types2.TypeParam) int {
	for idx, implicit := range dict.implicits {
		if implicit == typ {
			return idx
		}
	}

	for idx, receiver := range dict.receivers {
		if receiver == typ {
			return len(dict.implicits) + idx
		}
	}

	return len(dict.implicits) + len(dict.receivers) + typ.Index()
}

// A derivedInfo represents a reference to an encoded generic Go type.
type derivedInfo struct {
	idx index
}

// A typeInfo represents a reference to an encoded Go type.
//
// If derived is true, then the typeInfo represents a generic Go type
// that contains type parameters. In this case, idx is an index into
// the readerDict.derived{,Types} arrays.
//
// Otherwise, the typeInfo represents a non-generic Go type, and idx
// is an index into the reader.typs array instead.
type typeInfo struct {
	idx     index
	derived bool
}

// An objInfo represents a reference to an encoded, instantiated (if
// applicable) Go object.
type objInfo struct {
	idx       index      // index for the generic function declaration
	explicits []typeInfo // info for the type arguments
}

// A selectorInfo represents a reference to an encoded field or method
// name (i.e., objects that can only be accessed using selector
// expressions).
type selectorInfo struct {
	pkgIdx  index
	nameIdx index
}

// anyDerived reports whether any of info's explicit type arguments
// are derived types.
func (info objInfo) anyDerived() bool {
	for _, explicit := range info.explicits {
		if explicit.derived {
			return true
		}
	}
	return false
}

// equals reports whether info and other represent the same Go object
// (i.e., same base object and identical type arguments, if any).
func (info objInfo) equals(other objInfo) bool {
	if info.idx != other.idx {
		return false
	}
	assert(len(info.explicits) == len(other.explicits))
	for i, targ := range info.explicits {
		if targ != other.explicits[i] {
			return false
		}
	}
	return true
}

type writerMethodExprInfo struct {
	typeParamIdx int
	methodInfo   selectorInfo
}

// typeParamMethodExprIdx returns the index where the given encoded
// method expression function pointer appears within this dictionary's
// type parameters method expressions section, adding it if necessary.
func (dict *writerDict) typeParamMethodExprIdx(typeParamIdx int, methodInfo selectorInfo) int {
	newInfo := writerMethodExprInfo{typeParamIdx, methodInfo}

	for idx, oldInfo := range dict.typeParamMethodExprs {
		if oldInfo == newInfo {
			return idx
		}
	}

	idx := len(dict.typeParamMethodExprs)
	dict.typeParamMethodExprs = append(dict.typeParamMethodExprs, newInfo)
	return idx
}

// subdictIdx returns the index where the given encoded object's
// runtime dictionary appears within this dictionary's subdictionary
// section, adding it if necessary.
func (dict *writerDict) subdictIdx(newInfo objInfo) int {
	for idx, oldInfo := range dict.subdicts {
		if oldInfo.equals(newInfo) {
			return idx
		}
	}

	idx := len(dict.subdicts)
	dict.subdicts = append(dict.subdicts, newInfo)
	return idx
}

// rtypeIdx returns the index where the given encoded type's
// *runtime._type value appears within this dictionary's rtypes
// section, adding it if necessary.
func (dict *writerDict) rtypeIdx(newInfo typeInfo) int {
	for idx, oldInfo := range dict.rtypes {
		if oldInfo == newInfo {
			return idx
		}
	}

	idx := len(dict.rtypes)
	dict.rtypes = append(dict.rtypes, newInfo)
	return idx
}

// itabIdx returns the index where the given encoded type pair's
// *runtime.itab value appears within this dictionary's itabs section,
// adding it if necessary.
func (dict *writerDict) itabIdx(typInfo, ifaceInfo typeInfo) int {
	newInfo := itabInfo{typInfo, ifaceInfo}

	for idx, oldInfo := range dict.itabs {
		if oldInfo == newInfo {
			return idx
		}
	}

	idx := len(dict.itabs)
	dict.itabs = append(dict.itabs, newInfo)
	return idx
}

func (pw *pkgWriter) newWriter(k pkgbits.SectionKind, marker pkgbits.SyncMarker) *writer {
	return &writer{
		Encoder: pw.NewEncoder(k, marker),
		p:       pw,
	}
}

// @@@ Positions

// pos writes the position of p into the element bitstream.
func (w *writer) pos(p poser) {
	w.Sync(pkgbits.SyncPos)
	pos := p.Pos()

	// TODO(mdempsky): Track down the remaining cases here and fix them.
	if !w.Bool(pos.IsKnown()) {
		return
	}

	// TODO(mdempsky): Delta encoding.
	w.posBase(pos.Base())
	w.Uint(pos.Line())
	w.Uint(pos.Col())
}

// posBase writes a reference to the given PosBase into the element
// bitstream.
func (w *writer) posBase(b *syntax.PosBase) {
	w.Reloc(pkgbits.SectionPosBase, w.p.posBaseIdx(b))
}

// posBaseIdx returns the index for the given PosBase.
func (pw *pkgWriter) posBaseIdx(b *syntax.PosBase) index {
	if idx, ok := pw.posBasesIdx[b]; ok {
		return idx
	}

	w := pw.newWriter(pkgbits.SectionPosBase, pkgbits.SyncPosBase)
	w.p.posBasesIdx[b] = w.Idx

	w.String(trimFilename(b))

	if !w.Bool(b.IsFileBase()) {
		w.pos(b)
		w.Uint(b.Line())
		w.Uint(b.Col())
	}

	return w.Flush()
}

// @@@ Packages

// pkg writes a use of the given Package into the element bitstream.
func (w *writer) pkg(pkg *types2.Package) {
	w.pkgRef(w.p.pkgIdx(pkg))
}

func (w *writer) pkgRef(idx index) {
	w.Sync(pkgbits.SyncPkg)
	w.Reloc(pkgbits.SectionPkg, idx)
}

// pkgIdx returns the index for the given package, adding it to the
// package export data if needed.
func (pw *pkgWriter) pkgIdx(pkg *types2.Package) index {
	if idx, ok := pw.pkgsIdx[pkg]; ok {
		return idx
	}

	w := pw.newWriter(pkgbits.SectionPkg, pkgbits.SyncPkgDef)
	pw.pkgsIdx[pkg] = w.Idx

	// The universe and package unsafe need to be handled specially by
	// importers anyway, so we serialize them using just their package
	// path. This ensures that readers don't confuse them for
	// user-defined packages.
	switch pkg {
	case nil: // universe
		w.String("builtin") // same package path used by godoc
	case types2.Unsafe:
		w.String("unsafe")
	default:
		// TODO(mdempsky): Write out pkg.Path() for curpkg too.
		var path string
		if pkg != w.p.curpkg {
			path = pkg.Path()
		}
		base.Assertf(path != "builtin" && path != "unsafe", "unexpected path for user-defined package: %q", path)
		w.String(path)
		w.String(pkg.Name())

		w.Len(len(pkg.Imports()))
		for _, imp := range pkg.Imports() {
			w.pkg(imp)
		}
	}

	return w.Flush()
}

// @@@ Types

var (
	anyTypeName        = types2.Universe.Lookup("any").(*types2.TypeName)
	comparableTypeName = types2.Universe.Lookup("comparable").(*types2.TypeName)
	runeTypeName       = types2.Universe.Lookup("rune").(*types2.TypeName)
)

// typ writes a use of the given type into the bitstream.
func (w *writer) typ(typ types2.Type) {
	w.typInfo(w.p.typIdx(typ, w.dict))
}

// typInfo writes a use of the given type (specified as a typeInfo
// instead) into the bitstream.
func (w *writer) typInfo(info typeInfo) {
	w.Sync(pkgbits.SyncType)
	if w.Bool(info.derived) {
		w.Len(int(info.idx))
		w.derived = true
	} else {
		w.Reloc(pkgbits.SectionType, info.idx)
	}
}

// typIdx returns the index where the export data description of type
// can be read back in. If no such index exists yet, it's created.
//
// typIdx also reports whether typ is a derived type; that is, whether
// its identity depends on type parameters.
func (pw *pkgWriter) typIdx(typ types2.Type, dict *writerDict) typeInfo {
	// Strip non-global aliases, because they only appear in inline
	// bodies anyway. Otherwise, they can cause types.Sym collisions
	// (e.g., "main.C" for both of the local type aliases in
	// test/fixedbugs/issue50190.go).
	for {
		if alias, ok := typ.(*types2.Alias); ok && !isGlobal(alias.Obj()) {
			typ = alias.Rhs()
		} else {
			break
		}
	}

	if idx, ok := pw.typsIdx[typ]; ok {
		return typeInfo{idx: idx, derived: false}
	}
	if dict != nil {
		if idx, ok := dict.derivedIdx[typ]; ok {
			return typeInfo{idx: idx, derived: true}
		}
	}

	w := pw.newWriter(pkgbits.SectionType, pkgbits.SyncTypeIdx)
	w.dict = dict

	switch typ := typ.(type) {
	default:
		base.Fatalf("unexpected type: %v (%T)", typ, typ)

	case *types2.Basic:
		switch kind := typ.Kind(); {
		case kind == types2.Invalid:
			base.Fatalf("unexpected types2.Invalid")

		case types2.Typ[kind] == typ:
			w.Code(pkgbits.TypeBasic)
			w.Len(int(kind))

		default:
			// Handle "byte" and "rune" as references to their TypeNames.
			obj := types2.Universe.Lookup(typ.Name()).(*types2.TypeName)
			assert(obj.Type() == typ)

			w.Code(pkgbits.TypeNamed)
			w.namedType(obj, nil)
		}

	case *types2.Named:
		w.Code(pkgbits.TypeNamed)
		w.namedType(splitNamed(typ))

	case *types2.Alias:
		w.Code(pkgbits.TypeNamed)
		w.namedType(splitAlias(typ))

	case *types2.TypeParam:
		w.derived = true
		w.Code(pkgbits.TypeTypeParam)
		w.Len(w.dict.typeParamIndex(typ))

	case *types2.Array:
		w.Code(pkgbits.TypeArray)
		w.Uint64(uint64(typ.Len()))
		w.typ(typ.Elem())

	case *types2.Chan:
		w.Code(pkgbits.TypeChan)
		w.Len(int(typ.Dir()))
		w.typ(typ.Elem())

	case *types2.Map:
		w.Code(pkgbits.TypeMap)
		w.typ(typ.Key())
		w.typ(typ.Elem())

	case *types2.Pointer:
		w.Code(pkgbits.TypePointer)
		w.typ(typ.Elem())

	case *types2.Signature:
		base.Assertf(typ.TypeParams() == nil, "unexpected type params: %v", typ)
		w.Code(pkgbits.TypeSignature)
		w.signature(typ)

	case *types2.Slice:
		w.Code(pkgbits.TypeSlice)
		w.typ(typ.Elem())

	case *types2.Struct:
		w.Code(pkgbits.TypeStruct)
		w.structType(typ)

	case *types2.Interface:
		// Handle "any" as reference to its TypeName.
		// The underlying "any" interface is canonical, so this logic handles both
		// GODEBUG=gotypesalias=1 (when any is represented as a types2.Alias), and
		// gotypesalias=0.
		if types2.Unalias(typ) == types2.Unalias(anyTypeName.Type()) {
			w.Code(pkgbits.TypeNamed)
			w.obj(anyTypeName, nil)
			break
		}

		w.Code(pkgbits.TypeInterface)
		w.interfaceType(typ)

	case *types2.Union:
		w.Code(pkgbits.TypeUnion)
		w.unionType(typ)
	}

	if w.derived {
		idx := index(len(dict.derived))
		dict.derived = append(dict.derived, derivedInfo{idx: w.Flush()})
		dict.derivedIdx[typ] = idx
		return typeInfo{idx: idx, derived: true}
	}

	pw.typsIdx[typ] = w.Idx
	return typeInfo{idx: w.Flush(), derived: false}
}

// namedType writes a use of the given named type into the bitstream.
func (w *writer) namedType(obj *types2.TypeName, targs []types2.Type) {
	// Named types that are declared within a generic function (and
	// thus have implicit type parameters) are always derived types.
	if w.p.hasImplicitTypeParams(obj) {
		w.derived = true
	}

	w.obj(obj, targs)
}

func (w *writer) structType(typ *types2.Struct) {
	w.Len(typ.NumFields())
	for i := 0; i < typ.NumFields(); i++ {
		f := typ.Field(i)
		w.pos(f)
		w.selector(f)
		w.typ(f.Type())
		w.String(typ.Tag(i))
		w.Bool(f.Embedded())
	}
}

func (w *writer) unionType(typ *types2.Union) {
	w.Len(typ.Len())
	for i := 0; i < typ.Len(); i++ {
		t := typ.Term(i)
		w.Bool(t.Tilde())
		w.typ(t.Type())
	}
}

func (w *writer) interfaceType(typ *types2.Interface) {
	// If typ has no embedded types but it's not a basic interface, then
	// the natural description we write out below will fail to
	// reconstruct it.
	if typ.NumEmbeddeds() == 0 && !typ.IsMethodSet() {
		// Currently, this can only happen for the underlying Interface of
		// "comparable", which is needed to handle type declarations like
		// "type C comparable".
		assert(typ == comparableTypeName.Type().(*types2.Named).Underlying())

		// Export as "interface{ comparable }".
		w.Len(0)                         // NumExplicitMethods
		w.Len(1)                         // NumEmbeddeds
		w.Bool(false)                    // IsImplicit
		w.typ(comparableTypeName.Type()) // EmbeddedType(0)
		return
	}

	w.Len(typ.NumExplicitMethods())
	w.Len(typ.NumEmbeddeds())

	if typ.NumExplicitMethods() == 0 && typ.NumEmbeddeds() == 1 {
		w.Bool(typ.IsImplicit())
	} else {
		// Implicit interfaces always have 0 explicit methods and 1
		// embedded type, so we skip writing out the implicit flag
		// otherwise as a space optimization.
		assert(!typ.IsImplicit())
	}

	for i := 0; i < typ.NumExplicitMethods(); i++ {
		m := typ.ExplicitMethod(i)
		sig := m.Type().(*types2.Signature)
		assert(sig.TypeParams() == nil)

		w.pos(m)
		w.selector(m)
		w.signature(sig)
	}

	for i := 0; i < typ.NumEmbeddeds(); i++ {
		w.typ(typ.EmbeddedType(i))
	}
}

func (w *writer) signature(sig *types2.Signature) {
	w.Sync(pkgbits.SyncSignature)
	w.params(sig.Params())
	w.params(sig.Results())
	w.Bool(sig.Variadic())
}

func (w *writer) params(typ *types2.Tuple) {
	w.Sync(pkgbits.SyncParams)
	w.Len(typ.Len())
	for i := 0; i < typ.Len(); i++ {
		w.param(typ.At(i))
	}
}

func (w *writer) param(param *types2.Var) {
	w.Sync(pkgbits.SyncParam)
	w.pos(param)
	w.localIdent(param)
	w.typ(param.Type())
}

// @@@ Objects

// obj writes a use of the given object into the bitstream.
//
// If obj is a generic object, then explicits are the explicit type
// arguments used to instantiate it (i.e., used to substitute the
// object's own declared type parameters).
func (w *writer) obj(obj types2.Object, explicits []types2.Type) {
	w.objInfo(w.p.objInstIdx(obj, explicits, w.dict))
}

// objInfo writes a use of the given encoded object into the
// bitstream.
func (w *writer) objInfo(info objInfo) {
	w.Sync(pkgbits.SyncObject)
	if w.Version().Has(pkgbits.DerivedFuncInstance) {
		w.Bool(false)
	}
	w.Reloc(pkgbits.SectionObj, info.idx)

	w.Len(len(info.explicits))
	for _, info := range info.explicits {
		w.typInfo(info)
	}
}

// objInstIdx returns the indices for an object and a corresponding
// list of type arguments used to instantiate it, adding them to the
// export data as needed.
func (pw *pkgWriter) objInstIdx(obj types2.Object, explicits []types2.Type, dict *writerDict) objInfo {
	explicitInfos := make([]typeInfo, len(explicits))
	for i := range explicitInfos {
		explicitInfos[i] = pw.typIdx(explicits[i], dict)
	}
	return objInfo{idx: pw.objIdx(obj), explicits: explicitInfos}
}

// objIdx returns the index for the given Object, adding it to the
// export data as needed.
func (pw *pkgWriter) objIdx(obj types2.Object) index {
	// TODO(mdempsky): Validate that obj is a global object (or a local
	// defined type, which we hoist to global scope anyway).

	if idx, ok := pw.objsIdx[obj]; ok {
		return idx
	}

	dict := &writerDict{
		derivedIdx: make(map[types2.Type]index),
	}

	if isDefinedType(obj) && obj.Pkg() == pw.curpkg {
		decl, ok := pw.typDecls[obj.(*types2.TypeName)]
		if !ok {
			base.Fatalf("%v not in pw.typDecls", obj.(*types2.TypeName))
		}
		dict.implicits = decl.implicits
	}

	if isGenericMethod(obj.Type()) {
		dict.receivers = asTypeParamSlice(obj.Type().(*types2.Signature).RecvTypeParams())
	}

	// We encode objects into 4 elements across different sections, all
	// sharing the same index:
	//
	// - RelocName has just the object's qualified name (i.e.,
	//   Object.Pkg and Object.Name) and the CodeObj indicating what
	//   specific type of Object it is (Var, Func, etc).
	//
	// - RelocObj has the remaining public details about the object,
	//   relevant to go/types importers.
	//
	// - RelocObjExt has additional private details about the object,
	//   which are only relevant to cmd/compile itself. This is
	//   separated from RelocObj so that go/types importers are
	//   unaffected by internal compiler changes.
	//
	// - RelocObjDict has public details about the object's type
	//   parameters and derived type's used by the object. This is
	//   separated to facilitate the eventual introduction of
	//   shape-based stenciling.
	//
	// TODO(mdempsky): Re-evaluate whether RelocName still makes sense
	// to keep separate from RelocObj.

	w := pw.newWriter(pkgbits.SectionObj, pkgbits.SyncObject1)
	wext := pw.newWriter(pkgbits.SectionObjExt, pkgbits.SyncObject1)
	wname := pw.newWriter(pkgbits.SectionName, pkgbits.SyncObject1)
	wdict := pw.newWriter(pkgbits.SectionObjDict, pkgbits.SyncObject1)

	pw.objsIdx[obj] = w.Idx // break cycles
	assert(wext.Idx == w.Idx)
	assert(wname.Idx == w.Idx)
	assert(wdict.Idx == w.Idx)

	w.dict = dict
	wext.dict = dict

	code := w.doObj(wext, obj)
	w.Flush()
	wext.Flush()

	wname.qualifiedIdent(obj)
	wname.Code(code)
	wname.Flush()

	wdict.objDict(obj, w.dict)
	wdict.Flush()

	return w.Idx
}

// doObj writes the RelocObj definition for obj to w, and the
// RelocObjExt definition to wext.
func (w *writer) doObj(wext *writer, obj types2.Object) pkgbits.CodeObj {
	if obj.Pkg() != w.p.curpkg {
		return pkgbits.ObjStub
	}

	switch obj := obj.(type) {
	default:
		w.p.unexpected("object", obj)
		panic("unreachable")

	case *types2.Const:
		w.pos(obj)
		w.typ(obj.Type())
		w.Value(obj.Val())
		return pkgbits.ObjConst

	case *types2.Func:
		if base.Flag.LowerH > 0 {
			// Unified IR panics are the worst; this is a huge help in debugging them.
			defer func() {
				if p := recover(); p != nil {
					fmt.Printf("Intercepted unified IR writer panic for function %s, repanicking", obj.FullName())
					panic(p)
				}
			}()
		}
		decl, ok := w.p.funDecls[obj]
		assert(ok)
		sig := obj.Type().(*types2.Signature)

		w.pos(obj)
		if isGenericMethod(sig) {
			w.Bool(true) // generic method

			w.selector(obj)
			w.typeParamNames(sig.RecvTypeParams())
			w.param(sig.Recv())
		} else {
			if w.Version().Has(pkgbits.GenericMethods) {
				w.Bool(false) // function
			}
		}
		w.typeParamNames(sig.TypeParams())
		w.signature(sig)
		w.pos(decl)
		wext.funcExt(obj)
		return pkgbits.ObjFunc

	case *types2.TypeName:
		if obj.IsAlias() {
			w.pos(obj)
			rhs := obj.Type()
			var tparams *types2.TypeParamList
			if alias, ok := rhs.(*types2.Alias); ok { // materialized alias
				assert(alias.TypeArgs() == nil)
				tparams = alias.TypeParams()
				rhs = alias.Rhs()
			}
			if w.Version().Has(pkgbits.AliasTypeParamNames) {
				w.typeParamNames(tparams)
			}
			assert(w.Version().Has(pkgbits.AliasTypeParamNames) || tparams.Len() == 0)
			w.typ(rhs)
			return pkgbits.ObjAlias
		}

		named := obj.Type().(*types2.Named)
		assert(named.TypeArgs() == nil)

		w.pos(obj)
		w.typeParamNames(named.TypeParams())
		wext.typeExt(obj)
		w.typ(named.Underlying())

		// separate generic and non-generic methods
		var methods, gmethods []*types2.Func
		for i := range named.NumMethods() {
			m := named.Method(i)
			if isGenericMethod(m.Type()) {
				gmethods = append(gmethods, m)
			} else {
				methods = append(methods, m)
			}
		}
		// encode non-generic methods inline
		w.Len(len(methods))
		for _, m := range methods {
			w.method(wext, m)
		}
		if len(gmethods) > 0 {
			assert(w.Version().Has(pkgbits.GenericMethods))
		}
		// encode a pointer to each generic method
		if w.Version().Has(pkgbits.GenericMethods) {
			w.Len(len(gmethods))
			for _, m := range gmethods {
				w.Reloc(pkgbits.SectionObj, w.p.objIdx(m))
			}
		}

		return pkgbits.ObjType

	case *types2.Var:
		w.pos(obj)
		w.typ(obj.Type())
		wext.varExt(obj)
		return pkgbits.ObjVar
	}
}

// objDict writes the dictionary needed for reading the given object.
func (w *writer) objDict(obj types2.Object, dict *writerDict) {
	// TODO(mdempsky): Split objDict into multiple entries? reader.go
	// doesn't care about the type parameter bounds, and reader2.go
	// doesn't care about referenced functions.

	w.dict = dict // TODO(mdempsky): This is a bit sketchy.
	w.Len(len(dict.implicits))

	rtparams := objRecvTypeParams(obj)
	tparams := objTypeParams(obj)

	if w.Version().Has(pkgbits.GenericMethods) {
		w.Len(len(rtparams))
	} else {
		assert(len(rtparams) == 0)
	}
	w.Len(len(tparams))

	for _, rtparam := range rtparams {
		w.typ(rtparam.Constraint())
	}
	for _, tparam := range tparams {
		w.typ(tparam.Constraint())
	}

	nderived := len(dict.derived)
	w.Len(nderived)
	for _, typ := range dict.derived {
		w.Reloc(pkgbits.SectionType, typ.idx)
		if w.Version().Has(pkgbits.DerivedInfoNeeded) {
			w.Bool(false)
		}
	}

	// Write runtime dictionary information.
	//
	// N.B., the go/types importer reads up to the section, but doesn't
	// read any further, so it's safe to change. (See TODO above.)

	// For each type parameter, write out whether the constraint is a
	// basic interface. This is used to determine how aggressively we
	// can shape corresponding type arguments.
	//
	// This is somewhat redundant with writing out the full type
	// parameter constraints above, but the compiler currently skips
	// over those. Also, we don't care about the *declared* constraints,
	// but how the type parameters are actually *used*. E.g., if a type
	// parameter is constrained to `int | uint` but then never used in
	// arithmetic/conversions/etc, we could shape those together.
	for _, implicit := range dict.implicits {
		w.Bool(implicit.Underlying().(*types2.Interface).IsMethodSet())
	}
	for _, rtparam := range rtparams {
		w.Bool(rtparam.Underlying().(*types2.Interface).IsMethodSet())
	}
	for _, tparam := range tparams {
		w.Bool(tparam.Underlying().(*types2.Interface).IsMethodSet())
	}

	w.Len(len(dict.typeParamMethodExprs))
	for _, info := range dict.typeParamMethodExprs {
		w.Len(info.typeParamIdx)
		w.selectorInfo(info.methodInfo)
	}

	w.Len(len(dict.subdicts))
	for _, info := range dict.subdicts {
		w.objInfo(info)
	}

	w.Len(len(dict.rtypes))
	for _, info := range dict.rtypes {
		w.typInfo(info)
	}

	w.Len(len(dict.itabs))
	for _, info := range dict.itabs {
		w.typInfo(info.typ)
		w.typInfo(info.iface)
	}

	assert(len(dict.derived) == nderived)
}

func (w *writer) typeParamNames(tparams *types2.TypeParamList) {
	w.Sync(pkgbits.SyncTypeParamNames)

	ntparams := tparams.Len()
	for i := 0; i < ntparams; i++ {
		tparam := tparams.At(i).Obj()
		w.pos(tparam)
		w.localIdent(tparam)
	}
}

func (w *writer) method(wext *writer, meth *types2.Func) {
	decl, ok := w.p.funDecls[meth]
	assert(ok)
	sig := meth.Type().(*types2.Signature)

	w.Sync(pkgbits.SyncMethod)
	w.pos(meth)
	w.selector(meth)
	w.typeParamNames(sig.RecvTypeParams())
	w.param(sig.Recv())
	w.signature(sig)

	w.pos(decl) // XXX: Hack to workaround linker limitations.
	wext.funcExt(meth)
}

// qualifiedIdent writes out the name of an object typically declared at package
// scope. It's also used to refer to generic methods and locally defined types.
func (w *writer) qualifiedIdent(obj types2.Object) {
	w.Sync(pkgbits.SyncSym)

	name := obj.Name()
	if isDefinedType(obj) && obj.Pkg() == w.p.curpkg {
		decl, ok := w.p.typDecls[obj.(*types2.TypeName)]
		assert(ok)
		if decl.gen != 0 {
			// For local defined types, we embed a scope-disambiguation
			// number directly into their name. types.SplitVargenSuffix then
			// knows to look for this.
			//
			// TODO(mdempsky): Find a better solution; this is terrible.
			name = fmt.Sprintf("%s·%v", name, decl.gen)
		}
	}

	// Generic methods are promoted to objects and thus need qualified identifiers.
	// They must be contextualized by their defining type.
	if isGenericMethod(obj.Type()) {
		recv := obj.Type().(*types2.Signature).Recv().Type()
		fstr := "%s.%s"
		if _, ok := recv.(*types2.Pointer); ok {
			fstr = "(*%s).%s"
		}
		name = fmt.Sprintf(fstr, types2.Unalias(deref2(recv)).(*types2.Named).Obj().Name(), name)
	}

	w.pkg(obj.Pkg())
	w.String(name)
}

// TODO(mdempsky): We should be able to omit pkg from both localIdent
// and selector, because they should always be known from context.
// However, past frustrations with this optimization in iexport make
// me a little nervous to try it again.

// localIdent writes the name of a locally declared object (i.e.,
// objects that can only be accessed by non-qualified name, within the
// context of a particular function).
func (w *writer) localIdent(obj types2.Object) {
	assert(!isGlobal(obj))
	w.Sync(pkgbits.SyncLocalIdent)
	w.pkg(obj.Pkg())
	w.String(obj.Name())
}

// selector writes the name of a field or method (i.e., objects that
// can only be accessed using selector expressions).
func (w *writer) selector(obj types2.Object) {
	w.selectorInfo(w.p.selectorIdx(obj))
}

func (w *writer) selectorInfo(info selectorInfo) {
	w.Sync(pkgbits.SyncSelector)
	w.pkgRef(info.pkgIdx)
	w.StringRef(info.nameIdx)
}

func (pw *pkgWriter) selectorIdx(obj types2.Object) selectorInfo {
	pkgIdx := pw.pkgIdx(obj.Pkg())
	nameIdx := pw.StringIdx(obj.Name())
	return selectorInfo{pkgIdx: pkgIdx, nameIdx: nameIdx}
}

// @@@ Compiler extensions

func (w *writer) funcExt(obj *types2.Func) {
	decl, ok := w.p.funDecls[obj]
	assert(ok)

	// TODO(mdempsky): Extend these pragma validation flags to account
	// for generics. E.g., linkname probably doesn't make sense at
	// least.

	pragma := asPragmaFlag(decl.Pragma)
	if pragma&ir.Systemstack != 0 && pragma&ir.Nosplit != 0 {
		w.p.errorf(decl, "go:nosplit and go:systemstack cannot be combined")
	}
	wi := asWasmImport(decl.Pragma)
	we := asWasmExport(decl.Pragma)

	if decl.Body != nil {
		if pragma&ir.Noescape != 0 {
			w.p.errorf(decl, "can only use //go:noescape with external func implementations")
		}
		if wi != nil {
			w.p.errorf(decl, "can only use //go:wasmimport with external func implementations")
		}
		if (pragma&ir.UintptrKeepAlive != 0 && pragma&ir.UintptrEscapes == 0) && pragma&ir.Nosplit == 0 {
			// Stack growth can't handle uintptr arguments that may
			// be pointers (as we don't know which are pointers
			// when creating the stack map). Thus uintptrkeepalive
			// functions (and all transitive callees) must be
			// nosplit.
			//
			// N.B. uintptrescapes implies uintptrkeepalive but it
			// is OK since the arguments must escape to the heap.
			//
			// TODO(prattmic): Add recursive nosplit check of callees.
			// TODO(prattmic): Functions with no body (i.e.,
			// assembly) must also be nosplit, but we can't check
			// that here.
			w.p.errorf(decl, "go:uintptrkeepalive requires go:nosplit")
		}
	} else {
		if base.Flag.Complete || decl.Name.Value == "init" {
			// Linknamed functions are allowed to have no body. Hopefully
			// the linkname target has a body. See issue 23311.
			// Wasmimport functions are also allowed to have no body.
			if _, ok := w.p.linknames[obj]; !ok && wi == nil {
				w.p.errorf(decl, "missing function body")
			}
		}
	}

	sig, block := obj.Type().(*types2.Signature), decl.Body
	body, closureVars := w.p.bodyIdx(sig, block, w.dict)
	if len(closureVars) > 0 {
		fmt.Fprintln(os.Stderr, "CLOSURE", closureVars)
	}
	assert(len(closureVars) == 0)

	w.Sync(pkgbits.SyncFuncExt)
	w.pragmaFlag(pragma)
	w.linkname(obj)

	if buildcfg.GOARCH == "wasm" {
		if wi != nil {
			w.String(wi.Module)
			w.String(wi.Name)
		} else {
			w.String("")
			w.String("")
		}
		if we != nil {
			w.String(we.Name)
		} else {
			w.String("")
		}
	}

	w.Bool(false) // stub extension
	w.Reloc(pkgbits.SectionBody, body)
	w.Sync(pkgbits.SyncEOF)
}

func (w *writer) typeExt(obj *types2.TypeName) {
	decl, ok := w.p.typDecls[obj]
	assert(ok)

	w.Sync(pkgbits.SyncTypeExt)

	w.pragmaFlag(asPragmaFlag(decl.Pragma))

	// No LSym.SymIdx info yet.
	w.Int64(-1)
	w.Int64(-1)
}

func (w *writer) varExt(obj *types2.Var) {
	w.Sync(pkgbits.SyncVarExt)
	w.linkname(obj)
}

func (w *writer) linkname(obj types2.Object) {
	w.Sync(pkgbits.SyncLinkname)
	w.Int64(-1)
	info := w.p.linknames[obj]
	w.String(info.remote)
	w.Bool(info.std)
}

func (w *writer) pragmaFlag(p ir.PragmaFlag) {
	w.Sync(pkgbits.SyncPragma)
	w.Int(int(p))
}

// @@@ Function bodies

// bodyIdx returns the index for the given function body (specified by
// block), adding it to the export data
func (pw *pkgWriter) bodyIdx(sig *types2.Signature, block *syntax.BlockStmt, dict *writerDict) (idx index, closureVars []posVar) {
	w := pw.newWriter(pkgbits.SectionBody, pkgbits.SyncFuncBody)
	w.sig = sig
	w.dict = dict

	w.declareParams(sig)
	if w.Bool(block != nil) {
		w.stmts(block.List)
		w.pos(block.Rbrace)
	}

	return w.Flush(), w.closureVars
}

func (w *writer) declareParams(sig *types2.Signature) {
	addLocals := func(params *types2.Tuple) {
		for i := 0; i < params.Len(); i++ {
			w.addLocal(params.At(i))
		}
	}

	if recv := sig.Recv(); recv != nil {
		w.addLocal(recv)
	}
	addLocals(sig.Params())
	addLocals(sig.Results())
}

// addLocal records the declaration of a new local variable.
func (w *writer) addLocal(obj *types2.Var) {
	idx := len(w.localsIdx)

	w.Sync(pkgbits.SyncAddLocal)
	if w.p.SyncMarkers() {
		w.Int(idx)
	}
	w.varDictIndex(obj)

	if w.localsIdx == nil {
		w.localsIdx = make(map[*types2.Var]int)
	}
	w.localsIdx[obj] = idx
}

// useLocal writes a reference to the given local or free variable
// into the bitstream.
func (w *writer) useLocal(pos syntax.Pos, obj *types2.Var) {
	w.Sync(pkgbits.SyncUseObjLocal)

	if idx, ok := w.localsIdx[obj]; w.Bool(ok) {
		w.Len(idx)
		return
	}

	idx, ok := w.closureVarsIdx[obj]
	if !ok {
		if w.closureVarsIdx == nil {
			w.closureVarsIdx = make(map[*types2.Var]int)
		}
		idx = len(w.closureVars)
		w.closureVars = append(w.closureVars, posVar{pos, obj})
		w.closureVarsIdx[obj] = idx
	}
	w.Len(idx)
}

func (w *writer) openScope(pos syntax.Pos) {
	w.Sync(pkgbits.SyncOpenScope)
	w.pos(pos)
}

func (w *writer) closeScope(pos syntax.Pos) {
	w.Sync(pkgbits.SyncCloseScope)
	w.pos(pos)
	w.closeAnotherScope()
}

func (w *writer) closeAnotherScope() {
	w.Sync(pkgbits.SyncCloseAnotherScope)
}

// @@@ Statements

// stmt writes the given statement into the function body bitstream.
func (w *writer) stmt(stmt syntax.Stmt) {
	var stmts []syntax.Stmt
	if stmt != nil {
		stmts = []syntax.Stmt{stmt}
	}
	w.stmts(stmts)
}

func (w *writer) stmts(stmts []syntax.Stmt) {
	dead := false
	w.Sync(pkgbits.SyncStmts)
	var lastLabel = -1
	for i, stmt := range stmts {
		if _, ok := stmt.(*syntax.LabeledStmt); ok {
			lastLabel = i
		}
	}
	for i, stmt := range stmts {
		if dead && i > lastLabel {
			// Any statements after a terminating and last label statement are safe to omit.
			// Otherwise, code after label statement may refer to dead stmts between terminating
			// and label statement, see issue #65593.
			if _, ok := stmt.(*syntax.LabeledStmt); !ok {
				continue
			}
		}
		w.stmt1(stmt)
		dead = w.p.terminates(stmt)
	}
	w.Code(stmtEnd)
	w.Sync(pkgbits.SyncStmtsEnd)
}

func (w *writer) stmt1(stmt syntax.Stmt) {
	switch stmt := stmt.(type) {
	default:
		w.p.unexpected("statement", stmt)

	case nil, *syntax.EmptyStmt:
		return

	case *syntax.AssignStmt:
		switch {
		case stmt.Rhs == nil:
			w.Code(stmtIncDec)
			w.op(binOps[stmt.Op])
			w.expr(stmt.Lhs)
			w.pos(stmt)

		case stmt.Op != 0 && stmt.Op != syntax.Def:
			w.Code(stmtAssignOp)
			w.op(binOps[stmt.Op])
			w.expr(stmt.Lhs)
			w.pos(stmt)

			var typ types2.Type
			if stmt.Op != syntax.Shl && stmt.Op != syntax.Shr {
				typ = w.p.typeOf(stmt.Lhs)
			}
			w.implicitConvExpr(typ, stmt.Rhs)

		default:
			w.assignStmt(stmt, stmt.Lhs, stmt.Rhs)
		}

	case *syntax.BlockStmt:
		w.Code(stmtBlock)
		w.blockStmt(stmt)

	case *syntax.BranchStmt:
		w.Code(stmtBranch)
		w.pos(stmt)
		var op ir.Op
		switch stmt.Tok {
		case syntax.Break:
			op = ir.OBREAK
		case syntax.Continue:
			op = ir.OCONTINUE
		case syntax.Fallthrough:
			op = ir.OFALL
		case syntax.Goto:
			op = ir.OGOTO
		}
		w.op(op)
		w.optLabel(stmt.Label)

	case *syntax.CallStmt:
		w.Code(stmtCall)
		w.pos(stmt)
		var op ir.Op
		switch stmt.Tok {
		case syntax.Defer:
			op = ir.ODEFER
		case syntax.Go:
			op = ir.OGO
		}
		w.op(op)
		w.expr(stmt.Call)
		if stmt.Tok == syntax.Defer {
			w.optExpr(stmt.DeferAt)
		}

	case *syntax.DeclStmt:
		for _, decl := range stmt.DeclList {
			w.declStmt(decl)
		}

	case *syntax.ExprStmt:
		w.Code(stmtExpr)
		w.expr(stmt.X)

	case *syntax.ForStmt:
		w.Code(stmtFor)
		w.forStmt(stmt)

	case *syntax.IfStmt:
		w.Code(stmtIf)
		w.ifStmt(stmt)

	case *syntax.LabeledStmt:
		w.Code(stmtLabel)
		w.pos(stmt)
		w.label(stmt.Label)
		w.stmt1(stmt.Stmt)

	case *syntax.ReturnStmt:
		w.Code(stmtReturn)
		w.pos(stmt)

		resultTypes := w.sig.Results()
		dstType := func(i int) types2.Type {
			return resultTypes.At(i).Type()
		}
		w.multiExpr(stmt, dstType, syntax.UnpackListExpr(stmt.Results))

	case *syntax.SelectStmt:
		w.Code(stmtSelect)
		w.selectStmt(stmt)

	case *syntax.SendStmt:
		chanType := types2.CoreType(w.p.typeOf(stmt.Chan)).(*types2.Chan)

		w.Code(stmtSend)
		w.pos(stmt)
		w.expr(stmt.Chan)
		w.implicitConvExpr(chanType.Elem(), stmt.Value)

	case *syntax.SwitchStmt:
		w.Code(stmtSwitch)
		w.switchStmt(stmt)
	}
}

func (w *writer) assignList(expr syntax.Expr) {
	exprs := syntax.UnpackListExpr(expr)
	w.Len(len(exprs))

	for _, expr := range exprs {
		w.assign(expr)
	}
}

func (w *writer) assign(expr syntax.Expr) {
	expr = syntax.Unparen(expr)

	if name, ok := expr.(*syntax.Name); ok {
		if name.Value == "_" {
			w.Code(assignBlank)
			return
		}

		if obj, ok := w.p.info.Defs[name]; ok {
			obj := obj.(*types2.Var)

			w.Code(assignDef)
			w.pos(obj)
			w.localIdent(obj)
			w.typ(obj.Type())

			// TODO(mdempsky): Minimize locals index size by deferring
			// this until the variables actually come into scope.
			w.addLocal(obj)
			return
		}
	}

	w.Code(assignExpr)
	w.expr(expr)
}

func (w *writer) declStmt(decl syntax.Decl) {
	switch decl := decl.(type) {
	default:
		w.p.unexpected("declaration", decl)

	case *syntax.ConstDecl, *syntax.TypeDecl:

	case *syntax.VarDecl:
		w.assignStmt(decl, namesAsExpr(decl.NameList), decl.Values)
	}
}

// assignStmt writes out an assignment for "lhs = rhs".
func (w *writer) assignStmt(pos poser, lhs0, rhs0 syntax.Expr) {
	lhs := syntax.UnpackListExpr(lhs0)
	rhs := syntax.UnpackListExpr(rhs0)

	w.Code(stmtAssign)
	w.pos(pos)

	// As if w.assignList(lhs0).
	w.Len(len(lhs))
	for _, expr := range lhs {
		w.assign(expr)
	}

	dstType := func(i int) types2.Type {
		dst := lhs[i]

		// Finding dstType is somewhat involved, because for VarDecl
		// statements, the Names are only added to the info.{Defs,Uses}
		// maps, not to info.Types.
		if name, ok := syntax.Unparen(dst).(*syntax.Name); ok {
			if name.Value == "_" {
				return nil // ok: no implicit conversion
			} else if def, ok := w.p.info.Defs[name].(*types2.Var); ok {
				return def.Type()
			} else if use, ok := w.p.info.Uses[name].(*types2.Var); ok {
				return use.Type()
			} else {
				w.p.fatalf(dst, "cannot find type of destination object: %v", dst)
			}
		}

		return w.p.typeOf(dst)
	}

	w.multiExpr(pos, dstType, rhs)
}

func (w *writer) blockStmt(stmt *syntax.BlockStmt) {
	w.Sync(pkgbits.SyncBlockStmt)
	w.openScope(stmt.Pos())
	w.stmts(stmt.List)
	w.closeScope(stmt.Rbrace)
}

func (w *writer) forStmt(stmt *syntax.ForStmt) {
	w.Sync(pkgbits.SyncForStmt)
	w.openScope(stmt.Pos())

	if rang, ok := stmt.Init.(*syntax.RangeClause); w.Bool(ok) {
		w.pos(rang)
		w.assignList(rang.Lhs)
		w.expr(rang.X)

		xtyp := w.p.typeOf(rang.X)
		if _, isMap := types2.CoreType(xtyp).(*types2.Map); isMap {
			w.rtype(xtyp)
		}
		{
			lhs := syntax.UnpackListExpr(rang.Lhs)
			assign := func(i int, src types2.Type) {
				if i >= len(lhs) {
					return
				}
				dst := syntax.Unparen(lhs[i])
				if name, ok := dst.(*syntax.Name); ok && name.Value == "_" {
					return
				}

				var dstType types2.Type
				if rang.Def {
					// For `:=` assignments, the LHS names only appear in Defs,
					// not Types (as used by typeOf).
					dstType = w.p.info.Defs[dst.(*syntax.Name)].(*types2.Var).Type()
				} else {
					dstType = w.p.typeOf(dst)
				}

				w.convRTTI(src, dstType)
			}

			keyType, valueType := types2.RangeKeyVal(w.p.typeOf(rang.X))
			assign(0, keyType)
			assign(1, valueType)
		}

	} else {
		if stmt.Cond != nil && w.p.staticBool(&stmt.Cond) < 0 { // always false
			stmt.Post = nil
			stmt.Body.List = nil
		}

		w.pos(stmt)
		w.stmt(stmt.Init)
		w.optExpr(stmt.Cond)
		w.stmt(stmt.Post)
	}

	w.blockStmt(stmt.Body)
	w.Bool(w.distinctVars(stmt))
	w.closeAnotherScope()
}

func (w *writer) distinctVars(stmt *syntax.ForStmt) bool {
	lv := base.Debug.LoopVar
	fileVersion := w.p.info.FileVersions[stmt.Pos().FileBase()]
	is122 := fileVersion == "" || version.Compare(fileVersion, "go1.22") >= 0

	// Turning off loopvar for 1.22 is only possible with loopvarhash=qn
	//
	// Debug.LoopVar values to be preserved for 1.21 compatibility are 1 and 2,
	// which are also set (=1) by GOEXPERIMENT=loopvar.  The knobs for turning on
	// the new, unshared, loopvar behavior apply to versions less than 1.21 because
	// (1) 1.21 also did that and (2) this is believed to be the likely use case;
	// anyone checking to see if it affects their code will just run the GOEXPERIMENT
	// but will not also update all their go.mod files to 1.21.
	//
	// -gcflags=-d=loopvar=3 enables logging for 1.22 but does not turn loopvar on for <= 1.21.

	return is122 || lv > 0 && lv != 3
}

func (w *writer) ifStmt(stmt *syntax.IfStmt) {
	cond := w.p.staticBool(&stmt.Cond)

	w.Sync(pkgbits.SyncIfStmt)
	w.openScope(stmt.Pos())
	w.pos(stmt)
	w.stmt(stmt.Init)
	w.expr(stmt.Cond)
	w.Int(cond)
	if cond >= 0 {
		w.blockStmt(stmt.Then)
	} else {
		w.pos(stmt.Then.Rbrace)
	}
	if cond <= 0 {
		w.stmt(stmt.Else)
	}
	w.closeAnotherScope()
}

func (w *writer) selectStmt(stmt *syntax.SelectStmt) {
	w.Sync(pkgbits.SyncSelectStmt)

	w.pos(stmt)
	w.Len(len(stmt.Body))
	for i, clause := range stmt.Body {
		if i > 0 {
			w.closeScope(clause.Pos())
		}
		w.openScope(clause.Pos())

		w.pos(clause)
		w.stmt(clause.Comm)
		w.stmts(clause.Body)
	}
	if len(stmt.Body) > 0 {
		w.closeScope(stmt.Rbrace)
	}
}

func (w *writer) switchStmt(stmt *syntax.SwitchStmt) {
	w.Sync(pkgbits.SyncSwitchStmt)

	w.openScope(stmt.Pos())
	w.pos(stmt)
	w.stmt(stmt.Init)

	var iface, tagType types2.Type
	var tagTypeIsChan bool
	if guard, ok := stmt.Tag.(*syntax.TypeSwitchGuard); w.Bool(ok) {
		iface = w.p.typeOf(guard.X)

		w.pos(guard)
		if tag := guard.Lhs; w.Bool(tag != nil) {
			w.pos(tag)

			// Like w.localIdent, but we don't have a types2.Object.
			w.Sync(pkgbits.SyncLocalIdent)
			w.pkg(w.p.curpkg)
			w.String(tag.Value)
		}
		w.expr(guard.X)
	} else {
		tag := stmt.Tag

		var tagValue constant.Value
		if tag != nil {
			tv := w.p.typeAndValue(tag)
			tagType = tv.Type
			tagValue = tv.Value
			_, tagTypeIsChan = tagType.Underlying().(*types2.Chan)
		} else {
			tagType = types2.Typ[types2.Bool]
			tagValue = constant.MakeBool(true)
		}

		if tagValue != nil {
			// If the switch tag has a constant value, look for a case
			// clause that we always branch to.
			func() {
				var target *syntax.CaseClause
			Outer:
				for _, clause := range stmt.Body {
					if clause.Cases == nil {
						target = clause
					}
					for _, cas := range syntax.UnpackListExpr(clause.Cases) {
						tv := w.p.typeAndValue(cas)
						if tv.Value == nil {
							return // non-constant case; give up
						}
						if constant.Compare(tagValue, token.EQL, tv.Value) {
							target = clause
							break Outer
						}
					}
				}
				// We've found the target clause, if any.

				if target != nil {
					if hasFallthrough(target.Body) {
						return // fallthrough is tricky; give up
					}

					// Rewrite as single "default" case.
					target.Cases = nil
					stmt.Body = []*syntax.CaseClause{target}
				} else {
					stmt.Body = nil
				}

				// Clear switch tag (i.e., replace with implicit "true").
				tag = nil
				stmt.Tag = nil
				tagType = types2.Typ[types2.Bool]
			}()
		}

		// Walk is going to emit comparisons between the tag value and
		// each case expression, and we want these comparisons to always
		// have the same type. If there are any case values that can't be
		// converted to the tag value's type, then convert everything to
		// `any` instead.
		//
		// Except that we need to keep comparisons of channel values from
		// being wrapped in any(). See issue #67190.

		if !tagTypeIsChan {
		Outer:
			for _, clause := range stmt.Body {
				for _, cas := range syntax.UnpackListExpr(clause.Cases) {
					if casType := w.p.typeOf(cas); !types2.AssignableTo(casType, tagType) && (types2.IsInterface(casType) || types2.IsInterface(tagType)) {
						tagType = types2.NewInterfaceType(nil, nil)
						break Outer
					}
				}
			}
		}

		if w.Bool(tag != nil) {
			w.implicitConvExpr(tagType, tag)
		}
	}

	w.Len(len(stmt.Body))
	for i, clause := range stmt.Body {
		if i > 0 {
			w.closeScope(clause.Pos())
		}
		w.openScope(clause.Pos())

		w.pos(clause)

		cases := syntax.UnpackListExpr(clause.Cases)
		if iface != nil {
			w.Len(len(cases))
			for _, cas := range cases {
				if w.Bool(isNil(w.p, cas)) {
					continue
				}
				w.exprType(iface, cas)
			}
		} else {
			// As if w.exprList(clause.Cases),
			// but with implicit conversions to tagType.

			w.Sync(pkgbits.SyncExprList)
			w.Sync(pkgbits.SyncExprs)
			w.Len(len(cases))
			for _, cas := range cases {
				typ := tagType
				if tagTypeIsChan {
					typ = nil
				}
				w.implicitConvExpr(typ, cas)
			}
		}

		if obj, ok := w.p.info.Implicits[clause]; ok {
			// TODO(mdempsky): These pos details are quirkish, but also
			// necessary so the variable's position is correct for DWARF
			// scope assignment later. It would probably be better for us to
			// instead just set the variable's DWARF scoping info earlier so
			// we can give it the correct position information.
			pos := clause.Pos()
			if typs := syntax.UnpackListExpr(clause.Cases); len(typs) != 0 {
				pos = typeExprEndPos(typs[len(typs)-1])
			}
			w.pos(pos)

			obj := obj.(*types2.Var)
			w.typ(obj.Type())
			w.addLocal(obj)
		}

		w.stmts(clause.Body)
	}
	if len(stmt.Body) > 0 {
		w.closeScope(stmt.Rbrace)
	}

	w.closeScope(stmt.Rbrace)
}

func (w *writer) label(label *syntax.Name) {
	w.Sync(pkgbits.SyncLabel)

	// TODO(mdempsky): Replace label strings with dense indices.
	w.String(label.Value)
}

func (w *writer) optLabel(label *syntax.Name) {
	w.Sync(pkgbits.SyncOptLabel)
	if w.Bool(label != nil) {
		w.label(label)
	}
}

// @@@ Expressions

// expr writes the given expression into the function body bitstream.
func (w *writer) expr(expr syntax.Expr) {
	base.Assertf(expr != nil, "missing expression")

	expr = syntax.Unparen(expr) // skip parens; unneeded after typecheck

	obj, inst := lookupObj(w.p, expr)
	targs := asTypeSlice(inst.TypeArgs)

	if tv, ok := w.p.maybeTypeAndValue(expr); ok {
		if tv.IsRuntimeHelper() {
			if pkg := obj.Pkg(); pkg != nil && pkg.Name() == "runtime" {
				objName := obj.Name()
				w.Code(exprRuntimeBuiltin)
				w.String(objName)
				return
			}
		}

		if tv.IsType() {
			w.p.fatalf(expr, "unexpected type expression %v", syntax.String(expr))
		}

		if tv.Value != nil {
			w.Code(exprConst)
			w.pos(expr)
			typ := idealType(tv)
			assert(typ != nil)
			w.typ(typ)
			w.Value(tv.Value)
			return
		}

		if _, isNil := obj.(*types2.Nil); isNil {
			w.Code(exprZero)
			w.pos(expr)
			w.typ(tv.Type)
			return
		}

		// With shape types (and particular pointer shaping), we may have
		// an expression of type "go.shape.*uint8", but need to reshape it
		// to another shape-identical type to allow use in field
		// selection, indexing, etc.
		if typ := tv.Type; !tv.IsBuiltin() && !isTuple(typ) && !isUntyped(typ) {
			w.Code(exprReshape)
			w.typ(typ)
			// fallthrough
		}
	}

	if obj != nil {
		if len(targs) != 0 {
			obj := obj.(*types2.Func)

			w.Code(exprFuncInst)
			w.pos(expr)
			w.funcInst(obj, targs)
			return
		}

		if isGlobal(obj) {
			w.Code(exprGlobal)
			w.obj(obj, nil)
			return
		}

		obj := obj.(*types2.Var)
		assert(!obj.IsField())

		w.Code(exprLocal)
		w.useLocal(expr.Pos(), obj)
		return
	}

	switch expr := expr.(type) {
	default:
		w.p.unexpected("expression", expr)

	case *syntax.CompositeLit:
		w.Code(exprCompLit)
		w.compLit(expr)

	case *syntax.FuncLit:
		w.Code(exprFuncLit)
		w.funcLit(expr)

	case *syntax.SelectorExpr:
		sel, ok := w.p.info.Selections[expr]
		assert(ok)

		switch sel.Kind() {
		default:
			w.p.fatalf(expr, "unexpected selection kind: %v", sel.Kind())

		case types2.FieldVal:
			w.Code(exprFieldVal)
			w.expr(expr.X)
			w.pos(expr)
			w.selector(sel.Obj())

		case types2.MethodVal:
			w.methVal(expr, sel)

		case types2.MethodExpr:
			w.methExpr(expr, sel)
		}

	case *syntax.IndexExpr:
		// might be explicit instantiation of a generic method
		if selector, ok := expr.X.(*syntax.SelectorExpr); ok {
			if sel, ok := w.p.info.Selections[selector]; ok {
				switch sel.Kind() {
				default:
					w.p.fatalf(selector, "unexpected selection kind: %v", sel.Kind())
				case types2.FieldVal:
					// not a method
				case types2.MethodVal:
					w.methVal(selector, sel)
					return
				case types2.MethodExpr:
					w.methExpr(selector, sel)
					return
				}
			}
		}
		_ = w.p.typeOf(expr.Index) // ensure this is an index expression, not an instantiation

		xtyp := w.p.typeOf(expr.X)

		var keyType types2.Type
		if mapType, ok := types2.CoreType(xtyp).(*types2.Map); ok {
			keyType = mapType.Key()
		}

		w.Code(exprIndex)
		w.expr(expr.X)
		w.pos(expr)
		w.implicitConvExpr(keyType, expr.Index)
		if keyType != nil {
			w.rtype(xtyp)
		}

	case *syntax.SliceExpr:
		w.Code(exprSlice)
		w.expr(expr.X)
		w.pos(expr)
		for _, n := range &expr.Index {
			w.optExpr(n)
		}

	case *syntax.AssertExpr:
		iface := w.p.typeOf(expr.X)

		w.Code(exprAssert)
		w.expr(expr.X)
		w.pos(expr)
		w.exprType(iface, expr.Type)
		w.rtype(iface)

	case *syntax.Operation:
		if expr.Y == nil {
			w.Code(exprUnaryOp)
			w.op(unOps[expr.Op])
			w.pos(expr)
			w.expr(expr.X)
			break
		}

		var commonType types2.Type
		switch expr.Op {
		case syntax.Shl, syntax.Shr:
			// ok: operands are allowed to have different types
		default:
			xtyp := w.p.typeOf(expr.X)
			ytyp := w.p.typeOf(expr.Y)
			switch {
			case types2.AssignableTo(xtyp, ytyp):
				commonType = ytyp
			case types2.AssignableTo(ytyp, xtyp):
				commonType = xtyp
			default:
				w.p.fatalf(expr, "failed to find common type between %v and %v", xtyp, ytyp)
			}
		}

		w.Code(exprBinaryOp)
		w.op(binOps[expr.Op])
		w.implicitConvExpr(commonType, expr.X)
		w.pos(expr)
		w.implicitConvExpr(commonType, expr.Y)

	case *syntax.CallExpr:
		tv := w.p.typeAndValue(expr.Fun)
		if tv.IsType() {
			assert(len(expr.ArgList) == 1)
			assert(!expr.HasDots)
			w.convertExpr(tv.Type, expr.ArgList[0], false)
			break
		}

		var rtype types2.Type
		if tv.IsBuiltin() {
			switch obj, _ := lookupObj(w.p, syntax.Unparen(expr.Fun)); obj.Name() {
			case "make":
				assert(len(expr.ArgList) >= 1)
				assert(!expr.HasDots)

				w.Code(exprMake)
				w.pos(expr)
				w.exprType(nil, expr.ArgList[0])
				w.exprs(expr.ArgList[1:])

				typ := w.p.typeOf(expr)
				switch coreType := types2.CoreType(typ).(type) {
				default:
					w.p.fatalf(expr, "unexpected core type: %v", coreType)
				case *types2.Chan:
					w.rtype(typ)
				case *types2.Map:
					w.rtype(typ)
				case *types2.Slice:
					w.rtype(sliceElem(typ))
				}

				return

			case "new":
				assert(len(expr.ArgList) == 1)
				assert(!expr.HasDots)
				arg := expr.ArgList[0]

				w.Code(exprNew)
				w.pos(expr)
				tv := w.p.typeAndValue(arg)
				if w.Bool(!tv.IsType()) {
					w.expr(arg) // new(expr), go1.26
				} else {
					w.exprType(nil, arg) // new(T)
				}
				return

			case "Sizeof":
				assert(len(expr.ArgList) == 1)
				assert(!expr.HasDots)

				w.Code(exprSizeof)
				w.pos(expr)
				w.typ(w.p.typeOf(expr.ArgList[0]))
				return

			case "Alignof":
				assert(len(expr.ArgList) == 1)
				assert(!expr.HasDots)

				w.Code(exprAlignof)
				w.pos(expr)
				w.typ(w.p.typeOf(expr.ArgList[0]))
				return

			case "Offsetof":
				assert(len(expr.ArgList) == 1)
				assert(!expr.HasDots)
				selector := syntax.Unparen(expr.ArgList[0]).(*syntax.SelectorExpr)
				index := w.p.info.Selections[selector].Index()

				w.Code(exprOffsetof)
				w.pos(expr)
				w.typ(deref2(w.p.typeOf(selector.X)))
				w.Len(len(index) - 1)
				for _, idx := range index {
					w.Len(idx)
				}
				return

			case "append":
				rtype = sliceElem(w.p.typeOf(expr))
			case "copy":
				typ := w.p.typeOf(expr.ArgList[0])
				if tuple, ok := typ.(*types2.Tuple); ok { // "copy(g())"
					typ = tuple.At(0).Type()
				}
				rtype = sliceElem(typ)
			case "delete":
				typ := w.p.typeOf(expr.ArgList[0])
				if tuple, ok := typ.(*types2.Tuple); ok { // "delete(g())"
					typ = tuple.At(0).Type()
				}
				rtype = typ
			case "Slice":
				rtype = sliceElem(w.p.typeOf(expr))
			}
		}

		writeFunExpr := func() {
			fun := syntax.Unparen(expr.Fun)

			expr := fun
			if idx, ok := expr.(*syntax.IndexExpr); ok {
				expr = idx.X
			}
			if selector, ok := expr.(*syntax.SelectorExpr); ok {
				if sel, ok := w.p.info.Selections[selector]; ok && sel.Kind() == types2.MethodVal {
					w.Bool(true) // method call
					typ := w.recvExpr(selector, sel)
					w.methodExpr(selector, typ, sel)
					return
				}
			}

			w.Bool(false) // not a method call (i.e., normal function call)

			if obj, inst := lookupObj(w.p, fun); w.Bool(obj != nil && inst.TypeArgs.Len() != 0) {
				obj := obj.(*types2.Func)

				w.pos(fun)
				w.funcInst(obj, asTypeSlice(inst.TypeArgs))
				return
			}

			w.expr(fun)
		}

		sigType := types2.CoreType(tv.Type).(*types2.Signature)
		paramTypes := sigType.Params()

		w.Code(exprCall)
		writeFunExpr()
		w.pos(expr)

		paramType := func(i int) types2.Type {
			if sigType.Variadic() && !expr.HasDots && i >= paramTypes.Len()-1 {
				return paramTypes.At(paramTypes.Len() - 1).Type().(*types2.Slice).Elem()
			}
			return paramTypes.At(i).Type()
		}

		w.multiExpr(expr, paramType, expr.ArgList)
		w.Bool(expr.HasDots)
		if rtype != nil {
			w.rtype(rtype)
		}
	}
}

func sliceElem(typ types2.Type) types2.Type {
	return types2.CoreType(typ).(*types2.Slice).Elem()
}

func (w *writer) optExpr(expr syntax.Expr) {
	if w.Bool(expr != nil) {
		w.expr(expr)
	}
}

func (w *writer) methVal(expr *syntax.SelectorExpr, sel *types2.Selection) {
	w.Code(exprMethodVal)
	typ := w.recvExpr(expr, sel)
	w.pos(expr)
	w.methodExpr(expr, typ, sel)
}

func (w *writer) methExpr(expr *syntax.SelectorExpr, sel *types2.Selection) {
	w.Code(exprMethodExpr)

	tv := w.p.typeAndValue(expr.X)
	assert(tv.IsType())

	index := sel.Index()
	implicits := index[:len(index)-1]

	typ := tv.Type
	w.typ(typ)

	w.Len(len(implicits))
	for _, ix := range implicits {
		w.Len(ix)
		typ = deref2(typ).Underlying().(*types2.Struct).Field(ix).Type()
	}

	recv := sel.Obj().(*types2.Func).Type().(*types2.Signature).Recv().Type()
	if w.Bool(isPtrTo(typ, recv)) { // need deref
		typ = recv
	} else if w.Bool(isPtrTo(recv, typ)) { // need addr
		typ = recv
	}

	w.pos(expr)
	w.methodExpr(expr, typ, sel)
}

// recvExpr writes out expr.X, but handles any implicit addressing,
// dereferencing, and field selections appropriate for the method
// selection.
func (w *writer) recvExpr(expr *syntax.SelectorExpr, sel *types2.Selection) types2.Type {
	index := sel.Index()
	implicits := index[:len(index)-1]

	w.Code(exprRecv)
	w.expr(expr.X)
	w.pos(expr)
	w.Len(len(implicits))

	typ := w.p.typeOf(expr.X)
	for _, ix := range implicits {
		typ = deref2(typ).Underlying().(*types2.Struct).Field(ix).Type()
		w.Len(ix)
	}

	recv := sel.Obj().(*types2.Func).Type().(*types2.Signature).Recv().Type()
	if w.Bool(isPtrTo(typ, recv)) { // needs deref
		typ = recv
	} else if w.Bool(isPtrTo(recv, typ)) { // needs addr
		typ = recv
	}

	return typ
}

// funcInst writes a reference to an instantiated function.
func (w *writer) funcInst(obj *types2.Func, targs []types2.Type) {
	info := w.p.objInstIdx(obj, targs, w.dict)

	// Type arguments list contains derived types; we can emit a static
	// call to the shaped function, but need to dynamically compute the
	// runtime dictionary pointer.
	if w.Bool(info.anyDerived()) {
		w.Len(w.dict.subdictIdx(info))
		return
	}

	// Type arguments list is statically known; we can emit a static
	// call with a statically reference to the respective runtime
	// dictionary.
	w.objInfo(info)
}

// methodExpr writes out a reference to the method selected by
// expr. sel should be the corresponding types2.Selection, and recv
// the type produced after any implicit addressing, dereferencing, and
// field selection. (Note: recv might differ from sel.Obj()'s receiver
// parameter in the case of interface types, and is needed for
// handling type parameter methods.)
func (w *writer) methodExpr(expr *syntax.SelectorExpr, recv types2.Type, sel *types2.Selection) {
	fun := sel.Obj().(*types2.Func)
	sig := fun.Type().(*types2.Signature)

	w.typ(recv)

	// only pass the signature if it's not a generic method
	if isGenericMethod(sig) {
		assert(w.Version().Has(pkgbits.GenericMethods))
		w.Bool(true)
	} else {
		if w.Version().Has(pkgbits.GenericMethods) {
			w.Bool(false)
		}
		w.typ(sig)
	}

	w.pos(expr)
	w.selector(fun)

	// Method on a type parameter. These require an indirect call
	// through the current function's runtime dictionary.
	if typeParam, ok := types2.Unalias(recv).(*types2.TypeParam); w.Bool(ok) {
		typeParamIdx := w.dict.typeParamIndex(typeParam)
		methodInfo := w.p.selectorIdx(fun)

		w.Len(w.dict.typeParamMethodExprIdx(typeParamIdx, methodInfo))
		return
	}

	if isInterface(recv) != isInterface(sig.Recv().Type()) {
		w.p.fatalf(expr, "isInterface inconsistency: %v and %v", recv, sig.Recv().Type())
	}

	if isConcreteMethod(sig) {
		tname, tExplicits := splitNamed(types2.Unalias(deref2(recv)).(*types2.Named))
		var info objInfo
		if isGenericMethod(sig) {
			// For generic methods, the shaped object is the method itself.
			mExplicits := asTypeSlice(w.p.info.Instances[expr.Sel].TypeArgs)
			info = w.p.objInstIdx(fun.Origin(), slices.Concat(tExplicits, mExplicits), w.dict)
		} else {
			// For non-generic concrete methods on generic types, the shaped object
			// is the type. The method must be looked up on the type by name.
			info = w.p.objInstIdx(tname, tExplicits, w.dict)
		}
		// We don't know all of the type arguments statically. These can be
		// handled by a static call to the shaped method, but require
		// dynamically looking up the appropriate dictionary argument
		// in the current function's runtime dictionary.
		if info.anyDerived() {
			w.Bool(true) // dynamic subdictionary
			w.Len(w.dict.subdictIdx(info))
			return
		}
		// We know all of the type arguments statically. These can be handled
		// by a static call to the shaped method, and with a static reference
		// to either the receiver type's or method's dictionary (see above).
		if len(info.explicits) > 0 {
			w.Bool(false) // no dynamic subdictionary
			w.Bool(true)  // static dictionary
			w.objInfo(info)
			return
		}
		// no type arguments
	}

	w.Bool(false) // no dynamic subdictionary
	w.Bool(false) // no static dictionary
}

// multiExpr writes a sequence of expressions, where the i'th value is
// implicitly converted to dstType(i). It also handles when exprs is a
// single, multi-valued expression (e.g., the multi-valued argument in
// an f(g()) call, or the RHS operand in a comma-ok assignment).
func (w *writer) multiExpr(pos poser, dstType func(int) types2.Type, exprs []syntax.Expr) {
	w.Sync(pkgbits.SyncMultiExpr)

	if len(exprs) == 1 {
		expr := exprs[0]
		if tuple, ok := w.p.typeOf(expr).(*types2.Tuple); ok {
			assert(tuple.Len() > 1)
			w.Bool(true) // N:1 assignment
			w.pos(pos)
			w.expr(expr)

			w.Len(tuple.Len())
			for i := 0; i < tuple.Len(); i++ {
				src := tuple.At(i).Type()
				// TODO(mdempsky): Investigate not writing src here. I think
				// the reader should be able to infer it from expr anyway.
				w.typ(src)
				if dst := dstType(i); w.Bool(dst != nil && !types2.Identical(src, dst)) {
					if src == nil || dst == nil {
						w.p.fatalf(pos, "src is %v, dst is %v", src, dst)
					}
					if !types2.AssignableTo(src, dst) {
						w.p.fatalf(pos, "%v is not assignable to %v", src, dst)
					}
					w.typ(dst)
					w.convRTTI(src, dst)
				}
			}
			return
		}
	}

	w.Bool(false) // N:N assignment
	w.Len(len(exprs))
	for i, expr := range exprs {
		w.implicitConvExpr(dstType(i), expr)
	}
}

// implicitConvExpr is like expr, but if dst is non-nil and different
// from expr's type, then an implicit conversion operation is inserted
// at expr's position.
func (w *writer) implicitConvExpr(dst types2.Type, expr syntax.Expr) {
	w.convertExpr(dst, expr, true)
}

func (w *writer) convertExpr(dst types2.Type, expr syntax.Expr, implicit bool) {
	src := w.p.typeOf(expr)

	// Omit implicit no-op conversions.
	identical := dst == nil || types2.Identical(src, dst)
	if implicit && identical {
		w.expr(expr)
		return
	}

	if implicit && !types2.AssignableTo(src, dst) {
		w.p.fatalf(expr, "%v is not assignable to %v", src, dst)
	}

	w.Code(exprConvert)
	w.Bool(implicit)
	w.typ(dst)
	w.pos(expr)
	w.convRTTI(src, dst)
	w.Bool(isTypeParam(dst))
	w.Bool(identical)
	w.expr(expr)
}

func (w *writer) compLit(lit *syntax.CompositeLit) {
	typ := w.p.typeOf(lit)

	w.Sync(pkgbits.SyncCompLit)
	w.pos(lit)
	w.typ(typ)

	if ptr, ok := types2.CoreType(typ).(*types2.Pointer); ok {
		typ = ptr.Elem()
	}

	if w.Version().Has(pkgbits.CompactCompLiterals) {
		switch typ0 := typ; typ := types2.CoreType(typ).(type) {
		default:
			w.p.fatalf(lit, "unexpected composite literal type: %v", typ)
		case *types2.Array:
			w.arrayElems(typ.Elem(), lit.ElemList)
		case *types2.Map:
			w.rtype(typ0)
			w.mapElems(typ.Key(), typ.Elem(), lit.ElemList)
		case *types2.Slice:
			w.arrayElems(typ.Elem(), lit.ElemList)
		case *types2.Struct:
			w.structElems(typ, lit.NKeys == 0, lit.ElemList)
		}
		return
	}

	// old format
	var keyType, elemType types2.Type
	var structType *types2.Struct
	switch typ0 := typ; typ := types2.CoreType(typ).(type) {
	default:
		w.p.fatalf(lit, "unexpected composite literal type: %v", typ)
	case *types2.Array:
		elemType = typ.Elem()
	case *types2.Map:
		w.rtype(typ0)
		keyType, elemType = typ.Key(), typ.Elem()
	case *types2.Slice:
		elemType = typ.Elem()
	case *types2.Struct:
		structType = typ
	}

	w.Len(len(lit.ElemList))
	for i, elem := range lit.ElemList {
		elemType := elemType
		if structType != nil {
			if kv, ok := elem.(*syntax.KeyValueExpr); ok {
				// use position of expr.Key rather than of elem (which has position of ':')
				w.pos(kv.Key)
				i = fieldIndex(w.p.info, structType, kv.Key.(*syntax.Name))
				elem = kv.Value
			} else {
				w.pos(elem)
			}
			elemType = structType.Field(i).Type()
			w.Len(i)
		} else {
			if kv, ok := elem.(*syntax.KeyValueExpr); w.Bool(ok) {
				// use position of expr.Key rather than of elem (which has position of ':')
				w.pos(kv.Key)
				w.implicitConvExpr(keyType, kv.Key)
				elem = kv.Value
			}
		}
		w.implicitConvExpr(elemType, elem)
	}
}

func (w *writer) arrayElems(elemType types2.Type, elems []syntax.Expr) {
	valuesOnly := true
	for _, elem := range elems {
		if _, ok := elem.(*syntax.KeyValueExpr); ok {
			valuesOnly = false
			break
		}
	}

	if valuesOnly {
		w.Int(len(elems))
		for _, elem := range elems {
			w.implicitConvExpr(elemType, elem)
		}
		return
	}
	// some elements may have a key
	w.Int(-len(elems))
	for _, elem := range elems {
		if kv, ok := elem.(*syntax.KeyValueExpr); w.Bool(ok) {
			w.pos(kv.Key) // use position of Key rather than of elem (which has position of ':')
			w.implicitConvExpr(nil, kv.Key)
			elem = kv.Value
		}
		w.implicitConvExpr(elemType, elem)
	}
}

func (w *writer) mapElems(keyType, valueType types2.Type, elems []syntax.Expr) {
	// all elements have a key
	w.Int(-len(elems))
	for _, elem := range elems {
		kv := elem.(*syntax.KeyValueExpr)
		w.pos(kv.Key) // use position of Key rather than of elem (which has position of ':')
		w.implicitConvExpr(keyType, kv.Key)
		w.implicitConvExpr(valueType, kv.Value)
	}
}

func (w *writer) structElems(typ *types2.Struct, valuesOnly bool, elems []syntax.Expr) {
	n := len(elems)
	if valuesOnly {
		// no element has a key
		w.Int(n)
		for i, elem := range elems {
			w.pos(elem)
			w.implicitConvExpr(typ.Field(i).Type(), elem)
		}
		return
	}
	// all elements have a key
	w.Int(-n)
	for _, elem := range elems {
		kv := elem.(*syntax.KeyValueExpr)
		w.pos(kv.Key) // use position of Key rather than of elem (which has position of ':')
		// TODO(gri): rather than doing this lookup again, perhaps the index should be recorded by types2
		fld, index, _ := types2.LookupFieldOrMethod(typ, false, w.p.curpkg, kv.Key.(*syntax.Name).Value)
		if n := len(index); n > 1 {
			// embedded field
			w.Int(-n)
			for _, i := range index {
				w.Int(i)
			}
		} else { // n == 1
			w.Int(index[0])
		}
		w.implicitConvExpr(fld.Type(), kv.Value)
	}
}

func (w *writer) funcLit(expr *syntax.FuncLit) {
	sig := w.p.typeOf(expr).(*types2.Signature)

	body, closureVars := w.p.bodyIdx(sig, expr.Body, w.dict)

	w.Sync(pkgbits.SyncFuncLit)
	w.pos(expr)
	w.signature(sig)
	w.Bool(w.p.rangeFuncBodyClosures[expr])

	w.Len(len(closureVars))
	for _, cv := range closureVars {
		w.pos(cv.pos)
		w.useLocal(cv.pos, cv.var_)
	}

	w.Reloc(pkgbits.SectionBody, body)
}

type posVar struct {
	pos  syntax.Pos
	var_ *types2.Var
}

func (p posVar) String() string {
	return p.pos.String() + ":" + p.var_.String()
}

func (w *writer) exprs(exprs []syntax.Expr) {
	w.Sync(pkgbits.SyncExprs)
	w.Len(len(exprs))
	for _, expr := range exprs {
		w.expr(expr)
	}
}

// rtype writes information so that the reader can construct an
// expression of type *runtime._type representing typ.
func (w *writer) rtype(typ types2.Type) {
	typ = types2.Default(typ)

	info := w.p.typIdx(typ, w.dict)
	w.rtypeInfo(info)
}

func (w *writer) rtypeInfo(info typeInfo) {
	w.Sync(pkgbits.SyncRType)

	if w.Bool(info.derived) {
		w.Len(w.dict.rtypeIdx(info))
	} else {
		w.typInfo(info)
	}
}

// varDictIndex writes out information for populating DictIndex for
// the ir.Name that will represent obj.
func (w *writer) varDictIndex(obj *types2.Var) {
	info := w.p.typIdx(obj.Type(), w.dict)
	if w.Bool(info.derived) {
		w.Len(w.dict.rtypeIdx(info))
	}
}

// isUntyped reports whether typ is an untyped type.
func isUntyped(typ types2.Type) bool {
	// Note: types2.Unalias is unnecessary here, since untyped types can't be aliased.
	basic, ok := typ.(*types2.Basic)
	return ok && basic.Info()&types2.IsUntyped != 0
}

// isTuple reports whether typ is a tuple type.
func isTuple(typ types2.Type) bool {
	// Note: types2.Unalias is unnecessary here, since tuple types can't be aliased.
	_, ok := typ.(*types2.Tuple)
	return ok
}

func (w *writer) itab(typ, iface types2.Type) {
	typ = types2.Default(typ)
	iface = types2.Default(iface)

	typInfo := w.p.typIdx(typ, w.dict)
	ifaceInfo := w.p.typIdx(iface, w.dict)

	w.rtypeInfo(typInfo)
	w.rtypeInfo(ifaceInfo)
	if w.Bool(typInfo.derived || ifaceInfo.derived) {
		w.Len(w.dict.itabIdx(typInfo, ifaceInfo))
	}
}

// convRTTI writes information so that the reader can construct
// expressions for converting from src to dst.
func (w *writer) convRTTI(src, dst types2.Type) {
	w.Sync(pkgbits.SyncConvRTTI)
	w.itab(src, dst)
}

func (w *writer) exprType(iface types2.Type, typ syntax.Expr) {
	base.Assertf(iface == nil || isInterface(iface), "%v must be nil or an interface type", iface)

	tv := w.p.typeAndValue(typ)
	assert(tv.IsType())

	w.Sync(pkgbits.SyncExprType)
	w.pos(typ)

	if w.Bool(iface != nil && !iface.Underlying().(*types2.Interface).Empty()) {
		w.itab(tv.Type, iface)
	} else {
		w.rtype(tv.Type)

		info := w.p.typIdx(tv.Type, w.dict)
		w.Bool(info.derived)
	}
}

// isInterface reports whether typ is known to be an interface type.
// If typ is a type parameter, then isInterface reports an internal
// compiler error instead.
func isInterface(typ types2.Type) bool {
	if _, ok := types2.Unalias(typ).(*types2.TypeParam); ok {
		// typ is a type parameter and may be instantiated as either a
		// concrete or interface type, so the writer can't depend on
		// knowing this.
		base.Fatalf("%v is a type parameter", typ)
	}

	_, ok := typ.Underlying().(*types2.Interface)
	return ok
}

// isConcreteMethod reports whether typ is a concrete method. That is,
// it's a method with a receiver that isn't an interface type.
func isConcreteMethod(typ types2.Type) bool {
	sig, ok := typ.(*types2.Signature)
	return ok && sig.Recv() != nil && !isInterface(sig.Recv().Type())
}

// TODO(mark): Use isGenericMethod. It is included now to help justify
// the existence of isConcreteMethod.

// isGenericMethod reports whether typ is a generic method. That is,
// it's a method with type parameters apart from those which may or
// may not appear on the receiver type.
//
// Note that generic methods are always concrete methods.
func isGenericMethod(typ types2.Type) bool {
	sig, ok := typ.(*types2.Signature)
	return ok && sig.Recv() != nil && sig.TypeParams().Len() > 0
}

// op writes an Op into the bitstream.
func (w *writer) op(op ir.Op) {
	// TODO(mdempsky): Remove in favor of explicit codes? Would make
	// export data more stable against internal refactorings, but low
	// priority at the moment.
	assert(op != 0)
	w.Sync(pkgbits.SyncOp)
	w.Len(int(op))
}

// @@@ Package initialization

// Caution: This code is still clumsy, because toolstash -cmp is
// particularly sensitive to it.

type typeDeclGen struct {
	*syntax.TypeDecl
	gen int

	// Implicit type parameters in scope at this type declaration.
	implicits []*types2.TypeParam
}

type fileImports struct {
	importedEmbed, importedUnsafe bool
}

// declCollector is a visitor type that collects compiler-needed
// information about declarations that types2 doesn't track.
//
// Notably, it maps declared types and functions back to their
// declaration statement, keeps track of implicit type parameters, and
// assigns unique type "generation" numbers to local defined types.
type declCollector struct {
	pw         *pkgWriter
	typegen    *int
	file       *fileImports
	withinFunc bool
	implicits  []*types2.TypeParam
}

func (c *declCollector) withTParams(obj types2.Object) *declCollector {
	tparams := slices.Concat(objRecvTypeParams(obj), objTypeParams(obj))
	if len(tparams) == 0 {
		return c
	}

	copy := *c
	copy.implicits = copy.implicits[:len(copy.implicits):len(copy.implicits)]
	for _, tparam := range tparams {
		copy.implicits = append(copy.implicits, tparam)
	}
	return &copy
}

func (c *declCollector) Visit(n syntax.Node) syntax.Visitor {
	pw := c.pw

	switch n := n.(type) {
	case *syntax.File:
		pw.checkPragmas(n.Pragma, ir.GoBuildPragma, false)

	case *syntax.ImportDecl:
		pw.checkPragmas(n.Pragma, 0, false)

		switch pw.info.PkgNameOf(n).Imported().Path() {
		case "embed":
			c.file.importedEmbed = true
		case "unsafe":
			c.file.importedUnsafe = true
		}

	case *syntax.ConstDecl:
		pw.checkPragmas(n.Pragma, 0, false)

	case *syntax.FuncDecl:
		pw.checkPragmas(n.Pragma, funcPragmas, false)

		obj := pw.info.Defs[n.Name].(*types2.Func)
		pw.funDecls[obj] = n

		return c.withTParams(obj)

	case *syntax.TypeDecl:
		obj := pw.info.Defs[n.Name].(*types2.TypeName)
		d := typeDeclGen{TypeDecl: n, implicits: c.implicits}

		if n.Alias {
			pw.checkPragmas(n.Pragma, 0, false)
		} else {
			pw.checkPragmas(n.Pragma, 0, false)

			// Assign a unique ID to function-scoped defined types.
			if c.withinFunc {
				*c.typegen++
				d.gen = *c.typegen
			}
		}

		pw.typDecls[obj] = d

		// TODO(mdempsky): Omit? Not strictly necessary; only matters for
		// type declarations within function literals within parameterized
		// type declarations, but types2 the function literals will be
		// constant folded away.
		return c.withTParams(obj)

	case *syntax.VarDecl:
		pw.checkPragmas(n.Pragma, 0, true)

		if p, ok := n.Pragma.(*pragmas); ok && len(p.Embeds) > 0 {
			if err := checkEmbed(n, c.file.importedEmbed, c.withinFunc); err != nil {
				pw.errorf(p.Embeds[0].Pos, "%s", err)
			}
		}

	case *syntax.BlockStmt:
		if !c.withinFunc {
			copy := *c
			copy.withinFunc = true
			return &copy
		}
	}

	return c
}

func (pw *pkgWriter) collectDecls(noders []*noder) {
	var typegen int
	for _, p := range noders {
		var file fileImports

		syntax.Walk(p.file, &declCollector{
			pw:      pw,
			typegen: &typegen,
			file:    &file,
		})

		pw.cgoPragmas = append(pw.cgoPragmas, p.pragcgobuf...)

		for _, l := range p.linknames {
			directive := "go:linkname"
			if l.std {
				directive = "go:linknamestd"
			}
			if !file.importedUnsafe {
				pw.errorf(l.pos, "//%s only allowed in Go files that import \"unsafe\"", directive)
				continue
			}
			if strings.Contains(l.remote, "[") && strings.Contains(l.remote, "]") {
				pw.errorf(l.pos, "//%s reference of an instantiation is not allowed", directive)
				continue
			}

			switch obj := pw.curpkg.Scope().Lookup(l.local).(type) {
			case *types2.Func, *types2.Var:
				if _, ok := pw.linknames[obj]; !ok {
					pw.linknames[obj] = struct {
						remote string
						std    bool
					}{l.remote, l.std}
				} else {
					pw.errorf(l.pos, "duplicate //%s for %s", directive, l.local)
				}

			default:
				if types.AllowsGoVersion(1, 18) {
					pw.errorf(l.pos, "//%s must refer to declared function or variable", directive)
				}
			}
		}
	}
}

func (pw *pkgWriter) checkPragmas(p syntax.Pragma, allowed ir.PragmaFlag, embedOK bool) {
	if p == nil {
		return
	}
	pragma := p.(*pragmas)

	for _, pos := range pragma.Pos {
		if pos.Flag&^allowed != 0 {
			pw.errorf(pos.Pos, "misplaced compiler directive")
		}
	}

	if !embedOK {
		for _, e := range pragma.Embeds {
			pw.errorf(e.Pos, "misplaced go:embed directive")
		}
	}
}

func (w *writer) pkgInit(noders []*noder) {
	w.Len(len(w.p.cgoPragmas))
	for _, cgoPragma := range w.p.cgoPragmas {
		w.Strings(cgoPragma)
	}

	w.pkgInitOrder()

	w.Sync(pkgbits.SyncDecls)
	for _, p := range noders {
		for _, decl := range p.file.DeclList {
			w.pkgDecl(decl)
		}
	}
	w.Code(declEnd)

	w.Sync(pkgbits.SyncEOF)
}

func (w *writer) pkgInitOrder() {
	// TODO(mdempsky): Write as a function body instead?
	w.Len(len(w.p.info.InitOrder))
	for _, init := range w.p.info.InitOrder {
		w.Len(len(init.Lhs))
		for _, v := range init.Lhs {
			w.obj(v, nil)
		}
		w.expr(init.Rhs)
	}
}

func (w *writer) pkgDecl(decl syntax.Decl) {
	switch decl := decl.(type) {
	default:
		w.p.unexpected("declaration", decl)

	case *syntax.ImportDecl:

	case *syntax.ConstDecl:
		w.Code(declOther)
		w.pkgObjs(decl.NameList...)

	case *syntax.FuncDecl:
		if decl.Name.Value == "_" {
			break // skip blank functions
		}

		obj := w.p.info.Defs[decl.Name].(*types2.Func)
		sig := obj.Type().(*types2.Signature)

		if sig.RecvTypeParams() != nil || sig.TypeParams() != nil {
			break // skip generic functions
		}

		if recv := sig.Recv(); recv != nil {
			w.Code(declMethod)
			w.typ(recvBase(recv))
			w.selector(obj)
			break
		}

		w.Code(declFunc)
		w.pkgObjs(decl.Name)

	case *syntax.TypeDecl:
		if len(decl.TParamList) != 0 {
			break // skip generic type decls
		}

		if decl.Name.Value == "_" {
			break // skip blank type decls
		}

		name := w.p.info.Defs[decl.Name].(*types2.TypeName)
		// Skip type declarations for interfaces that are only usable as
		// type parameter bounds.
		if iface, ok := name.Type().Underlying().(*types2.Interface); ok && !iface.IsMethodSet() {
			break
		}

		w.Code(declOther)
		w.pkgObjs(decl.Name)

	case *syntax.VarDecl:
		w.Code(declVar)
		w.pkgObjs(decl.NameList...)

		var embeds []pragmaEmbed
		if p, ok := decl.Pragma.(*pragmas); ok {
			embeds = p.Embeds
		}
		w.Len(len(embeds))
		for _, embed := range embeds {
			w.pos(embed.Pos)
			w.Strings(embed.Patterns)
		}
	}
}

func (w *writer) pkgObjs(names ...*syntax.Name) {
	w.Sync(pkgbits.SyncDeclNames)
	w.Len(len(names))

	for _, name := range names {
		obj, ok := w.p.info.Defs[name]
		assert(ok)

		w.Sync(pkgbits.SyncDeclName)
		w.obj(obj, nil)
	}
}

// @@@ Helpers

// staticBool analyzes a boolean expression and reports whether it's
// always true (positive result), always false (negative result), or
// unknown (zero).
//
// It also simplifies the expression while preserving semantics, if
// possible.
func (pw *pkgWriter) staticBool(ep *syntax.Expr) int {
	if val := pw.typeAndValue(*ep).Value; val != nil {
		if constant.BoolVal(val) {
			return +1
		} else {
			return -1
		}
	}

	if e, ok := (*ep).(*syntax.Operation); ok {
		switch e.Op {
		case syntax.Not:
			return pw.staticBool(&e.X)

		case syntax.AndAnd:
			x := pw.staticBool(&e.X)
			if x < 0 {
				*ep = e.X
				return x
			}

			y := pw.staticBool(&e.Y)
			if x > 0 || y < 0 {
				if pw.typeAndValue(e.X).Value != nil {
					*ep = e.Y
				}
				return y
			}

		case syntax.OrOr:
			x := pw.staticBool(&e.X)
			if x > 0 {
				*ep = e.X
				return x
			}

			y := pw.staticBool(&e.Y)
			if x < 0 || y > 0 {
				if pw.typeAndValue(e.X).Value != nil {
					*ep = e.Y
				}
				return y
			}
		}
	}

	return 0
}

// hasImplicitTypeParams reports whether obj is a defined type with
// implicit type parameters (e.g., declared within a generic function
// or method).
func (pw *pkgWriter) hasImplicitTypeParams(obj *types2.TypeName) bool {
	if obj.Pkg() == pw.curpkg {
		decl, ok := pw.typDecls[obj]
		assert(ok)
		if len(decl.implicits) != 0 {
			return true
		}
	}
	return false
}

// isDefinedType reports whether obj is a defined type.
func isDefinedType(obj types2.Object) bool {
	if obj, ok := obj.(*types2.TypeName); ok {
		return !obj.IsAlias()
	}
	return false
}

// isGlobal reports whether obj was declared at package scope.
//
// Caveat: blank objects are not declared.
func isGlobal(obj types2.Object) bool {
	return obj.Parent() == obj.Pkg().Scope()
}

// lookupObj returns the object that expr refers to, if any. If expr
// is an explicit instantiation of a generic object, then the instance
// object is returned as well.
func lookupObj(p *pkgWriter, expr syntax.Expr) (obj types2.Object, inst types2.Instance) {
	if index, ok := expr.(*syntax.IndexExpr); ok {
		args := syntax.UnpackListExpr(index.Index)
		if len(args) == 1 {
			tv := p.typeAndValue(args[0])
			if tv.IsValue() {
				return // normal index expression
			}
		}

		expr = index.X
	}

	// Strip package qualifier, if present.
	if sel, ok := expr.(*syntax.SelectorExpr); ok {
		if !isPkgQual(p.info, sel) {
			return // normal selector expression
		}
		expr = sel.Sel
	}

	if name, ok := expr.(*syntax.Name); ok {
		obj = p.info.Uses[name]
		inst = p.info.Instances[name]
	}
	return
}

// isPkgQual reports whether the given selector expression is a
// package-qualified identifier.
func isPkgQual(info *types2.Info, sel *syntax.SelectorExpr) bool {
	if name, ok := sel.X.(*syntax.Name); ok {
		_, isPkgName := info.Uses[name].(*types2.PkgName)
		return isPkgName
	}
	return false
}

// isNil reports whether expr is a (possibly parenthesized) reference
// to the predeclared nil value.
func isNil(p *pkgWriter, expr syntax.Expr) bool {
	tv := p.typeAndValue(expr)
	return tv.IsNil()
}

// isBuiltin reports whether expr is a (possibly parenthesized)
// referenced to the specified built-in function.
func (pw *pkgWriter) isBuiltin(expr syntax.Expr, builtin string) bool {
	if name, ok := syntax.Unparen(expr).(*syntax.Name); ok && name.Value == builtin {
		return pw.typeAndValue(name).IsBuiltin()
	}
	return false
}

// recvBase returns the base type for the given receiver parameter.
func recvBase(recv *types2.Var) *types2.Named {
	typ := types2.Unalias(recv.Type())
	if ptr, ok := typ.(*types2.Pointer); ok {
		typ = types2.Unalias(ptr.Elem())
	}
	return typ.(*types2.Named)
}

// namesAsExpr returns a list of names as a syntax.Expr.
func namesAsExpr(names []*syntax.Name) syntax.Expr {
	if len(names) == 1 {
		return names[0]
	}

	exprs := make([]syntax.Expr, len(names))
	for i, name := range names {
		exprs[i] = name
	}
	return &syntax.ListExpr{ElemList: exprs}
}

// fieldIndex returns the index of the struct field named by key.
func fieldIndex(info *types2.Info, str *types2.Struct, key *syntax.Name) int {
	field := info.Uses[key].(*types2.Var)

	for i := 0; i < str.NumFields(); i++ {
		if str.Field(i) == field {
			return i
		}
	}

	panic(fmt.Sprintf("%s: %v is not a field of %v", key.Pos(), field, str))
}

// objRecvTypeParams returns the receiver type parameters on the given object.
func objRecvTypeParams(obj types2.Object) []*types2.TypeParam {
	if f, ok := obj.(*types2.Func); ok {
		return asTypeParamSlice(f.Signature().RecvTypeParams())
	}
	return nil
}

// objTypeParams returns the type parameters on the given object.
func objTypeParams(obj types2.Object) []*types2.TypeParam {
	switch t := obj.(type) {
	case *types2.Func:
		return asTypeParamSlice(t.Signature().TypeParams())
	case *types2.TypeName:
		switch t := obj.Type().(type) {
		case *types2.Named:
			return asTypeParamSlice(t.TypeParams())
		case *types2.Alias:
			return asTypeParamSlice(t.TypeParams())
		}
	}
	return nil
}

// asTypeParamSlice unpacks a types2.TypeParamList to a []types2.TypeParam
func asTypeParamSlice(l *types2.TypeParamList) []*types2.TypeParam {
	if l.Len() == 0 {
		return nil
	}
	s := make([]*types2.TypeParam, l.Len())
	for i := range l.Len() {
		s[i] = l.At(i)
	}
	return s
}

// splitNamed decomposes a use of a defined type into its original
// type definition and the type arguments used to instantiate it.
func splitNamed(typ *types2.Named) (*types2.TypeName, []types2.Type) {
	base.Assertf(typ.TypeParams().Len() == typ.TypeArgs().Len(), "use of uninstantiated type: %v", typ)

	orig := typ.Origin()
	base.Assertf(orig.TypeArgs() == nil, "origin %v of %v has type arguments", orig, typ)
	base.Assertf(typ.Obj() == orig.Obj(), "%v has object %v, but %v has object %v", typ, typ.Obj(), orig, orig.Obj())

	return typ.Obj(), asTypeSlice(typ.TypeArgs())
}

// splitAlias is like splitNamed, but for an alias type.
func splitAlias(typ *types2.Alias) (*types2.TypeName, []types2.Type) {
	orig := typ.Origin()
	base.Assertf(typ.Obj() == orig.Obj(), "alias type %v has object %v, but %v has object %v", typ, typ.Obj(), orig, orig.Obj())

	return typ.Obj(), asTypeSlice(typ.TypeArgs())
}

// asTypeSlice unpacks a types2.TypeList to a []types2.Type
func asTypeSlice(l *types2.TypeList) []types2.Type {
	if l.Len() == 0 {
		return nil
	}
	s := make([]types2.Type, l.Len())
	for i := range l.Len() {
		s[i] = l.At(i)
	}
	return s
}

func asPragmaFlag(p syntax.Pragma) ir.PragmaFlag {
	if p == nil {
		return 0
	}
	return p.(*pragmas).Flag
}

func asWasmImport(p syntax.Pragma) *WasmImport {
	if p == nil {
		return nil
	}
	return p.(*pragmas).WasmImport
}

func asWasmExport(p syntax.Pragma) *WasmExport {
	if p == nil {
		return nil
	}
	return p.(*pragmas).WasmExport
}

// isPtrTo reports whether from is the type *to.
func isPtrTo(from, to types2.Type) bool {
	ptr, ok := types2.Unalias(from).(*types2.Pointer)
	return ok && types2.Identical(ptr.Elem(), to)
}

// hasFallthrough reports whether stmts ends in a fallthrough
// statement.
func hasFallthrough(stmts []syntax.Stmt) bool {
	// From spec: the last non-empty statement may be a (possibly labeled) "fallthrough" statement
	// Stripping (possible nested) labeled statement if any.
	stmt := lastNonEmptyStmt(stmts)
	for {
		ls, ok := stmt.(*syntax.LabeledStmt)
		if !ok {
			break
		}
		stmt = ls.Stmt
	}
	last, ok := stmt.(*syntax.BranchStmt)
	return ok && last.Tok == syntax.Fallthrough
}

// lastNonEmptyStmt returns the last non-empty statement in list, if
// any.
func lastNonEmptyStmt(stmts []syntax.Stmt) syntax.Stmt {
	for i := len(stmts) - 1; i >= 0; i-- {
		stmt := stmts[i]
		if _, ok := stmt.(*syntax.EmptyStmt); !ok {
			return stmt
		}
	}
	return nil
}

// terminates reports whether stmt terminates normal control flow
// (i.e., does not merely advance to the following statement).
func (pw *pkgWriter) terminates(stmt syntax.Stmt) bool {
	switch stmt := stmt.(type) {
	case *syntax.BranchStmt:
		if stmt.Tok == syntax.Goto {
			return true
		}
	case *syntax.ReturnStmt:
		return true
	case *syntax.ExprStmt:
		if call, ok := syntax.Unparen(stmt.X).(*syntax.CallExpr); ok {
			if pw.isBuiltin(call.Fun, "panic") {
				return true
			}
		}

		// The handling of BlockStmt here is approximate, but it serves to
		// allow dead-code elimination for:
		//
		//	if true {
		//		return x
		//	}
		//	unreachable
	case *syntax.IfStmt:
		cond := pw.staticBool(&stmt.Cond)
		return (cond < 0 || pw.terminates(stmt.Then)) && (cond > 0 || pw.terminates(stmt.Else))
	case *syntax.BlockStmt:
		return pw.terminates(lastNonEmptyStmt(stmt.List))
	}

	return false
}

```

