# Domain Architecture: regexp

## Layout Topology
```text
regexp/
├── syntax
│   ├── compile.go
│   ├── doc.go
│   ├── make_perl_groups.pl
│   ├── op_string.go
│   ├── parse.go
│   ├── perl_groups.go
│   ├── prog.go
│   ├── regexp.go
│   └── simplify.go
├── backtrack.go
├── exec.go
├── onepass.go
└── regexp.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/regexp/backtrack.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// backtrack is a regular expression search with submatch
// tracking for small regular expressions and texts. It allocates
// a bit vector with (length of input) * (length of prog) bits,
// to make sure it never explores the same (character position, instruction)
// state multiple times. This limits the search to run in time linear in
// the length of the test.
//
// backtrack is a fast replacement for the NFA code on small
// regexps when onepass cannot be used.

package regexp

import (
	"regexp/syntax"
	"sync"
)

// A job is an entry on the backtracker's job stack. It holds
// the instruction pc and the position in the input.
type job struct {
	pc  uint32
	arg bool
	pos int
}

const (
	visitedBits        = 32
	maxBacktrackProg   = 500        // len(prog.Inst) <= max
	maxBacktrackVector = 256 * 1024 // bit vector size <= max (bits)
)

// bitState holds state for the backtracker.
type bitState struct {
	end      int
	cap      []int
	matchcap []int
	jobs     []job
	visited  []uint32

	inputs inputs
}

var bitStatePool sync.Pool

func newBitState() *bitState {
	b, ok := bitStatePool.Get().(*bitState)
	if !ok {
		b = new(bitState)
	}
	return b
}

func freeBitState(b *bitState) {
	b.inputs.clear()
	bitStatePool.Put(b)
}

// maxBitStateLen returns the maximum length of a string to search with
// the backtracker using prog.
func maxBitStateLen(prog *syntax.Prog) int {
	if !shouldBacktrack(prog) {
		return 0
	}
	return maxBacktrackVector / len(prog.Inst)
}

// shouldBacktrack reports whether the program is too
// long for the backtracker to run.
func shouldBacktrack(prog *syntax.Prog) bool {
	return len(prog.Inst) <= maxBacktrackProg
}

// reset resets the state of the backtracker.
// end is the end position in the input.
// ncap is the number of captures.
func (b *bitState) reset(prog *syntax.Prog, end int, ncap int) {
	b.end = end

	if cap(b.jobs) == 0 {
		b.jobs = make([]job, 0, 256)
	} else {
		b.jobs = b.jobs[:0]
	}

	visitedSize := (len(prog.Inst)*(end+1) + visitedBits - 1) / visitedBits
	if cap(b.visited) < visitedSize {
		b.visited = make([]uint32, visitedSize, maxBacktrackVector/visitedBits)
	} else {
		b.visited = b.visited[:visitedSize]
		clear(b.visited) // set to 0
	}

	if cap(b.cap) < ncap {
		b.cap = make([]int, ncap)
	} else {
		b.cap = b.cap[:ncap]
	}
	for i := range b.cap {
		b.cap[i] = -1
	}

	if cap(b.matchcap) < ncap {
		b.matchcap = make([]int, ncap)
	} else {
		b.matchcap = b.matchcap[:ncap]
	}
	for i := range b.matchcap {
		b.matchcap[i] = -1
	}
}

// shouldVisit reports whether the combination of (pc, pos) has not
// been visited yet.
func (b *bitState) shouldVisit(pc uint32, pos int) bool {
	n := uint(int(pc)*(b.end+1) + pos)
	if b.visited[n/visitedBits]&(1<<(n&(visitedBits-1))) != 0 {
		return false
	}
	b.visited[n/visitedBits] |= 1 << (n & (visitedBits - 1))
	return true
}

// push pushes (pc, pos, arg) onto the job stack if it should be
// visited.
func (b *bitState) push(re *Regexp, pc uint32, pos int, arg bool) {
	// Only check shouldVisit when arg is false.
	// When arg is true, we are continuing a previous visit.
	if re.prog.Inst[pc].Op != syntax.InstFail && (arg || b.shouldVisit(pc, pos)) {
		b.jobs = append(b.jobs, job{pc: pc, arg: arg, pos: pos})
	}
}

// tryBacktrack runs a backtracking search starting at pos.
func (re *Regexp) tryBacktrack(b *bitState, i input, pc uint32, pos int) bool {
	longest := re.longest

	b.push(re, pc, pos, false)
	for len(b.jobs) > 0 {
		l := len(b.jobs) - 1
		// Pop job off the stack.
		pc := b.jobs[l].pc
		pos := b.jobs[l].pos
		arg := b.jobs[l].arg
		b.jobs = b.jobs[:l]

		// Optimization: rather than push and pop,
		// code that is going to Push and continue
		// the loop simply updates ip, p, and arg
		// and jumps to CheckAndLoop. We have to
		// do the ShouldVisit check that Push
		// would have, but we avoid the stack
		// manipulation.
		goto Skip
	CheckAndLoop:
		if !b.shouldVisit(pc, pos) {
			continue
		}
	Skip:

		inst := &re.prog.Inst[pc]

		switch inst.Op {
		default:
			panic("bad inst")
		case syntax.InstFail:
			panic("unexpected InstFail")
		case syntax.InstAlt:
			// Cannot just
			//   b.push(inst.Out, pos, false)
			//   b.push(inst.Arg, pos, false)
			// If during the processing of inst.Out, we encounter
			// inst.Arg via another path, we want to process it then.
			// Pushing it here will inhibit that. Instead, re-push
			// inst with arg==true as a reminder to push inst.Arg out
			// later.
			if arg {
				// Finished inst.Out; try inst.Arg.
				arg = false
				pc = inst.Arg
				goto CheckAndLoop
			} else {
				b.push(re, pc, pos, true)
				pc = inst.Out
				goto CheckAndLoop
			}

		case syntax.InstAltMatch:
			// One opcode consumes runes; the other leads to match.
			switch re.prog.Inst[inst.Out].Op {
			case syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
				// inst.Arg is the match.
				b.push(re, inst.Arg, pos, false)
				pc = inst.Arg
				pos = b.end
				goto CheckAndLoop
			}
			// inst.Out is the match - non-greedy
			b.push(re, inst.Out, b.end, false)
			pc = inst.Out
			goto CheckAndLoop

		case syntax.InstRune:
			r, width := i.step(pos)
			if !inst.MatchRune(r) {
				continue
			}
			pos += width
			pc = inst.Out
			goto CheckAndLoop

		case syntax.InstRune1:
			r, width := i.step(pos)
			if r != inst.Rune[0] {
				continue
			}
			pos += width
			pc = inst.Out
			goto CheckAndLoop

		case syntax.InstRuneAnyNotNL:
			r, width := i.step(pos)
			if r == '\n' || r == endOfText {
				continue
			}
			pos += width
			pc = inst.Out
			goto CheckAndLoop

		case syntax.InstRuneAny:
			r, width := i.step(pos)
			if r == endOfText {
				continue
			}
			pos += width
			pc = inst.Out
			goto CheckAndLoop

		case syntax.InstCapture:
			if arg {
				// Finished inst.Out; restore the old value.
				b.cap[inst.Arg] = pos
				continue
			} else {
				if inst.Arg < uint32(len(b.cap)) {
					// Capture pos to register, but save old value.
					b.push(re, pc, b.cap[inst.Arg], true) // come back when we're done.
					b.cap[inst.Arg] = pos
				}
				pc = inst.Out
				goto CheckAndLoop
			}

		case syntax.InstEmptyWidth:
			flag := i.context(pos)
			if !flag.match(syntax.EmptyOp(inst.Arg)) {
				continue
			}
			pc = inst.Out
			goto CheckAndLoop

		case syntax.InstNop:
			pc = inst.Out
			goto CheckAndLoop

		case syntax.InstMatch:
			// We found a match. If the caller doesn't care
			// where the match is, no point going further.
			if len(b.cap) == 0 {
				return true
			}

			// Record best match so far.
			// Only need to check end point, because this entire
			// call is only considering one start position.
			if len(b.cap) > 1 {
				b.cap[1] = pos
			}
			if old := b.matchcap[1]; old == -1 || (longest && pos > 0 && pos > old) {
				copy(b.matchcap, b.cap)
			}

			// If going for first match, we're done.
			if !longest {
				return true
			}

			// If we used the entire text, no longer match is possible.
			if pos == b.end {
				return true
			}

			// Otherwise, continue on in hope of a longer match.
			continue
		}
	}

	return longest && len(b.matchcap) > 1 && b.matchcap[1] >= 0
}

// backtrack runs a backtracking search of prog on the input starting at pos.
func (re *Regexp) backtrack(ib []byte, is string, pos int, ncap int, dstCap []int) []int {
	startCond := re.cond
	if startCond == ^syntax.EmptyOp(0) { // impossible
		return nil
	}
	if startCond&syntax.EmptyBeginText != 0 && pos != 0 {
		// Anchored match, past beginning of text.
		return nil
	}

	b := newBitState()
	i, end := b.inputs.init(nil, ib, is)
	b.reset(re.prog, end, ncap)

	// Anchored search must start at the beginning of the input
	if startCond&syntax.EmptyBeginText != 0 {
		if len(b.cap) > 0 {
			b.cap[0] = pos
		}
		if !re.tryBacktrack(b, i, uint32(re.prog.Start), pos) {
			freeBitState(b)
			return nil
		}
	} else {

		// Unanchored search, starting from each possible text position.
		// Notice that we have to try the empty string at the end of
		// the text, so the loop condition is pos <= end, not pos < end.
		// This looks like it's quadratic in the size of the text,
		// but we are not clearing visited between calls to TrySearch,
		// so no work is duplicated and it ends up still being linear.
		width := -1
		for ; pos <= end && width != 0; pos += width {
			if len(re.prefix) > 0 {
				// Match requires literal prefix; fast search for it.
				advance := i.index(re, pos)
				if advance < 0 {
					freeBitState(b)
					return nil
				}
				pos += advance
			}

			if len(b.cap) > 0 {
				b.cap[0] = pos
			}
			if re.tryBacktrack(b, i, uint32(re.prog.Start), pos) {
				// Match must be leftmost; done.
				goto Match
			}
			_, width = i.step(pos)
		}
		freeBitState(b)
		return nil
	}

Match:
	dstCap = append(dstCap, b.matchcap...)
	freeBitState(b)
	return dstCap
}

```

// === FILE: references!/go/src/regexp/exec.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package regexp

import (
	"io"
	"regexp/syntax"
	"sync"
)

// A queue is a 'sparse array' holding pending threads of execution.
// See https://research.swtch.com/2008/03/using-uninitialized-memory-for-fun-and.html
type queue struct {
	sparse []uint32
	dense  []entry
}

// An entry is an entry on a queue.
// It holds both the instruction pc and the actual thread.
// Some queue entries are just place holders so that the machine
// knows it has considered that pc. Such entries have t == nil.
type entry struct {
	pc uint32
	t  *thread
}

// A thread is the state of a single path through the machine:
// an instruction and a corresponding capture array.
// See https://swtch.com/~rsc/regexp/regexp2.html
type thread struct {
	inst *syntax.Inst
	cap  []int
}

// A machine holds all the state during an NFA simulation for p.
type machine struct {
	re       *Regexp      // corresponding Regexp
	p        *syntax.Prog // compiled program
	q0, q1   queue        // two queues for runq, nextq
	pool     []*thread    // pool of available threads
	matched  bool         // whether a match was found
	matchcap []int        // capture information for the match

	inputs inputs
}

type inputs struct {
	// cached inputs, to avoid allocation
	bytes  inputBytes
	string inputString
	reader inputReader
}

func (i *inputs) newBytes(b []byte) input {
	i.bytes.str = b
	return &i.bytes
}

func (i *inputs) newString(s string) input {
	i.string.str = s
	return &i.string
}

func (i *inputs) newReader(r io.RuneReader) input {
	i.reader.r = r
	i.reader.atEOT = false
	i.reader.pos = 0
	return &i.reader
}

func (i *inputs) clear() {
	// We need to clear 1 of these.
	// Avoid the expense of clearing the others (pointer write barrier).
	if i.bytes.str != nil {
		i.bytes.str = nil
	} else if i.reader.r != nil {
		i.reader.r = nil
	} else {
		i.string.str = ""
	}
}

func (i *inputs) init(r io.RuneReader, b []byte, s string) (input, int) {
	if r != nil {
		return i.newReader(r), 0
	}
	if b != nil {
		return i.newBytes(b), len(b)
	}
	return i.newString(s), len(s)
}

func (m *machine) init(ncap int) {
	for _, t := range m.pool {
		t.cap = t.cap[:ncap]
	}
	m.matchcap = m.matchcap[:ncap]
}

// alloc allocates a new thread with the given instruction.
// It uses the free pool if possible.
func (m *machine) alloc(i *syntax.Inst) *thread {
	var t *thread
	if n := len(m.pool); n > 0 {
		t = m.pool[n-1]
		m.pool = m.pool[:n-1]
	} else {
		t = new(thread)
		t.cap = make([]int, len(m.matchcap), cap(m.matchcap))
	}
	t.inst = i
	return t
}

// A lazyFlag is a lazily-evaluated syntax.EmptyOp,
// for checking zero-width flags like ^ $ \A \z \B \b.
// It records the pair of relevant runes and does not
// determine the implied flags until absolutely necessary
// (most of the time, that means never).
type lazyFlag uint64

func newLazyFlag(r1, r2 rune) lazyFlag {
	return lazyFlag(uint64(r1)<<32 | uint64(uint32(r2)))
}

func (f lazyFlag) match(op syntax.EmptyOp) bool {
	if op == 0 {
		return true
	}
	r1 := rune(f >> 32)
	if op&syntax.EmptyBeginLine != 0 {
		if r1 != '\n' && r1 >= 0 {
			return false
		}
		op &^= syntax.EmptyBeginLine
	}
	if op&syntax.EmptyBeginText != 0 {
		if r1 >= 0 {
			return false
		}
		op &^= syntax.EmptyBeginText
	}
	if op == 0 {
		return true
	}
	r2 := rune(f)
	if op&syntax.EmptyEndLine != 0 {
		if r2 != '\n' && r2 >= 0 {
			return false
		}
		op &^= syntax.EmptyEndLine
	}
	if op&syntax.EmptyEndText != 0 {
		if r2 >= 0 {
			return false
		}
		op &^= syntax.EmptyEndText
	}
	if op == 0 {
		return true
	}
	if syntax.IsWordChar(r1) != syntax.IsWordChar(r2) {
		op &^= syntax.EmptyWordBoundary
	} else {
		op &^= syntax.EmptyNoWordBoundary
	}
	return op == 0
}

// match runs the machine over the input starting at pos.
// It reports whether a match was found.
// If so, m.matchcap holds the submatch information.
func (m *machine) match(i input, pos int) bool {
	startCond := m.re.cond
	if startCond == ^syntax.EmptyOp(0) { // impossible
		return false
	}
	m.matched = false
	for i := range m.matchcap {
		m.matchcap[i] = -1
	}
	runq, nextq := &m.q0, &m.q1
	r, r1 := endOfText, endOfText
	width, width1 := 0, 0
	r, width = i.step(pos)
	if r != endOfText {
		r1, width1 = i.step(pos + width)
	}
	var flag lazyFlag
	if pos == 0 {
		flag = newLazyFlag(-1, r)
	} else {
		flag = i.context(pos)
	}
	for {
		if len(runq.dense) == 0 {
			if startCond&syntax.EmptyBeginText != 0 && pos != 0 {
				// Anchored match, past beginning of text.
				break
			}
			if m.matched {
				// Have match; finished exploring alternatives.
				break
			}
			if len(m.re.prefix) > 0 && r1 != m.re.prefixRune && i.canCheckPrefix() {
				// Match requires literal prefix; fast search for it.
				advance := i.index(m.re, pos)
				if advance < 0 {
					break
				}
				pos += advance
				r, width = i.step(pos)
				r1, width1 = i.step(pos + width)
			}
		}
		if !m.matched {
			if len(m.matchcap) > 0 {
				m.matchcap[0] = pos
			}
			m.add(runq, uint32(m.p.Start), pos, m.matchcap, &flag, nil)
		}
		flag = newLazyFlag(r, r1)
		m.step(runq, nextq, pos, pos+width, r, &flag)
		if width == 0 {
			break
		}
		if len(m.matchcap) == 0 && m.matched {
			// Found a match and not paying attention
			// to where it is, so any match will do.
			break
		}
		pos += width
		r, width = r1, width1
		if r != endOfText {
			r1, width1 = i.step(pos + width)
		}
		runq, nextq = nextq, runq
	}
	m.clear(nextq)
	return m.matched
}

// clear frees all threads on the thread queue.
func (m *machine) clear(q *queue) {
	for _, d := range q.dense {
		if d.t != nil {
			m.pool = append(m.pool, d.t)
		}
	}
	q.dense = q.dense[:0]
}

// step executes one step of the machine, running each of the threads
// on runq and appending new threads to nextq.
// The step processes the rune c (which may be endOfText),
// which starts at position pos and ends at nextPos.
// nextCond gives the setting for the empty-width flags after c.
func (m *machine) step(runq, nextq *queue, pos, nextPos int, c rune, nextCond *lazyFlag) {
	longest := m.re.longest
	for j := 0; j < len(runq.dense); j++ {
		d := &runq.dense[j]
		t := d.t
		if t == nil {
			continue
		}
		if longest && m.matched && len(t.cap) > 0 && m.matchcap[0] < t.cap[0] {
			m.pool = append(m.pool, t)
			continue
		}
		i := t.inst
		add := false
		switch i.Op {
		default:
			panic("bad inst")

		case syntax.InstMatch:
			if len(t.cap) > 0 && (!longest || !m.matched || m.matchcap[1] < pos) {
				t.cap[1] = pos
				copy(m.matchcap, t.cap)
			}
			if !longest {
				// First-match mode: cut off all lower-priority threads.
				for _, d := range runq.dense[j+1:] {
					if d.t != nil {
						m.pool = append(m.pool, d.t)
					}
				}
				runq.dense = runq.dense[:0]
			}
			m.matched = true

		case syntax.InstRune:
			add = i.MatchRune(c)
		case syntax.InstRune1:
			add = c == i.Rune[0]
		case syntax.InstRuneAny:
			add = true
		case syntax.InstRuneAnyNotNL:
			add = c != '\n'
		}
		if add {
			t = m.add(nextq, i.Out, nextPos, t.cap, nextCond, t)
		}
		if t != nil {
			m.pool = append(m.pool, t)
		}
	}
	runq.dense = runq.dense[:0]
}

// add adds an entry to q for pc, unless the q already has such an entry.
// It also recursively adds an entry for all instructions reachable from pc by following
// empty-width conditions satisfied by cond.  pos gives the current position
// in the input.
func (m *machine) add(q *queue, pc uint32, pos int, cap []int, cond *lazyFlag, t *thread) *thread {
Again:
	if pc == 0 {
		return t
	}
	if j := q.sparse[pc]; j < uint32(len(q.dense)) && q.dense[j].pc == pc {
		return t
	}

	j := len(q.dense)
	q.dense = q.dense[:j+1]
	d := &q.dense[j]
	d.t = nil
	d.pc = pc
	q.sparse[pc] = uint32(j)

	i := &m.p.Inst[pc]
	switch i.Op {
	default:
		panic("unhandled")
	case syntax.InstFail:
		// nothing
	case syntax.InstAlt, syntax.InstAltMatch:
		t = m.add(q, i.Out, pos, cap, cond, t)
		pc = i.Arg
		goto Again
	case syntax.InstEmptyWidth:
		if cond.match(syntax.EmptyOp(i.Arg)) {
			pc = i.Out
			goto Again
		}
	case syntax.InstNop:
		pc = i.Out
		goto Again
	case syntax.InstCapture:
		if int(i.Arg) < len(cap) {
			opos := cap[i.Arg]
			cap[i.Arg] = pos
			m.add(q, i.Out, pos, cap, cond, nil)
			cap[i.Arg] = opos
		} else {
			pc = i.Out
			goto Again
		}
	case syntax.InstMatch, syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
		if t == nil {
			t = m.alloc(i)
		} else {
			t.inst = i
		}
		if len(cap) > 0 && &t.cap[0] != &cap[0] {
			copy(t.cap, cap)
		}
		d.t = t
		t = nil
	}
	return t
}

type onePassMachine struct {
	inputs   inputs
	matchcap []int
}

var onePassPool sync.Pool

func newOnePassMachine() *onePassMachine {
	m, ok := onePassPool.Get().(*onePassMachine)
	if !ok {
		m = new(onePassMachine)
	}
	return m
}

func freeOnePassMachine(m *onePassMachine) {
	m.inputs.clear()
	onePassPool.Put(m)
}

// doOnePass implements r.find using the one-pass execution engine.
func (re *Regexp) doOnePass(ir io.RuneReader, ib []byte, is string, pos, ncap int, dstCap []int) []int {
	startCond := re.cond
	if startCond == ^syntax.EmptyOp(0) { // impossible
		return nil
	}

	m := newOnePassMachine()
	if cap(m.matchcap) < ncap {
		m.matchcap = make([]int, ncap)
	} else {
		m.matchcap = m.matchcap[:ncap]
	}

	matched := false
	for i := range m.matchcap {
		m.matchcap[i] = -1
	}

	i, _ := m.inputs.init(ir, ib, is)

	r, r1 := endOfText, endOfText
	width, width1 := 0, 0
	r, width = i.step(pos)
	if r != endOfText {
		r1, width1 = i.step(pos + width)
	}
	var flag lazyFlag
	if pos == 0 {
		flag = newLazyFlag(-1, r)
	} else {
		flag = i.context(pos)
	}
	pc := re.onepass.Start
	inst := &re.onepass.Inst[pc]
	// If there is a simple literal prefix, skip over it.
	if pos == 0 && flag.match(syntax.EmptyOp(inst.Arg)) &&
		len(re.prefix) > 0 && i.canCheckPrefix() {
		// Match requires literal prefix; fast search for it.
		if !i.hasPrefix(re) {
			goto Return
		}
		pos += len(re.prefix)
		r, width = i.step(pos)
		r1, width1 = i.step(pos + width)
		flag = i.context(pos)
		pc = int(re.prefixEnd)
	}
	for {
		inst = &re.onepass.Inst[pc]
		pc = int(inst.Out)
		switch inst.Op {
		default:
			panic("bad inst")
		case syntax.InstMatch:
			matched = true
			if len(m.matchcap) > 0 {
				m.matchcap[0] = 0
				m.matchcap[1] = pos
			}
			goto Return
		case syntax.InstRune:
			if !inst.MatchRune(r) {
				goto Return
			}
		case syntax.InstRune1:
			if r != inst.Rune[0] {
				goto Return
			}
		case syntax.InstRuneAny:
			// Nothing
		case syntax.InstRuneAnyNotNL:
			if r == '\n' {
				goto Return
			}
		// peek at the input rune to see which branch of the Alt to take
		case syntax.InstAlt, syntax.InstAltMatch:
			pc = int(onePassNext(inst, r))
			continue
		case syntax.InstFail:
			goto Return
		case syntax.InstNop:
			continue
		case syntax.InstEmptyWidth:
			if !flag.match(syntax.EmptyOp(inst.Arg)) {
				goto Return
			}
			continue
		case syntax.InstCapture:
			if int(inst.Arg) < len(m.matchcap) {
				m.matchcap[inst.Arg] = pos
			}
			continue
		}
		if width == 0 {
			break
		}
		flag = newLazyFlag(r, r1)
		pos += width
		r, width = r1, width1
		if r != endOfText {
			r1, width1 = i.step(pos + width)
		}
	}

Return:
	if !matched {
		freeOnePassMachine(m)
		return nil
	}

	dstCap = append(dstCap, m.matchcap...)
	freeOnePassMachine(m)
	return dstCap
}

// doMatch reports whether either r, b or s match the regexp.
func (re *Regexp) doMatch(r io.RuneReader, b []byte, s string) bool {
	return re.find(r, b, s, 0, 0, nil) != nil
}

// find finds the leftmost match in the input, appends the position
// of its subexpressions to dstCap and returns dstCap.
//
// nil is returned if no matches are found and non-nil if matches are found.
func (re *Regexp) find(r io.RuneReader, b []byte, s string, pos int, ncap int, dstCap []int) []int {
	if dstCap == nil {
		// Make sure 'return dstCap' is non-nil.
		dstCap = arrayNoInts[:0:0]
	}

	if r == nil && len(b)+len(s) < re.minInputLen {
		return nil
	}

	if re.onepass != nil {
		return re.doOnePass(r, b, s, pos, ncap, dstCap)
	}
	if r == nil && len(b)+len(s) < re.maxBitStateLen {
		return re.backtrack(b, s, pos, ncap, dstCap)
	}

	m := re.get()
	i, _ := m.inputs.init(r, b, s)

	m.init(ncap)
	if !m.match(i, pos) {
		re.put(m)
		return nil
	}

	dstCap = append(dstCap, m.matchcap...)
	re.put(m)
	return dstCap
}

