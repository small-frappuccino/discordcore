# Domain Architecture: cmd/cgo

## Layout Topology
```text
cmd/cgo/
├── internal
│   ├── cgotest
│   │   └── overlaydir.go
│   ├── swig
│   ├── testcarchive
│   ├── testcshared
│   ├── testerrors
│   ├── testfortran
│   ├── testgodefs
│   ├── testlife
│   ├── testnocgo
│   │   └── nocgo.go
│   ├── testout
│   ├── testplugin
│   │   └── altpath
│   ├── testsanitizers
│   ├── testshared
│   ├── testso
│   ├── teststdio
│   └── testtls
│       ├── tls.c
│       ├── tls.go
│       └── tls_none.go
├── ast.go
├── doc.go
├── gcc.go
├── godefs.go
├── main.go
├── out.go
└── util.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/cmd/cgo/ast.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse input AST and prepare Prog structure.

package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"strings"
)

func parse(name string, src []byte, flags parser.Mode) *ast.File {
	ast1, err := parser.ParseFile(fset, name, src, flags)
	if err != nil {
		if list, ok := err.(scanner.ErrorList); ok {
			// If err is a scanner.ErrorList, its String will print just
			// the first error and then (+n more errors).
			// Instead, turn it into a new Error that will return
			// details for all the errors.
			for _, e := range list {
				fmt.Fprintln(os.Stderr, e)
			}
			os.Exit(2)
		}
		fatalf("parsing %s: %s", name, err)
	}
	return ast1
}

func sourceLine(n ast.Node) int {
	return fset.Position(n.Pos()).Line
}

// ParseGo populates f with information learned from the Go source code
// which was read from the named file. It gathers the C preamble
// attached to the import "C" comment, a list of references to C.xxx,
// a list of exported functions, and the actual AST, to be rewritten and
// printed.
func (f *File) ParseGo(abspath string, src []byte) {
	// Two different parses: once with comments, once without.
	// The printer is not good enough at printing comments in the
	// right place when we start editing the AST behind its back,
	// so we use ast1 to look for the doc comments on import "C"
	// and on exported functions, and we use ast2 for translating
	// and reprinting.
	// In cgo mode, we ignore ast2 and just apply edits directly
	// the text behind ast1. In godefs mode we modify and print ast2.
	ast1 := parse(abspath, src, parser.SkipObjectResolution|parser.ParseComments)
	ast2 := parse(abspath, src, parser.SkipObjectResolution)

	f.Package = ast1.Name.Name
	f.Name = make(map[string]*Name)
	f.NamePos = make(map[*Name]token.Pos)

	// In ast1, find the import "C" line and get any extra C preamble.
	sawC := false
	for _, decl := range ast1.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				s, ok := spec.(*ast.ImportSpec)
				if !ok || s.Path.Value != `"C"` {
					continue
				}
				sawC = true
				if s.Name != nil {
					error_(s.Path.Pos(), `cannot rename import "C"`)
				}
				cg := s.Doc
				if cg == nil && len(decl.Specs) == 1 {
					cg = decl.Doc
				}
				if cg != nil {
					if strings.ContainsAny(abspath, "\r\n") {
						// This should have been checked when the file path was first resolved,
						// but we double check here just to be sure.
						fatalf("internal error: ParseGo: abspath contains unexpected newline character: %q", abspath)
					}
					f.Preamble += fmt.Sprintf("#line %d %q\n", sourceLine(cg), abspath)
					f.Preamble += commentText(cg) + "\n"
					f.Preamble += "#line 1 \"cgo-generated-wrapper\"\n"
				}
			}

		case *ast.FuncDecl:
			// Also, reject attempts to declare methods on C.T or *C.T.
			// (The generated code would otherwise accept this
			// invalid input; see issue #57926.)
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				recvType := decl.Recv.List[0].Type
				if recvType != nil {
					t := recvType
					if star, ok := unparen(t).(*ast.StarExpr); ok {
						t = star.X
					}
					if sel, ok := unparen(t).(*ast.SelectorExpr); ok {
						var buf strings.Builder
						format.Node(&buf, fset, recvType)
						error_(sel.Pos(), `cannot define new methods on non-local type %s`, &buf)
					}
				}
			}
		}

	}
	if !sawC {
		error_(ast1.Package, `cannot find import "C"`)
	}

	// In ast2, strip the import "C" line.
	if *godefs {
		w := 0
		for _, decl := range ast2.Decls {
			d, ok := decl.(*ast.GenDecl)
			if !ok {
				ast2.Decls[w] = decl
				w++
				continue
			}
			ws := 0
			for _, spec := range d.Specs {
				s, ok := spec.(*ast.ImportSpec)
				if !ok || s.Path.Value != `"C"` {
					d.Specs[ws] = spec
					ws++
				}
			}
			if ws == 0 {
				continue
			}
			d.Specs = d.Specs[0:ws]
			ast2.Decls[w] = d
			w++
		}
		ast2.Decls = ast2.Decls[0:w]
	} else {
		for _, decl := range ast2.Decls {
			d, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range d.Specs {
				if s, ok := spec.(*ast.ImportSpec); ok && s.Path.Value == `"C"` {
					// Replace "C" with _ "unsafe", to keep program valid.
					// (Deleting import statement or clause is not safe if it is followed
					// in the source by an explicit semicolon.)
					f.Edit.Replace(f.offset(s.Path.Pos()), f.offset(s.Path.End()), `_ "unsafe"`)
				}
			}
		}
	}

	// Accumulate pointers to uses of C.x.
	if f.Ref == nil {
		f.Ref = make([]*Ref, 0, 8)
	}
	f.walk(ast2, ctxProg, (*File).validateIdents)
	f.walk(ast2, ctxProg, (*File).saveExprs)

	// Accumulate exported functions.
	// The comments are only on ast1 but we need to
	// save the function bodies from ast2.
	// The first walk fills in ExpFunc, and the
	// second walk changes the entries to
	// refer to ast2 instead.
	f.walk(ast1, ctxProg, (*File).saveExport)
	f.walk(ast2, ctxProg, (*File).saveExport2)

	f.Comments = ast1.Comments
	f.AST = ast2
}

// Like ast.CommentGroup's Text method but preserves
// leading blank lines, so that line numbers line up.
func commentText(g *ast.CommentGroup) string {
	pieces := make([]string, 0, len(g.List))
	for _, com := range g.List {
		c := com.Text
		// Remove comment markers.
		// The parser has given us exactly the comment text.
		switch c[1] {
		case '/':
			//-style comment (no newline at the end)
			c = c[2:] + "\n"
		case '*':
			/*-style comment */
			c = c[2 : len(c)-2]
		}
		pieces = append(pieces, c)
	}
	return strings.Join(pieces, "")
}

func (f *File) validateIdents(x any, context astContext) {
	if x, ok := x.(*ast.Ident); ok {
		if f.isMangledName(x.Name) {
			error_(x.Pos(), "identifier %q may conflict with identifiers generated by cgo", x.Name)
		}
	}
}

// Save various references we are going to need later.
func (f *File) saveExprs(x any, context astContext) {
	switch x := x.(type) {
	case *ast.Expr:
		switch (*x).(type) {
		case *ast.SelectorExpr:
			f.saveRef(x, context)
		}
	case *ast.CallExpr:
		f.saveCall(x, context)
	}
}

// Save references to C.xxx for later processing.
func (f *File) saveRef(n *ast.Expr, context astContext) {
	sel := (*n).(*ast.SelectorExpr)
	// For now, assume that the only instance of capital C is when
	// used as the imported package identifier.
	// The parser should take care of scoping in the future, so
	// that we will be able to distinguish a "top-level C" from a
	// local C.
	if l, ok := sel.X.(*ast.Ident); !ok || l.Name != "C" {
		return
	}
	if context == ctxAssign2 {
		context = ctxExpr
	}
	if context == ctxEmbedType {
		error_(sel.Pos(), "cannot embed C type")
	}
	goname := sel.Sel.Name
	if goname == "errno" {
		error_(sel.Pos(), "cannot refer to errno directly; see documentation")
		return
	}
	if goname == "_CMalloc" {
		error_(sel.Pos(), "cannot refer to C._CMalloc; use C.malloc")
		return
	}
	if goname == "malloc" {
		goname = "_CMalloc"
	}
	name := f.Name[goname]
	if name == nil {
		name = &Name{
			Go: goname,
		}
		f.Name[goname] = name
		f.NamePos[name] = sel.Pos()
	}
	f.Ref = append(f.Ref, &Ref{
		Name:    name,
		Expr:    n,
		Context: context,
	})
}

// Save calls to C.xxx for later processing.
func (f *File) saveCall(call *ast.CallExpr, context astContext) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	if l, ok := sel.X.(*ast.Ident); !ok || l.Name != "C" {
		return
	}
	c := &Call{Call: call, Deferred: context == ctxDefer}
	f.Calls = append(f.Calls, c)
}

// If a function should be exported add it to ExpFunc.
func (f *File) saveExport(x any, context astContext) {
	n, ok := x.(*ast.FuncDecl)
	if !ok {
		return
	}

	if n.Doc == nil {
		return
	}
	for _, c := range n.Doc.List {
		if !strings.HasPrefix(c.Text, "//export ") {
			continue
		}

		name := strings.TrimSpace(c.Text[9:])
		if name == "" {
			error_(c.Pos(), "export missing name")
		}

		if name != n.Name.Name {
			error_(c.Pos(), "export comment has wrong name %q, want %q", name, n.Name.Name)
		}

		f.ExpFunc = append(f.ExpFunc, &ExpFunc{
			Func:    n,
			ExpName: name,
			// Caution: Do not set the Doc field on purpose
			// to ensure that there are no unintended artifacts
			// in the binary. See https://go.dev/issue/76697.
		})
		break
	}
}

// Make f.ExpFunc[i] point at the Func from this AST instead of the other one.
func (f *File) saveExport2(x any, context astContext) {
	n, ok := x.(*ast.FuncDecl)
	if !ok {
		return
	}

	for _, exp := range f.ExpFunc {
		if exp.Func.Name.Name == n.Name.Name {
			exp.Func = n
			break
		}
	}
}

type astContext int

const (
	ctxProg astContext = iota
	ctxEmbedType
	ctxType
	ctxStmt
	ctxExpr
	ctxField
	ctxParam
	ctxAssign2 // assignment of a single expression to two variables
	ctxSwitch
	ctxTypeSwitch
	ctxFile
	ctxDecl
	ctxSpec
	ctxDefer
	ctxCall  // any function call other than ctxCall2
	ctxCall2 // function call whose result is assigned to two variables
	ctxSelector
)

// walk walks the AST x, calling visit(f, x, context) for each node.
func (f *File) walk(x any, context astContext, visit func(*File, any, astContext)) {
	visit(f, x, context)
	switch n := x.(type) {
	case *ast.Expr:
		f.walk(*n, context, visit)

	// everything else just recurs
	default:
		error_(token.NoPos, "unexpected type %T in walk", x)
		panic("unexpected type")

	case nil:

	// These are ordered and grouped to match ../../go/ast/ast.go
	case *ast.Field:
		if len(n.Names) == 0 && context == ctxField {
			f.walk(&n.Type, ctxEmbedType, visit)
		} else {
			f.walk(&n.Type, ctxType, visit)
		}
	case *ast.FieldList:
		for _, field := range n.List {
			f.walk(field, context, visit)
		}
	case *ast.BadExpr:
	case *ast.Ident:
	case *ast.Ellipsis:
		f.walk(&n.Elt, ctxType, visit)
	case *ast.BasicLit:
	case *ast.FuncLit:
		f.walk(n.Type, ctxType, visit)
		f.walk(n.Body, ctxStmt, visit)
	case *ast.CompositeLit:
		f.walk(&n.Type, ctxType, visit)
		f.walk(n.Elts, ctxExpr, visit)
	case *ast.ParenExpr:
		f.walk(&n.X, context, visit)
	case *ast.SelectorExpr:
		f.walk(&n.X, ctxSelector, visit)
	case *ast.IndexExpr:
		f.walk(&n.X, ctxExpr, visit)
		f.walk(&n.Index, ctxExpr, visit)
	case *ast.IndexListExpr:
		f.walk(&n.X, ctxExpr, visit)
		f.walk(n.Indices, ctxExpr, visit)
	case *ast.SliceExpr:
		f.walk(&n.X, ctxExpr, visit)
		if n.Low != nil {
			f.walk(&n.Low, ctxExpr, visit)
		}
		if n.High != nil {
			f.walk(&n.High, ctxExpr, visit)
		}
		if n.Max != nil {
			f.walk(&n.Max, ctxExpr, visit)
		}
	case *ast.TypeAssertExpr:
		f.walk(&n.X, ctxExpr, visit)
		f.walk(&n.Type, ctxType, visit)
	case *ast.CallExpr:
		if context == ctxAssign2 {
			f.walk(&n.Fun, ctxCall2, visit)
		} else {
			f.walk(&n.Fun, ctxCall, visit)
		}
		f.walk(n.Args, ctxExpr, visit)
	case *ast.StarExpr:
		f.walk(&n.X, context, visit)
	case *ast.UnaryExpr:
		f.walk(&n.X, ctxExpr, visit)
	case *ast.BinaryExpr:
		f.walk(&n.X, ctxExpr, visit)
		f.walk(&n.Y, ctxExpr, visit)
	case *ast.KeyValueExpr:
		f.walk(&n.Key, ctxExpr, visit)
		f.walk(&n.Value, ctxExpr, visit)

	case *ast.ArrayType:
		f.walk(&n.Len, ctxExpr, visit)
		f.walk(&n.Elt, ctxType, visit)
	case *ast.StructType:
		f.walk(n.Fields, ctxField, visit)
	case *ast.FuncType:
		if n.TypeParams != nil {
			f.walk(n.TypeParams, ctxParam, visit)
		}
		f.walk(n.Params, ctxParam, visit)
		if n.Results != nil {
			f.walk(n.Results, ctxParam, visit)
		}
	case *ast.InterfaceType:
		f.walk(n.Methods, ctxField, visit)
	case *ast.MapType:
		f.walk(&n.Key, ctxType, visit)
		f.walk(&n.Value, ctxType, visit)
	case *ast.ChanType:
		f.walk(&n.Value, ctxType, visit)

	case *ast.BadStmt:
	case *ast.DeclStmt:
		f.walk(n.Decl, ctxDecl, visit)
	case *ast.EmptyStmt:
	case *ast.LabeledStmt:
		f.walk(n.Stmt, ctxStmt, visit)
	case *ast.ExprStmt:
		f.walk(&n.X, ctxExpr, visit)
	case *ast.SendStmt:
		f.walk(&n.Chan, ctxExpr, visit)
		f.walk(&n.Value, ctxExpr, visit)
	case *ast.IncDecStmt:
		f.walk(&n.X, ctxExpr, visit)
	case *ast.AssignStmt:
		f.walk(n.Lhs, ctxExpr, visit)
		if len(n.Lhs) == 2 && len(n.Rhs) == 1 {
			f.walk(n.Rhs, ctxAssign2, visit)
		} else {
			f.walk(n.Rhs, ctxExpr, visit)
		}
	case *ast.GoStmt:
		f.walk(n.Call, ctxExpr, visit)
	case *ast.DeferStmt:
		f.walk(n.Call, ctxDefer, visit)
	case *ast.ReturnStmt:
		f.walk(n.Results, ctxExpr, visit)
	case *ast.BranchStmt:
	case *ast.BlockStmt:
		f.walk(n.List, context, visit)
	case *ast.IfStmt:
		f.walk(n.Init, ctxStmt, visit)
		f.walk(&n.Cond, ctxExpr, visit)
		f.walk(n.Body, ctxStmt, visit)
		f.walk(n.Else, ctxStmt, visit)
	case *ast.CaseClause:
		if context == ctxTypeSwitch {
			context = ctxType
		} else {
			context = ctxExpr
		}
		f.walk(n.List, context, visit)
		f.walk(n.Body, ctxStmt, visit)
	case *ast.SwitchStmt:
		f.walk(n.Init, ctxStmt, visit)
		f.walk(&n.Tag, ctxExpr, visit)
		f.walk(n.Body, ctxSwitch, visit)
	case *ast.TypeSwitchStmt:
		f.walk(n.Init, ctxStmt, visit)
		f.walk(n.Assign, ctxStmt, visit)
		f.walk(n.Body, ctxTypeSwitch, visit)
	case *ast.CommClause:
		f.walk(n.Comm, ctxStmt, visit)
		f.walk(n.Body, ctxStmt, visit)
	case *ast.SelectStmt:
		f.walk(n.Body, ctxStmt, visit)
	case *ast.ForStmt:
		f.walk(n.Init, ctxStmt, visit)
		f.walk(&n.Cond, ctxExpr, visit)
		f.walk(n.Post, ctxStmt, visit)
		f.walk(n.Body, ctxStmt, visit)
	case *ast.RangeStmt:
		f.walk(&n.Key, ctxExpr, visit)
		f.walk(&n.Value, ctxExpr, visit)
		f.walk(&n.X, ctxExpr, visit)
		f.walk(n.Body, ctxStmt, visit)

	case *ast.ImportSpec:
	case *ast.ValueSpec:
		f.walk(&n.Type, ctxType, visit)
		if len(n.Names) == 2 && len(n.Values) == 1 {
			f.walk(&n.Values[0], ctxAssign2, visit)
		} else {
			f.walk(n.Values, ctxExpr, visit)
		}
	case *ast.TypeSpec:
		if n.TypeParams != nil {
			f.walk(n.TypeParams, ctxParam, visit)
		}
		f.walk(&n.Type, ctxType, visit)

	case *ast.BadDecl:
	case *ast.GenDecl:
		f.walk(n.Specs, ctxSpec, visit)
	case *ast.FuncDecl:
		if n.Recv != nil {
			f.walk(n.Recv, ctxParam, visit)
		}
		f.walk(n.Type, ctxType, visit)
		if n.Body != nil {
			f.walk(n.Body, ctxStmt, visit)
		}

	case *ast.File:
		f.walk(n.Decls, ctxDecl, visit)

	case *ast.Package:
		for _, file := range n.Files {
			f.walk(file, ctxFile, visit)
		}

	case []ast.Decl:
		for _, d := range n {
			f.walk(d, context, visit)
		}
	case []ast.Expr:
		for i := range n {
			f.walk(&n[i], context, visit)
		}
	case []ast.Stmt:
		for _, s := range n {
			f.walk(s, context, visit)
		}
	case []ast.Spec:
		for _, s := range n {
			f.walk(s, context, visit)
		}
	}
}

// If x is of the form (T), unparen returns unparen(T), otherwise it returns x.
func unparen(x ast.Expr) ast.Expr {
	if p, isParen := x.(*ast.ParenExpr); isParen {
		x = unparen(p.X)
	}
	return x
}

```

// === FILE: references!/go/src/cmd/cgo/doc.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Cgo enables the creation of Go packages that call C code.

# Using cgo with the go command

To use cgo write normal Go code that imports a pseudo-package "C".
The Go code can then refer to types such as C.size_t, variables such
as C.stdout, or functions such as C.putchar.

If the import of "C" is immediately preceded by a comment, that
comment, called the preamble, is used as a header when compiling
the C parts of the package. For example:

	// #include <stdio.h>
	// #include <errno.h>
	import "C"

The preamble may contain any C code, including function and variable
declarations and definitions. These may then be referred to from Go
code as though they were defined in the package "C". All names
declared in the preamble may be used, even if they start with a
lower-case letter. Exception: static variables in the preamble may
not be referenced from Go code; static functions are permitted.

See $GOROOT/cmd/cgo/internal/teststdio and $GOROOT/misc/cgo/gmp for examples. See
"C? Go? Cgo!" for an introduction to using cgo:
https://golang.org/doc/articles/c_go_cgo.html.

