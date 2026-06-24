# Domain Architecture: cmd/trace

## Layout Topology
```text
cmd/trace/
├── doc.go
├── gen.go
├── goroutinegen.go
├── goroutines.go
├── gstate.go
├── jsontrace.go
├── main.go
├── pprof.go
├── procgen.go
├── regions.go
├── tasks.go
├── threadgen.go
└── viewer.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/trace/doc.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Trace is a tool for viewing trace files.

Trace files can be generated with:
  - runtime/trace.Start
  - net/http/pprof package
  - go test -trace

Example usage:
Generate a trace file with 'go test':

	go test -trace trace.out pkg

View the trace in a web browser:

	go tool trace trace.out

Generate a pprof-like profile from the trace:

	go tool trace -pprof=TYPE trace.out > TYPE.pprof

Supported profile types are:
  - net: network blocking profile
  - sync: synchronization blocking profile
  - syscall: syscall blocking profile
  - sched: scheduler latency profile

Then, you can use the pprof tool to analyze the profile:

	go tool pprof TYPE.pprof

Note that while the various profiles available when launching
'go tool trace' work on every browser, the trace viewer itself
(the 'view trace' page) comes from the Chrome/Chromium project
and is only actively tested on that browser.
*/
package main

```

// === FILE: references/go/src/cmd/trace/gen.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"internal/trace"
	"internal/trace/traceviewer"
	"strings"
)

// generator is an interface for generating a JSON trace for the trace viewer
// from a trace. Each method in this interface is a handler for a kind of event
// that is interesting to render in the UI via the JSON trace.
type generator interface {
	// Global parts.
	Sync() // Notifies the generator of an EventSync event.
	StackSample(ctx *traceContext, ev *trace.Event)
	GlobalRange(ctx *traceContext, ev *trace.Event)
	GlobalMetric(ctx *traceContext, ev *trace.Event)

	// Goroutine parts.
	GoroutineLabel(ctx *traceContext, ev *trace.Event)
	GoroutineRange(ctx *traceContext, ev *trace.Event)
	GoroutineTransition(ctx *traceContext, ev *trace.Event)

	// Proc parts.
	ProcRange(ctx *traceContext, ev *trace.Event)
	ProcTransition(ctx *traceContext, ev *trace.Event)

	// User annotations.
	Log(ctx *traceContext, ev *trace.Event)

	// Finish indicates the end of the trace and finalizes generation.
	Finish(ctx *traceContext)
}

// runGenerator produces a trace into ctx by running the generator over the parsed trace.
func runGenerator(ctx *traceContext, g generator, parsed *parsedTrace, opts *genOpts) {
	for i := range parsed.events {
		ev := &parsed.events[i]

		switch ev.Kind() {
		case trace.EventSync:
			g.Sync()
		case trace.EventStackSample:
			g.StackSample(ctx, ev)
		case trace.EventRangeBegin, trace.EventRangeActive, trace.EventRangeEnd:
			r := ev.Range()
			switch r.Scope.Kind {
			case trace.ResourceGoroutine:
				g.GoroutineRange(ctx, ev)
			case trace.ResourceProc:
				g.ProcRange(ctx, ev)
			case trace.ResourceNone:
				g.GlobalRange(ctx, ev)
			}
		case trace.EventMetric:
			g.GlobalMetric(ctx, ev)
		case trace.EventLabel:
			l := ev.Label()
			if l.Resource.Kind == trace.ResourceGoroutine {
				g.GoroutineLabel(ctx, ev)
			}
		case trace.EventStateTransition:
			switch ev.StateTransition().Resource.Kind {
			case trace.ResourceProc:
				g.ProcTransition(ctx, ev)
			case trace.ResourceGoroutine:
				g.GoroutineTransition(ctx, ev)
			}
		case trace.EventLog:
			g.Log(ctx, ev)
		}
	}
	for i, task := range opts.tasks {
		emitTask(ctx, task, i)
		if opts.mode&traceviewer.ModeGoroutineOriented != 0 {
			for _, region := range task.Regions {
				emitRegion(ctx, region)
			}
		}
	}
	g.Finish(ctx)
}

// emitTask emits information about a task into the trace viewer's event stream.
//
// sortIndex sets the order in which this task will appear related to other tasks,
// lowest first.
func emitTask(ctx *traceContext, task *trace.UserTaskSummary, sortIndex int) {
	// Collect information about the task.
	var startStack, endStack trace.Stack
	var startG, endG trace.GoID
	startTime, endTime := ctx.startTime, ctx.endTime
	if task.Start != nil {
		startStack = task.Start.Stack()
		startG = task.Start.Goroutine()
		startTime = task.Start.Time()
	}
	if task.End != nil {
		endStack = task.End.Stack()
		endG = task.End.Goroutine()
		endTime = task.End.Time()
	}
	arg := struct {
		ID     uint64 `json:"id"`
		StartG uint64 `json:"start_g,omitempty"`
		EndG   uint64 `json:"end_g,omitempty"`
	}{
		ID:     uint64(task.ID),
		StartG: uint64(startG),
		EndG:   uint64(endG),
	}

	// Emit the task slice and notify the emitter of the task.
	ctx.Task(uint64(task.ID), fmt.Sprintf("T%d %s", task.ID, task.Name), sortIndex)
	ctx.TaskSlice(traceviewer.SliceEvent{
		Name:     task.Name,
		Ts:       ctx.elapsed(startTime),
		Dur:      endTime.Sub(startTime),
		Resource: uint64(task.ID),
		Stack:    ctx.Stack(viewerFrames(startStack)),
		EndStack: ctx.Stack(viewerFrames(endStack)),
		Arg:      arg,
	})
	// Emit an arrow from the parent to the child.
	if task.Parent != nil && task.Start != nil && task.Start.Kind() == trace.EventTaskBegin {
		ctx.TaskArrow(traceviewer.ArrowEvent{
			Name:         "newTask",
			Start:        ctx.elapsed(task.Start.Time()),
			End:          ctx.elapsed(task.Start.Time()),
			FromResource: uint64(task.Parent.ID),
			ToResource:   uint64(task.ID),
			FromStack:    ctx.Stack(viewerFrames(task.Start.Stack())),
		})
	}
}

// emitRegion emits goroutine-based slice events to the UI. The caller
// must be emitting for a goroutine-oriented trace.
//
// TODO(mknyszek): Make regions part of the regular generator loop and
// treat them like ranges so that we can emit regions in traces oriented
// by proc or thread.
func emitRegion(ctx *traceContext, region *trace.UserRegionSummary) {
	if region.Name == "" {
		return
	}
	// Collect information about the region.
	var startStack, endStack trace.Stack
	goroutine := trace.NoGoroutine
	startTime, endTime := ctx.startTime, ctx.endTime
	if region.Start != nil {
		startStack = region.Start.Stack()
		startTime = region.Start.Time()
		goroutine = region.Start.Goroutine()
	}
	if region.End != nil {
		endStack = region.End.Stack()
		endTime = region.End.Time()
		goroutine = region.End.Goroutine()
	}
	if goroutine == trace.NoGoroutine {
		return
	}
	arg := struct {
		TaskID uint64 `json:"taskid"`
	}{
		TaskID: uint64(region.TaskID),
	}
	ctx.AsyncSlice(traceviewer.AsyncSliceEvent{
		SliceEvent: traceviewer.SliceEvent{
			Name:     region.Name,
			Ts:       ctx.elapsed(startTime),
			Dur:      endTime.Sub(startTime),
			Resource: uint64(goroutine),
			Stack:    ctx.Stack(viewerFrames(startStack)),
			EndStack: ctx.Stack(viewerFrames(endStack)),
			Arg:      arg,
		},
		Category:       "Region",
		Scope:          fmt.Sprintf("%x", region.TaskID),
		TaskColorIndex: uint64(region.TaskID),
	})
}

// Building blocks for generators.

// stackSampleGenerator implements a generic handler for stack sample events.
// The provided resource is the resource the stack sample should count against.
type stackSampleGenerator[R resource] struct {
	// getResource is a function to extract a resource ID from a stack sample event.
	getResource func(*trace.Event) R
}

// StackSample implements a stack sample event handler. It expects ev to be one such event.
func (g *stackSampleGenerator[R]) StackSample(ctx *traceContext, ev *trace.Event) {
	id := g.getResource(ev)
	if id == R(noResource) {
		// We have nowhere to put this in the UI.
		return
	}
	ctx.Instant(traceviewer.InstantEvent{
		Name:     "CPU profile sample",
		Ts:       ctx.elapsed(ev.Time()),
		Resource: uint64(id),
		Stack:    ctx.Stack(viewerFrames(ev.Stack())),
	})
}

// globalRangeGenerator implements a generic handler for EventRange* events that pertain
// to trace.ResourceNone (the global scope).
type globalRangeGenerator struct {
	ranges   map[string]activeRange
	seenSync int
}

// Sync notifies the generator of an EventSync event.
func (g *globalRangeGenerator) Sync() {
	g.seenSync++
}

// GlobalRange implements a handler for EventRange* events whose Scope.Kind is ResourceNone.
// It expects ev to be one such event.
func (g *globalRangeGenerator) GlobalRange(ctx *traceContext, ev *trace.Event) {
	if g.ranges == nil {
		g.ranges = make(map[string]activeRange)
	}
	r := ev.Range()
	switch ev.Kind() {
	case trace.EventRangeBegin:
		g.ranges[r.Name] = activeRange{ev.Time(), ev.Stack()}
	case trace.EventRangeActive:
		// If we've seen at least 2 Sync events (indicating that we're in at least the second
		// generation), then Active events are always redundant.
		if g.seenSync < 2 {
			// Otherwise, they extend back to the start of the trace.
			g.ranges[r.Name] = activeRange{ctx.startTime, ev.Stack()}
		}
	case trace.EventRangeEnd:
		// Only emit GC events, because we have nowhere to
		// put other events.
		ar := g.ranges[r.Name]
		if strings.Contains(r.Name, "GC") {
			ctx.Slice(traceviewer.SliceEvent{
				Name:     r.Name,
				Ts:       ctx.elapsed(ar.time),
				Dur:      ev.Time().Sub(ar.time),
				Resource: traceviewer.GCP,
				Stack:    ctx.Stack(viewerFrames(ar.stack)),
				EndStack: ctx.Stack(viewerFrames(ev.Stack())),
			})
		}
		delete(g.ranges, r.Name)
	}
}

// Finish flushes any outstanding ranges at the end of the trace.
func (g *globalRangeGenerator) Finish(ctx *traceContext) {
	for name, ar := range g.ranges {
		if !strings.Contains(name, "GC") {
			continue
		}
		ctx.Slice(traceviewer.SliceEvent{
			Name:     name,
			Ts:       ctx.elapsed(ar.time),
			Dur:      ctx.endTime.Sub(ar.time),
			Resource: traceviewer.GCP,
			Stack:    ctx.Stack(viewerFrames(ar.stack)),
		})
	}
}

// globalMetricGenerator implements a generic handler for Metric events.
type globalMetricGenerator struct {
}

// GlobalMetric implements an event handler for EventMetric events. ev must be one such event.
func (g *globalMetricGenerator) GlobalMetric(ctx *traceContext, ev *trace.Event) {
	m := ev.Metric()
	switch m.Name {
	case "/memory/classes/heap/objects:bytes":
		ctx.HeapAlloc(ctx.elapsed(ev.Time()), m.Value.Uint64())
	case "/gc/heap/goal:bytes":
		ctx.HeapGoal(ctx.elapsed(ev.Time()), m.Value.Uint64())
	case "/sched/gomaxprocs:threads":
		ctx.Gomaxprocs(m.Value.Uint64())
	}
}

// procRangeGenerator implements a generic handler for EventRange* events whose Scope.Kind is
// ResourceProc.
type procRangeGenerator struct {
	ranges   map[trace.Range]activeRange
	seenSync int
}

// Sync notifies the generator of an EventSync event.
func (g *procRangeGenerator) Sync() {
	g.seenSync++
}

// ProcRange implements a handler for EventRange* events whose Scope.Kind is ResourceProc.
// It expects ev to be one such event.
func (g *procRangeGenerator) ProcRange(ctx *traceContext, ev *trace.Event) {
	if g.ranges == nil {
		g.ranges = make(map[trace.Range]activeRange)
	}
	r := ev.Range()
	switch ev.Kind() {
	case trace.EventRangeBegin:
		g.ranges[r] = activeRange{ev.Time(), ev.Stack()}
	case trace.EventRangeActive:
		// If we've seen at least 2 Sync events (indicating that we're in at least the second
		// generation), then Active events are always redundant.
		if g.seenSync < 2 {
			// Otherwise, they extend back to the start of the trace.
			g.ranges[r] = activeRange{ctx.startTime, ev.Stack()}
		}
	case trace.EventRangeEnd:
		// Emit proc-based ranges.
		ar := g.ranges[r]
		ctx.Slice(traceviewer.SliceEvent{
			Name:     r.Name,
			Ts:       ctx.elapsed(ar.time),
			Dur:      ev.Time().Sub(ar.time),
			Resource: uint64(r.Scope.Proc()),
			Stack:    ctx.Stack(viewerFrames(ar.stack)),
			EndStack: ctx.Stack(viewerFrames(ev.Stack())),
		})
		delete(g.ranges, r)
	}
}

// Finish flushes any outstanding ranges at the end of the trace.
func (g *procRangeGenerator) Finish(ctx *traceContext) {
	for r, ar := range g.ranges {
		ctx.Slice(traceviewer.SliceEvent{
			Name:     r.Name,
			Ts:       ctx.elapsed(ar.time),
			Dur:      ctx.endTime.Sub(ar.time),
			Resource: uint64(r.Scope.Proc()),
			Stack:    ctx.Stack(viewerFrames(ar.stack)),
		})
	}
}

// activeRange represents an active EventRange* range.
type activeRange struct {
	time  trace.Time
	stack trace.Stack
}

// completedRange represents a completed EventRange* range.
type completedRange struct {
	name       string
	startTime  trace.Time
	endTime    trace.Time
	startStack trace.Stack
	endStack   trace.Stack
	arg        any
}

type logEventGenerator[R resource] struct {
	// getResource is a function to extract a resource ID from a Log event.
	getResource func(*trace.Event) R
}

// Log implements a log event handler. It expects ev to be one such event.
func (g *logEventGenerator[R]) Log(ctx *traceContext, ev *trace.Event) {
	id := g.getResource(ev)
	if id == R(noResource) {
		// We have nowhere to put this in the UI.
		return
	}

	// Construct the name to present.
	log := ev.Log()
	name := log.Message
	if log.Category != "" {
		name = "[" + log.Category + "] " + name
	}

	// Emit an instant event.
	ctx.Instant(traceviewer.InstantEvent{
		Name:     name,
		Ts:       ctx.elapsed(ev.Time()),
		Category: "user event",
		Resource: uint64(id),
		Stack:    ctx.Stack(viewerFrames(ev.Stack())),
	})
}

