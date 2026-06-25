# Domain Architecture: runtime/trace

## Layout Topology
```text
runtime/trace/
├── annotation.go
├── batch.go
├── encoding.go
├── flightrecorder.go
├── recorder.go
├── subscribe.go
└── trace.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/runtime/trace/annotation.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"context"
	"fmt"
	"sync/atomic"
	_ "unsafe"
)

type traceContextKey struct{}

// NewTask creates a task instance with the type taskType and returns
// it along with a Context that carries the task.
// If the input context contains a task, the new task is its subtask.
//
// The taskType is used to classify task instances. Analysis tools
// like the Go execution tracer may assume there are only a bounded
// number of unique task types in the system.
//
// The returned Task's [Task.End] method is used to mark the task's end.
// The trace tool measures task latency as the time between task creation
// and when the End method is called, and provides the latency
// distribution per task type.
// If the End method is called multiple times, only the first
// call is used in the latency measurement.
//
//	ctx, task := trace.NewTask(ctx, "awesomeTask")
//	trace.WithRegion(ctx, "preparation", prepWork)
//	// preparation of the task
//	go func() {  // continue processing the task in a separate goroutine.
//	    defer task.End()
//	    trace.WithRegion(ctx, "remainingWork", remainingWork)
//	}()
func NewTask(pctx context.Context, taskType string) (ctx context.Context, task *Task) {
	pid := fromContext(pctx).id
	id := newID()
	userTaskCreate(id, pid, taskType)
	s := &Task{id: id}
	return context.WithValue(pctx, traceContextKey{}, s), s

	// We allocate a new task even when
	// the tracing is disabled because the context and task
	// can be used across trace enable/disable boundaries,
	// which complicates the problem.
	//
	// For example, consider the following scenario:
	//   - trace is enabled.
	//   - trace.WithRegion is called, so a new context ctx
	//     with a new region is created.
	//   - trace is disabled.
	//   - trace is enabled again.
	//   - trace APIs with the ctx is called. Is the ID in the task
	//   a valid one to use?
	//
	// TODO(hyangah): reduce the overhead at least when
	// tracing is disabled. Maybe the id can embed a tracing
	// round number and ignore ids generated from previous
	// tracing round.
}

func fromContext(ctx context.Context) *Task {
	if s, ok := ctx.Value(traceContextKey{}).(*Task); ok {
		return s
	}
	return &bgTask
}

// Task is a data type for tracing a user-defined, logical operation.
type Task struct {
	id uint64
	// TODO(hyangah): record parent id?
}

// End marks the end of the operation represented by the [Task].
func (t *Task) End() {
	userTaskEnd(t.id)
}

var lastTaskID uint64 = 0 // task id issued last time

func newID() uint64 {
	// TODO(hyangah): use per-P cache
	return atomic.AddUint64(&lastTaskID, 1)
}

var bgTask = Task{id: uint64(0)}

// Log emits a one-off event with the given category and message.
// Category can be empty and the API assumes there are only a handful of
// unique categories in the system.
func Log(ctx context.Context, category, message string) {
	id := fromContext(ctx).id
	userLog(id, category, message)
}

// Logf is like [Log], but the value is formatted using the specified format spec.
func Logf(ctx context.Context, category, format string, args ...any) {
	if IsEnabled() {
		// Ideally this should be just Log, but that will
		// add one more frame in the stack trace.
		id := fromContext(ctx).id
		userLog(id, category, fmt.Sprintf(format, args...))
	}
}

const (
	regionStartCode = uint64(0)
	regionEndCode   = uint64(1)
)

// WithRegion starts a region associated with its calling goroutine, runs fn,
// and then ends the region. If the context carries a task, the region is
// associated with the task. Otherwise, the region is attached to the background
// task.
//
// The regionType is used to classify regions, so there should be only a
// handful of unique region types.
func WithRegion(ctx context.Context, regionType string, fn func()) {
	// NOTE:
	// WithRegion helps avoiding misuse of the API but in practice,
	// this is very restrictive:
	// - Use of WithRegion makes the stack traces captured from
	//   region start and end are identical.
	// - Refactoring the existing code to use WithRegion is sometimes
	//   hard and makes the code less readable.
	//     e.g. code block nested deep in the loop with various
	//          exit point with return values
	// - Refactoring the code to use this API with closure can
	//   cause different GC behavior such as retaining some parameters
	//   longer.
	// This causes more churns in code than I hoped, and sometimes
	// makes the code less readable.

	id := fromContext(ctx).id
	userRegion(id, regionStartCode, regionType)
	defer userRegion(id, regionEndCode, regionType)
	fn()
}

// StartRegion starts a region and returns it.
// The returned Region's [Region.End] method must be called
// from the same goroutine where the region was started.
// Within each goroutine, regions must nest. That is, regions started
// after this region must be ended before this region can be ended.
// Recommended usage is
//
//	defer trace.StartRegion(ctx, "myTracedRegion").End()
func StartRegion(ctx context.Context, regionType string) *Region {
	if !IsEnabled() {
		return noopRegion
	}
	id := fromContext(ctx).id
	userRegion(id, regionStartCode, regionType)
	return &Region{id, regionType}
}

// Region is a region of code whose execution time interval is traced.
type Region struct {
	id         uint64
	regionType string
}

var noopRegion = &Region{}

// End marks the end of the traced code region.
func (r *Region) End() {
	if r == noopRegion {
		return
	}
	userRegion(r.id, regionEndCode, r.regionType)
}

// IsEnabled reports whether tracing is enabled.
// The information is advisory only. The tracing status
// may have changed by the time this function returns.
func IsEnabled() bool {
	return tracing.enabled.Load()
}

//
// Function bodies are defined in runtime/trace.go
//

// emits UserTaskCreate event.
func userTaskCreate(id, parentID uint64, taskType string)

// emits UserTaskEnd event.
func userTaskEnd(id uint64)

// emits UserRegion event.
func userRegion(id, mode uint64, regionType string)

// emits UserLog event.
func userLog(id uint64, category, message string)

```

