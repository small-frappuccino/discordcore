# Domain Architecture: runtime/_mkmalloc

## Layout Topology
```text
runtime/_mkmalloc/
├── astutil
│   └── clone.go
├── constants.go
├── go.mod
├── go.sum
├── mkmalloc.go
└── mksizeclasses.go
```

## Source Stream Aggregation

// === FILE: references/go/src/runtime/_mkmalloc/astutil/clone.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is a copy of golang.org/x/tools/internal/astutil/clone.go

package astutil

import (
	"go/ast"
	"reflect"
)

// CloneNode returns a deep copy of a Node.
// It omits pointers to ast.{Scope,Object} variables.
func CloneNode[T ast.Node](n T) T {
	return cloneNode(n).(T)
}

func cloneNode(n ast.Node) ast.Node {
	var clone func(x reflect.Value) reflect.Value
	set := func(dst, src reflect.Value) {
		src = clone(src)
		if src.IsValid() {
			dst.Set(src)
		}
	}
	clone = func(x reflect.Value) reflect.Value {
		switch x.Kind() {
		case reflect.Pointer:
			if x.IsNil() {
				return x
			}
			// Skip fields of types potentially involved in cycles.
			switch x.Interface().(type) {
			case *ast.Object, *ast.Scope:
				return reflect.Zero(x.Type())
			}
			y := reflect.New(x.Type().Elem())
			set(y.Elem(), x.Elem())
			return y

		case reflect.Struct:
			y := reflect.New(x.Type()).Elem()
			for i := 0; i < x.Type().NumField(); i++ {
				set(y.Field(i), x.Field(i))
			}
			return y

		case reflect.Slice:
			if x.IsNil() {
				return x
			}
			y := reflect.MakeSlice(x.Type(), x.Len(), x.Cap())
			for i := 0; i < x.Len(); i++ {
				set(y.Index(i), x.Index(i))
			}
			return y

		case reflect.Interface:
			y := reflect.New(x.Type()).Elem()
			set(y, x.Elem())
			return y

		case reflect.Array, reflect.Chan, reflect.Func, reflect.Map, reflect.UnsafePointer:
			panic(x) // unreachable in AST

		default:
			return x // bool, string, number
		}
	}
	return clone(reflect.ValueOf(n)).Interface().(ast.Node)
}

```

// === FILE: references/go/src/runtime/_mkmalloc/constants.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

const (
	// Constants that we use and will transfer to the runtime.
	minHeapAlign = 8
	maxSmallSize = 32 << 10
	smallSizeDiv = 8
	smallSizeMax = 1024
	largeSizeDiv = 128
	pageShift    = 13
	tinySize     = 16

	// Derived constants.
	pageSize = 1 << pageShift
)

const (
	maxPtrSize = max(4, 8)
	maxPtrBits = 8 * maxPtrSize

	// Maximum size to generate size specialized functions for.
	// We've seen very limited benefit for specialized functions for larger
	// size classes, and with the wrapper they are sometimes slower
	// than the non-specialized functions.
	// This must match the constant in the compiler.
	specializedMallocMax = 80
)

```

// === FILE: references/go/src/runtime/_mkmalloc/go.mod ===
```text
module _mkmalloc

go 1.26

require golang.org/x/tools v0.33.0

```

// === FILE: references/go/src/runtime/_mkmalloc/go.sum ===
```text
golang.org/x/tools v0.33.0 h1:4qz2S3zmRxbGIhDIAgjxvFutSvH5EfnsYrRBj0UI0bc=
golang.org/x/tools v0.33.0/go.mod h1:CIJMaWEY88juyUfo7UbgPqbC8rU2OqfAV1h2Qp0oMYI=

```

// === FILE: references/go/src/runtime/_mkmalloc/mkmalloc.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"

	"golang.org/x/tools/go/ast/astutil"

	internalastutil "runtime/_mkmalloc/astutil"
)

var stdout = flag.Bool("stdout", false, "write sizeclasses source to stdout instead of sizeclasses.go")

func makeSizeToSizeClass(classes []class) []uint8 {
	sc := uint8(0)
	ret := make([]uint8, benchmarkMax+1)
	for i := range ret {
		if i > classes[sc].size {
			sc++
		}
		ret[i] = sc
	}
	return ret
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("mkmalloc: ")

	classes := makeClasses()
	sizeToSizeClass := makeSizeToSizeClass(classes)

	if *stdout {
		if _, err := os.Stdout.Write(mustFormat(generateSizeClasses(classes))); err != nil {
			log.Fatal(err)
		}
		return
	}

	sizeclasesesfile := "../../internal/runtime/gc/sizeclasses.go"
	if err := os.WriteFile(sizeclasesesfile, mustFormat(generateSizeClasses(classes)), 0666); err != nil {
		log.Fatal(err)
	}

	outfile := "../malloc_generated.go"
	if err := os.WriteFile(outfile, mustFormat(inline(specializedMallocConfig(classes, sizeToSizeClass))), 0666); err != nil {
		log.Fatal(err)
	}

	tablefile := "../malloc_tables_generated.go"
	if err := os.WriteFile(tablefile, mustFormat(generateTable(sizeToSizeClass)), 0666); err != nil {
		log.Fatal(err)
	}

	benchmarkFile := "../malloc_bench_generated_test.go"
	if err := os.WriteFile(benchmarkFile, mustFormat(append(inline(benchmarkConfig(classes, sizeToSizeClass)), []byte(generateTopBenchmark(classes, sizeToSizeClass))...)), 0666); err != nil {
		log.Fatal(err)
	}

}

// withLineNumbers returns b with line numbers added to help debugging.
func withLineNumbers(b []byte) []byte {
	var buf bytes.Buffer
	i := 1
	for line := range bytes.Lines(b) {
		fmt.Fprintf(&buf, "%d: %s", i, line)
		i++
	}
	return buf.Bytes()
}

// mustFormat formats the input source, or exits if there's an error.
func mustFormat(b []byte) []byte {
	formatted, err := format.Source(b)
	if err != nil {
		log.Fatalf("error formatting source: %v\nsource:\n%s\n", err, withLineNumbers(b))
	}
	return formatted
}

// generatorConfig is the configuration for the generator. It uses the given file to find
// its templates, and generates each of the functions specified by specs.
type generatorConfig struct {
	file  string
	specs []spec
}