// arrayNoInts is returned by find match if nil dstCap is passed
// to it with ncap=0.
var arrayNoInts [0]int

```

// === FILE: references!/go/src/regexp/onepass.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package regexp

import (
	"regexp/syntax"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// "One-pass" regexp execution.
// Some regexps can be analyzed to determine that they never need
// backtracking: they are guaranteed to run in one pass over the string
// without bothering to save all the usual NFA state.
// Detect those and execute them more quickly.

// A onePassProg is a compiled one-pass regular expression program.
// It is the same as syntax.Prog except for the use of onePassInst.
type onePassProg struct {
	Inst   []onePassInst
	Start  int // index of start instruction
	NumCap int // number of InstCapture insts in re
}

// A onePassInst is a single instruction in a one-pass regular expression program.
// It is the same as syntax.Inst except for the new 'Next' field.
type onePassInst struct {
	syntax.Inst
	Next []uint32
}

// onePassPrefix returns a literal string that all matches for the
// regexp must start with. Complete is true if the prefix
// is the entire match. Pc is the index of the last rune instruction
// in the string. The onePassPrefix skips over the mandatory
// EmptyBeginText.
func onePassPrefix(p *syntax.Prog) (prefix string, complete bool, pc uint32) {
	i := &p.Inst[p.Start]
	if i.Op != syntax.InstEmptyWidth || (syntax.EmptyOp(i.Arg))&syntax.EmptyBeginText == 0 {
		return "", i.Op == syntax.InstMatch, uint32(p.Start)
	}
	pc = i.Out
	i = &p.Inst[pc]
	for i.Op == syntax.InstNop {
		pc = i.Out
		i = &p.Inst[pc]
	}
	// Avoid allocation of buffer if prefix is empty.
	if iop(i) != syntax.InstRune || len(i.Rune) != 1 {
		return "", i.Op == syntax.InstMatch, uint32(p.Start)
	}

	// Have prefix; gather characters.
	var buf strings.Builder
	for iop(i) == syntax.InstRune && len(i.Rune) == 1 && syntax.Flags(i.Arg)&syntax.FoldCase == 0 && i.Rune[0] != utf8.RuneError {
		buf.WriteRune(i.Rune[0])
		pc, i = i.Out, &p.Inst[i.Out]
	}
	if i.Op == syntax.InstEmptyWidth &&
		syntax.EmptyOp(i.Arg)&syntax.EmptyEndText != 0 &&
		p.Inst[i.Out].Op == syntax.InstMatch {
		complete = true
	}
	return buf.String(), complete, pc
}

// onePassNext selects the next actionable state of the prog, based on the input character.
// It should only be called when i.Op == InstAlt or InstAltMatch, and from the one-pass machine.
// One of the alternates may ultimately lead without input to end of line. If the instruction
// is InstAltMatch the path to the InstMatch is in i.Out, the normal node in i.Next.
func onePassNext(i *onePassInst, r rune) uint32 {
	next := i.MatchRunePos(r)
	if next >= 0 {
		return i.Next[next]
	}
	if i.Op == syntax.InstAltMatch {
		return i.Out
	}
	return 0
}

func iop(i *syntax.Inst) syntax.InstOp {
	op := i.Op
	switch op {
	case syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
		op = syntax.InstRune
	}
	return op
}

// Sparse Array implementation is used as a queueOnePass.
type queueOnePass struct {
	sparse          []uint32
	dense           []uint32
	size, nextIndex uint32
}

func (q *queueOnePass) empty() bool {
	return q.nextIndex >= q.size
}

func (q *queueOnePass) next() (n uint32) {
	n = q.dense[q.nextIndex]
	q.nextIndex++
	return
}

func (q *queueOnePass) clear() {
	q.size = 0
	q.nextIndex = 0
}

func (q *queueOnePass) contains(u uint32) bool {
	if u >= uint32(len(q.sparse)) {
		return false
	}
	return q.sparse[u] < q.size && q.dense[q.sparse[u]] == u
}

func (q *queueOnePass) insert(u uint32) {
	if !q.contains(u) {
		q.insertNew(u)
	}
}

func (q *queueOnePass) insertNew(u uint32) {
	if u >= uint32(len(q.sparse)) {
		return
	}
	q.sparse[u] = q.size
	q.dense[q.size] = u
	q.size++
}

func newQueue(size int) (q *queueOnePass) {
	return &queueOnePass{
		sparse: make([]uint32, size),
		dense:  make([]uint32, size),
	}
}

// mergeRuneSets merges two non-intersecting runesets, and returns the merged result,
// and a NextIp array. The idea is that if a rune matches the OnePassRunes at index
// i, NextIp[i/2] is the target. If the input sets intersect, an empty runeset and a
// NextIp array with the single element mergeFailed is returned.
// The code assumes that both inputs contain ordered and non-intersecting rune pairs.
const mergeFailed = uint32(0xffffffff)

var (
	noRune = []rune{}
	noNext = []uint32{mergeFailed}
)

func mergeRuneSets(leftRunes, rightRunes *[]rune, leftPC, rightPC uint32) ([]rune, []uint32) {
	leftLen := len(*leftRunes)
	rightLen := len(*rightRunes)
	if leftLen&0x1 != 0 || rightLen&0x1 != 0 {
		panic("mergeRuneSets odd length []rune")
	}
	var (
		lx, rx int
	)
	merged := make([]rune, 0)
	next := make([]uint32, 0)
	ok := true
	defer func() {
		if !ok {
			merged = nil
			next = nil
		}
	}()

	ix := -1
	extend := func(newLow *int, newArray *[]rune, pc uint32) bool {
		if ix > 0 && (*newArray)[*newLow] <= merged[ix] {
			return false
		}
		merged = append(merged, (*newArray)[*newLow], (*newArray)[*newLow+1])
		*newLow += 2
		ix += 2
		next = append(next, pc)
		return true
	}

	for lx < leftLen || rx < rightLen {
		switch {
		case rx >= rightLen:
			ok = extend(&lx, leftRunes, leftPC)
		case lx >= leftLen:
			ok = extend(&rx, rightRunes, rightPC)
		case (*rightRunes)[rx] < (*leftRunes)[lx]:
			ok = extend(&rx, rightRunes, rightPC)
		default:
			ok = extend(&lx, leftRunes, leftPC)
		}
		if !ok {
			return noRune, noNext
		}
	}
	return merged, next
}

// cleanupOnePass drops working memory, and restores certain shortcut instructions.
func cleanupOnePass(prog *onePassProg, original *syntax.Prog) {
	for ix, instOriginal := range original.Inst {
		switch instOriginal.Op {
		case syntax.InstAlt, syntax.InstAltMatch, syntax.InstRune:
		case syntax.InstCapture, syntax.InstEmptyWidth, syntax.InstNop, syntax.InstMatch, syntax.InstFail:
			prog.Inst[ix].Next = nil
		case syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
			prog.Inst[ix].Next = nil
			prog.Inst[ix] = onePassInst{Inst: instOriginal}
		}
	}
}

// onePassCopy creates a copy of the original Prog, as we'll be modifying it.
func onePassCopy(prog *syntax.Prog) *onePassProg {
	p := &onePassProg{
		Start:  prog.Start,
		NumCap: prog.NumCap,
		Inst:   make([]onePassInst, len(prog.Inst)),
	}
	for i, inst := range prog.Inst {
		p.Inst[i] = onePassInst{Inst: inst}
	}

	// rewrites one or more common Prog constructs that enable some otherwise
	// non-onepass Progs to be onepass. A:BD (for example) means an InstAlt at
	// ip A, that points to ips B & C.
	// A:BC + B:DA => A:BC + B:CD
	// A:BC + B:DC => A:DC + B:DC
	for pc := range p.Inst {
		switch p.Inst[pc].Op {
		default:
			continue
		case syntax.InstAlt, syntax.InstAltMatch:
			// A:Bx + B:Ay
			p_A_Other := &p.Inst[pc].Out
			p_A_Alt := &p.Inst[pc].Arg
			// make sure a target is another Alt
			instAlt := p.Inst[*p_A_Alt]
			if !(instAlt.Op == syntax.InstAlt || instAlt.Op == syntax.InstAltMatch) {
				p_A_Alt, p_A_Other = p_A_Other, p_A_Alt
				instAlt = p.Inst[*p_A_Alt]
				if !(instAlt.Op == syntax.InstAlt || instAlt.Op == syntax.InstAltMatch) {
					continue
				}
			}
			instOther := p.Inst[*p_A_Other]
			// Analyzing both legs pointing to Alts is for another day
			if instOther.Op == syntax.InstAlt || instOther.Op == syntax.InstAltMatch {
				// too complicated
				continue
			}
			// simple empty transition loop
			// A:BC + B:DA => A:BC + B:DC
			p_B_Alt := &p.Inst[*p_A_Alt].Out
			p_B_Other := &p.Inst[*p_A_Alt].Arg
			patch := false
			if instAlt.Out == uint32(pc) {
				patch = true
			} else if instAlt.Arg == uint32(pc) {
				patch = true
				p_B_Alt, p_B_Other = p_B_Other, p_B_Alt
			}
			if patch {
				*p_B_Alt = *p_A_Other
			}

			// empty transition to common target
			// A:BC + B:DC => A:DC + B:DC
			if *p_A_Other == *p_B_Alt {
				*p_A_Alt = *p_B_Other
			}
		}
	}
	return p
}

var anyRuneNotNL = []rune{0, '\n' - 1, '\n' + 1, unicode.MaxRune}
var anyRune = []rune{0, unicode.MaxRune}

// makeOnePass creates a onepass Prog, if possible. It is possible if at any alt,
// the match engine can always tell which branch to take. The routine may modify
// p if it is turned into a onepass Prog. If it isn't possible for this to be a
// onepass Prog, the Prog nil is returned. makeOnePass is recursive
// to the size of the Prog.
func makeOnePass(p *onePassProg) *onePassProg {
	// If the machine is very long, it's not worth the time to check if we can use one pass.
	if len(p.Inst) >= 1000 {
		return nil
	}

	var (
		instQueue    = newQueue(len(p.Inst))
		visitQueue   = newQueue(len(p.Inst))
		check        func(uint32, []bool) bool
		onePassRunes = make([][]rune, len(p.Inst))
	)

	// check that paths from Alt instructions are unambiguous, and rebuild the new
	// program as a onepass program
	check = func(pc uint32, m []bool) (ok bool) {
		ok = true
		inst := &p.Inst[pc]
		if visitQueue.contains(pc) {
			return
		}
		visitQueue.insert(pc)
		switch inst.Op {
		case syntax.InstAlt, syntax.InstAltMatch:
			ok = check(inst.Out, m) && check(inst.Arg, m)
			// check no-input paths to InstMatch
			matchOut := m[inst.Out]
			matchArg := m[inst.Arg]
			if matchOut && matchArg {
				ok = false
				break
			}
			// Match on empty goes in inst.Out
			if matchArg {
				inst.Out, inst.Arg = inst.Arg, inst.Out
				matchOut, matchArg = matchArg, matchOut
			}
			if matchOut {
				m[pc] = true
				inst.Op = syntax.InstAltMatch
			}

			// build a dispatch operator from the two legs of the alt.
			onePassRunes[pc], inst.Next = mergeRuneSets(
				&onePassRunes[inst.Out], &onePassRunes[inst.Arg], inst.Out, inst.Arg)
			if len(inst.Next) > 0 && inst.Next[0] == mergeFailed {
				ok = false
				break
			}
		case syntax.InstCapture, syntax.InstNop:
			ok = check(inst.Out, m)
			m[pc] = m[inst.Out]
			// pass matching runes back through these no-ops.
			onePassRunes[pc] = append([]rune{}, onePassRunes[inst.Out]...)
			inst.Next = make([]uint32, len(onePassRunes[pc])/2+1)
			for i := range inst.Next {
				inst.Next[i] = inst.Out
			}
		case syntax.InstEmptyWidth:
			ok = check(inst.Out, m)
			m[pc] = m[inst.Out]
			onePassRunes[pc] = append([]rune{}, onePassRunes[inst.Out]...)
			inst.Next = make([]uint32, len(onePassRunes[pc])/2+1)
			for i := range inst.Next {
				inst.Next[i] = inst.Out
			}
		case syntax.InstMatch, syntax.InstFail:
			m[pc] = inst.Op == syntax.InstMatch
		case syntax.InstRune:
			m[pc] = false
			if len(inst.Next) > 0 {
				break
			}
			instQueue.insert(inst.Out)
			if len(inst.Rune) == 0 {
				onePassRunes[pc] = []rune{}
				inst.Next = []uint32{inst.Out}
				break
			}
			runes := make([]rune, 0)
			if len(inst.Rune) == 1 && syntax.Flags(inst.Arg)&syntax.FoldCase != 0 {
				r0 := inst.Rune[0]
				runes = append(runes, r0, r0)
				for r1 := unicode.SimpleFold(r0); r1 != r0; r1 = unicode.SimpleFold(r1) {
					runes = append(runes, r1, r1)
				}
				slices.Sort(runes)
			} else {
				runes = append(runes, inst.Rune...)
			}
			onePassRunes[pc] = runes
			inst.Next = make([]uint32, len(onePassRunes[pc])/2+1)
			for i := range inst.Next {
				inst.Next[i] = inst.Out
			}
			inst.Op = syntax.InstRune
		case syntax.InstRune1:
			m[pc] = false
			if len(inst.Next) > 0 {
				break
			}
			instQueue.insert(inst.Out)
			runes := []rune{}
			// expand case-folded runes
			if syntax.Flags(inst.Arg)&syntax.FoldCase != 0 {
				r0 := inst.Rune[0]
				runes = append(runes, r0, r0)
				for r1 := unicode.SimpleFold(r0); r1 != r0; r1 = unicode.SimpleFold(r1) {
					runes = append(runes, r1, r1)
				}
				slices.Sort(runes)
			} else {
				runes = append(runes, inst.Rune[0], inst.Rune[0])
			}
			onePassRunes[pc] = runes
			inst.Next = make([]uint32, len(onePassRunes[pc])/2+1)
			for i := range inst.Next {
				inst.Next[i] = inst.Out
			}
			inst.Op = syntax.InstRune
		case syntax.InstRuneAny:
			m[pc] = false
			if len(inst.Next) > 0 {
				break
			}
			instQueue.insert(inst.Out)
			onePassRunes[pc] = append([]rune{}, anyRune...)
			inst.Next = []uint32{inst.Out}
		case syntax.InstRuneAnyNotNL:
			m[pc] = false
			if len(inst.Next) > 0 {
				break
			}
			instQueue.insert(inst.Out)
			onePassRunes[pc] = append([]rune{}, anyRuneNotNL...)
			inst.Next = make([]uint32, len(onePassRunes[pc])/2+1)
			for i := range inst.Next {
				inst.Next[i] = inst.Out
			}
		}
		return
	}

	instQueue.clear()
	instQueue.insert(uint32(p.Start))
	m := make([]bool, len(p.Inst))
	for !instQueue.empty() {
		visitQueue.clear()
		pc := instQueue.next()
		if !check(pc, m) {
			p = nil
			break
		}
	}
	if p != nil {
		for i := range p.Inst {
			p.Inst[i].Rune = onePassRunes[i]
		}
	}
	return p
}

// compileOnePass returns a new *syntax.Prog suitable for onePass execution if the original Prog
// can be recharacterized as a one-pass regexp program, or syntax.nil if the
// Prog cannot be converted. For a one pass prog, the fundamental condition that must
// be true is: at any InstAlt, there must be no ambiguity about what branch to  take.
func compileOnePass(prog *syntax.Prog) (p *onePassProg) {
	if prog.Start == 0 {
		return nil
	}
	// onepass regexp is anchored
	if prog.Inst[prog.Start].Op != syntax.InstEmptyWidth ||
		syntax.EmptyOp(prog.Inst[prog.Start].Arg)&syntax.EmptyBeginText != syntax.EmptyBeginText {
		return nil
	}
	hasAlt := false
	for _, inst := range prog.Inst {
		if inst.Op == syntax.InstAlt || inst.Op == syntax.InstAltMatch {
			hasAlt = true
			break
		}
	}
	// If we have alternates, every instruction leading to InstMatch must be EmptyEndText.
	// Also, any match on empty text must be $.
	for _, inst := range prog.Inst {
		opOut := prog.Inst[inst.Out].Op
		switch inst.Op {
		default:
			if opOut == syntax.InstMatch && hasAlt {
				return nil
			}
		case syntax.InstAlt, syntax.InstAltMatch:
			if opOut == syntax.InstMatch || prog.Inst[inst.Arg].Op == syntax.InstMatch {
				return nil
			}
		case syntax.InstEmptyWidth:
			if opOut == syntax.InstMatch {
				if syntax.EmptyOp(inst.Arg)&syntax.EmptyEndText == syntax.EmptyEndText {
					continue
				}
				return nil
			}
		}
	}
	// Creates a slightly optimized copy of the original Prog
	// that cleans up some Prog idioms that block valid onepass programs
	p = onePassCopy(prog)

	// checkAmbiguity on InstAlts, build onepass Prog if possible
	p = makeOnePass(p)

	if p != nil {
		cleanupOnePass(p, prog)
	}
	return p
}

```

// === FILE: references!/go/src/regexp/regexp.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package regexp implements regular expression search.
//
// The syntax of the regular expressions accepted is the same
// general syntax used by Perl, Python, and other languages.
// More precisely, it is the syntax accepted by RE2 and described at
// https://golang.org/s/re2syntax, except for \C.
// For an overview of the syntax, see the [regexp/syntax] package.
//
// The regexp implementation provided by this package is
// guaranteed to run in time linear in the size of the input.
// (This is a property not guaranteed by most open source
// implementations of regular expressions.) For more information
// about this property, see https://swtch.com/~rsc/regexp/regexp1.html
// or any book about automata theory.
//
// All characters are UTF-8-encoded code points.
// Following [utf8.DecodeRune], each byte of an invalid UTF-8 sequence
// is treated as if it encoded utf8.RuneError (U+FFFD).
//
// There are 24 methods of [Regexp] that match a regular expression and identify
// the matched text. Their names are matched by this regular expression:
//
//	(All|Find|FindAll)(String)?(Submatch)?(Index)?
//
// The ‘All’ variants return an iterator over successive non-overlapping
// matches of the entire expression. The ‘FindAll’ variants return a slice
// of those matches instead. Empty matches abutting a preceding
// match are ignored. The ‘FindAll’ variants take an extra integer argument, n.
// If n >= 0, the function returns at most n matches/submatches;
// otherwise, it returns all of them.
//
// The ‘Find’ variants return only the first match that All or FindAll would return.
//
// If ‘String’ is present, the argument is a string; otherwise it is a []byte.
//
// By default, each returned match is denoted by the substring matching the
// regular expression, of type string or []byte according to the type of the argument.
// If ‘Submatch’ is present, each match is represented instead by a slice of
// the substrings matching the regular expression's parenthesized subexpressions
// (also known as capturing groups), numbered from left to right in order of opening
// parenthesis. Submatch 0 is the match of the entire expression, submatch 1 is
// the match of the first parenthesized subexpression, and so on.
// If ‘Index’ is present, each substring is instead denoted by a pair of byte indexes
// within the input string. If an index is negative or substring is nil, it means that
// the subexpression did not match any string in the input. For ‘String’ versions,
// an empty string means either no match or an empty match.
//
// There is also a subset of the methods that can be applied to text read from
// an [io.RuneReader]: [Regexp.MatchReader], [Regexp.FindReaderIndex],
// [Regexp.FindReaderSubmatchIndex].
// Note that regular expression matches may need to
// examine text beyond the text returned by a match, so the methods that
// match text from an [io.RuneReader] may read arbitrarily far into the input
// before returning.
//
// (There are a few other methods that do not match this pattern.)
package regexp

