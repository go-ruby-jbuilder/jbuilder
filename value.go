// Copyright (c) the go-ruby-jbuilder/jbuilder authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jbuilder

import "fmt"

// defaultString renders a value outside the model as its Ruby to_s-ish text, used
// only by the encoder's fallback (an unknown Go type reaching JSON output). It
// keeps the encoder total rather than panicking, mirroring jbuilder handing an
// arbitrary object to to_json (which stringifies via to_s).
func defaultString(v any) string {
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintf("%v", v)
}