// spec is the specification for a function for the inliner to produce. The function gets
// the given name, and is produced by starting with the function with the name given by
// templateFunc and applying each of the ops.
type spec struct {
	name         string
	templateFunc string
	ops          []op
}

// replacementKind specifies the operation to ben done by a op.
type replacementKind int

const (
	inlineFunc = replacementKind(iota)
	subBasicLit
	foldCondition
	subIdent
	deleteConst
)

// op is a single inlining operation for the inliner. Any calls to the function
// from are replaced with the inlined body of to. For non-functions, uses of from are
// replaced with the basic literal expression given by to.
type op struct {
	kind replacementKind
	from string
	to   string
}

func smallScanNoHeaderSCFuncName(sc, scMax uint8) string {
	if sc == 0 || sc > scMax {
		return "mallocPanic"
	}
	return fmt.Sprintf("mallocgcSmallScanNoHeaderSC%d", sc)
}

const tinyFuncName = "mallocgcTinySC2"

func smallNoScanSCFuncName(sc, scMax uint8) string {
	if sc < 2 || sc > scMax {
		return "mallocPanic"
	}
	return fmt.Sprintf("mallocgcSmallNoScanSC%d", sc)
}

// specializedMallocConfig produces an inlining config to stamp out the definitions of the size-specialized
// malloc functions to be written by mkmalloc.
func specializedMallocConfig(classes []class, sizeToSizeClass []uint8) generatorConfig {
	config := generatorConfig{file: "../malloc_stubs.go"}

	// Only generate specialized functions for sizes up to specializedMallocMax.
	// We've noticed limited benefit (or sometimes worse performance) for specialized
	// functions for larger sizes, and having too many functions causes icache issues.
	scMax := sizeToSizeClass[specializedMallocMax]

	str := fmt.Sprint

	// allocations with pointer bits
	{
		const noscan = 0
		for sc := uint8(0); sc <= scMax; sc++ {
			if sc == 0 {
				continue
			}
			name := smallScanNoHeaderSCFuncName(sc, scMax)
			elemsize := classes[sc].size
			config.specs = append(config.specs, spec{
				templateFunc: "mallocStub",
				name:         name,
				ops: []op{
					{inlineFunc, "inlinedMalloc", "smallStub"},
					{inlineFunc, "postMallocgc", "postMallocgc"},
					{foldCondition, "isNoScan_", str(false)},
					{inlineFunc, "heapSetTypeNoHeaderStub", "heapSetTypeNoHeaderStub"},
					{inlineFunc, "nextFreeFastStub", "nextFreeFastStub"},
					{inlineFunc, "writeHeapBitsSmallStub", "writeHeapBitsSmallStub"},
					{foldCondition, "isSlowPath_", str(false)},
					{subBasicLit, "elemsize_", str(elemsize)},
					{subBasicLit, "sizeclass_", str(sc)},
					{subBasicLit, "noscanint_", str(noscan)},
					{foldCondition, "isTiny_", str(false)},
					{subIdent, "mallocgcSlowPathStub", "mallocgcSmallScanSlowPath"},
				},
			})
		}
	}

	// allocations without pointer bits
	{
		const noscan = 1

		// tiny
		tinySizeClass := sizeToSizeClass[tinySize]
		{
			name := tinyFuncName
			elemsize := classes[tinySizeClass].size
			config.specs = append(config.specs, spec{
				templateFunc: "mallocStub",
				name:         name,
				ops: []op{
					{inlineFunc, "inlinedMalloc", "tinyStub"},
					{inlineFunc, "nextFreeFastTiny", "nextFreeFastTiny"},
					{inlineFunc, "postMallocgc", "postMallocgc"},
					{inlineFunc, "nextFreeFastStub", "nextFreeFastStub"},
					{foldCondition, "isSlowPath_", str(false)},
					{subBasicLit, "elemsize_", str(elemsize)},
					{subBasicLit, "sizeclass_", str(tinySizeClass)},
					{subBasicLit, "noscanint_", str(noscan)},
					{foldCondition, "isTiny_", str(true)},
				},
			})
		}

		// non-tiny
		for sc := uint8(tinySizeClass); sc <= scMax; sc++ {
			name := smallNoScanSCFuncName(sc, scMax)
			elemsize := classes[sc].size
			config.specs = append(config.specs, spec{
				templateFunc: "mallocStub",
				name:         name,
				ops: []op{
					{inlineFunc, "inlinedMalloc", "smallStub"},
					{inlineFunc, "postMallocgc", "postMallocgc"},
					{foldCondition, "isNoScan_", str(true)},
					{inlineFunc, "nextFreeFastStub", "nextFreeFastStub"},
					{foldCondition, "isSlowPath_", str(false)},
					{subBasicLit, "elemsize_", str(elemsize)},
					{subBasicLit, "sizeclass_", str(sc)},
					{subBasicLit, "noscanint_", str(noscan)},
					{foldCondition, "isTiny_", str(false)},
					{subIdent, "mallocgcSlowPathStub", "mallocgcSmallNoScanSlowPath"},
				},
			})
		}
	}

	// Non-size-specialized fallbacks in case we can't do the fast path.
	config.specs = append(config.specs, spec{
		templateFunc: "mallocStub",
		name:         "mallocgcTinySlowPath",
		ops: []op{
			{inlineFunc, "inlinedMalloc", "tinyStub"},
			{inlineFunc, "postMallocgc", "postMallocgc"},
			{inlineFunc, "nextFreeFastTiny", "nextFreeFastTiny"},
			{inlineFunc, "deductAssistCredit", "deductAssistCredit"},
			{foldCondition, "isSlowPath_", str(true)},
			{foldCondition, "isTiny_", str(true)},
			{subBasicLit, "elemsize_", str(classes[sizeToSizeClass[tinySize]].size)},
		},
	})
	config.specs = append(config.specs, spec{
		templateFunc: "mallocgcSlowPathStub",
		name:         "mallocgcSmallScanSlowPath",
		ops: []op{
			{inlineFunc, "mallocStub", "mallocStub"},
			{inlineFunc, "inlinedMalloc", "smallStub"},
			{inlineFunc, "heapSetTypeNoHeaderStub", "heapSetTypeNoHeaderStub"},
			{inlineFunc, "writeHeapBitsSmallStub", "writeHeapBitsSmallStub"},
			{inlineFunc, "postMallocgc", "postMallocgc"},
			{inlineFunc, "nextFreeFastStub", "nextFreeFastStub"},
			{inlineFunc, "deductAssistCredit", "deductAssistCredit"},
			{foldCondition, "isSlowPath_", str(true)},
			{foldCondition, "isTiny_", str(false)},
			{foldCondition, "isNoScan_", str(false)},

			// Remove constants used by size-specialized variants.
			{deleteConst, "elemsize", ""},
			{deleteConst, "sizeclass", ""},
			{deleteConst, "spc", ""},
		},
	})
	config.specs = append(config.specs, spec{
		templateFunc: "mallocgcSlowPathStub",
		name:         "mallocgcSmallNoScanSlowPath",
		ops: []op{
			{inlineFunc, "mallocStub", "mallocStub"},
			{inlineFunc, "inlinedMalloc", "smallStub"},
			{inlineFunc, "postMallocgc", "postMallocgc"},
			{inlineFunc, "nextFreeFastStub", "nextFreeFastStub"},
			{inlineFunc, "deductAssistCredit", "deductAssistCredit"},
			{foldCondition, "isSlowPath_", str(true)},
			{foldCondition, "isTiny_", str(false)},
			{foldCondition, "isNoScan_", str(true)},

			// Remove constants used by size-specialized variants.
			{deleteConst, "elemsize", ""},
			{deleteConst, "sizeclass", ""},
			{deleteConst, "spc", ""},
		},
	})

	return config
}

