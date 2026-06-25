# Domain Architecture: time

## Layout Topology
```text
time/
├── tzdata
│   └── tzdata.go
├── embed.go
├── format.go
├── format_rfc3339.go
├── genzabbrs.go
├── sleep.go
├── sys_plan9.go
├── sys_unix.go
├── sys_windows.go
├── tick.go
├── time.go
├── zoneinfo.go
├── zoneinfo_abbrs_windows.go
├── zoneinfo_android.go
├── zoneinfo_goroot.go
├── zoneinfo_ios.go
├── zoneinfo_js.go
├── zoneinfo_plan9.go
├── zoneinfo_read.go
├── zoneinfo_unix.go
├── zoneinfo_wasip1.go
└── zoneinfo_windows.go
```

## Source Stream Aggregation

// === FILE: references!/go/src/time/embed.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file is used with build tag timetzdata to embed tzdata into
// the binary.

//go:build timetzdata

package time

import _ "time/tzdata"

```

// === FILE: references!/go/src/time/format.go ===
```go
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import (
	"errors"
	"internal/stringslite"
	_ "unsafe" // for linkname
)

// These are predefined layouts for use in [Time.Format] and [time.Parse].
// The reference time used in these layouts is the specific time stamp:
//
//	01/02 03:04:05PM '06 -0700
//
// (January 2, 15:04:05, 2006, in time zone seven hours west of GMT).
// That value is recorded as the constant named [Layout], listed below. As a Unix
// time, this is 1136239445. Since MST is GMT-0700, the reference would be
// printed by the Unix date command as:
//
//	Mon Jan 2 15:04:05 MST 2006
//
// It is a regrettable historic error that the date uses the American convention
// of putting the numerical month before the day.
//
// The example for Time.Format demonstrates the working of the layout string
// in detail and is a good reference.
//
// Note that the [RFC822], [RFC850], and [RFC1123] formats should be applied
// only to local times. Applying them to UTC times will use "UTC" as the
// time zone abbreviation, while strictly speaking those RFCs require the
// use of "GMT" in that case.
// When using the [RFC1123] or [RFC1123Z] formats for parsing, note that these
// formats define a leading zero for the day-in-month portion, which is not
// strictly allowed by RFC 1123. This will result in an error when parsing
// date strings that occur in the first 9 days of a given month.
// In general [RFC1123Z] should be used instead of [RFC1123] for servers
// that insist on that format, and [RFC3339] should be preferred for new protocols.
// [RFC3339], [RFC822], [RFC822Z], [RFC1123], and [RFC1123Z] are useful for formatting;
// when used with time.Parse they do not accept all the time formats
// permitted by the RFCs and they do accept time formats not formally defined.
// The [RFC3339Nano] format removes trailing zeros from the seconds field
// and thus may not sort correctly once formatted.
//
// Most programs can use one of the defined constants as the layout passed to
// Format or Parse. The rest of this comment can be ignored unless you are
// creating a custom layout string.
//
// To define your own format, write down what the reference time would look like
// formatted your way; see the values of constants like [ANSIC], [StampMicro] or
// [Kitchen] for examples. The model is to demonstrate what the reference time
// looks like so that the Format and Parse methods can apply the same
// transformation to a general time value.
//
// Here is a summary of the components of a layout string. Each element shows by
// example the formatting of an element of the reference time. Only these values
// are recognized. Text in the layout string that is not recognized as part of
// the reference time is echoed verbatim during Format and expected to appear
// verbatim in the input to Parse.
//
//	Year: "2006" "06"
//	Month: "Jan" "January" "01" "1"
//	Day of the week: "Mon" "Monday"
//	Day of the month: "2" "_2" "02"
//	Day of the year: "__2" "002"
//	Hour: "15" "3" "03" (PM or AM)
//	Minute: "4" "04"
//	Second: "5" "05"
//	AM/PM mark: "PM"
//
// Numeric time zone offsets format as follows:
//
//	"-0700"     ±hhmm
//	"-07:00"    ±hh:mm
//	"-07"       ±hh
//	"-070000"   ±hhmmss
//	"-07:00:00" ±hh:mm:ss
//
// Replacing the sign in the format with a Z triggers
// the ISO 8601 behavior of printing Z instead of an
// offset for the UTC zone. Thus:
//
//	"Z0700"      Z or ±hhmm
//	"Z07:00"     Z or ±hh:mm
//	"Z07"        Z or ±hh
//	"Z070000"    Z or ±hhmmss
//	"Z07:00:00"  Z or ±hh:mm:ss
//
// Within the format string, the underscores in "_2" and "__2" represent spaces
// that may be replaced by digits if the following number has multiple digits,
// for compatibility with fixed-width Unix time formats. A leading zero represents
// a zero-padded value.
//
// The formats __2 and 002 are space-padded and zero-padded
// three-character day of year; there is no unpadded day of year format.
//
// A comma or decimal point followed by one or more zeros represents
// a fractional second, printed to the given number of decimal places.
// A comma or decimal point followed by one or more nines represents
// a fractional second, printed to the given number of decimal places, with
// trailing zeros removed.
// For example "15:04:05,000" or "15:04:05.000" formats or parses with
// millisecond precision.
//
// Some valid layouts are invalid time values for time.Parse, due to formats
// such as _ for space padding and Z for zone information.
const (
	Layout      = "01/02 03:04:05PM '06 -0700" // The reference time, in numerical order.
	ANSIC       = "Mon Jan _2 15:04:05 2006"
	UnixDate    = "Mon Jan _2 15:04:05 MST 2006"
	RubyDate    = "Mon Jan 02 15:04:05 -0700 2006"
	RFC822      = "02 Jan 06 15:04 MST"
	RFC822Z     = "02 Jan 06 15:04 -0700" // RFC822 with numeric zone
	RFC850      = "Monday, 02-Jan-06 15:04:05 MST"
	RFC1123     = "Mon, 02 Jan 2006 15:04:05 MST"
	RFC1123Z    = "Mon, 02 Jan 2006 15:04:05 -0700" // RFC1123 with numeric zone
	RFC3339     = "2006-01-02T15:04:05Z07:00"
	RFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"
	Kitchen     = "3:04PM"
	// Handy time stamps.
	Stamp      = "Jan _2 15:04:05"
	StampMilli = "Jan _2 15:04:05.000"
	StampMicro = "Jan _2 15:04:05.000000"
	StampNano  = "Jan _2 15:04:05.000000000"
	DateTime   = "2006-01-02 15:04:05"
	DateOnly   = "2006-01-02"
	TimeOnly   = "15:04:05"
)

const (
	_                        = iota
	stdLongMonth             = iota + stdNeedDate  // "January"
	stdMonth                                       // "Jan"
	stdNumMonth                                    // "1"
	stdZeroMonth                                   // "01"
	stdLongWeekDay                                 // "Monday"
	stdWeekDay                                     // "Mon"
	stdDay                                         // "2"
	stdUnderDay                                    // "_2"
	stdZeroDay                                     // "02"
	stdUnderYearDay          = iota + stdNeedYday  // "__2"
	stdZeroYearDay                                 // "002"
	stdHour                  = iota + stdNeedClock // "15"
	stdHour12                                      // "3"
	stdZeroHour12                                  // "03"
	stdMinute                                      // "4"
	stdZeroMinute                                  // "04"
	stdSecond                                      // "5"
	stdZeroSecond                                  // "05"
	stdLongYear              = iota + stdNeedDate  // "2006"
	stdYear                                        // "06"
	stdPM                    = iota + stdNeedClock // "PM"
	stdpm                                          // "pm"
	stdTZ                    = iota                // "MST"
	stdISO8601TZ                                   // "Z0700"  // prints Z for UTC
	stdISO8601SecondsTZ                            // "Z070000"
	stdISO8601ShortTZ                              // "Z07"
	stdISO8601ColonTZ                              // "Z07:00" // prints Z for UTC
	stdISO8601ColonSecondsTZ                       // "Z07:00:00"
	stdNumTZ                                       // "-0700"  // always numeric
	stdNumSecondsTz                                // "-070000"
	stdNumShortTZ                                  // "-07"    // always numeric
	stdNumColonTZ                                  // "-07:00" // always numeric
	stdNumColonSecondsTZ                           // "-07:00:00"
	stdFracSecond0                                 // ".0", ".00", ... , trailing zeros included
	stdFracSecond9                                 // ".9", ".99", ..., trailing zeros omitted

	stdNeedDate       = 1 << 8             // need month, day, year
	stdNeedYday       = 1 << 9             // need yday
	stdNeedClock      = 1 << 10            // need hour, minute, second
	stdArgShift       = 16                 // extra argument in high bits, above low stdArgShift
	stdSeparatorShift = 28                 // extra argument in high 4 bits for fractional second separators
	stdMask           = 1<<stdArgShift - 1 // mask out argument
)

// std0x records the std values for "01", "02", ..., "06".
var std0x = [...]int{stdZeroMonth, stdZeroDay, stdZeroHour12, stdZeroMinute, stdZeroSecond, stdYear}

// startsWithLowerCase reports whether the string has a lower-case letter at the beginning.
// Its purpose is to prevent matching strings like "Month" when looking for "Mon".
func startsWithLowerCase(str string) bool {
	if len(str) == 0 {
		return false
	}
	c := str[0]
	return 'a' <= c && c <= 'z'
}

// nextStdChunk finds the first occurrence of a std string in
// layout and returns the text before, the std string, and the text after.
//
// nextStdChunk should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - github.com/searKing/golang/go
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
//
//go:linkname nextStdChunk
func nextStdChunk(layout string) (prefix string, std int, suffix string) {
	for i := 0; i < len(layout); i++ {
		switch c := int(layout[i]); c {
		case 'J': // January, Jan
			if len(layout) >= i+3 && layout[i:i+3] == "Jan" {
				if len(layout) >= i+7 && layout[i:i+7] == "January" {
					return layout[0:i], stdLongMonth, layout[i+7:]
				}
				if !startsWithLowerCase(layout[i+3:]) {
					return layout[0:i], stdMonth, layout[i+3:]
				}
			}

		case 'M': // Monday, Mon, MST
			if len(layout) >= i+3 {
				if layout[i:i+3] == "Mon" {
					if len(layout) >= i+6 && layout[i:i+6] == "Monday" {
						return layout[0:i], stdLongWeekDay, layout[i+6:]
					}
					if !startsWithLowerCase(layout[i+3:]) {
						return layout[0:i], stdWeekDay, layout[i+3:]
					}
				}
				if layout[i:i+3] == "MST" {
					return layout[0:i], stdTZ, layout[i+3:]
				}
			}

		case '0': // 01, 02, 03, 04, 05, 06, 002
			if len(layout) >= i+2 && '1' <= layout[i+1] && layout[i+1] <= '6' {
				return layout[0:i], std0x[layout[i+1]-'1'], layout[i+2:]
			}
			if len(layout) >= i+3 && layout[i+1] == '0' && layout[i+2] == '2' {
				return layout[0:i], stdZeroYearDay, layout[i+3:]
			}

		case '1': // 15, 1
			if len(layout) >= i+2 && layout[i+1] == '5' {
				return layout[0:i], stdHour, layout[i+2:]
			}
			return layout[0:i], stdNumMonth, layout[i+1:]

		case '2': // 2006, 2
			if len(layout) >= i+4 && layout[i:i+4] == "2006" {
				return layout[0:i], stdLongYear, layout[i+4:]
			}
			return layout[0:i], stdDay, layout[i+1:]

		case '_': // _2, _2006, __2
			if len(layout) >= i+2 && layout[i+1] == '2' {
				// _2006 is really a literal _, followed by stdLongYear
				if len(layout) >= i+5 && layout[i+1:i+5] == "2006" {
					return layout[0 : i+1], stdLongYear, layout[i+5:]
				}
				return layout[0:i], stdUnderDay, layout[i+2:]
			}
			if len(layout) >= i+3 && layout[i+1] == '_' && layout[i+2] == '2' {
				return layout[0:i], stdUnderYearDay, layout[i+3:]
			}

		case '3':
			return layout[0:i], stdHour12, layout[i+1:]

		case '4':
			return layout[0:i], stdMinute, layout[i+1:]

		case '5':
			return layout[0:i], stdSecond, layout[i+1:]

		case 'P': // PM
			if len(layout) >= i+2 && layout[i+1] == 'M' {
				return layout[0:i], stdPM, layout[i+2:]
			}

		case 'p': // pm
			if len(layout) >= i+2 && layout[i+1] == 'm' {
				return layout[0:i], stdpm, layout[i+2:]
			}

		case '-': // -070000, -07:00:00, -0700, -07:00, -07
			if len(layout) >= i+7 && layout[i:i+7] == "-070000" {
				return layout[0:i], stdNumSecondsTz, layout[i+7:]
			}
			if len(layout) >= i+9 && layout[i:i+9] == "-07:00:00" {
				return layout[0:i], stdNumColonSecondsTZ, layout[i+9:]
			}
			if len(layout) >= i+5 && layout[i:i+5] == "-0700" {
				return layout[0:i], stdNumTZ, layout[i+5:]
			}
			if len(layout) >= i+6 && layout[i:i+6] == "-07:00" {
				return layout[0:i], stdNumColonTZ, layout[i+6:]
			}
			if len(layout) >= i+3 && layout[i:i+3] == "-07" {
				return layout[0:i], stdNumShortTZ, layout[i+3:]
			}

		case 'Z': // Z070000, Z07:00:00, Z0700, Z07:00,
			if len(layout) >= i+7 && layout[i:i+7] == "Z070000" {
				return layout[0:i], stdISO8601SecondsTZ, layout[i+7:]
			}
			if len(layout) >= i+9 && layout[i:i+9] == "Z07:00:00" {
				return layout[0:i], stdISO8601ColonSecondsTZ, layout[i+9:]
			}
			if len(layout) >= i+5 && layout[i:i+5] == "Z0700" {
				return layout[0:i], stdISO8601TZ, layout[i+5:]
			}
			if len(layout) >= i+6 && layout[i:i+6] == "Z07:00" {
				return layout[0:i], stdISO8601ColonTZ, layout[i+6:]
			}
			if len(layout) >= i+3 && layout[i:i+3] == "Z07" {
				return layout[0:i], stdISO8601ShortTZ, layout[i+3:]
			}

		case '.', ',': // ,000, or .000, or ,999, or .999 - repeated digits for fractional seconds.
			if i+1 < len(layout) && (layout[i+1] == '0' || layout[i+1] == '9') {
				ch := layout[i+1]
				j := i + 1
				for j < len(layout) && layout[j] == ch {
					j++
				}
				// String of digits must end here - only fractional second is all digits.
				if !isDigit(layout, j) {
					code := stdFracSecond0
					if layout[i+1] == '9' {
						code = stdFracSecond9
					}
					std := stdFracSecond(code, j-(i+1), c)
					return layout[0:i], std, layout[j:]
				}
			}
		}
	}
	return layout, 0, ""
}

var longDayNames = []string{
	"Sunday",
	"Monday",
	"Tuesday",
	"Wednesday",
	"Thursday",
	"Friday",
	"Saturday",
}

var shortDayNames = []string{
	"Sun",
	"Mon",
	"Tue",
	"Wed",
	"Thu",
	"Fri",
	"Sat",
}

var shortMonthNames = []string{
	"Jan",
	"Feb",
	"Mar",
	"Apr",
	"May",
	"Jun",
	"Jul",
	"Aug",
	"Sep",
	"Oct",
	"Nov",
	"Dec",
}

var longMonthNames = []string{
	"January",
	"February",
	"March",
	"April",
	"May",
	"June",
	"July",
	"August",
	"September",
	"October",
	"November",
	"December",
}

// match reports whether s1 and s2 match ignoring case.
// It is assumed s1 and s2 are the same length.
func match(s1, s2 string) bool {
	for i := 0; i < len(s1); i++ {
		c1 := s1[i]
		c2 := s2[i]
		if c1 != c2 {
			// Switch to lower-case; 'a'-'A' is known to be a single bit.
			c1 |= 'a' - 'A'
			c2 |= 'a' - 'A'
			if c1 != c2 || c1 < 'a' || c1 > 'z' {
				return false
			}
		}
	}
	return true
}

func lookup(tab []string, val string) (int, string, error) {
	for i, v := range tab {
		if len(val) >= len(v) && match(val[:len(v)], v) {
			return i, val[len(v):], nil
		}
	}
	return -1, val, errBad
}

// appendInt appends the decimal form of x to b and returns the result.
// If the decimal form (excluding sign) is shorter than width, the result is padded with leading 0's.
// Duplicates functionality in strconv, but avoids dependency.
func appendInt(b []byte, x int, width int) []byte {
	u := uint(x)
	if x < 0 {
		b = append(b, '-')
		u = uint(-x)
	}

	// 2-digit and 4-digit fields are the most common in time formats.
	utod := func(u uint) byte { return '0' + byte(u) }
	switch {
	case width == 2 && u < 1e2:
		return append(b, utod(u/1e1), utod(u%1e1))
	case width == 4 && u < 1e4:
		return append(b, utod(u/1e3), utod(u/1e2%1e1), utod(u/1e1%1e1), utod(u%1e1))
	}

	// Compute the number of decimal digits.
	var n int
	if u == 0 {
		n = 1
	}
	for u2 := u; u2 > 0; u2 /= 10 {
		n++
	}

	// Add 0-padding.
	for pad := width - n; pad > 0; pad-- {
		b = append(b, '0')
	}

	// Ensure capacity.
	if len(b)+n <= cap(b) {
		b = b[:len(b)+n]
	} else {
		b = append(b, make([]byte, n)...)
	}

	// Assemble decimal in reverse order.
	i := len(b) - 1
	for u >= 10 && i > 0 {
		q := u / 10
		b[i] = utod(u - q*10)
		u = q
		i--
	}
	b[i] = utod(u)
	return b
}

// Never printed, just needs to be non-nil for return by atoi.
var errAtoi = errors.New("time: invalid number")

// Duplicates functionality in strconv, but avoids dependency.
func atoi[bytes []byte | string](s bytes) (x int, err error) {
	neg := false
	if len(s) > 0 && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s = s[1:]
	}
	q, rem, err := leadingInt(s)
	x = int(q)
	if err != nil || len(rem) > 0 {
		return 0, errAtoi
	}
	if neg {
		x = -x
	}
	return x, nil
}

// The "std" value passed to appendNano contains two packed fields: the number of
// digits after the decimal and the separator character (period or comma).
// These functions pack and unpack that variable.
func stdFracSecond(code, n, c int) int {
	// Use 0xfff to make the failure case even more absurd.
	if c == '.' {
		return code | ((n & 0xfff) << stdArgShift)
	}
	return code | ((n & 0xfff) << stdArgShift) | 1<<stdSeparatorShift
}

func digitsLen(std int) int {
	return (std >> stdArgShift) & 0xfff
}

func separator(std int) byte {
	if (std >> stdSeparatorShift) == 0 {
		return '.'
	}
	return ','
}

// appendNano appends a fractional second, as nanoseconds, to b
// and returns the result. The nanosec must be within [0, 999999999].
func appendNano(b []byte, nanosec int, std int) []byte {
	trim := std&stdMask == stdFracSecond9
	n := digitsLen(std)
	if trim && (n == 0 || nanosec == 0) {
		return b
	}
	dot := separator(std)
	b = append(b, dot)
	b = appendInt(b, nanosec, 9)
	if n < 9 {
		b = b[:len(b)-9+n]
	}
	if trim {
		for len(b) > 0 && b[len(b)-1] == '0' {
			b = b[:len(b)-1]
		}
		if len(b) > 0 && b[len(b)-1] == dot {
			b = b[:len(b)-1]
		}
	}
	return b
}

// String returns the time formatted using the format string
//
//	"2006-01-02 15:04:05.999999999 -0700 MST"
//
// If the time has a monotonic clock reading, the returned string
// includes a final field "m=±<value>", where value is the monotonic
// clock reading formatted as a decimal number of seconds.
//
// The returned string is meant for debugging; for a stable serialized
// representation, use t.MarshalText, t.MarshalBinary, or t.Format
// with an explicit format string.
func (t Time) String() string {
	s := t.Format("2006-01-02 15:04:05.999999999 -0700 MST")

	// Format monotonic clock reading as m=±ddd.nnnnnnnnn.
	if t.wall&hasMonotonic != 0 {
		m2 := uint64(t.ext)
		sign := byte('+')
		if t.ext < 0 {
			sign = '-'
			m2 = -m2
		}
		m1, m2 := m2/1e9, m2%1e9
		m0, m1 := m1/1e9, m1%1e9
		buf := make([]byte, 0, 24)
		buf = append(buf, " m="...)
		buf = append(buf, sign)
		wid := 0
		if m0 != 0 {
			buf = appendInt(buf, int(m0), 0)
			wid = 9
		}
		buf = appendInt(buf, int(m1), wid)
		buf = append(buf, '.')
		buf = appendInt(buf, int(m2), 9)
		s += string(buf)
	}
	return s
}

// GoString implements [fmt.GoStringer] and formats t to be printed in Go source
// code.
func (t Time) GoString() string {
	abs := t.absSec()
	year, month, day := abs.days().date()
	hour, minute, second := abs.clock()

	buf := make([]byte, 0, len("time.Date(9999, time.September, 31, 23, 59, 59, 999999999, time.Local)"))
	buf = append(buf, "time.Date("...)
	buf = appendInt(buf, year, 0)
	if January <= month && month <= December {
		buf = append(buf, ", time."...)
		buf = append(buf, longMonthNames[month-1]...)
	} else {
		// It's difficult to construct a time.Time with a date outside the
		// standard range but we might as well try to handle the case.
		buf = appendInt(buf, int(month), 0)
	}
	buf = append(buf, ", "...)
	buf = appendInt(buf, day, 0)
	buf = append(buf, ", "...)
	buf = appendInt(buf, hour, 0)
	buf = append(buf, ", "...)
	buf = appendInt(buf, minute, 0)
	buf = append(buf, ", "...)
	buf = appendInt(buf, second, 0)
	buf = append(buf, ", "...)
	buf = appendInt(buf, t.Nanosecond(), 0)
	buf = append(buf, ", "...)
	switch loc := t.Location(); loc {
	case UTC, nil:
		buf = append(buf, "time.UTC"...)
	case Local:
		buf = append(buf, "time.Local"...)
	default:
		// there are several options for how we could display this, none of
		// which are great:
		//
		// - use Location(loc.name), which is not technically valid syntax
		// - use LoadLocation(loc.name), which will cause a syntax error when
		// embedded and also would require us to escape the string without
		// importing fmt or strconv
		// - try to use FixedZone, which would also require escaping the name
		// and would represent e.g. "America/Los_Angeles" daylight saving time
		// shifts inaccurately
		// - use the pointer format, which is no worse than you'd get with the
		// old fmt.Sprintf("%#v", t) format.
		//
		// Of these, Location(loc.name) is the least disruptive. This is an edge
		// case we hope not to hit too often.
		buf = append(buf, `time.Location(`...)
		buf = append(buf, quote(loc.name)...)
		buf = append(buf, ')')
	}
	buf = append(buf, ')')
	return string(buf)
}

// Format returns a textual representation of the time value formatted according
// to the layout defined by the argument. See the documentation for the
// constant called [Layout] to see how to represent the layout format.
//
// The executable example for [Time.Format] demonstrates the working
// of the layout string in detail and is a good reference.
func (t Time) Format(layout string) string {
	const bufSize = 64
	var b []byte
	max := len(layout) + 10
	if max < bufSize {
		var buf [bufSize]byte
		b = buf[:0]
	} else {
		b = make([]byte, 0, max)
	}
	b = t.AppendFormat(b, layout)
	return string(b)
}

// AppendFormat is like [Time.Format] but appends the textual
// representation to b and returns the extended buffer.
func (t Time) AppendFormat(b []byte, layout string) []byte {
	// Optimize for RFC3339 as it accounts for over half of all representations.
	switch layout {
	case RFC3339:
		return t.appendFormatRFC3339(b, false)
	case RFC3339Nano:
		return t.appendFormatRFC3339(b, true)
	default:
		return t.appendFormat(b, layout)
	}
}

func (t Time) appendFormat(b []byte, layout string) []byte {
	name, offset, abs := t.locabs()
	days := abs.days()

	var (
		year  int = -1
		month Month
		day   int
		yday  int = -1
		hour  int = -1
		min   int
		sec   int
	)

	// Each iteration generates one std value.
	for layout != "" {
		prefix, std, suffix := nextStdChunk(layout)
		if prefix != "" {
			b = append(b, prefix...)
		}
		if std == 0 {
			break
		}
		layout = suffix

		// Compute year, month, day if needed.
		if year < 0 && std&stdNeedDate != 0 {
			year, month, day = days.date()
		}
		if yday < 0 && std&stdNeedYday != 0 {
			_, yday = days.yearYday()
		}

		// Compute hour, minute, second if needed.
		if hour < 0 && std&stdNeedClock != 0 {
			hour, min, sec = abs.clock()
		}

		switch std & stdMask {
		case stdYear:
			y := year
			if y < 0 {
				y = -y
			}
			b = appendInt(b, y%100, 2)
		case stdLongYear:
			b = appendInt(b, year, 4)
		case stdMonth:
			b = append(b, month.String()[:3]...)
		case stdLongMonth:
			m := month.String()
			b = append(b, m...)
		case stdNumMonth:
			b = appendInt(b, int(month), 0)
		case stdZeroMonth:
			b = appendInt(b, int(month), 2)
		case stdWeekDay:
			b = append(b, days.weekday().String()[:3]...)
		case stdLongWeekDay:
			s := days.weekday().String()
			b = append(b, s...)
		case stdDay:
			b = appendInt(b, day, 0)
		case stdUnderDay:
			if day < 10 {
				b = append(b, ' ')
			}
			b = appendInt(b, day, 0)
		case stdZeroDay:
			b = appendInt(b, day, 2)
		case stdUnderYearDay:
			if yday < 100 {
				b = append(b, ' ')
				if yday < 10 {
					b = append(b, ' ')
				}
			}
			b = appendInt(b, yday, 0)
		case stdZeroYearDay:
			b = appendInt(b, yday, 3)
		case stdHour:
			b = appendInt(b, hour, 2)
		case stdHour12:
			// Noon is 12PM, midnight is 12AM.
			hr := hour % 12
			if hr == 0 {
				hr = 12
			}
			b = appendInt(b, hr, 0)
		case stdZeroHour12:
			// Noon is 12PM, midnight is 12AM.
			hr := hour % 12
			if hr == 0 {
				hr = 12
			}
			b = appendInt(b, hr, 2)
		case stdMinute:
			b = appendInt(b, min, 0)
		case stdZeroMinute:
			b = appendInt(b, min, 2)
		case stdSecond:
			b = appendInt(b, sec, 0)
		case stdZeroSecond:
			b = appendInt(b, sec, 2)
		case stdPM:
			if hour >= 12 {
				b = append(b, "PM"...)
			} else {
				b = append(b, "AM"...)
			}
		case stdpm:
			if hour >= 12 {
				b = append(b, "pm"...)
			} else {
				b = append(b, "am"...)
			}
		case stdISO8601TZ, stdISO8601ColonTZ, stdISO8601SecondsTZ, stdISO8601ShortTZ, stdISO8601ColonSecondsTZ, stdNumTZ, stdNumColonTZ, stdNumSecondsTz, stdNumShortTZ, stdNumColonSecondsTZ:
			// Ugly special case. We cheat and take the "Z" variants
			// to mean "the time zone as formatted for ISO 8601".
			if offset == 0 && (std == stdISO8601TZ || std == stdISO8601ColonTZ || std == stdISO8601SecondsTZ || std == stdISO8601ShortTZ || std == stdISO8601ColonSecondsTZ) {
				b = append(b, 'Z')
				break
			}
			zone := offset / 60 // convert to minutes
			absoffset := offset
			if zone < 0 {
				b = append(b, '-')
				zone = -zone
				absoffset = -absoffset
			} else {
				b = append(b, '+')
			}
			b = appendInt(b, zone/60, 2)
			if std == stdISO8601ColonTZ || std == stdNumColonTZ || std == stdISO8601ColonSecondsTZ || std == stdNumColonSecondsTZ {
				b = append(b, ':')
			}
			if std != stdNumShortTZ && std != stdISO8601ShortTZ {
				b = appendInt(b, zone%60, 2)
			}

			// append seconds if appropriate
			if std == stdISO8601SecondsTZ || std == stdNumSecondsTz || std == stdNumColonSecondsTZ || std == stdISO8601ColonSecondsTZ {
				if std == stdNumColonSecondsTZ || std == stdISO8601ColonSecondsTZ {
					b = append(b, ':')
				}
				b = appendInt(b, absoffset%60, 2)
			}

		case stdTZ:
			if name != "" {
				b = append(b, name...)
				break
			}
			// No time zone known for this time, but we must print one.
			// Use the -0700 format.
			zone := offset / 60 // convert to minutes
			if zone < 0 {
				b = append(b, '-')
				zone = -zone
			} else {
				b = append(b, '+')
			}
			b = appendInt(b, zone/60, 2)
			b = appendInt(b, zone%60, 2)
		case stdFracSecond0, stdFracSecond9:
			b = appendNano(b, t.Nanosecond(), std)
		}
	}
	return b
}

