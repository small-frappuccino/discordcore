# Domain Architecture: cmd/gofmt

## Layout Topology
```text
cmd/gofmt/
├── doc.go
├── gofmt.go
├── internal.go
├── rewrite.go
└── simplify.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/cmd/gofmt/doc.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Gofmt formats Go programs.
It uses tabs for indentation and blanks for alignment.
Alignment assumes that an editor is using a fixed-width font.

Without an explicit path, it processes the standard input.  Given a file,
it operates on that file; given a directory, it operates on all .go files in
that directory, recursively.  (Files starting with a period are ignored.)
By default, gofmt prints the reformatted sources to standard output.

Usage:

	gofmt [flags] [path ...]

The flags are:

	-d
		Do not print reformatted sources to standard output.
		If a file's formatting is different than gofmt's, print diffs
		to standard output.
	-e
		Print all (including spurious) errors.
	-l
		Do not print reformatted sources to standard output.
		If a file's formatting is different from gofmt's, print its name
		to standard output.
	-r rule
		Apply the rewrite rule to the source before reformatting.
	-s
		Try to simplify code (after applying the rewrite rule, if any).
	-w
		Do not print reformatted sources to standard output.
		If a file's formatting is different from gofmt's, overwrite it
		with gofmt's version. If an error occurred during overwriting,
		the original file is restored from an automatic backup.

Debugging support:

	-cpuprofile filename
		Write cpu profile to the specified file.

The rewrite rule specified with the -r flag must be a string of the form:

	pattern -> replacement

Both pattern and replacement must be valid Go expressions.
In the pattern, single-character lowercase identifiers serve as
wildcards matching arbitrary sub-expressions; those expressions
will be substituted for the same identifiers in the replacement.

When gofmt reads from standard input, it accepts either a full Go program
or a program fragment.  A program fragment must be a syntactically
valid declaration list, statement list, or expression.  When formatting
such a fragment, gofmt preserves leading indentation as well as leading
and trailing spaces, so that individual sections of a Go program can be
formatted by piping them through gofmt.

# Examples

To check files for unnecessary parentheses:

	gofmt -r '(a) -> a' -l *.go

To remove the parentheses:

	gofmt -r '(a) -> a' -w *.go

To convert the package tree from explicit slice upper bounds to implicit ones:

	gofmt -r 'α[β:len(α)] -> α[β:]' -w $GOROOT/src

# The simplify command

When invoked with -s gofmt will make the following source transformations where possible.

	An array, slice, or map composite literal of the form:
		[]T{T{}, T{}}
	will be simplified to:
		[]T{{}, {}}

	A slice expression of the form:
		s[a:len(s)]
	will be simplified to:
		s[a:]

	A range of the form:
		for x, _ = range v {...}
	will be simplified to:
		for x = range v {...}

	A range of the form:
		for _ = range v {...}
	will be simplified to:
		for range v {...}

This may result in changes that are incompatible with earlier versions of Go.
*/
package main

// BUG(rsc): The implementation of -r is a bit slow.
// BUG(gri): If -w fails, the restored original file may not have some of the
// original file attributes.

```

// === FILE: references!/go/src/cmd/gofmt/gofmt.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"internal/diff"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"

	"cmd/internal/telemetry/counter"

	"golang.org/x/sync/semaphore"
)

var (
	// main operation modes
	list        = flag.Bool("l", false, "list files whose formatting differs from gofmt's")
	write       = flag.Bool("w", false, "write result to (source) file instead of stdout")
	rewriteRule = flag.String("r", "", "rewrite rule (e.g., 'a[b:len(a)] -> a[b:]')")
	simplifyAST = flag.Bool("s", false, "simplify code")
	doDiff      = flag.Bool("d", false, "display diffs instead of rewriting files")
	allErrors   = flag.Bool("e", false, "report all errors (not just the first 10 on different lines)")

	// debugging
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to this file")

	// errors
	errFormattingDiffers = fmt.Errorf("formatting differs from gofmt's")
)

// Keep these in sync with go/format/format.go.
const (
	tabWidth    = 8
	printerMode = printer.UseSpaces | printer.TabIndent | printerNormalizeNumbers

	// printerNormalizeNumbers means to canonicalize number literal prefixes
	// and exponents while printing. See https://golang.org/doc/go1.13#gofmt.
	//
	// This value is defined in go/printer specifically for go/format and cmd/gofmt.
	printerNormalizeNumbers = 1 << 30
)

// fdSem guards the number of concurrently-open file descriptors.
//
// For now, this is arbitrarily set to 200, based on the observation that many
// platforms default to a kernel limit of 256. Ideally, perhaps we should derive
// it from rlimit on platforms that support that system call.
//
// File descriptors opened from outside of this package are not tracked,
// so this limit may be approximate.
var fdSem = make(chan bool, 200)

