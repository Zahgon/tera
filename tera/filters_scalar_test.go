//go:build !tera_unicode

package tera

import "testing"

func TestTruncateSplitsOnCharBoundary(t *testing.T) {
	for _, tc := range []struct {
		input  string
		length int
		want   string
	}{{"😀test", 1, "😀…"}, {"日本語hello", 3, "日本語…"}} {
		checkFilter(t, "truncate", tc.input, map[string]any{"length": tc.length}, tc.want)
	}
}