// inline applies the inlining operations given by the config.
func inline(config generatorConfig) []byte {
	var out bytes.Buffer

	// Read the template file in.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, config.file, nil, parser.SkipObjectResolution)
	if err != nil {
		log.Fatalf("parsing %s: %v", config.file, err)
	}

	// Collect the function and import declarations. The function
	// declarations in the template file provide both the templates
	// that will be stamped out, and the functions that will be inlined
	// into them. The imports from the template file will be copied
	// straight to the output.
	funcDecls := map[string]*ast.FuncDecl{}
	importDecls := []*ast.GenDecl{}
	for _, decl := range f.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			funcDecls[decl.Name.Name] = decl
		case *ast.GenDecl:
			if decl.Tok.String() == "import" {
				importDecls = append(importDecls, decl)
				continue
			}
		}
	}

	// Write out the package and import declarations.
	out.WriteString("// Code generated by mkmalloc.go; DO NOT EDIT.\n")
	out.WriteString("// See overview in malloc_stubs.go.\n\n")
	out.WriteString("package " + f.Name.Name + "\n\n")
	for _, importDecl := range importDecls {
		out.Write(mustFormatNode(fset, importDecl))
		out.WriteString("\n\n")
	}

	// Produce each of the inlined functions specified by specs.
	for _, spec := range config.specs {
		// Start with a renamed copy of the template function.
		containingFuncCopy := internalastutil.CloneNode(funcDecls[spec.templateFunc])
		if containingFuncCopy == nil {
			log.Fatal("did not find", spec.templateFunc)
		}
		containingFuncCopy.Name.Name = spec.name

		// Apply each of the ops given by the specs
		stamped := ast.Node(containingFuncCopy)
		for _, repl := range spec.ops {
			switch repl.kind {
			case inlineFunc:
				if toDecl, ok := funcDecls[repl.to]; ok {
					stamped = inlineFunction(stamped, repl.from, toDecl)
				}
			case subBasicLit:
				stamped = substituteWithBasicLit(stamped, repl.from, repl.to)
			case foldCondition:
				stamped = foldIfCondition(stamped, repl.from, repl.to)
			case subIdent:
				stamped = substituteIdent(stamped, repl.from, repl.to)
			case deleteConst:
				stamped = deleteConstDecl(stamped, repl.from)
			default:
				log.Fatalf("unknown op kind %v", repl.kind)
			}
		}

		stamped = cleanLabels(stamped)

		out.Write(mustFormatNode(fset, stamped))
		out.WriteString("\n\n")
	}

	return out.Bytes()
}

// substituteWithBasicLit recursively renames identifiers in the provided AST
// according to 'from' and 'to'.
func substituteWithBasicLit(node ast.Node, from, to string) ast.Node {
	// The op is a substitution of an identifier with a basic literal.
	toExpr, err := parser.ParseExpr(to)
	if err != nil {
		log.Fatalf("parsing expr %q: %v", to, err)
	}
	toLit, ok := toExpr.(*ast.BasicLit)
	if !ok {
		log.Fatalf("op 'to' expr %q is not a basic literal", to)
	}
	return astutil.Apply(node, func(cursor *astutil.Cursor) bool {
		if ident, ok := cursor.Node().(*ast.Ident); ok && ident.Name == from {
			replacement := *toLit
			replacement.ValuePos = ident.NamePos
			cursor.Replace(new(replacement))
		}
		return true
	}, nil)
}

// substituteIdent replaces the ident named 'from' to 'to'.
func substituteIdent(node ast.Node, from, to string) ast.Node {
	return astutil.Apply(node, func(cursor *astutil.Cursor) bool {
		if ident, ok := cursor.Node().(*ast.Ident); ok && ident.Name == from {
			cursor.Replace(&ast.Ident{Name: to, NamePos: ident.NamePos})
		}
		return true
	}, nil)
}

