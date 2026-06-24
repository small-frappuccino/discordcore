# Domain Architecture: cmd/compile/internal/typecheck

## Layout Topology
```text
cmd/compile/internal/typecheck/
├── _builtin
│   ├── coverage.go
│   └── runtime.go
├── bexport.go
├── builtin.go
├── const.go
├── dcl.go
├── export.go
├── expr.go
├── func.go
├── iexport.go
├── iimport.go
├── mkbuiltin.go
├── stmt.go
├── subr.go
├── syms.go
├── target.go
├── type.go
├── typecheck.go
└── universe.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/compile/internal/typecheck/_builtin/coverage.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// NOTE: If you change this file you must run "go generate"
// in cmd/compile/internal/typecheck
// to update builtin.go. This is not done automatically
// to avoid depending on having a working compiler binary.

//go:build ignore

package coverage

func initHook(istest bool)

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/_builtin/runtime.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// NOTE: If you change this file you must run "go generate"
// in cmd/compile/internal/typecheck
// to update builtin.go. This is not done automatically
// to avoid depending on having a working compiler binary.

//go:build ignore

package runtime

// emitted by compiler, not referred to by go programs

import "unsafe"

func newobject(typ *byte) *any
func mallocgc(size uintptr, typ *byte, needszero bool) unsafe.Pointer
func panicdivide()
func panicshift()
func panicmakeslicelen()
func panicmakeslicecap()
func throwinit()
func panicwrap()

func gopanic(interface{})
func gorecover() interface{}
func goschedguarded()

// Note: these declarations are just for wasm port.
// Other ports call assembly stubs instead.
func goPanicIndex(x int, y int)
func goPanicIndexU(x uint, y int)
func goPanicSliceAlen(x int, y int)
func goPanicSliceAlenU(x uint, y int)
func goPanicSliceAcap(x int, y int)
func goPanicSliceAcapU(x uint, y int)
func goPanicSliceB(x int, y int)
func goPanicSliceBU(x uint, y int)
func goPanicSlice3Alen(x int, y int)
func goPanicSlice3AlenU(x uint, y int)
func goPanicSlice3Acap(x int, y int)
func goPanicSlice3AcapU(x uint, y int)
func goPanicSlice3B(x int, y int)
func goPanicSlice3BU(x uint, y int)
func goPanicSlice3C(x int, y int)
func goPanicSlice3CU(x uint, y int)
func goPanicSliceConvert(x int, y int)

func printbool(bool)
func printfloat64(float64)
func printfloat32(float32)
func printint(int64)
func printhex(uint64)
func printuint(uint64)
func printcomplex128(complex128)
func printcomplex64(complex64)
func printstring(string)
func printquoted(string)
func printpointer(any)
func printuintptr(uintptr)
func printiface(any)
func printeface(any)
func printslice(any)
func printnl()
func printsp()
func printlock()
func printunlock()

func concatstring2(*[32]byte, string, string) string
func concatstring3(*[32]byte, string, string, string) string
func concatstring4(*[32]byte, string, string, string, string) string
func concatstring5(*[32]byte, string, string, string, string, string) string
func concatstrings(*[32]byte, []string) string

func concatbyte2(*[32]byte, string, string) []byte
func concatbyte3(*[32]byte, string, string, string) []byte
func concatbyte4(*[32]byte, string, string, string, string) []byte
func concatbyte5(*[32]byte, string, string, string, string, string) []byte
func concatbytes(*[32]byte, []string) []byte

func cmpstring(string, string) int
func intstring(*[4]byte, int64) string
func slicebytetostring(buf *[32]byte, ptr *byte, n int) string
func slicebytetostringtmp(ptr *byte, n int) string
func slicerunetostring(*[32]byte, []rune) string
func stringtoslicebyte(*[32]byte, string) []byte
func stringtoslicerune(*[32]rune, string) []rune
func slicecopy(toPtr *any, toLen int, fromPtr *any, fromLen int, wid uintptr) int

func decoderune(string, int) (retv rune, retk int)
func countrunes(string) int

// Convert non-interface type to the data word of a (empty or nonempty) interface.
func convT(typ *byte, elem *any) unsafe.Pointer

// Same as convT, for types with no pointers in them.
func convTnoptr(typ *byte, elem *any) unsafe.Pointer

// Specialized versions of convT for specific types.
// These functions take concrete types in the runtime. But they may
// be used for a wider range of types, which have the same memory
// layout as the parameter type. The compiler converts the
// to-be-converted type to the parameter type before calling the
// runtime function. This way, the call is ABI-insensitive.
func convT16(val uint16) unsafe.Pointer
func convT32(val uint32) unsafe.Pointer
func convT64(val uint64) unsafe.Pointer
func convTstring(val string) unsafe.Pointer
func convTslice(val []uint8) unsafe.Pointer

// interface type assertions x.(T)
func assertE2I(inter *byte, typ *byte) *byte
func assertE2I2(inter *byte, typ *byte) *byte
func panicdottypeE(have, want, iface *byte)
func panicdottypeI(have, want, iface *byte)
func panicnildottype(want *byte)
func typeAssert(s *byte, typ *byte) *byte

// interface switches
func interfaceSwitch(s *byte, t *byte) (int, *byte)

// interface equality. Type/itab pointers are already known to be equal, so
// we only need to pass one.
func ifaceeq(tab *uintptr, x, y unsafe.Pointer) (ret bool)
func efaceeq(typ *uintptr, x, y unsafe.Pointer) (ret bool)

// panic for various rangefunc iterator errors
func panicrangestate(state int)

// defer in range over func
func deferrangefunc() interface{}

func rand() uint64
func rand32() uint32

// *byte is really *runtime.Type
func makemap64(mapType *byte, hint int64, mapbuf *any) (hmap map[any]any)
func makemap(mapType *byte, hint int, mapbuf *any) (hmap map[any]any)
func makemap_small() (hmap map[any]any)
func mapaccess1(mapType *byte, hmap map[any]any, key *any) (val *any)
func mapaccess1_fast32(mapType *byte, hmap map[any]any, key uint32) (val *any)
func mapaccess1_fast64(mapType *byte, hmap map[any]any, key uint64) (val *any)
func mapaccess1_faststr(mapType *byte, hmap map[any]any, key string) (val *any)
func mapaccess1_fat(mapType *byte, hmap map[any]any, key *any, zero *byte) (val *any)
func mapaccess2(mapType *byte, hmap map[any]any, key *any) (val *any, pres bool)
func mapaccess2_fast32(mapType *byte, hmap map[any]any, key uint32) (val *any, pres bool)
func mapaccess2_fast64(mapType *byte, hmap map[any]any, key uint64) (val *any, pres bool)
func mapaccess2_faststr(mapType *byte, hmap map[any]any, key string) (val *any, pres bool)
func mapaccess2_fat(mapType *byte, hmap map[any]any, key *any, zero *byte) (val *any, pres bool)
func mapassign(mapType *byte, hmap map[any]any, key *any) (val *any)
func mapassign_fast32(mapType *byte, hmap map[any]any, key uint32) (val *any)
func mapassign_fast32ptr(mapType *byte, hmap map[any]any, key unsafe.Pointer) (val *any)
func mapassign_fast64(mapType *byte, hmap map[any]any, key uint64) (val *any)
func mapassign_fast64ptr(mapType *byte, hmap map[any]any, key unsafe.Pointer) (val *any)
func mapassign_faststr(mapType *byte, hmap map[any]any, key string) (val *any)
func mapIterStart(mapType *byte, hmap map[any]any, hiter *any)
func mapdelete(mapType *byte, hmap map[any]any, key *any)
func mapdelete_fast32(mapType *byte, hmap map[any]any, key uint32)
func mapdelete_fast64(mapType *byte, hmap map[any]any, key uint64)
func mapdelete_faststr(mapType *byte, hmap map[any]any, key string)
func mapIterNext(hiter *any)
func mapclear(mapType *byte, hmap map[any]any)

// *byte is really *runtime.Type
func makechan64(chanType *byte, size int64) (hchan chan any)
func makechan(chanType *byte, size int) (hchan chan any)
func chanrecv1(hchan <-chan any, elem *any)
func chanrecv2(hchan <-chan any, elem *any) bool
func chansend1(hchan chan<- any, elem *any)
func closechan(hchan chan<- any)
func chanlen(hchan any) int
func chancap(hchan any) int

var writeBarrier struct {
	enabled bool
	pad     [3]byte
	cgo     bool
	alignme uint64
}

// *byte is really *runtime.Type
func typedmemmove(typ *byte, dst *any, src *any)
func typedmemclr(typ *byte, dst *any)
func typedslicecopy(typ *byte, dstPtr *any, dstLen int, srcPtr *any, srcLen int) int

func selectnbsend(hchan chan<- any, elem *any) bool
func selectnbrecv(elem *any, hchan <-chan any) (bool, bool)

func selectsetpc(pc *uintptr)
func selectgo(cas0 *byte, order0 *byte, pc0 *uintptr, nsends int, nrecvs int, block bool) (int, bool)
func block()

func makeslice(typ *byte, len int, cap int) unsafe.Pointer
func makeslice64(typ *byte, len int64, cap int64) unsafe.Pointer
func makeslicecopy(typ *byte, tolen int, fromlen int, from unsafe.Pointer) unsafe.Pointer
func growslice(oldPtr *any, newLen, oldCap, num int, et *byte) (ary []any)
func growsliceBuf(oldPtr *any, newLen, oldCap, num int, et *byte, buf *any, bufLen int) (ary []any)
func growsliceBufNoAlias(oldPtr *any, newLen, oldCap, num int, et *byte, buf *any, bufLen int) (ary []any)
func growsliceNoAlias(oldPtr *any, newLen, oldCap, num int, et *byte) (ary []any)
func unsafeslicecheckptr(typ *byte, ptr unsafe.Pointer, len int64)
func panicunsafeslicelen()
func panicunsafeslicenilptr()
func unsafestringcheckptr(ptr unsafe.Pointer, len int64)
func panicunsafestringlen()
func panicunsafestringnilptr()

func moveSlice(typ *byte, old *byte, len, cap int) (*byte, int, int)
func moveSliceNoScan(elemSize uintptr, old *byte, len, cap int) (*byte, int, int)
func moveSliceNoCap(typ *byte, old *byte, len int) (*byte, int, int)
func moveSliceNoCapNoScan(elemSize uintptr, old *byte, len int) (*byte, int, int)

func memmove(to *any, frm *any, length uintptr)
func memclrNoHeapPointers(ptr unsafe.Pointer, n uintptr)
func memclrHasPointers(ptr unsafe.Pointer, n uintptr)

func memequal(x, y unsafe.Pointer, size uintptr) bool
func memequal0(x, y unsafe.Pointer) bool
func memequal8(x, y unsafe.Pointer) bool
func memequal16(x, y unsafe.Pointer) bool
func memequal32(x, y unsafe.Pointer) bool
func memequal64(x, y unsafe.Pointer) bool
func memequal128(x, y unsafe.Pointer) bool
func f32equal(p, q unsafe.Pointer) bool
func f64equal(p, q unsafe.Pointer) bool
func c64equal(p, q unsafe.Pointer) bool
func c128equal(p, q unsafe.Pointer) bool
func strequal(p, q unsafe.Pointer) bool
func interequal(p, q unsafe.Pointer) bool
func nilinterequal(p, q unsafe.Pointer) bool

func memhash(x unsafe.Pointer, h uintptr, size uintptr) uintptr
func memhash0(p unsafe.Pointer, h uintptr) uintptr
func memhash8(p unsafe.Pointer, h uintptr) uintptr
func memhash16(p unsafe.Pointer, h uintptr) uintptr
func memhash32(p unsafe.Pointer, h uintptr) uintptr
func memhash64(p unsafe.Pointer, h uintptr) uintptr
func memhash128(p unsafe.Pointer, h uintptr) uintptr
func f32hash(p unsafe.Pointer, h uintptr) uintptr
func f64hash(p unsafe.Pointer, h uintptr) uintptr
func c64hash(p unsafe.Pointer, h uintptr) uintptr
func c128hash(p unsafe.Pointer, h uintptr) uintptr
func strhash(a unsafe.Pointer, h uintptr) uintptr
func interhash(p unsafe.Pointer, h uintptr) uintptr
func nilinterhash(p unsafe.Pointer, h uintptr) uintptr

// only used on 32-bit
func int64div(int64, int64) int64
func uint64div(uint64, uint64) uint64
func int64mod(int64, int64) int64
func uint64mod(uint64, uint64) uint64
func float64toint64(float64) int64
func float64touint64(float64) uint64
func float64touint32(float64) uint32
func int64tofloat64(int64) float64
func int64tofloat32(int64) float32
func uint64tofloat64(uint64) float64
func uint64tofloat32(uint64) float32
func uint32tofloat64(uint32) float64

func complex128div(num complex128, den complex128) (quo complex128)

// race detection
func racefuncenter(uintptr)
func racefuncexit()
func raceread(uintptr)
func racewrite(uintptr)
func racereadrange(addr, size uintptr)
func racewriterange(addr, size uintptr)

// memory sanitizer
func msanread(addr, size uintptr)
func msanwrite(addr, size uintptr)
func msanmove(dst, src, size uintptr)

// address sanitizer
func asanread(addr, size uintptr)
func asanwrite(addr, size uintptr)

func checkptrAlignment(unsafe.Pointer, *byte, uintptr)
func checkptrArithmetic(unsafe.Pointer, []unsafe.Pointer)

func libfuzzerTraceCmp1(uint8, uint8, uint)
func libfuzzerTraceCmp2(uint16, uint16, uint)
func libfuzzerTraceCmp4(uint32, uint32, uint)
func libfuzzerTraceCmp8(uint64, uint64, uint)
func libfuzzerTraceConstCmp1(uint8, uint8, uint)
func libfuzzerTraceConstCmp2(uint16, uint16, uint)
func libfuzzerTraceConstCmp4(uint32, uint32, uint)
func libfuzzerTraceConstCmp8(uint64, uint64, uint)
func libfuzzerHookStrCmp(string, string, uint)
func libfuzzerHookEqualFold(string, string, uint)

func addCovMeta(p unsafe.Pointer, len uint32, hash [16]byte, pkpath string, pkgId int, cmode uint8, cgran uint8) uint32

// architecture variants
var x86HasAVX bool
var x86HasFMA bool
var x86HasPOPCNT bool
var x86HasSSE41 bool
var armHasVFPv4 bool
var arm64HasATOMICS bool
var loong64HasLAMCAS bool
var loong64HasLAM_BH bool
var loong64HasDBAR_HINTS bool
var loong64HasLSX bool
var riscv64HasZbb bool

func asanregisterglobals(unsafe.Pointer, uintptr)

// used by testing.B.Loop
func KeepAlive(interface{})

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/bexport.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

// Tags. Must be < 0.
const (
	// Objects
	packageTag = -(iota + 1)
	constTag
	typeTag
	varTag
	funcTag
	endTag
)

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/builtin.go ===
```go
// Code generated by mkbuiltin.go. DO NOT EDIT.

package typecheck