var (
	rewrite    func(*token.FileSet, *ast.File) *ast.File
	parserMode parser.Mode
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: gofmt [flags] [path ...]\n")
	flag.PrintDefaults()
}

func initParserMode() {
	parserMode = parser.ParseComments | parser.SkipObjectResolution
	if *allErrors {
		parserMode |= parser.AllErrors
	}
	// It's only -r that makes use of go/ast's object resolution,
	// so avoid the unnecessary work if the flag isn't used.
	if *rewriteRule == "" {
		parserMode |= parser.SkipObjectResolution
	}
}

func isGoFilename(name string) bool {
	return !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
}

// A sequencer performs concurrent tasks that may write output, but emits that
// output in a deterministic order.
type sequencer struct {
	maxWeight int64
	sem       *semaphore.Weighted   // weighted by input bytes (an approximate proxy for memory overhead)
	prev      <-chan *reporterState // 1-buffered
}

// newSequencer returns a sequencer that allows concurrent tasks up to maxWeight
// and writes tasks' output to out and err.
func newSequencer(maxWeight int64, out, err io.Writer) *sequencer {
	sem := semaphore.NewWeighted(maxWeight)
	prev := make(chan *reporterState, 1)
	prev <- &reporterState{out: out, err: err}
	return &sequencer{
		maxWeight: maxWeight,
		sem:       sem,
		prev:      prev,
	}
}

// exclusive is a weight that can be passed to a sequencer to cause
// a task to be executed without any other concurrent tasks.
const exclusive = -1

// Add blocks until the sequencer has enough weight to spare, then adds f as a
// task to be executed concurrently.
//
// If the weight is either negative or larger than the sequencer's maximum
// weight, Add blocks until all other tasks have completed, then the task
// executes exclusively (blocking all other calls to Add until it completes).
//
// f may run concurrently in a goroutine, but its output to the passed-in
// reporter will be sequential relative to the other tasks in the sequencer.
//
// If f invokes a method on the reporter, execution of that method may block
// until the previous task has finished. (To maximize concurrency, f should
// avoid invoking the reporter until it has finished any parallelizable work.)
//
// If f returns a non-nil error, that error will be reported after f's output
// (if any) and will cause a nonzero final exit code.
func (s *sequencer) Add(weight int64, f func(*reporter) error) {
	if weight < 0 || weight > s.maxWeight {
		weight = s.maxWeight
	}
	if err := s.sem.Acquire(context.TODO(), weight); err != nil {
		// Change the task from "execute f" to "report err".
		weight = 0
		f = func(*reporter) error { return err }
	}

	r := &reporter{prev: s.prev}
	next := make(chan *reporterState, 1)
	s.prev = next

	// Start f in parallel: it can run until it invokes a method on r, at which
	// point it will block until the previous task releases the output state.
	go func() {
		if err := f(r); err != nil {
			r.Report(err)
		}
		next <- r.getState() // Release the next task.
		s.sem.Release(weight)
	}()
}

// AddReport prints an error to s after the output of any previously-added
// tasks, causing the final exit code to be nonzero.
func (s *sequencer) AddReport(err error) {
	s.Add(0, func(*reporter) error { return err })
}

// GetExitCode waits for all previously-added tasks to complete, then returns an
// exit code for the sequence suitable for passing to os.Exit.
func (s *sequencer) GetExitCode() int {
	c := make(chan int, 1)
	s.Add(0, func(r *reporter) error {
		c <- r.ExitCode()
		return nil
	})
	return <-c
}

// A reporter reports output, warnings, and errors.
type reporter struct {
	prev  <-chan *reporterState
	state *reporterState
}

// reporterState carries the state of a reporter instance.
//
// Only one reporter at a time may have access to a reporterState.
type reporterState struct {
	out, err io.Writer
	exitCode int
}

// getState blocks until any prior reporters are finished with the reporter
// state, then returns the state for manipulation.
func (r *reporter) getState() *reporterState {
	if r.state == nil {
		r.state = <-r.prev
	}
	return r.state
}

// Warnf emits a warning message to the reporter's error stream,
// without changing its exit code.
func (r *reporter) Warnf(format string, args ...any) {
	fmt.Fprintf(r.getState().err, format, args...)
}

// Write emits a slice to the reporter's output stream.
//
// Any error is returned to the caller, and does not otherwise affect the
// reporter's exit code.
func (r *reporter) Write(p []byte) (int, error) {
	return r.getState().out.Write(p)
}

// Report emits a non-nil error to the reporter's error stream,
// changing its exit code to a nonzero value.
func (r *reporter) Report(err error) {
	if err == nil {
		panic("Report with nil error")
	}
	st := r.getState()
	if err == errFormattingDiffers {
		st.exitCode = 1
	} else {
		scanner.PrintError(st.err, err)
		st.exitCode = 2
	}
}