// foldIfCondition replaces 'from' with 'to', which must be "true" or "false".
// It then applies simplifications to any boolean expressions that have literal
// true or false values, from the bottom up. Any if statements that have a condition
// that is a literal true or false after the simplification will be replaced with
// their bodies (in the true case) or deleted (in the false case).
func foldIfCondition(node ast.Node, from, to string) ast.Node {
	boolLit := func(n ast.Expr) (v, ok bool) {
		if ident, ok := ast.Unparen(n).(*ast.Ident); ok {
			switch ident.Name {
			case "true":
				return true, true
			case "false":
				return false, true
			}
			return false, false
		}
		return false, false
	}
	handleIfs := func(cursor *astutil.Cursor) bool {
		switch n := cursor.Node().(type) {
		case *ast.Ident:
			// First, do the replacement.
			if n.Name == from {
				cursor.Replace(&ast.Ident{Name: to, NamePos: n.NamePos})
			}
		case *ast.UnaryExpr:
			if n.Op == token.NOT {
				if b, ok := boolLit(n.X); ok {
					name := "true"
					if b {
						name = "false"
					}
					cursor.Replace(&ast.Ident{Name: name, NamePos: n.Pos()})
				}
			}
		case *ast.BinaryExpr:
			xBool, xOk := boolLit(n.X)
			yBool, yOk := boolLit(n.Y)
			if n.Op == token.LAND {
				switch {
				case xOk && !xBool || yOk && !yBool:
					cursor.Replace(&ast.Ident{Name: "false", NamePos: n.Pos()})
				case xOk && xBool:
					cursor.Replace(n.Y)
				case yOk && yBool:
					cursor.Replace(n.X)
				}
			} else if n.Op == token.LOR {
				switch {
				case xOk && xBool || yOk && yBool:
					cursor.Replace(&ast.Ident{Name: "true", NamePos: n.Pos()})
				case xOk && !xBool:
					cursor.Replace(n.Y)
				case yOk && !yBool:
					cursor.Replace(n.X)
				}
			}
		case *ast.IfStmt:
			if v, ok := boolLit(n.Cond); ok {
				if cursor.Index() < 0 {
					replacement := ast.Node(&ast.EmptyStmt{})
					if v {
						replacement = n.Body
					}
					cursor.Replace(replacement)
					break
				}
				if v {
					for _, stmt := range n.Body.List {
						cursor.InsertBefore(stmt)
					}
				} else if n.Else != nil {
					if block, ok := n.Else.(*ast.BlockStmt); ok {
						for i := len(block.List) - 1; i >= 0; i-- {
							cursor.InsertAfter(block.List[i])
						}
					}
				}
				cursor.Delete()
			}
		case *ast.LabeledStmt:
			// This case isn't necessary but it moves the code
			// out of the block so that it looks cleaner.
			if inner, ok := n.Stmt.(*ast.BlockStmt); ok {
				if len(inner.List) == 0 {
					cursor.Delete()
					break
				}
				list := inner.List
				n.Stmt = list[0]
				for i := len(list) - 1; i > 0; i-- {
					cursor.InsertAfter(list[i])
				}
			}
		}
		return true
	}
	return astutil.Apply(node, nil, handleIfs)
}

func cleanLabels(node ast.Node) ast.Node {
	found := map[string]bool{}
	ast.Inspect(node, func(node ast.Node) bool {
		if branch, ok := node.(*ast.BranchStmt); ok {
			if branch.Label != nil {
				found[branch.Label.Name] = true
			}
		}
		return true
	})
	return astutil.Apply(node, nil, func(cursor *astutil.Cursor) bool {
		if lstmt, ok := cursor.Node().(*ast.LabeledStmt); ok {
			if !found[lstmt.Label.Name] {
				if _, ok := lstmt.Stmt.(*ast.EmptyStmt); ok {
					cursor.Delete()
				} else {
					cursor.Replace(lstmt.Stmt)
				}
			}
		}
		return true
	})
}

// reports whether this is a non-grouped constant decl named 'name'.
func isNamedConstDecl(node ast.Node, name string) bool {
	declStmt, ok := node.(*ast.DeclStmt)
	if !ok {
		return false
	}

	genDecl, ok := declStmt.Decl.(*ast.GenDecl)
	if !ok || genDecl.Tok != token.CONST {
		return false
	}

	if len(genDecl.Specs) != 1 {
		return false
	}
	vs, ok := genDecl.Specs[0].(*ast.ValueSpec)
	if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
		return false
	}

	return vs.Names[0].Name == name
}

// deleteConstDecl removes const declarations whose name matches the given name.
// It only applies to declaration statements with a single declaration.
func deleteConstDecl(node ast.Node, name string) ast.Node {
	return astutil.Apply(node, func(cursor *astutil.Cursor) bool {
		if isNamedConstDecl(cursor.Node(), name) {
			cursor.Delete()
		}
		return true
	}, nil)
}

// inlineFunction recursively replaces calls to the function 'from' with the body of the function
// 'toDecl'. All calls to 'from' must either have no return values and appear in standalone expression statements
// or otherwise must appear in assignment statements.
// The replacement is very simple: it doesn't substitute the arguments for the parameters, so the
// arguments to the function call must be the same identifier as the parameters to the function
// declared by 'toDecl'. If there are any calls to from where that's not the case there will be a fatal error.
func inlineFunction(node ast.Node, from string, toDecl *ast.FuncDecl) ast.Node {
	return astutil.Apply(node, func(cursor *astutil.Cursor) bool {
		switch node := cursor.Node().(type) {
		case *ast.AssignStmt:
			// TODO(matloob) CHECK function args have same name
			// as parameters (or parameter is "_").
			if len(node.Rhs) == 1 && isCallTo(node.Rhs[0], from) {
				args := node.Rhs[0].(*ast.CallExpr).Args
				if !argsMatchParameters(args, toDecl.Type.Params) {
					log.Fatalf("applying op: arguments to %v don't match parameter names of %v: %v", from, toDecl.Name, debugPrint(args...))
				}
				replaceAssignment(cursor, node, toDecl)
			}
			return false
		case *ast.ExprStmt:
			if callExpr, ok := node.X.(*ast.CallExpr); ok && isCallTo(callExpr, from) {
				if !argsMatchParameters(callExpr.Args, toDecl.Type.Params) {
					log.Fatalf("applying op: arguments to %v don't match parameter names of %v: %v", from, toDecl.Name, debugPrint(callExpr.Args...))
				}
				if toDecl.Type.Results != nil {
					log.Fatalf("applying op: call to %v, which does not appear in an assignment, is replaced with %v which has return values: %v", from, toDecl.Name, debugPrint(callExpr.Args...))
				}
				replaceCallExprStmt(cursor, toDecl)
			}
			return false
		case *ast.ReturnStmt:
			if len(node.Results) == 1 && isCallTo(node.Results[0], from) {
				args := node.Results[0].(*ast.CallExpr).Args
				if !argsMatchParameters(args, toDecl.Type.Params) {
					log.Fatalf("applying op: arguments to %v don't match parameter names of %v: %v", from, toDecl.Name, debugPrint(args...))
				}
				replaceTailCall(cursor, toDecl)
			}
			return false
		case *ast.CallExpr:
			if isCallTo(node, from) {
				switch cursor.Parent().(type) {
				case *ast.AssignStmt, *ast.ExprStmt:
				default:
					log.Fatalf("applying op: all calls to function %q being replaced must appear in an assignment or expression statement, appears in %T", from, cursor.Parent())
				}
			}
		}
		return true
	}, nil)
}