var errBad = errors.New("bad value for field") // placeholder not passed to user

// ParseError describes a problem parsing a time string.
type ParseError struct {
	Layout     string
	Value      string
	LayoutElem string
	ValueElem  string
	Message    string
}

// newParseError creates a new ParseError.
// The provided value and valueElem are cloned to avoid escaping their values.
func newParseError(layout, value, layoutElem, valueElem, message string) *ParseError {
	valueCopy := stringslite.Clone(value)
	valueElemCopy := stringslite.Clone(valueElem)
	return &ParseError{layout, valueCopy, layoutElem, valueElemCopy, message}
}

// These are borrowed from unicode/utf8 and strconv and replicate behavior in
// that package, since we can't take a dependency on either.
const (
	lowerhex  = "0123456789abcdef"
	runeSelf  = 0x80
	runeError = '\uFFFD'
)

func quote(s string) string {
	buf := make([]byte, 1, len(s)+2) // slice will be at least len(s) + quotes
	buf[0] = '"'
	for i, c := range s {
		if c >= runeSelf || c < ' ' {
			// This means you are asking us to parse a time.Duration or
			// time.Location with unprintable or non-ASCII characters in it.
			// We don't expect to hit this case very often. We could try to
			// reproduce strconv.Quote's behavior with full fidelity but
			// given how rarely we expect to hit these edge cases, speed and
			// conciseness are better.
			var width int
			if c == runeError {
				width = 1
				if i+2 < len(s) && s[i:i+3] == string(runeError) {
					width = 3
				}
			} else {
				width = len(string(c))
			}
			for j := 0; j < width; j++ {
				buf = append(buf, `\x`...)
				buf = append(buf, lowerhex[s[i+j]>>4])
				buf = append(buf, lowerhex[s[i+j]&0xF])
			}
		} else {
			if c == '"' || c == '\\' {
				buf = append(buf, '\\')
			}
			buf = append(buf, byte(c))
		}
	}
	buf = append(buf, '"')
	return string(buf)
}

// Error returns the string representation of a ParseError.
func (e *ParseError) Error() string {
	if e.Message == "" {
		return "parsing time " +
			quote(e.Value) + " as " +
			quote(e.Layout) + ": cannot parse " +
			quote(e.ValueElem) + " as " +
			quote(e.LayoutElem)
	}
	return "parsing time " +
		quote(e.Value) + e.Message
}

// isDigit reports whether s[i] is in range and is a decimal digit.
func isDigit[bytes []byte | string](s bytes, i int) bool {
	if len(s) <= i {
		return false
	}
	c := s[i]
	return '0' <= c && c <= '9'
}

// getnum parses s[0:1] or s[0:2] (fixed forces s[0:2])
// as a decimal integer and returns the integer and the
// remainder of the string.
func getnum(s string, fixed bool) (int, string, error) {
	if !isDigit(s, 0) {
		return 0, s, errBad
	}
	if !isDigit(s, 1) {
		if fixed {
			return 0, s, errBad
		}
		return int(s[0] - '0'), s[1:], nil
	}
	return int(s[0]-'0')*10 + int(s[1]-'0'), s[2:], nil
}

// getnum3 parses s[0:1], s[0:2], or s[0:3] (fixed forces s[0:3])
// as a decimal integer and returns the integer and the remainder
// of the string.
func getnum3(s string, fixed bool) (int, string, error) {
	var n, i int
	for i = 0; i < 3 && isDigit(s, i); i++ {
		n = n*10 + int(s[i]-'0')
	}
	if i == 0 || fixed && i != 3 {
		return 0, s, errBad
	}
	return n, s[i:], nil
}

func cutspace(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return s
}

// skip removes the given prefix from value,
// treating runs of space characters as equivalent.
func skip(value, prefix string) (string, error) {
	for len(prefix) > 0 {
		if prefix[0] == ' ' {
			if len(value) > 0 && value[0] != ' ' {
				return value, errBad
			}
			prefix = cutspace(prefix)
			value = cutspace(value)
			continue
		}
		if len(value) == 0 || value[0] != prefix[0] {
			return value, errBad
		}
		prefix = prefix[1:]
		value = value[1:]
	}
	return value, nil
}

// Parse parses a formatted string and returns the time value it represents.
// See the documentation for the constant called [Layout] to see how to
// represent the format. The second argument must be parseable using
// the format string (layout) provided as the first argument.
//
// The example for [Time.Format] demonstrates the working of the layout string
// in detail and is a good reference.
//
// When parsing (only), the input may contain a fractional second
// field immediately after the seconds field, even if the layout does not
// signify its presence. In that case either a comma or a decimal point
// followed by a maximal series of digits is parsed as a fractional second.
// Fractional seconds are truncated to nanosecond precision.
//
// Elements omitted from the layout are assumed to be zero or, when
// zero is impossible, one, so parsing "3:04pm" returns the time
// corresponding to Jan 1, year 0, 15:04:00 UTC (note that because the year is
// 0, this time is before the zero Time).
// Years must be in the range 0000..9999. The day of the week is checked
// for syntax but it is otherwise ignored.
//
// For layouts specifying the two-digit year 06, a value NN >= 69 will be treated
// as 19NN and a value NN < 69 will be treated as 20NN.
//
// Timestamps representing leap seconds (second 60) cannot be parsed.
// These are not representable by [Time].
//
// The remainder of this comment describes the handling of time zones.
//
// In the absence of a time zone indicator, Parse returns a time in UTC.
//
// When parsing a time with a zone offset like -0700, if the offset corresponds
// to a time zone used by the current location ([Local]), then Parse uses that
// location and zone in the returned time. Otherwise it records the time as
// being in a fabricated location with time fixed at the given zone offset.
//
// When parsing a time with a zone abbreviation like MST, if the zone abbreviation
// has a defined offset in the current location, then that offset is used.
// The zone abbreviation "UTC" is recognized as UTC regardless of location.
// If the zone abbreviation is unknown, Parse records the time as being
// in a fabricated location with the given zone abbreviation and a zero offset.
// This choice means that such a time can be parsed and reformatted with the
// same layout losslessly, but the exact instant used in the representation will
// differ by the actual zone offset. To avoid such problems, prefer time layouts
// that use a numeric zone offset, or use [ParseInLocation].
func Parse(layout, value string) (Time, error) {
	// Optimize for RFC3339 as it accounts for over half of all representations.
	if layout == RFC3339 || layout == RFC3339Nano {
		if t, ok := parseRFC3339(value, Local); ok {
			return t, nil
		}
	}
	return parse(layout, value, UTC, Local)
}

// ParseInLocation is like Parse but differs in two important ways.
// First, in the absence of time zone information, Parse interprets a time as UTC;
// ParseInLocation interprets the time as in the given location.
// Second, when given a zone offset or abbreviation, Parse tries to match it
// against the Local location; ParseInLocation uses the given location.
func ParseInLocation(layout, value string, loc *Location) (Time, error) {
	// Optimize for RFC3339 as it accounts for over half of all representations.
	if layout == RFC3339 || layout == RFC3339Nano {
		if t, ok := parseRFC3339(value, loc); ok {
			return t, nil
		}
	}
	return parse(layout, value, loc, loc)
}

func parse(layout, value string, defaultLocation, local *Location) (Time, error) {
	alayout, avalue := layout, value
	rangeErrString := "" // set if a value is out of range
	amSet := false       // do we need to subtract 12 from the hour for midnight?
	pmSet := false       // do we need to add 12 to the hour?

	// Time being constructed.
	var (
		year       int
		month      int = -1
		day        int = -1
		yday       int = -1
		hour       int
		min        int
		sec        int
		nsec       int
		z          *Location
		zoneOffset int = -1
		zoneName   string
	)

	// Each iteration processes one std value.
	for {
		var err error
		prefix, std, suffix := nextStdChunk(layout)
		stdstr := layout[len(prefix) : len(layout)-len(suffix)]
		value, err = skip(value, prefix)
		if err != nil {
			return Time{}, newParseError(alayout, avalue, prefix, value, "")
		}
		if std == 0 {
			if len(value) != 0 {
				return Time{}, newParseError(alayout, avalue, "", value, ": extra text: "+quote(value))
			}
			break
		}
		layout = suffix
		var p string
		hold := value
		switch std & stdMask {
		case stdYear:
			if len(value) < 2 {
				err = errBad
				break
			}
			p, value = value[0:2], value[2:]
			year, err = atoi(p)
			if err != nil {
				break
			}
			if year >= 69 { // Unix time starts Dec 31 1969 in some time zones
				year += 1900
			} else {
				year += 2000
			}
		case stdLongYear:
			if len(value) < 4 || !isDigit(value, 0) {
				err = errBad
				break
			}
			p, value = value[0:4], value[4:]
			year, err = atoi(p)
		case stdMonth:
			month, value, err = lookup(shortMonthNames, value)
			month++
		case stdLongMonth:
			month, value, err = lookup(longMonthNames, value)
			month++
		case stdNumMonth, stdZeroMonth:
			month, value, err = getnum(value, std == stdZeroMonth)
			if err == nil && (month <= 0 || 12 < month) {
				rangeErrString = "month"
			}
		case stdWeekDay:
			// Ignore weekday except for error checking.
			_, value, err = lookup(shortDayNames, value)
		case stdLongWeekDay:
			_, value, err = lookup(longDayNames, value)
		case stdDay, stdUnderDay, stdZeroDay:
			if std == stdUnderDay && len(value) > 0 && value[0] == ' ' {
				value = value[1:]
			}
			day, value, err = getnum(value, std == stdZeroDay)
			// Note that we allow any one- or two-digit day here.
			// The month, day, year combination is validated after we've completed parsing.
		case stdUnderYearDay, stdZeroYearDay:
			for i := 0; i < 2; i++ {
				if std == stdUnderYearDay && len(value) > 0 && value[0] == ' ' {
					value = value[1:]
				}
			}
			yday, value, err = getnum3(value, std == stdZeroYearDay)
			// Note that we allow any one-, two-, or three-digit year-day here.
			// The year-day, year combination is validated after we've completed parsing.
		case stdHour:
			hour, value, err = getnum(value, false)
			if hour < 0 || 24 <= hour {
				rangeErrString = "hour"
			}
		case stdHour12, stdZeroHour12:
			hour, value, err = getnum(value, std == stdZeroHour12)
			if hour < 0 || 12 < hour {
				rangeErrString = "hour"
			}
		case stdMinute, stdZeroMinute:
			min, value, err = getnum(value, std == stdZeroMinute)
			if min < 0 || 60 <= min {
				rangeErrString = "minute"
			}
		case stdSecond, stdZeroSecond:
			sec, value, err = getnum(value, std == stdZeroSecond)
			if err != nil {
				break
			}
			if sec < 0 || 60 <= sec {
				rangeErrString = "second"
				break
			}
			// Special case: do we have a fractional second but no
			// fractional second in the format?
			if len(value) >= 2 && commaOrPeriod(value[0]) && isDigit(value, 1) {
				_, std, _ = nextStdChunk(layout)
				std &= stdMask
				if std == stdFracSecond0 || std == stdFracSecond9 {
					// Fractional second in the layout; proceed normally
					break
				}
				// No fractional second in the layout but we have one in the input.
				n := 2
				for ; n < len(value) && isDigit(value, n); n++ {
				}
				nsec, rangeErrString, err = parseNanoseconds(value, n)
				value = value[n:]
			}
		case stdPM:
			if len(value) < 2 {
				err = errBad
				break
			}
			p, value = value[0:2], value[2:]
			switch p {
			case "PM":
				pmSet = true
			case "AM":
				amSet = true
			default:
				err = errBad
			}
		case stdpm:
			if len(value) < 2 {
				err = errBad
				break
			}
			p, value = value[0:2], value[2:]
			switch p {
			case "pm":
				pmSet = true
			case "am":
				amSet = true
			default:
				err = errBad
			}
		case stdISO8601TZ, stdISO8601ShortTZ, stdISO8601ColonTZ, stdISO8601SecondsTZ, stdISO8601ColonSecondsTZ:
			if len(value) >= 1 && value[0] == 'Z' {
				value = value[1:]
				z = UTC
				break
			}
			fallthrough
		case stdNumTZ, stdNumShortTZ, stdNumColonTZ, stdNumSecondsTz, stdNumColonSecondsTZ:
			var sign, hour, min, seconds string
			if std == stdISO8601ColonTZ || std == stdNumColonTZ {
				if len(value) < 6 {
					err = errBad
					break
				}
				if value[3] != ':' {
					err = errBad
					break
				}
				sign, hour, min, seconds, value = value[0:1], value[1:3], value[4:6], "00", value[6:]
			} else if std == stdNumShortTZ || std == stdISO8601ShortTZ {
				if len(value) < 3 {
					err = errBad
					break
				}
				sign, hour, min, seconds, value = value[0:1], value[1:3], "00", "00", value[3:]
			} else if std == stdISO8601ColonSecondsTZ || std == stdNumColonSecondsTZ {
				if len(value) < 9 {
					err = errBad
					break
				}
				if value[3] != ':' || value[6] != ':' {
					err = errBad
					break
				}
				sign, hour, min, seconds, value = value[0:1], value[1:3], value[4:6], value[7:9], value[9:]
			} else if std == stdISO8601SecondsTZ || std == stdNumSecondsTz {
				if len(value) < 7 {
					err = errBad
					break
				}
				sign, hour, min, seconds, value = value[0:1], value[1:3], value[3:5], value[5:7], value[7:]
			} else {
				if len(value) < 5 {
					err = errBad
					break
				}
				sign, hour, min, seconds, value = value[0:1], value[1:3], value[3:5], "00", value[5:]
			}
			var hr, mm, ss int
			hr, _, err = getnum(hour, true)
			if err == nil {
				mm, _, err = getnum(min, true)
				if err == nil {
					ss, _, err = getnum(seconds, true)
				}
			}

			// The range test use > rather than >=,
			// as some people do write offsets of 24 hours
			// or 60 minutes or 60 seconds.
			if hr > 24 {
				rangeErrString = "time zone offset hour"
			}
			if mm > 60 {
				rangeErrString = "time zone offset minute"
			}
			if ss > 60 {
				rangeErrString = "time zone offset second"
			}

			zoneOffset = (hr*60+mm)*60 + ss // offset is in seconds
			switch sign[0] {
			case '+':
			case '-':
				zoneOffset = -zoneOffset
			default:
				err = errBad
			}
		case stdTZ:
			// Does it look like a time zone?
			if len(value) >= 3 && value[0:3] == "UTC" {
				z = UTC
				value = value[3:]
				break
			}
			n, ok := parseTimeZone(value)
			if !ok {
				err = errBad
				break
			}
			zoneName, value = value[:n], value[n:]

		case stdFracSecond0:
			// stdFracSecond0 requires the exact number of digits as specified in
			// the layout.
			ndigit := 1 + digitsLen(std)
			if len(value) < ndigit {
				err = errBad
				break
			}
			nsec, rangeErrString, err = parseNanoseconds(value, ndigit)
			value = value[ndigit:]

		case stdFracSecond9:
			if len(value) < 2 || !commaOrPeriod(value[0]) || value[1] < '0' || '9' < value[1] {
				// Fractional second omitted.
				break
			}
			// Take any number of digits, even more than asked for,
			// because it is what the stdSecond case would do.
			i := 0
			for i+1 < len(value) && '0' <= value[i+1] && value[i+1] <= '9' {
				i++
			}
			nsec, rangeErrString, err = parseNanoseconds(value, 1+i)
			value = value[1+i:]
		}
		if rangeErrString != "" {
			return Time{}, newParseError(alayout, avalue, stdstr, value, ": "+rangeErrString+" out of range")
		}
		if err != nil {
			return Time{}, newParseError(alayout, avalue, stdstr, hold, "")
		}
	}
	if pmSet && hour < 12 {
		hour += 12
	} else if amSet && hour == 12 {
		hour = 0
	}

	// Convert yday to day, month.
	if yday >= 0 {
		var d int
		var m int
		if isLeap(year) {
			if yday == 31+29 {
				m = int(February)
				d = 29
			} else if yday > 31+29 {
				yday--
			}
		}
		if yday < 1 || yday > 365 {
			return Time{}, newParseError(alayout, avalue, "", value, ": day-of-year out of range")
		}
		if m == 0 {
			m = (yday-1)/31 + 1
			if daysBefore(Month(m+1)) < yday {
				m++
			}
			d = yday - daysBefore(Month(m))
		}
		// If month, day already seen, yday's m, d must match.
		// Otherwise, set them from m, d.
		if month >= 0 && month != m {
			return Time{}, newParseError(alayout, avalue, "", value, ": day-of-year does not match month")
		}
		month = m
		if day >= 0 && day != d {
			return Time{}, newParseError(alayout, avalue, "", value, ": day-of-year does not match day")
		}
		day = d
	} else {
		if month < 0 {
			month = int(January)
		}
		if day < 0 {
			day = 1
		}
	}

	// Validate the day of the month.
	if day < 1 || day > daysIn(Month(month), year) {
		return Time{}, newParseError(alayout, avalue, "", value, ": day out of range")
	}

	if z != nil {
		return Date(year, Month(month), day, hour, min, sec, nsec, z), nil
	}

	if zoneOffset != -1 {
		t := Date(year, Month(month), day, hour, min, sec, nsec, UTC)
		t.addSec(-int64(zoneOffset))

		// Look for local zone with the given offset.
		// If that zone was in effect at the given time, use it.
		name, offset, _, _, _ := local.lookup(t.unixSec())
		if offset == zoneOffset && (zoneName == "" || name == zoneName) {
			t.setLoc(local)
			return t, nil
		}

		// Otherwise create fake zone to record offset.
		zoneNameCopy := stringslite.Clone(zoneName) // avoid leaking the input value
		t.setLoc(FixedZone(zoneNameCopy, zoneOffset))
		return t, nil
	}

	if zoneName != "" {
		t := Date(year, Month(month), day, hour, min, sec, nsec, UTC)
		// Look for local zone with the given offset.
		// If that zone was in effect at the given time, use it.
		offset, ok := local.lookupName(zoneName, t.unixSec())
		if ok {
			t.addSec(-int64(offset))
			t.setLoc(local)
			return t, nil
		}

		// Otherwise, create fake zone with unknown offset.
		if len(zoneName) > 3 && zoneName[:3] == "GMT" {
			offset, _ = atoi(zoneName[3:]) // Guaranteed OK by parseGMT.
			offset *= 3600
		}
		zoneNameCopy := stringslite.Clone(zoneName) // avoid leaking the input value
		t.setLoc(FixedZone(zoneNameCopy, offset))
		return t, nil
	}

	// Otherwise, fall back to default.
	return Date(year, Month(month), day, hour, min, sec, nsec, defaultLocation), nil
}

// parseTimeZone parses a time zone string and returns its length. Time zones
// are human-generated and unpredictable. We can't do precise error checking.
// On the other hand, for a correct parse there must be a time zone at the
// beginning of the string, so it's almost always true that there's one
// there. We look at the beginning of the string for a run of upper-case letters.
// If there are more than 5, it's an error.
// If there are 4 or 5 and the last is a T, it's a time zone.
// If there are 3, it's a time zone.
// Otherwise, other than special cases, it's not a time zone.
// GMT is special because it can have an hour offset.
func parseTimeZone(value string) (length int, ok bool) {
	if len(value) < 3 {
		return 0, false
	}
	// Special case 1: ChST and MeST are the only zones with a lower-case letter.
	if len(value) >= 4 && (value[:4] == "ChST" || value[:4] == "MeST") {
		return 4, true
	}
	// Special case 2: GMT may have an hour offset; treat it specially.
	if value[:3] == "GMT" {
		length = parseGMT(value)
		return length, true
	}
	// Special Case 3: Some time zones are not named, but have +/-00 format
	if value[0] == '+' || value[0] == '-' {
		length = parseSignedOffset(value)
		ok := length > 0 // parseSignedOffset returns 0 in case of bad input
		return length, ok
	}
	// How many upper-case letters are there? Need at least three, at most five.
	var nUpper int
	for nUpper = 0; nUpper < 6; nUpper++ {
		if nUpper >= len(value) {
			break
		}
		if c := value[nUpper]; c < 'A' || 'Z' < c {
			break
		}
	}
	switch nUpper {
	case 0, 1, 2, 6:
		return 0, false
	case 5: // Must end in T to match.
		if value[4] == 'T' {
			return 5, true
		}
	case 4:
		// Must end in T, except one special case.
		if value[3] == 'T' || value[:4] == "WITA" {
			return 4, true
		}
	case 3:
		return 3, true
	}
	return 0, false
}

// parseGMT parses a GMT time zone. The input string is known to start "GMT".
// The function checks whether that is followed by a sign and a number in the
// range -23 through +23 excluding zero.
func parseGMT(value string) int {
	value = value[3:]
	if len(value) == 0 {
		return 3
	}

	return 3 + parseSignedOffset(value)
}

// parseSignedOffset parses a signed timezone offset (e.g. "+03" or "-04").
// The function checks for a signed number in the range -23 through +23 excluding zero.
// Returns length of the found offset string or 0 otherwise.
func parseSignedOffset(value string) int {
	sign := value[0]
	if sign != '-' && sign != '+' {
		return 0
	}
	x, rem, err := leadingInt(value[1:])

	// fail if nothing consumed by leadingInt
	if err != nil || value[1:] == rem {
		return 0
	}
	if x > 23 {
		return 0
	}
	return len(value) - len(rem)
}

func commaOrPeriod(b byte) bool {
	return b == '.' || b == ','
}

func parseNanoseconds[bytes []byte | string](value bytes, nbytes int) (ns int, rangeErrString string, err error) {
	if !commaOrPeriod(value[0]) {
		err = errBad
		return
	}
	if nbytes > 10 {
		value = value[:10]
		nbytes = 10
	}
	if ns, err = atoi(value[1:nbytes]); err != nil {
		return
	}
	if ns < 0 {
		rangeErrString = "fractional second"
		return
	}
	// We need nanoseconds, which means scaling by the number
	// of missing digits in the format, maximum length 10.
	scaleDigits := 10 - nbytes
	for i := 0; i < scaleDigits; i++ {
		ns *= 10
	}
	return
}

var errLeadingInt = errors.New("time: bad [0-9]*") // never printed

// leadingInt consumes the leading [0-9]* from s.
func leadingInt[bytes []byte | string](s bytes) (x uint64, rem bytes, err error) {
	i := 0
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		if x > 1<<63/10 {
			// overflow
			return 0, rem, errLeadingInt
		}
		x = x*10 + uint64(c) - '0'
		if x > 1<<63 {
			// overflow
			return 0, rem, errLeadingInt
		}
	}
	return x, s[i:], nil
}

// leadingFraction consumes the leading [0-9]* from s.
// It is used only for fractions, so does not return an error on overflow,
// it just stops accumulating precision.
func leadingFraction(s string) (x uint64, scale float64, rem string) {
	i := 0
	scale = 1
	overflow := false
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		if overflow {
			continue
		}
		if x > (1<<63-1)/10 {
			// It's possible for overflow to give a positive number, so take care.
			overflow = true
			continue
		}
		y := x*10 + uint64(c) - '0'
		if y > 1<<63 {
			overflow = true
			continue
		}
		x = y
		scale *= 10
	}
	return x, scale, s[i:]
}

// parseDurationError describes a problem parsing a duration string.
type parseDurationError struct {
	message string
	value   string
}

func (e *parseDurationError) Error() string {
	return "time: " + e.message + " " + quote(e.value)
}

var unitMap = map[string]uint64{
	"ns": uint64(Nanosecond),
	"us": uint64(Microsecond),
	"µs": uint64(Microsecond), // U+00B5 = micro symbol
	"μs": uint64(Microsecond), // U+03BC = Greek letter mu
	"ms": uint64(Millisecond),
	"s":  uint64(Second),
	"m":  uint64(Minute),
	"h":  uint64(Hour),
}