// === FILE: references!/go/src/runtime/trace/batch.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"fmt"
	"internal/trace/tracev2"
)

// timestamp is an unprocessed timestamp.
type timestamp uint64

type batch struct {
	time timestamp
	gen  uint64
	data []byte
}

// readBatch copies b and parses the trace batch header inside.
// Returns the batch, bytes read, and an error.
func readBatch(b []byte) (batch, uint64, error) {
	if len(b) == 0 {
		return batch{}, 0, fmt.Errorf("batch is empty")
	}
	data := make([]byte, len(b))
	copy(data, b)

	// Read batch header byte.
	if typ := tracev2.EventType(b[0]); typ == tracev2.EvEndOfGeneration {
		if len(b) != 1 {
			return batch{}, 1, fmt.Errorf("unexpected end of generation in batch of size >1")
		}
		return batch{data: data}, 1, nil
	}
	if typ := tracev2.EventType(b[0]); typ != tracev2.EvEventBatch && typ != tracev2.EvExperimentalBatch {
		return batch{}, 1, fmt.Errorf("expected batch event, got event %d", typ)
	}
	total := 1
	b = b[1:]

	// Read the generation
	gen, n, err := readUvarint(b)
	if err != nil {
		return batch{}, uint64(total + n), fmt.Errorf("error reading batch gen: %w", err)
	}
	total += n
	b = b[n:]

	// Read the M (discard it).
	_, n, err = readUvarint(b)
	if err != nil {
		return batch{}, uint64(total + n), fmt.Errorf("error reading batch M ID: %w", err)
	}
	total += n
	b = b[n:]

	// Read the timestamp.
	ts, n, err := readUvarint(b)
	if err != nil {
		return batch{}, uint64(total + n), fmt.Errorf("error reading batch timestamp: %w", err)
	}
	total += n
	b = b[n:]

	// Read the size of the batch to follow.
	size, n, err := readUvarint(b)
	if err != nil {
		return batch{}, uint64(total + n), fmt.Errorf("error reading batch size: %w", err)
	}
	if size > tracev2.MaxBatchSize {
		return batch{}, uint64(total + n), fmt.Errorf("invalid batch size %d, maximum is %d", size, tracev2.MaxBatchSize)
	}
	total += n
	total += int(size)
	if total != len(data) {
		return batch{}, uint64(total), fmt.Errorf("expected complete batch")
	}
	data = data[:total]

	// Return the batch.
	return batch{
		gen:  gen,
		time: timestamp(ts),
		data: data,
	}, uint64(total), nil
}

