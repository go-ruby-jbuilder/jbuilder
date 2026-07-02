// Copyright (c) the go-ruby-jbuilder/jbuilder authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jbuilder

import (
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// encodeValue serialises a value drawn from the Jbuilder value model into the
// compact, MRI-`jbuilder`-faithful JSON the gem's target! produces. The gem
// renders through ActiveSupport's JSON encoder (escape_html_entities_in_json on,
// ensure_ascii off), so this reproduces that encoder's exact byte output:
// insertion-ordered object keys, `<`/`>`/`&`/` `/` `
// escaping, literal (unescaped) non-ASCII UTF-8, and Ruby Float#to_s numbers.
func encodeValue(b *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		encodeString(b, x)
	case Symbol:
		encodeString(b, string(x))
	case int:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int8:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int16:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int32:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case uint:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint8:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint16:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint32:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case uint64:
		b.WriteString(strconv.FormatUint(x, 10))
	case *big.Int:
		b.WriteString(x.String())
	case float32:
		b.WriteString(formatRubyFloat(float64(x)))
	case float64:
		b.WriteString(formatRubyFloat(x))
	case []any:
		encodeArray(b, x)
	case *Jbuilder:
		encodeValue(b, x.value())
	case *object:
		encodeObject(b, x.keys, x.vals)
	case map[string]any:
		encodeGoMap(b, x)
	default:
		// Anything outside the model degrades to its Go string form as a JSON
		// string, mirroring jbuilder handing an unknown object to to_json (which
		// stringifies). This keeps the encoder total.
		encodeString(b, defaultString(x))
	}
}

// encodeArray renders a JSON array.
func encodeArray(b *strings.Builder, items []any) {
	b.WriteByte('[')
	for i, it := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		encodeValue(b, it)
	}
	b.WriteByte(']')
}

// encodeObject renders parallel key/value slices as a JSON object, preserving
// insertion order exactly as jbuilder's underlying Hash does.
func encodeObject(b *strings.Builder, keys []string, vals []any) {
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		encodeString(b, k)
		b.WriteByte(':')
		encodeValue(b, vals[i])
	}
	b.WriteByte('}')
}

// encodeGoMap renders a Go map. Go maps have no defined order, so — to stay
// deterministic — keys are emitted sorted, matching how a host would sort before
// building. (Ordered data should use the builder's object model.)
func encodeGoMap(b *strings.Builder, m map[string]any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		encodeString(b, k)
		b.WriteByte(':')
		encodeValue(b, m[k])
	}
	b.WriteByte('}')
}

// hexDigits is the lowercase hex alphabet used for \uXXXX escapes.
const hexDigits = "0123456789abcdef"