// ParseDuration parses a duration string.
// A duration string is a possibly signed sequence of
// decimal numbers, each with optional fraction and a unit suffix,
// such as "300ms", "-1.5h" or "2h45m".
// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
func ParseDuration(s string) (Duration, error) {
	// [-+]?([0-9]*(\.[0-9]*)?[a-z]+)+
	orig := s
	var d uint64
	neg := false

	// Consume [-+]?
	if s != "" {
		c := s[0]
		if c == '-' || c == '+' {
			neg = c == '-'
			s = s[1:]
		}
	}
	// Special case: if all that is left is "0", this is zero.
	if s == "0" {
		return 0, nil
	}
	if s == "" {
		return 0, &parseDurationError{"invalid duration", orig}
	}
	for s != "" {
		var (
			v, f  uint64      // integers before, after decimal point
			scale float64 = 1 // value = v + f/scale
		)

		var err error

		// The next character must be [0-9.]
		if !(s[0] == '.' || '0' <= s[0] && s[0] <= '9') {
			return 0, &parseDurationError{"invalid duration", orig}
		}
		// Consume [0-9]*
		pl := len(s)
		v, s, err = leadingInt(s)
		if err != nil {
			return 0, &parseDurationError{"invalid duration", orig}
		}
		pre := pl != len(s) // whether we consumed anything before a period

		// Consume (\.[0-9]*)?
		post := false
		if s != "" && s[0] == '.' {
			s = s[1:]
			pl := len(s)
			f, scale, s = leadingFraction(s)
			post = pl != len(s)
		}
		if !pre && !post {
			// no digits (e.g. ".s" or "-.s")
			return 0, &parseDurationError{"invalid duration", orig}
		}

		// Consume unit.
		i := 0
		for ; i < len(s); i++ {
			c := s[i]
			if c == '.' || '0' <= c && c <= '9' {
				break
			}
		}
		if i == 0 {
			return 0, &parseDurationError{"missing unit in duration", orig}
		}
		u := s[:i]
		s = s[i:]
		unit, ok := unitMap[u]
		if !ok {
			return 0, &parseDurationError{"unknown unit " + quote(u) + " in duration", orig}
		}
		if v > 1<<63/unit {
			// overflow
			return 0, &parseDurationError{"invalid duration", orig}
		}
		v *= unit
		if f > 0 {
			// float64 is needed to be nanosecond accurate for fractions of hours.
			// v >= 0 && (f*unit/scale) <= 3.6e+12 (ns/h, h is the largest unit)
			v += uint64(float64(f) * (float64(unit) / scale))
			if v > 1<<63 {
				// overflow
				return 0, &parseDurationError{"invalid duration", orig}
			}
		}
		d += v
		if d > 1<<63 {
			return 0, &parseDurationError{"invalid duration", orig}
		}
	}
	if neg {
		return -Duration(d), nil
	}
	if d > 1<<63-1 {
		return 0, &parseDurationError{"invalid duration", orig}
	}
	return Duration(d), nil
}

```

// === FILE: references!/go/src/time/format_rfc3339.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import "errors"

// RFC 3339 is the most commonly used format.
//
// It is implicitly used by the Time.(Marshal|Unmarshal)(Text|JSON) methods.
// Also, according to analysis on https://go.dev/issue/52746,
// RFC 3339 accounts for 57% of all explicitly specified time formats,
// with the second most popular format only being used 8% of the time.
// The overwhelming use of RFC 3339 compared to all other formats justifies
// the addition of logic to optimize formatting and parsing.

func (t Time) appendFormatRFC3339(b []byte, nanos bool) []byte {
	_, offset, abs := t.locabs()

	// Format date.
	year, month, day := abs.days().date()
	b = appendInt(b, year, 4)
	b = append(b, '-')
	b = appendInt(b, int(month), 2)
	b = append(b, '-')
	b = appendInt(b, day, 2)

	b = append(b, 'T')

	// Format time.
	hour, min, sec := abs.clock()
	b = appendInt(b, hour, 2)
	b = append(b, ':')
	b = appendInt(b, min, 2)
	b = append(b, ':')
	b = appendInt(b, sec, 2)

	if nanos {
		std := stdFracSecond(stdFracSecond9, 9, '.')
		b = appendNano(b, t.Nanosecond(), std)
	}

	if offset == 0 {
		return append(b, 'Z')
	}

	// Format zone.
	zone := offset / 60 // convert to minutes
	if zone < 0 {
		b = append(b, '-')
		zone = -zone
	} else {
		b = append(b, '+')
	}
	b = appendInt(b, zone/60, 2)
	b = append(b, ':')
	b = appendInt(b, zone%60, 2)
	return b
}

func (t Time) appendStrictRFC3339(b []byte) ([]byte, error) {
	n0 := len(b)
	b = t.appendFormatRFC3339(b, true)

	// Not all valid Go timestamps can be serialized as valid RFC 3339.
	// Explicitly check for these edge cases.
	// See https://go.dev/issue/4556 and https://go.dev/issue/54580.
	num2 := func(b []byte) byte { return 10*(b[0]-'0') + (b[1] - '0') }
	switch {
	case b[n0+len("9999")] != '-': // year must be exactly 4 digits wide
		return b, errors.New("year outside of range [0,9999]")
	case b[len(b)-1] != 'Z':
		c := b[len(b)-len("Z07:00")]
		if ('0' <= c && c <= '9') || num2(b[len(b)-len("07:00"):]) >= 24 {
			return b, errors.New("timezone hour outside of range [0,23]")
		}
	}
	return b, nil
}

func parseRFC3339[bytes []byte | string](s bytes, local *Location) (Time, bool) {
	// parseUint parses s as an unsigned decimal integer and
	// verifies that it is within some range.
	// If it is invalid or out-of-range,
	// it sets ok to false and returns the min value.
	ok := true
	parseUint := func(s bytes, min, max int) (x int) {
		for _, c := range []byte(s) {
			if c < '0' || '9' < c {
				ok = false
				return min
			}
			x = x*10 + int(c) - '0'
		}
		if x < min || max < x {
			ok = false
			return min
		}
		return x
	}

	// Parse the date and time.
	if len(s) < len("2006-01-02T15:04:05") {
		return Time{}, false
	}
	year := parseUint(s[0:4], 0, 9999)                       // e.g., 2006
	month := parseUint(s[5:7], 1, 12)                        // e.g., 01
	day := parseUint(s[8:10], 1, daysIn(Month(month), year)) // e.g., 02
	hour := parseUint(s[11:13], 0, 23)                       // e.g., 15
	min := parseUint(s[14:16], 0, 59)                        // e.g., 04
	sec := parseUint(s[17:19], 0, 59)                        // e.g., 05
	if !ok || !(s[4] == '-' && s[7] == '-' && s[10] == 'T' && s[13] == ':' && s[16] == ':') {
		return Time{}, false
	}
	s = s[19:]

	// Parse the fractional second.
	var nsec int
	if len(s) >= 2 && s[0] == '.' && isDigit(s, 1) {
		n := 2
		for ; n < len(s) && isDigit(s, n); n++ {
		}
		nsec, _, _ = parseNanoseconds(s, n)
		s = s[n:]
	}

	// Parse the time zone.
	t := Date(year, Month(month), day, hour, min, sec, nsec, UTC)
	if len(s) != 1 || s[0] != 'Z' {
		if len(s) != len("-07:00") {
			return Time{}, false
		}
		hr := parseUint(s[1:3], 0, 23) // e.g., 07
		mm := parseUint(s[4:6], 0, 59) // e.g., 00
		if !ok || !((s[0] == '-' || s[0] == '+') && s[3] == ':') {
			return Time{}, false
		}
		zoneOffset := (hr*60 + mm) * 60
		if s[0] == '-' {
			zoneOffset *= -1
		}
		t.addSec(-int64(zoneOffset))

		// Use local zone with the given offset if possible.
		if _, offset, _, _, _ := local.lookup(t.unixSec()); offset == zoneOffset {
			t.setLoc(local)
		} else {
			t.setLoc(FixedZone("", zoneOffset))
		}
	}
	return t, true
}

func parseStrictRFC3339(b []byte) (Time, error) {
	t, ok := parseRFC3339(b, Local)
	if !ok {
		t, err := Parse(RFC3339, string(b))
		if err != nil {
			return Time{}, err
		}

		// The parse template syntax cannot correctly validate RFC 3339.
		// Explicitly check for cases that Parse is unable to validate for.
		// See https://go.dev/issue/54580.
		num2 := func(b []byte) byte { return 10*(b[0]-'0') + (b[1] - '0') }
		switch {
		// TODO(https://go.dev/issue/54580): Strict parsing is disabled for now.
		// Enable this again with a GODEBUG opt-out.
		case true:
			return t, nil
		case b[len("2006-01-02T")+1] == ':': // hour must be two digits
			return Time{}, &ParseError{RFC3339, string(b), "15", string(b[len("2006-01-02T"):][:1]), ""}
		case b[len("2006-01-02T15:04:05")] == ',': // sub-second separator must be a period
			return Time{}, &ParseError{RFC3339, string(b), ".", ",", ""}
		case b[len(b)-1] != 'Z':
			switch {
			case num2(b[len(b)-len("07:00"):]) >= 24: // timezone hour must be in range
				return Time{}, &ParseError{RFC3339, string(b), "Z07:00", string(b[len(b)-len("Z07:00"):]), ": timezone hour out of range"}
			case num2(b[len(b)-len("00"):]) >= 60: // timezone minute must be in range
				return Time{}, &ParseError{RFC3339, string(b), "Z07:00", string(b[len(b)-len("Z07:00"):]), ": timezone minute out of range"}
			}
		default: // unknown error; should not occur
			return Time{}, &ParseError{RFC3339, string(b), RFC3339, string(b), ""}
		}
	}
	return t, nil
}

```

// === FILE: references!/go/src/time/genzabbrs.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

//
// usage:
//
// go run genzabbrs.go -output zoneinfo_abbrs_windows.go
//

package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"text/template"
	"time"
)

var filename = flag.String("output", "zoneinfo_abbrs_windows.go", "output file name")

// getAbbrs finds timezone abbreviations (standard and daylight saving time)
// for location l.
func getAbbrs(l *time.Location) (st, dt string) {
	t := time.Date(time.Now().Year(), 0, 1, 0, 0, 0, 0, l)
	abbr1, off1 := t.Zone()
	for i := 0; i < 12; i++ {
		t = t.AddDate(0, 1, 0)
		abbr2, off2 := t.Zone()
		if abbr1 != abbr2 {
			if off2-off1 < 0 { // southern hemisphere
				abbr1, abbr2 = abbr2, abbr1
			}
			return abbr1, abbr2
		}
	}
	return abbr1, abbr1
}

type zone struct {
	WinName  string
	UnixName string
	StTime   string
	DSTime   string
}

const wzURL = "https://raw.githubusercontent.com/unicode-org/cldr/main/common/supplemental/windowsZones.xml"

type MapZone struct {
	Other     string `xml:"other,attr"`
	Territory string `xml:"territory,attr"`
	Type      string `xml:"type,attr"`
}

type SupplementalData struct {
	Zones []MapZone `xml:"windowsZones>mapTimezones>mapZone"`
}

func readWindowsZones() ([]*zone, error) {
	r, err := http.Get(wzURL)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	var sd SupplementalData
	err = xml.Unmarshal(data, &sd)
	if err != nil {
		return nil, err
	}
	zs := make([]*zone, 0)
	for _, z := range sd.Zones {
		if z.Territory != "001" {
			// to avoid dups. I don't know why.
			continue
		}
		l, err := time.LoadLocation(z.Type)
		if err != nil {
			return nil, err
		}
		st, dt := getAbbrs(l)
		zs = append(zs, &zone{
			WinName:  z.Other,
			UnixName: z.Type,
			StTime:   st,
			DSTime:   dt,
		})
	}
	return zs, nil
}

