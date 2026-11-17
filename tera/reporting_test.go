// Ports the unit test of tera/src/reporting.rs: can_get_line_starts.

package tera

import (
	"fmt"
	"strings"
	"testing"
)

func TestCanGetLineStarts(t *testing.T) {
	// Go's reporter splits source directly rather than exposing a line-start table.
	// Check the same CRLF/empty-line boundaries through its diagnostic output.
	source := "foo\nbar\r\n\nbaz"
	for i, start := range []int{0, 4, 9, 10} {
		line := strings.Split(source[start:], "\n")[0]
		sp := Span{StartLine: i + 1, EndLine: i + 1, Start: start, End: start + 1, EndCol: 1}
		got := generateReport(ReportError{Message: "test", Filename: "source", Source: source, Span: sp})
		want := fmt.Sprintf("%d | %s\n", i+1, line)
		if !strings.Contains(got, want) {
			t.Errorf("expected line at byte %d: %q in %q", start, want, got)
		}
	}
}
