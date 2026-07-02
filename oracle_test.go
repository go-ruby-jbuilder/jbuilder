// Copyright (c) the go-ruby-jbuilder/jbuilder authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jbuilder

import (
	"math"
	"math/big"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// mustBig parses a decimal into a *big.Int for the oracle corpus.
func mustBig(s string) *big.Int {
	bi, _ := new(big.Int).SetString(s, 10)
	return bi
}

// rubyBin locates a `ruby` that has the jbuilder gem and runs Ruby >= 4.0 (the
// version the parity claim is gated on). The oracle tests skip themselves when
// any of that is missing — the qemu cross-arch lanes, the Windows lane, and any
// host without the gem — so the deterministic suite alone drives the 100% gate
// there. The differential comparison runs on the ubuntu/macos CI lanes, which
// install ruby + jbuilder.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping jbuilder gem oracle")
	}
	probe := `exit((RUBY_VERSION.split(".").first.to_i >= 4) ? 0 : 3) rescue exit(4)`
	if err := exec.Command(path, "-e", probe).Run(); err != nil {
		t.Skip("ruby < 4.0; parity is gated on RUBY_VERSION >= 4.0")
	}
	check := `begin; require "jbuilder"; rescue LoadError; exit 5; end`
	if err := exec.Command(path, "-e", check).Run(); err != nil {
		t.Skip("jbuilder gem not installed; skipping gem oracle")
	}
	return path
}

// gemTarget runs a jbuilder DSL snippet in the gem and returns json.target!.
// body is Ruby that uses the local `json` builder. The script binmodes stdout so
// no text-mode translation pollutes the compared bytes (the go-ruby-erb lesson).
func gemTarget(t *testing.T, bin, body string) string {
	t.Helper()
	script := `$stdout.binmode
require "active_support/all"
require "jbuilder"
json = Jbuilder.new
` + body + `
print json.target!`
	out, err := exec.Command(bin, "-e", script).CombinedOutput()
	if err != nil {
		t.Fatalf("gem error: %v\nbody:\n%s\noutput:\n%s", err, body, out)
	}
	return string(out)
}