func main() {
	flag.Parse()
	zs, err := readWindowsZones()
	if err != nil {
		log.Fatal(err)
	}
	slices.SortFunc(zs, func(a, b *zone) int {
		return strings.Compare(a.UnixName, b.UnixName)
	})
	var v = struct {
		URL string
		Zs  []*zone
	}{
		wzURL,
		zs,
	}
	var buf bytes.Buffer
	err = template.Must(template.New("prog").Parse(prog)).Execute(&buf, v)
	if err != nil {
		log.Fatal(err)
	}
	data, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(*filename, data, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

const prog = `
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by genzabbrs.go; DO NOT EDIT.
// Based on information from {{.URL}}

package time

type abbr struct {
	std string
	dst string
}

var abbrs = map[string]abbr{
{{range .Zs}}	"{{.WinName}}": {"{{.StTime}}", "{{.DSTime}}"}, // {{.UnixName}}
{{end}}}

`

```

// === FILE: references!/go/src/time/sleep.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import (
	"unsafe"
)

// Sleep pauses the current goroutine for at least the duration d.
// A negative or zero duration causes Sleep to return immediately.
func Sleep(d Duration)

// syncTimer returns c as an unsafe.Pointer, for passing to newTimer.
func syncTimer(c chan Time) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&c))
}

// when is a helper function for setting the 'when' field of a runtimeTimer.
// It returns what the time will be, in nanoseconds, Duration d in the future.
// If d is negative, it is ignored. If the returned value would be less than
// zero because of an overflow, MaxInt64 is returned.
func when(d Duration) int64 {
	if d <= 0 {
		return runtimeNano()
	}
	t := runtimeNano() + int64(d)
	if t < 0 {
		// N.B. runtimeNano() and d are always positive, so addition
		// (including overflow) will never result in t == 0.
		t = 1<<63 - 1 // math.MaxInt64
	}
	return t
}

// These functions are pushed to package time from package runtime.

// The arg cp is a chan Time, but the declaration in runtime uses a pointer,
// so we use a pointer here too. This keeps some tools that aggressively
// compare linknamed symbol definitions happier.
//
//go:linkname newTimer
func newTimer(when, period int64, f func(any, uintptr, int64), arg any, cp unsafe.Pointer) *Timer

//go:linkname stopTimer
func stopTimer(*Timer) bool

//go:linkname resetTimer
func resetTimer(t *Timer, when, period int64) bool

// Note: The runtime knows the layout of struct Timer, since newTimer allocates it.
// The runtime also knows that Ticker and Timer have the same layout.
// There are extra fields after the channel, reserved for the runtime
// and inaccessible to users.

// The Timer type represents a single event.
// When the Timer expires, the current time will be sent on C,
// unless the Timer was created by [AfterFunc].
// A Timer must be created with [NewTimer] or AfterFunc.
type Timer struct {
	C         <-chan Time
	initTimer bool
}

// Stop prevents the [Timer] from firing.
// It returns true if the call stops the timer, false if the timer has already
// expired or been stopped.
//
// For a func-based timer created with [AfterFunc](d, f),
// if t.Stop returns false, then the timer has already expired
// and the function f has been started in its own goroutine;
// Stop does not wait for f to complete before returning.
// If the caller needs to know whether f is completed,
// it must coordinate with f explicitly.
//
// For a chan-based timer created with NewTimer(d), as of Go 1.23,
// any receive from t.C after Stop has returned is guaranteed to block
// rather than receive a stale time value from before the Stop;
// if the program has not received from t.C already and the timer is
// running, Stop is guaranteed to return true.
// Before Go 1.23, the only safe way to use Stop was insert an extra
// <-t.C if Stop returned false to drain a potential stale value.
// See the [NewTimer] documentation for more details.
func (t *Timer) Stop() bool {
	if !t.initTimer {
		panic("time: Stop called on uninitialized Timer")
	}
	return stopTimer(t)
}

// NewTimer creates a new Timer that will send
// the current time on its channel after at least duration d.
//
// Before Go 1.23, the garbage collector did not recover
// timers that had not yet expired or been stopped, so code often
// immediately deferred t.Stop after calling NewTimer, to make
// the timer recoverable when it was no longer needed.
// As of Go 1.23, the garbage collector can recover unreferenced
// timers, even if they haven't expired or been stopped.
// The Stop method is no longer necessary to help the garbage collector.
// (Code may of course still want to call Stop to stop the timer for other reasons.)
//
// Before Go 1.23, the channel associated with a Timer was
// asynchronous (buffered, capacity 1), which meant that
// stale time values could be received even after [Timer.Stop]
// or [Timer.Reset] returned.
// As of Go 1.23, the channel is synchronous (unbuffered, capacity 0),
// eliminating the possibility of those stale values.
func NewTimer(d Duration) *Timer {
	c := make(chan Time, 1)
	t := newTimer(when(d), 0, sendTime, c, syncTimer(c))
	t.C = c
	return t
}

// Reset changes the timer to expire after duration d.
// It returns true if the timer had been active, false if the timer had
// expired or been stopped.
//
// For a func-based timer created with [AfterFunc](d, f), Reset either reschedules
// when f will run, in which case Reset returns true, or schedules f
// to run again, in which case it returns false.
// When Reset returns false, Reset neither waits for the prior f to
// complete before returning nor does it guarantee that the subsequent
// goroutine running f does not run concurrently with the prior
// one. If the caller needs to know whether the prior execution of
// f is completed, it must coordinate with f explicitly.
//
// For a chan-based timer created with NewTimer, as of Go 1.23,
// any receive from t.C after Reset has returned is guaranteed not
// to receive a time value corresponding to the previous timer settings;
// if the program has not received from t.C already and the timer is
// running, Reset is guaranteed to return true.
// Before Go 1.23, the only safe way to use Reset was to call [Timer.Stop]
// and explicitly drain the timer first.
// See the [NewTimer] documentation for more details.
func (t *Timer) Reset(d Duration) bool {
	if !t.initTimer {
		panic("time: Reset called on uninitialized Timer")
	}
	w := when(d)
	return resetTimer(t, w, 0)
}

// sendTime does a non-blocking send of the current time on c.
func sendTime(c any, seq uintptr, delta int64) {
	// delta is how long ago the channel send was supposed to happen.
	// The current time can be arbitrarily far into the future, because the runtime
	// can delay a sendTime call until a goroutine tries to receive from
	// the channel. Subtract delta to go back to the old time that we
	// used to send.
	select {
	case c.(chan Time) <- Now().Add(Duration(-delta)):
	default:
	}
}

// After waits for the duration to elapse and then sends the current time
// on the returned channel.
// It is equivalent to [NewTimer](d).C.
//
// Before Go 1.23, this documentation warned that the underlying
// [Timer] would not be recovered by the garbage collector until the
// timer fired, and that if efficiency was a concern, code should use
// NewTimer instead and call [Timer.Stop] if the timer is no longer needed.
// As of Go 1.23, the garbage collector can recover unreferenced,
// unstopped timers. There is no reason to prefer NewTimer when After will do.
func After(d Duration) <-chan Time {
	return NewTimer(d).C
}

// AfterFunc waits for the duration to elapse and then calls f
// in its own goroutine. It returns a [Timer] that can
// be used to cancel the call using its Stop method.
// The returned Timer's C field is not used and will be nil.
func AfterFunc(d Duration, f func()) *Timer {
	return newTimer(when(d), 0, goFunc, f, nil)
}

func goFunc(arg any, seq uintptr, delta int64) {
	go arg.(func())()
}

```

// === FILE: references!/go/src/time/sys_plan9.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build plan9

package time

import (
	"errors"
	"syscall"
)

// for testing: whatever interrupts a sleep
func interrupt() {
	// cannot predict pid, don't want to kill group
}

func open(name string) (uintptr, error) {
	fd, err := syscall.Open(name, syscall.O_RDONLY)
	if err != nil {
		return 0, err
	}
	return uintptr(fd), nil
}

func read(fd uintptr, buf []byte) (int, error) {
	return syscall.Read(int(fd), buf)
}

func closefd(fd uintptr) {
	syscall.Close(int(fd))
}

func preadn(fd uintptr, buf []byte, off int) error {
	whence := seekStart
	if off < 0 {
		whence = seekEnd
	}
	if _, err := syscall.Seek(int(fd), int64(off), whence); err != nil {
		return err
	}
	for len(buf) > 0 {
		m, err := syscall.Read(int(fd), buf)
		if m <= 0 {
			if err == nil {
				return errors.New("short read")
			}
			return err
		}
		buf = buf[m:]
	}
	return nil
}

```

// === FILE: references!/go/src/time/sys_unix.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || (js && wasm) || wasip1

package time

import (
	"errors"
	"runtime"
	"syscall"
)

// for testing: whatever interrupts a sleep
func interrupt() {
	// There is no mechanism in wasi to interrupt the call to poll_oneoff
	// used to implement runtime.usleep so this function does nothing, which
	// somewhat defeats the purpose of TestSleep but we are still better off
	// validating that time elapses when the process calls time.Sleep than
	// skipping the test altogether.
	if runtime.GOOS != "wasip1" {
		syscall.Kill(syscall.Getpid(), syscall.SIGCHLD)
	}
}

func open(name string) (uintptr, error) {
	fd, err := syscall.Open(name, syscall.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	return uintptr(fd), nil
}

func read(fd uintptr, buf []byte) (int, error) {
	return syscall.Read(int(fd), buf)
}

func closefd(fd uintptr) {
	syscall.Close(int(fd))
}

func preadn(fd uintptr, buf []byte, off int) error {
	whence := seekStart
	if off < 0 {
		whence = seekEnd
	}
	if _, err := syscall.Seek(int(fd), int64(off), whence); err != nil {
		return err
	}
	for len(buf) > 0 {
		m, err := syscall.Read(int(fd), buf)
		if m <= 0 {
			if err == nil {
				return errors.New("short read")
			}
			return err
		}
		buf = buf[m:]
	}
	return nil
}

```

// === FILE: references!/go/src/time/sys_windows.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import (
	"errors"
	"syscall"
)

// for testing: whatever interrupts a sleep
func interrupt() {
}

func open(name string) (uintptr, error) {
	fd, err := syscall.Open(name, syscall.O_RDONLY, 0)
	if err != nil {
		// This condition solves issue https://go.dev/issue/50248
		if err == syscall.ERROR_PATH_NOT_FOUND {
			err = syscall.ENOENT
		}
		return 0, err
	}
	return uintptr(fd), nil
}

func read(fd uintptr, buf []byte) (int, error) {
	return syscall.Read(syscall.Handle(fd), buf)
}

func closefd(fd uintptr) {
	syscall.Close(syscall.Handle(fd))
}

func preadn(fd uintptr, buf []byte, off int) error {
	whence := seekStart
	if off < 0 {
		whence = seekEnd
	}
	if _, err := syscall.Seek(syscall.Handle(fd), int64(off), whence); err != nil {
		return err
	}
	for len(buf) > 0 {
		m, err := syscall.Read(syscall.Handle(fd), buf)
		if m <= 0 {
			if err == nil {
				return errors.New("short read")
			}
			return err
		}
		buf = buf[m:]
	}
	return nil
}

```

// === FILE: references!/go/src/time/tick.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import "unsafe"

// Note: The runtime knows the layout of struct Ticker, since newTimer allocates it.
// Note also that Ticker and Timer have the same layout, so that newTimer can handle both.
// The initTimer and initTicker fields are named differently so that
// users cannot convert between the two without unsafe.

// A Ticker holds a channel that delivers “ticks” of a clock
// at intervals.
type Ticker struct {
	C          <-chan Time // The channel on which the ticks are delivered.
	initTicker bool
}

// NewTicker returns a new [Ticker] containing a channel that will send
// the current time on the channel after each tick. The period of the
// ticks is specified by the duration argument. The ticker will adjust
// the time interval or drop ticks to make up for slow receivers.
// The duration d must be greater than zero; if not, NewTicker will
// panic.
//
// Before Go 1.23, the garbage collector did not recover
// tickers that had not yet expired or been stopped, so code often
// immediately deferred t.Stop after calling NewTicker, to make
// the ticker recoverable when it was no longer needed.
// As of Go 1.23, the garbage collector can recover unreferenced
// tickers, even if they haven't been stopped.
// The Stop method is no longer necessary to help the garbage collector.
// (Code may of course still want to call Stop to stop the ticker for other reasons.)
func NewTicker(d Duration) *Ticker {
	if d <= 0 {
		panic("non-positive interval for NewTicker")
	}
	// Give the channel a 1-element time buffer.
	// If the client falls behind while reading, we drop ticks
	// on the floor until the client catches up.
	c := make(chan Time, 1)
	t := (*Ticker)(unsafe.Pointer(newTimer(when(d), int64(d), sendTime, c, syncTimer(c))))
	t.C = c
	return t
}

// Stop turns off a ticker. After Stop, no more ticks will be sent.
// Stop does not close the channel, to permit calling [Ticker.Reset],
// and to prevent a concurrent goroutine reading from the channel
// from seeing an erroneous "tick".
func (t *Ticker) Stop() {
	if !t.initTicker {
		// This is misuse, and the same for time.Timer would panic,
		// but this didn't always panic, and we keep it not panicking
		// to avoid breaking old programs. See issue 21874.
		return
	}
	stopTimer((*Timer)(unsafe.Pointer(t)))
}

// Reset stops a ticker and resets its period to the specified duration.
// The next tick will arrive after the new period elapses. The duration d
// must be greater than zero; if not, Reset will panic.
func (t *Ticker) Reset(d Duration) {
	if d <= 0 {
		panic("non-positive interval for Ticker.Reset")
	}
	if !t.initTicker {
		panic("time: Reset called on uninitialized Ticker")
	}
	resetTimer((*Timer)(unsafe.Pointer(t)), when(d), int64(d))
}

// Tick is a convenience wrapper for [NewTicker] providing access to the ticking
// channel only. Unlike NewTicker, Tick will return nil if d <= 0.
//
// Before Go 1.23, this documentation warned that the underlying
// [Ticker] would never be recovered by the garbage collector, and that
// if efficiency was a concern, code should use NewTicker instead and
// call [Ticker.Stop] when the ticker is no longer needed.
// As of Go 1.23, the garbage collector can recover unreferenced
// tickers, even if they haven't been stopped.
// The Stop method is no longer necessary to help the garbage collector.
// There is no longer any reason to prefer NewTicker when Tick will do.
func Tick(d Duration) <-chan Time {
	if d <= 0 {
		return nil
	}
	return NewTicker(d).C
}

```

// === FILE: references!/go/src/time/time.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package time provides functionality for measuring and displaying time.
//
// The calendrical calculations always assume a Gregorian calendar, with
// no leap seconds.
//
// # Monotonic Clocks
//
// Operating systems provide both a “wall clock,” which is subject to
// changes for clock synchronization, and a “monotonic clock,” which is
// not. The general rule is that the wall clock is for telling time and
// the monotonic clock is for measuring time. Rather than split the API,
// in this package the Time returned by [time.Now] contains both a wall
// clock reading and a monotonic clock reading; later time-telling
// operations use the wall clock reading, but later time-measuring
// operations, specifically comparisons and subtractions, use the
// monotonic clock reading.
//
// For example, this code always computes a positive elapsed time of
// approximately 20 milliseconds, even if the wall clock is changed during
// the operation being timed:
//
//	start := time.Now()
//	... operation that takes 20 milliseconds ...
//	t := time.Now()
//	elapsed := t.Sub(start)
//
// Other idioms, such as [time.Since](start), [time.Until](deadline), and
// time.Now().Before(deadline), are similarly robust against wall clock
// resets.
//
// The rest of this section gives the precise details of how operations
// use monotonic clocks, but understanding those details is not required
// to use this package.
//
// The Time returned by time.Now contains a monotonic clock reading.
// If Time t has a monotonic clock reading, t.Add adds the same duration to
// both the wall clock and monotonic clock readings to compute the result.
// Because t.AddDate(y, m, d), t.Round(d), and t.Truncate(d) are wall time
// computations, they always strip any monotonic clock reading from their results.
// Because t.In, t.Local, and t.UTC are used for their effect on the interpretation
// of the wall time, they also strip any monotonic clock reading from their results.
// The canonical way to strip a monotonic clock reading is to use t = t.Round(0).
//
// If Times t and u both contain monotonic clock readings, the operations
// t.After(u), t.Before(u), t.Equal(u), t.Compare(u), and t.Sub(u) are carried out
// using the monotonic clock readings alone, ignoring the wall clock
// readings. If either t or u contains no monotonic clock reading, these
// operations fall back to using the wall clock readings.
//
// On some systems the monotonic clock will stop if the computer goes to sleep.
// On such a system, t.Sub(u) may not accurately reflect the actual
// time that passed between t and u. The same applies to other functions and
// methods that subtract times, such as [Since], [Until], [Time.Before], [Time.After],
// [Time.Add], [Time.Equal] and [Time.Compare]. In some cases, you may need to strip
// the monotonic clock to get accurate results.
//
// Because the monotonic clock reading has no meaning outside
// the current process, the serialized forms generated by t.GobEncode,
// t.MarshalBinary, t.MarshalJSON, and t.MarshalText omit the monotonic
// clock reading, and t.Format provides no format for it. Similarly, the
// constructors [time.Date], [time.Parse], [time.ParseInLocation], and [time.Unix],
// as well as the unmarshalers t.GobDecode, t.UnmarshalBinary.
// t.UnmarshalJSON, and t.UnmarshalText always create times with
// no monotonic clock reading.
//
// The monotonic clock reading exists only in [Time] values. It is not
// a part of [Duration] values or the Unix times returned by t.Unix and
// friends.
//
// Note that the Go == operator compares not just the time instant but
// also the [Location] and the monotonic clock reading. See the
// documentation for the Time type for a discussion of equality
// testing for Time values.
//
// For debugging, the result of t.String does include the monotonic
// clock reading if present. If t != u because of different monotonic clock readings,
// that difference will be visible when printing t.String() and u.String().
//
// # Timer Resolution
//
// [Timer] resolution varies depending on the Go runtime, the operating system
// and the underlying hardware.
// On Unix, the resolution is ~1ms.
// On Windows version 1803 and newer, the resolution is ~0.5ms.
// On older Windows versions, the default resolution is ~16ms, but
// a higher resolution may be requested using [golang.org/x/sys/windows.TimeBeginPeriod].
package time

import (
	"errors"
	"math/bits"
	_ "unsafe" // for go:linkname
)

// A Time represents an instant in time with nanosecond precision.
//
// Programs using times should typically store and pass them as values,
// not pointers. That is, time variables and struct fields should be of
// type [time.Time], not *time.Time.
//
// A Time value can be used by multiple goroutines simultaneously except
// that the methods [Time.GobDecode], [Time.UnmarshalBinary], [Time.UnmarshalJSON] and
// [Time.UnmarshalText] are not concurrency-safe.
//
// Time instants can be compared using the [Time.Before], [Time.After], and [Time.Equal] methods.
// The [Time.Sub] method subtracts two instants, producing a [Duration].
// The [Time.Add] method adds a Time and a Duration, producing a Time.
//
// The zero value of type Time is January 1, year 1, 00:00:00.000000000 UTC.
// As this time is unlikely to come up in practice, the [Time.IsZero] method gives
// a simple way of detecting a time that has not been initialized explicitly.
//
// Each time has an associated [Location]. The methods [Time.Local], [Time.UTC], and Time.In return a
// Time with a specific Location. Changing the Location of a Time value with
// these methods does not change the actual instant it represents, only the time
// zone in which to interpret it.
//
// Representations of a Time value saved by the [Time.GobEncode], [Time.MarshalBinary], [Time.AppendBinary],
// [Time.MarshalJSON], [Time.MarshalText] and [Time.AppendText] methods store the [Time.Location]'s offset,
// but not the location name. They therefore lose information about Daylight Saving Time.
//
// In addition to the required “wall clock” reading, a Time may contain an optional
// reading of the current process's monotonic clock, to provide additional precision
// for comparison or subtraction.
// See the “Monotonic Clocks” section in the package documentation for details.
//
// Note that the Go == operator compares not just the time instant but also the
// Location and the monotonic clock reading. Therefore, Time values should not
// be used as map or database keys without first guaranteeing that the
// identical Location has been set for all values, which can be achieved
// through use of the UTC or Local method, and that the monotonic clock reading
// has been stripped by setting t = t.Round(0). In general, prefer t.Equal(u)
// to t == u, since t.Equal uses the most accurate comparison available and
// correctly handles the case when only one of its arguments has a monotonic
// clock reading.
type Time struct {
	// wall and ext encode the wall time seconds, wall time nanoseconds,
	// and optional monotonic clock reading in nanoseconds.
	//
	// From high to low bit position, wall encodes a 1-bit flag (hasMonotonic),
	// a 33-bit seconds field, and a 30-bit wall time nanoseconds field.
	// The nanoseconds field is in the range [0, 999999999].
	// If the hasMonotonic bit is 0, then the 33-bit field must be zero
	// and the full signed 64-bit wall seconds since Jan 1 year 1 is stored in ext.
	// If the hasMonotonic bit is 1, then the 33-bit field holds a 33-bit
	// unsigned wall seconds since Jan 1 year 1885, and ext holds a
	// signed 64-bit monotonic clock reading, nanoseconds since process start.
	wall uint64
	ext  int64

	// loc specifies the Location that should be used to
	// determine the minute, hour, month, day, and year
	// that correspond to this Time.
	// The nil location means UTC.
	// All UTC times are represented with loc==nil, never loc==&utcLoc.
	loc *Location
}

const (
	hasMonotonic = 1 << 63
	maxWall      = wallToInternal + (1<<33 - 1) // year 2157
	minWall      = wallToInternal               // year 1885
	nsecMask     = 1<<30 - 1
	nsecShift    = 30
)

// These helpers for manipulating the wall and monotonic clock readings
// take pointer receivers, even when they don't modify the time,
// to make them cheaper to call.

// nsec returns the time's nanoseconds.
func (t *Time) nsec() int32 {
	return int32(t.wall & nsecMask)
}

// sec returns the time's seconds since Jan 1 year 1.
func (t *Time) sec() int64 {
	if t.wall&hasMonotonic != 0 {
		return wallToInternal + int64(t.wall<<1>>(nsecShift+1))
	}
	return t.ext
}

// unixSec returns the time's seconds since Jan 1 1970 (Unix time).
func (t *Time) unixSec() int64 { return t.sec() + internalToUnix }

// addSec adds d seconds to the time.
func (t *Time) addSec(d int64) {
	if t.wall&hasMonotonic != 0 {
		sec := int64(t.wall << 1 >> (nsecShift + 1))
		dsec := sec + d
		if 0 <= dsec && dsec <= 1<<33-1 {
			t.wall = t.wall&nsecMask | uint64(dsec)<<nsecShift | hasMonotonic
			return
		}
		// Wall second now out of range for packed field.
		// Move to ext.
		t.stripMono()
	}

	// Check if the sum of t.ext and d overflows and handle it properly.
	sum := t.ext + d
	if (sum > t.ext) == (d > 0) {
		t.ext = sum
	} else if d > 0 {
		t.ext = 1<<63 - 1
	} else {
		t.ext = -(1<<63 - 1)
	}
}

// setLoc sets the location associated with the time.
func (t *Time) setLoc(loc *Location) {
	if loc == &utcLoc {
		loc = nil
	}
	t.stripMono()
	t.loc = loc
}

// stripMono strips the monotonic clock reading in t.
func (t *Time) stripMono() {
	if t.wall&hasMonotonic != 0 {
		t.ext = t.sec()
		t.wall &= nsecMask
	}
}

// setMono sets the monotonic clock reading in t.
// If t cannot hold a monotonic clock reading,
// because its wall time is too large,
// setMono is a no-op.
func (t *Time) setMono(m int64) {
	if t.wall&hasMonotonic == 0 {
		sec := t.ext
		if sec < minWall || maxWall < sec {
			return
		}
		t.wall |= hasMonotonic | uint64(sec-minWall)<<nsecShift
	}
	t.ext = m
}

// mono returns t's monotonic clock reading.
// It returns 0 for a missing reading.
// This function is used only for testing,
// so it's OK that technically 0 is a valid
// monotonic clock reading as well.
func (t *Time) mono() int64 {
	if t.wall&hasMonotonic == 0 {
		return 0
	}
	return t.ext
}

// IsZero reports whether t represents the zero time instant,
// January 1, year 1, 00:00:00 UTC.
func (t Time) IsZero() bool {
	// If hasMonotonic is set in t.wall, then the time can't be before 1885, so it can't be the year 1.
	// If hasMonotonic is zero, then all the bits in wall other than the nanoseconds field should be 0.
	// So if there are no nanoseconds then t.wall == 0, and if there are no seconds then t.ext == 0.
	// This is equivalent to t.sec() == 0 && t.nsec() == 0, but is more efficient.
	return t.wall == 0 && t.ext == 0
}

// After reports whether the time instant t is after u.
func (t Time) After(u Time) bool {
	if t.wall&u.wall&hasMonotonic != 0 {
		return t.ext > u.ext
	}
	ts := t.sec()
	us := u.sec()
	return ts > us || ts == us && t.nsec() > u.nsec()
}

// Before reports whether the time instant t is before u.
func (t Time) Before(u Time) bool {
	if t.wall&u.wall&hasMonotonic != 0 {
		return t.ext < u.ext
	}
	ts := t.sec()
	us := u.sec()
	return ts < us || ts == us && t.nsec() < u.nsec()
}

// Compare compares the time instant t with u. If t is before u, it returns -1;
// if t is after u, it returns +1; if they're the same, it returns 0.
func (t Time) Compare(u Time) int {
	var tc, uc int64
	if t.wall&u.wall&hasMonotonic != 0 {
		tc, uc = t.ext, u.ext
	} else {
		tc, uc = t.sec(), u.sec()
		if tc == uc {
			tc, uc = int64(t.nsec()), int64(u.nsec())
		}
	}
	switch {
	case tc < uc:
		return -1
	case tc > uc:
		return +1
	}
	return 0
}

// Equal reports whether t and u represent the same time instant.
// Two times can be equal even if they are in different locations.
// For example, 6:00 +0200 and 4:00 UTC are Equal.
// See the documentation on the Time type for the pitfalls of using == with
// Time values; most code should use Equal instead.
func (t Time) Equal(u Time) bool {
	if t.wall&u.wall&hasMonotonic != 0 {
		return t.ext == u.ext
	}
	return t.sec() == u.sec() && t.nsec() == u.nsec()
}

// A Month specifies a month of the year (January = 1, ...).
type Month int

const (
	January Month = 1 + iota
	February
	March
	April
	May
	June
	July
	August
	September
	October
	November
	December
)

// String returns the English name of the month ("January", "February", ...).
func (m Month) String() string {
	if January <= m && m <= December {
		return longMonthNames[m-1]
	}
	buf := make([]byte, 20)
	n := fmtInt(buf, uint64(m))
	return "%!Month(" + string(buf[n:]) + ")"
}

// A Weekday specifies a day of the week (Sunday = 0, ...).
type Weekday int

const (
	Sunday Weekday = iota
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
)

// String returns the English name of the day ("Sunday", "Monday", ...).
func (d Weekday) String() string {
	if Sunday <= d && d <= Saturday {
		return longDayNames[d]
	}
	buf := make([]byte, 20)
	n := fmtInt(buf, uint64(d))
	return "%!Weekday(" + string(buf[n:]) + ")"
}

// Computations on Times
//
// The zero value for a Time is defined to be
//	January 1, year 1, 00:00:00.000000000 UTC
// which (1) looks like a zero, or as close as you can get in a date
// (1-1-1 00:00:00 UTC), (2) is unlikely enough to arise in practice to
// be a suitable "not set" sentinel, unlike Jan 1 1970, and (3) has a
// non-negative year even in time zones west of UTC, unlike 1-1-0
// 00:00:00 UTC, which would be 12-31-(-1) 19:00:00 in New York.
//
// The zero Time value does not force a specific epoch for the time
// representation. For example, to use the Unix epoch internally, we
// could define that to distinguish a zero value from Jan 1 1970, that
// time would be represented by sec=-1, nsec=1e9. However, it does
// suggest a representation, namely using 1-1-1 00:00:00 UTC as the
// epoch, and that's what we do.
//
// The Add and Sub computations are oblivious to the choice of epoch.
//
// The presentation computations - year, month, minute, and so on - all
// rely heavily on division and modulus by positive constants. For
// calendrical calculations we want these divisions to round down, even
// for negative values, so that the remainder is always positive, but
// Go's division (like most hardware division instructions) rounds to
// zero. We can still do those computations and then adjust the result
// for a negative numerator, but it's annoying to write the adjustment
// over and over. Instead, we can change to a different epoch so long
// ago that all the times we care about will be positive, and then round
// to zero and round down coincide. These presentation routines already
// have to add the zone offset, so adding the translation to the
// alternate epoch is cheap. For example, having a non-negative time t
// means that we can write
//
//	sec = t % 60
//
// instead of
//
//	sec = t % 60
//	if sec < 0 {
//		sec += 60
//	}
//
// everywhere.
//
// The calendar runs on an exact 400 year cycle: a 400-year calendar
// printed for 1970-2369 will apply as well to 2370-2769. Even the days
// of the week match up. It simplifies date computations to choose the
// cycle boundaries so that the exceptional years are always delayed as
// long as possible: March 1, year 0 is such a day:
// the first leap day (Feb 29) is four years minus one day away,
// the first multiple-of-4 year without a Feb 29 is 100 years minus one day away,
// and the first multiple-of-100 year with a Feb 29 is 400 years minus one day away.
// March 1 year Y for any Y = 0 mod 400 is also such a day.
//
// Finally, it's convenient if the delta between the Unix epoch and
// long-ago epoch is representable by an int64 constant.
//
// These three considerations—choose an epoch as early as possible, that
// starts on March 1 of a year equal to 0 mod 400, and that is no more than
// 2⁶³ seconds earlier than 1970—bring us to the year -292277022400.
// We refer to this moment as the absolute zero instant, and to times
// measured as a uint64 seconds since this year as absolute times.
//
// Times measured as an int64 seconds since the year 1—the representation
// used for Time's sec field—are called internal times.
//
// Times measured as an int64 seconds since the year 1970 are called Unix
// times.
//
// It is tempting to just use the year 1 as the absolute epoch, defining
// that the routines are only valid for years >= 1. However, the
// routines would then be invalid when displaying the epoch in time zones
// west of UTC, since it is year 0. It doesn't seem tenable to say that
// printing the zero time correctly isn't supported in half the time
// zones. By comparison, it's reasonable to mishandle some times in
// the year -292277022400.
//
// All this is opaque to clients of the API and can be changed if a
// better implementation presents itself.
//
// The date calculations are implemented using the following clever math from
// Cassio Neri and Lorenz Schneider, “Euclidean affine functions and their
// application to calendar algorithms,” SP&E 2023. https://doi.org/10.1002/spe.3172
//
// Define a “calendrical division” (f, f°, f*) to be a triple of functions converting
// one time unit into a whole number of larger units and the remainder and back.
// For example, in a calendar with no leap years, (d/365, d%365, y*365) is the
// calendrical division for days into years:
//
//	(f)  year := days/365
//	(f°) yday := days%365
//	(f*) days := year*365 (+ yday)
//
// Note that f* is usually the “easy” function to write: it's the
// calendrical multiplication that inverts the more complex division.
//
// Neri and Schneider prove that when f* takes the form
//
//	f*(n) = (a n + b) / c
//
// using integer division rounding down with a ≥ c > 0,
// which they call a Euclidean affine function or EAF, then:
//
//	f(n) = (c n + c - b - 1) / a
//	f°(n) = (c n + c - b - 1) % a / c
//
// This gives a fairly direct calculation for any calendrical division for which
// we can write the calendrical multiplication in EAF form.
// Because the epoch has been shifted to March 1, all the calendrical
// multiplications turn out to be possible to write in EAF form.
// When a date is broken into [century, cyear, amonth, mday],
// with century, cyear, and mday 0-based,
// and amonth 3-based (March = 3, ..., January = 13, February = 14),
// the calendrical multiplications written in EAF form are:
//
//	yday = (153 (amonth-3) + 2) / 5 = (153 amonth - 457) / 5
//	cday = 365 cyear + cyear/4 = 1461 cyear / 4
//	centurydays = 36524 century + century/4 = 146097 century / 4
//	days = centurydays + cday + yday + mday.
//
// We can only handle one periodic cycle per equation, so the year
// calculation must be split into [century, cyear], handling both the
// 100-year cycle and the 400-year cycle.
//
// The yday calculation is not obvious but derives from the fact
// that the March through January calendar repeats the 5-month
// 153-day cycle 31, 30, 31, 30, 31 (we don't care about February
// because yday only ever count the days _before_ February 1,
// since February is the last month).
//
// Using the rule for deriving f and f° from f*, these multiplications
// convert to these divisions:
//
//	century := (4 days + 3) / 146097
//	cdays := (4 days + 3) % 146097 / 4
//	cyear := (4 cdays + 3) / 1461
//	ayday := (4 cdays + 3) % 1461 / 4
//	amonth := (5 ayday + 461) / 153
//	mday := (5 ayday + 461) % 153 / 5
//
// The a in ayday and amonth stands for absolute (March 1-based)
// to distinguish from the standard yday (January 1-based).
//
// After computing these, we can translate from the March 1 calendar
// to the standard January 1 calendar with branch-free math assuming a
// branch-free conversion from bool to int 0 or 1, denoted int(b) here:
//
//	isJanFeb := int(yday >= marchThruDecember)
//	month := amonth - isJanFeb*12
//	year := century*100 + cyear + isJanFeb
//	isLeap := int(cyear%4 == 0) & (int(cyear != 0) | int(century%4 == 0))
//	day := 1 + mday
//	yday := 1 + ayday + 31 + 28 + isLeap&^isJanFeb - 365*isJanFeb
//
// isLeap is the standard leap-year rule, but the split year form
// makes the divisions all reduce to binary masking.
// Note that day and yday are 1-based, in contrast to mday and ayday.

// To keep the various units separate, we define integer types
// for each. These are never stored in interfaces nor allocated,
// so their type information does not appear in Go binaries.
const (
	secondsPerMinute = 60
	secondsPerHour   = 60 * secondsPerMinute
	secondsPerDay    = 24 * secondsPerHour
	secondsPerWeek   = 7 * secondsPerDay
	daysPer400Years  = 365*400 + 97

	// Days from March 1 through end of year
	marchThruDecember = 31 + 30 + 31 + 30 + 31 + 31 + 30 + 31 + 30 + 31

	// absoluteYears is the number of years we subtract from internal time to get absolute time.
	// This value must be 0 mod 400, and it defines the “absolute zero instant”
	// mentioned in the “Computations on Times” comment above: March 1, -absoluteYears.
	// Dates before the absolute epoch will not compute correctly,
	// but otherwise the value can be changed as needed.
	absoluteYears = 292277022400

	// The year of the zero Time.
	// Assumed by the unixToInternal computation below.
	internalYear = 1

	// Offsets to convert between internal and absolute or Unix times.
	absoluteToInternal int64 = -(absoluteYears*365.2425 + marchThruDecember) * secondsPerDay
	internalToAbsolute       = -absoluteToInternal

	unixToInternal int64 = (1969*365 + 1969/4 - 1969/100 + 1969/400) * secondsPerDay
	internalToUnix int64 = -unixToInternal

	absoluteToUnix = absoluteToInternal + internalToUnix
	unixToAbsolute = unixToInternal + internalToAbsolute

	wallToInternal int64 = (1884*365 + 1884/4 - 1884/100 + 1884/400) * secondsPerDay
)

// An absSeconds counts the number of seconds since the absolute zero instant.
type absSeconds uint64

// An absDays counts the number of days since the absolute zero instant.
type absDays uint64

// An absCentury counts the number of centuries since the absolute zero instant.
type absCentury uint64

// An absCyear counts the number of years since the start of a century.
type absCyear int

// An absYday counts the number of days since the start of a year.
// Note that absolute years start on March 1.
type absYday int

// An absMonth counts the number of months since the start of a year.
// absMonth=0 denotes March.
type absMonth int

// An absLeap is a single bit (0 or 1) denoting whether a given year is a leap year.
type absLeap int

// An absJanFeb is a single bit (0 or 1) denoting whether a given day falls in January or February.
// That is a special case because the absolute years start in March (unlike normal calendar years).
type absJanFeb int

// dateToAbsDays takes a standard year/month/day and returns the
// number of days from the absolute epoch to that day.
// The days argument can be out of range and in particular can be negative.
func dateToAbsDays(year int64, month Month, day int) absDays {
	// See “Computations on Times” comment above.
	amonth := uint32(month)
	janFeb := uint32(0)
	if amonth < 3 {
		janFeb = 1
	}
	amonth += 12 * janFeb
	y := uint64(year) - uint64(janFeb) + absoluteYears

	// For amonth is in the range [3,14], we want:
	//
	//	ayday := (153*amonth - 457) / 5
	//
	// (See the “Computations on Times” comment above
	// as well as Neri and Schneider, section 7.)
	//
	// That is equivalent to:
	//
	//	ayday := (979*amonth - 2919) >> 5
	//
	// and the latter form uses a couple fewer instructions,
	// so use it, saving a few cycles.
	// See Neri and Schneider, section 8.3
	// for more about this optimization.
	//
	// (Note that there is no saved division, because the compiler
	// implements / 5 without division in all cases.)
	ayday := (979*amonth - 2919) >> 5

	century := y / 100
	cyear := uint32(y % 100)
	cday := 1461 * cyear / 4
	centurydays := 146097 * century / 4

	return absDays(centurydays + uint64(int64(cday+ayday)+int64(day)-1))
}

// days converts absolute seconds to absolute days.
func (abs absSeconds) days() absDays {
	return absDays(abs / secondsPerDay)
}

// split splits days into century, cyear, ayday.
func (days absDays) split() (century absCentury, cyear absCyear, ayday absYday) {
	// See “Computations on Times” comment above.
	d := 4*uint64(days) + 3
	century = absCentury(d / 146097)

	// This should be
	//	cday := uint32(d % 146097) / 4
	//	cd := 4*cday + 3
	// which is to say
	//	cday := uint32(d % 146097) >> 2
	//	cd := cday<<2 + 3
	// but of course (x>>2<<2)+3 == x|3,
	// so do that instead.
	cd := uint32(d%146097) | 3

	// For cdays in the range [0,146097] (100 years), we want:
	//
	//	cyear := (4 cdays + 3) / 1461
	//	yday := (4 cdays + 3) % 1461 / 4
	//
	// (See the “Computations on Times” comment above
	// as well as Neri and Schneider, section 7.)
	//
	// That is equivalent to:
	//
	//	cyear := (2939745 cdays) >> 32
	//	yday := (2939745 cdays) & 0xFFFFFFFF / 2939745 / 4
	//
	// so do that instead, saving a few cycles.
	// See Neri and Schneider, section 8.3
	// for more about this optimization.
	hi, lo := bits.Mul32(2939745, cd)
	cyear = absCyear(hi)
	ayday = absYday(lo / 2939745 / 4)
	return
}

// split splits ayday into absolute month and standard (1-based) day-in-month.
func (ayday absYday) split() (m absMonth, mday int) {
	// See “Computations on Times” comment above.
	//
	// For yday in the range [0,366],
	//
	//	amonth := (5 yday + 461) / 153
	//	mday := (5 yday + 461) % 153 / 5
	//
	// is equivalent to:
	//
	//	amonth = (2141 yday + 197913) >> 16
	//	mday = (2141 yday + 197913) & 0xFFFF / 2141
	//
	// so do that instead, saving a few cycles.
	// See Neri and Schneider, section 8.3.
	d := 2141*uint32(ayday) + 197913
	return absMonth(d >> 16), 1 + int((d&0xFFFF)/2141)
}

// janFeb returns 1 if the March 1-based ayday is in January or February, 0 otherwise.
func (ayday absYday) janFeb() absJanFeb {
	// See “Computations on Times” comment above.
	jf := absJanFeb(0)
	if ayday >= marchThruDecember {
		jf = 1
	}
	return jf
}

// month returns the standard Month for (m, janFeb)
func (m absMonth) month(janFeb absJanFeb) Month {
	// See “Computations on Times” comment above.
	return Month(m) - Month(janFeb)*12
}

// leap returns 1 if (century, cyear) is a leap year, 0 otherwise.
func (century absCentury) leap(cyear absCyear) absLeap {
	// See “Computations on Times” comment above.
	y4ok := 0
	if cyear%4 == 0 {
		y4ok = 1
	}
	y100ok := 0
	if cyear != 0 {
		y100ok = 1
	}
	y400ok := 0
	if century%4 == 0 {
		y400ok = 1
	}
	return absLeap(y4ok & (y100ok | y400ok))
}

// year returns the standard year for (century, cyear, janFeb).
func (century absCentury) year(cyear absCyear, janFeb absJanFeb) int {
	// See “Computations on Times” comment above.
	return int(uint64(century)*100-absoluteYears) + int(cyear) + int(janFeb)
}

// yday returns the standard 1-based yday for (ayday, janFeb, leap).
func (ayday absYday) yday(janFeb absJanFeb, leap absLeap) int {
	// See “Computations on Times” comment above.
	return int(ayday) + (1 + 31 + 28) + int(leap)&^int(janFeb) - 365*int(janFeb)
}

// date converts days into standard year, month, day.
func (days absDays) date() (year int, month Month, day int) {
	century, cyear, ayday := days.split()
	amonth, day := ayday.split()
	janFeb := ayday.janFeb()
	year = century.year(cyear, janFeb)
	month = amonth.month(janFeb)
	return
}

// yearYday converts days into the standard year and 1-based yday.
func (days absDays) yearYday() (year, yday int) {
	century, cyear, ayday := days.split()
	janFeb := ayday.janFeb()
	year = century.year(cyear, janFeb)
	yday = ayday.yday(janFeb, century.leap(cyear))
	return
}

// absSec returns the time t as an absolute seconds, adjusted by the zone offset.
// It is called when computing a presentation property like Month or Hour.
// We'd rather call it abs, but there are linknames to abs that make that problematic.
// See timeAbs below.
func (t Time) absSec() absSeconds {
	l := t.loc
	// Avoid function calls when possible.
	if l == nil || l == &localLoc {
		l = l.get()
	}
	sec := t.unixSec()
	if l != &utcLoc {
		if l.cacheZone != nil && l.cacheStart <= sec && sec < l.cacheEnd {
			sec += int64(l.cacheZone.offset)
		} else {
			_, offset, _, _, _ := l.lookup(sec)
			sec += int64(offset)
		}
	}
	return absSeconds(sec + (unixToInternal + internalToAbsolute))
}

// locabs is a combination of the Zone and abs methods,
// extracting both return values from a single zone lookup.
func (t Time) locabs() (name string, offset int, abs absSeconds) {
	l := t.loc
	if l == nil || l == &localLoc {
		l = l.get()
	}
	// Avoid function call if we hit the local time cache.
	sec := t.unixSec()
	if l != &utcLoc {
		if l.cacheZone != nil && l.cacheStart <= sec && sec < l.cacheEnd {
			name = l.cacheZone.name
			offset = l.cacheZone.offset
		} else {
			name, offset, _, _, _ = l.lookup(sec)
		}
		sec += int64(offset)
	} else {
		name = "UTC"
	}
	abs = absSeconds(sec + (unixToInternal + internalToAbsolute))
	return
}

// Date returns the year, month, and day in which t occurs.
func (t Time) Date() (year int, month Month, day int) {
	return t.absSec().days().date()
}

// Year returns the year in which t occurs.
func (t Time) Year() int {
	century, cyear, ayday := t.absSec().days().split()
	janFeb := ayday.janFeb()
	return century.year(cyear, janFeb)
}

// Month returns the month of the year specified by t.
func (t Time) Month() Month {
	_, _, ayday := t.absSec().days().split()
	amonth, _ := ayday.split()
	return amonth.month(ayday.janFeb())
}

// Day returns the day of the month specified by t.
func (t Time) Day() int {
	_, _, ayday := t.absSec().days().split()
	_, day := ayday.split()
	return day
}

// Weekday returns the day of the week specified by t.
func (t Time) Weekday() Weekday {
	return t.absSec().days().weekday()
}

// weekday returns the day of the week specified by days.
func (days absDays) weekday() Weekday {
	// March 1 of the absolute year, like March 1 of 2000, was a Wednesday.
	return Weekday((uint64(days) + uint64(Wednesday)) % 7)
}

// ISOWeek returns the ISO 8601 year and week number in which t occurs.
// Week ranges from 1 to 53. Jan 01 to Jan 03 of year n might belong to
// week 52 or 53 of year n-1, and Dec 29 to Dec 31 might belong to week 1
// of year n+1.
func (t Time) ISOWeek() (year, week int) {
	// According to the rule that the first calendar week of a calendar year is
	// the week including the first Thursday of that year, and that the last one is
	// the week immediately preceding the first calendar week of the next calendar year.
	// See https://www.iso.org/obp/ui#iso:std:iso:8601:-1:ed-1:v1:en:term:3.1.1.23 for details.

	// weeks start with Monday
	// Monday Tuesday Wednesday Thursday Friday Saturday Sunday
	// 1      2       3         4        5      6        7
	// +3     +2      +1        0        -1     -2       -3
	// the offset to Thursday
	days := t.absSec().days()
	thu := days + absDays(Thursday-((days-1).weekday()+1))
	year, yday := thu.yearYday()
	return year, (yday-1)/7 + 1
}

// Clock returns the hour, minute, and second within the day specified by t.
func (t Time) Clock() (hour, min, sec int) {
	return t.absSec().clock()
}

// clock returns the hour, minute, and second within the day specified by abs.
func (abs absSeconds) clock() (hour, min, sec int) {
	sec = int(abs % secondsPerDay)
	hour = sec / secondsPerHour
	sec -= hour * secondsPerHour
	min = sec / secondsPerMinute
	sec -= min * secondsPerMinute
	return
}

// Hour returns the hour within the day specified by t, in the range [0, 23].
func (t Time) Hour() int {
	return int(t.absSec()%secondsPerDay) / secondsPerHour
}

// Minute returns the minute offset within the hour specified by t, in the range [0, 59].
func (t Time) Minute() int {
	return int(t.absSec()%secondsPerHour) / secondsPerMinute
}

// Second returns the second offset within the minute specified by t, in the range [0, 59].
func (t Time) Second() int {
	return int(t.absSec() % secondsPerMinute)
}

// Nanosecond returns the nanosecond offset within the second specified by t,
// in the range [0, 999999999].
func (t Time) Nanosecond() int {
	return int(t.nsec())
}

// YearDay returns the day of the year specified by t, in the range [1,365] for non-leap years,
// and [1,366] in leap years.
func (t Time) YearDay() int {
	_, yday := t.absSec().days().yearYday()
	return yday
}

// A Duration represents the elapsed time between two instants
// as an int64 nanosecond count. The representation limits the
// largest representable duration to approximately 290 years.
type Duration int64

const (
	minDuration Duration = -1 << 63
	maxDuration Duration = 1<<63 - 1
)

// Common durations. There is no definition for units of Day or larger
// to avoid confusion across daylight savings time zone transitions.
//
// To count the number of units in a [Duration], divide:
//
//	second := time.Second
//	fmt.Print(int64(second/time.Millisecond)) // prints 1000
//
// To convert an integer number of units to a Duration, multiply:
//
//	seconds := 10
//	fmt.Print(time.Duration(seconds)*time.Second) // prints 10s
const (
	Nanosecond  Duration = 1
	Microsecond          = 1000 * Nanosecond
	Millisecond          = 1000 * Microsecond
	Second               = 1000 * Millisecond
	Minute               = 60 * Second
	Hour                 = 60 * Minute
)

// String returns a string representing the duration in the form "72h3m0.5s".
// Leading zero units are omitted. As a special case, durations less than one
// second format use a smaller unit (milli-, micro-, or nanoseconds) to ensure
// that the leading digit is non-zero. The zero duration formats as 0s.
func (d Duration) String() string {
	// This is inlinable to take advantage of "function outlining".
	// Thus, the caller can decide whether a string must be heap allocated.
	var arr [32]byte
	n := d.format(&arr)
	return string(arr[n:])
}

// format formats the representation of d into the end of buf and
// returns the offset of the first character.
func (d Duration) format(buf *[32]byte) int {
	// Largest time is 2540400h10m10.000000000s
	w := len(buf)

	u := uint64(d)
	neg := d < 0
	if neg {
		u = -u
	}

	if u < uint64(Second) {
		// Special case: if duration is smaller than a second,
		// use smaller units, like 1.2ms
		var prec int
		w--
		buf[w] = 's'
		w--
		switch {
		case u == 0:
			buf[w] = '0'
			return w
		case u < uint64(Microsecond):
			// print nanoseconds
			prec = 0
			buf[w] = 'n'
		case u < uint64(Millisecond):
			// print microseconds
			prec = 3
			// U+00B5 'µ' micro sign == 0xC2 0xB5
			w-- // Need room for two bytes.
			copy(buf[w:], "µ")
		default:
			// print milliseconds
			prec = 6
			buf[w] = 'm'
		}
		w, u = fmtFrac(buf[:w], u, prec)
		w = fmtInt(buf[:w], u)
	} else {
		w--
		buf[w] = 's'

		w, u = fmtFrac(buf[:w], u, 9)

		// u is now integer seconds
		w = fmtInt(buf[:w], u%60)
		u /= 60

		// u is now integer minutes
		if u > 0 {
			w--
			buf[w] = 'm'
			w = fmtInt(buf[:w], u%60)
			u /= 60

			// u is now integer hours
			// Stop at hours because days can be different lengths.
			if u > 0 {
				w--
				buf[w] = 'h'
				w = fmtInt(buf[:w], u)
			}
		}
	}

	if neg {
		w--
		buf[w] = '-'
	}

	return w
}

// fmtFrac formats the fraction of v/10**prec (e.g., ".12345") into the
// tail of buf, omitting trailing zeros. It omits the decimal
// point too when the fraction is 0. It returns the index where the
// output bytes begin and the value v/10**prec.
func fmtFrac(buf []byte, v uint64, prec int) (nw int, nv uint64) {
	// Omit trailing zeros up to and including decimal point.
	w := len(buf)
	print := false
	for i := 0; i < prec; i++ {
		digit := v % 10
		print = print || digit != 0
		if print {
			w--
			buf[w] = byte(digit) + '0'
		}
		v /= 10
	}
	if print {
		w--
		buf[w] = '.'
	}
	return w, v
}

// fmtInt formats v into the tail of buf.
// It returns the index where the output begins.
func fmtInt(buf []byte, v uint64) int {
	w := len(buf)
	if v == 0 {
		w--
		buf[w] = '0'
	} else {
		for v > 0 {
			w--
			buf[w] = byte(v%10) + '0'
			v /= 10
		}
	}
	return w
}

// Nanoseconds returns the duration as an integer nanosecond count.
func (d Duration) Nanoseconds() int64 { return int64(d) }

// Microseconds returns the duration as an integer microsecond count.
func (d Duration) Microseconds() int64 { return int64(d) / 1e3 }

// Milliseconds returns the duration as an integer millisecond count.
func (d Duration) Milliseconds() int64 { return int64(d) / 1e6 }

// These methods return float64 because the dominant
// use case is for printing a floating point number like 1.5s, and
// a truncation to integer would make them not useful in those cases.
// Splitting the integer and fraction ourselves guarantees that
// converting the returned float64 to an integer rounds the same
// way that a pure integer conversion would have, even in cases
// where, say, float64(d.Nanoseconds())/1e9 would have rounded
// differently.

// Seconds returns the duration as a floating point number of seconds.
func (d Duration) Seconds() float64 {
	sec := d / Second
	nsec := d % Second
	return float64(sec) + float64(nsec)/1e9
}

// Minutes returns the duration as a floating point number of minutes.
func (d Duration) Minutes() float64 {
	min := d / Minute
	nsec := d % Minute
	return float64(min) + float64(nsec)/(60*1e9)
}

// Hours returns the duration as a floating point number of hours.
func (d Duration) Hours() float64 {
	hour := d / Hour
	nsec := d % Hour
	return float64(hour) + float64(nsec)/(60*60*1e9)
}

// Truncate returns the result of rounding d toward zero to a multiple of m.
// If m <= 0, Truncate returns d unchanged.
func (d Duration) Truncate(m Duration) Duration {
	if m <= 0 {
		return d
	}
	return d - d%m
}

// lessThanHalf reports whether x+x < y but avoids overflow,
// assuming x and y are both positive (Duration is signed).
func lessThanHalf(x, y Duration) bool {
	return uint64(x)+uint64(x) < uint64(y)
}

// Round returns the result of rounding d to the nearest multiple of m.
// The rounding behavior for halfway values is to round away from zero.
// If the result exceeds the maximum (or minimum)
// value that can be stored in a [Duration],
// Round returns the maximum (or minimum) duration.
// If m <= 0, Round returns d unchanged.
func (d Duration) Round(m Duration) Duration {
	if m <= 0 {
		return d
	}
	r := d % m
	if d < 0 {
		r = -r
		if lessThanHalf(r, m) {
			return d + r
		}
		if d1 := d - m + r; d1 < d {
			return d1
		}
		return minDuration // overflow
	}
	if lessThanHalf(r, m) {
		return d - r
	}
	if d1 := d + m - r; d1 > d {
		return d1
	}
	return maxDuration // overflow
}

// Abs returns the absolute value of d.
// As a special case, Duration([math.MinInt64]) is converted to Duration([math.MaxInt64]),
// reducing its magnitude by 1 nanosecond.
func (d Duration) Abs() Duration {
	switch {
	case d >= 0:
		return d
	case d == minDuration:
		return maxDuration
	default:
		return -d
	}
}

// Add returns the time t+d.
func (t Time) Add(d Duration) Time {
	dsec := int64(d / 1e9)
	nsec := t.nsec() + int32(d%1e9)
	if nsec >= 1e9 {
		dsec++
		nsec -= 1e9
	} else if nsec < 0 {
		dsec--
		nsec += 1e9
	}
	t.wall = t.wall&^nsecMask | uint64(nsec) // update nsec
	t.addSec(dsec)
	if t.wall&hasMonotonic != 0 {
		te := t.ext + int64(d)
		if d < 0 && te > t.ext || d > 0 && te < t.ext {
			// Monotonic clock reading now out of range; degrade to wall-only.
			t.stripMono()
		} else {
			t.ext = te
		}
	}
	return t
}

// Sub returns the duration t-u. If the result exceeds the maximum (or minimum)
// value that can be stored in a [Duration], the maximum (or minimum) duration
// will be returned.
// To compute t-d for a duration d, use t.Add(-d).
func (t Time) Sub(u Time) Duration {
	if t.wall&u.wall&hasMonotonic != 0 {
		return subMono(t.ext, u.ext)
	}
	d := Duration(t.sec()-u.sec())*Second + Duration(t.nsec()-u.nsec())
	// Check for overflow or underflow.
	switch {
	case u.Add(d).Equal(t):
		return d // d is correct
	case t.Before(u):
		return minDuration // t - u is negative out of range
	default:
		return maxDuration // t - u is positive out of range
	}
}

func subMono(t, u int64) Duration {
	d := Duration(t - u)
	if d < 0 && t > u {
		return maxDuration // t - u is positive out of range
	}
	if d > 0 && t < u {
		return minDuration // t - u is negative out of range
	}
	return d
}

// Since returns the time elapsed since t.
// It is shorthand for time.Now().Sub(t).
func Since(t Time) Duration {
	if t.wall&hasMonotonic != 0 && !runtimeIsBubbled() {
		// Common case optimization: if t has monotonic time, then Sub will use only it.
		return subMono(runtimeNano()-startNano, t.ext)
	}
	return Now().Sub(t)
}

// Until returns the duration until t.
// It is shorthand for t.Sub(time.Now()).
func Until(t Time) Duration {
	if t.wall&hasMonotonic != 0 && !runtimeIsBubbled() {
		// Common case optimization: if t has monotonic time, then Sub will use only it.
		return subMono(t.ext, runtimeNano()-startNano)
	}
	return t.Sub(Now())
}

// AddDate returns the time corresponding to adding the
// given number of years, months, and days to t.
// For example, AddDate(-1, 2, 3) applied to January 1, 2011
// returns March 4, 2010.
//
// Note that dates are fundamentally coupled to timezones, and calendrical
// periods like days don't have fixed durations. AddDate uses the Location of
// the Time value to determine these durations. That means that the same
// AddDate arguments can produce a different shift in absolute time depending on
// the base Time value and its Location. For example, AddDate(0, 0, 1) applied
// to 12:00 on March 27 always returns 12:00 on March 28. At some locations and
// in some years this is a 24 hour shift. In others it's a 23 hour shift due to
// daylight savings time transitions.
//
// AddDate normalizes its result in the same way that Date does,
// so, for example, adding one month to October 31 yields
// December 1, the normalized form for November 31.
func (t Time) AddDate(years int, months int, days int) Time {
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	return Date(year+years, month+Month(months), day+days, hour, min, sec, int(t.nsec()), t.Location())
}

// daysBefore returns the number of days in a non-leap year before month m.
// daysBefore(December+1) returns 365.
func daysBefore(m Month) int {
	adj := 0
	if m >= March {
		adj = -2
	}

	// With the -2 adjustment after February,
	// we need to compute the running sum of:
	//	0  31  30  31  30  31  30  31  31  30  31  30  31
	// which is:
	//	0  31  61  92 122 153 183 214 245 275 306 336 367
	// This is almost exactly 367/12×(m-1) except for the
	// occasional off-by-one suggesting there may be an
	// integer approximation of the form (a×m + b)/c.
	// A brute force search over small a, b, c finds that
	// (214×m - 211) / 7 computes the function perfectly.
	return (214*int(m)-211)/7 + adj
}

func daysIn(m Month, year int) int {
	if m == February {
		if isLeap(year) {
			return 29
		}
		return 28
	}
	// With the special case of February eliminated, the pattern is
	//	31 30 31 30 31 30 31 31 30 31 30 31
	// Adding m&1 produces the basic alternation;
	// adding (m>>3)&1 inverts the alternation starting in August.
	return 30 + int((m+m>>3)&1)
}

// Provided by package runtime.
//
// now returns the current real time, and is superseded by runtimeNow which returns
// the fake synctest clock when appropriate.
//
// now should be an internal detail,
// but widely used packages access it using linkname.
// Notable members of the hall of shame include:
//   - gitee.com/quant1x/gox
//   - github.com/phuslu/log
//   - github.com/sethvargo/go-limiter
//   - github.com/ulule/limiter/v3
//
// Do not remove or change the type signature.
// See go.dev/issue/67401.
func now() (sec int64, nsec int32, mono int64)

// runtimeNow returns the current time.
// When called within a synctest.Run bubble, it returns the group's fake clock.
//
//go:linkname runtimeNow
func runtimeNow() (sec int64, nsec int32, mono int64)

// runtimeNano returns the current value of the runtime clock in nanoseconds.
// When called within a synctest.Run bubble, it returns the group's fake clock.
//
//go:linkname runtimeNano
func runtimeNano() int64

//go:linkname runtimeIsBubbled
func runtimeIsBubbled() bool

// Monotonic times are reported as offsets from startNano.
// We initialize startNano to runtimeNano() - 1 so that on systems where
// monotonic time resolution is fairly low (e.g. Windows 2008
// which appears to have a default resolution of 15ms),
// we avoid ever reporting a monotonic time of 0.
// (Callers may want to use 0 as "time not set".)
var startNano int64 = runtimeNano() - 1

// x/tools uses a linkname of time.Now in its tests. No harm done.
//go:linkname Now

// Now returns the current local time.
func Now() Time {
	sec, nsec, mono := runtimeNow()
	if mono == 0 {
		return Time{uint64(nsec), sec + unixToInternal, Local}
	}
	mono -= startNano
	sec += unixToInternal - minWall
	if uint64(sec)>>33 != 0 {
		// Seconds field overflowed the 33 bits available when
		// storing a monotonic time. This will be true after
		// March 16, 2157.
		return Time{uint64(nsec), sec + minWall, Local}
	}
	return Time{hasMonotonic | uint64(sec)<<nsecShift | uint64(nsec), mono, Local}
}

func unixTime(sec int64, nsec int32) Time {
	return Time{uint64(nsec), sec + unixToInternal, Local}
}

// UTC returns t with the location set to UTC.
func (t Time) UTC() Time {
	t.setLoc(&utcLoc)
	return t
}

// Local returns t with the location set to local time.
func (t Time) Local() Time {
	t.setLoc(Local)
	return t
}

// In returns a copy of t representing the same time instant, but
// with the copy's location information set to loc for display
// purposes.
//
// In panics if loc is nil.
func (t Time) In(loc *Location) Time {
	if loc == nil {
		panic("time: missing Location in call to Time.In")
	}
	t.setLoc(loc)
	return t
}

// Location returns the time zone information associated with t.
func (t Time) Location() *Location {
	l := t.loc
	if l == nil {
		l = UTC
	}
	return l
}

// Zone computes the time zone in effect at time t, returning the abbreviated
// name of the zone (such as "CET") and its offset in seconds east of UTC.
func (t Time) Zone() (name string, offset int) {
	name, offset, _, _, _ = t.loc.lookup(t.unixSec())
	return
}

// ZoneBounds returns the bounds of the time zone in effect at time t.
// The zone begins at start and the next zone begins at end.
// If the zone begins at the beginning of time, start will be returned as a zero Time.
// If the zone goes on forever, end will be returned as a zero Time.
// The Location of the returned times will be the same as t.
func (t Time) ZoneBounds() (start, end Time) {
	_, _, startSec, endSec, _ := t.loc.lookup(t.unixSec())
	if startSec != alpha {
		start = unixTime(startSec, 0)
		start.setLoc(t.loc)
	}
	if endSec != omega {
		end = unixTime(endSec, 0)
		end.setLoc(t.loc)
	}
	return
}

// Unix returns t as a Unix time, the number of seconds elapsed
// since January 1, 1970 UTC. The result does not depend on the
// location associated with t.
// Unix-like operating systems often record time as a 32-bit
// count of seconds, but since the method here returns a 64-bit
// value it is valid for billions of years into the past or future.
func (t Time) Unix() int64 {
	return t.unixSec()
}

// UnixMilli returns t as a Unix time, the number of milliseconds elapsed since
// January 1, 1970 UTC. The result is undefined if the Unix time in
// milliseconds cannot be represented by an int64 (a date more than 292 million
// years before or after 1970). The result does not depend on the
// location associated with t.
func (t Time) UnixMilli() int64 {
	return t.unixSec()*1e3 + int64(t.nsec())/1e6
}

// UnixMicro returns t as a Unix time, the number of microseconds elapsed since
// January 1, 1970 UTC. The result is undefined if the Unix time in
// microseconds cannot be represented by an int64 (a date before year -290307 or
// after year 294246). The result does not depend on the location associated
// with t.
func (t Time) UnixMicro() int64 {
	return t.unixSec()*1e6 + int64(t.nsec())/1e3
}

// UnixNano returns t as a Unix time, the number of nanoseconds elapsed
// since January 1, 1970 UTC. The result is undefined if the Unix time
// in nanoseconds cannot be represented by an int64 (a date before the year
// 1678 or after 2262). Note that this means the result of calling UnixNano
// on the zero Time is undefined. The result does not depend on the
// location associated with t.
func (t Time) UnixNano() int64 {
	return (t.unixSec())*1e9 + int64(t.nsec())
}

const (
	timeBinaryVersionV1 byte = iota + 1 // For general situation
	timeBinaryVersionV2                 // For LMT only
)

// AppendBinary implements the [encoding.BinaryAppender] interface.
func (t Time) AppendBinary(b []byte) ([]byte, error) {
	var offsetMin int16 // minutes east of UTC. -1 is UTC.
	var offsetSec int8
	version := timeBinaryVersionV1

	if t.Location() == UTC {
		offsetMin = -1
	} else {
		_, offset := t.Zone()
		if offset%60 != 0 {
			version = timeBinaryVersionV2
			offsetSec = int8(offset % 60)
		}

		offset /= 60
		if offset < -32768 || offset == -1 || offset > 32767 {
			return b, errors.New("Time.MarshalBinary: unexpected zone offset")
		}
		offsetMin = int16(offset)
	}

	sec := t.sec()
	nsec := t.nsec()
	b = append(b,
		version,       // byte 0 : version
		byte(sec>>56), // bytes 1-8: seconds
		byte(sec>>48),
		byte(sec>>40),
		byte(sec>>32),
		byte(sec>>24),
		byte(sec>>16),
		byte(sec>>8),
		byte(sec),
		byte(nsec>>24), // bytes 9-12: nanoseconds
		byte(nsec>>16),
		byte(nsec>>8),
		byte(nsec),
		byte(offsetMin>>8), // bytes 13-14: zone offset in minutes
		byte(offsetMin),
	)
	if version == timeBinaryVersionV2 {
		b = append(b, byte(offsetSec))
	}
	return b, nil
}

// MarshalBinary implements the [encoding.BinaryMarshaler] interface.
func (t Time) MarshalBinary() ([]byte, error) {
	b, err := t.AppendBinary(make([]byte, 0, 16))
	if err != nil {
		return nil, err
	}
	return b, nil
}

// UnmarshalBinary implements the [encoding.BinaryUnmarshaler] interface.
func (t *Time) UnmarshalBinary(data []byte) error {
	buf := data
	if len(buf) == 0 {
		return errors.New("Time.UnmarshalBinary: no data")
	}

	version := buf[0]
	if version != timeBinaryVersionV1 && version != timeBinaryVersionV2 {
		return errors.New("Time.UnmarshalBinary: unsupported version")
	}

	wantLen := /*version*/ 1 + /*sec*/ 8 + /*nsec*/ 4 + /*zone offset*/ 2
	if version == timeBinaryVersionV2 {
		wantLen++
	}
	if len(buf) != wantLen {
		return errors.New("Time.UnmarshalBinary: invalid length")
	}

	buf = buf[1:]
	sec := int64(buf[7]) | int64(buf[6])<<8 | int64(buf[5])<<16 | int64(buf[4])<<24 |
		int64(buf[3])<<32 | int64(buf[2])<<40 | int64(buf[1])<<48 | int64(buf[0])<<56

	buf = buf[8:]
	nsec := int32(buf[3]) | int32(buf[2])<<8 | int32(buf[1])<<16 | int32(buf[0])<<24

	buf = buf[4:]
	offset := int(int16(buf[1])|int16(buf[0])<<8) * 60
	if version == timeBinaryVersionV2 {
		offset += int(int8(buf[2]))
	}

	*t = Time{}
	t.wall = uint64(nsec)
	t.ext = sec

	if offset == -1*60 {
		t.setLoc(&utcLoc)
	} else if _, localoff, _, _, _ := Local.lookup(t.unixSec()); offset == localoff {
		t.setLoc(Local)
	} else {
		t.setLoc(FixedZone("", offset))
	}

	return nil
}

// TODO(rsc): Remove GobEncoder, GobDecoder, MarshalJSON, UnmarshalJSON in Go 2.
// The same semantics will be provided by the generic MarshalBinary, MarshalText,
// UnmarshalBinary, UnmarshalText.

// GobEncode implements the gob.GobEncoder interface.
func (t Time) GobEncode() ([]byte, error) {
	return t.MarshalBinary()
}

// GobDecode implements the gob.GobDecoder interface.
func (t *Time) GobDecode(data []byte) error {
	return t.UnmarshalBinary(data)
}

// MarshalJSON implements the [encoding/json.Marshaler] interface.
// The time is a quoted string in the RFC 3339 format with sub-second precision.
// If the timestamp cannot be represented as valid RFC 3339
// (e.g., the year is out of range), then an error is reported.
func (t Time) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, len(RFC3339Nano)+len(`""`))
	b = append(b, '"')
	b, err := t.appendStrictRFC3339(b)
	b = append(b, '"')
	if err != nil {
		return nil, errors.New("Time.MarshalJSON: " + err.Error())
	}
	return b, nil
}

