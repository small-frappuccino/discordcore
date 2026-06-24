# Domain Architecture: sort

## Layout Topology
```text
sort/
├── gen_sort_variants.go
├── search.go
├── slice.go
├── sort.go
├── zsortfunc.go
└── zsortinterface.go
```

## Source Stream Aggregation

// === FILE: references/go/src/sort/gen_sort_variants.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// This program is run via "go generate" (via a directive in sort.go)
// to generate implementation variants of the underlying sorting algorithm.
// When passed the -generic flag it generates generic variants of sorting;
// otherwise it generates the non-generic variants used by the sort package.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"text/template"
)

type Variant struct {
	// Name is the variant name: should be unique among variants.
	Name string

	// Path is the file path into which the generator will emit the code for this
	// variant.
	Path string

	// Package is the package this code will be emitted into.
	Package string

	// Imports is the imports needed for this package.
	Imports string

	// FuncSuffix is appended to all function names in this variant's code. All
	// suffixes should be unique within a package.
	FuncSuffix string

	// DataType is the type of the data parameter of functions in this variant's
	// code.
	DataType string

	// TypeParam is the optional type parameter for the function.
	TypeParam string

	// ExtraParam is an extra parameter to pass to the function. Should begin with
	// ", " to separate from other params.
	ExtraParam string

	// ExtraArg is an extra argument to pass to calls between functions; typically
	// it invokes ExtraParam. Should begin with ", " to separate from other args.
	ExtraArg string

	// Funcs is a map of functions used from within the template. The following
	// functions are expected to exist:
	//
	//    Less (name, i, j):
	//      emits a comparison expression that checks if the value `name` at
	//      index `i` is smaller than at index `j`.
	//
	//    Swap (name, i, j):
	//      emits a statement that performs a data swap between elements `i` and
	//      `j` of the value `name`.
	Funcs template.FuncMap
}

var (
	traditionalVariants = []Variant{
		Variant{
			Name:       "interface",
			Path:       "zsortinterface.go",
			Package:    "sort",
			Imports:    "",
			FuncSuffix: "",
			TypeParam:  "",
			ExtraParam: "",
			ExtraArg:   "",
			DataType:   "Interface",
			Funcs: template.FuncMap{
				"Less": func(name, i, j string) string {
					return fmt.Sprintf("%s.Less(%s, %s)", name, i, j)
				},
				"Swap": func(name, i, j string) string {
					return fmt.Sprintf("%s.Swap(%s, %s)", name, i, j)
				},
			},
		},
		Variant{
			Name:       "func",
			Path:       "zsortfunc.go",
			Package:    "sort",
			Imports:    "",
			FuncSuffix: "_func",
			TypeParam:  "",
			ExtraParam: "",
			ExtraArg:   "",
			DataType:   "lessSwap",
			Funcs: template.FuncMap{
				"Less": func(name, i, j string) string {
					return fmt.Sprintf("%s.Less(%s, %s)", name, i, j)
				},
				"Swap": func(name, i, j string) string {
					return fmt.Sprintf("%s.Swap(%s, %s)", name, i, j)
				},
			},
		},
	}

	genericVariants = []Variant{
		Variant{
			Name:       "generic_ordered",
			Path:       "zsortordered.go",
			Package:    "slices",
			Imports:    "import \"cmp\"\n",
			FuncSuffix: "Ordered",
			TypeParam:  "[E cmp.Ordered]",
			ExtraParam: "",
			ExtraArg:   "",
			DataType:   "[]E",
			Funcs: template.FuncMap{
				"Less": func(name, i, j string) string {
					return fmt.Sprintf("cmp.Less(%s[%s], %s[%s])", name, i, name, j)
				},
				"Swap": func(name, i, j string) string {
					return fmt.Sprintf("%s[%s], %s[%s] = %s[%s], %s[%s]", name, i, name, j, name, j, name, i)
				},
			},
		},
		Variant{
			Name:       "generic_func",
			Path:       "zsortanyfunc.go",
			Package:    "slices",
			FuncSuffix: "CmpFunc",
			TypeParam:  "[E any]",
			ExtraParam: ", cmp func(a, b E) int",
			ExtraArg:   ", cmp",
			DataType:   "[]E",
			Funcs: template.FuncMap{
				"Less": func(name, i, j string) string {
					return fmt.Sprintf("(cmp(%s[%s], %s[%s]) < 0)", name, i, name, j)
				},
				"Swap": func(name, i, j string) string {
					return fmt.Sprintf("%s[%s], %s[%s] = %s[%s], %s[%s]", name, i, name, j, name, j, name, i)
				},
			},
		},
	}

	expVariants = []Variant{
		Variant{
			Name:       "exp_ordered",
			Path:       "zsortordered.go",
			Package:    "slices",
			Imports:    "import \"golang.org/x/exp/constraints\"\n",
			FuncSuffix: "Ordered",
			TypeParam:  "[E constraints.Ordered]",
			ExtraParam: "",
			ExtraArg:   "",
			DataType:   "[]E",
			Funcs: template.FuncMap{
				"Less": func(name, i, j string) string {
					return fmt.Sprintf("cmpLess(%s[%s], %s[%s])", name, i, name, j)
				},
				"Swap": func(name, i, j string) string {
					return fmt.Sprintf("%s[%s], %s[%s] = %s[%s], %s[%s]", name, i, name, j, name, j, name, i)
				},
			},
		},
		Variant{
			Name:       "exp_func",
			Path:       "zsortanyfunc.go",
			Package:    "slices",
			FuncSuffix: "CmpFunc",
			TypeParam:  "[E any]",
			ExtraParam: ", cmp func(a, b E) int",
			ExtraArg:   ", cmp",
			DataType:   "[]E",
			Funcs: template.FuncMap{
				"Less": func(name, i, j string) string {
					return fmt.Sprintf("(cmp(%s[%s], %s[%s]) < 0)", name, i, name, j)
				},
				"Swap": func(name, i, j string) string {
					return fmt.Sprintf("%s[%s], %s[%s] = %s[%s], %s[%s]", name, i, name, j, name, j, name, i)
				},
			},
		},
	}
)

func main() {
	genGeneric := flag.Bool("generic", false, "generate generic versions")
	genExp := flag.Bool("exp", false, "generate x/exp/slices versions")
	flag.Parse()

	var variants []Variant
	if *genExp {
		variants = expVariants
	} else if *genGeneric {
		variants = genericVariants
	} else {
		variants = traditionalVariants
	}
	for i := range variants {
		generate(&variants[i])
	}
}

// generate generates the code for variant `v` into a file named by `v.Path`.
func generate(v *Variant) {
	// Parse templateCode anew for each variant because Parse requires Funcs to be
	// registered, and it helps type-check the funcs.
	tmpl, err := template.New("gen").Funcs(v.Funcs).Parse(templateCode)
	if err != nil {
		log.Fatal("template Parse:", err)
	}

	var out bytes.Buffer
	err = tmpl.Execute(&out, v)
	if err != nil {
		log.Fatal("template Execute:", err)
	}

	formatted, err := format.Source(out.Bytes())
	if err != nil {
		log.Fatal("format:", err)
	}

	if err := os.WriteFile(v.Path, formatted, 0644); err != nil {
		log.Fatal("WriteFile:", err)
	}
}