CFLAGS, CPPFLAGS, CXXFLAGS, FFLAGS and LDFLAGS may be defined with pseudo
#cgo directives within these comments to tweak the behavior of the C, C++
or Fortran compiler. Values defined in multiple directives are concatenated
together. The directive can include a list of build constraints limiting its
effect to systems satisfying one of the constraints
(see https://golang.org/pkg/go/build/#hdr-Build_Constraints for details about the constraint syntax).
For example:

	// #cgo CFLAGS: -DPNG_DEBUG=1
	// #cgo amd64 386 CFLAGS: -DX86=1
	// #cgo LDFLAGS: -lpng
	// #include <png.h>
	import "C"

Alternatively, CPPFLAGS and LDFLAGS may be obtained via the pkg-config tool
using a '#cgo pkg-config:' directive followed by the package names.
For example:

	// #cgo pkg-config: png cairo
	// #include <png.h>
	import "C"

The default pkg-config tool may be changed by setting the PKG_CONFIG environment variable.

For security reasons, only a limited set of flags are allowed, notably -D, -U, -I, and -l.
To allow additional flags, set CGO_CFLAGS_ALLOW to a regular expression
matching the new flags. To disallow flags that would otherwise be allowed,
set CGO_CFLAGS_DISALLOW to a regular expression matching arguments
that must be disallowed. In both cases the regular expression must match
a full argument: to allow -mfoo=bar, use CGO_CFLAGS_ALLOW='-mfoo.*',
not just CGO_CFLAGS_ALLOW='-mfoo'. Similarly named variables control
the allowed CPPFLAGS, CXXFLAGS, FFLAGS, and LDFLAGS.

Also for security reasons, only a limited set of characters are
permitted, notably alphanumeric characters and a few symbols, such as
'.', that will not be interpreted in unexpected ways. Attempts to use
forbidden characters will get a "malformed #cgo argument" error.

When building, the CGO_CFLAGS, CGO_CPPFLAGS, CGO_CXXFLAGS, CGO_FFLAGS and
CGO_LDFLAGS environment variables are added to the flags derived from
these directives. Package-specific flags should be set using the
directives, not the environment variables, so that builds work in
unmodified environments. Flags obtained from environment variables
are not subject to the security limitations described above.

All the cgo CPPFLAGS and CFLAGS directives in a package are concatenated and
used to compile C files in that package. All the CPPFLAGS and CXXFLAGS
directives in a package are concatenated and used to compile C++ files in that
package. All the CPPFLAGS and FFLAGS directives in a package are concatenated
and used to compile Fortran files in that package. All the LDFLAGS directives
in any package in the program are concatenated and used at link time. All the
pkg-config directives are concatenated and sent to pkg-config simultaneously
to add to each appropriate set of command-line flags.

When the cgo directives are parsed, any occurrence of the string ${SRCDIR}
will be replaced by the absolute path to the directory containing the source
file. This allows pre-compiled static libraries to be included in the package
directory and linked properly.
For example if package foo is in the directory /go/src/foo:

	// #cgo LDFLAGS: -L${SRCDIR}/libs -lfoo

Will be expanded to:

	// #cgo LDFLAGS: -L/go/src/foo/libs -lfoo

When the Go tool sees that one or more Go files use the special import
"C", it will look for other non-Go files in the directory and compile
them as part of the Go package. Any .c, .s, .S or .sx files will be
compiled with the C compiler. Any .cc, .cpp, or .cxx files will be
compiled with the C++ compiler. Any .f, .F, .for or .f90 files will be
compiled with the fortran compiler. Any .h, .hh, .hpp, or .hxx files will
not be compiled separately, but, if these header files are changed,
the package (including its non-Go source files) will be recompiled.
Note that changes to files in other directories do not cause the package
to be recompiled, so all non-Go source code for the package should be
stored in the package directory, not in subdirectories.
The default C and C++ compilers may be changed by the CC and CXX
environment variables, respectively; those environment variables
may include command line options.

The cgo tool will always invoke the C compiler with the source file's
directory in the include path; i.e. -I${SRCDIR} is always implied. This
means that if a header file foo/bar.h exists both in the source
directory and also in the system include directory (or some other place
specified by a -I flag), then "#include <foo/bar.h>" will always find the
local version in preference to any other version.

The cgo tool is enabled by default for native builds on systems where
it is expected to work. It is disabled by default when cross-compiling
as well as when the CC environment variable is unset and the default
C compiler (typically gcc or clang) cannot be found on the system PATH.
You can override the default by setting the CGO_ENABLED
environment variable when running the go tool: set it to 1 to enable
the use of cgo, and to 0 to disable it. The go tool will set the
build constraint "cgo" if cgo is enabled. The special import "C"
implies the "cgo" build constraint, as though the file also said
"//go:build cgo". Therefore, if cgo is disabled, files that import
"C" will not be built by the go tool. (For more about build constraints
see https://golang.org/pkg/go/build/#hdr-Build_Constraints).

When cross-compiling, you must specify a C cross-compiler for cgo to
use. You can do this by setting the generic CC_FOR_TARGET or the
more specific CC_FOR_${GOOS}_${GOARCH} (for example, CC_FOR_linux_arm)
environment variable when building the toolchain using make.bash,
or you can set the CC environment variable any time you run the go tool.

The CXX_FOR_TARGET, CXX_FOR_${GOOS}_${GOARCH}, and CXX
environment variables work in a similar way for C++ code.

# Go references to C

Within the Go file, C's struct field names that are keywords in Go
can be accessed by prefixing them with an underscore: if x points at a C
struct with a field named "type", x._type accesses the field.
C struct fields that cannot be expressed in Go, such as bit fields
or misaligned data, are omitted in the Go struct, replaced by
appropriate padding to reach the next field or the end of the struct.

The standard C numeric types are available under the names
C.char, C.schar (signed char), C.uchar (unsigned char),
C.short, C.ushort (unsigned short), C.int, C.uint (unsigned int),
C.long, C.ulong (unsigned long), C.longlong (long long),
C.ulonglong (unsigned long long), C.float, C.double,
C.complexfloat (complex float), and C.complexdouble (complex double).
The C type void* is represented by Go's unsafe.Pointer.
The C types __int128_t and __uint128_t are represented by [16]byte.

A few special C types which would normally be represented by a pointer
type in Go are instead represented by a uintptr.  See the Special
cases section below.

To access a struct, union, or enum type directly, prefix it with
struct_, union_, or enum_, as in C.struct_stat. The size of any C type
T is available as C.sizeof_T, as in C.sizeof_struct_stat. These
special prefixes means that there is no way to directly reference a C
identifier that starts with "struct_", "union_", "enum_", or
"sizeof_", such as a function named "struct_function".
A workaround is to use a "#define" in the preamble, as in
"#define c_struct_function struct_function" and then in the
Go code refer to "C.c_struct_function".

A C function may be declared in the Go file with a parameter type of
the special name _GoString_. This function may be called with an
ordinary Go string value. The string length, and a pointer to the
string contents, may be accessed by calling the C functions

	size_t _GoStringLen(_GoString_ s);
	const char *_GoStringPtr(_GoString_ s);

These functions are only available in the preamble, not in other C
files. The C code must not modify the contents of the pointer returned
by _GoStringPtr. Note that the string contents may not have a trailing
NUL byte.

As Go doesn't have support for C's union type in the general case,
C's union types are represented as a Go byte array with the same length.

Go structs cannot embed fields with C types.

Go code cannot refer to zero-sized fields that occur at the end of
non-empty C structs. To get the address of such a field (which is the
only operation you can do with a zero-sized field) you must take the
address of the struct and add the size of the struct.

Cgo translates C types into equivalent unexported Go types.
Because the translations are unexported, a Go package should not
expose C types in its exported API: a C type used in one Go package
is different from the same C type used in another.

Any C function (even void functions) may be called in a multiple
assignment context to retrieve both the return value (if any) and the
C errno variable as an error (use _ to skip the result value if the
function returns void). For example:

	n, err = C.sqrt(-1)
	_, err := C.voidFunc()
	var n, err = C.sqrt(1)

Note that the C errno value may be non-zero, and thus the err result may be
non-nil, even if the function call is successful. Unlike normal Go conventions,
you should first check whether the call succeeded before checking the error
result. For example:

	n, err := C.setenv(key, value, 1)
	if n != 0 {
		// we know the call failed, so it is now valid to use err
		return err
	}

Calling C function pointers is currently not supported, however you can
declare Go variables which hold C function pointers and pass them
back and forth between Go and C. C code may call function pointers
received from Go. For example:

	package main

	// typedef int (*intFunc) ();
	//
	// int
	// bridge_int_func(intFunc f)
	// {
	//		return f();
	// }
	//
	// int fortytwo()
	// {
	//	    return 42;
	// }
	import "C"
	import "fmt"

	func main() {
		f := C.intFunc(C.fortytwo)
		fmt.Println(int(C.bridge_int_func(f)))
		// Output: 42
	}

In C, a function argument written as a fixed size array
actually requires a pointer to the first element of the array.
C compilers are aware of this calling convention and adjust
the call accordingly, but Go cannot. In Go, you must pass
the pointer to the first element explicitly: C.f(&C.x[0]).

Calling variadic C functions is not supported. It is possible to
circumvent this by using a C function wrapper. For example:

	package main

	// #include <stdio.h>
	// #include <stdlib.h>
	//
	// static void myprint(char* s) {
	//   printf("%s\n", s);
	// }
	import "C"
	import "unsafe"

	func main() {
		cs := C.CString("Hello from stdio")
		C.myprint(cs)
		C.free(unsafe.Pointer(cs))
	}

A few special functions convert between Go and C types
by making copies of the data. In pseudo-Go definitions:

	// Go string to C string
	// The C string is allocated in the C heap using malloc.
	// It is the caller's responsibility to arrange for it to be
	// freed, such as by calling C.free (be sure to include stdlib.h
	// if C.free is needed).
	func C.CString(string) *C.char

	// Go []byte slice to C array
	// The C array is allocated in the C heap using malloc.
	// It is the caller's responsibility to arrange for it to be
	// freed, such as by calling C.free (be sure to include stdlib.h
	// if C.free is needed).
	func C.CBytes([]byte) unsafe.Pointer

	// C string to Go string
	func C.GoString(*C.char) string

	// C data with explicit length to Go string
	func C.GoStringN(*C.char, C.int) string

	// C data with explicit length to Go []byte
	func C.GoBytes(unsafe.Pointer, C.int) []byte

As a special case, C.malloc does not call the C library malloc directly
but instead calls a Go helper function that wraps the C library malloc
but guarantees never to return nil. If C's malloc indicates out of memory,
the helper function crashes the program, like when Go itself runs out
of memory. Because C.malloc cannot fail, it has no two-result form
that returns errno.

# C references to Go

Go functions can be exported for use by C code in the following way:

	//export MyFunction
	func MyFunction(arg1, arg2 int, arg3 string) int64 {...}

	//export MyFunction2
	func MyFunction2(arg1, arg2 int, arg3 string) (int64, *C.char) {...}

They will be available in the C code as:

	extern GoInt64 MyFunction(int arg1, int arg2, GoString arg3);
	extern struct MyFunction2_return MyFunction2(int arg1, int arg2, GoString arg3);

found in the _cgo_export.h generated header, after any preambles
copied from the cgo input files. Functions with multiple
return values are mapped to functions returning a struct.

Not all Go types can be mapped to C types in a useful way.
Go struct types are not supported; use a C struct type.
Go array types are not supported; use a C pointer.

Go functions that take arguments of type string may be called with the
C type _GoString_, described above. The _GoString_ type will be
automatically defined in the preamble. Note that there is no way for C
code to create a value of this type; this is only useful for passing
string values from Go to C and back to Go.

Using //export in a file places a restriction on the preamble:
since it is copied into two different C output files, it must not
contain any definitions, only declarations. If a file contains both
definitions and declarations, then the two output files will produce
duplicate symbols and the linker will fail. To avoid this, definitions
must be placed in preambles in other files, or in C source files.

# Passing pointers

Go is a garbage collected language, and the garbage collector needs to
know the location of every pointer to Go memory. Because of this,
there are restrictions on passing pointers between Go and C.

In this section the term Go pointer means a pointer to memory
allocated by Go (such as by using the & operator or calling the
predefined new function) and the term C pointer means a pointer to
memory allocated by C (such as by a call to C.malloc). Whether a
pointer is a Go pointer or a C pointer is a dynamic property
determined by how the memory was allocated; it has nothing to do with
the type of the pointer.

Note that values of some Go types, other than the type's zero value,
always include Go pointers. This is true of interface, channel, map,
and function types. A pointer type may hold a Go pointer or a C pointer.
Array, slice, string, and struct types may or may not include Go pointers,
depending on their type and how they are constructed. All the discussion
below about Go pointers applies not just to pointer types,
but also to other types that include Go pointers.

All Go pointers passed to C must point to pinned Go memory. Go pointers
passed as function arguments to C functions have the memory they point to
implicitly pinned for the duration of the call. Go memory reachable from
these function arguments must be pinned as long as the C code has access
to it. Whether Go memory is pinned is a dynamic property of that memory
region; it has nothing to do with the type of the pointer.

Go values created by calling new, by taking the address of a composite
literal, or by taking the address of a local variable may also have their
memory pinned using [runtime.Pinner]. This type may be used to manage
the duration of the memory's pinned status, potentially beyond the
duration of a C function call. Memory may be pinned more than once and
must be unpinned exactly the same number of times it has been pinned.

Go code may pass a Go pointer to C provided the memory to which it
points does not contain any Go pointers to memory that is unpinned. When
passing a pointer to a field in a struct, the Go memory in question is
the memory occupied by the field, not the entire struct. When passing a
pointer to an element in an array or slice, the Go memory in question is
the entire array or the entire backing array of the slice.

C code may keep a copy of a Go pointer only as long as the memory it
points to is pinned.

C code may not keep a copy of a Go pointer after the call returns,
unless the memory it points to is pinned with [runtime.Pinner] and the
Pinner is not unpinned while the Go pointer is stored in C memory.
This implies that C code may not keep a copy of a string, slice,
channel, and so forth, because they cannot be pinned with
[runtime.Pinner].

The _GoString_ type also may not be pinned with [runtime.Pinner].
Because it includes a Go pointer, the memory it points to is only pinned
for the duration of the call; _GoString_ values may not be retained by C
code.

A Go function called by C code may return a Go pointer to pinned memory
(which implies that it may not return a string, slice, channel, and so
forth). A Go function called by C code may take C pointers as arguments,
and it may store non-pointer data, C pointers, or Go pointers to pinned
memory through those pointers. It may not store a Go pointer to unpinned
memory in memory pointed to by a C pointer (which again, implies that it
may not store a string, slice, channel, and so forth). A Go function
called by C code may take a Go pointer but it must preserve the property
that the Go memory to which it points (and the Go memory to which that
memory points, and so on) is pinned.

These rules are checked dynamically at runtime. The checking is
controlled by the cgocheck setting of the GODEBUG environment
variable. The default setting is GODEBUG=cgocheck=1, which implements
reasonably cheap dynamic checks. These checks may be disabled
entirely using GODEBUG=cgocheck=0. Complete checking of pointer
handling, at some cost in run time, is available by setting
GOEXPERIMENT=cgocheck2 at build time.

It is possible to defeat this enforcement by using the unsafe package,
and of course there is nothing stopping the C code from doing anything
it likes. However, programs that break these rules are likely to fail
in unexpected and unpredictable ways.

The type [runtime/cgo.Handle] can be used to safely pass Go values
between Go and C.

Note: the current implementation has a bug. While Go code is permitted
to write nil or a C pointer (but not a Go pointer) to C memory, the
current implementation may sometimes cause a runtime error if the
contents of the C memory appear to be a Go pointer. Therefore, avoid
passing uninitialized C memory to Go code if the Go code is going to
store pointer values in it. Zero out the memory in C before passing it
to Go.

# Optimizing calls of C code

When passing a Go pointer to a C function the compiler normally ensures
that the Go object lives on the heap. If the C function does not keep
a copy of the Go pointer, and never passes the Go pointer back to Go code,
then this is unnecessary. The #cgo noescape directive may be used to tell
the compiler that no Go pointers escape via the named C function.
If the noescape directive is used and the C function does not handle the
pointer safely, the program may crash or see memory corruption.

For example:

	// #cgo noescape cFunctionName

When a Go function calls a C function, it prepares for the C function to
call back to a Go function. The #cgo nocallback directive may be used to
tell the compiler that these preparations are not necessary.
If the nocallback directive is used and the C function does call back into
Go code, the program will panic.

For example:

	// #cgo nocallback cFunctionName

# Special cases

A few special C types which would normally be represented by a pointer
type in Go are instead represented by a uintptr. Those include:

1. The *Ref types on Darwin, rooted at CoreFoundation's CFTypeRef type.

2. The object types from Java's JNI interface:

	jobject
	jclass
	jthrowable
	jstring
	jarray
	jbooleanArray
	jbyteArray
	jcharArray
	jshortArray
	jintArray
	jlongArray
	jfloatArray
	jdoubleArray
	jobjectArray
	jweak

3. The EGLDisplay and EGLConfig types from the EGL API.

These types are uintptr on the Go side because they would otherwise
confuse the Go garbage collector; they are sometimes not really
pointers but data structures encoded in a pointer type. All operations
on these types must happen in C. The proper constant to initialize an
empty such reference is 0, not nil.

These special cases were introduced in Go 1.10. For auto-updating code
from Go 1.9 and earlier, use the cftype or jni rewrites in the Go fix tool:

	go tool fix -r cftype <pkg>
	go tool fix -r jni <pkg>

It will replace nil with 0 in the appropriate places.

The EGLDisplay case was introduced in Go 1.12. Use the egl rewrite
to auto-update code from Go 1.11 and earlier:

	go tool fix -r egl <pkg>

The EGLConfig case was introduced in Go 1.15. Use the eglconf rewrite
to auto-update code from Go 1.14 and earlier:

	go tool fix -r eglconf <pkg>

# Using cgo directly

Usage:

	go tool cgo [cgo options] [-- compiler options] gofiles...

Cgo transforms the specified input Go source files into several output
Go and C source files.

The compiler options are passed through uninterpreted when
invoking the C compiler to compile the C parts of the package.

The following options are available when running cgo directly:

	-V
		Print cgo version and exit.
	-debug-define
		Debugging option. Print #defines.
	-debug-gcc
		Debugging option. Trace C compiler execution and output.
	-dynimport file
		Write list of symbols imported by file. Write to
		-dynout argument or to standard output. Used by go
		build when building a cgo package.
	-dynlinker
		Write dynamic linker as part of -dynimport output.
	-dynout file
		Write -dynimport output to file.
	-dynpackage package
		Set Go package for -dynimport output.
	-exportheader file
		If there are any exported functions, write the
		generated export declarations to file.
		C code can #include this to see the declarations.
	-gccgo
		Generate output for the gccgo compiler rather than the
		gc compiler.
	-gccgoprefix prefix
		The -fgo-prefix option to be used with gccgo.
	-gccgopkgpath path
		The -fgo-pkgpath option to be used with gccgo.
	-gccgo_define_cgoincomplete
		Define cgo.Incomplete locally rather than importing it from
		the "runtime/cgo" package. Used for old gccgo versions.
	-godefs
		Write out input file in Go syntax replacing C package
		names with real values. Used to generate files in the
		syscall package when bootstrapping a new target.
	-importpath string
		The import path for the Go package. Optional; used for
		nicer comments in the generated files.
	-import_runtime_cgo
		If set (which it is by default) import runtime/cgo in
		generated output.
	-import_syscall
		If set (which it is by default) import syscall in
		generated output.
	-ldflags flags
		Flags to pass to the C linker. The cmd/go tool uses
		this to pass in the flags in the CGO_LDFLAGS variable.
	-objdir directory
		Put all generated files in directory.
	-srcdir directory
		Find the Go input files, listed on the command line,
		in directory.
	-trimpath rewrites
		Apply trims and rewrites to source file paths.
*/
package main

/*
Implementation details.

Cgo provides a way for Go programs to call C code linked into the same
address space. This comment explains the operation of cgo.

Cgo reads a set of Go source files and looks for statements saying
import "C". If the import has a doc comment, that comment is
taken as literal C code to be used as a preamble to any C code
generated by cgo. A typical preamble #includes necessary definitions:

	// #include <stdio.h>
	import "C"

For more details about the usage of cgo, see the documentation
comment at the top of this file.

Understanding C

Cgo scans the Go source files that import "C" for uses of that
package, such as C.puts. It collects all such identifiers. The next
step is to determine each kind of name. In C.xxx the xxx might refer
to a type, a function, a constant, or a global variable. Cgo must
decide which.

The obvious thing for cgo to do is to process the preamble, expanding
#includes and processing the corresponding C code. That would require
a full C parser and type checker that was also aware of any extensions
known to the system compiler (for example, all the GNU C extensions) as
well as the system-specific header locations and system-specific
pre-#defined macros. This is certainly possible to do, but it is an
enormous amount of work.

Cgo takes a different approach. It determines the meaning of C
identifiers not by parsing C code but by feeding carefully constructed
programs into the system C compiler and interpreting the generated
error messages, debug information, and object files. In practice,
parsing these is significantly less work and more robust than parsing
C source.

Cgo first invokes gcc -E -dM on the preamble, in order to find out
about simple #defines for constants and the like. These are recorded
for later use.

Next, cgo needs to identify the kinds for each identifier. For the
identifiers C.foo, cgo generates this C program:

	<preamble>
	#line 1 "not-declared"
	void __cgo_f_1_1(void) { __typeof__(foo) *__cgo_undefined__1; }
	#line 1 "not-type"
	void __cgo_f_1_2(void) { foo *__cgo_undefined__2; }
	#line 1 "not-int-const"
	void __cgo_f_1_3(void) { enum { __cgo_undefined__3 = (foo)*1 }; }
	#line 1 "not-num-const"
	void __cgo_f_1_4(void) { static const double __cgo_undefined__4 = (foo); }
	#line 1 "not-str-lit"
	void __cgo_f_1_5(void) { static const char __cgo_undefined__5[] = (foo); }

This program will not compile, but cgo can use the presence or absence
of an error message on a given line to deduce the information it
needs. The program is syntactically valid regardless of whether each
name is a type or an ordinary identifier, so there will be no syntax
errors that might stop parsing early.

An error on not-declared:1 indicates that foo is undeclared.
An error on not-type:1 indicates that foo is not a type (if declared at all, it is an identifier).
An error on not-int-const:1 indicates that foo is not an integer constant.
An error on not-num-const:1 indicates that foo is not a number constant.
An error on not-str-lit:1 indicates that foo is not a string literal.
An error on not-signed-int-const:1 indicates that foo is not a signed integer constant.

The line number specifies the name involved. In the example, 1 is foo.

Next, cgo must learn the details of each type, variable, function, or
constant. It can do this by reading object files. If cgo has decided
that t1 is a type, v2 and v3 are variables or functions, and i4, i5
are integer constants, u6 is an unsigned integer constant, and f7 and f8
are float constants, and s9 and s10 are string constants, it generates:

	<preamble>
	__typeof__(t1) *__cgo__1;
	__typeof__(v2) *__cgo__2;
	__typeof__(v3) *__cgo__3;
	__typeof__(i4) *__cgo__4;
	enum { __cgo_enum__4 = i4 };
	__typeof__(i5) *__cgo__5;
	enum { __cgo_enum__5 = i5 };
	__typeof__(u6) *__cgo__6;
	enum { __cgo_enum__6 = u6 };
	__typeof__(f7) *__cgo__7;
	__typeof__(f8) *__cgo__8;
	__typeof__(s9) *__cgo__9;
	__typeof__(s10) *__cgo__10;

	long long __cgodebug_ints[] = {
		0, // t1
		0, // v2
		0, // v3
		i4,
		i5,
		u6,
		0, // f7
		0, // f8
		0, // s9
		0, // s10
		1
	};

	double __cgodebug_floats[] = {
		0, // t1
		0, // v2
		0, // v3
		0, // i4
		0, // i5
		0, // u6
		f7,
		f8,
		0, // s9
		0, // s10
		1
	};

	const char __cgodebug_str__9[] = s9;
	const unsigned long long __cgodebug_strlen__9 = sizeof(s9)-1;
	const char __cgodebug_str__10[] = s10;
	const unsigned long long __cgodebug_strlen__10 = sizeof(s10)-1;

and again invokes the system C compiler, to produce an object file
containing debug information. Cgo parses the DWARF debug information
for __cgo__N to learn the type of each identifier. (The types also
distinguish functions from global variables.) Cgo reads the constant
values from the __cgodebug_* from the object file's data segment.

At this point cgo knows the meaning of each C.xxx well enough to start
the translation process.

Translating Go

Given the input Go files x.go and y.go, cgo generates these source
files:

	x.cgo1.go       # for gc (cmd/compile)
	y.cgo1.go       # for gc
	_cgo_gotypes.go # for gc
	_cgo_import.go  # for gc (if -dynout _cgo_import.go)
	x.cgo2.c        # for gcc
	y.cgo2.c        # for gcc
	_cgo_defun.c    # for gcc (if -gccgo)
	_cgo_export.c   # for gcc
	_cgo_export.h   # for gcc
	_cgo_main.c     # for gcc
	_cgo_flags      # for build tool (if -gccgo)

The file x.cgo1.go is a copy of x.go with the import "C" removed and
references to C.xxx replaced with names like _Cfunc_xxx or _Ctype_xxx.
The definitions of those identifiers, written as Go functions, types,
or variables, are provided in _cgo_gotypes.go.

Here is a _cgo_gotypes.go containing definitions for needed C types:

	type _Ctype_char int8
	type _Ctype_int int32
	type _Ctype_void [0]byte

The _cgo_gotypes.go file also contains the definitions of the
functions. They all have similar bodies that invoke runtime·cgocall
to make a switch from the Go runtime world to the system C (GCC-based)
world.

For example, here is the definition of _Cfunc_puts:

	//go:cgo_import_static _cgo_be59f0f25121_Cfunc_puts
	//go:linkname __cgofn__cgo_be59f0f25121_Cfunc_puts _cgo_be59f0f25121_Cfunc_puts
	var __cgofn__cgo_be59f0f25121_Cfunc_puts byte
	var _cgo_be59f0f25121_Cfunc_puts = unsafe.Pointer(&__cgofn__cgo_be59f0f25121_Cfunc_puts)

	func _Cfunc_puts(p0 *_Ctype_char) (r1 _Ctype_int) {
		_cgo_runtime_cgocall(_cgo_be59f0f25121_Cfunc_puts, uintptr(unsafe.Pointer(&p0)))
		return
	}

The hexadecimal number is a hash of cgo's input, chosen to be
deterministic yet unlikely to collide with other uses. The actual
function _cgo_be59f0f25121_Cfunc_puts is implemented in a C source
file compiled by gcc, the file x.cgo2.c:

	void
	_cgo_be59f0f25121_Cfunc_puts(void *v)
	{
		struct {
			char* p0;
			int r;
			char __pad12[4];
		} __attribute__((__packed__, __gcc_struct__)) *a = v;
		a->r = puts((void*)a->p0);
	}

It extracts the arguments from the pointer to _Cfunc_puts's argument
frame, invokes the system C function (in this case, puts), stores the
result in the frame, and returns.

Linking

Once the _cgo_export.c and *.cgo2.c files have been compiled with gcc,
they need to be linked into the final binary, along with the libraries
they might depend on (in the case of puts, stdio). cmd/link has been
extended to understand basic ELF files, but it does not understand ELF
in the full complexity that modern C libraries embrace, so it cannot
in general generate direct references to the system libraries.

Instead, the build process generates an object file using dynamic
linkage to the desired libraries. The main function is provided by
_cgo_main.c:

	int main(int argc, char **argv) { return 0; }
	void crosscall2(void(*fn)(void*), void *a, int c, uintptr_t ctxt) { }
	uintptr_t _cgo_wait_runtime_init_done(void) { return 0; }
	void _cgo_release_context(uintptr_t ctxt) { }
	char* _cgo_topofstack(void) { return (char*)0; }
	void _cgo_allocate(void *a, int c) { }
	void _cgo_panic(void *a, int c) { }
	void _cgo_reginit(void) { }

The extra functions here are stubs to satisfy the references in the C
code generated for gcc. The build process links this stub, along with
_cgo_export.c and *.cgo2.c, into a dynamic executable and then lets
cgo examine the executable. Cgo records the list of shared library
references and resolved names and writes them into a new file
_cgo_import.go, which looks like:

	//go:cgo_dynamic_linker "/lib64/ld-linux-x86-64.so.2"
	//go:cgo_import_dynamic puts puts#GLIBC_2.2.5 "libc.so.6"
	//go:cgo_import_dynamic __libc_start_main __libc_start_main#GLIBC_2.2.5 "libc.so.6"
	//go:cgo_import_dynamic stdout stdout#GLIBC_2.2.5 "libc.so.6"
	//go:cgo_import_dynamic fflush fflush#GLIBC_2.2.5 "libc.so.6"
	//go:cgo_import_dynamic _ _ "libpthread.so.0"
	//go:cgo_import_dynamic _ _ "libc.so.6"

In the end, the compiled Go package, which will eventually be
presented to cmd/link as part of a larger program, contains:

	_go_.o        # gc-compiled object for _cgo_gotypes.go, _cgo_import.go, *.cgo1.go
	_all.o        # gcc-compiled object for _cgo_export.c, *.cgo2.c

If there is an error generating the _cgo_import.go file, then, instead
of adding _cgo_import.go to the package, the go tool adds an empty
file named dynimportfail. The _cgo_import.go file is only needed when
using internal linking mode, which is not the default when linking
programs that use cgo (as described below). If the linker sees a file
named dynimportfail it reports an error if it has been told to use
internal linking mode. This approach is taken because generating
_cgo_import.go requires doing a full C link of the package, which can
fail for reasons that are irrelevant when using external linking mode.

The final program will be a dynamic executable, so that cmd/link can avoid
needing to process arbitrary .o files. It only needs to process the .o
files generated from C files that cgo writes, and those are much more
limited in the ELF or other features that they use.

In essence, the _cgo_import.o file includes the extra linking
directives that cmd/link is not sophisticated enough to derive from _all.o
on its own. Similarly, the _all.o uses dynamic references to real
system object code because cmd/link is not sophisticated enough to process
the real code.

The main benefits of this system are that cmd/link remains relatively simple
(it does not need to implement a complete ELF and Mach-O linker) and
that gcc is not needed after the package is compiled. For example,
package net uses cgo for access to name resolution functions provided
by libc. Although gcc is needed to compile package net, gcc is not
needed to link programs that import package net.

Runtime

When using cgo, Go must not assume that it owns all details of the
process. In particular it needs to coordinate with C in the use of
threads and thread-local storage. The runtime package declares a few
variables:

	var (
		iscgo             bool
		_cgo_init         unsafe.Pointer
		_cgo_thread_start unsafe.Pointer
	)

Any package using cgo imports "runtime/cgo", which provides
initializations for these variables. It sets iscgo to true, _cgo_init
to a gcc-compiled function that can be called early during program
startup, and _cgo_thread_start to a gcc-compiled function that can be
used to create a new thread, in place of the runtime's usual direct
system calls.

Internal and External Linking

The text above describes "internal" linking, in which cmd/link parses and
links host object files (ELF, Mach-O, PE, and so on) into the final
executable itself. Keeping cmd/link simple means we cannot possibly
implement the full semantics of the host linker, so the kinds of
objects that can be linked directly into the binary is limited (other
code can only be used as a dynamic library). On the other hand, when
using internal linking, cmd/link can generate Go binaries by itself.

In order to allow linking arbitrary object files without requiring
dynamic libraries, cgo supports an "external" linking mode too. In
external linking mode, cmd/link does not process any host object files.
Instead, it collects all the Go code and writes a single go.o object
file containing it. Then it invokes the host linker (usually gcc) to
combine the go.o object file and any supporting non-Go code into a
final executable. External linking avoids the dynamic library
requirement but introduces a requirement that the host linker be
present to create such a binary.

Most builds both compile source code and invoke the linker to create a
binary. When cgo is involved, the compile step already requires gcc, so
it is not problematic for the link step to require gcc too.

An important exception is builds using a pre-compiled copy of the
standard library. In particular, package net uses cgo on most systems,
and we want to preserve the ability to compile pure Go code that
imports net without requiring gcc to be present at link time. (In this
case, the dynamic library requirement is less significant, because the
only library involved is libc.so, which can usually be assumed
present.)

This conflict between functionality and the gcc requirement means we
must support both internal and external linking, depending on the
circumstances: if net is the only cgo-using package, then internal
linking is probably fine, but if other packages are involved, so that there
are dependencies on libraries beyond libc, external linking is likely
to work better. The compilation of a package records the relevant
information to support both linking modes, leaving the decision
to be made when linking the final binary.

Linking Directives

In either linking mode, package-specific directives must be passed
through to cmd/link. These are communicated by writing //go: directives in a
Go source file compiled by gc. The directives are copied into the .o
object file and then processed by the linker.

The directives are:

//go:cgo_import_dynamic <local> [<remote> ["<library>"]]

	In internal linking mode, allow an unresolved reference to
	<local>, assuming it will be resolved by a dynamic library
	symbol. The optional <remote> specifies the symbol's name and
	possibly version in the dynamic library, and the optional "<library>"
	names the specific library where the symbol should be found.

	On AIX, the library pattern is slightly different. It must be
	"lib.a/obj.o" with obj.o the member of this library exporting
	this symbol.

	In the <remote>, # or @ can be used to introduce a symbol version.

	Examples:
	//go:cgo_import_dynamic puts
	//go:cgo_import_dynamic puts puts#GLIBC_2.2.5
	//go:cgo_import_dynamic puts puts#GLIBC_2.2.5 "libc.so.6"

	A side effect of the cgo_import_dynamic directive with a
	library is to make the final binary depend on that dynamic
	library. To get the dependency without importing any specific
	symbols, use _ for local and remote.

	Example:
	//go:cgo_import_dynamic _ _ "libc.so.6"

	For compatibility with current versions of SWIG,
	#pragma dynimport is an alias for //go:cgo_import_dynamic.

//go:cgo_dynamic_linker "<path>"

	In internal linking mode, use "<path>" as the dynamic linker
	in the final binary. This directive is only needed from one
	package when constructing a binary; by convention it is
	supplied by runtime/cgo.

	Example:
	//go:cgo_dynamic_linker "/lib/ld-linux.so.2"

//go:cgo_export_dynamic <local> <remote>

	In internal linking mode, put the Go symbol
	named <local> into the program's exported symbol table as
	<remote>, so that C code can refer to it by that name. This
	mechanism makes it possible for C code to call back into Go or
	to share Go's data.

	For compatibility with current versions of SWIG,
	#pragma dynexport is an alias for //go:cgo_export_dynamic.

//go:cgo_import_static <local>

	In external linking mode, allow unresolved references to
	<local> in the go.o object file prepared for the host linker,
	under the assumption that <local> will be supplied by the
	other object files that will be linked with go.o.

	Example:
	//go:cgo_import_static puts_wrapper

//go:cgo_export_static <local> <remote>

	In external linking mode, put the Go symbol
	named <local> into the program's exported symbol table as
	<remote>, so that C code can refer to it by that name. This
	mechanism makes it possible for C code to call back into Go or
	to share Go's data.

//go:cgo_ldflag "<arg>"

	In external linking mode, invoke the host linker (usually gcc)
	with "<arg>" as a command-line argument following the .o files.
	Note that the arguments are for "gcc", not "ld".

	Example:
	//go:cgo_ldflag "-lpthread"
	//go:cgo_ldflag "-L/usr/local/sqlite3/lib"

A package compiled with cgo will include directives for both
internal and external linking; the linker will select the appropriate
subset for the chosen linking mode.

Example

As a simple example, consider a package that uses cgo to call C.sin.
The following code will be generated by cgo:

	// compiled by gc

	//go:cgo_ldflag "-lm"

	type _Ctype_double float64

	//go:cgo_import_static _cgo_gcc_Cfunc_sin
	//go:linkname __cgo_gcc_Cfunc_sin _cgo_gcc_Cfunc_sin
	var __cgo_gcc_Cfunc_sin byte
	var _cgo_gcc_Cfunc_sin = unsafe.Pointer(&__cgo_gcc_Cfunc_sin)

	func _Cfunc_sin(p0 _Ctype_double) (r1 _Ctype_double) {
		_cgo_runtime_cgocall(_cgo_gcc_Cfunc_sin, uintptr(unsafe.Pointer(&p0)))
		return
	}

	// compiled by gcc, into foo.cgo2.o

	void
	_cgo_gcc_Cfunc_sin(void *v)
	{
		struct {
			double p0;
			double r;
		} __attribute__((__packed__)) *a = v;
		a->r = sin(a->p0);
	}

What happens at link time depends on whether the final binary is linked
using the internal or external mode. If other packages are compiled in
"external only" mode, then the final link will be an external one.
Otherwise the link will be an internal one.

The linking directives are used according to the kind of final link
used.

In internal mode, cmd/link itself processes all the host object files, in
particular foo.cgo2.o. To do so, it uses the cgo_import_dynamic and
cgo_dynamic_linker directives to learn that the otherwise undefined
reference to sin in foo.cgo2.o should be rewritten to refer to the
symbol sin with version GLIBC_2.2.5 from the dynamic library
"libm.so.6", and the binary should request "/lib/ld-linux.so.2" as its
runtime dynamic linker.

In external mode, cmd/link does not process any host object files, in
particular foo.cgo2.o. It links together the gc-generated object
files, along with any other Go code, into a go.o file. While doing
that, cmd/link will discover that there is no definition for
_cgo_gcc_Cfunc_sin, referred to by the gc-compiled source file. This
is okay, because cmd/link also processes the cgo_import_static directive and
knows that _cgo_gcc_Cfunc_sin is expected to be supplied by a host
object file, so cmd/link does not treat the missing symbol as an error when
creating go.o. Indeed, the definition for _cgo_gcc_Cfunc_sin will be
provided to the host linker by foo2.cgo.o, which in turn will need the
symbol 'sin'. cmd/link also processes the cgo_ldflag directives, so that it
knows that the eventual host link command must include the -lm
argument, so that the host linker will be able to find 'sin' in the
math library.

cmd/link Command Line Interface

The go command and any other Go-aware build systems invoke cmd/link
to link a collection of packages into a single binary. By default, cmd/link will
present the same interface it does today:

	cmd/link main.a

produces a file named a.out, even if cmd/link does so by invoking the host
linker in external linking mode.

By default, cmd/link will decide the linking mode as follows: if the only
packages using cgo are those on a list of known standard library
packages (net, os/user, runtime/cgo), cmd/link will use internal linking
mode. Otherwise, there are non-standard cgo packages involved, and cmd/link
will use external linking mode. The first rule means that a build of
the godoc binary, which uses net but no other cgo, can run without
needing gcc available. The second rule means that a build of a
cgo-wrapped library like sqlite3 can generate a standalone executable
instead of needing to refer to a dynamic library. The specific choice
can be overridden using a command line flag: cmd/link -linkmode=internal or
cmd/link -linkmode=external.

In an external link, cmd/link will create a temporary directory, write any
host object files found in package archives to that directory (renamed
to avoid conflicts), write the go.o file to that directory, and invoke
the host linker. The default value for the host linker is $CC, split
into fields, or else "gcc". The specific host linker command line can
be overridden using command line flags: cmd/link -extld=clang
-extldflags='-ggdb -O3'. If any package in a build includes a .cc or
other file compiled by the C++ compiler, the go tool will use the
-extld option to set the host linker to the C++ compiler.

These defaults mean that Go-aware build systems can ignore the linking
changes and keep running plain 'cmd/link' and get reasonable results, but
they can also control the linking details if desired.

*/

```

// === FILE: references!/go/src/cmd/cgo/gcc.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Annotate Ref in Prog with C types by parsing gcc debug output.
// Conversion of debug output to Go types.

package main

import (
	"bytes"
	"debug/dwarf"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"internal/xcoff"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"cmd/internal/quoted"
)

var debugDefine = flag.Bool("debug-define", false, "print relevant #defines")
var debugGcc = flag.Bool("debug-gcc", false, "print gcc invocations")

var nameToC = map[string]string{
	"schar":         "signed char",
	"uchar":         "unsigned char",
	"ushort":        "unsigned short",
	"uint":          "unsigned int",
	"ulong":         "unsigned long",
	"longlong":      "long long",
	"ulonglong":     "unsigned long long",
	"complexfloat":  "float _Complex",
	"complexdouble": "double _Complex",
}

var incomplete = "_cgopackage.Incomplete"

// cname returns the C name to use for C.s.
// The expansions are listed in nameToC and also
// struct_foo becomes "struct foo", and similarly for
// union and enum.
func cname(s string) string {
	if t, ok := nameToC[s]; ok {
		return t
	}

	if t, ok := strings.CutPrefix(s, "struct_"); ok {
		return "struct " + t
	}
	if t, ok := strings.CutPrefix(s, "union_"); ok {
		return "union " + t
	}
	if t, ok := strings.CutPrefix(s, "enum_"); ok {
		return "enum " + t
	}
	if t, ok := strings.CutPrefix(s, "sizeof_"); ok {
		return "sizeof(" + cname(t) + ")"
	}
	return s
}

// ProcessCgoDirectives processes the import C preamble:
//  1. discards all #cgo CFLAGS, LDFLAGS, nocallback and noescape directives,
//     so they don't make their way into _cgo_export.h.
//  2. parse the nocallback and noescape directives.
func (f *File) ProcessCgoDirectives() {
	linesIn := strings.Split(f.Preamble, "\n")
	linesOut := make([]string, 0, len(linesIn))
	f.NoCallbacks = make(map[string]bool)
	f.NoEscapes = make(map[string]bool)
	for _, line := range linesIn {
		l := strings.TrimSpace(line)
		if len(l) < 5 || l[:4] != "#cgo" || !unicode.IsSpace(rune(l[4])) {
			linesOut = append(linesOut, line)
		} else {
			linesOut = append(linesOut, "")

			// #cgo (nocallback|noescape) <function name>
			if fields := strings.Fields(l); len(fields) == 3 {
				directive := fields[1]
				funcName := fields[2]
				if directive == "nocallback" {
					f.NoCallbacks[funcName] = true
				} else if directive == "noescape" {
					f.NoEscapes[funcName] = true
				}
			}
		}
	}
	f.Preamble = strings.Join(linesOut, "\n")
}

// addToFlag appends args to flag.
func (p *Package) addToFlag(flag string, args []string) {
	if flag == "CFLAGS" {
		// We'll also need these when preprocessing for dwarf information.
		// However, discard any -g options: we need to be able
		// to parse the debug info, so stick to what we expect.
		for _, arg := range args {
			if !strings.HasPrefix(arg, "-g") {
				p.GccOptions = append(p.GccOptions, arg)
			}
		}
	}
	if flag == "LDFLAGS" {
		p.LdFlags = append(p.LdFlags, args...)
	}
}

// splitQuoted splits the string s around each instance of one or more consecutive
// white space characters while taking into account quotes and escaping, and
// returns an array of substrings of s or an empty list if s contains only white space.
// Single quotes and double quotes are recognized to prevent splitting within the
// quoted region, and are removed from the resulting substrings. If a quote in s
// isn't closed err will be set and r will have the unclosed argument as the
// last element. The backslash is used for escaping.
//
// For example, the following string:
//
//	`a b:"c d" 'e''f'  "g\""`
//
// Would be parsed as:
//
//	[]string{"a", "b:c d", "ef", `g"`}
func splitQuoted(s string) (r []string, err error) {
	var args []string
	arg := make([]rune, len(s))
	escaped := false
	quoted := false
	quote := '\x00'
	i := 0
	for _, r := range s {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
			continue
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
		case r == '"' || r == '\'':
			quoted = true
			quote = r
			continue
		case unicode.IsSpace(r):
			if quoted || i > 0 {
				quoted = false
				args = append(args, string(arg[:i]))
				i = 0
			}
			continue
		}
		arg[i] = r
		i++
	}
	if quoted || i > 0 {
		args = append(args, string(arg[:i]))
	}
	if quote != 0 {
		err = errors.New("unclosed quote")
	} else if escaped {
		err = errors.New("unfinished escaping")
	}
	return args, err
}

// loadDebug runs gcc to load debug information for the File. The debug
// information will be saved to the debugs field of the file, and be
// processed when Translate is called on the file later.
// loadDebug is called concurrently with different files.
func (f *File) loadDebug(p *Package) {
	for _, cref := range f.Ref {
		// Convert C.ulong to C.unsigned long, etc.
		cref.Name.C = cname(cref.Name.Go)
	}

	ft := fileTypedefs{typedefs: make(map[string]bool)}
	numTypedefs := -1
	for len(ft.typedefs) > numTypedefs {
		numTypedefs = len(ft.typedefs)
		// Also ask about any typedefs we've seen so far.
		for _, info := range ft.typedefList {
			if f.Name[info.typedef] != nil {
				continue
			}
			n := &Name{
				Go: info.typedef,
				C:  info.typedef,
			}
			f.Name[info.typedef] = n
			f.NamePos[n] = info.pos
		}
		needType := p.guessKinds(f)
		if len(needType) > 0 {
			f.debugs = append(f.debugs, p.loadDWARF(f, &ft, needType))
		}

		// In godefs mode we're OK with the typedefs, which
		// will presumably also be defined in the file, we
		// don't want to resolve them to their base types.
		if *godefs {
			break
		}
	}
}

// Translate rewrites f.AST, the original Go input, to remove
// references to the imported package C, replacing them with
// references to the equivalent Go types, functions, and variables.
// Preconditions: File.loadDebug must be called prior to translate.
func (p *Package) Translate(f *File) {
	var conv typeConv
	conv.Init(p.PtrSize, p.IntSize)
	for _, d := range f.debugs {
		p.recordTypes(f, d, &conv)
	}
	p.prepareNames(f)
	if p.rewriteCalls(f) {
		// Add `import _cgo_unsafe "unsafe"` after the package statement.
		f.Edit.Insert(f.offset(f.AST.Name.End()), "; import _cgo_unsafe \"unsafe\"")
	}
	p.rewriteRef(f)
}

// loadDefines coerces gcc into spitting out the #defines in use
// in the file f and saves relevant renamings in f.Name[name].Define.
// Returns true if env:CC is Clang
func (f *File) loadDefines(gccOptions []string) bool {
	var b bytes.Buffer
	b.WriteString(builtinProlog)
	b.WriteString(f.Preamble)
	stdout := gccDefines(b.Bytes(), gccOptions)

	var gccIsClang bool
	for line := range strings.SplitSeq(stdout, "\n") {
		if len(line) < 9 || line[0:7] != "#define" {
			continue
		}

		line = strings.TrimSpace(line[8:])

		var key, val string
		spaceIndex := strings.Index(line, " ")
		tabIndex := strings.Index(line, "\t")

		if spaceIndex == -1 && tabIndex == -1 {
			continue
		} else if tabIndex == -1 || (spaceIndex != -1 && spaceIndex < tabIndex) {
			key = line[0:spaceIndex]
			val = strings.TrimSpace(line[spaceIndex:])
		} else {
			key = line[0:tabIndex]
			val = strings.TrimSpace(line[tabIndex:])
		}

		if key == "__clang__" {
			gccIsClang = true
		}

		if n := f.Name[key]; n != nil {
			if *debugDefine {
				fmt.Fprintf(os.Stderr, "#define %s %s\n", key, val)
			}
			n.Define = val
		}
	}
	return gccIsClang
}

// guessKinds tricks gcc into revealing the kind of each
// name xxx for the references C.xxx in the Go input.
// The kind is either a constant, type, or variable.
// guessKinds is called concurrently with different files.
func (p *Package) guessKinds(f *File) []*Name {
	// Determine kinds for names we already know about,
	// like #defines or 'struct foo', before bothering with gcc.
	var names, needType []*Name
	optional := map[*Name]bool{}
	for _, key := range nameKeys(f.Name) {
		n := f.Name[key]
		// If we've already found this name as a #define
		// and we can translate it as a constant value, do so.
		if n.Define != "" {
			if i, err := strconv.ParseInt(n.Define, 0, 64); err == nil {
				n.Kind = "iconst"
				// Turn decimal into hex, just for consistency
				// with enum-derived constants. Otherwise
				// in the cgo -godefs output half the constants
				// are in hex and half are in whatever the #define used.
				n.Const = fmt.Sprintf("%#x", i)
			} else if n.Define[0] == '\'' {
				if _, err := parser.ParseExpr(n.Define); err == nil {
					n.Kind = "iconst"
					n.Const = n.Define
				}
			} else if n.Define[0] == '"' {
				if _, err := parser.ParseExpr(n.Define); err == nil {
					n.Kind = "sconst"
					n.Const = n.Define
				}
			}

			if n.IsConst() {
				continue
			}
		}

		// If this is a struct, union, or enum type name, no need to guess the kind.
		if strings.HasPrefix(n.C, "struct ") || strings.HasPrefix(n.C, "union ") || strings.HasPrefix(n.C, "enum ") {
			n.Kind = "type"
			needType = append(needType, n)
			continue
		}

		if (goos == "darwin" || goos == "ios") && strings.HasSuffix(n.C, "Ref") {
			// For FooRef, find out if FooGetTypeID exists.
			s := n.C[:len(n.C)-3] + "GetTypeID"
			n := &Name{Go: s, C: s}
			names = append(names, n)
			optional[n] = true
		}

		// Otherwise, we'll need to find out from gcc.
		names = append(names, n)
	}

	// Bypass gcc if there's nothing left to find out.
	if len(names) == 0 {
		return needType
	}

	// Coerce gcc into telling us whether each name is a type, a value, or undeclared.
	// For names, find out whether they are integer constants.
	// We used to look at specific warning or error messages here, but that tied the
	// behavior too closely to specific versions of the compilers.
	// Instead, arrange that we can infer what we need from only the presence or absence
	// of an error on a specific line.
	//
	// For each name, we generate these lines, where xxx is the index in toSniff plus one.
	//
	//	#line xxx "not-declared"
	//	void __cgo_f_xxx_1(void) { __typeof__(name) *__cgo_undefined__1; }
	//	#line xxx "not-type"
	//	void __cgo_f_xxx_2(void) { name *__cgo_undefined__2; }
	//	#line xxx "not-int-const"
	//	void __cgo_f_xxx_3(void) { enum { __cgo_undefined__3 = (name)*1 }; }
	//	#line xxx "not-num-const"
	//	void __cgo_f_xxx_4(void) { static const double __cgo_undefined__4 = (name); }
	//	#line xxx "not-str-lit"
	//	void __cgo_f_xxx_5(void) { static const char __cgo_undefined__5[] = (name); }
	//
	// If we see an error at not-declared:xxx, the corresponding name is not declared.
	// If we see an error at not-type:xxx, the corresponding name is not a type.
	// If we see an error at not-int-const:xxx, the corresponding name is not an integer constant.
	// If we see an error at not-num-const:xxx, the corresponding name is not a number constant.
	// If we see an error at not-str-lit:xxx, the corresponding name is not a string literal.
	//
	// The specific input forms are chosen so that they are valid C syntax regardless of
	// whether name denotes a type or an expression.

	var b bytes.Buffer
	b.WriteString(builtinProlog)
	b.WriteString(f.Preamble)

	for i, n := range names {
		fmt.Fprintf(&b, "#line %d \"not-declared\"\n"+
			"void __cgo_f_%d_1(void) { __typeof__(%s) *__cgo_undefined__1; }\n"+
			"#line %d \"not-type\"\n"+
			"void __cgo_f_%d_2(void) { %s *__cgo_undefined__2; }\n"+
			"#line %d \"not-int-const\"\n"+
			"void __cgo_f_%d_3(void) { enum { __cgo_undefined__3 = (%s)*1 }; }\n"+
			"#line %d \"not-num-const\"\n"+
			"void __cgo_f_%d_4(void) { static const double __cgo_undefined__4 = (%s); }\n"+
			"#line %d \"not-str-lit\"\n"+
			"void __cgo_f_%d_5(void) { static const char __cgo_undefined__5[] = (%s); }\n",
			i+1, i+1, n.C,
			i+1, i+1, n.C,
			i+1, i+1, n.C,
			i+1, i+1, n.C,
			i+1, i+1, n.C,
		)
	}
	fmt.Fprintf(&b, "#line 1 \"completed\"\n"+
		"int __cgo__1 = __cgo__2;\n")

	// We need to parse the output from this gcc command, so ensure that it
	// doesn't have any ANSI escape sequences in it. (TERM=dumb is
	// insufficient; if the user specifies CGO_CFLAGS=-fdiagnostics-color,
	// GCC will ignore TERM, and GCC can also be configured at compile-time
	// to ignore TERM.)
	stderr := p.gccErrors(b.Bytes(), "-fdiagnostics-color=never")
	if strings.Contains(stderr, "unrecognized command line option") {
		// We're using an old version of GCC that doesn't understand
		// -fdiagnostics-color. Those versions can't print color anyway,
		// so just rerun without that option.
		stderr = p.gccErrors(b.Bytes())
	}
	if stderr == "" {
		fatalf("%s produced no output\non input:\n%s", gccBaseCmd[0], b.Bytes())
	}

	completed := false
	sniff := make([]int, len(names))
	const (
		notType = 1 << iota
		notIntConst
		notNumConst
		notStrLiteral
		notDeclared
	)
	sawUnmatchedErrors := false
	for line := range strings.SplitSeq(stderr, "\n") {
		// Ignore warnings and random comments, with one
		// exception: newer GCC versions will sometimes emit
		// an error on a macro #define with a note referring
		// to where the expansion occurs. We care about where
		// the expansion occurs, so in that case treat the note
		// as an error.
		isError := strings.Contains(line, ": error:")
		isErrorNote := strings.Contains(line, ": note:") && sawUnmatchedErrors
		if !isError && !isErrorNote {
			continue
		}

		c1 := strings.Index(line, ":")
		if c1 < 0 {
			continue
		}
		c2 := strings.Index(line[c1+1:], ":")
		if c2 < 0 {
			continue
		}
		c2 += c1 + 1

		filename := line[:c1]
		i, _ := strconv.Atoi(line[c1+1 : c2])
		i--
		if i < 0 || i >= len(names) {
			if isError {
				sawUnmatchedErrors = true
			}
			continue
		}

		switch filename {
		case "completed":
			// Strictly speaking, there is no guarantee that seeing the error at completed:1
			// (at the end of the file) means we've seen all the errors from earlier in the file,
			// but usually it does. Certainly if we don't see the completed:1 error, we did
			// not get all the errors we expected.
			completed = true

		case "not-declared":
			sniff[i] |= notDeclared
		case "not-type":
			sniff[i] |= notType
		case "not-int-const":
			sniff[i] |= notIntConst
		case "not-num-const":
			sniff[i] |= notNumConst
		case "not-str-lit":
			sniff[i] |= notStrLiteral
		default:
			if isError {
				sawUnmatchedErrors = true
			}
			continue
		}

		sawUnmatchedErrors = false
	}

	if !completed {
		fatalf("%s did not produce error at completed:1\non input:\n%s\nfull error output:\n%s", gccBaseCmd[0], b.Bytes(), stderr)
	}

	for i, n := range names {
		switch sniff[i] {
		default:
			if sniff[i]&notDeclared != 0 && optional[n] {
				// Ignore optional undeclared identifiers.
				// Don't report an error, and skip adding n to the needType array.
				continue
			}
			error_(f.NamePos[n], "could not determine what C.%s refers to", fixGo(n.Go))
		case notStrLiteral | notType:
			n.Kind = "iconst"
		case notIntConst | notStrLiteral | notType:
			n.Kind = "fconst"
		case notIntConst | notNumConst | notType:
			n.Kind = "sconst"
		case notIntConst | notNumConst | notStrLiteral:
			n.Kind = "type"
		case notIntConst | notNumConst | notStrLiteral | notType:
			n.Kind = "not-type"
		}
		needType = append(needType, n)
	}
	if nerrors > 0 {
		// Check if compiling the preamble by itself causes any errors,
		// because the messages we've printed out so far aren't helpful
		// to users debugging preamble mistakes. See issue 8442.
		preambleErrors := p.gccErrors([]byte(builtinProlog + f.Preamble))
		if len(preambleErrors) > 0 {
			error_(token.NoPos, "\n%s errors for preamble:\n%s", gccBaseCmd[0], preambleErrors)
		}

		fatalf("unresolved names")
	}

	return needType
}

// loadDWARF parses the DWARF debug information generated
// by gcc to learn the details of the constants, variables, and types
// being referred to as C.xxx.
// loadDwarf is called concurrently with different files.
func (p *Package) loadDWARF(f *File, ft *fileTypedefs, names []*Name) *debug {
	// Extract the types from the DWARF section of an object
	// from a well-formed C program. Gcc only generates DWARF info
	// for symbols in the object file, so it is not enough to print the
	// preamble and hope the symbols we care about will be there.
	// Instead, emit
	//	__typeof__(names[i]) *__cgo__i;
	// for each entry in names and then dereference the type we
	// learn for __cgo__i.
	var b bytes.Buffer
	b.WriteString(builtinProlog)
	b.WriteString(f.Preamble)
	b.WriteString("#line 1 \"cgo-dwarf-inference\"\n")
	for i, n := range names {
		fmt.Fprintf(&b, "__typeof__(%s) *__cgo__%d;\n", n.C, i)
		if n.Kind == "iconst" {
			fmt.Fprintf(&b, "enum { __cgo_enum__%d = %s };\n", i, n.C)
		}
	}

	// We create a data block initialized with the values,
	// so we can read them out of the object file.
	fmt.Fprintf(&b, "long long __cgodebug_ints[] = {\n")
	for _, n := range names {
		if n.Kind == "iconst" {
			fmt.Fprintf(&b, "\t%s,\n", n.C)
		} else {
			fmt.Fprintf(&b, "\t0,\n")
		}
	}
	// for the last entry, we cannot use 0, otherwise
	// in case all __cgodebug_data is zero initialized,
	// LLVM-based gcc will place the it in the __DATA.__common
	// zero-filled section (our debug/macho doesn't support
	// this)
	fmt.Fprintf(&b, "\t1\n")
	fmt.Fprintf(&b, "};\n")

	// do the same work for floats.
	fmt.Fprintf(&b, "double __cgodebug_floats[] = {\n")
	for _, n := range names {
		if n.Kind == "fconst" {
			fmt.Fprintf(&b, "\t%s,\n", n.C)
		} else {
			fmt.Fprintf(&b, "\t0,\n")
		}
	}
	fmt.Fprintf(&b, "\t1\n")
	fmt.Fprintf(&b, "};\n")

	// do the same work for strings.
	for i, n := range names {
		if n.Kind == "sconst" {
			fmt.Fprintf(&b, "const char __cgodebug_str__%d[] = %s;\n", i, n.C)
			fmt.Fprintf(&b, "const unsigned long long __cgodebug_strlen__%d = sizeof(%s)-1;\n", i, n.C)
		}
	}

	d, ints, floats, strs := p.gccDebug(b.Bytes(), len(names))

	// Scan DWARF info for top-level TagVariable entries with AttrName __cgo__i.
	types := make([]dwarf.Type, len(names))
	r := d.Reader()
	for {
		e, err := r.Next()
		if err != nil {
			fatalf("reading DWARF entry: %s", err)
		}
		if e == nil {
			break
		}
		switch e.Tag {
		case dwarf.TagVariable:
			name, _ := e.Val(dwarf.AttrName).(string)
			// As of https://reviews.llvm.org/D123534, clang
			// now emits DW_TAG_variable DIEs that have
			// no name (so as to be able to describe the
			// type and source locations of constant strings)
			// like the second arg in the call below:
			//
			//     myfunction(42, "foo")
			//
			// If a var has no name we won't see attempts to
			// refer to it via "C.<name>", so skip these vars
			//
			// See issue 53000 for more context.
			if name == "" {
				break
			}
			typOff, _ := e.Val(dwarf.AttrType).(dwarf.Offset)
			if typOff == 0 {
				if e.Val(dwarf.AttrSpecification) != nil {
					// Since we are reading all the DWARF,
					// assume we will see the variable elsewhere.
					break
				}
				fatalf("malformed DWARF TagVariable entry")
			}
			if !strings.HasPrefix(name, "__cgo__") {
				break
			}
			typ, err := d.Type(typOff)
			if err != nil {
				fatalf("loading DWARF type: %s", err)
			}
			t, ok := typ.(*dwarf.PtrType)
			if !ok || t == nil {
				fatalf("internal error: %s has non-pointer type", name)
			}
			i, err := strconv.Atoi(name[7:])
			if err != nil {
				fatalf("malformed __cgo__ name: %s", name)
			}
			types[i] = t.Type
			ft.recordTypedefs(t.Type, f.NamePos[names[i]])
		}
		if e.Tag != dwarf.TagCompileUnit {
			r.SkipChildren()
		}
	}

	return &debug{names, types, ints, floats, strs}
}

// debug is the data extracted by running an iteration of loadDWARF on a file.
type debug struct {
	names  []*Name
	types  []dwarf.Type
	ints   []int64
	floats []float64
	strs   []string
}

func (p *Package) recordTypes(f *File, data *debug, conv *typeConv) {
	names, types, ints, floats, strs := data.names, data.types, data.ints, data.floats, data.strs

	// Record types and typedef information.
	for i, n := range names {
		if strings.HasSuffix(n.Go, "GetTypeID") && types[i].String() == "func() CFTypeID" {
			conv.getTypeIDs[n.Go[:len(n.Go)-9]] = true
		}
	}
	for i, n := range names {
		if types[i] == nil {
			continue
		}
		pos := f.NamePos[n]
		f, fok := types[i].(*dwarf.FuncType)
		if n.Kind != "type" && fok {
			n.Kind = "func"
			n.FuncType = conv.FuncType(f, pos)
		} else {
			n.Type = conv.Type(types[i], pos)
			switch n.Kind {
			case "iconst":
				if i < len(ints) {
					if _, ok := types[i].(*dwarf.UintType); ok {
						n.Const = fmt.Sprintf("%#x", uint64(ints[i]))
					} else {
						n.Const = fmt.Sprintf("%#x", ints[i])
					}
				}
			case "fconst":
				if i >= len(floats) {
					break
				}
				switch base(types[i]).(type) {
				case *dwarf.IntType, *dwarf.UintType:
					// This has an integer type so it's
					// not really a floating point
					// constant. This can happen when the
					// C compiler complains about using
					// the value as an integer constant,
					// but not as a general constant.
					// Treat this as a variable of the
					// appropriate type, not a constant,
					// to get C-style type handling,
					// avoiding the problem that C permits
					// uint64(-1) but Go does not.
					// See issue 26066.
					n.Kind = "var"
				default:
					n.Const = fmt.Sprintf("%f", floats[i])
				}
			case "sconst":
				if i < len(strs) {
					n.Const = fmt.Sprintf("%q", strs[i])
				}
			}
		}
		conv.FinishType(pos)
	}
}

type fileTypedefs struct {
	typedefs    map[string]bool // type names that appear in the types of the objects we're interested in
	typedefList []typedefInfo
}

// recordTypedefs remembers in ft.typedefs all the typedefs used in dtypes and its children.
func (ft *fileTypedefs) recordTypedefs(dtype dwarf.Type, pos token.Pos) {
	ft.recordTypedefs1(dtype, pos, map[dwarf.Type]bool{})
}

func (ft *fileTypedefs) recordTypedefs1(dtype dwarf.Type, pos token.Pos, visited map[dwarf.Type]bool) {
	if dtype == nil {
		return
	}
	if visited[dtype] {
		return
	}
	visited[dtype] = true
	switch dt := dtype.(type) {
	case *dwarf.TypedefType:
		if strings.HasPrefix(dt.Name, "__builtin") {
			// Don't look inside builtin types. There be dragons.
			return
		}
		if !ft.typedefs[dt.Name] {
			ft.typedefs[dt.Name] = true
			ft.typedefList = append(ft.typedefList, typedefInfo{dt.Name, pos})
			ft.recordTypedefs1(dt.Type, pos, visited)
		}
	case *dwarf.PtrType:
		ft.recordTypedefs1(dt.Type, pos, visited)
	case *dwarf.ArrayType:
		ft.recordTypedefs1(dt.Type, pos, visited)
	case *dwarf.QualType:
		ft.recordTypedefs1(dt.Type, pos, visited)
	case *dwarf.FuncType:
		ft.recordTypedefs1(dt.ReturnType, pos, visited)
		for _, a := range dt.ParamType {
			ft.recordTypedefs1(a, pos, visited)
		}
	case *dwarf.StructType:
		for _, l := range dt.Field {
			ft.recordTypedefs1(l.Type, pos, visited)
		}
	}
}

// prepareNames finalizes the Kind field of not-type names and sets
// the mangled name of all names.
func (p *Package) prepareNames(f *File) {
	for _, n := range f.Name {
		if n.Kind == "not-type" {
			if n.Define == "" {
				n.Kind = "var"
			} else {
				n.Kind = "macro"
				n.FuncType = &FuncType{
					Result: n.Type,
					Go: &ast.FuncType{
						Results: &ast.FieldList{List: []*ast.Field{{Type: n.Type.Go}}},
					},
				}
			}
		}
		p.mangleName(n)
		if n.Kind == "type" && typedef[n.Mangle] == nil {
			typedef[n.Mangle] = n.Type
		}
	}
}

// mangleName does name mangling to translate names
// from the original Go source files to the names
// used in the final Go files generated by cgo.
func (p *Package) mangleName(n *Name) {
	// When using gccgo variables have to be
	// exported so that they become global symbols
	// that the C code can refer to.
	prefix := "_C"
	if *gccgo && n.IsVar() {
		prefix = "C"
	}
	n.Mangle = prefix + n.Kind + "_" + n.Go
}

func (f *File) isMangledName(s string) bool {
	t, ok := strings.CutPrefix(s, "_C")
	if !ok {
		return false
	}
	return slices.ContainsFunc(nameKinds, func(k string) bool {
		return strings.HasPrefix(t, k+"_")
	})
}

// rewriteCalls rewrites all calls that pass pointers to check that
// they follow the rules for passing pointers between Go and C.
// This reports whether the package needs to import unsafe as _cgo_unsafe.
func (p *Package) rewriteCalls(f *File) bool {
	needsUnsafe := false
	// Walk backward so that in C.f1(C.f2()) we rewrite C.f2 first.
	for _, call := range f.Calls {
		if call.Done {
			continue
		}
		start := f.offset(call.Call.Pos())
		end := f.offset(call.Call.End())
		str, nu := p.rewriteCall(f, call)
		if str != "" {
			f.Edit.Replace(start, end, str)
			if nu {
				needsUnsafe = true
			}
		}
	}
	return needsUnsafe
}

// rewriteCall rewrites one call to add pointer checks.
// If any pointer checks are required, we rewrite the call into a
// function literal that calls _cgoCheckPointer for each pointer
// argument and then calls the original function.
// This returns the rewritten call and whether the package needs to
// import unsafe as _cgo_unsafe.
// If it returns the empty string, the call did not need to be rewritten.
func (p *Package) rewriteCall(f *File, call *Call) (string, bool) {
	// This is a call to C.xxx; set goname to "xxx".
	// It may have already been mangled by rewriteName.
	var goname string
	switch fun := call.Call.Fun.(type) {
	case *ast.SelectorExpr:
		goname = fun.Sel.Name
	case *ast.Ident:
		goname = strings.TrimPrefix(fun.Name, "_C2func_")
		goname = strings.TrimPrefix(goname, "_Cfunc_")
	}
	if goname == "" || goname == "malloc" {
		return "", false
	}
	name := f.Name[goname]
	if name == nil || name.Kind != "func" {
		// Probably a type conversion.
		return "", false
	}

	params := name.FuncType.Params
	args := call.Call.Args
	end := call.Call.End()

	// Avoid a crash if the number of arguments doesn't match
	// the number of parameters.
	// This will be caught when the generated file is compiled.
	if len(args) != len(params) {
		return "", false
	}

	any := false
	for i, param := range params {
		if p.needsPointerCheck(f, param.Go, args[i]) {
			any = true
			break
		}
	}
	if !any {
		return "", false
	}

	// We need to rewrite this call.
	//
	// Rewrite C.f(p) to
	//    func() {
	//            _cgo0 := p
	//            _cgoCheckPointer(_cgo0, nil)
	//            C.f(_cgo0)
	//    }()
	// Using a function literal like this lets us evaluate the
	// function arguments only once while doing pointer checks.
	// This is particularly useful when passing additional arguments
	// to _cgoCheckPointer, as done in checkIndex and checkAddr.
	//
	// When the function argument is a conversion to unsafe.Pointer,
	// we unwrap the conversion before checking the pointer,
	// and then wrap again when calling C.f. This lets us check
	// the real type of the pointer in some cases. See issue #25941.
	//
	// When the call to C.f is deferred, we use an additional function
	// literal to evaluate the arguments at the right time.
	//    defer func() func() {
	//            _cgo0 := p
	//            return func() {
	//                    _cgoCheckPointer(_cgo0, nil)
	//                    C.f(_cgo0)
	//            }
	//    }()()
	// This works because the defer statement evaluates the first
	// function literal in order to get the function to call.

	var sb bytes.Buffer
	sb.WriteString("func() ")
	if call.Deferred {
		sb.WriteString("func() ")
	}

	needsUnsafe := false
	result := false
	twoResults := false
	if !call.Deferred {
		// Check whether this call expects two results.
		for _, ref := range f.Ref {
			if ref.Expr != &call.Call.Fun {
				continue
			}
			if ref.Context == ctxCall2 {
				sb.WriteString("(")
				result = true
				twoResults = true
			}
			break
		}

		// Add the result type, if any.
		if name.FuncType.Result != nil {
			rtype := p.rewriteUnsafe(name.FuncType.Result.Go)
			if rtype != name.FuncType.Result.Go {
				needsUnsafe = true
			}
			sb.WriteString(gofmt(rtype))
			result = true
		}

		// Add the second result type, if any.
		if twoResults {
			if name.FuncType.Result == nil {
				// An explicit void result looks odd but it
				// seems to be how cgo has worked historically.
				sb.WriteString("_Ctype_void")
			}
			sb.WriteString(", error)")
		}
	}

	sb.WriteString("{ ")

	// Define _cgoN for each argument value.
	// Write _cgoCheckPointer calls to sbCheck.
	var sbCheck bytes.Buffer
	for i, param := range params {
		origArg := args[i]
		arg, nu := p.mangle(f, &args[i], true)
		if nu {
			needsUnsafe = true
		}

		// Use "var x T = ..." syntax to explicitly convert untyped
		// constants to the parameter type, to avoid a type mismatch.
		ptype := p.rewriteUnsafe(param.Go)

		if !p.needsPointerCheck(f, param.Go, args[i]) || param.BadPointer || p.checkUnsafeStringData(args[i]) {
			if ptype != param.Go {
				needsUnsafe = true
			}
			fmt.Fprintf(&sb, "var _cgo%d %s = %s; ", i,
				gofmt(ptype), gofmtPos(arg, origArg.Pos()))
			continue
		}

		// Check for &a[i].
		if p.checkIndex(&sb, &sbCheck, arg, i) {
			continue
		}

		// Check for &x.
		if p.checkAddr(&sb, &sbCheck, arg, i) {
			continue
		}

		// Check for a[:].
		if p.checkSlice(&sb, &sbCheck, arg, i) {
			continue
		}

		fmt.Fprintf(&sb, "_cgo%d := %s; ", i, gofmtPos(arg, origArg.Pos()))
		fmt.Fprintf(&sbCheck, "_cgoCheckPointer(_cgo%d, nil); ", i)
	}

	if call.Deferred {
		sb.WriteString("return func() { ")
	}

	// Write out the calls to _cgoCheckPointer.
	sb.WriteString(sbCheck.String())

	if result {
		sb.WriteString("return ")
	}

	m, nu := p.mangle(f, &call.Call.Fun, false)
	if nu {
		needsUnsafe = true
	}
	sb.WriteString(gofmtPos(m, end))

	sb.WriteString("(")
	for i := range params {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "_cgo%d", i)
	}
	sb.WriteString("); ")
	if call.Deferred {
		sb.WriteString("}")
	}
	sb.WriteString("}")
	if call.Deferred {
		sb.WriteString("()")
	}
	sb.WriteString("()")

	return sb.String(), needsUnsafe
}

