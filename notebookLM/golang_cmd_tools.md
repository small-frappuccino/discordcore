# Domain Architecture: cmd/tools

## Layout Topology
```text
cmd/tools/
└── tools.go
```

## Source Stream Aggregation

// === FILE: references/go/src/cmd/tools/tools.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build tools

package tools

// Arrange to vendor the bisect command for use
// by the internal/godebug package test.
import _ "golang.org/x/tools/cmd/bisect"

```