// encodeString writes s as a JSON string literal using ActiveSupport's JSON
// escaping rules (the ones jbuilder emits): the mandatory JSON escapes, the short
// forms for \b \t \n \f \r, `\u00XX` for the remaining C0 controls, HTML-safety
// escapes for `<` `>` `&` (as < > &), the line/paragraph
// separators U+2028/U+2029 as  / , and everything else — including all
// non-ASCII — passed through as literal UTF-8 bytes.
func encodeString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		case '<':
			b.WriteString(`\u003c`)
		case '>':
			b.WriteString(`\u003e`)
		case '&':
			b.WriteString(`\u0026`)
		case '\u2028':
			b.WriteString(`\u2028`)
		case '\u2029':
			b.WriteString(`\u2029`)
		default:
			if r < 0x20 {
				b.WriteString(`\u00`)
				b.WriteByte(hexDigits[(r>>4)&0xf])
				b.WriteByte(hexDigits[r&0xf])
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// formatRubyFloat renders f the way Ruby's Float#to_s does — the shortest decimal
// that round-trips, always with a decimal point, switching to `e` notation only
// when the decimal point sits beyond Ruby's fixed-notation window (decpt > 15 or
// decpt <= -4). This differs from Go's %g (which drops trailing `.0`, and uses
// exponents at different thresholds), so JSON numbers match the gem byte-for-byte.
func formatRubyFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}

	neg := false
	// Preserve the sign, including negative zero (Ruby prints "-0.0").
	if f < 0 || (f == 0 && 1/f < 0) {
		neg = true
		f = -f
	}

	if f == 0 {
		if neg {
			return "-0.0"
		}
		return "0.0"
	}

	// Shortest round-tripping digits in scientific form: "d.dddde±XX".
	sci := strconv.FormatFloat(f, 'e', -1, 64)
	mant, exp := splitSci(sci)
	// decpt is the position of the decimal point relative to the first digit:
	// value == 0.<digits> * 10^decpt. For mantissa "d.ddd" with exponent e,
	// decpt = e + 1.
	digits := strings.Replace(mant, ".", "", 1)
	decpt := exp + 1

	// Ruby's flo_to_s switches to exponent notation when the decimal point falls
	// outside its fixed window. Empirically (matching MRI byte-for-byte): exp when
	// the point is more than four places left of the first digit (decpt <= -4,
	// e.g. 1e-5), or when it sits past the 15-integer-digit ceiling *and* at or
	// beyond the last significant digit (decpt > 15 && decpt >= len(digits), e.g.
	// 1e15 or 9999999999999998.0). A long value whose significant digits reach
	// past the point (1234567890123456.8, decpt 16 but 17 digits) stays fixed.
	var out string
	if decpt <= -4 || (decpt > 15 && decpt >= len(digits)) {
		out = rubyExp(digits, decpt)
	} else {
		out = rubyFixed(digits, decpt)
	}
	if neg {
		return "-" + out
	}
	return out
}

// splitSci breaks strconv's 'e' output ("1.5e+20", "1e-05", "-3.14e+00") into its
// mantissa ("1.5") and integer exponent (20). The sign has already been stripped
// by the caller, so mant is unsigned here.
func splitSci(s string) (mant string, exp int) {
	i := strings.IndexByte(s, 'e')
	mant = s[:i]
	e, _ := strconv.Atoi(s[i+1:])
	return mant, e
}

// rubyFixed formats the significant digits in Ruby's fixed (non-exponent) style
// given the decimal-point position decpt, always leaving at least one digit on
// each side of the point.
func rubyFixed(digits string, decpt int) string {
	var b strings.Builder
	switch {
	case decpt <= 0:
		// 0.00ddd — decpt leading zeros after the point.
		b.WriteString("0.")
		for i := 0; i < -decpt; i++ {
			b.WriteByte('0')
		}
		b.WriteString(digits)
	case decpt >= len(digits):
		// ddd000.0 — digits then trailing zeros, then ".0".
		b.WriteString(digits)
		for i := len(digits); i < decpt; i++ {
			b.WriteByte('0')
		}
		b.WriteString(".0")
	default:
		// dd.ddd — split the digit run at the decimal point.
		b.WriteString(digits[:decpt])
		b.WriteByte('.')
		b.WriteString(digits[decpt:])
	}
	return b.String()
}

// rubyExp formats the digits in Ruby's exponent style: "d.ddde±NN" with a signed,
// at-least-two-digit exponent and a mandatory fractional part ("1.0e+15").
func rubyExp(digits string, decpt int) string {
	var b strings.Builder
	b.WriteByte(digits[0])
	b.WriteByte('.')
	if len(digits) > 1 {
		b.WriteString(digits[1:])
	} else {
		b.WriteByte('0')
	}
	b.WriteByte('e')
	e := decpt - 1
	if e < 0 {
		b.WriteByte('-')
		e = -e
	} else {
		b.WriteByte('+')
	}
	es := strconv.Itoa(e)
	if len(es) < 2 {
		b.WriteByte('0')
	}
	b.WriteString(es)
	return b.String()
}