```

// === FILE: references!/go/src/runtime/trace/encoding.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"errors"
)

// maxVarintLenN is the maximum length of a varint-encoded N-bit integer.
const maxVarintLen64 = 10

var (
	errOverflow = errors.New("binary: varint overflows a 64-bit integer")
	errEOB      = errors.New("binary: end of buffer")
)

// TODO deduplicate this function.
func readUvarint(b []byte) (uint64, int, error) {
	var x uint64
	var s uint
	var byt byte
	for i := 0; i < maxVarintLen64 && i < len(b); i++ {
		byt = b[i]
		if byt < 0x80 {
			if i == maxVarintLen64-1 && byt > 1 {
				return x, i, errOverflow
			}
			return x | uint64(byt)<<s, i + 1, nil
		}
		x |= uint64(byt&0x7f) << s
		s += 7
	}
	return x, len(b), errOverflow
}

// putUvarint encodes a uint64 into buf and returns the number of bytes written.
// If the buffer is too small, PutUvarint will panic.
// TODO deduplicate this function.
func putUvarint(buf []byte, x uint64) int {
	i := 0
	for x >= 0x80 {
		buf[i] = byte(x) | 0x80
		x >>= 7
		i++
	}
	buf[i] = byte(x)
	return i + 1
}

```

// === FILE: references!/go/src/runtime/trace/flightrecorder.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"fmt"
	"io"
	"sync"
	"time"
	_ "unsafe" // added for go linkname usage
)

// FlightRecorder represents a single consumer of a Go execution
// trace.
// It tracks a moving window over the execution trace produced by
// the runtime, always containing the most recent trace data.
//
// At most one flight recorder may be active at any given time,
// though flight recording is allowed to be concurrently active
// with a trace consumer using trace.Start.
// This restriction of only a single flight recorder may be removed
// in the future.
type FlightRecorder struct {
	err error

	// State specific to the recorder.
	header [16]byte
	active rawGeneration
	ringMu sync.Mutex
	ring   []rawGeneration
	freq   frequency // timestamp conversion factor, from the runtime

	// Externally-set options.
	targetSize   uint64
	targetPeriod time.Duration

	enabled bool       // whether the flight recorder is enabled.
	writing sync.Mutex // protects concurrent calls to WriteTo

	// The values of targetSize and targetPeriod we've committed to since the last Start.
	wantSize uint64
	wantDur  time.Duration
}

// NewFlightRecorder creates a new flight recorder from the provided configuration.
func NewFlightRecorder(cfg FlightRecorderConfig) *FlightRecorder {
	fr := new(FlightRecorder)
	if cfg.MaxBytes != 0 {
		fr.targetSize = cfg.MaxBytes
	} else {
		fr.targetSize = 10 << 20 // 10 MiB.
	}

	if cfg.MinAge != 0 {
		fr.targetPeriod = cfg.MinAge
	} else {
		fr.targetPeriod = 10 * time.Second
	}
	return fr
}

// Start activates the flight recorder and begins recording trace data.
// Only one call to trace.Start may be active at any given time.
// In addition, currently only one flight recorder may be active in the program.
// Returns an error if the flight recorder cannot be started or is already started.
func (fr *FlightRecorder) Start() error {
	if fr.enabled {
		return fmt.Errorf("cannot enable a enabled flight recorder")
	}
	fr.wantSize = fr.targetSize
	fr.wantDur = fr.targetPeriod
	fr.err = nil
	fr.freq = frequency(1.0 / (float64(runtime_traceClockUnitsPerSecond()) / 1e9))

	// Start tracing, data is sent to a recorder which forwards it to our own
	// storage.
	if err := tracing.subscribeFlightRecorder(&recorder{r: fr}); err != nil {
		return err
	}

	fr.enabled = true
	return nil
}

// Stop ends recording of trace data. It blocks until any concurrent WriteTo calls complete.
func (fr *FlightRecorder) Stop() {
	if !fr.enabled {
		return
	}
	fr.enabled = false
	tracing.unsubscribeFlightRecorder()

	// Reset all state. No need to lock because the reader has already exited.
	fr.active = rawGeneration{}
	fr.ring = nil
}