func (r *reporter) ExitCode() int {
	return r.getState().exitCode
}

// If info == nil, we are formatting stdin instead of a file.
// If in == nil, the source is the contents of the file with the given filename.
func processFile(filename string, info fs.FileInfo, in io.Reader, r *reporter) error {
	src, err := readFile(filename, info, in)
	if err != nil {
		return err
	}

	fileSet := token.NewFileSet()
	// If we are formatting stdin, we accept a program fragment in lieu of a
	// complete source file.
	fragmentOk := info == nil
	file, sourceAdj, indentAdj, err := parse(fileSet, filename, src, fragmentOk)
	if err != nil {
		return err
	}

	if rewrite != nil {
		if sourceAdj == nil {
			file = rewrite(fileSet, file)
		} else {
			r.Warnf("warning: rewrite ignored for incomplete programs\n")
		}
	}

	ast.SortImports(fileSet, file)

	if *simplifyAST {
		simplify(file)
	}

	res, err := format(fileSet, file, sourceAdj, indentAdj, src, printer.Config{Mode: printerMode, Tabwidth: tabWidth})
	if err != nil {
		return err
	}

	if !bytes.Equal(src, res) {
		// formatting has changed
		if *list {
			fmt.Fprintln(r, filename)
		}
		if *write {
			if info == nil {
				panic("-w should not have been allowed with stdin")
			}

			perm := info.Mode().Perm()
			if err := writeFile(filename, src, res, perm, info.Size()); err != nil {
				return err
			}
		}
		if *doDiff {
			newName := filepath.ToSlash(filename)
			oldName := newName + ".orig"
			r.Write(diff.Diff(oldName, src, newName, res))
			return errFormattingDiffers
		}
	}

	if !*list && !*write && !*doDiff {
		_, err = r.Write(res)
	}

	return err
}

// readFile reads the contents of filename, described by info.
// If in is non-nil, readFile reads directly from it.
// Otherwise, readFile opens and reads the file itself,
// with the number of concurrently-open files limited by fdSem.
func readFile(filename string, info fs.FileInfo, in io.Reader) ([]byte, error) {
	if in == nil {
		fdSem <- true
		var err error
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		in = f
		defer func() {
			f.Close()
			<-fdSem
		}()
	}

	// Compute the file's size and read its contents with minimal allocations.
	//
	// If we have the FileInfo from filepath.WalkDir, use it to make
	// a buffer of the right size and avoid ReadAll's reallocations.
	//
	// If the size is unknown (or bogus, or overflows an int), fall back to
	// a size-independent ReadAll.
	size := -1
	if info != nil && info.Mode().IsRegular() && int64(int(info.Size())) == info.Size() {
		size = int(info.Size())
	}
	if size+1 <= 0 {
		// The file is not known to be regular, so we don't have a reliable size for it.
		var err error
		src, err := io.ReadAll(in)
		if err != nil {
			return nil, err
		}
		return src, nil
	}

	// We try to read size+1 bytes so that we can detect modifications: if we
	// read more than size bytes, then the file was modified concurrently.
	// (If that happens, we could, say, append to src to finish the read, or
	// proceed with a truncated buffer — but the fact that it changed at all
	// indicates a possible race with someone editing the file, so we prefer to
	// stop to avoid corrupting it.)
	src := make([]byte, size+1)
	n, err := io.ReadFull(in, src)
	switch err {
	case nil, io.EOF, io.ErrUnexpectedEOF:
		// io.ReadFull returns io.EOF (for an empty file) or io.ErrUnexpectedEOF
		// (for a non-empty file) if the file was changed unexpectedly. Continue
		// with comparing file sizes in those cases.
	default:
		return nil, err
	}
	if n < size {
		return nil, fmt.Errorf("error: size of %s changed during reading (from %d to %d bytes)", filename, size, n)
	} else if n > size {
		return nil, fmt.Errorf("error: size of %s changed during reading (from %d to >=%d bytes)", filename, size, len(src))
	}
	return src[:n], nil
}

func main() {
	// Arbitrarily limit in-flight work to 2MiB times the number of threads.
	//
	// The actual overhead for the parse tree and output will depend on the
	// specifics of the file, but this at least keeps the footprint of the process
	// roughly proportional to GOMAXPROCS.
	maxWeight := (2 << 20) * int64(runtime.GOMAXPROCS(0))
	s := newSequencer(maxWeight, os.Stdout, os.Stderr)

	// call gofmtMain in a separate function
	// so that it can use defer and have them
	// run before the exit.
	gofmtMain(s)
	os.Exit(s.GetExitCode())
}