// argsMatchParameters reports whether the arguments given by args are all identifiers
// whose names are the same as the corresponding parameters in params.
func argsMatchParameters(args []ast.Expr, params *ast.FieldList) bool {
	var paramIdents []*ast.Ident
	for _, f := range params.List {
		paramIdents = append(paramIdents, f.Names...)
	}

	if len(args) != len(paramIdents) {
		return false
	}

	for i := range args {
		if !isIdentWithName(args[i], paramIdents[i].Name) {
			return false
		}
	}

	return true
}

// isIdentWithName reports whether the expression is an identifier with the given name.
func isIdentWithName(expr ast.Node, name string) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == name
}

// isCallTo reports whether the expression is a call expression to the function with the given name.
func isCallTo(expr ast.Expr, name string) bool {
	callexpr, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	return isIdentWithName(callexpr.Fun, name)
}

// replaceCallExprStmt replaces a standalone expression statement calling a function with no
// return values with the body of the function.
func replaceCallExprStmt(cursor *astutil.Cursor, funcdecl *ast.FuncDecl) {
	body := internalastutil.CloneNode(funcdecl.Body)
	for _, stmt := range body.List {
		cursor.InsertBefore(stmt)
	}
	cursor.Delete()
}

func replaceTailCall(cursor *astutil.Cursor, funcdecl *ast.FuncDecl) {
	if !hasTerminatingReturn(funcdecl.Body) {
		log.Fatal("function being inlined must have a return at the end")
	}

	body := internalastutil.CloneNode(funcdecl.Body)
	if len(body.List) < 1 {
		log.Fatal("replacing with empty bodied function")
	}

	// The op happens in two steps: first we insert the body of the function being inlined (except for
	// the final return) before the assignment, and then we change the assignment statement to replace the function call
	// with the expressions being returned.

	// Insert the body up to the final return.
	for _, stmt := range body.List {
		cursor.InsertBefore(stmt)
	}
	cursor.Delete()
}

// replaceAssignment replaces an assignment statement where the right hand side is a function call
// whose arguments have the same names as the parameters to funcdecl with the body of funcdecl.
// It sets the left hand side of the assignment to the return values of the function.
func replaceAssignment(cursor *astutil.Cursor, assign *ast.AssignStmt, funcdecl *ast.FuncDecl) {
	if !hasTerminatingReturn(funcdecl.Body) {
		log.Fatal("function being inlined must have a return at the end")
	}

	body := internalastutil.CloneNode(funcdecl.Body)
	if hasTerminatingAndNonterminatingReturn(funcdecl.Body) {
		// The function has multiple return points. Add the code that we'd continue with in the caller
		// after each of the return points. The calling function must have a terminating return
		// so we don't continue execution in the replaced function after we finish executing the
		// continue block that we add.
		body = addContinues(cursor, assign, body, everythingFollowingInParent(cursor)).(*ast.BlockStmt)
	}

	if len(body.List) < 1 {
		log.Fatal("replacing with empty bodied function")
	}

	// The op happens in two steps: first we insert the body of the function being inlined (except for
	// the final return) before the assignment, and then we change the assignment statement to replace the function call
	// with the expressions being returned.

	// Determine the expressions being returned.
	beforeReturn, ret := body.List[:len(body.List)-1], body.List[len(body.List)-1]
	returnStmt, ok := ret.(*ast.ReturnStmt)
	if !ok {
		log.Fatal("last stmt in function we're replacing with should be a return")
	}
	results := returnStmt.Results

	// Insert the body up to the final return.
	for _, stmt := range beforeReturn {
		cursor.InsertBefore(stmt)
	}

	// Rewrite the assignment statement.
	replaceWithAssignment(cursor, assign.Lhs, results, assign.Tok)
}

// hasTerminatingReturn reparts whether the block ends in a return statement.
func hasTerminatingReturn(block *ast.BlockStmt) bool {
	_, ok := block.List[len(block.List)-1].(*ast.ReturnStmt)
	return ok
}

// hasTerminatingAndNonterminatingReturn reports whether the block ends in a return
// statement, and also has a return elsewhere in it.
func hasTerminatingAndNonterminatingReturn(block *ast.BlockStmt) bool {
	if !hasTerminatingReturn(block) {
		return false
	}
	var ret bool
	for i := range block.List[:len(block.List)-1] {
		ast.Inspect(block.List[i], func(node ast.Node) bool {
			_, ok := node.(*ast.ReturnStmt)
			if ok {
				ret = true
				return false
			}
			return true
		})
	}
	return ret
}

// everythingFollowingInParent returns a block with everything in the parent block node of the cursor after
// the cursor itself. The cursor must point to an element in a block node's list.
func everythingFollowingInParent(cursor *astutil.Cursor) *ast.BlockStmt {
	parent := cursor.Parent()
	block, ok := parent.(*ast.BlockStmt)
	if !ok {
		log.Fatal("internal error: in everythingFollowingInParent, cursor doesn't point to element in block list")
	}

	blockcopy := internalastutil.CloneNode(block)      // get a clean copy
	blockcopy.List = blockcopy.List[cursor.Index()+1:] // and remove everything before and including stmt

	if _, ok := blockcopy.List[len(blockcopy.List)-1].(*ast.ReturnStmt); !ok {
		log.Printf("%s", mustFormatNode(token.NewFileSet(), blockcopy))
		log.Fatal("internal error: parent doesn't end in a return")
	}
	return blockcopy
}

// in the case that there's a return in the body being inlined (toBlock), addContinues
// replaces those returns that are not at the end of the function with the code in the
// caller after the function call that execution would continue with after the return.
// The block being added must end in a return.
func addContinues(cursor *astutil.Cursor, assignNode *ast.AssignStmt, toBlock *ast.BlockStmt, continueBlock *ast.BlockStmt) ast.Node {
	if !hasTerminatingReturn(continueBlock) {
		log.Fatal("the block being continued to in addContinues must end in a return")
	}
	applyFunc := func(cursor *astutil.Cursor) bool {
		ret, ok := cursor.Node().(*ast.ReturnStmt)
		if !ok {
			return true
		}

		if cursor.Parent() == toBlock && cursor.Index() == len(toBlock.List)-1 {
			return false
		}

		// This is the opposite of replacing a function call with the body. First
		// we replace the return statement with the assignment from the caller, and
		// then add the code we continue with.
		replaceWithAssignment(cursor, assignNode.Lhs, ret.Results, assignNode.Tok)
		cursor.InsertAfter(internalastutil.CloneNode(continueBlock))

		return false
	}
	return astutil.Apply(toBlock, applyFunc, nil)
}

