// Copyright (c) the go-ruby-jbuilder/jbuilder authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jbuilder

import "strings"

// KeyOp is a single key_format! transform (camelize/dasherize/underscore). The
// gem's json.key_format! takes a chain of these; they compose left to right.
type KeyOp func(string) string

// Camelize returns the camelize KeyOp. upper selects UpperCamelCase
// (json.key_format! camelize: :upper); false selects lowerCamelCase
// (camelize: :lower), matching ActiveSupport::Inflector.camelize with the default
// (empty) acronym table.
func Camelize(upper bool) KeyOp {
	return func(s string) string { return camelize(s, upper) }
}

// Dasherize returns the dasherize KeyOp (json.key_format! :dasherize):
// ActiveSupport's String#dasherize, which simply swaps `_` for `-`.
func Dasherize() KeyOp {
	return func(s string) string { return strings.ReplaceAll(s, "_", "-") }
}

// Underscore returns the underscore KeyOp (json.key_format! :underscore):
// ActiveSupport::Inflector.underscore with the default acronym table.
func Underscore() KeyOp {
	return func(s string) string { return underscore(s) }
}

// keyFormatter holds the active chain of key transforms.
type keyFormatter struct {
	ops []KeyOp
}

// format applies every op in order to key.
func (f *keyFormatter) format(key string) string {
	for _, op := range f.ops {
		key = op(key)
	}
	return key
}

// isLowerAlnum reports whether b is an ASCII lowercase letter or digit — the
// characters ActiveSupport's camelize leading regex `^[a-z\d]*` consumes.
func isLowerAlnum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// camelize reproduces ActiveSupport::Inflector.camelize (empty acronym table).
// For :upper it capitalises the leading `[a-z0-9]*` run then, for each `_`- or
// `/`-delimited segment, capitalises that segment; `/` becomes `::`. For :lower
// it lowercases the first character of the :upper result unless the leading run
// was empty.
func camelize(s string, upper bool) string {
	var b strings.Builder
	i := 0
	// Leading [a-z0-9]* run.
	start := i
	for i < len(s) && isLowerAlnum(s[i]) {
		i++
	}
	lead := s[start:i]
	if upper {
		b.WriteString(capitalizeSegment(lead))
	} else {
		// camelize(:lower) lowercases only the first char of the capitalised
		// leading run; an empty leading run leaves the rest as :upper produces.
		cap := capitalizeSegment(lead)
		if cap != "" {
			b.WriteByte(lowerByte(cap[0]))
			b.WriteString(cap[1:])
		}
	}
	// Remaining `(?:_|/)([a-z0-9]*)` segments (case-insensitive on the segment).
	for i < len(s) {
		c := s[i]
		if c == '_' || c == '/' {
			i++
			segStart := i
			for i < len(s) && isAlnum(s[i]) {
				i++
			}
			seg := s[segStart:i]
			if c == '/' {
				b.WriteString("::")
			}
			b.WriteString(capitalizeSegment(seg))
		} else {
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// isAlnum reports whether b is an ASCII letter or digit (the `[a-z\d]` segment
// class under the /i flag ActiveSupport uses for the trailing segments).
func isAlnum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// capitalizeSegment mirrors Ruby String#capitalize on an already-word segment:
// upcase the first character, downcase the rest.
func capitalizeSegment(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.WriteByte(upperByte(s[0]))
	for i := 1; i < len(s); i++ {
		b.WriteByte(lowerByte(s[i]))
	}
	return b.String()
}

// underscore reproduces ActiveSupport::Inflector.underscore (empty acronym
// table): `::`→`/`, insert `_` at CamelCase boundaries, `-`→`_`, downcase.
func underscore(s string) string {
	s = strings.ReplaceAll(s, "::", "/")
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case isUpper(c):
			// Boundary before an uppercase that follows a lowercase/digit
			// (fooBar → foo_bar) or that starts a new word after an acronym
			// run (HTTPResponse → http_response: the P before Response).
			if i > 0 && needUnderscoreBefore(s, i) {
				b.WriteByte('_')
			}
			b.WriteByte(lowerByte(c))
		case c == '-':
			b.WriteByte('_')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// needUnderscoreBefore reports whether ActiveSupport inserts a `_` immediately
// before the uppercase letter at index i. It fires when the previous char is a
// lowercase letter or digit (aB), or when this uppercase begins a word after a
// run of uppercases followed by more letters (…TR followed by lowercase, i.e.
// the acronym→Word split HTTPResponse → http_response).
func needUnderscoreBefore(s string, i int) bool {
	prev := s[i-1]
	if isLower(prev) || (prev >= '0' && prev <= '9') {
		return true
	}
	if isUpper(prev) && i+1 < len(s) && isLower(s[i+1]) {
		return true
	}
	return false
}

// isUpper reports whether b is an ASCII uppercase letter.
func isUpper(b byte) bool { return b >= 'A' && b <= 'Z' }

// isLower reports whether b is an ASCII lowercase letter.
func isLower(b byte) bool { return b >= 'a' && b <= 'z' }

// upperByte upcases an ASCII letter, leaving other bytes untouched.
func upperByte(b byte) byte {
	if isLower(b) {
		return b - ('a' - 'A')
	}
	return b
}

// lowerByte downcases an ASCII letter, leaving other bytes untouched.
func lowerByte(b byte) byte {
	if isUpper(b) {
		return b + ('a' - 'A')
	}
	return b
}
