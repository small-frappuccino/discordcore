# Domain Architecture: testing

## Layout Topology
```text
testing/
├── cryptotest
│   └── rand.go
├── fstest
│   ├── mapfs.go
│   └── testfs.go
├── internal
│   └── testdeps
│       └── deps.go
├── iotest
│   ├── logger.go
│   ├── reader.go
│   └── writer.go
├── quick
│   └── quick.go
├── slogtest
│   └── slogtest.go
├── synctest
│   └── synctest.go
├── allocs.go
├── benchmark.go
├── cover.go
├── example.go
├── fuzz.go
├── match.go
├── newcover.go
├── run_example.go
├── run_example_wasm.go
├── testing.go
├── testing_other.go
└── testing_windows.go
```

## Source Stream Aggregation

// === FILE: references/go/src/testing/allocs.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"runtime"
)

// AllocsPerRun returns the average number of allocations during calls to f.
// Although the return value has type float64, it will always be an integral value.
//
// To compute the number of allocations, the function will first be run once as
// a warm-up. The average number of allocations over the specified number of
// runs will then be measured and returned.
//
// AllocsPerRun sets [runtime.GOMAXPROCS] to 1 during its measurement and will restore
// it before returning.
func AllocsPerRun(runs int, f func()) (avg float64) {
	if parallelStart.Load() != parallelStop.Load() {
		panic("testing: AllocsPerRun called during parallel test")
	}
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))

	// Warm up the function
	f()

	// Measure the starting statistics
	var memstats runtime.MemStats
	runtime.ReadMemStats(&memstats)
	mallocs := 0 - memstats.Mallocs

	// Run the function the specified number of times
	for i := 0; i < runs; i++ {
		f()
	}

	// Read the final statistics
	runtime.ReadMemStats(&memstats)
	mallocs += memstats.Mallocs

	// Average the mallocs over the runs (not counting the warm-up).
	// We are forced to return a float64 because the API is silly, but do
	// the division as integers so we can ask if AllocsPerRun()==1
	// instead of AllocsPerRun()<2.
	return float64(mallocs / uint64(runs))
}

```

// === FILE: references/go/src/testing/benchmark.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"context"
	"flag"
	"fmt"
	"internal/sysinfo"
	"io"
	"math"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

func initBenchmarkFlags() {
	matchBenchmarks = flag.String("test.bench", "", "run only benchmarks matching `regexp`")
	benchmarkMemory = flag.Bool("test.benchmem", false, "print memory allocations for benchmarks")
	flag.Var(&benchTime, "test.benchtime", "run each benchmark for duration `d` or N times if `d` is of the form Nx")
}

var (
	matchBenchmarks *string
	benchmarkMemory *bool

	benchTime = durationOrCountFlag{d: 1 * time.Second} // changed during test of testing package
)

type durationOrCountFlag struct {
	d         time.Duration
	n         int
	allowZero bool
}

func (f *durationOrCountFlag) String() string {
	if f.n > 0 {
		return fmt.Sprintf("%dx", f.n)
	}
	return f.d.String()
}

func (f *durationOrCountFlag) Set(s string) error {
	if strings.HasSuffix(s, "x") {
		n, err := strconv.ParseInt(s[:len(s)-1], 10, 0)
		if err != nil || n < 0 || (!f.allowZero && n == 0) {
			return fmt.Errorf("invalid count")
		}
		*f = durationOrCountFlag{n: int(n)}
		return nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 || (!f.allowZero && d == 0) {
		return fmt.Errorf("invalid duration")
	}
	*f = durationOrCountFlag{d: d}
	return nil
}

// Global lock to ensure only one benchmark runs at a time.
var benchmarkLock sync.Mutex

// Used for every benchmark for measuring memory.
var memStats runtime.MemStats

// InternalBenchmark is an internal type but exported because it is cross-package;
// it is part of the implementation of the "go test" command.
type InternalBenchmark struct {
	Name string
	F    func(b *B)
}

// B is a type passed to [Benchmark] functions to manage benchmark
// timing and control the number of iterations.
//
// A benchmark ends when its Benchmark function returns or calls any of the methods
// [B.FailNow], [B.Fatal], [B.Fatalf], [B.SkipNow], [B.Skip], or [B.Skipf].
// Those methods must be called only from the goroutine running the Benchmark function.
// The other reporting methods, such as the variations of [B.Log] and [B.Error],
// may be called simultaneously from multiple goroutines.
//
// Like in tests, benchmark logs are accumulated during execution
// and dumped to standard output when done. Unlike in tests, benchmark logs
// are always printed, so as not to hide output whose existence may be
// affecting benchmark results.
type B struct {
	common
	importPath       string // import path of the package containing the benchmark
	bstate           *benchState
	N                int
	previousN        int           // number of iterations in the previous run
	previousDuration time.Duration // total duration of the previous run
	benchFunc        func(b *B)
	benchTime        durationOrCountFlag
	bytes            int64
	missingBytes     bool // one of the subbenchmarks does not have bytes set.
	timerOn          bool
	showAllocResult  bool
	result           BenchmarkResult
	parallelism      int // RunParallel creates parallelism*GOMAXPROCS goroutines
	// The initial states of memStats.Mallocs and memStats.TotalAlloc.
	startAllocs uint64
	startBytes  uint64
	// The net total of this test after being run.
	netAllocs uint64
	netBytes  uint64
	// Extra metrics collected by ReportMetric.
	extra map[string]float64

	// loop tracks the state of B.Loop
	loop struct {
		// n is the target number of iterations. It gets bumped up as we go.
		// When the benchmark loop is done, we commit this to b.N so users can
		// do reporting based on it, but we avoid exposing it until then.
		n uint64
		// i is the current Loop iteration. It's strictly monotonically
		// increasing toward n.
		//
		// The high bit is used to poison the Loop fast path and fall back to
		// the slow path.
		i uint64

		done bool // set when B.Loop return false
	}
}

// StartTimer starts timing a test. This function is called automatically
// before a benchmark starts, but it can also be used to resume timing after
// a call to [B.StopTimer].
func (b *B) StartTimer() {
	if !b.timerOn {
		runtime.ReadMemStats(&memStats)
		b.startAllocs = memStats.Mallocs
		b.startBytes = memStats.TotalAlloc
		b.start = highPrecisionTimeNow()
		b.timerOn = true
		b.loop.i &^= loopPoisonTimer
	}
}

// StopTimer stops timing a test. This can be used to pause the timer
// while performing steps that you don't want to measure.
func (b *B) StopTimer() {
	if b.timerOn {
		b.duration += highPrecisionTimeSince(b.start)
		runtime.ReadMemStats(&memStats)
		b.netAllocs += memStats.Mallocs - b.startAllocs
		b.netBytes += memStats.TotalAlloc - b.startBytes
		b.timerOn = false
		// If we hit B.Loop with the timer stopped, fail.
		b.loop.i |= loopPoisonTimer
	}
}

// ResetTimer zeroes the elapsed benchmark time and memory allocation counters
// and deletes user-reported metrics.
// It does not affect whether the timer is running.
func (b *B) ResetTimer() {
	if b.extra == nil {
		// Allocate the extra map before reading memory stats.
		// Pre-size it to make more allocation unlikely.
		b.extra = make(map[string]float64, 16)
	} else {
		clear(b.extra)
	}
	if b.timerOn {
		runtime.ReadMemStats(&memStats)
		b.startAllocs = memStats.Mallocs
		b.startBytes = memStats.TotalAlloc
		b.start = highPrecisionTimeNow()
	}
	b.duration = 0
	b.netAllocs = 0
	b.netBytes = 0
}

// SetBytes records the number of bytes processed in a single operation.
// If this is called, the benchmark will report ns/op and MB/s.
func (b *B) SetBytes(n int64) { b.bytes = n }

// ReportAllocs enables malloc statistics for this benchmark.
// It is equivalent to setting -test.benchmem, but it only affects the
// benchmark function that calls ReportAllocs.
func (b *B) ReportAllocs() {
	b.showAllocResult = true
}

// runN runs a single benchmark for the specified number of iterations.
func (b *B) runN(n int) {
	benchmarkLock.Lock()
	defer benchmarkLock.Unlock()
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer func() {
		b.runCleanup(normalPanic)
		b.checkRaces()
	}()
	// Try to get a comparable environment for each run
	// by clearing garbage from previous runs.
	runtime.GC()
	b.resetRaces()
	b.N = n
	b.loop.n = 0
	b.loop.i = 0
	b.loop.done = false
	b.ctx = ctx
	b.cancelCtx = cancelCtx

	b.parallelism = 1
	b.ResetTimer()
	b.StartTimer()
	b.benchFunc(b)
	b.StopTimer()
	b.previousN = n
	b.previousDuration = b.duration

	if b.loop.n > 0 && !b.loop.done && !b.failed {
		b.Error("benchmark function returned without B.Loop() == false (break or return in loop?)")
	}
}

// run1 runs the first iteration of benchFunc. It reports whether more
// iterations of this benchmarks should be run.
func (b *B) run1() bool {
	if bstate := b.bstate; bstate != nil {
		// Extend maxLen, if needed.
		if n := len(b.name) + bstate.extLen + 1; n > bstate.maxLen {
			bstate.maxLen = n + 8 // Add additional slack to avoid too many jumps in size.
		}
	}
	go func() {
		// Signal that we're done whether we return normally
		// or by FailNow's runtime.Goexit.
		defer func() {
			b.signal <- true
		}()

		b.runN(1)
	}()
	<-b.signal
	if b.failed {
		fmt.Fprintf(b.w, "%s--- FAIL: %s\n%s", b.chatty.prefix(), b.name, b.output)
		return false
	}
	// Only print the output if we know we are not going to proceed.
	// Otherwise it is printed in processBench.
	b.mu.RLock()
	finished := b.finished
	b.mu.RUnlock()
	if b.hasSub.Load() || finished {
		tag := "BENCH"
		if b.skipped {
			tag = "SKIP"
		}
		if b.chatty != nil && (len(b.output) > 0 || finished) {
			b.trimOutput()
			fmt.Fprintf(b.w, "%s--- %s: %s\n%s", b.chatty.prefix(), tag, b.name, b.output)
		}
		return false
	}
	return true
}

var labelsOnce sync.Once

// run executes the benchmark in a separate goroutine, including all of its
// subbenchmarks. b must not have subbenchmarks.
func (b *B) run() {
	labelsOnce.Do(func() {
		fmt.Fprintf(b.w, "goos: %s\n", runtime.GOOS)
		fmt.Fprintf(b.w, "goarch: %s\n", runtime.GOARCH)
		if b.importPath != "" {
			fmt.Fprintf(b.w, "pkg: %s\n", b.importPath)
		}
		if cpu := sysinfo.CPUName(); cpu != "" {
			fmt.Fprintf(b.w, "cpu: %s\n", cpu)
		}
	})
	if b.bstate != nil {
		// Running go test --test.bench
		b.bstate.processBench(b) // Must call doBench.
	} else {
		// Running func Benchmark.
		b.doBench()
	}
}

func (b *B) doBench() BenchmarkResult {
	go b.launch()
	<-b.signal
	return b.result
}

// Don't run more than 1e9 times. (This also keeps n in int range on 32 bit platforms.)
const maxBenchPredictIters = 1_000_000_000

func predictN(goalns int64, prevIters int64, prevns int64, last int64) int {
	if prevns == 0 {
		// Round up to dodge divide by zero. See https://go.dev/issue/70709.
		prevns = 1
	}

	// Order of operations matters.
	// For very fast benchmarks, prevIters ~= prevns.
	// If you divide first, you get 0 or 1,
	// which can hide an order of magnitude in execution time.
	// So multiply first, then divide.
	n := goalns * prevIters / prevns
	// Run more iterations than we think we'll need (1.2x).
	n += n / 5
	// Don't grow too fast in case we had timing errors previously.
	n = min(n, 100*last)
	// Be sure to run at least one more than last time.
	n = max(n, last+1)
	// Don't run more than 1e9 times. (This also keeps n in int range on 32 bit platforms.)
	n = min(n, maxBenchPredictIters)
	return int(n)
}

// launch launches the benchmark function. It gradually increases the number
// of benchmark iterations until the benchmark runs for the requested benchtime.
// launch is run by the doBench function as a separate goroutine.
// run1 must have been called on b.
func (b *B) launch() {
	// Signal that we're done whether we return normally
	// or by FailNow's runtime.Goexit.
	defer func() {
		b.signal <- true
	}()

	// b.Loop does its own ramp-up logic so we just need to run it once.
	// If b.loop.n is non zero, it means b.Loop has already run.
	if b.loop.n == 0 {
		// Run the benchmark for at least the specified amount of time.
		if b.benchTime.n > 0 {
			// We already ran a single iteration in run1.
			// If -benchtime=1x was requested, use that result.
			// See https://golang.org/issue/32051.
			if b.benchTime.n > 1 {
				b.runN(b.benchTime.n)
			}
		} else {
			d := b.benchTime.d
			for n := int64(1); !b.failed && b.duration < d && n < 1e9; {
				last := n
				// Predict required iterations.
				goalns := d.Nanoseconds()
				prevIters := int64(b.N)
				n = int64(predictN(goalns, prevIters, b.duration.Nanoseconds(), last))
				b.runN(int(n))
			}
		}
	}
	b.result = BenchmarkResult{b.N, b.duration, b.bytes, b.netAllocs, b.netBytes, b.extra}
}

// Elapsed returns the measured elapsed time of the benchmark.
// The duration reported by Elapsed matches the one measured by
// [B.StartTimer], [B.StopTimer], and [B.ResetTimer].
func (b *B) Elapsed() time.Duration {
	d := b.duration
	if b.timerOn {
		d += highPrecisionTimeSince(b.start)
	}
	return d
}

// ReportMetric adds "n unit" to the reported benchmark results.
// If the metric is per-iteration, the caller should divide by b.N,
// and by convention units should end in "/op".
// ReportMetric overrides any previously reported value for the same unit.
// ReportMetric panics if unit is the empty string or if unit contains
// any whitespace.
// If unit is a unit normally reported by the benchmark framework itself
// (such as "allocs/op"), ReportMetric will override that metric.
// Setting "ns/op" to 0 will suppress that built-in metric.
func (b *B) ReportMetric(n float64, unit string) {
	if unit == "" {
		panic("metric unit must not be empty")
	}
	if strings.IndexFunc(unit, unicode.IsSpace) >= 0 {
		panic("metric unit must not contain whitespace")
	}
	b.extra[unit] = n
}

func (b *B) stopOrScaleBLoop() bool {
	t := b.Elapsed()
	if t >= b.benchTime.d {
		// We've reached the target
		return false
	}
	// Loop scaling
	goalns := b.benchTime.d.Nanoseconds()
	prevIters := int64(b.loop.n)
	b.loop.n = uint64(predictN(goalns, prevIters, t.Nanoseconds(), prevIters))
	if b.loop.n&loopPoisonMask != 0 {
		// The iteration count should never get this high, but if it did we'd be
		// in big trouble.
		panic("loop iteration target overflow")
	}
	// predictN may have capped the number of iterations; make sure to
	// terminate if we've already hit that cap.
	return uint64(prevIters) < b.loop.n
}

func (b *B) loopSlowPath() bool {
	// Consistency checks
	if !b.timerOn {
		b.Fatal("B.Loop called with timer stopped")
	}
	if b.loop.i&loopPoisonMask != 0 {
		panic(fmt.Sprintf("unknown loop stop condition: %#x", b.loop.i))
	}

	if b.loop.n == 0 {
		// It's the first call to b.Loop() in the benchmark function.
		if b.benchTime.n > 0 {
			// Fixed iteration count.
			b.loop.n = uint64(b.benchTime.n)
		} else {
			// Initialize target to 1 to kick start loop scaling.
			b.loop.n = 1
		}
		// Within a b.Loop loop, we don't use b.N (to avoid confusion).
		b.N = 0
		b.ResetTimer()

		// Start the next iteration.
		b.loop.i++
		return true
	}

	// Should we keep iterating?
	var more bool
	if b.benchTime.n > 0 {
		// The iteration count is fixed, so we should have run this many and now
		// be done.
		if b.loop.i != uint64(b.benchTime.n) {
			// We shouldn't be able to reach the slow path in this case.
			panic(fmt.Sprintf("iteration count %d < fixed target %d", b.loop.i, b.benchTime.n))
		}
		more = false
	} else {
		// Handle fixed time case
		more = b.stopOrScaleBLoop()
	}
	if !more {
		b.StopTimer()
		// Commit iteration count
		b.N = int(b.loop.n)
		b.loop.done = true
		return false
	}

	// Start the next iteration.
	b.loop.i++
	return true
}

// Loop returns true as long as the benchmark should continue running.
//
// A typical benchmark is structured like:
//
//	func Benchmark(b *testing.B) {
//		... setup ...
//		for b.Loop() {
//			... code to measure ...
//		}
//		... cleanup ...
//	}
//
// Loop resets the benchmark timer the first time it is called in a benchmark,
// so any setup performed prior to starting the benchmark loop does not count
// toward the benchmark measurement. Likewise, when it returns false, it stops
// the timer so cleanup code is not measured.
//
// Within the body of a "for b.Loop() { ... }" loop, arguments to and
// results from function calls and assigned variables within the loop are kept
// alive, preventing the compiler from fully optimizing away the loop body.
// Currently, this is implemented as a compiler transformation that wraps such
// variables with a runtime.KeepAlive intrinsic call. This applies only to
// statements syntactically between the curly braces of the loop, and the loop
// condition must be written exactly as "b.Loop()".
//
// After Loop returns false, b.N contains the total number of iterations that
// ran, so the benchmark may use b.N to compute other average metrics.
//
// Prior to the introduction of Loop, benchmarks were expected to contain an
// explicit loop from 0 to b.N. Benchmarks should either use Loop or contain a
// loop to b.N, but not both. Loop offers more automatic management of the
// benchmark timer, and runs each benchmark function only once per measurement,
// whereas b.N-based benchmarks must run the benchmark function (and any
// associated setup and cleanup) several times.
func (b *B) Loop() bool {
	// This is written such that the fast path is as fast as possible and can be
	// inlined.
	//
	// There are three cases where we'll fall out of the fast path:
	//
	// - On the first call, both i and n are 0.
	//
	// - If the loop reaches the n'th iteration, then i == n and we need
	//   to figure out the new target iteration count or if we're done.
	//
	// - If the timer is stopped, it poisons the top bit of i so the slow
	//   path can do consistency checks and fail.
	if b.loop.i < b.loop.n {
		b.loop.i++
		return true
	}
	return b.loopSlowPath()
}

// The loopPoison constants can be OR'd into B.loop.i to cause it to fall back
// to the slow path.
const (
	loopPoisonTimer = uint64(1 << (63 - iota))
	// If necessary, add more poison bits here.

	// loopPoisonMask is the set of all loop poison bits. (iota-1) is the index
	// of the bit we just set, from which we recreate that bit mask. We subtract
	// 1 to set all of the bits below that bit, then complement the result to
	// get the mask. Sorry, not sorry.
	loopPoisonMask = ^uint64((1 << (63 - (iota - 1))) - 1)
)

// BenchmarkResult contains the results of a benchmark run.
type BenchmarkResult struct {
	N         int           // The number of iterations.
	T         time.Duration // The total time taken.
	Bytes     int64         // Bytes processed in one iteration.
	MemAllocs uint64        // The total number of memory allocations.
	MemBytes  uint64        // The total number of bytes allocated.

	// Extra records additional metrics reported by ReportMetric.
	Extra map[string]float64
}

// NsPerOp returns the "ns/op" metric.
func (r BenchmarkResult) NsPerOp() int64 {
	if v, ok := r.Extra["ns/op"]; ok {
		return int64(v)
	}
	if r.N <= 0 {
		return 0
	}
	return r.T.Nanoseconds() / int64(r.N)
}

// mbPerSec returns the "MB/s" metric.
func (r BenchmarkResult) mbPerSec() float64 {
	if v, ok := r.Extra["MB/s"]; ok {
		return v
	}
	if r.Bytes <= 0 || r.T <= 0 || r.N <= 0 {
		return 0
	}
	return (float64(r.Bytes) * float64(r.N) / 1e6) / r.T.Seconds()
}

// AllocsPerOp returns the "allocs/op" metric,
// which is calculated as r.MemAllocs / r.N.
func (r BenchmarkResult) AllocsPerOp() int64 {
	if v, ok := r.Extra["allocs/op"]; ok {
		return int64(v)
	}
	if r.N <= 0 {
		return 0
	}
	return int64(r.MemAllocs) / int64(r.N)
}

// AllocedBytesPerOp returns the "B/op" metric,
// which is calculated as r.MemBytes / r.N.
func (r BenchmarkResult) AllocedBytesPerOp() int64 {
	if v, ok := r.Extra["B/op"]; ok {
		return int64(v)
	}
	if r.N <= 0 {
		return 0
	}
	return int64(r.MemBytes) / int64(r.N)
}

// String returns a summary of the benchmark results.
// It follows the benchmark result line format from
// https://golang.org/design/14313-benchmark-format, not including the
// benchmark name.
// Extra metrics override built-in metrics of the same name.
// String does not include allocs/op or B/op, since those are reported
// by [BenchmarkResult.MemString].
func (r BenchmarkResult) String() string {
	buf := new(strings.Builder)
	fmt.Fprintf(buf, "%8d", r.N)

	// Get ns/op as a float.
	ns, ok := r.Extra["ns/op"]
	if !ok {
		ns = float64(r.T.Nanoseconds()) / float64(r.N)
	}
	if ns != 0 {
		buf.WriteByte('\t')
		prettyPrint(buf, ns, "ns/op")
	}

	if mbs := r.mbPerSec(); mbs != 0 {
		fmt.Fprintf(buf, "\t%7.2f MB/s", mbs)
	}

	// Print extra metrics that aren't represented in the standard
	// metrics.
	var extraKeys []string
	for k := range r.Extra {
		switch k {
		case "ns/op", "MB/s", "B/op", "allocs/op":
			// Built-in metrics reported elsewhere.
			continue
		}
		extraKeys = append(extraKeys, k)
	}
	slices.Sort(extraKeys)
	for _, k := range extraKeys {
		buf.WriteByte('\t')
		prettyPrint(buf, r.Extra[k], k)
	}
	return buf.String()
}

func prettyPrint(w io.Writer, x float64, unit string) {
	// Print all numbers with 10 places before the decimal point
	// and small numbers with four sig figs. Field widths are
	// chosen to fit the whole part in 10 places while aligning
	// the decimal point of all fractional formats.
	var format string
	switch y := math.Abs(x); {
	case y == 0 || y >= 999.95:
		format = "%10.0f %s"
	case y >= 99.995:
		format = "%12.1f %s"
	case y >= 9.9995:
		format = "%13.2f %s"
	case y >= 0.99995:
		format = "%14.3f %s"
	case y >= 0.099995:
		format = "%15.4f %s"
	case y >= 0.0099995:
		format = "%16.5f %s"
	case y >= 0.00099995:
		format = "%17.6f %s"
	default:
		format = "%18.7f %s"
	}
	fmt.Fprintf(w, format, x, unit)
}

// MemString returns r.AllocedBytesPerOp and r.AllocsPerOp in the same format as 'go test'.
func (r BenchmarkResult) MemString() string {
	return fmt.Sprintf("%8d B/op\t%8d allocs/op",
		r.AllocedBytesPerOp(), r.AllocsPerOp())
}

// benchmarkName returns full name of benchmark including procs suffix.
func benchmarkName(name string, n int) string {
	if n != 1 {
		return fmt.Sprintf("%s-%d", name, n)
	}
	return name
}

type benchState struct {
	match *matcher

	maxLen int // The largest recorded benchmark name.
	extLen int // Maximum extension length.
}

// RunBenchmarks is an internal function but exported because it is cross-package;
// it is part of the implementation of the "go test" command.
func RunBenchmarks(matchString func(pat, str string) (bool, error), benchmarks []InternalBenchmark) {
	runBenchmarks("", matchString, benchmarks)
}

func runBenchmarks(importPath string, matchString func(pat, str string) (bool, error), benchmarks []InternalBenchmark) bool {
	// If no flag was specified, don't run benchmarks.
	if len(*matchBenchmarks) == 0 {
		return true
	}
	// Collect matching benchmarks and determine longest name.
	maxprocs := 1
	for _, procs := range cpuList {
		if procs > maxprocs {
			maxprocs = procs
		}
	}
	bstate := &benchState{
		match:  newMatcher(matchString, *matchBenchmarks, "-test.bench", *skip),
		extLen: len(benchmarkName("", maxprocs)),
	}
	var bs []InternalBenchmark
	for _, Benchmark := range benchmarks {
		if _, matched, _ := bstate.match.fullName(nil, Benchmark.Name); matched {
			bs = append(bs, Benchmark)
			benchName := benchmarkName(Benchmark.Name, maxprocs)
			if l := len(benchName) + bstate.extLen + 1; l > bstate.maxLen {
				bstate.maxLen = l
			}
		}
	}
	main := &B{
		common: common{
			name:  "Main",
			w:     os.Stdout,
			bench: true,
		},
		importPath: importPath,
		benchFunc: func(b *B) {
			for _, Benchmark := range bs {
				b.Run(Benchmark.Name, Benchmark.F)
			}
		},
		benchTime: benchTime,
		bstate:    bstate,
	}
	if Verbose() {
		main.chatty = newChattyPrinter(main.w)
	}
	main.runN(1)
	return !main.failed
}

// processBench runs bench b for the configured CPU counts and prints the results.
func (s *benchState) processBench(b *B) {
	for i, procs := range cpuList {
		for j := uint(0); j < *count; j++ {
			runtime.GOMAXPROCS(procs)
			benchName := benchmarkName(b.name, procs)

			// If it's chatty, we've already printed this information.
			if b.chatty == nil {
				fmt.Fprintf(b.w, "%-*s\t", s.maxLen, benchName)
			}
			// Recompute the running time for all but the first iteration.
			if i > 0 || j > 0 {
				b = &B{
					common: common{
						signal: make(chan bool),
						name:   b.name,
						w:      b.w,
						chatty: b.chatty,
						bench:  true,
					},
					benchFunc: b.benchFunc,
					benchTime: b.benchTime,
				}
				b.setOutputWriter()
				b.run1()
			}
			r := b.doBench()
			if b.failed {
				// The output could be very long here, but probably isn't.
				// We print it all, regardless, because we don't want to trim the reason
				// the benchmark failed.
				fmt.Fprintf(b.w, "%s--- FAIL: %s\n%s", b.chatty.prefix(), benchName, b.output)
				continue
			}
			results := r.String()
			if b.chatty != nil {
				fmt.Fprintf(b.w, "%-*s\t", s.maxLen, benchName)
			}
			if *benchmarkMemory || b.showAllocResult {
				results += "\t" + r.MemString()
			}
			fmt.Fprintln(b.w, results)
			// Unlike with tests, we ignore the -chatty flag and always print output for
			// benchmarks since the output generation time will skew the results.
			if len(b.output) > 0 {
				b.trimOutput()
				fmt.Fprintf(b.w, "%s--- BENCH: %s\n%s", b.chatty.prefix(), benchName, b.output)
			}
			if p := runtime.GOMAXPROCS(-1); p != procs {
				fmt.Fprintf(os.Stderr, "testing: %s left GOMAXPROCS set to %d\n", benchName, p)
			}
			if b.chatty != nil && b.chatty.json {
				b.chatty.Updatef("", "=== NAME  %s\n", "")
			}
		}
	}
}

// If hideStdoutForTesting is true, Run does not print the benchName.
// This avoids a spurious print during 'go test' on package testing itself,
// which invokes b.Run in its own tests (see sub_test.go).
var hideStdoutForTesting = false

// Run benchmarks f as a subbenchmark with the given name. It reports
// whether there were any failures.
//
// A subbenchmark is like any other benchmark. A benchmark that calls Run at
// least once will not be measured itself and will be called once with N=1.
func (b *B) Run(name string, f func(b *B)) bool {
	// Since b has subbenchmarks, we will no longer run it as a benchmark itself.
	// Release the lock and acquire it on exit to ensure locks stay paired.
	b.hasSub.Store(true)
	benchmarkLock.Unlock()
	defer benchmarkLock.Lock()

	benchName, ok, partial := b.name, true, false
	if b.bstate != nil {
		benchName, ok, partial = b.bstate.match.fullName(&b.common, name)
	}
	if !ok {
		return true
	}
	var pc [maxStackLen]uintptr
	n := runtime.Callers(2, pc[:])
	sub := &B{
		common: common{
			signal:  make(chan bool),
			name:    benchName,
			parent:  &b.common,
			level:   b.level + 1,
			creator: pc[:n],
			w:       b.w,
			chatty:  b.chatty,
			bench:   true,
		},
		importPath: b.importPath,
		benchFunc:  f,
		benchTime:  b.benchTime,
		bstate:     b.bstate,
	}
	sub.setOutputWriter()
	if partial {
		// Partial name match, like -bench=X/Y matching BenchmarkX.
		// Only process sub-benchmarks, if any.
		sub.hasSub.Store(true)
	}

	if b.chatty != nil {
		labelsOnce.Do(func() {
			fmt.Printf("goos: %s\n", runtime.GOOS)
			fmt.Printf("goarch: %s\n", runtime.GOARCH)
			if b.importPath != "" {
				fmt.Printf("pkg: %s\n", b.importPath)
			}
			if cpu := sysinfo.CPUName(); cpu != "" {
				fmt.Printf("cpu: %s\n", cpu)
			}
		})

		if !hideStdoutForTesting {
			if b.chatty.json {
				b.chatty.Updatef(benchName, "=== RUN   %s\n", benchName)
			}
			fmt.Println(benchName)
		}
	}

	if sub.run1() {
		sub.run()
	}
	b.add(sub.result)
	return !sub.failed
}

// add simulates running benchmarks in sequence in a single iteration. It is
// used to give some meaningful results in case func Benchmark is used in
// combination with Run.
func (b *B) add(other BenchmarkResult) {
	r := &b.result
	// The aggregated BenchmarkResults resemble running all subbenchmarks as
	// in sequence in a single benchmark.
	r.N = 1
	r.T += time.Duration(other.NsPerOp())
	if other.Bytes == 0 {
		// Summing Bytes is meaningless in aggregate if not all subbenchmarks
		// set it.
		b.missingBytes = true
		r.Bytes = 0
	}
	if !b.missingBytes {
		r.Bytes += other.Bytes
	}
	r.MemAllocs += uint64(other.AllocsPerOp())
	r.MemBytes += uint64(other.AllocedBytesPerOp())
}

// trimOutput shortens the output from a benchmark, which can be very long.
func (b *B) trimOutput() {
	// The output is likely to appear multiple times because the benchmark
	// is run multiple times, but at least it will be seen. This is not a big deal
	// because benchmarks rarely print, but just in case, we trim it if it's too long.
	const maxNewlines = 10
	for nlCount, j := 0, 0; j < len(b.output); j++ {
		if b.output[j] == '\n' {
			nlCount++
			if nlCount >= maxNewlines {
				b.output = append(b.output[:j], "\n\t... [output truncated]\n"...)
				break
			}
		}
	}
}

// A PB is used by RunParallel for running parallel benchmarks.
type PB struct {
	globalN *atomic.Uint64 // shared between all worker goroutines iteration counter
	grain   uint64         // acquire that many iterations from globalN at once
	cache   uint64         // local cache of acquired iterations
	bN      uint64         // total number of iterations to execute (b.N)
}

// Next reports whether there are more iterations to execute.
func (pb *PB) Next() bool {
	if pb.cache == 0 {
		n := pb.globalN.Add(pb.grain)
		if n <= pb.bN {
			pb.cache = pb.grain
		} else if n < pb.bN+pb.grain {
			pb.cache = pb.bN + pb.grain - n
		} else {
			return false
		}
	}
	pb.cache--
	return true
}

// RunParallel runs a benchmark in parallel.
// It creates multiple goroutines and distributes b.N iterations among them.
// The number of goroutines defaults to GOMAXPROCS. To increase parallelism for
// non-CPU-bound benchmarks, call [B.SetParallelism] before RunParallel.
// RunParallel is usually used with the go test -cpu flag.
//
// The body function will be run in each goroutine. It should set up any
// goroutine-local state and then iterate until pb.Next returns false.
// It should not use the [B.StartTimer], [B.StopTimer], or [B.ResetTimer] functions,
// because they have global effect. It should also not call [B.Run].
//
// RunParallel reports ns/op values as wall time for the benchmark as a whole,
// not the sum of wall time or CPU time over each parallel goroutine.
func (b *B) RunParallel(body func(*PB)) {
	if b.N == 0 {
		return // Nothing to do when probing.
	}
	// Calculate grain size as number of iterations that take ~100µs.
	// 100µs is enough to amortize the overhead and provide sufficient
	// dynamic load balancing.
	grain := uint64(0)
	if b.previousN > 0 && b.previousDuration > 0 {
		grain = 1e5 * uint64(b.previousN) / uint64(b.previousDuration)
	}
	if grain < 1 {
		grain = 1
	}
	// We expect the inner loop and function call to take at least 10ns,
	// so do not do more than 100µs/10ns=1e4 iterations.
	if grain > 1e4 {
		grain = 1e4
	}

	var n atomic.Uint64
	numProcs := b.parallelism * runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	wg.Add(numProcs)
	for p := 0; p < numProcs; p++ {
		go func() {
			defer wg.Done()
			pb := &PB{
				globalN: &n,
				grain:   grain,
				bN:      uint64(b.N),
			}
			body(pb)
		}()
	}
	wg.Wait()
	if n.Load() <= uint64(b.N) && !b.Failed() {
		b.Fatal("RunParallel: body exited without pb.Next() == false")
	}
}

// SetParallelism sets the number of goroutines used by [B.RunParallel] to p*GOMAXPROCS.
// There is usually no need to call SetParallelism for CPU-bound benchmarks.
// If p is less than 1, this call will have no effect.
func (b *B) SetParallelism(p int) {
	if p >= 1 {
		b.parallelism = p
	}
}

// Benchmark benchmarks a single function. It is useful for creating
// custom benchmarks that do not use the "go test" command.
//
// If f depends on testing flags, then [Init] must be used to register
// those flags before calling Benchmark and before calling [flag.Parse].
//
// If f calls Run, the result will be an estimate of running all its
// subbenchmarks that don't call Run in sequence in a single benchmark.
func Benchmark(f func(b *B)) BenchmarkResult {
	b := &B{
		common: common{
			signal: make(chan bool),
			w:      discard{},
		},
		benchFunc: f,
		benchTime: benchTime,
	}
	b.setOutputWriter()
	if b.run1() {
		b.run()
	}
	return b.result
}

type discard struct{}

func (discard) Write(b []byte) (n int, err error) { return len(b), nil }

```