var templateCode = `// Code generated by gen_sort_variants.go; DO NOT EDIT.

// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package {{.Package}}

{{.Imports}}

// insertionSort{{.FuncSuffix}} sorts data[a:b] using insertion sort.
func insertionSort{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b int {{.ExtraParam}}) {
	for i := a + 1; i < b; i++ {
		for j := i; j > a && {{Less "data" "j" "j-1"}}; j-- {
			{{Swap "data" "j" "j-1"}}
		}
	}
}

// siftDown{{.FuncSuffix}} implements the heap property on data[lo:hi].
// first is an offset into the array where the root of the heap lies.
func siftDown{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, lo, hi, first int {{.ExtraParam}}) {
	root := lo
	for {
		child := 2*root + 1
		if child >= hi {
			break
		}
		if child+1 < hi && {{Less "data" "first+child" "first+child+1"}} {
			child++
		}
		if !{{Less "data" "first+root" "first+child"}} {
			return
		}
		{{Swap "data" "first+root" "first+child"}}
		root = child
	}
}

func heapSort{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b int {{.ExtraParam}}) {
	first := a
	lo := 0
	hi := b - a

	// Build heap with greatest element at top.
	for i := (hi - 1) / 2; i >= 0; i-- {
		siftDown{{.FuncSuffix}}(data, i, hi, first {{.ExtraArg}})
	}

	// Pop elements, largest first, into end of data.
	for i := hi - 1; i >= 0; i-- {
		{{Swap "data" "first" "first+i"}}
		siftDown{{.FuncSuffix}}(data, lo, i, first {{.ExtraArg}})
	}
}

// pdqsort{{.FuncSuffix}} sorts data[a:b].
// The algorithm based on pattern-defeating quicksort(pdqsort), but without the optimizations from BlockQuicksort.
// pdqsort paper: https://arxiv.org/pdf/2106.05123.pdf
// C++ implementation: https://github.com/orlp/pdqsort
// Rust implementation: https://docs.rs/pdqsort/latest/pdqsort/
// limit is the number of allowed bad (very unbalanced) pivots before falling back to heapsort.
func pdqsort{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b, limit int {{.ExtraParam}}) {
	const maxInsertion = 12

	var (
		wasBalanced    = true // whether the last partitioning was reasonably balanced
		wasPartitioned = true // whether the slice was already partitioned
	)

	for {
		length := b - a

		if length <= maxInsertion {
			insertionSort{{.FuncSuffix}}(data, a, b {{.ExtraArg}})
			return
		}

		// Fall back to heapsort if too many bad choices were made.
		if limit == 0 {
			heapSort{{.FuncSuffix}}(data, a, b {{.ExtraArg}})
			return
		}

		// If the last partitioning was imbalanced, we need to breaking patterns.
		if !wasBalanced {
			breakPatterns{{.FuncSuffix}}(data, a, b {{.ExtraArg}})
			limit--
		}

		pivot, hint := choosePivot{{.FuncSuffix}}(data, a, b {{.ExtraArg}})
		if hint == decreasingHint {
			reverseRange{{.FuncSuffix}}(data, a, b {{.ExtraArg}})
			// The chosen pivot was pivot-a elements after the start of the array.
			// After reversing it is pivot-a elements before the end of the array.
			// The idea came from Rust's implementation.
			pivot = (b - 1) - (pivot - a)
			hint = increasingHint
		}

		// The slice is likely already sorted.
		if wasBalanced && wasPartitioned && hint == increasingHint {
			if partialInsertionSort{{.FuncSuffix}}(data, a, b {{.ExtraArg}}) {
				return
			}
		}

		// Probably the slice contains many duplicate elements, partition the slice into
		// elements equal to and elements greater than the pivot.
		if a > 0 && !{{Less "data" "a-1" "pivot"}} {
			mid := partitionEqual{{.FuncSuffix}}(data, a, b, pivot {{.ExtraArg}})
			a = mid
			continue
		}

		mid, alreadyPartitioned := partition{{.FuncSuffix}}(data, a, b, pivot {{.ExtraArg}})
		wasPartitioned = alreadyPartitioned

		leftLen, rightLen := mid-a, b-mid
		balanceThreshold := length / 8
		if leftLen < rightLen {
			wasBalanced = leftLen >= balanceThreshold
			pdqsort{{.FuncSuffix}}(data, a, mid, limit {{.ExtraArg}})
			a = mid + 1
		} else {
			wasBalanced = rightLen >= balanceThreshold
			pdqsort{{.FuncSuffix}}(data, mid+1, b, limit {{.ExtraArg}})
			b = mid
		}
	}
}

// partition{{.FuncSuffix}} does one quicksort partition.
// Let p = data[pivot]
// Moves elements in data[a:b] around, so that data[i]<p and data[j]>=p for i<newpivot and j>newpivot.
// On return, data[newpivot] = p
func partition{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b, pivot int {{.ExtraParam}}) (newpivot int, alreadyPartitioned bool) {
	{{Swap "data" "a" "pivot"}}
	i, j := a+1, b-1 // i and j are inclusive of the elements remaining to be partitioned

	for i <= j && {{Less "data" "i" "a"}} {
		i++
	}
	for i <= j && !{{Less "data" "j" "a"}} {
		j--
	}
	if i > j {
		{{Swap "data" "j" "a"}}
		return j, true
	}
	{{Swap "data" "i" "j"}}
	i++
	j--

	for {
		for i <= j && {{Less "data" "i" "a"}} {
			i++
		}
		for i <= j && !{{Less "data" "j" "a"}} {
			j--
		}
		if i > j {
			break
		}
		{{Swap "data" "i" "j"}}
		i++
		j--
	}
	{{Swap "data" "j" "a"}}
	return j, false
}

// partitionEqual{{.FuncSuffix}} partitions data[a:b] into elements equal to data[pivot] followed by elements greater than data[pivot].
// It assumed that data[a:b] does not contain elements smaller than the data[pivot].
func partitionEqual{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b, pivot int {{.ExtraParam}}) (newpivot int) {
	{{Swap "data" "a" "pivot"}}
	i, j := a+1, b-1 // i and j are inclusive of the elements remaining to be partitioned

	for {
		for i <= j && !{{Less "data" "a" "i"}} {
			i++
		}
		for i <= j && {{Less "data" "a" "j"}} {
			j--
		}
		if i > j {
			break
		}
		{{Swap "data" "i" "j"}}
		i++
		j--
	}
	return i
}

// partialInsertionSort{{.FuncSuffix}} partially sorts a slice, returns true if the slice is sorted at the end.
func partialInsertionSort{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b int {{.ExtraParam}}) bool {
	const (
		maxSteps         = 5  // maximum number of adjacent out-of-order pairs that will get shifted
		shortestShifting = 50 // don't shift any elements on short arrays
	)
	i := a + 1
	for j := 0; j < maxSteps; j++ {
		for i < b && !{{Less "data" "i" "i-1"}} {
			i++
		}

		if i == b {
			return true
		}

		if b-a < shortestShifting {
			return false
		}

		{{Swap "data" "i" "i-1"}}

		// Shift the smaller one to the left.
		if i-a >= 2 {
			for j := i - 1; j >= 1; j-- {
				if !{{Less "data" "j" "j-1"}} {
					break
				}
				{{Swap "data" "j" "j-1"}}
			}
		}
		// Shift the greater one to the right.
		if b-i >= 2 {
			for j := i + 1; j < b; j++ {
				if !{{Less "data" "j" "j-1"}} {
					break
				}
				{{Swap "data" "j" "j-1"}}
			}
		}
	}
	return false
}

// breakPatterns{{.FuncSuffix}} scatters some elements around in an attempt to break some patterns
// that might cause imbalanced partitions in quicksort.
func breakPatterns{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b int {{.ExtraParam}}) {
	length := b - a
	if length >= 8 {
		random := xorshift(length)
		modulus := nextPowerOfTwo(length)

		for idx := a + (length/4)*2 - 1; idx <= a + (length/4)*2 + 1; idx++ {
			other := int(uint(random.Next()) & (modulus - 1))
			if other >= length {
				other -= length
			}
			{{Swap "data" "idx" "a+other"}}
		}
	}
}

// choosePivot{{.FuncSuffix}} chooses a pivot in data[a:b].
//
// [0,8): chooses a static pivot.
// [8,shortestNinther): uses the simple median-of-three method.
// [shortestNinther,∞): uses the Tukey ninther method.
func choosePivot{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b int {{.ExtraParam}}) (pivot int, hint sortedHint) {
	const (
		shortestNinther = 50
		maxSwaps        = 4 * 3
	)

	l := b - a

	var (
		swaps int
		i     = a + l/4*1
		j     = a + l/4*2
		k     = a + l/4*3
	)

	if l >= 8 {
		if l >= shortestNinther {
			// Tukey ninther method, the idea came from Rust's implementation.
			i = medianAdjacent{{.FuncSuffix}}(data, i, &swaps {{.ExtraArg}})
			j = medianAdjacent{{.FuncSuffix}}(data, j, &swaps {{.ExtraArg}})
			k = medianAdjacent{{.FuncSuffix}}(data, k, &swaps {{.ExtraArg}})
		}
		// Find the median among i, j, k and stores it into j.
		j = median{{.FuncSuffix}}(data, i, j, k, &swaps {{.ExtraArg}})
	}

	switch swaps {
	case 0:
		return j, increasingHint
	case maxSwaps:
		return j, decreasingHint
	default:
		return j, unknownHint
	}
}

// order2{{.FuncSuffix}} returns x,y where data[x] <= data[y], where x,y=a,b or x,y=b,a.
func order2{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b int, swaps *int {{.ExtraParam}}) (int, int) {
	if {{Less "data" "b" "a"}} {
		*swaps++
		return b, a
	}
	return a, b
}

// median{{.FuncSuffix}} returns x where data[x] is the median of data[a],data[b],data[c], where x is a, b, or c.
func median{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b, c int, swaps *int {{.ExtraParam}}) int {
	a, b = order2{{.FuncSuffix}}(data, a, b, swaps {{.ExtraArg}})
	b, c = order2{{.FuncSuffix}}(data, b, c, swaps {{.ExtraArg}})
	a, b = order2{{.FuncSuffix}}(data, a, b, swaps {{.ExtraArg}})
	return b
}

// medianAdjacent{{.FuncSuffix}} finds the median of data[a - 1], data[a], data[a + 1] and stores the index into a.
func medianAdjacent{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a int, swaps *int {{.ExtraParam}}) int {
	return median{{.FuncSuffix}}(data, a-1, a, a+1, swaps {{.ExtraArg}})
}

func reverseRange{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b int {{.ExtraParam}}) {
	i := a
	j := b - 1
	for i < j {
		{{Swap "data" "i" "j"}}
		i++
		j--
	}
}

func swapRange{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, b, n int {{.ExtraParam}}) {
	for i := 0; i < n; i++ {
		{{Swap "data" "a+i" "b+i"}}
	}
}

func stable{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, n int {{.ExtraParam}}) {
	blockSize := 20 // must be > 0
	a, b := 0, blockSize
	for b <= n {
		insertionSort{{.FuncSuffix}}(data, a, b {{.ExtraArg}})
		a = b
		b += blockSize
	}
	insertionSort{{.FuncSuffix}}(data, a, n {{.ExtraArg}})

	for blockSize < n {
		a, b = 0, 2*blockSize
		for b <= n {
			symMerge{{.FuncSuffix}}(data, a, a+blockSize, b {{.ExtraArg}})
			a = b
			b += 2 * blockSize
		}
		if m := a + blockSize; m < n {
			symMerge{{.FuncSuffix}}(data, a, m, n {{.ExtraArg}})
		}
		blockSize *= 2
	}
}

// symMerge{{.FuncSuffix}} merges the two sorted subsequences data[a:m] and data[m:b] using
// the SymMerge algorithm from Pok-Son Kim and Arne Kutzner, "Stable Minimum
// Storage Merging by Symmetric Comparisons", in Susanne Albers and Tomasz
// Radzik, editors, Algorithms - ESA 2004, volume 3221 of Lecture Notes in
// Computer Science, pages 714-723. Springer, 2004.
//
// Let M = m-a and N = b-n. Wolog M < N.
// The recursion depth is bound by ceil(log(N+M)).
// The algorithm needs O(M*log(N/M + 1)) calls to data.Less.
// The algorithm needs O((M+N)*log(M)) calls to data.Swap.
//
// The paper gives O((M+N)*log(M)) as the number of assignments assuming a
// rotation algorithm which uses O(M+N+gcd(M+N)) assignments. The argumentation
// in the paper carries through for Swap operations, especially as the block
// swapping rotate uses only O(M+N) Swaps.
//
// symMerge assumes non-degenerate arguments: a < m && m < b.
// Having the caller check this condition eliminates many leaf recursion calls,
// which improves performance.
func symMerge{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, m, b int {{.ExtraParam}}) {
	// Avoid unnecessary recursions of symMerge
	// by direct insertion of data[a] into data[m:b]
	// if data[a:m] only contains one element.
	if m-a == 1 {
		// Use binary search to find the lowest index i
		// such that data[i] >= data[a] for m <= i < b.
		// Exit the search loop with i == b in case no such index exists.
		i := m
		j := b
		for i < j {
			h := int(uint(i+j) >> 1)
			if {{Less "data" "h" "a"}} {
				i = h + 1
			} else {
				j = h
			}
		}
		// Swap values until data[a] reaches the position before i.
		for k := a; k < i-1; k++ {
			{{Swap "data" "k" "k+1"}}
		}
		return
	}

	// Avoid unnecessary recursions of symMerge
	// by direct insertion of data[m] into data[a:m]
	// if data[m:b] only contains one element.
	if b-m == 1 {
		// Use binary search to find the lowest index i
		// such that data[i] > data[m] for a <= i < m.
		// Exit the search loop with i == m in case no such index exists.
		i := a
		j := m
		for i < j {
			h := int(uint(i+j) >> 1)
			if !{{Less "data" "m" "h"}} {
				i = h + 1
			} else {
				j = h
			}
		}
		// Swap values until data[m] reaches the position i.
		for k := m; k > i; k-- {
			{{Swap "data" "k" "k-1"}}
		}
		return
	}

	mid := int(uint(a+b) >> 1)
	n := mid + m
	var start, r int
	if m > mid {
		start = n - b
		r = mid
	} else {
		start = a
		r = m
	}
	p := n - 1

	for start < r {
		c := int(uint(start+r) >> 1)
		if !{{Less "data" "p-c" "c"}} {
			start = c + 1
		} else {
			r = c
		}
	}

	end := n - start
	if start < m && m < end {
		rotate{{.FuncSuffix}}(data, start, m, end {{.ExtraArg}})
	}
	if a < start && start < mid {
		symMerge{{.FuncSuffix}}(data, a, start, mid {{.ExtraArg}})
	}
	if mid < end && end < b {
		symMerge{{.FuncSuffix}}(data, mid, end, b {{.ExtraArg}})
	}
}

// rotate{{.FuncSuffix}} rotates two consecutive blocks u = data[a:m] and v = data[m:b] in data:
// Data of the form 'x u v y' is changed to 'x v u y'.
// rotate performs at most b-a many calls to data.Swap,
// and it assumes non-degenerate arguments: a < m && m < b.
func rotate{{.FuncSuffix}}{{.TypeParam}}(data {{.DataType}}, a, m, b int {{.ExtraParam}}) {
	i := m - a
	j := b - m

	for i != j {
		if i > j {
			swapRange{{.FuncSuffix}}(data, m-i, m, j {{.ExtraArg}})
			i -= j
		} else {
			swapRange{{.FuncSuffix}}(data, m-i, m+j-i, i {{.ExtraArg}})
			j -= i
		}
	}
	// i == j
	swapRange{{.FuncSuffix}}(data, m-i, m, i {{.ExtraArg}})
}
`

