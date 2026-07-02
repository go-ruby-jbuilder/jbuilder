// Copyright (c) the go-ruby-jbuilder/jbuilder authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jbuilder

import (
	"math"
	"math/big"
	"testing"
)

// eq is a small string-equality helper with a labelled failure.
func eq(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s:\n got %q\nwant %q", label, got, want)
	}
}

func TestSetAndOrder(t *testing.T) {
	eq(t, "set", Encode(func(j *Jbuilder) {
		j.Set("name", "David")
		j.Set("age", 30)
	}), `{"name":"David","age":30}`)

	// Setting an existing key overwrites in place (order preserved, last wins).
	eq(t, "overwrite", Encode(func(j *Jbuilder) {
		j.Set("name", "a")
		j.Set("other", 1)
		j.Set("name", "b")
	}), `{"name":"b","other":1}`)
}

func TestSetKeySymbol(t *testing.T) {
	eq(t, "setkey", Encode(func(j *Jbuilder) {
		j.SetKey(Symbol("key"), "value")
	}), `{"key":"value"}`)
}

func TestBlockNested(t *testing.T) {
	eq(t, "block", Encode(func(j *Jbuilder) {
		j.Block("author", func(a *Jbuilder) {
			a.Set("name", "Ann")
			a.Set("id", 7)
		})
	}), `{"author":{"name":"Ann","id":7}}`)

	// An empty block yields {}.
	eq(t, "emptyblock", Encode(func(j *Jbuilder) {
		j.Block("x", func(a *Jbuilder) {})
	}), `{"x":{}}`)

	// Deeply nested blocks.
	eq(t, "deep", Encode(func(j *Jbuilder) {
		j.Block("a", func(a *Jbuilder) {
			a.Block("b", func(b *Jbuilder) {
				b.Set("c", 1)
			})
		})
	}), `{"a":{"b":{"c":1}}}`)
}

func TestArray(t *testing.T) {
	eq(t, "array", Encode(func(j *Jbuilder) {
		j.Array([]any{1, 2, 3}, func(c *Jbuilder, x any) { c.Set("v", x) })
	}), `[{"v":1},{"v":2},{"v":3}]`)

	// nil fn passes elements through verbatim.
	eq(t, "arrayplain", Encode(func(j *Jbuilder) {
		j.Array([]any{1, 2, 3}, nil)
	}), `[1,2,3]`)

	// Empty collection.
	eq(t, "arrayempty", Encode(func(j *Jbuilder) {
		j.Array([]any{}, func(c *Jbuilder, x any) { c.Set("v", x) })
	}), `[]`)

	// Array nested under a key.
	eq(t, "arrayunderkey", Encode(func(j *Jbuilder) {
		j.Block("list", func(l *Jbuilder) {
			l.Array([]any{1, 2}, func(c *Jbuilder, x any) { c.Set("n", x) })
		})
	}), `{"list":[{"n":1},{"n":2}]}`)
}

func TestChild(t *testing.T) {
	eq(t, "child", Encode(func(j *Jbuilder) {
		j.Child(func(c *Jbuilder) { c.Set("a", 1) })
		j.Child(func(c *Jbuilder) { c.Set("a", 2) })
	}), `[{"a":1},{"a":2}]`)

	eq(t, "childone", Encode(func(j *Jbuilder) {
		j.Child(func(c *Jbuilder) { c.Set("x", 1) })
	}), `[{"x":1}]`)
}

func TestNil(t *testing.T) {
	eq(t, "nil", Encode(func(j *Jbuilder) {
		j.Set("x", 1)
		j.Nil()
	}), `null`)

	// A plain nil value under a key is kept as null (no ignore_nil).
	eq(t, "nilval", Encode(func(j *Jbuilder) {
		j.Set("name", nil)
	}), `{"name":null}`)
}

func TestMerge(t *testing.T) {
	// merge! appends pairs without de-duplicating against existing keys.
	eq(t, "merge", Encode(func(j *Jbuilder) {
		j.Set("name", "a")
		j.Merge([]Pair{{"b", 2}})
	}), `{"name":"a","b":2}`)

	eq(t, "mergedup", Encode(func(j *Jbuilder) {
		j.Set("a", 1)
		j.Merge([]Pair{{"a", 99}, {"c", 3}})
	}), `{"a":1,"a":99,"c":3}`)

	// Merge onto a fresh builder makes it an object.
	eq(t, "mergefresh", Encode(func(j *Jbuilder) {
		j.Merge([]Pair{{"k", "v"}})
	}), `{"k":"v"}`)
}

