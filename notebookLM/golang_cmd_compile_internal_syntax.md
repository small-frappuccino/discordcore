# Domain Architecture: cmd/compile/internal/syntax

## Layout Topology
```text
cmd/compile/internal/syntax/
├── branches.go
├── dumper.go
├── nodes.go
├── operator_string.go
├── parser.go
├── pos.go
├── positions.go
├── printer.go
├── scanner.go
├── source.go
├── syntax.go
├── testing.go
├── token_string.go
├── tokens.go
├── type.go
└── walk.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/compile/internal/syntax/branches.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import "fmt"

// checkBranches checks correct use of labels and branch
// statements (break, continue, fallthrough, goto) in a function body.
// It catches:
//   - misplaced breaks, continues, and fallthroughs
//   - bad labeled breaks and continues
//   - invalid, unused, duplicate, and missing labels
//   - gotos jumping over variable declarations and into blocks
func checkBranches(body *BlockStmt, errh ErrorHandler) {
	if body == nil {
		return
	}

	// scope of all labels in this body
	ls := &labelScope{errh: errh}
	fwdGotos := ls.blockBranches(nil, targets{}, nil, body.Pos(), body.List)

	// If there are any forward gotos left, no matching label was
	// found for them. Either those labels were never defined, or
	// they are inside blocks and not reachable from the gotos.
	for _, fwd := range fwdGotos {
		name := fwd.Label.Value
		if l := ls.labels[name]; l != nil {
			l.used = true // avoid "defined and not used" error
			ls.errf(fwd.Label.Pos(), "goto %s jumps into block starting at %s", name, l.parent.start)
		} else {
			ls.errf(fwd.Label.Pos(), "label %s not defined", name)
		}
	}

	// spec: "It is illegal to define a label that is never used."
	for _, l := range ls.labels {
		if !l.used {
			l := l.lstmt.Label
			ls.errf(l.Pos(), "label %s defined and not used", l.Value)
		}
	}
}

type labelScope struct {
	errh   ErrorHandler
	labels map[string]*label // all label declarations inside the function; allocated lazily
}

type label struct {
	parent *block       // block containing this label declaration
	lstmt  *LabeledStmt // statement declaring the label
	used   bool         // whether the label is used or not
}

type block struct {
	parent *block       // immediately enclosing block, or nil
	start  Pos          // start of block
	lstmt  *LabeledStmt // labeled statement associated with this block, or nil
}

func (ls *labelScope) errf(pos Pos, format string, args ...any) {
	ls.errh(Error{pos, fmt.Sprintf(format, args...)})
}

// declare declares the label introduced by s in block b and returns
// the new label. If the label was already declared, declare reports
// and error and the existing label is returned instead.
func (ls *labelScope) declare(b *block, s *LabeledStmt) *label {
	name := s.Label.Value
	labels := ls.labels
	if labels == nil {
		labels = make(map[string]*label)
		ls.labels = labels
	} else if alt := labels[name]; alt != nil {
		ls.errf(s.Label.Pos(), "label %s already defined at %s", name, alt.lstmt.Label.Pos().String())
		return alt
	}
	l := &label{b, s, false}
	labels[name] = l
	return l
}

// gotoTarget returns the labeled statement matching the given name and
// declared in block b or any of its enclosing blocks. The result is nil
// if the label is not defined, or doesn't match a valid labeled statement.
func (ls *labelScope) gotoTarget(b *block, name string) *LabeledStmt {
	if l := ls.labels[name]; l != nil {
		l.used = true // even if it's not a valid target
		for ; b != nil; b = b.parent {
			if l.parent == b {
				return l.lstmt
			}
		}
	}
	return nil
}

var invalid = new(LabeledStmt) // singleton to signal invalid enclosing target

// enclosingTarget returns the innermost enclosing labeled statement matching
// the given name. The result is nil if the label is not defined, and invalid
// if the label is defined but doesn't label a valid labeled statement.
func (ls *labelScope) enclosingTarget(b *block, name string) *LabeledStmt {
	if l := ls.labels[name]; l != nil {
		l.used = true // even if it's not a valid target (see e.g., test/fixedbugs/bug136.go)
		for ; b != nil; b = b.parent {
			if l.lstmt == b.lstmt {
				return l.lstmt
			}
		}
		return invalid
	}
	return nil
}

// targets describes the target statements within which break
// or continue statements are valid.
type targets struct {
	breaks    Stmt     // *ForStmt, *SwitchStmt, *SelectStmt, or nil
	continues *ForStmt // or nil
	caseIndex int      // case index of immediately enclosing switch statement, or < 0
}

// blockBranches processes a block's body starting at start and returns the
// list of unresolved (forward) gotos. parent is the immediately enclosing
// block (or nil), ctxt provides information about the enclosing statements,
// and lstmt is the labeled statement associated with this block, or nil.
func (ls *labelScope) blockBranches(parent *block, ctxt targets, lstmt *LabeledStmt, start Pos, body []Stmt) []*BranchStmt {
	b := &block{parent: parent, start: start, lstmt: lstmt}

	var varPos Pos
	var varName Expr
	var fwdGotos, badGotos []*BranchStmt

	recordVarDecl := func(pos Pos, name Expr) {
		varPos = pos
		varName = name
		// Any existing forward goto jumping over the variable
		// declaration is invalid. The goto may still jump out
		// of the block and be ok, but we don't know that yet.
		// Remember all forward gotos as potential bad gotos.
		badGotos = append(badGotos[:0], fwdGotos...)
	}

	jumpsOverVarDecl := func(fwd *BranchStmt) bool {
		if varPos.IsKnown() {
			for _, bad := range badGotos {
				if fwd == bad {
					return true
				}
			}
		}
		return false
	}

	innerBlock := func(ctxt targets, start Pos, body []Stmt) {
		// Unresolved forward gotos from the inner block
		// become forward gotos for the current block.
		fwdGotos = append(fwdGotos, ls.blockBranches(b, ctxt, lstmt, start, body)...)
	}

	// A fallthrough statement counts as last statement in a statement
	// list even if there are trailing empty statements; remove them.
	stmtList := trimTrailingEmptyStmts(body)
	for stmtIndex, stmt := range stmtList {
		lstmt = nil
	L:
		switch s := stmt.(type) {
		case *DeclStmt:
			for _, d := range s.DeclList {
				if v, ok := d.(*VarDecl); ok {
					recordVarDecl(v.Pos(), v.NameList[0])
					break // the first VarDecl will do
				}
			}

		case *LabeledStmt:
			// declare non-blank label
			if name := s.Label.Value; name != "_" {
				l := ls.declare(b, s)
				// resolve matching forward gotos
				i := 0
				for _, fwd := range fwdGotos {
					if fwd.Label.Value == name {
						fwd.Target = s
						l.used = true
						if jumpsOverVarDecl(fwd) {
							ls.errf(
								fwd.Label.Pos(),
								"goto %s jumps over declaration of %s at %s",
								name, String(varName), varPos,
							)
						}
					} else {
						// no match - keep forward goto
						fwdGotos[i] = fwd
						i++
					}
				}
				fwdGotos = fwdGotos[:i]
				lstmt = s
			}
			// process labeled statement
			stmt = s.Stmt
			goto L

		case *BranchStmt:
			// unlabeled branch statement
			if s.Label == nil {
				switch s.Tok {
				case _Break:
					if t := ctxt.breaks; t != nil {
						s.Target = t
					} else {
						ls.errf(s.Pos(), "break is not in a loop, switch, or select")
					}
				case _Continue:
					if t := ctxt.continues; t != nil {
						s.Target = t
					} else {
						ls.errf(s.Pos(), "continue is not in a loop")
					}
				case _Fallthrough:
					msg := "fallthrough statement out of place"
					if t, _ := ctxt.breaks.(*SwitchStmt); t != nil {
						if _, ok := t.Tag.(*TypeSwitchGuard); ok {
							msg = "cannot fallthrough in type switch"
						} else if ctxt.caseIndex < 0 || stmtIndex+1 < len(stmtList) {
							// fallthrough nested in a block or not the last statement
							// use msg as is
						} else if ctxt.caseIndex+1 == len(t.Body) {
							msg = "cannot fallthrough final case in switch"
						} else {
							break // fallthrough ok
						}
					}
					ls.errf(s.Pos(), "%s", msg)
				case _Goto:
					fallthrough // should always have a label
				default:
					panic("invalid BranchStmt")
				}
				break
			}

			// labeled branch statement
			name := s.Label.Value
			switch s.Tok {
			case _Break:
				// spec: "If there is a label, it must be that of an enclosing
				// "for", "switch", or "select" statement, and that is the one
				// whose execution terminates."
				if t := ls.enclosingTarget(b, name); t != nil {
					switch t := t.Stmt.(type) {
					case *SwitchStmt, *SelectStmt, *ForStmt:
						s.Target = t
					default:
						ls.errf(s.Label.Pos(), "invalid break label %s", name)
					}
				} else {
					ls.errf(s.Label.Pos(), "break label not defined: %s", name)
				}

			case _Continue:
				// spec: "If there is a label, it must be that of an enclosing
				// "for" statement, and that is the one whose execution advances."
				if t := ls.enclosingTarget(b, name); t != nil {
					if t, ok := t.Stmt.(*ForStmt); ok {
						s.Target = t
					} else {
						ls.errf(s.Label.Pos(), "invalid continue label %s", name)
					}
				} else {
					ls.errf(s.Label.Pos(), "continue label not defined: %s", name)
				}

			case _Goto:
				if t := ls.gotoTarget(b, name); t != nil {
					s.Target = t
				} else {
					// label may be declared later - add goto to forward gotos
					fwdGotos = append(fwdGotos, s)
				}

			case _Fallthrough:
				fallthrough // should never have a label
			default:
				panic("invalid BranchStmt")
			}

		case *AssignStmt:
			if s.Op == Def {
				recordVarDecl(s.Pos(), s.Lhs)
			}

		case *BlockStmt:
			inner := targets{ctxt.breaks, ctxt.continues, -1}
			innerBlock(inner, s.Pos(), s.List)

		case *IfStmt:
			inner := targets{ctxt.breaks, ctxt.continues, -1}
			innerBlock(inner, s.Then.Pos(), s.Then.List)
			if s.Else != nil {
				innerBlock(inner, s.Else.Pos(), []Stmt{s.Else})
			}

		case *ForStmt:
			inner := targets{s, s, -1}
			innerBlock(inner, s.Body.Pos(), s.Body.List)

		case *SwitchStmt:
			inner := targets{s, ctxt.continues, -1}
			for i, cc := range s.Body {
				inner.caseIndex = i
				innerBlock(inner, cc.Pos(), cc.Body)
			}

		case *SelectStmt:
			inner := targets{s, ctxt.continues, -1}
			for _, cc := range s.Body {
				innerBlock(inner, cc.Pos(), cc.Body)
			}
		}
	}

	return fwdGotos
}

func trimTrailingEmptyStmts(list []Stmt) []Stmt {
	for i := len(list); i > 0; i-- {
		if _, ok := list[i-1].(*EmptyStmt); !ok {
			return list[:i]
		}
	}
	return nil
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/dumper.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements printing of syntax tree structures.

package syntax

import (
	"fmt"
	"io"
	"reflect"
	"unicode"
	"unicode/utf8"
)

// Fdump dumps the structure of the syntax tree rooted at n to w.
// It is intended for debugging purposes; no specific output format
// is guaranteed.
func Fdump(w io.Writer, n Node) (err error) {
	p := dumper{
		output: w,
		ptrmap: make(map[Node]int),
		last:   '\n', // force printing of line number on first line
	}

	defer func() {
		if e := recover(); e != nil {
			err = e.(writeError).err // re-panics if it's not a writeError
		}
	}()

	if n == nil {
		p.printf("nil\n")
		return
	}
	p.dump(reflect.ValueOf(n), n)
	p.printf("\n")

	return
}

type dumper struct {
	output io.Writer
	ptrmap map[Node]int // node -> dump line number
	indent int          // current indentation level
	last   byte         // last byte processed by Write
	line   int          // current line number
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

// writeError wraps locally caught write errors so we can distinguish
// them from genuine panics which we don't want to return as errors.
type writeError struct {
	err error
}

// printf is a convenience wrapper that takes care of print errors.
func (p *dumper) printf(format string, args ...any) {
	if _, err := fmt.Fprintf(p, format, args...); err != nil {
		panic(writeError{err})
	}
}

// dump prints the contents of x.
// If x is the reflect.Value of a struct s, where &s
// implements Node, then &s should be passed for n -
// this permits printing of the unexported span and
// comments fields of the embedded isNode field by
// calling the Span() and Comment() instead of using
// reflection.
func (p *dumper) dump(x reflect.Value, n Node) {
	switch x.Kind() {
	case reflect.Interface:
		if x.IsNil() {
			p.printf("nil")
			return
		}
		p.dump(x.Elem(), nil)

	case reflect.Ptr:
		if x.IsNil() {
			p.printf("nil")
			return
		}

		// special cases for identifiers w/o attached comments (common case)
		if x, ok := x.Interface().(*Name); ok {
			p.printf("%s @ %v", x.Value, x.Pos())
			return
		}

		p.printf("*")
		// Fields may share type expressions, and declarations
		// may share the same group - use ptrmap to keep track
		// of nodes that have been printed already.
		if ptr, ok := x.Interface().(Node); ok {
			if line, exists := p.ptrmap[ptr]; exists {
				p.printf("(Node @ %d)", line)
				return
			}
			p.ptrmap[ptr] = p.line
			n = ptr
		}
		p.dump(x.Elem(), n)

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
				p.dump(x.Index(i), nil)
				p.printf("\n")
			}
			p.indent--
		}
		p.printf("}")

	case reflect.Struct:
		typ := x.Type()

		// if span, ok := x.Interface().(lexical.Span); ok {
		// 	p.printf("%s", &span)
		// 	return
		// }

		p.printf("%s {", typ)
		p.indent++

		first := true
		if n != nil {
			p.printf("\n")
			first = false
			// p.printf("Span: %s\n", n.Span())
			// if c := *n.Comments(); c != nil {
			// 	p.printf("Comments: ")
			// 	p.dump(reflect.ValueOf(c), nil) // a Comment is not a Node
			// 	p.printf("\n")
			// }
		}

		for i, n := 0, typ.NumField(); i < n; i++ {
			// Exclude non-exported fields because their
			// values cannot be accessed via reflection.
			if name := typ.Field(i).Name; isExported(name) {
				if first {
					p.printf("\n")
					first = false
				}
				p.printf("%s: ", name)
				p.dump(x.Field(i), nil)
				p.printf("\n")
			}
		}

		p.indent--
		p.printf("}")

	default:
		switch x := x.Interface().(type) {
		case string:
			// print strings in quotes
			p.printf("%q", x)
		default:
			p.printf("%v", x)
		}
	}
}

func isExported(name string) bool {
	ch, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(ch)
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/nodes.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import "fmt"

// ----------------------------------------------------------------------------
// Nodes

type Node interface {
	// Pos() returns the position associated with the node as follows:
	// 1) The position of a node representing a terminal syntax production
	//    (Name, BasicLit, etc.) is the position of the respective production
	//    in the source.
	// 2) The position of a node representing a non-terminal production
	//    (IndexExpr, IfStmt, etc.) is the position of a token uniquely
	//    associated with that production; usually the left-most one
	//    ('[' for IndexExpr, 'if' for IfStmt, etc.)
	Pos() Pos
	SetPos(Pos)
	aNode()
}

type node struct {
	// commented out for now since not yet used
	// doc  *Comment // nil means no comment(s) attached
	pos Pos
}

func (n *node) Pos() Pos       { return n.pos }
func (n *node) SetPos(pos Pos) { n.pos = pos }
func (*node) aNode()           {}

// ----------------------------------------------------------------------------
// Files

// package PkgName; DeclList[0], DeclList[1], ...
type File struct {
	Pragma    Pragma
	PkgName   *Name
	DeclList  []Decl
	EOF       Pos
	GoVersion string
	node
}

func (f *File) String() string {
	return fmt.Sprintf("File{PkgName:%v, DeclList:%v}", f.PkgName, f.DeclList)
}

// ----------------------------------------------------------------------------
// Declarations

type (
	Decl interface {
		Node
		aDecl()
	}

	//              Path
	// LocalPkgName Path
	ImportDecl struct {
		Group        *Group // nil means not part of a group
		Pragma       Pragma
		LocalPkgName *Name     // including "."; nil means no rename present
		Path         *BasicLit // Path.Bad || Path.Kind == StringLit; nil means no path
		decl
	}

	// NameList
	// NameList      = Values
	// NameList Type = Values
	ConstDecl struct {
		Group    *Group // nil means not part of a group
		Pragma   Pragma
		NameList []*Name
		Type     Expr // nil means no type
		Values   Expr // nil means no values
		decl
	}

	// Name Type
	TypeDecl struct {
		Group      *Group // nil means not part of a group
		Pragma     Pragma
		Name       *Name
		TParamList []*Field // nil means no type parameters
		Alias      bool
		Type       Expr
		decl
	}

	// NameList Type
	// NameList Type = Values
	// NameList      = Values
	VarDecl struct {
		Group    *Group // nil means not part of a group
		Pragma   Pragma
		NameList []*Name
		Type     Expr // nil means no type
		Values   Expr // nil means no values
		decl
	}

	// func          Name Type { Body }
	// func          Name Type
	// func Receiver Name Type { Body }
	// func Receiver Name Type
	FuncDecl struct {
		Pragma     Pragma
		Recv       *Field // nil means regular function
		Name       *Name
		TParamList []*Field // nil means no type parameters
		Type       *FuncType
		Body       *BlockStmt // nil means no body (forward declaration)
		decl
	}
)

func (d *FuncDecl) String() string {
	return fmt.Sprintf("FuncDecl{Name:%v}", d.Name)
}

func (d *TypeDecl) String() string {
	return fmt.Sprintf("TypeDecl{Name:%v}", d.Name)
}

func (d *VarDecl) String() string {
	return fmt.Sprintf("FuncDecl{NameList:%v}", d.NameList)
}

type decl struct{ node }

func (*decl) aDecl() {}

// All declarations belonging to the same group point to the same Group node.
type Group struct {
	_ int // not empty so we are guaranteed different Group instances
}

// ----------------------------------------------------------------------------
// Expressions

func NewName(pos Pos, value string) *Name {
	n := new(Name)
	n.pos = pos
	n.Value = value
	return n
}

type (
	Expr interface {
		Node
		typeInfo
		aExpr()
	}

	// Placeholder for an expression that failed to parse
	// correctly and where we can't provide a better node.
	BadExpr struct {
		expr
	}

	// Value
	Name struct {
		Value string
		expr
	}

	// Value
	BasicLit struct {
		Value string
		Kind  LitKind
		Bad   bool // true means the literal Value has syntax errors
		expr
	}

	// Type { ElemList[0], ElemList[1], ... }
	CompositeLit struct {
		Type     Expr // nil means no literal type
		ElemList []Expr
		NKeys    int // number of elements with keys
		Rbrace   Pos
		expr
	}

	// Key: Value
	KeyValueExpr struct {
		Key, Value Expr
		expr
	}

	// func Type { Body }
	FuncLit struct {
		Type *FuncType
		Body *BlockStmt
		expr
	}

	// (X)
	ParenExpr struct {
		X Expr
		expr
	}

	// X.Sel
	SelectorExpr struct {
		X   Expr
		Sel *Name
		expr
	}

	// X[Index]
	// X[T1, T2, ...] (with Ti = Index.(*ListExpr).ElemList[i])
	IndexExpr struct {
		X     Expr
		Index Expr
		expr
	}

	// X[Index[0] : Index[1] : Index[2]]
	SliceExpr struct {
		X     Expr
		Index [3]Expr
		// Full indicates whether this is a simple or full slice expression.
		// In a valid AST, this is equivalent to Index[2] != nil.
		// TODO(mdempsky): This is only needed to report the "3-index
		// slice of string" error when Index[2] is missing.
		Full bool
		expr
	}

	// X.(Type)
	AssertExpr struct {
		X    Expr
		Type Expr
		expr
	}

	// X.(type)
	// Lhs := X.(type)
	TypeSwitchGuard struct {
		Lhs *Name // nil means no Lhs :=
		X   Expr  // X.(type)
		expr
	}

	Operation struct {
		Op   Operator
		X, Y Expr // Y == nil means unary expression
		expr
	}

	// Fun(ArgList[0], ArgList[1], ...)
	CallExpr struct {
		Fun     Expr
		ArgList []Expr // nil means no arguments
		HasDots bool   // last argument is followed by ...
		expr
	}

	// ElemList[0], ElemList[1], ...
	ListExpr struct {
		ElemList []Expr
		expr
	}

	// [Len]Elem
	ArrayType struct {
		// TODO(gri) consider using Name{"..."} instead of nil (permits attaching of comments)
		Len  Expr // nil means Len is ...
		Elem Expr
		expr
	}

	// []Elem
	SliceType struct {
		Elem Expr
		expr
	}

	// ...Elem
	DotsType struct {
		Elem Expr
		expr
	}

	// struct { FieldList[0] TagList[0]; FieldList[1] TagList[1]; ... }
	StructType struct {
		FieldList []*Field
		TagList   []*BasicLit // i >= len(TagList) || TagList[i] == nil means no tag for field i
		expr
	}

	// Name Type
	//      Type
	Field struct {
		Name *Name // nil means anonymous field/parameter (structs/parameters), or embedded element (interfaces)
		Type Expr  // field names declared in a list share the same Type (identical pointers)
		node
	}

	// interface { MethodList[0]; MethodList[1]; ... }
	InterfaceType struct {
		MethodList []*Field
		expr
	}

	FuncType struct {
		ParamList  []*Field
		ResultList []*Field
		expr
	}

	// map[Key]Value
	MapType struct {
		Key, Value Expr
		expr
	}

	//   chan Elem
	// <-chan Elem
	// chan<- Elem
	ChanType struct {
		Dir  ChanDir // 0 means no direction
		Elem Expr
		expr
	}
)

type expr struct {
	node
	typeAndValue // After typechecking, contains the results of typechecking this expression.
}

func (*expr) aExpr() {}

type ChanDir uint

const (
	_ ChanDir = iota
	SendOnly
	RecvOnly
)

// ----------------------------------------------------------------------------
// Statements

type (
	Stmt interface {
		Node
		aStmt()
	}

	SimpleStmt interface {
		Stmt
		aSimpleStmt()
	}

	EmptyStmt struct {
		simpleStmt
	}

	LabeledStmt struct {
		Label *Name
		Stmt  Stmt
		stmt
	}

	BlockStmt struct {
		List   []Stmt
		Rbrace Pos
		stmt
	}

	ExprStmt struct {
		X Expr
		simpleStmt
	}

	SendStmt struct {
		Chan, Value Expr // Chan <- Value
		simpleStmt
	}

	DeclStmt struct {
		DeclList []Decl
		stmt
	}

	AssignStmt struct {
		Op       Operator // 0 means no operation
		Lhs, Rhs Expr     // Rhs == nil means Lhs++ (Op == Add) or Lhs-- (Op == Sub)
		simpleStmt
	}

	BranchStmt struct {
		Tok   token // Break, Continue, Fallthrough, or Goto
		Label *Name
		// Target is the continuation of the control flow after executing
		// the branch; it is computed by the parser if CheckBranches is set.
		// Target is a *LabeledStmt for gotos, and a *SwitchStmt, *SelectStmt,
		// or *ForStmt for breaks and continues, depending on the context of
		// the branch. Target is not set for fallthroughs.
		Target Stmt
		stmt
	}

	CallStmt struct {
		Tok     token // Go or Defer
		Call    Expr
		DeferAt Expr // argument to runtime.deferprocat
		stmt
	}

	ReturnStmt struct {
		Results Expr // nil means no explicit return values
		stmt
	}

	IfStmt struct {
		Init SimpleStmt
		Cond Expr
		Then *BlockStmt
		Else Stmt // either nil, *IfStmt, or *BlockStmt
		stmt
	}

	ForStmt struct {
		Init SimpleStmt // incl. *RangeClause
		Cond Expr
		Post SimpleStmt
		Body *BlockStmt
		stmt
	}

	SwitchStmt struct {
		Init   SimpleStmt
		Tag    Expr // incl. *TypeSwitchGuard
		Body   []*CaseClause
		Rbrace Pos
		stmt
	}

	SelectStmt struct {
		Body   []*CommClause
		Rbrace Pos
		stmt
	}
)

type (
	RangeClause struct {
		Lhs Expr // nil means no Lhs = or Lhs :=
		Def bool // means :=
		X   Expr // range X
		simpleStmt
	}

	CaseClause struct {
		Cases Expr // nil means default clause
		Body  []Stmt
		Colon Pos
		node
	}

	CommClause struct {
		Comm  SimpleStmt // send or receive stmt; nil means default clause
		Body  []Stmt
		Colon Pos
		node
	}
)

type stmt struct{ node }

func (stmt) aStmt() {}

type simpleStmt struct {
	stmt
}

func (simpleStmt) aSimpleStmt() {}

// ----------------------------------------------------------------------------
// Comments

// TODO(gri) Consider renaming to CommentPos, CommentPlacement, etc.
// Kind = Above doesn't make much sense.
type CommentKind uint

const (
	Above CommentKind = iota
	Below
	Left
	Right
)

type Comment struct {
	Kind CommentKind
	Text string
	Next *Comment
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/operator_string.go ===
```go
// Code generated by "stringer -type Operator -linecomment tokens.go"; DO NOT EDIT.

