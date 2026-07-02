// Copyright (c) the go-ruby-jbuilder/jbuilder authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package jbuilder is a pure-Go (CGO-free) reimplementation of Ruby's jbuilder
// gem — the DSL that builds JSON with a method_missing builder: `json.name "x"`
// yields {"name":"x"}, nested blocks yield nested objects, and json.array!
// yields arrays. The gem drives everything through method_missing on a Jbuilder
// receiver; Go has no method_missing, so this package exposes the same behaviour
// as explicit driver methods (Set, Block, Array, Extract, Merge, Nil, Child, …)
// that the go-embedded-ruby host binds json.<name> onto. The result — obtained
// with (*Jbuilder).Encode / TargetJSON — is the exact compact JSON string the
// gem's target! returns for the same structure.
//
// The value model is deliberately small so a host can map its own object graph
// to and from it: nil, bool, integers (including *big.Int), float32/float64,
// string, Symbol, []any (arrays), *Jbuilder / *object (nested objects), and Go
// maps. Blocks are ordinary Go closures the host supplies.
package jbuilder

import "strings"

// Symbol is a Ruby Symbol value. As a JSON value it renders as a string; as a
// key it is the symbol name. Hosts pass Symbol where Ruby would use a :symbol so
// the distinction survives the boundary even though JSON collapses it to text.
type Symbol string

// object is an insertion-ordered set of key/value pairs — the Hash a Jbuilder
// accumulates. Keys are the already-formatted strings (key_format! has run);
// setting an existing key overwrites its value in place, preserving order, which
// is exactly how the gem's nested Hash behaves.
type object struct {
	keys []string
	vals []any
	idx  map[string]int
}

// newObject returns an empty ordered object.
func newObject() *object {
	return &object{idx: map[string]int{}}
}

// set stores val under key, overwriting in place if key already exists (last
// write wins, order preserved) and appending otherwise.
func (o *object) set(key string, val any) {
	if i, ok := o.idx[key]; ok {
		o.vals[i] = val
		return
	}
	o.idx[key] = len(o.keys)
	o.keys = append(o.keys, key)
	o.vals = append(o.vals, val)
}

// targetKind tracks whether a builder's target has become an object, an array, a
// bare value (via Nil), or is still undecided.
type targetKind int

const (
	kindUndecided targetKind = iota
	kindObject
	kindArray
	kindValue
)

// Jbuilder is the JSON builder. It starts undecided and becomes an object the
// first time a key is set, an array when Array/Child is used, or a bare value
// when Nil clears it — mirroring how the gem's @attributes morphs. An empty
// builder renders as {} (jbuilder's Jbuilder.new.target! default).
type Jbuilder struct {
	kind      targetKind
	obj       *object
	arr       []any
	val       any
	keyFormat *keyFormatter
	ignoreNil bool
}

// New returns a fresh, empty Jbuilder.
func New() *Jbuilder {
	return &Jbuilder{}
}

// ensureObject transitions the builder to object mode, allocating the ordered
// backing store on first use.
func (j *Jbuilder) ensureObject() {
	if j.kind != kindObject {
		j.kind = kindObject
		j.obj = newObject()
	}
}

// formatKey applies the active key_format! transform (if any) to a raw key.
func (j *Jbuilder) formatKey(key string) string {
	if j.keyFormat == nil {
		return key
	}
	return j.keyFormat.format(key)
}

// Set assigns value to key (json.<key> value / json.set!(key, value)). A nil
// value under an active IgnoreNil is dropped, matching json.ignore_nil!. Setting
// a key that already exists overwrites it in place (last write wins).
func (j *Jbuilder) Set(key string, value any) {
	if value == nil && j.ignoreNil {
		return
	}
	j.ensureObject()
	j.obj.set(j.formatKey(key), value)
}

// SetKey mirrors Set for a Symbol key (json.set! :sym, value), stringifying the
// symbol name as Ruby does when it becomes a JSON key.
func (j *Jbuilder) SetKey(key Symbol, value any) { j.Set(string(key), value) }

// Block builds a nested object under key from fn (json.<key> do … end). fn
// receives a child builder that inherits the parent's key_format! and
// ignore_nil! settings; whatever it produces becomes the value at key. A block
// that sets nothing yields {}.
func (j *Jbuilder) Block(key string, fn func(*Jbuilder)) {
	child := j.child()
	fn(child)
	j.ensureObject()
	j.obj.set(j.formatKey(key), child.value())
}

// child returns a nested builder sharing this one's formatting context.
func (j *Jbuilder) child() *Jbuilder {
	return &Jbuilder{keyFormat: j.keyFormat, ignoreNil: j.ignoreNil}
}

