<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-jbuilder/brand/main/social/go-ruby-jbuilder-jbuilder.png" alt="go-ruby-jbuilder/jbuilder" width="720"></p>

# jbuilder — go-ruby-jbuilder

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-jbuilder.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's [jbuilder](https://github.com/rails/jbuilder)
gem** — the `method_missing` DSL that builds JSON: `json.name "x"` yields
`{"name":"x"}`, nested `do … end` blocks yield nested objects, and `json.array!`
yields arrays. It renders the exact compact JSON string the gem's `target!`
returns for the same structure — matching insertion-ordered keys, the gem's
ActiveSupport JSON escaping, and Ruby's `json`-gem float formatting **byte-for-
byte** — **without any Ruby runtime**.

It is the Jbuilder backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime — a sibling
of [go-ruby-erb](https://github.com/go-ruby-erb/erb) (the ERB compiler),
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine),
and [go-ruby-yaml](https://github.com/go-ruby-yaml/yaml) (the Psych emitter).

> **How `method_missing` crosses the boundary.** The gem drives everything through
> `method_missing` on a `Jbuilder` receiver: `json.<name> value` becomes a key,
> `json.<name> do … end` a nested object. Go has no `method_missing`, so this
> library exposes the same behaviour as **explicit driver methods** — `Set`,
> `Block`, `Array`, `Child`, `Nil`, `Merge`, `Extract`, `KeyFormat`, `IgnoreNil` —
> that the host (rbgo) binds `json.<name>` onto. Assembling and rendering the JSON
> is fully deterministic and needs no interpreter, so it lives here as pure Go;
> evaluating the Ruby blocks and resolving Ruby objects/attributes stays the host's
> job, handed in as ordinary Go closures and resolved values.

## Features

Faithful port of Jbuilder's builder + JSON output, validated against the
`jbuilder` gem on every supported platform:

- **The full DSL surface** — key assignment (`json.name`), nested object blocks
  (`json.author do … end`), arrays (`json.array! collection do |x| … end` and the
  plain `json.array! collection`), incremental `json.child!`, `json.set!`,
  `json.merge!`, `json.extract!` / `json.call`, and `json.nil!` / `json.null!`.
- **Insertion-ordered objects** with last-write-wins on a repeated key, exactly as
  the gem's underlying `Hash` behaves; `merge!` appends literally (no de-dup),
  matching the gem.
- **`key_format!`** — `camelize: :lower` / `:upper`, `:dasherize`, and
  `:underscore`, reproducing `ActiveSupport::Inflector` (default acronym table),
  inherited by nested blocks.
- **`ignore_nil!`** — drop keys whose value is `nil`, inherited by nested blocks.
- **The gem's exact JSON bytes** — ActiveSupport's HTML-safe escaping (`<` `>` `&`
  as `<` `>` `&`, U+2028/U+2029 escaped), literal (unescaped)
  non-ASCII UTF-8, C0 controls as `\u00XX` with `\b \t \n \f \r` short forms, and
  numbers rendered like Ruby's `json` gem `Float#to_json`.

CGO-free, dependency-free, **100% test coverage**, `gofmt` + `go vet` clean, and
green across the six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le,
s390x) and three OSes.

## Install

```sh
go get github.com/go-ruby-jbuilder/jbuilder
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-jbuilder/jbuilder"
)

func main() {
	// json.name "David"; json.author do json.name "Ann"; json.id 7 end
	out := jbuilder.Encode(func(json *jbuilder.Jbuilder) {
		json.Set("name", "David")
		json.Block("author", func(json *jbuilder.Jbuilder) {
			json.Set("name", "Ann")
			json.Set("id", 7)
		})
	})
	fmt.Println(out)
	// {"name":"David","author":{"name":"Ann","id":7}}

	// json.array!([1,2,3]) { |x| json.v x }
	arr := jbuilder.Encode(func(json *jbuilder.Jbuilder) {
		json.Array([]any{1, 2, 3}, func(json *jbuilder.Jbuilder, x any) {
			json.Set("v", x)
		})
	})
	fmt.Println(arr) // [{"v":1},{"v":2},{"v":3}]
}
```

## API

The `method_missing` DSL is exposed as explicit driver methods; rbgo intercepts a
Ruby `json.<name>` call and dispatches to `Set` (a value) or `Block` (a passed
block), and maps the remaining `json.*!` methods one-to-one.

```go
type Jbuilder struct{ /* … */ }

func New() *Jbuilder
func Encode(fn func(*Jbuilder)) string           // Jbuilder.encode { |json| … }

func (j *Jbuilder) Set(key string, value any)            // json.<key> value / set!
func (j *Jbuilder) SetKey(key Symbol, value any)         // json.set! :sym, value
func (j *Jbuilder) Block(key string, fn func(*Jbuilder)) // json.<key> do … end
func (j *Jbuilder) Array(items []any, fn func(*Jbuilder, any)) // json.array! coll do |x| … end
func (j *Jbuilder) Child(fn func(*Jbuilder))             // json.child! { … }
func (j *Jbuilder) Nil()                                 // json.nil! / json.null!
func (j *Jbuilder) Merge(pairs []Pair)                   // json.merge!(hash)
func (j *Jbuilder) MergeArray(elems []any)               // json.merge!(array)
func (j *Jbuilder) Extract(pairs []Pair)                 // json.extract!(obj, …) / json.call
func (j *Jbuilder) KeyFormat(ops ...KeyOp)               // json.key_format! …
func (j *Jbuilder) IgnoreNil(on ...bool)                 // json.ignore_nil!
func (j *Jbuilder) Encode() string                       // json.target!
func (j *Jbuilder) TargetJSON() string                   // alias of Encode

type Symbol string
type Pair struct { Key string; Value any }
type KeyOp func(string) string
func Camelize(upper bool) KeyOp   // camelize: :upper / :lower
func Dasherize() KeyOp            // :dasherize
func Underscore() KeyOp           // :underscore
```

The value model a host maps to and from is deliberately small: `nil`, `bool`, the
integer kinds (including `*big.Int`), `float32`/`float64`, `string`, `Symbol`,
`[]any` (arrays), a nested `*Jbuilder`, and `map[string]any` (emitted sorted).
Blocks are ordinary Go closures the host supplies.

## Float parity

Numbers are rendered like the gem — Ruby's `json` gem `Float#to_json` — always
with a decimal point (`1.0`, `100.0`), fixed down to `0.000000001` and switching
to exponent form (`1e+15`, `1e-10`) at the same thresholds, with no forced `.0` in
the mantissa. Parity is byte-exact for **every value expressible in ≤15
significant digits** — every number a JSON payload realistically carries. The only
divergence is on the ~16–17-digit values that sit exactly on a floating-point ULP
tie, where Go's shortest-round-trip formatter (Ryū) and Ruby's dtoa pick a
different final digit (and where even Ruby's own `Float#to_s` and `to_json`
disagree); those extremes are outside the guarantee.

## Tests & coverage

The suite pairs deterministic, ruby-free tests (which alone hold coverage at
100%, so the qemu cross-arch and Windows lanes pass the gate) with a **differential
gem oracle**: each structure is built with both the Go driver and the `jbuilder`
gem and their `target!` compared byte-for-byte across the whole method surface,
plus a large random float corpus fed to Ruby by exact bit pattern (so no decimal
reparse can mask a formatting bug). The oracle skips itself where `ruby` / the
gem is absent, or on Ruby < 4.0 (the version the parity is gated on).

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-jbuilder/jbuilder authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
