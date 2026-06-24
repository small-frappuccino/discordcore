# Domain Architecture: cmd/vet

## Layout Topology
```text
cmd/vet/
├── README
├── doc.go
└── main.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/vet/doc.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Vet examines Go source code and reports suspicious constructs, such as Printf
calls whose arguments do not align with the format string. Vet uses heuristics
that do not guarantee all reports are genuine problems, but it can find errors
not caught by the compilers.

Vet is normally invoked through the go command.
This command vets the package in the current directory:

	go vet

whereas this one vets the packages whose path is provided:

	go vet my/project/...

Use "go help packages" to see other ways of specifying which packages to vet.

Vet's exit code is non-zero for erroneous invocation of the tool or if a
problem was reported, and 0 otherwise. Note that the tool does not
check every possible problem and depends on unreliable heuristics,
so it should be used as guidance only, not as a firm indicator of
program correctness.

To list the available checks, run "go tool vet help":

	appends          check for missing values after append
	asmdecl          report mismatches between assembly files and Go declarations
	assign           check for useless assignments
	atomic           check for common mistakes using the sync/atomic package
	bools            check for common mistakes involving boolean operators
	buildtag         check //go:build and // +build directives
	cgocall          detect some violations of the cgo pointer passing rules
	composites       check for unkeyed composite literals
	copylocks        check for locks erroneously passed by value
	defers           report common mistakes in defer statements
	directive        check Go toolchain directives such as //go:debug
	errorsas         report passing non-pointer or non-error values to errors.As
	framepointer     report assembly that clobbers the frame pointer before saving it
	hostport         check format of addresses passed to net.Dial
	httpresponse     check for mistakes using HTTP responses
	ifaceassert      detect impossible interface-to-interface type assertions
	loopclosure      check references to loop variables from within nested functions
	lostcancel       check cancel func returned by context.WithCancel is called
	nilfunc          check for useless comparisons between functions and nil
	printf           check consistency of Printf format strings and arguments
	shift            check for shifts that equal or exceed the width of the integer
	sigchanyzer      check for unbuffered channel of os.Signal
	slog             check for invalid structured logging calls
	stdmethods       check signature of methods of well-known interfaces
	stdversion       report uses of too-new standard library symbols
	stringintconv    check for string(int) conversions
	structtag        check that struct field tags conform to reflect.StructTag.Get
	testinggoroutine report calls to (*testing.T).Fatal from goroutines started by a test
	tests            check for common mistaken usages of tests and examples
	timeformat       check for calls of (time.Time).Format or time.Parse with 2006-02-01
	unmarshal        report passing non-pointer or non-interface values to unmarshal
	unreachable      check for unreachable code
	unsafeptr        check for invalid conversions of uintptr to unsafe.Pointer
	unusedresult     check for unused results of calls to some functions
	waitgroup        check for misuses of sync.WaitGroup

For details and flags of a particular check, such as printf, run "go tool vet help printf".

By default, all checks are performed.
If any flags are explicitly set to true, only those tests are run.
Conversely, if any flag is explicitly set to false, only those tests are disabled.
Thus -printf=true runs the printf check,
and -printf=false runs all checks except the printf check.

For information on writing a new check, see golang.org/x/tools/go/analysis.

Core flags:

	-c=N
	  	display offending line plus N lines of surrounding context
	-json
	  	emit analysis diagnostics (and errors) in JSON format
*/
package main

```

// === FILE: references/go/src/cmd/vet/main.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"cmd/internal/objabi"
	"cmd/internal/telemetry/counter"

	"golang.org/x/tools/go/analysis/suite/vet"
	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() {
	// Keep consistent with cmd/fix/main.go!
	counter.Open()
	objabi.AddVersionFlag()
	counter.Inc("vet/invocations")

	unitchecker.Main(vet.Suite...) // (never returns)
}

```

// === FILE: references/go/src/cmd/vet/README ===
```text
Vet is a tool that checks correctness of Go programs. It runs a suite of tests,
each tailored to check for a particular class of errors. Examples include incorrect
Printf format verbs and malformed build tags.

Over time many checks have been added to vet's suite, but many more have been
rejected as not appropriate for the tool. The criteria applied when selecting which
checks to add are:

Correctness:

Vet's checks are about correctness, not style. A vet check must identify real or
potential bugs that could cause incorrect compilation or execution. A check that
only identifies stylistic points or alternative correct approaches to a situation
is not acceptable.

Frequency:

Vet is run every day by many programmers, often as part of every compilation or
submission. The cost in execution time is considerable, especially in aggregate,
so checks must be likely enough to find real problems that they are worth the
overhead of the added check. A new check that finds only a handful of problems
across all existing programs, even if the problem is significant, is not worth
adding to the suite everyone runs daily.

Precision:

Most of vet's checks are heuristic and can generate both false positives (flagging
correct programs) and false negatives (not flagging incorrect ones). The rate of
both these failures must be very small. A check that is too noisy will be ignored
by the programmer overwhelmed by the output; a check that misses too many of the
cases it's looking for will give a false sense of security. Neither is acceptable.
A vet check must be accurate enough that everything it reports is worth examining,
and complete enough to encourage real confidence.

```