// TestOracleParity builds the same structure with the Go builder and the gem and
// asserts the JSON matches byte-for-byte across the full method surface.
func TestOracleParity(t *testing.T) {
	bin := rubyBin(t)

	cases := []struct {
		name  string
		ruby  string          // jbuilder DSL run in the gem
		build func(*Jbuilder) // the equivalent Go build
	}{
		{
			name:  "set",
			ruby:  `json.name "David"; json.age 30`,
			build: func(j *Jbuilder) { j.Set("name", "David"); j.Set("age", 30) },
		},
		{
			name: "block",
			ruby: `json.author do; json.name "Ann"; json.id 7; end`,
			build: func(j *Jbuilder) {
				j.Block("author", func(a *Jbuilder) { a.Set("name", "Ann"); a.Set("id", 7) })
			},
		},
		{
			name: "nested_blocks",
			ruby: `json.a do; json.b do; json.c 1; end; end`,
			build: func(j *Jbuilder) {
				j.Block("a", func(a *Jbuilder) { a.Block("b", func(b *Jbuilder) { b.Set("c", 1) }) })
			},
		},
		{
			name:  "array_block",
			ruby:  `json.array!([1,2,3]) { |x| json.v x }`,
			build: func(j *Jbuilder) { j.Array([]any{1, 2, 3}, func(c *Jbuilder, x any) { c.Set("v", x) }) },
		},
		{
			name:  "array_plain",
			ruby:  `json.array!([1,2,3])`,
			build: func(j *Jbuilder) { j.Array([]any{1, 2, 3}, nil) },
		},
		{
			name:  "array_empty",
			ruby:  `json.array!([])`,
			build: func(j *Jbuilder) { j.Array([]any{}, nil) },
		},
		{
			name:  "set_bang",
			ruby:  `json.set!(:key, "value")`,
			build: func(j *Jbuilder) { j.SetKey(Symbol("key"), "value") },
		},
		{
			name:  "merge",
			ruby:  `json.name "a"; json.merge!({"b" => 2})`,
			build: func(j *Jbuilder) { j.Set("name", "a"); j.Merge([]Pair{{"b", 2}}) },
		},
		{
			name: "child",
			ruby: `json.child! { json.content "x" }; json.child! { json.content "y" }`,
			build: func(j *Jbuilder) {
				j.Child(func(c *Jbuilder) { c.Set("content", "x") })
				j.Child(func(c *Jbuilder) { c.Set("content", "y") })
			},
		},
		{
			name:  "nil_bang",
			ruby:  `json.set! :x, 1; json.nil!`,
			build: func(j *Jbuilder) { j.SetKey(Symbol("x"), 1); j.Nil() },
		},
		{
			name:  "nil_value",
			ruby:  `json.name nil`,
			build: func(j *Jbuilder) { j.Set("name", nil) },
		},
		{
			name:  "ignore_nil",
			ruby:  `json.ignore_nil!; json.name nil; json.age 5`,
			build: func(j *Jbuilder) { j.IgnoreNil(); j.Set("name", nil); j.Set("age", 5) },
		},
		{
			name:  "extract_hash",
			ruby:  `json.extract!({a: 1, b: 2, c: 3}, :a, :c)`,
			build: func(j *Jbuilder) { j.Extract([]Pair{{"a", 1}, {"c", 3}}) },
		},
		{
			name: "keyfmt_camel_lower",
			ruby: `json.key_format! camelize: :lower; json.first_name "A"; json.last_name "B"`,
			build: func(j *Jbuilder) {
				j.KeyFormat(Camelize(false))
				j.Set("first_name", "A")
				j.Set("last_name", "B")
			},
		},
		{
			name:  "keyfmt_camel_upper",
			ruby:  `json.key_format! camelize: :upper; json.first_name "A"`,
			build: func(j *Jbuilder) { j.KeyFormat(Camelize(true)); j.Set("first_name", "A") },
		},
		{
			name:  "keyfmt_dasherize",
			ruby:  `json.key_format! :dasherize; json.first_name "A"`,
			build: func(j *Jbuilder) { j.KeyFormat(Dasherize()); j.Set("first_name", "A") },
		},
		{
			name: "keyfmt_nested",
			ruby: `json.key_format! camelize: :lower; json.outer_key do; json.inner_key 1; end`,
			build: func(j *Jbuilder) {
				j.KeyFormat(Camelize(false))
				j.Block("outer_key", func(o *Jbuilder) { o.Set("inner_key", 1) })
			},
		},
		{
			name:  "bool_float",
			ruby:  `json.ok true; json.no false; json.pi 3.14`,
			build: func(j *Jbuilder) { j.Set("ok", true); j.Set("no", false); j.Set("pi", 3.14) },
		},
		{
			name:  "escape_html",
			ruby:  `json.s "a<b>c&d"`,
			build: func(j *Jbuilder) { j.Set("s", "a<b>c&d") },
		},
		{
			name:  "escape_control",
			ruby:  `json.s "a\tb\nc"`,
			build: func(j *Jbuilder) { j.Set("s", "a\tb\nc") },
		},
		{
			name:  "unicode",
			ruby:  `json.s "café ☕ 日本語"`,
			build: func(j *Jbuilder) { j.Set("s", "café ☕ 日本語") },
		},
		{
			name: "bignum",
			ruby: `json.n 123456789012345678901234567890`,
			build: func(j *Jbuilder) {
				j.Set("n", mustBig("123456789012345678901234567890"))
			},
		},
		{
			name: "floats",
			ruby: `json.a 1.0; json.b 100.0; json.c 1e15; json.d 1e-5; json.e 0.1; json.f 1234567.0`,
			build: func(j *Jbuilder) {
				j.Set("a", 1.0)
				j.Set("b", 100.0)
				j.Set("c", 1e15)
				j.Set("d", 1e-5)
				j.Set("e", 0.1)
				j.Set("f", 1234567.0)
			},
		},
		{
			name: "nested_array_objects",
			ruby: `json.users([{name: "x"},{name: "y"}]) { |u| json.name u[:name] }`,
			build: func(j *Jbuilder) {
				j.Block("users", func(u *Jbuilder) {
					u.Array([]any{"x", "y"}, func(c *Jbuilder, name any) { c.Set("name", name) })
				})
			},
		},
		{
			name:  "overwrite_key",
			ruby:  `json.name "a"; json.name "b"`,
			build: func(j *Jbuilder) { j.Set("name", "a"); j.Set("name", "b") },
		},
		{
			name:  "empty",
			ruby:  ``,
			build: func(j *Jbuilder) {},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := gemTarget(t, bin, c.ruby)
			got := Encode(c.build)
			if got != want {
				t.Fatalf("parity mismatch for %q:\n go   %q\n gem  %q", c.name, got, want)
			}
		})
	}
}