// needsPointerCheck reports whether the type t needs a pointer check.
// This is true if t is a pointer and if the value to which it points
// might contain a pointer.
func (p *Package) needsPointerCheck(f *File, t ast.Expr, arg ast.Expr) bool {
	// An untyped nil does not need a pointer check, and when
	// _cgoCheckPointer returns the untyped nil the type assertion we
	// are going to insert will fail. Easier to just skip nil arguments.
	// TODO: Note that this fails if nil is shadowed.
	if id, ok := arg.(*ast.Ident); ok && id.Name == "nil" {
		return false
	}

	return p.hasPointer(f, t, true)
}

// hasPointer is used by needsPointerCheck. If top is true it returns
// whether t is or contains a pointer that might point to a pointer.
// If top is false it reports whether t is or contains a pointer.
// f may be nil.
func (p *Package) hasPointer(f *File, t ast.Expr, top bool) bool {
	switch t := t.(type) {
	case *ast.ArrayType:
		if t.Len == nil {
			if !top {
				return true
			}
			return p.hasPointer(f, t.Elt, false)
		}
		return p.hasPointer(f, t.Elt, top)
	case *ast.StructType:
		return slices.ContainsFunc(t.Fields.List, func(field *ast.Field) bool {
			return p.hasPointer(f, field.Type, top)
		})
	case *ast.StarExpr: // Pointer type.
		if !top {
			return true
		}
		// Check whether this is a pointer to a C union (or class)
		// type that contains a pointer.
		if unionWithPointer[t.X] {
			return true
		}
		return p.hasPointer(f, t.X, false)
	case *ast.FuncType, *ast.InterfaceType, *ast.MapType, *ast.ChanType:
		return true
	case *ast.Ident:
		// TODO: Handle types defined within function.
		for _, d := range p.Decl {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if ts.Name.Name == t.Name {
					return p.hasPointer(f, ts.Type, top)
				}
			}
		}
		if def := typedef[t.Name]; def != nil {
			return p.hasPointer(f, def.Go, top)
		}
		if t.Name == "string" {
			return !top
		}
		if t.Name == "error" {
			return true
		}
		if t.Name == "any" {
			return true
		}
		if goTypes[t.Name] != nil {
			return false
		}
		// We can't figure out the type. Conservative
		// approach is to assume it has a pointer.
		return true
	case *ast.SelectorExpr:
		if l, ok := t.X.(*ast.Ident); !ok || l.Name != "C" {
			// Type defined in a different package.
			// Conservative approach is to assume it has a
			// pointer.
			return true
		}
		if f == nil {
			// Conservative approach: assume pointer.
			return true
		}
		name := f.Name[t.Sel.Name]
		if name != nil && name.Kind == "type" && name.Type != nil && name.Type.Go != nil {
			return p.hasPointer(f, name.Type.Go, top)
		}
		// We can't figure out the type. Conservative
		// approach is to assume it has a pointer.
		return true
	default:
		error_(t.Pos(), "could not understand type %s", gofmt(t))
		return true
	}
}

// mangle replaces references to C names in arg with the mangled names,
// rewriting calls when it finds them.
// It removes the corresponding references in f.Ref and f.Calls, so that we
// don't try to do the replacement again in rewriteRef or rewriteCall.
// If addPosition is true, add position info to the idents of C names in arg.
func (p *Package) mangle(f *File, arg *ast.Expr, addPosition bool) (ast.Expr, bool) {
	needsUnsafe := false
	f.walk(arg, ctxExpr, func(f *File, arg any, context astContext) {
		px, ok := arg.(*ast.Expr)
		if !ok {
			return
		}
		sel, ok := (*px).(*ast.SelectorExpr)
		if ok {
			if l, ok := sel.X.(*ast.Ident); !ok || l.Name != "C" {
				return
			}

			for _, r := range f.Ref {
				if r.Expr == px {
					*px = p.rewriteName(f, r, addPosition)
					r.Done = true
					break
				}
			}

			return
		}

		call, ok := (*px).(*ast.CallExpr)
		if !ok {
			return
		}

		for _, c := range f.Calls {
			if !c.Done && c.Call.Lparen == call.Lparen {
				cstr, nu := p.rewriteCall(f, c)
				if cstr != "" {
					// Smuggle the rewritten call through an ident.
					*px = ast.NewIdent(cstr)
					if nu {
						needsUnsafe = true
					}
					c.Done = true
				}
			}
		}
	})
	return *arg, needsUnsafe
}

// checkIndex checks whether arg has the form &a[i], possibly inside
// type conversions. If so, then in the general case it writes
//
//	_cgoIndexNN := a
//	_cgoNN := &cgoIndexNN[i] // with type conversions, if any
//
// to sb, and writes
//
//	_cgoCheckPointer(_cgoNN, _cgoIndexNN)
//
// to sbCheck, and returns true. If a is a simple variable or field reference,
// it writes
//
//	_cgoIndexNN := &a
//
// and dereferences the uses of _cgoIndexNN. Taking the address avoids
// making a copy of an array.
//
// This tells _cgoCheckPointer to check the complete contents of the
// slice or array being indexed, but no other part of the memory allocation.
func (p *Package) checkIndex(sb, sbCheck *bytes.Buffer, arg ast.Expr, i int) bool {
	// Strip type conversions.
	x := arg
	for {
		c, ok := x.(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			break
		}
		if !p.isType(c.Fun) && !p.isUnsafeData(c.Fun, false) {
			break
		}
		x = c.Args[0]
	}
	u, ok := x.(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return false
	}
	index, ok := u.X.(*ast.IndexExpr)
	if !ok {
		return false
	}

	addr := ""
	deref := ""
	if p.isVariable(index.X) {
		addr = "&"
		deref = "*"
	}

	fmt.Fprintf(sb, "_cgoIndex%d := %s%s; ", i, addr, gofmtPos(index.X, index.X.Pos()))
	origX := index.X
	index.X = ast.NewIdent(fmt.Sprintf("_cgoIndex%d", i))
	if deref == "*" {
		index.X = &ast.StarExpr{X: index.X}
	}
	fmt.Fprintf(sb, "_cgo%d := %s; ", i, gofmtPos(arg, arg.Pos()))
	index.X = origX

	fmt.Fprintf(sbCheck, "_cgoCheckPointer(_cgo%d, %s_cgoIndex%d); ", i, deref, i)

	return true
}

// checkAddr checks whether arg has the form &x, possibly inside type
// conversions. If so, it writes
//
//	_cgoBaseNN := &x
//	_cgoNN := _cgoBaseNN // with type conversions, if any
//
// to sb, and writes
//
//	_cgoCheckPointer(_cgoBaseNN, true)
//
// to sbCheck, and returns true. This tells _cgoCheckPointer to check
// just the contents of the pointer being passed, not any other part
// of the memory allocation. This is run after checkIndex, which looks
// for the special case of &a[i], which requires different checks.
func (p *Package) checkAddr(sb, sbCheck *bytes.Buffer, arg ast.Expr, i int) bool {
	// Strip type conversions.
	px := &arg
	for {
		c, ok := (*px).(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			break
		}
		if !p.isType(c.Fun) && !p.isUnsafeData(c.Fun, false) {
			break
		}
		px = &c.Args[0]
	}
	if u, ok := (*px).(*ast.UnaryExpr); !ok || u.Op != token.AND {
		return false
	}

	fmt.Fprintf(sb, "_cgoBase%d := %s; ", i, gofmtPos(*px, (*px).Pos()))

	origX := *px
	*px = ast.NewIdent(fmt.Sprintf("_cgoBase%d", i))
	fmt.Fprintf(sb, "_cgo%d := %s; ", i, gofmtPos(arg, arg.Pos()))
	*px = origX

	// Use "0 == 0" to do the right thing in the unlikely event
	// that "true" is shadowed.
	fmt.Fprintf(sbCheck, "_cgoCheckPointer(_cgoBase%d, 0 == 0); ", i)

	return true
}

// checkSlice checks whether arg has the form x[i:j], possibly inside
// type conversions. If so, it writes
//
//	_cgoSliceNN := x[i:j]
//	_cgoNN := _cgoSliceNN // with type conversions, if any
//
// to sb, and writes
//
//	_cgoCheckPointer(_cgoSliceNN, true)
//
// to sbCheck, and returns true. This tells _cgoCheckPointer to check
// just the contents of the slice being passed, not any other part
// of the memory allocation.
func (p *Package) checkSlice(sb, sbCheck *bytes.Buffer, arg ast.Expr, i int) bool {
	// Strip type conversions.
	px := &arg
	for {
		c, ok := (*px).(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			break
		}
		if !p.isType(c.Fun) && !p.isUnsafeData(c.Fun, false) {
			break
		}
		px = &c.Args[0]
	}
	if _, ok := (*px).(*ast.SliceExpr); !ok {
		return false
	}

	fmt.Fprintf(sb, "_cgoSlice%d := %s; ", i, gofmtPos(*px, (*px).Pos()))

	origX := *px
	*px = ast.NewIdent(fmt.Sprintf("_cgoSlice%d", i))
	fmt.Fprintf(sb, "_cgo%d := %s; ", i, gofmtPos(arg, arg.Pos()))
	*px = origX

	// Use 0 == 0 to do the right thing in the unlikely event
	// that "true" is shadowed.
	fmt.Fprintf(sbCheck, "_cgoCheckPointer(_cgoSlice%d, 0 == 0); ", i)

	return true
}

// checkUnsafeStringData checks for a call to unsafe.StringData.
// The result of that call can't contain a pointer so there is
// no need to call _cgoCheckPointer.
func (p *Package) checkUnsafeStringData(arg ast.Expr) bool {
	x := arg
	for {
		c, ok := x.(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			break
		}
		if p.isUnsafeData(c.Fun, true) {
			return true
		}
		if !p.isType(c.Fun) {
			break
		}
		x = c.Args[0]
	}
	return false
}

// isType reports whether the expression is definitely a type.
// This is conservative--it returns false for an unknown identifier.
func (p *Package) isType(t ast.Expr) bool {
	switch t := t.(type) {
	case *ast.SelectorExpr:
		id, ok := t.X.(*ast.Ident)
		if !ok {
			return false
		}
		if id.Name == "unsafe" && t.Sel.Name == "Pointer" {
			return true
		}
		if id.Name == "C" && typedef["_Ctype_"+t.Sel.Name] != nil {
			return true
		}
		return false
	case *ast.Ident:
		// TODO: This ignores shadowing.
		switch t.Name {
		case "unsafe.Pointer", "bool", "byte",
			"complex64", "complex128",
			"error",
			"float32", "float64",
			"int", "int8", "int16", "int32", "int64",
			"rune", "string",
			"uint", "uint8", "uint16", "uint32", "uint64", "uintptr":

			return true
		}
		if strings.HasPrefix(t.Name, "_Ctype_") {
			return true
		}
	case *ast.ParenExpr:
		return p.isType(t.X)
	case *ast.StarExpr:
		return p.isType(t.X)
	case *ast.ArrayType, *ast.StructType, *ast.FuncType, *ast.InterfaceType,
		*ast.MapType, *ast.ChanType:

		return true
	}
	return false
}

// isUnsafeData reports whether the expression is unsafe.StringData
// or unsafe.SliceData. We can ignore these when checking for pointers
// because they don't change whether or not their argument contains
// any Go pointers. If onlyStringData is true we only check for StringData.
func (p *Package) isUnsafeData(x ast.Expr, onlyStringData bool) bool {
	st, ok := x.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := st.X.(*ast.Ident)
	if !ok {
		return false
	}
	if id.Name != "unsafe" {
		return false
	}
	if !onlyStringData && st.Sel.Name == "SliceData" {
		return true
	}
	return st.Sel.Name == "StringData"
}

// isVariable reports whether x is a variable, possibly with field references.
func (p *Package) isVariable(x ast.Expr) bool {
	switch x := x.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return p.isVariable(x.X)
	case *ast.IndexExpr:
		return true
	}
	return false
}

// rewriteUnsafe returns a version of t with references to unsafe.Pointer
// rewritten to use _cgo_unsafe.Pointer instead.
func (p *Package) rewriteUnsafe(t ast.Expr) ast.Expr {
	switch t := t.(type) {
	case *ast.Ident:
		// We don't see a SelectorExpr for unsafe.Pointer;
		// this is created by code in this file.
		if t.Name == "unsafe.Pointer" {
			return ast.NewIdent("_cgo_unsafe.Pointer")
		}
	case *ast.ArrayType:
		t1 := p.rewriteUnsafe(t.Elt)
		if t1 != t.Elt {
			r := *t
			r.Elt = t1
			return &r
		}
	case *ast.StructType:
		changed := false
		fields := *t.Fields
		fields.List = nil
		for _, f := range t.Fields.List {
			ft := p.rewriteUnsafe(f.Type)
			if ft == f.Type {
				fields.List = append(fields.List, f)
			} else {
				fn := *f
				fn.Type = ft
				fields.List = append(fields.List, &fn)
				changed = true
			}
		}
		if changed {
			r := *t
			r.Fields = &fields
			return &r
		}
	case *ast.StarExpr: // Pointer type.
		x1 := p.rewriteUnsafe(t.X)
		if x1 != t.X {
			r := *t
			r.X = x1
			return &r
		}
	}
	return t
}

