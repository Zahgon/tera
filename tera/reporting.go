package tera

import (
	"fmt"
	"strings"
)

func generateReport(e ReportError) string {
	loc := func(src string, sp Span) (string, string, int) {
		ls := strings.Split(src, "\n")
		n := sp.StartLine
		if n < 1 {
			n = 1
		}
		line := ""
		if n <= len(ls) {
			line = ls[n-1]
		}
		var b strings.Builder
		for i, c := range []rune(line) {
			if i >= sp.StartCol {
				break
			}
			if c == '\t' {
				b.WriteRune(c)
			} else {
				b.WriteByte(' ')
			}
		}
		w := sp.EndCol - sp.StartCol
		if w < 1 {
			w = 1
		}
		b.WriteString(strings.Repeat("^", w))
		return line, b.String(), n
	}
	line, u, n := loc(e.Source, e.Span)
	p := strings.Repeat(" ", len(fmt.Sprint(n)))
	s := fmt.Sprintf("error: %s\n%s--> %s:%d:%d\n%s |\n%d | %s\n%s | %s", e.Message, p, e.Filename, n, e.Span.StartCol+1, p, n, line, p, u)
	for _, v := range e.notes {
		l, u, n := loc(v.source, v.span)
		p := strings.Repeat(" ", len(fmt.Sprint(n)))
		s += fmt.Sprintf("\n\nnote: %s %s:%d:%d\n%s |\n%d | %s\n%s | %s", v.label, v.filename, n, v.span.StartCol+1, p, n, l, p, u)
	}
	return s
}