import (
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// Not inlining this function removes a significant chunk of init code.
//
//go:noinline
func newSig(params, results []*types.Field) *types.Type {
	return types.NewSignature(nil, params, results)
}

func params(tlist ...*types.Type) []*types.Field {
	flist := make([]*types.Field, len(tlist))
	for i, typ := range tlist {
		flist[i] = types.NewField(src.NoXPos, nil, typ)
	}
	return flist
}

var runtimeDecls = [...]struct {
	name string
	tag  int
	typ  int
}{
	{"newobject", funcTag, 4},
	{"mallocgc", funcTag, 8},
	{"panicdivide", funcTag, 9},
	{"panicshift", funcTag, 9},
	{"panicmakeslicelen", funcTag, 9},
	{"panicmakeslicecap", funcTag, 9},
	{"throwinit", funcTag, 9},
	{"panicwrap", funcTag, 9},
	{"gopanic", funcTag, 11},
	{"gorecover", funcTag, 12},
	{"goschedguarded", funcTag, 9},
	{"goPanicIndex", funcTag, 14},
	{"goPanicIndexU", funcTag, 16},
	{"goPanicSliceAlen", funcTag, 14},
	{"goPanicSliceAlenU", funcTag, 16},
	{"goPanicSliceAcap", funcTag, 14},
	{"goPanicSliceAcapU", funcTag, 16},
	{"goPanicSliceB", funcTag, 14},
	{"goPanicSliceBU", funcTag, 16},
	{"goPanicSlice3Alen", funcTag, 14},
	{"goPanicSlice3AlenU", funcTag, 16},
	{"goPanicSlice3Acap", funcTag, 14},
	{"goPanicSlice3AcapU", funcTag, 16},
	{"goPanicSlice3B", funcTag, 14},
	{"goPanicSlice3BU", funcTag, 16},
	{"goPanicSlice3C", funcTag, 14},
	{"goPanicSlice3CU", funcTag, 16},
	{"goPanicSliceConvert", funcTag, 14},
	{"printbool", funcTag, 17},
	{"printfloat64", funcTag, 19},
	{"printfloat32", funcTag, 21},
	{"printint", funcTag, 23},
	{"printhex", funcTag, 25},
	{"printuint", funcTag, 25},
	{"printcomplex128", funcTag, 27},
	{"printcomplex64", funcTag, 29},
	{"printstring", funcTag, 31},
	{"printquoted", funcTag, 31},
	{"printpointer", funcTag, 32},
	{"printuintptr", funcTag, 33},
	{"printiface", funcTag, 32},
	{"printeface", funcTag, 32},
	{"printslice", funcTag, 32},
	{"printnl", funcTag, 9},
	{"printsp", funcTag, 9},
	{"printlock", funcTag, 9},
	{"printunlock", funcTag, 9},
	{"concatstring2", funcTag, 36},
	{"concatstring3", funcTag, 37},
	{"concatstring4", funcTag, 38},
	{"concatstring5", funcTag, 39},
	{"concatstrings", funcTag, 41},
	{"concatbyte2", funcTag, 43},
	{"concatbyte3", funcTag, 44},
	{"concatbyte4", funcTag, 45},
	{"concatbyte5", funcTag, 46},
	{"concatbytes", funcTag, 47},
	{"cmpstring", funcTag, 48},
	{"intstring", funcTag, 51},
	{"slicebytetostring", funcTag, 52},
	{"slicebytetostringtmp", funcTag, 53},
	{"slicerunetostring", funcTag, 56},
	{"stringtoslicebyte", funcTag, 57},
	{"stringtoslicerune", funcTag, 60},
	{"slicecopy", funcTag, 61},
	{"decoderune", funcTag, 62},
	{"countrunes", funcTag, 63},
	{"convT", funcTag, 64},
	{"convTnoptr", funcTag, 64},
	{"convT16", funcTag, 66},
	{"convT32", funcTag, 68},
	{"convT64", funcTag, 69},
	{"convTstring", funcTag, 70},
	{"convTslice", funcTag, 73},
	{"assertE2I", funcTag, 74},
	{"assertE2I2", funcTag, 74},
	{"panicdottypeE", funcTag, 75},
	{"panicdottypeI", funcTag, 75},
	{"panicnildottype", funcTag, 76},
	{"typeAssert", funcTag, 74},
	{"interfaceSwitch", funcTag, 77},
	{"ifaceeq", funcTag, 79},
	{"efaceeq", funcTag, 79},
	{"panicrangestate", funcTag, 80},
	{"deferrangefunc", funcTag, 12},
	{"rand", funcTag, 81},
	{"rand32", funcTag, 82},
	{"makemap64", funcTag, 84},
	{"makemap", funcTag, 85},
	{"makemap_small", funcTag, 86},
	{"mapaccess1", funcTag, 87},
	{"mapaccess1_fast32", funcTag, 88},
	{"mapaccess1_fast64", funcTag, 89},
	{"mapaccess1_faststr", funcTag, 90},
	{"mapaccess1_fat", funcTag, 91},
	{"mapaccess2", funcTag, 92},
	{"mapaccess2_fast32", funcTag, 93},
	{"mapaccess2_fast64", funcTag, 94},
	{"mapaccess2_faststr", funcTag, 95},
	{"mapaccess2_fat", funcTag, 96},
	{"mapassign", funcTag, 87},
	{"mapassign_fast32", funcTag, 88},
	{"mapassign_fast32ptr", funcTag, 97},
	{"mapassign_fast64", funcTag, 89},
	{"mapassign_fast64ptr", funcTag, 97},
	{"mapassign_faststr", funcTag, 90},
	{"mapIterStart", funcTag, 98},
	{"mapdelete", funcTag, 98},
	{"mapdelete_fast32", funcTag, 99},
	{"mapdelete_fast64", funcTag, 100},
	{"mapdelete_faststr", funcTag, 101},
	{"mapIterNext", funcTag, 102},
	{"mapclear", funcTag, 103},
	{"makechan64", funcTag, 105},
	{"makechan", funcTag, 106},
	{"chanrecv1", funcTag, 108},
	{"chanrecv2", funcTag, 109},
	{"chansend1", funcTag, 111},
	{"closechan", funcTag, 112},
	{"chanlen", funcTag, 113},
	{"chancap", funcTag, 113},
	{"writeBarrier", varTag, 115},
	{"typedmemmove", funcTag, 116},
	{"typedmemclr", funcTag, 117},
	{"typedslicecopy", funcTag, 118},
	{"selectnbsend", funcTag, 119},
	{"selectnbrecv", funcTag, 120},
	{"selectsetpc", funcTag, 121},
	{"selectgo", funcTag, 122},
	{"block", funcTag, 9},
	{"makeslice", funcTag, 123},
	{"makeslice64", funcTag, 124},
	{"makeslicecopy", funcTag, 125},
	{"growslice", funcTag, 127},
	{"growsliceBuf", funcTag, 128},
	{"growsliceBufNoAlias", funcTag, 128},
	{"growsliceNoAlias", funcTag, 127},
	{"unsafeslicecheckptr", funcTag, 129},
	{"panicunsafeslicelen", funcTag, 9},
	{"panicunsafeslicenilptr", funcTag, 9},
	{"unsafestringcheckptr", funcTag, 130},
	{"panicunsafestringlen", funcTag, 9},
	{"panicunsafestringnilptr", funcTag, 9},
	{"moveSlice", funcTag, 131},
	{"moveSliceNoScan", funcTag, 132},
	{"moveSliceNoCap", funcTag, 133},
	{"moveSliceNoCapNoScan", funcTag, 134},
	{"memmove", funcTag, 135},
	{"memclrNoHeapPointers", funcTag, 136},
	{"memclrHasPointers", funcTag, 136},
	{"memequal", funcTag, 137},
	{"memequal0", funcTag, 138},
	{"memequal8", funcTag, 138},
	{"memequal16", funcTag, 138},
	{"memequal32", funcTag, 138},
	{"memequal64", funcTag, 138},
	{"memequal128", funcTag, 138},
	{"f32equal", funcTag, 138},
	{"f64equal", funcTag, 138},
	{"c64equal", funcTag, 138},
	{"c128equal", funcTag, 138},
	{"strequal", funcTag, 138},
	{"interequal", funcTag, 138},
	{"nilinterequal", funcTag, 138},
	{"memhash", funcTag, 139},
	{"memhash0", funcTag, 140},
	{"memhash8", funcTag, 140},
	{"memhash16", funcTag, 140},
	{"memhash32", funcTag, 140},
	{"memhash64", funcTag, 140},
	{"memhash128", funcTag, 140},
	{"f32hash", funcTag, 140},
	{"f64hash", funcTag, 140},
	{"c64hash", funcTag, 140},
	{"c128hash", funcTag, 140},
	{"strhash", funcTag, 140},
	{"interhash", funcTag, 140},
	{"nilinterhash", funcTag, 140},
	{"int64div", funcTag, 141},
	{"uint64div", funcTag, 142},
	{"int64mod", funcTag, 141},
	{"uint64mod", funcTag, 142},
	{"float64toint64", funcTag, 143},
	{"float64touint64", funcTag, 144},
	{"float64touint32", funcTag, 145},
	{"int64tofloat64", funcTag, 146},
	{"int64tofloat32", funcTag, 147},
	{"uint64tofloat64", funcTag, 148},
	{"uint64tofloat32", funcTag, 149},
	{"uint32tofloat64", funcTag, 150},
	{"complex128div", funcTag, 151},
	{"racefuncenter", funcTag, 33},
	{"racefuncexit", funcTag, 9},
	{"raceread", funcTag, 33},
	{"racewrite", funcTag, 33},
	{"racereadrange", funcTag, 152},
	{"racewriterange", funcTag, 152},
	{"msanread", funcTag, 152},
	{"msanwrite", funcTag, 152},
	{"msanmove", funcTag, 153},
	{"asanread", funcTag, 152},
	{"asanwrite", funcTag, 152},
	{"checkptrAlignment", funcTag, 154},
	{"checkptrArithmetic", funcTag, 156},
	{"libfuzzerTraceCmp1", funcTag, 157},
	{"libfuzzerTraceCmp2", funcTag, 158},
	{"libfuzzerTraceCmp4", funcTag, 159},
	{"libfuzzerTraceCmp8", funcTag, 160},
	{"libfuzzerTraceConstCmp1", funcTag, 157},
	{"libfuzzerTraceConstCmp2", funcTag, 158},
	{"libfuzzerTraceConstCmp4", funcTag, 159},
	{"libfuzzerTraceConstCmp8", funcTag, 160},
	{"libfuzzerHookStrCmp", funcTag, 161},
	{"libfuzzerHookEqualFold", funcTag, 161},
	{"addCovMeta", funcTag, 163},
	{"x86HasAVX", varTag, 6},
	{"x86HasFMA", varTag, 6},
	{"x86HasPOPCNT", varTag, 6},
	{"x86HasSSE41", varTag, 6},
	{"armHasVFPv4", varTag, 6},
	{"arm64HasATOMICS", varTag, 6},
	{"loong64HasLAMCAS", varTag, 6},
	{"loong64HasLAM_BH", varTag, 6},
	{"loong64HasDBAR_HINTS", varTag, 6},
	{"loong64HasLSX", varTag, 6},
	{"riscv64HasZbb", varTag, 6},
	{"asanregisterglobals", funcTag, 136},
	{"KeepAlive", funcTag, 11},
}

func runtimeTypes() []*types.Type {
	var typs [164]*types.Type
	typs[0] = types.ByteType
	typs[1] = types.NewPtr(typs[0])
	typs[2] = types.Types[types.TANY]
	typs[3] = types.NewPtr(typs[2])
	typs[4] = newSig(params(typs[1]), params(typs[3]))
	typs[5] = types.Types[types.TUINTPTR]
	typs[6] = types.Types[types.TBOOL]
	typs[7] = types.Types[types.TUNSAFEPTR]
	typs[8] = newSig(params(typs[5], typs[1], typs[6]), params(typs[7]))
	typs[9] = newSig(nil, nil)
	typs[10] = types.Types[types.TINTER]
	typs[11] = newSig(params(typs[10]), nil)
	typs[12] = newSig(nil, params(typs[10]))
	typs[13] = types.Types[types.TINT]
	typs[14] = newSig(params(typs[13], typs[13]), nil)
	typs[15] = types.Types[types.TUINT]
	typs[16] = newSig(params(typs[15], typs[13]), nil)
	typs[17] = newSig(params(typs[6]), nil)
	typs[18] = types.Types[types.TFLOAT64]
	typs[19] = newSig(params(typs[18]), nil)
	typs[20] = types.Types[types.TFLOAT32]
	typs[21] = newSig(params(typs[20]), nil)
	typs[22] = types.Types[types.TINT64]
	typs[23] = newSig(params(typs[22]), nil)
	typs[24] = types.Types[types.TUINT64]
	typs[25] = newSig(params(typs[24]), nil)
	typs[26] = types.Types[types.TCOMPLEX128]
	typs[27] = newSig(params(typs[26]), nil)
	typs[28] = types.Types[types.TCOMPLEX64]
	typs[29] = newSig(params(typs[28]), nil)
	typs[30] = types.Types[types.TSTRING]
	typs[31] = newSig(params(typs[30]), nil)
	typs[32] = newSig(params(typs[2]), nil)
	typs[33] = newSig(params(typs[5]), nil)
	typs[34] = types.NewArray(typs[0], 32)
	typs[35] = types.NewPtr(typs[34])
	typs[36] = newSig(params(typs[35], typs[30], typs[30]), params(typs[30]))
	typs[37] = newSig(params(typs[35], typs[30], typs[30], typs[30]), params(typs[30]))
	typs[38] = newSig(params(typs[35], typs[30], typs[30], typs[30], typs[30]), params(typs[30]))
	typs[39] = newSig(params(typs[35], typs[30], typs[30], typs[30], typs[30], typs[30]), params(typs[30]))
	typs[40] = types.NewSlice(typs[30])
	typs[41] = newSig(params(typs[35], typs[40]), params(typs[30]))
	typs[42] = types.NewSlice(typs[0])
	typs[43] = newSig(params(typs[35], typs[30], typs[30]), params(typs[42]))
	typs[44] = newSig(params(typs[35], typs[30], typs[30], typs[30]), params(typs[42]))
	typs[45] = newSig(params(typs[35], typs[30], typs[30], typs[30], typs[30]), params(typs[42]))
	typs[46] = newSig(params(typs[35], typs[30], typs[30], typs[30], typs[30], typs[30]), params(typs[42]))
	typs[47] = newSig(params(typs[35], typs[40]), params(typs[42]))
	typs[48] = newSig(params(typs[30], typs[30]), params(typs[13]))
	typs[49] = types.NewArray(typs[0], 4)
	typs[50] = types.NewPtr(typs[49])
	typs[51] = newSig(params(typs[50], typs[22]), params(typs[30]))
	typs[52] = newSig(params(typs[35], typs[1], typs[13]), params(typs[30]))
	typs[53] = newSig(params(typs[1], typs[13]), params(typs[30]))
	typs[54] = types.RuneType
	typs[55] = types.NewSlice(typs[54])
	typs[56] = newSig(params(typs[35], typs[55]), params(typs[30]))
	typs[57] = newSig(params(typs[35], typs[30]), params(typs[42]))
	typs[58] = types.NewArray(typs[54], 32)
	typs[59] = types.NewPtr(typs[58])
	typs[60] = newSig(params(typs[59], typs[30]), params(typs[55]))
	typs[61] = newSig(params(typs[3], typs[13], typs[3], typs[13], typs[5]), params(typs[13]))
	typs[62] = newSig(params(typs[30], typs[13]), params(typs[54], typs[13]))
	typs[63] = newSig(params(typs[30]), params(typs[13]))
	typs[64] = newSig(params(typs[1], typs[3]), params(typs[7]))
	typs[65] = types.Types[types.TUINT16]
	typs[66] = newSig(params(typs[65]), params(typs[7]))
	typs[67] = types.Types[types.TUINT32]
	typs[68] = newSig(params(typs[67]), params(typs[7]))
	typs[69] = newSig(params(typs[24]), params(typs[7]))
	typs[70] = newSig(params(typs[30]), params(typs[7]))
	typs[71] = types.Types[types.TUINT8]
	typs[72] = types.NewSlice(typs[71])
	typs[73] = newSig(params(typs[72]), params(typs[7]))
	typs[74] = newSig(params(typs[1], typs[1]), params(typs[1]))
	typs[75] = newSig(params(typs[1], typs[1], typs[1]), nil)
	typs[76] = newSig(params(typs[1]), nil)
	typs[77] = newSig(params(typs[1], typs[1]), params(typs[13], typs[1]))
	typs[78] = types.NewPtr(typs[5])
	typs[79] = newSig(params(typs[78], typs[7], typs[7]), params(typs[6]))
	typs[80] = newSig(params(typs[13]), nil)
	typs[81] = newSig(nil, params(typs[24]))
	typs[82] = newSig(nil, params(typs[67]))
	typs[83] = types.NewMap(typs[2], typs[2])
	typs[84] = newSig(params(typs[1], typs[22], typs[3]), params(typs[83]))
	typs[85] = newSig(params(typs[1], typs[13], typs[3]), params(typs[83]))
	typs[86] = newSig(nil, params(typs[83]))
	typs[87] = newSig(params(typs[1], typs[83], typs[3]), params(typs[3]))
	typs[88] = newSig(params(typs[1], typs[83], typs[67]), params(typs[3]))
	typs[89] = newSig(params(typs[1], typs[83], typs[24]), params(typs[3]))
	typs[90] = newSig(params(typs[1], typs[83], typs[30]), params(typs[3]))
	typs[91] = newSig(params(typs[1], typs[83], typs[3], typs[1]), params(typs[3]))
	typs[92] = newSig(params(typs[1], typs[83], typs[3]), params(typs[3], typs[6]))
	typs[93] = newSig(params(typs[1], typs[83], typs[67]), params(typs[3], typs[6]))
	typs[94] = newSig(params(typs[1], typs[83], typs[24]), params(typs[3], typs[6]))
	typs[95] = newSig(params(typs[1], typs[83], typs[30]), params(typs[3], typs[6]))
	typs[96] = newSig(params(typs[1], typs[83], typs[3], typs[1]), params(typs[3], typs[6]))
	typs[97] = newSig(params(typs[1], typs[83], typs[7]), params(typs[3]))
	typs[98] = newSig(params(typs[1], typs[83], typs[3]), nil)
	typs[99] = newSig(params(typs[1], typs[83], typs[67]), nil)
	typs[100] = newSig(params(typs[1], typs[83], typs[24]), nil)
	typs[101] = newSig(params(typs[1], typs[83], typs[30]), nil)
	typs[102] = newSig(params(typs[3]), nil)
	typs[103] = newSig(params(typs[1], typs[83]), nil)
	typs[104] = types.NewChan(typs[2], types.Cboth)
	typs[105] = newSig(params(typs[1], typs[22]), params(typs[104]))
	typs[106] = newSig(params(typs[1], typs[13]), params(typs[104]))
	typs[107] = types.NewChan(typs[2], types.Crecv)
	typs[108] = newSig(params(typs[107], typs[3]), nil)
	typs[109] = newSig(params(typs[107], typs[3]), params(typs[6]))
	typs[110] = types.NewChan(typs[2], types.Csend)
	typs[111] = newSig(params(typs[110], typs[3]), nil)
	typs[112] = newSig(params(typs[110]), nil)
	typs[113] = newSig(params(typs[2]), params(typs[13]))
	typs[114] = types.NewArray(typs[0], 3)
	typs[115] = types.NewStruct([]*types.Field{types.NewField(src.NoXPos, Lookup("enabled"), typs[6]), types.NewField(src.NoXPos, Lookup("pad"), typs[114]), types.NewField(src.NoXPos, Lookup("cgo"), typs[6]), types.NewField(src.NoXPos, Lookup("alignme"), typs[24])})
	typs[116] = newSig(params(typs[1], typs[3], typs[3]), nil)
	typs[117] = newSig(params(typs[1], typs[3]), nil)
	typs[118] = newSig(params(typs[1], typs[3], typs[13], typs[3], typs[13]), params(typs[13]))
	typs[119] = newSig(params(typs[110], typs[3]), params(typs[6]))
	typs[120] = newSig(params(typs[3], typs[107]), params(typs[6], typs[6]))
	typs[121] = newSig(params(typs[78]), nil)
	typs[122] = newSig(params(typs[1], typs[1], typs[78], typs[13], typs[13], typs[6]), params(typs[13], typs[6]))
	typs[123] = newSig(params(typs[1], typs[13], typs[13]), params(typs[7]))
	typs[124] = newSig(params(typs[1], typs[22], typs[22]), params(typs[7]))
	typs[125] = newSig(params(typs[1], typs[13], typs[13], typs[7]), params(typs[7]))
	typs[126] = types.NewSlice(typs[2])
	typs[127] = newSig(params(typs[3], typs[13], typs[13], typs[13], typs[1]), params(typs[126]))
	typs[128] = newSig(params(typs[3], typs[13], typs[13], typs[13], typs[1], typs[3], typs[13]), params(typs[126]))
	typs[129] = newSig(params(typs[1], typs[7], typs[22]), nil)
	typs[130] = newSig(params(typs[7], typs[22]), nil)
	typs[131] = newSig(params(typs[1], typs[1], typs[13], typs[13]), params(typs[1], typs[13], typs[13]))
	typs[132] = newSig(params(typs[5], typs[1], typs[13], typs[13]), params(typs[1], typs[13], typs[13]))
	typs[133] = newSig(params(typs[1], typs[1], typs[13]), params(typs[1], typs[13], typs[13]))
	typs[134] = newSig(params(typs[5], typs[1], typs[13]), params(typs[1], typs[13], typs[13]))
	typs[135] = newSig(params(typs[3], typs[3], typs[5]), nil)
	typs[136] = newSig(params(typs[7], typs[5]), nil)
	typs[137] = newSig(params(typs[7], typs[7], typs[5]), params(typs[6]))
	typs[138] = newSig(params(typs[7], typs[7]), params(typs[6]))
	typs[139] = newSig(params(typs[7], typs[5], typs[5]), params(typs[5]))
	typs[140] = newSig(params(typs[7], typs[5]), params(typs[5]))
	typs[141] = newSig(params(typs[22], typs[22]), params(typs[22]))
	typs[142] = newSig(params(typs[24], typs[24]), params(typs[24]))
	typs[143] = newSig(params(typs[18]), params(typs[22]))
	typs[144] = newSig(params(typs[18]), params(typs[24]))
	typs[145] = newSig(params(typs[18]), params(typs[67]))
	typs[146] = newSig(params(typs[22]), params(typs[18]))
	typs[147] = newSig(params(typs[22]), params(typs[20]))
	typs[148] = newSig(params(typs[24]), params(typs[18]))
	typs[149] = newSig(params(typs[24]), params(typs[20]))
	typs[150] = newSig(params(typs[67]), params(typs[18]))
	typs[151] = newSig(params(typs[26], typs[26]), params(typs[26]))
	typs[152] = newSig(params(typs[5], typs[5]), nil)
	typs[153] = newSig(params(typs[5], typs[5], typs[5]), nil)
	typs[154] = newSig(params(typs[7], typs[1], typs[5]), nil)
	typs[155] = types.NewSlice(typs[7])
	typs[156] = newSig(params(typs[7], typs[155]), nil)
	typs[157] = newSig(params(typs[71], typs[71], typs[15]), nil)
	typs[158] = newSig(params(typs[65], typs[65], typs[15]), nil)
	typs[159] = newSig(params(typs[67], typs[67], typs[15]), nil)
	typs[160] = newSig(params(typs[24], typs[24], typs[15]), nil)
	typs[161] = newSig(params(typs[30], typs[30], typs[15]), nil)
	typs[162] = types.NewArray(typs[0], 16)
	typs[163] = newSig(params(typs[7], typs[67], typs[162], typs[30], typs[13], typs[71], typs[71]), params(typs[67]))
	return typs[:]
}

var coverageDecls = [...]struct {
	name string
	tag  int
	typ  int
}{
	{"initHook", funcTag, 1},
}

func coverageTypes() []*types.Type {
	var typs [2]*types.Type
	typs[0] = types.Types[types.TBOOL]
	typs[1] = newSig(params(typs[0]), nil)
	return typs[:]
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/const.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"fmt"
	"go/constant"
	"go/token"
	"math"
	"math/big"
	"unicode"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
)

func roundFloat(v constant.Value, sz int64) constant.Value {
	switch sz {
	case 4:
		f, _ := constant.Float32Val(v)
		return makeFloat64(float64(f))
	case 8:
		f, _ := constant.Float64Val(v)
		return makeFloat64(f)
	}
	base.Fatalf("unexpected size: %v", sz)
	panic("unreachable")
}

// truncate float literal fv to 32-bit or 64-bit precision
// according to type; return truncated value.
func truncfltlit(v constant.Value, t *types.Type) constant.Value {
	if t.IsUntyped() {
		return v
	}

	return roundFloat(v, t.Size())
}

// truncate Real and Imag parts of Mpcplx to 32-bit or 64-bit
// precision, according to type; return truncated value. In case of
// overflow, calls Errorf but does not truncate the input value.
func trunccmplxlit(v constant.Value, t *types.Type) constant.Value {
	if t.IsUntyped() {
		return v
	}

	fsz := t.Size() / 2
	return makeComplex(roundFloat(constant.Real(v), fsz), roundFloat(constant.Imag(v), fsz))
}

// TODO(mdempsky): Replace these with better APIs.
func convlit(n ir.Node, t *types.Type) ir.Node    { return convlit1(n, t, false, nil) }
func DefaultLit(n ir.Node, t *types.Type) ir.Node { return convlit1(n, t, false, nil) }

// convlit1 converts an untyped expression n to type t. If n already
// has a type, convlit1 has no effect.
//
// For explicit conversions, t must be non-nil, and integer-to-string
// conversions are allowed.
//
// For implicit conversions (e.g., assignments), t may be nil; if so,
// n is converted to its default type.
//
// If there's an error converting n to t, context is used in the error
// message.
func convlit1(n ir.Node, t *types.Type, explicit bool, context func() string) ir.Node {
	if explicit && t == nil {
		base.Fatalf("explicit conversion missing type")
	}
	if t != nil && t.IsUntyped() {
		base.Fatalf("bad conversion to untyped: %v", t)
	}

	if n == nil || n.Type() == nil {
		// Allow sloppy callers.
		return n
	}
	if !n.Type().IsUntyped() {
		// Already typed; nothing to do.
		return n
	}

	// Nil is technically not a constant, so handle it specially.
	if n.Type().Kind() == types.TNIL {
		if n.Op() != ir.ONIL {
			base.Fatalf("unexpected op: %v (%v)", n, n.Op())
		}
		n = ir.Copy(n)
		if t == nil {
			base.Fatalf("use of untyped nil")
		}

		if !t.HasNil() {
			// Leave for caller to handle.
			return n
		}

		n.SetType(t)
		return n
	}

	if t == nil || !ir.OKForConst[t.Kind()] {
		t = defaultType(n.Type())
	}

	switch n.Op() {
	default:
		base.Fatalf("unexpected untyped expression: %v", n)

	case ir.OLITERAL:
		v := ConvertVal(n.Val(), t, explicit)
		if v.Kind() == constant.Unknown {
			n = ir.NewConstExpr(n.Val(), n)
			break
		}
		n = ir.NewConstExpr(v, n)
		n.SetType(t)
		return n

	case ir.OPLUS, ir.ONEG, ir.OBITNOT, ir.ONOT, ir.OREAL, ir.OIMAG:
		ot := operandType(n.Op(), t)
		if ot == nil {
			n = DefaultLit(n, nil)
			break
		}

		n := n.(*ir.UnaryExpr)
		n.X = convlit(n.X, ot)
		if n.X.Type() == nil {
			n.SetType(nil)
			return n
		}
		n.SetType(t)
		return n

	case ir.OADD, ir.OSUB, ir.OMUL, ir.ODIV, ir.OMOD, ir.OOR, ir.OXOR, ir.OAND, ir.OANDNOT, ir.OOROR, ir.OANDAND, ir.OCOMPLEX:
		ot := operandType(n.Op(), t)
		if ot == nil {
			n = DefaultLit(n, nil)
			break
		}

		var l, r ir.Node
		switch n := n.(type) {
		case *ir.BinaryExpr:
			n.X = convlit(n.X, ot)
			n.Y = convlit(n.Y, ot)
			l, r = n.X, n.Y
		case *ir.LogicalExpr:
			n.X = convlit(n.X, ot)
			n.Y = convlit(n.Y, ot)
			l, r = n.X, n.Y
		}

		if l.Type() == nil || r.Type() == nil {
			n.SetType(nil)
			return n
		}
		if !types.Identical(l.Type(), r.Type()) {
			base.Errorf("invalid operation: %v (mismatched types %v and %v)", n, l.Type(), r.Type())
			n.SetType(nil)
			return n
		}

		n.SetType(t)
		return n

	case ir.OEQ, ir.ONE, ir.OLT, ir.OLE, ir.OGT, ir.OGE:
		n := n.(*ir.BinaryExpr)
		if !t.IsBoolean() {
			break
		}
		n.SetType(t)
		return n

	case ir.OLSH, ir.ORSH:
		n := n.(*ir.BinaryExpr)
		n.X = convlit1(n.X, t, explicit, nil)
		n.SetType(n.X.Type())
		if n.Type() != nil && !n.Type().IsInteger() {
			base.Errorf("invalid operation: %v (shift of type %v)", n, n.Type())
			n.SetType(nil)
		}
		return n
	}

	if explicit {
		base.Fatalf("cannot convert %L to type %v", n, t)
	} else if context != nil {
		base.Fatalf("cannot use %L as type %v in %s", n, t, context())
	} else {
		base.Fatalf("cannot use %L as type %v", n, t)
	}

	n.SetType(nil)
	return n
}

func operandType(op ir.Op, t *types.Type) *types.Type {
	switch op {
	case ir.OCOMPLEX:
		if t.IsComplex() {
			return types.FloatForComplex(t)
		}
	case ir.OREAL, ir.OIMAG:
		if t.IsFloat() {
			return types.ComplexForFloat(t)
		}
	default:
		if okfor[op][t.Kind()] {
			return t
		}
	}
	return nil
}

// ConvertVal converts v into a representation appropriate for t. If
// no such representation exists, it returns constant.MakeUnknown()
// instead.
//
// If explicit is true, then conversions from integer to string are
// also allowed.
func ConvertVal(v constant.Value, t *types.Type, explicit bool) constant.Value {
	switch ct := v.Kind(); ct {
	case constant.Bool:
		if t.IsBoolean() {
			return v
		}

	case constant.String:
		if t.IsString() {
			return v
		}

	case constant.Int:
		if explicit && t.IsString() {
			return tostr(v)
		}
		fallthrough
	case constant.Float, constant.Complex:
		switch {
		case t.IsInteger():
			v = toint(v)
			return v
		case t.IsFloat():
			v = toflt(v)
			v = truncfltlit(v, t)
			return v
		case t.IsComplex():
			v = tocplx(v)
			v = trunccmplxlit(v, t)
			return v
		}
	}

	return constant.MakeUnknown()
}

func tocplx(v constant.Value) constant.Value {
	return constant.ToComplex(v)
}

func toflt(v constant.Value) constant.Value {
	if v.Kind() == constant.Complex {
		v = constant.Real(v)
	}

	return constant.ToFloat(v)
}

func toint(v constant.Value) constant.Value {
	if v.Kind() == constant.Complex {
		v = constant.Real(v)
	}

	if v := constant.ToInt(v); v.Kind() == constant.Int {
		return v
	}

	// The value of v cannot be represented as an integer;
	// so we need to print an error message.
	// Unfortunately some float values cannot be
	// reasonably formatted for inclusion in an error
	// message (example: 1 + 1e-100), so first we try to
	// format the float; if the truncation resulted in
	// something that looks like an integer we omit the
	// value from the error message.
	// (See issue #11371).
	f := ir.BigFloat(v)
	if f.MantExp(nil) > 2*ir.ConstPrec {
		base.Errorf("integer too large")
	} else {
		var t big.Float
		t.Parse(fmt.Sprint(v), 0)
		if t.IsInt() {
			base.Errorf("constant truncated to integer")
		} else {
			base.Errorf("constant %v truncated to integer", v)
		}
	}

	// Prevent follow-on errors.
	return constant.MakeUnknown()
}

func tostr(v constant.Value) constant.Value {
	if v.Kind() == constant.Int {
		r := unicode.ReplacementChar
		if x, ok := constant.Uint64Val(v); ok && x <= unicode.MaxRune {
			r = rune(x)
		}
		v = constant.MakeString(string(r))
	}
	return v
}

func makeFloat64(f float64) constant.Value {
	if math.IsInf(f, 0) {
		base.Fatalf("infinity is not a valid constant")
	}
	return constant.MakeFloat64(f)
}

func makeComplex(real, imag constant.Value) constant.Value {
	return constant.BinaryOp(constant.ToFloat(real), token.ADD, constant.MakeImag(constant.ToFloat(imag)))
}

// DefaultLit on both nodes simultaneously;
// if they're both ideal going in they better
// get the same type going out.
// force means must assign concrete (non-ideal) type.
// The results of defaultlit2 MUST be assigned back to l and r, e.g.
//
//	n.Left, n.Right = defaultlit2(n.Left, n.Right, force)
func defaultlit2(l ir.Node, r ir.Node, force bool) (ir.Node, ir.Node) {
	if l.Type() == nil || r.Type() == nil {
		return l, r
	}

	if !l.Type().IsInterface() && !r.Type().IsInterface() {
		// Can't mix bool with non-bool, string with non-string.
		if l.Type().IsBoolean() != r.Type().IsBoolean() {
			return l, r
		}
		if l.Type().IsString() != r.Type().IsString() {
			return l, r
		}
	}

	if !l.Type().IsUntyped() {
		r = convlit(r, l.Type())
		return l, r
	}

	if !r.Type().IsUntyped() {
		l = convlit(l, r.Type())
		return l, r
	}

	if !force {
		return l, r
	}

	// Can't mix nil with anything untyped.
	if ir.IsNil(l) || ir.IsNil(r) {
		return l, r
	}
	t := defaultType(mixUntyped(l.Type(), r.Type()))
	l = convlit(l, t)
	r = convlit(r, t)
	return l, r
}

func mixUntyped(t1, t2 *types.Type) *types.Type {
	if t1 == t2 {
		return t1
	}

	rank := func(t *types.Type) int {
		switch t {
		case types.UntypedInt:
			return 0
		case types.UntypedRune:
			return 1
		case types.UntypedFloat:
			return 2
		case types.UntypedComplex:
			return 3
		}
		base.Fatalf("bad type %v", t)
		panic("unreachable")
	}

	if rank(t2) > rank(t1) {
		return t2
	}
	return t1
}

func defaultType(t *types.Type) *types.Type {
	if !t.IsUntyped() || t.Kind() == types.TNIL {
		return t
	}

	switch t {
	case types.UntypedBool:
		return types.Types[types.TBOOL]
	case types.UntypedString:
		return types.Types[types.TSTRING]
	case types.UntypedInt:
		return types.Types[types.TINT]
	case types.UntypedRune:
		return types.RuneType
	case types.UntypedFloat:
		return types.Types[types.TFLOAT64]
	case types.UntypedComplex:
		return types.Types[types.TCOMPLEX128]
	}

	base.Fatalf("bad type %v", t)
	return nil
}

// IndexConst returns the index value of constant Node n.
func IndexConst(n ir.Node) int64 {
	return ir.IntVal(types.Types[types.TINT], toint(n.Val()))
}

// callOrChan reports whether n is a call or channel operation.
func callOrChan(n ir.Node) bool {
	switch n.Op() {
	case ir.OAPPEND,
		ir.OCALL,
		ir.OCALLFUNC,
		ir.OCALLINTER,
		ir.OCALLMETH,
		ir.OCAP,
		ir.OCLEAR,
		ir.OCLOSE,
		ir.OCOMPLEX,
		ir.OCOPY,
		ir.ODELETE,
		ir.OIMAG,
		ir.OLEN,
		ir.OMAKE,
		ir.OMAX,
		ir.OMIN,
		ir.ONEW,
		ir.OPANIC,
		ir.OPRINT,
		ir.OPRINTLN,
		ir.OREAL,
		ir.ORECOVER,
		ir.ORECV,
		ir.OUNSAFEADD,
		ir.OUNSAFESLICE,
		ir.OUNSAFESLICEDATA,
		ir.OUNSAFESTRING,
		ir.OUNSAFESTRINGDATA:
		return true
	}
	return false
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/dcl.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"fmt"
	"sync"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

var funcStack []*ir.Func // stack of previous values of ir.CurFunc

// DeclFunc declares the parameters for fn and adds it to
// Target.Funcs.
//
// Before returning, it sets CurFunc to fn. When the caller is done
// constructing fn, it must call FinishFuncBody to restore CurFunc.
func DeclFunc(fn *ir.Func) {
	fn.DeclareParams(true)
	fn.Nname.Defn = fn
	Target.Funcs = append(Target.Funcs, fn)

	funcStack = append(funcStack, ir.CurFunc)
	ir.CurFunc = fn
}

// FinishFuncBody restores ir.CurFunc to its state before the last
// call to DeclFunc.
func FinishFuncBody() {
	funcStack, ir.CurFunc = funcStack[:len(funcStack)-1], funcStack[len(funcStack)-1]
}

func CheckFuncStack() {
	if len(funcStack) != 0 {
		base.Fatalf("funcStack is non-empty: %v", len(funcStack))
	}
}

// TempAt makes a new Node off the books.
//
// N.B., the new Node is a function-local variable defaulting to function scope.
// It helps in some cases if an ODCL is also created and placed in a narrower scope,
// such as if the variable can be used in a loop body and potentially escape.
// TODO: Consider some mechanism to more conveniently create a block scoped temporary.
func TempAt(pos src.XPos, curfn *ir.Func, typ *types.Type) *ir.Name {
	if curfn == nil {
		base.FatalfAt(pos, "no curfn for TempAt")
	}
	if typ == nil {
		base.FatalfAt(pos, "TempAt called with nil type")
	}
	if typ.Kind() == types.TFUNC && typ.Recv() != nil {
		base.FatalfAt(pos, "misuse of method type: %v", typ)
	}
	types.CalcSize(typ)

	sym := &types.Sym{
		Name: autotmpname(len(curfn.Dcl)),
		Pkg:  types.LocalPkg,
	}
	name := curfn.NewLocal(pos, sym, typ)
	name.SetEsc(ir.EscNever)
	name.SetUsed(true)
	name.SetAutoTemp(true)

	return name
}

var (
	autotmpnamesmu sync.Mutex
	autotmpnames   []string
)

// autotmpname returns the name for an autotmp variable numbered n.
func autotmpname(n int) string {
	autotmpnamesmu.Lock()
	defer autotmpnamesmu.Unlock()

	// Grow autotmpnames, if needed.
	if n >= len(autotmpnames) {
		autotmpnames = append(autotmpnames, make([]string, n+1-len(autotmpnames))...)
		autotmpnames = autotmpnames[:cap(autotmpnames)]
	}

	s := autotmpnames[n]
	if s == "" {
		// Give each tmp a different name so that they can be registerized.
		// Add a preceding . to avoid clashing with legal names.
		prefix := ".autotmp_%d"

		s = fmt.Sprintf(prefix, n)
		autotmpnames[n] = s
	}
	return s
}

// f is method type, with receiver.
// return function type, receiver as first argument (or not).
func NewMethodType(sig *types.Type, recv *types.Type) *types.Type {
	nrecvs := 0
	if recv != nil {
		nrecvs++
	}

	// TODO(mdempsky): Move this function to types.
	// TODO(mdempsky): Preserve positions, names, and package from sig+recv.

	params := make([]*types.Field, nrecvs+sig.NumParams())
	if recv != nil {
		params[0] = types.NewField(base.Pos, nil, recv)
	}
	for i, param := range sig.Params() {
		d := types.NewField(base.Pos, nil, param.Type)
		d.SetIsDDD(param.IsDDD())
		params[nrecvs+i] = d
	}

	results := make([]*types.Field, sig.NumResults())
	for i, t := range sig.Results() {
		results[i] = types.NewField(base.Pos, nil, t.Type)
	}

	return types.NewSignature(nil, params, results)
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/export.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

// importfunc declares symbol s as an imported function with type t.
func importfunc(s *types.Sym, t *types.Type) {
	fn := ir.NewFunc(src.NoXPos, src.NoXPos, s, t)
	importsym(fn.Nname)
}

// importvar declares symbol s as an imported variable with type t.
func importvar(s *types.Sym, t *types.Type) {
	n := ir.NewNameAt(src.NoXPos, s, t)
	n.Class = ir.PEXTERN
	importsym(n)
}

func importsym(name *ir.Name) {
	sym := name.Sym()
	if sym.Def != nil {
		base.Fatalf("importsym of symbol that already exists: %v", sym.Def)
	}
	sym.Def = name
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/expr.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"fmt"
	"internal/types/errors"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

func tcShift(n, l, r ir.Node) (ir.Node, ir.Node, *types.Type) {
	if l.Type() == nil || r.Type() == nil {
		return l, r, nil
	}

	r = DefaultLit(r, types.Types[types.TUINT])
	t := r.Type()
	if !t.IsInteger() {
		base.Errorf("invalid operation: %v (shift count type %v, must be integer)", n, r.Type())
		return l, r, nil
	}
	t = l.Type()
	if t != nil && t.Kind() != types.TIDEAL && !t.IsInteger() {
		base.Errorf("invalid operation: %v (shift of type %v)", n, t)
		return l, r, nil
	}

	// no DefaultLit for left
	// the outer context gives the type
	t = l.Type()
	if (l.Type() == types.UntypedFloat || l.Type() == types.UntypedComplex) && r.Op() == ir.OLITERAL {
		t = types.UntypedInt
	}
	return l, r, t
}

// tcArith typechecks operands of a binary arithmetic expression.
// The result of tcArith MUST be assigned back to original operands,
// t is the type of the expression, and should be set by the caller. e.g:
//
//	n.X, n.Y, t = tcArith(n, op, n.X, n.Y)
//	n.SetType(t)
func tcArith(n ir.Node, op ir.Op, l, r ir.Node) (ir.Node, ir.Node, *types.Type) {
	l, r = defaultlit2(l, r, false)
	if l.Type() == nil || r.Type() == nil {
		return l, r, nil
	}
	t := l.Type()
	if t.Kind() == types.TIDEAL {
		t = r.Type()
	}
	aop := ir.OXXX
	if n.Op().IsCmp() && t.Kind() != types.TIDEAL && !types.Identical(l.Type(), r.Type()) {
		// comparison is okay as long as one side is
		// assignable to the other.  convert so they have
		// the same type.
		//
		// the only conversion that isn't a no-op is concrete == interface.
		// in that case, check comparability of the concrete type.
		// The conversion allocates, so only do it if the concrete type is huge.
		converted := false
		if r.Type().Kind() != types.TBLANK {
			aop, _ = assignOp(l.Type(), r.Type())
			if aop != ir.OXXX {
				if r.Type().IsInterface() && !l.Type().IsInterface() && !types.IsComparable(l.Type()) {
					base.Errorf("invalid operation: %v (operator %v not defined on %s)", n, op, typekind(l.Type()))
					return l, r, nil
				}

				types.CalcSize(l.Type())
				if r.Type().IsInterface() == l.Type().IsInterface() || l.Type().Size() >= 1<<16 {
					l = ir.NewConvExpr(base.Pos, aop, r.Type(), l)
					l.SetTypecheck(1)
				}

				t = r.Type()
				converted = true
			}
		}

		if !converted && l.Type().Kind() != types.TBLANK {
			aop, _ = assignOp(r.Type(), l.Type())
			if aop != ir.OXXX {
				if l.Type().IsInterface() && !r.Type().IsInterface() && !types.IsComparable(r.Type()) {
					base.Errorf("invalid operation: %v (operator %v not defined on %s)", n, op, typekind(r.Type()))
					return l, r, nil
				}

				types.CalcSize(r.Type())
				if r.Type().IsInterface() == l.Type().IsInterface() || r.Type().Size() >= 1<<16 {
					r = ir.NewConvExpr(base.Pos, aop, l.Type(), r)
					r.SetTypecheck(1)
				}

				t = l.Type()
			}
		}
	}

	if t.Kind() != types.TIDEAL && !types.Identical(l.Type(), r.Type()) {
		l, r = defaultlit2(l, r, true)
		if l.Type() == nil || r.Type() == nil {
			return l, r, nil
		}
		if l.Type().IsInterface() == r.Type().IsInterface() || aop == 0 {
			base.Errorf("invalid operation: %v (mismatched types %v and %v)", n, l.Type(), r.Type())
			return l, r, nil
		}
	}

	if t.Kind() == types.TIDEAL {
		t = mixUntyped(l.Type(), r.Type())
	}
	if dt := defaultType(t); !okfor[op][dt.Kind()] {
		base.Errorf("invalid operation: %v (operator %v not defined on %s)", n, op, typekind(t))
		return l, r, nil
	}

	// okfor allows any array == array, map == map, func == func.
	// restrict to slice/map/func == nil and nil == slice/map/func.
	if l.Type().IsArray() && !types.IsComparable(l.Type()) {
		base.Errorf("invalid operation: %v (%v cannot be compared)", n, l.Type())
		return l, r, nil
	}

	if l.Type().IsSlice() && !ir.IsNil(l) && !ir.IsNil(r) {
		base.Errorf("invalid operation: %v (slice can only be compared to nil)", n)
		return l, r, nil
	}

	if l.Type().IsMap() && !ir.IsNil(l) && !ir.IsNil(r) {
		base.Errorf("invalid operation: %v (map can only be compared to nil)", n)
		return l, r, nil
	}

	if l.Type().Kind() == types.TFUNC && !ir.IsNil(l) && !ir.IsNil(r) {
		base.Errorf("invalid operation: %v (func can only be compared to nil)", n)
		return l, r, nil
	}

	if l.Type().IsStruct() {
		if f := types.IncomparableField(l.Type()); f != nil {
			base.Errorf("invalid operation: %v (struct containing %v cannot be compared)", n, f.Type)
			return l, r, nil
		}
	}

	return l, r, t
}

// The result of tcCompLit MUST be assigned back to n, e.g.
//
//	n.Left = tcCompLit(n.Left)
func tcCompLit(n *ir.CompLitExpr) (res ir.Node) {
	if base.EnableTrace && base.Flag.LowerT {
		defer tracePrint("tcCompLit", n)(&res)
	}

	lno := base.Pos
	defer func() {
		base.Pos = lno
	}()

	ir.SetPos(n)

	t := n.Type()
	base.AssertfAt(t != nil, n.Pos(), "missing type in composite literal")

	switch t.Kind() {
	default:
		base.Errorf("invalid composite literal type %v", t)
		n.SetType(nil)

	case types.TARRAY:
		typecheckarraylit(t.Elem(), t.NumElem(), n.List, "array literal")
		n.SetOp(ir.OARRAYLIT)

	case types.TSLICE:
		length := typecheckarraylit(t.Elem(), -1, n.List, "slice literal")
		n.SetOp(ir.OSLICELIT)
		n.Len = length

	case types.TMAP:
		for i3, l := range n.List {
			ir.SetPos(l)
			if l.Op() != ir.OKEY {
				n.List[i3] = Expr(l)
				base.Errorf("missing key in map literal")
				continue
			}
			l := l.(*ir.KeyExpr)

			r := l.Key
			r = Expr(r)
			l.Key = AssignConv(r, t.Key(), "map key")

			r = l.Value
			r = Expr(r)
			l.Value = AssignConv(r, t.Elem(), "map value")
		}

		n.SetOp(ir.OMAPLIT)

	case types.TSTRUCT:
		// Need valid field offsets for Xoffset below.
		types.CalcSize(t)

		errored := false
		if len(n.List) != 0 && nokeys(n.List) {
			// simple list of variables
			ls := n.List
			for i, n1 := range ls {
				ir.SetPos(n1)
				n1 = Expr(n1)
				ls[i] = n1
				if i >= t.NumFields() {
					if !errored {
						base.Errorf("too many values in %v", n)
						errored = true
					}
					continue
				}

				f := t.Field(i)
				s := f.Sym

				// Do the test for assigning to unexported fields.
				// But if this is an instantiated function, then
				// the function has already been typechecked. In
				// that case, don't do the test, since it can fail
				// for the closure structs created in
				// walkClosure(), because the instantiated
				// function is compiled as if in the source
				// package of the generic function.
				if !(ir.CurFunc != nil && strings.Contains(ir.CurFunc.Nname.Sym().Name, "[")) {
					if s != nil && !types.IsExported(s.Name) && s.Pkg != types.LocalPkg {
						base.Errorf("implicit assignment of unexported field '%s' in %v literal", s.Name, t)
					}
				}
				// No pushtype allowed here. Must name fields for that.
				n1 = AssignConv(n1, f.Type, "field value")
				ls[i] = ir.NewStructKeyExpr(base.Pos, f, n1)
			}
			if len(ls) < t.NumFields() {
				base.Errorf("too few values in %v", n)
			}
		} else {
			hash := make(map[string]bool)

			// keyed list
			ls := n.List
			for i, n := range ls {
				ir.SetPos(n)

				sk, ok := n.(*ir.StructKeyExpr)
				if !ok {
					kv, ok := n.(*ir.KeyExpr)
					if !ok {
						if !errored {
							base.Errorf("mixture of field:value and value initializers")
							errored = true
						}
						ls[i] = Expr(n)
						continue
					}

					sk = tcStructLitKey(t, kv)
					if sk == nil {
						continue
					}

					fielddup(sk.Sym().Name, hash)
				}

				// No pushtype allowed here. Tried and rejected.
				sk.Value = Expr(sk.Value)
				sk.Value = AssignConv(sk.Value, sk.Field.Type, "field value")
				ls[i] = sk
			}
		}

		n.SetOp(ir.OSTRUCTLIT)
	}

	return n
}

// tcStructLitKey typechecks an OKEY node that appeared within a
// struct literal.
func tcStructLitKey(typ *types.Type, kv *ir.KeyExpr) *ir.StructKeyExpr {
	key := kv.Key

	sym := key.Sym()

	// An OXDOT uses the Sym field to hold
	// the field to the right of the dot,
	// so s will be non-nil, but an OXDOT
	// is never a valid struct literal key.
	if sym == nil || sym.Pkg != types.LocalPkg || key.Op() == ir.OXDOT || sym.IsBlank() {
		base.Errorf("invalid field name %v in struct initializer", key)
		return nil
	}

	if f := Lookdot1(nil, sym, typ, typ.Fields(), 0); f != nil {
		return ir.NewStructKeyExpr(kv.Pos(), f, kv.Value)
	}

	var f *types.Field
	if p, ambig := dotpath(sym, typ, &f, false); p != nil {
		if ambig {
			base.Errorf("ambiguous promoted field '%v' in struct literal of type %v", sym, typ)
			return nil
		}
		if f.IsMethod() {
			base.Errorf("cannot use method '%v' in struct literal of type %v", sym, typ)
			return nil
		}
		return ir.NewStructKeyExpr(kv.Pos(), f, kv.Value)
	}

	if ci := Lookdot1(nil, sym, typ, typ.Fields(), 2); ci != nil { // Case-insensitive lookup.
		if visible(ci.Sym) {
			base.Errorf("unknown field '%v' in struct literal of type %v (but does have %v)", sym, typ, ci.Sym)
		} else if nonexported(sym) && sym.Name == ci.Sym.Name { // Ensure exactness before the suggestion.
			base.Errorf("cannot refer to unexported field '%v' in struct literal of type %v", sym, typ)
		} else {
			base.Errorf("unknown field '%v' in struct literal of type %v", sym, typ)
		}
		return nil
	}

	p, _ := dotpath(sym, typ, &f, true)
	if p == nil || f.IsMethod() {
		base.Errorf("unknown field '%v' in struct literal of type %v", sym, typ)
		return nil
	}

	// dotpath returns the parent embedded types in reverse order.
	var ep []string
	for ei := len(p) - 1; ei >= 0; ei-- {
		ep = append(ep, p[ei].field.Sym.Name)
	}
	ep = append(ep, f.Sym.Name)
	base.Errorf("unknown field '%v' in struct literal of type %v (but does have %v)", sym, typ, strings.Join(ep, "."))
	return nil
}

// tcConv typechecks an OCONV node.
func tcConv(n *ir.ConvExpr) ir.Node {
	types.CheckSize(n.Type()) // ensure width is calculated for backend
	n.X = Expr(n.X)
	n.X = convlit1(n.X, n.Type(), true, nil)
	t := n.X.Type()
	if t == nil || n.Type() == nil {
		n.SetType(nil)
		return n
	}
	op, why := convertOp(n.X.Op() == ir.OLITERAL, t, n.Type())
	if op == ir.OXXX {
		// Due to //go:nointerface, we may be stricter than types2 here (#63333).
		base.ErrorfAt(n.Pos(), errors.InvalidConversion, "cannot convert %L to type %v%s", n.X, n.Type(), why)
		n.SetType(nil)
		return n
	}

	n.SetOp(op)
	switch n.Op() {
	case ir.OCONVNOP:
		if t.Kind() == n.Type().Kind() {
			switch t.Kind() {
			case types.TFLOAT32, types.TFLOAT64, types.TCOMPLEX64, types.TCOMPLEX128:
				// Floating point casts imply rounding and
				// so the conversion must be kept.
				n.SetOp(ir.OCONV)
			}
		}

	// do not convert to []byte literal. See CL 125796.
	// generated code and compiler memory footprint is better without it.
	case ir.OSTR2BYTES:
		// ok

	case ir.OSTR2RUNES:
		if n.X.Op() == ir.OLITERAL {
			return stringtoruneslit(n)
		}

	case ir.OBYTES2STR:
		if t.Elem() != types.ByteType && t.Elem() != types.Types[types.TUINT8] {
			// If t is a slice of a user-defined byte type B (not uint8
			// or byte), then add an extra CONVNOP from []B to []byte, so
			// that the call to slicebytetostring() added in walk will
			// typecheck correctly.
			n.X = ir.NewConvExpr(n.X.Pos(), ir.OCONVNOP, types.NewSlice(types.ByteType), n.X)
			n.X.SetTypecheck(1)
		}

	case ir.ORUNES2STR:
		if t.Elem() != types.RuneType && t.Elem() != types.Types[types.TINT32] {
			// If t is a slice of a user-defined rune type B (not uint32
			// or rune), then add an extra CONVNOP from []B to []rune, so
			// that the call to slicerunetostring() added in walk will
			// typecheck correctly.
			n.X = ir.NewConvExpr(n.X.Pos(), ir.OCONVNOP, types.NewSlice(types.RuneType), n.X)
			n.X.SetTypecheck(1)
		}

	}
	return n
}

// DotField returns a field selector expression that selects the
// index'th field of the given expression, which must be of struct or
// pointer-to-struct type.
func DotField(pos src.XPos, x ir.Node, index int) *ir.SelectorExpr {
	op, typ := ir.ODOT, x.Type()
	if typ.IsPtr() {
		op, typ = ir.ODOTPTR, typ.Elem()
	}
	if !typ.IsStruct() {
		base.FatalfAt(pos, "DotField of non-struct: %L", x)
	}

	// TODO(mdempsky): This is the backend's responsibility.
	types.CalcSize(typ)

	field := typ.Field(index)
	return dot(pos, field.Type, op, x, field)
}

func dot(pos src.XPos, typ *types.Type, op ir.Op, x ir.Node, selection *types.Field) *ir.SelectorExpr {
	n := ir.NewSelectorExpr(pos, op, x, selection.Sym)
	n.Selection = selection
	n.SetType(typ)
	n.SetTypecheck(1)
	return n
}

// XDotField returns an expression representing the field selection
// x.sym. If any implicit field selection are necessary, those are
// inserted too.
func XDotField(pos src.XPos, x ir.Node, sym *types.Sym) *ir.SelectorExpr {
	n := Expr(ir.NewSelectorExpr(pos, ir.OXDOT, x, sym)).(*ir.SelectorExpr)
	if n.Op() != ir.ODOT && n.Op() != ir.ODOTPTR {
		base.FatalfAt(pos, "unexpected result op: %v (%v)", n.Op(), n)
	}
	return n
}

// XDotMethod returns an expression representing the method value
// x.sym (i.e., x is a value, not a type). If any implicit field
// selection are necessary, those are inserted too.
//
// If callee is true, the result is an ODOTMETH/ODOTINTER, otherwise
// an OMETHVALUE.
func XDotMethod(pos src.XPos, x ir.Node, sym *types.Sym, callee bool) *ir.SelectorExpr {
	n := ir.NewSelectorExpr(pos, ir.OXDOT, x, sym)
	if callee {
		n = Callee(n).(*ir.SelectorExpr)
		if n.Op() != ir.ODOTMETH && n.Op() != ir.ODOTINTER {
			base.FatalfAt(pos, "unexpected result op: %v (%v)", n.Op(), n)
		}
	} else {
		n = Expr(n).(*ir.SelectorExpr)
		if n.Op() != ir.OMETHVALUE {
			base.FatalfAt(pos, "unexpected result op: %v (%v)", n.Op(), n)
		}
	}
	return n
}

// tcDot typechecks an OXDOT or ODOT node.
func tcDot(n *ir.SelectorExpr, top int) ir.Node {
	if n.Op() == ir.OXDOT {
		n = AddImplicitDots(n)
		n.SetOp(ir.ODOT)
		if n.X == nil {
			n.SetType(nil)
			return n
		}
	}

	n.X = Expr(n.X)
	n.X = DefaultLit(n.X, nil)

	t := n.X.Type()
	if t == nil {
		base.UpdateErrorDot(ir.Line(n), fmt.Sprint(n.X), fmt.Sprint(n))
		n.SetType(nil)
		return n
	}

	if n.X.Op() == ir.OTYPE {
		base.FatalfAt(n.Pos(), "use NewMethodExpr to construct OMETHEXPR")
	}

	if t.IsPtr() && !t.Elem().IsInterface() {
		t = t.Elem()
		if t == nil {
			n.SetType(nil)
			return n
		}
		n.SetOp(ir.ODOTPTR)
		types.CheckSize(t)
	}

	if n.Sel.IsBlank() {
		base.Errorf("cannot refer to blank field or method")
		n.SetType(nil)
		return n
	}

	if Lookdot(n, t, 0) == nil {
		// Legitimate field or method lookup failed, try to explain the error
		switch {
		case t.IsEmptyInterface():
			base.Errorf("%v undefined (type %v is interface with no methods)", n, n.X.Type())

		case t.IsPtr() && t.Elem().IsInterface():
			// Pointer to interface is almost always a mistake.
			base.Errorf("%v undefined (type %v is pointer to interface, not interface)", n, n.X.Type())

		case Lookdot(n, t, 1) != nil:
			// Field or method matches by name, but it is not exported.
			base.Errorf("%v undefined (cannot refer to unexported field or method %v)", n, n.Sel)

		default:
			if mt := Lookdot(n, t, 2); mt != nil && visible(mt.Sym) { // Case-insensitive lookup.
				base.Errorf("%v undefined (type %v has no field or method %v, but does have %v)", n, n.X.Type(), n.Sel, mt.Sym)
			} else {
				base.Errorf("%v undefined (type %v has no field or method %v)", n, n.X.Type(), n.Sel)
			}
		}
		n.SetType(nil)
		return n
	}

	if (n.Op() == ir.ODOTINTER || n.Op() == ir.ODOTMETH) && top&ctxCallee == 0 {
		n.SetOp(ir.OMETHVALUE)
		n.SetType(NewMethodType(n.Type(), nil))
	}
	return n
}

// tcDotType typechecks an ODOTTYPE node.
func tcDotType(n *ir.TypeAssertExpr) ir.Node {
	n.X = Expr(n.X)
	n.X = DefaultLit(n.X, nil)
	l := n.X
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}
	if !t.IsInterface() {
		base.Errorf("invalid type assertion: %v (non-interface type %v on left)", n, t)
		n.SetType(nil)
		return n
	}

	base.AssertfAt(n.Type() != nil, n.Pos(), "missing type: %v", n)

	if n.Type() != nil && !n.Type().IsInterface() {
		why := ImplementsExplain(n.Type(), t)
		if why != "" {
			base.Fatalf("impossible type assertion:\n\t%s", why)
			n.SetType(nil)
			return n
		}
	}
	return n
}

// tcITab typechecks an OITAB node.
func tcITab(n *ir.UnaryExpr) ir.Node {
	n.X = Expr(n.X)
	t := n.X.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}
	if !t.IsInterface() {
		base.Fatalf("OITAB of %v", t)
	}
	n.SetType(types.NewPtr(types.Types[types.TUINTPTR]))
	return n
}

// tcIndex typechecks an OINDEX node.
func tcIndex(n *ir.IndexExpr) ir.Node {
	n.X = Expr(n.X)
	n.X = DefaultLit(n.X, nil)
	n.X = implicitstar(n.X)
	l := n.X
	n.Index = Expr(n.Index)
	r := n.Index
	t := l.Type()
	if t == nil || r.Type() == nil {
		n.SetType(nil)
		return n
	}
	switch t.Kind() {
	default:
		base.Errorf("invalid operation: %v (type %v does not support indexing)", n, t)
		n.SetType(nil)
		return n

	case types.TSTRING, types.TARRAY, types.TSLICE:
		n.Index = indexlit(n.Index)
		if t.IsString() {
			n.SetType(types.ByteType)
		} else {
			n.SetType(t.Elem())
		}
		why := "string"
		if t.IsArray() {
			why = "array"
		} else if t.IsSlice() {
			why = "slice"
		}

		if n.Index.Type() != nil && !n.Index.Type().IsInteger() {
			base.Errorf("non-integer %s index %v", why, n.Index)
			return n
		}

	case types.TMAP:
		n.Index = AssignConv(n.Index, t.Key(), "map index")
		n.SetType(t.Elem())
		n.SetOp(ir.OINDEXMAP)
		n.Assigned = false
	}
	return n
}

// tcLenCap typechecks an OLEN or OCAP node.
func tcLenCap(n *ir.UnaryExpr) ir.Node {
	n.X = Expr(n.X)
	n.X = DefaultLit(n.X, nil)
	l := n.X
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}
	var ok bool
	if t.IsPtr() && t.Elem().IsArray() {
		ok = true
	} else if n.Op() == ir.OLEN {
		ok = okforlen[t.Kind()]
	} else {
		ok = okforcap[t.Kind()]
	}
	if !ok {
		base.Errorf("invalid argument %L for %v", l, n.Op())
		n.SetType(nil)
		return n
	}

	n.SetType(types.Types[types.TINT])
	return n
}

// tcUnsafeData typechecks an OUNSAFESLICEDATA or OUNSAFESTRINGDATA node.
func tcUnsafeData(n *ir.UnaryExpr) ir.Node {
	n.X = Expr(n.X)
	n.X = DefaultLit(n.X, nil)
	l := n.X
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}

	var kind types.Kind
	if n.Op() == ir.OUNSAFESLICEDATA {
		kind = types.TSLICE
	} else {
		/* kind is string */
		kind = types.TSTRING
	}

	if t.Kind() != kind {
		base.Errorf("invalid argument %L for %v", l, n.Op())
		n.SetType(nil)
		return n
	}

	if kind == types.TSTRING {
		t = types.ByteType
	} else {
		t = t.Elem()
	}
	n.SetType(types.NewPtr(t))
	return n
}