// UnmarshalJSON implements the [encoding/json.Unmarshaler] interface.
// The time must be a quoted string in the RFC 3339 format.
func (t *Time) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	// TODO(https://go.dev/issue/47353): Properly unescape a JSON string.
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.New("Time.UnmarshalJSON: input is not a JSON string")
	}
	data = data[len(`"`) : len(data)-len(`"`)]
	var err error
	*t, err = parseStrictRFC3339(data)
	return err
}

func (t Time) appendTo(b []byte, errPrefix string) ([]byte, error) {
	b, err := t.appendStrictRFC3339(b)
	if err != nil {
		return nil, errors.New(errPrefix + err.Error())
	}
	return b, nil
}

// AppendText implements the [encoding.TextAppender] interface.
// The time is formatted in RFC 3339 format with sub-second precision.
// If the timestamp cannot be represented as valid RFC 3339
// (e.g., the year is out of range), then an error is returned.
func (t Time) AppendText(b []byte) ([]byte, error) {
	return t.appendTo(b, "Time.AppendText: ")
}

// MarshalText implements the [encoding.TextMarshaler] interface. The output
// matches that of calling the [Time.AppendText] method.
//
// See [Time.AppendText] for more information.
func (t Time) MarshalText() ([]byte, error) {
	return t.appendTo(make([]byte, 0, len(RFC3339Nano)), "Time.MarshalText: ")
}