package syntax

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Def-1]
	_ = x[Not-2]
	_ = x[Recv-3]
	_ = x[Tilde-4]
	_ = x[OrOr-5]
	_ = x[AndAnd-6]
	_ = x[Eql-7]
	_ = x[Neq-8]
	_ = x[Lss-9]
	_ = x[Leq-10]
	_ = x[Gtr-11]
	_ = x[Geq-12]
	_ = x[Add-13]
	_ = x[Sub-14]
	_ = x[Or-15]
	_ = x[Xor-16]
	_ = x[Mul-17]
	_ = x[Div-18]
	_ = x[Rem-19]
	_ = x[And-20]
	_ = x[AndNot-21]
	_ = x[Shl-22]
	_ = x[Shr-23]
}

const _Operator_name = ":!<-~||&&==!=<<=>>=+-|^*/%&&^<<>>"

var _Operator_index = [...]uint8{0, 1, 2, 4, 5, 7, 9, 11, 13, 14, 16, 17, 19, 20, 21, 22, 23, 24, 25, 26, 27, 29, 31, 33}

func (i Operator) String() string {
	idx := int(i) - 1
	if i < 1 || idx >= len(_Operator_index)-1 {
		return "Operator(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Operator_name[_Operator_index[idx]:_Operator_index[idx+1]]
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/parser.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import (
	"fmt"
	"go/build/constraint"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

const debug = false
const trace = false

type parser struct {
	file  *PosBase
	errh  ErrorHandler
	mode  Mode
	pragh PragmaHandler
	scanner

	base      *PosBase // current position base
	first     error    // first error encountered
	errcnt    int      // number of errors encountered
	pragma    Pragma   // pragmas
	goVersion string   // Go version from //go:build line

	top    bool   // in top of file (before package clause)
	fnest  int    // function nesting level (for error handling)
	xnest  int    // expression nesting level (for complit ambiguity resolution)
	indent []byte // tracing support
}

func (p *parser) init(file *PosBase, r io.Reader, errh ErrorHandler, pragh PragmaHandler, mode Mode) {
	p.top = true
	p.file = file
	p.errh = errh
	p.mode = mode
	p.pragh = pragh
	p.scanner.init(
		r,
		// Error and directive handler for scanner.
		// Because the (line, col) positions passed to the
		// handler is always at or after the current reading
		// position, it is safe to use the most recent position
		// base to compute the corresponding Pos value.
		func(line, col uint, msg string) {
			if msg[0] != '/' {
				p.errorAt(p.posAt(line, col), msg)
				return
			}

			// otherwise it must be a comment containing a line or go: directive.
			// //line directives must be at the start of the line (column colbase).
			// /*line*/ directives can be anywhere in the line.
			text := commentText(msg)
			if (col == colbase || msg[1] == '*') && strings.HasPrefix(text, "line ") {
				var pos Pos // position immediately following the comment
				if msg[1] == '/' {
					// line comment (newline is part of the comment)
					pos = MakePos(p.file, line+1, colbase)
				} else {
					// regular comment
					// (if the comment spans multiple lines it's not
					// a valid line directive and will be discarded
					// by updateBase)
					pos = MakePos(p.file, line, col+uint(len(msg)))
				}
				p.updateBase(pos, line, col+2+5, text[5:]) // +2 to skip over // or /*
				return
			}

			// go: directive (but be conservative and test)
			if strings.HasPrefix(text, "go:") {
				if p.top && strings.HasPrefix(msg, "//go:build") {
					if x, err := constraint.Parse(msg); err == nil {
						p.goVersion = constraint.GoVersion(x)
					}
				}
				if pragh != nil {
					p.pragma = pragh(p.posAt(line, col+2), p.scanner.blank, text, p.pragma) // +2 to skip over // or /*
				}
			}
		},
		directives,
	)

	p.base = file
	p.first = nil
	p.errcnt = 0
	p.pragma = nil

	p.fnest = 0
	p.xnest = 0
	p.indent = nil
}

// takePragma returns the current parsed pragmas
// and clears them from the parser state.
func (p *parser) takePragma() Pragma {
	prag := p.pragma
	p.pragma = nil
	return prag
}

// clearPragma is called at the end of a statement or
// other Go form that does NOT accept a pragma.
// It sends the pragma back to the pragma handler
// to be reported as unused.
func (p *parser) clearPragma() {
	if p.pragma != nil {
		p.pragh(p.pos(), p.scanner.blank, "", p.pragma)
		p.pragma = nil
	}
}

// updateBase sets the current position base to a new line base at pos.
// The base's filename, line, and column values are extracted from text
// which is positioned at (tline, tcol) (only needed for error messages).
func (p *parser) updateBase(pos Pos, tline, tcol uint, text string) {
	i, n, ok := trailingDigits(text)
	if i == 0 {
		return // ignore (not a line directive)
	}
	// i > 0

	if !ok {
		// text has a suffix :xxx but xxx is not a number
		p.errorAt(p.posAt(tline, tcol+i), "invalid line number: "+text[i:])
		return
	}

	var line, col uint
	i2, n2, ok2 := trailingDigits(text[:i-1])
	if ok2 {
		//line filename:line:col
		i, i2 = i2, i
		line, col = n2, n
		if col == 0 || col > PosMax {
			p.errorAt(p.posAt(tline, tcol+i2), "invalid column number: "+text[i2:])
			return
		}
		text = text[:i2-1] // lop off ":col"
	} else {
		//line filename:line
		line = n
	}

	if line == 0 || line > PosMax {
		p.errorAt(p.posAt(tline, tcol+i), "invalid line number: "+text[i:])
		return
	}

	// If we have a column (//line filename:line:col form),
	// an empty filename means to use the previous filename.
	filename := text[:i-1] // lop off ":line"
	trimmed := false
	if filename == "" && ok2 {
		filename = p.base.Filename()
		trimmed = p.base.Trimmed()
	} else if filename != "" {
		filename = filepath.Clean(filename)
		if !filepath.IsAbs(filename) {
			if dir := filepath.Dir(p.file.Filename()); dir != "." {
				filename = filepath.Join(dir, filename)
			}
		}
	}

	p.base = NewLineBase(pos, filename, trimmed, line, col)
}

func commentText(s string) string {
	if s[:2] == "/*" {
		return s[2 : len(s)-2] // lop off /* and */
	}

	// line comment (does not include newline)
	// (on Windows, the line comment may end in \r\n)
	i := len(s)
	if s[i-1] == '\r' {
		i--
	}
	return s[2:i] // lop off //, and \r at end, if any
}

func trailingDigits(text string) (uint, uint, bool) {
	i := strings.LastIndexByte(text, ':') // look from right (Windows filenames may contain ':')
	if i < 0 {
		return 0, 0, false // no ':'
	}
	// i >= 0
	n, err := strconv.ParseUint(text[i+1:], 10, 0)
	return uint(i + 1), uint(n), err == nil
}

func (p *parser) got(tok token) bool {
	if p.tok == tok {
		p.next()
		return true
	}
	return false
}

func (p *parser) want(tok token) {
	if !p.got(tok) {
		p.syntaxError("expected " + tokstring(tok))
		p.advance()
	}
}

// gotAssign is like got(_Assign) but it also accepts ":="
// (and reports an error) for better parser error recovery.
func (p *parser) gotAssign() bool {
	switch p.tok {
	case _Define:
		p.syntaxError("expected =")
		fallthrough
	case _Assign:
		p.next()
		return true
	}
	return false
}

// ----------------------------------------------------------------------------
// Error handling

// posAt returns the Pos value for (line, col) and the current position base.
func (p *parser) posAt(line, col uint) Pos {
	return MakePos(p.base, line, col)
}

// errorAt reports an error at the given position.
func (p *parser) errorAt(pos Pos, msg string) {
	err := Error{pos, msg}
	if p.first == nil {
		p.first = err
	}
	p.errcnt++
	if p.errh == nil {
		panic(p.first)
	}
	p.errh(err)
}

// syntaxErrorAt reports a syntax error at the given position.
func (p *parser) syntaxErrorAt(pos Pos, msg string) {
	if trace {
		p.print("syntax error: " + msg)
	}

	if p.tok == _EOF && p.first != nil {
		return // avoid meaningless follow-up errors
	}

	// add punctuation etc. as needed to msg
	switch {
	case msg == "":
		// nothing to do
	case strings.HasPrefix(msg, "in "), strings.HasPrefix(msg, "at "), strings.HasPrefix(msg, "after "):
		msg = " " + msg
	case strings.HasPrefix(msg, "expected "):
		msg = ", " + msg
	default:
		// plain error - we don't care about current token
		p.errorAt(pos, "syntax error: "+msg)
		return
	}

	// determine token string
	var tok string
	switch p.tok {
	case _Name:
		tok = "name " + p.lit
	case _Semi:
		tok = p.lit
	case _Literal:
		tok = "literal " + p.lit
	case _Operator:
		tok = p.op.String()
	case _AssignOp:
		tok = p.op.String() + "="
	case _IncOp:
		tok = p.op.String()
		tok += tok
	default:
		tok = tokstring(p.tok)
	}

	// TODO(gri) This may print "unexpected X, expected Y".
	//           Consider "got X, expected Y" in this case.
	p.errorAt(pos, "syntax error: unexpected "+tok+msg)
}

// tokstring returns the English word for selected punctuation tokens
// for more readable error messages. Use tokstring (not tok.String())
// for user-facing (error) messages; use tok.String() for debugging
// output.
func tokstring(tok token) string {
	switch tok {
	case _Comma:
		return "comma"
	case _Semi:
		return "semicolon or newline"
	}
	s := tok.String()
	if _Break <= tok && tok <= _Var {
		return "keyword " + s
	}
	return s
}

// Convenience methods using the current token position.
func (p *parser) pos() Pos               { return p.posAt(p.line, p.col) }
func (p *parser) error(msg string)       { p.errorAt(p.pos(), msg) }
func (p *parser) syntaxError(msg string) { p.syntaxErrorAt(p.pos(), msg) }

// The stopset contains keywords that start a statement.
// They are good synchronization points in case of syntax
// errors and (usually) shouldn't be skipped over.
const stopset uint64 = 1<<_Break |
	1<<_Const |
	1<<_Continue |
	1<<_Defer |
	1<<_Fallthrough |
	1<<_For |
	1<<_Go |
	1<<_Goto |
	1<<_If |
	1<<_Return |
	1<<_Select |
	1<<_Switch |
	1<<_Type |
	1<<_Var

// advance consumes tokens until it finds a token of the stopset or followlist.
// The stopset is only considered if we are inside a function (p.fnest > 0).
// The followlist is the list of valid tokens that can follow a production;
// if it is empty, exactly one (non-EOF) token is consumed to ensure progress.
func (p *parser) advance(followlist ...token) {
	if trace {
		p.print(fmt.Sprintf("advance %s", followlist))
	}

	// compute follow set
	// (not speed critical, advance is only called in error situations)
	var followset uint64 = 1 << _EOF // don't skip over EOF
	if len(followlist) > 0 {
		if p.fnest > 0 {
			followset |= stopset
		}
		for _, tok := range followlist {
			followset |= 1 << tok
		}
	}

	for !contains(followset, p.tok) {
		if trace {
			p.print("skip " + p.tok.String())
		}
		p.next()
		if len(followlist) == 0 {
			break
		}
	}

	if trace {
		p.print("next " + p.tok.String())
	}
}

// usage: defer p.trace(msg)()
func (p *parser) trace(msg string) func() {
	p.print(msg + " (")
	const tab = ". "
	p.indent = append(p.indent, tab...)
	return func() {
		p.indent = p.indent[:len(p.indent)-len(tab)]
		if x := recover(); x != nil {
			panic(x) // skip print_trace
		}
		p.print(")")
	}
}

func (p *parser) print(msg string) {
	fmt.Printf("%5d: %s%s\n", p.line, p.indent, msg)
}

// ----------------------------------------------------------------------------
// Package files
//
// Parse methods are annotated with matching Go productions as appropriate.
// The annotations are intended as guidelines only since a single Go grammar
// rule may be covered by multiple parse methods and vice versa.
//
// Excluding methods returning slices, parse methods named xOrNil may return
// nil; all others are expected to return a valid non-nil node.

// SourceFile = PackageClause ";" { ImportDecl ";" } { TopLevelDecl ";" } .
func (p *parser) fileOrNil() *File {
	if trace {
		defer p.trace("file")()
	}

	f := new(File)
	f.pos = p.pos()

	// PackageClause
	f.GoVersion = p.goVersion
	p.top = false
	if !p.got(_Package) {
		p.syntaxError("package statement must be first")
		return nil
	}
	f.Pragma = p.takePragma()
	f.PkgName = p.name()
	p.want(_Semi)

	// don't bother continuing if package clause has errors
	if p.first != nil {
		return nil
	}

	// Accept import declarations anywhere for error tolerance, but complain.
	// { ( ImportDecl | TopLevelDecl ) ";" }
	prev := _Import
	for p.tok != _EOF {
		if p.tok == _Import && prev != _Import {
			p.syntaxError("imports must appear before other declarations")
		}
		prev = p.tok

		switch p.tok {
		case _Import:
			p.next()
			f.DeclList = p.appendGroup(f.DeclList, p.importDecl)

		case _Const:
			p.next()
			f.DeclList = p.appendGroup(f.DeclList, p.constDecl)

		case _Type:
			p.next()
			f.DeclList = p.appendGroup(f.DeclList, p.typeDecl)

		case _Var:
			p.next()
			f.DeclList = p.appendGroup(f.DeclList, p.varDecl)

		case _Func:
			p.next()
			if d := p.funcDeclOrNil(); d != nil {
				f.DeclList = append(f.DeclList, d)
			}

		default:
			if p.tok == _Lbrace && len(f.DeclList) > 0 && isEmptyFuncDecl(f.DeclList[len(f.DeclList)-1]) {
				// opening { of function declaration on next line
				p.syntaxError("unexpected semicolon or newline before {")
			} else {
				p.syntaxError("non-declaration statement outside function body")
			}
			p.advance(_Import, _Const, _Type, _Var, _Func)
			continue
		}

		// Reset p.pragma BEFORE advancing to the next token (consuming ';')
		// since comments before may set pragmas for the next function decl.
		p.clearPragma()

		if p.tok != _EOF && !p.got(_Semi) {
			p.syntaxError("after top level declaration")
			p.advance(_Import, _Const, _Type, _Var, _Func)
		}
	}
	// p.tok == _EOF

	p.clearPragma()
	f.EOF = p.pos()

	return f
}

func isEmptyFuncDecl(dcl Decl) bool {
	f, ok := dcl.(*FuncDecl)
	return ok && f.Body == nil
}

// ----------------------------------------------------------------------------
// Declarations

// list parses a possibly empty, sep-separated list of elements, optionally
// followed by sep, and closed by close (or EOF). sep must be one of _Comma
// or _Semi, and close must be one of _Rparen, _Rbrace, or _Rbrack.
//
// For each list element, f is called. Specifically, unless we're at close
// (or EOF), f is called at least once. After f returns true, no more list
// elements are accepted. list returns the position of the closing token.
//
// list = [ f { sep f } [sep] ] close .
func (p *parser) list(context string, sep, close token, f func() bool) Pos {
	if debug && (sep != _Comma && sep != _Semi || close != _Rparen && close != _Rbrace && close != _Rbrack) {
		panic("invalid sep or close argument for list")
	}

	done := false
	for p.tok != _EOF && p.tok != close && !done {
		done = f()
		// sep is optional before close
		if !p.got(sep) && p.tok != close {
			p.syntaxError(fmt.Sprintf("in %s; possibly missing %s or %s", context, tokstring(sep), tokstring(close)))
			p.advance(_Rparen, _Rbrack, _Rbrace)
			if p.tok != close {
				// position could be better but we had an error so we don't care
				return p.pos()
			}
		}
	}

	pos := p.pos()
	p.want(close)
	return pos
}

// appendGroup(f) = f | "(" { f ";" } ")" . // ";" is optional before ")"
func (p *parser) appendGroup(list []Decl, f func(*Group) Decl) []Decl {
	if p.tok == _Lparen {
		g := new(Group)
		p.clearPragma()
		p.next() // must consume "(" after calling clearPragma!
		p.list("grouped declaration", _Semi, _Rparen, func() bool {
			if x := f(g); x != nil {
				list = append(list, x)
			}
			return false
		})
	} else {
		if x := f(nil); x != nil {
			list = append(list, x)
		}
	}
	return list
}

// ImportSpec = [ "." | PackageName ] ImportPath .
// ImportPath = string_lit .
func (p *parser) importDecl(group *Group) Decl {
	if trace {
		defer p.trace("importDecl")()
	}

	d := new(ImportDecl)
	d.pos = p.pos()
	d.Group = group
	d.Pragma = p.takePragma()

	switch p.tok {
	case _Name:
		d.LocalPkgName = p.name()
	case _Dot:
		d.LocalPkgName = NewName(p.pos(), ".")
		p.next()
	}
	d.Path = p.oliteral()
	if d.Path == nil {
		p.syntaxError("missing import path")
		p.advance(_Semi, _Rparen)
		return d
	}
	if !d.Path.Bad && d.Path.Kind != StringLit {
		p.syntaxErrorAt(d.Path.Pos(), "import path must be a string")
		d.Path.Bad = true
	}
	// d.Path.Bad || d.Path.Kind == StringLit

	return d
}

// ConstSpec = IdentifierList [ [ Type ] "=" ExpressionList ] .
func (p *parser) constDecl(group *Group) Decl {
	if trace {
		defer p.trace("constDecl")()
	}

	d := new(ConstDecl)
	d.pos = p.pos()
	d.Group = group
	d.Pragma = p.takePragma()

	d.NameList = p.nameList(p.name())
	if p.tok != _EOF && p.tok != _Semi && p.tok != _Rparen {
		d.Type = p.typeOrNil()
		if p.gotAssign() {
			d.Values = p.exprList()
		}
	}

	return d
}

// TypeSpec = identifier [ TypeParams ] [ "=" ] Type .
func (p *parser) typeDecl(group *Group) Decl {
	if trace {
		defer p.trace("typeDecl")()
	}

	d := new(TypeDecl)
	d.pos = p.pos()
	d.Group = group
	d.Pragma = p.takePragma()

	d.Name = p.name()
	if p.tok == _Lbrack {
		// d.Name "[" ...
		// array/slice type or type parameter list
		pos := p.pos()
		p.next()
		switch p.tok {
		case _Name:
			// We may have an array type or a type parameter list.
			// In either case we expect an expression x (which may
			// just be a name, or a more complex expression) which
			// we can analyze further.
			//
			// A type parameter list may have a type bound starting
			// with a "[" as in: P []E. In that case, simply parsing
			// an expression would lead to an error: P[] is invalid.
			// But since index or slice expressions are never constant
			// and thus invalid array length expressions, if the name
			// is followed by "[" it must be the start of an array or
			// slice constraint. Only if we don't see a "[" do we
			// need to parse a full expression. Notably, name <- x
			// is not a concern because name <- x is a statement and
			// not an expression.
			var x Expr = p.name()
			if p.tok != _Lbrack {
				// To parse the expression starting with name, expand
				// the call sequence we would get by passing in name
				// to parser.expr, and pass in name to parser.pexpr.
				p.xnest++
				x = p.binaryExpr(p.pexpr(x, false), 0)
				p.xnest--
			}
			// Analyze expression x. If we can split x into a type parameter
			// name, possibly followed by a type parameter type, we consider
			// this the start of a type parameter list, with some caveats:
			// a single name followed by "]" tilts the decision towards an
			// array declaration; a type parameter type that could also be
			// an ordinary expression but which is followed by a comma tilts
			// the decision towards a type parameter list.
			if pname, ptype := extractName(x, p.tok == _Comma); pname != nil && (ptype != nil || p.tok != _Rbrack) {
				// d.Name "[" pname ...
				// d.Name "[" pname ptype ...
				// d.Name "[" pname ptype "," ...
				d.TParamList = p.paramList(pname, ptype, _Rbrack, true, false) // ptype may be nil
				d.Alias = p.gotAssign()
				d.Type = p.typeOrNil()
			} else {
				// d.Name "[" pname "]" ...
				// d.Name "[" x ...
				d.Type = p.arrayType(pos, x)
			}
		case _Rbrack:
			// d.Name "[" "]" ...
			p.next()
			d.Type = p.sliceType(pos)
		default:
			// d.Name "[" ...
			d.Type = p.arrayType(pos, nil)
		}
	} else {
		d.Alias = p.gotAssign()
		d.Type = p.typeOrNil()
	}

	if d.Type == nil {
		d.Type = p.badExpr()
		p.syntaxError("in type declaration")
		p.advance(_Semi, _Rparen)
	}

	return d
}

// extractName splits the expression x into (name, expr) if syntactically
// x can be written as name expr. The split only happens if expr is a type
// element (per the isTypeElem predicate) or if force is set.
// If x is just a name, the result is (name, nil). If the split succeeds,
// the result is (name, expr). Otherwise the result is (nil, x).
// Examples:
//
//	x           force    name    expr
//	------------------------------------
//	P*[]int     T/F      P       *[]int
//	P*E         T        P       *E
//	P*E         F        nil     P*E
//	P([]int)    T/F      P       []int
//	P(E)        T        P       E
//	P(E)        F        nil     P(E)
//	P*E|F|~G    T/F      P       *E|F|~G
//	P*E|F|G     T        P       *E|F|G
//	P*E|F|G     F        nil     P*E|F|G
func extractName(x Expr, force bool) (*Name, Expr) {
	switch x := x.(type) {
	case *Name:
		return x, nil
	case *Operation:
		if x.Y == nil {
			break // unary expr
		}
		switch x.Op {
		case Mul:
			if name, _ := x.X.(*Name); name != nil && (force || isTypeElem(x.Y)) {
				// x = name *x.Y
				op := *x
				op.X, op.Y = op.Y, nil // change op into unary *op.Y
				return name, &op
			}
		case Or:
			if name, lhs := extractName(x.X, force || isTypeElem(x.Y)); name != nil && lhs != nil {
				// x = name lhs|x.Y
				op := *x
				op.X = lhs
				return name, &op
			}
		}
	case *CallExpr:
		if name, _ := x.Fun.(*Name); name != nil {
			if len(x.ArgList) == 1 && !x.HasDots && (force || isTypeElem(x.ArgList[0])) {
				// The parser doesn't keep unnecessary parentheses.
				// Set the flag below to keep them, for testing
				// (see go.dev/issues/69206).
				const keep_parens = false
				if keep_parens {
					// x = name (x.ArgList[0])
					px := new(ParenExpr)
					px.pos = x.pos // position of "(" in call
					px.X = x.ArgList[0]
					return name, px
				} else {
					// x = name x.ArgList[0]
					return name, Unparen(x.ArgList[0])
				}
			}
		}
	}
	return nil, x
}

// isTypeElem reports whether x is a (possibly parenthesized) type element expression.
// The result is false if x could be a type element OR an ordinary (value) expression.
func isTypeElem(x Expr) bool {
	switch x := x.(type) {
	case *ArrayType, *StructType, *FuncType, *InterfaceType, *SliceType, *MapType, *ChanType:
		return true
	case *Operation:
		return isTypeElem(x.X) || (x.Y != nil && isTypeElem(x.Y)) || x.Op == Tilde
	case *ParenExpr:
		return isTypeElem(x.X)
	}
	return false
}

// VarSpec = IdentifierList ( Type [ "=" ExpressionList ] | "=" ExpressionList ) .
func (p *parser) varDecl(group *Group) Decl {
	if trace {
		defer p.trace("varDecl")()
	}

	d := new(VarDecl)
	d.pos = p.pos()
	d.Group = group
	d.Pragma = p.takePragma()

	d.NameList = p.nameList(p.name())
	if p.gotAssign() {
		d.Values = p.exprList()
	} else {
		d.Type = p.type_()
		if p.gotAssign() {
			d.Values = p.exprList()
		}
	}

	return d
}

// FunctionDecl = "func" FunctionName [ TypeParams ] ( Function | Signature ) .
// FunctionName = identifier .
// Function     = Signature FunctionBody .
// MethodDecl   = "func" Receiver MethodName ( Function | Signature ) .
// Receiver     = Parameters .
func (p *parser) funcDeclOrNil() *FuncDecl {
	if trace {
		defer p.trace("funcDecl")()
	}

	f := new(FuncDecl)
	f.pos = p.pos()
	f.Pragma = p.takePragma()

	hasRecv := false
	if p.got(_Lparen) {
		hasRecv = true
		rcvr := p.paramList(nil, nil, _Rparen, false, false)
		switch len(rcvr) {
		case 0:
			p.error("method has no receiver")
		default:
			p.error("method has multiple receivers")
			fallthrough
		case 1:
			f.Recv = rcvr[0]
		}
	}

	if p.tok == _Name {
		f.Name = p.name()
		f.TParamList, f.Type = p.funcType("")
	} else {
		f.Name = NewName(p.pos(), "_")
		f.Type = new(FuncType)
		f.Type.pos = p.pos()
		msg := "expected name or ("
		if hasRecv {
			msg = "expected name"
		}
		p.syntaxError(msg)
		p.advance(_Lbrace, _Semi)
	}

	if p.tok == _Lbrace {
		f.Body = p.funcBody()
	}

	return f
}

func (p *parser) funcBody() *BlockStmt {
	p.fnest++
	errcnt := p.errcnt
	body := p.blockStmt("")
	p.fnest--

	// Don't check branches if there were syntax errors in the function
	// as it may lead to spurious errors (e.g., see test/switch2.go) or
	// possibly crashes due to incomplete syntax trees.
	if p.mode&CheckBranches != 0 && errcnt == p.errcnt {
		checkBranches(body, p.errh)
	}

	return body
}

// ----------------------------------------------------------------------------
// Expressions

func (p *parser) expr() Expr {
	if trace {
		defer p.trace("expr")()
	}

	return p.binaryExpr(nil, 0)
}

// Expression = UnaryExpr | Expression binary_op Expression .
func (p *parser) binaryExpr(x Expr, prec int) Expr {
	// don't trace binaryExpr - only leads to overly nested trace output

	if x == nil {
		x = p.unaryExpr()
	}
	for (p.tok == _Operator || p.tok == _Star) && p.prec > prec {
		t := new(Operation)
		t.pos = p.pos()
		t.Op = p.op
		tprec := p.prec
		p.next()
		t.X = x
		t.Y = p.binaryExpr(nil, tprec)
		x = t
	}
	return x
}

// UnaryExpr = PrimaryExpr | unary_op UnaryExpr .
func (p *parser) unaryExpr() Expr {
	if trace {
		defer p.trace("unaryExpr")()
	}

	switch p.tok {
	case _Operator, _Star:
		switch p.op {
		case Mul, Add, Sub, Not, Xor, Tilde:
			x := new(Operation)
			x.pos = p.pos()
			x.Op = p.op
			p.next()
			x.X = p.unaryExpr()
			return x

		case And:
			x := new(Operation)
			x.pos = p.pos()
			x.Op = And
			p.next()
			// unaryExpr may have returned a parenthesized composite literal
			// (see comment in operand) - remove parentheses if any
			x.X = Unparen(p.unaryExpr())
			return x
		}

	case _Arrow:
		// receive op (<-x) or receive-only channel (<-chan E)
		pos := p.pos()
		p.next()

		// If the next token is _Chan we still don't know if it is
		// a channel (<-chan int) or a receive op (<-chan int(ch)).
		// We only know once we have found the end of the unaryExpr.

		x := p.unaryExpr()

		// There are two cases:
		//
		//   <-chan...  => <-x is a channel type
		//   <-x        => <-x is a receive operation
		//
		// In the first case, <- must be re-associated with
		// the channel type parsed already:
		//
		//   <-(chan E)   =>  (<-chan E)
		//   <-(chan<-E)  =>  (<-chan (<-E))

		if _, ok := x.(*ChanType); ok {
			// x is a channel type => re-associate <-
			dir := SendOnly
			t := x
			for dir == SendOnly {
				c, ok := t.(*ChanType)
				if !ok {
					break
				}
				dir = c.Dir
				if dir == RecvOnly {
					// t is type <-chan E but <-<-chan E is not permitted
					// (report same error as for "type _ <-<-chan E")
					p.syntaxError("unexpected <-, expected chan")
					// already progressed, no need to advance
				}
				c.Dir = RecvOnly
				t = c.Elem
			}
			if dir == SendOnly {
				// channel dir is <- but channel element E is not a channel
				// (report same error as for "type _ <-chan<-E")
				p.syntaxError(fmt.Sprintf("unexpected %s, expected chan", String(t)))
				// already progressed, no need to advance
			}
			return x
		}

		// x is not a channel type => we have a receive op
		o := new(Operation)
		o.pos = pos
		o.Op = Recv
		o.X = x
		return o
	}

	// TODO(mdempsky): We need parens here so we can report an
	// error for "(x) := true". It should be possible to detect
	// and reject that more efficiently though.
	return p.pexpr(nil, true)
}

// callStmt parses call-like statements that can be preceded by 'defer' and 'go'.
func (p *parser) callStmt() *CallStmt {
	if trace {
		defer p.trace("callStmt")()
	}

	s := new(CallStmt)
	s.pos = p.pos()
	s.Tok = p.tok // _Defer or _Go
	p.next()

	x := p.pexpr(nil, p.tok == _Lparen) // keep_parens so we can report error below
	if t := Unparen(x); t != x {
		p.errorAt(x.Pos(), fmt.Sprintf("expression in %s must not be parenthesized", s.Tok))
		// already progressed, no need to advance
		x = t
	}

	s.Call = x
	return s
}

// Operand     = Literal | OperandName | MethodExpr | "(" Expression ")" .
// Literal     = BasicLit | CompositeLit | FunctionLit .
// BasicLit    = int_lit | float_lit | imaginary_lit | rune_lit | string_lit .
// OperandName = identifier | QualifiedIdent.
func (p *parser) operand(keep_parens bool) Expr {
	if trace {
		defer p.trace("operand " + p.tok.String())()
	}

	switch p.tok {
	case _Name:
		return p.name()

	case _Literal:
		return p.oliteral()

	case _Lparen:
		pos := p.pos()
		p.next()
		p.xnest++
		x := p.expr()
		p.xnest--
		p.want(_Rparen)

		// Optimization: Record presence of ()'s only where needed
		// for error reporting. Don't bother in other cases; it is
		// just a waste of memory and time.
		//
		// Parentheses are not permitted around T in a composite
		// literal T{}. If the next token is a {, assume x is a
		// composite literal type T (it may not be, { could be
		// the opening brace of a block, but we don't know yet).
		if p.tok == _Lbrace {
			keep_parens = true
		}

		// Parentheses are also not permitted around the expression
		// in a go/defer statement. In that case, operand is called
		// with keep_parens set.
		if keep_parens {
			px := new(ParenExpr)
			px.pos = pos
			px.X = x
			x = px
		}
		return x

	case _Func:
		pos := p.pos()
		p.next()
		_, ftyp := p.funcType("function type")
		if p.tok == _Lbrace {
			p.xnest++

			f := new(FuncLit)
			f.pos = pos
			f.Type = ftyp
			f.Body = p.funcBody()

			p.xnest--
			return f
		}
		return ftyp

	case _Lbrack, _Chan, _Map, _Struct, _Interface:
		return p.type_() // othertype

	default:
		x := p.badExpr()
		p.syntaxError("expected expression")
		p.advance(_Rparen, _Rbrack, _Rbrace)
		return x
	}

	// Syntactically, composite literals are operands. Because a complit
	// type may be a qualified identifier which is handled by pexpr
	// (together with selector expressions), complits are parsed there
	// as well (operand is only called from pexpr).
}

// pexpr parses a PrimaryExpr.
//
//	PrimaryExpr =
//		Operand |
//		Conversion |
//		PrimaryExpr Selector |
//		PrimaryExpr Index |
//		PrimaryExpr Slice |
//		PrimaryExpr TypeAssertion |
//		PrimaryExpr Arguments .
//
//	Selector       = "." identifier .
//	Index          = "[" Expression "]" .
//	Slice          = "[" ( [ Expression ] ":" [ Expression ] ) |
//	                     ( [ Expression ] ":" Expression ":" Expression )
//	                 "]" .
//	TypeAssertion  = "." "(" Type ")" .
//	Arguments      = "(" [ ( ExpressionList | Type [ "," ExpressionList ] ) [ "..." ] [ "," ] ] ")" .
func (p *parser) pexpr(x Expr, keep_parens bool) Expr {
	if trace {
		defer p.trace("pexpr")()
	}

	if x == nil {
		x = p.operand(keep_parens)
	}

loop:
	for {
		pos := p.pos()
		switch p.tok {
		case _Dot:
			p.next()
			switch p.tok {
			case _Name:
				// pexpr '.' sym
				t := new(SelectorExpr)
				t.pos = pos
				t.X = x
				t.Sel = p.name()
				x = t

			case _Lparen:
				p.next()
				if p.got(_Type) {
					t := new(TypeSwitchGuard)
					// t.Lhs is filled in by parser.simpleStmt
					t.pos = pos
					t.X = x
					x = t
				} else {
					t := new(AssertExpr)
					t.pos = pos
					t.X = x
					t.Type = p.type_()
					x = t
				}
				p.want(_Rparen)

			default:
				p.syntaxError("expected name or (")
				p.advance(_Semi, _Rparen)
			}

		case _Lbrack:
			p.next()

			var i Expr
			if p.tok != _Colon {
				var comma bool
				if p.tok == _Rbrack {
					// invalid empty instance, slice or index expression; accept but complain
					p.syntaxError("expected operand")
					i = p.badExpr()
				} else {
					i, comma = p.typeList(false)
				}
				if comma || p.tok == _Rbrack {
					p.want(_Rbrack)
					// x[], x[i,] or x[i, j, ...]
					t := new(IndexExpr)
					t.pos = pos
					t.X = x
					t.Index = i
					x = t
					break
				}
			}

			// x[i:...
			// For better error message, don't simply use p.want(_Colon) here (go.dev/issue/47704).
			if !p.got(_Colon) {
				p.syntaxError("expected comma, : or ]")
				p.advance(_Comma, _Colon, _Rbrack)
			}
			p.xnest++
			t := new(SliceExpr)
			t.pos = pos
			t.X = x
			t.Index[0] = i
			if p.tok != _Colon && p.tok != _Rbrack {
				// x[i:j...
				t.Index[1] = p.expr()
			}
			if p.tok == _Colon {
				t.Full = true
				// x[i:j:...]
				if t.Index[1] == nil {
					p.error("middle index required in 3-index slice")
					t.Index[1] = p.badExpr()
				}
				p.next()
				if p.tok != _Rbrack {
					// x[i:j:k...
					t.Index[2] = p.expr()
				} else {
					p.error("final index required in 3-index slice")
					t.Index[2] = p.badExpr()
				}
			}
			p.xnest--
			p.want(_Rbrack)
			x = t

		case _Lparen:
			t := new(CallExpr)
			t.pos = pos
			p.next()
			t.Fun = x
			t.ArgList, t.HasDots = p.argList()
			x = t

		case _Lbrace:
			// operand may have returned a parenthesized complit
			// type; accept it but complain if we have a complit
			t := Unparen(x)
			// determine if '{' belongs to a composite literal or a block statement
			complit_ok := false
			switch t.(type) {
			case *Name, *SelectorExpr:
				if p.xnest >= 0 {
					// x is possibly a composite literal type
					complit_ok = true
				}
			case *IndexExpr:
				if p.xnest >= 0 && !isValue(t) {
					// x is possibly a composite literal type
					complit_ok = true
				}
			case *ArrayType, *SliceType, *StructType, *MapType:
				// x is a comptype
				complit_ok = true
			}
			if !complit_ok {
				break loop
			}
			if t != x {
				p.syntaxError("cannot parenthesize type in composite literal")
				// already progressed, no need to advance
			}
			n := p.complitexpr()
			n.Type = x
			x = n

		default:
			break loop
		}
	}

	return x
}

// isValue reports whether x syntactically must be a value (and not a type) expression.
func isValue(x Expr) bool {
	switch x := x.(type) {
	case *BasicLit, *CompositeLit, *FuncLit, *SliceExpr, *AssertExpr, *TypeSwitchGuard, *CallExpr:
		return true
	case *Operation:
		return x.Op != Mul || x.Y != nil // *T may be a type
	case *ParenExpr:
		return isValue(x.X)
	case *IndexExpr:
		return isValue(x.X) || isValue(x.Index)
	}
	return false
}

// Element = Expression | LiteralValue .
func (p *parser) bare_complitexpr() Expr {
	if trace {
		defer p.trace("bare_complitexpr")()
	}

	if p.tok == _Lbrace {
		// '{' start_complit braced_keyval_list '}'
		return p.complitexpr()
	}

	return p.expr()
}

// LiteralValue = "{" [ ElementList [ "," ] ] "}" .
func (p *parser) complitexpr() *CompositeLit {
	if trace {
		defer p.trace("complitexpr")()
	}

	x := new(CompositeLit)
	x.pos = p.pos()

	p.xnest++
	p.want(_Lbrace)
	x.Rbrace = p.list("composite literal", _Comma, _Rbrace, func() bool {
		// value
		e := p.bare_complitexpr()
		if p.tok == _Colon {
			// key ':' value
			l := new(KeyValueExpr)
			l.pos = p.pos()
			p.next()
			l.Key = e
			l.Value = p.bare_complitexpr()
			e = l
			x.NKeys++
		}
		x.ElemList = append(x.ElemList, e)
		return false
	})
	p.xnest--

	return x
}

// ----------------------------------------------------------------------------
// Types

func (p *parser) type_() Expr {
	if trace {
		defer p.trace("type_")()
	}

	typ := p.typeOrNil()
	if typ == nil {
		typ = p.badExpr()
		p.syntaxError("expected type")
		p.advance(_Comma, _Colon, _Semi, _Rparen, _Rbrack, _Rbrace)
	}

	return typ
}

func newIndirect(pos Pos, typ Expr) Expr {
	o := new(Operation)
	o.pos = pos
	o.Op = Mul
	o.X = typ
	return o
}

// typeOrNil is like type_ but it returns nil if there was no type
// instead of reporting an error.
//
//	Type     = TypeName | TypeLit | "(" Type ")" .
//	TypeName = identifier | QualifiedIdent .
//	TypeLit  = ArrayType | StructType | PointerType | FunctionType | InterfaceType |
//		      SliceType | MapType | Channel_Type .
func (p *parser) typeOrNil() Expr {
	if trace {
		defer p.trace("typeOrNil")()
	}

	pos := p.pos()
	switch p.tok {
	case _Star:
		// ptrtype
		p.next()
		return newIndirect(pos, p.type_())

	case _Arrow:
		// recvchantype
		p.next()
		p.want(_Chan)
		t := new(ChanType)
		t.pos = pos
		t.Dir = RecvOnly
		t.Elem = p.chanElem()
		return t

	case _Func:
		// fntype
		p.next()
		_, t := p.funcType("function type")
		return t

	case _Lbrack:
		// '[' oexpr ']' ntype
		// '[' _DotDotDot ']' ntype
		p.next()
		if p.got(_Rbrack) {
			return p.sliceType(pos)
		}
		return p.arrayType(pos, nil)

	case _Chan:
		// _Chan non_recvchantype
		// _Chan _Comm ntype
		p.next()
		t := new(ChanType)
		t.pos = pos
		if p.got(_Arrow) {
			t.Dir = SendOnly
		}
		t.Elem = p.chanElem()
		return t

	case _Map:
		// _Map '[' ntype ']' ntype
		p.next()
		p.want(_Lbrack)
		t := new(MapType)
		t.pos = pos
		t.Key = p.type_()
		p.want(_Rbrack)
		t.Value = p.type_()
		return t

	case _Struct:
		return p.structType()

	case _Interface:
		return p.interfaceType()

	case _Name:
		return p.qualifiedName(nil)

	case _Lparen:
		p.next()
		t := p.type_()
		p.want(_Rparen)
		// The parser doesn't keep unnecessary parentheses.
		// Set the flag below to keep them, for testing
		// (see e.g. tests for go.dev/issue/68639).
		const keep_parens = false
		if keep_parens {
			px := new(ParenExpr)
			px.pos = pos
			px.X = t
			t = px
		}
		return t
	}

	return nil
}

func (p *parser) typeInstance(typ Expr) Expr {
	if trace {
		defer p.trace("typeInstance")()
	}

	pos := p.pos()
	p.want(_Lbrack)
	x := new(IndexExpr)
	x.pos = pos
	x.X = typ
	if p.tok == _Rbrack {
		p.syntaxError("expected type argument list")
		x.Index = p.badExpr()
	} else {
		x.Index, _ = p.typeList(true)
	}
	p.want(_Rbrack)
	return x
}

// If context != "", type parameters are not permitted.
func (p *parser) funcType(context string) ([]*Field, *FuncType) {
	if trace {
		defer p.trace("funcType")()
	}

	typ := new(FuncType)
	typ.pos = p.pos()

	var tparamList []*Field
	if p.got(_Lbrack) {
		if context != "" {
			// accept but complain
			p.syntaxErrorAt(typ.pos, context+" must have no type parameters")
		}
		if p.tok == _Rbrack {
			p.syntaxError("empty type parameter list")
			p.next()
		} else {
			tparamList = p.paramList(nil, nil, _Rbrack, true, false)
		}
	}

	p.want(_Lparen)
	typ.ParamList = p.paramList(nil, nil, _Rparen, false, true)
	typ.ResultList = p.funcResult()

	return tparamList, typ
}

// "[" has already been consumed, and pos is its position.
// If len != nil it is the already consumed array length.
func (p *parser) arrayType(pos Pos, len Expr) Expr {
	if trace {
		defer p.trace("arrayType")()
	}

	if len == nil && !p.got(_DotDotDot) {
		p.xnest++
		len = p.expr()
		p.xnest--
	}
	if p.tok == _Comma {
		// Trailing commas are accepted in type parameter
		// lists but not in array type declarations.
		// Accept for better error handling but complain.
		p.syntaxError("unexpected comma; expected ]")
		p.next()
	}
	p.want(_Rbrack)
	t := new(ArrayType)
	t.pos = pos
	t.Len = len
	t.Elem = p.type_()
	return t
}

// "[" and "]" have already been consumed, and pos is the position of "[".
func (p *parser) sliceType(pos Pos) Expr {
	t := new(SliceType)
	t.pos = pos
	t.Elem = p.type_()
	return t
}

func (p *parser) chanElem() Expr {
	if trace {
		defer p.trace("chanElem")()
	}

	typ := p.typeOrNil()
	if typ == nil {
		typ = p.badExpr()
		p.syntaxError("missing channel element type")
		// assume element type is simply absent - don't advance
	}

	return typ
}

// StructType = "struct" "{" { FieldDecl ";" } "}" .
func (p *parser) structType() *StructType {
	if trace {
		defer p.trace("structType")()
	}

	typ := new(StructType)
	typ.pos = p.pos()

	p.want(_Struct)
	p.want(_Lbrace)
	p.list("struct type", _Semi, _Rbrace, func() bool {
		p.fieldDecl(typ)
		return false
	})

	return typ
}

// InterfaceType = "interface" "{" { ( MethodDecl | EmbeddedElem ) ";" } "}" .
func (p *parser) interfaceType() *InterfaceType {
	if trace {
		defer p.trace("interfaceType")()
	}

	typ := new(InterfaceType)
	typ.pos = p.pos()

	p.want(_Interface)
	p.want(_Lbrace)
	p.list("interface type", _Semi, _Rbrace, func() bool {
		var f *Field
		if p.tok == _Name {
			f = p.methodDecl()
		}
		if f == nil || f.Name == nil {
			f = p.embeddedElem(f)
		}
		typ.MethodList = append(typ.MethodList, f)
		return false
	})

	return typ
}

// Result = Parameters | Type .
func (p *parser) funcResult() []*Field {
	if trace {
		defer p.trace("funcResult")()
	}

	if p.got(_Lparen) {
		return p.paramList(nil, nil, _Rparen, false, false)
	}

	pos := p.pos()
	if typ := p.typeOrNil(); typ != nil {
		f := new(Field)
		f.pos = pos
		f.Type = typ
		return []*Field{f}
	}

	return nil
}

func (p *parser) addField(styp *StructType, pos Pos, name *Name, typ Expr, tag *BasicLit) {
	if tag != nil {
		for i := len(styp.FieldList) - len(styp.TagList); i > 0; i-- {
			styp.TagList = append(styp.TagList, nil)
		}
		styp.TagList = append(styp.TagList, tag)
	}

	f := new(Field)
	f.pos = pos
	f.Name = name
	f.Type = typ
	styp.FieldList = append(styp.FieldList, f)

	if debug && tag != nil && len(styp.FieldList) != len(styp.TagList) {
		panic("inconsistent struct field list")
	}
}

// FieldDecl      = (IdentifierList Type | AnonymousField) [ Tag ] .
// AnonymousField = [ "*" ] TypeName .
// Tag            = string_lit .
func (p *parser) fieldDecl(styp *StructType) {
	if trace {
		defer p.trace("fieldDecl")()
	}

	pos := p.pos()
	switch p.tok {
	case _Name:
		name := p.name()
		if p.tok == _Dot || p.tok == _Literal || p.tok == _Semi || p.tok == _Rbrace {
			// embedded type
			typ := p.qualifiedName(name)
			tag := p.oliteral()
			p.addField(styp, pos, nil, typ, tag)
			break
		}

		// name1, name2, ... Type [ tag ]
		names := p.nameList(name)
		var typ Expr

		// Careful dance: We don't know if we have an embedded instantiated
		// type T[P1, P2, ...] or a field T of array/slice type [P]E or []E.
		if len(names) == 1 && p.tok == _Lbrack {
			typ = p.arrayOrTArgs()
			if typ, ok := typ.(*IndexExpr); ok {
				// embedded type T[P1, P2, ...]
				typ.X = name // name == names[0]
				tag := p.oliteral()
				p.addField(styp, pos, nil, typ, tag)
				break
			}
		} else {
			// T P
			typ = p.type_()
		}

		tag := p.oliteral()

		for _, name := range names {
			p.addField(styp, name.Pos(), name, typ, tag)
		}

	case _Star:
		p.next()
		var typ Expr
		if p.tok == _Lparen {
			// *(T)
			p.syntaxError("cannot parenthesize embedded type")
			p.next()
			typ = p.qualifiedName(nil)
			p.got(_Rparen) // no need to complain if missing
		} else {
			// *T
			typ = p.qualifiedName(nil)
		}
		tag := p.oliteral()
		p.addField(styp, pos, nil, newIndirect(pos, typ), tag)

	case _Lparen:
		p.syntaxError("cannot parenthesize embedded type")
		p.next()
		var typ Expr
		if p.tok == _Star {
			// (*T)
			pos := p.pos()
			p.next()
			typ = newIndirect(pos, p.qualifiedName(nil))
		} else {
			// (T)
			typ = p.qualifiedName(nil)
		}
		p.got(_Rparen) // no need to complain if missing
		tag := p.oliteral()
		p.addField(styp, pos, nil, typ, tag)

	default:
		p.syntaxError("expected field name or embedded type")
		p.advance(_Semi, _Rbrace)
	}
}

func (p *parser) arrayOrTArgs() Expr {
	if trace {
		defer p.trace("arrayOrTArgs")()
	}

	pos := p.pos()
	p.want(_Lbrack)
	if p.got(_Rbrack) {
		return p.sliceType(pos)
	}

	// x [n]E or x[n,], x[n1, n2], ...
	n, comma := p.typeList(false)
	p.want(_Rbrack)
	if !comma {
		if elem := p.typeOrNil(); elem != nil {
			// x [n]E
			t := new(ArrayType)
			t.pos = pos
			t.Len = n
			t.Elem = elem
			return t
		}
	}

	// x[n,], x[n1, n2], ...
	t := new(IndexExpr)
	t.pos = pos
	// t.X will be filled in by caller
	t.Index = n
	return t
}

func (p *parser) oliteral() *BasicLit {
	if p.tok == _Literal {
		b := new(BasicLit)
		b.pos = p.pos()
		b.Value = p.lit
		b.Kind = p.kind
		b.Bad = p.bad
		p.next()
		return b
	}
	return nil
}

// MethodSpec        = MethodName Signature | InterfaceTypeName .
// MethodName        = identifier .
// InterfaceTypeName = TypeName .
func (p *parser) methodDecl() *Field {
	if trace {
		defer p.trace("methodDecl")()
	}

	f := new(Field)
	f.pos = p.pos()
	name := p.name()

	const context = "interface method"

	switch p.tok {
	case _Lparen:
		// method
		f.Name = name
		_, f.Type = p.funcType(context)

	case _Lbrack:
		// Careful dance: We don't know if we have a generic method m[T C](x T)
		// or an embedded instantiated type T[P1, P2] (we accept generic methods
		// for generality and robustness of parsing but complain with an error).
		pos := p.pos()
		p.next()

		// Empty type parameter or argument lists are not permitted.
		// Treat as if [] were absent.
		if p.tok == _Rbrack {
			// name[]
			pos := p.pos()
			p.next()
			if p.tok == _Lparen {
				// name[](
				p.errorAt(pos, "empty type parameter list")
				f.Name = name
				_, f.Type = p.funcType(context)
			} else {
				p.errorAt(pos, "empty type argument list")
				f.Type = name
			}
			break
		}

		// A type argument list looks like a parameter list with only
		// types. Parse a parameter list and decide afterwards.
		list := p.paramList(nil, nil, _Rbrack, false, false)
		if len(list) == 0 {
			// The type parameter list is not [] but we got nothing
			// due to other errors (reported by paramList). Treat
			// as if [] were absent.
			if p.tok == _Lparen {
				f.Name = name
				_, f.Type = p.funcType(context)
			} else {
				f.Type = name
			}
			break
		}

		// len(list) > 0
		if list[0].Name != nil {
			// generic method
			f.Name = name
			_, f.Type = p.funcType(context)
			p.errorAt(pos, "interface method must have no type parameters")
			break
		}

		// embedded instantiated type
		t := new(IndexExpr)
		t.pos = pos
		t.X = name
		if len(list) == 1 {
			t.Index = list[0].Type
		} else {
			// len(list) > 1
			l := new(ListExpr)
			l.pos = list[0].Pos()
			l.ElemList = make([]Expr, len(list))
			for i := range list {
				l.ElemList[i] = list[i].Type
			}
			t.Index = l
		}
		f.Type = t

	default:
		// embedded type
		f.Type = p.qualifiedName(name)
	}

	return f
}

// EmbeddedElem = MethodSpec | EmbeddedTerm { "|" EmbeddedTerm } .
func (p *parser) embeddedElem(f *Field) *Field {
	if trace {
		defer p.trace("embeddedElem")()
	}

	if f == nil {
		f = new(Field)
		f.pos = p.pos()
		f.Type = p.embeddedTerm()
	}

	for p.tok == _Operator && p.op == Or {
		t := new(Operation)
		t.pos = p.pos()
		t.Op = Or
		p.next()
		t.X = f.Type
		t.Y = p.embeddedTerm()
		f.Type = t
	}

	return f
}

// EmbeddedTerm = [ "~" ] Type .
func (p *parser) embeddedTerm() Expr {
	if trace {
		defer p.trace("embeddedTerm")()
	}

	if p.tok == _Operator && p.op == Tilde {
		t := new(Operation)
		t.pos = p.pos()
		t.Op = Tilde
		p.next()
		t.X = p.type_()
		return t
	}

	t := p.typeOrNil()
	if t == nil {
		t = p.badExpr()
		p.syntaxError("expected ~ term or type")
		p.advance(_Operator, _Semi, _Rparen, _Rbrack, _Rbrace)
	}

	return t
}

// ParameterDecl = [ IdentifierList ] [ "..." ] Type .
func (p *parser) paramDeclOrNil(name *Name, follow token) *Field {
	if trace {
		defer p.trace("paramDeclOrNil")()
	}

	// type set notation is ok in type parameter lists
	typeSetsOk := follow == _Rbrack

	pos := p.pos()
	if name != nil {
		pos = name.pos
	} else if typeSetsOk && p.tok == _Operator && p.op == Tilde {
		// "~" ...
		return p.embeddedElem(nil)
	}

	f := new(Field)
	f.pos = pos

	if p.tok == _Name || name != nil {
		// name
		if name == nil {
			name = p.name()
		}

		if p.tok == _Lbrack {
			// name "[" ...
			f.Type = p.arrayOrTArgs()
			if typ, ok := f.Type.(*IndexExpr); ok {
				// name "[" ... "]"
				typ.X = name
			} else {
				// name "[" n "]" E
				f.Name = name
			}
			if typeSetsOk && p.tok == _Operator && p.op == Or {
				// name "[" ... "]" "|" ...
				// name "[" n "]" E "|" ...
				f = p.embeddedElem(f)
			}
			return f
		}

		if p.tok == _Dot {
			// name "." ...
			f.Type = p.qualifiedName(name)
			if typeSetsOk && p.tok == _Operator && p.op == Or {
				// name "." name "|" ...
				f = p.embeddedElem(f)
			}
			return f
		}

		if typeSetsOk && p.tok == _Operator && p.op == Or {
			// name "|" ...
			f.Type = name
			return p.embeddedElem(f)
		}

		f.Name = name
	}

	if p.tok == _DotDotDot {
		// [name] "..." ...
		t := new(DotsType)
		t.pos = p.pos()
		p.next()
		t.Elem = p.typeOrNil()
		if t.Elem == nil {
			f.Type = p.badExpr()
			p.syntaxError("... is missing type")
		} else {
			f.Type = t
		}
		return f
	}

	if typeSetsOk && p.tok == _Operator && p.op == Tilde {
		// [name] "~" ...
		f.Type = p.embeddedElem(nil).Type
		return f
	}

	f.Type = p.typeOrNil()
	if typeSetsOk && p.tok == _Operator && p.op == Or && f.Type != nil {
		// [name] type "|"
		f = p.embeddedElem(f)
	}
	if f.Name != nil || f.Type != nil {
		return f
	}

	p.syntaxError("expected " + tokstring(follow))
	p.advance(_Comma, follow)
	return nil
}

// Parameters    = "(" [ ParameterList [ "," ] ] ")" .
// ParameterList = ParameterDecl { "," ParameterDecl } .
// "(" or "[" has already been consumed.
// If name != nil, it is the first name after "(" or "[".
// If typ != nil, name must be != nil, and (name, typ) is the first field in the list.
// In the result list, either all fields have a name, or no field has a name.
func (p *parser) paramList(name *Name, typ Expr, close token, requireNames, dddok bool) (list []*Field) {
	if trace {
		defer p.trace("paramList")()
	}

	// p.list won't invoke its function argument if we're at the end of the
	// parameter list. If we have a complete field, handle this case here.
	if name != nil && typ != nil && p.tok == close {
		p.next()
		par := new(Field)
		par.pos = name.pos
		par.Name = name
		par.Type = typ
		return []*Field{par}
	}

	var named int // number of parameters that have an explicit name and type
	var typed int // number of parameters that have an explicit type
	end := p.list("parameter list", _Comma, close, func() bool {
		var par *Field
		if typ != nil {
			if debug && name == nil {
				panic("initial type provided without name")
			}
			par = new(Field)
			par.pos = name.pos
			par.Name = name
			par.Type = typ
		} else {
			par = p.paramDeclOrNil(name, close)
		}
		name = nil // 1st name was consumed if present
		typ = nil  // 1st type was consumed if present
		if par != nil {
			if debug && par.Name == nil && par.Type == nil {
				panic("parameter without name or type")
			}
			if par.Name != nil && par.Type != nil {
				named++
			}
			if par.Type != nil {
				typed++
			}
			list = append(list, par)
		}
		return false
	})

	if len(list) == 0 {
		return
	}

	// distribute parameter types (len(list) > 0)
	if named == 0 && !requireNames {
		// all unnamed and we're not in a type parameter list => found names are named types
		for _, par := range list {
			if typ := par.Name; typ != nil {
				par.Type = typ
				par.Name = nil
			}
		}
	} else if named != len(list) {
		// some named or we're in a type parameter list => all must be named
		var errPos Pos // left-most error position (or unknown)
		var typ Expr   // current type (from right to left)
		for i := len(list) - 1; i >= 0; i-- {
			par := list[i]
			if par.Type != nil {
				typ = par.Type
				if par.Name == nil {
					errPos = StartPos(typ)
					par.Name = NewName(errPos, "_")
				}
			} else if typ != nil {
				par.Type = typ
			} else {
				// par.Type == nil && typ == nil => we only have a par.Name
				errPos = par.Name.Pos()
				t := p.badExpr()
				t.pos = errPos // correct position
				par.Type = t
			}
		}
		if errPos.IsKnown() {
			// Not all parameters are named because named != len(list).
			// If named == typed, there must be parameters that have no types.
			// They must be at the end of the parameter list, otherwise types
			// would have been filled in by the right-to-left sweep above and
			// there would be no error.
			// If requireNames is set, the parameter list is a type parameter
			// list.
			var msg string
			if named == typed {
				errPos = end // position error at closing token ) or ]
				if requireNames {
					msg = "missing type constraint"
				} else {
					msg = "missing parameter type"
				}
			} else {
				if requireNames {
					msg = "missing type parameter name"
					// go.dev/issue/60812
					if len(list) == 1 {
						msg += " or invalid array length"
					}
				} else {
					msg = "missing parameter name"
				}
			}
			p.syntaxErrorAt(errPos, msg)
		}
	}

	// check use of ...
	first := true // only report first occurrence
	for i, f := range list {
		if t, _ := f.Type.(*DotsType); t != nil && (!dddok || i+1 < len(list)) {
			if first {
				first = false
				if dddok {
					p.errorAt(t.pos, "can only use ... with final parameter")
				} else {
					p.errorAt(t.pos, "invalid use of ...")
				}
			}
			// use T instead of invalid ...T
			f.Type = t.Elem
		}
	}

	return
}

func (p *parser) badExpr() *BadExpr {
	b := new(BadExpr)
	b.pos = p.pos()
	return b
}

// ----------------------------------------------------------------------------
// Statements

// SimpleStmt = EmptyStmt | ExpressionStmt | SendStmt | IncDecStmt | Assignment | ShortVarDecl .
func (p *parser) simpleStmt(lhs Expr, keyword token) SimpleStmt {
	if trace {
		defer p.trace("simpleStmt")()
	}

	if keyword == _For && p.tok == _Range {
		// _Range expr
		if debug && lhs != nil {
			panic("invalid call of simpleStmt")
		}
		return p.newRangeClause(nil, false)
	}

	if lhs == nil {
		lhs = p.exprList()
	}

	if _, ok := lhs.(*ListExpr); !ok && p.tok != _Assign && p.tok != _Define {
		// expr
		pos := p.pos()
		switch p.tok {
		case _AssignOp:
			// lhs op= rhs
			op := p.op
			p.next()
			return p.newAssignStmt(pos, op, lhs, p.expr())

		case _IncOp:
			// lhs++ or lhs--
			op := p.op
			p.next()
			return p.newAssignStmt(pos, op, lhs, nil)

		case _Arrow:
			// lhs <- rhs
			s := new(SendStmt)
			s.pos = pos
			p.next()
			s.Chan = lhs
			s.Value = p.expr()
			return s

		default:
			// expr
			s := new(ExprStmt)
			s.pos = lhs.Pos()
			s.X = lhs
			return s
		}
	}

	// expr_list
	switch p.tok {
	case _Assign, _Define:
		pos := p.pos()
		var op Operator
		if p.tok == _Define {
			op = Def
		}
		p.next()

		if keyword == _For && p.tok == _Range {
			// expr_list op= _Range expr
			return p.newRangeClause(lhs, op == Def)
		}

		// expr_list op= expr_list
		rhs := p.exprList()

		if x, ok := rhs.(*TypeSwitchGuard); ok && keyword == _Switch && op == Def {
			if lhs, ok := lhs.(*Name); ok {
				// switch … lhs := rhs.(type)
				x.Lhs = lhs
				s := new(ExprStmt)
				s.pos = x.Pos()
				s.X = x
				return s
			}
		}

		return p.newAssignStmt(pos, op, lhs, rhs)

	default:
		p.syntaxError("expected := or = or comma")
		p.advance(_Semi, _Rbrace)
		// make the best of what we have
		if x, ok := lhs.(*ListExpr); ok {
			lhs = x.ElemList[0]
		}
		s := new(ExprStmt)
		s.pos = lhs.Pos()
		s.X = lhs
		return s
	}
}

func (p *parser) newRangeClause(lhs Expr, def bool) *RangeClause {
	r := new(RangeClause)
	r.pos = p.pos()
	p.next() // consume _Range
	r.Lhs = lhs
	r.Def = def
	r.X = p.expr()
	return r
}

func (p *parser) newAssignStmt(pos Pos, op Operator, lhs, rhs Expr) *AssignStmt {
	a := new(AssignStmt)
	a.pos = pos
	a.Op = op
	a.Lhs = lhs
	a.Rhs = rhs
	return a
}

func (p *parser) labeledStmtOrNil(label *Name) Stmt {
	if trace {
		defer p.trace("labeledStmt")()
	}

	s := new(LabeledStmt)
	s.pos = p.pos()
	s.Label = label

	p.want(_Colon)

	if p.tok == _Rbrace {
		// We expect a statement (incl. an empty statement), which must be
		// terminated by a semicolon. Because semicolons may be omitted before
		// an _Rbrace, seeing an _Rbrace implies an empty statement.
		e := new(EmptyStmt)
		e.pos = p.pos()
		s.Stmt = e
		return s
	}

	s.Stmt = p.stmtOrNil()
	if s.Stmt != nil {
		return s
	}

	// report error at line of ':' token
	p.syntaxErrorAt(s.pos, "missing statement after label")
	// we are already at the end of the labeled statement - no need to advance
	return nil // avoids follow-on errors (see e.g., fixedbugs/bug274.go)
}

// context must be a non-empty string unless we know that p.tok == _Lbrace.
func (p *parser) blockStmt(context string) *BlockStmt {
	if trace {
		defer p.trace("blockStmt")()
	}

	s := new(BlockStmt)
	s.pos = p.pos()

	// people coming from C may forget that braces are mandatory in Go
	if !p.got(_Lbrace) {
		p.syntaxError("expected { after " + context)
		p.advance(_Name, _Rbrace)
		s.Rbrace = p.pos() // in case we found "}"
		if p.got(_Rbrace) {
			return s
		}
	}

	s.List = p.stmtList()
	s.Rbrace = p.pos()
	p.want(_Rbrace)

	return s
}

func (p *parser) declStmt(f func(*Group) Decl) *DeclStmt {
	if trace {
		defer p.trace("declStmt")()
	}

	s := new(DeclStmt)
	s.pos = p.pos()

	p.next() // _Const, _Type, or _Var
	s.DeclList = p.appendGroup(nil, f)

	return s
}

func (p *parser) forStmt() Stmt {
	if trace {
		defer p.trace("forStmt")()
	}

	s := new(ForStmt)
	s.pos = p.pos()

	s.Init, s.Cond, s.Post = p.header(_For)
	s.Body = p.blockStmt("for clause")

	return s
}

func (p *parser) header(keyword token) (init SimpleStmt, cond Expr, post SimpleStmt) {
	p.want(keyword)

	if p.tok == _Lbrace {
		if keyword == _If {
			p.syntaxError("missing condition in if statement")
			cond = p.badExpr()
		}
		return
	}
	// p.tok != _Lbrace

	outer := p.xnest
	p.xnest = -1

	if p.tok != _Semi {
		// accept potential varDecl but complain
		if p.got(_Var) {
			p.syntaxError(fmt.Sprintf("var declaration not allowed in %s initializer", keyword.String()))
		}
		init = p.simpleStmt(nil, keyword)
		// If we have a range clause, we are done (can only happen for keyword == _For).
		if _, ok := init.(*RangeClause); ok {
			p.xnest = outer
			return
		}
	}

	var condStmt SimpleStmt
	var semi struct {
		pos Pos
		lit string // valid if pos.IsKnown()
	}
	if p.tok != _Lbrace {
		if p.tok == _Semi {
			semi.pos = p.pos()
			semi.lit = p.lit
			p.next()
		} else {
			// asking for a '{' rather than a ';' here leads to a better error message
			p.want(_Lbrace)
			if p.tok != _Lbrace {
				p.advance(_Lbrace, _Rbrace) // for better synchronization (e.g., go.dev/issue/22581)
			}
		}
		if keyword == _For {
			if p.tok != _Semi {
				if p.tok == _Lbrace {
					p.syntaxError("expected for loop condition")
					goto done
				}
				condStmt = p.simpleStmt(nil, 0 /* range not permitted */)
			}
			p.want(_Semi)
			if p.tok != _Lbrace {
				post = p.simpleStmt(nil, 0 /* range not permitted */)
				if a, _ := post.(*AssignStmt); a != nil && a.Op == Def {
					p.syntaxErrorAt(a.Pos(), "cannot declare in post statement of for loop")
				}
			}
		} else if p.tok != _Lbrace {
			condStmt = p.simpleStmt(nil, keyword)
		}
	} else {
		condStmt = init
		init = nil
	}

done:
	// unpack condStmt
	switch s := condStmt.(type) {
	case nil:
		if keyword == _If && semi.pos.IsKnown() {
			if semi.lit != "semicolon" {
				p.syntaxErrorAt(semi.pos, fmt.Sprintf("unexpected %s, expected { after if clause", semi.lit))
			} else {
				p.syntaxErrorAt(semi.pos, "missing condition in if statement")
			}
			b := new(BadExpr)
			b.pos = semi.pos
			cond = b
		}
	case *ExprStmt:
		cond = s.X
	default:
		// A common syntax error is to write '=' instead of '==',
		// which turns an expression into an assignment. Provide
		// a more explicit error message in that case to prevent
		// further confusion.
		var str string
		if as, ok := s.(*AssignStmt); ok && as.Op == 0 {
			// Emphasize complex Lhs and Rhs of assignment with parentheses to highlight '='.
			str = "assignment " + emphasize(as.Lhs) + " = " + emphasize(as.Rhs)
		} else {
			str = String(s)
		}
		p.syntaxErrorAt(s.Pos(), fmt.Sprintf("cannot use %s as value", str))
	}

	p.xnest = outer
	return
}

// emphasize returns a string representation of x, with (top-level)
// binary expressions emphasized by enclosing them in parentheses.
func emphasize(x Expr) string {
	s := String(x)
	if op, _ := x.(*Operation); op != nil && op.Y != nil {
		// binary expression
		return "(" + s + ")"
	}
	return s
}

func (p *parser) ifStmt() *IfStmt {
	if trace {
		defer p.trace("ifStmt")()
	}

	s := new(IfStmt)
	s.pos = p.pos()

	s.Init, s.Cond, _ = p.header(_If)
	s.Then = p.blockStmt("if clause")

	if p.got(_Else) {
		switch p.tok {
		case _If:
			s.Else = p.ifStmt()
		case _Lbrace:
			s.Else = p.blockStmt("")
		default:
			p.syntaxError("else must be followed by if or statement block")
			p.advance(_Name, _Rbrace)
		}
	}

	return s
}

func (p *parser) switchStmt() *SwitchStmt {
	if trace {
		defer p.trace("switchStmt")()
	}

	s := new(SwitchStmt)
	s.pos = p.pos()

	s.Init, s.Tag, _ = p.header(_Switch)

	if !p.got(_Lbrace) {
		p.syntaxError("missing { after switch clause")
		p.advance(_Case, _Default, _Rbrace)
	}
	for p.tok != _EOF && p.tok != _Rbrace {
		s.Body = append(s.Body, p.caseClause())
	}
	s.Rbrace = p.pos()
	p.want(_Rbrace)

	return s
}

func (p *parser) selectStmt() *SelectStmt {
	if trace {
		defer p.trace("selectStmt")()
	}

	s := new(SelectStmt)
	s.pos = p.pos()

	p.want(_Select)
	if !p.got(_Lbrace) {
		p.syntaxError("missing { after select clause")
		p.advance(_Case, _Default, _Rbrace)
	}
	for p.tok != _EOF && p.tok != _Rbrace {
		s.Body = append(s.Body, p.commClause())
	}
	s.Rbrace = p.pos()
	p.want(_Rbrace)

	return s
}

func (p *parser) caseClause() *CaseClause {
	if trace {
		defer p.trace("caseClause")()
	}

	c := new(CaseClause)
	c.pos = p.pos()

	switch p.tok {
	case _Case:
		p.next()
		c.Cases = p.exprList()

	case _Default:
		p.next()

	default:
		p.syntaxError("expected case or default or }")
		p.advance(_Colon, _Case, _Default, _Rbrace)
	}

	c.Colon = p.pos()
	p.want(_Colon)
	c.Body = p.stmtList()

	return c
}

func (p *parser) commClause() *CommClause {
	if trace {
		defer p.trace("commClause")()
	}

	c := new(CommClause)
	c.pos = p.pos()

	switch p.tok {
	case _Case:
		p.next()
		c.Comm = p.simpleStmt(nil, 0)

		// The syntax restricts the possible simple statements here to:
		//
		//     lhs <- x (send statement)
		//     <-x
		//     lhs = <-x
		//     lhs := <-x
		//
		// All these (and more) are recognized by simpleStmt and invalid
		// syntax trees are flagged later, during type checking.

	case _Default:
		p.next()

	default:
		p.syntaxError("expected case or default or }")
		p.advance(_Colon, _Case, _Default, _Rbrace)
	}

	c.Colon = p.pos()
	p.want(_Colon)
	c.Body = p.stmtList()

	return c
}

// stmtOrNil parses a statement if one is present, or else returns nil.
//
//	Statement =
//		Declaration | LabeledStmt | SimpleStmt |
//		GoStmt | ReturnStmt | BreakStmt | ContinueStmt | GotoStmt |
//		FallthroughStmt | Block | IfStmt | SwitchStmt | SelectStmt | ForStmt |
//		DeferStmt .
func (p *parser) stmtOrNil() Stmt {
	if trace {
		defer p.trace("stmt " + p.tok.String())()
	}

	// Most statements (assignments) start with an identifier;
	// look for it first before doing anything more expensive.
	if p.tok == _Name {
		p.clearPragma()
		lhs := p.exprList()
		if label, ok := lhs.(*Name); ok && p.tok == _Colon {
			return p.labeledStmtOrNil(label)
		}
		return p.simpleStmt(lhs, 0)
	}

	switch p.tok {
	case _Var:
		return p.declStmt(p.varDecl)

	case _Const:
		return p.declStmt(p.constDecl)

	case _Type:
		return p.declStmt(p.typeDecl)
	}

	p.clearPragma()

	switch p.tok {
	case _Lbrace:
		return p.blockStmt("")

	case _Operator, _Star:
		switch p.op {
		case Add, Sub, Mul, And, Xor, Not:
			return p.simpleStmt(nil, 0) // unary operators
		}

	case _Literal, _Func, _Lparen, // operands
		_Lbrack, _Struct, _Map, _Chan, _Interface, // composite types
		_Arrow: // receive operator
		return p.simpleStmt(nil, 0)

	case _For:
		return p.forStmt()

	case _Switch:
		return p.switchStmt()

	case _Select:
		return p.selectStmt()

	case _If:
		return p.ifStmt()

	case _Fallthrough:
		s := new(BranchStmt)
		s.pos = p.pos()
		p.next()
		s.Tok = _Fallthrough
		return s

	case _Break, _Continue:
		s := new(BranchStmt)
		s.pos = p.pos()
		s.Tok = p.tok
		p.next()
		if p.tok == _Name {
			s.Label = p.name()
		}
		return s

	case _Go, _Defer:
		return p.callStmt()

	case _Goto:
		s := new(BranchStmt)
		s.pos = p.pos()
		s.Tok = _Goto
		p.next()
		s.Label = p.name()
		return s

	case _Return:
		s := new(ReturnStmt)
		s.pos = p.pos()
		p.next()
		if p.tok != _Semi && p.tok != _Rbrace {
			s.Results = p.exprList()
		}
		return s

	case _Semi:
		s := new(EmptyStmt)
		s.pos = p.pos()
		return s
	}

	return nil
}

// StatementList = { Statement ";" } .
func (p *parser) stmtList() (l []Stmt) {
	if trace {
		defer p.trace("stmtList")()
	}

	for p.tok != _EOF && p.tok != _Rbrace && p.tok != _Case && p.tok != _Default {
		s := p.stmtOrNil()
		p.clearPragma()
		if s == nil {
			break
		}
		l = append(l, s)
		// ";" is optional before "}"
		if !p.got(_Semi) && p.tok != _Rbrace {
			p.syntaxError("at end of statement")
			p.advance(_Semi, _Rbrace, _Case, _Default)
			p.got(_Semi) // avoid spurious empty statement
		}
	}
	return
}

// argList parses a possibly empty, comma-separated list of arguments,
// optionally followed by a comma (if not empty), and closed by ")".
// The last argument may be followed by "...".
//
// argList = [ arg { "," arg } [ "..." ] [ "," ] ] ")" .
func (p *parser) argList() (list []Expr, hasDots bool) {
	if trace {
		defer p.trace("argList")()
	}

	p.xnest++
	p.list("argument list", _Comma, _Rparen, func() bool {
		list = append(list, p.expr())
		hasDots = p.got(_DotDotDot)
		return hasDots
	})
	p.xnest--

	return
}

// ----------------------------------------------------------------------------
// Common productions

func (p *parser) name() *Name {
	// no tracing to avoid overly verbose output

	if p.tok == _Name {
		n := NewName(p.pos(), p.lit)
		p.next()
		return n
	}

	n := NewName(p.pos(), "_")
	p.syntaxError("expected name")
	p.advance()
	return n
}

// IdentifierList = identifier { "," identifier } .
// The first name must be provided.
func (p *parser) nameList(first *Name) []*Name {
	if trace {
		defer p.trace("nameList")()
	}

	if debug && first == nil {
		panic("first name not provided")
	}

	l := []*Name{first}
	for p.got(_Comma) {
		l = append(l, p.name())
	}

	return l
}

// The first name may be provided, or nil.
func (p *parser) qualifiedName(name *Name) Expr {
	if trace {
		defer p.trace("qualifiedName")()
	}

	var x Expr
	switch {
	case name != nil:
		x = name
	case p.tok == _Name:
		x = p.name()
	default:
		x = NewName(p.pos(), "_")
		p.syntaxError("expected name")
		p.advance(_Dot, _Semi, _Rbrace)
	}

	if p.tok == _Dot {
		s := new(SelectorExpr)
		s.pos = p.pos()
		p.next()
		s.X = x
		s.Sel = p.name()
		x = s
	}

	if p.tok == _Lbrack {
		x = p.typeInstance(x)
	}

	return x
}

// ExpressionList = Expression { "," Expression } .
func (p *parser) exprList() Expr {
	if trace {
		defer p.trace("exprList")()
	}

	x := p.expr()
	if p.got(_Comma) {
		list := []Expr{x, p.expr()}
		for p.got(_Comma) {
			list = append(list, p.expr())
		}
		t := new(ListExpr)
		t.pos = x.Pos()
		t.ElemList = list
		x = t
	}
	return x
}

// typeList parses a non-empty, comma-separated list of types,
// optionally followed by a comma. If strict is set to false,
// the first element may also be a (non-type) expression.
// If there is more than one argument, the result is a *ListExpr.
// The comma result indicates whether there was a (separating or
// trailing) comma.
//
// typeList = arg { "," arg } [ "," ] .
func (p *parser) typeList(strict bool) (x Expr, comma bool) {
	if trace {
		defer p.trace("typeList")()
	}

	p.xnest++
	if strict {
		x = p.type_()
	} else {
		x = p.expr()
	}
	if p.got(_Comma) {
		comma = true
		if t := p.typeOrNil(); t != nil {
			list := []Expr{x, t}
			for p.got(_Comma) {
				if t = p.typeOrNil(); t == nil {
					break
				}
				list = append(list, t)
			}
			l := new(ListExpr)
			l.pos = x.Pos() // == list[0].Pos()
			l.ElemList = list
			x = l
		}
	}
	p.xnest--
	return
}

// Unparen returns e with any enclosing parentheses stripped.
func Unparen(x Expr) Expr {
	for {
		p, ok := x.(*ParenExpr)
		if !ok {
			break
		}
		x = p.X
	}
	return x
}

// UnpackListExpr unpacks a *ListExpr into a []Expr.
func UnpackListExpr(x Expr) []Expr {
	switch x := x.(type) {
	case nil:
		return nil
	case *ListExpr:
		return x.ElemList
	default:
		return []Expr{x}
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/pos.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import "fmt"

// PosMax is the largest line or column value that can be represented without loss.
// Incoming values (arguments) larger than PosMax will be set to PosMax.
//
// Keep this consistent with maxLineCol in go/scanner.
const PosMax = 1 << 30

// A Pos represents an absolute (line, col) source position
// with a reference to position base for computing relative
// (to a file, or line directive) position information.
// Pos values are intentionally light-weight so that they
// can be created without too much concern about space use.
type Pos struct {
	base      *PosBase
	line, col uint32
}

// MakePos returns a new Pos for the given PosBase, line and column.
func MakePos(base *PosBase, line, col uint) Pos { return Pos{base, sat32(line), sat32(col)} }

// TODO(gri) IsKnown makes an assumption about linebase < 1.
// Maybe we should check for Base() != nil instead.

func (pos Pos) Pos() Pos       { return pos }
func (pos Pos) IsKnown() bool  { return pos.line > 0 }
func (pos Pos) Base() *PosBase { return pos.base }
func (pos Pos) Line() uint     { return uint(pos.line) }
func (pos Pos) Col() uint      { return uint(pos.col) }

// FileBase returns the PosBase of the file containing pos,
// skipping over intermediate PosBases from //line directives.
// The result is nil if pos doesn't have a file base.
func (pos Pos) FileBase() *PosBase {
	b := pos.base
	for b != nil && b != b.pos.base {
		b = b.pos.base
	}
	// b == nil || b == b.pos.base
	return b
}

func (pos Pos) RelFilename() string { return pos.base.Filename() }

func (pos Pos) RelLine() uint {
	b := pos.base
	if b.Line() == 0 {
		// base line is unknown => relative line is unknown
		return 0
	}
	return b.Line() + (pos.Line() - b.Pos().Line())
}

func (pos Pos) RelCol() uint {
	b := pos.base
	if b.Col() == 0 {
		// base column is unknown => relative column is unknown
		// (the current specification for line directives requires
		// this to apply until the next PosBase/line directive,
		// not just until the new newline)
		return 0
	}
	if pos.Line() == b.Pos().Line() {
		// pos on same line as pos base => column is relative to pos base
		return b.Col() + (pos.Col() - b.Pos().Col())
	}
	return pos.Col()
}

// Cmp compares the positions p and q and returns a result r as follows:
//
//	r <  0: p is before q
//	r == 0: p and q are the same position (but may not be identical)
//	r >  0: p is after q
//
// If p and q are in different files, p is before q if the filename
// of p sorts lexicographically before the filename of q.
func (p Pos) Cmp(q Pos) int {
	pname := p.RelFilename()
	qname := q.RelFilename()
	switch {
	case pname < qname:
		return -1
	case pname > qname:
		return +1
	}

	pline := p.Line()
	qline := q.Line()
	switch {
	case pline < qline:
		return -1
	case pline > qline:
		return +1
	}

	pcol := p.Col()
	qcol := q.Col()
	switch {
	case pcol < qcol:
		return -1
	case pcol > qcol:
		return +1
	}

	return 0
}

func (pos Pos) String() string {
	rel := position_{pos.RelFilename(), pos.RelLine(), pos.RelCol()}
	abs := position_{pos.Base().Pos().RelFilename(), pos.Line(), pos.Col()}
	s := rel.String()
	if rel != abs {
		s += "[" + abs.String() + "]"
	}
	return s
}

// TODO(gri) cleanup: find better name, avoid conflict with position in error_test.go
type position_ struct {
	filename  string
	line, col uint
}

func (p position_) String() string {
	if p.line == 0 {
		if p.filename == "" {
			return "<unknown position>"
		}
		return p.filename
	}
	if p.col == 0 {
		return fmt.Sprintf("%s:%d", p.filename, p.line)
	}
	return fmt.Sprintf("%s:%d:%d", p.filename, p.line, p.col)
}

// A PosBase represents the base for relative position information:
// At position pos, the relative position is filename:line:col.
type PosBase struct {
	pos       Pos
	filename  string
	line, col uint32
	trimmed   bool // whether -trimpath has been applied
}

// NewFileBase returns a new PosBase for the given filename.
// A file PosBase's position is relative to itself, with the
// position being filename:1:1.
func NewFileBase(filename string) *PosBase {
	return NewTrimmedFileBase(filename, false)
}

// NewTrimmedFileBase is like NewFileBase, but allows specifying Trimmed.
func NewTrimmedFileBase(filename string, trimmed bool) *PosBase {
	base := &PosBase{MakePos(nil, linebase, colbase), filename, linebase, colbase, trimmed}
	base.pos.base = base
	return base
}

// NewLineBase returns a new PosBase for a line directive "line filename:line:col"
// relative to pos, which is the position of the character immediately following
// the comment containing the line directive. For a directive in a line comment,
// that position is the beginning of the next line (i.e., the newline character
// belongs to the line comment).
func NewLineBase(pos Pos, filename string, trimmed bool, line, col uint) *PosBase {
	return &PosBase{pos, filename, sat32(line), sat32(col), trimmed}
}

func (base *PosBase) IsFileBase() bool {
	if base == nil {
		return false
	}
	return base.pos.base == base
}

func (base *PosBase) Pos() (_ Pos) {
	if base == nil {
		return
	}
	return base.pos
}

func (base *PosBase) Filename() string {
	if base == nil {
		return ""
	}
	return base.filename
}

func (base *PosBase) Line() uint {
	if base == nil {
		return 0
	}
	return uint(base.line)
}

func (base *PosBase) Col() uint {
	if base == nil {
		return 0
	}
	return uint(base.col)
}

func (base *PosBase) Trimmed() bool {
	if base == nil {
		return false
	}
	return base.trimmed
}

func sat32(x uint) uint32 {
	if x > PosMax {
		return PosMax
	}
	return uint32(x)
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/positions.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements helper functions for scope position computations.

package syntax

// StartPos returns the start position of n.
func StartPos(n Node) Pos {
	// Cases for nodes which don't need a correction are commented out.
	for m := n; ; {
		switch n := m.(type) {
		case nil:
			panic("nil node")

		// packages
		case *File:
			// file block starts at the beginning of the file
			return MakePos(n.Pos().Base(), 1, 1)

		// declarations
		// case *ImportDecl:
		// case *ConstDecl:
		// case *TypeDecl:
		// case *VarDecl:
		// case *FuncDecl:

		// expressions
		// case *BadExpr:
		// case *Name:
		// case *BasicLit:
		case *CompositeLit:
			if n.Type != nil {
				m = n.Type
				continue
			}
			return n.Pos()
		case *KeyValueExpr:
			m = n.Key
		// case *FuncLit:
		// case *ParenExpr:
		case *SelectorExpr:
			m = n.X
		case *IndexExpr:
			m = n.X
		// case *SliceExpr:
		case *AssertExpr:
			m = n.X
		case *TypeSwitchGuard:
			if n.Lhs != nil {
				m = n.Lhs
				continue
			}
			m = n.X
		case *Operation:
			if n.Y != nil {
				m = n.X
				continue
			}
			return n.Pos()
		case *CallExpr:
			m = n.Fun
		case *ListExpr:
			if len(n.ElemList) > 0 {
				m = n.ElemList[0]
				continue
			}
			return n.Pos()
		// types
		// case *ArrayType:
		// case *SliceType:
		// case *DotsType:
		// case *StructType:
		// case *Field:
		// case *InterfaceType:
		// case *FuncType:
		// case *MapType:
		// case *ChanType:

		// statements
		// case *EmptyStmt:
		// case *LabeledStmt:
		// case *BlockStmt:
		// case *ExprStmt:
		case *SendStmt:
			m = n.Chan
		// case *DeclStmt:
		case *AssignStmt:
			m = n.Lhs
		// case *BranchStmt:
		// case *CallStmt:
		// case *ReturnStmt:
		// case *IfStmt:
		// case *ForStmt:
		// case *SwitchStmt:
		// case *SelectStmt:

		// helper nodes
		case *RangeClause:
			if n.Lhs != nil {
				m = n.Lhs
				continue
			}
			m = n.X
		// case *CaseClause:
		// case *CommClause:

		default:
			return n.Pos()
		}
	}
}

// EndPos returns the approximate end position of n in the source.
// For some nodes (*Name, *BasicLit) it returns the position immediately
// following the node; for others (*BlockStmt, *SwitchStmt, etc.) it
// returns the position of the closing '}'; and for some (*ParenExpr)
// the returned position is the end position of the last enclosed
// expression.
// Thus, EndPos should not be used for exact demarcation of the
// end of a node in the source; it is mostly useful to determine
// scope ranges where there is some leeway.
func EndPos(n Node) Pos {
	for m := n; ; {
		switch n := m.(type) {
		case nil:
			panic("nil node")

		// packages
		case *File:
			return n.EOF

		// declarations
		case *ImportDecl:
			m = n.Path
		case *ConstDecl:
			if n.Values != nil {
				m = n.Values
				continue
			}
			if n.Type != nil {
				m = n.Type
				continue
			}
			if l := len(n.NameList); l > 0 {
				m = n.NameList[l-1]
				continue
			}
			return n.Pos()
		case *TypeDecl:
			m = n.Type
		case *VarDecl:
			if n.Values != nil {
				m = n.Values
				continue
			}
			if n.Type != nil {
				m = n.Type
				continue
			}
			if l := len(n.NameList); l > 0 {
				m = n.NameList[l-1]
				continue
			}
			return n.Pos()
		case *FuncDecl:
			if n.Body != nil {
				m = n.Body
				continue
			}
			m = n.Type

		// expressions
		case *BadExpr:
			return n.Pos()
		case *Name:
			p := n.Pos()
			return MakePos(p.Base(), p.Line(), p.Col()+uint(len(n.Value)))
		case *BasicLit:
			p := n.Pos()
			return MakePos(p.Base(), p.Line(), p.Col()+uint(len(n.Value)))
		case *CompositeLit:
			return n.Rbrace
		case *KeyValueExpr:
			m = n.Value
		case *FuncLit:
			m = n.Body
		case *ParenExpr:
			m = n.X
		case *SelectorExpr:
			m = n.Sel
		case *IndexExpr:
			m = n.Index
		case *SliceExpr:
			for i := len(n.Index) - 1; i >= 0; i-- {
				if x := n.Index[i]; x != nil {
					m = x
					continue
				}
			}
			m = n.X
		case *AssertExpr:
			m = n.Type
		case *TypeSwitchGuard:
			m = n.X
		case *Operation:
			if n.Y != nil {
				m = n.Y
				continue
			}
			m = n.X
		case *CallExpr:
			if l := lastExpr(n.ArgList); l != nil {
				m = l
				continue
			}
			m = n.Fun
		case *ListExpr:
			if l := lastExpr(n.ElemList); l != nil {
				m = l
				continue
			}
			return n.Pos()

		// types
		case *ArrayType:
			m = n.Elem
		case *SliceType:
			m = n.Elem
		case *DotsType:
			m = n.Elem
		case *StructType:
			if l := lastField(n.FieldList); l != nil {
				m = l
				continue
			}
			return n.Pos()
			// TODO(gri) need to take TagList into account
		case *Field:
			if n.Type != nil {
				m = n.Type
				continue
			}
			m = n.Name
		case *InterfaceType:
			if l := lastField(n.MethodList); l != nil {
				m = l
				continue
			}
			return n.Pos()
		case *FuncType:
			if l := lastField(n.ResultList); l != nil {
				m = l
				continue
			}
			if l := lastField(n.ParamList); l != nil {
				m = l
				continue
			}
			return n.Pos()
		case *MapType:
			m = n.Value
		case *ChanType:
			m = n.Elem

		// statements
		case *EmptyStmt:
			return n.Pos()
		case *LabeledStmt:
			m = n.Stmt
		case *BlockStmt:
			return n.Rbrace
		case *ExprStmt:
			m = n.X
		case *SendStmt:
			m = n.Value
		case *DeclStmt:
			if l := lastDecl(n.DeclList); l != nil {
				m = l
				continue
			}
			return n.Pos()
		case *AssignStmt:
			m = n.Rhs
			if m == nil {
				p := EndPos(n.Lhs)
				return MakePos(p.Base(), p.Line(), p.Col()+2)
			}
		case *BranchStmt:
			if n.Label != nil {
				m = n.Label
				continue
			}
			return n.Pos()
		case *CallStmt:
			m = n.Call
		case *ReturnStmt:
			if n.Results != nil {
				m = n.Results
				continue
			}
			return n.Pos()
		case *IfStmt:
			if n.Else != nil {
				m = n.Else
				continue
			}
			m = n.Then
		case *ForStmt:
			m = n.Body
		case *SwitchStmt:
			return n.Rbrace
		case *SelectStmt:
			return n.Rbrace

		// helper nodes
		case *RangeClause:
			m = n.X
		case *CaseClause:
			if l := lastStmt(n.Body); l != nil {
				m = l
				continue
			}
			return n.Colon
		case *CommClause:
			if l := lastStmt(n.Body); l != nil {
				m = l
				continue
			}
			return n.Colon

		default:
			return n.Pos()
		}
	}
}

func lastDecl(list []Decl) Decl {
	if l := len(list); l > 0 {
		return list[l-1]
	}
	return nil
}

func lastExpr(list []Expr) Expr {
	if l := len(list); l > 0 {
		return list[l-1]
	}
	return nil
}

func lastStmt(list []Stmt) Stmt {
	if l := len(list); l > 0 {
		return list[l-1]
	}
	return nil
}

func lastField(list []*Field) *Field {
	if l := len(list); l > 0 {
		return list[l-1]
	}
	return nil
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/printer.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements printing of syntax trees in source format.

package syntax

import (
	"fmt"
	"io"
	"strings"
)

// Form controls print formatting.
type Form uint

const (
	_         Form = iota // default
	LineForm              // use spaces instead of linebreaks where possible
	ShortForm             // like LineForm but print "…" for non-empty function or composite literal bodies
)

// Fprint prints node x to w in the specified form.
// It returns the number of bytes written, and whether there was an error.
func Fprint(w io.Writer, x Node, form Form) (n int, err error) {
	p := printer{
		output:     w,
		form:       form,
		linebreaks: form == 0,
	}

	defer func() {
		n = p.written
		if e := recover(); e != nil {
			err = e.(writeError).err // re-panics if it's not a writeError
		}
	}()

	p.print(x)
	p.flush(_EOF)

	return
}

// String is a convenience function that prints n in ShortForm
// and returns the printed string.
func String(n Node) string {
	var buf strings.Builder
	_, err := Fprint(&buf, n, ShortForm)
	if err != nil {
		fmt.Fprintf(&buf, "<<< ERROR: %s", err)
	}
	return buf.String()
}

type ctrlSymbol int

const (
	none ctrlSymbol = iota
	semi
	blank
	newline
	indent
	outdent
	// comment
	// eolComment
)

type whitespace struct {
	last token
	kind ctrlSymbol
	//text string // comment text (possibly ""); valid if kind == comment
}

type printer struct {
	output     io.Writer
	written    int // number of bytes written
	form       Form
	linebreaks bool // print linebreaks instead of semis

	indent  int // current indentation level
	nlcount int // number of consecutive newlines

	pending []whitespace // pending whitespace
	lastTok token        // last token (after any pending semi) processed by print
}

// write is a thin wrapper around p.output.Write
// that takes care of accounting and error handling.
func (p *printer) write(data []byte) {
	n, err := p.output.Write(data)
	p.written += n
	if err != nil {
		panic(writeError{err})
	}
}

var (
	tabBytes    = []byte("\t\t\t\t\t\t\t\t")
	newlineByte = []byte("\n")
	blankByte   = []byte(" ")
)

func (p *printer) writeBytes(data []byte) {
	if len(data) == 0 {
		panic("expected non-empty []byte")
	}
	if p.nlcount > 0 && p.indent > 0 {
		// write indentation
		n := p.indent
		for n > len(tabBytes) {
			p.write(tabBytes)
			n -= len(tabBytes)
		}
		p.write(tabBytes[:n])
	}
	p.write(data)
	p.nlcount = 0
}

func (p *printer) writeString(s string) {
	p.writeBytes([]byte(s))
}

// If impliesSemi returns true for a non-blank line's final token tok,
// a semicolon is automatically inserted. Vice versa, a semicolon may
// be omitted in those cases.
func impliesSemi(tok token) bool {
	switch tok {
	case _Name,
		_Break, _Continue, _Fallthrough, _Return,
		/*_Inc, _Dec,*/ _Rparen, _Rbrack, _Rbrace: // TODO(gri) fix this
		return true
	}
	return false
}

// TODO(gri) provide table of []byte values for all tokens to avoid repeated string conversion

func (p *printer) addWhitespace(kind ctrlSymbol, text string) {
	p.pending = append(p.pending, whitespace{p.lastTok, kind /*text*/})
	switch kind {
	case semi:
		p.lastTok = _Semi
	case newline:
		p.lastTok = 0
		// TODO(gri) do we need to handle /*-style comments containing newlines here?
	}
}

func (p *printer) flush(next token) {
	// eliminate semis and redundant whitespace
	sawNewline := next == _EOF
	sawParen := next == _Rparen || next == _Rbrace
	for i := len(p.pending) - 1; i >= 0; i-- {
		switch p.pending[i].kind {
		case semi:
			k := semi
			if sawParen {
				sawParen = false
				k = none // eliminate semi
			} else if sawNewline && impliesSemi(p.pending[i].last) {
				sawNewline = false
				k = none // eliminate semi
			}
			p.pending[i].kind = k
		case newline:
			sawNewline = true
		case blank, indent, outdent:
			// nothing to do
		// case comment:
		// 	// A multi-line comment acts like a newline; and a ""
		// 	// comment implies by definition at least one newline.
		// 	if text := p.pending[i].text; strings.HasPrefix(text, "/*") && strings.ContainsRune(text, '\n') {
		// 		sawNewline = true
		// 	}
		// case eolComment:
		// 	// TODO(gri) act depending on sawNewline
		default:
			panic("unreachable")
		}
	}

	// print pending
	prev := none
	for i := range p.pending {
		switch p.pending[i].kind {
		case none:
			// nothing to do
		case semi:
			p.writeString(";")
			p.nlcount = 0
			prev = semi
		case blank:
			if prev != blank {
				// at most one blank
				p.writeBytes(blankByte)
				p.nlcount = 0
				prev = blank
			}
		case newline:
			const maxEmptyLines = 1
			if p.nlcount <= maxEmptyLines {
				p.write(newlineByte)
				p.nlcount++
				prev = newline
			}
		case indent:
			p.indent++
		case outdent:
			p.indent--
			if p.indent < 0 {
				panic("negative indentation")
			}
		// case comment:
		// 	if text := p.pending[i].text; text != "" {
		// 		p.writeString(text)
		// 		p.nlcount = 0
		// 		prev = comment
		// 	}
		// 	// TODO(gri) should check that line comments are always followed by newline
		default:
			panic("unreachable")
		}
	}

	p.pending = p.pending[:0] // re-use underlying array
}

func mayCombine(prev token, next byte) (b bool) {
	return // for now
	// switch prev {
	// case lexical.Int:
	// 	b = next == '.' // 1.
	// case lexical.Add:
	// 	b = next == '+' // ++
	// case lexical.Sub:
	// 	b = next == '-' // --
	// case lexical.Quo:
	// 	b = next == '*' // /*
	// case lexical.Lss:
	// 	b = next == '-' || next == '<' // <- or <<
	// case lexical.And:
	// 	b = next == '&' || next == '^' // && or &^
	// }
	// return
}

func (p *printer) print(args ...any) {
	for i := 0; i < len(args); i++ {
		switch x := args[i].(type) {
		case nil:
			// we should not reach here but don't crash

		case Node:
			p.printNode(x)

		case token:
			// _Name implies an immediately following string
			// argument which is the actual value to print.
			var s string
			if x == _Name {
				i++
				if i >= len(args) {
					panic("missing string argument after _Name")
				}
				s = args[i].(string)
			} else {
				s = x.String()
			}

			// TODO(gri) This check seems at the wrong place since it doesn't
			//           take into account pending white space.
			if mayCombine(p.lastTok, s[0]) {
				panic("adjacent tokens combine without whitespace")
			}

			if x == _Semi {
				// delay printing of semi
				p.addWhitespace(semi, "")
			} else {
				p.flush(x)
				p.writeString(s)
				p.nlcount = 0
				p.lastTok = x
			}

		case Operator:
			if x != 0 {
				p.flush(_Operator)
				p.writeString(x.String())
			}

		case ctrlSymbol:
			switch x {
			case none, semi /*, comment*/ :
				panic("unreachable")
			case newline:
				// TODO(gri) need to handle mandatory newlines after a //-style comment
				if !p.linebreaks {
					x = blank
				}
			}
			p.addWhitespace(x, "")

		// case *Comment: // comments are not Nodes
		// 	p.addWhitespace(comment, x.Text)

		default:
			panic(fmt.Sprintf("unexpected argument %v (%T)", x, x))
		}
	}
}

func (p *printer) printNode(n Node) {
	// ncom := *n.Comments()
	// if ncom != nil {
	// 	// TODO(gri) in general we cannot make assumptions about whether
	// 	// a comment is a /*- or a //-style comment since the syntax
	// 	// tree may have been manipulated. Need to make sure the correct
	// 	// whitespace is emitted.
	// 	for _, c := range ncom.Alone {
	// 		p.print(c, newline)
	// 	}
	// 	for _, c := range ncom.Before {
	// 		if c.Text == "" || lineComment(c.Text) {
	// 			panic("unexpected empty line or //-style 'before' comment")
	// 		}
	// 		p.print(c, blank)
	// 	}
	// }

	p.printRawNode(n)

	// if ncom != nil && len(ncom.After) > 0 {
	// 	for i, c := range ncom.After {
	// 		if i+1 < len(ncom.After) {
	// 			if c.Text == "" || lineComment(c.Text) {
	// 				panic("unexpected empty line or //-style non-final 'after' comment")
	// 			}
	// 		}
	// 		p.print(blank, c)
	// 	}
	// 	//p.print(newline)
	// }
}

func (p *printer) printRawNode(n Node) {
	switch n := n.(type) {
	case nil:
		// we should not reach here but don't crash

	// expressions and types
	case *BadExpr:
		p.print(_Name, "<bad expr>")

	case *Name:
		p.print(_Name, n.Value) // _Name requires actual value following immediately

	case *BasicLit:
		p.print(_Name, n.Value) // _Name requires actual value following immediately

	case *FuncLit:
		p.print(n.Type, blank)
		if n.Body != nil {
			if p.form == ShortForm {
				p.print(_Lbrace)
				if len(n.Body.List) > 0 {
					p.print(_Name, "…")
				}
				p.print(_Rbrace)
			} else {
				p.print(n.Body)
			}
		}

	case *CompositeLit:
		if n.Type != nil {
			p.print(n.Type)
		}
		p.print(_Lbrace)
		if p.form == ShortForm {
			if len(n.ElemList) > 0 {
				p.print(_Name, "…")
			}
		} else {
			if n.NKeys > 0 && n.NKeys == len(n.ElemList) {
				p.printExprLines(n.ElemList)
			} else {
				p.printExprList(n.ElemList)
			}
		}
		p.print(_Rbrace)

	case *ParenExpr:
		p.print(_Lparen, n.X, _Rparen)

	case *SelectorExpr:
		p.print(n.X, _Dot, n.Sel)

	case *IndexExpr:
		p.print(n.X, _Lbrack, n.Index, _Rbrack)

	case *SliceExpr:
		p.print(n.X, _Lbrack)
		if i := n.Index[0]; i != nil {
			p.printNode(i)
		}
		p.print(_Colon)
		if j := n.Index[1]; j != nil {
			p.printNode(j)
		}
		if k := n.Index[2]; k != nil {
			p.print(_Colon, k)
		}
		p.print(_Rbrack)

	case *AssertExpr:
		p.print(n.X, _Dot, _Lparen, n.Type, _Rparen)

	case *TypeSwitchGuard:
		if n.Lhs != nil {
			p.print(n.Lhs, blank, _Define, blank)
		}
		p.print(n.X, _Dot, _Lparen, _Type, _Rparen)

	case *CallExpr:
		p.print(n.Fun, _Lparen)
		p.printExprList(n.ArgList)
		if n.HasDots {
			p.print(_DotDotDot)
		}
		p.print(_Rparen)

	case *Operation:
		if n.Y == nil {
			// unary expr
			p.print(n.Op)
			// if n.Op == lexical.Range {
			// 	p.print(blank)
			// }
			p.print(n.X)
		} else {
			// binary expr
			// TODO(gri) eventually take precedence into account
			// to control possibly missing parentheses
			p.print(n.X, blank, n.Op, blank, n.Y)
		}

	case *KeyValueExpr:
		p.print(n.Key, _Colon, blank, n.Value)

	case *ListExpr:
		p.printExprList(n.ElemList)

	case *ArrayType:
		var len any = _DotDotDot
		if n.Len != nil {
			len = n.Len
		}
		p.print(_Lbrack, len, _Rbrack, n.Elem)

	case *SliceType:
		p.print(_Lbrack, _Rbrack, n.Elem)

	case *DotsType:
		p.print(_DotDotDot, n.Elem)

	case *StructType:
		p.print(_Struct)
		if len(n.FieldList) > 0 && p.linebreaks {
			p.print(blank)
		}
		p.print(_Lbrace)
		if len(n.FieldList) > 0 {
			if p.linebreaks {
				p.print(newline, indent)
				p.printFieldList(n.FieldList, n.TagList, _Semi)
				p.print(outdent, newline)
			} else {
				p.printFieldList(n.FieldList, n.TagList, _Semi)
			}
		}
		p.print(_Rbrace)

	case *FuncType:
		p.print(_Func)
		p.printSignature(n)

	case *InterfaceType:
		p.print(_Interface)
		if p.linebreaks && len(n.MethodList) > 1 {
			p.print(blank)
			p.print(_Lbrace)
			p.print(newline, indent)
			p.printMethodList(n.MethodList)
			p.print(outdent, newline)
		} else {
			p.print(_Lbrace)
			p.printMethodList(n.MethodList)
		}
		p.print(_Rbrace)

	case *MapType:
		p.print(_Map, _Lbrack, n.Key, _Rbrack, n.Value)

	case *ChanType:
		if n.Dir == RecvOnly {
			p.print(_Arrow)
		}
		p.print(_Chan)
		if n.Dir == SendOnly {
			p.print(_Arrow)
		}
		p.print(blank)
		if e, _ := n.Elem.(*ChanType); n.Dir == 0 && e != nil && e.Dir == RecvOnly {
			// don't print chan (<-chan T) as chan <-chan T
			p.print(_Lparen)
			p.print(n.Elem)
			p.print(_Rparen)
		} else {
			p.print(n.Elem)
		}

	// statements
	case *DeclStmt:
		p.printDecl(n.DeclList)

	case *EmptyStmt:
		// nothing to print

	case *LabeledStmt:
		p.print(outdent, n.Label, _Colon, indent, newline, n.Stmt)

	case *ExprStmt:
		p.print(n.X)

	case *SendStmt:
		p.print(n.Chan, blank, _Arrow, blank, n.Value)

	case *AssignStmt:
		p.print(n.Lhs)
		if n.Rhs == nil {
			// TODO(gri) This is going to break the mayCombine
			//           check once we enable that again.
			p.print(n.Op, n.Op) // ++ or --
		} else {
			p.print(blank, n.Op, _Assign, blank)
			p.print(n.Rhs)
		}

	case *CallStmt:
		p.print(n.Tok, blank, n.Call)

	case *ReturnStmt:
		p.print(_Return)
		if n.Results != nil {
			p.print(blank, n.Results)
		}

	case *BranchStmt:
		p.print(n.Tok)
		if n.Label != nil {
			p.print(blank, n.Label)
		}

	case *BlockStmt:
		p.print(_Lbrace)
		if len(n.List) > 0 {
			p.print(newline, indent)
			p.printStmtList(n.List, true)
			p.print(outdent, newline)
		}
		p.print(_Rbrace)

	case *IfStmt:
		p.print(_If, blank)
		if n.Init != nil {
			p.print(n.Init, _Semi, blank)
		}
		p.print(n.Cond, blank, n.Then)
		if n.Else != nil {
			p.print(blank, _Else, blank, n.Else)
		}

	case *SwitchStmt:
		p.print(_Switch, blank)
		if n.Init != nil {
			p.print(n.Init, _Semi, blank)
		}
		if n.Tag != nil {
			p.print(n.Tag, blank)
		}
		p.printSwitchBody(n.Body)

	case *SelectStmt:
		p.print(_Select, blank) // for now
		p.printSelectBody(n.Body)

	case *RangeClause:
		if n.Lhs != nil {
			tok := _Assign
			if n.Def {
				tok = _Define
			}
			p.print(n.Lhs, blank, tok, blank)
		}
		p.print(_Range, blank, n.X)

	case *ForStmt:
		p.print(_For, blank)
		if n.Init == nil && n.Post == nil {
			if n.Cond != nil {
				p.print(n.Cond, blank)
			}
		} else {
			if n.Init != nil {
				p.print(n.Init)
				// TODO(gri) clean this up
				if _, ok := n.Init.(*RangeClause); ok {
					p.print(blank, n.Body)
					break
				}
			}
			p.print(_Semi, blank)
			if n.Cond != nil {
				p.print(n.Cond)
			}
			p.print(_Semi, blank)
			if n.Post != nil {
				p.print(n.Post, blank)
			}
		}
		p.print(n.Body)

	case *ImportDecl:
		if n.Group == nil {
			p.print(_Import, blank)
		}
		if n.LocalPkgName != nil {
			p.print(n.LocalPkgName, blank)
		}
		p.print(n.Path)

	case *ConstDecl:
		if n.Group == nil {
			p.print(_Const, blank)
		}
		p.printNameList(n.NameList)
		if n.Type != nil {
			p.print(blank, n.Type)
		}
		if n.Values != nil {
			p.print(blank, _Assign, blank, n.Values)
		}

	case *TypeDecl:
		if n.Group == nil {
			p.print(_Type, blank)
		}
		p.print(n.Name)
		if n.TParamList != nil {
			p.printParameterList(n.TParamList, _Type)
		}
		p.print(blank)
		if n.Alias {
			p.print(_Assign, blank)
		}
		p.print(n.Type)

	case *VarDecl:
		if n.Group == nil {
			p.print(_Var, blank)
		}
		p.printNameList(n.NameList)
		if n.Type != nil {
			p.print(blank, n.Type)
		}
		if n.Values != nil {
			p.print(blank, _Assign, blank, n.Values)
		}

	case *FuncDecl:
		p.print(_Func, blank)
		if r := n.Recv; r != nil {
			p.print(_Lparen)
			if r.Name != nil {
				p.print(r.Name, blank)
			}
			p.printNode(r.Type)
			p.print(_Rparen, blank)
		}
		p.print(n.Name)
		if n.TParamList != nil {
			p.printParameterList(n.TParamList, _Func)
		}
		p.printSignature(n.Type)
		if n.Body != nil {
			p.print(blank, n.Body)
		}

	case *printGroup:
		p.print(n.Tok, blank, _Lparen)
		if len(n.Decls) > 0 {
			p.print(newline, indent)
			for _, d := range n.Decls {
				p.printNode(d)
				p.print(_Semi, newline)
			}
			p.print(outdent)
		}
		p.print(_Rparen)

	// files
	case *File:
		p.print(_Package, blank, n.PkgName)
		if len(n.DeclList) > 0 {
			p.print(_Semi, newline, newline)
			p.printDeclList(n.DeclList)
		}

	default:
		panic(fmt.Sprintf("syntax.Iterate: unexpected node type %T", n))
	}
}

func (p *printer) printFields(fields []*Field, tags []*BasicLit, i, j int) {
	if i+1 == j && fields[i].Name == nil {
		// anonymous field
		p.printNode(fields[i].Type)
	} else {
		for k, f := range fields[i:j] {
			if k > 0 {
				p.print(_Comma, blank)
			}
			p.printNode(f.Name)
		}
		p.print(blank)
		p.printNode(fields[i].Type)
	}
	if i < len(tags) && tags[i] != nil {
		p.print(blank)
		p.printNode(tags[i])
	}
}

func (p *printer) printFieldList(fields []*Field, tags []*BasicLit, sep token) {
	i0 := 0
	var typ Expr
	for i, f := range fields {
		if f.Name == nil || f.Type != typ {
			if i0 < i {
				p.printFields(fields, tags, i0, i)
				p.print(sep, newline)
				i0 = i
			}
			typ = f.Type
		}
	}
	p.printFields(fields, tags, i0, len(fields))
}

func (p *printer) printMethodList(methods []*Field) {
	for i, m := range methods {
		if i > 0 {
			p.print(_Semi, newline)
		}
		if m.Name != nil {
			p.printNode(m.Name)
			p.printSignature(m.Type.(*FuncType))
		} else {
			p.printNode(m.Type)
		}
	}
}

func (p *printer) printNameList(list []*Name) {
	for i, x := range list {
		if i > 0 {
			p.print(_Comma, blank)
		}
		p.printNode(x)
	}
}

func (p *printer) printExprList(list []Expr) {
	for i, x := range list {
		if i > 0 {
			p.print(_Comma, blank)
		}
		p.printNode(x)
	}
}

func (p *printer) printExprLines(list []Expr) {
	if len(list) > 0 {
		p.print(newline, indent)
		for _, x := range list {
			p.print(x, _Comma, newline)
		}
		p.print(outdent)
	}
}

func groupFor(d Decl) (token, *Group) {
	switch d := d.(type) {
	case *ImportDecl:
		return _Import, d.Group
	case *ConstDecl:
		return _Const, d.Group
	case *TypeDecl:
		return _Type, d.Group
	case *VarDecl:
		return _Var, d.Group
	case *FuncDecl:
		return _Func, nil
	default:
		panic("unreachable")
	}
}

type printGroup struct {
	node
	Tok   token
	Decls []Decl
}

func (p *printer) printDecl(list []Decl) {
	tok, group := groupFor(list[0])

	if group == nil {
		if len(list) != 1 {
			panic("unreachable")
		}
		p.printNode(list[0])
		return
	}

	// if _, ok := list[0].(*EmptyDecl); ok {
	// 	if len(list) != 1 {
	// 		panic("unreachable")
	// 	}
	// 	// TODO(gri) if there are comments inside the empty
	// 	// group, we may need to keep the list non-nil
	// 	list = nil
	// }

	// printGroup is here for consistent comment handling
	// (this is not yet used)
	var pg printGroup
	// *pg.Comments() = *group.Comments()
	pg.Tok = tok
	pg.Decls = list
	p.printNode(&pg)
}

func (p *printer) printDeclList(list []Decl) {
	i0 := 0
	var tok token
	var group *Group
	for i, x := range list {
		if s, g := groupFor(x); g == nil || g != group {
			if i0 < i {
				p.printDecl(list[i0:i])
				p.print(_Semi, newline)
				// print empty line between different declaration groups,
				// different kinds of declarations, or between functions
				if g != group || s != tok || s == _Func {
					p.print(newline)
				}
				i0 = i
			}
			tok, group = s, g
		}
	}
	p.printDecl(list[i0:])
}

func (p *printer) printSignature(sig *FuncType) {
	p.printParameterList(sig.ParamList, 0)
	if list := sig.ResultList; list != nil {
		p.print(blank)
		if len(list) == 1 && list[0].Name == nil {
			p.printNode(list[0].Type)
		} else {
			p.printParameterList(list, 0)
		}
	}
}

// If tok != 0 print a type parameter list: tok == _Type means
// a type parameter list for a type, tok == _Func means a type
// parameter list for a func.
func (p *printer) printParameterList(list []*Field, tok token) {
	open, close := _Lparen, _Rparen
	if tok != 0 {
		open, close = _Lbrack, _Rbrack
	}
	p.print(open)
	for i, f := range list {
		if i > 0 {
			p.print(_Comma, blank)
		}
		if f.Name != nil {
			p.printNode(f.Name)
			if i+1 < len(list) {
				f1 := list[i+1]
				if f1.Name != nil && f1.Type == f.Type {
					continue // no need to print type
				}
			}
			p.print(blank)
		}
		p.printNode(f.Type)
	}
	// A type parameter list [P T] where the name P and the type expression T syntactically
	// combine to another valid (value) expression requires a trailing comma, as in [P *T,]
	// (or an enclosing interface as in [P interface(*T)]), so that the type parameter list
	// is not parsed as an array length [P*T].
	if tok == _Type && len(list) == 1 && combinesWithName(list[0].Type) {
		p.print(_Comma)
	}
	p.print(close)
}

// combinesWithName reports whether a name followed by the expression x
// syntactically combines to another valid (value) expression. For instance
// using *T for x, "name *T" syntactically appears as the expression x*T.
// On the other hand, using  P|Q or *P|~Q for x, "name P|Q" or "name *P|~Q"
// cannot be combined into a valid (value) expression.
func combinesWithName(x Expr) bool {
	switch x := x.(type) {
	case *Operation:
		if x.Y == nil {
			// name *x.X combines to name*x.X if x.X is not a type element
			return x.Op == Mul && !isTypeElem(x.X)
		}
		// binary expressions
		return combinesWithName(x.X) && !isTypeElem(x.Y)
	case *ParenExpr:
		// Note that the parser strips parentheses in these cases
		// (see extractName, parser.typeOrNil) unless keep_parens
		// is set, so we should never reach here.
		// Do the right thing (rather than panic) for testing and
		// in case we change parser behavior.
		// See also go.dev/issues/69206.
		return !isTypeElem(x.X)
	}
	return false
}

func (p *printer) printStmtList(list []Stmt, braces bool) {
	for i, x := range list {
		p.print(x, _Semi)
		if i+1 < len(list) {
			p.print(newline)
		} else if braces {
			// Print an extra semicolon if the last statement is
			// an empty statement and we are in a braced block
			// because one semicolon is automatically removed.
			if _, ok := x.(*EmptyStmt); ok {
				p.print(x, _Semi)
			}
		}
	}
}

func (p *printer) printSwitchBody(list []*CaseClause) {
	p.print(_Lbrace)
	if len(list) > 0 {
		p.print(newline)
		for i, c := range list {
			p.printCaseClause(c, i+1 == len(list))
			p.print(newline)
		}
	}
	p.print(_Rbrace)
}

func (p *printer) printSelectBody(list []*CommClause) {
	p.print(_Lbrace)
	if len(list) > 0 {
		p.print(newline)
		for i, c := range list {
			p.printCommClause(c, i+1 == len(list))
			p.print(newline)
		}
	}
	p.print(_Rbrace)
}

func (p *printer) printCaseClause(c *CaseClause, braces bool) {
	if c.Cases != nil {
		p.print(_Case, blank, c.Cases)
	} else {
		p.print(_Default)
	}
	p.print(_Colon)
	if len(c.Body) > 0 {
		p.print(newline, indent)
		p.printStmtList(c.Body, braces)
		p.print(outdent)
	}
}

func (p *printer) printCommClause(c *CommClause, braces bool) {
	if c.Comm != nil {
		p.print(_Case, blank)
		p.print(c.Comm)
	} else {
		p.print(_Default)
	}
	p.print(_Colon)
	if len(c.Body) > 0 {
		p.print(newline, indent)
		p.printStmtList(c.Body, braces)
		p.print(outdent)
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/scanner.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements scanner, a lexical tokenizer for
// Go source. After initialization, consecutive calls of
// next advance the scanner one token at a time.
//
// This file, source.go, tokens.go, and token_string.go are self-contained
// (`go tool compile scanner.go source.go tokens.go token_string.go` compiles)
// and thus could be made into their own package.

package syntax

import (
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"
)

// The mode flags below control which comments are reported
// by calling the error handler. If no flag is set, comments
// are ignored.
const (
	comments   uint = 1 << iota // call handler for all comments
	directives                  // call handler for directives only
)

type scanner struct {
	source
	mode   uint
	nlsemi bool // if set '\n' and EOF translate to ';'

	// current token, valid after calling next()
	line, col uint
	blank     bool // line is blank up to col
	tok       token
	lit       string   // valid if tok is _Name, _Literal, or _Semi ("semicolon", "newline", or "EOF"); may be malformed if bad is true
	bad       bool     // valid if tok is _Literal, true if a syntax error occurred, lit may be malformed
	kind      LitKind  // valid if tok is _Literal
	op        Operator // valid if tok is _Operator, _Star, _AssignOp, or _IncOp
	prec      int      // valid if tok is _Operator, _Star, _AssignOp, or _IncOp
}

func (s *scanner) init(src io.Reader, errh func(line, col uint, msg string), mode uint) {
	s.source.init(src, errh)
	s.mode = mode
	s.nlsemi = false
}

// errorf reports an error at the most recently read character position.
func (s *scanner) errorf(format string, args ...any) {
	s.error(fmt.Sprintf(format, args...))
}

// errorAtf reports an error at a byte column offset relative to the current token start.
func (s *scanner) errorAtf(offset int, format string, args ...any) {
	s.errh(s.line, s.col+uint(offset), fmt.Sprintf(format, args...))
}

// setLit sets the scanner state for a recognized _Literal token.
func (s *scanner) setLit(kind LitKind, ok bool) {
	s.nlsemi = true
	s.tok = _Literal
	s.lit = string(s.segment())
	s.bad = !ok
	s.kind = kind
}

// next advances the scanner by reading the next token.
//
// If a read, source encoding, or lexical error occurs, next calls
// the installed error handler with the respective error position
// and message. The error message is guaranteed to be non-empty and
// never starts with a '/'. The error handler must exist.
//
// If the scanner mode includes the comments flag and a comment
// (including comments containing directives) is encountered, the
// error handler is also called with each comment position and text
// (including opening /* or // and closing */, but without a newline
// at the end of line comments). Comment text always starts with a /
// which can be used to distinguish these handler calls from errors.
//
// If the scanner mode includes the directives (but not the comments)
// flag, only comments containing a //line, /*line, or //go: directive
// are reported, in the same way as regular comments.
func (s *scanner) next() {
	nlsemi := s.nlsemi
	s.nlsemi = false

redo:
	// skip white space
	s.stop()
	startLine, startCol := s.pos()
	for s.ch == ' ' || s.ch == '\t' || s.ch == '\n' && !nlsemi || s.ch == '\r' {
		s.nextch()
	}

	// token start
	s.line, s.col = s.pos()
	s.blank = s.line > startLine || startCol == colbase
	s.start()
	if isLetter(s.ch) || s.ch >= utf8.RuneSelf && s.atIdentChar(true) {
		s.nextch()
		s.ident()
		return
	}

	switch s.ch {
	case -1:
		if nlsemi {
			s.lit = "EOF"
			s.tok = _Semi
			break
		}
		s.tok = _EOF

	case '\n':
		s.nextch()
		s.lit = "newline"
		s.tok = _Semi

	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		s.number(false)

	case '"':
		s.stdString()

	case '`':
		s.rawString()

	case '\'':
		s.rune()

	case '(':
		s.nextch()
		s.tok = _Lparen

	case '[':
		s.nextch()
		s.tok = _Lbrack

	case '{':
		s.nextch()
		s.tok = _Lbrace

	case ',':
		s.nextch()
		s.tok = _Comma

	case ';':
		s.nextch()
		s.lit = "semicolon"
		s.tok = _Semi

	case ')':
		s.nextch()
		s.nlsemi = true
		s.tok = _Rparen

	case ']':
		s.nextch()
		s.nlsemi = true
		s.tok = _Rbrack

	case '}':
		s.nextch()
		s.nlsemi = true
		s.tok = _Rbrace

	case ':':
		s.nextch()
		if s.ch == '=' {
			s.nextch()
			s.tok = _Define
			break
		}
		s.tok = _Colon

	case '.':
		s.nextch()
		if isDecimal(s.ch) {
			s.number(true)
			break
		}
		if s.ch == '.' {
			s.nextch()
			if s.ch == '.' {
				s.nextch()
				s.tok = _DotDotDot
				break
			}
			s.rewind() // now s.ch holds 1st '.'
			s.nextch() // consume 1st '.' again
		}
		s.tok = _Dot

	case '+':
		s.nextch()
		s.op, s.prec = Add, precAdd
		if s.ch != '+' {
			goto assignop
		}
		s.nextch()
		s.nlsemi = true
		s.tok = _IncOp

	case '-':
		s.nextch()
		s.op, s.prec = Sub, precAdd
		if s.ch != '-' {
			goto assignop
		}
		s.nextch()
		s.nlsemi = true
		s.tok = _IncOp

	case '*':
		s.nextch()
		s.op, s.prec = Mul, precMul
		// don't goto assignop - want _Star token
		if s.ch == '=' {
			s.nextch()
			s.tok = _AssignOp
			break
		}
		s.tok = _Star

	case '/':
		s.nextch()
		if s.ch == '/' {
			s.nextch()
			s.lineComment()
			goto redo
		}
		if s.ch == '*' {
			s.nextch()
			s.fullComment()
			if line, _ := s.pos(); line > s.line && nlsemi {
				// A multi-line comment acts like a newline;
				// it translates to a ';' if nlsemi is set.
				s.lit = "newline"
				s.tok = _Semi
				break
			}
			goto redo
		}
		s.op, s.prec = Div, precMul
		goto assignop

	case '%':
		s.nextch()
		s.op, s.prec = Rem, precMul
		goto assignop

	case '&':
		s.nextch()
		if s.ch == '&' {
			s.nextch()
			s.op, s.prec = AndAnd, precAndAnd
			s.tok = _Operator
			break
		}
		s.op, s.prec = And, precMul
		if s.ch == '^' {
			s.nextch()
			s.op = AndNot
		}
		goto assignop

	case '|':
		s.nextch()
		if s.ch == '|' {
			s.nextch()
			s.op, s.prec = OrOr, precOrOr
			s.tok = _Operator
			break
		}
		s.op, s.prec = Or, precAdd
		goto assignop

	case '^':
		s.nextch()
		s.op, s.prec = Xor, precAdd
		goto assignop

	case '<':
		s.nextch()
		if s.ch == '=' {
			s.nextch()
			s.op, s.prec = Leq, precCmp
			s.tok = _Operator
			break
		}
		if s.ch == '<' {
			s.nextch()
			s.op, s.prec = Shl, precMul
			goto assignop
		}
		if s.ch == '-' {
			s.nextch()
			s.tok = _Arrow
			break
		}
		s.op, s.prec = Lss, precCmp
		s.tok = _Operator

	case '>':
		s.nextch()
		if s.ch == '=' {
			s.nextch()
			s.op, s.prec = Geq, precCmp
			s.tok = _Operator
			break
		}
		if s.ch == '>' {
			s.nextch()
			s.op, s.prec = Shr, precMul
			goto assignop
		}
		s.op, s.prec = Gtr, precCmp
		s.tok = _Operator

	case '=':
		s.nextch()
		if s.ch == '=' {
			s.nextch()
			s.op, s.prec = Eql, precCmp
			s.tok = _Operator
			break
		}
		s.tok = _Assign

	case '!':
		s.nextch()
		if s.ch == '=' {
			s.nextch()
			s.op, s.prec = Neq, precCmp
			s.tok = _Operator
			break
		}
		s.op, s.prec = Not, 0
		s.tok = _Operator

	case '~':
		s.nextch()
		s.op, s.prec = Tilde, 0
		s.tok = _Operator

	default:
		s.errorf("invalid character %#U", s.ch)
		s.nextch()
		goto redo
	}

	return

assignop:
	if s.ch == '=' {
		s.nextch()
		s.tok = _AssignOp
		return
	}
	s.tok = _Operator
}

func (s *scanner) ident() {
	// accelerate common case (7bit ASCII)
	for isLetter(s.ch) || isDecimal(s.ch) {
		s.nextch()
	}

	// general case
	if s.ch >= utf8.RuneSelf {
		for s.atIdentChar(false) {
			s.nextch()
		}
	}

	// possibly a keyword
	lit := s.segment()
	if len(lit) >= 2 {
		if tok := keywordMap[hash(lit)]; tok != 0 && tokStrFast(tok) == string(lit) {
			s.nlsemi = contains(1<<_Break|1<<_Continue|1<<_Fallthrough|1<<_Return, tok)
			s.tok = tok
			return
		}
	}

	s.nlsemi = true
	s.lit = string(lit)
	s.tok = _Name
}

// tokStrFast is a faster version of token.String, which assumes that tok
// is one of the valid tokens - and can thus skip bounds checks.
func tokStrFast(tok token) string {
	return _token_name[_token_index[tok-1]:_token_index[tok]]
}

func (s *scanner) atIdentChar(first bool) bool {
	switch {
	case unicode.IsLetter(s.ch) || s.ch == '_':
		// ok
	case unicode.IsDigit(s.ch):
		if first {
			s.errorf("identifier cannot begin with digit %#U", s.ch)
		}
	case s.ch >= utf8.RuneSelf:
		s.errorf("invalid character %#U in identifier", s.ch)
	default:
		return false
	}
	return true
}

// hash is a perfect hash function for keywords.
// It assumes that s has at least length 2.
func hash(s []byte) uint {
	return (uint(s[0])<<4 ^ uint(s[1]) + uint(len(s))) & uint(len(keywordMap)-1)
}

var keywordMap [1 << 6]token // size must be power of two

func init() {
	// populate keywordMap
	for tok := _Break; tok <= _Var; tok++ {
		h := hash([]byte(tok.String()))
		if keywordMap[h] != 0 {
			panic("imperfect hash")
		}
		keywordMap[h] = tok
	}
}

func lower(ch rune) rune     { return ('a' - 'A') | ch } // returns lower-case ch iff ch is ASCII letter
func isLetter(ch rune) bool  { return 'a' <= lower(ch) && lower(ch) <= 'z' || ch == '_' }
func isDecimal(ch rune) bool { return '0' <= ch && ch <= '9' }
func isHex(ch rune) bool     { return '0' <= ch && ch <= '9' || 'a' <= lower(ch) && lower(ch) <= 'f' }

// digits accepts the sequence { digit | '_' }.
// If base <= 10, digits accepts any decimal digit but records
// the index (relative to the literal start) of a digit >= base
// in *invalid, if *invalid < 0.
// digits returns a bitset describing whether the sequence contained
// digits (bit 0 is set), or separators '_' (bit 1 is set).
func (s *scanner) digits(base int, invalid *int) (digsep int) {
	if base <= 10 {
		max := rune('0' + base)
		for isDecimal(s.ch) || s.ch == '_' {
			ds := 1
			if s.ch == '_' {
				ds = 2
			} else if s.ch >= max && *invalid < 0 {
				_, col := s.pos()
				*invalid = int(col - s.col) // record invalid rune index
			}
			digsep |= ds
			s.nextch()
		}
	} else {
		for isHex(s.ch) || s.ch == '_' {
			ds := 1
			if s.ch == '_' {
				ds = 2
			}
			digsep |= ds
			s.nextch()
		}
	}
	return
}

func (s *scanner) number(seenPoint bool) {
	ok := true
	kind := IntLit
	base := 10        // number base
	prefix := rune(0) // one of 0 (decimal), '0' (0-octal), 'x', 'o', or 'b'
	digsep := 0       // bit 0: digit present, bit 1: '_' present
	invalid := -1     // index of invalid digit in literal, or < 0

	// integer part
	if !seenPoint {
		if s.ch == '0' {
			s.nextch()
			switch lower(s.ch) {
			case 'x':
				s.nextch()
				base, prefix = 16, 'x'
			case 'o':
				s.nextch()
				base, prefix = 8, 'o'
			case 'b':
				s.nextch()
				base, prefix = 2, 'b'
			default:
				base, prefix = 8, '0'
				digsep = 1 // leading 0
			}
		}
		digsep |= s.digits(base, &invalid)
		if s.ch == '.' {
			if prefix == 'o' || prefix == 'b' {
				s.errorf("invalid radix point in %s literal", baseName(base))
				ok = false
			}
			s.nextch()
			seenPoint = true
		}
	}

	// fractional part
	if seenPoint {
		kind = FloatLit
		digsep |= s.digits(base, &invalid)
	}

	if digsep&1 == 0 && ok {
		s.errorf("%s literal has no digits", baseName(base))
		ok = false
	}

	// exponent
	if e := lower(s.ch); e == 'e' || e == 'p' {
		if ok {
			switch {
			case e == 'e' && prefix != 0 && prefix != '0':
				s.errorf("%q exponent requires decimal mantissa", s.ch)
				ok = false
			case e == 'p' && prefix != 'x':
				s.errorf("%q exponent requires hexadecimal mantissa", s.ch)
				ok = false
			}
		}
		s.nextch()
		kind = FloatLit
		if s.ch == '+' || s.ch == '-' {
			s.nextch()
		}
		digsep = s.digits(10, nil) | digsep&2 // don't lose sep bit
		if digsep&1 == 0 && ok {
			s.errorf("exponent has no digits")
			ok = false
		}
	} else if prefix == 'x' && kind == FloatLit && ok {
		s.errorf("hexadecimal mantissa requires a 'p' exponent")
		ok = false
	}

	// suffix 'i'
	if s.ch == 'i' {
		kind = ImagLit
		s.nextch()
	}

	s.setLit(kind, ok) // do this now so we can use s.lit below

	if kind == IntLit && invalid >= 0 && ok {
		s.errorAtf(invalid, "invalid digit %q in %s literal", s.lit[invalid], baseName(base))
		ok = false
	}

	if digsep&2 != 0 && ok {
		if i := invalidSep(s.lit); i >= 0 {
			s.errorAtf(i, "'_' must separate successive digits")
			ok = false
		}
	}

	s.bad = !ok // correct s.bad
}

func baseName(base int) string {
	switch base {
	case 2:
		return "binary"
	case 8:
		return "octal"
	case 10:
		return "decimal"
	case 16:
		return "hexadecimal"
	}
	panic("invalid base")
}

// invalidSep returns the index of the first invalid separator in x, or -1.
func invalidSep(x string) int {
	x1 := ' ' // prefix char, we only care if it's 'x'
	d := '.'  // digit, one of '_', '0' (a digit), or '.' (anything else)
	i := 0

	// a prefix counts as a digit
	if len(x) >= 2 && x[0] == '0' {
		x1 = lower(rune(x[1]))
		if x1 == 'x' || x1 == 'o' || x1 == 'b' {
			d = '0'
			i = 2
		}
	}

	// mantissa and exponent
	for ; i < len(x); i++ {
		p := d // previous digit
		d = rune(x[i])
		switch {
		case d == '_':
			if p != '0' {
				return i
			}
		case isDecimal(d) || x1 == 'x' && isHex(d):
			d = '0'
		default:
			if p == '_' {
				return i - 1
			}
			d = '.'
		}
	}
	if d == '_' {
		return len(x) - 1
	}

	return -1
}

func (s *scanner) rune() {
	ok := true
	s.nextch()

	n := 0
	for ; ; n++ {
		if s.ch == '\'' {
			if ok {
				if n == 0 {
					s.errorf("empty rune literal or unescaped '")
					ok = false
				} else if n != 1 {
					s.errorAtf(0, "more than one character in rune literal")
					ok = false
				}
			}
			s.nextch()
			break
		}
		if s.ch == '\\' {
			s.nextch()
			if !s.escape('\'') {
				ok = false
			}
			continue
		}
		if s.ch == '\n' {
			if ok {
				s.errorf("newline in rune literal")
				ok = false
			}
			break
		}
		if s.ch < 0 {
			if ok {
				s.errorAtf(0, "rune literal not terminated")
				ok = false
			}
			break
		}
		s.nextch()
	}

	s.setLit(RuneLit, ok)
}

func (s *scanner) stdString() {
	ok := true
	s.nextch()

	for {
		if s.ch == '"' {
			s.nextch()
			break
		}
		if s.ch == '\\' {
			s.nextch()
			if !s.escape('"') {
				ok = false
			}
			continue
		}
		if s.ch == '\n' {
			s.errorf("newline in string")
			ok = false
			break
		}
		if s.ch < 0 {
			s.errorAtf(0, "string not terminated")
			ok = false
			break
		}
		s.nextch()
	}

	s.setLit(StringLit, ok)
}

func (s *scanner) rawString() {
	ok := true
	s.nextch()

	for {
		if s.ch == '`' {
			s.nextch()
			break
		}
		if s.ch < 0 {
			s.errorAtf(0, "string not terminated")
			ok = false
			break
		}
		s.nextch()
	}
	// We leave CRs in the string since they are part of the
	// literal (even though they are not part of the literal
	// value).

	s.setLit(StringLit, ok)
}

func (s *scanner) comment(text string) {
	s.errorAtf(0, "%s", text)
}

func (s *scanner) skipLine() {
	// don't consume '\n' - needed for nlsemi logic
	for s.ch >= 0 && s.ch != '\n' {
		s.nextch()
	}
}

func (s *scanner) lineComment() {
	// opening has already been consumed

	if s.mode&comments != 0 {
		s.skipLine()
		s.comment(string(s.segment()))
		return
	}

	// are we saving directives? or is this definitely not a directive?
	if s.mode&directives == 0 || (s.ch != 'g' && s.ch != 'l') {
		s.stop()
		s.skipLine()
		return
	}

	// recognize go: or line directives
	prefix := "go:"
	if s.ch == 'l' {
		prefix = "line "
	}
	for _, m := range prefix {
		if s.ch != m {
			s.stop()
			s.skipLine()
			return
		}
		s.nextch()
	}

	// directive text
	s.skipLine()
	s.comment(string(s.segment()))
}

func (s *scanner) skipComment() bool {
	for s.ch >= 0 {
		for s.ch == '*' {
			s.nextch()
			if s.ch == '/' {
				s.nextch()
				return true
			}
		}
		s.nextch()
	}
	s.errorAtf(0, "comment not terminated")
	return false
}

func (s *scanner) fullComment() {
	/* opening has already been consumed */

	if s.mode&comments != 0 {
		if s.skipComment() {
			s.comment(string(s.segment()))
		}
		return
	}

	if s.mode&directives == 0 || s.ch != 'l' {
		s.stop()
		s.skipComment()
		return
	}

	// recognize line directive
	const prefix = "line "
	for _, m := range prefix {
		if s.ch != m {
			s.stop()
			s.skipComment()
			return
		}
		s.nextch()
	}

	// directive text
	if s.skipComment() {
		s.comment(string(s.segment()))
	}
}

func (s *scanner) escape(quote rune) bool {
	var n int
	var base, max uint32

	switch s.ch {
	case quote, 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\':
		s.nextch()
		return true
	case '0', '1', '2', '3', '4', '5', '6', '7':
		n, base, max = 3, 8, 255
	case 'x':
		s.nextch()
		n, base, max = 2, 16, 255
	case 'u':
		s.nextch()
		n, base, max = 4, 16, unicode.MaxRune
	case 'U':
		s.nextch()
		n, base, max = 8, 16, unicode.MaxRune
	default:
		if s.ch < 0 {
			return true // complain in caller about EOF
		}
		s.errorf("unknown escape")
		return false
	}

	var x uint32
	for i := n; i > 0; i-- {
		if s.ch < 0 {
			return true // complain in caller about EOF
		}
		d := base
		if isDecimal(s.ch) {
			d = uint32(s.ch) - '0'
		} else if 'a' <= lower(s.ch) && lower(s.ch) <= 'f' {
			d = uint32(lower(s.ch)) - 'a' + 10
		}
		if d >= base {
			s.errorf("invalid character %q in %s escape", s.ch, baseName(int(base)))
			return false
		}
		// d < base
		x = x*base + d
		s.nextch()
	}

	if x > max && base == 8 {
		s.errorf("octal escape value %d > 255", x)
		return false
	}

	if x > max || 0xD800 <= x && x < 0xE000 /* surrogate range */ {
		s.errorf("escape is invalid Unicode code point %#U", x)
		return false
	}

	return true
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/source.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements source, a buffered rune reader
// specialized for scanning Go code: Reading
// ASCII characters, maintaining current (line, col)
// position information, and recording of the most
// recently read source segment are highly optimized.
// This file is self-contained (go tool compile source.go
// compiles) and thus could be made into its own package.

package syntax

import (
	"io"
	"unicode/utf8"
)

// The source buffer is accessed using three indices b (begin),
// r (read), and e (end):
//
// - If b >= 0, it points to the beginning of a segment of most
//   recently read characters (typically a Go literal).
//
// - r points to the byte immediately following the most recently
//   read character ch, which starts at r-chw.
//
// - e points to the byte immediately following the last byte that
//   was read into the buffer.
//
// The buffer content is terminated at buf[e] with the sentinel
// character utf8.RuneSelf. This makes it possible to test for
// the common case of ASCII characters with a single 'if' (see
// nextch method).
//
//                +------ content in use -------+
//                v                             v
// buf [...read...|...segment...|ch|...unread...|s|...free...]
//                ^             ^  ^            ^
//                |             |  |            |
//                b         r-chw  r            e
//
// Invariant: -1 <= b < r <= e < len(buf) && buf[e] == sentinel

type source struct {
	in   io.Reader
	errh func(line, col uint, msg string)

	buf       []byte // source buffer
	ioerr     error  // pending I/O error, or nil
	b, r, e   int    // buffer indices (see comment above)
	line, col uint   // source position of ch (0-based)
	ch        rune   // most recently read character
	chw       int    // width of ch
}

const sentinel = utf8.RuneSelf

func (s *source) init(in io.Reader, errh func(line, col uint, msg string)) {
	s.in = in
	s.errh = errh

	if s.buf == nil {
		s.buf = make([]byte, nextSize(0))
	}
	s.buf[0] = sentinel
	s.ioerr = nil
	s.b, s.r, s.e = -1, 0, 0
	s.line, s.col = 0, 0
	s.ch = ' '
	s.chw = 0
}

// starting points for line and column numbers
const linebase = 1
const colbase = 1

// pos returns the (line, col) source position of s.ch.
func (s *source) pos() (line, col uint) {
	return linebase + s.line, colbase + s.col
}

// error reports the error msg at source position s.pos().
func (s *source) error(msg string) {
	line, col := s.pos()
	s.errh(line, col, msg)
}

// start starts a new active source segment (including s.ch).
// As long as stop has not been called, the active segment's
// bytes (excluding s.ch) may be retrieved by calling segment.
func (s *source) start()          { s.b = s.r - s.chw }
func (s *source) stop()           { s.b = -1 }
func (s *source) segment() []byte { return s.buf[s.b : s.r-s.chw] }

// rewind rewinds the scanner's read position and character s.ch
// to the start of the currently active segment, which must not
// contain any newlines (otherwise position information will be
// incorrect). Currently, rewind is only needed for handling the
// source sequence ".."; it must not be called outside an active
// segment.
func (s *source) rewind() {
	// ok to verify precondition - rewind is rarely called
	if s.b < 0 {
		panic("no active segment")
	}
	s.col -= uint(s.r - s.b)
	s.r = s.b
	s.nextch()
}

func (s *source) nextch() {
redo:
	s.col += uint(s.chw)
	if s.ch == '\n' {
		s.line++
		s.col = 0
	}

	// fast common case: at least one ASCII character
	if s.ch = rune(s.buf[s.r]); s.ch < sentinel {
		s.r++
		s.chw = 1
		if s.ch == 0 {
			s.error("invalid NUL character")
			goto redo
		}
		return
	}

	// slower general case: add more bytes to buffer if we don't have a full rune
	for s.e-s.r < utf8.UTFMax && !utf8.FullRune(s.buf[s.r:s.e]) && s.ioerr == nil {
		s.fill()
	}

	// EOF
	if s.r == s.e {
		if s.ioerr != io.EOF {
			// ensure we never start with a '/' (e.g., rooted path) in the error message
			s.error("I/O error: " + s.ioerr.Error())
			s.ioerr = nil
		}
		s.ch = -1
		s.chw = 0
		return
	}

	s.ch, s.chw = utf8.DecodeRune(s.buf[s.r:s.e])
	s.r += s.chw

	if s.ch == utf8.RuneError && s.chw == 1 {
		s.error("invalid UTF-8 encoding")
		goto redo
	}

	// BOM's are only allowed as the first character in a file
	const BOM = 0xfeff
	if s.ch == BOM {
		if s.line > 0 || s.col > 0 {
			s.error("invalid BOM in the middle of the file")
		}
		goto redo
	}
}

// fill reads more source bytes into s.buf.
// It returns with at least one more byte in the buffer, or with s.ioerr != nil.
func (s *source) fill() {
	// determine content to preserve
	b := s.r
	if s.b >= 0 {
		b = s.b
		s.b = 0 // after buffer has grown or content has been moved down
	}
	content := s.buf[b:s.e]

	// grow buffer or move content down
	if len(content)*2 > len(s.buf) {
		s.buf = make([]byte, nextSize(len(s.buf)))
		copy(s.buf, content)
	} else if b > 0 {
		copy(s.buf, content)
	}
	s.r -= b
	s.e -= b

	// read more data: try a limited number of times
	for i := 0; i < 10; i++ {
		var n int
		n, s.ioerr = s.in.Read(s.buf[s.e : len(s.buf)-1]) // -1 to leave space for sentinel
		if n < 0 {
			panic("negative read") // incorrect underlying io.Reader implementation
		}
		if n > 0 || s.ioerr != nil {
			s.e += n
			s.buf[s.e] = sentinel
			return
		}
		// n == 0
	}

	s.buf[s.e] = sentinel
	s.ioerr = io.ErrNoProgress
}

// nextSize returns the next bigger size for a buffer of a given size.
func nextSize(size int) int {
	const min = 4 << 10 // 4K: minimum buffer size
	const max = 1 << 20 // 1M: maximum buffer size which is still doubled
	if size < min {
		return min
	}
	if size <= max {
		return size << 1
	}
	return size + max
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/syntax.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import (
	"fmt"
	"io"
	"os"
)

// Mode describes the parser mode.
type Mode uint

// Modes supported by the parser.
const (
	CheckBranches Mode = 1 << iota // check correct use of labels, break, continue, and goto statements
)

// Error describes a syntax error. Error implements the error interface.
type Error struct {
	Pos Pos
	Msg string
}

func (err Error) Error() string {
	return fmt.Sprintf("%s: %s", err.Pos, err.Msg)
}

var _ error = Error{} // verify that Error implements error

// An ErrorHandler is called for each error encountered reading a .go file.
type ErrorHandler func(err error)

// A Pragma value augments a package, import, const, func, type, or var declaration.
// Its meaning is entirely up to the PragmaHandler,
// except that nil is used to mean “no pragma seen.”
type Pragma any

// A PragmaHandler is used to process //go: directives while scanning.
// It is passed the current pragma value, which starts out being nil,
// and it returns an updated pragma value.
// The text is the directive, with the "//" prefix stripped.
// The current pragma is saved at each package, import, const, func, type, or var
// declaration, into the File, ImportDecl, ConstDecl, FuncDecl, TypeDecl, or VarDecl node.
//
// If text is the empty string, the pragma is being returned
// to the handler unused, meaning it appeared before a non-declaration.
// The handler may wish to report an error. In this case, pos is the
// current parser position, not the position of the pragma itself.
// Blank specifies whether the line is blank before the pragma.
type PragmaHandler func(pos Pos, blank bool, text string, current Pragma) Pragma

// Parse parses a single Go source file from src and returns the corresponding
// syntax tree. If there are errors, Parse will return the first error found,
// and a possibly partially constructed syntax tree, or nil.
//
// If errh != nil, it is called with each error encountered, and Parse will
// process as much source as possible. In this case, the returned syntax tree
// is only nil if no correct package clause was found.
// If errh is nil, Parse will terminate immediately upon encountering the first
// error, and the returned syntax tree is nil.
//
// If pragh != nil, it is called with each pragma encountered.
func Parse(base *PosBase, src io.Reader, errh ErrorHandler, pragh PragmaHandler, mode Mode) (_ *File, first error) {
	defer func() {
		if p := recover(); p != nil {
			if err, ok := p.(Error); ok {
				first = err
				return
			}
			panic(p)
		}
	}()

	var p parser
	p.init(base, src, errh, pragh, mode)
	p.next()
	return p.fileOrNil(), p.first
}

// ParseFile behaves like Parse but it reads the source from the named file.
func ParseFile(filename string, errh ErrorHandler, pragh PragmaHandler, mode Mode) (*File, error) {
	f, err := os.Open(filename)
	if err != nil {
		if errh != nil {
			errh(err)
		}
		return nil, err
	}
	defer f.Close()
	return Parse(NewFileBase(filename), f, errh, pragh, mode)
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/testing.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements testing support.

package syntax

import (
	"io"
	"regexp"
)

// CommentsDo parses the given source and calls the provided handler for each
// comment or error. If the text provided to handler starts with a '/' it is
// the comment text; otherwise it is the error message.
func CommentsDo(src io.Reader, handler func(line, col uint, text string)) {
	var s scanner
	s.init(src, handler, comments)
	for s.tok != _EOF {
		s.next()
	}
}

// CommentMap collects all comments in the given src with comment text
// that matches the supplied regular expression rx and returns them as
// []Error lists in a map indexed by line number. The comment text is
// the comment with any comment markers ("//", "/*", or "*/") stripped.
// The position for each Error is the position of the token immediately
// preceding the comment and the Error message is the comment text,
// with all comments that are on the same line collected in a slice, in
// source order. If there is no preceding token (the matching comment
// appears at the beginning of the file), then the recorded position
// is unknown (line, col = 0, 0). If there are no matching comments,
// the result is nil.
func CommentMap(src io.Reader, rx *regexp.Regexp) (res map[uint][]Error) {
	// position of previous token
	var base *PosBase
	var prev struct{ line, col uint }

	var s scanner
	s.init(src, func(_, _ uint, text string) {
		if text[0] != '/' {
			return // not a comment, ignore
		}
		if text[1] == '*' {
			text = text[:len(text)-2] // strip trailing */
		}
		text = text[2:] // strip leading // or /*
		if rx.MatchString(text) {
			pos := MakePos(base, prev.line, prev.col)
			err := Error{pos, text}
			if res == nil {
				res = make(map[uint][]Error)
			}
			res[prev.line] = append(res[prev.line], err)
		}
	}, comments)

	for s.tok != _EOF {
		s.next()
		if s.tok == _Semi && s.lit != "semicolon" {
			continue // ignore automatically inserted semicolons
		}
		prev.line, prev.col = s.line, s.col
	}

	return
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/token_string.go ===
```go
// Code generated by "stringer -type token -linecomment tokens.go"; DO NOT EDIT.

package syntax

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[_EOF-1]
	_ = x[_Name-2]
	_ = x[_Literal-3]
	_ = x[_Operator-4]
	_ = x[_AssignOp-5]
	_ = x[_IncOp-6]
	_ = x[_Assign-7]
	_ = x[_Define-8]
	_ = x[_Arrow-9]
	_ = x[_Star-10]
	_ = x[_Lparen-11]
	_ = x[_Lbrack-12]
	_ = x[_Lbrace-13]
	_ = x[_Rparen-14]
	_ = x[_Rbrack-15]
	_ = x[_Rbrace-16]
	_ = x[_Comma-17]
	_ = x[_Semi-18]
	_ = x[_Colon-19]
	_ = x[_Dot-20]
	_ = x[_DotDotDot-21]
	_ = x[_Break-22]
	_ = x[_Case-23]
	_ = x[_Chan-24]
	_ = x[_Const-25]
	_ = x[_Continue-26]
	_ = x[_Default-27]
	_ = x[_Defer-28]
	_ = x[_Else-29]
	_ = x[_Fallthrough-30]
	_ = x[_For-31]
	_ = x[_Func-32]
	_ = x[_Go-33]
	_ = x[_Goto-34]
	_ = x[_If-35]
	_ = x[_Import-36]
	_ = x[_Interface-37]
	_ = x[_Map-38]
	_ = x[_Package-39]
	_ = x[_Range-40]
	_ = x[_Return-41]
	_ = x[_Select-42]
	_ = x[_Struct-43]
	_ = x[_Switch-44]
	_ = x[_Type-45]
	_ = x[_Var-46]
	_ = x[tokenCount-47]
}

const _token_name = "EOFnameliteralopop=opop=:=<-*([{)]},;:....breakcasechanconstcontinuedefaultdeferelsefallthroughforfuncgogotoifimportinterfacemappackagerangereturnselectstructswitchtypevar"

var _token_index = [...]uint8{0, 3, 7, 14, 16, 19, 23, 24, 26, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 42, 47, 51, 55, 60, 68, 75, 80, 84, 95, 98, 102, 104, 108, 110, 116, 125, 128, 135, 140, 146, 152, 158, 164, 168, 171, 171}

func (i token) String() string {
	idx := int(i) - 1
	if i < 1 || idx >= len(_token_index)-1 {
		return "token(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _token_name[_token_index[idx]:_token_index[idx+1]]
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/tokens.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

type Token uint

type token = Token

//go:generate stringer -type token -linecomment tokens.go

const (
	_    token = iota
	_EOF       // EOF

	// names and literals
	_Name    // name
	_Literal // literal

	// operators and operations
	// _Operator is excluding '*' (_Star)
	_Operator // op
	_AssignOp // op=
	_IncOp    // opop
	_Assign   // =
	_Define   // :=
	_Arrow    // <-
	_Star     // *

	// delimiters
	_Lparen    // (
	_Lbrack    // [
	_Lbrace    // {
	_Rparen    // )
	_Rbrack    // ]
	_Rbrace    // }
	_Comma     // ,
	_Semi      // ;
	_Colon     // :
	_Dot       // .
	_DotDotDot // ...

	// keywords
	_Break       // break
	_Case        // case
	_Chan        // chan
	_Const       // const
	_Continue    // continue
	_Default     // default
	_Defer       // defer
	_Else        // else
	_Fallthrough // fallthrough
	_For         // for
	_Func        // func
	_Go          // go
	_Goto        // goto
	_If          // if
	_Import      // import
	_Interface   // interface
	_Map         // map
	_Package     // package
	_Range       // range
	_Return      // return
	_Select      // select
	_Struct      // struct
	_Switch      // switch
	_Type        // type
	_Var         // var

	// empty line comment to exclude it from .String
	tokenCount //
)

const (
	// for BranchStmt
	Break       = _Break
	Continue    = _Continue
	Fallthrough = _Fallthrough
	Goto        = _Goto

	// for CallStmt
	Go    = _Go
	Defer = _Defer
)

// Make sure we have at most 64 tokens so we can use them in a set.
const _ uint64 = 1 << (tokenCount - 1)

// contains reports whether tok is in tokset.
func contains(tokset uint64, tok token) bool {
	return tokset&(1<<tok) != 0
}

type LitKind uint8

// TODO(gri) With the 'i' (imaginary) suffix now permitted on integer
// and floating-point numbers, having a single ImagLit does
// not represent the literal kind well anymore. Remove it?
const (
	IntLit LitKind = iota
	FloatLit
	ImagLit
	RuneLit
	StringLit
)

type Operator uint

//go:generate stringer -type Operator -linecomment tokens.go

const (
	_ Operator = iota

	// Def is the : in :=
	Def   // :
	Not   // !
	Recv  // <-
	Tilde // ~

	// precOrOr
	OrOr // ||

	// precAndAnd
	AndAnd // &&

	// precCmp
	Eql // ==
	Neq // !=
	Lss // <
	Leq // <=
	Gtr // >
	Geq // >=

	// precAdd
	Add // +
	Sub // -
	Or  // |
	Xor // ^

	// precMul
	Mul    // *
	Div    // /
	Rem    // %
	And    // &
	AndNot // &^
	Shl    // <<
	Shr    // >>
)

// Operator precedences
const (
	_ = iota
	precOrOr
	precAndAnd
	precCmp
	precAdd
	precMul
)

```

// === FILE: references/go/src/cmd/compile/internal/syntax/type.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import "go/constant"

// A Type represents a type of Go.
// All types implement the Type interface.
// (This type originally lived in types2. We moved it here
// so we could depend on it from other packages without
// introducing an import cycle.)
type Type interface {
	// Underlying returns the underlying type of a type.
	// Underlying types are never Named, TypeParam, or Alias types.
	//
	// See https://go.dev/ref/spec#Underlying_types.
	Underlying() Type

	// String returns a string representation of a type.
	String() string
}

// Expressions in the syntax package provide storage for
// the typechecker to record its results. This interface
// is the mechanism the typechecker uses to record results,
// and clients use to retrieve those results.
type typeInfo interface {
	SetTypeInfo(TypeAndValue)
	GetTypeInfo() TypeAndValue
}

// A TypeAndValue records the type information, constant
// value if known, and various other flags associated with
// an expression.
// This type is similar to types2.TypeAndValue, but exposes
// none of types2's internals.
type TypeAndValue struct {
	Type  Type
	Value constant.Value
	exprFlags
}

type exprFlags uint16

func (f exprFlags) IsVoid() bool          { return f&1 != 0 }
func (f exprFlags) IsType() bool          { return f&2 != 0 }
func (f exprFlags) IsBuiltin() bool       { return f&4 != 0 } // a language builtin that resembles a function call, e.g., "make, append, new"
func (f exprFlags) IsValue() bool         { return f&8 != 0 }
func (f exprFlags) IsNil() bool           { return f&16 != 0 }
func (f exprFlags) Addressable() bool     { return f&32 != 0 }
func (f exprFlags) Assignable() bool      { return f&64 != 0 }
func (f exprFlags) HasOk() bool           { return f&128 != 0 }
func (f exprFlags) IsRuntimeHelper() bool { return f&256 != 0 } // a runtime function called from transformed syntax

func (f *exprFlags) SetIsVoid()          { *f |= 1 }
func (f *exprFlags) SetIsType()          { *f |= 2 }
func (f *exprFlags) SetIsBuiltin()       { *f |= 4 }
func (f *exprFlags) SetIsValue()         { *f |= 8 }
func (f *exprFlags) SetIsNil()           { *f |= 16 }
func (f *exprFlags) SetAddressable()     { *f |= 32 }
func (f *exprFlags) SetAssignable()      { *f |= 64 }
func (f *exprFlags) SetHasOk()           { *f |= 128 }
func (f *exprFlags) SetIsRuntimeHelper() { *f |= 256 }

// a typeAndValue contains the results of typechecking an expression.
// It is embedded in expression nodes.
type typeAndValue struct {
	tv TypeAndValue
}

func (x *typeAndValue) SetTypeInfo(tv TypeAndValue) {
	x.tv = tv
}
func (x *typeAndValue) GetTypeInfo() TypeAndValue {
	return x.tv
}

```

// === FILE: references/go/src/cmd/compile/internal/syntax/walk.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements syntax tree walking.

package syntax

import "fmt"

// Inspect traverses an AST in pre-order: it starts by calling f(root);
// root must not be nil. If f returns true, Inspect invokes f recursively
// for each of the non-nil children of root, followed by a call of f(nil).
//
// See Walk for caveats about shared nodes.
func Inspect(root Node, f func(Node) bool) {
	Walk(root, inspector(f))
}

type inspector func(Node) bool

func (v inspector) Visit(node Node) Visitor {
	if v(node) {
		return v
	}
	return nil
}

// Walk traverses an AST in pre-order: It starts by calling
// v.Visit(node); node must not be nil. If the visitor w returned by
// v.Visit(node) is not nil, Walk is invoked recursively with visitor
// w for each of the non-nil children of node, followed by a call of
// w.Visit(nil).
//
// Some nodes may be shared among multiple parent nodes (e.g., types in
// field lists such as type T in "a, b, c T"). Such shared nodes are
// walked multiple times.
// TODO(gri) Revisit this design. It may make sense to walk those nodes
// only once. A place where this matters is types2.TestResolveIdents.
func Walk(root Node, v Visitor) {
	walker{v}.node(root)
}

// A Visitor's Visit method is invoked for each node encountered by Walk.
// If the result visitor w is not nil, Walk visits each of the children
// of node with the visitor w, followed by a call of w.Visit(nil).
type Visitor interface {
	Visit(node Node) (w Visitor)
}

type walker struct {
	v Visitor
}

func (w walker) node(n Node) {
	if n == nil {
		panic("nil node")
	}

	w.v = w.v.Visit(n)
	if w.v == nil {
		return
	}

	switch n := n.(type) {
	// packages
	case *File:
		w.node(n.PkgName)
		w.declList(n.DeclList)

	// declarations
	case *ImportDecl:
		if n.LocalPkgName != nil {
			w.node(n.LocalPkgName)
		}
		w.node(n.Path)

	case *ConstDecl:
		w.nameList(n.NameList)
		if n.Type != nil {
			w.node(n.Type)
		}
		if n.Values != nil {
			w.node(n.Values)
		}

	case *TypeDecl:
		w.node(n.Name)
		w.fieldList(n.TParamList)
		w.node(n.Type)

	case *VarDecl:
		w.nameList(n.NameList)
		if n.Type != nil {
			w.node(n.Type)
		}
		if n.Values != nil {
			w.node(n.Values)
		}

	case *FuncDecl:
		if n.Recv != nil {
			w.node(n.Recv)
		}
		w.node(n.Name)
		w.fieldList(n.TParamList)
		w.node(n.Type)
		if n.Body != nil {
			w.node(n.Body)
		}

	// expressions
	case *BadExpr: // nothing to do
	case *Name: // nothing to do
	case *BasicLit: // nothing to do

	case *CompositeLit:
		if n.Type != nil {
			w.node(n.Type)
		}
		w.exprList(n.ElemList)

	case *KeyValueExpr:
		w.node(n.Key)
		w.node(n.Value)

	case *FuncLit:
		w.node(n.Type)
		w.node(n.Body)

	case *ParenExpr:
		w.node(n.X)

	case *SelectorExpr:
		w.node(n.X)
		w.node(n.Sel)

	case *IndexExpr:
		w.node(n.X)
		w.node(n.Index)

	case *SliceExpr:
		w.node(n.X)
		for _, x := range n.Index {
			if x != nil {
				w.node(x)
			}
		}

	case *AssertExpr:
		w.node(n.X)
		w.node(n.Type)

	case *TypeSwitchGuard:
		if n.Lhs != nil {
			w.node(n.Lhs)
		}
		w.node(n.X)

	case *Operation:
		w.node(n.X)
		if n.Y != nil {
			w.node(n.Y)
		}

	case *CallExpr:
		w.node(n.Fun)
		w.exprList(n.ArgList)

	case *ListExpr:
		w.exprList(n.ElemList)

	// types
	case *ArrayType:
		if n.Len != nil {
			w.node(n.Len)
		}
		w.node(n.Elem)

	case *SliceType:
		w.node(n.Elem)

	case *DotsType:
		w.node(n.Elem)

	case *StructType:
		w.fieldList(n.FieldList)
		for _, t := range n.TagList {
			if t != nil {
				w.node(t)
			}
		}

	case *Field:
		if n.Name != nil {
			w.node(n.Name)
		}
		w.node(n.Type)

	case *InterfaceType:
		w.fieldList(n.MethodList)

	case *FuncType:
		w.fieldList(n.ParamList)
		w.fieldList(n.ResultList)

	case *MapType:
		w.node(n.Key)
		w.node(n.Value)

	case *ChanType:
		w.node(n.Elem)

	// statements
	case *EmptyStmt: // nothing to do

	case *LabeledStmt:
		w.node(n.Label)
		w.node(n.Stmt)

	case *BlockStmt:
		w.stmtList(n.List)

	case *ExprStmt:
		w.node(n.X)

	case *SendStmt:
		w.node(n.Chan)
		w.node(n.Value)

	case *DeclStmt:
		w.declList(n.DeclList)

	case *AssignStmt:
		w.node(n.Lhs)
		if n.Rhs != nil {
			w.node(n.Rhs)
		}

	case *BranchStmt:
		if n.Label != nil {
			w.node(n.Label)
		}
		// Target points to nodes elsewhere in the syntax tree

	case *CallStmt:
		w.node(n.Call)

	case *ReturnStmt:
		if n.Results != nil {
			w.node(n.Results)
		}

	case *IfStmt:
		if n.Init != nil {
			w.node(n.Init)
		}
		w.node(n.Cond)
		w.node(n.Then)
		if n.Else != nil {
			w.node(n.Else)
		}

	case *ForStmt:
		if n.Init != nil {
			w.node(n.Init)
		}
		if n.Cond != nil {
			w.node(n.Cond)
		}
		if n.Post != nil {
			w.node(n.Post)
		}
		w.node(n.Body)

	case *SwitchStmt:
		if n.Init != nil {
			w.node(n.Init)
		}
		if n.Tag != nil {
			w.node(n.Tag)
		}
		for _, s := range n.Body {
			w.node(s)
		}

	case *SelectStmt:
		for _, s := range n.Body {
			w.node(s)
		}

	// helper nodes
	case *RangeClause:
		if n.Lhs != nil {
			w.node(n.Lhs)
		}
		w.node(n.X)

	case *CaseClause:
		if n.Cases != nil {
			w.node(n.Cases)
		}
		w.stmtList(n.Body)

	case *CommClause:
		if n.Comm != nil {
			w.node(n.Comm)
		}
		w.stmtList(n.Body)

	default:
		panic(fmt.Sprintf("internal error: unknown node type %T", n))
	}

	w.v.Visit(nil)
}

func (w walker) declList(list []Decl) {
	for _, n := range list {
		w.node(n)
	}
}

func (w walker) exprList(list []Expr) {
	for _, n := range list {
		w.node(n)
	}
}

func (w walker) stmtList(list []Stmt) {
	for _, n := range list {
		w.node(n)
	}
}

func (w walker) nameList(list []*Name) {
	for _, n := range list {
		w.node(n)
	}
}

func (w walker) fieldList(list []*Field) {
	for _, n := range list {
		w.node(n)
	}
}

```