// === FILE: references/go/src/testing/cover.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Support for test coverage.

package testing

// CoverBlock records the coverage data for a single basic block.
// The fields are 1-indexed, as in an editor: The opening line of
// the file is number 1, for example. Columns are measured
// in bytes.
// NOTE: This struct is internal to the testing infrastructure and may change.
// It is not covered (yet) by the Go 1 compatibility guidelines.
type CoverBlock struct {
	Line0 uint32 // Line number for block start.
	Col0  uint16 // Column number for block start.
	Line1 uint32 // Line number for block end.
	Col1  uint16 // Column number for block end.
	Stmts uint16 // Number of statements included in this block.
}

// Cover records information about test coverage checking.
// NOTE: This struct is internal to the testing infrastructure and may change.
// It is not covered (yet) by the Go 1 compatibility guidelines.
type Cover struct {
	Mode            string
	Counters        map[string][]uint32
	Blocks          map[string][]CoverBlock
	CoveredPackages string
}

// RegisterCover records the coverage data accumulators for the tests.
// NOTE: This function is internal to the testing infrastructure and may change.
// It is not covered (yet) by the Go 1 compatibility guidelines.
func RegisterCover(c Cover) {
}

```

// === FILE: references/go/src/testing/cryptotest/rand.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cryptotest provides deterministic random source testing.
package cryptotest

import (
	cryptorand "crypto/rand"
	"internal/byteorder"
	"io"
	mathrand "math/rand/v2"
	"sync"
	"testing"

	// Import unsafe and crypto/rand, which imports crypto/internal/rand,
	// for the crypto/internal/rand.SetTestingReader go:linkname.
	_ "crypto/rand"
	_ "unsafe"
)

//go:linkname randSetTestingReader crypto/internal/rand.SetTestingReader
func randSetTestingReader(r io.Reader)

//go:linkname testingCheckParallel testing.checkParallel
func testingCheckParallel(t *testing.T)

// SetGlobalRandom sets a global, deterministic cryptographic randomness source
// for the duration of test t. It affects crypto/rand, and all implicit sources
// of cryptographic randomness in the crypto/... packages.
//
// SetGlobalRandom may be called multiple times in the same test to reset the
// random stream or change the seed.
//
// Because SetGlobalRandom affects the whole process, it cannot be used in
// parallel tests or tests with parallel ancestors.
//
// Note that the way cryptographic algorithms use randomness is generally not
// specified and may change over time. Thus, if a test expects a specific output
// from a cryptographic function, it may fail in the future even if it uses
// SetGlobalRandom.
//
// SetGlobalRandom is not supported when building against the Go Cryptographic
// Module v1.0.0 (i.e. when [crypto/fips140.Version] returns "v1.0.0").
func SetGlobalRandom(t *testing.T, seed uint64) {
	if t == nil {
		panic("cryptotest: SetGlobalRandom called with a nil *testing.T")
	}
	if !testing.Testing() {
		panic("cryptotest: SetGlobalRandom used in a non-test binary")
	}
	testingCheckParallel(t)

	var s [32]byte
	byteorder.LEPutUint64(s[:8], seed)
	r := &lockedReader{r: mathrand.NewChaCha8(s)}

	randSetTestingReader(r)
	previous := cryptorand.Reader
	cryptorand.Reader = r
	t.Cleanup(func() {
		cryptorand.Reader = previous
		randSetTestingReader(nil)
	})
}

type lockedReader struct {
	sync.Mutex
	r *mathrand.ChaCha8
}

func (lr *lockedReader) Read(b []byte) (n int, err error) {
	lr.Lock()
	defer lr.Unlock()
	return lr.r.Read(b)
}

```

// === FILE: references/go/src/testing/example.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"time"
)

type InternalExample struct {
	Name      string
	F         func()
	Output    string
	Unordered bool
}

// RunExamples is an internal function but exported because it is cross-package;
// it is part of the implementation of the "go test" command.
func RunExamples(matchString func(pat, str string) (bool, error), examples []InternalExample) (ok bool) {
	_, ok = runExamples(matchString, examples)
	return ok
}

func runExamples(matchString func(pat, str string) (bool, error), examples []InternalExample) (ran, ok bool) {
	ok = true

	m := newMatcher(matchString, *match, "-test.run", *skip)

	var eg InternalExample
	for _, eg = range examples {
		_, matched, _ := m.fullName(nil, eg.Name)
		if !matched {
			continue
		}
		ran = true
		if !runExample(eg) {
			ok = false
		}
	}

	return ran, ok
}

// processRunResult computes a summary and status of the result of running an example test.
// stdout is the captured output from stdout of the test.
// recovered is the result of invoking recover after running the test, in case it panicked.
//
// If stdout doesn't match the expected output or if recovered is non-nil, it'll print the cause of failure to stdout.
// If the test is chatty/verbose, it'll print a success message to stdout.
// If recovered is non-nil, it'll panic with that value.
// If the test panicked with nil, or invoked runtime.Goexit, it'll be
// made to fail and panic with errNilPanicOrGoexit
func (eg *InternalExample) processRunResult(stdout string, timeSpent time.Duration, finished bool, recovered any) (passed bool) {
	passed = true
	dstr := fmtDuration(timeSpent)
	var fail string
	got := strings.TrimSpace(stdout)
	want := strings.TrimSpace(eg.Output)
	if runtime.GOOS == "windows" {
		got = strings.ReplaceAll(got, "\r\n", "\n")
		want = strings.ReplaceAll(want, "\r\n", "\n")
	}
	if eg.Unordered {
		gotLines := slices.Sorted(strings.SplitSeq(got, "\n"))
		wantLines := slices.Sorted(strings.SplitSeq(want, "\n"))
		if !slices.Equal(gotLines, wantLines) && recovered == nil {
			fail = fmt.Sprintf("got:\n%s\nwant (unordered):\n%s\n", stdout, eg.Output)
		}
	} else {
		if got != want && recovered == nil {
			fail = fmt.Sprintf("got:\n%s\nwant:\n%s\n", got, want)
		}
	}
	if fail != "" || !finished || recovered != nil {
		fmt.Printf("%s--- FAIL: %s (%s)\n%s", chatty.prefix(), eg.Name, dstr, fail)
		passed = false
	} else if chatty.on {
		fmt.Printf("%s--- PASS: %s (%s)\n", chatty.prefix(), eg.Name, dstr)
	}

	if chatty.on && chatty.json {
		fmt.Printf("%s=== NAME   %s\n", chatty.prefix(), "")
	}

	if recovered != nil {
		// Propagate the previously recovered result, by panicking.
		panic(recovered)
	} else if !finished {
		panic(errNilPanicOrGoexit)
	}

	return
}

```

// === FILE: references/go/src/testing/fstest/mapfs.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fstest

import (
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"
)

// A MapFS is a simple in-memory file system for use in tests,
// represented as a map from path names (arguments to Open)
// to information about the files, directories, or symbolic links they represent.
//
// The map need not include parent directories for files contained
// in the map; those will be synthesized if needed.
// But a directory can still be included by setting the [MapFile.Mode]'s [fs.ModeDir] bit;
// this may be necessary for detailed control over the directory's [fs.FileInfo]
// or to create an empty directory.
//
// File system operations read directly from the map,
// so that the file system can be changed by editing the map as needed.
// An implication is that file system operations must not run concurrently
// with changes to the map, which would be a race.
// Another implication is that opening or reading a directory requires
// iterating over the entire map, so a MapFS should typically be used with not more
// than a few hundred entries or directory reads.
type MapFS map[string]*MapFile

// A MapFile describes a single file in a [MapFS].
type MapFile struct {
	Data    []byte      // file content or symlink destination
	Mode    fs.FileMode // fs.FileInfo.Mode
	ModTime time.Time   // fs.FileInfo.ModTime
	Sys     any         // fs.FileInfo.Sys
}

var _ fs.FS = MapFS(nil)
var _ fs.ReadLinkFS = MapFS(nil)
var _ fs.File = (*openMapFile)(nil)

// Open opens the named file after following any symbolic links.
func (fsys MapFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	realName, ok := fsys.resolveSymlinks(name)
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	file := fsys[realName]
	if file != nil && file.Mode&fs.ModeDir == 0 {
		// Ordinary file
		return &openMapFile{name, mapFileInfo{path.Base(name), file}, 0}, nil
	}

	// Directory, possibly synthesized.
	// Note that file can be nil here: the map need not contain explicit parent directories for all its files.
	// But file can also be non-nil, in case the user wants to set metadata for the directory explicitly.
	// Either way, we need to construct the list of children of this directory.
	var list []mapFileInfo
	var need = make(map[string]bool)
	if realName == "." {
		for fname, f := range fsys {
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					list = append(list, mapFileInfo{fname, f})
				}
			} else {
				need[fname[:i]] = true
			}
		}
	} else {
		prefix := realName + "/"
		for fname, f := range fsys {
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					list = append(list, mapFileInfo{felem, f})
				} else {
					need[fname[len(prefix):len(prefix)+i]] = true
				}
			}
		}
		// If the directory name is not in the map,
		// and there are no children of the name in the map,
		// then the directory is treated as not existing.
		if file == nil && list == nil && len(need) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
	}
	for _, fi := range list {
		delete(need, fi.name)
	}
	for name := range need {
		list = append(list, mapFileInfo{name, &MapFile{Mode: fs.ModeDir | 0555}})
	}
	slices.SortFunc(list, func(a, b mapFileInfo) int {
		return strings.Compare(a.name, b.name)
	})

	if file == nil {
		file = &MapFile{Mode: fs.ModeDir | 0555}
	}
	var elem string
	if name == "." {
		elem = "."
	} else {
		elem = name[strings.LastIndex(name, "/")+1:]
	}
	return &mapDir{name, mapFileInfo{elem, file}, list, 0}, nil
}

func (fsys MapFS) resolveSymlinks(name string) (_ string, ok bool) {
	// Fast path: if a symlink is in the map, resolve it.
	if file := fsys[name]; file != nil && file.Mode.Type() == fs.ModeSymlink {
		target := string(file.Data)
		if path.IsAbs(target) {
			return "", false
		}
		return fsys.resolveSymlinks(path.Join(path.Dir(name), target))
	}

	// Check if each parent directory (starting at root) is a symlink.
	for i := 0; i < len(name); {
		j := strings.Index(name[i:], "/")
		var dir string
		if j < 0 {
			dir = name
			i = len(name)
		} else {
			dir = name[:i+j]
			i += j
		}
		if file := fsys[dir]; file != nil && file.Mode.Type() == fs.ModeSymlink {
			target := string(file.Data)
			if path.IsAbs(target) {
				return "", false
			}
			return fsys.resolveSymlinks(path.Join(path.Dir(dir), target) + name[i:])
		}
		i += len("/")
	}
	return name, fs.ValidPath(name)
}

// ReadLink returns the destination of the named symbolic link.
func (fsys MapFS) ReadLink(name string) (string, error) {
	info, err := fsys.lstat(name)
	if err != nil {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: err}
	}
	if info.f.Mode.Type() != fs.ModeSymlink {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}
	return string(info.f.Data), nil
}

// Lstat returns a FileInfo describing the named file.
// If the file is a symbolic link, the returned FileInfo describes the symbolic link.
// Lstat makes no attempt to follow the link.
func (fsys MapFS) Lstat(name string) (fs.FileInfo, error) {
	info, err := fsys.lstat(name)
	if err != nil {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: err}
	}
	return info, nil
}

func (fsys MapFS) lstat(name string) (*mapFileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrNotExist
	}
	realDir, ok := fsys.resolveSymlinks(path.Dir(name))
	if !ok {
		return nil, fs.ErrNotExist
	}
	elem := path.Base(name)
	realName := path.Join(realDir, elem)

	file := fsys[realName]
	if file != nil {
		return &mapFileInfo{elem, file}, nil
	}

	if realName == "." {
		return &mapFileInfo{elem, &MapFile{Mode: fs.ModeDir | 0555}}, nil
	}
	// Maybe a directory.
	prefix := realName + "/"
	for fname := range fsys {
		if strings.HasPrefix(fname, prefix) {
			return &mapFileInfo{elem, &MapFile{Mode: fs.ModeDir | 0555}}, nil
		}
	}
	// If the directory name is not in the map,
	// and there are no children of the name in the map,
	// then the directory is treated as not existing.
	return nil, fs.ErrNotExist
}

// fsOnly is a wrapper that hides all but the fs.FS methods,
// to avoid an infinite recursion when implementing special
// methods in terms of helpers that would use them.
// (In general, implementing these methods using the package fs helpers
// is redundant and unnecessary, but having the methods may make
// MapFS exercise more code paths when used in tests.)
type fsOnly struct{ fs.FS }

func (fsys MapFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(fsOnly{fsys}, name)
}

func (fsys MapFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(fsOnly{fsys}, name)
}

func (fsys MapFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(fsOnly{fsys}, name)
}

func (fsys MapFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(fsOnly{fsys}, pattern)
}

type noSub struct {
	MapFS
}

func (noSub) Sub() {} // not the fs.SubFS signature

func (fsys MapFS) Sub(dir string) (fs.FS, error) {
	return fs.Sub(noSub{fsys}, dir)
}

// A mapFileInfo implements fs.FileInfo and fs.DirEntry for a given map file.
type mapFileInfo struct {
	name string
	f    *MapFile
}

func (i *mapFileInfo) Name() string               { return path.Base(i.name) }
func (i *mapFileInfo) Size() int64                { return int64(len(i.f.Data)) }
func (i *mapFileInfo) Mode() fs.FileMode          { return i.f.Mode }
func (i *mapFileInfo) Type() fs.FileMode          { return i.f.Mode.Type() }
func (i *mapFileInfo) ModTime() time.Time         { return i.f.ModTime }
func (i *mapFileInfo) IsDir() bool                { return i.f.Mode&fs.ModeDir != 0 }
func (i *mapFileInfo) Sys() any                   { return i.f.Sys }
func (i *mapFileInfo) Info() (fs.FileInfo, error) { return i, nil }

func (i *mapFileInfo) String() string {
	return fs.FormatFileInfo(i)
}

// An openMapFile is a regular (non-directory) fs.File open for reading.
type openMapFile struct {
	path string
	mapFileInfo
	offset int64
}

func (f *openMapFile) Stat() (fs.FileInfo, error) { return &f.mapFileInfo, nil }

func (f *openMapFile) Close() error { return nil }

func (f *openMapFile) Read(b []byte) (int, error) {
	if f.offset >= int64(len(f.f.Data)) {
		return 0, io.EOF
	}
	if f.offset < 0 {
		return 0, &fs.PathError{Op: "read", Path: f.path, Err: fs.ErrInvalid}
	}
	n := copy(b, f.f.Data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *openMapFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		// offset += 0
	case 1:
		offset += f.offset
	case 2:
		offset += int64(len(f.f.Data))
	}
	if offset < 0 || offset > int64(len(f.f.Data)) {
		return 0, &fs.PathError{Op: "seek", Path: f.path, Err: fs.ErrInvalid}
	}
	f.offset = offset
	return offset, nil
}

func (f *openMapFile) ReadAt(b []byte, offset int64) (int, error) {
	if offset < 0 || offset > int64(len(f.f.Data)) {
		return 0, &fs.PathError{Op: "read", Path: f.path, Err: fs.ErrInvalid}
	}
	n := copy(b, f.f.Data[offset:])
	if n < len(b) {
		return n, io.EOF
	}
	return n, nil
}

// A mapDir is a directory fs.File (so also an fs.ReadDirFile) open for reading.
type mapDir struct {
	path string
	mapFileInfo
	entry  []mapFileInfo
	offset int
}

func (d *mapDir) Stat() (fs.FileInfo, error) { return &d.mapFileInfo, nil }
func (d *mapDir) Close() error               { return nil }
func (d *mapDir) Read(b []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: fs.ErrInvalid}
}

func (d *mapDir) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.entry) - d.offset
	if n == 0 && count > 0 {
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = &d.entry[d.offset+i]
	}
	d.offset += n
	return list, nil
}

```

// === FILE: references/go/src/testing/fstest/testfs.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fstest implements support for testing implementations and users of file systems.
package fstest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"testing/iotest"
)

// TestFS tests a file system implementation.
// It walks the entire tree of files in fsys,
// opening and checking that each file behaves correctly.
// Symbolic links are not followed,
// but their Lstat values are checked
// if the file system implements [fs.ReadLinkFS].
// It also checks that the file system contains at least the expected files.
// As a special case, if no expected files are listed, fsys must be empty.
// Otherwise, fsys must contain at least the listed files; it can also contain others.
// The contents of fsys must not change concurrently with TestFS.
//
// If TestFS finds any misbehaviors, it returns either the first error or a
// list of errors. Use [errors.Is] or [errors.AsType] to inspect.
//
// Typical usage inside a test is:
//
//	if err := fstest.TestFS(myFS, "file/that/should/be/present"); err != nil {
//		t.Fatal(err)
//	}
func TestFS(fsys fs.FS, expected ...string) error {
	if err := testFS(fsys, expected...); err != nil {
		return err
	}
	for _, name := range expected {
		if i := strings.Index(name, "/"); i >= 0 {
			dir, dirSlash := name[:i], name[:i+1]
			var subExpected []string
			for _, name := range expected {
				if strings.HasPrefix(name, dirSlash) {
					subExpected = append(subExpected, name[len(dirSlash):])
				}
			}
			sub, err := fs.Sub(fsys, dir)
			if err != nil {
				return err
			}
			if err := testFS(sub, subExpected...); err != nil {
				return fmt.Errorf("testing fs.Sub(fsys, %s): %w", dir, err)
			}
			break // one sub-test is enough
		}
	}
	return nil
}

func testFS(fsys fs.FS, expected ...string) error {
	t := fsTester{fsys: fsys}
	t.checkDir(".")
	t.checkOpen(".")
	found := make(map[string]bool)
	for _, dir := range t.dirs {
		found[dir] = true
	}
	for _, file := range t.files {
		found[file] = true
	}
	delete(found, ".")
	if len(expected) == 0 && len(found) > 0 {
		list := slices.Sorted(maps.Keys(found))
		if len(list) > 15 {
			list = append(list[:10], "...")
		}
		t.errorf("expected empty file system but found files:\n%s", strings.Join(list, "\n"))
	}
	for _, name := range expected {
		if !found[name] {
			t.errorf("expected but not found: %s", name)
		}
	}
	if len(t.errors) == 0 {
		return nil
	}
	return fmt.Errorf("TestFS found errors:\n%w", errors.Join(t.errors...))
}

// An fsTester holds state for running the test.
type fsTester struct {
	fsys   fs.FS
	errors []error
	dirs   []string
	files  []string
}

// errorf adds an error to the list of errors.
func (t *fsTester) errorf(format string, args ...any) {
	t.errors = append(t.errors, fmt.Errorf(format, args...))
}

func (t *fsTester) openDir(dir string) fs.ReadDirFile {
	f, err := t.fsys.Open(dir)
	if err != nil {
		t.errorf("%s: Open: %w", dir, err)
		return nil
	}
	d, ok := f.(fs.ReadDirFile)
	if !ok {
		f.Close()
		t.errorf("%s: Open returned File type %T, not a fs.ReadDirFile", dir, f)
		return nil
	}
	return d
}