```

// === FILE: references/go/src/sort/search.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements binary search.

package sort

// Search uses binary search to find and return the smallest index i
// in [0, n) at which f(i) is true, assuming that on the range [0, n),
// f(i) == true implies f(i+1) == true. That is, Search requires that
// f is false for some (possibly empty) prefix of the input range [0, n)
// and then true for the (possibly empty) remainder; Search returns
// the first true index. If there is no such index, Search returns n.
// (Note that the "not found" return value is not -1 as in, for instance,
// strings.Index.)
// Search calls f(i) only for i in the range [0, n).
//
// A common use of Search is to find the index i for a value x in
// a sorted, indexable data structure such as an array or slice.
// In this case, the argument f, typically a closure, captures the value
// to be searched for, and how the data structure is indexed and
// ordered.
//
// For instance, given a slice data sorted in ascending order,
// the call Search(len(data), func(i int) bool { return data[i] >= 23 })
// returns the smallest index i such that data[i] >= 23. If the caller
// wants to find whether 23 is in the slice, it must test data[i] == 23
// separately.
//
// Searching data sorted in descending order would use the <=
// operator instead of the >= operator.
//
// To complete the example above, the following code tries to find the value
// x in an integer slice data sorted in ascending order:
//
//	x := 23
//	i := sort.Search(len(data), func(i int) bool { return data[i] >= x })
//	if i < len(data) && data[i] == x {
//		// x is present at data[i]
//	} else {
//		// x is not present in data,
//		// but i is the index where it would be inserted.
//	}
//
// As a more whimsical example, this program guesses your number:
//
//	func GuessingGame() {
//		var s string
//		fmt.Printf("Pick an integer from 0 to 100.\n")
//		answer := sort.Search(100, func(i int) bool {
//			fmt.Printf("Is your number <= %d? ", i)
//			fmt.Scanf("%s", &s)
//			return s != "" && s[0] == 'y'
//		})
//		fmt.Printf("Your number is %d.\n", answer)
//	}
func Search(n int, f func(int) bool) int {
	// Define f(-1) == false and f(n) == true.
	// Invariant: f(i-1) == false, f(j) == true.
	i, j := 0, n
	for i < j {
		h := int(uint(i+j) >> 1) // avoid overflow when computing h
		// i ≤ h < j
		if !f(h) {
			i = h + 1 // preserves f(i-1) == false
		} else {
			j = h // preserves f(j) == true
		}
	}
	// i == j, f(i-1) == false, and f(j) (= f(i)) == true  =>  answer is i.
	return i
}

// Find uses binary search to find and return the smallest index i in [0, n)
// at which cmp(i) <= 0. If there is no such index i, Find returns i = n.
// The found result is true if i < n and cmp(i) == 0.
// Find calls cmp(i) only for i in the range [0, n).
//
// To permit binary search, Find requires that cmp(i) > 0 for a leading
// prefix of the range, cmp(i) == 0 in the middle, and cmp(i) < 0 for
// the final suffix of the range. (Each subrange could be empty.)
// The usual way to establish this condition is to interpret cmp(i)
// as a comparison of a desired target value t against entry i in an
// underlying indexed data structure x, returning <0, 0, and >0
// when t < x[i], t == x[i], and t > x[i], respectively.
//
// For example, to look for a particular string in a sorted, random-access
// list of strings:
//
//	i, found := sort.Find(x.Len(), func(i int) int {
//	    return strings.Compare(target, x.At(i))
//	})
//	if found {
//	    fmt.Printf("found %s at entry %d\n", target, i)
//	} else {
//	    fmt.Printf("%s not found, would insert at %d", target, i)
//	}
func Find(n int, cmp func(int) int) (i int, found bool) {
	// The invariants here are similar to the ones in Search.
	// Define cmp(-1) > 0 and cmp(n) <= 0
	// Invariant: cmp(i-1) > 0, cmp(j) <= 0
	i, j := 0, n
	for i < j {
		h := int(uint(i+j) >> 1) // avoid overflow when computing h
		// i ≤ h < j
		if cmp(h) > 0 {
			i = h + 1 // preserves cmp(i-1) > 0
		} else {
			j = h // preserves cmp(j) <= 0
		}
	}
	// i == j, cmp(i-1) > 0 and cmp(j) <= 0
	return i, i < n && cmp(i) == 0
}

// Convenience wrappers for common cases.

// SearchInts searches for x in a sorted slice of ints and returns the index
// as specified by [Search]. The return value is the index to insert x if x is
// not present (it could be len(a)).
// The slice must be sorted in ascending order.
func SearchInts(a []int, x int) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// SearchFloat64s searches for x in a sorted slice of float64s and returns the index
// as specified by [Search]. The return value is the index to insert x if x is not
// present (it could be len(a)).
// The slice must be sorted in ascending order.
func SearchFloat64s(a []float64, x float64) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// SearchStrings searches for x in a sorted slice of strings and returns the index
// as specified by Search. The return value is the index to insert x if x is not
// present (it could be len(a)).
// The slice must be sorted in ascending order.
func SearchStrings(a []string, x string) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// Search returns the result of applying [SearchInts] to the receiver and x.
func (p IntSlice) Search(x int) int { return SearchInts(p, x) }

// Search returns the result of applying [SearchFloat64s] to the receiver and x.
func (p Float64Slice) Search(x float64) int { return SearchFloat64s(p, x) }

// Search returns the result of applying [SearchStrings] to the receiver and x.
func (p StringSlice) Search(x string) int { return SearchStrings(p, x) }

```

