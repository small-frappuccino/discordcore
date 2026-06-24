# Domain Architecture: plugin

## Layout Topology
```text
plugin/
├── plugin.go
├── plugin_dlopen.go
└── plugin_stubs.go
```

## Source Stream Aggregation

// === FILE: references/go/src/plugin/plugin.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package plugin implements loading and symbol resolution of Go plugins.
//
// A plugin is a Go main package with exported functions and variables that
// has been built with:
//
//	go build -buildmode=plugin
//
// When a plugin is first opened, the init functions of all packages not
// already part of the program are called. The main function is not run.
// A plugin is only initialized once, and cannot be closed.
//
// # Warnings
//
// The ability to dynamically load parts of an application during
// execution, perhaps based on user-defined configuration, may be a
// useful building block in some designs. In particular, because
// applications and dynamically loaded functions can share data
// structures directly, plugins may enable very high-performance
// integration of separate parts.
//
// However, the plugin mechanism has many significant drawbacks that
// should be considered carefully during the design. For example:
//
//   - Plugins are currently supported only on Linux, FreeBSD, and
//     macOS, making them unsuitable for applications intended to be
//     portable.
//
//   - Plugins are poorly supported by the Go race detector. Even simple
//     race conditions may not be automatically detected. See
//     https://go.dev/issue/24245 for more information.
//
//   - Applications that use plugins may require careful configuration
//     to ensure that the various parts of the program be made available
//     in the correct location in the file system (or container image).
//     By contrast, deploying an application consisting of a single static
//     executable is straightforward.
//
//   - Reasoning about program initialization is more difficult when
//     some packages may not be initialized until long after the
//     application has started running.
//
//   - Bugs in applications that load plugins could be exploited by
//     an attacker to load dangerous or untrusted libraries.
//
//   - Runtime crashes are likely to occur unless all parts of the
//     program (the application and all its plugins) are compiled
//     using exactly the same version of the toolchain, the same build
//     tags, and the same values of certain flags and environment
//     variables.
//
//   - Similar crashing problems are likely to arise unless all common
//     dependencies of the application and its plugins are built from
//     exactly the same source code.
//
//   - Together, these restrictions mean that, in practice, the
//     application and its plugins must all be built together by a
//     single person or component of a system. In that case, it may
//     be simpler for that person or component to generate Go source
//     files that blank-import the desired set of plugins and then
//     compile a static executable in the usual way.
//
// For these reasons, many users decide that traditional interprocess
// communication (IPC) mechanisms such as sockets, pipes, remote
// procedure call (RPC), shared memory mappings, or file system
// operations may be more suitable despite the performance overheads.
package plugin

// Plugin is a loaded Go plugin.
type Plugin struct {
	pluginpath string
	err        string        // set if plugin failed to load
	loaded     chan struct{} // closed when loaded
	syms       map[string]any
}

// Open opens a Go plugin.
// If a path has already been opened, then the existing *[Plugin] is returned.
// It is safe for concurrent use by multiple goroutines.
func Open(path string) (*Plugin, error) {
	return open(path)
}

// Lookup searches for a symbol named symName in plugin p.
// A symbol is any exported variable or function.
// It reports an error if the symbol is not found.
// It is safe for concurrent use by multiple goroutines.
func (p *Plugin) Lookup(symName string) (Symbol, error) {
	return lookup(p, symName)
}

// A Symbol is a pointer to a variable or function.
//
// For example, a plugin defined as
//
//	package main
//
//	import "fmt"
//
//	var V int
//
//	func F() { fmt.Printf("Hello, number %d\n", V) }
//
// may be loaded with the [Open] function and then the exported package
// symbols V and F can be accessed
//
//	p, err := plugin.Open("plugin_name.so")
//	if err != nil {
//		panic(err)
//	}
//	v, err := p.Lookup("V")
//	if err != nil {
//		panic(err)
//	}
//	f, err := p.Lookup("F")
//	if err != nil {
//		panic(err)
//	}
//	*v.(*int) = 7
//	f.(func())() // prints "Hello, number 7"
type Symbol any