// debugPrint prints out the expressions given by nodes for debugging.
func debugPrint(nodes ...ast.Expr) string {
	var b strings.Builder
	for i, node := range nodes {
		b.Write(mustFormatNode(token.NewFileSet(), node))
		if i != len(nodes)-1 {
			b.WriteString(", ")
		}
	}
	return b.String()
}

// mustFormatNode produces the formatted Go code for the given node.
func mustFormatNode(fset *token.FileSet, node any) []byte {
	var buf bytes.Buffer
	format.Node(&buf, fset, node)
	return buf.Bytes()
}

// mustMatchExprs makes sure that the expression lists have the same length,
// and returns the lists of the expressions on the lhs and rhs where the
// identifiers are not the same. These are used to produce assignment statements
// where the expressions on the right are assigned to the identifiers on the left.
func mustMatchExprs(lhs []ast.Expr, rhs []ast.Expr) ([]ast.Expr, []ast.Expr) {
	if len(lhs) != len(rhs) {
		log.Fatal("exprs don't match", debugPrint(lhs...), debugPrint(rhs...))
	}

	var newLhs, newRhs []ast.Expr
	for i := range lhs {
		lhsIdent, ok1 := lhs[i].(*ast.Ident)
		rhsIdent, ok2 := rhs[i].(*ast.Ident)
		if ok1 && ok2 && lhsIdent.Name == rhsIdent.Name {
			continue
		}
		newLhs = append(newLhs, lhs[i])
		newRhs = append(newRhs, rhs[i])
	}

	return newLhs, newRhs
}

// replaceWithAssignment replaces the node pointed to by the cursor with an assignment of the
// left hand side to the righthand side, removing any redundant assignments of a variable to itself,
// and replacing an assignment to a single basic literal with a constant declaration.
func replaceWithAssignment(cursor *astutil.Cursor, lhs, rhs []ast.Expr, tok token.Token) {
	newLhs, newRhs := mustMatchExprs(lhs, rhs)
	if len(newLhs) == 0 {
		cursor.Delete()
		return
	}
	if len(newRhs) == 1 {
		if lit, ok := newRhs[0].(*ast.BasicLit); ok {
			constDecl := &ast.DeclStmt{
				Decl: &ast.GenDecl{
					Tok: token.CONST,
					Specs: []ast.Spec{
						&ast.ValueSpec{
							Names:  []*ast.Ident{newLhs[0].(*ast.Ident)},
							Values: []ast.Expr{lit},
						},
					},
				},
			}
			cursor.Replace(constDecl)
			return
		}
	}
	newAssignment := &ast.AssignStmt{
		Lhs: newLhs,
		Rhs: newRhs,
		Tok: tok,
	}
	cursor.Replace(newAssignment)
}

// generateTable generates the file with the jump tables for the specialized malloc functions.
func generateTable(sizeToSizeClass []uint8) []byte {
	scMax := sizeToSizeClass[specializedMallocMax]

	var b bytes.Buffer
	fmt.Fprintf(&b, `// Code generated by mkmalloc.go; DO NOT EDIT.
//go:build !plan9

package runtime

import "unsafe"

var mallocScanTable = [%d]func(size uintptr, typ *_type, needzero bool) unsafe.Pointer{`, specializedMallocMax+1)

	for i := range uintptr(specializedMallocMax + 1) {
		fmt.Fprintf(&b, "%s,\n", smallScanNoHeaderSCFuncName(sizeToSizeClass[i], scMax))
	}

	fmt.Fprintf(&b, `
}

var mallocNoScanTable = [%d]func(size uintptr, typ *_type, needzero bool) unsafe.Pointer{`, specializedMallocMax+1)
	for i := range uintptr(specializedMallocMax + 1) {
		if i < 16 {
			fmt.Fprintf(&b, "%s,\n", "mallocPanic")
		} else {
			fmt.Fprintf(&b, "%s,\n", smallNoScanSCFuncName(sizeToSizeClass[i], scMax))
		}
	}

	fmt.Fprintln(&b, `
}`)

	return b.Bytes()
}

// Generate benchmarks for all potentially small sizes
// (sizes for which smallScanNoHeader would be called)
// gc.MinSizeForMallocHeader is defined as goarch.PtrSize * goarch.PtrBits.

const benchmarkMax = maxPtrSize * maxPtrBits

// benchmarkConfig produces an inlining config to stamp out microbenchmarks.
func benchmarkConfig(classes []class, sizeToSizeClass []uint8) generatorConfig {
	config := generatorConfig{file: "../malloc_stubs_test.go"}

	scMax := sizeToSizeClass[benchmarkMax]

	str := fmt.Sprint

	for sc := uint8(1); sc <= scMax; sc++ {
		elemsize := classes[sc].size
		config.specs = append(config.specs, spec{
			templateFunc: "benchmarkStub",
			name:         fmt.Sprintf("benchmarkMallocgcNoscan%d", elemsize),
			ops: []op{
				{subBasicLit, "size_", str(elemsize)},
				{foldCondition, "noscan_", str(true)},
			},
		})
		config.specs = append(config.specs, spec{
			templateFunc: "benchmarkStub",
			name:         fmt.Sprintf("benchmarkMallocgcScan%d", elemsize),
			ops: []op{
				{subBasicLit, "size_", str(elemsize)},
				{foldCondition, "noscan_", str(false)},
			},
		})
		config.specs = append(config.specs, spec{
			templateFunc: "benchmarkScanSliceStub",
			name:         fmt.Sprintf("benchmarkMallocgcScanSlice%d", elemsize),
			ops:          []op{{subBasicLit, "size_", str(elemsize)}},
		})
	}

	for size := 1; size < tinySize; size++ {
		config.specs = append(config.specs, spec{
			templateFunc: "benchmarkStubTiny",
			name:         fmt.Sprintf("benchmarkMallocgcTiny%d", size),
			ops:          []op{{subBasicLit, "size_", str(size)}, {foldCondition, "noscan_", str(true)}},
		})
	}

	return config
}