// checkDir checks the directory dir, which is expected to exist
// (it is either the root or was found in a directory listing with IsDir true).
func (t *fsTester) checkDir(dir string) {
	// Read entire directory.
	t.dirs = append(t.dirs, dir)
	d := t.openDir(dir)
	if d == nil {
		return
	}
	list, err := d.ReadDir(-1)
	if err != nil {
		d.Close()
		t.errorf("%s: ReadDir(-1): %w", dir, err)
		return
	}

	// Check all children.
	var prefix string
	if dir == "." {
		prefix = ""
	} else {
		prefix = dir + "/"
	}
	for _, info := range list {
		name := info.Name()
		switch {
		case name == ".", name == "..", name == "":
			t.errorf("%s: ReadDir: child has invalid name: %#q", dir, name)
			continue
		case strings.Contains(name, "/"):
			t.errorf("%s: ReadDir: child name contains slash: %#q", dir, name)
			continue
		case strings.Contains(name, `\`):
			t.errorf("%s: ReadDir: child name contains backslash: %#q", dir, name)
			continue
		}
		path := prefix + name
		t.checkStat(path, info)
		t.checkOpen(path)
		switch info.Type() {
		case fs.ModeDir:
			t.checkDir(path)
		case fs.ModeSymlink:
			// No further processing.
			// Avoid following symlinks to avoid potentially unbounded recursion.
			t.files = append(t.files, path)
		default:
			t.checkFile(path)
		}
	}

	// Check ReadDir(-1) at EOF.
	list2, err := d.ReadDir(-1)
	if len(list2) > 0 || err != nil {
		d.Close()
		t.errorf("%s: ReadDir(-1) at EOF = %d entries, %w, wanted 0 entries, nil", dir, len(list2), err)
		return
	}

	// Check ReadDir(1) at EOF (different results).
	list2, err = d.ReadDir(1)
	if len(list2) > 0 || err != io.EOF {
		d.Close()
		t.errorf("%s: ReadDir(1) at EOF = %d entries, %w, wanted 0 entries, EOF", dir, len(list2), err)
		return
	}

	// Check that close does not report an error.
	if err := d.Close(); err != nil {
		t.errorf("%s: Close: %w", dir, err)
	}

	// Check that closing twice doesn't crash.
	// The return value doesn't matter.
	d.Close()

	// Reopen directory, read a second time, make sure contents match.
	if d = t.openDir(dir); d == nil {
		return
	}
	defer d.Close()
	list2, err = d.ReadDir(-1)
	if err != nil {
		t.errorf("%s: second Open+ReadDir(-1): %w", dir, err)
		return
	}
	t.checkDirList(dir, "first Open+ReadDir(-1) vs second Open+ReadDir(-1)", list, list2)

	// Reopen directory, read a third time in pieces, make sure contents match.
	if d = t.openDir(dir); d == nil {
		return
	}
	defer d.Close()
	list2 = nil
	for {
		n := 1
		if len(list2) > 0 {
			n = 2
		}
		frag, err := d.ReadDir(n)
		if len(frag) > n {
			t.errorf("%s: third Open: ReadDir(%d) after %d: %d entries (too many)", dir, n, len(list2), len(frag))
			return
		}
		list2 = append(list2, frag...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.errorf("%s: third Open: ReadDir(%d) after %d: %w", dir, n, len(list2), err)
			return
		}
		if n == 0 {
			t.errorf("%s: third Open: ReadDir(%d) after %d: 0 entries but nil error", dir, n, len(list2))
			return
		}
	}
	t.checkDirList(dir, "first Open+ReadDir(-1) vs third Open+ReadDir(1,2) loop", list, list2)

	// If fsys has ReadDir, check that it matches and is sorted.
	if fsys, ok := t.fsys.(fs.ReadDirFS); ok {
		list2, err := fsys.ReadDir(dir)
		if err != nil {
			t.errorf("%s: fsys.ReadDir: %w", dir, err)
			return
		}
		t.checkDirList(dir, "first Open+ReadDir(-1) vs fsys.ReadDir", list, list2)

		for i := 0; i+1 < len(list2); i++ {
			if list2[i].Name() >= list2[i+1].Name() {
				t.errorf("%s: fsys.ReadDir: list not sorted: %s before %s", dir, list2[i].Name(), list2[i+1].Name())
			}
		}
	}

	// Check fs.ReadDir as well.
	list2, err = fs.ReadDir(t.fsys, dir)
	if err != nil {
		t.errorf("%s: fs.ReadDir: %w", dir, err)
		return
	}
	t.checkDirList(dir, "first Open+ReadDir(-1) vs fs.ReadDir", list, list2)

	for i := 0; i+1 < len(list2); i++ {
		if list2[i].Name() >= list2[i+1].Name() {
			t.errorf("%s: fs.ReadDir: list not sorted: %s before %s", dir, list2[i].Name(), list2[i+1].Name())
		}
	}

	t.checkGlob(dir, list2)
}

// formatEntry formats an fs.DirEntry into a string for error messages and comparison.
func formatEntry(entry fs.DirEntry) string {
	return fmt.Sprintf("%s IsDir=%v Type=%v", entry.Name(), entry.IsDir(), entry.Type())
}

// formatInfoEntry formats an fs.FileInfo into a string like the result of formatEntry, for error messages and comparison.
func formatInfoEntry(info fs.FileInfo) string {
	return fmt.Sprintf("%s IsDir=%v Type=%v", info.Name(), info.IsDir(), info.Mode().Type())
}

// formatInfo formats an fs.FileInfo into a string for error messages and comparison.
func formatInfo(info fs.FileInfo) string {
	return fmt.Sprintf("%s IsDir=%v Mode=%v Size=%d ModTime=%v", info.Name(), info.IsDir(), info.Mode(), info.Size(), info.ModTime())
}

// checkGlob checks that various glob patterns work if the file system implements GlobFS.
func (t *fsTester) checkGlob(dir string, list []fs.DirEntry) {
	if _, ok := t.fsys.(fs.GlobFS); !ok {
		return
	}

	// Make a complex glob pattern prefix that only matches dir.
	var glob string
	if dir != "." {
		elem := strings.Split(dir, "/")
		for i, e := range elem {
			var pattern []rune
			for j, r := range e {
				if r == '*' || r == '?' || r == '\\' || r == '[' || r == '-' {
					pattern = append(pattern, '\\', r)
					continue
				}
				switch (i + j) % 5 {
				case 0:
					pattern = append(pattern, r)
				case 1:
					pattern = append(pattern, '[', r, ']')
				case 2:
					pattern = append(pattern, '[', r, '-', r, ']')
				case 3:
					pattern = append(pattern, '[', '\\', r, ']')
				case 4:
					pattern = append(pattern, '[', '\\', r, '-', '\\', r, ']')
				}
			}
			elem[i] = string(pattern)
		}
		glob = strings.Join(elem, "/") + "/"
	}

	// Test that malformed patterns are detected.
	// The error is likely path.ErrBadPattern but need not be.
	if _, err := t.fsys.(fs.GlobFS).Glob(glob + "nonexist/[]"); err == nil {
		t.errorf("%s: Glob(%#q): bad pattern not detected", dir, glob+"nonexist/[]")
	}

	// Try to find a letter that appears in only some of the final names.
	c := rune('a')
	for ; c <= 'z'; c++ {
		have, haveNot := false, false
		for _, d := range list {
			if strings.ContainsRune(d.Name(), c) {
				have = true
			} else {
				haveNot = true
			}
		}
		if have && haveNot {
			break
		}
	}
	if c > 'z' {
		c = 'a'
	}
	glob += "*" + string(c) + "*"

	var want []string
	for _, d := range list {
		if strings.ContainsRune(d.Name(), c) {
			want = append(want, path.Join(dir, d.Name()))
		}
	}

	names, err := t.fsys.(fs.GlobFS).Glob(glob)
	if err != nil {
		t.errorf("%s: Glob(%#q): %w", dir, glob, err)
		return
	}
	if slices.Equal(want, names) {
		return
	}

	if !slices.IsSorted(names) {
		t.errorf("%s: Glob(%#q): unsorted output:\n%s", dir, glob, strings.Join(names, "\n"))
		slices.Sort(names)
	}

	var problems []string
	for len(want) > 0 || len(names) > 0 {
		switch {
		case len(want) > 0 && len(names) > 0 && want[0] == names[0]:
			want, names = want[1:], names[1:]
		case len(want) > 0 && (len(names) == 0 || want[0] < names[0]):
			problems = append(problems, "missing: "+want[0])
			want = want[1:]
		default:
			problems = append(problems, "extra: "+names[0])
			names = names[1:]
		}
	}
	t.errorf("%s: Glob(%#q): wrong output:\n%s", dir, glob, strings.Join(problems, "\n"))
}

// checkStat checks that a direct stat of path matches entry,
// which was found in the parent's directory listing.
func (t *fsTester) checkStat(path string, entry fs.DirEntry) {
	file, err := t.fsys.Open(path)
	if err != nil {
		t.errorf("%s: Open: %w", path, err)
		return
	}
	info, err := file.Stat()
	file.Close()
	if err != nil {
		t.errorf("%s: Stat: %w", path, err)
		return
	}
	fentry := formatEntry(entry)
	fientry := formatInfoEntry(info)
	// Note: mismatch here is OK for symlink, because Open dereferences symlink.
	if fentry != fientry && entry.Type()&fs.ModeSymlink == 0 {
		t.errorf("%s: mismatch:\n\tentry = %s\n\tfile.Stat() = %s", path, fentry, fientry)
	}

	einfo, err := entry.Info()
	if err != nil {
		t.errorf("%s: entry.Info: %w", path, err)
		return
	}
	finfo := formatInfo(info)
	if entry.Type()&fs.ModeSymlink != 0 {
		// For symlink, just check that entry.Info matches entry on common fields.
		// Open deferences symlink, so info itself may differ.
		feentry := formatInfoEntry(einfo)
		if fentry != feentry {
			t.errorf("%s: mismatch\n\tentry = %s\n\tentry.Info() = %s\n", path, fentry, feentry)
		}
	} else {
		feinfo := formatInfo(einfo)
		if feinfo != finfo {
			t.errorf("%s: mismatch:\n\tentry.Info() = %s\n\tfile.Stat() = %s\n", path, feinfo, finfo)
		}
	}

	// Stat should be the same as Open+Stat, even for symlinks.
	info2, err := fs.Stat(t.fsys, path)
	if err != nil {
		t.errorf("%s: fs.Stat: %w", path, err)
		return
	}
	finfo2 := formatInfo(info2)
	if finfo2 != finfo {
		t.errorf("%s: fs.Stat(...) = %s\n\twant %s", path, finfo2, finfo)
	}

	if fsys, ok := t.fsys.(fs.StatFS); ok {
		info2, err := fsys.Stat(path)
		if err != nil {
			t.errorf("%s: fsys.Stat: %w", path, err)
			return
		}
		finfo2 := formatInfo(info2)
		if finfo2 != finfo {
			t.errorf("%s: fsys.Stat(...) = %s\n\twant %s", path, finfo2, finfo)
		}
	}

	if fsys, ok := t.fsys.(fs.ReadLinkFS); ok {
		info2, err := fsys.Lstat(path)
		if err != nil {
			t.errorf("%s: fsys.Lstat: %v", path, err)
			return
		}
		fientry2 := formatInfoEntry(info2)
		if fentry != fientry2 {
			t.errorf("%s: mismatch:\n\tentry = %s\n\tfsys.Lstat(...) = %s", path, fentry, fientry2)
		}
		feinfo := formatInfo(einfo)
		finfo2 := formatInfo(info2)
		if feinfo != finfo2 {
			t.errorf("%s: mismatch:\n\tentry.Info() = %s\n\tfsys.Lstat(...) = %s\n", path, feinfo, finfo2)
		}
	}
}

// checkDirList checks that two directory lists contain the same files and file info.
// The order of the lists need not match.
func (t *fsTester) checkDirList(dir, desc string, list1, list2 []fs.DirEntry) {
	old := make(map[string]fs.DirEntry)
	checkMode := func(entry fs.DirEntry) {
		if entry.IsDir() != (entry.Type()&fs.ModeDir != 0) {
			if entry.IsDir() {
				t.errorf("%s: ReadDir returned %s with IsDir() = true, Type() & ModeDir = 0", dir, entry.Name())
			} else {
				t.errorf("%s: ReadDir returned %s with IsDir() = false, Type() & ModeDir = ModeDir", dir, entry.Name())
			}
		}
	}

	for _, entry1 := range list1 {
		old[entry1.Name()] = entry1
		checkMode(entry1)
	}

	var diffs []string
	for _, entry2 := range list2 {
		entry1 := old[entry2.Name()]
		if entry1 == nil {
			checkMode(entry2)
			diffs = append(diffs, "+ "+formatEntry(entry2))
			continue
		}
		if formatEntry(entry1) != formatEntry(entry2) {
			diffs = append(diffs, "- "+formatEntry(entry1), "+ "+formatEntry(entry2))
		}
		delete(old, entry2.Name())
	}
	for _, entry1 := range old {
		diffs = append(diffs, "- "+formatEntry(entry1))
	}

	if len(diffs) == 0 {
		return
	}

	slices.SortFunc(diffs, func(a, b string) int {
		fa := strings.Fields(a)
		fb := strings.Fields(b)
		// sort by name (i < j) and then +/- (j < i, because + < -)
		return strings.Compare(fa[1]+" "+fb[0], fb[1]+" "+fa[0])
	})

	t.errorf("%s: diff %s:\n\t%s", dir, desc, strings.Join(diffs, "\n\t"))
}

// checkFile checks that basic file reading works correctly.
func (t *fsTester) checkFile(file string) {
	t.files = append(t.files, file)

	// Read entire file.
	f, err := t.fsys.Open(file)
	if err != nil {
		t.errorf("%s: Open: %w", file, err)
		return
	}

	data, err := io.ReadAll(f)
	if err != nil {
		f.Close()
		t.errorf("%s: Open+ReadAll: %w", file, err)
		return
	}

	if err := f.Close(); err != nil {
		t.errorf("%s: Close: %w", file, err)
	}

	// Check that closing twice doesn't crash.
	// The return value doesn't matter.
	f.Close()

	// Check that ReadFile works if present.
	if fsys, ok := t.fsys.(fs.ReadFileFS); ok {
		data2, err := fsys.ReadFile(file)
		if err != nil {
			t.errorf("%s: fsys.ReadFile: %w", file, err)
			return
		}
		t.checkFileRead(file, "ReadAll vs fsys.ReadFile", data, data2)

		// Modify the data and check it again. Modifying the
		// returned byte slice should not affect the next call.
		for i := range data2 {
			data2[i]++
		}
		data2, err = fsys.ReadFile(file)
		if err != nil {
			t.errorf("%s: second call to fsys.ReadFile: %w", file, err)
			return
		}
		t.checkFileRead(file, "Readall vs second fsys.ReadFile", data, data2)

		t.checkBadPath(file, "ReadFile",
			func(name string) error { _, err := fsys.ReadFile(name); return err })
	}

	// Check that fs.ReadFile works with t.fsys.
	data2, err := fs.ReadFile(t.fsys, file)
	if err != nil {
		t.errorf("%s: fs.ReadFile: %w", file, err)
		return
	}
	t.checkFileRead(file, "ReadAll vs fs.ReadFile", data, data2)

	// Use iotest.TestReader to check small reads, Seek, ReadAt.
	f, err = t.fsys.Open(file)
	if err != nil {
		t.errorf("%s: second Open: %w", file, err)
		return
	}
	defer f.Close()
	if err := iotest.TestReader(f, data); err != nil {
		t.errorf("%s: failed TestReader:\n\t%s", file, strings.ReplaceAll(err.Error(), "\n", "\n\t"))
	}
}

func (t *fsTester) checkFileRead(file, desc string, data1, data2 []byte) {
	if string(data1) != string(data2) {
		t.errorf("%s: %s: different data returned\n\t%q\n\t%q", file, desc, data1, data2)
		return
	}
}

// checkOpen validates file opening behavior by attempting to open and then close the given file path.
func (t *fsTester) checkOpen(file string) {
	t.checkBadPath(file, "Open", func(file string) error {
		f, err := t.fsys.Open(file)
		if err == nil {
			f.Close()
		}
		return err
	})
}

// checkBadPath checks that various invalid forms of file's name cannot be opened using open.
func (t *fsTester) checkBadPath(file string, desc string, open func(string) error) {
	bad := []string{
		"/" + file,
		file + "/.",
	}
	if file == "." {
		bad = append(bad, "/")
	}
	if i := strings.Index(file, "/"); i >= 0 {
		bad = append(bad,
			file[:i]+"//"+file[i+1:],
			file[:i]+"/./"+file[i+1:],
			file[:i]+`\`+file[i+1:],
			file[:i]+"/../"+file,
		)
	}
	if i := strings.LastIndex(file, "/"); i >= 0 {
		bad = append(bad,
			file[:i]+"//"+file[i+1:],
			file[:i]+"/./"+file[i+1:],
			file[:i]+`\`+file[i+1:],
			file+"/../"+file[i+1:],
		)
	}

	for _, b := range bad {
		if err := open(b); err == nil {
			t.errorf("%s: %s(%s) succeeded, want error", file, desc, b)
		}
	}
}

```

// === FILE: references/go/src/testing/fuzz.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"
)

func initFuzzFlags() {
	matchFuzz = flag.String("test.fuzz", "", "run the fuzz test matching `regexp`")
	flag.Var(&fuzzDuration, "test.fuzztime", "time to spend fuzzing; default is to run indefinitely")
	flag.Var(&minimizeDuration, "test.fuzzminimizetime", "time to spend minimizing a value after finding a failing input")

	fuzzCacheDir = flag.String("test.fuzzcachedir", "", "directory where interesting fuzzing inputs are stored (for use only by cmd/go)")
	isFuzzWorker = flag.Bool("test.fuzzworker", false, "coordinate with the parent process to fuzz random values (for use only by cmd/go)")
}

var (
	matchFuzz        *string
	fuzzDuration     durationOrCountFlag
	minimizeDuration = durationOrCountFlag{d: 60 * time.Second, allowZero: true}
	fuzzCacheDir     *string
	isFuzzWorker     *bool

	// corpusDir is the parent directory of the fuzz test's seed corpus within
	// the package.
	corpusDir = "testdata/fuzz"
)

// fuzzWorkerExitCode is used as an exit code by fuzz worker processes after an
// internal error. This distinguishes internal errors from uncontrolled panics
// and other failures. Keep in sync with internal/fuzz.workerExitCode.
const fuzzWorkerExitCode = 70

// InternalFuzzTarget is an internal type but exported because it is
// cross-package; it is part of the implementation of the "go test" command.
type InternalFuzzTarget struct {
	Name string
	Fn   func(f *F)
}

// F is a type passed to fuzz tests.
//
// Fuzz tests run generated inputs against a provided fuzz target, which can
// find and report potential bugs in the code being tested.
//
// A fuzz test runs the seed corpus by default, which includes entries provided
// by [F.Add] and entries in the testdata/fuzz/<FuzzTestName> directory. After
// any necessary setup and calls to [F.Add], the fuzz test must then call
// [F.Fuzz] to provide the fuzz target. See the testing package documentation
// for an example, and see the [F.Fuzz] and [F.Add] method documentation for
// details.
//
// *F methods can only be called before [F.Fuzz]. Once the test is
// executing the fuzz target, only [*T] methods can be used. The only *F methods
// that are allowed in the [F.Fuzz] function are [F.Failed] and [F.Name].
type F struct {
	common
	fstate *fuzzState
	tstate *testState

	// inFuzzFn is true when the fuzz function is running. Most F methods cannot
	// be called when inFuzzFn is true.
	inFuzzFn bool

	// corpus is a set of seed corpus entries, added with F.Add and loaded
	// from testdata.
	corpus []corpusEntry

	result     fuzzResult
	fuzzCalled bool
}

var _ TB = (*F)(nil)

// corpusEntry is an alias to the same type as internal/fuzz.CorpusEntry.
// We use a type alias because we don't want to export this type, and we can't
// import internal/fuzz from testing.
type corpusEntry = struct {
	Parent     string
	Path       string
	Data       []byte
	Values     []any
	Generation int
	IsSeed     bool
}

// Helper marks the calling function as a test helper function.
// When printing file and line information, that function will be skipped.
// Helper may be called simultaneously from multiple goroutines.
func (f *F) Helper() {
	if f.inFuzzFn {
		panic("testing: f.Helper was called inside the fuzz target, use t.Helper instead")
	}

	// common.Helper is inlined here.
	// If we called it, it would mark F.Helper as the helper
	// instead of the caller.
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.helperPCs == nil {
		f.helperPCs = make(map[uintptr]struct{})
	}
	// repeating code from callerName here to save walking a stack frame
	var pc [1]uintptr
	n := runtime.Callers(2, pc[:]) // skip runtime.Callers + Helper
	if n == 0 {
		panic("testing: zero callers found")
	}
	if _, found := f.helperPCs[pc[0]]; !found {
		f.helperPCs[pc[0]] = struct{}{}
		f.helperNames = nil // map will be recreated next time it is needed
	}
}

// Fail marks the function as having failed but continues execution.
func (f *F) Fail() {
	// (*F).Fail may be called by (*T).Fail, which we should allow. However, we
	// shouldn't allow direct (*F).Fail calls from inside the (*F).Fuzz function.
	if f.inFuzzFn {
		panic("testing: f.Fail was called inside the fuzz target, use t.Fail instead")
	}
	f.common.Helper()
	f.common.Fail()
}

// Skipped reports whether the test was skipped.
func (f *F) Skipped() bool {
	// (*F).Skipped may be called by tRunner, which we should allow. However, we
	// shouldn't allow direct (*F).Skipped calls from inside the (*F).Fuzz function.
	if f.inFuzzFn {
		panic("testing: f.Skipped was called inside the fuzz target, use t.Skipped instead")
	}
	f.common.Helper()
	return f.common.Skipped()
}

// Add will add the arguments to the seed corpus for the fuzz test. This will be
// a no-op if called after or within the fuzz target, and args must match the
// arguments for the fuzz target.
func (f *F) Add(args ...any) {
	var values []any
	for i := range args {
		if t := reflect.TypeOf(args[i]); !supportedTypes[t] {
			panic(fmt.Sprintf("testing: unsupported type to Add %v", t))
		}
		values = append(values, args[i])
	}
	f.corpus = append(f.corpus, corpusEntry{Values: values, IsSeed: true, Path: fmt.Sprintf("seed#%d", len(f.corpus))})
}

// supportedTypes represents all of the supported types which can be fuzzed.
var supportedTypes = map[reflect.Type]bool{
	reflect.TypeFor[[]byte]():  true,
	reflect.TypeFor[string]():  true,
	reflect.TypeFor[bool]():    true,
	reflect.TypeFor[byte]():    true,
	reflect.TypeFor[rune]():    true,
	reflect.TypeFor[float32](): true,
	reflect.TypeFor[float64](): true,
	reflect.TypeFor[int]():     true,
	reflect.TypeFor[int8]():    true,
	reflect.TypeFor[int16]():   true,
	reflect.TypeFor[int32]():   true,
	reflect.TypeFor[int64]():   true,
	reflect.TypeFor[uint]():    true,
	reflect.TypeFor[uint8]():   true,
	reflect.TypeFor[uint16]():  true,
	reflect.TypeFor[uint32]():  true,
	reflect.TypeFor[uint64]():  true,
}

// Fuzz runs the fuzz function, ff, for fuzz testing. If ff fails for a set of
// arguments, those arguments will be added to the seed corpus.
//
// ff must be a function with no return value whose first argument is [*T] and
// whose remaining arguments are the types to be fuzzed.
// For example:
//
//	f.Fuzz(func(t *testing.T, b []byte, i int) { ... })
//
// The following types are allowed: []byte, string, bool, byte, rune, float32,
// float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64.
// More types may be supported in the future.
//
// ff must not call any [*F] methods, e.g. [F.Log], [F.Error], [F.Skip]. Use
// the corresponding [*T] method instead. The only [*F] methods that are allowed in
// the F.Fuzz function are [F.Failed] and [F.Name].
//
// This function should be fast and deterministic, and its behavior should not
// depend on shared state. No mutable input arguments, or pointers to them,
// should be retained between executions of the fuzz function, as the memory
// backing them may be mutated during a subsequent invocation. ff must not
// modify the underlying data of the arguments provided by the fuzzing engine.
//
// When fuzzing, F.Fuzz does not return until a problem is found, time runs out
// (set with -fuzztime), or the test process is interrupted by a signal. F.Fuzz
// should be called exactly once, unless [F.Skip] or [F.Fail] is called beforehand.
func (f *F) Fuzz(ff any) {
	if f.fuzzCalled {
		panic("testing: F.Fuzz called more than once")
	}
	f.fuzzCalled = true
	if f.failed {
		return
	}
	f.Helper()

	// ff should be in the form func(*testing.T, ...interface{})
	fn := reflect.ValueOf(ff)
	fnType := fn.Type()
	if fnType.Kind() != reflect.Func {
		panic("testing: F.Fuzz must receive a function")
	}
	if fnType.NumIn() < 2 || fnType.In(0) != reflect.TypeFor[*T]() {
		panic("testing: fuzz target must receive at least two arguments, where the first argument is a *T")
	}
	if fnType.NumOut() != 0 {
		panic("testing: fuzz target must not return a value")
	}

	// Save the types of the function to compare against the corpus.
	var types []reflect.Type
	for i := 1; i < fnType.NumIn(); i++ {
		t := fnType.In(i)
		if !supportedTypes[t] {
			panic(fmt.Sprintf("testing: unsupported type for fuzzing %v", t))
		}
		types = append(types, t)
	}

	// Load the testdata seed corpus. Check types of entries in the testdata
	// corpus and entries declared with F.Add.
	//
	// Don't load the seed corpus if this is a worker process; we won't use it.
	if f.fstate.mode != fuzzWorker {
		for _, c := range f.corpus {
			if err := f.fstate.deps.CheckCorpus(c.Values, types); err != nil {
				// TODO(#48302): Report the source location of the F.Add call.
				f.Fatal(err)
			}
		}

		// Load seed corpus
		c, err := f.fstate.deps.ReadCorpus(filepath.Join(corpusDir, f.name), types)
		if err != nil {
			f.Fatal(err)
		}
		for i := range c {
			c[i].IsSeed = true // these are all seed corpus values
			if f.fstate.mode == fuzzCoordinator {
				// If this is the coordinator process, zero the values, since we don't need
				// to hold onto them.
				c[i].Values = nil
			}
		}

		f.corpus = append(f.corpus, c...)
	}

	// run calls fn on a given input, as a subtest with its own T.
	// run is analogous to T.Run. The test filtering and cleanup works similarly.
	// fn is called in its own goroutine.
	run := func(captureOut io.Writer, e corpusEntry) (ok bool) {
		if e.Values == nil {
			// The corpusEntry must have non-nil Values in order to run the
			// test. If Values is nil, it is a bug in our code.
			panic(fmt.Sprintf("corpus file %q was not unmarshaled", e.Path))
		}
		if shouldFailFast() {
			return true
		}
		testName := f.name
		if e.Path != "" {
			testName = fmt.Sprintf("%s/%s", testName, filepath.Base(e.Path))
		}
		if f.tstate.isFuzzing {
			// Don't preserve subtest names while fuzzing. If fn calls T.Run,
			// there will be a very large number of subtests with duplicate names,
			// which will use a large amount of memory. The subtest names aren't
			// useful since there's no way to re-run them deterministically.
			f.tstate.match.clearSubNames()
		}

		ctx, cancelCtx := context.WithCancel(f.ctx)

		// Record the stack trace at the point of this call so that if the subtest
		// function - which runs in a separate stack - is marked as a helper, we can
		// continue walking the stack into the parent test.
		var pc [maxStackLen]uintptr
		n := runtime.Callers(2, pc[:])
		t := &T{
			common: common{
				barrier:   make(chan bool),
				signal:    make(chan bool),
				name:      testName,
				parent:    &f.common,
				level:     f.level + 1,
				creator:   pc[:n],
				chatty:    f.chatty,
				ctx:       ctx,
				cancelCtx: cancelCtx,
			},
			tstate: f.tstate,
		}
		if captureOut != nil {
			// t.parent aliases f.common.
			t.parent.w = captureOut
		}
		t.w = indenter{&t.common}
		t.setOutputWriter()
		if t.chatty != nil {
			t.chatty.Updatef(t.name, "=== RUN   %s\n", t.name)
		}
		f.common.inFuzzFn, f.inFuzzFn = true, true
		go tRunner(t, func(t *T) {
			args := []reflect.Value{reflect.ValueOf(t)}
			for _, v := range e.Values {
				args = append(args, reflect.ValueOf(v))
			}
			// Before resetting the current coverage, defer the snapshot so that
			// we make sure it is called right before the tRunner function
			// exits, regardless of whether it was executed cleanly, panicked,
			// or if the fuzzFn called t.Fatal.
			if f.tstate.isFuzzing {
				defer f.fstate.deps.SnapshotCoverage()
				f.fstate.deps.ResetCoverage()
			}
			fn.Call(args)
		})
		<-t.signal
		if t.chatty != nil && t.chatty.json {
			t.chatty.Updatef(t.parent.name, "=== NAME  %s\n", t.parent.name)
		}
		f.common.inFuzzFn, f.inFuzzFn = false, false
		return !t.Failed()
	}

	switch f.fstate.mode {
	case fuzzCoordinator:
		// Fuzzing is enabled, and this is the test process started by 'go test'.
		// Act as the coordinator process, and coordinate workers to perform the
		// actual fuzzing.
		corpusTargetDir := filepath.Join(corpusDir, f.name)
		cacheTargetDir := filepath.Join(*fuzzCacheDir, f.name)
		err := f.fstate.deps.CoordinateFuzzing(
			fuzzDuration.d,
			int64(fuzzDuration.n),
			minimizeDuration.d,
			int64(minimizeDuration.n),
			*parallel,
			f.corpus,
			types,
			corpusTargetDir,
			cacheTargetDir)
		if err != nil {
			f.result = fuzzResult{Error: err}
			f.Fail()
			fmt.Fprintf(f.w, "%v\n", err)
			if crashErr, ok := err.(fuzzCrashError); ok {
				crashPath := crashErr.CrashPath()
				fmt.Fprintf(f.w, "Failing input written to %s\n", crashPath)
				testName := filepath.Base(crashPath)
				fmt.Fprintf(f.w, "To re-run:\ngo test -run=%s/%s\n", f.name, testName)
			}
		}
		// TODO(jayconrod,katiehockman): Aggregate statistics across workers
		// and add to FuzzResult (ie. time taken, num iterations)

	case fuzzWorker:
		// Fuzzing is enabled, and this is a worker process. Follow instructions
		// from the coordinator.
		if err := f.fstate.deps.RunFuzzWorker(func(e corpusEntry) error {
			// Don't write to f.w (which points to Stdout) if running from a
			// fuzz worker. This would become very verbose, particularly during
			// minimization. Return the error instead, and let the caller deal
			// with the output.
			var buf strings.Builder
			if ok := run(&buf, e); !ok {
				return errors.New(buf.String())
			}
			return nil
		}); err != nil {
			// Internal errors are marked with f.Fail; user code may call this too, before F.Fuzz.
			// The worker will exit with fuzzWorkerExitCode, indicating this is a failure
			// (and 'go test' should exit non-zero) but a failing input should not be recorded.
			f.Errorf("communicating with fuzzing coordinator: %v", err)
		}

	default:
		// Fuzzing is not enabled, or will be done later. Only run the seed
		// corpus now.
		for _, e := range f.corpus {
			name := fmt.Sprintf("%s/%s", f.name, filepath.Base(e.Path))
			if _, ok, _ := f.tstate.match.fullName(nil, name); ok {
				run(f.w, e)
			}
		}
	}
}

func (f *F) report() {
	if *isFuzzWorker || f.parent == nil {
		return
	}
	dstr := fmtDuration(f.duration)
	format := "--- %s: %s (%s)\n"
	if f.Failed() {
		f.flushToParent(f.name, format, "FAIL", f.name, dstr)
	} else if f.chatty != nil {
		if f.Skipped() {
			f.flushToParent(f.name, format, "SKIP", f.name, dstr)
		} else {
			f.flushToParent(f.name, format, "PASS", f.name, dstr)
		}
	}
}

// fuzzResult contains the results of a fuzz run.
type fuzzResult struct {
	N     int           // The number of iterations.
	T     time.Duration // The total time taken.
	Error error         // Error is the error from the failing input
}

func (r fuzzResult) String() string {
	if r.Error == nil {
		return ""
	}
	return r.Error.Error()
}

// fuzzCrashError is satisfied by a failing input detected while fuzzing.
// These errors are written to the seed corpus and can be re-run with 'go test'.
// Errors within the fuzzing framework (like I/O errors between coordinator
// and worker processes) don't satisfy this interface.
type fuzzCrashError interface {
	error
	Unwrap() error

	// CrashPath returns the path of the subtest that corresponds to the saved
	// crash input file in the seed corpus. The test can be re-run with go test
	// -run=$test/$name $test is the fuzz test name, and $name is the
	// filepath.Base of the string returned here.
	CrashPath() string
}

// fuzzState holds fields common to all fuzz tests.
type fuzzState struct {
	deps testDeps
	mode fuzzMode
}

type fuzzMode uint8

const (
	seedCorpusOnly fuzzMode = iota
	fuzzCoordinator
	fuzzWorker
)

// runFuzzTests runs the fuzz tests matching the pattern for -run. This will
// only run the (*F).Fuzz function for each seed corpus without using the
// fuzzing engine to generate or mutate inputs.
func runFuzzTests(deps testDeps, fuzzTests []InternalFuzzTarget, deadline time.Time) (ran, ok bool) {
	ok = true
	if len(fuzzTests) == 0 || *isFuzzWorker {
		return ran, ok
	}
	m := newMatcher(deps.MatchString, *match, "-test.run", *skip)
	var mFuzz *matcher
	if *matchFuzz != "" {
		mFuzz = newMatcher(deps.MatchString, *matchFuzz, "-test.fuzz", *skip)
	}

	for _, procs := range cpuList {
		runtime.GOMAXPROCS(procs)
		for i := uint(0); i < *count; i++ {
			if shouldFailFast() {
				break
			}

			tstate := newTestState(*parallel, m)
			tstate.deadline = deadline
			fstate := &fuzzState{deps: deps, mode: seedCorpusOnly}
			root := common{w: os.Stdout} // gather output in one place
			if Verbose() {
				root.chatty = newChattyPrinter(root.w)
			}
			for _, ft := range fuzzTests {
				if shouldFailFast() {
					break
				}
				testName, matched, _ := tstate.match.fullName(nil, ft.Name)
				if !matched {
					continue
				}
				if mFuzz != nil {
					if _, fuzzMatched, _ := mFuzz.fullName(nil, ft.Name); fuzzMatched {
						// If this will be fuzzed, then don't run the seed corpus
						// right now. That will happen later.
						continue
					}
				}
				ctx, cancelCtx := context.WithCancel(context.Background())
				f := &F{
					common: common{
						signal:    make(chan bool),
						barrier:   make(chan bool),
						name:      testName,
						parent:    &root,
						level:     root.level + 1,
						chatty:    root.chatty,
						ctx:       ctx,
						cancelCtx: cancelCtx,
					},
					tstate: tstate,
					fstate: fstate,
				}
				f.w = indenter{&f.common}
				f.setOutputWriter()
				if f.chatty != nil {
					f.chatty.Updatef(f.name, "=== RUN   %s\n", f.name)
				}
				go fRunner(f, ft.Fn)
				<-f.signal
				if f.chatty != nil && f.chatty.json {
					f.chatty.Updatef(f.parent.name, "=== NAME  %s\n", f.parent.name)
				}
				ok = ok && !f.Failed()
				ran = ran || f.ran
			}
			if !ran {
				// There were no tests to run on this iteration.
				// This won't change, so no reason to keep trying.
				break
			}
		}
	}

	return ran, ok
}

// runFuzzing runs the fuzz test matching the pattern for -fuzz. Only one such
// fuzz test must match. This will run the fuzzing engine to generate and
// mutate new inputs against the fuzz target.
//
// If fuzzing is disabled (-test.fuzz is not set), runFuzzing
// returns immediately.
func runFuzzing(deps testDeps, fuzzTests []InternalFuzzTarget) (ok bool) {
	if len(fuzzTests) == 0 || *matchFuzz == "" {
		return true
	}
	m := newMatcher(deps.MatchString, *matchFuzz, "-test.fuzz", *skip)
	tstate := newTestState(1, m)
	tstate.isFuzzing = true
	fstate := &fuzzState{
		deps: deps,
	}
	root := common{w: os.Stdout}
	if *isFuzzWorker {
		root.w = io.Discard
		fstate.mode = fuzzWorker
	} else {
		fstate.mode = fuzzCoordinator
	}
	if Verbose() && !*isFuzzWorker {
		root.chatty = newChattyPrinter(root.w)
	}
	var fuzzTest *InternalFuzzTarget
	var testName string
	var matched []string
	for i := range fuzzTests {
		name, ok, _ := tstate.match.fullName(nil, fuzzTests[i].Name)
		if !ok {
			continue
		}
		matched = append(matched, name)
		fuzzTest = &fuzzTests[i]
		testName = name
	}
	if len(matched) == 0 {
		fmt.Fprintln(os.Stderr, "testing: warning: no fuzz tests to fuzz")
		return true
	}
	if len(matched) > 1 {
		fmt.Fprintf(os.Stderr, "testing: will not fuzz, -fuzz matches more than one fuzz test: %v\n", matched)
		return false
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	f := &F{
		common: common{
			signal:    make(chan bool),
			barrier:   nil, // T.Parallel has no effect when fuzzing.
			name:      testName,
			parent:    &root,
			level:     root.level + 1,
			chatty:    root.chatty,
			ctx:       ctx,
			cancelCtx: cancelCtx,
		},
		fstate: fstate,
		tstate: tstate,
	}
	f.w = indenter{&f.common}
	f.setOutputWriter()
	if f.chatty != nil {
		f.chatty.Updatef(f.name, "=== RUN   %s\n", f.name)
	}
	go fRunner(f, fuzzTest.Fn)
	<-f.signal
	if f.chatty != nil {
		f.chatty.Updatef(f.parent.name, "=== NAME  %s\n", f.parent.name)
	}
	return !f.failed
}

// fRunner wraps a call to a fuzz test and ensures that cleanup functions are
// called and status flags are set. fRunner should be called in its own
// goroutine. To wait for its completion, receive from f.signal.
//
// fRunner is analogous to tRunner, which wraps subtests started with T.Run.
// Unit tests and fuzz tests work a little differently, so for now, these
// functions aren't consolidated. In particular, because there are no F.Run and
// F.Parallel methods, i.e., no fuzz sub-tests or parallel fuzz tests, a few
// simplifications are made. We also require that F.Fuzz, F.Skip, or F.Fail is
// called.
func fRunner(f *F, fn func(*F)) {
	// When this goroutine is done, either because runtime.Goexit was called, a
	// panic started, or fn returned normally, record the duration and send
	// t.signal, indicating the fuzz test is done.
	defer func() {
		// Detect whether the fuzz test panicked or called runtime.Goexit
		// without calling F.Fuzz, F.Fail, or F.Skip. If it did, panic (possibly
		// replacing a nil panic value). Nothing should recover after fRunner
		// unwinds, so this should crash the process and print stack.
		// Unfortunately, recovering here adds stack frames, but the location of
		// the original panic should still be
		// clear.
		f.checkRaces()
		if f.Failed() {
			numFailed.Add(1)
		}
		err := recover()
		if err == nil {
			f.mu.RLock()
			fuzzNotCalled := !f.fuzzCalled && !f.skipped && !f.failed
			if !f.finished && !f.skipped && !f.failed {
				err = errNilPanicOrGoexit
			}
			f.mu.RUnlock()
			if fuzzNotCalled && err == nil {
				f.Error("returned without calling F.Fuzz, F.Fail, or F.Skip")
			}
		}

		// Use a deferred call to ensure that we report that the test is
		// complete even if a cleanup function calls F.FailNow. See issue 41355.
		didPanic := false
		defer func() {
			if !didPanic {
				// Only report that the test is complete if it doesn't panic,
				// as otherwise the test binary can exit before the panic is
				// reported to the user. See issue 41479.
				f.signal <- true
			}
		}()

		// If we recovered a panic or inappropriate runtime.Goexit, fail the test,
		// flush the output log up to the root, then panic.
		doPanic := func(err any) {
			f.Fail()
			if r := f.runCleanup(recoverAndReturnPanic); r != nil {
				f.Logf("cleanup panicked with %v", r)
			}
			for root := &f.common; root.parent != nil; root = root.parent {
				root.mu.Lock()
				root.duration += highPrecisionTimeSince(root.start)
				d := root.duration
				root.mu.Unlock()
				root.flushToParent(root.name, "--- FAIL: %s (%s)\n", root.name, fmtDuration(d))
			}
			didPanic = true
			panic(err)
		}
		if err != nil {
			doPanic(err)
		}

		// No panic or inappropriate Goexit.
		f.duration += highPrecisionTimeSince(f.start)

		if len(f.sub) > 0 {
			// Unblock inputs that called T.Parallel while running the seed corpus.
			// This only affects fuzz tests run as normal tests.
			// While fuzzing, T.Parallel has no effect, so f.sub is empty, and this
			// branch is not taken. f.barrier is nil in that case.
			f.tstate.release()
			close(f.barrier)
			// Wait for the subtests to complete.
			for _, sub := range f.sub {
				<-sub.signal
			}
			cleanupStart := highPrecisionTimeNow()
			err := f.runCleanup(recoverAndReturnPanic)
			f.duration += highPrecisionTimeSince(cleanupStart)
			if err != nil {
				doPanic(err)
			}
		}

		// Report after all subtests have finished.
		f.report()
		f.done = true
		f.setRan()
	}()
	defer func() {
		if len(f.sub) == 0 {
			f.runCleanup(normalPanic)
		}
	}()

	f.start = highPrecisionTimeNow()
	f.resetRaces()
	fn(f)

	// Code beyond this point will not be executed when FailNow or SkipNow
	// is invoked.
	f.mu.Lock()
	f.finished = true
	f.mu.Unlock()
}

```

// === FILE: references/go/src/testing/internal/testdeps/deps.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testdeps provides access to dependencies needed by test execution.
//
// This package is imported by the generated main package, which passes
// TestDeps into testing.Main. This allows tests to use packages at run time
// without making those packages direct dependencies of package testing.
// Direct dependencies of package testing are harder to write tests for.
package testdeps

import (
	"bufio"
	"context"
	"internal/fuzz"
	"internal/testlog"
	"io"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"runtime/pprof"
	"strings"
	"sync"
	"time"
)

// Cover indicates whether coverage is enabled.
var Cover bool

// TestDeps is an implementation of the testing.testDeps interface,
// suitable for passing to [testing.MainStart].
type TestDeps struct{}

var matchPat string
var matchRe *regexp.Regexp

func (TestDeps) MatchString(pat, str string) (result bool, err error) {
	if matchRe == nil || matchPat != pat {
		matchPat = pat
		matchRe, err = regexp.Compile(matchPat)
		if err != nil {
			return
		}
	}
	return matchRe.MatchString(str), nil
}

func (TestDeps) StartCPUProfile(w io.Writer) error {
	return pprof.StartCPUProfile(w)
}

func (TestDeps) StopCPUProfile() {
	pprof.StopCPUProfile()
}

func (TestDeps) WriteProfileTo(name string, w io.Writer, debug int) error {
	return pprof.Lookup(name).WriteTo(w, debug)
}

// ImportPath is the import path of the testing binary, set by the generated main function.
var ImportPath string

func (TestDeps) ImportPath() string {
	return ImportPath
}

var ModulePath string

func (TestDeps) ModulePath() string {
	return ModulePath
}

// testLog implements testlog.Interface, logging actions by package os.
type testLog struct {
	mu  sync.Mutex
	w   *bufio.Writer
	set bool
}

func (l *testLog) Getenv(key string) {
	l.add("getenv", key)
}

func (l *testLog) Open(name string) {
	l.add("open", name)
}

func (l *testLog) Stat(name string) {
	l.add("stat", name)
}

func (l *testLog) Chdir(name string) {
	l.add("chdir", name)
}

// add adds the (op, name) pair to the test log.
func (l *testLog) add(op, name string) {
	if strings.Contains(name, "\n") || name == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.w == nil {
		return
	}
	l.w.WriteString(op)
	l.w.WriteByte(' ')
	l.w.WriteString(name)
	l.w.WriteByte('\n')
}

var log testLog

func (TestDeps) StartTestLog(w io.Writer) {
	log.mu.Lock()
	log.w = bufio.NewWriter(w)
	if !log.set {
		// Tests that define TestMain and then run m.Run multiple times
		// will call StartTestLog/StopTestLog multiple times.
		// Checking log.set avoids calling testlog.SetLogger multiple times
		// (which will panic) and also avoids writing the header multiple times.
		log.set = true
		testlog.SetLogger(&log)
		log.w.WriteString("# test log\n") // known to cmd/go/internal/test/test.go
	}
	log.mu.Unlock()
}

func (TestDeps) StopTestLog() error {
	log.mu.Lock()
	defer log.mu.Unlock()
	err := log.w.Flush()
	log.w = nil
	return err
}

// SetPanicOnExit0 tells the os package whether to panic on os.Exit(0).
func (TestDeps) SetPanicOnExit0(v bool) {
	testlog.SetPanicOnExit0(v)
}

func (TestDeps) CoordinateFuzzing(
	timeout time.Duration,
	limit int64,
	minimizeTimeout time.Duration,
	minimizeLimit int64,
	parallel int,
	seed []fuzz.CorpusEntry,
	types []reflect.Type,
	corpusDir,
	cacheDir string) (err error) {
	// Fuzzing may be interrupted with a timeout or if the user presses ^C.
	// In either case, we'll stop worker processes gracefully and save
	// crashers and interesting values.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	err = fuzz.CoordinateFuzzing(ctx, fuzz.CoordinateFuzzingOpts{
		Log:             os.Stderr,
		Timeout:         timeout,
		Limit:           limit,
		MinimizeTimeout: minimizeTimeout,
		MinimizeLimit:   minimizeLimit,
		Parallel:        parallel,
		Seed:            seed,
		Types:           types,
		CorpusDir:       corpusDir,
		CacheDir:        cacheDir,
	})
	if err == ctx.Err() {
		return nil
	}
	return err
}

func (TestDeps) RunFuzzWorker(fn func(fuzz.CorpusEntry) error) error {
	// Worker processes may or may not receive a signal when the user presses ^C
	// On POSIX operating systems, a signal sent to a process group is delivered
	// to all processes in that group. This is not the case on Windows.
	// If the worker is interrupted, return quickly and without error.
	// If only the coordinator process is interrupted, it tells each worker
	// process to stop by closing its "fuzz_in" pipe.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	err := fuzz.RunFuzzWorker(ctx, fn)
	if err == ctx.Err() {
		return nil
	}
	return err
}

func (TestDeps) ReadCorpus(dir string, types []reflect.Type) ([]fuzz.CorpusEntry, error) {
	return fuzz.ReadCorpus(dir, types)
}

func (TestDeps) CheckCorpus(vals []any, types []reflect.Type) error {
	return fuzz.CheckCorpus(vals, types)
}

func (TestDeps) ResetCoverage() {
	fuzz.ResetCoverage()
}

func (TestDeps) SnapshotCoverage() {
	fuzz.SnapshotCoverage()
}

var CoverMode string
var Covered string
var CoverSelectedPackages []string

// These variables below are set at runtime (via code in testmain) to point
// to the equivalent functions in package internal/coverage/cfile; doing
// things this way allows us to have tests import internal/coverage/cfile
// only when -cover is in effect (as opposed to importing for all tests).
var (
	CoverSnapshotFunc           func() float64
	CoverProcessTestDirFunc     func(dir string, cfile string, cm string, cpkg string, w io.Writer, selpkgs []string) error
	CoverMarkProfileEmittedFunc func(val bool)
)

func (TestDeps) InitRuntimeCoverage() (mode string, tearDown func(string, string) (string, error), snapcov func() float64) {
	if CoverMode == "" {
		return
	}
	return CoverMode, coverTearDown, CoverSnapshotFunc
}

func coverTearDown(coverprofile string, gocoverdir string) (string, error) {
	var err error
	if gocoverdir == "" {
		gocoverdir, err = os.MkdirTemp("", "gocoverdir")
		if err != nil {
			return "error setting GOCOVERDIR: bad os.MkdirTemp return", err
		}
		defer os.RemoveAll(gocoverdir)
	}
	CoverMarkProfileEmittedFunc(true)
	cmode := CoverMode
	if err := CoverProcessTestDirFunc(gocoverdir, coverprofile, cmode, Covered, os.Stdout, CoverSelectedPackages); err != nil {
		return "error generating coverage report", err
	}
	return "", nil
}

```

// === FILE: references/go/src/testing/iotest/logger.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iotest

import (
	"io"
	"log"
)

type writeLogger struct {
	prefix string
	w      io.Writer
}

func (l *writeLogger) Write(p []byte) (n int, err error) {
	n, err = l.w.Write(p)
	if err != nil {
		log.Printf("%s %x: %v", l.prefix, p[0:n], err)
	} else {
		log.Printf("%s %x", l.prefix, p[0:n])
	}
	return
}

// NewWriteLogger returns a writer that behaves like w except
// that it logs (using [log.Printf]) each write to standard error,
// printing the prefix and the hexadecimal data written.
func NewWriteLogger(prefix string, w io.Writer) io.Writer {
	return &writeLogger{prefix, w}
}

type readLogger struct {
	prefix string
	r      io.Reader
}

func (l *readLogger) Read(p []byte) (n int, err error) {
	n, err = l.r.Read(p)
	if err != nil {
		log.Printf("%s %x: %v", l.prefix, p[0:n], err)
	} else {
		log.Printf("%s %x", l.prefix, p[0:n])
	}
	return
}

// NewReadLogger returns a reader that behaves like r except
// that it logs (using [log.Printf]) each read to standard error,
// printing the prefix and the hexadecimal data read.
func NewReadLogger(prefix string, r io.Reader) io.Reader {
	return &readLogger{prefix, r}
}

```

// === FILE: references/go/src/testing/iotest/reader.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package iotest implements Readers and Writers useful mainly for testing.
package iotest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// OneByteReader returns a Reader that implements
// each non-empty Read by reading one byte from r.
func OneByteReader(r io.Reader) io.Reader { return &oneByteReader{r} }

type oneByteReader struct {
	r io.Reader
}

func (r *oneByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return r.r.Read(p[0:1])
}

// HalfReader returns a Reader that implements Read
// by reading half as many requested bytes from r.
func HalfReader(r io.Reader) io.Reader { return &halfReader{r} }

type halfReader struct {
	r io.Reader
}

func (r *halfReader) Read(p []byte) (int, error) {
	return r.r.Read(p[0 : (len(p)+1)/2])
}

// DataErrReader changes the way errors are handled by a Reader. Normally, a
// Reader returns an error (typically EOF) from the first Read call after the
// last piece of data is read. DataErrReader wraps a Reader and changes its
// behavior so the final error is returned along with the final data, instead
// of in the first call after the final data.
func DataErrReader(r io.Reader) io.Reader { return &dataErrReader{r, nil, make([]byte, 1024)} }

type dataErrReader struct {
	r      io.Reader
	unread []byte
	data   []byte
}

func (r *dataErrReader) Read(p []byte) (n int, err error) {
	// loop because first call needs two reads:
	// one to get data and a second to look for an error.
	for {
		if len(r.unread) == 0 {
			n1, err1 := r.r.Read(r.data)
			r.unread = r.data[0:n1]
			err = err1
		}
		if n > 0 || err != nil {
			break
		}
		n = copy(p, r.unread)
		r.unread = r.unread[n:]
	}
	return
}

// ErrTimeout is a fake timeout error.
var ErrTimeout = errors.New("timeout")

// TimeoutReader returns [ErrTimeout] on the second read
// with no data. Subsequent calls to read succeed.
func TimeoutReader(r io.Reader) io.Reader { return &timeoutReader{r, 0} }

type timeoutReader struct {
	r     io.Reader
	count int
}

func (r *timeoutReader) Read(p []byte) (int, error) {
	r.count++
	if r.count == 2 {
		return 0, ErrTimeout
	}
	return r.r.Read(p)
}

// ErrReader returns an [io.Reader] that returns 0, err from all Read calls.
func ErrReader(err error) io.Reader {
	return &errReader{err: err}
}

type errReader struct {
	err error
}

func (r *errReader) Read(p []byte) (int, error) {
	return 0, r.err
}

type smallByteReader struct {
	r   io.Reader
	off int
	n   int
}

func (r *smallByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	r.n = r.n%3 + 1
	n := r.n
	if n > len(p) {
		n = len(p)
	}
	n, err := r.r.Read(p[0:n])
	if err != nil && err != io.EOF {
		err = fmt.Errorf("Read(%d bytes at offset %d): %v", n, r.off, err)
	}
	r.off += n
	return n, err
}

// TestReader tests that reading from r returns the expected file content.
// It does reads of different sizes, until EOF.
// If r implements [io.ReaderAt] or [io.Seeker], TestReader also checks
// that those operations behave as they should.
//
// If TestReader finds any misbehaviors, it returns an error reporting them.
// The error text may span multiple lines.
func TestReader(r io.Reader, content []byte) error {
	if len(content) > 0 {
		n, err := r.Read(nil)
		if n != 0 || err != nil {
			return fmt.Errorf("Read(0) = %d, %v, want 0, nil", n, err)
		}
	}

	data, err := io.ReadAll(&smallByteReader{r: r})
	if err != nil {
		return err
	}
	if !bytes.Equal(data, content) {
		return fmt.Errorf("ReadAll(small amounts) = %q\n\twant %q", data, content)
	}
	n, err := r.Read(make([]byte, 10))
	if n != 0 || err != io.EOF {
		return fmt.Errorf("Read(10) at EOF = %v, %v, want 0, EOF", n, err)
	}

	if r, ok := r.(io.ReadSeeker); ok {
		// Seek(0, 1) should report the current file position (EOF).
		if off, err := r.Seek(0, 1); off != int64(len(content)) || err != nil {
			return fmt.Errorf("Seek(0, 1) from EOF = %d, %v, want %d, nil", off, err, len(content))
		}

		// Seek backward partway through file, in two steps.
		// If middle == 0, len(content) == 0, can't use the -1 and +1 seeks.
		middle := len(content) - len(content)/3
		if middle > 0 {
			if off, err := r.Seek(-1, 1); off != int64(len(content)-1) || err != nil {
				return fmt.Errorf("Seek(-1, 1) from EOF = %d, %v, want %d, nil", -off, err, len(content)-1)
			}
			if off, err := r.Seek(int64(-len(content)/3), 1); off != int64(middle-1) || err != nil {
				return fmt.Errorf("Seek(%d, 1) from %d = %d, %v, want %d, nil", -len(content)/3, len(content)-1, off, err, middle-1)
			}
			if off, err := r.Seek(+1, 1); off != int64(middle) || err != nil {
				return fmt.Errorf("Seek(+1, 1) from %d = %d, %v, want %d, nil", middle-1, off, err, middle)
			}
		}

		// Seek(0, 1) should report the current file position (middle).
		if off, err := r.Seek(0, 1); off != int64(middle) || err != nil {
			return fmt.Errorf("Seek(0, 1) from %d = %d, %v, want %d, nil", middle, off, err, middle)
		}

		// Reading forward should return the last part of the file.
		data, err := io.ReadAll(&smallByteReader{r: r})
		if err != nil {
			return fmt.Errorf("ReadAll from offset %d: %v", middle, err)
		}
		if !bytes.Equal(data, content[middle:]) {
			return fmt.Errorf("ReadAll from offset %d = %q\n\twant %q", middle, data, content[middle:])
		}

		// Seek relative to end of file, but start elsewhere.
		if off, err := r.Seek(int64(middle/2), 0); off != int64(middle/2) || err != nil {
			return fmt.Errorf("Seek(%d, 0) from EOF = %d, %v, want %d, nil", middle/2, off, err, middle/2)
		}
		if off, err := r.Seek(int64(-len(content)/3), 2); off != int64(middle) || err != nil {
			return fmt.Errorf("Seek(%d, 2) from %d = %d, %v, want %d, nil", -len(content)/3, middle/2, off, err, middle)
		}

		// Reading forward should return the last part of the file (again).
		data, err = io.ReadAll(&smallByteReader{r: r})
		if err != nil {
			return fmt.Errorf("ReadAll from offset %d: %v", middle, err)
		}
		if !bytes.Equal(data, content[middle:]) {
			return fmt.Errorf("ReadAll from offset %d = %q\n\twant %q", middle, data, content[middle:])
		}

		// Absolute seek & read forward.
		if off, err := r.Seek(int64(middle/2), 0); off != int64(middle/2) || err != nil {
			return fmt.Errorf("Seek(%d, 0) from EOF = %d, %v, want %d, nil", middle/2, off, err, middle/2)
		}
		data, err = io.ReadAll(r)
		if err != nil {
			return fmt.Errorf("ReadAll from offset %d: %v", middle/2, err)
		}
		if !bytes.Equal(data, content[middle/2:]) {
			return fmt.Errorf("ReadAll from offset %d = %q\n\twant %q", middle/2, data, content[middle/2:])
		}
	}

	if r, ok := r.(io.ReaderAt); ok {
		data := make([]byte, len(content), len(content)+1)
		for i := range data {
			data[i] = 0xfe
		}
		n, err := r.ReadAt(data, 0)
		if n != len(data) || err != nil && err != io.EOF {
			return fmt.Errorf("ReadAt(%d, 0) = %v, %v, want %d, nil or EOF", len(data), n, err, len(data))
		}
		if !bytes.Equal(data, content) {
			return fmt.Errorf("ReadAt(%d, 0) = %q\n\twant %q", len(data), data, content)
		}

		n, err = r.ReadAt(data[:1], int64(len(data)))
		if n != 0 || err != io.EOF {
			return fmt.Errorf("ReadAt(1, %d) = %v, %v, want 0, EOF", len(data), n, err)
		}

		for i := range data {
			data[i] = 0xfe
		}
		n, err = r.ReadAt(data[:cap(data)], 0)
		if n != len(data) || err != io.EOF {
			return fmt.Errorf("ReadAt(%d, 0) = %v, %v, want %d, EOF", cap(data), n, err, len(data))
		}
		if !bytes.Equal(data, content) {
			return fmt.Errorf("ReadAt(%d, 0) = %q\n\twant %q", len(data), data, content)
		}

		for i := range data {
			data[i] = 0xfe
		}
		for i := range data {
			n, err = r.ReadAt(data[i:i+1], int64(i))
			if n != 1 || err != nil && (i != len(data)-1 || err != io.EOF) {
				want := "nil"
				if i == len(data)-1 {
					want = "nil or EOF"
				}
				return fmt.Errorf("ReadAt(1, %d) = %v, %v, want 1, %s", i, n, err, want)
			}
			if data[i] != content[i] {
				return fmt.Errorf("ReadAt(1, %d) = %q want %q", i, data[i:i+1], content[i:i+1])
			}
		}
	}
	return nil
}

```

// === FILE: references/go/src/testing/iotest/writer.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iotest

import "io"

// TruncateWriter returns a Writer that writes to w
// but stops silently after n bytes.
func TruncateWriter(w io.Writer, n int64) io.Writer {
	return &truncateWriter{w, n}
}

type truncateWriter struct {
	w io.Writer
	n int64
}

func (t *truncateWriter) Write(p []byte) (n int, err error) {
	if t.n <= 0 {
		return len(p), nil
	}
	// real write
	n = len(p)
	if int64(n) > t.n {
		n = int(t.n)
	}
	n, err = t.w.Write(p[0:n])
	t.n -= int64(n)
	if err == nil {
		n = len(p)
	}
	return
}

```

// === FILE: references/go/src/testing/match.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// matcher sanitizes, uniques, and filters names of subtests and subbenchmarks.
type matcher struct {
	filter    filterMatch
	skip      filterMatch
	matchFunc func(pat, str string) (bool, error)

	mu sync.Mutex

	// subNames is used to deduplicate subtest names.
	// Each key is the subtest name joined to the deduplicated name of the parent test.
	// Each value is the count of the number of occurrences of the given subtest name
	// already seen.
	subNames map[string]int32
}

type filterMatch interface {
	// matches checks the name against the receiver's pattern strings using the
	// given match function.
	matches(name []string, matchString func(pat, str string) (bool, error)) (ok, partial bool)

	// verify checks that the receiver's pattern strings are valid filters by
	// calling the given match function.
	verify(name string, matchString func(pat, str string) (bool, error)) error
}

// simpleMatch matches a test name if all of the pattern strings match in
// sequence.
type simpleMatch []string

// alternationMatch matches a test name if one of the alternations match.
type alternationMatch []filterMatch

// TODO: fix test_main to avoid race and improve caching, also allowing to
// eliminate this Mutex.
var matchMutex sync.Mutex

func allMatcher() *matcher {
	return newMatcher(nil, "", "", "")
}

func newMatcher(matchString func(pat, str string) (bool, error), patterns, name, skips string) *matcher {
	var filter, skip filterMatch
	if patterns == "" {
		filter = simpleMatch{} // always partial true
	} else {
		filter = splitRegexp(patterns)
		if err := filter.verify(name, matchString); err != nil {
			fmt.Fprintf(os.Stderr, "testing: invalid regexp for %s\n", err)
			os.Exit(1)
		}
	}
	if skips == "" {
		skip = alternationMatch{} // always false
	} else {
		skip = splitRegexp(skips)
		if err := skip.verify("-test.skip", matchString); err != nil {
			fmt.Fprintf(os.Stderr, "testing: invalid regexp for %v\n", err)
			os.Exit(1)
		}
	}
	return &matcher{
		filter:    filter,
		skip:      skip,
		matchFunc: matchString,
		subNames:  map[string]int32{},
	}
}

func (m *matcher) fullName(c *common, subname string) (name string, ok, partial bool) {
	name = subname

	m.mu.Lock()
	defer m.mu.Unlock()

	if c != nil && c.level > 0 {
		name = m.unique(c.name, rewrite(subname))
	}

	matchMutex.Lock()
	defer matchMutex.Unlock()

	// We check the full array of paths each time to allow for the case that a pattern contains a '/'.
	elem := strings.Split(name, "/")

	// filter must match.
	// accept partial match that may produce full match later.
	ok, partial = m.filter.matches(elem, m.matchFunc)
	if !ok {
		return name, false, false
	}

	// skip must not match.
	// ignore partial match so we can get to more precise match later.
	skip, partialSkip := m.skip.matches(elem, m.matchFunc)
	if skip && !partialSkip {
		return name, false, false
	}

	return name, ok, partial
}

// clearSubNames clears the matcher's internal state, potentially freeing
// memory. After this is called, T.Name may return the same strings as it did
// for earlier subtests.
func (m *matcher) clearSubNames() {
	m.mu.Lock()
	defer m.mu.Unlock()
	clear(m.subNames)
}

func (m simpleMatch) matches(name []string, matchString func(pat, str string) (bool, error)) (ok, partial bool) {
	for i, s := range name {
		if i >= len(m) {
			break
		}
		if ok, _ := matchString(m[i], s); !ok {
			return false, false
		}
	}
	return true, len(name) < len(m)
}

func (m simpleMatch) verify(name string, matchString func(pat, str string) (bool, error)) error {
	for i, s := range m {
		m[i] = rewrite(s)
	}
	// Verify filters before doing any processing.
	for i, s := range m {
		if _, err := matchString(s, "non-empty"); err != nil {
			return fmt.Errorf("element %d of %s (%q): %s", i, name, s, err)
		}
	}
	return nil
}

func (m alternationMatch) matches(name []string, matchString func(pat, str string) (bool, error)) (ok, partial bool) {
	for _, m := range m {
		if ok, partial = m.matches(name, matchString); ok {
			return ok, partial
		}
	}
	return false, false
}

func (m alternationMatch) verify(name string, matchString func(pat, str string) (bool, error)) error {
	for i, m := range m {
		if err := m.verify(name, matchString); err != nil {
			return fmt.Errorf("alternation %d of %s", i, err)
		}
	}
	return nil
}

func splitRegexp(s string) filterMatch {
	a := make(simpleMatch, 0, strings.Count(s, "/"))
	b := make(alternationMatch, 0, strings.Count(s, "|"))
	cs := 0
	cp := 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '[':
			cs++
		case ']':
			if cs--; cs < 0 { // An unmatched ']' is legal.
				cs = 0
			}
		case '(':
			if cs == 0 {
				cp++
			}
		case ')':
			if cs == 0 {
				cp--
			}
		case '\\':
			i++
		case '/':
			if cs == 0 && cp == 0 {
				a = append(a, s[:i])
				s = s[i+1:]
				i = 0
				continue
			}
		case '|':
			if cs == 0 && cp == 0 {
				a = append(a, s[:i])
				s = s[i+1:]
				i = 0
				b = append(b, a)
				a = make(simpleMatch, 0, len(a))
				continue
			}
		}
		i++
	}

	a = append(a, s)
	if len(b) == 0 {
		return a
	}
	return append(b, a)
}

// unique creates a unique name for the given parent and subname by affixing it
// with one or more counts, if necessary.
func (m *matcher) unique(parent, subname string) string {
	base := parent + "/" + subname

	for {
		n := m.subNames[base]
		if n < 0 {
			panic("subtest count overflow")
		}
		m.subNames[base] = n + 1

		if n == 0 && subname != "" {
			prefix, nn := parseSubtestNumber(base)
			if len(prefix) < len(base) && nn < m.subNames[prefix] {
				// This test is explicitly named like "parent/subname#NN",
				// and #NN was already used for the NNth occurrence of "parent/subname".
				// Loop to add a disambiguating suffix.
				continue
			}
			return base
		}

		name := fmt.Sprintf("%s#%02d", base, n)
		if m.subNames[name] != 0 {
			// This is the nth occurrence of base, but the name "parent/subname#NN"
			// collides with the first occurrence of a subtest *explicitly* named
			// "parent/subname#NN". Try the next number.
			continue
		}

		return name
	}
}

// parseSubtestNumber splits a subtest name into a "#%02d"-formatted int32
// suffix (if present), and a prefix preceding that suffix (always).
func parseSubtestNumber(s string) (prefix string, nn int32) {
	i := strings.LastIndex(s, "#")
	if i < 0 {
		return s, 0
	}

	prefix, suffix := s[:i], s[i+1:]
	if len(suffix) < 2 || (len(suffix) > 2 && suffix[0] == '0') {
		// Even if suffix is numeric, it is not a possible output of a "%02" format
		// string: it has either too few digits or too many leading zeroes.
		return s, 0
	}
	if suffix == "00" {
		if !strings.HasSuffix(prefix, "/") {
			// We only use "#00" as a suffix for subtests named with the empty
			// string — it isn't a valid suffix if the subtest name is non-empty.
			return s, 0
		}
	}

	n, err := strconv.ParseInt(suffix, 10, 32)
	if err != nil || n < 0 {
		return s, 0
	}
	return prefix, int32(n)
}

// rewrite rewrites a subname to having only printable characters and no white
// space.
func rewrite(s string) string {
	b := []byte{}
	for _, r := range s {
		switch {
		case isSpace(r):
			b = append(b, '_')
		case !strconv.IsPrint(r):
			s := strconv.QuoteRune(r)
			b = append(b, s[1:len(s)-1]...)
		default:
			b = append(b, string(r)...)
		}
	}
	return string(b)
}

func isSpace(r rune) bool {
	if r < 0x2000 {
		switch r {
		// Note: not the same as Unicode Z class.
		case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0, 0x1680:
			return true
		}
	} else {
		if r <= 0x200a {
			return true
		}
		switch r {
		case 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
			return true
		}
	}
	return false
}

```

// === FILE: references/go/src/testing/newcover.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Support for test coverage with redesigned coverage implementation.

package testing

import (
	"fmt"
	"os"
	_ "unsafe" // for linkname
)

// cover variable stores the current coverage mode and a
// tear-down function to be called at the end of the testing run.
var cover struct {
	mode        string
	tearDown    func(coverprofile string, gocoverdir string) (string, error)
	snapshotcov func() float64
}

// registerCover is invoked during "go test -cover" runs.
// It is used to record a 'tear down' function
// (to be called when the test is complete) and the coverage mode.
func registerCover(mode string, tearDown func(coverprofile string, gocoverdir string) (string, error), snapcov func() float64) {
	if mode == "" {
		return
	}
	cover.mode = mode
	cover.tearDown = tearDown
	cover.snapshotcov = snapcov
}

// coverReport reports the coverage percentage and
// writes a coverage profile if requested.
// This invokes a callback in _testmain.go that will
// emit coverage data at the point where test execution is complete,
// for "go test -cover" runs.
func coverReport() {
	if errmsg, err := cover.tearDown(*coverProfile, *gocoverdir); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", errmsg, err)
		os.Exit(2)
	}
}

// Coverage reports the current code coverage as a fraction in the range [0, 1].
// If coverage is not enabled, Coverage returns 0.
//
// When running a large set of sequential test cases, checking Coverage after each one
// can be useful for identifying which test cases exercise new code paths.
// It is not a replacement for the reports generated by 'go test -cover' and
// 'go tool cover'.
func Coverage() float64 {
	if cover.mode == "" {
		return 0.0
	}
	return cover.snapshotcov()
}

```

// === FILE: references/go/src/testing/quick/quick.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package quick implements utility functions to help with black box testing.
//
// The testing/quick package is frozen and is not accepting new features.
package quick

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"time"
)

var defaultMaxCount *int = flag.Int("quickchecks", 100, "The default number of iterations for each check")

// A Generator can generate random values of its own type.
type Generator interface {
	// Generate returns a random instance of the type on which it is a
	// method using the size as a size hint.
	Generate(rand *rand.Rand, size int) reflect.Value
}

// randFloat32 generates a random float taking the full range of a float32.
func randFloat32(rand *rand.Rand) float32 {
	f := rand.Float64() * math.MaxFloat32
	if rand.Int()&1 == 1 {
		f = -f
	}
	return float32(f)
}

// randFloat64 generates a random float taking the full range of a float64.
func randFloat64(rand *rand.Rand) float64 {
	f := rand.Float64() * math.MaxFloat64
	if rand.Int()&1 == 1 {
		f = -f
	}
	return f
}

// randInt64 returns a random int64.
func randInt64(rand *rand.Rand) int64 {
	return int64(rand.Uint64())
}

// complexSize is the maximum length of arbitrary values that contain other
// values.
const complexSize = 50

// Value returns an arbitrary value of the given type.
// If the type implements the [Generator] interface, that will be used.
// Note: To create arbitrary values for structs, all the fields must be exported.
func Value(t reflect.Type, rand *rand.Rand) (value reflect.Value, ok bool) {
	return sizedValue(t, rand, complexSize)
}

// sizedValue returns an arbitrary value of the given type. The size
// hint is used for shrinking as a function of indirection level so
// that recursive data structures will terminate.
func sizedValue(t reflect.Type, rand *rand.Rand, size int) (value reflect.Value, ok bool) {
	if m, ok := reflect.TypeAssert[Generator](reflect.Zero(t)); ok {
		return m.Generate(rand, size), true
	}

	v := reflect.New(t).Elem()
	switch concrete := t; concrete.Kind() {
	case reflect.Bool:
		v.SetBool(rand.Int()&1 == 0)
	case reflect.Float32:
		v.SetFloat(float64(randFloat32(rand)))
	case reflect.Float64:
		v.SetFloat(randFloat64(rand))
	case reflect.Complex64:
		v.SetComplex(complex(float64(randFloat32(rand)), float64(randFloat32(rand))))
	case reflect.Complex128:
		v.SetComplex(complex(randFloat64(rand), randFloat64(rand)))
	case reflect.Int16:
		v.SetInt(randInt64(rand))
	case reflect.Int32:
		v.SetInt(randInt64(rand))
	case reflect.Int64:
		v.SetInt(randInt64(rand))
	case reflect.Int8:
		v.SetInt(randInt64(rand))
	case reflect.Int:
		v.SetInt(randInt64(rand))
	case reflect.Uint16:
		v.SetUint(uint64(randInt64(rand)))
	case reflect.Uint32:
		v.SetUint(uint64(randInt64(rand)))
	case reflect.Uint64:
		v.SetUint(uint64(randInt64(rand)))
	case reflect.Uint8:
		v.SetUint(uint64(randInt64(rand)))
	case reflect.Uint:
		v.SetUint(uint64(randInt64(rand)))
	case reflect.Uintptr:
		v.SetUint(uint64(randInt64(rand)))
	case reflect.Map:
		numElems := rand.Intn(size)
		v.Set(reflect.MakeMap(concrete))
		for i := 0; i < numElems; i++ {
			key, ok1 := sizedValue(concrete.Key(), rand, size)
			value, ok2 := sizedValue(concrete.Elem(), rand, size)
			if !ok1 || !ok2 {
				return reflect.Value{}, false
			}
			v.SetMapIndex(key, value)
		}
	case reflect.Pointer:
		if rand.Intn(size) == 0 {
			v.SetZero() // Generate nil pointer.
		} else {
			elem, ok := sizedValue(concrete.Elem(), rand, size)
			if !ok {
				return reflect.Value{}, false
			}
			v.Set(reflect.New(concrete.Elem()))
			v.Elem().Set(elem)
		}
	case reflect.Slice:
		numElems := rand.Intn(size)
		sizeLeft := size - numElems
		v.Set(reflect.MakeSlice(concrete, numElems, numElems))
		for i := 0; i < numElems; i++ {
			elem, ok := sizedValue(concrete.Elem(), rand, sizeLeft)
			if !ok {
				return reflect.Value{}, false
			}
			v.Index(i).Set(elem)
		}
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			elem, ok := sizedValue(concrete.Elem(), rand, size)
			if !ok {
				return reflect.Value{}, false
			}
			v.Index(i).Set(elem)
		}
	case reflect.String:
		numChars := rand.Intn(complexSize)
		codePoints := make([]rune, numChars)
		for i := 0; i < numChars; i++ {
			codePoints[i] = rune(rand.Intn(0x10ffff))
		}
		v.SetString(string(codePoints))
	case reflect.Struct:
		n := v.NumField()
		// Divide sizeLeft evenly among the struct fields.
		sizeLeft := size
		if n > sizeLeft {
			sizeLeft = 1
		} else if n > 0 {
			sizeLeft /= n
		}
		for i := 0; i < n; i++ {
			elem, ok := sizedValue(concrete.Field(i).Type, rand, sizeLeft)
			if !ok {
				return reflect.Value{}, false
			}
			v.Field(i).Set(elem)
		}
	default:
		return reflect.Value{}, false
	}

	return v, true
}

// A Config structure contains options for running a test.
type Config struct {
	// MaxCount sets the maximum number of iterations.
	// If zero, MaxCountScale is used.
	MaxCount int
	// MaxCountScale is a non-negative scale factor applied to the
	// default maximum.
	// A count of zero implies the default, which is usually 100
	// but can be set by the -quickchecks flag.
	MaxCountScale float64
	// Rand specifies a source of random numbers.
	// If nil, a default pseudo-random source will be used.
	Rand *rand.Rand
	// Values specifies a function to generate a slice of
	// arbitrary reflect.Values that are congruent with the
	// arguments to the function being tested.
	// If nil, the top-level Value function is used to generate them.
	Values func([]reflect.Value, *rand.Rand)
}

var defaultConfig Config

// getRand returns the *rand.Rand to use for a given Config.
func (c *Config) getRand() *rand.Rand {
	if c.Rand == nil {
		return rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return c.Rand
}

// getMaxCount returns the maximum number of iterations to run for a given
// Config.
func (c *Config) getMaxCount() (maxCount int) {
	maxCount = c.MaxCount
	if maxCount == 0 {
		if c.MaxCountScale != 0 {
			maxCount = int(c.MaxCountScale * float64(*defaultMaxCount))
		} else {
			maxCount = *defaultMaxCount
		}
	}

	return
}

// A SetupError is the result of an error in the way that check is being
// used, independent of the functions being tested.
type SetupError string

func (s SetupError) Error() string { return string(s) }

// A CheckError is the result of Check finding an error.
type CheckError struct {
	Count int
	In    []any
}

func (s *CheckError) Error() string {
	return fmt.Sprintf("#%d: failed on input %s", s.Count, toString(s.In))
}

// A CheckEqualError is the result [CheckEqual] finding an error.
type CheckEqualError struct {
	CheckError
	Out1 []any
	Out2 []any
}

func (s *CheckEqualError) Error() string {
	return fmt.Sprintf("#%d: failed on input %s. Output 1: %s. Output 2: %s", s.Count, toString(s.In), toString(s.Out1), toString(s.Out2))
}

// Check looks for an input to f, any function that returns bool,
// such that f returns false. It calls f repeatedly, with arbitrary
// values for each argument. If f returns false on a given input,
// Check returns that input as a *[CheckError].
// For example:
//
//	func TestOddMultipleOfThree(t *testing.T) {
//		f := func(x int) bool {
//			y := OddMultipleOfThree(x)
//			return y%2 == 1 && y%3 == 0
//		}
//		if err := quick.Check(f, nil); err != nil {
//			t.Error(err)
//		}
//	}
func Check(f any, config *Config) error {
	if config == nil {
		config = &defaultConfig
	}

	fVal, fType, ok := functionAndType(f)
	if !ok {
		return SetupError("argument is not a function")
	}

	if fType.NumOut() != 1 {
		return SetupError("function does not return one value")
	}
	if fType.Out(0).Kind() != reflect.Bool {
		return SetupError("function does not return a bool")
	}

	arguments := make([]reflect.Value, fType.NumIn())
	rand := config.getRand()
	maxCount := config.getMaxCount()

	for i := 0; i < maxCount; i++ {
		err := arbitraryValues(arguments, fType, config, rand)
		if err != nil {
			return err
		}

		if !fVal.Call(arguments)[0].Bool() {
			return &CheckError{i + 1, toInterfaces(arguments)}
		}
	}

	return nil
}

// CheckEqual looks for an input on which f and g return different results.
// It calls f and g repeatedly with arbitrary values for each argument.
// If f and g return different answers, CheckEqual returns a *[CheckEqualError]
// describing the input and the outputs.
func CheckEqual(f, g any, config *Config) error {
	if config == nil {
		config = &defaultConfig
	}

	x, xType, ok := functionAndType(f)
	if !ok {
		return SetupError("f is not a function")
	}
	y, yType, ok := functionAndType(g)
	if !ok {
		return SetupError("g is not a function")
	}

	if xType != yType {
		return SetupError("functions have different types")
	}

	arguments := make([]reflect.Value, xType.NumIn())
	rand := config.getRand()
	maxCount := config.getMaxCount()

	for i := 0; i < maxCount; i++ {
		err := arbitraryValues(arguments, xType, config, rand)
		if err != nil {
			return err
		}

		xOut := toInterfaces(x.Call(arguments))
		yOut := toInterfaces(y.Call(arguments))

		if !reflect.DeepEqual(xOut, yOut) {
			return &CheckEqualError{CheckError{i + 1, toInterfaces(arguments)}, xOut, yOut}
		}
	}

	return nil
}

// arbitraryValues writes Values to args such that args contains Values
// suitable for calling f.
func arbitraryValues(args []reflect.Value, f reflect.Type, config *Config, rand *rand.Rand) (err error) {
	if config.Values != nil {
		config.Values(args, rand)
		return
	}

	for j := 0; j < len(args); j++ {
		var ok bool
		args[j], ok = Value(f.In(j), rand)
		if !ok {
			err = SetupError(fmt.Sprintf("cannot create arbitrary value of type %s for argument %d", f.In(j), j))
			return
		}
	}

	return
}

func functionAndType(f any) (v reflect.Value, t reflect.Type, ok bool) {
	v = reflect.ValueOf(f)
	ok = v.Kind() == reflect.Func
	if !ok {
		return
	}
	t = v.Type()
	return
}

func toInterfaces(values []reflect.Value) []any {
	ret := make([]any, len(values))
	for i, v := range values {
		ret[i] = v.Interface()
	}
	return ret
}

func toString(interfaces []any) string {
	s := make([]string, len(interfaces))
	for i, v := range interfaces {
		s[i] = fmt.Sprintf("%#v", v)
	}
	return strings.Join(s, ", ")
}

```

// === FILE: references/go/src/testing/run_example.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !js && !wasip1

// TODO(@musiol, @odeke-em): re-unify this entire file back into
// example.go when js/wasm gets an os.Pipe implementation
// and no longer needs this separation.

package testing

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func runExample(eg InternalExample) (ok bool) {
	if chatty.on {
		fmt.Printf("%s=== RUN   %s\n", chatty.prefix(), eg.Name)
	}

	// Capture stdout.
	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout = w
	outC := make(chan string)
	go func() {
		var buf strings.Builder
		_, err := io.Copy(&buf, r)
		r.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "testing: copying pipe: %v\n", err)
			os.Exit(1)
		}
		outC <- buf.String()
	}()

	finished := false
	start := time.Now()

	// Clean up in a deferred call so we can recover if the example panics.
	defer func() {
		timeSpent := time.Since(start)

		// Close pipe, restore stdout, get output.
		w.Close()
		os.Stdout = stdout
		out := <-outC

		err := recover()
		ok = eg.processRunResult(out, timeSpent, finished, err)
	}()

	// Run example.
	eg.F()
	finished = true
	return
}

```

// === FILE: references/go/src/testing/run_example_wasm.go ===
```go
// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build js || wasip1

package testing

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// TODO(@musiol, @odeke-em): unify this code back into
// example.go when js/wasm gets an os.Pipe implementation.
func runExample(eg InternalExample) (ok bool) {
	if chatty.on {
		fmt.Printf("%s=== RUN   %s\n", chatty.prefix(), eg.Name)
	}

	// Capture stdout to temporary file. We're not using
	// os.Pipe because it is not supported on js/wasm.
	stdout := os.Stdout
	f := createTempFile(eg.Name)
	os.Stdout = f
	finished := false
	start := time.Now()

	// Clean up in a deferred call so we can recover if the example panics.
	defer func() {
		timeSpent := time.Since(start)

		// Restore stdout, get output and remove temporary file.
		os.Stdout = stdout
		var buf strings.Builder
		_, seekErr := f.Seek(0, io.SeekStart)
		_, readErr := io.Copy(&buf, f)
		out := buf.String()
		f.Close()
		os.Remove(f.Name())
		if seekErr != nil {
			fmt.Fprintf(os.Stderr, "testing: seek temp file: %v\n", seekErr)
			os.Exit(1)
		}
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "testing: read temp file: %v\n", readErr)
			os.Exit(1)
		}

		err := recover()
		ok = eg.processRunResult(out, timeSpent, finished, err)
	}()

	// Run example.
	eg.F()
	finished = true
	return
}