// tcRecv typechecks an ORECV node.
func tcRecv(n *ir.UnaryExpr) ir.Node {
	n.X = Expr(n.X)
	n.X = DefaultLit(n.X, nil)
	l := n.X
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}
	if !t.IsChan() {
		base.Errorf("invalid operation: %v (receive from non-chan type %v)", n, t)
		n.SetType(nil)
		return n
	}

	if !t.ChanDir().CanRecv() {
		base.Errorf("invalid operation: %v (receive from send-only type %v)", n, t)
		n.SetType(nil)
		return n
	}

	n.SetType(t.Elem())
	return n
}

// tcSPtr typechecks an OSPTR node.
func tcSPtr(n *ir.UnaryExpr) ir.Node {
	n.X = Expr(n.X)
	t := n.X.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}
	if !t.IsSlice() && !t.IsString() {
		base.Fatalf("OSPTR of %v", t)
	}
	if t.IsString() {
		n.SetType(types.NewPtr(types.Types[types.TUINT8]))
	} else {
		n.SetType(types.NewPtr(t.Elem()))
	}
	return n
}

// tcSlice typechecks an OSLICE or OSLICE3 node.
func tcSlice(n *ir.SliceExpr) ir.Node {
	n.X = DefaultLit(Expr(n.X), nil)
	n.Low = indexlit(Expr(n.Low))
	n.High = indexlit(Expr(n.High))
	n.Max = indexlit(Expr(n.Max))
	hasmax := n.Op().IsSlice3()
	l := n.X
	if l.Type() == nil {
		n.SetType(nil)
		return n
	}
	if l.Type().IsArray() {
		if !ir.IsAddressable(n.X) {
			base.Errorf("invalid operation %v (slice of unaddressable value)", n)
			n.SetType(nil)
			return n
		}

		addr := NodAddr(n.X)
		addr.SetImplicit(true)
		n.X = Expr(addr)
		l = n.X
	}
	t := l.Type()
	var tp *types.Type
	if t.IsString() {
		if hasmax {
			base.Errorf("invalid operation %v (3-index slice of string)", n)
			n.SetType(nil)
			return n
		}
		n.SetType(t)
		n.SetOp(ir.OSLICESTR)
	} else if t.IsPtr() && t.Elem().IsArray() {
		tp = t.Elem()
		n.SetType(types.NewSlice(tp.Elem()))
		types.CalcSize(n.Type())
		if hasmax {
			n.SetOp(ir.OSLICE3ARR)
		} else {
			n.SetOp(ir.OSLICEARR)
		}
	} else if t.IsSlice() {
		n.SetType(t)
	} else {
		base.Errorf("cannot slice %v (type %v)", l, t)
		n.SetType(nil)
		return n
	}

	if n.Low != nil && !checksliceindex(n.Low) {
		n.SetType(nil)
		return n
	}
	if n.High != nil && !checksliceindex(n.High) {
		n.SetType(nil)
		return n
	}
	if n.Max != nil && !checksliceindex(n.Max) {
		n.SetType(nil)
		return n
	}
	return n
}

// tcSliceHeader typechecks an OSLICEHEADER node.
func tcSliceHeader(n *ir.SliceHeaderExpr) ir.Node {
	// Errors here are Fatalf instead of Errorf because only the compiler
	// can construct an OSLICEHEADER node.
	// Components used in OSLICEHEADER that are supplied by parsed source code
	// have already been typechecked in e.g. OMAKESLICE earlier.
	t := n.Type()
	if t == nil {
		base.Fatalf("no type specified for OSLICEHEADER")
	}

	if !t.IsSlice() {
		base.Fatalf("invalid type %v for OSLICEHEADER", n.Type())
	}

	if n.Ptr == nil || n.Ptr.Type() == nil || !n.Ptr.Type().IsUnsafePtr() {
		base.Fatalf("need unsafe.Pointer for OSLICEHEADER")
	}

	n.Ptr = Expr(n.Ptr)
	n.Len = DefaultLit(Expr(n.Len), types.Types[types.TINT])
	n.Cap = DefaultLit(Expr(n.Cap), types.Types[types.TINT])

	return n
}