// Enabled returns true if the flight recorder is active.
// Specifically, it will return true if Start did not return an error, and Stop has not yet been called.
// It is safe to call from multiple goroutines simultaneously.
func (fr *FlightRecorder) Enabled() bool { return fr.enabled }

// WriteTo snapshots the moving window tracked by the flight recorder.
// The snapshot is expected to contain data that is up-to-date as of when WriteTo is called,
// though this is not a hard guarantee.
// Only one goroutine may execute WriteTo at a time.
// An error is returned upon failure to write to w, if another WriteTo call is already in-progress,
// or if the flight recorder is inactive.
func (fr *FlightRecorder) WriteTo(w io.Writer) (n int64, err error) {
	if !fr.enabled {
		return 0, fmt.Errorf("cannot snapshot a disabled flight recorder")
	}
	if !fr.writing.TryLock() {
		// Indicates that a call to WriteTo was made while one was already in progress.
		// If the caller of WriteTo sees this error, they should use the result from the other call to WriteTo.
		return 0, fmt.Errorf("call to WriteTo for trace.FlightRecorder already in progress")
	}
	defer fr.writing.Unlock()

	// Force a global buffer flush.
	runtime_traceAdvance(false)

	// Now that everything has been flushed and written, grab whatever we have.
	//
	// N.B. traceAdvance blocks until the tracer goroutine has actually written everything
	// out, which means the generation we just flushed must have been already been observed
	// by the recorder goroutine. Because we flushed twice, the first flush is guaranteed to
	// have been both completed *and* processed by the recorder goroutine.
	fr.ringMu.Lock()
	gens := fr.ring
	fr.ringMu.Unlock()

	// Write the header.
	nw, err := w.Write(fr.header[:])
	if err != nil {
		return int64(nw), err
	}
	n += int64(nw)

	// Write all the data.
	for _, gen := range gens {
		for _, data := range gen.batches {
			// Write batch data.
			nw, err = w.Write(data)
			n += int64(nw)
			if err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

type FlightRecorderConfig struct {
	// MinAge is a lower bound on the age of an event in the flight recorder's window.
	//
	// The flight recorder will strive to promptly discard events older than the minimum age,
	// but older events may appear in the window snapshot. The age setting will always be
	// overridden by MaxBytes.
	//
	// If this is 0, the minimum age is implementation defined, but can be assumed to be on the order
	// of seconds.
	MinAge time.Duration

	// MaxBytes is an upper bound on the size of the window in bytes.
	//
	// This setting takes precedence over MinAge.
	// However, it does not make any guarantees on the size of the data WriteTo will write,
	// nor does it guarantee memory overheads will always stay below MaxBytes. Treat it
	// as a hint.
	//
	// If this is 0, the maximum size is implementation defined.
	MaxBytes uint64
}

//go:linkname runtime_traceClockUnitsPerSecond
func runtime_traceClockUnitsPerSecond() uint64

//go:linkname runtime_traceAdvance runtime.traceAdvance
func runtime_traceAdvance(stopTrace bool)

```

// === FILE: references!/go/src/runtime/trace/recorder.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"fmt"
	"slices"
	"time"
	_ "unsafe" // added for go linkname usage
)

// A recorder receives bytes from the runtime tracer, processes it.
type recorder struct {
	r *FlightRecorder

	headerReceived bool
}

func (w *recorder) Write(b []byte) (n int, err error) {
	r := w.r

	defer func() {
		if err != nil {
			// Propagate errors to the flightrecorder.
			if r.err == nil {
				r.err = err
			}
		}
	}()

	if !w.headerReceived {
		if len(b) < len(r.header) {
			return 0, fmt.Errorf("expected at least %d bytes in the first write", len(r.header))
		}
		r.header = ([16]byte)(b[:16])
		n += 16
		w.headerReceived = true
	}
	if len(b) == n {
		return n, nil
	}
	ba, nb, err := readBatch(b[n:]) // Every write from the runtime is guaranteed to be a complete batch.
	if err != nil {
		return len(b) - int(nb) - n, err
	}
	n += int(nb)

	// Append the batch to the current generation.
	if ba.gen != 0 && r.active.gen == 0 {
		r.active.gen = ba.gen
	}
	if ba.time != 0 && (r.active.minTime == 0 || r.active.minTime > r.freq.mul(ba.time)) {
		r.active.minTime = r.freq.mul(ba.time)
	}
	r.active.size += len(ba.data)
	r.active.batches = append(r.active.batches, ba.data)

	return len(b), nil
}

func (w *recorder) endGeneration() {
	r := w.r

	// Check if we're entering a new generation.
	r.ringMu.Lock()

	// Get the current trace clock time.
	now := traceTimeNow(r.freq)

	// Add the current generation to the ring. Make sure we always have at least one
	// complete generation by putting the active generation onto the new list, regardless
	// of whatever our settings are.
	//
	// N.B. Let's completely replace the ring here, so that WriteTo can just make a copy
	// and not worry about aliasing. This creates allocations, but at a very low rate.
	newRing := []rawGeneration{r.active}
	size := r.active.size
	for i := len(r.ring) - 1; i >= 0; i-- {
		// Stop adding older generations if the new ring already exceeds the thresholds.
		// This ensures we keep generations that cross a threshold, but not any that lie
		// entirely outside it.
		if uint64(size) > r.wantSize || now.Sub(newRing[len(newRing)-1].minTime) > r.wantDur {
			break
		}
		size += r.ring[i].size
		newRing = append(newRing, r.ring[i])
	}
	slices.Reverse(newRing)
	r.ring = newRing
	r.ringMu.Unlock()

	// Start a new active generation.
	r.active = rawGeneration{}
}

type rawGeneration struct {
	gen     uint64
	size    int
	minTime eventTime
	batches [][]byte
}

func traceTimeNow(freq frequency) eventTime {
	return freq.mul(timestamp(runtime_traceClockNow()))
}

//go:linkname runtime_traceClockNow runtime.traceClockNow
func runtime_traceClockNow() uint64

// frequency is nanoseconds per timestamp unit.
type frequency float64

// mul multiplies an unprocessed timestamp to produce a time in nanoseconds.
func (f frequency) mul(t timestamp) eventTime {
	return eventTime(float64(t) * float64(f))
}

// eventTime is a timestamp in nanoseconds.
//
// It corresponds to the monotonic clock on the platform that the
// trace was taken, and so is possible to correlate with timestamps
// for other traces taken on the same machine using the same clock
// (i.e. no reboots in between).
//
// The actual absolute value of the timestamp is only meaningful in
// relation to other timestamps from the same clock.
//
// BUG: Timestamps coming from traces on Windows platforms are
// only comparable with timestamps from the same trace. Timestamps
// across traces cannot be compared, because the system clock is
// not used as of Go 1.22.
//
// BUG: Traces produced by Go versions 1.21 and earlier cannot be
// compared with timestamps from other traces taken on the same
// machine. This is because the system clock was not used at all
// to collect those timestamps.
type eventTime int64

// Sub subtracts t0 from t, returning the duration in nanoseconds.
func (t eventTime) Sub(t0 eventTime) time.Duration {
	return time.Duration(int64(t) - int64(t0))
}

```

// === FILE: references!/go/src/runtime/trace/subscribe.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trace

import (
	"fmt"
	"internal/trace/tracev2"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	_ "unsafe"
)

var tracing traceMultiplexer

type traceMultiplexer struct {
	sync.Mutex
	enabled     atomic.Bool
	subscribers int

	subscribersMu    sync.Mutex
	traceStartWriter io.Writer
	flightRecorder   *recorder
}

func (t *traceMultiplexer) subscribeFlightRecorder(r *recorder) error {
	t.Lock()
	defer t.Unlock()

	t.subscribersMu.Lock()
	if t.flightRecorder != nil {
		t.subscribersMu.Unlock()
		return fmt.Errorf("flight recorder already enabled")
	}
	t.flightRecorder = r
	t.subscribersMu.Unlock()

	if err := t.addedSubscriber(); err != nil {
		t.subscribersMu.Lock()
		t.flightRecorder = nil
		t.subscribersMu.Unlock()
		return err
	}
	return nil
}

func (t *traceMultiplexer) unsubscribeFlightRecorder() error {
	t.Lock()
	defer t.Unlock()

	t.removingSubscriber()

	t.subscribersMu.Lock()
	if t.flightRecorder == nil {
		t.subscribersMu.Unlock()
		return fmt.Errorf("attempt to unsubscribe missing flight recorder")
	}
	t.flightRecorder = nil
	t.subscribersMu.Unlock()

	t.removedSubscriber()
	return nil
}

func (t *traceMultiplexer) subscribeTraceStartWriter(w io.Writer) error {
	t.Lock()
	defer t.Unlock()

	t.subscribersMu.Lock()
	if t.traceStartWriter != nil {
		t.subscribersMu.Unlock()
		return fmt.Errorf("execution tracer already enabled")
	}
	t.traceStartWriter = w
	t.subscribersMu.Unlock()

	if err := t.addedSubscriber(); err != nil {
		t.subscribersMu.Lock()
		t.traceStartWriter = nil
		t.subscribersMu.Unlock()
		return err
	}
	return nil
}

func (t *traceMultiplexer) unsubscribeTraceStartWriter() {
	t.Lock()
	defer t.Unlock()

	t.removingSubscriber()

	t.subscribersMu.Lock()
	if t.traceStartWriter == nil {
		t.subscribersMu.Unlock()
		return
	}
	t.traceStartWriter = nil
	t.subscribersMu.Unlock()

	t.removedSubscriber()
	return
}

func (t *traceMultiplexer) addedSubscriber() error {
	if t.enabled.Load() {
		// This is necessary for the trace reader goroutine to pick up on the new subscriber.
		runtime_traceAdvance(false)
	} else {
		if err := t.startLocked(); err != nil {
			return err
		}
	}
	t.subscribers++
	return nil
}

func (t *traceMultiplexer) removingSubscriber() {
	if t.subscribers == 0 {
		return
	}
	t.subscribers--
	if t.subscribers == 0 {
		runtime.StopTrace()
		t.enabled.Store(false)
	} else {
		// This is necessary to avoid missing trace data when the system is under high load.
		runtime_traceAdvance(false)
	}
}

func (t *traceMultiplexer) removedSubscriber() {
	if t.subscribers > 0 {
		// This is necessary for the trace reader goroutine to pick up on the new subscriber.
		runtime_traceAdvance(false)
	}
}

func (t *traceMultiplexer) startLocked() error {
	if err := runtime.StartTrace(); err != nil {
		return err
	}

	// Grab the trace reader goroutine's subscribers.
	//
	// We only update our subscribers if we see an end-of-generation
	// signal from the runtime after this, so any new subscriptions
	// or unsubscriptions must call traceAdvance to ensure the reader
	// goroutine sees an end-of-generation signal.
	t.subscribersMu.Lock()
	flightRecorder := t.flightRecorder
	traceStartWriter := t.traceStartWriter
	t.subscribersMu.Unlock()

	go func() {
		header := runtime.ReadTrace()
		if traceStartWriter != nil {
			traceStartWriter.Write(header)
		}
		if flightRecorder != nil {
			flightRecorder.Write(header)
		}

		for {
			data := runtime.ReadTrace()
			if data == nil {
				break
			}
			if traceStartWriter != nil {
				traceStartWriter.Write(data)
			}
			if flightRecorder != nil {
				flightRecorder.Write(data)
			}
			if len(data) == 1 && tracev2.EventType(data[0]) == tracev2.EvEndOfGeneration {
				if flightRecorder != nil {
					flightRecorder.endGeneration()
				}

				// Pick up any changes.
				t.subscribersMu.Lock()
				frIsNew := flightRecorder != t.flightRecorder && t.flightRecorder != nil
				trIsNew := traceStartWriter != t.traceStartWriter && t.traceStartWriter != nil
				flightRecorder = t.flightRecorder
				traceStartWriter = t.traceStartWriter
				t.subscribersMu.Unlock()

				if trIsNew {
					traceStartWriter.Write(header)
				}
				if frIsNew {
					flightRecorder.Write(header)
				}
			}
		}
	}()
	t.enabled.Store(true)
	return nil
}

```

// === FILE: references!/go/src/runtime/trace/trace.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package trace contains facilities for programs to generate traces
// for the Go execution tracer.
//
// # Tracing runtime activities
//
// The execution trace captures a wide range of execution events such as
// goroutine creation/blocking/unblocking, syscall enter/exit/block,
// GC-related events, changes of heap size, processor start/stop, etc.
// When CPU profiling is active, the execution tracer makes an effort to
// include those samples as well.
// A precise nanosecond-precision timestamp and a stack trace is
// captured for most events. The generated trace can be interpreted
// using `go tool trace`.
//
// Support for tracing tests and benchmarks built with the standard
// testing package is built into `go test`. For example, the following
// command runs the test in the current directory and writes the trace
// file (trace.out).
//
//	go test -trace=trace.out
//
// This runtime/trace package provides APIs to add equivalent tracing
// support to a standalone program. See the Example that demonstrates
// how to use this API to enable tracing.
//
// There is also a standard HTTP interface to trace data. Adding the
// following line will install a handler under the /debug/pprof/trace URL
// to download a live trace:
//
//	import _ "net/http/pprof"
//
// See the [net/http/pprof] package for more details about all of the
// debug endpoints installed by this import.
//
// # User annotation
//
// Package trace provides user annotation APIs that can be used to
// log interesting events during execution.
//
// There are three types of user annotations: log messages, regions,
// and tasks.
//
// [Log] emits a timestamped message to the execution trace along with
// additional information such as the category of the message and
// which goroutine called [Log]. The execution tracer provides UIs to filter
// and group goroutines using the log category and the message supplied
// in [Log].
//
// A region is for logging a time interval during a goroutine's execution.
// By definition, a region starts and ends in the same goroutine.
// Regions can be nested to represent subintervals.
// For example, the following code records four regions in the execution
// trace to trace the durations of sequential steps in a cappuccino making
// operation.
//
//	trace.WithRegion(ctx, "makeCappuccino", func() {
//
//	   // orderID allows to identify a specific order
//	   // among many cappuccino order region records.
//	   trace.Log(ctx, "orderID", orderID)
//
//	   trace.WithRegion(ctx, "steamMilk", steamMilk)
//	   trace.WithRegion(ctx, "extractCoffee", extractCoffee)
//	   trace.WithRegion(ctx, "mixMilkCoffee", mixMilkCoffee)
//	})
//
// A task is a higher-level component that aids tracing of logical
// operations such as an RPC request, an HTTP request, or an
// interesting local operation which may require multiple goroutines
// working together. Since tasks can involve multiple goroutines,
// they are tracked via a [context.Context] object. [NewTask] creates
// a new task and embeds it in the returned [context.Context] object.
// Log messages and regions are attached to the task, if any, in the
// Context passed to [Log] and [WithRegion].
//
// For example, assume that we decided to froth milk, extract coffee,
// and mix milk and coffee in separate goroutines. With a task,
// the trace tool can identify the goroutines involved in a specific
// cappuccino order.
//
//	ctx, task := trace.NewTask(ctx, "makeCappuccino")
//	trace.Log(ctx, "orderID", orderID)
//
//	milk := make(chan bool)
//	espresso := make(chan bool)
//
//	go func() {
//	        trace.WithRegion(ctx, "steamMilk", steamMilk)
//	        milk <- true
//	}()
//	go func() {
//	        trace.WithRegion(ctx, "extractCoffee", extractCoffee)
//	        espresso <- true
//	}()
//	go func() {
//	        defer task.End() // When assemble is done, the order is complete.
//	        <-espresso
//	        <-milk
//	        trace.WithRegion(ctx, "mixMilkCoffee", mixMilkCoffee)
//	}()
//
// The trace tool computes the latency of a task by measuring the
// time between the task creation and the task end and provides
// latency distributions for each task type found in the trace.
package trace

import (
	"io"
)

// Start enables tracing for the current program.
// While tracing, the trace will be buffered and written to w.
// Start returns an error if tracing is already enabled.
func Start(w io.Writer) error {
	return tracing.subscribeTraceStartWriter(w)
}

// Stop stops the current tracing, if any.
// Stop only returns after all the writes for the trace have completed.
func Stop() {
	tracing.unsubscribeTraceStartWriter()
}

```