```

// === FILE: references/go/src/cmd/trace/goroutinegen.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"internal/trace"
)

var _ generator = &goroutineGenerator{}

type goroutineGenerator struct {
	globalRangeGenerator
	globalMetricGenerator
	stackSampleGenerator[trace.GoID]
	logEventGenerator[trace.GoID]

	gStates map[trace.GoID]*gState[trace.GoID]
	focus   trace.GoID
	filter  map[trace.GoID]struct{}
}

func newGoroutineGenerator(ctx *traceContext, focus trace.GoID, filter map[trace.GoID]struct{}) *goroutineGenerator {
	gg := new(goroutineGenerator)
	rg := func(ev *trace.Event) trace.GoID {
		return ev.Goroutine()
	}
	gg.stackSampleGenerator.getResource = rg
	gg.logEventGenerator.getResource = rg
	gg.gStates = make(map[trace.GoID]*gState[trace.GoID])
	gg.focus = focus
	gg.filter = filter

	// Enable a filter on the emitter.
	if filter != nil {
		ctx.SetResourceFilter(func(resource uint64) bool {
			_, ok := filter[trace.GoID(resource)]
			return ok
		})
	}
	return gg
}

func (g *goroutineGenerator) Sync() {
	g.globalRangeGenerator.Sync()
}

func (g *goroutineGenerator) GoroutineLabel(ctx *traceContext, ev *trace.Event) {
	l := ev.Label()
	g.gStates[l.Resource.Goroutine()].setLabel(l.Label)
}

func (g *goroutineGenerator) GoroutineRange(ctx *traceContext, ev *trace.Event) {
	r := ev.Range()
	switch ev.Kind() {
	case trace.EventRangeBegin:
		g.gStates[r.Scope.Goroutine()].rangeBegin(ev.Time(), r.Name, ev.Stack())
	case trace.EventRangeActive:
		g.gStates[r.Scope.Goroutine()].rangeActive(r.Name)
	case trace.EventRangeEnd:
		gs := g.gStates[r.Scope.Goroutine()]
		gs.rangeEnd(ev.Time(), r.Name, ev.Stack(), ctx)
	}
}

func (g *goroutineGenerator) GoroutineTransition(ctx *traceContext, ev *trace.Event) {
	st := ev.StateTransition()
	goID := st.Resource.Goroutine()

	// If we haven't seen this goroutine before, create a new
	// gState for it.
	gs, ok := g.gStates[goID]
	if !ok {
		gs = newGState[trace.GoID](goID)
		g.gStates[goID] = gs
	}

	// Try to augment the name of the goroutine.
	gs.augmentName(st.Stack)

	// Handle the goroutine state transition.
	from, to := st.Goroutine()
	if from == to {
		// Filter out no-op events.
		return
	}
	if from.Executing() && !to.Executing() {
		if to == trace.GoWaiting {
			// Goroutine started blocking.
			gs.block(ev.Time(), ev.Stack(), st.Reason, ctx)
		} else {
			gs.stop(ev.Time(), ev.Stack(), ctx)
		}
	}
	if !from.Executing() && to.Executing() {
		start := ev.Time()
		if from == trace.GoUndetermined {
			// Back-date the event to the start of the trace.
			start = ctx.startTime
		}
		gs.start(start, goID, ctx)
	}

	if from == trace.GoWaiting {
		// Goroutine unblocked.
		gs.unblock(ev.Time(), ev.Stack(), ev.Goroutine(), ctx)
	}
	if from == trace.GoNotExist && to == trace.GoRunnable {
		// Goroutine was created.
		gs.created(ev.Time(), ev.Goroutine(), ev.Stack())
	}
	if from == trace.GoSyscall && to != trace.GoRunning {
		// Exiting blocked syscall.
		gs.syscallEnd(ev.Time(), true, ctx)
		gs.blockedSyscallEnd(ev.Time(), ev.Stack(), ctx)
	} else if from == trace.GoSyscall {
		// Check if we're exiting a syscall in a non-blocking way.
		gs.syscallEnd(ev.Time(), false, ctx)
	}

	// Handle syscalls.
	if to == trace.GoSyscall {
		start := ev.Time()
		if from == trace.GoUndetermined {
			// Back-date the event to the start of the trace.
			start = ctx.startTime
		}
		// Write down that we've entered a syscall. Note: we might have no G or P here
		// if we're in a cgo callback or this is a transition from GoUndetermined
		// (i.e. the G has been blocked in a syscall).
		gs.syscallBegin(start, goID, ev.Stack())
	}

	// Note down the goroutine transition.
	_, inMarkAssist := gs.activeRanges["GC mark assist"]
	ctx.GoroutineTransition(ctx.elapsed(ev.Time()), viewerGState(from, inMarkAssist), viewerGState(to, inMarkAssist))
}

func (g *goroutineGenerator) ProcRange(ctx *traceContext, ev *trace.Event) {
	// TODO(mknyszek): Extend procRangeGenerator to support rendering proc ranges
	// that overlap with a goroutine's execution.
}

func (g *goroutineGenerator) ProcTransition(ctx *traceContext, ev *trace.Event) {
	// Not needed. All relevant information for goroutines can be derived from goroutine transitions.
}

func (g *goroutineGenerator) Finish(ctx *traceContext) {
	ctx.SetResourceType("G")

	// Finish off global ranges.
	g.globalRangeGenerator.Finish(ctx)

	// Finish off all the goroutine slices.
	for id, gs := range g.gStates {
		gs.finish(ctx)

		// Tell the emitter about the goroutines we want to render.
		ctx.Resource(uint64(id), gs.name())
	}

	// Set the goroutine to focus on.
	if g.focus != trace.NoGoroutine {
		ctx.Focus(uint64(g.focus))
	}
}

```

// === FILE: references/go/src/cmd/trace/goroutines.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Goroutine-related profiles.

package main

import (
	"cmp"
	"fmt"
	"html/template"
	"internal/trace"
	"internal/trace/traceviewer"
	"log"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"
)

// GoroutinesHandlerFunc returns a HandlerFunc that serves list of goroutine groups.
func GoroutinesHandlerFunc(summaries map[trace.GoID]*trace.GoroutineSummary) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// goroutineGroup describes a group of goroutines grouped by name.
		type goroutineGroup struct {
			Name     string        // Start function.
			N        int           // Total number of goroutines in this group.
			ExecTime time.Duration // Total execution time of all goroutines in this group.
		}
		// Accumulate groups by Name.
		groupsByName := make(map[string]goroutineGroup)
		for _, summary := range summaries {
			group := groupsByName[summary.Name]
			group.Name = summary.Name
			group.N++
			group.ExecTime += summary.ExecTime
			groupsByName[summary.Name] = group
		}
		var groups []goroutineGroup
		for _, group := range groupsByName {
			groups = append(groups, group)
		}
		slices.SortFunc(groups, func(a, b goroutineGroup) int {
			return cmp.Compare(b.ExecTime, a.ExecTime)
		})
		w.Header().Set("Content-Type", "text/html;charset=utf-8")
		if err := templGoroutines.Execute(w, groups); err != nil {
			log.Printf("failed to execute template: %v", err)
			return
		}
	}
}

var templGoroutines = template.Must(template.New("").Parse(`
<html>
<style>` + traceviewer.CommonStyle + `
table {
  border-collapse: collapse;
}
td,
th {
  border: 1px solid black;
  padding-left: 8px;
  padding-right: 8px;
  padding-top: 4px;
  padding-bottom: 4px;
}
</style>
<body>
<h1>Goroutines</h1>
Below is a table of all goroutines in the trace grouped by start location and sorted by the total execution time of the group.<br>
<br>
Click a start location to view more details about that group.<br>
<br>
<table>
  <tr>
    <th>Start location</th>
	<th>Count</th>
	<th>Total execution time</th>
  </tr>
{{range $}}
  <tr>
    <td><code><a href="/goroutine?name={{.Name}}">{{or .Name "(Inactive, no stack trace sampled)"}}</a></code></td>
	<td>{{.N}}</td>
	<td>{{.ExecTime}}</td>
  </tr>
{{end}}
</table>
</body>
</html>
`))