func gofmtMain(s *sequencer) {
	counter.Open()
	flag.Usage = usage
	flag.Parse()
	counter.Inc("gofmt/invocations")
	counter.CountFlags("gofmt/flag:", *flag.CommandLine)

	if *cpuprofile != "" {
		fdSem <- true
		f, err := os.Create(*cpuprofile)
		if err != nil {
			s.AddReport(fmt.Errorf("creating cpu profile: %s", err))
			return
		}
		defer func() {
			f.Close()
			<-fdSem
		}()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	initParserMode()
	initRewrite()

	args := flag.Args()
	if len(args) == 0 {
		if *write {
			s.AddReport(fmt.Errorf("error: cannot use -w with standard input"))
			return
		}
		s.Add(0, func(r *reporter) error {
			return processFile("<standard input>", nil, os.Stdin, r)
		})
		return
	}

	for _, arg := range args {
		// Walk each given argument as a directory tree.
		// If the argument is not a directory, it's always formatted as a Go file.
		// If the argument is a directory, we walk it, ignoring non-Go files.
		if err := filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			case d.IsDir():
				return nil // simply recurse into directories
			case path == arg:
				// non-directories given as explicit arguments are always formatted
			case !isGoFilename(d.Name()):
				return nil // skip walked non-Go files
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			s.Add(fileWeight(path, info), func(r *reporter) error {
				return processFile(path, info, nil, r)
			})
			return nil
		}); err != nil {
			s.AddReport(err)
		}
	}
}

func fileWeight(path string, info fs.FileInfo) int64 {
	if info == nil {
		return exclusive
	}
	if info.Mode().Type() == fs.ModeSymlink {
		var err error
		info, err = os.Stat(path)
		if err != nil {
			return exclusive
		}
	}
	if !info.Mode().IsRegular() {
		// For non-regular files, FileInfo.Size is system-dependent and thus not a
		// reliable indicator of weight.
		return exclusive
	}
	return info.Size()
}

// writeFile updates a file with the new formatted data.
func writeFile(filename string, orig, formatted []byte, perm fs.FileMode, size int64) error {
	// Make a temporary backup file before rewriting the original file.
	bakname, err := backupFile(filename, orig, perm)
	if err != nil {
		return err
	}

	fdSem <- true
	defer func() { <-fdSem }()

	fout, err := os.OpenFile(filename, os.O_WRONLY, perm)
	if err != nil {
		// We couldn't even open the file, so it should
		// not have changed.
		os.Remove(bakname)
		return err
	}
	defer fout.Close() // for error paths

	restoreFail := func(err error) {
		fmt.Fprintf(os.Stderr, "gofmt: %s: error restoring file to original: %v; backup in %s\n", filename, err, bakname)
	}

	n, err := fout.Write(formatted)
	if err == nil && int64(n) < size {
		err = fout.Truncate(int64(n))
	}

	if err != nil {
		// Rewriting the file failed.

		if n == 0 {
			// Original file unchanged.
			os.Remove(bakname)
			return err
		}

		// Try to restore the original contents.

		no, erro := fout.WriteAt(orig, 0)
		if erro != nil {
			// That failed too.
			restoreFail(erro)
			return err
		}

		if no < n {
			// Original file is shorter. Truncate.
			if erro = fout.Truncate(int64(no)); erro != nil {
				restoreFail(erro)
				return err
			}
		}

		if erro := fout.Close(); erro != nil {
			restoreFail(erro)
			return err
		}

		// Original contents restored.
		os.Remove(bakname)
		return err
	}

	if err := fout.Close(); err != nil {
		restoreFail(err)
		return err
	}

	// File updated.
	os.Remove(bakname)
	return nil
}

// backupFile writes data to a new file named filename<number> with permissions perm,
// with <number> randomly chosen such that the file name is unique. backupFile returns
// the chosen file name.
func backupFile(filename string, data []byte, perm fs.FileMode) (string, error) {
	fdSem <- true
	defer func() { <-fdSem }()

	nextRandom := func() string {
		return strconv.Itoa(rand.Int())
	}

	dir, base := filepath.Split(filename)
	var (
		bakname string
		f       *os.File
	)
	for {
		bakname = filepath.Join(dir, base+"."+nextRandom())
		var err error
		f, err = os.OpenFile(bakname, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return "", err
		}
	}

	// write data to backup file
	_, err := f.Write(data)
	if err1 := f.Close(); err == nil {
		err = err1
	}

	return bakname, err
}