// tcStringHeader typechecks an OSTRINGHEADER node.
func tcStringHeader(n *ir.StringHeaderExpr) ir.Node {
	t := n.Type()
	if t == nil {
		base.Fatalf("no type specified for OSTRINGHEADER")
	}

	if !t.IsString() {
		base.Fatalf("invalid type %v for OSTRINGHEADER", n.Type())
	}

	if n.Ptr == nil || n.Ptr.Type() == nil || !n.Ptr.Type().IsUnsafePtr() {
		base.Fatalf("need unsafe.Pointer for OSTRINGHEADER")
	}

	n.Ptr = Expr(n.Ptr)
	n.Len = DefaultLit(Expr(n.Len), types.Types[types.TINT])

	return n
}

// tcStar typechecks an ODEREF node, which may be an expression or a type.
func tcStar(n *ir.StarExpr, top int) ir.Node {
	n.X = typecheck(n.X, ctxExpr|ctxType)
	l := n.X
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}

	// TODO(mdempsky): Remove (along with ctxType above) once I'm
	// confident this code path isn't needed any more.
	if l.Op() == ir.OTYPE {
		base.Fatalf("unexpected type in deref expression: %v", l)
	}

	if !t.IsPtr() {
		if top&(ctxExpr|ctxStmt) != 0 {
			base.Errorf("invalid indirect of %L", n.X)
			n.SetType(nil)
			return n
		}
		base.Errorf("%v is not a type", l)
		return n
	}

	n.SetType(t.Elem())
	return n
}

// tcUnaryArith typechecks a unary arithmetic expression.
func tcUnaryArith(n *ir.UnaryExpr) ir.Node {
	n.X = Expr(n.X)
	l := n.X
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}
	if !okfor[n.Op()][defaultType(t).Kind()] {
		base.Errorf("invalid operation: %v (operator %v not defined on %s)", n, n.Op(), typekind(t))
		n.SetType(nil)
		return n
	}

	n.SetType(t)
	return n
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/func.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/src"

	"fmt"
	"go/constant"
)

// MakeDotArgs package all the arguments that match a ... T parameter into a []T.
func MakeDotArgs(pos src.XPos, typ *types.Type, args []ir.Node) ir.Node {
	if len(args) == 0 {
		return ir.NewNilExpr(pos, typ)
	}

	args = append([]ir.Node(nil), args...)
	lit := ir.NewCompLitExpr(pos, ir.OCOMPLIT, typ, args)
	lit.SetImplicit(true)

	n := Expr(lit)
	if n.Type() == nil {
		base.FatalfAt(pos, "mkdotargslice: typecheck failed")
	}
	return n
}

// FixVariadicCall rewrites calls to variadic functions to use an
// explicit ... argument if one is not already present.
func FixVariadicCall(call *ir.CallExpr) {
	fntype := call.Fun.Type()
	if !fntype.IsVariadic() || call.IsDDD {
		return
	}

	vi := fntype.NumParams() - 1
	vt := fntype.Param(vi).Type

	args := call.Args
	extra := args[vi:]
	slice := MakeDotArgs(call.Pos(), vt, extra)
	for i := range extra {
		extra[i] = nil // allow GC
	}

	call.Args = append(args[:vi], slice)
	call.IsDDD = true
}

// FixMethodCall rewrites a method call t.M(...) into a function call T.M(t, ...).
func FixMethodCall(call *ir.CallExpr) {
	if call.Fun.Op() != ir.ODOTMETH {
		return
	}

	dot := call.Fun.(*ir.SelectorExpr)

	fn := NewMethodExpr(dot.Pos(), dot.X.Type(), dot.Selection.Sym)

	args := make([]ir.Node, 1+len(call.Args))
	args[0] = dot.X
	copy(args[1:], call.Args)

	call.SetOp(ir.OCALLFUNC)
	call.Fun = fn
	call.Args = args
}

func AssertFixedCall(call *ir.CallExpr) {
	if call.Fun.Type().IsVariadic() && !call.IsDDD {
		base.FatalfAt(call.Pos(), "missed FixVariadicCall")
	}
	if call.Op() == ir.OCALLMETH {
		base.FatalfAt(call.Pos(), "missed FixMethodCall")
	}
}

// ClosureType returns the struct type used to hold all the information
// needed in the closure for clo (clo must be a OCLOSURE node).
// The address of a variable of the returned type can be cast to a func.
func ClosureType(clo *ir.ClosureExpr) *types.Type {
	// Create closure in the form of a composite literal.
	// supposing the closure captures an int i and a string s
	// and has one float64 argument and no results,
	// the generated code looks like:
	//
	//	clos = &struct{F uintptr; X0 *int; X1 *string}{func.1, &i, &s}
	//
	// The use of the struct provides type information to the garbage
	// collector so that it can walk the closure. We could use (in this
	// case) [3]unsafe.Pointer instead, but that would leave the gc in
	// the dark. The information appears in the binary in the form of
	// type descriptors; the struct is unnamed and uses exported field
	// names so that closures in multiple packages with the same struct
	// type can share the descriptor.

	fields := make([]*types.Field, 1+len(clo.Func.ClosureVars))
	fields[0] = types.NewField(base.AutogeneratedPos, types.LocalPkg.Lookup("F"), types.Types[types.TUINTPTR])
	it := NewClosureStructIter(clo.Func.ClosureVars)
	i := 0
	for {
		n, typ, _ := it.Next()
		if n == nil {
			break
		}
		fields[1+i] = types.NewField(base.AutogeneratedPos, types.LocalPkg.LookupNum("X", i), typ)
		i++
	}
	typ := types.NewStruct(fields)
	typ.SetNoalg(true)
	return typ
}

// MethodValueType returns the struct type used to hold all the information
// needed in the closure for a OMETHVALUE node. The address of a variable of
// the returned type can be cast to a func.
func MethodValueType(n *ir.SelectorExpr) *types.Type {
	t := types.NewStruct([]*types.Field{
		types.NewField(base.Pos, Lookup("F"), types.Types[types.TUINTPTR]),
		types.NewField(base.Pos, Lookup("R"), n.X.Type()),
	})
	t.SetNoalg(true)
	return t
}

// type check function definition
// To be called by typecheck, not directly.
// (Call typecheck.Func instead.)
func tcFunc(n *ir.Func) {
	if base.EnableTrace && base.Flag.LowerT {
		defer tracePrint("tcFunc", n)(nil)
	}

	if name := n.Nname; name.Typecheck() == 0 {
		base.AssertfAt(name.Type() != nil, n.Pos(), "missing type: %v", name)
		name.SetTypecheck(1)
	}
}

// tcCall typechecks an OCALL node.
func tcCall(n *ir.CallExpr, top int) ir.Node {
	Stmts(n.Init()) // imported rewritten f(g()) calls (#30907)
	n.Fun = typecheck(n.Fun, ctxExpr|ctxType|ctxCallee)

	l := n.Fun

	if l.Op() == ir.ONAME && l.(*ir.Name).BuiltinOp != 0 {
		l := l.(*ir.Name)
		if n.IsDDD && l.BuiltinOp != ir.OAPPEND {
			base.Errorf("invalid use of ... with builtin %v", l)
		}

		// builtin: OLEN, OCAP, etc.
		switch l.BuiltinOp {
		default:
			base.Fatalf("unknown builtin %v", l)

		case ir.OAPPEND, ir.ODELETE, ir.OMAKE, ir.OMAX, ir.OMIN, ir.OPRINT, ir.OPRINTLN, ir.ORECOVER:
			n.SetOp(l.BuiltinOp)
			n.Fun = nil
			n.SetTypecheck(0) // re-typechecking new op is OK, not a loop
			return typecheck(n, top)

		case ir.OCAP, ir.OCLEAR, ir.OCLOSE, ir.OIMAG, ir.OLEN, ir.OPANIC, ir.OREAL, ir.OUNSAFESTRINGDATA, ir.OUNSAFESLICEDATA:
			typecheckargs(n)
			fallthrough
		case ir.ONEW:
			arg, ok := needOneArg(n, "%v", n.Op())
			if !ok {
				n.SetType(nil)
				return n
			}
			u := ir.NewUnaryExpr(n.Pos(), l.BuiltinOp, arg)
			return typecheck(ir.InitExpr(n.Init(), u), top) // typecheckargs can add to old.Init

		case ir.OCOMPLEX, ir.OCOPY, ir.OUNSAFEADD, ir.OUNSAFESLICE, ir.OUNSAFESTRING:
			typecheckargs(n)
			arg1, arg2, ok := needTwoArgs(n)
			if !ok {
				n.SetType(nil)
				return n
			}
			b := ir.NewBinaryExpr(n.Pos(), l.BuiltinOp, arg1, arg2)
			return typecheck(ir.InitExpr(n.Init(), b), top) // typecheckargs can add to old.Init
		}
		panic("unreachable")
	}

	n.Fun = DefaultLit(n.Fun, nil)
	l = n.Fun
	if l.Op() == ir.OTYPE {
		if n.IsDDD {
			base.Fatalf("invalid use of ... in type conversion to %v", l.Type())
		}

		// pick off before type-checking arguments
		arg, ok := needOneArg(n, "conversion to %v", l.Type())
		if !ok {
			n.SetType(nil)
			return n
		}

		n := ir.NewConvExpr(n.Pos(), ir.OCONV, nil, arg)
		n.SetType(l.Type())
		return tcConv(n)
	}

	RewriteNonNameCall(n)
	typecheckargs(n)
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}
	types.CheckSize(t)

	switch l.Op() {
	case ir.ODOTINTER:
		n.SetOp(ir.OCALLINTER)

	case ir.ODOTMETH:
		l := l.(*ir.SelectorExpr)
		n.SetOp(ir.OCALLMETH)

		// typecheckaste was used here but there wasn't enough
		// information further down the call chain to know if we
		// were testing a method receiver for unexported fields.
		// It isn't necessary, so just do a sanity check.
		tp := t.Recv().Type

		if l.X == nil || !types.Identical(l.X.Type(), tp) {
			base.Fatalf("method receiver")
		}

	default:
		n.SetOp(ir.OCALLFUNC)
		if t.Kind() != types.TFUNC {
			if o := l; o.Name() != nil && types.BuiltinPkg.Lookup(o.Sym().Name).Def != nil {
				// be more specific when the non-function
				// name matches a predeclared function
				base.Errorf("cannot call non-function %L, declared at %s",
					l, base.FmtPos(o.Name().Pos()))
			} else {
				base.Errorf("cannot call non-function %L", l)
			}
			n.SetType(nil)
			return n
		}
	}

	typecheckaste(ir.OCALL, n.Fun, n.IsDDD, t.Params(), n.Args, func() string { return fmt.Sprintf("argument to %v", n.Fun) })
	FixVariadicCall(n)
	FixMethodCall(n)
	if t.NumResults() == 0 {
		return n
	}
	if t.NumResults() == 1 {
		n.SetType(l.Type().Result(0).Type)

		if n.Op() == ir.OCALLFUNC && n.Fun.Op() == ir.ONAME {
			if sym := n.Fun.(*ir.Name).Sym(); types.RuntimeSymName(sym) == "getg" {
				// Emit code for runtime.getg() directly instead of calling function.
				// Most such rewrites (for example the similar one for math.Sqrt) should be done in walk,
				// so that the ordering pass can make sure to preserve the semantics of the original code
				// (in particular, the exact time of the function call) by introducing temporaries.
				// In this case, we know getg() always returns the same result within a given function
				// and we want to avoid the temporaries, so we do the rewrite earlier than is typical.
				n.SetOp(ir.OGETG)
			}
		}
		return n
	}

	// multiple return
	if top&(ctxMultiOK|ctxStmt) == 0 {
		base.Errorf("multiple-value %v() in single-value context", l)
		return n
	}

	n.SetType(l.Type().ResultsTuple())
	return n
}

// tcAppend typechecks an OAPPEND node.
func tcAppend(n *ir.CallExpr) ir.Node {
	typecheckargs(n)
	args := n.Args
	if len(args) == 0 {
		base.Errorf("missing arguments to append")
		n.SetType(nil)
		return n
	}

	t := args[0].Type()
	if t == nil {
		n.SetType(nil)
		return n
	}

	n.SetType(t)
	if !t.IsSlice() {
		if ir.IsNil(args[0]) {
			base.Errorf("first argument to append must be typed slice; have untyped nil")
			n.SetType(nil)
			return n
		}

		base.Errorf("first argument to append must be slice; have %L", t)
		n.SetType(nil)
		return n
	}

	if n.IsDDD {
		if len(args) == 1 {
			base.Errorf("cannot use ... on first argument to append")
			n.SetType(nil)
			return n
		}

		if len(args) != 2 {
			base.Errorf("too many arguments to append")
			n.SetType(nil)
			return n
		}

		// AssignConv is of args[1] not required here, as the
		// types of args[0] and args[1] don't need to match
		// (They will both have an underlying type which are
		// slices of identical base types, or be []byte and string.)
		// See issue 53888.
		return n
	}

	as := args[1:]
	for i, n := range as {
		if n.Type() == nil {
			continue
		}
		as[i] = AssignConv(n, t.Elem(), "append")
		types.CheckSize(as[i].Type()) // ensure width is calculated for backend
	}
	return n
}

// tcClear typechecks an OCLEAR node.
func tcClear(n *ir.UnaryExpr) ir.Node {
	n.X = Expr(n.X)
	n.X = DefaultLit(n.X, nil)
	l := n.X
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}

	switch {
	case t.IsMap(), t.IsSlice():
	default:
		base.Errorf("invalid operation: %v (argument must be a map or slice)", n)
		n.SetType(nil)
		return n
	}

	return n
}

// tcClose typechecks an OCLOSE node.
func tcClose(n *ir.UnaryExpr) ir.Node {
	n.X = Expr(n.X)
	n.X = DefaultLit(n.X, nil)
	l := n.X
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}
	if !t.IsChan() {
		base.Errorf("invalid operation: %v (non-chan type %v)", n, t)
		n.SetType(nil)
		return n
	}

	if !t.ChanDir().CanSend() {
		base.Errorf("invalid operation: %v (cannot close receive-only channel)", n)
		n.SetType(nil)
		return n
	}
	return n
}

// tcComplex typechecks an OCOMPLEX node.
func tcComplex(n *ir.BinaryExpr) ir.Node {
	l := Expr(n.X)
	r := Expr(n.Y)
	if l.Type() == nil || r.Type() == nil {
		n.SetType(nil)
		return n
	}
	l, r = defaultlit2(l, r, false)
	if l.Type() == nil || r.Type() == nil {
		n.SetType(nil)
		return n
	}
	n.X = l
	n.Y = r

	if !types.Identical(l.Type(), r.Type()) {
		base.Errorf("invalid operation: %v (mismatched types %v and %v)", n, l.Type(), r.Type())
		n.SetType(nil)
		return n
	}

	var t *types.Type
	switch l.Type().Kind() {
	default:
		base.Errorf("invalid operation: %v (arguments have type %v, expected floating-point)", n, l.Type())
		n.SetType(nil)
		return n

	case types.TIDEAL:
		t = types.UntypedComplex

	case types.TFLOAT32:
		t = types.Types[types.TCOMPLEX64]

	case types.TFLOAT64:
		t = types.Types[types.TCOMPLEX128]
	}
	n.SetType(t)
	return n
}

// tcCopy typechecks an OCOPY node.
func tcCopy(n *ir.BinaryExpr) ir.Node {
	n.SetType(types.Types[types.TINT])
	n.X = Expr(n.X)
	n.X = DefaultLit(n.X, nil)
	n.Y = Expr(n.Y)
	n.Y = DefaultLit(n.Y, nil)
	if n.X.Type() == nil || n.Y.Type() == nil {
		n.SetType(nil)
		return n
	}

	// copy([]byte, string)
	if n.X.Type().IsSlice() && n.Y.Type().IsString() {
		if types.Identical(n.X.Type().Elem(), types.ByteType) {
			return n
		}
		base.Errorf("arguments to copy have different element types: %L and string", n.X.Type())
		n.SetType(nil)
		return n
	}

	if !n.X.Type().IsSlice() || !n.Y.Type().IsSlice() {
		if !n.X.Type().IsSlice() && !n.Y.Type().IsSlice() {
			base.Errorf("arguments to copy must be slices; have %L, %L", n.X.Type(), n.Y.Type())
		} else if !n.X.Type().IsSlice() {
			base.Errorf("first argument to copy should be slice; have %L", n.X.Type())
		} else {
			base.Errorf("second argument to copy should be slice or string; have %L", n.Y.Type())
		}
		n.SetType(nil)
		return n
	}

	if !types.Identical(n.X.Type().Elem(), n.Y.Type().Elem()) {
		base.Errorf("arguments to copy have different element types: %L and %L", n.X.Type(), n.Y.Type())
		n.SetType(nil)
		return n
	}
	return n
}

// tcDelete typechecks an ODELETE node.
func tcDelete(n *ir.CallExpr) ir.Node {
	typecheckargs(n)
	args := n.Args
	if len(args) == 0 {
		base.Errorf("missing arguments to delete")
		n.SetType(nil)
		return n
	}

	if len(args) == 1 {
		base.Errorf("missing second (key) argument to delete")
		n.SetType(nil)
		return n
	}

	if len(args) != 2 {
		base.Errorf("too many arguments to delete")
		n.SetType(nil)
		return n
	}

	l := args[0]
	r := args[1]
	if l.Type() != nil && !l.Type().IsMap() {
		base.Errorf("first argument to delete must be map; have %L", l.Type())
		n.SetType(nil)
		return n
	}

	args[1] = AssignConv(r, l.Type().Key(), "delete")
	return n
}

// tcMake typechecks an OMAKE node.
func tcMake(n *ir.CallExpr) ir.Node {
	args := n.Args
	if len(args) == 0 {
		base.Errorf("missing argument to make")
		n.SetType(nil)
		return n
	}

	n.Args = nil
	l := args[0]
	l = typecheck(l, ctxType)
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}

	i := 1
	var nn ir.Node
	switch t.Kind() {
	default:
		base.Errorf("cannot make type %v", t)
		n.SetType(nil)
		return n

	case types.TSLICE:
		if i >= len(args) {
			base.Errorf("missing len argument to make(%v)", t)
			n.SetType(nil)
			return n
		}

		l = args[i]
		i++
		l = Expr(l)
		var r ir.Node
		if i < len(args) {
			r = args[i]
			i++
			r = Expr(r)
		}

		if l.Type() == nil || (r != nil && r.Type() == nil) {
			n.SetType(nil)
			return n
		}
		if !checkmake(t, "len", &l) || r != nil && !checkmake(t, "cap", &r) {
			n.SetType(nil)
			return n
		}
		nn = ir.NewMakeExpr(n.Pos(), ir.OMAKESLICE, l, r)

	case types.TMAP:
		if i < len(args) {
			l = args[i]
			i++
			l = Expr(l)
			l = DefaultLit(l, types.Types[types.TINT])
			if l.Type() == nil {
				n.SetType(nil)
				return n
			}
			if !checkmake(t, "size", &l) {
				n.SetType(nil)
				return n
			}
		} else {
			l = ir.NewInt(base.Pos, 0)
		}
		nn = ir.NewMakeExpr(n.Pos(), ir.OMAKEMAP, l, nil)
		nn.SetEsc(n.Esc())

	case types.TCHAN:
		l = nil
		if i < len(args) {
			l = args[i]
			i++
			l = Expr(l)
			l = DefaultLit(l, types.Types[types.TINT])
			if l.Type() == nil {
				n.SetType(nil)
				return n
			}
			if !checkmake(t, "buffer", &l) {
				n.SetType(nil)
				return n
			}
		} else {
			l = ir.NewInt(base.Pos, 0)
		}
		nn = ir.NewMakeExpr(n.Pos(), ir.OMAKECHAN, l, nil)
	}

	if i < len(args) {
		base.Errorf("too many arguments to make(%v)", t)
		n.SetType(nil)
		return n
	}

	nn.SetType(t)
	return nn
}

// tcMakeSliceCopy typechecks an OMAKESLICECOPY node.
func tcMakeSliceCopy(n *ir.MakeExpr) ir.Node {
	// Errors here are Fatalf instead of Errorf because only the compiler
	// can construct an OMAKESLICECOPY node.
	// Components used in OMAKESCLICECOPY that are supplied by parsed source code
	// have already been typechecked in OMAKE and OCOPY earlier.
	t := n.Type()

	if t == nil {
		base.Fatalf("no type specified for OMAKESLICECOPY")
	}

	if !t.IsSlice() {
		base.Fatalf("invalid type %v for OMAKESLICECOPY", n.Type())
	}

	if n.Len == nil {
		base.Fatalf("missing len argument for OMAKESLICECOPY")
	}

	if n.Cap == nil {
		base.Fatalf("missing slice argument to copy for OMAKESLICECOPY")
	}

	n.Len = Expr(n.Len)
	n.Cap = Expr(n.Cap)

	n.Len = DefaultLit(n.Len, types.Types[types.TINT])

	if !n.Len.Type().IsInteger() && n.Type().Kind() != types.TIDEAL {
		base.Errorf("non-integer len argument in OMAKESLICECOPY")
	}

	return n
}

// tcNew typechecks an ONEW node.
func tcNew(n *ir.UnaryExpr) ir.Node {
	if n.X == nil {
		// Fatalf because the OCALL above checked for us,
		// so this must be an internally-generated mistake.
		base.Fatalf("missing argument to new")
	}
	l := n.X
	l = typecheck(l, ctxType)
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}
	n.X = l
	n.SetType(types.NewPtr(t))
	return n
}

// tcPanic typechecks an OPANIC node.
func tcPanic(n *ir.UnaryExpr) ir.Node {
	n.X = Expr(n.X)
	n.X = AssignConv(n.X, types.Types[types.TINTER], "argument to panic")
	if n.X.Type() == nil {
		n.SetType(nil)
		return n
	}
	return n
}

// tcPrint typechecks an OPRINT or OPRINTN node.
func tcPrint(n *ir.CallExpr) ir.Node {
	typecheckargs(n)
	ls := n.Args
	for i1, n1 := range ls {
		// Special case for print: int constant is int64, not int.
		if ir.IsConst(n1, constant.Int) {
			ls[i1] = DefaultLit(ls[i1], types.Types[types.TINT64])
		} else {
			ls[i1] = DefaultLit(ls[i1], nil)
		}
	}
	return n
}

// tcMinMax typechecks an OMIN or OMAX node.
func tcMinMax(n *ir.CallExpr) ir.Node {
	typecheckargs(n)
	arg0 := n.Args[0]
	for _, arg := range n.Args[1:] {
		if !types.Identical(arg.Type(), arg0.Type()) {
			base.FatalfAt(n.Pos(), "mismatched arguments: %L and %L", arg0, arg)
		}
	}
	n.SetType(arg0.Type())
	return n
}

// tcRealImag typechecks an OREAL or OIMAG node.
func tcRealImag(n *ir.UnaryExpr) ir.Node {
	n.X = Expr(n.X)
	l := n.X
	t := l.Type()
	if t == nil {
		n.SetType(nil)
		return n
	}

	// Determine result type.
	switch t.Kind() {
	case types.TIDEAL:
		n.SetType(types.UntypedFloat)
	case types.TCOMPLEX64:
		n.SetType(types.Types[types.TFLOAT32])
	case types.TCOMPLEX128:
		n.SetType(types.Types[types.TFLOAT64])
	default:
		base.Errorf("invalid argument %L for %v", l, n.Op())
		n.SetType(nil)
		return n
	}
	return n
}

// tcRecover typechecks an ORECOVER node.
func tcRecover(n *ir.CallExpr) ir.Node {
	if len(n.Args) != 0 {
		base.Errorf("too many arguments to recover")
		n.SetType(nil)
		return n
	}

	n.SetType(types.Types[types.TINTER])
	return n
}

// tcUnsafeAdd typechecks an OUNSAFEADD node.
func tcUnsafeAdd(n *ir.BinaryExpr) *ir.BinaryExpr {
	n.X = AssignConv(Expr(n.X), types.Types[types.TUNSAFEPTR], "argument to unsafe.Add")
	n.Y = DefaultLit(Expr(n.Y), types.Types[types.TINT])
	if n.X.Type() == nil || n.Y.Type() == nil {
		n.SetType(nil)
		return n
	}
	if !n.Y.Type().IsInteger() {
		n.SetType(nil)
		return n
	}
	n.SetType(n.X.Type())
	return n
}

// tcUnsafeSlice typechecks an OUNSAFESLICE node.
func tcUnsafeSlice(n *ir.BinaryExpr) *ir.BinaryExpr {
	n.X = Expr(n.X)
	n.Y = Expr(n.Y)
	if n.X.Type() == nil || n.Y.Type() == nil {
		n.SetType(nil)
		return n
	}
	t := n.X.Type()
	if !t.IsPtr() {
		base.Errorf("first argument to unsafe.Slice must be pointer; have %L", t)
	} else if t.Elem().NotInHeap() {
		// TODO(mdempsky): This can be relaxed, but should only affect the
		// Go runtime itself. End users should only see not-in-heap
		// types due to incomplete C structs in cgo, and those types don't
		// have a meaningful size anyway.
		base.Errorf("unsafe.Slice of incomplete (or unallocatable) type not allowed")
	}

	if !checkunsafesliceorstring(n.Op(), &n.Y) {
		n.SetType(nil)
		return n
	}
	n.SetType(types.NewSlice(t.Elem()))
	return n
}

// tcUnsafeString typechecks an OUNSAFESTRING node.
func tcUnsafeString(n *ir.BinaryExpr) *ir.BinaryExpr {
	n.X = Expr(n.X)
	n.Y = Expr(n.Y)
	if n.X.Type() == nil || n.Y.Type() == nil {
		n.SetType(nil)
		return n
	}
	t := n.X.Type()
	if !t.IsPtr() || !types.Identical(t.Elem(), types.Types[types.TUINT8]) {
		base.Errorf("first argument to unsafe.String must be *byte; have %L", t)
	}

	if !checkunsafesliceorstring(n.Op(), &n.Y) {
		n.SetType(nil)
		return n
	}
	n.SetType(types.Types[types.TSTRING])
	return n
}

// ClosureStructIter iterates through a slice of closure variables returning
// their type and offset in the closure struct.
type ClosureStructIter struct {
	closureVars []*ir.Name
	offset      int64
	next        int
}

// NewClosureStructIter creates a new ClosureStructIter for closureVars.
func NewClosureStructIter(closureVars []*ir.Name) *ClosureStructIter {
	return &ClosureStructIter{
		closureVars: closureVars,
		offset:      int64(types.PtrSize), // PtrSize to skip past function entry PC field
		next:        0,
	}
}