func TestMergeArray(t *testing.T) {
	eq(t, "mergearr", Encode(func(j *Jbuilder) {
		j.Array([]any{1}, nil)
		j.MergeArray([]any{2, 3})
	}), `[1,2,3]`)

	// MergeArray on a fresh builder starts the array.
	eq(t, "mergearrfresh", Encode(func(j *Jbuilder) {
		j.MergeArray([]any{1, 2})
	}), `[1,2]`)
}

func TestExtract(t *testing.T) {
	eq(t, "extract", Encode(func(j *Jbuilder) {
		j.Extract([]Pair{{"a", 1}, {"b", 2}})
	}), `{"a":1,"b":2}`)

	// A nil attribute renders as null.
	eq(t, "extractnil", Encode(func(j *Jbuilder) {
		j.Extract([]Pair{{"a", 1}, {"b", nil}})
	}), `{"a":1,"b":null}`)
}

func TestKeyFormat(t *testing.T) {
	eq(t, "camelLower", Encode(func(j *Jbuilder) {
		j.KeyFormat(Camelize(false))
		j.Set("first_name", "A")
		j.Set("last_name", "B")
	}), `{"firstName":"A","lastName":"B"}`)

	eq(t, "camelUpper", Encode(func(j *Jbuilder) {
		j.KeyFormat(Camelize(true))
		j.Set("first_name", "A")
	}), `{"FirstName":"A"}`)

	eq(t, "dasherize", Encode(func(j *Jbuilder) {
		j.KeyFormat(Dasherize())
		j.Set("first_name", "A")
	}), `{"first-name":"A"}`)

	eq(t, "underscore", Encode(func(j *Jbuilder) {
		j.KeyFormat(Underscore())
		j.Set("firstName", "A")
	}), `{"first_name":"A"}`)

	// key_format! is inherited by nested blocks.
	eq(t, "camelNested", Encode(func(j *Jbuilder) {
		j.KeyFormat(Camelize(false))
		j.Block("outer_key", func(o *Jbuilder) {
			o.Set("inner_key", 1)
		})
	}), `{"outerKey":{"innerKey":1}}`)

	// Clearing the format (no ops) restores raw keys.
	eq(t, "clear", Encode(func(j *Jbuilder) {
		j.KeyFormat(Camelize(false))
		j.KeyFormat()
		j.Set("first_name", "A")
	}), `{"first_name":"A"}`)
}

func TestIgnoreNil(t *testing.T) {
	eq(t, "ignore", Encode(func(j *Jbuilder) {
		j.IgnoreNil()
		j.Set("name", nil)
		j.Set("age", 5)
	}), `{"age":5}`)

	// ignore_nil!(false) turns it back off.
	eq(t, "ignoreOff", Encode(func(j *Jbuilder) {
		j.IgnoreNil(true)
		j.IgnoreNil(false)
		j.Set("name", nil)
	}), `{"name":null}`)

	// Inherited by children.
	eq(t, "ignoreChild", Encode(func(j *Jbuilder) {
		j.IgnoreNil()
		j.Block("a", func(a *Jbuilder) {
			a.Set("x", nil)
			a.Set("y", 1)
		})
	}), `{"a":{"y":1}}`)
}

func TestTargetJSONAlias(t *testing.T) {
	j := New()
	j.Set("a", 1)
	eq(t, "targetjson", j.TargetJSON(), `{"a":1}`)
}

