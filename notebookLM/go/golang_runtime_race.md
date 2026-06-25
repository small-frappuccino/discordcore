# Domain Architecture: runtime/race

## Layout Topology
```text
runtime/race/
├── internal
│   ├── amd64v1
│   │   ├── doc.go
│   │   ├── race_darwin.patch
│   │   ├── race_darwin.syso
│   │   ├── race_freebsd.patch
│   │   ├── race_freebsd.syso
│   │   ├── race_linux.patch
│   │   ├── race_linux.syso
│   │   ├── race_netbsd.syso
│   │   ├── race_openbsd.syso
│   │   ├── race_windows.patch
│   │   └── race_windows.syso
│   └── amd64v3
│       ├── doc.go
│       ├── race_linux.patch
│       └── race_linux.syso
├── README
├── doc.go
├── mkcgo.sh
├── race.go
├── race_darwin_amd64.go
├── race_darwin_arm64.go
├── race_darwin_arm64.patch
├── race_darwin_arm64.syso
├── race_linux_arm64.patch
├── race_linux_arm64.syso
├── race_linux_loong64.patch
├── race_linux_loong64.syso
├── race_linux_ppc64le.syso
├── race_linux_riscv64.syso
├── race_linux_s390x.patch
├── race_linux_s390x.syso
├── race_v1_amd64.go
└── race_v3_amd64.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/runtime/race/doc.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package race implements data race detection logic.
// No public interface is provided.
// For details about the race detector see
// https://golang.org/doc/articles/race_detector.html
package race

//go:generate ./mkcgo.sh

```

// === FILE: references!/go/src/runtime/race/internal/amd64v1/doc.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This package holds the race detector .syso for
// amd64 architectures with GOAMD64<v3.

//go:build amd64 && ((linux && !amd64.v3) || darwin || freebsd || netbsd || openbsd || windows)

package amd64v1

```

// === FILE: references!/go/src/runtime/race/internal/amd64v1/race_darwin.patch ===
```text
From cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca Mon Sep 17 00:00:00 2001
From: Michael Pratt <michael@prattmic.com>
Date: Fri, 12 Dec 2025 16:31:44 +1100
Subject: [PATCH] [TSan] Zero-initialize Trace.local_head

Trace.local_head is currently uninitialized when Trace is created. It is
first initialized when the first event is added to the trace, via the
first call to TraceSwitchPartImpl.

However, ThreadContext::OnFinished uses local_head, assuming that it is
initialized. If it has not been initialized, we have undefined behavior,
likely crashing if the contents are garbage. The allocator (Alloc)
reuses previously allocations, so the contents of the uninitialized
memory are arbitrary.

In a C/C++ TSAN binary it is likely very difficult for a thread to start
and exit without a single event inbetween. For Go programs, code running
in the Go runtime itself is not TSan-instrumented, so goroutines that
exclusively run runtime code (such as GC workers) can quite reasonably
have no TSan events.

The addition of such a goroutine to the Go test.c is sufficient to
trigger this case, though for reliable failure (segfault) I've found it
necessary to poison the ThreadContext allocation like so:

(Example patch redacted because patch tries to apply this as a real
patch. See full commit at
https://github.com/llvm/llvm-project/commit/cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca).

The fix is trivial: local_head should be zero-initialized.
---
 compiler-rt/lib/tsan/go/test.c        | 4 ++++
 compiler-rt/lib/tsan/rtl/tsan_trace.h | 2 +-
 2 files changed, 5 insertions(+), 1 deletion(-)

diff --git a/compiler-rt/lib/tsan/go/test.c b/compiler-rt/lib/tsan/go/test.c
index d328ab1b331d7..fcd396227a4ab 100644
--- a/compiler-rt/lib/tsan/go/test.c
+++ b/compiler-rt/lib/tsan/go/test.c
@@ -91,6 +91,10 @@ int main(void) {
   __tsan_go_start(thr0, &thr1, (char*)&barfoo + 1);
   void *thr2 = 0;
   __tsan_go_start(thr0, &thr2, (char*)&barfoo + 1);
+  // Goroutine that exits without a single event.
+  void *thr3 = 0;
+  __tsan_go_start(thr0, &thr3, (char*)&barfoo + 1);
+  __tsan_go_end(thr3);
   __tsan_func_exit(thr0);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
diff --git a/compiler-rt/lib/tsan/rtl/tsan_trace.h b/compiler-rt/lib/tsan/rtl/tsan_trace.h
index 01bb7b34f43a2..1e791ff765fec 100644
--- a/compiler-rt/lib/tsan/rtl/tsan_trace.h
+++ b/compiler-rt/lib/tsan/rtl/tsan_trace.h
@@ -190,7 +190,7 @@ struct Trace {
   Mutex mtx;
   IList<TraceHeader, &TraceHeader::trace_parts, TracePart> parts;
   // First node non-queued into ctx->trace_part_recycle.
-  TracePart* local_head;
+  TracePart* local_head = nullptr;
   // Final position in the last part for finished threads.
   Event* final_pos = nullptr;
   // Number of trace parts allocated on behalf of this trace specifically.

```

// === FILE: references!/go/src/runtime/race/internal/amd64v1/race_darwin.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0xcf in position 0: invalid continuation byte

```

// === FILE: references!/go/src/runtime/race/internal/amd64v1/race_freebsd.patch ===
```text
From cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca Mon Sep 17 00:00:00 2001
From: Michael Pratt <michael@prattmic.com>
Date: Fri, 12 Dec 2025 16:31:44 +1100
Subject: [PATCH] [TSan] Zero-initialize Trace.local_head

Trace.local_head is currently uninitialized when Trace is created. It is
first initialized when the first event is added to the trace, via the
first call to TraceSwitchPartImpl.

However, ThreadContext::OnFinished uses local_head, assuming that it is
initialized. If it has not been initialized, we have undefined behavior,
likely crashing if the contents are garbage. The allocator (Alloc)
reuses previously allocations, so the contents of the uninitialized
memory are arbitrary.

In a C/C++ TSAN binary it is likely very difficult for a thread to start
and exit without a single event inbetween. For Go programs, code running
in the Go runtime itself is not TSan-instrumented, so goroutines that
exclusively run runtime code (such as GC workers) can quite reasonably
have no TSan events.

The addition of such a goroutine to the Go test.c is sufficient to
trigger this case, though for reliable failure (segfault) I've found it
necessary to poison the ThreadContext allocation like so:

(Example patch redacted because patch tries to apply this as a real
patch. See full commit at
https://github.com/llvm/llvm-project/commit/cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca).

The fix is trivial: local_head should be zero-initialized.
---
 compiler-rt/lib/tsan/go/test.c        | 4 ++++
 compiler-rt/lib/tsan/rtl/tsan_trace.h | 2 +-
 2 files changed, 5 insertions(+), 1 deletion(-)

diff --git a/compiler-rt/lib/tsan/go/test.c b/compiler-rt/lib/tsan/go/test.c
index d328ab1b331d7..fcd396227a4ab 100644
--- a/compiler-rt/lib/tsan/go/test.c
+++ b/compiler-rt/lib/tsan/go/test.c
@@ -91,6 +91,10 @@ int main(void) {
   __tsan_go_start(thr0, &thr1, (char*)&barfoo + 1);
   void *thr2 = 0;
   __tsan_go_start(thr0, &thr2, (char*)&barfoo + 1);
+  // Goroutine that exits without a single event.
+  void *thr3 = 0;
+  __tsan_go_start(thr0, &thr3, (char*)&barfoo + 1);
+  __tsan_go_end(thr3);
   __tsan_func_exit(thr0);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
diff --git a/compiler-rt/lib/tsan/rtl/tsan_trace.h b/compiler-rt/lib/tsan/rtl/tsan_trace.h
index 01bb7b34f43a2..1e791ff765fec 100644
--- a/compiler-rt/lib/tsan/rtl/tsan_trace.h
+++ b/compiler-rt/lib/tsan/rtl/tsan_trace.h
@@ -190,7 +190,7 @@ struct Trace {
   Mutex mtx;
   IList<TraceHeader, &TraceHeader::trace_parts, TracePart> parts;
   // First node non-queued into ctx->trace_part_recycle.
-  TracePart* local_head;
+  TracePart* local_head = nullptr;
   // Final position in the last part for finished threads.
   Event* final_pos = nullptr;
   // Number of trace parts allocated on behalf of this trace specifically.

```

// === FILE: references!/go/src/runtime/race/internal/amd64v1/race_freebsd.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0x92 in position 41: invalid start byte

```

// === FILE: references!/go/src/runtime/race/internal/amd64v1/race_linux.patch ===
```text
From cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca Mon Sep 17 00:00:00 2001
From: Michael Pratt <michael@prattmic.com>
Date: Fri, 12 Dec 2025 16:31:44 +1100
Subject: [PATCH] [TSan] Zero-initialize Trace.local_head

Trace.local_head is currently uninitialized when Trace is created. It is
first initialized when the first event is added to the trace, via the
first call to TraceSwitchPartImpl.

However, ThreadContext::OnFinished uses local_head, assuming that it is
initialized. If it has not been initialized, we have undefined behavior,
likely crashing if the contents are garbage. The allocator (Alloc)
reuses previously allocations, so the contents of the uninitialized
memory are arbitrary.

In a C/C++ TSAN binary it is likely very difficult for a thread to start
and exit without a single event inbetween. For Go programs, code running
in the Go runtime itself is not TSan-instrumented, so goroutines that
exclusively run runtime code (such as GC workers) can quite reasonably
have no TSan events.

The addition of such a goroutine to the Go test.c is sufficient to
trigger this case, though for reliable failure (segfault) I've found it
necessary to poison the ThreadContext allocation like so:

(Example patch redacted because patch tries to apply this as a real
patch. See full commit at
https://github.com/llvm/llvm-project/commit/cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca).

The fix is trivial: local_head should be zero-initialized.
---
 compiler-rt/lib/tsan/go/test.c        | 4 ++++
 compiler-rt/lib/tsan/rtl/tsan_trace.h | 2 +-
 2 files changed, 5 insertions(+), 1 deletion(-)

diff --git a/compiler-rt/lib/tsan/go/test.c b/compiler-rt/lib/tsan/go/test.c
index d328ab1b331d7..fcd396227a4ab 100644
--- a/compiler-rt/lib/tsan/go/test.c
+++ b/compiler-rt/lib/tsan/go/test.c
@@ -91,6 +91,10 @@ int main(void) {
   __tsan_go_start(thr0, &thr1, (char*)&barfoo + 1);
   void *thr2 = 0;
   __tsan_go_start(thr0, &thr2, (char*)&barfoo + 1);
+  // Goroutine that exits without a single event.
+  void *thr3 = 0;
+  __tsan_go_start(thr0, &thr3, (char*)&barfoo + 1);
+  __tsan_go_end(thr3);
   __tsan_func_exit(thr0);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
diff --git a/compiler-rt/lib/tsan/rtl/tsan_trace.h b/compiler-rt/lib/tsan/rtl/tsan_trace.h
index 01bb7b34f43a2..1e791ff765fec 100644
--- a/compiler-rt/lib/tsan/rtl/tsan_trace.h
+++ b/compiler-rt/lib/tsan/rtl/tsan_trace.h
@@ -190,7 +190,7 @@ struct Trace {
   Mutex mtx;
   IList<TraceHeader, &TraceHeader::trace_parts, TracePart> parts;
   // First node non-queued into ctx->trace_part_recycle.
-  TracePart* local_head;
+  TracePart* local_head = nullptr;
   // Final position in the last part for finished threads.
   Event* final_pos = nullptr;
   // Number of trace parts allocated on behalf of this trace specifically.

```

// === FILE: references!/go/src/runtime/race/internal/amd64v1/race_linux.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0xf0 in position 40: invalid continuation byte

```

// === FILE: references!/go/src/runtime/race/internal/amd64v1/race_netbsd.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0xee in position 41: invalid continuation byte

```

// === FILE: references!/go/src/runtime/race/internal/amd64v1/race_openbsd.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0xe6 in position 60: invalid continuation byte

```

// === FILE: references!/go/src/runtime/race/internal/amd64v1/race_windows.patch ===
```text
From cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca Mon Sep 17 00:00:00 2001
From: Michael Pratt <michael@prattmic.com>
Date: Fri, 12 Dec 2025 16:31:44 +1100
Subject: [PATCH] [TSan] Zero-initialize Trace.local_head

Trace.local_head is currently uninitialized when Trace is created. It is
first initialized when the first event is added to the trace, via the
first call to TraceSwitchPartImpl.

However, ThreadContext::OnFinished uses local_head, assuming that it is
initialized. If it has not been initialized, we have undefined behavior,
likely crashing if the contents are garbage. The allocator (Alloc)
reuses previously allocations, so the contents of the uninitialized
memory are arbitrary.

In a C/C++ TSAN binary it is likely very difficult for a thread to start
and exit without a single event inbetween. For Go programs, code running
in the Go runtime itself is not TSan-instrumented, so goroutines that
exclusively run runtime code (such as GC workers) can quite reasonably
have no TSan events.

The addition of such a goroutine to the Go test.c is sufficient to
trigger this case, though for reliable failure (segfault) I've found it
necessary to poison the ThreadContext allocation like so:

(Example patch redacted because patch tries to apply this as a real
patch. See full commit at
https://github.com/llvm/llvm-project/commit/cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca).

The fix is trivial: local_head should be zero-initialized.
---
 compiler-rt/lib/tsan/go/test.c        | 4 ++++
 compiler-rt/lib/tsan/rtl/tsan_trace.h | 2 +-
 2 files changed, 5 insertions(+), 1 deletion(-)

diff --git a/compiler-rt/lib/tsan/go/test.c b/compiler-rt/lib/tsan/go/test.c
index d328ab1b331d7..fcd396227a4ab 100644
--- a/compiler-rt/lib/tsan/go/test.c
+++ b/compiler-rt/lib/tsan/go/test.c
@@ -91,6 +91,10 @@ int main(void) {
   __tsan_go_start(thr0, &thr1, (char*)&barfoo + 1);
   void *thr2 = 0;
   __tsan_go_start(thr0, &thr2, (char*)&barfoo + 1);
+  // Goroutine that exits without a single event.
+  void *thr3 = 0;
+  __tsan_go_start(thr0, &thr3, (char*)&barfoo + 1);
+  __tsan_go_end(thr3);
   __tsan_func_exit(thr0);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
diff --git a/compiler-rt/lib/tsan/rtl/tsan_trace.h b/compiler-rt/lib/tsan/rtl/tsan_trace.h
index 01bb7b34f43a2..1e791ff765fec 100644
--- a/compiler-rt/lib/tsan/rtl/tsan_trace.h
+++ b/compiler-rt/lib/tsan/rtl/tsan_trace.h
@@ -190,7 +190,7 @@ struct Trace {
   Mutex mtx;
   IList<TraceHeader, &TraceHeader::trace_parts, TracePart> parts;
   // First node non-queued into ctx->trace_part_recycle.
-  TracePart* local_head;
+  TracePart* local_head = nullptr;
   // Final position in the last part for finished threads.
   Event* final_pos = nullptr;
   // Number of trace parts allocated on behalf of this trace specifically.

```

// === FILE: references!/go/src/runtime/race/internal/amd64v1/race_windows.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0x86 in position 1: invalid start byte

```

// === FILE: references!/go/src/runtime/race/internal/amd64v3/doc.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This package holds the race detector .syso for
// amd64 architectures with GOAMD64>=v3.

//go:build amd64 && linux && amd64.v3

package amd64v3

```

// === FILE: references!/go/src/runtime/race/internal/amd64v3/race_linux.patch ===
```text
From cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca Mon Sep 17 00:00:00 2001
From: Michael Pratt <michael@prattmic.com>
Date: Fri, 12 Dec 2025 16:31:44 +1100
Subject: [PATCH] [TSan] Zero-initialize Trace.local_head

Trace.local_head is currently uninitialized when Trace is created. It is
first initialized when the first event is added to the trace, via the
first call to TraceSwitchPartImpl.

However, ThreadContext::OnFinished uses local_head, assuming that it is
initialized. If it has not been initialized, we have undefined behavior,
likely crashing if the contents are garbage. The allocator (Alloc)
reuses previously allocations, so the contents of the uninitialized
memory are arbitrary.

In a C/C++ TSAN binary it is likely very difficult for a thread to start
and exit without a single event inbetween. For Go programs, code running
in the Go runtime itself is not TSan-instrumented, so goroutines that
exclusively run runtime code (such as GC workers) can quite reasonably
have no TSan events.

The addition of such a goroutine to the Go test.c is sufficient to
trigger this case, though for reliable failure (segfault) I've found it
necessary to poison the ThreadContext allocation like so:

(Example patch redacted because patch tries to apply this as a real
patch. See full commit at
https://github.com/llvm/llvm-project/commit/cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca).

The fix is trivial: local_head should be zero-initialized.
---
 compiler-rt/lib/tsan/go/test.c        | 4 ++++
 compiler-rt/lib/tsan/rtl/tsan_trace.h | 2 +-
 2 files changed, 5 insertions(+), 1 deletion(-)

diff --git a/compiler-rt/lib/tsan/go/test.c b/compiler-rt/lib/tsan/go/test.c
index d328ab1b331d7..fcd396227a4ab 100644
--- a/compiler-rt/lib/tsan/go/test.c
+++ b/compiler-rt/lib/tsan/go/test.c
@@ -91,6 +91,10 @@ int main(void) {
   __tsan_go_start(thr0, &thr1, (char*)&barfoo + 1);
   void *thr2 = 0;
   __tsan_go_start(thr0, &thr2, (char*)&barfoo + 1);
+  // Goroutine that exits without a single event.
+  void *thr3 = 0;
+  __tsan_go_start(thr0, &thr3, (char*)&barfoo + 1);
+  __tsan_go_end(thr3);
   __tsan_func_exit(thr0);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
diff --git a/compiler-rt/lib/tsan/rtl/tsan_trace.h b/compiler-rt/lib/tsan/rtl/tsan_trace.h
index 01bb7b34f43a2..1e791ff765fec 100644
--- a/compiler-rt/lib/tsan/rtl/tsan_trace.h
+++ b/compiler-rt/lib/tsan/rtl/tsan_trace.h
@@ -190,7 +190,7 @@ struct Trace {
   Mutex mtx;
   IList<TraceHeader, &TraceHeader::trace_parts, TracePart> parts;
   // First node non-queued into ctx->trace_part_recycle.
-  TracePart* local_head;
+  TracePart* local_head = nullptr;
   // Final position in the last part for finished threads.
   Event* final_pos = nullptr;
   // Number of trace parts allocated on behalf of this trace specifically.

```

// === FILE: references!/go/src/runtime/race/internal/amd64v3/race_linux.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0x80 in position 308: invalid start byte

```

// === FILE: references!/go/src/runtime/race/mkcgo.sh ===
```text
#!/bin/bash

hdr='
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by mkcgo.sh. DO NOT EDIT.

//go:build race

'

convert() {
	(echo "$hdr"; go tool cgo -dynpackage race -dynimport $1) | gofmt
}

convert race_darwin_arm64.syso >race_darwin_arm64.go
convert internal/amd64v1/race_darwin.syso >race_darwin_amd64.go


```

// === FILE: references!/go/src/runtime/race/race.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build race && ((linux && (amd64 || arm64 || loong64 || ppc64le || riscv64 || s390x)) || ((freebsd || netbsd || openbsd || windows) && amd64))

package race

// This file merely ensures that we link in runtime/cgo in race build,
// this in turn ensures that runtime uses pthread_create to create threads.
// The prebuilt race runtime lives in race_GOOS_GOARCH.syso.
// Calls to the runtime are done directly from src/runtime/race.go.

// On darwin we always use system DLLs to create threads,
// so we use race_darwin_$GOARCH.go to provide the syso-derived
// symbol information without needing to invoke cgo.
// This allows -race to be used on Mac systems without a C toolchain.

// void __race_unused_func(void);
import "C"

```

// === FILE: references!/go/src/runtime/race/race_darwin_amd64.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by mkcgo.sh. DO NOT EDIT.

//go:build race

package race

//go:cgo_import_dynamic _Block_object_assign _Block_object_assign ""
//go:cgo_import_dynamic _Block_object_dispose _Block_object_dispose ""
//go:cgo_import_dynamic _NSConcreteStackBlock _NSConcreteStackBlock ""
//go:cgo_import_dynamic _NSGetArgv _NSGetArgv ""
//go:cgo_import_dynamic _NSGetEnviron _NSGetEnviron ""
//go:cgo_import_dynamic _NSGetExecutablePath _NSGetExecutablePath ""
//go:cgo_import_dynamic __bzero __bzero ""
//go:cgo_import_dynamic __error __error ""
//go:cgo_import_dynamic __fork __fork ""
//go:cgo_import_dynamic __mmap __mmap ""
//go:cgo_import_dynamic __munmap __munmap ""
//go:cgo_import_dynamic __stack_chk_fail __stack_chk_fail ""
//go:cgo_import_dynamic __stack_chk_guard __stack_chk_guard ""
//go:cgo_import_dynamic _dyld_get_image_header _dyld_get_image_header ""
//go:cgo_import_dynamic _dyld_get_image_name _dyld_get_image_name ""
//go:cgo_import_dynamic _dyld_get_image_vmaddr_slide _dyld_get_image_vmaddr_slide ""
//go:cgo_import_dynamic _dyld_get_shared_cache_range _dyld_get_shared_cache_range ""
//go:cgo_import_dynamic _dyld_get_shared_cache_uuid _dyld_get_shared_cache_uuid ""
//go:cgo_import_dynamic _dyld_image_count _dyld_image_count ""
//go:cgo_import_dynamic _exit _exit ""
//go:cgo_import_dynamic abort abort ""
//go:cgo_import_dynamic arc4random_buf arc4random_buf ""
//go:cgo_import_dynamic close close ""
//go:cgo_import_dynamic dlsym dlsym ""
//go:cgo_import_dynamic dup dup ""
//go:cgo_import_dynamic dup2 dup2 ""
//go:cgo_import_dynamic dyld_shared_cache_iterate_text dyld_shared_cache_iterate_text ""
//go:cgo_import_dynamic execve execve ""
//go:cgo_import_dynamic exit exit ""
//go:cgo_import_dynamic fstat$INODE64 fstat$INODE64 ""
//go:cgo_import_dynamic ftruncate ftruncate ""
//go:cgo_import_dynamic getpid getpid ""
//go:cgo_import_dynamic getrlimit getrlimit ""
//go:cgo_import_dynamic gettimeofday gettimeofday ""
//go:cgo_import_dynamic getuid getuid ""
//go:cgo_import_dynamic grantpt grantpt ""
//go:cgo_import_dynamic ioctl ioctl ""
//go:cgo_import_dynamic isatty isatty ""
//go:cgo_import_dynamic lstat$INODE64 lstat$INODE64 ""
//go:cgo_import_dynamic mach_absolute_time mach_absolute_time ""
//go:cgo_import_dynamic mach_task_self_ mach_task_self_ ""
//go:cgo_import_dynamic mach_timebase_info mach_timebase_info ""
//go:cgo_import_dynamic mach_vm_region_recurse mach_vm_region_recurse ""
//go:cgo_import_dynamic madvise madvise ""
//go:cgo_import_dynamic malloc_num_zones malloc_num_zones ""
//go:cgo_import_dynamic malloc_zones malloc_zones ""
//go:cgo_import_dynamic memset_pattern16 memset_pattern16 ""
//go:cgo_import_dynamic mkdir mkdir ""
//go:cgo_import_dynamic mprotect mprotect ""
//go:cgo_import_dynamic open open ""
//go:cgo_import_dynamic pipe pipe ""
//go:cgo_import_dynamic posix_openpt posix_openpt ""
//go:cgo_import_dynamic posix_spawn posix_spawn ""
//go:cgo_import_dynamic posix_spawn_file_actions_addclose posix_spawn_file_actions_addclose ""
//go:cgo_import_dynamic posix_spawn_file_actions_adddup2 posix_spawn_file_actions_adddup2 ""
//go:cgo_import_dynamic posix_spawn_file_actions_destroy posix_spawn_file_actions_destroy ""
//go:cgo_import_dynamic posix_spawn_file_actions_init posix_spawn_file_actions_init ""
//go:cgo_import_dynamic posix_spawnattr_destroy posix_spawnattr_destroy ""
//go:cgo_import_dynamic posix_spawnattr_init posix_spawnattr_init ""
//go:cgo_import_dynamic posix_spawnattr_setflags posix_spawnattr_setflags ""
//go:cgo_import_dynamic pthread_attr_getstack pthread_attr_getstack ""
//go:cgo_import_dynamic pthread_create pthread_create ""
//go:cgo_import_dynamic pthread_get_stackaddr_np pthread_get_stackaddr_np ""
//go:cgo_import_dynamic pthread_get_stacksize_np pthread_get_stacksize_np ""
//go:cgo_import_dynamic pthread_getspecific pthread_getspecific ""
//go:cgo_import_dynamic pthread_introspection_hook_install pthread_introspection_hook_install ""
//go:cgo_import_dynamic pthread_join pthread_join ""
//go:cgo_import_dynamic pthread_self pthread_self ""
//go:cgo_import_dynamic pthread_sigmask pthread_sigmask ""
//go:cgo_import_dynamic pthread_threadid_np pthread_threadid_np ""
//go:cgo_import_dynamic read read ""
//go:cgo_import_dynamic readlink readlink ""
//go:cgo_import_dynamic realpath$DARWIN_EXTSN realpath$DARWIN_EXTSN ""
//go:cgo_import_dynamic rename rename ""
//go:cgo_import_dynamic sched_yield sched_yield ""
//go:cgo_import_dynamic setrlimit setrlimit ""
//go:cgo_import_dynamic sigaction sigaction ""
//go:cgo_import_dynamic stat$INODE64 stat$INODE64 ""
//go:cgo_import_dynamic sysconf sysconf ""
//go:cgo_import_dynamic sysctl sysctl ""
//go:cgo_import_dynamic sysctlbyname sysctlbyname ""
//go:cgo_import_dynamic task_info task_info ""
//go:cgo_import_dynamic tcgetattr tcgetattr ""
//go:cgo_import_dynamic tcsetattr tcsetattr ""
//go:cgo_import_dynamic unlink unlink ""
//go:cgo_import_dynamic unlockpt unlockpt ""
//go:cgo_import_dynamic usleep usleep ""
//go:cgo_import_dynamic vm_region_64 vm_region_64 ""
//go:cgo_import_dynamic vm_region_recurse_64 vm_region_recurse_64 ""
//go:cgo_import_dynamic waitpid waitpid ""
//go:cgo_import_dynamic write write ""

```

// === FILE: references!/go/src/runtime/race/race_darwin_arm64.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by mkcgo.sh. DO NOT EDIT.

//go:build race

package race

//go:cgo_import_dynamic _Block_object_assign _Block_object_assign ""
//go:cgo_import_dynamic _Block_object_dispose _Block_object_dispose ""
//go:cgo_import_dynamic _NSConcreteStackBlock _NSConcreteStackBlock ""
//go:cgo_import_dynamic _NSGetArgv _NSGetArgv ""
//go:cgo_import_dynamic _NSGetEnviron _NSGetEnviron ""
//go:cgo_import_dynamic _NSGetExecutablePath _NSGetExecutablePath ""
//go:cgo_import_dynamic __error __error ""
//go:cgo_import_dynamic __fork __fork ""
//go:cgo_import_dynamic __mmap __mmap ""
//go:cgo_import_dynamic __munmap __munmap ""
//go:cgo_import_dynamic __stack_chk_fail __stack_chk_fail ""
//go:cgo_import_dynamic __stack_chk_guard __stack_chk_guard ""
//go:cgo_import_dynamic _dyld_get_image_header _dyld_get_image_header ""
//go:cgo_import_dynamic _dyld_get_image_name _dyld_get_image_name ""
//go:cgo_import_dynamic _dyld_get_image_vmaddr_slide _dyld_get_image_vmaddr_slide ""
//go:cgo_import_dynamic _dyld_get_shared_cache_range _dyld_get_shared_cache_range ""
//go:cgo_import_dynamic _dyld_get_shared_cache_uuid _dyld_get_shared_cache_uuid ""
//go:cgo_import_dynamic _dyld_image_count _dyld_image_count ""
//go:cgo_import_dynamic _exit _exit ""
//go:cgo_import_dynamic abort abort ""
//go:cgo_import_dynamic arc4random_buf arc4random_buf ""
//go:cgo_import_dynamic bzero bzero ""
//go:cgo_import_dynamic close close ""
//go:cgo_import_dynamic dlsym dlsym ""
//go:cgo_import_dynamic dup dup ""
//go:cgo_import_dynamic dup2 dup2 ""
//go:cgo_import_dynamic dyld_shared_cache_iterate_text dyld_shared_cache_iterate_text ""
//go:cgo_import_dynamic execve execve ""
//go:cgo_import_dynamic exit exit ""
//go:cgo_import_dynamic fstat fstat ""
//go:cgo_import_dynamic ftruncate ftruncate ""
//go:cgo_import_dynamic getpid getpid ""
//go:cgo_import_dynamic getrlimit getrlimit ""
//go:cgo_import_dynamic gettimeofday gettimeofday ""
//go:cgo_import_dynamic getuid getuid ""
//go:cgo_import_dynamic grantpt grantpt ""
//go:cgo_import_dynamic ioctl ioctl ""
//go:cgo_import_dynamic isatty isatty ""
//go:cgo_import_dynamic lstat lstat ""
//go:cgo_import_dynamic mach_absolute_time mach_absolute_time ""
//go:cgo_import_dynamic mach_task_self_ mach_task_self_ ""
//go:cgo_import_dynamic mach_timebase_info mach_timebase_info ""
//go:cgo_import_dynamic mach_vm_region_recurse mach_vm_region_recurse ""
//go:cgo_import_dynamic madvise madvise ""
//go:cgo_import_dynamic malloc_num_zones malloc_num_zones ""
//go:cgo_import_dynamic malloc_zones malloc_zones ""
//go:cgo_import_dynamic memset_pattern16 memset_pattern16 ""
//go:cgo_import_dynamic mkdir mkdir ""
//go:cgo_import_dynamic mprotect mprotect ""
//go:cgo_import_dynamic open open ""
//go:cgo_import_dynamic pipe pipe ""
//go:cgo_import_dynamic posix_openpt posix_openpt ""
//go:cgo_import_dynamic posix_spawn posix_spawn ""
//go:cgo_import_dynamic posix_spawn_file_actions_addclose posix_spawn_file_actions_addclose ""
//go:cgo_import_dynamic posix_spawn_file_actions_adddup2 posix_spawn_file_actions_adddup2 ""
//go:cgo_import_dynamic posix_spawn_file_actions_destroy posix_spawn_file_actions_destroy ""
//go:cgo_import_dynamic posix_spawn_file_actions_init posix_spawn_file_actions_init ""
//go:cgo_import_dynamic posix_spawnattr_destroy posix_spawnattr_destroy ""
//go:cgo_import_dynamic posix_spawnattr_init posix_spawnattr_init ""
//go:cgo_import_dynamic posix_spawnattr_setflags posix_spawnattr_setflags ""
//go:cgo_import_dynamic pthread_attr_getstack pthread_attr_getstack ""
//go:cgo_import_dynamic pthread_create pthread_create ""
//go:cgo_import_dynamic pthread_get_stackaddr_np pthread_get_stackaddr_np ""
//go:cgo_import_dynamic pthread_get_stacksize_np pthread_get_stacksize_np ""
//go:cgo_import_dynamic pthread_getspecific pthread_getspecific ""
//go:cgo_import_dynamic pthread_introspection_hook_install pthread_introspection_hook_install ""
//go:cgo_import_dynamic pthread_join pthread_join ""
//go:cgo_import_dynamic pthread_self pthread_self ""
//go:cgo_import_dynamic pthread_sigmask pthread_sigmask ""
//go:cgo_import_dynamic pthread_threadid_np pthread_threadid_np ""
//go:cgo_import_dynamic read read ""
//go:cgo_import_dynamic readlink readlink ""
//go:cgo_import_dynamic realpath$DARWIN_EXTSN realpath$DARWIN_EXTSN ""
//go:cgo_import_dynamic rename rename ""
//go:cgo_import_dynamic sched_yield sched_yield ""
//go:cgo_import_dynamic setrlimit setrlimit ""
//go:cgo_import_dynamic sigaction sigaction ""
//go:cgo_import_dynamic stat stat ""
//go:cgo_import_dynamic sysconf sysconf ""
//go:cgo_import_dynamic sysctl sysctl ""
//go:cgo_import_dynamic sysctlbyname sysctlbyname ""
//go:cgo_import_dynamic task_info task_info ""
//go:cgo_import_dynamic tcgetattr tcgetattr ""
//go:cgo_import_dynamic tcsetattr tcsetattr ""
//go:cgo_import_dynamic unlink unlink ""
//go:cgo_import_dynamic unlockpt unlockpt ""
//go:cgo_import_dynamic usleep usleep ""
//go:cgo_import_dynamic vm_region_64 vm_region_64 ""
//go:cgo_import_dynamic vm_region_recurse_64 vm_region_recurse_64 ""
//go:cgo_import_dynamic waitpid waitpid ""
//go:cgo_import_dynamic write write ""

```

// === FILE: references!/go/src/runtime/race/race_darwin_arm64.patch ===
```text
From cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca Mon Sep 17 00:00:00 2001
From: Michael Pratt <michael@prattmic.com>
Date: Fri, 12 Dec 2025 16:31:44 +1100
Subject: [PATCH] [TSan] Zero-initialize Trace.local_head

Trace.local_head is currently uninitialized when Trace is created. It is
first initialized when the first event is added to the trace, via the
first call to TraceSwitchPartImpl.

However, ThreadContext::OnFinished uses local_head, assuming that it is
initialized. If it has not been initialized, we have undefined behavior,
likely crashing if the contents are garbage. The allocator (Alloc)
reuses previously allocations, so the contents of the uninitialized
memory are arbitrary.

In a C/C++ TSAN binary it is likely very difficult for a thread to start
and exit without a single event inbetween. For Go programs, code running
in the Go runtime itself is not TSan-instrumented, so goroutines that
exclusively run runtime code (such as GC workers) can quite reasonably
have no TSan events.

The addition of such a goroutine to the Go test.c is sufficient to
trigger this case, though for reliable failure (segfault) I've found it
necessary to poison the ThreadContext allocation like so:

(Example patch redacted because patch tries to apply this as a real
patch. See full commit at
https://github.com/llvm/llvm-project/commit/cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca).

The fix is trivial: local_head should be zero-initialized.
---
 compiler-rt/lib/tsan/go/test.c        | 4 ++++
 compiler-rt/lib/tsan/rtl/tsan_trace.h | 2 +-
 2 files changed, 5 insertions(+), 1 deletion(-)

diff --git a/compiler-rt/lib/tsan/go/test.c b/compiler-rt/lib/tsan/go/test.c
index d328ab1b331d7..fcd396227a4ab 100644
--- a/compiler-rt/lib/tsan/go/test.c
+++ b/compiler-rt/lib/tsan/go/test.c
@@ -91,6 +91,10 @@ int main(void) {
   __tsan_go_start(thr0, &thr1, (char*)&barfoo + 1);
   void *thr2 = 0;
   __tsan_go_start(thr0, &thr2, (char*)&barfoo + 1);
+  // Goroutine that exits without a single event.
+  void *thr3 = 0;
+  __tsan_go_start(thr0, &thr3, (char*)&barfoo + 1);
+  __tsan_go_end(thr3);
   __tsan_func_exit(thr0);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
diff --git a/compiler-rt/lib/tsan/rtl/tsan_trace.h b/compiler-rt/lib/tsan/rtl/tsan_trace.h
index 01bb7b34f43a2..1e791ff765fec 100644
--- a/compiler-rt/lib/tsan/rtl/tsan_trace.h
+++ b/compiler-rt/lib/tsan/rtl/tsan_trace.h
@@ -190,7 +190,7 @@ struct Trace {
   Mutex mtx;
   IList<TraceHeader, &TraceHeader::trace_parts, TracePart> parts;
   // First node non-queued into ctx->trace_part_recycle.
-  TracePart* local_head;
+  TracePart* local_head = nullptr;
   // Final position in the last part for finished threads.
   Event* final_pos = nullptr;
   // Number of trace parts allocated on behalf of this trace specifically.

```

// === FILE: references!/go/src/runtime/race/race_darwin_arm64.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0xcf in position 0: invalid continuation byte

```

// === FILE: references!/go/src/runtime/race/race_linux_arm64.patch ===
```text
From cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca Mon Sep 17 00:00:00 2001
From: Michael Pratt <michael@prattmic.com>
Date: Fri, 12 Dec 2025 16:31:44 +1100
Subject: [PATCH] [TSan] Zero-initialize Trace.local_head

Trace.local_head is currently uninitialized when Trace is created. It is
first initialized when the first event is added to the trace, via the
first call to TraceSwitchPartImpl.

However, ThreadContext::OnFinished uses local_head, assuming that it is
initialized. If it has not been initialized, we have undefined behavior,
likely crashing if the contents are garbage. The allocator (Alloc)
reuses previously allocations, so the contents of the uninitialized
memory are arbitrary.

In a C/C++ TSAN binary it is likely very difficult for a thread to start
and exit without a single event inbetween. For Go programs, code running
in the Go runtime itself is not TSan-instrumented, so goroutines that
exclusively run runtime code (such as GC workers) can quite reasonably
have no TSan events.

The addition of such a goroutine to the Go test.c is sufficient to
trigger this case, though for reliable failure (segfault) I've found it
necessary to poison the ThreadContext allocation like so:

(Example patch redacted because patch tries to apply this as a real
patch. See full commit at
https://github.com/llvm/llvm-project/commit/cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca).

The fix is trivial: local_head should be zero-initialized.
---
 compiler-rt/lib/tsan/go/test.c        | 4 ++++
 compiler-rt/lib/tsan/rtl/tsan_trace.h | 2 +-
 2 files changed, 5 insertions(+), 1 deletion(-)

diff --git a/compiler-rt/lib/tsan/go/test.c b/compiler-rt/lib/tsan/go/test.c
index d328ab1b331d7..fcd396227a4ab 100644
--- a/compiler-rt/lib/tsan/go/test.c
+++ b/compiler-rt/lib/tsan/go/test.c
@@ -91,6 +91,10 @@ int main(void) {
   __tsan_go_start(thr0, &thr1, (char*)&barfoo + 1);
   void *thr2 = 0;
   __tsan_go_start(thr0, &thr2, (char*)&barfoo + 1);
+  // Goroutine that exits without a single event.
+  void *thr3 = 0;
+  __tsan_go_start(thr0, &thr3, (char*)&barfoo + 1);
+  __tsan_go_end(thr3);
   __tsan_func_exit(thr0);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
diff --git a/compiler-rt/lib/tsan/rtl/tsan_trace.h b/compiler-rt/lib/tsan/rtl/tsan_trace.h
index 01bb7b34f43a2..1e791ff765fec 100644
--- a/compiler-rt/lib/tsan/rtl/tsan_trace.h
+++ b/compiler-rt/lib/tsan/rtl/tsan_trace.h
@@ -190,7 +190,7 @@ struct Trace {
   Mutex mtx;
   IList<TraceHeader, &TraceHeader::trace_parts, TracePart> parts;
   // First node non-queued into ctx->trace_part_recycle.
-  TracePart* local_head;
+  TracePart* local_head = nullptr;
   // Final position in the last part for finished threads.
   Event* final_pos = nullptr;
   // Number of trace parts allocated on behalf of this trace specifically.

```

// === FILE: references!/go/src/runtime/race/race_linux_arm64.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0xb7 in position 18: invalid start byte

```

// === FILE: references!/go/src/runtime/race/race_linux_loong64.patch ===
```text
From cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca Mon Sep 17 00:00:00 2001
From: Michael Pratt <michael@prattmic.com>
Date: Fri, 12 Dec 2025 16:31:44 +1100
Subject: [PATCH] [TSan] Zero-initialize Trace.local_head

Trace.local_head is currently uninitialized when Trace is created. It is
first initialized when the first event is added to the trace, via the
first call to TraceSwitchPartImpl.

However, ThreadContext::OnFinished uses local_head, assuming that it is
initialized. If it has not been initialized, we have undefined behavior,
likely crashing if the contents are garbage. The allocator (Alloc)
reuses previously allocations, so the contents of the uninitialized
memory are arbitrary.

In a C/C++ TSAN binary it is likely very difficult for a thread to start
and exit without a single event inbetween. For Go programs, code running
in the Go runtime itself is not TSan-instrumented, so goroutines that
exclusively run runtime code (such as GC workers) can quite reasonably
have no TSan events.

The addition of such a goroutine to the Go test.c is sufficient to
trigger this case, though for reliable failure (segfault) I've found it
necessary to poison the ThreadContext allocation like so:

(Example patch redacted because patch tries to apply this as a real
patch. See full commit at
https://github.com/llvm/llvm-project/commit/cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca).

The fix is trivial: local_head should be zero-initialized.
---
 compiler-rt/lib/tsan/go/test.c        | 4 ++++
 compiler-rt/lib/tsan/rtl/tsan_trace.h | 2 +-
 2 files changed, 5 insertions(+), 1 deletion(-)

diff --git a/compiler-rt/lib/tsan/go/test.c b/compiler-rt/lib/tsan/go/test.c
index d328ab1b331d7..fcd396227a4ab 100644
--- a/compiler-rt/lib/tsan/go/test.c
+++ b/compiler-rt/lib/tsan/go/test.c
@@ -91,6 +91,10 @@ int main(void) {
   __tsan_go_start(thr0, &thr1, (char*)&barfoo + 1);
   void *thr2 = 0;
   __tsan_go_start(thr0, &thr2, (char*)&barfoo + 1);
+  // Goroutine that exits without a single event.
+  void *thr3 = 0;
+  __tsan_go_start(thr0, &thr3, (char*)&barfoo + 1);
+  __tsan_go_end(thr3);
   __tsan_func_exit(thr0);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
diff --git a/compiler-rt/lib/tsan/rtl/tsan_trace.h b/compiler-rt/lib/tsan/rtl/tsan_trace.h
index 01bb7b34f43a2..1e791ff765fec 100644
--- a/compiler-rt/lib/tsan/rtl/tsan_trace.h
+++ b/compiler-rt/lib/tsan/rtl/tsan_trace.h
@@ -190,7 +190,7 @@ struct Trace {
   Mutex mtx;
   IList<TraceHeader, &TraceHeader::trace_parts, TracePart> parts;
   // First node non-queued into ctx->trace_part_recycle.
-  TracePart* local_head;
+  TracePart* local_head = nullptr;
   // Final position in the last part for finished threads.
   Event* final_pos = nullptr;
   // Number of trace parts allocated on behalf of this trace specifically.

```

// === FILE: references!/go/src/runtime/race/race_linux_loong64.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0x98 in position 40: invalid start byte

```

// === FILE: references!/go/src/runtime/race/race_linux_ppc64le.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0xe8 in position 40: invalid continuation byte

```

// === FILE: references!/go/src/runtime/race/race_linux_riscv64.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0xf3 in position 18: invalid continuation byte

```

// === FILE: references!/go/src/runtime/race/race_linux_s390x.patch ===
```text
From cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca Mon Sep 17 00:00:00 2001
From: Michael Pratt <michael@prattmic.com>
Date: Fri, 12 Dec 2025 16:31:44 +1100
Subject: [PATCH] [TSan] Zero-initialize Trace.local_head

Trace.local_head is currently uninitialized when Trace is created. It is
first initialized when the first event is added to the trace, via the
first call to TraceSwitchPartImpl.

However, ThreadContext::OnFinished uses local_head, assuming that it is
initialized. If it has not been initialized, we have undefined behavior,
likely crashing if the contents are garbage. The allocator (Alloc)
reuses previously allocations, so the contents of the uninitialized
memory are arbitrary.

In a C/C++ TSAN binary it is likely very difficult for a thread to start
and exit without a single event inbetween. For Go programs, code running
in the Go runtime itself is not TSan-instrumented, so goroutines that
exclusively run runtime code (such as GC workers) can quite reasonably
have no TSan events.

The addition of such a goroutine to the Go test.c is sufficient to
trigger this case, though for reliable failure (segfault) I've found it
necessary to poison the ThreadContext allocation like so:

(Example patch redacted because patch tries to apply this as a real
patch. See full commit at
https://github.com/llvm/llvm-project/commit/cdfdb06c9155080ec97d6e4f4dd90b6e7cefb0ca).

The fix is trivial: local_head should be zero-initialized.
---
 compiler-rt/lib/tsan/go/test.c        | 4 ++++
 compiler-rt/lib/tsan/rtl/tsan_trace.h | 2 +-
 2 files changed, 5 insertions(+), 1 deletion(-)

diff --git a/compiler-rt/lib/tsan/go/test.c b/compiler-rt/lib/tsan/go/test.c
index d328ab1b331d7..fcd396227a4ab 100644
--- a/compiler-rt/lib/tsan/go/test.c
+++ b/compiler-rt/lib/tsan/go/test.c
@@ -91,6 +91,10 @@ int main(void) {
   __tsan_go_start(thr0, &thr1, (char*)&barfoo + 1);
   void *thr2 = 0;
   __tsan_go_start(thr0, &thr2, (char*)&barfoo + 1);
+  // Goroutine that exits without a single event.
+  void *thr3 = 0;
+  __tsan_go_start(thr0, &thr3, (char*)&barfoo + 1);
+  __tsan_go_end(thr3);
   __tsan_func_exit(thr0);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
   __tsan_func_enter(thr1, (char*)&foobar + 1);
diff --git a/compiler-rt/lib/tsan/rtl/tsan_trace.h b/compiler-rt/lib/tsan/rtl/tsan_trace.h
index 01bb7b34f43a2..1e791ff765fec 100644
--- a/compiler-rt/lib/tsan/rtl/tsan_trace.h
+++ b/compiler-rt/lib/tsan/rtl/tsan_trace.h
@@ -190,7 +190,7 @@ struct Trace {
   Mutex mtx;
   IList<TraceHeader, &TraceHeader::trace_parts, TracePart> parts;
   // First node non-queued into ctx->trace_part_recycle.
-  TracePart* local_head;
+  TracePart* local_head = nullptr;
   // Final position in the last part for finished threads.
   Event* final_pos = nullptr;
   // Number of trace parts allocated on behalf of this trace specifically.

```

// === FILE: references!/go/src/runtime/race/race_linux_s390x.syso ===
```text
// [I/O FAULT]: Failed to map memory boundary - 'utf-8' codec can't decode byte 0x99 in position 46: invalid start byte

```

// === FILE: references!/go/src/runtime/race/race_v1_amd64.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (linux && !amd64.v3) || darwin || freebsd || netbsd || openbsd || windows

package race

import _ "runtime/race/internal/amd64v1"

```

// === FILE: references!/go/src/runtime/race/race_v3_amd64.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && amd64.v3

package race

import _ "runtime/race/internal/amd64v3"

```

// === FILE: references!/go/src/runtime/race/README ===
```text
runtime/race package contains the data race detector runtime library.
It is based on ThreadSanitizer race detector, that is currently a part of
the LLVM project (https://github.com/llvm/llvm-project/tree/main/compiler-rt).

To update the .syso files use golang.org/x/build/cmd/racebuild.

internal/amd64v1/race_darwin.syso built with LLVM 51bfeff0e4b0757ff773da6882f4d538996c9b04 with patch internal/amd64v1/race_darwin.patch and Go a61fd428974822a8c57a2b2840fc237e6711b24d.
internal/amd64v1/race_freebsd.syso built with LLVM 51bfeff0e4b0757ff773da6882f4d538996c9b04 with patch internal/amd64v1/race_freebsd.patch and Go a61fd428974822a8c57a2b2840fc237e6711b24d.
internal/amd64v1/race_linux.syso built with LLVM 51bfeff0e4b0757ff773da6882f4d538996c9b04 with patch internal/amd64v1/race_linux.patch and Go a61fd428974822a8c57a2b2840fc237e6711b24d.
internal/amd64v1/race_netbsd.syso built with LLVM 51bfeff0e4b0757ff773da6882f4d538996c9b04 and Go e7d582b55dda36e76ce4d0ce770139ca0915b7c5.
internal/amd64v1/race_openbsd.syso built with LLVM fcf6ae2f070eba73074b6ec8d8281e54d29dbeeb and Go 8f2db14cd35bbd674cb2988a508306de6655e425.
internal/amd64v1/race_windows.syso built with LLVM 51bfeff0e4b0757ff773da6882f4d538996c9b04 with patch internal/amd64v1/race_windows.patch and Go a61fd428974822a8c57a2b2840fc237e6711b24d.
internal/amd64v3/race_linux.syso built with LLVM 51bfeff0e4b0757ff773da6882f4d538996c9b04 with patch internal/amd64v3/race_linux.patch and Go a61fd428974822a8c57a2b2840fc237e6711b24d.
race_darwin_arm64.syso built with LLVM 51bfeff0e4b0757ff773da6882f4d538996c9b04 with patch race_darwin_arm64.patch and Go a61fd428974822a8c57a2b2840fc237e6711b24d.
race_linux_arm64.syso built with LLVM 51bfeff0e4b0757ff773da6882f4d538996c9b04 with patch race_linux_arm64.patch and Go a61fd428974822a8c57a2b2840fc237e6711b24d.
race_linux_loong64.syso built with LLVM 83fe85115da9dc25fa270d2ea8140113c8d49670 with patch race_linux_loong64.patch and Go a61fd428974822a8c57a2b2840fc237e6711b24d.
race_linux_ppc64le.syso built with LLVM 51bfeff0e4b0757ff773da6882f4d538996c9b04 and Go e7d582b55dda36e76ce4d0ce770139ca0915b7c5.
race_linux_riscv64.syso built with LLVM c3c24be13f7928460ca1e2fe613a1146c868854e and Go a21249436b6e1fd47356361d53dc053bbc074f90.
race_linux_s390x.syso built with LLVM 51bfeff0e4b0757ff773da6882f4d538996c9b04 with patch race_linux_s390x.patch and Go a61fd428974822a8c57a2b2840fc237e6711b24d.

```