func createTempFile(exampleName string) *os.File {
	for i := 0; ; i++ {
		name := fmt.Sprintf("%s/go-example-stdout-%s-%d.txt", os.TempDir(), exampleName, i)
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			fmt.Fprintf(os.Stderr, "testing: open temp file: %v\n", err)
			os.Exit(1)
		}
		return f
	}
}

```

// === FILE: references/go/src/testing/slogtest/slogtest.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package slogtest implements support for testing implementations of log/slog.Handler.
package slogtest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"testing"
	"time"
)

type testCase struct {
	// Subtest name.
	name string
	// If non-empty, explanation explains the violated constraint.
	explanation string
	// f executes a single log event using its argument logger.
	// So that mkdescs.sh can generate the right description,
	// the body of f must appear on a single line whose first
	// non-whitespace characters are "l.".
	f func(*slog.Logger)
	// If mod is not nil, it is called to modify the Record
	// generated by the Logger before it is passed to the Handler.
	mod func(*slog.Record)
	// checks is a list of checks to run on the result.
	checks []check
}

var cases = []testCase{
	{
		name:        "built-ins",
		explanation: withSource("this test expects slog.TimeKey, slog.LevelKey and slog.MessageKey"),
		f: func(l *slog.Logger) {
			l.Info("message")
		},
		checks: []check{
			hasKey(slog.TimeKey),
			hasKey(slog.LevelKey),
			hasAttr(slog.MessageKey, "message"),
		},
	},
	{
		name:        "attrs",
		explanation: withSource("a Handler should output attributes passed to the logging function"),
		f: func(l *slog.Logger) {
			l.Info("message", "k", "v")
		},
		checks: []check{
			hasAttr("k", "v"),
		},
	},
	{
		name:        "empty-attr",
		explanation: withSource("a Handler should ignore an empty Attr"),
		f: func(l *slog.Logger) {
			l.Info("msg", "a", "b", "", nil, "c", "d")
		},
		checks: []check{
			hasAttr("a", "b"),
			missingKey(""),
			hasAttr("c", "d"),
		},
	},
	{
		name:        "zero-time",
		explanation: withSource("a Handler should ignore a zero Record.Time"),
		f: func(l *slog.Logger) {
			l.Info("msg", "k", "v")
		},
		mod: func(r *slog.Record) { r.Time = time.Time{} },
		checks: []check{
			missingKey(slog.TimeKey),
		},
	},
	{
		name:        "WithAttrs",
		explanation: withSource("a Handler should include the attributes from the WithAttrs method"),
		f: func(l *slog.Logger) {
			l.With("a", "b").Info("msg", "k", "v")
		},
		checks: []check{
			hasAttr("a", "b"),
			hasAttr("k", "v"),
		},
	},
	{
		name:        "groups",
		explanation: withSource("a Handler should handle Group attributes"),
		f: func(l *slog.Logger) {
			l.Info("msg", "a", "b", slog.Group("G", slog.String("c", "d")), "e", "f")
		},
		checks: []check{
			hasAttr("a", "b"),
			inGroup("G", hasAttr("c", "d")),
			hasAttr("e", "f"),
		},
	},
	{
		name:        "empty-group",
		explanation: withSource("a Handler should ignore an empty group"),
		f: func(l *slog.Logger) {
			l.Info("msg", "a", "b", slog.Group("G"), "e", "f")
		},
		checks: []check{
			hasAttr("a", "b"),
			missingKey("G"),
			hasAttr("e", "f"),
		},
	},
	{
		name:        "inline-group",
		explanation: withSource("a Handler should inline the Attrs of a group with an empty key"),
		f: func(l *slog.Logger) {
			l.Info("msg", "a", "b", slog.Group("", slog.String("c", "d")), "e", "f")

		},
		checks: []check{
			hasAttr("a", "b"),
			hasAttr("c", "d"),
			hasAttr("e", "f"),
		},
	},
	{
		name:        "WithGroup",
		explanation: withSource("a Handler should handle the WithGroup method"),
		f: func(l *slog.Logger) {
			l.WithGroup("G").Info("msg", "a", "b")
		},
		checks: []check{
			hasKey(slog.TimeKey),
			hasKey(slog.LevelKey),
			hasAttr(slog.MessageKey, "msg"),
			missingKey("a"),
			inGroup("G", hasAttr("a", "b")),
		},
	},
	{
		name:        "multi-With",
		explanation: withSource("a Handler should handle multiple WithGroup and WithAttr calls"),
		f: func(l *slog.Logger) {
			l.With("a", "b").WithGroup("G").With("c", "d").WithGroup("H").Info("msg", "e", "f")
		},
		checks: []check{
			hasKey(slog.TimeKey),
			hasKey(slog.LevelKey),
			hasAttr(slog.MessageKey, "msg"),
			hasAttr("a", "b"),
			inGroup("G", hasAttr("c", "d")),
			inGroup("G", inGroup("H", hasAttr("e", "f"))),
		},
	},
	{
		name:        "empty-group-record",
		explanation: withSource("a Handler should not output groups if there are no attributes"),
		f: func(l *slog.Logger) {
			l.With("a", "b").WithGroup("G").With("c", "d").WithGroup("H").Info("msg")
		},
		checks: []check{
			hasKey(slog.TimeKey),
			hasKey(slog.LevelKey),
			hasAttr(slog.MessageKey, "msg"),
			hasAttr("a", "b"),
			inGroup("G", hasAttr("c", "d")),
			inGroup("G", missingKey("H")),
		},
	},
	{
		name:        "nested-empty-group-record",
		explanation: withSource("a Handler should not output nested groups if there are no attributes"),
		f: func(l *slog.Logger) {
			l.With("a", "b").WithGroup("G").With("c", "d").WithGroup("H").WithGroup("I").Info("msg")
		},
		checks: []check{
			hasKey(slog.TimeKey),
			hasKey(slog.LevelKey),
			hasAttr(slog.MessageKey, "msg"),
			hasAttr("a", "b"),
			inGroup("G", hasAttr("c", "d")),
			inGroup("G", missingKey("H")),
			inGroup("G", missingKey("I")),
		},
	},
	{
		name:        "resolve",
		explanation: withSource("a Handler should call Resolve on attribute values"),
		f: func(l *slog.Logger) {
			l.Info("msg", "k", &replace{"replaced"})
		},
		checks: []check{hasAttr("k", "replaced")},
	},
	{
		name:        "resolve-groups",
		explanation: withSource("a Handler should call Resolve on attribute values in groups"),
		f: func(l *slog.Logger) {
			l.Info("msg",
				slog.Group("G",
					slog.String("a", "v1"),
					slog.Any("b", &replace{"v2"})))
		},
		checks: []check{
			inGroup("G", hasAttr("a", "v1")),
			inGroup("G", hasAttr("b", "v2")),
		},
	},
	{
		name:        "resolve-WithAttrs",
		explanation: withSource("a Handler should call Resolve on attribute values from WithAttrs"),
		f: func(l *slog.Logger) {
			l = l.With("k", &replace{"replaced"})
			l.Info("msg")
		},
		checks: []check{hasAttr("k", "replaced")},
	},
	{
		name:        "resolve-WithAttrs-groups",
		explanation: withSource("a Handler should call Resolve on attribute values in groups from WithAttrs"),
		f: func(l *slog.Logger) {
			l = l.With(slog.Group("G",
				slog.String("a", "v1"),
				slog.Any("b", &replace{"v2"})))
			l.Info("msg")
		},
		checks: []check{
			inGroup("G", hasAttr("a", "v1")),
			inGroup("G", hasAttr("b", "v2")),
		},
	},
	{
		name:        "empty-PC",
		explanation: withSource("a Handler should not output SourceKey if the PC is zero"),
		f: func(l *slog.Logger) {
			l.Info("message")
		},
		mod: func(r *slog.Record) { r.PC = 0 },
		checks: []check{
			missingKey(slog.SourceKey),
		},
	},
}

// TestHandler tests a [slog.Handler].
// If TestHandler finds any misbehaviors, it returns an error for each,
// combined into a single error with [errors.Join].
//
// TestHandler installs the given Handler in a [slog.Logger] and
// makes several calls to the Logger's output methods.
// The Handler should be enabled for levels Info and above.
//
// The results function is invoked after all such calls.
// It should return a slice of map[string]any, one for each call to a Logger output method.
// The keys and values of the map should correspond to the keys and values of the Handler's
// output. Each group in the output should be represented as its own nested map[string]any.
// The standard keys [slog.TimeKey], [slog.LevelKey] and [slog.MessageKey] should be used.
//
// If the Handler outputs JSON, then calling [encoding/json.Unmarshal] with a `map[string]any`
// will create the right data structure.
//
// If a Handler intentionally drops an attribute that is checked by a test,
// then the results function should check for its absence and add it to the map it returns.
func TestHandler(h slog.Handler, results func() []map[string]any) error {
	// Run the handler on the test cases.
	for _, c := range cases {
		ht := h
		if c.mod != nil {
			ht = &wrapper{h, c.mod}
		}
		l := slog.New(ht)
		c.f(l)
	}

	// Collect and check the results.
	var errs []error
	res := results()
	if g, w := len(res), len(cases); g != w {
		return fmt.Errorf("got %d results, want %d", g, w)
	}
	for i, got := range res {
		c := cases[i]
		for _, check := range c.checks {
			if problem := check(got); problem != "" {
				errs = append(errs, fmt.Errorf("%s: %s", problem, c.explanation))
			}
		}
	}
	return errors.Join(errs...)
}

// Run exercises a [slog.Handler] on the same test cases as [TestHandler], but
// runs each case in a subtest. For each test case, it first calls newHandler to
// get an instance of the handler under test, then runs the test case, then
// calls result to get the result. If the test case fails, it calls t.Error.
func Run(t *testing.T, newHandler func(*testing.T) slog.Handler, result func(*testing.T) map[string]any) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHandler(t)
			if c.mod != nil {
				h = &wrapper{h, c.mod}
			}
			l := slog.New(h)
			c.f(l)
			got := result(t)
			for _, check := range c.checks {
				if p := check(got); p != "" {
					t.Errorf("%s: %s", p, c.explanation)
				}
			}
		})
	}
}

type check func(map[string]any) string

func hasKey(key string) check {
	return func(m map[string]any) string {
		if _, ok := m[key]; !ok {
			return fmt.Sprintf("missing key %q", key)
		}
		return ""
	}
}

func missingKey(key string) check {
	return func(m map[string]any) string {
		if _, ok := m[key]; ok {
			return fmt.Sprintf("unexpected key %q", key)
		}
		return ""
	}
}

func hasAttr(key string, wantVal any) check {
	return func(m map[string]any) string {
		if s := hasKey(key)(m); s != "" {
			return s
		}
		gotVal := m[key]
		if !reflect.DeepEqual(gotVal, wantVal) {
			return fmt.Sprintf("%q: got %#v, want %#v", key, gotVal, wantVal)
		}
		return ""
	}
}

func inGroup(name string, c check) check {
	return func(m map[string]any) string {
		v, ok := m[name]
		if !ok {
			return fmt.Sprintf("missing group %q", name)
		}
		g, ok := v.(map[string]any)
		if !ok {
			return fmt.Sprintf("value for group %q is not map[string]any", name)
		}
		return c(g)
	}
}

type wrapper struct {
	slog.Handler
	mod func(*slog.Record)
}

func (h *wrapper) Handle(ctx context.Context, r slog.Record) error {
	h.mod(&r)
	return h.Handler.Handle(ctx, r)
}

func withSource(s string) string {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		panic("runtime.Caller failed")
	}
	return fmt.Sprintf("%s (%s:%d)", s, file, line)
}

type replace struct {
	v any
}

func (r *replace) LogValue() slog.Value { return slog.AnyValue(r.v) }

func (r *replace) String() string {
	return fmt.Sprintf("<replace(%v)>", r.v)
}

```