// Array turns the whole builder into a JSON array by mapping fn over items
// (json.array! collection do |x| … end). For each element fn is called with a
// fresh child builder and the element; the child's rendered value becomes that
// array entry. Passing a nil fn emits the elements verbatim (json.array!
// collection), so scalar collections pass straight through.
func (j *Jbuilder) Array(items []any, fn func(*Jbuilder, any)) {
	j.kind = kindArray
	j.arr = make([]any, 0, len(items))
	for _, it := range items {
		if fn == nil {
			j.arr = append(j.arr, it)
			continue
		}
		child := j.child()
		fn(child, it)
		j.arr = append(j.arr, child.value())
	}
}

// Child appends one element built by fn to the builder's array
// (json.child! { … }), turning the builder into an array on first use. It is the
// incremental form of Array.
func (j *Jbuilder) Child(fn func(*Jbuilder)) {
	if j.kind != kindArray {
		j.kind = kindArray
		j.arr = nil
	}
	child := j.child()
	fn(child)
	j.arr = append(j.arr, child.value())
}

// Nil sets the whole target to null (json.nil! / json.null!), discarding any
// object or array built so far, exactly as the gem does.
func (j *Jbuilder) Nil() {
	j.kind = kindValue
	j.val = nil
}

// Merge folds pairs into the current object (json.merge!(hash)). Like the gem's
// merge!, it does not de-duplicate against existing keys via the ordered index —
// it appends the pairs in the given order — so a host reproducing jbuilder's
// literal merge! passes an ordered []Pair. Merging onto a fresh builder makes it
// an object.
func (j *Jbuilder) Merge(pairs []Pair) {
	j.ensureObject()
	for _, p := range pairs {
		j.obj.keys = append(j.obj.keys, j.formatKey(p.Key))
		j.obj.vals = append(j.obj.vals, p.Value)
	}
}

// MergeArray concatenates elems onto the builder's array (json.merge! with an
// array argument), turning the builder into an array if it was undecided.
func (j *Jbuilder) MergeArray(elems []any) {
	if j.kind != kindArray {
		j.kind = kindArray
		j.arr = nil
	}
	j.arr = append(j.arr, elems...)
}

// Pair is one ordered key/value entry, used by Extract, Merge and Call where the
// host has already resolved Ruby symbols/attributes to concrete values but wants
// to preserve their order.
type Pair struct {
	Key   string
	Value any
}

// Extract copies the named attributes from a host-resolved attribute list into
// the object (json.extract!(obj, :a, :b) / json.call(obj, :a, :b)). The host
// resolves obj.a / obj[:a] to values and passes them as ordered pairs; Extract
// sets each under its key (with key_format! applied). A missing attribute the
// host resolves to nil is written as null unless IgnoreNil is active.
func (j *Jbuilder) Extract(pairs []Pair) {
	for _, p := range pairs {
		j.Set(p.Key, p.Value)
	}
}

// KeyFormat sets the active key transform (json.key_format! camelize: :lower,
// etc.). Passing no ops clears formatting (json.key_format! with no arguments).
// The transform applies to keys set afterwards and is inherited by child blocks.
func (j *Jbuilder) KeyFormat(ops ...KeyOp) {
	if len(ops) == 0 {
		j.keyFormat = nil
		return
	}
	j.keyFormat = &keyFormatter{ops: ops}
}

// IgnoreNil enables (or, given false, disables) dropping keys whose value is nil
// (json.ignore_nil!). The setting is inherited by child blocks created after it.
func (j *Jbuilder) IgnoreNil(on ...bool) {
	if len(on) == 0 {
		j.ignoreNil = true
		return
	}
	j.ignoreNil = on[0]
}

// value returns the builder's current target in value-model form: the ordered
// object, the array, or the bare value. An undecided builder is treated as an
// empty object, matching Jbuilder.new.target! == "{}".
func (j *Jbuilder) value() any {
	switch j.kind {
	case kindArray:
		if j.arr == nil {
			return []any{}
		}
		return j.arr
	case kindValue:
		return j.val
	case kindObject:
		return j.obj
	default:
		return newObject()
	}
}

// Encode renders the builder to its compact JSON string — the exact bytes the
// gem's json.target! returns.
func (j *Jbuilder) Encode() string {
	var b strings.Builder
	encodeValue(&b, j.value())
	return b.String()
}

// TargetJSON is an alias for Encode named after the gem's target! for hosts that
// bind the Ruby method name directly.
func (j *Jbuilder) TargetJSON() string { return j.Encode() }

// Encode builds a JSON string by running fn against a fresh builder — the
// package-level convenience matching Jbuilder.encode { |json| … } and the common
// `render json: Jbuilder.encode { … }` form.
func Encode(fn func(*Jbuilder)) string {
	j := New()
	fn(j)
	return j.Encode()
}