// UnmarshalText implements the [encoding.TextUnmarshaler] interface.
// The time must be in the RFC 3339 format.
func (t *Time) UnmarshalText(data []byte) error {
	var err error
	*t, err = parseStrictRFC3339(data)
	return err
}

// Unix returns the local Time corresponding to the given Unix time,
// sec seconds and nsec nanoseconds since January 1, 1970 UTC.
// It is valid to pass nsec outside the range [0, 999999999].
// Not all sec values have a corresponding time value. One such
// value is 1<<63-1 (the largest int64 value).
func Unix(sec int64, nsec int64) Time {
	if nsec < 0 || nsec >= 1e9 {
		n := nsec / 1e9
		sec += n
		nsec -= n * 1e9
		if nsec < 0 {
			nsec += 1e9
			sec--
		}
	}
	return unixTime(sec, int32(nsec))
}

// UnixMilli returns the local Time corresponding to the given Unix time,
// msec milliseconds since January 1, 1970 UTC.
func UnixMilli(msec int64) Time {
	return Unix(msec/1e3, (msec%1e3)*1e6)
}

// UnixMicro returns the local Time corresponding to the given Unix time,
// usec microseconds since January 1, 1970 UTC.
func UnixMicro(usec int64) Time {
	return Unix(usec/1e6, (usec%1e6)*1e3)
}

// IsDST reports whether the time in the configured location is in Daylight Savings Time.
func (t Time) IsDST() bool {
	_, _, _, _, isDST := t.loc.lookup(t.Unix())
	return isDST
}

func isLeap(year int) bool {
	// year%4 == 0 && (year%100 != 0 || year%400 == 0)
	// Bottom 2 bits must be clear.
	// For multiples of 25, bottom 4 bits must be clear.
	// Thanks to Cassio Neri for this trick.
	mask := 0xf
	if year%25 != 0 {
		mask = 3
	}
	return year&mask == 0
}

// norm returns nhi, nlo such that
//
//	hi * base + lo == nhi * base + nlo
//	0 <= nlo < base
func norm(hi, lo, base int) (nhi, nlo int) {
	if lo < 0 {
		n := (-lo-1)/base + 1
		hi -= n
		lo += n * base
	}
	if lo >= base {
		n := lo / base
		hi += n
		lo -= n * base
	}
	return hi, lo
}

// Date returns the Time corresponding to
//
//	yyyy-mm-dd hh:mm:ss + nsec nanoseconds
//
// in the appropriate zone for that time in the given location.
//
// The month, day, hour, min, sec, and nsec values may be outside
// their usual ranges and will be normalized during the conversion.
// For example, October 32 converts to November 1.
//
// A daylight savings time transition skips or repeats times.
// For example, in the United States, March 13, 2011 2:15am never occurred,
// while November 6, 2011 1:15am occurred twice. In such cases, the
// choice of time zone, and therefore the time, is not well-defined.
// Date returns a time that is correct in one of the two zones involved
// in the transition, but it does not guarantee which.
//
// Date panics if loc is nil.
func Date(year int, month Month, day, hour, min, sec, nsec int, loc *Location) Time {
	if loc == nil {
		panic("time: missing Location in call to Date")
	}

	// Normalize month, overflowing into year.
	m := int(month) - 1
	year, m = norm(year, m, 12)
	month = Month(m) + 1

	// Normalize nsec, sec, min, hour, overflowing into day.
	sec, nsec = norm(sec, nsec, 1e9)
	min, sec = norm(min, sec, 60)
	hour, min = norm(hour, min, 60)
	day, hour = norm(day, hour, 24)

	// Convert to absolute time and then Unix time.
	unix := int64(dateToAbsDays(int64(year), month, day))*secondsPerDay +
		int64(hour*secondsPerHour+min*secondsPerMinute+sec) +
		absoluteToUnix

	// Look for zone offset for expected time, so we can adjust to UTC.
	// The lookup function expects UTC, so first we pass unix in the
	// hope that it will not be too close to a zone transition,
	// and then adjust if it is.
	_, offset, start, end, _ := loc.lookup(unix)
	if offset != 0 {
		utc := unix - int64(offset)
		// If utc is valid for the time zone we found, then we have the right offset.
		// If not, we get the correct offset by looking up utc in the location.
		if utc < start || utc >= end {
			_, offset, _, _, _ = loc.lookup(utc)
		}
		unix -= int64(offset)
	}

	t := unixTime(unix, int32(nsec))
	t.setLoc(loc)
	return t
}

// Truncate returns the result of rounding t down to a multiple of d (since the zero time).
// If d <= 0, Truncate returns t stripped of any monotonic clock reading but otherwise unchanged.
//
// Truncate operates on the time as an absolute duration since the
// zero time; it does not operate on the presentation form of the
// time. Thus, Truncate(Hour) may return a time with a non-zero
// minute, depending on the time's Location.
func (t Time) Truncate(d Duration) Time {
	t.stripMono()
	if d <= 0 {
		return t
	}
	_, r := div(t, d)
	return t.Add(-r)
}

// Round returns the result of rounding t to the nearest multiple of d (since the zero time).
// The rounding behavior for halfway values is to round up.
// If d <= 0, Round returns t stripped of any monotonic clock reading but otherwise unchanged.
//
// Round operates on the time as an absolute duration since the
// zero time; it does not operate on the presentation form of the
// time. Thus, Round(Hour) may return a time with a non-zero
// minute, depending on the time's Location.
func (t Time) Round(d Duration) Time {
	t.stripMono()
	if d <= 0 {
		return t
	}
	_, r := div(t, d)
	if lessThanHalf(r, d) {
		return t.Add(-r)
	}
	return t.Add(d - r)
}

// div divides t by d and returns the quotient parity and remainder.
// We don't use the quotient parity anymore (round half up instead of round to even)
// but it's still here in case we change our minds.
func div(t Time, d Duration) (qmod2 int, r Duration) {
	neg := false
	nsec := t.nsec()
	sec := t.sec()
	if sec < 0 {
		// Operate on absolute value.
		neg = true
		sec = -sec
		nsec = -nsec
		if nsec < 0 {
			nsec += 1e9
			sec-- // sec >= 1 before the -- so safe
		}
	}

	switch {
	// Special case: 2d divides 1 second.
	case d < Second && Second%(d+d) == 0:
		qmod2 = int(nsec/int32(d)) & 1
		r = Duration(nsec % int32(d))

	// Special case: d is a multiple of 1 second.
	case d%Second == 0:
		d1 := int64(d / Second)
		qmod2 = int(sec/d1) & 1
		r = Duration(sec%d1)*Second + Duration(nsec)

	// General case.
	// This could be faster if more cleverness were applied,
	// but it's really only here to avoid special case restrictions in the API.
	// No one will care about these cases.
	default:
		// Compute nanoseconds as 128-bit number.
		sec := uint64(sec)
		tmp := (sec >> 32) * 1e9
		u1 := tmp >> 32
		u0 := tmp << 32
		tmp = (sec & 0xFFFFFFFF) * 1e9
		u0x, u0 := u0, u0+tmp
		if u0 < u0x {
			u1++
		}
		u0x, u0 = u0, u0+uint64(nsec)
		if u0 < u0x {
			u1++
		}

		// Compute remainder by subtracting r<<k for decreasing k.
		// Quotient parity is whether we subtract on last round.
		d1 := uint64(d)
		for d1>>63 != 1 {
			d1 <<= 1
		}
		d0 := uint64(0)
		for {
			qmod2 = 0
			if u1 > d1 || u1 == d1 && u0 >= d0 {
				// subtract
				qmod2 = 1
				u0x, u0 = u0, u0-d0
				if u0 > u0x {
					u1--
				}
				u1 -= d1
			}
			if d1 == 0 && d0 == uint64(d) {
				break
			}
			d0 >>= 1
			d0 |= (d1 & 1) << 63
			d1 >>= 1
		}
		r = Duration(u0)
	}

	if neg && r != 0 {
		// If input was negative and not an exact multiple of d, we computed q, r such that
		//	q*d + r = -t
		// But the right answers are given by -(q-1), d-r:
		//	q*d + r = -t
		//	-q*d - r = t
		//	-(q-1)*d + (d - r) = t
		qmod2 ^= 1
		r = d - r
	}
	return
}

// Regrettable Linkname Compatibility
//
// timeAbs, absDate, and absClock mimic old internal details, no longer used.
// Widely used packages linknamed these to get “faster” time routines.
// Notable members of the hall of shame include:
//   - gitee.com/quant1x/gox
//   - github.com/phuslu/log
//
// phuslu hard-coded 'Unix time + 9223372028715321600' [sic]
// as the input to absDate and absClock, using the old Jan 1-based
// absolute times.
// quant1x linknamed the time.Time.abs method and passed the
// result of that method to absDate and absClock.
//
// Keeping both of these working forces us to provide these three
// routines here, operating on the old Jan 1-based epoch instead
// of the new March 1-based epoch. And the fact that time.Time.abs
// was linknamed means that we have to call the current abs method
// something different (time.Time.absSec, defined above) to make it
// possible to provide this simulation of the old routines here.
//
// None of this code is linked into the binary if not referenced by
// these linkname-happy packages. In particular, despite its name,
// time.Time.abs does not appear in the time.Time method table.
//
// Do not remove these routines or their linknames, or change the
// type signature or meaning of arguments.

//go:linkname legacyTimeTimeAbs time.Time.abs
func legacyTimeTimeAbs(t Time) uint64 {
	return uint64(t.absSec() - marchThruDecember*secondsPerDay)
}

//go:linkname legacyAbsClock time.absClock
func legacyAbsClock(abs uint64) (hour, min, sec int) {
	return absSeconds(abs + marchThruDecember*secondsPerDay).clock()
}

//go:linkname legacyAbsDate time.absDate
func legacyAbsDate(abs uint64, full bool) (year int, month Month, day int, yday int) {
	d := absSeconds(abs + marchThruDecember*secondsPerDay).days()
	year, month, day = d.date()
	_, yday = d.yearYday()
	yday-- // yearYday is 1-based, old API was 0-based
	return
}

```

// === FILE: references!/go/src/time/tzdata/tzdata.go ===
```go
// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tzdata provides an embedded copy of the timezone database.
// If this package is imported anywhere in the program, then if
// the time package cannot find tzdata files on the system,
// it will use this embedded information.
//
// Importing this package will increase the size of a program by about
// 450 KB.
//
// This package should normally be imported by a program's main package,
// not by a library. Libraries normally shouldn't decide whether to
// include the timezone database in a program.
//
// This package will be automatically imported if you build with
// -tags timetzdata.
package tzdata

// The test for this package is time/tzdata_test.go.

import (
	"errors"
	"syscall"
	_ "unsafe" // for go:linkname
)

// registerLoadFromEmbeddedTZData is defined in package time.
//
//go:linkname registerLoadFromEmbeddedTZData time.registerLoadFromEmbeddedTZData
func registerLoadFromEmbeddedTZData(func(string) (string, error))

func init() {
	registerLoadFromEmbeddedTZData(loadFromEmbeddedTZData)
}

// get4s returns the little-endian 32-bit value at the start of s.
func get4s(s string) int {
	if len(s) < 4 {
		return 0
	}
	return int(s[0]) | int(s[1])<<8 | int(s[2])<<16 | int(s[3])<<24
}

// get2s returns the little-endian 16-bit value at the start of s.
func get2s(s string) int {
	if len(s) < 2 {
		return 0
	}
	return int(s[0]) | int(s[1])<<8
}

// loadFromEmbeddedTZData returns the contents of the file with the given
// name in an uncompressed zip file, where the contents of the file can
// be found in embeddedTzdata.
// This is similar to time.loadTzinfoFromZip.
func loadFromEmbeddedTZData(name string) (string, error) {
	const (
		zecheader = 0x06054b50
		zcheader  = 0x02014b50
		ztailsize = 22

		zheadersize = 30
		zheader     = 0x04034b50
	)

	// zipdata is provided by zzipdata.go,
	// which is generated by cmd/dist during make.bash.
	z := zipdata

	idx := len(z) - ztailsize
	n := get2s(z[idx+10:])
	idx = get4s(z[idx+16:])

	for i := 0; i < n; i++ {
		// See time.loadTzinfoFromZip for zip entry layout.
		if get4s(z[idx:]) != zcheader {
			break
		}
		meth := get2s(z[idx+10:])
		size := get4s(z[idx+24:])
		namelen := get2s(z[idx+28:])
		xlen := get2s(z[idx+30:])
		fclen := get2s(z[idx+32:])
		off := get4s(z[idx+42:])
		zname := z[idx+46 : idx+46+namelen]
		idx += 46 + namelen + xlen + fclen
		if zname != name {
			continue
		}
		if meth != 0 {
			return "", errors.New("unsupported compression for " + name + " in embedded tzdata")
		}

		// See time.loadTzinfoFromZip for zip per-file header layout.
		idx = off
		if get4s(z[idx:]) != zheader ||
			get2s(z[idx+8:]) != meth ||
			get2s(z[idx+26:]) != namelen ||
			z[idx+30:idx+30+namelen] != name {
			return "", errors.New("corrupt embedded tzdata")
		}
		xlen = get2s(z[idx+28:])
		idx += 30 + namelen + xlen
		return z[idx : idx+size], nil
	}

	return "", syscall.ENOENT
}

```

// === FILE: references!/go/src/time/zoneinfo.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import (
	"errors"
	"sync"
	"syscall"
)

//go:generate env ZONEINFO=$GOROOT/lib/time/zoneinfo.zip go run genzabbrs.go -output zoneinfo_abbrs_windows.go

// A Location maps time instants to the zone in use at that time.
// Typically, the Location represents the collection of time offsets
// in use in a geographical area. For many Locations the time offset varies
// depending on whether daylight savings time is in use at the time instant.
//
// Location is used to provide a time zone in a printed Time value and for
// calculations involving intervals that may cross daylight savings time
// boundaries.
type Location struct {
	name string
	zone []zone
	tx   []zoneTrans

	// The tzdata information can be followed by a string that describes
	// how to handle DST transitions not recorded in zoneTrans.
	// The format is the TZ environment variable without a colon; see
	// https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap08.html.
	// Example string, for America/Los_Angeles: PST8PDT,M3.2.0,M11.1.0
	extend string

	// Most lookups will be for the current time.
	// To avoid the binary search through tx, keep a
	// static one-element cache that gives the correct
	// zone for the time when the Location was created.
	// if cacheStart <= t < cacheEnd,
	// lookup can return cacheZone.
	// The units for cacheStart and cacheEnd are seconds
	// since January 1, 1970 UTC, to match the argument
	// to lookup.
	cacheStart int64
	cacheEnd   int64
	cacheZone  *zone
}

// A zone represents a single time zone such as CET.
type zone struct {
	name   string // abbreviated name, "CET"
	offset int    // seconds east of UTC
	isDST  bool   // is this zone Daylight Savings Time?
}

// A zoneTrans represents a single time zone transition.
type zoneTrans struct {
	when         int64 // transition time, in seconds since 1970 GMT
	index        uint8 // the index of the zone that goes into effect at that time
	isstd, isutc bool  // ignored - no idea what these mean
}

// alpha and omega are the beginning and end of time for zone
// transitions.
const (
	alpha = -1 << 63  // math.MinInt64
	omega = 1<<63 - 1 // math.MaxInt64
)

// UTC represents Universal Coordinated Time (UTC).
var UTC *Location = &utcLoc

// utcLoc is separate so that get can refer to &utcLoc
// and ensure that it never returns a nil *Location,
// even if a badly behaved client has changed UTC.
var utcLoc = Location{name: "UTC"}

// Local represents the system's local time zone.
// On Unix systems, Local consults the TZ environment
// variable to find the time zone to use. No TZ means
// use the system default /etc/localtime.
// TZ="" means use UTC.
// TZ="foo" means use file foo in the system timezone directory.
var Local *Location = &localLoc

// localLoc is separate so that initLocal can initialize
// it even if a client has changed Local.
var localLoc Location
var localOnce sync.Once

func (l *Location) get() *Location {
	if l == nil {
		return &utcLoc
	}
	if l == &localLoc {
		localOnce.Do(initLocal)
	}
	return l
}

// String returns a descriptive name for the time zone information,
// corresponding to the name argument to [LoadLocation] or [FixedZone].
func (l *Location) String() string {
	return l.get().name
}

var unnamedFixedZones []*Location
var unnamedFixedZonesOnce sync.Once

// FixedZone returns a [Location] that always uses
// the given zone name and offset (seconds east of UTC).
func FixedZone(name string, offset int) *Location {
	// Most calls to FixedZone have an unnamed zone with an offset by the hour.
	// Optimize for that case by returning the same *Location for a given hour.
	const hoursBeforeUTC = 12
	const hoursAfterUTC = 14
	hour := offset / 60 / 60
	if name == "" && -hoursBeforeUTC <= hour && hour <= +hoursAfterUTC && hour*60*60 == offset {
		unnamedFixedZonesOnce.Do(func() {
			unnamedFixedZones = make([]*Location, hoursBeforeUTC+1+hoursAfterUTC)
			for hr := -hoursBeforeUTC; hr <= +hoursAfterUTC; hr++ {
				unnamedFixedZones[hr+hoursBeforeUTC] = fixedZone("", hr*60*60)
			}
		})
		return unnamedFixedZones[hour+hoursBeforeUTC]
	}
	return fixedZone(name, offset)
}

func fixedZone(name string, offset int) *Location {
	l := &Location{
		name:       name,
		zone:       []zone{{name, offset, false}},
		tx:         []zoneTrans{{alpha, 0, false, false}},
		cacheStart: alpha,
		cacheEnd:   omega,
	}
	l.cacheZone = &l.zone[0]
	return l
}

// lookup returns information about the time zone in use at an
// instant in time expressed as seconds since January 1, 1970 00:00:00 UTC.
//
// The returned information gives the name of the zone (such as "CET"),
// the start and end times bracketing sec when that zone is in effect,
// the offset in seconds east of UTC (such as -5*60*60), and whether
// the daylight savings is being observed at that time.
func (l *Location) lookup(sec int64) (name string, offset int, start, end int64, isDST bool) {
	l = l.get()

	if len(l.zone) == 0 {
		name = "UTC"
		offset = 0
		start = alpha
		end = omega
		isDST = false
		return
	}

	if zone := l.cacheZone; zone != nil && l.cacheStart <= sec && sec < l.cacheEnd {
		name = zone.name
		offset = zone.offset
		start = l.cacheStart
		end = l.cacheEnd
		isDST = zone.isDST
		return
	}

	if len(l.tx) == 0 || sec < l.tx[0].when {
		zone := &l.zone[l.lookupFirstZone()]
		name = zone.name
		offset = zone.offset
		start = alpha
		if len(l.tx) > 0 {
			end = l.tx[0].when
		} else {
			end = omega
		}
		isDST = zone.isDST
		return
	}

	// Binary search for entry with largest time <= sec.
	// Not using sort.Search to avoid dependencies.
	tx := l.tx
	end = omega
	lo := 0
	hi := len(tx)
	for hi-lo > 1 {
		m := int(uint(lo+hi) >> 1)
		lim := tx[m].when
		if sec < lim {
			end = lim
			hi = m
		} else {
			lo = m
		}
	}
	zone := &l.zone[tx[lo].index]
	name = zone.name
	offset = zone.offset
	start = tx[lo].when
	// end = maintained during the search
	isDST = zone.isDST

	// If we're at the end of the known zone transitions,
	// try the extend string.
	if lo == len(tx)-1 && l.extend != "" {
		if ename, eoffset, estart, eend, eisDST, ok := tzset(l.extend, start, sec); ok {
			return ename, eoffset, estart, eend, eisDST
		}
	}

	return
}

// lookupFirstZone returns the index of the time zone to use for times
// before the first transition time, or when there are no transition
// times.
//
// The reference implementation in localtime.c from
// https://www.iana.org/time-zones/repository/releases/tzcode2013g.tar.gz
// implements the following algorithm for these cases:
//  1. If the first zone is unused by the transitions, use it.
//  2. Otherwise, if there are transition times, and the first
//     transition is to a zone in daylight time, find the first
//     non-daylight-time zone before and closest to the first transition
//     zone.
//  3. Otherwise, use the first zone that is not daylight time, if
//     there is one.
//  4. Otherwise, use the first zone.
func (l *Location) lookupFirstZone() int {
	// Case 1.
	if !l.firstZoneUsed() {
		return 0
	}

	// Case 2.
	if len(l.tx) > 0 && l.zone[l.tx[0].index].isDST {
		for zi := int(l.tx[0].index) - 1; zi >= 0; zi-- {
			if !l.zone[zi].isDST {
				return zi
			}
		}
	}

	// Case 3.
	for zi := range l.zone {
		if !l.zone[zi].isDST {
			return zi
		}
	}

	// Case 4.
	return 0
}

// firstZoneUsed reports whether the first zone is used by some
// transition.
func (l *Location) firstZoneUsed() bool {
	for _, tx := range l.tx {
		if tx.index == 0 {
			return true
		}
	}
	return false
}

// tzset takes a timezone string like the one found in the TZ environment
// variable, the time of the last time zone transition expressed as seconds
// since January 1, 1970 00:00:00 UTC, and a time expressed the same way.
// We call this a tzset string since in C the function tzset reads TZ.
// The return values are as for lookup, plus ok which reports whether the
// parse succeeded.
func tzset(s string, lastTxSec, sec int64) (name string, offset int, start, end int64, isDST, ok bool) {
	var (
		stdName, dstName     string
		stdOffset, dstOffset int
	)

	stdName, s, ok = tzsetName(s)
	if ok {
		stdOffset, s, ok = tzsetOffset(s)
	}
	if !ok {
		return "", 0, 0, 0, false, false
	}

	// The numbers in the tzset string are added to local time to get UTC,
	// but our offsets are added to UTC to get local time,
	// so we negate the number we see here.
	stdOffset = -stdOffset

	if len(s) == 0 || s[0] == ',' {
		// No daylight savings time.
		return stdName, stdOffset, lastTxSec, omega, false, true
	}

	dstName, s, ok = tzsetName(s)
	if ok {
		if len(s) == 0 || s[0] == ',' {
			dstOffset = stdOffset + secondsPerHour
		} else {
			dstOffset, s, ok = tzsetOffset(s)
			dstOffset = -dstOffset // as with stdOffset, above
		}
	}
	if !ok {
		return "", 0, 0, 0, false, false
	}

	if len(s) == 0 {
		// Default DST rules per tzcode.
		s = ",M3.2.0,M11.1.0"
	}
	// The TZ definition does not mention ';' here but tzcode accepts it.
	if s[0] != ',' && s[0] != ';' {
		return "", 0, 0, 0, false, false
	}
	s = s[1:]

	var startRule, endRule rule
	startRule, s, ok = tzsetRule(s)
	if !ok || len(s) == 0 || s[0] != ',' {
		return "", 0, 0, 0, false, false
	}
	s = s[1:]
	endRule, s, ok = tzsetRule(s)
	if !ok || len(s) > 0 {
		return "", 0, 0, 0, false, false
	}

	// Compute start of year in seconds since Unix epoch,
	// and seconds since then to get to sec.
	year, yday := absSeconds(sec + unixToInternal + internalToAbsolute).days().yearYday()
	ysec := int64((yday-1)*secondsPerDay) + sec%secondsPerDay
	ystart := sec - ysec

	startSec := int64(tzruleTime(year, startRule, stdOffset))
	endSec := int64(tzruleTime(year, endRule, dstOffset))
	dstIsDST, stdIsDST := true, false
	// Note: this is a flipping of "DST" and "STD" while retaining the labels
	// This happens in southern hemispheres. The labelling here thus is a little
	// inconsistent with the goal.
	if endSec < startSec {
		startSec, endSec = endSec, startSec
		stdName, dstName = dstName, stdName
		stdOffset, dstOffset = dstOffset, stdOffset
		stdIsDST, dstIsDST = dstIsDST, stdIsDST
	}

	// The start and end values that we return are accurate
	// close to a daylight savings transition, but are otherwise
	// just the start and end of the year. That suffices for
	// the only caller that cares, which is Date.
	if ysec < startSec {
		return stdName, stdOffset, ystart, startSec + ystart, stdIsDST, true
	} else if ysec >= endSec {
		return stdName, stdOffset, endSec + ystart, ystart + 365*secondsPerDay, stdIsDST, true
	} else {
		return dstName, dstOffset, startSec + ystart, endSec + ystart, dstIsDST, true
	}
}

// tzsetName returns the timezone name at the start of the tzset string s,
// and the remainder of s, and reports whether the parsing is OK.
func tzsetName(s string) (string, string, bool) {
	if len(s) == 0 {
		return "", "", false
	}
	if s[0] != '<' {
		for i, r := range s {
			switch r {
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', ',', '-', '+':
				if i < 3 {
					return "", "", false
				}
				return s[:i], s[i:], true
			}
		}
		if len(s) < 3 {
			return "", "", false
		}
		return s, "", true
	} else {
		for i, r := range s {
			if r == '>' {
				return s[1:i], s[i+1:], true
			}
		}
		return "", "", false
	}
}

// tzsetOffset returns the timezone offset at the start of the tzset string s,
// and the remainder of s, and reports whether the parsing is OK.
// The timezone offset is returned as a number of seconds.
func tzsetOffset(s string) (offset int, rest string, ok bool) {
	if len(s) == 0 {
		return 0, "", false
	}
	neg := false
	if s[0] == '+' {
		s = s[1:]
	} else if s[0] == '-' {
		s = s[1:]
		neg = true
	}

	// The tzdata code permits values up to 24 * 7 here,
	// although POSIX does not.
	var hours int
	hours, s, ok = tzsetNum(s, 0, 24*7)
	if !ok {
		return 0, "", false
	}
	off := hours * secondsPerHour
	if len(s) == 0 || s[0] != ':' {
		if neg {
			off = -off
		}
		return off, s, true
	}

	var mins int
	mins, s, ok = tzsetNum(s[1:], 0, 59)
	if !ok {
		return 0, "", false
	}
	off += mins * secondsPerMinute
	if len(s) == 0 || s[0] != ':' {
		if neg {
			off = -off
		}
		return off, s, true
	}

	var secs int
	secs, s, ok = tzsetNum(s[1:], 0, 59)
	if !ok {
		return 0, "", false
	}
	off += secs

	if neg {
		off = -off
	}
	return off, s, true
}

// ruleKind is the kinds of rules that can be seen in a tzset string.
type ruleKind int

const (
	ruleJulian ruleKind = iota
	ruleDOY
	ruleMonthWeekDay
)

// rule is a rule read from a tzset string.
type rule struct {
	kind ruleKind
	day  int
	week int
	mon  int
	time int // transition time
}

// tzsetRule parses a rule from a tzset string.
// It returns the rule, and the remainder of the string, and reports success.
func tzsetRule(s string) (rule, string, bool) {
	var r rule
	if len(s) == 0 {
		return rule{}, "", false
	}
	ok := false
	if s[0] == 'J' {
		var jday int
		jday, s, ok = tzsetNum(s[1:], 1, 365)
		if !ok {
			return rule{}, "", false
		}
		r.kind = ruleJulian
		r.day = jday
	} else if s[0] == 'M' {
		var mon int
		mon, s, ok = tzsetNum(s[1:], 1, 12)
		if !ok || len(s) == 0 || s[0] != '.' {
			return rule{}, "", false

		}
		var week int
		week, s, ok = tzsetNum(s[1:], 1, 5)
		if !ok || len(s) == 0 || s[0] != '.' {
			return rule{}, "", false
		}
		var day int
		day, s, ok = tzsetNum(s[1:], 0, 6)
		if !ok {
			return rule{}, "", false
		}
		r.kind = ruleMonthWeekDay
		r.day = day
		r.week = week
		r.mon = mon
	} else {
		var day int
		day, s, ok = tzsetNum(s, 0, 365)
		if !ok {
			return rule{}, "", false
		}
		r.kind = ruleDOY
		r.day = day
	}

	if len(s) == 0 || s[0] != '/' {
		r.time = 2 * secondsPerHour // 2am is the default
		return r, s, true
	}

	offset, s, ok := tzsetOffset(s[1:])
	if !ok {
		return rule{}, "", false
	}
	r.time = offset

	return r, s, true
}

// tzsetNum parses a number from a tzset string.
// It returns the number, and the remainder of the string, and reports success.
// The number must be between min and max.
func tzsetNum(s string, min, max int) (num int, rest string, ok bool) {
	if len(s) == 0 {
		return 0, "", false
	}
	num = 0
	for i, r := range s {
		if r < '0' || r > '9' {
			if i == 0 || num < min {
				return 0, "", false
			}
			return num, s[i:], true
		}
		num *= 10
		num += int(r) - '0'
		if num > max {
			return 0, "", false
		}
	}
	if num < min {
		return 0, "", false
	}
	return num, "", true
}

// tzruleTime takes a year, a rule, and a timezone offset,
// and returns the number of seconds since the start of the year
// that the rule takes effect.
func tzruleTime(year int, r rule, off int) int {
	var s int
	switch r.kind {
	case ruleJulian:
		s = (r.day - 1) * secondsPerDay
		if isLeap(year) && r.day >= 60 {
			s += secondsPerDay
		}
	case ruleDOY:
		s = r.day * secondsPerDay
	case ruleMonthWeekDay:
		// Zeller's Congruence.
		m1 := (r.mon+9)%12 + 1
		yy0 := year
		if r.mon <= 2 {
			yy0--
		}
		yy1 := yy0 / 100
		yy2 := yy0 % 100
		dow := ((26*m1-2)/10 + 1 + yy2 + yy2/4 + yy1/4 - 2*yy1) % 7
		if dow < 0 {
			dow += 7
		}
		// Now dow is the day-of-week of the first day of r.mon.
		// Get the day-of-month of the first "dow" day.
		d := r.day - dow
		if d < 0 {
			d += 7
		}
		for i := 1; i < r.week; i++ {
			if d+7 >= daysIn(Month(r.mon), year) {
				break
			}
			d += 7
		}
		d += daysBefore(Month(r.mon))
		if isLeap(year) && r.mon > 2 {
			d++
		}
		s = d * secondsPerDay
	}

	return s + r.time - off
}

// lookupName returns information about the time zone with
// the given name (such as "EST") at the given pseudo-Unix time
// (what the given time of day would be in UTC).
func (l *Location) lookupName(name string, unix int64) (offset int, ok bool) {
	l = l.get()

	// First try for a zone with the right name that was actually
	// in effect at the given time. (In Sydney, Australia, both standard
	// and daylight-savings time are abbreviated "EST". Using the
	// offset helps us pick the right one for the given time.
	// It's not perfect: during the backward transition we might pick
	// either one.)
	for i := range l.zone {
		zone := &l.zone[i]
		if zone.name == name {
			nam, offset, _, _, _ := l.lookup(unix - int64(zone.offset))
			if nam == zone.name {
				return offset, true
			}
		}
	}

	// Otherwise fall back to an ordinary name match.
	for i := range l.zone {
		zone := &l.zone[i]
		if zone.name == name {
			return zone.offset, true
		}
	}

	// Otherwise, give up.
	return
}

// NOTE(rsc): Eventually we will need to accept the POSIX TZ environment
// syntax too, but I don't feel like implementing it today.

var errLocation = errors.New("time: invalid location name")

var zoneinfo *string
var zoneinfoOnce sync.Once

// LoadLocation returns a [Location] with the given name.
//
// If the name is "" or "UTC", LoadLocation returns [UTC].
// If the name is "Local", LoadLocation returns [Local].
//
// Otherwise, a new [Location] is created where the name is taken
// to be a location name corresponding to a file
// in the IANA Time Zone database, such as "America/New_York".
//
// LoadLocation looks for the IANA Time Zone database in the following
// locations in order:
//
//   - the directory or uncompressed zip file named by the ZONEINFO environment variable
//   - on a Unix system, the system standard installation location
//   - $GOROOT/lib/time/zoneinfo.zip
//   - the time/tzdata package, if it was imported
func LoadLocation(name string) (*Location, error) {
	if name == "" || name == "UTC" {
		return UTC, nil
	}
	if name == "Local" {
		return Local, nil
	}
	if containsDotDot(name) || name[0] == '/' || name[0] == '\\' {
		// No valid IANA Time Zone name contains a single dot,
		// much less dot dot. Likewise, none begin with a slash.
		return nil, errLocation
	}
	zoneinfoOnce.Do(func() {
		env, _ := syscall.Getenv("ZONEINFO")
		zoneinfo = &env
	})
	var firstErr error
	if *zoneinfo != "" {
		if zoneData, err := loadTzinfoFromDirOrZip(*zoneinfo, name); err == nil {
			if z, err := LoadLocationFromTZData(name, zoneData); err == nil {
				return z, nil
			}
			firstErr = err
		} else if err != syscall.ENOENT {
			firstErr = err
		}
	}
	if z, err := loadLocation(name, platformZoneSources); err == nil {
		return z, nil
	} else if firstErr == nil {
		firstErr = err
	}
	return nil, firstErr
}

// containsDotDot reports whether s contains "..".
func containsDotDot(s string) bool {
	if len(s) < 2 {
		return false
	}
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '.' && s[i+1] == '.' {
			return true
		}
	}
	return false
}

```