// Next returns the next name, type and offset of the next closure variable.
// A nil name is returned after the last closure variable.
func (iter *ClosureStructIter) Next() (n *ir.Name, typ *types.Type, offset int64) {
	if iter.next >= len(iter.closureVars) {
		return nil, nil, 0
	}
	n = iter.closureVars[iter.next]
	typ = n.Type()
	if !n.Byval() {
		typ = types.NewPtr(typ)
	}
	iter.next++
	offset = types.RoundUp(iter.offset, typ.Alignment())
	iter.offset = offset + typ.Size()
	return n, typ, offset
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/iexport.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Indexed package export.
//
// The indexed export data format is an evolution of the previous
// binary export data format. Its chief contribution is introducing an
// index table, which allows efficient random access of individual
// declarations and inline function bodies. In turn, this allows
// avoiding unnecessary work for compilation units that import large
// packages.
//
//
// The top-level data format is structured as:
//
//     Header struct {
//         Tag        byte   // 'i'
//         Version    uvarint
//         StringSize uvarint
//         DataSize   uvarint
//     }
//
//     Strings [StringSize]byte
//     Data    [DataSize]byte
//
//     MainIndex []struct{
//         PkgPath   stringOff
//         PkgName   stringOff
//         PkgHeight uvarint
//
//         Decls []struct{
//             Name   stringOff
//             Offset declOff
//         }
//     }
//
//     Fingerprint [8]byte
//
// uvarint means a uint64 written out using uvarint encoding.
//
// []T means a uvarint followed by that many T objects. In other
// words:
//
//     Len   uvarint
//     Elems [Len]T
//
// stringOff means a uvarint that indicates an offset within the
// Strings section. At that offset is another uvarint, followed by
// that many bytes, which form the string value.
//
// declOff means a uvarint that indicates an offset within the Data
// section where the associated declaration can be found.
//
//
// There are five kinds of declarations, distinguished by their first
// byte:
//
//     type Var struct {
//         Tag  byte // 'V'
//         Pos  Pos
//         Type typeOff
//     }
//
//     type Func struct {
//         Tag       byte // 'F' or 'G'
//         Pos       Pos
//         TypeParams []typeOff  // only present if Tag == 'G'
//         Signature Signature
//     }
//
//     type Const struct {
//         Tag   byte // 'C'
//         Pos   Pos
//         Value Value
//     }
//
//     type Type struct {
//         Tag        byte // 'T' or 'U'
//         Pos        Pos
//         TypeParams []typeOff  // only present if Tag == 'U'
//         Underlying typeOff
//
//         Methods []struct{  // omitted if Underlying is an interface type
//             Pos       Pos
//             Name      stringOff
//             Recv      Param
//             Signature Signature
//         }
//     }
//
//     type Alias struct {
//         Tag  byte // 'A' or 'B'
//         Pos  Pos
//         TypeParams []typeOff  // only present if Tag == 'B'
//         Type typeOff
//     }
//
//     // "Automatic" declaration of each typeparam
//     type TypeParam struct {
//         Tag        byte // 'P'
//         Pos        Pos
//         Implicit   bool
//         Constraint typeOff
//     }
//
// typeOff means a uvarint that either indicates a predeclared type,
// or an offset into the Data section. If the uvarint is less than
// predeclReserved, then it indicates the index into the predeclared
// types list (see predeclared in bexport.go for order). Otherwise,
// subtracting predeclReserved yields the offset of a type descriptor.
//
// Value means a type, kind, and type-specific value. See
// (*exportWriter).value for details.
//
//
// There are twelve kinds of type descriptors, distinguished by an itag:
//
//     type DefinedType struct {
//         Tag     itag // definedType
//         Name    stringOff
//         PkgPath stringOff
//     }
//
//     type PointerType struct {
//         Tag  itag // pointerType
//         Elem typeOff
//     }
//
//     type SliceType struct {
//         Tag  itag // sliceType
//         Elem typeOff
//     }
//
//     type ArrayType struct {
//         Tag  itag // arrayType
//         Len  uint64
//         Elem typeOff
//     }
//
//     type ChanType struct {
//         Tag  itag   // chanType
//         Dir  uint64 // 1 RecvOnly; 2 SendOnly; 3 SendRecv
//         Elem typeOff
//     }
//
//     type MapType struct {
//         Tag  itag // mapType
//         Key  typeOff
//         Elem typeOff
//     }
//
//     type FuncType struct {
//         Tag       itag // signatureType
//         PkgPath   stringOff
//         Signature Signature
//     }
//
//     type StructType struct {
//         Tag     itag // structType
//         PkgPath stringOff
//         Fields []struct {
//             Pos      Pos
//             Name     stringOff
//             Type     typeOff
//             Embedded bool
//             Note     stringOff
//         }
//     }
//
//     type InterfaceType struct {
//         Tag     itag // interfaceType
//         PkgPath stringOff
//         Embeddeds []struct {
//             Pos  Pos
//             Type typeOff
//         }
//         Methods []struct {
//             Pos       Pos
//             Name      stringOff
//             Signature Signature
//         }
//     }
//
//     // Reference to a type param declaration
//     type TypeParamType struct {
//         Tag     itag // typeParamType
//         Name    stringOff
//         PkgPath stringOff
//     }
//
//     // Instantiation of a generic type (like List[T2] or List[int])
//     type InstanceType struct {
//         Tag     itag // instanceType
//         Pos     pos
//         TypeArgs []typeOff
//         BaseType typeOff
//     }
//
//     type UnionType struct {
//         Tag     itag // interfaceType
//         Terms   []struct {
//             tilde bool
//             Type  typeOff
//         }
//     }
//
//
//
//     type Signature struct {
//         Params   []Param
//         Results  []Param
//         Variadic bool  // omitted if Results is empty
//     }
//
//     type Param struct {
//         Pos  Pos
//         Name stringOff
//         Type typOff
//     }
//
//
// Pos encodes a file:line:column triple, incorporating a simple delta
// encoding scheme within a data object. See exportWriter.pos for
// details.
//
//
// Compiler-specific details.
//
// cmd/compile writes out a second index for inline bodies and also
// appends additional compiler-specific details after declarations.
// Third-party tools are not expected to depend on these details and
// they're expected to change much more rapidly, so they're omitted
// here. See exportWriter's varExt/funcExt/etc methods for details.

package typecheck

const blankMarker = "$"

// The name used for dictionary parameters or local variables.
const LocalDictName = ".dict"

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/iimport.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Indexed package import.
// See iexport.go for the export data format.

package typecheck

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
)

// HaveInlineBody reports whether we have fn's inline body available
// for inlining.
//
// It's a function literal so that it can be overridden for
// GOEXPERIMENT=unified.
var HaveInlineBody = func(fn *ir.Func) bool {
	base.Fatalf("HaveInlineBody not overridden")
	panic("unreachable")
}

func SetBaseTypeIndex(t *types.Type, i, pi int64) {
	if t.Obj() == nil {
		base.Fatalf("SetBaseTypeIndex on non-defined type %v", t)
	}
	if i != -1 && pi != -1 {
		typeSymIdx[t] = [2]int64{i, pi}
	}
}

// Map imported type T to the index of type descriptor symbols of T and *T,
// so we can use index to reference the symbol.
// TODO(mdempsky): Store this information directly in the Type's Name.
var typeSymIdx = make(map[*types.Type][2]int64)

func BaseTypeIndex(t *types.Type) int64 {
	tbase := t
	if t.IsPtr() && t.Sym() == nil && t.Elem().Sym() != nil {
		tbase = t.Elem()
	}
	i, ok := typeSymIdx[tbase]
	if !ok {
		return -1
	}
	if t != tbase {
		return i[1]
	}
	return i[0]
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/mkbuiltin.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// Generate builtin.go from builtin/runtime.go.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var stdout = flag.Bool("stdout", false, "write to stdout instead of builtin.go")
var nofmt = flag.Bool("nofmt", false, "skip formatting builtin.go")

func main() {
	flag.Parse()

	var b bytes.Buffer
	fmt.Fprintln(&b, "// Code generated by mkbuiltin.go. DO NOT EDIT.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "package typecheck")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, `import (`)
	fmt.Fprintln(&b, `      "cmd/compile/internal/types"`)
	fmt.Fprintln(&b, `      "cmd/internal/src"`)
	fmt.Fprintln(&b, `)`)

	fmt.Fprintln(&b, `
// Not inlining this function removes a significant chunk of init code.
//go:noinline
func newSig(params, results []*types.Field) *types.Type {
	return types.NewSignature(nil, params, results)
}

func params(tlist ...*types.Type) []*types.Field {
	flist := make([]*types.Field, len(tlist))
	for i, typ := range tlist {
		flist[i] = types.NewField(src.NoXPos, nil, typ)
	}
	return flist
}
`)

	mkbuiltin(&b, "runtime")
	mkbuiltin(&b, "coverage")

	var err error
	out := b.Bytes()
	if !*nofmt {
		out, err = format.Source(out)
		if err != nil {
			log.Fatal(err)
		}
	}
	if *stdout {
		_, err = os.Stdout.Write(out)
	} else {
		err = os.WriteFile("builtin.go", out, 0666)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func mkbuiltin(w io.Writer, name string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join("_builtin", name+".go"), nil, parser.SkipObjectResolution)
	if err != nil {
		log.Fatal(err)
	}

	var interner typeInterner

	fmt.Fprintf(w, "var %sDecls = [...]struct { name string; tag int; typ int }{\n", name)
	for _, decl := range f.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Recv != nil {
				log.Fatal("methods unsupported")
			}
			if decl.Body != nil {
				log.Fatal("unexpected function body")
			}
			fmt.Fprintf(w, "{%q, funcTag, %d},\n", decl.Name.Name, interner.intern(decl.Type))
		case *ast.GenDecl:
			if decl.Tok == token.IMPORT {
				if len(decl.Specs) != 1 || decl.Specs[0].(*ast.ImportSpec).Path.Value != "\"unsafe\"" {
					log.Fatal("runtime cannot import other package")
				}
				continue
			}
			if decl.Tok != token.VAR {
				log.Fatal("unhandled declaration kind", decl.Tok)
			}
			for _, spec := range decl.Specs {
				spec := spec.(*ast.ValueSpec)
				if len(spec.Values) != 0 {
					log.Fatal("unexpected values")
				}
				typ := interner.intern(spec.Type)
				for _, name := range spec.Names {
					fmt.Fprintf(w, "{%q, varTag, %d},\n", name.Name, typ)
				}
			}
		default:
			log.Fatal("unhandled decl type", decl)
		}
	}
	fmt.Fprintln(w, "}")

	fmt.Fprintln(w)
	fmt.Fprintf(w, "func %sTypes() []*types.Type {\n", name)
	fmt.Fprintf(w, "var typs [%d]*types.Type\n", len(interner.typs))
	for i, typ := range interner.typs {
		fmt.Fprintf(w, "typs[%d] = %s\n", i, typ)
	}
	fmt.Fprintln(w, "return typs[:]")
	fmt.Fprintln(w, "}")
}

// typeInterner maps Go type expressions to compiler code that
// constructs the denoted type. It recognizes and reuses common
// subtype expressions.
type typeInterner struct {
	typs []string
	hash map[string]int
}

func (i *typeInterner) intern(t ast.Expr) int {
	x := i.mktype(t)
	v, ok := i.hash[x]
	if !ok {
		v = len(i.typs)
		if i.hash == nil {
			i.hash = make(map[string]int)
		}
		i.hash[x] = v
		i.typs = append(i.typs, x)
	}
	return v
}

func (i *typeInterner) subtype(t ast.Expr) string {
	return fmt.Sprintf("typs[%d]", i.intern(t))
}

func (i *typeInterner) mktype(t ast.Expr) string {
	switch t := t.(type) {
	case *ast.Ident:
		switch t.Name {
		case "byte":
			return "types.ByteType"
		case "rune":
			return "types.RuneType"
		}
		return fmt.Sprintf("types.Types[types.T%s]", strings.ToUpper(t.Name))
	case *ast.SelectorExpr:
		if t.X.(*ast.Ident).Name != "unsafe" || t.Sel.Name != "Pointer" {
			log.Fatalf("unhandled type: %#v", t)
		}
		return "types.Types[types.TUNSAFEPTR]"

	case *ast.ArrayType:
		if t.Len == nil {
			return fmt.Sprintf("types.NewSlice(%s)", i.subtype(t.Elt))
		}
		return fmt.Sprintf("types.NewArray(%s, %d)", i.subtype(t.Elt), intconst(t.Len))
	case *ast.ChanType:
		dir := "types.Cboth"
		switch t.Dir {
		case ast.SEND:
			dir = "types.Csend"
		case ast.RECV:
			dir = "types.Crecv"
		}
		return fmt.Sprintf("types.NewChan(%s, %s)", i.subtype(t.Value), dir)
	case *ast.FuncType:
		return fmt.Sprintf("newSig(%s, %s)", i.fields(t.Params, false), i.fields(t.Results, false))
	case *ast.InterfaceType:
		if len(t.Methods.List) != 0 {
			log.Fatal("non-empty interfaces unsupported")
		}
		return "types.Types[types.TINTER]"
	case *ast.MapType:
		return fmt.Sprintf("types.NewMap(%s, %s)", i.subtype(t.Key), i.subtype(t.Value))
	case *ast.StarExpr:
		return fmt.Sprintf("types.NewPtr(%s)", i.subtype(t.X))
	case *ast.StructType:
		return fmt.Sprintf("types.NewStruct(%s)", i.fields(t.Fields, true))

	default:
		log.Fatalf("unhandled type: %#v", t)
		panic("unreachable")
	}
}

func (i *typeInterner) fields(fl *ast.FieldList, keepNames bool) string {
	if fl == nil || len(fl.List) == 0 {
		return "nil"
	}

	var res []string
	for _, f := range fl.List {
		typ := i.subtype(f.Type)
		if len(f.Names) == 0 {
			res = append(res, typ)
		} else {
			for _, name := range f.Names {
				if keepNames {
					res = append(res, fmt.Sprintf("types.NewField(src.NoXPos, Lookup(%q), %s)", name.Name, typ))
				} else {
					res = append(res, typ)
				}
			}
		}
	}

	if keepNames {
		return fmt.Sprintf("[]*types.Field{%s}", strings.Join(res, ", "))
	}
	return fmt.Sprintf("params(%s)", strings.Join(res, ", "))
}

func intconst(e ast.Expr) int64 {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind != token.INT {
			log.Fatalf("expected INT, got %v", e.Kind)
		}
		x, err := strconv.ParseInt(e.Value, 0, 64)
		if err != nil {
			log.Fatal(err)
		}
		return x
	default:
		log.Fatalf("unhandled expr: %#v", e)
		panic("unreachable")
	}
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/stmt.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/src"
	"internal/types/errors"
)

func RangeExprType(t *types.Type) *types.Type {
	if t.IsPtr() && t.Elem().IsArray() {
		return t.Elem()
	}
	return t
}

// type check assignment.
// if this assignment is the definition of a var on the left side,
// fill in the var's type.
func tcAssign(n *ir.AssignStmt) {
	if base.EnableTrace && base.Flag.LowerT {
		defer tracePrint("tcAssign", n)(nil)
	}

	if n.Y == nil {
		n.X = AssignExpr(n.X)
		return
	}

	lhs, rhs := []ir.Node{n.X}, []ir.Node{n.Y}
	assign(n, lhs, rhs)
	n.X, n.Y = lhs[0], rhs[0]

	// TODO(mdempsky): This seems out of place.
	if !ir.IsBlank(n.X) {
		types.CheckSize(n.X.Type()) // ensure width is calculated for backend
	}
}

func tcAssignList(n *ir.AssignListStmt) {
	if base.EnableTrace && base.Flag.LowerT {
		defer tracePrint("tcAssignList", n)(nil)
	}

	assign(n, n.Lhs, n.Rhs)
}

