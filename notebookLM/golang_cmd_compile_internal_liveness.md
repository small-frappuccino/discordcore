# Domain Architecture: cmd/compile/internal/liveness

## Layout Topology
```text
cmd/compile/internal/liveness/
├── arg.go
├── bvset.go
├── intervals.go
├── mergelocals.go
└── plive.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/compile/internal/liveness/arg.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package liveness

import (
	"fmt"
	"internal/abi"

	"cmd/compile/internal/base"
	"cmd/compile/internal/bitvec"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/ssa"
	"cmd/internal/obj"
)

// Argument liveness tracking.
//
// For arguments passed in registers, this file tracks if their spill slots
// are live for runtime traceback. An argument spill slot is live at a PC
// if we know that an actual value has stored into it at or before this point.
//
// Stack args are always live and not tracked in this code. Stack args are
// laid out before register spill slots, so we emit the smallest offset that
// needs tracking. Slots before that offset are always live. That offset is
// usually the offset of the first spill slot. But if the first spill slot is
// always live (e.g. if it is address-taken), it will be the offset of a later
// one.
//
// The liveness information is emitted as a FUNCDATA and a PCDATA.
//
// FUNCDATA format:
// - start (smallest) offset that needs tracking (1 byte)
// - a list of bitmaps.
//   In a bitmap bit i is set if the i-th spill slot is live.
//
// At a PC where the liveness info changes, a PCDATA indicates the
// byte offset of the liveness map in the FUNCDATA. PCDATA -1 is a
// special case indicating all slots are live (for binary size
// saving).

const allLiveIdx = -1

// name and offset
type nameOff struct {
	n   *ir.Name
	off int64
}

func (a nameOff) FrameOffset() int64 { return a.n.FrameOffset() + a.off }
func (a nameOff) String() string     { return fmt.Sprintf("%v+%d", a.n, a.off) }

type blockArgEffects struct {
	livein  bitvec.BitVec // variables live at block entry
	liveout bitvec.BitVec // variables live at block exit
}

type argLiveness struct {
	fn   *ir.Func
	f    *ssa.Func
	args []nameOff         // name and offset of spill slots
	idx  map[nameOff]int32 // index in args

	be []blockArgEffects // indexed by block ID

	bvset bvecSet // Set of liveness bitmaps, used for uniquifying.

	// Liveness map indices at each Value (where it changes) and Block entry.
	// During the computation the indices are temporarily index to bvset.
	// At the end they will be index (offset) to the output funcdata (changed
	// in (*argLiveness).emit).
	blockIdx map[ssa.ID]int
	valueIdx map[ssa.ID]int
}

// ArgLiveness computes the liveness information of register argument spill slots.
// An argument's spill slot is "live" if we know it contains a meaningful value,
// that is, we have stored the register value to it.
// Returns the liveness map indices at each Block entry and at each Value (where
// it changes).
func ArgLiveness(fn *ir.Func, f *ssa.Func, pp *objw.Progs) (blockIdx, valueIdx map[ssa.ID]int) {
	if f.OwnAux.ABIInfo().InRegistersUsed() == 0 || base.Flag.N != 0 {
		// No register args. Nothing to emit.
		// Or if -N is used we spill everything upfront so it is always live.
		return nil, nil
	}

	lv := &argLiveness{
		fn:       fn,
		f:        f,
		idx:      make(map[nameOff]int32),
		be:       make([]blockArgEffects, f.NumBlocks()),
		blockIdx: make(map[ssa.ID]int),
		valueIdx: make(map[ssa.ID]int),
	}
	// Gather all register arg spill slots.
	for _, a := range f.OwnAux.ABIInfo().InParams() {
		n := a.Name
		if n == nil || len(a.Registers) == 0 {
			continue
		}
		_, offs := a.RegisterTypesAndOffsets()
		for _, off := range offs {
			if n.FrameOffset()+off > 0xff {
				// We only print a limited number of args, with stack
				// offsets no larger than 255.
				continue
			}
			lv.args = append(lv.args, nameOff{n, off})
		}
	}
	if len(lv.args) > 10 {
		lv.args = lv.args[:10] // We print no more than 10 args.
	}

	// We spill address-taken or non-SSA-able value upfront, so they are always live.
	alwaysLive := func(n *ir.Name) bool { return n.Addrtaken() || !ssa.CanSSA(n.Type()) }

	// We'll emit the smallest offset for the slots that need liveness info.
	// No need to include a slot with a lower offset if it is always live.
	for len(lv.args) > 0 && alwaysLive(lv.args[0].n) {
		lv.args = lv.args[1:]
	}
	if len(lv.args) == 0 {
		return // everything is always live
	}

	for i, a := range lv.args {
		lv.idx[a] = int32(i)
	}

	nargs := int32(len(lv.args))
	bulk := bitvec.NewBulk(nargs, int32(len(f.Blocks)*2))
	for _, b := range f.Blocks {
		be := &lv.be[b.ID]
		be.livein = bulk.Next()
		be.liveout = bulk.Next()

		// initialize to all 1s, so we can AND them
		be.livein.Not()
		be.liveout.Not()
	}

	entrybe := &lv.be[f.Entry.ID]
	entrybe.livein.Clear()
	for i, a := range lv.args {
		if alwaysLive(a.n) {
			entrybe.livein.Set(int32(i))
		}
	}

	// Visit blocks in reverse-postorder, compute block effects.
	po := f.Postorder()
	for i := len(po) - 1; i >= 0; i-- {
		b := po[i]
		be := &lv.be[b.ID]

		// A slot is live at block entry if it is live in all predecessors.
		for _, pred := range b.Preds {
			pb := pred.Block()
			be.livein.And(be.livein, lv.be[pb.ID].liveout)
		}

		be.liveout.Copy(be.livein)
		for _, v := range b.Values {
			lv.valueEffect(v, be.liveout)
		}
	}

	// Coalesce identical live vectors. Compute liveness indices at each PC
	// where it changes.
	live := bitvec.New(nargs)
	addToSet := func(bv bitvec.BitVec) (int, bool) {
		if bv.Count() == int(nargs) { // special case for all live
			return allLiveIdx, false
		}
		return lv.bvset.add(bv)
	}
	for _, b := range lv.f.Blocks {
		be := &lv.be[b.ID]
		lv.blockIdx[b.ID], _ = addToSet(be.livein)

		live.Copy(be.livein)
		var lastv *ssa.Value
		for i, v := range b.Values {
			if lv.valueEffect(v, live) {
				// Record that liveness changes but not emit a map now.
				// For a sequence of StoreRegs we only need to emit one
				// at last.
				lastv = v
			}
			if lastv != nil && (mayFault(v) || i == len(b.Values)-1) {
				// Emit the liveness map if it may fault or at the end of
				// the block. We may need a traceback if the instruction
				// may cause a panic.
				var added bool
				lv.valueIdx[lastv.ID], added = addToSet(live)
				if added {
					// live is added to bvset and we cannot modify it now.
					// Make a copy.
					t := live
					live = bitvec.New(nargs)
					live.Copy(t)
				}
				lastv = nil
			}
		}

		// Sanity check.
		if !live.Eq(be.liveout) {
			panic("wrong arg liveness map at block end")
		}
	}

	// Emit funcdata symbol, update indices to offsets in the symbol data.
	lsym := lv.emit()
	fn.LSym.Func().ArgLiveInfo = lsym

	//lv.print()

	p := pp.Prog(obj.AFUNCDATA)
	p.From.SetConst(abi.FUNCDATA_ArgLiveInfo)
	p.To.Type = obj.TYPE_MEM
	p.To.Name = obj.NAME_EXTERN
	p.To.Sym = lsym

	return lv.blockIdx, lv.valueIdx
}

// valueEffect applies the effect of v to live, return whether it is changed.
func (lv *argLiveness) valueEffect(v *ssa.Value, live bitvec.BitVec) bool {
	if v.Op != ssa.OpStoreReg { // TODO: include other store instructions?
		return false
	}
	n, off := ssa.AutoVar(v)
	if n.Class != ir.PPARAM {
		return false
	}
	i, ok := lv.idx[nameOff{n, off}]
	if !ok || live.Get(i) {
		return false
	}
	live.Set(i)
	return true
}

func mayFault(v *ssa.Value) bool {
	switch v.Op {
	case ssa.OpLoadReg, ssa.OpStoreReg, ssa.OpCopy, ssa.OpPhi,
		ssa.OpVarDef, ssa.OpVarLive, ssa.OpKeepAlive,
		ssa.OpSelect0, ssa.OpSelect1, ssa.OpSelectN, ssa.OpMakeResult,
		ssa.OpConvert, ssa.OpInlMark, ssa.OpGetG:
		return false
	}
	if len(v.Args) == 0 {
		return false // assume constant op cannot fault
	}
	return true // conservatively assume all other ops could fault
}

func (lv *argLiveness) print() {
	fmt.Println("argument liveness:", lv.f.Name)
	live := bitvec.New(int32(len(lv.args)))
	for _, b := range lv.f.Blocks {
		be := &lv.be[b.ID]

		fmt.Printf("%v: live in: ", b)
		lv.printLivenessVec(be.livein)
		if idx, ok := lv.blockIdx[b.ID]; ok {
			fmt.Printf("   #%d", idx)
		}
		fmt.Println()

		for _, v := range b.Values {
			if lv.valueEffect(v, live) {
				fmt.Printf("  %v: ", v)
				lv.printLivenessVec(live)
				if idx, ok := lv.valueIdx[v.ID]; ok {
					fmt.Printf("   #%d", idx)
				}
				fmt.Println()
			}
		}

		fmt.Printf("%v: live out: ", b)
		lv.printLivenessVec(be.liveout)
		fmt.Println()
	}
	fmt.Println("liveness maps data:", lv.fn.LSym.Func().ArgLiveInfo.P)
}

func (lv *argLiveness) printLivenessVec(bv bitvec.BitVec) {
	for i, a := range lv.args {
		if bv.Get(int32(i)) {
			fmt.Printf("%v ", a)
		}
	}
}

func (lv *argLiveness) emit() *obj.LSym {
	livenessMaps := lv.bvset.extractUnique()

	// stack offsets of register arg spill slots
	argOffsets := make([]uint8, len(lv.args))
	for i, a := range lv.args {
		off := a.FrameOffset()
		if off > 0xff {
			panic("offset too large")
		}
		argOffsets[i] = uint8(off)
	}

	idx2off := make([]int, len(livenessMaps))

	lsym := base.Ctxt.Lookup(lv.fn.LSym.Name + ".argliveinfo")
	lsym.Set(obj.AttrContentAddressable, true)
	lsym.Align = 1

	off := objw.Uint8(lsym, 0, argOffsets[0]) // smallest offset that needs liveness info.
	for idx, live := range livenessMaps {
		idx2off[idx] = off
		off = objw.BitVec(lsym, off, live)
	}

	// Update liveness indices to offsets.
	for i, x := range lv.blockIdx {
		if x != allLiveIdx {
			lv.blockIdx[i] = idx2off[x]
		}
	}
	for i, x := range lv.valueIdx {
		if x != allLiveIdx {
			lv.valueIdx[i] = idx2off[x]
		}
	}

	return lsym
}

```

// === FILE: references/go/src/cmd/compile/internal/liveness/bvset.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package liveness

import "cmd/compile/internal/bitvec"

// FNV-1 hash function constants.
const (
	h0 = 2166136261
	hp = 16777619
)

// bvecSet is a set of bvecs, in initial insertion order.
type bvecSet struct {
	index []int           // hash -> uniq index. -1 indicates empty slot.
	uniq  []bitvec.BitVec // unique bvecs, in insertion order
}

func (m *bvecSet) grow() {
	// Allocate new index.
	n := len(m.index) * 2
	if n == 0 {
		n = 32
	}
	newIndex := make([]int, n)
	for i := range newIndex {
		newIndex[i] = -1
	}

	// Rehash into newIndex.
	for i, bv := range m.uniq {
		h := hashbitmap(h0, bv) % uint32(len(newIndex))
		for {
			j := newIndex[h]
			if j < 0 {
				newIndex[h] = i
				break
			}
			h++
			if h == uint32(len(newIndex)) {
				h = 0
			}
		}
	}
	m.index = newIndex
}

// add adds bv to the set and returns its index in m.extractUnique,
// and whether it is newly added.
// If it is newly added, the caller must not modify bv after this.
func (m *bvecSet) add(bv bitvec.BitVec) (int, bool) {
	if len(m.uniq)*4 >= len(m.index) {
		m.grow()
	}

	index := m.index
	h := hashbitmap(h0, bv) % uint32(len(index))
	for {
		j := index[h]
		if j < 0 {
			// New bvec.
			index[h] = len(m.uniq)
			m.uniq = append(m.uniq, bv)
			return len(m.uniq) - 1, true
		}
		jlive := m.uniq[j]
		if bv.Eq(jlive) {
			// Existing bvec.
			return j, false
		}

		h++
		if h == uint32(len(index)) {
			h = 0
		}
	}
}

// extractUnique returns this slice of unique bit vectors in m, as
// indexed by the result of bvecSet.add.
func (m *bvecSet) extractUnique() []bitvec.BitVec {
	return m.uniq
}

func hashbitmap(h uint32, bv bitvec.BitVec) uint32 {
	n := int((bv.N + 31) / 32)
	for i := 0; i < n; i++ {
		w := bv.B[i]
		h = (h * hp) ^ (w & 0xff)
		h = (h * hp) ^ ((w >> 8) & 0xff)
		h = (h * hp) ^ ((w >> 16) & 0xff)
		h = (h * hp) ^ ((w >> 24) & 0xff)
	}

	return h
}