// === FILE: references/go/src/testing/synctest/synctest.go ===
```go
// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package synctest provides support for testing concurrent code.
//
// The [Test] function runs a function in an isolated "bubble".
// Any goroutines started within the bubble are also part of the bubble.
//
// Each test should be entirely self-contained:
// The following guidelines should apply to most tests:
//
//   - Avoid interacting with goroutines not started from within the test.
//   - Avoid using the network. Use a fake network implementation as needed.
//   - Avoid interacting with external processes.
//   - Avoid leaking goroutines in background tasks.
//
// # Time
//
// Within a bubble, the [time] package uses a fake clock.
// Each bubble has its own clock.
// The initial time is midnight UTC 2000-01-01.
//
// Time in a bubble only advances when every goroutine in the
// bubble is durably blocked.
// See below for the exact definition of "durably blocked".
//
// For example, this test runs immediately rather than taking
// two seconds:
//
//	func TestTime(t *testing.T) {
//		synctest.Test(t, func(t *testing.T) {
//			start := time.Now() // always midnight UTC 2000-01-01
//			go func() {
//				time.Sleep(1 * time.Second)
//				t.Log(time.Since(start)) // always logs "1s"
//			}()
//			time.Sleep(2 * time.Second) // the goroutine above will run before this Sleep returns
//			t.Log(time.Since(start))    // always logs "2s"
//		})
//	}
//
// Time stops advancing when the root goroutine of the bubble exits.
//
// # Blocking
//
// A goroutine in a bubble is "durably blocked" when it is blocked
// and can only be unblocked by another goroutine in the same bubble.
// A goroutine which can be unblocked by an event from outside its
// bubble is not durably blocked.
//
// The [Wait] function blocks until all other goroutines in the
// bubble are durably blocked.
//
// For example:
//
//	func TestWait(t *testing.T) {
//		synctest.Test(t, func(t *testing.T) {
//			done := false
//			go func() {
//				done = true
//			}()
//			// Wait will block until the goroutine above has finished.
//			synctest.Wait()
//			t.Log(done) // always logs "true"
//		})
//	}
//
// When every goroutine in a bubble is durably blocked:
//
//   - [Wait] returns, if it has been called.
//   - Otherwise, time advances to the next time that will
//     unblock at least one goroutine, if there is such a time
//     and the root goroutine of the bubble has not exited.
//   - Otherwise, there is a deadlock and [Test] panics.
//
// The following operations durably block a goroutine:
//
//   - a blocking send or receive on a channel created within the bubble
//   - a blocking select statement where every case is a channel created
//     within the bubble
//   - [sync.Cond.Wait]
//   - [sync.WaitGroup.Wait], when [sync.WaitGroup.Add] was called within the bubble
//   - [time.Sleep]
//
// Operations not in the above list are not durably blocking.
// In particular, the following operations may block a goroutine,
// but are not durably blocking because the goroutine can be unblocked
// by an event occurring outside its bubble:
//
//   - locking a [sync.Mutex] or [sync.RWMutex]
//   - blocking on I/O, such as reading from a network socket
//   - system calls
//
// # Isolation
//
// A channel, [time.Timer], or [time.Ticker] created within a bubble
// is associated with it. Operating on a bubbled channel, timer, or
// ticker from outside the bubble panics.
//
// A [sync.WaitGroup] becomes associated with a bubble on the first
// call to Add or Go. Once a WaitGroup is associated with a bubble,
// calling Add or Go from outside that bubble is a fatal error.
// (As a technical limitation, a WaitGroup defined as a package
// variable, such as "var wg sync.WaitGroup", cannot be associated
// with a bubble and operations on it may not be durably blocking.
// This limitation does not apply to a *WaitGroup stored in a
// package variable, such as "var wg = new(sync.WaitGroup)".)
//
// [sync.Cond.Wait] is durably blocking. Waking a goroutine in a bubble
// blocked on Cond.Wait from outside the bubble is a fatal error.
//
// Cleanup functions and finalizers registered with
// [runtime.AddCleanup] and [runtime.SetFinalizer]
// run outside of any bubble.
//
// # Example: Context.AfterFunc
//
// This example demonstrates testing the [context.AfterFunc] function.
//
// AfterFunc registers a function to execute in a new goroutine
// after a context is canceled.
//
// The test verifies that the function is not run before the context is canceled,
// and is run after the context is canceled.
//
//	func TestContextAfterFunc(t *testing.T) {
//		synctest.Test(t, func(t *testing.T) {
//			// Create a context.Context which can be canceled.
//			ctx, cancel := context.WithCancel(t.Context())
//
//			// context.AfterFunc registers a function to be called
//			// when a context is canceled.
//			afterFuncCalled := false
//			context.AfterFunc(ctx, func() {
//				afterFuncCalled = true
//			})
//
//			// The context has not been canceled, so the AfterFunc is not called.
//			synctest.Wait()
//			if afterFuncCalled {
//				t.Fatalf("before context is canceled: AfterFunc called")
//			}
//
//			// Cancel the context and wait for the AfterFunc to finish executing.
//			// Verify that the AfterFunc ran.
//			cancel()
//			synctest.Wait()
//			if !afterFuncCalled {
//				t.Fatalf("after context is canceled: AfterFunc not called")
//			}
//		})
//	}
//
// # Example: Context.WithTimeout
//
// This example demonstrates testing the [context.WithTimeout] function.
//
// WithTimeout creates a context which is canceled after a timeout.
//
// The test verifies that the context is not canceled before the timeout expires,
// and is canceled after the timeout expires.
//
//	func TestContextWithTimeout(t *testing.T) {
//		synctest.Test(t, func(t *testing.T) {
//			// Create a context.Context which is canceled after a timeout.
//			const timeout = 5 * time.Second
//			ctx, cancel := context.WithTimeout(t.Context(), timeout)
//			defer cancel()
//
//			// Wait just less than the timeout.
//			time.Sleep(timeout - time.Nanosecond)
//			synctest.Wait()
//			if err := ctx.Err(); err != nil {
//				t.Fatalf("before timeout: ctx.Err() = %v, want nil\n", err)
//			}
//
//			// Wait the rest of the way until the timeout.
//			time.Sleep(time.Nanosecond)
//			synctest.Wait()
//			if err := ctx.Err(); err != context.DeadlineExceeded {
//				t.Fatalf("after timeout: ctx.Err() = %v, want DeadlineExceeded\n", err)
//			}
//		})
//	}
//
// # Example: HTTP 100 Continue
//
// This example demonstrates testing [http.Transport]'s 100 Continue handling.
//
// An HTTP client sending a request can include an "Expect: 100-continue" header
// to tell the server that the client has additional data to send.
// The server may then respond with an 100 Continue information response
// to request the data, or some other status to tell the client the data is not needed.
// For example, a client uploading a large file might use this feature to confirm
// that the server is willing to accept the file before sending it.
//
// This test confirms that when sending an "Expect: 100-continue" header
// the HTTP client does not send a request's content before the server requests it,
// and that it does send the content after receiving a 100 Continue response.
//
//	func TestHTTPTransport100Continue(t *testing.T) {
//		synctest.Test(t, func(*testing.T) {
//			// Create an in-process fake network connection.
//			// We cannot use a loopback network connection for this test,
//			// because goroutines blocked on network I/O prevent a synctest
//			// bubble from becoming idle.
//			srvConn, cliConn := net.Pipe()
//			defer cliConn.Close()
//			defer srvConn.Close()
//
//			tr := &http.Transport{
//				// Use the fake network connection created above.
//				DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
//					return cliConn, nil
//				},
//				// Enable "Expect: 100-continue" handling.
//				ExpectContinueTimeout: 5 * time.Second,
//			}
//
//			// Send a request with the "Expect: 100-continue" header set.
//			// Send it in a new goroutine, since it won't complete until the end of the test.
//			body := "request body"
//			go func() {
//				req, _ := http.NewRequest("PUT", "http://test.tld/", strings.NewReader(body))
//				req.Header.Set("Expect", "100-continue")
//				resp, err := tr.RoundTrip(req)
//				if err != nil {
//					t.Errorf("RoundTrip: unexpected error %v\n", err)
//				} else {
//					resp.Body.Close()
//				}
//			}()
//
//			// Read the request headers sent by the client.
//			req, err := http.ReadRequest(bufio.NewReader(srvConn))
//			if err != nil {
//				t.Fatalf("ReadRequest: %v\n", err)
//			}
//
//			// Start a new goroutine copying the body sent by the client into a buffer.
//			// Wait for all goroutines in the bubble to block and verify that we haven't
//			// read anything from the client yet.
//			var gotBody bytes.Buffer
//			go io.Copy(&gotBody, req.Body)
//			synctest.Wait()
//			if got, want := gotBody.String(), ""; got != want {
//				t.Fatalf("before sending 100 Continue, read body: %q, want %q\n", got, want)
//			}
//
//			// Write a "100 Continue" response to the client and verify that
//			// it sends the request body.
//			srvConn.Write([]byte("HTTP/1.1 100 Continue\r\n\r\n"))
//			synctest.Wait()
//			if got, want := gotBody.String(), body; got != want {
//				t.Fatalf("after sending 100 Continue, read body: %q, want %q\n", got, want)
//			}
//
//			// Finish up by sending the "200 OK" response to conclude the request.
//			srvConn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
//
//			// We started several goroutines during the test.
//			// The synctest.Test call will wait for all of them to exit before returning.
//		})
//	}
package synctest

import (
	"internal/synctest"
	"testing"
	"time"
	_ "unsafe" // for linkname
)

// Test executes f in a new bubble.
//
// Test waits for all goroutines in the bubble to exit before returning.
// If the goroutines in the bubble become deadlocked, the test fails.
//
// Test must not be called from within a bubble.
//
// The [*testing.T] provided to f has the following properties:
//
//   - T.Cleanup functions run inside the bubble,
//     immediately before Test returns.
//   - T.Context returns a [context.Context] with a Done channel
//     associated with the bubble.
//   - T.Run, T.Parallel, and T.Deadline must not be called.
func Test(t *testing.T, f func(*testing.T)) {
	var ok bool
	synctest.Run(func() {
		ok = testingSynctestTest(t, f)
	})
	if !ok {
		// Fail the test outside the bubble,
		// so test durations get set using real time.
		t.FailNow()
	}
}

//go:linkname testingSynctestTest testing/synctest.testingSynctestTest
func testingSynctestTest(t *testing.T, f func(*testing.T)) bool

// Wait blocks until every goroutine within the current bubble,
// other than the current goroutine, is durably blocked.
//
// Wait must not be called from outside a bubble.
// Wait must not be called concurrently by multiple goroutines
// in the same bubble.
func Wait() {
	synctest.Wait()
}

// Sleep blocks until the current bubble's clock has advanced
// by the duration of d and every goroutine within the current bubble,
// other than the current goroutine, is durably blocked.
//
// This is exactly equivalent to
//
//	time.Sleep(d)
//	synctest.Wait()
//
// In tests, this is often preferable to calling only [time.Sleep].
// If the test itself and another goroutine running the system under test
// sleeps for the exact same amount of time, it's unpredictable which
// of the two goroutines will run first. The test itself usually wants
// to wait for the system under test to "settle" after sleeping.
// This is what Sleep accomplishes.
func Sleep(d time.Duration) {
	time.Sleep(d)
	Wait()
}

```

