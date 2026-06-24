# Domain Architecture: errors

## Layout Topology
```text
errors/
├── errors.go
├── join.go
└── wrap.go
```

## Source Stream Aggregation

// === FILE: references/go/src/errors/errors.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errors implements functions to manipulate errors.
//
// The [New] function creates errors whose only content is a text message.
//
// An error e wraps another error if e's type has one of the methods
//
//	Unwrap() error
//	Unwrap() []error
//
// If e.Unwrap() returns a non-nil error w or a slice containing w,
// then we say that e wraps w. A nil error returned from e.Unwrap()
// indicates that e does not wrap any error. It is invalid for an
// Unwrap method to return an []error containing a nil error value.
//
// An easy way to create wrapped errors is to call [fmt.Errorf] and apply
// the %w verb to the error argument:
//
//	wrapsErr := fmt.Errorf("... %w ...", ..., err, ...)
//
// Successive unwrapping of an error creates a tree. The [Is] and [As]
// functions inspect an error's tree by examining first the error
// itself followed by the tree of each of its children in turn
// (pre-order, depth-first traversal).
//
// See https://go.dev/blog/go1.13-errors for a deeper discussion of the
// philosophy of wrapping and when to wrap.
//
// [Is] examines the tree of its first argument looking for an error that
// matches the second. It reports whether it finds a match. It should be
// used in preference to simple equality checks:
//
//	if errors.Is(err, fs.ErrExist)
//
// is preferable to
//
//	if err == fs.ErrExist
//
// because the former will succeed if err wraps [io/fs.ErrExist].
//
// [AsType] examines the tree of its argument looking for an error whose
// type matches its type argument. If it succeeds, it returns the
// corresponding value of that type and true. Otherwise, it returns the
// zero value of that type and false. The form
//
//	if perr, ok := errors.AsType[*fs.PathError](err); ok {
//		fmt.Println(perr.Path)
//	}
//
// is preferable to
//
//	if perr, ok := err.(*fs.PathError); ok {
//		fmt.Println(perr.Path)
//	}
//
// because the former will succeed if err wraps an [*io/fs.PathError].
package errors

// New returns an error that formats as the given text.
// Each call to New returns a distinct error value even if the text is identical.
func New(text string) error {
	return &errorString{text}
}

// errorString is a trivial implementation of error.
type errorString struct {
	s string
}

func (e *errorString) Error() string {
	return e.s
}

// ErrUnsupported indicates that a requested operation cannot be performed,
// because it is unsupported. For example, a call to [os.Link] when using a
// file system that does not support hard links.
//
// Functions and methods should not return this error but should instead
// return an error including appropriate context that satisfies
//
//	errors.Is(err, errors.ErrUnsupported)
//
// either by directly wrapping ErrUnsupported or by implementing an [Is] method.
//
// Functions and methods should document the cases in which an error
// wrapping this will be returned.
var ErrUnsupported = New("unsupported operation")

```

// === FILE: references/go/src/errors/join.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"unsafe"
)

// Join returns an error that wraps the given errors.
// Any nil error values are discarded.
// Join returns nil if every value in errs is nil.
// The error formats as the concatenation of the strings obtained
// by calling the Error method of each element of errs, with a newline
// between each string.
//
// A non-nil error returned by Join implements the Unwrap() []error method.
// The errors may be inspected with [Is] and [As].
func Join(errs ...error) error {
	n := 0
	for _, err := range errs {
		if err != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	e := &joinError{
		errs: make([]error, 0, n),
	}
	for _, err := range errs {
		if err != nil {
			e.errs = append(e.errs, err)
		}
	}
	return e
}

type joinError struct {
	errs []error
}

func (e *joinError) Error() string {
	// Since Join returns nil if every value in errs is nil,
	// e.errs cannot be empty.
	if len(e.errs) == 1 {
		return e.errs[0].Error()
	}

	b := []byte(e.errs[0].Error())
	for _, err := range e.errs[1:] {
		b = append(b, '\n')
		b = append(b, err.Error()...)
	}
	// At this point, b has at least one byte '\n'.
	return unsafe.String(&b[0], len(b))
}

func (e *joinError) Unwrap() []error {
	return e.errs
}

```

// === FILE: references/go/src/errors/wrap.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"internal/reflectlite"
)

// Unwrap returns the result of calling the Unwrap method on err, if err's
// type contains an Unwrap method returning error.
// Otherwise, Unwrap returns nil.
//
// Unwrap only calls a method of the form "Unwrap() error".
// In particular Unwrap does not unwrap errors returned by [Join].
func Unwrap(err error) error {
	u, ok := err.(interface {
		Unwrap() error
	})
	if !ok {
		return nil
	}
	return u.Unwrap()
}