// === FILE: references/go/src/sort/slice.go ===
```go
// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sort

import (
	"internal/reflectlite"
	"math/bits"
)

// Slice sorts the slice x given the provided less function.
// It panics if x is not a slice.
//
// The sort is not guaranteed to be stable: equal elements
// may be reversed from their original order.
// For a stable sort, use [SliceStable].
//
// The less function must satisfy the same requirements as
// the Interface type's Less method.
//
// Note: in many situations, the newer [slices.SortFunc] function is more
// ergonomic and runs faster.
func Slice(x any, less func(i, j int) bool) {
	rv := reflectlite.ValueOf(x)
	swap := reflectlite.Swapper(x)
	length := rv.Len()
	limit := bits.Len(uint(length))
	pdqsort_func(lessSwap{less, swap}, 0, length, limit)
}

// SliceStable sorts the slice x using the provided less
// function, keeping equal elements in their original order.
// It panics if x is not a slice.
//
// The less function must satisfy the same requirements as
// the Interface type's Less method.
//
// Note: in many situations, the newer [slices.SortStableFunc] function is more
// ergonomic and runs faster.
func SliceStable(x any, less func(i, j int) bool) {
	rv := reflectlite.ValueOf(x)
	swap := reflectlite.Swapper(x)
	stable_func(lessSwap{less, swap}, rv.Len())
}

// SliceIsSorted reports whether the slice x is sorted according to the provided less function.
// It panics if x is not a slice.
//
// Note: in many situations, the newer [slices.IsSortedFunc] function is more
// ergonomic and runs faster.
func SliceIsSorted(x any, less func(i, j int) bool) bool {
	rv := reflectlite.ValueOf(x)
	n := rv.Len()
	for i := n - 1; i > 0; i-- {
		if less(i, i-1) {
			return false
		}
	}
	return true
}

```

// === FILE: references/go/src/sort/sort.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run gen_sort_variants.go

// Package sort provides primitives for sorting slices and user-defined collections.
package sort

import (
	"math/bits"
	"slices"
)

// An implementation of Interface can be sorted by the routines in this package.
// The methods refer to elements of the underlying collection by integer index.
type Interface interface {
	// Len is the number of elements in the collection.
	Len() int

	// Less reports whether the element with index i
	// must sort before the element with index j.
	//
	// If both Less(i, j) and Less(j, i) are false,
	// then the elements at index i and j are considered equal.
	// Sort may place equal elements in any order in the final result,
	// while Stable preserves the original input order of equal elements.
	//
	// Less must describe a [Strict Weak Ordering]. For example:
	//  - if both Less(i, j) and Less(j, k) are true, then Less(i, k) must be true as well.
	//  - if both Less(i, j) and Less(j, k) are false, then Less(i, k) must be false as well.
	//
	// Note that floating-point comparison (the < operator on float32 or float64 values)
	// is not a strict weak ordering when not-a-number (NaN) values are involved.
	// See Float64Slice.Less for a correct implementation for floating-point values.
	//
	// [Strict Weak Ordering]: https://en.wikipedia.org/wiki/Weak_ordering#Strict_weak_orderings
	Less(i, j int) bool

	// Swap swaps the elements with indexes i and j.
	Swap(i, j int)
}

// Sort sorts data in ascending order as determined by the Less method.
// It makes one call to data.Len to determine n and O(n*log(n)) calls to
// data.Less and data.Swap. The sort is not guaranteed to be stable.
//
// Note: in many situations, the newer [slices.SortFunc] function is more
// ergonomic and runs faster.
func Sort(data Interface) {
	n := data.Len()
	if n <= 1 {
		return
	}
	limit := bits.Len(uint(n))
	pdqsort(data, 0, n, limit)
}

type sortedHint int // hint for pdqsort when choosing the pivot

const (
	unknownHint sortedHint = iota
	increasingHint
	decreasingHint
)

// xorshift paper: https://www.jstatsoft.org/article/view/v008i14/xorshift.pdf
type xorshift uint64

func (r *xorshift) Next() uint64 {
	*r ^= *r << 13
	*r ^= *r >> 7
	*r ^= *r << 17
	return uint64(*r)
}

func nextPowerOfTwo(length int) uint {
	shift := uint(bits.Len(uint(length)))
	return uint(1 << shift)
}

// lessSwap is a pair of Less and Swap function for use with the
// auto-generated func-optimized variant of sort.go in
// zfuncversion.go.
type lessSwap struct {
	Less func(i, j int) bool
	Swap func(i, j int)
}

type reverse struct {
	// This embedded Interface permits Reverse to use the methods of
	// another Interface implementation.
	Interface
}

// Less returns the opposite of the embedded implementation's Less method.
func (r reverse) Less(i, j int) bool {
	return r.Interface.Less(j, i)
}

// Reverse returns the reverse order for data.
func Reverse(data Interface) Interface {
	return &reverse{data}
}

// IsSorted reports whether data is sorted.
//
// Note: in many situations, the newer [slices.IsSortedFunc] function is more
// ergonomic and runs faster.
func IsSorted(data Interface) bool {
	n := data.Len()
	for i := n - 1; i > 0; i-- {
		if data.Less(i, i-1) {
			return false
		}
	}
	return true
}