// === FILE: references/go/src/testing/testing.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testing provides support for automated testing of Go packages.
// It is intended to be used in concert with the "go test" command, which automates
// execution of any function of the form
//
//	func TestXxx(*testing.T)
//
// where Xxx does not start with a lowercase letter. The function name
// serves to identify the test routine.
//
// Within these functions, use [T.Error], [T.Fail] or related methods to signal failure.
//
// To write a new test suite, create a file that
// contains the TestXxx functions as described here,
// and give that file a name ending in "_test.go".
// The file will be excluded from regular
// package builds but will be included when the "go test" command is run.
//
// The test file can be in the same package as the one being tested,
// or in a corresponding package with the suffix "_test".
//
// If the test file is in the same package, it may refer to unexported
// identifiers within the package, as in this example:
//
//	package abs
//
//	import "testing"
//
//	func TestAbs(t *testing.T) {
//	    got := abs(-1)
//	    if got != 1 {
//	        t.Errorf("abs(-1) = %d; want 1", got)
//	    }
//	}
//
// If the file is in a separate "_test" package, the package being tested
// must be imported explicitly and only its exported identifiers may be used.
// This is known as "black box" testing.
//
//	package abs_test
//
//	import (
//		"testing"
//
//		"path_to_pkg/abs"
//	)
//
//	func TestAbs(t *testing.T) {
//	    got := abs.Abs(-1)
//	    if got != 1 {
//	        t.Errorf("Abs(-1) = %d; want 1", got)
//	    }
//	}
//
// For more detail, run [go help test] and [go help testflag].
//
// # Benchmarks
//
// Functions of the form
//
//	func BenchmarkXxx(*testing.B)
//
// are considered benchmarks, and are executed by the "go test" command when
// its -bench flag is provided. Benchmarks are run sequentially.
//
// For a description of the testing flags, see [go help testflag].
//
// A sample benchmark function looks like this:
//
//	func BenchmarkRandInt(b *testing.B) {
//	    for b.Loop() {
//	        rand.Int()
//	    }
//	}
//
// The output
//
//	BenchmarkRandInt-8   	68453040	        17.8 ns/op
//
// means that the body of the loop ran 68453040 times at a speed of 17.8 ns per loop.
//
// Only the body of the loop is timed, so benchmarks may do expensive
// setup before calling b.Loop, which will not be counted toward the
// benchmark measurement:
//
//	func BenchmarkBigLen(b *testing.B) {
//	    big := NewBig()
//	    for b.Loop() {
//	        big.Len()
//	    }
//	}
//
// If a benchmark needs to test performance in a parallel setting, it may use
// the RunParallel helper function; such benchmarks are intended to be used with
// the go test -cpu flag:
//
//	func BenchmarkTemplateParallel(b *testing.B) {
//	    templ := template.Must(template.New("test").Parse("Hello, {{.}}!"))
//	    b.RunParallel(func(pb *testing.PB) {
//	        var buf bytes.Buffer
//	        for pb.Next() {
//	            buf.Reset()
//	            templ.Execute(&buf, "World")
//	        }
//	    })
//	}
//
// A detailed specification of the benchmark results format is given
// in https://go.dev/design/14313-benchmark-format.
//
// There are standard tools for working with benchmark results at
// [golang.org/x/perf/cmd].
// In particular, [golang.org/x/perf/cmd/benchstat] performs
// statistically robust A/B comparisons.
//
// # b.N-style benchmarks
//
// Prior to the introduction of [B.Loop], benchmarks were written in a
// different style using B.N. For example:
//
//	func BenchmarkRandInt(b *testing.B) {
//	    for range b.N {
//	        rand.Int()
//	    }
//	}
//
// In this style of benchmark, the benchmark function must run
// the target code b.N times. The benchmark function is called
// multiple times with b.N adjusted until the benchmark function
// lasts long enough to be timed reliably. This also means any setup
// done before the loop may be run several times.
//
// If a benchmark needs some expensive setup before running, the timer
// should be explicitly reset:
//
//	func BenchmarkBigLen(b *testing.B) {
//	    big := NewBig()
//	    b.ResetTimer()
//	    for range b.N {
//	        big.Len()
//	    }
//	}
//
// New benchmarks should prefer using [B.Loop], which is more robust
// and more efficient.
//
// # Examples
//
// The package also runs and verifies example code. Example functions may
// include a concluding line comment that begins with "Output:" and is compared with
// the standard output of the function when the tests are run. (The comparison
// ignores leading and trailing space.) These are examples of an example:
//
//	func ExampleHello() {
//	    fmt.Println("hello")
//	    // Output: hello
//	}
//
//	func ExampleSalutations() {
//	    fmt.Println("hello, and")
//	    fmt.Println("goodbye")
//	    // Output:
//	    // hello, and
//	    // goodbye
//	}
//
// The comment prefix "Unordered output:" is like "Output:", but matches any
// line order:
//
//	func ExamplePerm() {
//	    for _, value := range Perm(5) {
//	        fmt.Println(value)
//	    }
//	    // Unordered output: 4
//	    // 2
//	    // 1
//	    // 3
//	    // 0
//	}
//
// Example functions without output comments are compiled but not executed.
//
// The naming convention to declare examples for the package, a function F, a type T and
// method M on type T are:
//
//	func Example() { ... }
//	func ExampleF() { ... }
//	func ExampleT() { ... }
//	func ExampleT_M() { ... }
//
// Multiple example functions for a package/type/function/method may be provided by
// appending a distinct suffix to the name. The suffix must start with a
// lower-case letter.
//
//	func Example_suffix() { ... }
//	func ExampleF_suffix() { ... }
//	func ExampleT_suffix() { ... }
//	func ExampleT_M_suffix() { ... }
//
// The entire test file is presented as the example when it contains a single
// example function, at least one other function, type, variable, or constant
// declaration, and no test or benchmark functions.
//
// # Fuzzing
//
// 'go test' and the testing package support fuzzing, a testing technique where
// a function is called with randomly generated inputs to find bugs not
// anticipated by unit tests.
//
// Functions of the form
//
//	func FuzzXxx(*testing.F)
//
// are considered fuzz tests.
//
// For example:
//
//	func FuzzHex(f *testing.F) {
//	  for _, seed := range [][]byte{{}, {0}, {9}, {0xa}, {0xf}, {1, 2, 3, 4}} {
//	    f.Add(seed)
//	  }
//	  f.Fuzz(func(t *testing.T, in []byte) {
//	    enc := hex.EncodeToString(in)
//	    out, err := hex.DecodeString(enc)
//	    if err != nil {
//	      t.Fatalf("%v: decode: %v", in, err)
//	    }
//	    if !bytes.Equal(in, out) {
//	      t.Fatalf("%v: not equal after round trip: %v", in, out)
//	    }
//	  })
//	}
//
// A fuzz test maintains a seed corpus, or a set of inputs which are run by
// default, and can seed input generation. Seed inputs may be registered by
// calling [F.Add] or by storing files in the directory testdata/fuzz/<Name>
// (where <Name> is the name of the fuzz test) within the package containing
// the fuzz test. Seed inputs are optional, but the fuzzing engine may find
// bugs more efficiently when provided with a set of small seed inputs with good
// code coverage. These seed inputs can also serve as regression tests for bugs
// identified through fuzzing.
//
// The function passed to [F.Fuzz] within the fuzz test is considered the fuzz
// target. A fuzz target must accept a [*T] parameter, followed by one or more
// parameters for random inputs. The types of arguments passed to [F.Add] must
// be identical to the types of these parameters. The fuzz target may signal
// that it's found a problem the same way tests do: by calling [T.Fail] (or any
// method that calls it like [T.Error] or [T.Fatal]) or by panicking.
//
// When fuzzing is enabled (by setting the -fuzz flag to a regular expression
// that matches a specific fuzz test), the fuzz target is called with arguments
// generated by repeatedly making random changes to the seed inputs. On
// supported platforms, 'go test' compiles the test executable with fuzzing
// coverage instrumentation. The fuzzing engine uses that instrumentation to
// find and cache inputs that expand coverage, increasing the likelihood of
// finding bugs. If the fuzz target fails for a given input, the fuzzing engine
// writes the inputs that caused the failure to a file in the directory
// testdata/fuzz/<Name> within the package directory. This file later serves as
// a seed input. If the file can't be written at that location (for example,
// because the directory is read-only), the fuzzing engine writes the file to
// the fuzz cache directory within the build cache instead.
//
// When fuzzing is disabled, the fuzz target is called with the seed inputs
// registered with [F.Add] and seed inputs from testdata/fuzz/<Name>. In this
// mode, the fuzz test acts much like a regular test, with subtests started
// with [F.Fuzz] instead of [T.Run].
//
// See https://go.dev/doc/fuzz for documentation about fuzzing.
//
// # Skipping
//
// Tests or benchmarks may be skipped at run time with a call to
// [T.Skip] or [B.Skip]:
//
//	func TestTimeConsuming(t *testing.T) {
//	    if testing.Short() {
//	        t.Skip("skipping test in short mode.")
//	    }
//	    ...
//	}
//
// The [T.Skip] method can be used in a fuzz target if the input is invalid,
// but should not be considered a failing input. For example:
//
//	func FuzzJSONMarshaling(f *testing.F) {
//	    f.Fuzz(func(t *testing.T, b []byte) {
//	        var v interface{}
//	        if err := json.Unmarshal(b, &v); err != nil {
//	            t.Skip()
//	        }
//	        if _, err := json.Marshal(v); err != nil {
//	            t.Errorf("Marshal: %v", err)
//	        }
//	    })
//	}
//
// # Subtests and Sub-benchmarks
//
// The [T.Run] and [B.Run] methods allow defining subtests and sub-benchmarks,
// without having to define separate functions for each. This enables uses
// like table-driven benchmarks and creating hierarchical tests.
// It also provides a way to share common setup and tear-down code:
//
//	func TestFoo(t *testing.T) {
//	    // <setup code>
//	    t.Run("A=1", func(t *testing.T) { ... })
//	    t.Run("A=2", func(t *testing.T) { ... })
//	    t.Run("B=1", func(t *testing.T) { ... })
//	    // <tear-down code>
//	}
//
// Each subtest and sub-benchmark has a unique name: the combination of the name
// of the top-level test and the sequence of names passed to Run, separated by
// slashes, with an optional trailing sequence number for disambiguation.
//
// The argument to the -run, -bench, and -fuzz command-line flags is an unanchored regular
// expression that matches the test's name. For tests with multiple slash-separated
// elements, such as subtests, the argument is itself slash-separated, with
// expressions matching each name element in turn. Because it is unanchored, an
// empty expression matches any string.
// For example, using "matching" to mean "whose name contains":
//
//	go test -run ''        # Run all tests.
//	go test -run Foo       # Run top-level tests matching "Foo", such as "TestFooBar".
//	go test -run Foo/A=    # For top-level tests matching "Foo", run subtests matching "A=".
//	go test -run /A=1      # For all top-level tests, run subtests matching "A=1".
//	go test -fuzz FuzzFoo  # Fuzz the target matching "FuzzFoo"
//
// The -run argument can also be used to run a specific value in the seed
// corpus, for debugging. For example:
//
//	go test -run=FuzzFoo/9ddb952d9814
//
// The -fuzz and -run flags can both be set, in order to fuzz a target but
// skip the execution of all other tests.
//
// Subtests can also be used to control parallelism. A parent test will only
// complete once all of its subtests complete. In this example, all tests are
// run in parallel with each other, and only with each other, regardless of
// other top-level tests that may be defined:
//
//	func TestGroupedParallel(t *testing.T) {
//	    for _, tc := range tests {
//	        t.Run(tc.Name, func(t *testing.T) {
//	            t.Parallel()
//	            ...
//	        })
//	    }
//	}
//
// Run does not return until parallel subtests have completed, providing a way
// to clean up after a group of parallel tests:
//
//	func TestTeardownParallel(t *testing.T) {
//	    // This Run will not return until the parallel tests finish.
//	    t.Run("group", func(t *testing.T) {
//	        t.Run("Test1", parallelTest1)
//	        t.Run("Test2", parallelTest2)
//	        t.Run("Test3", parallelTest3)
//	    })
//	    // <tear-down code>
//	}
//
// # Main
//
// It is sometimes necessary for a test or benchmark program to do extra setup or teardown
// before or after it executes. It is also sometimes necessary to control
// which code runs on the main thread. To support these and other cases,
// if a test file contains a function:
//
//	func TestMain(m *testing.M)
//
// then the generated test will call TestMain(m) instead of running the tests or benchmarks
// directly. TestMain runs in the main goroutine and can do whatever setup
// and teardown is necessary around a call to m.Run. m.Run will return an exit
// code that may be passed to [os.Exit]. If TestMain returns, the test wrapper
// will pass the result of m.Run to [os.Exit] itself.
//
// When TestMain is called, flag.Parse has not been run. If TestMain depends on
// command-line flags, including those of the testing package, it should call
// [flag.Parse] explicitly. Command line flags are always parsed by the time test
// or benchmark functions run.
//
// A simple implementation of TestMain is:
//
//	func TestMain(m *testing.M) {
//		// call flag.Parse() here if TestMain uses flags
//		m.Run()
//	}
//
// TestMain is a low-level primitive and should not be necessary for casual
// testing needs, where ordinary test functions suffice.
//
// [go help test]: https://pkg.go.dev/cmd/go#hdr-Test_packages
// [go help testflag]: https://pkg.go.dev/cmd/go#hdr-Testing_flags
package testing

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"internal/race"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"runtime/trace"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	_ "unsafe" // for linkname
)

var initRan bool

var (
	parallelStart atomic.Int64 // number of parallel tests started
	parallelStop  atomic.Int64 // number of parallel tests stopped
)

// Init registers testing flags. These flags are automatically registered by
// the "go test" command before running test functions, so Init is only needed
// when calling functions such as Benchmark without using "go test".
//
// Init is not safe to call concurrently. It has no effect if it was already called.
func Init() {
	if initRan {
		return
	}
	initRan = true
	// The short flag requests that tests run more quickly, but its functionality
	// is provided by test writers themselves. The testing package is just its
	// home. The all.bash installation script sets it to make installation more
	// efficient, but by default the flag is off so a plain "go test" will do a
	// full test of the package.
	short = flag.Bool("test.short", false, "run smaller test suite to save time")

	// The failfast flag requests that test execution stop after the first test failure.
	failFast = flag.Bool("test.failfast", false, "do not start new tests after the first test failure")

	// The directory in which to create profile files and the like. When run from
	// "go test", the binary always runs in the source directory for the package;
	// this flag lets "go test" tell the binary to write the files in the directory where
	// the "go test" command is run.
	outputDir = flag.String("test.outputdir", "", "write profiles to `dir`")
	artifacts = flag.Bool("test.artifacts", false, "store test artifacts in test.,outputdir")
	// Report as tests are run; default is silent for success.
	flag.Var(&chatty, "test.v", "verbose: print additional output")
	count = flag.Uint("test.count", 1, "run tests and benchmarks `n` times")
	coverProfile = flag.String("test.coverprofile", "", "write a coverage profile to `file`")
	gocoverdir = flag.String("test.gocoverdir", "", "write coverage intermediate files to this directory")
	matchList = flag.String("test.list", "", "list tests, examples, and benchmarks matching `regexp` then exit")
	match = flag.String("test.run", "", "run only tests and examples matching `regexp`")
	skip = flag.String("test.skip", "", "do not list or run tests matching `regexp`")
	memProfile = flag.String("test.memprofile", "", "write an allocation profile to `file`")
	memProfileRate = flag.Int("test.memprofilerate", 0, "set memory allocation profiling `rate` (see runtime.MemProfileRate)")
	cpuProfile = flag.String("test.cpuprofile", "", "write a cpu profile to `file`")
	blockProfile = flag.String("test.blockprofile", "", "write a goroutine blocking profile to `file`")
	blockProfileRate = flag.Int("test.blockprofilerate", 1, "set blocking profile `rate` (see runtime.SetBlockProfileRate)")
	mutexProfile = flag.String("test.mutexprofile", "", "write a mutex contention profile to the named file after execution")
	mutexProfileFraction = flag.Int("test.mutexprofilefraction", 1, "if >= 0, calls runtime.SetMutexProfileFraction()")
	panicOnExit0 = flag.Bool("test.paniconexit0", false, "panic on call to os.Exit(0)")
	traceFile = flag.String("test.trace", "", "write an execution trace to `file`")
	timeout = flag.Duration("test.timeout", 0, "panic test binary after duration `d` (default 0, timeout disabled)")
	cpuListStr = flag.String("test.cpu", "", "comma-separated `list` of cpu counts to run each test with")
	parallel = flag.Int("test.parallel", runtime.GOMAXPROCS(0), "run at most `n` tests in parallel")
	testlog = flag.String("test.testlogfile", "", "write test action log to `file` (for use only by cmd/go)")
	shuffle = flag.String("test.shuffle", "off", "randomize the execution order of tests and benchmarks")
	fullPath = flag.Bool("test.fullpath", false, "show full file names in error messages")

	initBenchmarkFlags()
	initFuzzFlags()
}

var (
	// Flags, registered during Init.
	short                *bool
	failFast             *bool
	outputDir            *string
	artifacts            *bool
	chatty               chattyFlag
	count                *uint
	coverProfile         *string
	gocoverdir           *string
	matchList            *string
	match                *string
	skip                 *string
	memProfile           *string
	memProfileRate       *int
	cpuProfile           *string
	blockProfile         *string
	blockProfileRate     *int
	mutexProfile         *string
	mutexProfileFraction *int
	panicOnExit0         *bool
	traceFile            *string
	timeout              *time.Duration
	cpuListStr           *string
	parallel             *int
	shuffle              *string
	testlog              *string
	fullPath             *bool

	haveExamples bool // are there examples?

	cpuList     []int
	testlogFile *os.File
	artifactDir string

	numFailed atomic.Uint32 // number of test failures

	running sync.Map // map[string]time.Time of running, unpaused tests
)

type chattyFlag struct {
	on   bool // -v is set in some form
	json bool // -v=test2json is set, to make output better for test2json
}

func (*chattyFlag) IsBoolFlag() bool { return true }

func (f *chattyFlag) Set(arg string) error {
	switch arg {
	default:
		return fmt.Errorf("invalid flag -test.v=%s", arg)
	case "true", "test2json":
		f.on = true
		f.json = arg == "test2json"
	case "false":
		f.on = false
		f.json = false
	}
	return nil
}

func (f *chattyFlag) String() string {
	if f.json {
		return "test2json"
	}
	if f.on {
		return "true"
	}
	return "false"
}

func (f *chattyFlag) Get() any {
	if f.json {
		return "test2json"
	}
	return f.on
}

const (
	markFraming  byte = 'V' &^ '@' // ^V: framing
	markErrBegin byte = 'O' &^ '@' // ^O: start of error
	markErrEnd   byte = 'N' &^ '@' // ^N: end of error
	markEscape   byte = '[' &^ '@' // ^[: escape
)

func (f *chattyFlag) prefix() string {
	if f.json {
		return string(markFraming)
	}
	return ""
}

type chattyPrinter struct {
	w          io.Writer
	lastNameMu sync.Mutex // guards lastName
	lastName   string     // last printed test name in chatty mode
	json       bool       // -v=json output mode
}

func newChattyPrinter(w io.Writer) *chattyPrinter {
	return &chattyPrinter{w: w, json: chatty.json}
}

// prefix is like chatty.prefix but using p.json instead of chatty.json.
// Using p.json allows tests to check the json behavior without modifying
// the global variable. For convenience, we allow p == nil and treat
// that as not in json mode (because it's not chatty at all).
func (p *chattyPrinter) prefix() string {
	if p != nil && p.json {
		return string(markFraming)
	}
	return ""
}

// Updatef prints a message about the status of the named test to w.
//
// The formatted message must include the test name itself.
func (p *chattyPrinter) Updatef(testName, format string, args ...any) {
	p.lastNameMu.Lock()
	defer p.lastNameMu.Unlock()

	// Since the message already implies an association with a specific new test,
	// we don't need to check what the old test name was or log an extra NAME line
	// for it. (We're updating it anyway, and the current message already includes
	// the test name.)
	p.lastName = testName
	fmt.Fprintf(p.w, p.prefix()+format, args...)
}

// Printf prints a message, generated by the named test, that does not
// necessarily mention that tests's name itself.
func (p *chattyPrinter) Printf(testName, format string, args ...any) {
	p.lastNameMu.Lock()
	defer p.lastNameMu.Unlock()

	if p.lastName == "" {
		p.lastName = testName
	} else if p.lastName != testName {
		fmt.Fprintf(p.w, "%s=== NAME  %s\n", p.prefix(), testName)
		p.lastName = testName
	}

	fmt.Fprintf(p.w, format, args...)
}

type stringWriter interface {
	io.Writer
	io.StringWriter
}

// escapeWriter is a [io.Writer] that escapes test framing markers.
type escapeWriter struct {
	w stringWriter
}

func (w escapeWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w escapeWriter) Write(p []byte) (int, error) {
	var n, m int
	var err error
	for len(p) > 0 {
		i := w.nextMark(p)
		if i < 0 {
			break
		}

		m, err = w.w.Write(p[:i])
		n += m
		if err != nil {
			break
		}

		m, err = w.w.Write([]byte{markEscape, p[i]})
		if err != nil {
			break
		}
		if m != 2 {
			return n, fmt.Errorf("short write")
		}
		n++
		p = p[i+1:]
	}
	m, err = w.w.Write(p)
	n += m
	return n, err
}

func (escapeWriter) nextMark(p []byte) int {
	for i, b := range p {
		switch b {
		case markFraming, markErrBegin, markErrEnd, markEscape:
			return i
		}
	}
	return -1
}

// The maximum number of stack frames to go through when skipping helper functions for
// the purpose of decorating log messages.
const maxStackLen = 50

// common holds the elements common between T and B and
// captures common methods such as Errorf.
type common struct {
	mu          sync.RWMutex         // guards this group of fields
	output      []byte               // Output generated by test or benchmark.
	w           io.Writer            // For flushToParent.
	o           *outputWriter        // Writes output.
	ran         bool                 // Test or benchmark (or one of its subtests) was executed.
	failed      bool                 // Test or benchmark has failed.
	skipped     bool                 // Test or benchmark has been skipped.
	done        bool                 // Test is finished and all subtests have completed.
	helperPCs   map[uintptr]struct{} // functions to be skipped when writing file/line info
	helperNames map[string]struct{}  // helperPCs converted to function names
	cleanups    []func()             // optional functions to be called at the end of the test
	cleanupName string               // Name of the cleanup function.
	cleanupPc   []uintptr            // The stack trace at the point where Cleanup was called.
	finished    bool                 // Test function has completed.
	inFuzzFn    bool                 // Whether the fuzz target, if this is one, is running.
	isSynctest  bool

	chatty         *chattyPrinter // A copy of chattyPrinter, if the chatty flag is set.
	bench          bool           // Whether the current test is a benchmark.
	hasSub         atomic.Bool    // whether there are sub-benchmarks.
	cleanupStarted atomic.Bool    // Registered cleanup callbacks have started to execute
	runner         string         // Function name of tRunner running the test.
	isParallel     bool           // Whether the test is parallel.

	parent     *common
	level      int       // Nesting depth of test or benchmark.
	creator    []uintptr // If level > 0, the stack trace at the point where the parent called t.Run.
	modulePath string
	importPath string
	name       string            // Name of test or benchmark.
	start      highPrecisionTime // Time test or benchmark started
	duration   time.Duration
	barrier    chan bool // To signal parallel subtests they may start. Nil when T.Parallel is not present (B) or not usable (when fuzzing).
	signal     chan bool // To signal a test is done.
	sub        []*T      // Queue of subtests to be run in parallel.

	lastRaceErrors  atomic.Int64 // Max value of race.Errors seen during the test or its subtests.
	raceErrorLogged atomic.Bool

	tempDirMu  sync.Mutex
	tempDir    string
	tempDirErr error
	tempDirSeq int32

	artifactDirOnce sync.Once
	artifactDir     string
	artifactDirErr  error

	ctx       context.Context
	cancelCtx context.CancelFunc
}

// Short reports whether the -test.short flag is set.
func Short() bool {
	if short == nil {
		panic("testing: Short called before Init")
	}
	// Catch code that calls this from TestMain without first calling flag.Parse.
	if !flag.Parsed() {
		panic("testing: Short called before Parse")
	}

	return *short
}

// testBinary is set by cmd/go to "1" if this is a binary built by "go test".
// The value is set to "1" by a -X option to cmd/link. We assume that
// because this is possible, the compiler will not optimize testBinary
// into a constant on the basis that it is an unexported package-scope
// variable that is never changed. If the compiler ever starts implementing
// such an optimization, we will need some technique to mark this variable
// as "changed by a cmd/link -X option".
var testBinary = "0"

// Testing reports whether the current code is being run in a test.
// This will report true in programs created by "go test",
// false in programs created by "go build".
func Testing() bool {
	return testBinary == "1"
}

// CoverMode reports what the test coverage mode is set to. The
// values are "set", "count", or "atomic". The return value will be
// empty if test coverage is not enabled.
func CoverMode() string {
	return cover.mode
}

// Verbose reports whether the -test.v flag is set.
func Verbose() bool {
	// Same as in Short.
	if !flag.Parsed() {
		panic("testing: Verbose called before Parse")
	}
	return chatty.on
}

func (c *common) checkFuzzFn(name string) {
	if c.inFuzzFn {
		panic(fmt.Sprintf("testing: f.%s was called inside the fuzz target, use t.%s instead", name, name))
	}
}

// frameSkip searches, starting after skip frames, for the first caller frame
// in a function not marked as a helper and returns that frame.
// The search stops if it finds a tRunner function that
// was the entry point into the test and the test is not a subtest.
// This function must be called with c.mu held.
func (c *common) frameSkip(skip int) runtime.Frame {
	// If the search continues into the parent test, we'll have to hold
	// its mu temporarily. If we then return, we need to unlock it.
	shouldUnlock := false
	defer func() {
		if shouldUnlock {
			c.mu.Unlock()
		}
	}()
	var pc [maxStackLen]uintptr
	// Skip two extra frames to account for this function
	// and runtime.Callers itself.
	n := runtime.Callers(skip+2, pc[:])
	if n == 0 {
		panic("testing: zero callers found")
	}
	frames := runtime.CallersFrames(pc[:n])
	var firstFrame, prevFrame, frame runtime.Frame
	skipRange := false
	for more := true; more; prevFrame = frame {
		frame, more = frames.Next()
		if skipRange {
			// Skip the iterator function when a helper
			// functions does a range over function.
			skipRange = false
			continue
		}
		if frame.Function == "runtime.gopanic" {
			continue
		}
		if frame.Function == c.cleanupName {
			frames = runtime.CallersFrames(c.cleanupPc)
			continue
		}
		if firstFrame.PC == 0 {
			firstFrame = frame
		}
		if frame.Function == c.runner {
			// We've gone up all the way to the tRunner calling
			// the test function (so the user must have
			// called tb.Helper from inside that test function).
			// If this is a top-level test, only skip up to the test function itself.
			// If we're in a subtest, continue searching in the parent test,
			// starting from the point of the call to Run which created this subtest.
			if c.level > 1 {
				frames = runtime.CallersFrames(c.creator)
				parent := c.parent
				// We're no longer looking at the current c after this point,
				// so we should unlock its mu, unless it's the original receiver,
				// in which case our caller doesn't expect us to do that.
				if shouldUnlock {
					c.mu.Unlock()
				}
				c = parent
				// Remember to unlock c.mu when we no longer need it, either
				// because we went up another nesting level, or because we
				// returned.
				shouldUnlock = true
				c.mu.Lock()
				continue
			}
			return prevFrame
		}
		// If more helper PCs have been added since we last did the conversion
		if c.helperNames == nil {
			c.helperNames = make(map[string]struct{})
			for pc := range c.helperPCs {
				c.helperNames[pcToName(pc)] = struct{}{}
			}
		}

		fnName := frame.Function
		// Ignore trailing -rangeN used for iterator functions.
		const rangeSuffix = "-range"
		if suffixIdx := strings.LastIndex(fnName, rangeSuffix); suffixIdx > 0 {
			ok := true
			for i := suffixIdx + len(rangeSuffix); i < len(fnName); i++ {
				if fnName[i] < '0' || fnName[i] > '9' {
					ok = false
					break
				}
			}
			if ok {
				fnName = fnName[:suffixIdx]
				skipRange = true
			}
		}

		if _, ok := c.helperNames[fnName]; !ok {
			// Found a frame that wasn't inside a helper function.
			return frame
		}
	}
	return firstFrame
}

// flushToParent writes c.output to the parent after first writing the header
// with the given format and arguments.
func (c *common) flushToParent(testName, format string, args ...any) {
	p := c.parent
	p.mu.Lock()
	defer p.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.output) > 0 {
		// Add the current c.output to the print,
		// and then arrange for the print to replace c.output.
		// (This displays the logged output after the --- FAIL line.)
		format += "%s"
		args = append(args[:len(args):len(args)], c.output)
		c.output = c.output[:0]
	}

	if c.chatty != nil && (p.w == c.chatty.w || c.chatty.json) {
		// We're flushing to the actual output, so track that this output is
		// associated with a specific test (and, specifically, that the next output
		// is *not* associated with that test).
		//
		// Moreover, if c.output is non-empty it is important that this write be
		// atomic with respect to the output of other tests, so that we don't end up
		// with confusing '=== NAME' lines in the middle of our '--- PASS' block.
		// Neither humans nor cmd/test2json can parse those easily.
		// (See https://go.dev/issue/40771.)
		//
		// If test2json is used, we never flush to parent tests,
		// so that the json stream shows subtests as they finish.
		// (See https://go.dev/issue/29811.)
		c.chatty.Updatef(testName, format, args...)
	} else {
		// We're flushing to the output buffer of the parent test, which will
		// itself follow a test-name header when it is finally flushed to stdout.
		fmt.Fprintf(p.w, c.chatty.prefix()+format, args...)
	}
}

type indenter struct {
	c *common
}

const indent = "    "

func (w indenter) Write(b []byte) (n int, err error) {
	n = len(b)
	for len(b) > 0 {
		end := bytes.IndexByte(b, '\n')
		if end == -1 {
			end = len(b)
		} else {
			end++
		}
		// An indent of 4 spaces will neatly align the dashes with the status
		// indicator of the parent.
		line := b[:end]
		if line[0] == markFraming {
			w.c.output = append(w.c.output, markFraming)
			line = line[1:]
		}
		w.c.output = append(w.c.output, indent...)
		w.c.output = append(w.c.output, line...)
		b = b[end:]
	}
	return
}