// GoroutineHandler creates a handler that serves information about
// goroutines in a particular group.
func GoroutineHandler(summaries map[trace.GoID]*trace.GoroutineSummary) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		goroutineName := r.FormValue("name")

		type goroutine struct {
			*trace.GoroutineSummary
			NonOverlappingStats map[string]time.Duration
			HasRangeTime        bool
		}

		// Collect all the goroutines in the group.
		var (
			goroutines              []goroutine
			name                    string
			totalExecTime, execTime time.Duration
			maxTotalTime            time.Duration
		)
		validNonOverlappingStats := make(map[string]struct{})
		validRangeStats := make(map[string]struct{})
		for _, summary := range summaries {
			totalExecTime += summary.ExecTime

			if summary.Name != goroutineName {
				continue
			}
			nonOverlappingStats := summary.NonOverlappingStats()
			for name := range nonOverlappingStats {
				validNonOverlappingStats[name] = struct{}{}
			}
			var totalRangeTime time.Duration
			for name, dt := range summary.RangeTime {
				validRangeStats[name] = struct{}{}
				totalRangeTime += dt
			}
			goroutines = append(goroutines, goroutine{
				GoroutineSummary:    summary,
				NonOverlappingStats: nonOverlappingStats,
				HasRangeTime:        totalRangeTime != 0,
			})
			name = summary.Name
			execTime += summary.ExecTime
			if maxTotalTime < summary.TotalTime {
				maxTotalTime = summary.TotalTime
			}
		}

		// Compute the percent of total execution time these goroutines represent.
		execTimePercent := ""
		if totalExecTime > 0 {
			execTimePercent = fmt.Sprintf("%.2f%%", float64(execTime)/float64(totalExecTime)*100)
		}

		// Sort.
		sortBy := r.FormValue("sortby")
		if _, ok := validNonOverlappingStats[sortBy]; ok {
			slices.SortFunc(goroutines, func(a, b goroutine) int {
				return cmp.Compare(b.NonOverlappingStats[sortBy], a.NonOverlappingStats[sortBy])
			})
		} else {
			// Sort by total time by default.
			slices.SortFunc(goroutines, func(a, b goroutine) int {
				return cmp.Compare(b.TotalTime, a.TotalTime)
			})
		}

		// Write down all the non-overlapping stats and sort them.
		allNonOverlappingStats := make([]string, 0, len(validNonOverlappingStats))
		for name := range validNonOverlappingStats {
			allNonOverlappingStats = append(allNonOverlappingStats, name)
		}
		slices.SortFunc(allNonOverlappingStats, func(a, b string) int {
			if a == b {
				return 0
			}
			if a == "Execution time" {
				return -1
			}
			if b == "Execution time" {
				return 1
			}
			return cmp.Compare(a, b)
		})

		// Write down all the range stats and sort them.
		allRangeStats := make([]string, 0, len(validRangeStats))
		for name := range validRangeStats {
			allRangeStats = append(allRangeStats, name)
		}
		sort.Strings(allRangeStats)

		err := templGoroutine.Execute(w, struct {
			Name                string
			N                   int
			ExecTimePercent     string
			MaxTotal            time.Duration
			Goroutines          []goroutine
			NonOverlappingStats []string
			RangeStats          []string
		}{
			Name:                name,
			N:                   len(goroutines),
			ExecTimePercent:     execTimePercent,
			MaxTotal:            maxTotalTime,
			Goroutines:          goroutines,
			NonOverlappingStats: allNonOverlappingStats,
			RangeStats:          allRangeStats,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to execute template: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

func stat2Color(statName string) string {
	color := "#636363"
	if strings.HasPrefix(statName, "Block time") {
		color = "#d01c8b"
	}
	switch statName {
	case "Sched wait time":
		color = "#2c7bb6"
	case "Syscall execution time":
		color = "#7b3294"
	case "Execution time":
		color = "#d7191c"
	}
	return color
}

var templGoroutine = template.Must(template.New("").Funcs(template.FuncMap{
	"percent": func(dividend, divisor time.Duration) template.HTML {
		if divisor == 0 {
			return ""
		}
		return template.HTML(fmt.Sprintf("(%.1f%%)", float64(dividend)/float64(divisor)*100))
	},
	"headerStyle": func(statName string) template.HTMLAttr {
		return template.HTMLAttr(fmt.Sprintf("style=\"background-color: %s;\"", stat2Color(statName)))
	},
	"barStyle": func(statName string, dividend, divisor time.Duration) template.HTMLAttr {
		width := "0"
		if divisor != 0 {
			width = fmt.Sprintf("%.2f%%", float64(dividend)/float64(divisor)*100)
		}
		return template.HTMLAttr(fmt.Sprintf("style=\"width: %s; background-color: %s;\"", width, stat2Color(statName)))
	},
}).Parse(`
<!DOCTYPE html>
<title>Goroutines: {{.Name}}</title>
<style>` + traceviewer.CommonStyle + `
th {
  background-color: #050505;
  color: #fff;
}
th.link {
  cursor: pointer;
}
table {
  border-collapse: collapse;
}
td,
th {
  padding-left: 8px;
  padding-right: 8px;
  padding-top: 4px;
  padding-bottom: 4px;
}
.details tr:hover {
  background-color: #f2f2f2;
}
.details td {
  text-align: right;
  border: 1px solid black;
}
.details td.id {
  text-align: left;
}
.stacked-bar-graph {
  width: 300px;
  height: 10px;
  color: #414042;
  white-space: nowrap;
  font-size: 5px;
}
.stacked-bar-graph span {
  display: inline-block;
  width: 100%;
  height: 100%;
  box-sizing: border-box;
  float: left;
  padding: 0;
}
</style>

<script>
function reloadTable(key, value) {
  let params = new URLSearchParams(window.location.search);
  params.set(key, value);
  window.location.search = params.toString();
}
</script>

<h1>Goroutines</h1>

Table of contents
<ul>
	<li><a href="#summary">Summary</a></li>
	<li><a href="#breakdown">Breakdown</a></li>
	<li><a href="#ranges">Special ranges</a></li>
</ul>

<h3 id="summary">Summary</h3>

<table class="summary">
	<tr>
		<td>Goroutine start location:</td>
		<td><code>{{.Name}}</code></td>
	</tr>
	<tr>
		<td>Count:</td>
		<td>{{.N}}</td>
	</tr>
	<tr>
		<td>Execution Time:</td>
		<td>{{.ExecTimePercent}} of total program execution time </td>
	</tr>
	<tr>
		<td>Network wait profile:</td>
		<td> <a href="/io?name={{.Name}}">graph</a> <a href="/io?name={{.Name}}&raw=1" download="io.profile">(download)</a></td>
	</tr>
	<tr>
		<td>Sync block profile:</td>
		<td> <a href="/block?name={{.Name}}">graph</a> <a href="/block?name={{.Name}}&raw=1" download="block.profile">(download)</a></td>
	</tr>
	<tr>
		<td>Syscall profile:</td>
		<td> <a href="/syscall?name={{.Name}}">graph</a> <a href="/syscall?name={{.Name}}&raw=1" download="syscall.profile">(download)</a></td>
		</tr>
	<tr>
		<td>Scheduler wait profile:</td>
		<td> <a href="/sched?name={{.Name}}">graph</a> <a href="/sched?name={{.Name}}&raw=1" download="sched.profile">(download)</a></td>
	</tr>
</table>

<h3 id="breakdown">Breakdown</h3>

The table below breaks down where each goroutine is spent its time during the
traced period.
All of the columns except total time are non-overlapping.
<br>
<br>

<table class="details">
<tr>
<th> Goroutine</th>
<th class="link" onclick="reloadTable('sortby', 'Total time')"> Total</th>
<th></th>
{{range $.NonOverlappingStats}}
<th class="link" onclick="reloadTable('sortby', '{{.}}')" {{headerStyle .}}> {{.}}</th>
{{end}}
</tr>
{{range .Goroutines}}
	<tr>
		<td> <a href="/trace?goid={{.ID}}">{{.ID}}</a> </td>
		<td> {{ .TotalTime.String }} </td>
		<td>
			<div class="stacked-bar-graph">
			{{$Goroutine := .}}
			{{range $.NonOverlappingStats}}
				{{$Time := index $Goroutine.NonOverlappingStats .}}
				{{if $Time}}
					<span {{barStyle . $Time $.MaxTotal}}>&nbsp;</span>
				{{end}}
			{{end}}
			</div>
		</td>
		{{$Goroutine := .}}
		{{range $.NonOverlappingStats}}
			{{$Time := index $Goroutine.NonOverlappingStats .}}
			<td> {{$Time.String}}</td>
		{{end}}
	</tr>
{{end}}
</table>

<h3 id="ranges">Special ranges</h3>

The table below describes how much of the traced period each goroutine spent in
certain special time ranges.
If a goroutine has spent no time in any special time ranges, it is excluded from
the table.
For example, how much time it spent helping the GC. Note that these times do
overlap with the times from the first table.
In general the goroutine may not be executing in these special time ranges.
For example, it may have blocked while trying to help the GC.
This must be taken into account when interpreting the data.
<br>
<br>

<table class="details">
<tr>
<th> Goroutine</th>
<th> Total</th>
{{range $.RangeStats}}
<th {{headerStyle .}}> {{.}}</th>
{{end}}
</tr>
{{range .Goroutines}}
	{{if .HasRangeTime}}
		<tr>
			<td> <a href="/trace?goid={{.ID}}">{{.ID}}</a> </td>
			<td> {{ .TotalTime.String }} </td>
			{{$Goroutine := .}}
			{{range $.RangeStats}}
				{{$Time := index $Goroutine.RangeTime .}}
				<td> {{$Time.String}}</td>
			{{end}}
		</tr>
	{{end}}
{{end}}
</table>
`))

```

// === FILE: references/go/src/cmd/trace/gstate.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"internal/trace"
	"internal/trace/traceviewer"
	"internal/trace/traceviewer/format"
	"strings"
)

// resource is a generic constraint interface for resource IDs.
type resource interface {
	trace.GoID | trace.ProcID | trace.ThreadID
}

// noResource indicates the lack of a resource.
const noResource = -1

// gState represents the trace viewer state of a goroutine in a trace.
//
// The type parameter on this type is the resource which is used to construct
// a timeline of events. e.g. R=ProcID for a proc-oriented view, R=GoID for
// a goroutine-oriented view, etc.
type gState[R resource] struct {
	baseName  string
	named     bool   // Whether baseName has been set.
	label     string // EventLabel extension.
	isSystemG bool

	executing R // The resource this goroutine is executing on. (Could be itself.)

	// lastStopStack is the stack trace at the point of the last
	// call to the stop method. This tends to be a more reliable way
	// of picking up stack traces, since the parser doesn't provide
	// a stack for every state transition event.
	lastStopStack trace.Stack

	// activeRanges is the set of all active ranges on the goroutine.
	activeRanges map[string]activeRange

	// completedRanges is a list of ranges that completed since before the
	// goroutine stopped executing. These are flushed on every stop or block.
	completedRanges []completedRange

	// startRunningTime is the most recent event that caused a goroutine to
	// transition to GoRunning.
	startRunningTime trace.Time

	// startSyscall is the most recent event that caused a goroutine to
	// transition to GoSyscall.
	syscall struct {
		time   trace.Time
		stack  trace.Stack
		active bool
	}

	// startBlockReason is the StateTransition.Reason of the most recent
	// event that caused a goroutine to transition to GoWaiting.
	startBlockReason string

	// startCause is the event that allowed this goroutine to start running.
	// It's used to generate flow events. This is typically something like
	// an unblock event or a goroutine creation event.
	//
	// startCause.resource is the resource on which startCause happened, but is
	// listed separately because the cause may have happened on a resource that
	// isn't R (or perhaps on some abstract nebulous resource, like trace.NetpollP).
	startCause struct {
		time     trace.Time
		name     string
		resource uint64
		stack    trace.Stack
	}
}

// newGState constructs a new goroutine state for the goroutine
// identified by the provided ID.
func newGState[R resource](goID trace.GoID) *gState[R] {
	return &gState[R]{
		baseName:     fmt.Sprintf("G%d", goID),
		executing:    R(noResource),
		activeRanges: make(map[string]activeRange),
	}
}

// augmentName attempts to use stk to augment the name of the goroutine
// with stack information. This stack must be related to the goroutine
// in some way, but it doesn't really matter which stack.
func (gs *gState[R]) augmentName(stk trace.Stack) {
	if gs.named {
		return
	}
	if stk == trace.NoStack {
		return
	}
	name := lastFunc(stk)
	gs.baseName += fmt.Sprintf(" %s", name)
	gs.named = true
	gs.isSystemG = trace.IsSystemGoroutine(name)
}

// setLabel adds an additional label to the goroutine's name.
func (gs *gState[R]) setLabel(label string) {
	gs.label = label
}

// name returns a name for the goroutine.
func (gs *gState[R]) name() string {
	name := gs.baseName
	if gs.label != "" {
		name += " (" + gs.label + ")"
	}
	return name
}

// setStartCause sets the reason a goroutine will be allowed to start soon.
// For example, via unblocking or exiting a blocked syscall.
func (gs *gState[R]) setStartCause(ts trace.Time, name string, resource uint64, stack trace.Stack) {
	gs.startCause.time = ts
	gs.startCause.name = name
	gs.startCause.resource = resource
	gs.startCause.stack = stack
}

// created indicates that this goroutine was just created by the provided creator.
func (gs *gState[R]) created(ts trace.Time, creator R, stack trace.Stack) {
	if creator == R(noResource) {
		return
	}
	gs.setStartCause(ts, "go", uint64(creator), stack)
}

// start indicates that a goroutine has started running on a proc.
func (gs *gState[R]) start(ts trace.Time, resource R, ctx *traceContext) {
	// Set the time for all the active ranges.
	for name := range gs.activeRanges {
		gs.activeRanges[name] = activeRange{ts, trace.NoStack}
	}

	if gs.startCause.name != "" {
		// It has a start cause. Emit a flow event.
		ctx.Arrow(traceviewer.ArrowEvent{
			Name:         gs.startCause.name,
			Start:        ctx.elapsed(gs.startCause.time),
			End:          ctx.elapsed(ts),
			FromResource: gs.startCause.resource,
			ToResource:   uint64(resource),
			FromStack:    ctx.Stack(viewerFrames(gs.startCause.stack)),
		})
		gs.startCause.time = 0
		gs.startCause.name = ""
		gs.startCause.resource = 0
		gs.startCause.stack = trace.NoStack
	}
	gs.executing = resource
	gs.startRunningTime = ts
}

// syscallBegin indicates that the goroutine entered a syscall on a proc.
func (gs *gState[R]) syscallBegin(ts trace.Time, resource R, stack trace.Stack) {
	gs.syscall.time = ts
	gs.syscall.stack = stack
	gs.syscall.active = true
	if gs.executing == R(noResource) {
		gs.executing = resource
		gs.startRunningTime = ts
	}
}

// syscallEnd ends the syscall slice, wherever the syscall is at. This is orthogonal
// to blockedSyscallEnd -- both must be called when a syscall ends and that syscall
// blocked. They're kept separate because syscallEnd indicates the point at which the
// goroutine is no longer executing on the resource (e.g. a proc) whereas blockedSyscallEnd
// is the point at which the goroutine actually exited the syscall regardless of which
// resource that happened on.
func (gs *gState[R]) syscallEnd(ts trace.Time, blocked bool, ctx *traceContext) {
	if !gs.syscall.active {
		return
	}
	blockString := "no"
	if blocked {
		blockString = "yes"
	}
	gs.completedRanges = append(gs.completedRanges, completedRange{
		name:       "syscall",
		startTime:  gs.syscall.time,
		endTime:    ts,
		startStack: gs.syscall.stack,
		arg:        format.BlockedArg{Blocked: blockString},
	})
	gs.syscall.active = false
	gs.syscall.time = 0
	gs.syscall.stack = trace.NoStack
}

// blockedSyscallEnd indicates the point at which the blocked syscall ended. This is distinct
// and orthogonal to syscallEnd; both must be called if the syscall blocked. This sets up an instant
// to emit a flow event from, indicating explicitly that this goroutine was unblocked by the system.
func (gs *gState[R]) blockedSyscallEnd(ts trace.Time, stack trace.Stack, ctx *traceContext) {
	name := "exit blocked syscall"
	gs.setStartCause(ts, name, traceviewer.SyscallP, stack)

	// Emit an syscall exit instant event for the "Syscall" lane.
	ctx.Instant(traceviewer.InstantEvent{
		Name:     name,
		Ts:       ctx.elapsed(ts),
		Resource: traceviewer.SyscallP,
		Stack:    ctx.Stack(viewerFrames(stack)),
	})
}

// unblock indicates that the goroutine gs represents has been unblocked.
func (gs *gState[R]) unblock(ts trace.Time, stack trace.Stack, resource R, ctx *traceContext) {
	name := "unblock"
	viewerResource := uint64(resource)
	if gs.startBlockReason != "" {
		name = fmt.Sprintf("%s (%s)", name, gs.startBlockReason)
	}
	if strings.Contains(gs.startBlockReason, "network") {
		// Attribute the network instant to the nebulous "NetpollP" if
		// resource isn't a thread, because there's a good chance that
		// resource isn't going to be valid in this case.
		//
		// TODO(mknyszek): Handle this invalidness in a more general way.
		if _, ok := any(resource).(trace.ThreadID); !ok {
			// Emit an unblock instant event for the "Network" lane.
			viewerResource = traceviewer.NetpollP
		}
		ctx.Instant(traceviewer.InstantEvent{
			Name:     name,
			Ts:       ctx.elapsed(ts),
			Resource: viewerResource,
			Stack:    ctx.Stack(viewerFrames(stack)),
		})
	}
	gs.startBlockReason = ""
	if viewerResource != 0 {
		gs.setStartCause(ts, name, viewerResource, stack)
	}
}

// block indicates that the goroutine has stopped executing on a proc -- specifically,
// it blocked for some reason.
func (gs *gState[R]) block(ts trace.Time, stack trace.Stack, reason string, ctx *traceContext) {
	gs.startBlockReason = reason
	gs.stop(ts, stack, ctx)
}

// stop indicates that the goroutine has stopped executing on a proc.
func (gs *gState[R]) stop(ts trace.Time, stack trace.Stack, ctx *traceContext) {
	// Emit the execution time slice.
	var stk int
	if gs.lastStopStack != trace.NoStack {
		stk = ctx.Stack(viewerFrames(gs.lastStopStack))
	}
	var endStk int
	if stack != trace.NoStack {
		endStk = ctx.Stack(viewerFrames(stack))
	}
	// Check invariants.
	if gs.startRunningTime == 0 {
		panic("silently broken trace or generator invariant (startRunningTime != 0) not held")
	}
	if gs.executing == R(noResource) {
		panic("non-executing goroutine stopped")
	}
	ctx.Slice(traceviewer.SliceEvent{
		Name:     gs.name(),
		Ts:       ctx.elapsed(gs.startRunningTime),
		Dur:      ts.Sub(gs.startRunningTime),
		Resource: uint64(gs.executing),
		Stack:    stk,
		EndStack: endStk,
	})

	// Flush completed ranges.
	for _, cr := range gs.completedRanges {
		ctx.Slice(traceviewer.SliceEvent{
			Name:     cr.name,
			Ts:       ctx.elapsed(cr.startTime),
			Dur:      cr.endTime.Sub(cr.startTime),
			Resource: uint64(gs.executing),
			Stack:    ctx.Stack(viewerFrames(cr.startStack)),
			EndStack: ctx.Stack(viewerFrames(cr.endStack)),
			Arg:      cr.arg,
		})
	}
	gs.completedRanges = gs.completedRanges[:0]

	// Continue in-progress ranges.
	for name, r := range gs.activeRanges {
		// Check invariant.
		if r.time == 0 {
			panic("silently broken trace or generator invariant (activeRanges time != 0) not held")
		}
		ctx.Slice(traceviewer.SliceEvent{
			Name:     name,
			Ts:       ctx.elapsed(r.time),
			Dur:      ts.Sub(r.time),
			Resource: uint64(gs.executing),
			Stack:    ctx.Stack(viewerFrames(r.stack)),
		})
	}

	// Clear the range info.
	for name := range gs.activeRanges {
		gs.activeRanges[name] = activeRange{0, trace.NoStack}
	}

	gs.startRunningTime = 0
	gs.lastStopStack = stack
	gs.executing = R(noResource)
}

// finalize writes out any in-progress slices as if the goroutine stopped.
// This must only be used once the trace has been fully processed and no
// further events will be processed. This method may leave the gState in
// an inconsistent state.
func (gs *gState[R]) finish(ctx *traceContext) {
	if gs.executing != R(noResource) {
		gs.syscallEnd(ctx.endTime, false, ctx)
		gs.stop(ctx.endTime, trace.NoStack, ctx)
	}
}

// rangeBegin indicates the start of a special range of time.
func (gs *gState[R]) rangeBegin(ts trace.Time, name string, stack trace.Stack) {
	if gs.executing != R(noResource) {
		// If we're executing, start the slice from here.
		gs.activeRanges[name] = activeRange{ts, stack}
	} else {
		// If the goroutine isn't executing, there's no place for
		// us to create a slice from. Wait until it starts executing.
		gs.activeRanges[name] = activeRange{0, stack}
	}
}

// rangeActive indicates that a special range of time has been in progress.
func (gs *gState[R]) rangeActive(name string) {
	if gs.executing != R(noResource) {
		// If we're executing, and the range is active, then start
		// from wherever the goroutine started running from.
		gs.activeRanges[name] = activeRange{gs.startRunningTime, trace.NoStack}
	} else {
		// If the goroutine isn't executing, there's no place for
		// us to create a slice from. Wait until it starts executing.
		gs.activeRanges[name] = activeRange{0, trace.NoStack}
	}
}

// rangeEnd indicates the end of a special range of time.
func (gs *gState[R]) rangeEnd(ts trace.Time, name string, stack trace.Stack, ctx *traceContext) {
	if gs.executing != R(noResource) {
		r := gs.activeRanges[name]
		gs.completedRanges = append(gs.completedRanges, completedRange{
			name:       name,
			startTime:  r.time,
			endTime:    ts,
			startStack: r.stack,
			endStack:   stack,
		})
	}
	delete(gs.activeRanges, name)
}

func lastFunc(s trace.Stack) (fn string) {
	for frame := range s.Frames() {
		fn = frame.Func
	}
	return
}

```

// === FILE: references/go/src/cmd/trace/jsontrace.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"cmp"
	"log"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"

	"internal/trace"
	"internal/trace/traceviewer"
)

func JSONTraceHandler(parsed *parsedTrace) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		opts := defaultGenOpts()

		switch r.FormValue("view") {
		case "thread":
			opts.mode = traceviewer.ModeThreadOriented
		}
		if goids := r.FormValue("goid"); goids != "" {
			// Render trace focused on a particular goroutine.

			id, err := strconv.ParseUint(goids, 10, 64)
			if err != nil {
				log.Printf("failed to parse goid parameter %q: %v", goids, err)
				return
			}
			goid := trace.GoID(id)
			g, ok := parsed.summary.Goroutines[goid]
			if !ok {
				log.Printf("failed to find goroutine %d", goid)
				return
			}
			opts.mode = traceviewer.ModeGoroutineOriented
			if g.StartTime != 0 {
				opts.startTime = g.StartTime.Sub(parsed.startTime())
			} else {
				opts.startTime = 0
			}
			if g.EndTime != 0 {
				opts.endTime = g.EndTime.Sub(parsed.startTime())
			} else { // The goroutine didn't end.
				opts.endTime = parsed.endTime().Sub(parsed.startTime())
			}
			opts.focusGoroutine = goid
			opts.goroutines = trace.RelatedGoroutinesV2(parsed.events, goid)
		} else if taskids := r.FormValue("focustask"); taskids != "" {
			taskid, err := strconv.ParseUint(taskids, 10, 64)
			if err != nil {
				log.Printf("failed to parse focustask parameter %q: %v", taskids, err)
				return
			}
			task, ok := parsed.summary.Tasks[trace.TaskID(taskid)]
			if !ok || (task.Start == nil && task.End == nil) {
				log.Printf("failed to find task with id %d", taskid)
				return
			}
			opts.setTask(parsed, task)
		} else if taskids := r.FormValue("taskid"); taskids != "" {
			taskid, err := strconv.ParseUint(taskids, 10, 64)
			if err != nil {
				log.Printf("failed to parse taskid parameter %q: %v", taskids, err)
				return
			}
			task, ok := parsed.summary.Tasks[trace.TaskID(taskid)]
			if !ok {
				log.Printf("failed to find task with id %d", taskid)
				return
			}
			// This mode is goroutine-oriented.
			opts.mode = traceviewer.ModeGoroutineOriented
			opts.setTask(parsed, task)

			// Pick the goroutine to orient ourselves around by just
			// trying to pick the earliest event in the task that makes
			// any sense. Though, we always want the start if that's there.
			var firstEv *trace.Event
			if task.Start != nil {
				firstEv = task.Start
			} else {
				for _, logEv := range task.Logs {
					if firstEv == nil || logEv.Time() < firstEv.Time() {
						firstEv = logEv
					}
				}
				if task.End != nil && (firstEv == nil || task.End.Time() < firstEv.Time()) {
					firstEv = task.End
				}
			}
			if firstEv == nil || firstEv.Goroutine() == trace.NoGoroutine {
				log.Printf("failed to find task with id %d", taskid)
				return
			}

			// Set the goroutine filtering options.
			goid := firstEv.Goroutine()
			opts.focusGoroutine = goid
			goroutines := make(map[trace.GoID]struct{})
			for _, task := range opts.tasks {
				// Find only directly involved goroutines.
				for id := range task.Goroutines {
					goroutines[id] = struct{}{}
				}
			}
			opts.goroutines = goroutines
		}

		// Parse start and end options. Both or none must be present.
		start := int64(0)
		end := int64(math.MaxInt64)
		if startStr, endStr := r.FormValue("start"), r.FormValue("end"); startStr != "" && endStr != "" {
			var err error
			start, err = strconv.ParseInt(startStr, 10, 64)
			if err != nil {
				log.Printf("failed to parse start parameter %q: %v", startStr, err)
				return
			}

			end, err = strconv.ParseInt(endStr, 10, 64)
			if err != nil {
				log.Printf("failed to parse end parameter %q: %v", endStr, err)
				return
			}
		}

		c := traceviewer.ViewerDataTraceConsumer(w, start, end)
		if err := generateTrace(parsed, opts, c); err != nil {
			log.Printf("failed to generate trace: %v", err)
		}
	})
}