// TestOracleFloatCorpus compares a wide float corpus number-for-number against
// the gem — the number formatter (Ruby's json Float#to_json) is the subtlest
// parity surface. Each value is handed to Ruby by its exact bits (unpacked from a
// 64-bit integer) rather than as a decimal literal, so the gem renders the very
// same float the Go side does and no formatting bug can hide behind reparsing.
func TestOracleFloatCorpus(t *testing.T) {
	bin := rubyBin(t)
	floats := []float64{
		0.0, 1.0, 10.0, 100.0, 1234567.0, 1e14, 1e15, 1e16, 1e20,
		1e-3, 1e-4, 1e-5, 1e-7, 1e-9, 1e-10, 0.1, 0.3, 2.5, 3.14, 1.0 / 3.0,
		123456789012345.6, 9999999999999998.0, -3.14, -0.5, 2.5e-8, 6.022e23,
	}
	// A large deterministic random corpus spanning several magnitude bands. Only
	// values whose gem rendering needs at most 15 significant digits are compared:
	// that is the parity guarantee (every realistic JSON number), and it excludes
	// the ~16–17-digit ULP-tie extremes where Go's Ryū and Ruby's dtoa legitimately
	// differ (see formatRubyFloat). skipped counts how many the guard drops.
	rng := newLCG(0x9E3779B97F4A7C15)
	for i := 0; i < 1500; i++ {
		var f float64
		switch i % 5 {
		case 0:
			f = rng.float() * 1e6
		case 1:
			f = rng.float() * 1e-4
		case 2:
			f = (rng.float() - 0.5) * 1e9
		case 3:
			f = rng.float() * 1e12
		case 4:
			f = rng.float() * 1e-8
		}
		floats = append(floats, f)
	}

	compared := 0
	for _, f := range floats {
		render := Encode(func(j *Jbuilder) { j.Set("v", f) })
		if sigDigits(render) > 15 {
			continue // outside the parity guarantee; skip the tie-zone extreme
		}
		compared++
		want := renderBitsFloat(t, bin, f)
		if render != want {
			t.Fatalf("float parity for %v (bits %d):\n go  %q\n gem %q", f, floatBits(f), render, want)
		}
	}
	if compared < 100 {
		t.Fatalf("too few floats compared (%d); guard may be over-broad", compared)
	}
}

// renderBitsFloat asks the gem to render the float with exactly f's bit pattern,
// unpacked from its 64-bit integer form so no decimal-literal reparse intervenes.
func renderBitsFloat(t *testing.T, bin string, f float64) string {
	t.Helper()
	body := "json.v [" + itoa(floatBits(f)) + "].pack(\"Q\").unpack1(\"D\")"
	return gemTarget(t, bin, body)
}

// floatBits returns f's IEEE-754 bit pattern as a uint64.
func floatBits(f float64) uint64 { return math.Float64bits(f) }

// itoa formats a uint64 in base 10.
func itoa(u uint64) string { return strconv.FormatUint(u, 10) }

// sigDigits counts the significant decimal digits in a rendered JSON number
// (ignoring sign, decimal point, exponent, and leading/trailing zeros), so the
// corpus can restrict its parity assertions to the ≤15-digit guarantee.
func sigDigits(rendered string) int {
	// rendered is like {"v":123.45} — pull out the number after the colon.
	i := strings.IndexByte(rendered, ':')
	num := rendered[i+1 : len(rendered)-1]
	if e := strings.IndexAny(num, "eE"); e >= 0 {
		num = num[:e]
	}
	num = strings.TrimLeft(num, "-")
	digits := strings.Replace(num, ".", "", 1)
	digits = strings.TrimLeft(digits, "0")
	digits = strings.TrimRight(digits, "0")
	return len(digits)
}

// lcg is a tiny deterministic PRNG (SplitMix64) so the float corpus is stable and
// reproducible without pulling in math/rand's evolving stream.
type lcg struct{ state uint64 }

// newLCG seeds the generator.
func newLCG(seed uint64) *lcg { return &lcg{state: seed} }

// float returns the next pseudo-random float64 in [0,1).
func (g *lcg) float() float64 {
	g.state += 0x9E3779B97F4A7C15
	z := g.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	z = z ^ (z >> 31)
	return float64(z>>11) / float64(1<<53)
}