// fmtDuration returns a string representing d in the form "87.00s".
func fmtDuration(d time.Duration) string {
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// TB is the interface common to [T], [B], and [F].
type TB interface {
	ArtifactDir() string
	Attr(key, value string)
	Cleanup(func())
	Error(args ...any)
	Errorf(format string, args ...any)
	Fail()
	FailNow()
	Failed() bool
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Helper()
	Log(args ...any)
	Logf(format string, args ...any)
	Name() string
	Setenv(key, value string)
	Chdir(dir string)
	Skip(args ...any)
	SkipNow()
	Skipf(format string, args ...any)
	Skipped() bool
	TempDir() string
	Context() context.Context
	Output() io.Writer

	// A private method to prevent users implementing the
	// interface and so future additions to it will not
	// violate Go 1 compatibility.
	private()
}

var (
	_ TB = (*T)(nil)
	_ TB = (*B)(nil)
)

// T is a type passed to Test functions to manage test state and support formatted test logs.
//
// A test ends when its Test function returns or calls any of the methods
// [T.FailNow], [T.Fatal], [T.Fatalf], [T.SkipNow], [T.Skip], or [T.Skipf]. Those methods, as well as
// the [T.Parallel] method, must be called only from the goroutine running the
// Test function.
//
// The other reporting methods, such as the variations of [T.Log] and [T.Error],
// may be called simultaneously from multiple goroutines.
type T struct {
	common
	denyParallel bool
	tstate       *testState // For running tests and subtests.
}

func (c *common) private() {}

// Name returns the name of the running (sub-) test or benchmark.
//
// The name will include the name of the test along with the names of
// any nested sub-tests. If two sibling sub-tests have the same name,
// Name will append a suffix to guarantee the returned name is unique.
func (c *common) Name() string {
	return c.name
}

func (c *common) setRan() {
	if c.parent != nil {
		c.parent.setRan()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ran = true
}

// Fail marks the function as having failed but continues execution.
func (c *common) Fail() {
	if c.parent != nil {
		c.parent.Fail()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// c.done needs to be locked to synchronize checks to c.done in parent tests.
	if c.done {
		panic("Fail in goroutine after " + c.name + " has completed")
	}
	c.failed = true
}

// Failed reports whether the function has failed.
func (c *common) Failed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.done && int64(race.Errors()) > c.lastRaceErrors.Load() {
		c.mu.RUnlock()
		c.checkRaces()
		c.mu.RLock()
	}

	return c.failed
}

// FailNow marks the function as having failed and stops its execution
// by calling [runtime.Goexit] (which then runs all deferred calls in the
// current goroutine).
// Execution will continue at the next test or benchmark.
// FailNow must be called from the goroutine running the
// test or benchmark function, not from other goroutines
// created during the test. Calling FailNow does not stop
// those other goroutines.
func (c *common) FailNow() {
	c.checkFuzzFn("FailNow")
	c.Fail()

	// Calling runtime.Goexit will exit the goroutine, which
	// will run the deferred functions in this goroutine,
	// which will eventually run the deferred lines in tRunner,
	// which will signal to the test loop that this test is done.
	//
	// A previous version of this code said:
	//
	//	c.duration = ...
	//	c.signal <- c.self
	//	runtime.Goexit()
	//
	// This previous version duplicated code (those lines are in
	// tRunner no matter what), but worse the goroutine teardown
	// implicit in runtime.Goexit was not guaranteed to complete
	// before the test exited. If a test deferred an important cleanup
	// function (like removing temporary files), there was no guarantee
	// it would run on a test failure. Because we send on c.signal during
	// a top-of-stack deferred function now, we know that the send
	// only happens after any other stacked defers have completed.
	c.mu.Lock()
	c.finished = true
	c.mu.Unlock()
	runtime.Goexit()
}

// log generates the output. It is always at the same stack depth. log inserts
// indentation and the final newline if necessary. It prefixes the string
// with the file and line of the call site.
func (c *common) log(s string, isErr bool) {
	s = strings.TrimSuffix(s, "\n")

	// Second and subsequent lines are indented 4 spaces. This is in addition to
	// the indentation provided by outputWriter.
	s = strings.ReplaceAll(s, "\n", "\n"+indent)
	s += "\n"

	n := c.destination()
	if n == nil {
		// The test and all its parents are done. The log cannot be output.
		panic("Log in goroutine after " + c.name + " has completed: " + s)
	}

	// Prefix with the call site. It is located by skipping 3 functions:
	// callSite + log + public function
	s = n.callSite(3) + s

	// Output buffered logs.
	n.flushPartial()

	n.o.write([]byte(s), isErr)
}

// destination selects the test to which output should be appended. It returns the
// test if it is incomplete. Otherwise, it finds its closest incomplete parent.
func (c *common) destination() *common {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.done && !c.isSynctest {
		return c
	}
	for parent := c.parent; parent != nil; parent = parent.parent {
		parent.mu.Lock()
		defer parent.mu.Unlock()
		if !parent.done {
			return parent
		}
	}
	return nil
}

// callSite retrieves and formats the file and line of the call site.
func (c *common) callSite(skip int) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	frame := c.frameSkip(skip)
	file := frame.File
	line := frame.Line
	if file != "" {
		if *fullPath {
			// If relative path, truncate file name at last file name separator.
		} else {
			file = filepath.Base(file)
		}
	} else {
		file = "???"
	}
	if line == 0 {
		line = 1
	}

	return fmt.Sprintf("%s:%d: ", file, line)
}

// flushPartial checks the buffer for partial logs and outputs them.
func (c *common) flushPartial() {
	partial := func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return (c.o != nil) && (len(c.o.partial) > 0)
	}

	if partial() {
		c.o.Write([]byte("\n"))
	}
}

// Output returns a Writer that writes to the same test output stream as TB.Log.
// The output is indented like TB.Log lines, but Output does not
// add source locations or newlines. The output is internally line
// buffered, and a call to TB.Log or the end of the test will implicitly
// flush the buffer, followed by a newline. After a test function and all its
// parents return, neither Output nor the Write method may be called.
func (c *common) Output() io.Writer {
	c.checkFuzzFn("Output")
	n := c.destination()
	if n == nil {
		panic("Output called after " + c.name + " has completed")
	}
	return n.o
}

// setOutputWriter initializes an outputWriter and sets it as a common field.
func (c *common) setOutputWriter() {
	c.o = &outputWriter{c: c}
}

// outputWriter buffers, formats and writes log messages.
type outputWriter struct {
	c       *common
	partial []byte // incomplete ('\n'-free) suffix of last Write
}

// Write writes a log message to the test's output stream, properly formatted and
// indented. It may not be called after a test function and all its parents return.
func (o *outputWriter) Write(p []byte) (int, error) {
	return o.write(p, false)
}

func (o *outputWriter) write(p []byte, isErr bool) (int, error) {
	// o can be nil if this is called from a top-level *TB that is no longer active.
	// Just ignore the message in that case.
	if o == nil || o.c == nil {
		return 0, nil
	}
	if o.c.destination() == nil {
		panic("Write called after " + o.c.name + " has completed")
	}

	o.c.mu.Lock()
	defer o.c.mu.Unlock()

	// The last element is a partial line.
	lines := bytes.SplitAfter(p, []byte("\n"))
	last := len(lines) - 1 // Inv: 0 <= last
	for i, line := range lines[:last] {
		// Emit partial line from previous call.
		if i == 0 && len(o.partial) > 0 {
			line = slices.Concat(o.partial, line)
			o.partial = o.partial[:0]
		}
		o.writeLine(line, isErr && i == 0, isErr && i == last-1)
	}
	// Save partial line for next call.
	o.partial = append(o.partial, lines[last]...)

	return len(p), nil
}

// writeLine generates the output for a given line.
func (o *outputWriter) writeLine(b []byte, errBegin, errEnd bool) {
	if o.c.done || (o.c.chatty == nil) {
		o.c.output = append(o.c.output, indent...)
		o.c.output = append(o.c.output, b...)
		return
	}

	// Escape the framing marker.
	b = escapeMarkers(b)

	// If this is the start of an error, add ^O to the start of the output.
	var strErrBegin, strErrEnd string
	if errBegin && o.c.chatty.json {
		strErrBegin = string(markErrBegin)
	}

	// If this is the end of an error, add ^N to the end of the output. If the
	// last character of the output is \n, add ^N before the \n, otherwise
	// test2json will not handle it correctly.
	var c []byte
	if errEnd && o.c.chatty.json {
		i := len(b)
		if len(b) > 0 && b[i-1] == '\n' {
			b, c = b[:i-1], b[i-1:]
		}
		strErrEnd = string(markErrEnd)
	}

	if o.c.bench {
		// Benchmarks don't print === CONT, so we should skip the test
		// printer and just print straight to stdout.
		fmt.Printf("%s%s%s%s%s", strErrBegin, indent, b, strErrEnd, c)
	} else {
		o.c.chatty.Printf(o.c.name, "%s%s%s%s%s", strErrBegin, indent, b, strErrEnd, c)
	}
}

func escapeMarkers(b []byte) []byte {
	j := nextMark(b)
	if j < 0 {
		// Allocation-free fast path.
		return b
	}

	c := make([]byte, 0, len(b)+10)
	i := 0
	for i < len(b) && j >= i {
		if j > i {
			c = append(c, b[i:j]...)
		}
		c = append(c, markEscape, b[j])
		i = j + 1
		j = i + nextMark(b[i:])
	}
	if i < len(b) {
		c = append(c, b[i:]...)
	}
	return c
}

func nextMark(b []byte) int {
	for i, b := range b {
		switch b {
		case markFraming, markEscape, markErrBegin, markErrEnd:
			return i
		}
	}
	return -1
}

// Log formats its arguments using default formatting, analogous to [fmt.Println],
// and records the text in the error log. For tests, the text will be printed only if
// the test fails or the -test.v flag is set. For benchmarks, the text is always
// printed to avoid having performance depend on the value of the -test.v flag.
// It is an error to call Log after a test or benchmark returns.
func (c *common) Log(args ...any) {
	c.checkFuzzFn("Log")
	c.log(fmt.Sprintln(args...), false)
}

// Logf formats its arguments according to the format, analogous to [fmt.Printf], and
// records the text in the error log. A final newline is added if not provided. For
// tests, the text will be printed only if the test fails or the -test.v flag is
// set. For benchmarks, the text is always printed to avoid having performance
// depend on the value of the -test.v flag.
// It is an error to call Logf after a test or benchmark returns.
func (c *common) Logf(format string, args ...any) {
	c.checkFuzzFn("Logf")
	c.log(fmt.Sprintf(format, args...), false)
}

// Error is equivalent to Log followed by Fail.
func (c *common) Error(args ...any) {
	c.checkFuzzFn("Error")
	c.log(fmt.Sprintln(args...), true)
	c.Fail()
}

// Errorf is equivalent to Logf followed by Fail.
func (c *common) Errorf(format string, args ...any) {
	c.checkFuzzFn("Errorf")
	c.log(fmt.Sprintf(format, args...), true)
	c.Fail()
}

// Fatal is equivalent to Log followed by FailNow.
func (c *common) Fatal(args ...any) {
	c.checkFuzzFn("Fatal")
	c.log(fmt.Sprintln(args...), true)
	c.FailNow()
}

// Fatalf is equivalent to Logf followed by FailNow.
func (c *common) Fatalf(format string, args ...any) {
	c.checkFuzzFn("Fatalf")
	c.log(fmt.Sprintf(format, args...), true)
	c.FailNow()
}

// Skip is equivalent to Log followed by SkipNow.
func (c *common) Skip(args ...any) {
	c.checkFuzzFn("Skip")
	c.log(fmt.Sprintln(args...), false)
	c.SkipNow()
}

// Skipf is equivalent to Logf followed by SkipNow.
func (c *common) Skipf(format string, args ...any) {
	c.checkFuzzFn("Skipf")
	c.log(fmt.Sprintf(format, args...), false)
	c.SkipNow()
}

// SkipNow marks the test as having been skipped and stops its execution
// by calling [runtime.Goexit].
// If a test fails (see Error, Errorf, Fail) and is then skipped,
// it is still considered to have failed.
// Execution will continue at the next test or benchmark. See also FailNow.
// SkipNow must be called from the goroutine running the test, not from
// other goroutines created during the test. Calling SkipNow does not stop
// those other goroutines.
func (c *common) SkipNow() {
	c.checkFuzzFn("SkipNow")
	c.mu.Lock()
	c.skipped = true
	c.finished = true
	c.mu.Unlock()
	runtime.Goexit()
}

// Skipped reports whether the test was skipped.
func (c *common) Skipped() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.skipped
}