func TestScalarTypes(t *testing.T) {
	eq(t, "bool", Encode(func(j *Jbuilder) {
		j.Set("ok", true)
		j.Set("no", false)
	}), `{"ok":true,"no":false}`)

	eq(t, "symval", Encode(func(j *Jbuilder) {
		j.Set("a", Symbol("hello"))
	}), `{"a":"hello"}`)

	eq(t, "bignum", Encode(func(j *Jbuilder) {
		bi, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
		j.Set("n", bi)
	}), `{"n":123456789012345678901234567890}`)

	eq(t, "ints", Encode(func(j *Jbuilder) {
		j.Set("a", int8(1))
		j.Set("b", int16(2))
		j.Set("c", int32(3))
		j.Set("d", int64(4))
		j.Set("e", -7)
	}), `{"a":1,"b":2,"c":3,"d":4,"e":-7}`)

	eq(t, "uints", Encode(func(j *Jbuilder) {
		j.Set("a", uint(1))
		j.Set("b", uint8(2))
		j.Set("c", uint16(3))
		j.Set("d", uint32(4))
		j.Set("e", uint64(5))
	}), `{"a":1,"b":2,"c":3,"d":4,"e":5}`)

	eq(t, "float32", Encode(func(j *Jbuilder) {
		j.Set("x", float32(0.5))
	}), `{"x":0.5}`)

	// A float64 value goes through the encoder's float64 case.
	eq(t, "float64", Encode(func(j *Jbuilder) {
		j.Set("pi", 3.14)
	}), `{"pi":3.14}`)
}

func TestNestedContainers(t *testing.T) {
	eq(t, "hashval", Encode(func(j *Jbuilder) {
		j.Set("data", map[string]any{"a": 1, "b": []any{1, 2}})
	}), `{"data":{"a":1,"b":[1,2]}}`)

	// A raw []any of scalars.
	eq(t, "tags", Encode(func(j *Jbuilder) {
		j.Set("tags", []any{"a", "b"})
	}), `{"tags":["a","b"]}`)

	// Nested *Jbuilder value.
	eq(t, "nestedjb", func() string {
		inner := New()
		inner.Set("k", 1)
		outer := New()
		outer.Set("wrap", inner)
		return outer.Encode()
	}(), `{"wrap":{"k":1}}`)

	// Array of Go maps.
	eq(t, "arrhashes", Encode(func(j *Jbuilder) {
		j.Set("items", []any{map[string]any{"a": 1}, map[string]any{"b": 2}})
	}), `{"items":[{"a":1},{"b":2}]}`)
}

// stringer exercises the encoder's fallback path for out-of-model values.
type stringer struct{ s string }

func (s stringer) String() string { return s.s }

func TestEncoderFallback(t *testing.T) {
	// A fmt.Stringer degrades to its String().
	eq(t, "stringer", Encode(func(j *Jbuilder) {
		j.Set("x", stringer{"hi"})
	}), `{"x":"hi"}`)

	// An arbitrary type degrades to %v.
	eq(t, "arbitrary", Encode(func(j *Jbuilder) {
		j.Set("x", struct{ A int }{A: 5})
	}), `{"x":"{5}"}`)
}

func TestStringEscaping(t *testing.T) {
	cases := []struct{ in, want string }{
		{`a"b`, `"a\"b"`},
		{`a\b`, `"a\\b"`},
		{"a\tb\nc\rd\fe\bf", `"a\tb\nc\rd\fe\bf"`},
		{"a<b>c&d", `"a\u003cb\u003ec\u0026d"`},
		{"\u2028\u2029", `"\u2028\u2029"`},
		{"\x01\x1f", `"\u0001\u001f"`},
		{"caf\u00e9 \u2615", "\"caf\u00e9 \u2615\""}, // non-ASCII stays literal UTF-8
		{"a/b", `"a/b"`},                             // forward slash is NOT escaped
		{"", `""`},
		{"\x7f", "\"\x7f\""}, // DEL stays literal
	}
	for _, c := range cases {
		got := Encode(func(j *Jbuilder) { j.Set("s", c.in) })
		want := `{"s":` + c.want + `}`
		eq(t, "escape "+c.in, got, want)
	}
}