```

// === FILE: references/go/src/cmd/compile/internal/liveness/intervals.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package liveness

// This file defines an "Intervals" helper type that stores a
// sorted sequence of disjoint ranges or intervals. An Intervals
// example: { [0,5) [9-12) [100,101) }, which corresponds to the
// numbers 0-4, 9-11, and 100. Once an Intervals object is created, it
// can be tested to see if it has any overlap with another Intervals
// object, or it can be merged with another Intervals object to form a
// union of the two.
//
// The intended use case for this helper is in describing object or
// variable lifetime ranges within a linearized program representation
// where each IR instruction has a slot or index. Example:
//
//          b1:
//  0        VarDef abc
//  1        memset(abc,0)
//  2        VarDef xyz
//  3        memset(xyz,0)
//  4        abc.f1 = 2
//  5        xyz.f3 = 9
//  6        if q goto B4
//  7 B3:    z = xyz.x
//  8        goto B5
//  9 B4:    z = abc.x
//           // fallthrough
// 10 B5:    z++
//
// To describe the lifetime of the variables above we might use these
// intervals:
//
//    "abc"   [1,7), [9,10)
//    "xyz"   [3,8)
//
// Clients can construct an Intervals object from a given IR sequence
// using the "IntervalsBuilder" helper abstraction (one builder per
// candidate variable), by making a
// backwards sweep and invoking the Live/Kill methods to note the
// starts and end of a given lifetime. For the example above, we would
// expect to see this sequence of calls to Live/Kill:
//
//    abc:  Live(9), Kill(8), Live(6), Kill(0)
//    xyz:  Live(8), Kill(2)

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

const debugtrace = false

// Interval hols the range [st,en).
type Interval struct {
	st, en int
}

// Intervals is a sequence of sorted, disjoint intervals.
type Intervals []Interval

func (i Interval) String() string {
	return fmt.Sprintf("[%d,%d)", i.st, i.en)
}

// TEMPORARY until bootstrap version catches up.
func imin(i, j int) int {
	if i < j {
		return i
	}
	return j
}

// TEMPORARY until bootstrap version catches up.
func imax(i, j int) int {
	if i > j {
		return i
	}
	return j
}

// Overlaps returns true if here is any overlap between i and i2.
func (i Interval) Overlaps(i2 Interval) bool {
	return (imin(i.en, i2.en) - imax(i.st, i2.st)) > 0
}

// adjacent returns true if the start of one interval is equal to the
// end of another interval (e.g. they represent consecutive ranges).
func (i1 Interval) adjacent(i2 Interval) bool {
	return i1.en == i2.st || i2.en == i1.st
}

// MergeInto merges interval i2 into i1. This version happens to
// require that the two intervals either overlap or are adjacent.
func (i1 *Interval) MergeInto(i2 Interval) error {
	if !i1.Overlaps(i2) && !i1.adjacent(i2) {
		return fmt.Errorf("merge method invoked on non-overlapping/non-adjacent")
	}
	i1.st = imin(i1.st, i2.st)
	i1.en = imax(i1.en, i2.en)
	return nil
}

// IntervalsBuilder is a helper for constructing intervals based on
// live dataflow sets for a series of BBs where we're making a
// backwards pass over each BB looking for uses and kills. The
// expected use case is:
//
//   - invoke MakeIntervalsBuilder to create a new object "b"
//   - series of calls to b.Live/b.Kill based on a backwards reverse layout
//     order scan over instructions
//   - invoke b.Finish() to produce final set
//
// See the Live method comment for an IR example.
type IntervalsBuilder struct {
	s Intervals
	// index of last instruction visited plus 1
	lidx int
}

func (c *IntervalsBuilder) last() int {
	return c.lidx - 1
}

func (c *IntervalsBuilder) setLast(x int) {
	c.lidx = x + 1
}

func (c *IntervalsBuilder) Finish() (Intervals, error) {
	// Reverse intervals list and check.
	slices.Reverse(c.s)
	if err := check(c.s); err != nil {
		return Intervals{}, err
	}
	r := c.s
	return r, nil
}

// Live method should be invoked on instruction at position p if instr
// contains an upwards-exposed use of a resource. See the example in
// the comment at the beginning of this file for an example.
func (c *IntervalsBuilder) Live(pos int) error {
	if pos < 0 {
		return fmt.Errorf("bad pos, negative")
	}
	if c.last() == -1 {
		c.setLast(pos)
		if debugtrace {
			fmt.Fprintf(os.Stderr, "=-= begin lifetime at pos=%d\n", pos)
		}
		c.s = append(c.s, Interval{st: pos, en: pos + 1})
		return nil
	}
	if pos >= c.last() {
		return fmt.Errorf("pos not decreasing")
	}
	// extend lifetime across this pos
	c.s[len(c.s)-1].st = pos
	c.setLast(pos)
	return nil
}

// Kill method should be invoked on instruction at position p if instr
// should be treated as having a kill (lifetime end) for the
// resource. See the example in the comment at the beginning of this
// file for an example. Note that if we see a kill at position K for a
// resource currently live since J, this will result in a lifetime
// segment of [K+1,J+1), the assumption being that the first live
// instruction will be the one after the kill position, not the kill
// position itself.
func (c *IntervalsBuilder) Kill(pos int) error {
	if pos < 0 {
		return fmt.Errorf("bad pos, negative")
	}
	if c.last() == -1 {
		return nil
	}
	if pos >= c.last() {
		return fmt.Errorf("pos not decreasing")
	}
	c.s[len(c.s)-1].st = pos + 1
	// terminate lifetime
	c.setLast(-1)
	if debugtrace {
		fmt.Fprintf(os.Stderr, "=-= term lifetime at pos=%d\n", pos)
	}
	return nil
}

// check examines the intervals in "is" to try to find internal
// inconsistencies or problems.
func check(is Intervals) error {
	for i := 0; i < len(is); i++ {
		st := is[i].st
		en := is[i].en
		if en <= st {
			return fmt.Errorf("bad range elem %d:%d, en<=st", st, en)
		}
		if i == 0 {
			continue
		}
		// check for badly ordered starts
		pst := is[i-1].st
		pen := is[i-1].en
		if pst >= st {
			return fmt.Errorf("range start not ordered %d:%d less than prev %d:%d", st, en,
				pst, pen)
		}
		// check end of last range against start of this range
		if pen > st {
			return fmt.Errorf("bad range elem %d:%d overlaps prev %d:%d", st, en,
				pst, pen)
		}
	}
	return nil
}

func (is *Intervals) String() string {
	var sb strings.Builder
	for i := range *is {
		if i != 0 {
			sb.WriteString(" ")
		}
		sb.WriteString((*is)[i].String())
	}
	return sb.String()
}

// intWithIdx holds an interval i and an index pairIndex storing i's
// position (either 0 or 1) within some previously specified interval
// pair <I1,I2>; a pairIndex of -1 is used to signal "end of
// iteration". Used for Intervals operations, not expected to be
// exported.
type intWithIdx struct {
	i         Interval
	pairIndex int
}

func (iwi intWithIdx) done() bool {
	return iwi.pairIndex == -1
}

// pairVisitor provides a way to visit (iterate through) each interval
// within a pair of Intervals in order of increasing start time. Expected
// usage model:
//
//	func example(i1, i2 Intervals) {
//	  var pairVisitor pv
//	  cur := pv.init(i1, i2);
//	  for !cur.done() {
//	     fmt.Printf("interval %s from i%d", cur.i.String(), cur.pairIndex+1)
//	     cur = pv.nxt()
//	  }
//	}
//
// Used internally for Intervals operations, not expected to be exported.
type pairVisitor struct {
	cur    intWithIdx
	i1pos  int
	i2pos  int
	i1, i2 Intervals
}

// init initializes a pairVisitor for the specified pair of intervals
// i1 and i2 and returns an intWithIdx object that points to the first
// interval by start position within i1/i2.
func (pv *pairVisitor) init(i1, i2 Intervals) intWithIdx {
	pv.i1, pv.i2 = i1, i2
	pv.cur = pv.sel()
	return pv.cur
}

// nxt advances the pairVisitor to the next interval by starting
// position within the pair, returning an intWithIdx that describes
// the interval.
func (pv *pairVisitor) nxt() intWithIdx {
	if pv.cur.pairIndex == 0 {
		pv.i1pos++
	} else {
		pv.i2pos++
	}
	pv.cur = pv.sel()
	return pv.cur
}

// sel is a helper function used by 'init' and 'nxt' above; it selects
// the earlier of the two intervals at the current positions within i1
// and i2, or a degenerate (pairIndex -1) intWithIdx if we have no
// more intervals to visit.
func (pv *pairVisitor) sel() intWithIdx {
	var c1, c2 intWithIdx
	if pv.i1pos >= len(pv.i1) {
		c1.pairIndex = -1
	} else {
		c1 = intWithIdx{i: pv.i1[pv.i1pos], pairIndex: 0}
	}
	if pv.i2pos >= len(pv.i2) {
		c2.pairIndex = -1
	} else {
		c2 = intWithIdx{i: pv.i2[pv.i2pos], pairIndex: 1}
	}
	if c1.pairIndex == -1 {
		return c2
	}
	if c2.pairIndex == -1 {
		return c1
	}
	if c1.i.st <= c2.i.st {
		return c1
	}
	return c2
}

// Overlaps returns whether any of the component ranges in is overlaps
// with some range in is2.
func (is Intervals) Overlaps(is2 Intervals) bool {
	// check for empty intervals
	if len(is) == 0 || len(is2) == 0 {
		return false
	}
	li := len(is)
	li2 := len(is2)
	// check for completely disjoint ranges
	if is[li-1].en <= is2[0].st ||
		is[0].st >= is2[li2-1].en {
		return false
	}
	// walk the combined sets of intervals and check for piecewise
	// overlap.
	var pv pairVisitor
	first := pv.init(is, is2)
	for {
		second := pv.nxt()
		if second.done() {
			break
		}
		if first.pairIndex == second.pairIndex {
			first = second
			continue
		}
		if first.i.Overlaps(second.i) {
			return true
		}
		first = second
	}
	return false
}

// Merge combines the intervals from "is" and "is2" and returns
// a new Intervals object containing all combined ranges from the
// two inputs.
func (is Intervals) Merge(is2 Intervals) Intervals {
	if len(is) == 0 {
		return is2
	} else if len(is2) == 0 {
		return is
	}
	// walk the combined set of intervals and merge them together.
	var ret Intervals
	var pv pairVisitor
	cur := pv.init(is, is2)
	for {
		second := pv.nxt()
		if second.done() {
			break
		}

		// Check for overlap between cur and second. If no overlap
		// then add cur to result and move on.
		if !cur.i.Overlaps(second.i) && !cur.i.adjacent(second.i) {
			ret = append(ret, cur.i)
			cur = second
			continue
		}
		// cur overlaps with second; merge second into cur
		cur.i.MergeInto(second.i)
	}
	ret = append(ret, cur.i)
	return ret
}

```

// === FILE: references/go/src/cmd/compile/internal/liveness/mergelocals.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package liveness

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/bitvec"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/ssa"
	"cmd/internal/src"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// MergeLocalsState encapsulates information about which AUTO
// (stack-allocated) variables within a function can be safely
// merged/overlapped, e.g. share a stack slot with some other auto).
// An instance of MergeLocalsState is produced by MergeLocals() below
// and then consumed in ssagen.AllocFrame. The map 'partition'
// contains entries of the form <N,SL> where N is an *ir.Name and SL
// is a slice holding the indices (within 'vars') of other variables
// that share the same slot, specifically the slot of the first
// element in the partition, which we'll call the "leader". For
// example, if a function contains five variables where v1/v2/v3 are
// safe to overlap and v4/v5 are safe to overlap, the MergeLocalsState
// content might look like
//
//	vars: [v1, v2, v3, v4, v5]
//	partition: v1 -> [1, 0, 2], v2 -> [1, 0, 2], v3 -> [1, 0, 2]
//	           v4 -> [3, 4], v5 -> [3, 4]
//
// A nil MergeLocalsState indicates that no local variables meet the
// necessary criteria for overlap.
type MergeLocalsState struct {
	// contains auto vars that participate in overlapping
	vars []*ir.Name
	// maps auto variable to overlap partition
	partition map[*ir.Name][]int
}

// candRegion is a sub-range (start, end) corresponding to an interval
// [st,en] within the list of candidate variables.
type candRegion struct {
	st, en int
}

// cstate holds state information we'll need during the analysis
// phase of stack slot merging but can be discarded when the analysis
// is done.
type cstate struct {
	fn             *ir.Func
	f              *ssa.Func
	lv             *Liveness
	cands          []*ir.Name
	nameToSlot     map[*ir.Name]int32
	regions        []candRegion
	indirectUE     map[ssa.ID][]*ir.Name
	ivs            []Intervals
	hashDeselected map[*ir.Name]bool
	trace          int // debug trace level
}

// MergeLocals analyzes the specified ssa function f to determine which
// of its auto variables can safely share the same stack slot, returning
// a state object that describes how the overlap should be done.
func MergeLocals(fn *ir.Func, f *ssa.Func) *MergeLocalsState {

	// Create a container object for useful state info and then
	// call collectMergeCandidates to see if there are vars suitable
	// for stack slot merging.
	cs := &cstate{
		fn:    fn,
		f:     f,
		trace: base.Debug.MergeLocalsTrace,
	}
	cs.collectMergeCandidates()
	if len(cs.regions) == 0 {
		return nil
	}

	// Kick off liveness analysis.
	//
	// If we have a local variable such as "r2" below that's written
	// but then not read, something like:
	//
	//      vardef r1
	//      r1.x = ...
	//      vardef r2
	//      r2.x = 0
	//      r2.y = ...
	//      <call foo>
	//      // no subsequent use of r2
	//      ... = r1.x
	//
	// then for the purpose of calculating stack maps at the call, we
	// can ignore "r2" completely during liveness analysis for stack
	// maps, however for stack slock merging we most definitely want
	// to treat the writes as "uses".
	cs.lv = newliveness(fn, f, cs.cands, cs.nameToSlot, 0)
	cs.lv.conservativeWrites = true
	cs.lv.prologue()
	cs.lv.solve()

	// Compute intervals for each candidate based on the liveness and
	// on block effects.
	cs.computeIntervals()

	// Perform merging within each region of the candidates list.
	rv := cs.performMerging()
	if err := rv.check(); err != nil {
		base.FatalfAt(fn.Pos(), "invalid mergelocals state: %v", err)
	}
	return rv
}

// Subsumed returns whether variable n is subsumed, e.g. appears
// in an overlap position but is not the leader in that partition.
func (mls *MergeLocalsState) Subsumed(n *ir.Name) bool {
	if sl, ok := mls.partition[n]; ok && mls.vars[sl[0]] != n {
		return true
	}
	return false
}