func generateTopBenchmark(classes []class, sizeToSizeClass []uint8) string {
	scMax := sizeToSizeClass[benchmarkMax]
	bench := `func BenchmarkMallocgc(b *testing.B) {
		b.Run("scan=noscan", func(b *testing.B) {
`
	for size := 1; size < tinySize; size++ {
		bench += fmt.Sprintf(`b.Run("size=%d", benchmarkMallocgcTiny%d)`, size, size) + "\n"
	}
	for sc := uint8(2); sc <= scMax; sc++ {
		elemsize := classes[sc].size
		bench += fmt.Sprintf(`b.Run("size=%d", benchmarkMallocgcNoscan%d)`, elemsize, elemsize) + "\n"
	}
	bench += `})
		b.Run("scan=scan", func(b *testing.B) {
`
	for sc := uint8(1); sc <= scMax; sc++ {
		elemsize := classes[sc].size
		bench += fmt.Sprintf(`b.Run("size=%d", benchmarkMallocgcScan%d)`, elemsize, elemsize) + "\n"

	}
	bench += `})
		b.Run("scan=scanslice", func(b *testing.B) {
`
	for sc := uint8(1); sc <= scMax; sc++ {
		elemsize := classes[sc].size
		bench += fmt.Sprintf(`b.Run("size=%d", benchmarkMallocgcScanSlice%d)`, elemsize, elemsize) + "\n"
	}
	bench += `})
}`

	return bench
}