func TestFloatFormatting(t *testing.T) {
	cases := []struct {
		f    float64
		want string
	}{
		{0.0, "0.0"},
		{1.0, "1.0"},
		{100.0, "100.0"},
		{1234567.0, "1234567.0"},
		{1e15, "1e+15"},
		{1e16, "1e+16"},
		{1e14, "100000000000000.0"},
		{1e-3, "0.001"},
		{1e-4, "0.0001"},
		{1e-5, "0.00001"},
		{1e20, "1e+20"},
		{3.14, "3.14"},
		{0.1, "0.1"},
		{1.0 / 3.0, "0.3333333333333333"},
		{1e-7, "0.0000001"},
		{2.5, "2.5"},
		{9999999999999998.0, "9.999999999999998e+15"},
		{1.7976931348623157e308, "1.7976931348623157e+308"},
		{123456789012345.6, "123456789012345.6"},
		{1e-9, "0.000000001"},
		{1e-10, "1e-10"},
		{2.5e-8, "0.000000025"},
		{6.022e23, "6.022e+23"},
	}
	for _, c := range cases {
		got := formatRubyFloat(c.f)
		eq(t, "float", got, c.want)
	}

	// Sign / special values.
	eq(t, "negzero", formatRubyFloat(math.Copysign(0, -1)), "-0.0")
	eq(t, "neg", formatRubyFloat(-3.14), "-3.14")
	eq(t, "nan", formatRubyFloat(math.NaN()), "NaN")
	eq(t, "inf", formatRubyFloat(math.Inf(1)), "Infinity")
	eq(t, "neginf", formatRubyFloat(math.Inf(-1)), "-Infinity")
}

func TestKeyFormatInflections(t *testing.T) {
	// camelize(:lower)
	camL := map[string]string{
		"first_name": "firstName", "b_c": "bC", "user_id": "userId",
		"http_response": "httpResponse", "a1_b2": "a1B2", "already": "already",
		"foo__bar": "fooBar", "a": "a",
	}
	for in, want := range camL {
		eq(t, "camL "+in, camelize(in, false), want)
	}
	// camelize(:upper)
	camU := map[string]string{
		"first_name": "FirstName", "b_c": "BC", "user_id": "UserId",
		"http_response": "HttpResponse", "a1_b2": "A1B2", "id": "Id",
		"foo__bar": "FooBar", "a": "A",
	}
	for in, want := range camU {
		eq(t, "camU "+in, camelize(in, true), want)
	}
	// slash → :: in camelize
	eq(t, "slash", camelize("admin/user", true), "Admin::User")
	eq(t, "slashLower", camelize("admin/user", false), "admin::User")
	// leading non-alnum segment preserved
	eq(t, "leadnil", camelize("", true), "")
	// A non-delimiter, non-alnum character in the tail is passed through verbatim
	// (exercises camelize's default branch) and capitalizeSegment on a symbol.
	eq(t, "dot", camelize("a.b", true), "A.b")
	// A segment whose first letter is already uppercase drives capitalizeSegment
	// through upperByte's non-lowercase branch.
	eq(t, "seg_upper", camelize("a_Bc", true), "ABc")

	// underscore
	under := map[string]string{
		"FirstName": "first_name", "http_response": "http_response",
		"HTTPResponse": "http_response", "userId": "user_id",
		"iOS": "i_os", "last-name": "last_name", "Camel": "camel",
		"a1_b2": "a1_b2",
	}
	for in, want := range under {
		eq(t, "under "+in, underscore(in), want)
	}
	// :: handled by underscore
	eq(t, "underns", underscore("Admin::User"), "admin/user")

	// dasherize
	eq(t, "dash", Dasherize()("first_name"), "first-name")
	eq(t, "dashnoop", Dasherize()("FirstName"), "FirstName")

	// chained ops compose left to right
	f := &keyFormatter{ops: []KeyOp{Underscore(), Camelize(false)}}
	eq(t, "chain", f.format("FirstName"), "firstName")
}

func TestValueEdgeCases(t *testing.T) {
	// An undecided builder is an empty object.
	eq(t, "undecided", New().Encode(), `{}`)

	// An array that was declared but never filled renders [].
	j := New()
	j.Array(nil, nil)
	eq(t, "nilarr", j.Encode(), `[]`)

	// kindValue with a nil value.
	j2 := New()
	j2.Nil()
	eq(t, "nilkind", j2.Encode(), `null`)

	// An array whose backing slice was never allocated (MergeArray of nothing)
	// still renders [].
	j3 := New()
	j3.MergeArray(nil)
	eq(t, "arrnilbacking", j3.Encode(), `[]`)
}