// traceContext is a wrapper around a traceviewer.Emitter with some additional
// information that's useful to most parts of trace viewer JSON emission.
type traceContext struct {
	*traceviewer.Emitter
	startTime trace.Time
	endTime   trace.Time
}

// elapsed returns the elapsed time between the trace time and the start time
// of the trace.
func (ctx *traceContext) elapsed(now trace.Time) time.Duration {
	return now.Sub(ctx.startTime)
}

type genOpts struct {
	mode      traceviewer.Mode
	startTime time.Duration
	endTime   time.Duration

	// Used if mode != 0.
	focusGoroutine trace.GoID
	goroutines     map[trace.GoID]struct{} // Goroutines to be displayed for goroutine-oriented or task-oriented view. goroutines[0] is the main goroutine.
	tasks          []*trace.UserTaskSummary
}

// setTask sets a task to focus on.
func (opts *genOpts) setTask(parsed *parsedTrace, task *trace.UserTaskSummary) {
	opts.mode |= traceviewer.ModeTaskOriented
	if task.Start != nil {
		opts.startTime = task.Start.Time().Sub(parsed.startTime())
	} else { // The task started before the trace did.
		opts.startTime = 0
	}
	if task.End != nil {
		opts.endTime = task.End.Time().Sub(parsed.startTime())
	} else { // The task didn't end.
		opts.endTime = parsed.endTime().Sub(parsed.startTime())
	}
	opts.tasks = task.Descendents()
	slices.SortStableFunc(opts.tasks, func(a, b *trace.UserTaskSummary) int {
		aStart, bStart := parsed.startTime(), parsed.startTime()
		if a.Start != nil {
			aStart = a.Start.Time()
		}
		if b.Start != nil {
			bStart = b.Start.Time()
		}
		if a.Start != b.Start {
			return cmp.Compare(aStart, bStart)
		}
		// Break ties with the end time.
		aEnd, bEnd := parsed.endTime(), parsed.endTime()
		if a.End != nil {
			aEnd = a.End.Time()
		}
		if b.End != nil {
			bEnd = b.End.Time()
		}
		return cmp.Compare(aEnd, bEnd)
	})
}

func defaultGenOpts() *genOpts {
	return &genOpts{
		startTime: time.Duration(0),
		endTime:   time.Duration(math.MaxInt64),
	}
}

func generateTrace(parsed *parsedTrace, opts *genOpts, c traceviewer.TraceConsumer) error {
	ctx := &traceContext{
		Emitter:   traceviewer.NewEmitter(c, opts.startTime, opts.endTime),
		startTime: parsed.events[0].Time(),
		endTime:   parsed.events[len(parsed.events)-1].Time(),
	}
	defer ctx.Flush()

	var g generator
	if opts.mode&traceviewer.ModeGoroutineOriented != 0 {
		g = newGoroutineGenerator(ctx, opts.focusGoroutine, opts.goroutines)
	} else if opts.mode&traceviewer.ModeThreadOriented != 0 {
		g = newThreadGenerator()
	} else {
		g = newProcGenerator()
	}
	runGenerator(ctx, g, parsed, opts)
	return nil
}

```

// === FILE: references/go/src/cmd/trace/main.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"cmd/internal/browser"
	"cmd/internal/telemetry/counter"
	"cmp"
	"flag"
	"fmt"
	"internal/trace"
	"internal/trace/raw"
	"internal/trace/tracev2"
	"internal/trace/traceviewer"
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof" // Required to use pprof
	"net/netip"
	"os"
	"slices"
	"sync/atomic"
	"text/tabwriter"
	"time"
)

const usageMessage = "" +
	`Usage of 'go tool trace':
Given a trace file produced by 'go test':
	go test -trace=trace.out pkg

Open a web browser displaying trace:
	go tool trace [flags] [pkg.test] trace.out

Generate a pprof-like profile from the trace:
    go tool trace -pprof=TYPE [pkg.test] trace.out

[pkg.test] argument is required for traces produced by Go 1.6 and below.
Go 1.7 does not require the binary argument.

Supported profile types are:
    - net: network blocking profile
    - sync: synchronization blocking profile
    - syscall: syscall blocking profile
    - sched: scheduler latency profile

Flags:
	-http=addr: HTTP server listen address (e.g., ':6060')
	-pprof=type: print a pprof-like profile instead
	-d=mode: print debug info and exit (modes: wire, parsed, footprint)

When providing only a port to -http (e.g., ':6060'), the tool listens only on localhost.
To listen on all addresses, explicitly add the unspecified address (e.g., '0.0.0.0:6060').

Note that while the various profiles available when launching
'go tool trace' work on every browser, the trace viewer itself
(the 'view trace' page) comes from the Chrome/Chromium project
and is only actively tested on that browser.
`

var (
	httpFlag  = flag.String("http", "localhost:0", "HTTP server listen address (e.g., ':6060')")
	pprofFlag = flag.String("pprof", "", "print a pprof-like profile instead")
	debugFlag = flag.String("d", "", "print debug info and exit (modes: wire, parsed, footprint)")

	// The binary file name, left here for serveSVGProfile.
	programBinary string
	traceFile     string
)

func main() {
	counter.Open()
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usageMessage)
		os.Exit(2)
	}
	flag.Parse()
	counter.Inc("trace/invocations")
	counter.CountFlags("trace/flag:", *flag.CommandLine)

	// Go 1.7 traces embed symbol info and does not require the binary.
	// But we optionally accept binary as first arg for Go 1.5 traces.
	switch flag.NArg() {
	case 1:
		traceFile = flag.Arg(0)
	case 2:
		programBinary = flag.Arg(0)
		traceFile = flag.Arg(1)
	default:
		flag.Usage()
	}

	tracef, err := os.Open(traceFile)
	if err != nil {
		logAndDie(fmt.Errorf("failed to read trace file: %w", err))
	}
	defer tracef.Close()

	// Get the size of the trace file.
	fi, err := tracef.Stat()
	if err != nil {
		logAndDie(fmt.Errorf("failed to stat trace file: %v", err))
	}
	traceSize := fi.Size()

	// Handle requests for profiles.
	if *pprofFlag != "" {
		parsed, err := parseTrace(tracef, traceSize)
		if err != nil {
			logAndDie(err)
		}
		var f traceviewer.ProfileFunc
		switch *pprofFlag {
		case "net":
			f = pprofByGoroutine(computePprofIO(), parsed)
		case "sync":
			f = pprofByGoroutine(computePprofBlock(), parsed)
		case "syscall":
			f = pprofByGoroutine(computePprofSyscall(), parsed)
		case "sched":
			f = pprofByGoroutine(computePprofSched(), parsed)
		default:
			logAndDie(fmt.Errorf("unknown pprof type %s\n", *pprofFlag))
		}
		records, err := f(&http.Request{})
		if err != nil {
			logAndDie(fmt.Errorf("failed to generate pprof: %v\n", err))
		}
		if err := traceviewer.BuildProfile(records).Write(os.Stdout); err != nil {
			logAndDie(fmt.Errorf("failed to generate pprof: %v\n", err))
		}
		logAndDie(nil)
	}

	// Debug flags.
	if *debugFlag != "" {
		switch *debugFlag {
		case "parsed":
			logAndDie(debugProcessedEvents(tracef))
		case "wire":
			logAndDie(debugRawEvents(tracef))
		case "footprint":
			logAndDie(debugEventsFootprint(tracef))
		default:
			logAndDie(fmt.Errorf("invalid debug mode %s, want one of: parsed, wire, footprint", *debugFlag))
		}
	}

	addr, err := listenAddr(*httpFlag)
	if err != nil {
		logAndDie(fmt.Errorf("malformed -http value %q: %v", *httpFlag, err))
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logAndDie(fmt.Errorf("failed to create server socket: %w", err))
	}

	addr = ln.Addr().String()
	url, simplified, err := addrURL(addr)
	if err != nil {
		logAndDie(fmt.Errorf("failed to compute server URL: %v", err))
	}

	log.Print("Preparing trace for viewer...")
	parsed, err := parseTraceInteractive(tracef, traceSize)
	if err != nil {
		logAndDie(err)
	}
	// N.B. tracef not needed after this point.
	// We might double-close, but that's fine; we ignore the error.
	tracef.Close()

	// Print a nice message for a partial trace.
	if parsed.err != nil {
		log.Printf("Encountered error, but able to proceed. Error: %v", parsed.err)

		lost := parsed.size - parsed.valid
		pct := float64(lost) / float64(parsed.size) * 100
		log.Printf("Lost %.2f%% of the latest trace data due to error (%s of %s)", pct, byteCount(lost), byteCount(parsed.size))
	}

	log.Print("Splitting trace for viewer...")
	ranges, err := splitTrace(parsed)
	if err != nil {
		logAndDie(err)
	}

	if simplified {
		// Warn that the URL below is simplified. i.e., we are actually
		// listening on more addresses than the URL implies.
		log.Printf("Full server listen address: %s", addr)
	}
	// N.B. gopls depends on the format of this log message. See
	// golang.org/x/tools/gopls/internal/debug.startFlightRecorder.
	log.Printf("Opening browser. Trace viewer is listening on %s", url)
	browser.Open(addr)

	mutatorUtil := func(flags trace.UtilFlags) ([][]trace.MutatorUtil, error) {
		return trace.MutatorUtilizationV2(parsed.events, flags), nil
	}

	mux := http.NewServeMux()

	// Main endpoint.
	mux.Handle("/", traceviewer.MainHandler([]traceviewer.View{
		{Type: traceviewer.ViewProc, Ranges: ranges},
		// N.B. Use the same ranges for threads. It takes a long time to compute
		// the split a second time, but the makeup of the events are similar enough
		// that this is still a good split.
		{Type: traceviewer.ViewThread, Ranges: ranges},
	}))

	// Catapult handlers.
	mux.Handle("/trace", traceviewer.TraceHandler())
	mux.Handle("/jsontrace", JSONTraceHandler(parsed))
	mux.Handle("/static/", traceviewer.StaticHandler())

	// Goroutines handlers.
	mux.HandleFunc("/goroutines", GoroutinesHandlerFunc(parsed.summary.Goroutines))
	mux.HandleFunc("/goroutine", GoroutineHandler(parsed.summary.Goroutines))

	// MMU handler.
	mux.HandleFunc("/mmu", traceviewer.MMUHandlerFunc(ranges, mutatorUtil))

	// Basic pprof endpoints.
	mux.HandleFunc("/io", traceviewer.SVGProfileHandlerFunc(pprofByGoroutine(computePprofIO(), parsed)))
	mux.HandleFunc("/block", traceviewer.SVGProfileHandlerFunc(pprofByGoroutine(computePprofBlock(), parsed)))
	mux.HandleFunc("/syscall", traceviewer.SVGProfileHandlerFunc(pprofByGoroutine(computePprofSyscall(), parsed)))
	mux.HandleFunc("/sched", traceviewer.SVGProfileHandlerFunc(pprofByGoroutine(computePprofSched(), parsed)))

	// Region-based pprof endpoints.
	mux.HandleFunc("/regionio", traceviewer.SVGProfileHandlerFunc(pprofByRegion(computePprofIO(), parsed)))
	mux.HandleFunc("/regionblock", traceviewer.SVGProfileHandlerFunc(pprofByRegion(computePprofBlock(), parsed)))
	mux.HandleFunc("/regionsyscall", traceviewer.SVGProfileHandlerFunc(pprofByRegion(computePprofSyscall(), parsed)))
	mux.HandleFunc("/regionsched", traceviewer.SVGProfileHandlerFunc(pprofByRegion(computePprofSched(), parsed)))

	// Region endpoints.
	mux.HandleFunc("/userregions", UserRegionsHandlerFunc(parsed))
	mux.HandleFunc("/userregion", UserRegionHandlerFunc(parsed))

	// Task endpoints.
	mux.HandleFunc("/usertasks", UserTasksHandlerFunc(parsed))
	mux.HandleFunc("/usertask", UserTaskHandlerFunc(parsed))

	err = http.Serve(ln, mux)
	logAndDie(fmt.Errorf("failed to start http server: %w", err))
}

func logAndDie(err error) {
	if err == nil {
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}

// listenAddr returns the address to listen on given the addr address flag.
//
// If addr does not specify a host (e.g., ":8080"), then default to listening
// only on localhost rather than all addresses. To listen on all addresses,
// explicitly set the unspecified address (e.g., "0.0.0.0:8080").
func listenAddr(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	if host == "" {
		host = "localhost"
	}
	return net.JoinHostPort(host, port), nil
}

// addrURL returns an HTTP URL that may be used to connect to addr.
//
// It also returns a bool indicating if the returned URL uses a rewritten address.
func addrURL(addr string) (string, bool, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", false, err
	}

	if host == "" {
		// No host implies unspecified address, so rewrite to
		// localhost, as below.
		//
		// addr should come from a net.Listener and thus always include
		// a host, but handle this just in case.
		host = "localhost"
		return "http://" + net.JoinHostPort(host, port), true, nil
	}

	ipaddr, err := netip.ParseAddr(host)
	if err != nil {
		// Not an IP address, no change required.
		return "http://" + net.JoinHostPort(host, port), false, nil
	}

	if ipaddr.IsUnspecified() {
		// An unspecified address means (e.g., 0.0.0.0) this addr is
		// listening on all addresses. It doesn't make sense to connect
		// to the unspecified address [1], so rewrite to localhost. A
		// connection to localhost with route to the same place.
		//
		// [1] Linux happens to treat connect to the unspecified
		// address as loopback, but other OSes, such as Windows, treat
		// it as an error.
		host = "localhost"
		return "http://" + net.JoinHostPort(host, port), true, nil
	}

	return "http://" + net.JoinHostPort(host, port), false, nil
}

func parseTraceInteractive(tr io.Reader, size int64) (parsed *parsedTrace, err error) {
	done := make(chan struct{})
	cr := countingReader{r: tr}
	go func() {
		parsed, err = parseTrace(&cr, size)
		done <- struct{}{}
	}()
	ticker := time.NewTicker(5 * time.Second)
progressLoop:
	for {
		select {
		case <-ticker.C:
		case <-done:
			ticker.Stop()
			break progressLoop
		}
		progress := cr.bytesRead.Load()
		pct := float64(progress) / float64(size) * 100
		log.Printf("%s of %s (%.1f%%) processed...", byteCount(progress), byteCount(size), pct)
	}
	return
}

type parsedTrace struct {
	events      []trace.Event
	summary     *trace.Summary
	size, valid int64
	err         error
}

func parseTrace(rr io.Reader, size int64) (*parsedTrace, error) {
	// Set up the reader.
	cr := countingReader{r: rr}
	r, err := trace.NewReader(&cr)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace reader: %w", err)
	}

	// Set up state.
	s := trace.NewSummarizer()
	t := new(parsedTrace)
	var validBytes int64
	var validEvents int
	for {
		ev, err := r.ReadEvent()
		if err == io.EOF {
			validBytes = cr.bytesRead.Load()
			validEvents = len(t.events)
			break
		}
		if err != nil {
			t.err = err
			break
		}
		t.events = append(t.events, ev)
		s.Event(&t.events[len(t.events)-1])

		if ev.Kind() == trace.EventSync {
			validBytes = cr.bytesRead.Load()
			validEvents = len(t.events)
		}
	}

	// Check to make sure we got at least one good generation.
	if validEvents == 0 {
		return nil, fmt.Errorf("failed to parse any useful part of the trace: %v", t.err)
	}

	// Finish off the parsedTrace.
	t.summary = s.Finalize()
	t.valid = validBytes
	t.size = size
	t.events = t.events[:validEvents]
	return t, nil
}

func (t *parsedTrace) startTime() trace.Time {
	return t.events[0].Time()
}

func (t *parsedTrace) endTime() trace.Time {
	return t.events[len(t.events)-1].Time()
}

// splitTrace splits the trace into a number of ranges, each resulting in approx 100 MiB of
// json output (the trace viewer can hardly handle more).
func splitTrace(parsed *parsedTrace) ([]traceviewer.Range, error) {
	// TODO(mknyszek): Split traces by generation by doing a quick first pass over the
	// trace to identify all the generation boundaries.
	s, c := traceviewer.SplittingTraceConsumer(100 << 20) // 100 MiB
	if err := generateTrace(parsed, defaultGenOpts(), c); err != nil {
		return nil, err
	}
	return s.Ranges, nil
}

func debugProcessedEvents(trc io.Reader) error {
	tr, err := trace.NewReader(trc)
	if err != nil {
		return err
	}
	for {
		ev, err := tr.ReadEvent()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		fmt.Println(ev.String())
	}
}

func debugRawEvents(trc io.Reader) error {
	rr, err := raw.NewReader(trc)
	if err != nil {
		return err
	}
	for {
		ev, err := rr.ReadEvent()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		fmt.Println(ev.String())
	}
}

func debugEventsFootprint(trc io.Reader) error {
	cr := countingReader{r: trc}
	tr, err := raw.NewReader(&cr)
	if err != nil {
		return err
	}
	type eventStats struct {
		typ   tracev2.EventType
		count int
		bytes int
	}
	var stats [256]eventStats
	for i := range stats {
		stats[i].typ = tracev2.EventType(i)
	}
	eventsRead := 0
	for {
		e, err := tr.ReadEvent()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		s := &stats[e.Ev]
		s.count++
		s.bytes += e.EncodedSize()
		eventsRead++
	}
	slices.SortFunc(stats[:], func(a, b eventStats) int {
		return cmp.Compare(b.bytes, a.bytes)
	})
	specs := tr.Version().Specs()
	w := tabwriter.NewWriter(os.Stdout, 3, 8, 2, ' ', 0)
	fmt.Fprintf(w, "Event\tBytes\t%%\tCount\t%%\n")
	fmt.Fprintf(w, "-\t-\t-\t-\t-\n")
	for i := range stats {
		stat := &stats[i]
		name := ""
		if int(stat.typ) >= len(specs) {
			name = fmt.Sprintf("<unknown (%d)>", stat.typ)
		} else {
			name = specs[stat.typ].Name
		}
		bytesPct := float64(stat.bytes) / float64(cr.bytesRead.Load()) * 100
		countPct := float64(stat.count) / float64(eventsRead) * 100
		fmt.Fprintf(w, "%s\t%d\t%.2f%%\t%d\t%.2f%%\n", name, stat.bytes, bytesPct, stat.count, countPct)
	}
	w.Flush()
	return nil
}

type countingReader struct {
	r         io.Reader
	bytesRead atomic.Int64
}

func (c *countingReader) Read(buf []byte) (n int, err error) {
	n, err = c.r.Read(buf)
	c.bytesRead.Add(int64(n))
	return n, err
}

type byteCount int64

func (b byteCount) String() string {
	var suffix string
	var divisor int64
	switch {
	case b < 1<<10:
		suffix = "B"
		divisor = 1
	case b < 1<<20:
		suffix = "KiB"
		divisor = 1 << 10
	case b < 1<<30:
		suffix = "MiB"
		divisor = 1 << 20
	case b < 1<<40:
		suffix = "GiB"
		divisor = 1 << 30
	}
	if divisor == 1 {
		return fmt.Sprintf("%d %s", b, suffix)
	}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(divisor), suffix)
}

```