// rewriteRef rewrites all the C.xxx references in f.AST to refer to the
// Go equivalents, now that we have figured out the meaning of all
// the xxx. In *godefs mode, rewriteRef replaces the names
// with full definitions instead of mangled names.
func (p *Package) rewriteRef(f *File) {
	// Keep a list of all the functions, to remove the ones
	// only used as expressions and avoid generating bridge
	// code for them.
	functions := make(map[string]bool)

	for _, n := range f.Name {
		if n.Kind == "func" {
			functions[n.Go] = false
		}
	}

	// Now that we have all the name types filled in,
	// scan through the Refs to identify the ones that
	// are trying to do a ,err call. Also check that
	// functions are only used in calls.
	for _, r := range f.Ref {
		if r.Name.IsConst() && r.Name.Const == "" {
			error_(r.Pos(), "unable to find value of constant C.%s", fixGo(r.Name.Go))
		}

		if r.Name.Kind == "func" {
			switch r.Context {
			case ctxCall, ctxCall2:
				functions[r.Name.Go] = true
			}
		}

		expr := p.rewriteName(f, r, false)

		if *godefs {
			// Substitute definition for mangled type name.
			if r.Name.Type != nil && r.Name.Kind == "type" {
				expr = r.Name.Type.Go
			}
			if id, ok := expr.(*ast.Ident); ok {
				if t := typedef[id.Name]; t != nil {
					expr = t.Go
				}
				if id.Name == r.Name.Mangle && r.Name.Const != "" {
					expr = ast.NewIdent(r.Name.Const)
				}
			}
		}

		// Copy position information from old expr into new expr,
		// in case expression being replaced is first on line.
		// See golang.org/issue/6563.
		pos := (*r.Expr).Pos()
		if x, ok := expr.(*ast.Ident); ok {
			expr = &ast.Ident{NamePos: pos, Name: x.Name}
		}

		// Change AST, because some later processing depends on it,
		// and also because -godefs mode still prints the AST.
		old := *r.Expr
		*r.Expr = expr

		// Record source-level edit for cgo output.
		if !r.Done {
			// Prepend a space in case the earlier code ends
			// with '/', which would give us a "//" comment.
			repl := " " + gofmtPos(expr, old.Pos())
			end := fset.Position(old.End())
			// Subtract 1 from the column if we are going to
			// append a close parenthesis. That will set the
			// correct column for the following characters.
			sub := 0
			if r.Name.Kind != "type" {
				sub = 1
			}
			if end.Column > sub {
				repl = fmt.Sprintf("%s /*line :%d:%d*/", repl, end.Line, end.Column-sub)
			}
			if r.Name.Kind != "type" {
				repl = "(" + repl + ")"
			}
			f.Edit.Replace(f.offset(old.Pos()), f.offset(old.End()), repl)
		}
	}

	// Remove functions only used as expressions, so their respective
	// bridge functions are not generated.
	for name, used := range functions {
		if !used {
			delete(f.Name, name)
		}
	}
}

// rewriteName returns the expression used to rewrite a reference.
// If addPosition is true, add position info in the ident name.
func (p *Package) rewriteName(f *File, r *Ref, addPosition bool) ast.Expr {
	getNewIdent := ast.NewIdent
	if addPosition {
		getNewIdent = func(newName string) *ast.Ident {
			mangledIdent := ast.NewIdent(newName)
			if len(newName) == len(r.Name.Go) {
				return mangledIdent
			}
			p := fset.Position((*r.Expr).End())
			if p.Column == 0 {
				return mangledIdent
			}
			return ast.NewIdent(fmt.Sprintf("%s /*line :%d:%d*/", newName, p.Line, p.Column))
		}
	}
	var expr ast.Expr = getNewIdent(r.Name.Mangle) // default
	switch r.Context {
	case ctxCall, ctxCall2:
		if r.Name.Kind != "func" {
			if r.Name.Kind == "type" {
				r.Context = ctxType
				if r.Name.Type == nil {
					error_(r.Pos(), "invalid conversion to C.%s: undefined C type '%s'", fixGo(r.Name.Go), r.Name.C)
				}
				break
			}
			error_(r.Pos(), "call of non-function C.%s", fixGo(r.Name.Go))
			break
		}
		if r.Context == ctxCall2 {
			if builtinDefs[r.Name.Go] != "" {
				error_(r.Pos(), "no two-result form for C.%s", r.Name.Go)
				break
			}
			// Invent new Name for the two-result function.
			n := f.Name["2"+r.Name.Go]
			if n == nil {
				n = new(Name)
				*n = *r.Name
				n.AddError = true
				n.Mangle = "_C2func_" + n.Go
				f.Name["2"+r.Name.Go] = n
			}
			expr = getNewIdent(n.Mangle)
			r.Name = n
			break
		}
	case ctxExpr:
		switch r.Name.Kind {
		case "func":
			if builtinDefs[r.Name.C] != "" {
				error_(r.Pos(), "use of builtin '%s' not in function call", fixGo(r.Name.C))
			}

			// Function is being used in an expression, to e.g. pass around a C function pointer.
			// Create a new Name for this Ref which causes the variable to be declared in Go land.
			fpName := "fp_" + r.Name.Go
			name := f.Name[fpName]
			if name == nil {
				name = &Name{
					Go:   fpName,
					C:    r.Name.C,
					Kind: "fpvar",
					Type: &Type{Size: p.PtrSize, Align: p.PtrSize, C: c("void*"), Go: ast.NewIdent("unsafe.Pointer")},
				}
				p.mangleName(name)
				f.Name[fpName] = name
			}
			r.Name = name
			// Rewrite into call to _Cgo_ptr to prevent assignments. The _Cgo_ptr
			// function is defined in out.go and simply returns its argument. See
			// issue 7757.
			expr = &ast.CallExpr{
				Fun:  &ast.Ident{NamePos: (*r.Expr).Pos(), Name: "_Cgo_ptr"},
				Args: []ast.Expr{getNewIdent(name.Mangle)},
			}
		case "type":
			// Okay - might be new(T), T(x), Generic[T], etc.
			if r.Name.Type == nil {
				error_(r.Pos(), "expression C.%s: undefined C type '%s'", fixGo(r.Name.Go), r.Name.C)
			}
		case "var":
			expr = &ast.StarExpr{Star: (*r.Expr).Pos(), X: expr}
		case "macro":
			expr = &ast.CallExpr{Fun: expr}
		}
	case ctxSelector:
		if r.Name.Kind == "var" {
			expr = &ast.StarExpr{Star: (*r.Expr).Pos(), X: expr}
		} else {
			error_(r.Pos(), "only C variables allowed in selector expression %s", fixGo(r.Name.Go))
		}
	case ctxType:
		if r.Name.Kind != "type" {
			error_(r.Pos(), "expression C.%s used as type", fixGo(r.Name.Go))
		} else if r.Name.Type == nil {
			// Use of C.enum_x, C.struct_x or C.union_x without C definition.
			// GCC won't raise an error when using pointers to such unknown types.
			error_(r.Pos(), "type C.%s: undefined C type '%s'", fixGo(r.Name.Go), r.Name.C)
		}
	default:
		if r.Name.Kind == "func" {
			error_(r.Pos(), "must call C.%s", fixGo(r.Name.Go))
		}
	}
	return expr
}

// gofmtPos returns the gofmt-formatted string for an AST node,
// with a comment setting the position before the node.
func gofmtPos(n ast.Expr, pos token.Pos) string {
	s := gofmt(n)
	p := fset.Position(pos)
	if p.Column == 0 {
		return s
	}
	return fmt.Sprintf("/*line :%d:%d*/%s", p.Line, p.Column, s)
}

// checkGCCBaseCmd returns the start of the compiler command line.
// It uses $CC if set, or else $GCC, or else the compiler recorded
// during the initial build as defaultCC.
// defaultCC is defined in zdefaultcc.go, written by cmd/dist.
//
// The compiler command line is split into arguments on whitespace. Quotes
// are understood, so arguments may contain whitespace.
//
// checkGCCBaseCmd confirms that the compiler exists in PATH, returning
// an error if it does not.
func checkGCCBaseCmd() ([]string, error) {
	// Use $CC if set, since that's what the build uses.
	value := os.Getenv("CC")
	if value == "" {
		// Try $GCC if set, since that's what we used to use.
		value = os.Getenv("GCC")
	}
	if value == "" {
		value = defaultCC(goos, goarch)
	}
	args, err := quoted.Split(value)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, errors.New("CC not set and no default found")
	}
	if _, err := exec.LookPath(args[0]); err != nil {
		return nil, fmt.Errorf("C compiler %q not found: %v", args[0], err)
	}
	return args[:len(args):len(args)], nil
}

// gccMachine returns the gcc -m flag to use, either "-m32", "-m64" or "-marm".
func gccMachine() []string {
	switch goarch {
	case "amd64":
		if goos == "darwin" {
			return []string{"-arch", "x86_64", "-m64"}
		}
		return []string{"-m64"}
	case "arm64":
		if goos == "darwin" {
			return []string{"-arch", "arm64"}
		}
	case "386":
		return []string{"-m32"}
	case "arm":
		return []string{"-marm"} // not thumb
	case "s390":
		return []string{"-m31"}
	case "s390x":
		return []string{"-m64"}
	case "mips64", "mips64le":
		if gomips64 == "hardfloat" {
			return []string{"-mabi=64", "-mhard-float"}
		} else if gomips64 == "softfloat" {
			return []string{"-mabi=64", "-msoft-float"}
		}
	case "mips", "mipsle":
		if gomips == "hardfloat" {
			return []string{"-mabi=32", "-mfp32", "-mhard-float", "-mno-odd-spreg"}
		} else if gomips == "softfloat" {
			return []string{"-mabi=32", "-msoft-float"}
		}
	case "loong64":
		return []string{"-mabi=lp64d"}
	}
	return nil
}

var n atomic.Int64

func gccTmp() string {
	c := strconv.Itoa(int(n.Add(1)))
	return filepath.Join(outputDir(), "_cgo_"+c+".o")
}

// gccCmd returns the gcc command line to use for compiling
// the input.
// gccCommand is called concurrently for different files.
func (p *Package) gccCmd(ofile string) []string {
	c := append(gccBaseCmd,
		"-w",         // no warnings
		"-Wno-error", // warnings are not errors
		"-o"+ofile,   // write object to tmp
		"-gdwarf-2",  // generate DWARF v2 debugging symbols
		"-c",         // do not link
		"-xc",        // input language is C
	)
	if p.GccIsClang {
		c = append(c,
			"-ferror-limit=0",
			// Apple clang version 1.7 (tags/Apple/clang-77) (based on LLVM 2.9svn)
			// doesn't have -Wno-unneeded-internal-declaration, so we need yet another
			// flag to disable the warning. Yes, really good diagnostics, clang.
			"-Wno-unknown-warning-option",
			"-Wno-unneeded-internal-declaration",
			"-Wno-unused-function",
			"-Qunused-arguments",
			// Clang embeds prototypes for some builtin functions,
			// like malloc and calloc, but all size_t parameters are
			// incorrectly typed unsigned long. We work around that
			// by disabling the builtin functions (this is safe as
			// it won't affect the actual compilation of the C code).
			// See: https://golang.org/issue/6506.
			"-fno-builtin",
		)
	}

	c = append(c, p.GccOptions...)
	c = append(c, gccMachine()...)
	if goos == "aix" {
		c = append(c, "-maix64")
		c = append(c, "-mcmodel=large")
	}
	// disable LTO so we get an object whose symbols we can read
	c = append(c, "-fno-lto")
	c = append(c, "-") //read input from standard input
	return c
}

// gccDebug runs gcc -gdwarf-2 over the C program stdin and
// returns the corresponding DWARF data and, if present, debug data block.
// gccDebug is called concurrently with different C programs.
func (p *Package) gccDebug(stdin []byte, nnames int) (d *dwarf.Data, ints []int64, floats []float64, strs []string) {
	ofile := gccTmp()
	runGcc(stdin, p.gccCmd(ofile))

	isDebugInts := func(s string) bool {
		// Some systems use leading _ to denote non-assembly symbols.
		return s == "__cgodebug_ints" || s == "___cgodebug_ints"
	}
	isDebugFloats := func(s string) bool {
		// Some systems use leading _ to denote non-assembly symbols.
		return s == "__cgodebug_floats" || s == "___cgodebug_floats"
	}
	indexOfDebugStr := func(s string) int {
		// Some systems use leading _ to denote non-assembly symbols.
		if strings.HasPrefix(s, "___") {
			s = s[1:]
		}
		if strings.HasPrefix(s, "__cgodebug_str__") {
			if n, err := strconv.Atoi(s[len("__cgodebug_str__"):]); err == nil {
				return n
			}
		}
		return -1
	}
	indexOfDebugStrlen := func(s string) int {
		// Some systems use leading _ to denote non-assembly symbols.
		if strings.HasPrefix(s, "___") {
			s = s[1:]
		}
		if t, ok := strings.CutPrefix(s, "__cgodebug_strlen__"); ok {
			if n, err := strconv.Atoi(t); err == nil {
				return n
			}
		}
		return -1
	}

	strs = make([]string, nnames)

	strdata := make(map[int]string, nnames)
	strlens := make(map[int]int, nnames)

	buildStrings := func() {
		for n, strlen := range strlens {
			data := strdata[n]
			if len(data) <= strlen {
				fatalf("invalid string literal")
			}
			strs[n] = data[:strlen]
		}
	}

	if f, err := macho.Open(ofile); err == nil {
		defer f.Close()
		d, err := f.DWARF()
		if err != nil {
			fatalf("cannot load DWARF output from %s: %v", ofile, err)
		}
		bo := f.ByteOrder
		if f.Symtab != nil {
			for i := range f.Symtab.Syms {
				s := &f.Symtab.Syms[i]
				switch {
				case isDebugInts(s.Name):
					// Found it. Now find data section.
					if i := int(s.Sect) - 1; 0 <= i && i < len(f.Sections) {
						sect := f.Sections[i]
						if sect.Addr <= s.Value && s.Value < sect.Addr+sect.Size {
							if sdat, err := sect.Data(); err == nil {
								data := sdat[s.Value-sect.Addr:]
								ints = make([]int64, len(data)/8)
								for i := range ints {
									ints[i] = int64(bo.Uint64(data[i*8:]))
								}
							}
						}
					}
				case isDebugFloats(s.Name):
					// Found it. Now find data section.
					if i := int(s.Sect) - 1; 0 <= i && i < len(f.Sections) {
						sect := f.Sections[i]
						if sect.Addr <= s.Value && s.Value < sect.Addr+sect.Size {
							if sdat, err := sect.Data(); err == nil {
								data := sdat[s.Value-sect.Addr:]
								floats = make([]float64, len(data)/8)
								for i := range floats {
									floats[i] = math.Float64frombits(bo.Uint64(data[i*8:]))
								}
							}
						}
					}
				default:
					if n := indexOfDebugStr(s.Name); n != -1 {
						// Found it. Now find data section.
						if i := int(s.Sect) - 1; 0 <= i && i < len(f.Sections) {
							sect := f.Sections[i]
							if sect.Addr <= s.Value && s.Value < sect.Addr+sect.Size {
								if sdat, err := sect.Data(); err == nil {
									data := sdat[s.Value-sect.Addr:]
									strdata[n] = string(data)
								}
							}
						}
						break
					}
					if n := indexOfDebugStrlen(s.Name); n != -1 {
						// Found it. Now find data section.
						if i := int(s.Sect) - 1; 0 <= i && i < len(f.Sections) {
							sect := f.Sections[i]
							if sect.Addr <= s.Value && s.Value < sect.Addr+sect.Size {
								if sdat, err := sect.Data(); err == nil {
									data := sdat[s.Value-sect.Addr:]
									strlen := bo.Uint64(data[:8])
									if strlen > (1<<(uint(p.IntSize*8)-1) - 1) { // greater than MaxInt?
										fatalf("string literal too big")
									}
									strlens[n] = int(strlen)
								}
							}
						}
						break
					}
				}
			}

			buildStrings()
		}
		return d, ints, floats, strs
	}

	if f, err := elf.Open(ofile); err == nil {
		defer f.Close()
		d, err := f.DWARF()
		if err != nil {
			fatalf("cannot load DWARF output from %s: %v", ofile, err)
		}
		bo := f.ByteOrder
		symtab, err := f.Symbols()
		if err == nil {
			// Check for use of -fsanitize=hwaddress (issue 53285).
			removeTag := func(v uint64) uint64 { return v }
			if goarch == "arm64" {
				for i := range symtab {
					if symtab[i].Name == "__hwasan_init" {
						// -fsanitize=hwaddress on ARM
						// uses the upper byte of a
						// memory address as a hardware
						// tag. Remove it so that
						// we can find the associated
						// data.
						removeTag = func(v uint64) uint64 { return v &^ (0xff << (64 - 8)) }
						break
					}
				}
			}

			for i := range symtab {
				s := &symtab[i]
				switch {
				case isDebugInts(s.Name):
					// Found it. Now find data section.
					if i := int(s.Section); 0 <= i && i < len(f.Sections) {
						sect := f.Sections[i]
						val := removeTag(s.Value)
						if sect.Addr <= val && val < sect.Addr+sect.Size {
							if sdat, err := sect.Data(); err == nil {
								data := sdat[val-sect.Addr:]
								ints = make([]int64, len(data)/8)
								for i := range ints {
									ints[i] = int64(bo.Uint64(data[i*8:]))
								}
							}
						}
					}
				case isDebugFloats(s.Name):
					// Found it. Now find data section.
					if i := int(s.Section); 0 <= i && i < len(f.Sections) {
						sect := f.Sections[i]
						val := removeTag(s.Value)
						if sect.Addr <= val && val < sect.Addr+sect.Size {
							if sdat, err := sect.Data(); err == nil {
								data := sdat[val-sect.Addr:]
								floats = make([]float64, len(data)/8)
								for i := range floats {
									floats[i] = math.Float64frombits(bo.Uint64(data[i*8:]))
								}
							}
						}
					}
				default:
					if n := indexOfDebugStr(s.Name); n != -1 {
						// Found it. Now find data section.
						if i := int(s.Section); 0 <= i && i < len(f.Sections) {
							sect := f.Sections[i]
							val := removeTag(s.Value)
							if sect.Addr <= val && val < sect.Addr+sect.Size {
								if sdat, err := sect.Data(); err == nil {
									data := sdat[val-sect.Addr:]
									strdata[n] = string(data)
								}
							}
						}
						break
					}
					if n := indexOfDebugStrlen(s.Name); n != -1 {
						// Found it. Now find data section.
						if i := int(s.Section); 0 <= i && i < len(f.Sections) {
							sect := f.Sections[i]
							val := removeTag(s.Value)
							if sect.Addr <= val && val < sect.Addr+sect.Size {
								if sdat, err := sect.Data(); err == nil {
									data := sdat[val-sect.Addr:]
									strlen := bo.Uint64(data[:8])
									if strlen > (1<<(uint(p.IntSize*8)-1) - 1) { // greater than MaxInt?
										fatalf("string literal too big")
									}
									strlens[n] = int(strlen)
								}
							}
						}
						break
					}
				}
			}

			buildStrings()
		}
		return d, ints, floats, strs
	}

	if f, err := pe.Open(ofile); err == nil {
		defer f.Close()
		d, err := f.DWARF()
		if err != nil {
			fatalf("cannot load DWARF output from %s: %v", ofile, err)
		}
		bo := binary.LittleEndian
		for _, s := range f.Symbols {
			switch {
			case isDebugInts(s.Name):
				if i := int(s.SectionNumber) - 1; 0 <= i && i < len(f.Sections) {
					sect := f.Sections[i]
					if s.Value < sect.Size {
						if sdat, err := sect.Data(); err == nil {
							data := sdat[s.Value:]
							ints = make([]int64, len(data)/8)
							for i := range ints {
								ints[i] = int64(bo.Uint64(data[i*8:]))
							}
						}
					}
				}
			case isDebugFloats(s.Name):
				if i := int(s.SectionNumber) - 1; 0 <= i && i < len(f.Sections) {
					sect := f.Sections[i]
					if s.Value < sect.Size {
						if sdat, err := sect.Data(); err == nil {
							data := sdat[s.Value:]
							floats = make([]float64, len(data)/8)
							for i := range floats {
								floats[i] = math.Float64frombits(bo.Uint64(data[i*8:]))
							}
						}
					}
				}
			default:
				if n := indexOfDebugStr(s.Name); n != -1 {
					if i := int(s.SectionNumber) - 1; 0 <= i && i < len(f.Sections) {
						sect := f.Sections[i]
						if s.Value < sect.Size {
							if sdat, err := sect.Data(); err == nil {
								data := sdat[s.Value:]
								strdata[n] = string(data)
							}
						}
					}
					break
				}
				if n := indexOfDebugStrlen(s.Name); n != -1 {
					if i := int(s.SectionNumber) - 1; 0 <= i && i < len(f.Sections) {
						sect := f.Sections[i]
						if s.Value < sect.Size {
							if sdat, err := sect.Data(); err == nil {
								data := sdat[s.Value:]
								strlen := bo.Uint64(data[:8])
								if strlen > (1<<(uint(p.IntSize*8)-1) - 1) { // greater than MaxInt?
									fatalf("string literal too big")
								}
								strlens[n] = int(strlen)
							}
						}
					}
					break
				}
			}
		}

		buildStrings()

		return d, ints, floats, strs
	}

	if f, err := xcoff.Open(ofile); err == nil {
		defer f.Close()
		d, err := f.DWARF()
		if err != nil {
			fatalf("cannot load DWARF output from %s: %v", ofile, err)
		}
		bo := binary.BigEndian
		for _, s := range f.Symbols {
			switch {
			case isDebugInts(s.Name):
				if i := s.SectionNumber - 1; 0 <= i && i < len(f.Sections) {
					sect := f.Sections[i]
					if s.Value < sect.Size {
						if sdat, err := sect.Data(); err == nil {
							data := sdat[s.Value:]
							ints = make([]int64, len(data)/8)
							for i := range ints {
								ints[i] = int64(bo.Uint64(data[i*8:]))
							}
						}
					}
				}
			case isDebugFloats(s.Name):
				if i := s.SectionNumber - 1; 0 <= i && i < len(f.Sections) {
					sect := f.Sections[i]
					if s.Value < sect.Size {
						if sdat, err := sect.Data(); err == nil {
							data := sdat[s.Value:]
							floats = make([]float64, len(data)/8)
							for i := range floats {
								floats[i] = math.Float64frombits(bo.Uint64(data[i*8:]))
							}
						}
					}
				}
			default:
				if n := indexOfDebugStr(s.Name); n != -1 {
					if i := s.SectionNumber - 1; 0 <= i && i < len(f.Sections) {
						sect := f.Sections[i]
						if s.Value < sect.Size {
							if sdat, err := sect.Data(); err == nil {
								data := sdat[s.Value:]
								strdata[n] = string(data)
							}
						}
					}
					break
				}
				if n := indexOfDebugStrlen(s.Name); n != -1 {
					if i := s.SectionNumber - 1; 0 <= i && i < len(f.Sections) {
						sect := f.Sections[i]
						if s.Value < sect.Size {
							if sdat, err := sect.Data(); err == nil {
								data := sdat[s.Value:]
								strlen := bo.Uint64(data[:8])
								if strlen > (1<<(uint(p.IntSize*8)-1) - 1) { // greater than MaxInt?
									fatalf("string literal too big")
								}
								strlens[n] = int(strlen)
							}
						}
					}
					break
				}
			}
		}

		buildStrings()
		return d, ints, floats, strs
	}
	fatalf("cannot parse gcc output %s as ELF, Mach-O, PE, XCOFF object", ofile)
	panic("not reached")
}

// gccDefines runs gcc -E -dM -xc - over the C program stdin
// and returns the corresponding standard output, which is the
// #defines that gcc encountered while processing the input
// and its included files.
func gccDefines(stdin []byte, gccOptions []string) string {
	base := append(gccBaseCmd, "-E", "-dM", "-xc")
	base = append(base, gccMachine()...)
	stdout, _ := runGcc(stdin, append(append(base, gccOptions...), "-"))
	return stdout
}

// gccErrors runs gcc over the C program stdin and returns
// the errors that gcc prints. That is, this function expects
// gcc to fail.
// gccErrors is called concurrently with different C programs.
func (p *Package) gccErrors(stdin []byte, extraArgs ...string) string {
	// TODO(rsc): require failure
	args := p.gccCmd(gccTmp())

	// Optimization options can confuse the error messages; remove them.
	nargs := make([]string, 0, len(args)+len(extraArgs))
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-O") {
			nargs = append(nargs, arg)
		}
	}

	// Force -O0 optimization and append extra arguments, but keep the
	// trailing "-" at the end.
	li := len(nargs) - 1
	last := nargs[li]
	nargs[li] = "-O0"
	nargs = append(nargs, extraArgs...)
	nargs = append(nargs, last)

	if *debugGcc {
		fmt.Fprintf(os.Stderr, "$ %s <<EOF\n", strings.Join(nargs, " "))
		os.Stderr.Write(stdin)
		fmt.Fprint(os.Stderr, "EOF\n")
	}
	stdout, stderr, _ := run(stdin, nargs)
	if *debugGcc {
		os.Stderr.Write(stdout)
		os.Stderr.Write(stderr)
	}
	return string(stderr)
}

// runGcc runs the gcc command line args with stdin on standard input.
// If the command exits with a non-zero exit status, runGcc prints
// details about what was run and exits.
// Otherwise runGcc returns the data written to standard output and standard error.
// Note that for some of the uses we expect useful data back
// on standard error, but for those uses gcc must still exit 0.
func runGcc(stdin []byte, args []string) (string, string) {
	if *debugGcc {
		fmt.Fprintf(os.Stderr, "$ %s <<EOF\n", strings.Join(args, " "))
		os.Stderr.Write(stdin)
		fmt.Fprint(os.Stderr, "EOF\n")
	}
	stdout, stderr, ok := run(stdin, args)
	if *debugGcc {
		os.Stderr.Write(stdout)
		os.Stderr.Write(stderr)
	}
	if !ok {
		os.Stderr.Write(stderr)
		os.Exit(2)
	}
	return string(stdout), string(stderr)
}

// A typeConv is a translator from dwarf types to Go types
// with equivalent memory layout.
type typeConv struct {
	// Cache of already-translated or in-progress types.
	m map[string]*Type

	// Map from types to incomplete pointers to those types.
	ptrs map[string][]*Type
	// Keys of ptrs in insertion order (deterministic worklist)
	// ptrKeys contains exactly the keys in ptrs.
	ptrKeys []dwarf.Type

	// Type names X for which there exists an XGetTypeID function with type func() CFTypeID.
	getTypeIDs map[string]bool

	// incompleteStructs contains C structs that should be marked Incomplete.
	incompleteStructs map[string]bool

	// Predeclared types.
	bool                                   ast.Expr
	byte                                   ast.Expr // denotes padding
	int8, int16, int32, int64              ast.Expr
	uint8, uint16, uint32, uint64, uintptr ast.Expr
	float32, float64                       ast.Expr
	complex64, complex128                  ast.Expr
	void                                   ast.Expr
	string                                 ast.Expr
	goVoid                                 ast.Expr // _Ctype_void, denotes C's void
	goVoidPtr                              ast.Expr // unsafe.Pointer or *byte

	ptrSize int64
	intSize int64
}

var tagGen int
var typedef = make(map[string]*Type)
var goIdent = make(map[string]*ast.Ident)

// unionWithPointer is true for a Go type that represents a C union (or class)
// that may contain a pointer. This is used for cgo pointer checking.
var unionWithPointer = make(map[ast.Expr]bool)

// anonymousStructTag provides a consistent tag for an anonymous struct.
// The same dwarf.StructType pointer will always get the same tag.
var anonymousStructTag = make(map[*dwarf.StructType]string)

func (c *typeConv) Init(ptrSize, intSize int64) {
	c.ptrSize = ptrSize
	c.intSize = intSize
	c.m = make(map[string]*Type)
	c.ptrs = make(map[string][]*Type)
	c.getTypeIDs = make(map[string]bool)
	c.incompleteStructs = make(map[string]bool)
	c.bool = c.Ident("bool")
	c.byte = c.Ident("byte")
	c.int8 = c.Ident("int8")
	c.int16 = c.Ident("int16")
	c.int32 = c.Ident("int32")
	c.int64 = c.Ident("int64")
	c.uint8 = c.Ident("uint8")
	c.uint16 = c.Ident("uint16")
	c.uint32 = c.Ident("uint32")
	c.uint64 = c.Ident("uint64")
	c.uintptr = c.Ident("uintptr")
	c.float32 = c.Ident("float32")
	c.float64 = c.Ident("float64")
	c.complex64 = c.Ident("complex64")
	c.complex128 = c.Ident("complex128")
	c.void = c.Ident("void")
	c.string = c.Ident("string")
	c.goVoid = c.Ident("_Ctype_void")

	// Normally cgo translates void* to unsafe.Pointer,
	// but for historical reasons -godefs uses *byte instead.
	if *godefs {
		c.goVoidPtr = &ast.StarExpr{X: c.byte}
	} else {
		c.goVoidPtr = c.Ident("unsafe.Pointer")
	}
}

// base strips away qualifiers and typedefs to get the underlying type.
func base(dt dwarf.Type) dwarf.Type {
	for {
		if d, ok := dt.(*dwarf.QualType); ok {
			dt = d.Type
			continue
		}
		if d, ok := dt.(*dwarf.TypedefType); ok {
			dt = d.Type
			continue
		}
		break
	}
	return dt
}

// unqual strips away qualifiers from a DWARF type.
// In general we don't care about top-level qualifiers.
func unqual(dt dwarf.Type) dwarf.Type {
	for {
		if d, ok := dt.(*dwarf.QualType); ok {
			dt = d.Type
		} else {
			break
		}
	}
	return dt
}

// Map from dwarf text names to aliases we use in package "C".
var dwarfToName = map[string]string{
	"long int":               "long",
	"long unsigned int":      "ulong",
	"unsigned int":           "uint",
	"short unsigned int":     "ushort",
	"unsigned short":         "ushort", // Used by Clang; issue 13129.
	"short int":              "short",
	"long long int":          "longlong",
	"long long unsigned int": "ulonglong",
	"signed char":            "schar",
	"unsigned char":          "uchar",
	"unsigned long":          "ulong",     // Used by Clang 14; issue 53013.
	"unsigned long long":     "ulonglong", // Used by Clang 14; issue 53013.
}

const signedDelta = 64

// String returns the current type representation. Format arguments
// are assembled within this method so that any changes in mutable
// values are taken into account.
func (tr *TypeRepr) String() string {
	if len(tr.Repr) == 0 {
		return ""
	}
	if len(tr.FormatArgs) == 0 {
		return tr.Repr
	}
	return fmt.Sprintf(tr.Repr, tr.FormatArgs...)
}

// Empty reports whether the result of String would be "".
func (tr *TypeRepr) Empty() bool {
	return len(tr.Repr) == 0
}

// Set modifies the type representation.
// If fargs are provided, repr is used as a format for fmt.Sprintf.
// Otherwise, repr is used unprocessed as the type representation.
func (tr *TypeRepr) Set(repr string, fargs ...any) {
	tr.Repr = repr
	tr.FormatArgs = fargs
}

// FinishType completes any outstanding type mapping work.
// In particular, it resolves incomplete pointer types.
func (c *typeConv) FinishType(pos token.Pos) {
	// Completing one pointer type might produce more to complete.
	// Keep looping until they're all done.
	for len(c.ptrKeys) > 0 {
		dtype := c.ptrKeys[0]
		dtypeKey := dtype.String()
		c.ptrKeys = c.ptrKeys[1:]
		ptrs := c.ptrs[dtypeKey]
		delete(c.ptrs, dtypeKey)

		// Note Type might invalidate c.ptrs[dtypeKey].
		t := c.Type(dtype, pos)
		for _, ptr := range ptrs {
			ptr.Go.(*ast.StarExpr).X = t.Go
			ptr.C.Set("%s*", t.C)
		}
	}
}

// Type returns a *Type with the same memory layout as
// dtype when used as the type of a variable or a struct field.
func (c *typeConv) Type(dtype dwarf.Type, pos token.Pos) *Type {
	return c.loadType(dtype, pos, "")
}