// Is reports whether any error in err's tree matches target.
// The target must be comparable.
//
// The tree consists of err itself, followed by the errors obtained by repeatedly
// calling its Unwrap() error or Unwrap() []error method. When err wraps multiple
// errors, Is examines err followed by a depth-first traversal of its children.
//
// An error is considered to match a target if it is equal to that target or if
// it implements a method Is(error) bool such that Is(target) returns true.
//
// An error type might provide an Is method so it can be treated as equivalent
// to an existing error. For example, if MyError defines
//
//	func (m MyError) Is(target error) bool { return target == fs.ErrExist }
//
// then Is(MyError{}, fs.ErrExist) returns true. See [syscall.Errno.Is] for
// an example in the standard library. An Is method should only shallowly
// compare err and the target and not call [Unwrap] on either.
func Is(err, target error) bool {
	if err == nil || target == nil {
		return err == target
	}

	isComparable := reflectlite.TypeOf(target).Comparable()
	return is(err, target, isComparable)
}

func is(err, target error, targetComparable bool) bool {
	for {
		if targetComparable && err == target {
			return true
		}
		if x, ok := err.(interface{ Is(error) bool }); ok && x.Is(target) {
			return true
		}
		switch x := err.(type) {
		case interface{ Unwrap() error }:
			err = x.Unwrap()
			if err == nil {
				return false
			}
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if is(err, target, targetComparable) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}
}

// As finds the first error in err's tree that matches target, and if one is found, sets
// target to that error value and returns true. Otherwise, it returns false.
//
// For most uses, prefer [AsType]. As is equivalent to [AsType] but sets its target
// argument rather than returning the matching error and doesn't require its target
// argument to implement error.
//
// The tree consists of err itself, followed by the errors obtained by repeatedly
// calling its Unwrap() error or Unwrap() []error method. When err wraps multiple
// errors, As examines err followed by a depth-first traversal of its children.
//
// An error matches target if the error's concrete value is assignable to the value
// pointed to by target, or if the error has a method As(any) bool such that
// As(target) returns true. In the latter case, the As method is responsible for
// setting target.
//
// An error type might provide an As method so it can be treated as if it were a
// different error type.
//
// As panics if target is not a non-nil pointer to either a type that implements
// error, or to any interface type.
func As(err error, target any) bool {
	if err == nil {
		return false
	}
	if target == nil {
		panic("errors: target cannot be nil")
	}
	val := reflectlite.ValueOf(target)
	typ := val.Type()
	if typ.Kind() != reflectlite.Ptr || val.IsNil() {
		panic("errors: target must be a non-nil pointer")
	}
	targetType := typ.Elem()
	if targetType.Kind() != reflectlite.Interface && !targetType.Implements(errorType) {
		panic("errors: *target must be interface or implement error")
	}
	return as(err, target, val, targetType)
}

func as(err error, target any, targetVal reflectlite.Value, targetType reflectlite.Type) bool {
	for {
		if reflectlite.TypeOf(err).AssignableTo(targetType) {
			targetVal.Elem().Set(reflectlite.ValueOf(err))
			return true
		}
		if x, ok := err.(interface{ As(any) bool }); ok && x.As(target) {
			return true
		}
		switch x := err.(type) {
		case interface{ Unwrap() error }:
			err = x.Unwrap()
			if err == nil {
				return false
			}
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if err == nil {
					continue
				}
				if as(err, target, targetVal, targetType) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}
}

var errorType = reflectlite.TypeOf((*error)(nil)).Elem()

// AsType finds the first error in err's tree that matches the type E, and
// if one is found, returns that error value and true. Otherwise, it
// returns the zero value of E and false.
//
// The tree consists of err itself, followed by the errors obtained by
// repeatedly calling its Unwrap() error or Unwrap() []error method. When
// err wraps multiple errors, AsType examines err followed by a
// depth-first traversal of its children.
//
// An error err matches the type E if the type assertion err.(E) holds,
// or if the error has a method As(any) bool such that err.As(target)
// returns true when target is a non-nil *E. In the latter case, the As
// method is responsible for setting target.
func AsType[E error](err error) (E, bool) {
	if err == nil {
		var zero E
		return zero, false
	}
	var pe *E // lazily initialized
	return asType(err, &pe)
}

func asType[E error](err error, ppe **E) (_ E, _ bool) {
	for {
		if e, ok := err.(E); ok {
			return e, true
		}
		if x, ok := err.(interface{ As(any) bool }); ok {
			if *ppe == nil {
				*ppe = new(E)
			}
			if x.As(*ppe) {
				return **ppe, true
			}
		}
		switch x := err.(type) {
		case interface{ Unwrap() error }:
			err = x.Unwrap()
			if err == nil {
				return
			}
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if err == nil {
					continue
				}
				if x, ok := asType(err, ppe); ok {
					return x, true
				}
			}
			return
		default:
			return
		}
	}
}

```