// === FILE: references!/go/src/time/zoneinfo_abbrs_windows.go ===
```go
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by genzabbrs.go; DO NOT EDIT.
// Based on information from https://raw.githubusercontent.com/unicode-org/cldr/main/common/supplemental/windowsZones.xml

package time

type abbr struct {
	std string
	dst string
}

var abbrs = map[string]abbr{
	"Egypt Standard Time":             {"EET", "EEST"},    // Africa/Cairo
	"Morocco Standard Time":           {"+00", "+01"},     // Africa/Casablanca
	"South Africa Standard Time":      {"SAST", "SAST"},   // Africa/Johannesburg
	"South Sudan Standard Time":       {"CAT", "CAT"},     // Africa/Juba
	"Sudan Standard Time":             {"CAT", "CAT"},     // Africa/Khartoum
	"W. Central Africa Standard Time": {"WAT", "WAT"},     // Africa/Lagos
	"E. Africa Standard Time":         {"EAT", "EAT"},     // Africa/Nairobi
	"Sao Tome Standard Time":          {"GMT", "GMT"},     // Africa/Sao_Tome
	"Libya Standard Time":             {"EET", "EET"},     // Africa/Tripoli
	"Namibia Standard Time":           {"CAT", "CAT"},     // Africa/Windhoek
	"Aleutian Standard Time":          {"HST", "HDT"},     // America/Adak
	"Alaskan Standard Time":           {"AKST", "AKDT"},   // America/Anchorage
	"Tocantins Standard Time":         {"-03", "-03"},     // America/Araguaina
	"Paraguay Standard Time":          {"-04", "-03"},     // America/Asuncion
	"Bahia Standard Time":             {"-03", "-03"},     // America/Bahia
	"SA Pacific Standard Time":        {"-05", "-05"},     // America/Bogota
	"Argentina Standard Time":         {"-03", "-03"},     // America/Buenos_Aires
	"Eastern Standard Time (Mexico)":  {"EST", "EST"},     // America/Cancun
	"Venezuela Standard Time":         {"-04", "-04"},     // America/Caracas
	"SA Eastern Standard Time":        {"-03", "-03"},     // America/Cayenne
	"Central Standard Time":           {"CST", "CDT"},     // America/Chicago
	"Central Brazilian Standard Time": {"-04", "-04"},     // America/Cuiaba
	"Mountain Standard Time":          {"MST", "MDT"},     // America/Denver
	"Greenland Standard Time":         {"-02", "-01"},     // America/Godthab
	"Turks And Caicos Standard Time":  {"EST", "EDT"},     // America/Grand_Turk
	"Central America Standard Time":   {"CST", "CST"},     // America/Guatemala
	"Atlantic Standard Time":          {"AST", "ADT"},     // America/Halifax
	"Cuba Standard Time":              {"CST", "CDT"},     // America/Havana
	"US Eastern Standard Time":        {"EST", "EDT"},     // America/Indianapolis
	"SA Western Standard Time":        {"-04", "-04"},     // America/La_Paz
	"Pacific Standard Time":           {"PST", "PDT"},     // America/Los_Angeles
	"Mountain Standard Time (Mexico)": {"MST", "MST"},     // America/Mazatlan
	"Central Standard Time (Mexico)":  {"CST", "CST"},     // America/Mexico_City
	"Saint Pierre Standard Time":      {"-03", "-02"},     // America/Miquelon
	"Montevideo Standard Time":        {"-03", "-03"},     // America/Montevideo
	"Eastern Standard Time":           {"EST", "EDT"},     // America/New_York
	"US Mountain Standard Time":       {"MST", "MST"},     // America/Phoenix
	"Haiti Standard Time":             {"EST", "EDT"},     // America/Port-au-Prince
	"Magallanes Standard Time":        {"-03", "-03"},     // America/Punta_Arenas
	"Canada Central Standard Time":    {"CST", "CST"},     // America/Regina
	"Pacific SA Standard Time":        {"-04", "-03"},     // America/Santiago
	"E. South America Standard Time":  {"-03", "-03"},     // America/Sao_Paulo
	"Newfoundland Standard Time":      {"NST", "NDT"},     // America/St_Johns
	"Pacific Standard Time (Mexico)":  {"PST", "PDT"},     // America/Tijuana
	"Yukon Standard Time":             {"MST", "MST"},     // America/Whitehorse
	"Jordan Standard Time":            {"+03", "+03"},     // Asia/Amman
	"Arabic Standard Time":            {"+03", "+03"},     // Asia/Baghdad
	"Azerbaijan Standard Time":        {"+04", "+04"},     // Asia/Baku
	"SE Asia Standard Time":           {"+07", "+07"},     // Asia/Bangkok
	"Altai Standard Time":             {"+07", "+07"},     // Asia/Barnaul
	"Middle East Standard Time":       {"EET", "EEST"},    // Asia/Beirut
	"Central Asia Standard Time":      {"+06", "+06"},     // Asia/Bishkek
	"India Standard Time":             {"IST", "IST"},     // Asia/Calcutta
	"Transbaikal Standard Time":       {"+09", "+09"},     // Asia/Chita
	"Sri Lanka Standard Time":         {"+0530", "+0530"}, // Asia/Colombo
	"Syria Standard Time":             {"+03", "+03"},     // Asia/Damascus
	"Bangladesh Standard Time":        {"+06", "+06"},     // Asia/Dhaka
	"Arabian Standard Time":           {"+04", "+04"},     // Asia/Dubai
	"West Bank Standard Time":         {"EET", "EEST"},    // Asia/Hebron
	"W. Mongolia Standard Time":       {"+07", "+07"},     // Asia/Hovd
	"North Asia East Standard Time":   {"+08", "+08"},     // Asia/Irkutsk
	"Israel Standard Time":            {"IST", "IDT"},     // Asia/Jerusalem
	"Afghanistan Standard Time":       {"+0430", "+0430"}, // Asia/Kabul
	"Russia Time Zone 11":             {"+12", "+12"},     // Asia/Kamchatka
	"Pakistan Standard Time":          {"PKT", "PKT"},     // Asia/Karachi
	"Nepal Standard Time":             {"+0545", "+0545"}, // Asia/Katmandu
	"North Asia Standard Time":        {"+07", "+07"},     // Asia/Krasnoyarsk
	"Magadan Standard Time":           {"+11", "+11"},     // Asia/Magadan
	"N. Central Asia Standard Time":   {"+07", "+07"},     // Asia/Novosibirsk
	"Omsk Standard Time":              {"+06", "+06"},     // Asia/Omsk
	"North Korea Standard Time":       {"KST", "KST"},     // Asia/Pyongyang
	"Qyzylorda Standard Time":         {"+05", "+05"},     // Asia/Qyzylorda
	"Myanmar Standard Time":           {"+0630", "+0630"}, // Asia/Rangoon
	"Arab Standard Time":              {"+03", "+03"},     // Asia/Riyadh
	"Sakhalin Standard Time":          {"+11", "+11"},     // Asia/Sakhalin
	"Korea Standard Time":             {"KST", "KST"},     // Asia/Seoul
	"China Standard Time":             {"CST", "CST"},     // Asia/Shanghai
	"Singapore Standard Time":         {"+08", "+08"},     // Asia/Singapore
	"Russia Time Zone 10":             {"+11", "+11"},     // Asia/Srednekolymsk
	"Taipei Standard Time":            {"CST", "CST"},     // Asia/Taipei
	"West Asia Standard Time":         {"+05", "+05"},     // Asia/Tashkent
	"Georgian Standard Time":          {"+04", "+04"},     // Asia/Tbilisi
	"Iran Standard Time":              {"+0330", "+0330"}, // Asia/Tehran
	"Tokyo Standard Time":             {"JST", "JST"},     // Asia/Tokyo
	"Tomsk Standard Time":             {"+07", "+07"},     // Asia/Tomsk
	"Ulaanbaatar Standard Time":       {"+08", "+08"},     // Asia/Ulaanbaatar
	"Vladivostok Standard Time":       {"+10", "+10"},     // Asia/Vladivostok
	"Yakutsk Standard Time":           {"+09", "+09"},     // Asia/Yakutsk
	"Ekaterinburg Standard Time":      {"+05", "+05"},     // Asia/Yekaterinburg
	"Caucasus Standard Time":          {"+04", "+04"},     // Asia/Yerevan
	"Azores Standard Time":            {"-01", "+00"},     // Atlantic/Azores
	"Cape Verde Standard Time":        {"-01", "-01"},     // Atlantic/Cape_Verde
	"Greenwich Standard Time":         {"GMT", "GMT"},     // Atlantic/Reykjavik
	"Cen. Australia Standard Time":    {"ACST", "ACDT"},   // Australia/Adelaide
	"E. Australia Standard Time":      {"AEST", "AEST"},   // Australia/Brisbane
	"AUS Central Standard Time":       {"ACST", "ACST"},   // Australia/Darwin
	"Aus Central W. Standard Time":    {"+0845", "+0845"}, // Australia/Eucla
	"Tasmania Standard Time":          {"AEST", "AEDT"},   // Australia/Hobart
	"Lord Howe Standard Time":         {"+1030", "+11"},   // Australia/Lord_Howe
	"W. Australia Standard Time":      {"AWST", "AWST"},   // Australia/Perth
	"AUS Eastern Standard Time":       {"AEST", "AEDT"},   // Australia/Sydney
	"UTC-11":                          {"-11", "-11"},     // Etc/GMT+11
	"Dateline Standard Time":          {"-12", "-12"},     // Etc/GMT+12
	"UTC-02":                          {"-02", "-02"},     // Etc/GMT+2
	"UTC-08":                          {"-08", "-08"},     // Etc/GMT+8
	"UTC-09":                          {"-09", "-09"},     // Etc/GMT+9
	"UTC+12":                          {"+12", "+12"},     // Etc/GMT-12
	"UTC+13":                          {"+13", "+13"},     // Etc/GMT-13
	"UTC":                             {"UTC", "UTC"},     // Etc/UTC
	"Astrakhan Standard Time":         {"+04", "+04"},     // Europe/Astrakhan
	"W. Europe Standard Time":         {"CET", "CEST"},    // Europe/Berlin
	"GTB Standard Time":               {"EET", "EEST"},    // Europe/Bucharest
	"Central Europe Standard Time":    {"CET", "CEST"},    // Europe/Budapest
	"E. Europe Standard Time":         {"EET", "EEST"},    // Europe/Chisinau
	"Turkey Standard Time":            {"+03", "+03"},     // Europe/Istanbul
	"Kaliningrad Standard Time":       {"EET", "EET"},     // Europe/Kaliningrad
	"FLE Standard Time":               {"EET", "EEST"},    // Europe/Kiev
	"GMT Standard Time":               {"GMT", "BST"},     // Europe/London
	"Belarus Standard Time":           {"+03", "+03"},     // Europe/Minsk
	"Russian Standard Time":           {"MSK", "MSK"},     // Europe/Moscow
	"Romance Standard Time":           {"CET", "CEST"},    // Europe/Paris
	"Russia Time Zone 3":              {"+04", "+04"},     // Europe/Samara
	"Saratov Standard Time":           {"+04", "+04"},     // Europe/Saratov
	"Volgograd Standard Time":         {"MSK", "MSK"},     // Europe/Volgograd
	"Central European Standard Time":  {"CET", "CEST"},    // Europe/Warsaw
	"Mauritius Standard Time":         {"+04", "+04"},     // Indian/Mauritius
	"Samoa Standard Time":             {"+13", "+13"},     // Pacific/Apia
	"New Zealand Standard Time":       {"NZST", "NZDT"},   // Pacific/Auckland
	"Bougainville Standard Time":      {"+11", "+11"},     // Pacific/Bougainville
	"Chatham Islands Standard Time":   {"+1245", "+1345"}, // Pacific/Chatham
	"Easter Island Standard Time":     {"-06", "-05"},     // Pacific/Easter
	"Fiji Standard Time":              {"+12", "+12"},     // Pacific/Fiji
	"Central Pacific Standard Time":   {"+11", "+11"},     // Pacific/Guadalcanal
	"Hawaiian Standard Time":          {"HST", "HST"},     // Pacific/Honolulu
	"Line Islands Standard Time":      {"+14", "+14"},     // Pacific/Kiritimati
	"Marquesas Standard Time":         {"-0930", "-0930"}, // Pacific/Marquesas
	"Norfolk Standard Time":           {"+11", "+12"},     // Pacific/Norfolk
	"West Pacific Standard Time":      {"+10", "+10"},     // Pacific/Port_Moresby
	"Tonga Standard Time":             {"+13", "+13"},     // Pacific/Tongatapu
}

```

// === FILE: references!/go/src/time/zoneinfo_android.go ===
```go
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse the "tzdata" packed timezone file used on Android.
// The format is lifted from ZoneInfoDB.java and ZoneInfo.java in
// java/libcore/util in the AOSP.

package time

import (
	"errors"
	"syscall"
)

var platformZoneSources = []string{
	"/system/usr/share/zoneinfo/tzdata",
	"/data/misc/zoneinfo/current/tzdata",
}

func initLocal() {
	// TODO(elias.naur): getprop persist.sys.timezone
	localLoc = *UTC
}

func init() {
	loadTzinfoFromTzdata = androidLoadTzinfoFromTzdata
}

var allowGorootSource = true

func gorootZoneSource(goroot string) (string, bool) {
	if goroot == "" || !allowGorootSource {
		return "", false
	}
	return goroot + "/lib/time/zoneinfo.zip", true
}

func androidLoadTzinfoFromTzdata(file, name string) ([]byte, error) {
	const (
		headersize = 12 + 3*4
		namesize   = 40
		entrysize  = namesize + 3*4
	)
	if len(name) > namesize {
		return nil, errors.New(name + " is longer than the maximum zone name length (40 bytes)")
	}
	fd, err := open(file)
	if err != nil {
		return nil, err
	}
	defer closefd(fd)

	buf := make([]byte, headersize)
	if err := preadn(fd, buf, 0); err != nil {
		return nil, errors.New("corrupt tzdata file " + file)
	}
	d := dataIO{buf, false}
	if magic := d.read(6); string(magic) != "tzdata" {
		return nil, errors.New("corrupt tzdata file " + file)
	}
	d = dataIO{buf[12:], false}
	indexOff, _ := d.big4()
	dataOff, _ := d.big4()
	indexSize := dataOff - indexOff
	entrycount := indexSize / entrysize
	buf = make([]byte, indexSize)
	if err := preadn(fd, buf, int(indexOff)); err != nil {
		return nil, errors.New("corrupt tzdata file " + file)
	}
	for i := 0; i < int(entrycount); i++ {
		entry := buf[i*entrysize : (i+1)*entrysize]
		// len(name) <= namesize is checked at function entry
		if string(entry[:len(name)]) != name {
			continue
		}
		d := dataIO{entry[namesize:], false}
		off, _ := d.big4()
		size, _ := d.big4()
		buf := make([]byte, size)
		if err := preadn(fd, buf, int(off+dataOff)); err != nil {
			return nil, errors.New("corrupt tzdata file " + file)
		}
		return buf, nil
	}
	return nil, syscall.ENOENT
}

```

// === FILE: references!/go/src/time/zoneinfo_goroot.go ===
```go
// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !ios && !android

package time

func gorootZoneSource(goroot string) (string, bool) {
	if goroot == "" {
		return "", false
	}
	return goroot + "/lib/time/zoneinfo.zip", true
}

```

// === FILE: references!/go/src/time/zoneinfo_ios.go ===
```go
// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ios

package time

import (
	"syscall"
)

var platformZoneSources []string // none on iOS

func gorootZoneSource(goroot string) (string, bool) {
	// The working directory at initialization is the root of the
	// app bundle: "/private/.../bundlename.app". That's where we
	// keep zoneinfo.zip for tethered iOS builds.
	// For self-hosted iOS builds, the zoneinfo.zip is in GOROOT.
	var roots []string
	if goroot != "" {
		roots = append(roots, goroot+"/lib/time")
	}
	wd, err := syscall.Getwd()
	if err == nil {
		roots = append(roots, wd)
	}
	for _, r := range roots {
		var st syscall.Stat_t
		fd, err := syscall.Open(r, syscall.O_RDONLY, 0)
		if err != nil {
			continue
		}
		defer syscall.Close(fd)
		if err := syscall.Fstat(fd, &st); err == nil {
			return r + "/zoneinfo.zip", true
		}
	}
	return "", false
}

func initLocal() {
	// TODO(crawshaw): [NSTimeZone localTimeZone]
	localLoc = *UTC
}

```

// === FILE: references!/go/src/time/zoneinfo_js.go ===
```go
// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build js && wasm

package time

import (
	"internal/strconv"
	"syscall/js"
)

var platformZoneSources = []string{
	"/usr/share/zoneinfo/",
	"/usr/share/lib/zoneinfo/",
	"/usr/lib/locale/TZ/",
}

func initLocal() {
	localLoc.name = "Local"

	z := zone{}
	d := js.Global().Get("Date").New()
	offset := d.Call("getTimezoneOffset").Int() * -1
	z.offset = offset * 60
	// According to https://tc39.github.io/ecma262/#sec-timezoneestring,
	// the timezone name from (new Date()).toTimeString() is an implementation-dependent
	// result, and in Google Chrome, it gives the fully expanded name rather than
	// the abbreviation.
	// Hence, we construct the name from the offset.
	z.name = "UTC"
	if offset < 0 {
		z.name += "-"
		offset *= -1
	} else {
		z.name += "+"
	}
	z.name += strconv.Itoa(offset / 60)
	min := offset % 60
	if min != 0 {
		z.name += ":" + strconv.Itoa(min)
	}
	localLoc.zone = []zone{z}
}

```

// === FILE: references!/go/src/time/zoneinfo_plan9.go ===
```go
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse Plan 9 timezone(2) files.

package time

import (
	"syscall"
)

var platformZoneSources []string // none on Plan 9

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n'
}

// Copied from strings to avoid a dependency.
func fields(s string) []string {
	// First count the fields.
	n := 0
	inField := false
	for _, rune := range s {
		wasInField := inField
		inField = !isSpace(rune)
		if inField && !wasInField {
			n++
		}
	}

	// Now create them.
	a := make([]string, n)
	na := 0
	fieldStart := -1 // Set to -1 when looking for start of field.
	for i, rune := range s {
		if isSpace(rune) {
			if fieldStart >= 0 {
				a[na] = s[fieldStart:i]
				na++
				fieldStart = -1
			}
		} else if fieldStart == -1 {
			fieldStart = i
		}
	}
	if fieldStart >= 0 { // Last field might end at EOF.
		a[na] = s[fieldStart:]
	}
	return a
}

func loadZoneDataPlan9(s string) (l *Location, err error) {
	f := fields(s)
	if len(f) < 4 {
		if len(f) == 2 && f[0] == "GMT" {
			return UTC, nil
		}
		return nil, errBadData
	}

	var zones [2]zone

	// standard timezone offset
	o, err := atoi(f[1])
	if err != nil {
		return nil, errBadData
	}
	zones[0] = zone{name: f[0], offset: o, isDST: false}

	// alternate timezone offset
	o, err = atoi(f[3])
	if err != nil {
		return nil, errBadData
	}
	zones[1] = zone{name: f[2], offset: o, isDST: true}

	// transition time pairs
	var tx []zoneTrans
	f = f[4:]
	for i := 0; i < len(f); i++ {
		zi := 0
		if i%2 == 0 {
			zi = 1
		}
		t, err := atoi(f[i])
		if err != nil {
			return nil, errBadData
		}
		t -= zones[0].offset
		tx = append(tx, zoneTrans{when: int64(t), index: uint8(zi)})
	}

	// Committed to succeed.
	l = &Location{zone: zones[:], tx: tx}

	// Fill in the cache with information about right now,
	// since that will be the most common lookup.
	sec, _, _ := runtimeNow()
	for i := range tx {
		if tx[i].when <= sec && (i+1 == len(tx) || sec < tx[i+1].when) {
			l.cacheStart = tx[i].when
			l.cacheEnd = omega
			if i+1 < len(tx) {
				l.cacheEnd = tx[i+1].when
			}
			l.cacheZone = &l.zone[tx[i].index]
		}
	}

	return l, nil
}

func loadZoneFilePlan9(name string) (*Location, error) {
	b, err := readFile(name)
	if err != nil {
		return nil, err
	}
	return loadZoneDataPlan9(string(b))
}

func initLocal() {
	t, ok := syscall.Getenv("timezone")
	if ok {
		if z, err := loadZoneDataPlan9(t); err == nil {
			localLoc = *z
			return
		}
	} else {
		if z, err := loadZoneFilePlan9("/adm/timezone/local"); err == nil {
			localLoc = *z
			localLoc.name = "Local"
			return
		}
	}

	// Fall back to UTC.
	localLoc.name = "UTC"
}

```