// loadType recursively loads the requested dtype and its dependency graph.
func (c *typeConv) loadType(dtype dwarf.Type, pos token.Pos, parent string) *Type {
	// Always recompute bad pointer typedefs, as the set of such
	// typedefs changes as we see more types.
	checkCache := true
	if dtt, ok := dtype.(*dwarf.TypedefType); ok && c.badPointerTypedef(dtt) {
		checkCache = false
	}

	// The cache key should be relative to its parent.
	// See issue https://golang.org/issue/31891
	key := parent + " > " + dtype.String()

	if checkCache {
		if t, ok := c.m[key]; ok {
			if t.Go == nil {
				fatalf("%s: type conversion loop at %s", lineno(pos), dtype)
			}
			return t
		}
	}

	t := new(Type)
	t.Size = dtype.Size() // note: wrong for array of pointers, corrected below
	t.Align = -1
	t.C = &TypeRepr{Repr: dtype.Common().Name}
	c.m[key] = t

	switch dt := dtype.(type) {
	default:
		fatalf("%s: unexpected type: %s", lineno(pos), dtype)

	case *dwarf.AddrType:
		if t.Size != c.ptrSize {
			fatalf("%s: unexpected: %d-byte address type - %s", lineno(pos), t.Size, dtype)
		}
		t.Go = c.uintptr
		t.Align = t.Size

	case *dwarf.ArrayType:
		if dt.StrideBitSize > 0 {
			// Cannot represent bit-sized elements in Go.
			t.Go = c.Opaque(t.Size)
			break
		}
		count := dt.Count
		if count == -1 {
			// Indicates flexible array member, which Go doesn't support.
			// Translate to zero-length array instead.
			count = 0
		}
		sub := c.Type(dt.Type, pos)
		t.Align = sub.Align
		t.Go = &ast.ArrayType{
			Len: c.intExpr(count),
			Elt: sub.Go,
		}
		// Recalculate t.Size now that we know sub.Size.
		t.Size = count * sub.Size
		t.C.Set("__typeof__(%s[%d])", sub.C, dt.Count)

	case *dwarf.BoolType:
		t.Go = c.bool
		t.Align = 1

	case *dwarf.CharType:
		if t.Size != 1 {
			fatalf("%s: unexpected: %d-byte char type - %s", lineno(pos), t.Size, dtype)
		}
		t.Go = c.int8
		t.Align = 1

	case *dwarf.EnumType:
		if t.Align = t.Size; t.Align >= c.ptrSize {
			t.Align = c.ptrSize
		}
		t.C.Set("enum " + dt.EnumName)
		signed := 0
		t.EnumValues = make(map[string]int64)
		for _, ev := range dt.Val {
			t.EnumValues[ev.Name] = ev.Val
			if ev.Val < 0 {
				signed = signedDelta
			}
		}
		switch t.Size + int64(signed) {
		default:
			fatalf("%s: unexpected: %d-byte enum type - %s", lineno(pos), t.Size, dtype)
		case 1:
			t.Go = c.uint8
		case 2:
			t.Go = c.uint16
		case 4:
			t.Go = c.uint32
		case 8:
			t.Go = c.uint64
		case 1 + signedDelta:
			t.Go = c.int8
		case 2 + signedDelta:
			t.Go = c.int16
		case 4 + signedDelta:
			t.Go = c.int32
		case 8 + signedDelta:
			t.Go = c.int64
		}

	case *dwarf.FloatType:
		switch t.Size {
		default:
			fatalf("%s: unexpected: %d-byte float type - %s", lineno(pos), t.Size, dtype)
		case 4:
			t.Go = c.float32
		case 8:
			t.Go = c.float64
		}
		if t.Align = t.Size; t.Align >= c.ptrSize {
			t.Align = c.ptrSize
		}

	case *dwarf.ComplexType:
		switch t.Size {
		default:
			fatalf("%s: unexpected: %d-byte complex type - %s", lineno(pos), t.Size, dtype)
		case 8:
			t.Go = c.complex64
		case 16:
			t.Go = c.complex128
		}
		if t.Align = t.Size / 2; t.Align >= c.ptrSize {
			t.Align = c.ptrSize
		}

	case *dwarf.FuncType:
		// No attempt at translation: would enable calls
		// directly between worlds, but we need to moderate those.
		t.Go = c.uintptr
		t.Align = c.ptrSize

	case *dwarf.IntType:
		if dt.BitSize > 0 {
			fatalf("%s: unexpected: %d-bit int type - %s", lineno(pos), dt.BitSize, dtype)
		}

		if t.Align = t.Size; t.Align >= c.ptrSize {
			t.Align = c.ptrSize
		}

		switch t.Size {
		default:
			fatalf("%s: unexpected: %d-byte int type - %s", lineno(pos), t.Size, dtype)
		case 1:
			t.Go = c.int8
		case 2:
			t.Go = c.int16
		case 4:
			t.Go = c.int32
		case 8:
			t.Go = c.int64
		case 16:
			t.Go = &ast.ArrayType{
				Len: c.intExpr(t.Size),
				Elt: c.uint8,
			}
			// t.Align is the alignment of the Go type.
			t.Align = 1
		}

	case *dwarf.PtrType:
		// Clang doesn't emit DW_AT_byte_size for pointer types.
		if t.Size != c.ptrSize && t.Size != -1 {
			fatalf("%s: unexpected: %d-byte pointer type - %s", lineno(pos), t.Size, dtype)
		}
		t.Size = c.ptrSize
		t.Align = c.ptrSize

		if _, ok := base(dt.Type).(*dwarf.VoidType); ok {
			t.Go = c.goVoidPtr
			t.C.Set("void*")
			dq := dt.Type
			for {
				if d, ok := dq.(*dwarf.QualType); ok {
					t.C.Set(d.Qual + " " + t.C.String())
					dq = d.Type
				} else {
					break
				}
			}
			break
		}

		// Placeholder initialization; completed in FinishType.
		t.Go = &ast.StarExpr{}
		t.C.Set("<incomplete>*")
		key := dt.Type.String()
		if _, ok := c.ptrs[key]; !ok {
			c.ptrKeys = append(c.ptrKeys, dt.Type)
		}
		c.ptrs[key] = append(c.ptrs[key], t)

	case *dwarf.QualType:
		t1 := c.Type(dt.Type, pos)
		t.Size = t1.Size
		t.Align = t1.Align
		t.Go = t1.Go
		if unionWithPointer[t1.Go] {
			unionWithPointer[t.Go] = true
		}
		t.EnumValues = nil
		t.Typedef = ""
		t.C.Set("%s "+dt.Qual, t1.C)
		return t

	case *dwarf.StructType:
		// Convert to Go struct, being careful about alignment.
		// Have to give it a name to simulate C "struct foo" references.
		tag := dt.StructName
		if dt.ByteSize < 0 && tag == "" { // opaque unnamed struct - should not be possible
			break
		}
		if tag == "" {
			tag = anonymousStructTag[dt]
			if tag == "" {
				tag = "__" + strconv.Itoa(tagGen)
				tagGen++
				anonymousStructTag[dt] = tag
			}
		} else if t.C.Empty() {
			t.C.Set(dt.Kind + " " + tag)
		}
		name := c.Ident("_Ctype_" + dt.Kind + "_" + tag)
		t.Go = name // publish before recursive calls
		goIdent[name.Name] = name
		if dt.ByteSize < 0 {
			// Don't override old type
			if _, ok := typedef[name.Name]; ok {
				break
			}

			// Size calculation in c.Struct/c.Opaque will die with size=-1 (unknown),
			// so execute the basic things that the struct case would do
			// other than try to determine a Go representation.
			tt := *t
			tt.C = &TypeRepr{"%s %s", []any{dt.Kind, tag}}
			// We don't know what the representation of this struct is, so don't let
			// anyone allocate one on the Go side. As a side effect of this annotation,
			// pointers to this type will not be considered pointers in Go. They won't
			// get writebarrier-ed or adjusted during a stack copy. This should handle
			// all the cases badPointerTypedef used to handle, but hopefully will
			// continue to work going forward without any more need for cgo changes.
			tt.Go = c.Ident(incomplete)
			typedef[name.Name] = &tt
			break
		}
		switch dt.Kind {
		case "class", "union":
			t.Go = c.Opaque(t.Size)
			if c.dwarfHasPointer(dt, pos) {
				unionWithPointer[t.Go] = true
			}
			if t.C.Empty() {
				t.C.Set("__typeof__(unsigned char[%d])", t.Size)
			}
			t.Align = 1 // TODO: should probably base this on field alignment.
			typedef[name.Name] = t
		case "struct":
			g, csyntax, align := c.Struct(dt, pos)
			if t.C.Empty() {
				t.C.Set(csyntax)
			}
			t.Align = align
			tt := *t
			if tag != "" {
				tt.C = &TypeRepr{"struct %s", []any{tag}}
			}
			tt.Go = g
			if c.incompleteStructs[tag] {
				tt.Go = c.Ident(incomplete)
			}
			typedef[name.Name] = &tt
		}

	case *dwarf.TypedefType:
		// Record typedef for printing.
		if dt.Name == "_GoString_" {
			// Special C name for Go string type.
			// Knows string layout used by compilers: pointer plus length,
			// which rounds up to 2 pointers after alignment.
			t.Go = c.string
			t.Size = c.ptrSize * 2
			t.Align = c.ptrSize
			break
		}
		if dt.Name == "_GoBytes_" {
			// Special C name for Go []byte type.
			// Knows slice layout used by compilers: pointer, length, cap.
			t.Go = c.Ident("[]byte")
			t.Size = c.ptrSize + 4 + 4
			t.Align = c.ptrSize
			break
		}
		name := c.Ident("_Ctype_" + dt.Name)
		goIdent[name.Name] = name
		akey := ""
		if c.anonymousStructTypedef(dt) {
			// only load type recursively for typedefs of anonymous
			// structs, see issues 37479 and 37621.
			akey = key
		}
		sub := c.loadType(dt.Type, pos, akey)
		if c.badPointerTypedef(dt) {
			// Treat this typedef as a uintptr.
			s := *sub
			s.Go = c.uintptr
			s.BadPointer = true
			sub = &s
			// Make sure we update any previously computed type.
			if oldType := typedef[name.Name]; oldType != nil {
				oldType.Go = sub.Go
				oldType.BadPointer = true
			}
		}
		if c.badVoidPointerTypedef(dt) {
			// Treat this typedef as a pointer to a _cgopackage.Incomplete.
			s := *sub
			s.Go = c.Ident("*" + incomplete)
			sub = &s
			// Make sure we update any previously computed type.
			if oldType := typedef[name.Name]; oldType != nil {
				oldType.Go = sub.Go
			}
		}
		// Check for non-pointer "struct <tag>{...}; typedef struct <tag> *<name>"
		// typedefs that should be marked Incomplete.
		if ptr, ok := dt.Type.(*dwarf.PtrType); ok {
			if strct, ok := ptr.Type.(*dwarf.StructType); ok {
				if c.badStructPointerTypedef(dt.Name, strct) {
					c.incompleteStructs[strct.StructName] = true
					// Make sure we update any previously computed type.
					name := "_Ctype_struct_" + strct.StructName
					if oldType := typedef[name]; oldType != nil {
						oldType.Go = c.Ident(incomplete)
					}
				}
			}
		}
		t.Go = name
		t.BadPointer = sub.BadPointer
		if unionWithPointer[sub.Go] {
			unionWithPointer[t.Go] = true
		}
		t.Size = sub.Size
		t.Align = sub.Align
		oldType := typedef[name.Name]
		if oldType == nil {
			tt := *t
			tt.Go = sub.Go
			tt.BadPointer = sub.BadPointer
			typedef[name.Name] = &tt
		}

		// If sub.Go.Name is "_Ctype_struct_foo" or "_Ctype_union_foo" or "_Ctype_class_foo",
		// use that as the Go form for this typedef too, so that the typedef will be interchangeable
		// with the base type.
		// In -godefs mode, do this for all typedefs.
		if isStructUnionClass(sub.Go) || *godefs {
			t.Go = sub.Go

			if isStructUnionClass(sub.Go) {
				// Use the typedef name for C code.
				typedef[sub.Go.(*ast.Ident).Name].C = t.C
			}

			// If we've seen this typedef before, and it
			// was an anonymous struct/union/class before
			// too, use the old definition.
			// TODO: it would be safer to only do this if
			// we verify that the types are the same.
			if oldType != nil && isStructUnionClass(oldType.Go) {
				t.Go = oldType.Go
			}
		}

	case *dwarf.UcharType:
		if t.Size != 1 {
			fatalf("%s: unexpected: %d-byte uchar type - %s", lineno(pos), t.Size, dtype)
		}
		t.Go = c.uint8
		t.Align = 1

	case *dwarf.UintType:
		if dt.BitSize > 0 {
			fatalf("%s: unexpected: %d-bit uint type - %s", lineno(pos), dt.BitSize, dtype)
		}

		if t.Align = t.Size; t.Align >= c.ptrSize {
			t.Align = c.ptrSize
		}

		switch t.Size {
		default:
			fatalf("%s: unexpected: %d-byte uint type - %s", lineno(pos), t.Size, dtype)
		case 1:
			t.Go = c.uint8
		case 2:
			t.Go = c.uint16
		case 4:
			t.Go = c.uint32
		case 8:
			t.Go = c.uint64
		case 16:
			t.Go = &ast.ArrayType{
				Len: c.intExpr(t.Size),
				Elt: c.uint8,
			}
			// t.Align is the alignment of the Go type.
			t.Align = 1
		}

	case *dwarf.VoidType:
		t.Go = c.goVoid
		t.C.Set("void")
		t.Align = 1
	}

	switch dtype.(type) {
	case *dwarf.AddrType, *dwarf.BoolType, *dwarf.CharType, *dwarf.ComplexType, *dwarf.IntType, *dwarf.FloatType, *dwarf.UcharType, *dwarf.UintType:
		s := dtype.Common().Name
		if s != "" {
			if ss, ok := dwarfToName[s]; ok {
				s = ss
			}
			s = strings.ReplaceAll(s, " ", "")
			name := c.Ident("_Ctype_" + s)
			tt := *t
			typedef[name.Name] = &tt
			if !*godefs {
				t.Go = name
			}
		}
	}

	if t.Size < 0 {
		// Unsized types are [0]byte, unless they're typedefs of other types
		// or structs with tags.
		// if so, use the name we've already defined.
		t.Size = 0
		switch dt := dtype.(type) {
		case *dwarf.TypedefType:
			// ok
		case *dwarf.StructType:
			if dt.StructName != "" {
				break
			}
			t.Go = c.Opaque(0)
		default:
			t.Go = c.Opaque(0)
		}
		if t.C.Empty() {
			t.C.Set("void")
		}
	}

	if t.C.Empty() {
		fatalf("%s: internal error: did not create C name for %s", lineno(pos), dtype)
	}

	return t
}

// isStructUnionClass reports whether the type described by the Go syntax x
// is a struct, union, or class with a tag.
func isStructUnionClass(x ast.Expr) bool {
	id, ok := x.(*ast.Ident)
	if !ok {
		return false
	}
	name := id.Name
	return strings.HasPrefix(name, "_Ctype_struct_") ||
		strings.HasPrefix(name, "_Ctype_union_") ||
		strings.HasPrefix(name, "_Ctype_class_")
}

// FuncArg returns a Go type with the same memory layout as
// dtype when used as the type of a C function argument.
func (c *typeConv) FuncArg(dtype dwarf.Type, pos token.Pos) *Type {
	t := c.Type(unqual(dtype), pos)
	switch dt := dtype.(type) {
	case *dwarf.ArrayType:
		// Arrays are passed implicitly as pointers in C.
		// In Go, we must be explicit.
		tr := &TypeRepr{}
		tr.Set("%s*", t.C)
		return &Type{
			Size:  c.ptrSize,
			Align: c.ptrSize,
			Go:    &ast.StarExpr{X: t.Go},
			C:     tr,
		}
	case *dwarf.TypedefType:
		// C has much more relaxed rules than Go for
		// implicit type conversions. When the parameter
		// is type T defined as *X, simulate a little of the
		// laxness of C by making the argument *X instead of T.
		if ptr, ok := base(dt.Type).(*dwarf.PtrType); ok {
			// Unless the typedef happens to point to void* since
			// Go has special rules around using unsafe.Pointer.
			if _, void := base(ptr.Type).(*dwarf.VoidType); void {
				break
			}
			// ...or the typedef is one in which we expect bad pointers.
			// It will be a uintptr instead of *X.
			if c.baseBadPointerTypedef(dt) {
				break
			}

			t = c.Type(ptr, pos)
			if t == nil {
				return nil
			}

			// For a struct/union/class, remember the C spelling,
			// in case it has __attribute__((unavailable)).
			// See issue 2888.
			if isStructUnionClass(t.Go) {
				t.Typedef = dt.Name
			}
		}
	}
	return t
}

// FuncType returns the Go type analogous to dtype.
// There is no guarantee about matching memory layout.
func (c *typeConv) FuncType(dtype *dwarf.FuncType, pos token.Pos) *FuncType {
	p := make([]*Type, len(dtype.ParamType))
	gp := make([]*ast.Field, len(dtype.ParamType))
	for i, f := range dtype.ParamType {
		// gcc's DWARF generator outputs a single DotDotDotType parameter for
		// function pointers that specify no parameters (e.g. void
		// (*__cgo_0)()). Treat this special case as void. This case is
		// invalid according to ISO C anyway (i.e. void (*__cgo_1)(...) is not
		// legal).
		if _, ok := f.(*dwarf.DotDotDotType); ok && i == 0 {
			p, gp = nil, nil
			break
		}
		p[i] = c.FuncArg(f, pos)
		gp[i] = &ast.Field{Type: p[i].Go}
	}
	var r *Type
	var gr []*ast.Field
	if _, ok := base(dtype.ReturnType).(*dwarf.VoidType); ok {
		gr = []*ast.Field{{Type: c.goVoid}}
	} else if dtype.ReturnType != nil {
		r = c.Type(unqual(dtype.ReturnType), pos)
		gr = []*ast.Field{{Type: r.Go}}
	}
	return &FuncType{
		Params: p,
		Result: r,
		Go: &ast.FuncType{
			Params:  &ast.FieldList{List: gp},
			Results: &ast.FieldList{List: gr},
		},
	}
}

// Identifier
func (c *typeConv) Ident(s string) *ast.Ident {
	return ast.NewIdent(s)
}

// Opaque type of n bytes.
func (c *typeConv) Opaque(n int64) ast.Expr {
	return &ast.ArrayType{
		Len: c.intExpr(n),
		Elt: c.byte,
	}
}

// Expr for integer n.
func (c *typeConv) intExpr(n int64) ast.Expr {
	return &ast.BasicLit{
		Kind:  token.INT,
		Value: strconv.FormatInt(n, 10),
	}
}

// Add padding of given size to fld.
func (c *typeConv) pad(fld []*ast.Field, sizes []int64, size int64) ([]*ast.Field, []int64) {
	n := len(fld)
	fld = fld[0 : n+1]
	fld[n] = &ast.Field{Names: []*ast.Ident{c.Ident("_")}, Type: c.Opaque(size)}
	sizes = sizes[0 : n+1]
	sizes[n] = size
	return fld, sizes
}

// Struct conversion: return Go and (gc) C syntax for type.
func (c *typeConv) Struct(dt *dwarf.StructType, pos token.Pos) (expr *ast.StructType, csyntax string, align int64) {
	// Minimum alignment for a struct is 1 byte.
	align = 1

	var buf strings.Builder
	buf.WriteString("struct {")
	fld := make([]*ast.Field, 0, 2*len(dt.Field)+1) // enough for padding around every field
	sizes := make([]int64, 0, 2*len(dt.Field)+1)
	off := int64(0)

	// Rename struct fields that happen to be named Go keywords into
	// _{keyword}. Create a map from C ident -> Go ident. The Go ident will
	// be mangled. Any existing identifier that already has the same name on
	// the C-side will cause the Go-mangled version to be prefixed with _.
	// (e.g. in a struct with fields '_type' and 'type', the latter would be
	// rendered as '__type' in Go).
	ident := make(map[string]string)
	used := make(map[string]bool)
	for _, f := range dt.Field {
		ident[f.Name] = f.Name
		used[f.Name] = true
	}

	if !*godefs {
		for cid, goid := range ident {
			if token.Lookup(goid).IsKeyword() {
				// Avoid keyword
				goid = "_" + goid

				// Also avoid existing fields
				for _, exist := used[goid]; exist; _, exist = used[goid] {
					goid = "_" + goid
				}

				used[goid] = true
				ident[cid] = goid
			}
		}
	}

	anon := 0
	for _, f := range dt.Field {
		name := f.Name
		ft := f.Type

		// In godefs mode, if this field is a C11
		// anonymous union then treat the first field in the
		// union as the field in the struct. This handles
		// cases like the glibc <sys/resource.h> file; see
		// issue 6677.
		if *godefs {
			if st, ok := f.Type.(*dwarf.StructType); ok && name == "" && st.Kind == "union" && len(st.Field) > 0 && !used[st.Field[0].Name] {
				name = st.Field[0].Name
				ident[name] = name
				ft = st.Field[0].Type
			}
		}

		// TODO: Handle fields that are anonymous structs by
		// promoting the fields of the inner struct.

		t := c.Type(ft, pos)
		tgo := t.Go
		size := t.Size
		talign := t.Align
		if f.BitOffset > 0 || f.BitSize > 0 {
			// The layout of bitfields is implementation defined,
			// so we don't know how they correspond to Go fields
			// even if they are aligned at byte boundaries.
			continue
		}

		if talign > 0 && f.ByteOffset%talign != 0 {
			// Drop misaligned fields, the same way we drop integer bit fields.
			// The goal is to make available what can be made available.
			// Otherwise one bad and unneeded field in an otherwise okay struct
			// makes the whole program not compile. Much of the time these
			// structs are in system headers that cannot be corrected.
			continue
		}

		// Round off up to talign, assumed to be a power of 2.
		origOff := off
		off = (off + talign - 1) &^ (talign - 1)

		if f.ByteOffset > off {
			fld, sizes = c.pad(fld, sizes, f.ByteOffset-origOff)
			off = f.ByteOffset
		}
		if f.ByteOffset < off {
			// Drop a packed field that we can't represent.
			continue
		}

		n := len(fld)
		fld = fld[0 : n+1]
		if name == "" {
			name = fmt.Sprintf("anon%d", anon)
			anon++
			ident[name] = name
		}
		fld[n] = &ast.Field{Names: []*ast.Ident{c.Ident(ident[name])}, Type: tgo}
		sizes = sizes[0 : n+1]
		sizes[n] = size
		off += size
		buf.WriteString(t.C.String())
		buf.WriteString(" ")
		buf.WriteString(name)
		buf.WriteString("; ")
		if talign > align {
			align = talign
		}
	}
	if off < dt.ByteSize {
		fld, sizes = c.pad(fld, sizes, dt.ByteSize-off)
		off = dt.ByteSize
	}

	// If the last field in a non-zero-sized struct is zero-sized
	// the compiler is going to pad it by one (see issue 9401).
	// We can't permit that, because then the size of the Go
	// struct will not be the same as the size of the C struct.
	// Our only option in such a case is to remove the field,
	// which means that it cannot be referenced from Go.
	for off > 0 && sizes[len(sizes)-1] == 0 {
		n := len(sizes)
		fld = fld[0 : n-1]
		sizes = sizes[0 : n-1]
	}

	if off != dt.ByteSize {
		fatalf("%s: struct size calculation error off=%d bytesize=%d", lineno(pos), off, dt.ByteSize)
	}
	buf.WriteString("}")
	csyntax = buf.String()

	if *godefs {
		godefsFields(fld)
	}
	expr = &ast.StructType{Fields: &ast.FieldList{List: fld}}
	return
}

// dwarfHasPointer reports whether the DWARF type dt contains a pointer.
func (c *typeConv) dwarfHasPointer(dt dwarf.Type, pos token.Pos) bool {
	switch dt := dt.(type) {
	default:
		fatalf("%s: unexpected type: %s", lineno(pos), dt)
		return false

	case *dwarf.AddrType, *dwarf.BoolType, *dwarf.CharType, *dwarf.EnumType,
		*dwarf.FloatType, *dwarf.ComplexType, *dwarf.FuncType,
		*dwarf.IntType, *dwarf.UcharType, *dwarf.UintType, *dwarf.VoidType:

		return false

	case *dwarf.ArrayType:
		return c.dwarfHasPointer(dt.Type, pos)

	case *dwarf.PtrType:
		return true

	case *dwarf.QualType:
		return c.dwarfHasPointer(dt.Type, pos)

	case *dwarf.StructType:
		return slices.ContainsFunc(dt.Field, func(f *dwarf.StructField) bool {
			return c.dwarfHasPointer(f.Type, pos)
		})

	case *dwarf.TypedefType:
		if dt.Name == "_GoString_" || dt.Name == "_GoBytes_" {
			return true
		}
		return c.dwarfHasPointer(dt.Type, pos)
	}
}