```

// === FILE: references!/go/src/cmd/gofmt/internal.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(gri): This file and the file src/go/format/internal.go are
// the same (but for this comment and the package name). Do not modify
// one without the other. Determine if we can factor out functionality
// in a public API. See also #11844 for context.

package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
)

// parse parses src, which was read from the named file,
// as a Go source file, declaration, or statement list.
func parse(fset *token.FileSet, filename string, src []byte, fragmentOk bool) (
	file *ast.File,
	sourceAdj func(src []byte, indent int) []byte,
	indentAdj int,
	err error,
) {
	// Try as whole source file.
	file, err = parser.ParseFile(fset, filename, src, parserMode)
	// If there's no error, return. If the error is that the source file didn't begin with a
	// package line and source fragments are ok, fall through to
	// try as a source fragment. Stop and return on any other error.
	if err == nil || !fragmentOk || !strings.Contains(err.Error(), "expected 'package'") {
		return
	}

	// If this is a declaration list, make it a source file
	// by inserting a package clause.
	// Insert using a ';', not a newline, so that the line numbers
	// in psrc match the ones in src.
	psrc := append([]byte("package p;"), src...)
	file, err = parser.ParseFile(fset, filename, psrc, parserMode)
	if err == nil {
		sourceAdj = func(src []byte, indent int) []byte {
			// Remove the package clause.
			// Gofmt has turned the ';' into a '\n'.
			src = src[indent+len("package p\n"):]
			return bytes.TrimSpace(src)
		}
		return
	}
	// If the error is that the source file didn't begin with a
	// declaration, fall through to try as a statement list.
	// Stop and return on any other error.
	if !strings.Contains(err.Error(), "expected declaration") {
		return
	}

	// If this is a statement list, make it a source file
	// by inserting a package clause and turning the list
	// into a function body. This handles expressions too.
	// Insert using a ';', not a newline, so that the line numbers
	// in fsrc match the ones in src. Add an extra '\n' before the '}'
	// to make sure comments are flushed before the '}'.
	fsrc := append(append([]byte("package p; func _() {"), src...), '\n', '\n', '}')
	file, err = parser.ParseFile(fset, filename, fsrc, parserMode)
	if err == nil {
		sourceAdj = func(src []byte, indent int) []byte {
			// Cap adjusted indent to zero.
			if indent < 0 {
				indent = 0
			}
			// Remove the wrapping.
			// Gofmt has turned the "; " into a "\n\n".
			// There will be two non-blank lines with indent, hence 2*indent.
			src = src[2*indent+len("package p\n\nfunc _() {"):]
			// Remove only the "}\n" suffix: remaining whitespaces will be trimmed anyway
			src = src[:len(src)-len("}\n")]
			return bytes.TrimSpace(src)
		}
		// Gofmt has also indented the function body one level.
		// Adjust that with indentAdj.
		indentAdj = -1
	}

	// Succeeded, or out of options.
	return
}

// format formats the given package file originally obtained from src
// and adjusts the result based on the original source via sourceAdj
// and indentAdj.
func format(
	fset *token.FileSet,
	file *ast.File,
	sourceAdj func(src []byte, indent int) []byte,
	indentAdj int,
	src []byte,
	cfg printer.Config,
) ([]byte, error) {
	if sourceAdj == nil {
		// Complete source file.
		var buf bytes.Buffer
		err := cfg.Fprint(&buf, fset, file)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	// Partial source file.
	// Determine and prepend leading space.
	i, j := 0, 0
	for j < len(src) && isSpace(src[j]) {
		if src[j] == '\n' {
			i = j + 1 // byte offset of last line in leading space
		}
		j++
	}
	var res []byte
	res = append(res, src[:i]...)

	// Determine and prepend indentation of first code line.
	// Spaces are ignored unless there are no tabs,
	// in which case spaces count as one tab.
	indent := 0
	hasSpace := false
	for _, b := range src[i:j] {
		switch b {
		case ' ':
			hasSpace = true
		case '\t':
			indent++
		}
	}
	if indent == 0 && hasSpace {
		indent = 1
	}
	for i := 0; i < indent; i++ {
		res = append(res, '\t')
	}

	// Format the source.
	// Write it without any leading and trailing space.
	cfg.Indent = indent + indentAdj
	var buf bytes.Buffer
	err := cfg.Fprint(&buf, fset, file)
	if err != nil {
		return nil, err
	}
	out := sourceAdj(buf.Bytes(), cfg.Indent)

	// If the adjusted output is empty, the source
	// was empty but (possibly) for white space.
	// The result is the incoming source.
	if len(out) == 0 {
		return src, nil
	}

	// Otherwise, append output to leading space.
	res = append(res, out...)

	// Determine and append trailing space.
	i = len(src)
	for i > 0 && isSpace(src[i-1]) {
		i--
	}
	return append(res, src[i:]...), nil
}

// isSpace reports whether the byte is a space character.
// isSpace defines a space as being among the following bytes: ' ', '\t', '\n' and '\r'.
func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

```

// === FILE: references!/go/src/cmd/gofmt/rewrite.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