import (
	"bytes"
	"io"
	"iter"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Regexp is the representation of a compiled regular expression.
// A Regexp is safe for concurrent use by multiple goroutines,
// except for configuration methods, such as [Regexp.Longest].
type Regexp struct {
	expr           string       // as passed to Compile
	prog           *syntax.Prog // compiled program
	onepass        *onePassProg // onepass program or nil
	numSubexp      int
	maxBitStateLen int
	subexpNames    []string
	prefix         string         // required prefix in unanchored matches
	prefixBytes    []byte         // prefix, as a []byte
	prefixRune     rune           // first rune in prefix
	prefixEnd      uint32         // pc for last rune in prefix
	mpool          int            // pool for machines
	matchcap       int            // size of recorded match lengths
	prefixComplete bool           // prefix is the entire regexp
	cond           syntax.EmptyOp // empty-width conditions required at start of match
	minInputLen    int            // minimum length of the input in bytes

	// This field can be modified by the Longest method,
	// but it is otherwise read-only.
	longest bool // whether regexp prefers leftmost-longest match
}

// String returns the source text used to compile the regular expression.
func (re *Regexp) String() string {
	return re.expr
}

// Copy returns a new [Regexp] object copied from re.
// Calling [Regexp.Longest] on one copy does not affect another.
//
// Deprecated: In earlier releases, when using a [Regexp] in multiple goroutines,
// giving each goroutine its own copy helped to avoid lock contention.
// As of Go 1.12, using Copy is no longer necessary to avoid lock contention.
// Copy may still be appropriate if the reason for its use is to make
// two copies with different [Regexp.Longest] settings.
func (re *Regexp) Copy() *Regexp {
	re2 := *re
	return &re2
}

// Compile parses a regular expression and returns, if successful,
// a [Regexp] object that can be used to match against text.
//
// When matching against text, the regexp returns a match that
// begins as early as possible in the input (leftmost), and among those
// it chooses the one that a backtracking search would have found first.
// This so-called leftmost-first matching is the same semantics
// that Perl, Python, and other implementations use, although this
// package implements it without the expense of backtracking.
// For POSIX leftmost-longest matching, see [CompilePOSIX].
func Compile(expr string) (*Regexp, error) {
	return compile(expr, syntax.Perl, false)
}

// CompilePOSIX is like [Compile] but restricts the regular expression
// to POSIX ERE (egrep) syntax and changes the match semantics to
// leftmost-longest.
//
// That is, when matching against text, the regexp returns a match that
// begins as early as possible in the input (leftmost), and among those
// it chooses a match that is as long as possible.
// This so-called leftmost-longest matching is the same semantics
// that early regular expression implementations used and that POSIX
// specifies.
//
// However, there can be multiple leftmost-longest matches, with different
// submatch choices, and here this package diverges from POSIX.
// Among the possible leftmost-longest matches, this package chooses
// the one that a backtracking search would have found first, while POSIX
// specifies that the match be chosen to maximize the length of the first
// subexpression, then the second, and so on from left to right.
// The POSIX rule is computationally prohibitive and not even well-defined.
// See https://swtch.com/~rsc/regexp/regexp2.html#posix for details.
func CompilePOSIX(expr string) (*Regexp, error) {
	return compile(expr, syntax.POSIX, true)
}

// Longest makes future searches prefer the leftmost-longest match.
// That is, when matching against text, the regexp returns a match that
// begins as early as possible in the input (leftmost), and among those
// it chooses a match that is as long as possible.
// This method modifies the [Regexp] and may not be called concurrently
// with any other methods.
func (re *Regexp) Longest() {
	re.longest = true
}

func compile(expr string, mode syntax.Flags, longest bool) (*Regexp, error) {
	re, err := syntax.Parse(expr, mode)
	if err != nil {
		return nil, err
	}
	maxCap := re.MaxCap()
	capNames := re.CapNames()

	re = re.Simplify()
	prog, err := syntax.Compile(re)
	if err != nil {
		return nil, err
	}
	matchcap := prog.NumCap
	if matchcap < 2 {
		matchcap = 2
	}
	regexp := &Regexp{
		expr:        expr,
		prog:        prog,
		onepass:     compileOnePass(prog),
		numSubexp:   maxCap,
		subexpNames: capNames,
		cond:        prog.StartCond(),
		longest:     longest,
		matchcap:    matchcap,
		minInputLen: minInputLen(re),
	}
	if regexp.onepass == nil {
		regexp.prefix, regexp.prefixComplete = prog.Prefix()
		regexp.maxBitStateLen = maxBitStateLen(prog)
	} else {
		regexp.prefix, regexp.prefixComplete, regexp.prefixEnd = onePassPrefix(prog)
	}
	if regexp.prefix != "" {
		// TODO(rsc): Remove this allocation by adding
		// IndexString to package bytes.
		regexp.prefixBytes = []byte(regexp.prefix)
		regexp.prefixRune, _ = utf8.DecodeRuneInString(regexp.prefix)
	}

	n := len(prog.Inst)
	i := 0
	for matchSize[i] != 0 && matchSize[i] < n {
		i++
	}
	regexp.mpool = i

	return regexp, nil
}

// Pools of *machine for use during (*Regexp).find,
// split up by the size of the execution queues.
// matchPool[i] machines have queue size matchSize[i].
// On a 64-bit system each queue entry is 16 bytes,
// so matchPool[0] has 16*2*128 = 4kB queues, etc.
// The final matchPool is a catch-all for very large queues.
var (
	matchSize = [...]int{128, 512, 2048, 16384, 0}
	matchPool [len(matchSize)]sync.Pool
)

// get returns a machine to use for matching re.
// It uses the re's machine cache if possible, to avoid
// unnecessary allocation.
func (re *Regexp) get() *machine {
	m, ok := matchPool[re.mpool].Get().(*machine)
	if !ok {
		m = new(machine)
	}
	m.re = re
	m.p = re.prog
	if cap(m.matchcap) < re.matchcap {
		m.matchcap = make([]int, re.matchcap)
		for _, t := range m.pool {
			t.cap = make([]int, re.matchcap)
		}
	}

	// Allocate queues if needed.
	// Or reallocate, for "large" match pool.
	n := matchSize[re.mpool]
	if n == 0 { // large pool
		n = len(re.prog.Inst)
	}
	if len(m.q0.sparse) < n {
		m.q0 = queue{make([]uint32, n), make([]entry, 0, n)}
		m.q1 = queue{make([]uint32, n), make([]entry, 0, n)}
	}
	return m
}

// put returns a machine to the correct machine pool.
func (re *Regexp) put(m *machine) {
	m.re = nil
	m.p = nil
	m.inputs.clear()
	matchPool[re.mpool].Put(m)
}

// minInputLen walks the regexp to find the minimum length of any matchable input.
func minInputLen(re *syntax.Regexp) int {
	switch re.Op {
	default:
		return 0
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL, syntax.OpCharClass:
		return 1
	case syntax.OpLiteral:
		l := 0
		for _, r := range re.Rune {
			if r == utf8.RuneError {
				l++
			} else {
				l += utf8.RuneLen(r)
			}
		}
		return l
	case syntax.OpCapture, syntax.OpPlus:
		return minInputLen(re.Sub[0])
	case syntax.OpRepeat:
		return re.Min * minInputLen(re.Sub[0])
	case syntax.OpConcat:
		l := 0
		for _, sub := range re.Sub {
			l += minInputLen(sub)
		}
		return l
	case syntax.OpAlternate:
		l := minInputLen(re.Sub[0])
		var lnext int
		for _, sub := range re.Sub[1:] {
			lnext = minInputLen(sub)
			if lnext < l {
				l = lnext
			}
		}
		return l
	}
}

// MustCompile is like [Compile] but panics if the expression cannot be parsed.
// It simplifies safe initialization of global variables holding compiled regular
// expressions.
func MustCompile(str string) *Regexp {
	regexp, err := Compile(str)
	if err != nil {
		panic(`regexp: Compile(` + quote(str) + `): ` + err.Error())
	}
	return regexp
}

// MustCompilePOSIX is like [CompilePOSIX] but panics if the expression cannot be parsed.
// It simplifies safe initialization of global variables holding compiled regular
// expressions.
func MustCompilePOSIX(str string) *Regexp {
	regexp, err := CompilePOSIX(str)
	if err != nil {
		panic(`regexp: CompilePOSIX(` + quote(str) + `): ` + err.Error())
	}
	return regexp
}

func quote(s string) string {
	if strconv.CanBackquote(s) {
		return "`" + s + "`"
	}
	return strconv.Quote(s)
}

// NumSubexp returns the number of parenthesized subexpressions in this [Regexp].
func (re *Regexp) NumSubexp() int {
	return re.numSubexp
}

// SubexpNames returns the names of the parenthesized subexpressions
// in this [Regexp]. The name for the first sub-expression is names[1],
// so that if m is a match slice, the name for m[i] is SubexpNames()[i].
// Since the Regexp as a whole cannot be named, names[0] is always
// the empty string. The slice should not be modified.
func (re *Regexp) SubexpNames() []string {
	return re.subexpNames
}

// SubexpIndex returns the index of the first subexpression with the given name,
// or -1 if there is no subexpression with that name.
//
// Note that multiple subexpressions can be written using the same name, as in
// (?P<bob>a+)(?P<bob>b+), which declares two subexpressions named "bob".
// In this case, SubexpIndex returns the index of the leftmost such subexpression
// in the regular expression.
func (re *Regexp) SubexpIndex(name string) int {
	if name != "" {
		for i, s := range re.subexpNames {
			if name == s {
				return i
			}
		}
	}
	return -1
}

const endOfText rune = -1

// input abstracts different representations of the input text. It provides
// one-character lookahead.
type input interface {
	step(pos int) (r rune, width int) // advance one rune
	canCheckPrefix() bool             // can we look ahead without losing info?
	hasPrefix(re *Regexp) bool
	index(re *Regexp, pos int) int
	context(pos int) lazyFlag
}

// inputString scans a string.
type inputString struct {
	str string
}

func (i *inputString) step(pos int) (rune, int) {
	if pos < len(i.str) {
		return utf8.DecodeRuneInString(i.str[pos:])
	}
	return endOfText, 0
}

func (i *inputString) canCheckPrefix() bool {
	return true
}

func (i *inputString) hasPrefix(re *Regexp) bool {
	return strings.HasPrefix(i.str, re.prefix)
}

func (i *inputString) index(re *Regexp, pos int) int {
	return strings.Index(i.str[pos:], re.prefix)
}

func (i *inputString) context(pos int) lazyFlag {
	r1, r2 := endOfText, endOfText
	// 0 < pos && pos <= len(i.str)
	if uint(pos-1) < uint(len(i.str)) {
		r1, _ = utf8.DecodeLastRuneInString(i.str[:pos])
	}
	// 0 <= pos && pos < len(i.str)
	if uint(pos) < uint(len(i.str)) {
		r2, _ = utf8.DecodeRuneInString(i.str[pos:])
	}
	return newLazyFlag(r1, r2)
}

// inputBytes scans a byte slice.
type inputBytes struct {
	str []byte
}

func (i *inputBytes) step(pos int) (rune, int) {
	if pos < len(i.str) {
		return utf8.DecodeRune(i.str[pos:])
	}
	return endOfText, 0
}

func (i *inputBytes) canCheckPrefix() bool {
	return true
}

func (i *inputBytes) hasPrefix(re *Regexp) bool {
	return bytes.HasPrefix(i.str, re.prefixBytes)
}

func (i *inputBytes) index(re *Regexp, pos int) int {
	return bytes.Index(i.str[pos:], re.prefixBytes)
}

func (i *inputBytes) context(pos int) lazyFlag {
	r1, r2 := endOfText, endOfText
	// 0 < pos && pos <= len(i.str)
	if uint(pos-1) < uint(len(i.str)) {
		r1, _ = utf8.DecodeLastRune(i.str[:pos])
	}
	// 0 <= pos && pos < len(i.str)
	if uint(pos) < uint(len(i.str)) {
		r2, _ = utf8.DecodeRune(i.str[pos:])
	}
	return newLazyFlag(r1, r2)
}

// inputReader scans a RuneReader.
type inputReader struct {
	r     io.RuneReader
	atEOT bool
	pos   int
}

func (i *inputReader) step(pos int) (rune, int) {
	if !i.atEOT && pos != i.pos {
		return endOfText, 0

	}
	r, w, err := i.r.ReadRune()
	if err != nil {
		i.atEOT = true
		return endOfText, 0
	}
	i.pos += w
	return r, w
}

func (i *inputReader) canCheckPrefix() bool {
	return false
}

func (i *inputReader) hasPrefix(re *Regexp) bool {
	return false
}

func (i *inputReader) index(re *Regexp, pos int) int {
	return -1
}

func (i *inputReader) context(pos int) lazyFlag {
	return 0 // not used
}

// LiteralPrefix returns a literal string that must begin any match
// of the regular expression re. It returns the boolean true if the
// literal string comprises the entire regular expression.
func (re *Regexp) LiteralPrefix() (prefix string, complete bool) {
	return re.prefix, re.prefixComplete
}

// MatchReader reports whether the text returned by the [io.RuneReader]
// contains any match of the regular expression re.
func (re *Regexp) MatchReader(r io.RuneReader) bool {
	return re.doMatch(r, nil, "")
}

// MatchString reports whether the string s
// contains any match of the regular expression re.
func (re *Regexp) MatchString(s string) bool {
	return re.doMatch(nil, nil, s)
}

// Match reports whether the byte slice b
// contains any match of the regular expression re.
func (re *Regexp) Match(b []byte) bool {
	return re.doMatch(nil, b, "")
}

// MatchReader reports whether the text returned by the [io.RuneReader]
// contains any match of the regular expression pattern.
// More complicated queries need to use [Compile] and the full [Regexp] interface.
func MatchReader(pattern string, r io.RuneReader) (matched bool, err error) {
	re, err := Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchReader(r), nil
}

// MatchString reports whether the string s
// contains any match of the regular expression pattern.
// More complicated queries need to use [Compile] and the full [Regexp] interface.
func MatchString(pattern string, s string) (matched bool, err error) {
	re, err := Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(s), nil
}

// Match reports whether the byte slice b
// contains any match of the regular expression pattern.
// More complicated queries need to use [Compile] and the full [Regexp] interface.
func Match(pattern string, b []byte) (matched bool, err error) {
	re, err := Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.Match(b), nil
}

// ReplaceAllString returns a copy of src, replacing matches of the [Regexp]
// with the replacement string repl.
// Inside repl, $ signs are interpreted as in [Regexp.Expand].
func (re *Regexp) ReplaceAllString(src, repl string) string {
	n := 2
	if strings.Contains(repl, "$") {
		n = 2 * (re.numSubexp + 1)
	}
	b := re.replaceAll(nil, src, n, func(dst []byte, match []int) []byte {
		return re.expand(dst, repl, nil, src, match)
	})
	return string(b)
}

// ReplaceAllLiteralString returns a copy of src, replacing matches of the [Regexp]
// with the replacement string repl. The replacement repl is substituted directly,
// without using [Regexp.Expand].
func (re *Regexp) ReplaceAllLiteralString(src, repl string) string {
	return string(re.replaceAll(nil, src, 2, func(dst []byte, match []int) []byte {
		return append(dst, repl...)
	}))
}

// ReplaceAllStringFunc returns a copy of src in which all matches of the
// [Regexp] have been replaced by the return value of function repl applied
// to the matched substring. The replacement returned by repl is substituted
// directly, without using [Regexp.Expand].
func (re *Regexp) ReplaceAllStringFunc(src string, repl func(string) string) string {
	b := re.replaceAll(nil, src, 2, func(dst []byte, match []int) []byte {
		return append(dst, repl(src[match[0]:match[1]])...)
	})
	return string(b)
}

func (re *Regexp) replaceAll(bsrc []byte, src string, nmatch int, repl func(dst []byte, m []int) []byte) []byte {
	lastMatchEnd := 0 // end position of the most recent match
	searchPos := 0    // position where we next look for a match
	var buf []byte
	var endPos int
	if bsrc != nil {
		endPos = len(bsrc)
	} else {
		endPos = len(src)
	}
	if nmatch > re.prog.NumCap {
		nmatch = re.prog.NumCap
	}

	var dstCap [2]int
	for searchPos <= endPos {
		a := re.find(nil, bsrc, src, searchPos, nmatch, dstCap[:0])
		if len(a) == 0 {
			break // no more matches
		}

		// Copy the unmatched characters before this match.
		if bsrc != nil {
			buf = append(buf, bsrc[lastMatchEnd:a[0]]...)
		} else {
			buf = append(buf, src[lastMatchEnd:a[0]]...)
		}

		// Now insert a copy of the replacement string, but not for a
		// match of the empty string immediately after another match.
		// (Otherwise, we get double replacement for patterns that
		// match both empty and nonempty strings.)
		if a[1] > lastMatchEnd || a[0] == 0 {
			buf = repl(buf, a)
		}
		lastMatchEnd = a[1]

		// Advance past this match; always advance at least one character.
		var width int
		if bsrc != nil {
			_, width = utf8.DecodeRune(bsrc[searchPos:])
		} else {
			_, width = utf8.DecodeRuneInString(src[searchPos:])
		}
		if searchPos+width > a[1] {
			searchPos += width
		} else if searchPos+1 > a[1] {
			// This clause is only needed at the end of the input
			// string. In that case, DecodeRuneInString returns width=0.
			searchPos++
		} else {
			searchPos = a[1]
		}
	}

	// Copy the unmatched characters after the last match.
	if bsrc != nil {
		buf = append(buf, bsrc[lastMatchEnd:]...)
	} else {
		buf = append(buf, src[lastMatchEnd:]...)
	}

	return buf
}

// ReplaceAll returns a copy of src, replacing matches of the [Regexp]
// with the replacement text repl.
// Inside repl, $ signs are interpreted as in [Regexp.Expand].
func (re *Regexp) ReplaceAll(src, repl []byte) []byte {
	n := 2
	if bytes.IndexByte(repl, '$') >= 0 {
		n = 2 * (re.numSubexp + 1)
	}
	srepl := ""
	b := re.replaceAll(src, "", n, func(dst []byte, match []int) []byte {
		if len(srepl) != len(repl) {
			srepl = string(repl)
		}
		return re.expand(dst, srepl, src, "", match)
	})
	return b
}

// ReplaceAllLiteral returns a copy of src, replacing matches of the [Regexp]
// with the replacement bytes repl. The replacement repl is substituted directly,
// without using [Regexp.Expand].
func (re *Regexp) ReplaceAllLiteral(src, repl []byte) []byte {
	return re.replaceAll(src, "", 2, func(dst []byte, match []int) []byte {
		return append(dst, repl...)
	})
}

// ReplaceAllFunc returns a copy of src in which all matches of the
// [Regexp] have been replaced by the return value of function repl applied
// to the matched byte slice. The replacement returned by repl is substituted
// directly, without using [Regexp.Expand].
func (re *Regexp) ReplaceAllFunc(src []byte, repl func([]byte) []byte) []byte {
	return re.replaceAll(src, "", 2, func(dst []byte, match []int) []byte {
		return append(dst, repl(src[match[0]:match[1]])...)
	})
}

// Bitmap used by func special to check whether a character needs to be escaped.
var specialBytes [16]byte

// special reports whether byte b needs to be escaped by QuoteMeta.
func special(b byte) bool {
	return b < utf8.RuneSelf && specialBytes[b%16]&(1<<(b/16)) != 0
}

func init() {
	for _, b := range []byte(`\.+*?()|[]{}^$`) {
		specialBytes[b%16] |= 1 << (b / 16)
	}
}

// QuoteMeta returns a string that escapes all regular expression metacharacters
// inside the argument text; the returned string is a regular expression matching
// the literal text.
func QuoteMeta(s string) string {
	// A byte loop is correct because all metacharacters are ASCII.
	var i int
	for i = 0; i < len(s); i++ {
		if special(s[i]) {
			break
		}
	}
	// No meta characters found, so return original string.
	if i >= len(s) {
		return s
	}

	b := make([]byte, 2*len(s)-i)
	copy(b, s[:i])
	j := i
	for ; i < len(s); i++ {
		if special(s[i]) {
			b[j] = '\\'
			j++
		}
		b[j] = s[i]
		j++
	}
	return string(b[:j])
}

// The number of capture values in the program may correspond
// to fewer capturing expressions than are in the regexp.
// For example, "(a){0}" turns into an empty program, so the
// maximum capture in the program is 0 but we need to return
// an expression for \1.  Pad appends -1s to the slice a as needed.
func (re *Regexp) pad(a []int) []int {
	if a == nil {
		// No match.
		return nil
	}
	n := (1 + re.numSubexp) * 2
	for len(a) < n {
		a = append(a, -1)
	}
	return a
}

// matches yields the location of successive matches in the input text.
// The input text is b if non-nil, otherwise s.
func (re *Regexp) matches(s string, b []byte, max, ncap int) iter.Seq[[]int] {
	return func(yield func([]int) bool) {
		if max == 0 {
			return
		}
		var end int
		if b == nil {
			end = len(s)
		} else {
			end = len(b)
		}
		var matches []int
		for pos, prevMatchEnd := 0, -1; pos <= end; {
			matches = re.find(nil, b, s, pos, ncap, matches[:0])
			if len(matches) == 0 {
				break
			}

			accept := true
			if matches[1] == pos {
				// We've found an empty match.
				if matches[0] == prevMatchEnd {
					// We don't allow an empty match right
					// after a previous match, so ignore it.
					accept = false
				}
				var width int
				if b == nil {
					is := inputString{str: s}
					_, width = is.step(pos)
				} else {
					ib := inputBytes{str: b}
					_, width = ib.step(pos)
				}
				if width > 0 {
					pos += width
				} else {
					pos = end + 1
				}
			} else {
				pos = matches[1]
			}
			prevMatchEnd = matches[1]

			if accept {
				if !yield(re.pad(matches)) {
					return
				}
				if max > 0 {
					if max--; max == 0 {
						return
					}
				}
			}
		}
	}
}

// Find returns the text of the leftmost match for re in b.
// The return value is nil for no match.
func (re *Regexp) Find(b []byte) []byte {
	var dstCap [2]int
	a := re.find(nil, b, "", 0, 2, dstCap[:0])
	if a == nil {
		return nil
	}
	return b[a[0]:a[1]:a[1]]
}

// FindString returns the text of the leftmost match for re in s.
// The return value is the empty string both for an empty match and for no match.
// To distinguish those two cases, use [Regexp.FindStringIndex] or [Regexp.FindStringSubmatch].
func (re *Regexp) FindString(s string) string {
	var dstCap [2]int
	a := re.find(nil, nil, s, 0, 2, dstCap[:0])
	if a == nil {
		return ""
	}
	return s[a[0]:a[1]]
}

// FindIndex returns the location of the leftmost match for re in b.
// The match itself is at b[m[0]:m[1]].
// The return value is nil for no match.
func (re *Regexp) FindIndex(b []byte) (m []int) {
	m = re.find(nil, b, "", 0, 2, nil)
	if m == nil {
		return nil
	}
	return m[0:2]
}

// FindStringIndex returns the location of the leftmost match for re in s.
// The match itself is at s[m[0]:m[1]].
// The return value is nil for no match.
func (re *Regexp) FindStringIndex(s string) (m []int) {
	m = re.find(nil, nil, s, 0, 2, nil)
	if m == nil {
		return nil
	}
	return m[0:2]
}

// FindReaderIndex returns the location of the leftmost match for re in r.
// The match starts at byte index m[0] and ends just before byte index m[1].
// The return value is nil for no match.
//
// FindReaderIndex may read arbitrarily far from r,
// including reading beyond the returned match.
func (re *Regexp) FindReaderIndex(r io.RuneReader) (m []int) {
	m = re.find(r, nil, "", 0, 2, nil)
	if m == nil {
		return nil
	}
	return m[0:2]
}

// FindSubmatch returns the first match for re in b, including submatches.
// The overall match is m[0], the first submatch is m[1], and so on.
// The return value is nil for no match.
func (re *Regexp) FindSubmatch(b []byte) [][]byte {
	var dstCap [4]int
	m := re.find(nil, b, "", 0, re.prog.NumCap, dstCap[:0])
	if m == nil {
		return nil
	}
	sub := make([][]byte, 1+re.numSubexp)
	for i := range sub {
		if 2*i < len(m) && m[2*i] >= 0 {
			sub[i] = b[m[2*i]:m[2*i+1]:m[2*i+1]]
		}
	}
	return sub
}

// FindStringSubmatch returns the first match for re in s, including submatches.
// The overall match is s[0], the first submatch is s[1], and so on.
// The return value is nil for no match.
func (re *Regexp) FindStringSubmatch(s string) []string {
	var dstCap [4]int
	a := re.find(nil, nil, s, 0, re.prog.NumCap, dstCap[:0])
	if a == nil {
		return nil
	}
	ret := make([]string, 1+re.numSubexp)
	for i := range ret {
		if 2*i < len(a) && a[2*i] >= 0 {
			ret[i] = s[a[2*i]:a[2*i+1]]
		}
	}
	return ret
}

// FindSubmatchIndex returns the first match for re in b, including submatches.
// The overall match is b[m[0]:m[1]], the first submatch is b[m[2]:m[3]], and so on.
// The return value is nil for no match.
func (re *Regexp) FindSubmatchIndex(b []byte) []int {
	return re.pad(re.find(nil, b, "", 0, re.prog.NumCap, nil))
}

// FindStringSubmatchIndex returns the first match for re in s, including submatches.
// The overall match is s[m[0]:m[1]], the first submatch is s[m[2]:m[3]], and so on.
// The return value is nil for no match.
func (re *Regexp) FindStringSubmatchIndex(s string) []int {
	return re.pad(re.find(nil, nil, s, 0, re.prog.NumCap, nil))
}

// FindReaderSubmatchIndex returns the first match for re in r, including submatches.
// The overall match is at byte index m[0] up to m[1],
// the first submatch is at byte index m[2] up to m[3], and so on.
// The return value is nil for no match.
//
// FindReaderSubmatchIndex may read arbitrarily far from r,
// including reading beyond the returned match.
func (re *Regexp) FindReaderSubmatchIndex(r io.RuneReader) []int {
	return re.pad(re.find(r, nil, "", 0, re.prog.NumCap, nil))
}

// all returns at most n matches for re in b.
func (re *Regexp) all(b []byte, n int) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		for m := range re.matches("", b, n, 2) {
			if !yield(b[m[0]:m[1]:m[1]]) {
				break
			}
		}
	}
}

// allString returns at most n matches for re in s.
func (re *Regexp) allString(s string, n int) iter.Seq[string] {
	return func(yield func(string) bool) {
		for m := range re.matches(s, nil, n, 2) {
			if !yield(s[m[0]:m[1]]) {
				break
			}
		}
	}
}

// allIndex returns the locations of at most n matches for re in b.
func (re *Regexp) allIndex(b []byte, n int) iter.Seq[[]int] {
	return func(yield func([]int) bool) {
		for m := range re.matches("", b, n, 2) {
			if !yield([]int{m[0], m[1]}) {
				break
			}
		}
	}
}

// allStringIndex returns the locations of at most n matches for re in s.
func (re *Regexp) allStringIndex(s string, n int) iter.Seq[[]int] {
	return func(yield func([]int) bool) {
		for m := range re.matches(s, nil, n, 2) {
			if !yield([]int{m[0], m[1]}) {
				break
			}
		}
	}
}

// allSubmatch returns the locations of at most n matches for re in b,
// including submatch locations.
func (re *Regexp) allSubmatch(b []byte, n int) iter.Seq[[][]byte] {
	return func(yield func([][]byte) bool) {
		for m := range re.matches("", b, n, re.prog.NumCap) {
			sub := make([][]byte, len(m)/2)
			for i := range sub {
				if m[2*i] >= 0 {
					sub[i] = b[m[2*i]:m[2*i+1]:m[2*i+1]]
				}
			}
			if !yield(sub) {
				break
			}
		}
	}
}

// allStringSubmatch returns the locations of at most n matches for re in s,
// including submatch locations.
func (re *Regexp) allStringSubmatch(s string, n int) iter.Seq[[]string] {
	return func(yield func([]string) bool) {
		for m := range re.matches(s, nil, n, re.prog.NumCap) {
			sub := make([]string, len(m)/2)
			for i := range sub {
				if m[2*i] >= 0 {
					sub[i] = s[m[2*i]:m[2*i+1]]
				}
			}
			if !yield(sub) {
				break
			}
		}
	}
}

// allSubmatchIndex returns the locations of at most n matches for re in b,
// including submatch locations.
func (re *Regexp) allSubmatchIndex(b []byte, n int) iter.Seq[[]int] {
	return func(yield func([]int) bool) {
		for m := range re.matches("", b, n, re.prog.NumCap) {
			if !yield(slices.Clone(m)) {
				break
			}
		}
	}
}

// allStringSubmatchIndex returns the locations of at most n matches for re in s,
// including submatch locations.
func (re *Regexp) allStringSubmatchIndex(s string, n int) iter.Seq[[]int] {
	return func(yield func([]int) bool) {
		for m := range re.matches(s, nil, n, re.prog.NumCap) {
			if !yield(slices.Clone(m)) {
				break
			}
		}
	}
}

// All returns all the matches for re in b.
func (re *Regexp) _All(b []byte) iter.Seq[[]byte] {
	return re.all(b, -1)
}

// AllString returns all the matches for re in s.
func (re *Regexp) _AllString(s string) iter.Seq[string] {
	return re.allString(s, -1)
}

// AllIndex returns the locations of all matches for re in b.
func (re *Regexp) _AllIndex(b []byte) iter.Seq[[]int] {
	return re.allIndex(b, -1)
}

// AllStringIndex returns the locations of all matches for re in s.
func (re *Regexp) _AllStringIndex(s string) iter.Seq[[]int] {
	return re.allStringIndex(s, -1)
}

// AllSubmatch returns the locations of all matches for re in b,
// including submatch locations.
// In each returned match m, the overall match is m[0],
// the first submatch is m[1], and so on.
func (re *Regexp) _AllSubmatch(b []byte) iter.Seq[[][]byte] {
	return re.allSubmatch(b, -1)
}

// AllStringSubmatch returns the locations of all matches for re in s,
// including submatch locations.
// In each returned match m, m[0] is the overall match,
// m[1] is the first submatch, and so on.
func (re *Regexp) _AllStringSubmatch(s string) iter.Seq[[]string] {
	return re.allStringSubmatch(s, -1)
}