// IsLeader returns whether a variable n is the leader (first element)
// in a sharing partition.
func (mls *MergeLocalsState) IsLeader(n *ir.Name) bool {
	if sl, ok := mls.partition[n]; ok && mls.vars[sl[0]] == n {
		return true
	}
	return false
}

// Leader returns the leader variable for subsumed var n.
func (mls *MergeLocalsState) Leader(n *ir.Name) *ir.Name {
	if sl, ok := mls.partition[n]; ok {
		if mls.vars[sl[0]] == n {
			panic("variable is not subsumed")
		}
		return mls.vars[sl[0]]
	}
	panic("not a merge candidate")
}

// Followers writes a list of the followers for leader n into the slice tmp.
func (mls *MergeLocalsState) Followers(n *ir.Name, tmp []*ir.Name) []*ir.Name {
	tmp = tmp[:0]
	sl, ok := mls.partition[n]
	if !ok {
		panic("no entry for leader")
	}
	if mls.vars[sl[0]] != n {
		panic("followers invoked on subsumed var")
	}
	for _, k := range sl[1:] {
		tmp = append(tmp, mls.vars[k])
	}
	slices.SortStableFunc(tmp, func(a, b *ir.Name) int {
		return strings.Compare(a.Sym().Name, b.Sym().Name)
	})
	return tmp
}

// EstSavings returns the estimated reduction in stack size (number of bytes) for
// the given merge locals state via a pair of ints, the first for non-pointer types and the second for pointer types.
func (mls *MergeLocalsState) EstSavings() (int, int) {
	totnp := 0
	totp := 0
	for n := range mls.partition {
		if mls.Subsumed(n) {
			sz := int(n.Type().Size())
			if n.Type().HasPointers() {
				totp += sz
			} else {
				totnp += sz
			}
		}
	}
	return totnp, totp
}

// check tests for various inconsistencies and problems in mls,
// returning an error if any problems are found.
func (mls *MergeLocalsState) check() error {
	if mls == nil {
		return nil
	}
	used := make(map[int]bool)
	seenv := make(map[*ir.Name]int)
	for ii, v := range mls.vars {
		if prev, ok := seenv[v]; ok {
			return fmt.Errorf("duplicate var %q in vslots: %d and %d\n",
				v.Sym().Name, ii, prev)
		}
		seenv[v] = ii
	}
	for k, sl := range mls.partition {
		// length of slice value needs to be more than 1
		if len(sl) < 2 {
			return fmt.Errorf("k=%q v=%+v slice len %d invalid",
				k.Sym().Name, sl, len(sl))
		}
		// values in the slice need to be var indices
		for i, v := range sl {
			if v < 0 || v > len(mls.vars)-1 {
				return fmt.Errorf("k=%q v=+%v slpos %d vslot %d out of range of m.v", k.Sym().Name, sl, i, v)
			}
		}
	}
	for k, sl := range mls.partition {
		foundk := false
		for i, v := range sl {
			vv := mls.vars[v]
			if i == 0 {
				if !mls.IsLeader(vv) {
					return fmt.Errorf("k=%s v=+%v slpos 0 vslot %d IsLeader(%q) is false should be true", k.Sym().Name, sl, v, vv.Sym().Name)
				}
			} else {
				if !mls.Subsumed(vv) {
					return fmt.Errorf("k=%s v=+%v slpos %d vslot %d Subsumed(%q) is false should be true", k.Sym().Name, sl, i, v, vv.Sym().Name)
				}
				if mls.Leader(vv) != mls.vars[sl[0]] {
					return fmt.Errorf("k=%s v=+%v slpos %d vslot %d Leader(%q) got %v want %v", k.Sym().Name, sl, i, v, vv.Sym().Name, mls.Leader(vv), mls.vars[sl[0]])
				}
			}
			if vv == k {
				foundk = true
				if used[v] {
					return fmt.Errorf("k=%s v=+%v val slice used violation at slpos %d vslot %d", k.Sym().Name, sl, i, v)
				}
				used[v] = true
			}
		}
		if !foundk {
			return fmt.Errorf("k=%s v=+%v slice value missing k", k.Sym().Name, sl)
		}
		vl := mls.vars[sl[0]]
		for _, v := range sl[1:] {
			vv := mls.vars[v]
			if vv.Type().Size() > vl.Type().Size() {
				return fmt.Errorf("k=%s v=+%v follower %s size %d larger than leader %s size %d", k.Sym().Name, sl, vv.Sym().Name, vv.Type().Size(), vl.Sym().Name, vl.Type().Size())
			}
			if vv.Type().HasPointers() && !vl.Type().HasPointers() {
				return fmt.Errorf("k=%s v=+%v follower %s hasptr=true but leader %s hasptr=false", k.Sym().Name, sl, vv.Sym().Name, vl.Sym().Name)
			}
			if vv.Type().Alignment() > vl.Type().Alignment() {
				return fmt.Errorf("k=%s v=+%v follower %s align %d greater than leader %s align %d", k.Sym().Name, sl, vv.Sym().Name, vv.Type().Alignment(), vl.Sym().Name, vl.Type().Alignment())
			}
		}
	}
	for i := range used {
		if !used[i] {
			return fmt.Errorf("pos %d var %q unused", i, mls.vars[i])
		}
	}
	return nil
}