func initRewrite() {
	if *rewriteRule == "" {
		rewrite = nil // disable any previous rewrite
		return
	}
	f := strings.Split(*rewriteRule, "->")
	if len(f) != 2 {
		fmt.Fprintf(os.Stderr, "rewrite rule must be of the form 'pattern -> replacement'\n")
		os.Exit(2)
	}
	pattern := parseExpr(f[0], "pattern")
	replace := parseExpr(f[1], "replacement")
	rewrite = func(fset *token.FileSet, p *ast.File) *ast.File {
		return rewriteFile(fset, pattern, replace, p)
	}
}

// parseExpr parses s as an expression.
// It might make sense to expand this to allow statement patterns,
// but there are problems with preserving formatting and also
// with what a wildcard for a statement looks like.
func parseExpr(s, what string) ast.Expr {
	x, err := parser.ParseExpr(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parsing %s %s at %s\n", what, s, err)
		os.Exit(2)
	}
	return x
}

// Keep this function for debugging.
/*
func dump(msg string, val reflect.Value) {
	fmt.Printf("%s:\n", msg)
	ast.Print(fileSet, val.Interface())
	fmt.Println()
}
*/

// rewriteFile applies the rewrite rule 'pattern -> replace' to an entire file.
func rewriteFile(fileSet *token.FileSet, pattern, replace ast.Expr, p *ast.File) *ast.File {
	cmap := ast.NewCommentMap(fileSet, p, p.Comments)
	m := make(map[string]reflect.Value)
	pat := reflect.ValueOf(pattern)
	repl := reflect.ValueOf(replace)

	var rewriteVal func(val reflect.Value) reflect.Value
	rewriteVal = func(val reflect.Value) reflect.Value {
		// don't bother if val is invalid to start with
		if !val.IsValid() {
			return reflect.Value{}
		}
		val = apply(rewriteVal, val)
		clear(m)
		if match(m, pat, val) {
			val = subst(m, repl, reflect.ValueOf(val.Interface().(ast.Node).Pos()))
		}
		return val
	}

	r := apply(rewriteVal, reflect.ValueOf(p)).Interface().(*ast.File)
	r.Comments = cmap.Filter(r).Comments() // recreate comments list
	return r
}

// set is a wrapper for x.Set(y); it protects the caller from panics if x cannot be changed to y.
func set(x, y reflect.Value) {
	// don't bother if x cannot be set or y is invalid
	if !x.CanSet() || !y.IsValid() {
		return
	}
	defer func() {
		if x := recover(); x != nil {
			if s, ok := x.(string); ok &&
				(strings.Contains(s, "type mismatch") || strings.Contains(s, "not assignable")) {
				// x cannot be set to y - ignore this rewrite
				return
			}
			panic(x)
		}
	}()
	x.Set(y)
}

// Values/types for special cases.
var (
	objectPtrNil = reflect.ValueOf((*ast.Object)(nil))
	scopePtrNil  = reflect.ValueOf((*ast.Scope)(nil))

	identType     = reflect.TypeFor[*ast.Ident]()
	objectPtrType = reflect.TypeFor[*ast.Object]()
	positionType  = reflect.TypeFor[token.Pos]()
	callExprType  = reflect.TypeFor[*ast.CallExpr]()
	scopePtrType  = reflect.TypeFor[*ast.Scope]()
)

// apply replaces each AST field x in val with f(x), returning val.
// To avoid extra conversions, f operates on the reflect.Value form.
func apply(f func(reflect.Value) reflect.Value, val reflect.Value) reflect.Value {
	if !val.IsValid() {
		return reflect.Value{}
	}

	// *ast.Objects introduce cycles and are likely incorrect after
	// rewrite; don't follow them but replace with nil instead
	if val.Type() == objectPtrType {
		return objectPtrNil
	}

	// similarly for scopes: they are likely incorrect after a rewrite;
	// replace them with nil
	if val.Type() == scopePtrType {
		return scopePtrNil
	}

	switch v := reflect.Indirect(val); v.Kind() {
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			e := v.Index(i)
			set(e, f(e))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			e := v.Field(i)
			set(e, f(e))
		}
	case reflect.Interface:
		e := v.Elem()
		set(v, f(e))
	}
	return val
}

func isWildcard(s string) bool {
	rune, size := utf8.DecodeRuneInString(s)
	return size == len(s) && unicode.IsLower(rune)
}