// === FILE: references/go/src/cmd/trace/pprof.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Serving of pprof-like profiles.

package main

import (
	"cmp"
	"fmt"
	"internal/trace"
	"internal/trace/traceviewer"
	"net/http"
	"slices"
	"strings"
	"time"
)

func pprofByGoroutine(compute computePprofFunc, t *parsedTrace) traceviewer.ProfileFunc {
	return func(r *http.Request) ([]traceviewer.ProfileRecord, error) {
		name := r.FormValue("name")
		gToIntervals, err := pprofMatchingGoroutines(name, t)
		if err != nil {
			return nil, err
		}
		return compute(gToIntervals, t.events)
	}
}

func pprofByRegion(compute computePprofFunc, t *parsedTrace) traceviewer.ProfileFunc {
	return func(r *http.Request) ([]traceviewer.ProfileRecord, error) {
		filter, err := newRegionFilter(r)
		if err != nil {
			return nil, err
		}
		gToIntervals, err := pprofMatchingRegions(filter, t)
		if err != nil {
			return nil, err
		}
		return compute(gToIntervals, t.events)
	}
}

// pprofMatchingGoroutines returns the ids of goroutines of the matching name and its interval.
// If the id string is empty, returns nil without an error.
func pprofMatchingGoroutines(name string, t *parsedTrace) (map[trace.GoID][]interval, error) {
	res := make(map[trace.GoID][]interval)
	for _, g := range t.summary.Goroutines {
		if name != "" && g.Name != name {
			continue
		}
		endTime := g.EndTime
		if g.EndTime == 0 {
			endTime = t.endTime() // Use the trace end time, since the goroutine is still live then.
		}
		res[g.ID] = []interval{{start: g.StartTime, end: endTime}}
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("failed to find matching goroutines for name: %s", name)
	}
	return res, nil
}

// pprofMatchingRegions returns the time intervals of matching regions
// grouped by the goroutine id. If the filter is nil, returns nil without an error.
func pprofMatchingRegions(filter *regionFilter, t *parsedTrace) (map[trace.GoID][]interval, error) {
	if filter == nil {
		return nil, nil
	}

	gToIntervals := make(map[trace.GoID][]interval)
	for _, g := range t.summary.Goroutines {
		for _, r := range g.Regions {
			if !filter.match(t, r) {
				continue
			}
			gToIntervals[g.ID] = append(gToIntervals[g.ID], regionInterval(t, r))
		}
	}

	for g, intervals := range gToIntervals {
		// In order to remove nested regions and
		// consider only the outermost regions,
		// first, we sort based on the start time
		// and then scan through to select only the outermost regions.
		slices.SortFunc(intervals, func(a, b interval) int {
			if c := cmp.Compare(a.start, b.start); c != 0 {
				return c
			}
			return cmp.Compare(a.end, b.end)
		})
		var lastTimestamp trace.Time
		var n int
		// Select only the outermost regions.
		for _, i := range intervals {
			if lastTimestamp <= i.start {
				intervals[n] = i // new non-overlapping region starts.
				lastTimestamp = i.end
				n++
			}
			// Otherwise, skip because this region overlaps with a previous region.
		}
		gToIntervals[g] = intervals[:n]
	}
	return gToIntervals, nil
}

type computePprofFunc func(gToIntervals map[trace.GoID][]interval, events []trace.Event) ([]traceviewer.ProfileRecord, error)

// computePprofIO returns a computePprofFunc that generates IO pprof-like profile (time spent in
// IO wait, currently only network blocking event).
func computePprofIO() computePprofFunc {
	return makeComputePprofFunc(trace.GoWaiting, func(reason string) bool {
		return reason == "network"
	})
}

// computePprofBlock returns a computePprofFunc that generates blocking pprof-like profile
// (time spent blocked on synchronization primitives).
func computePprofBlock() computePprofFunc {
	return makeComputePprofFunc(trace.GoWaiting, func(reason string) bool {
		return strings.Contains(reason, "chan") || strings.Contains(reason, "sync") || strings.Contains(reason, "select")
	})
}

// computePprofSyscall returns a computePprofFunc that generates a syscall pprof-like
// profile (time spent in syscalls).
func computePprofSyscall() computePprofFunc {
	return makeComputePprofFunc(trace.GoSyscall, func(_ string) bool {
		return true
	})
}

// computePprofSched returns a computePprofFunc that generates a scheduler latency pprof-like profile
// (time between a goroutine become runnable and actually scheduled for execution).
func computePprofSched() computePprofFunc {
	return makeComputePprofFunc(trace.GoRunnable, func(_ string) bool {
		return true
	})
}

// makeComputePprofFunc returns a computePprofFunc that generates a profile of time goroutines spend
// in a particular state for the specified reasons.
func makeComputePprofFunc(state trace.GoState, trackReason func(string) bool) computePprofFunc {
	return func(gToIntervals map[trace.GoID][]interval, events []trace.Event) ([]traceviewer.ProfileRecord, error) {
		stacks := newStackMap()
		tracking := make(map[trace.GoID]*trace.Event)
		for i := range events {
			ev := &events[i]

			// Filter out any non-state-transitions and events without stacks.
			if ev.Kind() != trace.EventStateTransition {
				continue
			}

			// The state transition has to apply to a goroutine.
			st := ev.StateTransition()
			if st.Resource.Kind != trace.ResourceGoroutine {
				continue
			}
			id := st.Resource.Goroutine()
			_, new := st.Goroutine()

			// Check if we're tracking this goroutine.
			startEv := tracking[id]
			if startEv == nil {
				// We're not. Start tracking if the new state
				// matches what we want and the transition is
				// for one of the reasons we care about.
				if new == state && trackReason(st.Reason) {
					tracking[id] = ev
				}
				continue
			}
			// We're tracking this goroutine.
			if new == state {
				// We're tracking this goroutine, but it's just transitioning
				// to the same state (this is a no-ip
				continue
			}
			// The goroutine has transitioned out of the state we care about,
			// so remove it from tracking and record the stack.
			delete(tracking, id)

			overlapping := pprofOverlappingDuration(gToIntervals, id, interval{startEv.Time(), ev.Time()})
			if overlapping > 0 {
				rec := stacks.getOrAdd(startEv.Stack())
				rec.Count++
				rec.Time += overlapping
			}
		}
		return stacks.profile(), nil
	}
}

// pprofOverlappingDuration returns the overlapping duration between
// the time intervals in gToIntervals and the specified event.
// If gToIntervals is nil, this simply returns the event's duration.
func pprofOverlappingDuration(gToIntervals map[trace.GoID][]interval, id trace.GoID, sample interval) time.Duration {
	if gToIntervals == nil { // No filtering.
		return sample.duration()
	}
	intervals := gToIntervals[id]
	if len(intervals) == 0 {
		return 0
	}

	var overlapping time.Duration
	for _, i := range intervals {
		if o := i.overlap(sample); o > 0 {
			overlapping += o
		}
	}
	return overlapping
}

// interval represents a time interval in the trace.
type interval struct {
	start, end trace.Time
}

func (i interval) duration() time.Duration {
	return i.end.Sub(i.start)
}

func (i1 interval) overlap(i2 interval) time.Duration {
	// Assume start1 <= end1 and start2 <= end2
	if i1.end < i2.start || i2.end < i1.start {
		return 0
	}
	if i1.start < i2.start { // choose the later one
		i1.start = i2.start
	}
	if i1.end > i2.end { // choose the earlier one
		i1.end = i2.end
	}
	return i1.duration()
}

// pprofMaxStack is the extent of the deduplication we're willing to do.
//
// Because slices aren't comparable and we want to leverage maps for deduplication,
// we have to choose a fixed constant upper bound on the amount of frames we want
// to support. In practice this is fine because there's a maximum depth to these
// stacks anyway.
const pprofMaxStack = 128

// stackMap is a map of trace.Stack to some value V.
type stackMap struct {
	// stacks contains the full list of stacks in the set, however
	// it is insufficient for deduplication because trace.Stack
	// equality is only optimistic. If two trace.Stacks are equal,
	// then they are guaranteed to be equal in content. If they are
	// not equal, then they might still be equal in content.
	stacks map[trace.Stack]*traceviewer.ProfileRecord

	// pcs is the source-of-truth for deduplication. It is a map of
	// the actual PCs in the stack to a trace.Stack.
	pcs map[[pprofMaxStack]uint64]trace.Stack
}