// AllSubmatchIndex returns the locations of all matches for re in b,
// including submatch locations.
// In each returned match m, the overall match is b[m[0]:m[1]],
// the first submatch is b[m[2]:m[3]], and so on.
func (re *Regexp) _AllSubmatchIndex(b []byte) iter.Seq[[]int] {
	return re.allSubmatchIndex(b, -1)
}

// AllStringSubmatchIndex returns the locations of all matches for re in s,
// including submatch locations.
// In each returned match m, the overall match is s[m[0]:m[1]],
// the first submatch is s[m[2]:m[3]], and so on.
func (re *Regexp) _AllStringSubmatchIndex(s string) iter.Seq[[]int] {
	return re.allStringSubmatchIndex(s, -1)
}

// FindAll returns all the matches for re in b.
// If n >= 0, FindAll returns no more than n matches.
// See [Regexp.All] for the equivalent iterator form.
func (re *Regexp) FindAll(b []byte, n int) [][]byte {
	return slices.Collect(re.all(b, n))
}

// FindAllString returns all the matches for re in s.
// If n >= 0, FindAllString returns no more than n matches.
// See [Regexp.AllString] for the equivalent iterator form.
func (re *Regexp) FindAllString(s string, n int) []string {
	return slices.Collect(re.allString(s, n))
}

// FindAllIndex returns the locations of all matches for re in b.
// If n >= 0, FindAllIndex returns no more than n matches.
// See [Regexp.AllIndex] for the equivalent iterator form.
func (re *Regexp) FindAllIndex(b []byte, n int) [][]int {
	return slices.Collect(re.allIndex(b, n))
}

// FindAllStringIndex returns the locations of all matches for re in s.
// If n >= 0, FindAllStringIndex returns no more than n matches.
// See [Regexp.AllStringIndex] for the equivalent iterator form.
func (re *Regexp) FindAllStringIndex(s string, n int) [][]int {
	return slices.Collect(re.allStringIndex(s, n))
}

// FindAllSubmatch returns the locations of all matches for re in b,
// including submatch locations.
// In each returned match m, the overall match is m[0],
// the first submatch is m[1], and so on.
// If n >= 0, FindAllSubmatch returns no more than n matches.
// See [Regexp.AllSubmatch] for the equivalent iterator form.
func (re *Regexp) FindAllSubmatch(b []byte, n int) [][][]byte {
	return slices.Collect(re.allSubmatch(b, n))
}

// FindAllStringSubmatch returns the locations of all matches for re in s,
// including submatch locations.
// In each returned match m, m[0] is the overall match,
// m[1] is the first submatch, and so on.
// If n >= 0, FindAllStringSubmatch returns no more than n matches.
// See [Regexp.AllStringSubmatch] for the equivalent iterator form.
func (re *Regexp) FindAllStringSubmatch(s string, n int) [][]string {
	return slices.Collect(re.allStringSubmatch(s, n))
}

// FindAllSubmatchIndex returns the locations of all matches for re in b,
// including submatch locations.
// In each returned match m, the overall match is b[m[0]:m[1]],
// the first submatch is b[m[2]:m[3]], and so on.
// If n >= 0, FindAllSubmatchIndex returns no more than n matches.
// See [Regexp.AllSubmatchIndex] for the equivalent iterator form.
func (re *Regexp) FindAllSubmatchIndex(b []byte, n int) [][]int {
	return slices.Collect(re.allSubmatchIndex(b, n))
}

// FindAllStringSubmatchIndex returns the locations of all matches for re in s,
// including submatch locations.
// In each returned match m, the overall match is s[m[0]:m[1]],
// the first submatch is s[m[2]:m[3]], and so on.
// If n >= 0, FindAllStringSubmatchIndex returns no more than n matches.
// See [Regexp.AllStringSubmatchIndex] for the equivalent iterator form.
func (re *Regexp) FindAllStringSubmatchIndex(s string, n int) [][]int {
	return slices.Collect(re.allStringSubmatchIndex(s, n))
}

// Expand appends template to dst and returns the result; during the
// append, Expand replaces variables in the template with corresponding
// matches drawn from src. The match slice should have been returned by
// [Regexp.FindSubmatchIndex].
//
// In the template, a variable is denoted by a substring of the form
// $name or ${name}, where name is a non-empty sequence of letters,
// digits, and underscores. A purely numeric name like $1 refers to
// the submatch with the corresponding index; other names refer to
// capturing parentheses named with the (?P<name>...) syntax. A
// reference to an out of range or unmatched index or a name that is not
// present in the regular expression is replaced with an empty slice.
//
// In the $name form, name is taken to be as long as possible: $1x is
// equivalent to ${1x}, not ${1}x, and, $10 is equivalent to ${10}, not ${1}0.
//
// To insert a literal $ in the output, use $$ in the template.
func (re *Regexp) Expand(dst []byte, template []byte, src []byte, match []int) []byte {
	return re.expand(dst, string(template), src, "", match)
}

// ExpandString is like [Regexp.Expand] but the template and source are strings.
// It appends to and returns a byte slice in order to give the calling
// code control over allocation.
func (re *Regexp) ExpandString(dst []byte, template string, src string, match []int) []byte {
	return re.expand(dst, template, nil, src, match)
}

func (re *Regexp) expand(dst []byte, template string, bsrc []byte, src string, match []int) []byte {
	for len(template) > 0 {
		before, after, ok := strings.Cut(template, "$")
		if !ok {
			break
		}
		dst = append(dst, before...)
		template = after
		if template != "" && template[0] == '$' {
			// Treat $$ as $.
			dst = append(dst, '$')
			template = template[1:]
			continue
		}
		name, num, rest, ok := extract(template)
		if !ok {
			// Malformed; treat $ as raw text.
			dst = append(dst, '$')
			continue
		}
		template = rest
		if num >= 0 {
			if 2*num+1 < len(match) && match[2*num] >= 0 {
				if bsrc != nil {
					dst = append(dst, bsrc[match[2*num]:match[2*num+1]]...)
				} else {
					dst = append(dst, src[match[2*num]:match[2*num+1]]...)
				}
			}
		} else {
			for i, namei := range re.subexpNames {
				if name == namei && 2*i+1 < len(match) && match[2*i] >= 0 {
					if bsrc != nil {
						dst = append(dst, bsrc[match[2*i]:match[2*i+1]]...)
					} else {
						dst = append(dst, src[match[2*i]:match[2*i+1]]...)
					}
					break
				}
			}
		}
	}
	dst = append(dst, template...)
	return dst
}

// extract returns the name from a leading "name" or "{name}" in str.
// (The $ has already been removed by the caller.)
// If it is a number, extract returns num set to that number; otherwise num = -1.
func extract(str string) (name string, num int, rest string, ok bool) {
	if str == "" {
		return
	}
	brace := false
	if str[0] == '{' {
		brace = true
		str = str[1:]
	}
	i := 0
	for i < len(str) {
		rune, size := utf8.DecodeRuneInString(str[i:])
		if !unicode.IsLetter(rune) && !unicode.IsDigit(rune) && rune != '_' {
			break
		}
		i += size
	}
	if i == 0 {
		// empty name is not okay
		return
	}
	name = str[:i]
	if brace {
		if i >= len(str) || str[i] != '}' {
			// missing closing brace
			return
		}
		i++
	}

	// Parse number.
	num = 0
	for i := 0; i < len(name); i++ {
		if name[i] < '0' || '9' < name[i] || num >= 1e8 {
			num = -1
			break
		}
		num = num*10 + int(name[i]) - '0'
	}
	// Disallow leading zeros.
	if name[0] == '0' && len(name) > 1 {
		num = -1
	}

	rest = str[i:]
	ok = true
	return
}

// Split slices s into substrings separated by the expression and returns a slice of
// the substrings between those expression matches.
//
// The slice returned by this method consists of all the substrings of s
// not contained in the slice returned by [Regexp.FindAllString]. When called on an expression
// that contains no metacharacters, it is equivalent to [strings.SplitN].
//
// Example:
//
//	s := regexp.MustCompile("a*").Split("abaabaccadaaae", 5)
//	// s: ["", "b", "b", "c", "cadaaae"]
//
// The count determines the number of substrings to return:
//   - n > 0: at most n substrings; the last substring will be the unsplit remainder;
//   - n == 0: the result is nil (zero substrings);
//   - n < 0: all substrings.
func (re *Regexp) Split(s string, n int) []string {
	if n == 0 {
		return nil
	}
	if len(re.expr) > 0 && len(s) == 0 {
		return []string{""}
	}

	matches := re.FindAllStringIndex(s, n)
	strings := make([]string, 0, len(matches))

	beg := 0
	end := 0
	for _, match := range matches {
		if n > 0 && len(strings) >= n-1 {
			break
		}

		end = match[0]
		if match[1] != 0 {
			strings = append(strings, s[beg:end])
		}
		beg = match[1]
	}

	if end != len(s) {
		strings = append(strings, s[beg:])
	}

	return strings
}

// AppendText implements [encoding.TextAppender]. The output
// matches that of calling the [Regexp.String] method.
//
// Note that the output is lossy in some cases: This method does not indicate
// POSIX regular expressions (i.e. those compiled by calling [CompilePOSIX]), or
// those for which the [Regexp.Longest] method has been called.
func (re *Regexp) AppendText(b []byte) ([]byte, error) {
	return append(b, re.String()...), nil
}

// MarshalText implements [encoding.TextMarshaler]. The output
// matches that of calling the [Regexp.AppendText] method.
//
// See [Regexp.AppendText] for more information.
func (re *Regexp) MarshalText() ([]byte, error) {
	return re.AppendText(nil)
}

// UnmarshalText implements [encoding.TextUnmarshaler] by calling
// [Compile] on the encoded value.
func (re *Regexp) UnmarshalText(text []byte) error {
	newRE, err := Compile(string(text))
	if err != nil {
		return err
	}
	*re = *newRE
	return nil
}

```

// === FILE: references!/go/src/regexp/syntax/compile.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import "unicode"

// A patchList is a list of instruction pointers that need to be filled in (patched).
// Because the pointers haven't been filled in yet, we can reuse their storage
// to hold the list. It's kind of sleazy, but works well in practice.
// See https://swtch.com/~rsc/regexp/regexp1.html for inspiration.
//
// These aren't really pointers: they're integers, so we can reinterpret them
// this way without using package unsafe. A value l.head denotes
// p.inst[l.head>>1].Out (l.head&1==0) or .Arg (l.head&1==1).
// head == 0 denotes the empty list, okay because we start every program
// with a fail instruction, so we'll never want to point at its output link.
type patchList struct {
	head, tail uint32
}

func makePatchList(n uint32) patchList {
	return patchList{n, n}
}

func (l patchList) patch(p *Prog, val uint32) {
	head := l.head
	for head != 0 {
		i := &p.Inst[head>>1]
		if head&1 == 0 {
			head = i.Out
			i.Out = val
		} else {
			head = i.Arg
			i.Arg = val
		}
	}
}

func (l1 patchList) append(p *Prog, l2 patchList) patchList {
	if l1.head == 0 {
		return l2
	}
	if l2.head == 0 {
		return l1
	}

	i := &p.Inst[l1.tail>>1]
	if l1.tail&1 == 0 {
		i.Out = l2.head
	} else {
		i.Arg = l2.head
	}
	return patchList{l1.head, l2.tail}
}

// A frag represents a compiled program fragment.
type frag struct {
	i        uint32    // index of first instruction
	out      patchList // where to record end instruction
	nullable bool      // whether fragment can match empty string
}

type compiler struct {
	p *Prog
}

// Compile compiles the regexp into a program to be executed.
// The regexp should have been simplified already (returned from re.Simplify).
func Compile(re *Regexp) (*Prog, error) {
	var c compiler
	c.init()
	f := c.compile(re)
	f.out.patch(c.p, c.inst(InstMatch).i)
	c.p.Start = int(f.i)
	return c.p, nil
}

func (c *compiler) init() {
	c.p = new(Prog)
	c.p.NumCap = 2 // implicit ( and ) for whole match $0
	c.inst(InstFail)
}

var anyRuneNotNL = []rune{0, '\n' - 1, '\n' + 1, unicode.MaxRune}
var anyRune = []rune{0, unicode.MaxRune}

func (c *compiler) compile(re *Regexp) frag {
	switch re.Op {
	case OpNoMatch:
		return c.fail()
	case OpEmptyMatch:
		return c.nop()
	case OpLiteral:
		if len(re.Rune) == 0 {
			return c.nop()
		}
		var f frag
		for j := range re.Rune {
			f1 := c.rune(re.Rune[j:j+1], re.Flags)
			if j == 0 {
				f = f1
			} else {
				f = c.cat(f, f1)
			}
		}
		return f
	case OpCharClass:
		return c.rune(re.Rune, re.Flags)
	case OpAnyCharNotNL:
		return c.rune(anyRuneNotNL, 0)
	case OpAnyChar:
		return c.rune(anyRune, 0)
	case OpBeginLine:
		return c.empty(EmptyBeginLine)
	case OpEndLine:
		return c.empty(EmptyEndLine)
	case OpBeginText:
		return c.empty(EmptyBeginText)
	case OpEndText:
		return c.empty(EmptyEndText)
	case OpWordBoundary:
		return c.empty(EmptyWordBoundary)
	case OpNoWordBoundary:
		return c.empty(EmptyNoWordBoundary)
	case OpCapture:
		bra := c.cap(uint32(re.Cap << 1))
		sub := c.compile(re.Sub[0])
		ket := c.cap(uint32(re.Cap<<1 | 1))
		return c.cat(c.cat(bra, sub), ket)
	case OpStar:
		return c.star(c.compile(re.Sub[0]), re.Flags&NonGreedy != 0)
	case OpPlus:
		return c.plus(c.compile(re.Sub[0]), re.Flags&NonGreedy != 0)
	case OpQuest:
		return c.quest(c.compile(re.Sub[0]), re.Flags&NonGreedy != 0)
	case OpConcat:
		if len(re.Sub) == 0 {
			return c.nop()
		}
		var f frag
		for i, sub := range re.Sub {
			if i == 0 {
				f = c.compile(sub)
			} else {
				f = c.cat(f, c.compile(sub))
			}
		}
		return f
	case OpAlternate:
		var f frag
		for _, sub := range re.Sub {
			f = c.alt(f, c.compile(sub))
		}
		return f
	}
	panic("regexp: unhandled case in compile")
}

func (c *compiler) inst(op InstOp) frag {
	// TODO: impose length limit
	f := frag{i: uint32(len(c.p.Inst)), nullable: true}
	c.p.Inst = append(c.p.Inst, Inst{Op: op})
	return f
}

func (c *compiler) nop() frag {
	f := c.inst(InstNop)
	f.out = makePatchList(f.i << 1)
	return f
}

func (c *compiler) fail() frag {
	return frag{}
}

func (c *compiler) cap(arg uint32) frag {
	f := c.inst(InstCapture)
	f.out = makePatchList(f.i << 1)
	c.p.Inst[f.i].Arg = arg

	if c.p.NumCap < int(arg)+1 {
		c.p.NumCap = int(arg) + 1
	}
	return f
}

func (c *compiler) cat(f1, f2 frag) frag {
	// concat of failure is failure
	if f1.i == 0 || f2.i == 0 {
		return frag{}
	}

	// TODO: elide nop

	f1.out.patch(c.p, f2.i)
	return frag{f1.i, f2.out, f1.nullable && f2.nullable}
}

func (c *compiler) alt(f1, f2 frag) frag {
	// alt of failure is other
	if f1.i == 0 {
		return f2
	}
	if f2.i == 0 {
		return f1
	}

	f := c.inst(InstAlt)
	i := &c.p.Inst[f.i]
	i.Out = f1.i
	i.Arg = f2.i
	f.out = f1.out.append(c.p, f2.out)
	f.nullable = f1.nullable || f2.nullable
	return f
}

func (c *compiler) quest(f1 frag, nongreedy bool) frag {
	f := c.inst(InstAlt)
	i := &c.p.Inst[f.i]
	if nongreedy {
		i.Arg = f1.i
		f.out = makePatchList(f.i << 1)
	} else {
		i.Out = f1.i
		f.out = makePatchList(f.i<<1 | 1)
	}
	f.out = f.out.append(c.p, f1.out)
	return f
}

// loop returns the fragment for the main loop of a plus or star.
// For plus, it can be used after changing the entry to f1.i.
// For star, it can be used directly when f1 can't match an empty string.
// (When f1 can match an empty string, f1* must be implemented as (f1+)?
// to get the priority match order correct.)
func (c *compiler) loop(f1 frag, nongreedy bool) frag {
	f := c.inst(InstAlt)
	i := &c.p.Inst[f.i]
	if nongreedy {
		i.Arg = f1.i
		f.out = makePatchList(f.i << 1)
	} else {
		i.Out = f1.i
		f.out = makePatchList(f.i<<1 | 1)
	}
	f1.out.patch(c.p, f.i)
	return f
}

func (c *compiler) star(f1 frag, nongreedy bool) frag {
	if f1.nullable {
		// Use (f1+)? to get priority match order correct.
		// See golang.org/issue/46123.
		return c.quest(c.plus(f1, nongreedy), nongreedy)
	}
	return c.loop(f1, nongreedy)
}

func (c *compiler) plus(f1 frag, nongreedy bool) frag {
	return frag{f1.i, c.loop(f1, nongreedy).out, f1.nullable}
}

func (c *compiler) empty(op EmptyOp) frag {
	f := c.inst(InstEmptyWidth)
	c.p.Inst[f.i].Arg = uint32(op)
	f.out = makePatchList(f.i << 1)
	return f
}

func (c *compiler) rune(r []rune, flags Flags) frag {
	f := c.inst(InstRune)
	f.nullable = false
	i := &c.p.Inst[f.i]
	i.Rune = r
	flags &= FoldCase // only relevant flag is FoldCase
	if len(r) != 1 || unicode.SimpleFold(r[0]) == r[0] {
		// and sometimes not even that
		flags &^= FoldCase
	}
	i.Arg = uint32(flags)
	f.out = makePatchList(f.i << 1)

	// Special cases for exec machine.
	switch {
	case flags&FoldCase == 0 && (len(r) == 1 || len(r) == 2 && r[0] == r[1]):
		i.Op = InstRune1
	case len(r) == 2 && r[0] == 0 && r[1] == unicode.MaxRune:
		i.Op = InstRuneAny
	case len(r) == 4 && r[0] == 0 && r[1] == '\n'-1 && r[2] == '\n'+1 && r[3] == unicode.MaxRune:
		i.Op = InstRuneAnyNotNL
	}

	return f
}

```

// === FILE: references!/go/src/regexp/syntax/doc.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by mksyntaxgo from the RE2 distribution. DO NOT EDIT.

/*
Package syntax parses regular expressions into parse trees and compiles
parse trees into programs. Most clients of regular expressions will use the
facilities of package [regexp] (such as [regexp.Compile] and [regexp.Match]) instead of this package.

# Syntax

The regular expression syntax understood by this package when parsing with the [Perl] flag is as follows.
Parts of the syntax can be disabled by passing alternate flags to [Parse].

Single characters:

	.              any character, possibly including newline (flag s=true)
	[xyz]          character class
	[^xyz]         negated character class
	\d             Perl character class
	\D             negated Perl character class
	[[:alpha:]]    ASCII character class
	[[:^alpha:]]   negated ASCII character class
	\pN            Unicode character class (one-letter name)
	\p{Greek}      Unicode character class
	\PN            negated Unicode character class (one-letter name)
	\P{Greek}      negated Unicode character class

Composites:

	xy             x followed by y
	x|y            x or y (prefer x)

Repetitions:

	x*             zero or more x, prefer more
	x+             one or more x, prefer more
	x?             zero or one x, prefer one
	x{n,m}         n or n+1 or ... or m x, prefer more
	x{n,}          n or more x, prefer more
	x{n}           exactly n x
	x*?            zero or more x, prefer fewer
	x+?            one or more x, prefer fewer
	x??            zero or one x, prefer zero
	x{n,m}?        n or n+1 or ... or m x, prefer fewer
	x{n,}?         n or more x, prefer fewer
	x{n}?          exactly n x

Implementation restriction: The counting forms x{n,m}, x{n,}, and x{n}
reject forms that create a minimum or maximum repetition count above 1000.
Unlimited repetitions are not subject to this restriction.

Grouping:

	(re)           numbered capturing group (submatch)
	(?P<name>re)   named & numbered capturing group (submatch)
	(?<name>re)    named & numbered capturing group (submatch)
	(?:re)         non-capturing group
	(?flags)       set flags within current group; non-capturing
	(?flags:re)    set flags during re; non-capturing

	Flag syntax is xyz (set) or -xyz (clear) or xy-z (set xy, clear z). The flags are:

	i              case-insensitive (default false)
	m              multi-line mode: ^ and $ match begin/end line in addition to begin/end text (default false)
	s              let . match \n (default false)
	U              ungreedy: swap meaning of x* and x*?, x+ and x+?, etc (default false)

Empty strings:

	^              at beginning of text or line (flag m=true)
	$              at end of text (like \z not \Z) or line (flag m=true)
	\A             at beginning of text
	\b             at ASCII word boundary (\w on one side and \W, \A, or \z on the other)
	\B             not at ASCII word boundary
	\z             at end of text

Escape sequences:

	\a             bell (== \007)
	\f             form feed (== \014)
	\t             horizontal tab (== \011)
	\n             newline (== \012)
	\r             carriage return (== \015)
	\v             vertical tab character (== \013)
	\*             literal *, for any punctuation character *
	\123           octal character code (up to three digits)
	\x7F           hex character code (exactly two digits)
	\x{10FFFF}     hex character code
	\Q...\E        literal text ... even if ... has punctuation

Character class elements:

	x              single character
	A-Z            character range (inclusive)
	\d             Perl character class
	[:foo:]        ASCII character class foo
	\p{Foo}        Unicode character class Foo
	\pF            Unicode character class F (one-letter name)

Named character classes as character class elements:

	[\d]           digits (== \d)
	[^\d]          not digits (== \D)
	[\D]           not digits (== \D)
	[^\D]          not not digits (== \d)
	[[:name:]]     named ASCII class inside character class (== [:name:])
	[^[:name:]]    named ASCII class inside negated character class (== [:^name:])
	[\p{Name}]     named Unicode property inside character class (== \p{Name})
	[^\p{Name}]    named Unicode property inside negated character class (== \P{Name})

Perl character classes (all ASCII-only):

	\d             digits (== [0-9])
	\D             not digits (== [^0-9])
	\s             whitespace (== [\t\n\f\r ])
	\S             not whitespace (== [^\t\n\f\r ])
	\w             word characters (== [0-9A-Za-z_])
	\W             not word characters (== [^0-9A-Za-z_])

ASCII character classes:

	[[:alnum:]]    alphanumeric (== [0-9A-Za-z])
	[[:alpha:]]    alphabetic (== [A-Za-z])
	[[:ascii:]]    ASCII (== [\x00-\x7F])
	[[:blank:]]    blank (== [\t ])
	[[:cntrl:]]    control (== [\x00-\x1F\x7F])
	[[:digit:]]    digits (== [0-9])
	[[:graph:]]    graphical (== [!-~] == [A-Za-z0-9!"#$%&'()*+,\-./:;<=>?@[\\\]^_`{|}~])
	[[:lower:]]    lower case (== [a-z])
	[[:print:]]    printable (== [ -~] == [ [:graph:]])
	[[:punct:]]    punctuation (== [!-/:-@[-`{-~])
	[[:space:]]    whitespace (== [\t\n\v\f\r ])
	[[:upper:]]    upper case (== [A-Z])
	[[:word:]]     word characters (== [0-9A-Za-z_])
	[[:xdigit:]]   hex digit (== [0-9A-Fa-f])

Unicode character classes are those in [unicode.Categories],
[unicode.CategoryAliases], and [unicode.Scripts].
*/
package syntax

```

// === FILE: references!/go/src/regexp/syntax/make_perl_groups.pl ===
```text
#!/usr/bin/perl
# Copyright 2008 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Modified version of RE2's make_perl_groups.pl.

# Generate table entries giving character ranges
# for POSIX/Perl character classes.  Rather than
# figure out what the definition is, it is easier to ask
# Perl about each letter from 0-128 and write down
# its answer.

use strict;
use warnings;

my @posixclasses = (
	"[:alnum:]",
	"[:alpha:]",
	"[:ascii:]",
	"[:blank:]",
	"[:cntrl:]",
	"[:digit:]",
	"[:graph:]",
	"[:lower:]",
	"[:print:]",
	"[:punct:]",
	"[:space:]",
	"[:upper:]",
	"[:word:]",
	"[:xdigit:]",
);

my @perlclasses = (
	"\\d",
	"\\s",
	"\\w",
);

my %overrides = (
	# Prior to Perl 5.18, \s did not match vertical tab.
	# RE2 preserves that original behaviour.
	"\\s:11" => 0,
);