// === FILE: references!/go/src/time/zoneinfo_read.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse "zoneinfo" time zone file.
// This is a fairly standard file format used on OS X, Linux, BSD, Sun, and others.
// See tzfile(5), https://en.wikipedia.org/wiki/Zoneinfo,
// and ftp://munnari.oz.au/pub/oldtz/

package time

import (
	"errors"
	"internal/bytealg"
	"runtime"
	"syscall"
	_ "unsafe" // for linkname
)

// registerLoadFromEmbeddedTZData is called by the time/tzdata package,
// if it is imported.
//
//go:linkname registerLoadFromEmbeddedTZData
func registerLoadFromEmbeddedTZData(f func(string) (string, error)) {
	loadFromEmbeddedTZData = f
}

// loadFromEmbeddedTZData is used to load a specific tzdata file
// from tzdata information embedded in the binary itself.
// This is set when the time/tzdata package is imported,
// via registerLoadFromEmbeddedTzdata.
var loadFromEmbeddedTZData func(zipname string) (string, error)

// maxFileSize is the max permitted size of files read by readFile.
// As reference, the zoneinfo.zip distributed by Go is ~350 KB,
// so 10MB is overkill.
const maxFileSize = 10 << 20

type fileSizeError string

func (f fileSizeError) Error() string {
	return "time: file " + string(f) + " is too large"
}

// Copies of io.Seek* constants to avoid importing "io":
const (
	seekStart   = 0
	seekCurrent = 1
	seekEnd     = 2
)

// Simple I/O interface to binary blob of data.
type dataIO struct {
	p     []byte
	error bool
}

func (d *dataIO) read(n int) []byte {
	if len(d.p) < n {
		d.p = nil
		d.error = true
		return nil
	}
	p := d.p[0:n]
	d.p = d.p[n:]
	return p
}

func (d *dataIO) big4() (n uint32, ok bool) {
	p := d.read(4)
	if len(p) < 4 {
		d.error = true
		return 0, false
	}
	return uint32(p[3]) | uint32(p[2])<<8 | uint32(p[1])<<16 | uint32(p[0])<<24, true
}

func (d *dataIO) big8() (n uint64, ok bool) {
	n1, ok1 := d.big4()
	n2, ok2 := d.big4()
	if !ok1 || !ok2 {
		d.error = true
		return 0, false
	}
	return (uint64(n1) << 32) | uint64(n2), true
}

func (d *dataIO) byte() (n byte, ok bool) {
	p := d.read(1)
	if len(p) < 1 {
		d.error = true
		return 0, false
	}
	return p[0], true
}

// rest returns the rest of the data in the buffer.
func (d *dataIO) rest() []byte {
	r := d.p
	d.p = nil
	return r
}

// Make a string by stopping at the first NUL
func byteString(p []byte) string {
	if i := bytealg.IndexByte(p, 0); i != -1 {
		p = p[:i]
	}
	return string(p)
}

var errBadData = errors.New("malformed time zone information")

// LoadLocationFromTZData returns a new [Location] with the given name
// initialized from the IANA Time Zone database-formatted data.
// The data should be in the format of a standard IANA time zone file
// (for example, the content of /etc/localtime on Unix systems).
func LoadLocationFromTZData(name string, data []byte) (*Location, error) {
	d := dataIO{data, false}

	// 4-byte magic "TZif"
	if magic := d.read(4); string(magic) != "TZif" {
		return nil, errBadData
	}

	// 1-byte version, then 15 bytes of padding
	var version int
	var p []byte
	if p = d.read(16); len(p) != 16 {
		return nil, errBadData
	} else {
		switch p[0] {
		case 0:
			version = 1
		case '2':
			version = 2
		case '3':
			version = 3
		default:
			return nil, errBadData
		}
	}

	// six big-endian 32-bit integers:
	//	number of UTC/local indicators
	//	number of standard/wall indicators
	//	number of leap seconds
	//	number of transition times
	//	number of local time zones
	//	number of characters of time zone abbrev strings
	const (
		NUTCLocal = iota
		NStdWall
		NLeap
		NTime
		NZone
		NChar
	)
	var n [6]int
	for i := 0; i < 6; i++ {
		nn, ok := d.big4()
		if !ok {
			return nil, errBadData
		}
		if uint32(int(nn)) != nn {
			return nil, errBadData
		}
		n[i] = int(nn)
	}

	// If we have version 2 or 3, then the data is first written out
	// in a 32-bit format, then written out again in a 64-bit format.
	// Skip the 32-bit format and read the 64-bit one, as it can
	// describe a broader range of dates.

	is64 := false
	if version > 1 {
		// Skip the 32-bit data.
		skip := n[NTime]*4 +
			n[NTime] +
			n[NZone]*6 +
			n[NChar] +
			n[NLeap]*8 +
			n[NStdWall] +
			n[NUTCLocal]
		// Skip the version 2 header that we just read.
		skip += 4 + 16
		d.read(skip)

		is64 = true

		// Read the counts again, they can differ.
		for i := 0; i < 6; i++ {
			nn, ok := d.big4()
			if !ok {
				return nil, errBadData
			}
			if uint32(int(nn)) != nn {
				return nil, errBadData
			}
			n[i] = int(nn)
		}
	}

	size := 4
	if is64 {
		size = 8
	}

	// Transition times.
	txtimes := dataIO{d.read(n[NTime] * size), false}

	// Time zone indices for transition times.
	txzones := d.read(n[NTime])

	// Zone info structures
	zonedata := dataIO{d.read(n[NZone] * 6), false}

	// Time zone abbreviations.
	abbrev := d.read(n[NChar])

	// Leap-second time pairs
	d.read(n[NLeap] * (size + 4))

	// Whether tx times associated with local time types
	// are specified as standard time or wall time.
	isstd := d.read(n[NStdWall])

	// Whether tx times associated with local time types
	// are specified as UTC or local time.
	isutc := d.read(n[NUTCLocal])

	if d.error { // ran out of data
		return nil, errBadData
	}

	var extend string
	rest := d.rest()
	if len(rest) > 2 && rest[0] == '\n' && rest[len(rest)-1] == '\n' {
		extend = string(rest[1 : len(rest)-1])
	}

	// Now we can build up a useful data structure.
	// First the zone information.
	//	utcoff[4] isdst[1] nameindex[1]
	nzone := n[NZone]
	if nzone == 0 {
		// Reject tzdata files with no zones. There's nothing useful in them.
		// This also avoids a panic later when we add and then use a fake transition (golang.org/issue/29437).
		return nil, errBadData
	}
	zones := make([]zone, nzone)
	for i := range zones {
		var ok bool
		var n uint32
		if n, ok = zonedata.big4(); !ok {
			return nil, errBadData
		}
		if uint32(int(n)) != n {
			return nil, errBadData
		}
		zones[i].offset = int(int32(n))
		var b byte
		if b, ok = zonedata.byte(); !ok {
			return nil, errBadData
		}
		zones[i].isDST = b != 0
		if b, ok = zonedata.byte(); !ok || int(b) >= len(abbrev) {
			return nil, errBadData
		}
		zones[i].name = byteString(abbrev[b:])
		if runtime.GOOS == "aix" && len(name) > 8 && (name[:8] == "Etc/GMT+" || name[:8] == "Etc/GMT-") {
			// There is a bug with AIX 7.2 TL 0 with files in Etc,
			// GMT+1 will return GMT-1 instead of GMT+1 or -01.
			if name != "Etc/GMT+0" {
				// GMT+0 is OK
				zones[i].name = name[4:]
			}
		}
	}

	// Now the transition time info.
	tx := make([]zoneTrans, n[NTime])
	for i := range tx {
		var n int64
		if !is64 {
			if n4, ok := txtimes.big4(); !ok {
				return nil, errBadData
			} else {
				n = int64(int32(n4))
			}
		} else {
			if n8, ok := txtimes.big8(); !ok {
				return nil, errBadData
			} else {
				n = int64(n8)
			}
		}
		tx[i].when = n
		if int(txzones[i]) >= len(zones) {
			return nil, errBadData
		}
		tx[i].index = txzones[i]
		if i < len(isstd) {
			tx[i].isstd = isstd[i] != 0
		}
		if i < len(isutc) {
			tx[i].isutc = isutc[i] != 0
		}
	}

	if len(tx) == 0 {
		// Build fake transition to cover all time.
		// This happens in fixed locations like "Etc/GMT0".
		tx = append(tx, zoneTrans{when: alpha, index: 0})
	}

	// Committed to succeed.
	l := &Location{zone: zones, tx: tx, name: name, extend: extend}

	// Fill in the cache with information about right now,
	// since that will be the most common lookup.
	sec, _, _ := runtimeNow()
	for i := range tx {
		if tx[i].when <= sec && (i+1 == len(tx) || sec < tx[i+1].when) {
			l.cacheStart = tx[i].when
			l.cacheEnd = omega
			l.cacheZone = &l.zone[tx[i].index]
			if i+1 < len(tx) {
				l.cacheEnd = tx[i+1].when
			} else if l.extend != "" {
				// If we're at the end of the known zone transitions,
				// try the extend string.
				if name, offset, estart, eend, isDST, ok := tzset(l.extend, l.cacheStart, sec); ok {
					l.cacheStart = estart
					l.cacheEnd = eend
					// Find the zone that is returned by tzset to avoid allocation if possible.
					if zoneIdx := findZone(l.zone, name, offset, isDST); zoneIdx != -1 {
						l.cacheZone = &l.zone[zoneIdx]
					} else {
						l.cacheZone = &zone{
							name:   name,
							offset: offset,
							isDST:  isDST,
						}
					}
				}
			}
			break
		}
	}

	return l, nil
}

func findZone(zones []zone, name string, offset int, isDST bool) int {
	for i, z := range zones {
		if z.name == name && z.offset == offset && z.isDST == isDST {
			return i
		}
	}
	return -1
}

// loadTzinfoFromDirOrZip returns the contents of the file with the given name
// in dir. dir can either be an uncompressed zip file, or a directory.
func loadTzinfoFromDirOrZip(dir, name string) ([]byte, error) {
	if len(dir) > 4 && dir[len(dir)-4:] == ".zip" {
		return loadTzinfoFromZip(dir, name)
	}
	if dir != "" {
		name = dir + "/" + name
	}
	return readFile(name)
}

// There are 500+ zoneinfo files. Rather than distribute them all
// individually, we ship them in an uncompressed zip file.
// Used this way, the zip file format serves as a commonly readable
// container for the individual small files. We choose zip over tar
// because zip files have a contiguous table of contents, making
// individual file lookups faster, and because the per-file overhead
// in a zip file is considerably less than tar's 512 bytes.

// get4 returns the little-endian 32-bit value in b.
func get4(b []byte) int {
	if len(b) < 4 {
		return 0
	}
	return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 | int(b[3])<<24
}

// get2 returns the little-endian 16-bit value in b.
func get2(b []byte) int {
	if len(b) < 2 {
		return 0
	}
	return int(b[0]) | int(b[1])<<8
}

// loadTzinfoFromZip returns the contents of the file with the given name
// in the given uncompressed zip file.
func loadTzinfoFromZip(zipfile, name string) ([]byte, error) {
	fd, err := open(zipfile)
	if err != nil {
		return nil, err
	}
	defer closefd(fd)

	const (
		zecheader = 0x06054b50
		zcheader  = 0x02014b50
		ztailsize = 22

		zheadersize = 30
		zheader     = 0x04034b50
	)

	buf := make([]byte, ztailsize)
	if err := preadn(fd, buf, -ztailsize); err != nil || get4(buf) != zecheader {
		return nil, errors.New("corrupt zip file " + zipfile)
	}
	n := get2(buf[10:])
	size := get4(buf[12:])
	off := get4(buf[16:])

	buf = make([]byte, size)
	if err := preadn(fd, buf, off); err != nil {
		return nil, errors.New("corrupt zip file " + zipfile)
	}

	for i := 0; i < n; i++ {
		// zip entry layout:
		//	0	magic[4]
		//	4	madevers[1]
		//	5	madeos[1]
		//	6	extvers[1]
		//	7	extos[1]
		//	8	flags[2]
		//	10	meth[2]
		//	12	modtime[2]
		//	14	moddate[2]
		//	16	crc[4]
		//	20	csize[4]
		//	24	uncsize[4]
		//	28	namelen[2]
		//	30	xlen[2]
		//	32	fclen[2]
		//	34	disknum[2]
		//	36	iattr[2]
		//	38	eattr[4]
		//	42	off[4]
		//	46	name[namelen]
		//	46+namelen+xlen+fclen - next header
		//
		if get4(buf) != zcheader {
			break
		}
		meth := get2(buf[10:])
		size := get4(buf[24:])
		namelen := get2(buf[28:])
		xlen := get2(buf[30:])
		fclen := get2(buf[32:])
		off := get4(buf[42:])
		zname := buf[46 : 46+namelen]
		buf = buf[46+namelen+xlen+fclen:]
		if string(zname) != name {
			continue
		}
		if meth != 0 {
			return nil, errors.New("unsupported compression for " + name + " in " + zipfile)
		}

		// zip per-file header layout:
		//	0	magic[4]
		//	4	extvers[1]
		//	5	extos[1]
		//	6	flags[2]
		//	8	meth[2]
		//	10	modtime[2]
		//	12	moddate[2]
		//	14	crc[4]
		//	18	csize[4]
		//	22	uncsize[4]
		//	26	namelen[2]
		//	28	xlen[2]
		//	30	name[namelen]
		//	30+namelen+xlen - file data
		//
		buf = make([]byte, zheadersize+namelen)
		if err := preadn(fd, buf, off); err != nil ||
			get4(buf) != zheader ||
			get2(buf[8:]) != meth ||
			get2(buf[26:]) != namelen ||
			string(buf[30:30+namelen]) != name {
			return nil, errors.New("corrupt zip file " + zipfile)
		}
		xlen = get2(buf[28:])

		buf = make([]byte, size)
		if err := preadn(fd, buf, off+30+namelen+xlen); err != nil {
			return nil, errors.New("corrupt zip file " + zipfile)
		}

		return buf, nil
	}

	return nil, syscall.ENOENT
}

// loadTzinfoFromTzdata returns the time zone information of the time zone
// with the given name, from a tzdata database file as they are typically
// found on android.
var loadTzinfoFromTzdata func(file, name string) ([]byte, error)

// loadTzinfo returns the time zone information of the time zone
// with the given name, from a given source. A source may be a
// timezone database directory, tzdata database file or an uncompressed
// zip file, containing the contents of such a directory.
func loadTzinfo(name string, source string) ([]byte, error) {
	if len(source) >= 6 && source[len(source)-6:] == "tzdata" {
		return loadTzinfoFromTzdata(source, name)
	}
	return loadTzinfoFromDirOrZip(source, name)
}

// loadLocation returns the Location with the given name from one of
// the specified sources. See loadTzinfo for a list of supported sources.
// The first timezone data matching the given name that is successfully loaded
// and parsed is returned as a Location.
func loadLocation(name string, sources []string) (z *Location, firstErr error) {
	for _, source := range sources {
		zoneData, err := loadTzinfo(name, source)
		if err == nil {
			if z, err = LoadLocationFromTZData(name, zoneData); err == nil {
				return z, nil
			}
		}
		if firstErr == nil && err != syscall.ENOENT {
			firstErr = err
		}
	}
	if loadFromEmbeddedTZData != nil {
		zoneData, err := loadFromEmbeddedTZData(name)
		if err == nil {
			if z, err = LoadLocationFromTZData(name, []byte(zoneData)); err == nil {
				return z, nil
			}
		}
		if firstErr == nil && err != syscall.ENOENT {
			firstErr = err
		}
	}
	if source, ok := gorootZoneSource(runtime.GOROOT()); ok {
		zoneData, err := loadTzinfo(name, source)
		if err == nil {
			if z, err = LoadLocationFromTZData(name, zoneData); err == nil {
				return z, nil
			}
		}
		if firstErr == nil && err != syscall.ENOENT {
			firstErr = err
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, errors.New("unknown time zone " + name)
}

// readFile reads and returns the content of the named file.
// It is a trivial implementation of os.ReadFile, reimplemented
// here to avoid depending on io/ioutil or os.
// It returns an error if name exceeds maxFileSize bytes.
func readFile(name string) ([]byte, error) {
	f, err := open(name)
	if err != nil {
		return nil, err
	}
	defer closefd(f)
	var (
		buf [4096]byte
		ret []byte
		n   int
	)
	for {
		n, err = read(f, buf[:])
		if n > 0 {
			ret = append(ret, buf[:n]...)
		}
		if n == 0 || err != nil {
			break
		}
		if len(ret) > maxFileSize {
			return nil, fileSizeError(name)
		}
	}
	return ret, err
}

```

// === FILE: references!/go/src/time/zoneinfo_unix.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix && !ios && !android

// Parse "zoneinfo" time zone file.
// This is a fairly standard file format used on OS X, Linux, BSD, Sun, and others.
// See tzfile(5), https://en.wikipedia.org/wiki/Zoneinfo,
// and ftp://munnari.oz.au/pub/oldtz/

package time

import (
	"syscall"
)

// Many systems use /usr/share/zoneinfo, Solaris 2 has
// /usr/share/lib/zoneinfo, IRIX 6 has /usr/lib/locale/TZ,
// NixOS has /etc/zoneinfo.
var platformZoneSources = []string{
	"/usr/share/zoneinfo/",
	"/usr/share/lib/zoneinfo/",
	"/usr/lib/locale/TZ/",
	"/etc/zoneinfo",
}

func initLocal() {
	// consult $TZ to find the time zone to use.
	// no $TZ means use the system default /etc/localtime.
	// $TZ="" means use UTC.
	// $TZ="foo" or $TZ=":foo" if foo is an absolute path, then the file pointed
	// by foo will be used to initialize timezone; otherwise, file
	// /usr/share/zoneinfo/foo will be used.

	tz, ok := syscall.Getenv("TZ")
	switch {
	case !ok:
		z, err := loadLocation("localtime", []string{"/etc"})
		if err == nil {
			localLoc = *z
			localLoc.name = "Local"
			return
		}
	case tz != "":
		if tz[0] == ':' {
			tz = tz[1:]
		}
		if tz != "" && tz[0] == '/' {
			if z, err := loadLocation(tz, []string{""}); err == nil {
				localLoc = *z
				if tz == "/etc/localtime" {
					localLoc.name = "Local"
				} else {
					localLoc.name = tz
				}
				return
			}
		} else if tz != "" && tz != "UTC" {
			if z, err := loadLocation(tz, platformZoneSources); err == nil {
				localLoc = *z
				return
			}
		}
	}

	// Fall back to UTC.
	localLoc.name = "UTC"
}

```

// === FILE: references!/go/src/time/zoneinfo_wasip1.go ===
```go
// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

// in wasip1 zoneinfo is managed by the runtime.
var platformZoneSources = []string{}

func initLocal() {
	localLoc.name = "Local"
}

```

// === FILE: references!/go/src/time/zoneinfo_windows.go ===
```go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import (
	"errors"
	"internal/syscall/windows/registry"
	"syscall"
)

var platformZoneSources []string // none: Windows uses system calls instead

// TODO(rsc): Fall back to copy of zoneinfo files.

// BUG(brainman,rsc): On Windows, the operating system does not provide complete
// time zone information.
// The implementation assumes that this year's rules for daylight savings
// time apply to all previous and future years as well.

// matchZoneKey checks if stdname and dstname match the corresponding key
// values "MUI_Std" and "MUI_Dlt" or "Std" and "Dlt" in the kname key stored
// under the open registry key zones.
func matchZoneKey(zones registry.Key, kname string, stdname, dstname string) (matched bool, err2 error) {
	k, err := registry.OpenKey(zones, kname, registry.READ)
	if err != nil {
		return false, err
	}
	defer k.Close()

	var std, dlt string
	// Try MUI_Std and MUI_Dlt first, fallback to Std and Dlt if *any* error occurs
	std, err = k.GetMUIStringValue("MUI_Std")
	if err == nil {
		dlt, err = k.GetMUIStringValue("MUI_Dlt")
	}
	if err != nil { // Fallback to Std and Dlt
		if std, _, err = k.GetStringValue("Std"); err != nil {
			return false, err
		}
		if dlt, _, err = k.GetStringValue("Dlt"); err != nil {
			return false, err
		}
	}

	if std != stdname {
		return false, nil
	}
	if dlt != dstname && dstname != stdname {
		return false, nil
	}
	return true, nil
}

// toEnglishName searches the registry for an English name of a time zone
// whose zone names are stdname and dstname and returns the English name.
func toEnglishName(stdname, dstname string) (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Time Zones`, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()

	names, err := k.ReadSubKeyNames()
	if err != nil {
		return "", err
	}
	for _, name := range names {
		matched, err := matchZoneKey(k, name, stdname, dstname)
		if err == nil && matched {
			return name, nil
		}
	}
	return "", errors.New(`English name for time zone "` + stdname + `" not found in registry`)
}

// extractCAPS extracts capital letters from description desc.
func extractCAPS(desc string) string {
	var short []rune
	for _, c := range desc {
		if 'A' <= c && c <= 'Z' {
			short = append(short, c)
		}
	}
	return string(short)
}

// abbrev returns the abbreviations to use for the given zone z.
func abbrev(z *syscall.Timezoneinformation) (std, dst string) {
	stdName := syscall.UTF16ToString(z.StandardName[:])
	a, ok := abbrs[stdName]
	if !ok {
		dstName := syscall.UTF16ToString(z.DaylightName[:])
		// Perhaps stdName is not English. Try to convert it.
		englishName, err := toEnglishName(stdName, dstName)
		if err == nil {
			a, ok = abbrs[englishName]
			if ok {
				return a.std, a.dst
			}
		}
		// fallback to using capital letters
		return extractCAPS(stdName), extractCAPS(dstName)
	}
	return a.std, a.dst
}

// pseudoUnix returns the pseudo-Unix time (seconds since Jan 1 1970 *LOCAL TIME*)
// denoted by the system date+time d in the given year.
// It is up to the caller to convert this local time into a UTC-based time.
func pseudoUnix(year int, d *syscall.Systemtime) int64 {
	// Windows specifies daylight savings information in "day in month" format:
	// d.Month is month number (1-12)
	// d.DayOfWeek is appropriate weekday (Sunday=0 to Saturday=6)
	// d.Day is week within the month (1 to 5, where 5 is last week of the month)
	// d.Hour, d.Minute and d.Second are absolute time
	day := 1
	t := Date(year, Month(d.Month), day, int(d.Hour), int(d.Minute), int(d.Second), 0, UTC)
	i := int(d.DayOfWeek) - int(t.Weekday())
	if i < 0 {
		i += 7
	}
	day += i
	if week := int(d.Day) - 1; week < 4 {
		day += week * 7
	} else {
		// "Last" instance of the day.
		day += 4 * 7
		if day > daysIn(Month(d.Month), year) {
			day -= 7
		}
	}
	return t.sec() + int64(day-1)*secondsPerDay + internalToUnix
}

func initLocalFromTZI(i *syscall.Timezoneinformation) {
	l := &localLoc

	l.name = "Local"

	nzone := 1
	if i.StandardDate.Month > 0 {
		nzone++
	}
	l.zone = make([]zone, nzone)

	stdname, dstname := abbrev(i)

	std := &l.zone[0]
	std.name = stdname
	if nzone == 1 {
		// No daylight savings.
		std.offset = -int(i.Bias) * 60
		l.cacheStart = alpha
		l.cacheEnd = omega
		l.cacheZone = std
		l.tx = make([]zoneTrans, 1)
		l.tx[0].when = l.cacheStart
		l.tx[0].index = 0
		return
	}

	// StandardBias must be ignored if StandardDate is not set,
	// so this computation is delayed until after the nzone==1
	// return above.
	std.offset = -int(i.Bias+i.StandardBias) * 60

	dst := &l.zone[1]
	dst.name = dstname
	dst.offset = -int(i.Bias+i.DaylightBias) * 60
	dst.isDST = true

	// Arrange so that d0 is first transition date, d1 second,
	// i0 is index of zone after first transition, i1 second.
	d0 := &i.StandardDate
	d1 := &i.DaylightDate
	i0 := 0
	i1 := 1
	if d0.Month > d1.Month {
		d0, d1 = d1, d0
		i0, i1 = i1, i0
	}

	// 2 tx per year, 100 years on each side of this year
	l.tx = make([]zoneTrans, 400)

	t := Now().UTC()
	year := t.Year()
	txi := 0
	for y := year - 100; y < year+100; y++ {
		tx := &l.tx[txi]
		tx.when = pseudoUnix(y, d0) - int64(l.zone[i1].offset)
		tx.index = uint8(i0)
		txi++

		tx = &l.tx[txi]
		tx.when = pseudoUnix(y, d1) - int64(l.zone[i0].offset)
		tx.index = uint8(i1)
		txi++
	}
}

var usPacific = syscall.Timezoneinformation{
	Bias: 8 * 60,
	StandardName: [32]uint16{
		'P', 'a', 'c', 'i', 'f', 'i', 'c', ' ', 'S', 't', 'a', 'n', 'd', 'a', 'r', 'd', ' ', 'T', 'i', 'm', 'e',
	},
	StandardDate: syscall.Systemtime{Month: 11, Day: 1, Hour: 2},
	DaylightName: [32]uint16{
		'P', 'a', 'c', 'i', 'f', 'i', 'c', ' ', 'D', 'a', 'y', 'l', 'i', 'g', 'h', 't', ' ', 'T', 'i', 'm', 'e',
	},
	DaylightDate: syscall.Systemtime{Month: 3, Day: 2, Hour: 2},
	DaylightBias: -60,
}

var aus = syscall.Timezoneinformation{
	Bias: -10 * 60,
	StandardName: [32]uint16{
		'A', 'U', 'S', ' ', 'E', 'a', 's', 't', 'e', 'r', 'n', ' ', 'S', 't', 'a', 'n', 'd', 'a', 'r', 'd', ' ', 'T', 'i', 'm', 'e',
	},
	StandardDate: syscall.Systemtime{Month: 4, Day: 1, Hour: 3},
	DaylightName: [32]uint16{
		'A', 'U', 'S', ' ', 'E', 'a', 's', 't', 'e', 'r', 'n', ' ', 'D', 'a', 'y', 'l', 'i', 'g', 'h', 't', ' ', 'T', 'i', 'm', 'e',
	},
	DaylightDate: syscall.Systemtime{Month: 10, Day: 1, Hour: 2},
	DaylightBias: -60,
}

func initLocal() {
	var i syscall.Timezoneinformation
	if _, err := syscall.GetTimeZoneInformation(&i); err != nil {
		localLoc.name = "UTC"
		return
	}
	initLocalFromTZI(&i)
}

```