```

// === FILE: references/go/src/plugin/plugin_dlopen.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (linux && cgo) || (darwin && cgo) || (freebsd && cgo)

package plugin

/*
#cgo linux LDFLAGS: -ldl
#include <dlfcn.h>
#include <limits.h>
#include <stdlib.h>
#include <stdint.h>

#include <stdio.h>

static uintptr_t pluginOpen(const char* path, char** err) {
	void* h = dlopen(path, RTLD_NOW|RTLD_GLOBAL);
	if (h == NULL) {
		*err = (char*)dlerror();
	}
	return (uintptr_t)h;
}

static void* pluginLookup(uintptr_t h, const char* name, char** err) {
	void* r = dlsym((void*)h, name);
	if (r == NULL) {
		*err = (char*)dlerror();
	}
	return r;
}
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"
)

func open(name string) (*Plugin, error) {
	cPath := make([]byte, C.PATH_MAX+1)
	cRelName := make([]byte, len(name)+1)
	copy(cRelName, name)
	if C.realpath(
		(*C.char)(unsafe.Pointer(&cRelName[0])),
		(*C.char)(unsafe.Pointer(&cPath[0]))) == nil {
		return nil, errors.New(`plugin.Open("` + name + `"): realpath failed`)
	}

	filepath := C.GoString((*C.char)(unsafe.Pointer(&cPath[0])))

	pluginsMu.Lock()
	if p := plugins[filepath]; p != nil {
		pluginsMu.Unlock()
		if p.err != "" {
			return nil, errors.New(`plugin.Open("` + name + `"): ` + p.err + ` (previous failure)`)
		}
		<-p.loaded
		return p, nil
	}
	var cErr *C.char
	h := C.pluginOpen((*C.char)(unsafe.Pointer(&cPath[0])), &cErr)
	if h == 0 {
		pluginsMu.Unlock()
		return nil, errors.New(`plugin.Open("` + name + `"): ` + C.GoString(cErr))
	}
	// TODO(crawshaw): look for plugin note, confirm it is a Go plugin
	// and it was built with the correct toolchain.
	if len(name) > 3 && name[len(name)-3:] == ".so" {
		name = name[:len(name)-3]
	}
	if plugins == nil {
		plugins = make(map[string]*Plugin)
	}
	pluginpath, syms, initTasks, errstr := lastmoduleinit()
	if errstr != "" {
		plugins[filepath] = &Plugin{
			pluginpath: pluginpath,
			err:        errstr,
		}
		pluginsMu.Unlock()
		return nil, errors.New(`plugin.Open("` + name + `"): ` + errstr)
	}
	// This function can be called from the init function of a plugin.
	// Drop a placeholder in the map so subsequent opens can wait on it.
	p := &Plugin{
		pluginpath: pluginpath,
		loaded:     make(chan struct{}),
	}
	plugins[filepath] = p
	pluginsMu.Unlock()

	doInit(initTasks)

	// Fill out the value of each plugin symbol.
	updatedSyms := map[string]any{}
	for symName, sym := range syms {
		isFunc := symName[0] == '.'
		if isFunc {
			delete(syms, symName)
			symName = symName[1:]
		}

		fullName := pluginpath + "." + symName
		cname := make([]byte, len(fullName)+1)
		copy(cname, fullName)

		p := C.pluginLookup(h, (*C.char)(unsafe.Pointer(&cname[0])), &cErr)
		if p == nil {
			return nil, errors.New(`plugin.Open("` + name + `"): could not find symbol ` + symName + `: ` + C.GoString(cErr))
		}
		valp := (*[2]unsafe.Pointer)(unsafe.Pointer(&sym))
		if isFunc {
			(*valp)[1] = unsafe.Pointer(&p)
		} else {
			(*valp)[1] = p
		}
		// we can't add to syms during iteration as we'll end up processing
		// some symbols twice with the inability to tell if the symbol is a function
		updatedSyms[symName] = sym
	}
	p.syms = updatedSyms

	close(p.loaded)
	return p, nil
}

func lookup(p *Plugin, symName string) (Symbol, error) {
	if s := p.syms[symName]; s != nil {
		return s, nil
	}
	return nil, errors.New("plugin: symbol " + symName + " not found in plugin " + p.pluginpath)
}

var (
	pluginsMu sync.Mutex
	plugins   map[string]*Plugin
)

// lastmoduleinit is defined in package runtime.
func lastmoduleinit() (pluginpath string, syms map[string]any, inittasks []*initTask, errstr string)

// doInit is defined in package runtime.
//
//go:linkname doInit runtime.doInit
func doInit(t []*initTask)

type initTask struct {
	// fields defined in runtime.initTask. We only handle pointers to an initTask
	// in this package, so the contents are irrelevant.
}

```

// === FILE: references/go/src/plugin/plugin_stubs.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build (!linux && !freebsd && !darwin) || !cgo

package plugin

import "errors"

func lookup(p *Plugin, symName string) (Symbol, error) {
	return nil, errors.New("plugin: not implemented")
}

func open(name string) (*Plugin, error) {
	return nil, errors.New("plugin: not implemented")
}

```

