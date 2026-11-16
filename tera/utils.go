package tera

import (
	"io"
	"strings"
)

// EscapeFn writes escaped text to an output writer.
type EscapeFn func(input string, w io.Writer) error

// EscapeHTML escapes the five HTML significant characters.
func EscapeHTML(input string, w io.Writer) error {
	return writeAll(w, []byte(strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#39;").Replace(input)))
}