func assign(stmt ir.Node, lhs, rhs []ir.Node) {
	// delicate little dance.
	// the definition of lhs may refer to this assignment
	// as its definition, in which case it will call tcAssign.
	// in that case, do not call typecheck back, or it will cycle.
	// if the variable has a type (ntype) then typechecking
	// will not look at defn, so it is okay (and desirable,
	// so that the conversion below happens).

	checkLHS := func(i int, typ *types.Type) {
		if n := lhs[i]; typ != nil && ir.DeclaredBy(n, stmt) && n.Type() == nil {
			base.Assertf(typ.Kind() == types.TNIL, "unexpected untyped nil")
			n.SetType(defaultType(typ))
		}
		if lhs[i].Typecheck() == 0 {
			lhs[i] = AssignExpr(lhs[i])
		}
		checkassign(lhs[i])
	}

	assignType := func(i int, typ *types.Type) {
		checkLHS(i, typ)
		if typ != nil {
			checkassignto(typ, lhs[i])
		}
	}

	cr := len(rhs)
	if len(rhs) == 1 {
		rhs[0] = typecheck(rhs[0], ctxExpr|ctxMultiOK)
		if rtyp := rhs[0].Type(); rtyp != nil && rtyp.IsFuncArgStruct() {
			cr = rtyp.NumFields()
		}
	} else {
		Exprs(rhs)
	}

	// x, ok = y
assignOK:
	for len(lhs) == 2 && cr == 1 {
		stmt := stmt.(*ir.AssignListStmt)
		r := rhs[0]

		switch r.Op() {
		case ir.OINDEXMAP:
			stmt.SetOp(ir.OAS2MAPR)
		case ir.ORECV:
			stmt.SetOp(ir.OAS2RECV)
		case ir.ODOTTYPE:
			r := r.(*ir.TypeAssertExpr)
			stmt.SetOp(ir.OAS2DOTTYPE)
			r.SetOp(ir.ODOTTYPE2)
		case ir.ODYNAMICDOTTYPE:
			r := r.(*ir.DynamicTypeAssertExpr)
			stmt.SetOp(ir.OAS2DOTTYPE)
			r.SetOp(ir.ODYNAMICDOTTYPE2)
		default:
			break assignOK
		}

		assignType(0, r.Type())
		assignType(1, types.UntypedBool)
		return
	}

	if len(lhs) != cr {
		if r, ok := rhs[0].(*ir.CallExpr); ok && len(rhs) == 1 {
			if r.Type() != nil {
				base.ErrorfAt(stmt.Pos(), errors.WrongAssignCount, "assignment mismatch: %d variable%s but %v returns %d value%s", len(lhs), plural(len(lhs)), r.Fun, cr, plural(cr))
			}
		} else {
			base.ErrorfAt(stmt.Pos(), errors.WrongAssignCount, "assignment mismatch: %d variable%s but %v value%s", len(lhs), plural(len(lhs)), len(rhs), plural(len(rhs)))
		}

		for i := range lhs {
			checkLHS(i, nil)
		}
		return
	}

	// x,y,z = f()
	if cr > len(rhs) {
		stmt := stmt.(*ir.AssignListStmt)
		stmt.SetOp(ir.OAS2FUNC)
		r := rhs[0].(*ir.CallExpr)
		rtyp := r.Type()

		mismatched := false
		failed := false
		for i := range lhs {
			result := rtyp.Field(i).Type
			assignType(i, result)

			if lhs[i].Type() == nil || result == nil {
				failed = true
			} else if lhs[i] != ir.BlankNode && !types.Identical(lhs[i].Type(), result) {
				mismatched = true
			}
		}
		if mismatched && !failed {
			RewriteMultiValueCall(stmt, r)
		}
		return
	}

	for i, r := range rhs {
		checkLHS(i, r.Type())
		if lhs[i].Type() != nil {
			rhs[i] = AssignConv(r, lhs[i].Type(), "assignment")
		}
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// tcCheckNil typechecks an OCHECKNIL node.
func tcCheckNil(n *ir.UnaryExpr) ir.Node {
	n.X = Expr(n.X)
	if !n.X.Type().IsPtrShaped() {
		base.FatalfAt(n.Pos(), "%L is not pointer shaped", n.X)
	}
	return n
}

// tcFor typechecks an OFOR node.
func tcFor(n *ir.ForStmt) ir.Node {
	Stmts(n.Init())
	n.Cond = Expr(n.Cond)
	n.Cond = DefaultLit(n.Cond, nil)
	if n.Cond != nil {
		t := n.Cond.Type()
		if t != nil && !t.IsBoolean() {
			base.Errorf("non-bool %L used as for condition", n.Cond)
		}
	}
	n.Post = Stmt(n.Post)
	Stmts(n.Body)
	return n
}

// tcGoDefer typechecks (normalizes) an OGO/ODEFER statement.
func tcGoDefer(n *ir.GoDeferStmt) {
	call := normalizeGoDeferCall(n.Pos(), n.Op(), n.Call, n.PtrInit())
	call.GoDefer = true
	n.Call = call
}

// normalizeGoDeferCall normalizes call into a normal function call
// with no arguments and no results, suitable for use in an OGO/ODEFER
// statement.
//
// For example, it normalizes:
//
//	f(x, y)
//
// into:
//
//	x1, y1 := x, y          // added to init
//	func() { f(x1, y1) }()  // result
func normalizeGoDeferCall(pos src.XPos, op ir.Op, call ir.Node, init *ir.Nodes) *ir.CallExpr {
	init.Append(ir.TakeInit(call)...)

	if call, ok := call.(*ir.CallExpr); ok && call.Op() == ir.OCALLFUNC {
		if sig := call.Fun.Type(); sig.NumParams()+sig.NumResults() == 0 {
			return call // already in normal form
		}
	}

	// Create a new wrapper function without parameters or results.
	wrapperFn := ir.NewClosureFunc(pos, pos, op, types.NewSignature(nil, nil, nil), ir.CurFunc, Target, 0)
	wrapperFn.DeclareParams(true)
	wrapperFn.SetWrapper(true)

	// argps collects the list of operands within the call expression
	// that must be evaluated at the go/defer statement.
	var argps []*ir.Node

	var visit func(argp *ir.Node)
	visit = func(argp *ir.Node) {
		arg := *argp
		if arg == nil {
			return
		}

		// Recognize a few common expressions that can be evaluated within
		// the wrapper, so we don't need to allocate space for them within
		// the closure.
		switch arg.Op() {
		case ir.OLITERAL, ir.ONIL, ir.OMETHEXPR, ir.ONEW:
			return
		case ir.ONAME:
			arg := arg.(*ir.Name)
			if arg.Class == ir.PFUNC {
				return // reference to global function
			}
		case ir.OADDR:
			arg := arg.(*ir.AddrExpr)
			if arg.X.Op() == ir.OLINKSYMOFFSET {
				return // address of global symbol
			}

		case ir.OCONVNOP:
			arg := arg.(*ir.ConvExpr)

			// For unsafe.Pointer->uintptr conversion arguments, save the
			// unsafe.Pointer argument. This is necessary to handle cases
			// like fixedbugs/issue24491a.go correctly.
			//
			// TODO(mdempsky): Limit to static callees with
			// //go:uintptr{escapes,keepalive}?
			if arg.Type().IsUintptr() && arg.X.Type().IsUnsafePtr() {
				visit(&arg.X)
				return
			}

		case ir.OARRAYLIT, ir.OSLICELIT, ir.OSTRUCTLIT:
			// TODO(mdempsky): For very large slices, it may be preferable
			// to construct them at the go/defer statement instead.
			list := arg.(*ir.CompLitExpr).List
			for i, el := range list {
				switch el := el.(type) {
				case *ir.KeyExpr:
					visit(&el.Value)
				case *ir.StructKeyExpr:
					visit(&el.Value)
				default:
					visit(&list[i])
				}
			}
			return
		}

		argps = append(argps, argp)
	}

	visitList := func(list []ir.Node) {
		for i := range list {
			visit(&list[i])
		}
	}

	switch call.Op() {
	default:
		base.Fatalf("unexpected call op: %v", call.Op())

	case ir.OCALLFUNC:
		call := call.(*ir.CallExpr)

		// If the callee is a named function, link to the original callee.
		if wrapped := ir.StaticCalleeName(call.Fun); wrapped != nil {
			wrapperFn.WrappedFunc = wrapped.Func
		}

		visit(&call.Fun)
		visitList(call.Args)

	case ir.OCALLINTER:
		call := call.(*ir.CallExpr)
		argps = append(argps, &call.Fun.(*ir.SelectorExpr).X) // must be first for OCHECKNIL; see below
		visitList(call.Args)

	case ir.OAPPEND, ir.ODELETE, ir.OPRINT, ir.OPRINTLN, ir.ORECOVER:
		call := call.(*ir.CallExpr)
		visitList(call.Args)
		visit(&call.RType)

	case ir.OCOPY:
		call := call.(*ir.BinaryExpr)
		visit(&call.X)
		visit(&call.Y)
		visit(&call.RType)

	case ir.OCLEAR, ir.OCLOSE, ir.OPANIC:
		call := call.(*ir.UnaryExpr)
		visit(&call.X)
	}

	if len(argps) != 0 {
		// Found one or more operands that need to be evaluated upfront
		// and spilled to temporary variables, which can be captured by
		// the wrapper function.

		stmtPos := base.Pos
		callPos := base.Pos

		as := ir.NewAssignListStmt(callPos, ir.OAS2, make([]ir.Node, len(argps)), make([]ir.Node, len(argps)))
		for i, argp := range argps {
			arg := *argp

			pos := callPos
			if ir.HasUniquePos(arg) {
				pos = arg.Pos()
			}

			// tmp := arg
			tmp := TempAt(pos, ir.CurFunc, arg.Type())
			init.Append(Stmt(ir.NewDecl(pos, ir.ODCL, tmp)))
			tmp.Defn = as
			as.Lhs[i] = tmp
			as.Rhs[i] = arg

			// Rewrite original expression to use/capture tmp.
			*argp = ir.NewClosureVar(pos, wrapperFn, tmp)
		}
		init.Append(Stmt(as))

		// For "go/defer iface.M()", if iface is nil, we need to panic at
		// the point of the go/defer statement.
		if call.Op() == ir.OCALLINTER {
			iface := as.Lhs[0]
			init.Append(Stmt(ir.NewUnaryExpr(stmtPos, ir.OCHECKNIL, ir.NewUnaryExpr(iface.Pos(), ir.OITAB, iface))))
		}
	}

	// Move call into the wrapper function, now that it's safe to
	// evaluate there.
	wrapperFn.Body = []ir.Node{call}

	// Finally, construct a call to the wrapper.
	return Call(call.Pos(), wrapperFn.OClosure, nil, false).(*ir.CallExpr)
}

// tcIf typechecks an OIF node.
func tcIf(n *ir.IfStmt) ir.Node {
	Stmts(n.Init())
	n.Cond = Expr(n.Cond)
	n.Cond = DefaultLit(n.Cond, nil)
	if n.Cond != nil {
		t := n.Cond.Type()
		if t != nil && !t.IsBoolean() {
			base.Errorf("non-bool %L used as if condition", n.Cond)
		}
	}
	Stmts(n.Body)
	Stmts(n.Else)
	return n
}

// range
func tcRange(n *ir.RangeStmt) {
	n.X = Expr(n.X)

	// delicate little dance.  see tcAssignList
	if n.Key != nil {
		if !ir.DeclaredBy(n.Key, n) {
			n.Key = AssignExpr(n.Key)
		}
		checkassign(n.Key)
	}
	if n.Value != nil {
		if !ir.DeclaredBy(n.Value, n) {
			n.Value = AssignExpr(n.Value)
		}
		checkassign(n.Value)
	}

	// second half of dance
	n.SetTypecheck(1)
	if n.Key != nil && n.Key.Typecheck() == 0 {
		n.Key = AssignExpr(n.Key)
	}
	if n.Value != nil && n.Value.Typecheck() == 0 {
		n.Value = AssignExpr(n.Value)
	}

	Stmts(n.Body)
}

// tcReturn typechecks an ORETURN node.
func tcReturn(n *ir.ReturnStmt) ir.Node {
	if ir.CurFunc == nil {
		base.FatalfAt(n.Pos(), "return outside function")
	}

	typecheckargs(n)
	if len(n.Results) != 0 {
		typecheckaste(ir.ORETURN, nil, false, ir.CurFunc.Type().Results(), n.Results, func() string { return "return argument" })
	}
	return n
}

// select
func tcSelect(sel *ir.SelectStmt) {
	var def *ir.CommClause
	lno := ir.SetPos(sel)
	Stmts(sel.Init())
	for _, ncase := range sel.Cases {
		if ncase.Comm == nil {
			// default
			if def != nil {
				base.ErrorfAt(ncase.Pos(), errors.DuplicateDefault, "multiple defaults in select (first at %v)", ir.Line(def))
			} else {
				def = ncase
			}
		} else {
			n := Stmt(ncase.Comm)
			ncase.Comm = n
			oselrecv2 := func(dst, recv ir.Node, def bool) {
				selrecv := ir.NewAssignListStmt(n.Pos(), ir.OSELRECV2, []ir.Node{dst, ir.BlankNode}, []ir.Node{recv})
				selrecv.Def = def
				selrecv.SetTypecheck(1)
				selrecv.SetInit(n.Init())
				ncase.Comm = selrecv
			}
			switch n.Op() {
			default:
				pos := n.Pos()
				if n.Op() == ir.ONAME {
					// We don't have the right position for ONAME nodes (see #15459 and
					// others). Using ncase.Pos for now as it will provide the correct
					// line number (assuming the expression follows the "case" keyword
					// on the same line). This matches the approach before 1.10.
					pos = ncase.Pos()
				}
				base.ErrorfAt(pos, errors.InvalidSelectCase, "select case must be receive, send or assign recv")

			case ir.OAS:
				// convert x = <-c into x, _ = <-c
				// remove implicit conversions; the eventual assignment
				// will reintroduce them.
				n := n.(*ir.AssignStmt)
				if r := n.Y; r.Op() == ir.OCONVNOP || r.Op() == ir.OCONVIFACE {
					r := r.(*ir.ConvExpr)
					if r.Implicit() {
						n.Y = r.X
					}
				}
				if n.Y.Op() != ir.ORECV {
					base.ErrorfAt(n.Pos(), errors.InvalidSelectCase, "select assignment must have receive on right hand side")
					break
				}
				oselrecv2(n.X, n.Y, n.Def)

			case ir.OAS2RECV:
				n := n.(*ir.AssignListStmt)
				if n.Rhs[0].Op() != ir.ORECV {
					base.ErrorfAt(n.Pos(), errors.InvalidSelectCase, "select assignment must have receive on right hand side")
					break
				}
				n.SetOp(ir.OSELRECV2)

			case ir.ORECV:
				// convert <-c into _, _ = <-c
				n := n.(*ir.UnaryExpr)
				oselrecv2(ir.BlankNode, n, false)

			case ir.OSEND:
				break
			}
		}

		Stmts(ncase.Body)
	}

	base.Pos = lno
}

// tcSend typechecks an OSEND node.
func tcSend(n *ir.SendStmt) ir.Node {
	n.Chan = Expr(n.Chan)
	n.Value = Expr(n.Value)
	n.Chan = DefaultLit(n.Chan, nil)
	t := n.Chan.Type()
	if t == nil {
		return n
	}
	if !t.IsChan() {
		base.Errorf("invalid operation: %v (send to non-chan type %v)", n, t)
		return n
	}

	if !t.ChanDir().CanSend() {
		base.Errorf("invalid operation: %v (send to receive-only type %v)", n, t)
		return n
	}

	n.Value = AssignConv(n.Value, t.Elem(), "send")
	if n.Value.Type() == nil {
		return n
	}
	return n
}

// tcSwitch typechecks a switch statement.
func tcSwitch(n *ir.SwitchStmt) {
	Stmts(n.Init())
	if n.Tag != nil && n.Tag.Op() == ir.OTYPESW {
		tcSwitchType(n)
	} else {
		tcSwitchExpr(n)
	}
}

func tcSwitchExpr(n *ir.SwitchStmt) {
	t := types.Types[types.TBOOL]
	if n.Tag != nil {
		n.Tag = Expr(n.Tag)
		n.Tag = DefaultLit(n.Tag, nil)
		t = n.Tag.Type()
	}

	var nilonly string
	if t != nil {
		switch {
		case t.IsMap():
			nilonly = "map"
		case t.Kind() == types.TFUNC:
			nilonly = "func"
		case t.IsSlice():
			nilonly = "slice"

		case !types.IsComparable(t):
			if t.IsStruct() {
				base.ErrorfAt(n.Pos(), errors.InvalidExprSwitch, "cannot switch on %L (struct containing %v cannot be compared)", n.Tag, types.IncomparableField(t).Type)
			} else {
				base.ErrorfAt(n.Pos(), errors.InvalidExprSwitch, "cannot switch on %L", n.Tag)
			}
			t = nil
		}
	}

	var defCase ir.Node
	for _, ncase := range n.Cases {
		ls := ncase.List
		if len(ls) == 0 { // default:
			if defCase != nil {
				base.ErrorfAt(ncase.Pos(), errors.DuplicateDefault, "multiple defaults in switch (first at %v)", ir.Line(defCase))
			} else {
				defCase = ncase
			}
		}

		for i := range ls {
			ir.SetPos(ncase)
			ls[i] = Expr(ls[i])
			ls[i] = DefaultLit(ls[i], t)
			n1 := ls[i]
			if t == nil || n1.Type() == nil {
				continue
			}

			if nilonly != "" && !ir.IsNil(n1) {
				base.ErrorfAt(ncase.Pos(), errors.MismatchedTypes, "invalid case %v in switch (can only compare %s %v to nil)", n1, nilonly, n.Tag)
			} else if t.IsInterface() && !n1.Type().IsInterface() && !types.IsComparable(n1.Type()) {
				base.ErrorfAt(ncase.Pos(), errors.UndefinedOp, "invalid case %L in switch (incomparable type)", n1)
			} else {
				op1, _ := assignOp(n1.Type(), t)
				op2, _ := assignOp(t, n1.Type())
				if op1 == ir.OXXX && op2 == ir.OXXX {
					if n.Tag != nil {
						base.ErrorfAt(ncase.Pos(), errors.MismatchedTypes, "invalid case %v in switch on %v (mismatched types %v and %v)", n1, n.Tag, n1.Type(), t)
					} else {
						base.ErrorfAt(ncase.Pos(), errors.MismatchedTypes, "invalid case %v in switch (mismatched types %v and bool)", n1, n1.Type())
					}
				}
			}
		}

		Stmts(ncase.Body)
	}
}

func tcSwitchType(n *ir.SwitchStmt) {
	guard := n.Tag.(*ir.TypeSwitchGuard)
	guard.X = Expr(guard.X)
	t := guard.X.Type()
	if t != nil && !t.IsInterface() {
		base.ErrorfAt(n.Pos(), errors.InvalidTypeSwitch, "cannot type switch on non-interface value %L", guard.X)
		t = nil
	}

	// We don't actually declare the type switch's guarded
	// declaration itself. So if there are no cases, we won't
	// notice that it went unused.
	if v := guard.Tag; v != nil && !ir.IsBlank(v) && len(n.Cases) == 0 {
		base.ErrorfAt(v.Pos(), errors.UnusedVar, "%v declared but not used", v.Sym())
	}

	var defCase, nilCase ir.Node
	var ts typeSet
	for _, ncase := range n.Cases {
		ls := ncase.List
		if len(ls) == 0 { // default:
			if defCase != nil {
				base.ErrorfAt(ncase.Pos(), errors.DuplicateDefault, "multiple defaults in switch (first at %v)", ir.Line(defCase))
			} else {
				defCase = ncase
			}
		}

		for i := range ls {
			ls[i] = typecheck(ls[i], ctxExpr|ctxType)
			n1 := ls[i]
			if t == nil || n1.Type() == nil {
				continue
			}

			if ir.IsNil(n1) { // case nil:
				if nilCase != nil {
					base.ErrorfAt(ncase.Pos(), errors.DuplicateCase, "multiple nil cases in type switch (first at %v)", ir.Line(nilCase))
				} else {
					nilCase = ncase
				}
				continue
			}
			if n1.Op() == ir.ODYNAMICTYPE {
				continue
			}
			if n1.Op() != ir.OTYPE {
				base.ErrorfAt(ncase.Pos(), errors.NotAType, "%L is not a type", n1)
				continue
			}
			if !n1.Type().IsInterface() {
				why := ImplementsExplain(n1.Type(), t)
				if why != "" {
					base.ErrorfAt(ncase.Pos(), errors.ImpossibleAssert, "impossible type switch case: %L cannot have dynamic type %v (%s)", guard.X, n1.Type(), why)
				}
				continue
			}

			ts.add(ncase.Pos(), n1.Type())
		}

		if ncase.Var != nil {
			// Assign the clause variable's type.
			vt := t
			if len(ls) == 1 {
				if ls[0].Op() == ir.OTYPE || ls[0].Op() == ir.ODYNAMICTYPE {
					vt = ls[0].Type()
				} else if !ir.IsNil(ls[0]) {
					// Invalid single-type case;
					// mark variable as broken.
					vt = nil
				}
			}

			nvar := ncase.Var
			nvar.SetType(vt)
			if vt != nil {
				nvar = AssignExpr(nvar).(*ir.Name)
			} else {
				// Clause variable is broken; prevent typechecking.
				nvar.SetTypecheck(1)
			}
			ncase.Var = nvar
		}

		Stmts(ncase.Body)
	}
}

type typeSet struct {
	m map[string]src.XPos
}

func (s *typeSet) add(pos src.XPos, typ *types.Type) {
	if s.m == nil {
		s.m = make(map[string]src.XPos)
	}

	ls := typ.LinkString()
	if prev, ok := s.m[ls]; ok {
		base.ErrorfAt(pos, errors.DuplicateCase, "duplicate case %v in type switch\n\tprevious case at %s", typ, base.FmtPos(prev))
		return
	}
	s.m[ls] = pos
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/subr.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"fmt"
	"slices"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
	"cmd/internal/src"
)

func AssignConv(n ir.Node, t *types.Type, context string) ir.Node {
	return assignconvfn(n, t, func() string { return context })
}

// LookupNum returns types.LocalPkg.LookupNum(prefix, n).
func LookupNum(prefix string, n int) *types.Sym {
	return types.LocalPkg.LookupNum(prefix, n)
}

// Given funarg struct list, return list of fn args.
func NewFuncParams(origs []*types.Field) []*types.Field {
	res := make([]*types.Field, len(origs))
	for i, orig := range origs {
		p := types.NewField(orig.Pos, orig.Sym, orig.Type)
		p.SetIsDDD(orig.IsDDD())
		res[i] = p
	}
	return res
}

// NodAddr returns a node representing &n at base.Pos.
func NodAddr(n ir.Node) *ir.AddrExpr {
	return NodAddrAt(base.Pos, n)
}

// NodAddrAt returns a node representing &n at position pos.
func NodAddrAt(pos src.XPos, n ir.Node) *ir.AddrExpr {
	return ir.NewAddrExpr(pos, Expr(n))
}

// LinksymAddr returns a new expression that evaluates to the address
// of lsym. typ specifies the type of the addressed memory.
func LinksymAddr(pos src.XPos, lsym *obj.LSym, typ *types.Type) *ir.AddrExpr {
	n := ir.NewLinksymExpr(pos, lsym, typ)
	return Expr(NodAddrAt(pos, n)).(*ir.AddrExpr)
}

func NodNil() ir.Node {
	return ir.NewNilExpr(base.Pos, types.Types[types.TNIL])
}

// AddImplicitDots finds missing fields in obj.field that
// will give the shortest unique addressing and
// modifies the tree with missing field names.
func AddImplicitDots(n *ir.SelectorExpr) *ir.SelectorExpr {
	n.X = typecheck(n.X, ctxType|ctxExpr)
	t := n.X.Type()
	if t == nil {
		return n
	}

	if n.X.Op() == ir.OTYPE {
		return n
	}

	s := n.Sel
	if s == nil {
		return n
	}

	switch path, ambig := dotpath(s, t, nil, false); {
	case path != nil:
		// rebuild elided dots
		for c := len(path) - 1; c >= 0; c-- {
			dot := ir.NewSelectorExpr(n.Pos(), ir.ODOT, n.X, path[c].field.Sym)
			dot.SetImplicit(true)
			dot.SetType(path[c].field.Type)
			n.X = dot
		}
	case ambig:
		base.Errorf("ambiguous selector %v", n)
		n.X = nil
	}

	return n
}

// CalcMethods calculates all the methods (including embedding) of a non-interface
// type t.
func CalcMethods(t *types.Type) {
	if t == nil || len(t.AllMethods()) != 0 {
		return
	}

	// mark top-level method symbols
	// so that expand1 doesn't consider them.
	for _, f := range t.Methods() {
		f.Sym.SetUniq(true)
	}

	// generate all reachable methods
	slist = slist[:0]
	expand1(t, true)

	// check each method to be uniquely reachable
	var ms []*types.Field
	for i, sl := range slist {
		slist[i].field = nil
		sl.field.Sym.SetUniq(false)

		var f *types.Field
		path, _ := dotpath(sl.field.Sym, t, &f, false)
		if path == nil {
			continue
		}

		// dotpath may have dug out arbitrary fields, we only want methods.
		if !f.IsMethod() {
			continue
		}

		// add it to the base type method list
		f = f.Copy()
		f.Embedded = 1 // needs a trampoline
		for _, d := range path {
			if d.field.Type.IsPtr() {
				f.Embedded = 2
				break
			}
		}
		ms = append(ms, f)
	}

	for _, f := range t.Methods() {
		f.Sym.SetUniq(false)
	}

	ms = append(ms, t.Methods()...)
	slices.SortFunc(ms, types.CompareFields)
	t.SetAllMethods(ms)
}

// adddot1 returns the number of fields or methods named s at depth d in Type t.
// If exactly one exists, it will be returned in *save (if save is not nil),
// and dotlist will contain the path of embedded fields traversed to find it,
// in reverse order. If none exist, more will indicate whether t contains any
// embedded fields at depth d, so callers can decide whether to retry at
// a greater depth.
func adddot1(s *types.Sym, t *types.Type, d int, save **types.Field, ignorecase bool) (c int, more bool) {
	if t.Recur() {
		return
	}
	t.SetRecur(true)
	defer t.SetRecur(false)

	var u *types.Type
	d--
	if d < 0 {
		// We've reached our target depth. If t has any fields/methods
		// named s, then we're done. Otherwise, we still need to check
		// below for embedded fields.
		c = lookdot0(s, t, save, ignorecase)
		if c != 0 {
			return c, false
		}
	}

	u = t
	if u.IsPtr() {
		u = u.Elem()
	}
	if !u.IsStruct() && !u.IsInterface() {
		return c, false
	}

	var fields []*types.Field
	if u.IsStruct() {
		fields = u.Fields()
	} else {
		fields = u.AllMethods()
	}
	for _, f := range fields {
		if f.Embedded == 0 || f.Sym == nil {
			continue
		}
		if d < 0 {
			// Found an embedded field at target depth.
			return c, true
		}
		a, more1 := adddot1(s, f.Type, d, save, ignorecase)
		if a != 0 && c == 0 {
			dotlist[d].field = f
		}
		c += a
		if more1 {
			more = true
		}
	}

	return c, more
}

// dotlist is used by adddot1 to record the path of embedded fields
// used to access a target field or method.
// Must be non-nil so that dotpath returns a non-nil slice even if d is zero.
var dotlist = make([]dlist, 10)

// Convert node n for assignment to type t.
func assignconvfn(n ir.Node, t *types.Type, context func() string) ir.Node {
	if n == nil || n.Type() == nil {
		return n
	}

	if t.Kind() == types.TBLANK && n.Type().Kind() == types.TNIL {
		base.Errorf("use of untyped nil")
	}

	n = convlit1(n, t, false, context)
	if n.Type() == nil {
		base.Fatalf("cannot assign %v to %v", n, t)
	}
	if n.Type().IsUntyped() {
		base.Fatalf("%L has untyped type", n)
	}
	if t.Kind() == types.TBLANK {
		return n
	}
	if types.Identical(n.Type(), t) {
		return n
	}

	op, why := assignOp(n.Type(), t)
	if op == ir.OXXX {
		base.Errorf("cannot use %L as type %v in %s%s", n, t, context(), why)
		op = ir.OCONV
	}

	r := ir.NewConvExpr(base.Pos, op, t, n)
	r.SetTypecheck(1)
	r.SetImplicit(true)
	return r
}

// Is type src assignment compatible to type dst?
// If so, return op code to use in conversion.
// If not, return OXXX. In this case, the string return parameter may
// hold a reason why. In all other cases, it'll be the empty string.
func assignOp(src, dst *types.Type) (ir.Op, string) {
	if src == dst {
		return ir.OCONVNOP, ""
	}
	if src == nil || dst == nil || src.Kind() == types.TFORW || dst.Kind() == types.TFORW || src.Underlying() == nil || dst.Underlying() == nil {
		return ir.OXXX, ""
	}

	// 1. src type is identical to dst.
	if types.Identical(src, dst) {
		return ir.OCONVNOP, ""
	}

	// 2. src and dst have identical underlying types and
	//   a. either src or dst is not a named type, or
	//   b. both are empty interface types, or
	//   c. at least one is a gcshape type.
	// For assignable but different non-empty interface types,
	// we want to recompute the itab. Recomputing the itab ensures
	// that itabs are unique (thus an interface with a compile-time
	// type I has an itab with interface type I).
	if types.Identical(src.Underlying(), dst.Underlying()) {
		if src.IsEmptyInterface() {
			// Conversion between two empty interfaces
			// requires no code.
			return ir.OCONVNOP, ""
		}
		if (src.Sym() == nil || dst.Sym() == nil) && !src.IsInterface() {
			// Conversion between two types, at least one unnamed,
			// needs no conversion. The exception is nonempty interfaces
			// which need to have their itab updated.
			return ir.OCONVNOP, ""
		}
		if src.IsShape() || dst.IsShape() {
			// Conversion between a shape type and one of the types
			// it represents also needs no conversion.
			return ir.OCONVNOP, ""
		}
	}

	// 3. dst is an interface type and src implements dst.
	if dst.IsInterface() && src.Kind() != types.TNIL {
		if src.IsShape() {
			// Shape types implement things they have already
			// been typechecked to implement, even if they
			// don't have the methods for them.
			return ir.OCONVIFACE, ""
		}
		if src.HasShape() {
			// Unified IR uses OCONVIFACE for converting all derived types
			// to interface type, not just type arguments themselves.
			return ir.OCONVIFACE, ""
		}

		why := ImplementsExplain(src, dst)
		if why == "" {
			return ir.OCONVIFACE, ""
		}
		return ir.OXXX, ":\n\t" + why
	}

	if isptrto(dst, types.TINTER) {
		why := fmt.Sprintf(":\n\t%v is pointer to interface, not interface", dst)
		return ir.OXXX, why
	}

	if src.IsInterface() && dst.Kind() != types.TBLANK {
		var why string
		if Implements(dst, src) {
			why = ": need type assertion"
		}
		return ir.OXXX, why
	}

	// 4. src is a bidirectional channel value, dst is a channel type,
	// src and dst have identical element types, and
	// either src or dst is not a named type.
	if src.IsChan() && src.ChanDir() == types.Cboth && dst.IsChan() {
		if types.Identical(src.Elem(), dst.Elem()) && (src.Sym() == nil || dst.Sym() == nil) {
			return ir.OCONVNOP, ""
		}
	}

	// 5. src is the predeclared identifier nil and dst is a nillable type.
	if src.Kind() == types.TNIL {
		switch dst.Kind() {
		case types.TPTR,
			types.TFUNC,
			types.TMAP,
			types.TCHAN,
			types.TINTER,
			types.TSLICE:
			return ir.OCONVNOP, ""
		}
	}

	// 6. rule about untyped constants - already converted by DefaultLit.

	// 7. Any typed value can be assigned to the blank identifier.
	if dst.Kind() == types.TBLANK {
		return ir.OCONVNOP, ""
	}

	return ir.OXXX, ""
}

// Can we convert a value of type src to a value of type dst?
// If so, return op code to use in conversion (maybe OCONVNOP).
// If not, return OXXX. In this case, the string return parameter may
// hold a reason why. In all other cases, it'll be the empty string.
// srcConstant indicates whether the value of type src is a constant.
func convertOp(srcConstant bool, src, dst *types.Type) (ir.Op, string) {
	if src == dst {
		return ir.OCONVNOP, ""
	}
	if src == nil || dst == nil {
		return ir.OXXX, ""
	}

	// Conversions from regular to not-in-heap are not allowed
	// (unless it's unsafe.Pointer). These are runtime-specific
	// rules.
	// (a) Disallow (*T) to (*U) where T is not-in-heap but U isn't.
	if src.IsPtr() && dst.IsPtr() && dst.Elem().NotInHeap() && !src.Elem().NotInHeap() {
		why := fmt.Sprintf(":\n\t%v is incomplete (or unallocatable), but %v is not", dst.Elem(), src.Elem())
		return ir.OXXX, why
	}
	// (b) Disallow string to []T where T is not-in-heap.
	if src.IsString() && dst.IsSlice() && dst.Elem().NotInHeap() && (dst.Elem().Kind() == types.ByteType.Kind() || dst.Elem().Kind() == types.RuneType.Kind()) {
		why := fmt.Sprintf(":\n\t%v is incomplete (or unallocatable)", dst.Elem())
		return ir.OXXX, why
	}

	// 1. src can be assigned to dst.
	op, why := assignOp(src, dst)
	if op != ir.OXXX {
		return op, why
	}

	// The rules for interfaces are no different in conversions
	// than assignments. If interfaces are involved, stop now
	// with the good message from assignop.
	// Otherwise clear the error.
	if src.IsInterface() || dst.IsInterface() {
		return ir.OXXX, why
	}

	// 2. Ignoring struct tags, src and dst have identical underlying types.
	if types.IdenticalIgnoreTags(src.Underlying(), dst.Underlying()) {
		return ir.OCONVNOP, ""
	}

	// 3. src and dst are unnamed pointer types and, ignoring struct tags,
	// their base types have identical underlying types.
	if src.IsPtr() && dst.IsPtr() && src.Sym() == nil && dst.Sym() == nil {
		if types.IdenticalIgnoreTags(src.Elem().Underlying(), dst.Elem().Underlying()) {
			return ir.OCONVNOP, ""
		}
	}

	// 4. src and dst are both integer or floating point types.
	if (src.IsInteger() || src.IsFloat()) && (dst.IsInteger() || dst.IsFloat()) {
		if types.SimType[src.Kind()] == types.SimType[dst.Kind()] {
			return ir.OCONVNOP, ""
		}
		return ir.OCONV, ""
	}

	// 5. src and dst are both complex types.
	if src.IsComplex() && dst.IsComplex() {
		if types.SimType[src.Kind()] == types.SimType[dst.Kind()] {
			return ir.OCONVNOP, ""
		}
		return ir.OCONV, ""
	}

	// Special case for constant conversions: any numeric
	// conversion is potentially okay. We'll validate further
	// within evconst. See #38117.
	if srcConstant && (src.IsInteger() || src.IsFloat() || src.IsComplex()) && (dst.IsInteger() || dst.IsFloat() || dst.IsComplex()) {
		return ir.OCONV, ""
	}

	// 6. src is an integer or has type []byte or []rune
	// and dst is a string type.
	if src.IsInteger() && dst.IsString() {
		return ir.ORUNESTR, ""
	}

	if src.IsSlice() && dst.IsString() {
		if src.Elem().Kind() == types.ByteType.Kind() {
			return ir.OBYTES2STR, ""
		}
		if src.Elem().Kind() == types.RuneType.Kind() {
			return ir.ORUNES2STR, ""
		}
	}

	// 7. src is a string and dst is []byte or []rune.
	// String to slice.
	if src.IsString() && dst.IsSlice() {
		if dst.Elem().Kind() == types.ByteType.Kind() {
			return ir.OSTR2BYTES, ""
		}
		if dst.Elem().Kind() == types.RuneType.Kind() {
			return ir.OSTR2RUNES, ""
		}
	}

	// 8. src is a pointer or uintptr and dst is unsafe.Pointer.
	if (src.IsPtr() || src.IsUintptr()) && dst.IsUnsafePtr() {
		return ir.OCONVNOP, ""
	}

	// 9. src is unsafe.Pointer and dst is a pointer or uintptr.
	if src.IsUnsafePtr() && (dst.IsPtr() || dst.IsUintptr()) {
		return ir.OCONVNOP, ""
	}

	// 10. src is a slice and dst is an array or pointer-to-array.
	// They must have same element type.
	if src.IsSlice() {
		if dst.IsArray() && types.Identical(src.Elem(), dst.Elem()) {
			return ir.OSLICE2ARR, ""
		}
		if dst.IsPtr() && dst.Elem().IsArray() &&
			types.Identical(src.Elem(), dst.Elem().Elem()) {
			return ir.OSLICE2ARRPTR, ""
		}
	}

	return ir.OXXX, ""
}

// Code to resolve elided DOTs in embedded types.

// A dlist stores a pointer to a TFIELD Type embedded within
// a TSTRUCT or TINTER Type.
type dlist struct {
	field *types.Field
}

// dotpath computes the unique shortest explicit selector path to fully qualify
// a selection expression x.f, where x is of type t and f is the symbol s.
// If no such path exists, dotpath returns nil.
// If there are multiple shortest paths to the same depth, ambig is true.
func dotpath(s *types.Sym, t *types.Type, save **types.Field, ignorecase bool) (path []dlist, ambig bool) {
	// The embedding of types within structs imposes a tree structure onto
	// types: structs parent the types they embed, and types parent their
	// fields or methods. Our goal here is to find the shortest path to
	// a field or method named s in the subtree rooted at t. To accomplish
	// that, we iteratively perform depth-first searches of increasing depth
	// until we either find the named field/method or exhaust the tree.
	for d := 0; ; d++ {
		if d > len(dotlist) {
			dotlist = append(dotlist, dlist{})
		}
		if c, more := adddot1(s, t, d, save, ignorecase); c == 1 {
			return dotlist[:d], false
		} else if c > 1 {
			return nil, true
		} else if !more {
			return nil, false
		}
	}
}

func expand0(t *types.Type) {
	u := t
	if u.IsPtr() {
		u = u.Elem()
	}

	if u.IsInterface() {
		for _, f := range u.AllMethods() {
			if f.Sym.Uniq() {
				continue
			}
			f.Sym.SetUniq(true)
			slist = append(slist, symlink{field: f})
		}

		return
	}

	u = types.ReceiverBaseType(t)
	if u != nil {
		for _, f := range u.Methods() {
			if f.Sym.Uniq() {
				continue
			}
			f.Sym.SetUniq(true)
			slist = append(slist, symlink{field: f})
		}
	}
}

func expand1(t *types.Type, top bool) {
	if t.Recur() {
		return
	}
	t.SetRecur(true)

	if !top {
		expand0(t)
	}

	u := t
	if u.IsPtr() {
		u = u.Elem()
	}

	if u.IsStruct() || u.IsInterface() {
		var fields []*types.Field
		if u.IsStruct() {
			fields = u.Fields()
		} else {
			fields = u.AllMethods()
		}
		for _, f := range fields {
			if f.Embedded == 0 {
				continue
			}
			if f.Sym == nil {
				continue
			}
			expand1(f.Type, false)
		}
	}

	t.SetRecur(false)
}

func ifacelookdot(s *types.Sym, t *types.Type, ignorecase bool) *types.Field {
	if t == nil {
		return nil
	}

	var m *types.Field
	path, _ := dotpath(s, t, &m, ignorecase)
	if path == nil {
		return nil
	}

	if !m.IsMethod() {
		return nil
	}

	return m
}

// Implements reports whether t implements the interface iface. t can be
// an interface, a type parameter, or a concrete type.
func Implements(t, iface *types.Type) bool {
	var missing, have *types.Field
	var ptr int
	return implements(t, iface, &missing, &have, &ptr)
}

// ImplementsExplain reports whether t implements the interface iface. t can be
// an interface, a type parameter, or a concrete type. If t does not implement
// iface, a non-empty string is returned explaining why.
func ImplementsExplain(t, iface *types.Type) string {
	var missing, have *types.Field
	var ptr int
	if implements(t, iface, &missing, &have, &ptr) {
		return ""
	}

	if isptrto(t, types.TINTER) {
		return fmt.Sprintf("%v is pointer to interface, not interface", t)
	} else if have != nil && have.Sym == missing.Sym && have.Nointerface() {
		return fmt.Sprintf("%v does not implement %v (%v method is marked 'nointerface')", t, iface, missing.Sym)
	} else if have != nil && have.Sym == missing.Sym {
		return fmt.Sprintf("%v does not implement %v (wrong type for %v method)\n"+
			"\t\thave %v%S\n\t\twant %v%S", t, iface, missing.Sym, have.Sym, have.Type, missing.Sym, missing.Type)
	} else if ptr != 0 {
		return fmt.Sprintf("%v does not implement %v (%v method has pointer receiver)", t, iface, missing.Sym)
	} else if have != nil {
		return fmt.Sprintf("%v does not implement %v (missing %v method)\n"+
			"\t\thave %v%S\n\t\twant %v%S", t, iface, missing.Sym, have.Sym, have.Type, missing.Sym, missing.Type)
	}
	return fmt.Sprintf("%v does not implement %v (missing %v method)", t, iface, missing.Sym)
}

// implements reports whether t implements the interface iface. t can be
// an interface, a type parameter, or a concrete type. If implements returns
// false, it stores a method of iface that is not implemented in *m. If the
// method name matches but the type is wrong, it additionally stores the type
// of the method (on t) in *samename.
func implements(t, iface *types.Type, m, samename **types.Field, ptr *int) bool {
	t0 := t
	if t == nil {
		return false
	}

	if t.IsInterface() {
		i := 0
		tms := t.AllMethods()
		for _, im := range iface.AllMethods() {
			for i < len(tms) && tms[i].Sym != im.Sym {
				i++
			}
			if i == len(tms) {
				*m = im
				*samename = nil
				*ptr = 0
				return false
			}
			tm := tms[i]
			if !types.Identical(tm.Type, im.Type) {
				*m = im
				*samename = tm
				*ptr = 0
				return false
			}
		}

		return true
	}

	t = types.ReceiverBaseType(t)
	var tms []*types.Field
	if t != nil {
		CalcMethods(t)
		tms = t.AllMethods()
	}
	i := 0
	for _, im := range iface.AllMethods() {
		for i < len(tms) && tms[i].Sym != im.Sym {
			i++
		}
		if i == len(tms) {
			*m = im
			*samename = ifacelookdot(im.Sym, t, true)
			*ptr = 0
			return false
		}
		tm := tms[i]
		if tm.Nointerface() || !types.Identical(tm.Type, im.Type) {
			*m = im
			*samename = tm
			*ptr = 0
			return false
		}

		// if pointer receiver in method,
		// the method does not exist for value types.
		if !types.IsMethodApplicable(t0, tm) {
			if false && base.Flag.LowerR != 0 {
				base.Errorf("interface pointer mismatch")
			}

			*m = im
			*samename = nil
			*ptr = 1
			return false
		}
	}

	return true
}

func isptrto(t *types.Type, et types.Kind) bool {
	if t == nil {
		return false
	}
	if !t.IsPtr() {
		return false
	}
	t = t.Elem()
	if t == nil {
		return false
	}
	if t.Kind() != et {
		return false
	}
	return true
}

// lookdot0 returns the number of fields or methods named s associated
// with Type t. If exactly one exists, it will be returned in *save
// (if save is not nil).
func lookdot0(s *types.Sym, t *types.Type, save **types.Field, ignorecase bool) int {
	u := t
	if u.IsPtr() {
		u = u.Elem()
	}

	c := 0
	if u.IsStruct() || u.IsInterface() {
		var fields []*types.Field
		if u.IsStruct() {
			fields = u.Fields()
		} else {
			fields = u.AllMethods()
		}
		for _, f := range fields {
			if f.Sym == s || (ignorecase && f.IsMethod() && strings.EqualFold(f.Sym.Name, s.Name)) {
				if save != nil {
					*save = f
				}
				c++
			}
		}
	}

	u = t
	if t.Sym() != nil && t.IsPtr() && !t.Elem().IsPtr() {
		// If t is a defined pointer type, then x.m is shorthand for (*x).m.
		u = t.Elem()
	}
	u = types.ReceiverBaseType(u)
	if u != nil {
		for _, f := range u.Methods() {
			if f.Embedded == 0 && (f.Sym == s || (ignorecase && strings.EqualFold(f.Sym.Name, s.Name))) {
				if save != nil {
					*save = f
				}
				c++
			}
		}
	}

	return c
}

var slist []symlink

// Code to help generate trampoline functions for methods on embedded
// types. These are approx the same as the corresponding AddImplicitDots
// routines except that they expect to be called with unique tasks and
// they return the actual methods.

type symlink struct {
	field *types.Field
}

// FieldOffset returns the offset of field f in t,
// including any implicit offsets from embedded fields.
func FieldOffset(t *types.Type, f *types.Field) int64 {
	if f.Sym == nil {
		return f.Offset
	}
	path, ambig := dotpath(f.Sym, t, nil, false)
	if path == nil || ambig {
		return f.Offset
	}
	var offset int64
	for _, d := range path {
		offset += d.field.Offset
	}
	offset += f.Offset
	return offset
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/syms.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/obj"
)

// LookupRuntime returns a function or variable declared in
// _builtin/runtime.go. If types_ is non-empty, successive occurrences
// of the "any" placeholder type will be substituted.
func LookupRuntime(name string, types_ ...*types.Type) *ir.Name {
	s := ir.Pkgs.Runtime.Lookup(name)
	if s == nil || s.Def == nil {
		base.Fatalf("LookupRuntime: can't find runtime.%s", name)
	}
	n := s.Def.(*ir.Name)
	if len(types_) != 0 {
		n = substArgTypes(n, types_...)
	}
	return n
}

// SubstArgTypes substitutes the given list of types for
// successive occurrences of the "any" placeholder in the
// type syntax expression n.Type.
func substArgTypes(old *ir.Name, types_ ...*types.Type) *ir.Name {
	for _, t := range types_ {
		types.CalcSize(t)
	}
	n := ir.NewNameAt(old.Pos(), old.Sym(), types.SubstAny(old.Type(), &types_))
	n.Class = old.Class
	n.Func = old.Func
	if len(types_) > 0 {
		base.Fatalf("SubstArgTypes: too many argument types")
	}
	return n
}

// AutoLabel generates a new Name node for use with
// an automatically generated label.
// prefix is a short mnemonic (e.g. ".s" for switch)
// to help with debugging.
// It should begin with "." to avoid conflicts with
// user labels.
func AutoLabel(prefix string) *types.Sym {
	if prefix[0] != '.' {
		base.Fatalf("autolabel prefix must start with '.', have %q", prefix)
	}
	fn := ir.CurFunc
	if ir.CurFunc == nil {
		base.Fatalf("autolabel outside function")
	}
	n := fn.Label
	fn.Label++
	return LookupNum(prefix, int(n))
}

func Lookup(name string) *types.Sym {
	return types.LocalPkg.Lookup(name)
}

// InitRuntime loads the definitions for the low-level runtime functions,
// so that the compiler can generate calls to them,
// but does not make them visible to user code.
func InitRuntime() {
	base.Timer.Start("fe", "loadsys")

	typs := runtimeTypes()
	for _, d := range &runtimeDecls {
		sym := ir.Pkgs.Runtime.Lookup(d.name)
		typ := typs[d.typ]
		switch d.tag {
		case funcTag:
			importfunc(sym, typ)
		case varTag:
			importvar(sym, typ)
		default:
			base.Fatalf("unhandled declaration tag %v", d.tag)
		}
	}
}

// LookupRuntimeFunc looks up Go function name in package runtime. This function
// must follow the internal calling convention.
func LookupRuntimeFunc(name string) *obj.LSym {
	return LookupRuntimeABI(name, obj.ABIInternal)
}

// LookupRuntimeVar looks up a variable (or assembly function) name in package
// runtime. If this is a function, it may have a special calling
// convention.
func LookupRuntimeVar(name string) *obj.LSym {
	return LookupRuntimeABI(name, obj.ABI0)
}

// LookupRuntimeABI looks up a name in package runtime using the given ABI.
func LookupRuntimeABI(name string, abi obj.ABI) *obj.LSym {
	return base.PkgLinksym("runtime", name, abi)
}

// InitCoverage loads the definitions for routines called
// by code coverage instrumentation (similar to InitRuntime above).
func InitCoverage() {
	typs := coverageTypes()
	for _, d := range &coverageDecls {
		sym := ir.Pkgs.Coverage.Lookup(d.name)
		typ := typs[d.typ]
		switch d.tag {
		case funcTag:
			importfunc(sym, typ)
		case varTag:
			importvar(sym, typ)
		default:
			base.Fatalf("unhandled declaration tag %v", d.tag)
		}
	}
}

// LookupCoverage looks up the Go function 'name' in package
// runtime/coverage. This function must follow the internal calling
// convention.
func LookupCoverage(name string) *ir.Name {
	sym := ir.Pkgs.Coverage.Lookup(name)
	if sym == nil {
		base.Fatalf("LookupCoverage: can't find runtime/coverage.%s", name)
	}
	return sym.Def.(*ir.Name)
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/target.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run mkbuiltin.go

package typecheck

import "cmd/compile/internal/ir"

// Target is the package being compiled.
var Target *ir.Package

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/type.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/typecheck.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"fmt"
	"go/constant"
	"strings"

	"cmd/compile/internal/base"
	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

func AssignExpr(n ir.Node) ir.Node { return typecheck(n, ctxExpr|ctxAssign) }
func Expr(n ir.Node) ir.Node       { return typecheck(n, ctxExpr) }
func Stmt(n ir.Node) ir.Node       { return typecheck(n, ctxStmt) }

func Exprs(exprs []ir.Node) { typecheckslice(exprs, ctxExpr) }
func Stmts(stmts []ir.Node) { typecheckslice(stmts, ctxStmt) }

func Call(pos src.XPos, callee ir.Node, args []ir.Node, dots bool) ir.Node {
	call := ir.NewCallExpr(pos, ir.OCALL, callee, args)
	call.IsDDD = dots
	return typecheck(call, ctxStmt|ctxExpr)
}

func Callee(n ir.Node) ir.Node {
	return typecheck(n, ctxExpr|ctxCallee)
}

var traceIndent []byte

func tracePrint(title string, n ir.Node) func(np *ir.Node) {
	indent := traceIndent

	// guard against nil
	var pos, op string
	var tc uint8
	if n != nil {
		pos = base.FmtPos(n.Pos())
		op = n.Op().String()
		tc = n.Typecheck()
	}

	types.SkipSizeForTracing = true
	defer func() { types.SkipSizeForTracing = false }()
	fmt.Printf("%s: %s%s %p %s %v tc=%d\n", pos, indent, title, n, op, n, tc)
	traceIndent = append(traceIndent, ". "...)

	return func(np *ir.Node) {
		traceIndent = traceIndent[:len(traceIndent)-2]

		// if we have a result, use that
		if np != nil {
			n = *np
		}

		// guard against nil
		// use outer pos, op so we don't get empty pos/op if n == nil (nicer output)
		var tc uint8
		var typ *types.Type
		if n != nil {
			pos = base.FmtPos(n.Pos())
			op = n.Op().String()
			tc = n.Typecheck()
			typ = n.Type()
		}

		types.SkipSizeForTracing = true
		defer func() { types.SkipSizeForTracing = false }()
		fmt.Printf("%s: %s=> %p %s %v tc=%d type=%L\n", pos, indent, n, op, n, tc, typ)
	}
}

const (
	ctxStmt    = 1 << iota // evaluated at statement level
	ctxExpr                // evaluated in value context
	ctxType                // evaluated in type context
	ctxCallee              // call-only expressions are ok
	ctxMultiOK             // multivalue function returns are ok
	ctxAssign              // assigning to expression
)

// type checks the whole tree of an expression.
// calculates expression types.
// evaluates compile time constants.
// marks variables that escape the local frame.
// rewrites n.Op to be more specific in some cases.

func typecheckslice(l []ir.Node, top int) {
	for i := range l {
		l[i] = typecheck(l[i], top)
	}
}

var _typekind = []string{
	types.TINT:        "int",
	types.TUINT:       "uint",
	types.TINT8:       "int8",
	types.TUINT8:      "uint8",
	types.TINT16:      "int16",
	types.TUINT16:     "uint16",
	types.TINT32:      "int32",
	types.TUINT32:     "uint32",
	types.TINT64:      "int64",
	types.TUINT64:     "uint64",
	types.TUINTPTR:    "uintptr",
	types.TCOMPLEX64:  "complex64",
	types.TCOMPLEX128: "complex128",
	types.TFLOAT32:    "float32",
	types.TFLOAT64:    "float64",
	types.TBOOL:       "bool",
	types.TSTRING:     "string",
	types.TPTR:        "pointer",
	types.TUNSAFEPTR:  "unsafe.Pointer",
	types.TSTRUCT:     "struct",
	types.TINTER:      "interface",
	types.TCHAN:       "chan",
	types.TMAP:        "map",
	types.TARRAY:      "array",
	types.TSLICE:      "slice",
	types.TFUNC:       "func",
	types.TNIL:        "nil",
	types.TIDEAL:      "untyped number",
}

func typekind(t *types.Type) string {
	if t.IsUntyped() {
		return fmt.Sprintf("%v", t)
	}
	et := t.Kind()
	if int(et) < len(_typekind) {
		s := _typekind[et]
		if s != "" {
			return s
		}
	}
	return fmt.Sprintf("etype=%d", et)
}

// typecheck type checks node n.
// The result of typecheck MUST be assigned back to n, e.g.
//
//	n.Left = typecheck(n.Left, top)
func typecheck(n ir.Node, top int) (res ir.Node) {
	if n == nil {
		return nil
	}

	// only trace if there's work to do
	if base.EnableTrace && base.Flag.LowerT {
		defer tracePrint("typecheck", n)(&res)
	}

	lno := ir.SetPos(n)
	defer func() { base.Pos = lno }()

	// Skip typecheck if already done.
	// But re-typecheck ONAME/OTYPE/OLITERAL/OPACK node in case context has changed.
	if n.Typecheck() == 1 || n.Typecheck() == 3 {
		switch n.Op() {
		case ir.ONAME:
			break

		default:
			return n
		}
	}

	if n.Typecheck() == 2 {
		base.FatalfAt(n.Pos(), "typechecking loop")
	}

	n.SetTypecheck(2)
	n = typecheck1(n, top)
	n.SetTypecheck(1)

	t := n.Type()
	if t != nil && !t.IsFuncArgStruct() && n.Op() != ir.OTYPE {
		switch t.Kind() {
		case types.TFUNC, // might have TANY; wait until it's called
			types.TANY, types.TFORW, types.TIDEAL, types.TNIL, types.TBLANK:
			break

		default:
			types.CheckSize(t)
		}
	}

	return n
}

// indexlit implements typechecking of untyped values as
// array/slice indexes. It is almost equivalent to DefaultLit
// but also accepts untyped numeric values representable as
// value of type int (see also checkmake for comparison).
// The result of indexlit MUST be assigned back to n, e.g.
//
//	n.Left = indexlit(n.Left)
func indexlit(n ir.Node) ir.Node {
	if n != nil && n.Type() != nil && n.Type().Kind() == types.TIDEAL {
		return DefaultLit(n, types.Types[types.TINT])
	}
	return n
}

// typecheck1 should ONLY be called from typecheck.
func typecheck1(n ir.Node, top int) ir.Node {
	// Skip over parens.
	for n.Op() == ir.OPAREN {
		n = n.(*ir.ParenExpr).X
	}

	switch n.Op() {
	default:
		ir.Dump("typecheck", n)
		base.Fatalf("typecheck %v", n.Op())
		panic("unreachable")

	case ir.ONAME:
		n := n.(*ir.Name)
		if n.BuiltinOp != 0 {
			if top&ctxCallee == 0 {
				base.Errorf("use of builtin %v not in function call", n.Sym())
				n.SetType(nil)
				return n
			}
			return n
		}
		if top&ctxAssign == 0 {
			// not a write to the variable
			if ir.IsBlank(n) {
				base.Errorf("cannot use _ as value")
				n.SetType(nil)
				return n
			}
			n.SetUsed(true)
		}
		return n

	// type or expr
	case ir.ODEREF:
		n := n.(*ir.StarExpr)
		return tcStar(n, top)

	// x op= y
	case ir.OASOP:
		n := n.(*ir.AssignOpStmt)
		n.X, n.Y = Expr(n.X), Expr(n.Y)
		checkassign(n.X)
		if n.IncDec && !okforarith[n.X.Type().Kind()] {
			base.Errorf("invalid operation: %v (non-numeric type %v)", n, n.X.Type())
			return n
		}
		switch n.AsOp {
		case ir.OLSH, ir.ORSH:
			n.X, n.Y, _ = tcShift(n, n.X, n.Y)
		case ir.OADD, ir.OAND, ir.OANDNOT, ir.ODIV, ir.OMOD, ir.OMUL, ir.OOR, ir.OSUB, ir.OXOR:
			n.X, n.Y, _ = tcArith(n, n.AsOp, n.X, n.Y)
		default:
			base.Fatalf("invalid assign op: %v", n.AsOp)
		}
		return n

	// logical operators
	case ir.OANDAND, ir.OOROR:
		n := n.(*ir.LogicalExpr)
		n.X, n.Y = Expr(n.X), Expr(n.Y)
		if n.X.Type() == nil || n.Y.Type() == nil {
			n.SetType(nil)
			return n
		}
		// For "x == x && len(s)", it's better to report that "len(s)" (type int)
		// can't be used with "&&" than to report that "x == x" (type untyped bool)
		// can't be converted to int (see issue #41500).
		if !n.X.Type().IsBoolean() {
			base.Errorf("invalid operation: %v (operator %v not defined on %s)", n, n.Op(), typekind(n.X.Type()))
			n.SetType(nil)
			return n
		}
		if !n.Y.Type().IsBoolean() {
			base.Errorf("invalid operation: %v (operator %v not defined on %s)", n, n.Op(), typekind(n.Y.Type()))
			n.SetType(nil)
			return n
		}
		l, r, t := tcArith(n, n.Op(), n.X, n.Y)
		n.X, n.Y = l, r
		n.SetType(t)
		return n

	// shift operators
	case ir.OLSH, ir.ORSH:
		n := n.(*ir.BinaryExpr)
		n.X, n.Y = Expr(n.X), Expr(n.Y)
		l, r, t := tcShift(n, n.X, n.Y)
		n.X, n.Y = l, r
		n.SetType(t)
		return n

	// comparison operators
	case ir.OEQ, ir.OGE, ir.OGT, ir.OLE, ir.OLT, ir.ONE:
		n := n.(*ir.BinaryExpr)
		n.X, n.Y = Expr(n.X), Expr(n.Y)
		l, r, t := tcArith(n, n.Op(), n.X, n.Y)
		if t != nil {
			n.X, n.Y = l, r
			n.SetType(types.UntypedBool)
			n.X, n.Y = defaultlit2(l, r, true)
		}
		return n

	// binary operators
	case ir.OADD, ir.OAND, ir.OANDNOT, ir.ODIV, ir.OMOD, ir.OMUL, ir.OOR, ir.OSUB, ir.OXOR:
		n := n.(*ir.BinaryExpr)
		n.X, n.Y = Expr(n.X), Expr(n.Y)
		l, r, t := tcArith(n, n.Op(), n.X, n.Y)
		if t != nil && t.Kind() == types.TSTRING && n.Op() == ir.OADD {
			// create or update OADDSTR node with list of strings in x + y + z + (w + v) + ...
			var add *ir.AddStringExpr
			if l.Op() == ir.OADDSTR {
				add = l.(*ir.AddStringExpr)
				add.SetPos(n.Pos())
			} else {
				add = ir.NewAddStringExpr(n.Pos(), []ir.Node{l})
			}
			if r.Op() == ir.OADDSTR {
				r := r.(*ir.AddStringExpr)
				add.List.Append(r.List.Take()...)
			} else {
				add.List.Append(r)
			}
			add.SetType(t)
			return add
		}
		n.X, n.Y = l, r
		n.SetType(t)
		return n

	case ir.OBITNOT, ir.ONEG, ir.ONOT, ir.OPLUS:
		n := n.(*ir.UnaryExpr)
		return tcUnaryArith(n)

	// exprs
	case ir.OCOMPLIT:
		return tcCompLit(n.(*ir.CompLitExpr))

	case ir.OXDOT, ir.ODOT:
		n := n.(*ir.SelectorExpr)
		return tcDot(n, top)

	case ir.ODOTTYPE:
		n := n.(*ir.TypeAssertExpr)
		return tcDotType(n)

	case ir.OINDEX:
		n := n.(*ir.IndexExpr)
		return tcIndex(n)

	case ir.ORECV:
		n := n.(*ir.UnaryExpr)
		return tcRecv(n)

	case ir.OSEND:
		n := n.(*ir.SendStmt)
		return tcSend(n)

	case ir.OSLICEHEADER:
		n := n.(*ir.SliceHeaderExpr)
		return tcSliceHeader(n)

	case ir.OSTRINGHEADER:
		n := n.(*ir.StringHeaderExpr)
		return tcStringHeader(n)

	case ir.OMAKESLICECOPY:
		n := n.(*ir.MakeExpr)
		return tcMakeSliceCopy(n)

	case ir.OSLICE, ir.OSLICE3:
		n := n.(*ir.SliceExpr)
		return tcSlice(n)

	// call and call like
	case ir.OCALL:
		n := n.(*ir.CallExpr)
		return tcCall(n, top)

	case ir.OCAP, ir.OLEN:
		n := n.(*ir.UnaryExpr)
		return tcLenCap(n)

	case ir.OMIN, ir.OMAX:
		n := n.(*ir.CallExpr)
		return tcMinMax(n)

	case ir.OREAL, ir.OIMAG:
		n := n.(*ir.UnaryExpr)
		return tcRealImag(n)

	case ir.OCOMPLEX:
		n := n.(*ir.BinaryExpr)
		return tcComplex(n)

	case ir.OCLEAR:
		n := n.(*ir.UnaryExpr)
		return tcClear(n)

	case ir.OCLOSE:
		n := n.(*ir.UnaryExpr)
		return tcClose(n)

	case ir.ODELETE:
		n := n.(*ir.CallExpr)
		return tcDelete(n)

	case ir.OAPPEND:
		n := n.(*ir.CallExpr)
		return tcAppend(n)

	case ir.OCOPY:
		n := n.(*ir.BinaryExpr)
		return tcCopy(n)

	case ir.OCONV:
		n := n.(*ir.ConvExpr)
		return tcConv(n)

	case ir.OMAKE:
		n := n.(*ir.CallExpr)
		return tcMake(n)

	case ir.ONEW:
		n := n.(*ir.UnaryExpr)
		return tcNew(n)

	case ir.OPRINT, ir.OPRINTLN:
		n := n.(*ir.CallExpr)
		return tcPrint(n)

	case ir.OPANIC:
		n := n.(*ir.UnaryExpr)
		return tcPanic(n)

	case ir.ORECOVER:
		n := n.(*ir.CallExpr)
		return tcRecover(n)

	case ir.OUNSAFEADD:
		n := n.(*ir.BinaryExpr)
		return tcUnsafeAdd(n)

	case ir.OUNSAFESLICE:
		n := n.(*ir.BinaryExpr)
		return tcUnsafeSlice(n)

	case ir.OUNSAFESLICEDATA:
		n := n.(*ir.UnaryExpr)
		return tcUnsafeData(n)

	case ir.OUNSAFESTRING:
		n := n.(*ir.BinaryExpr)
		return tcUnsafeString(n)

	case ir.OUNSAFESTRINGDATA:
		n := n.(*ir.UnaryExpr)
		return tcUnsafeData(n)

	case ir.OITAB:
		n := n.(*ir.UnaryExpr)
		return tcITab(n)

	case ir.OIDATA:
		// Whoever creates the OIDATA node must know a priori the concrete type at that moment,
		// usually by just having checked the OITAB.
		n := n.(*ir.UnaryExpr)
		base.Fatalf("cannot typecheck interface data %v", n)
		panic("unreachable")

	case ir.OSPTR:
		n := n.(*ir.UnaryExpr)
		return tcSPtr(n)

	case ir.OCFUNC:
		n := n.(*ir.UnaryExpr)
		n.X = Expr(n.X)
		n.SetType(types.Types[types.TUINTPTR])
		return n

	case ir.OGETCALLERSP:
		n := n.(*ir.CallExpr)
		if len(n.Args) != 0 {
			base.FatalfAt(n.Pos(), "unexpected arguments: %v", n)
		}
		n.SetType(types.Types[types.TUINTPTR])
		return n

	case ir.OCONVNOP:
		n := n.(*ir.ConvExpr)
		n.X = Expr(n.X)
		return n

	// statements
	case ir.OAS:
		n := n.(*ir.AssignStmt)
		tcAssign(n)

		// Code that creates temps does not bother to set defn, so do it here.
		if n.X.Op() == ir.ONAME && ir.IsAutoTmp(n.X) {
			n.X.Name().Defn = n
		}
		return n

	case ir.OAS2:
		tcAssignList(n.(*ir.AssignListStmt))
		return n

	case ir.OBREAK,
		ir.OCONTINUE,
		ir.ODCL,
		ir.OGOTO,
		ir.OFALL:
		return n

	case ir.OBLOCK:
		n := n.(*ir.BlockStmt)
		Stmts(n.List)
		return n

	case ir.OLABEL:
		if n.Sym().IsBlank() {
			// Empty identifier is valid but useless.
			// Eliminate now to simplify life later.
			// See issues 7538, 11589, 11593.
			n = ir.NewBlockStmt(n.Pos(), nil)
		}
		return n

	case ir.ODEFER, ir.OGO:
		n := n.(*ir.GoDeferStmt)
		n.Call = typecheck(n.Call, ctxStmt|ctxExpr)
		tcGoDefer(n)
		return n

	case ir.OFOR:
		n := n.(*ir.ForStmt)
		return tcFor(n)

	case ir.OIF:
		n := n.(*ir.IfStmt)
		return tcIf(n)

	case ir.ORETURN:
		n := n.(*ir.ReturnStmt)
		return tcReturn(n)

	case ir.OTAILCALL:
		n := n.(*ir.TailCallStmt)
		n.Call = typecheck(n.Call, ctxStmt|ctxExpr).(*ir.CallExpr)
		return n

	case ir.OCHECKNIL:
		n := n.(*ir.UnaryExpr)
		return tcCheckNil(n)

	case ir.OSELECT:
		tcSelect(n.(*ir.SelectStmt))
		return n

	case ir.OSWITCH:
		tcSwitch(n.(*ir.SwitchStmt))
		return n

	case ir.ORANGE:
		tcRange(n.(*ir.RangeStmt))
		return n

	case ir.OTYPESW:
		n := n.(*ir.TypeSwitchGuard)
		base.Fatalf("use of .(type) outside type switch")
		return n

	case ir.ODCLFUNC:
		tcFunc(n.(*ir.Func))
		return n
	}

	// No return n here!
	// Individual cases can type-assert n, introducing a new one.
	// Each must execute its own return n.
}

func typecheckargs(n ir.InitNode) {
	var list []ir.Node
	switch n := n.(type) {
	default:
		base.Fatalf("typecheckargs %+v", n.Op())
	case *ir.CallExpr:
		list = n.Args
		if n.IsDDD {
			Exprs(list)
			return
		}
	case *ir.ReturnStmt:
		list = n.Results
	}
	if len(list) != 1 {
		Exprs(list)
		return
	}

	typecheckslice(list, ctxExpr|ctxMultiOK)
	t := list[0].Type()
	if t == nil || !t.IsFuncArgStruct() {
		return
	}

	// Rewrite f(g()) into t1, t2, ... = g(); f(t1, t2, ...).
	RewriteMultiValueCall(n, list[0])
}

// RewriteNonNameCall replaces non-Name call expressions with temps,
// rewriting f()(...) to t0 := f(); t0(...).
func RewriteNonNameCall(n *ir.CallExpr) {
	np := &n.Fun
	if dot, ok := (*np).(*ir.SelectorExpr); ok && (dot.Op() == ir.ODOTMETH || dot.Op() == ir.ODOTINTER || dot.Op() == ir.OMETHVALUE) {
		np = &dot.X // peel away method selector
	}

	// Check for side effects in the callee expression.
	// We explicitly special case new(T) though, because it doesn't have
	// observable side effects, and keeping it in place allows better escape analysis.
	if !ir.Any(*np, func(n ir.Node) bool { return n.Op() != ir.ONEW && callOrChan(n) }) {
		return
	}

	tmp := TempAt(base.Pos, ir.CurFunc, (*np).Type())
	as := ir.NewAssignStmt(base.Pos, tmp, *np)
	as.PtrInit().Append(Stmt(ir.NewDecl(n.Pos(), ir.ODCL, tmp)))
	*np = tmp

	n.PtrInit().Append(Stmt(as))
}

// RewriteMultiValueCall rewrites multi-valued f() to use temporaries,
// so the backend wouldn't need to worry about tuple-valued expressions.
func RewriteMultiValueCall(n ir.InitNode, call ir.Node) {
	as := ir.NewAssignListStmt(base.Pos, ir.OAS2, nil, []ir.Node{call})
	results := call.Type().Fields()
	list := make([]ir.Node, len(results))
	for i, result := range results {
		tmp := TempAt(base.Pos, ir.CurFunc, result.Type)
		as.PtrInit().Append(ir.NewDecl(base.Pos, ir.ODCL, tmp))
		as.Lhs.Append(tmp)
		list[i] = tmp
	}

	n.PtrInit().Append(Stmt(as))

	switch n := n.(type) {
	default:
		base.Fatalf("RewriteMultiValueCall %+v", n.Op())
	case *ir.CallExpr:
		n.Args = list
	case *ir.ReturnStmt:
		n.Results = list
	case *ir.AssignListStmt:
		if n.Op() != ir.OAS2FUNC {
			base.Fatalf("RewriteMultiValueCall: invalid op %v", n.Op())
		}
		as.SetOp(ir.OAS2FUNC)
		n.SetOp(ir.OAS2)
		n.Rhs = make([]ir.Node, len(list))
		for i, tmp := range list {
			n.Rhs[i] = AssignConv(tmp, n.Lhs[i].Type(), "assignment")
		}
	}
}

func checksliceindex(r ir.Node) bool {
	t := r.Type()
	if t == nil {
		return false
	}
	if !t.IsInteger() {
		base.Errorf("invalid slice index %v (type %v)", r, t)
		return false
	}
	return true
}

// The result of implicitstar MUST be assigned back to n, e.g.
//
//	n.Left = implicitstar(n.Left)
func implicitstar(n ir.Node) ir.Node {
	// insert implicit * if needed for fixed array
	t := n.Type()
	if t == nil || !t.IsPtr() {
		return n
	}
	t = t.Elem()
	if t == nil {
		return n
	}
	if !t.IsArray() {
		return n
	}
	star := ir.NewStarExpr(base.Pos, n)
	star.SetImplicit(true)
	return Expr(star)
}

func needOneArg(n *ir.CallExpr, f string, args ...any) (ir.Node, bool) {
	if len(n.Args) == 0 {
		p := fmt.Sprintf(f, args...)
		base.Errorf("missing argument to %s: %v", p, n)
		return nil, false
	}

	if len(n.Args) > 1 {
		p := fmt.Sprintf(f, args...)
		base.Errorf("too many arguments to %s: %v", p, n)
		return n.Args[0], false
	}

	return n.Args[0], true
}

func needTwoArgs(n *ir.CallExpr) (ir.Node, ir.Node, bool) {
	if len(n.Args) != 2 {
		if len(n.Args) < 2 {
			base.Errorf("not enough arguments in call to %v", n)
		} else {
			base.Errorf("too many arguments in call to %v", n)
		}
		return nil, nil, false
	}
	return n.Args[0], n.Args[1], true
}

// Lookdot1 looks up the specified method s in the list fs of methods, returning
// the matching field or nil. If dostrcmp is 0, it matches the symbols. If
// dostrcmp is 1, it matches by name exactly. If dostrcmp is 2, it matches names
// with case folding.
func Lookdot1(errnode ir.Node, s *types.Sym, t *types.Type, fs []*types.Field, dostrcmp int) *types.Field {
	var r *types.Field
	for _, f := range fs {
		if dostrcmp != 0 && f.Sym.Name == s.Name {
			return f
		}
		if dostrcmp == 2 && strings.EqualFold(f.Sym.Name, s.Name) {
			return f
		}
		if f.Sym != s {
			continue
		}
		if r != nil {
			if errnode != nil {
				base.Errorf("ambiguous selector %v", errnode)
			} else if t.IsPtr() {
				base.Errorf("ambiguous selector (%v).%v", t, s)
			} else {
				base.Errorf("ambiguous selector %v.%v", t, s)
			}
			break
		}

		r = f
	}

	return r
}

// NewMethodExpr returns an OMETHEXPR node representing method
// expression "recv.sym".
func NewMethodExpr(pos src.XPos, recv *types.Type, sym *types.Sym) *ir.SelectorExpr {
	// Compute the method set for recv.
	var ms []*types.Field
	if recv.IsInterface() {
		ms = recv.AllMethods()
	} else {
		mt := types.ReceiverBaseType(recv)
		if mt == nil {
			base.FatalfAt(pos, "type %v has no receiver base type", recv)
		}
		CalcMethods(mt)
		ms = mt.AllMethods()
	}

	m := Lookdot1(nil, sym, recv, ms, 0)
	if m == nil {
		base.FatalfAt(pos, "type %v has no method %v", recv, sym)
	}

	if !types.IsMethodApplicable(recv, m) {
		base.FatalfAt(pos, "invalid method expression %v.%v (needs pointer receiver)", recv, sym)
	}

	n := ir.NewSelectorExpr(pos, ir.OMETHEXPR, ir.TypeNode(recv), sym)
	n.Selection = m
	n.SetType(NewMethodType(m.Type, recv))
	n.SetTypecheck(1)
	return n
}

func derefall(t *types.Type) *types.Type {
	for t != nil && t.IsPtr() {
		t = t.Elem()
	}
	return t
}

// Lookdot looks up field or method n.Sel in the type t and returns the matching
// field. It transforms the op of node n to ODOTINTER or ODOTMETH, if appropriate.
// It also may add a StarExpr node to n.X as needed for access to non-pointer
// methods. If dostrcmp is 0, it matches the field/method with the exact symbol
// as n.Sel (appropriate for exported fields). If dostrcmp is 1, it matches by name
// exactly. If dostrcmp is 2, it matches names with case folding.
func Lookdot(n *ir.SelectorExpr, t *types.Type, dostrcmp int) *types.Field {
	s := n.Sel

	types.CalcSize(t)
	var f1 *types.Field
	if t.IsStruct() {
		f1 = Lookdot1(n, s, t, t.Fields(), dostrcmp)
	} else if t.IsInterface() {
		f1 = Lookdot1(n, s, t, t.AllMethods(), dostrcmp)
	}

	var f2 *types.Field
	if n.X.Type() == t || n.X.Type().Sym() == nil {
		mt := types.ReceiverBaseType(t)
		if mt != nil {
			f2 = Lookdot1(n, s, mt, mt.Methods(), dostrcmp)
		}
	}

	if f1 != nil {
		if dostrcmp > 1 {
			// Already in the process of diagnosing an error.
			return f1
		}
		if f2 != nil {
			base.Errorf("%v is both field and method", n.Sel)
		}
		if f1.Offset == types.BADWIDTH {
			base.Fatalf("Lookdot badwidth t=%v, f1=%v@%p", t, f1, f1)
		}
		n.Selection = f1
		n.SetType(f1.Type)
		if t.IsInterface() {
			if n.X.Type().IsPtr() {
				star := ir.NewStarExpr(base.Pos, n.X)
				star.SetImplicit(true)
				n.X = Expr(star)
			}

			n.SetOp(ir.ODOTINTER)
		}
		return f1
	}

	if f2 != nil {
		if dostrcmp > 1 {
			// Already in the process of diagnosing an error.
			return f2
		}
		orig := n.X
		tt := n.X.Type()
		types.CalcSize(tt)
		rcvr := f2.Type.Recv().Type
		if !types.Identical(rcvr, tt) {
			if rcvr.IsPtr() && types.Identical(rcvr.Elem(), tt) {
				checklvalue(n.X, "call pointer method on")
				addr := NodAddr(n.X)
				addr.SetImplicit(true)
				n.X = typecheck(addr, ctxType|ctxExpr)
			} else if tt.IsPtr() && (!rcvr.IsPtr() || rcvr.IsPtr() && rcvr.Elem().NotInHeap()) && types.Identical(tt.Elem(), rcvr) {
				star := ir.NewStarExpr(base.Pos, n.X)
				star.SetImplicit(true)
				n.X = typecheck(star, ctxType|ctxExpr)
			} else if tt.IsPtr() && tt.Elem().IsPtr() && types.Identical(derefall(tt), derefall(rcvr)) {
				base.Errorf("calling method %v with receiver %L requires explicit dereference", n.Sel, n.X)
				for tt.IsPtr() {
					// Stop one level early for method with pointer receiver.
					if rcvr.IsPtr() && !tt.Elem().IsPtr() {
						break
					}
					star := ir.NewStarExpr(base.Pos, n.X)
					star.SetImplicit(true)
					n.X = typecheck(star, ctxType|ctxExpr)
					tt = tt.Elem()
				}
			} else {
				base.Fatalf("method mismatch: %v for %v", rcvr, tt)
			}
		}

		// Check that we haven't implicitly dereferenced any defined pointer types.
		for x := n.X; ; {
			var inner ir.Node
			implicit := false
			switch x := x.(type) {
			case *ir.AddrExpr:
				inner, implicit = x.X, x.Implicit()
			case *ir.SelectorExpr:
				inner, implicit = x.X, x.Implicit()
			case *ir.StarExpr:
				inner, implicit = x.X, x.Implicit()
			}
			if !implicit {
				break
			}
			if inner.Type().Sym() != nil && (x.Op() == ir.ODEREF || x.Op() == ir.ODOTPTR) {
				// Found an implicit dereference of a defined pointer type.
				// Restore n.X for better error message.
				n.X = orig
				return nil
			}
			x = inner
		}

		n.Selection = f2
		n.SetType(f2.Type)
		n.SetOp(ir.ODOTMETH)

		return f2
	}

	return nil
}

func nokeys(l ir.Nodes) bool {
	for _, n := range l {
		if n.Op() == ir.OKEY || n.Op() == ir.OSTRUCTKEY {
			return false
		}
	}
	return true
}

func hasddd(params []*types.Field) bool {
	// TODO(mdempsky): Simply check the last param.
	for _, tl := range params {
		if tl.IsDDD() {
			return true
		}
	}

	return false
}

// typecheck assignment: type list = expression list
func typecheckaste(op ir.Op, call ir.Node, isddd bool, params []*types.Field, nl ir.Nodes, desc func() string) {
	var t *types.Type
	var i int

	lno := base.Pos
	defer func() { base.Pos = lno }()

	var n ir.Node
	if len(nl) == 1 {
		n = nl[0]
	}

	n1 := len(params)
	n2 := len(nl)
	if !hasddd(params) {
		if isddd {
			goto invalidddd
		}
		if n2 > n1 {
			goto toomany
		}
		if n2 < n1 {
			goto notenough
		}
	} else {
		if !isddd {
			if n2 < n1-1 {
				goto notenough
			}
		} else {
			if n2 > n1 {
				goto toomany
			}
			if n2 < n1 {
				goto notenough
			}
		}
	}

	i = 0
	for _, tl := range params {
		t = tl.Type
		if tl.IsDDD() {
			if isddd {
				if i >= len(nl) {
					goto notenough
				}
				if len(nl)-i > 1 {
					goto toomany
				}
				n = nl[i]
				ir.SetPos(n)
				if n.Type() != nil {
					nl[i] = assignconvfn(n, t, desc)
				}
				return
			}

			// TODO(mdempsky): Make into ... call with implicit slice.
			for ; i < len(nl); i++ {
				n = nl[i]
				ir.SetPos(n)
				if n.Type() != nil {
					nl[i] = assignconvfn(n, t.Elem(), desc)
				}
			}
			return
		}

		if i >= len(nl) {
			goto notenough
		}
		n = nl[i]
		ir.SetPos(n)
		if n.Type() != nil {
			nl[i] = assignconvfn(n, t, desc)
		}
		i++
	}

	if i < len(nl) {
		goto toomany
	}

invalidddd:
	if isddd {
		if call != nil {
			base.Errorf("invalid use of ... in call to %v", call)
		} else {
			base.Errorf("invalid use of ... in %v", op)
		}
	}
	return

notenough:
	if n == nil || n.Type() != nil {
		base.Fatalf("not enough arguments to %v", op)
	}
	return

toomany:
	base.Fatalf("too many arguments to %v", op)
}

// type check composite.
func fielddup(name string, hash map[string]bool) {
	if hash[name] {
		base.Errorf("duplicate field name in struct literal: %s", name)
		return
	}
	hash[name] = true
}

// typecheckarraylit type-checks a sequence of slice/array literal elements.
func typecheckarraylit(elemType *types.Type, bound int64, elts []ir.Node, ctx string) int64 {
	// If there are key/value pairs, create a map to keep seen
	// keys so we can check for duplicate indices.
	var indices map[int64]bool
	for _, elt := range elts {
		if elt.Op() == ir.OKEY {
			indices = make(map[int64]bool)
			break
		}
	}

	var key, length int64
	for i, elt := range elts {
		ir.SetPos(elt)
		r := elts[i]
		var kv *ir.KeyExpr
		if elt.Op() == ir.OKEY {
			elt := elt.(*ir.KeyExpr)
			elt.Key = Expr(elt.Key)
			key = IndexConst(elt.Key)
			kv = elt
			r = elt.Value
		}

		r = Expr(r)
		r = AssignConv(r, elemType, ctx)
		if kv != nil {
			kv.Value = r
		} else {
			elts[i] = r
		}

		if key >= 0 {
			if indices != nil {
				if indices[key] {
					base.Errorf("duplicate index in %s: %d", ctx, key)
				} else {
					indices[key] = true
				}
			}

			if bound >= 0 && key >= bound {
				base.Errorf("array index %d out of bounds [0:%d]", key, bound)
				bound = -1
			}
		}

		key++
		if key > length {
			length = key
		}
	}

	return length
}

// visible reports whether sym is exported or locally defined.
func visible(sym *types.Sym) bool {
	return sym != nil && (types.IsExported(sym.Name) || sym.Pkg == types.LocalPkg)
}

// nonexported reports whether sym is an unexported field.
func nonexported(sym *types.Sym) bool {
	return sym != nil && !types.IsExported(sym.Name)
}

func checklvalue(n ir.Node, verb string) {
	if !ir.IsAddressable(n) {
		base.Errorf("cannot %s %v", verb, n)
	}
}

func checkassign(n ir.Node) {
	// have already complained about n being invalid
	if n.Type() == nil {
		if base.Errors() == 0 {
			base.Fatalf("expected an error about %v", n)
		}
		return
	}

	if ir.IsAddressable(n) {
		return
	}
	if n.Op() == ir.OINDEXMAP {
		n := n.(*ir.IndexExpr)
		n.Assigned = true
		return
	}

	defer n.SetType(nil)

	switch {
	case n.Op() == ir.ODOT && n.(*ir.SelectorExpr).X.Op() == ir.OINDEXMAP:
		base.Errorf("cannot assign to struct field %v in map", n)
	case (n.Op() == ir.OINDEX && n.(*ir.IndexExpr).X.Type().IsString()) || n.Op() == ir.OSLICESTR:
		base.Errorf("cannot assign to %v (strings are immutable)", n)
	case n.Op() == ir.OLITERAL && n.Sym() != nil && ir.IsConstNode(n):
		base.Errorf("cannot assign to %v (declared const)", n)
	default:
		base.Errorf("cannot assign to %v", n)
	}
}

func checkassignto(src *types.Type, dst ir.Node) {
	// TODO(mdempsky): Handle all untyped types correctly.
	if src == types.UntypedBool && dst.Type().IsBoolean() {
		return
	}

	if op, why := assignOp(src, dst.Type()); op == ir.OXXX {
		base.Errorf("cannot assign %v to %L in multiple assignment%s", src, dst, why)
		return
	}
}

// The result of stringtoruneslit MUST be assigned back to n, e.g.
//
//	n.Left = stringtoruneslit(n.Left)
func stringtoruneslit(n *ir.ConvExpr) ir.Node {
	if n.X.Op() != ir.OLITERAL || n.X.Val().Kind() != constant.String {
		base.Fatalf("stringtoarraylit %v", n)
	}

	var l []ir.Node
	i := 0
	for _, r := range ir.StringVal(n.X) {
		l = append(l, ir.NewKeyExpr(base.Pos, ir.NewInt(base.Pos, int64(i)), ir.NewInt(base.Pos, int64(r))))
		i++
	}

	return Expr(ir.NewCompLitExpr(base.Pos, ir.OCOMPLIT, n.Type(), l))
}

func checkmake(t *types.Type, arg string, np *ir.Node) bool {
	n := *np
	if !n.Type().IsInteger() && n.Type().Kind() != types.TIDEAL {
		base.Errorf("non-integer %s argument in make(%v) - %v", arg, t, n.Type())
		return false
	}

	// DefaultLit is necessary for non-constants too: n might be 1.1<<k.
	// TODO(gri) The length argument requirements for (array/slice) make
	// are the same as for index expressions. Factor the code better;
	// for instance, indexlit might be called here and incorporate some
	// of the bounds checks done for make.
	n = DefaultLit(n, types.Types[types.TINT])
	*np = n

	return true
}

// checkunsafesliceorstring is like checkmake but for unsafe.{Slice,String}.
func checkunsafesliceorstring(op ir.Op, np *ir.Node) bool {
	n := *np
	if !n.Type().IsInteger() && n.Type().Kind() != types.TIDEAL {
		base.Errorf("non-integer len argument in %v - %v", op, n.Type())
		return false
	}

	// DefaultLit is necessary for non-constants too: n might be 1.1<<k.
	n = DefaultLit(n, types.Types[types.TINT])
	*np = n

	return true
}

func Conv(n ir.Node, t *types.Type) ir.Node {
	if types.IdenticalStrict(n.Type(), t) {
		return n
	}
	n = ir.NewConvExpr(base.Pos, ir.OCONV, nil, n)
	n.SetType(t)
	n = Expr(n)
	return n
}

// ConvNop converts node n to type t using the OCONVNOP op
// and typechecks the result with ctxExpr.
func ConvNop(n ir.Node, t *types.Type) ir.Node {
	if types.IdenticalStrict(n.Type(), t) {
		return n
	}
	n = ir.NewConvExpr(base.Pos, ir.OCONVNOP, nil, n)
	n.SetType(t)
	n = Expr(n)
	return n
}

```

// === FILE: references/go/src/cmd/compile/internal/typecheck/universe.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typecheck

import (
	"go/constant"

	"cmd/compile/internal/ir"
	"cmd/compile/internal/types"
	"cmd/internal/src"
)

var (
	okfor [ir.OEND][]bool
)

var (
	okforeq    [types.NTYPE]bool
	okforadd   [types.NTYPE]bool
	okforand   [types.NTYPE]bool
	okfornone  [types.NTYPE]bool
	okforbool  [types.NTYPE]bool
	okforcap   [types.NTYPE]bool
	okforlen   [types.NTYPE]bool
	okforarith [types.NTYPE]bool
)

var builtinFuncs = [...]struct {
	name string
	op   ir.Op
}{
	{"append", ir.OAPPEND},
	{"cap", ir.OCAP},
	{"clear", ir.OCLEAR},
	{"close", ir.OCLOSE},
	{"complex", ir.OCOMPLEX},
	{"copy", ir.OCOPY},
	{"delete", ir.ODELETE},
	{"imag", ir.OIMAG},
	{"len", ir.OLEN},
	{"make", ir.OMAKE},
	{"max", ir.OMAX},
	{"min", ir.OMIN},
	{"new", ir.ONEW},
	{"panic", ir.OPANIC},
	{"print", ir.OPRINT},
	{"println", ir.OPRINTLN},
	{"real", ir.OREAL},
	{"recover", ir.ORECOVER},
}

var unsafeFuncs = [...]struct {
	name string
	op   ir.Op
}{
	{"Add", ir.OUNSAFEADD},
	{"Slice", ir.OUNSAFESLICE},
	{"SliceData", ir.OUNSAFESLICEDATA},
	{"String", ir.OUNSAFESTRING},
	{"StringData", ir.OUNSAFESTRINGDATA},
}

// InitUniverse initializes the universe block.
func InitUniverse() {
	types.InitTypes(func(sym *types.Sym, typ *types.Type) types.Object {
		n := ir.NewDeclNameAt(src.NoXPos, ir.OTYPE, sym)
		n.SetType(typ)
		n.SetTypecheck(1)
		sym.Def = n
		return n
	})

	for _, s := range &builtinFuncs {
		ir.NewBuiltin(types.BuiltinPkg.Lookup(s.name), s.op)
	}

	for _, s := range &unsafeFuncs {
		ir.NewBuiltin(types.UnsafePkg.Lookup(s.name), s.op)
	}

	s := types.BuiltinPkg.Lookup("true")
	s.Def = ir.NewConstAt(src.NoXPos, s, types.UntypedBool, constant.MakeBool(true))

	s = types.BuiltinPkg.Lookup("false")
	s.Def = ir.NewConstAt(src.NoXPos, s, types.UntypedBool, constant.MakeBool(false))

	s = Lookup("_")
	types.BlankSym = s
	ir.BlankNode = ir.NewNameAt(src.NoXPos, s, types.Types[types.TBLANK])
	s.Def = ir.BlankNode

	s = types.BuiltinPkg.Lookup("_")
	s.Def = ir.NewNameAt(src.NoXPos, s, types.Types[types.TBLANK])

	s = types.BuiltinPkg.Lookup("nil")
	s.Def = NodNil()

	// initialize okfor
	for et := types.Kind(0); et < types.NTYPE; et++ {
		if types.IsInt[et] || et == types.TIDEAL {
			okforeq[et] = true
			types.IsOrdered[et] = true
			okforarith[et] = true
			okforadd[et] = true
			okforand[et] = true
			ir.OKForConst[et] = true
			types.IsSimple[et] = true
		}

		if types.IsFloat[et] {
			okforeq[et] = true
			types.IsOrdered[et] = true
			okforadd[et] = true
			okforarith[et] = true
			ir.OKForConst[et] = true
			types.IsSimple[et] = true
		}

		if types.IsComplex[et] {
			okforeq[et] = true
			okforadd[et] = true
			okforarith[et] = true
			ir.OKForConst[et] = true
			types.IsSimple[et] = true
		}
	}

	types.IsSimple[types.TBOOL] = true

	okforadd[types.TSTRING] = true

	okforbool[types.TBOOL] = true

	okforcap[types.TARRAY] = true
	okforcap[types.TCHAN] = true
	okforcap[types.TSLICE] = true

	ir.OKForConst[types.TBOOL] = true
	ir.OKForConst[types.TSTRING] = true

	okforlen[types.TARRAY] = true
	okforlen[types.TCHAN] = true
	okforlen[types.TMAP] = true
	okforlen[types.TSLICE] = true
	okforlen[types.TSTRING] = true

	okforeq[types.TPTR] = true
	okforeq[types.TUNSAFEPTR] = true
	okforeq[types.TINTER] = true
	okforeq[types.TCHAN] = true
	okforeq[types.TSTRING] = true
	okforeq[types.TBOOL] = true
	okforeq[types.TMAP] = true    // nil only; refined in typecheck
	okforeq[types.TFUNC] = true   // nil only; refined in typecheck
	okforeq[types.TSLICE] = true  // nil only; refined in typecheck
	okforeq[types.TARRAY] = true  // only if element type is comparable; refined in typecheck
	okforeq[types.TSTRUCT] = true // only if all struct fields are comparable; refined in typecheck

	types.IsOrdered[types.TSTRING] = true

	for i := range okfor {
		okfor[i] = okfornone[:]
	}

	// binary
	okfor[ir.OADD] = okforadd[:]
	okfor[ir.OAND] = okforand[:]
	okfor[ir.OANDAND] = okforbool[:]
	okfor[ir.OANDNOT] = okforand[:]
	okfor[ir.ODIV] = okforarith[:]
	okfor[ir.OEQ] = okforeq[:]
	okfor[ir.OGE] = types.IsOrdered[:]
	okfor[ir.OGT] = types.IsOrdered[:]
	okfor[ir.OLE] = types.IsOrdered[:]
	okfor[ir.OLT] = types.IsOrdered[:]
	okfor[ir.OMOD] = okforand[:]
	okfor[ir.OMUL] = okforarith[:]
	okfor[ir.ONE] = okforeq[:]
	okfor[ir.OOR] = okforand[:]
	okfor[ir.OOROR] = okforbool[:]
	okfor[ir.OSUB] = okforarith[:]
	okfor[ir.OXOR] = okforand[:]
	okfor[ir.OLSH] = okforand[:]
	okfor[ir.ORSH] = okforand[:]

	// unary
	okfor[ir.OBITNOT] = okforand[:]
	okfor[ir.ONEG] = okforarith[:]
	okfor[ir.ONOT] = okforbool[:]
	okfor[ir.OPLUS] = okforarith[:]

	// special
	okfor[ir.OCAP] = okforcap[:]
	okfor[ir.OLEN] = okforlen[:]
}

```