sub ComputeClass($) {
  my @ranges;
  my ($class) = @_;
  my $regexp = "[$class]";
  my $start = -1;
  for (my $i=0; $i<=129; $i++) {
    if ($i == 129) { $i = 256; }
    if ($i <= 128 && ($overrides{"$class:$i"} // chr($i) =~ $regexp)) {
      if ($start < 0) {
        $start = $i;
      }
    } else {
      if ($start >= 0) {
        push @ranges, [$start, $i-1];
      }
      $start = -1;
    }
  }
  return @ranges;
}

sub PrintClass($$@) {
  my ($cname, $name, @ranges) = @_;
  print "var code$cname = []rune{  /* $name */\n";
  for (my $i=0; $i<@ranges; $i++) {
    my @a = @{$ranges[$i]};
    printf "\t0x%x, 0x%x,\n", $a[0], $a[1];
  }
  print "}\n\n";
  my $n = @ranges;
  my $negname = $name;
  if ($negname =~ /:/) {
    $negname =~ s/:/:^/;
  } else {
    $negname =~ y/a-z/A-Z/;
  }
  return "\t`$name`: {+1, code$cname},\n" .
  	"\t`$negname`: {-1, code$cname},\n";
}

my $gen = 0;

sub PrintClasses($@) {
  my ($cname, @classes) = @_;
  my @entries;
  foreach my $cl (@classes) {
    my @ranges = ComputeClass($cl);
    push @entries, PrintClass(++$gen, $cl, @ranges);
  }
  print "var ${cname}Group = map[string]charGroup{\n";
  foreach my $e (@entries) {
    print $e;
  }
  print "}\n";
  my $count = @entries;
}

# Prepare gofmt command
my $gofmt;

if (@ARGV > 0 && $ARGV[0] =~ /\.go$/) {
  # Send the output of gofmt to the given file
  open($gofmt, '|-', 'gofmt >'.$ARGV[0]) or die;
} else {
  open($gofmt, '|-', 'gofmt') or die;
}

# Redirect STDOUT to gofmt input
select $gofmt;

print <<EOF;
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by make_perl_groups.pl; DO NOT EDIT.

package syntax

EOF

PrintClasses("perl", @perlclasses);
PrintClasses("posix", @posixclasses);

```

// === FILE: references!/go/src/regexp/syntax/op_string.go ===
```go
// Code generated by "stringer -type Op -trimprefix Op"; DO NOT EDIT.

package syntax

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[OpNoMatch-1]
	_ = x[OpEmptyMatch-2]
	_ = x[OpLiteral-3]
	_ = x[OpCharClass-4]
	_ = x[OpAnyCharNotNL-5]
	_ = x[OpAnyChar-6]
	_ = x[OpBeginLine-7]
	_ = x[OpEndLine-8]
	_ = x[OpBeginText-9]
	_ = x[OpEndText-10]
	_ = x[OpWordBoundary-11]
	_ = x[OpNoWordBoundary-12]
	_ = x[OpCapture-13]
	_ = x[OpStar-14]
	_ = x[OpPlus-15]
	_ = x[OpQuest-16]
	_ = x[OpRepeat-17]
	_ = x[OpConcat-18]
	_ = x[OpAlternate-19]
	_ = x[opPseudo-128]
}

const (
	_Op_name_0 = "NoMatchEmptyMatchLiteralCharClassAnyCharNotNLAnyCharBeginLineEndLineBeginTextEndTextWordBoundaryNoWordBoundaryCaptureStarPlusQuestRepeatConcatAlternate"
	_Op_name_1 = "opPseudo"
)

var (
	_Op_index_0 = [...]uint8{0, 7, 17, 24, 33, 45, 52, 61, 68, 77, 84, 96, 110, 117, 121, 125, 130, 136, 142, 151}
)

