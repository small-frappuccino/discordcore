# Domain Architecture: cmd/compile/internal/types2

## Layout Topology
```text
cmd/compile/internal/types2/
├── README.md
├── alias.go
├── api.go
├── api_predicates.go
├── array.go
├── assignments.go
├── basic.go
├── builtins.go
├── call.go
├── chan.go
├── check.go
├── compiler_internal.go
├── compilersupport.go
├── const.go
├── context.go
├── conversions.go
├── cycles.go
├── decl.go
├── errors.go
├── errsupport.go
├── expr.go
├── format.go
├── gccgosizes.go
├── gcsizes.go
├── index.go
├── infer.go
├── initorder.go
├── instantiate.go
├── interface.go
├── labels.go
├── literals.go
├── lookup.go
├── map.go
├── mono.go
├── named.go
├── object.go
├── objset.go
├── operand.go
├── package.go
├── pointer.go
├── predicates.go
├── range.go
├── recording.go
├── resolver.go
├── return.go
├── scope.go
├── selection.go
├── signature.go
├── sizes.go
├── slice.go
├── stmt.go
├── struct.go
├── subst.go
├── termlist.go
├── trie.go
├── tuple.go
├── type.go
├── typelists.go
├── typeparam.go
├── typeset.go
├── typestring.go
├── typeterm.go
├── typexpr.go
├── under.go
├── unify.go
├── union.go
├── universe.go
├── util.go
├── validtype.go
└── version.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/cmd/compile/internal/types2/alias.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
)

// An Alias represents an alias type.
//
// Alias types are created by alias declarations such as:
//
//	type A = int
//
// The type on the right-hand side of the declaration can be accessed
// using [Alias.Rhs]. This type may itself be an alias.
// Call [Unalias] to obtain the first non-alias type in a chain of
// alias type declarations.
//
// Like a defined ([Named]) type, an alias type has a name.
// Use the [Alias.Obj] method to access its [TypeName] object.
type Alias struct {
	obj     *TypeName      // corresponding declared alias object
	orig    *Alias         // original, uninstantiated alias
	tparams *TypeParamList // type parameters, or nil
	targs   *TypeList      // type arguments, or nil
	fromRHS Type           // RHS of type alias declaration; may be an alias
	actual  Type           // actual (aliased) type; never an alias
}

// NewAlias creates a new Alias type with the given type name and rhs.
// If rhs is nil, the alias is incomplete.
func NewAlias(obj *TypeName, rhs Type) *Alias {
	alias := (*Checker)(nil).newAlias(obj, rhs)
	// Ensure that alias.actual is set (#65455).
	alias.cleanup()
	return alias
}

// Obj returns the type name for the declaration defining the alias type a.
// For instantiated types, this is same as the type name of the origin type.
func (a *Alias) Obj() *TypeName { return a.orig.obj }

func (a *Alias) String() string { return TypeString(a, nil) }

// Underlying returns the [underlying type] of the alias type a, which is the
// underlying type of the aliased type. Underlying types are never Named,
// TypeParam, or Alias types.
//
// [underlying type]: https://go.dev/ref/spec#Underlying_types.
func (a *Alias) Underlying() Type { return unalias(a).Underlying() }

// Origin returns the generic Alias type of which a is an instance.
// If a is not an instance of a generic alias, Origin returns a.
func (a *Alias) Origin() *Alias { return a.orig }

// TypeParams returns the type parameters of the alias type a, or nil.
// A generic Alias and its instances have the same type parameters.
func (a *Alias) TypeParams() *TypeParamList { return a.tparams }

// SetTypeParams sets the type parameters of the alias type a.
// The alias a must not have type arguments.
func (a *Alias) SetTypeParams(tparams []*TypeParam) {
	assert(a.targs == nil)
	a.tparams = bindTParams(tparams)
}

// TypeArgs returns the type arguments used to instantiate the Alias type.
// If a is not an instance of a generic alias, the result is nil.
func (a *Alias) TypeArgs() *TypeList { return a.targs }

// Rhs returns the type R on the right-hand side of an alias
// declaration "type A = R", which may be another alias.
func (a *Alias) Rhs() Type { return a.fromRHS }

// Unalias returns t if it is not an alias type;
// otherwise it follows t's alias chain until it
// reaches a non-alias type which is then returned.
// Consequently, the result is never an alias type.
// Returns nil if the alias is incomplete.
func Unalias(t Type) Type {
	if a0, _ := t.(*Alias); a0 != nil {
		return unalias(a0)
	}
	return t
}

func unalias(a0 *Alias) Type {
	if a0.actual != nil {
		return a0.actual
	}
	var t Type
	for a := a0; a != nil; a, _ = t.(*Alias) {
		t = a.fromRHS
	}
	// It's fine to memoize nil types since it's the zero value for actual.
	// It accomplishes nothing.
	a0.actual = t
	return t
}

// asNamed returns t as *Named if that is t's
// actual type. It returns nil otherwise.
func asNamed(t Type) *Named {
	n, _ := Unalias(t).(*Named)
	return n
}

// newAlias creates a new Alias type with the given type name and rhs.
// If rhs is nil, the alias is incomplete.
func (check *Checker) newAlias(obj *TypeName, rhs Type) *Alias {
	a := new(Alias)
	a.obj = obj
	a.orig = a
	a.fromRHS = rhs
	if obj.typ == nil {
		obj.typ = a
	}

	// Ensure that a.actual is set at the end of type checking.
	if check != nil {
		check.needsCleanup(a)
	}

	return a
}

// newAliasInstance creates a new alias instance for the given origin and type
// arguments, recording pos as the position of its synthetic object (for error
// reporting).
func (check *Checker) newAliasInstance(pos syntax.Pos, orig *Alias, targs []Type, expanding *Named, ctxt *Context) *Alias {
	assert(len(targs) > 0)
	obj := NewTypeName(pos, orig.obj.pkg, orig.obj.name, nil)
	rhs := check.subst(pos, orig.fromRHS, makeSubstMap(orig.TypeParams().list(), targs), expanding, ctxt)
	res := check.newAlias(obj, rhs)
	res.orig = orig
	res.tparams = orig.tparams
	res.targs = newTypeList(targs)
	return res
}

func (a *Alias) cleanup() {
	// Ensure a.actual is set before types are published,
	// so unalias is a pure "getter", not a "setter".
	unalias(a)
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/api.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package types2 declares the data types and implements
// the algorithms for type-checking of Go packages. Use
// Config.Check to invoke the type checker for a package.
// Alternatively, create a new type checker with NewChecker
// and invoke it incrementally by calling Checker.Files.
//
// Type-checking consists of several interdependent phases:
//
// Name resolution maps each identifier (syntax.Name) in the program to the
// language object (Object) it denotes.
// Use Info.{Defs,Uses,Implicits} for the results of name resolution.
//
// Constant folding computes the exact constant value (constant.Value)
// for every expression (syntax.Expr) that is a compile-time constant.
// Use Info.Types[expr].Value for the results of constant folding.
//
// Type inference computes the type (Type) of every expression (syntax.Expr)
// and checks for compliance with the language specification.
// Use Info.Types[expr].Type for the results of type inference.
package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	. "internal/types/errors"
	"strings"
)

// An Error describes a type-checking error; it implements the error interface.
// A "soft" error is an error that still permits a valid interpretation of a
// package (such as "unused variable"); "hard" errors may lead to unpredictable
// behavior if ignored.
type Error struct {
	Pos  syntax.Pos // error position
	Msg  string     // default error message, user-friendly
	Full string     // full error message, for debugging (may contain internal details)
	Soft bool       // if set, error is "soft"
	Code Code       // error code
}

// Error returns an error string formatted as follows:
// filename:line:column: message
func (err Error) Error() string {
	return fmt.Sprintf("%s: %s", err.Pos, err.Msg)
}

// FullError returns an error string like Error, buy it may contain
// type-checker internal details such as subscript indices for type
// parameters and more. Useful for debugging.
func (err Error) FullError() string {
	return fmt.Sprintf("%s: %s", err.Pos, err.Full)
}

// An ArgumentError holds an error associated with an argument index.
type ArgumentError struct {
	Index int
	Err   error
}

func (e *ArgumentError) Error() string { return e.Err.Error() }
func (e *ArgumentError) Unwrap() error { return e.Err }

// An Importer resolves import paths to Packages.
//
// CAUTION: This interface does not support the import of locally
// vendored packages. See https://golang.org/s/go15vendor.
// If possible, external implementations should implement ImporterFrom.
type Importer interface {
	// Import returns the imported package for the given import path.
	// The semantics is like for ImporterFrom.ImportFrom except that
	// dir and mode are ignored (since they are not present).
	Import(path string) (*Package, error)
}

// ImportMode is reserved for future use.
type ImportMode int

// An ImporterFrom resolves import paths to packages; it
// supports vendoring per https://golang.org/s/go15vendor.
// Use go/importer to obtain an ImporterFrom implementation.
type ImporterFrom interface {
	// Importer is present for backward-compatibility. Calling
	// Import(path) is the same as calling ImportFrom(path, "", 0);
	// i.e., locally vendored packages may not be found.
	// The types package does not call Import if an ImporterFrom
	// is present.
	Importer

	// ImportFrom returns the imported package for the given import
	// path when imported by a package file located in dir.
	// If the import failed, besides returning an error, ImportFrom
	// is encouraged to cache and return a package anyway, if one
	// was created. This will reduce package inconsistencies and
	// follow-on type checker errors due to the missing package.
	// The mode value must be 0; it is reserved for future use.
	// Two calls to ImportFrom with the same path and dir must
	// return the same package.
	ImportFrom(path, dir string, mode ImportMode) (*Package, error)
}

// A Config specifies the configuration for type checking.
// The zero value for Config is a ready-to-use default configuration.
type Config struct {
	// Context is the context used for resolving global identifiers. If nil, the
	// type checker will initialize this field with a newly created context.
	Context *Context

	// GoVersion describes the accepted Go language version. The string must
	// start with a prefix of the form "go%d.%d" (e.g. "go1.20", "go1.21rc1", or
	// "go1.21.0") or it must be empty; an empty string disables Go language
	// version checks. If the format is invalid, invoking the type checker will
	// result in an error.
	GoVersion string

	// If IgnoreFuncBodies is set, function bodies are not
	// type-checked.
	IgnoreFuncBodies bool

	// If FakeImportC is set, `import "C"` (for packages requiring Cgo)
	// declares an empty "C" package and errors are omitted for qualified
	// identifiers referring to package C (which won't find an object).
	// This feature is intended for the standard library cmd/api tool.
	//
	// Caution: Effects may be unpredictable due to follow-on errors.
	//          Do not use casually!
	FakeImportC bool

	// If IgnoreBranchErrors is set, branch/label errors are ignored.
	IgnoreBranchErrors bool

	// If go115UsesCgo is set, the type checker expects the
	// _cgo_gotypes.go file generated by running cmd/cgo to be
	// provided as a package source file. Qualified identifiers
	// referring to package C will be resolved to cgo-provided
	// declarations within _cgo_gotypes.go.
	//
	// It is an error to set both FakeImportC and go115UsesCgo.
	go115UsesCgo bool

	// If Trace is set, a debug trace is printed to stdout.
	Trace bool

	// If Error != nil, it is called with each error found
	// during type checking; err has dynamic type Error.
	// Secondary errors (for instance, to enumerate all types
	// involved in an invalid recursive type declaration) have
	// error strings that start with a '\t' character.
	// If Error == nil, type-checking stops with the first
	// error found.
	Error func(err error)

	// An importer is used to import packages referred to from
	// import declarations.
	// If the installed importer implements ImporterFrom, the type
	// checker calls ImportFrom instead of Import.
	// The type checker reports an error if an importer is needed
	// but none was installed.
	Importer Importer

	// If Sizes != nil, it provides the sizing functions for package unsafe.
	// Otherwise SizesFor("gc", "amd64") is used instead.
	Sizes Sizes

	// If DisableUnusedImportCheck is set, packages are not checked
	// for unused imports.
	DisableUnusedImportCheck bool

	// If a non-empty ErrorURL format string is provided, it is used
	// to format an error URL link that is appended to the first line
	// of an error message. ErrorURL must be a format string containing
	// exactly one "%s" format, e.g. "[go.dev/e/%s]".
	ErrorURL string
}

// Info holds result type information for a type-checked package.
// Only the information for which a map is provided is collected.
// If the package has type errors, the collected information may
// be incomplete.
type Info struct {
	// Types maps expressions to their types, and for constant
	// expressions, also their values. Invalid expressions are
	// omitted.
	//
	// For (possibly parenthesized) identifiers denoting built-in
	// functions, the recorded signatures are call-site specific:
	// if the call result is not a constant, the recorded type is
	// an argument-specific signature. Otherwise, the recorded type
	// is invalid.
	//
	// The Types map does not record the type of every identifier,
	// only those that appear where an arbitrary expression is
	// permitted. For instance:
	// - an identifier f in a selector expression x.f is found
	//   only in the Selections map;
	// - an identifier z in a variable declaration 'var z int'
	//   is found only in the Defs map;
	// - an identifier p denoting a package in a qualified
	//   identifier p.X is found only in the Uses map.
	//
	// Similarly, no type is recorded for the (synthetic) FuncType
	// node in a FuncDecl.Type field, since there is no corresponding
	// syntactic function type expression in the source in this case
	// Instead, the function type is found in the Defs.map entry for
	// the corresponding function declaration.
	Types map[syntax.Expr]TypeAndValue

	// If StoreTypesInSyntax is set, type information identical to
	// that which would be put in the Types map, will be set in
	// syntax.Expr.TypeAndValue (independently of whether Types
	// is nil or not).
	StoreTypesInSyntax bool

	// Instances maps identifiers denoting generic types or functions to their
	// type arguments and instantiated type.
	//
	// For example, Instances will map the identifier for 'T' in the type
	// instantiation T[int, string] to the type arguments [int, string] and
	// resulting instantiated *Named type. Given a generic function
	// func F[A any](A), Instances will map the identifier for 'F' in the call
	// expression F(int(1)) to the inferred type arguments [int], and resulting
	// instantiated *Signature.
	//
	// Invariant: Instantiating Uses[id].Type() with Instances[id].TypeArgs
	// results in an equivalent of Instances[id].Type.
	Instances map[*syntax.Name]Instance

	// Defs maps identifiers to the objects they define (including
	// package names, dots "." of dot-imports, and blank "_" identifiers).
	// For identifiers that do not denote objects (e.g., the package name
	// in package clauses, or symbolic variables t in t := x.(type) of
	// type switch headers), the corresponding objects are nil.
	//
	// For an embedded field, Defs returns the field *Var it defines.
	//
	// Invariant: Defs[id] == nil || Defs[id].Pos() == id.Pos()
	Defs map[*syntax.Name]Object

	// Uses maps identifiers to the objects they denote.
	//
	// For an embedded field, Uses returns the *TypeName it denotes.
	//
	// Invariant: Uses[id].Pos() != id.Pos()
	Uses map[*syntax.Name]Object

	// Implicits maps nodes to their implicitly declared objects, if any.
	// The following node and object types may appear:
	//
	//     node               declared object
	//
	//     *syntax.ImportDecl    *PkgName for imports without renames
	//     *syntax.CaseClause    type-specific *Var for each type switch case clause (incl. default)
	//     *syntax.Field         anonymous parameter *Var (incl. unnamed results)
	//
	Implicits map[syntax.Node]Object

	// Selections maps selector expressions (excluding qualified identifiers)
	// to their corresponding selections.
	Selections map[*syntax.SelectorExpr]*Selection

	// Scopes maps syntax.Nodes to the scopes they define. Package scopes are not
	// associated with a specific node but with all files belonging to a package.
	// Thus, the package scope can be found in the type-checked Package object.
	// Scopes nest, with the Universe scope being the outermost scope, enclosing
	// the package scope, which contains (one or more) files scopes, which enclose
	// function scopes which in turn enclose statement and function literal scopes.
	// Note that even though package-level functions are declared in the package
	// scope, the function scopes are embedded in the file scope of the file
	// containing the function declaration.
	//
	// The Scope of a function contains the declarations of any
	// type parameters, parameters, and named results, plus any
	// local declarations in the body block.
	// It is coextensive with the complete extent of the
	// function's syntax ([*ast.FuncDecl] or [*ast.FuncLit]).
	// The Scopes mapping does not contain an entry for the
	// function body ([*ast.BlockStmt]); the function's scope is
	// associated with the [*ast.FuncType].
	//
	// The following node types may appear in Scopes:
	//
	//     *syntax.File
	//     *syntax.FuncType
	//     *syntax.TypeDecl
	//     *syntax.BlockStmt
	//     *syntax.IfStmt
	//     *syntax.SwitchStmt
	//     *syntax.CaseClause
	//     *syntax.CommClause
	//     *syntax.ForStmt
	//
	Scopes map[syntax.Node]*Scope

	// InitOrder is the list of package-level initializers in the order in which
	// they must be executed. Initializers referring to variables related by an
	// initialization dependency appear in topological order, the others appear
	// in source order. Variables without an initialization expression do not
	// appear in this list.
	InitOrder []*Initializer

	// FileVersions maps a file to its Go version string.
	// If the file doesn't specify a version, the reported
	// string is Config.GoVersion.
	// Version strings begin with “go”, like “go1.21”, and
	// are suitable for use with the [go/version] package.
	FileVersions map[*syntax.PosBase]string
}

func (info *Info) recordTypes() bool {
	return info.Types != nil || info.StoreTypesInSyntax
}

// TypeOf returns the type of expression e, or nil if not found.
// Precondition 1: the Types map is populated or StoreTypesInSyntax is set.
// Precondition 2: Uses and Defs maps are populated.
func (info *Info) TypeOf(e syntax.Expr) Type {
	if info.Types != nil {
		if t, ok := info.Types[e]; ok {
			return t.Type
		}
	} else if info.StoreTypesInSyntax {
		if tv := e.GetTypeInfo(); tv.Type != nil {
			return tv.Type
		}
	}

	if id, _ := e.(*syntax.Name); id != nil {
		if obj := info.ObjectOf(id); obj != nil {
			return obj.Type()
		}
	}
	return nil
}

// ObjectOf returns the object denoted by the specified id,
// or nil if not found.
//
// If id is an embedded struct field, ObjectOf returns the field (*Var)
// it defines, not the type (*TypeName) it uses.
//
// Precondition: the Uses and Defs maps are populated.
func (info *Info) ObjectOf(id *syntax.Name) Object {
	if obj := info.Defs[id]; obj != nil {
		return obj
	}
	return info.Uses[id]
}

// PkgNameOf returns the local package name defined by the import,
// or nil if not found.
//
// For dot-imports, the package name is ".".
//
// Precondition: the Defs and Implicts maps are populated.
func (info *Info) PkgNameOf(imp *syntax.ImportDecl) *PkgName {
	var obj Object
	if imp.LocalPkgName != nil {
		obj = info.Defs[imp.LocalPkgName]
	} else {
		obj = info.Implicits[imp]
	}
	pkgname, _ := obj.(*PkgName)
	return pkgname
}

// TypeAndValue reports the type and value (for constants)
// of the corresponding expression.
type TypeAndValue struct {
	mode  operandMode
	Type  Type
	Value constant.Value
}

// IsVoid reports whether the corresponding expression
// is a function call without results.
func (tv TypeAndValue) IsVoid() bool {
	return tv.mode == novalue
}

// IsType reports whether the corresponding expression specifies a type.
func (tv TypeAndValue) IsType() bool {
	return tv.mode == typexpr
}

// IsBuiltin reports whether the corresponding expression denotes
// a (possibly parenthesized) built-in function.
func (tv TypeAndValue) IsBuiltin() bool {
	return tv.mode == builtin
}

// IsValue reports whether the corresponding expression is a value.
// Builtins are not considered values. Constant values have a non-
// nil Value.
func (tv TypeAndValue) IsValue() bool {
	switch tv.mode {
	case constant_, variable, mapindex, value, nilvalue, commaok, commaerr:
		return true
	}
	return false
}

// IsNil reports whether the corresponding expression denotes the
// predeclared value nil. Depending on context, it may have been
// given a type different from UntypedNil.
func (tv TypeAndValue) IsNil() bool {
	return tv.mode == nilvalue
}

// Addressable reports whether the corresponding expression
// is addressable (https://golang.org/ref/spec#Address_operators).
func (tv TypeAndValue) Addressable() bool {
	return tv.mode == variable
}

// Assignable reports whether the corresponding expression
// is assignable to (provided a value of the right type).
func (tv TypeAndValue) Assignable() bool {
	return tv.mode == variable || tv.mode == mapindex
}

// HasOk reports whether the corresponding expression may be
// used on the rhs of a comma-ok assignment.
func (tv TypeAndValue) HasOk() bool {
	return tv.mode == commaok || tv.mode == mapindex
}

// Instance reports the type arguments and instantiated type for type and
// function instantiations. For type instantiations, Type will be of dynamic
// type *Named. For function instantiations, Type will be of dynamic type
// *Signature.
type Instance struct {
	TypeArgs *TypeList
	Type     Type
}

func (inst Instance) String() string {
	return fmt.Sprintf("%s%s", inst.TypeArgs, inst.Type)
}

// An Initializer describes a package-level variable, or a list of variables in case
// of a multi-valued initialization expression, and the corresponding initialization
// expression.
type Initializer struct {
	Lhs []*Var // var Lhs = Rhs
	Rhs syntax.Expr
}

func (init *Initializer) String() string {
	var buf strings.Builder
	for i, lhs := range init.Lhs {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(lhs.Name())
	}
	buf.WriteString(" = ")
	syntax.Fprint(&buf, init.Rhs, syntax.ShortForm)
	return buf.String()
}

// Check type-checks a package and returns the resulting package object and
// the first error if any. Additionally, if info != nil, Check populates each
// of the non-nil maps in the Info struct.
//
// The package is marked as complete if no errors occurred, otherwise it is
// incomplete. See Config.Error for controlling behavior in the presence of
// errors.
//
// The package is specified by a list of *syntax.Files and corresponding
// file set, and the package path the package is identified with.
// The clean path must not be empty or dot (".").
func (conf *Config) Check(path string, files []*syntax.File, info *Info) (*Package, error) {
	pkg := NewPackage(path, "")
	return pkg, NewChecker(conf, pkg, info).Files(files)
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/api_predicates.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements exported type predicates.

package types2

// AssertableTo reports whether a value of type V can be asserted to have type T.
//
// The behavior of AssertableTo is unspecified in three cases:
//   - if T is Typ[Invalid]
//   - if V is a generalized interface; i.e., an interface that may only be used
//     as a type constraint in Go code
//   - if T is an uninstantiated generic type
func AssertableTo(V *Interface, T Type) bool {
	// Checker.newAssertableTo suppresses errors for invalid types, so we need special
	// handling here.
	if !isValid(T.Underlying()) {
		return false
	}
	return (*Checker)(nil).newAssertableTo(V, T, nil)
}

// AssignableTo reports whether a value of type V is assignable to a variable
// of type T.
//
// The behavior of AssignableTo is unspecified if V or T is Typ[Invalid] or an
// uninstantiated generic type.
func AssignableTo(V, T Type) bool {
	x := operand{mode_: value, typ_: V}
	ok, _ := x.assignableTo(nil, T, nil) // check not needed for non-constant x
	return ok
}

// ConvertibleTo reports whether a value of type V is convertible to a value of
// type T.
//
// The behavior of ConvertibleTo is unspecified if V or T is Typ[Invalid] or an
// uninstantiated generic type.
func ConvertibleTo(V, T Type) bool {
	x := operand{mode_: value, typ_: V}
	return x.convertibleTo(nil, T, nil) // check not needed for non-constant x
}

// Implements reports whether type V implements interface T.
//
// The behavior of Implements is unspecified if V is Typ[Invalid] or an uninstantiated
// generic type.
func Implements(V Type, T *Interface) bool {
	if T.Empty() {
		// All types (even Typ[Invalid]) implement the empty interface.
		return true
	}
	// Checker.implements suppresses errors for invalid types, so we need special
	// handling here.
	if !isValid(V.Underlying()) {
		return false
	}
	return (*Checker)(nil).implements(V, T, false, nil)
}

// Satisfies reports whether type V satisfies the constraint T.
//
// The behavior of Satisfies is unspecified if V is Typ[Invalid] or an uninstantiated
// generic type.
func Satisfies(V Type, T *Interface) bool {
	return (*Checker)(nil).implements(V, T, true, nil)
}

// Identical reports whether x and y are identical types.
// Receivers of [Signature] types are ignored.
//
// Predicates such as [Identical], [Implements], and
// [Satisfies] assume that both operands belong to a
// consistent collection of symbols ([Object] values).
// For example, two [Named] types can be identical only if their
// [Named.Obj] methods return the same [TypeName] symbol.
// A collection of symbols is consistent if, for each logical
// package whose path is P, the creation of those symbols
// involved at most one call to [NewPackage](P, ...).
// To ensure consistency, use a single [Importer] for
// all loaded packages and their dependencies.
// For more information, see https://github.com/golang/go/issues/57497.
func Identical(x, y Type) bool {
	var c comparer
	return c.identical(x, y, nil)
}

// IdenticalIgnoreTags reports whether x and y are identical types if tags are ignored.
// Receivers of [Signature] types are ignored.
func IdenticalIgnoreTags(x, y Type) bool {
	var c comparer
	c.ignoreTags = true
	return c.identical(x, y, nil)
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/array.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

// An Array represents an array type.
type Array struct {
	len  int64
	elem Type
}

// NewArray returns a new array type for the given element type and length.
// A negative length indicates an unknown length.
func NewArray(elem Type, len int64) *Array { return &Array{len: len, elem: elem} }

// Len returns the length of array a.
// A negative result indicates an unknown length.
func (a *Array) Len() int64 { return a.len }

// Elem returns element type of array a.
func (a *Array) Elem() Type { return a.elem }

func (a *Array) Underlying() Type { return a }
func (a *Array) String() string   { return TypeString(a, nil) }

```

// === FILE: references!/go/src/cmd/compile/internal/types2/assignments.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements initialization and assignment checks.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	. "internal/types/errors"
	"strings"
)

// assignment reports whether x can be assigned to a variable of type T,
// if necessary by attempting to convert untyped values to the appropriate
// type. context describes the context in which the assignment takes place.
// Use T == nil to indicate assignment to an untyped blank identifier.
// If the assignment check fails, x.mode is set to invalid.
func (check *Checker) assignment(x *operand, T Type, context string) {
	check.singleValue(x)

	switch x.mode() {
	case invalid:
		return // error reported before
	case nilvalue:
		assert(isTypes2)
		// ok
	case constant_, variable, mapindex, value, commaok, commaerr:
		// ok
	default:
		// we may get here because of other problems (go.dev/issue/39634, crash 12)
		// TODO(gri) do we need a new "generic" error code here?
		check.errorf(x, IncompatibleAssign, "cannot assign %s to %s in %s", x, T, context)
		x.invalidate()
		return
	}

	if isUntyped(x.typ()) {
		target := T
		// spec: "If an untyped constant is assigned to a variable of interface
		// type or the blank identifier, the constant is first converted to type
		// bool, rune, int, float64, complex128 or string respectively, depending
		// on whether the value is a boolean, rune, integer, floating-point,
		// complex, or string constant."
		if isTypes2 {
			if x.isNil() {
				if T == nil {
					check.errorf(x, UntypedNilUse, "use of untyped nil in %s", context)
					x.invalidate()
					return
				}
			} else if T == nil || isNonTypeParamInterface(T) {
				target = Default(x.typ())
			}
		} else { // go/types
			if T == nil || isNonTypeParamInterface(T) {
				if T == nil && x.typ() == Typ[UntypedNil] {
					check.errorf(x, UntypedNilUse, "use of untyped nil in %s", context)
					x.invalidate()
					return
				}
				target = Default(x.typ())
			}
		}
		newType, val, code := check.implicitTypeAndValue(x, target)
		if code != 0 {
			msg := check.sprintf("cannot use %s as %s value in %s", x, target, context)
			switch code {
			case TruncatedFloat:
				msg += " (truncated)"
			case NumericOverflow:
				msg += " (overflows)"
			default:
				code = IncompatibleAssign
			}
			check.error(x, code, msg)
			x.invalidate()
			return
		}
		if val != nil {
			x.val = val
			check.updateExprVal(x.expr, val)
		}
		if newType != x.typ() {
			x.typ_ = newType
			check.updateExprType(x.expr, newType, false)
		}
	}
	// x.typ is typed

	// A generic (non-instantiated) function value cannot be assigned to a variable.
	check.nonGeneric(newTarget(T, context), x)
	if !x.isValid() {
		return
	}

	// spec: "If a left-hand side is the blank identifier, any typed or
	// non-constant value except for the predeclared identifier nil may
	// be assigned to it."
	if T == nil {
		return
	}

	cause := ""
	if ok, code := x.assignableTo(check, T, &cause); !ok {
		if cause != "" {
			check.errorf(x, code, "cannot use %s as %s value in %s: %s", x, T, context, cause)
		} else {
			check.errorf(x, code, "cannot use %s as %s value in %s", x, T, context)
		}
		x.invalidate()
	}
}

func (check *Checker) initConst(lhs *Const, x *operand) {
	if !x.isValid() || !isValid(x.typ()) || !isValid(lhs.typ) {
		if lhs.typ == nil {
			lhs.typ = Typ[Invalid]
		}
		return
	}

	// rhs must be a constant
	if x.mode() != constant_ {
		check.errorf(x, InvalidConstInit, "%s is not constant", x)
		if lhs.typ == nil {
			lhs.typ = Typ[Invalid]
		}
		return
	}
	assert(isConstType(x.typ()))

	// If the lhs doesn't have a type yet, use the type of x.
	if lhs.typ == nil {
		lhs.typ = x.typ()
	}

	check.assignment(x, lhs.typ, "constant declaration")
	if !x.isValid() {
		return
	}

	lhs.val = x.val
}

// initVar checks the initialization lhs = x in a variable declaration.
// If lhs doesn't have a type yet, it is given the type of x,
// or Typ[Invalid] in case of an error.
// If the initialization check fails, x.mode is set to invalid.
func (check *Checker) initVar(lhs *Var, x *operand, context string) {
	if !x.isValid() || !isValid(x.typ()) || !isValid(lhs.typ) {
		if lhs.typ == nil {
			lhs.typ = Typ[Invalid]
		}
		x.invalidate()
		return
	}

	// If lhs doesn't have a type yet, use the type of x.
	if lhs.typ == nil {
		typ := x.typ()
		if isUntyped(typ) {
			// convert untyped types to default types
			if typ == Typ[UntypedNil] {
				check.errorf(x, UntypedNilUse, "use of untyped nil in %s", context)
				lhs.typ = Typ[Invalid]
				x.invalidate()
				return
			}
			typ = Default(typ)
		}
		lhs.typ = typ
	}

	check.assignment(x, lhs.typ, context)
}

// lhsVar checks a lhs variable in an assignment and returns its type.
// lhsVar takes care of not counting a lhs identifier as a "use" of
// that identifier. The result is nil if it is the blank identifier,
// and Typ[Invalid] if it is an invalid lhs expression.
func (check *Checker) lhsVar(lhs syntax.Expr) Type {
	// Determine if the lhs is a (possibly parenthesized) identifier.
	ident, _ := syntax.Unparen(lhs).(*syntax.Name)

	// Don't evaluate lhs if it is the blank identifier.
	if ident != nil && ident.Value == "_" {
		check.recordDef(ident, nil)
		return nil
	}

	// If the lhs is an identifier denoting a variable v, this reference
	// is not a 'use' of v. Remember current value of v.used and restore
	// after evaluating the lhs via check.expr.
	var v *Var
	var v_used bool
	if ident != nil {
		if obj := check.lookup(ident.Value); obj != nil {
			// It's ok to mark non-local variables, but ignore variables
			// from other packages to avoid potential race conditions with
			// dot-imported variables.
			if w, _ := obj.(*Var); w != nil && w.pkg == check.pkg {
				v = w
				v_used = check.usedVars[v]
			}
		}
	}

	var x operand
	check.expr(nil, &x, lhs)

	if v != nil {
		check.usedVars[v] = v_used // restore v.used
	}

	if !x.isValid() || !isValid(x.typ()) {
		return Typ[Invalid]
	}

	// spec: "Each left-hand side operand must be addressable, a map index
	// expression, or the blank identifier. Operands may be parenthesized."
	switch x.mode() {
	case invalid:
		return Typ[Invalid]
	case variable, mapindex:
		// ok
	default:
		if sel, ok := x.expr.(*syntax.SelectorExpr); ok {
			var op operand
			check.expr(nil, &op, sel.X)
			if op.mode() == mapindex {
				check.errorf(&x, UnaddressableFieldAssign, "cannot assign to struct field %s in map", ExprString(x.expr))
				return Typ[Invalid]
			}
		}
		check.errorf(&x, UnassignableOperand, "cannot assign to %s (neither addressable nor a map index expression)", x.expr)
		return Typ[Invalid]
	}

	return x.typ()
}

// assignVar checks the assignment lhs = rhs (if x == nil), or lhs = x (if x != nil).
// If x != nil, it must be the evaluation of rhs (and rhs will be ignored).
// If the assignment check fails and x != nil, x.mode is set to invalid.
func (check *Checker) assignVar(lhs, rhs syntax.Expr, x *operand, context string) {
	T := check.lhsVar(lhs) // nil if lhs is _
	if !isValid(T) {
		if x != nil {
			x.invalidate()
		} else {
			check.use(rhs)
		}
		return
	}

	if x == nil {
		var target *target
		// avoid calling ExprString if not needed
		if T != nil {
			if _, ok := T.Underlying().(*Signature); ok {
				target = newTarget(T, ExprString(lhs))
			}
		}
		x = new(operand)
		check.expr(target, x, rhs)
	}

	if T == nil && context == "assignment" {
		context = "assignment to _ identifier"
	}
	check.assignment(x, T, context)
}

// operandTypes returns the list of types for the given operands.
func operandTypes(list []*operand) (res []Type) {
	for _, x := range list {
		res = append(res, x.typ())
	}
	return res
}

// varTypes returns the list of types for the given variables.
func varTypes(list []*Var) (res []Type) {
	for _, x := range list {
		res = append(res, x.typ)
	}
	return res
}

// typesSummary returns a string of the form "(t1, t2, ...)" where the
// ti's are user-friendly string representations for the given types.
// If variadic is set and the last type is a slice, its string is of
// the form "...E" where E is the slice's element type.
// If hasDots is set, the last argument string is of the form "T..."
// where T is the last type.
// Only one of variadic and hasDots may be set.
func (check *Checker) typesSummary(list []Type, variadic, hasDots bool) string {
	assert(!(variadic && hasDots))
	var res []string
	for i, t := range list {
		var s string
		switch {
		case t == nil:
			fallthrough // should not happen but be cautious
		case !isValid(t):
			s = "unknown type"
		case isUntyped(t): // => *Basic
			if isNumeric(t) {
				// Do not imply a specific type requirement:
				// "have number, want float64" is better than
				// "have untyped int, want float64" or
				// "have int, want float64".
				s = "number"
			} else {
				// If we don't have a number, omit the "untyped" qualifier
				// for compactness.
				s = strings.ReplaceAll(t.(*Basic).name, "untyped ", "")
			}
		default:
			s = check.sprintf("%s", t)
		}
		// handle ... parameters/arguments
		if i == len(list)-1 {
			switch {
			case variadic:
				// In correct code, the parameter type is a slice, but be careful.
				if t, _ := t.(*Slice); t != nil {
					s = check.sprintf("%s", t.elem)
				}
				s = "..." + s
			case hasDots:
				s += "..."
			}
		}
		res = append(res, s)
	}
	return "(" + strings.Join(res, ", ") + ")"
}

func measure(x int, unit string) string {
	if x != 1 {
		unit += "s"
	}
	return fmt.Sprintf("%d %s", x, unit)
}

func (check *Checker) assignError(rhs []syntax.Expr, l, r int) {
	vars := measure(l, "variable")
	vals := measure(r, "value")
	rhs0 := rhs[0]

	if len(rhs) == 1 {
		if call, _ := syntax.Unparen(rhs0).(*syntax.CallExpr); call != nil {
			check.errorf(rhs0, WrongAssignCount, "assignment mismatch: %s but %s returns %s", vars, call.Fun, vals)
			return
		}
	}
	check.errorf(rhs0, WrongAssignCount, "assignment mismatch: %s but %s", vars, vals)
}

func (check *Checker) returnError(at poser, lhs []*Var, rhs []*operand) {
	l, r := len(lhs), len(rhs)
	qualifier := "not enough"
	if r > l {
		at = rhs[l] // report at first extra value
		qualifier = "too many"
	} else if r > 0 {
		at = rhs[r-1] // report at last value
	}
	err := check.newError(WrongResultCount)
	err.addf(at, "%s return values", qualifier)
	err.addf(nopos, "have %s", check.typesSummary(operandTypes(rhs), false, false))
	err.addf(nopos, "want %s", check.typesSummary(varTypes(lhs), false, false))
	err.report()
}

// initVars type-checks assignments of initialization expressions orig_rhs
// to variables lhs.
// If returnStmt is non-nil, initVars type-checks the implicit assignment
// of result expressions orig_rhs to function result parameters lhs.
func (check *Checker) initVars(lhs []*Var, orig_rhs []syntax.Expr, returnStmt syntax.Stmt) {
	l, r := len(lhs), len(orig_rhs)

	context := "assignment"
	if returnStmt != nil {
		context = "return statement"
	} else if l > 1 {
		context = "multiple assignment"
	}

	// If l == 1 and the rhs is a single call, for a better
	// error message don't handle it as n:n mapping below.
	isCall := false
	if r == 1 {
		_, isCall = syntax.Unparen(orig_rhs[0]).(*syntax.CallExpr)
	}

	// If we have a n:n mapping from lhs variable to rhs expression,
	// each value can be assigned to its corresponding variable.
	if l == r && !isCall {
		var x operand
		for i, lhs := range lhs {
			desc := lhs.name
			if returnStmt != nil && desc == "" {
				desc = "result variable"
			}
			check.expr(newTarget(lhs.typ, desc), &x, orig_rhs[i])
			check.initVar(lhs, &x, context)
		}
		return
	}

	// If we don't have an n:n mapping, the rhs must be a single expression
	// resulting in 2 or more values; otherwise we have an assignment mismatch.
	if r != 1 {
		// Only report a mismatch error if there are no other errors on the rhs.
		if check.use(orig_rhs...) {
			if returnStmt != nil {
				rhs := check.exprList(orig_rhs)
				check.returnError(returnStmt, lhs, rhs)
			} else {
				check.assignError(orig_rhs, l, r)
			}
		}
		// ensure that LHS variables have a type
		for _, v := range lhs {
			if v.typ == nil {
				v.typ = Typ[Invalid]
			}
		}
		return
	}

	rhs, commaOk := check.multiExpr(orig_rhs[0], l == 2 && returnStmt == nil)
	r = len(rhs)
	if l == r {
		for i, lhs := range lhs {
			check.initVar(lhs, rhs[i], context)
		}
		// Only record comma-ok expression if both initializations succeeded
		// (go.dev/issue/59371).
		if commaOk && rhs[0].mode() != invalid && rhs[1].mode() != invalid {
			check.recordCommaOkTypes(orig_rhs[0], rhs)
		}
		return
	}

	// In all other cases we have an assignment mismatch.
	// Only report a mismatch error if there are no other errors on the rhs.
	if rhs[0].mode() != invalid {
		if returnStmt != nil {
			check.returnError(returnStmt, lhs, rhs)
		} else {
			check.assignError(orig_rhs, l, r)
		}
	}
	// ensure that LHS variables have a type
	for _, v := range lhs {
		if v.typ == nil {
			v.typ = Typ[Invalid]
		}
	}
	// orig_rhs[0] was already evaluated
}

// assignVars type-checks assignments of expressions orig_rhs to variables lhs.
func (check *Checker) assignVars(lhs, orig_rhs []syntax.Expr) {
	l, r := len(lhs), len(orig_rhs)

	context := "assignment"
	if l > 1 {
		context = "multiple assignment"
	}

	// If l == 1 and the rhs is a single call, for a better
	// error message don't handle it as n:n mapping below.
	isCall := false
	if r == 1 {
		_, isCall = syntax.Unparen(orig_rhs[0]).(*syntax.CallExpr)
	}

	// If we have a n:n mapping from lhs variable to rhs expression,
	// each value can be assigned to its corresponding variable.
	if l == r && !isCall {
		for i, lhs := range lhs {
			check.assignVar(lhs, orig_rhs[i], nil, context)
		}
		return
	}

	// If we don't have an n:n mapping, the rhs must be a single expression
	// resulting in 2 or more values; otherwise we have an assignment mismatch.
	if r != 1 {
		// Only report a mismatch error if there are no other errors on the lhs or rhs.
		okLHS := check.useLHS(lhs...)
		okRHS := check.use(orig_rhs...)
		if okLHS && okRHS {
			check.assignError(orig_rhs, l, r)
		}
		return
	}

	rhs, commaOk := check.multiExpr(orig_rhs[0], l == 2)
	r = len(rhs)
	if l == r {
		for i, lhs := range lhs {
			check.assignVar(lhs, nil, rhs[i], context)
		}
		// Only record comma-ok expression if both assignments succeeded
		// (go.dev/issue/59371).
		if commaOk && rhs[0].mode() != invalid && rhs[1].mode() != invalid {
			check.recordCommaOkTypes(orig_rhs[0], rhs)
		}
		return
	}

	// In all other cases we have an assignment mismatch.
	// Only report a mismatch error if there are no other errors on the rhs.
	if rhs[0].mode() != invalid {
		check.assignError(orig_rhs, l, r)
	}
	check.useLHS(lhs...)
	// orig_rhs[0] was already evaluated
}

func (check *Checker) shortVarDecl(pos poser, lhs, rhs []syntax.Expr) {
	top := len(check.delayed)
	scope := check.scope

	// collect lhs variables
	seen := make(map[string]bool, len(lhs))
	lhsVars := make([]*Var, len(lhs))
	newVars := make([]*Var, 0, len(lhs))
	hasErr := false
	for i, lhs := range lhs {
		ident, _ := lhs.(*syntax.Name)
		if ident == nil {
			check.useLHS(lhs)
			// TODO(gri) This is redundant with a go/parser error. Consider omitting in go/types?
			check.errorf(lhs, BadDecl, "non-name %s on left side of :=", lhs)
			hasErr = true
			continue
		}

		name := ident.Value
		if name != "_" {
			if seen[name] {
				check.errorf(lhs, RepeatedDecl, "%s repeated on left side of :=", lhs)
				hasErr = true
				continue
			}
			seen[name] = true
		}

		// Use the correct obj if the ident is redeclared. The
		// variable's scope starts after the declaration; so we
		// must use Scope.Lookup here and call Scope.Insert
		// (via check.declare) later.
		if alt := scope.Lookup(name); alt != nil {
			check.recordUse(ident, alt)
			// redeclared object must be a variable
			if obj, _ := alt.(*Var); obj != nil {
				lhsVars[i] = obj
			} else {
				check.errorf(lhs, UnassignableOperand, "cannot assign to %s", lhs)
				hasErr = true
			}
			continue
		}

		// declare new variable
		obj := newVar(LocalVar, ident.Pos(), check.pkg, name, nil)
		lhsVars[i] = obj
		if name != "_" {
			newVars = append(newVars, obj)
		}
		check.recordDef(ident, obj)
	}

	// create dummy variables where the lhs is invalid
	for i, obj := range lhsVars {
		if obj == nil {
			lhsVars[i] = newVar(LocalVar, lhs[i].Pos(), check.pkg, "_", nil)
		}
	}

	check.initVars(lhsVars, rhs, nil)

	// process function literals in rhs expressions before scope changes
	check.processDelayed(top)

	if len(newVars) == 0 && !hasErr {
		check.softErrorf(pos, NoNewVar, "no new variables on left side of :=")
		return
	}

	// declare new variables
	// spec: "The scope of a constant or variable identifier declared inside
	// a function begins at the end of the ConstSpec or VarSpec (ShortVarDecl
	// for short variable declarations) and ends at the end of the innermost
	// containing block."
	scopePos := endPos(rhs[len(rhs)-1])
	for _, obj := range newVars {
		check.declare(scope, nil, obj, scopePos) // id = nil: recordDef already called
	}
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/basic.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

// BasicKind describes the kind of basic type.
type BasicKind int

const (
	Invalid BasicKind = iota // type is invalid

	// predeclared types
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	Complex64
	Complex128
	String
	UnsafePointer

	// types for untyped values
	UntypedBool
	UntypedInt
	UntypedRune
	UntypedFloat
	UntypedComplex
	UntypedString
	UntypedNil

	// aliases
	Byte = Uint8
	Rune = Int32
)

// BasicInfo is a set of flags describing properties of a basic type.
type BasicInfo int

// Properties of basic types.
const (
	IsBoolean BasicInfo = 1 << iota
	IsInteger
	IsUnsigned
	IsFloat
	IsComplex
	IsString
	IsUntyped

	IsOrdered   = IsInteger | IsFloat | IsString
	IsNumeric   = IsInteger | IsFloat | IsComplex
	IsConstType = IsBoolean | IsNumeric | IsString
)

// A Basic represents a basic type.
type Basic struct {
	kind BasicKind
	info BasicInfo
	name string
}

// Kind returns the kind of basic type b.
func (b *Basic) Kind() BasicKind { return b.kind }

// Info returns information about properties of basic type b.
func (b *Basic) Info() BasicInfo { return b.info }

// Name returns the name of basic type b.
func (b *Basic) Name() string { return b.name }

func (b *Basic) Underlying() Type { return b }
func (b *Basic) String() string   { return TypeString(b, nil) }

```

// === FILE: references!/go/src/cmd/compile/internal/types2/builtins.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of builtin function calls.

package types2

import (
	"cmd/compile/internal/syntax"
	"go/constant"
	"go/token"
	. "internal/types/errors"
)

// builtin type-checks a call to the built-in specified by id and
// reports whether the call is valid, with *x holding the result;
// but x.expr is not set. If the call is invalid, the result is
// false, and *x is undefined.
func (check *Checker) builtin(x *operand, call *syntax.CallExpr, id builtinId) (_ bool) {
	argList := call.ArgList

	// append is the only built-in that permits the use of ... for the last argument
	bin := predeclaredFuncs[id]
	if hasDots(call) && id != _Append {
		check.errorf(dddErrPos(call),
			InvalidDotDotDot,
			invalidOp+"invalid use of ... with built-in %s", bin.name)
		check.use(argList...)
		return
	}

	// For len(x) and cap(x) we need to know if x contains any function calls or
	// receive operations. Save/restore current setting and set hasCallOrRecv to
	// false for the evaluation of x so that we can check it afterwards.
	// Note: We must do this _before_ calling exprList because exprList evaluates
	//       all arguments.
	if id == _Len || id == _Cap {
		defer func(b bool) {
			check.hasCallOrRecv = b
		}(check.hasCallOrRecv)
		check.hasCallOrRecv = false
	}

	// Evaluate arguments for built-ins that use ordinary (value) arguments.
	// For built-ins with special argument handling (make, new, etc.),
	// evaluation is done by the respective built-in code.
	var args []*operand // not valid for _Make, _New, _Offsetof, _Trace
	var nargs int
	switch id {
	default:
		// check all arguments
		args = check.exprList(argList)
		nargs = len(args)
		for _, a := range args {
			if !a.isValid() {
				return
			}
		}
		// first argument is always in x
		if nargs > 0 {
			*x = *args[0]
		}
	case _Make, _New, _Offsetof, _Trace:
		// arguments require special handling
		nargs = len(argList)
	}

	// check argument count
	{
		msg := ""
		if nargs < bin.nargs {
			msg = "not enough"
		} else if !bin.variadic && nargs > bin.nargs {
			msg = "too many"
		}
		if msg != "" {
			check.errorf(argErrPos(call), WrongArgCount, invalidOp+"%s arguments for %v (expected %d, found %d)", msg, call, bin.nargs, nargs)
			return
		}
	}

	switch id {
	case _Append:
		// append(s S, x ...E) S, where E is the element type of S
		// spec: "The variadic function append appends zero or more values x to
		// a slice s of type S and returns the resulting slice, also of type S.
		// The values x are passed to a parameter of type ...E where E is the
		// element type of S and the respective parameter passing rules apply.
		// As a special case, append also accepts a first argument assignable
		// to type []byte with a second argument of string type followed by ... .
		// This form appends the bytes of the string."

		// In either case, the first argument must be a slice; in particular it
		// cannot be the predeclared nil value. Note that nil is not excluded by
		// the assignability requirement alone for the special case (go.dev/issue/76220).
		// spec: "If S is a type parameter, all types in its type set
		// must have the same underlying slice type []E."
		E, err := sliceElem(x)
		if err != nil {
			check.errorf(x, InvalidAppend, "invalid append: %s", err.format(check))
			return
		}

		// Handle append(bytes, y...) special case, where
		// the type set of y is {string} or {string, []byte}.
		var sig *Signature
		if nargs == 2 && hasDots(call) {
			if ok, _ := x.assignableTo(check, NewSlice(universeByte), nil); ok {
				y := args[1]
				hasString := false
				for _, u := range typeset(y.typ()) {
					if s, _ := u.(*Slice); s != nil && Identical(s.elem, universeByte) {
						// typeset ⊇ {[]byte}
					} else if u != nil && isString(u) {
						// typeset ⊇ {string}
						hasString = true
					} else {
						y = nil
						break
					}
				}
				if y != nil && hasString {
					// setting the signature also signals that we're done
					sig = makeSig(x.typ(), x.typ(), y.typ())
					sig.variadic = true
				}
			}
		}

		// general case
		if sig == nil {
			// check arguments by creating custom signature
			sig = makeSig(x.typ(), x.typ(), NewSlice(E)) // []E required for variadic signature
			sig.variadic = true
			check.arguments(call, sig, nil, nil, args, nil) // discard result (we know the result type)
			// ok to continue even if check.arguments reported errors
		}

		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, sig)
		}
		x.mode_ = value
		// x.typ is unchanged

	case _Cap, _Len:
		// cap(x)
		// len(x)
		mode := invalid
		var val constant.Value
		switch t := arrayPtrDeref(x.typ().Underlying()).(type) {
		case *Basic:
			if isString(t) && id == _Len {
				if x.mode() == constant_ {
					mode = constant_
					val = constant.MakeInt64(constant.StringLen(x.val))
				} else {
					mode = value
				}
			}

		case *Array:
			mode = value
			// spec: "The expressions len(s) and cap(s) are constants
			// if the type of s is an array or pointer to an array and
			// the expression s does not contain channel receives or
			// function calls; in this case s is not evaluated."
			if !check.hasCallOrRecv {
				mode = constant_
				if t.len >= 0 {
					val = constant.MakeInt64(t.len)
				} else {
					val = constant.MakeUnknown()
				}
			}

		case *Slice, *Chan:
			mode = value

		case *Map:
			if id == _Len {
				mode = value
			}

		case *Interface:
			if !isTypeParam(x.typ()) {
				break
			}
			if underIs(x.typ(), func(u Type) bool {
				switch t := arrayPtrDeref(u).(type) {
				case *Basic:
					if isString(t) && id == _Len {
						return true
					}
				case *Array, *Slice, *Chan:
					return true
				case *Map:
					if id == _Len {
						return true
					}
				}
				return false
			}) {
				mode = value
			}
		}

		if mode == invalid {
			// avoid error if underlying type is invalid
			if isValid(x.typ().Underlying()) {
				code := InvalidCap
				if id == _Len {
					code = InvalidLen
				}
				check.errorf(x, code, invalidArg+"%s for built-in %s", x, bin.name)
			}
			return
		}

		// record the signature before changing x.typ
		if check.recordTypes() && mode != constant_ {
			check.recordBuiltinType(call.Fun, makeSig(Typ[Int], x.typ()))
		}

		x.mode_ = mode
		x.typ_ = Typ[Int]
		x.val = val

	case _Clear:
		// clear(m)
		check.verifyVersionf(call.Fun, go1_21, "clear")

		if !underIs(x.typ(), func(u Type) bool {
			switch u.(type) {
			case *Map, *Slice:
				return true
			}
			check.errorf(x, InvalidClear, invalidArg+"cannot clear %s: argument must be (or constrained by) map or slice", x)
			return false
		}) {
			return
		}

		x.mode_ = novalue
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(nil, x.typ()))
		}

	case _Close:
		// close(c)
		if !underIs(x.typ(), func(u Type) bool {
			uch, _ := u.(*Chan)
			if uch == nil {
				check.errorf(x, InvalidClose, invalidOp+"cannot close non-channel %s", x)
				return false
			}
			if uch.dir == RecvOnly {
				check.errorf(x, InvalidClose, invalidOp+"cannot close receive-only channel %s", x)
				return false
			}
			return true
		}) {
			return
		}
		x.mode_ = novalue
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(nil, x.typ()))
		}

	case _Complex:
		// complex(x, y floatT) complexT
		y := args[1]

		// convert or check untyped arguments
		d := 0
		if isUntyped(x.typ()) {
			d |= 1
		}
		if isUntyped(y.typ()) {
			d |= 2
		}
		switch d {
		case 0:
			// x and y are typed => nothing to do
		case 1:
			// only x is untyped => convert to type of y
			check.convertUntyped(x, y.typ())
		case 2:
			// only y is untyped => convert to type of x
			check.convertUntyped(y, x.typ())
		case 3:
			// x and y are untyped =>
			// 1) if both are constants, convert them to untyped
			//    floating-point numbers if possible,
			// 2) if one of them is not constant (possible because
			//    it contains a shift that is yet untyped), convert
			//    both of them to float64 since they must have the
			//    same type to succeed (this will result in an error
			//    because shifts of floats are not permitted)
			if x.mode() == constant_ && y.mode() == constant_ {
				toFloat := func(x *operand) {
					if isNumeric(x.typ()) && constant.Sign(constant.Imag(x.val)) == 0 {
						x.typ_ = Typ[UntypedFloat]
					}
				}
				toFloat(x)
				toFloat(y)
			} else {
				check.convertUntyped(x, Typ[Float64])
				check.convertUntyped(y, Typ[Float64])
				// x and y should be invalid now, but be conservative
				// and check below
			}
		}
		if !x.isValid() || !y.isValid() {
			return
		}

		// both argument types must be identical
		if !Identical(x.typ(), y.typ()) {
			check.errorf(x, InvalidComplex, invalidOp+"%v (mismatched types %s and %s)", call, x.typ(), y.typ())
			return
		}

		// the argument types must be of floating-point type
		// (applyTypeFunc never calls f with a type parameter)
		f := func(typ Type) Type {
			assert(!isTypeParam(typ))
			if t, _ := typ.Underlying().(*Basic); t != nil {
				switch t.kind {
				case Float32:
					return Typ[Complex64]
				case Float64:
					return Typ[Complex128]
				case UntypedFloat:
					return Typ[UntypedComplex]
				}
			}
			return nil
		}
		resTyp := check.applyTypeFunc(f, x, id)
		if resTyp == nil {
			check.errorf(x, InvalidComplex, invalidArg+"arguments have type %s, expected floating-point", x.typ())
			return
		}

		// if both arguments are constants, the result is a constant
		if x.mode() == constant_ && y.mode() == constant_ {
			x.val = constant.BinaryOp(constant.ToFloat(x.val), token.ADD, constant.MakeImag(constant.ToFloat(y.val)))
		} else {
			x.mode_ = value
		}

		if check.recordTypes() && x.mode() != constant_ {
			check.recordBuiltinType(call.Fun, makeSig(resTyp, x.typ(), x.typ()))
		}

		x.typ_ = resTyp

	case _Copy:
		// copy(x, y []E) int
		// spec: "The function copy copies slice elements from a source src to a destination
		// dst and returns the number of elements copied. Both arguments must have identical
		// element type E and must be assignable to a slice of type []E.
		// The number of elements copied is the minimum of len(src) and len(dst).
		// As a special case, copy also accepts a destination argument assignable to type
		// []byte with a source argument of a string type.
		// This form copies the bytes from the string into the byte slice."

		// In either case, the first argument must be a slice; in particular it
		// cannot be the predeclared nil value. Note that nil is not excluded by
		// the assignability requirement alone for the special case (go.dev/issue/79687).
		// spec: "If the type of one or both arguments is a type parameter, all types
		// in their respective type sets must have the same underlying slice type []E."
		dstE, err := sliceElem(x)
		if err != nil {
			check.errorf(x, InvalidCopy, "invalid copy: %s", err.format(check))
			return
		}

		// get special case out of the way
		y := args[1]
		var special bool
		if ok, _ := x.assignableTo(check, NewSlice(universeByte), nil); ok {
			special = true
			for _, u := range typeset(y.typ()) {
				if s, _ := u.(*Slice); s != nil && Identical(s.elem, universeByte) {
					// typeset ⊇ {[]byte}
				} else if u != nil && isString(u) {
					// typeset ⊇ {string}
				} else {
					special = false
					break
				}
			}
		}

		// general case
		if !special {
			srcE, err := sliceElem(y)
			if err != nil {
				// If we have a string, for a better error message proceed with byte element type.
				if !allString(y.typ()) {
					check.errorf(y, InvalidCopy, "invalid copy: %s", err.format(check))
					return
				}
				srcE = universeByte
			}
			if !Identical(dstE, srcE) {
				check.errorf(x, InvalidCopy, "invalid copy: arguments %s and %s have different element types %s and %s", x, y, dstE, srcE)
				return
			}
		}

		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(Typ[Int], x.typ(), y.typ()))
		}
		x.mode_ = value
		x.typ_ = Typ[Int]

	case _Delete:
		// delete(map_, key)
		// map_ must be a map type or a type parameter describing map types.
		// The key cannot be a type parameter for now.
		map_ := x.typ()
		var key Type
		if !underIs(map_, func(u Type) bool {
			map_, _ := u.(*Map)
			if map_ == nil {
				check.errorf(x, InvalidDelete, invalidArg+"%s is not a map", x)
				return false
			}
			if key != nil && !Identical(map_.key, key) {
				check.errorf(x, InvalidDelete, invalidArg+"maps of %s must have identical key types", x)
				return false
			}
			key = map_.key
			return true
		}) {
			return
		}

		*x = *args[1] // key
		check.assignment(x, key, "argument to delete")
		if !x.isValid() {
			return
		}

		x.mode_ = novalue
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(nil, map_, key))
		}

	case _Imag, _Real:
		// imag(complexT) floatT
		// real(complexT) floatT

		// convert or check untyped argument
		if isUntyped(x.typ()) {
			if x.mode() == constant_ {
				// an untyped constant number can always be considered
				// as a complex constant
				if isNumeric(x.typ()) {
					x.typ_ = Typ[UntypedComplex]
				}
			} else {
				// an untyped non-constant argument may appear if
				// it contains a (yet untyped non-constant) shift
				// expression: convert it to complex128 which will
				// result in an error (shift of complex value)
				check.convertUntyped(x, Typ[Complex128])
				// x should be invalid now, but be conservative and check
				if !x.isValid() {
					return
				}
			}
		}

		// the argument must be of complex type
		// (applyTypeFunc never calls f with a type parameter)
		f := func(typ Type) Type {
			assert(!isTypeParam(typ))
			if t, _ := typ.Underlying().(*Basic); t != nil {
				switch t.kind {
				case Complex64:
					return Typ[Float32]
				case Complex128:
					return Typ[Float64]
				case UntypedComplex:
					return Typ[UntypedFloat]
				}
			}
			return nil
		}
		resTyp := check.applyTypeFunc(f, x, id)
		if resTyp == nil {
			code := InvalidImag
			if id == _Real {
				code = InvalidReal
			}
			check.errorf(x, code, invalidArg+"argument has type %s, expected complex type", x.typ())
			return
		}

		// if the argument is a constant, the result is a constant
		if x.mode() == constant_ {
			if id == _Real {
				x.val = constant.Real(x.val)
			} else {
				x.val = constant.Imag(x.val)
			}
		} else {
			x.mode_ = value
		}

		if check.recordTypes() && x.mode() != constant_ {
			check.recordBuiltinType(call.Fun, makeSig(resTyp, x.typ()))
		}

		x.typ_ = resTyp

	case _Make:
		// make(T, n)
		// make(T, n, m)
		// (no argument evaluated yet)
		arg0 := argList[0]
		T := check.varType(arg0)
		if !isValid(T) {
			return
		}

		u, err := commonUnder(T, func(_, u Type) *typeError {
			switch u.(type) {
			case *Slice, *Map, *Chan:
				return nil // ok
			case nil:
				return typeErrorf("no specific type")
			default:
				return typeErrorf("type must be slice, map, or channel")
			}
		})
		if err != nil {
			check.errorf(arg0, InvalidMake, invalidArg+"cannot make %s: %s", arg0, err.format(check))
			return
		}

		var min int // minimum number of arguments
		switch u.(type) {
		case *Slice:
			min = 2
		case *Map, *Chan:
			min = 1
		default:
			// any other type was excluded above
			panic("unreachable")
		}
		if nargs < min || min+1 < nargs {
			check.errorf(call, WrongArgCount, invalidOp+"%v expects %d or %d arguments; found %d", call, min, min+1, nargs)
			return
		}

		types := []Type{T}
		var sizes []int64 // constant integer arguments, if any
		for _, arg := range argList[1:] {
			typ, size := check.index(arg, -1) // ok to continue with typ == Typ[Invalid]
			types = append(types, typ)
			if size >= 0 {
				sizes = append(sizes, size)
			}
		}
		if len(sizes) == 2 && sizes[0] > sizes[1] {
			check.error(argList[1], SwappedMakeArgs, invalidArg+"length and capacity swapped")
			// safe to continue
		}
		x.mode_ = value
		x.typ_ = T
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(x.typ(), types...))
		}

	case _Max, _Min:
		// max(x, ...)
		// min(x, ...)
		check.verifyVersionf(call.Fun, go1_21, "built-in %s", bin.name)

		op := token.LSS
		if id == _Max {
			op = token.GTR
		}

		for i, a := range args {
			if !a.isValid() {
				return
			}

			if !allOrdered(a.typ()) {
				check.errorf(a, InvalidMinMaxOperand, invalidArg+"%s cannot be ordered", a)
				return
			}

			// The first argument is already in x and there's nothing left to do.
			if i > 0 {
				check.matchTypes(x, a)
				if !x.isValid() {
					return
				}

				if !Identical(x.typ(), a.typ()) {
					check.errorf(a, MismatchedTypes, invalidArg+"mismatched types %s (previous argument) and %s (type of %s)", x.typ(), a.typ(), a.expr)
					return
				}

				if x.mode() == constant_ && a.mode() == constant_ {
					if constant.Compare(a.val, op, x.val) {
						*x = *a
					}
				} else {
					x.mode_ = value
				}
			}
		}

		// If nargs == 1, make sure x.mode is either a value or a constant.
		if x.mode() != constant_ {
			x.mode_ = value
			// A value must not be untyped.
			check.assignment(x, &emptyInterface, "argument to built-in "+bin.name)
			if !x.isValid() {
				return
			}
		}

		// Use the final type computed above for all arguments.
		for _, a := range args {
			check.updateExprType(a.expr, x.typ(), true)
		}

		if check.recordTypes() && x.mode() != constant_ {
			types := make([]Type, nargs)
			for i := range types {
				types[i] = x.typ()
			}
			check.recordBuiltinType(call.Fun, makeSig(x.typ(), types...))
		}

	case _New:
		// new(T) or new(expr)
		// (no argument evaluated yet)
		arg := argList[0]
		check.exprOrType(x, arg, false)
		check.exclude(x, 1<<novalue|1<<builtin)
		switch x.mode() {
		case invalid:
			return
		case typexpr:
			// new(T)
			check.validVarType(arg, x.typ())
		default:
			// new(expr)
			if isUntyped(x.typ()) {
				// check for overflow and untyped nil
				check.assignment(x, nil, "argument to new")
				if !x.isValid() {
					return
				}
				assert(isTyped(x.typ()))
			}
			// report version error only if there are no other errors
			check.verifyVersionf(call.Fun, go1_26, "new(%s)", arg)
		}

		T := x.typ()
		x.mode_ = value
		x.typ_ = NewPointer(T)
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(x.typ(), T))
		}

	case _Panic:
		// panic(x)
		// record panic call if inside a function with result parameters
		// (for use in Checker.isTerminating)
		if check.sig != nil && check.sig.results.Len() > 0 {
			// function has result parameters
			p := check.isPanic
			if p == nil {
				// allocate lazily
				p = make(map[*syntax.CallExpr]bool)
				check.isPanic = p
			}
			p[call] = true
		}

		check.assignment(x, &emptyInterface, "argument to panic")
		if !x.isValid() {
			return
		}

		x.mode_ = novalue
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(nil, &emptyInterface))
		}

	case _Print, _Println:
		// print(x, y, ...)
		// println(x, y, ...)
		var params []Type
		if nargs > 0 {
			params = make([]Type, nargs)
			for i, a := range args {
				check.assignment(a, nil, "argument to built-in "+predeclaredFuncs[id].name)
				if !a.isValid() {
					return
				}
				params[i] = a.typ()
			}
		}

		x.mode_ = novalue
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(nil, params...))
		}

	case _Recover:
		// recover() interface{}
		x.mode_ = value
		x.typ_ = &emptyInterface
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(x.typ()))
		}

	case _Add:
		// unsafe.Add(ptr unsafe.Pointer, len IntegerType) unsafe.Pointer
		check.verifyVersionf(call.Fun, go1_17, "unsafe.Add")

		check.assignment(x, Typ[UnsafePointer], "argument to unsafe.Add")
		if !x.isValid() {
			return
		}

		y := args[1]
		if !check.isValidIndex(y, InvalidUnsafeAdd, "length", true) {
			return
		}

		x.mode_ = value
		x.typ_ = Typ[UnsafePointer]
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(x.typ(), x.typ(), y.typ()))
		}

	case _Alignof:
		// unsafe.Alignof(x T) uintptr
		check.assignment(x, nil, "argument to unsafe.Alignof")
		if !x.isValid() {
			return
		}

		if check.hasVarSize(x.typ()) {
			x.mode_ = value
			if check.recordTypes() {
				check.recordBuiltinType(call.Fun, makeSig(Typ[Uintptr], x.typ()))
			}
		} else {
			x.mode_ = constant_
			x.val = constant.MakeInt64(check.conf.alignof(x.typ()))
			// result is constant - no need to record signature
		}
		x.typ_ = Typ[Uintptr]

	case _Offsetof:
		// unsafe.Offsetof(x T) uintptr, where x must be a selector
		// (no argument evaluated yet)
		arg0 := argList[0]
		selx, _ := syntax.Unparen(arg0).(*syntax.SelectorExpr)
		if selx == nil {
			check.errorf(arg0, BadOffsetofSyntax, invalidArg+"%s is not a selector expression", arg0)
			check.use(arg0)
			return
		}

		check.expr(nil, x, selx.X)
		if !x.isValid() {
			return
		}

		base := derefStructPtr(x.typ())
		sel := selx.Sel.Value
		obj, index, indirect := lookupFieldOrMethod(base, false, check.pkg, sel, false)
		switch obj.(type) {
		case nil:
			check.errorf(x, MissingFieldOrMethod, invalidArg+"%s has no single field %s", base, sel)
			return
		case *Func:
			// TODO(gri) Using derefStructPtr may result in methods being found
			// that don't actually exist. An error either way, but the error
			// message is confusing. See: https://play.golang.org/p/al75v23kUy ,
			// but go/types reports: "invalid argument: x.m is a method value".
			check.errorf(arg0, InvalidOffsetof, invalidArg+"%s is a method value", arg0)
			return
		}
		if indirect {
			check.errorf(x, InvalidOffsetof, invalidArg+"field %s is embedded via a pointer in %s", sel, base)
			return
		}

		// TODO(gri) Should we pass x.typ instead of base (and have indirect report if derefStructPtr indirected)?
		check.recordSelection(selx, FieldVal, base, obj, index, false)

		// record the selector expression (was bug - go.dev/issue/47895)
		{
			mode := value
			if x.mode() == variable || indirect {
				mode = variable
			}
			check.record(&operand{mode, selx, obj.Type(), nil, 0})
		}

		// The field offset is considered a variable even if the field is declared before
		// the part of the struct which is variable-sized. This makes both the rules
		// simpler and also permits (or at least doesn't prevent) a compiler from re-
		// arranging struct fields if it wanted to.
		if check.hasVarSize(base) {
			x.mode_ = value
			if check.recordTypes() {
				check.recordBuiltinType(call.Fun, makeSig(Typ[Uintptr], obj.Type()))
			}
		} else {
			offs := check.conf.offsetof(base, index)
			if offs < 0 {
				check.errorf(x, TypeTooLarge, "%s is too large", x)
				return
			}
			x.mode_ = constant_
			x.val = constant.MakeInt64(offs)
			// result is constant - no need to record signature
		}
		x.typ_ = Typ[Uintptr]

	case _Sizeof:
		// unsafe.Sizeof(x T) uintptr
		check.assignment(x, nil, "argument to unsafe.Sizeof")
		if !x.isValid() {
			return
		}

		if check.hasVarSize(x.typ()) {
			x.mode_ = value
			if check.recordTypes() {
				check.recordBuiltinType(call.Fun, makeSig(Typ[Uintptr], x.typ()))
			}
		} else {
			size := check.conf.sizeof(x.typ())
			if size < 0 {
				check.errorf(x, TypeTooLarge, "%s is too large", x)
				return
			}
			x.mode_ = constant_
			x.val = constant.MakeInt64(size)
			// result is constant - no need to record signature
		}
		x.typ_ = Typ[Uintptr]

	case _Slice:
		// unsafe.Slice(ptr *T, len IntegerType) []T
		check.verifyVersionf(call.Fun, go1_17, "unsafe.Slice")

		u, _ := commonUnder(x.typ(), nil)
		ptr, _ := u.(*Pointer)
		if ptr == nil {
			check.errorf(x, InvalidUnsafeSlice, invalidArg+"%s is not a pointer", x)
			return
		}

		y := args[1]
		if !check.isValidIndex(y, InvalidUnsafeSlice, "length", false) {
			return
		}

		x.mode_ = value
		x.typ_ = NewSlice(ptr.base)
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(x.typ(), ptr, y.typ()))
		}

	case _SliceData:
		// unsafe.SliceData(slice []T) *T
		check.verifyVersionf(call.Fun, go1_20, "unsafe.SliceData")

		u, _ := commonUnder(x.typ(), nil)
		slice, _ := u.(*Slice)
		if slice == nil {
			check.errorf(x, InvalidUnsafeSliceData, invalidArg+"%s is not a slice", x)
			return
		}

		x.mode_ = value
		x.typ_ = NewPointer(slice.elem)
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(x.typ(), slice))
		}

	case _String:
		// unsafe.String(ptr *byte, len IntegerType) string
		check.verifyVersionf(call.Fun, go1_20, "unsafe.String")

		check.assignment(x, NewPointer(universeByte), "argument to unsafe.String")
		if !x.isValid() {
			return
		}

		y := args[1]
		if !check.isValidIndex(y, InvalidUnsafeString, "length", false) {
			return
		}

		x.mode_ = value
		x.typ_ = Typ[String]
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(x.typ(), NewPointer(universeByte), y.typ()))
		}

	case _StringData:
		// unsafe.StringData(str string) *byte
		check.verifyVersionf(call.Fun, go1_20, "unsafe.StringData")

		check.assignment(x, Typ[String], "argument to unsafe.StringData")
		if !x.isValid() {
			return
		}

		x.mode_ = value
		x.typ_ = NewPointer(universeByte)
		if check.recordTypes() {
			check.recordBuiltinType(call.Fun, makeSig(x.typ(), Typ[String]))
		}

	case _Assert:
		// assert(pred) causes a typechecker error if pred is false.
		// The result of assert is the value of pred if there is no error.
		// Note: assert is only available in self-test mode.
		if x.mode() != constant_ || !isBoolean(x.typ()) {
			check.errorf(x, Test, invalidArg+"%s is not a boolean constant", x)
			return
		}
		if x.val.Kind() != constant.Bool {
			check.errorf(x, Test, "internal error: value of %s should be a boolean constant", x)
			return
		}
		if !constant.BoolVal(x.val) {
			check.errorf(call, Test, "%v failed", call)
			// compile-time assertion failure - safe to continue
		}
		// result is constant - no need to record signature

	case _Trace:
		// trace(x, y, z, ...) dumps the positions, expressions, and
		// values of its arguments. The result of trace is the value
		// of the first argument.
		// Note: trace is only available in self-test mode.
		// (no argument evaluated yet)
		if nargs == 0 {
			check.dump("%v: trace() without arguments", atPos(call))
			x.mode_ = novalue
			break
		}
		var t operand
		x1 := x
		for _, arg := range argList {
			check.rawExpr(nil, x1, arg, nil, false) // permit trace for types, e.g.: new(trace(T))
			check.dump("%v: %s", atPos(x1), x1)
			x1 = &t // use incoming x only for first argument
		}
		if !x.isValid() {
			return
		}
		// trace is only available in test mode - no need to record signature

	default:
		panic("unreachable")
	}

	assert(x.isValid())
	return true
}

// sliceElem returns the slice element type for a slice operand x
// or a type error if x is not a slice (or a type set of slices).
func sliceElem(x *operand) (Type, *typeError) {
	var E Type
	for _, u := range typeset(x.typ()) {
		s, _ := u.(*Slice)
		if s == nil {
			if x.isNil() {
				// Printing x in this case would just print "nil".
				// Special case this so we can emphasize "untyped".
				return nil, typeErrorf("argument must be a slice; have untyped nil")
			} else {
				return nil, typeErrorf("argument must be a slice; have %s", x)
			}
		}
		if E == nil {
			E = s.elem
		} else if !Identical(E, s.elem) {
			return nil, typeErrorf("mismatched slice element types %s and %s in %s", E, s.elem, x)
		}
	}
	return E, nil
}

// hasVarSize reports if the size of type t is variable due to type parameters
// or if the type is infinitely-sized due to a cycle for which the type has not
// yet been checked.
func (check *Checker) hasVarSize(t Type) bool {
	// Note: We could use Underlying here, but passing through the RHS may yield
	// better error messages and allows us to stash the result on each traversed
	// Named type.
	switch t := Unalias(t).(type) {
	case *Named:
		if t.stateHas(hasVarSize) {
			return t.varSize
		}

		if i, ok := check.objPathIdx[t.obj]; ok {
			cycle := check.objPath[i:]
			check.cycleError(cycle, firstInSrc(cycle))
			return true
		}

		obj := t.obj
		check.push(obj)
		defer check.pop()

		// Careful, we're inspecting t.fromRHS, so we need to unpack first.
		t.unpack()
		varSize := check.hasVarSize(t.rhs())

		// Special case for portable simd types that rewrite to unknown sizes.
		if pkg := obj.Pkg(); pkg != nil && pkg.Path() == "simd" && obj.Name() == "_simd" {
			varSize = true
		}

		t.mu.Lock()
		defer t.mu.Unlock()

		// Careful, t.varSize has lock-free readers. Since we might be racing
		// another call to hasVarSize, we have to avoid overwriting t.varSize.
		// Otherwise, the race detector will be tripped.
		if !t.stateHas(hasVarSize) {
			t.varSize = varSize
			t.setState(hasVarSize)
		}

		return varSize

	case *Array:
		// The array length is already computed. If it was a valid length, it
		// is constant; else, an error was reported in the computation.
		return check.hasVarSize(t.elem)

	case *Struct:
		for _, f := range t.fields {
			if check.hasVarSize(f.typ) {
				return true
			}
		}

	case *TypeParam:
		return true
	}

	return false
}

// applyTypeFunc applies f to x. If x is a type parameter,
// the result is a type parameter constrained by a new
// interface bound. The type bounds for that interface
// are computed by applying f to each of the type bounds
// of x. If any of these applications of f return nil,
// applyTypeFunc returns nil.
// If x is not a type parameter, the result is f(x).
func (check *Checker) applyTypeFunc(f func(Type) Type, x *operand, id builtinId) Type {
	if tp, _ := Unalias(x.typ()).(*TypeParam); tp != nil {
		// Test if t satisfies the requirements for the argument
		// type and collect possible result types at the same time.
		var terms []*Term
		if !tp.is(func(t *term) bool {
			if t == nil {
				return false
			}
			if r := f(t.typ); r != nil {
				terms = append(terms, NewTerm(t.tilde, r))
				return true
			}
			return false
		}) {
			return nil
		}

		// We can type-check this fine but we're introducing a synthetic
		// type parameter for the result. It's not clear what the API
		// implications are here. Report an error for 1.18 (see go.dev/issue/50912),
		// but continue type-checking.
		var code Code
		switch id {
		case _Real:
			code = InvalidReal
		case _Imag:
			code = InvalidImag
		case _Complex:
			code = InvalidComplex
		default:
			panic("unreachable")
		}
		check.softErrorf(x, code, "%s not supported as argument to built-in %s for go1.18 (see go.dev/issue/50937)", x, predeclaredFuncs[id].name)

		// Construct a suitable new type parameter for the result type.
		// The type parameter is placed in the current package so export/import
		// works as expected.
		tpar := NewTypeName(nopos, check.pkg, tp.obj.name, nil)
		ptyp := check.newTypeParam(tpar, NewInterfaceType(nil, []Type{NewUnion(terms)})) // assigns type to tpar as a side-effect
		ptyp.index = tp.index

		return ptyp
	}

	return f(x.typ())
}

// makeSig makes a signature for the given argument and result types.
// Default types are used for untyped arguments, and res may be nil.
func makeSig(res Type, args ...Type) *Signature {
	list := make([]*Var, len(args))
	for i, param := range args {
		list[i] = NewParam(nopos, nil, "", Default(param))
	}
	params := NewTuple(list...)
	var result *Tuple
	if res != nil {
		assert(!isUntyped(res))
		result = NewTuple(newVar(ResultVar, nopos, nil, "", res))
	}
	return &Signature{params: params, results: result}
}

// arrayPtrDeref returns A if typ is of the form *A and A is an array;
// otherwise it returns typ.
func arrayPtrDeref(typ Type) Type {
	if p, ok := Unalias(typ).(*Pointer); ok {
		if a, _ := p.base.Underlying().(*Array); a != nil {
			return a
		}
	}
	return typ
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/call.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of call and selector expressions.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
	"strings"
)

// funcInst type-checks a function instantiation.
// The incoming x must be a generic function.
// If inst != nil, it provides some or all of the type arguments (inst.Index).
// If target != nil, it may be used to infer missing type arguments of x, if any.
// At least one of T or inst must be provided.
//
// There are two modes of operation:
//
//  1. If infer == true, funcInst infers missing type arguments as needed and
//     instantiates the function x. The returned results are nil.
//
//  2. If infer == false and inst provides all type arguments, funcInst
//     instantiates the function x. The returned results are nil.
//     If inst doesn't provide enough type arguments, funcInst returns the
//     available arguments; x remains unchanged.
//
// If an error (other than a version error) occurs in any case, it is reported
// and x.mode is set to invalid.
func (check *Checker) funcInst(T *target, pos syntax.Pos, x *operand, inst *syntax.IndexExpr, infer bool) []Type {
	assert(T != nil || inst != nil)

	var instErrPos poser
	if inst != nil {
		instErrPos = inst.Pos()
		x.expr = inst // if we don't have an index expression, keep the existing expression of x
	} else {
		instErrPos = pos
	}
	versionErr := !check.verifyVersionf(instErrPos, go1_18, "function instantiation")

	// targs and xlist are the type arguments and corresponding type expressions, or nil.
	var targs []Type
	var xlist []syntax.Expr
	if inst != nil {
		xlist = syntax.UnpackListExpr(inst.Index)
		targs = check.typeList(xlist)
		if targs == nil {
			x.invalidate()
			return nil
		}
		assert(len(targs) == len(xlist))
	}

	// Check the number of type arguments (got) vs number of type parameters (want).
	// Note that x is a function value, not a type expression, so we don't need to
	// call Underlying below.
	sig := x.typ().(*Signature)
	got, want := len(targs), sig.TypeParams().Len()
	if got > want {
		// Providing too many type arguments is always an error.
		check.errorf(xlist[got-1], WrongTypeArgCount, "got %d type arguments but want %d", got, want)
		x.invalidate()
		return nil
	}

	if got < want {
		if !infer {
			return targs
		}

		// If the uninstantiated or partially instantiated function x is used in
		// an assignment (tsig != nil), infer missing type arguments by treating
		// the assignment
		//
		//    var tvar tsig = x
		//
		// like a call g(tvar) of the synthetic generic function g
		//
		//    func g[type_parameters_of_x](func_type_of_x)
		//
		var args []*operand
		var params []*Var
		var reverse bool
		if T != nil && sig.tparams != nil {
			if !versionErr && !check.allowVersion(go1_21) {
				if inst != nil {
					check.versionErrorf(instErrPos, go1_21, "partially instantiated function in assignment")
				} else {
					check.versionErrorf(instErrPos, go1_21, "implicitly instantiated function in assignment")
				}
			}
			gsig := NewSignatureType(nil, nil, nil, sig.params, sig.results, sig.variadic)
			params = []*Var{NewParam(x.Pos(), check.pkg, "", gsig)}
			// The type of the argument operand is tsig, which is the type of the LHS in an assignment
			// or the result type in a return statement. Create a pseudo-expression for that operand
			// that makes sense when reported in error messages from infer, below.
			expr := syntax.NewName(x.Pos(), T.desc)
			args = []*operand{{mode_: value, expr: expr, typ_: T.sig}}
			reverse = true
		}

		// Rename type parameters to avoid problems with recursive instantiations.
		// Note that NewTuple(params...) below is (*Tuple)(nil) if len(params) == 0, as desired.
		tparams, params2 := check.renameTParams(pos, sig.TypeParams().list(), NewTuple(params...))

		err := check.newError(CannotInferTypeArgs)
		targs = check.infer(pos, tparams, targs, params2.(*Tuple), args, reverse, err)
		if targs == nil {
			if !err.empty() {
				err.report()
			}
			x.invalidate()
			return nil
		}
		got = len(targs)
	}
	assert(got == want)

	// instantiate function signature
	sig = check.instantiateSignature(x.Pos(), x.expr, sig, targs, xlist)

	x.typ_ = sig
	x.mode_ = value
	return nil
}

func (check *Checker) instantiateSignature(pos syntax.Pos, expr syntax.Expr, typ *Signature, targs []Type, xlist []syntax.Expr) (res *Signature) {
	assert(check != nil)
	assert(len(targs) == typ.TypeParams().Len())

	if check.conf.Trace {
		check.trace(pos, "-- instantiating signature %s with %s", typ, targs)
		check.indent++
		defer func() {
			check.indent--
			check.trace(pos, "=> %s (under = %s)", res, res.Underlying())
		}()
	}

	// For signatures, Checker.instance will always succeed because the type argument
	// count is correct at this point (see assertion above); hence the type assertion
	// to *Signature will always succeed.
	inst := check.instance(pos, typ, targs, nil, check.context()).(*Signature)
	assert(inst.TypeParams().Len() == 0) // signature is not generic anymore
	check.recordInstance(expr, targs, inst)
	assert(len(xlist) <= len(targs))

	// verify instantiation lazily (was go.dev/issue/50450)
	check.later(func() {
		tparams := typ.TypeParams().list()
		// check type constraints
		if i, err := check.verify(pos, tparams, targs, check.context()); err != nil {
			// best position for error reporting
			pos := pos
			if i < len(xlist) {
				pos = syntax.StartPos(xlist[i])
			}
			check.softErrorf(pos, InvalidTypeArg, "%s", err)
		} else {
			check.mono.recordInstance(check.pkg, pos, tparams, targs, xlist)
		}
	}).describef(pos, "verify instantiation")

	return inst
}

func (check *Checker) callExpr(x *operand, call *syntax.CallExpr) exprKind {
	var inst *syntax.IndexExpr // function instantiation, if any
	if iexpr, _ := call.Fun.(*syntax.IndexExpr); iexpr != nil {
		if check.indexExpr(x, iexpr) {
			// Delay function instantiation to argument checking,
			// where we combine type and value arguments for type
			// inference.
			assert(x.mode() == value)
			inst = iexpr
		}
		x.expr = iexpr
		check.record(x)
	} else {
		check.exprOrType(x, call.Fun, true)
	}
	// x.typ may be generic

	switch x.mode() {
	case invalid:
		check.use(call.ArgList...)
		x.expr = call
		return statement

	case typexpr:
		// conversion
		check.nonGeneric(nil, x)
		if !x.isValid() {
			return conversion
		}
		T := x.typ()
		x.invalidate()
		// We cannot convert a value to an incomplete type; make sure it's complete.
		if !check.isComplete(T) {
			x.expr = call
			return conversion
		}
		switch n := len(call.ArgList); n {
		case 0:
			check.errorf(call, WrongArgCount, "missing argument in conversion to %s", T)
		case 1:
			check.expr(newTarget(T, "conversion"), x, call.ArgList[0])
			if x.isValid() {
				if t, _ := T.Underlying().(*Interface); t != nil && !isTypeParam(T) {
					if !t.IsMethodSet() {
						check.errorf(call, MisplacedConstraintIface, "cannot use interface %s in conversion (contains specific type constraints or is comparable)", T)
						break
					}
				}
				if hasDots(call) {
					check.errorf(call.ArgList[0], BadDotDotDotSyntax, "invalid use of ... in conversion to %s", T)
					break
				}
				check.conversion(x, T)
			}
		default:
			check.use(call.ArgList...)
			check.errorf(call.ArgList[n-1], WrongArgCount, "too many arguments in conversion to %s", T)
		}
		x.expr = call
		return conversion

	case builtin:
		// no need to check for non-genericity here
		id := x.id
		if !check.builtin(x, call, id) {
			x.invalidate()
		}
		x.expr = call
		// a non-constant result implies a function call
		if x.isValid() && x.mode() != constant_ {
			check.hasCallOrRecv = true
		}
		return predeclaredFuncs[id].kind
	}

	// ordinary function/method call
	// signature may be generic
	cgocall := x.mode() == cgofunc

	// If the operand type is a type parameter, all types in its type set
	// must have a common underlying type, which must be a signature.
	u, err := commonUnder(x.typ(), func(t, u Type) *typeError {
		if _, ok := u.(*Signature); u != nil && !ok {
			return typeErrorf("%s is not a function", t)
		}
		return nil
	})
	if err != nil {
		check.errorf(x, InvalidCall, invalidOp+"cannot call %s: %s", x, err.format(check))
		x.invalidate()
		x.expr = call
		return statement
	}
	sig := u.(*Signature) // u must be a signature per the commonUnder condition

	// Capture wasGeneric before sig is potentially instantiated below.
	wasGeneric := sig.TypeParams().Len() > 0

	// evaluate type arguments, if any
	var xlist []syntax.Expr
	var targs []Type
	if inst != nil {
		xlist = syntax.UnpackListExpr(inst.Index)
		targs = check.typeList(xlist)
		if targs == nil {
			check.use(call.ArgList...)
			x.invalidate()
			x.expr = call
			return statement
		}
		assert(len(targs) == len(xlist))

		// check number of type arguments (got) vs number of type parameters (want)
		got, want := len(targs), sig.TypeParams().Len()
		if got > want {
			check.errorf(xlist[want], WrongTypeArgCount, "got %d type arguments but want %d", got, want)
			check.use(call.ArgList...)
			x.invalidate()
			x.expr = call
			return statement
		}

		// If sig is generic and all type arguments are provided, preempt function
		// argument type inference by explicitly instantiating the signature. This
		// ensures that we record accurate type information for sig, even if there
		// is an error checking its arguments (for example, if an incorrect number
		// of arguments is supplied).
		if got == want && want > 0 {
			check.verifyVersionf(inst, go1_18, "function instantiation")
			sig = check.instantiateSignature(inst.Pos(), inst, sig, targs, xlist)
			// targs have been consumed; proceed with checking arguments of the
			// non-generic signature.
			targs = nil
			xlist = nil
		}
	}

	// evaluate arguments
	args, atargs := check.genericExprList(call.ArgList)
	sig = check.arguments(call, sig, targs, xlist, args, atargs)

	if wasGeneric && sig.TypeParams().Len() == 0 {
		// update the recorded type of call.Fun to its instantiated type
		check.recordTypeAndValue(call.Fun, value, sig, nil)
	}

	// determine result
	switch sig.results.Len() {
	case 0:
		x.mode_ = novalue
	case 1:
		if cgocall {
			x.mode_ = commaerr
		} else {
			x.mode_ = value
		}
		typ := sig.results.vars[0].typ // unpack tuple
		// We cannot return a value of an incomplete type; make sure it's complete.
		if !check.isComplete(typ) {
			x.invalidate()
			x.expr = call
			return statement
		}
		x.typ_ = typ
	default:
		x.mode_ = value
		x.typ_ = sig.results
	}
	x.expr = call
	check.hasCallOrRecv = true

	// if type inference failed, a parameterized result must be invalidated
	// (operands cannot have a parameterized type)
	if x.mode() == value && sig.TypeParams().Len() > 0 && isParameterized(sig.TypeParams().list(), x.typ()) {
		x.invalidate()
	}

	return statement
}

// exprList evaluates a list of expressions and returns the corresponding operands.
// A single-element expression list may evaluate to multiple operands.
func (check *Checker) exprList(elist []syntax.Expr) (xlist []*operand) {
	if n := len(elist); n == 1 {
		xlist, _ = check.multiExpr(elist[0], false)
	} else if n > 1 {
		// multiple (possibly invalid) values
		xlist = make([]*operand, n)
		for i, e := range elist {
			var x operand
			check.expr(nil, &x, e)
			xlist[i] = &x
		}
	}
	return
}

// genericExprList is like exprList but result operands may be uninstantiated or partially
// instantiated generic functions (where constraint information is insufficient to infer
// the missing type arguments) for Go 1.21 and later.
// For each non-generic or uninstantiated generic operand, the corresponding targsList and
// elements do not exist (targsList is nil) or the elements are nil.
// For each partially instantiated generic function operand, the corresponding
// targsList elements are the operand's partial type arguments.
func (check *Checker) genericExprList(elist []syntax.Expr) (resList []*operand, targsList [][]Type) {
	if debug {
		defer func() {
			// type arguments must only exist for partially instantiated functions
			for i, x := range resList {
				if i < len(targsList) {
					if n := len(targsList[i]); n > 0 {
						// x must be a partially instantiated function
						assert(n < x.typ().(*Signature).TypeParams().Len())
					}
				}
			}
		}()
	}

	// Before Go 1.21, uninstantiated or partially instantiated argument functions are
	// not permitted. Checker.funcInst must infer missing type arguments in that case.
	infer := true // for -lang < go1.21
	n := len(elist)
	if n > 0 && check.allowVersion(go1_21) {
		infer = false
	}

	if n == 1 {
		// single value (possibly a partially instantiated function), or a multi-valued expression
		e := elist[0]
		var x operand
		if inst, _ := e.(*syntax.IndexExpr); inst != nil && check.indexExpr(&x, inst) {
			// x is a generic function.
			targs := check.funcInst(nil, x.Pos(), &x, inst, infer)
			if targs != nil {
				// x was not instantiated: collect the (partial) type arguments.
				targsList = [][]Type{targs}
				// Update x.expr so that we can record the partially instantiated function.
				x.expr = inst
			} else {
				// x was instantiated: we must record it here because we didn't
				// use the usual expression evaluators.
				check.record(&x)
			}
			resList = []*operand{&x}
		} else {
			// x is not a function instantiation (it may still be a generic function).
			check.rawExpr(nil, &x, e, nil, true)
			check.exclude(&x, 1<<novalue|1<<builtin|1<<typexpr)
			if t, ok := x.typ().(*Tuple); ok && x.isValid() {
				// x is a function call returning multiple values; it cannot be generic.
				resList = make([]*operand, t.Len())
				for i, v := range t.vars {
					resList[i] = &operand{mode_: value, expr: e, typ_: v.typ}
				}
			} else {
				// x is exactly one value (possibly invalid or uninstantiated generic function).
				resList = []*operand{&x}
			}
		}
	} else if n > 1 {
		// multiple values
		resList = make([]*operand, n)
		targsList = make([][]Type, n)
		for i, e := range elist {
			var x operand
			if inst, _ := e.(*syntax.IndexExpr); inst != nil && check.indexExpr(&x, inst) {
				// x is a generic function.
				targs := check.funcInst(nil, x.Pos(), &x, inst, infer)
				if targs != nil {
					// x was not instantiated: collect the (partial) type arguments.
					targsList[i] = targs
					// Update x.expr so that we can record the partially instantiated function.
					x.expr = inst
				} else {
					// x was instantiated: we must record it here because we didn't
					// use the usual expression evaluators.
					check.record(&x)
				}
			} else {
				// x is exactly one value (possibly invalid or uninstantiated generic function).
				check.genericExpr(&x, e, nil)
			}
			resList[i] = &x
		}
	}

	return
}

// arguments type-checks arguments passed to a function call with the given signature.
// The function and its arguments may be generic, and possibly partially instantiated.
// targs and xlist are the function's type arguments (and corresponding expressions).
// args are the function arguments. If an argument args[i] is a partially instantiated
// generic function, atargs[i] are the corresponding type arguments.
// If the callee is variadic, arguments adjusts its signature to match the provided
// arguments. The type parameters and arguments of the callee and all its arguments
// are used together to infer any missing type arguments, and the callee and argument
// functions are instantiated as necessary.
// The result signature is the (possibly adjusted and instantiated) function signature.
// If an error occurred, the result signature is the incoming sig.
func (check *Checker) arguments(call *syntax.CallExpr, sig *Signature, targs []Type, xlist []syntax.Expr, args []*operand, atargs [][]Type) (rsig *Signature) {
	rsig = sig

	// Function call argument/parameter count requirements
	//
	//               | standard call    | dotdotdot call |
	// --------------+------------------+----------------+
	// standard func | nargs == npars   | invalid        |
	// --------------+------------------+----------------+
	// variadic func | nargs >= npars-1 | nargs == npars |
	// --------------+------------------+----------------+

	nargs := len(args)
	npars := sig.params.Len()
	ddd := hasDots(call)

	// set up parameters
	sigParams := sig.params // adjusted for variadic functions (may be nil for empty parameter lists!)
	adjusted := false       // indicates if sigParams is different from sig.params
	if sig.variadic {
		if ddd {
			// variadic_func(a, b, c...)
			if len(call.ArgList) == 1 && nargs > 1 {
				// f()... is not permitted if f() is multi-valued
				//check.errorf(call.Ellipsis, "cannot use ... with %d-valued %s", nargs, call.ArgList[0])
				check.errorf(call, InvalidDotDotDot, "cannot use ... with %d-valued %s", nargs, call.ArgList[0])
				return
			}
		} else {
			// variadic_func(a, b, c)
			if nargs >= npars-1 {
				// Create custom parameters for arguments: keep
				// the first npars-1 parameters and add one for
				// each argument mapping to the ... parameter.
				vars := make([]*Var, npars-1) // npars > 0 for variadic functions
				copy(vars, sig.params.vars)
				last := sig.params.vars[npars-1]
				typ := last.typ.(*Slice).elem
				for len(vars) < nargs {
					vars = append(vars, NewParam(last.pos, last.pkg, last.name, typ))
				}
				sigParams = NewTuple(vars...) // possibly nil!
				adjusted = true
				npars = nargs
			} else {
				// nargs < npars-1
				npars-- // for correct error message below
			}
		}
	} else {
		if ddd {
			// standard_func(a, b, c...)
			//check.errorf(call.Ellipsis, "cannot use ... in call to non-variadic %s", call.Fun)
			check.errorf(call, NonVariadicDotDotDot, "cannot use ... in call to non-variadic %s", call.Fun)
			return
		}
		// standard_func(a, b, c)
	}

	// check argument count
	if nargs != npars {
		var at poser = call
		qualifier := "not enough"
		if nargs > npars {
			at = args[npars].expr // report at first extra argument
			qualifier = "too many"
		} else if nargs > 0 {
			at = args[nargs-1].expr // report at last argument
		}
		// take care of empty parameter lists represented by nil tuples
		var params []*Var
		if sig.params != nil {
			params = sig.params.vars
		}
		err := check.newError(WrongArgCount)
		err.addf(at, "%s arguments in call to %s", qualifier, call.Fun)
		err.addf(nopos, "have %s", check.typesSummary(operandTypes(args), false, ddd))
		err.addf(nopos, "want %s", check.typesSummary(varTypes(params), sig.variadic, false))
		err.report()
		return
	}

	// collect type parameters of callee and generic function arguments
	var tparams []*TypeParam

	// collect type parameters of callee
	n := sig.TypeParams().Len()
	if n > 0 {
		if !check.allowVersion(go1_18) {
			if iexpr, _ := call.Fun.(*syntax.IndexExpr); iexpr != nil {
				check.versionErrorf(iexpr, go1_18, "function instantiation")
			} else {
				check.versionErrorf(call, go1_18, "implicit function instantiation")
			}
		}
		// rename type parameters to avoid problems with recursive calls
		var tmp Type
		tparams, tmp = check.renameTParams(call.Pos(), sig.TypeParams().list(), sigParams)
		sigParams = tmp.(*Tuple)
		// make sure targs and tparams have the same length
		for len(targs) < len(tparams) {
			targs = append(targs, nil)
		}
	}
	assert(len(tparams) == len(targs))

	// collect type parameters from generic function arguments
	var genericArgs []int // indices of generic function arguments
	if enableReverseTypeInference {
		for i, arg := range args {
			// generic arguments cannot have a defined (*Named) type - no need for underlying type below
			if asig, _ := arg.typ().(*Signature); asig != nil && asig.TypeParams().Len() > 0 {
				// The argument type is a generic function signature. This type is
				// pointer-identical with (it's copied from) the type of the generic
				// function argument and thus the function object.
				// Before we change the type (type parameter renaming, below), make
				// a clone of it as otherwise we implicitly modify the object's type
				// (go.dev/issues/63260).
				asig = clone(asig)
				// Rename type parameters for cases like f(g, g); this gives each
				// generic function argument a unique type identity (go.dev/issues/59956).
				// TODO(gri) Consider only doing this if a function argument appears
				//           multiple times, which is rare (possible optimization).
				atparams, tmp := check.renameTParams(call.Pos(), asig.TypeParams().list(), asig)
				asig = tmp.(*Signature)
				asig.tparams = &TypeParamList{atparams} // renameTParams doesn't touch associated type parameters
				arg.typ_ = asig                         // new type identity for the function argument
				tparams = append(tparams, atparams...)
				// add partial list of type arguments, if any
				if i < len(atargs) {
					targs = append(targs, atargs[i]...)
				}
				// make sure targs and tparams have the same length
				for len(targs) < len(tparams) {
					targs = append(targs, nil)
				}
				genericArgs = append(genericArgs, i)
			}
		}
	}
	assert(len(tparams) == len(targs))

	// at the moment we only support implicit instantiations of argument functions
	_ = len(genericArgs) > 0 && check.verifyVersionf(args[genericArgs[0]], go1_21, "implicitly instantiated function as argument")

	// tparams holds the type parameters of the callee and generic function arguments, if any:
	// the first n type parameters belong to the callee, followed by mi type parameters for each
	// of the generic function arguments, where mi = args[i].typ.(*Signature).TypeParams().Len().

	// infer missing type arguments of callee and function arguments
	if len(tparams) > 0 {
		err := check.newError(CannotInferTypeArgs)
		targs = check.infer(call.Pos(), tparams, targs, sigParams, args, false, err)
		if targs == nil {
			// TODO(gri) If infer inferred the first targs[:n], consider instantiating
			//           the call signature for better error messages/gopls behavior.
			//           Perhaps instantiate as much as we can, also for arguments.
			//           This will require changes to how infer returns its results.
			if !err.empty() {
				check.errorf(err.pos(), CannotInferTypeArgs, "in call to %s, %s", call.Fun, err.msg())
			}
			return
		}

		// update result signature: instantiate if needed
		if n > 0 {
			rsig = check.instantiateSignature(call.Pos(), call.Fun, sig, targs[:n], xlist)
			// If the callee's parameter list was adjusted we need to update (instantiate)
			// it separately. Otherwise we can simply use the result signature's parameter
			// list.
			if adjusted {
				sigParams = check.subst(call.Pos(), sigParams, makeSubstMap(tparams[:n], targs[:n]), nil, check.context()).(*Tuple)
			} else {
				sigParams = rsig.params
			}
		}

		// compute argument signatures: instantiate if needed
		j := n
		for _, i := range genericArgs {
			arg := args[i]
			asig := arg.typ().(*Signature)
			k := j + asig.TypeParams().Len()
			// targs[j:k] are the inferred type arguments for asig
			arg.typ_ = check.instantiateSignature(call.Pos(), arg.expr, asig, targs[j:k], nil) // TODO(gri) provide xlist if possible (partial instantiations)
			check.record(arg)                                                                  // record here because we didn't use the usual expr evaluators
			j = k
		}
	}

	// check arguments
	if len(args) > 0 {
		context := check.sprintf("argument to %s", call.Fun)
		for i, a := range args {
			check.assignment(a, sigParams.vars[i].typ, context)
		}
	}

	return
}

var cgoPrefixes = [...]string{
	"_Ciconst_",
	"_Cfconst_",
	"_Csconst_",
	"_Ctype_",
	"_Cvar_", // actually a pointer to the var
	"_Cfpvar_fp_",
	"_Cfunc_",
	"_Cmacro_", // function to evaluate the expanded expression
}

func (check *Checker) selector(x *operand, e *syntax.SelectorExpr, wantType bool) {
	// these must be declared before the "goto Error" statements
	var (
		obj      Object
		index    []int
		indirect bool
	)

	sel := e.Sel.Value
	// If the identifier refers to a package, handle everything here
	// so we don't need a "package" mode for operands: package names
	// can only appear in qualified identifiers which are mapped to
	// selector expressions.
	if ident, ok := e.X.(*syntax.Name); ok {
		obj := check.lookup(ident.Value)
		if pname, _ := obj.(*PkgName); pname != nil {
			assert(pname.pkg == check.pkg)
			check.recordUse(ident, pname)
			check.usedPkgNames[pname] = true
			pkg := pname.imported

			var exp Object
			funcMode := value
			if pkg.cgo {
				// cgo special cases C.malloc: it's
				// rewritten to _CMalloc and does not
				// support two-result calls.
				if sel == "malloc" {
					sel = "_CMalloc"
				} else {
					funcMode = cgofunc
				}
				for _, prefix := range cgoPrefixes {
					// cgo objects are part of the current package (in file
					// _cgo_gotypes.go). Use regular lookup.
					exp = check.lookup(prefix + sel)
					if exp != nil {
						break
					}
				}
				if exp == nil {
					if isValidName(sel) {
						check.errorf(e.Sel, UndeclaredImportedName, "undefined: %s", syntax.Expr(e)) // cast to syntax.Expr to silence vet
					}
					goto Error
				}
				check.objDecl(exp)
			} else {
				exp = pkg.scope.Lookup(sel)
				if exp == nil {
					if !pkg.fake && isValidName(sel) {
						// Try to give a better error message when selector matches an object name ignoring case.
						exps := pkg.scope.lookupIgnoringCase(sel, true)
						if len(exps) >= 1 {
							// report just the first one
							check.errorf(e.Sel, UndeclaredImportedName, "undefined: %s (but have %s)", syntax.Expr(e), exps[0].Name())
						} else {
							check.errorf(e.Sel, UndeclaredImportedName, "undefined: %s", syntax.Expr(e))
						}
					}
					goto Error
				}
				if !exp.Exported() {
					check.errorf(e.Sel, UnexportedName, "name %s not exported by package %s", sel, pkg.name)
					// ok to continue
				}
			}
			check.recordUse(e.Sel, exp)

			// Simplified version of the code for *syntax.Names:
			// - imported objects are always fully initialized
			switch exp := exp.(type) {
			case *Const:
				assert(exp.Val() != nil)
				x.mode_ = constant_
				x.typ_ = exp.typ
				x.val = exp.val
			case *TypeName:
				x.mode_ = typexpr
				x.typ_ = exp.typ
			case *Var:
				x.mode_ = variable
				x.typ_ = exp.typ
				if pkg.cgo && strings.HasPrefix(exp.name, "_Cvar_") {
					x.typ_ = x.typ().(*Pointer).base
				}
			case *Func:
				x.mode_ = funcMode
				x.typ_ = exp.typ
				if pkg.cgo && strings.HasPrefix(exp.name, "_Cmacro_") {
					x.mode_ = value
					x.typ_ = x.typ().(*Signature).results.vars[0].typ
				}
			case *Builtin:
				x.mode_ = builtin
				x.typ_ = exp.typ
				x.id = exp.id
			default:
				check.dump("%v: unexpected object %v", atPos(e.Sel), exp)
				panic("unreachable")
			}
			x.expr = e
			return
		}
	}

	check.exprOrType(x, e.X, false)
	switch x.mode() {
	case builtin:
		check.errorf(e.Pos(), UncalledBuiltin, "invalid use of %s in selector expression", x)
		goto Error
	case invalid:
		goto Error
	}

	// We cannot select on an incomplete type; make sure it's complete.
	if !check.isComplete(x.typ()) {
		goto Error
	}

	// Avoid crashing when checking an invalid selector in a method declaration.
	//
	//   type S[T any] struct{}
	//   type V = S[any]
	//   func (fs *S[T]) M(x V.M) {}
	//
	// All codepaths below return a non-type expression. If we get here while
	// expecting a type expression, it is an error.
	//
	// See go.dev/issue/57522 for more details.
	if wantType {
		check.errorf(e.Sel, NotAType, "%s is not a type", syntax.Expr(e))
		goto Error
	}

	// Additionally, if x.typ is a pointer type, selecting implicitly dereferences the value, meaning
	// its base type must also be complete.
	if p, ok := x.typ().Underlying().(*Pointer); ok && !check.isComplete(p.base) {
		goto Error
	}

	obj, index, indirect = lookupFieldOrMethod(x.typ(), x.mode() == variable, check.pkg, sel, false)
	if obj == nil {
		// Don't report another error if the underlying type was invalid (go.dev/issue/49541).
		if !isValid(x.typ().Underlying()) {
			goto Error
		}

		if index != nil {
			// TODO(gri) should provide actual type where the conflict happens
			check.errorf(e.Sel, AmbiguousSelector, "ambiguous selector %s.%s", x.expr, sel)
			goto Error
		}

		if indirect {
			if x.mode() == typexpr {
				check.errorf(e.Sel, InvalidMethodExpr, "invalid method expression %s.%s (needs pointer receiver (*%s).%s)", x.typ(), sel, x.typ(), sel)
			} else {
				check.errorf(e.Sel, InvalidMethodExpr, "cannot call pointer method %s on %s", sel, x.typ())
			}
			goto Error
		}

		var why string
		if isInterfacePtr(x.typ()) {
			why = check.interfacePtrError(x.typ())
		} else {
			alt, _, _ := lookupFieldOrMethod(x.typ(), x.mode() == variable, check.pkg, sel, true)
			why = check.lookupError(x.typ(), sel, alt, false)
		}
		check.errorf(e.Sel, MissingFieldOrMethod, "%s.%s undefined (%s)", x.expr, sel, why)
		goto Error
	}
	// obj != nil

	switch obj := obj.(type) {
	case *Var:
		if x.mode() == typexpr {
			check.errorf(e.X, MissingFieldOrMethod, "operand for field selector %s must be value of type %s", sel, x.typ())
			goto Error
		}

		// field value
		check.recordSelection(e, FieldVal, x.typ(), obj, index, indirect)
		if x.mode() == variable || indirect {
			x.mode_ = variable
		} else {
			x.mode_ = value
		}
		x.typ_ = obj.typ

	case *Func:
		check.objDecl(obj) // ensure fully set-up signature
		check.addDeclDep(obj)
		// TODO(mark): Assert that sig.rparams is nil here?

		if x.mode() == typexpr {
			// method expression
			check.recordSelection(e, MethodExpr, x.typ(), obj, index, indirect)

			sig := obj.typ.(*Signature)
			if sig.recv == nil {
				check.error(e, InvalidDeclCycle, "illegal cycle in method declaration")
				goto Error
			}

			// The receiver type becomes the type of the first function
			// argument of the method expression's function type.
			var params []*Var
			if sig.params != nil {
				params = sig.params.vars
			}
			// Be consistent about named/unnamed parameters. This is not needed
			// for type-checking, but the newly constructed signature may appear
			// in an error message and then have mixed named/unnamed parameters.
			// (An alternative would be to not print parameter names in errors,
			// but it's useful to see them; this is cheap and method expressions
			// are rare.)
			name := ""
			if len(params) > 0 && params[0].name != "" {
				// name needed
				name = sig.recv.name
				if name == "" {
					name = "_"
				}
			}
			params = append([]*Var{NewParam(sig.recv.pos, sig.recv.pkg, name, x.typ())}, params...)
			x.mode_ = value
			x.typ_ = &Signature{
				tparams:  sig.tparams,
				recvold:  methodExprSentinel,
				params:   NewTuple(params...),
				results:  sig.results,
				variadic: sig.variadic,
			}
		} else {
			// method value

			// TODO(gri) If we needed to take into account the receiver's
			// addressability, should we report the type &(x.typ) instead?
			check.recordSelection(e, MethodVal, x.typ(), obj, index, indirect)

			x.mode_ = value

			// remove/stash receiver
			sig := *obj.typ.(*Signature)
			sig.recvold = sig.recv
			sig.recv = nil
			x.typ_ = &sig
		}

	default:
		panic("unreachable")
	}

	// everything went well
	x.expr = e
	return

Error:
	x.invalidate()
	x.typ_ = Typ[Invalid]
	x.expr = e
}

// use type-checks each argument.
// Useful to make sure expressions are evaluated
// (and variables are "used") in the presence of
// other errors. Arguments may be nil.
// Reports if all arguments evaluated without error.
func (check *Checker) use(args ...syntax.Expr) bool { return check.useN(args, false) }

// useLHS is like use, but doesn't "use" top-level identifiers.
// It should be called instead of use if the arguments are
// expressions on the lhs of an assignment.
func (check *Checker) useLHS(args ...syntax.Expr) bool { return check.useN(args, true) }

func (check *Checker) useN(args []syntax.Expr, lhs bool) bool {
	ok := true
	for _, e := range args {
		if !check.use1(e, lhs) {
			ok = false
		}
	}
	return ok
}

func (check *Checker) use1(e syntax.Expr, lhs bool) bool {
	var x operand
	x.mode_ = value // anything but invalid
	switch n := syntax.Unparen(e).(type) {
	case nil:
		// nothing to do
	case *syntax.Name:
		// don't report an error evaluating blank
		if n.Value == "_" {
			break
		}
		// If the lhs is an identifier denoting a variable v, this assignment
		// is not a 'use' of v. Remember current value of v.used and restore
		// after evaluating the lhs via check.rawExpr.
		var v *Var
		var v_used bool
		if lhs {
			if obj := check.lookup(n.Value); obj != nil {
				// It's ok to mark non-local variables, but ignore variables
				// from other packages to avoid potential race conditions with
				// dot-imported variables.
				if w, _ := obj.(*Var); w != nil && w.pkg == check.pkg {
					v = w
					v_used = check.usedVars[v]
				}
			}
		}
		check.exprOrType(&x, n, true)
		if v != nil {
			check.usedVars[v] = v_used // restore v.used
		}
	case *syntax.ListExpr:
		return check.useN(n.ElemList, lhs)
	default:
		check.rawExpr(nil, &x, e, nil, true)
	}
	return x.isValid()
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/chan.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

// A Chan represents a channel type.
type Chan struct {
	dir  ChanDir
	elem Type
}

// A ChanDir value indicates a channel direction.
type ChanDir int

// The direction of a channel is indicated by one of these constants.
const (
	SendRecv ChanDir = iota
	SendOnly
	RecvOnly
)

// NewChan returns a new channel type for the given direction and element type.
func NewChan(dir ChanDir, elem Type) *Chan {
	return &Chan{dir: dir, elem: elem}
}

// Dir returns the direction of channel c.
func (c *Chan) Dir() ChanDir { return c.dir }

// Elem returns the element type of channel c.
func (c *Chan) Elem() Type { return c.elem }

func (c *Chan) Underlying() Type { return c }
func (c *Chan) String() string   { return TypeString(c, nil) }

```

// === FILE: references!/go/src/cmd/compile/internal/types2/check.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements the Check function, which drives type-checking.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	. "internal/types/errors"
	"os"
)

// nopos indicates an unknown position
var nopos syntax.Pos

// debugging/development support
const debug = false // leave on during development

// position tracing for panics during type checking
const tracePos = true

// exprInfo stores information about an untyped expression.
type exprInfo struct {
	isLhs bool // expression is lhs operand of a shift with delayed type-check
	mode  operandMode
	typ   *Basic
	val   constant.Value // constant value; or nil (if not a constant)
}

// An environment represents the environment within which an object is
// type-checked.
type environment struct {
	decl          *declInfo                 // package-level declaration whose init expression/function body is checked
	scope         *Scope                    // top-most scope for lookups
	version       goVersion                 // current accepted language version; changes across files
	iota          constant.Value            // value of iota in a constant declaration; nil otherwise
	errpos        syntax.Pos                // if valid, identifier position of a constant with inherited initializer
	inTParamList  bool                      // set if inside a type parameter list
	sig           *Signature                // function signature if inside a function; nil otherwise
	isPanic       map[*syntax.CallExpr]bool // set of panic call expressions (used for termination check)
	hasLabel      bool                      // set if a function makes use of labels (only ~1% of functions); unused outside functions
	hasCallOrRecv bool                      // set if an expression contains a function call or channel receive operation
}

// lookupScope looks up name in the current environment and if an object
// is found it returns the scope containing the object and the object.
// Otherwise it returns (nil, nil).
//
// Note that obj.Parent() may be different from the returned scope if the
// object was inserted into the scope and already had a parent at that
// time (see Scope.Insert). This can only happen for dot-imported objects
// whose parent is the scope of the package that exported them.
func (env *environment) lookupScope(name string) (*Scope, Object) {
	for s := env.scope; s != nil; s = s.parent {
		if obj := s.Lookup(name); obj != nil {
			return s, obj
		}
	}
	return nil, nil
}

// lookup is like lookupScope but it only returns the object (or nil).
func (env *environment) lookup(name string) Object {
	_, obj := env.lookupScope(name)
	return obj
}

// An importKey identifies an imported package by import path and source directory
// (directory containing the file containing the import). In practice, the directory
// may always be the same, or may not matter. Given an (import path, directory), an
// importer must always return the same package (but given two different import paths,
// an importer may still return the same package by mapping them to the same package
// paths).
type importKey struct {
	path, dir string
}

// A dotImportKey describes a dot-imported object in the given scope.
type dotImportKey struct {
	scope *Scope
	name  string
}

// An action describes a (delayed) action.
type action struct {
	version goVersion   // applicable language version
	f       func()      // action to be executed
	desc    *actionDesc // action description; may be nil, requires debug to be set
}

// If debug is set, describef sets a printf-formatted description for action a.
// Otherwise, it is a no-op.
func (a *action) describef(pos poser, format string, args ...any) {
	if debug {
		a.desc = &actionDesc{pos, format, args}
	}
}

// An actionDesc provides information on an action.
// For debugging only.
type actionDesc struct {
	pos    poser
	format string
	args   []any
}

// A Checker maintains the state of the type checker.
// It must be created with NewChecker.
type Checker struct {
	// package information
	// (initialized by NewChecker, valid for the life-time of checker)
	conf *Config
	ctxt *Context // context for de-duplicating instances
	pkg  *Package
	*Info
	nextID  uint64                 // unique Id for type parameters (first valid Id is 1)
	objMap  map[Object]*declInfo   // maps package-level objects and (non-interface) methods to declaration info
	objList []Object               // source-ordered keys of objMap
	impMap  map[importKey]*Package // maps (import path, source directory) to (complete or fake) package
	// see TODO in validtype.go
	// valids  instanceLookup      // valid *Named (incl. instantiated) types per the validType check

	// pkgPathMap maps package names to the set of distinct import paths we've
	// seen for that name, anywhere in the import graph. It is used for
	// disambiguating package names in error messages.
	//
	// pkgPathMap is allocated lazily, so that we don't pay the price of building
	// it on the happy path. seenPkgMap tracks the packages that we've already
	// walked.
	pkgPathMap map[string]map[string]bool
	seenPkgMap map[*Package]bool

	// information collected during type-checking of a set of package files
	// (initialized by Files, valid only for the duration of check.Files;
	// maps and lists are allocated on demand)
	files         []*syntax.File             // list of package files
	versions      map[*syntax.PosBase]string // maps files to version strings (each file has an entry); shared with Info.FileVersions if present; may be unaltered Config.GoVersion
	imports       []*PkgName                 // list of imported packages
	dotImportMap  map[dotImportKey]*PkgName  // maps dot-imported objects to the package they were dot-imported through
	brokenAliases map[*TypeName]bool         // set of aliases with broken (not yet determined) types
	unionTypeSets map[*Union]*_TypeSet       // computed type sets for union types
	usedVars      map[*Var]bool              // set of used variables
	usedPkgNames  map[*PkgName]bool          // set of used package names
	mono          monoGraph                  // graph for detecting non-monomorphizable instantiation loops

	firstErr   error                    // first error encountered
	methods    map[*TypeName][]*Func    // maps package scope type names to associated non-blank (non-interface) methods
	untyped    map[syntax.Expr]exprInfo // map of expressions without final type
	delayed    []action                 // stack of delayed action segments; segments are processed in FIFO order
	objPath    []Object                 // path of object dependencies during type-checking (for cycle reporting)
	objPathIdx map[Object]int           // map of object to object path index during type-checking (for cycle reporting)
	cleaners   []cleaner                // list of types that may need a final cleanup at the end of type-checking

	// environment within which the current object is type-checked (valid only
	// for the duration of type-checking a specific object)
	environment

	// debugging
	posStack []syntax.Pos // stack of source positions seen; used for panic tracing
	indent   int          // indentation for tracing
}

// addDeclDep adds the dependency edge (check.decl -> to) if check.decl exists
func (check *Checker) addDeclDep(to Object) {
	from := check.decl
	if from == nil {
		return // not in a package-level init expression
	}
	if _, found := check.objMap[to]; !found {
		return // to is not a package-level object
	}
	from.addDep(to)
}

func (check *Checker) rememberUntyped(e syntax.Expr, lhs bool, mode operandMode, typ *Basic, val constant.Value) {
	m := check.untyped
	if m == nil {
		m = make(map[syntax.Expr]exprInfo)
		check.untyped = m
	}
	m[e] = exprInfo{lhs, mode, typ, val}
}

// later pushes f on to the stack of actions that will be processed later;
// either at the end of the current statement, or in case of a local constant
// or variable declaration, before the constant or variable is in scope
// (so that f still sees the scope before any new declarations).
// later returns the pushed action so one can provide a description
// via action.describef for debugging, if desired.
func (check *Checker) later(f func()) *action {
	i := len(check.delayed)
	check.delayed = append(check.delayed, action{version: check.version, f: f})
	return &check.delayed[i]
}

// push pushes obj onto the object path and records its index in the path index map.
func (check *Checker) push(obj Object) {
	if check.objPathIdx == nil {
		check.objPathIdx = make(map[Object]int)
	}
	check.objPathIdx[obj] = len(check.objPath)
	check.objPath = append(check.objPath, obj)
}

// pop pops an object from the object path and removes it from the path index map.
func (check *Checker) pop() {
	i := len(check.objPath) - 1
	obj := check.objPath[i]
	check.objPath[i] = nil // help the garbage collector
	check.objPath = check.objPath[:i]
	delete(check.objPathIdx, obj)
}

type cleaner interface {
	cleanup()
}

// needsCleanup records objects/types that implement the cleanup method
// which will be called at the end of type-checking.
func (check *Checker) needsCleanup(c cleaner) {
	check.cleaners = append(check.cleaners, c)
}

// NewChecker returns a new Checker instance for a given package.
// Package files may be added incrementally via checker.Files.
func NewChecker(conf *Config, pkg *Package, info *Info) *Checker {
	// make sure we have a configuration
	if conf == nil {
		conf = new(Config)
	}

	// make sure we have an info struct
	if info == nil {
		info = new(Info)
	}

	// Note: clients may call NewChecker with the Unsafe package, which is
	// globally shared and must not be mutated. Therefore NewChecker must not
	// mutate *pkg.
	//
	// (previously, pkg.goVersion was mutated here: go.dev/issue/61212)

	return &Checker{
		conf:         conf,
		ctxt:         conf.Context,
		pkg:          pkg,
		Info:         info,
		objMap:       make(map[Object]*declInfo),
		impMap:       make(map[importKey]*Package),
		usedVars:     make(map[*Var]bool),
		usedPkgNames: make(map[*PkgName]bool),
	}
}

// initFiles initializes the files-specific portion of checker.
// The provided files must all belong to the same package.
func (check *Checker) initFiles(files []*syntax.File) {
	// start with a clean slate (check.Files may be called multiple times)
	// TODO(gri): what determines which fields are zeroed out here, vs at the end
	// of checkFiles?
	check.files = nil
	check.imports = nil
	check.dotImportMap = nil

	check.firstErr = nil
	check.methods = nil
	check.untyped = nil
	check.delayed = nil
	check.objPath = nil
	check.objPathIdx = nil
	check.cleaners = nil

	// We must initialize usedVars and usedPkgNames both here and in NewChecker,
	// because initFiles is not called in the CheckExpr or Eval codepaths, yet we
	// want to free this memory at the end of Files ('used' predicates are
	// only needed in the context of a given file).
	check.usedVars = make(map[*Var]bool)
	check.usedPkgNames = make(map[*PkgName]bool)

	// determine package name and collect valid files
	pkg := check.pkg
	for _, file := range files {
		switch name := file.PkgName.Value; pkg.name {
		case "":
			if name != "_" {
				pkg.name = name
			} else {
				check.error(file.PkgName, BlankPkgName, "invalid package name _")
			}
			fallthrough

		case name:
			check.files = append(check.files, file)

		default:
			check.errorf(file, MismatchedPkgName, "package %s; expected package %s", name, pkg.name)
			// ignore this file
		}
	}

	// reuse Info.FileVersions if provided
	versions := check.Info.FileVersions
	if versions == nil {
		versions = make(map[*syntax.PosBase]string)
	}
	check.versions = versions

	pkgVersion := asGoVersion(check.conf.GoVersion)
	if pkgVersion.isValid() && len(files) > 0 && pkgVersion.cmp(go_current) > 0 {
		check.errorf(files[0], TooNew, "package requires newer Go version %v (application built with %v)",
			pkgVersion, go_current)
	}

	// determine Go version for each file
	for _, file := range check.files {
		// use unaltered Config.GoVersion by default
		// (This version string may contain dot-release numbers as in go1.20.1,
		// unlike file versions which are Go language versions only, if valid.)
		v := check.conf.GoVersion

		// If the file specifies a version, use max(fileVersion, go1.21).
		if fileVersion := asGoVersion(file.GoVersion); fileVersion.isValid() {
			// Go 1.21 introduced the feature of allowing //go:build lines
			// to sometimes set the Go version in a given file. Versions Go 1.21 and later
			// can be set backwards compatibly as that was the first version
			// files with go1.21 or later build tags could be built with.
			//
			// Set the version to max(fileVersion, go1.21): That will allow a
			// downgrade to a version before go1.22, where the for loop semantics
			// change was made, while being backwards compatible with versions of
			// go before the new //go:build semantics were introduced.
			v = string(versionMax(fileVersion, go1_21))

			// Report a specific error for each tagged file that's too new.
			// (Normally the build system will have filtered files by version,
			// but clients can present arbitrary files to the type checker.)
			if fileVersion.cmp(go_current) > 0 {
				// Use position of 'package [p]' for types/types2 consistency.
				// (Ideally we would use the //build tag itself.)
				check.errorf(file.PkgName, TooNew, "file requires newer Go version %v", fileVersion)
			}
		}
		versions[file.Pos().FileBase()] = v // file.Pos().FileBase() may be nil for tests
	}
}

func versionMax(a, b goVersion) goVersion {
	if a.cmp(b) > 0 {
		return a
	}
	return b
}

// pushPos pushes pos onto the pos stack.
func (check *Checker) pushPos(pos syntax.Pos) {
	check.posStack = append(check.posStack, pos)
}

// popPos pops from the pos stack.
func (check *Checker) popPos() {
	check.posStack = check.posStack[:len(check.posStack)-1]
}

// A bailout panic is used for early termination.
type bailout struct{}

func (check *Checker) handleBailout(err *error) {
	switch p := recover().(type) {
	case nil, bailout:
		// normal return or early exit
		*err = check.firstErr
	default:
		if len(check.posStack) > 0 {
			doPrint := func(ps []syntax.Pos) {
				for i := len(ps) - 1; i >= 0; i-- {
					fmt.Fprintf(os.Stderr, "\t%v\n", ps[i])
				}
			}

			fmt.Fprintln(os.Stderr, "The following panic happened checking types near:")
			if len(check.posStack) <= 10 {
				doPrint(check.posStack)
			} else {
				// if it's long, truncate the middle; it's least likely to help
				doPrint(check.posStack[len(check.posStack)-5:])
				fmt.Fprintln(os.Stderr, "\t...")
				doPrint(check.posStack[:5])
			}
		}

		// re-panic
		panic(p)
	}
}

// Files checks the provided files as part of the checker's package.
func (check *Checker) Files(files []*syntax.File) (err error) {
	if check.pkg == Unsafe {
		// Defensive handling for Unsafe, which cannot be type checked, and must
		// not be mutated. See https://go.dev/issue/61212 for an example of where
		// Unsafe is passed to NewChecker.
		return nil
	}

	// Avoid early returns here! Nearly all errors can be
	// localized to a piece of syntax and needn't prevent
	// type-checking of the rest of the package.

	defer check.handleBailout(&err)
	check.checkFiles(files)
	return
}

// checkFiles type-checks the specified files. Errors are reported as
// a side effect, not by returning early, to ensure that well-formed
// syntax is properly type annotated even in a package containing
// errors.
func (check *Checker) checkFiles(files []*syntax.File) {
	print := func(msg string) {
		if check.conf.Trace {
			fmt.Println()
			fmt.Println(msg)
		}
	}

	print("== initFiles ==")
	check.initFiles(files)

	print("== collectObjects ==")
	check.collectObjects()

	print("== sortObjects ==")
	check.sortObjects()

	print("== directCycles ==")
	check.directCycles()

	print("== packageObjects ==")
	check.packageObjects()

	print("== processDelayed ==")
	check.processDelayed(0) // incl. all functions

	print("== cleanup ==")
	check.cleanup()

	print("== initOrder ==")
	check.initOrder()

	if !check.conf.DisableUnusedImportCheck {
		print("== unusedImports ==")
		check.unusedImports()
	}

	print("== recordUntyped ==")
	check.recordUntyped()

	if check.firstErr == nil {
		// TODO(mdempsky): Ensure monomorph is safe when errors exist.
		check.monomorph()
	}

	check.pkg.goVersion = check.conf.GoVersion
	check.pkg.complete = true

	// no longer needed - release memory
	check.imports = nil
	check.dotImportMap = nil
	check.pkgPathMap = nil
	check.seenPkgMap = nil
	check.brokenAliases = nil
	check.unionTypeSets = nil
	check.usedVars = nil
	check.usedPkgNames = nil
	check.ctxt = nil

	// TODO(gri): shouldn't the cleanup above occur after the bailout?
	// TODO(gri) There's more memory we should release at this point.
}

// processDelayed processes all delayed actions pushed after top.
func (check *Checker) processDelayed(top int) {
	// If each delayed action pushes a new action, the
	// stack will continue to grow during this loop.
	// However, it is only processing functions (which
	// are processed in a delayed fashion) that may
	// add more actions (such as nested functions), so
	// this is a sufficiently bounded process.
	savedVersion := check.version
	for i := top; i < len(check.delayed); i++ {
		a := &check.delayed[i]
		if check.conf.Trace {
			if a.desc != nil {
				check.trace(a.desc.pos.Pos(), "-- "+a.desc.format, a.desc.args...)
			} else {
				check.trace(nopos, "-- delayed %p", a.f)
			}
		}
		check.version = a.version // reestablish the effective Go version captured earlier
		a.f()                     // may append to check.delayed
		if check.conf.Trace {
			fmt.Println()
		}
	}
	assert(top <= len(check.delayed)) // stack must not have shrunk
	check.delayed = check.delayed[:top]
	check.version = savedVersion
}

// cleanup runs cleanup for all collected cleaners.
func (check *Checker) cleanup() {
	// Don't use a range clause since Named.cleanup may add more cleaners.
	for i := 0; i < len(check.cleaners); i++ {
		check.cleaners[i].cleanup()
	}
	check.cleaners = nil
}

// types2-specific support for recording type information in the syntax tree.
func (check *Checker) recordTypeAndValueInSyntax(x syntax.Expr, mode operandMode, typ Type, val constant.Value) {
	if check.StoreTypesInSyntax {
		tv := TypeAndValue{mode, typ, val}
		stv := syntax.TypeAndValue{Type: typ, Value: val}
		if tv.IsVoid() {
			stv.SetIsVoid()
		}
		if tv.IsType() {
			stv.SetIsType()
		}
		if tv.IsBuiltin() {
			stv.SetIsBuiltin()
		}
		if tv.IsValue() {
			stv.SetIsValue()
		}
		if tv.IsNil() {
			stv.SetIsNil()
		}
		if tv.Addressable() {
			stv.SetAddressable()
		}
		if tv.Assignable() {
			stv.SetAssignable()
		}
		if tv.HasOk() {
			stv.SetHasOk()
		}
		x.SetTypeInfo(stv)
	}
}

// types2-specific support for recording type information in the syntax tree.
func (check *Checker) recordCommaOkTypesInSyntax(x syntax.Expr, t0, t1 Type) {
	if check.StoreTypesInSyntax {
		// Note: this loop is duplicated because the type of tv is different.
		// Above it is types2.TypeAndValue, here it is syntax.TypeAndValue.
		for {
			tv := x.GetTypeInfo()
			assert(tv.Type != nil) // should have been recorded already
			pos := x.Pos()
			tv.Type = NewTuple(
				NewParam(pos, check.pkg, "", t0),
				NewParam(pos, check.pkg, "", t1),
			)
			x.SetTypeInfo(tv)
			p, _ := x.(*syntax.ParenExpr)
			if p == nil {
				break
			}
			x = p.X
		}
	}
}

// instantiatedIdent determines the identifier of the type instantiated in expr.
// Helper function for recordInstance in recording.go.
func instantiatedIdent(expr syntax.Expr) *syntax.Name {
	var selOrIdent syntax.Expr
	switch e := expr.(type) {
	case *syntax.IndexExpr:
		selOrIdent = e.X
	case *syntax.SelectorExpr, *syntax.Name:
		selOrIdent = e
	}
	switch x := selOrIdent.(type) {
	case *syntax.Name:
		return x
	case *syntax.SelectorExpr:
		return x.Sel
	}

	// extra debugging of go.dev/issue/63933
	panic(sprintf(nil, true, "instantiated ident not found; please report: %s", expr))
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/compiler_internal.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
)

// This file should not be copied to go/types.  See go.dev/issue/67477

// RenameResult takes an array of (result) fields and an index, and if the indexed field
// does not have a name and if the result in the signature also does not have a name,
// then the signature and field are renamed to
//
//	fmt.Sprintf("#rv%d", i+1)
//
// the newly named object is inserted into the signature's scope,
// and the object and new field name are returned.
//
// The intended use for RenameResult is to allow rangefunc to assign results within a closure.
// This is a hack, as narrowly targeted as possible to discourage abuse.
func (s *Signature) RenameResult(results []*syntax.Field, i int) (*Var, *syntax.Name) {
	a := results[i]
	obj := s.Results().At(i)

	if !(obj.name == "" || obj.name == "_" && a.Name == nil || a.Name.Value == "_") {
		panic("Cannot change an existing name")
	}

	pos := a.Pos()
	typ := a.Type.GetTypeInfo().Type

	name := fmt.Sprintf("#rv%d", i+1)
	obj.name = name
	s.scope.Insert(obj)
	obj.setScopePos(pos)

	tv := syntax.TypeAndValue{Type: typ}
	tv.SetIsValue()

	n := syntax.NewName(pos, obj.Name())
	n.SetTypeInfo(tv)

	a.Name = n

	return obj, n
}

// Comment returns the scope comment, for debugging purposes.
func (s *Scope) Comment() string { return s.comment }

```

// === FILE: references!/go/src/cmd/compile/internal/types2/compilersupport.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Helper functions exported for the compiler.
// Do not use internally.

package types2

// If t is a pointer, AsPointer returns that type, otherwise it returns nil.
func AsPointer(t Type) *Pointer {
	u, _ := t.Underlying().(*Pointer)
	return u
}

// If typ is a type parameter, CoreType returns the single underlying
// type of all types in the corresponding type constraint if it exists, or
// nil otherwise. If the type set contains only unrestricted and restricted
// channel types (with identical element types), the single underlying type
// is the restricted channel type if the restrictions are always the same.
// If typ is not a type parameter, CoreType returns the underlying type.
func CoreType(t Type) Type {
	u, _ := commonUnder(t, nil)
	return u
}

// RangeKeyVal returns the key and value types for a range over typ.
// It panics if range over typ is invalid.
func RangeKeyVal(typ Type) (Type, Type) {
	key, val, _, ok := rangeKeyVal(nil, typ, nil)
	assert(ok)
	return key, val
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/const.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements functions for untyped constant operands.

package types2

import (
	"cmd/compile/internal/syntax"
	"go/constant"
	"go/token"
	. "internal/types/errors"
	"math"
)

// overflow checks that the constant x is representable by its type.
// For untyped constants, it checks that the value doesn't become
// arbitrarily large.
func (check *Checker) overflow(x *operand, opPos syntax.Pos) {
	assert(x.mode() == constant_)

	if x.val.Kind() == constant.Unknown {
		// TODO(gri) We should report exactly what went wrong. At the
		//           moment we don't have the (go/constant) API for that.
		//           See also TODO in go/constant/value.go.
		check.error(atPos(opPos), InvalidConstVal, "constant result is not representable")
		return
	}

	// Typed constants must be representable in
	// their type after each constant operation.
	// x.typ cannot be a type parameter (type
	// parameters cannot be constant types).
	if isTyped(x.typ()) {
		check.representable(x, x.typ().Underlying().(*Basic))
		return
	}

	// Untyped integer values must not grow arbitrarily.
	const prec = 512 // 512 is the constant precision
	if x.val.Kind() == constant.Int && constant.BitLen(x.val) > prec {
		op := opName(x.expr)
		if op != "" {
			op += " "
		}
		check.errorf(atPos(opPos), InvalidConstVal, "constant %soverflow", op)
		x.val = constant.MakeUnknown()
		return
	}

	// String values must not become arbitrarily long (go.dev/issue/78346).
	const maxLen = int64(2e9) // cmd/internal/obj.MaxSymSize
	if x.val.Kind() == constant.String {
		len := constant.StringLen(x.val)
		if len > maxLen {
			check.errorf(atPos(opPos), InvalidConstVal, "constant string too long (%d bytes > %d bytes)", len, maxLen)
			x.val = constant.MakeUnknown()
			return
		}
	}
}

// representableConst reports whether x can be represented as
// value of the given basic type and for the configuration
// provided (only needed for int/uint sizes).
//
// If rounded != nil, *rounded is set to the rounded value of x for
// representable floating-point and complex values, and to an Int
// value for integer values; it is left alone otherwise.
// It is ok to provide the addressof the first argument for rounded.
//
// The check parameter may be nil if representableConst is invoked
// (indirectly) through an exported API call (AssignableTo, ConvertibleTo)
// because we don't need the Checker's config for those calls.
func representableConst(x constant.Value, check *Checker, typ *Basic, rounded *constant.Value) bool {
	if x.Kind() == constant.Unknown {
		return true // avoid follow-up errors
	}

	var conf *Config
	if check != nil {
		conf = check.conf
	}

	sizeof := func(T Type) int64 {
		s := conf.sizeof(T)
		return s
	}

	switch {
	case isInteger(typ):
		x := constant.ToInt(x)
		if x.Kind() != constant.Int {
			return false
		}
		if rounded != nil {
			*rounded = x
		}
		if x, ok := constant.Int64Val(x); ok {
			switch typ.kind {
			case Int:
				var s = uint(sizeof(typ)) * 8
				return int64(-1)<<(s-1) <= x && x <= int64(1)<<(s-1)-1
			case Int8:
				const s = 8
				return -1<<(s-1) <= x && x <= 1<<(s-1)-1
			case Int16:
				const s = 16
				return -1<<(s-1) <= x && x <= 1<<(s-1)-1
			case Int32:
				const s = 32
				return -1<<(s-1) <= x && x <= 1<<(s-1)-1
			case Int64, UntypedInt:
				return true
			case Uint, Uintptr:
				if s := uint(sizeof(typ)) * 8; s < 64 {
					return 0 <= x && x <= int64(1)<<s-1
				}
				return 0 <= x
			case Uint8:
				const s = 8
				return 0 <= x && x <= 1<<s-1
			case Uint16:
				const s = 16
				return 0 <= x && x <= 1<<s-1
			case Uint32:
				const s = 32
				return 0 <= x && x <= 1<<s-1
			case Uint64:
				return 0 <= x
			default:
				panic("unreachable")
			}
		}
		// x does not fit into int64
		switch n := constant.BitLen(x); typ.kind {
		case Uint, Uintptr:
			var s = uint(sizeof(typ)) * 8
			return constant.Sign(x) >= 0 && n <= int(s)
		case Uint64:
			return constant.Sign(x) >= 0 && n <= 64
		case UntypedInt:
			return true
		}

	case isFloat(typ):
		x := constant.ToFloat(x)
		if x.Kind() != constant.Float {
			return false
		}
		switch typ.kind {
		case Float32:
			if rounded == nil {
				return fitsFloat32(x)
			}
			r := roundFloat32(x)
			if r != nil {
				*rounded = r
				return true
			}
		case Float64:
			if rounded == nil {
				return fitsFloat64(x)
			}
			r := roundFloat64(x)
			if r != nil {
				*rounded = r
				return true
			}
		case UntypedFloat:
			return true
		default:
			panic("unreachable")
		}

	case isComplex(typ):
		x := constant.ToComplex(x)
		if x.Kind() != constant.Complex {
			return false
		}
		switch typ.kind {
		case Complex64:
			if rounded == nil {
				return fitsFloat32(constant.Real(x)) && fitsFloat32(constant.Imag(x))
			}
			re := roundFloat32(constant.Real(x))
			im := roundFloat32(constant.Imag(x))
			if re != nil && im != nil {
				*rounded = constant.BinaryOp(re, token.ADD, constant.MakeImag(im))
				return true
			}
		case Complex128:
			if rounded == nil {
				return fitsFloat64(constant.Real(x)) && fitsFloat64(constant.Imag(x))
			}
			re := roundFloat64(constant.Real(x))
			im := roundFloat64(constant.Imag(x))
			if re != nil && im != nil {
				*rounded = constant.BinaryOp(re, token.ADD, constant.MakeImag(im))
				return true
			}
		case UntypedComplex:
			return true
		default:
			panic("unreachable")
		}

	case isString(typ):
		return x.Kind() == constant.String

	case isBoolean(typ):
		return x.Kind() == constant.Bool
	}

	return false
}

func fitsFloat32(x constant.Value) bool {
	f32, _ := constant.Float32Val(x)
	f := float64(f32)
	return !math.IsInf(f, 0)
}

func roundFloat32(x constant.Value) constant.Value {
	f32, _ := constant.Float32Val(x)
	f := float64(f32)
	if !math.IsInf(f, 0) {
		return constant.MakeFloat64(f)
	}
	return nil
}

func fitsFloat64(x constant.Value) bool {
	f, _ := constant.Float64Val(x)
	return !math.IsInf(f, 0)
}

func roundFloat64(x constant.Value) constant.Value {
	f, _ := constant.Float64Val(x)
	if !math.IsInf(f, 0) {
		return constant.MakeFloat64(f)
	}
	return nil
}

// representable checks that a constant operand is representable in the given
// basic type.
func (check *Checker) representable(x *operand, typ *Basic) {
	v, code := check.representation(x, typ)
	if code != 0 {
		check.invalidConversion(code, x, typ)
		x.invalidate()
		return
	}
	assert(v != nil)
	x.val = v
}

// representation returns the representation of the constant operand x as the
// basic type typ.
//
// If no such representation is possible, it returns a non-zero error code.
func (check *Checker) representation(x *operand, typ *Basic) (constant.Value, Code) {
	assert(x.mode() == constant_)
	v := x.val
	if !representableConst(x.val, check, typ, &v) {
		if isNumeric(x.typ()) && isNumeric(typ) {
			// numeric conversion : error msg
			//
			// integer -> integer : overflows
			// integer -> float   : overflows (actually not possible)
			// float   -> integer : truncated
			// float   -> float   : overflows
			//
			if !isInteger(x.typ()) && isInteger(typ) {
				return nil, TruncatedFloat
			} else {
				return nil, NumericOverflow
			}
		}
		return nil, InvalidConstVal
	}
	return v, 0
}

func (check *Checker) invalidConversion(code Code, x *operand, target Type) {
	msg := "cannot convert %s to type %s"
	switch code {
	case TruncatedFloat:
		msg = "%s truncated to %s"
	case NumericOverflow:
		msg = "%s overflows %s"
	}
	check.errorf(x, code, msg, x, target)
}

// convertUntyped attempts to set the type of an untyped value to the target type.
func (check *Checker) convertUntyped(x *operand, target Type) {
	newType, val, code := check.implicitTypeAndValue(x, target)
	if code != 0 {
		t := target
		if !isTypeParam(target) {
			t = safeUnderlying(target)
		}
		check.invalidConversion(code, x, t)
		x.invalidate()
		return
	}
	if val != nil {
		x.val = val
		check.updateExprVal(x.expr, val)
	}
	if newType != x.typ() {
		x.typ_ = newType
		check.updateExprType(x.expr, newType, false)
	}
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/context.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// This file contains a definition of the type-checking context; an opaque type
// that may be supplied by users during instantiation.
//
// Contexts serve two purposes:
//  - reduce the duplication of identical instances
//  - short-circuit instantiation cycles
//
// For the latter purpose, we must always have a context during instantiation,
// whether or not it is supplied by the user. For both purposes, it must be the
// case that hashing a pointer-identical type produces consistent results
// (somewhat obviously).
//
// However, neither of these purposes require that our hash is perfect, and so
// this was not an explicit design goal of the context type. In fact, due to
// concurrent use it is convenient not to guarantee de-duplication.
//
// Nevertheless, in the future it could be helpful to allow users to leverage
// contexts to canonicalize instances, and it would probably be possible to
// achieve such a guarantee.

// A Context is an opaque type checking context. It may be used to share
// identical type instances across type-checked packages or calls to
// Instantiate. Contexts are safe for concurrent use.
//
// The use of a shared context does not guarantee that identical instances are
// deduplicated in all cases.
type Context struct {
	mu        sync.Mutex
	typeMap   map[string][]ctxtEntry // type hash -> instances entries
	nextID    int                    // next unique ID
	originIDs map[Type]int           // origin type -> unique ID
}

type ctxtEntry struct {
	orig     Type
	targs    []Type
	instance Type // = orig[targs]
}

// NewContext creates a new Context.
func NewContext() *Context {
	return &Context{
		typeMap:   make(map[string][]ctxtEntry),
		originIDs: make(map[Type]int),
	}
}

// instanceHash returns a string representation of typ instantiated with targs.
// The hash should be a perfect hash, though out of caution the type checker
// does not assume this. The result is guaranteed to not contain blanks.
func (ctxt *Context) instanceHash(orig Type, targs []Type) string {
	assert(ctxt != nil)
	assert(orig != nil)
	var buf bytes.Buffer

	h := newTypeHasher(&buf, ctxt)
	h.string(strconv.Itoa(ctxt.getID(orig)))
	// Because we've already written the unique origin ID this call to h.typ is
	// unnecessary, but we leave it for hash readability. It can be removed later
	// if performance is an issue.
	h.typ(orig)
	if len(targs) > 0 {
		// TODO(rfindley): consider asserting on isGeneric(typ) here, if and when
		// isGeneric handles *Signature types.
		h.typeList(targs)
	}

	return strings.ReplaceAll(buf.String(), " ", "#")
}

// lookup returns an existing instantiation of orig with targs, if it exists.
// Otherwise, it returns nil.
func (ctxt *Context) lookup(h string, orig Type, targs []Type) Type {
	ctxt.mu.Lock()
	defer ctxt.mu.Unlock()

	for _, e := range ctxt.typeMap[h] {
		if identicalInstance(orig, targs, e.orig, e.targs) {
			return e.instance
		}
		if debug {
			// Panic during development to surface any imperfections in our hash.
			panic(fmt.Sprintf("non-identical instances: (orig: %s, targs: %v) and %s", orig, targs, e.instance))
		}
	}

	return nil
}

// update de-duplicates inst against previously seen types with the hash h.
// If an identical type is found with the type hash h, the previously seen
// type is returned. Otherwise, inst is returned, and recorded in the Context
// for the hash h.
func (ctxt *Context) update(h string, orig Type, targs []Type, inst Type) Type {
	assert(inst != nil)

	ctxt.mu.Lock()
	defer ctxt.mu.Unlock()

	for _, e := range ctxt.typeMap[h] {
		if inst == nil || Identical(inst, e.instance) {
			return e.instance
		}
		if debug {
			// Panic during development to surface any imperfections in our hash.
			panic(fmt.Sprintf("%s and %s are not identical", inst, e.instance))
		}
	}

	ctxt.typeMap[h] = append(ctxt.typeMap[h], ctxtEntry{
		orig:     orig,
		targs:    targs,
		instance: inst,
	})

	return inst
}

// getID returns a unique ID for the type t.
func (ctxt *Context) getID(t Type) int {
	ctxt.mu.Lock()
	defer ctxt.mu.Unlock()
	id, ok := ctxt.originIDs[t]
	if !ok {
		id = ctxt.nextID
		ctxt.originIDs[t] = id
		ctxt.nextID++
	}
	return id
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/conversions.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of conversions.

package types2

import (
	"go/constant"
	. "internal/types/errors"
	"unicode"
)

// conversion type-checks the conversion T(x).
// The result is in x.
func (check *Checker) conversion(x *operand, T Type) {
	constArg := x.mode() == constant_

	constConvertibleTo := func(T Type, val *constant.Value) bool {
		switch t, _ := T.Underlying().(*Basic); {
		case t == nil:
			// nothing to do
		case representableConst(x.val, check, t, val):
			return true
		case isInteger(x.typ()) && isString(t):
			codepoint := unicode.ReplacementChar
			if i, ok := constant.Uint64Val(x.val); ok && i <= unicode.MaxRune {
				codepoint = rune(i)
			}
			if val != nil {
				*val = constant.MakeString(string(codepoint))
			}
			return true
		}
		return false
	}

	var ok bool
	var cause string
	switch {
	case constArg && isConstType(T):
		// constant conversion
		ok = constConvertibleTo(T, &x.val)
		// A conversion from an integer constant to an integer type
		// can only fail if there's overflow. Give a concise error.
		// (go.dev/issue/63563)
		if !ok && isInteger(x.typ()) && isInteger(T) {
			check.errorf(x, InvalidConversion, "constant %s overflows %s", x.val, T)
			x.invalidate()
			return
		}
	case constArg && isTypeParam(T):
		// x is convertible to T if it is convertible
		// to each specific type in the type set of T.
		// If T's type set is empty, or if it doesn't
		// have specific types, constant x cannot be
		// converted.
		ok = underIs(T, func(u Type) bool {
			// u is nil if there are no specific type terms
			if u == nil {
				cause = check.sprintf("%s does not contain specific types", T)
				return false
			}
			if isString(x.typ()) && isBytesOrRunes(u) {
				return true
			}
			if !constConvertibleTo(u, nil) {
				if isInteger(x.typ()) && isInteger(u) {
					// see comment above on constant conversion
					cause = check.sprintf("constant %s overflows %s (in %s)", x.val, u, T)
				} else {
					cause = check.sprintf("cannot convert %s to type %s (in %s)", x, u, T)
				}
				return false
			}
			return true
		})
		x.mode_ = value // type parameters are not constants
	case x.convertibleTo(check, T, &cause):
		// non-constant conversion
		ok = true
		x.mode_ = value
	}

	if !ok {
		if cause != "" {
			check.errorf(x, InvalidConversion, "cannot convert %s to type %s: %s", x, T, cause)
		} else {
			check.errorf(x, InvalidConversion, "cannot convert %s to type %s", x, T)
		}
		x.invalidate()
		return
	}

	// The conversion argument types are final. For untyped values the
	// conversion provides the type, per the spec: "A constant may be
	// given a type explicitly by a constant declaration or conversion,...".
	if isUntyped(x.typ()) {
		final := T
		// - For conversions to interfaces, except for untyped nil arguments
		//   and isTypes2, use the argument's default type.
		// - For conversions of untyped constants to non-constant types, also
		//   use the default type (e.g., []byte("foo") should report string
		//   not []byte as type for the constant "foo").
		// - If !isTypes2, keep untyped nil for untyped nil arguments.
		// - For constant integer to string conversions, keep the argument type.
		//   (See also the TODO below.)
		if isTypes2 && x.typ() == Typ[UntypedNil] {
			// ok
		} else if isNonTypeParamInterface(T) || constArg && !isConstType(T) || !isTypes2 && x.isNil() {
			final = Default(x.typ()) // default type of untyped nil is untyped nil
		} else if x.mode() == constant_ && isInteger(x.typ()) && allString(T) {
			final = x.typ()
		}
		check.updateExprType(x.expr, final, true)
	}

	x.typ_ = T
}

// TODO(gri) convertibleTo checks if T(x) is valid. It assumes that the type
// of x is fully known, but that's not the case for say string(1<<s + 1.0):
// Here, the type of 1<<s + 1.0 will be UntypedFloat which will lead to the
// (correct!) refusal of the conversion. But the reported error is essentially
// "cannot convert untyped float value to string", yet the correct error (per
// the spec) is that we cannot shift a floating-point value: 1 in 1<<s should
// be converted to UntypedFloat because of the addition of 1.0. Fixing this
// is tricky because we'd have to run updateExprType on the argument first.
// (go.dev/issue/21982.)

// convertibleTo reports whether T(x) is valid. In the failure case, *cause
// may be set to the cause for the failure.
// The check parameter may be nil if convertibleTo is invoked through an
// exported API call, i.e., when all methods have been type-checked.
func (x *operand) convertibleTo(check *Checker, T Type, cause *string) bool {
	// "x is assignable to T"
	if ok, _ := x.assignableTo(check, T, cause); ok {
		return true
	}

	origT := T
	V := Unalias(x.typ())
	T = Unalias(T)
	Vu := V.Underlying()
	Tu := T.Underlying()
	Vp, _ := V.(*TypeParam)
	Tp, _ := T.(*TypeParam)

	// "V and T have identical underlying types if tags are ignored
	// and V and T are not type parameters"
	if IdenticalIgnoreTags(Vu, Tu) && Vp == nil && Tp == nil {
		return true
	}

	// "V and T are unnamed pointer types and their pointer base types
	// have identical underlying types if tags are ignored
	// and their pointer base types are not type parameters"
	if V, ok := V.(*Pointer); ok {
		if T, ok := T.(*Pointer); ok {
			if IdenticalIgnoreTags(V.base.Underlying(), T.base.Underlying()) && !isTypeParam(V.base) && !isTypeParam(T.base) {
				return true
			}
		}
	}

	// "V and T are both integer or floating point types"
	if isIntegerOrFloat(Vu) && isIntegerOrFloat(Tu) {
		return true
	}

	// "V and T are both complex types"
	if isComplex(Vu) && isComplex(Tu) {
		return true
	}

	// "V is an integer or a slice of bytes or runes and T is a string type"
	if (isInteger(Vu) || isBytesOrRunes(Vu)) && isString(Tu) {
		return true
	}

	// "V is a string and T is a slice of bytes or runes"
	if isString(Vu) && isBytesOrRunes(Tu) {
		return true
	}

	// package unsafe:
	// "any pointer or value of underlying type uintptr can be converted into a unsafe.Pointer"
	if (isPointer(Vu) || isUintptr(Vu)) && isUnsafePointer(Tu) {
		return true
	}
	// "and vice versa"
	if isUnsafePointer(Vu) && (isPointer(Tu) || isUintptr(Tu)) {
		return true
	}

	// "V is a slice, T is an array or pointer-to-array type,
	// and the slice and array types have identical element types."
	if s, _ := Vu.(*Slice); s != nil {
		switch a := Tu.(type) {
		case *Array:
			if Identical(s.Elem(), a.Elem()) {
				if check == nil || check.allowVersion(go1_20) {
					return true
				}
				// check != nil
				if cause != nil {
					// TODO(gri) consider restructuring versionErrorf so we can use it here and below
					*cause = "conversion of slice to array requires go1.20 or later"
				}
				return false
			}
		case *Pointer:
			if a, _ := a.Elem().Underlying().(*Array); a != nil {
				if Identical(s.Elem(), a.Elem()) {
					if check == nil || check.allowVersion(go1_17) {
						return true
					}
					// check != nil
					if cause != nil {
						*cause = "conversion of slice to array pointer requires go1.17 or later"
					}
					return false
				}
			}
		}
	}

	// optimization: if we don't have type parameters, we're done
	if Vp == nil && Tp == nil {
		return false
	}

	errorf := func(format string, args ...any) {
		if check != nil && cause != nil {
			msg := check.sprintf(format, args...)
			if *cause != "" {
				msg += "\n\t" + *cause
			}
			*cause = msg
		}
	}

	// generic cases with specific type terms
	// (generic operands cannot be constants, so we can ignore x.val)
	switch {
	case Vp != nil && Tp != nil:
		x := *x // don't clobber outer x
		return Vp.is(func(V *term) bool {
			if V == nil {
				return false // no specific types
			}
			x.typ_ = V.typ
			return Tp.is(func(T *term) bool {
				if T == nil {
					return false // no specific types
				}
				if !x.convertibleTo(check, T.typ, cause) {
					errorf("cannot convert %s (in %s) to type %s (in %s)", V.typ, Vp, T.typ, Tp)
					return false
				}
				return true
			})
		})
	case Vp != nil:
		x := *x // don't clobber outer x
		return Vp.is(func(V *term) bool {
			if V == nil {
				return false // no specific types
			}
			x.typ_ = V.typ
			if !x.convertibleTo(check, T, cause) {
				errorf("cannot convert %s (in %s) to type %s", V.typ, Vp, origT)
				return false
			}
			return true
		})
	case Tp != nil:
		return Tp.is(func(T *term) bool {
			if T == nil {
				return false // no specific types
			}
			if !x.convertibleTo(check, T.typ, cause) {
				errorf("cannot convert %s to type %s (in %s)", x.typ(), T.typ, Tp)
				return false
			}
			return true
		})
	}

	return false
}

func isUintptr(typ Type) bool {
	t, _ := typ.Underlying().(*Basic)
	return t != nil && t.kind == Uintptr
}

func isUnsafePointer(typ Type) bool {
	t, _ := typ.Underlying().(*Basic)
	return t != nil && t.kind == UnsafePointer
}

func isPointer(typ Type) bool {
	_, ok := typ.Underlying().(*Pointer)
	return ok
}

func isBytesOrRunes(typ Type) bool {
	if s, _ := typ.Underlying().(*Slice); s != nil {
		t, _ := s.elem.Underlying().(*Basic)
		return t != nil && (t.kind == Byte || t.kind == Rune)
	}
	return false
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/cycles.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import "cmd/compile/internal/syntax"

// directCycles searches for direct cycles among package level type declarations.
// See directCycle for details.
func (check *Checker) directCycles() {
	pathIdx := make(map[*TypeName]int)
	for _, obj := range check.objList {
		if tname, ok := obj.(*TypeName); ok {
			check.directCycle(tname, pathIdx)
		}
	}
}

// directCycle checks if the declaration of the type given by tname contains a direct cycle.
// A direct cycle exists if the path from tname's declaration's RHS leads from type name to
// type name and eventually ends up on that path again, via regular or alias declarations;
// in other words if there are no type literals (or basic types) on the path, and the path
// doesn't end in an undeclared object.
// If a cycle is detected, a cycle error is reported and the type at the start of the cycle
// is marked as invalid.
//
// The pathIdx map tracks which type names have been processed. An entry can be
// in 1 of 3 states as used in a typical 3-state (white/grey/black) graph marking
// algorithm for cycle detection:
//
//   - entry not found: tname has not been seen before (white)
//   - value is >= 0  : tname has been seen but is not done (grey); the value is the path index
//   - value is <  0  : tname has been seen and is done (black)
//
// When directCycle returns, the pathIdx entries for all type names on the path
// that starts at tname are marked black, regardless of whether there was a cycle.
// This ensures that a type name is traversed only once.
func (check *Checker) directCycle(tname *TypeName, pathIdx map[*TypeName]int) {
	if debug && check.conf.Trace {
		check.trace(tname.Pos(), "-- check direct cycle for %s", tname)
	}

	var path []*TypeName
	for {
		start, found := pathIdx[tname]
		if start < 0 {
			// tname is marked black - do not traverse it again.
			// (start can only be < 0 if it was found in the first place)
			break
		}

		if found {
			// tname is marked grey - we have a cycle on the path beginning at start.
			// Mark tname as invalid.
			tname.setType(Typ[Invalid])

			// collect type names on cycle
			var cycle []Object
			for _, tname := range path[start:] {
				cycle = append(cycle, tname)
			}

			check.cycleError(cycle, firstInSrc(cycle))
			break
		}

		// tname is marked white - mark it grey and add it to the path.
		pathIdx[tname] = len(path)
		path = append(path, tname)

		// For direct cycle detection, we don't care about whether we have an alias or not.
		// If the associated type is not a name, we're at the end of the path and we're done.
		rhs, ok := check.objMap[tname].tdecl.Type.(*syntax.Name)
		if !ok {
			break
		}

		// Determine the RHS type. If it is not found in the package scope, we either
		// have an error (which will be reported later), or the type exists elsewhere
		// (universe scope, file scope via dot-import) and a cycle is not possible in
		// the first place. If it is not a type name, we cannot have a direct cycle
		// either. In all these cases we can stop.
		tname1, ok := check.pkg.scope.Lookup(rhs.Value).(*TypeName)
		if !ok {
			break
		}

		// Otherwise, continue with the RHS.
		tname = tname1
	}

	// Mark all traversed type names black.
	// (ensure that pathIdx doesn't contain any grey entries upon returning)
	for _, tname := range path {
		pathIdx[tname] = -1
	}

	if debug {
		for _, i := range pathIdx {
			assert(i < 0)
		}
	}
}

// isComplete returns whether a type is complete (i.e. up to having an underlying type).
// Incomplete types will panic if [Type.Underlying] is called on them.
func (check *Checker) isComplete(t Type) bool {
	var (
		obj Object
		rhs Type
	)
	switch t := t.(type) {
	case *Alias:
		obj = t.obj
		rhs = t.fromRHS
	case *Named:
		obj = t.obj
		rhs = t.fromRHS
	default:
		return true
	}

	if i, found := check.objPathIdx[obj]; found {
		cycle := check.objPath[i:]
		check.cycleError(cycle, firstInSrc(cycle))
		return false
	}

	// We must walk through names because we permit certain cycles of names.
	// Consider:
	//
	//   type A B
	//   type B [unsafe.Sizeof(A{})]int
	//
	// starting at B. At the site of A{}, A has no underlying type, and so a
	// cycle must be reported.
	return check.isComplete(rhs)
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/decl.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	. "internal/types/errors"
	"slices"
)

func (check *Checker) declare(scope *Scope, id *syntax.Name, obj Object, pos syntax.Pos) {
	// spec: "The blank identifier, represented by the underscore
	// character _, may be used in a declaration like any other
	// identifier but the declaration does not introduce a new
	// binding."
	if obj.Name() != "_" {
		if alt := scope.Insert(obj); alt != nil {
			err := check.newError(DuplicateDecl)
			err.addf(obj, "%s redeclared in this block", obj.Name())
			err.addAltDecl(alt)
			err.report()
			return
		}
		obj.setScopePos(pos)
	}
	if id != nil {
		check.recordDef(id, obj)
	}
}

// pathString returns a string of the form a->b-> ... ->g for a path [a, b, ... g].
func pathString(path []Object) string {
	var s string
	for i, p := range path {
		if i > 0 {
			s += "->"
		}
		s += p.Name()
	}
	return s
}

// objDecl type-checks the declaration of obj in its respective (file) environment.
func (check *Checker) objDecl(obj Object) {
	if tracePos {
		check.pushPos(obj.Pos())
		defer func() {
			// If we're panicking, keep stack of source positions.
			if p := recover(); p != nil {
				panic(p)
			}
			check.popPos()
		}()
	}

	if check.conf.Trace && obj.Type() == nil {
		if check.indent == 0 {
			fmt.Println() // empty line between top-level objects for readability
		}
		check.trace(obj.Pos(), "-- checking %s (objPath = %s)", obj, pathString(check.objPath))
		check.indent++
		defer func() {
			check.indent--
			check.trace(obj.Pos(), "=> %s", obj)
		}()
	}

	// Checking the declaration of an object means determining its type
	// (and also its value for constants). An object (and thus its type)
	// may be in 1 of 3 states:
	//
	// - not in Checker.objPathIdx and type == nil : type is not yet known (white)
	// -     in Checker.objPathIdx                 : type is pending       (grey)
	// - not in Checker.objPathIdx and type != nil : type is known         (black)
	//
	// During type-checking, an object changes from white to grey to black.
	// Predeclared objects start as black (their type is known without checking).
	//
	// A black object may only depend on (refer to) to other black objects. White
	// and grey objects may depend on white or black objects. A dependency on a
	// grey object indicates a (possibly invalid) cycle.
	//
	// When an object is marked grey, it is pushed onto the object path (a stack)
	// and its index in the path is recorded in the path index map. It is popped
	// and removed from the map when its type is determined (and marked black).

	// If this object is grey, we have a (possibly invalid) cycle. This is signaled
	// by a non-nil type for the object, except for constants and variables whose
	// type may be non-nil (known), or nil if it depends on a not-yet known
	// initialization value.
	//
	// In the former case, set the type to Typ[Invalid] because we have an
	// initialization cycle. The cycle error will be reported later, when
	// determining initialization order.
	//
	// TODO(gri) Report cycle here and simplify initialization order code.
	if _, ok := check.objPathIdx[obj]; ok {
		switch obj := obj.(type) {
		case *Const, *Var:
			if !check.validCycle(obj) || obj.Type() == nil {
				obj.setType(Typ[Invalid])
			}
		case *TypeName:
			if !check.validCycle(obj) {
				obj.setType(Typ[Invalid])
			}
		case *Func:
			if !check.validCycle(obj) {
				// Don't set type to Typ[Invalid]; plenty of code asserts that
				// functions have a *Signature type. Instead, leave the type
				// as an empty signature, which makes it impossible to
				// initialize a variable with the function.
			}
		default:
			panic("unreachable")
		}

		assert(obj.Type() != nil)
		return
	}

	if obj.Type() != nil { // black, meaning it's already type-checked
		return
	}

	// white, meaning it must be type-checked

	check.push(obj)
	defer check.pop()

	d, ok := check.objMap[obj]
	assert(ok)

	// save/restore current environment and set up object environment
	defer func(env environment) {
		check.environment = env
	}(check.environment)
	check.environment = environment{scope: d.file, version: d.version}

	// Const and var declarations must not have initialization
	// cycles. We track them by remembering the current declaration
	// in check.decl. Initialization expressions depending on other
	// consts, vars, or functions, add dependencies to the current
	// check.decl.
	switch obj := obj.(type) {
	case *Const:
		check.decl = d // new package-level const decl
		check.constDecl(obj, d.vtyp, d.init, d.inherited)
	case *Var:
		check.decl = d // new package-level var decl
		check.varDecl(obj, d.lhs, d.vtyp, d.init)
	case *TypeName:
		// invalid recursive types are detected via path
		check.typeDecl(obj, d.tdecl)
		check.collectMethods(obj) // methods can only be added to top-level types
	case *Func:
		// functions may be recursive - no need to track dependencies
		check.funcDecl(obj, d)
	default:
		panic("unreachable")
	}
}

// validCycle reports whether the cycle starting with obj is valid and
// reports an error if it is not.
func (check *Checker) validCycle(obj Object) (valid bool) {
	// The object map contains the package scope objects and the non-interface methods.
	if debug {
		info := check.objMap[obj]
		inObjMap := info != nil && (info.fdecl == nil || info.fdecl.Recv == nil) // exclude methods
		isPkgObj := obj.Parent() == check.pkg.scope
		if isPkgObj != inObjMap {
			check.dump("%v: inconsistent object map for %s (isPkgObj = %v, inObjMap = %v)", obj.Pos(), obj, isPkgObj, inObjMap)
			panic("unreachable")
		}
	}

	// Count cycle objects.
	start, found := check.objPathIdx[obj]
	assert(found)
	cycle := check.objPath[start:]
	tparCycle := false // if set, the cycle is through a type parameter list
	nval := 0          // number of (constant or variable) values in the cycle
	ndef := 0          // number of type definitions in the cycle
loop:
	for _, obj := range cycle {
		switch obj := obj.(type) {
		case *Const, *Var:
			nval++
		case *TypeName:
			// If we reach a generic type that is part of a cycle
			// and we are in a type parameter list, we have a cycle
			// through a type parameter list.
			if check.inTParamList && isGeneric(obj.typ) {
				tparCycle = true
				break loop
			}
			if !obj.IsAlias() {
				ndef++
			}
		case *Func:
			// ignored for now
		default:
			panic("unreachable")
		}
	}

	if check.conf.Trace {
		check.trace(obj.Pos(), "## cycle detected: objPath = %s->%s (len = %d)", pathString(cycle), obj.Name(), len(cycle))
		if tparCycle {
			check.trace(obj.Pos(), "## cycle contains: generic type in a type parameter list")
		} else {
			check.trace(obj.Pos(), "## cycle contains: %d values, %d type definitions", nval, ndef)
		}
		defer func() {
			if valid {
				check.trace(obj.Pos(), "=> cycle is valid")
			} else {
				check.trace(obj.Pos(), "=> error: cycle is invalid")
			}
		}()
	}

	// Cycles through type parameter lists are ok (go.dev/issue/68162).
	if tparCycle {
		return true
	}

	// A cycle involving only constants and variables is invalid but we
	// ignore them here because they are reported via the initialization
	// cycle check.
	if nval == len(cycle) {
		return true
	}

	// A cycle involving only types (and possibly functions) must have at least
	// one type definition to be permitted: If there is no type definition, we
	// have a sequence of alias type names which will expand ad infinitum.
	if nval == 0 && ndef > 0 {
		return true
	}

	check.cycleError(cycle, firstInSrc(cycle))
	return false
}

// cycleError reports a declaration cycle starting with the object at cycle[start].
func (check *Checker) cycleError(cycle []Object, start int) {
	// name returns the (possibly qualified) object name.
	// This is needed because with generic types, cycles
	// may refer to imported types. See go.dev/issue/50788.
	// TODO(gri) This functionality is used elsewhere. Factor it out.
	name := func(obj Object) string {
		// include any type arguments in the reported error message
		if n := asNamed(obj.Type()); n != nil && n.inst != nil {
			return TypeString(n, check.qualifier)
		}
		return packagePrefix(obj.Pkg(), check.qualifier) + obj.Name()
	}

	// If obj is a type alias, mark it as valid (not broken) in order to avoid follow-on errors.
	obj := cycle[start]
	tname, _ := obj.(*TypeName)
	if tname != nil {
		if a, ok := tname.Type().(*Alias); ok {
			a.fromRHS = Typ[Invalid]
		}
	}

	// report a more concise error for self references
	if len(cycle) == 1 {
		if tname != nil {
			check.errorf(obj, InvalidDeclCycle, "invalid recursive type: %s refers to itself", name(obj))
		} else {
			check.errorf(obj, InvalidDeclCycle, "invalid cycle in declaration: %s refers to itself", name(obj))
		}
		return
	}

	err := check.newError(InvalidDeclCycle)
	if tname != nil {
		err.addf(obj, "invalid recursive type %s", name(obj))
	} else {
		err.addf(obj, "invalid cycle in declaration of %s", name(obj))
	}
	// "cycle[i] refers to cycle[j]" for (i,j) = (s,s+1), (s+1,s+2), ..., (n-1,0), (0,1), ..., (s-1,s) for len(cycle) = n, s = start.
	for i := range cycle {
		next := cycle[(start+i+1)%len(cycle)]
		err.addf(obj, "%s refers to %s", name(obj), name(next))
		obj = next
	}
	err.report()
}

// firstInSrc reports the index of the object with the "smallest"
// source position in path. path must not be empty.
func firstInSrc(path []Object) int {
	fst, pos := 0, path[0].Pos()
	for i, t := range path[1:] {
		if cmpPos(t.Pos(), pos) < 0 {
			fst, pos = i+1, t.Pos()
		}
	}
	return fst
}

func (check *Checker) constDecl(obj *Const, typ, init syntax.Expr, inherited bool) {
	assert(obj.typ == nil)

	// use the correct value of iota and errpos
	defer func(iota constant.Value, errpos syntax.Pos) {
		check.iota = iota
		check.errpos = errpos
	}(check.iota, check.errpos)
	check.iota = obj.val
	check.errpos = nopos

	// provide valid constant value under all circumstances
	obj.val = constant.MakeUnknown()

	// determine type, if any
	if typ != nil {
		t := check.typ(typ)
		if !isConstType(t) {
			// don't report an error if the type is an invalid C (defined) type
			// (go.dev/issue/22090)
			if isValid(t.Underlying()) {
				check.errorf(typ, InvalidConstType, "invalid constant type %s", t)
			}
			obj.typ = Typ[Invalid]
			return
		}
		obj.typ = t
	}

	// check initialization
	var x operand
	if init != nil {
		if inherited {
			// The initialization expression is inherited from a previous
			// constant declaration, and (error) positions refer to that
			// expression and not the current constant declaration. Use
			// the constant identifier position for any errors during
			// init expression evaluation since that is all we have
			// (see issues go.dev/issue/42991, go.dev/issue/42992).
			check.errpos = obj.pos
		}
		check.expr(nil, &x, init)
	}
	check.initConst(obj, &x)
}

func (check *Checker) varDecl(obj *Var, lhs []*Var, typ, init syntax.Expr) {
	assert(obj.typ == nil)

	// determine type, if any
	if typ != nil {
		obj.typ = check.varType(typ)
		// We cannot spread the type to all lhs variables if there
		// are more than one since that would mark them as checked
		// (see Checker.objDecl) and the assignment of init exprs,
		// if any, would not be checked.
		//
		// TODO(gri) If we have no init expr, we should distribute
		// a given type otherwise we need to re-evaluate the type
		// expr for each lhs variable, leading to duplicate work.
	}

	// check initialization
	if init == nil {
		if typ == nil {
			// error reported before by arityMatch
			obj.typ = Typ[Invalid]
		}
		return
	}

	if lhs == nil || len(lhs) == 1 {
		assert(lhs == nil || lhs[0] == obj)
		var x operand
		check.expr(newTarget(obj.typ, obj.name), &x, init)
		check.initVar(obj, &x, "variable declaration")
		return
	}

	if debug {
		// obj must be one of lhs
		if !slices.Contains(lhs, obj) {
			panic("inconsistent lhs")
		}
	}

	// We have multiple variables on the lhs and one init expr.
	// Make sure all variables have been given the same type if
	// one was specified, otherwise they assume the type of the
	// init expression values (was go.dev/issue/15755).
	if typ != nil {
		for _, lhs := range lhs {
			lhs.typ = obj.typ
		}
	}

	check.initVars(lhs, []syntax.Expr{init}, nil)
}

// isImportedConstraint reports whether typ is an imported type constraint.
func (check *Checker) isImportedConstraint(typ Type) bool {
	named := asNamed(typ)
	if named == nil || named.obj.pkg == check.pkg || named.obj.pkg == nil {
		return false
	}
	u, _ := named.Underlying().(*Interface)
	return u != nil && !u.IsMethodSet()
}

func (check *Checker) typeDecl(obj *TypeName, tdecl *syntax.TypeDecl) {
	assert(obj.typ == nil)

	// Only report a version error if we have not reported one already.
	versionErr := false

	var rhs Type
	check.later(func() {
		if t := asNamed(obj.typ); t != nil { // type may be invalid
			check.validType(t)
		}
		// If typ is local, an error was already reported where typ is specified/defined.
		_ = !versionErr && check.isImportedConstraint(rhs) && check.verifyVersionf(tdecl.Type, go1_18, "using type constraint %s", rhs)
	}).describef(obj, "validType(%s)", obj.Name())

	// First type parameter, or nil.
	var tparam0 *syntax.Field
	if len(tdecl.TParamList) > 0 {
		tparam0 = tdecl.TParamList[0]
	}

	// alias declaration
	if tdecl.Alias {
		// Report highest version requirement first so that fixing a version issue
		// avoids possibly two -lang changes (first to Go 1.9 and then to Go 1.23).
		if !versionErr && tparam0 != nil && !check.verifyVersionf(tparam0, go1_23, "generic type alias") {
			versionErr = true
		}
		if !versionErr && !check.verifyVersionf(tdecl, go1_9, "type alias") {
			versionErr = true
		}

		alias := check.newAlias(obj, nil)

		// If we could not type the RHS, set it to invalid. This should
		// only ever happen if we panic before setting.
		defer func() {
			if alias.fromRHS == nil {
				alias.fromRHS = Typ[Invalid]
				unalias(alias)
			}
		}()

		// handle type parameters even if not allowed (Alias type is supported)
		if tparam0 != nil {
			check.openScope(tdecl, "type parameters")
			defer check.closeScope()
			check.collectTypeParams(&alias.tparams, tdecl.TParamList)
		}

		rhs = check.declaredType(tdecl.Type, obj)
		assert(rhs != nil)
		alias.fromRHS = rhs

		// spec: In an alias declaration the given type cannot be a type parameter declared in the same declaration."
		// (see also go.dev/issue/75884, go.dev/issue/#75885)
		if tpar, ok := rhs.(*TypeParam); ok && alias.tparams != nil && slices.Index(alias.tparams.list(), tpar) >= 0 {
			check.error(tdecl.Type, MisplacedTypeParam, "cannot use type parameter declared in alias declaration as RHS")
			alias.fromRHS = Typ[Invalid]
		}

		return
	}

	// type definition or generic type declaration
	if !versionErr && tparam0 != nil && !check.verifyVersionf(tparam0, go1_18, "type parameter") {
		versionErr = true
	}

	named := check.newNamed(obj, nil, nil)
	if tdecl.TParamList != nil {
		check.openScope(tdecl, "type parameters")
		defer check.closeScope()
		check.collectTypeParams(&named.tparams, tdecl.TParamList)
	}

	rhs = check.declaredType(tdecl.Type, obj)
	assert(rhs != nil)
	named.fromRHS = rhs

	// spec: "In a type definition the given type cannot be a type parameter."
	// (See also go.dev/issue/45639.)
	if isTypeParam(rhs) {
		check.error(tdecl.Type, MisplacedTypeParam, "cannot use a type parameter as RHS in type declaration")
		named.fromRHS = Typ[Invalid]
	}
}

func (check *Checker) collectTypeParams(dst **TypeParamList, list []*syntax.Field) {
	tparams := make([]*TypeParam, len(list))

	// Declare type parameters up-front.
	// The scope of type parameters starts at the beginning of the type parameter
	// list (so we can have mutually recursive parameterized type bounds).
	if len(list) > 0 {
		scopePos := list[0].Pos()
		for i, f := range list {
			tparams[i] = check.declareTypeParam(f.Name, scopePos)
		}
	}

	// Set the type parameters before collecting the type constraints because
	// the parameterized type may be used by the constraints (go.dev/issue/47887).
	// Example: type T[P T[P]] interface{}
	*dst = bindTParams(tparams)

	// Signal to cycle detection that we are in a type parameter list.
	// We can only be inside one type parameter list at any given time:
	// function closures may appear inside a type parameter list but they
	// cannot be generic, and their bodies are processed in delayed and
	// sequential fashion. Note that with each new declaration, we save
	// the existing environment and restore it when done; thus inTParamList
	// is true exactly only when we are in a specific type parameter list.
	assert(!check.inTParamList)
	check.inTParamList = true
	defer func() {
		check.inTParamList = false
	}()

	// Keep track of bounds for later validation.
	var bound Type
	for i, f := range list {
		// Optimization: Re-use the previous type bound if it hasn't changed.
		// This also preserves the grouped output of type parameter lists
		// when printing type strings.
		if i == 0 || f.Type != list[i-1].Type {
			bound = check.bound(f.Type)
			if isTypeParam(bound) {
				// We may be able to allow this since it is now well-defined what
				// the underlying type and thus type set of a type parameter is.
				// But we may need some additional form of cycle detection within
				// type parameter lists.
				check.error(f.Type, MisplacedTypeParam, "cannot use a type parameter as constraint")
				bound = Typ[Invalid]
			}
		}
		tparams[i].bound = bound
	}
}

func (check *Checker) bound(x syntax.Expr) Type {
	// A type set literal of the form ~T and A|B may only appear as constraint;
	// embed it in an implicit interface so that only interface type-checking
	// needs to take care of such type expressions.
	if op, _ := x.(*syntax.Operation); op != nil && (op.Op == syntax.Tilde || op.Op == syntax.Or) {
		t := check.typ(&syntax.InterfaceType{MethodList: []*syntax.Field{{Type: x}}})
		// mark t as implicit interface if all went well
		if t, _ := t.(*Interface); t != nil {
			t.implicit = true
		}
		return t
	}
	return check.typ(x)
}

func (check *Checker) declareTypeParam(name *syntax.Name, scopePos syntax.Pos) *TypeParam {
	// Use Typ[Invalid] for the type constraint to ensure that a type
	// is present even if the actual constraint has not been assigned
	// yet.
	// TODO(gri) Need to systematically review all uses of type parameter
	//           constraints to make sure we don't rely on them if they
	//           are not properly set yet.
	tname := NewTypeName(name.Pos(), check.pkg, name.Value, nil)
	tpar := check.newTypeParam(tname, Typ[Invalid]) // assigns type to tname as a side-effect
	check.declare(check.scope, name, tname, scopePos)
	return tpar
}

func (check *Checker) collectMethods(obj *TypeName) {
	// get associated methods
	// (Checker.collectObjects only collects methods with non-blank names;
	// Checker.resolveBaseTypeName ensures that obj is not an alias name
	// if it has attached methods.)
	methods := check.methods[obj]
	if methods == nil {
		return
	}
	delete(check.methods, obj)
	assert(!check.objMap[obj].tdecl.Alias) // don't use TypeName.IsAlias (requires fully set up object)

	// use an objset to check for name conflicts
	var mset objset

	// spec: "If the base type is a struct type, the non-blank method
	// and field names must be distinct."
	base := asNamed(obj.typ) // shouldn't fail but be conservative
	if base != nil {
		assert(base.TypeArgs().Len() == 0) // collectMethods should not be called on an instantiated type

		// See go.dev/issue/52529: we must delay the expansion of underlying here, as
		// base may not be fully set-up.
		check.later(func() {
			check.checkFieldUniqueness(base)
		}).describef(obj, "verifying field uniqueness for %v", base)

		// Checker.Files may be called multiple times; additional package files
		// may add methods to already type-checked types. Add pre-existing methods
		// so that we can detect redeclarations.
		for i := 0; i < base.NumMethods(); i++ {
			m := base.Method(i)
			assert(m.name != "_")
			assert(mset.insert(m) == nil)
		}
	}

	// add valid methods
	for _, m := range methods {
		// spec: "For a base type, the non-blank names of methods bound
		// to it must be unique."
		assert(m.name != "_")
		if alt := mset.insert(m); alt != nil {
			if alt.Pos().IsKnown() {
				check.errorf(m.pos, DuplicateMethod, "method %s.%s already declared at %v", obj.Name(), m.name, alt.Pos())
			} else {
				check.errorf(m.pos, DuplicateMethod, "method %s.%s already declared", obj.Name(), m.name)
			}
			continue
		}

		if base != nil {
			base.AddMethod(m)
		}
	}
}

func (check *Checker) checkFieldUniqueness(base *Named) {
	if t, _ := base.Underlying().(*Struct); t != nil {
		var mset objset
		for i := 0; i < base.NumMethods(); i++ {
			m := base.Method(i)
			assert(m.name != "_")
			assert(mset.insert(m) == nil)
		}

		// Check that any non-blank field names of base are distinct from its
		// method names.
		for _, fld := range t.fields {
			if fld.name != "_" {
				if alt := mset.insert(fld); alt != nil {
					// Struct fields should already be unique, so we should only
					// encounter an alternate via collision with a method name.
					_ = alt.(*Func)

					// For historical consistency, we report the primary error on the
					// method, and the alt decl on the field.
					err := check.newError(DuplicateFieldAndMethod)
					err.addf(alt, "field and method with the same name %s", fld.name)
					err.addAltDecl(fld)
					err.report()
				}
			}
		}
	}
}

func (check *Checker) funcDecl(obj *Func, decl *declInfo) {
	assert(obj.typ == nil)

	// func declarations cannot use iota
	assert(check.iota == nil)

	sig := new(Signature)
	obj.typ = sig // guard against cycles

	fdecl := decl.fdecl
	check.funcType(sig, fdecl.Recv, fdecl.TParamList, fdecl.Type)

	if fdecl.Pragma != nil {
		if p, ok := fdecl.Pragma.(interface{ Nointerface() bool }); ok && p.Nointerface() {
			obj.nointerface = true
		}
	}

	// Set the scope's extent to the complete "func (...) { ... }"
	// so that Scope.Innermost works correctly.
	sig.scope.pos = fdecl.Pos()
	sig.scope.end = syntax.EndPos(fdecl)

	if len(fdecl.TParamList) > 0 && fdecl.Body == nil {
		check.softErrorf(fdecl, BadDecl, "generic function is missing function body")
	}

	// function body must be type-checked after global declarations
	// (functions implemented elsewhere have no body)
	if !check.conf.IgnoreFuncBodies && fdecl.Body != nil {
		check.later(func() {
			check.funcBody(decl, obj.name, sig, fdecl.Body, nil)
		}).describef(obj, "func %s", obj.name)
	}
}

func (check *Checker) declStmt(list []syntax.Decl) {
	pkg := check.pkg

	first := -1                // index of first ConstDecl in the current group, or -1
	var last *syntax.ConstDecl // last ConstDecl with init expressions, or nil
	for index, decl := range list {
		if _, ok := decl.(*syntax.ConstDecl); !ok {
			first = -1 // we're not in a constant declaration
		}

		switch s := decl.(type) {
		case *syntax.ConstDecl:
			top := len(check.delayed)

			// iota is the index of the current constDecl within the group
			if first < 0 || s.Group == nil || list[index-1].(*syntax.ConstDecl).Group != s.Group {
				first = index
				last = nil
			}
			iota := constant.MakeInt64(int64(index - first))

			// determine which initialization expressions to use
			inherited := true
			switch {
			case s.Type != nil || s.Values != nil:
				last = s
				inherited = false
			case last == nil:
				last = new(syntax.ConstDecl) // make sure last exists
				inherited = false
			}

			// declare all constants
			lhs := make([]*Const, len(s.NameList))
			values := syntax.UnpackListExpr(last.Values)
			for i, name := range s.NameList {
				obj := NewConst(name.Pos(), pkg, name.Value, nil, iota)
				lhs[i] = obj

				var init syntax.Expr
				if i < len(values) {
					init = values[i]
				}

				check.constDecl(obj, last.Type, init, inherited)
			}

			// Constants must always have init values.
			check.arity(s.Pos(), s.NameList, values, true, inherited)

			// process function literals in init expressions before scope changes
			check.processDelayed(top)

			// spec: "The scope of a constant or variable identifier declared
			// inside a function begins at the end of the ConstSpec or VarSpec
			// (ShortVarDecl for short variable declarations) and ends at the
			// end of the innermost containing block."
			scopePos := syntax.EndPos(s)
			for i, name := range s.NameList {
				check.declare(check.scope, name, lhs[i], scopePos)
			}

		case *syntax.VarDecl:
			top := len(check.delayed)

			lhs0 := make([]*Var, len(s.NameList))
			for i, name := range s.NameList {
				lhs0[i] = newVar(LocalVar, name.Pos(), pkg, name.Value, nil)
			}

			// initialize all variables
			values := syntax.UnpackListExpr(s.Values)
			for i, obj := range lhs0 {
				var lhs []*Var
				var init syntax.Expr
				switch len(values) {
				case len(s.NameList):
					// lhs and rhs match
					init = values[i]
				case 1:
					// rhs is expected to be a multi-valued expression
					lhs = lhs0
					init = values[0]
				default:
					if i < len(values) {
						init = values[i]
					}
				}
				check.varDecl(obj, lhs, s.Type, init)
				if len(values) == 1 {
					// If we have a single lhs variable we are done either way.
					// If we have a single rhs expression, it must be a multi-
					// valued expression, in which case handling the first lhs
					// variable will cause all lhs variables to have a type
					// assigned, and we are done as well.
					if debug {
						for _, obj := range lhs0 {
							assert(obj.typ != nil)
						}
					}
					break
				}
			}

			// If we have no type, we must have values.
			if s.Type == nil || values != nil {
				check.arity(s.Pos(), s.NameList, values, false, false)
			}

			// process function literals in init expressions before scope changes
			check.processDelayed(top)

			// declare all variables
			// (only at this point are the variable scopes (parents) set)
			scopePos := syntax.EndPos(s) // see constant declarations
			for i, name := range s.NameList {
				// see constant declarations
				check.declare(check.scope, name, lhs0[i], scopePos)
			}

		case *syntax.TypeDecl:
			obj := NewTypeName(s.Name.Pos(), pkg, s.Name.Value, nil)
			// spec: "The scope of a type identifier declared inside a function
			// begins at the identifier in the TypeSpec and ends at the end of
			// the innermost containing block."
			scopePos := s.Name.Pos()
			check.declare(check.scope, s.Name, obj, scopePos)
			check.push(obj) // mark as grey
			check.typeDecl(obj, s)
			check.pop()

		default:
			check.errorf(s, InvalidSyntaxTree, "unknown syntax.Decl node %T", s)
		}
	}
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/errors.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements error reporting.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	. "internal/types/errors"
	"runtime"
	"strings"
)

func assert(p bool) {
	if !p {
		msg := "assertion failed"
		// Include information about the assertion location. Due to panic recovery,
		// this location is otherwise buried in the middle of the panicking stack.
		if _, file, line, ok := runtime.Caller(1); ok {
			msg = fmt.Sprintf("%s:%d: %s", file, line, msg)
		}
		panic(msg)
	}
}

// An errorDesc describes part of a type-checking error.
type errorDesc struct {
	pos syntax.Pos
	msg string
}

// An error_ represents a type-checking error.
// A new error_ is created with Checker.newError.
// To report an error_, call error_.report.
type error_ struct {
	check *Checker
	desc  []errorDesc
	code  Code
	soft  bool // TODO(gri) eventually determine this from an error code
}

// newError returns a new error_ with the given error code.
func (check *Checker) newError(code Code) *error_ {
	if code == 0 {
		panic("error code must not be 0")
	}
	return &error_{check: check, code: code}
}

// addf adds formatted error information to err.
// It may be called multiple times to provide additional information.
// The position of the first call to addf determines the position of the reported Error.
// Subsequent calls to addf provide additional information in the form of additional lines
// in the error message (types2) or continuation errors identified by a tab-indented error
// message (go/types).
func (err *error_) addf(at poser, format string, args ...any) {
	err.desc = append(err.desc, errorDesc{atPos(at), err.check.sprintf(format, args...)})
}

// addAltDecl is a specialized form of addf reporting another declaration of obj.
func (err *error_) addAltDecl(obj Object) {
	if pos := obj.Pos(); pos.IsKnown() {
		// We use "other" rather than "previous" here because
		// the first declaration seen may not be textually
		// earlier in the source.
		err.addf(obj, "other declaration of %s", obj.Name())
	}
}

func (err *error_) empty() bool {
	return err.desc == nil
}

func (err *error_) pos() syntax.Pos {
	if err.empty() {
		return nopos
	}
	return err.desc[0].pos
}

// msg returns the formatted error message without the primary error position pos().
func (err *error_) msg() string {
	if err.empty() {
		return "no error"
	}

	var buf strings.Builder
	for i := range err.desc {
		p := &err.desc[i]
		if i > 0 {
			fmt.Fprint(&buf, "\n\t")
			if p.pos.IsKnown() {
				fmt.Fprintf(&buf, "%s: ", p.pos)
			}
		}
		buf.WriteString(p.msg)
	}
	return buf.String()
}

// report reports the error err, setting check.firstError if necessary.
func (err *error_) report() {
	if err.empty() {
		panic("no error")
	}

	// Cheap trick: Don't report errors with messages containing
	// "invalid operand" or "invalid type" as those tend to be
	// follow-on errors which don't add useful information. Only
	// exclude them if these strings are not at the beginning,
	// and only if we have at least one error already reported.
	check := err.check
	if check.firstErr != nil {
		// It is sufficient to look at the first sub-error only.
		msg := err.desc[0].msg
		if strings.Index(msg, "invalid operand") > 0 || strings.Index(msg, "invalid type") > 0 {
			return
		}
	}

	if check.conf.Trace {
		check.trace(err.pos(), "ERROR: %s (code = %d)", err.desc[0].msg, err.code)
	}

	// In go/types, if there is a sub-error with a valid position,
	// call the typechecker error handler for each sub-error.
	// Otherwise, call it once, with a single combined message.
	multiError := false
	if !isTypes2 {
		for i := 1; i < len(err.desc); i++ {
			if err.desc[i].pos.IsKnown() {
				multiError = true
				break
			}
		}
	}

	if multiError {
		for i := range err.desc {
			p := &err.desc[i]
			check.handleError(i, p.pos, err.code, p.msg, err.soft)
		}
	} else {
		check.handleError(0, err.pos(), err.code, err.msg(), err.soft)
	}

	// make sure the error is not reported twice
	err.desc = nil
}

// handleError should only be called by error_.report.
func (check *Checker) handleError(index int, pos syntax.Pos, code Code, msg string, soft bool) {
	assert(code != 0)

	if index == 0 {
		// If we are encountering an error while evaluating an inherited
		// constant initialization expression, pos is the position of
		// the original expression, and not of the currently declared
		// constant identifier. Use the provided errpos instead.
		// TODO(gri) We may also want to augment the error message and
		// refer to the position (pos) in the original expression.
		if check.errpos.Pos().IsKnown() {
			assert(check.iota != nil)
			pos = check.errpos
		}

		// Report invalid syntax trees explicitly.
		if code == InvalidSyntaxTree {
			msg = "invalid syntax tree: " + msg
		}

		// If we have a URL for error codes, add a link to the first line.
		if check.conf.ErrorURL != "" {
			url := fmt.Sprintf(check.conf.ErrorURL, code)
			if i := strings.Index(msg, "\n"); i >= 0 {
				msg = msg[:i] + url + msg[i:]
			} else {
				msg += url
			}
		}
	} else {
		// Indent sub-error.
		// Position information is passed explicitly to Error, below.
		msg = "\t" + msg
	}

	e := Error{
		Pos:  pos,
		Msg:  stripAnnotations(msg),
		Full: msg,
		Soft: soft,
		Code: code,
	}

	if check.firstErr == nil {
		check.firstErr = e
	}

	f := check.conf.Error
	if f == nil {
		panic(bailout{}) // record first error and exit
	}
	f(e)
}

const (
	invalidArg = "invalid argument: "
	invalidOp  = "invalid operation: "
)

// The poser interface is used to extract the position of type-checker errors.
type poser interface {
	Pos() syntax.Pos
}

func (check *Checker) error(at poser, code Code, msg string) {
	err := check.newError(code)
	err.addf(at, "%s", msg)
	err.report()
}

func (check *Checker) errorf(at poser, code Code, format string, args ...any) {
	err := check.newError(code)
	err.addf(at, format, args...)
	err.report()
}

func (check *Checker) softErrorf(at poser, code Code, format string, args ...any) {
	err := check.newError(code)
	err.addf(at, format, args...)
	err.soft = true
	err.report()
}

func (check *Checker) versionErrorf(at poser, v goVersion, format string, args ...any) {
	msg := check.sprintf(format, args...)
	err := check.newError(UnsupportedFeature)
	err.addf(at, "%s requires %s or later", msg, v)
	err.report()
}

// atPos reports the left (= start) position of at.
func atPos(at poser) syntax.Pos {
	switch x := at.(type) {
	case *operand:
		if x.expr != nil {
			return syntax.StartPos(x.expr)
		}
	case syntax.Node:
		return syntax.StartPos(x)
	}
	return at.Pos()
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/errsupport.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements support functions for error messages.

package types2

// lookupError returns a case-specific error when a lookup of selector sel in the
// given type fails but an object with alternative spelling (case folding) is found.
// If structLit is set, the error message is specifically for struct literal fields.
func (check *Checker) lookupError(typ Type, sel string, obj Object, structLit bool) string {
	// Provide more detail if there is an unexported object, or one with different capitalization.
	// If selector and object are in the same package (==), export doesn't matter, otherwise (!=) it does.
	// Messages depend on whether it's a general lookup or a field lookup in a struct literal.
	//
	// case           sel     pkg   have   message (examples for general lookup)
	// ---------------------------------------------------------------------------------------------------------
	// ok             x.Foo   ==    Foo
	// misspelled     x.Foo   ==    FoO    type X has no field or method Foo, but does have field FoO
	// misspelled     x.Foo   ==    foo    type X has no field or method Foo, but does have field foo
	// misspelled     x.Foo   ==    foO    type X has no field or method Foo, but does have field foO
	//
	// misspelled     x.foo   ==    Foo    type X has no field or method foo, but does have field Foo
	// misspelled     x.foo   ==    FoO    type X has no field or method foo, but does have field FoO
	// ok             x.foo   ==    foo
	// misspelled     x.foo   ==    foO    type X has no field or method foo, but does have field foO
	//
	// ok             x.Foo   !=    Foo
	// misspelled     x.Foo   !=    FoO    type X has no field or method Foo, but does have field FoO
	// unexported     x.Foo   !=    foo    type X has no field or method Foo, but does have unexported field foo
	// missing        x.Foo   !=    foO    type X has no field or method Foo
	//
	// misspelled     x.foo   !=    Foo    type X has no field or method foo, but does have field Foo
	// missing        x.foo   !=    FoO    type X has no field or method foo
	// inaccessible   x.foo   !=    foo    cannot refer to unexported field foo
	// missing        x.foo   !=    foO    type X has no field or method foo

	const (
		ok           = iota
		missing      // no object found
		misspelled   // found object with different spelling
		unexported   // found object with name differing only in first letter
		inaccessible // found object with matching name but inaccessible from the current package
	)

	// determine case
	e := missing
	var alt string // alternative spelling of selector; if any
	if obj != nil {
		alt = obj.Name()
		if obj.Pkg() == check.pkg {
			assert(alt != sel) // otherwise there is no lookup error
			e = misspelled
		} else if isExported(sel) {
			if isExported(alt) {
				e = misspelled
			} else if tail(sel) == tail(alt) {
				e = unexported
			}
		} else if isExported(alt) {
			if tail(sel) == tail(alt) {
				e = misspelled
			}
		} else if sel == alt {
			e = inaccessible
		}
	}

	if structLit {
		switch e {
		case missing:
			return check.sprintf("unknown field %s in struct literal of type %s", sel, typ)
		case misspelled:
			return check.sprintf("unknown field %s in struct literal of type %s, but does have %s", sel, typ, alt)
		case unexported:
			return check.sprintf("unknown field %s in struct literal of type %s, but does have unexported %s", sel, typ, alt)
		case inaccessible:
			return check.sprintf("cannot refer to unexported field %s in struct literal of type %s", alt, typ)
		}
	} else {
		what := "object"
		switch obj.(type) {
		case *Var:
			what = "field"
		case *Func:
			what = "method"
		}
		switch e {
		case missing:
			return check.sprintf("type %s has no field or method %s", typ, sel)
		case misspelled:
			return check.sprintf("type %s has no field or method %s, but does have %s %s", typ, sel, what, alt)
		case unexported:
			return check.sprintf("type %s has no field or method %s, but does have unexported %s %s", typ, sel, what, alt)
		case inaccessible:
			return check.sprintf("cannot refer to unexported %s %s", what, alt)
		}
	}

	panic("unreachable")
}

// tail returns the string s without its first (UTF-8) character.
// If len(s) == 0, the result is s.
func tail(s string) string {
	for i, _ := range s {
		if i > 0 {
			return s[i:]
		}
	}
	return s
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/expr.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of expressions.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	"go/token"
	. "internal/types/errors"
)

/*
Basic algorithm:

Expressions are checked recursively, top down. Expression checker functions
are generally of the form:

  func f(x *operand, e *syntax.Expr, ...)

where e is the expression to be checked, and x is the result of the check.
The check performed by f may fail in which case x.mode_ == invalid, and
related error messages will have been issued by f.

If a hint argument is present, it is the composite literal element type
of an outer composite literal; it is used to type-check composite literal
elements that have no explicit type specification in the source
(e.g.: []T{{...}, {...}}, the hint is the type T in this case).

All expressions are checked via rawExpr, which dispatches according
to expression kind. Upon returning, rawExpr is recording the types and
constant values for all expressions that have an untyped type (those types
may change on the way up in the expression tree). Usually these are constants,
but the results of comparisons or non-constant shifts of untyped constants
may also be untyped, but not constant.

Untyped expressions may eventually become fully typed (i.e., not untyped),
typically when the value is assigned to a variable, or is used otherwise.
The updateExprType method is used to record this final type and update
the recorded types: the type-checked expression tree is again traversed down,
and the new type is propagated as needed. Untyped constant expression values
that become fully typed must now be representable by the full type (constant
sub-expression trees are left alone except for their roots). This mechanism
ensures that a client sees the actual (run-time) type an untyped value would
have. It also permits type-checking of lhs shift operands "as if the shift
were not present": when updateExprType visits an untyped lhs shift operand
and assigns it it's final type, that type must be an integer type, and a
constant lhs must be representable as an integer.

When an expression gets its final type, either on the way out from rawExpr,
on the way down in updateExprType, or at the end of the type checker run,
the type (and constant value, if any) is recorded via Info.Types, if present.
*/

type opPredicates map[syntax.Operator]func(Type) bool

var unaryOpPredicates opPredicates

func init() {
	// Setting unaryOpPredicates in init avoids declaration cycles.
	unaryOpPredicates = opPredicates{
		syntax.Add: allNumeric,
		syntax.Sub: allNumeric,
		syntax.Xor: allInteger,
		syntax.Not: allBoolean,
	}
}

func (check *Checker) op(m opPredicates, x *operand, op syntax.Operator) bool {
	if pred := m[op]; pred != nil {
		if !pred(x.typ()) {
			check.errorf(x, UndefinedOp, invalidOp+"operator %s not defined on %s", op, x)
			return false
		}
	} else {
		check.errorf(x, InvalidSyntaxTree, "unknown operator %s", op)
		return false
	}
	return true
}

// opPos returns the position of the operator if x is an operation;
// otherwise it returns the start position of x.
func opPos(x syntax.Expr) syntax.Pos {
	switch op := x.(type) {
	case nil:
		return nopos // don't crash
	case *syntax.Operation:
		return op.Pos()
	default:
		return syntax.StartPos(x)
	}
}

// opName returns the name of the operation if x is an operation
// that might overflow; otherwise it returns the empty string.
func opName(x syntax.Expr) string {
	if e, _ := x.(*syntax.Operation); e != nil {
		op := int(e.Op)
		if e.Y == nil {
			if op < len(op2str1) {
				return op2str1[op]
			}
		} else {
			if op < len(op2str2) {
				return op2str2[op]
			}
		}
	}
	return ""
}

var op2str1 = [...]string{
	syntax.Xor: "bitwise complement",
}

// This is only used for operations that may cause overflow.
var op2str2 = [...]string{
	syntax.Add: "addition",
	syntax.Sub: "subtraction",
	syntax.Xor: "bitwise XOR",
	syntax.Mul: "multiplication",
	syntax.Shl: "shift",
}

func (check *Checker) unary(x *operand, e *syntax.Operation) {
	check.expr(nil, x, e.X)
	if !x.isValid() {
		return
	}

	op := e.Op
	switch op {
	case syntax.And:
		// spec: "As an exception to the addressability
		// requirement x may also be a composite literal."
		if _, ok := syntax.Unparen(e.X).(*syntax.CompositeLit); !ok && x.mode() != variable {
			check.errorf(x, UnaddressableOperand, invalidOp+"cannot take address of %s", x)
			x.invalidate()
			return
		}
		x.mode_ = value
		x.typ_ = &Pointer{base: x.typ()}
		return

	case syntax.Recv:
		// We cannot receive a value with an incomplete type; make sure it's complete.
		if elem := check.chanElem(x, x, true); elem != nil && check.isComplete(elem) {
			x.mode_ = commaok
			x.typ_ = elem
			check.hasCallOrRecv = true
			return
		}
		x.invalidate()
		return

	case syntax.Tilde:
		// Provide a better error position and message than what check.op below would do.
		if !allInteger(x.typ()) {
			check.error(e, UndefinedOp, "cannot use ~ outside of interface or type constraint")
			x.invalidate()
			return
		}
		check.error(e, UndefinedOp, "cannot use ~ outside of interface or type constraint (use ^ for bitwise complement)")
		op = syntax.Xor
	}

	if !check.op(unaryOpPredicates, x, op) {
		x.invalidate()
		return
	}

	if x.mode() == constant_ {
		if x.val.Kind() == constant.Unknown {
			// nothing to do (and don't cause an error below in the overflow check)
			return
		}
		var prec uint
		if isUnsigned(x.typ()) {
			prec = uint(check.conf.sizeof(x.typ()) * 8)
		}
		x.val = constant.UnaryOp(op2tok[op], x.val, prec)
		x.expr = e
		check.overflow(x, opPos(x.expr))
		return
	}

	x.mode_ = value
	// x.typ remains unchanged
}

// chanElem returns the channel element type of x for a receive from x (recv == true)
// or send to x (recv == false) operation. If the operation is not valid, chanElem
// reports an error and returns nil.
func (check *Checker) chanElem(pos poser, x *operand, recv bool) Type {
	u, err := commonUnder(x.typ(), func(t, u Type) *typeError {
		if u == nil {
			return typeErrorf("no specific channel type")
		}
		ch, _ := u.(*Chan)
		if ch == nil {
			return typeErrorf("non-channel %s", t)
		}
		if recv && ch.dir == SendOnly {
			return typeErrorf("send-only channel %s", t)
		}
		if !recv && ch.dir == RecvOnly {
			return typeErrorf("receive-only channel %s", t)
		}
		return nil
	})

	if u != nil {
		return u.(*Chan).elem
	}

	cause := err.format(check)
	if recv {
		if isTypeParam(x.typ()) {
			check.errorf(pos, InvalidReceive, invalidOp+"cannot receive from %s: %s", x, cause)
		} else {
			// In this case, only the non-channel and send-only channel error are possible.
			check.errorf(pos, InvalidReceive, invalidOp+"cannot receive from %s %s", cause, x)
		}
	} else {
		if isTypeParam(x.typ()) {
			check.errorf(pos, InvalidSend, invalidOp+"cannot send to %s: %s", x, cause)
		} else {
			// In this case, only the non-channel and receive-only channel error are possible.
			check.errorf(pos, InvalidSend, invalidOp+"cannot send to %s %s", cause, x)
		}
	}
	return nil
}

func isShift(op syntax.Operator) bool {
	return op == syntax.Shl || op == syntax.Shr
}

func isComparison(op syntax.Operator) bool {
	// Note: tokens are not ordered well to make this much easier
	switch op {
	case syntax.Eql, syntax.Neq, syntax.Lss, syntax.Leq, syntax.Gtr, syntax.Geq:
		return true
	}
	return false
}

// updateExprType updates the type of x to typ and invokes itself
// recursively for the operands of x, depending on expression kind.
// If typ is still an untyped and not the final type, updateExprType
// only updates the recorded untyped type for x and possibly its
// operands. Otherwise (i.e., typ is not an untyped type anymore,
// or it is the final type for x), the type and value are recorded.
// Also, if x is a constant, it must be representable as a value of typ,
// and if x is the (formerly untyped) lhs operand of a non-constant
// shift, it must be an integer value.
func (check *Checker) updateExprType(x syntax.Expr, typ Type, final bool) {
	old, found := check.untyped[x]
	if !found {
		return // nothing to do
	}

	// update operands of x if necessary
	switch x := x.(type) {
	case *syntax.BadExpr,
		*syntax.FuncLit,
		*syntax.CompositeLit,
		*syntax.IndexExpr,
		*syntax.SliceExpr,
		*syntax.AssertExpr,
		*syntax.ListExpr,
		//*syntax.StarExpr,
		*syntax.KeyValueExpr,
		*syntax.ArrayType,
		*syntax.StructType,
		*syntax.FuncType,
		*syntax.InterfaceType,
		*syntax.MapType,
		*syntax.ChanType:
		// These expression are never untyped - nothing to do.
		// The respective sub-expressions got their final types
		// upon assignment or use.
		if debug {
			check.dump("%v: found old type(%s): %s (new: %s)", atPos(x), x, old.typ, typ)
			panic("unreachable")
		}
		return

	case *syntax.CallExpr:
		// Resulting in an untyped constant (e.g., built-in complex).
		// The respective calls take care of calling updateExprType
		// for the arguments if necessary.

	case *syntax.Name, *syntax.BasicLit, *syntax.SelectorExpr:
		// An identifier denoting a constant, a constant literal,
		// or a qualified identifier (imported untyped constant).
		// No operands to take care of.

	case *syntax.ParenExpr:
		check.updateExprType(x.X, typ, final)

	// case *syntax.UnaryExpr:
	// 	// If x is a constant, the operands were constants.
	// 	// The operands don't need to be updated since they
	// 	// never get "materialized" into a typed value. If
	// 	// left in the untyped map, they will be processed
	// 	// at the end of the type check.
	// 	if old.val != nil {
	// 		break
	// 	}
	// 	check.updateExprType(x.X, typ, final)

	case *syntax.Operation:
		if x.Y == nil {
			// unary expression
			if x.Op == syntax.Mul {
				// see commented out code for StarExpr above
				// TODO(gri) needs cleanup
				if debug {
					panic("unimplemented")
				}
				return
			}
			// If x is a constant, the operands were constants.
			// The operands don't need to be updated since they
			// never get "materialized" into a typed value. If
			// left in the untyped map, they will be processed
			// at the end of the type check.
			if old.val != nil {
				break
			}
			check.updateExprType(x.X, typ, final)
			break
		}

		// binary expression
		if old.val != nil {
			break // see comment for unary expressions
		}
		if isComparison(x.Op) {
			// The result type is independent of operand types
			// and the operand types must have final types.
		} else if isShift(x.Op) {
			// The result type depends only on lhs operand.
			// The rhs type was updated when checking the shift.
			check.updateExprType(x.X, typ, final)
		} else {
			// The operand types match the result type.
			check.updateExprType(x.X, typ, final)
			check.updateExprType(x.Y, typ, final)
		}

	default:
		panic("unreachable")
	}

	// If the new type is not final and still untyped, just
	// update the recorded type.
	if !final && isUntyped(typ) {
		old.typ = typ.Underlying().(*Basic)
		check.untyped[x] = old
		return
	}

	// Otherwise we have the final (typed or untyped type).
	// Remove it from the map of yet untyped expressions.
	delete(check.untyped, x)

	if old.isLhs {
		// If x is the lhs of a shift, its final type must be integer.
		// We already know from the shift check that it is representable
		// as an integer if it is a constant.
		if !allInteger(typ) {
			check.errorf(x, InvalidShiftOperand, invalidOp+"shifted operand %s (type %s) must be integer", x, typ)
			return
		}
		// Even if we have an integer, if the value is a constant we
		// still must check that it is representable as the specific
		// int type requested (was go.dev/issue/22969). Fall through here.
	}
	if old.val != nil {
		// If x is a constant, it must be representable as a value of typ.
		c := operand{old.mode, x, old.typ, old.val, 0}
		check.convertUntyped(&c, typ)
		if !c.isValid() {
			return
		}
	}

	// Everything's fine, record final type and value for x.
	check.recordTypeAndValue(x, old.mode, typ, old.val)
}

// updateExprVal updates the value of x to val.
func (check *Checker) updateExprVal(x syntax.Expr, val constant.Value) {
	if info, ok := check.untyped[x]; ok {
		info.val = val
		check.untyped[x] = info
	}
}

// implicitTypeAndValue returns the implicit type of x when used in a context
// where the target type is expected. If no such implicit conversion is
// possible, it returns a nil Type and non-zero error code.
//
// If x is a constant operand, the returned constant.Value will be the
// representation of x in this context.
func (check *Checker) implicitTypeAndValue(x *operand, target Type) (Type, constant.Value, Code) {
	if !x.isValid() || isTyped(x.typ()) || !isValid(target) {
		return x.typ(), nil, 0
	}
	// x is untyped

	if isUntyped(target) {
		// both x and target are untyped
		if m := maxType(x.typ(), target); m != nil {
			return m, nil, 0
		}
		return nil, nil, InvalidUntypedConversion
	}

	if x.isNil() {
		assert(isUntyped(x.typ()))
		if hasNil(target) {
			return target, nil, 0
		}
		return nil, nil, InvalidUntypedConversion
	}

	switch u := target.Underlying().(type) {
	case *Basic:
		if x.mode() == constant_ {
			v, code := check.representation(x, u)
			if code != 0 {
				return nil, nil, code
			}
			return target, v, code
		}
		// Non-constant untyped values may appear as the
		// result of comparisons (untyped bool), intermediate
		// (delayed-checked) rhs operands of shifts, and as
		// the value nil.
		switch x.typ().(*Basic).kind {
		case UntypedBool:
			if !isBoolean(target) {
				return nil, nil, InvalidUntypedConversion
			}
		case UntypedInt, UntypedRune, UntypedFloat, UntypedComplex:
			if !isNumeric(target) {
				return nil, nil, InvalidUntypedConversion
			}
		case UntypedString:
			// Non-constant untyped string values are not permitted by the spec and
			// should not occur during normal typechecking passes, but this path is
			// reachable via the AssignableTo API.
			if !isString(target) {
				return nil, nil, InvalidUntypedConversion
			}
		default:
			return nil, nil, InvalidUntypedConversion
		}
	case *Interface:
		if isTypeParam(target) {
			if !underIs(target, func(u Type) bool {
				if u == nil {
					return false
				}
				t, _, _ := check.implicitTypeAndValue(x, u)
				return t != nil
			}) {
				return nil, nil, InvalidUntypedConversion
			}
			break
		}
		// Update operand types to the default type rather than the target
		// (interface) type: values must have concrete dynamic types.
		// Untyped nil was handled upfront.
		if !u.Empty() {
			return nil, nil, InvalidUntypedConversion // cannot assign untyped values to non-empty interfaces
		}
		return Default(x.typ()), nil, 0 // default type for nil is nil
	default:
		return nil, nil, InvalidUntypedConversion
	}
	return target, nil, 0
}

// If switchCase is true, the operator op is ignored.
func (check *Checker) comparison(x, y *operand, op syntax.Operator, switchCase bool) {
	// Avoid spurious errors if any of the operands has an invalid type (go.dev/issue/54405).
	if !isValid(x.typ()) || !isValid(y.typ()) {
		x.invalidate()
		return
	}

	if switchCase {
		op = syntax.Eql
	}

	errOp := x  // operand for which error is reported, if any
	cause := "" // specific error cause, if any

	// spec: "In any comparison, the first operand must be assignable
	// to the type of the second operand, or vice versa."
	code := MismatchedTypes
	ok, _ := x.assignableTo(check, y.typ(), nil)
	if !ok {
		ok, _ = y.assignableTo(check, x.typ(), nil)
	}
	if !ok {
		// Report the error on the 2nd operand since we only
		// know after seeing the 2nd operand whether we have
		// a type mismatch.
		errOp = y
		cause = check.sprintf("mismatched types %s and %s", x.typ(), y.typ())
		goto Error
	}

	// check if comparison is defined for operands
	code = UndefinedOp
	switch op {
	case syntax.Eql, syntax.Neq:
		// spec: "The equality operators == and != apply to operands that are comparable."
		switch {
		case x.isNil() || y.isNil():
			// Comparison against nil requires that the other operand type has nil.
			typ := x.typ()
			if x.isNil() {
				typ = y.typ()
			}
			if !hasNil(typ) {
				// This case should only be possible for "nil == nil".
				// Report the error on the 2nd operand since we only
				// know after seeing the 2nd operand whether we have
				// an invalid comparison.
				errOp = y
				goto Error
			}

		case !Comparable(x.typ()):
			errOp = x
			cause = check.incomparableCause(x.typ())
			goto Error

		case !Comparable(y.typ()):
			errOp = y
			cause = check.incomparableCause(y.typ())
			goto Error
		}

	case syntax.Lss, syntax.Leq, syntax.Gtr, syntax.Geq:
		// spec: The ordering operators <, <=, >, and >= apply to operands that are ordered."
		switch {
		case !allOrdered(x.typ()):
			errOp = x
			goto Error
		case !allOrdered(y.typ()):
			errOp = y
			goto Error
		}

	default:
		panic("unreachable")
	}

	// comparison is ok
	if x.mode() == constant_ && y.mode() == constant_ {
		x.val = constant.MakeBool(constant.Compare(x.val, op2tok[op], y.val))
		// The operands are never materialized; no need to update
		// their types.
	} else {
		x.mode_ = value
		// The operands have now their final types, which at run-
		// time will be materialized. Update the expression trees.
		// If the current types are untyped, the materialized type
		// is the respective default type.
		check.updateExprType(x.expr, Default(x.typ()), true)
		check.updateExprType(y.expr, Default(y.typ()), true)
	}

	// spec: "Comparison operators compare two operands and yield
	//        an untyped boolean value."
	x.typ_ = Typ[UntypedBool]
	return

Error:
	// We have an offending operand errOp and possibly an error cause.
	if cause == "" {
		if isTypeParam(x.typ()) || isTypeParam(y.typ()) {
			// TODO(gri) should report the specific type causing the problem, if any
			if !isTypeParam(x.typ()) {
				errOp = y
			}
			cause = check.sprintf("type parameter %s cannot use operator %s", errOp.typ(), op)
		} else {
			// catch-all: neither x nor y is a type parameter
			what := compositeKind(errOp.typ())
			if what == "" {
				what = check.sprintf("%s", errOp.typ())
			}
			cause = check.sprintf("operator %s not defined on %s", op, what)
		}
	}
	if switchCase {
		check.errorf(x, code, "invalid case %s in switch on %s (%s)", x.expr, y.expr, cause) // error position always at 1st operand
	} else {
		check.errorf(errOp, code, invalidOp+"%s %s %s (%s)", x.expr, op, y.expr, cause)
	}
	x.invalidate()
}

// incomparableCause returns a more specific cause why typ is not comparable.
// If there is no more specific cause, the result is "".
func (check *Checker) incomparableCause(typ Type) string {
	switch typ.Underlying().(type) {
	case *Slice, *Signature, *Map:
		return compositeKind(typ) + " can only be compared to nil"
	}
	// see if we can extract a more specific error
	return comparableType(typ, true, nil).format(check)
}

// If e != nil, it must be the shift expression; it may be nil for non-constant shifts.
func (check *Checker) shift(x, y *operand, e syntax.Expr, op syntax.Operator) {
	// TODO(gri) This function seems overly complex. Revisit.

	var xval constant.Value
	if x.mode() == constant_ {
		xval = constant.ToInt(x.val)
	}

	if allInteger(x.typ()) || isUntyped(x.typ()) && xval != nil && xval.Kind() == constant.Int {
		// The lhs is of integer type or an untyped constant representable
		// as an integer. Nothing to do.
	} else {
		// shift has no chance
		check.errorf(x, InvalidShiftOperand, invalidOp+"shifted operand %s must be integer", x)
		x.invalidate()
		return
	}

	// spec: "The right operand in a shift expression must have integer type
	// or be an untyped constant representable by a value of type uint."

	// Check that constants are representable by uint, but do not convert them
	// (see also go.dev/issue/47243).
	var yval constant.Value
	if y.mode() == constant_ {
		// Provide a good error message for negative shift counts.
		yval = constant.ToInt(y.val) // consider -1, 1.0, but not -1.1
		if yval.Kind() == constant.Int && constant.Sign(yval) < 0 {
			check.errorf(y, InvalidShiftCount, invalidOp+"negative shift count %s", y)
			x.invalidate()
			return
		}

		if isUntyped(y.typ()) {
			// Caution: Check for representability here, rather than in the switch
			// below, because isInteger includes untyped integers (was bug go.dev/issue/43697).
			check.representable(y, Typ[Uint])
			if !y.isValid() {
				x.invalidate()
				return
			}
		}
	} else {
		// Check that RHS is otherwise at least of integer type.
		switch {
		case allInteger(y.typ()):
			if !allUnsigned(y.typ()) && !check.verifyVersionf(y, go1_13, invalidOp+"signed shift count %s", y) {
				x.invalidate()
				return
			}
		case isUntyped(y.typ()):
			// This is incorrect, but preserves pre-existing behavior.
			// See also go.dev/issue/47410.
			check.convertUntyped(y, Typ[Uint])
			if !y.isValid() {
				x.invalidate()
				return
			}
		default:
			check.errorf(y, InvalidShiftCount, invalidOp+"shift count %s must be integer", y)
			x.invalidate()
			return
		}
	}

	if x.mode() == constant_ {
		if y.mode() == constant_ {
			// if either x or y has an unknown value, the result is unknown
			if x.val.Kind() == constant.Unknown || y.val.Kind() == constant.Unknown {
				x.val = constant.MakeUnknown()
				// ensure the correct type - see comment below
				if !isInteger(x.typ()) {
					x.typ_ = Typ[UntypedInt]
				}
				return
			}
			// rhs must be within reasonable bounds in constant shifts
			const shiftBound = 1023 - 1 + 52 // so we can express smallestFloat64 (see go.dev/issue/44057)
			s, ok := constant.Uint64Val(yval)
			if !ok || s > shiftBound {
				check.errorf(y, InvalidShiftCount, invalidOp+"invalid shift count %s", y)
				x.invalidate()
				return
			}
			// The lhs is representable as an integer but may not be an integer
			// (e.g., 2.0, an untyped float) - this can only happen for untyped
			// non-integer numeric constants. Correct the type so that the shift
			// result is of integer type.
			if !isInteger(x.typ()) {
				x.typ_ = Typ[UntypedInt]
			}
			// x is a constant so xval != nil and it must be of Int kind.
			x.val = constant.Shift(xval, op2tok[op], uint(s))
			x.expr = e
			check.overflow(x, opPos(x.expr))
			return
		}

		// non-constant shift with constant lhs
		if isUntyped(x.typ()) {
			// spec: "If the left operand of a non-constant shift
			// expression is an untyped constant, the type of the
			// constant is what it would be if the shift expression
			// were replaced by its left operand alone.".
			//
			// Delay operand checking until we know the final type
			// by marking the lhs expression as lhs shift operand.
			//
			// Usually (in correct programs), the lhs expression
			// is in the untyped map. However, it is possible to
			// create incorrect programs where the same expression
			// is evaluated twice (via a declaration cycle) such
			// that the lhs expression type is determined in the
			// first round and thus deleted from the map, and then
			// not found in the second round (double insertion of
			// the same expr node still just leads to one entry for
			// that node, and it can only be deleted once).
			// Be cautious and check for presence of entry.
			// Example: var e, f = int(1<<""[f]) // go.dev/issue/11347
			if info, found := check.untyped[x.expr]; found {
				info.isLhs = true
				check.untyped[x.expr] = info
			}
			// keep x's type
			x.mode_ = value
			return
		}
	}

	// non-constant shift - lhs must be an integer
	if !allInteger(x.typ()) {
		check.errorf(x, InvalidShiftOperand, invalidOp+"shifted operand %s must be integer", x)
		x.invalidate()
		return
	}

	x.mode_ = value
}

var binaryOpPredicates opPredicates

func init() {
	// Setting binaryOpPredicates in init avoids declaration cycles.
	binaryOpPredicates = opPredicates{
		syntax.Add: allNumericOrString,
		syntax.Sub: allNumeric,
		syntax.Mul: allNumeric,
		syntax.Div: allNumeric,
		syntax.Rem: allInteger,

		syntax.And:    allInteger,
		syntax.Or:     allInteger,
		syntax.Xor:    allInteger,
		syntax.AndNot: allInteger,

		syntax.AndAnd: allBoolean,
		syntax.OrOr:   allBoolean,
	}
}

// If e != nil, it must be the binary expression; it may be nil for non-constant expressions
// (when invoked for an assignment operation where the binary expression is implicit).
func (check *Checker) binary(x *operand, e syntax.Expr, lhs, rhs syntax.Expr, op syntax.Operator) {
	var y operand

	check.expr(nil, x, lhs)
	check.expr(nil, &y, rhs)

	if !x.isValid() {
		return
	}
	if !y.isValid() {
		x.invalidate()
		x.expr = y.expr
		return
	}

	if isShift(op) {
		check.shift(x, &y, e, op)
		return
	}

	check.matchTypes(x, &y)
	if !x.isValid() {
		return
	}

	if isComparison(op) {
		check.comparison(x, &y, op, false)
		return
	}

	if !Identical(x.typ(), y.typ()) {
		// only report an error if we have valid types
		// (otherwise we had an error reported elsewhere already)
		if isValid(x.typ()) && isValid(y.typ()) {
			if e != nil {
				check.errorf(x, MismatchedTypes, invalidOp+"%s (mismatched types %s and %s)", e, x.typ(), y.typ())
			} else {
				check.errorf(x, MismatchedTypes, invalidOp+"%s %s= %s (mismatched types %s and %s)", lhs, op, rhs, x.typ(), y.typ())
			}
		}
		x.invalidate()
		return
	}

	if !check.op(binaryOpPredicates, x, op) {
		x.invalidate()
		return
	}

	if op == syntax.Div || op == syntax.Rem {
		// check for zero divisor
		if (x.mode() == constant_ || allInteger(x.typ())) && y.mode() == constant_ && constant.Sign(y.val) == 0 {
			check.error(&y, DivByZero, invalidOp+"division by zero")
			x.invalidate()
			return
		}

		// check for divisor underflow in complex division (see go.dev/issue/20227)
		if x.mode() == constant_ && y.mode() == constant_ && isComplex(x.typ()) {
			re, im := constant.Real(y.val), constant.Imag(y.val)
			re2, im2 := constant.BinaryOp(re, token.MUL, re), constant.BinaryOp(im, token.MUL, im)
			if constant.Sign(re2) == 0 && constant.Sign(im2) == 0 {
				check.error(&y, DivByZero, invalidOp+"division by zero")
				x.invalidate()
				return
			}
		}
	}

	if x.mode() == constant_ && y.mode() == constant_ {
		// if either x or y has an unknown value, the result is unknown
		if x.val.Kind() == constant.Unknown || y.val.Kind() == constant.Unknown {
			x.val = constant.MakeUnknown()
			// x.typ is unchanged
			return
		}
		// force integer division for integer operands
		tok := op2tok[op]
		if op == syntax.Div && isInteger(x.typ()) {
			tok = token.QUO_ASSIGN
		}
		x.val = constant.BinaryOp(x.val, tok, y.val)
		x.expr = e
		check.overflow(x, opPos(x.expr))
		return
	}

	x.mode_ = value
	// x.typ is unchanged
}

// matchTypes attempts to convert any untyped types x and y such that they match.
// If an error occurs, x.mode is set to invalid.
func (check *Checker) matchTypes(x, y *operand) {
	// mayConvert reports whether the operands x and y may
	// possibly have matching types after converting one
	// untyped operand to the type of the other.
	// If mayConvert returns true, we try to convert the
	// operands to each other's types, and if that fails
	// we report a conversion failure.
	// If mayConvert returns false, we continue without an
	// attempt at conversion, and if the operand types are
	// not compatible, we report a type mismatch error.
	mayConvert := func(x, y *operand) bool {
		// If both operands are typed, there's no need for an implicit conversion.
		if isTyped(x.typ()) && isTyped(y.typ()) {
			return false
		}
		// A numeric type can only convert to another numeric type.
		if allNumeric(x.typ()) != allNumeric(y.typ()) {
			return false
		}
		// An untyped operand may convert to its default type when paired with an empty interface
		// TODO(gri) This should only matter for comparisons (the only binary operation that is
		//           valid with interfaces), but in that case the assignability check should take
		//           care of the conversion. Verify and possibly eliminate this extra test.
		if isNonTypeParamInterface(x.typ()) || isNonTypeParamInterface(y.typ()) {
			return true
		}
		// A boolean type can only convert to another boolean type.
		if allBoolean(x.typ()) != allBoolean(y.typ()) {
			return false
		}
		// A string type can only convert to another string type.
		if allString(x.typ()) != allString(y.typ()) {
			return false
		}
		// Untyped nil can only convert to a type that has a nil.
		if x.isNil() {
			return hasNil(y.typ())
		}
		if y.isNil() {
			return hasNil(x.typ())
		}
		// An untyped operand cannot convert to a pointer.
		// TODO(gri) generalize to type parameters
		if isPointer(x.typ()) || isPointer(y.typ()) {
			return false
		}
		return true
	}

	if mayConvert(x, y) {
		check.convertUntyped(x, y.typ())
		if !x.isValid() {
			return
		}
		check.convertUntyped(y, x.typ())
		if !y.isValid() {
			x.invalidate()
			return
		}
	}
}

// exprKind describes the kind of an expression; the kind
// determines if an expression is valid in 'statement context'.
type exprKind int

const (
	conversion exprKind = iota
	expression
	statement
)

// target represent the (signature) type and description of the LHS
// variable of an assignment, or of a function result variable.
type target struct {
	sig  *Signature
	desc string
}

// newTarget creates a new target for the given type and description.
// The result is nil if typ is not a signature.
func newTarget(typ Type, desc string) *target {
	if typ != nil {
		if u, _ := commonUnder(typ, nil); u != nil {
			if sig, _ := u.(*Signature); sig != nil {
				return &target{sig, desc}
			}
		}
	}
	return nil
}

// rawExpr typechecks expression e and initializes x with the expression
// value or type. If an error occurred, x.mode is set to invalid.
// If a non-nil target T is given and e is a generic function,
// T is used to infer the type arguments for e.
// If hint != nil, it is the type of a composite literal element.
// If allowGeneric is set, the operand type may be an uninstantiated
// parameterized type or function value.
func (check *Checker) rawExpr(T *target, x *operand, e syntax.Expr, hint Type, allowGeneric bool) exprKind {
	if check.conf.Trace {
		check.trace(e.Pos(), "-- expr %s", e)
		check.indent++
		defer func() {
			check.indent--
			check.trace(e.Pos(), "=> %s", x)
		}()
	}

	kind := check.exprInternal(T, x, e, hint)

	if !allowGeneric {
		check.nonGeneric(T, x)
	}

	check.record(x)

	return kind
}

// If x is a generic type, or a generic function whose type arguments cannot be inferred
// from a non-nil target T, nonGeneric reports an error and invalidates x.mode and x.typ.
// Otherwise it leaves x alone.
func (check *Checker) nonGeneric(T *target, x *operand) {
	if !x.isValid() || x.mode() == novalue {
		return
	}
	var what string
	switch t := x.typ().(type) {
	case *Alias, *Named:
		if isGeneric(t) {
			what = "type"
		}
	case *Signature:
		if t.tparams != nil {
			if enableReverseTypeInference && T != nil {
				check.funcInst(T, x.Pos(), x, nil, true)
				return
			}
			what = "function"
		}
	}
	if what != "" {
		check.errorf(x.expr, WrongTypeArgCount, "cannot use generic %s %s without instantiation", what, x.expr)
		x.invalidate()
		x.typ_ = Typ[Invalid]
	}
}

// exprInternal contains the core of type checking of expressions.
// Must only be called by rawExpr.
// (See rawExpr for an explanation of the parameters.)
func (check *Checker) exprInternal(T *target, x *operand, e syntax.Expr, hint Type) exprKind {
	// make sure x has a valid state in case of bailout
	// (was go.dev/issue/5770)
	x.invalidate()
	x.typ_ = Typ[Invalid]

	switch e := e.(type) {
	case nil:
		panic("unreachable")

	case *syntax.BadExpr:
		goto Error // error was reported before

	case *syntax.Name:
		check.ident(x, e, false)

	case *syntax.DotsType:
		// dots are handled explicitly where they are valid
		check.error(e, InvalidSyntaxTree, "invalid use of ...")
		goto Error

	case *syntax.BasicLit:
		if e.Bad {
			goto Error // error reported during parsing
		}
		check.basicLit(x, e)
		if !x.isValid() {
			goto Error
		}

	case *syntax.FuncLit:
		check.funcLit(x, e)
		if !x.isValid() {
			goto Error
		}

	case *syntax.CompositeLit:
		check.compositeLit(x, e, hint)
		if !x.isValid() {
			goto Error
		}

	case *syntax.ParenExpr:
		// type inference doesn't go past parentheses (target type T = nil)
		kind := check.rawExpr(nil, x, e.X, nil, false)
		x.expr = e
		return kind

	case *syntax.SelectorExpr:
		check.selector(x, e, false)

	case *syntax.IndexExpr:
		if check.indexExpr(x, e) {
			if !enableReverseTypeInference {
				T = nil
			}
			check.funcInst(T, e.Pos(), x, e, true)
		}
		if !x.isValid() {
			goto Error
		}

	case *syntax.SliceExpr:
		check.sliceExpr(x, e)
		if !x.isValid() {
			goto Error
		}

	case *syntax.AssertExpr:
		check.expr(nil, x, e.X)
		if !x.isValid() {
			goto Error
		}
		// x.(type) expressions are encoded via TypeSwitchGuards
		if e.Type == nil {
			check.error(e, InvalidSyntaxTree, "invalid use of AssertExpr")
			goto Error
		}
		if isTypeParam(x.typ()) {
			check.errorf(x, InvalidAssert, invalidOp+"cannot use type assertion on type parameter value %s", x)
			goto Error
		}
		if _, ok := x.typ().Underlying().(*Interface); !ok {
			check.errorf(x, InvalidAssert, invalidOp+"%s is not an interface", x)
			goto Error
		}
		T := check.varType(e.Type)
		if !isValid(T) {
			goto Error
		}
		// We cannot assert to an incomplete type; make sure it's complete.
		if !check.isComplete(T) {
			goto Error
		}
		check.typeAssertion(e, x, T, false)
		x.mode_ = commaok
		x.typ_ = T

	case *syntax.TypeSwitchGuard:
		// x.(type) expressions are handled explicitly in type switches
		check.error(e, InvalidSyntaxTree, "use of .(type) outside type switch")
		check.use(e.X)
		goto Error

	case *syntax.CallExpr:
		return check.callExpr(x, e)

	case *syntax.ListExpr:
		// catch-all for unexpected expression lists
		check.error(e, InvalidSyntaxTree, "unexpected list of expressions")
		goto Error

	// case *syntax.UnaryExpr:
	// 	check.expr(x, e.X)
	// 	if x.mode == invalid {
	// 		goto Error
	// 	}
	// 	check.unary(x, e, e.Op)
	// 	if x.mode == invalid {
	// 		goto Error
	// 	}
	// 	if e.Op == token.ARROW {
	// 		x.expr = e
	// 		return statement // receive operations may appear in statement context
	// 	}

	// case *syntax.BinaryExpr:
	// 	check.binary(x, e, e.X, e.Y, e.Op)
	// 	if x.mode == invalid {
	// 		goto Error
	// 	}

	case *syntax.Operation:
		if e.Y == nil {
			// unary expression
			if e.Op == syntax.Mul {
				// pointer indirection
				check.exprOrType(x, e.X, false)
				switch x.mode() {
				case invalid:
					goto Error
				case typexpr:
					check.validVarType(e.X, x.typ())
					x.typ_ = &Pointer{base: x.typ()}
				default:
					var base Type
					if !underIs(x.typ(), func(u Type) bool {
						p, _ := u.(*Pointer)
						if p == nil {
							check.errorf(x, InvalidIndirection, invalidOp+"cannot indirect %s", x)
							return false
						}
						if base != nil && !Identical(p.base, base) {
							check.errorf(x, InvalidIndirection, invalidOp+"pointers of %s must have identical base types", x)
							return false
						}
						base = p.base
						return true
					}) {
						goto Error
					}
					// We cannot dereference a pointer with an incomplete base type; make sure it's complete.
					if !check.isComplete(base) {
						goto Error
					}
					x.mode_ = variable
					x.typ_ = base
				}
				break
			}

			check.unary(x, e)
			if !x.isValid() {
				goto Error
			}
			if e.Op == syntax.Recv {
				x.expr = e
				return statement // receive operations may appear in statement context
			}
			break
		}

		// binary expression
		check.binary(x, e, e.X, e.Y, e.Op)
		if !x.isValid() {
			goto Error
		}

	case *syntax.KeyValueExpr:
		// key:value expressions are handled in composite literals
		check.error(e, InvalidSyntaxTree, "no key:value expected")
		goto Error

	case *syntax.ArrayType, *syntax.SliceType, *syntax.StructType, *syntax.FuncType,
		*syntax.InterfaceType, *syntax.MapType, *syntax.ChanType:
		x.mode_ = typexpr
		x.typ_ = check.typ(e)
		// Note: rawExpr (caller of exprInternal) will call check.recordTypeAndValue
		// even though check.typ has already called it. This is fine as both
		// times the same expression and type are recorded. It is also not a
		// performance issue because we only reach here for composite literal
		// types, which are comparatively rare.

	default:
		panic(fmt.Sprintf("%s: unknown expression type %T", atPos(e), e))
	}

	// everything went well
	x.expr = e
	return expression

Error:
	x.invalidate()
	x.expr = e
	return statement // avoid follow-up errors
}

// keyVal maps a complex, float, integer, string or boolean constant value
// to the corresponding complex128, float64, int64, uint64, string, or bool
// Go value if possible; otherwise it returns x.
// A complex constant that can be represented as a float (such as 1.2 + 0i)
// is returned as a floating point value; if a floating point value can be
// represented as an integer (such as 1.0) it is returned as an integer value.
// This ensures that constants of different kind but equal value (such as
// 1.0 + 0i, 1.0, 1) result in the same value.
func keyVal(x constant.Value) any {
	switch x.Kind() {
	case constant.Complex:
		f := constant.ToFloat(x)
		if f.Kind() != constant.Float {
			r, _ := constant.Float64Val(constant.Real(x))
			i, _ := constant.Float64Val(constant.Imag(x))
			return complex(r, i)
		}
		x = f
		fallthrough
	case constant.Float:
		i := constant.ToInt(x)
		if i.Kind() != constant.Int {
			v, _ := constant.Float64Val(x)
			return v
		}
		x = i
		fallthrough
	case constant.Int:
		if v, ok := constant.Int64Val(x); ok {
			return v
		}
		if v, ok := constant.Uint64Val(x); ok {
			return v
		}
	case constant.String:
		return constant.StringVal(x)
	case constant.Bool:
		return constant.BoolVal(x)
	}
	return x
}

// typeAssertion checks x.(T). The type of x must be an interface.
func (check *Checker) typeAssertion(e syntax.Expr, x *operand, T Type, typeSwitch bool) {
	var cause string
	if check.assertableTo(x.typ(), T, &cause) {
		return // success
	}

	if typeSwitch {
		check.errorf(e, ImpossibleAssert, "impossible type switch case: %s\n\t%s cannot have dynamic type %s %s", e, x, T, cause)
		return
	}

	check.errorf(e, ImpossibleAssert, "impossible type assertion: %s\n\t%s does not implement %s %s", e, T, x.typ(), cause)
}

// expr typechecks expression e and initializes x with the expression value.
// If a non-nil target T is given and e is a generic function or
// a function call, T is used to infer the type arguments for e.
// The result must be a single value.
// If an error occurred, x.mode is set to invalid.
func (check *Checker) expr(T *target, x *operand, e syntax.Expr) {
	check.rawExpr(T, x, e, nil, false)
	check.exclude(x, 1<<novalue|1<<builtin|1<<typexpr)
	check.singleValue(x)
}

// genericExpr is like expr but the result may also be generic.
func (check *Checker) genericExpr(x *operand, e syntax.Expr, hint Type) {
	check.rawExpr(nil, x, e, hint, true)
	check.exclude(x, 1<<novalue|1<<builtin|1<<typexpr)
	check.singleValue(x)
}

// multiExpr typechecks e and returns its value (or values) in list.
// If allowCommaOk is set and e is a map index, comma-ok, or comma-err
// expression, the result is a two-element list containing the value
// of e, and an untyped bool value or an error value, respectively.
// If an error occurred, list[0] is not valid.
func (check *Checker) multiExpr(e syntax.Expr, allowCommaOk bool) (list []*operand, commaOk bool) {
	var x operand
	check.rawExpr(nil, &x, e, nil, false)
	check.exclude(&x, 1<<novalue|1<<builtin|1<<typexpr)

	if t, ok := x.typ().(*Tuple); ok && x.isValid() {
		// multiple values
		list = make([]*operand, t.Len())
		for i, v := range t.vars {
			// create a dummy expression (in place of e) for better error messages
			dummy := syntax.NewName(syntax.StartPos(e), nth(i+1, "function result"))
			list[i] = &operand{mode_: value, expr: dummy, typ_: v.typ}
		}
		return
	}

	// exactly one (possibly invalid or comma-ok) value
	list = []*operand{&x}
	if allowCommaOk && (x.mode() == mapindex || x.mode() == commaok || x.mode() == commaerr) {
		var what string = "ok value of (comma, ok) expression"
		var typ Type = Typ[UntypedBool]
		if x.mode() == commaerr {
			what = "err value of (comma, err) expression"
			typ = universeError
		}
		// create a dummy expression (in place of e) for better error messages
		dummy := syntax.NewName(syntax.StartPos(e), what)
		x2 := &operand{mode_: value, expr: dummy, typ_: typ}
		list = append(list, x2)
		commaOk = true
	}

	return
}

// nth returns a string of the form "nth " + what, where nth
// stands for 1st, 2nd, 3rd, 4th, etc. depending on n.
func nth(n int, what string) string {
	var ext string
	switch n {
	case 1:
		ext = "st"
	case 2:
		ext = "nd"
	case 3:
		ext = "rd"
	default:
		ext = "th"
	}
	return fmt.Sprintf("%d%s %s", n, ext, what)
}

// exprOrType typechecks expression or type e and initializes x with the expression value or type.
// If allowGeneric is set, the operand type may be an uninstantiated parameterized type or function
// value.
// If an error occurred, x.mode is set to invalid.
func (check *Checker) exprOrType(x *operand, e syntax.Expr, allowGeneric bool) {
	check.rawExpr(nil, x, e, nil, allowGeneric)
	check.exclude(x, 1<<novalue)
	check.singleValue(x)
}

// exclude reports an error if x.mode is in modeset and sets x.mode to invalid.
// The modeset may contain any of 1<<novalue, 1<<builtin, 1<<typexpr.
func (check *Checker) exclude(x *operand, modeset uint) {
	if modeset&(1<<x.mode()) != 0 {
		var msg string
		var code Code
		switch x.mode() {
		case novalue:
			if modeset&(1<<typexpr) != 0 {
				msg = "%s used as value"
			} else {
				msg = "%s used as value or type"
			}
			code = TooManyValues
		case builtin:
			msg = "%s must be called"
			code = UncalledBuiltin
		case typexpr:
			msg = "%s is not an expression"
			code = NotAnExpr
		default:
			panic("unreachable")
		}
		check.errorf(x, code, msg, x)
		x.invalidate()
	}
}

// singleValue reports an error if x describes a tuple and sets x.mode to invalid.
func (check *Checker) singleValue(x *operand) {
	if x.mode() == value {
		// tuple types are never named - no need for underlying type below
		if t, ok := x.typ().(*Tuple); ok {
			assert(t.Len() != 1)
			check.errorf(x, TooManyValues, "multiple-value %s in single-value context", x)
			x.invalidate()
		}
	}
}

// op2tok translates syntax.Operators into token.Tokens.
var op2tok = [...]token.Token{
	syntax.Def:  token.ILLEGAL,
	syntax.Not:  token.NOT,
	syntax.Recv: token.ILLEGAL,

	syntax.OrOr:   token.LOR,
	syntax.AndAnd: token.LAND,

	syntax.Eql: token.EQL,
	syntax.Neq: token.NEQ,
	syntax.Lss: token.LSS,
	syntax.Leq: token.LEQ,
	syntax.Gtr: token.GTR,
	syntax.Geq: token.GEQ,

	syntax.Add: token.ADD,
	syntax.Sub: token.SUB,
	syntax.Or:  token.OR,
	syntax.Xor: token.XOR,

	syntax.Mul:    token.MUL,
	syntax.Div:    token.QUO,
	syntax.Rem:    token.REM,
	syntax.And:    token.AND,
	syntax.AndNot: token.AND_NOT,
	syntax.Shl:    token.SHL,
	syntax.Shr:    token.SHR,
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/format.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements (error and trace) message formatting support.

package types2

import (
	"bytes"
	"cmd/compile/internal/syntax"
	"fmt"
	"strconv"
	"strings"
)

func sprintf(qf Qualifier, tpSubscripts bool, format string, args ...any) string {
	for i, arg := range args {
		switch a := arg.(type) {
		case nil:
			arg = "<nil>"
		case operand:
			panic("got operand instead of *operand")
		case *operand:
			arg = operandString(a, qf)
		case []*operand:
			var buf strings.Builder
			buf.WriteByte('[')
			for i, x := range a {
				if i > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(operandString(x, qf))
			}
			buf.WriteByte(']')
			arg = buf.String()
		case syntax.Pos:
			arg = a.String()
		case syntax.Expr:
			arg = ExprString(a)
		case []syntax.Expr:
			var buf strings.Builder
			buf.WriteByte('[')
			for i, x := range a {
				if i > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(ExprString(x))
			}
			buf.WriteByte(']')
			arg = buf.String()
		case Object:
			arg = ObjectString(a, qf)
		case Type:
			var buf bytes.Buffer
			w := newTypeWriter(&buf, qf)
			w.tpSubscripts = tpSubscripts
			w.typ(a)
			arg = buf.String()
		case []Type:
			var buf bytes.Buffer
			w := newTypeWriter(&buf, qf)
			w.tpSubscripts = tpSubscripts
			buf.WriteByte('[')
			for i, x := range a {
				if i > 0 {
					buf.WriteString(", ")
				}
				w.typ(x)
			}
			buf.WriteByte(']')
			arg = buf.String()
		case []*TypeParam:
			var buf bytes.Buffer
			w := newTypeWriter(&buf, qf)
			w.tpSubscripts = tpSubscripts
			buf.WriteByte('[')
			for i, x := range a {
				if i > 0 {
					buf.WriteString(", ")
				}
				w.typ(x)
			}
			buf.WriteByte(']')
			arg = buf.String()
		}
		args[i] = arg
	}
	return fmt.Sprintf(format, args...)
}

// check may be nil.
func (check *Checker) sprintf(format string, args ...any) string {
	var qf Qualifier
	if check != nil {
		qf = check.qualifier
	}
	return sprintf(qf, false, format, args...)
}

func (check *Checker) trace(pos syntax.Pos, format string, args ...any) {
	// Use the width of line and pos values to align the ":" by adding padding before it.
	// Cap padding at 5: 3 digits for the line, 2 digits for the column number, which is
	// ok for most cases.
	w := ndigits(pos.Line()) + ndigits(pos.Col())
	pad := "     "[:max(5-w, 0)]
	fmt.Printf("%s%s:  %s%s\n",
		pos,
		pad,
		strings.Repeat(".  ", check.indent),
		sprintf(check.qualifier, true, format, args...),
	)
}

// ndigits returns the number of decimal digits in x.
// For x > 100, the result is always 3.
func ndigits(x uint) int {
	switch {
	case x < 10:
		return 1
	case x < 100:
		return 2
	default:
		return 3
	}
}

// dump is only needed for debugging
func (check *Checker) dump(format string, args ...any) {
	fmt.Println(sprintf(check.qualifier, true, format, args...))
}

func (check *Checker) qualifier(pkg *Package) string {
	// Qualify the package unless it's the package being type-checked.
	if pkg != check.pkg {
		if check.pkgPathMap == nil {
			check.pkgPathMap = make(map[string]map[string]bool)
			check.seenPkgMap = make(map[*Package]bool)
			check.markImports(check.pkg)
		}
		// If the same package name was used by multiple packages, display the full path.
		if len(check.pkgPathMap[pkg.name]) > 1 {
			return strconv.Quote(pkg.path)
		}
		return pkg.name
	}
	return ""
}

// markImports recursively walks pkg and its imports, to record unique import
// paths in pkgPathMap.
func (check *Checker) markImports(pkg *Package) {
	if check.seenPkgMap[pkg] {
		return
	}
	check.seenPkgMap[pkg] = true

	forName, ok := check.pkgPathMap[pkg.name]
	if !ok {
		forName = make(map[string]bool)
		check.pkgPathMap[pkg.name] = forName
	}
	forName[pkg.path] = true

	for _, imp := range pkg.imports {
		check.markImports(imp)
	}
}

// stripAnnotations removes internal (type) annotations from s.
func stripAnnotations(s string) string {
	var buf strings.Builder
	for _, r := range s {
		// strip #'s and subscript digits
		if r < '₀' || '₀'+10 <= r { // '₀' == U+2080
			buf.WriteRune(r)
		}
	}
	if buf.Len() < len(s) {
		return buf.String()
	}
	return s
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/gccgosizes.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a copy of the file generated during the gccgo build process.
// Last update 2019-01-22.

package types2

var gccgoArchSizes = map[string]*StdSizes{
	"386":         {4, 4},
	"alpha":       {8, 8},
	"amd64":       {8, 8},
	"amd64p32":    {4, 8},
	"arm":         {4, 8},
	"armbe":       {4, 8},
	"arm64":       {8, 8},
	"arm64be":     {8, 8},
	"ia64":        {8, 8},
	"loong64":     {8, 8},
	"m68k":        {4, 2},
	"mips":        {4, 8},
	"mipsle":      {4, 8},
	"mips64":      {8, 8},
	"mips64le":    {8, 8},
	"mips64p32":   {4, 8},
	"mips64p32le": {4, 8},
	"nios2":       {4, 8},
	"ppc":         {4, 8},
	"ppc64":       {8, 8},
	"ppc64le":     {8, 8},
	"riscv":       {4, 8},
	"riscv64":     {8, 8},
	"s390":        {4, 8},
	"s390x":       {8, 8},
	"sh":          {4, 8},
	"shbe":        {4, 8},
	"sparc":       {4, 8},
	"sparc64":     {8, 8},
	"wasm":        {8, 8},
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/gcsizes.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

type gcSizes struct {
	WordSize int64 // word size in bytes - must be >= 4 (32bits)
	MaxAlign int64 // maximum alignment in bytes - must be >= 1
}

func (s *gcSizes) Alignof(T Type) (result int64) {
	defer func() {
		assert(result >= 1)
	}()

	// For arrays and structs, alignment is defined in terms
	// of alignment of the elements and fields, respectively.
	switch t := T.Underlying().(type) {
	case *Array:
		// spec: "For a variable x of array type: unsafe.Alignof(x)
		// is the same as unsafe.Alignof(x[0]), but at least 1."
		return s.Alignof(t.elem)
	case *Struct:
		if len(t.fields) == 0 && IsSyncAtomicAlign64(T) {
			// Special case: sync/atomic.align64 is an
			// empty struct we recognize as a signal that
			// the struct it contains must be
			// 64-bit-aligned.
			//
			// This logic is equivalent to the logic in
			// cmd/compile/internal/types/size.go:calcStructOffset
			return 8
		}

		// spec: "For a variable x of struct type: unsafe.Alignof(x)
		// is the largest of the values unsafe.Alignof(x.f) for each
		// field f of x, but at least 1."
		max := int64(1)
		for _, f := range t.fields {
			if a := s.Alignof(f.typ); a > max {
				max = a
			}
		}
		return max
	case *Slice, *Interface:
		// Multiword data structures are effectively structs
		// in which each element has size WordSize.
		// Type parameters lead to variable sizes/alignments;
		// StdSizes.Alignof won't be called for them.
		assert(!isTypeParam(T))
		return s.WordSize
	case *Basic:
		// Strings are like slices and interfaces.
		if t.Info()&IsString != 0 {
			return s.WordSize
		}
	case *TypeParam, *Union:
		panic("unreachable")
	}
	a := s.Sizeof(T) // may be 0 or negative
	// spec: "For a variable x of any type: unsafe.Alignof(x) is at least 1."
	if a < 1 {
		return 1
	}
	// complex{64,128} are aligned like [2]float{32,64}.
	if isComplex(T) {
		a /= 2
	}
	if a > s.MaxAlign {
		return s.MaxAlign
	}
	return a
}

func (s *gcSizes) Offsetsof(fields []*Var) []int64 {
	offsets := make([]int64, len(fields))
	var offs int64
	for i, f := range fields {
		if offs < 0 {
			// all remaining offsets are too large
			offsets[i] = -1
			continue
		}
		// offs >= 0
		a := s.Alignof(f.typ)
		offs = align(offs, a) // possibly < 0 if align overflows
		offsets[i] = offs
		if d := s.Sizeof(f.typ); d >= 0 && offs >= 0 {
			offs += d // ok to overflow to < 0
		} else {
			offs = -1 // f.typ or offs is too large
		}
	}
	return offsets
}

func (s *gcSizes) Sizeof(T Type) int64 {
	switch t := T.Underlying().(type) {
	case *Basic:
		assert(isTyped(T))
		k := t.kind
		if int(k) < len(basicSizes) {
			if s := basicSizes[k]; s > 0 {
				return int64(s)
			}
		}
		if k == String {
			return s.WordSize * 2
		}
	case *Array:
		n := t.len
		if n <= 0 {
			return 0
		}
		// n > 0
		esize := s.Sizeof(t.elem)
		if esize < 0 {
			return -1 // element too large
		}
		if esize == 0 {
			return 0 // 0-size element
		}
		// esize > 0
		// Final size is esize * n; and size must be <= maxInt64.
		const maxInt64 = 1<<63 - 1
		if esize > maxInt64/n {
			return -1 // esize * n overflows
		}
		return esize * n
	case *Slice:
		return s.WordSize * 3
	case *Struct:
		n := t.NumFields()
		if n == 0 {
			return 0
		}
		offsets := s.Offsetsof(t.fields)
		offs := offsets[n-1]
		size := s.Sizeof(t.fields[n-1].typ)
		if offs < 0 || size < 0 {
			return -1 // type too large
		}
		// gc: The last field of a non-zero-sized struct is not allowed to
		// have size 0.
		if offs > 0 && size == 0 {
			size = 1
		}
		// gc: Size includes alignment padding.
		return align(offs+size, s.Alignof(t)) // may overflow to < 0 which is ok
	case *Interface:
		// Type parameters lead to variable sizes/alignments;
		// StdSizes.Sizeof won't be called for them.
		assert(!isTypeParam(T))
		return s.WordSize * 2
	case *TypeParam, *Union:
		panic("unreachable")
	}
	return s.WordSize // catch-all
}

// gcSizesFor returns the Sizes used by gc for an architecture.
// The result is a nil *gcSizes pointer (which is not a valid types.Sizes)
// if a compiler/architecture pair is not known.
func gcSizesFor(compiler, arch string) *gcSizes {
	if compiler != "gc" {
		return nil
	}
	return gcArchSizes[arch]
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/index.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of index/slice expressions.

package types2

import (
	"cmd/compile/internal/syntax"
	"go/constant"
	. "internal/types/errors"
)

// If e is a valid function instantiation, indexExpr returns true.
// In that case x represents the uninstantiated function value and
// it is the caller's responsibility to instantiate the function.
func (check *Checker) indexExpr(x *operand, e *syntax.IndexExpr) (isFuncInst bool) {
	check.exprOrType(x, e.X, true)
	// x may be generic

	switch x.mode() {
	case invalid:
		check.use(e.Index)
		return false

	case typexpr:
		// type instantiation
		x.invalidate()
		// TODO(gri) here we re-evaluate e.X - try to avoid this
		x.typ_ = check.varType(e)
		if isValid(x.typ()) {
			x.mode_ = typexpr
		}
		return false

	case value:
		if sig, _ := x.typ().Underlying().(*Signature); sig != nil && sig.TypeParams().Len() > 0 {
			// function instantiation
			return true
		}
	}

	// x should not be generic at this point, but be safe and check
	check.nonGeneric(nil, x)
	if !x.isValid() {
		return false
	}

	// We cannot index on an incomplete type; make sure it's complete.
	if !check.isComplete(x.typ()) {
		x.invalidate()
		return false
	}
	switch typ := x.typ().Underlying().(type) {
	case *Pointer:
		// Additionally, if x.typ is a pointer to an array type, indexing implicitly dereferences the value, meaning
		// its base type must also be complete.
		if !check.isComplete(typ.base) {
			x.invalidate()
			return false
		}
	case *Map:
		// Lastly, if x.typ is a map type, indexing must produce a value of a complete type, meaning
		// its element type must also be complete.
		if !check.isComplete(typ.elem) {
			x.invalidate()
			return false
		}
	}

	// ordinary index expression
	valid := false
	length := int64(-1) // valid if >= 0
	switch typ := x.typ().Underlying().(type) {
	case *Basic:
		if isString(typ) {
			valid = true
			if x.mode() == constant_ {
				length = constant.StringLen(x.val)
			}
			// an indexed string always yields a byte value
			// (not a constant) even if the string and the
			// index are constant
			x.mode_ = value
			x.typ_ = universeByte // use 'byte' name
		}

	case *Array:
		valid = true
		length = typ.len
		if x.mode() != variable {
			x.mode_ = value
		}
		x.typ_ = typ.elem

	case *Pointer:
		if typ, _ := typ.base.Underlying().(*Array); typ != nil {
			valid = true
			length = typ.len
			x.mode_ = variable
			x.typ_ = typ.elem
		}

	case *Slice:
		valid = true
		x.mode_ = variable
		x.typ_ = typ.elem

	case *Map:
		index := check.singleIndex(e)
		if index == nil {
			x.invalidate()
			return false
		}
		var key operand
		check.genericExpr(&key, index, nil)
		check.assignment(&key, typ.key, "map index")
		// ok to continue even if indexing failed - map element type is known
		x.mode_ = mapindex
		x.typ_ = typ.elem
		x.expr = e
		return false

	case *Interface:
		if !isTypeParam(x.typ()) {
			break
		}
		// TODO(gri) report detailed failure cause for better error messages
		var key, elem Type // key != nil: we must have all maps
		mode := variable   // non-maps result mode
		// TODO(gri) factor out closure and use it for non-typeparam cases as well
		if underIs(x.typ(), func(u Type) bool {
			l := int64(-1) // valid if >= 0
			var k, e Type  // k is only set for maps
			switch t := u.(type) {
			case *Basic:
				if isString(t) {
					e = universeByte
					mode = value
				}
			case *Array:
				l = t.len
				e = t.elem
				if x.mode() != variable {
					mode = value
				}
			case *Pointer:
				if t, _ := t.base.Underlying().(*Array); t != nil {
					l = t.len
					e = t.elem
				}
			case *Slice:
				e = t.elem
			case *Map:
				k = t.key
				e = t.elem
			}
			if e == nil {
				return false
			}
			if elem == nil {
				// first type
				length = l
				key, elem = k, e
				return true
			}
			// all map keys must be identical (incl. all nil)
			// (that is, we cannot mix maps with other types)
			if !Identical(key, k) {
				return false
			}
			// all element types must be identical
			if !Identical(elem, e) {
				return false
			}
			// track the minimal length for arrays, if any
			if l >= 0 && l < length {
				length = l
			}
			return true
		}) {
			// For maps, the index expression must be assignable to the map key type.
			if key != nil {
				index := check.singleIndex(e)
				if index == nil {
					x.invalidate()
					return false
				}
				var k operand
				check.genericExpr(&k, index, nil)
				check.assignment(&k, key, "map index")
				// ok to continue even if indexing failed - map element type is known
				x.mode_ = mapindex
				x.typ_ = elem
				x.expr = e
				return false
			}

			// no maps
			valid = true
			x.mode_ = mode
			x.typ_ = elem
		}
	}

	if !valid {
		check.errorf(e.Pos(), NonSliceableOperand, "cannot index %s", x)
		check.use(e.Index)
		x.invalidate()
		return false
	}

	index := check.singleIndex(e)
	if index == nil {
		x.invalidate()
		return false
	}

	// In pathological (invalid) cases (e.g.: type T1 [][[]T1{}[0][0]]T0)
	// the element type may be accessed before it's set. Make sure we have
	// a valid type.
	if x.typ() == nil {
		x.typ_ = Typ[Invalid]
	}

	check.index(index, length)
	return false
}

func (check *Checker) sliceExpr(x *operand, e *syntax.SliceExpr) {
	check.expr(nil, x, e.X)
	if !x.isValid() {
		check.use(e.Index[:]...)
		return
	}

	// determine common underlying type cu
	var ct, cu Type // type and respective common underlying type
	var hasString bool
	for t, u := range typeset(x.typ()) {
		if u == nil {
			check.errorf(x, NonSliceableOperand, "cannot slice %s: no specific type in %s", x, x.typ())
			cu = nil
			break
		}

		// Treat strings like byte slices but remember that we saw a string.
		if isString(u) {
			u = NewSlice(universeByte)
			hasString = true
		}

		// If this is the first type we're seeing, we're done.
		if cu == nil {
			ct, cu = t, u
			continue
		}

		// Otherwise, the current type must have the same underlying type as all previous types.
		if !Identical(cu, u) {
			check.errorf(x, NonSliceableOperand, "cannot slice %s: %s and %s have different underlying types", x, ct, t)
			cu = nil
			break
		}
	}
	if hasString {
		// If we saw a string, proceed with string type,
		// but don't go from untyped string to string.
		cu = Typ[String]
		if !isTypeParam(x.typ()) {
			cu = x.typ().Underlying() // untyped string remains untyped
		}
	}

	// Note that we don't permit slice expressions where x is a type expression, so we don't check for that here.
	// However, if x.typ is a pointer to an array type, slicing implicitly dereferences the value, meaning
	// its base type must also be complete.
	if p, ok := x.typ().Underlying().(*Pointer); ok && !check.isComplete(p.base) {
		x.invalidate()
		return
	}

	valid := false
	length := int64(-1) // valid if >= 0
	switch u := cu.(type) {
	case nil:
		// error reported above
		x.invalidate()
		return

	case *Basic:
		if isString(u) {
			if e.Full {
				at := e.Index[2]
				if at == nil {
					at = e // e.Index[2] should be present but be careful
				}
				check.error(at, InvalidSliceExpr, invalidOp+"3-index slice of string")
				x.invalidate()
				return
			}
			valid = true
			if x.mode() == constant_ {
				length = constant.StringLen(x.val)
			}
			// spec: "For untyped string operands the result
			// is a non-constant value of type string."
			if isUntyped(x.typ()) {
				x.typ_ = Typ[String]
			}
		}

	case *Array:
		valid = true
		length = u.len
		if x.mode() != variable {
			check.errorf(x, NonSliceableOperand, "cannot slice unaddressable value %s", x)
			x.invalidate()
			return
		}
		x.typ_ = &Slice{elem: u.elem}

	case *Pointer:
		if u, _ := u.base.Underlying().(*Array); u != nil {
			valid = true
			length = u.len
			x.typ_ = &Slice{elem: u.elem}
		}

	case *Slice:
		valid = true
		// x.typ doesn't change
	}

	if !valid {
		check.errorf(x, NonSliceableOperand, "cannot slice %s", x)
		x.invalidate()
		return
	}

	x.mode_ = value

	// spec: "Only the first index may be omitted; it defaults to 0."
	if e.Full && (e.Index[1] == nil || e.Index[2] == nil) {
		check.error(e, InvalidSyntaxTree, "2nd and 3rd index required in 3-index slice")
		x.invalidate()
		return
	}

	// check indices
	var ind [3]int64
	for i, expr := range e.Index {
		x := int64(-1)
		switch {
		case expr != nil:
			// The "capacity" is only known statically for strings, arrays,
			// and pointers to arrays, and it is the same as the length for
			// those types.
			max := int64(-1)
			if length >= 0 {
				max = length + 1
			}
			if _, v := check.index(expr, max); v >= 0 {
				x = v
			}
		case i == 0:
			// default is 0 for the first index
			x = 0
		case length >= 0:
			// default is length (== capacity) otherwise
			x = length
		}
		ind[i] = x
	}

	// constant indices must be in range
	// (check.index already checks that existing indices >= 0)
L:
	for i, x := range ind[:len(ind)-1] {
		if x > 0 {
			for j, y := range ind[i+1:] {
				if y >= 0 && y < x {
					// The value y corresponds to the expression e.Index[i+1+j].
					// Because y >= 0, it must have been set from the expression
					// when checking indices and thus e.Index[i+1+j] is not nil.
					check.errorf(e.Index[i+1+j], SwappedSliceIndices, "invalid slice indices: %d < %d", y, x)
					break L // only report one error, ok to continue
				}
			}
		}
	}
}

// singleIndex returns the (single) index from the index expression e.
// If the index is missing, or if there are multiple indices, an error
// is reported and the result is nil.
func (check *Checker) singleIndex(e *syntax.IndexExpr) syntax.Expr {
	index := e.Index
	if index == nil {
		check.errorf(e, InvalidSyntaxTree, "missing index for %s", e.X)
		return nil
	}
	if l, _ := index.(*syntax.ListExpr); l != nil {
		if n := len(l.ElemList); n <= 1 {
			check.errorf(e, InvalidSyntaxTree, "invalid use of ListExpr for index expression %v with %d indices", e, n)
			return nil
		}
		// len(l.ElemList) > 1
		check.error(l.ElemList[1], InvalidIndex, invalidOp+"more than one index")
		index = l.ElemList[0] // continue with first index
	}
	return index
}

// index checks an index expression for validity.
// If max >= 0, it is the upper bound for index.
// If the result typ is != Typ[Invalid], index is valid and typ is its (possibly named) integer type.
// If the result val >= 0, index is valid and val is its constant int value.
func (check *Checker) index(index syntax.Expr, max int64) (typ Type, val int64) {
	typ = Typ[Invalid]
	val = -1

	var x operand
	check.expr(nil, &x, index)
	if !check.isValidIndex(&x, InvalidIndex, "index", false) {
		return
	}

	if x.mode() != constant_ {
		return x.typ(), -1
	}

	if x.val.Kind() == constant.Unknown {
		return
	}

	v, ok := constant.Int64Val(x.val)
	assert(ok)
	if max >= 0 && v >= max {
		check.errorf(&x, InvalidIndex, invalidArg+"index %s out of bounds [0:%d]", x.val.String(), max)
		return
	}

	// 0 <= v [ && v < max ]
	return x.typ(), v
}

// isValidIndex checks whether operand x satisfies the criteria for integer
// index values. If allowNegative is set, a constant operand may be negative.
// If the operand is not valid, an error is reported (using what as context)
// and the result is false.
func (check *Checker) isValidIndex(x *operand, code Code, what string, allowNegative bool) bool {
	if !x.isValid() {
		return false
	}

	// spec: "a constant index that is untyped is given type int"
	check.convertUntyped(x, Typ[Int])
	if !x.isValid() {
		return false
	}

	// spec: "the index x must be of integer type or an untyped constant"
	if !allInteger(x.typ()) {
		check.errorf(x, code, invalidArg+"%s %s must be integer", what, x)
		return false
	}

	if x.mode() == constant_ {
		// spec: "a constant index must be non-negative ..."
		if !allowNegative && constant.Sign(x.val) < 0 {
			check.errorf(x, code, invalidArg+"%s %s must not be negative", what, x)
			return false
		}

		// spec: "... and representable by a value of type int"
		if !representableConst(x.val, check, Typ[Int], &x.val) {
			check.errorf(x, code, invalidArg+"%s %s overflows int", what, x)
			return false
		}
	}

	return true
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/infer.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements type parameter inference.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"slices"
	"strings"
)

// If enableReverseTypeInference is set, uninstantiated and
// partially instantiated generic functions may be assigned
// (incl. returned) to variables of function type and type
// inference will attempt to infer the missing type arguments.
// Available with go1.21.
const enableReverseTypeInference = true // disable for debugging

// infer attempts to infer the complete set of type arguments for generic function instantiation/call
// based on the given type parameters tparams, type arguments targs, function parameters params, and
// function arguments args, if any. There must be at least one type parameter, no more type arguments
// than type parameters, and params and args must match in number (incl. zero).
// If reverse is set, an error message's contents are reversed for a better error message for some
// errors related to reverse type inference (where the function call is synthetic).
// If successful, infer returns the complete list of given and inferred type arguments, one for each
// type parameter. Otherwise the result is nil. Errors are reported through the err parameter.
// Note: infer may fail (return nil) due to invalid args operands without reporting additional errors.
func (check *Checker) infer(pos syntax.Pos, tparams []*TypeParam, targs []Type, params *Tuple, args []*operand, reverse bool, err *error_) (inferred []Type) {
	// Don't verify result conditions if there's no error handler installed:
	// in that case, an error leads to an exit panic and the result value may
	// be incorrect. But in that case it doesn't matter because callers won't
	// be able to use it either.
	if check.conf.Error != nil {
		defer func() {
			assert(inferred == nil || len(inferred) == len(tparams) && !slices.Contains(inferred, nil))
		}()
	}

	if traceInference {
		check.dump("== infer : %s%s ➞ %s", tparams, params, targs) // aligned with rename print below
		defer func() {
			check.dump("=> %s ➞ %s\n", tparams, inferred)
		}()
	}

	// There must be at least one type parameter, and no more type arguments than type parameters.
	n := len(tparams)
	assert(n > 0 && len(targs) <= n)

	// Parameters and arguments must match in number.
	assert(params.Len() == len(args))

	// If we already have all type arguments, we're done.
	if len(targs) == n && !slices.Contains(targs, nil) {
		return targs
	}

	// If we have invalid (ordinary) arguments, an error was reported before.
	// Avoid additional inference errors and exit early (go.dev/issue/60434).
	for _, arg := range args {
		if !arg.isValid() {
			return nil
		}
	}

	// Make sure we have a "full" list of type arguments, some of which may
	// be nil (unknown). Make a copy so as to not clobber the incoming slice.
	if len(targs) < n {
		targs2 := make([]Type, n)
		copy(targs2, targs)
		targs = targs2
	}
	// len(targs) == n

	// Continue with the type arguments we have. Avoid matching generic
	// parameters that already have type arguments against function arguments:
	// It may fail because matching uses type identity while parameter passing
	// uses assignment rules. Instantiate the parameter list with the type
	// arguments we have, and continue with that parameter list.

	// Substitute type arguments for their respective type parameters in params,
	// if any. Note that nil targs entries are ignored by check.subst.
	// We do this for better error messages; it's not needed for correctness.
	// For instance, given:
	//
	//   func f[P, Q any](P, Q) {}
	//
	//   func _(s string) {
	//           f[int](s, s) // ERROR
	//   }
	//
	// With substitution, we get the error:
	//   "cannot use s (variable of type string) as int value in argument to f[int]"
	//
	// Without substitution we get the (worse) error:
	//   "type string of s does not match inferred type int for P"
	// even though the type int was provided (not inferred) for P.
	//
	// TODO(gri) We might be able to finesse this in the error message reporting
	//           (which only happens in case of an error) and then avoid doing
	//           the substitution (which always happens).
	if params.Len() > 0 {
		smap := makeSubstMap(tparams, targs)
		params = check.subst(nopos, params, smap, nil, check.context()).(*Tuple)
	}

	// Unify parameter and argument types for generic parameters with typed arguments
	// and collect the indices of generic parameters with untyped arguments.
	// Terminology: generic parameter = function parameter with a type-parameterized type
	u := newUnifier(check, tparams, targs, check.allowVersion(go1_21))

	errorf := func(tpar, targ Type, arg *operand) {
		// provide a better error message if we can
		targs := u.inferred(tparams)
		if targs[0] == nil {
			// The first type parameter couldn't be inferred.
			// If none of them could be inferred, don't try
			// to provide the inferred type in the error msg.
			allFailed := true
			for _, targ := range targs {
				if targ != nil {
					allFailed = false
					break
				}
			}
			if allFailed {
				err.addf(arg, "type %s of %s does not match %s (cannot infer %s)", targ, arg.expr, tpar, typeParamsString(tparams))
				return
			}
		}
		smap := makeSubstMap(tparams, targs)
		// TODO(gri): pass a poser here, rather than arg.Pos().
		inferred := check.subst(arg.Pos(), tpar, smap, nil, check.context())
		// CannotInferTypeArgs indicates a failure of inference, though the actual
		// error may be better attributed to a user-provided type argument (hence
		// InvalidTypeArg). We can't differentiate these cases, so fall back on
		// the more general CannotInferTypeArgs.
		if inferred != tpar {
			if reverse {
				err.addf(arg, "inferred type %s for %s does not match type %s of %s", inferred, tpar, targ, arg.expr)
			} else {
				err.addf(arg, "type %s of %s does not match inferred type %s for %s", targ, arg.expr, inferred, tpar)
			}
		} else {
			err.addf(arg, "type %s of %s does not match %s", targ, arg.expr, tpar)
		}
	}

	// indices of generic parameters with untyped arguments, for later use
	var untyped []int

	// --- 1 ---
	// use information from function arguments

	if traceInference {
		u.tracef("== function parameters: %s", params)
		u.tracef("-- function arguments : %s", args)
	}

	for i, arg := range args {
		if !arg.isValid() {
			// An error was reported earlier. Ignore this arg
			// and continue, we may still be able to infer all
			// targs resulting in fewer follow-on errors.
			// TODO(gri) determine if we still need this check
			continue
		}
		par := params.At(i)
		if isParameterized(tparams, par.typ) || isParameterized(tparams, arg.typ()) {
			// Function parameters are always typed. Arguments may be untyped.
			// Collect the indices of untyped arguments and handle them later.
			if isTyped(arg.typ()) {
				if !u.unify(par.typ, arg.typ(), assign) {
					errorf(par.typ, arg.typ(), arg)
					return nil
				}
			} else if _, ok := par.typ.(*TypeParam); ok && !arg.isNil() {
				// Since default types are all basic (i.e., non-composite) types, an
				// untyped argument will never match a composite parameter type; the
				// only parameter type it can possibly match against is a *TypeParam.
				// Thus, for untyped arguments we only need to look at parameter types
				// that are single type parameters.
				// Also, untyped nils don't have a default type and can be ignored.
				// Finally, it's not possible to have an alias type denoting a type
				// parameter declared by the current function and use it in the same
				// function signature; hence we don't need to Unalias before the
				// .(*TypeParam) type assertion above.
				untyped = append(untyped, i)
			}
		}
	}

	if traceInference {
		inferred := u.inferred(tparams)
		u.tracef("=> %s ➞ %s\n", tparams, inferred)
	}

	// --- 2 ---
	// use information from type parameter constraints

	if traceInference {
		u.tracef("== type parameters: %s", tparams)
	}

	// Unify type parameters with their constraints as long
	// as progress is being made.
	//
	// This is an O(n^2) algorithm where n is the number of
	// type parameters: if there is progress, at least one
	// type argument is inferred per iteration, and we have
	// a doubly nested loop.
	//
	// In practice this is not a problem because the number
	// of type parameters tends to be very small (< 5 or so).
	// (It should be possible for unification to efficiently
	// signal newly inferred type arguments; then the loops
	// here could handle the respective type parameters only,
	// but that will come at a cost of extra complexity which
	// may not be worth it.)
	for i := 0; ; i++ {
		nn := u.unknowns()
		if traceInference {
			if i > 0 {
				fmt.Println()
			}
			u.tracef("-- iteration %d", i)
		}

		for _, tpar := range tparams {
			tx := u.at(tpar)
			core, single := coreTerm(tpar)
			if traceInference {
				u.tracef("-- type parameter %s = %s: core(%s) = %s, single = %v", tpar, tx, tpar, core, single)
			}

			// If the type parameter's constraint has a core term (i.e., a core type with tilde information)
			// try to unify the type parameter with that core type.
			if core != nil {
				// A type parameter can be unified with its constraint's core type in two cases.
				switch {
				case tx != nil:
					if traceInference {
						u.tracef("-> unify type parameter %s (type %s) with constraint core type %s", tpar, tx, core.typ)
					}
					// The corresponding type argument tx is known. There are 2 cases:
					// 1) If the core type has a tilde, per spec requirement for tilde
					//    elements, the core type is an underlying (literal) type.
					//    And because of the tilde, the underlying type of tx must match
					//    against the core type.
					//    But because unify automatically matches a defined type against
					//    an underlying literal type, we can simply unify tx with the
					//    core type.
					// 2) If the core type doesn't have a tilde, we also must unify tx
					//    with the core type.
					if !u.unify(tx, core.typ, 0) {
						// TODO(gri) Type parameters that appear in the constraint and
						//           for which we have type arguments inferred should
						//           use those type arguments for a better error message.
						err.addf(pos, "%s (type %s) does not satisfy %s", tpar, tx, tpar.Constraint())
						return nil
					}
				case single && !core.tilde:
					if traceInference {
						u.tracef("-> set type parameter %s to constraint's common underlying type %s", tpar, core.typ)
					}
					// The corresponding type argument tx is unknown and the core term
					// describes a single specific type and no tilde.
					// In this case the type argument must be that single type; set it.
					u.set(tpar, core.typ)
				}
			}

			// Independent of whether there is a core term, if the type argument tx is known
			// it must implement the methods of the type constraint, possibly after unification
			// of the relevant method signatures, otherwise tx cannot satisfy the constraint.
			// This unification step may provide additional type arguments.
			//
			// Note: The type argument tx may be known but contain references to other type
			// parameters (i.e., tx may still be parameterized).
			// In this case the methods of tx don't correctly reflect the final method set
			// and we may get a missing method error below. Skip this step in this case.
			//
			// TODO(gri) We should be able continue even with a parameterized tx if we add
			// a simplify step beforehand (see below). This will require factoring out the
			// simplify phase so we can call it from here.
			if tx != nil && !isParameterized(tparams, tx) {
				if traceInference {
					u.tracef("-> unify type parameter %s (type %s) methods with constraint methods", tpar, tx)
				}
				// TODO(gri) Now that unification handles interfaces, this code can
				//           be reduced to calling u.unify(tx, tpar.iface(), assign)
				//           (which will compare signatures exactly as we do below).
				//           We leave it as is for now because missingMethod provides
				//           a failure cause which allows for a better error message.
				//           Eventually, unify should return an error with cause.
				var cause string
				constraint := tpar.iface()
				if !check.hasAllMethods(tx, constraint, true, func(x, y Type) bool { return u.unify(x, y, exact) }, &cause) {
					// TODO(gri) better error message (see TODO above)
					err.addf(pos, "%s (type %s) does not satisfy %s %s", tpar, tx, tpar.Constraint(), cause)
					return nil
				}
			}
		}

		if u.unknowns() == nn {
			break // no progress
		}
	}

	if traceInference {
		inferred := u.inferred(tparams)
		u.tracef("=> %s ➞ %s\n", tparams, inferred)
	}

	// --- 3 ---
	// use information from untyped constants

	if traceInference {
		u.tracef("== untyped arguments: %v", untyped)
	}

	// Some generic parameters with untyped arguments may have been given a type by now.
	// Collect all remaining parameters that don't have a type yet and determine the
	// maximum untyped type for each of those parameters, if possible.
	var maxUntyped map[*TypeParam]Type // lazily allocated (we may not need it)
	for _, index := range untyped {
		tpar := params.At(index).typ.(*TypeParam) // is type parameter (no alias) by construction of untyped
		if u.at(tpar) == nil {
			arg := args[index] // arg corresponding to tpar
			if maxUntyped == nil {
				maxUntyped = make(map[*TypeParam]Type)
			}
			max := maxUntyped[tpar]
			if max == nil {
				max = arg.typ()
			} else {
				m := maxType(max, arg.typ())
				if m == nil {
					err.addf(arg, "mismatched types %s and %s (cannot infer %s)", max, arg.typ(), tpar)
					return nil
				}
				max = m
			}
			maxUntyped[tpar] = max
		}
	}
	// maxUntyped contains the maximum untyped type for each type parameter
	// which doesn't have a type yet. Set the respective default types.
	for tpar, typ := range maxUntyped {
		d := Default(typ)
		assert(isTyped(d))
		u.set(tpar, d)
	}

	// --- simplify ---

	// u.inferred(tparams) now contains the incoming type arguments plus any additional type
	// arguments which were inferred. The inferred non-nil entries may still contain
	// references to other type parameters found in constraints.
	// For instance, for [A any, B interface{ []C }, C interface{ *A }], if A == int
	// was given, unification produced the type list [int, []C, *A]. We eliminate the
	// remaining type parameters by substituting the type parameters in this type list
	// until nothing changes anymore.
	inferred = u.inferred(tparams)
	if debug {
		for i, targ := range targs {
			assert(targ == nil || inferred[i] == targ)
		}
	}

	// The data structure of each (provided or inferred) type represents a graph, where
	// each node corresponds to a type and each (directed) vertex points to a component
	// type. The substitution process described above repeatedly replaces type parameter
	// nodes in these graphs with the graphs of the types the type parameters stand for,
	// which creates a new (possibly bigger) graph for each type.
	// The substitution process will not stop if the replacement graph for a type parameter
	// also contains that type parameter.
	// For instance, for [A interface{ *A }], without any type argument provided for A,
	// unification produces the type list [*A]. Substituting A in *A with the value for
	// A will lead to infinite expansion by producing [**A], [****A], [********A], etc.,
	// because the graph A -> *A has a cycle through A.
	// Generally, cycles may occur across multiple type parameters and inferred types
	// (for instance, consider [P interface{ *Q }, Q interface{ func(P) }]).
	// We eliminate cycles by walking the graphs for all type parameters. If a cycle
	// through a type parameter is detected, killCycles nils out the respective type
	// (in the inferred list) which kills the cycle, and marks the corresponding type
	// parameter as not inferred.
	//
	// TODO(gri) If useful, we could report the respective cycle as an error. We don't
	//           do this now because type inference will fail anyway, and furthermore,
	//           constraints with cycles of this kind cannot currently be satisfied by
	//           any user-supplied type. But should that change, reporting an error
	//           would be wrong.
	killCycles(tparams, inferred)

	// dirty tracks the indices of all types that may still contain type parameters.
	// We know that nil type entries and entries corresponding to provided (non-nil)
	// type arguments are clean, so exclude them from the start.
	var dirty []int
	for i, typ := range inferred {
		if typ != nil && (i >= len(targs) || targs[i] == nil) {
			dirty = append(dirty, i)
		}
	}

	for len(dirty) > 0 {
		if traceInference {
			u.tracef("-- simplify %s ➞ %s", tparams, inferred)
		}
		// TODO(gri) Instead of creating a new substMap for each iteration,
		// provide an update operation for substMaps and only change when
		// needed. Optimization.
		smap := makeSubstMap(tparams, inferred)
		n := 0
		for _, index := range dirty {
			t0 := inferred[index]
			if t1 := check.subst(nopos, t0, smap, nil, check.context()); t1 != t0 {
				// t0 was simplified to t1.
				// If t0 was a generic function, but the simplified signature t1 does
				// not contain any type parameters anymore, the function is not generic
				// anymore. Remove its type parameters. (go.dev/issue/59953)
				// Note that if t0 was a signature, t1 must be a signature, and t1
				// can only be a generic signature if it originated from a generic
				// function argument. Those signatures are never defined types and
				// thus there is no need to call Underlying below.
				// TODO(gri) Consider doing this in Checker.subst.
				//           Then this would fall out automatically here and also
				//           in instantiation (where we also explicitly nil out
				//           type parameters).
				if sig, _ := t1.(*Signature); sig != nil && sig.TypeParams().Len() > 0 && !isParameterized(tparams, sig) {
					sig.tparams = nil
				}
				inferred[index] = t1
				dirty[n] = index
				n++
			}
		}
		dirty = dirty[:n]
	}

	// Once nothing changes anymore, we may still have type parameters left;
	// e.g., a constraint with core type *P may match a type parameter Q but
	// we don't have any type arguments to fill in for *P or Q (go.dev/issue/45548).
	// Don't let such inferences escape; instead treat them as unresolved.
	for i, typ := range inferred {
		if typ == nil || isParameterized(tparams, typ) {
			obj := tparams[i].obj
			err.addf(pos, "cannot infer %s (declared at %v)", obj.name, obj.pos)
			return nil
		}
	}

	return
}

// renameTParams renames the type parameters in the given type such that each type
// parameter is given a new identity. renameTParams returns the new type parameters
// and updated type. If the result type is unchanged from the argument type, none
// of the type parameters in tparams occurred in the type.
// If typ is a generic function, type parameters held with typ are not changed and
// must be updated separately if desired.
// The positions is only used for debug traces.
func (check *Checker) renameTParams(pos syntax.Pos, tparams []*TypeParam, typ Type) ([]*TypeParam, Type) {
	// For the purpose of type inference we must differentiate type parameters
	// occurring in explicit type or value function arguments from the type
	// parameters we are solving for via unification because they may be the
	// same in self-recursive calls:
	//
	//   func f[P constraint](x P) {
	//           f(x)
	//   }
	//
	// In this example, without type parameter renaming, the P used in the
	// instantiation f[P] has the same pointer identity as the P we are trying
	// to solve for through type inference. This causes problems for type
	// unification. Because any such self-recursive call is equivalent to
	// a mutually recursive call, type parameter renaming can be used to
	// create separate, disentangled type parameters. The above example
	// can be rewritten into the following equivalent code:
	//
	//   func f[P constraint](x P) {
	//           f2(x)
	//   }
	//
	//   func f2[P2 constraint](x P2) {
	//           f(x)
	//   }
	//
	// Type parameter renaming turns the first example into the second
	// example by renaming the type parameter P into P2.
	if len(tparams) == 0 {
		return nil, typ // nothing to do
	}

	tparams2 := make([]*TypeParam, len(tparams))
	for i, tparam := range tparams {
		tname := NewTypeName(tparam.Obj().Pos(), tparam.Obj().Pkg(), tparam.Obj().Name(), nil)
		tparams2[i] = NewTypeParam(tname, nil)
		tparams2[i].index = tparam.index // == i
	}

	renameMap := makeRenameMap(tparams, tparams2)
	for i, tparam := range tparams {
		tparams2[i].bound = check.subst(pos, tparam.bound, renameMap, nil, check.context())
	}

	return tparams2, check.subst(pos, typ, renameMap, nil, check.context())
}

// typeParamsString produces a string containing all the type parameter names
// in list suitable for human consumption.
func typeParamsString(list []*TypeParam) string {
	// common cases
	n := len(list)
	switch n {
	case 0:
		return ""
	case 1:
		return list[0].obj.name
	case 2:
		return list[0].obj.name + " and " + list[1].obj.name
	}

	// general case (n > 2)
	var buf strings.Builder
	for i, tname := range list[:n-1] {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(tname.obj.name)
	}
	buf.WriteString(", and ")
	buf.WriteString(list[n-1].obj.name)
	return buf.String()
}

// isParameterized reports whether typ contains any of the type parameters of tparams.
// If typ is a generic function, isParameterized ignores the type parameter declarations;
// it only considers the signature proper (incoming and result parameters).
func isParameterized(tparams []*TypeParam, typ Type) bool {
	w := tpWalker{
		tparams: tparams,
		seen:    make(map[Type]bool),
	}
	return w.isParameterized(typ)
}

type tpWalker struct {
	tparams []*TypeParam
	seen    map[Type]bool
}

func (w *tpWalker) isParameterized(typ Type) (res bool) {
	// detect cycles
	if x, ok := w.seen[typ]; ok {
		return x
	}
	w.seen[typ] = false
	defer func() {
		w.seen[typ] = res
	}()

	switch t := typ.(type) {
	case *Basic:
		// nothing to do

	case *Alias:
		return w.isParameterized(Unalias(t))

	case *Array:
		return w.isParameterized(t.elem)

	case *Slice:
		return w.isParameterized(t.elem)

	case *Struct:
		return w.varList(t.fields)

	case *Pointer:
		return w.isParameterized(t.base)

	case *Tuple:
		// This case does not occur from within isParameterized
		// because tuples only appear in signatures where they
		// are handled explicitly. But isParameterized is also
		// called by Checker.callExpr with a function result tuple
		// if instantiation failed (go.dev/issue/59890).
		return t != nil && w.varList(t.vars)

	case *Signature:
		// t.tparams may not be nil if we are looking at a signature
		// of a generic function type (or an interface method) that is
		// part of the type we're testing. We don't care about these type
		// parameters.
		// Similarly, the receiver of a method may declare (rather than
		// use) type parameters, we don't care about those either.
		// Thus, we only need to look at the input and result parameters.
		return t.params != nil && w.varList(t.params.vars) || t.results != nil && w.varList(t.results.vars)

	case *Interface:
		tset := t.typeSet()
		for _, m := range tset.methods {
			if w.isParameterized(m.typ) {
				return true
			}
		}
		return tset.is(func(t *term) bool {
			return t != nil && w.isParameterized(t.typ)
		})

	case *Map:
		return w.isParameterized(t.key) || w.isParameterized(t.elem)

	case *Chan:
		return w.isParameterized(t.elem)

	case *Named:
		for _, t := range t.TypeArgs().list() {
			if w.isParameterized(t) {
				return true
			}
		}

	case *TypeParam:
		return slices.Index(w.tparams, t) >= 0

	default:
		panic(fmt.Sprintf("unexpected %T", typ))
	}

	return false
}

func (w *tpWalker) varList(list []*Var) bool {
	for _, v := range list {
		if w.isParameterized(v.typ) {
			return true
		}
	}
	return false
}

// If the type parameter has a single specific type S, coreTerm returns (S, true).
// Otherwise, if tpar has a core type T, it returns a term corresponding to that
// core type and false. In that case, if any term of tpar has a tilde, the core
// term has a tilde. In all other cases coreTerm returns (nil, false).
func coreTerm(tpar *TypeParam) (*term, bool) {
	n := 0
	var single *term // valid if n == 1
	var tilde bool
	tpar.is(func(t *term) bool {
		if t == nil {
			assert(n == 0)
			return false // no terms
		}
		n++
		single = t
		if t.tilde {
			tilde = true
		}
		return true
	})
	if n == 1 {
		if debug {
			u, _ := commonUnder(tpar, nil)
			assert(single.typ.Underlying() == u)
		}
		return single, true
	}
	if typ, _ := commonUnder(tpar, nil); typ != nil {
		// A core type is always an underlying type.
		// If any term of tpar has a tilde, we don't
		// have a precise core type and we must return
		// a tilde as well.
		return &term{tilde, typ}, false
	}
	return nil, false
}

// killCycles walks through the given type parameters and looks for cycles
// created by type parameters whose inferred types refer back to that type
// parameter, either directly or indirectly. If such a cycle is detected,
// it is killed by setting the corresponding inferred type to nil.
//
// TODO(gri) Determine if we can simply abort inference as soon as we have
// found a single cycle.
func killCycles(tparams []*TypeParam, inferred []Type) {
	w := cycleFinder{tparams, inferred, make(map[Type]bool)}
	for _, t := range tparams {
		w.typ(t) // t != nil
	}
}

type cycleFinder struct {
	tparams  []*TypeParam
	inferred []Type
	seen     map[Type]bool
}

func (w *cycleFinder) typ(typ Type) {
	typ = Unalias(typ)
	if w.seen[typ] {
		// We have seen typ before. If it is one of the type parameters
		// in w.tparams, iterative substitution will lead to infinite expansion.
		// Nil out the corresponding type which effectively kills the cycle.
		if tpar, _ := typ.(*TypeParam); tpar != nil {
			if i := slices.Index(w.tparams, tpar); i >= 0 {
				// cycle through tpar
				w.inferred[i] = nil
			}
		}
		// If we don't have one of our type parameters, the cycle is due
		// to an ordinary recursive type and we can just stop walking it.
		return
	}
	w.seen[typ] = true
	defer delete(w.seen, typ)

	switch t := typ.(type) {
	case *Basic:
		// nothing to do

	// *Alias:
	//      This case should not occur because of Unalias(typ) at the top.

	case *Array:
		w.typ(t.elem)

	case *Slice:
		w.typ(t.elem)

	case *Struct:
		w.varList(t.fields)

	case *Pointer:
		w.typ(t.base)

	// case *Tuple:
	//      This case should not occur because tuples only appear
	//      in signatures where they are handled explicitly.

	case *Signature:
		if t.params != nil {
			w.varList(t.params.vars)
		}
		if t.results != nil {
			w.varList(t.results.vars)
		}

	case *Union:
		for _, t := range t.terms {
			w.typ(t.typ)
		}

	case *Interface:
		for _, m := range t.methods {
			w.typ(m.typ)
		}
		for _, t := range t.embeddeds {
			w.typ(t)
		}

	case *Map:
		w.typ(t.key)
		w.typ(t.elem)

	case *Chan:
		w.typ(t.elem)

	case *Named:
		for _, tpar := range t.TypeArgs().list() {
			w.typ(tpar)
		}

	case *TypeParam:
		if i := slices.Index(w.tparams, t); i >= 0 && w.inferred[i] != nil {
			w.typ(w.inferred[i])
		}

	default:
		panic(fmt.Sprintf("unexpected %T", typ))
	}
}

func (w *cycleFinder) varList(list []*Var) {
	for _, v := range list {
		w.typ(v.typ)
	}
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/initorder.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmp"
	"container/heap"
	"fmt"
	. "internal/types/errors"
	"slices"
	"sort"
)

// initOrder computes the Info.InitOrder for package variables.
func (check *Checker) initOrder() {
	// An InitOrder may already have been computed if a package is
	// built from several calls to (*Checker).Files. Clear it.
	check.Info.InitOrder = check.Info.InitOrder[:0]

	// Compute the object dependency graph and initialize
	// a priority queue with the list of graph nodes.
	pq := nodeQueue(dependencyGraph(check.objMap))
	heap.Init(&pq)

	const debug = false
	if debug {
		fmt.Printf("Computing initialization order for %s\n\n", check.pkg)
		fmt.Println("Object dependency graph:")
		for obj, d := range check.objMap {
			// only print objects that may appear in the dependency graph
			if obj, _ := obj.(dependency); obj != nil {
				if len(d.deps) > 0 {
					fmt.Printf("\t%s depends on\n", obj.Name())
					for dep := range d.deps {
						fmt.Printf("\t\t%s\n", dep.Name())
					}
				} else {
					fmt.Printf("\t%s has no dependencies\n", obj.Name())
				}
			}
		}
		fmt.Println()

		fmt.Println("Transposed object dependency graph (functions eliminated):")
		for _, n := range pq {
			fmt.Printf("\t%s depends on %d nodes\n", n.obj.Name(), n.ndeps)
			for p := range n.pred {
				fmt.Printf("\t\t%s is dependent\n", p.obj.Name())
			}
		}
		fmt.Println()

		fmt.Println("Processing nodes:")
	}

	// Determine initialization order by removing the highest priority node
	// (the one with the fewest dependencies) and its edges from the graph,
	// repeatedly, until there are no nodes left.
	// In a valid Go program, those nodes always have zero dependencies (after
	// removing all incoming dependencies), otherwise there are initialization
	// cycles.
	emitted := make(map[*declInfo]bool)
	for len(pq) > 0 {
		// get the next node
		n := heap.Pop(&pq).(*graphNode)

		if debug {
			fmt.Printf("\t%s (src pos %d) depends on %d nodes now\n",
				n.obj.Name(), n.obj.order(), n.ndeps)
		}

		// if n still depends on other nodes, we have a cycle
		if n.ndeps > 0 {
			cycle := findPath(check.objMap, n.obj, n.obj, make(map[Object]bool))
			// If n.obj is not part of the cycle (e.g., n.obj->b->c->d->c),
			// cycle will be nil. Don't report anything in that case since
			// the cycle is reported when the algorithm gets to an object
			// in the cycle.
			// Furthermore, once an object in the cycle is encountered,
			// the cycle will be broken (dependency count will be reduced
			// below), and so the remaining nodes in the cycle don't trigger
			// another error (unless they are part of multiple cycles).
			if cycle != nil {
				check.reportCycle(cycle)
			}
			// Ok to continue, but the variable initialization order
			// will be incorrect at this point since it assumes no
			// cycle errors.
		}

		// reduce dependency count of all dependent nodes
		// and update priority queue
		for p := range n.pred {
			p.ndeps--
			heap.Fix(&pq, p.index)
		}

		// record the init order for variables with initializers only
		v, _ := n.obj.(*Var)
		info := check.objMap[v]
		if v == nil || !info.hasInitializer() {
			continue
		}

		// n:1 variable declarations such as: a, b = f()
		// introduce a node for each lhs variable (here: a, b);
		// but they all have the same initializer - emit only
		// one, for the first variable seen
		if emitted[info] {
			continue // initializer already emitted, if any
		}
		emitted[info] = true

		infoLhs := info.lhs // possibly nil (see declInfo.lhs field comment)
		if infoLhs == nil {
			infoLhs = []*Var{v}
		}
		init := &Initializer{infoLhs, info.init}
		check.Info.InitOrder = append(check.Info.InitOrder, init)
	}

	if debug {
		fmt.Println()
		fmt.Println("Initialization order:")
		for _, init := range check.Info.InitOrder {
			fmt.Printf("\t%s\n", init)
		}
		fmt.Println()
	}
}

// findPath returns the (reversed) list of objects []Object{to, ... from}
// such that there is a path of object dependencies from 'from' to 'to'.
// If there is no such path, the result is nil.
func findPath(objMap map[Object]*declInfo, from, to Object, seen map[Object]bool) []Object {
	if seen[from] {
		return nil
	}
	seen[from] = true

	// sort deps for deterministic result
	var deps []Object
	for d := range objMap[from].deps {
		deps = append(deps, d)
	}
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].order() < deps[j].order()
	})

	for _, d := range deps {
		if d == to {
			return []Object{d}
		}
		if P := findPath(objMap, d, to, seen); P != nil {
			return append(P, d)
		}
	}

	return nil
}

// reportCycle reports an error for the given cycle.
func (check *Checker) reportCycle(cycle []Object) {
	obj := cycle[0]

	// report a more concise error for self references
	if len(cycle) == 1 {
		check.errorf(obj, InvalidInitCycle, "initialization cycle: %s refers to itself", obj.Name())
		return
	}

	err := check.newError(InvalidInitCycle)
	err.addf(obj, "initialization cycle for %s", obj.Name())
	// "cycle[i] refers to cycle[j]" for (i,j) = (0,n-1), (n-1,n-2), ..., (1,0) for len(cycle) = n.
	for j := len(cycle) - 1; j >= 0; j-- {
		next := cycle[j]
		err.addf(obj, "%s refers to %s", obj.Name(), next.Name())
		obj = next
	}
	err.report()
}

// ----------------------------------------------------------------------------
// Object dependency graph

// A dependency is an object that may be a dependency in an initialization
// expression. Only constants, variables, and functions can be dependencies.
// Constants are here because constant expression cycles are reported during
// initialization order computation.
type dependency interface {
	Object
	isDependency()
}

// A graphNode represents a node in the object dependency graph.
// Each node p in n.pred represents an edge p->n, and each node
// s in n.succ represents an edge n->s; with a->b indicating that
// a depends on b.
type graphNode struct {
	obj        dependency // object represented by this node
	pred, succ nodeSet    // consumers and dependencies of this node (lazily initialized)
	index      int        // node index in graph slice/priority queue
	ndeps      int        // number of outstanding dependencies before this object can be initialized
}

// cost returns the cost of removing this node, which involves copying each
// predecessor to each successor (and vice-versa).
func (n *graphNode) cost() int {
	return len(n.pred) * len(n.succ)
}

type nodeSet map[*graphNode]bool

func (s *nodeSet) add(p *graphNode) {
	if *s == nil {
		*s = make(nodeSet)
	}
	(*s)[p] = true
}

// dependencyGraph computes the object dependency graph from the given objMap,
// with any function nodes removed. The resulting graph contains only constants
// and variables.
func dependencyGraph(objMap map[Object]*declInfo) []*graphNode {
	// M is the dependency (Object) -> graphNode mapping
	M := make(map[dependency]*graphNode)
	for obj := range objMap {
		// only consider nodes that may be an initialization dependency
		if obj, _ := obj.(dependency); obj != nil {
			M[obj] = &graphNode{obj: obj}
		}
	}

	// compute edges for graph M
	// (We need to include all nodes, even isolated ones, because they still need
	// to be scheduled for initialization in correct order relative to other nodes.)
	for obj, n := range M {
		// for each dependency obj -> d (= deps[i]), create graph edges n->s and s->n
		for d := range objMap[obj].deps {
			// only consider nodes that may be an initialization dependency
			if d, _ := d.(dependency); d != nil {
				d := M[d]
				n.succ.add(d)
				d.pred.add(n)
			}
		}
	}

	var G, funcG []*graphNode // separate non-functions and functions
	for _, n := range M {
		if _, ok := n.obj.(*Func); ok {
			funcG = append(funcG, n)
		} else {
			G = append(G, n)
		}
	}

	// remove function nodes and collect remaining graph nodes in G
	// (Mutually recursive functions may introduce cycles among themselves
	// which are permitted. Yet such cycles may incorrectly inflate the dependency
	// count for variables which in turn may not get scheduled for initialization
	// in correct order.)
	//
	// Note that because we recursively copy predecessors and successors
	// throughout the function graph, the cost of removing a function at
	// position X is proportional to cost * (len(funcG)-X). Therefore, we should
	// remove high-cost functions last.
	slices.SortFunc(funcG, func(a, b *graphNode) int {
		return cmp.Compare(a.cost(), b.cost())
	})
	for _, n := range funcG {
		// connect each predecessor p of n with each successor s
		// and drop the function node (don't collect it in G)
		for p := range n.pred {
			// ignore self-cycles
			if p != n {
				// Each successor s of n becomes a successor of p, and
				// each predecessor p of n becomes a predecessor of s.
				for s := range n.succ {
					// ignore self-cycles
					if s != n {
						p.succ.add(s)
						s.pred.add(p)
					}
				}
				delete(p.succ, n) // remove edge to n
			}
		}
		for s := range n.succ {
			delete(s.pred, n) // remove edge to n
		}
	}

	// fill in index and ndeps fields
	for i, n := range G {
		n.index = i
		n.ndeps = len(n.succ)
	}

	return G
}

// ----------------------------------------------------------------------------
// Priority queue

// nodeQueue implements the container/heap interface;
// a nodeQueue may be used as a priority queue.
type nodeQueue []*graphNode

func (a nodeQueue) Len() int { return len(a) }

func (a nodeQueue) Swap(i, j int) {
	x, y := a[i], a[j]
	a[i], a[j] = y, x
	x.index, y.index = j, i
}

func (a nodeQueue) Less(i, j int) bool {
	x, y := a[i], a[j]

	// Prioritize all constants before non-constants. See go.dev/issue/66575/.
	_, xConst := x.obj.(*Const)
	_, yConst := y.obj.(*Const)
	if xConst != yConst {
		return xConst
	}

	// nodes are prioritized by number of incoming dependencies (1st key)
	// and source order (2nd key)
	return x.ndeps < y.ndeps || x.ndeps == y.ndeps && x.obj.order() < y.obj.order()
}

func (a *nodeQueue) Push(x any) {
	panic("unreachable")
}

func (a *nodeQueue) Pop() any {
	n := len(*a)
	x := (*a)[n-1]
	x.index = -1 // for safety
	*a = (*a)[:n-1]
	return x
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/instantiate.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements instantiation of generic types
// through substitution of type parameters by type arguments.

package types2

import (
	"cmd/compile/internal/syntax"
	"errors"
	"fmt"
	. "internal/types/errors"
)

// A genericType implements access to its type parameters.
type genericType interface {
	Type
	TypeParams() *TypeParamList
}

// Instantiate instantiates the type orig with the given type arguments targs.
// orig must be a generic *Alias, *Named, or *Signature type. If there is no error,
// the resulting Type is an instantiated type of the same kind (*Alias, *Named
// or *Signature, respectively).
//
// Methods attached to a *Named type are also instantiated, and associated with
// a new *Func that has the same position as the original method, but nil function
// scope.
//
// If ctxt is non-nil, it may be used to de-duplicate the instance against
// previous instances with the same identity. As a special case, generic
// *Signature origin types are only considered identical if they are pointer
// equivalent, so that instantiating distinct (but possibly identical)
// signatures will yield different instances. The use of a shared context does
// not guarantee that identical instances are deduplicated in all cases.
//
// If validate is set, Instantiate verifies that the type orig is in fact generic,
// that the number of type arguments and parameters match, and that the type arguments
// satisfy their respective type constraints.
// If verification fails, the resulting error may wrap an *ArgumentError indicating
// which type argument did not satisfy its type parameter constraint, and why.
//
// If validate is not set, Instantiate does not check if orig is generic, verify the
// type argument count, or check whether the type arguments satisfy their constraints.
// Instantiate is guaranteed to not return an error, but may panic. Specifically,
// for *Signature types, Instantiate will panic immediately if the type argument
// count is incorrect; for *Named types, a panic may occur later inside the
// *Named API.
func Instantiate(ctxt *Context, orig Type, targs []Type, validate bool) (Type, error) {
	if ctxt == nil {
		ctxt = NewContext()
	}
	orig_, ok := orig.(genericType) // signature of Instantiate must not change for backward-compatibility
	if !ok {
		panic(sprintf(nil, false, "cannot instantiate non-generic %s: expected *Named, *Alias, or *Signature", orig))
	}
	if len(targs) == 0 {
		panic(sprintf(nil, false, "cannot instantiate %s: empty type argument list", orig))
	}

	if validate {
		tparams := orig_.TypeParams().list()
		if len(tparams) == 0 {
			return nil, fmt.Errorf("cannot instantiate non-generic %s: has no type parameters", orig)
		}
		if len(targs) != len(tparams) {
			return nil, fmt.Errorf("cannot instantiate %s: got %d type arguments but have %d type parameters", orig, len(targs), len(tparams))
		}
		if i, err := (*Checker)(nil).verify(nopos, tparams, targs, ctxt); err != nil {
			return nil, &ArgumentError{i, err}
		}
	}

	inst := (*Checker)(nil).instance(nopos, orig_, targs, nil, ctxt)
	return inst, nil
}

// instance instantiates the given original (generic) function or type with the
// provided type arguments and returns the resulting instance. If an identical
// instance exists already in the given contexts, it returns that instance,
// otherwise it creates a new one. If there is an error (such as wrong number
// of type arguments), the result is Typ[Invalid].
//
// If expanding is non-nil, it is the Named instance type currently being
// expanded. If ctxt is non-nil, it is the context associated with the current
// type-checking pass or call to Instantiate. At least one of expanding or ctxt
// must be non-nil.
//
// For Named types the resulting instance may be unexpanded.
//
// check may be nil (when not type-checking syntax); pos is used only if check is non-nil.
func (check *Checker) instance(pos syntax.Pos, orig genericType, targs []Type, expanding *Named, ctxt *Context) (res Type) {
	// The order of the contexts below matters: we always prefer instances in the
	// expanding instance context in order to preserve reference cycles.
	//
	// Invariant: if expanding != nil, the returned instance will be the instance
	// recorded in expanding.inst.ctxt.
	var ctxts []*Context
	if expanding != nil {
		ctxts = append(ctxts, expanding.inst.ctxt)
	}
	if ctxt != nil {
		ctxts = append(ctxts, ctxt)
	}
	assert(len(ctxts) > 0)

	// Compute all hashes; hashes may differ across contexts due to different
	// unique IDs for Named types within the hasher.
	hashes := make([]string, len(ctxts))
	for i, ctxt := range ctxts {
		hashes[i] = ctxt.instanceHash(orig, targs)
	}

	// Record the result in all contexts.
	// Prefer to re-use existing types from expanding context, if it exists, to reduce
	// the memory pinned by the Named type.
	updateContexts := func(res Type) Type {
		for i := len(ctxts) - 1; i >= 0; i-- {
			res = ctxts[i].update(hashes[i], orig, targs, res)
		}
		return res
	}

	// typ may already have been instantiated with identical type arguments. In
	// that case, re-use the existing instance.
	for i, ctxt := range ctxts {
		if inst := ctxt.lookup(hashes[i], orig, targs); inst != nil {
			return updateContexts(inst)
		}
	}

	switch orig := orig.(type) {
	case *Named:
		res = check.newNamedInstance(pos, orig, targs, expanding) // substituted lazily

	case *Alias:
		// verify type parameter count (see go.dev/issue/71198 for a test case)
		tparams := orig.TypeParams()
		if !check.validateTArgLen(pos, orig.obj.Name(), tparams.Len(), len(targs)) {
			// TODO(gri) Consider returning a valid alias instance with invalid
			//           underlying (aliased) type to match behavior of *Named
			//           types. Then this function will never return an invalid
			//           result.
			return Typ[Invalid]
		}
		if tparams.Len() == 0 {
			return orig // nothing to do (minor optimization)
		}

		res = check.newAliasInstance(pos, orig, targs, expanding, ctxt)

	case *Signature:
		assert(expanding == nil) // function instances cannot be reached from Named types
		// Note that orig may be a generic method on a generic type. In that case, orig
		// is an instantiated type. It will not have receiver type parameters, but will
		// still have ordinary type parameters.
		assert(orig.RecvTypeParams() == nil)
		assert(orig.TypeParams() != nil)

		tparams := orig.TypeParams()
		// TODO(gri) investigate if this is needed (type argument and parameter count seem to be correct here)
		if !check.validateTArgLen(pos, orig.String(), tparams.Len(), len(targs)) {
			return Typ[Invalid]
		}
		if tparams.Len() == 0 {
			return orig // nothing to do (minor optimization)
		}
		sig := check.subst(pos, orig, makeSubstMap(tparams.list(), targs), nil, ctxt).(*Signature)
		// If the signature doesn't use its type parameters, subst
		// will not make a copy. In that case, make a copy now (so
		// we can set tparams to nil w/o causing side-effects).
		if sig == orig {
			copy := *sig
			sig = &copy
		}
		// After instantiating a generic signature, it is not generic
		// anymore; we need to set tparams to nil.
		sig.tparams = nil
		res = sig

	default:
		// only types and functions can be generic
		panic(fmt.Sprintf("%v: cannot instantiate %v", pos, orig))
	}

	// Update all contexts; it's possible that we've lost a race.
	return updateContexts(res)
}

// validateTArgLen checks that the number of type arguments (got) matches the
// number of type parameters (want); if they don't match an error is reported.
// If validation fails and check is nil, validateTArgLen panics.
func (check *Checker) validateTArgLen(pos syntax.Pos, name string, want, got int) bool {
	var qual string
	switch {
	case got < want:
		qual = "not enough"
	case got > want:
		qual = "too many"
	default:
		return true
	}

	msg := check.sprintf("%s type arguments for type %s: have %d, want %d", qual, name, got, want)
	if check != nil {
		check.error(atPos(pos), WrongTypeArgCount, msg)
		return false
	}

	panic(fmt.Sprintf("%v: %s", pos, msg))
}

// check may be nil; pos is used only if check is non-nil.
func (check *Checker) verify(pos syntax.Pos, tparams []*TypeParam, targs []Type, ctxt *Context) (int, error) {
	smap := makeSubstMap(tparams, targs)
	for i, tpar := range tparams {
		// Ensure that we have a (possibly implicit) interface as type bound (go.dev/issue/51048).
		tpar.iface()
		// The type parameter bound is parameterized with the same type parameters
		// as the instantiated type; before we can use it for bounds checking we
		// need to instantiate it with the type arguments with which we instantiated
		// the parameterized type.
		bound := check.subst(pos, tpar.bound, smap, nil, ctxt)
		var cause string
		if !check.implements(targs[i], bound, true, &cause) {
			return i, errors.New(cause)
		}
	}
	return -1, nil
}

// implements checks if V implements T. The receiver may be nil if implements
// is called through an exported API call such as AssignableTo. If constraint
// is set, T is a type constraint.
//
// If the provided cause is non-nil, it may be set to an error string
// explaining why V does not implement (or satisfy, for constraints) T.
func (check *Checker) implements(V, T Type, constraint bool, cause *string) bool {
	Vu := V.Underlying()
	Tu := T.Underlying()
	if !isValid(Vu) || !isValid(Tu) {
		return true // avoid follow-on errors
	}
	if p, _ := Vu.(*Pointer); p != nil && !isValid(p.base.Underlying()) {
		return true // avoid follow-on errors (see go.dev/issue/49541 for an example)
	}

	verb := "implement"
	if constraint {
		verb = "satisfy"
	}

	Ti, _ := Tu.(*Interface)
	if Ti == nil {
		if cause != nil {
			var detail string
			if isInterfacePtr(Tu) {
				detail = check.interfacePtrError(T)
			} else {
				detail = check.sprintf("%s is not an interface", T)
			}
			*cause = check.sprintf("%s does not %s %s (%s)", V, verb, T, detail)
		}
		return false
	}

	// Every type satisfies the empty interface.
	if Ti.Empty() {
		return true
	}
	// T is not the empty interface (i.e., the type set of T is restricted)

	// An interface V with an empty type set satisfies any interface.
	// (The empty set is a subset of any set.)
	Vi, _ := Vu.(*Interface)
	if Vi != nil && Vi.typeSet().IsEmpty() {
		return true
	}
	// type set of V is not empty

	// No type with non-empty type set satisfies the empty type set.
	if Ti.typeSet().IsEmpty() {
		if cause != nil {
			*cause = check.sprintf("cannot %s %s (empty type set)", verb, T)
		}
		return false
	}

	// V must implement T's methods, if any.
	if !check.hasAllMethods(V, T, true, Identical, cause) /* !Implements(V, T) */ {
		if cause != nil {
			*cause = check.sprintf("%s does not %s %s %s", V, verb, T, *cause)
		}
		return false
	}

	// Only check comparability if we don't have a more specific error.
	checkComparability := func() bool {
		if !Ti.IsComparable() {
			return true
		}
		// If T is comparable, V must be comparable.
		// If V is strictly comparable, we're done.
		if comparableType(V, false /* strict comparability */, nil) == nil {
			return true
		}
		// For constraint satisfaction, use dynamic (spec) comparability
		// so that ordinary, non-type parameter interfaces implement comparable.
		if constraint && comparableType(V, true /* spec comparability */, nil) == nil {
			// V is comparable if we are at Go 1.20 or higher.
			if check == nil || check.allowVersion(go1_20) {
				return true
			}
			if cause != nil {
				*cause = check.sprintf("%s to %s comparable requires go1.20 or later", V, verb)
			}
			return false
		}
		if cause != nil {
			*cause = check.sprintf("%s does not %s comparable", V, verb)
		}
		return false
	}

	// V must also be in the set of types of T, if any.
	// Constraints with empty type sets were already excluded above.
	if !Ti.typeSet().hasTerms() {
		return checkComparability() // nothing to do
	}

	// If V is itself an interface, each of its possible types must be in the set
	// of T types (i.e., the V type set must be a subset of the T type set).
	// Interfaces V with empty type sets were already excluded above.
	if Vi != nil {
		if !Vi.typeSet().subsetOf(Ti.typeSet()) {
			// TODO(gri) report which type is missing
			if cause != nil {
				*cause = check.sprintf("%s does not %s %s", V, verb, T)
			}
			return false
		}
		return checkComparability()
	}

	// Otherwise, V's type must be included in the iface type set.
	var alt Type
	if Ti.typeSet().is(func(t *term) bool {
		if !t.includes(V) {
			// If V ∉ t.typ but V ∈ ~t.typ then remember this type
			// so we can suggest it as an alternative in the error
			// message.
			if alt == nil && !t.tilde && Identical(t.typ, t.typ.Underlying()) {
				tt := *t
				tt.tilde = true
				if tt.includes(V) {
					alt = t.typ
				}
			}
			return true
		}
		return false
	}) {
		if cause != nil {
			var detail string
			switch {
			case alt != nil:
				detail = check.sprintf("possibly missing ~ for %s in %s", alt, T)
			case mentions(Ti, V):
				detail = check.sprintf("%s mentions %s, but %s is not in the type set of %s", T, V, V, T)
			default:
				detail = check.sprintf("%s missing in %s", V, Ti.typeSet().terms)
			}
			*cause = check.sprintf("%s does not %s %s (%s)", V, verb, T, detail)
		}
		return false
	}

	return checkComparability()
}

// mentions reports whether type T "mentions" typ in an (embedded) element or term
// of T (whether typ is in the type set of T or not). For better error messages.
func mentions(T, typ Type) bool {
	switch T := T.(type) {
	case *Interface:
		for _, e := range T.embeddeds {
			if mentions(e, typ) {
				return true
			}
		}
	case *Union:
		for _, t := range T.terms {
			if mentions(t.typ, typ) {
				return true
			}
		}
	default:
		if Identical(T, typ) {
			return true
		}
	}
	return false
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/interface.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// ----------------------------------------------------------------------------
// API

// An Interface represents an interface type.
type Interface struct {
	check     *Checker      // for error reporting; nil once type set is computed
	methods   []*Func       // ordered list of explicitly declared methods
	embeddeds []Type        // ordered list of explicitly embedded elements
	embedPos  *[]syntax.Pos // positions of embedded elements; or nil (for error messages) - use pointer to save space
	implicit  bool          // interface is wrapper for type set literal (non-interface T, ~T, or A|B)
	complete  bool          // indicates that all fields (except for tset) are set up

	tset *_TypeSet // type set described by this interface, computed lazily
}

// typeSet returns the type set for interface t.
func (t *Interface) typeSet() *_TypeSet { return computeInterfaceTypeSet(t.check, nopos, t) }

// emptyInterface represents the empty interface
var emptyInterface = Interface{complete: true, tset: &topTypeSet}

// NewInterfaceType returns a new interface for the given methods and embedded types.
// NewInterfaceType takes ownership of the provided methods and may modify their types
// by setting missing receivers.
func NewInterfaceType(methods []*Func, embeddeds []Type) *Interface {
	if len(methods) == 0 && len(embeddeds) == 0 {
		return &emptyInterface
	}

	// set method receivers if necessary
	typ := (*Checker)(nil).newInterface()
	for _, m := range methods {
		if sig := m.typ.(*Signature); sig.recv == nil {
			sig.recv = newVar(RecvVar, m.pos, m.pkg, "", typ)
		}
	}

	// sort for API stability
	sortMethods(methods)

	typ.methods = methods
	typ.embeddeds = embeddeds
	typ.complete = true

	return typ
}

// check may be nil
func (check *Checker) newInterface() *Interface {
	typ := &Interface{check: check}
	if check != nil {
		check.needsCleanup(typ)
	}
	return typ
}

// MarkImplicit marks the interface t as implicit, meaning this interface
// corresponds to a constraint literal such as ~T or A|B without explicit
// interface embedding. MarkImplicit should be called before any concurrent use
// of implicit interfaces.
func (t *Interface) MarkImplicit() {
	t.implicit = true
}

// NumExplicitMethods returns the number of explicitly declared methods of interface t.
func (t *Interface) NumExplicitMethods() int { return len(t.methods) }

// ExplicitMethod returns the i'th explicitly declared method of interface t for 0 <= i < t.NumExplicitMethods().
// The methods are ordered by their unique Id.
func (t *Interface) ExplicitMethod(i int) *Func { return t.methods[i] }

// NumEmbeddeds returns the number of embedded types in interface t.
func (t *Interface) NumEmbeddeds() int { return len(t.embeddeds) }

// EmbeddedType returns the i'th embedded type of interface t for 0 <= i < t.NumEmbeddeds().
func (t *Interface) EmbeddedType(i int) Type { return t.embeddeds[i] }

// NumMethods returns the total number of methods of interface t.
func (t *Interface) NumMethods() int { return t.typeSet().NumMethods() }

// Method returns the i'th method of interface t for 0 <= i < t.NumMethods().
// The methods are ordered by their unique Id.
func (t *Interface) Method(i int) *Func { return t.typeSet().Method(i) }

// Empty reports whether t is the empty interface.
func (t *Interface) Empty() bool { return t.typeSet().IsAll() }

// IsComparable reports whether each type in interface t's type set is comparable.
func (t *Interface) IsComparable() bool { return t.typeSet().IsComparable(nil) }

// IsMethodSet reports whether the interface t is fully described by its method set.
func (t *Interface) IsMethodSet() bool { return t.typeSet().IsMethodSet() }

// IsImplicit reports whether the interface t is a wrapper for a type set literal.
func (t *Interface) IsImplicit() bool { return t.implicit }

func (t *Interface) Underlying() Type { return t }
func (t *Interface) String() string   { return TypeString(t, nil) }

// ----------------------------------------------------------------------------
// Implementation

func (t *Interface) cleanup() {
	t.typeSet() // any interface that escapes type checking must be safe for concurrent use
	t.check = nil
	t.embedPos = nil
}

func (check *Checker) interfaceType(ityp *Interface, iface *syntax.InterfaceType, def *TypeName) {
	addEmbedded := func(pos syntax.Pos, typ Type) {
		ityp.embeddeds = append(ityp.embeddeds, typ)
		if ityp.embedPos == nil {
			ityp.embedPos = new([]syntax.Pos)
		}
		*ityp.embedPos = append(*ityp.embedPos, pos)
	}

	for _, f := range iface.MethodList {
		if f.Name == nil {
			addEmbedded(atPos(f.Type), parseUnion(check, f.Type))
			continue
		}
		// f.Name != nil

		// We have a method with name f.Name.
		name := f.Name.Value
		if name == "_" {
			check.error(f.Name, BlankIfaceMethod, "methods must have a unique non-blank name")
			continue // ignore
		}

		typ := check.typ(f.Type)
		sig, _ := typ.(*Signature)
		if sig == nil {
			if isValid(typ) {
				check.errorf(f.Type, InvalidSyntaxTree, "%s is not a method signature", typ)
			}
			continue // ignore
		}

		// use named receiver type if available (for better error messages)
		var recvTyp Type = ityp
		if def != nil {
			if named := asNamed(def.typ); named != nil {
				recvTyp = named
			}
		}
		sig.recv = newVar(RecvVar, f.Name.Pos(), check.pkg, "", recvTyp)

		m := NewFunc(f.Name.Pos(), check.pkg, name, sig)
		check.recordDef(f.Name, m)
		ityp.methods = append(ityp.methods, m)
	}

	// All methods and embedded elements for this interface are collected;
	// i.e., this interface may be used in a type set computation.
	ityp.complete = true

	if len(ityp.methods) == 0 && len(ityp.embeddeds) == 0 {
		// empty interface
		ityp.tset = &topTypeSet
		return
	}

	// sort for API stability
	// (don't sort embeddeds: they must correspond to *embedPos entries)
	sortMethods(ityp.methods)

	// Compute type set as soon as possible to report any errors.
	// Subsequent uses of type sets will use this computed type
	// set and won't need to pass in a *Checker.
	check.later(func() {
		computeInterfaceTypeSet(check, iface.Pos(), ityp)
	}).describef(iface, "compute type set for %s", ityp)
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/labels.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
	"slices"
)

// labels checks correct label use in body.
func (check *Checker) labels(body *syntax.BlockStmt) {
	// set of all labels in this body
	all := NewScope(nil, body.Pos(), syntax.EndPos(body), "label")

	fwdJumps := check.blockBranches(all, nil, nil, body.List)

	// If there are any forward jumps left, no label was found for
	// the corresponding goto statements. Either those labels were
	// never defined, or they are inside blocks and not reachable
	// for the respective gotos.
	for _, jmp := range fwdJumps {
		var msg string
		var code Code
		name := jmp.Label.Value
		if alt := all.Lookup(name); alt != nil {
			msg = "goto %s jumps into block"
			code = JumpIntoBlock
			alt.(*Label).used = true // avoid another error
		} else {
			msg = "label %s not declared"
			code = UndeclaredLabel
		}
		check.errorf(jmp.Label, code, msg, name)
	}

	// spec: "It is illegal to define a label that is never used."
	for name, obj := range all.elems {
		obj = resolve(name, obj)
		if lbl := obj.(*Label); !lbl.used {
			check.softErrorf(lbl.pos, UnusedLabel, "label %s declared and not used", lbl.name)
		}
	}
}

// A block tracks label declarations in a block and its enclosing blocks.
type block struct {
	parent *block                         // enclosing block
	lstmt  *syntax.LabeledStmt            // labeled statement to which this block belongs, or nil
	labels map[string]*syntax.LabeledStmt // allocated lazily
}

// insert records a new label declaration for the current block.
// The label must not have been declared before in any block.
func (b *block) insert(s *syntax.LabeledStmt) {
	name := s.Label.Value
	if debug {
		assert(b.gotoTarget(name) == nil)
	}
	labels := b.labels
	if labels == nil {
		labels = make(map[string]*syntax.LabeledStmt)
		b.labels = labels
	}
	labels[name] = s
}

// gotoTarget returns the labeled statement in the current
// or an enclosing block with the given label name, or nil.
func (b *block) gotoTarget(name string) *syntax.LabeledStmt {
	for s := b; s != nil; s = s.parent {
		if t := s.labels[name]; t != nil {
			return t
		}
	}
	return nil
}

// enclosingTarget returns the innermost enclosing labeled
// statement with the given label name, or nil.
func (b *block) enclosingTarget(name string) *syntax.LabeledStmt {
	for s := b; s != nil; s = s.parent {
		if t := s.lstmt; t != nil && t.Label.Value == name {
			return t
		}
	}
	return nil
}

// blockBranches processes a block's statement list and returns the set of outgoing forward jumps.
// all is the scope of all declared labels, parent the set of labels declared in the immediately
// enclosing block, and lstmt is the labeled statement this block is associated with (or nil).
func (check *Checker) blockBranches(all *Scope, parent *block, lstmt *syntax.LabeledStmt, list []syntax.Stmt) []*syntax.BranchStmt {
	b := &block{parent, lstmt, nil}

	var (
		varDeclPos         syntax.Pos
		fwdJumps, badJumps []*syntax.BranchStmt
	)

	// All forward jumps jumping over a variable declaration are possibly
	// invalid (they may still jump out of the block and be ok).
	// recordVarDecl records them for the given position.
	recordVarDecl := func(pos syntax.Pos) {
		varDeclPos = pos
		badJumps = append(badJumps[:0], fwdJumps...) // copy fwdJumps to badJumps
	}

	jumpsOverVarDecl := func(jmp *syntax.BranchStmt) bool {
		return varDeclPos.IsKnown() && slices.Contains(badJumps, jmp)
	}

	var stmtBranches func(*syntax.LabeledStmt, syntax.Stmt)
	stmtBranches = func(lstmt *syntax.LabeledStmt, s syntax.Stmt) {
		switch s := s.(type) {
		case *syntax.DeclStmt:
			for _, d := range s.DeclList {
				if d, _ := d.(*syntax.VarDecl); d != nil {
					recordVarDecl(d.Pos())
				}
			}

		case *syntax.LabeledStmt:
			// declare non-blank label
			if name := s.Label.Value; name != "_" {
				lbl := NewLabel(s.Label.Pos(), check.pkg, name)
				if alt := all.Insert(lbl); alt != nil {
					err := check.newError(DuplicateLabel)
					err.soft = true
					err.addf(lbl.pos, "label %s already declared", name)
					err.addAltDecl(alt)
					err.report()
					// ok to continue
				} else {
					b.insert(s)
					check.recordDef(s.Label, lbl)
				}
				// resolve matching forward jumps and remove them from fwdJumps
				i := 0
				for _, jmp := range fwdJumps {
					if jmp.Label.Value == name {
						// match
						lbl.used = true
						check.recordUse(jmp.Label, lbl)
						if jumpsOverVarDecl(jmp) {
							check.softErrorf(
								jmp.Label,
								JumpOverDecl,
								"goto %s jumps over variable declaration at line %d",
								name,
								varDeclPos.Line(),
							)
							// ok to continue
						}
					} else {
						// no match - record new forward jump
						fwdJumps[i] = jmp
						i++
					}
				}
				fwdJumps = fwdJumps[:i]
				lstmt = s
			}
			stmtBranches(lstmt, s.Stmt)

		case *syntax.BranchStmt:
			if s.Label == nil {
				return // checked in 1st pass (check.stmt)
			}

			// determine and validate target
			name := s.Label.Value
			switch s.Tok {
			case syntax.Break:
				// spec: "If there is a label, it must be that of an enclosing
				// "for", "switch", or "select" statement, and that is the one
				// whose execution terminates."
				valid := false
				if t := b.enclosingTarget(name); t != nil {
					switch t.Stmt.(type) {
					case *syntax.SwitchStmt, *syntax.SelectStmt, *syntax.ForStmt:
						valid = true
					}
				}
				if !valid {
					check.errorf(s.Label, MisplacedLabel, "invalid break label %s", name)
					return
				}

			case syntax.Continue:
				// spec: "If there is a label, it must be that of an enclosing
				// "for" statement, and that is the one whose execution advances."
				valid := false
				if t := b.enclosingTarget(name); t != nil {
					switch t.Stmt.(type) {
					case *syntax.ForStmt:
						valid = true
					}
				}
				if !valid {
					check.errorf(s.Label, MisplacedLabel, "invalid continue label %s", name)
					return
				}

			case syntax.Goto:
				if b.gotoTarget(name) == nil {
					// label may be declared later - add branch to forward jumps
					fwdJumps = append(fwdJumps, s)
					return
				}

			default:
				check.errorf(s, InvalidSyntaxTree, "branch statement: %s %s", s.Tok, name)
				return
			}

			// record label use
			obj := all.Lookup(name)
			obj.(*Label).used = true
			check.recordUse(s.Label, obj)

		case *syntax.AssignStmt:
			if s.Op == syntax.Def {
				recordVarDecl(s.Pos())
			}

		case *syntax.BlockStmt:
			// Unresolved forward jumps inside the nested block
			// become forward jumps in the current block.
			fwdJumps = append(fwdJumps, check.blockBranches(all, b, lstmt, s.List)...)

		case *syntax.IfStmt:
			stmtBranches(lstmt, s.Then)
			if s.Else != nil {
				stmtBranches(lstmt, s.Else)
			}

		case *syntax.SwitchStmt:
			b := &block{b, lstmt, nil}
			for _, s := range s.Body {
				fwdJumps = append(fwdJumps, check.blockBranches(all, b, nil, s.Body)...)
			}

		case *syntax.SelectStmt:
			b := &block{b, lstmt, nil}
			for _, s := range s.Body {
				fwdJumps = append(fwdJumps, check.blockBranches(all, b, nil, s.Body)...)
			}

		case *syntax.ForStmt:
			stmtBranches(lstmt, s.Body)
		}
	}

	for _, s := range list {
		stmtBranches(nil, s)
	}

	return fwdJumps
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/literals.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of literals.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
	"strings"
)

// langCompat reports an error if the representation of a numeric
// literal is not compatible with the current language version.
func (check *Checker) langCompat(lit *syntax.BasicLit) {
	s := lit.Value
	if len(s) <= 2 || check.allowVersion(go1_13) {
		return
	}
	// len(s) > 2
	if strings.Contains(s, "_") {
		check.versionErrorf(lit, go1_13, "underscore in numeric literal")
		return
	}
	if s[0] != '0' {
		return
	}
	radix := s[1]
	if radix == 'b' || radix == 'B' {
		check.versionErrorf(lit, go1_13, "binary literal")
		return
	}
	if radix == 'o' || radix == 'O' {
		check.versionErrorf(lit, go1_13, "0o/0O-style octal literal")
		return
	}
	if lit.Kind != syntax.IntLit && (radix == 'x' || radix == 'X') {
		check.versionErrorf(lit, go1_13, "hexadecimal floating-point literal")
	}
}

func (check *Checker) basicLit(x *operand, e *syntax.BasicLit) {
	switch e.Kind {
	case syntax.IntLit, syntax.FloatLit, syntax.ImagLit:
		check.langCompat(e)
		// The max. mantissa precision for untyped numeric values
		// is 512 bits, or 4048 bits for each of the two integer
		// parts of a fraction for floating-point numbers that are
		// represented accurately in the go/constant package.
		// Constant literals that are longer than this many bits
		// are not meaningful; and excessively long constants may
		// consume a lot of space and time for a useless conversion.
		// Cap constant length with a generous upper limit that also
		// allows for separators between all digits.
		const limit = 10000
		if len(e.Value) > limit {
			check.errorf(e, InvalidConstVal, "excessively long constant: %s... (%d chars)", e.Value[:10], len(e.Value))
			x.invalidate()
			return
		}
	}
	x.setConst(e.Kind, e.Value)
	if !x.isValid() {
		// The parser already establishes syntactic correctness.
		// If we reach here it's because of number under-/overflow.
		// TODO(gri) setConst (and in turn the go/constant package)
		// should return an error describing the issue.
		check.errorf(e, InvalidConstVal, "malformed constant: %s", e.Value)
		x.invalidate()
		return
	}
	// Ensure that integer values don't overflow (go.dev/issue/54280).
	x.expr = e // make sure that check.overflow below has an error position
	check.overflow(x, opPos(x.expr))
}

func (check *Checker) funcLit(x *operand, e *syntax.FuncLit) {
	if sig, ok := check.typ(e.Type).(*Signature); ok {
		// Set the Scope's extent to the complete "func (...) {...}"
		// so that Scope.Innermost works correctly.
		sig.scope.pos = e.Pos()
		sig.scope.end = endPos(e)
		if !check.conf.IgnoreFuncBodies && e.Body != nil {
			// Anonymous functions are considered part of the
			// init expression/func declaration which contains
			// them: use existing package-level declaration info.
			decl := check.decl // capture for use in closure below
			iota := check.iota // capture for use in closure below (go.dev/issue/22345)
			// Don't type-check right away because the function may
			// be part of a type definition to which the function
			// body refers. Instead, type-check as soon as possible,
			// but before the enclosing scope contents changes (go.dev/issue/22992).
			check.later(func() {
				check.funcBody(decl, "<function literal>", sig, e.Body, iota)
			}).describef(e, "func literal")
		}
		x.mode_ = value
		x.typ_ = sig
	} else {
		check.errorf(e, InvalidSyntaxTree, "invalid function literal %v", e)
		x.invalidate()
	}
}

func (check *Checker) compositeLit(x *operand, e *syntax.CompositeLit, hint Type) {
	var typ, base Type
	var isElem bool // true if composite literal is an element of an enclosing composite literal

	switch {
	case e.Type != nil:
		// composite literal type present - use it
		// [...]T array types may only appear with composite literals.
		// Check for them here so we don't have to handle ... in general.
		if atyp, _ := e.Type.(*syntax.ArrayType); atyp != nil && isdddArray(atyp) {
			// We have an "open" [...]T array type.
			// Create a new ArrayType with unknown length (-1)
			// and finish setting it up after analyzing the literal.
			typ = &Array{len: -1, elem: check.varType(atyp.Elem)}
			base = typ
			break
		}
		typ = check.typ(e.Type)
		base = typ

	case hint != nil:
		// no composite literal type present - use hint (element type of enclosing type)
		typ = hint
		base = typ
		// *T implies &T{}
		u, _ := commonUnder(base, nil)
		if b, ok := deref(u); ok {
			base = b
		}
		isElem = true

	default:
		// TODO(gri) provide better error messages depending on context
		check.error(e, UntypedLit, "missing type in composite literal")
		// continue with invalid type so that elements are "used" (go.dev/issue/69092)
		typ = Typ[Invalid]
		base = typ
	}

	// We cannot create a literal of an incomplete type; make sure it's complete.
	if !check.isComplete(base) {
		x.invalidate()
		return
	}

	switch u, _ := commonUnder(base, nil); utyp := u.(type) {
	case *Struct:
		if len(e.ElemList) == 0 {
			break
		}
		// Convention for error messages on invalid struct literals:
		// we mention the struct type only if it clarifies the error
		// (e.g., a duplicate field error doesn't need the struct type).
		fields := utyp.fields
		if _, ok := e.ElemList[0].(*syntax.KeyValueExpr); ok {
			// all elements must have keys
			visited := make(trie[*Var])
			for _, e := range e.ElemList {
				kv, _ := e.(*syntax.KeyValueExpr)
				if kv == nil {
					check.error(e, MixedStructLit, "mixture of field:value and value elements in struct literal")
					continue
				}
				key, _ := kv.Key.(*syntax.Name)
				// do all possible checks early (before exiting due to errors)
				// so we don't drop information on the floor
				check.genericExpr(x, kv.Value, nil)
				if key == nil {
					check.errorf(kv, InvalidLitField, "invalid field name %s in struct literal", kv.Key)
					continue
				}
				obj, index, indirect := lookupFieldOrMethod(utyp, false, check.pkg, key.Value, false)
				if obj == nil {
					alt, _, _ := lookupFieldOrMethod(utyp, false, check.pkg, key.Value, true)
					msg := check.lookupError(base, key.Value, alt, true)
					check.error(kv.Key, MissingLitField, msg)
					continue
				}
				fld, _ := obj.(*Var)
				if fld == nil {
					check.errorf(kv.Key, MissingLitField, "%s is not a field", kv.Key)
					continue
				}
				if len(index) > 1 && !check.verifyVersionf(kv.Key, go1_27, "use of promoted field %s in struct literal of type %s", fieldPath(utyp, index), base) {
					continue
				}
				if indirect {
					check.errorf(kv.Key, InvalidLitField, "invalid implicit pointer indirection to reach %s", kv.Key)
					continue
				}
				check.recordUse(key, fld)
				etyp := fld.typ
				check.assignment(x, etyp, "struct literal")
				if alt, n := visited.insert(index, fld); n != 0 {
					if fld == alt {
						check.errorf(kv, DuplicateLitField, "duplicate field name %s in struct literal", fld.name)
					} else if n < len(index) {
						check.errorf(kv, DuplicateLitField, "cannot specify promoted field %s and enclosing embedded field %s", fld.name, alt.name)
					} else { // n > len(index)
						check.errorf(kv, DuplicateLitField, "cannot specify embedded field %s and enclosed promoted field %s", fld.name, alt.name)
					}
				}
			}
		} else {
			// no element must have a key
			for i, e := range e.ElemList {
				if kv, _ := e.(*syntax.KeyValueExpr); kv != nil {
					check.error(kv, MixedStructLit, "mixture of field:value and value elements in struct literal")
					continue
				}
				check.genericExpr(x, e, nil)
				if i >= len(fields) {
					check.errorf(x, InvalidStructLit, "too many values in struct literal of type %s", base)
					break // cannot continue
				}
				// i < len(fields)
				fld := fields[i]
				if !fld.Exported() && fld.pkg != check.pkg {
					check.errorf(x, UnexportedLitField, "implicit assignment to unexported field %s in struct literal of type %s", fld.name, base)
					continue
				}
				etyp := fld.typ
				check.assignment(x, etyp, "struct literal")
			}
			if len(e.ElemList) < len(fields) {
				var hint string
				for _, fld := range fields {
					if !fld.Exported() && fld.pkg != check.pkg {
						hint = " (type has unexported fields - use key:value pairs)"
						break
					}
				}
				check.errorf(inNode(e, e.Rbrace), InvalidStructLit, "too few values in struct literal of type %s%s", base, hint)
				// ok to continue
			}
		}

	case *Array:
		n := check.indexedElts(e.ElemList, utyp.elem, utyp.len)
		// If we have an array of unknown length (usually [...]T arrays, but also
		// arrays [n]T where n is invalid) set the length now that we know it and
		// record the type for the array (usually done by check.typ which is not
		// called for [...]T). We handle [...]T arrays and arrays with invalid
		// length the same here because it makes sense to "guess" the length for
		// the latter if we have a composite literal; e.g. for [n]int{1, 2, 3}
		// where n is invalid for some reason, it seems fair to assume it should
		// be 3 (see also Checked.arrayLength and go.dev/issue/27346).
		if utyp.len < 0 {
			utyp.len = n
			// e.Type is missing if we have a composite literal element
			// that is itself a composite literal with omitted type. In
			// that case there is nothing to record (there is no type in
			// the source at that point).
			if e.Type != nil {
				check.recordTypeAndValue(e.Type, typexpr, utyp, nil)
			}
		}

	case *Slice:
		check.indexedElts(e.ElemList, utyp.elem, -1)

	case *Map:
		// If the map key type is an interface (but not a type parameter),
		// the type of a constant key must be considered when checking for
		// duplicates.
		keyIsInterface := isNonTypeParamInterface(utyp.key)
		visited := make(map[any][]Type, len(e.ElemList))
		for _, e := range e.ElemList {
			kv, _ := e.(*syntax.KeyValueExpr)
			if kv == nil {
				check.error(e, MissingLitKey, "missing key in map literal")
				continue
			}
			check.genericExpr(x, kv.Key, utyp.key)
			check.assignment(x, utyp.key, "map literal")
			if !x.isValid() {
				continue
			}
			if x.mode() == constant_ {
				duplicate := false
				xkey := keyVal(x.val)
				if keyIsInterface {
					for _, vtyp := range visited[xkey] {
						if Identical(vtyp, x.typ()) {
							duplicate = true
							break
						}
					}
					visited[xkey] = append(visited[xkey], x.typ())
				} else {
					_, duplicate = visited[xkey]
					visited[xkey] = nil
				}
				if duplicate {
					check.errorf(x, DuplicateLitKey, "duplicate key %s in map literal", x.val)
					continue
				}
			}
			check.genericExpr(x, kv.Value, utyp.elem)
			check.assignment(x, utyp.elem, "map literal")
		}

	default:
		// when "using" all elements unpack KeyValueExpr
		// explicitly because check.use doesn't accept them
		for _, e := range e.ElemList {
			if kv, _ := e.(*syntax.KeyValueExpr); kv != nil {
				// Ideally, we should also "use" kv.Key but we can't know
				// if it's an externally defined struct key or not. Going
				// forward anyway can lead to other errors. Give up instead.
				e = kv.Value
			}
			check.use(e)
		}
		// if utyp is invalid, an error was reported before
		if isValid(utyp) {
			var qualifier string
			if isElem {
				qualifier = " element"
			}
			var cause string
			if utyp == nil {
				cause = " (no common underlying type)"
			}
			check.errorf(e, InvalidLit, "invalid composite literal%s type %s%s", qualifier, typ, cause)
			x.invalidate()
			return
		}
	}

	x.mode_ = value
	x.typ_ = typ
}

// indexedElts checks the elements (elts) of an array or slice composite literal
// against the literal's element type (typ), and the element indices against
// the literal length if known (length >= 0). It returns the length of the
// literal (maximum index value + 1).
func (check *Checker) indexedElts(elts []syntax.Expr, typ Type, length int64) int64 {
	visited := make(map[int64]bool, len(elts))
	var index, max int64
	for _, e := range elts {
		// determine and check index
		validIndex := false
		eval := e
		if kv, _ := e.(*syntax.KeyValueExpr); kv != nil {
			if typ, i := check.index(kv.Key, length); isValid(typ) {
				if i >= 0 {
					index = i
					validIndex = true
				} else {
					check.errorf(e, InvalidLitIndex, "index %s must be integer constant", kv.Key)
				}
			}
			eval = kv.Value
		} else if length >= 0 && index >= length {
			check.errorf(e, OversizeArrayLit, "index %d is out of bounds (>= %d)", index, length)
		} else {
			validIndex = true
		}

		// if we have a valid index, check for duplicate entries
		if validIndex {
			if visited[index] {
				check.errorf(e, DuplicateLitKey, "duplicate index %d in array or slice literal", index)
			}
			visited[index] = true
		}
		index++
		if index > max {
			max = index
		}

		// check element against composite literal element type
		var x operand
		check.genericExpr(&x, eval, typ)
		check.assignment(&x, typ, "array or slice literal")
	}
	return max
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/lookup.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements various field and method lookup functions.

package types2

import (
	"bytes"
	"strings"
)

// LookupSelection selects the field or method whose ID is Id(pkg,
// name), on a value of type T. If addressable is set, T is the type
// of an addressable variable (this matters only for method lookups).
// T must not be nil.
//
// If the selection is valid:
//
//   - [Selection.Obj] returns the field ([Var]) or method ([Func]);
//   - [Selection.Indirect] reports whether there were any pointer
//     indirections on the path to the field or method.
//   - [Selection.Index] returns the index sequence, defined below.
//
// The last index entry is the field or method index in the (possibly
// embedded) type where the entry was found, either:
//
//  1. the list of declared methods of a named type; or
//  2. the list of all methods (method set) of an interface type; or
//  3. the list of fields of a struct type.
//
// The earlier index entries are the indices of the embedded struct
// fields traversed to get to the found entry, starting at depth 0.
//
// See also [LookupFieldOrMethod], which returns the components separately.
func LookupSelection(T Type, addressable bool, pkg *Package, name string) (Selection, bool) {
	obj, index, indirect := LookupFieldOrMethod(T, addressable, pkg, name)
	var kind SelectionKind
	switch obj.(type) {
	case nil:
		return Selection{}, false
	case *Func:
		kind = MethodVal
	case *Var:
		kind = FieldVal
	default:
		panic(obj) // can't happen
	}
	return Selection{kind, T, obj, index, indirect}, true
}

// Internal use of LookupFieldOrMethod: If the obj result is a method
// associated with a concrete (non-interface) type, the method's signature
// may not be fully set up. Call Checker.objDecl(obj, nil) before accessing
// the method's type.

// LookupFieldOrMethod looks up a field or method with given package and name
// in T and returns the corresponding *Var or *Func, an index sequence, and a
// bool indicating if there were any pointer indirections on the path to the
// field or method. If addressable is set, T is the type of an addressable
// variable (only matters for method lookups). T must not be nil.
//
// The last index entry is the field or method index in the (possibly embedded)
// type where the entry was found, either:
//
//  1. the list of declared methods of a named type; or
//  2. the list of all methods (method set) of an interface type; or
//  3. the list of fields of a struct type.
//
// The earlier index entries are the indices of the embedded struct fields
// traversed to get to the found entry, starting at depth 0.
//
// If no entry is found, a nil object is returned. In this case, the returned
// index and indirect values have the following meaning:
//
//   - If index != nil, the index sequence points to an ambiguous entry
//     (the same name appeared more than once at the same embedding level).
//
//   - If indirect is set, a method with a pointer receiver type was found
//     but there was no pointer on the path from the actual receiver type to
//     the method's formal receiver base type, nor was the receiver addressable.
//
// See also [LookupSelection], which returns the result as a [Selection].
func LookupFieldOrMethod(T Type, addressable bool, pkg *Package, name string) (obj Object, index []int, indirect bool) {
	if T == nil {
		panic("LookupFieldOrMethod on nil type")
	}
	return lookupFieldOrMethod(T, addressable, pkg, name, false)
}

// lookupFieldOrMethod is like LookupFieldOrMethod but with the additional foldCase parameter
// (see Object.sameId for the meaning of foldCase).
func lookupFieldOrMethod(T Type, addressable bool, pkg *Package, name string, foldCase bool) (obj Object, index []int, indirect bool) {
	// Methods cannot be associated to a named pointer type.
	// (spec: "The type denoted by T is called the receiver base type;
	// it must not be a pointer or interface type and it must be declared
	// in the same package as the method.").
	// Thus, if we have a named pointer type, proceed with the underlying
	// pointer type but discard the result if it is a method since we would
	// not have found it for T (see also go.dev/issue/8590).
	if t := asNamed(T); t != nil {
		if p, _ := t.Underlying().(*Pointer); p != nil {
			obj, index, indirect = lookupFieldOrMethodImpl(p, false, pkg, name, foldCase)
			if _, ok := obj.(*Func); ok {
				return nil, nil, false
			}
			return
		}
	}

	obj, index, indirect = lookupFieldOrMethodImpl(T, addressable, pkg, name, foldCase)

	// If we didn't find anything and if we have a type parameter with a common underlying
	// type, see if there is a matching field (but not a method, those need to be declared
	// explicitly in the constraint). If the constraint is a named pointer type (see above),
	// we are ok here because only fields are accepted as results.
	const enableTParamFieldLookup = false // see go.dev/issue/51576
	if enableTParamFieldLookup && obj == nil && isTypeParam(T) {
		if t, _ := commonUnder(T, nil); t != nil {
			obj, index, indirect = lookupFieldOrMethodImpl(t, addressable, pkg, name, foldCase)
			if _, ok := obj.(*Var); !ok {
				obj, index, indirect = nil, nil, false // accept fields (variables) only
			}
		}
	}
	return
}

// lookupFieldOrMethodImpl is the implementation of lookupFieldOrMethod.
// Notably, in contrast to lookupFieldOrMethod, it won't find struct fields
// in base types of defined (*Named) pointer types T. For instance, given
// the declaration:
//
//	type T *struct{f int}
//
// lookupFieldOrMethodImpl won't find the field f in the defined (*Named) type T
// (methods on T are not permitted in the first place).
//
// Thus, lookupFieldOrMethodImpl should only be called by lookupFieldOrMethod
// and missingMethod (the latter doesn't care about struct fields).
//
// The resulting object may not be fully type-checked.
func lookupFieldOrMethodImpl(T Type, addressable bool, pkg *Package, name string, foldCase bool) (obj Object, index []int, indirect bool) {
	// WARNING: The code in this function is extremely subtle - do not modify casually!

	if name == "_" {
		return // blank fields/methods are never found
	}

	// Importantly, we must not call Underlying before the call to deref below (nor
	// does deref call Underlying), as doing so could incorrectly result in finding
	// methods of the pointer base type when T is a (*Named) pointer type.
	typ, isPtr := deref(T)

	// *typ where typ is an interface (incl. a type parameter) has no methods.
	if isPtr {
		if _, ok := typ.Underlying().(*Interface); ok {
			return
		}
	}

	// Start with typ as single entry at shallowest depth.
	current := []embeddedType{{typ, nil, isPtr, false}}

	// seen tracks named types that we have seen already, allocated lazily.
	// Used to avoid endless searches in case of recursive types.
	//
	// We must use a lookup on identity rather than a simple map[*Named]bool as
	// instantiated types may be identical but not equal.
	var seen instanceLookup

	// search current depth
	for len(current) > 0 {
		var next []embeddedType // embedded types found at current depth

		// look for (pkg, name) in all types at current depth
		for _, e := range current {
			typ := e.typ

			// If we have a named type, we may have associated methods.
			// Look for those first.
			if named := asNamed(typ); named != nil {
				if alt := seen.lookup(named); alt != nil {
					// We have seen this type before, at a more shallow depth
					// (note that multiples of this type at the current depth
					// were consolidated before). The type at that depth shadows
					// this same type at the current depth, so we can ignore
					// this one.
					continue
				}
				seen.add(named)

				// look for a matching attached method
				if i, m := named.lookupMethod(pkg, name, foldCase); m != nil {
					// potential match
					// caution: method may not have a proper signature yet
					index = concat(e.index, i)
					if obj != nil || e.multiples {
						return nil, index, false // collision
					}
					obj = m
					indirect = e.indirect
					continue // we can't have a matching field or interface method
				}
			}

			switch t := typ.Underlying().(type) {
			case *Struct:
				// look for a matching field and collect embedded types
				for i, f := range t.fields {
					if f.sameId(pkg, name, foldCase) {
						assert(f.typ != nil)
						index = concat(e.index, i)
						if obj != nil || e.multiples {
							return nil, index, false // collision
						}
						obj = f
						indirect = e.indirect
						continue // we can't have a matching interface method
					}
					// Collect embedded struct fields for searching the next
					// lower depth, but only if we have not seen a match yet
					// (if we have a match it is either the desired field or
					// we have a name collision on the same depth; in either
					// case we don't need to look further).
					// Embedded fields are always of the form T or *T where
					// T is a type name. If e.typ appeared multiple times at
					// this depth, f.typ appears multiple times at the next
					// depth.
					if obj == nil && f.embedded {
						typ, isPtr := deref(f.typ)
						// TODO(gri) optimization: ignore types that can't
						// have fields or methods (only Named, Struct, and
						// Interface types need to be considered).
						next = append(next, embeddedType{typ, concat(e.index, i), e.indirect || isPtr, e.multiples})
					}
				}

			case *Interface:
				// look for a matching method (interface may be a type parameter)
				if i, m := t.typeSet().LookupMethod(pkg, name, foldCase); m != nil {
					assert(m.typ != nil)
					index = concat(e.index, i)
					if obj != nil || e.multiples {
						return nil, index, false // collision
					}
					obj = m
					indirect = e.indirect
				}
			}
		}

		if obj != nil {
			// found a potential match
			// spec: "A method call x.m() is valid if the method set of (the type of) x
			//        contains m and the argument list can be assigned to the parameter
			//        list of m. If x is addressable and &x's method set contains m, x.m()
			//        is shorthand for (&x).m()".
			if f, _ := obj.(*Func); f != nil {
				// determine if method has a pointer receiver
				if f.hasPtrRecv() && !indirect && !addressable {
					return nil, nil, true // pointer/addressable receiver required
				}
			}
			return
		}

		current = consolidateMultiples(next)
	}

	return nil, nil, false // not found
}

// embeddedType represents an embedded type
type embeddedType struct {
	typ       Type
	index     []int // embedded field indices, starting with index at depth 0
	indirect  bool  // if set, there was a pointer indirection on the path to this field
	multiples bool  // if set, typ appears multiple times at this depth
}

// consolidateMultiples collects multiple list entries with the same type
// into a single entry marked as containing multiples. The result is the
// consolidated list.
func consolidateMultiples(list []embeddedType) []embeddedType {
	if len(list) <= 1 {
		return list // at most one entry - nothing to do
	}

	n := 0                     // number of entries w/ unique type
	prev := make(map[Type]int) // index at which type was previously seen
	for _, e := range list {
		if i, found := lookupType(prev, e.typ); found {
			list[i].multiples = true
			// ignore this entry
		} else {
			prev[e.typ] = n
			list[n] = e
			n++
		}
	}
	return list[:n]
}

func lookupType(m map[Type]int, typ Type) (int, bool) {
	// fast path: maybe the types are equal
	if i, found := m[typ]; found {
		return i, true
	}

	for t, i := range m {
		if Identical(t, typ) {
			return i, true
		}
	}

	return 0, false
}

type instanceLookup struct {
	// buf is used to avoid allocating the map m in the common case of a small
	// number of instances.
	buf [3]*Named
	m   map[*Named][]*Named
}

func (l *instanceLookup) lookup(inst *Named) *Named {
	for _, t := range l.buf {
		if t != nil && Identical(inst, t) {
			return t
		}
	}
	for _, t := range l.m[inst.Origin()] {
		if Identical(inst, t) {
			return t
		}
	}
	return nil
}

func (l *instanceLookup) add(inst *Named) {
	for i, t := range l.buf {
		if t == nil {
			l.buf[i] = inst
			return
		}
	}
	if l.m == nil {
		l.m = make(map[*Named][]*Named)
	}
	insts := l.m[inst.Origin()]
	l.m[inst.Origin()] = append(insts, inst)
}

// MissingMethod returns (nil, false) if V implements T, otherwise it
// returns a missing method required by T and whether it is missing or
// just has the wrong type: either a pointer receiver or wrong signature.
//
// For non-interface types V, or if static is set, V implements T if all
// methods of T are present in V. Otherwise (V is an interface and static
// is not set), MissingMethod only checks that methods of T which are also
// present in V have matching types (e.g., for a type assertion x.(T) where
// x is of interface type V).
func MissingMethod(V Type, T *Interface, static bool) (method *Func, wrongType bool) {
	return (*Checker)(nil).missingMethod(V, T, static, Identical, nil)
}

// missingMethod is like MissingMethod but accepts a *Checker as receiver,
// a comparator equivalent for type comparison, and a *string for error causes.
// The receiver may be nil if missingMethod is invoked through an exported
// API call (such as MissingMethod), i.e., when all methods have been type-
// checked.
// The underlying type of T must be an interface; T (rather than its under-
// lying type) is used for better error messages (reported through *cause).
// The comparator is used to compare signatures.
// If a method is missing and cause is not nil, *cause describes the error.
func (check *Checker) missingMethod(V, T Type, static bool, equivalent func(x, y Type) bool, cause *string) (method *Func, wrongType bool) {
	methods := T.Underlying().(*Interface).typeSet().methods // T must be an interface
	if len(methods) == 0 {
		return nil, false
	}

	const (
		ok = iota
		notFound
		wrongName
		unexported
		wrongSig
		ambigSel
		ptrRecv
		field
		nointerface
	)

	state := ok
	var m *Func // method on T we're trying to implement
	var f *Func // method on V, if found (state is one of ok, wrongName, wrongSig)

	if u, _ := V.Underlying().(*Interface); u != nil {
		tset := u.typeSet()
		for _, m = range methods {
			_, f = tset.LookupMethod(m.pkg, m.name, false)

			if f == nil {
				if !static {
					continue
				}
				state = notFound
				break
			}

			if !equivalent(f.typ, m.typ) {
				state = wrongSig
				break
			}
		}
	} else {
		for _, m = range methods {
			obj, index, indirect := lookupFieldOrMethodImpl(V, false, m.pkg, m.name, false)

			// check if m is ambiguous, on *V, or on V with case-folding
			if obj == nil {
				switch {
				case index != nil:
					state = ambigSel
				case indirect:
					state = ptrRecv
				default:
					state = notFound
					obj, _, _ = lookupFieldOrMethodImpl(V, false, m.pkg, m.name, true /* fold case */)
					f, _ = obj.(*Func)
					if f != nil {
						state = wrongName
						if f.name == m.name {
							// If the names are equal, f must be unexported
							// (otherwise the package wouldn't matter).
							state = unexported
						}
					}
				}
				break
			}

			// we must have a method (not a struct field)
			f, _ = obj.(*Func)
			if f == nil {
				state = field
				break
			}

			// methods may not have a fully set up signature yet
			if check != nil {
				check.objDecl(f)
			}

			if f.nointerface {
				state = nointerface
				break
			}

			if !equivalent(f.typ, m.typ) {
				state = wrongSig
				break
			}
		}
	}

	if state == ok {
		return nil, false
	}

	if cause != nil {
		if f != nil {
			// This method may be formatted in funcString below, so must have a fully
			// set up signature.
			if check != nil {
				check.objDecl(f)
			}
		}
		switch state {
		case notFound:
			switch {
			case isInterfacePtr(V):
				*cause = "(" + check.interfacePtrError(V) + ")"
			case isInterfacePtr(T):
				*cause = "(" + check.interfacePtrError(T) + ")"
			default:
				*cause = check.sprintf("(missing method %s)", m.Name())
			}
		case wrongName:
			fs, ms := check.funcString(f, false), check.funcString(m, false)
			*cause = check.sprintf("(missing method %s)\n\t\thave %s\n\t\twant %s", m.Name(), fs, ms)
		case unexported:
			*cause = check.sprintf("(unexported method %s)", m.Name())
		case wrongSig:
			fs, ms := check.funcString(f, false), check.funcString(m, false)
			if fs == ms {
				// Don't report "want Foo, have Foo".
				// Add package information to disambiguate (go.dev/issue/54258).
				fs, ms = check.funcString(f, true), check.funcString(m, true)
			}
			if fs == ms {
				// We still have "want Foo, have Foo".
				// This is most likely due to different type parameters with
				// the same name appearing in the instantiated signatures
				// (go.dev/issue/61685).
				// Rather than reporting this misleading error cause, for now
				// just point out that the method signature is incorrect.
				// TODO(gri) should find a good way to report the root cause
				*cause = check.sprintf("(wrong type for method %s)", m.Name())
				break
			}
			*cause = check.sprintf("(wrong type for method %s)\n\t\thave %s\n\t\twant %s", m.Name(), fs, ms)
		case ambigSel:
			*cause = check.sprintf("(ambiguous selector %s.%s)", V, m.Name())
		case ptrRecv:
			*cause = check.sprintf("(method %s has pointer receiver)", m.Name())
		case field:
			*cause = check.sprintf("(%s.%s is a field, not a method)", V, m.Name())
		case nointerface:
			*cause = check.sprintf("(%s method is marked 'nointerface')", m.Name())
		default:
			panic("unreachable")
		}
	}

	return m, state == wrongSig || state == ptrRecv
}

// hasAllMethods is similar to checkMissingMethod but instead reports whether all methods are present.
// If V is not a valid type, or if it is a struct containing embedded fields with invalid types, the
// result is true because it is not possible to say with certainty whether a method is missing or not
// (an embedded field may have the method in question).
// If the result is false and cause is not nil, *cause describes the error.
// Use hasAllMethods to avoid follow-on errors due to incorrect types.
func (check *Checker) hasAllMethods(V, T Type, static bool, equivalent func(x, y Type) bool, cause *string) bool {
	if !isValid(V) {
		return true // we don't know anything about V, assume it implements T
	}
	m, _ := check.missingMethod(V, T, static, equivalent, cause)
	return m == nil || hasInvalidEmbeddedFields(V, nil)
}

// hasInvalidEmbeddedFields reports whether T is a struct (or a pointer to a struct) that contains
// (directly or indirectly) embedded fields with invalid types.
func hasInvalidEmbeddedFields(T Type, seen map[*Struct]bool) bool {
	if S, _ := derefStructPtr(T).Underlying().(*Struct); S != nil && !seen[S] {
		if seen == nil {
			seen = make(map[*Struct]bool)
		}
		seen[S] = true
		for _, f := range S.fields {
			if f.embedded && (!isValid(f.typ) || hasInvalidEmbeddedFields(f.typ, seen)) {
				return true
			}
		}
	}
	return false
}

func isInterfacePtr(T Type) bool {
	p, _ := T.Underlying().(*Pointer)
	return p != nil && IsInterface(p.base)
}

// check may be nil.
func (check *Checker) interfacePtrError(T Type) string {
	assert(isInterfacePtr(T))
	if p, _ := T.Underlying().(*Pointer); isTypeParam(p.base) {
		return check.sprintf("type %s is pointer to type parameter, not type parameter", T)
	}
	return check.sprintf("type %s is pointer to interface, not interface", T)
}

// funcString returns a string of the form name + signature for f.
// check may be nil.
func (check *Checker) funcString(f *Func, pkgInfo bool) string {
	buf := bytes.NewBufferString(f.name)
	var qf Qualifier
	if check != nil && !pkgInfo {
		qf = check.qualifier
	}
	w := newTypeWriter(buf, qf)
	w.pkgInfo = pkgInfo
	w.paramNames = false
	w.signature(f.typ.(*Signature))
	return buf.String()
}

// assertableTo reports whether a value of type V can be asserted to have type T.
// The receiver may be nil if assertableTo is invoked through an exported API call
// (such as AssertableTo), i.e., when all methods have been type-checked.
// The underlying type of V must be an interface.
// If the result is false and cause is not nil, *cause describes the error.
// TODO(gri) replace calls to this function with calls to newAssertableTo.
func (check *Checker) assertableTo(V, T Type, cause *string) bool {
	// no static check is required if T is an interface
	// spec: "If T is an interface type, x.(T) asserts that the
	//        dynamic type of x implements the interface T."
	if IsInterface(T) {
		return true
	}
	// TODO(gri) fix this for generalized interfaces
	return check.hasAllMethods(T, V, false, Identical, cause)
}

// newAssertableTo reports whether a value of type V can be asserted to have type T.
// It also implements behavior for interfaces that currently are only permitted
// in constraint position (we have not yet defined that behavior in the spec).
// The underlying type of V must be an interface.
// If the result is false and cause is not nil, *cause is set to the error cause.
func (check *Checker) newAssertableTo(V, T Type, cause *string) bool {
	// no static check is required if T is an interface
	// spec: "If T is an interface type, x.(T) asserts that the
	//        dynamic type of x implements the interface T."
	if IsInterface(T) {
		return true
	}
	return check.implements(T, V, false, cause)
}

// deref dereferences typ if it is a *Pointer (but not a *Named type
// with an underlying pointer type!) and returns its base and true.
// Otherwise it returns (typ, false).
func deref(typ Type) (Type, bool) {
	if p, _ := Unalias(typ).(*Pointer); p != nil {
		// p.base should never be nil, but be conservative
		if p.base == nil {
			if debug {
				panic("pointer with nil base type (possibly due to an invalid cyclic declaration)")
			}
			return Typ[Invalid], true
		}
		return p.base, true
	}
	return typ, false
}

// derefStructPtr dereferences typ if it is a (named or unnamed) pointer to a
// (named or unnamed) struct and returns its base. Otherwise it returns typ.
func derefStructPtr(typ Type) Type {
	if p, _ := typ.Underlying().(*Pointer); p != nil {
		if _, ok := p.base.Underlying().(*Struct); ok {
			return p.base
		}
	}
	return typ
}

// concat returns the result of concatenating list and i.
// The result does not share its underlying array with list.
func concat(list []int, i int) []int {
	var t []int
	t = append(t, list...)
	return append(t, i)
}

// methodIndex returns the index of and method with matching package and name, or (-1, nil).
// See Object.sameId for the meaning of foldCase.
func methodIndex(methods []*Func, pkg *Package, name string, foldCase bool) (int, *Func) {
	if name != "_" {
		for i, m := range methods {
			if m.sameId(pkg, name, foldCase) {
				return i, m
			}
		}
	}
	return -1, nil
}

// Given a (possibly pointer to a) struct type and field index sequence,
// fieldPath returns the dot-separated concatenated field names for the
// given index sequence (e.g. "a.b.c").
// Use for error reporting etc. where speed is not important.
func fieldPath(typ Type, index []int) string {
	var names []string
	for _, i := range index {
		u, ok := derefStructPtr(typ).Underlying().(*Struct)
		if !ok {
			// should not happen if index is valid for typ
			break
		}
		fld := u.Field(i)
		names = append(names, fld.name)
		typ = fld.typ
	}
	return strings.Join(names, ".")
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/map.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

// A Map represents a map type.
type Map struct {
	key, elem Type
}

// NewMap returns a new map for the given key and element types.
func NewMap(key, elem Type) *Map {
	return &Map{key: key, elem: elem}
}

// Key returns the key type of map m.
func (m *Map) Key() Type { return m.key }

// Elem returns the element type of map m.
func (m *Map) Elem() Type { return m.elem }

func (t *Map) Underlying() Type { return t }
func (t *Map) String() string   { return TypeString(t, nil) }

```

// === FILE: references!/go/src/cmd/compile/internal/types2/mono.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// This file implements a check to validate that a Go package doesn't
// have unbounded recursive instantiation, which is not compatible
// with compilers using static instantiation (such as
// monomorphization).
//
// It implements a sort of "type flow" analysis by detecting which
// type parameters are instantiated with other type parameters (or
// types derived thereof). A package cannot be statically instantiated
// if the graph has any cycles involving at least one derived type.
//
// Concretely, we construct a directed, weighted graph. Vertices are
// used to represent type parameters as well as some defined
// types. Edges are used to represent how types depend on each other:
//
// * Everywhere a type-parameterized function or type is instantiated,
//   we add edges to each type parameter from the vertices (if any)
//   representing each type parameter or defined type referenced by
//   the type argument. If the type argument is just the referenced
//   type itself, then the edge has weight 0, otherwise 1.
//
// * For every defined type declared within a type-parameterized
//   function or method, we add an edge of weight 1 to the defined
//   type from each ambient type parameter.
//
// For example, given:
//
//	func f[A, B any]() {
//		type T int
//		f[T, map[A]B]()
//	}
//
// we construct vertices representing types A, B, and T. Because of
// declaration "type T int", we construct edges T<-A and T<-B with
// weight 1; and because of instantiation "f[T, map[A]B]" we construct
// edges A<-T with weight 0, and B<-A and B<-B with weight 1.
//
// Finally, we look for any positive-weight cycles. Zero-weight cycles
// are allowed because static instantiation will reach a fixed point.

type monoGraph struct {
	vertices []monoVertex
	edges    []monoEdge

	// canon maps method receiver type parameters to their respective
	// receiver type's type parameters.
	canon map[*TypeParam]*TypeParam

	// nameIdx maps a defined type or (canonical) type parameter to its
	// vertex index.
	nameIdx map[*TypeName]int
}

type monoVertex struct {
	weight int // weight of heaviest known path to this vertex
	pre    int // previous edge (if any) in the above path
	len    int // length of the above path

	// obj is the defined type or type parameter represented by this
	// vertex.
	obj *TypeName
}

type monoEdge struct {
	dst, src int
	weight   int

	pos syntax.Pos
	typ Type
}

func (check *Checker) monomorph() {
	// We detect unbounded instantiation cycles using a variant of
	// Bellman-Ford's algorithm. Namely, instead of always running |V|
	// iterations, we run until we either reach a fixed point or we've
	// found a path of length |V|. This allows us to terminate earlier
	// when there are no cycles, which should be the common case.

	again := true
	for again {
		again = false

		for i, edge := range check.mono.edges {
			src := &check.mono.vertices[edge.src]
			dst := &check.mono.vertices[edge.dst]

			// N.B., we're looking for the greatest weight paths, unlike
			// typical Bellman-Ford.
			w := src.weight + edge.weight
			if w <= dst.weight {
				continue
			}

			dst.pre = i
			dst.len = src.len + 1
			if dst.len == len(check.mono.vertices) {
				check.reportInstanceLoop(edge.dst)
				return
			}

			dst.weight = w
			again = true
		}
	}
}

func (check *Checker) reportInstanceLoop(v int) {
	var stack []int
	seen := make([]bool, len(check.mono.vertices))

	// We have a path that contains a cycle and ends at v, but v may
	// only be reachable from the cycle, not on the cycle itself. We
	// start by walking backwards along the path until we find a vertex
	// that appears twice.
	for !seen[v] {
		stack = append(stack, v)
		seen[v] = true
		v = check.mono.edges[check.mono.vertices[v].pre].src
	}

	// Trim any vertices we visited before visiting v the first
	// time. Since v is the first vertex we found within the cycle, any
	// vertices we visited earlier cannot be part of the cycle.
	for stack[0] != v {
		stack = stack[1:]
	}

	// TODO(mdempsky): Pivot stack so we report the cycle from the top?

	err := check.newError(InvalidInstanceCycle)
	obj0 := check.mono.vertices[v].obj
	err.addf(obj0, "instantiation cycle:")

	qf := RelativeTo(check.pkg)
	for _, v := range stack {
		edge := check.mono.edges[check.mono.vertices[v].pre]
		obj := check.mono.vertices[edge.dst].obj

		switch obj.Type().(type) {
		default:
			panic("unexpected type")
		case *Named:
			err.addf(atPos(edge.pos), "%s implicitly parameterized by %s", obj.Name(), TypeString(edge.typ, qf)) // secondary error, \t indented
		case *TypeParam:
			err.addf(atPos(edge.pos), "%s instantiated as %s", obj.Name(), TypeString(edge.typ, qf)) // secondary error, \t indented
		}
	}
	err.report()
}

// recordCanon records that tpar is the canonical type parameter
// corresponding to method type parameter mpar.
func (w *monoGraph) recordCanon(mpar, tpar *TypeParam) {
	if w.canon == nil {
		w.canon = make(map[*TypeParam]*TypeParam)
	}
	w.canon[mpar] = tpar
}

// recordInstance records that the given type parameters were
// instantiated with the corresponding type arguments.
func (w *monoGraph) recordInstance(pkg *Package, pos syntax.Pos, tparams []*TypeParam, targs []Type, xlist []syntax.Expr) {
	for i, tpar := range tparams {
		pos := pos
		if i < len(xlist) {
			pos = startPos(xlist[i])
		}
		w.assign(pkg, pos, tpar, targs[i])
	}
}

// assign records that tpar was instantiated as targ at pos.
func (w *monoGraph) assign(pkg *Package, pos syntax.Pos, tpar *TypeParam, targ Type) {
	// Go generics do not have an analog to C++`s template-templates,
	// where a template parameter can itself be an instantiable
	// template. So any instantiation cycles must occur within a single
	// package. Accordingly, we can ignore instantiations of imported
	// type parameters.
	//
	// TODO(mdempsky): Push this check up into recordInstance? All type
	// parameters in a list will appear in the same package.
	if tpar.Obj().Pkg() != pkg {
		return
	}

	// flow adds an edge from vertex src representing that typ flows to tpar.
	flow := func(src int, typ Type) {
		weight := 1
		if typ == targ {
			weight = 0
		}

		w.addEdge(w.typeParamVertex(tpar), src, weight, pos, targ)
	}

	// Recursively walk the type argument to find any defined types or
	// type parameters.
	var do func(typ Type)
	do = func(typ Type) {
		switch typ := Unalias(typ).(type) {
		default:
			panic("unexpected type")

		case *TypeParam:
			assert(typ.Obj().Pkg() == pkg)
			flow(w.typeParamVertex(typ), typ)

		case *Named:
			if src := w.localNamedVertex(pkg, typ.Origin()); src >= 0 {
				flow(src, typ)
			}

			targs := typ.TypeArgs()
			for i := 0; i < targs.Len(); i++ {
				do(targs.At(i))
			}

		case *Array:
			do(typ.Elem())
		case *Basic:
			// ok
		case *Chan:
			do(typ.Elem())
		case *Map:
			do(typ.Key())
			do(typ.Elem())
		case *Pointer:
			do(typ.Elem())
		case *Slice:
			do(typ.Elem())

		case *Interface:
			for i := 0; i < typ.NumMethods(); i++ {
				do(typ.Method(i).Type())
			}
		case *Signature:
			tuple := func(tup *Tuple) {
				for i := 0; i < tup.Len(); i++ {
					do(tup.At(i).Type())
				}
			}
			tuple(typ.Params())
			tuple(typ.Results())
		case *Struct:
			for i := 0; i < typ.NumFields(); i++ {
				do(typ.Field(i).Type())
			}
		}
	}
	do(targ)
}

// localNamedVertex returns the index of the vertex representing
// named, or -1 if named doesn't need representation.
func (w *monoGraph) localNamedVertex(pkg *Package, named *Named) int {
	obj := named.Obj()
	if obj.Pkg() != pkg {
		return -1 // imported type
	}

	root := pkg.Scope()
	if obj.Parent() == root {
		return -1 // package scope, no ambient type parameters
	}

	if idx, ok := w.nameIdx[obj]; ok {
		return idx
	}

	idx := -1

	// Walk the type definition's scope to find any ambient type
	// parameters that it's implicitly parameterized by.
	for scope := obj.Parent(); scope != root; scope = scope.Parent() {
		for _, elem := range scope.elems {
			if elem, ok := elem.(*TypeName); ok && !elem.IsAlias() && cmpPos(elem.Pos(), obj.Pos()) < 0 {
				if tpar, ok := elem.Type().(*TypeParam); ok {
					if idx < 0 {
						idx = len(w.vertices)
						w.vertices = append(w.vertices, monoVertex{obj: obj})
					}

					w.addEdge(idx, w.typeParamVertex(tpar), 1, obj.Pos(), tpar)
				}
			}
		}
	}

	if w.nameIdx == nil {
		w.nameIdx = make(map[*TypeName]int)
	}
	w.nameIdx[obj] = idx
	return idx
}

// typeParamVertex returns the index of the vertex representing tpar.
func (w *monoGraph) typeParamVertex(tpar *TypeParam) int {
	if x, ok := w.canon[tpar]; ok {
		tpar = x
	}

	obj := tpar.Obj()

	if idx, ok := w.nameIdx[obj]; ok {
		return idx
	}

	if w.nameIdx == nil {
		w.nameIdx = make(map[*TypeName]int)
	}

	idx := len(w.vertices)
	w.vertices = append(w.vertices, monoVertex{obj: obj})
	w.nameIdx[obj] = idx
	return idx
}

func (w *monoGraph) addEdge(dst, src, weight int, pos syntax.Pos, typ Type) {
	// TODO(mdempsky): Deduplicate redundant edges?
	w.edges = append(w.edges, monoEdge{
		dst:    dst,
		src:    src,
		weight: weight,

		pos: pos,
		typ: typ,
	})
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/named.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	"strings"
	"sync"
	"sync/atomic"
)

// Type-checking Named types is subtle, because they may be recursively
// defined, and because their full details may be spread across multiple
// declarations (via methods). For this reason they are type-checked lazily,
// to avoid information being accessed before it is complete.
//
// Conceptually, it is helpful to think of named types as having two distinct
// sets of information:
//  - "LHS" information, defining their identity: Obj() and TypeArgs()
//  - "RHS" information, defining their details: TypeParams(), Underlying(),
//    and methods.
//
// In this taxonomy, LHS information is available immediately, but RHS
// information is lazy. Specifically, a named type N may be constructed in any
// of the following ways:
//  1. type-checked from the source
//  2. loaded eagerly from export data
//  3. loaded lazily from export data (when using unified IR)
//  4. instantiated from a generic type
//
// In cases 1, 3, and 4, it is possible that the underlying type or methods of
// N may not be immediately available.
//  - During type-checking, we allocate N before type-checking its underlying
//    type or methods, so that we can create recursive references.
//  - When loading from export data, we may load its methods and underlying
//    type lazily using a provided load function.
//  - After instantiating, we lazily expand the underlying type and methods
//    (note that instances may be created while still in the process of
//    type-checking the original type declaration).
//
// In cases 3 and 4 this lazy construction may also occur concurrently, due to
// concurrent use of the type checker API (after type checking or importing has
// finished). It is critical that we keep track of state, so that Named types
// are constructed exactly once and so that we do not access their details too
// soon.
//
// We achieve this by tracking state with an atomic state variable, and
// guarding potentially concurrent calculations with a mutex. See [stateMask]
// for details.
//
// GLOSSARY: Here are a few terms used in this file to describe Named types:
//  - We say that a Named type is "instantiated" if it has been constructed by
//    instantiating a generic named type with type arguments.
//  - We say that a Named type is "declared" if it corresponds to a type
//    declaration in the source. Instantiated named types correspond to a type
//    instantiation in the source, not a declaration. But their Origin type is
//    a declared type.
//  - We say that a Named type is "unpacked" if its RHS information has been
//    populated, normalizing its representation for use in type-checking
//    operations and abstracting away how it was created:
//      - For a Named type constructed from unified IR, this involves invoking
//        a lazy loader function to extract details from UIR as needed.
//      - For an instantiated Named type, this involves extracting information
//        from its origin and substituting type arguments into a "synthetic"
//        RHS; this process is called "expanding" the RHS (see below).
//  - We say that a Named type is "expanded" if it is an instantiated type and
//    type parameters in its RHS and methods have been substituted with the type
//    arguments from the instantiation. A type may be partially expanded if some
//    but not all of these details have been substituted. Similarly, we refer to
//    these individual details (RHS or method) as being "expanded".
//
// Some invariants to keep in mind: each declared Named type has a single
// corresponding object, and that object's type is the (possibly generic) Named
// type. Declared Named types are identical if and only if their pointers are
// identical. On the other hand, multiple instantiated Named types may be
// identical even though their pointers are not identical. One has to use
// Identical to compare them. For instantiated named types, their obj is a
// synthetic placeholder that records their position of the corresponding
// instantiation in the source (if they were constructed during type checking).
//
// To prevent infinite expansion of named instances that are created outside of
// type-checking, instances share a Context with other instances created during
// their expansion. Via the pidgeonhole principle, this guarantees that in the
// presence of a cycle of named types, expansion will eventually find an
// existing instance in the Context and short-circuit the expansion.
//
// Once an instance is fully expanded, we can nil out this shared Context to unpin
// memory, though the Context may still be held by other incomplete instances
// in its "lineage".

// A Named represents a named (defined) type.
//
// A declaration such as:
//
//	type S struct { ... }
//
// creates a defined type whose underlying type is a struct,
// and binds this type to the object S, a [TypeName].
// Use [Named.Underlying] to access the underlying type.
// Use [Named.Obj] to obtain the object S.
//
// Before type aliases (Go 1.9), the spec called defined types "named types".
type Named struct {
	check *Checker  // non-nil during type-checking; nil otherwise
	obj   *TypeName // corresponding declared object for declared types; see above for instantiated types

	allowNilRHS bool // may be true from creation via [NewNamed] until [Named.SetUnderlying]

	inst *instance // information for instantiated types; nil otherwise

	mu         sync.Mutex     // guards all fields below
	state_     uint32         // the current state of this type; must only be accessed atomically or when mu is held
	fromRHS    Type           // the declaration RHS this type is derived from
	tparams    *TypeParamList // type parameters, or nil
	underlying Type           // underlying type, or nil
	varSize    bool           // whether the type has variable size

	// methods declared for this type (not the method set of this type)
	// Signatures are type-checked lazily.
	// For non-instantiated types, this is a fully populated list of methods. For
	// instantiated types, methods are individually expanded when they are first
	// accessed.
	methods []*Func

	// loader may be provided to lazily load type parameters, underlying type, methods, and delayed functions
	loader func(*Named) ([]*TypeParam, Type, []*Func, []func())
}

// instance holds information that is only necessary for instantiated named
// types.
type instance struct {
	orig            *Named    // original, uninstantiated type
	targs           *TypeList // type arguments
	expandedMethods int       // number of expanded methods; expandedMethods <= len(orig.methods)
	ctxt            *Context  // local Context; set to nil after full expansion
}

// stateMask represents each state in the lifecycle of a named type.
//
// Each named type begins in the initial state. A named type may transition to a new state
// according to the below diagram:
//
//	initial
//	lazyLoaded
//	unpacked
//	└── hasMethods
//	└── hasUnder
//	└── hasVarSize
//
// That is, descent down the tree is mostly linear (initial through unpacked), except upon
// reaching the leaves (hasMethods, hasUnder, and hasVarSize). A type may occupy any
// combination of the leaf states at once (they are independent states).
//
// To represent this independence, the set of active states is represented with a bit set. State
// transitions are monotonic. Once a state bit is set, it remains set.
//
// The above constraints significantly narrow the possible bit sets for a named type. With bits
// set left-to-right, they are:
//
//	00000 | initial
//	10000 | lazyLoaded
//	11000 | unpacked, which implies lazyLoaded
//	11100 | hasMethods, which implies unpacked (which in turn implies lazyLoaded)
//	11010 | hasUnder, which implies unpacked ...
//	11001 | hasVarSize, which implies unpacked ...
//	11110 | both hasMethods and hasUnder which implies unpacked ...
//	...   | (other combinations of leaf states)
//
// To read the state of a named type, use [Named.stateHas]; to write, use [Named.setState].
type stateMask uint32

const (
	// initially, type parameters, RHS, underlying, and methods might be unavailable
	lazyLoaded stateMask = 1 << iota // methods are available, but constraints might be unexpanded (for generic types)
	unpacked                         // methods might be unexpanded (for instances)
	hasMethods                       // methods are all expanded (for instances)
	hasUnder                         // underlying type is available
	hasVarSize                       // varSize is available
)

// NewNamed returns a new named type for the given type name, underlying type, and associated methods.
// If the given type name obj doesn't have a type yet, its type is set to the returned named type.
// The underlying type must not be a *Named.
func NewNamed(obj *TypeName, underlying Type, methods []*Func) *Named {
	if asNamed(underlying) != nil {
		panic("underlying type must not be *Named")
	}
	n := (*Checker)(nil).newNamed(obj, underlying, methods)
	if underlying == nil {
		n.allowNilRHS = true
	} else {
		n.SetUnderlying(underlying)
	}
	return n

}

// unpack populates the type parameters, methods, and RHS of n.
//
// For the purposes of unpacking, there are three categories of named types:
//  1. Lazy loaded types
//  2. Instantiated types
//  3. All others
//
// Note that the above form a partition.
//
// Lazy loaded types:
// Type parameters, methods, and RHS of n become accessible and are fully
// expanded.
//
// Instantiated types:
// Type parameters, methods, and RHS of n become accessible, though methods
// are lazily populated as needed.
//
// All others:
// Effectively, nothing happens.
func (n *Named) unpack() *Named {
	if n.stateHas(lazyLoaded | unpacked) { // avoid locking below
		return n
	}

	// TODO(rfindley): if n.check is non-nil we can avoid locking here, since
	// type-checking is not concurrent. Evaluate if this is worth doing.
	n.mu.Lock()
	defer n.mu.Unlock()

	// only atomic for consistency; we are holding the mutex
	if n.stateHas(lazyLoaded | unpacked) {
		return n
	}

	// underlying comes after unpacking, do not set it
	defer (func() { assert(!n.stateHas(hasUnder)) })()

	if n.inst != nil {
		assert(n.fromRHS == nil) // instantiated types are not declared types
		assert(n.loader == nil)  // cannot import an instantiation

		orig := n.inst.orig
		orig.unpack()

		n.fromRHS = n.expandRHS()
		n.tparams = orig.tparams

		if len(orig.methods) == 0 {
			n.setState(lazyLoaded | unpacked | hasMethods) // nothing further to do
			n.inst.ctxt = nil
		} else {
			n.setState(lazyLoaded | unpacked)
		}
		return n
	}

	// TODO(mdempsky): Since we're passing n to the loader anyway
	// (necessary because types2 expects the receiver type for methods
	// on defined interface types to be the Named rather than the
	// underlying Interface), maybe it should just handle calling
	// SetTypeParams, SetUnderlying, and AddMethod instead?  Those
	// methods would need to support reentrant calls though. It would
	// also make the API more future-proof towards further extensions.
	if n.loader != nil {
		assert(n.fromRHS == nil) // not loaded yet
		assert(n.inst == nil)    // cannot import an instantiation

		tparams, underlying, methods, delayed := n.loader(n)
		n.loader = nil

		n.tparams = bindTParams(tparams)
		n.fromRHS = underlying // for cycle detection
		n.methods = methods

		n.setState(lazyLoaded) // avoid deadlock calling delayed functions
		for _, f := range delayed {
			f()
		}
	}

	n.setState(lazyLoaded | unpacked | hasMethods)
	return n
}

// stateHas atomically determines whether the current state includes any active bit in sm.
func (n *Named) stateHas(m stateMask) bool {
	return stateMask(atomic.LoadUint32(&n.state_))&m != 0
}

// setState atomically sets the current state to include each active bit in sm.
// Must only be called while holding n.mu.
func (n *Named) setState(m stateMask) {
	atomic.OrUint32(&n.state_, uint32(m))
	// verify state transitions
	if debug {
		m := stateMask(atomic.LoadUint32(&n.state_))
		u := m&unpacked != 0
		// unpacked => lazyLoaded
		if u {
			assert(m&lazyLoaded != 0)
		}
		// hasMethods => unpacked
		if m&hasMethods != 0 {
			assert(u)
		}
		// hasUnder => unpacked
		if m&hasUnder != 0 {
			assert(u)
		}
		// hasVarSize => unpacked
		if m&hasVarSize != 0 {
			assert(u)
		}
	}
}

// newNamed is like NewNamed but with a *Checker receiver.
func (check *Checker) newNamed(obj *TypeName, fromRHS Type, methods []*Func) *Named {
	typ := &Named{check: check, obj: obj, fromRHS: fromRHS, methods: methods}
	if obj.typ == nil {
		obj.typ = typ
	}
	// Ensure that typ is always sanity-checked.
	if check != nil {
		check.needsCleanup(typ)
	}
	return typ
}

// newNamedInstance creates a new named instance for the given origin and type
// arguments, recording pos as the position of its synthetic object (for error
// reporting).
//
// If set, expanding is the named type instance currently being expanded, that
// led to the creation of this instance.
func (check *Checker) newNamedInstance(pos syntax.Pos, orig *Named, targs []Type, expanding *Named) *Named {
	assert(len(targs) > 0)

	obj := NewTypeName(pos, orig.obj.pkg, orig.obj.name, nil)
	inst := &instance{orig: orig, targs: newTypeList(targs)}

	// Only pass the expanding context to the new instance if their packages
	// match. Since type reference cycles are only possible within a single
	// package, this is sufficient for the purposes of short-circuiting cycles.
	// Avoiding passing the context in other cases prevents unnecessary coupling
	// of types across packages.
	if expanding != nil && expanding.Obj().pkg == obj.pkg {
		inst.ctxt = expanding.inst.ctxt
	}
	typ := &Named{check: check, obj: obj, inst: inst}
	obj.typ = typ
	// Ensure that typ is always sanity-checked.
	if check != nil {
		check.needsCleanup(typ)
	}
	return typ
}

func (n *Named) cleanup() {
	// Instances can have a nil underlying at the end of type checking — they
	// will lazily expand it as needed. All other types must have one.
	if n.inst == nil {
		n.Underlying()
	}
	n.check = nil
}

// Obj returns the type name for the declaration defining the named type t. For
// instantiated types, this is same as the type name of the origin type.
func (t *Named) Obj() *TypeName {
	if t.inst == nil {
		return t.obj
	}
	return t.inst.orig.obj
}

// Origin returns the generic type from which the named type t is
// instantiated. If t is not an instantiated type, the result is t.
func (t *Named) Origin() *Named {
	if t.inst == nil {
		return t
	}
	return t.inst.orig
}

// TypeParams returns the type parameters of the named type t, or nil.
// The result is non-nil for an (originally) generic type even if it is instantiated.
func (t *Named) TypeParams() *TypeParamList { return t.unpack().tparams }

// SetTypeParams sets the type parameters of the named type t.
// t must not have type arguments.
func (t *Named) SetTypeParams(tparams []*TypeParam) {
	assert(t.inst == nil)
	t.unpack().tparams = bindTParams(tparams)
}

// TypeArgs returns the type arguments used to instantiate the named type t.
func (t *Named) TypeArgs() *TypeList {
	if t.inst == nil {
		return nil
	}
	return t.inst.targs
}

// NumMethods returns the number of explicit methods defined for t.
func (t *Named) NumMethods() int {
	return len(t.Origin().unpack().methods)
}

// Method returns the i'th method of named type t for 0 <= i < t.NumMethods().
//
// For an ordinary or instantiated type t, the receiver base type of this method
// is the named type t. The returned Func's Signature will not have receiver
// type parameters.
//
// For an uninstantiated generic type t, each method receiver is instantiated with
// its receiver type parameters. The returned Func's Signature will have the
// receiver type parameters used to instantiate the receiver.
//
// Methods are numbered deterministically: given the same list of source files
// presented to the type checker, or the same sequence of NewMethod and AddMethod
// calls, the mapping from method index to corresponding method remains the same.
// But the specific ordering is not specified and must not be relied on as it may
// change in the future.
func (t *Named) Method(i int) *Func {
	t.unpack()

	if t.stateHas(hasMethods) {
		return t.methods[i]
	}

	assert(t.inst != nil) // only instances should have unexpanded methods
	orig := t.inst.orig

	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.methods) != len(orig.methods) {
		assert(len(t.methods) == 0)
		t.methods = make([]*Func, len(orig.methods))
	}

	if t.methods[i] == nil {
		assert(t.inst.ctxt != nil) // we should still have a context remaining from the resolution phase
		t.methods[i] = t.expandMethod(i)
		t.inst.expandedMethods++

		// Check if we've created all methods at this point. If we have, mark the
		// type as having all of its methods.
		if t.inst.expandedMethods == len(orig.methods) {
			t.setState(hasMethods)
			t.inst.ctxt = nil // no need for a context anymore
		}
	}

	return t.methods[i]
}

// expandMethod substitutes type arguments in the i'th method for an
// instantiated receiver. A returned Func's Signature never has
// receiver type parameters.
func (t *Named) expandMethod(i int) *Func {
	// t.orig.methods is not lazy. orig is the declared function on t, which
	// must have receiver type parameters (since t is generic).
	orig := t.inst.orig.Method(i)
	assert(orig != nil)

	check := t.check
	// Ensure that the original method is type-checked.
	if check != nil {
		check.objDecl(orig)
	}

	oldSig := orig.typ.(*Signature)
	rtpars := oldSig.rparams.list()
	rtargs := t.inst.targs.list()

	// Consider:
	//
	// 	type T[P any] struct{}
	// 	func (t T[P]) m() { t.m() }
	//
	// At t.m, m is expanded for T[P] to get a new Func, which must be different from
	// the declared Func for the origin method T.m; notably, the Func for t.m lacks
	// receiver type parameters, since it is instantiated (as opposed to declared)
	// and thus no longer generic. One must not return the origin method here.

	// We can only substitute if we have a correspondence between type arguments
	// and type parameters. This check is necessary in the presence of invalid
	// code.
	newSig := oldSig
	if len(rtpars) == len(rtargs) {
		smap := makeSubstMap(rtpars, rtargs)
		var ctxt *Context
		if check != nil {
			ctxt = check.context()
		}
		newSig = check.subst(orig.pos, oldSig, smap, t, ctxt).(*Signature)
	}

	if newSig == oldSig {
		// No substitution occurred, but we still need to create a new signature to
		// hold the instantiated receiver.
		copy := *oldSig
		newSig = &copy
	}

	var rtyp Type
	if orig.hasPtrRecv() {
		rtyp = NewPointer(t)
	} else {
		rtyp = t
	}

	newSig.recv = cloneVar(oldSig.recv, rtyp)
	newSig.rparams = nil

	return cloneFunc(orig, newSig)
}

// SetUnderlying sets the underlying type and marks t as complete.
// t must not have type arguments.
func (t *Named) SetUnderlying(u Type) {
	assert(t.inst == nil)
	if u == nil {
		panic("underlying type must not be nil")
	}
	if asNamed(u) != nil {
		panic("underlying type must not be *Named")
	}
	// be careful to uphold the state invariants
	t.mu.Lock()
	defer t.mu.Unlock()

	t.fromRHS = u
	t.allowNilRHS = false
	t.setState(lazyLoaded | unpacked | hasMethods) // TODO(markfreeman): Why hasMethods?

	t.underlying = u
	t.setState(hasUnder)
}

// AddMethod adds method m unless it is already in the method list.
// The method must be in the same package as t, and t must not have
// type arguments.
func (t *Named) AddMethod(m *Func) {
	assert(samePkg(t.obj.pkg, m.pkg))
	assert(t.inst == nil)
	t.unpack()
	if t.methodIndex(m.name, false) < 0 {
		t.methods = append(t.methods, m)
	}
}

// methodIndex returns the index of the method with the given name.
// If foldCase is set, capitalization in the name is ignored.
// The result is negative if no such method exists.
func (t *Named) methodIndex(name string, foldCase bool) int {
	if name == "_" {
		return -1
	}
	if foldCase {
		for i, m := range t.methods {
			if strings.EqualFold(m.name, name) {
				return i
			}
		}
	} else {
		for i, m := range t.methods {
			if m.name == name {
				return i
			}
		}
	}
	return -1
}

// rhs returns [Named.fromRHS].
//
// In debug mode, it also asserts that n is in an appropriate state.
func (n *Named) rhs() Type {
	if debug {
		assert(n.stateHas(lazyLoaded | unpacked))
	}
	return n.fromRHS
}

// Underlying returns the [underlying type] of the named type t, resolving all
// forwarding declarations. Underlying types are never Named, TypeParam, or
// Alias types.
//
// [underlying type]: https://go.dev/ref/spec#Underlying_types.
func (n *Named) Underlying() Type {
	n.unpack()

	// The gccimporter depends on writing a nil underlying via NewNamed and
	// immediately reading it back. Rather than putting that in Named.under
	// and complicating things there, we just check for that special case here.
	if n.rhs() == nil {
		assert(n.allowNilRHS)
		return nil
	}

	if !n.stateHas(hasUnder) { // minor performance optimization
		n.resolveUnderlying()
	}

	return n.underlying
}

func (t *Named) String() string { return TypeString(t, nil) }

// ----------------------------------------------------------------------------
// Implementation
//
// TODO(rfindley): reorganize the loading and expansion methods under this
// heading.

// resolveUnderlying computes the underlying type of n. If n already has an
// underlying type, nothing happens.
//
// It does so by following RHS type chains for alias and named types. If any
// other type T is found, each named type in the chain has its underlying
// type set to T. Aliases are skipped because their underlying type is
// not memoized.
//
// resolveUnderlying assumes that there are no direct cycles; if there were
// any, they were broken (by setting the respective types to invalid) during
// the directCycles check phase.
func (n *Named) resolveUnderlying() {
	assert(n.stateHas(lazyLoaded | unpacked))

	var seen map[*Named]bool // for debugging only
	if debug {
		seen = make(map[*Named]bool)
	}

	var path []*Named
	var u Type
	for rhs := Type(n); u == nil; {
		switch t := rhs.(type) {
		case *Alias:
			rhs = unalias(t)

		case *Named:
			if debug {
				assert(!seen[t])
				seen[t] = true
			}

			// don't recalculate the underlying
			if t.stateHas(hasUnder) {
				u = t.underlying
				break
			}

			if debug {
				seen[t] = true
			}
			path = append(path, t)

			t.unpack()
			rhs = t.rhs()
			assert(rhs != nil)

		default:
			u = rhs // any type literal or predeclared type works
		}
	}

	for _, t := range path {
		func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			// Careful, t.underlying has lock-free readers. Since we might be racing
			// another call to resolveUnderlying, we have to avoid overwriting
			// t.underlying. Otherwise, the race detector will be tripped.
			if !t.stateHas(hasUnder) {
				t.underlying = u
				t.setState(hasUnder)
			}
		}()
	}
}

func (n *Named) lookupMethod(pkg *Package, name string, foldCase bool) (int, *Func) {
	n.unpack()
	if samePkg(n.obj.pkg, pkg) || isExported(name) || foldCase {
		// If n is an instance, we may not have yet instantiated all of its methods.
		// Look up the method index in orig, and only instantiate method at the
		// matching index (if any).
		if i := n.Origin().methodIndex(name, foldCase); i >= 0 {
			// For instances, m.Method(i) will be different from the orig method.
			return i, n.Method(i)
		}
	}
	return -1, nil
}

// context returns the type-checker context.
func (check *Checker) context() *Context {
	if check.ctxt == nil {
		check.ctxt = NewContext()
	}
	return check.ctxt
}

// expandRHS crafts a synthetic RHS for an instantiated type using the RHS of
// its origin type (which must be a generic type).
//
// Suppose that we had:
//
//	type T[P any] struct {
//	  f P
//	}
//
//	type U T[int]
//
// When we go to U, we observe T[int]. Since T[int] is an instantiation, it has no
// declaration. Here, we craft a synthetic RHS for T[int] as if it were declared,
// somewhat similar to:
//
//	type T[int] struct {
//	  f int
//	}
//
// And note that the synthetic RHS here is the same as the underlying for U. Now,
// consider:
//
//	type T[_ any] U
//	type U int
//	type V T[U]
//
// The synthetic RHS for T[U] becomes:
//
//	type T[U] U
//
// Whereas the underlying of V is int, not U.
func (n *Named) expandRHS() (rhs Type) {
	check := n.check
	if check != nil && check.conf.Trace {
		check.trace(n.obj.pos, "-- Named.expandRHS %s", n)
		check.indent++
		defer func() {
			check.indent--
			check.trace(n.obj.pos, "=> %s (rhs = %s)", n, rhs)
		}()
	}

	assert(!n.stateHas(unpacked))
	assert(n.inst.orig.stateHas(lazyLoaded | unpacked))

	if n.inst.ctxt == nil {
		n.inst.ctxt = NewContext()
	}

	ctxt := n.inst.ctxt
	orig := n.inst.orig

	targs := n.inst.targs
	tpars := orig.tparams

	if targs.Len() != tpars.Len() {
		return Typ[Invalid]
	}

	h := ctxt.instanceHash(orig, targs.list())
	u := ctxt.update(h, orig, targs.list(), n) // block fixed point infinite instantiation
	assert(n == u)

	m := makeSubstMap(tpars.list(), targs.list())
	if check != nil {
		ctxt = check.context()
	}

	rhs = check.subst(n.obj.pos, orig.rhs(), m, n, ctxt)

	// TODO(markfreeman): Can we handle this in substitution?
	// If the RHS is an interface, we must set the receiver of interface methods
	// to the named type.
	if iface, _ := rhs.(*Interface); iface != nil {
		if methods, copied := replaceRecvType(iface.methods, orig, n); copied {
			// If the RHS doesn't use type parameters, it may not have been
			// substituted; we need to craft a new interface first.
			if iface == orig.rhs() {
				assert(iface.complete) // otherwise we are copying incomplete data

				crafted := check.newInterface()
				crafted.complete = true
				crafted.implicit = false
				crafted.embeddeds = iface.embeddeds

				iface = crafted
			}
			iface.methods = methods
			iface.tset = nil // recompute type set with new methods

			// go.dev/issue/61561: We have to complete the interface even without a checker.
			if check == nil {
				iface.typeSet()
			}

			return iface
		}
	}

	return rhs
}

// safeUnderlying returns the underlying type of typ without expanding
// instances, to avoid infinite recursion.
//
// TODO(rfindley): eliminate this function or give it a better name.
func safeUnderlying(typ Type) Type {
	if t := asNamed(typ); t != nil {
		return t.underlying
	}
	return typ.Underlying()
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/object.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"bytes"
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	"strings"
	"unicode"
	"unicode/utf8"
)

// An Object is a named language entity.
// An Object may be a constant ([Const]), type name ([TypeName]),
// variable or struct field ([Var]), function or method ([Func]),
// imported package ([PkgName]), label ([Label]),
// built-in function ([Builtin]),
// or the predeclared identifier 'nil' ([Nil]).
//
// The environment, which is structured as a tree of Scopes,
// maps each name to the unique Object that it denotes.
type Object interface {
	Parent() *Scope  // scope in which this object is declared; nil for methods and struct fields
	Pos() syntax.Pos // position of object identifier in declaration
	Pkg() *Package   // package to which this object belongs; nil for labels and objects in the Universe scope
	Name() string    // package local object name
	Type() Type      // object type
	Exported() bool  // reports whether the name starts with a capital letter
	Id() string      // object name if exported, qualified name if not exported (see func Id)

	// String returns a human-readable string of the object.
	// Use [ObjectString] to control how package names are formatted in the string.
	String() string

	// order reflects a package-level object's source order: if object
	// a is before object b in the source, then a.order() < b.order().
	// order returns a value > 0 for package-level objects; it returns
	// 0 for all other objects (including objects in file scopes).
	order() uint32

	// setType sets the type of the object.
	setType(Type)

	// setOrder sets the order number of the object. It must be > 0.
	setOrder(uint32)

	// setParent sets the parent scope of the object.
	setParent(*Scope)

	// sameId reports whether obj.Id() and Id(pkg, name) are the same.
	// If foldCase is true, names are considered equal if they are equal with case folding
	// and their packages are ignored (e.g., pkg1.m, pkg1.M, pkg2.m, and pkg2.M are all equal).
	sameId(pkg *Package, name string, foldCase bool) bool

	// scopePos returns the start position of the scope of this Object
	scopePos() syntax.Pos

	// setScopePos sets the start position of the scope for this Object.
	setScopePos(pos syntax.Pos)
}

func isExported(name string) bool {
	ch, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(ch)
}

// Id returns name if it is exported, otherwise it
// returns the name qualified with the package path.
func Id(pkg *Package, name string) string {
	if isExported(name) {
		return name
	}
	// unexported names need the package path for differentiation
	// (if there's no package, make sure we don't start with '.'
	// as that may change the order of methods between a setup
	// inside a package and outside a package - which breaks some
	// tests)
	path := "_"
	// pkg is nil for objects in Universe scope and possibly types
	// introduced via Eval (see also comment in object.sameId)
	if pkg != nil && pkg.path != "" {
		path = pkg.path
	}
	return path + "." + name
}

// An object implements the common parts of an Object.
type object struct {
	parent    *Scope
	pos       syntax.Pos
	pkg       *Package
	name      string
	typ       Type
	order_    uint32
	scopePos_ syntax.Pos
}

// Parent returns the scope in which the object is declared.
// The result is nil for methods and struct fields.
func (obj *object) Parent() *Scope { return obj.parent }

// Pos returns the declaration position of the object's identifier.
func (obj *object) Pos() syntax.Pos { return obj.pos }

// Pkg returns the package to which the object belongs.
// The result is nil for labels and objects in the Universe scope.
func (obj *object) Pkg() *Package { return obj.pkg }

// Name returns the object's (package-local, unqualified) name.
func (obj *object) Name() string { return obj.name }

// Type returns the object's type.
func (obj *object) Type() Type { return obj.typ }

// Exported reports whether the object is exported (starts with a capital letter).
// It doesn't take into account whether the object is in a local (function) scope
// or not.
func (obj *object) Exported() bool { return isExported(obj.name) }

// Id is a wrapper for Id(obj.Pkg(), obj.Name()).
func (obj *object) Id() string { return Id(obj.pkg, obj.name) }

func (obj *object) String() string       { panic("abstract") }
func (obj *object) order() uint32        { return obj.order_ }
func (obj *object) scopePos() syntax.Pos { return obj.scopePos_ }

func (obj *object) setParent(parent *Scope)    { obj.parent = parent }
func (obj *object) setType(typ Type)           { obj.typ = typ }
func (obj *object) setOrder(order uint32)      { assert(order > 0); obj.order_ = order }
func (obj *object) setScopePos(pos syntax.Pos) { obj.scopePos_ = pos }

func (obj *object) sameId(pkg *Package, name string, foldCase bool) bool {
	// If we don't care about capitalization, we also ignore packages.
	if foldCase && strings.EqualFold(obj.name, name) {
		return true
	}
	// spec:
	// "Two identifiers are different if they are spelled differently,
	// or if they appear in different packages and are not exported.
	// Otherwise, they are the same."
	if obj.name != name {
		return false
	}
	// obj.Name == name
	if obj.Exported() {
		return true
	}
	// not exported, so packages must be the same
	return samePkg(obj.pkg, pkg)
}

// cmp reports whether object a is ordered before object b.
// cmp returns:
//
//	-1 if a is before b
//	 0 if a is equivalent to b
//	+1 if a is behind b
//
// Objects are ordered nil before non-nil, exported before
// non-exported, then by name, and finally (for non-exported
// functions) by package path.
func (a *object) cmp(b *object) int {
	if a == b {
		return 0
	}

	// Nil before non-nil.
	if a == nil {
		return -1
	}
	if b == nil {
		return +1
	}

	// Exported functions before non-exported.
	ea := isExported(a.name)
	eb := isExported(b.name)
	if ea != eb {
		if ea {
			return -1
		}
		return +1
	}

	// Order by name and then (for non-exported names) by package.
	if a.name != b.name {
		return strings.Compare(a.name, b.name)
	}
	if !ea {
		return strings.Compare(a.pkg.path, b.pkg.path)
	}

	return 0
}

// A PkgName represents an imported Go package.
// PkgNames don't have a type.
type PkgName struct {
	object
	imported *Package
}

// NewPkgName returns a new PkgName object representing an imported package.
// The remaining arguments set the attributes found with all Objects.
func NewPkgName(pos syntax.Pos, pkg *Package, name string, imported *Package) *PkgName {
	return &PkgName{object{nil, pos, pkg, name, Typ[Invalid], 0, nopos}, imported}
}

// Imported returns the package that was imported.
// It is distinct from Pkg(), which is the package containing the import statement.
func (obj *PkgName) Imported() *Package { return obj.imported }

// A Const represents a declared constant.
type Const struct {
	object
	val constant.Value
}

// NewConst returns a new constant with value val.
// The remaining arguments set the attributes found with all Objects.
func NewConst(pos syntax.Pos, pkg *Package, name string, typ Type, val constant.Value) *Const {
	return &Const{object{nil, pos, pkg, name, typ, 0, nopos}, val}
}

// Val returns the constant's value.
func (obj *Const) Val() constant.Value { return obj.val }

func (*Const) isDependency() {} // a constant may be a dependency of an initialization expression

// A TypeName is an [Object] that represents a type with a name:
// a defined type ([Named]),
// an alias type ([Alias]),
// a type parameter ([TypeParam]),
// or a predeclared type such as int or error.
type TypeName struct {
	object
}

// NewTypeName returns a new type name denoting the given typ.
// The remaining arguments set the attributes found with all Objects.
//
// The typ argument may be a defined (Named) type or an alias type.
// It may also be nil such that the returned TypeName can be used as
// argument for NewNamed, which will set the TypeName's type as a side-
// effect.
func NewTypeName(pos syntax.Pos, pkg *Package, name string, typ Type) *TypeName {
	return &TypeName{object{nil, pos, pkg, name, typ, 0, nopos}}
}

// NewTypeNameLazy returns a new defined type like NewTypeName, but it
// lazily calls unpack to finish constructing the Named object.
func NewTypeNameLazy(pos syntax.Pos, pkg *Package, name string, load func(*Named) ([]*TypeParam, Type, []*Func, []func())) *TypeName {
	obj := NewTypeName(pos, pkg, name, nil)
	n := (*Checker)(nil).newNamed(obj, nil, nil)
	n.loader = load
	return obj
}

// IsAlias reports whether obj is an alias name for a type.
func (obj *TypeName) IsAlias() bool {
	switch t := obj.typ.(type) {
	case nil:
		return false
	// case *Alias:
	//	handled by default case
	case *Basic:
		// unsafe.Pointer is not an alias.
		if obj.pkg == Unsafe {
			return false
		}
		// Any user-defined type name for a basic type is an alias for a
		// basic type (because basic types are pre-declared in the Universe
		// scope, outside any package scope), and so is any type name with
		// a different name than the name of the basic type it refers to.
		// Additionally, we need to look for "byte" and "rune" because they
		// are aliases but have the same names (for better error messages).
		return obj.pkg != nil || t.name != obj.name || t == universeByte || t == universeRune
	case *Named:
		return obj != t.obj
	case *TypeParam:
		return obj != t.obj
	default:
		return true
	}
}

// A Var represents a declared variable (including function parameters and results, and struct fields).
type Var struct {
	object
	origin   *Var // if non-nil, the Var from which this one was instantiated
	kind     VarKind
	embedded bool // if set, the variable is an embedded struct field, and name is the type name
}

// A VarKind discriminates the various kinds of variables.
type VarKind uint8

const (
	_          VarKind = iota // (not meaningful)
	PackageVar                // a package-level variable
	LocalVar                  // a local variable
	RecvVar                   // a method receiver variable
	ParamVar                  // a function parameter variable
	ResultVar                 // a function result variable
	FieldVar                  // a struct field
)

var varKindNames = [...]string{
	0:          "VarKind(0)",
	PackageVar: "PackageVar",
	LocalVar:   "LocalVar",
	RecvVar:    "RecvVar",
	ParamVar:   "ParamVar",
	ResultVar:  "ResultVar",
	FieldVar:   "FieldVar",
}

func (kind VarKind) String() string {
	if 0 <= kind && int(kind) < len(varKindNames) {
		return varKindNames[kind]
	}
	return fmt.Sprintf("VarKind(%d)", kind)
}

// Kind reports what kind of variable v is.
func (v *Var) Kind() VarKind { return v.kind }

// SetKind sets the kind of the variable.
// It should be used only immediately after [NewVar] or [NewParam].
func (v *Var) SetKind(kind VarKind) { v.kind = kind }

// NewVar returns a new variable.
// The arguments set the attributes found with all Objects.
//
// The caller must subsequently call [Var.SetKind]
// if the desired Var is not of kind [PackageVar].
func NewVar(pos syntax.Pos, pkg *Package, name string, typ Type) *Var {
	return newVar(PackageVar, pos, pkg, name, typ)
}

// NewParam returns a new variable representing a function parameter.
//
// The caller must subsequently call [Var.SetKind] if the desired Var
// is not of kind [ParamVar]: for example, [RecvVar] or [ResultVar].
func NewParam(pos syntax.Pos, pkg *Package, name string, typ Type) *Var {
	return newVar(ParamVar, pos, pkg, name, typ)
}

// NewField returns a new variable representing a struct field.
// For embedded fields, the name is the unqualified type name
// under which the field is accessible.
func NewField(pos syntax.Pos, pkg *Package, name string, typ Type, embedded bool) *Var {
	v := newVar(FieldVar, pos, pkg, name, typ)
	v.embedded = embedded
	return v
}

// newVar returns a new variable.
// The arguments set the attributes found with all Objects.
func newVar(kind VarKind, pos syntax.Pos, pkg *Package, name string, typ Type) *Var {
	return &Var{object: object{nil, pos, pkg, name, typ, 0, nopos}, kind: kind}
}

// Anonymous reports whether the variable is an embedded field.
// Same as Embedded; only present for backward-compatibility.
func (obj *Var) Anonymous() bool { return obj.embedded }

// Embedded reports whether the variable is an embedded field.
func (obj *Var) Embedded() bool { return obj.embedded }

// IsField reports whether the variable is a struct field.
func (obj *Var) IsField() bool { return obj.kind == FieldVar }

// Origin returns the canonical Var for its receiver, i.e. the Var object
// recorded in Info.Defs.
//
// For synthetic Vars created during instantiation (such as struct fields or
// function parameters that depend on type arguments), this will be the
// corresponding Var on the generic (uninstantiated) type. For all other Vars
// Origin returns the receiver.
func (obj *Var) Origin() *Var {
	if obj.origin != nil {
		return obj.origin
	}
	return obj
}

func (*Var) isDependency() {} // a variable may be a dependency of an initialization expression

// A Func represents a declared function, concrete method, or abstract
// (interface) method. Its Type() is always a *Signature.
// An abstract method may belong to many interfaces due to embedding.
type Func struct {
	object
	origin      *Func // if non-nil, the Func from which this one was instantiated
	hasPtrRecv_ bool  // only valid for methods that don't have a type yet; use hasPtrRecv() to read
	nointerface bool
}

// NewFunc returns a new function with the given signature, representing
// the function's type.
func NewFunc(pos syntax.Pos, pkg *Package, name string, sig *Signature) *Func {
	var typ Type
	if sig != nil {
		typ = sig
	} else {
		// Don't store a (typed) nil *Signature.
		// We can't simply replace it with new(Signature) either,
		// as this would violate object.{Type,color} invariants.
		// TODO(adonovan): propose to disallow NewFunc with nil *Signature.
	}
	return &Func{object{nil, pos, pkg, name, typ, 0, nopos}, nil, false, false}
}

// Signature returns the signature (type) of the function or method.
func (obj *Func) Signature() *Signature {
	if obj.typ != nil {
		return obj.typ.(*Signature) // normal case
	}
	// No signature: Signature was called either:
	// - within go/types, before a FuncDecl's initially
	//   nil Func.Type was lazily populated, indicating
	//   a types bug; or
	// - by a client after NewFunc(..., nil),
	//   which is arguably a client bug, but we need a
	//   proposal to tighten NewFunc's precondition.
	// For now, return a trivial signature.
	return new(Signature)
}

// FullName returns the package- or receiver-type-qualified name of
// function or method obj.
func (obj *Func) FullName() string {
	var buf bytes.Buffer
	writeFuncName(&buf, obj, nil)
	return buf.String()
}

// Scope returns the scope of the function's body block.
// The result is nil for imported or instantiated functions and methods
// (but there is also no mechanism to get to an instantiated function).
func (obj *Func) Scope() *Scope { return obj.typ.(*Signature).scope }

// Origin returns the canonical Func for its receiver, i.e. the Func object
// recorded in Info.Defs.
//
// For synthetic functions created during instantiation (such as methods on an
// instantiated Named type or interface methods that depend on type arguments),
// this will be the corresponding Func on the generic (uninstantiated) type.
// For all other Funcs Origin returns the receiver.
func (obj *Func) Origin() *Func {
	if obj.origin != nil {
		return obj.origin
	}
	return obj
}

// Pkg returns the package to which the function belongs.
//
// The result is nil for methods of types in the Universe scope,
// like method Error of the error built-in interface type.
func (obj *Func) Pkg() *Package { return obj.object.Pkg() }

// hasPtrRecv reports whether the receiver is of the form *T for the given method obj.
func (obj *Func) hasPtrRecv() bool {
	// If a method's receiver type is set, use that as the source of truth for the receiver.
	// Caution: Checker.funcDecl (decl.go) marks a function by setting its type to an empty
	// signature. We may reach here before the signature is fully set up: we must explicitly
	// check if the receiver is set (we cannot just look for non-nil obj.typ).
	if sig, _ := obj.typ.(*Signature); sig != nil && sig.recv != nil {
		_, isPtr := deref(sig.recv.typ)
		return isPtr
	}

	// If a method's type is not set it may be a method/function that is:
	// 1) client-supplied (via NewFunc with no signature), or
	// 2) internally created but not yet type-checked.
	// For case 1) we can't do anything; the client must know what they are doing.
	// For case 2) we can use the information gathered by the resolver.
	return obj.hasPtrRecv_
}

func (*Func) isDependency() {} // a function may be a dependency of an initialization expression

// A Label represents a declared label.
// Labels don't have a type.
type Label struct {
	object
	used bool // set if the label was used
}

// NewLabel returns a new label.
func NewLabel(pos syntax.Pos, pkg *Package, name string) *Label {
	return &Label{object{pos: pos, pkg: pkg, name: name, typ: Typ[Invalid]}, false}
}

// A Builtin represents a built-in function.
// Builtins don't have a valid type.
type Builtin struct {
	object
	id builtinId
}

func newBuiltin(id builtinId) *Builtin {
	return &Builtin{object{name: predeclaredFuncs[id].name, typ: Typ[Invalid]}, id}
}

// Nil represents the predeclared value nil.
type Nil struct {
	object
}

func writeObject(buf *bytes.Buffer, obj Object, qf Qualifier) {
	var tname *TypeName
	typ := obj.Type()

	switch obj := obj.(type) {
	case *PkgName:
		fmt.Fprintf(buf, "package %s", obj.Name())
		if path := obj.imported.path; path != "" && path != obj.name {
			fmt.Fprintf(buf, " (%q)", path)
		}
		return

	case *Const:
		buf.WriteString("const")

	case *TypeName:
		tname = obj
		buf.WriteString("type")
		if isTypeParam(typ) {
			buf.WriteString(" parameter")
		}

	case *Var:
		if obj.IsField() {
			buf.WriteString("field")
		} else {
			buf.WriteString("var")
		}

	case *Func:
		buf.WriteString("func ")
		writeFuncName(buf, obj, qf)
		if typ != nil {
			WriteSignature(buf, typ.(*Signature), qf)
		}
		return

	case *Label:
		buf.WriteString("label")
		typ = nil

	case *Builtin:
		buf.WriteString("builtin")
		typ = nil

	case *Nil:
		buf.WriteString("nil")
		return

	default:
		panic(fmt.Sprintf("writeObject(%T)", obj))
	}

	buf.WriteByte(' ')

	// For package-level objects, qualify the name.
	if obj.Pkg() != nil && obj.Pkg().scope.Lookup(obj.Name()) == obj {
		buf.WriteString(packagePrefix(obj.Pkg(), qf))
	}
	buf.WriteString(obj.Name())

	if typ == nil {
		return
	}

	if tname != nil {
		switch t := typ.(type) {
		case *Basic:
			// Don't print anything more for basic types since there's
			// no more information.
			return
		case genericType:
			if t.TypeParams().Len() > 0 {
				newTypeWriter(buf, qf).tParamList(t.TypeParams().list())
			}
		}
		if tname.IsAlias() {
			buf.WriteString(" =")
			if alias, ok := typ.(*Alias); ok { // materialized? TODO(gri) Do we still need this (e.g. for byte, rune)?
				typ = alias.fromRHS
			}
		} else if t, _ := typ.(*TypeParam); t != nil {
			typ = t.bound
		} else {
			// TODO(gri) should this be fromRHS for *Named?
			// (See discussion in #66559.)
			typ = typ.Underlying()
		}
	}

	// Special handling for any: because WriteType will format 'any' as 'any',
	// resulting in the object string `type any = any` rather than `type any =
	// interface{}`. To avoid this, swap in a different empty interface.
	if obj.Name() == "any" && obj.Parent() == Universe {
		assert(Identical(typ, &emptyInterface))
		typ = &emptyInterface
	}

	buf.WriteByte(' ')
	WriteType(buf, typ, qf)
}

func packagePrefix(pkg *Package, qf Qualifier) string {
	if pkg == nil {
		return ""
	}
	var s string
	if qf != nil {
		s = qf(pkg)
	} else {
		s = pkg.Path()
	}
	if s != "" {
		s += "."
	}
	return s
}

// ObjectString returns the string form of obj.
// The Qualifier controls the printing of
// package-level objects, and may be nil.
func ObjectString(obj Object, qf Qualifier) string {
	var buf bytes.Buffer
	writeObject(&buf, obj, qf)
	return buf.String()
}

func (obj *PkgName) String() string  { return ObjectString(obj, nil) }
func (obj *Const) String() string    { return ObjectString(obj, nil) }
func (obj *TypeName) String() string { return ObjectString(obj, nil) }
func (obj *Var) String() string      { return ObjectString(obj, nil) }
func (obj *Func) String() string     { return ObjectString(obj, nil) }
func (obj *Label) String() string    { return ObjectString(obj, nil) }
func (obj *Builtin) String() string  { return ObjectString(obj, nil) }
func (obj *Nil) String() string      { return ObjectString(obj, nil) }

func writeFuncName(buf *bytes.Buffer, f *Func, qf Qualifier) {
	if f.typ != nil {
		sig := f.typ.(*Signature)
		if recv := sig.Recv(); recv != nil {
			buf.WriteByte('(')
			if _, ok := recv.Type().(*Interface); ok {
				// gcimporter creates abstract methods of
				// named interfaces using the interface type
				// (not the named type) as the receiver.
				// Don't print it in full.
				buf.WriteString("interface")
			} else {
				WriteType(buf, recv.Type(), qf)
			}
			buf.WriteByte(')')
			buf.WriteByte('.')
		} else if f.pkg != nil {
			buf.WriteString(packagePrefix(f.pkg, qf))
		}
	}
	buf.WriteString(f.name)
}

// objectKind returns a description of the object's kind.
func objectKind(obj Object) string {
	switch obj := obj.(type) {
	case *PkgName:
		return "package name"
	case *Const:
		return "constant"
	case *TypeName:
		if obj.IsAlias() {
			return "type alias"
		} else if _, ok := obj.Type().(*TypeParam); ok {
			return "type parameter"
		} else {
			return "defined type"
		}
	case *Var:
		switch obj.Kind() {
		case PackageVar:
			return "package-level variable"
		case LocalVar:
			return "local variable"
		case RecvVar:
			return "receiver"
		case ParamVar:
			return "parameter"
		case ResultVar:
			return "result variable"
		case FieldVar:
			return "struct field"
		}
	case *Func:
		if obj.Signature().Recv() != nil {
			return "method"
		} else {
			return "function"
		}
	case *Label:
		return "label"
	case *Builtin:
		return "built-in function"
	case *Nil:
		return "untyped nil"
	}
	if debug {
		panic(fmt.Sprintf("unknown symbol (%T)", obj))
	}
	return "unknown symbol"
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/objset.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements objsets.
//
// An objset is similar to a Scope but objset elements
// are identified by their unique id, instead of their
// object name.

package types2

// An objset is a set of objects identified by their unique id.
// The zero value for objset is a ready-to-use empty objset.
type objset map[string]Object // initialized lazily

// insert attempts to insert an object obj into objset s.
// If s already contains an alternative object alt with
// the same name, insert leaves s unchanged and returns alt.
// Otherwise it inserts obj and returns nil.
func (s *objset) insert(obj Object) Object {
	id := obj.Id()
	if alt := (*s)[id]; alt != nil {
		return alt
	}
	if *s == nil {
		*s = make(map[string]Object)
	}
	(*s)[id] = obj
	return nil
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/operand.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines operands and associated operations.

package types2

import (
	"bytes"
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	. "internal/types/errors"
)

// An operandMode specifies the (addressing) mode of an operand.
type operandMode byte

const (
	invalid   operandMode = iota // operand is invalid
	novalue                      // operand represents no value (result of a function call w/o result)
	builtin                      // operand is a built-in function
	typexpr                      // operand is a type
	constant_                    // operand is a constant; the operand's typ is a Basic type
	variable                     // operand is an addressable variable
	mapindex                     // operand is a map index expression (acts like a variable on lhs, commaok on rhs of an assignment)
	value                        // operand is a computed value
	nilvalue                     // operand is the nil value - only used by types2
	commaok                      // like value, but operand may be used in a comma,ok expression
	commaerr                     // like commaok, but second value is error, not boolean
	cgofunc                      // operand is a cgo function
)

var operandModeString = [...]string{
	invalid:   "invalid operand",
	novalue:   "no value",
	builtin:   "built-in",
	typexpr:   "type",
	constant_: "constant",
	variable:  "variable",
	mapindex:  "map index expression",
	value:     "value",
	nilvalue:  "nil", // only used by types2
	commaok:   "comma, ok expression",
	commaerr:  "comma, error expression",
	cgofunc:   "cgo function",
}

// An operand represents an intermediate value during type checking.
// Operands have an (addressing) mode, the expression evaluating to
// the operand, the operand's type, a value for constants, and an id
// for built-in functions.
// The zero value of operand is a ready to use invalid operand.
type operand struct {
	mode_ operandMode
	expr  syntax.Expr
	typ_  Type
	val   constant.Value
	id    builtinId
}

func (x *operand) mode() operandMode {
	return x.mode_
}

func (x *operand) typ() Type {
	return x.typ_
}

func (x *operand) isValid() bool {
	return x.mode() != invalid
}

func (x *operand) invalidate() {
	x.mode_ = invalid
}

// Pos returns the position of the expression corresponding to x.
// If x is invalid the position is nopos.
func (x *operand) Pos() syntax.Pos {
	// x.expr may not be set if x is invalid
	if x.expr == nil {
		return nopos
	}
	return x.expr.Pos()
}

// Operand string formats
// (not all "untyped" cases can appear due to the type system,
// but they fall out naturally here)
//
// mode       format
//
// invalid    <expr> (               <mode>                    )
// novalue    <expr> (               <mode>                    )
// builtin    <expr> (               <mode>                    )
// typexpr    <expr> (               <mode>                    )
//
// constant   <expr> (<untyped kind> <mode>                    )
// constant   <expr> (               <mode>       of type <typ>)
// constant   <expr> (<untyped kind> <mode> <val>              )
// constant   <expr> (               <mode> <val> of type <typ>)
//
// variable   <expr> (<untyped kind> <mode>                    )
// variable   <expr> (               <mode>       of type <typ>)
//
// mapindex   <expr> (<untyped kind> <mode>                    )
// mapindex   <expr> (               <mode>       of type <typ>)
//
// value      <expr> (<untyped kind> <mode>                    )
// value      <expr> (               <mode>       of type <typ>)
//
// nilvalue   untyped nil
// nilvalue   nil    (                            of type <typ>)
//
// commaok    <expr> (<untyped kind> <mode>                    )
// commaok    <expr> (               <mode>       of type <typ>)
//
// commaerr   <expr> (<untyped kind> <mode>                    )
// commaerr   <expr> (               <mode>       of type <typ>)
//
// cgofunc    <expr> (<untyped kind> <mode>                    )
// cgofunc    <expr> (               <mode>       of type <typ>)
func operandString(x *operand, qf Qualifier) string {
	// special-case nil
	if isTypes2 {
		if x.mode() == nilvalue {
			switch x.typ() {
			case nil, Typ[Invalid]:
				return "nil (with invalid type)"
			case Typ[UntypedNil]:
				return "nil"
			default:
				return fmt.Sprintf("nil (of type %s)", TypeString(x.typ(), qf))
			}
		}
	} else { // go/types
		if x.mode() == value && x.typ() == Typ[UntypedNil] {
			return "nil"
		}
	}

	var buf bytes.Buffer

	var expr string
	if x.expr != nil {
		expr = ExprString(x.expr)
	} else {
		switch x.mode() {
		case builtin:
			expr = predeclaredFuncs[x.id].name
		case typexpr:
			expr = TypeString(x.typ(), qf)
		case constant_:
			expr = x.val.String()
		}
	}

	// <expr> (
	if expr != "" {
		buf.WriteString(expr)
		buf.WriteString(" (")
	}

	// <untyped kind>
	hasType := false
	switch x.mode() {
	case invalid, novalue, builtin, typexpr:
		// no type
	default:
		// should have a type, but be cautious (don't crash during printing)
		if x.typ() != nil {
			if isUntyped(x.typ()) {
				buf.WriteString(x.typ().(*Basic).name)
				buf.WriteByte(' ')
				break
			}
			hasType = true
		}
	}

	// <mode>
	buf.WriteString(operandModeString[x.mode()])

	// <val>
	if x.mode() == constant_ {
		if s := x.val.String(); s != expr {
			buf.WriteByte(' ')
			buf.WriteString(s)
		}
	}

	// <typ>
	if hasType {
		if isValid(x.typ()) {
			var desc string
			if isGeneric(x.typ()) {
				desc = "generic "
			}

			// Describe the type structure if it is an *Alias or *Named type.
			// If the type is a renamed basic type, describe the basic type,
			// as in "int32 type MyInt" for a *Named type MyInt.
			// If it is a type parameter, describe the constraint instead.
			tpar, _ := Unalias(x.typ()).(*TypeParam)
			if tpar == nil {
				switch x.typ().(type) {
				case *Alias, *Named:
					what := compositeKind(x.typ())
					if what == "" {
						// x.typ must be basic type
						what = x.typ().Underlying().(*Basic).name
					}
					desc += what + " "
				}
			}
			// desc is "" or has a trailing space at the end

			buf.WriteString(" of " + desc + "type ")
			WriteType(&buf, x.typ(), qf)

			if tpar != nil {
				buf.WriteString(" constrained by ")
				WriteType(&buf, tpar.bound, qf) // do not compute interface type sets here
				// If we have the type set and it's empty, say so for better error messages.
				if hasEmptyTypeset(tpar) {
					buf.WriteString(" with empty type set")
				}
			}
		} else {
			buf.WriteString(" with invalid type")
		}
	}

	// )
	if expr != "" {
		buf.WriteByte(')')
	}

	return buf.String()
}

// compositeKind returns the kind of the given composite type
// ("array", "slice", etc.) or the empty string if typ is not
// composite but a basic type.
func compositeKind(typ Type) string {
	switch typ.Underlying().(type) {
	case *Basic:
		return ""
	case *Array:
		return "array"
	case *Slice:
		return "slice"
	case *Struct:
		return "struct"
	case *Pointer:
		return "pointer"
	case *Signature:
		return "func"
	case *Interface:
		return "interface"
	case *Map:
		return "map"
	case *Chan:
		return "chan"
	case *Tuple:
		return "tuple"
	case *Union:
		return "union"
	default:
		panic("unreachable")
	}
}

func (x *operand) String() string {
	return operandString(x, nil)
}

// setConst sets x to the untyped constant for literal lit.
func (x *operand) setConst(k syntax.LitKind, lit string) {
	var kind BasicKind
	switch k {
	case syntax.IntLit:
		kind = UntypedInt
	case syntax.FloatLit:
		kind = UntypedFloat
	case syntax.ImagLit:
		kind = UntypedComplex
	case syntax.RuneLit:
		kind = UntypedRune
	case syntax.StringLit:
		kind = UntypedString
	default:
		panic("unreachable")
	}

	val := makeFromLiteral(lit, k)
	if val.Kind() == constant.Unknown {
		x.invalidate()
		x.typ_ = Typ[Invalid]
		return
	}
	x.mode_ = constant_
	x.typ_ = Typ[kind]
	x.val = val
}

// isNil reports whether x is the (untyped) nil value.
func (x *operand) isNil() bool {
	if isTypes2 {
		return x.mode() == nilvalue
	} else { // go/types
		return x.mode() == value && x.typ() == Typ[UntypedNil]
	}
}

// assignableTo reports whether x is assignable to a variable of type T. If the
// result is false and a non-nil cause is provided, it may be set to a more
// detailed explanation of the failure (result != ""). The returned error code
// is only valid if the (first) result is false. The check parameter may be nil
// if assignableTo is invoked through an exported API call, i.e., when all
// methods have been type-checked.
func (x *operand) assignableTo(check *Checker, T Type, cause *string) (bool, Code) {
	if !x.isValid() || !isValid(T) {
		return true, 0 // avoid spurious errors
	}

	origT := T
	V := Unalias(x.typ())
	T = Unalias(T)

	// x's type is identical to T
	if Identical(V, T) {
		return true, 0
	}

	Vu := V.Underlying()
	Tu := T.Underlying()
	Vp, _ := V.(*TypeParam)
	Tp, _ := T.(*TypeParam)

	// x is an untyped value representable by a value of type T.
	if isUntyped(Vu) {
		assert(Vp == nil)
		if Tp != nil {
			// T is a type parameter: x is assignable to T if it is
			// representable by each specific type in the type set of T.
			return Tp.is(func(t *term) bool {
				if t == nil {
					return false
				}
				// A term may be a tilde term but the underlying
				// type of an untyped value doesn't change so we
				// don't need to do anything special.
				newType, _, _ := check.implicitTypeAndValue(x, t.typ)
				return newType != nil
			}), IncompatibleAssign
		}
		newType, _, _ := check.implicitTypeAndValue(x, T)
		return newType != nil, IncompatibleAssign
	}
	// Vu is typed

	// x's type V and T have identical underlying types
	// and at least one of V or T is not a named type
	// and neither V nor T is a type parameter.
	if Identical(Vu, Tu) && (!hasName(V) || !hasName(T)) && Vp == nil && Tp == nil {
		return true, 0
	}

	// T is an interface type, but not a type parameter, and V implements T.
	// Also handle the case where T is a pointer to an interface so that we get
	// the Checker.implements error cause.
	if _, ok := Tu.(*Interface); ok && Tp == nil || isInterfacePtr(Tu) {
		if check.implements(V, T, false, cause) {
			return true, 0
		}
		// V doesn't implement T but V may still be assignable to T if V
		// is a type parameter; do not report an error in that case yet.
		if Vp == nil {
			return false, InvalidIfaceAssign
		}
		if cause != nil {
			*cause = ""
		}
	}

	// If V is an interface, check if a missing type assertion is the problem.
	if Vi, _ := Vu.(*Interface); Vi != nil && Vp == nil {
		if check.implements(T, V, false, nil) {
			// T implements V, so give hint about type assertion.
			if cause != nil {
				*cause = "need type assertion"
			}
			return false, IncompatibleAssign
		}
	}

	// x is a bidirectional channel value, T is a channel
	// type, x's type V and T have identical element types,
	// and at least one of V or T is not a named type.
	if Vc, ok := Vu.(*Chan); ok && Vc.dir == SendRecv {
		if Tc, ok := Tu.(*Chan); ok && Identical(Vc.elem, Tc.elem) {
			return !hasName(V) || !hasName(T), InvalidChanAssign
		}
	}

	// optimization: if we don't have type parameters, we're done
	if Vp == nil && Tp == nil {
		return false, IncompatibleAssign
	}

	errorf := func(format string, args ...any) {
		if check != nil && cause != nil {
			msg := check.sprintf(format, args...)
			if *cause != "" {
				msg += "\n\t" + *cause
			}
			*cause = msg
		}
	}

	// x's type V is not a named type and T is a type parameter, and
	// x is assignable to each specific type in T's type set.
	if !hasName(V) && Tp != nil {
		ok := false
		code := IncompatibleAssign
		Tp.is(func(T *term) bool {
			if T == nil {
				return false // no specific types
			}
			ok, code = x.assignableTo(check, T.typ, cause)
			if !ok {
				errorf("cannot assign %s to %s (in %s)", x.typ(), T.typ, Tp)
				return false
			}
			return true
		})
		return ok, code
	}

	// x's type V is a type parameter and T is not a named type,
	// and values x' of each specific type in V's type set are
	// assignable to T.
	if Vp != nil && !hasName(T) {
		x := *x // don't clobber outer x
		ok := false
		code := IncompatibleAssign
		Vp.is(func(V *term) bool {
			if V == nil {
				return false // no specific types
			}
			x.typ_ = V.typ
			ok, code = x.assignableTo(check, T, cause)
			if !ok {
				errorf("cannot assign %s (in %s) to %s", V.typ, Vp, origT)
				return false
			}
			return true
		})
		return ok, code
	}

	return false, IncompatibleAssign
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/package.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"fmt"
)

// A Package describes a Go package.
type Package struct {
	path      string
	name      string
	scope     *Scope
	imports   []*Package
	complete  bool
	fake      bool   // scope lookup errors are silently dropped if package is fake (internal use only)
	cgo       bool   // uses of this package will be rewritten into uses of declarations from _cgo_gotypes.go
	goVersion string // minimum Go version required for package (by Config.GoVersion, typically from go.mod)
}

// NewPackage returns a new Package for the given package path and name.
// The package is not complete and contains no explicit imports.
func NewPackage(path, name string) *Package {
	scope := NewScope(Universe, nopos, nopos, fmt.Sprintf("package %q", path))
	return &Package{path: path, name: name, scope: scope}
}

// Path returns the package path.
func (pkg *Package) Path() string { return pkg.path }

// Name returns the package name.
func (pkg *Package) Name() string { return pkg.name }

// SetName sets the package name.
func (pkg *Package) SetName(name string) { pkg.name = name }

// GoVersion returns the minimum Go version required by this package.
// If the minimum version is unknown, GoVersion returns the empty string.
// Individual source files may specify a different minimum Go version,
// as reported in the [go/ast.File.GoVersion] field.
func (pkg *Package) GoVersion() string { return pkg.goVersion }

// Scope returns the (complete or incomplete) package scope
// holding the objects declared at package level (TypeNames,
// Consts, Vars, and Funcs).
// For a nil pkg receiver, Scope returns the Universe scope.
func (pkg *Package) Scope() *Scope {
	if pkg != nil {
		return pkg.scope
	}
	return Universe
}

// A package is complete if its scope contains (at least) all
// exported objects; otherwise it is incomplete.
func (pkg *Package) Complete() bool { return pkg.complete }

// MarkComplete marks a package as complete.
func (pkg *Package) MarkComplete() { pkg.complete = true }

// Imports returns the list of packages directly imported by
// pkg; the list is in source order.
//
// If pkg was loaded from export data, Imports includes packages that
// provide package-level objects referenced by pkg. This may be more or
// less than the set of packages directly imported by pkg's source code.
//
// If pkg uses cgo and the FakeImportC configuration option
// was enabled, the imports list may contain a fake "C" package.
func (pkg *Package) Imports() []*Package { return pkg.imports }

// SetImports sets the list of explicitly imported packages to list.
// It is the caller's responsibility to make sure list elements are unique.
func (pkg *Package) SetImports(list []*Package) { pkg.imports = list }

func (pkg *Package) String() string {
	return fmt.Sprintf("package %s (%q)", pkg.name, pkg.path)
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/pointer.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

// A Pointer represents a pointer type.
type Pointer struct {
	base Type // element type
}

// NewPointer returns a new pointer type for the given element (base) type.
func NewPointer(elem Type) *Pointer { return &Pointer{base: elem} }

// Elem returns the element type for the given pointer p.
func (p *Pointer) Elem() Type { return p.base }

func (p *Pointer) Underlying() Type { return p }
func (p *Pointer) String() string   { return TypeString(p, nil) }

```

// === FILE: references!/go/src/cmd/compile/internal/types2/predicates.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements commonly used type predicates.

package types2

import (
	"slices"
	"unicode"
)

// isValid reports whether t is a valid type.
func isValid(t Type) bool { return Unalias(t) != Typ[Invalid] }

// The isX predicates below report whether t is an X.
// If t is a type parameter the result is false; i.e.,
// these predicates don't look inside a type parameter.

func isBoolean(t Type) bool        { return isBasic(t, IsBoolean) }
func isInteger(t Type) bool        { return isBasic(t, IsInteger) }
func isUnsigned(t Type) bool       { return isBasic(t, IsUnsigned) }
func isFloat(t Type) bool          { return isBasic(t, IsFloat) }
func isComplex(t Type) bool        { return isBasic(t, IsComplex) }
func isNumeric(t Type) bool        { return isBasic(t, IsNumeric) }
func isString(t Type) bool         { return isBasic(t, IsString) }
func isIntegerOrFloat(t Type) bool { return isBasic(t, IsInteger|IsFloat) }
func isConstType(t Type) bool      { return isBasic(t, IsConstType) }

// isBasic reports whether t.Underlying() is a basic type with the specified info.
// If t is a type parameter the result is false; i.e.,
// isBasic does not look inside a type parameter.
func isBasic(t Type, info BasicInfo) bool {
	u, _ := t.Underlying().(*Basic)
	return u != nil && u.info&info != 0
}

// The allX predicates below report whether t is an X.
// If t is a type parameter the result is true if isX is true
// for all specified types of the type parameter's type set.

func allBoolean(t Type) bool         { return allBasic(t, IsBoolean) }
func allInteger(t Type) bool         { return allBasic(t, IsInteger) }
func allUnsigned(t Type) bool        { return allBasic(t, IsUnsigned) }
func allNumeric(t Type) bool         { return allBasic(t, IsNumeric) }
func allString(t Type) bool          { return allBasic(t, IsString) }
func allOrdered(t Type) bool         { return allBasic(t, IsOrdered) }
func allNumericOrString(t Type) bool { return allBasic(t, IsNumeric|IsString) }

// allBasic reports whether t.Underlying() is a basic type with the specified info.
// If t is a type parameter, the result is true if isBasic(t, info) is true
// for all specific types of the type parameter's type set.
func allBasic(t Type, info BasicInfo) bool {
	if tpar, _ := Unalias(t).(*TypeParam); tpar != nil {
		return tpar.is(func(t *term) bool { return t != nil && isBasic(t.typ, info) })
	}
	return isBasic(t, info)
}

// hasName reports whether t has a name. This includes
// predeclared types, defined types, and type parameters.
// hasName may be called with types that are not fully set up.
func hasName(t Type) bool {
	switch Unalias(t).(type) {
	case *Basic, *Named, *TypeParam:
		return true
	}
	return false
}

// isTypeLit reports whether t is a type literal.
// This includes all non-defined types, but also basic types.
// isTypeLit may be called with types that are not fully set up.
func isTypeLit(t Type) bool {
	switch Unalias(t).(type) {
	case *Named, *TypeParam:
		return false
	}
	return true
}

// isTyped reports whether t is typed; i.e., not an untyped
// constant or boolean.
// Safe to call from types that are not fully set up.
func isTyped(t Type) bool {
	// Alias and named types cannot denote untyped types
	// so there's no need to call Unalias or Underlying, below.
	b, _ := t.(*Basic)
	return b == nil || b.info&IsUntyped == 0
}

// isUntyped(t) is the same as !isTyped(t).
// Safe to call from types that are not fully set up.
func isUntyped(t Type) bool {
	return !isTyped(t)
}

// isUntypedNumeric reports whether t is an untyped numeric type.
// Safe to call from types that are not fully set up.
func isUntypedNumeric(t Type) bool {
	// Alias and named types cannot denote untyped types
	// so there's no need to call Unalias or Underlying, below.
	b, _ := t.(*Basic)
	return b != nil && b.info&IsUntyped != 0 && b.info&IsNumeric != 0
}

// IsInterface reports whether t is an interface type.
func IsInterface(t Type) bool {
	_, ok := t.Underlying().(*Interface)
	return ok
}

// isNonTypeParamInterface reports whether t is an interface type but not a type parameter.
func isNonTypeParamInterface(t Type) bool {
	return !isTypeParam(t) && IsInterface(t)
}

// isTypeParam reports whether t is a type parameter.
func isTypeParam(t Type) bool {
	_, ok := Unalias(t).(*TypeParam)
	return ok
}

// hasEmptyTypeset reports whether t is a type parameter with an empty type set.
// The function does not force the computation of the type set and so is safe to
// use anywhere, but it may report a false negative if the type set has not been
// computed yet.
func hasEmptyTypeset(t Type) bool {
	if tpar, _ := Unalias(t).(*TypeParam); tpar != nil && tpar.bound != nil {
		iface, _ := safeUnderlying(tpar.bound).(*Interface)
		return iface != nil && iface.tset != nil && iface.tset.IsEmpty()
	}
	return false
}

// isGeneric reports whether a type is a generic, uninstantiated type
// (generic signatures are not included).
// TODO(gri) should we include signatures or assert that they are not present?
func isGeneric(t Type) bool {
	// A parameterized type is only generic if it doesn't have an instantiation already.
	if alias, _ := t.(*Alias); alias != nil && alias.tparams != nil && alias.targs == nil {
		return true
	}
	named := asNamed(t)
	return named != nil && named.obj != nil && named.inst == nil && named.TypeParams().Len() > 0
}

// Comparable reports whether values of type T are comparable.
func Comparable(T Type) bool {
	return comparableType(T, true, nil) == nil
}

// If T is comparable, comparableType returns nil.
// Otherwise it returns a type error explaining why T is not comparable.
// If dynamic is set, non-type parameter interfaces are always comparable.
func comparableType(T Type, dynamic bool, seen map[Type]bool) *typeError {
	if seen[T] {
		return nil
	}
	if seen == nil {
		seen = make(map[Type]bool)
	}
	seen[T] = true

	switch t := T.Underlying().(type) {
	case *Basic:
		// assume invalid types to be comparable to avoid follow-up errors
		if t.kind == UntypedNil {
			return typeErrorf("")
		}

	case *Pointer, *Chan:
		// always comparable

	case *Struct:
		for _, f := range t.fields {
			if comparableType(f.typ, dynamic, seen) != nil {
				return typeErrorf("struct containing %s cannot be compared", f.typ)
			}
		}

	case *Array:
		if comparableType(t.elem, dynamic, seen) != nil {
			return typeErrorf("%s cannot be compared", T)
		}

	case *Interface:
		if dynamic && !isTypeParam(T) || t.typeSet().IsComparable(seen) {
			return nil
		}
		var cause string
		if t.typeSet().IsEmpty() {
			cause = "empty type set"
		} else {
			cause = "incomparable types in type set"
		}
		return typeErrorf(cause)

	default:
		return typeErrorf("")
	}

	return nil
}

// hasNil reports whether type t includes the nil value.
func hasNil(t Type) bool {
	switch u := t.Underlying().(type) {
	case *Basic:
		return u.kind == UnsafePointer
	case *Slice, *Pointer, *Signature, *Map, *Chan:
		return true
	case *Interface:
		return !isTypeParam(t) || underIs(t, func(u Type) bool {
			return u != nil && hasNil(u)
		})
	}
	return false
}

// samePkg reports whether packages a and b are the same.
func samePkg(a, b *Package) bool {
	// package is nil for objects in universe scope
	if a == nil || b == nil {
		return a == b
	}
	// a != nil && b != nil
	return a.path == b.path
}

// An ifacePair is a node in a stack of interface type pairs compared for identity.
type ifacePair struct {
	x, y *Interface
	prev *ifacePair
}

func (p *ifacePair) identical(q *ifacePair) bool {
	return p.x == q.x && p.y == q.y || p.x == q.y && p.y == q.x
}

// A comparer is used to compare types.
type comparer struct {
	ignoreTags     bool // if set, identical ignores struct tags
	ignoreInvalids bool // if set, identical treats an invalid type as identical to any type
}

// For changes to this code the corresponding changes should be made to unifier.nify.
func (c *comparer) identical(x, y Type, p *ifacePair) bool {
	x = Unalias(x)
	y = Unalias(y)

	if x == y {
		return true
	}

	if c.ignoreInvalids && (!isValid(x) || !isValid(y)) {
		return true
	}

	switch x := x.(type) {
	case *Basic:
		// Basic types are singletons except for the rune and byte
		// aliases, thus we cannot solely rely on the x == y check
		// above. See also comment in TypeName.IsAlias.
		if y, ok := y.(*Basic); ok {
			return x.kind == y.kind
		}

	case *Array:
		// Two array types are identical if they have identical element types
		// and the same array length.
		if y, ok := y.(*Array); ok {
			// If one or both array lengths are unknown (< 0) due to some error,
			// assume they are the same to avoid spurious follow-on errors.
			return (x.len < 0 || y.len < 0 || x.len == y.len) && c.identical(x.elem, y.elem, p)
		}

	case *Slice:
		// Two slice types are identical if they have identical element types.
		if y, ok := y.(*Slice); ok {
			return c.identical(x.elem, y.elem, p)
		}

	case *Struct:
		// Two struct types are identical if they have the same sequence of fields,
		// and if corresponding fields have the same names, and identical types,
		// and identical tags. Two embedded fields are considered to have the same
		// name. Lower-case field names from different packages are always different.
		if y, ok := y.(*Struct); ok {
			if x.NumFields() == y.NumFields() {
				for i, f := range x.fields {
					g := y.fields[i]
					if f.embedded != g.embedded ||
						!c.ignoreTags && x.Tag(i) != y.Tag(i) ||
						!f.sameId(g.pkg, g.name, false) ||
						!c.identical(f.typ, g.typ, p) {
						return false
					}
				}
				return true
			}
		}

	case *Pointer:
		// Two pointer types are identical if they have identical base types.
		if y, ok := y.(*Pointer); ok {
			return c.identical(x.base, y.base, p)
		}

	case *Tuple:
		// Two tuples types are identical if they have the same number of elements
		// and corresponding elements have identical types.
		if y, ok := y.(*Tuple); ok {
			if x.Len() == y.Len() {
				if x != nil {
					for i, v := range x.vars {
						w := y.vars[i]
						if !c.identical(v.typ, w.typ, p) {
							return false
						}
					}
				}
				return true
			}
		}

	case *Signature:
		y, _ := y.(*Signature)
		if y == nil {
			return false
		}

		// Two function types are identical if they have the same number of
		// parameters and result values, corresponding parameter and result types
		// are identical, and either both functions are variadic or neither is.
		// Parameter and result names are not required to match, and type
		// parameters are considered identical modulo renaming.

		if x.TypeParams().Len() != y.TypeParams().Len() {
			return false
		}

		// In the case of generic signatures, we will substitute in yparams and
		// yresults.
		yparams := y.params
		yresults := y.results

		if x.TypeParams().Len() > 0 {
			// We must ignore type parameter names when comparing x and y. The
			// easiest way to do this is to substitute x's type parameters for y's.
			xtparams := x.TypeParams().list()
			ytparams := y.TypeParams().list()

			var targs []Type
			for i := range xtparams {
				targs = append(targs, x.TypeParams().At(i))
			}
			smap := makeSubstMap(ytparams, targs)

			var check *Checker   // ok to call subst on a nil *Checker
			ctxt := NewContext() // need a non-nil Context for the substitution below

			// Constraints must be pair-wise identical, after substitution.
			for i, xtparam := range xtparams {
				ybound := check.subst(nopos, ytparams[i].bound, smap, nil, ctxt)
				if !c.identical(xtparam.bound, ybound, p) {
					return false
				}
			}

			yparams = check.subst(nopos, y.params, smap, nil, ctxt).(*Tuple)
			yresults = check.subst(nopos, y.results, smap, nil, ctxt).(*Tuple)
		}

		return x.variadic == y.variadic &&
			c.identical(x.params, yparams, p) &&
			c.identical(x.results, yresults, p)

	case *Union:
		if y, _ := y.(*Union); y != nil {
			// TODO(rfindley): can this be reached during type checking? If so,
			// consider passing a type set map.
			unionSets := make(map[*Union]*_TypeSet)
			xset := computeUnionTypeSet(nil, unionSets, nopos, x)
			yset := computeUnionTypeSet(nil, unionSets, nopos, y)
			return xset.terms.equal(yset.terms)
		}

	case *Interface:
		// Two interface types are identical if they describe the same type sets.
		// With the existing implementation restriction, this simplifies to:
		//
		// Two interface types are identical if they have the same set of methods with
		// the same names and identical function types, and if any type restrictions
		// are the same. Lower-case method names from different packages are always
		// different. The order of the methods is irrelevant.
		if y, ok := y.(*Interface); ok {
			xset := x.typeSet()
			yset := y.typeSet()
			if xset.comparable != yset.comparable {
				return false
			}
			if !xset.terms.equal(yset.terms) {
				return false
			}
			a := xset.methods
			b := yset.methods
			if len(a) == len(b) {
				// Interface types are the only types where cycles can occur
				// that are not "terminated" via named types; and such cycles
				// can only be created via method parameter types that are
				// anonymous interfaces (directly or indirectly) embedding
				// the current interface. Example:
				//
				//    type T interface {
				//        m() interface{T}
				//    }
				//
				// If two such (differently named) interfaces are compared,
				// endless recursion occurs if the cycle is not detected.
				//
				// If x and y were compared before, they must be equal
				// (if they were not, the recursion would have stopped);
				// search the ifacePair stack for the same pair.
				//
				// This is a quadratic algorithm, but in practice these stacks
				// are extremely short (bounded by the nesting depth of interface
				// type declarations that recur via parameter types, an extremely
				// rare occurrence). An alternative implementation might use a
				// "visited" map, but that is probably less efficient overall.
				q := &ifacePair{x, y, p}
				for p != nil {
					if p.identical(q) {
						return true // same pair was compared before
					}
					p = p.prev
				}
				if debug {
					assertSortedMethods(a)
					assertSortedMethods(b)
				}
				for i, f := range a {
					g := b[i]
					if f.Id() != g.Id() || !c.identical(f.typ, g.typ, q) {
						return false
					}
				}
				return true
			}
		}

	case *Map:
		// Two map types are identical if they have identical key and value types.
		if y, ok := y.(*Map); ok {
			return c.identical(x.key, y.key, p) && c.identical(x.elem, y.elem, p)
		}

	case *Chan:
		// Two channel types are identical if they have identical value types
		// and the same direction.
		if y, ok := y.(*Chan); ok {
			return x.dir == y.dir && c.identical(x.elem, y.elem, p)
		}

	case *Named:
		// Two named types are identical if their type names originate
		// in the same type declaration; if they are instantiated they
		// must have identical type argument lists.
		if y := asNamed(y); y != nil {
			// check type arguments before origins to match unifier
			// (for correct source code we need to do all checks so
			// order doesn't matter)
			xargs := x.TypeArgs().list()
			yargs := y.TypeArgs().list()
			if len(xargs) != len(yargs) {
				return false
			}
			for i, xarg := range xargs {
				if !Identical(xarg, yargs[i]) {
					return false
				}
			}
			return identicalOrigin(x, y)
		}

	case *TypeParam:
		// nothing to do (x and y being equal is caught in the very beginning of this function)

	case nil:
		// avoid a crash in case of nil type

	default:
		panic("unreachable")
	}

	return false
}

// identicalOrigin reports whether x and y originated in the same declaration.
func identicalOrigin(x, y *Named) bool {
	// TODO(gri) is this correct?
	return x.Origin().obj == y.Origin().obj
}

// identicalInstance reports if two type instantiations are identical.
// Instantiations are identical if their origin and type arguments are
// identical.
func identicalInstance(xorig Type, xargs []Type, yorig Type, yargs []Type) bool {
	if !slices.EqualFunc(xargs, yargs, Identical) {
		return false
	}

	return Identical(xorig, yorig)
}

// Default returns the default "typed" type for an "untyped" type;
// it returns the incoming type for all other types. The default type
// for untyped nil is untyped nil.
func Default(t Type) Type {
	// Alias and named types cannot denote untyped types
	// so there's no need to call Unalias or Underlying, below.
	if t, _ := t.(*Basic); t != nil {
		switch t.kind {
		case UntypedBool:
			return Typ[Bool]
		case UntypedInt:
			return Typ[Int]
		case UntypedRune:
			return universeRune // use 'rune' name
		case UntypedFloat:
			return Typ[Float64]
		case UntypedComplex:
			return Typ[Complex128]
		case UntypedString:
			return Typ[String]
		}
	}
	return t
}

// maxType returns the "largest" type that encompasses both x and y.
// If x and y are different untyped numeric types, the result is the type of x or y
// that appears later in this list: integer, rune, floating-point, complex.
// Otherwise, if x != y, the result is nil.
func maxType(x, y Type) Type {
	// We only care about untyped types (for now), so == is good enough.
	// TODO(gri) investigate generalizing this function to simplify code elsewhere
	if x == y {
		return x
	}
	if isUntypedNumeric(x) && isUntypedNumeric(y) {
		// untyped types are basic types
		if x.(*Basic).kind > y.(*Basic).kind {
			return x
		}
		return y
	}
	return nil
}

// clone makes a "flat copy" of *p and returns a pointer to the copy.
func clone[P *T, T any](p P) P {
	c := *p
	return &c
}

// isValidName reports whether s is a valid Go identifier.
func isValidName(s string) bool {
	for i, ch := range s {
		if !(unicode.IsLetter(ch) || ch == '_' || i > 0 && unicode.IsDigit(ch)) {
			return false
		}
	}
	return true
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/range.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of range statements.

package types2

import (
	"cmd/compile/internal/syntax"
	"go/constant"
	. "internal/types/errors"
)

// rangeStmt type-checks a range statement of form
//
//	for sKey, sValue = range rangeVar { ... }
//
// where sKey, sValue, sExtra may be nil. isDef indicates whether these
// variables are assigned to only (=) or whether there is a short variable
// declaration (:=). If the latter and there are no variables, an error is
// reported at noNewVarPos.
func (check *Checker) rangeStmt(inner stmtContext, rangeStmt *syntax.ForStmt, noNewVarPos poser, sKey, sValue, sExtra, rangeVar syntax.Expr, isDef bool) {
	// check expression to iterate over
	var x operand

	// From the spec:
	//   The range expression x is evaluated before beginning the loop,
	//   with one exception: if at most one iteration variable is present
	//   and x or len(x) is constant, the range expression is not evaluated.
	// So we have to be careful not to evaluate the arg in the
	// described situation.

	check.hasCallOrRecv = false
	check.expr(nil, &x, rangeVar)

	if isTypes2 && x.isValid() && sValue == nil && !check.hasCallOrRecv {
		if t, ok := arrayPtrDeref(x.typ().Underlying()).(*Array); ok {
			for {
				// Put constant info on the thing inside parentheses.
				// That's where (*../noder/writer).expr expects it.
				// See issue 73476.
				p, ok := rangeVar.(*syntax.ParenExpr)
				if !ok {
					break
				}
				rangeVar = p.X
			}
			// Override type of rangeVar to be a constant
			// (and thus side-effects will not be computed
			// by the backend).
			check.record(&operand{
				mode_: constant_,
				expr:  rangeVar,
				typ_:  Typ[Int],
				val:   constant.MakeInt64(t.len),
				id:    x.id,
			})
		}
	}

	// determine key/value types
	var key, val Type
	if x.isValid() {
		k, v, cause, ok := rangeKeyVal(check, x.typ(), func(v goVersion) bool {
			return check.allowVersion(v)
		})
		switch {
		case !ok && cause != "":
			check.softErrorf(&x, InvalidRangeExpr, "cannot range over %s: %s", &x, cause)
		case !ok:
			check.softErrorf(&x, InvalidRangeExpr, "cannot range over %s", &x)
		case k == nil && sKey != nil:
			check.softErrorf(sKey, InvalidIterVar, "range over %s permits no iteration variables", &x)
		case v == nil && sValue != nil:
			check.softErrorf(sValue, InvalidIterVar, "range over %s permits only one iteration variable", &x)
		case sExtra != nil:
			check.softErrorf(sExtra, InvalidIterVar, "range clause permits at most two iteration variables")
		}
		key, val = k, v
	}

	// Open the for-statement block scope now, after the range clause.
	// Iteration variables declared with := need to go in this scope (was go.dev/issue/51437).
	check.openScope(rangeStmt, "range")
	defer check.closeScope()

	// check assignment to/declaration of iteration variables
	// (irregular assignment, cannot easily map to existing assignment checks)

	// lhs expressions and initialization value (rhs) types
	lhs := [2]syntax.Expr{sKey, sValue} // sKey, sValue may be nil
	rhs := [2]Type{key, val}            // key, val may be nil

	rangeOverInt := isInteger(x.typ())

	if isDef {
		// short variable declaration
		var vars []*Var
		for i, lhs := range lhs {
			if lhs == nil {
				continue
			}

			// determine lhs variable
			var obj *Var
			if ident, _ := lhs.(*syntax.Name); ident != nil {
				// declare new variable
				name := ident.Value
				obj = newVar(LocalVar, ident.Pos(), check.pkg, name, nil)
				check.recordDef(ident, obj)
				// _ variables don't count as new variables
				if name != "_" {
					vars = append(vars, obj)
				}
			} else {
				check.errorf(lhs, InvalidSyntaxTree, "cannot declare %s", lhs)
				obj = newVar(LocalVar, lhs.Pos(), check.pkg, "_", nil) // dummy variable
			}
			assert(obj.typ == nil)

			// initialize lhs iteration variable, if any
			typ := rhs[i]
			if typ == nil || typ == Typ[Invalid] {
				// typ == Typ[Invalid] can happen if allowVersion fails.
				obj.typ = Typ[Invalid]
				check.usedVars[obj] = true // don't complain about unused variable
				continue
			}

			if rangeOverInt {
				assert(i == 0) // at most one iteration variable (rhs[1] == nil or Typ[Invalid] for rangeOverInt)
				check.initVar(obj, &x, "range clause")
			} else {
				var y operand
				y.mode_ = value
				y.expr = lhs // we don't have a better rhs expression to use here
				y.typ_ = typ
				check.initVar(obj, &y, "assignment") // error is on variable, use "assignment" not "range clause"
			}
			assert(obj.typ != nil)
		}

		// declare variables
		if len(vars) > 0 {
			scopePos := rangeStmt.Body.Pos()
			for _, obj := range vars {
				check.declare(check.scope, nil /* recordDef already called */, obj, scopePos)
			}
		} else {
			check.error(noNewVarPos, NoNewVar, "no new variables on left side of :=")
		}
	} else if sKey != nil /* lhs[0] != nil */ {
		// ordinary assignment
		for i, lhs := range lhs {
			if lhs == nil {
				continue
			}

			// assign to lhs iteration variable, if any
			typ := rhs[i]
			if typ == nil || typ == Typ[Invalid] {
				continue
			}

			if rangeOverInt {
				assert(i == 0) // at most one iteration variable (rhs[1] == nil or Typ[Invalid] for rangeOverInt)
				check.assignVar(lhs, nil, &x, "range clause")
				// If the assignment succeeded, if x was untyped before, it now
				// has a type inferred via the assignment. It must be an integer.
				// (go.dev/issues/67027)
				if x.isValid() && !isInteger(x.typ()) {
					check.softErrorf(lhs, InvalidRangeExpr, "cannot use iteration variable of type %s", x.typ())
				}
			} else {
				var y operand
				y.mode_ = value
				y.expr = lhs // we don't have a better rhs expression to use here
				y.typ_ = typ
				check.assignVar(lhs, nil, &y, "assignment") // error is on variable, use "assignment" not "range clause"
			}
		}
	} else if rangeOverInt {
		// If we don't have any iteration variables, we still need to
		// check that a (possibly untyped) integer range expression x
		// is valid.
		// We do this by checking the assignment _ = x. This ensures
		// that an untyped x can be converted to a value of its default
		// type (rune or int).
		check.assignment(&x, nil, "range clause")
	}

	check.stmt(inner, rangeStmt.Body)
}

// rangeKeyVal returns the key and value type produced by a range clause
// over an expression of type orig.
// If allowVersion != nil, it is used to check the required language version.
// If the range clause is not permitted, rangeKeyVal returns ok = false.
// When ok = false, rangeKeyVal may also return a reason in cause.
// The check parameter is only used in case of an error; it may be nil.
func rangeKeyVal(check *Checker, orig Type, allowVersion func(goVersion) bool) (key, val Type, cause string, ok bool) {
	bad := func(cause string) (Type, Type, string, bool) {
		return Typ[Invalid], Typ[Invalid], cause, false
	}

	rtyp, err := commonUnder(orig, func(t, u Type) *typeError {
		// A channel must permit receive operations.
		if ch, _ := u.(*Chan); ch != nil && ch.dir == SendOnly {
			return typeErrorf("receive from send-only channel %s", t)
		}
		return nil
	})
	if rtyp == nil {
		return bad(err.format(check))
	}

	switch typ := arrayPtrDeref(rtyp).(type) {
	case *Basic:
		if isString(typ) {
			return Typ[Int], universeRune, "", true // use 'rune' name
		}
		if isInteger(typ) {
			if allowVersion != nil && !allowVersion(go1_22) {
				return bad("requires go1.22 or later")
			}
			return orig, nil, "", true
		}
	case *Array:
		return Typ[Int], typ.elem, "", true
	case *Slice:
		return Typ[Int], typ.elem, "", true
	case *Map:
		return typ.key, typ.elem, "", true
	case *Chan:
		assert(typ.dir != SendOnly)
		return typ.elem, nil, "", true
	case *Signature:
		if allowVersion != nil && !allowVersion(go1_23) {
			return bad("requires go1.23 or later")
		}
		// check iterator arity
		// TODO(gri) error messages could be less verbose (consider rangeStmt error and cause returned here)
		switch {
		case typ.Params().Len() != 1:
			return bad("func must be func(yield func(...) bool): wrong argument count")
		case typ.Results().Len() != 0:
			return bad("func must be func(yield func(...) bool): unexpected results")
		}
		assert(typ.Recv() == nil)
		// check iterator argument type
		u, err := commonUnder(typ.Params().At(0).Type(), nil)
		cb, _ := u.(*Signature)
		switch {
		case cb == nil:
			if err != nil {
				return bad(check.sprintf("func must be func(yield func(...) bool): in yield type, %s", err.format(check)))
			} else {
				return bad("func must be func(yield func(...) bool): argument is not func")
			}
		case cb.Params().Len() > 2:
			return bad("func must be func(yield func(...) bool): yield func has too many parameters")
		case cb.Results().Len() != 1 || !Identical(cb.Results().At(0).Type(), universeBool):
			// see go.dev/issues/71131, go.dev/issues/71164
			if cb.Results().Len() == 1 && isBoolean(cb.Results().At(0).Type()) {
				return bad("func must be func(yield func(...) bool): yield func returns user-defined boolean, not bool")
			} else {
				return bad("func must be func(yield func(...) bool): yield func does not return bool")
			}
		case cb.Variadic():
			return bad(check.sprintf("yield func of type %s cannot be variadic", cb))
		}
		assert(cb.Recv() == nil)
		// determine key and value types, if any
		if cb.Params().Len() >= 1 {
			key = cb.Params().At(0).Type()
		}
		if cb.Params().Len() >= 2 {
			val = cb.Params().At(1).Type()
		}
		return key, val, "", true
	}
	return
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/README.md ===
```markdown
This file describes some of the typecheckers internal organization and conventions.
It is not meant to be complete; rather it is a living document that will be updated
as needed.

Read this file first before starting to make changes in the code.

#
### Overall organization

There are two almost identical typecheckers:

- cmd/compile/internal/types2 (or types2 for short)
- go/types

types2 is internal and used by the compiler.
go/types is the std library typechecker and its API must remain strictly
backward-compatible.
The types2 API closely matches the go/types API but may not have some
deprecated functions anymore (which we need to maintain in go/types).

They differ primarily in what syntax tree they operate on:

- types2 uses the syntax tree defined by cmd/compile/internal/syntax
- go/types uses the syntax tree defined by go/ast

We aim to keep the respective sources very closely in sync.
**Any change will need to be made to both typechecker source bases**.

Many go/types files can be generated automatically from the
corresponding types2 sources.
This is done via a generator (go/types/generate_test.go) which may be invoked via
`go generate` in the go/types directory.
Generated files are clearly marked with a comment at the top and should not
be modified by hand.
For this reason, it is usually best to make changes to the types2 sources first.
The changes only need to be ported by hand for the go/types files that cannot
be generated yet.

New files may be added to the list of generated files by adding a respective
entry to the table in generate_test.go (and possibly describing any necessary
source transformations).

In the following, examples and commands are based on types2 but usually apply
directly to go/types.


#
### Tests

There is a comprehensive suite of tests in the form of annotated source files.
The tests are in:

- src/internal/types/testdata/ (shared between go/types and types2)
- ./testdata/local (typechecker local tests, for rare situations only)

Tests are .go files annotated with `/* ERROR "msg" */` or `/* ERRORx "msg" */`
comments (or the respective line comment form).
For each such error comment, typechecking the respective file is expected to
report an error at the position of the syntactic token _immediately preceding_
the comment.
For `ERROR`, the `"msg"` string must be a substring of the error message
reported by the typechecker;
for `ERRORx`, the `"msg"` string must be a regular expresspion matching the
reported error.

For each issue #NNNN that is fixed in the typecheckers, a test
should be added as src/internal/types/testdata/fixedbugs/issueNNNN.go.


#
### Debugging

The pre-existing template ./testdata/manual.go is convenient for debugging
on-off situations. Simply populate it with the code of interest and then
run `go test -run Manual` which will typecheck that file.

Useful debugging flags (together with `go test -run Manual`):

- -halt (panic and produce a stack trace where the first error is reported)
- -v    (produce a typechecking trace)
- -verify       (verify `ERROR` comments in manual.go)


#
### Frequently used types and variables

#### Checker

File: check.go

A `Checker` maintains all typechecking state relevant for typechecking a package.
Typically the receiver type for typechecker methods.


#### operand

File: operand.go

An `operand` describes the type and value (if any) of an expression.
The `operandMode` describes the kind of expression (constant, variable, etc.).
Operands are the primary result of typechecking an expression.
If typechecking of an expression fails, the resulting operand has mode `invalid`.


#### Typ

File: universe.go

The `Typ` array provides access to all predeclared basic types.
`Typ[Invalid]` is used to denote an invalid type.


#
### Internal coding conventions

#### Predicates

File: predicates.go (commonly used predicates only)

Predicates are typically named in form `isX`, such as `isInteger`.

#### Type-checking expressions

Typically, there is a Checker method for typechecking a particular expression.
For instance, there is a method `Checker.unary` that typechecks unary expressions.
The basic form of such a function f is as follows:
```
func (check *Checker) f(x *operand, e syntax.Expr, /* addition arguments, if any */)
```
The result of typechecking expression `e` is returned via the operand `x`
(which sometimes also serves as incoming argument).
If an error occurred the function f will report the error and try to continue
as best as it can, but it may return an invalid operand (`x.mode == invalid`).
Callers may need to explicitly check for invalid operands.


#
### TODO

Add more relevant content.

```

// === FILE: references!/go/src/cmd/compile/internal/types2/recording.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements recording of type information
// in the types2.Info maps.

package types2

import (
	"cmd/compile/internal/syntax"
	"go/constant"
)

func (check *Checker) record(x *operand) {
	// convert x into a user-friendly set of values
	// TODO(gri) this code can be simplified
	var typ Type
	var val constant.Value
	switch x.mode() {
	case invalid:
		typ = Typ[Invalid]
	case novalue:
		typ = (*Tuple)(nil)
	case constant_:
		typ = x.typ()
		val = x.val
	default:
		typ = x.typ()
	}
	assert(x.expr != nil && typ != nil)

	if isUntyped(typ) {
		// delay type and value recording until we know the type
		// or until the end of type checking
		check.rememberUntyped(x.expr, false, x.mode(), typ.(*Basic), val)
	} else {
		check.recordTypeAndValue(x.expr, x.mode(), typ, val)
	}
}

func (check *Checker) recordUntyped() {
	if !debug && !check.recordTypes() {
		return // nothing to do
	}

	for x, info := range check.untyped {
		if debug && isTyped(info.typ) {
			check.dump("%v: %s (type %s) is typed", atPos(x), x, info.typ)
			panic("unreachable")
		}
		check.recordTypeAndValue(x, info.mode, info.typ, info.val)
	}
}

func (check *Checker) recordTypeAndValue(x syntax.Expr, mode operandMode, typ Type, val constant.Value) {
	assert(x != nil)
	assert(typ != nil)
	if mode == invalid {
		return // omit
	}
	if mode == constant_ {
		assert(val != nil)
		// We check allBasic(typ, IsConstType) here as constant expressions may be
		// recorded as type parameters.
		assert(!isValid(typ) || allBasic(typ, IsConstType))
	}
	if m := check.Types; m != nil {
		m[x] = TypeAndValue{mode, typ, val}
	}
	check.recordTypeAndValueInSyntax(x, mode, typ, val)
}

func (check *Checker) recordBuiltinType(f syntax.Expr, sig *Signature) {
	// f must be a (possibly parenthesized, possibly qualified)
	// identifier denoting a built-in (including unsafe's non-constant
	// functions Add and Slice): record the signature for f and possible
	// children.
	for {
		check.recordTypeAndValue(f, builtin, sig, nil)
		switch p := f.(type) {
		case *syntax.Name, *syntax.SelectorExpr:
			return // we're done
		case *syntax.ParenExpr:
			f = p.X
		default:
			panic("unreachable")
		}
	}
}

// recordCommaOkTypes updates recorded types to reflect that x is used in a commaOk context
// (and therefore has tuple type).
func (check *Checker) recordCommaOkTypes(x syntax.Expr, a []*operand) {
	assert(x != nil)
	assert(len(a) == 2)
	if a[0].mode() == invalid {
		return
	}
	t0, t1 := a[0].typ(), a[1].typ()
	assert(isTyped(t0) && isTyped(t1) && (allBoolean(t1) || t1 == universeError))
	if m := check.Types; m != nil {
		for {
			tv := m[x]
			assert(tv.Type != nil) // should have been recorded already
			pos := x.Pos()
			tv.Type = NewTuple(
				newVar(LocalVar, pos, check.pkg, "", t0),
				newVar(LocalVar, pos, check.pkg, "", t1),
			)
			m[x] = tv
			// if x is a parenthesized expression (p.X), update p.X
			p, _ := x.(*syntax.ParenExpr)
			if p == nil {
				break
			}
			x = p.X
		}
	}
	check.recordCommaOkTypesInSyntax(x, t0, t1)
}

// recordInstance records instantiation information into check.Info, if the
// Instances map is non-nil. The given expr must be an ident, selector, or
// index (list) expr with ident or selector operand.
//
// TODO(rfindley): the expr parameter is fragile. See if we can access the
// instantiated identifier in some other way.
func (check *Checker) recordInstance(expr syntax.Expr, targs []Type, typ Type) {
	ident := instantiatedIdent(expr)
	assert(ident != nil)
	assert(typ != nil)
	if m := check.Instances; m != nil {
		// If this is an instance of a method value/expression, replace the
		// receiver for the Signature stored in Instances (go.dev/issue/79657).
		if sig, _ := typ.(*Signature); sig != nil && sig.recvold != nil {
			copy := *sig
			copy.recvold = nil // not strictly necessary
			if sig.recvold == methodExprSentinel {
				pars := *copy.params
				copy.recv = pars.vars[0]
				pars.vars = pars.vars[1:]
				copy.params = &pars
				m[ident] = Instance{newTypeList(targs), &copy}
				return
			}
			copy.recv = sig.recvold
			m[ident] = Instance{newTypeList(targs), &copy}
			return
		}
		m[ident] = Instance{newTypeList(targs), typ}
	}
}

func (check *Checker) recordDef(id *syntax.Name, obj Object) {
	assert(id != nil)
	if m := check.Defs; m != nil {
		m[id] = obj
	}
}

func (check *Checker) recordUse(id *syntax.Name, obj Object) {
	assert(id != nil)
	assert(obj != nil)
	if m := check.Uses; m != nil {
		m[id] = obj
	}
}

func (check *Checker) recordImplicit(node syntax.Node, obj Object) {
	assert(node != nil)
	assert(obj != nil)
	if m := check.Implicits; m != nil {
		m[node] = obj
	}
}

func (check *Checker) recordSelection(x *syntax.SelectorExpr, kind SelectionKind, recv Type, obj Object, index []int, indirect bool) {
	assert(obj != nil && (recv == nil || len(index) > 0))
	check.recordUse(x.Sel, obj)
	if m := check.Selections; m != nil {
		m[x] = &Selection{kind, recv, obj, index, indirect}
	}
}

func (check *Checker) recordScope(node syntax.Node, scope *Scope) {
	assert(node != nil)
	assert(scope != nil)
	if m := check.Scopes; m != nil {
		m[node] = scope
	}
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/resolver.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	"cmp"
	"fmt"
	"go/constant"
	. "internal/types/errors"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// A declInfo describes a package-level const, type, var, or func declaration.
type declInfo struct {
	file      *Scope           // scope of file containing this declaration
	version   goVersion        // Go version of file containing this declaration
	lhs       []*Var           // lhs of n:1 variable declarations, or nil
	vtyp      syntax.Expr      // type, or nil (for const and var declarations only)
	init      syntax.Expr      // init/orig expression, or nil (for const and var declarations only)
	inherited bool             // if set, the init expression is inherited from a previous constant declaration
	tdecl     *syntax.TypeDecl // type declaration, or nil
	fdecl     *syntax.FuncDecl // func declaration, or nil

	// The deps field tracks initialization expression dependencies.
	deps map[Object]bool // lazily initialized
}

// hasInitializer reports whether the declared object has an initialization
// expression or function body.
func (d *declInfo) hasInitializer() bool {
	return d.init != nil || d.fdecl != nil && d.fdecl.Body != nil
}

// addDep adds obj to the set of objects d's init expression depends on.
func (d *declInfo) addDep(obj Object) {
	m := d.deps
	if m == nil {
		m = make(map[Object]bool)
		d.deps = m
	}
	m[obj] = true
}

// arity checks that the lhs and rhs of a const or var decl
// have a matching number of names and initialization values.
// If inherited is set, the initialization values are from
// another (constant) declaration.
func (check *Checker) arity(pos syntax.Pos, names []*syntax.Name, inits []syntax.Expr, constDecl, inherited bool) {
	l := len(names)
	r := len(inits)

	const code = WrongAssignCount
	switch {
	case l < r:
		n := inits[l]
		if inherited {
			check.errorf(pos, code, "extra init expr at %s", n.Pos())
		} else {
			check.errorf(n, code, "extra init expr %s", n)
		}
	case l > r && (constDecl || r != 1): // if r == 1 it may be a multi-valued function and we can't say anything yet
		n := names[r]
		check.errorf(n, code, "missing init expr for %s", n.Value)
	}
}

func validatedImportPath(path string) (string, error) {
	s, err := strconv.Unquote(path)
	if err != nil {
		return "", err
	}
	if s == "" {
		return "", fmt.Errorf("empty string")
	}
	const illegalChars = `!"#$%&'()*,:;<=>?[\]^{|}` + "`\uFFFD"
	for _, r := range s {
		if !unicode.IsGraphic(r) || unicode.IsSpace(r) || strings.ContainsRune(illegalChars, r) {
			return s, fmt.Errorf("invalid character %#U", r)
		}
	}
	return s, nil
}

// declarePkgObj declares obj in the package scope, records its ident -> obj mapping,
// and updates check.objMap. The object must not be a function or method.
func (check *Checker) declarePkgObj(ident *syntax.Name, obj Object, d *declInfo) {
	assert(ident.Value == obj.Name())

	// spec: "A package-scope or file-scope identifier with name init
	// may only be declared to be a function with this (func()) signature."
	if ident.Value == "init" {
		check.error(ident, InvalidInitDecl, "cannot declare init - must be func")
		return
	}

	// spec: "The main package must have package name main and declare
	// a function main that takes no arguments and returns no value."
	if ident.Value == "main" && check.pkg.name == "main" {
		check.error(ident, InvalidMainDecl, "cannot declare main - must be func")
		return
	}

	check.declare(check.pkg.scope, ident, obj, nopos)
	check.objMap[obj] = d
	obj.setOrder(uint32(len(check.objMap)))
}

// filename returns a filename suitable for debugging output.
func (check *Checker) filename(fileNo int) string {
	file := check.files[fileNo]
	if pos := file.Pos(); pos.IsKnown() {
		// return check.fset.File(pos).Name()
		// TODO(gri) do we need the actual file name here?
		return pos.RelFilename()
	}
	return fmt.Sprintf("file[%d]", fileNo)
}

func (check *Checker) importPackage(pos syntax.Pos, path, dir string) *Package {
	// If we already have a package for the given (path, dir)
	// pair, use it instead of doing a full import.
	// Checker.impMap only caches packages that are marked Complete
	// or fake (dummy packages for failed imports). Incomplete but
	// non-fake packages do require an import to complete them.
	key := importKey{path, dir}
	imp := check.impMap[key]
	if imp != nil {
		return imp
	}

	// no package yet => import it
	if path == "C" && (check.conf.FakeImportC || check.conf.go115UsesCgo) {
		if check.conf.FakeImportC && check.conf.go115UsesCgo {
			check.error(pos, BadImportPath, "cannot use FakeImportC and go115UsesCgo together")
		}
		imp = NewPackage("C", "C")
		imp.fake = true // package scope is not populated
		imp.cgo = check.conf.go115UsesCgo
	} else {
		// ordinary import
		var err error
		if importer := check.conf.Importer; importer == nil {
			err = fmt.Errorf("Config.Importer not installed")
		} else if importerFrom, ok := importer.(ImporterFrom); ok {
			imp, err = importerFrom.ImportFrom(path, dir, 0)
			if imp == nil && err == nil {
				err = fmt.Errorf("Config.Importer.ImportFrom(%s, %s, 0) returned nil but no error", path, dir)
			}
		} else {
			imp, err = importer.Import(path)
			if imp == nil && err == nil {
				err = fmt.Errorf("Config.Importer.Import(%s) returned nil but no error", path)
			}
		}
		// make sure we have a valid package name
		// (errors here can only happen through manipulation of packages after creation)
		if err == nil && imp != nil && (imp.name == "_" || imp.name == "") {
			err = fmt.Errorf("invalid package name: %q", imp.name)
			imp = nil // create fake package below
		}
		if err != nil {
			check.errorf(pos, BrokenImport, "could not import %s (%s)", path, err)
			if imp == nil {
				// create a new fake package
				// come up with a sensible package name (heuristic)
				name := strings.TrimSuffix(path, "/")
				if i := strings.LastIndex(name, "/"); i >= 0 {
					name = name[i+1:]
				}
				imp = NewPackage(path, name)
			}
			// continue to use the package as best as we can
			imp.fake = true // avoid follow-up lookup failures
		}
	}

	// package should be complete or marked fake, but be cautious
	if imp.complete || imp.fake {
		check.impMap[key] = imp
		// Once we've formatted an error message, keep the pkgPathMap
		// up-to-date on subsequent imports. It is used for package
		// qualification in error messages.
		if check.pkgPathMap != nil {
			check.markImports(imp)
		}
		return imp
	}

	// something went wrong (importer may have returned incomplete package without error)
	return nil
}

// collectObjects collects all file and package objects and inserts them
// into their respective scopes. It also performs imports and associates
// methods with receiver base type names.
func (check *Checker) collectObjects() {
	pkg := check.pkg

	// pkgImports is the set of packages already imported by any package file seen
	// so far. Used to avoid duplicate entries in pkg.imports. Allocate and populate
	// it (pkg.imports may not be empty if we are checking test files incrementally).
	// Note that pkgImports is keyed by package (and thus package path), not by an
	// importKey value. Two different importKey values may map to the same package
	// which is why we cannot use the check.impMap here.
	var pkgImports = make(map[*Package]bool)
	for _, imp := range pkg.imports {
		pkgImports[imp] = true
	}

	type methodInfo struct {
		obj  *Func        // method
		ptr  bool         // true if pointer receiver
		recv *syntax.Name // receiver type name
	}
	var methods []methodInfo // collected methods with valid receivers and non-blank _ names

	fileScopes := make([]*Scope, len(check.files)) // fileScopes[i] corresponds to check.files[i]
	for fileNo, file := range check.files {
		check.version = asGoVersion(check.versions[file.Pos().FileBase()])

		// The package identifier denotes the current package,
		// but there is no corresponding package object.
		check.recordDef(file.PkgName, nil)

		fileScope := NewScope(pkg.scope, syntax.StartPos(file), syntax.EndPos(file), check.filename(fileNo))
		fileScopes[fileNo] = fileScope
		check.recordScope(file, fileScope)

		// determine file directory, necessary to resolve imports
		// FileName may be "" (typically for tests) in which case
		// we get "." as the directory which is what we would want.
		fileDir := dir(file.PkgName.Pos().RelFilename()) // TODO(gri) should this be filename?

		first := -1                // index of first ConstDecl in the current group, or -1
		var last *syntax.ConstDecl // last ConstDecl with init expressions, or nil
		for index, decl := range file.DeclList {
			if _, ok := decl.(*syntax.ConstDecl); !ok {
				first = -1 // we're not in a constant declaration
			}

			switch s := decl.(type) {
			case *syntax.ImportDecl:
				// import package
				if s.Path == nil || s.Path.Bad {
					continue // error reported during parsing
				}
				path, err := validatedImportPath(s.Path.Value)
				if err != nil {
					check.errorf(s.Path, BadImportPath, "invalid import path (%s)", err)
					continue
				}

				imp := check.importPackage(s.Path.Pos(), path, fileDir)
				if imp == nil {
					continue
				}

				// local name overrides imported package name
				name := imp.name
				if s.LocalPkgName != nil {
					name = s.LocalPkgName.Value
					if path == "C" {
						// match 1.17 cmd/compile (not prescribed by spec)
						check.error(s.LocalPkgName, ImportCRenamed, `cannot rename import "C"`)
						continue
					}
				}

				if name == "init" {
					check.error(s, InvalidInitDecl, "cannot import package as init - init must be a func")
					continue
				}

				// add package to list of explicit imports
				// (this functionality is provided as a convenience
				// for clients; it is not needed for type-checking)
				if !pkgImports[imp] {
					pkgImports[imp] = true
					pkg.imports = append(pkg.imports, imp)
				}

				pkgName := NewPkgName(s.Pos(), pkg, name, imp)
				if s.LocalPkgName != nil {
					// in a dot-import, the dot represents the package
					check.recordDef(s.LocalPkgName, pkgName)
				} else {
					check.recordImplicit(s, pkgName)
				}

				if imp.fake {
					// match 1.17 cmd/compile (not prescribed by spec)
					check.usedPkgNames[pkgName] = true
				}

				// add import to file scope
				check.imports = append(check.imports, pkgName)
				if name == "." {
					// dot-import
					if check.dotImportMap == nil {
						check.dotImportMap = make(map[dotImportKey]*PkgName)
					}
					// merge imported scope with file scope
					for name, obj := range imp.scope.elems {
						// Note: Avoid eager resolve(name, obj) here, so we only
						// resolve dot-imported objects as needed.

						// A package scope may contain non-exported objects,
						// do not import them!
						if isExported(name) {
							// declare dot-imported object
							// (Do not use check.declare because it modifies the object
							// via Object.setScopePos, which leads to a race condition;
							// the object may be imported into more than one file scope
							// concurrently. See go.dev/issue/32154.)
							if alt := fileScope.Lookup(name); alt != nil {
								err := check.newError(DuplicateDecl)
								err.addf(s.LocalPkgName, "%s redeclared in this block", alt.Name())
								err.addAltDecl(alt)
								err.report()
							} else {
								fileScope.insert(name, obj)
								check.dotImportMap[dotImportKey{fileScope, name}] = pkgName
							}
						}
					}
				} else {
					// declare imported package object in file scope
					// (no need to provide s.LocalPkgName since we called check.recordDef earlier)
					check.declare(fileScope, nil, pkgName, nopos)
				}

			case *syntax.ConstDecl:
				// iota is the index of the current constDecl within the group
				if first < 0 || s.Group == nil || file.DeclList[index-1].(*syntax.ConstDecl).Group != s.Group {
					first = index
					last = nil
				}
				iota := constant.MakeInt64(int64(index - first))

				// determine which initialization expressions to use
				inherited := true
				switch {
				case s.Type != nil || s.Values != nil:
					last = s
					inherited = false
				case last == nil:
					last = new(syntax.ConstDecl) // make sure last exists
					inherited = false
				}

				// declare all constants
				values := syntax.UnpackListExpr(last.Values)
				for i, name := range s.NameList {
					obj := NewConst(name.Pos(), pkg, name.Value, nil, iota)

					var init syntax.Expr
					if i < len(values) {
						init = values[i]
					}

					d := &declInfo{file: fileScope, version: check.version, vtyp: last.Type, init: init, inherited: inherited}
					check.declarePkgObj(name, obj, d)
				}

				// Constants must always have init values.
				check.arity(s.Pos(), s.NameList, values, true, inherited)

			case *syntax.VarDecl:
				lhs := make([]*Var, len(s.NameList))
				// If there's exactly one rhs initializer, use
				// the same declInfo d1 for all lhs variables
				// so that each lhs variable depends on the same
				// rhs initializer (n:1 var declaration).
				var d1 *declInfo
				if _, ok := s.Values.(*syntax.ListExpr); !ok {
					// The lhs elements are only set up after the for loop below,
					// but that's ok because declarePkgObj only collects the declInfo
					// for a later phase.
					d1 = &declInfo{file: fileScope, version: check.version, lhs: lhs, vtyp: s.Type, init: s.Values}
				}

				// declare all variables
				values := syntax.UnpackListExpr(s.Values)
				for i, name := range s.NameList {
					obj := newVar(PackageVar, name.Pos(), pkg, name.Value, nil)
					lhs[i] = obj

					d := d1
					if d == nil {
						// individual assignments
						var init syntax.Expr
						if i < len(values) {
							init = values[i]
						}
						d = &declInfo{file: fileScope, version: check.version, vtyp: s.Type, init: init}
					}

					check.declarePkgObj(name, obj, d)
				}

				// If we have no type, we must have values.
				if s.Type == nil || values != nil {
					check.arity(s.Pos(), s.NameList, values, false, false)
				}

			case *syntax.TypeDecl:
				obj := NewTypeName(s.Name.Pos(), pkg, s.Name.Value, nil)
				check.declarePkgObj(s.Name, obj, &declInfo{file: fileScope, version: check.version, tdecl: s})

			case *syntax.FuncDecl:
				name := s.Name.Value
				obj := NewFunc(s.Name.Pos(), pkg, name, nil) // signature set later
				var tparam0 *syntax.Field
				if len(s.TParamList) > 0 {
					tparam0 = s.TParamList[0]
				}
				if s.Recv == nil {
					// regular function
					if name == "init" || name == "main" && pkg.name == "main" {
						// init and main functions must not declare type and ordinary parameters or results
						code := InvalidInitDecl
						if name == "main" {
							code = InvalidMainDecl
						}
						if tparam0 != nil {
							check.softErrorf(tparam0, code, "func %s must have no type parameters", name)
						}
						if t := s.Type; len(t.ParamList) != 0 || len(t.ResultList) != 0 {
							check.softErrorf(s.Name, code, "func %s must have no arguments and no return values", name)
						}
					} else {
						_ = tparam0 != nil && check.verifyVersionf(tparam0, go1_18, "type parameter")
					}
					// don't declare init functions in the package scope - they are invisible
					if name == "init" {
						obj.parent = pkg.scope
						check.recordDef(s.Name, obj)
						if s.Body == nil {
							check.softErrorf(obj.pos, MissingInitBody, "func init must have a body")
						}
					} else {
						check.declare(pkg.scope, s.Name, obj, nopos)
					}
				} else {
					// method
					// d.Recv != nil
					ptr, base, _ := check.unpackRecv(s.Recv.Type, false)
					// Methods with invalid receiver cannot be associated to a type, and
					// methods with blank _ names are never found; no need to collect any
					// of them. They will still be type-checked with all the other functions.
					if recv, _ := base.(*syntax.Name); recv != nil && name != "_" {
						methods = append(methods, methodInfo{obj, ptr, recv})
					}
					_ = tparam0 != nil && check.verifyVersionf(tparam0, go1_27, "generic method")
					check.recordDef(s.Name, obj)
				}
				info := &declInfo{file: fileScope, version: check.version, fdecl: s}
				// Methods are not package-level objects but we still track them in the
				// object map so that we can handle them like regular functions (if the
				// receiver is invalid); also we need their fdecl info when associating
				// them with their receiver base type, below.
				check.objMap[obj] = info
				obj.setOrder(uint32(len(check.objMap)))

			default:
				check.errorf(s, InvalidSyntaxTree, "unknown syntax.Decl node %T", s)
			}
		}
	}

	// verify that objects in package and file scopes have different names
	for _, scope := range fileScopes {
		for name, obj := range scope.elems {
			if alt := pkg.scope.Lookup(name); alt != nil {
				obj = resolve(name, obj)
				err := check.newError(DuplicateDecl)
				if pkg, ok := obj.(*PkgName); ok {
					err.addf(alt, "%s already declared through import of %s", alt.Name(), pkg.Imported())
					err.addAltDecl(pkg)
				} else {
					err.addf(alt, "%s already declared through dot-import of %s", alt.Name(), obj.Pkg())
					// TODO(gri) dot-imported objects don't have a position; addAltDecl won't print anything
					err.addAltDecl(obj)
				}
				err.report()
			}
		}
	}

	// Now that we have all package scope objects and all methods,
	// associate methods with receiver base type name where possible.
	// Ignore methods that have an invalid receiver. They will be
	// type-checked later, with regular functions.
	if methods == nil {
		return
	}

	check.methods = make(map[*TypeName][]*Func)
	for i := range methods {
		m := &methods[i]
		// Determine the receiver base type and associate m with it.
		ptr, base := check.resolveBaseTypeName(m.ptr, m.recv)
		if base != nil {
			m.obj.hasPtrRecv_ = ptr
			check.methods[base] = append(check.methods[base], m.obj)
		}
	}
}

// sortObjects sorts package-level objects by source-order for reproducible processing
func (check *Checker) sortObjects() {
	check.objList = make([]Object, len(check.objMap))
	i := 0
	for obj := range check.objMap {
		check.objList[i] = obj
		i++
	}
	slices.SortFunc(check.objList, func(a, b Object) int {
		return cmp.Compare(a.order(), b.order())
	})
}

// unpackRecv unpacks a receiver type expression and returns its components: ptr indicates
// whether rtyp is a pointer receiver, base is the receiver base type expression stripped
// of its type parameters (if any), and tparams are its type parameter names, if any. The
// type parameters are only unpacked if unpackParams is set. For instance, given the rtyp
//
//	*T[A, _]
//
// ptr is true, base is T, and tparams is [A, _] (assuming unpackParams is set).
// Note that base may not be a *syntax.Name for erroneous programs.
func (check *Checker) unpackRecv(rtyp syntax.Expr, unpackParams bool) (ptr bool, base syntax.Expr, tparams []*syntax.Name) {
	// unpack receiver type
	base = syntax.Unparen(rtyp)
	if t, _ := base.(*syntax.Operation); t != nil && t.Op == syntax.Mul && t.Y == nil {
		ptr = true
		base = syntax.Unparen(t.X)
	}

	// unpack type parameters, if any
	if ptyp, _ := base.(*syntax.IndexExpr); ptyp != nil {
		base = ptyp.X
		if unpackParams {
			for _, arg := range syntax.UnpackListExpr(ptyp.Index) {
				var par *syntax.Name
				switch arg := arg.(type) {
				case *syntax.Name:
					par = arg
				case *syntax.BadExpr:
					// ignore - error already reported by parser
				case nil:
					check.error(ptyp, InvalidSyntaxTree, "parameterized receiver contains nil parameters")
				default:
					check.errorf(arg, BadDecl, "receiver type parameter %s must be an identifier", arg)
				}
				if par == nil {
					par = syntax.NewName(arg.Pos(), "_")
				}
				tparams = append(tparams, par)
			}

		}
	}

	return
}

// resolveBaseTypeName returns the non-alias base type name for the given name, and whether
// there was a pointer indirection to get to it. The base type name must be declared
// in package scope, and there can be at most one pointer indirection. Traversals
// through generic alias types are not permitted. If no such type name exists, the
// returned base is nil.
func (check *Checker) resolveBaseTypeName(ptr bool, name *syntax.Name) (ptr_ bool, base *TypeName) {
	// Algorithm: Starting from name, which is expected to denote a type,
	// we follow that type through non-generic alias declarations until
	// we reach a non-alias type name.
	var seen map[*TypeName]bool
	for name != nil {
		// name must denote an object found in the current package scope
		// (note that dot-imported objects are not in the package scope!)
		obj := check.pkg.scope.Lookup(name.Value)
		if obj == nil {
			break
		}

		// the object must be a type name...
		tname, _ := obj.(*TypeName)
		if tname == nil {
			break
		}

		// ... which we have not seen before
		if seen[tname] {
			break
		}

		// we're done if tdecl describes a defined type (not an alias)
		tdecl := check.objMap[tname].tdecl // must exist for objects in package scope
		if !tdecl.Alias {
			return ptr, tname
		}

		// an alias must not be generic
		// (importantly, we must not collect such methods - was https://go.dev/issue/70417)
		if tdecl.TParamList != nil {
			break
		}

		// otherwise, remember this type name and continue resolving
		if seen == nil {
			seen = make(map[*TypeName]bool)
		}
		seen[tname] = true

		// The syntax parser strips unnecessary parentheses; call Unparen for consistency with go/types.
		typ := syntax.Unparen(tdecl.Type)

		// dereference a pointer type
		if pexpr, _ := typ.(*syntax.Operation); pexpr != nil && pexpr.Op == syntax.Mul && pexpr.Y == nil {
			// if we've already seen a pointer, we're done
			if ptr {
				break
			}
			ptr = true
			typ = syntax.Unparen(pexpr.X) // continue with pointer base type
		}

		// After dereferencing, typ must be a locally defined type name.
		// Referring to other packages (qualified identifiers) or going
		// through instantiated types (index expressions) is not permitted,
		// so we can ignore those.
		name, _ = typ.(*syntax.Name)
	}

	// no base type found
	return false, nil
}

// packageObjects typechecks all package objects, but not function bodies.
func (check *Checker) packageObjects() {
	// add new methods to already type-checked types (from a prior Checker.Files call)
	for _, obj := range check.objList {
		if obj, _ := obj.(*TypeName); obj != nil && obj.typ != nil {
			check.collectMethods(obj)
		}
	}

	if false {
		// TODO: determine if we can enable this code now or
		//       if there are still problems with cycles and
		//       aliases.
		//
		// For example, in GOROOT/test/typeparam/issue50259.go,
		//
		// 	type T[_ any] struct{}
		// 	type A T[B]
		// 	type B = T[A]
		//
		// TypeName A has Type Named during checking, but by
		// the time the unified export data is written out,
		// its Type is Invalid.
		//
		// Investigate and reenable this branch.
		for _, obj := range check.objList {
			check.objDecl(obj)
		}
	} else {
		// To avoid problems with cycles, process non-alias type declarations first, followed by
		// alias declarations, and then everything else. This appears to avoid most situations
		// where the type of an alias is needed before it is available.
		// There may still be cases where this is not good enough (see also go.dev/issue/25838).
		// In those cases Checker.ident will report an error ("invalid use of type alias").
		var aliasList []*TypeName
		var othersList []Object // everything that's not a type
		// phase 1: non-alias type declarations
		for _, obj := range check.objList {
			if tname, _ := obj.(*TypeName); tname != nil {
				if check.objMap[tname].tdecl.Alias {
					aliasList = append(aliasList, tname)
				} else {
					check.objDecl(obj)
				}
			} else {
				othersList = append(othersList, obj)
			}
		}
		// phase 2: alias type declarations
		for _, obj := range aliasList {
			check.objDecl(obj)
		}
		// phase 3: all other declarations
		for _, obj := range othersList {
			check.objDecl(obj)
		}
	}

	// At this point we may have a non-empty check.methods map; this means that not all
	// entries were deleted at the end of typeDecl because the respective receiver base
	// types were not found. In that case, an error was reported when declaring those
	// methods. We can now safely discard this map.
	check.methods = nil
}

// unusedImports checks for unused imports.
func (check *Checker) unusedImports() {
	// If function bodies are not checked, packages' uses are likely missing - don't check.
	if check.conf.IgnoreFuncBodies {
		return
	}

	// spec: "It is illegal (...) to directly import a package without referring to
	// any of its exported identifiers. To import a package solely for its side-effects
	// (initialization), use the blank identifier as explicit package name."

	for _, obj := range check.imports {
		if obj.name != "_" && !check.usedPkgNames[obj] {
			check.errorUnusedPkg(obj)
		}
	}
}

func (check *Checker) errorUnusedPkg(obj *PkgName) {
	// If the package was imported with a name other than the final
	// import path element, show it explicitly in the error message.
	// Note that this handles both renamed imports and imports of
	// packages containing unconventional package declarations.
	// Note that this uses / always, even on Windows, because Go import
	// paths always use forward slashes.
	path := obj.imported.path
	elem := path
	if i := strings.LastIndex(elem, "/"); i >= 0 {
		elem = elem[i+1:]
	}
	if obj.name == "" || obj.name == "." || obj.name == elem {
		check.softErrorf(obj, UnusedImport, "%q imported and not used", path)
	} else {
		check.softErrorf(obj, UnusedImport, "%q imported as %s and not used", path, obj.name)
	}
}

// dir makes a good-faith attempt to return the directory
// portion of path. If path is empty, the result is ".".
// (Per the go/build package dependency tests, we cannot import
// path/filepath and simply use filepath.Dir.)
func dir(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i > 0 {
		return path[:i]
	}
	// i <= 0
	return "."
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/return.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements isTerminating.

package types2

import (
	"cmd/compile/internal/syntax"
)

// isTerminating reports if s is a terminating statement.
// If s is labeled, label is the label name; otherwise s
// is "".
func (check *Checker) isTerminating(s syntax.Stmt, label string) bool {
	switch s := s.(type) {
	default:
		panic("unreachable")

	case *syntax.DeclStmt, *syntax.EmptyStmt, *syntax.SendStmt,
		*syntax.AssignStmt, *syntax.CallStmt:
		// no chance

	case *syntax.LabeledStmt:
		return check.isTerminating(s.Stmt, s.Label.Value)

	case *syntax.ExprStmt:
		// calling the predeclared (possibly parenthesized) panic() function is terminating
		if call, ok := syntax.Unparen(s.X).(*syntax.CallExpr); ok && check.isPanic[call] {
			return true
		}

	case *syntax.ReturnStmt:
		return true

	case *syntax.BranchStmt:
		if s.Tok == syntax.Goto || s.Tok == syntax.Fallthrough {
			return true
		}

	case *syntax.BlockStmt:
		return check.isTerminatingList(s.List, "")

	case *syntax.IfStmt:
		if s.Else != nil &&
			check.isTerminating(s.Then, "") &&
			check.isTerminating(s.Else, "") {
			return true
		}

	case *syntax.SwitchStmt:
		return check.isTerminatingSwitch(s.Body, label)

	case *syntax.SelectStmt:
		for _, cc := range s.Body {
			if !check.isTerminatingList(cc.Body, "") || hasBreakList(cc.Body, label, true) {
				return false
			}

		}
		return true

	case *syntax.ForStmt:
		if _, ok := s.Init.(*syntax.RangeClause); ok {
			// Range clauses guarantee that the loop terminates,
			// so the loop is not a terminating statement. See go.dev/issue/49003.
			break
		}
		if s.Cond == nil && !hasBreak(s.Body, label, true) {
			return true
		}
	}

	return false
}

func (check *Checker) isTerminatingList(list []syntax.Stmt, label string) bool {
	// trailing empty statements are permitted - skip them
	for i := len(list) - 1; i >= 0; i-- {
		if _, ok := list[i].(*syntax.EmptyStmt); !ok {
			return check.isTerminating(list[i], label)
		}
	}
	return false // all statements are empty
}

func (check *Checker) isTerminatingSwitch(body []*syntax.CaseClause, label string) bool {
	hasDefault := false
	for _, cc := range body {
		if cc.Cases == nil {
			hasDefault = true
		}
		if !check.isTerminatingList(cc.Body, "") || hasBreakList(cc.Body, label, true) {
			return false
		}
	}
	return hasDefault
}

// TODO(gri) For nested breakable statements, the current implementation of hasBreak
// will traverse the same subtree repeatedly, once for each label. Replace
// with a single-pass label/break matching phase.

// hasBreak reports if s is or contains a break statement
// referring to the label-ed statement or implicit-ly the
// closest outer breakable statement.
func hasBreak(s syntax.Stmt, label string, implicit bool) bool {
	switch s := s.(type) {
	default:
		panic("unreachable")

	case *syntax.DeclStmt, *syntax.EmptyStmt, *syntax.ExprStmt,
		*syntax.SendStmt, *syntax.AssignStmt, *syntax.CallStmt,
		*syntax.ReturnStmt:
		// no chance

	case *syntax.LabeledStmt:
		return hasBreak(s.Stmt, label, implicit)

	case *syntax.BranchStmt:
		if s.Tok == syntax.Break {
			if s.Label == nil {
				return implicit
			}
			if s.Label.Value == label {
				return true
			}
		}

	case *syntax.BlockStmt:
		return hasBreakList(s.List, label, implicit)

	case *syntax.IfStmt:
		if hasBreak(s.Then, label, implicit) ||
			s.Else != nil && hasBreak(s.Else, label, implicit) {
			return true
		}

	case *syntax.SwitchStmt:
		if label != "" && hasBreakCaseList(s.Body, label, false) {
			return true
		}

	case *syntax.SelectStmt:
		if label != "" && hasBreakCommList(s.Body, label, false) {
			return true
		}

	case *syntax.ForStmt:
		if label != "" && hasBreak(s.Body, label, false) {
			return true
		}
	}

	return false
}

func hasBreakList(list []syntax.Stmt, label string, implicit bool) bool {
	for _, s := range list {
		if hasBreak(s, label, implicit) {
			return true
		}
	}
	return false
}

func hasBreakCaseList(list []*syntax.CaseClause, label string, implicit bool) bool {
	for _, s := range list {
		if hasBreakList(s.Body, label, implicit) {
			return true
		}
	}
	return false
}

func hasBreakCommList(list []*syntax.CommClause, label string, implicit bool) bool {
	for _, s := range list {
		if hasBreakList(s.Body, label, implicit) {
			return true
		}
	}
	return false
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/scope.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements Scopes.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
)

// A Scope maintains a set of objects and links to its containing
// (parent) and contained (children) scopes. Objects may be inserted
// and looked up by name. The zero value for Scope is a ready-to-use
// empty scope.
type Scope struct {
	parent   *Scope
	children []*Scope
	number   int               // parent.children[number-1] is this scope; 0 if there is no parent
	elems    map[string]Object // lazily allocated
	pos, end syntax.Pos        // scope extent; may be invalid
	comment  string            // for debugging only
	isFunc   bool              // set if this is a function scope (internal use only)
}

// NewScope returns a new, empty scope contained in the given parent
// scope, if any. The comment is for debugging only.
func NewScope(parent *Scope, pos, end syntax.Pos, comment string) *Scope {
	s := &Scope{parent, nil, 0, nil, pos, end, comment, false}
	// don't add children to Universe scope!
	if parent != nil && parent != Universe {
		parent.children = append(parent.children, s)
		s.number = len(parent.children)
	}
	return s
}

// Parent returns the scope's containing (parent) scope.
func (s *Scope) Parent() *Scope { return s.parent }

// Len returns the number of scope elements.
func (s *Scope) Len() int { return len(s.elems) }

// Names returns the scope's element names in sorted order.
func (s *Scope) Names() []string {
	names := make([]string, len(s.elems))
	i := 0
	for name := range s.elems {
		names[i] = name
		i++
	}
	slices.Sort(names)
	return names
}

// NumChildren returns the number of scopes nested in s.
func (s *Scope) NumChildren() int { return len(s.children) }

// Child returns the i'th child scope for 0 <= i < NumChildren().
func (s *Scope) Child(i int) *Scope { return s.children[i] }

// Lookup returns the object in scope s with the given name if such an
// object exists; otherwise the result is nil.
func (s *Scope) Lookup(name string) Object { return resolve(name, s.elems[name]) }

// lookupIgnoringCase returns the objects in scope s whose names match
// the given name ignoring case. If exported is set, only exported names
// are returned.
func (s *Scope) lookupIgnoringCase(name string, exported bool) []Object {
	var matches []Object
	for _, n := range s.Names() {
		if (!exported || isExported(n)) && strings.EqualFold(n, name) {
			matches = append(matches, s.Lookup(n))
		}
	}
	return matches
}

// Insert attempts to insert an object obj into scope s.
// If s already contains an alternative object alt with
// the same name, Insert leaves s unchanged and returns alt.
// Otherwise it inserts obj, sets the object's parent scope
// if not already set, and returns nil.
func (s *Scope) Insert(obj Object) Object {
	name := obj.Name()
	if alt := s.Lookup(name); alt != nil {
		return alt
	}
	s.insert(name, obj)
	// TODO(gri) Can we always set the parent to s (or is there
	// a need to keep the original parent or some race condition)?
	// If we can, than we may not need environment.lookupScope
	// which is only there so that we get the correct scope for
	// marking "used" dot-imported packages.
	if obj.Parent() == nil {
		obj.setParent(s)
	}
	return nil
}

// InsertLazy is like Insert, but allows deferring construction of the
// inserted object until it's accessed with Lookup. The Object
// returned by resolve must have the same name as given to InsertLazy.
// If s already contains an alternative object with the same name,
// InsertLazy leaves s unchanged and returns false. Otherwise it
// records the binding and returns true. The object's parent scope
// will be set to s after resolve is called.
func (s *Scope) InsertLazy(name string, resolve func() Object) bool {
	if s.elems[name] != nil {
		return false
	}
	s.insert(name, &lazyObject{parent: s, resolve: resolve})
	return true
}

func (s *Scope) insert(name string, obj Object) {
	if s.elems == nil {
		s.elems = make(map[string]Object)
	}
	s.elems[name] = obj
}

// WriteTo writes a string representation of the scope to w,
// with the scope elements sorted by name.
// The level of indentation is controlled by n >= 0, with
// n == 0 for no indentation.
// If recurse is set, it also writes nested (children) scopes.
func (s *Scope) WriteTo(w io.Writer, n int, recurse bool) {
	const ind = ".  "
	indn := strings.Repeat(ind, n)

	fmt.Fprintf(w, "%s%s scope %p {\n", indn, s.comment, s)

	indn1 := indn + ind
	for _, name := range s.Names() {
		fmt.Fprintf(w, "%s%s\n", indn1, s.Lookup(name))
	}

	if recurse {
		for _, s := range s.children {
			s.WriteTo(w, n+1, recurse)
		}
	}

	fmt.Fprintf(w, "%s}\n", indn)
}

// String returns a string representation of the scope, for debugging.
func (s *Scope) String() string {
	var buf strings.Builder
	s.WriteTo(&buf, 0, false)
	return buf.String()
}

// A lazyObject represents an imported Object that has not been fully
// resolved yet by its importer.
type lazyObject struct {
	parent  *Scope
	resolve func() Object
	obj     Object
	once    sync.Once
}

// resolve returns the Object represented by obj, resolving lazy
// objects as appropriate.
func resolve(name string, obj Object) Object {
	if lazy, ok := obj.(*lazyObject); ok {
		lazy.once.Do(func() {
			obj := lazy.resolve()

			if _, ok := obj.(*lazyObject); ok {
				panic("recursive lazy object")
			}
			if obj.Name() != name {
				panic("lazy object has unexpected name")
			}

			if obj.Parent() == nil {
				obj.setParent(lazy.parent)
			}
			lazy.obj = obj
		})

		obj = lazy.obj
	}
	return obj
}

// stub implementations so *lazyObject implements Object and we can
// store them directly into Scope.elems.
func (*lazyObject) Parent() *Scope                     { panic("unreachable") }
func (*lazyObject) Pos() syntax.Pos                    { panic("unreachable") }
func (*lazyObject) Pkg() *Package                      { panic("unreachable") }
func (*lazyObject) Name() string                       { panic("unreachable") }
func (*lazyObject) Type() Type                         { panic("unreachable") }
func (*lazyObject) Exported() bool                     { panic("unreachable") }
func (*lazyObject) Id() string                         { panic("unreachable") }
func (*lazyObject) String() string                     { panic("unreachable") }
func (*lazyObject) order() uint32                      { panic("unreachable") }
func (*lazyObject) setType(Type)                       { panic("unreachable") }
func (*lazyObject) setOrder(uint32)                    { panic("unreachable") }
func (*lazyObject) setParent(*Scope)                   { panic("unreachable") }
func (*lazyObject) sameId(*Package, string, bool) bool { panic("unreachable") }
func (*lazyObject) scopePos() syntax.Pos               { panic("unreachable") }
func (*lazyObject) setScopePos(syntax.Pos)             { panic("unreachable") }

```

// === FILE: references!/go/src/cmd/compile/internal/types2/selection.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements Selections.

package types2

import (
	"bytes"
	"fmt"
)

// SelectionKind describes the kind of a selector expression x.f
// (excluding qualified identifiers).
//
// If x is a struct or *struct, a selector expression x.f may denote a
// sequence of selection operations x.a.b.c.f. The SelectionKind
// describes the kind of the final (explicit) operation; all the
// previous (implicit) operations are always field selections.
// Each element of Indices specifies an implicit field (a, b, c)
// by its index in the struct type of the field selection operand.
//
// For a FieldVal operation, the final selection refers to the field
// specified by Selection.Obj.
//
// For a MethodVal operation, the final selection refers to a method.
// If the "pointerness" of the method's declared receiver does not
// match that of the effective receiver after implicit field
// selection, then an & or * operation is implicitly applied to the
// receiver variable or value.
// So, x.f denotes (&x.a.b.c).f when f requires a pointer receiver but
// x.a.b.c is a non-pointer variable; and it denotes (*x.a.b.c).f when
// f requires a non-pointer receiver but x.a.b.c is a pointer value.
//
// All pointer indirections, whether due to implicit or explicit field
// selections or * operations inserted for "pointerness", panic if
// applied to a nil pointer, so a method call x.f() may panic even
// before the function call.
//
// By contrast, a MethodExpr operation T.f is essentially equivalent
// to a function literal of the form:
//
//	func(x T, args) (results) { return x.f(args) }
//
// Consequently, any implicit field selections and * operations
// inserted for "pointerness" are not evaluated until the function is
// called, so a T.f or (*T).f expression never panics.
type SelectionKind int

const (
	FieldVal   SelectionKind = iota // x.f is a struct field selector
	MethodVal                       // x.f is a method selector
	MethodExpr                      // x.f is a method expression
)

// A Selection describes a selector expression x.f.
// For the declarations:
//
//	type T struct{ x int; E }
//	type E struct{}
//	func (e E) m() {}
//	var p *T
//
// the following relations exist:
//
//	Selector    Kind          Recv    Obj    Type       Index     Indirect
//
//	p.x         FieldVal      T       x      int        {0}       true
//	p.m         MethodVal     *T      m      func()     {1, 0}    true
//	T.m         MethodExpr    T       m      func(T)    {1, 0}    false
type Selection struct {
	kind     SelectionKind
	recv     Type   // type of x
	obj      Object // object denoted by x.f
	index    []int  // path from x to x.f
	indirect bool   // set if there was any pointer indirection on the path
}

// Kind returns the selection kind.
func (s *Selection) Kind() SelectionKind { return s.kind }

// Recv returns the type of x in x.f.
func (s *Selection) Recv() Type { return s.recv }

// Obj returns the object denoted by x.f; a *Var for
// a field selection, and a *Func in all other cases.
func (s *Selection) Obj() Object { return s.obj }

// Type returns the type of x.f, which may be different from the type of f.
// See Selection for more information.
func (s *Selection) Type() Type {
	switch s.kind {
	case MethodVal:
		// TODO(mark) Align this with call.go if possible.
		// The type of x.f is a method with its receiver type set
		// to the type of x.
		sig := *s.obj.(*Func).typ.(*Signature)
		recv := *sig.recv
		recv.typ = s.recv
		sig.recv = &recv
		return &sig

	case MethodExpr:
		// The type of x.f is a function (without receiver)
		// and an additional first argument with the same type as x.
		// TODO(gri) Similar code is already in call.go - factor!
		// TODO(gri) Compute this eagerly to avoid allocations.
		sig := *s.obj.(*Func).typ.(*Signature)
		arg0 := *sig.recv
		sig.recvold = sig.recv // stash receiver (for consistency with call.go)
		sig.recv = nil
		arg0.typ = s.recv
		var params []*Var
		if sig.params != nil {
			params = sig.params.vars
		}
		sig.params = NewTuple(append([]*Var{&arg0}, params...)...)
		return &sig
	}

	// In all other cases, the type of x.f is the type of f.
	return s.obj.Type()
}

// Index describes the path from x to f in x.f.
// The last index entry is the field or method index of the type declaring f;
// either:
//
//  1. the list of declared methods of a named type; or
//  2. the list of methods of an interface type; or
//  3. the list of fields of a struct type.
//
// The earlier index entries are the indices of the embedded fields implicitly
// traversed to get from (the type of) x to f, starting at embedding depth 0.
func (s *Selection) Index() []int { return s.index }

// Indirect reports whether any pointer indirection was required to get from
// x to f in x.f.
//
// Beware: Indirect spuriously returns true (Go issue #8353) for a
// MethodVal selection in which the receiver argument and parameter
// both have type *T so there is no indirection.
// Unfortunately, a fix is too risky.
func (s *Selection) Indirect() bool { return s.indirect }

func (s *Selection) String() string { return SelectionString(s, nil) }

// SelectionString returns the string form of s.
// The Qualifier controls the printing of
// package-level objects, and may be nil.
//
// Examples:
//
//	"field (T) f int"
//	"method (T) f(X) Y"
//	"method expr (T) f(X) Y"
func SelectionString(s *Selection, qf Qualifier) string {
	var k string
	switch s.kind {
	case FieldVal:
		k = "field "
	case MethodVal:
		k = "method "
	case MethodExpr:
		k = "method expr "
	default:
		panic("unreachable")
	}
	var buf bytes.Buffer
	buf.WriteString(k)
	buf.WriteByte('(')
	WriteType(&buf, s.Recv(), qf)
	fmt.Fprintf(&buf, ") %s", s.obj.Name())
	if T := s.Type(); s.kind == FieldVal {
		buf.WriteByte(' ')
		WriteType(&buf, T, qf)
	} else {
		WriteSignature(&buf, T.(*Signature), qf)
	}
	return buf.String()
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/signature.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	. "internal/types/errors"
	"path/filepath"
	"strings"
)

// ----------------------------------------------------------------------------
// API

// A Signature represents a (non-builtin) function or method type.
// The receiver is ignored when comparing signatures for identity.
type Signature struct {
	// We need to keep the scope in Signature (rather than passing it around
	// and store it in the Func Object) because when type-checking a function
	// literal we call the general type checker which returns a general Type.
	// We then unpack the *Signature and use the scope for the literal body.
	rparams  *TypeParamList // receiver type parameters from left to right, or nil
	tparams  *TypeParamList // type parameters from left to right, or nil
	scope    *Scope         // function scope for package-local and non-instantiated signatures; nil otherwise
	recv     *Var           // nil if not a method
	recvold  *Var           // receiver dropped via method selection; or nil
	params   *Tuple         // (incoming) parameters from left to right; or nil
	results  *Tuple         // (outgoing) results from left to right; or nil
	variadic bool           // true if the last parameter's type is of the form ...T

	// If recvold is the sentinel value [methExpr], then recvold should
	// instead be sourced from params[0]. Otherwise, recvold points to
	// the receiver of the original method signature from which this
	// function signature was cloned via a selector expression.

	// If variadic, the last element of params ordinarily has an
	// unnamed Slice type. As a special case, in a call to append,
	// it may be string, or a TypeParam T whose typeset ⊇ {string, []byte}.
	// It may even be a named []byte type if a client instantiates
	// T at such a type.
}

// sentinel value for detecting method expressions
var methodExprSentinel = &Var{}

// NewSignatureType creates a new function type for the given receiver,
// receiver type parameters, type parameters, parameters, and results.
//
// If variadic is set, params must hold at least one parameter and the
// last parameter must be an unnamed slice or a type parameter whose
// type set has an unnamed slice as common underlying type.
//
// As a special case, to support append([]byte, str...), for variadic
// signatures the last parameter may also be a string type, or a type
// parameter containing a mix of byte slices and string types in its
// type set. It may even be a named []byte slice type resulting from
// substitution of such a type parameter.
//
// If recvTypeParams is non-empty, recv must be non-nil.
func NewSignatureType(recv *Var, recvTypeParams, typeParams []*TypeParam, params, results *Tuple, variadic bool) *Signature {
	if variadic {
		n := params.Len()
		if n == 0 {
			panic("variadic function must have at least one parameter")
		}
		last := params.At(n - 1).typ
		var S *Slice
		for t := range typeset(last) {
			if t == nil {
				break
			}
			var s *Slice
			if isString(t) {
				s = NewSlice(universeByte)
			} else {
				// Variadic Go functions have a last parameter of type []T,
				// suggesting we should reject a named slice type B here.
				//
				// However, a call to built-in append(slice, x...)
				// where x has a TypeParam type [T ~string | ~[]byte],
				// has the type func([]byte, T). Since a client may
				// instantiate this type at T=B, we must permit
				// named slice types, even when this results in a
				// signature func([]byte, B) where type B []byte.
				//
				// (The caller of NewSignatureType may have no way to
				// know that it is dealing with the append special case.)
				s, _ = t.Underlying().(*Slice)
			}
			if S == nil {
				S = s
			} else if s == nil || !Identical(S, s) {
				S = nil
				break
			}
		}
		if S == nil {
			panic(fmt.Sprintf("got %s, want variadic parameter of slice or string type", last))
		}
	}
	sig := &Signature{recv: recv, params: params, results: results, variadic: variadic}
	if len(recvTypeParams) != 0 {
		if recv == nil {
			panic("function with receiver type parameters must have a receiver")
		}
		sig.rparams = bindTParams(recvTypeParams)
	}
	if len(typeParams) != 0 {
		sig.tparams = bindTParams(typeParams)
	}
	return sig
}

// Recv returns the receiver of signature s (if a method), or nil if a
// function. It is ignored when comparing signatures for identity.
//
// For an abstract method, Recv returns the enclosing interface either
// as a *[Named] or an *[Interface]. Due to embedding, an interface may
// contain methods whose receiver type is a different interface.
func (s *Signature) Recv() *Var { return s.recv }

// TypeParams returns the type parameters of signature s, or nil.
func (s *Signature) TypeParams() *TypeParamList { return s.tparams }

// RecvTypeParams returns the receiver type parameters of signature s, or nil.
func (s *Signature) RecvTypeParams() *TypeParamList { return s.rparams }

// Params returns the parameters of signature s, or nil.
// See [NewSignatureType] for details of variadic functions.
func (s *Signature) Params() *Tuple { return s.params }

// Results returns the results of signature s, or nil.
func (s *Signature) Results() *Tuple { return s.results }

// Variadic reports whether the signature s is variadic.
func (s *Signature) Variadic() bool { return s.variadic }

func (s *Signature) Underlying() Type { return s }
func (s *Signature) String() string   { return TypeString(s, nil) }

// ----------------------------------------------------------------------------
// Implementation

// funcType type-checks a function or method type.
func (check *Checker) funcType(sig *Signature, recvPar *syntax.Field, tparams []*syntax.Field, ftyp *syntax.FuncType) {
	check.openScope(ftyp, "function")
	check.scope.isFunc = true
	check.recordScope(ftyp, check.scope)
	sig.scope = check.scope
	defer check.closeScope()

	// collect method receiver, if any
	var recv *Var
	var rparams *TypeParamList
	if recvPar != nil {
		// all type parameters' scopes start after the method name
		scopePos := ftyp.Pos()
		recv, rparams = check.collectRecv(recvPar, scopePos)
	}

	// collect and declare function type parameters
	if tparams != nil {
		check.collectTypeParams(&sig.tparams, tparams)
	}

	// collect ordinary and result parameters
	pnames, params, variadic := check.collectParams(ParamVar, ftyp.ParamList)
	rnames, results, _ := check.collectParams(ResultVar, ftyp.ResultList)

	// declare named receiver, ordinary, and result parameters
	scopePos := syntax.EndPos(ftyp) // all parameter's scopes start after the signature
	if recv != nil && recv.name != "" {
		check.declare(check.scope, recvPar.Name, recv, scopePos)
	}
	check.declareParams(pnames, params, scopePos)
	check.declareParams(rnames, results, scopePos)

	sig.recv = recv
	sig.rparams = rparams
	sig.params = NewTuple(params...)
	sig.results = NewTuple(results...)
	sig.variadic = variadic
}

// collectRecv extracts the method receiver and its type parameters (if any) from rparam.
// It declares the type parameters (but not the receiver) in the current scope, and
// returns the receiver variable and its type parameter list (if any).
func (check *Checker) collectRecv(rparam *syntax.Field, scopePos syntax.Pos) (*Var, *TypeParamList) {
	// Unpack the receiver parameter which is of the form
	//
	//	"(" [rname] ["*"] rbase ["[" rtparams "]"] ")"
	//
	// The receiver name rname, the pointer indirection, and the
	// receiver type parameters rtparams may not be present.
	rptr, rbase, rtparams := check.unpackRecv(rparam.Type, true)

	// Determine the receiver base type.
	var recvType Type = Typ[Invalid]
	var recvTParamsList *TypeParamList
	if rtparams == nil {
		// If there are no type parameters, we can simply typecheck rparam.Type.
		// If that is a generic type, varType will complain.
		// Further receiver constraints will be checked later, with validRecv.
		// We use rparam.Type (rather than base) to correctly record pointer
		// and parentheses in types2.Info (was bug, see go.dev/issue/68639).
		recvType = check.varType(rparam.Type)
		// Defining new methods on instantiated (alias or defined) types is not permitted.
		// Follow literal pointer/alias type chain and check.
		// (Correct code permits at most one pointer indirection, but for this check it
		// doesn't matter if we have multiple pointers.)
		a, _ := unpointer(recvType).(*Alias) // recvType is not generic per above
		for a != nil {
			baseType := unpointer(a.fromRHS)
			if g, _ := baseType.(genericType); g != nil && g.TypeParams() != nil {
				check.errorf(rbase, InvalidRecv, "cannot define new methods on instantiated type %s", g)
				recvType = Typ[Invalid] // avoid follow-on errors by Checker.validRecv
				break
			}
			a, _ = baseType.(*Alias)
		}
	} else {
		// If there are type parameters, rbase must denote a generic base type.
		// Important: rbase must be resolved before declaring any receiver type
		// parameters (which may have the same name, see below).
		var baseType *Named // nil if not valid
		var cause string
		if t := check.genericType(rbase, &cause); isValid(t) {
			switch t := t.(type) {
			case *Named:
				baseType = t
			case *Alias:
				// Methods on generic aliases are not permitted.
				// Only report an error if the alias type is valid.
				if isValid(t) {
					check.errorf(rbase, InvalidRecv, "cannot define new methods on generic alias type %s", t)
				}
				// Ok to continue but do not set basetype in this case so that
				// recvType remains invalid (was bug, see go.dev/issue/70417).
			default:
				panic("unreachable")
			}
		} else {
			if cause != "" {
				check.errorf(rbase, InvalidRecv, "%s", cause)
			}
			// Ok to continue but do not set baseType (see comment above).
		}

		// Collect the type parameters declared by the receiver (see also
		// Checker.collectTypeParams). The scope of the type parameter T in
		// "func (r T[T]) f() {}" starts after f, not at r, so we declare it
		// after typechecking rbase (see go.dev/issue/52038).
		recvTParams := make([]*TypeParam, len(rtparams))
		for i, rparam := range rtparams {
			tpar := check.declareTypeParam(rparam, scopePos)
			recvTParams[i] = tpar
			// For historic reasons, type parameters in receiver type expressions
			// are considered both definitions and uses and thus must be recorded
			// in the Info.Uses and Info.Types maps (see go.dev/issue/68670).
			check.recordUse(rparam, tpar.obj)
			check.recordTypeAndValue(rparam, typexpr, tpar, nil)
		}
		recvTParamsList = bindTParams(recvTParams)

		// Get the type parameter bounds from the receiver base type
		// and set them for the respective (local) receiver type parameters.
		if baseType != nil {
			baseTParams := baseType.TypeParams().list()
			if len(recvTParams) == len(baseTParams) {
				smap := makeRenameMap(baseTParams, recvTParams)
				for i, recvTPar := range recvTParams {
					baseTPar := baseTParams[i]
					check.mono.recordCanon(recvTPar, baseTPar)
					// baseTPar.bound is possibly parameterized by other type parameters
					// defined by the generic base type. Substitute those parameters with
					// the receiver type parameters declared by the current method.
					recvTPar.bound = check.subst(recvTPar.obj.pos, baseTPar.bound, smap, nil, check.context())
				}
			} else {
				got := measure(len(recvTParams), "type parameter")
				check.errorf(rbase, BadRecv, "receiver declares %s, but receiver base type declares %d", got, len(baseTParams))
			}

			// The type parameters declared by the receiver also serve as
			// type arguments for the receiver type. Instantiate the receiver.
			check.verifyVersionf(rbase, go1_18, "type instantiation")
			targs := make([]Type, len(recvTParams))
			for i, targ := range recvTParams {
				targs[i] = targ
			}
			recvType = check.instance(rparam.Type.Pos(), baseType, targs, nil, check.context())
			check.recordInstance(rbase, targs, recvType)

			// Reestablish pointerness if needed (but avoid a pointer to an invalid type).
			if rptr && isValid(recvType) {
				recvType = NewPointer(recvType)
			}

			check.recordParenthesizedRecvTypes(rparam.Type, recvType)
		}
	}

	// Create the receiver parameter.
	// recvType is invalid if baseType was never set.
	var recv *Var
	if rname := rparam.Name; rname != nil && rname.Value != "" {
		// named receiver
		recv = newVar(RecvVar, rname.Pos(), check.pkg, rname.Value, recvType)
		// In this case, the receiver is declared by the caller
		// because it must be declared after any type parameters
		// (otherwise it might shadow one of them).
	} else {
		// anonymous receiver
		recv = newVar(RecvVar, rparam.Pos(), check.pkg, "", recvType)
		check.recordImplicit(rparam, recv)
	}

	// Delay validation of receiver type as it may cause premature expansion of types
	// the receiver type is dependent on (see go.dev/issue/51232, go.dev/issue/51233).
	check.later(func() {
		check.validRecv(rbase, recv)
	}).describef(recv, "validRecv(%s)", recv)

	return recv, recvTParamsList
}

func unpointer(t Type) Type {
	for {
		p, _ := t.(*Pointer)
		if p == nil {
			return t
		}
		t = p.base
	}
}

// recordParenthesizedRecvTypes records parenthesized intermediate receiver type
// expressions that all map to the same type, by recursively unpacking expr and
// recording the corresponding type for it. Example:
//
//	expression  -->  type
//	----------------------
//	(*(T[P]))        *T[P]
//	 *(T[P])         *T[P]
//	  (T[P])          T[P]
//	   T[P]           T[P]
func (check *Checker) recordParenthesizedRecvTypes(expr syntax.Expr, typ Type) {
	for {
		check.recordTypeAndValue(expr, typexpr, typ, nil)
		switch e := expr.(type) {
		case *syntax.ParenExpr:
			expr = e.X
		case *syntax.Operation:
			if e.Op == syntax.Mul && e.Y == nil {
				expr = e.X
				// In a correct program, typ must be an unnamed
				// pointer type. But be careful and don't panic.
				ptr, _ := typ.(*Pointer)
				if ptr == nil {
					return // something is wrong
				}
				typ = ptr.base
				break
			}
			return // cannot unpack any further
		default:
			return // cannot unpack any further
		}
	}
}

// collectParams collects (but does not declare) all parameter/result
// variables of list and returns the list of names and corresponding
// variables, and whether the (parameter) list is variadic.
// Anonymous parameters are recorded with nil names.
func (check *Checker) collectParams(kind VarKind, list []*syntax.Field) (names []*syntax.Name, params []*Var, variadic bool) {
	if list == nil {
		return
	}

	var named, anonymous bool

	var typ Type
	var prev syntax.Expr
	for i, field := range list {
		ftype := field.Type
		// type-check type of grouped fields only once
		if ftype != prev {
			prev = ftype
			if t, _ := ftype.(*syntax.DotsType); t != nil {
				ftype = t.Elem
				if kind == ParamVar && i == len(list)-1 {
					variadic = true
				} else {
					check.error(t, InvalidSyntaxTree, "invalid use of ...")
					// ignore ... and continue
				}
			}
			typ = check.varType(ftype)
		}
		// The parser ensures that f.Tag is nil and we don't
		// care if a constructed AST contains a non-nil tag.
		if field.Name != nil {
			// named parameter
			name := field.Name.Value
			if name == "" {
				check.error(field.Name, InvalidSyntaxTree, "anonymous parameter")
				// ok to continue
			}
			par := newVar(kind, field.Name.Pos(), check.pkg, name, typ)
			// named parameter is declared by caller
			names = append(names, field.Name)
			params = append(params, par)
			named = true
		} else {
			// anonymous parameter
			par := newVar(kind, field.Pos(), check.pkg, "", typ)
			check.recordImplicit(field, par)
			names = append(names, nil)
			params = append(params, par)
			anonymous = true
		}
	}

	if named && anonymous {
		check.error(list[0], InvalidSyntaxTree, "list contains both named and anonymous parameters")
		// ok to continue
	}

	// For a variadic function, change the last parameter's type from T to []T.
	// Since we type-checked T rather than ...T, we also need to retro-actively
	// record the type for ...T.
	if variadic {
		last := params[len(params)-1]
		last.typ = &Slice{elem: last.typ}
		check.recordTypeAndValue(list[len(list)-1].Type, typexpr, last.typ, nil)
	}

	return
}

// declareParams declares each named parameter in the current scope.
func (check *Checker) declareParams(names []*syntax.Name, params []*Var, scopePos syntax.Pos) {
	for i, name := range names {
		if name != nil && name.Value != "" {
			check.declare(check.scope, name, params[i], scopePos)
		}
	}
}

// validRecv verifies that the receiver satisfies its respective spec requirements
// and reports an error otherwise.
func (check *Checker) validRecv(pos poser, recv *Var) {
	// spec: "The receiver type must be of the form T or *T where T is a type name."
	rtyp, _ := deref(recv.typ)
	atyp := Unalias(rtyp)
	if !isValid(atyp) {
		return // error was reported before
	}
	// spec: "The type denoted by T is called the receiver base type; it must not
	// be a pointer or interface type and it must be declared in the same package
	// as the method."
	switch T := atyp.(type) {
	case *Named:
		if T.obj.pkg != check.pkg || isCGoTypeObj(T.obj) {
			check.errorf(pos, InvalidRecv, "cannot define new methods on non-local type %s", rtyp)
			break
		}
		var cause string
		switch u := T.Underlying().(type) {
		case *Basic:
			// unsafe.Pointer is treated like a regular pointer
			if u.kind == UnsafePointer {
				cause = "unsafe.Pointer"
			}
		case *Pointer, *Interface:
			cause = "pointer or interface type"
		case *TypeParam:
			// The underlying type of a receiver base type cannot be a
			// type parameter: "type T[P any] P" is not a valid declaration.
			panic("unreachable")
		}
		if cause != "" {
			check.errorf(pos, InvalidRecv, "invalid receiver type %s (%s)", rtyp, cause)
		}
	case *Basic:
		check.errorf(pos, InvalidRecv, "cannot define new methods on non-local type %s", rtyp)
	default:
		check.errorf(pos, InvalidRecv, "invalid receiver type %s", recv.typ)
	}
}

// isCGoTypeObj reports whether the given type name was created by cgo.
func isCGoTypeObj(obj *TypeName) bool {
	return strings.HasPrefix(obj.name, "_Ctype_") ||
		strings.HasPrefix(filepath.Base(obj.pos.FileBase().Filename()), "_cgo_")
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/sizes.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements Sizes.

package types2

// Sizes defines the sizing functions for package unsafe.
type Sizes interface {
	// Alignof returns the alignment of a variable of type T.
	// Alignof must implement the alignment guarantees required by the spec.
	// The result must be >= 1.
	Alignof(T Type) int64

	// Offsetsof returns the offsets of the given struct fields, in bytes.
	// Offsetsof must implement the offset guarantees required by the spec.
	// A negative entry in the result indicates that the struct is too large.
	Offsetsof(fields []*Var) []int64

	// Sizeof returns the size of a variable of type T.
	// Sizeof must implement the size guarantees required by the spec.
	// A negative result indicates that T is too large.
	Sizeof(T Type) int64
}

// StdSizes is a convenience type for creating commonly used Sizes.
// It makes the following simplifying assumptions:
//
//   - The size of explicitly sized basic types (int16, etc.) is the
//     specified size.
//   - The size of strings and interfaces is 2*WordSize.
//   - The size of slices is 3*WordSize.
//   - The size of an array of n elements corresponds to the size of
//     a struct of n consecutive fields of the array's element type.
//   - The size of a struct is the offset of the last field plus that
//     field's size. As with all element types, if the struct is used
//     in an array its size must first be aligned to a multiple of the
//     struct's alignment.
//   - All other types have size WordSize.
//   - Arrays and structs are aligned per spec definition; all other
//     types are naturally aligned with a maximum alignment MaxAlign.
//
// *StdSizes implements Sizes.
type StdSizes struct {
	WordSize int64 // word size in bytes - must be >= 4 (32bits)
	MaxAlign int64 // maximum alignment in bytes - must be >= 1
}

func (s *StdSizes) Alignof(T Type) (result int64) {
	defer func() {
		assert(result >= 1)
	}()

	// For arrays and structs, alignment is defined in terms
	// of alignment of the elements and fields, respectively.
	switch t := T.Underlying().(type) {
	case *Array:
		// spec: "For a variable x of array type: unsafe.Alignof(x)
		// is the same as unsafe.Alignof(x[0]), but at least 1."
		return s.Alignof(t.elem)
	case *Struct:
		if len(t.fields) == 0 && IsSyncAtomicAlign64(T) {
			// Special case: sync/atomic.align64 is an
			// empty struct we recognize as a signal that
			// the struct it contains must be
			// 64-bit-aligned.
			//
			// This logic is equivalent to the logic in
			// cmd/compile/internal/types/size.go:calcStructOffset
			return 8
		}

		// spec: "For a variable x of struct type: unsafe.Alignof(x)
		// is the largest of the values unsafe.Alignof(x.f) for each
		// field f of x, but at least 1."
		max := int64(1)
		for _, f := range t.fields {
			if a := s.Alignof(f.typ); a > max {
				max = a
			}
		}
		return max
	case *Slice, *Interface:
		// Multiword data structures are effectively structs
		// in which each element has size WordSize.
		// Type parameters lead to variable sizes/alignments;
		// StdSizes.Alignof won't be called for them.
		assert(!isTypeParam(T))
		return s.WordSize
	case *Basic:
		// Strings are like slices and interfaces.
		if t.Info()&IsString != 0 {
			return s.WordSize
		}
	case *TypeParam, *Union:
		panic("unreachable")
	}
	a := s.Sizeof(T) // may be 0 or negative
	// spec: "For a variable x of any type: unsafe.Alignof(x) is at least 1."
	if a < 1 {
		return 1
	}
	// complex{64,128} are aligned like [2]float{32,64}.
	if isComplex(T) {
		a /= 2
	}
	if a > s.MaxAlign {
		return s.MaxAlign
	}
	return a
}

func IsSyncAtomicAlign64(T Type) bool {
	named := asNamed(T)
	if named == nil {
		return false
	}
	obj := named.Obj()
	return obj.Name() == "align64" &&
		obj.Pkg() != nil &&
		(obj.Pkg().Path() == "sync/atomic" ||
			obj.Pkg().Path() == "internal/runtime/atomic")
}

func (s *StdSizes) Offsetsof(fields []*Var) []int64 {
	offsets := make([]int64, len(fields))
	var offs int64
	for i, f := range fields {
		if offs < 0 {
			// all remaining offsets are too large
			offsets[i] = -1
			continue
		}
		// offs >= 0
		a := s.Alignof(f.typ)
		offs = align(offs, a) // possibly < 0 if align overflows
		offsets[i] = offs
		if d := s.Sizeof(f.typ); d >= 0 && offs >= 0 {
			offs += d // ok to overflow to < 0
		} else {
			offs = -1 // f.typ or offs is too large
		}
	}
	return offsets
}

var basicSizes = [...]byte{
	Bool:       1,
	Int8:       1,
	Int16:      2,
	Int32:      4,
	Int64:      8,
	Uint8:      1,
	Uint16:     2,
	Uint32:     4,
	Uint64:     8,
	Float32:    4,
	Float64:    8,
	Complex64:  8,
	Complex128: 16,
}

func (s *StdSizes) Sizeof(T Type) int64 {
	switch t := T.Underlying().(type) {
	case *Basic:
		assert(isTyped(T))
		k := t.kind
		if int(k) < len(basicSizes) {
			if s := basicSizes[k]; s > 0 {
				return int64(s)
			}
		}
		if k == String {
			return s.WordSize * 2
		}
	case *Array:
		n := t.len
		if n <= 0 {
			return 0
		}
		// n > 0
		esize := s.Sizeof(t.elem)
		if esize < 0 {
			return -1 // element too large
		}
		if esize == 0 {
			return 0 // 0-size element
		}
		// esize > 0
		a := s.Alignof(t.elem)
		ea := align(esize, a) // possibly < 0 if align overflows
		if ea < 0 {
			return -1
		}
		// ea >= 1
		n1 := n - 1 // n1 >= 0
		// Final size is ea*n1 + esize; and size must be <= maxInt64.
		const maxInt64 = 1<<63 - 1
		if n1 > 0 && ea > maxInt64/n1 {
			return -1 // ea*n1 overflows
		}
		return ea*n1 + esize // may still overflow to < 0 which is ok
	case *Slice:
		return s.WordSize * 3
	case *Struct:
		n := t.NumFields()
		if n == 0 {
			return 0
		}
		offsets := s.Offsetsof(t.fields)
		offs := offsets[n-1]
		size := s.Sizeof(t.fields[n-1].typ)
		if offs < 0 || size < 0 {
			return -1 // type too large
		}
		return offs + size // may overflow to < 0 which is ok
	case *Interface:
		// Type parameters lead to variable sizes/alignments;
		// StdSizes.Sizeof won't be called for them.
		assert(!isTypeParam(T))
		return s.WordSize * 2
	case *TypeParam, *Union:
		panic("unreachable")
	}
	return s.WordSize // catch-all
}

// common architecture word sizes and alignments
var gcArchSizes = map[string]*gcSizes{
	"386":      {4, 4},
	"amd64":    {8, 8},
	"amd64p32": {4, 8},
	"arm":      {4, 4},
	"arm64":    {8, 8},
	"loong64":  {8, 8},
	"mips":     {4, 4},
	"mipsle":   {4, 4},
	"mips64":   {8, 8},
	"mips64le": {8, 8},
	"ppc64":    {8, 8},
	"ppc64le":  {8, 8},
	"riscv64":  {8, 8},
	"s390x":    {8, 8},
	"sparc64":  {8, 8},
	"wasm":     {8, 8},
	// When adding more architectures here,
	// update the doc string of SizesFor below.
}

// SizesFor returns the Sizes used by a compiler for an architecture.
// The result is nil if a compiler/architecture pair is not known.
//
// Supported architectures for compiler "gc":
// "386", "amd64", "amd64p32", "arm", "arm64", "loong64", "mips", "mipsle",
// "mips64", "mips64le", "ppc64", "ppc64le", "riscv64", "s390x", "sparc64", "wasm".
func SizesFor(compiler, arch string) Sizes {
	switch compiler {
	case "gc":
		if s := gcSizesFor(compiler, arch); s != nil {
			return Sizes(s)
		}
	case "gccgo":
		if s, ok := gccgoArchSizes[arch]; ok {
			return Sizes(s)
		}
	}
	return nil
}

// stdSizes is used if Config.Sizes == nil.
var stdSizes = SizesFor("gc", "amd64")

func (conf *Config) alignof(T Type) int64 {
	f := stdSizes.Alignof
	if conf.Sizes != nil {
		f = conf.Sizes.Alignof
	}
	if a := f(T); a >= 1 {
		return a
	}
	panic("implementation of alignof returned an alignment < 1")
}

func (conf *Config) offsetsof(T *Struct) []int64 {
	var offsets []int64
	if T.NumFields() > 0 {
		// compute offsets on demand
		f := stdSizes.Offsetsof
		if conf.Sizes != nil {
			f = conf.Sizes.Offsetsof
		}
		offsets = f(T.fields)
		// sanity checks
		if len(offsets) != T.NumFields() {
			panic("implementation of offsetsof returned the wrong number of offsets")
		}
	}
	return offsets
}

// offsetof returns the offset of the field specified via
// the index sequence relative to T. All embedded fields
// must be structs (rather than pointers to structs).
// If the offset is too large (because T is too large),
// the result is negative.
func (conf *Config) offsetof(T Type, index []int) int64 {
	var offs int64
	for _, i := range index {
		s := T.Underlying().(*Struct)
		d := conf.offsetsof(s)[i]
		if d < 0 {
			return -1
		}
		offs += d
		if offs < 0 {
			return -1
		}
		T = s.fields[i].typ
	}
	return offs
}

// sizeof returns the size of T.
// If T is too large, the result is negative.
func (conf *Config) sizeof(T Type) int64 {
	f := stdSizes.Sizeof
	if conf.Sizes != nil {
		f = conf.Sizes.Sizeof
	}
	return f(T)
}

// align returns the smallest y >= x such that y % a == 0.
// a must be within 1 and 8 and it must be a power of 2.
// The result may be negative due to overflow.
func align(x, a int64) int64 {
	assert(x >= 0 && 1 <= a && a <= 8 && a&(a-1) == 0)
	return (x + a - 1) &^ (a - 1)
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/slice.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

// A Slice represents a slice type.
type Slice struct {
	elem Type
}

// NewSlice returns a new slice type for the given element type.
func NewSlice(elem Type) *Slice { return &Slice{elem: elem} }

// Elem returns the element type of slice s.
func (s *Slice) Elem() Type { return s.elem }

func (s *Slice) Underlying() Type { return s }
func (s *Slice) String() string   { return TypeString(s, nil) }

```

// === FILE: references!/go/src/cmd/compile/internal/types2/stmt.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements typechecking of statements.

package types2

import (
	"cmd/compile/internal/syntax"
	"go/constant"
	. "internal/types/errors"
	"slices"
)

// decl may be nil
func (check *Checker) funcBody(decl *declInfo, name string, sig *Signature, body *syntax.BlockStmt, iota constant.Value) {
	if check.conf.IgnoreFuncBodies {
		panic("function body not ignored")
	}

	if check.conf.Trace {
		check.trace(body.Pos(), "-- %s: %s", name, sig)
	}

	// save/restore current environment and set up function environment
	// (and use 0 indentation at function start)
	defer func(env environment, indent int) {
		check.environment = env
		check.indent = indent
	}(check.environment, check.indent)
	check.environment = environment{
		decl:    decl,
		scope:   sig.scope,
		version: check.version, // TODO(adonovan): would decl.version (if decl != nil) be better?
		iota:    iota,
		sig:     sig,
	}
	check.indent = 0

	check.stmtList(0, body.List)

	if check.hasLabel && !check.conf.IgnoreBranchErrors {
		check.labels(body)
	}

	if sig.results.Len() > 0 && !check.isTerminating(body, "") {
		check.error(body.Rbrace, MissingReturn, "missing return")
	}

	// spec: "Implementation restriction: A compiler may make it illegal to
	// declare a variable inside a function body if the variable is never used."
	check.usage(sig.scope)
}

func (check *Checker) usage(scope *Scope) {
	needUse := func(kind VarKind) bool {
		return !(kind == RecvVar || kind == ParamVar || kind == ResultVar)
	}
	var unused []*Var
	for name, elem := range scope.elems {
		elem = resolve(name, elem)
		if v, _ := elem.(*Var); v != nil && needUse(v.kind) && !check.usedVars[v] {
			unused = append(unused, v)
		}
	}
	slices.SortFunc(unused, func(a, b *Var) int {
		return cmpPos(a.pos, b.pos)
	})
	for _, v := range unused {
		check.softErrorf(v.pos, UnusedVar, "declared and not used: %s", v.name)
	}

	for _, scope := range scope.children {
		// Don't go inside function literal scopes a second time;
		// they are handled explicitly by funcBody.
		if !scope.isFunc {
			check.usage(scope)
		}
	}
}

// stmtContext is a bitset describing which
// control-flow statements are permissible,
// and provides additional context information
// for better error messages.
type stmtContext uint

const (
	// permissible control-flow statements
	breakOk stmtContext = 1 << iota
	continueOk
	fallthroughOk

	// additional context information
	finalSwitchCase
	inTypeSwitch
)

func (check *Checker) simpleStmt(s syntax.Stmt) {
	if s != nil {
		check.stmt(0, s)
	}
}

func trimTrailingEmptyStmts(list []syntax.Stmt) []syntax.Stmt {
	for i := len(list); i > 0; i-- {
		if _, ok := list[i-1].(*syntax.EmptyStmt); !ok {
			return list[:i]
		}
	}
	return nil
}

func (check *Checker) stmtList(ctxt stmtContext, list []syntax.Stmt) {
	ok := ctxt&fallthroughOk != 0
	inner := ctxt &^ fallthroughOk
	list = trimTrailingEmptyStmts(list) // trailing empty statements are "invisible" to fallthrough analysis
	for i, s := range list {
		inner := inner
		if ok && i+1 == len(list) {
			inner |= fallthroughOk
		}
		check.stmt(inner, s)
	}
}

func (check *Checker) multipleSwitchDefaults(list []*syntax.CaseClause) {
	var first *syntax.CaseClause
	for _, c := range list {
		if c.Cases == nil {
			if first != nil {
				check.errorf(c, DuplicateDefault, "multiple defaults (first at %s)", first.Pos())
				// TODO(gri) probably ok to bail out after first error (and simplify this code)
			} else {
				first = c
			}
		}
	}
}

func (check *Checker) multipleSelectDefaults(list []*syntax.CommClause) {
	var first *syntax.CommClause
	for _, c := range list {
		if c.Comm == nil {
			if first != nil {
				check.errorf(c, DuplicateDefault, "multiple defaults (first at %s)", first.Pos())
				// TODO(gri) probably ok to bail out after first error (and simplify this code)
			} else {
				first = c
			}
		}
	}
}

func (check *Checker) openScope(node syntax.Node, comment string) {
	scope := NewScope(check.scope, node.Pos(), syntax.EndPos(node), comment)
	check.recordScope(node, scope)
	check.scope = scope
}

func (check *Checker) closeScope() {
	check.scope = check.scope.Parent()
}

func (check *Checker) suspendedCall(keyword string, call syntax.Expr) {
	code := InvalidDefer
	if keyword == "go" {
		code = InvalidGo
	}

	if _, ok := call.(*syntax.CallExpr); !ok {
		check.errorf(call, code, "expression in %s must be function call", keyword)
		check.use(call)
		return
	}

	var x operand
	var msg string
	switch check.rawExpr(nil, &x, call, nil, false) {
	case conversion:
		msg = "requires function call, not conversion"
	case expression:
		msg = "discards result of"
		code = UnusedResults
	case statement:
		return
	default:
		panic("unreachable")
	}
	check.errorf(&x, code, "%s %s %s", keyword, msg, &x)
}

// goVal returns the Go value for val, or nil.
func goVal(val constant.Value) any {
	// val should exist, but be conservative and check
	if val == nil {
		return nil
	}
	// Match implementation restriction of other compilers.
	// gc only checks duplicates for integer, floating-point
	// and string values, so only create Go values for these
	// types.
	switch val.Kind() {
	case constant.Int:
		if x, ok := constant.Int64Val(val); ok {
			return x
		}
		if x, ok := constant.Uint64Val(val); ok {
			return x
		}
	case constant.Float:
		if x, ok := constant.Float64Val(val); ok {
			return x
		}
	case constant.String:
		return constant.StringVal(val)
	}
	return nil
}

// A valueMap maps a case value (of a basic Go type) to a list of positions
// where the same case value appeared, together with the corresponding case
// types.
// Since two case values may have the same "underlying" value but different
// types we need to also check the value's types (e.g., byte(1) vs myByte(1))
// when the switch expression is of interface type.
type (
	valueMap  map[any][]valueType // underlying Go value -> valueType
	valueType struct {
		pos syntax.Pos
		typ Type
	}
)

func (check *Checker) caseValues(x *operand, values []syntax.Expr, seen valueMap) {
L:
	for _, e := range values {
		var v operand
		check.expr(nil, &v, e)
		if !x.isValid() || !v.isValid() {
			continue L
		}
		check.convertUntyped(&v, x.typ())
		if !v.isValid() {
			continue L
		}
		// Order matters: By comparing v against x, error positions are at the case values.
		res := v // keep original v unchanged
		check.comparison(&res, x, syntax.Eql, true)
		if !res.isValid() {
			continue L
		}
		if v.mode() != constant_ {
			continue L // we're done
		}
		// look for duplicate values
		if val := goVal(v.val); val != nil {
			// look for duplicate types for a given value
			// (quadratic algorithm, but these lists tend to be very short)
			for _, vt := range seen[val] {
				if Identical(v.typ(), vt.typ) {
					err := check.newError(DuplicateCase)
					err.addf(&v, "duplicate case %s in expression switch", &v)
					err.addf(vt.pos, "previous case")
					err.report()
					continue L
				}
			}
			seen[val] = append(seen[val], valueType{v.Pos(), v.typ()})
		}
	}
}

// isNil reports whether the expression e denotes the predeclared value nil.
func (check *Checker) isNil(e syntax.Expr) bool {
	// The only way to express the nil value is by literally writing nil (possibly in parentheses).
	if name, _ := syntax.Unparen(e).(*syntax.Name); name != nil {
		_, ok := check.lookup(name.Value).(*Nil)
		return ok
	}
	return false
}

// caseTypes typechecks the type expressions of a type case, checks for duplicate types
// using the seen map, and verifies that each type is valid with respect to the type of
// the operand x corresponding to the type switch expression. If that expression is not
// valid, x must be nil.
//
//	switch <x>.(type) {
//	case <types>: ...
//	...
//	}
//
// caseTypes returns the case-specific type for a variable v introduced through a short
// variable declaration by the type switch:
//
//	switch v := <x>.(type) {
//	case <types>: // T is the type of <v> in this case
//	...
//	}
//
// If there is exactly one type expression, T is the type of that expression. If there
// are multiple type expressions, or if predeclared nil is among the types, the result
// is the type of x. If x is invalid (nil), the result is the invalid type.
func (check *Checker) caseTypes(x *operand, types []syntax.Expr, seen map[Type]syntax.Expr) Type {
	var T Type
	var dummy operand
L:
	for _, e := range types {
		// The spec allows the value nil instead of a type.
		if check.isNil(e) {
			T = nil
			check.expr(nil, &dummy, e) // run e through expr so we get the usual Info recordings
		} else {
			T = check.varType(e)
			if !isValid(T) {
				continue L
			}
		}
		// look for duplicate types
		// (quadratic algorithm, but type switches tend to be reasonably small)
		for t, other := range seen {
			if T == nil && t == nil || T != nil && t != nil && Identical(T, t) {
				// talk about "case" rather than "type" because of nil case
				Ts := "nil"
				if T != nil {
					Ts = TypeString(T, check.qualifier)
				}
				err := check.newError(DuplicateCase)
				err.addf(e, "duplicate case %s in type switch", Ts)
				err.addf(other, "previous case")
				err.report()
				continue L
			}
		}
		seen[T] = e
		if x != nil && T != nil {
			check.typeAssertion(e, x, T, true)
		}
	}

	// spec: "In clauses with a case listing exactly one type, the variable has that type;
	// otherwise, the variable has the type of the expression in the TypeSwitchGuard.
	if len(types) != 1 || T == nil {
		T = Typ[Invalid]
		if x != nil {
			T = x.typ()
		}
	}

	assert(T != nil)
	return T
}

// TODO(gri) Once we are certain that typeHash is correct in all situations, use this version of caseTypes instead.
// (Currently it may be possible that different types have identical names and import paths due to ImporterFrom.)
func (check *Checker) caseTypes_currently_unused(x *operand, xtyp *Interface, types []syntax.Expr, seen map[string]syntax.Expr) Type {
	var T Type
	var dummy operand
L:
	for _, e := range types {
		// The spec allows the value nil instead of a type.
		var hash string
		if check.isNil(e) {
			check.expr(nil, &dummy, e) // run e through expr so we get the usual Info recordings
			T = nil
			hash = "<nil>" // avoid collision with a type named nil
		} else {
			T = check.varType(e)
			if !isValid(T) {
				continue L
			}
			panic("enable typeHash(T, nil)")
			// hash = typeHash(T, nil)
		}
		// look for duplicate types
		if other := seen[hash]; other != nil {
			// talk about "case" rather than "type" because of nil case
			Ts := "nil"
			if T != nil {
				Ts = TypeString(T, check.qualifier)
			}
			err := check.newError(DuplicateCase)
			err.addf(e, "duplicate case %s in type switch", Ts)
			err.addf(other, "previous case")
			err.report()
			continue L
		}
		seen[hash] = e
		if T != nil {
			check.typeAssertion(e, x, T, true)
		}
	}

	// spec: "In clauses with a case listing exactly one type, the variable has that type;
	// otherwise, the variable has the type of the expression in the TypeSwitchGuard.
	if len(types) != 1 || T == nil {
		T = Typ[Invalid]
		if x != nil {
			T = x.typ()
		}
	}

	assert(T != nil)
	return T
}

// stmt typechecks statement s.
func (check *Checker) stmt(ctxt stmtContext, s syntax.Stmt) {
	// statements must end with the same top scope as they started with
	if debug {
		defer func(scope *Scope) {
			// don't check if code is panicking
			if p := recover(); p != nil {
				panic(p)
			}
			assert(scope == check.scope)
		}(check.scope)
	}

	// process collected function literals before scope changes
	defer check.processDelayed(len(check.delayed))

	// reset context for statements of inner blocks
	inner := ctxt &^ (fallthroughOk | finalSwitchCase | inTypeSwitch)

	switch s := s.(type) {
	case *syntax.EmptyStmt:
		// ignore

	case *syntax.DeclStmt:
		check.declStmt(s.DeclList)

	case *syntax.LabeledStmt:
		check.hasLabel = true
		check.stmt(ctxt, s.Stmt)

	case *syntax.ExprStmt:
		// spec: "With the exception of specific built-in functions,
		// function and method calls and receive operations can appear
		// in statement context. Such statements may be parenthesized."
		var x operand
		kind := check.rawExpr(nil, &x, s.X, nil, false)
		var msg string
		var code Code
		switch x.mode() {
		default:
			if kind == statement {
				return
			}
			msg = "is not used"
			code = UnusedExpr
		case builtin:
			msg = "must be called"
			code = UncalledBuiltin
		case typexpr:
			msg = "is not an expression"
			code = NotAnExpr
		}
		check.errorf(&x, code, "%s %s", &x, msg)

	case *syntax.SendStmt:
		var ch, val operand
		check.expr(nil, &ch, s.Chan)
		check.genericExpr(&val, s.Value, nil)
		if !ch.isValid() || !val.isValid() {
			return
		}
		if elem := check.chanElem(s, &ch, false); elem != nil {
			check.assignment(&val, elem, "send")
		}

	case *syntax.AssignStmt:
		if s.Rhs == nil {
			// x++ or x--
			// (no need to call unpackExpr as s.Lhs must be single-valued)
			var x operand
			check.expr(nil, &x, s.Lhs)
			if !x.isValid() {
				return
			}
			if !allNumeric(x.typ()) {
				check.errorf(s.Lhs, NonNumericIncDec, invalidOp+"%s%s%s (non-numeric type %s)", s.Lhs, s.Op, s.Op, x.typ())
				return
			}
			check.assignVar(s.Lhs, nil, &x, "assignment")
			return
		}

		lhs := syntax.UnpackListExpr(s.Lhs)
		rhs := syntax.UnpackListExpr(s.Rhs)
		switch s.Op {
		case 0:
			check.assignVars(lhs, rhs)
			return
		case syntax.Def:
			check.shortVarDecl(s.Pos(), lhs, rhs)
			return
		}

		// assignment operations
		if len(lhs) != 1 || len(rhs) != 1 {
			check.errorf(s, MultiValAssignOp, "assignment operation %s requires single-valued expressions", s.Op)
			return
		}

		var x operand
		check.binary(&x, nil, lhs[0], rhs[0], s.Op)
		check.assignVar(lhs[0], nil, &x, "assignment")

	case *syntax.CallStmt:
		kind := "go"
		if s.Tok == syntax.Defer {
			kind = "defer"
		}
		check.suspendedCall(kind, s.Call)

	case *syntax.ReturnStmt:
		res := check.sig.results
		// Return with implicit results allowed for function with named results.
		// (If one is named, all are named.)
		results := syntax.UnpackListExpr(s.Results)
		if len(results) == 0 && res.Len() > 0 && res.vars[0].name != "" {
			// spec: "Implementation restriction: A compiler may disallow an empty expression
			// list in a "return" statement if a different entity (constant, type, or variable)
			// with the same name as a result parameter is in scope at the place of the return."
			for _, obj := range res.vars {
				if alt := check.lookup(obj.name); alt != nil && alt != obj {
					err := check.newError(OutOfScopeResult)
					err.addf(s, "result parameter %s not in scope at return", obj.name)
					err.addf(alt, "inner declaration of %s", obj)
					err.report()
					// ok to continue
				}
			}
		} else {
			var lhs []*Var
			if res.Len() > 0 {
				lhs = res.vars
			}
			check.initVars(lhs, results, s)
		}

	case *syntax.BranchStmt:
		if s.Label != nil {
			check.hasLabel = true
			break // checked in 2nd pass (check.labels)
		}
		if check.conf.IgnoreBranchErrors {
			break
		}
		switch s.Tok {
		case syntax.Break:
			if ctxt&breakOk == 0 {
				check.error(s, MisplacedBreak, "break not in for, switch, or select statement")
			}
		case syntax.Continue:
			if ctxt&continueOk == 0 {
				check.error(s, MisplacedContinue, "continue not in for statement")
			}
		case syntax.Fallthrough:
			if ctxt&fallthroughOk == 0 {
				var msg string
				switch {
				case ctxt&finalSwitchCase != 0:
					msg = "cannot fallthrough final case in switch"
				case ctxt&inTypeSwitch != 0:
					msg = "cannot fallthrough in type switch"
				default:
					msg = "fallthrough statement out of place"
				}
				check.error(s, MisplacedFallthrough, msg)
			}
		case syntax.Goto:
			// goto's must have labels, should have been caught above
			fallthrough
		default:
			check.errorf(s, InvalidSyntaxTree, "branch statement: %s", s.Tok)
		}

	case *syntax.BlockStmt:
		check.openScope(s, "block")
		defer check.closeScope()

		check.stmtList(inner, s.List)

	case *syntax.IfStmt:
		check.openScope(s, "if")
		defer check.closeScope()

		check.simpleStmt(s.Init)
		var x operand
		check.expr(nil, &x, s.Cond)
		if x.isValid() && !allBoolean(x.typ()) {
			check.error(s.Cond, InvalidCond, "non-boolean condition in if statement")
		}
		check.stmt(inner, s.Then)
		// The parser produces a correct AST but if it was modified
		// elsewhere the else branch may be invalid. Check again.
		switch s.Else.(type) {
		case nil:
			// valid or error already reported
		case *syntax.IfStmt, *syntax.BlockStmt:
			check.stmt(inner, s.Else)
		default:
			check.error(s.Else, InvalidSyntaxTree, "invalid else branch in if statement")
		}

	case *syntax.SwitchStmt:
		inner |= breakOk
		check.openScope(s, "switch")
		defer check.closeScope()

		check.simpleStmt(s.Init)

		if g, _ := s.Tag.(*syntax.TypeSwitchGuard); g != nil {
			check.typeSwitchStmt(inner|inTypeSwitch, s, g)
		} else {
			check.switchStmt(inner, s)
		}

	case *syntax.SelectStmt:
		inner |= breakOk

		check.multipleSelectDefaults(s.Body)

		for _, clause := range s.Body {
			if clause == nil {
				continue // error reported before
			}

			// clause.Comm must be a SendStmt, RecvStmt, or default case
			valid := false
			var rhs syntax.Expr // rhs of RecvStmt, or nil
			switch s := clause.Comm.(type) {
			case nil, *syntax.SendStmt:
				valid = true
			case *syntax.AssignStmt:
				if _, ok := s.Rhs.(*syntax.ListExpr); !ok {
					rhs = s.Rhs
				}
			case *syntax.ExprStmt:
				rhs = s.X
			}

			// if present, rhs must be a receive operation
			if rhs != nil {
				if x, _ := syntax.Unparen(rhs).(*syntax.Operation); x != nil && x.Y == nil && x.Op == syntax.Recv {
					valid = true
				}
			}

			if !valid {
				check.error(clause.Comm, InvalidSelectCase, "select case must be send or receive (possibly with assignment)")
				continue
			}
			check.openScope(clause, "case")
			if clause.Comm != nil {
				check.stmt(inner, clause.Comm)
			}
			check.stmtList(inner, clause.Body)
			check.closeScope()
		}

	case *syntax.ForStmt:
		inner |= breakOk | continueOk

		if rclause, _ := s.Init.(*syntax.RangeClause); rclause != nil {
			// extract sKey, sValue, s.Extra from the range clause
			sKey := rclause.Lhs            // possibly nil
			var sValue, sExtra syntax.Expr // possibly nil
			if p, _ := sKey.(*syntax.ListExpr); p != nil {
				if len(p.ElemList) < 2 {
					check.error(s, InvalidSyntaxTree, "invalid lhs in range clause")
					return
				}
				// len(p.ElemList) >= 2
				sKey = p.ElemList[0]
				sValue = p.ElemList[1]
				if len(p.ElemList) > 2 {
					// delay error reporting until we know more
					sExtra = p.ElemList[2]
				}
			}
			check.rangeStmt(inner, s, s, sKey, sValue, sExtra, rclause.X, rclause.Def)
			break
		}

		check.openScope(s, "for")
		defer check.closeScope()

		check.simpleStmt(s.Init)
		if s.Cond != nil {
			var x operand
			check.expr(nil, &x, s.Cond)
			if x.isValid() && !allBoolean(x.typ()) {
				check.error(s.Cond, InvalidCond, "non-boolean condition in for statement")
			}
		}
		check.simpleStmt(s.Post)
		// spec: "The init statement may be a short variable
		// declaration, but the post statement must not."
		if s, _ := s.Post.(*syntax.AssignStmt); s != nil && s.Op == syntax.Def {
			// The parser already reported an error.
			check.use(s.Lhs) // avoid follow-up errors
		}
		check.stmt(inner, s.Body)

	default:
		check.error(s, InvalidSyntaxTree, "invalid statement")
	}
}

func (check *Checker) switchStmt(inner stmtContext, s *syntax.SwitchStmt) {
	// init statement already handled

	var x operand
	if s.Tag != nil {
		check.expr(nil, &x, s.Tag)
		// By checking assignment of x to an invisible temporary
		// (as a compiler would), we get all the relevant checks.
		check.assignment(&x, nil, "switch expression")
		if x.isValid() && !Comparable(x.typ()) && !hasNil(x.typ()) {
			check.errorf(&x, InvalidExprSwitch, "cannot switch on %s (%s is not comparable)", &x, x.typ())
			x.invalidate()
		}
	} else {
		// spec: "A missing switch expression is
		// equivalent to the boolean value true."
		x.mode_ = constant_
		x.typ_ = Typ[Bool]
		x.val = constant.MakeBool(true)
		// TODO(gri) should have a better position here
		pos := s.Rbrace
		if len(s.Body) > 0 {
			pos = s.Body[0].Pos()
		}
		x.expr = syntax.NewName(pos, "true")
	}

	check.multipleSwitchDefaults(s.Body)

	seen := make(valueMap) // map of seen case values to positions and types
	for i, clause := range s.Body {
		if clause == nil {
			check.error(clause, InvalidSyntaxTree, "incorrect expression switch case")
			continue
		}
		inner := inner
		if i+1 < len(s.Body) {
			inner |= fallthroughOk
		} else {
			inner |= finalSwitchCase
		}
		check.caseValues(&x, syntax.UnpackListExpr(clause.Cases), seen)
		check.openScope(clause, "case")
		check.stmtList(inner, clause.Body)
		check.closeScope()
	}
}

func (check *Checker) typeSwitchStmt(inner stmtContext, s *syntax.SwitchStmt, guard *syntax.TypeSwitchGuard) {
	// init statement already handled

	// A type switch guard must be of the form:
	//
	//     TypeSwitchGuard = [ identifier ":=" ] PrimaryExpr "." "(" "type" ")" .
	//                          \__lhs__/        \___rhs___/

	// check lhs, if any
	lhs := guard.Lhs
	if lhs != nil {
		if lhs.Value == "_" {
			// _ := x.(type) is an invalid short variable declaration
			check.softErrorf(lhs, NoNewVar, "no new variable on left side of :=")
			lhs = nil // avoid declared and not used error below
		} else {
			check.recordDef(lhs, nil) // lhs variable is implicitly declared in each cause clause
		}
	}

	// check rhs
	var sx *operand // switch expression against which cases are compared against; nil if invalid
	{
		var x operand
		check.expr(nil, &x, guard.X)
		if x.isValid() {
			if isTypeParam(x.typ()) {
				check.errorf(&x, InvalidTypeSwitch, "cannot use type switch on type parameter value %s", &x)
			} else if IsInterface(x.typ()) {
				sx = &x
			} else {
				check.errorf(&x, InvalidTypeSwitch, "%s is not an interface", &x)
			}
		}
	}

	check.multipleSwitchDefaults(s.Body)

	var lhsVars []*Var                 // list of implicitly declared lhs variables
	seen := make(map[Type]syntax.Expr) // map of seen types to positions
	for _, clause := range s.Body {
		if clause == nil {
			check.error(s, InvalidSyntaxTree, "incorrect type switch case")
			continue
		}
		// Check each type in this type switch case.
		cases := syntax.UnpackListExpr(clause.Cases)
		T := check.caseTypes(sx, cases, seen)
		check.openScope(clause, "case")
		// If lhs exists, declare a corresponding variable in the case-local scope.
		if lhs != nil {
			obj := newVar(LocalVar, lhs.Pos(), check.pkg, lhs.Value, T)
			check.declare(check.scope, nil, obj, clause.Colon)
			check.recordImplicit(clause, obj)
			// For the "declared and not used" error, all lhs variables act as
			// one; i.e., if any one of them is 'used', all of them are 'used'.
			// Collect them for later analysis.
			lhsVars = append(lhsVars, obj)
		}
		check.stmtList(inner, clause.Body)
		check.closeScope()
	}

	// If lhs exists, we must have at least one lhs variable that was used.
	// (We can't use check.usage because that only looks at one scope; and
	// we don't want to use the same variable for all scopes and change the
	// variable type underfoot.)
	if lhs != nil {
		var used bool
		for _, v := range lhsVars {
			if check.usedVars[v] {
				used = true
			}
			check.usedVars[v] = true // avoid usage error when checking entire function
		}
		if !used {
			check.softErrorf(lhs, UnusedVar, "%s declared and not used", lhs.Value)
		}
	}
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/struct.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
	"strconv"
)

// ----------------------------------------------------------------------------
// API

// A Struct represents a struct type.
type Struct struct {
	fields []*Var   // fields != nil indicates the struct is set up (possibly with len(fields) == 0)
	tags   []string // field tags; nil if there are no tags
}

// NewStruct returns a new struct with the given fields and corresponding field tags.
// If a field with index i has a tag, tags[i] must be that tag, but len(tags) may be
// only as long as required to hold the tag with the largest index i. Consequently,
// if no field has a tag, tags may be nil.
func NewStruct(fields []*Var, tags []string) *Struct {
	var fset objset
	for _, f := range fields {
		if f.name != "_" && fset.insert(f) != nil {
			panic("multiple fields with the same name")
		}
	}
	if len(tags) > len(fields) {
		panic("more tags than fields")
	}
	s := &Struct{fields: fields, tags: tags}
	s.markComplete()
	return s
}

// NumFields returns the number of fields in the struct (including blank and embedded fields).
func (s *Struct) NumFields() int { return len(s.fields) }

// Field returns the i'th field for 0 <= i < NumFields().
func (s *Struct) Field(i int) *Var { return s.fields[i] }

// Tag returns the i'th field tag for 0 <= i < NumFields().
func (s *Struct) Tag(i int) string {
	if i < len(s.tags) {
		return s.tags[i]
	}
	return ""
}

func (s *Struct) Underlying() Type { return s }
func (s *Struct) String() string   { return TypeString(s, nil) }

// ----------------------------------------------------------------------------
// Implementation

func (s *Struct) markComplete() {
	if s.fields == nil {
		s.fields = make([]*Var, 0)
	}
}

func (check *Checker) structType(styp *Struct, e *syntax.StructType) {
	if e.FieldList == nil {
		styp.markComplete()
		return
	}

	// struct fields and tags
	var fields []*Var
	var tags []string

	// for double-declaration checks
	var fset objset

	// current field typ and tag
	var typ Type
	var tag string
	add := func(ident *syntax.Name, embedded bool) {
		if tag != "" && tags == nil {
			tags = make([]string, len(fields))
		}
		if tags != nil {
			tags = append(tags, tag)
		}

		pos := ident.Pos()
		name := ident.Value
		fld := NewField(pos, check.pkg, name, typ, embedded)
		// spec: "Within a struct, non-blank field names must be unique."
		if name == "_" || check.declareInSet(&fset, pos, fld) {
			fields = append(fields, fld)
			check.recordDef(ident, fld)
		}
	}

	// addInvalid adds an embedded field of invalid type to the struct for
	// fields with errors; this keeps the number of struct fields in sync
	// with the source as long as the fields are _ or have different names
	// (go.dev/issue/25627).
	addInvalid := func(ident *syntax.Name) {
		typ = Typ[Invalid]
		tag = ""
		add(ident, true)
	}

	var prev syntax.Expr
	for i, f := range e.FieldList {
		// Fields declared syntactically with the same type (e.g.: a, b, c T)
		// share the same type expression. Only check type if it's a new type.
		if i == 0 || f.Type != prev {
			typ = check.varType(f.Type)
			prev = f.Type
		}
		tag = ""
		if i < len(e.TagList) {
			tag = check.tag(e.TagList[i])
		}
		if f.Name != nil {
			// named field
			add(f.Name, false)
		} else {
			// embedded field
			// spec: "An embedded type must be specified as a type name T or as a
			// pointer to a non-interface type name *T, and T itself may not be a
			// pointer type."
			pos := syntax.StartPos(f.Type) // position of type, for errors
			name := embeddedFieldIdent(f.Type)
			if name == nil {
				check.errorf(pos, InvalidSyntaxTree, "invalid embedded field type %s", f.Type)
				name = syntax.NewName(pos, "_")
				addInvalid(name)
				continue
			}
			add(name, true) // struct{p.T} field has position of T

			// Because we have a name, typ must be of the form T or *T, where T is the name
			// of a (named or alias) type, and t (= deref(typ)) must be the type of T.
			// We must delay this check to the end because we don't want to instantiate
			// (via t.Underlying()) a possibly incomplete type.
			embeddedTyp := typ // for closure below
			embeddedPos := pos
			check.later(func() {
				t, isPtr := deref(embeddedTyp)
				switch u := t.Underlying().(type) {
				case *Basic:
					if !isValid(t) {
						// error was reported before
						return
					}
					// unsafe.Pointer is treated like a regular pointer
					if u.kind == UnsafePointer {
						check.error(embeddedPos, InvalidPtrEmbed, "embedded field type cannot be unsafe.Pointer")
					}
				case *Pointer:
					check.error(embeddedPos, InvalidPtrEmbed, "embedded field type cannot be a pointer")
				case *Interface:
					if isTypeParam(t) {
						// The error code here is inconsistent with other error codes for
						// invalid embedding, because this restriction may be relaxed in the
						// future, and so it did not warrant a new error code.
						check.error(embeddedPos, MisplacedTypeParam, "embedded field type cannot be a (pointer to a) type parameter")
						break
					}
					if isPtr {
						check.error(embeddedPos, InvalidPtrEmbed, "embedded field type cannot be a pointer to an interface")
					}
				}
			}).describef(embeddedPos, "check embedded type %s", embeddedTyp)
		}
	}

	styp.fields = fields
	styp.tags = tags
	styp.markComplete()
}

func embeddedFieldIdent(e syntax.Expr) *syntax.Name {
	switch e := e.(type) {
	case *syntax.Name:
		return e
	case *syntax.Operation:
		if base := ptrBase(e); base != nil {
			// *T is valid, but **T is not
			if op, _ := base.(*syntax.Operation); op == nil || ptrBase(op) == nil {
				return embeddedFieldIdent(e.X)
			}
		}
	case *syntax.SelectorExpr:
		return e.Sel
	case *syntax.IndexExpr:
		return embeddedFieldIdent(e.X)
	}
	return nil // invalid embedded field
}

func (check *Checker) declareInSet(oset *objset, pos syntax.Pos, obj Object) bool {
	if alt := oset.insert(obj); alt != nil {
		err := check.newError(DuplicateDecl)
		err.addf(pos, "%s redeclared", obj.Name())
		err.addAltDecl(alt)
		err.report()
		return false
	}
	return true
}

func (check *Checker) tag(t *syntax.BasicLit) string {
	// If t.Bad, an error was reported during parsing.
	if t != nil && !t.Bad {
		if t.Kind == syntax.StringLit {
			if val, err := strconv.Unquote(t.Value); err == nil {
				return val
			}
		}
		check.errorf(t, InvalidSyntaxTree, "incorrect tag syntax: %q", t.Value)
	}
	return ""
}

func ptrBase(x *syntax.Operation) syntax.Expr {
	if x.Op == syntax.Mul && x.Y == nil {
		return x.X
	}
	return nil
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/subst.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements type parameter substitution.

package types2

import (
	"cmd/compile/internal/syntax"
)

type substMap map[*TypeParam]Type

// makeSubstMap creates a new substitution map mapping tpars[i] to targs[i].
// If targs[i] is nil, tpars[i] is not substituted.
func makeSubstMap(tpars []*TypeParam, targs []Type) substMap {
	assert(len(tpars) == len(targs))
	proj := make(substMap, len(tpars))
	for i, tpar := range tpars {
		proj[tpar] = targs[i]
	}
	return proj
}

// makeRenameMap is like makeSubstMap, but creates a map used to rename type
// parameters in from with the type parameters in to.
func makeRenameMap(from, to []*TypeParam) substMap {
	assert(len(from) == len(to))
	proj := make(substMap, len(from))
	for i, tpar := range from {
		proj[tpar] = to[i]
	}
	return proj
}

func (m substMap) empty() bool {
	return len(m) == 0
}

func (m substMap) lookup(tpar *TypeParam) Type {
	if t := m[tpar]; t != nil {
		return t
	}
	return tpar
}

// subst returns the type typ with its type parameters tpars replaced by the
// corresponding type arguments targs, recursively. subst doesn't modify the
// incoming type. If a substitution took place, the result type is different
// from the incoming type.
//
// If expanding is non-nil, it is the instance type currently being expanded.
// One of expanding or ctxt must be non-nil.
func (check *Checker) subst(pos syntax.Pos, typ Type, smap substMap, expanding *Named, ctxt *Context) Type {
	assert(expanding != nil || ctxt != nil)

	if smap.empty() {
		return typ
	}

	// common cases
	switch t := typ.(type) {
	case *Basic:
		return typ // nothing to do
	case *TypeParam:
		return smap.lookup(t)
	}

	// general case
	subst := subster{
		pos:       pos,
		smap:      smap,
		check:     check,
		expanding: expanding,
		ctxt:      ctxt,
	}
	return subst.typ(typ)
}

type subster struct {
	pos       syntax.Pos
	smap      substMap
	check     *Checker // nil if called via Instantiate
	expanding *Named   // if non-nil, the instance that is being expanded
	ctxt      *Context
}

func (subst *subster) typ(typ Type) Type {
	switch t := typ.(type) {
	case nil:
		// Call typOrNil if it's possible that typ is nil.
		panic("nil typ")

	case *Basic:
		// nothing to do

	case *Alias:
		// This code follows the code for *Named types closely.
		// TODO(gri) try to factor better
		orig := t.Origin()
		n := orig.TypeParams().Len()
		if n == 0 {
			return t // type is not parameterized
		}

		// TODO(gri) do we need this for Alias types?
		if t.TypeArgs().Len() != n {
			return Typ[Invalid] // error reported elsewhere
		}

		// already instantiated
		// For each (existing) type argument determine if it needs
		// to be substituted; i.e., if it is or contains a type parameter
		// that has a type argument for it.
		if targs := substList(t.TypeArgs().list(), subst.typ); targs != nil {
			return subst.check.newAliasInstance(subst.pos, t.orig, targs, subst.expanding, subst.ctxt)
		}

	case *Array:
		elem := subst.typOrNil(t.elem)
		if elem != t.elem {
			return &Array{len: t.len, elem: elem}
		}

	case *Slice:
		elem := subst.typOrNil(t.elem)
		if elem != t.elem {
			return &Slice{elem: elem}
		}

	case *Struct:
		if fields := substList(t.fields, subst.var_); fields != nil {
			s := &Struct{fields: fields, tags: t.tags}
			s.markComplete()
			return s
		}

	case *Pointer:
		base := subst.typ(t.base)
		if base != t.base {
			return &Pointer{base: base}
		}

	case *Tuple:
		return subst.tuple(t)

	case *Signature:
		// Preserve the receiver: it is handled during *Interface and *Named type
		// substitution.
		//
		// Naively doing the substitution here can lead to an infinite recursion in
		// the case where the receiver is an interface. For example, consider the
		// following declaration:
		//
		//  type T[A any] struct { f interface{ m() } }
		//
		// In this case, the type of f is an interface that is itself the receiver
		// type of all of its methods. Because we have no type name to break
		// cycles, substituting in the recv results in an infinite loop of
		// recv->interface->recv->interface->...
		recv := t.recv

		// If t is a generic method signature whose own type parameters are not
		// themselves the subject of this substitution, we are substituting the
		// receiver type parameters (via Named.expandMethod). Because a method
		// type parameter's bound may refer to a receiver type parameter
		// (e.g. func (G[T]) M[P interface{ ~*T }]), we must create fresh type
		// parameters with substituted bounds, and rename occurrences in params
		// and results so they refer to the fresh parameters. Otherwise the
		// resulting signature would retain a free reference to the original
		// receiver type parameter.
		//
		// Fresh type parameters are always created, even when the bounds are
		// unaffected by the substitution, so that the methods of distinct
		// instances of the receiver type have distinct (method-specific) type
		// parameters.
		//
		// When t's type parameters are the variables being substituted, we are
		// instantiating t itself; the caller (Checker.instance for *Signature)
		// sets tparams to nil afterward, so we leave them in place here.
		tparams := t.tparams
		s := subst
		if n := tparams.Len(); n > 0 {
			// If (any) one of the signature's type parameters is in the
			// substitution map, this subst call is an instantiation of the
			// signature.
			_, instantiating := subst.smap[tparams.At(0)]
			if debug {
				// When calling subst on a signature, the substitution either
				// applies to all of the type parameters (all are in the map)
				// or none of them (none are in the map).
				for _, tp := range tparams.list() {
					_, ok := subst.smap[tp]
					assert(ok == instantiating)
				}
			}
			if !instantiating {
				fresh := make([]*TypeParam, n)
				// We're introducing a fresh set of method type parameters
				// which appear elsewhere in the signature (parameter or
				// result types, or the bounds of other type parameters).
				// Create an updated substitution map containing the
				// existing entries plus an entry for each fresh type
				// parameter so that they are substituted simultaneously
				// when we proceed with the outer substitution.
				smap := make(substMap, len(subst.smap)+n)
				for k, v := range subst.smap {
					smap[k] = v
				}
				for i, tp := range tparams.list() {
					tname := NewTypeName(tp.Obj().Pos(), tp.Obj().Pkg(), tp.Obj().Name(), nil)
					ftp := subst.check.newTypeParam(tname, nil)
					ftp.index = tp.index
					fresh[i] = ftp
					smap[tp] = ftp
					// The fresh parameter stands in for tp in the mono graph,
					// so that instantiations of (e.g.) G[int].M and G[A].M
					// are tracked against the same vertex as the origin's tp.
					if subst.check != nil {
						subst.check.mono.recordCanon(ftp, tp)
					}
				}
				// Now that we have the updated substitution map, use it to
				// compute the constraints for the fresh type parameters.
				for i, tp := range tparams.list() {
					fresh[i].bound = subst.check.subst(subst.pos, tp.bound, smap, subst.expanding, subst.ctxt)
				}
				// Continue with the fresh type parameters and updated map.
				tparams = &TypeParamList{tparams: fresh}
				s = &subster{
					pos:       subst.pos,
					smap:      smap,
					check:     subst.check,
					expanding: subst.expanding,
					ctxt:      subst.ctxt,
				}
			}
		}

		params := s.tuple(t.params)
		results := s.tuple(t.results)
		if params != t.params || results != t.results || tparams != t.tparams {
			return &Signature{
				rparams: t.rparams,
				tparams: tparams,
				// instantiated signatures have a nil scope
				recv:     recv,
				recvold:  t.recvold,
				params:   params,
				results:  results,
				variadic: t.variadic,
			}
		}

	case *Union:
		if terms := substList(t.terms, subst.term); terms != nil {
			// term list substitution may introduce duplicate terms (unlikely but possible).
			// This is ok; lazy type set computation will determine the actual type set
			// in normal form.
			return &Union{terms}
		}

	case *Interface:
		methods := substList(t.methods, subst.func_)
		embeddeds := substList(t.embeddeds, subst.typ)
		if methods != nil || embeddeds != nil {
			if methods == nil {
				methods = t.methods
			}
			if embeddeds == nil {
				embeddeds = t.embeddeds
			}
			iface := subst.check.newInterface()
			iface.embeddeds = embeddeds
			iface.embedPos = t.embedPos
			iface.implicit = t.implicit
			assert(t.complete) // otherwise we are copying incomplete data
			iface.complete = t.complete
			// If we've changed the interface type, we may need to replace its
			// receiver if the receiver type is the original interface. Receivers of
			// *Named type are replaced during named type expansion.
			//
			// Notably, it's possible to reach here and not create a new *Interface,
			// even though the receiver type may be parameterized. For example:
			//
			//  type T[P any] interface{ m() }
			//
			// In this case the interface will not be substituted here, because its
			// method signatures do not depend on the type parameter P, but we still
			// need to create new interface methods to hold the instantiated
			// receiver. This is handled by Named.expandUnderlying.
			iface.methods, _ = replaceRecvType(methods, t, iface)

			// If check != nil, check.newInterface will have saved the interface for later completion.
			if subst.check == nil { // golang/go#61561: all newly created interfaces must be completed
				iface.typeSet()
			}
			return iface
		}

	case *Map:
		key := subst.typ(t.key)
		elem := subst.typ(t.elem)
		if key != t.key || elem != t.elem {
			return &Map{key: key, elem: elem}
		}

	case *Chan:
		elem := subst.typ(t.elem)
		if elem != t.elem {
			return &Chan{dir: t.dir, elem: elem}
		}

	case *Named:
		// subst is called during expansion, so in this function we need to be
		// careful not to call any methods that would cause t to be expanded: doing
		// so would result in deadlock.
		//
		// So we call t.Origin().TypeParams() rather than t.TypeParams().
		orig := t.Origin()
		n := orig.TypeParams().Len()
		if n == 0 {
			return t // type is not parameterized
		}

		if t.TypeArgs().Len() != n {
			return Typ[Invalid] // error reported elsewhere
		}

		// already instantiated
		// For each (existing) type argument determine if it needs
		// to be substituted; i.e., if it is or contains a type parameter
		// that has a type argument for it.
		if targs := substList(t.TypeArgs().list(), subst.typ); targs != nil {
			// Create a new instance and populate the context to avoid endless
			// recursion. The position used here is irrelevant because validation only
			// occurs on t (we don't call validType on named), but we use subst.pos to
			// help with debugging.
			return subst.check.instance(subst.pos, orig, targs, subst.expanding, subst.ctxt)
		}

	case *TypeParam:
		return subst.smap.lookup(t)

	default:
		panic("unreachable")
	}

	return typ
}

// typOrNil is like typ but if the argument is nil it is replaced with Typ[Invalid].
// A nil type may appear in pathological cases such as type T[P any] []func(_ T([]_))
// where an array/slice element is accessed before it is set up.
func (subst *subster) typOrNil(typ Type) Type {
	if typ == nil {
		return Typ[Invalid]
	}
	return subst.typ(typ)
}

func (subst *subster) var_(v *Var) *Var {
	if v != nil {
		if typ := subst.typ(v.typ); typ != v.typ {
			return cloneVar(v, typ)
		}
	}
	return v
}

func cloneVar(v *Var, typ Type) *Var {
	copy := *v
	copy.typ = typ
	copy.origin = v.Origin()
	return &copy
}

func (subst *subster) tuple(t *Tuple) *Tuple {
	if t != nil {
		if vars := substList(t.vars, subst.var_); vars != nil {
			return &Tuple{vars: vars}
		}
	}
	return t
}

// substList applies subst to each element of the incoming slice.
// If at least one element changes, the result is a new slice with
// all the (possibly updated) elements of the incoming slice;
// otherwise the result it nil. The incoming slice is unchanged.
func substList[T comparable](in []T, subst func(T) T) (out []T) {
	for i, t := range in {
		if u := subst(t); u != t {
			if out == nil {
				// lazily allocate a new slice on first substitution
				out = make([]T, len(in))
				copy(out, in)
			}
			out[i] = u
		}
	}
	return
}

func (subst *subster) func_(f *Func) *Func {
	if f != nil {
		if typ := subst.typ(f.typ); typ != f.typ {
			return cloneFunc(f, typ)
		}
	}
	return f
}

func cloneFunc(f *Func, typ Type) *Func {
	copy := *f
	copy.typ = typ
	copy.origin = f.Origin()
	return &copy
}

func (subst *subster) term(t *Term) *Term {
	if typ := subst.typ(t.typ); typ != t.typ {
		return NewTerm(t.tilde, typ)
	}
	return t
}

// replaceRecvType updates any function receivers that have type old to have
// type new. It does not modify the input slice; if modifications are required,
// the input slice and any affected signatures will be copied before mutating.
//
// The resulting out slice contains the updated functions, and copied reports
// if anything was modified.
func replaceRecvType(in []*Func, old, new Type) (out []*Func, copied bool) {
	out = in
	for i, method := range in {
		sig := method.Signature()
		if sig.recv != nil && sig.recv.Type() == old {
			if !copied {
				// Allocate a new methods slice before mutating for the first time.
				// This is defensive, as we may share methods across instantiations of
				// a given interface type if they do not get substituted.
				out = make([]*Func, len(in))
				copy(out, in)
				copied = true
			}
			newsig := *sig
			newsig.recv = cloneVar(sig.recv, new)
			out[i] = cloneFunc(method, &newsig)
		}
	}
	return
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/termlist.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import "strings"

// A termlist represents the type set represented by the union
// t1 ∪ y2 ∪ ... tn of the type sets of the terms t1 to tn.
// A termlist is in normal form if all terms are disjoint.
// termlist operations don't require the operands to be in
// normal form.
type termlist []*term

// allTermlist represents the set of all types.
// It is in normal form.
var allTermlist = termlist{new(term)}

// termSep is the separator used between individual terms.
const termSep = " | "

// String prints the termlist exactly (without normalization).
func (xl termlist) String() string {
	if len(xl) == 0 {
		return "∅"
	}
	var buf strings.Builder
	for i, x := range xl {
		if i > 0 {
			buf.WriteString(termSep)
		}
		buf.WriteString(x.String())
	}
	return buf.String()
}

// isEmpty reports whether the termlist xl represents the empty set of types.
func (xl termlist) isEmpty() bool {
	// If there's a non-nil term, the entire list is not empty.
	// If the termlist is in normal form, this requires at most
	// one iteration.
	for _, x := range xl {
		if x != nil {
			return false
		}
	}
	return true
}

// isAll reports whether the termlist xl represents the set of all types.
func (xl termlist) isAll() bool {
	// If there's a 𝓤 term, the entire list is 𝓤.
	// If the termlist is in normal form, this requires at most
	// one iteration.
	for _, x := range xl {
		if x != nil && x.typ == nil {
			return true
		}
	}
	return false
}

// norm returns the normal form of xl.
func (xl termlist) norm() termlist {
	// Quadratic algorithm, but good enough for now.
	// TODO(gri) fix asymptotic performance
	used := make([]bool, len(xl))
	var rl termlist
	for i, xi := range xl {
		if xi == nil || used[i] {
			continue
		}
		for j := i + 1; j < len(xl); j++ {
			xj := xl[j]
			if xj == nil || used[j] {
				continue
			}
			if u1, u2 := xi.union(xj); u2 == nil {
				// If we encounter a 𝓤 term, the entire list is 𝓤.
				// Exit early.
				// (Note that this is not just an optimization;
				// if we continue, we may end up with a 𝓤 term
				// and other terms and the result would not be
				// in normal form.)
				if u1.typ == nil {
					return allTermlist
				}
				xi = u1
				used[j] = true // xj is now unioned into xi - ignore it in future iterations
			}
		}
		rl = append(rl, xi)
	}
	return rl
}

// union returns the union xl ∪ yl.
func (xl termlist) union(yl termlist) termlist {
	return append(xl, yl...).norm()
}

// intersect returns the intersection xl ∩ yl.
func (xl termlist) intersect(yl termlist) termlist {
	if xl.isEmpty() || yl.isEmpty() {
		return nil
	}

	// Quadratic algorithm, but good enough for now.
	// TODO(gri) fix asymptotic performance
	var rl termlist
	for _, x := range xl {
		for _, y := range yl {
			if r := x.intersect(y); r != nil {
				rl = append(rl, r)
			}
		}
	}
	return rl.norm()
}

// equal reports whether xl and yl represent the same type set.
func (xl termlist) equal(yl termlist) bool {
	// TODO(gri) this should be more efficient
	return xl.subsetOf(yl) && yl.subsetOf(xl)
}

// includes reports whether t ∈ xl.
func (xl termlist) includes(t Type) bool {
	for _, x := range xl {
		if x.includes(t) {
			return true
		}
	}
	return false
}

// supersetOf reports whether y ⊆ xl.
func (xl termlist) supersetOf(y *term) bool {
	for _, x := range xl {
		if y.subsetOf(x) {
			return true
		}
	}
	return false
}

// subsetOf reports whether xl ⊆ yl.
func (xl termlist) subsetOf(yl termlist) bool {
	if yl.isEmpty() {
		return xl.isEmpty()
	}

	// each term x of xl must be a subset of yl
	for _, x := range xl {
		if !yl.supersetOf(x) {
			return false // x is not a subset yl
		}
	}
	return true
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/trie.go ===
```go
// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import "sort"

// A trie[V] maps keys to values V, similar to a map.
// A key is a list of integers; two keys collide if one of them is a prefix of the other.
// For instance, [1, 2] and [1, 2, 3] collide, but [1, 2, 3] and [1, 2, 4] do not.
// If all keys have length 1, a trie degenerates into an ordinary map[int]V.
type trie[V any] map[int]any // map value is either trie[V] (non-leaf node), or V (leaf node)

// insert inserts a value with a key into the trie; key must not be empty.
// If key doesn't collide with any other key in the trie, insert succeeds
// and returns (val, 0). Otherwise, insert fails and returns (alt, n) where
// alt is an unspecified but deterministically chosen value with a colliding
// key and n is the length > 0 of the common key prefix. The trie is not
// changed in this case.
func (tr trie[V]) insert(key []int, val V) (V, int) {
	// A key serves as the path from the trie root to its corresponding value.
	for l, index := range key {
		if v, exists := tr[index]; exists {
			// An entry already exists for this index; check its type to determine collision.
			switch v := v.(type) {
			case trie[V]:
				// Path continues.
				// Must check this case first in case V is any which would act as catch-all.
				tr = v
			case V:
				// Collision: An existing key ends here.
				// This means the existing key is a prefix of (or exactly equal to) key.
				return v, l + 1
			case nil:
				// Handle esoteric case where V is any and val is nil.
				var zero V
				return zero, l + 1
			default:
				panic("trie.insert: invalid entry")
			}
		} else {
			// Path doesn't exist yet, we need to build it.
			if l == len(key)-1 {
				// No prefix collision detected; insert val as a new leaf node.
				tr[index] = val
				return val, 0
			}
			node := make(trie[V])
			tr[index] = node
			tr = node
		}
	}

	if len(key) == 0 {
		panic("trie.insert: key must not be empty")
	}

	// Collision: path ends here, but the trie continues.
	// This means key is a prefix of an existing key.
	// Return a value from the subtrie.
	for len(tr) != 0 {
		// see switch above
		switch v := tr.pickValue().(type) {
		case trie[V]:
			tr = v
		case V:
			return v, len(key)
		case nil:
			var zero V
			return zero, len(key)
		default:
			panic("trie.insert: invalid entry")
		}
	}

	panic("trie.insert: unreachable")
}

// pickValue deterministically picks a value of the trie and returns it.
// Only called in case of a collision.
// The trie must not be empty.
func (tr trie[V]) pickValue() any {
	var keys []int
	for k := range tr {
		keys = append(keys, k)
	}
	sort.Ints(keys) // guarantee deterministic element pick
	return tr[keys[0]]
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/tuple.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

// A Tuple represents an ordered list of variables; a nil *Tuple is a valid (empty) tuple.
// Tuples are used as components of signatures and to represent the type of multiple
// assignments; they are not first class types of Go.
type Tuple struct {
	vars []*Var
}

// NewTuple returns a new tuple for the given variables.
func NewTuple(x ...*Var) *Tuple {
	if len(x) > 0 {
		return &Tuple{vars: x}
	}
	return nil
}

// Len returns the number variables of tuple t.
func (t *Tuple) Len() int {
	if t != nil {
		return len(t.vars)
	}
	return 0
}

// At returns the i'th variable of tuple t.
func (t *Tuple) At(i int) *Var { return t.vars[i] }

func (t *Tuple) Underlying() Type { return t }
func (t *Tuple) String() string   { return TypeString(t, nil) }

```

// === FILE: references!/go/src/cmd/compile/internal/types2/type.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import "cmd/compile/internal/syntax"

// A Type represents a type of Go.
// All types implement the Type interface.
type Type = syntax.Type

```

// === FILE: references!/go/src/cmd/compile/internal/types2/typelists.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import "bytes"

// TypeParamList holds a list of type parameters.
type TypeParamList struct{ tparams []*TypeParam }

// Len returns the number of type parameters in the list.
// It is safe to call on a nil receiver.
func (l *TypeParamList) Len() int { return len(l.list()) }

// At returns the i'th type parameter in the list.
func (l *TypeParamList) At(i int) *TypeParam { return l.tparams[i] }

// list is for internal use where we expect a []*TypeParam.
// TODO(rfindley): list should probably be eliminated: we can pass around a
// TypeParamList instead.
func (l *TypeParamList) list() []*TypeParam {
	if l == nil {
		return nil
	}
	return l.tparams
}

func (l *TypeParamList) String() string {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, tparam := range l.tparams {
		if i > 0 {
			buf.WriteString(", ")
		}
		WriteType(&buf, tparam, nil)
	}
	buf.WriteByte(']')
	return buf.String()
}

// TypeList holds a list of types.
type TypeList struct{ types []Type }

// newTypeList returns a new TypeList with the types in list.
func newTypeList(list []Type) *TypeList {
	if len(list) == 0 {
		return nil
	}
	return &TypeList{list}
}

// Len returns the number of types in the list.
// It is safe to call on a nil receiver.
func (l *TypeList) Len() int { return len(l.list()) }

// At returns the i'th type in the list.
func (l *TypeList) At(i int) Type { return l.types[i] }

// list is for internal use where we expect a []Type.
// TODO(rfindley): list should probably be eliminated: we can pass around a
// TypeList instead.
func (l *TypeList) list() []Type {
	if l == nil {
		return nil
	}
	return l.types
}

func (l *TypeList) String() string {
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, t := range l.types {
		if i > 0 {
			buf.WriteString(", ")
		}
		WriteType(&buf, t, nil)
	}
	buf.WriteByte(']')
	return buf.String()
}

// ----------------------------------------------------------------------------
// Implementation

func bindTParams(list []*TypeParam) *TypeParamList {
	if len(list) == 0 {
		return nil
	}
	for i, typ := range list {
		if typ.index >= 0 {
			panic("type parameter bound more than once")
		}
		typ.index = i
	}
	return &TypeParamList{tparams: list}
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/typeparam.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import "sync/atomic"

// Note: This is a uint32 rather than a uint64 because the
// respective 64 bit atomic instructions are not available
// on all platforms.
var lastID atomic.Uint32

// nextID returns a value increasing monotonically by 1 with
// each call, starting with 1. It may be called concurrently.
func nextID() uint64 { return uint64(lastID.Add(1)) }

// A TypeParam represents the type of a type parameter in a generic declaration.
//
// A TypeParam has a name; use the [TypeParam.Obj] method to access
// its [TypeName] object.
type TypeParam struct {
	check *Checker  // for lazy type bound completion
	id    uint64    // unique id, for debugging only
	obj   *TypeName // corresponding type name
	index int       // type parameter index in source order, starting at 0
	bound Type      // any type, but underlying is eventually *Interface for correct programs (see TypeParam.iface)
}

// NewTypeParam returns a new TypeParam. Type parameters may be set on a Named
// type by calling SetTypeParams. Setting a type parameter on more than one type
// will result in a panic.
//
// The constraint argument can be nil, and set later via SetConstraint. If the
// constraint is non-nil, it must be fully defined.
func NewTypeParam(obj *TypeName, constraint Type) *TypeParam {
	return (*Checker)(nil).newTypeParam(obj, constraint)
}

// check may be nil
func (check *Checker) newTypeParam(obj *TypeName, constraint Type) *TypeParam {
	// Always increment lastID, even if it is not used.
	id := nextID()
	if check != nil {
		check.nextID++
		id = check.nextID
	}
	typ := &TypeParam{check: check, id: id, obj: obj, index: -1, bound: constraint}
	if obj.typ == nil {
		obj.typ = typ
	}
	// iface may mutate typ.bound, so we must ensure that iface() is called
	// at least once before the resulting TypeParam escapes.
	if check != nil {
		check.needsCleanup(typ)
	} else if constraint != nil {
		typ.iface()
	}
	return typ
}

// Obj returns the type name for the type parameter t.
func (t *TypeParam) Obj() *TypeName { return t.obj }

// Index returns the index of the type param within its param list, or -1 if
// the type parameter has not yet been bound to a type.
func (t *TypeParam) Index() int {
	return t.index
}

// Constraint returns the type constraint specified for t.
func (t *TypeParam) Constraint() Type {
	return t.bound
}

// SetConstraint sets the type constraint for t.
//
// It must be called by users of NewTypeParam after the bound's underlying is
// fully defined, and before using the type parameter in any way other than to
// form other types. Once SetConstraint returns the receiver, t is safe for
// concurrent use.
func (t *TypeParam) SetConstraint(bound Type) {
	if bound == nil {
		panic("nil constraint")
	}
	t.bound = bound
	// iface may mutate t.bound (if bound is not an interface), so ensure that
	// this is done before returning.
	t.iface()
}

// Underlying returns the [underlying type] of the type parameter t, which is
// the underlying type of its constraint. This type is always an interface.
//
// [underlying type]: https://go.dev/ref/spec#Underlying_types.
func (t *TypeParam) Underlying() Type {
	return t.iface()
}

func (t *TypeParam) String() string { return TypeString(t, nil) }

// ----------------------------------------------------------------------------
// Implementation

func (t *TypeParam) cleanup() {
	t.iface()
	t.check = nil
}

// iface returns the constraint interface of t.
func (t *TypeParam) iface() *Interface {
	bound := t.bound

	// determine constraint interface
	var ityp *Interface
	switch u := bound.Underlying().(type) {
	case *Basic:
		if !isValid(u) {
			// error is reported elsewhere
			return &emptyInterface
		}
	case *Interface:
		if isTypeParam(bound) {
			// error is reported in Checker.collectTypeParams
			return &emptyInterface
		}
		ityp = u
	}

	// If we don't have an interface, wrap constraint into an implicit interface.
	if ityp == nil {
		ityp = NewInterfaceType(nil, []Type{bound})
		ityp.implicit = true
		t.bound = ityp // update t.bound for next time (optimization)
	}

	// compute type set if necessary
	if ityp.tset == nil {
		// pos is used for tracing output; start with the type parameter position.
		pos := t.obj.pos
		// use the (original or possibly instantiated) type bound position if we have one
		if n := asNamed(bound); n != nil {
			pos = n.obj.pos
		}
		computeInterfaceTypeSet(t.check, pos, ityp)
	}

	return ityp
}

// is calls f with the specific type terms of t's constraint and reports whether
// all calls to f returned true. If there are no specific terms, is
// returns the result of f(nil).
func (t *TypeParam) is(f func(*term) bool) bool {
	return t.iface().typeSet().is(f)
}

// typeset reports whether f(t, y) is true for all (type/underlying type) pairs of the
// specific type terms of t's constraint.
// If there are no specific terms, typeset returns f(nil, nil).
// In any case, typeset is guaranteed to call f at least once.
func (t *TypeParam) typeset(f func(t, u Type) bool) bool {
	return t.iface().typeSet().all(f)
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/typeset.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
	"slices"
	"strings"
)

// ----------------------------------------------------------------------------
// API

// A _TypeSet represents the type set of an interface.
// Because of existing language restrictions, methods can be "factored out"
// from the terms. The actual type set is the intersection of the type set
// implied by the methods and the type set described by the terms and the
// comparable bit. To test whether a type is included in a type set
// ("implements" relation), the type must implement all methods _and_ be
// an element of the type set described by the terms and the comparable bit.
// If the term list describes the set of all types and comparable is true,
// only comparable types are meant; in all other cases comparable is false.
type _TypeSet struct {
	methods    []*Func  // all methods of the interface; sorted by unique ID
	terms      termlist // type terms of the type set
	comparable bool     // invariant: !comparable || terms.isAll()
}

// IsEmpty reports whether s is the empty set.
func (s *_TypeSet) IsEmpty() bool { return s.terms.isEmpty() }

// IsAll reports whether s is the set of all types (corresponding to the empty interface).
func (s *_TypeSet) IsAll() bool { return s.IsMethodSet() && len(s.methods) == 0 }

// IsMethodSet reports whether the interface t is fully described by its method set.
func (s *_TypeSet) IsMethodSet() bool { return !s.comparable && s.terms.isAll() }

// IsComparable reports whether each type in the set is comparable.
func (s *_TypeSet) IsComparable(seen map[Type]bool) bool {
	if s.terms.isAll() {
		return s.comparable
	}
	return s.is(func(t *term) bool {
		return t != nil && comparableType(t.typ, false, seen) == nil
	})
}

// NumMethods returns the number of methods available.
func (s *_TypeSet) NumMethods() int { return len(s.methods) }

// Method returns the i'th method of s for 0 <= i < s.NumMethods().
// The methods are ordered by their unique ID.
func (s *_TypeSet) Method(i int) *Func { return s.methods[i] }

// LookupMethod returns the index of and method with matching package and name, or (-1, nil).
func (s *_TypeSet) LookupMethod(pkg *Package, name string, foldCase bool) (int, *Func) {
	return methodIndex(s.methods, pkg, name, foldCase)
}

func (s *_TypeSet) String() string {
	switch {
	case s.IsEmpty():
		return "∅"
	case s.IsAll():
		return "𝓤"
	}

	hasMethods := len(s.methods) > 0
	hasTerms := s.hasTerms()

	var buf strings.Builder
	buf.WriteByte('{')
	if s.comparable {
		buf.WriteString("comparable")
		if hasMethods || hasTerms {
			buf.WriteString("; ")
		}
	}
	for i, m := range s.methods {
		if i > 0 {
			buf.WriteString("; ")
		}
		buf.WriteString(m.String())
	}
	if hasMethods && hasTerms {
		buf.WriteString("; ")
	}
	if hasTerms {
		buf.WriteString(s.terms.String())
	}
	buf.WriteString("}")
	return buf.String()
}

// ----------------------------------------------------------------------------
// Implementation

// hasTerms reports whether s has specific type terms.
func (s *_TypeSet) hasTerms() bool { return !s.terms.isEmpty() && !s.terms.isAll() }

// subsetOf reports whether s1 ⊆ s2.
func (s1 *_TypeSet) subsetOf(s2 *_TypeSet) bool { return s1.terms.subsetOf(s2.terms) }

// all reports whether f(t, u) is true for each (type/underlying type) pairs in s.
// If s has no specific terms, all calls f(nil, nil).
// In any case, all is guaranteed to call f at least once.
func (s *_TypeSet) all(f func(t, u Type) bool) bool {
	if !s.hasTerms() {
		return f(nil, nil)
	}

	for _, t := range s.terms {
		assert(t.typ != nil)
		// Unalias(x) == x.Underlying() for ~x terms
		u := Unalias(t.typ)
		if !t.tilde {
			u = u.Underlying()
		}
		if debug {
			assert(Identical(u, u.Underlying()))
		}
		if !f(t.typ, u) {
			return false
		}
	}
	return true
}

// is calls f with the specific type terms of s and reports whether
// all calls to f returned true. If there are no specific terms, is
// returns the result of f(nil).
func (s *_TypeSet) is(f func(*term) bool) bool {
	if !s.hasTerms() {
		return f(nil)
	}
	for _, t := range s.terms {
		assert(t.typ != nil)
		if !f(t) {
			return false
		}
	}
	return true
}

// topTypeSet may be used as type set for the empty interface.
var topTypeSet = _TypeSet{terms: allTermlist}

// computeInterfaceTypeSet may be called with check == nil.
func computeInterfaceTypeSet(check *Checker, pos syntax.Pos, ityp *Interface) *_TypeSet {
	if ityp.tset != nil {
		return ityp.tset
	}

	// If the interface is not fully set up yet, the type set will
	// not be complete, which may lead to errors when using the
	// type set (e.g. missing method). Don't compute a partial type
	// set (and don't store it!), so that we still compute the full
	// type set eventually. Instead, return the top type set and
	// let any follow-on errors play out.
	//
	// TODO(gri) Consider recording when this happens and reporting
	// it as an error (but only if there were no other errors so
	// to not have unnecessary follow-on errors).
	if !ityp.complete {
		return &topTypeSet
	}

	if check != nil && check.conf.Trace {
		// Types don't generally have position information.
		// If we don't have a valid pos provided, try to use
		// one close enough.
		if !pos.IsKnown() && len(ityp.methods) > 0 {
			pos = ityp.methods[0].pos
		}

		check.trace(pos, "-- type set for %s", ityp)
		check.indent++
		defer func() {
			check.indent--
			check.trace(pos, "=> %s ", ityp.typeSet())
		}()
	}

	// An infinitely expanding interface (due to a cycle) is detected
	// elsewhere (Checker.validType), so here we simply assume we only
	// have valid interfaces. Mark the interface as complete to avoid
	// infinite recursion if the validType check occurs later for some
	// reason.
	ityp.tset = &_TypeSet{terms: allTermlist} // TODO(gri) is this sufficient?

	var unionSets map[*Union]*_TypeSet
	if check != nil {
		if check.unionTypeSets == nil {
			check.unionTypeSets = make(map[*Union]*_TypeSet)
		}
		unionSets = check.unionTypeSets
	} else {
		unionSets = make(map[*Union]*_TypeSet)
	}

	// Methods of embedded interfaces are collected unchanged; i.e., the identity
	// of a method I.m's Func Object of an interface I is the same as that of
	// the method m in an interface that embeds interface I. On the other hand,
	// if a method is embedded via multiple overlapping embedded interfaces, we
	// don't provide a guarantee which "original m" got chosen for the embedding
	// interface. See also go.dev/issue/34421.
	//
	// If we don't care to provide this identity guarantee anymore, instead of
	// reusing the original method in embeddings, we can clone the method's Func
	// Object and give it the position of a corresponding embedded interface. Then
	// we can get rid of the mpos map below and simply use the cloned method's
	// position.

	var seen objset
	var allMethods []*Func
	mpos := make(map[*Func]syntax.Pos) // method specification or method embedding position, for good error messages
	addMethod := func(pos syntax.Pos, m *Func, explicit bool) {
		switch other := seen.insert(m); {
		case other == nil:
			allMethods = append(allMethods, m)
			mpos[m] = pos
		case explicit:
			if check != nil {
				err := check.newError(DuplicateDecl)
				err.addf(atPos(pos), "duplicate method %s", m.name)
				err.addf(atPos(mpos[other.(*Func)]), "other declaration of method %s", m.name)
				err.report()
			}
		default:
			// We have a duplicate method name in an embedded (not explicitly declared) method.
			// Check method signatures after all types are computed (go.dev/issue/33656).
			// If we're pre-go1.14 (overlapping embeddings are not permitted), report that
			// error here as well (even though we could do it eagerly) because it's the same
			// error message.
			if check != nil {
				check.later(func() {
					if pos.IsKnown() && !check.allowVersion(go1_14) || !Identical(m.typ, other.Type()) {
						err := check.newError(DuplicateDecl)
						err.addf(atPos(pos), "duplicate method %s", m.name)
						err.addf(atPos(mpos[other.(*Func)]), "other declaration of method %s", m.name)
						err.report()
					}
				}).describef(atPos(pos), "duplicate method check for %s", m.name)
			}
		}
	}

	for _, m := range ityp.methods {
		addMethod(m.pos, m, true)
	}

	// collect embedded elements
	allTerms := allTermlist
	allComparable := false
	for i, typ := range ityp.embeddeds {
		// The embedding position is nil for imported interfaces.
		// We don't need to do version checks in those cases.
		var pos syntax.Pos // embedding position
		if ityp.embedPos != nil {
			pos = (*ityp.embedPos)[i]
		}
		var comparable bool
		var terms termlist
		switch u := typ.Underlying().(type) {
		case *Interface:
			// For now we don't permit type parameters as constraints.
			assert(!isTypeParam(typ))
			tset := computeInterfaceTypeSet(check, pos, u)
			// If typ is local, an error was already reported where typ is specified/defined.
			if pos.IsKnown() && check != nil && check.isImportedConstraint(typ) && !check.verifyVersionf(atPos(pos), go1_18, "embedding constraint interface %s", typ) {
				continue
			}
			comparable = tset.comparable
			for _, m := range tset.methods {
				addMethod(pos, m, false) // use embedding position pos rather than m.pos
			}
			terms = tset.terms
		case *Union:
			if pos.IsKnown() && check != nil && !check.verifyVersionf(atPos(pos), go1_18, "embedding interface element %s", u) {
				continue
			}
			tset := computeUnionTypeSet(check, unionSets, pos, u)
			if tset == &invalidTypeSet {
				continue // ignore invalid unions
			}
			assert(!tset.comparable)
			assert(len(tset.methods) == 0)
			terms = tset.terms
		default:
			if !isValid(u) {
				continue
			}
			if pos.IsKnown() && check != nil && !check.verifyVersionf(atPos(pos), go1_18, "embedding non-interface type %s", typ) {
				continue
			}
			terms = termlist{{false, typ}}
		}

		// The type set of an interface is the intersection of the type sets of all its elements.
		// Due to language restrictions, only embedded interfaces can add methods, they are handled
		// separately. Here we only need to intersect the term lists and comparable bits.
		allTerms, allComparable = intersectTermLists(allTerms, allComparable, terms, comparable)
	}

	ityp.tset.comparable = allComparable
	if len(allMethods) != 0 {
		sortMethods(allMethods)
		ityp.tset.methods = allMethods
	}
	ityp.tset.terms = allTerms

	return ityp.tset
}

// TODO(gri) The intersectTermLists function belongs to the termlist implementation.
//           The comparable type set may also be best represented as a term (using
//           a special type).

// intersectTermLists computes the intersection of two term lists and respective comparable bits.
// xcomp, ycomp are valid only if xterms.isAll() and yterms.isAll() respectively.
func intersectTermLists(xterms termlist, xcomp bool, yterms termlist, ycomp bool) (termlist, bool) {
	terms := xterms.intersect(yterms)
	// If one of xterms or yterms is marked as comparable,
	// the result must only include comparable types.
	comp := xcomp || ycomp
	if comp && !terms.isAll() {
		// only keep comparable terms
		i := 0
		for _, t := range terms {
			assert(t.typ != nil)
			if comparableType(t.typ, false /* strictly comparable */, nil) == nil {
				terms[i] = t
				i++
			}
		}
		terms = terms[:i]
		if !terms.isAll() {
			comp = false
		}
	}
	assert(!comp || terms.isAll()) // comparable invariant
	return terms, comp
}

func compareFunc(a, b *Func) int {
	return a.cmp(&b.object)
}

func sortMethods(list []*Func) {
	slices.SortFunc(list, compareFunc)
}

func assertSortedMethods(list []*Func) {
	if !debug {
		panic("assertSortedMethods called outside debug mode")
	}
	if !slices.IsSortedFunc(list, compareFunc) {
		panic("methods not sorted")
	}
}

// invalidTypeSet is a singleton type set to signal an invalid type set
// due to an error. It's also a valid empty type set, so consumers of
// type sets may choose to ignore it.
var invalidTypeSet _TypeSet

// computeUnionTypeSet may be called with check == nil.
// The result is &invalidTypeSet if the union overflows.
func computeUnionTypeSet(check *Checker, unionSets map[*Union]*_TypeSet, pos syntax.Pos, utyp *Union) *_TypeSet {
	if tset, _ := unionSets[utyp]; tset != nil {
		return tset
	}

	// avoid infinite recursion (see also computeInterfaceTypeSet)
	unionSets[utyp] = new(_TypeSet)

	var allTerms termlist
	for _, t := range utyp.terms {
		var terms termlist
		u := t.typ.Underlying()
		if ui, _ := u.(*Interface); ui != nil {
			// For now we don't permit type parameters as constraints.
			assert(!isTypeParam(t.typ))
			terms = computeInterfaceTypeSet(check, pos, ui).terms
		} else if !isValid(u) {
			continue
		} else {
			if t.tilde && !Identical(t.typ, u) {
				// There is no underlying type which is t.typ.
				// The corresponding type set is empty.
				t = nil // ∅ term
			}
			terms = termlist{(*term)(t)}
		}
		// The type set of a union expression is the union
		// of the type sets of each term.
		allTerms = allTerms.union(terms)
		if len(allTerms) > maxTermCount {
			if check != nil {
				check.errorf(atPos(pos), InvalidUnion, "cannot handle more than %d union terms (implementation limitation)", maxTermCount)
			}
			unionSets[utyp] = &invalidTypeSet
			return unionSets[utyp]
		}
	}
	unionSets[utyp].terms = allTerms

	return unionSets[utyp]
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/typestring.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements printing of types.

package types2

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// A Qualifier controls how named package-level objects are printed in
// calls to [TypeString], [ObjectString], and [SelectionString].
//
// These three formatting routines call the Qualifier for each
// package-level object O, and if the Qualifier returns a non-empty
// string p, the object is printed in the form p.O.
// If it returns an empty string, only the object name O is printed.
//
// Using a nil Qualifier is equivalent to using (*[Package]).Path: the
// object is qualified by the import path, e.g., "encoding/json.Marshal".
type Qualifier func(*Package) string

// RelativeTo returns a [Qualifier] that fully qualifies members of
// all packages other than pkg.
func RelativeTo(pkg *Package) Qualifier {
	if pkg == nil {
		return nil
	}
	return func(other *Package) string {
		if pkg == other {
			return "" // same package; unqualified
		}
		return other.Path()
	}
}

// TypeString returns the string representation of typ.
// The [Qualifier] controls the printing of
// package-level objects, and may be nil.
func TypeString(typ Type, qf Qualifier) string {
	var buf bytes.Buffer
	WriteType(&buf, typ, qf)
	return buf.String()
}

// WriteType writes the string representation of typ to buf.
// The [Qualifier] controls the printing of
// package-level objects, and may be nil.
func WriteType(buf *bytes.Buffer, typ Type, qf Qualifier) {
	newTypeWriter(buf, qf).typ(typ)
}

// WriteSignature writes the representation of the signature sig to buf,
// without a leading "func" keyword. The [Qualifier] controls the printing
// of package-level objects, and may be nil.
func WriteSignature(buf *bytes.Buffer, sig *Signature, qf Qualifier) {
	newTypeWriter(buf, qf).signature(sig)
}

type typeWriter struct {
	buf          *bytes.Buffer
	seen         map[Type]bool
	qf           Qualifier
	ctxt         *Context       // if non-nil, we are type hashing
	tparams      *TypeParamList // local type parameters
	paramNames   bool           // if set, write function parameter names, otherwise, write types only
	tpSubscripts bool           // if set, write type parameter indices as subscripts
	pkgInfo      bool           // package-annotate first unexported-type field to avoid confusing type description
}

func newTypeWriter(buf *bytes.Buffer, qf Qualifier) *typeWriter {
	return &typeWriter{buf, make(map[Type]bool), qf, nil, nil, true, false, false}
}

func newTypeHasher(buf *bytes.Buffer, ctxt *Context) *typeWriter {
	assert(ctxt != nil)
	return &typeWriter{buf, make(map[Type]bool), nil, ctxt, nil, false, false, false}
}

func (w *typeWriter) byte(b byte) {
	if w.ctxt != nil {
		if b == ' ' {
			b = '#'
		}
		w.buf.WriteByte(b)
		return
	}
	w.buf.WriteByte(b)
	if b == ',' || b == ';' {
		w.buf.WriteByte(' ')
	}
}

func (w *typeWriter) string(s string) {
	w.buf.WriteString(s)
}

func (w *typeWriter) error(msg string) {
	if w.ctxt != nil {
		panic(msg)
	}
	w.buf.WriteString("<" + msg + ">")
}

func (w *typeWriter) typ(typ Type) {
	if w.seen[typ] {
		w.error("cycle to " + goTypeName(typ))
		return
	}
	w.seen[typ] = true
	defer delete(w.seen, typ)

	switch t := typ.(type) {
	case nil:
		w.error("nil")

	case *Basic:
		// exported basic types go into package unsafe
		// (currently this is just unsafe.Pointer)
		if isExported(t.name) {
			if obj, _ := Unsafe.scope.Lookup(t.name).(*TypeName); obj != nil {
				w.typeName(obj)
				break
			}
		}
		w.string(t.name)

	case *Array:
		w.byte('[')
		w.string(strconv.FormatInt(t.len, 10))
		w.byte(']')
		w.typ(t.elem)

	case *Slice:
		w.string("[]")
		w.typ(t.elem)

	case *Struct:
		w.string("struct{")
		for i, f := range t.fields {
			if i > 0 {
				w.byte(';')
			}

			// If disambiguating one struct for another, look for the first unexported field.
			// Do this first in case of nested structs; tag the first-outermost field.
			pkgAnnotate := false
			if w.qf == nil && w.pkgInfo && !isExported(f.name) {
				// note for embedded types, type name is field name, and "string" etc are lower case hence unexported.
				pkgAnnotate = true
				w.pkgInfo = false // only tag once
			}

			// This doesn't do the right thing for embedded type
			// aliases where we should print the alias name, not
			// the aliased type (see go.dev/issue/44410).
			if !f.embedded {
				w.string(f.name)
				w.byte(' ')
			}
			w.typ(f.typ)
			if pkgAnnotate {
				w.string(" /* package ")
				w.string(f.pkg.Path())
				w.string(" */ ")
			}
			if tag := t.Tag(i); tag != "" {
				w.byte(' ')
				// TODO(gri) If tag contains blanks, replacing them with '#'
				//           in Context.TypeHash may produce another tag
				//           accidentally.
				w.string(strconv.Quote(tag))
			}
		}
		w.byte('}')

	case *Pointer:
		w.byte('*')
		w.typ(t.base)

	case *Tuple:
		w.tuple(t, false)

	case *Signature:
		w.string("func")
		w.signature(t)

	case *Union:
		// Unions only appear as (syntactic) embedded elements
		// in interfaces and syntactically cannot be empty.
		if t.Len() == 0 {
			w.error("empty union")
			break
		}
		for i, t := range t.terms {
			if i > 0 {
				w.string(termSep)
			}
			if t.tilde {
				w.byte('~')
			}
			w.typ(t.typ)
		}

	case *Interface:
		if w.ctxt == nil {
			if t == asNamed(universeComparable.Type()).underlying {
				w.string("interface{comparable}")
				break
			}
		}
		if t.implicit {
			if len(t.methods) == 0 && len(t.embeddeds) == 1 {
				w.typ(t.embeddeds[0])
				break
			}
			// Something's wrong with the implicit interface.
			// Print it as such and continue.
			w.string("/* implicit */ ")
		}
		w.string("interface{")
		first := true
		if w.ctxt != nil {
			w.typeSet(t.typeSet())
		} else {
			for _, m := range t.methods {
				if !first {
					w.byte(';')
				}
				first = false
				w.string(m.name)
				w.signature(m.typ.(*Signature))
			}
			for _, typ := range t.embeddeds {
				if !first {
					w.byte(';')
				}
				first = false
				w.typ(typ)
			}
		}
		w.byte('}')

	case *Map:
		w.string("map[")
		w.typ(t.key)
		w.byte(']')
		w.typ(t.elem)

	case *Chan:
		var s string
		var parens bool
		switch t.dir {
		case SendRecv:
			s = "chan "
			// chan (<-chan T) requires parentheses
			if c, _ := t.elem.(*Chan); c != nil && c.dir == RecvOnly {
				parens = true
			}
		case SendOnly:
			s = "chan<- "
		case RecvOnly:
			s = "<-chan "
		default:
			w.error("unknown channel direction")
		}
		w.string(s)
		if parens {
			w.byte('(')
		}
		w.typ(t.elem)
		if parens {
			w.byte(')')
		}

	case *Named:
		// If hashing, write a unique prefix for t to represent its identity, since
		// named type identity is pointer identity.
		if w.ctxt != nil {
			w.string(strconv.Itoa(w.ctxt.getID(t)))
		}
		w.typeName(t.obj) // when hashing written for readability of the hash only
		if t.inst != nil {
			// instantiated type
			w.typeList(t.inst.targs.list())
		} else if w.ctxt == nil && t.TypeParams().Len() != 0 { // For type hashing, don't need to format the TypeParams
			// parameterized type
			w.tParamList(t.TypeParams().list())
		}

	case *TypeParam:
		if t.obj == nil {
			w.error("unnamed type parameter")
			break
		}
		if i := slices.Index(w.tparams.list(), t); i >= 0 {
			// The names of type parameters that are declared by the type being
			// hashed are not part of the type identity. Replace them with a
			// placeholder indicating their index.
			w.string(fmt.Sprintf("$%d", i))
		} else {
			w.string(t.obj.name)
			if w.tpSubscripts || w.ctxt != nil {
				w.string(subscript(t.id))
			}
			// If the type parameter name is the same as a predeclared object
			// (say int), point out where it is declared to avoid confusing
			// error messages. This doesn't need to be super-elegant; we just
			// need a clear indication that this is not a predeclared name.
			if w.ctxt == nil && Universe.Lookup(t.obj.name) != nil {
				if isTypes2 {
					w.string(fmt.Sprintf(" /* with %s declared at %v */", t.obj.name, t.obj.Pos()))
				} else {
					// Can't print position information because
					// we don't have a token.FileSet accessible.
					w.string("/* type parameter */")
				}
			}
		}

	case *Alias:
		w.typeName(t.obj)
		if list := t.targs.list(); len(list) != 0 {
			// instantiated type
			w.typeList(list)
		} else if w.ctxt == nil && t.TypeParams().Len() != 0 { // For type hashing, don't need to format the TypeParams
			// parameterized type
			w.tParamList(t.TypeParams().list())
		}
		if w.ctxt != nil {
			// TODO(gri) do we need to print the alias type name, too?
			w.typ(Unalias(t.obj.typ))
		}

	default:
		// For externally defined implementations of Type.
		// Note: In this case cycles won't be caught.
		w.string(t.String())
	}
}

// typeSet writes a canonical hash for an interface type set.
func (w *typeWriter) typeSet(s *_TypeSet) {
	assert(w.ctxt != nil)
	first := true
	for _, m := range s.methods {
		if !first {
			w.byte(';')
		}
		first = false
		w.string(m.name)
		w.signature(m.typ.(*Signature))
	}
	switch {
	case s.terms.isAll():
		// nothing to do
	case s.terms.isEmpty():
		w.string(s.terms.String())
	default:
		var termHashes []string
		for _, term := range s.terms {
			// terms are not canonically sorted, so we sort their hashes instead.
			var buf bytes.Buffer
			if term.tilde {
				buf.WriteByte('~')
			}
			newTypeHasher(&buf, w.ctxt).typ(term.typ)
			termHashes = append(termHashes, buf.String())
		}
		slices.Sort(termHashes)
		if !first {
			w.byte(';')
		}
		w.string(strings.Join(termHashes, "|"))
	}
}

func (w *typeWriter) typeList(list []Type) {
	w.byte('[')
	for i, typ := range list {
		if i > 0 {
			w.byte(',')
		}
		w.typ(typ)
	}
	w.byte(']')
}

func (w *typeWriter) tParamList(list []*TypeParam) {
	w.byte('[')
	var prev Type
	for i, tpar := range list {
		// Determine the type parameter and its constraint.
		// list is expected to hold type parameter names,
		// but don't crash if that's not the case.
		if tpar == nil {
			w.error("nil type parameter")
			continue
		}
		if i > 0 {
			if tpar.bound != prev {
				// bound changed - write previous one before advancing
				w.byte(' ')
				w.typ(prev)
			}
			w.byte(',')
		}
		prev = tpar.bound
		w.typ(tpar)
	}
	if prev != nil {
		w.byte(' ')
		w.typ(prev)
	}
	w.byte(']')
}

func (w *typeWriter) typeName(obj *TypeName) {
	w.string(packagePrefix(obj.pkg, w.qf))
	w.string(obj.name)
}

func (w *typeWriter) tuple(tup *Tuple, variadic bool) {
	w.byte('(')
	if tup != nil {
		for i, v := range tup.vars {
			if i > 0 {
				w.byte(',')
			}
			// parameter names are ignored for type identity and thus type hashes
			if w.ctxt == nil && v.name != "" && w.paramNames {
				w.string(v.name)
				w.byte(' ')
			}
			typ := v.typ
			if variadic && i == len(tup.vars)-1 {
				if slice, ok := typ.(*Slice); ok {
					w.string("...")
					w.typ(slice.elem)
				} else {
					// append(slice, str...) entails various special
					// cases, especially in conjunction with generics.
					// str may be:
					// - a string,
					// - a TypeParam whose typeset includes string, or
					// - a named []byte slice type B resulting from
					//   a client instantiating append([]byte, T) at T=B.
					// For such cases we use the irregular notation
					// func([]byte, T...), with the dots after the type.
					w.typ(typ)
					w.string("...")
				}
			} else {
				w.typ(typ)
			}
		}
	}
	w.byte(')')
}

func (w *typeWriter) signature(sig *Signature) {
	if sig.TypeParams().Len() != 0 {
		if w.ctxt != nil {
			assert(w.tparams == nil)
			w.tparams = sig.TypeParams()
			defer func() {
				w.tparams = nil
			}()
		}
		w.tParamList(sig.TypeParams().list())
	}

	w.tuple(sig.params, sig.variadic)

	n := sig.results.Len()
	if n == 0 {
		// no result
		return
	}

	w.byte(' ')
	if n == 1 && (w.ctxt != nil || sig.results.vars[0].name == "") {
		// single unnamed result (if type hashing, name must be ignored)
		w.typ(sig.results.vars[0].typ)
		return
	}

	// multiple or named result(s)
	w.tuple(sig.results, false)
}

// subscript returns the decimal (utf8) representation of x using subscript digits.
func subscript(x uint64) string {
	const w = len("₀") // all digits 0...9 have the same utf8 width
	var buf [32 * w]byte
	i := len(buf)
	for {
		i -= w
		utf8.EncodeRune(buf[i:], '₀'+rune(x%10)) // '₀' == U+2080
		x /= 10
		if x == 0 {
			break
		}
	}
	return string(buf[i:])
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/typeterm.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

// A term describes elementary type sets:
//
//	 ∅:  (*term)(nil)     == ∅                      // set of no types (empty set)
//	 𝓤:  &term{}          == 𝓤                      // set of all types (𝓤niverse)
//	 T:  &term{false, T}  == {T}                    // set of type T
//	~t:  &term{true, t}   == {t' | under(t') == t}  // set of types with underlying type t
type term struct {
	tilde bool // valid if typ != nil
	typ   Type
}

func (x *term) String() string {
	switch {
	case x == nil:
		return "∅"
	case x.typ == nil:
		return "𝓤"
	case x.tilde:
		return "~" + x.typ.String()
	default:
		return x.typ.String()
	}
}

// equal reports whether x and y represent the same type set.
func (x *term) equal(y *term) bool {
	// easy cases
	switch {
	case x == nil || y == nil:
		return x == y
	case x.typ == nil || y.typ == nil:
		return x.typ == y.typ
	}
	// ∅ ⊂ x, y ⊂ 𝓤

	return x.tilde == y.tilde && Identical(x.typ, y.typ)
}

// union returns the union x ∪ y: zero, one, or two non-nil terms.
func (x *term) union(y *term) (_, _ *term) {
	// easy cases
	switch {
	case x == nil && y == nil:
		return nil, nil // ∅ ∪ ∅ == ∅
	case x == nil:
		return y, nil // ∅ ∪ y == y
	case y == nil:
		return x, nil // x ∪ ∅ == x
	case x.typ == nil:
		return x, nil // 𝓤 ∪ y == 𝓤
	case y.typ == nil:
		return y, nil // x ∪ 𝓤 == 𝓤
	}
	// ∅ ⊂ x, y ⊂ 𝓤

	if x.disjoint(y) {
		return x, y // x ∪ y == (x, y) if x ∩ y == ∅
	}
	// x.typ == y.typ

	// ~t ∪ ~t == ~t
	// ~t ∪  T == ~t
	//  T ∪ ~t == ~t
	//  T ∪  T ==  T
	if x.tilde || !y.tilde {
		return x, nil
	}
	return y, nil
}

// intersect returns the intersection x ∩ y.
func (x *term) intersect(y *term) *term {
	// easy cases
	switch {
	case x == nil || y == nil:
		return nil // ∅ ∩ y == ∅ and ∩ ∅ == ∅
	case x.typ == nil:
		return y // 𝓤 ∩ y == y
	case y.typ == nil:
		return x // x ∩ 𝓤 == x
	}
	// ∅ ⊂ x, y ⊂ 𝓤

	if x.disjoint(y) {
		return nil // x ∩ y == ∅ if x ∩ y == ∅
	}
	// x.typ == y.typ

	// ~t ∩ ~t == ~t
	// ~t ∩  T ==  T
	//  T ∩ ~t ==  T
	//  T ∩  T ==  T
	if !x.tilde || y.tilde {
		return x
	}
	return y
}

// includes reports whether t ∈ x.
func (x *term) includes(t Type) bool {
	// easy cases
	switch {
	case x == nil:
		return false // t ∈ ∅ == false
	case x.typ == nil:
		return true // t ∈ 𝓤 == true
	}
	// ∅ ⊂ x ⊂ 𝓤

	u := t
	if x.tilde {
		u = u.Underlying()
	}
	return Identical(x.typ, u)
}

// subsetOf reports whether x ⊆ y.
func (x *term) subsetOf(y *term) bool {
	// easy cases
	switch {
	case x == nil:
		return true // ∅ ⊆ y == true
	case y == nil:
		return false // x ⊆ ∅ == false since x != ∅
	case y.typ == nil:
		return true // x ⊆ 𝓤 == true
	case x.typ == nil:
		return false // 𝓤 ⊆ y == false since y != 𝓤
	}
	// ∅ ⊂ x, y ⊂ 𝓤

	if x.disjoint(y) {
		return false // x ⊆ y == false if x ∩ y == ∅
	}
	// x.typ == y.typ

	// ~t ⊆ ~t == true
	// ~t ⊆ T == false
	//  T ⊆ ~t == true
	//  T ⊆  T == true
	return !x.tilde || y.tilde
}

// disjoint reports whether x ∩ y == ∅.
// x.typ and y.typ must not be nil.
func (x *term) disjoint(y *term) bool {
	if debug && (x.typ == nil || y.typ == nil) {
		panic("invalid argument(s)")
	}
	ux := x.typ
	if y.tilde {
		ux = ux.Underlying()
	}
	uy := y.typ
	if x.tilde {
		uy = uy.Underlying()
	}
	return !Identical(ux, uy)
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/typexpr.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements type-checking of identifiers and type expressions.

package types2

import (
	"cmd/compile/internal/syntax"
	"fmt"
	"go/constant"
	. "internal/types/errors"
	"strings"
)

// ident type-checks identifier e and initializes x with the value or type of e.
// If an error occurred, x.mode is set to invalid.
// If wantType is set, the identifier e is expected to denote a type.
func (check *Checker) ident(x *operand, e *syntax.Name, wantType bool) {
	x.invalidate()
	x.expr = e

	scope, obj := check.lookupScope(e.Value)
	switch obj {
	case nil:
		if e.Value == "_" {
			check.error(e, InvalidBlank, "cannot use _ as value or type")
		} else if isValidName(e.Value) {
			check.errorf(e, UndeclaredName, "undefined: %s", e.Value)
		}
		return
	case universeAny, universeComparable:
		if !check.verifyVersionf(e, go1_18, "predeclared %s", e.Value) {
			return // avoid follow-on errors
		}
	}

	check.recordUse(e, obj)

	// If we want a type but don't have one, stop right here and avoid potential problems
	// with missing underlying types. This also gives better error messages in some cases
	// (see go.dev/issue/65344).
	_, gotType := obj.(*TypeName)
	if !gotType && wantType {
		check.errorf(e, NotAType, "%s (%s) is not a type", obj.Name(), objectKind(obj))

		// avoid "declared but not used" errors
		// (don't use Checker.use - we don't want to evaluate too much)
		if v, _ := obj.(*Var); v != nil && v.pkg == check.pkg /* see Checker.use1 */ {
			check.usedVars[v] = true
		}
		return
	}

	// Type-check the object.
	// Only call Checker.objDecl if the object doesn't have a type yet
	// (in which case we must actually determine it) or the object is a
	// TypeName from the current package and we also want a type (in which case
	// we might detect a cycle which needs to be reported). Otherwise we can skip
	// the call and avoid a possible cycle error in favor of the more informative
	// "not a type/value" error that this function's caller will issue (see
	// go.dev/issue/25790).
	//
	// Note that it is important to avoid calling objDecl on objects from other
	// packages, to avoid races: see issue #69912.
	typ := obj.Type()
	if typ == nil || (gotType && wantType && obj.Pkg() == check.pkg) {
		check.objDecl(obj)
		typ = obj.Type() // type must have been assigned by Checker.objDecl
	}
	assert(typ != nil)

	// The object may have been dot-imported.
	// If so, mark the respective package as used.
	// (This code is only needed for dot-imports. Without them,
	// we only have to mark variables, see *Var case below).
	if pkgName := check.dotImportMap[dotImportKey{scope, obj.Name()}]; pkgName != nil {
		check.usedPkgNames[pkgName] = true
	}

	switch obj := obj.(type) {
	case *PkgName:
		check.errorf(e, InvalidPkgUse, "use of package %s not in selector", obj.name)
		return

	case *Const:
		check.addDeclDep(obj)
		if !isValid(typ) {
			return
		}
		if obj == universeIota {
			if check.iota == nil {
				check.error(e, InvalidIota, "cannot use iota outside constant declaration")
				return
			}
			x.val = check.iota
		} else {
			x.val = obj.val
		}
		assert(x.val != nil)
		x.mode_ = constant_

	case *TypeName:
		x.mode_ = typexpr

	case *Var:
		// It's ok to mark non-local variables, but ignore variables
		// from other packages to avoid potential race conditions with
		// dot-imported variables.
		if obj.pkg == check.pkg {
			check.usedVars[obj] = true
		}
		check.addDeclDep(obj)
		if !isValid(typ) {
			return
		}
		x.mode_ = variable

	case *Func:
		check.addDeclDep(obj)
		x.mode_ = value

	case *Builtin:
		x.id = obj.id
		x.mode_ = builtin

	case *Nil:
		x.mode_ = nilvalue

	default:
		panic("unreachable")
	}

	x.typ_ = typ
}

// typ type-checks the type expression e and returns its type, or Typ[Invalid].
// The type must not be an (uninstantiated) generic type.
func (check *Checker) typ(e syntax.Expr) Type {
	return check.declaredType(e, nil)
}

// varType type-checks the type expression e and returns its type, or Typ[Invalid].
// The type must not be an (uninstantiated) generic type and it must not be a
// constraint interface.
func (check *Checker) varType(e syntax.Expr) Type {
	typ := check.declaredType(e, nil)
	check.validVarType(e, typ)
	return typ
}

// validVarType reports an error if typ is a constraint interface.
// The expression e is used for error reporting, if any.
func (check *Checker) validVarType(e syntax.Expr, typ Type) {
	// If we have a type parameter there's nothing to do.
	if isTypeParam(typ) {
		return
	}

	// We don't want to call typ.Underlying() or complete interfaces while we are in
	// the middle of type-checking parameter declarations that might belong
	// to interface methods. Delay this check to the end of type-checking.
	check.later(func() {
		if t, _ := typ.Underlying().(*Interface); t != nil {
			pos := syntax.StartPos(e)
			tset := computeInterfaceTypeSet(check, pos, t) // TODO(gri) is this the correct position?
			if !tset.IsMethodSet() {
				if tset.comparable {
					check.softErrorf(pos, MisplacedConstraintIface, "cannot use type %s outside a type constraint: interface is (or embeds) comparable", typ)
				} else {
					check.softErrorf(pos, MisplacedConstraintIface, "cannot use type %s outside a type constraint: interface contains type constraints", typ)
				}
			}
		}
	}).describef(e, "check var type %s", typ)
}

// declaredType is like typ but also accepts a type name def.
// If def != nil, e is the type specification for the [Alias] or [Named] type
// named def, and def.typ.fromRHS will be set to the [Type] of e immediately
// after its creation.
func (check *Checker) declaredType(e syntax.Expr, def *TypeName) Type {
	typ := check.typInternal(e, def)
	assert(isTyped(typ))
	if isGeneric(typ) {
		check.errorf(e, WrongTypeArgCount, "cannot use generic type %s without instantiation", typ)
		typ = Typ[Invalid]
	}
	check.recordTypeAndValue(e, typexpr, typ, nil)
	return typ
}

// genericType is like typ but the type must be an (uninstantiated) generic
// type. If cause is non-nil and the type expression was a valid type but not
// generic, cause will be populated with a message describing the error.
//
// Note: If the type expression was invalid and an error was reported before,
// cause will not be populated; thus cause alone cannot be used to determine
// if an error occurred.
func (check *Checker) genericType(e syntax.Expr, cause *string) Type {
	typ := check.typInternal(e, nil)
	assert(isTyped(typ))
	if isValid(typ) && !isGeneric(typ) {
		if cause != nil {
			*cause = check.sprintf("%s is not a generic type", typ)
		}
		typ = Typ[Invalid]
	}
	// TODO(gri) what is the correct call below?
	check.recordTypeAndValue(e, typexpr, typ, nil)
	return typ
}

// goTypeName returns the Go type name for typ and
// removes any occurrences of "types2." from that name.
func goTypeName(typ Type) string {
	return strings.ReplaceAll(fmt.Sprintf("%T", typ), "types2.", "")
}

// typInternal drives type checking of types.
// Must only be called by declaredType or genericType.
func (check *Checker) typInternal(e0 syntax.Expr, def *TypeName) (T Type) {
	if check.conf.Trace {
		check.trace(e0.Pos(), "-- type %s", e0)
		check.indent++
		defer func() {
			check.indent--
			var under Type
			if T != nil {
				// Calling T.Underlying() here may lead to endless instantiations.
				// Test case: type T[P any] *T[P]
				under = safeUnderlying(T)
			}
			if T == under {
				check.trace(e0.Pos(), "=> %s // %s", T, goTypeName(T))
			} else {
				check.trace(e0.Pos(), "=> %s (under = %s) // %s", T, under, goTypeName(T))
			}
		}()
	}

	switch e := e0.(type) {
	case *syntax.BadExpr:
		// ignore - error reported before

	case *syntax.Name:
		var x operand
		check.ident(&x, e, true)

		switch x.mode() {
		case typexpr:
			return x.typ()
		case invalid:
			// ignore - error reported before
		case novalue:
			check.errorf(&x, NotAType, "%s used as type", &x)
		default:
			check.errorf(&x, NotAType, "%s is not a type", &x)
		}

	case *syntax.SelectorExpr:
		var x operand
		check.selector(&x, e, true)

		switch x.mode() {
		case typexpr:
			return x.typ()
		case invalid:
			// ignore - error reported before
		case novalue:
			check.errorf(&x, NotAType, "%s used as type", &x)
		default:
			check.errorf(&x, NotAType, "%s is not a type", &x)
		}

	case *syntax.IndexExpr:
		check.verifyVersionf(e, go1_18, "type instantiation")
		return check.instantiatedType(e.X, syntax.UnpackListExpr(e.Index))

	case *syntax.ParenExpr:
		// Generic types must be instantiated before they can be used in any form.
		// Consequently, generic types cannot be parenthesized.
		return check.declaredType(e.X, def)

	case *syntax.ArrayType:
		typ := new(Array)
		if e.Len != nil {
			typ.len = check.arrayLength(e.Len)
		} else {
			// [...]array
			check.error(e, BadDotDotDotSyntax, "invalid use of [...] array (outside a composite literal)")
			typ.len = -1
		}
		typ.elem = check.varType(e.Elem)
		if typ.len >= 0 {
			return typ
		}
		// report error if we encountered [...]

	case *syntax.SliceType:
		typ := new(Slice)
		typ.elem = check.varType(e.Elem)
		return typ

	case *syntax.DotsType:
		// dots are handled explicitly where they are valid
		check.error(e, InvalidSyntaxTree, "invalid use of ...")

	case *syntax.StructType:
		typ := new(Struct)
		check.structType(typ, e)
		return typ

	case *syntax.Operation:
		if e.Op == syntax.Mul && e.Y == nil {
			typ := new(Pointer)
			typ.base = Typ[Invalid] // avoid nil base in invalid recursive type declaration
			typ.base = check.varType(e.X)
			// If typ.base is invalid, it's unlikely that *base is particularly
			// useful - even a valid dereferenciation will lead to an invalid
			// type again, and in some cases we get unexpected follow-on errors
			// (e.g., go.dev/issue/49005). Return an invalid type instead.
			if !isValid(typ.base) {
				return Typ[Invalid]
			}
			return typ
		}

		check.errorf(e0, NotAType, "%s is not a type", e0)
		check.use(e0)

	case *syntax.FuncType:
		typ := new(Signature)
		check.funcType(typ, nil, nil, e)
		return typ

	case *syntax.InterfaceType:
		typ := check.newInterface()
		check.interfaceType(typ, e, def)
		return typ

	case *syntax.MapType:
		typ := new(Map)
		typ.key = check.varType(e.Key)
		typ.elem = check.varType(e.Value)

		// spec: "The comparison operators == and != must be fully defined
		// for operands of the key type; thus the key type must not be a
		// function, map, or slice."
		//
		// Delay this check because it requires fully setup types;
		// it is safe to continue in any case (was go.dev/issue/6667).
		check.later(func() {
			if !Comparable(typ.key) {
				var why string
				if isTypeParam(typ.key) {
					why = " (missing comparable constraint)"
				}
				check.errorf(e.Key, IncomparableMapKey, "invalid map key type %s%s", typ.key, why)
			}
		}).describef(e.Key, "check map key %s", typ.key)

		return typ

	case *syntax.ChanType:
		typ := new(Chan)

		dir := SendRecv
		switch e.Dir {
		case 0:
			// nothing to do
		case syntax.SendOnly:
			dir = SendOnly
		case syntax.RecvOnly:
			dir = RecvOnly
		default:
			check.errorf(e, InvalidSyntaxTree, "unknown channel direction %d", e.Dir)
			// ok to continue
		}

		typ.dir = dir
		typ.elem = check.varType(e.Elem)
		return typ

	default:
		check.errorf(e0, NotAType, "%s is not a type", e0)
		check.use(e0)
	}

	typ := Typ[Invalid]
	return typ
}

func (check *Checker) instantiatedType(x syntax.Expr, xlist []syntax.Expr) (res Type) {
	if check.conf.Trace {
		check.trace(x.Pos(), "-- instantiating type %s with %s", x, xlist)
		check.indent++
		defer func() {
			check.indent--
			// Don't format the underlying here. It will always be nil.
			check.trace(x.Pos(), "=> %s", res)
		}()
	}

	var cause string
	typ := check.genericType(x, &cause)
	if cause != "" {
		check.errorf(x, NotAGenericType, invalidOp+"%s%s (%s)", x, xlist, cause)
	}
	if !isValid(typ) {
		return typ // error already reported
	}
	// typ must be a generic Alias or Named type (but not a *Signature)
	if _, ok := typ.(*Signature); ok {
		panic("unexpected generic signature")
	}
	gtyp := typ.(genericType)

	// evaluate arguments
	targs := check.typeList(xlist)
	if targs == nil {
		return Typ[Invalid]
	}

	// create instance
	// The instance is not generic anymore as it has type arguments, but unless
	// instantiation failed, it still satisfies the genericType interface because
	// it has type parameters, too.
	ityp := check.instance(x.Pos(), gtyp, targs, nil, check.context())
	inst, _ := ityp.(genericType)
	if inst == nil {
		return Typ[Invalid]
	}

	// For Named types, orig.tparams may not be set up, so we need to do expansion later.
	check.later(func() {
		// This is an instance from the source, not from recursive substitution,
		// and so it must be resolved during type-checking so that we can report
		// errors.
		check.recordInstance(x, targs, inst)

		name := inst.(interface{ Obj() *TypeName }).Obj().name
		tparams := inst.TypeParams().list()
		if check.validateTArgLen(x.Pos(), name, len(tparams), len(targs)) {
			// check type constraints
			if i, err := check.verify(x.Pos(), inst.TypeParams().list(), targs, check.context()); err != nil {
				// best position for error reporting
				pos := x.Pos()
				if i < len(xlist) {
					pos = syntax.StartPos(xlist[i])
				}
				check.softErrorf(pos, InvalidTypeArg, "%s", err)
			} else {
				check.mono.recordInstance(check.pkg, x.Pos(), tparams, targs, xlist)
			}
		}
	}).describef(x, "verify instantiation %s", inst)

	return inst
}

// arrayLength type-checks the array length expression e
// and returns the constant length >= 0, or a value < 0
// to indicate an error (and thus an unknown length).
func (check *Checker) arrayLength(e syntax.Expr) int64 {
	// If e is an identifier, the array declaration might be an
	// attempt at a parameterized type declaration with missing
	// constraint. Provide an error message that mentions array
	// length.
	if name, _ := e.(*syntax.Name); name != nil {
		obj := check.lookup(name.Value)
		if obj == nil {
			check.errorf(name, InvalidArrayLen, "undefined array length %s or missing type constraint", name.Value)
			return -1
		}
		if _, ok := obj.(*Const); !ok {
			check.errorf(name, InvalidArrayLen, "invalid array length %s", name.Value)
			return -1
		}
	}

	var x operand
	check.expr(nil, &x, e)
	if x.mode() != constant_ {
		if x.isValid() {
			check.errorf(&x, InvalidArrayLen, "array length %s must be constant", &x)
		}
		return -1
	}

	if isUntyped(x.typ()) || isInteger(x.typ()) {
		if val := constant.ToInt(x.val); val.Kind() == constant.Int {
			if representableConst(val, check, Typ[Int], nil) {
				if n, ok := constant.Int64Val(val); ok && n >= 0 {
					return n
				}
			}
		}
	}

	var msg string
	if isInteger(x.typ()) {
		msg = "invalid array length %s"
	} else {
		msg = "array length %s must be integer"
	}
	check.errorf(&x, InvalidArrayLen, msg, &x)
	return -1
}

// typeList provides the list of types corresponding to the incoming expression list.
// If an error occurred, the result is nil, but all list elements were type-checked.
func (check *Checker) typeList(list []syntax.Expr) []Type {
	res := make([]Type, len(list)) // res != nil even if len(list) == 0
	for i, x := range list {
		t := check.varType(x)
		if !isValid(t) {
			res = nil
		}
		if res != nil {
			res[i] = t
		}
	}
	return res
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/under.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import "iter"

// If typ is a type parameter, underIs returns the result of typ.underIs(f).
// Otherwise, underIs returns the result of f(typ.Underlying()).
func underIs(typ Type, f func(Type) bool) bool {
	return all(typ, func(_, u Type) bool {
		return f(u)
	})
}

// all reports whether f(t, u) is true for all (type/underlying type)
// pairs in the typeset of t. See [typeset] for details of sequence.
func all(t Type, f func(t, u Type) bool) bool {
	if p, _ := Unalias(t).(*TypeParam); p != nil {
		return p.typeset(f)
	}
	return f(t, t.Underlying())
}

// typeset is an iterator over the (type/underlying type) pairs of the
// specific type terms of the type set implied by t.
// If t is a type parameter, the implied type set is the type set of t's constraint.
// In this case, if there are no specific terms, the iterator produces (nil, nil).
// If t is not a type parameter, the implied type set consists of just t.
// In any case, typeset is guaranteed to produce at least one set of results.
func typeset(t Type) iter.Seq2[Type, Type] {
	return func(yield func(t, u Type) bool) {
		all(t, yield)
	}
}

// A typeError describes a type error.
type typeError struct {
	format_ string
	args    []any
}

var emptyTypeError typeError

func typeErrorf(format string, args ...any) *typeError {
	if format == "" {
		return &emptyTypeError
	}
	return &typeError{format, args}
}

// format formats a type error as a string.
// check may be nil.
func (err *typeError) format(check *Checker) string {
	return check.sprintf(err.format_, err.args...)
}

// If t is a type parameter, cond is nil, and t's type set contains no channel types,
// commonUnder returns the common underlying type of all types in t's type set if
// it exists, or nil and a type error otherwise.
//
// If t is a type parameter, cond is nil, and there are channel types, t's type set
// must only contain channel types, they must all have the same element types,
// channel directions must not conflict, and commonUnder returns one of the most
// restricted channels. Otherwise, the function returns nil and a type error.
//
// If cond != nil, each pair (t, u) of type and underlying type in t's type set
// must satisfy the condition expressed by cond. If the result of cond is != nil,
// commonUnder returns nil and the type error reported by cond.
// Note that cond is called before any other conditions are checked; specifically
// cond may be called with (nil, nil) if the type set contains no specific types.
//
// If t is not a type parameter, commonUnder behaves as if t was a type parameter
// with the single type t in its type set.
func commonUnder(t Type, cond func(t, u Type) *typeError) (Type, *typeError) {
	var ct, cu Type // type and respective common underlying type
	for t, u := range typeset(t) {
		if cond != nil {
			if err := cond(t, u); err != nil {
				return nil, err
			}
		}

		if u == nil {
			return nil, typeErrorf("no specific type")
		}

		// If this is the first type we're seeing, we're done.
		if cu == nil {
			ct, cu = t, u
			continue
		}

		// If we've seen a channel before, and we have a channel now, they must be compatible.
		if chu, _ := cu.(*Chan); chu != nil {
			if ch, _ := u.(*Chan); ch != nil {
				if !Identical(chu.elem, ch.elem) {
					return nil, typeErrorf("channels %s and %s have different element types", ct, t)
				}
				// If we have different channel directions, keep the restricted one
				// and complain if they conflict.
				switch {
				case chu.dir == ch.dir:
					// nothing to do
				case chu.dir == SendRecv:
					ct, cu = t, u // switch to restricted channel
				case ch.dir != SendRecv:
					return nil, typeErrorf("channels %s and %s have conflicting directions", ct, t)
				}
				continue
			}
		}

		// Otherwise, the current type must have the same underlying type as all previous types.
		if !Identical(cu, u) {
			return nil, typeErrorf("%s and %s have different underlying types", ct, t)
		}
	}
	return cu, nil
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/unify.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements type unification.
//
// Type unification attempts to make two types x and y structurally
// equivalent by determining the types for a given list of (bound)
// type parameters which may occur within x and y. If x and y are
// structurally different (say []T vs chan T), or conflicting
// types are determined for type parameters, unification fails.
// If unification succeeds, as a side-effect, the types of the
// bound type parameters may be determined.
//
// Unification typically requires multiple calls u.unify(x, y) to
// a given unifier u, with various combinations of types x and y.
// In each call, additional type parameter types may be determined
// as a side effect and recorded in u.
// If a call fails (returns false), unification fails.
//
// In the unification context, structural equivalence of two types
// ignores the difference between a defined type and its underlying
// type if one type is a defined type and the other one is not.
// It also ignores the difference between an (external, unbound)
// type parameter and its core type.
// If two types are not structurally equivalent, they cannot be Go
// identical types. On the other hand, if they are structurally
// equivalent, they may be Go identical or at least assignable, or
// they may be in the type set of a constraint.
// Whether they indeed are identical or assignable is determined
// upon instantiation and function argument passing.

package types2

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

const (
	// Upper limit for recursion depth. Used to catch infinite recursions
	// due to implementation issues (e.g., see issues go.dev/issue/48619, go.dev/issue/48656).
	unificationDepthLimit = 50

	// Whether to panic when unificationDepthLimit is reached.
	// If disabled, a recursion depth overflow results in a (quiet)
	// unification failure.
	panicAtUnificationDepthLimit = true

	// If enableCoreTypeUnification is set, unification will consider
	// the core types, if any, of non-local (unbound) type parameters.
	enableCoreTypeUnification = true

	// If traceInference is set, unification will print a trace of its operation.
	// Interpretation of trace:
	//   x ≡ y    attempt to unify types x and y
	//   p ➞ y    type parameter p is set to type y (p is inferred to be y)
	//   p ⇄ q    type parameters p and q match (p is inferred to be q and vice versa)
	//   x ≢ y    types x and y cannot be unified
	//   [p, q, ...] ➞ [x, y, ...]    mapping from type parameters to types
	traceInference = false
)

// A unifier maintains a list of type parameters and
// corresponding types inferred for each type parameter.
// A unifier is created by calling newUnifier.
type unifier struct {
	check *Checker
	// handles maps each type parameter to its inferred type through
	// an indirection *Type called (inferred type) "handle".
	// Initially, each type parameter has its own, separate handle,
	// with a nil (i.e., not yet inferred) type.
	// After a type parameter P is unified with a type parameter Q,
	// P and Q share the same handle (and thus type). This ensures
	// that inferring the type for a given type parameter P will
	// automatically infer the same type for all other parameters
	// unified (joined) with P.
	handles                  map[*TypeParam]*Type
	depth                    int  // recursion depth during unification
	enableInterfaceInference bool // use shared methods for better inference
}

// newUnifier returns a new unifier initialized with the given type parameter
// and corresponding type argument lists. The type argument list may be shorter
// than the type parameter list, and it may contain nil types. Matching type
// parameters and arguments must have the same index.
func newUnifier(check *Checker, tparams []*TypeParam, targs []Type, enableInterfaceInference bool) *unifier {
	assert(len(tparams) >= len(targs))
	handles := make(map[*TypeParam]*Type, len(tparams))
	// Allocate all handles up-front: in a correct program, all type parameters
	// must be resolved and thus eventually will get a handle.
	// Also, sharing of handles caused by unified type parameters is rare and
	// so it's ok to not optimize for that case (and delay handle allocation).
	for i, x := range tparams {
		var t Type
		if i < len(targs) {
			t = targs[i]
		}
		handles[x] = &t
	}
	return &unifier{check, handles, 0, enableInterfaceInference}
}

// unifyMode controls the behavior of the unifier.
type unifyMode uint

const (
	// If assign is set, we are unifying types involved in an assignment:
	// they may match inexactly at the top, but element types must match
	// exactly.
	assign unifyMode = 1 << iota

	// If exact is set, types unify if they are identical (or can be
	// made identical with suitable arguments for type parameters).
	// Otherwise, a named type and a type literal unify if their
	// underlying types unify, channel directions are ignored, and
	// if there is an interface, the other type must implement the
	// interface.
	exact
)

func (m unifyMode) String() string {
	switch m {
	case 0:
		return "inexact"
	case assign:
		return "assign"
	case exact:
		return "exact"
	case assign | exact:
		return "assign, exact"
	}
	return fmt.Sprintf("mode %d", m)
}

// unify attempts to unify x and y and reports whether it succeeded.
// As a side-effect, types may be inferred for type parameters.
// The mode parameter controls how types are compared.
func (u *unifier) unify(x, y Type, mode unifyMode) bool {
	return u.nify(x, y, mode, nil)
}

func (u *unifier) tracef(format string, args ...any) {
	// TODO(gri) consider adjusting this to use Checker.trace
	fmt.Println(strings.Repeat(".  ", u.depth) + sprintf(nil, true, format, args...))
}

// String returns a string representation of the current mapping
// from type parameters to types.
func (u *unifier) String() string {
	// sort type parameters for reproducible strings
	tparams := make(typeParamsById, len(u.handles))
	i := 0
	for tpar := range u.handles {
		tparams[i] = tpar
		i++
	}
	sort.Sort(tparams)

	var buf bytes.Buffer
	w := newTypeWriter(&buf, nil)
	w.byte('[')
	for i, x := range tparams {
		if i > 0 {
			w.string(", ")
		}
		w.typ(x)
		w.string(": ")
		w.typ(u.at(x))
	}
	w.byte(']')
	return buf.String()
}

type typeParamsById []*TypeParam

func (s typeParamsById) Len() int           { return len(s) }
func (s typeParamsById) Less(i, j int) bool { return s[i].id < s[j].id }
func (s typeParamsById) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// join unifies the given type parameters x and y.
// If both type parameters already have a type associated with them
// and they are not joined, join fails and returns false.
func (u *unifier) join(x, y *TypeParam) bool {
	if traceInference {
		u.tracef("%s ⇄ %s", x, y)
	}
	switch hx, hy := u.handles[x], u.handles[y]; {
	case hx == hy:
		// Both type parameters already share the same handle. Nothing to do.
	case *hx != nil && *hy != nil:
		// Both type parameters have (possibly different) inferred types. Cannot join.
		return false
	case *hx != nil:
		// Only type parameter x has an inferred type. Use handle of x.
		u.setHandle(y, hx)
	// This case is treated like the default case.
	// case *hy != nil:
	// 	// Only type parameter y has an inferred type. Use handle of y.
	//	u.setHandle(x, hy)
	default:
		// Neither type parameter has an inferred type. Use handle of y.
		u.setHandle(x, hy)
	}
	return true
}

// asBoundTypeParam returns x.(*TypeParam) if x is a type parameter recorded with u.
// Otherwise, the result is nil.
func (u *unifier) asBoundTypeParam(x Type) *TypeParam {
	if x, _ := Unalias(x).(*TypeParam); x != nil {
		if _, found := u.handles[x]; found {
			return x
		}
	}
	return nil
}

// setHandle sets the handle for type parameter x
// (and all its joined type parameters) to h.
func (u *unifier) setHandle(x *TypeParam, h *Type) {
	hx := u.handles[x]
	assert(hx != nil)
	for y, hy := range u.handles {
		if hy == hx {
			u.handles[y] = h
		}
	}
}

// at returns the (possibly nil) type for type parameter x.
func (u *unifier) at(x *TypeParam) Type {
	return *u.handles[x]
}

// set sets the type t for type parameter x;
// t must not be nil.
func (u *unifier) set(x *TypeParam, t Type) {
	assert(t != nil)
	if traceInference {
		u.tracef("%s ➞ %s", x, t)
	}
	*u.handles[x] = t
}

// unknowns returns the number of type parameters for which no type has been set yet.
func (u *unifier) unknowns() int {
	n := 0
	for _, h := range u.handles {
		if *h == nil {
			n++
		}
	}
	return n
}

// inferred returns the list of inferred types for the given type parameter list.
// The result is never nil and has the same length as tparams; result types that
// could not be inferred are nil. Corresponding type parameters and result types
// have identical indices.
func (u *unifier) inferred(tparams []*TypeParam) []Type {
	list := make([]Type, len(tparams))
	for i, x := range tparams {
		list[i] = u.at(x)
	}
	return list
}

// asInterface returns the underlying type of x as an interface if
// it is a non-type parameter interface. Otherwise it returns nil.
func asInterface(x Type) (i *Interface) {
	if _, ok := Unalias(x).(*TypeParam); !ok {
		i, _ = x.Underlying().(*Interface)
	}
	return i
}

// nify implements the core unification algorithm which is an
// adapted version of Checker.identical. For changes to that
// code the corresponding changes should be made here.
// Must not be called directly from outside the unifier.
func (u *unifier) nify(x, y Type, mode unifyMode, p *ifacePair) (result bool) {
	u.depth++
	if traceInference {
		u.tracef("%s ≡ %s\t// %s", x, y, mode)
	}
	defer func() {
		if traceInference && !result {
			u.tracef("%s ≢ %s", x, y)
		}
		u.depth--
	}()

	// nothing to do if x == y
	if x == y || Unalias(x) == Unalias(y) {
		return true
	}

	// Stop gap for cases where unification fails.
	if u.depth > unificationDepthLimit {
		if traceInference {
			u.tracef("depth %d >= %d", u.depth, unificationDepthLimit)
		}
		if panicAtUnificationDepthLimit {
			panic("unification reached recursion depth limit")
		}
		return false
	}

	// Unification is symmetric, so we can swap the operands.
	// Ensure that if we have at least one
	// - defined type, make sure one is in y
	// - type parameter recorded with u, make sure one is in x
	if asNamed(x) != nil || u.asBoundTypeParam(y) != nil {
		if traceInference {
			u.tracef("%s ≡ %s\t// swap", y, x)
		}
		x, y = y, x
	}

	// Unification will fail if we match a defined type against a type literal.
	// If we are matching types in an assignment, at the top-level, types with
	// the same type structure are permitted as long as at least one of them
	// is not a defined type. To accommodate for that possibility, we continue
	// unification with the underlying type of a defined type if the other type
	// is a type literal. This is controlled by the exact unification mode.
	// We also continue if the other type is a basic type because basic types
	// are valid underlying types and may appear as core types of type constraints.
	// If we exclude them, inferred defined types for type parameters may not
	// match against the core types of their constraints (even though they might
	// correctly match against some of the types in the constraint's type set).
	// Finally, if unification (incorrectly) succeeds by matching the underlying
	// type of a defined type against a basic type (because we include basic types
	// as type literals here), and if that leads to an incorrectly inferred type,
	// we will fail at function instantiation or argument assignment time.
	//
	// If we have at least one defined type, there is one in y.
	if ny := asNamed(y); mode&exact == 0 && ny != nil && isTypeLit(x) && !(u.enableInterfaceInference && IsInterface(x)) {
		if traceInference {
			u.tracef("%s ≡ under %s", x, ny)
		}
		y = ny.Underlying()
		// Per the spec, a defined type cannot have an underlying type
		// that is a type parameter.
		assert(!isTypeParam(y))
		// x and y may be identical now
		if x == y || Unalias(x) == Unalias(y) {
			return true
		}
	}

	// Cases where at least one of x or y is a type parameter recorded with u.
	// If we have at least one type parameter, there is one in x.
	// If we have exactly one type parameter, because it is in x,
	// isTypeLit(x) is false and y was not changed above. In other
	// words, if y was a defined type, it is still a defined type
	// (relevant for the logic below).
	switch px, py := u.asBoundTypeParam(x), u.asBoundTypeParam(y); {
	case px != nil && py != nil:
		// both x and y are type parameters
		if u.join(px, py) {
			return true
		}
		// both x and y have an inferred type - they must match
		return u.nify(u.at(px), u.at(py), mode, p)

	case px != nil:
		// x is a type parameter, y is not
		if x := u.at(px); x != nil {
			// x has an inferred type which must match y
			if u.nify(x, y, mode, p) {
				// We have a match, possibly through underlying types.
				xi := asInterface(x)
				yi := asInterface(y)
				xn := asNamed(x) != nil
				yn := asNamed(y) != nil
				// If we have two interfaces, what to do depends on
				// whether they are named and their method sets.
				if xi != nil && yi != nil {
					// Both types are interfaces.
					// If both types are defined types, they must be identical
					// because unification doesn't know which type has the "right" name.
					if xn && yn {
						return Identical(x, y)
					}
					// In all other cases, the method sets must match.
					// The types unified so we know that corresponding methods
					// match and we can simply compare the number of methods.
					// TODO(gri) We may be able to relax this rule and select
					// the more general interface. But if one of them is a defined
					// type, it's not clear how to choose and whether we introduce
					// an order dependency or not. Requiring the same method set
					// is conservative.
					if len(xi.typeSet().methods) != len(yi.typeSet().methods) {
						return false
					}
				} else if xi != nil || yi != nil {
					// One but not both of them are interfaces.
					// In this case, either x or y could be viable matches for the corresponding
					// type parameter, which means choosing either introduces an order dependence.
					// Therefore, we must fail unification (go.dev/issue/60933).
					return false
				}
				// If we have inexact unification and one of x or y is a defined type, select the
				// defined type. This ensures that in a series of types, all matching against the
				// same type parameter, we infer a defined type if there is one, independent of
				// order. Type inference or assignment may fail, which is ok.
				// Selecting a defined type, if any, ensures that we don't lose the type name;
				// and since we have inexact unification, a value of equally named or matching
				// undefined type remains assignable (go.dev/issue/43056).
				//
				// Similarly, if we have inexact unification and there are no defined types but
				// channel types, select a directed channel, if any. This ensures that in a series
				// of unnamed types, all matching against the same type parameter, we infer the
				// directed channel if there is one, independent of order.
				// Selecting a directional channel, if any, ensures that a value of another
				// inexactly unifying channel type remains assignable (go.dev/issue/62157).
				//
				// If we have multiple defined channel types, they are either identical or we
				// have assignment conflicts, so we can ignore directionality in this case.
				//
				// If we have defined and literal channel types, a defined type wins to avoid
				// order dependencies.
				if mode&exact == 0 {
					switch {
					case xn:
						// x is a defined type: nothing to do.
					case yn:
						// x is not a defined type and y is a defined type: select y.
						u.set(px, y)
					default:
						// Neither x nor y are defined types.
						if yc, _ := y.Underlying().(*Chan); yc != nil && yc.dir != SendRecv {
							// y is a directed channel type: select y.
							u.set(px, y)
						}
					}
				}
				return true
			}
			return false
		}
		// otherwise, infer type from y
		u.set(px, y)
		return true
	}

	// x != y if we get here
	assert(x != y && Unalias(x) != Unalias(y))

	// If u.EnableInterfaceInference is set and we don't require exact unification,
	// if both types are interfaces, one interface must have a subset of the
	// methods of the other and corresponding method signatures must unify.
	// If only one type is an interface, all its methods must be present in the
	// other type and corresponding method signatures must unify.
	if u.enableInterfaceInference && mode&exact == 0 {
		// One or both interfaces may be defined types.
		// Look under the name, but not under type parameters (go.dev/issue/60564).
		xi := asInterface(x)
		yi := asInterface(y)
		// If we have two interfaces, check the type terms for equivalence,
		// and unify common methods if possible.
		if xi != nil && yi != nil {
			xset := xi.typeSet()
			yset := yi.typeSet()
			if xset.comparable != yset.comparable {
				return false
			}
			// For now we require terms to be equal.
			// We should be able to relax this as well, eventually.
			if !xset.terms.equal(yset.terms) {
				return false
			}
			// Interface types are the only types where cycles can occur
			// that are not "terminated" via named types; and such cycles
			// can only be created via method parameter types that are
			// anonymous interfaces (directly or indirectly) embedding
			// the current interface. Example:
			//
			//    type T interface {
			//        m() interface{T}
			//    }
			//
			// If two such (differently named) interfaces are compared,
			// endless recursion occurs if the cycle is not detected.
			//
			// If x and y were compared before, they must be equal
			// (if they were not, the recursion would have stopped);
			// search the ifacePair stack for the same pair.
			//
			// This is a quadratic algorithm, but in practice these stacks
			// are extremely short (bounded by the nesting depth of interface
			// type declarations that recur via parameter types, an extremely
			// rare occurrence). An alternative implementation might use a
			// "visited" map, but that is probably less efficient overall.
			q := &ifacePair{xi, yi, p}
			for p != nil {
				if p.identical(q) {
					return true // same pair was compared before
				}
				p = p.prev
			}
			// The method set of x must be a subset of the method set
			// of y or vice versa, and the common methods must unify.
			xmethods := xset.methods
			ymethods := yset.methods
			// The smaller method set must be the subset, if it exists.
			if len(xmethods) > len(ymethods) {
				xmethods, ymethods = ymethods, xmethods
			}
			// len(xmethods) <= len(ymethods)
			// Collect the ymethods in a map for quick lookup.
			ymap := make(map[string]*Func, len(ymethods))
			for _, ym := range ymethods {
				ymap[ym.Id()] = ym
			}
			// All xmethods must exist in ymethods and corresponding signatures must unify.
			for _, xm := range xmethods {
				if ym := ymap[xm.Id()]; ym == nil || !u.nify(xm.typ, ym.typ, exact, p) {
					return false
				}
			}
			return true
		}

		// We don't have two interfaces. If we have one, make sure it's in xi.
		if yi != nil {
			xi = yi
			y = x
		}

		// If we have one interface, at a minimum each of the interface methods
		// must be implemented and thus unify with a corresponding method from
		// the non-interface type, otherwise unification fails.
		if xi != nil {
			// All xi methods must exist in y and corresponding signatures must unify.
			// A generic method never satisfies an interface method, so fail rather
			// than unify ym's own type parameter into an inference variable.
			xmethods := xi.typeSet().methods
			for _, xm := range xmethods {
				obj, _, _ := LookupFieldOrMethod(y, false, xm.pkg, xm.name)
				ym, _ := obj.(*Func)
				if ym == nil {
					return false
				}
				u.check.objDecl(ym) // ensure fully set-up signature
				if ym.Signature().TypeParams() != nil || !u.nify(xm.typ, ym.typ, exact, p) {
					return false
				}
			}
			return true
		}
	}

	// Unless we have exact unification, neither x nor y are interfaces now.
	// Except for unbound type parameters (see below), x and y must be structurally
	// equivalent to unify.

	// If we get here and x or y is a type parameter, they are unbound
	// (not recorded with the unifier).
	// Ensure that if we have at least one type parameter, it is in x
	// (the earlier swap checks for _recorded_ type parameters only).
	// This ensures that the switch switches on the type parameter.
	//
	// TODO(gri) Factor out type parameter handling from the switch.
	if isTypeParam(y) {
		if traceInference {
			u.tracef("%s ≡ %s\t// swap", y, x)
		}
		x, y = y, x
	}

	// Type elements (array, slice, etc. elements) use emode for unification.
	// Element types must match exactly if the types are used in an assignment.
	emode := mode
	if mode&assign != 0 {
		emode |= exact
	}

	// Continue with unaliased types but don't lose original alias names, if any (go.dev/issue/67628).
	xorig, x := x, Unalias(x)
	yorig, y := y, Unalias(y)

	switch x := x.(type) {
	case *Basic:
		// Basic types are singletons except for the rune and byte
		// aliases, thus we cannot solely rely on the x == y check
		// above. See also comment in TypeName.IsAlias.
		if y, ok := y.(*Basic); ok {
			return x.kind == y.kind
		}

	case *Array:
		// Two array types unify if they have the same array length
		// and their element types unify.
		if y, ok := y.(*Array); ok {
			// If one or both array lengths are unknown (< 0) due to some error,
			// assume they are the same to avoid spurious follow-on errors.
			return (x.len < 0 || y.len < 0 || x.len == y.len) && u.nify(x.elem, y.elem, emode, p)
		}

	case *Slice:
		// Two slice types unify if their element types unify.
		if y, ok := y.(*Slice); ok {
			return u.nify(x.elem, y.elem, emode, p)
		}

	case *Struct:
		// Two struct types unify if they have the same sequence of fields,
		// and if corresponding fields have the same names, their (field) types unify,
		// and they have identical tags. Two embedded fields are considered to have the same
		// name. Lower-case field names from different packages are always different.
		if y, ok := y.(*Struct); ok {
			if x.NumFields() == y.NumFields() {
				for i, f := range x.fields {
					g := y.fields[i]
					if f.embedded != g.embedded ||
						x.Tag(i) != y.Tag(i) ||
						!f.sameId(g.pkg, g.name, false) ||
						!u.nify(f.typ, g.typ, emode, p) {
						return false
					}
				}
				return true
			}
		}

	case *Pointer:
		// Two pointer types unify if their base types unify.
		if y, ok := y.(*Pointer); ok {
			return u.nify(x.base, y.base, emode, p)
		}

	case *Tuple:
		// Two tuples types unify if they have the same number of elements
		// and the types of corresponding elements unify.
		if y, ok := y.(*Tuple); ok {
			if x.Len() == y.Len() {
				if x != nil {
					for i, v := range x.vars {
						w := y.vars[i]
						if !u.nify(v.typ, w.typ, mode, p) {
							return false
						}
					}
				}
				return true
			}
		}

	case *Signature:
		// Two function types unify if they have the same number of parameters
		// and result values, corresponding parameter and result types unify,
		// and either both functions are variadic or neither is.
		// Parameter and result names are not required to match.
		// TODO(gri) handle type parameters or document why we can ignore them.
		if y, ok := y.(*Signature); ok {
			return x.variadic == y.variadic &&
				u.nify(x.params, y.params, emode, p) &&
				u.nify(x.results, y.results, emode, p)
		}

	case *Interface:
		assert(!u.enableInterfaceInference || mode&exact != 0) // handled before this switch

		// Two interface types unify if they have the same set of methods with
		// the same names, and corresponding function types unify.
		// Lower-case method names from different packages are always different.
		// The order of the methods is irrelevant.
		if y, ok := y.(*Interface); ok {
			xset := x.typeSet()
			yset := y.typeSet()
			if xset.comparable != yset.comparable {
				return false
			}
			if !xset.terms.equal(yset.terms) {
				return false
			}
			a := xset.methods
			b := yset.methods
			if len(a) == len(b) {
				// Interface types are the only types where cycles can occur
				// that are not "terminated" via named types; and such cycles
				// can only be created via method parameter types that are
				// anonymous interfaces (directly or indirectly) embedding
				// the current interface. Example:
				//
				//    type T interface {
				//        m() interface{T}
				//    }
				//
				// If two such (differently named) interfaces are compared,
				// endless recursion occurs if the cycle is not detected.
				//
				// If x and y were compared before, they must be equal
				// (if they were not, the recursion would have stopped);
				// search the ifacePair stack for the same pair.
				//
				// This is a quadratic algorithm, but in practice these stacks
				// are extremely short (bounded by the nesting depth of interface
				// type declarations that recur via parameter types, an extremely
				// rare occurrence). An alternative implementation might use a
				// "visited" map, but that is probably less efficient overall.
				q := &ifacePair{x, y, p}
				for p != nil {
					if p.identical(q) {
						return true // same pair was compared before
					}
					p = p.prev
				}
				if debug {
					assertSortedMethods(a)
					assertSortedMethods(b)
				}
				for i, f := range a {
					g := b[i]
					if f.Id() != g.Id() || !u.nify(f.typ, g.typ, exact, q) {
						return false
					}
				}
				return true
			}
		}

	case *Map:
		// Two map types unify if their key and value types unify.
		if y, ok := y.(*Map); ok {
			return u.nify(x.key, y.key, emode, p) && u.nify(x.elem, y.elem, emode, p)
		}

	case *Chan:
		// Two channel types unify if their value types unify
		// and if they have the same direction.
		// The channel direction is ignored for inexact unification.
		if y, ok := y.(*Chan); ok {
			return (mode&exact == 0 || x.dir == y.dir) && u.nify(x.elem, y.elem, emode, p)
		}

	case *Named:
		// Two named types unify if their type names originate in the same type declaration.
		// If they are instantiated, their type argument lists must unify.
		if y := asNamed(y); y != nil {
			// Check type arguments before origins so they unify
			// even if the origins don't match; for better error
			// messages (see go.dev/issue/53692).
			xargs := x.TypeArgs().list()
			yargs := y.TypeArgs().list()
			if len(xargs) != len(yargs) {
				return false
			}
			for i, xarg := range xargs {
				if !u.nify(xarg, yargs[i], mode, p) {
					return false
				}
			}
			return identicalOrigin(x, y)
		}

	case *TypeParam:
		// x must be an unbound type parameter (see comment above).
		if debug {
			assert(u.asBoundTypeParam(x) == nil)
		}
		// By definition, a valid type argument must be in the type set of
		// the respective type constraint. Therefore, the type argument's
		// underlying type must be in the set of underlying types of that
		// constraint. If there is a single such underlying type, it's the
		// constraint's core type. It must match the type argument's under-
		// lying type, irrespective of whether the actual type argument,
		// which may be a defined type, is actually in the type set (that
		// will be determined at instantiation time).
		// Thus, if we have the core type of an unbound type parameter,
		// we know the structure of the possible types satisfying such
		// parameters. Use that core type for further unification
		// (see go.dev/issue/50755 for a test case).
		if enableCoreTypeUnification {
			// Because the core type is always an underlying type,
			// unification will take care of matching against a
			// defined or literal type automatically.
			// If y is also an unbound type parameter, we will end
			// up here again with x and y swapped, so we don't
			// need to take care of that case separately.
			if cx, _ := commonUnder(x, nil); cx != nil {
				if traceInference {
					u.tracef("core %s ≡ %s", xorig, yorig)
				}
				// If y is a defined type, it may not match against cx which
				// is an underlying type (incl. int, string, etc.). Use assign
				// mode here so that the unifier automatically uses y.Underlying()
				// if necessary.
				return u.nify(cx, yorig, assign, p)
			}
		}
		// x != y and there's nothing to do

	case nil:
		// avoid a crash in case of nil type

	default:
		panic(sprintf(nil, true, "u.nify(%s, %s, %d)", xorig, yorig, mode))
	}

	return false
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/union.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"cmd/compile/internal/syntax"
	. "internal/types/errors"
)

// ----------------------------------------------------------------------------
// API

// A Union represents a union of terms embedded in an interface.
type Union struct {
	terms []*Term // list of syntactical terms (not a canonicalized termlist)
}

// NewUnion returns a new Union type with the given terms.
// It is an error to create an empty union; they are syntactically not possible.
func NewUnion(terms []*Term) *Union {
	if len(terms) == 0 {
		panic("empty union")
	}
	return &Union{terms}
}

func (u *Union) Len() int         { return len(u.terms) }
func (u *Union) Term(i int) *Term { return u.terms[i] }

func (u *Union) Underlying() Type { return u }
func (u *Union) String() string   { return TypeString(u, nil) }

// A Term represents a term in a Union.
type Term term

// NewTerm returns a new union term.
func NewTerm(tilde bool, typ Type) *Term { return &Term{tilde, typ} }

func (t *Term) Tilde() bool    { return t.tilde }
func (t *Term) Type() Type     { return t.typ }
func (t *Term) String() string { return (*term)(t).String() }

// ----------------------------------------------------------------------------
// Implementation

// Avoid excessive type-checking times due to quadratic termlist operations.
const maxTermCount = 100

// parseUnion parses uexpr as a union of expressions.
// The result is a Union type, or Typ[Invalid] for some errors.
func parseUnion(check *Checker, uexpr syntax.Expr) Type {
	blist, tlist := flattenUnion(nil, uexpr)
	assert(len(blist) == len(tlist)-1)

	var terms []*Term

	var u Type
	for i, x := range tlist {
		term := parseTilde(check, x)
		if len(tlist) == 1 && !term.tilde {
			// Single type. Ok to return early because all relevant
			// checks have been performed in parseTilde (no need to
			// run through term validity check below).
			return term.typ // typ already recorded through check.typ in parseTilde
		}
		if len(terms) >= maxTermCount {
			if isValid(u) {
				check.errorf(x, InvalidUnion, "cannot handle more than %d union terms (implementation limitation)", maxTermCount)
				u = Typ[Invalid]
			}
		} else {
			terms = append(terms, term)
			u = &Union{terms}
		}

		if i > 0 {
			check.recordTypeAndValue(blist[i-1], typexpr, u, nil)
		}
	}

	if !isValid(u) {
		return u
	}

	// Check validity of terms.
	// Do this check later because it requires types to be set up.
	// Note: This is a quadratic algorithm, but unions tend to be short.
	check.later(func() {
		for i, t := range terms {
			if !isValid(t.typ) {
				continue
			}

			u := t.typ.Underlying()
			f, _ := u.(*Interface)
			if t.tilde {
				if f != nil {
					check.errorf(tlist[i], InvalidUnion, "invalid use of ~ (%s is an interface)", t.typ)
					continue // don't report another error for t
				}

				if !Identical(u, t.typ) {
					check.errorf(tlist[i], InvalidUnion, "invalid use of ~ (underlying type of %s is %s)", t.typ, u)
					continue
				}
			}

			// Stand-alone embedded interfaces are ok and are handled by the single-type case
			// in the beginning. Embedded interfaces with tilde are excluded above. If we reach
			// here, we must have at least two terms in the syntactic term list (but not necessarily
			// in the term list of the union's type set).
			if f != nil {
				tset := f.typeSet()
				switch {
				case tset.NumMethods() != 0:
					check.errorf(tlist[i], InvalidUnion, "cannot use %s in union (%s contains methods)", t, t)
				case t.typ == universeComparable.Type():
					check.error(tlist[i], InvalidUnion, "cannot use comparable in union")
				case tset.comparable:
					check.errorf(tlist[i], InvalidUnion, "cannot use %s in union (%s embeds comparable)", t, t)
				}
				continue // terms with interface types are not subject to the no-overlap rule
			}

			// Report overlapping (non-disjoint) terms such as
			// a|a, a|~a, ~a|~a, and ~a|A (where under(A) == a).
			if j := overlappingTerm(terms[:i], t); j >= 0 {
				check.softErrorf(tlist[i], InvalidUnion, "overlapping terms %s and %s", t, terms[j])
			}
		}
	}).describef(uexpr, "check term validity %s", uexpr)

	return u
}

func parseTilde(check *Checker, tx syntax.Expr) *Term {
	x := tx
	var tilde bool
	if op, _ := x.(*syntax.Operation); op != nil && op.Op == syntax.Tilde {
		x = op.X
		tilde = true
	}
	typ := check.typ(x)
	// Embedding stand-alone type parameters is not permitted (go.dev/issue/47127).
	// We don't need this restriction anymore if we make the underlying type of a type
	// parameter its constraint interface: if we embed a lone type parameter, we will
	// simply use its underlying type (like we do for other named, embedded interfaces),
	// and since the underlying type is an interface the embedding is well defined.
	if isTypeParam(typ) {
		if tilde {
			check.errorf(x, MisplacedTypeParam, "type in term %s cannot be a type parameter", tx)
		} else {
			check.error(x, MisplacedTypeParam, "term cannot be a type parameter")
		}
		typ = Typ[Invalid]
	}
	term := NewTerm(tilde, typ)
	if tilde {
		check.recordTypeAndValue(tx, typexpr, &Union{[]*Term{term}}, nil)
	}
	return term
}

// overlappingTerm reports the index of the term x in terms which is
// overlapping (not disjoint) from y. The result is < 0 if there is no
// such term. The type of term y must not be an interface, and terms
// with an interface type are ignored in the terms list.
func overlappingTerm(terms []*Term, y *Term) int {
	assert(!IsInterface(y.typ))
	for i, x := range terms {
		if IsInterface(x.typ) {
			continue
		}
		// disjoint requires non-nil, non-top arguments,
		// and non-interface types as term types.
		if debug {
			if x == nil || x.typ == nil || y == nil || y.typ == nil {
				panic("empty or top union term")
			}
		}
		if !(*term)(x).disjoint((*term)(y)) {
			return i
		}
	}
	return -1
}

// flattenUnion walks a union type expression of the form A | B | C | ...,
// extracting both the binary exprs (blist) and leaf types (tlist).
func flattenUnion(list []syntax.Expr, x syntax.Expr) (blist, tlist []syntax.Expr) {
	if o, _ := x.(*syntax.Operation); o != nil && o.Op == syntax.Or {
		blist, tlist = flattenUnion(list, o.X)
		blist = append(blist, o)
		x = o.Y
	}
	return blist, append(tlist, x)
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/universe.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file sets up the universe scope and the unsafe package.

package types2

import (
	"go/constant"
	"strings"
)

// The Universe scope contains all predeclared objects of Go.
// It is the outermost scope of any chain of nested scopes.
var Universe *Scope

// The Unsafe package is the package returned by an importer
// for the import path "unsafe".
var Unsafe *Package

var (
	universeIota       Object
	universeBool       Type
	universeByte       Type // uint8 alias, but has name "byte"
	universeRune       Type // int32 alias, but has name "rune"
	universeError      Type
	universeAny        Object
	universeComparable Object
)

// Typ contains the predeclared *Basic types indexed by their
// corresponding BasicKind.
//
// The *Basic type for Typ[Byte] will have the name "uint8".
// Use Universe.Lookup("byte").Type() to obtain the specific
// alias basic type named "byte" (and analogous for "rune").
var Typ = [...]*Basic{
	Invalid: {Invalid, 0, "invalid type"},

	Bool:          {Bool, IsBoolean, "bool"},
	Int:           {Int, IsInteger, "int"},
	Int8:          {Int8, IsInteger, "int8"},
	Int16:         {Int16, IsInteger, "int16"},
	Int32:         {Int32, IsInteger, "int32"},
	Int64:         {Int64, IsInteger, "int64"},
	Uint:          {Uint, IsInteger | IsUnsigned, "uint"},
	Uint8:         {Uint8, IsInteger | IsUnsigned, "uint8"},
	Uint16:        {Uint16, IsInteger | IsUnsigned, "uint16"},
	Uint32:        {Uint32, IsInteger | IsUnsigned, "uint32"},
	Uint64:        {Uint64, IsInteger | IsUnsigned, "uint64"},
	Uintptr:       {Uintptr, IsInteger | IsUnsigned, "uintptr"},
	Float32:       {Float32, IsFloat, "float32"},
	Float64:       {Float64, IsFloat, "float64"},
	Complex64:     {Complex64, IsComplex, "complex64"},
	Complex128:    {Complex128, IsComplex, "complex128"},
	String:        {String, IsString, "string"},
	UnsafePointer: {UnsafePointer, 0, "Pointer"},

	UntypedBool:    {UntypedBool, IsBoolean | IsUntyped, "untyped bool"},
	UntypedInt:     {UntypedInt, IsInteger | IsUntyped, "untyped int"},
	UntypedRune:    {UntypedRune, IsInteger | IsUntyped, "untyped rune"},
	UntypedFloat:   {UntypedFloat, IsFloat | IsUntyped, "untyped float"},
	UntypedComplex: {UntypedComplex, IsComplex | IsUntyped, "untyped complex"},
	UntypedString:  {UntypedString, IsString | IsUntyped, "untyped string"},
	UntypedNil:     {UntypedNil, IsUntyped, "untyped nil"},
}

var basicAliases = [...]*Basic{
	{Byte, IsInteger | IsUnsigned, "byte"},
	{Rune, IsInteger, "rune"},
}

func defPredeclaredTypes() {
	for _, t := range Typ {
		def(NewTypeName(nopos, nil, t.name, t))
	}

	for _, t := range basicAliases {
		def(NewTypeName(nopos, nil, t.name, t))
	}

	// type error interface{ Error() string }
	{
		obj := NewTypeName(nopos, nil, "error", nil)
		typ := NewNamed(obj, nil, nil)

		// error.Error() string
		recv := newVar(RecvVar, nopos, nil, "", typ)
		res := newVar(ResultVar, nopos, nil, "", Typ[String])
		sig := NewSignatureType(recv, nil, nil, nil, NewTuple(res), false)
		err := NewFunc(nopos, nil, "Error", sig)

		// interface{ Error() string }
		ityp := &Interface{methods: []*Func{err}, complete: true}
		computeInterfaceTypeSet(nil, nopos, ityp) // prevent races due to lazy computation of tset

		typ.fromRHS = ityp
		typ.Underlying()
		def(obj)
	}

	// type any = interface{}
	{
		obj := NewTypeName(nopos, nil, "any", nil)
		NewAlias(obj, &emptyInterface)
		def(obj)
	}

	// type comparable interface{} // marked as comparable
	{
		obj := NewTypeName(nopos, nil, "comparable", nil)
		NewNamed(obj, &Interface{complete: true, tset: &_TypeSet{nil, allTermlist, true}}, nil)
		def(obj)
	}
}

var predeclaredConsts = [...]struct {
	name string
	kind BasicKind
	val  constant.Value
}{
	{"true", UntypedBool, constant.MakeBool(true)},
	{"false", UntypedBool, constant.MakeBool(false)},
	{"iota", UntypedInt, constant.MakeInt64(0)},
}

func defPredeclaredConsts() {
	for _, c := range predeclaredConsts {
		def(NewConst(nopos, nil, c.name, Typ[c.kind], c.val))
	}
}

func defPredeclaredNil() {
	def(&Nil{object{name: "nil", typ: Typ[UntypedNil]}})
}

// A builtinId is the id of a builtin function.
type builtinId int

const (
	// universe scope
	_Append builtinId = iota
	_Cap
	_Clear
	_Close
	_Complex
	_Copy
	_Delete
	_Imag
	_Len
	_Make
	_Max
	_Min
	_New
	_Panic
	_Print
	_Println
	_Real
	_Recover

	// package unsafe
	_Add
	_Alignof
	_Offsetof
	_Sizeof
	_Slice
	_SliceData
	_String
	_StringData

	// testing support
	_Assert
	_Trace
)

var predeclaredFuncs = [...]struct {
	name     string
	nargs    int
	variadic bool
	kind     exprKind
}{
	_Append:  {"append", 1, true, expression},
	_Cap:     {"cap", 1, false, expression},
	_Clear:   {"clear", 1, false, statement},
	_Close:   {"close", 1, false, statement},
	_Complex: {"complex", 2, false, expression},
	_Copy:    {"copy", 2, false, statement},
	_Delete:  {"delete", 2, false, statement},
	_Imag:    {"imag", 1, false, expression},
	_Len:     {"len", 1, false, expression},
	_Make:    {"make", 1, true, expression},
	// To disable max/min, remove the next two lines.
	_Max:     {"max", 1, true, expression},
	_Min:     {"min", 1, true, expression},
	_New:     {"new", 1, false, expression},
	_Panic:   {"panic", 1, false, statement},
	_Print:   {"print", 0, true, statement},
	_Println: {"println", 0, true, statement},
	_Real:    {"real", 1, false, expression},
	_Recover: {"recover", 0, false, statement},

	_Add:        {"Add", 2, false, expression},
	_Alignof:    {"Alignof", 1, false, expression},
	_Offsetof:   {"Offsetof", 1, false, expression},
	_Sizeof:     {"Sizeof", 1, false, expression},
	_Slice:      {"Slice", 2, false, expression},
	_SliceData:  {"SliceData", 1, false, expression},
	_String:     {"String", 2, false, expression},
	_StringData: {"StringData", 1, false, expression},

	_Assert: {"assert", 1, false, statement},
	_Trace:  {"trace", 0, true, statement},
}

func defPredeclaredFuncs() {
	for i := range predeclaredFuncs {
		id := builtinId(i)
		if id == _Assert || id == _Trace {
			continue // only define these in testing environment
		}
		def(newBuiltin(id))
	}
}

// DefPredeclaredTestFuncs defines the assert and trace built-ins.
// These built-ins are intended for debugging and testing of this
// package only.
func DefPredeclaredTestFuncs() {
	if Universe.Lookup("assert") != nil {
		return // already defined
	}
	def(newBuiltin(_Assert))
	def(newBuiltin(_Trace))
}

func init() {
	Universe = NewScope(nil, nopos, nopos, "universe")
	Unsafe = NewPackage("unsafe", "unsafe")
	Unsafe.complete = true

	defPredeclaredTypes()
	defPredeclaredConsts()
	defPredeclaredNil()
	defPredeclaredFuncs()

	universeIota = Universe.Lookup("iota")
	universeBool = Universe.Lookup("bool").Type()
	universeByte = Universe.Lookup("byte").Type()
	universeRune = Universe.Lookup("rune").Type()
	universeError = Universe.Lookup("error").Type()
	universeAny = Universe.Lookup("any")
	universeComparable = Universe.Lookup("comparable")
}

// Objects with names containing blanks are internal and not entered into
// a scope. Objects with exported names are inserted in the unsafe package
// scope; other objects are inserted in the universe scope.
func def(obj Object) {
	assert(obj.Type() != nil)
	name := obj.Name()
	if strings.Contains(name, " ") {
		return // nothing to do
	}
	// fix Obj link for named types
	if typ := asNamed(obj.Type()); typ != nil {
		typ.obj = obj.(*TypeName)
	}
	// exported identifiers go into package unsafe
	scope := Universe
	if obj.Exported() {
		scope = Unsafe.scope
		// set Pkg field
		switch obj := obj.(type) {
		case *TypeName:
			obj.pkg = Unsafe
		case *Builtin:
			obj.pkg = Unsafe
		default:
			panic("unreachable")
		}
	}
	if scope.Insert(obj) != nil {
		panic("double declaration of predeclared identifier")
	}
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/util.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains various functionality that is
// different between go/types and types2. Factoring
// out this code allows more of the rest of the code
// to be shared.

package types2

import (
	"cmd/compile/internal/syntax"
	"go/constant"
	"go/token"
)

const isTypes2 = true

// cmpPos compares the positions p and q and returns a result r as follows:
//
// r <  0: p is before q
// r == 0: p and q are the same position (but may not be identical)
// r >  0: p is after q
//
// If p and q are in different files, p is before q if the filename
// of p sorts lexicographically before the filename of q.
func cmpPos(p, q syntax.Pos) int { return p.Cmp(q) }

// hasDots reports whether the last argument in the call is followed by ...
func hasDots(call *syntax.CallExpr) bool { return call.HasDots }

// dddErrPos returns the node (poser) for reporting an invalid ... use in a call.
func dddErrPos(call *syntax.CallExpr) *syntax.CallExpr {
	// TODO(gri) should use "..." instead of call position
	return call
}

// isdddArray reports whether atyp is of the form [...]E.
func isdddArray(atyp *syntax.ArrayType) bool { return atyp.Len == nil }

// argErrPos returns the node (poser) for reporting an invalid argument count.
func argErrPos(call *syntax.CallExpr) *syntax.CallExpr { return call }

// ExprString returns a string representation of x.
func ExprString(x syntax.Node) string { return syntax.String(x) }

// startPos returns the start position of node n.
func startPos(n syntax.Node) syntax.Pos { return syntax.StartPos(n) }

// endPos returns the position of the first character immediately after node n.
func endPos(n syntax.Node) syntax.Pos { return syntax.EndPos(n) }

// inNode is a dummy function returning pos.
func inNode(_ syntax.Node, pos syntax.Pos) syntax.Pos { return pos }

// makeFromLiteral returns the constant value for the given literal string and kind.
func makeFromLiteral(lit string, kind syntax.LitKind) constant.Value {
	return constant.MakeFromLiteral(lit, kind2tok[kind], 0)
}

var kind2tok = [...]token.Token{
	syntax.IntLit:    token.INT,
	syntax.FloatLit:  token.FLOAT,
	syntax.ImagLit:   token.IMAG,
	syntax.RuneLit:   token.CHAR,
	syntax.StringLit: token.STRING,
}

```

// === FILE: references!/go/src/cmd/compile/internal/types2/validtype.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import "cmd/compile/internal/syntax"

// validType verifies that the given type does not "expand" indefinitely
// producing a cycle in the type graph.
// (Cycles involving alias types, as in "type A = [10]A" are detected
// earlier, via the objDecl cycle detection mechanism.)
func (check *Checker) validType(typ *Named) {
	check.validType0(nopos, typ, nil, nil)
}

// validType0 checks if the given type is valid. If typ is a type parameter
// its value is looked up in the type argument list of the instantiated
// (enclosing) type, if it exists. Otherwise the type parameter must be from
// an enclosing function and can be ignored.
// The nest list describes the stack (the "nest in memory") of types which
// contain (or embed in the case of interfaces) other types. For instance, a
// struct named S which contains a field of named type F contains (the memory
// of) F in S, leading to the nest S->F. If a type appears in its own nest
// (say S->F->S) we have an invalid recursive type. The path list is the full
// path of named types in a cycle, it is only needed for error reporting.
func (check *Checker) validType0(pos syntax.Pos, typ Type, nest, path []*Named) bool {
	typ = Unalias(typ)

	if check.conf.Trace {
		if t, _ := typ.(*Named); t != nil && t.obj != nil /* obj should always exist but be conservative */ {
			pos = t.obj.pos
		}
		check.indent++
		check.trace(pos, "validType(%s) nest %v, path %v", typ, pathString(makeObjList(nest)), pathString(makeObjList(path)))
		defer func() {
			check.indent--
		}()
	}

	switch t := typ.(type) {
	case nil:
		// We should never see a nil type but be conservative and panic
		// only in debug mode.
		if debug {
			panic("validType0(nil)")
		}

	case *Array:
		return check.validType0(pos, t.elem, nest, path)

	case *Struct:
		for _, f := range t.fields {
			if !check.validType0(pos, f.typ, nest, path) {
				return false
			}
		}

	case *Union:
		for _, t := range t.terms {
			if !check.validType0(pos, t.typ, nest, path) {
				return false
			}
		}

	case *Interface:
		for _, etyp := range t.embeddeds {
			if !check.validType0(pos, etyp, nest, path) {
				return false
			}
		}

	case *Named:
		// TODO(gri) The optimization below is incorrect (see go.dev/issue/65711):
		//           in that issue `type A[P any] [1]P` is a valid type on its own
		//           and the (uninstantiated) A is recorded in check.valids. As a
		//           consequence, when checking the remaining declarations, which
		//           are not valid, the validity check ends prematurely because A
		//           is considered valid, even though its validity depends on the
		//           type argument provided to it.
		//
		//           A correct optimization is important for pathological cases.
		//           Keep code around for reference until we found an optimization.
		//
		// // Exit early if we already know t is valid.
		// // This is purely an optimization but it prevents excessive computation
		// // times in pathological cases such as testdata/fixedbugs/issue6977.go.
		// // (Note: The valids map could also be allocated locally, once for each
		// // validType call.)
		// if check.valids.lookup(t) != nil {
		// 	break
		// }

		// If the current type t is also found in nest, (the memory of) t is
		// embedded in itself, indicating an invalid recursive type.
		for _, e := range nest {
			if Identical(e, t) {
				// We have a cycle. If t != t.Origin() then t is an instance of
				// the generic type t.Origin(). Because t is in the nest, t must
				// occur within the definition (RHS) of the generic type t.Origin(),
				// directly or indirectly, after expansion of the RHS.
				// Therefore t.Origin() must be invalid, no matter how it is
				// instantiated since the instantiation t of t.Origin() happens
				// inside t.Origin()'s RHS and thus is always the same and always
				// present.
				// Therefore we can mark the underlying of both t and t.Origin()
				// as invalid. If t is not an instance of a generic type, t and
				// t.Origin() are the same.
				// Furthermore, because we check all types in a package for validity
				// before type checking is complete, any exported type that is invalid
				// will have an invalid underlying type and we can't reach here with
				// such a type (invalid types are excluded above).
				// Thus, if we reach here with a type t, both t and t.Origin() (if
				// different in the first place) must be from the current package;
				// they cannot have been imported.
				// Therefore it is safe to change their underlying types; there is
				// no chance for a race condition (the types of the current package
				// are not yet available to other goroutines).
				assert(t.obj.pkg == check.pkg)
				assert(t.Origin().obj.pkg == check.pkg)

				// let t become invalid when it is unpacked
				t.Origin().fromRHS = Typ[Invalid]

				// Find the starting point of the cycle and report it.
				// Because each type in nest must also appear in path (see invariant below),
				// type t must be in path since it was found in nest. But not every type in path
				// is in nest. Specifically t may appear in path with an earlier index than the
				// index of t in nest. Search again.
				for start, p := range path {
					if Identical(p, t) {
						check.cycleError(makeObjList(path[start:]), 0)
						return false
					}
				}
				panic("cycle start not found")
			}
		}

		// No cycle was found. Check the RHS of t.
		// Every type added to nest is also added to path; thus every type that is in nest
		// must also be in path (invariant). But not every type in path is in nest, since
		// nest may be pruned (see below, *TypeParam case).
		t.Origin().unpack()
		if !check.validType0(pos, t.Origin().rhs(), append(nest, t), append(path, t)) {
			return false
		}

		// see TODO above
		// check.valids.add(t) // t is valid

	case *TypeParam:
		// A type parameter stands for the type (argument) it was instantiated with.
		// Check the corresponding type argument for validity if we are in an
		// instantiated type.
		if d := len(nest) - 1; d >= 0 {
			inst := nest[d] // the type instance
			// Find the corresponding type argument for the type parameter
			// and proceed with checking that type argument.
			for i, tparam := range inst.TypeParams().list() {
				// The type parameter and type argument lists should
				// match in length but be careful in case of errors.
				if t == tparam && i < inst.TypeArgs().Len() {
					targ := inst.TypeArgs().At(i)
					// The type argument must be valid in the enclosing
					// type (where inst was instantiated), hence we must
					// check targ's validity in the type nest excluding
					// the current (instantiated) type (see the example
					// at the end of this file).
					// For error reporting we keep the full path.
					res := check.validType0(pos, targ, nest[:d], path)
					// The check.validType0 call with nest[:d] may have
					// overwritten the entry at the current depth d.
					// Restore the entry (was issue go.dev/issue/66323).
					nest[d] = inst
					return res
				}
			}
		}
	}

	return true
}

// makeObjList returns the list of type name objects for the given
// list of named types.
func makeObjList(tlist []*Named) []Object {
	olist := make([]Object, len(tlist))
	for i, t := range tlist {
		olist[i] = t.obj
	}
	return olist
}

// Here is an example illustrating why we need to exclude the
// instantiated type from nest when evaluating the validity of
// a type parameter. Given the declarations
//
//   var _ A[A[string]]
//
//   type A[P any] struct { _ B[P] }
//   type B[P any] struct { _ P }
//
// we want to determine if the type A[A[string]] is valid.
// We start evaluating A[A[string]] outside any type nest:
//
//   A[A[string]]
//         nest =
//         path =
//
// The RHS of A is now evaluated in the A[A[string]] nest:
//
//   struct{_ B[P₁]}
//         nest = A[A[string]]
//         path = A[A[string]]
//
// The struct has a single field of type B[P₁] with which
// we continue:
//
//   B[P₁]
//         nest = A[A[string]]
//         path = A[A[string]]
//
//   struct{_ P₂}
//         nest = A[A[string]]->B[P]
//         path = A[A[string]]->B[P]
//
// Eventually we reach the type parameter P of type B (P₂):
//
//   P₂
//         nest = A[A[string]]->B[P]
//         path = A[A[string]]->B[P]
//
// The type argument for P of B is the type parameter P of A (P₁).
// It must be evaluated in the type nest that existed when B was
// instantiated:
//
//   P₁
//         nest = A[A[string]]        <== type nest at B's instantiation time
//         path = A[A[string]]->B[P]
//
// If we'd use the current nest it would correspond to the path
// which will be wrong as we will see shortly. P's type argument
// is A[string], which again must be evaluated in the type nest
// that existed when A was instantiated with A[string]. That type
// nest is empty:
//
//   A[string]
//         nest =                     <== type nest at A's instantiation time
//         path = A[A[string]]->B[P]
//
// Evaluation then proceeds as before for A[string]:
//
//   struct{_ B[P₁]}
//         nest = A[string]
//         path = A[A[string]]->B[P]->A[string]
//
// Now we reach B[P] again. If we had not adjusted nest, it would
// correspond to path, and we would find B[P] in nest, indicating
// a cycle, which would clearly be wrong since there's no cycle in
// A[string]:
//
//   B[P₁]
//         nest = A[string]
//         path = A[A[string]]->B[P]->A[string]  <== path contains B[P]!
//
// But because we use the correct type nest, evaluation proceeds without
// errors and we get the evaluation sequence:
//
//   struct{_ P₂}
//         nest = A[string]->B[P]
//         path = A[A[string]]->B[P]->A[string]->B[P]
//   P₂
//         nest = A[string]->B[P]
//         path = A[A[string]]->B[P]->A[string]->B[P]
//   P₁
//         nest = A[string]
//         path = A[A[string]]->B[P]->A[string]->B[P]
//   string
//         nest =
//         path = A[A[string]]->B[P]->A[string]->B[P]
//
// At this point we're done and A[A[string]] and is valid.

```

// === FILE: references!/go/src/cmd/compile/internal/types2/version.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import (
	"fmt"
	"go/version"
	"internal/goversion"
)

// A goVersion is a Go language version string of the form "go1.%d"
// where d is the minor version number. goVersion strings don't
// contain release numbers ("go1.20.1" is not a valid goVersion).
type goVersion string

// asGoVersion returns v as a goVersion (e.g., "go1.20.1" becomes "go1.20").
// If v is not a valid Go version, the result is the empty string.
func asGoVersion(v string) goVersion {
	return goVersion(version.Lang(v))
}

// isValid reports whether v is a valid Go version.
func (v goVersion) isValid() bool {
	return v != ""
}

// cmp returns -1, 0, or +1 depending on whether x < y, x == y, or x > y,
// interpreted as Go versions.
func (x goVersion) cmp(y goVersion) int {
	return version.Compare(string(x), string(y))
}

var (
	// Go versions that introduced language changes
	go1_9  = asGoVersion("go1.9")
	go1_13 = asGoVersion("go1.13")
	go1_14 = asGoVersion("go1.14")
	go1_17 = asGoVersion("go1.17")
	go1_18 = asGoVersion("go1.18")
	go1_20 = asGoVersion("go1.20")
	go1_21 = asGoVersion("go1.21")
	go1_22 = asGoVersion("go1.22")
	go1_23 = asGoVersion("go1.23")
	go1_26 = asGoVersion("go1.26")
	go1_27 = asGoVersion("go1.27")

	// current (deployed) Go version
	go_current = asGoVersion(fmt.Sprintf("go1.%d", goversion.Version))
)

// allowVersion reports whether the current effective Go version
// (which may vary from one file to another) is allowed to use the
// feature version (want).
func (check *Checker) allowVersion(want goVersion) bool {
	return !check.version.isValid() || check.version.cmp(want) >= 0
}

// verifyVersionf is like allowVersion but also accepts a format string and arguments
// which are used to report a version error if allowVersion returns false.
func (check *Checker) verifyVersionf(at poser, v goVersion, format string, args ...any) bool {
	if !check.allowVersion(v) {
		check.versionErrorf(at, v, format, args...)
		return false
	}
	return true
}

```