```

// === FILE: references/go/src/runtime/_mkmalloc/mksizeclasses.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Generate tables for small malloc size classes.
//
// See malloc.go for overview.
//
// The size classes are chosen so that rounding an allocation
// request up to the next size class wastes at most 12.5% (1.125x).
//
// Each size class has its own page count that gets allocated
// and chopped up when new objects of the size class are needed.
// That page count is chosen so that chopping up the run of
// pages into objects of the given size wastes at most 12.5% (1.125x)
// of the memory. It is not necessary that the cutoff here be
// the same as above.
//
// The two sources of waste multiply, so the worst possible case
// for the above constraints would be that allocations of some
// size might have a 26.6% (1.266x) overhead.
// In practice, only one of the wastes comes into play for a
// given size (sizes < 512 waste mainly on the round-up,
// sizes > 512 waste mainly on the page chopping).
// For really small sizes, alignment constraints force the
// overhead higher.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"math"
	"math/bits"
)

// Generate internal/runtime/gc/msize.go

func generateSizeClasses(classes []class) []byte {
	flag.Parse()

	var b bytes.Buffer
	fmt.Fprintln(&b, "// Code generated by mksizeclasses.go; DO NOT EDIT.")
	fmt.Fprintln(&b, "//go:generate go -C ../../../runtime/_mkmalloc run mksizeclasses.go")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "package gc")

	printComment(&b, classes)

	printClasses(&b, classes)

	return b.Bytes()
}

type class struct {
	size   int // max size
	npages int // number of pages
}

func powerOfTwo(x int) bool {
	return x != 0 && x&(x-1) == 0
}

func makeClasses() []class {
	var classes []class

	classes = append(classes, class{}) // class #0 is a dummy entry

	align := minHeapAlign
	for size := align; size <= maxSmallSize; size += align {
		if powerOfTwo(size) { // bump alignment once in a while
			if size >= 2048 {
				align = 256
			} else if size >= 128 {
				align = size / 8
			} else if size >= 32 {
				align = 16 // heap bitmaps assume 16 byte alignment for allocations >= 32 bytes.
			}
		}
		if !powerOfTwo(align) {
			panic("incorrect alignment")
		}

		// Make the allocnpages big enough that
		// the leftover is less than 1/8 of the total,
		// so wasted space is at most 12.5%.
		allocsize := pageSize
		for allocsize%size > allocsize/8 {
			allocsize += pageSize
		}
		npages := allocsize / pageSize

		// If the previous sizeclass chose the same
		// allocation size and fit the same number of
		// objects into the page, we might as well
		// use just this size instead of having two
		// different sizes.
		if len(classes) > 1 && npages == classes[len(classes)-1].npages && allocsize/size == allocsize/classes[len(classes)-1].size {
			classes[len(classes)-1].size = size
			continue
		}
		classes = append(classes, class{size: size, npages: npages})
	}

	// Increase object sizes if we can fit the same number of larger objects
	// into the same number of pages. For example, we choose size 8448 above
	// with 6 objects in 7 pages. But we can well use object size 9472,
	// which is also 6 objects in 7 pages but +1024 bytes (+12.12%).
	// We need to preserve at least largeSizeDiv alignment otherwise
	// sizeToClass won't work.
	for i := range classes {
		if i == 0 {
			continue
		}
		c := &classes[i]
		psize := c.npages * pageSize
		new_size := (psize / (psize / c.size)) &^ (largeSizeDiv - 1)
		if new_size > c.size {
			c.size = new_size
		}
	}

	if len(classes) != 68 {
		panic("number of size classes has changed")
	}

	for i := range classes {
		computeDivMagic(&classes[i])
	}

	return classes
}

// computeDivMagic checks that the division required to compute object
// index from span offset can be computed using 32-bit multiplication.
// n / c.size is implemented as (n * (^uint32(0)/uint32(c.size) + 1)) >> 32
// for all 0 <= n <= c.npages * pageSize
func computeDivMagic(c *class) {
	// divisor
	d := c.size
	if d == 0 {
		return
	}

	// maximum input value for which the formula needs to work.
	max := c.npages * pageSize

	// As reported in [1], if n and d are unsigned N-bit integers, we
	// can compute n / d as ⌊n * c / 2^F⌋, where c is ⌈2^F / d⌉ and F is
	// computed with:
	//
	// 	Algorithm 2: Algorithm to select the number of fractional bits
	// 	and the scaled approximate reciprocal in the case of unsigned
	// 	integers.
	//
	// 	if d is a power of two then
	// 		Let F ← log₂(d) and c = 1.
	// 	else
	// 		Let F ← N + L where L is the smallest integer
	// 		such that d ≤ (2^(N+L) mod d) + 2^L.
	// 	end if
	//
	// [1] "Faster Remainder by Direct Computation: Applications to
	// Compilers and Software Libraries" Daniel Lemire, Owen Kaser,
	// Nathan Kurz arXiv:1902.01961
	//
	// To minimize the risk of introducing errors, we implement the
	// algorithm exactly as stated, rather than trying to adapt it to
	// fit typical Go idioms.
	N := bits.Len(uint(max))
	var F int
	if powerOfTwo(d) {
		F = int(math.Log2(float64(d)))
		if d != 1<<F {
			panic("imprecise log2")
		}
	} else {
		for L := 0; ; L++ {
			if d <= ((1<<(N+L))%d)+(1<<L) {
				F = N + L
				break
			}
		}
	}

	// Also, noted in the paper, F is the smallest number of fractional
	// bits required. We use 32 bits, because it works for all size
	// classes and is fast on all CPU architectures that we support.
	if F > 32 {
		fmt.Printf("d=%d max=%d N=%d F=%d\n", c.size, max, N, F)
		panic("size class requires more than 32 bits of precision")
	}

	// Brute force double-check with the exact computation that will be
	// done by the runtime.
	m := ^uint32(0)/uint32(c.size) + 1
	for n := 0; n <= max; n++ {
		if uint32((uint64(n)*uint64(m))>>32) != uint32(n/c.size) {
			fmt.Printf("d=%d max=%d m=%d n=%d\n", d, max, m, n)
			panic("bad 32-bit multiply magic")
		}
	}
}

func printComment(w io.Writer, classes []class) {
	fmt.Fprintf(w, "// %-5s  %-9s  %-10s  %-7s  %-10s  %-9s  %-9s\n", "class", "bytes/obj", "bytes/span", "objects", "tail waste", "max waste", "min align")
	prevSize := 0
	var minAligns [pageShift + 1]int
	for i, c := range classes {
		if i == 0 {
			continue
		}
		spanSize := c.npages * pageSize
		objects := spanSize / c.size
		tailWaste := spanSize - c.size*(spanSize/c.size)
		maxWaste := float64((c.size-prevSize-1)*objects+tailWaste) / float64(spanSize)
		alignBits := bits.TrailingZeros(uint(c.size))
		if alignBits > pageShift {
			// object alignment is capped at page alignment
			alignBits = pageShift
		}
		for i := range minAligns {
			if i > alignBits {
				minAligns[i] = 0
			} else if minAligns[i] == 0 {
				minAligns[i] = c.size
			}
		}
		prevSize = c.size
		fmt.Fprintf(w, "// %5d  %9d  %10d  %7d  %10d  %8.2f%%  %9d\n", i, c.size, spanSize, objects, tailWaste, 100*maxWaste, 1<<alignBits)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "// %-9s  %-4s  %-12s\n", "alignment", "bits", "min obj size")
	for bits, size := range minAligns {
		if size == 0 {
			break
		}
		if bits+1 < len(minAligns) && size == minAligns[bits+1] {
			continue
		}
		fmt.Fprintf(w, "// %9d  %4d  %12d\n", 1<<bits, bits, size)
	}
	fmt.Fprintf(w, "\n")
}

func maxObjsPerSpan(classes []class) int {
	most := 0
	for _, c := range classes[1:] {
		n := c.npages * pageSize / c.size
		most = max(most, n)
	}
	return most
}

func maxNPages(classes []class) int {
	most := 0
	for _, c := range classes[1:] {
		most = max(most, c.npages)
	}
	return most
}

func printClasses(w io.Writer, classes []class) {
	sizeToSizeClass := func(size int) int {
		for j, c := range classes {
			if c.size >= size {
				return j
			}
		}
		panic("unreachable")
	}

	fmt.Fprintln(w, "const (")
	fmt.Fprintf(w, "MinHeapAlign = %d\n", minHeapAlign)
	fmt.Fprintf(w, "MaxSmallSize = %d\n", maxSmallSize)
	fmt.Fprintf(w, "SmallSizeDiv = %d\n", smallSizeDiv)
	fmt.Fprintf(w, "SmallSizeMax = %d\n", smallSizeMax)
	fmt.Fprintf(w, "LargeSizeDiv = %d\n", largeSizeDiv)
	fmt.Fprintf(w, "NumSizeClasses = %d\n", len(classes))
	fmt.Fprintf(w, "PageShift = %d\n", pageShift)
	fmt.Fprintf(w, "MaxObjsPerSpan = %d\n", maxObjsPerSpan(classes))
	fmt.Fprintf(w, "MaxSizeClassNPages = %d\n", maxNPages(classes))
	fmt.Fprintf(w, "TinySize = %d\n", tinySize)
	fmt.Fprintf(w, "TinySizeClass = %d\n", sizeToSizeClass(tinySize))
	fmt.Fprintln(w, ")")

	fmt.Fprint(w, "var SizeClassToSize = [NumSizeClasses]uint16 {")
	for _, c := range classes {
		fmt.Fprintf(w, "%d,", c.size)
	}
	fmt.Fprintln(w, "}")

	fmt.Fprint(w, "var SizeClassToNPages = [NumSizeClasses]uint8 {")
	for _, c := range classes {
		fmt.Fprintf(w, "%d,", c.npages)
	}
	fmt.Fprintln(w, "}")

	fmt.Fprint(w, "var SizeClassToDivMagic = [NumSizeClasses]uint32 {")
	for _, c := range classes {
		if c.size == 0 {
			fmt.Fprintf(w, "0,")
			continue
		}
		fmt.Fprintf(w, "^uint32(0)/%d+1,", c.size)
	}
	fmt.Fprintln(w, "}")

	// map from size to size class, for small sizes.
	sc := make([]int, smallSizeMax/smallSizeDiv+1)
	for i := range sc {
		size := i * smallSizeDiv
		sc[i] = sizeToSizeClass(size)
	}
	fmt.Fprint(w, "var SizeToSizeClass8 = [SmallSizeMax/SmallSizeDiv+1]uint8 {")
	for _, v := range sc {
		fmt.Fprintf(w, "%d,", v)
	}
	fmt.Fprintln(w, "}")

	// map from size to size class, for large sizes.
	sc = make([]int, (maxSmallSize-smallSizeMax)/largeSizeDiv+1)
	for i := range sc {
		size := smallSizeMax + i*largeSizeDiv
		sc[i] = sizeToSizeClass(size)
	}
	fmt.Fprint(w, "var SizeToSizeClass128 = [(MaxSmallSize-SmallSizeMax)/LargeSizeDiv+1]uint8 {")
	for _, v := range sc {
		fmt.Fprintf(w, "%d,", v)
	}
	fmt.Fprintln(w, "}")
}

```

