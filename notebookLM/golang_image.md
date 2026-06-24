# Domain Architecture: image

## Layout Topology
```text
image/
├── color
│   ├── palette
│   │   ├── gen.go
│   │   ├── generate.go
│   │   └── palette.go
│   ├── color.go
│   └── ycbcr.go
├── draw
│   └── draw.go
├── gif
│   ├── reader.go
│   └── writer.go
├── internal
│   └── imageutil
│       ├── gen.go
│       ├── imageutil.go
│       └── impl.go
├── jpeg
│   ├── dct.go
│   ├── huffman.go
│   ├── reader.go
│   ├── scan.go
│   └── writer.go
├── png
│   ├── paeth.go
│   ├── reader.go
│   └── writer.go
├── format.go
├── geom.go
├── image.go
├── names.go
└── ycbcr.go
```

## Source Stream Aggregation

// === FILE: references/go/src/image/color/color.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package color implements a basic color library.
package color

// Color can convert itself to alpha-premultiplied 16-bits per channel RGBA.
// The conversion may be lossy.
type Color interface {
	// RGBA returns the alpha-premultiplied red, green, blue and alpha values
	// for the color. Each value ranges within [0, 0xffff], but is represented
	// by a uint32 so that multiplying by a blend factor up to 0xffff will not
	// overflow.
	//
	// An alpha-premultiplied color component c has been scaled by alpha (a),
	// so has valid values 0 <= c <= a.
	RGBA() (r, g, b, a uint32)
}

// RGBA represents a traditional 32-bit alpha-premultiplied color, having 8
// bits for each of red, green, blue and alpha.
//
// An alpha-premultiplied color component C has been scaled by alpha (A), so
// has valid values 0 <= C <= A.
type RGBA struct {
	R, G, B, A uint8
}

func (c RGBA) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r |= r << 8
	g = uint32(c.G)
	g |= g << 8
	b = uint32(c.B)
	b |= b << 8
	a = uint32(c.A)
	a |= a << 8
	return
}

// RGBA64 represents a 64-bit alpha-premultiplied color, having 16 bits for
// each of red, green, blue and alpha.
//
// An alpha-premultiplied color component C has been scaled by alpha (A), so
// has valid values 0 <= C <= A.
type RGBA64 struct {
	R, G, B, A uint16
}

func (c RGBA64) RGBA() (r, g, b, a uint32) {
	return uint32(c.R), uint32(c.G), uint32(c.B), uint32(c.A)
}

// NRGBA represents a non-alpha-premultiplied 32-bit color.
type NRGBA struct {
	R, G, B, A uint8
}

func (c NRGBA) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r |= r << 8
	r *= uint32(c.A)
	r /= 0xff
	g = uint32(c.G)
	g |= g << 8
	g *= uint32(c.A)
	g /= 0xff
	b = uint32(c.B)
	b |= b << 8
	b *= uint32(c.A)
	b /= 0xff
	a = uint32(c.A)
	a |= a << 8
	return
}

// NRGBA64 represents a non-alpha-premultiplied 64-bit color,
// having 16 bits for each of red, green, blue and alpha.
type NRGBA64 struct {
	R, G, B, A uint16
}

func (c NRGBA64) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r *= uint32(c.A)
	r /= 0xffff
	g = uint32(c.G)
	g *= uint32(c.A)
	g /= 0xffff
	b = uint32(c.B)
	b *= uint32(c.A)
	b /= 0xffff
	a = uint32(c.A)
	return
}

// Alpha represents an 8-bit alpha color.
type Alpha struct {
	A uint8
}

func (c Alpha) RGBA() (r, g, b, a uint32) {
	a = uint32(c.A)
	a |= a << 8
	return a, a, a, a
}

// Alpha16 represents a 16-bit alpha color.
type Alpha16 struct {
	A uint16
}

func (c Alpha16) RGBA() (r, g, b, a uint32) {
	a = uint32(c.A)
	return a, a, a, a
}

// Gray represents an 8-bit grayscale color.
type Gray struct {
	Y uint8
}

func (c Gray) RGBA() (r, g, b, a uint32) {
	y := uint32(c.Y)
	y |= y << 8
	return y, y, y, 0xffff
}

// Gray16 represents a 16-bit grayscale color.
type Gray16 struct {
	Y uint16
}

func (c Gray16) RGBA() (r, g, b, a uint32) {
	y := uint32(c.Y)
	return y, y, y, 0xffff
}

// Model can convert any [Color] to one from its own color model. The conversion
// may be lossy.
type Model interface {
	Convert(c Color) Color
}

// ModelFunc returns a [Model] that invokes f to implement the conversion.
func ModelFunc(f func(Color) Color) Model {
	// Note: using *modelFunc as the implementation
	// means that callers can still use comparisons
	// like m == RGBAModel. This is not possible if
	// we use the func value directly, because funcs
	// are no longer comparable.
	return &modelFunc{f}
}

type modelFunc struct {
	f func(Color) Color
}

func (m *modelFunc) Convert(c Color) Color {
	return m.f(c)
}

// Models for the standard color types.
var (
	RGBAModel    Model = ModelFunc(rgbaModel)
	RGBA64Model  Model = ModelFunc(rgba64Model)
	NRGBAModel   Model = ModelFunc(nrgbaModel)
	NRGBA64Model Model = ModelFunc(nrgba64Model)
	AlphaModel   Model = ModelFunc(alphaModel)
	Alpha16Model Model = ModelFunc(alpha16Model)
	GrayModel    Model = ModelFunc(grayModel)
	Gray16Model  Model = ModelFunc(gray16Model)
)

func rgbaModel(c Color) Color {
	if _, ok := c.(RGBA); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	return RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func rgba64Model(c Color) Color {
	if _, ok := c.(RGBA64); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	return RGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
}

func nrgbaModel(c Color) Color {
	if _, ok := c.(NRGBA); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	if a == 0xffff {
		return NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 0xff}
	}
	if a == 0 {
		return NRGBA{0, 0, 0, 0}
	}
	// Since Color.RGBA returns an alpha-premultiplied color, we should have r <= a && g <= a && b <= a.
	r = (r * 0xffff) / a
	g = (g * 0xffff) / a
	b = (b * 0xffff) / a
	return NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func nrgba64Model(c Color) Color {
	if _, ok := c.(NRGBA64); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	if a == 0xffff {
		return NRGBA64{uint16(r), uint16(g), uint16(b), 0xffff}
	}
	if a == 0 {
		return NRGBA64{0, 0, 0, 0}
	}
	// Since Color.RGBA returns an alpha-premultiplied color, we should have r <= a && g <= a && b <= a.
	r = (r * 0xffff) / a
	g = (g * 0xffff) / a
	b = (b * 0xffff) / a
	return NRGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
}

func alphaModel(c Color) Color {
	if _, ok := c.(Alpha); ok {
		return c
	}
	_, _, _, a := c.RGBA()
	return Alpha{uint8(a >> 8)}
}

func alpha16Model(c Color) Color {
	if _, ok := c.(Alpha16); ok {
		return c
	}
	_, _, _, a := c.RGBA()
	return Alpha16{uint16(a)}
}

func grayModel(c Color) Color {
	if _, ok := c.(Gray); ok {
		return c
	}
	r, g, b, _ := c.RGBA()

	// These coefficients (the fractions 0.299, 0.587 and 0.114) are the same
	// as those given by the JFIF specification and used by func RGBToYCbCr in
	// ycbcr.go.
	//
	// Note that 19595 + 38470 + 7471 equals 65536.
	//
	// The 24 is 16 + 8. The 16 is the same as used in RGBToYCbCr. The 8 is
	// because the return value is 8 bit color, not 16 bit color.
	y := (19595*r + 38470*g + 7471*b + 1<<15) >> 24

	return Gray{uint8(y)}
}

func gray16Model(c Color) Color {
	if _, ok := c.(Gray16); ok {
		return c
	}
	r, g, b, _ := c.RGBA()

	// These coefficients (the fractions 0.299, 0.587 and 0.114) are the same
	// as those given by the JFIF specification and used by func RGBToYCbCr in
	// ycbcr.go.
	//
	// Note that 19595 + 38470 + 7471 equals 65536.
	y := (19595*r + 38470*g + 7471*b + 1<<15) >> 16

	return Gray16{uint16(y)}
}

// Palette is a palette of colors.
type Palette []Color

// Convert returns the palette color closest to c in Euclidean R,G,B space.
func (p Palette) Convert(c Color) Color {
	if len(p) == 0 {
		return nil
	}
	return p[p.Index(c)]
}

// Index returns the index of the palette color closest to c in Euclidean
// R,G,B,A space.
func (p Palette) Index(c Color) int {
	// A batch version of this computation is in image/draw/draw.go.

	cr, cg, cb, ca := c.RGBA()
	ret, bestSum := 0, uint32(1<<32-1)
	for i, v := range p {
		vr, vg, vb, va := v.RGBA()
		sum := sqDiff(cr, vr) + sqDiff(cg, vg) + sqDiff(cb, vb) + sqDiff(ca, va)
		if sum < bestSum {
			if sum == 0 {
				return i
			}
			ret, bestSum = i, sum
		}
	}
	return ret
}

// sqDiff returns the squared-difference of x and y, shifted by 2 so that
// adding four of those won't overflow a uint32.
//
// x and y are both assumed to be in the range [0, 0xffff].
func sqDiff(x, y uint32) uint32 {
	// The canonical code of this function looks as follows:
	//
	//	var d uint32
	//	if x > y {
	//		d = x - y
	//	} else {
	//		d = y - x
	//	}
	//	return (d * d) >> 2
	//
	// Language spec guarantees the following properties of unsigned integer
	// values operations with respect to overflow/wrap around:
	//
	// > For unsigned integer values, the operations +, -, *, and << are
	// > computed modulo 2n, where n is the bit width of the unsigned
	// > integer's type. Loosely speaking, these unsigned integer operations
	// > discard high bits upon overflow, and programs may rely on ``wrap
	// > around''.
	//
	// Considering these properties and the fact that this function is
	// called in the hot paths (x,y loops), it is reduced to the below code
	// which is slightly faster. See TestSqDiff for correctness check.
	d := x - y
	return (d * d) >> 2
}

// Standard colors.
var (
	Black       = Gray16{0}
	White       = Gray16{0xffff}
	Transparent = Alpha16{0}
	Opaque      = Alpha16{0xffff}
)

```

// === FILE: references/go/src/image/color/palette/gen.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

package main

// This program generates palette.go. Invoke it as
//	go run gen.go -output palette.go

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"os"
)

var filename = flag.String("output", "palette.go", "output file name")

func main() {
	flag.Parse()

	var buf bytes.Buffer

	fmt.Fprintln(&buf, `// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.`)
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, "// Code generated by go run gen.go -output palette.go; DO NOT EDIT.")
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, "package palette")
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, `import "image/color"`)
	fmt.Fprintln(&buf)
	printPlan9(&buf)
	printWebSafe(&buf)

	data, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(*filename, data, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func printPlan9(w io.Writer) {
	c, lines := [3]int{}, [256]string{}
	for r, i := 0, 0; r != 4; r++ {
		for v := 0; v != 4; v, i = v+1, i+16 {
			for g, j := 0, v-r; g != 4; g++ {
				for b := 0; b != 4; b, j = b+1, j+1 {
					den := r
					if g > den {
						den = g
					}
					if b > den {
						den = b
					}
					if den == 0 {
						c[0] = 0x11 * v
						c[1] = 0x11 * v
						c[2] = 0x11 * v
					} else {
						num := 17 * (4*den + v)
						c[0] = r * num / den
						c[1] = g * num / den
						c[2] = b * num / den
					}
					lines[i+(j&0x0f)] =
						fmt.Sprintf("\tcolor.RGBA{0x%02x, 0x%02x, 0x%02x, 0xff},", c[0], c[1], c[2])
				}
			}
		}
	}
	fmt.Fprintln(w, "// Plan9 is a 256-color palette that partitions the 24-bit RGB space")
	fmt.Fprintln(w, "// into 4×4×4 subdivision, with 4 shades in each subcube. Compared to the")
	fmt.Fprintln(w, "// [WebSafe], the idea is to reduce the color resolution by dicing the")
	fmt.Fprintln(w, "// color cube into fewer cells, and to use the extra space to increase the")
	fmt.Fprintln(w, "// intensity resolution. This results in 16 gray shades (4 gray subcubes with")
	fmt.Fprintln(w, "// 4 samples in each), 13 shades of each primary and secondary color (3")
	fmt.Fprintln(w, "// subcubes with 4 samples plus black) and a reasonable selection of colors")
	fmt.Fprintln(w, "// covering the rest of the color cube. The advantage is better representation")
	fmt.Fprintln(w, "// of continuous tones.")
	fmt.Fprintln(w, "//")
	fmt.Fprintln(w, "// This palette was used in the Plan 9 Operating System, described at")
	fmt.Fprintln(w, "// https://9p.io/magic/man2html/6/color")
	fmt.Fprintln(w, "var Plan9 = []color.Color{")
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)
}

func printWebSafe(w io.Writer) {
	lines := [6 * 6 * 6]string{}
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				lines[36*r+6*g+b] =
					fmt.Sprintf("\tcolor.RGBA{0x%02x, 0x%02x, 0x%02x, 0xff},", 0x33*r, 0x33*g, 0x33*b)
			}
		}
	}
	fmt.Fprintln(w, "// WebSafe is a 216-color palette that was popularized by early versions")
	fmt.Fprintln(w, "// of Netscape Navigator. It is also known as the Netscape Color Cube.")
	fmt.Fprintln(w, "//")
	fmt.Fprintln(w, "// See https://en.wikipedia.org/wiki/Web_colors#Web-safe_colors for details.")
	fmt.Fprintln(w, "var WebSafe = []color.Color{")
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)
}

```

// === FILE: references/go/src/image/color/palette/generate.go ===
```go
// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run gen.go -output palette.go

// Package palette provides standard color palettes.
package palette

```

// === FILE: references/go/src/image/color/palette/palette.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by go run gen.go -output palette.go; DO NOT EDIT.

package palette

import "image/color"

// Plan9 is a 256-color palette that partitions the 24-bit RGB space
// into 4×4×4 subdivision, with 4 shades in each subcube. Compared to the
// [WebSafe], the idea is to reduce the color resolution by dicing the
// color cube into fewer cells, and to use the extra space to increase the
// intensity resolution. This results in 16 gray shades (4 gray subcubes with
// 4 samples in each), 13 shades of each primary and secondary color (3
// subcubes with 4 samples plus black) and a reasonable selection of colors
// covering the rest of the color cube. The advantage is better representation
// of continuous tones.
//
// This palette was used in the Plan 9 Operating System, described at
// https://9p.io/magic/man2html/6/color
var Plan9 = []color.Color{
	color.RGBA{0x00, 0x00, 0x00, 0xff},
	color.RGBA{0x00, 0x00, 0x44, 0xff},
	color.RGBA{0x00, 0x00, 0x88, 0xff},
	color.RGBA{0x00, 0x00, 0xcc, 0xff},
	color.RGBA{0x00, 0x44, 0x00, 0xff},
	color.RGBA{0x00, 0x44, 0x44, 0xff},
	color.RGBA{0x00, 0x44, 0x88, 0xff},
	color.RGBA{0x00, 0x44, 0xcc, 0xff},
	color.RGBA{0x00, 0x88, 0x00, 0xff},
	color.RGBA{0x00, 0x88, 0x44, 0xff},
	color.RGBA{0x00, 0x88, 0x88, 0xff},
	color.RGBA{0x00, 0x88, 0xcc, 0xff},
	color.RGBA{0x00, 0xcc, 0x00, 0xff},
	color.RGBA{0x00, 0xcc, 0x44, 0xff},
	color.RGBA{0x00, 0xcc, 0x88, 0xff},
	color.RGBA{0x00, 0xcc, 0xcc, 0xff},
	color.RGBA{0x00, 0xdd, 0xdd, 0xff},
	color.RGBA{0x11, 0x11, 0x11, 0xff},
	color.RGBA{0x00, 0x00, 0x55, 0xff},
	color.RGBA{0x00, 0x00, 0x99, 0xff},
	color.RGBA{0x00, 0x00, 0xdd, 0xff},
	color.RGBA{0x00, 0x55, 0x00, 0xff},
	color.RGBA{0x00, 0x55, 0x55, 0xff},
	color.RGBA{0x00, 0x4c, 0x99, 0xff},
	color.RGBA{0x00, 0x49, 0xdd, 0xff},
	color.RGBA{0x00, 0x99, 0x00, 0xff},
	color.RGBA{0x00, 0x99, 0x4c, 0xff},
	color.RGBA{0x00, 0x99, 0x99, 0xff},
	color.RGBA{0x00, 0x93, 0xdd, 0xff},
	color.RGBA{0x00, 0xdd, 0x00, 0xff},
	color.RGBA{0x00, 0xdd, 0x49, 0xff},
	color.RGBA{0x00, 0xdd, 0x93, 0xff},
	color.RGBA{0x00, 0xee, 0x9e, 0xff},
	color.RGBA{0x00, 0xee, 0xee, 0xff},
	color.RGBA{0x22, 0x22, 0x22, 0xff},
	color.RGBA{0x00, 0x00, 0x66, 0xff},
	color.RGBA{0x00, 0x00, 0xaa, 0xff},
	color.RGBA{0x00, 0x00, 0xee, 0xff},
	color.RGBA{0x00, 0x66, 0x00, 0xff},
	color.RGBA{0x00, 0x66, 0x66, 0xff},
	color.RGBA{0x00, 0x55, 0xaa, 0xff},
	color.RGBA{0x00, 0x4f, 0xee, 0xff},
	color.RGBA{0x00, 0xaa, 0x00, 0xff},
	color.RGBA{0x00, 0xaa, 0x55, 0xff},
	color.RGBA{0x00, 0xaa, 0xaa, 0xff},
	color.RGBA{0x00, 0x9e, 0xee, 0xff},
	color.RGBA{0x00, 0xee, 0x00, 0xff},
	color.RGBA{0x00, 0xee, 0x4f, 0xff},
	color.RGBA{0x00, 0xff, 0x55, 0xff},
	color.RGBA{0x00, 0xff, 0xaa, 0xff},
	color.RGBA{0x00, 0xff, 0xff, 0xff},
	color.RGBA{0x33, 0x33, 0x33, 0xff},
	color.RGBA{0x00, 0x00, 0x77, 0xff},
	color.RGBA{0x00, 0x00, 0xbb, 0xff},
	color.RGBA{0x00, 0x00, 0xff, 0xff},
	color.RGBA{0x00, 0x77, 0x00, 0xff},
	color.RGBA{0x00, 0x77, 0x77, 0xff},
	color.RGBA{0x00, 0x5d, 0xbb, 0xff},
	color.RGBA{0x00, 0x55, 0xff, 0xff},
	color.RGBA{0x00, 0xbb, 0x00, 0xff},
	color.RGBA{0x00, 0xbb, 0x5d, 0xff},
	color.RGBA{0x00, 0xbb, 0xbb, 0xff},
	color.RGBA{0x00, 0xaa, 0xff, 0xff},
	color.RGBA{0x00, 0xff, 0x00, 0xff},
	color.RGBA{0x44, 0x00, 0x44, 0xff},
	color.RGBA{0x44, 0x00, 0x88, 0xff},
	color.RGBA{0x44, 0x00, 0xcc, 0xff},
	color.RGBA{0x44, 0x44, 0x00, 0xff},
	color.RGBA{0x44, 0x44, 0x44, 0xff},
	color.RGBA{0x44, 0x44, 0x88, 0xff},
	color.RGBA{0x44, 0x44, 0xcc, 0xff},
	color.RGBA{0x44, 0x88, 0x00, 0xff},
	color.RGBA{0x44, 0x88, 0x44, 0xff},
	color.RGBA{0x44, 0x88, 0x88, 0xff},
	color.RGBA{0x44, 0x88, 0xcc, 0xff},
	color.RGBA{0x44, 0xcc, 0x00, 0xff},
	color.RGBA{0x44, 0xcc, 0x44, 0xff},
	color.RGBA{0x44, 0xcc, 0x88, 0xff},
	color.RGBA{0x44, 0xcc, 0xcc, 0xff},
	color.RGBA{0x44, 0x00, 0x00, 0xff},
	color.RGBA{0x55, 0x00, 0x00, 0xff},
	color.RGBA{0x55, 0x00, 0x55, 0xff},
	color.RGBA{0x4c, 0x00, 0x99, 0xff},
	color.RGBA{0x49, 0x00, 0xdd, 0xff},
	color.RGBA{0x55, 0x55, 0x00, 0xff},
	color.RGBA{0x55, 0x55, 0x55, 0xff},
	color.RGBA{0x4c, 0x4c, 0x99, 0xff},
	color.RGBA{0x49, 0x49, 0xdd, 0xff},
	color.RGBA{0x4c, 0x99, 0x00, 0xff},
	color.RGBA{0x4c, 0x99, 0x4c, 0xff},
	color.RGBA{0x4c, 0x99, 0x99, 0xff},
	color.RGBA{0x49, 0x93, 0xdd, 0xff},
	color.RGBA{0x49, 0xdd, 0x00, 0xff},
	color.RGBA{0x49, 0xdd, 0x49, 0xff},
	color.RGBA{0x49, 0xdd, 0x93, 0xff},
	color.RGBA{0x49, 0xdd, 0xdd, 0xff},
	color.RGBA{0x4f, 0xee, 0xee, 0xff},
	color.RGBA{0x66, 0x00, 0x00, 0xff},
	color.RGBA{0x66, 0x00, 0x66, 0xff},
	color.RGBA{0x55, 0x00, 0xaa, 0xff},
	color.RGBA{0x4f, 0x00, 0xee, 0xff},
	color.RGBA{0x66, 0x66, 0x00, 0xff},
	color.RGBA{0x66, 0x66, 0x66, 0xff},
	color.RGBA{0x55, 0x55, 0xaa, 0xff},
	color.RGBA{0x4f, 0x4f, 0xee, 0xff},
	color.RGBA{0x55, 0xaa, 0x00, 0xff},
	color.RGBA{0x55, 0xaa, 0x55, 0xff},
	color.RGBA{0x55, 0xaa, 0xaa, 0xff},
	color.RGBA{0x4f, 0x9e, 0xee, 0xff},
	color.RGBA{0x4f, 0xee, 0x00, 0xff},
	color.RGBA{0x4f, 0xee, 0x4f, 0xff},
	color.RGBA{0x4f, 0xee, 0x9e, 0xff},
	color.RGBA{0x55, 0xff, 0xaa, 0xff},
	color.RGBA{0x55, 0xff, 0xff, 0xff},
	color.RGBA{0x77, 0x00, 0x00, 0xff},
	color.RGBA{0x77, 0x00, 0x77, 0xff},
	color.RGBA{0x5d, 0x00, 0xbb, 0xff},
	color.RGBA{0x55, 0x00, 0xff, 0xff},
	color.RGBA{0x77, 0x77, 0x00, 0xff},
	color.RGBA{0x77, 0x77, 0x77, 0xff},
	color.RGBA{0x5d, 0x5d, 0xbb, 0xff},
	color.RGBA{0x55, 0x55, 0xff, 0xff},
	color.RGBA{0x5d, 0xbb, 0x00, 0xff},
	color.RGBA{0x5d, 0xbb, 0x5d, 0xff},
	color.RGBA{0x5d, 0xbb, 0xbb, 0xff},
	color.RGBA{0x55, 0xaa, 0xff, 0xff},
	color.RGBA{0x55, 0xff, 0x00, 0xff},
	color.RGBA{0x55, 0xff, 0x55, 0xff},
	color.RGBA{0x88, 0x00, 0x88, 0xff},
	color.RGBA{0x88, 0x00, 0xcc, 0xff},
	color.RGBA{0x88, 0x44, 0x00, 0xff},
	color.RGBA{0x88, 0x44, 0x44, 0xff},
	color.RGBA{0x88, 0x44, 0x88, 0xff},
	color.RGBA{0x88, 0x44, 0xcc, 0xff},
	color.RGBA{0x88, 0x88, 0x00, 0xff},
	color.RGBA{0x88, 0x88, 0x44, 0xff},
	color.RGBA{0x88, 0x88, 0x88, 0xff},
	color.RGBA{0x88, 0x88, 0xcc, 0xff},
	color.RGBA{0x88, 0xcc, 0x00, 0xff},
	color.RGBA{0x88, 0xcc, 0x44, 0xff},
	color.RGBA{0x88, 0xcc, 0x88, 0xff},
	color.RGBA{0x88, 0xcc, 0xcc, 0xff},
	color.RGBA{0x88, 0x00, 0x00, 0xff},
	color.RGBA{0x88, 0x00, 0x44, 0xff},
	color.RGBA{0x99, 0x00, 0x4c, 0xff},
	color.RGBA{0x99, 0x00, 0x99, 0xff},
	color.RGBA{0x93, 0x00, 0xdd, 0xff},
	color.RGBA{0x99, 0x4c, 0x00, 0xff},
	color.RGBA{0x99, 0x4c, 0x4c, 0xff},
	color.RGBA{0x99, 0x4c, 0x99, 0xff},
	color.RGBA{0x93, 0x49, 0xdd, 0xff},
	color.RGBA{0x99, 0x99, 0x00, 0xff},
	color.RGBA{0x99, 0x99, 0x4c, 0xff},
	color.RGBA{0x99, 0x99, 0x99, 0xff},
	color.RGBA{0x93, 0x93, 0xdd, 0xff},
	color.RGBA{0x93, 0xdd, 0x00, 0xff},
	color.RGBA{0x93, 0xdd, 0x49, 0xff},
	color.RGBA{0x93, 0xdd, 0x93, 0xff},
	color.RGBA{0x93, 0xdd, 0xdd, 0xff},
	color.RGBA{0x99, 0x00, 0x00, 0xff},
	color.RGBA{0xaa, 0x00, 0x00, 0xff},
	color.RGBA{0xaa, 0x00, 0x55, 0xff},
	color.RGBA{0xaa, 0x00, 0xaa, 0xff},
	color.RGBA{0x9e, 0x00, 0xee, 0xff},
	color.RGBA{0xaa, 0x55, 0x00, 0xff},
	color.RGBA{0xaa, 0x55, 0x55, 0xff},
	color.RGBA{0xaa, 0x55, 0xaa, 0xff},
	color.RGBA{0x9e, 0x4f, 0xee, 0xff},
	color.RGBA{0xaa, 0xaa, 0x00, 0xff},
	color.RGBA{0xaa, 0xaa, 0x55, 0xff},
	color.RGBA{0xaa, 0xaa, 0xaa, 0xff},
	color.RGBA{0x9e, 0x9e, 0xee, 0xff},
	color.RGBA{0x9e, 0xee, 0x00, 0xff},
	color.RGBA{0x9e, 0xee, 0x4f, 0xff},
	color.RGBA{0x9e, 0xee, 0x9e, 0xff},
	color.RGBA{0x9e, 0xee, 0xee, 0xff},
	color.RGBA{0xaa, 0xff, 0xff, 0xff},
	color.RGBA{0xbb, 0x00, 0x00, 0xff},
	color.RGBA{0xbb, 0x00, 0x5d, 0xff},
	color.RGBA{0xbb, 0x00, 0xbb, 0xff},
	color.RGBA{0xaa, 0x00, 0xff, 0xff},
	color.RGBA{0xbb, 0x5d, 0x00, 0xff},
	color.RGBA{0xbb, 0x5d, 0x5d, 0xff},
	color.RGBA{0xbb, 0x5d, 0xbb, 0xff},
	color.RGBA{0xaa, 0x55, 0xff, 0xff},
	color.RGBA{0xbb, 0xbb, 0x00, 0xff},
	color.RGBA{0xbb, 0xbb, 0x5d, 0xff},
	color.RGBA{0xbb, 0xbb, 0xbb, 0xff},
	color.RGBA{0xaa, 0xaa, 0xff, 0xff},
	color.RGBA{0xaa, 0xff, 0x00, 0xff},
	color.RGBA{0xaa, 0xff, 0x55, 0xff},
	color.RGBA{0xaa, 0xff, 0xaa, 0xff},
	color.RGBA{0xcc, 0x00, 0xcc, 0xff},
	color.RGBA{0xcc, 0x44, 0x00, 0xff},
	color.RGBA{0xcc, 0x44, 0x44, 0xff},
	color.RGBA{0xcc, 0x44, 0x88, 0xff},
	color.RGBA{0xcc, 0x44, 0xcc, 0xff},
	color.RGBA{0xcc, 0x88, 0x00, 0xff},
	color.RGBA{0xcc, 0x88, 0x44, 0xff},
	color.RGBA{0xcc, 0x88, 0x88, 0xff},
	color.RGBA{0xcc, 0x88, 0xcc, 0xff},
	color.RGBA{0xcc, 0xcc, 0x00, 0xff},
	color.RGBA{0xcc, 0xcc, 0x44, 0xff},
	color.RGBA{0xcc, 0xcc, 0x88, 0xff},
	color.RGBA{0xcc, 0xcc, 0xcc, 0xff},
	color.RGBA{0xcc, 0x00, 0x00, 0xff},
	color.RGBA{0xcc, 0x00, 0x44, 0xff},
	color.RGBA{0xcc, 0x00, 0x88, 0xff},
	color.RGBA{0xdd, 0x00, 0x93, 0xff},
	color.RGBA{0xdd, 0x00, 0xdd, 0xff},
	color.RGBA{0xdd, 0x49, 0x00, 0xff},
	color.RGBA{0xdd, 0x49, 0x49, 0xff},
	color.RGBA{0xdd, 0x49, 0x93, 0xff},
	color.RGBA{0xdd, 0x49, 0xdd, 0xff},
	color.RGBA{0xdd, 0x93, 0x00, 0xff},
	color.RGBA{0xdd, 0x93, 0x49, 0xff},
	color.RGBA{0xdd, 0x93, 0x93, 0xff},
	color.RGBA{0xdd, 0x93, 0xdd, 0xff},
	color.RGBA{0xdd, 0xdd, 0x00, 0xff},
	color.RGBA{0xdd, 0xdd, 0x49, 0xff},
	color.RGBA{0xdd, 0xdd, 0x93, 0xff},
	color.RGBA{0xdd, 0xdd, 0xdd, 0xff},
	color.RGBA{0xdd, 0x00, 0x00, 0xff},
	color.RGBA{0xdd, 0x00, 0x49, 0xff},
	color.RGBA{0xee, 0x00, 0x4f, 0xff},
	color.RGBA{0xee, 0x00, 0x9e, 0xff},
	color.RGBA{0xee, 0x00, 0xee, 0xff},
	color.RGBA{0xee, 0x4f, 0x00, 0xff},
	color.RGBA{0xee, 0x4f, 0x4f, 0xff},
	color.RGBA{0xee, 0x4f, 0x9e, 0xff},
	color.RGBA{0xee, 0x4f, 0xee, 0xff},
	color.RGBA{0xee, 0x9e, 0x00, 0xff},
	color.RGBA{0xee, 0x9e, 0x4f, 0xff},
	color.RGBA{0xee, 0x9e, 0x9e, 0xff},
	color.RGBA{0xee, 0x9e, 0xee, 0xff},
	color.RGBA{0xee, 0xee, 0x00, 0xff},
	color.RGBA{0xee, 0xee, 0x4f, 0xff},
	color.RGBA{0xee, 0xee, 0x9e, 0xff},
	color.RGBA{0xee, 0xee, 0xee, 0xff},
	color.RGBA{0xee, 0x00, 0x00, 0xff},
	color.RGBA{0xff, 0x00, 0x00, 0xff},
	color.RGBA{0xff, 0x00, 0x55, 0xff},
	color.RGBA{0xff, 0x00, 0xaa, 0xff},
	color.RGBA{0xff, 0x00, 0xff, 0xff},
	color.RGBA{0xff, 0x55, 0x00, 0xff},
	color.RGBA{0xff, 0x55, 0x55, 0xff},
	color.RGBA{0xff, 0x55, 0xaa, 0xff},
	color.RGBA{0xff, 0x55, 0xff, 0xff},
	color.RGBA{0xff, 0xaa, 0x00, 0xff},
	color.RGBA{0xff, 0xaa, 0x55, 0xff},
	color.RGBA{0xff, 0xaa, 0xaa, 0xff},
	color.RGBA{0xff, 0xaa, 0xff, 0xff},
	color.RGBA{0xff, 0xff, 0x00, 0xff},
	color.RGBA{0xff, 0xff, 0x55, 0xff},
	color.RGBA{0xff, 0xff, 0xaa, 0xff},
	color.RGBA{0xff, 0xff, 0xff, 0xff},
}

// WebSafe is a 216-color palette that was popularized by early versions
// of Netscape Navigator. It is also known as the Netscape Color Cube.
//
// See https://en.wikipedia.org/wiki/Web_colors#Web-safe_colors for details.
var WebSafe = []color.Color{
	color.RGBA{0x00, 0x00, 0x00, 0xff},
	color.RGBA{0x00, 0x00, 0x33, 0xff},
	color.RGBA{0x00, 0x00, 0x66, 0xff},
	color.RGBA{0x00, 0x00, 0x99, 0xff},
	color.RGBA{0x00, 0x00, 0xcc, 0xff},
	color.RGBA{0x00, 0x00, 0xff, 0xff},
	color.RGBA{0x00, 0x33, 0x00, 0xff},
	color.RGBA{0x00, 0x33, 0x33, 0xff},
	color.RGBA{0x00, 0x33, 0x66, 0xff},
	color.RGBA{0x00, 0x33, 0x99, 0xff},
	color.RGBA{0x00, 0x33, 0xcc, 0xff},
	color.RGBA{0x00, 0x33, 0xff, 0xff},
	color.RGBA{0x00, 0x66, 0x00, 0xff},
	color.RGBA{0x00, 0x66, 0x33, 0xff},
	color.RGBA{0x00, 0x66, 0x66, 0xff},
	color.RGBA{0x00, 0x66, 0x99, 0xff},
	color.RGBA{0x00, 0x66, 0xcc, 0xff},
	color.RGBA{0x00, 0x66, 0xff, 0xff},
	color.RGBA{0x00, 0x99, 0x00, 0xff},
	color.RGBA{0x00, 0x99, 0x33, 0xff},
	color.RGBA{0x00, 0x99, 0x66, 0xff},
	color.RGBA{0x00, 0x99, 0x99, 0xff},
	color.RGBA{0x00, 0x99, 0xcc, 0xff},
	color.RGBA{0x00, 0x99, 0xff, 0xff},
	color.RGBA{0x00, 0xcc, 0x00, 0xff},
	color.RGBA{0x00, 0xcc, 0x33, 0xff},
	color.RGBA{0x00, 0xcc, 0x66, 0xff},
	color.RGBA{0x00, 0xcc, 0x99, 0xff},
	color.RGBA{0x00, 0xcc, 0xcc, 0xff},
	color.RGBA{0x00, 0xcc, 0xff, 0xff},
	color.RGBA{0x00, 0xff, 0x00, 0xff},
	color.RGBA{0x00, 0xff, 0x33, 0xff},
	color.RGBA{0x00, 0xff, 0x66, 0xff},
	color.RGBA{0x00, 0xff, 0x99, 0xff},
	color.RGBA{0x00, 0xff, 0xcc, 0xff},
	color.RGBA{0x00, 0xff, 0xff, 0xff},
	color.RGBA{0x33, 0x00, 0x00, 0xff},
	color.RGBA{0x33, 0x00, 0x33, 0xff},
	color.RGBA{0x33, 0x00, 0x66, 0xff},
	color.RGBA{0x33, 0x00, 0x99, 0xff},
	color.RGBA{0x33, 0x00, 0xcc, 0xff},
	color.RGBA{0x33, 0x00, 0xff, 0xff},
	color.RGBA{0x33, 0x33, 0x00, 0xff},
	color.RGBA{0x33, 0x33, 0x33, 0xff},
	color.RGBA{0x33, 0x33, 0x66, 0xff},
	color.RGBA{0x33, 0x33, 0x99, 0xff},
	color.RGBA{0x33, 0x33, 0xcc, 0xff},
	color.RGBA{0x33, 0x33, 0xff, 0xff},
	color.RGBA{0x33, 0x66, 0x00, 0xff},
	color.RGBA{0x33, 0x66, 0x33, 0xff},
	color.RGBA{0x33, 0x66, 0x66, 0xff},
	color.RGBA{0x33, 0x66, 0x99, 0xff},
	color.RGBA{0x33, 0x66, 0xcc, 0xff},
	color.RGBA{0x33, 0x66, 0xff, 0xff},
	color.RGBA{0x33, 0x99, 0x00, 0xff},
	color.RGBA{0x33, 0x99, 0x33, 0xff},
	color.RGBA{0x33, 0x99, 0x66, 0xff},
	color.RGBA{0x33, 0x99, 0x99, 0xff},
	color.RGBA{0x33, 0x99, 0xcc, 0xff},
	color.RGBA{0x33, 0x99, 0xff, 0xff},
	color.RGBA{0x33, 0xcc, 0x00, 0xff},
	color.RGBA{0x33, 0xcc, 0x33, 0xff},
	color.RGBA{0x33, 0xcc, 0x66, 0xff},
	color.RGBA{0x33, 0xcc, 0x99, 0xff},
	color.RGBA{0x33, 0xcc, 0xcc, 0xff},
	color.RGBA{0x33, 0xcc, 0xff, 0xff},
	color.RGBA{0x33, 0xff, 0x00, 0xff},
	color.RGBA{0x33, 0xff, 0x33, 0xff},
	color.RGBA{0x33, 0xff, 0x66, 0xff},
	color.RGBA{0x33, 0xff, 0x99, 0xff},
	color.RGBA{0x33, 0xff, 0xcc, 0xff},
	color.RGBA{0x33, 0xff, 0xff, 0xff},
	color.RGBA{0x66, 0x00, 0x00, 0xff},
	color.RGBA{0x66, 0x00, 0x33, 0xff},
	color.RGBA{0x66, 0x00, 0x66, 0xff},
	color.RGBA{0x66, 0x00, 0x99, 0xff},
	color.RGBA{0x66, 0x00, 0xcc, 0xff},
	color.RGBA{0x66, 0x00, 0xff, 0xff},
	color.RGBA{0x66, 0x33, 0x00, 0xff},
	color.RGBA{0x66, 0x33, 0x33, 0xff},
	color.RGBA{0x66, 0x33, 0x66, 0xff},
	color.RGBA{0x66, 0x33, 0x99, 0xff},
	color.RGBA{0x66, 0x33, 0xcc, 0xff},
	color.RGBA{0x66, 0x33, 0xff, 0xff},
	color.RGBA{0x66, 0x66, 0x00, 0xff},
	color.RGBA{0x66, 0x66, 0x33, 0xff},
	color.RGBA{0x66, 0x66, 0x66, 0xff},
	color.RGBA{0x66, 0x66, 0x99, 0xff},
	color.RGBA{0x66, 0x66, 0xcc, 0xff},
	color.RGBA{0x66, 0x66, 0xff, 0xff},
	color.RGBA{0x66, 0x99, 0x00, 0xff},
	color.RGBA{0x66, 0x99, 0x33, 0xff},
	color.RGBA{0x66, 0x99, 0x66, 0xff},
	color.RGBA{0x66, 0x99, 0x99, 0xff},
	color.RGBA{0x66, 0x99, 0xcc, 0xff},
	color.RGBA{0x66, 0x99, 0xff, 0xff},
	color.RGBA{0x66, 0xcc, 0x00, 0xff},
	color.RGBA{0x66, 0xcc, 0x33, 0xff},
	color.RGBA{0x66, 0xcc, 0x66, 0xff},
	color.RGBA{0x66, 0xcc, 0x99, 0xff},
	color.RGBA{0x66, 0xcc, 0xcc, 0xff},
	color.RGBA{0x66, 0xcc, 0xff, 0xff},
	color.RGBA{0x66, 0xff, 0x00, 0xff},
	color.RGBA{0x66, 0xff, 0x33, 0xff},
	color.RGBA{0x66, 0xff, 0x66, 0xff},
	color.RGBA{0x66, 0xff, 0x99, 0xff},
	color.RGBA{0x66, 0xff, 0xcc, 0xff},
	color.RGBA{0x66, 0xff, 0xff, 0xff},
	color.RGBA{0x99, 0x00, 0x00, 0xff},
	color.RGBA{0x99, 0x00, 0x33, 0xff},
	color.RGBA{0x99, 0x00, 0x66, 0xff},
	color.RGBA{0x99, 0x00, 0x99, 0xff},
	color.RGBA{0x99, 0x00, 0xcc, 0xff},
	color.RGBA{0x99, 0x00, 0xff, 0xff},
	color.RGBA{0x99, 0x33, 0x00, 0xff},
	color.RGBA{0x99, 0x33, 0x33, 0xff},
	color.RGBA{0x99, 0x33, 0x66, 0xff},
	color.RGBA{0x99, 0x33, 0x99, 0xff},
	color.RGBA{0x99, 0x33, 0xcc, 0xff},
	color.RGBA{0x99, 0x33, 0xff, 0xff},
	color.RGBA{0x99, 0x66, 0x00, 0xff},
	color.RGBA{0x99, 0x66, 0x33, 0xff},
	color.RGBA{0x99, 0x66, 0x66, 0xff},
	color.RGBA{0x99, 0x66, 0x99, 0xff},
	color.RGBA{0x99, 0x66, 0xcc, 0xff},
	color.RGBA{0x99, 0x66, 0xff, 0xff},
	color.RGBA{0x99, 0x99, 0x00, 0xff},
	color.RGBA{0x99, 0x99, 0x33, 0xff},
	color.RGBA{0x99, 0x99, 0x66, 0xff},
	color.RGBA{0x99, 0x99, 0x99, 0xff},
	color.RGBA{0x99, 0x99, 0xcc, 0xff},
	color.RGBA{0x99, 0x99, 0xff, 0xff},
	color.RGBA{0x99, 0xcc, 0x00, 0xff},
	color.RGBA{0x99, 0xcc, 0x33, 0xff},
	color.RGBA{0x99, 0xcc, 0x66, 0xff},
	color.RGBA{0x99, 0xcc, 0x99, 0xff},
	color.RGBA{0x99, 0xcc, 0xcc, 0xff},
	color.RGBA{0x99, 0xcc, 0xff, 0xff},
	color.RGBA{0x99, 0xff, 0x00, 0xff},
	color.RGBA{0x99, 0xff, 0x33, 0xff},
	color.RGBA{0x99, 0xff, 0x66, 0xff},
	color.RGBA{0x99, 0xff, 0x99, 0xff},
	color.RGBA{0x99, 0xff, 0xcc, 0xff},
	color.RGBA{0x99, 0xff, 0xff, 0xff},
	color.RGBA{0xcc, 0x00, 0x00, 0xff},
	color.RGBA{0xcc, 0x00, 0x33, 0xff},
	color.RGBA{0xcc, 0x00, 0x66, 0xff},
	color.RGBA{0xcc, 0x00, 0x99, 0xff},
	color.RGBA{0xcc, 0x00, 0xcc, 0xff},
	color.RGBA{0xcc, 0x00, 0xff, 0xff},
	color.RGBA{0xcc, 0x33, 0x00, 0xff},
	color.RGBA{0xcc, 0x33, 0x33, 0xff},
	color.RGBA{0xcc, 0x33, 0x66, 0xff},
	color.RGBA{0xcc, 0x33, 0x99, 0xff},
	color.RGBA{0xcc, 0x33, 0xcc, 0xff},
	color.RGBA{0xcc, 0x33, 0xff, 0xff},
	color.RGBA{0xcc, 0x66, 0x00, 0xff},
	color.RGBA{0xcc, 0x66, 0x33, 0xff},
	color.RGBA{0xcc, 0x66, 0x66, 0xff},
	color.RGBA{0xcc, 0x66, 0x99, 0xff},
	color.RGBA{0xcc, 0x66, 0xcc, 0xff},
	color.RGBA{0xcc, 0x66, 0xff, 0xff},
	color.RGBA{0xcc, 0x99, 0x00, 0xff},
	color.RGBA{0xcc, 0x99, 0x33, 0xff},
	color.RGBA{0xcc, 0x99, 0x66, 0xff},
	color.RGBA{0xcc, 0x99, 0x99, 0xff},
	color.RGBA{0xcc, 0x99, 0xcc, 0xff},
	color.RGBA{0xcc, 0x99, 0xff, 0xff},
	color.RGBA{0xcc, 0xcc, 0x00, 0xff},
	color.RGBA{0xcc, 0xcc, 0x33, 0xff},
	color.RGBA{0xcc, 0xcc, 0x66, 0xff},
	color.RGBA{0xcc, 0xcc, 0x99, 0xff},
	color.RGBA{0xcc, 0xcc, 0xcc, 0xff},
	color.RGBA{0xcc, 0xcc, 0xff, 0xff},
	color.RGBA{0xcc, 0xff, 0x00, 0xff},
	color.RGBA{0xcc, 0xff, 0x33, 0xff},
	color.RGBA{0xcc, 0xff, 0x66, 0xff},
	color.RGBA{0xcc, 0xff, 0x99, 0xff},
	color.RGBA{0xcc, 0xff, 0xcc, 0xff},
	color.RGBA{0xcc, 0xff, 0xff, 0xff},
	color.RGBA{0xff, 0x00, 0x00, 0xff},
	color.RGBA{0xff, 0x00, 0x33, 0xff},
	color.RGBA{0xff, 0x00, 0x66, 0xff},
	color.RGBA{0xff, 0x00, 0x99, 0xff},
	color.RGBA{0xff, 0x00, 0xcc, 0xff},
	color.RGBA{0xff, 0x00, 0xff, 0xff},
	color.RGBA{0xff, 0x33, 0x00, 0xff},
	color.RGBA{0xff, 0x33, 0x33, 0xff},
	color.RGBA{0xff, 0x33, 0x66, 0xff},
	color.RGBA{0xff, 0x33, 0x99, 0xff},
	color.RGBA{0xff, 0x33, 0xcc, 0xff},
	color.RGBA{0xff, 0x33, 0xff, 0xff},
	color.RGBA{0xff, 0x66, 0x00, 0xff},
	color.RGBA{0xff, 0x66, 0x33, 0xff},
	color.RGBA{0xff, 0x66, 0x66, 0xff},
	color.RGBA{0xff, 0x66, 0x99, 0xff},
	color.RGBA{0xff, 0x66, 0xcc, 0xff},
	color.RGBA{0xff, 0x66, 0xff, 0xff},
	color.RGBA{0xff, 0x99, 0x00, 0xff},
	color.RGBA{0xff, 0x99, 0x33, 0xff},
	color.RGBA{0xff, 0x99, 0x66, 0xff},
	color.RGBA{0xff, 0x99, 0x99, 0xff},
	color.RGBA{0xff, 0x99, 0xcc, 0xff},
	color.RGBA{0xff, 0x99, 0xff, 0xff},
	color.RGBA{0xff, 0xcc, 0x00, 0xff},
	color.RGBA{0xff, 0xcc, 0x33, 0xff},
	color.RGBA{0xff, 0xcc, 0x66, 0xff},
	color.RGBA{0xff, 0xcc, 0x99, 0xff},
	color.RGBA{0xff, 0xcc, 0xcc, 0xff},
	color.RGBA{0xff, 0xcc, 0xff, 0xff},
	color.RGBA{0xff, 0xff, 0x00, 0xff},
	color.RGBA{0xff, 0xff, 0x33, 0xff},
	color.RGBA{0xff, 0xff, 0x66, 0xff},
	color.RGBA{0xff, 0xff, 0x99, 0xff},
	color.RGBA{0xff, 0xff, 0xcc, 0xff},
	color.RGBA{0xff, 0xff, 0xff, 0xff},
}

```

// === FILE: references/go/src/image/color/ycbcr.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package color

// RGBToYCbCr converts an RGB triple to a Y'CbCr triple.
func RGBToYCbCr(r, g, b uint8) (uint8, uint8, uint8) {
	// The JFIF specification says:
	//	Y' =  0.2990*R + 0.5870*G + 0.1140*B
	//	Cb = -0.1687*R - 0.3313*G + 0.5000*B + 128
	//	Cr =  0.5000*R - 0.4187*G - 0.0813*B + 128
	// https://www.w3.org/Graphics/JPEG/jfif3.pdf says Y but means Y'.

	r1 := int32(r)
	g1 := int32(g)
	b1 := int32(b)

	// yy is in range [0,0xff].
	//
	// Note that 19595 + 38470 + 7471 equals 65536.
	yy := (19595*r1 + 38470*g1 + 7471*b1 + 1<<15) >> 16

	// The bit twiddling below is equivalent to
	//
	// cb := (-11056*r1 - 21712*g1 + 32768*b1 + 257<<15) >> 16
	// if cb < 0 {
	//     cb = 0
	// } else if cb > 0xff {
	//     cb = ^int32(0)
	// }
	//
	// but uses fewer branches and is faster.
	// Note that the uint8 type conversion in the return
	// statement will convert ^int32(0) to 0xff.
	// The code below to compute cr uses a similar pattern.
	//
	// Note that -11056 - 21712 + 32768 equals 0.
	cb := -11056*r1 - 21712*g1 + 32768*b1 + 257<<15
	if uint32(cb)&0xff000000 == 0 {
		cb >>= 16
	} else {
		cb = ^(cb >> 31)
	}

	// Note that 32768 - 27440 - 5328 equals 0.
	cr := 32768*r1 - 27440*g1 - 5328*b1 + 257<<15
	if uint32(cr)&0xff000000 == 0 {
		cr >>= 16
	} else {
		cr = ^(cr >> 31)
	}

	return uint8(yy), uint8(cb), uint8(cr)
}

// YCbCrToRGB converts a Y'CbCr triple to an RGB triple.
func YCbCrToRGB(y, cb, cr uint8) (uint8, uint8, uint8) {
	// The JFIF specification says:
	//	R = Y' + 1.40200*(Cr-128)
	//	G = Y' - 0.34414*(Cb-128) - 0.71414*(Cr-128)
	//	B = Y' + 1.77200*(Cb-128)
	// https://www.w3.org/Graphics/JPEG/jfif3.pdf says Y but means Y'.
	//
	// Those formulae use non-integer multiplication factors. When computing,
	// integer math is generally faster than floating point math. We multiply
	// all of those factors by 1<<16 and round to the nearest integer:
	//	 91881 = roundToNearestInteger(1.40200 * 65536).
	//	 22554 = roundToNearestInteger(0.34414 * 65536).
	//	 46802 = roundToNearestInteger(0.71414 * 65536).
	//	116130 = roundToNearestInteger(1.77200 * 65536).
	//
	// Adding a rounding adjustment in the range [0, 1<<16-1] and then shifting
	// right by 16 gives us an integer math version of the original formulae.
	//	R = (65536*Y' +  91881 *(Cr-128)                  + adjustment) >> 16
	//	G = (65536*Y' -  22554 *(Cb-128) - 46802*(Cr-128) + adjustment) >> 16
	//	B = (65536*Y' + 116130 *(Cb-128)                  + adjustment) >> 16
	// A constant rounding adjustment of 1<<15, one half of 1<<16, would mean
	// round-to-nearest when dividing by 65536 (shifting right by 16).
	// Similarly, a constant rounding adjustment of 0 would mean round-down.
	//
	// Defining YY1 = 65536*Y' + adjustment simplifies the formulae and
	// requires fewer CPU operations:
	//	R = (YY1 +  91881 *(Cr-128)                 ) >> 16
	//	G = (YY1 -  22554 *(Cb-128) - 46802*(Cr-128)) >> 16
	//	B = (YY1 + 116130 *(Cb-128)                 ) >> 16
	//
	// The inputs (y, cb, cr) are 8 bit color, ranging in [0x00, 0xff]. In this
	// function, the output is also 8 bit color, but in the related YCbCr.RGBA
	// method, below, the output is 16 bit color, ranging in [0x0000, 0xffff].
	// Outputting 16 bit color simply requires changing the 16 to 8 in the "R =
	// etc >> 16" equation, and likewise for G and B.
	//
	// As mentioned above, a constant rounding adjustment of 1<<15 is a natural
	// choice, but there is an additional constraint: if c0 := YCbCr{Y: y, Cb:
	// 0x80, Cr: 0x80} and c1 := Gray{Y: y} then c0.RGBA() should equal
	// c1.RGBA(). Specifically, if y == 0 then "R = etc >> 8" should yield
	// 0x0000 and if y == 0xff then "R = etc >> 8" should yield 0xffff. If we
	// used a constant rounding adjustment of 1<<15, then it would yield 0x0080
	// and 0xff80 respectively.
	//
	// Note that when cb == 0x80 and cr == 0x80 then the formulae collapse to:
	//	R = YY1 >> n
	//	G = YY1 >> n
	//	B = YY1 >> n
	// where n is 16 for this function (8 bit color output) and 8 for the
	// YCbCr.RGBA method (16 bit color output).
	//
	// The solution is to make the rounding adjustment non-constant, and equal
	// to 257*Y', which ranges over [0, 1<<16-1] as Y' ranges over [0, 255].
	// YY1 is then defined as:
	//	YY1 = 65536*Y' + 257*Y'
	// or equivalently:
	//	YY1 = Y' * 0x10101
	yy1 := int32(y) * 0x10101
	cb1 := int32(cb) - 128
	cr1 := int32(cr) - 128

	// The bit twiddling below is equivalent to
	//
	// r := (yy1 + 91881*cr1) >> 16
	// if r < 0 {
	//     r = 0
	// } else if r > 0xff {
	//     r = ^int32(0)
	// }
	//
	// but uses fewer branches and is faster.
	// Note that the uint8 type conversion in the return
	// statement will convert ^int32(0) to 0xff.
	// The code below to compute g and b uses a similar pattern.
	r := yy1 + 91881*cr1
	if uint32(r)&0xff000000 == 0 {
		r >>= 16
	} else {
		r = ^(r >> 31)
	}

	g := yy1 - 22554*cb1 - 46802*cr1
	if uint32(g)&0xff000000 == 0 {
		g >>= 16
	} else {
		g = ^(g >> 31)
	}

	b := yy1 + 116130*cb1
	if uint32(b)&0xff000000 == 0 {
		b >>= 16
	} else {
		b = ^(b >> 31)
	}

	return uint8(r), uint8(g), uint8(b)
}

// YCbCr represents a fully opaque 24-bit Y'CbCr color, having 8 bits each for
// one luma and two chroma components.
//
// JPEG, VP8, the MPEG family and other codecs use this color model. Such
// codecs often use the terms YUV and Y'CbCr interchangeably, but strictly
// speaking, the term YUV applies only to analog video signals, and Y' (luma)
// is Y (luminance) after applying gamma correction.
//
// Conversion between RGB and Y'CbCr is lossy and there are multiple, slightly
// different formulae for converting between the two. This package follows
// the JFIF specification at https://www.w3.org/Graphics/JPEG/jfif3.pdf.
type YCbCr struct {
	Y, Cb, Cr uint8
}

func (c YCbCr) RGBA() (uint32, uint32, uint32, uint32) {
	// This code is a copy of the YCbCrToRGB function above, except that it
	// returns values in the range [0, 0xffff] instead of [0, 0xff]. There is a
	// subtle difference between doing this and having YCbCr satisfy the Color
	// interface by first converting to an RGBA. The latter loses some
	// information by going to and from 8 bits per channel.
	//
	// For example, this code:
	//	const y, cb, cr = 0x7f, 0x7f, 0x7f
	//	r, g, b := color.YCbCrToRGB(y, cb, cr)
	//	r0, g0, b0, _ := color.YCbCr{y, cb, cr}.RGBA()
	//	r1, g1, b1, _ := color.RGBA{r, g, b, 0xff}.RGBA()
	//	fmt.Printf("0x%04x 0x%04x 0x%04x\n", r0, g0, b0)
	//	fmt.Printf("0x%04x 0x%04x 0x%04x\n", r1, g1, b1)
	// prints:
	//	0x7e18 0x808d 0x7db9
	//	0x7e7e 0x8080 0x7d7d

	yy1 := int32(c.Y) * 0x10101
	cb1 := int32(c.Cb) - 128
	cr1 := int32(c.Cr) - 128

	// The bit twiddling below is equivalent to
	//
	// r := (yy1 + 91881*cr1) >> 8
	// if r < 0 {
	//     r = 0
	// } else if r > 0xff {
	//     r = 0xffff
	// }
	//
	// but uses fewer branches and is faster.
	// The code below to compute g and b uses a similar pattern.
	r := yy1 + 91881*cr1
	if uint32(r)&0xff000000 == 0 {
		r >>= 8
	} else {
		r = ^(r >> 31) & 0xffff
	}

	g := yy1 - 22554*cb1 - 46802*cr1
	if uint32(g)&0xff000000 == 0 {
		g >>= 8
	} else {
		g = ^(g >> 31) & 0xffff
	}

	b := yy1 + 116130*cb1
	if uint32(b)&0xff000000 == 0 {
		b >>= 8
	} else {
		b = ^(b >> 31) & 0xffff
	}

	return uint32(r), uint32(g), uint32(b), 0xffff
}

// YCbCrModel is the [Model] for Y'CbCr colors.
var YCbCrModel Model = ModelFunc(yCbCrModel)

func yCbCrModel(c Color) Color {
	if _, ok := c.(YCbCr); ok {
		return c
	}
	r, g, b, _ := c.RGBA()
	y, u, v := RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))
	return YCbCr{y, u, v}
}

// NYCbCrA represents a non-alpha-premultiplied Y'CbCr-with-alpha color, having
// 8 bits each for one luma, two chroma and one alpha component.
type NYCbCrA struct {
	YCbCr
	A uint8
}

func (c NYCbCrA) RGBA() (uint32, uint32, uint32, uint32) {
	// The first part of this method is the same as YCbCr.RGBA.
	yy1 := int32(c.Y) * 0x10101
	cb1 := int32(c.Cb) - 128
	cr1 := int32(c.Cr) - 128

	// The bit twiddling below is equivalent to
	//
	// r := (yy1 + 91881*cr1) >> 8
	// if r < 0 {
	//     r = 0
	// } else if r > 0xff {
	//     r = 0xffff
	// }
	//
	// but uses fewer branches and is faster.
	// The code below to compute g and b uses a similar pattern.
	r := yy1 + 91881*cr1
	if uint32(r)&0xff000000 == 0 {
		r >>= 8
	} else {
		r = ^(r >> 31) & 0xffff
	}

	g := yy1 - 22554*cb1 - 46802*cr1
	if uint32(g)&0xff000000 == 0 {
		g >>= 8
	} else {
		g = ^(g >> 31) & 0xffff
	}

	b := yy1 + 116130*cb1
	if uint32(b)&0xff000000 == 0 {
		b >>= 8
	} else {
		b = ^(b >> 31) & 0xffff
	}

	// The second part of this method applies the alpha.
	a := uint32(c.A) * 0x101
	return uint32(r) * a / 0xffff, uint32(g) * a / 0xffff, uint32(b) * a / 0xffff, a
}

// NYCbCrAModel is the [Model] for non-alpha-premultiplied Y'CbCr-with-alpha
// colors.
var NYCbCrAModel Model = ModelFunc(nYCbCrAModel)

func nYCbCrAModel(c Color) Color {
	switch c := c.(type) {
	case NYCbCrA:
		return c
	case YCbCr:
		return NYCbCrA{c, 0xff}
	}
	r, g, b, a := c.RGBA()

	// Convert from alpha-premultiplied to non-alpha-premultiplied.
	if a != 0 {
		r = (r * 0xffff) / a
		g = (g * 0xffff) / a
		b = (b * 0xffff) / a
	}

	y, u, v := RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))
	return NYCbCrA{YCbCr{Y: y, Cb: u, Cr: v}, uint8(a >> 8)}
}

// RGBToCMYK converts an RGB triple to a CMYK quadruple.
func RGBToCMYK(r, g, b uint8) (uint8, uint8, uint8, uint8) {
	rr := uint32(r)
	gg := uint32(g)
	bb := uint32(b)
	w := rr
	if w < gg {
		w = gg
	}
	if w < bb {
		w = bb
	}
	if w == 0 {
		return 0, 0, 0, 0xff
	}
	c := (w - rr) * 0xff / w
	m := (w - gg) * 0xff / w
	y := (w - bb) * 0xff / w
	return uint8(c), uint8(m), uint8(y), uint8(0xff - w)
}

// CMYKToRGB converts a [CMYK] quadruple to an RGB triple.
func CMYKToRGB(c, m, y, k uint8) (uint8, uint8, uint8) {
	w := 0xffff - uint32(k)*0x101
	r := (0xffff - uint32(c)*0x101) * w / 0xffff
	g := (0xffff - uint32(m)*0x101) * w / 0xffff
	b := (0xffff - uint32(y)*0x101) * w / 0xffff
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

// CMYK represents a fully opaque CMYK color, having 8 bits for each of cyan,
// magenta, yellow and black.
//
// It is not associated with any particular color profile.
type CMYK struct {
	C, M, Y, K uint8
}

func (c CMYK) RGBA() (uint32, uint32, uint32, uint32) {
	// This code is a copy of the CMYKToRGB function above, except that it
	// returns values in the range [0, 0xffff] instead of [0, 0xff].

	w := 0xffff - uint32(c.K)*0x101
	r := (0xffff - uint32(c.C)*0x101) * w / 0xffff
	g := (0xffff - uint32(c.M)*0x101) * w / 0xffff
	b := (0xffff - uint32(c.Y)*0x101) * w / 0xffff
	return r, g, b, 0xffff
}

// CMYKModel is the [Model] for CMYK colors.
var CMYKModel Model = ModelFunc(cmykModel)

func cmykModel(c Color) Color {
	if _, ok := c.(CMYK); ok {
		return c
	}
	r, g, b, _ := c.RGBA()
	cc, mm, yy, kk := RGBToCMYK(uint8(r>>8), uint8(g>>8), uint8(b>>8))
	return CMYK{cc, mm, yy, kk}
}

```

// === FILE: references/go/src/image/draw/draw.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package draw provides image composition functions.
//
// See "The Go image/draw package" for an introduction to this package:
// https://golang.org/doc/articles/image_draw.html
package draw

import (
	"image"
	"image/color"
	"image/internal/imageutil"
)

// m is the maximum color value returned by image.Color.RGBA.
const m = 1<<16 - 1

// Image is an image.Image with a Set method to change a single pixel.
type Image interface {
	image.Image
	Set(x, y int, c color.Color)
}

// RGBA64Image extends both the [Image] and [image.RGBA64Image] interfaces with a
// SetRGBA64 method to change a single pixel. SetRGBA64 is equivalent to
// calling Set, but it can avoid allocations from converting concrete color
// types to the [color.Color] interface type.
type RGBA64Image interface {
	image.RGBA64Image
	Set(x, y int, c color.Color)
	SetRGBA64(x, y int, c color.RGBA64)
}

// Quantizer produces a palette for an image.
type Quantizer interface {
	// Quantize appends up to cap(p) - len(p) colors to p and returns the
	// updated palette suitable for converting m to a paletted image.
	Quantize(p color.Palette, m image.Image) color.Palette
}

// Op is a Porter-Duff compositing operator.
type Op int

const (
	// Over specifies ``(src in mask) over dst''.
	Over Op = iota
	// Src specifies ``src in mask''.
	Src
)

// Draw implements the [Drawer] interface by calling the Draw function with this
// [Op].
func (op Op) Draw(dst Image, r image.Rectangle, src image.Image, sp image.Point) {
	DrawMask(dst, r, src, sp, nil, image.Point{}, op)
}

// Drawer contains the [Draw] method.
type Drawer interface {
	// Draw aligns r.Min in dst with sp in src and then replaces the
	// rectangle r in dst with the result of drawing src on dst.
	Draw(dst Image, r image.Rectangle, src image.Image, sp image.Point)
}

// FloydSteinberg is a [Drawer] that is the [Src] [Op] with Floyd-Steinberg error
// diffusion.
var FloydSteinberg Drawer = floydSteinberg{}

type floydSteinberg struct{}

func (floydSteinberg) Draw(dst Image, r image.Rectangle, src image.Image, sp image.Point) {
	clip(dst, &r, src, &sp, nil, nil)
	if r.Empty() {
		return
	}
	drawPaletted(dst, r, src, sp, true)
}

// clip clips r against each image's bounds (after translating into the
// destination image's coordinate space) and shifts the points sp and mp by
// the same amount as the change in r.Min.
func clip(dst Image, r *image.Rectangle, src image.Image, sp *image.Point, mask image.Image, mp *image.Point) {
	orig := r.Min
	*r = r.Intersect(dst.Bounds())
	*r = r.Intersect(src.Bounds().Add(orig.Sub(*sp)))
	if mask != nil {
		*r = r.Intersect(mask.Bounds().Add(orig.Sub(*mp)))
	}
	dx := r.Min.X - orig.X
	dy := r.Min.Y - orig.Y
	if dx == 0 && dy == 0 {
		return
	}
	sp.X += dx
	sp.Y += dy
	if mp != nil {
		mp.X += dx
		mp.Y += dy
	}
}

func processBackward(dst image.Image, r image.Rectangle, src image.Image, sp image.Point) bool {
	return dst == src &&
		r.Overlaps(r.Add(sp.Sub(r.Min))) &&
		(sp.Y < r.Min.Y || (sp.Y == r.Min.Y && sp.X < r.Min.X))
}

// Draw calls [DrawMask] with a nil mask.
func Draw(dst Image, r image.Rectangle, src image.Image, sp image.Point, op Op) {
	DrawMask(dst, r, src, sp, nil, image.Point{}, op)
}

// DrawMask aligns r.Min in dst with sp in src and mp in mask and then replaces the rectangle r
// in dst with the result of a Porter-Duff composition. A nil mask is treated as opaque.
func DrawMask(dst Image, r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point, op Op) {
	clip(dst, &r, src, &sp, mask, &mp)
	if r.Empty() {
		return
	}

	// Fast paths for special cases. If none of them apply, then we fall back
	// to general but slower implementations.
	//
	// For NRGBA and NRGBA64 image types, the code paths aren't just faster.
	// They also avoid the information loss that would otherwise occur from
	// converting non-alpha-premultiplied color to and from alpha-premultiplied
	// color. See TestDrawSrcNonpremultiplied.
	switch dst0 := dst.(type) {
	case *image.RGBA:
		if op == Over {
			if mask == nil {
				switch src0 := src.(type) {
				case *image.Uniform:
					sr, sg, sb, sa := src0.RGBA()
					if sa == 0xffff {
						drawFillSrc(dst0, r, sr, sg, sb, sa)
					} else {
						drawFillOver(dst0, r, sr, sg, sb, sa)
					}
					return
				case *image.RGBA:
					drawCopyOver(dst0, r, src0, sp)
					return
				case *image.NRGBA:
					drawNRGBAOver(dst0, r, src0, sp)
					return
				case *image.YCbCr:
					// An image.YCbCr is always fully opaque, and so if the
					// mask is nil (i.e. fully opaque) then the op is
					// effectively always Src. Similarly for image.Gray and
					// image.CMYK.
					if imageutil.DrawYCbCr(dst0, r, src0, sp) {
						return
					}
				case *image.Gray:
					drawGray(dst0, r, src0, sp)
					return
				case *image.CMYK:
					drawCMYK(dst0, r, src0, sp)
					return
				}
			} else if mask0, ok := mask.(*image.Alpha); ok {
				switch src0 := src.(type) {
				case *image.Uniform:
					drawGlyphOver(dst0, r, src0, mask0, mp)
					return
				case *image.RGBA:
					drawRGBAMaskOver(dst0, r, src0, sp, mask0, mp)
					return
				case *image.Gray:
					drawGrayMaskOver(dst0, r, src0, sp, mask0, mp)
					return
				// Case order matters. The next case (image.RGBA64Image) is an
				// interface type that the concrete types above also implement.
				case image.RGBA64Image:
					drawRGBA64ImageMaskOver(dst0, r, src0, sp, mask0, mp)
					return
				}
			}
		} else {
			if mask == nil {
				switch src0 := src.(type) {
				case *image.Uniform:
					sr, sg, sb, sa := src0.RGBA()
					drawFillSrc(dst0, r, sr, sg, sb, sa)
					return
				case *image.RGBA:
					d0 := dst0.PixOffset(r.Min.X, r.Min.Y)
					s0 := src0.PixOffset(sp.X, sp.Y)
					drawCopySrc(
						dst0.Pix[d0:], dst0.Stride, r, src0.Pix[s0:], src0.Stride, sp, 4*r.Dx())
					return
				case *image.NRGBA:
					drawNRGBASrc(dst0, r, src0, sp)
					return
				case *image.YCbCr:
					if imageutil.DrawYCbCr(dst0, r, src0, sp) {
						return
					}
				case *image.Gray:
					drawGray(dst0, r, src0, sp)
					return
				case *image.CMYK:
					drawCMYK(dst0, r, src0, sp)
					return
				}
			}
		}
		drawRGBA(dst0, r, src, sp, mask, mp, op)
		return
	case *image.Paletted:
		if op == Src && mask == nil {
			if src0, ok := src.(*image.Uniform); ok {
				colorIndex := uint8(dst0.Palette.Index(src0.C))
				i0 := dst0.PixOffset(r.Min.X, r.Min.Y)
				i1 := i0 + r.Dx()
				for i := i0; i < i1; i++ {
					dst0.Pix[i] = colorIndex
				}
				firstRow := dst0.Pix[i0:i1]
				for y := r.Min.Y + 1; y < r.Max.Y; y++ {
					i0 += dst0.Stride
					i1 += dst0.Stride
					copy(dst0.Pix[i0:i1], firstRow)
				}
				return
			} else if !processBackward(dst, r, src, sp) {
				drawPaletted(dst0, r, src, sp, false)
				return
			}
		}
	case *image.NRGBA:
		if op == Src && mask == nil {
			if src0, ok := src.(*image.NRGBA); ok {
				d0 := dst0.PixOffset(r.Min.X, r.Min.Y)
				s0 := src0.PixOffset(sp.X, sp.Y)
				drawCopySrc(
					dst0.Pix[d0:], dst0.Stride, r, src0.Pix[s0:], src0.Stride, sp, 4*r.Dx())
				return
			}
		}
	case *image.NRGBA64:
		if op == Src && mask == nil {
			if src0, ok := src.(*image.NRGBA64); ok {
				d0 := dst0.PixOffset(r.Min.X, r.Min.Y)
				s0 := src0.PixOffset(sp.X, sp.Y)
				drawCopySrc(
					dst0.Pix[d0:], dst0.Stride, r, src0.Pix[s0:], src0.Stride, sp, 8*r.Dx())
				return
			}
		}
	}

	x0, x1, dx := r.Min.X, r.Max.X, 1
	y0, y1, dy := r.Min.Y, r.Max.Y, 1
	if processBackward(dst, r, src, sp) {
		x0, x1, dx = x1-1, x0-1, -1
		y0, y1, dy = y1-1, y0-1, -1
	}

	// FALLBACK1.17
	//
	// Try the draw.RGBA64Image and image.RGBA64Image interfaces, part of the
	// standard library since Go 1.17. These are like the draw.Image and
	// image.Image interfaces but they can avoid allocations from converting
	// concrete color types to the color.Color interface type.

	if dst0, _ := dst.(RGBA64Image); dst0 != nil {
		if src0, _ := src.(image.RGBA64Image); src0 != nil {
			if mask == nil {
				sy := sp.Y + y0 - r.Min.Y
				my := mp.Y + y0 - r.Min.Y
				for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
					sx := sp.X + x0 - r.Min.X
					mx := mp.X + x0 - r.Min.X
					for x := x0; x != x1; x, sx, mx = x+dx, sx+dx, mx+dx {
						if op == Src {
							dst0.SetRGBA64(x, y, src0.RGBA64At(sx, sy))
						} else {
							srgba := src0.RGBA64At(sx, sy)
							a := m - uint32(srgba.A)
							drgba := dst0.RGBA64At(x, y)
							dst0.SetRGBA64(x, y, color.RGBA64{
								R: uint16((uint32(drgba.R)*a)/m) + srgba.R,
								G: uint16((uint32(drgba.G)*a)/m) + srgba.G,
								B: uint16((uint32(drgba.B)*a)/m) + srgba.B,
								A: uint16((uint32(drgba.A)*a)/m) + srgba.A,
							})
						}
					}
				}
				return

			} else if mask0, _ := mask.(image.RGBA64Image); mask0 != nil {
				sy := sp.Y + y0 - r.Min.Y
				my := mp.Y + y0 - r.Min.Y
				for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
					sx := sp.X + x0 - r.Min.X
					mx := mp.X + x0 - r.Min.X
					for x := x0; x != x1; x, sx, mx = x+dx, sx+dx, mx+dx {
						ma := uint32(mask0.RGBA64At(mx, my).A)
						switch {
						case ma == 0:
							if op == Over {
								// No-op.
							} else {
								dst0.SetRGBA64(x, y, color.RGBA64{})
							}
						case ma == m && op == Src:
							dst0.SetRGBA64(x, y, src0.RGBA64At(sx, sy))
						default:
							srgba := src0.RGBA64At(sx, sy)
							if op == Over {
								drgba := dst0.RGBA64At(x, y)
								a := m - (uint32(srgba.A) * ma / m)
								dst0.SetRGBA64(x, y, color.RGBA64{
									R: uint16((uint32(drgba.R)*a + uint32(srgba.R)*ma) / m),
									G: uint16((uint32(drgba.G)*a + uint32(srgba.G)*ma) / m),
									B: uint16((uint32(drgba.B)*a + uint32(srgba.B)*ma) / m),
									A: uint16((uint32(drgba.A)*a + uint32(srgba.A)*ma) / m),
								})
							} else {
								dst0.SetRGBA64(x, y, color.RGBA64{
									R: uint16(uint32(srgba.R) * ma / m),
									G: uint16(uint32(srgba.G) * ma / m),
									B: uint16(uint32(srgba.B) * ma / m),
									A: uint16(uint32(srgba.A) * ma / m),
								})
							}
						}
					}
				}
				return
			}
		}
	}

	// FALLBACK1.0
	//
	// If none of the faster code paths above apply, use the draw.Image and
	// image.Image interfaces, part of the standard library since Go 1.0.

	var out color.RGBA64
	sy := sp.Y + y0 - r.Min.Y
	my := mp.Y + y0 - r.Min.Y
	for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
		sx := sp.X + x0 - r.Min.X
		mx := mp.X + x0 - r.Min.X
		for x := x0; x != x1; x, sx, mx = x+dx, sx+dx, mx+dx {
			ma := uint32(m)
			if mask != nil {
				_, _, _, ma = mask.At(mx, my).RGBA()
			}
			switch {
			case ma == 0:
				if op == Over {
					// No-op.
				} else {
					dst.Set(x, y, color.Transparent)
				}
			case ma == m && op == Src:
				dst.Set(x, y, src.At(sx, sy))
			default:
				sr, sg, sb, sa := src.At(sx, sy).RGBA()
				if op == Over {
					dr, dg, db, da := dst.At(x, y).RGBA()
					a := m - (sa * ma / m)
					out.R = uint16((dr*a + sr*ma) / m)
					out.G = uint16((dg*a + sg*ma) / m)
					out.B = uint16((db*a + sb*ma) / m)
					out.A = uint16((da*a + sa*ma) / m)
				} else {
					out.R = uint16(sr * ma / m)
					out.G = uint16(sg * ma / m)
					out.B = uint16(sb * ma / m)
					out.A = uint16(sa * ma / m)
				}
				// The third argument is &out instead of out (and out is
				// declared outside of the inner loop) to avoid the implicit
				// conversion to color.Color here allocating memory in the
				// inner loop if sizeof(color.RGBA64) > sizeof(uintptr).
				dst.Set(x, y, &out)
			}
		}
	}
}

func drawFillOver(dst *image.RGBA, r image.Rectangle, sr, sg, sb, sa uint32) {
	// The 0x101 is here for the same reason as in drawRGBA.
	a := (m - sa) * 0x101
	i0 := dst.PixOffset(r.Min.X, r.Min.Y)
	i1 := i0 + r.Dx()*4
	for y := r.Min.Y; y != r.Max.Y; y++ {
		for i := i0; i < i1; i += 4 {
			dr := &dst.Pix[i+0]
			dg := &dst.Pix[i+1]
			db := &dst.Pix[i+2]
			da := &dst.Pix[i+3]

			*dr = uint8((uint32(*dr)*a/m + sr) >> 8)
			*dg = uint8((uint32(*dg)*a/m + sg) >> 8)
			*db = uint8((uint32(*db)*a/m + sb) >> 8)
			*da = uint8((uint32(*da)*a/m + sa) >> 8)
		}
		i0 += dst.Stride
		i1 += dst.Stride
	}
}

func drawFillSrc(dst *image.RGBA, r image.Rectangle, sr, sg, sb, sa uint32) {
	sr8 := uint8(sr >> 8)
	sg8 := uint8(sg >> 8)
	sb8 := uint8(sb >> 8)
	sa8 := uint8(sa >> 8)
	// The built-in copy function is faster than a straightforward for loop to fill the destination with
	// the color, but copy requires a slice source. We therefore use a for loop to fill the first row, and
	// then use the first row as the slice source for the remaining rows.
	i0 := dst.PixOffset(r.Min.X, r.Min.Y)
	i1 := i0 + r.Dx()*4
	for i := i0; i < i1; i += 4 {
		dst.Pix[i+0] = sr8
		dst.Pix[i+1] = sg8
		dst.Pix[i+2] = sb8
		dst.Pix[i+3] = sa8
	}
	firstRow := dst.Pix[i0:i1]
	for y := r.Min.Y + 1; y < r.Max.Y; y++ {
		i0 += dst.Stride
		i1 += dst.Stride
		copy(dst.Pix[i0:i1], firstRow)
	}
}

func drawCopyOver(dst *image.RGBA, r image.Rectangle, src *image.RGBA, sp image.Point) {
	dx, dy := r.Dx(), r.Dy()
	d0 := dst.PixOffset(r.Min.X, r.Min.Y)
	s0 := src.PixOffset(sp.X, sp.Y)
	var (
		ddelta, sdelta int
		i0, i1, idelta int
	)
	if r.Min.Y < sp.Y || r.Min.Y == sp.Y && r.Min.X <= sp.X {
		ddelta = dst.Stride
		sdelta = src.Stride
		i0, i1, idelta = 0, dx*4, +4
	} else {
		// If the source start point is higher than the destination start point, or equal height but to the left,
		// then we compose the rows in right-to-left, bottom-up order instead of left-to-right, top-down.
		d0 += (dy - 1) * dst.Stride
		s0 += (dy - 1) * src.Stride
		ddelta = -dst.Stride
		sdelta = -src.Stride
		i0, i1, idelta = (dx-1)*4, -4, -4
	}
	for ; dy > 0; dy-- {
		dpix := dst.Pix[d0:]
		spix := src.Pix[s0:]
		for i := i0; i != i1; i += idelta {
			s := spix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
			sr := uint32(s[0]) * 0x101
			sg := uint32(s[1]) * 0x101
			sb := uint32(s[2]) * 0x101
			sa := uint32(s[3]) * 0x101

			// The 0x101 is here for the same reason as in drawRGBA.
			a := (m - sa) * 0x101

			d := dpix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
			d[0] = uint8((uint32(d[0])*a/m + sr) >> 8)
			d[1] = uint8((uint32(d[1])*a/m + sg) >> 8)
			d[2] = uint8((uint32(d[2])*a/m + sb) >> 8)
			d[3] = uint8((uint32(d[3])*a/m + sa) >> 8)
		}
		d0 += ddelta
		s0 += sdelta
	}
}

// drawCopySrc copies bytes to dstPix from srcPix. These arguments roughly
// correspond to the Pix fields of the image package's concrete image.Image
// implementations, but are offset (dstPix is dst.Pix[dpOffset:] not dst.Pix).
func drawCopySrc(
	dstPix []byte, dstStride int, r image.Rectangle,
	srcPix []byte, srcStride int, sp image.Point,
	bytesPerRow int) {

	d0, s0, ddelta, sdelta, dy := 0, 0, dstStride, srcStride, r.Dy()
	if r.Min.Y > sp.Y {
		// If the source start point is higher than the destination start
		// point, then we compose the rows in bottom-up order instead of
		// top-down. Unlike the drawCopyOver function, we don't have to check
		// the x coordinates because the built-in copy function can handle
		// overlapping slices.
		d0 = (dy - 1) * dstStride
		s0 = (dy - 1) * srcStride
		ddelta = -dstStride
		sdelta = -srcStride
	}
	for ; dy > 0; dy-- {
		copy(dstPix[d0:d0+bytesPerRow], srcPix[s0:s0+bytesPerRow])
		d0 += ddelta
		s0 += sdelta
	}
}

func drawNRGBAOver(dst *image.RGBA, r image.Rectangle, src *image.NRGBA, sp image.Point) {
	i0 := (r.Min.X - dst.Rect.Min.X) * 4
	i1 := (r.Max.X - dst.Rect.Min.X) * 4
	si0 := (sp.X - src.Rect.Min.X) * 4
	yMax := r.Max.Y - dst.Rect.Min.Y

	y := r.Min.Y - dst.Rect.Min.Y
	sy := sp.Y - src.Rect.Min.Y
	for ; y != yMax; y, sy = y+1, sy+1 {
		dpix := dst.Pix[y*dst.Stride:]
		spix := src.Pix[sy*src.Stride:]

		for i, si := i0, si0; i < i1; i, si = i+4, si+4 {
			// Convert from non-premultiplied color to pre-multiplied color.
			s := spix[si : si+4 : si+4] // Small cap improves performance, see https://golang.org/issue/27857
			sa := uint32(s[3]) * 0x101
			sr := uint32(s[0]) * sa / 0xff
			sg := uint32(s[1]) * sa / 0xff
			sb := uint32(s[2]) * sa / 0xff

			d := dpix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
			dr := uint32(d[0])
			dg := uint32(d[1])
			db := uint32(d[2])
			da := uint32(d[3])

			// The 0x101 is here for the same reason as in drawRGBA.
			a := (m - sa) * 0x101

			d[0] = uint8((dr*a/m + sr) >> 8)
			d[1] = uint8((dg*a/m + sg) >> 8)
			d[2] = uint8((db*a/m + sb) >> 8)
			d[3] = uint8((da*a/m + sa) >> 8)
		}
	}
}

func drawNRGBASrc(dst *image.RGBA, r image.Rectangle, src *image.NRGBA, sp image.Point) {
	i0 := (r.Min.X - dst.Rect.Min.X) * 4
	i1 := (r.Max.X - dst.Rect.Min.X) * 4
	si0 := (sp.X - src.Rect.Min.X) * 4
	yMax := r.Max.Y - dst.Rect.Min.Y

	y := r.Min.Y - dst.Rect.Min.Y
	sy := sp.Y - src.Rect.Min.Y
	for ; y != yMax; y, sy = y+1, sy+1 {
		dpix := dst.Pix[y*dst.Stride:]
		spix := src.Pix[sy*src.Stride:]

		for i, si := i0, si0; i < i1; i, si = i+4, si+4 {
			// Convert from non-premultiplied color to pre-multiplied color.
			s := spix[si : si+4 : si+4] // Small cap improves performance, see https://golang.org/issue/27857
			sa := uint32(s[3]) * 0x101
			sr := uint32(s[0]) * sa / 0xff
			sg := uint32(s[1]) * sa / 0xff
			sb := uint32(s[2]) * sa / 0xff

			d := dpix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
			d[0] = uint8(sr >> 8)
			d[1] = uint8(sg >> 8)
			d[2] = uint8(sb >> 8)
			d[3] = uint8(sa >> 8)
		}
	}
}

func drawGray(dst *image.RGBA, r image.Rectangle, src *image.Gray, sp image.Point) {
	i0 := (r.Min.X - dst.Rect.Min.X) * 4
	i1 := (r.Max.X - dst.Rect.Min.X) * 4
	si0 := (sp.X - src.Rect.Min.X) * 1
	yMax := r.Max.Y - dst.Rect.Min.Y

	y := r.Min.Y - dst.Rect.Min.Y
	sy := sp.Y - src.Rect.Min.Y
	for ; y != yMax; y, sy = y+1, sy+1 {
		dpix := dst.Pix[y*dst.Stride:]
		spix := src.Pix[sy*src.Stride:]

		for i, si := i0, si0; i < i1; i, si = i+4, si+1 {
			p := spix[si]
			d := dpix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
			d[0] = p
			d[1] = p
			d[2] = p
			d[3] = 255
		}
	}
}

func drawCMYK(dst *image.RGBA, r image.Rectangle, src *image.CMYK, sp image.Point) {
	i0 := (r.Min.X - dst.Rect.Min.X) * 4
	i1 := (r.Max.X - dst.Rect.Min.X) * 4
	si0 := (sp.X - src.Rect.Min.X) * 4
	yMax := r.Max.Y - dst.Rect.Min.Y

	y := r.Min.Y - dst.Rect.Min.Y
	sy := sp.Y - src.Rect.Min.Y
	for ; y != yMax; y, sy = y+1, sy+1 {
		dpix := dst.Pix[y*dst.Stride:]
		spix := src.Pix[sy*src.Stride:]

		for i, si := i0, si0; i < i1; i, si = i+4, si+4 {
			s := spix[si : si+4 : si+4] // Small cap improves performance, see https://golang.org/issue/27857
			d := dpix[i : i+4 : i+4]
			d[0], d[1], d[2] = color.CMYKToRGB(s[0], s[1], s[2], s[3])
			d[3] = 255
		}
	}
}

func drawGlyphOver(dst *image.RGBA, r image.Rectangle, src *image.Uniform, mask *image.Alpha, mp image.Point) {
	i0 := dst.PixOffset(r.Min.X, r.Min.Y)
	i1 := i0 + r.Dx()*4
	mi0 := mask.PixOffset(mp.X, mp.Y)
	sr, sg, sb, sa := src.RGBA()
	for y, my := r.Min.Y, mp.Y; y != r.Max.Y; y, my = y+1, my+1 {
		for i, mi := i0, mi0; i < i1; i, mi = i+4, mi+1 {
			ma := uint32(mask.Pix[mi])
			if ma == 0 {
				continue
			}
			ma |= ma << 8

			// The 0x101 is here for the same reason as in drawRGBA.
			a := (m - (sa * ma / m)) * 0x101

			d := dst.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
			d[0] = uint8((uint32(d[0])*a + sr*ma) / m >> 8)
			d[1] = uint8((uint32(d[1])*a + sg*ma) / m >> 8)
			d[2] = uint8((uint32(d[2])*a + sb*ma) / m >> 8)
			d[3] = uint8((uint32(d[3])*a + sa*ma) / m >> 8)
		}
		i0 += dst.Stride
		i1 += dst.Stride
		mi0 += mask.Stride
	}
}

func drawGrayMaskOver(dst *image.RGBA, r image.Rectangle, src *image.Gray, sp image.Point, mask *image.Alpha, mp image.Point) {
	x0, x1, dx := r.Min.X, r.Max.X, 1
	y0, y1, dy := r.Min.Y, r.Max.Y, 1
	if r.Overlaps(r.Add(sp.Sub(r.Min))) {
		if sp.Y < r.Min.Y || sp.Y == r.Min.Y && sp.X < r.Min.X {
			x0, x1, dx = x1-1, x0-1, -1
			y0, y1, dy = y1-1, y0-1, -1
		}
	}

	sy := sp.Y + y0 - r.Min.Y
	my := mp.Y + y0 - r.Min.Y
	sx0 := sp.X + x0 - r.Min.X
	mx0 := mp.X + x0 - r.Min.X
	sx1 := sx0 + (x1 - x0)
	i0 := dst.PixOffset(x0, y0)
	di := dx * 4
	for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
		for i, sx, mx := i0, sx0, mx0; sx != sx1; i, sx, mx = i+di, sx+dx, mx+dx {
			mi := mask.PixOffset(mx, my)
			ma := uint32(mask.Pix[mi])
			ma |= ma << 8
			si := src.PixOffset(sx, sy)
			sy := uint32(src.Pix[si])
			sy |= sy << 8
			sa := uint32(0xffff)

			d := dst.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
			dr := uint32(d[0])
			dg := uint32(d[1])
			db := uint32(d[2])
			da := uint32(d[3])

			// dr, dg, db and da are all 8-bit color at the moment, ranging in [0,255].
			// We work in 16-bit color, and so would normally do:
			// dr |= dr << 8
			// and similarly for dg, db and da, but instead we multiply a
			// (which is a 16-bit color, ranging in [0,65535]) by 0x101.
			// This yields the same result, but is fewer arithmetic operations.
			a := (m - (sa * ma / m)) * 0x101

			d[0] = uint8((dr*a + sy*ma) / m >> 8)
			d[1] = uint8((dg*a + sy*ma) / m >> 8)
			d[2] = uint8((db*a + sy*ma) / m >> 8)
			d[3] = uint8((da*a + sa*ma) / m >> 8)
		}
		i0 += dy * dst.Stride
	}
}

func drawRGBAMaskOver(dst *image.RGBA, r image.Rectangle, src *image.RGBA, sp image.Point, mask *image.Alpha, mp image.Point) {
	x0, x1, dx := r.Min.X, r.Max.X, 1
	y0, y1, dy := r.Min.Y, r.Max.Y, 1
	if dst == src && r.Overlaps(r.Add(sp.Sub(r.Min))) {
		if sp.Y < r.Min.Y || sp.Y == r.Min.Y && sp.X < r.Min.X {
			x0, x1, dx = x1-1, x0-1, -1
			y0, y1, dy = y1-1, y0-1, -1
		}
	}

	sy := sp.Y + y0 - r.Min.Y
	my := mp.Y + y0 - r.Min.Y
	sx0 := sp.X + x0 - r.Min.X
	mx0 := mp.X + x0 - r.Min.X
	sx1 := sx0 + (x1 - x0)
	i0 := dst.PixOffset(x0, y0)
	di := dx * 4
	for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
		for i, sx, mx := i0, sx0, mx0; sx != sx1; i, sx, mx = i+di, sx+dx, mx+dx {
			mi := mask.PixOffset(mx, my)
			ma := uint32(mask.Pix[mi])
			ma |= ma << 8
			si := src.PixOffset(sx, sy)
			sr := uint32(src.Pix[si+0])
			sg := uint32(src.Pix[si+1])
			sb := uint32(src.Pix[si+2])
			sa := uint32(src.Pix[si+3])
			sr |= sr << 8
			sg |= sg << 8
			sb |= sb << 8
			sa |= sa << 8
			d := dst.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
			dr := uint32(d[0])
			dg := uint32(d[1])
			db := uint32(d[2])
			da := uint32(d[3])

			// dr, dg, db and da are all 8-bit color at the moment, ranging in [0,255].
			// We work in 16-bit color, and so would normally do:
			// dr |= dr << 8
			// and similarly for dg, db and da, but instead we multiply a
			// (which is a 16-bit color, ranging in [0,65535]) by 0x101.
			// This yields the same result, but is fewer arithmetic operations.
			a := (m - (sa * ma / m)) * 0x101

			d[0] = uint8((dr*a + sr*ma) / m >> 8)
			d[1] = uint8((dg*a + sg*ma) / m >> 8)
			d[2] = uint8((db*a + sb*ma) / m >> 8)
			d[3] = uint8((da*a + sa*ma) / m >> 8)
		}
		i0 += dy * dst.Stride
	}
}

func drawRGBA64ImageMaskOver(dst *image.RGBA, r image.Rectangle, src image.RGBA64Image, sp image.Point, mask *image.Alpha, mp image.Point) {
	x0, x1, dx := r.Min.X, r.Max.X, 1
	y0, y1, dy := r.Min.Y, r.Max.Y, 1
	if image.Image(dst) == src && r.Overlaps(r.Add(sp.Sub(r.Min))) {
		if sp.Y < r.Min.Y || sp.Y == r.Min.Y && sp.X < r.Min.X {
			x0, x1, dx = x1-1, x0-1, -1
			y0, y1, dy = y1-1, y0-1, -1
		}
	}

	sy := sp.Y + y0 - r.Min.Y
	my := mp.Y + y0 - r.Min.Y
	sx0 := sp.X + x0 - r.Min.X
	mx0 := mp.X + x0 - r.Min.X
	sx1 := sx0 + (x1 - x0)
	i0 := dst.PixOffset(x0, y0)
	di := dx * 4
	for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
		for i, sx, mx := i0, sx0, mx0; sx != sx1; i, sx, mx = i+di, sx+dx, mx+dx {
			mi := mask.PixOffset(mx, my)
			ma := uint32(mask.Pix[mi])
			ma |= ma << 8
			srgba := src.RGBA64At(sx, sy)
			d := dst.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
			dr := uint32(d[0])
			dg := uint32(d[1])
			db := uint32(d[2])
			da := uint32(d[3])

			// dr, dg, db and da are all 8-bit color at the moment, ranging in [0,255].
			// We work in 16-bit color, and so would normally do:
			// dr |= dr << 8
			// and similarly for dg, db and da, but instead we multiply a
			// (which is a 16-bit color, ranging in [0,65535]) by 0x101.
			// This yields the same result, but is fewer arithmetic operations.
			a := (m - (uint32(srgba.A) * ma / m)) * 0x101

			d[0] = uint8((dr*a + uint32(srgba.R)*ma) / m >> 8)
			d[1] = uint8((dg*a + uint32(srgba.G)*ma) / m >> 8)
			d[2] = uint8((db*a + uint32(srgba.B)*ma) / m >> 8)
			d[3] = uint8((da*a + uint32(srgba.A)*ma) / m >> 8)
		}
		i0 += dy * dst.Stride
	}
}

func drawRGBA(dst *image.RGBA, r image.Rectangle, src image.Image, sp image.Point, mask image.Image, mp image.Point, op Op) {
	x0, x1, dx := r.Min.X, r.Max.X, 1
	y0, y1, dy := r.Min.Y, r.Max.Y, 1
	if image.Image(dst) == src && r.Overlaps(r.Add(sp.Sub(r.Min))) {
		if sp.Y < r.Min.Y || sp.Y == r.Min.Y && sp.X < r.Min.X {
			x0, x1, dx = x1-1, x0-1, -1
			y0, y1, dy = y1-1, y0-1, -1
		}
	}

	sy := sp.Y + y0 - r.Min.Y
	my := mp.Y + y0 - r.Min.Y
	sx0 := sp.X + x0 - r.Min.X
	mx0 := mp.X + x0 - r.Min.X
	sx1 := sx0 + (x1 - x0)
	i0 := dst.PixOffset(x0, y0)
	di := dx * 4

	// Try the image.RGBA64Image interface, part of the standard library since
	// Go 1.17.
	//
	// This optimization is similar to how FALLBACK1.17 optimizes FALLBACK1.0
	// in DrawMask, except here the concrete type of dst is known to be
	// *image.RGBA.
	if src0, _ := src.(image.RGBA64Image); src0 != nil {
		if mask == nil {
			if op == Over {
				for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
					for i, sx, mx := i0, sx0, mx0; sx != sx1; i, sx, mx = i+di, sx+dx, mx+dx {
						srgba := src0.RGBA64At(sx, sy)
						d := dst.Pix[i : i+4 : i+4]
						dr := uint32(d[0])
						dg := uint32(d[1])
						db := uint32(d[2])
						da := uint32(d[3])
						a := (m - uint32(srgba.A)) * 0x101
						d[0] = uint8((dr*a/m + uint32(srgba.R)) >> 8)
						d[1] = uint8((dg*a/m + uint32(srgba.G)) >> 8)
						d[2] = uint8((db*a/m + uint32(srgba.B)) >> 8)
						d[3] = uint8((da*a/m + uint32(srgba.A)) >> 8)
					}
					i0 += dy * dst.Stride
				}
			} else {
				for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
					for i, sx, mx := i0, sx0, mx0; sx != sx1; i, sx, mx = i+di, sx+dx, mx+dx {
						srgba := src0.RGBA64At(sx, sy)
						d := dst.Pix[i : i+4 : i+4]
						d[0] = uint8(srgba.R >> 8)
						d[1] = uint8(srgba.G >> 8)
						d[2] = uint8(srgba.B >> 8)
						d[3] = uint8(srgba.A >> 8)
					}
					i0 += dy * dst.Stride
				}
			}
			return

		} else if mask0, _ := mask.(image.RGBA64Image); mask0 != nil {
			if op == Over {
				for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
					for i, sx, mx := i0, sx0, mx0; sx != sx1; i, sx, mx = i+di, sx+dx, mx+dx {
						ma := uint32(mask0.RGBA64At(mx, my).A)
						srgba := src0.RGBA64At(sx, sy)
						d := dst.Pix[i : i+4 : i+4]
						dr := uint32(d[0])
						dg := uint32(d[1])
						db := uint32(d[2])
						da := uint32(d[3])
						a := (m - (uint32(srgba.A) * ma / m)) * 0x101
						d[0] = uint8((dr*a + uint32(srgba.R)*ma) / m >> 8)
						d[1] = uint8((dg*a + uint32(srgba.G)*ma) / m >> 8)
						d[2] = uint8((db*a + uint32(srgba.B)*ma) / m >> 8)
						d[3] = uint8((da*a + uint32(srgba.A)*ma) / m >> 8)
					}
					i0 += dy * dst.Stride
				}
			} else {
				for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
					for i, sx, mx := i0, sx0, mx0; sx != sx1; i, sx, mx = i+di, sx+dx, mx+dx {
						ma := uint32(mask0.RGBA64At(mx, my).A)
						srgba := src0.RGBA64At(sx, sy)
						d := dst.Pix[i : i+4 : i+4]
						d[0] = uint8(uint32(srgba.R) * ma / m >> 8)
						d[1] = uint8(uint32(srgba.G) * ma / m >> 8)
						d[2] = uint8(uint32(srgba.B) * ma / m >> 8)
						d[3] = uint8(uint32(srgba.A) * ma / m >> 8)
					}
					i0 += dy * dst.Stride
				}
			}
			return
		}
	}

	// Use the image.Image interface, part of the standard library since Go
	// 1.0.
	//
	// This is similar to FALLBACK1.0 in DrawMask, except here the concrete
	// type of dst is known to be *image.RGBA.
	for y := y0; y != y1; y, sy, my = y+dy, sy+dy, my+dy {
		for i, sx, mx := i0, sx0, mx0; sx != sx1; i, sx, mx = i+di, sx+dx, mx+dx {
			ma := uint32(m)
			if mask != nil {
				_, _, _, ma = mask.At(mx, my).RGBA()
			}
			sr, sg, sb, sa := src.At(sx, sy).RGBA()
			d := dst.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
			if op == Over {
				dr := uint32(d[0])
				dg := uint32(d[1])
				db := uint32(d[2])
				da := uint32(d[3])

				// dr, dg, db and da are all 8-bit color at the moment, ranging in [0,255].
				// We work in 16-bit color, and so would normally do:
				// dr |= dr << 8
				// and similarly for dg, db and da, but instead we multiply a
				// (which is a 16-bit color, ranging in [0,65535]) by 0x101.
				// This yields the same result, but is fewer arithmetic operations.
				a := (m - (sa * ma / m)) * 0x101

				d[0] = uint8((dr*a + sr*ma) / m >> 8)
				d[1] = uint8((dg*a + sg*ma) / m >> 8)
				d[2] = uint8((db*a + sb*ma) / m >> 8)
				d[3] = uint8((da*a + sa*ma) / m >> 8)

			} else {
				d[0] = uint8(sr * ma / m >> 8)
				d[1] = uint8(sg * ma / m >> 8)
				d[2] = uint8(sb * ma / m >> 8)
				d[3] = uint8(sa * ma / m >> 8)
			}
		}
		i0 += dy * dst.Stride
	}
}

// clamp clamps i to the interval [0, 0xffff].
func clamp(i int32) int32 {
	if i < 0 {
		return 0
	}
	if i > 0xffff {
		return 0xffff
	}
	return i
}

// sqDiff returns the squared-difference of x and y, shifted by 2 so that
// adding four of those won't overflow a uint32.
//
// x and y are both assumed to be in the range [0, 0xffff].
func sqDiff(x, y int32) uint32 {
	// This is an optimized code relying on the overflow/wrap around
	// properties of unsigned integers operations guaranteed by the language
	// spec. See sqDiff from the image/color package for more details.
	d := uint32(x - y)
	return (d * d) >> 2
}

func drawPaletted(dst Image, r image.Rectangle, src image.Image, sp image.Point, floydSteinberg bool) {
	// TODO(nigeltao): handle the case where the dst and src overlap.
	// Does it even make sense to try and do Floyd-Steinberg whilst
	// walking the image backward (right-to-left bottom-to-top)?

	// If dst is an *image.Paletted, we have a fast path for dst.Set and
	// dst.At. The dst.Set equivalent is a batch version of the algorithm
	// used by color.Palette's Index method in image/color/color.go, plus
	// optional Floyd-Steinberg error diffusion.
	palette, pix, stride := [][4]int32(nil), []byte(nil), 0
	if p, ok := dst.(*image.Paletted); ok {
		palette = make([][4]int32, len(p.Palette))
		for i, col := range p.Palette {
			r, g, b, a := col.RGBA()
			palette[i][0] = int32(r)
			palette[i][1] = int32(g)
			palette[i][2] = int32(b)
			palette[i][3] = int32(a)
		}
		pix, stride = p.Pix[p.PixOffset(r.Min.X, r.Min.Y):], p.Stride
	}

	// quantErrorCurr and quantErrorNext are the Floyd-Steinberg quantization
	// errors that have been propagated to the pixels in the current and next
	// rows. The +2 simplifies calculation near the edges.
	var quantErrorCurr, quantErrorNext [][4]int32
	if floydSteinberg {
		quantErrorCurr = make([][4]int32, r.Dx()+2)
		quantErrorNext = make([][4]int32, r.Dx()+2)
	}
	pxRGBA := func(x, y int) (r, g, b, a uint32) { return src.At(x, y).RGBA() }
	// Fast paths for special cases to avoid excessive use of the color.Color
	// interface which escapes to the heap but need to be discovered for
	// each pixel on r. See also https://golang.org/issues/15759.
	switch src0 := src.(type) {
	case *image.RGBA:
		pxRGBA = func(x, y int) (r, g, b, a uint32) { return src0.RGBAAt(x, y).RGBA() }
	case *image.NRGBA:
		pxRGBA = func(x, y int) (r, g, b, a uint32) { return src0.NRGBAAt(x, y).RGBA() }
	case *image.YCbCr:
		pxRGBA = func(x, y int) (r, g, b, a uint32) { return src0.YCbCrAt(x, y).RGBA() }
	}

	// Loop over each source pixel.
	out := color.RGBA64{A: 0xffff}
	for y := 0; y != r.Dy(); y++ {
		for x := 0; x != r.Dx(); x++ {
			// er, eg and eb are the pixel's R,G,B values plus the
			// optional Floyd-Steinberg error.
			sr, sg, sb, sa := pxRGBA(sp.X+x, sp.Y+y)
			er, eg, eb, ea := int32(sr), int32(sg), int32(sb), int32(sa)
			if floydSteinberg {
				er = clamp(er + quantErrorCurr[x+1][0]/16)
				eg = clamp(eg + quantErrorCurr[x+1][1]/16)
				eb = clamp(eb + quantErrorCurr[x+1][2]/16)
				ea = clamp(ea + quantErrorCurr[x+1][3]/16)
			}

			if palette != nil {
				// Find the closest palette color in Euclidean R,G,B,A space:
				// the one that minimizes sum-squared-difference.
				// TODO(nigeltao): consider smarter algorithms.
				bestIndex, bestSum := 0, uint32(1<<32-1)
				for index, p := range palette {
					sum := sqDiff(er, p[0]) + sqDiff(eg, p[1]) + sqDiff(eb, p[2]) + sqDiff(ea, p[3])
					if sum < bestSum {
						bestIndex, bestSum = index, sum
						if sum == 0 {
							break
						}
					}
				}
				pix[y*stride+x] = byte(bestIndex)

				if !floydSteinberg {
					continue
				}
				er -= palette[bestIndex][0]
				eg -= palette[bestIndex][1]
				eb -= palette[bestIndex][2]
				ea -= palette[bestIndex][3]

			} else {
				out.R = uint16(er)
				out.G = uint16(eg)
				out.B = uint16(eb)
				out.A = uint16(ea)
				// The third argument is &out instead of out (and out is
				// declared outside of the inner loop) to avoid the implicit
				// conversion to color.Color here allocating memory in the
				// inner loop if sizeof(color.RGBA64) > sizeof(uintptr).
				dst.Set(r.Min.X+x, r.Min.Y+y, &out)

				if !floydSteinberg {
					continue
				}
				sr, sg, sb, sa = dst.At(r.Min.X+x, r.Min.Y+y).RGBA()
				er -= int32(sr)
				eg -= int32(sg)
				eb -= int32(sb)
				ea -= int32(sa)
			}

			// Propagate the Floyd-Steinberg quantization error.
			quantErrorNext[x+0][0] += er * 3
			quantErrorNext[x+0][1] += eg * 3
			quantErrorNext[x+0][2] += eb * 3
			quantErrorNext[x+0][3] += ea * 3
			quantErrorNext[x+1][0] += er * 5
			quantErrorNext[x+1][1] += eg * 5
			quantErrorNext[x+1][2] += eb * 5
			quantErrorNext[x+1][3] += ea * 5
			quantErrorNext[x+2][0] += er * 1
			quantErrorNext[x+2][1] += eg * 1
			quantErrorNext[x+2][2] += eb * 1
			quantErrorNext[x+2][3] += ea * 1
			quantErrorCurr[x+2][0] += er * 7
			quantErrorCurr[x+2][1] += eg * 7
			quantErrorCurr[x+2][2] += eb * 7
			quantErrorCurr[x+2][3] += ea * 7
		}

		// Recycle the quantization error buffers.
		if floydSteinberg {
			quantErrorCurr, quantErrorNext = quantErrorNext, quantErrorCurr
			clear(quantErrorNext)
		}
	}
}

```

// === FILE: references/go/src/image/format.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package image

import (
	"bufio"
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

// ErrFormat indicates that decoding encountered an unknown format.
var ErrFormat = errors.New("image: unknown format")

// A format holds an image format's name, magic header and how to decode it.
type format struct {
	name, magic  string
	decode       func(io.Reader) (Image, error)
	decodeConfig func(io.Reader) (Config, error)
}

// Formats is the list of registered formats.
var (
	formatsMu     sync.Mutex
	atomicFormats atomic.Value
)

// RegisterFormat registers an image format for use by [Decode].
// Name is the name of the format, like "jpeg" or "png".
// Magic is the magic prefix that identifies the format's encoding. The magic
// string can contain "?" wildcards that each match any one byte.
// [Decode] is the function that decodes the encoded image.
// [DecodeConfig] is the function that decodes just its configuration.
func RegisterFormat(name, magic string, decode func(io.Reader) (Image, error), decodeConfig func(io.Reader) (Config, error)) {
	formatsMu.Lock()
	formats, _ := atomicFormats.Load().([]format)
	atomicFormats.Store(append(formats, format{name, magic, decode, decodeConfig}))
	formatsMu.Unlock()
}

// A reader is an io.Reader that can also peek ahead.
type reader interface {
	io.Reader
	Peek(int) ([]byte, error)
}

// asReader converts an io.Reader to a reader.
func asReader(r io.Reader) reader {
	if rr, ok := r.(reader); ok {
		return rr
	}
	return bufio.NewReader(r)
}

// match reports whether magic matches b. Magic may contain "?" wildcards.
func match(magic string, b []byte) bool {
	if len(magic) != len(b) {
		return false
	}
	for i, c := range b {
		if magic[i] != c && magic[i] != '?' {
			return false
		}
	}
	return true
}

// sniff determines the format of r's data.
func sniff(r reader) format {
	formats, _ := atomicFormats.Load().([]format)
	for _, f := range formats {
		b, err := r.Peek(len(f.magic))
		if err == nil && match(f.magic, b) {
			return f
		}
	}
	return format{}
}

// Decode decodes an image that has been encoded in a registered format.
// The string returned is the format name used during format registration.
// Format registration is typically done by an init function in the codec-
// specific package.
//
// Decoding may allocate memory proportional to the width and height in the
// image header before all pixel data is consumed or validated. When
// decoding untrusted input, call [DecodeConfig] first to inspect dimensions
// and reject images that would exceed resource limits; see the "Security
// Considerations" section in the [image] package documentation.
func Decode(r io.Reader) (Image, string, error) {
	rr := asReader(r)
	f := sniff(rr)
	if f.decode == nil {
		return nil, "", ErrFormat
	}
	m, err := f.decode(rr)
	return m, f.name, err
}

// DecodeConfig decodes the color model and dimensions of an image that has
// been encoded in a registered format. The string returned is the format name
// used during format registration. Format registration is typically done by
// an init function in the codec-specific package.
//
// DecodeConfig reads only format headers and does not allocate a full-size
// pixel buffer, so it can be used to check dimensions before calling [Decode].
func DecodeConfig(r io.Reader) (Config, string, error) {
	rr := asReader(r)
	f := sniff(rr)
	if f.decodeConfig == nil {
		return Config{}, "", ErrFormat
	}
	c, err := f.decodeConfig(rr)
	return c, f.name, err
}

```

// === FILE: references/go/src/image/geom.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package image

import (
	"image/color"
	"math/bits"
	"strconv"
)

// A Point is an X, Y coordinate pair. The axes increase right and down.
type Point struct {
	X, Y int
}

// String returns a string representation of p like "(3,4)".
func (p Point) String() string {
	return "(" + strconv.Itoa(p.X) + "," + strconv.Itoa(p.Y) + ")"
}

// Add returns the vector p+q.
func (p Point) Add(q Point) Point {
	return Point{p.X + q.X, p.Y + q.Y}
}

// Sub returns the vector p-q.
func (p Point) Sub(q Point) Point {
	return Point{p.X - q.X, p.Y - q.Y}
}

// Mul returns the vector p*k.
func (p Point) Mul(k int) Point {
	return Point{p.X * k, p.Y * k}
}

// Div returns the vector p/k.
func (p Point) Div(k int) Point {
	return Point{p.X / k, p.Y / k}
}

// In reports whether p is in r.
func (p Point) In(r Rectangle) bool {
	return r.Min.X <= p.X && p.X < r.Max.X &&
		r.Min.Y <= p.Y && p.Y < r.Max.Y
}

// Mod returns the point q in r such that p.X-q.X is a multiple of r's width
// and p.Y-q.Y is a multiple of r's height.
func (p Point) Mod(r Rectangle) Point {
	w, h := r.Dx(), r.Dy()
	p = p.Sub(r.Min)
	p.X = p.X % w
	if p.X < 0 {
		p.X += w
	}
	p.Y = p.Y % h
	if p.Y < 0 {
		p.Y += h
	}
	return p.Add(r.Min)
}

// Eq reports whether p and q are equal.
func (p Point) Eq(q Point) bool {
	return p == q
}

// ZP is the zero [Point].
//
// Deprecated: Use a literal [image.Point] instead.
var ZP Point

// Pt is shorthand for [Point]{X, Y}.
func Pt(X, Y int) Point {
	return Point{X, Y}
}

// A Rectangle contains the points with Min.X <= X < Max.X, Min.Y <= Y < Max.Y.
// It is well-formed if Min.X <= Max.X and likewise for Y. Points are always
// well-formed. A rectangle's methods always return well-formed outputs for
// well-formed inputs.
//
// A Rectangle is also an [Image] whose bounds are the rectangle itself. At
// returns color.Opaque for points in the rectangle and color.Transparent
// otherwise.
type Rectangle struct {
	Min, Max Point
}

// String returns a string representation of r like "(3,4)-(6,5)".
func (r Rectangle) String() string {
	return r.Min.String() + "-" + r.Max.String()
}

// Dx returns r's width.
func (r Rectangle) Dx() int {
	return r.Max.X - r.Min.X
}

// Dy returns r's height.
func (r Rectangle) Dy() int {
	return r.Max.Y - r.Min.Y
}

// Size returns r's width and height.
func (r Rectangle) Size() Point {
	return Point{
		r.Max.X - r.Min.X,
		r.Max.Y - r.Min.Y,
	}
}

// Add returns the rectangle r translated by p.
func (r Rectangle) Add(p Point) Rectangle {
	return Rectangle{
		Point{r.Min.X + p.X, r.Min.Y + p.Y},
		Point{r.Max.X + p.X, r.Max.Y + p.Y},
	}
}

// Sub returns the rectangle r translated by -p.
func (r Rectangle) Sub(p Point) Rectangle {
	return Rectangle{
		Point{r.Min.X - p.X, r.Min.Y - p.Y},
		Point{r.Max.X - p.X, r.Max.Y - p.Y},
	}
}

// Inset returns the rectangle r inset by n, which may be negative. If either
// of r's dimensions is less than 2*n then an empty rectangle near the center
// of r will be returned.
func (r Rectangle) Inset(n int) Rectangle {
	if r.Dx() < 2*n {
		r.Min.X = (r.Min.X + r.Max.X) / 2
		r.Max.X = r.Min.X
	} else {
		r.Min.X += n
		r.Max.X -= n
	}
	if r.Dy() < 2*n {
		r.Min.Y = (r.Min.Y + r.Max.Y) / 2
		r.Max.Y = r.Min.Y
	} else {
		r.Min.Y += n
		r.Max.Y -= n
	}
	return r
}

// Intersect returns the largest rectangle contained by both r and s. If the
// two rectangles do not overlap then the zero rectangle will be returned.
func (r Rectangle) Intersect(s Rectangle) Rectangle {
	if r.Min.X < s.Min.X {
		r.Min.X = s.Min.X
	}
	if r.Min.Y < s.Min.Y {
		r.Min.Y = s.Min.Y
	}
	if r.Max.X > s.Max.X {
		r.Max.X = s.Max.X
	}
	if r.Max.Y > s.Max.Y {
		r.Max.Y = s.Max.Y
	}
	// Letting r0 and s0 be the values of r and s at the time that the method
	// is called, this next line is equivalent to:
	//
	// if max(r0.Min.X, s0.Min.X) >= min(r0.Max.X, s0.Max.X) || likewiseForY { etc }
	if r.Empty() {
		return Rectangle{}
	}
	return r
}

// Union returns the smallest rectangle that contains both r and s.
func (r Rectangle) Union(s Rectangle) Rectangle {
	if r.Empty() {
		return s
	}
	if s.Empty() {
		return r
	}
	if r.Min.X > s.Min.X {
		r.Min.X = s.Min.X
	}
	if r.Min.Y > s.Min.Y {
		r.Min.Y = s.Min.Y
	}
	if r.Max.X < s.Max.X {
		r.Max.X = s.Max.X
	}
	if r.Max.Y < s.Max.Y {
		r.Max.Y = s.Max.Y
	}
	return r
}

// Empty reports whether the rectangle contains no points.
func (r Rectangle) Empty() bool {
	return r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y
}

// Eq reports whether r and s contain the same set of points. All empty
// rectangles are considered equal.
func (r Rectangle) Eq(s Rectangle) bool {
	return r == s || r.Empty() && s.Empty()
}

// Overlaps reports whether r and s have a non-empty intersection.
func (r Rectangle) Overlaps(s Rectangle) bool {
	return !r.Empty() && !s.Empty() &&
		r.Min.X < s.Max.X && s.Min.X < r.Max.X &&
		r.Min.Y < s.Max.Y && s.Min.Y < r.Max.Y
}

// In reports whether every point in r is in s.
func (r Rectangle) In(s Rectangle) bool {
	if r.Empty() {
		return true
	}
	// Note that r.Max is an exclusive bound for r, so that r.In(s)
	// does not require that r.Max.In(s).
	return s.Min.X <= r.Min.X && r.Max.X <= s.Max.X &&
		s.Min.Y <= r.Min.Y && r.Max.Y <= s.Max.Y
}

// Canon returns the canonical version of r. The returned rectangle has minimum
// and maximum coordinates swapped if necessary so that it is well-formed.
func (r Rectangle) Canon() Rectangle {
	if r.Max.X < r.Min.X {
		r.Min.X, r.Max.X = r.Max.X, r.Min.X
	}
	if r.Max.Y < r.Min.Y {
		r.Min.Y, r.Max.Y = r.Max.Y, r.Min.Y
	}
	return r
}

// At implements the [Image] interface.
func (r Rectangle) At(x, y int) color.Color {
	if (Point{x, y}).In(r) {
		return color.Opaque
	}
	return color.Transparent
}

// RGBA64At implements the [RGBA64Image] interface.
func (r Rectangle) RGBA64At(x, y int) color.RGBA64 {
	if (Point{x, y}).In(r) {
		return color.RGBA64{0xffff, 0xffff, 0xffff, 0xffff}
	}
	return color.RGBA64{}
}

// Bounds implements the [Image] interface.
func (r Rectangle) Bounds() Rectangle {
	return r
}

// ColorModel implements the [Image] interface.
func (r Rectangle) ColorModel() color.Model {
	return color.Alpha16Model
}

// ZR is the zero [Rectangle].
//
// Deprecated: Use a literal [image.Rectangle] instead.
var ZR Rectangle

// Rect is shorthand for [Rectangle]{Pt(x0, y0), [Pt](x1, y1)}. The returned
// rectangle has minimum and maximum coordinates swapped if necessary so that
// it is well-formed.
func Rect(x0, y0, x1, y1 int) Rectangle {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	return Rectangle{Point{x0, y0}, Point{x1, y1}}
}

// mul3NonNeg returns (x * y * z), unless at least one argument is negative or
// if the computation overflows the int type, in which case it returns -1.
func mul3NonNeg(x int, y int, z int) int {
	if (x < 0) || (y < 0) || (z < 0) {
		return -1
	}
	hi, lo := bits.Mul64(uint64(x), uint64(y))
	if hi != 0 {
		return -1
	}
	hi, lo = bits.Mul64(lo, uint64(z))
	if hi != 0 {
		return -1
	}
	a := int(lo)
	if (a < 0) || (uint64(a) != lo) {
		return -1
	}
	return a
}

// add2NonNeg returns (x + y), unless at least one argument is negative or if
// the computation overflows the int type, in which case it returns -1.
func add2NonNeg(x int, y int) int {
	if (x < 0) || (y < 0) {
		return -1
	}
	a := x + y
	if a < 0 {
		return -1
	}
	return a
}

```

// === FILE: references/go/src/image/gif/reader.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gif implements a GIF image decoder and encoder.
//
// The GIF specification is at https://www.w3.org/Graphics/GIF/spec-gif89a.txt.
//
// When decoding untrusted input, read dimensions with [DecodeConfig] before
// calling [Decode] or [DecodeAll]; see those functions and the "Security
// Considerations" section in the [image] package documentation.
package gif

import (
	"bufio"
	"compress/lzw"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
)

var (
	errNotEnough = errors.New("gif: not enough image data")
	errTooMuch   = errors.New("gif: too much image data")
	errBadPixel  = errors.New("gif: invalid pixel value")
)

// If the io.Reader does not also have ReadByte, then decode will introduce its own buffering.
type reader interface {
	io.Reader
	io.ByteReader
}

// Masks etc.
const (
	// Fields.
	fColorTable         = 1 << 7
	fInterlace          = 1 << 6
	fColorTableBitsMask = 7

	// Graphic control flags.
	gcTransparentColorSet = 1 << 0
	gcDisposalMethodMask  = 7 << 2
)

// Disposal Methods.
const (
	DisposalNone       = 0x01
	DisposalBackground = 0x02
	DisposalPrevious   = 0x03
)

// Section indicators.
const (
	sExtension       = 0x21
	sImageDescriptor = 0x2C
	sTrailer         = 0x3B
)

// Extensions.
const (
	eText           = 0x01 // Plain Text
	eGraphicControl = 0xF9 // Graphic Control
	eComment        = 0xFE // Comment
	eApplication    = 0xFF // Application
)

func readFull(r io.Reader, b []byte) error {
	_, err := io.ReadFull(r, b)
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}

func readByte(r io.ByteReader) (byte, error) {
	b, err := r.ReadByte()
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return b, err
}

// decoder is the type used to decode a GIF file.
type decoder struct {
	r reader

	// From header.
	vers            string
	width           int
	height          int
	loopCount       int
	delayTime       int
	backgroundIndex byte
	disposalMethod  byte

	// From image descriptor.
	imageFields byte

	// From graphics control.
	transparentIndex    byte
	hasTransparentIndex bool

	// Computed.
	globalColorTable color.Palette

	// Used when decoding.
	delay    []int
	disposal []byte
	image    []*image.Paletted
	tmp      [1024]byte // must be at least 768 so we can read color table
}

// blockReader parses the block structure of GIF image data, which comprises
// (n, (n bytes)) blocks, with 1 <= n <= 255. It is the reader given to the
// LZW decoder, which is thus immune to the blocking. After the LZW decoder
// completes, there will be a 0-byte block remaining (0, ()), which is
// consumed when checking that the blockReader is exhausted.
//
// To avoid the allocation of a bufio.Reader for the lzw Reader, blockReader
// implements io.ByteReader and buffers blocks into the decoder's "tmp" buffer.
type blockReader struct {
	d    *decoder
	i, j uint8 // d.tmp[i:j] contains the buffered bytes
	err  error
}

func (b *blockReader) fill() {
	if b.err != nil {
		return
	}
	b.j, b.err = readByte(b.d.r)
	if b.j == 0 && b.err == nil {
		b.err = io.EOF
	}
	if b.err != nil {
		return
	}

	b.i = 0
	b.err = readFull(b.d.r, b.d.tmp[:b.j])
	if b.err != nil {
		b.j = 0
	}
}

func (b *blockReader) ReadByte() (byte, error) {
	if b.i == b.j {
		b.fill()
		if b.err != nil {
			return 0, b.err
		}
	}

	c := b.d.tmp[b.i]
	b.i++
	return c, nil
}

// blockReader must implement io.Reader, but its Read shouldn't ever actually
// be called in practice. The compress/lzw package will only call [blockReader.ReadByte].
func (b *blockReader) Read(p []byte) (int, error) {
	if len(p) == 0 || b.err != nil {
		return 0, b.err
	}
	if b.i == b.j {
		b.fill()
		if b.err != nil {
			return 0, b.err
		}
	}

	n := copy(p, b.d.tmp[b.i:b.j])
	b.i += uint8(n)
	return n, nil
}

// close primarily detects whether or not a block terminator was encountered
// after reading a sequence of data sub-blocks. It allows at most one trailing
// sub-block worth of data. I.e., if some number of bytes exist in one sub-block
// following the end of LZW data, the very next sub-block must be the block
// terminator. If the very end of LZW data happened to fill one sub-block, at
// most one more sub-block of length 1 may exist before the block-terminator.
// These accommodations allow us to support GIFs created by less strict encoders.
// See https://golang.org/issue/16146.
func (b *blockReader) close() error {
	if b.err == io.EOF {
		// A clean block-sequence terminator was encountered while reading.
		return nil
	} else if b.err != nil {
		// Some other error was encountered while reading.
		return b.err
	}

	if b.i == b.j {
		// We reached the end of a sub block reading LZW data. We'll allow at
		// most one more sub block of data with a length of 1 byte.
		b.fill()
		if b.err == io.EOF {
			return nil
		} else if b.err != nil {
			return b.err
		} else if b.j > 1 {
			return errTooMuch
		}
	}

	// Part of a sub-block remains buffered. We expect that the next attempt to
	// buffer a sub-block will reach the block terminator.
	b.fill()
	if b.err == io.EOF {
		return nil
	} else if b.err != nil {
		return b.err
	}

	return errTooMuch
}

// decode reads a GIF image from r and stores the result in d.
func (d *decoder) decode(r io.Reader, configOnly, keepAllFrames bool) error {
	// Add buffering if r does not provide ReadByte.
	if rr, ok := r.(reader); ok {
		d.r = rr
	} else {
		d.r = bufio.NewReader(r)
	}

	d.loopCount = -1

	err := d.readHeaderAndScreenDescriptor()
	if err != nil {
		return err
	}
	if configOnly {
		return nil
	}

	for {
		c, err := readByte(d.r)
		if err != nil {
			return fmt.Errorf("gif: reading frames: %v", err)
		}
		switch c {
		case sExtension:
			if err = d.readExtension(); err != nil {
				return err
			}

		case sImageDescriptor:
			if err = d.readImageDescriptor(keepAllFrames); err != nil {
				return err
			}

			if !keepAllFrames && len(d.image) == 1 {
				return nil
			}

		case sTrailer:
			if len(d.image) == 0 {
				return fmt.Errorf("gif: missing image data")
			}
			return nil

		default:
			return fmt.Errorf("gif: unknown block type: 0x%.2x", c)
		}
	}
}

func (d *decoder) readHeaderAndScreenDescriptor() error {
	err := readFull(d.r, d.tmp[:13])
	if err != nil {
		return fmt.Errorf("gif: reading header: %v", err)
	}
	d.vers = string(d.tmp[:6])
	if d.vers != "GIF87a" && d.vers != "GIF89a" {
		return fmt.Errorf("gif: can't recognize format %q", d.vers)
	}
	d.width = int(d.tmp[6]) + int(d.tmp[7])<<8
	d.height = int(d.tmp[8]) + int(d.tmp[9])<<8
	if fields := d.tmp[10]; fields&fColorTable != 0 {
		d.backgroundIndex = d.tmp[11]
		// readColorTable overwrites the contents of d.tmp, but that's OK.
		if d.globalColorTable, err = d.readColorTable(fields); err != nil {
			return err
		}
	}
	// d.tmp[12] is the Pixel Aspect Ratio, which is ignored.
	return nil
}

func (d *decoder) readColorTable(fields byte) (color.Palette, error) {
	n := 1 << (1 + uint(fields&fColorTableBitsMask))
	err := readFull(d.r, d.tmp[:3*n])
	if err != nil {
		return nil, fmt.Errorf("gif: reading color table: %s", err)
	}
	j, p := 0, make(color.Palette, n)
	for i := range p {
		p[i] = color.RGBA{d.tmp[j+0], d.tmp[j+1], d.tmp[j+2], 0xFF}
		j += 3
	}
	return p, nil
}

func (d *decoder) readExtension() error {
	extension, err := readByte(d.r)
	if err != nil {
		return fmt.Errorf("gif: reading extension: %v", err)
	}
	size := 0
	switch extension {
	case eText:
		size = 13
	case eGraphicControl:
		return d.readGraphicControl()
	case eComment:
		// nothing to do but read the data.
	case eApplication:
		b, err := readByte(d.r)
		if err != nil {
			return fmt.Errorf("gif: reading extension: %v", err)
		}
		// The spec requires size be 11, but Adobe sometimes uses 10.
		size = int(b)
	default:
		return fmt.Errorf("gif: unknown extension 0x%.2x", extension)
	}
	if size > 0 {
		if err := readFull(d.r, d.tmp[:size]); err != nil {
			return fmt.Errorf("gif: reading extension: %v", err)
		}
	}

	// Application Extension with "NETSCAPE2.0" as string and 1 in data means
	// this extension defines a loop count.
	if extension == eApplication && string(d.tmp[:size]) == "NETSCAPE2.0" {
		n, err := d.readBlock()
		if err != nil {
			return fmt.Errorf("gif: reading extension: %v", err)
		}
		if n == 0 {
			return nil
		}
		if n == 3 && d.tmp[0] == 1 {
			d.loopCount = int(d.tmp[1]) | int(d.tmp[2])<<8
		}
	}
	for {
		n, err := d.readBlock()
		if err != nil {
			return fmt.Errorf("gif: reading extension: %v", err)
		}
		if n == 0 {
			return nil
		}
	}
}

func (d *decoder) readGraphicControl() error {
	if err := readFull(d.r, d.tmp[:6]); err != nil {
		return fmt.Errorf("gif: can't read graphic control: %s", err)
	}
	if d.tmp[0] != 4 {
		return fmt.Errorf("gif: invalid graphic control extension block size: %d", d.tmp[0])
	}
	flags := d.tmp[1]
	d.disposalMethod = (flags & gcDisposalMethodMask) >> 2
	d.delayTime = int(d.tmp[2]) | int(d.tmp[3])<<8
	if flags&gcTransparentColorSet != 0 {
		d.transparentIndex = d.tmp[4]
		d.hasTransparentIndex = true
	}
	if d.tmp[5] != 0 {
		return fmt.Errorf("gif: invalid graphic control extension block terminator: %d", d.tmp[5])
	}
	return nil
}

func (d *decoder) readImageDescriptor(keepAllFrames bool) error {
	m, err := d.newImageFromDescriptor()
	if err != nil {
		return err
	}
	useLocalColorTable := d.imageFields&fColorTable != 0
	if useLocalColorTable {
		m.Palette, err = d.readColorTable(d.imageFields)
		if err != nil {
			return err
		}
	} else {
		if d.globalColorTable == nil {
			return errors.New("gif: no color table")
		}
		m.Palette = d.globalColorTable
	}
	if d.hasTransparentIndex {
		if !useLocalColorTable {
			// Clone the global color table.
			m.Palette = append(color.Palette(nil), d.globalColorTable...)
		}
		if ti := int(d.transparentIndex); ti < len(m.Palette) {
			m.Palette[ti] = color.RGBA{}
		} else {
			// The transparentIndex is out of range, which is an error
			// according to the spec, but Firefox and Google Chrome
			// seem OK with this, so we enlarge the palette with
			// transparent colors. See golang.org/issue/15059.
			p := make(color.Palette, ti+1)
			copy(p, m.Palette)
			for i := len(m.Palette); i < len(p); i++ {
				p[i] = color.RGBA{}
			}
			m.Palette = p
		}
	}
	litWidth, err := readByte(d.r)
	if err != nil {
		return fmt.Errorf("gif: reading image data: %v", err)
	}
	if litWidth < 2 || litWidth > 8 {
		return fmt.Errorf("gif: pixel size in decode out of range: %d", litWidth)
	}
	// A wonderfully Go-like piece of magic.
	br := &blockReader{d: d}
	lzwr := lzw.NewReader(br, lzw.LSB, int(litWidth))
	defer lzwr.Close()
	if err = readFull(lzwr, m.Pix); err != nil {
		if err != io.ErrUnexpectedEOF {
			return fmt.Errorf("gif: reading image data: %v", err)
		}
		return errNotEnough
	}
	// In theory, both lzwr and br should be exhausted. Reading from them
	// should yield (0, io.EOF).
	//
	// The spec (Appendix F - Compression), says that "An End of
	// Information code... must be the last code output by the encoder
	// for an image". In practice, though, giflib (a widely used C
	// library) does not enforce this, so we also accept lzwr returning
	// io.ErrUnexpectedEOF (meaning that the encoded stream hit io.EOF
	// before the LZW decoder saw an explicit end code), provided that
	// the io.ReadFull call above successfully read len(m.Pix) bytes.
	// See https://golang.org/issue/9856 for an example GIF.
	if n, err := lzwr.Read(d.tmp[256:257]); n != 0 || (err != io.EOF && err != io.ErrUnexpectedEOF) {
		if err != nil {
			return fmt.Errorf("gif: reading image data: %v", err)
		}
		return errTooMuch
	}

	// In practice, some GIFs have an extra byte in the data sub-block
	// stream, which we ignore. See https://golang.org/issue/16146.
	if err := br.close(); err == errTooMuch {
		return errTooMuch
	} else if err != nil {
		return fmt.Errorf("gif: reading image data: %v", err)
	}

	// Check that the color indexes are inside the palette.
	if len(m.Palette) < 256 {
		for _, pixel := range m.Pix {
			if int(pixel) >= len(m.Palette) {
				return errBadPixel
			}
		}
	}

	// Undo the interlacing if necessary.
	if d.imageFields&fInterlace != 0 {
		uninterlace(m)
	}

	if keepAllFrames || len(d.image) == 0 {
		d.image = append(d.image, m)
		d.delay = append(d.delay, d.delayTime)
		d.disposal = append(d.disposal, d.disposalMethod)
	}
	// The GIF89a spec, Section 23 (Graphic Control Extension) says:
	// "The scope of this extension is the first graphic rendering block
	// to follow." We therefore reset the GCE fields to zero.
	d.delayTime = 0
	d.hasTransparentIndex = false
	return nil
}

func (d *decoder) newImageFromDescriptor() (*image.Paletted, error) {
	if err := readFull(d.r, d.tmp[:9]); err != nil {
		return nil, fmt.Errorf("gif: can't read image descriptor: %s", err)
	}
	left := int(d.tmp[0]) + int(d.tmp[1])<<8
	top := int(d.tmp[2]) + int(d.tmp[3])<<8
	width := int(d.tmp[4]) + int(d.tmp[5])<<8
	height := int(d.tmp[6]) + int(d.tmp[7])<<8
	d.imageFields = d.tmp[8]

	// The GIF89a spec, Section 20 (Image Descriptor) says: "Each image must
	// fit within the boundaries of the Logical Screen, as defined in the
	// Logical Screen Descriptor."
	//
	// This is conceptually similar to testing
	//	frameBounds := image.Rect(left, top, left+width, top+height)
	//	imageBounds := image.Rect(0, 0, d.width, d.height)
	//	if !frameBounds.In(imageBounds) { etc }
	// but the semantics of the Go image.Rectangle type is that r.In(s) is true
	// whenever r is an empty rectangle, even if r.Min.X > s.Max.X. Here, we
	// want something stricter.
	//
	// Note that, by construction, left >= 0 && top >= 0, so we only have to
	// explicitly compare frameBounds.Max (left+width, top+height) against
	// imageBounds.Max (d.width, d.height) and not frameBounds.Min (left, top)
	// against imageBounds.Min (0, 0).
	if left+width > d.width || top+height > d.height {
		return nil, errors.New("gif: frame bounds larger than image bounds")
	}
	return image.NewPaletted(image.Rectangle{
		Min: image.Point{left, top},
		Max: image.Point{left + width, top + height},
	}, nil), nil
}

func (d *decoder) readBlock() (int, error) {
	n, err := readByte(d.r)
	if n == 0 || err != nil {
		return 0, err
	}
	if err := readFull(d.r, d.tmp[:n]); err != nil {
		return 0, err
	}
	return int(n), nil
}

// interlaceScan defines the ordering for a pass of the interlace algorithm.
type interlaceScan struct {
	skip, start int
}

// interlacing represents the set of scans in an interlaced GIF image.
var interlacing = []interlaceScan{
	{8, 0}, // Group 1 : Every 8th. row, starting with row 0.
	{8, 4}, // Group 2 : Every 8th. row, starting with row 4.
	{4, 2}, // Group 3 : Every 4th. row, starting with row 2.
	{2, 1}, // Group 4 : Every 2nd. row, starting with row 1.
}

// uninterlace rearranges the pixels in m to account for interlaced input.
func uninterlace(m *image.Paletted) {
	var nPix []uint8
	dx := m.Bounds().Dx()
	dy := m.Bounds().Dy()
	nPix = make([]uint8, dx*dy)
	offset := 0 // steps through the input by sequential scan lines.
	for _, pass := range interlacing {
		nOffset := pass.start * dx // steps through the output as defined by pass.
		for y := pass.start; y < dy; y += pass.skip {
			copy(nPix[nOffset:nOffset+dx], m.Pix[offset:offset+dx])
			offset += dx
			nOffset += dx * pass.skip
		}
	}
	m.Pix = nPix
}

// Decode reads a GIF image from r and returns the first embedded
// image as an [image.Image].
//
// When decoding images from untrusted sources, it is safest to
// first call DecodeConfig and check the image size so
// that unexpectedly large memory allocations may be safely
// avoided.
func Decode(r io.Reader) (image.Image, error) {
	var d decoder
	if err := d.decode(r, false, false); err != nil {
		return nil, err
	}
	return d.image[0], nil
}

// GIF represents the possibly multiple images stored in a GIF file.
type GIF struct {
	Image []*image.Paletted // The successive images.
	Delay []int             // The successive delay times, one per frame, in 100ths of a second.
	// LoopCount controls the number of times an animation will be
	// restarted during display.
	// A LoopCount of 0 means to loop forever.
	// A LoopCount of -1 means to show each frame only once.
	// Otherwise, the animation is looped LoopCount+1 times.
	LoopCount int
	// Disposal is the successive disposal methods, one per frame. For
	// backwards compatibility, a nil Disposal is valid to pass to EncodeAll,
	// and implies that each frame's disposal method is 0 (no disposal
	// specified).
	Disposal []byte
	// Config is the global color table (palette), width and height. A nil or
	// empty-color.Palette Config.ColorModel means that each frame has its own
	// color table and there is no global color table. Each frame's bounds must
	// be within the rectangle defined by the two points (0, 0) and
	// (Config.Width, Config.Height).
	//
	// For backwards compatibility, a zero-valued Config is valid to pass to
	// EncodeAll, and implies that the overall GIF's width and height equals
	// the first frame's bounds' Rectangle.Max point.
	Config image.Config
	// BackgroundIndex is the background index in the global color table, for
	// use with the DisposalBackground disposal method.
	BackgroundIndex byte
}

// DecodeAll reads a GIF image from r and returns the sequential frames
// and timing information.
//
// Like [Decode], this allocates a paletted buffer per frame from width and
// height in the image descriptors. [DecodeAll] retains every decoded frame in
// memory. For untrusted input, call [DecodeConfig] first to verify the
// logical screen size and reject inputs that would require excessive memory.
func DecodeAll(r io.Reader) (*GIF, error) {
	var d decoder
	if err := d.decode(r, false, true); err != nil {
		return nil, err
	}
	gif := &GIF{
		Image:     d.image,
		LoopCount: d.loopCount,
		Delay:     d.delay,
		Disposal:  d.disposal,
		Config: image.Config{
			ColorModel: d.globalColorTable,
			Width:      d.width,
			Height:     d.height,
		},
		BackgroundIndex: d.backgroundIndex,
	}
	return gif, nil
}

// DecodeConfig returns the global color model and dimensions of a GIF image
// without decoding the entire image.
//
// It reads the logical screen descriptor and global color table only; it does
// not allocate pixel buffers for frames. Use it to check width and height
// before calling [Decode] or [DecodeAll].
func DecodeConfig(r io.Reader) (image.Config, error) {
	var d decoder
	if err := d.decode(r, true, false); err != nil {
		return image.Config{}, err
	}
	return image.Config{
		ColorModel: d.globalColorTable,
		Width:      d.width,
		Height:     d.height,
	}, nil
}

func init() {
	image.RegisterFormat("gif", "GIF8?a", Decode, DecodeConfig)
}

```

// === FILE: references/go/src/image/gif/writer.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gif

import (
	"bufio"
	"bytes"
	"compress/lzw"
	"errors"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"internal/byteorder"
	"io"
	"math/bits"
)

// Graphic control extension fields.
const (
	gcLabel     = 0xF9
	gcBlockSize = 0x04
)

func log2(x int) int {
	if x < 2 {
		return 0
	}
	return bits.Len(uint(x-1)) - 1
}

// writer is a buffered writer.
type writer interface {
	Flush() error
	io.Writer
	io.ByteWriter
}

// encoder encodes an image to the GIF format.
type encoder struct {
	// w is the writer to write to. err is the first error encountered during
	// writing. All attempted writes after the first error become no-ops.
	w   writer
	err error
	// g is a reference to the data that is being encoded.
	g GIF
	// globalCT is the size in bytes of the global color table.
	globalCT int
	// buf is a scratch buffer. It must be at least 256 for the blockWriter.
	buf              [256]byte
	globalColorTable [3 * 256]byte
	localColorTable  [3 * 256]byte
}

// blockWriter writes the block structure of GIF image data, which
// comprises (n, (n bytes)) blocks, with 1 <= n <= 255. It is the
// writer given to the LZW encoder, which is thus immune to the
// blocking.
type blockWriter struct {
	e *encoder
}

func (b blockWriter) setup() {
	b.e.buf[0] = 0
}

func (b blockWriter) Flush() error {
	return b.e.err
}

func (b blockWriter) WriteByte(c byte) error {
	if b.e.err != nil {
		return b.e.err
	}

	// Append c to buffered sub-block.
	b.e.buf[0]++
	b.e.buf[b.e.buf[0]] = c
	if b.e.buf[0] < 255 {
		return nil
	}

	// Flush block
	b.e.write(b.e.buf[:256])
	b.e.buf[0] = 0
	return b.e.err
}

// blockWriter must be an io.Writer for lzw.NewWriter, but this is never
// actually called.
func (b blockWriter) Write(data []byte) (int, error) {
	for i, c := range data {
		if err := b.WriteByte(c); err != nil {
			return i, err
		}
	}
	return len(data), nil
}

func (b blockWriter) close() {
	// Write the block terminator (0x00), either by itself, or along with a
	// pending sub-block.
	if b.e.buf[0] == 0 {
		b.e.writeByte(0)
	} else {
		n := uint(b.e.buf[0])
		b.e.buf[n+1] = 0
		b.e.write(b.e.buf[:n+2])
	}
	b.e.flush()
}

func (e *encoder) flush() {
	if e.err != nil {
		return
	}
	e.err = e.w.Flush()
}

func (e *encoder) write(p []byte) {
	if e.err != nil {
		return
	}
	_, e.err = e.w.Write(p)
}

func (e *encoder) writeByte(b byte) {
	if e.err != nil {
		return
	}
	e.err = e.w.WriteByte(b)
}

func (e *encoder) writeHeader() {
	if e.err != nil {
		return
	}
	_, e.err = io.WriteString(e.w, "GIF89a")
	if e.err != nil {
		return
	}

	// Logical screen width and height.
	byteorder.LEPutUint16(e.buf[0:2], uint16(e.g.Config.Width))
	byteorder.LEPutUint16(e.buf[2:4], uint16(e.g.Config.Height))
	e.write(e.buf[:4])

	if p, ok := e.g.Config.ColorModel.(color.Palette); ok && len(p) > 0 {
		paddedSize := log2(len(p)) // Size of Global Color Table: 2^(1+n).
		e.buf[0] = fColorTable | uint8(paddedSize)
		e.buf[1] = e.g.BackgroundIndex
		e.buf[2] = 0x00 // Pixel Aspect Ratio.
		e.write(e.buf[:3])
		var err error
		e.globalCT, err = encodeColorTable(e.globalColorTable[:], p, paddedSize)
		if err != nil && e.err == nil {
			e.err = err
			return
		}
		e.write(e.globalColorTable[:e.globalCT])
	} else {
		// All frames have a local color table, so a global color table
		// is not needed.
		e.buf[0] = 0x00
		e.buf[1] = 0x00 // Background Color Index.
		e.buf[2] = 0x00 // Pixel Aspect Ratio.
		e.write(e.buf[:3])
	}

	// Add animation info if necessary.
	if len(e.g.Image) > 1 && e.g.LoopCount >= 0 {
		e.buf[0] = 0x21 // Extension Introducer.
		e.buf[1] = 0xff // Application Label.
		e.buf[2] = 0x0b // Block Size.
		e.write(e.buf[:3])
		_, err := io.WriteString(e.w, "NETSCAPE2.0") // Application Identifier.
		if err != nil && e.err == nil {
			e.err = err
			return
		}
		e.buf[0] = 0x03 // Block Size.
		e.buf[1] = 0x01 // Sub-block Index.
		byteorder.LEPutUint16(e.buf[2:4], uint16(e.g.LoopCount))
		e.buf[4] = 0x00 // Block Terminator.
		e.write(e.buf[:5])
	}
}

func encodeColorTable(dst []byte, p color.Palette, size int) (int, error) {
	if uint(size) >= 8 {
		return 0, errors.New("gif: cannot encode color table with more than 256 entries")
	}
	for i, c := range p {
		if c == nil {
			return 0, errors.New("gif: cannot encode color table with nil entries")
		}
		var r, g, b uint8
		// It is most likely that the palette is full of color.RGBAs, so they
		// get a fast path.
		if rgba, ok := c.(color.RGBA); ok {
			r, g, b = rgba.R, rgba.G, rgba.B
		} else {
			rr, gg, bb, _ := c.RGBA()
			r, g, b = uint8(rr>>8), uint8(gg>>8), uint8(bb>>8)
		}
		dst[3*i+0] = r
		dst[3*i+1] = g
		dst[3*i+2] = b
	}
	n := 1 << (size + 1)
	if n > len(p) {
		// Pad with black.
		clear(dst[3*len(p) : 3*n])
	}
	return 3 * n, nil
}

func (e *encoder) colorTablesMatch(localLen, transparentIndex int) bool {
	localSize := 3 * localLen
	if transparentIndex >= 0 {
		trOff := 3 * transparentIndex
		return bytes.Equal(e.globalColorTable[:trOff], e.localColorTable[:trOff]) &&
			bytes.Equal(e.globalColorTable[trOff+3:localSize], e.localColorTable[trOff+3:localSize])
	}
	return bytes.Equal(e.globalColorTable[:localSize], e.localColorTable[:localSize])
}

func (e *encoder) writeImageBlock(pm *image.Paletted, delay int, disposal byte) {
	if e.err != nil {
		return
	}

	if len(pm.Palette) == 0 {
		e.err = errors.New("gif: cannot encode image block with empty palette")
		return
	}

	b := pm.Bounds()
	if b.Min.X < 0 || b.Max.X >= 1<<16 || b.Min.Y < 0 || b.Max.Y >= 1<<16 {
		e.err = errors.New("gif: image block is too large to encode")
		return
	}
	if !b.In(image.Rectangle{Max: image.Point{e.g.Config.Width, e.g.Config.Height}}) {
		e.err = errors.New("gif: image block is out of bounds")
		return
	}

	transparentIndex := -1
	for i, c := range pm.Palette {
		if c == nil {
			e.err = errors.New("gif: cannot encode color table with nil entries")
			return
		}
		if _, _, _, a := c.RGBA(); a == 0 {
			transparentIndex = i
			break
		}
	}

	if delay > 0 || disposal != 0 || transparentIndex != -1 {
		e.buf[0] = sExtension  // Extension Introducer.
		e.buf[1] = gcLabel     // Graphic Control Label.
		e.buf[2] = gcBlockSize // Block Size.
		if transparentIndex != -1 {
			e.buf[3] = 0x01 | disposal<<2
		} else {
			e.buf[3] = 0x00 | disposal<<2
		}
		byteorder.LEPutUint16(e.buf[4:6], uint16(delay)) // Delay Time (1/100ths of a second)

		// Transparent color index.
		if transparentIndex != -1 {
			e.buf[6] = uint8(transparentIndex)
		} else {
			e.buf[6] = 0x00
		}
		e.buf[7] = 0x00 // Block Terminator.
		e.write(e.buf[:8])
	}
	e.buf[0] = sImageDescriptor
	byteorder.LEPutUint16(e.buf[1:3], uint16(b.Min.X))
	byteorder.LEPutUint16(e.buf[3:5], uint16(b.Min.Y))
	byteorder.LEPutUint16(e.buf[5:7], uint16(b.Dx()))
	byteorder.LEPutUint16(e.buf[7:9], uint16(b.Dy()))
	e.write(e.buf[:9])

	// To determine whether or not this frame's palette is the same as the
	// global palette, we can check a couple things. First, do they actually
	// point to the same []color.Color? If so, they are equal so long as the
	// frame's palette is not longer than the global palette...
	paddedSize := log2(len(pm.Palette)) // Size of Local Color Table: 2^(1+n).
	if gp, ok := e.g.Config.ColorModel.(color.Palette); ok && len(pm.Palette) <= len(gp) && &gp[0] == &pm.Palette[0] {
		e.writeByte(0) // Use the global color table.
	} else {
		ct, err := encodeColorTable(e.localColorTable[:], pm.Palette, paddedSize)
		if err != nil {
			if e.err == nil {
				e.err = err
			}
			return
		}
		// This frame's palette is not the very same slice as the global
		// palette, but it might be a copy, possibly with one value turned into
		// transparency by DecodeAll.
		if ct <= e.globalCT && e.colorTablesMatch(len(pm.Palette), transparentIndex) {
			e.writeByte(0) // Use the global color table.
		} else {
			// Use a local color table.
			e.writeByte(fColorTable | uint8(paddedSize))
			e.write(e.localColorTable[:ct])
		}
	}

	litWidth := paddedSize + 1
	if litWidth < 2 {
		litWidth = 2
	}
	e.writeByte(uint8(litWidth)) // LZW Minimum Code Size.

	bw := blockWriter{e: e}
	bw.setup()
	lzww := lzw.NewWriter(bw, lzw.LSB, litWidth)
	if dx := b.Dx(); dx == pm.Stride {
		_, e.err = lzww.Write(pm.Pix[:dx*b.Dy()])
		if e.err != nil {
			lzww.Close()
			return
		}
	} else {
		for i, y := 0, b.Min.Y; y < b.Max.Y; i, y = i+pm.Stride, y+1 {
			_, e.err = lzww.Write(pm.Pix[i : i+dx])
			if e.err != nil {
				lzww.Close()
				return
			}
		}
	}
	lzww.Close() // flush to bw
	bw.close()   // flush to e.w
}

// Options are the encoding parameters.
type Options struct {
	// NumColors is the maximum number of colors used in the image.
	// It ranges from 1 to 256.
	NumColors int

	// Quantizer is used to produce a palette with size NumColors.
	// palette.Plan9 is used in place of a nil Quantizer.
	Quantizer draw.Quantizer

	// Drawer is used to convert the source image to the desired palette.
	// draw.FloydSteinberg is used in place of a nil Drawer.
	Drawer draw.Drawer
}

// EncodeAll writes the images in g to w in GIF format with the
// given loop count and delay between frames.
func EncodeAll(w io.Writer, g *GIF) error {
	if len(g.Image) == 0 {
		return errors.New("gif: must provide at least one image")
	}

	if len(g.Image) != len(g.Delay) {
		return errors.New("gif: mismatched image and delay lengths")
	}

	e := encoder{g: *g}
	// The GIF.Disposal, GIF.Config and GIF.BackgroundIndex fields were added
	// in Go 1.5. Valid Go 1.4 code, such as when the Disposal field is omitted
	// in a GIF struct literal, should still produce valid GIFs.
	if e.g.Disposal != nil && len(e.g.Image) != len(e.g.Disposal) {
		return errors.New("gif: mismatched image and disposal lengths")
	}
	if e.g.Config == (image.Config{}) {
		p := g.Image[0].Bounds().Max
		e.g.Config.Width = p.X
		e.g.Config.Height = p.Y
	} else if e.g.Config.ColorModel != nil {
		if _, ok := e.g.Config.ColorModel.(color.Palette); !ok {
			return errors.New("gif: GIF color model must be a color.Palette")
		}
	}

	if ww, ok := w.(writer); ok {
		e.w = ww
	} else {
		e.w = bufio.NewWriter(w)
	}

	e.writeHeader()
	for i, pm := range g.Image {
		disposal := uint8(0)
		if g.Disposal != nil {
			disposal = g.Disposal[i]
		}
		e.writeImageBlock(pm, g.Delay[i], disposal)
	}
	e.writeByte(sTrailer)
	e.flush()
	return e.err
}

// Encode writes the Image m to w in GIF format.
func Encode(w io.Writer, m image.Image, o *Options) error {
	// Check for bounds and size restrictions.
	b := m.Bounds()
	if b.Dx() >= 1<<16 || b.Dy() >= 1<<16 {
		return errors.New("gif: image is too large to encode")
	}

	opts := Options{}
	if o != nil {
		opts = *o
	}
	if opts.NumColors < 1 || 256 < opts.NumColors {
		opts.NumColors = 256
	}
	if opts.Drawer == nil {
		opts.Drawer = draw.FloydSteinberg
	}

	pm, _ := m.(*image.Paletted)
	if pm == nil {
		if cp, ok := m.ColorModel().(color.Palette); ok {
			pm = image.NewPaletted(b, cp)
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					pm.Set(x, y, cp.Convert(m.At(x, y)))
				}
			}
		}
	}
	if pm == nil || len(pm.Palette) > opts.NumColors {
		// Set pm to be a palettedized copy of m, including its bounds, which
		// might not start at (0, 0).
		//
		// TODO: Pick a better sub-sample of the Plan 9 palette.
		pm = image.NewPaletted(b, palette.Plan9[:opts.NumColors])
		if opts.Quantizer != nil {
			pm.Palette = opts.Quantizer.Quantize(make(color.Palette, 0, opts.NumColors), m)
		}
		opts.Drawer.Draw(pm, b, m, b.Min)
	}

	// When calling Encode instead of EncodeAll, the single-frame image is
	// translated such that its top-left corner is (0, 0), so that the single
	// frame completely fills the overall GIF's bounds.
	if pm.Rect.Min != (image.Point{}) {
		dup := *pm
		dup.Rect = dup.Rect.Sub(dup.Rect.Min)
		pm = &dup
	}

	return EncodeAll(w, &GIF{
		Image: []*image.Paletted{pm},
		Delay: []int{0},
		Config: image.Config{
			ColorModel: pm.Palette,
			Width:      b.Dx(),
			Height:     b.Dy(),
		},
	})
}

```

// === FILE: references/go/src/image/image.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package image implements a basic 2-D image library.
//
// The fundamental interface is called [Image]. An [Image] contains colors, which
// are described in the image/color package.
//
// Values of the [Image] interface are created either by calling functions such
// as [NewRGBA] and [NewPaletted], or by calling [Decode] on an [io.Reader] containing
// image data in a format such as GIF, JPEG or PNG. Decoding any particular
// image format requires the prior registration of a decoder function.
// Registration is typically automatic as a side effect of initializing that
// format's package so that, to decode a PNG image, it suffices to have
//
//	import _ "image/png"
//
// in a program's main package. The _ means to import a package purely for its
// initialization side effects.
//
// See "The Go image package" for more details:
// https://golang.org/doc/articles/image_package.html
//
// # Security Considerations
//
// The image package can be used to parse arbitrarily large images, which can
// cause resource exhaustion on machines which do not have enough memory to
// store them. When operating on arbitrary images, [DecodeConfig] should be called
// before [Decode], so that the program can decide whether the image, as defined
// in the returned header, can be safely decoded with the available resources. A
// call to [Decode] which produces an extremely large image, as defined in the
// header returned by [DecodeConfig], is not considered a security issue,
// regardless of whether the image is itself malformed or not. A call to
// [DecodeConfig] which returns a header which does not match the image returned
// by [Decode] may be considered a security issue, and should be reported per the
// [Go Security Policy].
//
// [Go Security Policy]: https://go.dev/security/policy
package image

import (
	"image/color"
)

// Config holds an image's color model and dimensions.
type Config struct {
	ColorModel    color.Model
	Width, Height int
}

// Image is a finite rectangular grid of [color.Color] values taken from a color
// model.
type Image interface {
	// ColorModel returns the Image's color model.
	ColorModel() color.Model
	// Bounds returns the domain for which At can return non-zero color.
	// The bounds do not necessarily contain the point (0, 0).
	Bounds() Rectangle
	// At returns the color of the pixel at (x, y).
	// At(Bounds().Min.X, Bounds().Min.Y) returns the upper-left pixel of the grid.
	// At(Bounds().Max.X-1, Bounds().Max.Y-1) returns the lower-right one.
	At(x, y int) color.Color
}

// RGBA64Image is an [Image] whose pixels can be converted directly to a
// color.RGBA64.
type RGBA64Image interface {
	// RGBA64At returns the RGBA64 color of the pixel at (x, y). It is
	// equivalent to calling At(x, y).RGBA() and converting the resulting
	// 32-bit return values to a color.RGBA64, but it can avoid allocations
	// from converting concrete color types to the color.Color interface type.
	RGBA64At(x, y int) color.RGBA64
	Image
}

// PalettedImage is an image whose colors may come from a limited palette.
// If m is a PalettedImage and m.ColorModel() returns a [color.Palette] p,
// then m.At(x, y) should be equivalent to p[m.ColorIndexAt(x, y)]. If m's
// color model is not a color.Palette, then ColorIndexAt's behavior is
// undefined.
type PalettedImage interface {
	// ColorIndexAt returns the palette index of the pixel at (x, y).
	ColorIndexAt(x, y int) uint8
	Image
}

// pixelBufferLength returns the length of the []uint8 typed Pix slice field
// for the NewXxx functions. Conceptually, this is just (bpp * width * height),
// but this function panics if at least one of those is negative or if the
// computation would overflow the int type.
//
// This panics instead of returning an error because of backwards
// compatibility. The NewXxx functions do not return an error.
func pixelBufferLength(bytesPerPixel int, r Rectangle, imageTypeName string) int {
	totalLength := mul3NonNeg(bytesPerPixel, r.Dx(), r.Dy())
	if totalLength < 0 {
		panic("image: New" + imageTypeName + " Rectangle has huge or negative dimensions")
	}
	return totalLength
}

// RGBA is an in-memory image whose At method returns [color.RGBA] values.
type RGBA struct {
	// Pix holds the image's pixels, in R, G, B, A order. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*4].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect Rectangle
}

func (p *RGBA) ColorModel() color.Model { return color.RGBAModel }

func (p *RGBA) Bounds() Rectangle { return p.Rect }

func (p *RGBA) At(x, y int) color.Color {
	return p.RGBAAt(x, y)
}

func (p *RGBA) RGBA64At(x, y int) color.RGBA64 {
	if !(Point{x, y}.In(p.Rect)) {
		return color.RGBA64{}
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	r := uint16(s[0])
	g := uint16(s[1])
	b := uint16(s[2])
	a := uint16(s[3])
	return color.RGBA64{
		(r << 8) | r,
		(g << 8) | g,
		(b << 8) | b,
		(a << 8) | a,
	}
}

func (p *RGBA) RGBAAt(x, y int) color.RGBA {
	if !(Point{x, y}.In(p.Rect)) {
		return color.RGBA{}
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	return color.RGBA{s[0], s[1], s[2], s[3]}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *RGBA) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*4
}

func (p *RGBA) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	c1 := color.RGBAModel.Convert(c).(color.RGBA)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = c1.R
	s[1] = c1.G
	s[2] = c1.B
	s[3] = c1.A
}

func (p *RGBA) SetRGBA64(x, y int, c color.RGBA64) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = uint8(c.R >> 8)
	s[1] = uint8(c.G >> 8)
	s[2] = uint8(c.B >> 8)
	s[3] = uint8(c.A >> 8)
}

func (p *RGBA) SetRGBA(x, y int, c color.RGBA) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = c.R
	s[1] = c.G
	s[2] = c.B
	s[3] = c.A
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *RGBA) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &RGBA{}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &RGBA{
		Pix:    p.Pix[i:],
		Stride: p.Stride,
		Rect:   r,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *RGBA) Opaque() bool {
	if p.Rect.Empty() {
		return true
	}
	i0, i1 := 3, p.Rect.Dx()*4
	for y := p.Rect.Min.Y; y < p.Rect.Max.Y; y++ {
		for i := i0; i < i1; i += 4 {
			if p.Pix[i] != 0xff {
				return false
			}
		}
		i0 += p.Stride
		i1 += p.Stride
	}
	return true
}

// NewRGBA returns a new [RGBA] image with the given bounds.
func NewRGBA(r Rectangle) *RGBA {
	return &RGBA{
		Pix:    make([]uint8, pixelBufferLength(4, r, "RGBA")),
		Stride: 4 * r.Dx(),
		Rect:   r,
	}
}

// RGBA64 is an in-memory image whose At method returns [color.RGBA64] values.
type RGBA64 struct {
	// Pix holds the image's pixels, in R, G, B, A order and big-endian format. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*8].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect Rectangle
}

func (p *RGBA64) ColorModel() color.Model { return color.RGBA64Model }

func (p *RGBA64) Bounds() Rectangle { return p.Rect }

func (p *RGBA64) At(x, y int) color.Color {
	return p.RGBA64At(x, y)
}

func (p *RGBA64) RGBA64At(x, y int) color.RGBA64 {
	if !(Point{x, y}.In(p.Rect)) {
		return color.RGBA64{}
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+8 : i+8] // Small cap improves performance, see https://golang.org/issue/27857
	return color.RGBA64{
		uint16(s[0])<<8 | uint16(s[1]),
		uint16(s[2])<<8 | uint16(s[3]),
		uint16(s[4])<<8 | uint16(s[5]),
		uint16(s[6])<<8 | uint16(s[7]),
	}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *RGBA64) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*8
}

func (p *RGBA64) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	c1 := color.RGBA64Model.Convert(c).(color.RGBA64)
	s := p.Pix[i : i+8 : i+8] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = uint8(c1.R >> 8)
	s[1] = uint8(c1.R)
	s[2] = uint8(c1.G >> 8)
	s[3] = uint8(c1.G)
	s[4] = uint8(c1.B >> 8)
	s[5] = uint8(c1.B)
	s[6] = uint8(c1.A >> 8)
	s[7] = uint8(c1.A)
}

func (p *RGBA64) SetRGBA64(x, y int, c color.RGBA64) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+8 : i+8] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = uint8(c.R >> 8)
	s[1] = uint8(c.R)
	s[2] = uint8(c.G >> 8)
	s[3] = uint8(c.G)
	s[4] = uint8(c.B >> 8)
	s[5] = uint8(c.B)
	s[6] = uint8(c.A >> 8)
	s[7] = uint8(c.A)
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *RGBA64) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &RGBA64{}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &RGBA64{
		Pix:    p.Pix[i:],
		Stride: p.Stride,
		Rect:   r,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *RGBA64) Opaque() bool {
	if p.Rect.Empty() {
		return true
	}
	i0, i1 := 6, p.Rect.Dx()*8
	for y := p.Rect.Min.Y; y < p.Rect.Max.Y; y++ {
		for i := i0; i < i1; i += 8 {
			if p.Pix[i+0] != 0xff || p.Pix[i+1] != 0xff {
				return false
			}
		}
		i0 += p.Stride
		i1 += p.Stride
	}
	return true
}

// NewRGBA64 returns a new [RGBA64] image with the given bounds.
func NewRGBA64(r Rectangle) *RGBA64 {
	return &RGBA64{
		Pix:    make([]uint8, pixelBufferLength(8, r, "RGBA64")),
		Stride: 8 * r.Dx(),
		Rect:   r,
	}
}

// NRGBA is an in-memory image whose At method returns [color.NRGBA] values.
type NRGBA struct {
	// Pix holds the image's pixels, in R, G, B, A order. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*4].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect Rectangle
}

func (p *NRGBA) ColorModel() color.Model { return color.NRGBAModel }

func (p *NRGBA) Bounds() Rectangle { return p.Rect }

func (p *NRGBA) At(x, y int) color.Color {
	return p.NRGBAAt(x, y)
}

func (p *NRGBA) RGBA64At(x, y int) color.RGBA64 {
	r, g, b, a := p.NRGBAAt(x, y).RGBA()
	return color.RGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
}

func (p *NRGBA) NRGBAAt(x, y int) color.NRGBA {
	if !(Point{x, y}.In(p.Rect)) {
		return color.NRGBA{}
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	return color.NRGBA{s[0], s[1], s[2], s[3]}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *NRGBA) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*4
}

func (p *NRGBA) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	c1 := color.NRGBAModel.Convert(c).(color.NRGBA)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = c1.R
	s[1] = c1.G
	s[2] = c1.B
	s[3] = c1.A
}

func (p *NRGBA) SetRGBA64(x, y int, c color.RGBA64) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	r, g, b, a := uint32(c.R), uint32(c.G), uint32(c.B), uint32(c.A)
	if (a != 0) && (a != 0xffff) {
		r = (r * 0xffff) / a
		g = (g * 0xffff) / a
		b = (b * 0xffff) / a
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = uint8(r >> 8)
	s[1] = uint8(g >> 8)
	s[2] = uint8(b >> 8)
	s[3] = uint8(a >> 8)
}

func (p *NRGBA) SetNRGBA(x, y int, c color.NRGBA) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = c.R
	s[1] = c.G
	s[2] = c.B
	s[3] = c.A
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *NRGBA) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &NRGBA{}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &NRGBA{
		Pix:    p.Pix[i:],
		Stride: p.Stride,
		Rect:   r,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *NRGBA) Opaque() bool {
	if p.Rect.Empty() {
		return true
	}
	i0, i1 := 3, p.Rect.Dx()*4
	for y := p.Rect.Min.Y; y < p.Rect.Max.Y; y++ {
		for i := i0; i < i1; i += 4 {
			if p.Pix[i] != 0xff {
				return false
			}
		}
		i0 += p.Stride
		i1 += p.Stride
	}
	return true
}

// NewNRGBA returns a new [NRGBA] image with the given bounds.
func NewNRGBA(r Rectangle) *NRGBA {
	return &NRGBA{
		Pix:    make([]uint8, pixelBufferLength(4, r, "NRGBA")),
		Stride: 4 * r.Dx(),
		Rect:   r,
	}
}

// NRGBA64 is an in-memory image whose At method returns [color.NRGBA64] values.
type NRGBA64 struct {
	// Pix holds the image's pixels, in R, G, B, A order and big-endian format. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*8].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect Rectangle
}

func (p *NRGBA64) ColorModel() color.Model { return color.NRGBA64Model }

func (p *NRGBA64) Bounds() Rectangle { return p.Rect }

func (p *NRGBA64) At(x, y int) color.Color {
	return p.NRGBA64At(x, y)
}

func (p *NRGBA64) RGBA64At(x, y int) color.RGBA64 {
	r, g, b, a := p.NRGBA64At(x, y).RGBA()
	return color.RGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
}

func (p *NRGBA64) NRGBA64At(x, y int) color.NRGBA64 {
	if !(Point{x, y}.In(p.Rect)) {
		return color.NRGBA64{}
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+8 : i+8] // Small cap improves performance, see https://golang.org/issue/27857
	return color.NRGBA64{
		uint16(s[0])<<8 | uint16(s[1]),
		uint16(s[2])<<8 | uint16(s[3]),
		uint16(s[4])<<8 | uint16(s[5]),
		uint16(s[6])<<8 | uint16(s[7]),
	}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *NRGBA64) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*8
}

func (p *NRGBA64) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	c1 := color.NRGBA64Model.Convert(c).(color.NRGBA64)
	s := p.Pix[i : i+8 : i+8] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = uint8(c1.R >> 8)
	s[1] = uint8(c1.R)
	s[2] = uint8(c1.G >> 8)
	s[3] = uint8(c1.G)
	s[4] = uint8(c1.B >> 8)
	s[5] = uint8(c1.B)
	s[6] = uint8(c1.A >> 8)
	s[7] = uint8(c1.A)
}

func (p *NRGBA64) SetRGBA64(x, y int, c color.RGBA64) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	r, g, b, a := uint32(c.R), uint32(c.G), uint32(c.B), uint32(c.A)
	if (a != 0) && (a != 0xffff) {
		r = (r * 0xffff) / a
		g = (g * 0xffff) / a
		b = (b * 0xffff) / a
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+8 : i+8] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = uint8(r >> 8)
	s[1] = uint8(r)
	s[2] = uint8(g >> 8)
	s[3] = uint8(g)
	s[4] = uint8(b >> 8)
	s[5] = uint8(b)
	s[6] = uint8(a >> 8)
	s[7] = uint8(a)
}

func (p *NRGBA64) SetNRGBA64(x, y int, c color.NRGBA64) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+8 : i+8] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = uint8(c.R >> 8)
	s[1] = uint8(c.R)
	s[2] = uint8(c.G >> 8)
	s[3] = uint8(c.G)
	s[4] = uint8(c.B >> 8)
	s[5] = uint8(c.B)
	s[6] = uint8(c.A >> 8)
	s[7] = uint8(c.A)
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *NRGBA64) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &NRGBA64{}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &NRGBA64{
		Pix:    p.Pix[i:],
		Stride: p.Stride,
		Rect:   r,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *NRGBA64) Opaque() bool {
	if p.Rect.Empty() {
		return true
	}
	i0, i1 := 6, p.Rect.Dx()*8
	for y := p.Rect.Min.Y; y < p.Rect.Max.Y; y++ {
		for i := i0; i < i1; i += 8 {
			if p.Pix[i+0] != 0xff || p.Pix[i+1] != 0xff {
				return false
			}
		}
		i0 += p.Stride
		i1 += p.Stride
	}
	return true
}

// NewNRGBA64 returns a new [NRGBA64] image with the given bounds.
func NewNRGBA64(r Rectangle) *NRGBA64 {
	return &NRGBA64{
		Pix:    make([]uint8, pixelBufferLength(8, r, "NRGBA64")),
		Stride: 8 * r.Dx(),
		Rect:   r,
	}
}

// Alpha is an in-memory image whose At method returns [color.Alpha] values.
type Alpha struct {
	// Pix holds the image's pixels, as alpha values. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*1].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect Rectangle
}

func (p *Alpha) ColorModel() color.Model { return color.AlphaModel }

func (p *Alpha) Bounds() Rectangle { return p.Rect }

func (p *Alpha) At(x, y int) color.Color {
	return p.AlphaAt(x, y)
}

func (p *Alpha) RGBA64At(x, y int) color.RGBA64 {
	a := uint16(p.AlphaAt(x, y).A)
	a |= a << 8
	return color.RGBA64{a, a, a, a}
}

func (p *Alpha) AlphaAt(x, y int) color.Alpha {
	if !(Point{x, y}.In(p.Rect)) {
		return color.Alpha{}
	}
	i := p.PixOffset(x, y)
	return color.Alpha{p.Pix[i]}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *Alpha) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*1
}

func (p *Alpha) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i] = color.AlphaModel.Convert(c).(color.Alpha).A
}

func (p *Alpha) SetRGBA64(x, y int, c color.RGBA64) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i] = uint8(c.A >> 8)
}

func (p *Alpha) SetAlpha(x, y int, c color.Alpha) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i] = c.A
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *Alpha) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &Alpha{}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &Alpha{
		Pix:    p.Pix[i:],
		Stride: p.Stride,
		Rect:   r,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *Alpha) Opaque() bool {
	if p.Rect.Empty() {
		return true
	}
	i0, i1 := 0, p.Rect.Dx()
	for y := p.Rect.Min.Y; y < p.Rect.Max.Y; y++ {
		for i := i0; i < i1; i++ {
			if p.Pix[i] != 0xff {
				return false
			}
		}
		i0 += p.Stride
		i1 += p.Stride
	}
	return true
}

// NewAlpha returns a new [Alpha] image with the given bounds.
func NewAlpha(r Rectangle) *Alpha {
	return &Alpha{
		Pix:    make([]uint8, pixelBufferLength(1, r, "Alpha")),
		Stride: 1 * r.Dx(),
		Rect:   r,
	}
}

// Alpha16 is an in-memory image whose At method returns [color.Alpha16] values.
type Alpha16 struct {
	// Pix holds the image's pixels, as alpha values in big-endian format. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*2].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect Rectangle
}

func (p *Alpha16) ColorModel() color.Model { return color.Alpha16Model }

func (p *Alpha16) Bounds() Rectangle { return p.Rect }

func (p *Alpha16) At(x, y int) color.Color {
	return p.Alpha16At(x, y)
}

func (p *Alpha16) RGBA64At(x, y int) color.RGBA64 {
	a := p.Alpha16At(x, y).A
	return color.RGBA64{a, a, a, a}
}

func (p *Alpha16) Alpha16At(x, y int) color.Alpha16 {
	if !(Point{x, y}.In(p.Rect)) {
		return color.Alpha16{}
	}
	i := p.PixOffset(x, y)
	return color.Alpha16{uint16(p.Pix[i+0])<<8 | uint16(p.Pix[i+1])}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *Alpha16) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*2
}

func (p *Alpha16) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	c1 := color.Alpha16Model.Convert(c).(color.Alpha16)
	p.Pix[i+0] = uint8(c1.A >> 8)
	p.Pix[i+1] = uint8(c1.A)
}

func (p *Alpha16) SetRGBA64(x, y int, c color.RGBA64) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i+0] = uint8(c.A >> 8)
	p.Pix[i+1] = uint8(c.A)
}

func (p *Alpha16) SetAlpha16(x, y int, c color.Alpha16) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i+0] = uint8(c.A >> 8)
	p.Pix[i+1] = uint8(c.A)
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *Alpha16) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &Alpha16{}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &Alpha16{
		Pix:    p.Pix[i:],
		Stride: p.Stride,
		Rect:   r,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *Alpha16) Opaque() bool {
	if p.Rect.Empty() {
		return true
	}
	i0, i1 := 0, p.Rect.Dx()*2
	for y := p.Rect.Min.Y; y < p.Rect.Max.Y; y++ {
		for i := i0; i < i1; i += 2 {
			if p.Pix[i+0] != 0xff || p.Pix[i+1] != 0xff {
				return false
			}
		}
		i0 += p.Stride
		i1 += p.Stride
	}
	return true
}

// NewAlpha16 returns a new [Alpha16] image with the given bounds.
func NewAlpha16(r Rectangle) *Alpha16 {
	return &Alpha16{
		Pix:    make([]uint8, pixelBufferLength(2, r, "Alpha16")),
		Stride: 2 * r.Dx(),
		Rect:   r,
	}
}

// Gray is an in-memory image whose At method returns [color.Gray] values.
type Gray struct {
	// Pix holds the image's pixels, as gray values. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*1].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect Rectangle
}

func (p *Gray) ColorModel() color.Model { return color.GrayModel }

func (p *Gray) Bounds() Rectangle { return p.Rect }

func (p *Gray) At(x, y int) color.Color {
	return p.GrayAt(x, y)
}

func (p *Gray) RGBA64At(x, y int) color.RGBA64 {
	gray := uint16(p.GrayAt(x, y).Y)
	gray |= gray << 8
	return color.RGBA64{gray, gray, gray, 0xffff}
}

func (p *Gray) GrayAt(x, y int) color.Gray {
	if !(Point{x, y}.In(p.Rect)) {
		return color.Gray{}
	}
	i := p.PixOffset(x, y)
	return color.Gray{p.Pix[i]}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *Gray) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*1
}

func (p *Gray) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i] = color.GrayModel.Convert(c).(color.Gray).Y
}

func (p *Gray) SetRGBA64(x, y int, c color.RGBA64) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	// This formula is the same as in color.grayModel.
	gray := (19595*uint32(c.R) + 38470*uint32(c.G) + 7471*uint32(c.B) + 1<<15) >> 24
	i := p.PixOffset(x, y)
	p.Pix[i] = uint8(gray)
}

func (p *Gray) SetGray(x, y int, c color.Gray) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i] = c.Y
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *Gray) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &Gray{}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &Gray{
		Pix:    p.Pix[i:],
		Stride: p.Stride,
		Rect:   r,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *Gray) Opaque() bool {
	return true
}

// NewGray returns a new [Gray] image with the given bounds.
func NewGray(r Rectangle) *Gray {
	return &Gray{
		Pix:    make([]uint8, pixelBufferLength(1, r, "Gray")),
		Stride: 1 * r.Dx(),
		Rect:   r,
	}
}

// Gray16 is an in-memory image whose At method returns [color.Gray16] values.
type Gray16 struct {
	// Pix holds the image's pixels, as gray values in big-endian format. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*2].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect Rectangle
}

func (p *Gray16) ColorModel() color.Model { return color.Gray16Model }

func (p *Gray16) Bounds() Rectangle { return p.Rect }

func (p *Gray16) At(x, y int) color.Color {
	return p.Gray16At(x, y)
}

func (p *Gray16) RGBA64At(x, y int) color.RGBA64 {
	gray := p.Gray16At(x, y).Y
	return color.RGBA64{gray, gray, gray, 0xffff}
}

func (p *Gray16) Gray16At(x, y int) color.Gray16 {
	if !(Point{x, y}.In(p.Rect)) {
		return color.Gray16{}
	}
	i := p.PixOffset(x, y)
	return color.Gray16{uint16(p.Pix[i+0])<<8 | uint16(p.Pix[i+1])}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *Gray16) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*2
}

func (p *Gray16) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	c1 := color.Gray16Model.Convert(c).(color.Gray16)
	p.Pix[i+0] = uint8(c1.Y >> 8)
	p.Pix[i+1] = uint8(c1.Y)
}

func (p *Gray16) SetRGBA64(x, y int, c color.RGBA64) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	// This formula is the same as in color.gray16Model.
	gray := (19595*uint32(c.R) + 38470*uint32(c.G) + 7471*uint32(c.B) + 1<<15) >> 16
	i := p.PixOffset(x, y)
	p.Pix[i+0] = uint8(gray >> 8)
	p.Pix[i+1] = uint8(gray)
}

func (p *Gray16) SetGray16(x, y int, c color.Gray16) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i+0] = uint8(c.Y >> 8)
	p.Pix[i+1] = uint8(c.Y)
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *Gray16) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &Gray16{}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &Gray16{
		Pix:    p.Pix[i:],
		Stride: p.Stride,
		Rect:   r,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *Gray16) Opaque() bool {
	return true
}

// NewGray16 returns a new [Gray16] image with the given bounds.
func NewGray16(r Rectangle) *Gray16 {
	return &Gray16{
		Pix:    make([]uint8, pixelBufferLength(2, r, "Gray16")),
		Stride: 2 * r.Dx(),
		Rect:   r,
	}
}

// CMYK is an in-memory image whose At method returns [color.CMYK] values.
type CMYK struct {
	// Pix holds the image's pixels, in C, M, Y, K order. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*4].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect Rectangle
}

func (p *CMYK) ColorModel() color.Model { return color.CMYKModel }

func (p *CMYK) Bounds() Rectangle { return p.Rect }

func (p *CMYK) At(x, y int) color.Color {
	return p.CMYKAt(x, y)
}

func (p *CMYK) RGBA64At(x, y int) color.RGBA64 {
	r, g, b, a := p.CMYKAt(x, y).RGBA()
	return color.RGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
}

func (p *CMYK) CMYKAt(x, y int) color.CMYK {
	if !(Point{x, y}.In(p.Rect)) {
		return color.CMYK{}
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	return color.CMYK{s[0], s[1], s[2], s[3]}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *CMYK) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*4
}

func (p *CMYK) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	c1 := color.CMYKModel.Convert(c).(color.CMYK)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = c1.C
	s[1] = c1.M
	s[2] = c1.Y
	s[3] = c1.K
}

func (p *CMYK) SetRGBA64(x, y int, c color.RGBA64) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	cc, mm, yy, kk := color.RGBToCMYK(uint8(c.R>>8), uint8(c.G>>8), uint8(c.B>>8))
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = cc
	s[1] = mm
	s[2] = yy
	s[3] = kk
}

func (p *CMYK) SetCMYK(x, y int, c color.CMYK) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	s := p.Pix[i : i+4 : i+4] // Small cap improves performance, see https://golang.org/issue/27857
	s[0] = c.C
	s[1] = c.M
	s[2] = c.Y
	s[3] = c.K
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *CMYK) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &CMYK{}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &CMYK{
		Pix:    p.Pix[i:],
		Stride: p.Stride,
		Rect:   r,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *CMYK) Opaque() bool {
	return true
}

// NewCMYK returns a new CMYK image with the given bounds.
func NewCMYK(r Rectangle) *CMYK {
	return &CMYK{
		Pix:    make([]uint8, pixelBufferLength(4, r, "CMYK")),
		Stride: 4 * r.Dx(),
		Rect:   r,
	}
}

// Paletted is an in-memory image of uint8 indices into a given palette.
type Paletted struct {
	// Pix holds the image's pixels, as palette indices. The pixel at
	// (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*1].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect Rectangle
	// Palette is the image's palette.
	Palette color.Palette
}

func (p *Paletted) ColorModel() color.Model { return p.Palette }

func (p *Paletted) Bounds() Rectangle { return p.Rect }

func (p *Paletted) At(x, y int) color.Color {
	if len(p.Palette) == 0 {
		return nil
	}
	if !(Point{x, y}.In(p.Rect)) {
		return p.Palette[0]
	}
	i := p.PixOffset(x, y)
	return p.Palette[p.Pix[i]]
}

func (p *Paletted) RGBA64At(x, y int) color.RGBA64 {
	if len(p.Palette) == 0 {
		return color.RGBA64{}
	}
	c := color.Color(nil)
	if !(Point{x, y}.In(p.Rect)) {
		c = p.Palette[0]
	} else {
		i := p.PixOffset(x, y)
		c = p.Palette[p.Pix[i]]
	}
	r, g, b, a := c.RGBA()
	return color.RGBA64{
		uint16(r),
		uint16(g),
		uint16(b),
		uint16(a),
	}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *Paletted) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*1
}

func (p *Paletted) Set(x, y int, c color.Color) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i] = uint8(p.Palette.Index(c))
}

func (p *Paletted) SetRGBA64(x, y int, c color.RGBA64) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i] = uint8(p.Palette.Index(c))
}

func (p *Paletted) ColorIndexAt(x, y int) uint8 {
	if !(Point{x, y}.In(p.Rect)) {
		return 0
	}
	i := p.PixOffset(x, y)
	return p.Pix[i]
}

func (p *Paletted) SetColorIndex(x, y int, index uint8) {
	if !(Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i] = index
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *Paletted) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &Paletted{
			Palette: p.Palette,
		}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &Paletted{
		Pix:     p.Pix[i:],
		Stride:  p.Stride,
		Rect:    p.Rect.Intersect(r),
		Palette: p.Palette,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *Paletted) Opaque() bool {
	var present [256]bool
	i0, i1 := 0, p.Rect.Dx()
	for y := p.Rect.Min.Y; y < p.Rect.Max.Y; y++ {
		for _, c := range p.Pix[i0:i1] {
			present[c] = true
		}
		i0 += p.Stride
		i1 += p.Stride
	}
	for i, c := range p.Palette {
		if !present[i] {
			continue
		}
		_, _, _, a := c.RGBA()
		if a != 0xffff {
			return false
		}
	}
	return true
}

// NewPaletted returns a new [Paletted] image with the given width, height and
// palette.
func NewPaletted(r Rectangle, p color.Palette) *Paletted {
	return &Paletted{
		Pix:     make([]uint8, pixelBufferLength(1, r, "Paletted")),
		Stride:  1 * r.Dx(),
		Rect:    r,
		Palette: p,
	}
}

```

// === FILE: references/go/src/image/internal/imageutil/gen.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
)

var debug = flag.Bool("debug", false, "")

func main() {
	flag.Parse()

	w := new(bytes.Buffer)
	w.WriteString(pre)
	for _, sratio := range subsampleRatios {
		fmt.Fprintf(w, sratioCase, sratio, sratioLines[sratio])
	}
	w.WriteString(post)

	if *debug {
		os.Stdout.Write(w.Bytes())
		return
	}
	out, err := format.Source(w.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("impl.go", out, 0660); err != nil {
		log.Fatal(err)
	}
}

const pre = `// Code generated by go run gen.go; DO NOT EDIT.

package imageutil

import (
	"image"
)

// DrawYCbCr draws the YCbCr source image on the RGBA destination image with
// r.Min in dst aligned with sp in src. It reports whether the draw was
// successful. If it returns false, no dst pixels were changed.
//
// This function assumes that r is entirely within dst's bounds and the
// translation of r from dst coordinate space to src coordinate space is
// entirely within src's bounds.
func DrawYCbCr(dst *image.RGBA, r image.Rectangle, src *image.YCbCr, sp image.Point) (ok bool) {
	// This function exists in the image/internal/imageutil package because it
	// is needed by both the image/draw and image/jpeg packages, but it doesn't
	// seem right for one of those two to depend on the other.
	//
	// Another option is to have this code be exported in the image package,
	// but we'd need to make sure we're totally happy with the API (for the
	// rest of Go 1 compatibility), and decide if we want to have a more
	// general purpose DrawToRGBA method for other image types. One possibility
	// is:
	//
	// func (src *YCbCr) CopyToRGBA(dst *RGBA, dr, sr Rectangle) (effectiveDr, effectiveSr Rectangle)
	//
	// in the spirit of the built-in copy function for 1-dimensional slices,
	// that also allowed a CopyFromRGBA method if needed.

	x0 := (r.Min.X - dst.Rect.Min.X) * 4
	x1 := (r.Max.X - dst.Rect.Min.X) * 4
	y0 := r.Min.Y - dst.Rect.Min.Y
	y1 := r.Max.Y - dst.Rect.Min.Y
	switch src.SubsampleRatio {
`

const post = `
	default:
		return false
	}
	return true
}
`

const sratioCase = `
	case image.YCbCrSubsampleRatio%s:
		for y, sy := y0, sp.Y; y != y1; y, sy = y+1, sy+1 {
			dpix := dst.Pix[y*dst.Stride:]
			yi := (sy-src.Rect.Min.Y)*src.YStride + (sp.X - src.Rect.Min.X)
			%s

				// This is an inline version of image/color/ycbcr.go's func YCbCrToRGB.
				yy1 := int32(src.Y[yi]) * 0x10101
				cb1 := int32(src.Cb[ci]) - 128
				cr1 := int32(src.Cr[ci]) - 128

				// The bit twiddling below is equivalent to
				//
				// r := (yy1 + 91881*cr1) >> 16
				// if r < 0 {
				//     r = 0
				// } else if r > 0xff {
				//     r = ^int32(0)
				// }
				//
				// but uses fewer branches and is faster.
				// Note that the uint8 type conversion in the return
				// statement will convert ^int32(0) to 0xff.
				// The code below to compute g and b uses a similar pattern.
				r := yy1 + 91881*cr1
				if uint32(r)&0xff000000 == 0 {
					r >>= 16
				} else {
					r = ^(r >> 31)
				}

				g := yy1 - 22554*cb1 - 46802*cr1
				if uint32(g)&0xff000000 == 0 {
					g >>= 16
				} else {
					g = ^(g >> 31)
				}

				b := yy1 + 116130*cb1
				if uint32(b)&0xff000000 == 0 {
					b >>= 16
				} else {
					b = ^(b >> 31)
				}


				// use a temp slice to hint to the compiler that a single bounds check suffices
				rgba := dpix[x : x+4 : len(dpix)]
				rgba[0] = uint8(r)
				rgba[1] = uint8(g)
				rgba[2] = uint8(b)
				rgba[3] = 255
			}
		}
`

var subsampleRatios = []string{
	"444",
	"422",
	"420",
	"440",
}

var sratioLines = map[string]string{
	"444": `
		ci := (sy-src.Rect.Min.Y)*src.CStride + (sp.X - src.Rect.Min.X)
		for x := x0; x != x1; x, yi, ci = x+4, yi+1, ci+1 {
	`,
	"422": `
		ciBase := (sy-src.Rect.Min.Y)*src.CStride - src.Rect.Min.X/2
		for x, sx := x0, sp.X; x != x1; x, sx, yi = x+4, sx+1, yi+1 {
			ci := ciBase + sx/2
	`,
	"420": `
		ciBase := (sy/2-src.Rect.Min.Y/2)*src.CStride - src.Rect.Min.X/2
		for x, sx := x0, sp.X; x != x1; x, sx, yi = x+4, sx+1, yi+1 {
			ci := ciBase + sx/2
	`,
	"440": `
		ci := (sy/2-src.Rect.Min.Y/2)*src.CStride + (sp.X - src.Rect.Min.X)
		for x := x0; x != x1; x, yi, ci = x+4, yi+1, ci+1 {
	`,
}

```

// === FILE: references/go/src/image/internal/imageutil/imageutil.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run gen.go

// Package imageutil contains code shared by image-related packages.
package imageutil

```

// === FILE: references/go/src/image/internal/imageutil/impl.go ===
```go
// Code generated by go run gen.go; DO NOT EDIT.

package imageutil

import (
	"image"
)

// DrawYCbCr draws the YCbCr source image on the RGBA destination image with
// r.Min in dst aligned with sp in src. It reports whether the draw was
// successful. If it returns false, no dst pixels were changed.
//
// This function assumes that r is entirely within dst's bounds and the
// translation of r from dst coordinate space to src coordinate space is
// entirely within src's bounds.
func DrawYCbCr(dst *image.RGBA, r image.Rectangle, src *image.YCbCr, sp image.Point) (ok bool) {
	// This function exists in the image/internal/imageutil package because it
	// is needed by both the image/draw and image/jpeg packages, but it doesn't
	// seem right for one of those two to depend on the other.
	//
	// Another option is to have this code be exported in the image package,
	// but we'd need to make sure we're totally happy with the API (for the
	// rest of Go 1 compatibility), and decide if we want to have a more
	// general purpose DrawToRGBA method for other image types. One possibility
	// is:
	//
	// func (src *YCbCr) CopyToRGBA(dst *RGBA, dr, sr Rectangle) (effectiveDr, effectiveSr Rectangle)
	//
	// in the spirit of the built-in copy function for 1-dimensional slices,
	// that also allowed a CopyFromRGBA method if needed.

	x0 := (r.Min.X - dst.Rect.Min.X) * 4
	x1 := (r.Max.X - dst.Rect.Min.X) * 4
	y0 := r.Min.Y - dst.Rect.Min.Y
	y1 := r.Max.Y - dst.Rect.Min.Y
	switch src.SubsampleRatio {

	case image.YCbCrSubsampleRatio444:
		for y, sy := y0, sp.Y; y != y1; y, sy = y+1, sy+1 {
			dpix := dst.Pix[y*dst.Stride:]
			yi := (sy-src.Rect.Min.Y)*src.YStride + (sp.X - src.Rect.Min.X)

			ci := (sy-src.Rect.Min.Y)*src.CStride + (sp.X - src.Rect.Min.X)
			for x := x0; x != x1; x, yi, ci = x+4, yi+1, ci+1 {

				// This is an inline version of image/color/ycbcr.go's func YCbCrToRGB.
				yy1 := int32(src.Y[yi]) * 0x10101
				cb1 := int32(src.Cb[ci]) - 128
				cr1 := int32(src.Cr[ci]) - 128

				// The bit twiddling below is equivalent to
				//
				// r := (yy1 + 91881*cr1) >> 16
				// if r < 0 {
				//     r = 0
				// } else if r > 0xff {
				//     r = ^int32(0)
				// }
				//
				// but uses fewer branches and is faster.
				// Note that the uint8 type conversion in the return
				// statement will convert ^int32(0) to 0xff.
				// The code below to compute g and b uses a similar pattern.
				r := yy1 + 91881*cr1
				if uint32(r)&0xff000000 == 0 {
					r >>= 16
				} else {
					r = ^(r >> 31)
				}

				g := yy1 - 22554*cb1 - 46802*cr1
				if uint32(g)&0xff000000 == 0 {
					g >>= 16
				} else {
					g = ^(g >> 31)
				}

				b := yy1 + 116130*cb1
				if uint32(b)&0xff000000 == 0 {
					b >>= 16
				} else {
					b = ^(b >> 31)
				}

				// use a temp slice to hint to the compiler that a single bounds check suffices
				rgba := dpix[x : x+4 : len(dpix)]
				rgba[0] = uint8(r)
				rgba[1] = uint8(g)
				rgba[2] = uint8(b)
				rgba[3] = 255
			}
		}

	case image.YCbCrSubsampleRatio422:
		for y, sy := y0, sp.Y; y != y1; y, sy = y+1, sy+1 {
			dpix := dst.Pix[y*dst.Stride:]
			yi := (sy-src.Rect.Min.Y)*src.YStride + (sp.X - src.Rect.Min.X)

			ciBase := (sy-src.Rect.Min.Y)*src.CStride - src.Rect.Min.X/2
			for x, sx := x0, sp.X; x != x1; x, sx, yi = x+4, sx+1, yi+1 {
				ci := ciBase + sx/2

				// This is an inline version of image/color/ycbcr.go's func YCbCrToRGB.
				yy1 := int32(src.Y[yi]) * 0x10101
				cb1 := int32(src.Cb[ci]) - 128
				cr1 := int32(src.Cr[ci]) - 128

				// The bit twiddling below is equivalent to
				//
				// r := (yy1 + 91881*cr1) >> 16
				// if r < 0 {
				//     r = 0
				// } else if r > 0xff {
				//     r = ^int32(0)
				// }
				//
				// but uses fewer branches and is faster.
				// Note that the uint8 type conversion in the return
				// statement will convert ^int32(0) to 0xff.
				// The code below to compute g and b uses a similar pattern.
				r := yy1 + 91881*cr1
				if uint32(r)&0xff000000 == 0 {
					r >>= 16
				} else {
					r = ^(r >> 31)
				}

				g := yy1 - 22554*cb1 - 46802*cr1
				if uint32(g)&0xff000000 == 0 {
					g >>= 16
				} else {
					g = ^(g >> 31)
				}

				b := yy1 + 116130*cb1
				if uint32(b)&0xff000000 == 0 {
					b >>= 16
				} else {
					b = ^(b >> 31)
				}

				// use a temp slice to hint to the compiler that a single bounds check suffices
				rgba := dpix[x : x+4 : len(dpix)]
				rgba[0] = uint8(r)
				rgba[1] = uint8(g)
				rgba[2] = uint8(b)
				rgba[3] = 255
			}
		}

	case image.YCbCrSubsampleRatio420:
		for y, sy := y0, sp.Y; y != y1; y, sy = y+1, sy+1 {
			dpix := dst.Pix[y*dst.Stride:]
			yi := (sy-src.Rect.Min.Y)*src.YStride + (sp.X - src.Rect.Min.X)

			ciBase := (sy/2-src.Rect.Min.Y/2)*src.CStride - src.Rect.Min.X/2
			for x, sx := x0, sp.X; x != x1; x, sx, yi = x+4, sx+1, yi+1 {
				ci := ciBase + sx/2

				// This is an inline version of image/color/ycbcr.go's func YCbCrToRGB.
				yy1 := int32(src.Y[yi]) * 0x10101
				cb1 := int32(src.Cb[ci]) - 128
				cr1 := int32(src.Cr[ci]) - 128

				// The bit twiddling below is equivalent to
				//
				// r := (yy1 + 91881*cr1) >> 16
				// if r < 0 {
				//     r = 0
				// } else if r > 0xff {
				//     r = ^int32(0)
				// }
				//
				// but uses fewer branches and is faster.
				// Note that the uint8 type conversion in the return
				// statement will convert ^int32(0) to 0xff.
				// The code below to compute g and b uses a similar pattern.
				r := yy1 + 91881*cr1
				if uint32(r)&0xff000000 == 0 {
					r >>= 16
				} else {
					r = ^(r >> 31)
				}

				g := yy1 - 22554*cb1 - 46802*cr1
				if uint32(g)&0xff000000 == 0 {
					g >>= 16
				} else {
					g = ^(g >> 31)
				}

				b := yy1 + 116130*cb1
				if uint32(b)&0xff000000 == 0 {
					b >>= 16
				} else {
					b = ^(b >> 31)
				}

				// use a temp slice to hint to the compiler that a single bounds check suffices
				rgba := dpix[x : x+4 : len(dpix)]
				rgba[0] = uint8(r)
				rgba[1] = uint8(g)
				rgba[2] = uint8(b)
				rgba[3] = 255
			}
		}

	case image.YCbCrSubsampleRatio440:
		for y, sy := y0, sp.Y; y != y1; y, sy = y+1, sy+1 {
			dpix := dst.Pix[y*dst.Stride:]
			yi := (sy-src.Rect.Min.Y)*src.YStride + (sp.X - src.Rect.Min.X)

			ci := (sy/2-src.Rect.Min.Y/2)*src.CStride + (sp.X - src.Rect.Min.X)
			for x := x0; x != x1; x, yi, ci = x+4, yi+1, ci+1 {

				// This is an inline version of image/color/ycbcr.go's func YCbCrToRGB.
				yy1 := int32(src.Y[yi]) * 0x10101
				cb1 := int32(src.Cb[ci]) - 128
				cr1 := int32(src.Cr[ci]) - 128

				// The bit twiddling below is equivalent to
				//
				// r := (yy1 + 91881*cr1) >> 16
				// if r < 0 {
				//     r = 0
				// } else if r > 0xff {
				//     r = ^int32(0)
				// }
				//
				// but uses fewer branches and is faster.
				// Note that the uint8 type conversion in the return
				// statement will convert ^int32(0) to 0xff.
				// The code below to compute g and b uses a similar pattern.
				r := yy1 + 91881*cr1
				if uint32(r)&0xff000000 == 0 {
					r >>= 16
				} else {
					r = ^(r >> 31)
				}

				g := yy1 - 22554*cb1 - 46802*cr1
				if uint32(g)&0xff000000 == 0 {
					g >>= 16
				} else {
					g = ^(g >> 31)
				}

				b := yy1 + 116130*cb1
				if uint32(b)&0xff000000 == 0 {
					b >>= 16
				} else {
					b = ^(b >> 31)
				}

				// use a temp slice to hint to the compiler that a single bounds check suffices
				rgba := dpix[x : x+4 : len(dpix)]
				rgba[0] = uint8(r)
				rgba[1] = uint8(g)
				rgba[2] = uint8(b)
				rgba[3] = 255
			}
		}

	default:
		return false
	}
	return true
}

```

// === FILE: references/go/src/image/jpeg/dct.go ===
```go
// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jpeg

// Discrete Cosine Transformation (DCT) implementations using the algorithm from
// Christoph Loeffler, Adriaan Lightenberg, and George S. Mostchytz,
// “Practical Fast 1-D DCT Algorithms with 11 Multiplications,” ICASSP 1989.
// https://ieeexplore.ieee.org/document/266596
//
// Since the paper is paywalled, the rest of this comment gives a summary.
//
// A 1-dimensional forward DCT (1D FDCT) takes as input 8 values x0..x7
// and transforms them in place into the result values.
//
// The mathematical definition of the N-point 1D FDCT is:
//
//	X[k] = α_k Σ_n x[n] * cos (2n+1)*k*π/2N
//
// where α₀ = √2 and α_k = 1 for k > 0.
//
// For our purposes, N=8, so the angles end up being multiples of π/16.
// The most direct implementation of this definition would require 64 multiplications.
//
// Loeffler's paper presents a more efficient computation that requires only
// 11 multiplications and works in terms of three basic operations:
//
//  - A “butterfly” x0, x1 = x0+x1, x0-x1.
//    The inverse is x0, x1 = (x0+x1)/2, (x0-x1)/2.
//
//  - A scaling of x0 by k: x0 *= k. The inverse is scaling by 1/k.
//
//  - A rotation of x0, x1 by θ, defined as:
//    x0, x1 = x0 cos θ + x1 sin θ, -x0 sin θ + x1 cos θ.
//    The inverse is rotation by -θ.
//
// The algorithm proceeds in four stages:
//
// Stage 1:
//  - butterfly x0, x7; x1, x6; x2, x5; x3, x4.
//
// Stage 2:
//  - butterfly x0, x3; x1, x2
//  - rotate x4, x7 by 3π/16
//  - rotate x5, x6 by π/16.
//
// Stage 3:
//  - butterfly x0, x1; x4, x6; x7, x5
//  - rotate x2, x3 by 6π/16 and scale by √2.
//
// Stage 4:
//  - butterfly x7, x4
//  - scale x5, x6 by √2.
//
// Finally, the values are permuted. The permutation can be read as either:
//  - x0, x4, x2, x6, x7, x3, x5, x1 = x0, x1, x2, x3, x4, x5, x6, x7 (paper's form)
//  - x0, x1, x2, x3, x4, x5, x6, x7 = x0, x7, x2, x5, x1, x6, x3, x4 (sorted by LHS)
// The code below uses the second form to make it easier to merge adjacent stores.
// (Note that unlike in recursive FFT implementations, the permutation here is
// not always mapping indexes to their bit reversals.)
//
// As written above, the rotation requires four multiplications, but it can be
// reduced to three by refactoring (see [dctBox] below), and the scaling in
// stage 3 can be merged into the rotation constants, so the overall cost
// of a 1D FDCT is 11 multiplies.
//
// The 1D inverse DCT (IDCT) is the 1D FDCT run backward
// with all the basic operations inverted.

// dctBox implements a 3-multiply, 3-add rotation+scaling.
// Given x0, x1, k*cos θ, and k*sin θ, dctBox returns the
// rotated and scaled coordinates.
// (It is called dctBox because the rotate+scale operation
// is drawn as a box in Figures 1 and 2 in the paper.)
func dctBox(x0, x1, kcos, ksin int32) (y0, y1 int32) {
	// y0 = x0*kcos + x1*ksin
	// y1 = -x0*ksin + x1*kcos
	ksum := kcos * (x0 + x1)
	y0 = ksum + (ksin-kcos)*x1
	y1 = ksum - (kcos+ksin)*x0
	return y0, y1
}

// A block is an 8x8 input to a 2D DCT (either the FDCT or IDCT).
// The input is actually only 8x8 uint8 values, and the outputs are 8x8 int16,
// but it is convenient to use int32s for intermediate storage,
// so we define only a single block type of [8*8]int32.
//
// A 2D DCT is implemented as 1D DCTs over the rows and columns.
//
// dct_test.go defines a String method for nice printing in tests.
type block [blockSize]int32

const blockSize = 8 * 8

// Note on Numerical Precision
//
// The inputs to both the FDCT and IDCT are uint8 values stored in a block,
// and the outputs are int16s in the same block, but the overall operation
// uses int32 values as fixed-point intermediate values.
// In the code comments below, the notation “QN.M” refers to a
// signed value of 1+N+M significant bits, one of which is the sign bit,
// and M of which hold fractional (sub-integer) precision.
// For example, 255 as a Q8.0 value is stored as int32(255),
// while 255 as a Q8.1 value is stored as int32(510),
// and 255.5 as a Q8.1 value is int32(511).
// The notation UQN.M refers to an unsigned value of N+M significant bits.
// See https://en.wikipedia.org/wiki/Q_(number_format) for more.
//
// In general we only need to keep about 16 significant bits, but it is more
// efficient and somewhat more precise to let unnecessary fractional bits
// accumulate and shift them away in bulk rather than after every operation.
// As such, it is important to keep track of the number of fractional bits
// in each variable at different points in the code, to avoid mistakes like
// adding numbers with different fractional precisions, as well as to keep
// track of the total number of bits, to avoid overflow. A comment like:
//
//	// x[123] now Q8.2.
//
// means that x1, x2, and x3 are all Q8.2 (11-bit) values.
// Keeping extra precision bits also reduces the size of the errors introduced
// by using right shift to approximate rounded division.

// Constants needed for the implementation.
// These are all 60-bit precision fixed-point constants.
// The function c(val, b) rounds the constant to b bits.
// c is simple enough that calls to it with constant args
// are inlined and constant-propagated down to an inline constant.
// Each constant is commented with its Ivy definition (see robpike.io/ivy),
// using this scaling helper function:
//
//	op fix x = floor 0.5 + x * 2**60
const (
	cos1          = 1130768441178740757 // fix cos 1*pi/16
	sin1          = 224923827593068887  // fix sin 1*pi/16
	cos3          = 958619196450722178  // fix cos 3*pi/16
	sin3          = 640528868967736374  // fix sin 3*pi/16
	sqrt2         = 1630477228166597777 // fix sqrt 2
	sqrt2_cos6    = 623956622067911264  // fix (sqrt 2)*cos 6*pi/16
	sqrt2_sin6    = 1506364539328854985 // fix (sqrt 2)*sin 6*pi/16
	sqrt2inv      = 815238614083298888  // fix 1/sqrt 2
	sqrt2inv_cos6 = 311978311033955632  // fix (1/sqrt 2)*cos 6*pi/16
	sqrt2inv_sin6 = 753182269664427492  // fix (1/sqrt 2)*sin 6*pi/16
)

func c(x uint64, bits int) int32 {
	return int32((x + (1 << (59 - bits))) >> (60 - bits))
}

// fdct implements the forward DCT.
// Inputs are UQ8.0; outputs are Q13.0.
func fdct(b *block) {
	fdctCols(b)
	fdctRows(b)
}

// fdctCols applies the 1D DCT to the columns of b.
// Inputs are UQ8.0 in [0,255] but interpreted as [-128,127].
// Outputs are Q10.18.
func fdctCols(b *block) {
	for i := range 8 {
		x0 := b[0*8+i]
		x1 := b[1*8+i]
		x2 := b[2*8+i]
		x3 := b[3*8+i]
		x4 := b[4*8+i]
		x5 := b[5*8+i]
		x6 := b[6*8+i]
		x7 := b[7*8+i]

		// x[01234567] are UQ8.0 in [0,255].

		// Stage 1: four butterflies.
		// In general a butterfly of QN.M inputs produces Q(N+1).M outputs.
		// A butterfly of UQN.M inputs produces a UQ(N+1).M sum and a QN.M difference.

		x0, x7 = x0+x7, x0-x7
		x1, x6 = x1+x6, x1-x6
		x2, x5 = x2+x5, x2-x5
		x3, x4 = x3+x4, x3-x4
		// x[0123] now UQ9.0 in [0, 510].
		// x[4567] now Q8.0 in [-255,255].

		// Stage 2: two boxes and two butterflies.
		// A box on QN.M inputs with B-bit constants
		// produces Q(N+1).(M+B) outputs.
		// (The +1 is from the addition.)

		x4, x7 = dctBox(x4, x7, c(cos3, 18), c(sin3, 18))
		x5, x6 = dctBox(x5, x6, c(cos1, 18), c(sin1, 18))
		// x[47] now Q9.18 in [-354, 354].
		// x[56] now Q9.18 in [-300, 300].

		x0, x3 = x0+x3, x0-x3
		x1, x2 = x1+x2, x1-x2
		// x[01] now UQ10.0 in [0, 1020].
		// x[23] now Q9.0 in [-510, 510].

		// Stage 3: one box and three butterflies.

		x2, x3 = dctBox(x2, x3, c(sqrt2_cos6, 18), c(sqrt2_sin6, 18))
		// x[23] now Q10.18 in [-943, 943].

		x0, x1 = x0+x1, x0-x1
		// x0 now UQ11.0 in [0, 2040].
		// x1 now Q10.0 in [-1020, 1020].

		// Store x0, x1, x2, x3 to their permuted targets.
		// The original +128 in every input value
		// has cancelled out except in the “DC signal” x0.
		// Subtracting 128*8 here is equivalent to subtracting 128
		// from every input before we started, but cheaper.
		// It also converts x0 from UQ11.18 to Q10.18.
		b[0*8+i] = (x0 - 128*8) << 18
		b[4*8+i] = x1 << 18
		b[2*8+i] = x2
		b[6*8+i] = x3

		x4, x6 = x4+x6, x4-x6
		x7, x5 = x7+x5, x7-x5
		// x[4567] now Q10.18 in [-654, 654].

		// Stage 4: two √2 scalings and one butterfly.

		x5 = (x5 >> 12) * c(sqrt2, 12)
		x6 = (x6 >> 12) * c(sqrt2, 12)
		// x[56] still Q10.18 in [-925, 925] (= 654√2).
		x7, x4 = x7+x4, x7-x4
		// x[47] still Q10.18 in [-925, 925] (not Q11.18!).
		// This is not obvious at all! See “Note on 925” below.

		// Store x4 x5 x6 x7 to their permuted targets.
		b[1*8+i] = x7
		b[3*8+i] = x5
		b[5*8+i] = x6
		b[7*8+i] = x4
	}
}

// fdctRows applies the 1D DCT to the rows of b.
// Inputs are Q10.18; outputs are Q13.0.
func fdctRows(b *block) {
	for i := range 8 {
		x := b[8*i : 8*i+8 : 8*i+8]
		x0 := x[0]
		x1 := x[1]
		x2 := x[2]
		x3 := x[3]
		x4 := x[4]
		x5 := x[5]
		x6 := x[6]
		x7 := x[7]

		// x[01234567] are Q10.18 [-1020, 1020].

		// Stage 1: four butterflies.

		x0, x7 = x0+x7, x0-x7
		x1, x6 = x1+x6, x1-x6
		x2, x5 = x2+x5, x2-x5
		x3, x4 = x3+x4, x3-x4
		// x[01234567] now Q11.18 in [-2040, 2040].

		// Stage 2: two boxes and two butterflies.

		x4, x7 = dctBox(x4>>14, x7>>14, c(cos3, 14), c(sin3, 14))
		x5, x6 = dctBox(x5>>14, x6>>14, c(cos1, 14), c(sin1, 14))
		// x[47] now Q12.18 in [-2830, 2830].
		// x[56] now Q12.18 in [-2400, 2400].
		x0, x3 = x0+x3, x0-x3
		x1, x2 = x1+x2, x1-x2
		// x[01234567] now Q12.18 in [-4080, 4080].

		// Stage 3: one box and three butterflies.

		x2, x3 = dctBox(x2>>14, x3>>14, c(sqrt2_cos6, 14), c(sqrt2_sin6, 14))
		// x[23] now Q13.18 in [-7539, 7539].
		x0, x1 = x0+x1, x0-x1
		// x[01] now Q13.18 in [-8160, 8160].
		x4, x6 = x4+x6, x4-x6
		x7, x5 = x7+x5, x7-x5
		// x[4567] now Q13.18 in [-5230, 5230].

		// Stage 4: two √2 scalings and one butterfly.

		x5 = (x5 >> 14) * c(sqrt2, 14)
		x6 = (x6 >> 14) * c(sqrt2, 14)
		// x[56] still Q13.18 in [-7397, 7397] (= 5230√2).
		x7, x4 = x7+x4, x7-x4
		// x[47] still Q13.18 in [-7395, 7395] (= 2040*3.6246).
		// See “Note on 925” below.

		// Cut from Q13.18 to Q13.0.
		x0 = (x0 + 1<<17) >> 18
		x1 = (x1 + 1<<17) >> 18
		x2 = (x2 + 1<<17) >> 18
		x3 = (x3 + 1<<17) >> 18
		x4 = (x4 + 1<<17) >> 18
		x5 = (x5 + 1<<17) >> 18
		x6 = (x6 + 1<<17) >> 18
		x7 = (x7 + 1<<17) >> 18

		// Note: Unlike in fdctCols, saved all stores for the end
		// because they are adjacent memory locations and some systems
		// can use multiword stores.
		x[0] = x0
		x[1] = x7
		x[2] = x2
		x[3] = x5
		x[4] = x1
		x[5] = x6
		x[6] = x3
		x[7] = x4
	}
}

// “Note on 925”, deferred from above to avoid interrupting code.
//
// In fdctCols, heading into stage 2, the values x4, x5, x6, x7 are in [-255, 255].
// Let's call those specific values b4, b5, b6, b7, and trace how x[4567] evolve:
//
// Stage 2:
//	x4 = b4*cos3 + b7*sin3
//	x7 = -b4*sin3 + b7*cos3
//	x5 = b5*cos1 + b6*sin1
//	x6 = -b5*sin1 + b6*cos1
//
// Stage 3:
//
//	x4 = x4+x6 =  b4*cos3 + b7*sin3 - b5*sin1 + b6*cos1
//	x6 = x4-x6 =  b4*cos3 + b7*sin3 + b5*sin1 - b6*cos1
//	x7 = x7+x5 = -b4*sin3 + b7*cos3 + b5*cos1 + b6*sin1
//	x5 = x7-x5 = -b4*sin3 + b7*cos3 - b5*cos1 - b6*sin1
//
// Stage 4:
//
//	x7 = x7+x4 = -b4*sin3 + b7*cos3 + b5*cos1 + b6*sin1 + b4*cos3 + b7*sin3 - b5*sin1 + b6*cos1
//	   = b4*(cos3-sin3) + b5*(cos1-sin1) + b6*(cos1+sin1) + b7*(cos3+sin3)
//	   < 255*(0.2759 + 0.7857 + 1.1759 + 1.3871) = 255*3.6246 < 925.
//
//	x4 = x7-x4 = -b4*sin3 + b7*cos3 + b5*cos1 + b6*sin1 - b4*cos3 - b7*sin3 + b5*sin1 - b6*cos1
//	   = -b4*(cos3+sin3) + b5*(cos1+sin1) + b6*(sin1-cos1) + b7*(cos3-sin3)
//	   < same 925.
//
// The fact that x5, x6 are also at most 925 is not a coincidence: we are computing
// the same kinds of numbers for all four, just with different paths to them.
//
// In fdctRows, the same analysis applies, but the initial values are
// in [-2040, 2040] instead of [-255, 255], so the bound is 2040*3.6246 < 7395.

// idct implements the inverse DCT.
// Inputs are UQ8.0; outputs are Q10.3.
func idct(b *block) {
	// A 2D IDCT is a 1D IDCT on rows followed by columns.
	idctRows(b)
	idctCols(b)
}

// idctRows applies the 1D IDCT to the rows of b.
// Inputs are UQ8.0; outputs are Q9.20.
func idctRows(b *block) {
	for i := range 8 {
		x := b[8*i : 8*i+8 : 8*i+8]
		x0 := x[0]
		x7 := x[1]
		x2 := x[2]
		x5 := x[3]
		x1 := x[4]
		x6 := x[5]
		x3 := x[6]
		x4 := x[7]

		// Run FDCT backward.
		// Independent operations have been reordered somewhat
		// to make precision tracking easier.
		//
		// Note that “x0, x1 = x0+x1, x0-x1” is now a reverse butterfly
		// and carries with it an implicit divide by two: the extra bit
		// is added to the precision, not the value size.

		// x[01234567] are UQ8.0 in [0, 255].

		// Stages 4, 3, 2: x0, x1, x2, x3.

		x0 <<= 17
		x1 <<= 17
		// x0, x1 now UQ8.17.
		x0, x1 = x0+x1, x0-x1
		// x0 now UQ8.18 in [0, 255].
		// x1 now Q7.18 in [-127½, 127½].

		// Note: (1/sqrt 2)*((cos 6*pi/16)+(sin 6*pi/16)) < 0.924, so no new high bit.
		x2, x3 = dctBox(x2, x3, c(sqrt2inv_cos6, 18), -c(sqrt2inv_sin6, 18))
		// x[23] now Q8.18 in [-236, 236].
		x1, x2 = x1+x2, x1-x2
		x0, x3 = x0+x3, x0-x3
		// x[0123] now Q8.19 in [-246, 246].

		// Stages 4, 3, 2: x4, x5, x6, x7.

		x4 <<= 7
		x7 <<= 7
		// x[47] now UQ8.7
		x7, x4 = x7+x4, x7-x4
		// x7 now UQ8.8 in [0, 255].
		// x4 now Q7.8 in [-127½, 127½].

		x6 = x6 * c(sqrt2inv, 8)
		x5 = x5 * c(sqrt2inv, 8)
		// x[56] now UQ8.8 in [0, 181].
		// Note that 1/√2 has five 0s in its binary representation after
		// the 8th bit, so this multipliy is actually producing 12 bits of precision.

		x7, x5 = x7+x5, x7-x5
		x4, x6 = x4+x6, x4-x6
		// x[4567] now Q8.9 in [-218, 218].

		x4, x7 = dctBox(x4>>2, x7>>2, c(cos3, 12), -c(sin3, 12))
		x5, x6 = dctBox(x5>>2, x6>>2, c(cos1, 12), -c(sin1, 12))
		// x[4567] now Q9.19 in [-303, 303].

		// Stage 1.

		x0, x7 = x0+x7, x0-x7
		x1, x6 = x1+x6, x1-x6
		x2, x5 = x2+x5, x2-x5
		x3, x4 = x3+x4, x3-x4
		// x[01234567] now Q9.20 in [-275, 275].

		// Note: we don't need all 20 bits of “precision”,
		// but it is faster to let idctCols shift it away as part
		// of other operations rather than downshift here.

		x[0] = x0
		x[1] = x1
		x[2] = x2
		x[3] = x3
		x[4] = x4
		x[5] = x5
		x[6] = x6
		x[7] = x7
	}
}

// idctCols applies the 1D IDCT to the columns of b.
// Inputs are Q9.20.
// Outputs are Q10.3. That is, the result is the IDCT*8.
func idctCols(b *block) {
	for i := range 8 {
		x0 := b[0*8+i]
		x7 := b[1*8+i]
		x2 := b[2*8+i]
		x5 := b[3*8+i]
		x1 := b[4*8+i]
		x6 := b[5*8+i]
		x3 := b[6*8+i]
		x4 := b[7*8+i]

		// x[012345678] are Q9.20.

		// Start by adding 0.5 to x0 (the incoming DC signal).
		// The butterflies will add it to all the other values,
		// and then the final shifts will round properly.
		x0 += 1 << 19

		// Stages 4, 3, 2: x0, x1, x2, x3.

		x0, x1 = (x0+x1)>>2, (x0-x1)>>2
		// x[01] now Q9.19.
		// Note: (1/sqrt 2)*((cos 6*pi/16)+(sin 6*pi/16)) < 1, so no new high bit.
		x2, x3 = dctBox(x2>>13, x3>>13, c(sqrt2inv_cos6, 12), -c(sqrt2inv_sin6, 12))
		// x[0123] now Q9.19.

		x1, x2 = x1+x2, x1-x2
		x0, x3 = x0+x3, x0-x3
		// x[0123] now Q9.20.

		// Stages 4, 3, 2: x4, x5, x6, x7.

		x7, x4 = x7+x4, x7-x4
		// x[47] now Q9.21.

		x5 = (x5 >> 13) * c(sqrt2inv, 14)
		x6 = (x6 >> 13) * c(sqrt2inv, 14)
		// x[56] now Q9.21.

		x7, x5 = x7+x5, x7-x5
		x4, x6 = x4+x6, x4-x6
		// x[4567] now Q9.22.

		x4, x7 = dctBox(x4>>14, x7>>14, c(cos3, 12), -c(sin3, 12))
		x5, x6 = dctBox(x5>>14, x6>>14, c(cos1, 12), -c(sin1, 12))
		// x[4567] now Q10.20.

		x0, x7 = x0+x7, x0-x7
		x1, x6 = x1+x6, x1-x6
		x2, x5 = x2+x5, x2-x5
		x3, x4 = x3+x4, x3-x4
		// x[01234567] now Q10.21.

		x0 >>= 18
		x1 >>= 18
		x2 >>= 18
		x3 >>= 18
		x4 >>= 18
		x5 >>= 18
		x6 >>= 18
		x7 >>= 18
		// x[01234567] now Q10.3.

		b[0*8+i] = x0
		b[1*8+i] = x1
		b[2*8+i] = x2
		b[3*8+i] = x3
		b[4*8+i] = x4
		b[5*8+i] = x5
		b[6*8+i] = x6
		b[7*8+i] = x7
	}
}

```

// === FILE: references/go/src/image/jpeg/huffman.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jpeg

import (
	"io"
)

// maxCodeLength is the maximum (inclusive) number of bits in a Huffman code.
const maxCodeLength = 16

// maxNCodes is the maximum (inclusive) number of codes in a Huffman tree.
const maxNCodes = 256

// lutSize is the log-2 size of the Huffman decoder's look-up table.
const lutSize = 8

// huffman is a Huffman decoder, specified in section C.
type huffman struct {
	// length is the number of codes in the tree.
	nCodes int32
	// lut is the look-up table for the next lutSize bits in the bit-stream.
	// The high 8 bits of the uint16 are the encoded value. The low 8 bits
	// are 1 plus the code length, or 0 if the value is too large to fit in
	// lutSize bits.
	lut [1 << lutSize]uint16
	// vals are the decoded values, sorted by their encoding.
	vals [maxNCodes]uint8
	// minCodes[i] is the minimum code of length i, or -1 if there are no
	// codes of that length.
	minCodes [maxCodeLength]int32
	// maxCodes[i] is the maximum code of length i, or -1 if there are no
	// codes of that length.
	maxCodes [maxCodeLength]int32
	// valsIndices[i] is the index into vals of minCodes[i].
	valsIndices [maxCodeLength]int32
}

// errShortHuffmanData means that an unexpected EOF occurred while decoding
// Huffman data.
var errShortHuffmanData = FormatError("short Huffman data")

// ensureNBits reads bytes from the byte buffer to ensure that d.bits.n is at
// least n. For best performance (avoiding function calls inside hot loops),
// the caller is the one responsible for first checking that d.bits.n < n.
func (d *decoder) ensureNBits(n int32) error {
	for {
		c, err := d.readByteStuffedByte()
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				return errShortHuffmanData
			}
			return err
		}
		d.bits.a = d.bits.a<<8 | uint32(c)
		d.bits.n += 8
		if d.bits.m == 0 {
			d.bits.m = 1 << 7
		} else {
			d.bits.m <<= 8
		}
		if d.bits.n >= n {
			break
		}
	}
	return nil
}

// receiveExtend is the composition of RECEIVE and EXTEND, specified in section
// F.2.2.1.
//
// It returns the signed integer that's encoded in t bits, where t < 16. The
// possible return values are:
//
//   - t ==  0:   0
//   - t ==  1:   -1, +1
//   - t ==  2:   -3, -2, +2, +3
//   - t ==  3:   -7, -6, -5, -4, +4, +5, +6, +7
//   - ...
//   - t == 15:   -32767, -32766, ..., -16384, +16384, ..., +32766, +32767
func (d *decoder) receiveExtend(t uint8) (int32, error) {
	if d.bits.n < int32(t) {
		if err := d.ensureNBits(int32(t)); err != nil {
			return 0, err
		}
	}
	d.bits.n -= int32(t)
	d.bits.m >>= t
	s := int32(1) << t
	x := int32(d.bits.a>>uint8(d.bits.n)) & (s - 1)

	// This adjustment, assuming two's complement, is a branchless equivalent of:
	//
	// if x < s>>1 {
	//   x += ((-1) << t) + 1
	// }
	//
	// sign is either -1 or 0, depending on whether x is in the low or high
	// half of the range 0 .. 1<<t.
	sign := (x >> (t - 1)) - 1
	x += sign & (((-1) << t) + 1)

	return x, nil
}

// processDHT processes a Define Huffman Table marker, and initializes a huffman
// struct from its contents. Specified in section B.2.4.2.
func (d *decoder) processDHT(n int) error {
	for n > 0 {
		if n < 17 {
			return FormatError("DHT has wrong length")
		}
		if err := d.readFull(d.tmp[:17]); err != nil {
			return err
		}
		tc := d.tmp[0] >> 4
		if tc > maxTc {
			return FormatError("bad Tc value")
		}
		th := d.tmp[0] & 0x0f
		// The baseline th <= 1 restriction is specified in table B.5.
		if th > maxTh || (d.baseline && th > 1) {
			return FormatError("bad Th value")
		}
		h := &d.huff[tc][th]

		// Read nCodes and h.vals (and derive h.nCodes).
		// nCodes[i] is the number of codes with code length i.
		// h.nCodes is the total number of codes.
		h.nCodes = 0
		var nCodes [maxCodeLength]int32
		for i := range nCodes {
			nCodes[i] = int32(d.tmp[i+1])
			h.nCodes += nCodes[i]
		}
		if h.nCodes == 0 {
			return FormatError("Huffman table has zero length")
		}
		if h.nCodes > maxNCodes {
			return FormatError("Huffman table has excessive length")
		}
		n -= int(h.nCodes) + 17
		if n < 0 {
			return FormatError("DHT has wrong length")
		}
		if err := d.readFull(h.vals[:h.nCodes]); err != nil {
			return err
		}

		// Derive the look-up table.
		clear(h.lut[:])
		var x, code uint32
		for i := uint32(0); i < lutSize; i++ {
			code <<= 1
			for j := int32(0); j < nCodes[i]; j++ {
				// The codeLength is 1+i, so shift code by 8-(1+i) to
				// calculate the high bits for every 8-bit sequence
				// whose codeLength's high bits matches code.
				// The high 8 bits of lutValue are the encoded value.
				// The low 8 bits are 1 plus the codeLength.
				base := uint8(code << (7 - i))
				lutValue := uint16(h.vals[x])<<8 | uint16(2+i)
				for k := uint8(0); k < 1<<(7-i); k++ {
					h.lut[base|k] = lutValue
				}
				code++
				x++
			}
		}

		// Derive minCodes, maxCodes, and valsIndices.
		var c, index int32
		for i, n := range nCodes {
			if n == 0 {
				h.minCodes[i] = -1
				h.maxCodes[i] = -1
				h.valsIndices[i] = -1
			} else {
				h.minCodes[i] = c
				h.maxCodes[i] = c + n - 1
				h.valsIndices[i] = index
				c += n
				index += n
			}
			c <<= 1
		}
	}
	return nil
}

// decodeHuffman returns the next Huffman-coded value from the bit-stream,
// decoded according to h.
func (d *decoder) decodeHuffman(h *huffman) (uint8, error) {
	if h.nCodes == 0 {
		return 0, FormatError("uninitialized Huffman table")
	}

	if d.bits.n < 8 {
		if err := d.ensureNBits(8); err != nil {
			if err != errMissingFF00 && err != errShortHuffmanData {
				return 0, err
			}
			// There are no more bytes of data in this segment, but we may still
			// be able to read the next symbol out of the previously read bits.
			// First, undo the readByte that the ensureNBits call made.
			if d.bytes.nUnreadable != 0 {
				d.unreadByteStuffedByte()
			}
			goto slowPath
		}
	}
	if v := h.lut[(d.bits.a>>uint32(d.bits.n-lutSize))&0xff]; v != 0 {
		n := (v & 0xff) - 1
		d.bits.n -= int32(n)
		d.bits.m >>= n
		return uint8(v >> 8), nil
	}

slowPath:
	for i, code := 0, int32(0); i < maxCodeLength; i++ {
		if d.bits.n == 0 {
			if err := d.ensureNBits(1); err != nil {
				return 0, err
			}
		}
		if d.bits.a&d.bits.m != 0 {
			code |= 1
		}
		d.bits.n--
		d.bits.m >>= 1
		if code <= h.maxCodes[i] {
			return h.vals[h.valsIndices[i]+code-h.minCodes[i]], nil
		}
		code <<= 1
	}
	return 0, FormatError("bad Huffman code")
}

func (d *decoder) decodeBit() (bool, error) {
	if d.bits.n == 0 {
		if err := d.ensureNBits(1); err != nil {
			return false, err
		}
	}
	ret := d.bits.a&d.bits.m != 0
	d.bits.n--
	d.bits.m >>= 1
	return ret, nil
}

func (d *decoder) decodeBits(n int32) (uint32, error) {
	if d.bits.n < n {
		if err := d.ensureNBits(n); err != nil {
			return 0, err
		}
	}
	ret := d.bits.a >> uint32(d.bits.n-n)
	ret &= (1 << uint32(n)) - 1
	d.bits.n -= n
	d.bits.m >>= uint32(n)
	return ret, nil
}

```

// === FILE: references/go/src/image/jpeg/reader.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package jpeg implements a JPEG image decoder and encoder.
//
// JPEG is defined in ITU-T T.81: https://www.w3.org/Graphics/JPEG/itu-t81.pdf.
package jpeg

import (
	"image"
	"image/color"
	"image/internal/imageutil"
	"io"
)

// A FormatError reports that the input is not a valid JPEG.
type FormatError string

func (e FormatError) Error() string { return "invalid JPEG format: " + string(e) }

// An UnsupportedError reports that the input uses a valid but unimplemented JPEG feature.
type UnsupportedError string

func (e UnsupportedError) Error() string { return "unsupported JPEG feature: " + string(e) }

var errUnsupportedSubsamplingRatio = UnsupportedError("luma/chroma subsampling ratio")

// Component specification, specified in section B.2.2.
type component struct {
	h       int   // Horizontal sampling factor.
	v       int   // Vertical sampling factor.
	c       uint8 // Component identifier.
	tq      uint8 // Quantization table destination selector.
	expandH int   // Horizontal expansion factor for non-standard subsampling.
	expandV int   // Vertical expansion factor for non-standard subsampling.
}

const (
	dcTable = 0
	acTable = 1
	maxTc   = 1
	maxTh   = 3
	maxTq   = 3

	maxComponents = 4
)

const (
	sof0Marker = 0xc0 // Start Of Frame (Baseline Sequential).
	sof1Marker = 0xc1 // Start Of Frame (Extended Sequential).
	sof2Marker = 0xc2 // Start Of Frame (Progressive).
	dhtMarker  = 0xc4 // Define Huffman Table.
	rst0Marker = 0xd0 // ReSTart (0).
	rst7Marker = 0xd7 // ReSTart (7).
	soiMarker  = 0xd8 // Start Of Image.
	eoiMarker  = 0xd9 // End Of Image.
	sosMarker  = 0xda // Start Of Scan.
	dqtMarker  = 0xdb // Define Quantization Table.
	driMarker  = 0xdd // Define Restart Interval.
	comMarker  = 0xfe // COMment.
	// "APPlication specific" markers aren't part of the JPEG spec per se,
	// but in practice, their use is described at
	// https://www.sno.phy.queensu.ca/~phil/exiftool/TagNames/JPEG.html
	app0Marker  = 0xe0
	app14Marker = 0xee
	app15Marker = 0xef
)

// See https://www.sno.phy.queensu.ca/~phil/exiftool/TagNames/JPEG.html#Adobe
const (
	adobeTransformUnknown = 0
	adobeTransformYCbCr   = 1
	adobeTransformYCbCrK  = 2
)

// unzig maps from the zig-zag ordering to the natural ordering. For example,
// unzig[3] is the column and row of the fourth element in zig-zag order. The
// value is 16, which means first column (16%8 == 0) and third row (16/8 == 2).
var unzig = [blockSize]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
}

// Deprecated: Reader is not used by the [image/jpeg] package and should
// not be used by others. It is kept for compatibility.
type Reader interface {
	io.ByteReader
	io.Reader
}

// bits holds the unprocessed bits that have been taken from the byte-stream.
// The n least significant bits of a form the unread bits, to be read in MSB to
// LSB order.
type bits struct {
	a uint32 // accumulator.
	m uint32 // mask. m==1<<(n-1) when n>0, with m==0 when n==0.
	n int32  // the number of unread bits in a.
}

type decoder struct {
	r    io.Reader
	bits bits
	// bytes is a byte buffer, similar to a bufio.Reader, except that it
	// has to be able to unread more than 1 byte, due to byte stuffing.
	// Byte stuffing is specified in section F.1.2.3.
	bytes struct {
		// buf[i:j] are the buffered bytes read from the underlying
		// io.Reader that haven't yet been passed further on.
		buf  [4096]byte
		i, j int
		// nUnreadable is the number of bytes to back up i after
		// overshooting. It can be 0, 1 or 2.
		nUnreadable int
	}
	width, height int

	img1        *image.Gray
	img3        *image.YCbCr
	blackPix    []byte
	blackStride int

	// For non-standard subsampling ratios (flex mode).
	flex       bool // True if using non-standard subsampling that requires manual pixel expansion.
	maxH, maxV int  // Maximum horizontal and vertical sampling factors across all components.

	ri    int // Restart Interval.
	nComp int

	// As per section 4.5, there are four modes of operation (selected by the
	// SOF? markers): sequential DCT, progressive DCT, lossless and
	// hierarchical, although this implementation does not support the latter
	// two non-DCT modes. Sequential DCT is further split into baseline and
	// extended, as per section 4.11.
	baseline    bool
	progressive bool

	jfif                bool
	adobeTransformValid bool
	adobeTransform      uint8
	eobRun              uint16 // End-of-Band run, specified in section G.1.2.2.

	comp       [maxComponents]component
	progCoeffs [maxComponents][]block // Saved state between progressive-mode scans.
	huff       [maxTc + 1][maxTh + 1]huffman
	quant      [maxTq + 1]block // Quantization tables, in zig-zag order.
	tmp        [2 * blockSize]byte
}

// fill fills up the d.bytes.buf buffer from the underlying io.Reader. It
// should only be called when there are no unread bytes in d.bytes.
func (d *decoder) fill() error {
	if d.bytes.i != d.bytes.j {
		panic("jpeg: fill called when unread bytes exist")
	}
	// Move the last 2 bytes to the start of the buffer, in case we need
	// to call unreadByteStuffedByte.
	if d.bytes.j > 2 {
		d.bytes.buf[0] = d.bytes.buf[d.bytes.j-2]
		d.bytes.buf[1] = d.bytes.buf[d.bytes.j-1]
		d.bytes.i, d.bytes.j = 2, 2
	}
	// Fill in the rest of the buffer.
	n, err := d.r.Read(d.bytes.buf[d.bytes.j:])
	d.bytes.j += n
	if n > 0 {
		return nil
	}
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}

// unreadByteStuffedByte undoes the most recent readByteStuffedByte call,
// giving a byte of data back from d.bits to d.bytes. The Huffman look-up table
// requires at least 8 bits for look-up, which means that Huffman decoding can
// sometimes overshoot and read one or two too many bytes. Two-byte overshoot
// can happen when expecting to read a 0xff 0x00 byte-stuffed byte.
func (d *decoder) unreadByteStuffedByte() {
	d.bytes.i -= d.bytes.nUnreadable
	d.bytes.nUnreadable = 0
	if d.bits.n >= 8 {
		d.bits.a >>= 8
		d.bits.n -= 8
		d.bits.m >>= 8
	}
}

// readByte returns the next byte, whether buffered or not buffered. It does
// not care about byte stuffing.
func (d *decoder) readByte() (x byte, err error) {
	for d.bytes.i == d.bytes.j {
		if err = d.fill(); err != nil {
			return 0, err
		}
	}
	x = d.bytes.buf[d.bytes.i]
	d.bytes.i++
	d.bytes.nUnreadable = 0
	return x, nil
}

// errMissingFF00 means that readByteStuffedByte encountered an 0xff byte (a
// marker byte) that wasn't the expected byte-stuffed sequence 0xff, 0x00.
var errMissingFF00 = FormatError("missing 0xff00 sequence")

// readByteStuffedByte is like readByte but is for byte-stuffed Huffman data.
func (d *decoder) readByteStuffedByte() (x byte, err error) {
	// Take the fast path if d.bytes.buf contains at least two bytes.
	if d.bytes.i+2 <= d.bytes.j {
		x = d.bytes.buf[d.bytes.i]
		d.bytes.i++
		d.bytes.nUnreadable = 1
		if x != 0xff {
			return x, err
		}
		if d.bytes.buf[d.bytes.i] != 0x00 {
			return 0, errMissingFF00
		}
		d.bytes.i++
		d.bytes.nUnreadable = 2
		return 0xff, nil
	}

	d.bytes.nUnreadable = 0

	x, err = d.readByte()
	if err != nil {
		return 0, err
	}
	d.bytes.nUnreadable = 1
	if x != 0xff {
		return x, nil
	}

	x, err = d.readByte()
	if err != nil {
		return 0, err
	}
	d.bytes.nUnreadable = 2
	if x != 0x00 {
		return 0, errMissingFF00
	}
	return 0xff, nil
}

// readFull reads exactly len(p) bytes into p. It does not care about byte
// stuffing.
func (d *decoder) readFull(p []byte) error {
	// Unread the overshot bytes, if any.
	if d.bytes.nUnreadable != 0 {
		if d.bits.n >= 8 {
			d.unreadByteStuffedByte()
		}
		d.bytes.nUnreadable = 0
	}

	for {
		n := copy(p, d.bytes.buf[d.bytes.i:d.bytes.j])
		p = p[n:]
		d.bytes.i += n
		if len(p) == 0 {
			break
		}
		if err := d.fill(); err != nil {
			return err
		}
	}
	return nil
}

// ignore ignores the next n bytes.
func (d *decoder) ignore(n int) error {
	// Unread the overshot bytes, if any.
	if d.bytes.nUnreadable != 0 {
		if d.bits.n >= 8 {
			d.unreadByteStuffedByte()
		}
		d.bytes.nUnreadable = 0
	}

	for {
		m := d.bytes.j - d.bytes.i
		if m > n {
			m = n
		}
		d.bytes.i += m
		n -= m
		if n == 0 {
			break
		}
		if err := d.fill(); err != nil {
			return err
		}
	}
	return nil
}

// Specified in section B.2.2.
func (d *decoder) processSOF(n int) error {
	if d.nComp != 0 {
		return FormatError("multiple SOF markers")
	}
	switch n {
	case 6 + 3*1: // Grayscale image.
		d.nComp = 1
	case 6 + 3*3: // YCbCr or RGB image.
		d.nComp = 3
	case 6 + 3*4: // YCbCrK or CMYK image.
		d.nComp = 4
	default:
		return UnsupportedError("number of components")
	}
	if err := d.readFull(d.tmp[:n]); err != nil {
		return err
	}
	// We only support 8-bit precision.
	if d.tmp[0] != 8 {
		return UnsupportedError("precision")
	}
	d.height = int(d.tmp[1])<<8 + int(d.tmp[2])
	d.width = int(d.tmp[3])<<8 + int(d.tmp[4])
	if int(d.tmp[5]) != d.nComp {
		return FormatError("SOF has wrong length")
	}

	for i := 0; i < d.nComp; i++ {
		d.comp[i].c = d.tmp[6+3*i]
		// Section B.2.2 states that "the value of C_i shall be different from
		// the values of C_1 through C_(i-1)".
		for j := 0; j < i; j++ {
			if d.comp[i].c == d.comp[j].c {
				return FormatError("repeated component identifier")
			}
		}

		d.comp[i].tq = d.tmp[8+3*i]
		if d.comp[i].tq > maxTq {
			return FormatError("bad Tq value")
		}

		hv := d.tmp[7+3*i]
		h, v := int(hv>>4), int(hv&0x0f)
		if h < 1 || 4 < h || v < 1 || 4 < v {
			return FormatError("luma/chroma subsampling ratio")
		}
		if h == 3 || v == 3 {
			return errUnsupportedSubsamplingRatio
		}
		switch d.nComp {
		case 1:
			// If a JPEG image has only one component, section A.2 says "this data
			// is non-interleaved by definition" and section A.2.2 says "[in this
			// case...] the order of data units within a scan shall be left-to-right
			// and top-to-bottom... regardless of the values of H_1 and V_1". Section
			// 4.8.2 also says "[for non-interleaved data], the MCU is defined to be
			// one data unit". Similarly, section A.1.1 explains that it is the ratio
			// of H_i to max_j(H_j) that matters, and similarly for V. For grayscale
			// images, H_1 is the maximum H_j for all components j, so that ratio is
			// always 1. The component's (h, v) is effectively always (1, 1): even if
			// the nominal (h, v) is (2, 1), a 20x5 image is encoded in three 8x8
			// MCUs, not two 16x8 MCUs.
			h, v = 1, 1

		case 3:
			// For YCbCr images, we support both standard subsampling ratios
			// (4:4:4, 4:4:0, 4:2:2, 4:2:0, 4:1:1, 4:1:0) and non-standard ratios
			// where components may have different sampling factors. The only
			// restriction is that each component's sampling factors must evenly
			// divide the maximum factors (validated after the loop).

		case 4:
			// For 4-component images (either CMYK or YCbCrK), we only support two
			// hv vectors: [0x11 0x11 0x11 0x11] and [0x22 0x11 0x11 0x22].
			// Theoretically, 4-component JPEG images could mix and match hv values
			// but in practice, those two combinations are the only ones in use,
			// and it simplifies the applyBlack code below if we can assume that:
			//	- for CMYK, the C and K channels have full samples, and if the M
			//	  and Y channels subsample, they subsample both horizontally and
			//	  vertically.
			//	- for YCbCrK, the Y and K channels have full samples.
			switch i {
			case 0:
				if hv != 0x11 && hv != 0x22 {
					return errUnsupportedSubsamplingRatio
				}
			case 1, 2:
				if hv != 0x11 {
					return errUnsupportedSubsamplingRatio
				}
			case 3:
				if d.comp[0].h != h || d.comp[0].v != v {
					return errUnsupportedSubsamplingRatio
				}
			}
		}

		d.maxH, d.maxV = max(d.maxH, h), max(d.maxV, v)
		d.comp[i].h = h
		d.comp[i].v = v
	}

	// For 3-component images, validate that maxH and maxV are evenly divisible
	// by each component's sampling factors.
	if d.nComp == 3 {
		for i := 0; i < 3; i++ {
			if d.maxH%d.comp[i].h != 0 || d.maxV%d.comp[i].v != 0 {
				return errUnsupportedSubsamplingRatio
			}
		}
	}

	// Compute expansion factors for each component.
	for i := 0; i < d.nComp; i++ {
		d.comp[i].expandH = d.maxH / d.comp[i].h
		d.comp[i].expandV = d.maxV / d.comp[i].v
	}

	return nil
}

// Specified in section B.2.4.1.
func (d *decoder) processDQT(n int) error {
loop:
	for n > 0 {
		n--
		x, err := d.readByte()
		if err != nil {
			return err
		}
		tq := x & 0x0f
		if tq > maxTq {
			return FormatError("bad Tq value")
		}
		switch x >> 4 {
		default:
			return FormatError("bad Pq value")
		case 0:
			if n < blockSize {
				break loop
			}
			n -= blockSize
			if err := d.readFull(d.tmp[:blockSize]); err != nil {
				return err
			}
			for i := range d.quant[tq] {
				d.quant[tq][i] = int32(d.tmp[i])
			}
		case 1:
			if n < 2*blockSize {
				break loop
			}
			n -= 2 * blockSize
			if err := d.readFull(d.tmp[:2*blockSize]); err != nil {
				return err
			}
			for i := range d.quant[tq] {
				d.quant[tq][i] = int32(d.tmp[2*i])<<8 | int32(d.tmp[2*i+1])
			}
		}
	}
	if n != 0 {
		return FormatError("DQT has wrong length")
	}
	return nil
}

// Specified in section B.2.4.4.
func (d *decoder) processDRI(n int) error {
	if n != 2 {
		return FormatError("DRI has wrong length")
	}
	if err := d.readFull(d.tmp[:2]); err != nil {
		return err
	}
	d.ri = int(d.tmp[0])<<8 + int(d.tmp[1])
	return nil
}

func (d *decoder) processApp0Marker(n int) error {
	if n < 5 {
		return d.ignore(n)
	}
	if err := d.readFull(d.tmp[:5]); err != nil {
		return err
	}
	n -= 5

	d.jfif = d.tmp[0] == 'J' && d.tmp[1] == 'F' && d.tmp[2] == 'I' && d.tmp[3] == 'F' && d.tmp[4] == '\x00'

	if n > 0 {
		return d.ignore(n)
	}
	return nil
}

func (d *decoder) processApp14Marker(n int) error {
	if n < 12 {
		return d.ignore(n)
	}
	if err := d.readFull(d.tmp[:12]); err != nil {
		return err
	}
	n -= 12

	if d.tmp[0] == 'A' && d.tmp[1] == 'd' && d.tmp[2] == 'o' && d.tmp[3] == 'b' && d.tmp[4] == 'e' {
		d.adobeTransformValid = true
		d.adobeTransform = d.tmp[11]
	}

	if n > 0 {
		return d.ignore(n)
	}
	return nil
}

// decode reads a JPEG image from r and returns it as an image.Image.
func (d *decoder) decode(r io.Reader, configOnly bool) (image.Image, error) {
	d.r = r

	// Check for the Start Of Image marker.
	if err := d.readFull(d.tmp[:2]); err != nil {
		return nil, err
	}
	if d.tmp[0] != 0xff || d.tmp[1] != soiMarker {
		return nil, FormatError("missing SOI marker")
	}

	// Process the remaining segments until the End Of Image marker.
	for {
		err := d.readFull(d.tmp[:2])
		if err != nil {
			return nil, err
		}
		for d.tmp[0] != 0xff {
			// Strictly speaking, this is a format error. However, libjpeg is
			// liberal in what it accepts. As of version 9, next_marker in
			// jdmarker.c treats this as a warning (JWRN_EXTRANEOUS_DATA) and
			// continues to decode the stream. Even before next_marker sees
			// extraneous data, jpeg_fill_bit_buffer in jdhuff.c reads as many
			// bytes as it can, possibly past the end of a scan's data. It
			// effectively puts back any markers that it overscanned (e.g. an
			// "\xff\xd9" EOI marker), but it does not put back non-marker data,
			// and thus it can silently ignore a small number of extraneous
			// non-marker bytes before next_marker has a chance to see them (and
			// print a warning).
			//
			// We are therefore also liberal in what we accept. Extraneous data
			// is silently ignored.
			//
			// This is similar to, but not exactly the same as, the restart
			// mechanism within a scan (the RST[0-7] markers).
			//
			// Note that extraneous 0xff bytes in e.g. SOS data are escaped as
			// "\xff\x00", and so are detected a little further down below.
			d.tmp[0] = d.tmp[1]
			d.tmp[1], err = d.readByte()
			if err != nil {
				return nil, err
			}
		}
		marker := d.tmp[1]
		if marker == 0 {
			// Treat "\xff\x00" as extraneous data.
			continue
		}
		for marker == 0xff {
			// Section B.1.1.2 says, "Any marker may optionally be preceded by any
			// number of fill bytes, which are bytes assigned code X'FF'".
			marker, err = d.readByte()
			if err != nil {
				return nil, err
			}
		}
		if marker == eoiMarker { // End Of Image.
			break
		}
		if rst0Marker <= marker && marker <= rst7Marker {
			// Figures B.2 and B.16 of the specification suggest that restart markers should
			// only occur between Entropy Coded Segments and not after the final ECS.
			// However, some encoders may generate incorrect JPEGs with a final restart
			// marker. That restart marker will be seen here instead of inside the processSOS
			// method, and is ignored as a harmless error. Restart markers have no extra data,
			// so we check for this before we read the 16-bit length of the segment.
			continue
		}

		// Read the 16-bit length of the segment. The value includes the 2 bytes for the
		// length itself, so we subtract 2 to get the number of remaining bytes.
		if err = d.readFull(d.tmp[:2]); err != nil {
			return nil, err
		}
		n := int(d.tmp[0])<<8 + int(d.tmp[1]) - 2
		if n < 0 {
			return nil, FormatError("short segment length")
		}

		switch marker {
		case sof0Marker, sof1Marker, sof2Marker:
			d.baseline = marker == sof0Marker
			d.progressive = marker == sof2Marker
			err = d.processSOF(n)
			if configOnly && d.jfif {
				return nil, err
			}
		case dhtMarker:
			if configOnly {
				err = d.ignore(n)
			} else {
				err = d.processDHT(n)
			}
		case dqtMarker:
			if configOnly {
				err = d.ignore(n)
			} else {
				err = d.processDQT(n)
			}
		case sosMarker:
			if configOnly {
				return nil, nil
			}
			err = d.processSOS(n)
		case driMarker:
			if configOnly {
				err = d.ignore(n)
			} else {
				err = d.processDRI(n)
			}
		case app0Marker:
			err = d.processApp0Marker(n)
		case app14Marker:
			err = d.processApp14Marker(n)
		default:
			if app0Marker <= marker && marker <= app15Marker || marker == comMarker {
				err = d.ignore(n)
			} else if marker < 0xc0 { // See Table B.1 "Marker code assignments".
				err = FormatError("unknown marker")
			} else {
				err = UnsupportedError("unknown marker")
			}
		}
		if err != nil {
			return nil, err
		}
	}

	if d.progressive {
		if err := d.reconstructProgressiveImage(); err != nil {
			return nil, err
		}
	}
	if d.img1 != nil {
		return d.img1, nil
	}
	if d.img3 != nil {
		if d.blackPix != nil {
			return d.applyBlack()
		} else if d.isRGB() {
			return d.convertToRGB()
		}
		return d.img3, nil
	}
	return nil, FormatError("missing SOS marker")
}

// applyBlack combines d.img3 and d.blackPix into a CMYK image. The formula
// used depends on whether the JPEG image is stored as CMYK or YCbCrK,
// indicated by the APP14 (Adobe) metadata.
//
// Adobe CMYK JPEG images are inverted, where 255 means no ink instead of full
// ink, so we apply "v = 255 - v" at various points. Note that a double
// inversion is a no-op, so inversions might be implicit in the code below.
func (d *decoder) applyBlack() (image.Image, error) {
	if !d.adobeTransformValid {
		return nil, UnsupportedError("unknown color model: 4-component JPEG doesn't have Adobe APP14 metadata")
	}

	// If the 4-component JPEG image isn't explicitly marked as "Unknown (RGB
	// or CMYK)" as per
	// https://www.sno.phy.queensu.ca/~phil/exiftool/TagNames/JPEG.html#Adobe
	// we assume that it is YCbCrK. This matches libjpeg's jdapimin.c.
	if d.adobeTransform != adobeTransformUnknown {
		// Convert the YCbCr part of the YCbCrK to RGB, invert the RGB to get
		// CMY, and patch in the original K. The RGB to CMY inversion cancels
		// out the 'Adobe inversion' described in the applyBlack doc comment
		// above, so in practice, only the fourth channel (black) is inverted.
		bounds := d.img3.Bounds()
		img := image.NewRGBA(bounds)
		imageutil.DrawYCbCr(img, bounds, d.img3, bounds.Min)
		for iBase, y := 0, bounds.Min.Y; y < bounds.Max.Y; iBase, y = iBase+img.Stride, y+1 {
			for i, x := iBase+3, bounds.Min.X; x < bounds.Max.X; i, x = i+4, x+1 {
				img.Pix[i] = 255 - d.blackPix[(y-bounds.Min.Y)*d.blackStride+(x-bounds.Min.X)]
			}
		}
		return &image.CMYK{
			Pix:    img.Pix,
			Stride: img.Stride,
			Rect:   img.Rect,
		}, nil
	}

	// The first three channels (cyan, magenta, yellow) of the CMYK
	// were decoded into d.img3, but each channel was decoded into a separate
	// []byte slice, and some channels may be subsampled. We interleave the
	// separate channels into an image.CMYK's single []byte slice containing 4
	// contiguous bytes per pixel.
	bounds := d.img3.Bounds()
	img := image.NewCMYK(bounds)

	translations := [4]struct {
		src    []byte
		stride int
	}{
		{d.img3.Y, d.img3.YStride},
		{d.img3.Cb, d.img3.CStride},
		{d.img3.Cr, d.img3.CStride},
		{d.blackPix, d.blackStride},
	}
	for t, translation := range translations {
		subsample := d.comp[t].h != d.comp[0].h || d.comp[t].v != d.comp[0].v
		for iBase, y := 0, bounds.Min.Y; y < bounds.Max.Y; iBase, y = iBase+img.Stride, y+1 {
			sy := y - bounds.Min.Y
			if subsample {
				sy /= 2
			}
			for i, x := iBase+t, bounds.Min.X; x < bounds.Max.X; i, x = i+4, x+1 {
				sx := x - bounds.Min.X
				if subsample {
					sx /= 2
				}
				img.Pix[i] = 255 - translation.src[sy*translation.stride+sx]
			}
		}
	}
	return img, nil
}

func (d *decoder) isRGB() bool {
	if d.jfif {
		return false
	}
	if d.adobeTransformValid && d.adobeTransform == adobeTransformUnknown {
		// https://www.sno.phy.queensu.ca/~phil/exiftool/TagNames/JPEG.html#Adobe
		// says that 0 means Unknown (and in practice RGB) and 1 means YCbCr.
		return true
	}
	return d.comp[0].c == 'R' && d.comp[1].c == 'G' && d.comp[2].c == 'B'
}

func (d *decoder) convertToRGB() (image.Image, error) {
	// Historically, we only supported 4:4:4, 4:4:0, 4:2:2, 4:2:0, 4:1:1 or
	// 4:1:0 chroma subsampling ratios. Other configurations (including situations
	// where Chroma-Blue and Chroma-Red have different subsampling) are very rare,
	// but not impossible. That restriction was relaxed in Go 1.27 (2026).
	//
	// It's also very rare but not impossible for 3-channel JPEG images to be
	// RGB instead of YCbCr, in which case this convertToRGB function will be
	// called. Note that RGB-instead-of-YCbCr is a property of the JPEG file
	// itself (in the SOF marker), not of the Go code decoding the image.
	//
	// convertToRGB still makes those historical assumptions and does not
	// support the intersection of (1) atypical chroma subsampling and (2)
	// RGB-instead-of-YCbCr. Both of those are very rare and the intersection
	// is even more so.
	h0, h1, h2 := d.comp[0].h, d.comp[1].h, d.comp[2].h
	v0, v1, v2 := d.comp[0].v, d.comp[1].v, d.comp[2].v
	if (h1 != h2) || (h0%h1 != 0) || (v1 != v2) || (v0%v1 != 0) {
		return nil, errUnsupportedSubsamplingRatio
	}

	cScale := h0 / h1
	bounds := d.img3.Bounds()
	img := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		po := img.PixOffset(bounds.Min.X, y)
		yo := d.img3.YOffset(bounds.Min.X, y)
		co := d.img3.COffset(bounds.Min.X, y)
		for i, iMax := 0, bounds.Max.X-bounds.Min.X; i < iMax; i++ {
			img.Pix[po+4*i+0] = d.img3.Y[yo+i]
			img.Pix[po+4*i+1] = d.img3.Cb[co+i/cScale]
			img.Pix[po+4*i+2] = d.img3.Cr[co+i/cScale]
			img.Pix[po+4*i+3] = 255
		}
	}
	return img, nil
}

// Decode reads a JPEG image from r and returns it as an [image.Image].
func Decode(r io.Reader) (image.Image, error) {
	var d decoder
	return d.decode(r, false)
}

// DecodeConfig returns the color model and dimensions of a JPEG image without
// decoding the entire image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	var d decoder
	if _, err := d.decode(r, true); err != nil {
		return image.Config{}, err
	}
	switch d.nComp {
	case 1:
		return image.Config{
			ColorModel: color.GrayModel,
			Width:      d.width,
			Height:     d.height,
		}, nil
	case 3:
		cm := color.YCbCrModel
		if d.isRGB() {
			cm = color.RGBAModel
		}
		return image.Config{
			ColorModel: cm,
			Width:      d.width,
			Height:     d.height,
		}, nil
	case 4:
		return image.Config{
			ColorModel: color.CMYKModel,
			Width:      d.width,
			Height:     d.height,
		}, nil
	}
	return image.Config{}, FormatError("missing SOF marker")
}

func init() {
	image.RegisterFormat("jpeg", "\xff\xd8", Decode, DecodeConfig)
}

```

// === FILE: references/go/src/image/jpeg/scan.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jpeg

import (
	"image"
)

// makeImg allocates and initializes the destination image.
func (d *decoder) makeImg(mxx, myy int) {
	if d.nComp == 1 {
		m := image.NewGray(image.Rect(0, 0, 8*mxx, 8*myy))
		d.img1 = m.SubImage(image.Rect(0, 0, d.width, d.height)).(*image.Gray)
		return
	}

	// Determine if we need flex mode for non-standard subsampling.
	// Flex mode is needed when:
	// - Cb and Cr have different sampling factors, or
	// - The Y component doesn't have the maximum sampling factors, or
	// - The ratio doesn't match any standard YCbCrSubsampleRatio.
	subsampleRatio := image.YCbCrSubsampleRatio444
	if d.comp[1].h != d.comp[2].h || d.comp[1].v != d.comp[2].v ||
		d.maxH != d.comp[0].h || d.maxV != d.comp[0].v {
		d.flex = true
	} else {
		hRatio := d.maxH / d.comp[1].h
		vRatio := d.maxV / d.comp[1].v
		switch hRatio<<4 | vRatio {
		case 0x11:
			subsampleRatio = image.YCbCrSubsampleRatio444
		case 0x12:
			subsampleRatio = image.YCbCrSubsampleRatio440
		case 0x21:
			subsampleRatio = image.YCbCrSubsampleRatio422
		case 0x22:
			subsampleRatio = image.YCbCrSubsampleRatio420
		case 0x41:
			subsampleRatio = image.YCbCrSubsampleRatio411
		case 0x42:
			subsampleRatio = image.YCbCrSubsampleRatio410
		default:
			d.flex = true
		}
	}

	m := image.NewYCbCr(image.Rect(0, 0, 8*d.maxH*mxx, 8*d.maxV*myy), subsampleRatio)
	d.img3 = m.SubImage(image.Rect(0, 0, d.width, d.height)).(*image.YCbCr)

	if d.nComp == 4 {
		h3, v3 := d.comp[3].h, d.comp[3].v
		d.blackPix = make([]byte, 8*h3*mxx*8*v3*myy)
		d.blackStride = 8 * h3 * mxx
	}
}

// Specified in section B.2.3.
func (d *decoder) processSOS(n int) error {
	if d.nComp == 0 {
		return FormatError("missing SOF marker")
	}
	if n < 6 || 4+2*d.nComp < n || n%2 != 0 {
		return FormatError("SOS has wrong length")
	}
	if err := d.readFull(d.tmp[:n]); err != nil {
		return err
	}
	nComp := int(d.tmp[0])
	if n != 4+2*nComp {
		return FormatError("SOS length inconsistent with number of components")
	}
	var scan [maxComponents]struct {
		compIndex uint8
		td        uint8 // DC table selector.
		ta        uint8 // AC table selector.
	}
	totalHV := 0
	for i := 0; i < nComp; i++ {
		cs := d.tmp[1+2*i] // Component selector.
		compIndex := -1
		for j, comp := range d.comp[:d.nComp] {
			if cs == comp.c {
				compIndex = j
			}
		}
		if compIndex < 0 {
			return FormatError("unknown component selector")
		}
		scan[i].compIndex = uint8(compIndex)
		// Section B.2.3 states that "the value of Cs_j shall be different from
		// the values of Cs_1 through Cs_(j-1)". Since we have previously
		// verified that a frame's component identifiers (C_i values in section
		// B.2.2) are unique, it suffices to check that the implicit indexes
		// into d.comp are unique.
		for j := 0; j < i; j++ {
			if scan[i].compIndex == scan[j].compIndex {
				return FormatError("repeated component selector")
			}
		}
		totalHV += d.comp[compIndex].h * d.comp[compIndex].v

		// The baseline t <= 1 restriction is specified in table B.3.
		scan[i].td = d.tmp[2+2*i] >> 4
		if t := scan[i].td; t > maxTh || (d.baseline && t > 1) {
			return FormatError("bad Td value")
		}
		scan[i].ta = d.tmp[2+2*i] & 0x0f
		if t := scan[i].ta; t > maxTh || (d.baseline && t > 1) {
			return FormatError("bad Ta value")
		}
	}
	// Section B.2.3 states that if there is more than one component then the
	// total H*V values in a scan must be <= 10.
	if d.nComp > 1 && totalHV > 10 {
		return FormatError("total sampling factors too large")
	}

	// zigStart and zigEnd are the spectral selection bounds.
	// ah and al are the successive approximation high and low values.
	// The spec calls these values Ss, Se, Ah and Al.
	//
	// For progressive JPEGs, these are the two more-or-less independent
	// aspects of progression. Spectral selection progression is when not
	// all of a block's 64 DCT coefficients are transmitted in one pass.
	// For example, three passes could transmit coefficient 0 (the DC
	// component), coefficients 1-5, and coefficients 6-63, in zig-zag
	// order. Successive approximation is when not all of the bits of a
	// band of coefficients are transmitted in one pass. For example,
	// three passes could transmit the 6 most significant bits, followed
	// by the second-least significant bit, followed by the least
	// significant bit.
	//
	// For sequential JPEGs, these parameters are hard-coded to 0/63/0/0, as
	// per table B.3.
	zigStart, zigEnd, ah, al := int32(0), int32(blockSize-1), uint32(0), uint32(0)
	if d.progressive {
		zigStart = int32(d.tmp[1+2*nComp])
		zigEnd = int32(d.tmp[2+2*nComp])
		ah = uint32(d.tmp[3+2*nComp] >> 4)
		al = uint32(d.tmp[3+2*nComp] & 0x0f)
		if (zigStart == 0 && zigEnd != 0) || zigStart > zigEnd || blockSize <= zigEnd {
			return FormatError("bad spectral selection bounds")
		}
		if zigStart != 0 && nComp != 1 {
			return FormatError("progressive AC coefficients for more than one component")
		}
		if ah != 0 && ah != al+1 {
			return FormatError("bad successive approximation values")
		}
	}

	// mxx and myy are the number of MCUs (Minimum Coded Units) in the image.
	// The MCU dimensions are based on the maximum sampling factors.
	// For standard subsampling, maxH/maxV equals h0/v0 (Y's factors).
	// For flex mode, Y may not have the maximum factors.
	mxx := (d.width + 8*d.maxH - 1) / (8 * d.maxH)
	myy := (d.height + 8*d.maxV - 1) / (8 * d.maxV)
	if d.img1 == nil && d.img3 == nil {
		d.makeImg(mxx, myy)
	}
	if d.progressive {
		for i := 0; i < nComp; i++ {
			compIndex := scan[i].compIndex
			if d.progCoeffs[compIndex] == nil {
				d.progCoeffs[compIndex] = make([]block, mxx*myy*d.comp[compIndex].h*d.comp[compIndex].v)
			}
		}
	}

	d.bits = bits{}
	mcu, expectedRST := 0, uint8(rst0Marker)
	var (
		// b is the decoded coefficients, in natural (not zig-zag) order.
		b  block
		dc [maxComponents]int32
		// bx and by are the location of the current block, in units of 8x8
		// blocks: the third block in the first row has (bx, by) = (2, 0).
		bx, by     int
		blockCount int
	)
	for my := 0; my < myy; my++ {
		for mx := 0; mx < mxx; mx++ {
			for i := 0; i < nComp; i++ {
				compIndex := scan[i].compIndex
				hi := d.comp[compIndex].h
				vi := d.comp[compIndex].v
				for j := 0; j < hi*vi; j++ {
					// The blocks are traversed one MCU at a time. For 4:2:0 chroma
					// subsampling, there are four Y 8x8 blocks in every 16x16 MCU.
					//
					// For a sequential 32x16 pixel image, the Y blocks visiting order is:
					//	0 1 4 5
					//	2 3 6 7
					//
					// For progressive images, the interleaved scans (those with nComp > 1)
					// are traversed as above, but non-interleaved scans are traversed left
					// to right, top to bottom:
					//	0 1 2 3
					//	4 5 6 7
					// Only DC scans (zigStart == 0) can be interleaved. AC scans must have
					// only one component.
					//
					// To further complicate matters, for non-interleaved scans, there is no
					// data for any blocks that are inside the image at the MCU level but
					// outside the image at the pixel level. For example, a 24x16 pixel 4:2:0
					// progressive image consists of two 16x16 MCUs. The interleaved scans
					// will process 8 Y blocks:
					//	0 1 4 5
					//	2 3 6 7
					// The non-interleaved scans will process only 6 Y blocks:
					//	0 1 2
					//	3 4 5
					if nComp != 1 {
						bx = hi*mx + j%hi
						by = vi*my + j/hi
					} else {
						q := mxx * hi
						bx = blockCount % q
						by = blockCount / q
						blockCount++
						if bx*8 >= d.width || by*8 >= d.height {
							continue
						}
					}

					// Load the previous partially decoded coefficients, if applicable.
					if d.progressive {
						b = d.progCoeffs[compIndex][by*mxx*hi+bx]
					} else {
						b = block{}
					}

					if ah != 0 {
						if err := d.refine(&b, &d.huff[acTable][scan[i].ta], zigStart, zigEnd, 1<<al); err != nil {
							return err
						}
					} else {
						zig := zigStart
						if zig == 0 {
							zig++
							// Decode the DC coefficient, as specified in section F.2.2.1.
							value, err := d.decodeHuffman(&d.huff[dcTable][scan[i].td])
							if err != nil {
								return err
							}
							if value > 16 {
								return UnsupportedError("excessive DC component")
							}
							dcDelta, err := d.receiveExtend(value)
							if err != nil {
								return err
							}
							dc[compIndex] += dcDelta
							b[0] = dc[compIndex] << al
						}

						if zig <= zigEnd && d.eobRun > 0 {
							d.eobRun--
						} else {
							// Decode the AC coefficients, as specified in section F.2.2.2.
							huff := &d.huff[acTable][scan[i].ta]
							for ; zig <= zigEnd; zig++ {
								value, err := d.decodeHuffman(huff)
								if err != nil {
									return err
								}
								val0 := value >> 4
								val1 := value & 0x0f
								if val1 != 0 {
									zig += int32(val0)
									if zig > zigEnd {
										break
									}
									ac, err := d.receiveExtend(val1)
									if err != nil {
										return err
									}
									b[unzig[zig]] = ac << al
								} else {
									if val0 != 0x0f {
										d.eobRun = uint16(1 << val0)
										if val0 != 0 {
											bits, err := d.decodeBits(int32(val0))
											if err != nil {
												return err
											}
											d.eobRun |= uint16(bits)
										}
										d.eobRun--
										break
									}
									zig += 0x0f
								}
							}
						}
					}

					if d.progressive {
						// Save the coefficients.
						d.progCoeffs[compIndex][by*mxx*hi+bx] = b
						// At this point, we could call reconstructBlock to dequantize and perform the
						// inverse DCT, to save early stages of a progressive image to the *image.YCbCr
						// buffers (the whole point of progressive encoding), but in Go, the jpeg.Decode
						// function does not return until the entire image is decoded, so we "continue"
						// here to avoid wasted computation. Instead, reconstructBlock is called on each
						// accumulated block by the reconstructProgressiveImage method after all of the
						// SOS markers are processed.
						continue
					}
					if err := d.reconstructBlock(&b, bx, by, int(compIndex)); err != nil {
						return err
					}
				} // for j
			} // for i
			mcu++
			if d.ri > 0 && mcu%d.ri == 0 && mcu < mxx*myy {
				// For well-formed input, the RST[0-7] restart marker follows
				// immediately. For corrupt input, call findRST to try to
				// resynchronize.
				if err := d.readFull(d.tmp[:2]); err != nil {
					return err
				} else if d.tmp[0] != 0xff || d.tmp[1] != expectedRST {
					if err := d.findRST(expectedRST); err != nil {
						return err
					}
				}
				expectedRST++
				if expectedRST == rst7Marker+1 {
					expectedRST = rst0Marker
				}
				// Reset the Huffman decoder.
				d.bits = bits{}
				// Reset the DC components, as per section F.2.1.3.1.
				dc = [maxComponents]int32{}
				// Reset the progressive decoder state, as per section G.1.2.2.
				d.eobRun = 0
			}
		} // for mx
	} // for my

	return nil
}

// refine decodes a successive approximation refinement block, as specified in
// section G.1.2.
func (d *decoder) refine(b *block, h *huffman, zigStart, zigEnd, delta int32) error {
	// Refining a DC component is trivial.
	if zigStart == 0 {
		if zigEnd != 0 {
			panic("unreachable")
		}
		bit, err := d.decodeBit()
		if err != nil {
			return err
		}
		if bit {
			b[0] |= delta
		}
		return nil
	}

	// Refining AC components is more complicated; see sections G.1.2.2 and G.1.2.3.
	zig := zigStart
	if d.eobRun == 0 {
	loop:
		for ; zig <= zigEnd; zig++ {
			z := int32(0)
			value, err := d.decodeHuffman(h)
			if err != nil {
				return err
			}
			val0 := value >> 4
			val1 := value & 0x0f

			switch val1 {
			case 0:
				if val0 != 0x0f {
					d.eobRun = uint16(1 << val0)
					if val0 != 0 {
						bits, err := d.decodeBits(int32(val0))
						if err != nil {
							return err
						}
						d.eobRun |= uint16(bits)
					}
					break loop
				}
			case 1:
				z = delta
				bit, err := d.decodeBit()
				if err != nil {
					return err
				}
				if !bit {
					z = -z
				}
			default:
				return FormatError("unexpected Huffman code")
			}

			zig, err = d.refineNonZeroes(b, zig, zigEnd, int32(val0), delta)
			if err != nil {
				return err
			}
			if zig > zigEnd {
				return FormatError("too many coefficients")
			}
			if z != 0 {
				b[unzig[zig]] = z
			}
		}
	}
	if d.eobRun > 0 {
		d.eobRun--
		if _, err := d.refineNonZeroes(b, zig, zigEnd, -1, delta); err != nil {
			return err
		}
	}
	return nil
}

// refineNonZeroes refines non-zero entries of b in zig-zag order. If nz >= 0,
// the first nz zero entries are skipped over.
func (d *decoder) refineNonZeroes(b *block, zig, zigEnd, nz, delta int32) (int32, error) {
	for ; zig <= zigEnd; zig++ {
		u := unzig[zig]
		if b[u] == 0 {
			if nz == 0 {
				break
			}
			nz--
			continue
		}
		bit, err := d.decodeBit()
		if err != nil {
			return 0, err
		}
		if !bit {
			continue
		}
		if b[u] >= 0 {
			b[u] += delta
		} else {
			b[u] -= delta
		}
	}
	return zig, nil
}

func (d *decoder) reconstructProgressiveImage() error {
	// The mxx, by and bx variables have the same meaning as in the
	// processSOS method.
	mxx := (d.width + 8*d.maxH - 1) / (8 * d.maxH)
	for i := 0; i < d.nComp; i++ {
		if d.progCoeffs[i] == nil {
			continue
		}
		v := 8 * d.maxV / d.comp[i].v
		h := 8 * d.maxH / d.comp[i].h
		stride := mxx * d.comp[i].h
		for by := 0; by*v < d.height; by++ {
			for bx := 0; bx*h < d.width; bx++ {
				if err := d.reconstructBlock(&d.progCoeffs[i][by*stride+bx], bx, by, i); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// reconstructBlock dequantizes, performs the inverse DCT and stores the block
// to the image.
func (d *decoder) reconstructBlock(b *block, bx, by, compIndex int) error {
	qt := &d.quant[d.comp[compIndex].tq]
	for zig := 0; zig < blockSize; zig++ {
		b[unzig[zig]] *= qt[zig]
	}
	idct(b)

	var h, v int
	if d.flex {
		// Flex mode: scale bx and by according to the component's sampling factors.
		h = d.comp[compIndex].expandH
		v = d.comp[compIndex].expandV
		bx, by = bx*h, by*v
	}

	dst, stride := []byte(nil), 0
	if d.nComp == 1 {
		dst, stride = d.img1.Pix[8*(by*d.img1.Stride+bx):], d.img1.Stride
	} else {
		switch compIndex {
		case 0:
			dst, stride = d.img3.Y[8*(by*d.img3.YStride+bx):], d.img3.YStride
		case 1:
			dst, stride = d.img3.Cb[8*(by*d.img3.CStride+bx):], d.img3.CStride
		case 2:
			dst, stride = d.img3.Cr[8*(by*d.img3.CStride+bx):], d.img3.CStride
		case 3:
			dst, stride = d.blackPix[8*(by*d.blackStride+bx):], d.blackStride
		default:
			return UnsupportedError("too many components")
		}
	}

	if d.flex {
		// Flex mode: expand each source pixel to h×v destination pixels.
		for y := 0; y < 8; y++ {
			y8 := y * 8
			yv := y * v
			for x := 0; x < 8; x++ {
				val := uint8(max(0, min(255, b[y8+x]+128)))
				xh := x * h
				for yy := 0; yy < v; yy++ {
					for xx := 0; xx < h; xx++ {
						dst[(yv+yy)*stride+xh+xx] = val
					}
				}
			}
		}
		return nil
	}

	// Level shift by +128, clip to [0, 255], and write to dst.
	for y := 0; y < 8; y++ {
		y8 := y * 8
		yStride := y * stride
		for x := 0; x < 8; x++ {
			dst[yStride+x] = uint8(max(0, min(255, b[y8+x]+128)))
		}
	}
	return nil
}

// findRST advances past the next RST restart marker that matches expectedRST.
// Other than I/O errors, it is also an error if we encounter an {0xFF, M}
// two-byte marker sequence where M is not 0x00, 0xFF or the expectedRST.
//
// This is similar to libjpeg's jdmarker.c's next_marker function.
// https://github.com/libjpeg-turbo/libjpeg-turbo/blob/2dfe6c0fe9e18671105e94f7cbf044d4a1d157e6/jdmarker.c#L892-L935
//
// Precondition: d.tmp[:2] holds the next two bytes of JPEG-encoded input
// (input in the d.readFull sense).
func (d *decoder) findRST(expectedRST uint8) error {
	for {
		// i is the index such that, at the bottom of the loop, we read 2-i
		// bytes into d.tmp[i:2], maintaining the invariant that d.tmp[:2]
		// holds the next two bytes of JPEG-encoded input. It is either 0 or 1,
		// so that each iteration advances by 1 or 2 bytes (or returns).
		i := 0

		if d.tmp[0] == 0xff {
			if d.tmp[1] == expectedRST {
				return nil
			} else if d.tmp[1] == 0xff {
				i = 1
			} else if d.tmp[1] != 0x00 {
				// libjpeg's jdmarker.c's jpeg_resync_to_restart does something
				// fancy here, treating RST markers within two (modulo 8) of
				// expectedRST differently from RST markers that are 'more
				// distant'. Until we see evidence that recovering from such
				// cases is frequent enough to be worth the complexity, we take
				// a simpler approach for now. Any marker that's not 0x00, 0xff
				// or expectedRST is a fatal FormatError.
				return FormatError("bad RST marker")
			}

		} else if d.tmp[1] == 0xff {
			d.tmp[0] = 0xff
			i = 1
		}

		if err := d.readFull(d.tmp[i:2]); err != nil {
			return err
		}
	}
}

```

// === FILE: references/go/src/image/jpeg/writer.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jpeg

import (
	"bufio"
	"errors"
	"image"
	"image/color"
	"io"
)

// div returns a/b rounded to the nearest integer, instead of rounded to zero.
func div(a, b int32) int32 {
	if a >= 0 {
		return (a + (b >> 1)) / b
	}
	return -((-a + (b >> 1)) / b)
}

// bitCount counts the number of bits needed to hold an integer.
var bitCount = [256]byte{
	0, 1, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4,
	5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
}

type quantIndex int

const (
	quantIndexLuminance quantIndex = iota
	quantIndexChrominance
	nQuantIndex
)

// unscaledQuant are the unscaled quantization tables in zig-zag order. Each
// encoder copies and scales the tables according to its quality parameter.
// The values are derived from section K.1 of the spec, after converting from
// natural to zig-zag order.
var unscaledQuant = [nQuantIndex][blockSize]byte{
	// Luminance.
	{
		16, 11, 12, 14, 12, 10, 16, 14,
		13, 14, 18, 17, 16, 19, 24, 40,
		26, 24, 22, 22, 24, 49, 35, 37,
		29, 40, 58, 51, 61, 60, 57, 51,
		56, 55, 64, 72, 92, 78, 64, 68,
		87, 69, 55, 56, 80, 109, 81, 87,
		95, 98, 103, 104, 103, 62, 77, 113,
		121, 112, 100, 120, 92, 101, 103, 99,
	},
	// Chrominance.
	{
		17, 18, 18, 24, 21, 24, 47, 26,
		26, 47, 99, 66, 56, 66, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
		99, 99, 99, 99, 99, 99, 99, 99,
	},
}

type huffIndex int

const (
	huffIndexLuminanceDC huffIndex = iota
	huffIndexLuminanceAC
	huffIndexChrominanceDC
	huffIndexChrominanceAC
	nHuffIndex
)

// huffmanSpec specifies a Huffman encoding.
type huffmanSpec struct {
	// count[i] is the number of codes of length i+1 bits.
	count [16]byte
	// value[i] is the decoded value of the i'th codeword.
	value []byte
}

// theHuffmanSpec is the Huffman encoding specifications.
//
// This encoder uses the same Huffman encoding for all images. It is also the
// same Huffman encoding used by section K.3 of the spec.
//
// The DC tables have 12 decoded values, called categories.
//
// The AC tables have 162 decoded values: bytes that pack a 4-bit Run and a
// 4-bit Size. There are 16 valid Runs and 10 valid Sizes, plus two special R|S
// cases: 0|0 (meaning EOB) and F|0 (meaning ZRL).
var theHuffmanSpec = [nHuffIndex]huffmanSpec{
	// Luminance DC.
	{
		[16]byte{0, 1, 5, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0},
		[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
	},
	// Luminance AC.
	{
		[16]byte{0, 2, 1, 3, 3, 2, 4, 3, 5, 5, 4, 4, 0, 0, 1, 125},
		[]byte{
			0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12,
			0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07,
			0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
			0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0,
			0x24, 0x33, 0x62, 0x72, 0x82, 0x09, 0x0a, 0x16,
			0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
			0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39,
			0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49,
			0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
			0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69,
			0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79,
			0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
			0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98,
			0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7,
			0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
			0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5,
			0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4,
			0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
			0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea,
			0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
			0xf9, 0xfa,
		},
	},
	// Chrominance DC.
	{
		[16]byte{0, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0},
		[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
	},
	// Chrominance AC.
	{
		[16]byte{0, 2, 1, 2, 4, 4, 3, 4, 7, 5, 4, 4, 0, 1, 2, 119},
		[]byte{
			0x00, 0x01, 0x02, 0x03, 0x11, 0x04, 0x05, 0x21,
			0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71,
			0x13, 0x22, 0x32, 0x81, 0x08, 0x14, 0x42, 0x91,
			0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0,
			0x15, 0x62, 0x72, 0xd1, 0x0a, 0x16, 0x24, 0x34,
			0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26,
			0x27, 0x28, 0x29, 0x2a, 0x35, 0x36, 0x37, 0x38,
			0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
			0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
			0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68,
			0x69, 0x6a, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78,
			0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
			0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95, 0x96,
			0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5,
			0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4,
			0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3,
			0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xd2,
			0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda,
			0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9,
			0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8,
			0xf9, 0xfa,
		},
	},
}

// huffmanLUT is a compiled look-up table representation of a huffmanSpec.
// Each value maps to a uint32 of which the 8 most significant bits hold the
// codeword size in bits and the 24 least significant bits hold the codeword.
// The maximum codeword size is 16 bits.
type huffmanLUT []uint32

func (h *huffmanLUT) init(s huffmanSpec) {
	maxValue := 0
	for _, v := range s.value {
		if int(v) > maxValue {
			maxValue = int(v)
		}
	}
	*h = make([]uint32, maxValue+1)
	code, k := uint32(0), 0
	for i := 0; i < len(s.count); i++ {
		nBits := uint32(i+1) << 24
		for j := uint8(0); j < s.count[i]; j++ {
			(*h)[s.value[k]] = nBits | code
			code++
			k++
		}
		code <<= 1
	}
}

// theHuffmanLUT are compiled representations of theHuffmanSpec.
var theHuffmanLUT [4]huffmanLUT

func init() {
	for i, s := range theHuffmanSpec {
		theHuffmanLUT[i].init(s)
	}
}

// writer is a buffered writer.
type writer interface {
	Flush() error
	io.Writer
	io.ByteWriter
}

// encoder encodes an image to the JPEG format.
type encoder struct {
	// w is the writer to write to. err is the first error encountered during
	// writing. All attempted writes after the first error become no-ops.
	w   writer
	err error
	// buf is a scratch buffer.
	buf [16]byte
	// bits and nBits are accumulated bits to write to w.
	bits, nBits uint32
	// quant is the scaled quantization tables, in zig-zag order.
	quant [nQuantIndex][blockSize]byte
}

func (e *encoder) flush() {
	if e.err != nil {
		return
	}
	e.err = e.w.Flush()
}

func (e *encoder) write(p []byte) {
	if e.err != nil {
		return
	}
	_, e.err = e.w.Write(p)
}

func (e *encoder) writeByte(b byte) {
	if e.err != nil {
		return
	}
	e.err = e.w.WriteByte(b)
}

// emit emits the least significant nBits bits of bits to the bit-stream.
// The precondition is bits < 1<<nBits && nBits <= 16.
func (e *encoder) emit(bits, nBits uint32) {
	nBits += e.nBits
	bits <<= 32 - nBits
	bits |= e.bits
	for nBits >= 8 {
		b := uint8(bits >> 24)
		e.writeByte(b)
		if b == 0xff {
			e.writeByte(0x00)
		}
		bits <<= 8
		nBits -= 8
	}
	e.bits, e.nBits = bits, nBits
}

// emitHuff emits the given value with the given Huffman encoder.
func (e *encoder) emitHuff(h huffIndex, value int32) {
	x := theHuffmanLUT[h][value]
	e.emit(x&(1<<24-1), x>>24)
}

// emitHuffRLE emits a run of runLength copies of value encoded with the given
// Huffman encoder.
func (e *encoder) emitHuffRLE(h huffIndex, runLength, value int32) {
	a, b := value, value
	if a < 0 {
		a, b = -value, value-1
	}
	var nBits uint32
	if a < 0x100 {
		nBits = uint32(bitCount[a])
	} else {
		nBits = 8 + uint32(bitCount[a>>8])
	}
	e.emitHuff(h, runLength<<4|int32(nBits))
	if nBits > 0 {
		e.emit(uint32(b)&(1<<nBits-1), nBits)
	}
}

// writeMarkerHeader writes the header for a marker with the given length.
func (e *encoder) writeMarkerHeader(marker uint8, markerlen int) {
	e.buf[0] = 0xff
	e.buf[1] = marker
	e.buf[2] = uint8(markerlen >> 8)
	e.buf[3] = uint8(markerlen & 0xff)
	e.write(e.buf[:4])
}

// writeDQT writes the Define Quantization Table marker.
func (e *encoder) writeDQT() {
	const markerlen = 2 + int(nQuantIndex)*(1+blockSize)
	e.writeMarkerHeader(dqtMarker, markerlen)
	for i := range e.quant {
		e.writeByte(uint8(i))
		e.write(e.quant[i][:])
	}
}

// writeSOF0 writes the Start Of Frame (Baseline Sequential) marker.
func (e *encoder) writeSOF0(size image.Point, nComponent int) {
	markerlen := 8 + 3*nComponent
	e.writeMarkerHeader(sof0Marker, markerlen)
	e.buf[0] = 8 // 8-bit color.
	e.buf[1] = uint8(size.Y >> 8)
	e.buf[2] = uint8(size.Y & 0xff)
	e.buf[3] = uint8(size.X >> 8)
	e.buf[4] = uint8(size.X & 0xff)
	e.buf[5] = uint8(nComponent)
	if nComponent == 1 {
		e.buf[6] = 1
		// No subsampling for grayscale image.
		e.buf[7] = 0x11
		e.buf[8] = 0x00
	} else {
		for i := 0; i < nComponent; i++ {
			e.buf[3*i+6] = uint8(i + 1)
			// We use 4:2:0 chroma subsampling.
			e.buf[3*i+7] = "\x22\x11\x11"[i]
			e.buf[3*i+8] = "\x00\x01\x01"[i]
		}
	}
	e.write(e.buf[:3*(nComponent-1)+9])
}

// writeDHT writes the Define Huffman Table marker.
func (e *encoder) writeDHT(nComponent int) {
	markerlen := 2
	specs := theHuffmanSpec[:]
	if nComponent == 1 {
		// Drop the Chrominance tables.
		specs = specs[:2]
	}
	for _, s := range specs {
		markerlen += 1 + 16 + len(s.value)
	}
	e.writeMarkerHeader(dhtMarker, markerlen)
	for i, s := range specs {
		e.writeByte("\x00\x10\x01\x11"[i])
		e.write(s.count[:])
		e.write(s.value)
	}
}

// writeBlock writes a block of pixel data using the given quantization table,
// returning the post-quantized DC value of the DCT-transformed block. b is in
// natural (not zig-zag) order.
func (e *encoder) writeBlock(b *block, q quantIndex, prevDC int32) int32 {
	fdct(b)
	// Emit the DC delta.
	dc := div(b[0], 8*int32(e.quant[q][0]))
	e.emitHuffRLE(huffIndex(2*q+0), 0, dc-prevDC)
	// Emit the AC components.
	h, runLength := huffIndex(2*q+1), int32(0)
	for zig := 1; zig < blockSize; zig++ {
		ac := div(b[unzig[zig]], 8*int32(e.quant[q][zig]))
		if ac == 0 {
			runLength++
		} else {
			for runLength > 15 {
				e.emitHuff(h, 0xf0)
				runLength -= 16
			}
			e.emitHuffRLE(h, runLength, ac)
			runLength = 0
		}
	}
	if runLength > 0 {
		e.emitHuff(h, 0x00)
	}
	return dc
}

// toYCbCr converts the 8x8 region of m whose top-left corner is p to its
// YCbCr values.
func toYCbCr(m image.Image, p image.Point, yBlock, cbBlock, crBlock *block) {
	b := m.Bounds()
	xmax := b.Max.X - 1
	ymax := b.Max.Y - 1
	for j := 0; j < 8; j++ {
		for i := 0; i < 8; i++ {
			r, g, b, _ := m.At(min(p.X+i, xmax), min(p.Y+j, ymax)).RGBA()
			yy, cb, cr := color.RGBToYCbCr(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			yBlock[8*j+i] = int32(yy)
			cbBlock[8*j+i] = int32(cb)
			crBlock[8*j+i] = int32(cr)
		}
	}
}

// grayToY stores the 8x8 region of m whose top-left corner is p in yBlock.
func grayToY(m *image.Gray, p image.Point, yBlock *block) {
	b := m.Bounds()
	xmax := b.Max.X - 1
	ymax := b.Max.Y - 1
	pix := m.Pix
	for j := 0; j < 8; j++ {
		for i := 0; i < 8; i++ {
			idx := m.PixOffset(min(p.X+i, xmax), min(p.Y+j, ymax))
			yBlock[8*j+i] = int32(pix[idx])
		}
	}
}

// rgbaToYCbCr is a specialized version of toYCbCr for image.RGBA images.
func rgbaToYCbCr(m *image.RGBA, p image.Point, yBlock, cbBlock, crBlock *block) {
	b := m.Bounds()
	xmax := b.Max.X - 1
	ymax := b.Max.Y - 1
	for j := 0; j < 8; j++ {
		sj := p.Y + j
		if sj > ymax {
			sj = ymax
		}
		offset := (sj-b.Min.Y)*m.Stride - b.Min.X*4
		for i := 0; i < 8; i++ {
			sx := p.X + i
			if sx > xmax {
				sx = xmax
			}
			pix := m.Pix[offset+sx*4:]
			yy, cb, cr := color.RGBToYCbCr(pix[0], pix[1], pix[2])
			yBlock[8*j+i] = int32(yy)
			cbBlock[8*j+i] = int32(cb)
			crBlock[8*j+i] = int32(cr)
		}
	}
}

// yCbCrToYCbCr is a specialized version of toYCbCr for image.YCbCr images.
func yCbCrToYCbCr(m *image.YCbCr, p image.Point, yBlock, cbBlock, crBlock *block) {
	b := m.Bounds()
	xmax := b.Max.X - 1
	ymax := b.Max.Y - 1
	for j := 0; j < 8; j++ {
		sy := p.Y + j
		if sy > ymax {
			sy = ymax
		}
		for i := 0; i < 8; i++ {
			sx := p.X + i
			if sx > xmax {
				sx = xmax
			}
			yi := m.YOffset(sx, sy)
			ci := m.COffset(sx, sy)
			yBlock[8*j+i] = int32(m.Y[yi])
			cbBlock[8*j+i] = int32(m.Cb[ci])
			crBlock[8*j+i] = int32(m.Cr[ci])
		}
	}
}

// scale scales the 16x16 region represented by the 4 src blocks to the 8x8
// dst block.
func scale(dst *block, src *[4]block) {
	for i := 0; i < 4; i++ {
		dstOff := (i&2)<<4 | (i&1)<<2
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				j := 16*y + 2*x
				sum := src[i][j] + src[i][j+1] + src[i][j+8] + src[i][j+9]
				dst[8*y+x+dstOff] = (sum + 2) >> 2
			}
		}
	}
}

// sosHeaderY is the SOS marker "\xff\xda" followed by 8 bytes:
//   - the marker length "\x00\x08",
//   - the number of components "\x01",
//   - component 1 uses DC table 0 and AC table 0 "\x01\x00",
//   - the bytes "\x00\x3f\x00". Section B.2.3 of the spec says that for
//     sequential DCTs, those bytes (8-bit Ss, 8-bit Se, 4-bit Ah, 4-bit Al)
//     should be 0x00, 0x3f, 0x00<<4 | 0x00.
var sosHeaderY = []byte{
	0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3f, 0x00,
}

// sosHeaderYCbCr is the SOS marker "\xff\xda" followed by 12 bytes:
//   - the marker length "\x00\x0c",
//   - the number of components "\x03",
//   - component 1 uses DC table 0 and AC table 0 "\x01\x00",
//   - component 2 uses DC table 1 and AC table 1 "\x02\x11",
//   - component 3 uses DC table 1 and AC table 1 "\x03\x11",
//   - the bytes "\x00\x3f\x00". Section B.2.3 of the spec says that for
//     sequential DCTs, those bytes (8-bit Ss, 8-bit Se, 4-bit Ah, 4-bit Al)
//     should be 0x00, 0x3f, 0x00<<4 | 0x00.
var sosHeaderYCbCr = []byte{
	0xff, 0xda, 0x00, 0x0c, 0x03, 0x01, 0x00, 0x02,
	0x11, 0x03, 0x11, 0x00, 0x3f, 0x00,
}

// writeSOS writes the StartOfScan marker.
func (e *encoder) writeSOS(m image.Image) {
	switch m.(type) {
	case *image.Gray:
		e.write(sosHeaderY)
	default:
		e.write(sosHeaderYCbCr)
	}
	var (
		// Scratch buffers to hold the YCbCr values.
		// The blocks are in natural (not zig-zag) order.
		b      block
		cb, cr [4]block
		// DC components are delta-encoded.
		prevDCY, prevDCCb, prevDCCr int32
	)
	bounds := m.Bounds()
	switch m := m.(type) {
	// TODO(wathiede): switch on m.ColorModel() instead of type.
	case *image.Gray:
		for y := bounds.Min.Y; y < bounds.Max.Y; y += 8 {
			for x := bounds.Min.X; x < bounds.Max.X; x += 8 {
				p := image.Pt(x, y)
				grayToY(m, p, &b)
				prevDCY = e.writeBlock(&b, 0, prevDCY)
			}
		}
	default:
		rgba, _ := m.(*image.RGBA)
		ycbcr, _ := m.(*image.YCbCr)
		for y := bounds.Min.Y; y < bounds.Max.Y; y += 16 {
			for x := bounds.Min.X; x < bounds.Max.X; x += 16 {
				for i := 0; i < 4; i++ {
					xOff := (i & 1) * 8
					yOff := (i & 2) * 4
					p := image.Pt(x+xOff, y+yOff)
					if rgba != nil {
						rgbaToYCbCr(rgba, p, &b, &cb[i], &cr[i])
					} else if ycbcr != nil {
						yCbCrToYCbCr(ycbcr, p, &b, &cb[i], &cr[i])
					} else {
						toYCbCr(m, p, &b, &cb[i], &cr[i])
					}
					prevDCY = e.writeBlock(&b, 0, prevDCY)
				}
				scale(&b, &cb)
				prevDCCb = e.writeBlock(&b, 1, prevDCCb)
				scale(&b, &cr)
				prevDCCr = e.writeBlock(&b, 1, prevDCCr)
			}
		}
	}
	// Pad the last byte with 1's.
	e.emit(0x7f, 7)
}

// DefaultQuality is the default quality encoding parameter.
const DefaultQuality = 75

// Options are the encoding parameters.
// Quality ranges from 1 to 100 inclusive, higher is better.
type Options struct {
	Quality int
}

// Encode writes the Image m to w in JPEG 4:2:0 baseline format with the given
// options. Default parameters are used if a nil *[Options] is passed.
func Encode(w io.Writer, m image.Image, o *Options) error {
	b := m.Bounds()
	if b.Dx() >= 1<<16 || b.Dy() >= 1<<16 {
		return errors.New("jpeg: image is too large to encode")
	}
	var e encoder
	if ww, ok := w.(writer); ok {
		e.w = ww
	} else {
		e.w = bufio.NewWriter(w)
	}
	// Clip quality to [1, 100].
	quality := DefaultQuality
	if o != nil {
		quality = o.Quality
		if quality < 1 {
			quality = 1
		} else if quality > 100 {
			quality = 100
		}
	}
	// Convert from a quality rating to a scaling factor.
	var scale int
	if quality < 50 {
		scale = 5000 / quality
	} else {
		scale = 200 - quality*2
	}
	// Initialize the quantization tables.
	for i := range e.quant {
		for j := range e.quant[i] {
			x := int(unscaledQuant[i][j])
			x = (x*scale + 50) / 100
			if x < 1 {
				x = 1
			} else if x > 255 {
				x = 255
			}
			e.quant[i][j] = uint8(x)
		}
	}
	// Compute number of components based on input image type.
	nComponent := 3
	switch m.(type) {
	// TODO(wathiede): switch on m.ColorModel() instead of type.
	case *image.Gray:
		nComponent = 1
	}
	// Write the Start Of Image marker.
	e.buf[0] = 0xff
	e.buf[1] = 0xd8
	e.write(e.buf[:2])
	// Write the quantization tables.
	e.writeDQT()
	// Write the image dimensions.
	e.writeSOF0(b.Size(), nComponent)
	// Write the Huffman tables.
	e.writeDHT(nComponent)
	// Write the image data.
	e.writeSOS(m)
	// Write the End Of Image marker.
	e.buf[0] = 0xff
	e.buf[1] = 0xd9
	e.write(e.buf[:2])
	e.flush()
	return e.err
}

```

// === FILE: references/go/src/image/names.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package image

import (
	"image/color"
)

var (
	// Black is an opaque black uniform image.
	Black = NewUniform(color.Black)
	// White is an opaque white uniform image.
	White = NewUniform(color.White)
	// Transparent is a fully transparent uniform image.
	Transparent = NewUniform(color.Transparent)
	// Opaque is a fully opaque uniform image.
	Opaque = NewUniform(color.Opaque)
)

// Uniform is an infinite-sized [Image] of uniform color.
// It implements the [color.Color], [color.Model], and [Image] interfaces.
type Uniform struct {
	C color.Color
}

func (c *Uniform) RGBA() (r, g, b, a uint32) {
	return c.C.RGBA()
}

func (c *Uniform) ColorModel() color.Model {
	return c
}

func (c *Uniform) Convert(color.Color) color.Color {
	return c.C
}

func (c *Uniform) Bounds() Rectangle { return Rectangle{Point{-1e9, -1e9}, Point{1e9, 1e9}} }

func (c *Uniform) At(x, y int) color.Color { return c.C }

func (c *Uniform) RGBA64At(x, y int) color.RGBA64 {
	r, g, b, a := c.C.RGBA()
	return color.RGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (c *Uniform) Opaque() bool {
	_, _, _, a := c.C.RGBA()
	return a == 0xffff
}

// NewUniform returns a new [Uniform] image of the given color.
func NewUniform(c color.Color) *Uniform {
	return &Uniform{c}
}

```

// === FILE: references/go/src/image/png/paeth.go ===
```go
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package png

// intSize is either 32 or 64.
const intSize = 32 << (^uint(0) >> 63)

func abs(x int) int {
	// m := -1 if x < 0. m := 0 otherwise.
	m := x >> (intSize - 1)

	// In two's complement representation, the negative number
	// of any number (except the smallest one) can be computed
	// by flipping all the bits and add 1. This is faster than
	// code with a branch.
	// See Hacker's Delight, section 2-4.
	return (x ^ m) - m
}

// paeth implements the Paeth filter function, as per the PNG specification.
func paeth(a, b, c uint8) uint8 {
	// This is an optimized version of the sample code in the PNG spec.
	// For example, the sample code starts with:
	//	p := int(a) + int(b) - int(c)
	//	pa := abs(p - int(a))
	// but the optimized form uses fewer arithmetic operations:
	//	pa := int(b) - int(c)
	//	pa = abs(pa)
	pc := int(c)
	pa := int(b) - pc
	pb := int(a) - pc
	pc = abs(pa + pb)
	pa = abs(pa)
	pb = abs(pb)
	if pa <= pb && pa <= pc {
		return a
	} else if pb <= pc {
		return b
	}
	return c
}

// filterPaeth applies the Paeth filter to the cdat slice.
// cdat is the current row's data, pdat is the previous row's data.
func filterPaeth(cdat, pdat []byte, bytesPerPixel int) {
	var a, b, c, pa, pb, pc int
	for i := 0; i < bytesPerPixel; i++ {
		a, c = 0, 0
		for j := i; j < len(cdat); j += bytesPerPixel {
			b = int(pdat[j])
			pa = b - c
			pb = a - c
			pc = abs(pa + pb)
			pa = abs(pa)
			pb = abs(pb)
			if pa <= pb && pa <= pc {
				// No-op.
			} else if pb <= pc {
				a = b
			} else {
				a = c
			}
			a += int(cdat[j])
			a &= 0xff
			cdat[j] = uint8(a)
			c = b
		}
	}
}

```

// === FILE: references/go/src/image/png/reader.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package png implements a PNG image decoder and encoder.
//
// The PNG specification is at https://www.w3.org/TR/PNG/.
package png

import (
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"hash"
	"hash/crc32"
	"image"
	"image/color"
	"io"
)

// Color type, as per the PNG spec.
const (
	ctGrayscale      = 0
	ctTrueColor      = 2
	ctPaletted       = 3
	ctGrayscaleAlpha = 4
	ctTrueColorAlpha = 6
)

// A cb is a combination of color type and bit depth.
const (
	cbInvalid = iota
	cbG1
	cbG2
	cbG4
	cbG8
	cbGA8
	cbTC8
	cbP1
	cbP2
	cbP4
	cbP8
	cbTCA8
	cbG16
	cbGA16
	cbTC16
	cbTCA16
)

func cbPaletted(cb int) bool {
	return cbP1 <= cb && cb <= cbP8
}

func cbTrueColor(cb int) bool {
	return cb == cbTC8 || cb == cbTC16
}

// Filter type, as per the PNG spec.
const (
	ftNone    = 0
	ftSub     = 1
	ftUp      = 2
	ftAverage = 3
	ftPaeth   = 4
	nFilter   = 5
)

// Interlace type.
const (
	itNone  = 0
	itAdam7 = 1
)

// interlaceScan defines the placement and size of a pass for Adam7 interlacing.
type interlaceScan struct {
	xFactor, yFactor, xOffset, yOffset int
}

// interlacing defines Adam7 interlacing, with 7 passes of reduced images.
// See https://www.w3.org/TR/PNG/#8Interlace
var interlacing = []interlaceScan{
	{8, 8, 0, 0},
	{8, 8, 4, 0},
	{4, 8, 0, 4},
	{4, 4, 2, 0},
	{2, 4, 0, 2},
	{2, 2, 1, 0},
	{1, 2, 0, 1},
}

// Decoding stage.
// The PNG specification says that the IHDR, PLTE (if present), tRNS (if
// present), IDAT and IEND chunks must appear in that order. There may be
// multiple IDAT chunks, and IDAT chunks must be sequential (i.e. they may not
// have any other chunks between them).
// https://www.w3.org/TR/PNG/#5ChunkOrdering
const (
	dsStart = iota
	dsSeenIHDR
	dsSeenPLTE
	dsSeentRNS
	dsSeenIDAT
	dsSeenIEND
)

const pngHeader = "\x89PNG\r\n\x1a\n"

type decoder struct {
	r             io.Reader
	img           image.Image
	crc           hash.Hash32
	width, height int
	depth         int
	palette       color.Palette
	cb            int
	stage         int
	idatLength    uint32
	tmp           [3 * 256]byte
	interlace     int

	// useTransparent and transparent are used for grayscale and truecolor
	// transparency, as opposed to palette transparency.
	useTransparent bool
	transparent    [6]byte
}

// A FormatError reports that the input is not a valid PNG.
type FormatError string

func (e FormatError) Error() string { return "png: invalid format: " + string(e) }

var chunkOrderError = FormatError("chunk out of order")

// An UnsupportedError reports that the input uses a valid but unimplemented PNG feature.
type UnsupportedError string

func (e UnsupportedError) Error() string { return "png: unsupported feature: " + string(e) }

func (d *decoder) parseIHDR(length uint32) error {
	if length != 13 {
		return FormatError("bad IHDR length")
	}
	if _, err := io.ReadFull(d.r, d.tmp[:13]); err != nil {
		return err
	}
	d.crc.Write(d.tmp[:13])
	if d.tmp[10] != 0 {
		return UnsupportedError("compression method")
	}
	if d.tmp[11] != 0 {
		return UnsupportedError("filter method")
	}
	if d.tmp[12] != itNone && d.tmp[12] != itAdam7 {
		return FormatError("invalid interlace method")
	}
	d.interlace = int(d.tmp[12])

	w := int32(binary.BigEndian.Uint32(d.tmp[0:4]))
	h := int32(binary.BigEndian.Uint32(d.tmp[4:8]))
	if w <= 0 || h <= 0 {
		return FormatError("non-positive dimension")
	}
	nPixels64 := int64(w) * int64(h)
	nPixels := int(nPixels64)
	if nPixels64 != int64(nPixels) {
		return UnsupportedError("dimension overflow")
	}
	// There can be up to 8 bytes per pixel, for 16 bits per channel RGBA.
	if nPixels != (nPixels*8)/8 {
		return UnsupportedError("dimension overflow")
	}

	d.cb = cbInvalid
	d.depth = int(d.tmp[8])
	switch d.depth {
	case 1:
		switch d.tmp[9] {
		case ctGrayscale:
			d.cb = cbG1
		case ctPaletted:
			d.cb = cbP1
		}
	case 2:
		switch d.tmp[9] {
		case ctGrayscale:
			d.cb = cbG2
		case ctPaletted:
			d.cb = cbP2
		}
	case 4:
		switch d.tmp[9] {
		case ctGrayscale:
			d.cb = cbG4
		case ctPaletted:
			d.cb = cbP4
		}
	case 8:
		switch d.tmp[9] {
		case ctGrayscale:
			d.cb = cbG8
		case ctTrueColor:
			d.cb = cbTC8
		case ctPaletted:
			d.cb = cbP8
		case ctGrayscaleAlpha:
			d.cb = cbGA8
		case ctTrueColorAlpha:
			d.cb = cbTCA8
		}
	case 16:
		switch d.tmp[9] {
		case ctGrayscale:
			d.cb = cbG16
		case ctTrueColor:
			d.cb = cbTC16
		case ctGrayscaleAlpha:
			d.cb = cbGA16
		case ctTrueColorAlpha:
			d.cb = cbTCA16
		}
	}
	if d.cb == cbInvalid {
		return UnsupportedError(fmt.Sprintf("bit depth %d, color type %d", d.tmp[8], d.tmp[9]))
	}
	d.width, d.height = int(w), int(h)
	return d.verifyChecksum()
}

func (d *decoder) parsePLTE(length uint32) error {
	np := int(length / 3) // The number of palette entries.
	if length%3 != 0 || np <= 0 || np > 256 || np > 1<<uint(d.depth) {
		return FormatError("bad PLTE length")
	}
	n, err := io.ReadFull(d.r, d.tmp[:3*np])
	if err != nil {
		return err
	}
	d.crc.Write(d.tmp[:n])
	switch d.cb {
	case cbP1, cbP2, cbP4, cbP8:
		d.palette = make(color.Palette, 256)
		for i := 0; i < np; i++ {
			d.palette[i] = color.RGBA{d.tmp[3*i+0], d.tmp[3*i+1], d.tmp[3*i+2], 0xff}
		}
		for i := np; i < 256; i++ {
			// Initialize the rest of the palette to opaque black. The spec (section
			// 11.2.3) says that "any out-of-range pixel value found in the image data
			// is an error", but some real-world PNG files have out-of-range pixel
			// values. We fall back to opaque black, the same as libpng 1.5.13;
			// ImageMagick 6.5.7 returns an error.
			d.palette[i] = color.RGBA{0x00, 0x00, 0x00, 0xff}
		}
		d.palette = d.palette[:np]
	case cbTC8, cbTCA8, cbTC16, cbTCA16:
		// As per the PNG spec, a PLTE chunk is optional (and for practical purposes,
		// ignorable) for the ctTrueColor and ctTrueColorAlpha color types (section 4.1.2).
	default:
		return FormatError("PLTE, color type mismatch")
	}
	return d.verifyChecksum()
}

func (d *decoder) parsetRNS(length uint32) error {
	switch d.cb {
	case cbG1, cbG2, cbG4, cbG8, cbG16:
		if length != 2 {
			return FormatError("bad tRNS length")
		}
		n, err := io.ReadFull(d.r, d.tmp[:length])
		if err != nil {
			return err
		}
		d.crc.Write(d.tmp[:n])

		copy(d.transparent[:], d.tmp[:length])
		switch d.cb {
		case cbG1:
			d.transparent[1] *= 0xff
		case cbG2:
			d.transparent[1] *= 0x55
		case cbG4:
			d.transparent[1] *= 0x11
		}
		d.useTransparent = true

	case cbTC8, cbTC16:
		if length != 6 {
			return FormatError("bad tRNS length")
		}
		n, err := io.ReadFull(d.r, d.tmp[:length])
		if err != nil {
			return err
		}
		d.crc.Write(d.tmp[:n])

		copy(d.transparent[:], d.tmp[:length])
		d.useTransparent = true

	case cbP1, cbP2, cbP4, cbP8:
		if length > 256 {
			return FormatError("bad tRNS length")
		}
		n, err := io.ReadFull(d.r, d.tmp[:length])
		if err != nil {
			return err
		}
		d.crc.Write(d.tmp[:n])

		if len(d.palette) < n {
			d.palette = d.palette[:n]
		}
		for i := 0; i < n; i++ {
			rgba := d.palette[i].(color.RGBA)
			d.palette[i] = color.NRGBA{rgba.R, rgba.G, rgba.B, d.tmp[i]}
		}

	default:
		return FormatError("tRNS, color type mismatch")
	}
	return d.verifyChecksum()
}

// Read presents one or more IDAT chunks as one continuous stream (minus the
// intermediate chunk headers and footers). If the PNG data looked like:
//
//	... len0 IDAT xxx crc0 len1 IDAT yy crc1 len2 IEND crc2
//
// then this reader presents xxxyy. For well-formed PNG data, the decoder state
// immediately before the first Read call is that d.r is positioned between the
// first IDAT and xxx, and the decoder state immediately after the last Read
// call is that d.r is positioned between yy and crc1.
func (d *decoder) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for d.idatLength == 0 {
		// We have exhausted an IDAT chunk. Verify the checksum of that chunk.
		if err := d.verifyChecksum(); err != nil {
			return 0, err
		}
		// Read the length and chunk type of the next chunk, and check that
		// it is an IDAT chunk.
		if _, err := io.ReadFull(d.r, d.tmp[:8]); err != nil {
			return 0, err
		}
		d.idatLength = binary.BigEndian.Uint32(d.tmp[:4])
		if string(d.tmp[4:8]) != "IDAT" {
			return 0, FormatError("not enough pixel data")
		}
		d.crc.Reset()
		d.crc.Write(d.tmp[4:8])
	}
	if int(d.idatLength) < 0 {
		return 0, UnsupportedError("IDAT chunk length overflow")
	}
	n, err := d.r.Read(p[:min(len(p), int(d.idatLength))])
	d.crc.Write(p[:n])
	d.idatLength -= uint32(n)
	return n, err
}

// decode decodes the IDAT data into an image.
func (d *decoder) decode() (image.Image, error) {
	r, err := zlib.NewReader(d)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var img image.Image
	if d.interlace == itNone {
		img, err = d.readImagePass(r, 0, false)
		if err != nil {
			return nil, err
		}
	} else if d.interlace == itAdam7 {
		// Allocate a blank image of the full size.
		img, err = d.readImagePass(nil, 0, true)
		if err != nil {
			return nil, err
		}
		for pass := 0; pass < 7; pass++ {
			imagePass, err := d.readImagePass(r, pass, false)
			if err != nil {
				return nil, err
			}
			if imagePass != nil {
				d.mergePassInto(img, imagePass, pass)
			}
		}
	}

	// Check for EOF, to verify the zlib checksum.
	n := 0
	for i := 0; n == 0 && err == nil; i++ {
		if i == 100 {
			return nil, io.ErrNoProgress
		}
		n, err = r.Read(d.tmp[:1])
	}
	if err != nil && err != io.EOF {
		return nil, FormatError(err.Error())
	}
	if n != 0 || d.idatLength != 0 {
		return nil, FormatError("too much pixel data")
	}

	return img, nil
}

// readImagePass reads a single image pass, sized according to the pass number.
func (d *decoder) readImagePass(r io.Reader, pass int, allocateOnly bool) (image.Image, error) {
	bitsPerPixel := 0
	pixOffset := 0
	var (
		gray     *image.Gray
		rgba     *image.RGBA
		paletted *image.Paletted
		nrgba    *image.NRGBA
		gray16   *image.Gray16
		rgba64   *image.RGBA64
		nrgba64  *image.NRGBA64
		img      image.Image
	)
	width, height := d.width, d.height
	if d.interlace == itAdam7 && !allocateOnly {
		p := interlacing[pass]
		// Add the multiplication factor and subtract one, effectively rounding up.
		width = (width - p.xOffset + p.xFactor - 1) / p.xFactor
		height = (height - p.yOffset + p.yFactor - 1) / p.yFactor
		// A PNG image can't have zero width or height, but for an interlaced
		// image, an individual pass might have zero width or height. If so, we
		// shouldn't even read a per-row filter type byte, so return early.
		if width == 0 || height == 0 {
			return nil, nil
		}
	}
	switch d.cb {
	case cbG1, cbG2, cbG4, cbG8:
		bitsPerPixel = d.depth
		if d.useTransparent {
			nrgba = image.NewNRGBA(image.Rect(0, 0, width, height))
			img = nrgba
		} else {
			gray = image.NewGray(image.Rect(0, 0, width, height))
			img = gray
		}
	case cbGA8:
		bitsPerPixel = 16
		nrgba = image.NewNRGBA(image.Rect(0, 0, width, height))
		img = nrgba
	case cbTC8:
		bitsPerPixel = 24
		if d.useTransparent {
			nrgba = image.NewNRGBA(image.Rect(0, 0, width, height))
			img = nrgba
		} else {
			rgba = image.NewRGBA(image.Rect(0, 0, width, height))
			img = rgba
		}
	case cbP1, cbP2, cbP4, cbP8:
		bitsPerPixel = d.depth
		paletted = image.NewPaletted(image.Rect(0, 0, width, height), d.palette)
		img = paletted
	case cbTCA8:
		bitsPerPixel = 32
		nrgba = image.NewNRGBA(image.Rect(0, 0, width, height))
		img = nrgba
	case cbG16:
		bitsPerPixel = 16
		if d.useTransparent {
			nrgba64 = image.NewNRGBA64(image.Rect(0, 0, width, height))
			img = nrgba64
		} else {
			gray16 = image.NewGray16(image.Rect(0, 0, width, height))
			img = gray16
		}
	case cbGA16:
		bitsPerPixel = 32
		nrgba64 = image.NewNRGBA64(image.Rect(0, 0, width, height))
		img = nrgba64
	case cbTC16:
		bitsPerPixel = 48
		if d.useTransparent {
			nrgba64 = image.NewNRGBA64(image.Rect(0, 0, width, height))
			img = nrgba64
		} else {
			rgba64 = image.NewRGBA64(image.Rect(0, 0, width, height))
			img = rgba64
		}
	case cbTCA16:
		bitsPerPixel = 64
		nrgba64 = image.NewNRGBA64(image.Rect(0, 0, width, height))
		img = nrgba64
	}
	if allocateOnly {
		return img, nil
	}
	bytesPerPixel := (bitsPerPixel + 7) / 8

	// The +1 is for the per-row filter type, which is at cr[0].
	rowSize := 1 + (int64(bitsPerPixel)*int64(width)+7)/8
	if rowSize != int64(int(rowSize)) {
		return nil, UnsupportedError("dimension overflow")
	}
	// cr and pr are the bytes for the current and previous row.
	cr := make([]uint8, rowSize)
	pr := make([]uint8, rowSize)

	for y := 0; y < height; y++ {
		// Read the decompressed bytes.
		_, err := io.ReadFull(r, cr)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil, FormatError("not enough pixel data")
			}
			return nil, err
		}

		// Apply the filter.
		cdat := cr[1:]
		pdat := pr[1:]
		switch cr[0] {
		case ftNone:
			// No-op.
		case ftSub:
			for i := bytesPerPixel; i < len(cdat); i++ {
				cdat[i] += cdat[i-bytesPerPixel]
			}
		case ftUp:
			for i, p := range pdat {
				cdat[i] += p
			}
		case ftAverage:
			// The first column has no column to the left of it, so it is a
			// special case. We know that the first column exists because we
			// check above that width != 0, and so len(cdat) != 0.
			for i := 0; i < bytesPerPixel; i++ {
				cdat[i] += pdat[i] / 2
			}
			for i := bytesPerPixel; i < len(cdat); i++ {
				cdat[i] += uint8((int(cdat[i-bytesPerPixel]) + int(pdat[i])) / 2)
			}
		case ftPaeth:
			filterPaeth(cdat, pdat, bytesPerPixel)
		default:
			return nil, FormatError("bad filter type")
		}

		// Convert from bytes to colors.
		switch d.cb {
		case cbG1:
			if d.useTransparent {
				ty := d.transparent[1]
				for x := 0; x < width; x += 8 {
					b := cdat[x/8]
					for x2 := 0; x2 < 8 && x+x2 < width; x2++ {
						ycol := (b >> 7) * 0xff
						acol := uint8(0xff)
						if ycol == ty {
							acol = 0x00
						}
						nrgba.SetNRGBA(x+x2, y, color.NRGBA{ycol, ycol, ycol, acol})
						b <<= 1
					}
				}
			} else {
				for x := 0; x < width; x += 8 {
					b := cdat[x/8]
					for x2 := 0; x2 < 8 && x+x2 < width; x2++ {
						gray.SetGray(x+x2, y, color.Gray{(b >> 7) * 0xff})
						b <<= 1
					}
				}
			}
		case cbG2:
			if d.useTransparent {
				ty := d.transparent[1]
				for x := 0; x < width; x += 4 {
					b := cdat[x/4]
					for x2 := 0; x2 < 4 && x+x2 < width; x2++ {
						ycol := (b >> 6) * 0x55
						acol := uint8(0xff)
						if ycol == ty {
							acol = 0x00
						}
						nrgba.SetNRGBA(x+x2, y, color.NRGBA{ycol, ycol, ycol, acol})
						b <<= 2
					}
				}
			} else {
				for x := 0; x < width; x += 4 {
					b := cdat[x/4]
					for x2 := 0; x2 < 4 && x+x2 < width; x2++ {
						gray.SetGray(x+x2, y, color.Gray{(b >> 6) * 0x55})
						b <<= 2
					}
				}
			}
		case cbG4:
			if d.useTransparent {
				ty := d.transparent[1]
				for x := 0; x < width; x += 2 {
					b := cdat[x/2]
					for x2 := 0; x2 < 2 && x+x2 < width; x2++ {
						ycol := (b >> 4) * 0x11
						acol := uint8(0xff)
						if ycol == ty {
							acol = 0x00
						}
						nrgba.SetNRGBA(x+x2, y, color.NRGBA{ycol, ycol, ycol, acol})
						b <<= 4
					}
				}
			} else {
				for x := 0; x < width; x += 2 {
					b := cdat[x/2]
					for x2 := 0; x2 < 2 && x+x2 < width; x2++ {
						gray.SetGray(x+x2, y, color.Gray{(b >> 4) * 0x11})
						b <<= 4
					}
				}
			}
		case cbG8:
			if d.useTransparent {
				ty := d.transparent[1]
				for x := 0; x < width; x++ {
					ycol := cdat[x]
					acol := uint8(0xff)
					if ycol == ty {
						acol = 0x00
					}
					nrgba.SetNRGBA(x, y, color.NRGBA{ycol, ycol, ycol, acol})
				}
			} else {
				copy(gray.Pix[pixOffset:], cdat)
				pixOffset += gray.Stride
			}
		case cbGA8:
			for x := 0; x < width; x++ {
				ycol := cdat[2*x+0]
				nrgba.SetNRGBA(x, y, color.NRGBA{ycol, ycol, ycol, cdat[2*x+1]})
			}
		case cbTC8:
			if d.useTransparent {
				pix, i, j := nrgba.Pix, pixOffset, 0
				tr, tg, tb := d.transparent[1], d.transparent[3], d.transparent[5]
				for x := 0; x < width; x++ {
					r := cdat[j+0]
					g := cdat[j+1]
					b := cdat[j+2]
					a := uint8(0xff)
					if r == tr && g == tg && b == tb {
						a = 0x00
					}
					pix[i+0] = r
					pix[i+1] = g
					pix[i+2] = b
					pix[i+3] = a
					i += 4
					j += 3
				}
				pixOffset += nrgba.Stride
			} else {
				pix, i, j := rgba.Pix, pixOffset, 0
				for x := 0; x < width; x++ {
					pix[i+0] = cdat[j+0]
					pix[i+1] = cdat[j+1]
					pix[i+2] = cdat[j+2]
					pix[i+3] = 0xff
					i += 4
					j += 3
				}
				pixOffset += rgba.Stride
			}
		case cbP1:
			for x := 0; x < width; x += 8 {
				b := cdat[x/8]
				for x2 := 0; x2 < 8 && x+x2 < width; x2++ {
					idx := b >> 7
					if len(paletted.Palette) <= int(idx) {
						paletted.Palette = paletted.Palette[:int(idx)+1]
					}
					paletted.SetColorIndex(x+x2, y, idx)
					b <<= 1
				}
			}
		case cbP2:
			for x := 0; x < width; x += 4 {
				b := cdat[x/4]
				for x2 := 0; x2 < 4 && x+x2 < width; x2++ {
					idx := b >> 6
					if len(paletted.Palette) <= int(idx) {
						paletted.Palette = paletted.Palette[:int(idx)+1]
					}
					paletted.SetColorIndex(x+x2, y, idx)
					b <<= 2
				}
			}
		case cbP4:
			for x := 0; x < width; x += 2 {
				b := cdat[x/2]
				for x2 := 0; x2 < 2 && x+x2 < width; x2++ {
					idx := b >> 4
					if len(paletted.Palette) <= int(idx) {
						paletted.Palette = paletted.Palette[:int(idx)+1]
					}
					paletted.SetColorIndex(x+x2, y, idx)
					b <<= 4
				}
			}
		case cbP8:
			if len(paletted.Palette) != 256 {
				for x := 0; x < width; x++ {
					if len(paletted.Palette) <= int(cdat[x]) {
						paletted.Palette = paletted.Palette[:int(cdat[x])+1]
					}
				}
			}
			copy(paletted.Pix[pixOffset:], cdat)
			pixOffset += paletted.Stride
		case cbTCA8:
			copy(nrgba.Pix[pixOffset:], cdat)
			pixOffset += nrgba.Stride
		case cbG16:
			if d.useTransparent {
				ty := uint16(d.transparent[0])<<8 | uint16(d.transparent[1])
				for x := 0; x < width; x++ {
					ycol := uint16(cdat[2*x+0])<<8 | uint16(cdat[2*x+1])
					acol := uint16(0xffff)
					if ycol == ty {
						acol = 0x0000
					}
					nrgba64.SetNRGBA64(x, y, color.NRGBA64{ycol, ycol, ycol, acol})
				}
			} else {
				for x := 0; x < width; x++ {
					ycol := uint16(cdat[2*x+0])<<8 | uint16(cdat[2*x+1])
					gray16.SetGray16(x, y, color.Gray16{ycol})
				}
			}
		case cbGA16:
			for x := 0; x < width; x++ {
				ycol := uint16(cdat[4*x+0])<<8 | uint16(cdat[4*x+1])
				acol := uint16(cdat[4*x+2])<<8 | uint16(cdat[4*x+3])
				nrgba64.SetNRGBA64(x, y, color.NRGBA64{ycol, ycol, ycol, acol})
			}
		case cbTC16:
			if d.useTransparent {
				tr := uint16(d.transparent[0])<<8 | uint16(d.transparent[1])
				tg := uint16(d.transparent[2])<<8 | uint16(d.transparent[3])
				tb := uint16(d.transparent[4])<<8 | uint16(d.transparent[5])
				for x := 0; x < width; x++ {
					rcol := uint16(cdat[6*x+0])<<8 | uint16(cdat[6*x+1])
					gcol := uint16(cdat[6*x+2])<<8 | uint16(cdat[6*x+3])
					bcol := uint16(cdat[6*x+4])<<8 | uint16(cdat[6*x+5])
					acol := uint16(0xffff)
					if rcol == tr && gcol == tg && bcol == tb {
						acol = 0x0000
					}
					nrgba64.SetNRGBA64(x, y, color.NRGBA64{rcol, gcol, bcol, acol})
				}
			} else {
				for x := 0; x < width; x++ {
					rcol := uint16(cdat[6*x+0])<<8 | uint16(cdat[6*x+1])
					gcol := uint16(cdat[6*x+2])<<8 | uint16(cdat[6*x+3])
					bcol := uint16(cdat[6*x+4])<<8 | uint16(cdat[6*x+5])
					rgba64.SetRGBA64(x, y, color.RGBA64{rcol, gcol, bcol, 0xffff})
				}
			}
		case cbTCA16:
			for x := 0; x < width; x++ {
				rcol := uint16(cdat[8*x+0])<<8 | uint16(cdat[8*x+1])
				gcol := uint16(cdat[8*x+2])<<8 | uint16(cdat[8*x+3])
				bcol := uint16(cdat[8*x+4])<<8 | uint16(cdat[8*x+5])
				acol := uint16(cdat[8*x+6])<<8 | uint16(cdat[8*x+7])
				nrgba64.SetNRGBA64(x, y, color.NRGBA64{rcol, gcol, bcol, acol})
			}
		}

		// The current row for y is the previous row for y+1.
		pr, cr = cr, pr
	}

	return img, nil
}

// mergePassInto merges a single pass into a full sized image.
func (d *decoder) mergePassInto(dst image.Image, src image.Image, pass int) {
	p := interlacing[pass]
	var (
		srcPix        []uint8
		dstPix        []uint8
		stride        int
		rect          image.Rectangle
		bytesPerPixel int
	)
	switch target := dst.(type) {
	case *image.Alpha:
		srcPix = src.(*image.Alpha).Pix
		dstPix, stride, rect = target.Pix, target.Stride, target.Rect
		bytesPerPixel = 1
	case *image.Alpha16:
		srcPix = src.(*image.Alpha16).Pix
		dstPix, stride, rect = target.Pix, target.Stride, target.Rect
		bytesPerPixel = 2
	case *image.Gray:
		srcPix = src.(*image.Gray).Pix
		dstPix, stride, rect = target.Pix, target.Stride, target.Rect
		bytesPerPixel = 1
	case *image.Gray16:
		srcPix = src.(*image.Gray16).Pix
		dstPix, stride, rect = target.Pix, target.Stride, target.Rect
		bytesPerPixel = 2
	case *image.NRGBA:
		srcPix = src.(*image.NRGBA).Pix
		dstPix, stride, rect = target.Pix, target.Stride, target.Rect
		bytesPerPixel = 4
	case *image.NRGBA64:
		srcPix = src.(*image.NRGBA64).Pix
		dstPix, stride, rect = target.Pix, target.Stride, target.Rect
		bytesPerPixel = 8
	case *image.Paletted:
		source := src.(*image.Paletted)
		srcPix = source.Pix
		dstPix, stride, rect = target.Pix, target.Stride, target.Rect
		bytesPerPixel = 1
		if len(target.Palette) < len(source.Palette) {
			// readImagePass can return a paletted image whose implicit palette
			// length (one more than the maximum Pix value) is larger than the
			// explicit palette length (what's in the PLTE chunk). Make the
			// same adjustment here.
			target.Palette = source.Palette
		}
	case *image.RGBA:
		srcPix = src.(*image.RGBA).Pix
		dstPix, stride, rect = target.Pix, target.Stride, target.Rect
		bytesPerPixel = 4
	case *image.RGBA64:
		srcPix = src.(*image.RGBA64).Pix
		dstPix, stride, rect = target.Pix, target.Stride, target.Rect
		bytesPerPixel = 8
	}
	s, bounds := 0, src.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		dBase := (y*p.yFactor+p.yOffset-rect.Min.Y)*stride + (p.xOffset-rect.Min.X)*bytesPerPixel
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			d := dBase + x*p.xFactor*bytesPerPixel
			copy(dstPix[d:], srcPix[s:s+bytesPerPixel])
			s += bytesPerPixel
		}
	}
}

func (d *decoder) parseIDAT(length uint32) (err error) {
	d.idatLength = length
	d.img, err = d.decode()
	if err != nil {
		return err
	}
	return d.verifyChecksum()
}

func (d *decoder) parseIEND(length uint32) error {
	if length != 0 {
		return FormatError("bad IEND length")
	}
	return d.verifyChecksum()
}

func (d *decoder) parseChunk(configOnly bool) error {
	// Read the length and chunk type.
	if _, err := io.ReadFull(d.r, d.tmp[:8]); err != nil {
		return err
	}
	length := binary.BigEndian.Uint32(d.tmp[:4])
	d.crc.Reset()
	d.crc.Write(d.tmp[4:8])

	// Read the chunk data.
	switch string(d.tmp[4:8]) {
	case "IHDR":
		if d.stage != dsStart {
			return chunkOrderError
		}
		d.stage = dsSeenIHDR
		return d.parseIHDR(length)
	case "PLTE":
		if d.stage != dsSeenIHDR {
			return chunkOrderError
		}
		d.stage = dsSeenPLTE
		return d.parsePLTE(length)
	case "tRNS":
		if cbPaletted(d.cb) {
			if d.stage != dsSeenPLTE {
				return chunkOrderError
			}
		} else if cbTrueColor(d.cb) {
			if d.stage != dsSeenIHDR && d.stage != dsSeenPLTE {
				return chunkOrderError
			}
		} else if d.stage != dsSeenIHDR {
			return chunkOrderError
		}
		d.stage = dsSeentRNS
		return d.parsetRNS(length)
	case "IDAT":
		if d.stage < dsSeenIHDR || d.stage > dsSeenIDAT || (d.stage == dsSeenIHDR && cbPaletted(d.cb)) {
			return chunkOrderError
		} else if d.stage == dsSeenIDAT {
			// Ignore trailing zero-length or garbage IDAT chunks.
			//
			// This does not affect valid PNG images that contain multiple IDAT
			// chunks, since the first call to parseIDAT below will consume all
			// consecutive IDAT chunks required for decoding the image.
			break
		}
		d.stage = dsSeenIDAT
		if configOnly {
			return nil
		}
		return d.parseIDAT(length)
	case "IEND":
		if d.stage != dsSeenIDAT {
			return chunkOrderError
		}
		d.stage = dsSeenIEND
		return d.parseIEND(length)
	}
	if length > 0x7fffffff {
		return FormatError(fmt.Sprintf("Bad chunk length: %d", length))
	}
	// Ignore this chunk (of a known length).
	var ignored [4096]byte
	for length > 0 {
		n, err := io.ReadFull(d.r, ignored[:min(len(ignored), int(length))])
		if err != nil {
			return err
		}
		d.crc.Write(ignored[:n])
		length -= uint32(n)
	}
	return d.verifyChecksum()
}

func (d *decoder) verifyChecksum() error {
	if _, err := io.ReadFull(d.r, d.tmp[:4]); err != nil {
		return err
	}
	if binary.BigEndian.Uint32(d.tmp[:4]) != d.crc.Sum32() {
		return FormatError("invalid checksum")
	}
	return nil
}

func (d *decoder) checkHeader() error {
	_, err := io.ReadFull(d.r, d.tmp[:len(pngHeader)])
	if err != nil {
		return err
	}
	if string(d.tmp[:len(pngHeader)]) != pngHeader {
		return FormatError("not a PNG file")
	}
	return nil
}

// Decode reads a PNG image from r and returns it as an [image.Image].
// The type of Image returned depends on the PNG contents.
func Decode(r io.Reader) (image.Image, error) {
	d := &decoder{
		r:   r,
		crc: crc32.NewIEEE(),
	}
	if err := d.checkHeader(); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	for d.stage != dsSeenIEND {
		if err := d.parseChunk(false); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}
	}
	return d.img, nil
}

// DecodeConfig returns the color model and dimensions of a PNG image without
// decoding the entire image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	d := &decoder{
		r:   r,
		crc: crc32.NewIEEE(),
	}
	if err := d.checkHeader(); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return image.Config{}, err
	}

	for {
		if err := d.parseChunk(true); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return image.Config{}, err
		}

		if cbPaletted(d.cb) {
			if d.stage >= dsSeentRNS {
				break
			}
		} else {
			if d.stage >= dsSeenIHDR {
				break
			}
		}
	}

	var cm color.Model
	switch d.cb {
	case cbG1, cbG2, cbG4, cbG8:
		cm = color.GrayModel
	case cbGA8:
		cm = color.NRGBAModel
	case cbTC8:
		cm = color.RGBAModel
	case cbP1, cbP2, cbP4, cbP8:
		cm = d.palette
	case cbTCA8:
		cm = color.NRGBAModel
	case cbG16:
		cm = color.Gray16Model
	case cbGA16:
		cm = color.NRGBA64Model
	case cbTC16:
		cm = color.RGBA64Model
	case cbTCA16:
		cm = color.NRGBA64Model
	}
	return image.Config{
		ColorModel: cm,
		Width:      d.width,
		Height:     d.height,
	}, nil
}

func init() {
	image.RegisterFormat("png", pngHeader, Decode, DecodeConfig)
}

```

// === FILE: references/go/src/image/png/writer.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package png

import (
	"bufio"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"io"
	"strconv"
)

// Encoder configures encoding PNG images.
type Encoder struct {
	CompressionLevel CompressionLevel

	// BufferPool optionally specifies a buffer pool to get temporary
	// EncoderBuffers when encoding an image.
	BufferPool EncoderBufferPool
}

// EncoderBufferPool is an interface for getting and returning temporary
// instances of the [EncoderBuffer] struct. This can be used to reuse buffers
// when encoding multiple images.
type EncoderBufferPool interface {
	Get() *EncoderBuffer
	Put(*EncoderBuffer)
}

// EncoderBuffer holds the buffers used for encoding PNG images.
type EncoderBuffer encoder

type encoder struct {
	enc     *Encoder
	w       io.Writer
	m       image.Image
	cb      int
	err     error
	header  [8]byte
	footer  [4]byte
	tmp     [4 * 256]byte
	cr      [nFilter][]uint8
	pr      []uint8
	zw      *zlib.Writer
	zwLevel int
	bw      *bufio.Writer
}

// CompressionLevel indicates the compression level.
type CompressionLevel int

const (
	DefaultCompression CompressionLevel = 0
	NoCompression      CompressionLevel = -1
	BestSpeed          CompressionLevel = -2
	BestCompression    CompressionLevel = -3

	// Positive CompressionLevel values are reserved to mean a numeric zlib
	// compression level, although that is not implemented yet.
)

type opaquer interface {
	Opaque() bool
}

// Returns whether or not the image is fully opaque.
func opaque(m image.Image) bool {
	if o, ok := m.(opaquer); ok {
		return o.Opaque()
	}
	b := m.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := m.At(x, y).RGBA()
			if a != 0xffff {
				return false
			}
		}
	}
	return true
}

// The absolute value of a byte interpreted as a signed int8.
func abs8(d uint8) int {
	if d < 128 {
		return int(d)
	}
	return 256 - int(d)
}

func (e *encoder) writeChunk(b []byte, name string) {
	if e.err != nil {
		return
	}
	n := uint32(len(b))
	if int(n) != len(b) {
		e.err = UnsupportedError(name + " chunk is too large: " + strconv.Itoa(len(b)))
		return
	}
	binary.BigEndian.PutUint32(e.header[:4], n)
	e.header[4] = name[0]
	e.header[5] = name[1]
	e.header[6] = name[2]
	e.header[7] = name[3]
	crc := crc32.NewIEEE()
	crc.Write(e.header[4:8])
	crc.Write(b)
	binary.BigEndian.PutUint32(e.footer[:4], crc.Sum32())

	_, e.err = e.w.Write(e.header[:8])
	if e.err != nil {
		return
	}
	_, e.err = e.w.Write(b)
	if e.err != nil {
		return
	}
	_, e.err = e.w.Write(e.footer[:4])
}

func (e *encoder) writeIHDR() {
	b := e.m.Bounds()
	binary.BigEndian.PutUint32(e.tmp[0:4], uint32(b.Dx()))
	binary.BigEndian.PutUint32(e.tmp[4:8], uint32(b.Dy()))
	// Set bit depth and color type.
	switch e.cb {
	case cbG8:
		e.tmp[8] = 8
		e.tmp[9] = ctGrayscale
	case cbTC8:
		e.tmp[8] = 8
		e.tmp[9] = ctTrueColor
	case cbP8:
		e.tmp[8] = 8
		e.tmp[9] = ctPaletted
	case cbP4:
		e.tmp[8] = 4
		e.tmp[9] = ctPaletted
	case cbP2:
		e.tmp[8] = 2
		e.tmp[9] = ctPaletted
	case cbP1:
		e.tmp[8] = 1
		e.tmp[9] = ctPaletted
	case cbTCA8:
		e.tmp[8] = 8
		e.tmp[9] = ctTrueColorAlpha
	case cbG16:
		e.tmp[8] = 16
		e.tmp[9] = ctGrayscale
	case cbTC16:
		e.tmp[8] = 16
		e.tmp[9] = ctTrueColor
	case cbTCA16:
		e.tmp[8] = 16
		e.tmp[9] = ctTrueColorAlpha
	}
	e.tmp[10] = 0 // default compression method
	e.tmp[11] = 0 // default filter method
	e.tmp[12] = 0 // non-interlaced
	e.writeChunk(e.tmp[:13], "IHDR")
}

func (e *encoder) writePLTEAndTRNS(p color.Palette) {
	if len(p) < 1 || len(p) > 256 {
		e.err = FormatError("bad palette length: " + strconv.Itoa(len(p)))
		return
	}
	last := -1
	for i, c := range p {
		c1 := color.NRGBAModel.Convert(c).(color.NRGBA)
		e.tmp[3*i+0] = c1.R
		e.tmp[3*i+1] = c1.G
		e.tmp[3*i+2] = c1.B
		if c1.A != 0xff {
			last = i
		}
		e.tmp[3*256+i] = c1.A
	}
	e.writeChunk(e.tmp[:3*len(p)], "PLTE")
	if last != -1 {
		e.writeChunk(e.tmp[3*256:3*256+1+last], "tRNS")
	}
}

// An encoder is an io.Writer that satisfies writes by writing PNG IDAT chunks,
// including an 8-byte header and 4-byte CRC checksum per Write call. Such calls
// should be relatively infrequent, since writeIDATs uses a [bufio.Writer].
//
// This method should only be called from writeIDATs (via writeImage).
// No other code should treat an encoder as an io.Writer.
func (e *encoder) Write(b []byte) (int, error) {
	e.writeChunk(b, "IDAT")
	if e.err != nil {
		return 0, e.err
	}
	return len(b), nil
}

// Chooses the filter to use for encoding the current row, and applies it.
// The return value is the index of the filter and also of the row in cr that has had it applied.
func filter(cr *[nFilter][]byte, pr []byte, bpp int) int {
	// We try all five filter types, and pick the one that minimizes the sum of absolute differences.
	// This is the same heuristic that libpng uses, although the filters are attempted in order of
	// estimated most likely to be minimal (ftUp, ftPaeth, ftNone, ftSub, ftAverage), rather than
	// in their enumeration order (ftNone, ftSub, ftUp, ftAverage, ftPaeth).
	cdat0 := cr[0][1:]
	cdat1 := cr[1][1:]
	cdat2 := cr[2][1:]
	cdat3 := cr[3][1:]
	cdat4 := cr[4][1:]
	pdat := pr[1:]
	n := len(cdat0)

	// The up filter.
	sum := 0
	for i := 0; i < n; i++ {
		cdat2[i] = cdat0[i] - pdat[i]
		sum += abs8(cdat2[i])
	}
	best := sum
	filter := ftUp

	// The Paeth filter.
	sum = 0
	for i := 0; i < bpp; i++ {
		cdat4[i] = cdat0[i] - pdat[i]
		sum += abs8(cdat4[i])
	}
	for i := bpp; i < n; i++ {
		cdat4[i] = cdat0[i] - paeth(cdat0[i-bpp], pdat[i], pdat[i-bpp])
		sum += abs8(cdat4[i])
		if sum >= best {
			break
		}
	}
	if sum < best {
		best = sum
		filter = ftPaeth
	}

	// The none filter.
	sum = 0
	for i := 0; i < n; i++ {
		sum += abs8(cdat0[i])
		if sum >= best {
			break
		}
	}
	if sum < best {
		best = sum
		filter = ftNone
	}

	// The sub filter.
	sum = 0
	for i := 0; i < bpp; i++ {
		cdat1[i] = cdat0[i]
		sum += abs8(cdat1[i])
	}
	for i := bpp; i < n; i++ {
		cdat1[i] = cdat0[i] - cdat0[i-bpp]
		sum += abs8(cdat1[i])
		if sum >= best {
			break
		}
	}
	if sum < best {
		best = sum
		filter = ftSub
	}

	// The average filter.
	sum = 0
	for i := 0; i < bpp; i++ {
		cdat3[i] = cdat0[i] - pdat[i]/2
		sum += abs8(cdat3[i])
	}
	for i := bpp; i < n; i++ {
		cdat3[i] = cdat0[i] - uint8((int(cdat0[i-bpp])+int(pdat[i]))/2)
		sum += abs8(cdat3[i])
		if sum >= best {
			break
		}
	}
	if sum < best {
		filter = ftAverage
	}

	return filter
}

func (e *encoder) writeImage(w io.Writer, m image.Image, cb int, level int) error {
	if e.zw == nil || e.zwLevel != level {
		zw, err := zlib.NewWriterLevel(w, level)
		if err != nil {
			return err
		}
		e.zw = zw
		e.zwLevel = level
	} else {
		e.zw.Reset(w)
	}
	defer e.zw.Close()

	bitsPerPixel := 0

	switch cb {
	case cbG8:
		bitsPerPixel = 8
	case cbTC8:
		bitsPerPixel = 24
	case cbP8:
		bitsPerPixel = 8
	case cbP4:
		bitsPerPixel = 4
	case cbP2:
		bitsPerPixel = 2
	case cbP1:
		bitsPerPixel = 1
	case cbTCA8:
		bitsPerPixel = 32
	case cbTC16:
		bitsPerPixel = 48
	case cbTCA16:
		bitsPerPixel = 64
	case cbG16:
		bitsPerPixel = 16
	}

	// cr[*] and pr are the bytes for the current and previous row.
	// cr[0] is unfiltered (or equivalently, filtered with the ftNone filter).
	// cr[ft], for non-zero filter types ft, are buffers for transforming cr[0] under the
	// other PNG filter types. These buffers are allocated once and re-used for each row.
	// The +1 is for the per-row filter type, which is at cr[*][0].
	b := m.Bounds()
	sz := 1 + (bitsPerPixel*b.Dx()+7)/8
	for i := range e.cr {
		if cap(e.cr[i]) < sz {
			e.cr[i] = make([]uint8, sz)
		} else {
			e.cr[i] = e.cr[i][:sz]
		}
		e.cr[i][0] = uint8(i)
	}
	cr := e.cr
	if cap(e.pr) < sz {
		e.pr = make([]uint8, sz)
	} else {
		e.pr = e.pr[:sz]
		clear(e.pr)
	}
	pr := e.pr

	gray, _ := m.(*image.Gray)
	rgba, _ := m.(*image.RGBA)
	paletted, _ := m.(*image.Paletted)
	nrgba, _ := m.(*image.NRGBA)

	for y := b.Min.Y; y < b.Max.Y; y++ {
		// Convert from colors to bytes.
		i := 1
		switch cb {
		case cbG8:
			if gray != nil {
				offset := (y - b.Min.Y) * gray.Stride
				copy(cr[0][1:], gray.Pix[offset:offset+b.Dx()])
			} else {
				for x := b.Min.X; x < b.Max.X; x++ {
					c := color.GrayModel.Convert(m.At(x, y)).(color.Gray)
					cr[0][i] = c.Y
					i++
				}
			}
		case cbTC8:
			// We have previously verified that the alpha value is fully opaque.
			cr0 := cr[0]
			stride, pix := 0, []byte(nil)
			if rgba != nil {
				stride, pix = rgba.Stride, rgba.Pix
			} else if nrgba != nil {
				stride, pix = nrgba.Stride, nrgba.Pix
			}
			if stride != 0 {
				j0 := (y - b.Min.Y) * stride
				j1 := j0 + b.Dx()*4
				for j := j0; j < j1; j += 4 {
					cr0[i+0] = pix[j+0]
					cr0[i+1] = pix[j+1]
					cr0[i+2] = pix[j+2]
					i += 3
				}
			} else {
				for x := b.Min.X; x < b.Max.X; x++ {
					r, g, b, _ := m.At(x, y).RGBA()
					cr0[i+0] = uint8(r >> 8)
					cr0[i+1] = uint8(g >> 8)
					cr0[i+2] = uint8(b >> 8)
					i += 3
				}
			}
		case cbP8:
			if paletted != nil {
				offset := (y - b.Min.Y) * paletted.Stride
				copy(cr[0][1:], paletted.Pix[offset:offset+b.Dx()])
			} else {
				pi := m.(image.PalettedImage)
				for x := b.Min.X; x < b.Max.X; x++ {
					cr[0][i] = pi.ColorIndexAt(x, y)
					i += 1
				}
			}

		case cbP4, cbP2, cbP1:
			pi := m.(image.PalettedImage)

			var a uint8
			var c int
			pixelsPerByte := 8 / bitsPerPixel
			for x := b.Min.X; x < b.Max.X; x++ {
				a = a<<uint(bitsPerPixel) | pi.ColorIndexAt(x, y)
				c++
				if c == pixelsPerByte {
					cr[0][i] = a
					i += 1
					a = 0
					c = 0
				}
			}
			if c != 0 {
				for c != pixelsPerByte {
					a = a << uint(bitsPerPixel)
					c++
				}
				cr[0][i] = a
			}

		case cbTCA8:
			if nrgba != nil {
				offset := (y - b.Min.Y) * nrgba.Stride
				copy(cr[0][1:], nrgba.Pix[offset:offset+b.Dx()*4])
			} else if rgba != nil {
				dst := cr[0][1:]
				src := rgba.Pix[rgba.PixOffset(b.Min.X, y):rgba.PixOffset(b.Max.X, y)]
				for ; len(src) >= 4; dst, src = dst[4:], src[4:] {
					d := (*[4]byte)(dst)
					s := (*[4]byte)(src)
					if s[3] == 0x00 {
						d[0] = 0
						d[1] = 0
						d[2] = 0
						d[3] = 0
					} else if s[3] == 0xff {
						copy(d[:], s[:])
					} else {
						// This code does the same as color.NRGBAModel.Convert(
						// rgba.At(x, y)).(color.NRGBA) but with no extra memory
						// allocations or interface/function call overhead.
						//
						// The multiplier m combines 0x101 (which converts
						// 8-bit color to 16-bit color) and 0xffff (which, when
						// combined with the division-by-a, converts from
						// alpha-premultiplied to non-alpha-premultiplied).
						const m = 0x101 * 0xffff
						a := uint32(s[3]) * 0x101
						d[0] = uint8((uint32(s[0]) * m / a) >> 8)
						d[1] = uint8((uint32(s[1]) * m / a) >> 8)
						d[2] = uint8((uint32(s[2]) * m / a) >> 8)
						d[3] = s[3]
					}
				}
			} else {
				// Convert from image.Image (which is alpha-premultiplied) to PNG's non-alpha-premultiplied.
				for x := b.Min.X; x < b.Max.X; x++ {
					c := color.NRGBAModel.Convert(m.At(x, y)).(color.NRGBA)
					cr[0][i+0] = c.R
					cr[0][i+1] = c.G
					cr[0][i+2] = c.B
					cr[0][i+3] = c.A
					i += 4
				}
			}
		case cbG16:
			for x := b.Min.X; x < b.Max.X; x++ {
				c := color.Gray16Model.Convert(m.At(x, y)).(color.Gray16)
				cr[0][i+0] = uint8(c.Y >> 8)
				cr[0][i+1] = uint8(c.Y)
				i += 2
			}
		case cbTC16:
			// We have previously verified that the alpha value is fully opaque.
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, b, _ := m.At(x, y).RGBA()
				cr[0][i+0] = uint8(r >> 8)
				cr[0][i+1] = uint8(r)
				cr[0][i+2] = uint8(g >> 8)
				cr[0][i+3] = uint8(g)
				cr[0][i+4] = uint8(b >> 8)
				cr[0][i+5] = uint8(b)
				i += 6
			}
		case cbTCA16:
			// Convert from image.Image (which is alpha-premultiplied) to PNG's non-alpha-premultiplied.
			for x := b.Min.X; x < b.Max.X; x++ {
				c := color.NRGBA64Model.Convert(m.At(x, y)).(color.NRGBA64)
				cr[0][i+0] = uint8(c.R >> 8)
				cr[0][i+1] = uint8(c.R)
				cr[0][i+2] = uint8(c.G >> 8)
				cr[0][i+3] = uint8(c.G)
				cr[0][i+4] = uint8(c.B >> 8)
				cr[0][i+5] = uint8(c.B)
				cr[0][i+6] = uint8(c.A >> 8)
				cr[0][i+7] = uint8(c.A)
				i += 8
			}
		}

		// Apply the filter.
		// Skip filter for NoCompression and paletted images (cbP8) as
		// "filters are rarely useful on palette images" and will result
		// in larger files (see http://www.libpng.org/pub/png/book/chapter09.html).
		f := ftNone
		if level != zlib.NoCompression && cb != cbP8 && cb != cbP4 && cb != cbP2 && cb != cbP1 {
			// Since we skip paletted images we don't have to worry about
			// bitsPerPixel not being a multiple of 8
			bpp := bitsPerPixel / 8
			f = filter(&cr, pr, bpp)
		}

		// Write the compressed bytes.
		if _, err := e.zw.Write(cr[f]); err != nil {
			return err
		}

		// The current row for y is the previous row for y+1.
		pr, cr[0] = cr[0], pr
	}
	return nil
}

// Write the actual image data to one or more IDAT chunks.
func (e *encoder) writeIDATs() {
	if e.err != nil {
		return
	}
	if e.bw == nil {
		e.bw = bufio.NewWriterSize(e, 1<<15)
	} else {
		e.bw.Reset(e)
	}
	e.err = e.writeImage(e.bw, e.m, e.cb, levelToZlib(e.enc.CompressionLevel))
	if e.err != nil {
		return
	}
	e.err = e.bw.Flush()
}

// This function is required because we want the zero value of
// Encoder.CompressionLevel to map to zlib.DefaultCompression.
func levelToZlib(l CompressionLevel) int {
	switch l {
	case DefaultCompression:
		return zlib.DefaultCompression
	case NoCompression:
		return zlib.NoCompression
	case BestSpeed:
		return zlib.BestSpeed
	case BestCompression:
		return zlib.BestCompression
	default:
		return zlib.DefaultCompression
	}
}

func (e *encoder) writeIEND() { e.writeChunk(nil, "IEND") }

// Encode writes the Image m to w in PNG format. Any Image may be
// encoded, but images that are not [image.NRGBA] might be encoded lossily.
//
// Note that the exact bytes written to w are not covered by the Go 1
// compatibility promise. Callers, including tests, should not depend on the
// exact written bytes.
func Encode(w io.Writer, m image.Image) error {
	var e Encoder
	return e.Encode(w, m)
}

// Encode writes the Image m to w in PNG format.
//
// Note that the exact bytes written to w are not covered by the Go 1
// compatibility promise. Callers, including tests, should not depend on the
// exact written bytes.
func (enc *Encoder) Encode(w io.Writer, m image.Image) error {
	// Obviously, negative widths and heights are invalid. Furthermore, the PNG
	// spec section 11.2.2 says that zero is invalid. Excessively large images are
	// also rejected.
	mw, mh := int64(m.Bounds().Dx()), int64(m.Bounds().Dy())
	if mw <= 0 || mh <= 0 || mw >= 1<<32 || mh >= 1<<32 {
		return FormatError("invalid image size: " + strconv.FormatInt(mw, 10) + "x" + strconv.FormatInt(mh, 10))
	}

	var e *encoder
	if enc.BufferPool != nil {
		buffer := enc.BufferPool.Get()
		e = (*encoder)(buffer)

	}
	if e == nil {
		e = &encoder{}
	}
	if enc.BufferPool != nil {
		defer enc.BufferPool.Put((*EncoderBuffer)(e))
	}

	e.enc = enc
	e.w = w
	e.m = m

	var pal color.Palette
	// cbP8 encoding needs PalettedImage's ColorIndexAt method.
	if _, ok := m.(image.PalettedImage); ok {
		pal, _ = m.ColorModel().(color.Palette)
	}
	if pal != nil {
		if len(pal) <= 2 {
			e.cb = cbP1
		} else if len(pal) <= 4 {
			e.cb = cbP2
		} else if len(pal) <= 16 {
			e.cb = cbP4
		} else {
			e.cb = cbP8
		}
	} else {
		switch m.ColorModel() {
		case color.GrayModel:
			e.cb = cbG8
		case color.Gray16Model:
			e.cb = cbG16
		case color.RGBAModel, color.NRGBAModel, color.AlphaModel:
			if opaque(m) {
				e.cb = cbTC8
			} else {
				e.cb = cbTCA8
			}
		default:
			if opaque(m) {
				e.cb = cbTC16
			} else {
				e.cb = cbTCA16
			}
		}
	}

	_, e.err = io.WriteString(w, pngHeader)
	e.writeIHDR()
	if pal != nil {
		e.writePLTEAndTRNS(pal)
	}
	e.writeIDATs()
	e.writeIEND()
	return e.err
}

```

// === FILE: references/go/src/image/ycbcr.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package image

import (
	"image/color"
)

// YCbCrSubsampleRatio is the chroma subsample ratio used in a YCbCr image.
type YCbCrSubsampleRatio int

const (
	YCbCrSubsampleRatio444 YCbCrSubsampleRatio = iota
	YCbCrSubsampleRatio422
	YCbCrSubsampleRatio420
	YCbCrSubsampleRatio440
	YCbCrSubsampleRatio411
	YCbCrSubsampleRatio410
)

func (s YCbCrSubsampleRatio) String() string {
	switch s {
	case YCbCrSubsampleRatio444:
		return "YCbCrSubsampleRatio444"
	case YCbCrSubsampleRatio422:
		return "YCbCrSubsampleRatio422"
	case YCbCrSubsampleRatio420:
		return "YCbCrSubsampleRatio420"
	case YCbCrSubsampleRatio440:
		return "YCbCrSubsampleRatio440"
	case YCbCrSubsampleRatio411:
		return "YCbCrSubsampleRatio411"
	case YCbCrSubsampleRatio410:
		return "YCbCrSubsampleRatio410"
	}
	return "YCbCrSubsampleRatioUnknown"
}

// YCbCr is an in-memory image of Y'CbCr colors. There is one Y sample per
// pixel, but each Cb and Cr sample can span one or more pixels.
// YStride is the Y slice index delta between vertically adjacent pixels.
// CStride is the Cb and Cr slice index delta between vertically adjacent pixels
// that map to separate chroma samples.
// It is not an absolute requirement, but YStride and len(Y) are typically
// multiples of 8, and:
//
//	For 4:4:4, CStride == YStride/1 && len(Cb) == len(Cr) == len(Y)/1.
//	For 4:2:2, CStride == YStride/2 && len(Cb) == len(Cr) == len(Y)/2.
//	For 4:2:0, CStride == YStride/2 && len(Cb) == len(Cr) == len(Y)/4.
//	For 4:4:0, CStride == YStride/1 && len(Cb) == len(Cr) == len(Y)/2.
//	For 4:1:1, CStride == YStride/4 && len(Cb) == len(Cr) == len(Y)/4.
//	For 4:1:0, CStride == YStride/4 && len(Cb) == len(Cr) == len(Y)/8.
type YCbCr struct {
	Y, Cb, Cr      []uint8
	YStride        int
	CStride        int
	SubsampleRatio YCbCrSubsampleRatio
	Rect           Rectangle
}

func (p *YCbCr) ColorModel() color.Model {
	return color.YCbCrModel
}

func (p *YCbCr) Bounds() Rectangle {
	return p.Rect
}

func (p *YCbCr) At(x, y int) color.Color {
	return p.YCbCrAt(x, y)
}

func (p *YCbCr) RGBA64At(x, y int) color.RGBA64 {
	r, g, b, a := p.YCbCrAt(x, y).RGBA()
	return color.RGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
}

func (p *YCbCr) YCbCrAt(x, y int) color.YCbCr {
	if !(Point{x, y}.In(p.Rect)) {
		return color.YCbCr{}
	}
	yi := p.YOffset(x, y)
	ci := p.COffset(x, y)
	return color.YCbCr{
		p.Y[yi],
		p.Cb[ci],
		p.Cr[ci],
	}
}

// YOffset returns the index of the first element of Y that corresponds to
// the pixel at (x, y).
func (p *YCbCr) YOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.YStride + (x - p.Rect.Min.X)
}

// COffset returns the index of the first element of Cb or Cr that corresponds
// to the pixel at (x, y).
func (p *YCbCr) COffset(x, y int) int {
	switch p.SubsampleRatio {
	case YCbCrSubsampleRatio422:
		return (y-p.Rect.Min.Y)*p.CStride + (x/2 - p.Rect.Min.X/2)
	case YCbCrSubsampleRatio420:
		return (y/2-p.Rect.Min.Y/2)*p.CStride + (x/2 - p.Rect.Min.X/2)
	case YCbCrSubsampleRatio440:
		return (y/2-p.Rect.Min.Y/2)*p.CStride + (x - p.Rect.Min.X)
	case YCbCrSubsampleRatio411:
		return (y-p.Rect.Min.Y)*p.CStride + (x/4 - p.Rect.Min.X/4)
	case YCbCrSubsampleRatio410:
		return (y/2-p.Rect.Min.Y/2)*p.CStride + (x/4 - p.Rect.Min.X/4)
	}
	// Default to 4:4:4 subsampling.
	return (y-p.Rect.Min.Y)*p.CStride + (x - p.Rect.Min.X)
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *YCbCr) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &YCbCr{
			SubsampleRatio: p.SubsampleRatio,
		}
	}
	yi := p.YOffset(r.Min.X, r.Min.Y)
	ci := p.COffset(r.Min.X, r.Min.Y)
	return &YCbCr{
		Y:              p.Y[yi:],
		Cb:             p.Cb[ci:],
		Cr:             p.Cr[ci:],
		SubsampleRatio: p.SubsampleRatio,
		YStride:        p.YStride,
		CStride:        p.CStride,
		Rect:           r,
	}
}

func (p *YCbCr) Opaque() bool {
	return true
}

func yCbCrSize(r Rectangle, subsampleRatio YCbCrSubsampleRatio) (w, h, cw, ch int) {
	w, h = r.Dx(), r.Dy()
	switch subsampleRatio {
	case YCbCrSubsampleRatio422:
		cw = (r.Max.X+1)/2 - r.Min.X/2
		ch = h
	case YCbCrSubsampleRatio420:
		cw = (r.Max.X+1)/2 - r.Min.X/2
		ch = (r.Max.Y+1)/2 - r.Min.Y/2
	case YCbCrSubsampleRatio440:
		cw = w
		ch = (r.Max.Y+1)/2 - r.Min.Y/2
	case YCbCrSubsampleRatio411:
		cw = (r.Max.X+3)/4 - r.Min.X/4
		ch = h
	case YCbCrSubsampleRatio410:
		cw = (r.Max.X+3)/4 - r.Min.X/4
		ch = (r.Max.Y+1)/2 - r.Min.Y/2
	default:
		// Default to 4:4:4 subsampling.
		cw = w
		ch = h
	}
	return
}

// NewYCbCr returns a new YCbCr image with the given bounds and subsample
// ratio.
func NewYCbCr(r Rectangle, subsampleRatio YCbCrSubsampleRatio) *YCbCr {
	w, h, cw, ch := yCbCrSize(r, subsampleRatio)

	// totalLength should be the same as i2, below, for a valid Rectangle r.
	totalLength := add2NonNeg(
		mul3NonNeg(1, w, h),
		mul3NonNeg(2, cw, ch),
	)
	if totalLength < 0 {
		panic("image: NewYCbCr Rectangle has huge or negative dimensions")
	}

	i0 := w*h + 0*cw*ch
	i1 := w*h + 1*cw*ch
	i2 := w*h + 2*cw*ch
	b := make([]byte, i2)
	return &YCbCr{
		Y:              b[:i0:i0],
		Cb:             b[i0:i1:i1],
		Cr:             b[i1:i2:i2],
		SubsampleRatio: subsampleRatio,
		YStride:        w,
		CStride:        cw,
		Rect:           r,
	}
}

// NYCbCrA is an in-memory image of non-alpha-premultiplied Y'CbCr-with-alpha
// colors. A and AStride are analogous to the Y and YStride fields of the
// embedded YCbCr.
type NYCbCrA struct {
	YCbCr
	A       []uint8
	AStride int
}

func (p *NYCbCrA) ColorModel() color.Model {
	return color.NYCbCrAModel
}

func (p *NYCbCrA) At(x, y int) color.Color {
	return p.NYCbCrAAt(x, y)
}

func (p *NYCbCrA) RGBA64At(x, y int) color.RGBA64 {
	r, g, b, a := p.NYCbCrAAt(x, y).RGBA()
	return color.RGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
}

func (p *NYCbCrA) NYCbCrAAt(x, y int) color.NYCbCrA {
	if !(Point{X: x, Y: y}.In(p.Rect)) {
		return color.NYCbCrA{}
	}
	yi := p.YOffset(x, y)
	ci := p.COffset(x, y)
	ai := p.AOffset(x, y)
	return color.NYCbCrA{
		color.YCbCr{
			Y:  p.Y[yi],
			Cb: p.Cb[ci],
			Cr: p.Cr[ci],
		},
		p.A[ai],
	}
}

// AOffset returns the index of the first element of A that corresponds to the
// pixel at (x, y).
func (p *NYCbCrA) AOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.AStride + (x - p.Rect.Min.X)
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value shares pixels with the original image.
func (p *NYCbCrA) SubImage(r Rectangle) Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be inside
	// either r1 or r2 if the intersection is empty. Without explicitly checking for
	// this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &NYCbCrA{
			YCbCr: YCbCr{
				SubsampleRatio: p.SubsampleRatio,
			},
		}
	}
	yi := p.YOffset(r.Min.X, r.Min.Y)
	ci := p.COffset(r.Min.X, r.Min.Y)
	ai := p.AOffset(r.Min.X, r.Min.Y)
	return &NYCbCrA{
		YCbCr: YCbCr{
			Y:              p.Y[yi:],
			Cb:             p.Cb[ci:],
			Cr:             p.Cr[ci:],
			SubsampleRatio: p.SubsampleRatio,
			YStride:        p.YStride,
			CStride:        p.CStride,
			Rect:           r,
		},
		A:       p.A[ai:],
		AStride: p.AStride,
	}
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (p *NYCbCrA) Opaque() bool {
	if p.Rect.Empty() {
		return true
	}
	i0, i1 := 0, p.Rect.Dx()
	for y := p.Rect.Min.Y; y < p.Rect.Max.Y; y++ {
		for _, a := range p.A[i0:i1] {
			if a != 0xff {
				return false
			}
		}
		i0 += p.AStride
		i1 += p.AStride
	}
	return true
}

// NewNYCbCrA returns a new [NYCbCrA] image with the given bounds and subsample
// ratio.
func NewNYCbCrA(r Rectangle, subsampleRatio YCbCrSubsampleRatio) *NYCbCrA {
	w, h, cw, ch := yCbCrSize(r, subsampleRatio)

	// totalLength should be the same as i3, below, for a valid Rectangle r.
	totalLength := add2NonNeg(
		mul3NonNeg(2, w, h),
		mul3NonNeg(2, cw, ch),
	)
	if totalLength < 0 {
		panic("image: NewNYCbCrA Rectangle has huge or negative dimension")
	}

	i0 := 1*w*h + 0*cw*ch
	i1 := 1*w*h + 1*cw*ch
	i2 := 1*w*h + 2*cw*ch
	i3 := 2*w*h + 2*cw*ch
	b := make([]byte, i3)
	return &NYCbCrA{
		YCbCr: YCbCr{
			Y:              b[:i0:i0],
			Cb:             b[i0:i1:i1],
			Cr:             b[i1:i2:i2],
			SubsampleRatio: subsampleRatio,
			YStride:        w,
			CStride:        cw,
			Rect:           r,
		},
		A:       b[i2:],
		AStride: w,
	}
}

```