func newStackMap() *stackMap {
	return &stackMap{
		stacks: make(map[trace.Stack]*traceviewer.ProfileRecord),
		pcs:    make(map[[pprofMaxStack]uint64]trace.Stack),
	}
}

func (m *stackMap) getOrAdd(stack trace.Stack) *traceviewer.ProfileRecord {
	// Fast path: check to see if this exact stack is already in the map.
	if rec, ok := m.stacks[stack]; ok {
		return rec
	}
	// Slow path: the stack may still be in the map.

	// Grab the stack's PCs as the source-of-truth.
	var pcs [pprofMaxStack]uint64
	pcsForStack(stack, &pcs)

	// Check the source-of-truth.
	var rec *traceviewer.ProfileRecord
	if existing, ok := m.pcs[pcs]; ok {
		// In the map.
		rec = m.stacks[existing]
		delete(m.stacks, existing)
	} else {
		// Not in the map.
		rec = new(traceviewer.ProfileRecord)
	}
	// Insert regardless of whether we have a match in m.pcs.
	// Even if we have a match, we want to keep the newest version
	// of that stack, since we're much more likely tos see it again
	// as we iterate through the trace linearly. Simultaneously, we
	// are likely to never see the old stack again.
	m.pcs[pcs] = stack
	m.stacks[stack] = rec
	return rec
}

func (m *stackMap) profile() []traceviewer.ProfileRecord {
	prof := make([]traceviewer.ProfileRecord, 0, len(m.stacks))
	for stack, record := range m.stacks {
		rec := *record
		var i int
		for frame := range stack.Frames() {
			rec.Stack = append(rec.Stack, frame)
			i++
			// Cut this off at pprofMaxStack because that's as far
			// as our deduplication goes.
			if i >= pprofMaxStack {
				break
			}
		}
		prof = append(prof, rec)
	}
	return prof
}

// pcsForStack extracts the first pprofMaxStack PCs from stack into pcs.
func pcsForStack(stack trace.Stack, pcs *[pprofMaxStack]uint64) {
	for i, frame := range slices.Collect(stack.Frames()) {
		if i >= len(pcs) {
			break
		}
		pcs[i] = frame.PC
	}
}

```

// === FILE: references/go/src/cmd/trace/procgen.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"internal/trace"
	"internal/trace/traceviewer"
	"internal/trace/traceviewer/format"
)

var _ generator = &procGenerator{}

type procGenerator struct {
	globalRangeGenerator
	globalMetricGenerator
	procRangeGenerator
	stackSampleGenerator[trace.ProcID]
	logEventGenerator[trace.ProcID]

	gStates   map[trace.GoID]*gState[trace.ProcID]
	inSyscall map[trace.ProcID]*gState[trace.ProcID]
	maxProc   trace.ProcID
}

func newProcGenerator() *procGenerator {
	pg := new(procGenerator)
	rg := func(ev *trace.Event) trace.ProcID {
		return ev.Proc()
	}
	pg.stackSampleGenerator.getResource = rg
	pg.logEventGenerator.getResource = rg
	pg.gStates = make(map[trace.GoID]*gState[trace.ProcID])
	pg.inSyscall = make(map[trace.ProcID]*gState[trace.ProcID])
	return pg
}

func (g *procGenerator) Sync() {
	g.globalRangeGenerator.Sync()
	g.procRangeGenerator.Sync()
}

func (g *procGenerator) GoroutineLabel(ctx *traceContext, ev *trace.Event) {
	l := ev.Label()
	g.gStates[l.Resource.Goroutine()].setLabel(l.Label)
}

func (g *procGenerator) GoroutineRange(ctx *traceContext, ev *trace.Event) {
	r := ev.Range()
	switch ev.Kind() {
	case trace.EventRangeBegin:
		g.gStates[r.Scope.Goroutine()].rangeBegin(ev.Time(), r.Name, ev.Stack())
	case trace.EventRangeActive:
		g.gStates[r.Scope.Goroutine()].rangeActive(r.Name)
	case trace.EventRangeEnd:
		gs := g.gStates[r.Scope.Goroutine()]
		gs.rangeEnd(ev.Time(), r.Name, ev.Stack(), ctx)
	}
}

func (g *procGenerator) GoroutineTransition(ctx *traceContext, ev *trace.Event) {
	st := ev.StateTransition()
	goID := st.Resource.Goroutine()

	// If we haven't seen this goroutine before, create a new
	// gState for it.
	gs, ok := g.gStates[goID]
	if !ok {
		gs = newGState[trace.ProcID](goID)
		g.gStates[goID] = gs
	}
	// If we haven't already named this goroutine, try to name it.
	gs.augmentName(st.Stack)

	// Handle the goroutine state transition.
	from, to := st.Goroutine()
	if from == to {
		// Filter out no-op events.
		return
	}
	if from == trace.GoRunning && !to.Executing() {
		if to == trace.GoWaiting {
			// Goroutine started blocking.
			gs.block(ev.Time(), ev.Stack(), st.Reason, ctx)
		} else {
			gs.stop(ev.Time(), ev.Stack(), ctx)
		}
	}
	if !from.Executing() && to == trace.GoRunning {
		start := ev.Time()
		if from == trace.GoUndetermined {
			// Back-date the event to the start of the trace.
			start = ctx.startTime
		}
		gs.start(start, ev.Proc(), ctx)
	}

	if from == trace.GoWaiting {
		// Goroutine was unblocked.
		gs.unblock(ev.Time(), ev.Stack(), ev.Proc(), ctx)
	}
	if from == trace.GoNotExist && to == trace.GoRunnable {
		// Goroutine was created.
		gs.created(ev.Time(), ev.Proc(), ev.Stack())
	}
	if from == trace.GoSyscall && to != trace.GoRunning {
		// Goroutine exited a blocked syscall.
		gs.blockedSyscallEnd(ev.Time(), ev.Stack(), ctx)
	}

	// Handle syscalls.
	if to == trace.GoSyscall && ev.Proc() != trace.NoProc {
		start := ev.Time()
		if from == trace.GoUndetermined {
			// Back-date the event to the start of the trace.
			start = ctx.startTime
		}
		// Write down that we've entered a syscall. Note: we might have no P here
		// if we're in a cgo callback or this is a transition from GoUndetermined
		// (i.e. the G has been blocked in a syscall).
		gs.syscallBegin(start, ev.Proc(), ev.Stack())
		g.inSyscall[ev.Proc()] = gs
	}
	// Check if we're exiting a non-blocking syscall.
	_, didNotBlock := g.inSyscall[ev.Proc()]
	if from == trace.GoSyscall && didNotBlock {
		gs.syscallEnd(ev.Time(), false, ctx)
		delete(g.inSyscall, ev.Proc())
	}

	// Note down the goroutine transition.
	_, inMarkAssist := gs.activeRanges["GC mark assist"]
	ctx.GoroutineTransition(ctx.elapsed(ev.Time()), viewerGState(from, inMarkAssist), viewerGState(to, inMarkAssist))
}

func (g *procGenerator) ProcTransition(ctx *traceContext, ev *trace.Event) {
	st := ev.StateTransition()
	proc := st.Resource.Proc()

	g.maxProc = max(g.maxProc, proc)
	viewerEv := traceviewer.InstantEvent{
		Resource: uint64(proc),
		Stack:    ctx.Stack(viewerFrames(ev.Stack())),

		// Annotate with the thread and proc. The proc is redundant, but this is to
		// stay consistent with the thread view, where it's useful information.
		Arg: format.SchedCtxArg{
			ProcID:   uint64(st.Resource.Proc()),
			ThreadID: uint64(ev.Thread()),
		},
	}

	from, to := st.Proc()
	if from == to {
		// Filter out no-op events.
		return
	}
	if to.Executing() {
		start := ev.Time()
		if from == trace.ProcUndetermined {
			start = ctx.startTime
		}
		viewerEv.Name = "proc start"
		viewerEv.Ts = ctx.elapsed(start)
		ctx.IncThreadStateCount(ctx.elapsed(start), traceviewer.ThreadStateRunning, 1)
	}
	if from.Executing() {
		start := ev.Time()
		viewerEv.Name = "proc stop"
		viewerEv.Ts = ctx.elapsed(start)
		ctx.IncThreadStateCount(ctx.elapsed(start), traceviewer.ThreadStateRunning, -1)

		// Check if this proc was in a syscall before it stopped.
		// This means the syscall blocked. We need to emit it to the
		// viewer at this point because we only display the time the
		// syscall occupied a P when the viewer is in per-P mode.
		//
		// TODO(mknyszek): We could do better in a per-M mode because
		// all events have to happen on *some* thread, and in v2 traces
		// we know what that thread is.
		gs, ok := g.inSyscall[proc]
		if ok {
			// Emit syscall slice for blocked syscall.
			gs.syscallEnd(start, true, ctx)
			gs.stop(start, ev.Stack(), ctx)
			delete(g.inSyscall, proc)
		}
	}
	// TODO(mknyszek): Consider modeling procs differently and have them be
	// transition to and from NotExist when GOMAXPROCS changes. We can emit
	// events for this to clearly delineate GOMAXPROCS changes.

	if viewerEv.Name != "" {
		ctx.Instant(viewerEv)
	}
}

func (g *procGenerator) Finish(ctx *traceContext) {
	ctx.SetResourceType("PROCS")

	// Finish off ranges first. It doesn't really matter for the global ranges,
	// but the proc ranges need to either be a subset of a goroutine slice or
	// their own slice entirely. If the former, it needs to end first.
	g.procRangeGenerator.Finish(ctx)
	g.globalRangeGenerator.Finish(ctx)

	// Finish off all the goroutine slices.
	for _, gs := range g.gStates {
		gs.finish(ctx)
	}

	// Name all the procs to the emitter.
	for i := uint64(0); i <= uint64(g.maxProc); i++ {
		ctx.Resource(i, fmt.Sprintf("Proc %v", i))
	}
}

```

// === FILE: references/go/src/cmd/trace/regions.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"cmp"
	"fmt"
	"html/template"
	"internal/trace"
	"internal/trace/traceviewer"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// UserRegionsHandlerFunc returns a HandlerFunc that reports all regions found in the trace.
func UserRegionsHandlerFunc(t *parsedTrace) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Summarize all the regions.
		summary := make(map[regionFingerprint]regionStats)
		for _, g := range t.summary.Goroutines {
			for _, r := range g.Regions {
				id := fingerprintRegion(r)
				stats, ok := summary[id]
				if !ok {
					stats.regionFingerprint = id
				}
				stats.add(t, r)
				summary[id] = stats
			}
		}
		// Sort regions by PC and name.
		userRegions := make([]regionStats, 0, len(summary))
		for _, stats := range summary {
			userRegions = append(userRegions, stats)
		}
		slices.SortFunc(userRegions, func(a, b regionStats) int {
			if c := cmp.Compare(a.Type, b.Type); c != 0 {
				return c
			}
			return cmp.Compare(a.Frame.PC, b.Frame.PC)
		})
		// Emit table.
		err := templUserRegionTypes.Execute(w, userRegions)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to execute template: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

// regionFingerprint is a way to categorize regions that goes just one step beyond the region's Type
// by including the top stack frame.
type regionFingerprint struct {
	Frame trace.StackFrame
	Type  string
}

func fingerprintRegion(r *trace.UserRegionSummary) regionFingerprint {
	return regionFingerprint{
		Frame: regionTopStackFrame(r),
		Type:  r.Name,
	}
}

func regionTopStackFrame(r *trace.UserRegionSummary) trace.StackFrame {
	var frame trace.StackFrame
	if r.Start != nil && r.Start.Stack() != trace.NoStack {
		for f := range r.Start.Stack().Frames() {
			frame = f
		}
	}
	return frame
}

type regionStats struct {
	regionFingerprint
	Histogram traceviewer.TimeHistogram
}

func (s *regionStats) UserRegionURL() func(min, max time.Duration) string {
	return func(min, max time.Duration) string {
		return fmt.Sprintf("/userregion?type=%s&pc=%x&latmin=%v&latmax=%v", template.URLQueryEscaper(s.Type), s.Frame.PC, template.URLQueryEscaper(min), template.URLQueryEscaper(max))
	}
}

func (s *regionStats) add(t *parsedTrace, region *trace.UserRegionSummary) {
	s.Histogram.Add(regionInterval(t, region).duration())
}

var templUserRegionTypes = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<title>Regions</title>
<style>` + traceviewer.CommonStyle + `
.histoTime {
  width: 20%;
  white-space:nowrap;
}
th {
  background-color: #050505;
  color: #fff;
}
table {
  border-collapse: collapse;
}
td,
th {
  padding-left: 8px;
  padding-right: 8px;
  padding-top: 4px;
  padding-bottom: 4px;
}
</style>
<body>
<h1>Regions</h1>

Below is a table containing a summary of all the user-defined regions in the trace.
Regions are grouped by the region type and the point at which the region started.
The rightmost column of the table contains a latency histogram for each region group.
Note that this histogram only counts regions that began and ended within the traced
period.
However, the "Count" column includes all regions, including those that only started
or ended during the traced period.
Regions that were active through the trace period were not recorded, and so are not
accounted for at all.
Click on the links to explore a breakdown of time spent for each region by goroutine
and user-defined task.
<br>
<br>