// match reports whether pattern matches val,
// recording wildcard submatches in m.
// If m == nil, match checks whether pattern == val.
func match(m map[string]reflect.Value, pattern, val reflect.Value) bool {
	// Wildcard matches any expression. If it appears multiple
	// times in the pattern, it must match the same expression
	// each time.
	if m != nil && pattern.IsValid() && pattern.Type() == identType {
		name := pattern.Interface().(*ast.Ident).Name
		if isWildcard(name) && val.IsValid() {
			// wildcards only match valid (non-nil) expressions.
			if _, ok := val.Interface().(ast.Expr); ok && !val.IsNil() {
				if old, ok := m[name]; ok {
					return match(nil, old, val)
				}
				m[name] = val
				return true
			}
		}
	}

	// Otherwise, pattern and val must match recursively.
	if !pattern.IsValid() || !val.IsValid() {
		return !pattern.IsValid() && !val.IsValid()
	}
	if pattern.Type() != val.Type() {
		return false
	}

	// Special cases.
	switch pattern.Type() {
	case identType:
		// For identifiers, only the names need to match
		// (and none of the other *ast.Object information).
		// This is a common case, handle it all here instead
		// of recursing down any further via reflection.
		p := pattern.Interface().(*ast.Ident)
		v := val.Interface().(*ast.Ident)
		return p == nil && v == nil || p != nil && v != nil && p.Name == v.Name
	case objectPtrType, positionType:
		// object pointers and token positions always match
		return true
	case callExprType:
		// For calls, the Ellipsis fields (token.Pos) must
		// match since that is how f(x) and f(x...) are different.
		// Check them here but fall through for the remaining fields.
		p := pattern.Interface().(*ast.CallExpr)
		v := val.Interface().(*ast.CallExpr)
		if p.Ellipsis.IsValid() != v.Ellipsis.IsValid() {
			return false
		}
	}

	p := reflect.Indirect(pattern)
	v := reflect.Indirect(val)
	if !p.IsValid() || !v.IsValid() {
		return !p.IsValid() && !v.IsValid()
	}

	switch p.Kind() {
	case reflect.Slice:
		if p.Len() != v.Len() {
			return false
		}
		for i := 0; i < p.Len(); i++ {
			if !match(m, p.Index(i), v.Index(i)) {
				return false
			}
		}
		return true

	case reflect.Struct:
		for i := 0; i < p.NumField(); i++ {
			if !match(m, p.Field(i), v.Field(i)) {
				return false
			}
		}
		return true

	case reflect.Interface:
		return match(m, p.Elem(), v.Elem())
	}

	// Handle token integers, etc.
	return p.Interface() == v.Interface()
}

// subst returns a copy of pattern with values from m substituted in place
// of wildcards and pos used as the position of tokens from the pattern.
// if m == nil, subst returns a copy of pattern and doesn't change the line
// number information.
func subst(m map[string]reflect.Value, pattern reflect.Value, pos reflect.Value) reflect.Value {
	if !pattern.IsValid() {
		return reflect.Value{}
	}

	// Wildcard gets replaced with map value.
	if m != nil && pattern.Type() == identType {
		name := pattern.Interface().(*ast.Ident).Name
		if isWildcard(name) {
			if old, ok := m[name]; ok {
				return subst(nil, old, reflect.Value{})
			}
		}
	}

	if pos.IsValid() && pattern.Type() == positionType {
		// use new position only if old position was valid in the first place
		if old := pattern.Interface().(token.Pos); !old.IsValid() {
			return pattern
		}
		return pos
	}

	// Otherwise copy.
	switch p := pattern; p.Kind() {
	case reflect.Slice:
		if p.IsNil() {
			// Do not turn nil slices into empty slices. go/ast
			// guarantees that certain lists will be nil if not
			// populated.
			return reflect.Zero(p.Type())
		}
		v := reflect.MakeSlice(p.Type(), p.Len(), p.Len())
		for i := 0; i < p.Len(); i++ {
			v.Index(i).Set(subst(m, p.Index(i), pos))
		}
		return v

	case reflect.Struct:
		v := reflect.New(p.Type()).Elem()
		for i := 0; i < p.NumField(); i++ {
			v.Field(i).Set(subst(m, p.Field(i), pos))
		}
		return v

	case reflect.Pointer:
		v := reflect.New(p.Type()).Elem()
		if elem := p.Elem(); elem.IsValid() {
			v.Set(subst(m, elem, pos).Addr())
		}
		return v

	case reflect.Interface:
		v := reflect.New(p.Type()).Elem()
		if elem := p.Elem(); elem.IsValid() {
			v.Set(subst(m, elem, pos))
		}
		return v
	}

	return pattern
}