func upper(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == '_' {
		return "X" + s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// godefsFields rewrites field names for use in Go or C definitions.
// It strips leading common prefixes (like tv_ in tv_sec, tv_usec)
// converts names to upper case, and rewrites _ into Pad_godefs_n,
// so that all fields are exported.
func godefsFields(fld []*ast.Field) {
	prefix := fieldPrefix(fld)

	// Issue 48396: check for duplicate field names.
	if prefix != "" {
		names := make(map[string]bool)
	fldLoop:
		for _, f := range fld {
			for _, n := range f.Names {
				name := n.Name
				if name == "_" {
					continue
				}
				if name != prefix {
					name = strings.TrimPrefix(n.Name, prefix)
				}
				name = upper(name)
				if names[name] {
					// Field name conflict: don't remove prefix.
					prefix = ""
					break fldLoop
				}
				names[name] = true
			}
		}
	}

	npad := 0
	for _, f := range fld {
		for _, n := range f.Names {
			if n.Name != prefix {
				n.Name = strings.TrimPrefix(n.Name, prefix)
			}
			if n.Name == "_" {
				// Use exported name instead.
				n.Name = "Pad_cgo_" + strconv.Itoa(npad)
				npad++
			}
			n.Name = upper(n.Name)
		}
	}
}

// fieldPrefix returns the prefix that should be removed from all the
// field names when generating the C or Go code. For generated
// C, we leave the names as is (tv_sec, tv_usec), since that's what
// people are used to seeing in C. For generated Go code, such as
// package syscall's data structures, we drop a common prefix
// (so sec, usec, which will get turned into Sec, Usec for exporting).
func fieldPrefix(fld []*ast.Field) string {
	prefix := ""
	for _, f := range fld {
		for _, n := range f.Names {
			// Ignore field names that don't have the prefix we're
			// looking for. It is common in C headers to have fields
			// named, say, _pad in an otherwise prefixed header.
			// If the struct has 3 fields tv_sec, tv_usec, _pad1, then we
			// still want to remove the tv_ prefix.
			// The check for "orig_" here handles orig_eax in the
			// x86 ptrace register sets, which otherwise have all fields
			// with reg_ prefixes.
			if strings.HasPrefix(n.Name, "orig_") || strings.HasPrefix(n.Name, "_") {
				continue
			}
			i := strings.Index(n.Name, "_")
			if i < 0 {
				continue
			}
			if prefix == "" {
				prefix = n.Name[:i+1]
			} else if prefix != n.Name[:i+1] {
				return ""
			}
		}
	}
	return prefix
}

// anonymousStructTypedef reports whether dt is a C typedef for an anonymous
// struct.
func (c *typeConv) anonymousStructTypedef(dt *dwarf.TypedefType) bool {
	st, ok := dt.Type.(*dwarf.StructType)
	return ok && st.StructName == ""
}

// badPointerTypedef reports whether dt is a C typedef that should not be
// considered a pointer in Go. A typedef is bad if C code sometimes stores
// non-pointers in this type.
// TODO: Currently our best solution is to find these manually and list them as
// they come up. A better solution is desired.
// Note: DEPRECATED. There is now a better solution. Search for incomplete in this file.
func (c *typeConv) badPointerTypedef(dt *dwarf.TypedefType) bool {
	if c.badCFType(dt) {
		return true
	}
	if c.badJNI(dt) {
		return true
	}
	if c.badEGLType(dt) {
		return true
	}
	return false
}

// badVoidPointerTypedef is like badPointerTypeDef, but for "void *" typedefs that should be _cgopackage.Incomplete.
func (c *typeConv) badVoidPointerTypedef(dt *dwarf.TypedefType) bool {
	// Match the Windows HANDLE type (#42018).
	if goos != "windows" || dt.Name != "HANDLE" {
		return false
	}
	// Check that the typedef is "typedef void *<name>".
	if ptr, ok := dt.Type.(*dwarf.PtrType); ok {
		if _, ok := ptr.Type.(*dwarf.VoidType); ok {
			return true
		}
	}
	return false
}

// badStructPointerTypedef is like badVoidPointerTypedef but for structs.
func (c *typeConv) badStructPointerTypedef(name string, dt *dwarf.StructType) bool {
	// Windows handle types can all potentially contain non-pointers.
	// badVoidPointerTypedef handles the "void *" HANDLE type, but other
	// handles are defined as
	//
	// struct <name>__{int unused;}; typedef struct <name>__ *name;
	//
	// by the DECLARE_HANDLE macro in STRICT mode. The macro is declared in
	// the Windows ntdef.h header,
	//
	// https://github.com/tpn/winsdk-10/blob/master/Include/10.0.16299.0/shared/ntdef.h#L779
	if goos != "windows" {
		return false
	}
	if len(dt.Field) != 1 {
		return false
	}
	if dt.StructName != name+"__" {
		return false
	}
	if f := dt.Field[0]; f.Name != "unused" || f.Type.Common().Name != "int" {
		return false
	}
	return true
}

// baseBadPointerTypedef reports whether the base of a chain of typedefs is a bad typedef
// as badPointerTypedef reports.
func (c *typeConv) baseBadPointerTypedef(dt *dwarf.TypedefType) bool {
	for {
		if t, ok := dt.Type.(*dwarf.TypedefType); ok {
			dt = t
			continue
		}
		break
	}
	return c.badPointerTypedef(dt)
}

func (c *typeConv) badCFType(dt *dwarf.TypedefType) bool {
	// The real bad types are CFNumberRef and CFDateRef.
	// Sometimes non-pointers are stored in these types.
	// CFTypeRef is a supertype of those, so it can have bad pointers in it as well.
	// We return true for the other *Ref types just so casting between them is easier.
	// We identify the correct set of types as those ending in Ref and for which
	// there exists a corresponding GetTypeID function.
	// See comment below for details about the bad pointers.
	if goos != "darwin" && goos != "ios" {
		return false
	}
	s := dt.Name
	if !strings.HasSuffix(s, "Ref") {
		return false
	}
	s = s[:len(s)-3]
	if s == "CFType" {
		return true
	}
	if c.getTypeIDs[s] {
		return true
	}
	if i := strings.Index(s, "Mutable"); i >= 0 && c.getTypeIDs[s[:i]+s[i+7:]] {
		// Mutable and immutable variants share a type ID.
		return true
	}
	return false
}

// Comment from Darwin's CFInternal.h
/*
// Tagged pointer support
// Low-bit set means tagged object, next 3 bits (currently)
// define the tagged object class, next 4 bits are for type
// information for the specific tagged object class. Thus,
// the low byte is for type info, and the rest of a pointer
// (32 or 64-bit) is for payload, whatever the tagged class.
//
// Note that the specific integers used to identify the
// specific tagged classes can and will change from release
// to release (that's why this stuff is in CF*Internal*.h),
// as can the definition of type info vs payload above.
//
#if __LP64__
#define CF_IS_TAGGED_OBJ(PTR)	((uintptr_t)(PTR) & 0x1)
#define CF_TAGGED_OBJ_TYPE(PTR)	((uintptr_t)(PTR) & 0xF)
#else
#define CF_IS_TAGGED_OBJ(PTR)	0
#define CF_TAGGED_OBJ_TYPE(PTR)	0
#endif

enum {
    kCFTaggedObjectID_Invalid = 0,
    kCFTaggedObjectID_Atom = (0 << 1) + 1,
    kCFTaggedObjectID_Undefined3 = (1 << 1) + 1,
    kCFTaggedObjectID_Undefined2 = (2 << 1) + 1,
    kCFTaggedObjectID_Integer = (3 << 1) + 1,
    kCFTaggedObjectID_DateTS = (4 << 1) + 1,
    kCFTaggedObjectID_ManagedObjectID = (5 << 1) + 1, // Core Data
    kCFTaggedObjectID_Date = (6 << 1) + 1,
    kCFTaggedObjectID_Undefined7 = (7 << 1) + 1,
};
*/

func (c *typeConv) badJNI(dt *dwarf.TypedefType) bool {
	// In Dalvik and ART, the jobject type in the JNI interface of the JVM has the
	// property that it is sometimes (always?) a small integer instead of a real pointer.
	// Note: although only the android JVMs are bad in this respect, we declare the JNI types
	// bad regardless of platform, so the same Go code compiles on both android and non-android.
	if parent, ok := jniTypes[dt.Name]; ok {
		// Try to make sure we're talking about a JNI type, not just some random user's
		// type that happens to use the same name.
		// C doesn't have the notion of a package, so it's hard to be certain.

		// Walk up to jobject, checking each typedef on the way.
		w := dt
		for parent != "" {
			t, ok := w.Type.(*dwarf.TypedefType)
			if !ok || t.Name != parent {
				return false
			}
			w = t
			parent, ok = jniTypes[w.Name]
			if !ok {
				return false
			}
		}

		// Check that the typedef is either:
		// 1:
		//     	struct _jobject;
		//     	typedef struct _jobject *jobject;
		// 2: (in NDK16 in C++)
		//     	class _jobject {};
		//     	typedef _jobject* jobject;
		// 3: (in NDK16 in C)
		//     	typedef void* jobject;
		if ptr, ok := w.Type.(*dwarf.PtrType); ok {
			switch v := ptr.Type.(type) {
			case *dwarf.VoidType:
				return true
			case *dwarf.StructType:
				if v.StructName == "_jobject" && len(v.Field) == 0 {
					switch v.Kind {
					case "struct":
						if v.Incomplete {
							return true
						}
					case "class":
						if !v.Incomplete {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func (c *typeConv) badEGLType(dt *dwarf.TypedefType) bool {
	if dt.Name != "EGLDisplay" && dt.Name != "EGLConfig" {
		return false
	}
	// Check that the typedef is "typedef void *<name>".
	if ptr, ok := dt.Type.(*dwarf.PtrType); ok {
		if _, ok := ptr.Type.(*dwarf.VoidType); ok {
			return true
		}
	}
	return false
}

// jniTypes maps from JNI types that we want to be uintptrs, to the underlying type to which
// they are mapped. The base "jobject" maps to the empty string.
var jniTypes = map[string]string{
	"jobject":       "",
	"jclass":        "jobject",
	"jthrowable":    "jobject",
	"jstring":       "jobject",
	"jarray":        "jobject",
	"jbooleanArray": "jarray",
	"jbyteArray":    "jarray",
	"jcharArray":    "jarray",
	"jshortArray":   "jarray",
	"jintArray":     "jarray",
	"jlongArray":    "jarray",
	"jfloatArray":   "jarray",
	"jdoubleArray":  "jarray",
	"jobjectArray":  "jarray",
	"jweak":         "jobject",
}

```

// === FILE: references!/go/src/cmd/cgo/godefs.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// godefs returns the output for -godefs mode.
func (p *Package) godefs(f *File, args []string) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "// Code generated by cmd/cgo -godefs; DO NOT EDIT.\n")
	fmt.Fprintf(&buf, "// %s %s\n", filepath.Base(args[0]), strings.Join(args[1:], " "))
	fmt.Fprintf(&buf, "\n")

	override := make(map[string]string)

	// Allow source file to specify override mappings.
	// For example, the socket data structures refer
	// to in_addr and in_addr6 structs but we want to be
	// able to treat them as byte arrays, so the godefs
	// inputs in package syscall say
	//
	//	// +godefs map struct_in_addr [4]byte
	//	// +godefs map struct_in_addr6 [16]byte
	//
	for _, g := range f.Comments {
		for _, c := range g.List {
			i := strings.Index(c.Text, "+godefs map")
			if i < 0 {
				continue
			}
			s := strings.TrimSpace(c.Text[i+len("+godefs map"):])
			i = strings.Index(s, " ")
			if i < 0 {
				fmt.Fprintf(os.Stderr, "invalid +godefs map comment: %s\n", c.Text)
				continue
			}
			override["_Ctype_"+strings.TrimSpace(s[:i])] = strings.TrimSpace(s[i:])
		}
	}
	for _, n := range f.Name {
		if s := override[n.Go]; s != "" {
			override[n.Mangle] = s
		}
	}

	// Otherwise, if the source file says type T C.whatever,
	// use "T" as the mangling of C.whatever,
	// except in the definition (handled at end of function).
	refName := make(map[*ast.Expr]*Name)
	for _, r := range f.Ref {
		refName[r.Expr] = r.Name
	}
	for _, d := range f.AST.Decls {
		d, ok := d.(*ast.GenDecl)
		if !ok || d.Tok != token.TYPE {
			continue
		}
		for _, s := range d.Specs {
			s := s.(*ast.TypeSpec)
			n := refName[&s.Type]
			if n != nil && n.Mangle != "" {
				override[n.Mangle] = s.Name.Name
			}
		}
	}

	// Extend overrides using typedefs:
	// If we know that C.xxx should format as T
	// and xxx is a typedef for yyy, make C.yyy format as T.
	for typ, def := range typedef {
		if new := override[typ]; new != "" {
			if id, ok := def.Go.(*ast.Ident); ok {
				override[id.Name] = new
			}
		}
	}

	// Apply overrides.
	for old, new := range override {
		if id := goIdent[old]; id != nil {
			id.Name = new
		}
	}

	// Any names still using the _C syntax are not going to compile,
	// although in general we don't know whether they all made it
	// into the file, so we can't warn here.
	//
	// The most common case is union types, which begin with
	// _Ctype_union and for which typedef[name] is a Go byte
	// array of the appropriate size (such as [4]byte).
	// Substitute those union types with byte arrays.
	for name, id := range goIdent {
		if id.Name == name && strings.Contains(name, "_Ctype_union") {
			if def := typedef[name]; def != nil {
				id.Name = gofmt(def)
			}
		}
	}

	conf.Fprint(&buf, fset, f.AST)

	return buf.String()
}

var gofmtBuf strings.Builder

// gofmt returns the gofmt-formatted string for an AST node.
func gofmt(n any) string {
	gofmtBuf.Reset()
	err := printer.Fprint(&gofmtBuf, fset, n)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return gofmtBuf.String()
}

```

// === FILE: references!/go/src/cmd/cgo/internal/cgotest/overlaydir.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cgotest

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// OverlayDir makes a minimal-overhead copy of srcRoot in which new files may be added.
func OverlayDir(dstRoot, srcRoot string) error {
	dstRoot = filepath.Clean(dstRoot)
	if err := os.MkdirAll(dstRoot, 0777); err != nil {
		return err
	}

	srcRoot, err := filepath.Abs(srcRoot)
	if err != nil {
		return err
	}

	return filepath.Walk(srcRoot, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil || srcPath == srcRoot {
			return err
		}

		suffix := strings.TrimPrefix(srcPath, srcRoot)
		for len(suffix) > 0 && suffix[0] == filepath.Separator {
			suffix = suffix[1:]
		}
		dstPath := filepath.Join(dstRoot, suffix)

		perm := info.Mode() & os.ModePerm
		if info.Mode()&os.ModeSymlink != 0 {
			info, err = os.Stat(srcPath)
			if err != nil {
				return err
			}
			perm = info.Mode() & os.ModePerm
		}

		// Always copy directories (don't symlink them).
		// If we add a file in the overlay, we don't want to add it in the original.
		if info.IsDir() {
			return os.MkdirAll(dstPath, perm|0200)
		}

		// If the OS supports symlinks, use them instead of copying bytes.
		if err := os.Symlink(srcPath, dstPath); err == nil {
			return nil
		}

		// Otherwise, copy the bytes.
		src, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if err != nil {
			return err
		}

		_, err = io.Copy(dst, src)
		if closeErr := dst.Close(); err == nil {
			err = closeErr
		}
		return err
	})
}

```

// === FILE: references!/go/src/cmd/cgo/internal/testnocgo/nocgo.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test that -static works when not using cgo.  This test is in
// misc/cgo to take advantage of the testing framework support for
// when -static is expected to work.

package nocgo

func NoCgo() int {
	c := make(chan int)

	// The test is run with external linking, which means that
	// goroutines will be created via the runtime/cgo package.
	// Make sure that works.
	go func() {
		c <- 42
	}()

	return <-c
}

```

// === FILE: references!/go/src/cmd/cgo/internal/testtls/tls.c ===
```text
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include <stddef.h>

#if __STDC_VERSION__ >= 201112L && !defined(__STDC_NO_THREADS__)

// Mingw seems not to have threads.h, so we use the _Thread_local keyword rather
// than the thread_local macro.
static _Thread_local int tls;

const char *
checkTLS() {
	return NULL;
}

void
setTLS(int v)
{
	tls = v;
}

int
getTLS()
{
	return tls;
}

#else

const char *
checkTLS() {
	return "_Thread_local requires C11 and not __STDC_NO_THREADS__";
}

void
setTLS(int v) {
}

int
getTLS()
{
	return 0;
}

#endif

```

// === FILE: references!/go/src/cmd/cgo/internal/testtls/tls.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cgotlstest

// extern const char *checkTLS();
// extern void setTLS(int);
// extern int getTLS();
import "C"

import (
	"runtime"
	"testing"
)

func testTLS(t *testing.T) {
	if skip := C.checkTLS(); skip != nil {
		t.Skipf("%s", C.GoString(skip))
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if val := C.getTLS(); val != 0 {
		t.Fatalf("at start, C.getTLS() = %#x, want 0", val)
	}

	const keyVal = 0x1234
	C.setTLS(keyVal)
	if val := C.getTLS(); val != keyVal {
		t.Fatalf("at end, C.getTLS() = %#x, want %#x", val, keyVal)
	}
}

```

// === FILE: references!/go/src/cmd/cgo/internal/testtls/tls_none.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !cgo

package cgotlstest

import "testing"

func testTLS(t *testing.T) {
	t.Skip("cgo not supported")
}

```

// === FILE: references!/go/src/cmd/cgo/main.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Cgo; see doc.go for an overview.

// TODO(rsc):
//	Emit correct line number annotations.
//	Make gc understand the annotations.

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"internal/buildcfg"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"

	"cmd/internal/edit"
	"cmd/internal/hash"
	"cmd/internal/objabi"
	"cmd/internal/par"
	"cmd/internal/telemetry/counter"
)

// A Package collects information about the package we're going to write.
type Package struct {
	PackageName string // name of package
	PackagePath string
	PtrSize     int64
	IntSize     int64
	GccOptions  []string
	GccIsClang  bool
	LdFlags     []string // #cgo LDFLAGS
	Written     map[string]bool
	Name        map[string]*Name // accumulated Name from Files
	ExpFunc     []*ExpFunc       // accumulated ExpFunc from Files
	Decl        []ast.Decl
	GoFiles     []string        // list of Go files
	GccFiles    []string        // list of gcc output files
	Preamble    string          // collected preamble for _cgo_export.h
	noCallbacks map[string]bool // C function names with #cgo nocallback directive
	noEscapes   map[string]bool // C function names with #cgo noescape directive
}

// A typedefInfo is an element on Package.typedefList: a typedef name
// and the position where it was required.
type typedefInfo struct {
	typedef string
	pos     token.Pos
}

// A File collects information about a single Go input file.
type File struct {
	AST         *ast.File           // parsed AST
	Comments    []*ast.CommentGroup // comments from file
	Package     string              // Package name
	Preamble    string              // C preamble (doc comment on import "C")
	Ref         []*Ref              // all references to C.xxx in AST
	Calls       []*Call             // all calls to C.xxx in AST
	ExpFunc     []*ExpFunc          // exported functions for this file
	Name        map[string]*Name    // map from Go name to Name
	NamePos     map[*Name]token.Pos // map from Name to position of the first reference
	NoCallbacks map[string]bool     // C function names with #cgo nocallback directive
	NoEscapes   map[string]bool     // C function names with #cgo noescape directive
	Edit        *edit.Buffer

	debugs []*debug // debug data from iterations of gccDebug. Initialized by File.loadDebug.
}

func (f *File) offset(p token.Pos) int {
	return fset.Position(p).Offset
}

func nameKeys(m map[string]*Name) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// A Call refers to a call of a C.xxx function in the AST.
type Call struct {
	Call     *ast.CallExpr
	Deferred bool
	Done     bool
}

// A Ref refers to an expression of the form C.xxx in the AST.
type Ref struct {
	Name    *Name
	Expr    *ast.Expr
	Context astContext
	Done    bool
}

func (r *Ref) Pos() token.Pos {
	return (*r.Expr).Pos()
}

var nameKinds = []string{"iconst", "fconst", "sconst", "type", "var", "fpvar", "func", "macro", "not-type"}

// A Name collects information about C.xxx.
type Name struct {
	Go       string // name used in Go referring to package C
	Mangle   string // name used in generated Go
	C        string // name used in C
	Define   string // #define expansion
	Kind     string // one of the nameKinds
	Type     *Type  // the type of xxx
	FuncType *FuncType
	AddError bool
	Const    string // constant definition
}

// IsVar reports whether Kind is either "var" or "fpvar"
func (n *Name) IsVar() bool {
	return n.Kind == "var" || n.Kind == "fpvar"
}

// IsConst reports whether Kind is either "iconst", "fconst" or "sconst"
func (n *Name) IsConst() bool {
	return strings.HasSuffix(n.Kind, "const")
}

// An ExpFunc is an exported function, callable from C.
// Such functions are identified in the Go input file
// by doc comments containing the line //export ExpName
type ExpFunc struct {
	Func    *ast.FuncDecl
	ExpName string // name to use from C
	Doc     string
}

// A TypeRepr contains the string representation of a type.
type TypeRepr struct {
	Repr       string
	FormatArgs []any
}

// A Type collects information about a type in both the C and Go worlds.
type Type struct {
	Size       int64
	Align      int64
	C          *TypeRepr
	Go         ast.Expr
	EnumValues map[string]int64
	Typedef    string
	BadPointer bool // this pointer type should be represented as a uintptr (deprecated)
}

func (t *Type) fuzzyMatch(t2 *Type) bool {
	if t == nil || t2 == nil {
		return false
	}
	return t.Size == t2.Size && t.Align == t2.Align
}

// A FuncType collects information about a function type in both the C and Go worlds.
type FuncType struct {
	Params []*Type
	Result *Type
	Go     *ast.FuncType
}

func (t *FuncType) fuzzyMatch(t2 *FuncType) bool {
	if t == nil || t2 == nil {
		return false
	}
	if !t.Result.fuzzyMatch(t2.Result) {
		return false
	}
	if len(t.Params) != len(t2.Params) {
		return false
	}
	for i := range t.Params {
		if !t.Params[i].fuzzyMatch(t2.Params[i]) {
			return false
		}
	}
	return true
}

func usage() {
	fmt.Fprint(os.Stderr, "usage: cgo -- [compiler options] file.go ...\n")
	flag.PrintDefaults()
	os.Exit(2)
}

var ptrSizeMap = map[string]int64{
	"386":      4,
	"alpha":    8,
	"amd64":    8,
	"arm":      4,
	"arm64":    8,
	"loong64":  8,
	"m68k":     4,
	"mips":     4,
	"mipsle":   4,
	"mips64":   8,
	"mips64le": 8,
	"nios2":    4,
	"ppc":      4,
	"ppc64":    8,
	"ppc64le":  8,
	"riscv":    4,
	"riscv64":  8,
	"s390":     4,
	"s390x":    8,
	"sh":       4,
	"shbe":     4,
	"sparc":    4,
	"sparc64":  8,
}

var intSizeMap = map[string]int64{
	"386":      4,
	"alpha":    8,
	"amd64":    8,
	"arm":      4,
	"arm64":    8,
	"loong64":  8,
	"m68k":     4,
	"mips":     4,
	"mipsle":   4,
	"mips64":   8,
	"mips64le": 8,
	"nios2":    4,
	"ppc":      4,
	"ppc64":    8,
	"ppc64le":  8,
	"riscv":    4,
	"riscv64":  8,
	"s390":     4,
	"s390x":    8,
	"sh":       4,
	"shbe":     4,
	"sparc":    4,
	"sparc64":  8,
}

var cPrefix string

var fset = token.NewFileSet()

var dynobj = flag.String("dynimport", "", "if non-empty, print dynamic import data for that file")
var dynout = flag.String("dynout", "", "write -dynimport output to this file")
var dynpackage = flag.String("dynpackage", "main", "set Go package for -dynimport output")
var dynlinker = flag.Bool("dynlinker", false, "record dynamic linker information in -dynimport mode")

// This flag is for bootstrapping a new Go implementation,
// to generate Go types that match the data layout and
// constant values used in the host's C libraries and system calls.
var godefs = flag.Bool("godefs", false, "for bootstrap: write Go definitions for C file to standard output")

var srcDir = flag.String("srcdir", "", "source directory")
var objDir = flag.String("objdir", "", "object directory")
var importPath = flag.String("importpath", "", "import path of package being built (for comments in generated files)")
var exportHeader = flag.String("exportheader", "", "where to write export header if any exported functions")

var ldflags = flag.String("ldflags", "", "flags to pass to C linker")

var gccgo = flag.Bool("gccgo", false, "generate files for use with gccgo")
var gccgoprefix = flag.String("gccgoprefix", "", "-fgo-prefix option used with gccgo")
var gccgopkgpath = flag.String("gccgopkgpath", "", "-fgo-pkgpath option used with gccgo")
var gccgoMangler func(string) string
var gccgoDefineCgoIncomplete = flag.Bool("gccgo_define_cgoincomplete", false, "define cgo.Incomplete for older gccgo/GoLLVM")
var importRuntimeCgo = flag.Bool("import_runtime_cgo", true, "import runtime/cgo in generated code")
var importSyscall = flag.Bool("import_syscall", true, "import syscall in generated code")
var trimpath = flag.String("trimpath", "", "applies supplied rewrites or trims prefixes to recorded source file paths")

var goarch, goos, gomips, gomips64 string
var gccBaseCmd []string

func main() {
	counter.Open()
	objabi.AddVersionFlag() // -V
	objabi.Flagparse(usage)
	counter.Inc("cgo/invocations")
	counter.CountFlags("cgo/flag:", *flag.CommandLine)

	if *gccgoDefineCgoIncomplete {
		if !*gccgo {
			fmt.Fprintf(os.Stderr, "cgo: -gccgo_define_cgoincomplete without -gccgo\n")
			os.Exit(2)
		}
		incomplete = "_cgopackage_Incomplete"
	}

	if *dynobj != "" {
		// cgo -dynimport is essentially a separate helper command
		// built into the cgo binary. It scans a gcc-produced executable
		// and dumps information about the imported symbols and the
		// imported libraries. The 'go build' rules for cgo prepare an
		// appropriate executable and then use its import information
		// instead of needing to make the linkers duplicate all the
		// specialized knowledge gcc has about where to look for imported
		// symbols and which ones to use.
		dynimport(*dynobj)
		return
	}

	if *godefs {
		// Generating definitions pulled from header files,
		// to be checked into Go repositories.
		// Line numbers are just noise.
		conf.Mode &^= printer.SourcePos
	}

	if *objDir == "" {
		*objDir = "_obj"
	}

	args := flag.Args()
	if len(args) < 1 {
		usage()
	}

	// Find first arg that looks like a go file and assume everything before
	// that are options to pass to gcc.
	var i int
	for i = len(args); i > 0; i-- {
		if !strings.HasSuffix(args[i-1], ".go") {
			break
		}
	}
	if i == len(args) {
		usage()
	}

	// Save original command line arguments for the godefs generated comment. Relative file
	// paths in os.Args will be rewritten to absolute file paths in the loop below.
	osArgs := make([]string, len(os.Args))
	copy(osArgs, os.Args[:])
	goFiles := args[i:]

	for _, arg := range args[:i] {
		if arg == "-fsanitize=thread" {
			tsanProlog = yesTsanProlog
		}
		if arg == "-fsanitize=memory" {
			msanProlog = yesMsanProlog
		}
	}

	p := newPackage(args[:i])

	// We need a C compiler to be available. Check this.
	var err error
	gccBaseCmd, err = checkGCCBaseCmd()
	if err != nil {
		fatalf("%v", err)
		os.Exit(2)
	}

	// Record linker flags for external linking.
	if *ldflags != "" {
		args, err := splitQuoted(*ldflags)
		if err != nil {
			fatalf("bad -ldflags option: %q (%s)", *ldflags, err)
		}
		p.addToFlag("LDFLAGS", args)
	}

	// For backward compatibility for Bazel, record CGO_LDFLAGS
	// from the environment for external linking.
	// This should not happen with cmd/go, which removes CGO_LDFLAGS
	// from the environment when invoking cgo.
	// This can be removed when we no longer need to support
	// older versions of Bazel. See issue #66456 and
	// https://github.com/bazelbuild/rules_go/issues/3979.
	if envFlags := os.Getenv("CGO_LDFLAGS"); envFlags != "" {
		args, err := splitQuoted(envFlags)
		if err != nil {
			fatalf("bad CGO_LDFLAGS: %q (%s)", envFlags, err)
		}
		p.addToFlag("LDFLAGS", args)
	}

	// Need a unique prefix for the global C symbols that
	// we use to coordinate between gcc and ourselves.
	// We already put _cgo_ at the beginning, so the main
	// concern is other cgo wrappers for the same functions.
	// Use the beginning of the 16 bytes hash of the input to disambiguate.
	h := hash.New32()
	io.WriteString(h, *importPath)
	var once sync.Once
	q := par.NewQueue(runtime.GOMAXPROCS(0))
	fs := make([]*File, len(goFiles))
	for i, input := range goFiles {
		if *srcDir != "" {
			input = filepath.Join(*srcDir, input)
		}

		// Create absolute path for file, so that it will be used in error
		// messages and recorded in debug line number information.
		// This matches the rest of the toolchain. See golang.org/issue/5122.
		if aname, err := filepath.Abs(input); err == nil {
			input = aname
		}

		b, err := os.ReadFile(input)
		if err != nil {
			fatalf("%s", err)
		}
		if _, err = h.Write(b); err != nil {
			fatalf("%s", err)
		}

		q.Add(func() {
			// Apply trimpath to the file path. The path won't be read from after this point.
			input, _ = objabi.ApplyRewrites(input, *trimpath)
			if strings.ContainsAny(input, "\r\n") {
				// ParseGo, (*Package).writeOutput, and printer.Fprint in SourcePos mode
				// all emit line directives, which don't permit newlines in the file path.
				// Bail early if we see anything newline-like in the trimmed path.
				fatalf("input path contains newline character: %q", input)
			}
			goFiles[i] = input

			f := new(File)
			f.Edit = edit.NewBuffer(b)
			f.ParseGo(input, b)
			f.ProcessCgoDirectives()
			gccIsClang := f.loadDefines(p.GccOptions)
			once.Do(func() {
				p.GccIsClang = gccIsClang
			})

			fs[i] = f

			f.loadDebug(p)
		})
	}

	<-q.Idle()

	cPrefix = fmt.Sprintf("_%x", h.Sum(nil)[0:6])

	for i, input := range goFiles {
		f := fs[i]
		p.Translate(f)
		for _, cref := range f.Ref {
			switch cref.Context {
			case ctxCall, ctxCall2:
				if cref.Name.Kind != "type" {
					break
				}
				old := *cref.Expr
				*cref.Expr = cref.Name.Type.Go
				f.Edit.Replace(f.offset(old.Pos()), f.offset(old.End()), gofmt(cref.Name.Type.Go))
			}
		}
		if nerrors > 0 {
			os.Exit(2)
		}
		p.PackagePath = f.Package
		p.Record(f)
		if *godefs {
			os.Stdout.WriteString(p.godefs(f, osArgs))
		} else {
			p.writeOutput(f, input)
		}
	}
	cFunctions := make(map[string]bool)
	for _, key := range nameKeys(p.Name) {
		n := p.Name[key]
		if n.FuncType != nil {
			cFunctions[n.C] = true
		}
	}

	for funcName := range p.noEscapes {
		if _, found := cFunctions[funcName]; !found {
			error_(token.NoPos, "#cgo noescape %s: no matched C function", funcName)
		}
	}

	for funcName := range p.noCallbacks {
		if _, found := cFunctions[funcName]; !found {
			error_(token.NoPos, "#cgo nocallback %s: no matched C function", funcName)
		}
	}

	if !*godefs {
		p.writeDefs()
	}
	if nerrors > 0 {
		os.Exit(2)
	}
}

// newPackage returns a new Package that will invoke
// gcc with the additional arguments specified in args.
func newPackage(args []string) *Package {
	goarch = runtime.GOARCH
	if s := os.Getenv("GOARCH"); s != "" {
		goarch = s
	}
	goos = runtime.GOOS
	if s := os.Getenv("GOOS"); s != "" {
		goos = s
	}
	buildcfg.Check()
	gomips = buildcfg.GOMIPS
	gomips64 = buildcfg.GOMIPS64
	ptrSize := ptrSizeMap[goarch]
	if ptrSize == 0 {
		fatalf("unknown ptrSize for $GOARCH %q", goarch)
	}
	intSize := intSizeMap[goarch]
	if intSize == 0 {
		fatalf("unknown intSize for $GOARCH %q", goarch)
	}

	// Reset locale variables so gcc emits English errors [sic].
	os.Setenv("LANG", "en_US.UTF-8")
	os.Setenv("LC_ALL", "C")

	p := &Package{
		PtrSize:     ptrSize,
		IntSize:     intSize,
		Written:     make(map[string]bool),
		noCallbacks: make(map[string]bool),
		noEscapes:   make(map[string]bool),
	}
	p.addToFlag("CFLAGS", args)
	return p
}

// Record what needs to be recorded about f.
func (p *Package) Record(f *File) {
	if p.PackageName == "" {
		p.PackageName = f.Package
	} else if p.PackageName != f.Package {
		error_(token.NoPos, "inconsistent package names: %s, %s", p.PackageName, f.Package)
	}

	if p.Name == nil {
		p.Name = f.Name
	} else {
		// Merge the new file's names in with the existing names.
		for k, v := range f.Name {
			if p.Name[k] == nil {
				// Never seen before, just save it.
				p.Name[k] = v
			} else if p.incompleteTypedef(p.Name[k].Type) && p.Name[k].FuncType == nil {
				// Old one is incomplete, just use new one.
				p.Name[k] = v
			} else if p.incompleteTypedef(v.Type) && v.FuncType == nil {
				// New one is incomplete, just use old one.
				// Nothing to do.
			} else if _, ok := nameToC[k]; ok {
				// Names we predefine may appear inconsistent
				// if some files typedef them and some don't.
				// Issue 26743.
			} else if !reflect.DeepEqual(p.Name[k], v) {
				// We don't require strict func type equality, because some functions
				// can have things like typedef'd arguments that are equivalent to
				// the standard arguments. e.g.
				//     int usleep(unsigned);
				//     int usleep(useconds_t);
				// So we just check size/alignment of arguments. At least that
				// avoids problems like those in #67670 and #67699.
				ok := false
				ft1 := p.Name[k].FuncType
				ft2 := v.FuncType
				if ft1.fuzzyMatch(ft2) {
					// Retry DeepEqual with the FuncType field cleared.
					x1 := *p.Name[k]
					x2 := *v
					x1.FuncType = nil
					x2.FuncType = nil
					if reflect.DeepEqual(&x1, &x2) {
						ok = true
					}
				}
				if !ok {
					error_(token.NoPos, "inconsistent definitions for C.%s", fixGo(k))
				}
			}
		}
	}

	// merge nocallback & noescape
	maps.Copy(p.noCallbacks, f.NoCallbacks)
	maps.Copy(p.noEscapes, f.NoEscapes)

	if f.ExpFunc != nil {
		p.ExpFunc = append(p.ExpFunc, f.ExpFunc...)
		p.Preamble += "\n" + f.Preamble
	}
	p.Decl = append(p.Decl, f.AST.Decls...)
}

// incompleteTypedef reports whether t appears to be an incomplete
// typedef definition.
func (p *Package) incompleteTypedef(t *Type) bool {
	return t == nil || (t.Size == 0 && t.Align == -1)
}

```

// === FILE: references!/go/src/cmd/cgo/out.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"cmd/internal/pkgpath"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"internal/xcoff"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	conf         = printer.Config{Mode: printer.SourcePos, Tabwidth: 8}
	noSourceConf = printer.Config{Tabwidth: 8}
)

// writeDefs creates output files to be compiled by gc and gcc.
func (p *Package) writeDefs() {
	var fgo2, fc io.Writer
	f := creat("_cgo_gotypes.go")
	defer f.Close()
	fgo2 = f
	if *gccgo {
		f := creat("_cgo_defun.c")
		defer f.Close()
		fc = f
	}
	fm := creat("_cgo_main.c")

	var gccgoInit strings.Builder

	if !*gccgo {
		for _, arg := range p.LdFlags {
			fmt.Fprintf(fgo2, "//go:cgo_ldflag %q\n", arg)
		}
	} else {
		fflg := creat("_cgo_flags")
		for _, arg := range p.LdFlags {
			fmt.Fprintf(fflg, "_CGO_LDFLAGS=%s\n", arg)
		}
		fflg.Close()
	}

	// Write C main file for using gcc to resolve imports.
	fmt.Fprintf(fm, "#include <stddef.h>\n") // For size_t below.
	fmt.Fprintf(fm, "int main(int argc __attribute__((unused)), char **argv __attribute__((unused))) { return 0; }\n")
	if *importRuntimeCgo {
		fmt.Fprintf(fm, "void crosscall2(void(*fn)(void*) __attribute__((unused)), void *a __attribute__((unused)), int c __attribute__((unused)), size_t ctxt __attribute__((unused))) { }\n")
		fmt.Fprintf(fm, "size_t _cgo_wait_runtime_init_done(void) { return 0; }\n")
		fmt.Fprintf(fm, "void _cgo_release_context(size_t ctxt __attribute__((unused))) { }\n")
		fmt.Fprintf(fm, "char* _cgo_topofstack(void) { return (char*)0; }\n")
	} else {
		// If we're not importing runtime/cgo, we *are* runtime/cgo,
		// which provides these functions. We just need a prototype.
		fmt.Fprintf(fm, "void crosscall2(void(*fn)(void*), void *a, int c, size_t ctxt);\n")
		fmt.Fprintf(fm, "size_t _cgo_wait_runtime_init_done(void);\n")
		fmt.Fprintf(fm, "void _cgo_release_context(size_t);\n")
	}
	fmt.Fprintf(fm, "void _cgo_allocate(void *a __attribute__((unused)), int c __attribute__((unused))) { }\n")
	fmt.Fprintf(fm, "void _cgo_panic(void *a __attribute__((unused)), int c __attribute__((unused))) { }\n")
	fmt.Fprintf(fm, "void _cgo_reginit(void) { }\n")

	// Write second Go output: definitions of _C_xxx.
	// In a separate file so that the import of "unsafe" does not
	// pollute the original file.
	fmt.Fprintf(fgo2, "// Code generated by cmd/cgo; DO NOT EDIT.\n\n")
	fmt.Fprintf(fgo2, "package %s\n\n", p.PackageName)
	fmt.Fprintf(fgo2, "import \"unsafe\"\n\n")
	if *importSyscall {
		fmt.Fprintf(fgo2, "import \"syscall\"\n\n")
	}
	if *importRuntimeCgo {
		if !*gccgoDefineCgoIncomplete {
			fmt.Fprintf(fgo2, "import _cgopackage \"runtime/cgo\"\n\n")
			fmt.Fprintf(fgo2, "type _ _cgopackage.Incomplete\n") // prevent import-not-used error
		} else {
			fmt.Fprintf(fgo2, "//go:notinheap\n")
			fmt.Fprintf(fgo2, "type _cgopackage_Incomplete struct{ _ struct{ _ struct{} } }\n")
		}
	}
	if *importSyscall {
		fmt.Fprintf(fgo2, "var _ syscall.Errno\n")
	}
	fmt.Fprintf(fgo2, "func _Cgo_ptr(ptr unsafe.Pointer) unsafe.Pointer { return ptr }\n\n")

	if !*gccgo {
		fmt.Fprintf(fgo2, "//go:linkname _Cgo_always_false runtime.cgoAlwaysFalse\n")
		fmt.Fprintf(fgo2, "var _Cgo_always_false bool\n")
		fmt.Fprintf(fgo2, "//go:linkname _Cgo_use runtime.cgoUse\n")
		fmt.Fprintf(fgo2, "func _Cgo_use(interface{})\n")
		fmt.Fprintf(fgo2, "//go:linkname _Cgo_keepalive runtime.cgoKeepAlive\n")
		fmt.Fprintf(fgo2, "//go:noescape\n")
		fmt.Fprintf(fgo2, "func _Cgo_keepalive(interface{})\n")
	}
	fmt.Fprintf(fgo2, "//go:linkname _Cgo_no_callback runtime.cgoNoCallback\n")
	fmt.Fprintf(fgo2, "func _Cgo_no_callback(bool)\n")

	typedefNames := make([]string, 0, len(typedef))
	for name := range typedef {
		if name == "_Ctype_void" {
			// We provide an appropriate declaration for
			// _Ctype_void below (#39877).
			continue
		}
		typedefNames = append(typedefNames, name)
	}
	sort.Strings(typedefNames)
	for _, name := range typedefNames {
		def := typedef[name]
		fmt.Fprintf(fgo2, "type %s ", name)
		// We don't have source info for these types, so write them out without source info.
		// Otherwise types would look like:
		//
		// type _Ctype_struct_cb struct {
		// //line :1
		//        on_test *[0]byte
		// //line :1
		// }
		//
		// Which is not useful. Moreover we never override source info,
		// so subsequent source code uses the same source info.
		// Moreover, empty file name makes compile emit no source debug info at all.
		var buf bytes.Buffer
		noSourceConf.Fprint(&buf, fset, def.Go)
		if bytes.HasPrefix(buf.Bytes(), []byte("_Ctype_")) ||
			strings.HasPrefix(name, "_Ctype_enum_") ||
			strings.HasPrefix(name, "_Ctype_union_") {
			// This typedef is of the form `typedef a b` and should be an alias.
			fmt.Fprintf(fgo2, "= ")
		}
		fmt.Fprintf(fgo2, "%s", buf.Bytes())
		fmt.Fprintf(fgo2, "\n\n")
	}
	if *gccgo {
		fmt.Fprintf(fgo2, "type _Ctype_void byte\n")
	} else {
		fmt.Fprintf(fgo2, "type _Ctype_void [0]byte\n")
	}

	if *gccgo {
		fmt.Fprint(fgo2, gccgoGoProlog)
		fmt.Fprint(fc, p.cPrologGccgo())
	} else {
		fmt.Fprint(fgo2, goProlog)
	}

	if fc != nil {
		fmt.Fprintf(fc, "#line 1 \"cgo-generated-wrappers\"\n")
	}
	if fm != nil {
		fmt.Fprintf(fm, "#line 1 \"cgo-generated-wrappers\"\n")
	}

	gccgoSymbolPrefix := p.gccgoSymbolPrefix()

	cVars := make(map[string]bool)
	for _, key := range nameKeys(p.Name) {
		n := p.Name[key]
		if !n.IsVar() {
			continue
		}

		if !cVars[n.C] {
			if *gccgo {
				fmt.Fprintf(fc, "extern byte *%s;\n", n.C)
			} else {
				// Force a reference to all symbols so that
				// the external linker will add DT_NEEDED
				// entries as needed on ELF systems.
				// Treat function variables differently
				// to avoid type conflict errors from LTO
				// (Link Time Optimization).
				if n.Kind == "fpvar" {
					fmt.Fprintf(fm, "extern void %s();\n", n.C)
				} else {
					fmt.Fprintf(fm, "extern char %s[];\n", n.C)
					fmt.Fprintf(fm, "void *_cgohack_%s = %s;\n\n", n.C, n.C)
				}
				fmt.Fprintf(fgo2, "//go:linkname __cgo_%s %s\n", n.C, n.C)
				fmt.Fprintf(fgo2, "//go:cgo_import_static %s\n", n.C)
				fmt.Fprintf(fgo2, "var __cgo_%s byte\n", n.C)
			}
			cVars[n.C] = true
		}

		var node ast.Node
		if n.Kind == "var" {
			node = &ast.StarExpr{X: n.Type.Go}
		} else if n.Kind == "fpvar" {
			node = n.Type.Go
		} else {
			panic(fmt.Errorf("invalid var kind %q", n.Kind))
		}
		if *gccgo {
			fmt.Fprintf(fc, `extern void *%s __asm__("%s.%s");`, n.Mangle, gccgoSymbolPrefix, gccgoToSymbol(n.Mangle))
			fmt.Fprintf(&gccgoInit, "\t%s = &%s;\n", n.Mangle, n.C)
			fmt.Fprintf(fc, "\n")
		}

		fmt.Fprintf(fgo2, "var %s ", n.Mangle)
		conf.Fprint(fgo2, fset, node)
		if !*gccgo {
			fmt.Fprintf(fgo2, " = (")
			conf.Fprint(fgo2, fset, node)
			fmt.Fprintf(fgo2, ")(unsafe.Pointer(&__cgo_%s))", n.C)
		}
		fmt.Fprintf(fgo2, "\n")
	}
	if *gccgo {
		fmt.Fprintf(fc, "\n")
	}

	for _, key := range nameKeys(p.Name) {
		n := p.Name[key]
		if n.Const != "" {
			fmt.Fprintf(fgo2, "const %s = %s\n", n.Mangle, n.Const)
		}
	}
	fmt.Fprintf(fgo2, "\n")

	callsMalloc := false
	for _, key := range nameKeys(p.Name) {
		n := p.Name[key]
		if n.FuncType != nil {
			p.writeDefsFunc(fgo2, n, &callsMalloc)
		}
	}

	fgcc := creat("_cgo_export.c")
	fgcch := creat("_cgo_export.h")
	if *gccgo {
		p.writeGccgoExports(fgo2, fm, fgcc, fgcch)
	} else {
		p.writeExports(fgo2, fm, fgcc, fgcch)
	}

	if callsMalloc && !*gccgo {
		fmt.Fprint(fgo2, strings.ReplaceAll(cMallocDefGo, "PREFIX", cPrefix))
		fmt.Fprint(fgcc, strings.ReplaceAll(strings.Replace(cMallocDefC, "PREFIX", cPrefix, -1), "PACKED", p.packedAttribute()))
	}

	if err := fgcc.Close(); err != nil {
		fatalf("%s", err)
	}
	if err := fgcch.Close(); err != nil {
		fatalf("%s", err)
	}

	if *exportHeader != "" && len(p.ExpFunc) > 0 {
		fexp, err := os.Create(*exportHeader)
		if err != nil {
			fatalf("%s", err)
		}
		fgcch, err := os.Open(filepath.Join(outputDir(), "_cgo_export.h"))
		if err != nil {
			fatalf("%s", err)
		}
		defer fgcch.Close()
		_, err = io.Copy(fexp, fgcch)
		if err != nil {
			fatalf("%s", err)
		}
		if err = fexp.Close(); err != nil {
			fatalf("%s", err)
		}
	}

	init := gccgoInit.String()
	if init != "" {
		// The init function does nothing but simple
		// assignments, so it won't use much stack space, so
		// it's OK to not split the stack. Splitting the stack
		// can run into a bug in clang (as of 2018-11-09):
		// this is a leaf function, and when clang sees a leaf
		// function it won't emit the split stack prologue for
		// the function. However, if this function refers to a
		// non-split-stack function, which will happen if the
		// cgo code refers to a C function not compiled with
		// -fsplit-stack, then the linker will think that it
		// needs to adjust the split stack prologue, but there
		// won't be one. Marking the function explicitly
		// no_split_stack works around this problem by telling
		// the linker that it's OK if there is no split stack
		// prologue.
		fmt.Fprintln(fc, "static void init(void) __attribute__ ((constructor, no_split_stack));")
		fmt.Fprintln(fc, "static void init(void) {")
		fmt.Fprint(fc, init)
		fmt.Fprintln(fc, "}")
	}
}

// elfImportedSymbols is like elf.File.ImportedSymbols, but it
// includes weak symbols.
//
// A bug in some versions of LLD (at least LLD 8) cause it to emit
// several pthreads symbols as weak, but we need to import those. See
// issue #31912 or https://bugs.llvm.org/show_bug.cgi?id=42442.
//
// When doing external linking, we hand everything off to the external
// linker, which will create its own dynamic symbol tables. For
// internal linking, this may turn weak imports into strong imports,
// which could cause dynamic linking to fail if a symbol really isn't
// defined. However, the standard library depends on everything it
// imports, and this is the primary use of dynamic symbol tables with
// internal linking.
func elfImportedSymbols(f *elf.File) []elf.ImportedSymbol {
	syms, _ := f.DynamicSymbols()
	var imports []elf.ImportedSymbol
	for _, s := range syms {
		if (elf.ST_BIND(s.Info) == elf.STB_GLOBAL || elf.ST_BIND(s.Info) == elf.STB_WEAK) && s.Section == elf.SHN_UNDEF {
			imports = append(imports, elf.ImportedSymbol{
				Name:    s.Name,
				Library: s.Library,
				Version: s.Version,
			})
		}
	}
	return imports
}

func dynimport(obj string) {
	stdout := os.Stdout
	if *dynout != "" {
		f, err := os.Create(*dynout)
		if err != nil {
			fatalf("%s", err)
		}
		defer func() {
			if err = f.Close(); err != nil {
				fatalf("error closing %s: %v", *dynout, err)
			}
		}()

		stdout = f
	}

	fmt.Fprintf(stdout, "package %s\n", *dynpackage)

	if f, err := elf.Open(obj); err == nil {
		defer f.Close()
		if *dynlinker {
			// Emit the cgo_dynamic_linker line.
			if sec := f.Section(".interp"); sec != nil {
				if data, err := sec.Data(); err == nil && len(data) > 1 {
					// skip trailing \0 in data
					fmt.Fprintf(stdout, "//go:cgo_dynamic_linker %q\n", string(data[:len(data)-1]))
				}
			}
		}
		sym := elfImportedSymbols(f)
		for _, s := range sym {
			targ := s.Name
			if s.Version != "" {
				targ += "#" + s.Version
			}
			checkImportSymName(s.Name)
			checkImportSymName(targ)
			fmt.Fprintf(stdout, "//go:cgo_import_dynamic %s %s %q\n", s.Name, targ, s.Library)
		}
		lib, _ := f.ImportedLibraries()
		for _, l := range lib {
			fmt.Fprintf(stdout, "//go:cgo_import_dynamic _ _ %q\n", l)
		}
		return
	}

	if f, err := macho.Open(obj); err == nil {
		defer f.Close()
		sym, _ := f.ImportedSymbols()
		for _, s := range sym {
			s = strings.TrimPrefix(s, "_")
			checkImportSymName(s)
			fmt.Fprintf(stdout, "//go:cgo_import_dynamic %s %s %q\n", s, s, "")
		}
		lib, _ := f.ImportedLibraries()
		for _, l := range lib {
			fmt.Fprintf(stdout, "//go:cgo_import_dynamic _ _ %q\n", l)
		}
		return
	}

	if f, err := pe.Open(obj); err == nil {
		defer f.Close()
		sym, _ := f.ImportedSymbols()
		for _, s := range sym {
			ss := strings.Split(s, ":")
			name := strings.Split(ss[0], "@")[0]
			checkImportSymName(name)
			checkImportSymName(ss[0])
			fmt.Fprintf(stdout, "//go:cgo_import_dynamic %s %s %q\n", name, ss[0], strings.ToLower(ss[1]))
		}
		return
	}

	if f, err := xcoff.Open(obj); err == nil {
		defer f.Close()
		sym, err := f.ImportedSymbols()
		if err != nil {
			fatalf("cannot load imported symbols from XCOFF file %s: %v", obj, err)
		}
		for _, s := range sym {
			if s.Name == "runtime_rt0_go" || s.Name == "_rt0_ppc64_aix_lib" {
				// These symbols are imported by runtime/cgo but
				// must not be added to _cgo_import.go as there are
				// Go symbols.
				continue
			}
			checkImportSymName(s.Name)
			fmt.Fprintf(stdout, "//go:cgo_import_dynamic %s %s %q\n", s.Name, s.Name, s.Library)
		}
		lib, err := f.ImportedLibraries()
		if err != nil {
			fatalf("cannot load imported libraries from XCOFF file %s: %v", obj, err)
		}
		for _, l := range lib {
			fmt.Fprintf(stdout, "//go:cgo_import_dynamic _ _ %q\n", l)
		}
		return
	}

	fatalf("cannot parse %s as ELF, Mach-O, PE or XCOFF", obj)
}

// checkImportSymName checks a symbol name we are going to emit as part
// of a //go:cgo_import_dynamic pragma. These names come from object
// files, so they may be corrupt. We are going to emit them unquoted,
// so while they don't need to be valid symbol names (and in some cases,
// involving symbol versions, they won't be) they must contain only
// graphic characters and must not contain Go comments.
func checkImportSymName(s string) {
	for _, c := range s {
		if !unicode.IsGraphic(c) || unicode.IsSpace(c) {
			fatalf("dynamic symbol %q contains unsupported character", s)
		}
	}
	if strings.Contains(s, "//") || strings.Contains(s, "/*") {
		fatalf("dynamic symbol %q contains Go comment", s)
	}
}

// Construct a gcc struct matching the gc argument frame.
// Assumes that in gcc, char is 1 byte, short 2 bytes, int 4 bytes, long long 8 bytes.
// These assumptions are checked by the gccProlog.
// Also assumes that gc convention is to word-align the
// input and output parameters.
func (p *Package) structType(n *Name) (string, int64) {
	// It's possible for us to see a type with a top-level const here,
	// which will give us an unusable struct type. See #75751.
	// The top-level const will always appear as a final qualifier,
	// constructed by typeConv.loadType in the dwarf.QualType case.
	// The top-level const is meaningless here and can simply be removed.
	stripConst := func(s string) string {
		i := strings.LastIndex(s, "const")
		if i == -1 {
			return s
		}

		// A top-level const can only be followed by other qualifiers.
		if r, ok := strings.CutSuffix(s, "const"); ok {
			return strings.TrimSpace(r)
		}

		var nonConst []string
		for _, f := range strings.Fields(s[i:]) {
			switch f {
			case "const":
			case "restrict", "volatile":
				nonConst = append(nonConst, f)
			default:
				return s
			}
		}

		return strings.TrimSpace(s[:i]) + " " + strings.Join(nonConst, " ")
	}

	var buf strings.Builder
	fmt.Fprint(&buf, "struct {\n")
	off := int64(0)
	for i, t := range n.FuncType.Params {
		if off%t.Align != 0 {
			pad := t.Align - off%t.Align
			fmt.Fprintf(&buf, "\t\tchar __pad%d[%d];\n", off, pad)
			off += pad
		}
		c := t.Typedef
		if c == "" {
			c = stripConst(t.C.String())
		}
		fmt.Fprintf(&buf, "\t\t%s p%d;\n", c, i)
		off += t.Size
	}
	if off%p.PtrSize != 0 {
		pad := p.PtrSize - off%p.PtrSize
		fmt.Fprintf(&buf, "\t\tchar __pad%d[%d];\n", off, pad)
		off += pad
	}
	if t := n.FuncType.Result; t != nil {
		if off%t.Align != 0 {
			pad := t.Align - off%t.Align
			fmt.Fprintf(&buf, "\t\tchar __pad%d[%d];\n", off, pad)
			off += pad
		}
		fmt.Fprintf(&buf, "\t\t%s r;\n", stripConst(t.C.String()))
		off += t.Size
	}
	if off%p.PtrSize != 0 {
		pad := p.PtrSize - off%p.PtrSize
		fmt.Fprintf(&buf, "\t\tchar __pad%d[%d];\n", off, pad)
		off += pad
	}
	if off == 0 {
		fmt.Fprintf(&buf, "\t\tchar unused;\n") // avoid empty struct
	}
	fmt.Fprintf(&buf, "\t}")
	return buf.String(), off
}

func (p *Package) writeDefsFunc(fgo2 io.Writer, n *Name, callsMalloc *bool) {
	name := n.Go
	gtype := n.FuncType.Go
	void := gtype.Results == nil || len(gtype.Results.List) == 0
	if n.AddError {
		// Add "error" to return type list.
		// Type list is known to be 0 or 1 element - it's a C function.
		err := &ast.Field{Type: ast.NewIdent("error")}
		l := gtype.Results.List
		if len(l) == 0 {
			l = []*ast.Field{err}
		} else {
			l = []*ast.Field{l[0], err}
		}
		t := new(ast.FuncType)
		*t = *gtype
		t.Results = &ast.FieldList{List: l}
		gtype = t
	}

	// Go func declaration.
	d := &ast.FuncDecl{
		Name: ast.NewIdent(n.Mangle),
		Type: gtype,
	}

	// Builtins defined in the C prolog.
	inProlog := builtinDefs[name] != ""
	cname := fmt.Sprintf("_cgo%s%s", cPrefix, n.Mangle)
	paramnames := []string(nil)
	if d.Type.Params != nil {
		for i, param := range d.Type.Params.List {
			paramName := fmt.Sprintf("p%d", i)
			param.Names = []*ast.Ident{ast.NewIdent(paramName)}
			paramnames = append(paramnames, paramName)
		}
	}

	if *gccgo {
		// Gccgo style hooks.
		fmt.Fprint(fgo2, "\n")
		conf.Fprint(fgo2, fset, d)
		fmt.Fprint(fgo2, " {\n")
		if !inProlog {
			fmt.Fprint(fgo2, "\tdefer syscall.CgocallDone()\n")
			fmt.Fprint(fgo2, "\tsyscall.Cgocall()\n")
		}
		if n.AddError {
			fmt.Fprint(fgo2, "\tsyscall.SetErrno(0)\n")
		}
		fmt.Fprint(fgo2, "\t")
		if !void {
			fmt.Fprint(fgo2, "r := ")
		}
		fmt.Fprintf(fgo2, "%s(%s)\n", cname, strings.Join(paramnames, ", "))

		if n.AddError {
			fmt.Fprint(fgo2, "\te := syscall.GetErrno()\n")
			fmt.Fprint(fgo2, "\tif e != 0 {\n")
			fmt.Fprint(fgo2, "\t\treturn ")
			if !void {
				fmt.Fprint(fgo2, "r, ")
			}
			fmt.Fprint(fgo2, "e\n")
			fmt.Fprint(fgo2, "\t}\n")
			fmt.Fprint(fgo2, "\treturn ")
			if !void {
				fmt.Fprint(fgo2, "r, ")
			}
			fmt.Fprint(fgo2, "nil\n")
		} else if !void {
			fmt.Fprint(fgo2, "\treturn r\n")
		}

		fmt.Fprint(fgo2, "}\n")

		// declare the C function.
		fmt.Fprintf(fgo2, "//extern %s\n", cname)
		d.Name = ast.NewIdent(cname)
		if n.AddError {
			l := d.Type.Results.List
			d.Type.Results.List = l[:len(l)-1]
		}
		conf.Fprint(fgo2, fset, d)
		fmt.Fprint(fgo2, "\n")

		return
	}

	if inProlog {
		fmt.Fprint(fgo2, builtinDefs[name])
		if strings.Contains(builtinDefs[name], "_cgo_cmalloc") {
			*callsMalloc = true
		}
		return
	}

	// Wrapper calls into gcc, passing a pointer to the argument frame.
	fmt.Fprintf(fgo2, "//go:cgo_import_static %s\n", cname)
	fmt.Fprintf(fgo2, "//go:linkname __cgofn_%s %s\n", cname, cname)
	fmt.Fprintf(fgo2, "var __cgofn_%s byte\n", cname)
	fmt.Fprintf(fgo2, "var %s = unsafe.Pointer(&__cgofn_%s)\n", cname, cname)

	nret := 0
	if !void {
		d.Type.Results.List[0].Names = []*ast.Ident{ast.NewIdent("r1")}
		nret = 1
	}
	if n.AddError {
		d.Type.Results.List[nret].Names = []*ast.Ident{ast.NewIdent("r2")}
	}

	fmt.Fprint(fgo2, "\n")
	fmt.Fprint(fgo2, "//go:cgo_unsafe_args\n")
	conf.Fprint(fgo2, fset, d)
	fmt.Fprint(fgo2, " {\n")

	// NOTE: Using uintptr to hide from escape analysis.
	arg := "0"
	if len(paramnames) > 0 {
		arg = "uintptr(unsafe.Pointer(&p0))"
	} else if !void {
		arg = "uintptr(unsafe.Pointer(&r1))"
	}

	noCallback := p.noCallbacks[n.C]
	if noCallback {
		// disable cgocallback, will check it in runtime.
		fmt.Fprintf(fgo2, "\t_Cgo_no_callback(true)\n")
	}

	prefix := ""
	if n.AddError {
		prefix = "errno := "
	}
	fmt.Fprintf(fgo2, "\t%s_cgo_runtime_cgocall(%s, %s)\n", prefix, cname, arg)
	if n.AddError {
		fmt.Fprintf(fgo2, "\tif errno != 0 { r2 = syscall.Errno(errno) }\n")
	}
	if noCallback {
		fmt.Fprintf(fgo2, "\t_Cgo_no_callback(false)\n")
	}

	// Use _Cgo_keepalive instead of _Cgo_use when noescape & nocallback exist,
	// so that the compiler won't force to escape them to heap.
	// Instead, make the compiler keep them alive by using _Cgo_keepalive.
	touchFunc := "_Cgo_use"
	if p.noEscapes[n.C] && p.noCallbacks[n.C] {
		touchFunc = "_Cgo_keepalive"
	}

	if len(paramnames) > 0 {
		fmt.Fprintf(fgo2, "\tif _Cgo_always_false {\n")
		for _, name := range paramnames {
			fmt.Fprintf(fgo2, "\t\t%s(%s)\n", touchFunc, name)
		}
		fmt.Fprintf(fgo2, "\t}\n")
	}

	fmt.Fprintf(fgo2, "\treturn\n")
	fmt.Fprintf(fgo2, "}\n")
}

// writeOutput creates stubs for a specific source file to be compiled by gc
func (p *Package) writeOutput(f *File, srcfile string) {
	base := srcfile
	base = strings.TrimSuffix(base, ".go")
	base = filepath.Base(base)
	fgo1 := creat(base + ".cgo1.go")
	fgcc := creat(base + ".cgo2.c")

	p.GoFiles = append(p.GoFiles, base+".cgo1.go")
	p.GccFiles = append(p.GccFiles, base+".cgo2.c")

	// Write Go output: Go input with rewrites of C.xxx to _C_xxx.
	fmt.Fprintf(fgo1, "// Code generated by cmd/cgo; DO NOT EDIT.\n\n")
	if strings.ContainsAny(srcfile, "\r\n") {
		// This should have been checked when the file path was first resolved,
		// but we double check here just to be sure.
		fatalf("internal error: writeOutput: srcfile contains unexpected newline character: %q", srcfile)
	}
	fmt.Fprintf(fgo1, "//line %s:1:1\n", srcfile)
	fgo1.Write(f.Edit.Bytes())

	// While we process the vars and funcs, also write gcc output.
	// Gcc output starts with the preamble.
	fmt.Fprintf(fgcc, "%s\n", builtinProlog)
	fmt.Fprintf(fgcc, "%s\n", f.Preamble)
	fmt.Fprintf(fgcc, "%s\n", gccProlog)
	fmt.Fprintf(fgcc, "%s\n", tsanProlog)
	fmt.Fprintf(fgcc, "%s\n", msanProlog)

	for _, key := range nameKeys(f.Name) {
		n := f.Name[key]
		if n.FuncType != nil {
			p.writeOutputFunc(fgcc, n)
		}
	}

	fgo1.Close()
	fgcc.Close()
}

// fixGo converts the internal Name.Go field into the name we should show
// to users in error messages. There's only one for now: on input we rewrite
// C.malloc into C._CMalloc, so change it back here.
func fixGo(name string) string {
	if name == "_CMalloc" {
		return "malloc"
	}
	return name
}

var isBuiltin = map[string]bool{
	"_Cfunc_CString":   true,
	"_Cfunc_CBytes":    true,
	"_Cfunc_GoString":  true,
	"_Cfunc_GoStringN": true,
	"_Cfunc_GoBytes":   true,
	"_Cfunc__CMalloc":  true,
}

func (p *Package) writeOutputFunc(fgcc *os.File, n *Name) {
	name := n.Mangle
	if isBuiltin[name] || p.Written[name] {
		// The builtins are already defined in the C prolog, and we don't
		// want to duplicate function definitions we've already done.
		return
	}
	p.Written[name] = true

	if *gccgo {
		p.writeGccgoOutputFunc(fgcc, n)
		return
	}

	ctype, _ := p.structType(n)

	// Gcc wrapper unpacks the C argument struct
	// and calls the actual C function.
	fmt.Fprintf(fgcc, "CGO_NO_SANITIZE_THREAD\n")
	if n.AddError {
		fmt.Fprintf(fgcc, "int\n")
	} else {
		fmt.Fprintf(fgcc, "void\n")
	}
	fmt.Fprintf(fgcc, "_cgo%s%s(void *v)\n", cPrefix, n.Mangle)
	fmt.Fprintf(fgcc, "{\n")
	if n.AddError {
		fmt.Fprintf(fgcc, "\tint _cgo_errno;\n")
	}
	// We're trying to write a gcc struct that matches gc's layout.
	// Use packed attribute to force no padding in this struct in case
	// gcc has different packing requirements.
	tr := n.FuncType.Result
	if (n.Kind != "macro" && len(n.FuncType.Params) > 0) || tr != nil {
		fmt.Fprintf(fgcc, "\t%s %v *_cgo_a = v;\n", ctype, p.packedAttribute())
	}
	if tr != nil {
		// Save the stack top for use below.
		fmt.Fprintf(fgcc, "\tchar *_cgo_stktop = _cgo_topofstack();\n")
		fmt.Fprintf(fgcc, "\t__typeof__(_cgo_a->r) _cgo_r;\n")
	}
	fmt.Fprintf(fgcc, "\t_cgo_tsan_acquire();\n")
	if n.AddError {
		fmt.Fprintf(fgcc, "\terrno = 0;\n")
	}
	fmt.Fprintf(fgcc, "\t")
	if tr != nil {
		fmt.Fprintf(fgcc, "_cgo_r = ")
		if c := tr.C.String(); c[len(c)-1] == '*' {
			fmt.Fprint(fgcc, "(__typeof__(_cgo_a->r)) ")
		}
	}
	if n.Kind == "macro" {
		fmt.Fprintf(fgcc, "%s;\n", n.C)
	} else {
		fmt.Fprintf(fgcc, "%s(", n.C)
		for i := range n.FuncType.Params {
			if i > 0 {
				fmt.Fprintf(fgcc, ", ")
			}
			fmt.Fprintf(fgcc, "_cgo_a->p%d", i)
		}
		fmt.Fprintf(fgcc, ");\n")
	}
	if n.AddError {
		fmt.Fprintf(fgcc, "\t_cgo_errno = errno;\n")
	}
	fmt.Fprintf(fgcc, "\t_cgo_tsan_release();\n")
	if tr != nil {
		// The cgo call may have caused a stack copy (via a callback).
		// Adjust the return value pointer appropriately.
		fmt.Fprintf(fgcc, "\t_cgo_a = (void*)((char*)_cgo_a + (_cgo_topofstack() - _cgo_stktop));\n")
		// Save the return value.
		fmt.Fprintf(fgcc, "\t_cgo_a->r = _cgo_r;\n")
		// The return value is on the Go stack. If we are using msan,
		// and if the C value is partially or completely uninitialized,
		// the assignment will mark the Go stack as uninitialized.
		// The Go compiler does not update msan for changes to the
		// stack. It is possible that the stack will remain
		// uninitialized, and then later be used in a way that is
		// visible to msan, possibly leading to a false positive.
		// Mark the stack space as written, to avoid this problem.
		// See issue 26209.
		fmt.Fprintf(fgcc, "\t_cgo_msan_write(&_cgo_a->r, sizeof(_cgo_a->r));\n")
	}
	if n.AddError {
		fmt.Fprintf(fgcc, "\treturn _cgo_errno;\n")
	}
	fmt.Fprintf(fgcc, "}\n")
	fmt.Fprintf(fgcc, "\n")
}

// Write out a wrapper for a function when using gccgo. This is a
// simple wrapper that just calls the real function. We only need a
// wrapper to support static functions in the prologue--without a
// wrapper, we can't refer to the function, since the reference is in
// a different file.
func (p *Package) writeGccgoOutputFunc(fgcc *os.File, n *Name) {
	fmt.Fprintf(fgcc, "CGO_NO_SANITIZE_THREAD\n")
	if t := n.FuncType.Result; t != nil {
		fmt.Fprintf(fgcc, "%s\n", t.C.String())
	} else {
		fmt.Fprintf(fgcc, "void\n")
	}
	fmt.Fprintf(fgcc, "_cgo%s%s(", cPrefix, n.Mangle)
	for i, t := range n.FuncType.Params {
		if i > 0 {
			fmt.Fprintf(fgcc, ", ")
		}
		c := t.Typedef
		if c == "" {
			c = t.C.String()
		}
		fmt.Fprintf(fgcc, "%s p%d", c, i)
	}
	fmt.Fprintf(fgcc, ")\n")
	fmt.Fprintf(fgcc, "{\n")
	if t := n.FuncType.Result; t != nil {
		fmt.Fprintf(fgcc, "\t%s _cgo_r;\n", t.C.String())
	}
	fmt.Fprintf(fgcc, "\t_cgo_tsan_acquire();\n")
	fmt.Fprintf(fgcc, "\t")
	if t := n.FuncType.Result; t != nil {
		fmt.Fprintf(fgcc, "_cgo_r = ")
		// Cast to void* to avoid warnings due to omitted qualifiers.
		if c := t.C.String(); c[len(c)-1] == '*' {
			fmt.Fprintf(fgcc, "(void*)")
		}
	}
	if n.Kind == "macro" {
		fmt.Fprintf(fgcc, "%s;\n", n.C)
	} else {
		fmt.Fprintf(fgcc, "%s(", n.C)
		for i := range n.FuncType.Params {
			if i > 0 {
				fmt.Fprintf(fgcc, ", ")
			}
			fmt.Fprintf(fgcc, "p%d", i)
		}
		fmt.Fprintf(fgcc, ");\n")
	}
	fmt.Fprintf(fgcc, "\t_cgo_tsan_release();\n")
	if t := n.FuncType.Result; t != nil {
		fmt.Fprintf(fgcc, "\treturn ")
		// Cast to void* to avoid warnings due to omitted qualifiers
		// and explicit incompatible struct types.
		if c := t.C.String(); c[len(c)-1] == '*' {
			fmt.Fprintf(fgcc, "(void*)")
		}
		fmt.Fprintf(fgcc, "_cgo_r;\n")
	}
	fmt.Fprintf(fgcc, "}\n")
	fmt.Fprintf(fgcc, "\n")
}

// packedAttribute returns host compiler struct attribute that will be
// used to match gc's struct layout. For example, on 386 Windows,
// gcc wants to 8-align int64s, but gc does not.
// Use __gcc_struct__ to work around https://gcc.gnu.org/PR52991 on x86,
// and https://golang.org/issue/5603.
func (p *Package) packedAttribute() string {
	s := "__attribute__((__packed__"
	if !p.GccIsClang && (goarch == "amd64" || goarch == "386") {
		s += ", __gcc_struct__"
	}
	return s + "))"
}

// exportParamName returns the value of param as it should be
// displayed in a c header file. If param contains any non-ASCII
// characters, this function will return the character p followed by
// the value of position; otherwise, this function will return the
// value of param.
func exportParamName(param string, position int) string {
	if param == "" {
		return fmt.Sprintf("p%d", position)
	}

	pname := param

	for i := 0; i < len(param); i++ {
		if param[i] > unicode.MaxASCII {
			pname = fmt.Sprintf("p%d", position)
			break
		}
	}

	return pname
}

// Write out the various stubs we need to support functions exported
// from Go so that they are callable from C.
func (p *Package) writeExports(fgo2, fm, fgcc, fgcch io.Writer) {
	p.writeExportHeader(fgcch)

	fmt.Fprintf(fgcc, "/* Code generated by cmd/cgo; DO NOT EDIT. */\n\n")
	fmt.Fprintf(fgcc, "#include <stdlib.h>\n")
	fmt.Fprintf(fgcc, "#include \"_cgo_export.h\"\n\n")

	// We use packed structs, but they are always aligned.
	// The pragmas and address-of-packed-member are only recognized as
	// warning groups in clang 4.0+, so ignore unknown pragmas first.
	fmt.Fprintf(fgcc, "#pragma GCC diagnostic ignored \"-Wunknown-pragmas\"\n")
	fmt.Fprintf(fgcc, "#pragma GCC diagnostic ignored \"-Wpragmas\"\n")
	fmt.Fprintf(fgcc, "#pragma GCC diagnostic ignored \"-Waddress-of-packed-member\"\n")
	fmt.Fprintf(fgcc, "#pragma GCC diagnostic ignored \"-Wunknown-warning-option\"\n")
	fmt.Fprintf(fgcc, "#pragma GCC diagnostic ignored \"-Wunaligned-access\"\n")

	fmt.Fprintf(fgcc, "extern void crosscall2(void (*fn)(void *), void *, int, size_t);\n")
	fmt.Fprintf(fgcc, "extern size_t _cgo_wait_runtime_init_done(void);\n")
	fmt.Fprintf(fgcc, "extern void _cgo_release_context(size_t);\n\n")
	fmt.Fprintf(fgcc, "extern char* _cgo_topofstack(void);")
	fmt.Fprintf(fgcc, "%s\n", tsanProlog)
	fmt.Fprintf(fgcc, "%s\n", msanProlog)

	for _, exp := range p.ExpFunc {
		fn := exp.Func

		// Construct a struct that will be used to communicate
		// arguments from C to Go. The C and Go definitions
		// just have to agree. The gcc struct will be compiled
		// with __attribute__((packed)) so all padding must be
		// accounted for explicitly.
		var ctype strings.Builder
		const start = "struct {\n"
		ctype.WriteString(start)
		gotype := new(bytes.Buffer)
		fmt.Fprintf(gotype, "struct {\n")
		off := int64(0)
		npad := 0
		// the align is at least 1 (for char)
		maxAlign := int64(1)
		argField := func(typ ast.Expr, namePat string, args ...any) {
			name := fmt.Sprintf(namePat, args...)
			t := p.cgoType(typ)
			if off%t.Align != 0 {
				pad := t.Align - off%t.Align
				fmt.Fprintf(&ctype, "\t\tchar __pad%d[%d];\n", npad, pad)
				off += pad
				npad++
			}
			fmt.Fprintf(&ctype, "\t\t%s %s;\n", t.C, name)
			fmt.Fprintf(gotype, "\t\t%s ", name)
			noSourceConf.Fprint(gotype, fset, typ)
			fmt.Fprintf(gotype, "\n")
			off += t.Size
			// keep track of the maximum alignment among all fields
			// so that we can align the struct correctly
			if t.Align > maxAlign {
				maxAlign = t.Align
			}
		}
		if fn.Recv != nil {
			argField(fn.Recv.List[0].Type, "recv")
		}
		fntype := fn.Type
		forFieldList(fntype.Params,
			func(i int, aname string, atype ast.Expr) {
				argField(atype, "p%d", i)
			})
		forFieldList(fntype.Results,
			func(i int, aname string, atype ast.Expr) {
				argField(atype, "r%d", i)
			})
		if ctype.Len() == len(start) {
			ctype.WriteString("\t\tchar unused;\n") // avoid empty struct
		}
		ctype.WriteString("\t}")
		fmt.Fprintf(gotype, "\t}")

		// Get the return type of the wrapper function
		// compiled by gcc.
		gccResult := ""
		if fntype.Results == nil || len(fntype.Results.List) == 0 {
			gccResult = "void"
		} else if len(fntype.Results.List) == 1 && len(fntype.Results.List[0].Names) <= 1 {
			gccResult = p.cgoType(fntype.Results.List[0].Type).C.String()
		} else {
			fmt.Fprintf(fgcch, "\n/* Return type for %s */\n", exp.ExpName)
			fmt.Fprintf(fgcch, "struct %s_return {\n", exp.ExpName)
			forFieldList(fntype.Results,
				func(i int, aname string, atype ast.Expr) {
					fmt.Fprintf(fgcch, "\t%s r%d;", p.cgoType(atype).C, i)
					if len(aname) > 0 {
						fmt.Fprintf(fgcch, " /* %s */", aname)
					}
					fmt.Fprint(fgcch, "\n")
				})
			fmt.Fprintf(fgcch, "};\n")
			gccResult = "struct " + exp.ExpName + "_return"
		}

		// Build the wrapper function compiled by gcc.
		var s strings.Builder
		fmt.Fprintf(&s, "%s %s(", gccResult, exp.ExpName)
		if fn.Recv != nil {
			s.WriteString(p.cgoType(fn.Recv.List[0].Type).C.String())
			s.WriteString(" recv")
		}

		if len(fntype.Params.List) > 0 {
			forFieldList(fntype.Params,
				func(i int, aname string, atype ast.Expr) {
					if i > 0 || fn.Recv != nil {
						s.WriteString(", ")
					}
					fmt.Fprintf(&s, "%s %s", p.cgoType(atype).C, exportParamName(aname, i))
				})
		} else {
			s.WriteString("void")
		}
		s.WriteByte(')')

		if len(exp.Doc) > 0 {
			fmt.Fprintf(fgcch, "\n%s", exp.Doc)
			if !strings.HasSuffix(exp.Doc, "\n") {
				fmt.Fprint(fgcch, "\n")
			}
		}
		fmt.Fprintf(fgcch, "extern %s;\n", s.String())

		fmt.Fprintf(fgcc, "extern void _cgoexp%s_%s(void *);\n", cPrefix, exp.ExpName)
		fmt.Fprintf(fgcc, "\nCGO_NO_SANITIZE_THREAD")
		fmt.Fprintf(fgcc, "\n%s\n", s.String())
		fmt.Fprintf(fgcc, "{\n")
		fmt.Fprintf(fgcc, "\tsize_t _cgo_ctxt = _cgo_wait_runtime_init_done();\n")
		// The results part of the argument structure must be
		// initialized to 0 so the write barriers generated by
		// the assignments to these fields in Go are safe.
		//
		// We use a local static variable to get the zeroed
		// value of the argument type. This avoids including
		// string.h for memset, and is also robust to C++
		// types with constructors. Both GCC and LLVM optimize
		// this into just zeroing _cgo_a.
		//
		// The struct should be aligned to the maximum alignment
		// of any of its fields. This to avoid alignment
		// issues.
		fmt.Fprintf(fgcc, "\ttypedef %s %v __attribute__((aligned(%d))) _cgo_argtype;\n", ctype.String(), p.packedAttribute(), maxAlign)
		fmt.Fprintf(fgcc, "\tstatic _cgo_argtype _cgo_zero;\n")
		fmt.Fprintf(fgcc, "\t_cgo_argtype _cgo_a = _cgo_zero;\n")
		if gccResult != "void" && (len(fntype.Results.List) > 1 || len(fntype.Results.List[0].Names) > 1) {
			fmt.Fprintf(fgcc, "\t%s r;\n", gccResult)
		}
		if fn.Recv != nil {
			fmt.Fprintf(fgcc, "\t_cgo_a.recv = recv;\n")
		}
		forFieldList(fntype.Params,
			func(i int, aname string, atype ast.Expr) {
				fmt.Fprintf(fgcc, "\t_cgo_a.p%d = %s;\n", i, exportParamName(aname, i))
			})
		fmt.Fprintf(fgcc, "\t_cgo_tsan_release();\n")
		fmt.Fprintf(fgcc, "\tcrosscall2(_cgoexp%s_%s, &_cgo_a, %d, _cgo_ctxt);\n", cPrefix, exp.ExpName, off)
		fmt.Fprintf(fgcc, "\t_cgo_tsan_acquire();\n")
		fmt.Fprintf(fgcc, "\t_cgo_release_context(_cgo_ctxt);\n")
		if gccResult != "void" {
			if len(fntype.Results.List) == 1 && len(fntype.Results.List[0].Names) <= 1 {
				fmt.Fprintf(fgcc, "\treturn _cgo_a.r0;\n")
			} else {
				forFieldList(fntype.Results,
					func(i int, aname string, atype ast.Expr) {
						fmt.Fprintf(fgcc, "\tr.r%d = _cgo_a.r%d;\n", i, i)
					})
				fmt.Fprintf(fgcc, "\treturn r;\n")
			}
		}
		fmt.Fprintf(fgcc, "}\n")

		// In internal linking mode, the Go linker sees both
		// the C wrapper written above and the Go wrapper it
		// references. Hence, export the C wrapper (e.g., for
		// if we're building a shared object). The Go linker
		// will resolve the C wrapper's reference to the Go
		// wrapper without a separate export.
		fmt.Fprintf(fgo2, "//go:cgo_export_dynamic %s\n", exp.ExpName)
		// cgo_export_static refers to a symbol by its linker
		// name, so set the linker name of the Go wrapper.
		fmt.Fprintf(fgo2, "//go:linkname _cgoexp%s_%s _cgoexp%s_%s\n", cPrefix, exp.ExpName, cPrefix, exp.ExpName)
		// In external linking mode, the Go linker sees the Go
		// wrapper, but not the C wrapper. For this case,
		// export the Go wrapper so the host linker can
		// resolve the reference from the C wrapper to the Go
		// wrapper.
		fmt.Fprintf(fgo2, "//go:cgo_export_static _cgoexp%s_%s\n", cPrefix, exp.ExpName)

		// Build the wrapper function compiled by cmd/compile.
		// This unpacks the argument struct above and calls the Go function.
		fmt.Fprintf(fgo2, "func _cgoexp%s_%s(a *%s) {\n", cPrefix, exp.ExpName, gotype)

		fmt.Fprintf(fm, "void _cgoexp%s_%s(void* p __attribute__((unused))){}\n", cPrefix, exp.ExpName)

		fmt.Fprintf(fgo2, "\t")

		if gccResult != "void" {
			// Write results back to frame.
			forFieldList(fntype.Results,
				func(i int, aname string, atype ast.Expr) {
					if i > 0 {
						fmt.Fprintf(fgo2, ", ")
					}
					fmt.Fprintf(fgo2, "a.r%d", i)
				})
			fmt.Fprintf(fgo2, " = ")
		}
		if fn.Recv != nil {
			fmt.Fprintf(fgo2, "a.recv.")
		}
		fmt.Fprintf(fgo2, "%s(", exp.Func.Name)
		forFieldList(fntype.Params,
			func(i int, aname string, atype ast.Expr) {
				if i > 0 {
					fmt.Fprint(fgo2, ", ")
				}
				fmt.Fprintf(fgo2, "a.p%d", i)
			})
		fmt.Fprint(fgo2, ")\n")
		if gccResult != "void" {
			// Verify that any results don't contain any
			// Go pointers.
			forFieldList(fntype.Results,
				func(i int, aname string, atype ast.Expr) {
					if !p.hasPointer(nil, atype, false) {
						return
					}

					// Use the export'ed file/line in error messages.
					pos := fset.Position(exp.Func.Pos())
					fmt.Fprintf(fgo2, "//line %s:%d\n", pos.Filename, pos.Line)
					fmt.Fprintf(fgo2, "\t_cgoCheckResult(a.r%d)\n", i)
				})
		}
		fmt.Fprint(fgo2, "}\n")
	}

	fmt.Fprintf(fgcch, "%s", gccExportHeaderEpilog)
}

// Write out the C header allowing C code to call exported gccgo functions.
func (p *Package) writeGccgoExports(fgo2, fm, fgcc, fgcch io.Writer) {
	gccgoSymbolPrefix := p.gccgoSymbolPrefix()

	p.writeExportHeader(fgcch)

	fmt.Fprintf(fgcc, "/* Code generated by cmd/cgo; DO NOT EDIT. */\n\n")
	fmt.Fprintf(fgcc, "#include \"_cgo_export.h\"\n")

	fmt.Fprintf(fgcc, "%s\n", gccgoExportFileProlog)
	fmt.Fprintf(fgcc, "%s\n", tsanProlog)
	fmt.Fprintf(fgcc, "%s\n", msanProlog)

	for _, exp := range p.ExpFunc {
		fn := exp.Func
		fntype := fn.Type

		cdeclBuf := new(strings.Builder)
		resultCount := 0
		forFieldList(fntype.Results,
			func(i int, aname string, atype ast.Expr) { resultCount++ })
		switch resultCount {
		case 0:
			fmt.Fprintf(cdeclBuf, "void")
		case 1:
			forFieldList(fntype.Results,
				func(i int, aname string, atype ast.Expr) {
					t := p.cgoType(atype)
					fmt.Fprintf(cdeclBuf, "%s", t.C)
				})
		default:
			// Declare a result struct.
			fmt.Fprintf(fgcch, "\n/* Return type for %s */\n", exp.ExpName)
			fmt.Fprintf(fgcch, "struct %s_return {\n", exp.ExpName)
			forFieldList(fntype.Results,
				func(i int, aname string, atype ast.Expr) {
					t := p.cgoType(atype)
					fmt.Fprintf(fgcch, "\t%s r%d;", t.C, i)
					if len(aname) > 0 {
						fmt.Fprintf(fgcch, " /* %s */", aname)
					}
					fmt.Fprint(fgcch, "\n")
				})
			fmt.Fprintf(fgcch, "};\n")
			fmt.Fprintf(cdeclBuf, "struct %s_return", exp.ExpName)
		}

		cRet := cdeclBuf.String()

		cdeclBuf = new(strings.Builder)
		fmt.Fprintf(cdeclBuf, "(")
		if fn.Recv != nil {
			fmt.Fprintf(cdeclBuf, "%s recv", p.cgoType(fn.Recv.List[0].Type).C.String())
		}
		// Function parameters.
		forFieldList(fntype.Params,
			func(i int, aname string, atype ast.Expr) {
				if i > 0 || fn.Recv != nil {
					fmt.Fprintf(cdeclBuf, ", ")
				}
				t := p.cgoType(atype)
				fmt.Fprintf(cdeclBuf, "%s p%d", t.C, i)
			})
		fmt.Fprintf(cdeclBuf, ")")
		cParams := cdeclBuf.String()

		if len(exp.Doc) > 0 {
			fmt.Fprintf(fgcch, "\n%s", exp.Doc)
		}

		fmt.Fprintf(fgcch, "extern %s %s%s;\n", cRet, exp.ExpName, cParams)

		// We need to use a name that will be exported by the
		// Go code; otherwise gccgo will make it static and we
		// will not be able to link against it from the C
		// code.
		goName := "Cgoexp_" + exp.ExpName
		fmt.Fprintf(fgcc, `extern %s %s %s __asm__("%s.%s");`, cRet, goName, cParams, gccgoSymbolPrefix, gccgoToSymbol(goName))
		fmt.Fprint(fgcc, "\n")

		fmt.Fprint(fgcc, "\nCGO_NO_SANITIZE_THREAD\n")
		fmt.Fprintf(fgcc, "%s %s %s {\n", cRet, exp.ExpName, cParams)
		if resultCount > 0 {
			fmt.Fprintf(fgcc, "\t%s r;\n", cRet)
		}
		fmt.Fprintf(fgcc, "\tif(_cgo_wait_runtime_init_done)\n")
		fmt.Fprintf(fgcc, "\t\t_cgo_wait_runtime_init_done();\n")
		fmt.Fprintf(fgcc, "\t_cgo_tsan_release();\n")
		fmt.Fprint(fgcc, "\t")
		if resultCount > 0 {
			fmt.Fprint(fgcc, "r = ")
		}
		fmt.Fprintf(fgcc, "%s(", goName)
		if fn.Recv != nil {
			fmt.Fprint(fgcc, "recv")
		}
		forFieldList(fntype.Params,
			func(i int, aname string, atype ast.Expr) {
				if i > 0 || fn.Recv != nil {
					fmt.Fprintf(fgcc, ", ")
				}
				fmt.Fprintf(fgcc, "p%d", i)
			})
		fmt.Fprint(fgcc, ");\n")
		fmt.Fprintf(fgcc, "\t_cgo_tsan_acquire();\n")
		if resultCount > 0 {
			fmt.Fprint(fgcc, "\treturn r;\n")
		}
		fmt.Fprint(fgcc, "}\n")

		// Dummy declaration for _cgo_main.c
		fmt.Fprintf(fm, `char %s[1] __asm__("%s.%s");`, goName, gccgoSymbolPrefix, gccgoToSymbol(goName))
		fmt.Fprint(fm, "\n")

		// For gccgo we use a wrapper function in Go, in order
		// to call CgocallBack and CgocallBackDone.

		// This code uses printer.Fprint, not conf.Fprint,
		// because we don't want //line comments in the middle
		// of the function types.
		fmt.Fprint(fgo2, "\n")
		fmt.Fprintf(fgo2, "func %s(", goName)
		if fn.Recv != nil {
			fmt.Fprint(fgo2, "recv ")
			printer.Fprint(fgo2, fset, fn.Recv.List[0].Type)
		}
		forFieldList(fntype.Params,
			func(i int, aname string, atype ast.Expr) {
				if i > 0 || fn.Recv != nil {
					fmt.Fprintf(fgo2, ", ")
				}
				fmt.Fprintf(fgo2, "p%d ", i)
				printer.Fprint(fgo2, fset, atype)
			})
		fmt.Fprintf(fgo2, ")")
		if resultCount > 0 {
			fmt.Fprintf(fgo2, " (")
			forFieldList(fntype.Results,
				func(i int, aname string, atype ast.Expr) {
					if i > 0 {
						fmt.Fprint(fgo2, ", ")
					}
					printer.Fprint(fgo2, fset, atype)
				})
			fmt.Fprint(fgo2, ")")
		}
		fmt.Fprint(fgo2, " {\n")
		fmt.Fprint(fgo2, "\tsyscall.CgocallBack()\n")
		fmt.Fprint(fgo2, "\tdefer syscall.CgocallBackDone()\n")
		fmt.Fprint(fgo2, "\t")
		if resultCount > 0 {
			fmt.Fprint(fgo2, "return ")
		}
		if fn.Recv != nil {
			fmt.Fprint(fgo2, "recv.")
		}
		fmt.Fprintf(fgo2, "%s(", exp.Func.Name)
		forFieldList(fntype.Params,
			func(i int, aname string, atype ast.Expr) {
				if i > 0 {
					fmt.Fprint(fgo2, ", ")
				}
				fmt.Fprintf(fgo2, "p%d", i)
			})
		fmt.Fprint(fgo2, ")\n")
		fmt.Fprint(fgo2, "}\n")
	}

	fmt.Fprintf(fgcch, "%s", gccExportHeaderEpilog)
}

// writeExportHeader writes out the start of the _cgo_export.h file.
func (p *Package) writeExportHeader(fgcch io.Writer) {
	fmt.Fprintf(fgcch, "/* Code generated by cmd/cgo; DO NOT EDIT. */\n\n")
	pkg := *importPath
	if pkg == "" {
		pkg = p.PackagePath
	}
	fmt.Fprintf(fgcch, "/* package %s */\n\n", pkg)
	fmt.Fprintf(fgcch, "%s\n", builtinExportProlog)

	// Remove absolute paths from #line comments in the preamble.
	// They aren't useful for people using the header file,
	// and they mean that the header files change based on the
	// exact location of GOPATH.
	re := regexp.MustCompile(`(?m)^(#line\s+\d+\s+")[^"]*[/\\]([^"]*")`)
	preamble := re.ReplaceAllString(p.Preamble, "$1$2")

	fmt.Fprintf(fgcch, "/* Start of preamble from import \"C\" comments.  */\n\n")
	fmt.Fprintf(fgcch, "%s\n", preamble)
	fmt.Fprintf(fgcch, "\n/* End of preamble from import \"C\" comments.  */\n\n")

	fmt.Fprintf(fgcch, "%s\n", p.gccExportHeaderProlog())
}

// gccgoToSymbol converts a name to a mangled symbol for gccgo.
func gccgoToSymbol(ppath string) string {
	if gccgoMangler == nil {
		var err error
		cmd := os.Getenv("GCCGO")
		if cmd == "" {
			cmd, err = exec.LookPath("gccgo")
			if err != nil {
				fatalf("unable to locate gccgo: %v", err)
			}
		}
		gccgoMangler, err = pkgpath.ToSymbolFunc(cmd, outputDir())
		if err != nil {
			fatalf("%v", err)
		}
	}
	return gccgoMangler(ppath)
}

// Return the package prefix when using gccgo.
func (p *Package) gccgoSymbolPrefix() string {
	if !*gccgo {
		return ""
	}

	if *gccgopkgpath != "" {
		return gccgoToSymbol(*gccgopkgpath)
	}
	if *gccgoprefix == "" && p.PackageName == "main" {
		return "main"
	}
	prefix := gccgoToSymbol(*gccgoprefix)
	if prefix == "" {
		prefix = "go"
	}
	return prefix + "." + p.PackageName
}

// Call a function for each entry in an ast.FieldList, passing the
// index into the list, the name if any, and the type.
func forFieldList(fl *ast.FieldList, fn func(int, string, ast.Expr)) {
	if fl == nil {
		return
	}
	i := 0
	for _, r := range fl.List {
		if r.Names == nil {
			fn(i, "", r.Type)
			i++
		} else {
			for _, n := range r.Names {
				fn(i, n.Name, r.Type)
				i++
			}
		}
	}
}

func c(repr string, args ...any) *TypeRepr {
	return &TypeRepr{repr, args}
}

// Map predeclared Go types to Type.
var goTypes = map[string]*Type{
	"bool":       {Size: 1, Align: 1, C: c("GoUint8")},
	"byte":       {Size: 1, Align: 1, C: c("GoUint8")},
	"int":        {Size: 0, Align: 0, C: c("GoInt")},
	"uint":       {Size: 0, Align: 0, C: c("GoUint")},
	"rune":       {Size: 4, Align: 4, C: c("GoInt32")},
	"int8":       {Size: 1, Align: 1, C: c("GoInt8")},
	"uint8":      {Size: 1, Align: 1, C: c("GoUint8")},
	"int16":      {Size: 2, Align: 2, C: c("GoInt16")},
	"uint16":     {Size: 2, Align: 2, C: c("GoUint16")},
	"int32":      {Size: 4, Align: 4, C: c("GoInt32")},
	"uint32":     {Size: 4, Align: 4, C: c("GoUint32")},
	"int64":      {Size: 8, Align: 8, C: c("GoInt64")},
	"uint64":     {Size: 8, Align: 8, C: c("GoUint64")},
	"float32":    {Size: 4, Align: 4, C: c("GoFloat32")},
	"float64":    {Size: 8, Align: 8, C: c("GoFloat64")},
	"complex64":  {Size: 8, Align: 4, C: c("GoComplex64")},
	"complex128": {Size: 16, Align: 8, C: c("GoComplex128")},
}

// Map an ast type to a Type.
func (p *Package) cgoType(e ast.Expr) *Type {
	return p.doCgoType(e, make(map[ast.Expr]bool))
}

// Map an ast type to a Type, avoiding cycles.
func (p *Package) doCgoType(e ast.Expr, m map[ast.Expr]bool) *Type {
	if m[e] {
		fatalf("%s: invalid recursive type", fset.Position(e.Pos()))
	}
	m[e] = true
	switch t := e.(type) {
	case *ast.StarExpr:
		x := p.doCgoType(t.X, m)
		return &Type{Size: p.PtrSize, Align: p.PtrSize, C: c("%s*", x.C)}
	case *ast.ArrayType:
		if t.Len == nil {
			// Slice: pointer, len, cap.
			return &Type{Size: p.PtrSize * 3, Align: p.PtrSize, C: c("GoSlice")}
		}
		// Non-slice array types are not supported.
	case *ast.StructType:
		// Not supported.
	case *ast.FuncType:
		return &Type{Size: p.PtrSize, Align: p.PtrSize, C: c("void*")}
	case *ast.InterfaceType:
		return &Type{Size: 2 * p.PtrSize, Align: p.PtrSize, C: c("GoInterface")}
	case *ast.MapType:
		return &Type{Size: p.PtrSize, Align: p.PtrSize, C: c("GoMap")}
	case *ast.ChanType:
		return &Type{Size: p.PtrSize, Align: p.PtrSize, C: c("GoChan")}
	case *ast.Ident:
		goTypesFixup := func(r *Type) *Type {
			if r.Size == 0 { // int or uint
				rr := new(Type)
				*rr = *r
				rr.Size = p.IntSize
				rr.Align = p.IntSize
				r = rr
			}
			if r.Align > p.PtrSize {
				r.Align = p.PtrSize
			}
			return r
		}
		// Look up the type in the top level declarations.
		// TODO: Handle types defined within a function.
		for _, d := range p.Decl {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if ts.Name.Name == t.Name {
					// Give a better error than the one
					// above if we detect a recursive type.
					if m[ts.Type] {
						fatalf("%s: invalid recursive type: %s refers to itself", fset.Position(e.Pos()), t.Name)
					}
					return p.doCgoType(ts.Type, m)
				}
			}
		}
		if def := typedef[t.Name]; def != nil {
			if defgo, ok := def.Go.(*ast.Ident); ok {
				switch defgo.Name {
				case "complex64", "complex128":
					// MSVC does not support the _Complex keyword
					// nor the complex macro.
					// Use GoComplex64 and GoComplex128 instead,
					// which are typedef-ed to a compatible type.
					// See go.dev/issues/36233.
					return goTypesFixup(goTypes[defgo.Name])
				}
			}
			return def
		}
		if t.Name == "uintptr" {
			return &Type{Size: p.PtrSize, Align: p.PtrSize, C: c("GoUintptr")}
		}
		if t.Name == "string" {
			// The string data is 1 pointer + 1 (pointer-sized) int.
			return &Type{Size: 2 * p.PtrSize, Align: p.PtrSize, C: c("GoString")}
		}
		if t.Name == "error" {
			return &Type{Size: 2 * p.PtrSize, Align: p.PtrSize, C: c("GoInterface")}
		}
		if t.Name == "any" {
			return &Type{Size: 2 * p.PtrSize, Align: p.PtrSize, C: c("GoInterface")}
		}
		if r, ok := goTypes[t.Name]; ok {
			return goTypesFixup(r)
		}
		error_(e.Pos(), "unrecognized Go type %s", t.Name)
		return &Type{Size: 4, Align: 4, C: c("int")}
	case *ast.SelectorExpr:
		id, ok := t.X.(*ast.Ident)
		if ok && id.Name == "unsafe" && t.Sel.Name == "Pointer" {
			return &Type{Size: p.PtrSize, Align: p.PtrSize, C: c("void*")}
		}
	}
	error_(e.Pos(), "Go type not supported in export: %s", gofmt(e))
	return &Type{Size: 4, Align: 4, C: c("int")}
}

const gccProlog = `
#line 1 "cgo-gcc-prolog"
/*
  If x and y are not equal, the type will be invalid
  (have a negative array count) and an inscrutable error will come
  out of the compiler and hopefully mention "name".
*/
#define __cgo_compile_assert_eq(x, y, name) typedef char name[(x-y)*(x-y)*-2UL+1UL];

/* Check at compile time that the sizes we use match our expectations. */
#define __cgo_size_assert(t, n) __cgo_compile_assert_eq(sizeof(t), (size_t)n, _cgo_sizeof_##t##_is_not_##n)

__cgo_size_assert(char, 1)
__cgo_size_assert(short, 2)
__cgo_size_assert(int, 4)
typedef long long __cgo_long_long;
__cgo_size_assert(__cgo_long_long, 8)
__cgo_size_assert(float, 4)
__cgo_size_assert(double, 8)

extern char* _cgo_topofstack(void);

/*
  We use packed structs, but they are always aligned.
  The pragmas and address-of-packed-member are only recognized as warning
  groups in clang 4.0+, so ignore unknown pragmas first.
*/
#pragma GCC diagnostic ignored "-Wunknown-pragmas"
#pragma GCC diagnostic ignored "-Wpragmas"
#pragma GCC diagnostic ignored "-Waddress-of-packed-member"
#pragma GCC diagnostic ignored "-Wunknown-warning-option"
#pragma GCC diagnostic ignored "-Wunaligned-access"

#include <errno.h>
#include <string.h>
`

// Prologue defining TSAN functions in C.
const noTsanProlog = `
#define CGO_NO_SANITIZE_THREAD
#define _cgo_tsan_acquire()
#define _cgo_tsan_release()
`

// This must match the TSAN code in runtime/cgo/libcgo.h.
// This is used when the code is built with the C/C++ Thread SANitizer,
// which is not the same as the Go race detector.
// __tsan_acquire tells TSAN that we are acquiring a lock on a variable,
// in this case _cgo_sync. __tsan_release releases the lock.
// (There is no actual lock, we are just telling TSAN that there is.)
//
// When we call from Go to C we call _cgo_tsan_acquire.
// When the C function returns we call _cgo_tsan_release.
// Similarly, when C calls back into Go we call _cgo_tsan_release
// and then call _cgo_tsan_acquire when we return to C.
// These calls tell TSAN that there is a serialization point at the C call.
//
// This is necessary because TSAN, which is a C/C++ tool, can not see
// the synchronization in the Go code. Without these calls, when
// multiple goroutines call into C code, TSAN does not understand
// that the calls are properly synchronized on the Go side.
//
// To be clear, if the calls are not properly synchronized on the Go side,
// we will be hiding races. But when using TSAN on mixed Go C/C++ code
// it is more important to avoid false positives, which reduce confidence
// in the tool, than to avoid false negatives.
const yesTsanProlog = `
#line 1 "cgo-tsan-prolog"
#define CGO_NO_SANITIZE_THREAD __attribute__ ((no_sanitize_thread))

long long _cgo_sync __attribute__ ((common));

extern void __tsan_acquire(void*);
extern void __tsan_release(void*);

__attribute__ ((unused))
static void _cgo_tsan_acquire() {
	__tsan_acquire(&_cgo_sync);
}

__attribute__ ((unused))
static void _cgo_tsan_release() {
	__tsan_release(&_cgo_sync);
}
`

// Set to yesTsanProlog if we see -fsanitize=thread in the flags for gcc.
var tsanProlog = noTsanProlog

// noMsanProlog is a prologue defining an MSAN function in C.
// This is used when not compiling with -fsanitize=memory.
const noMsanProlog = `
#define _cgo_msan_write(addr, sz)
`

// yesMsanProlog is a prologue defining an MSAN function in C.
// This is used when compiling with -fsanitize=memory.
// See the comment above where _cgo_msan_write is called.
const yesMsanProlog = `
extern void __msan_unpoison(const volatile void *, size_t);

#define _cgo_msan_write(addr, sz) __msan_unpoison((addr), (sz))
`

// msanProlog is set to yesMsanProlog if we see -fsanitize=memory in the flags
// for the C compiler.
var msanProlog = noMsanProlog

const builtinProlog = `
#line 1 "cgo-builtin-prolog"
#include <stddef.h>

/* Define intgo when compiling with GCC.  */
typedef ptrdiff_t intgo;

#define GO_CGO_GOSTRING_TYPEDEF
typedef struct { const char *p; intgo n; } _GoString_;
typedef struct { char *p; intgo n; intgo c; } _GoBytes_;
_GoString_ GoString(char *p);
_GoString_ GoStringN(char *p, int l);
_GoBytes_ GoBytes(void *p, int n);
char *CString(_GoString_);
void *CBytes(_GoBytes_);
void *_CMalloc(size_t);

__attribute__ ((unused))
static size_t _GoStringLen(_GoString_ s) { return (size_t)s.n; }

__attribute__ ((unused))
static const char *_GoStringPtr(_GoString_ s) { return s.p; }
`

const goProlog = `
//go:linkname _cgo_runtime_cgocall runtime.cgocall
func _cgo_runtime_cgocall(unsafe.Pointer, uintptr) int32

//go:linkname _cgoCheckPointer runtime.cgoCheckPointer
//go:noescape
func _cgoCheckPointer(interface{}, interface{})

//go:linkname _cgoCheckResult runtime.cgoCheckResult
//go:noescape
func _cgoCheckResult(interface{})
`

const gccgoGoProlog = `
func _cgoCheckPointer(interface{}, interface{})

func _cgoCheckResult(interface{})
`

const goStringDef = `
//go:linkname _cgo_runtime_gostring runtime.gostring
func _cgo_runtime_gostring(*_Ctype_char) string

// GoString converts the C string p into a Go string.
func _Cfunc_GoString(p *_Ctype_char) string {
	return _cgo_runtime_gostring(p)
}
`

const goStringNDef = `
//go:linkname _cgo_runtime_gostringn runtime.gostringn
func _cgo_runtime_gostringn(*_Ctype_char, int) string

// GoStringN converts the C data p with explicit length l to a Go string.
func _Cfunc_GoStringN(p *_Ctype_char, l _Ctype_int) string {
	return _cgo_runtime_gostringn(p, int(l))
}
`

const goBytesDef = `
//go:linkname _cgo_runtime_gobytes runtime.gobytes
func _cgo_runtime_gobytes(unsafe.Pointer, int) []byte

// GoBytes converts the C data p with explicit length l to a Go []byte.
func _Cfunc_GoBytes(p unsafe.Pointer, l _Ctype_int) []byte {
	return _cgo_runtime_gobytes(p, int(l))
}
`

const cStringDef = `
// CString converts the Go string s to a C string.
//
// The C string is allocated in the C heap using malloc.
// It is the caller's responsibility to arrange for it to be
// freed, such as by calling C.free (be sure to include stdlib.h
// if C.free is needed).
func _Cfunc_CString(s string) *_Ctype_char {
	if len(s)+1 <= 0 {
		panic("string too large")
	}
	p := _cgo_cmalloc(uint64(len(s)+1))
	sliceHeader := struct {
		p   unsafe.Pointer
		len int
		cap int
	}{p, len(s)+1, len(s)+1}
	b := *(*[]byte)(unsafe.Pointer(&sliceHeader))
	copy(b, s)
	b[len(s)] = 0
	return (*_Ctype_char)(p)
}
`

const cBytesDef = `
// CBytes converts the Go []byte slice b to a C array.
//
// The C array is allocated in the C heap using malloc.
// It is the caller's responsibility to arrange for it to be
// freed, such as by calling C.free (be sure to include stdlib.h
// if C.free is needed).
func _Cfunc_CBytes(b []byte) unsafe.Pointer {
	p := _cgo_cmalloc(uint64(len(b)))
	sliceHeader := struct {
		p   unsafe.Pointer
		len int
		cap int
	}{p, len(b), len(b)}
	s := *(*[]byte)(unsafe.Pointer(&sliceHeader))
	copy(s, b)
	return p
}
`

const cMallocDef = `
func _Cfunc__CMalloc(n _Ctype_size_t) unsafe.Pointer {
	return _cgo_cmalloc(uint64(n))
}
`

var builtinDefs = map[string]string{
	"GoString":  goStringDef,
	"GoStringN": goStringNDef,
	"GoBytes":   goBytesDef,
	"CString":   cStringDef,
	"CBytes":    cBytesDef,
	"_CMalloc":  cMallocDef,
}

// Definitions for C.malloc in Go and in C. We define it ourselves
// since we call it from functions we define, such as C.CString.
// Also, we have historically ensured that C.malloc does not return
// nil even for an allocation of 0.

const cMallocDefGo = `
//go:cgo_import_static _cgoPREFIX_Cfunc__Cmalloc
//go:linkname __cgofn__cgoPREFIX_Cfunc__Cmalloc _cgoPREFIX_Cfunc__Cmalloc
var __cgofn__cgoPREFIX_Cfunc__Cmalloc byte
var _cgoPREFIX_Cfunc__Cmalloc = unsafe.Pointer(&__cgofn__cgoPREFIX_Cfunc__Cmalloc)

//go:linkname runtime_throw runtime.throw
func runtime_throw(string)

//go:cgo_unsafe_args
func _cgo_cmalloc(p0 uint64) (r1 unsafe.Pointer) {
	_cgo_runtime_cgocall(_cgoPREFIX_Cfunc__Cmalloc, uintptr(unsafe.Pointer(&p0)))
	if r1 == nil {
		runtime_throw("runtime: C malloc failed")
	}
	return
}
`

// cMallocDefC defines the C version of C.malloc for the gc compiler.
// It is defined here because C.CString and friends need a definition.
// We define it by hand, rather than simply inventing a reference to
// C.malloc, because <stdlib.h> may not have been included.
// This is approximately what writeOutputFunc would generate, but
// skips the cgo_topofstack code (which is only needed if the C code
// calls back into Go). This also avoids returning nil for an
// allocation of 0 bytes.
const cMallocDefC = `
CGO_NO_SANITIZE_THREAD
void _cgoPREFIX_Cfunc__Cmalloc(void *v) {
	struct {
		unsigned long long p0;
		void *r1;
	} PACKED *a = v;
	void *ret;
	_cgo_tsan_acquire();
	ret = malloc(a->p0);
	if (ret == NULL && a->p0 == 0) {
		ret = malloc(1);
	}
	a->r1 = ret;
	_cgo_tsan_release();
}
`

func (p *Package) cPrologGccgo() string {
	r := strings.NewReplacer(
		"PREFIX", cPrefix,
		"GCCGOSYMBOLPREF", p.gccgoSymbolPrefix(),
		"_cgoCheckPointer", gccgoToSymbol("_cgoCheckPointer"),
		"_cgoCheckResult", gccgoToSymbol("_cgoCheckResult"))
	return r.Replace(cPrologGccgo)
}

const cPrologGccgo = `
#line 1 "cgo-c-prolog-gccgo"
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef unsigned char byte;
typedef intptr_t intgo;

struct __go_string {
	const unsigned char *__data;
	intgo __length;
};

typedef struct __go_open_array {
	void* __values;
	intgo __count;
	intgo __capacity;
} Slice;

struct __go_string __go_byte_array_to_string(const void* p, intgo len);
struct __go_open_array __go_string_to_byte_array (struct __go_string str);

extern void runtime_throw(const char *);

const char *_cgoPREFIX_Cfunc_CString(struct __go_string s) {
	char *p = malloc(s.__length+1);
	if(p == NULL)
		runtime_throw("runtime: C malloc failed");
	memmove(p, s.__data, s.__length);
	p[s.__length] = 0;
	return p;
}

void *_cgoPREFIX_Cfunc_CBytes(struct __go_open_array b) {
	char *p = malloc(b.__count);
	if(p == NULL)
		runtime_throw("runtime: C malloc failed");
	memmove(p, b.__values, b.__count);
	return p;
}

struct __go_string _cgoPREFIX_Cfunc_GoString(char *p) {
	intgo len = (p != NULL) ? strlen(p) : 0;
	return __go_byte_array_to_string(p, len);
}

struct __go_string _cgoPREFIX_Cfunc_GoStringN(char *p, int32_t n) {
	return __go_byte_array_to_string(p, n);
}

Slice _cgoPREFIX_Cfunc_GoBytes(char *p, int32_t n) {
	struct __go_string s = { (const unsigned char *)p, n };
	return __go_string_to_byte_array(s);
}

void *_cgoPREFIX_Cfunc__CMalloc(size_t n) {
	void *p = malloc(n);
	if(p == NULL && n == 0)
		p = malloc(1);
	if(p == NULL)
		runtime_throw("runtime: C malloc failed");
	return p;
}

struct __go_type_descriptor;
typedef struct __go_empty_interface {
	const struct __go_type_descriptor *__type_descriptor;
	void *__object;
} Eface;

extern void runtimeCgoCheckPointer(Eface, Eface)
	__asm__("runtime.cgoCheckPointer")
	__attribute__((weak));

extern void localCgoCheckPointer(Eface, Eface)
	__asm__("GCCGOSYMBOLPREF._cgoCheckPointer");

void localCgoCheckPointer(Eface ptr, Eface arg) {
	if(runtimeCgoCheckPointer) {
		runtimeCgoCheckPointer(ptr, arg);
	}
}

extern void runtimeCgoCheckResult(Eface)
	__asm__("runtime.cgoCheckResult")
	__attribute__((weak));

extern void localCgoCheckResult(Eface)
	__asm__("GCCGOSYMBOLPREF._cgoCheckResult");

void localCgoCheckResult(Eface val) {
	if(runtimeCgoCheckResult) {
		runtimeCgoCheckResult(val);
	}
}
`

// builtinExportProlog is a shorter version of builtinProlog,
// to be put into the _cgo_export.h file.
// For historical reasons we can't use builtinProlog in _cgo_export.h,
// because _cgo_export.h defines GoString as a struct while builtinProlog
// defines it as a function. We don't change this to avoid unnecessarily
// breaking existing code.
// The test of GO_CGO_GOSTRING_TYPEDEF avoids a duplicate definition
// error if a Go file with a cgo comment #include's the export header
// generated by a different package.
const builtinExportProlog = `
#line 1 "cgo-builtin-export-prolog"

#include <stddef.h>

#ifndef GO_CGO_EXPORT_PROLOGUE_H
#define GO_CGO_EXPORT_PROLOGUE_H

#ifndef GO_CGO_GOSTRING_TYPEDEF
typedef struct { const char *p; ptrdiff_t n; } _GoString_;
extern size_t _GoStringLen(_GoString_ s);
extern const char *_GoStringPtr(_GoString_ s);
#endif

#endif
`

func (p *Package) gccExportHeaderProlog() string {
	return strings.ReplaceAll(gccExportHeaderProlog, "GOINTBITS", fmt.Sprint(8*p.IntSize))
}

// gccExportHeaderProlog is written to the exported header, after the
// import "C" comment preamble but before the generated declarations
// of exported functions. This permits the generated declarations to
// use the type names that appear in goTypes, above.
//
// The test of GO_CGO_GOSTRING_TYPEDEF avoids a duplicate definition
// error if a Go file with a cgo comment #include's the export header
// generated by a different package. Unfortunately GoString means two
// different things: in this prolog it means a C name for the Go type,
// while in the prolog written into the start of the C code generated
// from a cgo-using Go file it means the C.GoString function. There is
// no way to resolve this conflict, but it also doesn't make much
// difference, as Go code never wants to refer to the latter meaning.
const gccExportHeaderProlog = `
/* Start of boilerplate cgo prologue.  */
#line 1 "cgo-gcc-export-header-prolog"

#ifndef GO_CGO_PROLOGUE_H
#define GO_CGO_PROLOGUE_H

typedef signed char GoInt8;
typedef unsigned char GoUint8;
typedef short GoInt16;
typedef unsigned short GoUint16;
typedef int GoInt32;
typedef unsigned int GoUint32;
typedef long long GoInt64;
typedef unsigned long long GoUint64;
typedef GoIntGOINTBITS GoInt;
typedef GoUintGOINTBITS GoUint;
typedef size_t GoUintptr;
typedef float GoFloat32;
typedef double GoFloat64;
#ifdef _MSC_VER
#if !defined(__cplusplus) || _MSVC_LANG <= 201402L
#include <complex.h>
typedef _Fcomplex GoComplex64;
typedef _Dcomplex GoComplex128;
#else
#include <complex>
typedef std::complex<float> GoComplex64;
typedef std::complex<double> GoComplex128;
#endif
#else
typedef float _Complex GoComplex64;
typedef double _Complex GoComplex128;
#endif

/*
  static assertion to make sure the file is being used on architecture
  at least with matching size of GoInt.
*/
typedef char _check_for_GOINTBITS_bit_pointer_matching_GoInt[sizeof(void*)==GOINTBITS/8 ? 1:-1];

#ifndef GO_CGO_GOSTRING_TYPEDEF
typedef _GoString_ GoString;
#endif
typedef void *GoMap;
typedef void *GoChan;
typedef struct { void *t; void *v; } GoInterface;
typedef struct { void *data; GoInt len; GoInt cap; } GoSlice;

#endif

/* End of boilerplate cgo prologue.  */

#ifdef __cplusplus
extern "C" {
#endif
`

// gccExportHeaderEpilog goes at the end of the generated header file.
const gccExportHeaderEpilog = `
#ifdef __cplusplus
}
#endif
`

// gccgoExportFileProlog is written to the _cgo_export.c file when
// using gccgo.
// We use weak declarations, and test the addresses, so that this code
// works with older versions of gccgo.
const gccgoExportFileProlog = `
#line 1 "cgo-gccgo-export-file-prolog"
extern _Bool runtime_iscgo __attribute__ ((weak));

static void GoInit(void) __attribute__ ((constructor));
static void GoInit(void) {
	if(&runtime_iscgo)
		runtime_iscgo = 1;
}

extern size_t _cgo_wait_runtime_init_done(void) __attribute__ ((weak));
`

```

// === FILE: references!/go/src/cmd/cgo/util.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
)

// run runs the command argv, feeding in stdin on standard input.
// It returns the output to standard output and standard error.
// ok indicates whether the command exited successfully.
func run(stdin []byte, argv []string) (stdout, stderr []byte, ok bool) {
	if i := slices.Index(argv, "-xc"); i >= 0 && argv[len(argv)-1] == "-" {
		// Some compilers have trouble with standard input.
		// Others have trouble with -xc.
		// Avoid both problems by writing a file with a .c extension.
		f, err := os.CreateTemp("", "cgo-gcc-input-")
		if err != nil {
			fatalf("%s", err)
		}
		name := f.Name()
		f.Close()
		if err := os.WriteFile(name+".c", stdin, 0666); err != nil {
			os.Remove(name)
			fatalf("%s", err)
		}
		defer os.Remove(name)
		defer os.Remove(name + ".c")

		// Build new argument list without -xc and trailing -.
		new := append(argv[:i:i], argv[i+1:len(argv)-1]...)

		// Since we are going to write the file to a temporary directory,
		// we will need to add -I . explicitly to the command line:
		// any #include "foo" before would have looked in the current
		// directory as the directory "holding" standard input, but now
		// the temporary directory holds the input.
		// We've also run into compilers that reject "-I." but allow "-I", ".",
		// so be sure to use two arguments.
		// This matters mainly for people invoking cgo -godefs by hand.
		new = append(new, "-I", ".")

		// Finish argument list with path to C file.
		new = append(new, name+".c")

		argv = new
		stdin = nil
	}

	p := exec.Command(argv[0], argv[1:]...)
	p.Stdin = bytes.NewReader(stdin)
	var bout, berr bytes.Buffer
	p.Stdout = &bout
	p.Stderr = &berr
	// Disable escape codes in clang error messages.
	p.Env = append(os.Environ(), "TERM=dumb")
	err := p.Run()
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		fatalf("exec %s: %s", argv[0], err)
	}
	ok = p.ProcessState.Success()
	stdout, stderr = bout.Bytes(), berr.Bytes()
	return
}

func lineno(pos token.Pos) string {
	return fset.Position(pos).String()
}

// Die with an error message.
func fatalf(msg string, args ...any) {
	// If we've already printed other errors, they might have
	// caused the fatal condition. Assume they're enough.
	if nerrors == 0 {
		fmt.Fprintf(os.Stderr, "cgo: "+msg+"\n", args...)
	}
	os.Exit(2)
}

var nerrors int

func error_(pos token.Pos, msg string, args ...any) {
	nerrors++
	if pos.IsValid() {
		fmt.Fprintf(os.Stderr, "%s: ", fset.Position(pos).String())
	} else {
		fmt.Fprintf(os.Stderr, "cgo: ")
	}
	fmt.Fprintf(os.Stderr, msg, args...)
	fmt.Fprintf(os.Stderr, "\n")
}

// create creates a file in the output directory.
func creat(name string) *os.File {
	f, err := os.Create(filepath.Join(outputDir(), name))
	if err != nil {
		fatalf("%s", err)
	}
	return f
}

// outputDir returns the output directory, making sure that it exists.
func outputDir() string {
	os.MkdirAll(*objDir, 0o700)
	return *objDir
}

```