<table border="1" sortable="1">
<tr>
<th>Region type</th>
<th>Count</th>
<th>Duration distribution (complete tasks)</th>
</tr>
{{range $}}
  <tr>
    <td><pre>{{printf "%q" .Type}}<br>{{.Frame.Func}} @ {{printf "0x%x" .Frame.PC}}<br>{{.Frame.File}}:{{.Frame.Line}}</pre></td>
    <td><a href="/userregion?type={{.Type}}&pc={{.Frame.PC | printf "%x"}}">{{.Histogram.Count}}</a></td>
    <td>{{.Histogram.ToHTML (.UserRegionURL)}}</td>
  </tr>
{{end}}
</table>
</body>
</html>
`))

// UserRegionHandlerFunc returns a HandlerFunc that presents the details of the selected regions.
func UserRegionHandlerFunc(t *parsedTrace) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Construct the filter from the request.
		filter, err := newRegionFilter(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Collect all the regions with their goroutines.
		type region struct {
			*trace.UserRegionSummary
			Goroutine           trace.GoID
			NonOverlappingStats map[string]time.Duration
			HasRangeTime        bool
		}
		var regions []region
		var maxTotal time.Duration
		validNonOverlappingStats := make(map[string]struct{})
		validRangeStats := make(map[string]struct{})
		for _, g := range t.summary.Goroutines {
			for _, r := range g.Regions {
				if !filter.match(t, r) {
					continue
				}
				nonOverlappingStats := r.NonOverlappingStats()
				for name := range nonOverlappingStats {
					validNonOverlappingStats[name] = struct{}{}
				}
				var totalRangeTime time.Duration
				for name, dt := range r.RangeTime {
					validRangeStats[name] = struct{}{}
					totalRangeTime += dt
				}
				regions = append(regions, region{
					UserRegionSummary:   r,
					Goroutine:           g.ID,
					NonOverlappingStats: nonOverlappingStats,
					HasRangeTime:        totalRangeTime != 0,
				})
				if maxTotal < r.TotalTime {
					maxTotal = r.TotalTime
				}
			}
		}

		// Sort.
		sortBy := r.FormValue("sortby")
		if _, ok := validNonOverlappingStats[sortBy]; ok {
			slices.SortFunc(regions, func(a, b region) int {
				return cmp.Compare(b.NonOverlappingStats[sortBy], a.NonOverlappingStats[sortBy])
			})
		} else {
			// Sort by total time by default.
			slices.SortFunc(regions, func(a, b region) int {
				return cmp.Compare(b.TotalTime, a.TotalTime)
			})
		}

		// Write down all the non-overlapping stats and sort them.
		allNonOverlappingStats := make([]string, 0, len(validNonOverlappingStats))
		for name := range validNonOverlappingStats {
			allNonOverlappingStats = append(allNonOverlappingStats, name)
		}
		slices.SortFunc(allNonOverlappingStats, func(a, b string) int {
			if a == b {
				return 0
			}
			if a == "Execution time" {
				return -1
			}
			if b == "Execution time" {
				return 1
			}
			return cmp.Compare(a, b)
		})

		// Write down all the range stats and sort them.
		allRangeStats := make([]string, 0, len(validRangeStats))
		for name := range validRangeStats {
			allRangeStats = append(allRangeStats, name)
		}
		sort.Strings(allRangeStats)

		err = templUserRegionType.Execute(w, struct {
			MaxTotal            time.Duration
			Regions             []region
			Name                string
			Filter              *regionFilter
			NonOverlappingStats []string
			RangeStats          []string
		}{
			MaxTotal:            maxTotal,
			Regions:             regions,
			Name:                filter.name,
			Filter:              filter,
			NonOverlappingStats: allNonOverlappingStats,
			RangeStats:          allRangeStats,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to execute template: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

var templUserRegionType = template.Must(template.New("").Funcs(template.FuncMap{
	"headerStyle": func(statName string) template.HTMLAttr {
		return template.HTMLAttr(fmt.Sprintf("style=\"background-color: %s;\"", stat2Color(statName)))
	},
	"barStyle": func(statName string, dividend, divisor time.Duration) template.HTMLAttr {
		width := "0"
		if divisor != 0 {
			width = fmt.Sprintf("%.2f%%", float64(dividend)/float64(divisor)*100)
		}
		return template.HTMLAttr(fmt.Sprintf("style=\"width: %s; background-color: %s;\"", width, stat2Color(statName)))
	},
	"filterParams": func(f *regionFilter) template.URL {
		return template.URL(f.params.Encode())
	},
}).Parse(`
<!DOCTYPE html>
<title>Regions: {{.Name}}</title>
<style>` + traceviewer.CommonStyle + `
th {
  background-color: #050505;
  color: #fff;
}
th.link {
  cursor: pointer;
}
table {
  border-collapse: collapse;
}
td,
th {
  padding-left: 8px;
  padding-right: 8px;
  padding-top: 4px;
  padding-bottom: 4px;
}
.details tr:hover {
  background-color: #f2f2f2;
}
.details td {
  text-align: right;
  border: 1px solid #000;
}
.details td.id {
  text-align: left;
}
.stacked-bar-graph {
  width: 300px;
  height: 10px;
  color: #414042;
  white-space: nowrap;
  font-size: 5px;
}
.stacked-bar-graph span {
  display: inline-block;
  width: 100%;
  height: 100%;
  box-sizing: border-box;
  float: left;
  padding: 0;
}
</style>

<script>
function reloadTable(key, value) {
  let params = new URLSearchParams(window.location.search);
  params.set(key, value);
  window.location.search = params.toString();
}
</script>

<h1>Regions: {{.Name}}</h1>

Table of contents
<ul>
	<li><a href="#summary">Summary</a></li>
	<li><a href="#breakdown">Breakdown</a></li>
	<li><a href="#ranges">Special ranges</a></li>
</ul>

<h3 id="summary">Summary</h3>

{{ with $p := filterParams .Filter}}
<table class="summary">
	<tr>
		<td>Network wait profile:</td>
		<td> <a href="/regionio?{{$p}}">graph</a> <a href="/regionio?{{$p}}&raw=1" download="io.profile">(download)</a></td>
	</tr>
	<tr>
		<td>Sync block profile:</td>
		<td> <a href="/regionblock?{{$p}}">graph</a> <a href="/regionblock?{{$p}}&raw=1" download="block.profile">(download)</a></td>
	</tr>
	<tr>
		<td>Syscall profile:</td>
		<td> <a href="/regionsyscall?{{$p}}">graph</a> <a href="/regionsyscall?{{$p}}&raw=1" download="syscall.profile">(download)</a></td>
	</tr>
	<tr>
		<td>Scheduler wait profile:</td>
		<td> <a href="/regionsched?{{$p}}">graph</a> <a href="/regionsched?{{$p}}&raw=1" download="sched.profile">(download)</a></td>
	</tr>
</table>
{{ end }}

<h3 id="breakdown">Breakdown</h3>

The table below breaks down where each goroutine is spent its time during the
traced period.
All of the columns except total time are non-overlapping.
<br>
<br>

<table class="details">
<tr>
<th> Goroutine </th>
<th> Task </th>
<th class="link" onclick="reloadTable('sortby', 'Total time')"> Total</th>
<th></th>
{{range $.NonOverlappingStats}}
<th class="link" onclick="reloadTable('sortby', '{{.}}')" {{headerStyle .}}> {{.}}</th>
{{end}}
</tr>
{{range .Regions}}
	<tr>
		<td> <a href="/trace?goid={{.Goroutine}}">{{.Goroutine}}</a> </td>
		<td> {{if .TaskID}}<a href="/trace?focustask={{.TaskID}}">{{.TaskID}}</a>{{end}} </td>
		<td> {{ .TotalTime.String }} </td>
		<td>
			<div class="stacked-bar-graph">
			{{$Region := .}}
			{{range $.NonOverlappingStats}}
				{{$Time := index $Region.NonOverlappingStats .}}
				{{if $Time}}
					<span {{barStyle . $Time $.MaxTotal}}>&nbsp;</span>
				{{end}}
			{{end}}
			</div>
		</td>
		{{$Region := .}}
		{{range $.NonOverlappingStats}}
			{{$Time := index $Region.NonOverlappingStats .}}
			<td> {{$Time.String}}</td>
		{{end}}
	</tr>
{{end}}
</table>

<h3 id="ranges">Special ranges</h3>

The table below describes how much of the traced period each goroutine spent in
certain special time ranges.
If a goroutine has spent no time in any special time ranges, it is excluded from
the table.
For example, how much time it spent helping the GC. Note that these times do
overlap with the times from the first table.
In general the goroutine may not be executing in these special time ranges.
For example, it may have blocked while trying to help the GC.
This must be taken into account when interpreting the data.
<br>
<br>

<table class="details">
<tr>
<th> Goroutine</th>
<th> Task </th>
<th> Total</th>
{{range $.RangeStats}}
<th {{headerStyle .}}> {{.}}</th>
{{end}}
</tr>
{{range .Regions}}
	{{if .HasRangeTime}}
		<tr>
			<td> <a href="/trace?goid={{.Goroutine}}">{{.Goroutine}}</a> </td>
			<td> {{if .TaskID}}<a href="/trace?focustask={{.TaskID}}">{{.TaskID}}</a>{{end}} </td>
			<td> {{ .TotalTime.String }} </td>
			{{$Region := .}}
			{{range $.RangeStats}}
				{{$Time := index $Region.RangeTime .}}
				<td> {{$Time.String}}</td>
			{{end}}
		</tr>
	{{end}}
{{end}}
</table>
`))

// regionFilter represents a region filter specified by a user of cmd/trace.
type regionFilter struct {
	name   string
	params url.Values
	cond   []func(*parsedTrace, *trace.UserRegionSummary) bool
}

// match returns true if a region, described by its ID and summary, matches
// the filter.
func (f *regionFilter) match(t *parsedTrace, s *trace.UserRegionSummary) bool {
	for _, c := range f.cond {
		if !c(t, s) {
			return false
		}
	}
	return true
}

// newRegionFilter creates a new region filter from URL query variables.
func newRegionFilter(r *http.Request) (*regionFilter, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	var name []string
	var conditions []func(*parsedTrace, *trace.UserRegionSummary) bool
	filterParams := make(url.Values)

	param := r.Form
	if typ, ok := param["type"]; ok && len(typ) > 0 {
		name = append(name, fmt.Sprintf("%q", typ[0]))
		conditions = append(conditions, func(_ *parsedTrace, r *trace.UserRegionSummary) bool {
			return r.Name == typ[0]
		})
		filterParams.Add("type", typ[0])
	}
	if pc, err := strconv.ParseUint(r.FormValue("pc"), 16, 64); err == nil {
		encPC := fmt.Sprintf("0x%x", pc)
		name = append(name, "@ "+encPC)
		conditions = append(conditions, func(_ *parsedTrace, r *trace.UserRegionSummary) bool {
			return regionTopStackFrame(r).PC == pc
		})
		filterParams.Add("pc", encPC)
	}

	if lat, err := time.ParseDuration(r.FormValue("latmin")); err == nil {
		name = append(name, fmt.Sprintf("(latency >= %s)", lat))
		conditions = append(conditions, func(t *parsedTrace, r *trace.UserRegionSummary) bool {
			return regionInterval(t, r).duration() >= lat
		})
		filterParams.Add("latmin", lat.String())
	}
	if lat, err := time.ParseDuration(r.FormValue("latmax")); err == nil {
		name = append(name, fmt.Sprintf("(latency <= %s)", lat))
		conditions = append(conditions, func(t *parsedTrace, r *trace.UserRegionSummary) bool {
			return regionInterval(t, r).duration() <= lat
		})
		filterParams.Add("latmax", lat.String())
	}

	return &regionFilter{
		name:   strings.Join(name, " "),
		cond:   conditions,
		params: filterParams,
	}, nil
}

func regionInterval(t *parsedTrace, s *trace.UserRegionSummary) interval {
	var i interval
	if s.Start != nil {
		i.start = s.Start.Time()
	} else {
		i.start = t.startTime()
	}
	if s.End != nil {
		i.end = s.End.Time()
	} else {
		i.end = t.endTime()
	}
	return i
}

```

// === FILE: references/go/src/cmd/trace/tasks.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"cmp"
	"fmt"
	"html/template"
	"internal/trace"
	"internal/trace/traceviewer"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"
)

// UserTasksHandlerFunc returns a HandlerFunc that reports all tasks found in the trace.
func UserTasksHandlerFunc(t *parsedTrace) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tasks := t.summary.Tasks

		// Summarize groups of tasks with the same name.
		summary := make(map[string]taskStats)
		for _, task := range tasks {
			stats, ok := summary[task.Name]
			if !ok {
				stats.Type = task.Name
			}
			stats.add(task)
			summary[task.Name] = stats
		}

		// Sort tasks by type.
		userTasks := make([]taskStats, 0, len(summary))
		for _, stats := range summary {
			userTasks = append(userTasks, stats)
		}
		slices.SortFunc(userTasks, func(a, b taskStats) int {
			return cmp.Compare(a.Type, b.Type)
		})

		// Emit table.
		err := templUserTaskTypes.Execute(w, userTasks)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to execute template: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

type taskStats struct {
	Type      string
	Count     int                       // Complete + incomplete tasks
	Histogram traceviewer.TimeHistogram // Complete tasks only
}

func (s *taskStats) UserTaskURL(complete bool) func(min, max time.Duration) string {
	return func(min, max time.Duration) string {
		return fmt.Sprintf("/usertask?type=%s&complete=%v&latmin=%v&latmax=%v", template.URLQueryEscaper(s.Type), template.URLQueryEscaper(complete), template.URLQueryEscaper(min), template.URLQueryEscaper(max))
	}
}

func (s *taskStats) add(task *trace.UserTaskSummary) {
	s.Count++
	if task.Complete() {
		s.Histogram.Add(task.End.Time().Sub(task.Start.Time()))
	}
}

var templUserTaskTypes = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<title>Tasks</title>
<style>` + traceviewer.CommonStyle + `
.histoTime {
  width: 20%;
  white-space:nowrap;
}
th {
  background-color: #050505;
  color: #fff;
}
table {
  border-collapse: collapse;
}
td,
th {
  padding-left: 8px;
  padding-right: 8px;
  padding-top: 4px;
  padding-bottom: 4px;
}
</style>
<body>
Search log text: <form action="/usertask"><input name="logtext" type="text"><input type="submit"></form><br>
<table border="1" sortable="1">
<tr>
<th>Task type</th>
<th>Count</th>
<th>Duration distribution (complete tasks)</th>
</tr>
{{range $}}
  <tr>
    <td>{{.Type}}</td>
    <td><a href="/usertask?type={{.Type}}">{{.Count}}</a></td>
    <td>{{.Histogram.ToHTML (.UserTaskURL true)}}</td>
  </tr>
{{end}}
</table>
</body>
</html>
`))

// UserTaskHandlerFunc returns a HandlerFunc that presents the details of the selected tasks.
func UserTaskHandlerFunc(t *parsedTrace) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := newTaskFilter(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		type event struct {
			WhenString string
			Elapsed    time.Duration
			Goroutine  trace.GoID
			What       string
			// TODO: include stack trace of creation time
		}
		type task struct {
			WhenString string
			ID         trace.TaskID
			Duration   time.Duration
			Complete   bool
			Events     []event
			Start, End time.Duration // Time since the beginning of the trace
			GCTime     time.Duration
		}
		var tasks []task
		for _, summary := range t.summary.Tasks {
			if !filter.match(t, summary) {
				continue
			}

			// Collect all the events for the task.
			var rawEvents []*trace.Event
			if summary.Start != nil {
				rawEvents = append(rawEvents, summary.Start)
			}
			if summary.End != nil {
				rawEvents = append(rawEvents, summary.End)
			}
			rawEvents = append(rawEvents, summary.Logs...)
			for _, r := range summary.Regions {
				if r.Start != nil {
					rawEvents = append(rawEvents, r.Start)
				}
				if r.End != nil {
					rawEvents = append(rawEvents, r.End)
				}
			}

			// Sort them.
			slices.SortStableFunc(rawEvents, func(a, b *trace.Event) int {
				return cmp.Compare(a.Time(), b.Time())
			})

			// Summarize them.
			var events []event
			last := t.startTime()
			for _, ev := range rawEvents {
				what := describeEvent(ev)
				if what == "" {
					continue
				}
				sinceStart := ev.Time().Sub(t.startTime())
				events = append(events, event{
					WhenString: fmt.Sprintf("%2.9f", sinceStart.Seconds()),
					Elapsed:    ev.Time().Sub(last),
					What:       what,
					Goroutine:  primaryGoroutine(ev),
				})
				last = ev.Time()
			}
			taskSpan := taskInterval(t, summary)
			taskStart := taskSpan.start.Sub(t.startTime())

			// Produce the task summary.
			tasks = append(tasks, task{
				WhenString: fmt.Sprintf("%2.9fs", taskStart.Seconds()),
				Duration:   taskSpan.duration(),
				ID:         summary.ID,
				Complete:   summary.Complete(),
				Events:     events,
				Start:      taskStart,
				End:        taskStart + taskSpan.duration(),
			})
		}
		// Sort the tasks by duration.
		slices.SortFunc(tasks, func(a, b task) int {
			return cmp.Compare(a.Duration, b.Duration)
		})

		// Emit table.
		err = templUserTaskType.Execute(w, struct {
			Name  string
			Tasks []task
		}{
			Name:  filter.name,
			Tasks: tasks,
		})
		if err != nil {
			log.Printf("failed to execute template: %v", err)
			http.Error(w, fmt.Sprintf("failed to execute template: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

var templUserTaskType = template.Must(template.New("userTask").Funcs(template.FuncMap{
	"elapsed":       elapsed,
	"asMillisecond": asMillisecond,
	"trimSpace":     strings.TrimSpace,
}).Parse(`
<!DOCTYPE html>
<title>Tasks: {{.Name}}</title>
<style>` + traceviewer.CommonStyle + `
body {
  font-family: sans-serif;
}
table#req-status td.family {
  padding-right: 2em;
}
table#req-status td.active {
  padding-right: 1em;
}
table#req-status td.empty {
  color: #aaa;
}
table#reqs {
  margin-top: 1em;
  border-collapse: collapse;
}
table#reqs tr.first {
  font-weight: bold;
}
table#reqs td {
  font-family: monospace;
}
table#reqs td.when {
  text-align: right;
  white-space: nowrap;
}
table#reqs td.elapsed {
  padding: 0 0.5em;
  text-align: right;
  white-space: pre;
  width: 10em;
}
address {
  font-size: smaller;
  margin-top: 5em;
}
</style>
<body>