// Helper marks the calling function as a test helper function.
// When printing file and line information, that function will be skipped.
// Helper may be called simultaneously from multiple goroutines.
func (c *common) Helper() {
	if c.isSynctest {
		c = c.parent
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.helperPCs == nil {
		c.helperPCs = make(map[uintptr]struct{})
	}
	// repeating code from callerName here to save walking a stack frame
	var pc [1]uintptr
	n := runtime.Callers(2, pc[:]) // skip runtime.Callers + Helper
	if n == 0 {
		panic("testing: zero callers found")
	}
	if _, found := c.helperPCs[pc[0]]; !found {
		c.helperPCs[pc[0]] = struct{}{}
		c.helperNames = nil // map will be recreated next time it is needed
	}
}

// Cleanup registers a function to be called when the test (or subtest) and all its
// subtests complete. Cleanup functions will be called in last added,
// first called order.
func (c *common) Cleanup(f func()) {
	c.checkFuzzFn("Cleanup")
	var pc [maxStackLen]uintptr
	// Skip two extra frames to account for this function and runtime.Callers itself.
	n := runtime.Callers(2, pc[:])
	cleanupPc := pc[:n]

	fn := func() {
		defer func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.cleanupName = ""
			c.cleanupPc = nil
		}()

		name := callerName(0)
		c.mu.Lock()
		c.cleanupName = name
		c.cleanupPc = cleanupPc
		c.mu.Unlock()

		f()
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanups = append(c.cleanups, fn)
}

// ArtifactDir returns a directory in which the test should store output files.
// When the -artifacts flag is provided, this directory is located
// under the output directory. Otherwise, ArtifactDir returns a temporary directory
// that is removed after the test completes.
//
// Each test or subtest within each test package has a unique artifact directory.
// Repeated calls to ArtifactDir in the same test or subtest return the same directory.
// Subtest outputs are not located under the parent test's output directory.
func (c *common) ArtifactDir() string {
	c.checkFuzzFn("ArtifactDir")
	c.artifactDirOnce.Do(func() {
		c.artifactDir, c.artifactDirErr = c.makeArtifactDir()
	})
	if c.artifactDirErr != nil {
		c.Fatalf("ArtifactDir: %v", c.artifactDirErr)
	}
	return c.artifactDir
}

func hashString(s string) (h uint64) {
	// FNV, used here to avoid a dependency on maphash.
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return
}

// makeArtifactDir creates the artifact directory for a test.
// The artifact directory is:
//
//	<output dir>/_artifacts/<test package>/<test name>/<random>
//
// The test package is the package import path with the module name prefix removed.
// The test name is truncated if too long.
// Special characters are removed from the path.
func (c *common) makeArtifactDir() (string, error) {
	if !*artifacts {
		return c.makeTempDir()
	}

	artifactBase := filepath.Join(artifactDir, c.relativeArtifactBase())
	if err := os.MkdirAll(artifactBase, 0o777); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(artifactBase, "")
	if err != nil {
		return "", err
	}
	if c.chatty != nil {
		c.chatty.Updatef(c.name, "=== ARTIFACTS %s %v\n", c.name, dir)
	}
	return dir, nil
}

func (c *common) relativeArtifactBase() string {
	// If the test name is longer than maxNameSize, truncate it and replace the last
	// hashSize bytes with a hash of the full name.
	const maxNameSize = 64
	name := strings.ReplaceAll(c.name, "/", "__")
	if len(name) > maxNameSize {
		h := fmt.Sprintf("%0x", hashString(name))
		name = name[:maxNameSize-len(h)] + h
	}

	// Remove the module path prefix from the import path.
	// If this is the root package, pkg will be empty.
	pkg := strings.TrimPrefix(c.importPath, c.modulePath)
	// Remove the leading slash.
	pkg = strings.TrimPrefix(pkg, "/")

	base := name
	if pkg != "" {
		// Join with /, not filepath.Join: the import path is /-separated,
		// and we don't want removeSymbolsExcept to strip \ separators on Windows.
		base = pkg + "/" + name
	}
	base = removeSymbolsExcept(base, "!#$%&()+,-.=@^_{}~ /")
	base, err := filepath.Localize(base)
	if err != nil {
		// This name can't be safely converted into a local filepath.
		// Drop it and just use _artifacts/<random>.
		base = ""
	}

	return base
}

func removeSymbolsExcept(s, allowed string) string {
	mapper := func(r rune) rune {
		if unicode.IsLetter(r) ||
			unicode.IsNumber(r) ||
			strings.ContainsRune(allowed, r) {
			return r
		}
		return -1 // disallowed symbol
	}
	return strings.Map(mapper, s)
}

// TempDir returns a temporary directory for the test to use.
// The directory is automatically removed when the test and
// all its subtests complete.
// Each subsequent call to TempDir returns a unique directory;
// if the directory creation fails, TempDir terminates the test by calling Fatal.
// If the environment variable GOTMPDIR is set, the temporary directory will
// be created somewhere beneath it.
func (c *common) TempDir() string {
	c.checkFuzzFn("TempDir")
	dir, err := c.makeTempDir()
	if err != nil {
		c.Fatalf("TempDir: %v", err)
	}
	return dir
}

func (c *common) makeTempDir() (string, error) {
	// Use a single parent directory for all the temporary directories
	// created by a test, each numbered sequentially.
	c.tempDirMu.Lock()
	var nonExistent bool
	if c.tempDir == "" { // Usually the case with js/wasm
		nonExistent = true
	} else {
		_, err := os.Stat(c.tempDir)
		nonExistent = os.IsNotExist(err)
		if err != nil && !nonExistent {
			return "", err
		}
	}

	if nonExistent {
		c.Helper()

		pattern := c.Name()
		// Limit length of file names on disk.
		// Invalid runes from slicing are dropped by strings.Map below.
		pattern = pattern[:min(len(pattern), 64)]

		// Drop unusual characters (such as path separators or
		// characters interacting with globs) from the directory name to
		// avoid surprising os.MkdirTemp behavior.
		const allowed = "!#$%&()+,-.=@^_{}~ "
		pattern = removeSymbolsExcept(pattern, allowed)

		c.tempDir, c.tempDirErr = os.MkdirTemp(os.Getenv("GOTMPDIR"), pattern)
		if c.tempDirErr == nil {
			c.Cleanup(func() {
				if err := removeAll(c.tempDir); err != nil {
					c.Errorf("TempDir RemoveAll cleanup: %v", err)
				}
			})
		}
	}

	if c.tempDirErr == nil {
		c.tempDirSeq++
	}
	seq := c.tempDirSeq
	c.tempDirMu.Unlock()

	if c.tempDirErr != nil {
		return "", c.tempDirErr
	}

	dir := fmt.Sprintf("%s%c%03d", c.tempDir, os.PathSeparator, seq)
	if err := os.Mkdir(dir, 0o777); err != nil {
		return "", err
	}
	return dir, nil
}

// removeAll is like os.RemoveAll, but retries Windows "Access is denied."
// errors up to an arbitrary timeout.
//
// Those errors have been known to occur spuriously on at least the
// windows-amd64-2012 builder (https://go.dev/issue/50051), and can only occur
// legitimately if the test leaves behind a temp file that either is still open
// or the test otherwise lacks permission to delete. In the case of legitimate
// failures, a failing test may take a bit longer to fail, but once the test is
// fixed the extra latency will go away.
func removeAll(path string) error {
	const arbitraryTimeout = 2 * time.Second
	var (
		start     time.Time
		nextSleep = 1 * time.Millisecond
	)
	for {
		err := os.RemoveAll(path)
		if !isWindowsRetryable(err) {
			return err
		}
		if start.IsZero() {
			start = time.Now()
		} else if d := time.Since(start) + nextSleep; d >= arbitraryTimeout {
			return err
		}
		time.Sleep(nextSleep)
		nextSleep += time.Duration(rand.Int63n(int64(nextSleep)))
	}
}

// Setenv calls [os.Setenv] and uses Cleanup to
// restore the environment variable to its original value
// after the test.
//
// Because Setenv affects the whole process, it cannot be used
// in parallel tests or tests with parallel ancestors.
func (c *common) Setenv(key, value string) {
	c.checkFuzzFn("Setenv")
	prevValue, ok := os.LookupEnv(key)

	if err := os.Setenv(key, value); err != nil {
		c.Fatalf("cannot set environment variable: %v", err)
	}

	if ok {
		c.Cleanup(func() {
			os.Setenv(key, prevValue)
		})
	} else {
		c.Cleanup(func() {
			os.Unsetenv(key)
		})
	}
}

// Chdir calls [os.Chdir] and uses Cleanup to restore the current
// working directory to its original value after the test. On Unix, it
// also sets PWD environment variable for the duration of the test.
//
// Because Chdir affects the whole process, it cannot be used
// in parallel tests or tests with parallel ancestors.
func (c *common) Chdir(dir string) {
	c.checkFuzzFn("Chdir")
	oldwd, err := os.Open(".")
	if err != nil {
		c.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		c.Fatal(err)
	}
	// On POSIX platforms, PWD represents “an absolute pathname of the
	// current working directory.” Since we are changing the working
	// directory, we should also set or update PWD to reflect that.
	switch runtime.GOOS {
	case "windows", "plan9":
		// Windows and Plan 9 do not use the PWD variable.
	default:
		if !filepath.IsAbs(dir) {
			dir, err = os.Getwd()
			if err != nil {
				c.Fatal(err)
			}
		}
		c.Setenv("PWD", dir)
	}
	c.Cleanup(func() {
		err := oldwd.Chdir()
		oldwd.Close()
		if err != nil {
			// It's not safe to continue with tests if we can't
			// get back to the original working directory. Since
			// we are holding a dirfd, this is highly unlikely.
			panic("testing.Chdir: " + err.Error())
		}
	})
}

// Context returns a context that is canceled just before
// Cleanup-registered functions are called.
//
// Cleanup functions can wait for any resources
// that shut down on [context.Context.Done] before the test or benchmark completes.
func (c *common) Context() context.Context {
	c.checkFuzzFn("Context")
	return c.ctx
}

// Attr emits a test attribute associated with this test.
//
// The key must not contain whitespace.
// The value must not contain newlines or carriage returns.
//
// The meaning of different attribute keys is left up to
// continuous integration systems and test frameworks.
//
// Test attributes are emitted immediately in the test log,
// but they are intended to be treated as unordered.
func (c *common) Attr(key, value string) {
	if strings.ContainsFunc(key, unicode.IsSpace) {
		c.Errorf("disallowed whitespace in attribute key %q", key)
		return
	}
	if strings.ContainsAny(value, "\r\n") {
		c.Errorf("disallowed newline in attribute value %q", value)
		return
	}
	if c.chatty == nil {
		return
	}
	c.chatty.Updatef(c.name, "=== ATTR  %s %v %v\n", c.name, key, value)
}

// panicHandling controls the panic handling used by runCleanup.
type panicHandling int

const (
	normalPanic panicHandling = iota
	recoverAndReturnPanic
)

// runCleanup is called at the end of the test.
// If ph is recoverAndReturnPanic, it will catch panics, and return the
// recovered value if any.
func (c *common) runCleanup(ph panicHandling) (panicVal any) {
	c.cleanupStarted.Store(true)
	defer c.cleanupStarted.Store(false)

	if ph == recoverAndReturnPanic {
		defer func() {
			panicVal = recover()
		}()
	}

	// Make sure that if a cleanup function panics,
	// we still run the remaining cleanup functions.
	defer func() {
		c.mu.Lock()
		recur := len(c.cleanups) > 0
		c.mu.Unlock()
		if recur {
			c.runCleanup(normalPanic)
		}
	}()

	if c.cancelCtx != nil {
		c.cancelCtx()
	}

	for {
		var cleanup func()
		c.mu.Lock()
		if len(c.cleanups) > 0 {
			last := len(c.cleanups) - 1
			cleanup = c.cleanups[last]
			c.cleanups = c.cleanups[:last]
		}
		c.mu.Unlock()
		if cleanup == nil {
			return nil
		}
		cleanup()
	}
}

// resetRaces updates c.parent's count of data race errors (or the global count,
// if c has no parent), and updates c.lastRaceErrors to match.
//
// Any races that occurred prior to this call to resetRaces will
// not be attributed to c.
func (c *common) resetRaces() {
	if c.parent == nil {
		c.lastRaceErrors.Store(int64(race.Errors()))
	} else {
		c.lastRaceErrors.Store(c.parent.checkRaces())
	}
}

// checkRaces checks whether the global count of data race errors has increased
// since c's count was last reset.
//
// If so, it marks c as having failed due to those races (logging an error for
// the first such race), and updates the race counts for the parents of c so
// that if they are currently suspended (such as in a call to T.Run) they will
// not log separate errors for the race(s).
//
// Note that multiple tests may be marked as failed due to the same race if they
// are executing in parallel.
func (c *common) checkRaces() (raceErrors int64) {
	raceErrors = int64(race.Errors())
	for {
		last := c.lastRaceErrors.Load()
		if raceErrors <= last {
			// All races have already been reported.
			return raceErrors
		}
		if c.lastRaceErrors.CompareAndSwap(last, raceErrors) {
			break
		}
	}

	if c.raceErrorLogged.CompareAndSwap(false, true) {
		// This is the first race we've encountered for this test.
		// Mark the test as failed, and log the reason why only once.
		// (Note that the race detector itself will still write a goroutine
		// dump for any further races it detects.)
		c.Errorf("race detected during execution of test")
	}

	// Update the parent(s) of this test so that they don't re-report the race.
	parent := c.parent
	for parent != nil {
		for {
			last := parent.lastRaceErrors.Load()
			if raceErrors <= last {
				// This race was already reported by another (likely parallel) subtest.
				return raceErrors
			}
			if parent.lastRaceErrors.CompareAndSwap(last, raceErrors) {
				break
			}
		}
		parent = parent.parent
	}

	return raceErrors
}

// callerName gives the function name (qualified with a package path)
// for the caller after skip frames (where 0 means the current function).
func callerName(skip int) string {
	var pc [1]uintptr
	n := runtime.Callers(skip+2, pc[:]) // skip + runtime.Callers + callerName
	if n == 0 {
		panic("testing: zero callers found")
	}
	return pcToName(pc[0])
}

func pcToName(pc uintptr) string {
	pcs := []uintptr{pc}
	frames := runtime.CallersFrames(pcs)
	frame, _ := frames.Next()
	return frame.Function
}

const parallelConflict = `testing: test using t.Setenv, t.Chdir, or cryptotest.SetGlobalRandom can not use t.Parallel`

// Parallel signals that this test is to be run in parallel with (and only with)
// other parallel tests, and pauses until all non-parallel tests have finished.
//
// When a test is run multiple times due to use of -test.count or -test.cpu,
// multiple instances of a single test never run in parallel with each other.
func (t *T) Parallel() {
	if t.isParallel {
		panic("testing: t.Parallel called multiple times")
	}
	if t.isSynctest {
		panic("testing: t.Parallel called inside synctest bubble")
	}
	if t.denyParallel {
		panic(parallelConflict)
	}
	if t.parent.barrier == nil {
		// T.Parallel has no effect when fuzzing.
		// Multiple processes may run in parallel, but only one input can run at a
		// time per process so we can attribute crashes to specific inputs.
		return
	}

	t.isParallel = true

	// We don't want to include the time we spend waiting for serial tests
	// in the test duration. Record the elapsed time thus far and reset the
	// timer afterwards.
	t.duration += highPrecisionTimeSince(t.start)

	// Add to the list of tests to be released by the parent.
	t.parent.sub = append(t.parent.sub, t)

	// Report any races during execution of this test up to this point.
	//
	// We will assume that any races that occur between here and the point where
	// we unblock are not caused by this subtest. That assumption usually holds,
	// although it can be wrong if the test spawns a goroutine that races in the
	// background while the rest of the test is blocked on the call to Parallel.
	// If that happens, we will misattribute the background race to some other
	// test, or to no test at all — but that false-negative is so unlikely that it
	// is not worth adding race-report noise for the common case where the test is
	// completely suspended during the call to Parallel.
	t.checkRaces()

	if t.chatty != nil {
		t.chatty.Updatef(t.name, "=== PAUSE %s\n", t.name)
	}
	running.Delete(t.name)

	t.signal <- true   // Release calling test.
	<-t.parent.barrier // Wait for the parent test to complete.
	t.tstate.waitParallel()
	parallelStart.Add(1)

	if t.chatty != nil {
		t.chatty.Updatef(t.name, "=== CONT  %s\n", t.name)
	}
	running.Store(t.name, highPrecisionTimeNow())
	t.start = highPrecisionTimeNow()

	// Reset the local race counter to ignore any races that happened while this
	// goroutine was blocked, such as in the parent test or in other parallel
	// subtests.
	//
	// (Note that we don't call parent.checkRaces here:
	// if other parallel subtests have already introduced races, we want to
	// let them report those races instead of attributing them to the parent.)
	t.lastRaceErrors.Store(int64(race.Errors()))
}

// checkParallel is called by [testing/cryptotest.SetGlobalRandom].
//
//go:linkname checkParallel testing.checkParallel
func checkParallel(t *T) {
	t.checkParallel()
}

func (t *T) checkParallel() {
	// Non-parallel subtests that have parallel ancestors may still
	// run in parallel with other tests: they are only non-parallel
	// with respect to the other subtests of the same parent.
	// Since calls like SetEnv or Chdir affects the whole process, we need
	// to deny those if the current test or any parent is parallel.
	for c := &t.common; c != nil; c = c.parent {
		if c.isParallel {
			panic(parallelConflict)
		}
	}

	t.denyParallel = true
}

// Setenv calls os.Setenv(key, value) and uses Cleanup to
// restore the environment variable to its original value
// after the test.
//
// Because Setenv affects the whole process, it cannot be used
// in parallel tests or tests with parallel ancestors.
func (t *T) Setenv(key, value string) {
	t.checkParallel()
	t.common.Setenv(key, value)
}

// Chdir calls [os.Chdir] and uses Cleanup to restore the current
// working directory to its original value after the test. On Unix, it
// also sets PWD environment variable for the duration of the test.
//
// Because Chdir affects the whole process, it cannot be used
// in parallel tests or tests with parallel ancestors.
func (t *T) Chdir(dir string) {
	t.checkParallel()
	t.common.Chdir(dir)
}

// InternalTest is an internal type but exported because it is cross-package;
// it is part of the implementation of the "go test" command.
type InternalTest struct {
	Name string
	F    func(*T)
}

var errNilPanicOrGoexit = errors.New("test executed panic(nil) or runtime.Goexit")

func tRunner(t *T, fn func(t *T)) {
	t.runner = callerName(0)

	// When this goroutine is done, either because fn(t)
	// returned normally or because a test failure triggered
	// a call to runtime.Goexit, record the duration and send
	// a signal saying that the test is done.
	defer func() {
		t.checkRaces()

		// TODO(#61034): This is the wrong place for this check.
		if t.Failed() {
			numFailed.Add(1)
		}

		// Check if the test panicked or Goexited inappropriately.
		//
		// If this happens in a normal test, print output but continue panicking.
		// tRunner is called in its own goroutine, so this terminates the process.
		//
		// If this happens while fuzzing, recover from the panic and treat it like a
		// normal failure. It's important that the process keeps running in order to
		// find short inputs that cause panics.
		err := recover()
		signal := true

		t.mu.RLock()
		finished := t.finished
		t.mu.RUnlock()
		if !finished && err == nil {
			err = errNilPanicOrGoexit
			for p := t.parent; p != nil; p = p.parent {
				p.mu.RLock()
				finished = p.finished
				p.mu.RUnlock()
				if finished {
					if !t.isParallel {
						t.Errorf("%v: subtest may have called FailNow on a parent test", err)
						err = nil
					}
					signal = false
					break
				}
			}
		}

		if err != nil && t.tstate.isFuzzing {
			prefix := "panic: "
			if err == errNilPanicOrGoexit {
				prefix = ""
			}
			t.Errorf("%s%s\n%s\n", prefix, err, string(debug.Stack()))
			t.mu.Lock()
			t.finished = true
			t.mu.Unlock()
			err = nil
		}

		// Use a deferred call to ensure that we report that the test is
		// complete even if a cleanup function calls t.FailNow. See issue 41355.
		didPanic := false
		defer func() {
			// Only report that the test is complete if it doesn't panic,
			// as otherwise the test binary can exit before the panic is
			// reported to the user. See issue 41479.
			if didPanic {
				return
			}
			if err != nil {
				panic(err)
			}
			running.Delete(t.name)
			if t.isParallel {
				parallelStop.Add(1)
			}
			t.signal <- signal
		}()

		doPanic := func(err any) {
			t.Fail()
			if r := t.runCleanup(recoverAndReturnPanic); r != nil {
				t.Logf("cleanup panicked with %v", r)
			}
			// Flush the output log up to the root before dying.
			// Skip this if this *T is a synctest bubble, because we're not a subtest.
			for root := &t.common; !root.isSynctest && root.parent != nil; root = root.parent {
				root.mu.Lock()
				root.duration += highPrecisionTimeSince(root.start)
				d := root.duration
				root.mu.Unlock()
				// Output buffered logs.
				root.flushPartial()
				root.flushToParent(root.name, "--- FAIL: %s (%s)\n", root.name, fmtDuration(d))
				if r := root.parent.runCleanup(recoverAndReturnPanic); r != nil {
					fmt.Fprintf(root.parent.w, "cleanup panicked with %v", r)
				}
			}
			didPanic = true
			panic(err)
		}
		if err != nil {
			doPanic(err)
		}

		t.duration += highPrecisionTimeSince(t.start)

		if len(t.sub) > 0 {
			// Run parallel subtests.

			// Decrease the running count for this test and mark it as no longer running.
			t.tstate.release()
			running.Delete(t.name)

			// Release the parallel subtests.
			close(t.barrier)
			// Wait for subtests to complete.
			for _, sub := range t.sub {
				<-sub.signal
			}

			// Run any cleanup callbacks, marking the test as running
			// in case the cleanup hangs.
			cleanupStart := highPrecisionTimeNow()
			running.Store(t.name, cleanupStart)
			err := t.runCleanup(recoverAndReturnPanic)
			t.duration += highPrecisionTimeSince(cleanupStart)
			if err != nil {
				doPanic(err)
			}
			t.checkRaces()
			if !t.isParallel {
				// Reacquire the count for sequential tests. See comment in Run.
				t.tstate.waitParallel()
			}
		} else if t.isParallel {
			// Only release the count for this test if it was run as a parallel
			// test. See comment in Run method.
			t.tstate.release()
		}
		// Output buffered logs.
		for root := &t.common; root.parent != nil; root = root.parent {
			root.flushPartial()
		}
		t.report() // Report after all subtests have finished.

		// Do not lock t.done to allow race detector to detect race in case
		// the user does not appropriately synchronize a goroutine.
		t.done = true
		if t.parent != nil && !t.hasSub.Load() {
			t.setRan()
		}
	}()
	defer func() {
		if len(t.sub) == 0 {
			t.runCleanup(normalPanic)
		}
	}()

	t.start = highPrecisionTimeNow()
	t.resetRaces()
	fn(t)

	// code beyond here will not be executed when FailNow is invoked
	t.mu.Lock()
	t.finished = true
	t.mu.Unlock()
}

// Run runs f as a subtest of t called name. It runs f in a separate goroutine
// and blocks until f returns or calls t.Parallel to become a parallel test.
// Run reports whether f succeeded (or at least did not fail before calling t.Parallel).
//
// Run may be called simultaneously from multiple goroutines, but all such calls
// must return before the outer test function for t returns.
func (t *T) Run(name string, f func(t *T)) bool {
	if t.isSynctest {
		panic("testing: t.Run called inside synctest bubble")
	}
	if t.cleanupStarted.Load() {
		panic("testing: t.Run called during t.Cleanup")
	}

	t.hasSub.Store(true)
	testName, ok, _ := t.tstate.match.fullName(&t.common, name)
	if !ok || shouldFailFast() {
		return true
	}
	// Record the stack trace at the point of this call so that if the subtest
	// function - which runs in a separate stack - is marked as a helper, we can
	// continue walking the stack into the parent test.
	var pc [maxStackLen]uintptr
	n := runtime.Callers(2, pc[:])

	// There's no reason to inherit this context from parent. The user's code can't observe
	// the difference between the background context and the one from the parent test.
	ctx, cancelCtx := context.WithCancel(context.Background())
	t = &T{
		common: common{
			barrier:    make(chan bool),
			signal:     make(chan bool, 1),
			name:       testName,
			modulePath: t.modulePath,
			importPath: t.importPath,
			parent:     &t.common,
			level:      t.level + 1,
			creator:    pc[:n],
			chatty:     t.chatty,
			ctx:        ctx,
			cancelCtx:  cancelCtx,
		},
		tstate: t.tstate,
	}
	t.w = indenter{&t.common}
	t.setOutputWriter()

	if t.chatty != nil {
		t.chatty.Updatef(t.name, "=== RUN   %s\n", t.name)
	}
	running.Store(t.name, highPrecisionTimeNow())

	// Instead of reducing the running count of this test before calling the
	// tRunner and increasing it afterwards, we rely on tRunner keeping the
	// count correct. This ensures that a sequence of sequential tests runs
	// without being preempted, even when their parent is a parallel test. This
	// may especially reduce surprises if *parallel == 1.
	go tRunner(t, f)

	// The parent goroutine will block until the subtest either finishes or calls
	// Parallel, but in general we don't know whether the parent goroutine is the
	// top-level test function or some other goroutine it has spawned.
	// To avoid confusing false-negatives, we leave the parent in the running map
	// even though in the typical case it is blocked.

	if !<-t.signal {
		// At this point, it is likely that FailNow was called on one of the
		// parent tests by one of the subtests. Continue aborting up the chain.
		runtime.Goexit()
	}

	if t.chatty != nil && t.chatty.json {
		t.chatty.Updatef(t.parent.name, "=== NAME  %s\n", t.parent.name)
	}
	return !t.failed
}

// testingSynctestTest runs f within a synctest bubble.
// It is called by synctest.Test, from within an already-created bubble.
//
//go:linkname testingSynctestTest testing/synctest.testingSynctestTest
func testingSynctestTest(t *T, f func(*T)) (ok bool) {
	if t.cleanupStarted.Load() {
		panic("testing: synctest.Run called during t.Cleanup")
	}

	var pc [maxStackLen]uintptr
	n := runtime.Callers(2, pc[:])

	ctx, cancelCtx := context.WithCancel(context.Background())
	t2 := &T{
		common: common{
			barrier:    make(chan bool),
			signal:     make(chan bool, 1),
			name:       t.name,
			parent:     &t.common,
			level:      t.level + 1,
			creator:    pc[:n],
			chatty:     t.chatty,
			ctx:        ctx,
			cancelCtx:  cancelCtx,
			isSynctest: true,
		},
		tstate: t.tstate,
	}

	go tRunner(t2, f)
	if !<-t2.signal {
		// At this point, it is likely that FailNow was called on one of the
		// parent tests by one of the subtests. Continue aborting up the chain.
		runtime.Goexit()
	}
	return !t2.failed
}

// Deadline reports the time at which the test binary will have
// exceeded the timeout specified by the -timeout flag.
//
// The ok result is false if the -timeout flag indicates “no timeout” (0).
func (t *T) Deadline() (deadline time.Time, ok bool) {
	if t.isSynctest {
		// There's no point in returning a real-clock deadline to
		// a test using a fake clock. We could return "no timeout",
		// but panicking makes it easier for users to catch the error.
		panic("testing: t.Deadline called inside synctest bubble")
	}
	deadline = t.tstate.deadline
	return deadline, !deadline.IsZero()
}

// testState holds all fields that are common to all tests. This includes
// synchronization primitives to run at most *parallel tests.
type testState struct {
	match    *matcher
	deadline time.Time

	// isFuzzing is true in the state used when generating random inputs
	// for fuzz targets. isFuzzing is false when running normal tests and
	// when running fuzz tests as unit tests (without -fuzz or when -fuzz
	// does not match).
	isFuzzing bool

	mu sync.Mutex

	// Channel used to signal tests that are ready to be run in parallel.
	startParallel chan bool

	// running is the number of tests currently running in parallel.
	// This does not include tests that are waiting for subtests to complete.
	running int

	// numWaiting is the number tests waiting to be run in parallel.
	numWaiting int

	// maxParallel is a copy of the parallel flag.
	maxParallel int
}

func newTestState(maxParallel int, m *matcher) *testState {
	return &testState{
		match:         m,
		startParallel: make(chan bool),
		maxParallel:   maxParallel,
		running:       1, // Set the count to 1 for the main (sequential) test.
	}
}

func (s *testState) waitParallel() {
	s.mu.Lock()
	if s.running < s.maxParallel {
		s.running++
		s.mu.Unlock()
		return
	}
	s.numWaiting++
	s.mu.Unlock()
	<-s.startParallel
}

func (s *testState) release() {
	s.mu.Lock()
	if s.numWaiting == 0 {
		s.running--
		s.mu.Unlock()
		return
	}
	s.numWaiting--
	s.mu.Unlock()
	s.startParallel <- true // Pick a waiting test to be run.
}

// No one should be using func Main anymore.
// See the doc comment on func Main and use MainStart instead.
var errMain = errors.New("testing: unexpected use of func Main")

type matchStringOnly func(pat, str string) (bool, error)

func (f matchStringOnly) MatchString(pat, str string) (bool, error)   { return f(pat, str) }
func (f matchStringOnly) StartCPUProfile(w io.Writer) error           { return errMain }
func (f matchStringOnly) StopCPUProfile()                             {}
func (f matchStringOnly) WriteProfileTo(string, io.Writer, int) error { return errMain }
func (f matchStringOnly) ModulePath() string                          { return "" }
func (f matchStringOnly) ImportPath() string                          { return "" }
func (f matchStringOnly) StartTestLog(io.Writer)                      {}
func (f matchStringOnly) StopTestLog() error                          { return errMain }
func (f matchStringOnly) SetPanicOnExit0(bool)                        {}
func (f matchStringOnly) CoordinateFuzzing(time.Duration, int64, time.Duration, int64, int, []corpusEntry, []reflect.Type, string, string) error {
	return errMain
}
func (f matchStringOnly) RunFuzzWorker(func(corpusEntry) error) error { return errMain }
func (f matchStringOnly) ReadCorpus(string, []reflect.Type) ([]corpusEntry, error) {
	return nil, errMain
}
func (f matchStringOnly) CheckCorpus([]any, []reflect.Type) error { return nil }
func (f matchStringOnly) ResetCoverage()                          {}
func (f matchStringOnly) SnapshotCoverage()                       {}

func (f matchStringOnly) InitRuntimeCoverage() (mode string, tearDown func(string, string) (string, error), snapcov func() float64) {
	return
}

// Main is an internal function, part of the implementation of the "go test" command.
// It was exported because it is cross-package and predates "internal" packages.
// It is no longer used by "go test" but preserved, as much as possible, for other
// systems that simulate "go test" using Main, but Main sometimes cannot be updated as
// new functionality is added to the testing package.
// Systems simulating "go test" should be updated to use [MainStart].
func Main(matchString func(pat, str string) (bool, error), tests []InternalTest, benchmarks []InternalBenchmark, examples []InternalExample) {
	os.Exit(MainStart(matchStringOnly(matchString), tests, benchmarks, nil, examples).Run())
}

// M is a type passed to a TestMain function to run the actual tests.
type M struct {
	deps        testDeps
	tests       []InternalTest
	benchmarks  []InternalBenchmark
	fuzzTargets []InternalFuzzTarget
	examples    []InternalExample

	timer     *time.Timer
	afterOnce sync.Once

	numRun int

	// value to pass to os.Exit, the outer test func main
	// harness calls os.Exit with this code. See #34129.
	exitCode int
}

// testDeps is an internal interface of functionality that is
// passed into this package by a test's generated main package.
// The canonical implementation of this interface is
// testing/internal/testdeps's TestDeps.
type testDeps interface {
	ImportPath() string
	ModulePath() string
	MatchString(pat, str string) (bool, error)
	SetPanicOnExit0(bool)
	StartCPUProfile(io.Writer) error
	StopCPUProfile()
	StartTestLog(io.Writer)
	StopTestLog() error
	WriteProfileTo(string, io.Writer, int) error
	CoordinateFuzzing(time.Duration, int64, time.Duration, int64, int, []corpusEntry, []reflect.Type, string, string) error
	RunFuzzWorker(func(corpusEntry) error) error
	ReadCorpus(string, []reflect.Type) ([]corpusEntry, error)
	CheckCorpus([]any, []reflect.Type) error
	ResetCoverage()
	SnapshotCoverage()
	InitRuntimeCoverage() (mode string, tearDown func(coverprofile string, gocoverdir string) (string, error), snapcov func() float64)
}

// MainStart is meant for use by tests generated by 'go test'.
// It is not meant to be called directly and is not subject to the Go 1 compatibility document.
// It may change signature from release to release.
func MainStart(deps testDeps, tests []InternalTest, benchmarks []InternalBenchmark, fuzzTargets []InternalFuzzTarget, examples []InternalExample) *M {
	registerCover(deps.InitRuntimeCoverage())
	Init()
	return &M{
		deps:        deps,
		tests:       tests,
		benchmarks:  benchmarks,
		fuzzTargets: fuzzTargets,
		examples:    examples,
	}
}

var (
	testingTesting bool
	realStderr     *os.File
)

// Run runs the tests. It returns an exit code to pass to os.Exit.
// The exit code is zero when all tests pass, and non-zero for any kind
// of failure. For machine readable test results, parse the output of
// 'go test -json'.
func (m *M) Run() (code int) {
	defer func() {
		code = m.exitCode
	}()

	// Count the number of calls to m.Run.
	// We only ever expected 1, but we didn't enforce that,
	// and now there are tests in the wild that call m.Run multiple times.
	// Sigh. go.dev/issue/23129.
	m.numRun++

	// TestMain may have already called flag.Parse.
	if !flag.Parsed() {
		flag.Parse()
	}

	if chatty.json {
		// With -v=json, stdout and stderr are pointing to the same pipe,
		// which is leading into test2json. In general, operating systems
		// do a good job of ensuring that writes to the same pipe through
		// different file descriptors are delivered whole, so that writing
		// AAA to stdout and BBB to stderr simultaneously produces
		// AAABBB or BBBAAA on the pipe, not something like AABBBA.
		// However, the exception to this is when the pipe fills: in that
		// case, Go's use of non-blocking I/O means that writing AAA
		// or BBB might be split across multiple system calls, making it
		// entirely possible to get output like AABBBA. The same problem
		// happens inside the operating system kernel if we switch to
		// blocking I/O on the pipe. This interleaved output can do things
		// like print unrelated messages in the middle of a TestFoo line,
		// which confuses test2json. Setting os.Stderr = os.Stdout will make
		// them share a single pfd, which will hold a lock for each program
		// write, preventing any interleaving.
		//
		// It might be nice to set Stderr = Stdout always, or perhaps if
		// we can tell they are the same file, but for now -v=json is
		// a very clear signal. Making the two files the same may cause
		// surprises if programs close os.Stdout but expect to be able
		// to continue to write to os.Stderr, but it's hard to see why a
		// test would think it could take over global state that way.
		//
		// This fix only helps programs where the output is coming directly
		// from Go code. It does not help programs in which a subprocess is
		// writing to stderr or stdout at the same time that a Go test is writing output.
		// It also does not help when the output is coming from the runtime,
		// such as when using the print/println functions, since that code writes
		// directly to fd 2 without any locking.
		// We keep realStderr around to prevent fd 2 from being closed.
		//
		// See go.dev/issue/33419.
		realStderr = os.Stderr
		os.Stderr = os.Stdout
	}

	if *parallel < 1 {
		fmt.Fprintln(os.Stderr, "testing: -parallel can only be given a positive integer")
		flag.Usage()
		m.exitCode = 2
		return
	}
	if *matchFuzz != "" && *fuzzCacheDir == "" {
		fmt.Fprintln(os.Stderr, "testing: -test.fuzzcachedir must be set if -test.fuzz is set")
		flag.Usage()
		m.exitCode = 2
		return
	}

	if *matchList != "" {
		listTests(m.deps.MatchString, m.tests, m.benchmarks, m.fuzzTargets, m.examples)
		m.exitCode = 0
		return
	}

	if *shuffle != "off" {
		var n int64
		var err error
		if *shuffle == "on" {
			n = time.Now().UnixNano()
		} else {
			n, err = strconv.ParseInt(*shuffle, 10, 64)
			if err != nil {
				fmt.Fprintln(os.Stderr, `testing: -shuffle should be "off", "on", or a valid integer:`, err)
				m.exitCode = 2
				return
			}
		}
		fmt.Println("-test.shuffle", n)
		rng := rand.New(rand.NewSource(n))
		rng.Shuffle(len(m.tests), func(i, j int) { m.tests[i], m.tests[j] = m.tests[j], m.tests[i] })
		rng.Shuffle(len(m.benchmarks), func(i, j int) { m.benchmarks[i], m.benchmarks[j] = m.benchmarks[j], m.benchmarks[i] })
	}

	parseCpuList()

	m.before()
	defer m.after()

	// Run tests, examples, and benchmarks unless this is a fuzz worker process.
	// Workers start after this is done by their parent process, and they should
	// not repeat this work.
	if !*isFuzzWorker {
		deadline := m.startAlarm()
		haveExamples = len(m.examples) > 0
		testRan, testOk := runTests(m.deps.ModulePath(), m.deps.ImportPath(), m.deps.MatchString, m.tests, deadline)
		fuzzTargetsRan, fuzzTargetsOk := runFuzzTests(m.deps, m.fuzzTargets, deadline)
		exampleRan, exampleOk := runExamples(m.deps.MatchString, m.examples)
		m.stopAlarm()
		if !testRan && !exampleRan && !fuzzTargetsRan && *matchBenchmarks == "" && *matchFuzz == "" {
			fmt.Fprintln(os.Stderr, "testing: warning: no tests to run")
			if testingTesting && *match != "^$" {
				// If this happens during testing of package testing it could be that
				// package testing's own logic for when to run a test is broken,
				// in which case every test will run nothing and succeed,
				// with no obvious way to detect this problem (since no tests are running).
				// So make 'no tests to run' a hard failure when testing package testing itself.
				fmt.Print(chatty.prefix(), "FAIL: package testing must run tests\n")
				testOk = false
			}
		}
		anyFailed := !testOk || !exampleOk || !fuzzTargetsOk || !runBenchmarks(m.deps.ImportPath(), m.deps.MatchString, m.benchmarks)
		if !anyFailed && race.Errors() > 0 {
			fmt.Print(chatty.prefix(), "testing: race detected outside of test execution\n")
			anyFailed = true
		}
		if anyFailed {
			fmt.Print(chatty.prefix(), "FAIL\n")
			m.exitCode = 1
			return
		}
	}

	fuzzingOk := runFuzzing(m.deps, m.fuzzTargets)
	if !fuzzingOk {
		fmt.Print(chatty.prefix(), "FAIL\n")
		if *isFuzzWorker {
			m.exitCode = fuzzWorkerExitCode
		} else {
			m.exitCode = 1
		}
		return
	}

	m.exitCode = 0
	if !*isFuzzWorker {
		fmt.Print(chatty.prefix(), "PASS\n")
	}
	return
}

func (t *T) report() {
	if t.parent == nil {
		return
	}
	if t.isSynctest {
		return // t.parent will handle reporting
	}
	dstr := fmtDuration(t.duration)
	format := "--- %s: %s (%s)\n"
	if t.Failed() {
		t.flushToParent(t.name, format, "FAIL", t.name, dstr)
	} else if t.chatty != nil {
		if t.Skipped() {
			t.flushToParent(t.name, format, "SKIP", t.name, dstr)
		} else {
			t.flushToParent(t.name, format, "PASS", t.name, dstr)
		}
	}
}

func listTests(matchString func(pat, str string) (bool, error), tests []InternalTest, benchmarks []InternalBenchmark, fuzzTargets []InternalFuzzTarget, examples []InternalExample) {
	if _, err := matchString(*matchList, "non-empty"); err != nil {
		fmt.Fprintf(os.Stderr, "testing: invalid regexp in -test.list (%q): %s\n", *matchList, err)
		os.Exit(1)
	}

	for _, test := range tests {
		if ok, _ := matchString(*matchList, test.Name); ok {
			fmt.Println(test.Name)
		}
	}
	for _, bench := range benchmarks {
		if ok, _ := matchString(*matchList, bench.Name); ok {
			fmt.Println(bench.Name)
		}
	}
	for _, fuzzTarget := range fuzzTargets {
		if ok, _ := matchString(*matchList, fuzzTarget.Name); ok {
			fmt.Println(fuzzTarget.Name)
		}
	}
	for _, example := range examples {
		if ok, _ := matchString(*matchList, example.Name); ok {
			fmt.Println(example.Name)
		}
	}
}

// RunTests is an internal function but exported because it is cross-package;
// it is part of the implementation of the "go test" command.
func RunTests(matchString func(pat, str string) (bool, error), tests []InternalTest) (ok bool) {
	var deadline time.Time
	if *timeout > 0 {
		deadline = time.Now().Add(*timeout)
	}
	ran, ok := runTests("", "", matchString, tests, deadline)
	if !ran && !haveExamples {
		fmt.Fprintln(os.Stderr, "testing: warning: no tests to run")
	}
	return ok
}

func runTests(modulePath, importPath string, matchString func(pat, str string) (bool, error), tests []InternalTest, deadline time.Time) (ran, ok bool) {
	ok = true
	for _, procs := range cpuList {
		runtime.GOMAXPROCS(procs)
		for i := uint(0); i < *count; i++ {
			if shouldFailFast() {
				break
			}
			if i > 0 && !ran {
				// There were no tests to run on the first
				// iteration. This won't change, so no reason
				// to keep trying.
				break
			}
			ctx, cancelCtx := context.WithCancel(context.Background())
			tstate := newTestState(*parallel, newMatcher(matchString, *match, "-test.run", *skip))
			tstate.deadline = deadline
			t := &T{
				common: common{
					signal:     make(chan bool, 1),
					barrier:    make(chan bool),
					w:          os.Stdout,
					ctx:        ctx,
					cancelCtx:  cancelCtx,
					modulePath: modulePath,
					importPath: importPath,
				},
				tstate: tstate,
			}
			if Verbose() {
				t.chatty = newChattyPrinter(t.w)
			}
			tRunner(t, func(t *T) {
				for _, test := range tests {
					t.Run(test.Name, test.F)
				}
			})
			select {
			case <-t.signal:
			default:
				panic("internal error: tRunner exited without sending on t.signal")
			}
			ok = ok && !t.Failed()
			ran = ran || t.ran
		}
	}
	return ran, ok
}

// before runs before all testing.
func (m *M) before() {
	if *memProfileRate > 0 {
		runtime.MemProfileRate = *memProfileRate
	}
	if *cpuProfile != "" {
		f, err := os.Create(toOutputDir(*cpuProfile))
		if err != nil {
			fmt.Fprintf(os.Stderr, "testing: %s\n", err)
			return
		}
		if err := m.deps.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "testing: can't start cpu profile: %s\n", err)
			f.Close()
			return
		}
		// Could save f so after can call f.Close; not worth the effort.
	}
	if *traceFile != "" {
		f, err := os.Create(toOutputDir(*traceFile))
		if err != nil {
			fmt.Fprintf(os.Stderr, "testing: %s\n", err)
			return
		}
		if err := trace.Start(f); err != nil {
			fmt.Fprintf(os.Stderr, "testing: can't start tracing: %s\n", err)
			f.Close()
			return
		}
		// Could save f so after can call f.Close; not worth the effort.
	}
	if *blockProfile != "" && *blockProfileRate >= 0 {
		runtime.SetBlockProfileRate(*blockProfileRate)
	}
	if *mutexProfile != "" && *mutexProfileFraction >= 0 {
		runtime.SetMutexProfileFraction(*mutexProfileFraction)
	}
	if *coverProfile != "" && CoverMode() == "" {
		fmt.Fprintf(os.Stderr, "testing: cannot use -test.coverprofile because test binary was not built with coverage enabled\n")
		os.Exit(2)
	}
	if *gocoverdir != "" && CoverMode() == "" {
		fmt.Fprintf(os.Stderr, "testing: cannot use -test.gocoverdir because test binary was not built with coverage enabled\n")
		os.Exit(2)
	}
	if *artifacts {
		var err error
		artifactDir, err = filepath.Abs(toOutputDir("_artifacts"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "testing: cannot make -test.outputdir absolute: %v\n", err)
			os.Exit(2)
		}
		if err := os.Mkdir(artifactDir, 0o777); err != nil && !errors.Is(err, os.ErrExist) {
			fmt.Fprintf(os.Stderr, "testing: %v\n", err)
			os.Exit(2)
		}
	}
	if *testlog != "" {
		// Note: Not using toOutputDir.
		// This file is for use by cmd/go, not users.
		var f *os.File
		var err error
		if m.numRun == 1 {
			f, err = os.Create(*testlog)
		} else {
			f, err = os.OpenFile(*testlog, os.O_WRONLY, 0)
			if err == nil {
				f.Seek(0, io.SeekEnd)
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "testing: %s\n", err)
			os.Exit(2)
		}
		m.deps.StartTestLog(f)
		testlogFile = f
	}
	if *panicOnExit0 {
		m.deps.SetPanicOnExit0(true)
	}
}

// after runs after all testing.
func (m *M) after() {
	m.afterOnce.Do(func() {
		m.writeProfiles()
	})

	// Restore PanicOnExit0 after every run, because we set it to true before
	// every run. Otherwise, if m.Run is called multiple times the behavior of
	// os.Exit(0) will not be restored after the second run.
	if *panicOnExit0 {
		m.deps.SetPanicOnExit0(false)
	}
}

func (m *M) writeProfiles() {
	if *testlog != "" {
		if err := m.deps.StopTestLog(); err != nil {
			fmt.Fprintf(os.Stderr, "testing: can't write %s: %s\n", *testlog, err)
			os.Exit(2)
		}
		if err := testlogFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "testing: can't write %s: %s\n", *testlog, err)
			os.Exit(2)
		}
	}
	if *cpuProfile != "" {
		m.deps.StopCPUProfile() // flushes profile to disk
	}
	if *traceFile != "" {
		trace.Stop() // flushes trace to disk
	}
	if *memProfile != "" {
		f, err := os.Create(toOutputDir(*memProfile))
		if err != nil {
			fmt.Fprintf(os.Stderr, "testing: %s\n", err)
			os.Exit(2)
		}
		runtime.GC() // materialize all statistics
		if err = m.deps.WriteProfileTo("allocs", f, 0); err != nil {
			fmt.Fprintf(os.Stderr, "testing: can't write %s: %s\n", *memProfile, err)
			os.Exit(2)
		}
		f.Close()
	}
	if *blockProfile != "" && *blockProfileRate >= 0 {
		f, err := os.Create(toOutputDir(*blockProfile))
		if err != nil {
			fmt.Fprintf(os.Stderr, "testing: %s\n", err)
			os.Exit(2)
		}
		if err = m.deps.WriteProfileTo("block", f, 0); err != nil {
			fmt.Fprintf(os.Stderr, "testing: can't write %s: %s\n", *blockProfile, err)
			os.Exit(2)
		}
		f.Close()
	}
	if *mutexProfile != "" && *mutexProfileFraction >= 0 {
		f, err := os.Create(toOutputDir(*mutexProfile))
		if err != nil {
			fmt.Fprintf(os.Stderr, "testing: %s\n", err)
			os.Exit(2)
		}
		if err = m.deps.WriteProfileTo("mutex", f, 0); err != nil {
			fmt.Fprintf(os.Stderr, "testing: can't write %s: %s\n", *mutexProfile, err)
			os.Exit(2)
		}
		f.Close()
	}
	if CoverMode() != "" {
		coverReport()
	}
}

// toOutputDir returns the file name relocated, if required, to outputDir.
// Simple implementation to avoid pulling in path/filepath.
func toOutputDir(path string) string {
	if *outputDir == "" || path == "" {
		return path
	}
	// On Windows, it's clumsy, but we can be almost always correct
	// by just looking for a drive letter and a colon.
	// Absolute paths always have a drive letter (ignoring UNC).
	// Problem: if path == "C:A" and outputdir == "C:\Go" it's unclear
	// what to do, but even then path/filepath doesn't help.
	// TODO: Worth doing better? Probably not, because we're here only
	// under the management of go test.
	if runtime.GOOS == "windows" && len(path) >= 2 {
		letter, colon := path[0], path[1]
		if ('a' <= letter && letter <= 'z' || 'A' <= letter && letter <= 'Z') && colon == ':' {
			// If path starts with a drive letter we're stuck with it regardless.
			return path
		}
	}
	if os.IsPathSeparator(path[0]) {
		return path
	}
	return fmt.Sprintf("%s%c%s", *outputDir, os.PathSeparator, path)
}

// startAlarm starts an alarm if requested.
func (m *M) startAlarm() time.Time {
	if *timeout <= 0 {
		return time.Time{}
	}

	deadline := time.Now().Add(*timeout)
	m.timer = time.AfterFunc(*timeout, func() {
		m.after()
		debug.SetTraceback("all")
		extra := ""

		if list := runningList(); len(list) > 0 {
			var b strings.Builder
			b.WriteString("\nrunning tests:")
			for _, name := range list {
				b.WriteString("\n\t")
				b.WriteString(name)
			}
			extra = b.String()
		}
		panic(fmt.Sprintf("test timed out after %v%s", *timeout, extra))
	})
	return deadline
}

// runningList returns the list of running tests.
func runningList() []string {
	var list []string
	running.Range(func(k, v any) bool {
		list = append(list, fmt.Sprintf("%s (%v)", k.(string), highPrecisionTimeSince(v.(highPrecisionTime)).Round(time.Second)))
		return true
	})
	slices.Sort(list)
	return list
}

// stopAlarm turns off the alarm.
func (m *M) stopAlarm() {
	if *timeout > 0 {
		m.timer.Stop()
	}
}

func parseCpuList() {
	for val := range strings.SplitSeq(*cpuListStr, ",") {
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		cpu, err := strconv.Atoi(val)
		if err != nil || cpu <= 0 {
			fmt.Fprintf(os.Stderr, "testing: invalid value %q for -test.cpu\n", val)
			os.Exit(1)
		}
		cpuList = append(cpuList, cpu)
	}
	if cpuList == nil {
		cpuList = append(cpuList, runtime.GOMAXPROCS(-1))
	}
}

func shouldFailFast() bool {
	return *failFast && numFailed.Load() > 0
}

```

// === FILE: references/go/src/testing/testing_other.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows

package testing

import "time"

// isWindowsRetryable reports whether err is a Windows error code
// that may be fixed by retrying a failed filesystem operation.
func isWindowsRetryable(err error) bool {
	return false
}

// highPrecisionTime represents a single point in time.
// On all systems except Windows, using time.Time is fine.
type highPrecisionTime struct {
	now time.Time
}

// highPrecisionTimeNow returns high precision time for benchmarking.
func highPrecisionTimeNow() highPrecisionTime {
	return highPrecisionTime{now: time.Now()}
}

// highPrecisionTimeSince returns duration since b.
func highPrecisionTimeSince(b highPrecisionTime) time.Duration {
	return time.Since(b.now)
}

```

// === FILE: references/go/src/testing/testing_windows.go ===
```go
// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package testing

import (
	"errors"
	"internal/syscall/windows"
	"math/bits"
	"syscall"
	"time"
)

// isWindowsRetryable reports whether err is a Windows error code
// that may be fixed by retrying a failed filesystem operation.
func isWindowsRetryable(err error) bool {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			break
		}
		err = unwrapped
	}
	if err == syscall.ERROR_ACCESS_DENIED {
		return true // Observed in https://go.dev/issue/50051.
	}
	if err == windows.ERROR_SHARING_VIOLATION {
		return true // Observed in https://go.dev/issue/51442.
	}
	return false
}

// highPrecisionTime represents a single point in time with query performance counter.
// time.Time on Windows has low system granularity, which is not suitable for
// measuring short time intervals.
//
// TODO: If Windows runtime implements high resolution timing then highPrecisionTime
// can be removed.
type highPrecisionTime struct {
	now int64
}

// highPrecisionTimeNow returns high precision time for benchmarking.
func highPrecisionTimeNow() highPrecisionTime {
	var t highPrecisionTime
	// This should always succeed for Windows XP and above.
	t.now = windows.QueryPerformanceCounter()
	return t
}

func (a highPrecisionTime) sub(b highPrecisionTime) time.Duration {
	delta := a.now - b.now

	if queryPerformanceFrequency == 0 {
		queryPerformanceFrequency = windows.QueryPerformanceFrequency()
	}
	hi, lo := bits.Mul64(uint64(delta), uint64(time.Second)/uint64(time.Nanosecond))
	quo, _ := bits.Div64(hi, lo, uint64(queryPerformanceFrequency))
	return time.Duration(quo)
}

var queryPerformanceFrequency int64

// highPrecisionTimeSince returns duration since a.
func highPrecisionTimeSince(a highPrecisionTime) time.Duration {
	return highPrecisionTimeNow().sub(a)
}

```