// Convenience types for common cases

// IntSlice attaches the methods of Interface to []int, sorting in increasing order.
type IntSlice []int

func (x IntSlice) Len() int           { return len(x) }
func (x IntSlice) Less(i, j int) bool { return x[i] < x[j] }
func (x IntSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// Sort is a convenience method: x.Sort() calls Sort(x).
func (x IntSlice) Sort() { Sort(x) }

// Float64Slice implements Interface for a []float64, sorting in increasing order,
// with not-a-number (NaN) values ordered before other values.
type Float64Slice []float64

func (x Float64Slice) Len() int { return len(x) }

// Less reports whether x[i] should be ordered before x[j], as required by the sort Interface.
// Note that floating-point comparison by itself is not a transitive relation: it does not
// report a consistent ordering for not-a-number (NaN) values.
// This implementation of Less places NaN values before any others, by using:
//
//	x[i] < x[j] || (math.IsNaN(x[i]) && !math.IsNaN(x[j]))
func (x Float64Slice) Less(i, j int) bool { return x[i] < x[j] || (isNaN(x[i]) && !isNaN(x[j])) }
func (x Float64Slice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// isNaN is a copy of math.IsNaN to avoid a dependency on the math package.
func isNaN(f float64) bool {
	return f != f
}

// Sort is a convenience method: x.Sort() calls Sort(x).
func (x Float64Slice) Sort() { Sort(x) }

// StringSlice attaches the methods of Interface to []string, sorting in increasing order.
type StringSlice []string

func (x StringSlice) Len() int           { return len(x) }
func (x StringSlice) Less(i, j int) bool { return x[i] < x[j] }
func (x StringSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// Sort is a convenience method: x.Sort() calls Sort(x).
func (x StringSlice) Sort() { Sort(x) }

// Convenience wrappers for common cases

// Ints sorts a slice of ints in increasing order.
//
// Note: as of Go 1.22, this function simply calls [slices.Sort].
func Ints(x []int) { slices.Sort(x) }

// Float64s sorts a slice of float64s in increasing order.
// Not-a-number (NaN) values are ordered before other values.
//
// Note: as of Go 1.22, this function simply calls [slices.Sort].
func Float64s(x []float64) { slices.Sort(x) }

// Strings sorts a slice of strings in increasing order.
//
// Note: as of Go 1.22, this function simply calls [slices.Sort].
func Strings(x []string) { slices.Sort(x) }

// IntsAreSorted reports whether the slice x is sorted in increasing order.
//
// Note: as of Go 1.22, this function simply calls [slices.IsSorted].
func IntsAreSorted(x []int) bool { return slices.IsSorted(x) }

// Float64sAreSorted reports whether the slice x is sorted in increasing order,
// with not-a-number (NaN) values before any other values.
//
// Note: as of Go 1.22, this function simply calls [slices.IsSorted].
func Float64sAreSorted(x []float64) bool { return slices.IsSorted(x) }

// StringsAreSorted reports whether the slice x is sorted in increasing order.
//
// Note: as of Go 1.22, this function simply calls [slices.IsSorted].
func StringsAreSorted(x []string) bool { return slices.IsSorted(x) }

// Notes on stable sorting:
// The used algorithms are simple and provably correct on all input and use
// only logarithmic additional stack space. They perform well if compared
// experimentally to other stable in-place sorting algorithms.
//
// Remarks on other algorithms evaluated:
//  - GCC's 4.6.3 stable_sort with merge_without_buffer from libstdc++:
//    Not faster.
//  - GCC's __rotate for block rotations: Not faster.
//  - "Practical in-place mergesort" from  Jyrki Katajainen, Tomi A. Pasanen
//    and Jukka Teuhola; Nordic Journal of Computing 3,1 (1996), 27-40:
//    The given algorithms are in-place, number of Swap and Assignments
//    grow as n log n but the algorithm is not stable.
//  - "Fast Stable In-Place Sorting with O(n) Data Moves" J.I. Munro and
//    V. Raman in Algorithmica (1996) 16, 115-160:
//    This algorithm either needs additional 2n bits or works only if there
//    are enough different elements available to encode some permutations
//    which have to be undone later (so not stable on any input).
//  - All the optimal in-place sorting/merging algorithms I found are either
//    unstable or rely on enough different elements in each step to encode the
//    performed block rearrangements. See also "In-Place Merging Algorithms",
//    Denham Coates-Evely, Department of Computer Science, Kings College,
//    January 2004 and the references in there.
//  - Often "optimal" algorithms are optimal in the number of assignments
//    but Interface has only Swap as operation.

// Stable sorts data in ascending order as determined by the Less method,
// while keeping the original order of equal elements.
//
// It makes one call to data.Len to determine n, O(n*log(n)) calls to
// data.Less and O(n*log(n)*log(n)) calls to data.Swap.
//
// Note: in many situations, the newer slices.SortStableFunc function is more
// ergonomic and runs faster.
func Stable(data Interface) {
	stable(data, data.Len())
}

/*
Complexity of Stable Sorting


Complexity of block swapping rotation

Each Swap puts one new element into its correct, final position.
Elements which reach their final position are no longer moved.
Thus block swapping rotation needs |u|+|v| calls to Swaps.
This is best possible as each element might need a move.

Pay attention when comparing to other optimal algorithms which
typically count the number of assignments instead of swaps:
E.g. the optimal algorithm of Dudzinski and Dydek for in-place
rotations uses O(u + v + gcd(u,v)) assignments which is
better than our O(3 * (u+v)) as gcd(u,v) <= u.


Stable sorting by SymMerge and BlockSwap rotations

SymMerg complexity for same size input M = N:
Calls to Less:  O(M*log(N/M+1)) = O(N*log(2)) = O(N)
Calls to Swap:  O((M+N)*log(M)) = O(2*N*log(N)) = O(N*log(N))

(The following argument does not fuzz over a missing -1 or
other stuff which does not impact the final result).

Let n = data.Len(). Assume n = 2^k.

Plain merge sort performs log(n) = k iterations.
On iteration i the algorithm merges 2^(k-i) blocks, each of size 2^i.

Thus iteration i of merge sort performs:
Calls to Less  O(2^(k-i) * 2^i) = O(2^k) = O(2^log(n)) = O(n)
Calls to Swap  O(2^(k-i) * 2^i * log(2^i)) = O(2^k * i) = O(n*i)

In total k = log(n) iterations are performed; so in total:
Calls to Less O(log(n) * n)
Calls to Swap O(n + 2*n + 3*n + ... + (k-1)*n + k*n)
   = O((k/2) * k * n) = O(n * k^2) = O(n * log^2(n))


Above results should generalize to arbitrary n = 2^k + p
and should not be influenced by the initial insertion sort phase:
Insertion sort is O(n^2) on Swap and Less, thus O(bs^2) per block of
size bs at n/bs blocks:  O(bs*n) Swaps and Less during insertion sort.
Merge sort iterations start at i = log(bs). With t = log(bs) constant:
Calls to Less O((log(n)-t) * n + bs*n) = O(log(n)*n + (bs-t)*n)
   = O(n * log(n))
Calls to Swap O(n * log^2(n) - (t^2+t)/2*n) = O(n * log^2(n))

*/

```

// === FILE: references/go/src/sort/zsortfunc.go ===
```go
// Code generated by gen_sort_variants.go; DO NOT EDIT.

// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sort

// insertionSort_func sorts data[a:b] using insertion sort.
func insertionSort_func(data lessSwap, a, b int) {
	for i := a + 1; i < b; i++ {
		for j := i; j > a && data.Less(j, j-1); j-- {
			data.Swap(j, j-1)
		}
	}
}

// siftDown_func implements the heap property on data[lo:hi].
// first is an offset into the array where the root of the heap lies.
func siftDown_func(data lessSwap, lo, hi, first int) {
	root := lo
	for {
		child := 2*root + 1
		if child >= hi {
			break
		}
		if child+1 < hi && data.Less(first+child, first+child+1) {
			child++
		}
		if !data.Less(first+root, first+child) {
			return
		}
		data.Swap(first+root, first+child)
		root = child
	}
}

func heapSort_func(data lessSwap, a, b int) {
	first := a
	lo := 0
	hi := b - a

	// Build heap with greatest element at top.
	for i := (hi - 1) / 2; i >= 0; i-- {
		siftDown_func(data, i, hi, first)
	}

	// Pop elements, largest first, into end of data.
	for i := hi - 1; i >= 0; i-- {
		data.Swap(first, first+i)
		siftDown_func(data, lo, i, first)
	}
}

// pdqsort_func sorts data[a:b].
// The algorithm based on pattern-defeating quicksort(pdqsort), but without the optimizations from BlockQuicksort.
// pdqsort paper: https://arxiv.org/pdf/2106.05123.pdf
// C++ implementation: https://github.com/orlp/pdqsort
// Rust implementation: https://docs.rs/pdqsort/latest/pdqsort/
// limit is the number of allowed bad (very unbalanced) pivots before falling back to heapsort.
func pdqsort_func(data lessSwap, a, b, limit int) {
	const maxInsertion = 12

	var (
		wasBalanced    = true // whether the last partitioning was reasonably balanced
		wasPartitioned = true // whether the slice was already partitioned
	)

	for {
		length := b - a

		if length <= maxInsertion {
			insertionSort_func(data, a, b)
			return
		}

		// Fall back to heapsort if too many bad choices were made.
		if limit == 0 {
			heapSort_func(data, a, b)
			return
		}

		// If the last partitioning was imbalanced, we need to breaking patterns.
		if !wasBalanced {
			breakPatterns_func(data, a, b)
			limit--
		}

		pivot, hint := choosePivot_func(data, a, b)
		if hint == decreasingHint {
			reverseRange_func(data, a, b)
			// The chosen pivot was pivot-a elements after the start of the array.
			// After reversing it is pivot-a elements before the end of the array.
			// The idea came from Rust's implementation.
			pivot = (b - 1) - (pivot - a)
			hint = increasingHint
		}

		// The slice is likely already sorted.
		if wasBalanced && wasPartitioned && hint == increasingHint {
			if partialInsertionSort_func(data, a, b) {
				return
			}
		}

		// Probably the slice contains many duplicate elements, partition the slice into
		// elements equal to and elements greater than the pivot.
		if a > 0 && !data.Less(a-1, pivot) {
			mid := partitionEqual_func(data, a, b, pivot)
			a = mid
			continue
		}

		mid, alreadyPartitioned := partition_func(data, a, b, pivot)
		wasPartitioned = alreadyPartitioned

		leftLen, rightLen := mid-a, b-mid
		balanceThreshold := length / 8
		if leftLen < rightLen {
			wasBalanced = leftLen >= balanceThreshold
			pdqsort_func(data, a, mid, limit)
			a = mid + 1
		} else {
			wasBalanced = rightLen >= balanceThreshold
			pdqsort_func(data, mid+1, b, limit)
			b = mid
		}
	}
}

// partition_func does one quicksort partition.
// Let p = data[pivot]
// Moves elements in data[a:b] around, so that data[i]<p and data[j]>=p for i<newpivot and j>newpivot.
// On return, data[newpivot] = p
func partition_func(data lessSwap, a, b, pivot int) (newpivot int, alreadyPartitioned bool) {
	data.Swap(a, pivot)
	i, j := a+1, b-1 // i and j are inclusive of the elements remaining to be partitioned

	for i <= j && data.Less(i, a) {
		i++
	}
	for i <= j && !data.Less(j, a) {
		j--
	}
	if i > j {
		data.Swap(j, a)
		return j, true
	}
	data.Swap(i, j)
	i++
	j--

	for {
		for i <= j && data.Less(i, a) {
			i++
		}
		for i <= j && !data.Less(j, a) {
			j--
		}
		if i > j {
			break
		}
		data.Swap(i, j)
		i++
		j--
	}
	data.Swap(j, a)
	return j, false
}

// partitionEqual_func partitions data[a:b] into elements equal to data[pivot] followed by elements greater than data[pivot].
// It assumed that data[a:b] does not contain elements smaller than the data[pivot].
func partitionEqual_func(data lessSwap, a, b, pivot int) (newpivot int) {
	data.Swap(a, pivot)
	i, j := a+1, b-1 // i and j are inclusive of the elements remaining to be partitioned

	for {
		for i <= j && !data.Less(a, i) {
			i++
		}
		for i <= j && data.Less(a, j) {
			j--
		}
		if i > j {
			break
		}
		data.Swap(i, j)
		i++
		j--
	}
	return i
}

// partialInsertionSort_func partially sorts a slice, returns true if the slice is sorted at the end.
func partialInsertionSort_func(data lessSwap, a, b int) bool {
	const (
		maxSteps         = 5  // maximum number of adjacent out-of-order pairs that will get shifted
		shortestShifting = 50 // don't shift any elements on short arrays
	)
	i := a + 1
	for j := 0; j < maxSteps; j++ {
		for i < b && !data.Less(i, i-1) {
			i++
		}

		if i == b {
			return true
		}

		if b-a < shortestShifting {
			return false
		}

		data.Swap(i, i-1)

		// Shift the smaller one to the left.
		if i-a >= 2 {
			for j := i - 1; j >= 1; j-- {
				if !data.Less(j, j-1) {
					break
				}
				data.Swap(j, j-1)
			}
		}
		// Shift the greater one to the right.
		if b-i >= 2 {
			for j := i + 1; j < b; j++ {
				if !data.Less(j, j-1) {
					break
				}
				data.Swap(j, j-1)
			}
		}
	}
	return false
}

// breakPatterns_func scatters some elements around in an attempt to break some patterns
// that might cause imbalanced partitions in quicksort.
func breakPatterns_func(data lessSwap, a, b int) {
	length := b - a
	if length >= 8 {
		random := xorshift(length)
		modulus := nextPowerOfTwo(length)

		for idx := a + (length/4)*2 - 1; idx <= a+(length/4)*2+1; idx++ {
			other := int(uint(random.Next()) & (modulus - 1))
			if other >= length {
				other -= length
			}
			data.Swap(idx, a+other)
		}
	}
}

// choosePivot_func chooses a pivot in data[a:b].
//
// [0,8): chooses a static pivot.
// [8,shortestNinther): uses the simple median-of-three method.
// [shortestNinther,∞): uses the Tukey ninther method.
func choosePivot_func(data lessSwap, a, b int) (pivot int, hint sortedHint) {
	const (
		shortestNinther = 50
		maxSwaps        = 4 * 3
	)

	l := b - a

	var (
		swaps int
		i     = a + l/4*1
		j     = a + l/4*2
		k     = a + l/4*3
	)

	if l >= 8 {
		if l >= shortestNinther {
			// Tukey ninther method, the idea came from Rust's implementation.
			i = medianAdjacent_func(data, i, &swaps)
			j = medianAdjacent_func(data, j, &swaps)
			k = medianAdjacent_func(data, k, &swaps)
		}
		// Find the median among i, j, k and stores it into j.
		j = median_func(data, i, j, k, &swaps)
	}

	switch swaps {
	case 0:
		return j, increasingHint
	case maxSwaps:
		return j, decreasingHint
	default:
		return j, unknownHint
	}
}

// order2_func returns x,y where data[x] <= data[y], where x,y=a,b or x,y=b,a.
func order2_func(data lessSwap, a, b int, swaps *int) (int, int) {
	if data.Less(b, a) {
		*swaps++
		return b, a
	}
	return a, b
}

// median_func returns x where data[x] is the median of data[a],data[b],data[c], where x is a, b, or c.
func median_func(data lessSwap, a, b, c int, swaps *int) int {
	a, b = order2_func(data, a, b, swaps)
	b, c = order2_func(data, b, c, swaps)
	a, b = order2_func(data, a, b, swaps)
	return b
}

// medianAdjacent_func finds the median of data[a - 1], data[a], data[a + 1] and stores the index into a.
func medianAdjacent_func(data lessSwap, a int, swaps *int) int {
	return median_func(data, a-1, a, a+1, swaps)
}

func reverseRange_func(data lessSwap, a, b int) {
	i := a
	j := b - 1
	for i < j {
		data.Swap(i, j)
		i++
		j--
	}
}

func swapRange_func(data lessSwap, a, b, n int) {
	for i := 0; i < n; i++ {
		data.Swap(a+i, b+i)
	}
}

func stable_func(data lessSwap, n int) {
	blockSize := 20 // must be > 0
	a, b := 0, blockSize
	for b <= n {
		insertionSort_func(data, a, b)
		a = b
		b += blockSize
	}
	insertionSort_func(data, a, n)

	for blockSize < n {
		a, b = 0, 2*blockSize
		for b <= n {
			symMerge_func(data, a, a+blockSize, b)
			a = b
			b += 2 * blockSize
		}
		if m := a + blockSize; m < n {
			symMerge_func(data, a, m, n)
		}
		blockSize *= 2
	}
}

// symMerge_func merges the two sorted subsequences data[a:m] and data[m:b] using
// the SymMerge algorithm from Pok-Son Kim and Arne Kutzner, "Stable Minimum
// Storage Merging by Symmetric Comparisons", in Susanne Albers and Tomasz
// Radzik, editors, Algorithms - ESA 2004, volume 3221 of Lecture Notes in
// Computer Science, pages 714-723. Springer, 2004.
//
// Let M = m-a and N = b-n. Wolog M < N.
// The recursion depth is bound by ceil(log(N+M)).
// The algorithm needs O(M*log(N/M + 1)) calls to data.Less.
// The algorithm needs O((M+N)*log(M)) calls to data.Swap.
//
// The paper gives O((M+N)*log(M)) as the number of assignments assuming a
// rotation algorithm which uses O(M+N+gcd(M+N)) assignments. The argumentation
// in the paper carries through for Swap operations, especially as the block
// swapping rotate uses only O(M+N) Swaps.
//
// symMerge assumes non-degenerate arguments: a < m && m < b.
// Having the caller check this condition eliminates many leaf recursion calls,
// which improves performance.
func symMerge_func(data lessSwap, a, m, b int) {
	// Avoid unnecessary recursions of symMerge
	// by direct insertion of data[a] into data[m:b]
	// if data[a:m] only contains one element.
	if m-a == 1 {
		// Use binary search to find the lowest index i
		// such that data[i] >= data[a] for m <= i < b.
		// Exit the search loop with i == b in case no such index exists.
		i := m
		j := b
		for i < j {
			h := int(uint(i+j) >> 1)
			if data.Less(h, a) {
				i = h + 1
			} else {
				j = h
			}
		}
		// Swap values until data[a] reaches the position before i.
		for k := a; k < i-1; k++ {
			data.Swap(k, k+1)
		}
		return
	}

	// Avoid unnecessary recursions of symMerge
	// by direct insertion of data[m] into data[a:m]
	// if data[m:b] only contains one element.
	if b-m == 1 {
		// Use binary search to find the lowest index i
		// such that data[i] > data[m] for a <= i < m.
		// Exit the search loop with i == m in case no such index exists.
		i := a
		j := m
		for i < j {
			h := int(uint(i+j) >> 1)
			if !data.Less(m, h) {
				i = h + 1
			} else {
				j = h
			}
		}
		// Swap values until data[m] reaches the position i.
		for k := m; k > i; k-- {
			data.Swap(k, k-1)
		}
		return
	}

	mid := int(uint(a+b) >> 1)
	n := mid + m
	var start, r int
	if m > mid {
		start = n - b
		r = mid
	} else {
		start = a
		r = m
	}
	p := n - 1

	for start < r {
		c := int(uint(start+r) >> 1)
		if !data.Less(p-c, c) {
			start = c + 1
		} else {
			r = c
		}
	}

	end := n - start
	if start < m && m < end {
		rotate_func(data, start, m, end)
	}
	if a < start && start < mid {
		symMerge_func(data, a, start, mid)
	}
	if mid < end && end < b {
		symMerge_func(data, mid, end, b)
	}
}

// rotate_func rotates two consecutive blocks u = data[a:m] and v = data[m:b] in data:
// Data of the form 'x u v y' is changed to 'x v u y'.
// rotate performs at most b-a many calls to data.Swap,
// and it assumes non-degenerate arguments: a < m && m < b.
func rotate_func(data lessSwap, a, m, b int) {
	i := m - a
	j := b - m

	for i != j {
		if i > j {
			swapRange_func(data, m-i, m, j)
			i -= j
		} else {
			swapRange_func(data, m-i, m+j-i, i)
			j -= i
		}
	}
	// i == j
	swapRange_func(data, m-i, m, i)
}

```

// === FILE: references/go/src/sort/zsortinterface.go ===
```go
// Code generated by gen_sort_variants.go; DO NOT EDIT.

// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sort

// insertionSort sorts data[a:b] using insertion sort.
func insertionSort(data Interface, a, b int) {
	for i := a + 1; i < b; i++ {
		for j := i; j > a && data.Less(j, j-1); j-- {
			data.Swap(j, j-1)
		}
	}
}

// siftDown implements the heap property on data[lo:hi].
// first is an offset into the array where the root of the heap lies.
func siftDown(data Interface, lo, hi, first int) {
	root := lo
	for {
		child := 2*root + 1
		if child >= hi {
			break
		}
		if child+1 < hi && data.Less(first+child, first+child+1) {
			child++
		}
		if !data.Less(first+root, first+child) {
			return
		}
		data.Swap(first+root, first+child)
		root = child
	}
}

func heapSort(data Interface, a, b int) {
	first := a
	lo := 0
	hi := b - a

	// Build heap with greatest element at top.
	for i := (hi - 1) / 2; i >= 0; i-- {
		siftDown(data, i, hi, first)
	}

	// Pop elements, largest first, into end of data.
	for i := hi - 1; i >= 0; i-- {
		data.Swap(first, first+i)
		siftDown(data, lo, i, first)
	}
}

// pdqsort sorts data[a:b].
// The algorithm based on pattern-defeating quicksort(pdqsort), but without the optimizations from BlockQuicksort.
// pdqsort paper: https://arxiv.org/pdf/2106.05123.pdf
// C++ implementation: https://github.com/orlp/pdqsort
// Rust implementation: https://docs.rs/pdqsort/latest/pdqsort/
// limit is the number of allowed bad (very unbalanced) pivots before falling back to heapsort.
func pdqsort(data Interface, a, b, limit int) {
	const maxInsertion = 12

	var (
		wasBalanced    = true // whether the last partitioning was reasonably balanced
		wasPartitioned = true // whether the slice was already partitioned
	)

	for {
		length := b - a

		if length <= maxInsertion {
			insertionSort(data, a, b)
			return
		}

		// Fall back to heapsort if too many bad choices were made.
		if limit == 0 {
			heapSort(data, a, b)
			return
		}

		// If the last partitioning was imbalanced, we need to breaking patterns.
		if !wasBalanced {
			breakPatterns(data, a, b)
			limit--
		}

		pivot, hint := choosePivot(data, a, b)
		if hint == decreasingHint {
			reverseRange(data, a, b)
			// The chosen pivot was pivot-a elements after the start of the array.
			// After reversing it is pivot-a elements before the end of the array.
			// The idea came from Rust's implementation.
			pivot = (b - 1) - (pivot - a)
			hint = increasingHint
		}

		// The slice is likely already sorted.
		if wasBalanced && wasPartitioned && hint == increasingHint {
			if partialInsertionSort(data, a, b) {
				return
			}
		}

		// Probably the slice contains many duplicate elements, partition the slice into
		// elements equal to and elements greater than the pivot.
		if a > 0 && !data.Less(a-1, pivot) {
			mid := partitionEqual(data, a, b, pivot)
			a = mid
			continue
		}

		mid, alreadyPartitioned := partition(data, a, b, pivot)
		wasPartitioned = alreadyPartitioned

		leftLen, rightLen := mid-a, b-mid
		balanceThreshold := length / 8
		if leftLen < rightLen {
			wasBalanced = leftLen >= balanceThreshold
			pdqsort(data, a, mid, limit)
			a = mid + 1
		} else {
			wasBalanced = rightLen >= balanceThreshold
			pdqsort(data, mid+1, b, limit)
			b = mid
		}
	}
}

// partition does one quicksort partition.
// Let p = data[pivot]
// Moves elements in data[a:b] around, so that data[i]<p and data[j]>=p for i<newpivot and j>newpivot.
// On return, data[newpivot] = p
func partition(data Interface, a, b, pivot int) (newpivot int, alreadyPartitioned bool) {
	data.Swap(a, pivot)
	i, j := a+1, b-1 // i and j are inclusive of the elements remaining to be partitioned

	for i <= j && data.Less(i, a) {
		i++
	}
	for i <= j && !data.Less(j, a) {
		j--
	}
	if i > j {
		data.Swap(j, a)
		return j, true
	}
	data.Swap(i, j)
	i++
	j--

	for {
		for i <= j && data.Less(i, a) {
			i++
		}
		for i <= j && !data.Less(j, a) {
			j--
		}
		if i > j {
			break
		}
		data.Swap(i, j)
		i++
		j--
	}
	data.Swap(j, a)
	return j, false
}

// partitionEqual partitions data[a:b] into elements equal to data[pivot] followed by elements greater than data[pivot].
// It assumed that data[a:b] does not contain elements smaller than the data[pivot].
func partitionEqual(data Interface, a, b, pivot int) (newpivot int) {
	data.Swap(a, pivot)
	i, j := a+1, b-1 // i and j are inclusive of the elements remaining to be partitioned

	for {
		for i <= j && !data.Less(a, i) {
			i++
		}
		for i <= j && data.Less(a, j) {
			j--
		}
		if i > j {
			break
		}
		data.Swap(i, j)
		i++
		j--
	}
	return i
}

// partialInsertionSort partially sorts a slice, returns true if the slice is sorted at the end.
func partialInsertionSort(data Interface, a, b int) bool {
	const (
		maxSteps         = 5  // maximum number of adjacent out-of-order pairs that will get shifted
		shortestShifting = 50 // don't shift any elements on short arrays
	)
	i := a + 1
	for j := 0; j < maxSteps; j++ {
		for i < b && !data.Less(i, i-1) {
			i++
		}

		if i == b {
			return true
		}

		if b-a < shortestShifting {
			return false
		}

		data.Swap(i, i-1)

		// Shift the smaller one to the left.
		if i-a >= 2 {
			for j := i - 1; j >= 1; j-- {
				if !data.Less(j, j-1) {
					break
				}
				data.Swap(j, j-1)
			}
		}
		// Shift the greater one to the right.
		if b-i >= 2 {
			for j := i + 1; j < b; j++ {
				if !data.Less(j, j-1) {
					break
				}
				data.Swap(j, j-1)
			}
		}
	}
	return false
}

// breakPatterns scatters some elements around in an attempt to break some patterns
// that might cause imbalanced partitions in quicksort.
func breakPatterns(data Interface, a, b int) {
	length := b - a
	if length >= 8 {
		random := xorshift(length)
		modulus := nextPowerOfTwo(length)

		for idx := a + (length/4)*2 - 1; idx <= a+(length/4)*2+1; idx++ {
			other := int(uint(random.Next()) & (modulus - 1))
			if other >= length {
				other -= length
			}
			data.Swap(idx, a+other)
		}
	}
}

// choosePivot chooses a pivot in data[a:b].
//
// [0,8): chooses a static pivot.
// [8,shortestNinther): uses the simple median-of-three method.
// [shortestNinther,∞): uses the Tukey ninther method.
func choosePivot(data Interface, a, b int) (pivot int, hint sortedHint) {
	const (
		shortestNinther = 50
		maxSwaps        = 4 * 3
	)

	l := b - a

	var (
		swaps int
		i     = a + l/4*1
		j     = a + l/4*2
		k     = a + l/4*3
	)

	if l >= 8 {
		if l >= shortestNinther {
			// Tukey ninther method, the idea came from Rust's implementation.
			i = medianAdjacent(data, i, &swaps)
			j = medianAdjacent(data, j, &swaps)
			k = medianAdjacent(data, k, &swaps)
		}
		// Find the median among i, j, k and stores it into j.
		j = median(data, i, j, k, &swaps)
	}

	switch swaps {
	case 0:
		return j, increasingHint
	case maxSwaps:
		return j, decreasingHint
	default:
		return j, unknownHint
	}
}

// order2 returns x,y where data[x] <= data[y], where x,y=a,b or x,y=b,a.
func order2(data Interface, a, b int, swaps *int) (int, int) {
	if data.Less(b, a) {
		*swaps++
		return b, a
	}
	return a, b
}

// median returns x where data[x] is the median of data[a],data[b],data[c], where x is a, b, or c.
func median(data Interface, a, b, c int, swaps *int) int {
	a, b = order2(data, a, b, swaps)
	b, c = order2(data, b, c, swaps)
	a, b = order2(data, a, b, swaps)
	return b
}

// medianAdjacent finds the median of data[a - 1], data[a], data[a + 1] and stores the index into a.
func medianAdjacent(data Interface, a int, swaps *int) int {
	return median(data, a-1, a, a+1, swaps)
}

func reverseRange(data Interface, a, b int) {
	i := a
	j := b - 1
	for i < j {
		data.Swap(i, j)
		i++
		j--
	}
}

func swapRange(data Interface, a, b, n int) {
	for i := 0; i < n; i++ {
		data.Swap(a+i, b+i)
	}
}

func stable(data Interface, n int) {
	blockSize := 20 // must be > 0
	a, b := 0, blockSize
	for b <= n {
		insertionSort(data, a, b)
		a = b
		b += blockSize
	}
	insertionSort(data, a, n)

	for blockSize < n {
		a, b = 0, 2*blockSize
		for b <= n {
			symMerge(data, a, a+blockSize, b)
			a = b
			b += 2 * blockSize
		}
		if m := a + blockSize; m < n {
			symMerge(data, a, m, n)
		}
		blockSize *= 2
	}
}

// symMerge merges the two sorted subsequences data[a:m] and data[m:b] using
// the SymMerge algorithm from Pok-Son Kim and Arne Kutzner, "Stable Minimum
// Storage Merging by Symmetric Comparisons", in Susanne Albers and Tomasz
// Radzik, editors, Algorithms - ESA 2004, volume 3221 of Lecture Notes in
// Computer Science, pages 714-723. Springer, 2004.
//
// Let M = m-a and N = b-n. Wolog M < N.
// The recursion depth is bound by ceil(log(N+M)).
// The algorithm needs O(M*log(N/M + 1)) calls to data.Less.
// The algorithm needs O((M+N)*log(M)) calls to data.Swap.
//
// The paper gives O((M+N)*log(M)) as the number of assignments assuming a
// rotation algorithm which uses O(M+N+gcd(M+N)) assignments. The argumentation
// in the paper carries through for Swap operations, especially as the block
// swapping rotate uses only O(M+N) Swaps.
//
// symMerge assumes non-degenerate arguments: a < m && m < b.
// Having the caller check this condition eliminates many leaf recursion calls,
// which improves performance.
func symMerge(data Interface, a, m, b int) {
	// Avoid unnecessary recursions of symMerge
	// by direct insertion of data[a] into data[m:b]
	// if data[a:m] only contains one element.
	if m-a == 1 {
		// Use binary search to find the lowest index i
		// such that data[i] >= data[a] for m <= i < b.
		// Exit the search loop with i == b in case no such index exists.
		i := m
		j := b
		for i < j {
			h := int(uint(i+j) >> 1)
			if data.Less(h, a) {
				i = h + 1
			} else {
				j = h
			}
		}
		// Swap values until data[a] reaches the position before i.
		for k := a; k < i-1; k++ {
			data.Swap(k, k+1)
		}
		return
	}

	// Avoid unnecessary recursions of symMerge
	// by direct insertion of data[m] into data[a:m]
	// if data[m:b] only contains one element.
	if b-m == 1 {
		// Use binary search to find the lowest index i
		// such that data[i] > data[m] for a <= i < m.
		// Exit the search loop with i == m in case no such index exists.
		i := a
		j := m
		for i < j {
			h := int(uint(i+j) >> 1)
			if !data.Less(m, h) {
				i = h + 1
			} else {
				j = h
			}
		}
		// Swap values until data[m] reaches the position i.
		for k := m; k > i; k-- {
			data.Swap(k, k-1)
		}
		return
	}

	mid := int(uint(a+b) >> 1)
	n := mid + m
	var start, r int
	if m > mid {
		start = n - b
		r = mid
	} else {
		start = a
		r = m
	}
	p := n - 1

	for start < r {
		c := int(uint(start+r) >> 1)
		if !data.Less(p-c, c) {
			start = c + 1
		} else {
			r = c
		}
	}

	end := n - start
	if start < m && m < end {
		rotate(data, start, m, end)
	}
	if a < start && start < mid {
		symMerge(data, a, start, mid)
	}
	if mid < end && end < b {
		symMerge(data, mid, end, b)
	}
}

// rotate rotates two consecutive blocks u = data[a:m] and v = data[m:b] in data:
// Data of the form 'x u v y' is changed to 'x v u y'.
// rotate performs at most b-a many calls to data.Swap,
// and it assumes non-degenerate arguments: a < m && m < b.
func rotate(data Interface, a, m, b int) {
	i := m - a
	j := b - m

	for i != j {
		if i > j {
			swapRange(data, m-i, m, j)
			i -= j
		} else {
			swapRange(data, m-i, m+j-i, i)
			j -= i
		}
	}
	// i == j
	swapRange(data, m-i, m, i)
}

```