<h2>User Task: {{.Name}}</h2>

Search log text: <form onsubmit="window.location.search+='&logtext='+window.logtextinput.value; return false">
<input name="logtext" id="logtextinput" type="text"><input type="submit">
</form><br>

<table id="reqs">
	<tr>
		<th>When</th>
		<th>Elapsed</th>
		<th>Goroutine</th>
		<th>Events</th>
	</tr>
	{{range $el := $.Tasks}}
	<tr class="first">
		<td class="when">{{$el.WhenString}}</td>
		<td class="elapsed">{{$el.Duration}}</td>
		<td></td>
		<td>
			<a href="/trace?focustask={{$el.ID}}#{{asMillisecond $el.Start}}:{{asMillisecond $el.End}}">Task {{$el.ID}}</a>
			<a href="/trace?taskid={{$el.ID}}#{{asMillisecond $el.Start}}:{{asMillisecond $el.End}}">(goroutine view)</a>
			({{if .Complete}}complete{{else}}incomplete{{end}})
		</td>
	</tr>
	{{range $el.Events}}
	<tr>
		<td class="when">{{.WhenString}}</td>
		<td class="elapsed">{{elapsed .Elapsed}}</td>
		<td class="goid">{{.Goroutine}}</td>
		<td>{{.What}}</td>
	</tr>
	{{end}}
    {{end}}
</body>
</html>
`))

// taskFilter represents a task filter specified by a user of cmd/trace.
type taskFilter struct {
	name string
	cond []func(*parsedTrace, *trace.UserTaskSummary) bool
}

// match returns true if a task, described by its ID and summary, matches
// the filter.
func (f *taskFilter) match(t *parsedTrace, task *trace.UserTaskSummary) bool {
	if t == nil {
		return false
	}
	for _, c := range f.cond {
		if !c(t, task) {
			return false
		}
	}
	return true
}

// newTaskFilter creates a new task filter from URL query variables.
func newTaskFilter(r *http.Request) (*taskFilter, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	var name []string
	var conditions []func(*parsedTrace, *trace.UserTaskSummary) bool

	param := r.Form
	if typ, ok := param["type"]; ok && len(typ) > 0 {
		name = append(name, fmt.Sprintf("%q", typ[0]))
		conditions = append(conditions, func(_ *parsedTrace, task *trace.UserTaskSummary) bool {
			return task.Name == typ[0]
		})
	}
	if complete := r.FormValue("complete"); complete == "1" {
		name = append(name, "complete")
		conditions = append(conditions, func(_ *parsedTrace, task *trace.UserTaskSummary) bool {
			return task.Complete()
		})
	} else if complete == "0" {
		name = append(name, "incomplete")
		conditions = append(conditions, func(_ *parsedTrace, task *trace.UserTaskSummary) bool {
			return !task.Complete()
		})
	}
	if lat, err := time.ParseDuration(r.FormValue("latmin")); err == nil {
		name = append(name, fmt.Sprintf("latency >= %s", lat))
		conditions = append(conditions, func(t *parsedTrace, task *trace.UserTaskSummary) bool {
			return task.Complete() && taskInterval(t, task).duration() >= lat
		})
	}
	if lat, err := time.ParseDuration(r.FormValue("latmax")); err == nil {
		name = append(name, fmt.Sprintf("latency <= %s", lat))
		conditions = append(conditions, func(t *parsedTrace, task *trace.UserTaskSummary) bool {
			return task.Complete() && taskInterval(t, task).duration() <= lat
		})
	}
	if text := r.FormValue("logtext"); text != "" {
		name = append(name, fmt.Sprintf("log contains %q", text))
		conditions = append(conditions, func(_ *parsedTrace, task *trace.UserTaskSummary) bool {
			return taskMatches(task, text)
		})
	}

	return &taskFilter{name: strings.Join(name, ","), cond: conditions}, nil
}

func taskInterval(t *parsedTrace, s *trace.UserTaskSummary) interval {
	var i interval
	if s.Start != nil {
		i.start = s.Start.Time()
	} else {
		i.start = t.startTime()
	}
	if s.End != nil {
		i.end = s.End.Time()
	} else {
		i.end = t.endTime()
	}
	return i
}

func taskMatches(t *trace.UserTaskSummary, text string) bool {
	matches := func(s string) bool {
		return strings.Contains(s, text)
	}
	if matches(t.Name) {
		return true
	}
	for _, r := range t.Regions {
		if matches(r.Name) {
			return true
		}
	}
	for _, ev := range t.Logs {
		log := ev.Log()
		if matches(log.Category) {
			return true
		}
		if matches(log.Message) {
			return true
		}
	}
	return false
}

func describeEvent(ev *trace.Event) string {
	switch ev.Kind() {
	case trace.EventStateTransition:
		st := ev.StateTransition()
		if st.Resource.Kind != trace.ResourceGoroutine {
			return ""
		}
		old, new := st.Goroutine()
		return fmt.Sprintf("%s -> %s", old, new)
	case trace.EventRegionBegin:
		return fmt.Sprintf("region %q begin", ev.Region().Type)
	case trace.EventRegionEnd:
		return fmt.Sprintf("region %q end", ev.Region().Type)
	case trace.EventTaskBegin:
		t := ev.Task()
		return fmt.Sprintf("task %q (ID %d, parent %d) begin", t.Type, t.ID, t.Parent)
	case trace.EventTaskEnd:
		return "task end"
	case trace.EventLog:
		log := ev.Log()
		if log.Category == "" {
			return fmt.Sprintf("log %q", log.Message)
		}
		return fmt.Sprintf("log (category: %s): %q", log.Category, log.Message)
	}
	return ""
}

func primaryGoroutine(ev *trace.Event) trace.GoID {
	if ev.Kind() != trace.EventStateTransition {
		return ev.Goroutine()
	}
	st := ev.StateTransition()
	if st.Resource.Kind != trace.ResourceGoroutine {
		return trace.NoGoroutine
	}
	return st.Resource.Goroutine()
}

func elapsed(d time.Duration) string {
	b := fmt.Appendf(nil, "%.9f", d.Seconds())

	// For subsecond durations, blank all zeros before decimal point,
	// and all zeros between the decimal point and the first non-zero digit.
	if d < time.Second {
		dot := bytes.IndexByte(b, '.')
		for i := 0; i < dot; i++ {
			b[i] = ' '
		}
		for i := dot + 1; i < len(b); i++ {
			if b[i] == '0' {
				b[i] = ' '
			} else {
				break
			}
		}
	}
	return string(b)
}

func asMillisecond(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / float64(time.Millisecond)
}

```

// === FILE: references/go/src/cmd/trace/threadgen.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"internal/trace"
	"internal/trace/traceviewer"
	"internal/trace/traceviewer/format"
)

var _ generator = &threadGenerator{}

type threadGenerator struct {
	globalRangeGenerator
	globalMetricGenerator
	stackSampleGenerator[trace.ThreadID]
	logEventGenerator[trace.ThreadID]

	gStates map[trace.GoID]*gState[trace.ThreadID]
	threads map[trace.ThreadID]struct{}
}

func newThreadGenerator() *threadGenerator {
	tg := new(threadGenerator)
	rg := func(ev *trace.Event) trace.ThreadID {
		return ev.Thread()
	}
	tg.stackSampleGenerator.getResource = rg
	tg.logEventGenerator.getResource = rg
	tg.gStates = make(map[trace.GoID]*gState[trace.ThreadID])
	tg.threads = make(map[trace.ThreadID]struct{})
	return tg
}

func (g *threadGenerator) Sync() {
	g.globalRangeGenerator.Sync()
}

func (g *threadGenerator) GoroutineLabel(ctx *traceContext, ev *trace.Event) {
	l := ev.Label()
	g.gStates[l.Resource.Goroutine()].setLabel(l.Label)
}

func (g *threadGenerator) GoroutineRange(ctx *traceContext, ev *trace.Event) {
	r := ev.Range()
	switch ev.Kind() {
	case trace.EventRangeBegin:
		g.gStates[r.Scope.Goroutine()].rangeBegin(ev.Time(), r.Name, ev.Stack())
	case trace.EventRangeActive:
		g.gStates[r.Scope.Goroutine()].rangeActive(r.Name)
	case trace.EventRangeEnd:
		gs := g.gStates[r.Scope.Goroutine()]
		gs.rangeEnd(ev.Time(), r.Name, ev.Stack(), ctx)
	}
}

func (g *threadGenerator) GoroutineTransition(ctx *traceContext, ev *trace.Event) {
	if ev.Thread() != trace.NoThread {
		if _, ok := g.threads[ev.Thread()]; !ok {
			g.threads[ev.Thread()] = struct{}{}
		}
	}

	st := ev.StateTransition()
	goID := st.Resource.Goroutine()

	// If we haven't seen this goroutine before, create a new
	// gState for it.
	gs, ok := g.gStates[goID]
	if !ok {
		gs = newGState[trace.ThreadID](goID)
		g.gStates[goID] = gs
	}
	// If we haven't already named this goroutine, try to name it.
	gs.augmentName(st.Stack)

	// Handle the goroutine state transition.
	from, to := st.Goroutine()
	if from == to {
		// Filter out no-op events.
		return
	}
	if from.Executing() && !to.Executing() {
		if to == trace.GoWaiting {
			// Goroutine started blocking.
			gs.block(ev.Time(), ev.Stack(), st.Reason, ctx)
		} else {
			gs.stop(ev.Time(), ev.Stack(), ctx)
		}
	}
	if !from.Executing() && to.Executing() {
		start := ev.Time()
		if from == trace.GoUndetermined {
			// Back-date the event to the start of the trace.
			start = ctx.startTime
		}
		gs.start(start, ev.Thread(), ctx)
	}

	if from == trace.GoWaiting {
		// Goroutine was unblocked.
		gs.unblock(ev.Time(), ev.Stack(), ev.Thread(), ctx)
	}
	if from == trace.GoNotExist && to == trace.GoRunnable {
		// Goroutine was created.
		gs.created(ev.Time(), ev.Thread(), ev.Stack())
	}
	if from == trace.GoSyscall {
		// Exiting syscall.
		gs.syscallEnd(ev.Time(), to != trace.GoRunning, ctx)
	}

	// Handle syscalls.
	if to == trace.GoSyscall {
		start := ev.Time()
		if from == trace.GoUndetermined {
			// Back-date the event to the start of the trace.
			start = ctx.startTime
		}
		// Write down that we've entered a syscall. Note: we might have no P here
		// if we're in a cgo callback or this is a transition from GoUndetermined
		// (i.e. the G has been blocked in a syscall).
		gs.syscallBegin(start, ev.Thread(), ev.Stack())
	}

	// Note down the goroutine transition.
	_, inMarkAssist := gs.activeRanges["GC mark assist"]
	ctx.GoroutineTransition(ctx.elapsed(ev.Time()), viewerGState(from, inMarkAssist), viewerGState(to, inMarkAssist))
}

func (g *threadGenerator) ProcTransition(ctx *traceContext, ev *trace.Event) {
	if ev.Thread() != trace.NoThread {
		if _, ok := g.threads[ev.Thread()]; !ok {
			g.threads[ev.Thread()] = struct{}{}
		}
	}

	st := ev.StateTransition()
	viewerEv := traceviewer.InstantEvent{
		Resource: uint64(ev.Thread()),
		Stack:    ctx.Stack(viewerFrames(ev.Stack())),

		// Annotate with the thread and proc. The thread is redundant, but this is to
		// stay consistent with the proc view.
		Arg: format.SchedCtxArg{
			ProcID:   uint64(st.Resource.Proc()),
			ThreadID: uint64(ev.Thread()),
		},
	}

	from, to := st.Proc()
	if from == to {
		// Filter out no-op events.
		return
	}
	if to.Executing() {
		start := ev.Time()
		if from == trace.ProcUndetermined {
			start = ctx.startTime
		}
		viewerEv.Name = "proc start"
		viewerEv.Ts = ctx.elapsed(start)
		// TODO(mknyszek): We don't have a state machine for threads, so approximate
		// running threads with running Ps.
		ctx.IncThreadStateCount(ctx.elapsed(start), traceviewer.ThreadStateRunning, 1)
	}
	if from.Executing() {
		start := ev.Time()
		viewerEv.Name = "proc stop"
		viewerEv.Ts = ctx.elapsed(start)
		// TODO(mknyszek): We don't have a state machine for threads, so approximate
		// running threads with running Ps.
		ctx.IncThreadStateCount(ctx.elapsed(start), traceviewer.ThreadStateRunning, -1)
	}
	// TODO(mknyszek): Consider modeling procs differently and have them be
	// transition to and from NotExist when GOMAXPROCS changes. We can emit
	// events for this to clearly delineate GOMAXPROCS changes.

	if viewerEv.Name != "" {
		ctx.Instant(viewerEv)
	}
}

func (g *threadGenerator) ProcRange(ctx *traceContext, ev *trace.Event) {
	// TODO(mknyszek): Extend procRangeGenerator to support rendering proc ranges on threads.
}

func (g *threadGenerator) Finish(ctx *traceContext) {
	ctx.SetResourceType("OS THREADS")

	// Finish off global ranges.
	g.globalRangeGenerator.Finish(ctx)

	// Finish off all the goroutine slices.
	for _, gs := range g.gStates {
		gs.finish(ctx)
	}

	// Name all the threads to the emitter.
	for id := range g.threads {
		ctx.Resource(uint64(id), fmt.Sprintf("Thread %d", id))
	}
}

```

// === FILE: references/go/src/cmd/trace/viewer.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"internal/trace"
	"internal/trace/traceviewer"
	"slices"
)

// viewerFrames returns the frames of the stack of ev. The given frame slice is
// used to store the frames to reduce allocations.
func viewerFrames(stk trace.Stack) []trace.StackFrame {
	return slices.Collect(stk.Frames())
}

func viewerGState(state trace.GoState, inMarkAssist bool) traceviewer.GState {
	switch state {
	case trace.GoUndetermined:
		return traceviewer.GDead
	case trace.GoNotExist:
		return traceviewer.GDead
	case trace.GoRunnable:
		return traceviewer.GRunnable
	case trace.GoRunning:
		return traceviewer.GRunning
	case trace.GoWaiting:
		if inMarkAssist {
			return traceviewer.GWaitingGC
		}
		return traceviewer.GWaiting
	case trace.GoSyscall:
		// N.B. A goroutine in a syscall is considered "executing" (state.Executing() == true).
		return traceviewer.GRunning
	default:
		panic(fmt.Sprintf("unknown GoState: %s", state.String()))
	}
}

```