```

// === FILE: references!/go/src/cmd/gofmt/simplify.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/ast"
	"go/token"
	"reflect"
)

type simplifier struct{}

func (s simplifier) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.CompositeLit:
		// array, slice, and map composite literals may be simplified
		outer := n
		var keyType, eltType ast.Expr
		switch typ := outer.Type.(type) {
		case *ast.ArrayType:
			eltType = typ.Elt
		case *ast.MapType:
			keyType = typ.Key
			eltType = typ.Value
		}

		if eltType != nil {
			var ktyp reflect.Value
			if keyType != nil {
				ktyp = reflect.ValueOf(keyType)
			}
			typ := reflect.ValueOf(eltType)
			for i, x := range outer.Elts {
				px := &outer.Elts[i]
				// look at value of indexed/named elements
				if t, ok := x.(*ast.KeyValueExpr); ok {
					if keyType != nil {
						s.simplifyLiteral(ktyp, keyType, t.Key, &t.Key)
					}
					x = t.Value
					px = &t.Value
				}
				s.simplifyLiteral(typ, eltType, x, px)
			}
			// node was simplified - stop walk (there are no subnodes to simplify)
			return nil
		}

	case *ast.SliceExpr:
		// a slice expression of the form: s[a:len(s)]
		// can be simplified to: s[a:]
		// if s is "simple enough" (for now we only accept identifiers)
		//
		// Note: This may not be correct because len may have been redeclared in
		//       the same package. However, this is extremely unlikely and so far
		//       (April 2022, after years of supporting this rewrite feature)
		//       has never come up, so let's keep it working as is (see also #15153).
		//
		// Also note that this code used to use go/ast's object tracking,
		// which was removed in exchange for go/parser.Mode.SkipObjectResolution.
		// False positives are extremely unlikely as described above,
		// and go/ast's object tracking is incomplete in any case.
		if n.Max != nil {
			// - 3-index slices always require the 2nd and 3rd index
			break
		}
		if s, _ := n.X.(*ast.Ident); s != nil {
			// the array/slice object is a single identifier
			if call, _ := n.High.(*ast.CallExpr); call != nil && len(call.Args) == 1 && !call.Ellipsis.IsValid() {
				// the high expression is a function call with a single argument
				if fun, _ := call.Fun.(*ast.Ident); fun != nil && fun.Name == "len" {
					// the function called is "len"
					if arg, _ := call.Args[0].(*ast.Ident); arg != nil && arg.Name == s.Name {
						// the len argument is the array/slice object
						n.High = nil
					}
				}
			}
		}
		// Note: We could also simplify slice expressions of the form s[0:b] to s[:b]
		//       but we leave them as is since sometimes we want to be very explicit
		//       about the lower bound.
		// An example where the 0 helps:
		//       x, y, z := b[0:2], b[2:4], b[4:6]
		// An example where it does not:
		//       x, y := b[:n], b[n:]

	case *ast.RangeStmt:
		// - a range of the form: for x, _ = range v {...}
		// can be simplified to: for x = range v {...}
		// - a range of the form: for _ = range v {...}
		// can be simplified to: for range v {...}
		if isBlank(n.Value) {
			n.Value = nil
		}
		if isBlank(n.Key) && n.Value == nil {
			n.Key = nil
		}
	}

	return s
}

func (s simplifier) simplifyLiteral(typ reflect.Value, astType, x ast.Expr, px *ast.Expr) {
	ast.Walk(s, x) // simplify x

	// if the element is a composite literal and its literal type
	// matches the outer literal's element type exactly, the inner
	// literal type may be omitted
	if inner, ok := x.(*ast.CompositeLit); ok {
		if match(nil, typ, reflect.ValueOf(inner.Type)) {
			inner.Type = nil
		}
	}
	// if the outer literal's element type is a pointer type *T
	// and the element is & of a composite literal of type T,
	// the inner &T may be omitted.
	if ptr, ok := astType.(*ast.StarExpr); ok {
		if addr, ok := x.(*ast.UnaryExpr); ok && addr.Op == token.AND {
			if inner, ok := addr.X.(*ast.CompositeLit); ok {
				if match(nil, reflect.ValueOf(ptr.X), reflect.ValueOf(inner.Type)) {
					inner.Type = nil // drop T
					*px = inner      // drop &
				}
			}
		}
	}
}

func isBlank(x ast.Expr) bool {
	ident, ok := x.(*ast.Ident)
	return ok && ident.Name == "_"
}

func simplify(f *ast.File) {
	// remove empty declarations such as "const ()", etc
	removeEmptyDeclGroups(f)

	var s simplifier
	ast.Walk(s, f)
}

func removeEmptyDeclGroups(f *ast.File) {
	i := 0
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); !ok || !isEmpty(f, g) {
			f.Decls[i] = d
			i++
		}
	}
	f.Decls = f.Decls[:i]
}

func isEmpty(f *ast.File, g *ast.GenDecl) bool {
	if g.Doc != nil || g.Specs != nil {
		return false
	}

	for _, c := range f.Comments {
		// if there is a comment in the declaration, it is not considered empty
		if g.Pos() <= c.Pos() && c.End() <= g.End() {
			return false
		}
	}

	return true
}

```