func (i Op) String() string {
	switch {
	case 1 <= i && i <= 19:
		i -= 1
		return _Op_name_0[_Op_index_0[i]:_Op_index_0[i+1]]
	case i == 128:
		return _Op_name_1
	default:
		return "Op(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}

```

// === FILE: references!/go/src/regexp/syntax/parse.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import (
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// An Error describes a failure to parse a regular expression
// and gives the offending expression.
type Error struct {
	Code ErrorCode
	Expr string
}

func (e *Error) Error() string {
	return "error parsing regexp: " + e.Code.String() + ": `" + e.Expr + "`"
}

// An ErrorCode describes a failure to parse a regular expression.
type ErrorCode string

const (
	// Unexpected error
	ErrInternalError ErrorCode = "regexp/syntax: internal error"

	// Parse errors
	ErrInvalidCharClass      ErrorCode = "invalid character class"
	ErrInvalidCharRange      ErrorCode = "invalid character class range"
	ErrInvalidEscape         ErrorCode = "invalid escape sequence"
	ErrInvalidNamedCapture   ErrorCode = "invalid named capture"
	ErrInvalidPerlOp         ErrorCode = "invalid or unsupported Perl syntax"
	ErrInvalidRepeatOp       ErrorCode = "invalid nested repetition operator"
	ErrInvalidRepeatSize     ErrorCode = "invalid repeat count"
	ErrInvalidUTF8           ErrorCode = "invalid UTF-8"
	ErrMissingBracket        ErrorCode = "missing closing ]"
	ErrMissingParen          ErrorCode = "missing closing )"
	ErrMissingRepeatArgument ErrorCode = "missing argument to repetition operator"
	ErrTrailingBackslash     ErrorCode = "trailing backslash at end of expression"
	ErrUnexpectedParen       ErrorCode = "unexpected )"
	ErrNestingDepth          ErrorCode = "expression nests too deeply"
	ErrLarge                 ErrorCode = "expression too large"
)

func (e ErrorCode) String() string {
	return string(e)
}

// Flags control the behavior of the parser and record information about regexp context.
type Flags uint16

const (
	FoldCase      Flags = 1 << iota // case-insensitive match
	Literal                         // treat pattern as literal string
	ClassNL                         // allow character classes like [^a-z] and [[:space:]] to match newline
	DotNL                           // allow . to match newline
	OneLine                         // treat ^ and $ as only matching at beginning and end of text
	NonGreedy                       // make repetition operators default to non-greedy
	PerlX                           // allow Perl extensions
	UnicodeGroups                   // allow \p{Han}, \P{Han} for Unicode group and negation
	WasDollar                       // regexp OpEndText was $, not \z
	Simple                          // regexp contains no counted repetition

	MatchNL = ClassNL | DotNL

	Perl        = ClassNL | OneLine | PerlX | UnicodeGroups // as close to Perl as possible
	POSIX Flags = 0                                         // POSIX syntax
)

// Pseudo-ops for parsing stack.
const (
	opLeftParen = opPseudo + iota
	opVerticalBar
)

// maxHeight is the maximum height of a regexp parse tree.
// It is somewhat arbitrarily chosen, but the idea is to be large enough
// that no one will actually hit in real use but at the same time small enough
// that recursion on the Regexp tree will not hit the 1GB Go stack limit.
// The maximum amount of stack for a single recursive frame is probably
// closer to 1kB, so this could potentially be raised, but it seems unlikely
// that people have regexps nested even this deeply.
// We ran a test on Google's C++ code base and turned up only
// a single use case with depth > 100; it had depth 128.
// Using depth 1000 should be plenty of margin.
// As an optimization, we don't even bother calculating heights
// until we've allocated at least maxHeight Regexp structures.
const maxHeight = 1000

// maxSize is the maximum size of a compiled regexp in Insts.
// It too is somewhat arbitrarily chosen, but the idea is to be large enough
// to allow significant regexps while at the same time small enough that
// the compiled form will not take up too much memory.
// 128 MB is enough for a 3.3 million Inst structures, which roughly
// corresponds to a 3.3 MB regexp.
const (
	maxSize  = 128 << 20 / instSize
	instSize = 5 * 8 // byte, 2 uint32, slice is 5 64-bit words
)

// maxRunes is the maximum number of runes allowed in a regexp tree
// counting the runes in all the nodes.
// Ignoring character classes p.numRunes is always less than the length of the regexp.
// Character classes can make it much larger: each \pL adds 1292 runes.
// 128 MB is enough for 32M runes, which is over 26k \pL instances.
// Note that repetitions do not make copies of the rune slices,
// so \pL{1000} is only one rune slice, not 1000.
// We could keep a cache of character classes we've seen,
// so that all the \pL we see use the same rune list,
// but that doesn't remove the problem entirely:
// consider something like [\pL01234][\pL01235][\pL01236]...[\pL^&*()].
// And because the Rune slice is exposed directly in the Regexp,
// there is not an opportunity to change the representation to allow
// partial sharing between different character classes.
// So the limit is the best we can do.
const (
	maxRunes = 128 << 20 / runeSize
	runeSize = 4 // rune is int32
)

type parser struct {
	flags       Flags     // parse mode flags
	stack       []*Regexp // stack of parsed expressions
	free        *Regexp
	numCap      int // number of capturing groups seen
	wholeRegexp string
	tmpClass    []rune            // temporary char class work space
	numRegexp   int               // number of regexps allocated
	numRunes    int               // number of runes in char classes
	repeats     int64             // product of all repetitions seen
	height      map[*Regexp]int   // regexp height, for height limit check
	size        map[*Regexp]int64 // regexp compiled size, for size limit check
}

func (p *parser) newRegexp(op Op) *Regexp {
	re := p.free
	if re != nil {
		p.free = re.Sub0[0]
		*re = Regexp{}
	} else {
		re = new(Regexp)
		p.numRegexp++
	}
	re.Op = op
	return re
}

func (p *parser) reuse(re *Regexp) {
	if p.height != nil {
		delete(p.height, re)
	}
	re.Sub0[0] = p.free
	p.free = re
}

func (p *parser) checkLimits(re *Regexp) {
	if p.numRunes > maxRunes {
		panic(ErrLarge)
	}
	p.checkSize(re)
	p.checkHeight(re)
}

func (p *parser) checkSize(re *Regexp) {
	if p.size == nil {
		// We haven't started tracking size yet.
		// Do a relatively cheap check to see if we need to start.
		// Maintain the product of all the repeats we've seen
		// and don't track if the total number of regexp nodes
		// we've seen times the repeat product is in budget.
		if p.repeats == 0 {
			p.repeats = 1
		}
		if re.Op == OpRepeat {
			n := re.Max
			if n == -1 {
				n = re.Min
			}
			if n <= 0 {
				n = 1
			}
			if int64(n) > maxSize/p.repeats {
				p.repeats = maxSize
			} else {
				p.repeats *= int64(n)
			}
		}
		if int64(p.numRegexp) < maxSize/p.repeats {
			return
		}

		// We need to start tracking size.
		// Make the map and belatedly populate it
		// with info about everything we've constructed so far.
		p.size = make(map[*Regexp]int64)
		for _, re := range p.stack {
			p.checkSize(re)
		}
	}

	if p.calcSize(re, true) > maxSize {
		panic(ErrLarge)
	}
}

func (p *parser) calcSize(re *Regexp, force bool) int64 {
	if !force {
		if size, ok := p.size[re]; ok {
			return size
		}
	}

	var size int64
	switch re.Op {
	case OpLiteral:
		size = int64(len(re.Rune))
	case OpCapture, OpStar:
		// star can be 1+ or 2+; assume 2 pessimistically
		size = 2 + p.calcSize(re.Sub[0], false)
	case OpPlus, OpQuest:
		size = 1 + p.calcSize(re.Sub[0], false)
	case OpConcat:
		for _, sub := range re.Sub {
			size += p.calcSize(sub, false)
		}
	case OpAlternate:
		for _, sub := range re.Sub {
			size += p.calcSize(sub, false)
		}
		if len(re.Sub) > 1 {
			size += int64(len(re.Sub)) - 1
		}
	case OpRepeat:
		sub := p.calcSize(re.Sub[0], false)
		if re.Max == -1 {
			if re.Min == 0 {
				size = 2 + sub // x*
			} else {
				size = 1 + int64(re.Min)*sub // xxx+
			}
			break
		}
		// x{2,5} = xx(x(x(x)?)?)?
		size = int64(re.Max)*sub + int64(re.Max-re.Min)
	}

	size = max(1, size)
	p.size[re] = size
	return size
}

func (p *parser) checkHeight(re *Regexp) {
	if p.numRegexp < maxHeight {
		return
	}
	if p.height == nil {
		p.height = make(map[*Regexp]int)
		for _, re := range p.stack {
			p.checkHeight(re)
		}
	}
	if p.calcHeight(re, true) > maxHeight {
		panic(ErrNestingDepth)
	}
}

func (p *parser) calcHeight(re *Regexp, force bool) int {
	if !force {
		if h, ok := p.height[re]; ok {
			return h
		}
	}
	h := 1
	for _, sub := range re.Sub {
		hsub := p.calcHeight(sub, false)
		if h < 1+hsub {
			h = 1 + hsub
		}
	}
	p.height[re] = h
	return h
}

// Parse stack manipulation.

// push pushes the regexp re onto the parse stack and returns the regexp.
func (p *parser) push(re *Regexp) *Regexp {
	p.numRunes += len(re.Rune)
	if re.Op == OpCharClass && len(re.Rune) == 2 && re.Rune[0] == re.Rune[1] {
		// Single rune.
		if p.maybeConcat(re.Rune[0], p.flags&^FoldCase) {
			return nil
		}
		re.Op = OpLiteral
		re.Rune = re.Rune[:1]
		re.Flags = p.flags &^ FoldCase
	} else if re.Op == OpCharClass && len(re.Rune) == 4 &&
		re.Rune[0] == re.Rune[1] && re.Rune[2] == re.Rune[3] &&
		unicode.SimpleFold(re.Rune[0]) == re.Rune[2] &&
		unicode.SimpleFold(re.Rune[2]) == re.Rune[0] ||
		re.Op == OpCharClass && len(re.Rune) == 2 &&
			re.Rune[0]+1 == re.Rune[1] &&
			unicode.SimpleFold(re.Rune[0]) == re.Rune[1] &&
			unicode.SimpleFold(re.Rune[1]) == re.Rune[0] {
		// Case-insensitive rune like [Aa] or [Δδ].
		if p.maybeConcat(re.Rune[0], p.flags|FoldCase) {
			return nil
		}

		// Rewrite as (case-insensitive) literal.
		re.Op = OpLiteral
		re.Rune = re.Rune[:1]
		re.Flags = p.flags | FoldCase
	} else {
		// Incremental concatenation.
		p.maybeConcat(-1, 0)
	}

	p.stack = append(p.stack, re)
	p.checkLimits(re)
	return re
}

// maybeConcat implements incremental concatenation
// of literal runes into string nodes. The parser calls this
// before each push, so only the top fragment of the stack
// might need processing. Since this is called before a push,
// the topmost literal is no longer subject to operators like *
// (Otherwise ab* would turn into (ab)*.)
// If r >= 0 and there's a node left over, maybeConcat uses it
// to push r with the given flags.
// maybeConcat reports whether r was pushed.
func (p *parser) maybeConcat(r rune, flags Flags) bool {
	n := len(p.stack)
	if n < 2 {
		return false
	}

	re1 := p.stack[n-1]
	re2 := p.stack[n-2]
	if re1.Op != OpLiteral || re2.Op != OpLiteral || re1.Flags&FoldCase != re2.Flags&FoldCase {
		return false
	}

	// Push re1 into re2.
	re2.Rune = append(re2.Rune, re1.Rune...)

	// Reuse re1 if possible.
	if r >= 0 {
		re1.Rune = re1.Rune0[:1]
		re1.Rune[0] = r
		re1.Flags = flags
		return true
	}

	p.stack = p.stack[:n-1]
	p.reuse(re1)
	return false // did not push r
}

// literal pushes a literal regexp for the rune r on the stack.
func (p *parser) literal(r rune) {
	re := p.newRegexp(OpLiteral)
	re.Flags = p.flags
	if p.flags&FoldCase != 0 {
		r = minFoldRune(r)
	}
	re.Rune0[0] = r
	re.Rune = re.Rune0[:1]
	p.push(re)
}

// minFoldRune returns the minimum rune fold-equivalent to r.
func minFoldRune(r rune) rune {
	if r < minFold || r > maxFold {
		return r
	}
	m := r
	r0 := r
	for r = unicode.SimpleFold(r); r != r0; r = unicode.SimpleFold(r) {
		m = min(m, r)
	}
	return m
}

// op pushes a regexp with the given op onto the stack
// and returns that regexp.
func (p *parser) op(op Op) *Regexp {
	re := p.newRegexp(op)
	re.Flags = p.flags
	return p.push(re)
}

// repeat replaces the top stack element with itself repeated according to op, min, max.
// before is the regexp suffix starting at the repetition operator.
// after is the regexp suffix following after the repetition operator.
// repeat returns an updated 'after' and an error, if any.
func (p *parser) repeat(op Op, min, max int, before, after, lastRepeat string) (string, error) {
	flags := p.flags
	if p.flags&PerlX != 0 {
		if len(after) > 0 && after[0] == '?' {
			after = after[1:]
			flags ^= NonGreedy
		}
		if lastRepeat != "" {
			// In Perl it is not allowed to stack repetition operators:
			// a** is a syntax error, not a doubled star, and a++ means
			// something else entirely, which we don't support!
			return "", &Error{ErrInvalidRepeatOp, lastRepeat[:len(lastRepeat)-len(after)]}
		}
	}
	n := len(p.stack)
	if n == 0 {
		return "", &Error{ErrMissingRepeatArgument, before[:len(before)-len(after)]}
	}
	sub := p.stack[n-1]
	if sub.Op >= opPseudo {
		return "", &Error{ErrMissingRepeatArgument, before[:len(before)-len(after)]}
	}

	re := p.newRegexp(op)
	re.Min = min
	re.Max = max
	re.Flags = flags
	re.Sub = re.Sub0[:1]
	re.Sub[0] = sub
	p.stack[n-1] = re
	p.checkLimits(re)

	if op == OpRepeat && (min >= 2 || max >= 2) && !repeatIsValid(re, 1000) {
		return "", &Error{ErrInvalidRepeatSize, before[:len(before)-len(after)]}
	}

	return after, nil
}

// repeatIsValid reports whether the repetition re is valid.
// Valid means that the combination of the top-level repetition
// and any inner repetitions does not exceed n copies of the
// innermost thing.
// This function rewalks the regexp tree and is called for every repetition,
// so we have to worry about inducing quadratic behavior in the parser.
// We avoid this by only calling repeatIsValid when min or max >= 2.
// In that case the depth of any >= 2 nesting can only get to 9 without
// triggering a parse error, so each subtree can only be rewalked 9 times.
func repeatIsValid(re *Regexp, n int) bool {
	if re.Op == OpRepeat {
		m := re.Max
		if m == 0 {
			return true
		}
		if m < 0 {
			m = re.Min
		}
		if m > n {
			return false
		}
		if m > 0 {
			n /= m
		}
	}
	for _, sub := range re.Sub {
		if !repeatIsValid(sub, n) {
			return false
		}
	}
	return true
}

// concat replaces the top of the stack (above the topmost '|' or '(') with its concatenation.
func (p *parser) concat() *Regexp {
	p.maybeConcat(-1, 0)

	// Scan down to find pseudo-operator | or (.
	i := len(p.stack)
	for i > 0 && p.stack[i-1].Op < opPseudo {
		i--
	}
	subs := p.stack[i:]
	p.stack = p.stack[:i]

	// Empty concatenation is special case.
	if len(subs) == 0 {
		return p.push(p.newRegexp(OpEmptyMatch))
	}

	return p.push(p.collapse(subs, OpConcat))
}

// alternate replaces the top of the stack (above the topmost '(') with its alternation.
func (p *parser) alternate() *Regexp {
	// Scan down to find pseudo-operator (.
	// There are no | above (.
	i := len(p.stack)
	for i > 0 && p.stack[i-1].Op < opPseudo {
		i--
	}
	subs := p.stack[i:]
	p.stack = p.stack[:i]

	// Make sure top class is clean.
	// All the others already are (see swapVerticalBar).
	if len(subs) > 0 {
		cleanAlt(subs[len(subs)-1])
	}

	// Empty alternate is special case
	// (shouldn't happen but easy to handle).
	if len(subs) == 0 {
		return p.push(p.newRegexp(OpNoMatch))
	}

	return p.push(p.collapse(subs, OpAlternate))
}

// cleanAlt cleans re for eventual inclusion in an alternation.
func cleanAlt(re *Regexp) {
	switch re.Op {
	case OpCharClass:
		re.Rune = cleanClass(&re.Rune)
		if len(re.Rune) == 2 && re.Rune[0] == 0 && re.Rune[1] == unicode.MaxRune {
			re.Rune = nil
			re.Op = OpAnyChar
			return
		}
		if len(re.Rune) == 4 && re.Rune[0] == 0 && re.Rune[1] == '\n'-1 && re.Rune[2] == '\n'+1 && re.Rune[3] == unicode.MaxRune {
			re.Rune = nil
			re.Op = OpAnyCharNotNL
			return
		}
		if cap(re.Rune)-len(re.Rune) > 100 {
			// re.Rune will not grow any more.
			// Make a copy or inline to reclaim storage.
			re.Rune = append(re.Rune0[:0], re.Rune...)
		}
	}
}

// collapse returns the result of applying op to sub.
// If sub contains op nodes, they all get hoisted up
// so that there is never a concat of a concat or an
// alternate of an alternate.
func (p *parser) collapse(subs []*Regexp, op Op) *Regexp {
	if len(subs) == 1 {
		return subs[0]
	}
	re := p.newRegexp(op)
	re.Sub = re.Sub0[:0]
	for _, sub := range subs {
		if sub.Op == op {
			re.Sub = append(re.Sub, sub.Sub...)
			p.reuse(sub)
		} else {
			re.Sub = append(re.Sub, sub)
		}
	}
	if op == OpAlternate {
		re.Sub = p.factor(re.Sub)
		if len(re.Sub) == 1 {
			old := re
			re = re.Sub[0]
			p.reuse(old)
		}
	}
	return re
}

// factor factors common prefixes from the alternation list sub.
// It returns a replacement list that reuses the same storage and
// frees (passes to p.reuse) any removed *Regexps.
//
// For example,
//
//	ABC|ABD|AEF|BCX|BCY
//
// simplifies by literal prefix extraction to
//
//	A(B(C|D)|EF)|BC(X|Y)
//
// which simplifies by character class introduction to
//
//	A(B[CD]|EF)|BC[XY]
func (p *parser) factor(sub []*Regexp) []*Regexp {
	if len(sub) < 2 {
		return sub
	}

	// Round 1: Factor out common literal prefixes.
	var str []rune
	var strflags Flags
	start := 0
	out := sub[:0]
	for i := 0; i <= len(sub); i++ {
		// Invariant: the Regexps that were in sub[0:start] have been
		// used or marked for reuse, and the slice space has been reused
		// for out (len(out) <= start).
		//
		// Invariant: sub[start:i] consists of regexps that all begin
		// with str as modified by strflags.
		var istr []rune
		var iflags Flags
		if i < len(sub) {
			istr, iflags = p.leadingString(sub[i])
			if iflags == strflags {
				same := 0
				for same < len(str) && same < len(istr) && str[same] == istr[same] {
					same++
				}
				if same > 0 {
					// Matches at least one rune in current range.
					// Keep going around.
					str = str[:same]
					continue
				}
			}
		}

		// Found end of a run with common leading literal string:
		// sub[start:i] all begin with str[:len(str)], but sub[i]
		// does not even begin with str[0].
		//
		// Factor out common string and append factored expression to out.
		if i == start {
			// Nothing to do - run of length 0.
		} else if i == start+1 {
			// Just one: don't bother factoring.
			out = append(out, sub[start])
		} else {
			// Construct factored form: prefix(suffix1|suffix2|...)
			prefix := p.newRegexp(OpLiteral)
			prefix.Flags = strflags
			prefix.Rune = append(prefix.Rune[:0], str...)

			for j := start; j < i; j++ {
				sub[j] = p.removeLeadingString(sub[j], len(str))
				p.checkLimits(sub[j])
			}
			suffix := p.collapse(sub[start:i], OpAlternate) // recurse

			re := p.newRegexp(OpConcat)
			re.Sub = append(re.Sub[:0], prefix, suffix)
			out = append(out, re)
		}

		// Prepare for next iteration.
		start = i
		str = istr
		strflags = iflags
	}
	sub = out

	// Round 2: Factor out common simple prefixes,
	// just the first piece of each concatenation.
	// This will be good enough a lot of the time.
	//
	// Complex subexpressions (e.g. involving quantifiers)
	// are not safe to factor because that collapses their
	// distinct paths through the automaton, which affects
	// correctness in some cases.
	start = 0
	out = sub[:0]
	var first *Regexp
	for i := 0; i <= len(sub); i++ {
		// Invariant: the Regexps that were in sub[0:start] have been
		// used or marked for reuse, and the slice space has been reused
		// for out (len(out) <= start).
		//
		// Invariant: sub[start:i] consists of regexps that all begin with ifirst.
		var ifirst *Regexp
		if i < len(sub) {
			ifirst = p.leadingRegexp(sub[i])
			if first != nil && first.Equal(ifirst) &&
				// first must be a character class OR a fixed repeat of a character class.
				(isCharClass(first) || (first.Op == OpRepeat && first.Min == first.Max && isCharClass(first.Sub[0]))) {
				continue
			}
		}

		// Found end of a run with common leading regexp:
		// sub[start:i] all begin with first but sub[i] does not.
		//
		// Factor out common regexp and append factored expression to out.
		if i == start {
			// Nothing to do - run of length 0.
		} else if i == start+1 {
			// Just one: don't bother factoring.
			out = append(out, sub[start])
		} else {
			// Construct factored form: prefix(suffix1|suffix2|...)
			prefix := first
			for j := start; j < i; j++ {
				reuse := j != start // prefix came from sub[start]
				sub[j] = p.removeLeadingRegexp(sub[j], reuse)
				p.checkLimits(sub[j])
			}
			suffix := p.collapse(sub[start:i], OpAlternate) // recurse

			re := p.newRegexp(OpConcat)
			re.Sub = append(re.Sub[:0], prefix, suffix)
			out = append(out, re)
		}

		// Prepare for next iteration.
		start = i
		first = ifirst
	}
	sub = out

	// Round 3: Collapse runs of single literals into character classes.
	start = 0
	out = sub[:0]
	for i := 0; i <= len(sub); i++ {
		// Invariant: the Regexps that were in sub[0:start] have been
		// used or marked for reuse, and the slice space has been reused
		// for out (len(out) <= start).
		//
		// Invariant: sub[start:i] consists of regexps that are either
		// literal runes or character classes.
		if i < len(sub) && isCharClass(sub[i]) {
			continue
		}

		// sub[i] is not a char or char class;
		// emit char class for sub[start:i]...
		if i == start {
			// Nothing to do - run of length 0.
		} else if i == start+1 {
			out = append(out, sub[start])
		} else {
			// Make new char class.
			// Start with most complex regexp in sub[start].
			max := start
			for j := start + 1; j < i; j++ {
				if sub[max].Op < sub[j].Op || sub[max].Op == sub[j].Op && len(sub[max].Rune) < len(sub[j].Rune) {
					max = j
				}
			}
			sub[start], sub[max] = sub[max], sub[start]

			for j := start + 1; j < i; j++ {
				mergeCharClass(sub[start], sub[j])
				p.reuse(sub[j])
			}
			cleanAlt(sub[start])
			out = append(out, sub[start])
		}

		// ... and then emit sub[i].
		if i < len(sub) {
			out = append(out, sub[i])
		}
		start = i + 1
	}
	sub = out

	// Round 4: Collapse runs of empty matches into a single empty match.
	start = 0
	out = sub[:0]
	for i := range sub {
		if i+1 < len(sub) && sub[i].Op == OpEmptyMatch && sub[i+1].Op == OpEmptyMatch {
			continue
		}
		out = append(out, sub[i])
	}
	sub = out

	return sub
}

// leadingString returns the leading literal string that re begins with.
// The string refers to storage in re or its children.
func (p *parser) leadingString(re *Regexp) ([]rune, Flags) {
	if re.Op == OpConcat && len(re.Sub) > 0 {
		re = re.Sub[0]
	}
	if re.Op != OpLiteral {
		return nil, 0
	}
	return re.Rune, re.Flags & FoldCase
}

// removeLeadingString removes the first n leading runes
// from the beginning of re. It returns the replacement for re.
func (p *parser) removeLeadingString(re *Regexp, n int) *Regexp {
	if re.Op == OpConcat && len(re.Sub) > 0 {
		// Removing a leading string in a concatenation
		// might simplify the concatenation.
		sub := re.Sub[0]
		sub = p.removeLeadingString(sub, n)
		re.Sub[0] = sub
		if sub.Op == OpEmptyMatch {
			p.reuse(sub)
			switch len(re.Sub) {
			case 0, 1:
				// Impossible but handle.
				re.Op = OpEmptyMatch
				re.Sub = nil
			case 2:
				old := re
				re = re.Sub[1]
				p.reuse(old)
			default:
				copy(re.Sub, re.Sub[1:])
				re.Sub = re.Sub[:len(re.Sub)-1]
			}
		}
		return re
	}

	if re.Op == OpLiteral {
		re.Rune = re.Rune[:copy(re.Rune, re.Rune[n:])]
		if len(re.Rune) == 0 {
			re.Op = OpEmptyMatch
		}
	}
	return re
}

// leadingRegexp returns the leading regexp that re begins with.
// The regexp refers to storage in re or its children.
func (p *parser) leadingRegexp(re *Regexp) *Regexp {
	if re.Op == OpEmptyMatch {
		return nil
	}
	if re.Op == OpConcat && len(re.Sub) > 0 {
		sub := re.Sub[0]
		if sub.Op == OpEmptyMatch {
			return nil
		}
		return sub
	}
	return re
}

// removeLeadingRegexp removes the leading regexp in re.
// It returns the replacement for re.
// If reuse is true, it passes the removed regexp (if no longer needed) to p.reuse.
func (p *parser) removeLeadingRegexp(re *Regexp, reuse bool) *Regexp {
	if re.Op == OpConcat && len(re.Sub) > 0 {
		if reuse {
			p.reuse(re.Sub[0])
		}
		re.Sub = re.Sub[:copy(re.Sub, re.Sub[1:])]
		switch len(re.Sub) {
		case 0:
			re.Op = OpEmptyMatch
			re.Sub = nil
		case 1:
			old := re
			re = re.Sub[0]
			p.reuse(old)
		}
		return re
	}
	if reuse {
		p.reuse(re)
	}
	return p.newRegexp(OpEmptyMatch)
}

func literalRegexp(s string, flags Flags) *Regexp {
	re := &Regexp{Op: OpLiteral}
	re.Flags = flags
	re.Rune = re.Rune0[:0] // use local storage for small strings
	for _, c := range s {
		if len(re.Rune) >= cap(re.Rune) {
			// string is too long to fit in Rune0.  let Go handle it
			re.Rune = []rune(s)
			break
		}
		re.Rune = append(re.Rune, c)
	}
	return re
}

// Parsing.

// Parse parses a regular expression string s, controlled by the specified
// Flags, and returns a regular expression parse tree. The syntax is
// described in the top-level comment.
func Parse(s string, flags Flags) (*Regexp, error) {
	return parse(s, flags)
}

func parse(s string, flags Flags) (_ *Regexp, err error) {
	defer func() {
		switch r := recover(); r {
		default:
			panic(r)
		case nil:
			// ok
		case ErrLarge: // too big
			err = &Error{Code: ErrLarge, Expr: s}
		case ErrNestingDepth:
			err = &Error{Code: ErrNestingDepth, Expr: s}
		}
	}()

	if flags&Literal != 0 {
		// Trivial parser for literal string.
		if err := checkUTF8(s); err != nil {
			return nil, err
		}
		return literalRegexp(s, flags), nil
	}

	// Otherwise, must do real work.
	var (
		p          parser
		c          rune
		op         Op
		lastRepeat string
	)
	p.flags = flags
	p.wholeRegexp = s
	t := s
	for t != "" {
		repeat := ""
	BigSwitch:
		switch t[0] {
		default:
			if c, t, err = nextRune(t); err != nil {
				return nil, err
			}
			p.literal(c)

		case '(':
			if p.flags&PerlX != 0 && len(t) >= 2 && t[1] == '?' {
				// Flag changes and non-capturing groups.
				if t, err = p.parsePerlFlags(t); err != nil {
					return nil, err
				}
				break
			}
			p.numCap++
			p.op(opLeftParen).Cap = p.numCap
			t = t[1:]
		case '|':
			p.parseVerticalBar()
			t = t[1:]
		case ')':
			if err = p.parseRightParen(); err != nil {
				return nil, err
			}
			t = t[1:]
		case '^':
			if p.flags&OneLine != 0 {
				p.op(OpBeginText)
			} else {
				p.op(OpBeginLine)
			}
			t = t[1:]
		case '$':
			if p.flags&OneLine != 0 {
				p.op(OpEndText).Flags |= WasDollar
			} else {
				p.op(OpEndLine)
			}
			t = t[1:]
		case '.':
			if p.flags&DotNL != 0 {
				p.op(OpAnyChar)
			} else {
				p.op(OpAnyCharNotNL)
			}
			t = t[1:]
		case '[':
			if t, err = p.parseClass(t); err != nil {
				return nil, err
			}
		case '*', '+', '?':
			before := t
			switch t[0] {
			case '*':
				op = OpStar
			case '+':
				op = OpPlus
			case '?':
				op = OpQuest
			}
			after := t[1:]
			if after, err = p.repeat(op, 0, 0, before, after, lastRepeat); err != nil {
				return nil, err
			}
			repeat = before
			t = after
		case '{':
			op = OpRepeat
			before := t
			min, max, after, ok := p.parseRepeat(t)
			if !ok {
				// If the repeat cannot be parsed, { is a literal.
				p.literal('{')
				t = t[1:]
				break
			}
			if min < 0 || min > 1000 || max > 1000 || max >= 0 && min > max {
				// Numbers were too big, or max is present and min > max.
				return nil, &Error{ErrInvalidRepeatSize, before[:len(before)-len(after)]}
			}
			if after, err = p.repeat(op, min, max, before, after, lastRepeat); err != nil {
				return nil, err
			}
			repeat = before
			t = after
		case '\\':
			if p.flags&PerlX != 0 && len(t) >= 2 {
				switch t[1] {
				case 'A':
					p.op(OpBeginText)
					t = t[2:]
					break BigSwitch
				case 'b':
					p.op(OpWordBoundary)
					t = t[2:]
					break BigSwitch
				case 'B':
					p.op(OpNoWordBoundary)
					t = t[2:]
					break BigSwitch
				case 'C':
					// any byte; not supported
					return nil, &Error{ErrInvalidEscape, t[:2]}
				case 'Q':
					// \Q ... \E: the ... is always literals
					var lit string
					lit, t, _ = strings.Cut(t[2:], `\E`)
					for lit != "" {
						c, rest, err := nextRune(lit)
						if err != nil {
							return nil, err
						}
						p.literal(c)
						lit = rest
					}
					break BigSwitch
				case 'z':
					p.op(OpEndText)
					t = t[2:]
					break BigSwitch
				}
			}

			re := p.newRegexp(OpCharClass)
			re.Flags = p.flags

			// Look for Unicode character group like \p{Han}
			if len(t) >= 2 && (t[1] == 'p' || t[1] == 'P') {
				r, rest, err := p.parseUnicodeClass(t, re.Rune0[:0])
				if err != nil {
					return nil, err
				}
				if r != nil {
					re.Rune = r
					t = rest
					p.push(re)
					break BigSwitch
				}
			}

			// Perl character class escape.
			if r, rest := p.parsePerlClassEscape(t, re.Rune0[:0]); r != nil {
				re.Rune = r
				t = rest
				p.push(re)
				break BigSwitch
			}
			p.reuse(re)

			// Ordinary single-character escape.
			if c, t, err = p.parseEscape(t); err != nil {
				return nil, err
			}
			p.literal(c)
		}
		lastRepeat = repeat
	}

	p.concat()
	if p.swapVerticalBar() {
		// pop vertical bar
		p.stack = p.stack[:len(p.stack)-1]
	}
	p.alternate()

	n := len(p.stack)
	if n != 1 {
		return nil, &Error{ErrMissingParen, s}
	}
	return p.stack[0], nil
}

// parseRepeat parses {min} (max=min) or {min,} (max=-1) or {min,max}.
// If s is not of that form, it returns ok == false.
// If s has the right form but the values are too big, it returns min == -1, ok == true.
func (p *parser) parseRepeat(s string) (min, max int, rest string, ok bool) {
	if s == "" || s[0] != '{' {
		return
	}
	s = s[1:]
	var ok1 bool
	if min, s, ok1 = p.parseInt(s); !ok1 {
		return
	}
	if s == "" {
		return
	}
	if s[0] != ',' {
		max = min
	} else {
		s = s[1:]
		if s == "" {
			return
		}
		if s[0] == '}' {
			max = -1
		} else if max, s, ok1 = p.parseInt(s); !ok1 {
			return
		} else if max < 0 {
			// parseInt found too big a number
			min = -1
		}
	}
	if s == "" || s[0] != '}' {
		return
	}
	rest = s[1:]
	ok = true
	return
}

// parsePerlFlags parses a Perl flag setting or non-capturing group or both,
// like (?i) or (?: or (?i:.  It removes the prefix from s and updates the parse state.
// The caller must have ensured that s begins with "(?".
func (p *parser) parsePerlFlags(s string) (rest string, err error) {
	t := s

	// Check for named captures, first introduced in Python's regexp library.
	// As usual, there are three slightly different syntaxes:
	//
	//   (?P<name>expr)   the original, introduced by Python
	//   (?<name>expr)    the .NET alteration, adopted by Perl 5.10
	//   (?'name'expr)    another .NET alteration, adopted by Perl 5.10
	//
	// Perl 5.10 gave in and implemented the Python version too,
	// but they claim that the last two are the preferred forms.
	// PCRE and languages based on it (specifically, PHP and Ruby)
	// support all three as well. EcmaScript 4 uses only the Python form.
	//
	// In both the open source world (via Code Search) and the
	// Google source tree, (?P<expr>name) and (?<expr>name) are the
	// dominant forms of named captures and both are supported.
	startsWithP := len(t) > 4 && t[2] == 'P' && t[3] == '<'
	startsWithName := len(t) > 3 && t[2] == '<'

	if startsWithP || startsWithName {
		// position of expr start
		exprStartPos := 4
		if startsWithName {
			exprStartPos = 3
		}

		// Pull out name.
		end := strings.IndexRune(t, '>')
		if end < 0 {
			if err = checkUTF8(t); err != nil {
				return "", err
			}
			return "", &Error{ErrInvalidNamedCapture, s}
		}

		capture := t[:end+1]        // "(?P<name>" or "(?<name>"
		name := t[exprStartPos:end] // "name"
		if err = checkUTF8(name); err != nil {
			return "", err
		}
		if !isValidCaptureName(name) {
			return "", &Error{ErrInvalidNamedCapture, capture}
		}

		// Like ordinary capture, but named.
		p.numCap++
		re := p.op(opLeftParen)
		re.Cap = p.numCap
		re.Name = name
		return t[end+1:], nil
	}

	// Non-capturing group. Might also twiddle Perl flags.
	var c rune
	t = t[2:] // skip (?
	flags := p.flags
	sign := +1
	sawFlag := false
Loop:
	for t != "" {
		if c, t, err = nextRune(t); err != nil {
			return "", err
		}
		switch c {
		default:
			break Loop

		// Flags.
		case 'i':
			flags |= FoldCase
			sawFlag = true
		case 'm':
			flags &^= OneLine
			sawFlag = true
		case 's':
			flags |= DotNL
			sawFlag = true
		case 'U':
			flags |= NonGreedy
			sawFlag = true

		// Switch to negation.
		case '-':
			if sign < 0 {
				break Loop
			}
			sign = -1
			// Invert flags so that | above turn into &^ and vice versa.
			// We'll invert flags again before using it below.
			flags = ^flags
			sawFlag = false

		// End of flags, starting group or not.
		case ':', ')':
			if sign < 0 {
				if !sawFlag {
					break Loop
				}
				flags = ^flags
			}
			if c == ':' {
				// Open new group
				p.op(opLeftParen)
			}
			p.flags = flags
			return t, nil
		}
	}

	return "", &Error{ErrInvalidPerlOp, s[:len(s)-len(t)]}
}

// isValidCaptureName reports whether name
// is a valid capture name: [A-Za-z0-9_]+.
// PCRE limits names to 32 bytes.
// Python rejects names starting with digits.
// We don't enforce either of those.
func isValidCaptureName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if c != '_' && !isalnum(c) {
			return false
		}
	}
	return true
}

// parseInt parses a decimal integer.
func (p *parser) parseInt(s string) (n int, rest string, ok bool) {
	if s == "" || s[0] < '0' || '9' < s[0] {
		return
	}
	// Disallow leading zeros.
	if len(s) >= 2 && s[0] == '0' && '0' <= s[1] && s[1] <= '9' {
		return
	}
	t := s
	for s != "" && '0' <= s[0] && s[0] <= '9' {
		s = s[1:]
	}
	rest = s
	ok = true
	// Have digits, compute value.
	t = t[:len(t)-len(s)]
	for i := 0; i < len(t); i++ {
		// Avoid overflow.
		if n >= 1e8 {
			n = -1
			break
		}
		n = n*10 + int(t[i]) - '0'
	}
	return
}

// can this be represented as a character class?
// single-rune literal string, char class, ., and .|\n.
func isCharClass(re *Regexp) bool {
	return re.Op == OpLiteral && len(re.Rune) == 1 ||
		re.Op == OpCharClass ||
		re.Op == OpAnyCharNotNL ||
		re.Op == OpAnyChar
}

// does re match r?
func matchRune(re *Regexp, r rune) bool {
	switch re.Op {
	case OpLiteral:
		return len(re.Rune) == 1 && re.Rune[0] == r
	case OpCharClass:
		for i := 0; i < len(re.Rune); i += 2 {
			if re.Rune[i] <= r && r <= re.Rune[i+1] {
				return true
			}
		}
		return false
	case OpAnyCharNotNL:
		return r != '\n'
	case OpAnyChar:
		return true
	}
	return false
}

// parseVerticalBar handles a | in the input.
func (p *parser) parseVerticalBar() {
	p.concat()

	// The concatenation we just parsed is on top of the stack.
	// If it sits above an opVerticalBar, swap it below
	// (things below an opVerticalBar become an alternation).
	// Otherwise, push a new vertical bar.
	if !p.swapVerticalBar() {
		p.op(opVerticalBar)
	}
}

// mergeCharClass makes dst = dst|src.
// The caller must ensure that dst.Op >= src.Op,
// to reduce the amount of copying.
func mergeCharClass(dst, src *Regexp) {
	switch dst.Op {
	case OpAnyChar:
		// src doesn't add anything.
	case OpAnyCharNotNL:
		// src might add \n
		if matchRune(src, '\n') {
			dst.Op = OpAnyChar
		}
	case OpCharClass:
		// src is simpler, so either literal or char class
		if src.Op == OpLiteral {
			dst.Rune = appendLiteral(dst.Rune, src.Rune[0], src.Flags)
		} else {
			dst.Rune = appendClass(dst.Rune, src.Rune)
		}
	case OpLiteral:
		// both literal
		if src.Rune[0] == dst.Rune[0] && src.Flags == dst.Flags {
			break
		}
		dst.Op = OpCharClass
		dst.Rune = appendLiteral(dst.Rune[:0], dst.Rune[0], dst.Flags)
		dst.Rune = appendLiteral(dst.Rune, src.Rune[0], src.Flags)
	}
}

// If the top of the stack is an element followed by an opVerticalBar
// swapVerticalBar swaps the two and returns true.
// Otherwise it returns false.
func (p *parser) swapVerticalBar() bool {
	// If above and below vertical bar are literal or char class,
	// can merge into a single char class.
	n := len(p.stack)
	if n >= 3 && p.stack[n-2].Op == opVerticalBar && isCharClass(p.stack[n-1]) && isCharClass(p.stack[n-3]) {
		re1 := p.stack[n-1]
		re3 := p.stack[n-3]
		// Make re3 the more complex of the two.
		if re1.Op > re3.Op {
			re1, re3 = re3, re1
			p.stack[n-3] = re3
		}
		mergeCharClass(re3, re1)
		p.reuse(re1)
		p.stack = p.stack[:n-1]
		return true
	}

	if n >= 2 {
		re1 := p.stack[n-1]
		re2 := p.stack[n-2]
		if re2.Op == opVerticalBar {
			if n >= 3 {
				// Now out of reach.
				// Clean opportunistically.
				cleanAlt(p.stack[n-3])
			}
			p.stack[n-2] = re1
			p.stack[n-1] = re2
			return true
		}
	}
	return false
}

// parseRightParen handles a ) in the input.
func (p *parser) parseRightParen() error {
	p.concat()
	if p.swapVerticalBar() {
		// pop vertical bar
		p.stack = p.stack[:len(p.stack)-1]
	}
	p.alternate()

	n := len(p.stack)
	if n < 2 {
		return &Error{ErrUnexpectedParen, p.wholeRegexp}
	}
	re1 := p.stack[n-1]
	re2 := p.stack[n-2]
	p.stack = p.stack[:n-2]
	if re2.Op != opLeftParen {
		return &Error{ErrUnexpectedParen, p.wholeRegexp}
	}
	// Restore flags at time of paren.
	p.flags = re2.Flags
	if re2.Cap == 0 {
		// Just for grouping.
		p.push(re1)
	} else {
		re2.Op = OpCapture
		re2.Sub = re2.Sub0[:1]
		re2.Sub[0] = re1
		p.push(re2)
	}
	return nil
}

// parseEscape parses an escape sequence at the beginning of s
// and returns the rune.
func (p *parser) parseEscape(s string) (r rune, rest string, err error) {
	t := s[1:]
	if t == "" {
		return 0, "", &Error{ErrTrailingBackslash, ""}
	}
	c, t, err := nextRune(t)
	if err != nil {
		return 0, "", err
	}

Switch:
	switch c {
	default:
		if c < utf8.RuneSelf && !isalnum(c) {
			// Escaped non-word characters are always themselves.
			// PCRE is not quite so rigorous: it accepts things like
			// \q, but we don't. We once rejected \_, but too many
			// programs and people insist on using it, so allow \_.
			return c, t, nil
		}

	// Octal escapes.
	case '1', '2', '3', '4', '5', '6', '7':
		// Single non-zero digit is a backreference; not supported
		if t == "" || t[0] < '0' || t[0] > '7' {
			break
		}
		fallthrough
	case '0':
		// Consume up to three octal digits; already have one.
		r = c - '0'
		for i := 1; i < 3; i++ {
			if t == "" || t[0] < '0' || t[0] > '7' {
				break
			}
			r = r*8 + rune(t[0]) - '0'
			t = t[1:]
		}
		return r, t, nil

	// Hexadecimal escapes.
	case 'x':
		if t == "" {
			break
		}
		if c, t, err = nextRune(t); err != nil {
			return 0, "", err
		}
		if c == '{' {
			// Any number of digits in braces.
			// Perl accepts any text at all; it ignores all text
			// after the first non-hex digit. We require only hex digits,
			// and at least one.
			nhex := 0
			r = 0
			for {
				if t == "" {
					break Switch
				}
				if c, t, err = nextRune(t); err != nil {
					return 0, "", err
				}
				if c == '}' {
					break
				}
				v := unhex(c)
				if v < 0 {
					break Switch
				}
				r = r*16 + v
				if r > unicode.MaxRune {
					break Switch
				}
				nhex++
			}
			if nhex == 0 {
				break Switch
			}
			return r, t, nil
		}

		// Easy case: two hex digits.
		x := unhex(c)
		if c, t, err = nextRune(t); err != nil {
			return 0, "", err
		}
		y := unhex(c)
		if x < 0 || y < 0 {
			break
		}
		return x*16 + y, t, nil

	// C escapes. There is no case 'b', to avoid misparsing
	// the Perl word-boundary \b as the C backspace \b
	// when in POSIX mode. In Perl, /\b/ means word-boundary
	// but /[\b]/ means backspace. We don't support that.
	// If you want a backspace, embed a literal backspace
	// character or use \x08.
	case 'a':
		return '\a', t, err
	case 'f':
		return '\f', t, err
	case 'n':
		return '\n', t, err
	case 'r':
		return '\r', t, err
	case 't':
		return '\t', t, err
	case 'v':
		return '\v', t, err
	}
	return 0, "", &Error{ErrInvalidEscape, s[:len(s)-len(t)]}
}

// parseClassChar parses a character class character at the beginning of s
// and returns it.
func (p *parser) parseClassChar(s, wholeClass string) (r rune, rest string, err error) {
	if s == "" {
		return 0, "", &Error{Code: ErrMissingBracket, Expr: wholeClass}
	}

	// Allow regular escape sequences even though
	// many need not be escaped in this context.
	if s[0] == '\\' {
		return p.parseEscape(s)
	}

	return nextRune(s)
}

type charGroup struct {
	sign  int
	class []rune
}

//go:generate perl make_perl_groups.pl perl_groups.go

// parsePerlClassEscape parses a leading Perl character class escape like \d
// from the beginning of s. If one is present, it appends the characters to r
// and returns the new slice r and the remainder of the string.
func (p *parser) parsePerlClassEscape(s string, r []rune) (out []rune, rest string) {
	if p.flags&PerlX == 0 || len(s) < 2 || s[0] != '\\' {
		return
	}
	g := perlGroup[s[0:2]]
	if g.sign == 0 {
		return
	}
	return p.appendGroup(r, g), s[2:]
}

// parseNamedClass parses a leading POSIX named character class like [:alnum:]
// from the beginning of s. If one is present, it appends the characters to r
// and returns the new slice r and the remainder of the string.
func (p *parser) parseNamedClass(s string, r []rune) (out []rune, rest string, err error) {
	if len(s) < 2 || s[0] != '[' || s[1] != ':' {
		return
	}

	i := strings.Index(s[2:], ":]")
	if i < 0 {
		return
	}
	i += 2
	name, s := s[0:i+2], s[i+2:]
	g := posixGroup[name]
	if g.sign == 0 {
		return nil, "", &Error{ErrInvalidCharRange, name}
	}
	return p.appendGroup(r, g), s, nil
}

func (p *parser) appendGroup(r []rune, g charGroup) []rune {
	if p.flags&FoldCase == 0 {
		if g.sign < 0 {
			r = appendNegatedClass(r, g.class)
		} else {
			r = appendClass(r, g.class)
		}
	} else {
		tmp := p.tmpClass[:0]
		tmp = appendFoldedClass(tmp, g.class)
		p.tmpClass = tmp
		tmp = cleanClass(&p.tmpClass)
		if g.sign < 0 {
			r = appendNegatedClass(r, tmp)
		} else {
			r = appendClass(r, tmp)
		}
	}
	return r
}

var anyTable = &unicode.RangeTable{
	R16: []unicode.Range16{{Lo: 0, Hi: 1<<16 - 1, Stride: 1}},
	R32: []unicode.Range32{{Lo: 1 << 16, Hi: unicode.MaxRune, Stride: 1}},
}

var asciiTable = &unicode.RangeTable{
	R16: []unicode.Range16{{Lo: 0, Hi: 0x7F, Stride: 1}},
}

var asciiFoldTable = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0, Hi: 0x7F, Stride: 1},
		{Lo: 0x017F, Hi: 0x017F, Stride: 1}, // Old English long s (ſ), folds to S/s.
		{Lo: 0x212A, Hi: 0x212A, Stride: 1}, // Kelvin K, folds to K/k.
	},
}

// aliases is a lazily constructed copy of unicode.CategoryAliases and unicode.Scripts
// but with the keys passed through canonicalName, to support inexact matches.
var aliases struct {
	once       sync.Once
	categories map[string]string
	scripts    map[string]string
}

// initAliases initializes categoryAliases by canonicalizing unicode.CategoryAliases.
func initAliases() {
	aliases.categories = make(map[string]string)
	aliases.scripts = make(map[string]string)
	for name, actual := range unicode.CategoryAliases {
		aliases.categories[canonicalName(name)] = actual
	}
	for name := range unicode.Scripts {
		aliases.scripts[canonicalName(name)] = name
	}
}

// canonicalName returns the canonical lookup string for name.
// The canonical name has a leading uppercase letter and then lowercase letters,
// and it omits all underscores, spaces, and hyphens.
// (We could have used all lowercase, but this way most package unicode
// map keys are already canonical.)
func canonicalName(name string) string {
	var b []byte
	first := true
	for i := range len(name) {
		c := name[i]
		switch {
		case c == '_' || c == '-' || c == ' ':
			c = ' '
		case first:
			if 'a' <= c && c <= 'z' {
				c -= 'a' - 'A'
			}
			first = false
		default:
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
		}
		if b == nil {
			if c == name[i] && c != ' ' {
				// No changes so far, avoid allocating b.
				continue
			}
			b = make([]byte, i, len(name))
			copy(b, name[:i])
		}
		if c == ' ' {
			continue
		}
		b = append(b, c)
	}
	if b == nil {
		return name
	}
	return string(b)
}

// unicodeTable returns the unicode.RangeTable identified by name
// and the table of additional fold-equivalent code points.
// If sign < 0, the result should be inverted.
func unicodeTable(name string) (tab, fold *unicode.RangeTable, sign int) {
	name = canonicalName(name)

	// Special cases: Any, Assigned, and ASCII.
	// Also LC is the only non-canonical Categories key, so handle it here.
	switch name {
	case "Any":
		return anyTable, anyTable, +1
	case "Assigned":
		return unicode.Cn, unicode.Cn, -1 // invert Cn (unassigned)
	case "Ascii":
		return asciiTable, asciiFoldTable, +1
	case "Lc":
		return unicode.Categories["LC"], unicode.FoldCategory["LC"], +1
	}
	if t := unicode.Categories[name]; t != nil {
		return t, unicode.FoldCategory[name], +1
	}
	if t := unicode.Scripts[name]; t != nil {
		return t, unicode.FoldScript[name], +1
	}

	// unicode.CategoryAliases makes liberal use of underscores in its names
	// (they are defined that way by Unicode), but we want to match ignoring
	// the underscores, so make our own map with canonical names.
	aliases.once.Do(initAliases)
	if actual := aliases.categories[name]; actual != "" {
		t := unicode.Categories[actual]
		return t, unicode.FoldCategory[actual], +1
	}
	if actual := aliases.scripts[name]; actual != "" {
		t := unicode.Scripts[actual]
		return t, unicode.FoldScript[actual], +1
	}
	return nil, nil, 0
}

// parseUnicodeClass parses a leading Unicode character class like \p{Han}
// from the beginning of s. If one is present, it appends the characters to r
// and returns the new slice r and the remainder of the string.
func (p *parser) parseUnicodeClass(s string, r []rune) (out []rune, rest string, err error) {
	if p.flags&UnicodeGroups == 0 || len(s) < 2 || s[0] != '\\' || s[1] != 'p' && s[1] != 'P' {
		return
	}

	// Committed to parse or return error.
	sign := +1
	if s[1] == 'P' {
		sign = -1
	}
	t := s[2:]
	c, t, err := nextRune(t)
	if err != nil {
		return
	}
	var seq, name string
	if c != '{' {
		// Single-letter name.
		seq = s[:len(s)-len(t)]
		name = seq[2:]
	} else {
		// Name is in braces.
		end := strings.IndexRune(s, '}')
		if end < 0 {
			if err = checkUTF8(s); err != nil {
				return
			}
			return nil, "", &Error{ErrInvalidCharRange, s}
		}
		seq, t = s[:end+1], s[end+1:]
		name = s[3:end]
		if err = checkUTF8(name); err != nil {
			return
		}
	}

	// Group can have leading negation too.  \p{^Han} == \P{Han}, \P{^Han} == \p{Han}.
	if name != "" && name[0] == '^' {
		sign = -sign
		name = name[1:]
	}

	tab, fold, tsign := unicodeTable(name)
	if tab == nil {
		return nil, "", &Error{ErrInvalidCharRange, seq}
	}
	if tsign < 0 {
		sign = -sign
	}

	if p.flags&FoldCase == 0 || fold == nil {
		if sign > 0 {
			r = appendTable(r, tab)
		} else {
			r = appendNegatedTable(r, tab)
		}
	} else {
		// Merge and clean tab and fold in a temporary buffer.
		// This is necessary for the negative case and just tidy
		// for the positive case.
		tmp := p.tmpClass[:0]
		tmp = appendTable(tmp, tab)
		tmp = appendTable(tmp, fold)
		p.tmpClass = tmp
		tmp = cleanClass(&p.tmpClass)
		if sign > 0 {
			r = appendClass(r, tmp)
		} else {
			r = appendNegatedClass(r, tmp)
		}
	}
	return r, t, nil
}

// parseClass parses a character class at the beginning of s
// and pushes it onto the parse stack.
func (p *parser) parseClass(s string) (rest string, err error) {
	t := s[1:] // chop [
	re := p.newRegexp(OpCharClass)
	re.Flags = p.flags
	re.Rune = re.Rune0[:0]

	sign := +1
	if t != "" && t[0] == '^' {
		sign = -1
		t = t[1:]

		// If character class does not match \n, add it here,
		// so that negation later will do the right thing.
		if p.flags&ClassNL == 0 {
			re.Rune = append(re.Rune, '\n', '\n')
		}
	}

	class := re.Rune
	first := true // ] and - are okay as first char in class
	for t == "" || t[0] != ']' || first {
		// POSIX: - is only okay unescaped as first or last in class.
		// Perl: - is okay anywhere.
		if t != "" && t[0] == '-' && p.flags&PerlX == 0 && !first && (len(t) == 1 || t[1] != ']') {
			_, size := utf8.DecodeRuneInString(t[1:])
			return "", &Error{Code: ErrInvalidCharRange, Expr: t[:1+size]}
		}
		first = false

		// Look for POSIX [:alnum:] etc.
		if len(t) > 2 && t[0] == '[' && t[1] == ':' {
			nclass, nt, err := p.parseNamedClass(t, class)
			if err != nil {
				return "", err
			}
			if nclass != nil {
				class, t = nclass, nt
				continue
			}
		}

		// Look for Unicode character group like \p{Han}.
		nclass, nt, err := p.parseUnicodeClass(t, class)
		if err != nil {
			return "", err
		}
		if nclass != nil {
			class, t = nclass, nt
			continue
		}

		// Look for Perl character class symbols (extension).
		if nclass, nt := p.parsePerlClassEscape(t, class); nclass != nil {
			class, t = nclass, nt
			continue
		}

		// Single character or simple range.
		rng := t
		var lo, hi rune
		if lo, t, err = p.parseClassChar(t, s); err != nil {
			return "", err
		}
		hi = lo
		// [a-] means (a|-) so check for final ].
		if len(t) >= 2 && t[0] == '-' && t[1] != ']' {
			t = t[1:]
			if hi, t, err = p.parseClassChar(t, s); err != nil {
				return "", err
			}
			if hi < lo {
				rng = rng[:len(rng)-len(t)]
				return "", &Error{Code: ErrInvalidCharRange, Expr: rng}
			}
		}
		if p.flags&FoldCase == 0 {
			class = appendRange(class, lo, hi)
		} else {
			class = appendFoldedRange(class, lo, hi)
		}
	}
	t = t[1:] // chop ]

	// Use &re.Rune instead of &class to avoid allocation.
	re.Rune = class
	class = cleanClass(&re.Rune)
	if sign < 0 {
		class = negateClass(class)
	}
	re.Rune = class
	p.push(re)
	return t, nil
}

// cleanClass sorts the ranges (pairs of elements of r),
// merges them, and eliminates duplicates.
func cleanClass(rp *[]rune) []rune {

	// Sort by lo increasing, hi decreasing to break ties.
	sort.Sort(ranges{rp})

	r := *rp
	if len(r) < 2 {
		return r
	}

	// Merge abutting, overlapping.
	w := 2 // write index
	for i := 2; i < len(r); i += 2 {
		lo, hi := r[i], r[i+1]
		if lo <= r[w-1]+1 {
			// merge with previous range
			if hi > r[w-1] {
				r[w-1] = hi
			}
			continue
		}
		// new disjoint range
		r[w] = lo
		r[w+1] = hi
		w += 2
	}

	return r[:w]
}

// inCharClass reports whether r is in the class.
// It assumes the class has been cleaned by cleanClass.
func inCharClass(r rune, class []rune) bool {
	_, ok := sort.Find(len(class)/2, func(i int) int {
		lo, hi := class[2*i], class[2*i+1]
		if r > hi {
			return +1
		}
		if r < lo {
			return -1
		}
		return 0
	})
	return ok
}

// appendLiteral returns the result of appending the literal x to the class r.
func appendLiteral(r []rune, x rune, flags Flags) []rune {
	if flags&FoldCase != 0 {
		return appendFoldedRange(r, x, x)
	}
	return appendRange(r, x, x)
}

// appendRange returns the result of appending the range lo-hi to the class r.
func appendRange(r []rune, lo, hi rune) []rune {
	// Expand last range or next to last range if it overlaps or abuts.
	// Checking two ranges helps when appending case-folded
	// alphabets, so that one range can be expanding A-Z and the
	// other expanding a-z.
	n := len(r)
	for i := 2; i <= 4; i += 2 { // twice, using i=2, i=4
		if n >= i {
			rlo, rhi := r[n-i], r[n-i+1]
			if lo <= rhi+1 && rlo <= hi+1 {
				if lo < rlo {
					r[n-i] = lo
				}
				if hi > rhi {
					r[n-i+1] = hi
				}
				return r
			}
		}
	}

	return append(r, lo, hi)
}

const (
	// minimum and maximum runes involved in folding.
	// checked during test.
	minFold = 0x0041
	maxFold = 0x1e943
)

// appendFoldedRange returns the result of appending the range lo-hi
// and its case folding-equivalent runes to the class r.
func appendFoldedRange(r []rune, lo, hi rune) []rune {
	// Optimizations.
	if lo <= minFold && hi >= maxFold {
		// Range is full: folding can't add more.
		return appendRange(r, lo, hi)
	}
	if hi < minFold || lo > maxFold {
		// Range is outside folding possibilities.
		return appendRange(r, lo, hi)
	}
	if lo < minFold {
		// [lo, minFold-1] needs no folding.
		r = appendRange(r, lo, minFold-1)
		lo = minFold
	}
	if hi > maxFold {
		// [maxFold+1, hi] needs no folding.
		r = appendRange(r, maxFold+1, hi)
		hi = maxFold
	}

	// Brute force. Depend on appendRange to coalesce ranges on the fly.
	for c := lo; c <= hi; c++ {
		r = appendRange(r, c, c)
		f := unicode.SimpleFold(c)
		for f != c {
			r = appendRange(r, f, f)
			f = unicode.SimpleFold(f)
		}
	}
	return r
}

// appendClass returns the result of appending the class x to the class r.
// It assume x is clean.
func appendClass(r []rune, x []rune) []rune {
	for i := 0; i < len(x); i += 2 {
		r = appendRange(r, x[i], x[i+1])
	}
	return r
}

// appendFoldedClass returns the result of appending the case folding of the class x to the class r.
func appendFoldedClass(r []rune, x []rune) []rune {
	for i := 0; i < len(x); i += 2 {
		r = appendFoldedRange(r, x[i], x[i+1])
	}
	return r
}

// appendNegatedClass returns the result of appending the negation of the class x to the class r.
// It assumes x is clean.
func appendNegatedClass(r []rune, x []rune) []rune {
	nextLo := '\u0000'
	for i := 0; i < len(x); i += 2 {
		lo, hi := x[i], x[i+1]
		if nextLo <= lo-1 {
			r = appendRange(r, nextLo, lo-1)
		}
		nextLo = hi + 1
	}
	if nextLo <= unicode.MaxRune {
		r = appendRange(r, nextLo, unicode.MaxRune)
	}
	return r
}

// appendTable returns the result of appending x to the class r.
func appendTable(r []rune, x *unicode.RangeTable) []rune {
	for _, xr := range x.R16 {
		lo, hi, stride := rune(xr.Lo), rune(xr.Hi), rune(xr.Stride)
		if stride == 1 {
			r = appendRange(r, lo, hi)
			continue
		}
		for c := lo; c <= hi; c += stride {
			r = appendRange(r, c, c)
		}
	}
	for _, xr := range x.R32 {
		lo, hi, stride := rune(xr.Lo), rune(xr.Hi), rune(xr.Stride)
		if stride == 1 {
			r = appendRange(r, lo, hi)
			continue
		}
		for c := lo; c <= hi; c += stride {
			r = appendRange(r, c, c)
		}
	}
	return r
}

// appendNegatedTable returns the result of appending the negation of x to the class r.
func appendNegatedTable(r []rune, x *unicode.RangeTable) []rune {
	nextLo := '\u0000' // lo end of next class to add
	for _, xr := range x.R16 {
		lo, hi, stride := rune(xr.Lo), rune(xr.Hi), rune(xr.Stride)
		if stride == 1 {
			if nextLo <= lo-1 {
				r = appendRange(r, nextLo, lo-1)
			}
			nextLo = hi + 1
			continue
		}
		for c := lo; c <= hi; c += stride {
			if nextLo <= c-1 {
				r = appendRange(r, nextLo, c-1)
			}
			nextLo = c + 1
		}
	}
	for _, xr := range x.R32 {
		lo, hi, stride := rune(xr.Lo), rune(xr.Hi), rune(xr.Stride)
		if stride == 1 {
			if nextLo <= lo-1 {
				r = appendRange(r, nextLo, lo-1)
			}
			nextLo = hi + 1
			continue
		}
		for c := lo; c <= hi; c += stride {
			if nextLo <= c-1 {
				r = appendRange(r, nextLo, c-1)
			}
			nextLo = c + 1
		}
	}
	if nextLo <= unicode.MaxRune {
		r = appendRange(r, nextLo, unicode.MaxRune)
	}
	return r
}

// negateClass overwrites r and returns r's negation.
// It assumes the class r is already clean.
func negateClass(r []rune) []rune {
	nextLo := '\u0000' // lo end of next class to add
	w := 0             // write index
	for i := 0; i < len(r); i += 2 {
		lo, hi := r[i], r[i+1]
		if nextLo <= lo-1 {
			r[w] = nextLo
			r[w+1] = lo - 1
			w += 2
		}
		nextLo = hi + 1
	}
	r = r[:w]
	if nextLo <= unicode.MaxRune {
		// It's possible for the negation to have one more
		// range - this one - than the original class, so use append.
		r = append(r, nextLo, unicode.MaxRune)
	}
	return r
}

// ranges implements sort.Interface on a []rune.
// The choice of receiver type definition is strange
// but avoids an allocation since we already have
// a *[]rune.
type ranges struct {
	p *[]rune
}

func (ra ranges) Less(i, j int) bool {
	p := *ra.p
	i *= 2
	j *= 2
	return p[i] < p[j] || p[i] == p[j] && p[i+1] > p[j+1]
}

func (ra ranges) Len() int {
	return len(*ra.p) / 2
}

func (ra ranges) Swap(i, j int) {
	p := *ra.p
	i *= 2
	j *= 2
	p[i], p[i+1], p[j], p[j+1] = p[j], p[j+1], p[i], p[i+1]
}

func checkUTF8(s string) error {
	for s != "" {
		rune, size := utf8.DecodeRuneInString(s)
		if rune == utf8.RuneError && size == 1 {
			return &Error{Code: ErrInvalidUTF8, Expr: s}
		}
		s = s[size:]
	}
	return nil
}

func nextRune(s string) (c rune, t string, err error) {
	c, size := utf8.DecodeRuneInString(s)
	if c == utf8.RuneError && size == 1 {
		return 0, "", &Error{Code: ErrInvalidUTF8, Expr: s}
	}
	return c, s[size:], nil
}

func isalnum(c rune) bool {
	return '0' <= c && c <= '9' || 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z'
}

func unhex(c rune) rune {
	if '0' <= c && c <= '9' {
		return c - '0'
	}
	if 'a' <= c && c <= 'f' {
		return c - 'a' + 10
	}
	if 'A' <= c && c <= 'F' {
		return c - 'A' + 10
	}
	return -1
}

```

// === FILE: references!/go/src/regexp/syntax/perl_groups.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by make_perl_groups.pl; DO NOT EDIT.

package syntax

var code1 = []rune{ /* \d */
	0x30, 0x39,
}

var code2 = []rune{ /* \s */
	0x9, 0xa,
	0xc, 0xd,
	0x20, 0x20,
}

var code3 = []rune{ /* \w */
	0x30, 0x39,
	0x41, 0x5a,
	0x5f, 0x5f,
	0x61, 0x7a,
}

var perlGroup = map[string]charGroup{
	`\d`: {+1, code1},
	`\D`: {-1, code1},
	`\s`: {+1, code2},
	`\S`: {-1, code2},
	`\w`: {+1, code3},
	`\W`: {-1, code3},
}
var code4 = []rune{ /* [:alnum:] */
	0x30, 0x39,
	0x41, 0x5a,
	0x61, 0x7a,
}

var code5 = []rune{ /* [:alpha:] */
	0x41, 0x5a,
	0x61, 0x7a,
}

var code6 = []rune{ /* [:ascii:] */
	0x0, 0x7f,
}

var code7 = []rune{ /* [:blank:] */
	0x9, 0x9,
	0x20, 0x20,
}

var code8 = []rune{ /* [:cntrl:] */
	0x0, 0x1f,
	0x7f, 0x7f,
}

var code9 = []rune{ /* [:digit:] */
	0x30, 0x39,
}

var code10 = []rune{ /* [:graph:] */
	0x21, 0x7e,
}

var code11 = []rune{ /* [:lower:] */
	0x61, 0x7a,
}

var code12 = []rune{ /* [:print:] */
	0x20, 0x7e,
}

var code13 = []rune{ /* [:punct:] */
	0x21, 0x2f,
	0x3a, 0x40,
	0x5b, 0x60,
	0x7b, 0x7e,
}

var code14 = []rune{ /* [:space:] */
	0x9, 0xd,
	0x20, 0x20,
}

var code15 = []rune{ /* [:upper:] */
	0x41, 0x5a,
}

var code16 = []rune{ /* [:word:] */
	0x30, 0x39,
	0x41, 0x5a,
	0x5f, 0x5f,
	0x61, 0x7a,
}

var code17 = []rune{ /* [:xdigit:] */
	0x30, 0x39,
	0x41, 0x46,
	0x61, 0x66,
}

var posixGroup = map[string]charGroup{
	`[:alnum:]`:   {+1, code4},
	`[:^alnum:]`:  {-1, code4},
	`[:alpha:]`:   {+1, code5},
	`[:^alpha:]`:  {-1, code5},
	`[:ascii:]`:   {+1, code6},
	`[:^ascii:]`:  {-1, code6},
	`[:blank:]`:   {+1, code7},
	`[:^blank:]`:  {-1, code7},
	`[:cntrl:]`:   {+1, code8},
	`[:^cntrl:]`:  {-1, code8},
	`[:digit:]`:   {+1, code9},
	`[:^digit:]`:  {-1, code9},
	`[:graph:]`:   {+1, code10},
	`[:^graph:]`:  {-1, code10},
	`[:lower:]`:   {+1, code11},
	`[:^lower:]`:  {-1, code11},
	`[:print:]`:   {+1, code12},
	`[:^print:]`:  {-1, code12},
	`[:punct:]`:   {+1, code13},
	`[:^punct:]`:  {-1, code13},
	`[:space:]`:   {+1, code14},
	`[:^space:]`:  {-1, code14},
	`[:upper:]`:   {+1, code15},
	`[:^upper:]`:  {-1, code15},
	`[:word:]`:    {+1, code16},
	`[:^word:]`:   {-1, code16},
	`[:xdigit:]`:  {+1, code17},
	`[:^xdigit:]`: {-1, code17},
}

```

// === FILE: references!/go/src/regexp/syntax/prog.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Compiled program.
// May not belong in this package, but convenient for now.

// A Prog is a compiled regular expression program.
type Prog struct {
	Inst   []Inst
	Start  int // index of start instruction
	NumCap int // number of InstCapture insts in re
}

// An InstOp is an instruction opcode.
type InstOp uint8

const (
	InstAlt InstOp = iota
	InstAltMatch
	InstCapture
	InstEmptyWidth
	InstMatch
	InstFail
	InstNop
	InstRune
	InstRune1
	InstRuneAny
	InstRuneAnyNotNL
)

var instOpNames = []string{
	"InstAlt",
	"InstAltMatch",
	"InstCapture",
	"InstEmptyWidth",
	"InstMatch",
	"InstFail",
	"InstNop",
	"InstRune",
	"InstRune1",
	"InstRuneAny",
	"InstRuneAnyNotNL",
}

func (i InstOp) String() string {
	if uint(i) >= uint(len(instOpNames)) {
		return ""
	}
	return instOpNames[i]
}

// An EmptyOp specifies a kind or mixture of zero-width assertions.
type EmptyOp uint8

const (
	EmptyBeginLine EmptyOp = 1 << iota
	EmptyEndLine
	EmptyBeginText
	EmptyEndText
	EmptyWordBoundary
	EmptyNoWordBoundary
)

// EmptyOpContext returns the zero-width assertions
// satisfied at the position between the runes r1 and r2.
// Passing r1 == -1 indicates that the position is
// at the beginning of the text.
// Passing r2 == -1 indicates that the position is
// at the end of the text.
func EmptyOpContext(r1, r2 rune) EmptyOp {
	var op EmptyOp = EmptyNoWordBoundary
	var boundary byte
	switch {
	case IsWordChar(r1):
		boundary = 1
	case r1 == '\n':
		op |= EmptyBeginLine
	case r1 < 0:
		op |= EmptyBeginText | EmptyBeginLine
	}
	switch {
	case IsWordChar(r2):
		boundary ^= 1
	case r2 == '\n':
		op |= EmptyEndLine
	case r2 < 0:
		op |= EmptyEndText | EmptyEndLine
	}
	if boundary != 0 { // IsWordChar(r1) != IsWordChar(r2)
		op ^= (EmptyWordBoundary | EmptyNoWordBoundary)
	}
	return op
}

// IsWordChar reports whether r is considered a “word character”
// during the evaluation of the \b and \B zero-width assertions.
// These assertions are ASCII-only: the word characters are [A-Za-z0-9_].
func IsWordChar(r rune) bool {
	// Test for lowercase letters first, as these occur more
	// frequently than uppercase letters in common cases.
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' || r == '_'
}

// An Inst is a single instruction in a regular expression program.
type Inst struct {
	Op   InstOp
	Out  uint32 // all but InstMatch, InstFail
	Arg  uint32 // InstAlt, InstAltMatch, InstCapture, InstEmptyWidth
	Rune []rune
}

func (p *Prog) String() string {
	var b strings.Builder
	dumpProg(&b, p)
	return b.String()
}

// skipNop follows any no-op or capturing instructions.
func (p *Prog) skipNop(pc uint32) *Inst {
	i := &p.Inst[pc]
	for i.Op == InstNop || i.Op == InstCapture {
		i = &p.Inst[i.Out]
	}
	return i
}

// op returns i.Op but merges all the Rune special cases into InstRune
func (i *Inst) op() InstOp {
	op := i.Op
	switch op {
	case InstRune1, InstRuneAny, InstRuneAnyNotNL:
		op = InstRune
	}
	return op
}

// Prefix returns a literal string that all matches for the
// regexp must start with. Complete is true if the prefix
// is the entire match.
func (p *Prog) Prefix() (prefix string, complete bool) {
	i := p.skipNop(uint32(p.Start))

	// Avoid allocation of buffer if prefix is empty.
	if i.op() != InstRune || len(i.Rune) != 1 {
		return "", i.Op == InstMatch
	}

	// Have prefix; gather characters.
	var buf strings.Builder
	for i.op() == InstRune && len(i.Rune) == 1 && Flags(i.Arg)&FoldCase == 0 && i.Rune[0] != utf8.RuneError {
		buf.WriteRune(i.Rune[0])
		i = p.skipNop(i.Out)
	}
	return buf.String(), i.Op == InstMatch
}

// StartCond returns the leading empty-width conditions that must
// be true in any match. It returns ^EmptyOp(0) if no matches are possible.
func (p *Prog) StartCond() EmptyOp {
	var flag EmptyOp
	pc := uint32(p.Start)
	i := &p.Inst[pc]
Loop:
	for {
		switch i.Op {
		case InstEmptyWidth:
			flag |= EmptyOp(i.Arg)
		case InstFail:
			return ^EmptyOp(0)
		case InstCapture, InstNop:
			// skip
		default:
			break Loop
		}
		pc = i.Out
		i = &p.Inst[pc]
	}
	return flag
}

const noMatch = -1

// MatchRune reports whether the instruction matches (and consumes) r.
// It should only be called when i.Op == [InstRune].
func (i *Inst) MatchRune(r rune) bool {
	return i.MatchRunePos(r) != noMatch
}

// MatchRunePos checks whether the instruction matches (and consumes) r.
// If so, MatchRunePos returns the index of the matching rune pair
// (or, when len(i.Rune) == 1, rune singleton).
// If not, MatchRunePos returns -1.
// MatchRunePos should only be called when i.Op == [InstRune].
func (i *Inst) MatchRunePos(r rune) int {
	rune := i.Rune

	switch len(rune) {
	case 0:
		return noMatch

	case 1:
		// Special case: single-rune slice is from literal string, not char class.
		r0 := rune[0]
		if r == r0 {
			return 0
		}
		if Flags(i.Arg)&FoldCase != 0 {
			for r1 := unicode.SimpleFold(r0); r1 != r0; r1 = unicode.SimpleFold(r1) {
				if r == r1 {
					return 0
				}
			}
		}
		return noMatch

	case 2:
		if r >= rune[0] && r <= rune[1] {
			return 0
		}
		return noMatch

	case 4, 6, 8:
		// Linear search for a few pairs.
		// Should handle ASCII well.
		for j := 0; j < len(rune); j += 2 {
			if r < rune[j] {
				return noMatch
			}
			if r <= rune[j+1] {
				return j / 2
			}
		}
		return noMatch
	}

	// Otherwise binary search.
	lo := 0
	hi := len(rune) / 2
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if c := rune[2*m]; c <= r {
			if r <= rune[2*m+1] {
				return m
			}
			lo = m + 1
		} else {
			hi = m
		}
	}
	return noMatch
}

// MatchEmptyWidth reports whether the instruction matches
// an empty string between the runes before and after.
// It should only be called when i.Op == [InstEmptyWidth].
func (i *Inst) MatchEmptyWidth(before rune, after rune) bool {
	switch EmptyOp(i.Arg) {
	case EmptyBeginLine:
		return before == '\n' || before == -1
	case EmptyEndLine:
		return after == '\n' || after == -1
	case EmptyBeginText:
		return before == -1
	case EmptyEndText:
		return after == -1
	case EmptyWordBoundary:
		return IsWordChar(before) != IsWordChar(after)
	case EmptyNoWordBoundary:
		return IsWordChar(before) == IsWordChar(after)
	}
	panic("unknown empty width arg")
}

func (i *Inst) String() string {
	var b strings.Builder
	dumpInst(&b, i)
	return b.String()
}

func bw(b *strings.Builder, args ...string) {
	for _, s := range args {
		b.WriteString(s)
	}
}

func dumpProg(b *strings.Builder, p *Prog) {
	for j := range p.Inst {
		i := &p.Inst[j]
		pc := strconv.Itoa(j)
		if len(pc) < 3 {
			b.WriteString("   "[len(pc):])
		}
		if j == p.Start {
			pc += "*"
		}
		bw(b, pc, "\t")
		dumpInst(b, i)
		bw(b, "\n")
	}
}

func u32(i uint32) string {
	return strconv.FormatUint(uint64(i), 10)
}

func dumpInst(b *strings.Builder, i *Inst) {
	switch i.Op {
	case InstAlt:
		bw(b, "alt -> ", u32(i.Out), ", ", u32(i.Arg))
	case InstAltMatch:
		bw(b, "altmatch -> ", u32(i.Out), ", ", u32(i.Arg))
	case InstCapture:
		bw(b, "cap ", u32(i.Arg), " -> ", u32(i.Out))
	case InstEmptyWidth:
		bw(b, "empty ", u32(i.Arg), " -> ", u32(i.Out))
	case InstMatch:
		bw(b, "match")
	case InstFail:
		bw(b, "fail")
	case InstNop:
		bw(b, "nop -> ", u32(i.Out))
	case InstRune:
		if i.Rune == nil {
			// shouldn't happen
			bw(b, "rune <nil>")
		}
		bw(b, "rune ", strconv.QuoteToASCII(string(i.Rune)))
		if Flags(i.Arg)&FoldCase != 0 {
			bw(b, "/i")
		}
		bw(b, " -> ", u32(i.Out))
	case InstRune1:
		bw(b, "rune1 ", strconv.QuoteToASCII(string(i.Rune)), " -> ", u32(i.Out))
	case InstRuneAny:
		bw(b, "any -> ", u32(i.Out))
	case InstRuneAnyNotNL:
		bw(b, "anynotnl -> ", u32(i.Out))
	}
}

```

// === FILE: references!/go/src/regexp/syntax/regexp.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

// Note to implementers:
// In this package, re is always a *Regexp and r is always a rune.

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// A Regexp is a node in a regular expression syntax tree.
type Regexp struct {
	Op       Op // operator
	Flags    Flags
	Sub      []*Regexp  // subexpressions, if any
	Sub0     [1]*Regexp // storage for short Sub
	Rune     []rune     // matched runes, for OpLiteral, OpCharClass
	Rune0    [2]rune    // storage for short Rune
	Min, Max int        // min, max for OpRepeat
	Cap      int        // capturing index, for OpCapture
	Name     string     // capturing name, for OpCapture
}

//go:generate stringer -type Op -trimprefix Op

// An Op is a single regular expression operator.
type Op uint8

// Operators are listed in precedence order, tightest binding to weakest.
// Character class operators are listed simplest to most complex
// (OpLiteral, OpCharClass, OpAnyCharNotNL, OpAnyChar).

const (
	OpNoMatch        Op = 1 + iota // matches no strings
	OpEmptyMatch                   // matches empty string
	OpLiteral                      // matches Runes sequence
	OpCharClass                    // matches Runes interpreted as range pair list
	OpAnyCharNotNL                 // matches any character except newline
	OpAnyChar                      // matches any character
	OpBeginLine                    // matches empty string at beginning of line
	OpEndLine                      // matches empty string at end of line
	OpBeginText                    // matches empty string at beginning of text
	OpEndText                      // matches empty string at end of text
	OpWordBoundary                 // matches word boundary `\b`
	OpNoWordBoundary               // matches word non-boundary `\B`
	OpCapture                      // capturing subexpression with index Cap, optional name Name
	OpStar                         // matches Sub[0] zero or more times
	OpPlus                         // matches Sub[0] one or more times
	OpQuest                        // matches Sub[0] zero or one times
	OpRepeat                       // matches Sub[0] at least Min times, at most Max (Max == -1 is no limit)
	OpConcat                       // matches concatenation of Subs
	OpAlternate                    // matches alternation of Subs
)

const opPseudo Op = 128 // where pseudo-ops start

// Equal reports whether x and y have identical structure.
func (x *Regexp) Equal(y *Regexp) bool {
	if x == nil || y == nil {
		return x == y
	}
	if x.Op != y.Op {
		return false
	}
	switch x.Op {
	case OpEndText:
		// The parse flags remember whether this is \z or \Z.
		if x.Flags&WasDollar != y.Flags&WasDollar {
			return false
		}

	case OpLiteral, OpCharClass:
		return x.Flags&FoldCase == y.Flags&FoldCase && slices.Equal(x.Rune, y.Rune)

	case OpAlternate, OpConcat:
		return slices.EqualFunc(x.Sub, y.Sub, (*Regexp).Equal)

	case OpStar, OpPlus, OpQuest:
		if x.Flags&NonGreedy != y.Flags&NonGreedy || !x.Sub[0].Equal(y.Sub[0]) {
			return false
		}

	case OpRepeat:
		if x.Flags&NonGreedy != y.Flags&NonGreedy || x.Min != y.Min || x.Max != y.Max || !x.Sub[0].Equal(y.Sub[0]) {
			return false
		}

	case OpCapture:
		if x.Cap != y.Cap || x.Name != y.Name || !x.Sub[0].Equal(y.Sub[0]) {
			return false
		}
	}
	return true
}

// printFlags is a bit set indicating which flags (including non-capturing parens) to print around a regexp.
type printFlags uint8

const (
	flagI    printFlags = 1 << iota // (?i:
	flagM                           // (?m:
	flagS                           // (?s:
	flagOff                         // )
	flagPrec                        // (?: )
	negShift = 5                    // flagI<<negShift is (?-i:
)

// addSpan enables the flags f around start..last,
// by setting flags[start] = f and flags[last] = flagOff.
func addSpan(start, last *Regexp, f printFlags, flags *map[*Regexp]printFlags) {
	if *flags == nil {
		*flags = make(map[*Regexp]printFlags)
	}
	(*flags)[start] = f
	(*flags)[last] |= flagOff // maybe start==last
}

// calcFlags calculates the flags to print around each subexpression in re,
// storing that information in (*flags)[sub] for each affected subexpression.
// The first time an entry needs to be written to *flags, calcFlags allocates the map.
// calcFlags also calculates the flags that must be active or can't be active
// around re and returns those flags.
func calcFlags(re *Regexp, flags *map[*Regexp]printFlags) (must, cant printFlags) {
	switch re.Op {
	default:
		return 0, 0

	case OpLiteral:
		// If literal is fold-sensitive, return (flagI, 0) or (0, flagI)
		// according to whether (?i) is active.
		// If literal is not fold-sensitive, return 0, 0.
		for _, r := range re.Rune {
			if minFold <= r && r <= maxFold && unicode.SimpleFold(r) != r {
				if re.Flags&FoldCase != 0 {
					return flagI, 0
				} else {
					return 0, flagI
				}
			}
		}
		return 0, 0

	case OpCharClass:
		// If literal is fold-sensitive, return 0, flagI - (?i) has been compiled out.
		// If literal is not fold-sensitive, return 0, 0.
		for i := 0; i < len(re.Rune); i += 2 {
			lo := max(minFold, re.Rune[i])
			hi := min(maxFold, re.Rune[i+1])
			for r := lo; r <= hi; r++ {
				for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
					if !(lo <= f && f <= hi) && !inCharClass(f, re.Rune) {
						return 0, flagI
					}
				}
			}
		}
		return 0, 0

	case OpAnyCharNotNL: // (?-s).
		return 0, flagS

	case OpAnyChar: // (?s).
		return flagS, 0

	case OpBeginLine, OpEndLine: // (?m)^ (?m)$
		return flagM, 0

	case OpEndText:
		if re.Flags&WasDollar != 0 { // (?-m)$
			return 0, flagM
		}
		return 0, 0

	case OpCapture, OpStar, OpPlus, OpQuest, OpRepeat:
		return calcFlags(re.Sub[0], flags)

	case OpConcat, OpAlternate:
		// Gather the must and cant for each subexpression.
		// When we find a conflicting subexpression, insert the necessary
		// flags around the previously identified span and start over.
		var must, cant, allCant printFlags
		start := 0
		last := 0
		did := false
		for i, sub := range re.Sub {
			subMust, subCant := calcFlags(sub, flags)
			if must&subCant != 0 || subMust&cant != 0 {
				if must != 0 {
					addSpan(re.Sub[start], re.Sub[last], must, flags)
				}
				must = 0
				cant = 0
				start = i
				did = true
			}
			must |= subMust
			cant |= subCant
			allCant |= subCant
			if subMust != 0 {
				last = i
			}
			if must == 0 && start == i {
				start++
			}
		}
		if !did {
			// No conflicts: pass the accumulated must and cant upward.
			return must, cant
		}
		if must != 0 {
			// Conflicts found; need to finish final span.
			addSpan(re.Sub[start], re.Sub[last], must, flags)
		}
		return 0, allCant
	}
}

// writeRegexp writes the Perl syntax for the regular expression re to b.
func writeRegexp(b *strings.Builder, re *Regexp, f printFlags, flags map[*Regexp]printFlags) {
	f |= flags[re]
	if f&flagPrec != 0 && f&^(flagOff|flagPrec) != 0 && f&flagOff != 0 {
		// flagPrec is redundant with other flags being added and terminated
		f &^= flagPrec
	}
	if f&^(flagOff|flagPrec) != 0 {
		b.WriteString(`(?`)
		if f&flagI != 0 {
			b.WriteString(`i`)
		}
		if f&flagM != 0 {
			b.WriteString(`m`)
		}
		if f&flagS != 0 {
			b.WriteString(`s`)
		}
		if f&((flagM|flagS)<<negShift) != 0 {
			b.WriteString(`-`)
			if f&(flagM<<negShift) != 0 {
				b.WriteString(`m`)
			}
			if f&(flagS<<negShift) != 0 {
				b.WriteString(`s`)
			}
		}
		b.WriteString(`:`)
	}
	if f&flagOff != 0 {
		defer b.WriteString(`)`)
	}
	if f&flagPrec != 0 {
		b.WriteString(`(?:`)
		defer b.WriteString(`)`)
	}

	switch re.Op {
	default:
		b.WriteString("<invalid op" + strconv.Itoa(int(re.Op)) + ">")
	case OpNoMatch:
		b.WriteString(`[^\x00-\x{10FFFF}]`)
	case OpEmptyMatch:
		b.WriteString(`(?:)`)
	case OpLiteral:
		for _, r := range re.Rune {
			escape(b, r, false)
		}
	case OpCharClass:
		if len(re.Rune)%2 != 0 {
			b.WriteString(`[invalid char class]`)
			break
		}
		b.WriteRune('[')
		if len(re.Rune) == 0 {
			b.WriteString(`^\x00-\x{10FFFF}`)
		} else if re.Rune[0] == 0 && re.Rune[len(re.Rune)-1] == unicode.MaxRune && len(re.Rune) > 2 {
			// Contains 0 and MaxRune. Probably a negated class.
			// Print the gaps.
			b.WriteRune('^')
			for i := 1; i < len(re.Rune)-1; i += 2 {
				lo, hi := re.Rune[i]+1, re.Rune[i+1]-1
				escape(b, lo, lo == '-')
				if lo != hi {
					if hi != lo+1 {
						b.WriteRune('-')
					}
					escape(b, hi, hi == '-')
				}
			}
		} else {
			for i := 0; i < len(re.Rune); i += 2 {
				lo, hi := re.Rune[i], re.Rune[i+1]
				escape(b, lo, lo == '-')
				if lo != hi {
					if hi != lo+1 {
						b.WriteRune('-')
					}
					escape(b, hi, hi == '-')
				}
			}
		}
		b.WriteRune(']')
	case OpAnyCharNotNL, OpAnyChar:
		b.WriteString(`.`)
	case OpBeginLine:
		b.WriteString(`^`)
	case OpEndLine:
		b.WriteString(`$`)
	case OpBeginText:
		b.WriteString(`\A`)
	case OpEndText:
		if re.Flags&WasDollar != 0 {
			b.WriteString(`$`)
		} else {
			b.WriteString(`\z`)
		}
	case OpWordBoundary:
		b.WriteString(`\b`)
	case OpNoWordBoundary:
		b.WriteString(`\B`)
	case OpCapture:
		if re.Name != "" {
			b.WriteString(`(?P<`)
			b.WriteString(re.Name)
			b.WriteRune('>')
		} else {
			b.WriteRune('(')
		}
		if re.Sub[0].Op != OpEmptyMatch {
			writeRegexp(b, re.Sub[0], flags[re.Sub[0]], flags)
		}
		b.WriteRune(')')
	case OpStar, OpPlus, OpQuest, OpRepeat:
		p := printFlags(0)
		sub := re.Sub[0]
		if sub.Op > OpCapture || sub.Op == OpLiteral && len(sub.Rune) > 1 {
			p = flagPrec
		}
		writeRegexp(b, sub, p, flags)

		switch re.Op {
		case OpStar:
			b.WriteRune('*')
		case OpPlus:
			b.WriteRune('+')
		case OpQuest:
			b.WriteRune('?')
		case OpRepeat:
			b.WriteRune('{')
			b.WriteString(strconv.Itoa(re.Min))
			if re.Max != re.Min {
				b.WriteRune(',')
				if re.Max >= 0 {
					b.WriteString(strconv.Itoa(re.Max))
				}
			}
			b.WriteRune('}')
		}
		if re.Flags&NonGreedy != 0 {
			b.WriteRune('?')
		}
	case OpConcat:
		for _, sub := range re.Sub {
			p := printFlags(0)
			if sub.Op == OpAlternate {
				p = flagPrec
			}
			writeRegexp(b, sub, p, flags)
		}
	case OpAlternate:
		for i, sub := range re.Sub {
			if i > 0 {
				b.WriteRune('|')
			}
			writeRegexp(b, sub, 0, flags)
		}
	}
}

func (re *Regexp) String() string {
	var b strings.Builder
	var flags map[*Regexp]printFlags
	must, cant := calcFlags(re, &flags)
	must |= (cant &^ flagI) << negShift
	if must != 0 {
		must |= flagOff
	}
	writeRegexp(&b, re, must, flags)
	return b.String()
}

const meta = `\.+*?()|[]{}^$`

func escape(b *strings.Builder, r rune, force bool) {
	if unicode.IsPrint(r) {
		if strings.ContainsRune(meta, r) || force {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
		return
	}

	switch r {
	case '\a':
		b.WriteString(`\a`)
	case '\f':
		b.WriteString(`\f`)
	case '\n':
		b.WriteString(`\n`)
	case '\r':
		b.WriteString(`\r`)
	case '\t':
		b.WriteString(`\t`)
	case '\v':
		b.WriteString(`\v`)
	default:
		if r < 0x100 {
			b.WriteString(`\x`)
			s := strconv.FormatInt(int64(r), 16)
			if len(s) == 1 {
				b.WriteRune('0')
			}
			b.WriteString(s)
			break
		}
		b.WriteString(`\x{`)
		b.WriteString(strconv.FormatInt(int64(r), 16))
		b.WriteString(`}`)
	}
}

// MaxCap walks the regexp to find the maximum capture index.
func (re *Regexp) MaxCap() int {
	m := 0
	if re.Op == OpCapture {
		m = re.Cap
	}
	for _, sub := range re.Sub {
		if n := sub.MaxCap(); m < n {
			m = n
		}
	}
	return m
}

// CapNames walks the regexp to find the names of capturing groups.
func (re *Regexp) CapNames() []string {
	names := make([]string, re.MaxCap()+1)
	re.capNames(names)
	return names
}

func (re *Regexp) capNames(names []string) {
	if re.Op == OpCapture {
		names[re.Cap] = re.Name
	}
	for _, sub := range re.Sub {
		sub.capNames(names)
	}
}

```

// === FILE: references!/go/src/regexp/syntax/simplify.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syntax

// Simplify returns a regexp equivalent to re but without counted repetitions
// and with various other simplifications, such as rewriting /(?:a+)+/ to /a+/.
// The resulting regexp will execute correctly but its string representation
// will not produce the same parse tree, because capturing parentheses
// may have been duplicated or removed. For example, the simplified form
// for /(x){1,2}/ is /(x)(x)?/ but both parentheses capture as $1.
// The returned regexp may share structure with or be the original.
func (re *Regexp) Simplify() *Regexp {
	if re == nil {
		return nil
	}
	switch re.Op {
	case OpCapture, OpConcat, OpAlternate:
		// Simplify children, building new Regexp if children change.
		nre := re
		for i, sub := range re.Sub {
			nsub := sub.Simplify()
			if nre == re && nsub != sub {
				// Start a copy.
				nre = new(Regexp)
				*nre = *re
				nre.Rune = nil
				nre.Sub = append(nre.Sub0[:0], re.Sub[:i]...)
			}
			if nre != re {
				nre.Sub = append(nre.Sub, nsub)
			}
		}
		return nre

	case OpStar, OpPlus, OpQuest:
		sub := re.Sub[0].Simplify()
		return simplify1(re.Op, re.Flags, sub, re)

	case OpRepeat:
		// Special special case: x{0} matches the empty string
		// and doesn't even need to consider x.
		if re.Min == 0 && re.Max == 0 {
			return &Regexp{Op: OpEmptyMatch}
		}

		// The fun begins.
		sub := re.Sub[0].Simplify()

		// x{n,} means at least n matches of x.
		if re.Max == -1 {
			// Special case: x{0,} is x*.
			if re.Min == 0 {
				return simplify1(OpStar, re.Flags, sub, nil)
			}

			// Special case: x{1,} is x+.
			if re.Min == 1 {
				return simplify1(OpPlus, re.Flags, sub, nil)
			}

			// General case: x{4,} is xxxx+.
			nre := &Regexp{Op: OpConcat}
			nre.Sub = nre.Sub0[:0]
			for i := 0; i < re.Min-1; i++ {
				nre.Sub = append(nre.Sub, sub)
			}
			nre.Sub = append(nre.Sub, simplify1(OpPlus, re.Flags, sub, nil))
			return nre
		}

		// Special case x{0} handled above.

		// Special case: x{1} is just x.
		if re.Min == 1 && re.Max == 1 {
			return sub
		}

		// General case: x{n,m} means n copies of x and m copies of x?
		// The machine will do less work if we nest the final m copies,
		// so that x{2,5} = xx(x(x(x)?)?)?

		// Build leading prefix: xx.
		var prefix *Regexp
		if re.Min > 0 {
			prefix = &Regexp{Op: OpConcat}
			prefix.Sub = prefix.Sub0[:0]
			for i := 0; i < re.Min; i++ {
				prefix.Sub = append(prefix.Sub, sub)
			}
		}

		// Build and attach suffix: (x(x(x)?)?)?
		if re.Max > re.Min {
			suffix := simplify1(OpQuest, re.Flags, sub, nil)
			for i := re.Min + 1; i < re.Max; i++ {
				nre2 := &Regexp{Op: OpConcat}
				nre2.Sub = append(nre2.Sub0[:0], sub, suffix)
				suffix = simplify1(OpQuest, re.Flags, nre2, nil)
			}
			if prefix == nil {
				return suffix
			}
			prefix.Sub = append(prefix.Sub, suffix)
		}
		if prefix != nil {
			return prefix
		}

		// Some degenerate case like min > max or min < max < 0.
		// Handle as impossible match.
		return &Regexp{Op: OpNoMatch}
	}

	return re
}

// simplify1 implements Simplify for the unary OpStar,
// OpPlus, and OpQuest operators. It returns the simple regexp
// equivalent to
//
//	Regexp{Op: op, Flags: flags, Sub: {sub}}
//
// under the assumption that sub is already simple, and
// without first allocating that structure. If the regexp
// to be returned turns out to be equivalent to re, simplify1
// returns re instead.
//
// simplify1 is factored out of Simplify because the implementation
// for other operators generates these unary expressions.
// Letting them call simplify1 makes sure the expressions they
// generate are simple.
func simplify1(op Op, flags Flags, sub, re *Regexp) *Regexp {
	// Special case: repeat the empty string as much as
	// you want, but it's still the empty string.
	if sub.Op == OpEmptyMatch {
		return sub
	}
	// The operators are idempotent if the flags match.
	if op == sub.Op && flags&NonGreedy == sub.Flags&NonGreedy {
		return sub
	}
	if re != nil && re.Op == op && re.Flags&NonGreedy == flags&NonGreedy && sub == re.Sub[0] {
		return re
	}

	re = &Regexp{Op: op, Flags: flags}
	re.Sub = append(re.Sub0[:0], sub)
	return re
}

```