func (mls *MergeLocalsState) String() string {
	var leaders []*ir.Name
	for n, sl := range mls.partition {
		if n == mls.vars[sl[0]] {
			leaders = append(leaders, n)
		}
	}
	slices.SortFunc(leaders, func(a, b *ir.Name) int {
		return strings.Compare(a.Sym().Name, b.Sym().Name)
	})
	var sb strings.Builder
	for _, n := range leaders {
		sb.WriteString(n.Sym().Name + ":")
		sl := mls.partition[n]
		for _, k := range sl[1:] {
			n := mls.vars[k]
			sb.WriteString(" " + n.Sym().Name)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// collectMergeCandidates visits all of the AUTO vars declared in
// function fn and identifies a list of candidate variables for
// merging / overlapping. On return the "cands" field of cs will be
// filled in with our set of potentially overlappable candidate
// variables, the "regions" field will hold regions/sequence of
// compatible vars within the candidates list, "nameToSlot" field will
// be populated, and the "indirectUE" field will be filled in with
// information about indirect upwards-exposed uses in the func.
func (cs *cstate) collectMergeCandidates() {
	var cands []*ir.Name

	// Collect up the available set of appropriate AUTOs in the
	// function as a first step, and bail if we have fewer than
	// two candidates.
	for _, n := range cs.fn.Dcl {
		if !n.Used() {
			continue
		}
		if !ssa.IsMergeCandidate(n) {
			continue
		}
		cands = append(cands, n)
	}
	if len(cands) < 2 {
		return
	}

	// Sort by pointerness, size, and then name.
	sort.SliceStable(cands, func(i, j int) bool {
		return nameLess(cands[i], cands[j])
	})

	if cs.trace > 1 {
		fmt.Fprintf(os.Stderr, "=-= raw cand list for func %v:\n", cs.fn)
		for i := range cands {
			dumpCand(cands[i], i)
		}
	}

	// Now generate an initial pruned candidate list and regions list.
	// This may be empty if we don't have enough compatible candidates.
	initial, _ := cs.genRegions(cands)
	if len(initial) < 2 {
		return
	}

	// Set up for hash bisection if enabled.
	cs.setupHashBisection(initial)

	// Create and populate an indirect use table that we'll use
	// during interval construction. As part of this process we may
	// wind up tossing out additional candidates, so check to make
	// sure we still have something to work with.
	cs.cands, cs.regions = cs.populateIndirectUseTable(initial)
	if len(cs.cands) < 2 {
		return
	}

	// At this point we have a final pruned set of candidates and a
	// corresponding set of regions for the candidates. Build a
	// name-to-slot map for the candidates.
	cs.nameToSlot = make(map[*ir.Name]int32)
	for i, n := range cs.cands {
		cs.nameToSlot[n] = int32(i)
	}

	if cs.trace > 1 {
		fmt.Fprintf(os.Stderr, "=-= pruned candidate list for fn %v:\n", cs.fn)
		for i := range cs.cands {
			dumpCand(cs.cands[i], i)
		}
	}
}

// genRegions generates a set of regions within cands corresponding
// to potentially overlappable/mergeable variables.
func (cs *cstate) genRegions(cands []*ir.Name) ([]*ir.Name, []candRegion) {
	var pruned []*ir.Name
	var regions []candRegion
	st := 0
	for {
		en := nextRegion(cands, st)
		if en == -1 {
			break
		}
		if st == en {
			// region has just one element, we can skip it
			st++
			continue
		}
		pst := len(pruned)
		pen := pst + (en - st)
		if cs.trace > 1 {
			fmt.Fprintf(os.Stderr, "=-= addregion st=%d en=%d: add part %d -> %d\n", st, en, pst, pen)
		}

		// non-empty region, add to pruned
		pruned = append(pruned, cands[st:en+1]...)
		regions = append(regions, candRegion{st: pst, en: pen})
		st = en + 1
	}
	if len(pruned) < 2 {
		return nil, nil
	}
	return pruned, regions
}

func (cs *cstate) dumpFunc() {
	fmt.Fprintf(os.Stderr, "=-= mergelocalsdumpfunc %v:\n", cs.fn)
	ii := 0
	for k, b := range cs.f.Blocks {
		fmt.Fprintf(os.Stderr, "b%d:\n", k)
		for _, v := range b.Values {
			pos := base.Ctxt.PosTable.Pos(v.Pos)
			fmt.Fprintf(os.Stderr, "=-= %d L%d|C%d %s\n", ii, pos.RelLine(), pos.RelCol(), v.LongString())
			ii++
		}
	}
}

func (cs *cstate) dumpFuncIfSelected() {
	if base.Debug.MergeLocalsDumpFunc == "" {
		return
	}
	if !strings.HasSuffix(fmt.Sprintf("%v", cs.fn),
		base.Debug.MergeLocalsDumpFunc) {
		return
	}
	cs.dumpFunc()
}

// setupHashBisection checks to see if any of the candidate
// variables have been de-selected by our hash debug. Here
// we also implement the -d=mergelocalshtrace flag, which turns
// on debug tracing only if we have at least two candidates
// selected by the hash debug for this function.
func (cs *cstate) setupHashBisection(cands []*ir.Name) {
	if base.Debug.MergeLocalsHash == "" {
		return
	}
	deselected := make(map[*ir.Name]bool)
	selCount := 0
	for _, cand := range cands {
		if !base.MergeLocalsHash.MatchPosWithInfo(cand.Pos(), "mergelocals", nil) {
			deselected[cand] = true
		} else {
			deselected[cand] = false
			selCount++
		}
	}
	if selCount < len(cands) {
		cs.hashDeselected = deselected
	}
	if base.Debug.MergeLocalsHTrace != 0 && selCount >= 2 {
		cs.trace = base.Debug.MergeLocalsHTrace
	}
}

// populateIndirectUseTable creates and populates the "indirectUE" table
// within cs by doing some additional analysis of how the vars in
// cands are accessed in the function.
//
// It is possible to have situations where a given ir.Name is
// non-address-taken at the source level, but whose address is
// materialized in order to accommodate the needs of
// architecture-dependent operations or one sort or another (examples
// include things like LoweredZero/DuffZero, etc). The issue here is
// that the SymAddr op will show up as touching a variable of
// interest, but the subsequent memory op will not. This is generally
// not an issue for computing whether something is live across a call,
// but it is problematic for collecting the more fine-grained live
// interval info that drives stack slot merging.
//
// To handle this problem, make a forward pass over each basic block
// looking for instructions of the form vK := SymAddr(N) where N is a
// raw candidate. Create an entry in a map at that point from vK to
// its use count. Continue the walk, looking for uses of vK: when we
// see one, record it in a side table as an upwards exposed use of N.
// Each time we see a use, decrement the use count in the map, and if
// we hit zero, remove the map entry. If we hit the end of the basic
// block and we still have map entries, then evict the name in
// question from the candidate set.
func (cs *cstate) populateIndirectUseTable(cands []*ir.Name) ([]*ir.Name, []candRegion) {

	// main indirect UE table, this is what we're producing in this func
	indirectUE := make(map[ssa.ID][]*ir.Name)

	// this map holds the current set of candidates; the set may
	// shrink if we have to evict any candidates.
	rawcands := make(map[*ir.Name]struct{})

	// maps ssa value V to the ir.Name it is taking the addr of,
	// plus a count of the uses we've seen of V during a block walk.
	pendingUses := make(map[ssa.ID]nameCount)

	// A temporary indirect UE tab just for the current block
	// being processed; used to help with evictions.
	blockIndirectUE := make(map[ssa.ID][]*ir.Name)

	// temporary map used to record evictions in a given block.
	evicted := make(map[*ir.Name]bool)
	for _, n := range cands {
		rawcands[n] = struct{}{}
	}
	for k := 0; k < len(cs.f.Blocks); k++ {
		clear(pendingUses)
		clear(blockIndirectUE)
		b := cs.f.Blocks[k]
		for _, v := range b.Values {
			if n, e := affectedVar(v); n != nil {
				if _, ok := rawcands[n]; ok {
					if e&ssa.SymAddr != 0 && v.Uses != 0 {
						// we're taking the address of candidate var n
						if _, ok := pendingUses[v.ID]; ok {
							// should never happen
							base.FatalfAt(v.Pos, "internal error: apparent multiple defs for SSA value %d", v.ID)
						}
						// Stash an entry in pendingUses recording
						// that we took the address of "n" via this
						// val.
						pendingUses[v.ID] = nameCount{n: n, count: v.Uses}
						if cs.trace > 2 {
							fmt.Fprintf(os.Stderr, "=-= SymAddr(%s) on %s\n",
								n.Sym().Name, v.LongString())
						}
					}
				}
			}
			for _, arg := range v.Args {
				if nc, ok := pendingUses[arg.ID]; ok {
					// We found a use of some value that took the
					// address of nc.n. Record this inst as a
					// potential indirect use.
					if cs.trace > 2 {
						fmt.Fprintf(os.Stderr, "=-= add indirectUE(%s) count=%d on %s\n", nc.n.Sym().Name, nc.count, v.LongString())
					}
					blockIndirectUE[v.ID] = append(blockIndirectUE[v.ID], nc.n)
					nc.count--
					if nc.count == 0 {
						// That was the last use of the value. Clean
						// up the entry in pendingUses.
						if cs.trace > 2 {
							fmt.Fprintf(os.Stderr, "=-= last use of v%d\n",
								arg.ID)
						}
						delete(pendingUses, arg.ID)
					} else {
						// Not the last use; record the decremented
						// use count and move on.
						pendingUses[arg.ID] = nc
					}
				}
			}
		}

		// We've reached the end of this basic block: if we have any
		// leftover entries in pendingUses, then evict the
		// corresponding names from the candidate set. The idea here
		// is that if we materialized the address of some local and
		// that value is flowing out of the block off somewhere else,
		// we're going to treat that local as truly address-taken and
		// not have it be a merge candidate.
		clear(evicted)
		if len(pendingUses) != 0 {
			for id, nc := range pendingUses {
				if cs.trace > 2 {
					fmt.Fprintf(os.Stderr, "=-= evicting %q due to pendingUse %d count %d\n", nc.n.Sym().Name, id, nc.count)
				}
				delete(rawcands, nc.n)
				evicted[nc.n] = true
			}
		}
		// Copy entries from blockIndirectUE into final indirectUE. Skip
		// anything that we evicted in the loop above.
		for id, sl := range blockIndirectUE {
			for _, n := range sl {
				if evicted[n] {
					continue
				}
				indirectUE[id] = append(indirectUE[id], n)
				if cs.trace > 2 {
					fmt.Fprintf(os.Stderr, "=-= add final indUE v%d name %s\n", id, n.Sym().Name)
				}
			}
		}
	}
	if len(rawcands) < 2 {
		return nil, nil
	}
	cs.indirectUE = indirectUE
	if cs.trace > 2 {
		fmt.Fprintf(os.Stderr, "=-= iuetab:\n")
		ids := make([]ssa.ID, 0, len(indirectUE))
		for k := range indirectUE {
			ids = append(ids, k)
		}
		slices.Sort(ids)
		for _, id := range ids {
			fmt.Fprintf(os.Stderr, "  v%d:", id)
			for _, n := range indirectUE[id] {
				fmt.Fprintf(os.Stderr, " %s", n.Sym().Name)
			}
			fmt.Fprintf(os.Stderr, "\n")
		}
	}

	pruned := cands[:0]
	for k := range rawcands {
		pruned = append(pruned, k)
	}
	sort.Slice(pruned, func(i, j int) bool {
		return nameLess(pruned[i], pruned[j])
	})
	var regions []candRegion
	pruned, regions = cs.genRegions(pruned)
	if len(pruned) < 2 {
		return nil, nil
	}
	return pruned, regions
}

type nameCount struct {
	n     *ir.Name
	count int32
}

// nameLess compares ci with cj to see if ci should be less than cj in
// a relative ordering of candidate variables. This is used to sort
// vars by pointerness (variables with pointers first), then in order
// of decreasing alignment, then by decreasing size. We are assuming a
// merging algorithm that merges later entries in the list into
// earlier entries. An example ordered candidate list produced by
// nameLess:
//
//	idx   name    type       align    size
//	0:    abc     [10]*int   8        80
//	1:    xyz     [9]*int    8        72
//	2:    qrs     [2]*int    8        16
//	3:    tuv     [9]int     8        72
//	4:    wxy     [9]int32   4        36
//	5:    jkl     [8]int32   4        32
func nameLess(ci, cj *ir.Name) bool {
	if ci.Type().HasPointers() != cj.Type().HasPointers() {
		return ci.Type().HasPointers()
	}
	if ci.Type().Alignment() != cj.Type().Alignment() {
		return cj.Type().Alignment() < ci.Type().Alignment()
	}
	if ci.Type().Size() != cj.Type().Size() {
		return cj.Type().Size() < ci.Type().Size()
	}
	if ci.Sym().Name != cj.Sym().Name {
		return ci.Sym().Name < cj.Sym().Name
	}
	return fmt.Sprintf("%v", ci.Pos()) < fmt.Sprintf("%v", cj.Pos())
}

// nextRegion starts at location idx and walks forward in the cands
// slice looking for variables that are "compatible" (potentially
// overlappable, in the sense that they could potentially share the
// stack slot of cands[idx]); it returns the end of the new region
// (range of compatible variables starting at idx).
func nextRegion(cands []*ir.Name, idx int) int {
	n := len(cands)
	if idx >= n {
		return -1
	}
	c0 := cands[idx]
	szprev := c0.Type().Size()
	alnprev := c0.Type().Alignment()
	for j := idx + 1; j < n; j++ {
		cj := cands[j]
		szj := cj.Type().Size()
		if szj > szprev {
			return j - 1
		}
		alnj := cj.Type().Alignment()
		if alnj > alnprev {
			return j - 1
		}
		szprev = szj
		alnprev = alnj
	}
	return n - 1
}

// mergeVisitRegion tries to perform overlapping of variables with a
// given subrange of cands described by st and en (indices into our
// candidate var list), where the variables within this range have
// already been determined to be compatible with respect to type,
// size, etc. Overlapping is done in a greedy fashion: we select the
// first element in the st->en range, then walk the rest of the
// elements adding in vars whose lifetimes don't overlap with the
// first element, then repeat the process until we run out of work.
// Ordering of the candidates within the region [st,en] is important;
// within the list the assumption is that if we overlap two variables
// X and Y where X precedes Y in the list, we need to make X the
// "leader" (keep X's slot and set Y's frame offset to X's) as opposed
// to the other way around, since it's possible that Y is smaller in
// size than X.
func (cs *cstate) mergeVisitRegion(mls *MergeLocalsState, st, en int) {
	if cs.trace > 1 {
		fmt.Fprintf(os.Stderr, "=-= mergeVisitRegion(st=%d, en=%d)\n", st, en)
	}
	n := en - st + 1
	used := bitvec.New(int32(n))

	nxt := func(slot int) int {
		for c := slot - st; c < n; c++ {
			if used.Get(int32(c)) {
				continue
			}
			return c + st
		}
		return -1
	}

	navail := n
	cands := cs.cands
	ivs := cs.ivs
	if cs.trace > 1 {
		fmt.Fprintf(os.Stderr, "  =-= navail = %d\n", navail)
	}
	for navail >= 2 {
		leader := nxt(st)
		used.Set(int32(leader - st))
		navail--

		if cs.trace > 1 {
			fmt.Fprintf(os.Stderr, "  =-= begin leader %d used=%s\n", leader,
				used.String())
		}
		elems := []int{leader}
		lints := ivs[leader]

		for succ := nxt(leader + 1); succ != -1; succ = nxt(succ + 1) {

			// Skip if de-selected by merge locals hash.
			if cs.hashDeselected != nil && cs.hashDeselected[cands[succ]] {
				continue
			}
			// Skip if already used.
			if used.Get(int32(succ - st)) {
				continue
			}
			if cs.trace > 1 {
				fmt.Fprintf(os.Stderr, "  =-= overlap of %d[%v] {%s} with %d[%v] {%s} is: %v\n", leader, cands[leader], lints.String(), succ, cands[succ], ivs[succ].String(), lints.Overlaps(ivs[succ]))
			}

			// Can we overlap leader with this var?
			if lints.Overlaps(ivs[succ]) {
				continue
			} else {
				// Add to overlap set.
				elems = append(elems, succ)
				lints = lints.Merge(ivs[succ])
			}
		}
		if len(elems) > 1 {
			// We found some things to overlap with leader. Add the
			// candidate elements to "vars" and update "partition".
			off := len(mls.vars)
			sl := make([]int, len(elems))
			for i, candslot := range elems {
				sl[i] = off + i
				mls.vars = append(mls.vars, cands[candslot])
				mls.partition[cands[candslot]] = sl
			}
			navail -= (len(elems) - 1)
			for i := range elems {
				used.Set(int32(elems[i] - st))
			}
			if cs.trace > 1 {
				fmt.Fprintf(os.Stderr, "=-= overlapping %+v:\n", sl)
				for i := range sl {
					dumpCand(mls.vars[sl[i]], sl[i])
				}
				for i, v := range elems {
					fmt.Fprintf(os.Stderr, "=-= %d: sl=%d %s\n", i, v, ivs[v])
				}
			}
		}
	}
}

// performMerging carries out variable merging within each of the
// candidate ranges in regions, returning a state object
// that describes the variable overlaps.
func (cs *cstate) performMerging() *MergeLocalsState {
	cands := cs.cands

	mls := &MergeLocalsState{
		partition: make(map[*ir.Name][]int),
	}

	// Dump state before attempting overlap.
	if cs.trace > 1 {
		fmt.Fprintf(os.Stderr, "=-= cands live before overlap:\n")
		for i := range cands {
			c := cands[i]
			fmt.Fprintf(os.Stderr, "%d: %v sz=%d ivs=%s\n",
				i, c.Sym().Name, c.Type().Size(), cs.ivs[i].String())
		}
		fmt.Fprintf(os.Stderr, "=-= regions (%d): ", len(cs.regions))
		for _, cr := range cs.regions {
			fmt.Fprintf(os.Stderr, " [%d,%d]", cr.st, cr.en)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Apply a greedy merge/overlap strategy within each region
	// of compatible variables.
	for _, cr := range cs.regions {
		cs.mergeVisitRegion(mls, cr.st, cr.en)
	}
	if len(mls.vars) == 0 {
		return nil
	}
	return mls
}

// computeIntervals performs a backwards sweep over the instructions
// of the function we're compiling, building up an Intervals object
// for each candidate variable by looking for upwards exposed uses
// and kills.
func (cs *cstate) computeIntervals() {
	lv := cs.lv
	ibuilders := make([]IntervalsBuilder, len(cs.cands))
	nvars := int32(len(lv.vars))
	liveout := bitvec.New(nvars)

	cs.dumpFuncIfSelected()

	// Count instructions.
	ninstr := 0
	for _, b := range lv.f.Blocks {
		ninstr += len(b.Values)
	}
	// current instruction index during backwards walk
	iidx := ninstr - 1

	// Make a backwards pass over all blocks
	for k := len(lv.f.Blocks) - 1; k >= 0; k-- {
		b := lv.f.Blocks[k]
		be := lv.blockEffects(b)

		if cs.trace > 2 {
			fmt.Fprintf(os.Stderr, "=-= liveout from tail of b%d: ", k)
			for j := range lv.vars {
				if be.liveout.Get(int32(j)) {
					fmt.Fprintf(os.Stderr, " %q", lv.vars[j].Sym().Name)
				}
			}
			fmt.Fprintf(os.Stderr, "\n")
		}

		// Take into account effects taking place at end of this basic
		// block by comparing our current live set with liveout for
		// the block. If a given var was not live before and is now
		// becoming live we need to mark this transition with a
		// builder "Live" call; similarly if a var was live before and
		// is now no longer live, we need a "Kill" call.
		for j := range lv.vars {
			isLive := liveout.Get(int32(j))
			blockLiveOut := be.liveout.Get(int32(j))
			if isLive {
				if !blockLiveOut {
					if cs.trace > 2 {
						fmt.Fprintf(os.Stderr, "=+= at instr %d block boundary kill of %v\n", iidx, lv.vars[j])
					}
					ibuilders[j].Kill(iidx)
				}
			} else if blockLiveOut {
				if cs.trace > 2 {
					fmt.Fprintf(os.Stderr, "=+= at block-end instr %d %v becomes live\n",
						iidx, lv.vars[j])
				}
				ibuilders[j].Live(iidx)
			}
		}

		// Set our working "currently live" set to the previously
		// computed live out set for the block.
		liveout.Copy(be.liveout)

		// Now walk backwards through this block.
		for i := len(b.Values) - 1; i >= 0; i-- {
			v := b.Values[i]

			if cs.trace > 2 {
				fmt.Fprintf(os.Stderr, "=-= b%d instr %d: %s\n", k, iidx, v.LongString())
			}

			// Update liveness based on what we see happening in this
			// instruction.
			pos, e := lv.valueEffects(v)
			becomeslive := e&uevar != 0
			iskilled := e&varkill != 0
			if becomeslive && iskilled {
				// we do not ever expect to see both a kill and an
				// upwards exposed use given our size constraints.
				panic("should never happen")
			}
			if iskilled && liveout.Get(pos) {
				ibuilders[pos].Kill(iidx)
				liveout.Unset(pos)
				if cs.trace > 2 {
					fmt.Fprintf(os.Stderr, "=+= at instr %d kill of %v\n",
						iidx, lv.vars[pos])
				}
			} else if becomeslive && !liveout.Get(pos) {
				ibuilders[pos].Live(iidx)
				liveout.Set(pos)
				if cs.trace > 2 {
					fmt.Fprintf(os.Stderr, "=+= at instr %d upwards-exposed use of %v\n",
						iidx, lv.vars[pos])
				}
			}

			if cs.indirectUE != nil {
				// Now handle "indirect" upwards-exposed uses.
				ues := cs.indirectUE[v.ID]
				for _, n := range ues {
					if pos, ok := lv.idx[n]; ok {
						if !liveout.Get(pos) {
							ibuilders[pos].Live(iidx)
							liveout.Set(pos)
							if cs.trace > 2 {
								fmt.Fprintf(os.Stderr, "=+= at instr %d v%d indirect upwards-exposed use of %v\n", iidx, v.ID, lv.vars[pos])
							}
						}
					}
				}
			}
			iidx--
		}

		// This check disabled for now due to the way scheduling works
		// for ops that materialize values of local variables. For
		// many architecture we have rewrite rules of this form:
		//
		// (LocalAddr <t> {sym} base mem) && t.Elem().HasPointers() => (MOVDaddr {sym} (SPanchored base mem))
		// (LocalAddr <t> {sym} base _)  && !t.Elem().HasPointers() => (MOVDaddr {sym} base)
		//
		// which are designed to ensure that if you have a pointerful
		// variable "abc" sequence
		//
		//    v30 = VarDef <mem> {abc} v21
		//    v31 = LocalAddr <*SB> {abc} v2 v30
		//    v32 = Zero <mem> {SB} [2056] v31 v30
		//
		// this will be lowered into
		//
		//    v30 = VarDef <mem> {sb} v21
		//   v106 = SPanchored <uintptr> v2 v30
		//    v31 = MOVDaddr <*SB> {sb} v106
		//     v3 = DUFFZERO <mem> [2056] v31 v30
		//
		// Note the SPanchored: this ensures that the scheduler won't
		// move the MOVDaddr earlier than the vardef. With a variable
		// "xyz" that has no pointers, however, if we start with
		//
		//    v66 = VarDef <mem> {t2} v65
		//    v67 = LocalAddr <*T> {t2} v2 v66
		//    v68 = Zero <mem> {T} [2056] v67 v66
		//
		// we might lower to
		//
		//    v66 = VarDef <mem> {t2} v65
		//    v29 = MOVDaddr <*T> {t2} [2032] v2
		//    v43 = LoweredZero <mem> v67 v29 v66
		//    v68 = Zero [2056] v2 v43
		//
		// where that MOVDaddr can float around arbitrarily, meaning
		// that we may see an upwards-exposed use to it before the
		// VarDef.
		//
		// One avenue to restoring the check below would be to change
		// the rewrite rules to something like
		//
		// (LocalAddr <t> {sym} base mem) && (t.Elem().HasPointers() || isMergeCandidate(t) => (MOVDaddr {sym} (SPanchored base mem))
		//
		// however that change will have to be carefully evaluated,
		// since it would constrain the scheduler for _all_ LocalAddr
		// ops for potential merge candidates, even if we don't
		// actually succeed in any overlaps. This will be revisitged in
		// a later CL if possible.
		//
		const checkLiveOnEntry = false
		if checkLiveOnEntry && b == lv.f.Entry {
			for j, v := range lv.vars {
				if liveout.Get(int32(j)) {
					lv.f.Fatalf("%v %L recorded as live on entry",
						lv.fn.Nname, v)
				}
			}
		}
	}
	if iidx != -1 {
		panic("iidx underflow")
	}

	// Finish intervals construction.
	ivs := make([]Intervals, len(cs.cands))
	for i := range cs.cands {
		var err error
		ivs[i], err = ibuilders[i].Finish()
		if err != nil {
			cs.dumpFunc()
			base.FatalfAt(cs.cands[i].Pos(), "interval construct error for var %q in func %q (%d instrs): %v", cs.cands[i].Sym().Name, ir.FuncName(cs.fn), ninstr, err)
		}
	}
	cs.ivs = ivs
}

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

func dumpCand(c *ir.Name, i int) {
	fmt.Fprintf(os.Stderr, " %d: %s %q sz=%d hp=%v align=%d t=%v\n",
		i, fmtFullPos(c.Pos()), c.Sym().Name, c.Type().Size(),
		c.Type().HasPointers(), c.Type().Alignment(), c.Type())
}

// for unit testing only.
func MakeMergeLocalsState(partition map[*ir.Name][]int, vars []*ir.Name) (*MergeLocalsState, error) {
	mls := &MergeLocalsState{partition: partition, vars: vars}
	if err := mls.check(); err != nil {
		return nil, err
	}
	return mls, nil
}

```

// === FILE: references/go/src/cmd/compile/internal/liveness/plive.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Garbage collector liveness bitmap generation.

// The command line flag -live causes this code to print debug information.
// The levels are:
//
//	-live (aka -live=1): print liveness lists as code warnings at safe points
//	-live=2: print an assembly listing with liveness annotations
//
// Each level includes the earlier output as well.

package liveness

import (
	"cmp"
	"fmt"
	"math"
	"os"
	"slices"
	"sort"
	"strings"

	"cmd/compile/internal/abi"
	"cmd/compile/internal/base"
	"cmd/compile/internal/bitvec"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/objw"
	"cmd/compile/internal/reflectdata"
	"cmd/compile/internal/ssa"
	"cmd/compile/internal/typebits"
	"cmd/compile/internal/types"
	"cmd/internal/hash"
	"cmd/internal/obj"
	"cmd/internal/src"

	rtabi "internal/abi"
)

// OpVarDef is an annotation for the liveness analysis, marking a place
// where a complete initialization (definition) of a variable begins.
// Since the liveness analysis can see initialization of single-word
// variables quite easy, OpVarDef is only needed for multi-word
// variables satisfying isfat(n.Type). For simplicity though, buildssa
// emits OpVarDef regardless of variable width.
//
// An 'OpVarDef x' annotation in the instruction stream tells the liveness
// analysis to behave as though the variable x is being initialized at that
// point in the instruction stream. The OpVarDef must appear before the
// actual (multi-instruction) initialization, and it must also appear after
// any uses of the previous value, if any. For example, if compiling:
//
//	x = x[1:]
//
// it is important to generate code like:
//
//	base, len, cap = pieces of x[1:]
//	OpVarDef x
//	x = {base, len, cap}
//
// If instead the generated code looked like:
//
//	OpVarDef x
//	base, len, cap = pieces of x[1:]
//	x = {base, len, cap}
//
// then the liveness analysis would decide the previous value of x was
// unnecessary even though it is about to be used by the x[1:] computation.
// Similarly, if the generated code looked like:
//
//	base, len, cap = pieces of x[1:]
//	x = {base, len, cap}
//	OpVarDef x
//
// then the liveness analysis will not preserve the new value of x, because
// the OpVarDef appears to have "overwritten" it.
//
// OpVarDef is a bit of a kludge to work around the fact that the instruction
// stream is working on single-word values but the liveness analysis
// wants to work on individual variables, which might be multi-word
// aggregates. It might make sense at some point to look into letting
// the liveness analysis work on single-word values as well, although
// there are complications around interface values, slices, and strings,
// all of which cannot be treated as individual words.

// blockEffects summarizes the liveness effects on an SSA block.
type blockEffects struct {
	// Computed during Liveness.prologue using only the content of
	// individual blocks:
	//
	//	uevar: upward exposed variables (used before set in block)
	//	varkill: killed variables (set in block)
	uevar   bitvec.BitVec
	varkill bitvec.BitVec

	// Computed during Liveness.solve using control flow information:
	//
	//	livein: variables live at block entry
	//	liveout: variables live at block exit
	livein  bitvec.BitVec
	liveout bitvec.BitVec
}

// A collection of global state used by Liveness analysis.
type Liveness struct {
	fn         *ir.Func
	f          *ssa.Func
	vars       []*ir.Name
	idx        map[*ir.Name]int32
	stkptrsize int64

	be []blockEffects

	// allUnsafe indicates that all points in this function are
	// unsafe-points.
	allUnsafe bool
	// unsafePoints bit i is set if Value ID i is an unsafe-point
	// (preemption is not allowed). Only valid if !allUnsafe.
	unsafePoints bitvec.BitVec
	// unsafeBlocks bit i is set if Block ID i is an unsafe-point
	// (preemption is not allowed on any end-of-block
	// instructions). Only valid if !allUnsafe.
	unsafeBlocks bitvec.BitVec

	// An array with a bit vector for each safe point in the
	// current Block during liveness.epilogue. Indexed in Value
	// order for that block. Additionally, for the entry block
	// livevars[0] is the entry bitmap. liveness.compact moves
	// these to stackMaps.
	livevars []bitvec.BitVec

	// livenessMap maps from safe points (i.e., CALLs) to their
	// liveness map indexes.
	livenessMap Map
	stackMapSet bvecSet
	stackMaps   []bitvec.BitVec

	cache progeffectscache

	// partLiveArgs includes input arguments (PPARAM) that may
	// be partially live. That is, it is considered live because
	// a part of it is used, but we may not initialize all parts.
	partLiveArgs map[*ir.Name]bool

	doClobber     bool // Whether to clobber dead stack slots in this function.
	noClobberArgs bool // Do not clobber function arguments

	// treat "dead" writes as equivalent to reads during the analysis;
	// used only during liveness analysis for stack slot merging (doesn't
	// make sense for stackmap analysis).
	conservativeWrites bool
}

// Map maps from *ssa.Value to StackMapIndex.
// Also keeps track of unsafe ssa.Values and ssa.Blocks.
// (unsafe = can't be interrupted during GC.)
type Map struct {
	Vals         map[ssa.ID]objw.StackMapIndex
	UnsafeVals   map[ssa.ID]bool
	UnsafeBlocks map[ssa.ID]bool
	// The set of live, pointer-containing variables at the DeferReturn
	// call (only set when open-coded defers are used).
	DeferReturn objw.StackMapIndex
}

func (m *Map) reset() {
	if m.Vals == nil {
		m.Vals = make(map[ssa.ID]objw.StackMapIndex)
		m.UnsafeVals = make(map[ssa.ID]bool)
		m.UnsafeBlocks = make(map[ssa.ID]bool)
	} else {
		clear(m.Vals)
		clear(m.UnsafeVals)
		clear(m.UnsafeBlocks)
	}
	m.DeferReturn = objw.StackMapDontCare
}

func (m *Map) set(v *ssa.Value, i objw.StackMapIndex) {
	m.Vals[v.ID] = i
}
func (m *Map) setUnsafeVal(v *ssa.Value) {
	m.UnsafeVals[v.ID] = true
}
func (m *Map) setUnsafeBlock(b *ssa.Block) {
	m.UnsafeBlocks[b.ID] = true
}

func (m Map) Get(v *ssa.Value) objw.StackMapIndex {
	// If v isn't in the map, then it's a "don't care".
	if idx, ok := m.Vals[v.ID]; ok {
		return idx
	}
	return objw.StackMapDontCare
}
func (m Map) GetUnsafe(v *ssa.Value) bool {
	// default is safe
	return m.UnsafeVals[v.ID]
}
func (m Map) GetUnsafeBlock(b *ssa.Block) bool {
	// default is safe
	return m.UnsafeBlocks[b.ID]
}

type progeffectscache struct {
	retuevar    []int32
	tailuevar   []int32
	initialized bool
}

// shouldTrack reports whether the liveness analysis
// should track the variable n.
// We don't care about variables that have no pointers,
// nor do we care about non-local variables,
// nor do we care about empty structs (handled by the pointer check),
// nor do we care about the fake PAUTOHEAP variables.
func shouldTrack(n *ir.Name) bool {
	return (n.Class == ir.PAUTO && n.Esc() != ir.EscHeap || n.Class == ir.PPARAM || n.Class == ir.PPARAMOUT) && n.Type().HasPointers()
}

// getvariables returns the list of on-stack variables that we need to track
// and a map for looking up indices by *Node.
func getvariables(fn *ir.Func) ([]*ir.Name, map[*ir.Name]int32) {
	var vars []*ir.Name
	for _, n := range fn.Dcl {
		if shouldTrack(n) {
			vars = append(vars, n)
		}
	}
	idx := make(map[*ir.Name]int32, len(vars))
	for i, n := range vars {
		idx[n] = int32(i)
	}
	return vars, idx
}

func (lv *Liveness) initcache() {
	if lv.cache.initialized {
		base.Fatalf("liveness cache initialized twice")
		return
	}
	lv.cache.initialized = true

	for i, node := range lv.vars {
		switch node.Class {
		case ir.PPARAM:
			// A return instruction with a p.to is a tail return, which brings
			// the stack pointer back up (if it ever went down) and then jumps
			// to a new function entirely. That form of instruction must read
			// all the parameters for correctness, and similarly it must not
			// read the out arguments - they won't be set until the new
			// function runs.
			lv.cache.tailuevar = append(lv.cache.tailuevar, int32(i))

		case ir.PPARAMOUT:
			// All results are live at every return point.
			// Note that this point is after escaping return values
			// are copied back to the stack using their PAUTOHEAP references.
			lv.cache.retuevar = append(lv.cache.retuevar, int32(i))
		}
	}
}

// A liveEffect is a set of flags that describe an instruction's
// liveness effects on a variable.
//
// The possible flags are:
//
//	uevar - used by the instruction
//	varkill - killed by the instruction (set)
//
// A kill happens after the use (for an instruction that updates a value, for example).
type liveEffect int

const (
	uevar liveEffect = 1 << iota
	varkill
)

// valueEffects returns the index of a variable in lv.vars and the
// liveness effects v has on that variable.
// If v does not affect any tracked variables, it returns -1, 0.
func (lv *Liveness) valueEffects(v *ssa.Value) (int32, liveEffect) {
	n, e := affectedVar(v)
	if e == 0 || n == nil { // cheapest checks first
		return -1, 0
	}
	// AllocFrame has dropped unused variables from
	// lv.fn.Func.Dcl, but they might still be referenced by
	// OpVarFoo pseudo-ops. Ignore them to prevent "lost track of
	// variable" ICEs (issue 19632).
	switch v.Op {
	case ssa.OpVarDef, ssa.OpVarLive, ssa.OpKeepAlive:
		if !n.Used() {
			return -1, 0
		}
	}

	if n.Class == ir.PPARAM && !n.Addrtaken() && n.Type().Size() > int64(types.PtrSize) {
		// Only aggregate-typed arguments that are not address-taken can be
		// partially live.
		lv.partLiveArgs[n] = true
	}

	var effect liveEffect
	// Read is a read, obviously.
	//
	// Addr is a read also, as any subsequent holder of the pointer must be able
	// to see all the values (including initialization) written so far.
	// This also prevents a variable from "coming back from the dead" and presenting
	// stale pointers to the garbage collector. See issue 28445.
	if e&(ssa.SymRead|ssa.SymAddr) != 0 {
		effect |= uevar
	}
	if e&ssa.SymWrite != 0 {
		if !isfat(n.Type()) || v.Op == ssa.OpVarDef {
			effect |= varkill
		} else if lv.conservativeWrites {
			effect |= uevar
		}
	}

	if effect == 0 {
		return -1, 0
	}

	if pos, ok := lv.idx[n]; ok {
		return pos, effect
	}
	return -1, 0
}

// affectedVar returns the *ir.Name node affected by v.
func affectedVar(v *ssa.Value) (*ir.Name, ssa.SymEffect) {
	// Special cases.
	switch v.Op {
	case ssa.OpLoadReg:
		n, _ := ssa.AutoVar(v.Args[0])
		return n, ssa.SymRead
	case ssa.OpStoreReg:
		n, _ := ssa.AutoVar(v)
		return n, ssa.SymWrite

	case ssa.OpArgIntReg:
		// This forces the spill slot for the register to be live at function entry.
		// one of the following holds for a function F with pointer-valued register arg X:
		//  0. No GC (so an uninitialized spill slot is okay)
		//  1. GC at entry of F.  GC is precise, but the spills around morestack initialize X's spill slot
		//  2. Stack growth at entry of F.  Same as GC.
		//  3. GC occurs within F itself.  This has to be from preemption, and thus GC is conservative.
		//     a. X is in a register -- then X is seen, and the spill slot is also scanned conservatively.
		//     b. X is spilled -- the spill slot is initialized, and scanned conservatively
		//     c. X is not live -- the spill slot is scanned conservatively, and it may contain X from an earlier spill.
		//  4. GC within G, transitively called from F
		//    a. X is live at call site, therefore is spilled, to its spill slot (which is live because of subsequent LoadReg).
		//    b. X is not live at call site -- but neither is its spill slot.
		n, _ := ssa.AutoVar(v)
		return n, ssa.SymRead

	case ssa.OpVarLive:
		return v.Aux.(*ir.Name), ssa.SymRead
	case ssa.OpVarDef:
		return v.Aux.(*ir.Name), ssa.SymWrite
	case ssa.OpKeepAlive:
		n, _ := ssa.AutoVar(v.Args[0])
		return n, ssa.SymRead
	}

	e := v.Op.SymEffect()
	if e == 0 {
		return nil, 0
	}

	switch a := v.Aux.(type) {
	case nil, *obj.LSym:
		// ok, but no node
		return nil, e
	case *ir.Name:
		return a, e
	default:
		base.Fatalf("weird aux: %s", v.LongString())
		return nil, e
	}
}

type livenessFuncCache struct {
	be          []blockEffects
	livenessMap Map
}

// Constructs a new liveness structure used to hold the global state of the
// liveness computation. The cfg argument is a slice of *BasicBlocks and the
// vars argument is a slice of *Nodes.
func newliveness(fn *ir.Func, f *ssa.Func, vars []*ir.Name, idx map[*ir.Name]int32, stkptrsize int64) *Liveness {
	lv := &Liveness{
		fn:         fn,
		f:          f,
		vars:       vars,
		idx:        idx,
		stkptrsize: stkptrsize,
	}

	// Significant sources of allocation are kept in the ssa.Cache
	// and reused. Surprisingly, the bit vectors themselves aren't
	// a major source of allocation, but the liveness maps are.
	if lc, _ := f.Cache.Liveness.(*livenessFuncCache); lc == nil {
		// Prep the cache so liveness can fill it later.
		f.Cache.Liveness = new(livenessFuncCache)
	} else {
		if cap(lc.be) >= f.NumBlocks() {
			lv.be = lc.be[:f.NumBlocks()]
		}
		lv.livenessMap = Map{
			Vals:         lc.livenessMap.Vals,
			UnsafeVals:   lc.livenessMap.UnsafeVals,
			UnsafeBlocks: lc.livenessMap.UnsafeBlocks,
			DeferReturn:  objw.StackMapDontCare,
		}
		lc.livenessMap.Vals = nil
		lc.livenessMap.UnsafeVals = nil
		lc.livenessMap.UnsafeBlocks = nil
	}
	if lv.be == nil {
		lv.be = make([]blockEffects, f.NumBlocks())
	}

	nblocks := int32(len(f.Blocks))
	nvars := int32(len(vars))
	bulk := bitvec.NewBulk(nvars, nblocks*4)
	for _, b := range f.Blocks {
		be := lv.blockEffects(b)

		be.uevar = bulk.Next()
		be.varkill = bulk.Next()
		be.livein = bulk.Next()
		be.liveout = bulk.Next()
	}
	lv.livenessMap.reset()

	lv.markUnsafePoints()

	lv.partLiveArgs = make(map[*ir.Name]bool)

	lv.enableClobber()

	return lv
}

func (lv *Liveness) blockEffects(b *ssa.Block) *blockEffects {
	return &lv.be[b.ID]
}

// Generates live pointer value maps for arguments and local variables. The
// this argument and the in arguments are always assumed live. The vars
// argument is a slice of *Nodes.
func (lv *Liveness) pointerMap(liveout bitvec.BitVec, vars []*ir.Name, args, locals bitvec.BitVec) {
	var slotsSeen map[int64]*ir.Name
	checkForDuplicateSlots := base.Debug.MergeLocals != 0
	if checkForDuplicateSlots {
		slotsSeen = make(map[int64]*ir.Name)
	}
	for i := int32(0); ; i++ {
		i = liveout.Next(i)
		if i < 0 {
			break
		}
		node := vars[i]
		switch node.Class {
		case ir.PPARAM, ir.PPARAMOUT:
			if !node.IsOutputParamInRegisters() {
				if node.FrameOffset() < 0 {
					lv.f.Fatalf("Node %v has frameoffset %d\n", node.Sym().Name, node.FrameOffset())
				}
				typebits.SetNoCheck(node.Type(), node.FrameOffset(), args)
				break
			}
			fallthrough // PPARAMOUT in registers acts memory-allocates like an AUTO
		case ir.PAUTO:
			if checkForDuplicateSlots {
				if prev, ok := slotsSeen[node.FrameOffset()]; ok {
					base.FatalfAt(node.Pos(), "two vars live at pointerMap generation: %q and %q", prev.Sym().Name, node.Sym().Name)
				}
				slotsSeen[node.FrameOffset()] = node
			}
			typebits.Set(node.Type(), node.FrameOffset()+lv.stkptrsize, locals)
		}
	}
}

// IsUnsafe indicates that all points in this function are
// unsafe-points.
func IsUnsafe(f *ssa.Func) bool {
	// The runtime assumes the only safe-points are function
	// prologues (because that's how it used to be). We could and
	// should improve that, but for now keep consider all points
	// in the runtime unsafe. obj will add prologues and their
	// safe-points.
	//
	// go:nosplit functions are similar. Since safe points used to
	// be coupled with stack checks, go:nosplit often actually
	// means "no safe points in this function".
	return base.Flag.CompilingRuntime || f.NoSplit
}

// markUnsafePoints finds unsafe points and computes lv.unsafePoints.
func (lv *Liveness) markUnsafePoints() {
	if IsUnsafe(lv.f) {
		// No complex analysis necessary.
		lv.allUnsafe = true
		return
	}

	lv.unsafePoints = bitvec.New(int32(lv.f.NumValues()))
	lv.unsafeBlocks = bitvec.New(int32(lv.f.NumBlocks()))

	// Mark architecture-specific unsafe points.
	for _, b := range lv.f.Blocks {
		for _, v := range b.Values {
			if v.Op.UnsafePoint() {
				lv.unsafePoints.Set(int32(v.ID))
			}
		}
	}

	for _, b := range lv.f.Blocks {
		for _, v := range b.Values {
			if v.Op != ssa.OpWBend {
				continue
			}
			// WBend appears at the start of a block, like this:
			//    ...
			//    if wbEnabled: goto C else D
			// C:
			//    ... some write barrier enabled code ...
			//    goto B
			// D:
			//    ... some write barrier disabled code ...
			//    goto B
			// B:
			//    m1 = Phi mem_C mem_D
			//    m2 = store operation ... m1
			//    m3 = store operation ... m2
			//    m4 = WBend m3

			// Find first memory op in the block, which should be a Phi.
			m := v
			for {
				m = m.MemoryArg()
				if m.Block != b {
					lv.f.Fatalf("can't find Phi before write barrier end mark %v", v)
				}
				if m.Op == ssa.OpPhi {
					break
				}
			}
			// Find the two predecessor blocks (write barrier on and write barrier off)
			if len(m.Args) != 2 {
				lv.f.Fatalf("phi before write barrier end mark has %d args, want 2", len(m.Args))
			}
			c := b.Preds[0].Block()
			d := b.Preds[1].Block()

			// Find their common predecessor block (the one that branches based on wb on/off).
			// It might be a diamond pattern, or one of the blocks in the diamond pattern might
			// be missing.
			var decisionBlock *ssa.Block
			if len(c.Preds) == 1 && c.Preds[0].Block() == d {
				decisionBlock = d
			} else if len(d.Preds) == 1 && d.Preds[0].Block() == c {
				decisionBlock = c
			} else if len(c.Preds) == 1 && len(d.Preds) == 1 && c.Preds[0].Block() == d.Preds[0].Block() {
				decisionBlock = c.Preds[0].Block()
			} else {
				lv.f.Fatalf("can't find write barrier pattern %v", v)
			}
			if len(decisionBlock.Succs) != 2 {
				lv.f.Fatalf("common predecessor block the wrong type %s", decisionBlock.Kind)
			}

			// Flow backwards from the control value to find the
			// flag load. We don't know what lowered ops we're
			// looking for, but all current arches produce a
			// single op that does the memory load from the flag
			// address, so we look for that.
			var load *ssa.Value
			v := decisionBlock.Controls[0]
			for {
				if v.MemoryArg() != nil {
					// Single instruction to load (and maybe compare) the write barrier flag.
					if sym, ok := v.Aux.(*obj.LSym); ok && sym == ir.Syms.WriteBarrier {
						load = v
						break
					}
					// Some architectures have to materialize the address separate from
					// the load.
					if sym, ok := v.Args[0].Aux.(*obj.LSym); ok && sym == ir.Syms.WriteBarrier {
						load = v
						break
					}
					v.Fatalf("load of write barrier flag not from correct global: %s", v.LongString())
				}
				// Common case: just flow backwards.
				if len(v.Args) == 1 || len(v.Args) == 2 && v.Args[0] == v.Args[1] {
					// Note: 386 lowers Neq32 to (TESTL cond cond),
					v = v.Args[0]
					continue
				}
				v.Fatalf("write barrier control value has more than one argument: %s", v.LongString())
			}

			// Mark everything after the load unsafe.
			found := false
			for _, v := range decisionBlock.Values {
				if found {
					lv.unsafePoints.Set(int32(v.ID))
				}
				found = found || v == load
			}
			lv.unsafeBlocks.Set(int32(decisionBlock.ID))

			// Mark the write barrier on/off blocks as unsafe.
			for _, e := range decisionBlock.Succs {
				x := e.Block()
				if x == b {
					continue
				}
				for _, v := range x.Values {
					lv.unsafePoints.Set(int32(v.ID))
				}
				lv.unsafeBlocks.Set(int32(x.ID))
			}

			// Mark from the join point up to the WBend as unsafe.
			for _, v := range b.Values {
				if v.Op == ssa.OpWBend {
					break
				}
				lv.unsafePoints.Set(int32(v.ID))
			}
		}
	}
}

// Returns true for instructions that must have a stack map.
//
// This does not necessarily mean the instruction is a safe-point. In
// particular, call Values can have a stack map in case the callee
// grows the stack, but not themselves be a safe-point.
func (lv *Liveness) hasStackMap(v *ssa.Value) bool {
	if !v.Op.IsCall() {
		return false
	}
	// wbZero and wbCopy are write barriers and
	// deeply non-preemptible. They are unsafe points and
	// hence should not have liveness maps.
	if sym, ok := v.Aux.(*ssa.AuxCall); ok && (sym.Fn == ir.Syms.WBZero || sym.Fn == ir.Syms.WBMove) {
		return false
	}
	return true
}

// Initializes the sets for solving the live variables. Visits all the
// instructions in each basic block to summarizes the information at each basic
// block
func (lv *Liveness) prologue() {
	lv.initcache()

	for _, b := range lv.f.Blocks {
		be := lv.blockEffects(b)

		// Walk the block instructions backward and update the block
		// effects with the each prog effects.
		for j := len(b.Values) - 1; j >= 0; j-- {
			pos, e := lv.valueEffects(b.Values[j])
			if e&varkill != 0 {
				be.varkill.Set(pos)
				be.uevar.Unset(pos)
			}
			if e&uevar != 0 {
				be.uevar.Set(pos)
			}
		}
	}
}

// Solve the liveness dataflow equations.
func (lv *Liveness) solve() {
	// These temporary bitvectors exist to avoid successive allocations and
	// frees within the loop.
	nvars := int32(len(lv.vars))
	newlivein := bitvec.New(nvars)
	newliveout := bitvec.New(nvars)

	// Walk blocks in postorder ordering. This improves convergence.
	po := lv.f.Postorder()

	// Iterate through the blocks in reverse round-robin fashion. A work
	// queue might be slightly faster. As is, the number of iterations is
	// so low that it hardly seems to be worth the complexity.

	for change := true; change; {
		change = false
		for _, b := range po {
			be := lv.blockEffects(b)

			newliveout.Clear()
			switch b.Kind {
			case ssa.BlockRet:
				for _, pos := range lv.cache.retuevar {
					newliveout.Set(pos)
				}
			case ssa.BlockRetJmp:
				for _, pos := range lv.cache.tailuevar {
					newliveout.Set(pos)
				}
			case ssa.BlockExit:
				// panic exit - nothing to do
			default:
				// A variable is live on output from this block
				// if it is live on input to some successor.
				//
				// out[b] = \bigcup_{s \in succ[b]} in[s]
				newliveout.Copy(lv.blockEffects(b.Succs[0].Block()).livein)
				for _, succ := range b.Succs[1:] {
					newliveout.Or(newliveout, lv.blockEffects(succ.Block()).livein)
				}
			}

			if !be.liveout.Eq(newliveout) {
				change = true
				be.liveout.Copy(newliveout)
			}

			// A variable is live on input to this block
			// if it is used by this block, or live on output from this block and
			// not set by the code in this block.
			//
			// in[b] = uevar[b] \cup (out[b] \setminus varkill[b])
			newlivein.AndNot(be.liveout, be.varkill)
			be.livein.Or(newlivein, be.uevar)
		}
	}
}

// Visits all instructions in a basic block and computes a bit vector of live
// variables at each safe point locations.
func (lv *Liveness) epilogue() {
	nvars := int32(len(lv.vars))
	liveout := bitvec.New(nvars)
	livedefer := bitvec.New(nvars) // always-live variables

	// If there is a defer (that could recover), then all output
	// parameters are live all the time.  In addition, any locals
	// that are pointers to heap-allocated output parameters are
	// also always live (post-deferreturn code needs these
	// pointers to copy values back to the stack).
	// TODO: if the output parameter is heap-allocated, then we
	// don't need to keep the stack copy live?
	if lv.fn.HasDefer() {
		for i, n := range lv.vars {
			if n.Class == ir.PPARAMOUT {
				if n.IsOutputParamHeapAddr() {
					// Just to be paranoid.  Heap addresses are PAUTOs.
					base.Fatalf("variable %v both output param and heap output param", n)
				}
				if n.Heapaddr != nil {
					// If this variable moved to the heap, then
					// its stack copy is not live.
					continue
				}
				// Note: zeroing is handled by zeroResults in ../ssagen/ssa.go.
				livedefer.Set(int32(i))
			}
			if n.IsOutputParamHeapAddr() {
				// This variable will be overwritten early in the function
				// prologue (from the result of a mallocgc) but we need to
				// zero it in case that malloc causes a stack scan.
				n.SetNeedzero(true)
				livedefer.Set(int32(i))
			}
			if n.OpenDeferSlot() {
				// Open-coded defer args slots must be live
				// everywhere in a function, since a panic can
				// occur (almost) anywhere. Because it is live
				// everywhere, it must be zeroed on entry.
				livedefer.Set(int32(i))
				// It was already marked as Needzero when created.
				if !n.Needzero() {
					base.Fatalf("all pointer-containing defer arg slots should have Needzero set")
				}
			}
		}
	}

	// We must analyze the entry block first. The runtime assumes
	// the function entry map is index 0. Conveniently, layout
	// already ensured that the entry block is first.
	if lv.f.Entry != lv.f.Blocks[0] {
		lv.f.Fatalf("entry block must be first")
	}

	{
		// Reserve an entry for function entry.
		live := bitvec.New(nvars)
		lv.livevars = append(lv.livevars, live)
	}

	for _, b := range lv.f.Blocks {
		be := lv.blockEffects(b)

		// Walk forward through the basic block instructions and
		// allocate liveness maps for those instructions that need them.
		for _, v := range b.Values {
			if !lv.hasStackMap(v) {
				continue
			}

			live := bitvec.New(nvars)
			lv.livevars = append(lv.livevars, live)
		}

		// walk backward, construct maps at each safe point
		index := int32(len(lv.livevars) - 1)

		liveout.Copy(be.liveout)
		for i := len(b.Values) - 1; i >= 0; i-- {
			v := b.Values[i]

			if lv.hasStackMap(v) {
				// Found an interesting instruction, record the
				// corresponding liveness information.

				live := &lv.livevars[index]
				live.Or(*live, liveout)
				live.Or(*live, livedefer) // only for non-entry safe points
				index--
			}

			// Update liveness information.
			pos, e := lv.valueEffects(v)
			if e&varkill != 0 {
				liveout.Unset(pos)
			}
			if e&uevar != 0 {
				liveout.Set(pos)
			}
		}

		if b == lv.f.Entry {
			if index != 0 {
				base.Fatalf("bad index for entry point: %v", index)
			}

			// Check to make sure only input variables are live.
			for i, n := range lv.vars {
				if !liveout.Get(int32(i)) {
					continue
				}
				if n.Class == ir.PPARAM {
					continue // ok
				}
				base.FatalfAt(n.Pos(), "bad live variable at entry of %v: %L", lv.fn.Nname, n)
			}

			// Record live variables.
			live := &lv.livevars[index]
			live.Or(*live, liveout)
		}

		if lv.doClobber {
			lv.clobber(b)
		}

		// The liveness maps for this block are now complete. Compact them.
		lv.compact(b)
	}

	// If we have an open-coded deferreturn call, make a liveness map for it.
	if lv.fn.OpenCodedDeferDisallowed() {
		lv.livenessMap.DeferReturn = objw.StackMapDontCare
	} else {
		idx, _ := lv.stackMapSet.add(livedefer)
		lv.livenessMap.DeferReturn = objw.StackMapIndex(idx)
	}

	// Done compacting. Throw out the stack map set.
	lv.stackMaps = lv.stackMapSet.extractUnique()
	lv.stackMapSet = bvecSet{}

	// Useful sanity check: on entry to the function,
	// the only things that can possibly be live are the
	// input parameters.
	for j, n := range lv.vars {
		if n.Class != ir.PPARAM && lv.stackMaps[0].Get(int32(j)) {
			lv.f.Fatalf("%v %L recorded as live on entry", lv.fn.Nname, n)
		}
	}
}

// Compact coalesces identical bitmaps from lv.livevars into the sets
// lv.stackMapSet.
//
// Compact clears lv.livevars.
//
// There are actually two lists of bitmaps, one list for the local variables and one
// list for the function arguments. Both lists are indexed by the same PCDATA
// index, so the corresponding pairs must be considered together when
// merging duplicates. The argument bitmaps change much less often during
// function execution than the local variable bitmaps, so it is possible that
// we could introduce a separate PCDATA index for arguments vs locals and
// then compact the set of argument bitmaps separately from the set of
// local variable bitmaps. As of 2014-04-02, doing this to the godoc binary
// is actually a net loss: we save about 50k of argument bitmaps but the new
// PCDATA tables cost about 100k. So for now we keep using a single index for
// both bitmap lists.
func (lv *Liveness) compact(b *ssa.Block) {
	pos := 0
	if b == lv.f.Entry {
		// Handle entry stack map.
		lv.stackMapSet.add(lv.livevars[0])
		pos++
	}
	for _, v := range b.Values {
		if lv.hasStackMap(v) {
			idx, _ := lv.stackMapSet.add(lv.livevars[pos])
			pos++
			lv.livenessMap.set(v, objw.StackMapIndex(idx))
		}
		if lv.allUnsafe || v.Op != ssa.OpClobber && lv.unsafePoints.Get(int32(v.ID)) {
			lv.livenessMap.setUnsafeVal(v)
		}
	}
	if lv.allUnsafe || lv.unsafeBlocks.Get(int32(b.ID)) {
		lv.livenessMap.setUnsafeBlock(b)
	}

	// Reset livevars.
	lv.livevars = lv.livevars[:0]
}

func (lv *Liveness) enableClobber() {
	// The clobberdead experiment inserts code to clobber pointer slots in all
	// the dead variables (locals and args) at every synchronous safepoint.
	if !base.Flag.ClobberDead {
		return
	}
	if lv.fn.Pragma&ir.CgoUnsafeArgs != 0 {
		// C or assembly code uses the exact frame layout. Don't clobber.
		return
	}
	if len(lv.vars) > 10000 || len(lv.f.Blocks) > 10000 {
		// Be careful to avoid doing too much work.
		// Bail if >10000 variables or >10000 blocks.
		// Otherwise, giant functions make this experiment generate too much code.
		return
	}
	if lv.f.Name == "forkAndExecInChild" {
		// forkAndExecInChild calls vfork on some platforms.
		// The code we add here clobbers parts of the stack in the child.
		// When the parent resumes, it is using the same stack frame. But the
		// child has clobbered stack variables that the parent needs. Boom!
		// In particular, the sys argument gets clobbered.
		return
	}
	if lv.f.Name == "wbBufFlush" ||
		((lv.f.Name == "callReflect" || lv.f.Name == "callMethod") && lv.fn.ABIWrapper()) {
		// runtime.wbBufFlush must not modify its arguments. See the comments
		// in runtime/mwbbuf.go:wbBufFlush.
		//
		// reflect.callReflect and reflect.callMethod are called from special
		// functions makeFuncStub and methodValueCall. The runtime expects
		// that it can find the first argument (ctxt) at 0(SP) in makeFuncStub
		// and methodValueCall's frame (see runtime/traceback.go:getArgInfo).
		// Normally callReflect and callMethod already do not modify the
		// argument, and keep it alive. But the compiler-generated ABI wrappers
		// don't do that. Special case the wrappers to not clobber its arguments.
		lv.noClobberArgs = true
	}
	if h := os.Getenv("GOCLOBBERDEADHASH"); h != "" {
		// Clobber only functions where the hash of the function name matches a pattern.
		// Useful for binary searching for a miscompiled function.
		hstr := ""
		for _, b := range hash.Sum32([]byte(lv.f.Name)) {
			hstr += fmt.Sprintf("%08b", b)
		}
		if !strings.HasSuffix(hstr, h) {
			return
		}
		fmt.Printf("\t\t\tCLOBBERDEAD %s\n", lv.f.Name)
	}
	lv.doClobber = true
}

// Inserts code to clobber pointer slots in all the dead variables (locals and args)
// at every synchronous safepoint in b.
func (lv *Liveness) clobber(b *ssa.Block) {
	// Copy block's values to a temporary.
	oldSched := append([]*ssa.Value{}, b.Values...)
	b.Values = b.Values[:0]
	idx := 0

	// Clobber pointer slots in all dead variables at entry.
	if b == lv.f.Entry {
		for len(oldSched) > 0 && len(oldSched[0].Args) == 0 {
			// Skip argless ops. We need to skip at least
			// the lowered ClosurePtr op, because it
			// really wants to be first. This will also
			// skip ops like InitMem and SP, which are ok.
			b.Values = append(b.Values, oldSched[0])
			oldSched = oldSched[1:]
		}
		clobber(lv, b, lv.livevars[0])
		idx++
	}

	// Copy values into schedule, adding clobbering around safepoints.
	for _, v := range oldSched {
		if !lv.hasStackMap(v) {
			b.Values = append(b.Values, v)
			continue
		}
		clobber(lv, b, lv.livevars[idx])
		b.Values = append(b.Values, v)
		idx++
	}
}

// clobber generates code to clobber pointer slots in all dead variables
// (those not marked in live). Clobbering instructions are added to the end
// of b.Values.
func clobber(lv *Liveness, b *ssa.Block, live bitvec.BitVec) {
	for i, n := range lv.vars {
		if !live.Get(int32(i)) && !n.Addrtaken() && !n.OpenDeferSlot() && !n.IsOutputParamHeapAddr() {
			// Don't clobber stack objects (address-taken). They are
			// tracked dynamically.
			// Also don't clobber slots that are live for defers (see
			// the code setting livedefer in epilogue).
			if lv.noClobberArgs && n.Class == ir.PPARAM {
				continue
			}
			clobberVar(b, n)
		}
	}
}

// clobberVar generates code to trash the pointers in v.
// Clobbering instructions are added to the end of b.Values.
func clobberVar(b *ssa.Block, v *ir.Name) {
	clobberWalk(b, v, 0, v.Type())
}

// b = block to which we append instructions
// v = variable
// offset = offset of (sub-portion of) variable to clobber (in bytes)
// t = type of sub-portion of v.
func clobberWalk(b *ssa.Block, v *ir.Name, offset int64, t *types.Type) {
	if !t.HasPointers() {
		return
	}
	switch t.Kind() {
	case types.TPTR,
		types.TUNSAFEPTR,
		types.TFUNC,
		types.TCHAN,
		types.TMAP:
		clobberPtr(b, v, offset)

	case types.TSTRING:
		// struct { byte *str; int len; }
		clobberPtr(b, v, offset)

	case types.TINTER:
		// struct { Itab *tab; void *data; }
		// or, when isnilinter(t)==true:
		// struct { Type *type; void *data; }
		clobberPtr(b, v, offset)
		clobberPtr(b, v, offset+int64(types.PtrSize))

	case types.TSLICE:
		// struct { byte *array; int len; int cap; }
		clobberPtr(b, v, offset)

	case types.TARRAY:
		for i := int64(0); i < t.NumElem(); i++ {
			clobberWalk(b, v, offset+i*t.Elem().Size(), t.Elem())
		}

	case types.TSTRUCT:
		for _, t1 := range t.Fields() {
			clobberWalk(b, v, offset+t1.Offset, t1.Type)
		}

	default:
		base.Fatalf("clobberWalk: unexpected type, %v", t)
	}
}

// clobberPtr generates a clobber of the pointer at offset offset in v.
// The clobber instruction is added at the end of b.
func clobberPtr(b *ssa.Block, v *ir.Name, offset int64) {
	b.NewValue0IA(src.NoXPos, ssa.OpClobber, types.TypeVoid, offset, v)
}

func (lv *Liveness) showlive(v *ssa.Value, live bitvec.BitVec) {
	if base.Flag.Live == 0 || ir.FuncName(lv.fn) == "init" || strings.HasPrefix(ir.FuncName(lv.fn), ".") {
		return
	}
	if lv.fn.Wrapper() || lv.fn.Dupok() {
		// Skip reporting liveness information for compiler-generated wrappers.
		return
	}
	if !(v == nil || v.Op.IsCall()) {
		// Historically we only printed this information at
		// calls. Keep doing so.
		return
	}
	if live.IsEmpty() {
		return
	}

	pos, s := lv.format(v, live)

	base.WarnfAt(pos, "%s", s)
}

func (lv *Liveness) Format(v *ssa.Value) string {
	if v == nil {
		_, s := lv.format(nil, lv.stackMaps[0])
		return s
	}
	if idx := lv.livenessMap.Get(v); idx.StackMapValid() {
		_, s := lv.format(v, lv.stackMaps[idx])
		return s
	}
	return ""
}

func (lv *Liveness) format(v *ssa.Value, live bitvec.BitVec) (src.XPos, string) {
	pos := lv.fn.Nname.Pos()
	if v != nil {
		pos = v.Pos
	}

	s := "live at "
	if v == nil {
		s += fmt.Sprintf("entry to %s:", ir.FuncName(lv.fn))
	} else if sym, ok := v.Aux.(*ssa.AuxCall); ok && sym.Fn != nil {
		fn := sym.Fn.Name
		if pos := strings.Index(fn, "."); pos >= 0 {
			fn = fn[pos+1:]
		}
		s += fmt.Sprintf("call to %s:", fn)
	} else {
		s += "indirect call:"
	}

	// Sort variable names for display. Variables aren't in any particular order, and
	// the order can change by architecture, particularly with differences in regabi.
	var names []string
	for j, n := range lv.vars {
		if live.Get(int32(j)) {
			names = append(names, n.Sym().Name)
		}
	}
	sort.Strings(names)
	for _, v := range names {
		s += " " + v
	}
	return pos, s
}

func (lv *Liveness) printbvec(printed bool, name string, live bitvec.BitVec) bool {
	if live.IsEmpty() {
		return printed
	}

	if !printed {
		fmt.Printf("\t")
	} else {
		fmt.Printf(" ")
	}
	fmt.Printf("%s=", name)

	comma := ""
	for i, n := range lv.vars {
		if !live.Get(int32(i)) {
			continue
		}
		fmt.Printf("%s%s", comma, n.Sym().Name)
		comma = ","
	}
	return true
}

// printeffect is like printbvec, but for valueEffects.
func (lv *Liveness) printeffect(printed bool, name string, pos int32, x bool) bool {
	if !x {
		return printed
	}
	if !printed {
		fmt.Printf("\t")
	} else {
		fmt.Printf(" ")
	}
	fmt.Printf("%s=", name)
	if x {
		fmt.Printf("%s", lv.vars[pos].Sym().Name)
	}

	return true
}

// Prints the computed liveness information and inputs, for debugging.
// This format synthesizes the information used during the multiple passes
// into a single presentation.
func (lv *Liveness) printDebug() {
	fmt.Printf("liveness: %s\n", ir.FuncName(lv.fn))

	for i, b := range lv.f.Blocks {
		if i > 0 {
			fmt.Printf("\n")
		}

		// bb#0 pred=1,2 succ=3,4
		fmt.Printf("bb#%d pred=", b.ID)
		for j, pred := range b.Preds {
			if j > 0 {
				fmt.Printf(",")
			}
			fmt.Printf("%d", pred.Block().ID)
		}
		fmt.Printf(" succ=")
		for j, succ := range b.Succs {
			if j > 0 {
				fmt.Printf(",")
			}
			fmt.Printf("%d", succ.Block().ID)
		}
		fmt.Printf("\n")

		be := lv.blockEffects(b)

		// initial settings
		printed := false
		printed = lv.printbvec(printed, "uevar", be.uevar)
		printed = lv.printbvec(printed, "livein", be.livein)
		if printed {
			fmt.Printf("\n")
		}

		// program listing, with individual effects listed

		if b == lv.f.Entry {
			live := lv.stackMaps[0]
			fmt.Printf("(%s) function entry\n", base.FmtPos(lv.fn.Nname.Pos()))
			fmt.Printf("\tlive=")
			printed = false
			for j, n := range lv.vars {
				if !live.Get(int32(j)) {
					continue
				}
				if printed {
					fmt.Printf(",")
				}
				fmt.Printf("%v", n)
				printed = true
			}
			fmt.Printf("\n")
		}

		for _, v := range b.Values {
			fmt.Printf("(%s) %v\n", base.FmtPos(v.Pos), v.LongString())

			pcdata := lv.livenessMap.Get(v)

			pos, effect := lv.valueEffects(v)
			printed = false
			printed = lv.printeffect(printed, "uevar", pos, effect&uevar != 0)
			printed = lv.printeffect(printed, "varkill", pos, effect&varkill != 0)
			if printed {
				fmt.Printf("\n")
			}

			if pcdata.StackMapValid() {
				fmt.Printf("\tlive=")
				printed = false
				if pcdata.StackMapValid() {
					live := lv.stackMaps[pcdata]
					for j, n := range lv.vars {
						if !live.Get(int32(j)) {
							continue
						}
						if printed {
							fmt.Printf(",")
						}
						fmt.Printf("%v", n)
						printed = true
					}
				}
				fmt.Printf("\n")
			}

			if lv.livenessMap.GetUnsafe(v) {
				fmt.Printf("\tunsafe-point\n")
			}
		}
		if lv.livenessMap.GetUnsafeBlock(b) {
			fmt.Printf("\tunsafe-block\n")
		}

		// bb bitsets
		fmt.Printf("end\n")
		printed = false
		printed = lv.printbvec(printed, "varkill", be.varkill)
		printed = lv.printbvec(printed, "liveout", be.liveout)
		if printed {
			fmt.Printf("\n")
		}
	}

	fmt.Printf("\n")
}

// Dumps a slice of bitmaps to a symbol as a sequence of uint32 values. The
// first word dumped is the total number of bitmaps. The second word is the
// length of the bitmaps. All bitmaps are assumed to be of equal length. The
// remaining bytes are the raw bitmaps.
func (lv *Liveness) emit() (argsSym, liveSym *obj.LSym) {
	// Size args bitmaps to be just large enough to hold the largest pointer.
	// First, find the largest Xoffset node we care about.
	// (Nodes without pointers aren't in lv.vars; see ShouldTrack.)
	var maxArgNode *ir.Name
	for _, n := range lv.vars {
		switch n.Class {
		case ir.PPARAM, ir.PPARAMOUT:
			if !n.IsOutputParamInRegisters() {
				if maxArgNode == nil || n.FrameOffset() > maxArgNode.FrameOffset() {
					maxArgNode = n
				}
			}
		}
	}
	// Next, find the offset of the largest pointer in the largest node.
	var maxArgs int64
	if maxArgNode != nil {
		maxArgs = maxArgNode.FrameOffset() + types.PtrDataSize(maxArgNode.Type())
	}

	// Size locals bitmaps to be stkptrsize sized.
	// We cannot shrink them to only hold the largest pointer,
	// because their size is used to calculate the beginning
	// of the local variables frame.
	// Further discussion in https://golang.org/cl/104175.
	// TODO: consider trimming leading zeros.
	// This would require shifting all bitmaps.
	maxLocals := lv.stkptrsize

	// Temporary symbols for encoding bitmaps.
	var argsSymTmp, liveSymTmp obj.LSym

	args := bitvec.New(int32(maxArgs / int64(types.PtrSize)))
	aoff := objw.Uint32(&argsSymTmp, 0, uint32(len(lv.stackMaps))) // number of bitmaps
	aoff = objw.Uint32(&argsSymTmp, aoff, uint32(args.N))          // number of bits in each bitmap

	locals := bitvec.New(int32(maxLocals / int64(types.PtrSize)))
	loff := objw.Uint32(&liveSymTmp, 0, uint32(len(lv.stackMaps))) // number of bitmaps
	loff = objw.Uint32(&liveSymTmp, loff, uint32(locals.N))        // number of bits in each bitmap

	// Check for overflow before serializing stackmaps
	checkStackmapOverflow(args, len(lv.stackMaps), lv.fn.Pos())
	checkStackmapOverflow(locals, len(lv.stackMaps), lv.fn.Pos())

	for _, live := range lv.stackMaps {
		args.Clear()
		locals.Clear()

		lv.pointerMap(live, lv.vars, args, locals)

		aoff = objw.BitVec(&argsSymTmp, aoff, args)
		loff = objw.BitVec(&liveSymTmp, loff, locals)
	}

	// These symbols will be added to Ctxt.Data by addGCLocals
	// after parallel compilation is done.
	return base.Ctxt.GCLocalsSym(argsSymTmp.P), base.Ctxt.GCLocalsSym(liveSymTmp.P)
}

// Entry pointer for Compute analysis. Solves for the Compute of
// pointer variables in the function and emits a runtime data
// structure read by the garbage collector.
// Returns a map from GC safe points to their corresponding stack map index,
// and a map that contains all input parameters that may be partially live.
func Compute(curfn *ir.Func, f *ssa.Func, stkptrsize int64, pp *objw.Progs, retLiveness bool) (Map, map[*ir.Name]bool, *Liveness) {
	// Construct the global liveness state.
	vars, idx := getvariables(curfn)
	lv := newliveness(curfn, f, vars, idx, stkptrsize)

	// Run the dataflow framework.
	lv.prologue()
	lv.solve()
	lv.epilogue()
	if base.Flag.Live > 0 {
		lv.showlive(nil, lv.stackMaps[0])
		for _, b := range f.Blocks {
			for _, val := range b.Values {
				if idx := lv.livenessMap.Get(val); idx.StackMapValid() {
					lv.showlive(val, lv.stackMaps[idx])
				}
			}
		}
	}
	if base.Flag.Live >= 2 {
		lv.printDebug()
	}

	// Update the function cache.
	{
		cache := f.Cache.Liveness.(*livenessFuncCache)
		if cap(lv.be) < 2000 { // Threshold from ssa.Cache slices.
			clear(lv.be)
			cache.be = lv.be
		}
		if len(lv.livenessMap.Vals) < 2000 {
			cache.livenessMap = lv.livenessMap
		}
	}

	// Emit the live pointer map data structures
	ls := curfn.LSym
	fninfo := ls.Func()
	fninfo.GCArgs, fninfo.GCLocals = lv.emit()

	p := pp.Prog(obj.AFUNCDATA)
	p.From.SetConst(rtabi.FUNCDATA_ArgsPointerMaps)
	p.To.Type = obj.TYPE_MEM
	p.To.Name = obj.NAME_EXTERN
	p.To.Sym = fninfo.GCArgs

	p = pp.Prog(obj.AFUNCDATA)
	p.From.SetConst(rtabi.FUNCDATA_LocalsPointerMaps)
	p.To.Type = obj.TYPE_MEM
	p.To.Name = obj.NAME_EXTERN
	p.To.Sym = fninfo.GCLocals

	if x := lv.emitStackObjects(); x != nil {
		p := pp.Prog(obj.AFUNCDATA)
		p.From.SetConst(rtabi.FUNCDATA_StackObjects)
		p.To.Type = obj.TYPE_MEM
		p.To.Name = obj.NAME_EXTERN
		p.To.Sym = x
	}

	retLv := lv
	if !retLiveness {
		retLv = nil
	}

	return lv.livenessMap, lv.partLiveArgs, retLv
}

func (lv *Liveness) emitStackObjects() *obj.LSym {
	var vars []*ir.Name
	for _, n := range lv.fn.Dcl {
		if shouldTrack(n) && n.Addrtaken() && n.Esc() != ir.EscHeap {
			vars = append(vars, n)
		}
	}
	if len(vars) == 0 {
		return nil
	}

	// Sort variables from lowest to highest address.
	slices.SortFunc(vars, func(a, b *ir.Name) int { return cmp.Compare(a.FrameOffset(), b.FrameOffset()) })

	// Populate the stack object data.
	// Format must match runtime/stack.go:stackObjectRecord.
	x := base.Ctxt.Lookup(lv.fn.LSym.Name + ".stkobj")
	x.Set(obj.AttrContentAddressable, true)
	x.Align = 4
	lv.fn.LSym.Func().StackObjects = x
	off := 0
	off = objw.Uintptr(x, off, uint64(len(vars)))
	for _, v := range vars {
		// Note: arguments and return values have non-negative Xoffset,
		// in which case the offset is relative to argp.
		// Locals have a negative Xoffset, in which case the offset is relative to varp.
		// We already limit the frame size, so the offset and the object size
		// should not be too big.
		frameOffset := v.FrameOffset()
		if frameOffset != int64(int32(frameOffset)) {
			base.Fatalf("frame offset too big: %v %d", v, frameOffset)
		}
		off = objw.Uint32(x, off, uint32(frameOffset))

		t := v.Type()
		sz := t.Size()
		if sz != int64(int32(sz)) {
			base.Fatalf("stack object too big: %v of type %v, size %d", v, t, sz)
		}
		lsym, ptrBytes := reflectdata.GCSym(t, false)
		off = objw.Uint32(x, off, uint32(sz))
		off = objw.Uint32(x, off, uint32(ptrBytes))
		off = objw.SymPtrOff(x, off, lsym)
	}

	if base.Flag.Live != 0 {
		for _, v := range vars {
			base.WarnfAt(v.Pos(), "stack object %v %v", v, v.Type())
		}
	}

	return x
}

// isfat reports whether a variable of type t needs multiple assignments to initialize.
// For example:
//
//	type T struct { x, y int }
//	x := T{x: 0, y: 1}
//
// Then we need:
//
//	var t T
//	t.x = 0
//	t.y = 1
//
// to fully initialize t.
func isfat(t *types.Type) bool {
	if t != nil {
		switch t.Kind() {
		case types.TSLICE, types.TSTRING,
			types.TINTER: // maybe remove later
			return true
		case types.TARRAY:
			// Array of 1 element, check if element is fat
			if t.NumElem() == 1 {
				return isfat(t.Elem())
			}
			return true
		case types.TSTRUCT:
			if t.IsSIMD() {
				return false
			}
			// Struct with 1 field, check if field is fat
			if t.NumFields() == 1 {
				return isfat(t.Field(0).Type)
			}
			return true
		}
	}

	return false
}

// WriteFuncMap writes the pointer bitmaps for bodyless function fn's
// inputs and outputs as the value of symbol <fn>.args_stackmap.
// If fn has outputs, two bitmaps are written, otherwise just one.
func WriteFuncMap(fn *ir.Func, abiInfo *abi.ABIParamResultInfo) {
	if ir.FuncName(fn) == "_" {
		return
	}
	nptr := int(abiInfo.ArgWidth() / int64(types.PtrSize))
	bv := bitvec.New(int32(nptr))

	for _, p := range abiInfo.InParams() {
		typebits.SetNoCheck(p.Type, p.FrameOffset(abiInfo), bv)
	}

	nbitmap := 1
	if fn.Type().NumResults() > 0 {
		nbitmap = 2
	}

	// defensive check: function arguments can't realistically be large enough for overflow here
	checkStackmapOverflow(bv, nbitmap, fn.Pos())

	lsym := base.Ctxt.Lookup(fn.LSym.Name + ".args_stackmap")
	lsym.Set(obj.AttrLinkname, true) // allow args_stackmap referenced from assembly
	off := objw.Uint32(lsym, 0, uint32(nbitmap))
	off = objw.Uint32(lsym, off, uint32(bv.N))
	off = objw.BitVec(lsym, off, bv)

	if fn.Type().NumResults() > 0 {
		for _, p := range abiInfo.OutParams() {
			if len(p.Registers) == 0 {
				typebits.SetNoCheck(p.Type, p.FrameOffset(abiInfo), bv)
			}
		}
		off = objw.BitVec(lsym, off, bv)
	}

	objw.Global(lsym, int32(off), obj.RODATA|obj.LOCAL)
}

// checkStackmapOverflow checks for potential overflow in runtime stackmap reading.
// Runtime computes: n * ((nbit+7)/8) using int32 arithmetic.
// See runtime.stackmapdata implementation.
func checkStackmapOverflow(bv bitvec.BitVec, count int, pos src.XPos) {
	if bv.N <= 0 || count <= 0 {
		return
	}
	bytesPerBitVec := (int64(bv.N) + 7) >> 3
	totalBytes := bytesPerBitVec * int64(count)
	if totalBytes > math.MaxInt32 {
		// runtime.stackmap has to support 64-bit values to avoid this restriction, see issue 77170
		base.FatalfAt(pos, "liveness stackmaps are too large: nbit=%d count=%d totalBytes=%d exceeds MaxInt32", bv.N, count, totalBytes)
	}
}

```

