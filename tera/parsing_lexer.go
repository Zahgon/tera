package tera

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Keats/tera/internal/rustrt"
)

type token struct {
	k, s      string
	v         Value
	sp        Span
	ws, wsEnd bool
	err       error
}

func (t token) display() string {
	switch t.k {
	case "content":
		return "content"
	case "vs":
		return "`{{`"
	case "ve":
		return "`}}`"
	case "ts":
		return "`{%`"
	case "te":
		return "`%}`"
	case "id":
		return "identifier"
	case "str":
		return "string"
	case "int":
		return "integer"
	case "float":
		return "float"
	case "bool":
		return "bool"
	}
	return "`" + t.k + "`"
}
func (t token) debug() string {
	switch t.k {
	case "id":
		return "IDENT(" + t.s + ")"
	case "str":
		return "STRING(" + rustrt.DebugString(t.s) + ")"
	case "int":
		return "INTEGER(" + t.v.String() + ")"
	case "float":
		return "FLOAT(" + t.v.String() + ")"
	case "bool":
		return "BOOL(" + t.v.String() + ")"
	case "ve":
		return fmt.Sprintf("VARIABLE_END(%t)", t.ws)
	case "te":
		return fmt.Sprintf("TAG_END(%t)", t.ws)
	}
	m := map[string]string{"+": "PLUS", "-": "MINUS", "*": "MUL", "/": "DIV", "//": "FLOORDIV", "**": "POWER", "%": "MOD", "!": "BANG", ".": "DOT", "?.": "QUESTION_MARK_DOT", "?[": "QUESTION_MARK_LEFT_BRACKET", ",": "COMMA", ":": "COLON", "~": "TILDE", "=": "ASSIGN", "|": "PIPE", "==": "EQ", "!=": "NE", ">": "GT", ">=": "GTE", "<": "LT", "</": "CLOSING_TAG_START", "<=": "LTE", "[": "LEFT_BRACKET", "]": "RIGHT_BRACKET", "(": "LEFT_PAREN", ")": "RIGHT_PAREN", "{": "LEFT_BRACE", "}": "RIGHT_BRACE", "...": "SPREAD", "@": "AT"}
	return m[t.k]
}
func asciiSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f'
}
func asciiAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func digit(c byte) bool      { return c >= '0' && c <= '9' }
func lex(source string, d Delimiters) []token {
	out := []token{}
	pos, line, col, state := 0, 1, 0, 0
	spanStart := func() Span {
		return Span{StartLine: line, EndLine: line, StartCol: col, EndCol: col, Start: pos, End: pos}
	}
	advance := func(n int) {
		s := source[pos : pos+n]
		for _, r := range s {
			if r == '\n' {
				line++
				col = 0
			} else {
				col++
			}
		}
		pos += n
	}
	emit := func(t token) { t.sp.EndLine = line; t.sp.EndCol = col; t.sp.End = pos; out = append(out, t) }
	fail := func(msg string, sp Span) {
		sp.EndLine = line
		sp.EndCol = col
		sp.End = pos
		out = append(out, token{k: "err", sp: sp, err: syntaxError(msg, sp)})
	}
	skipTag := func(s, name string) (int, bool, bool) {
		orig := s
		if strings.HasPrefix(s, "-") {
			s = s[1:]
		}
		for len(s) > 0 && asciiSpace(s[0]) {
			s = s[1:]
		}
		if !strings.HasPrefix(s, name) {
			return 0, false, false
		}
		s = s[len(name):]
		for len(s) > 0 && asciiSpace(s[0]) {
			s = s[1:]
		}
		ws := strings.HasPrefix(s, "-")
		if ws {
			s = s[1:]
		}
		if !strings.HasPrefix(s, d.BlockEnd) {
			return 0, false, false
		}
		return len(orig) - len(s) + 2, ws, true
	}
	for pos < len(source) {
		sp := spanStart()
		rest := source[pos:]
		if state == 0 {
			k := ""
			end := ""
			switch {
			case strings.HasPrefix(rest, d.VariableStart):
				k = "vs"
				end = d.VariableEnd
			case strings.HasPrefix(rest, d.BlockStart):
				k = "ts"
				end = d.BlockEnd
			case strings.HasPrefix(rest, d.CommentStart):
				k = "comment"
				end = d.CommentEnd
			}
			if k != "" {
				ws := len(rest) > 2 && rest[2] == '-'
				n := 2
				if ws {
					n++
				}
				advance(n)
				if k == "comment" {
					i := strings.Index(source[pos:], end)
					if i < 0 {
						fail("Closing comment tag `"+end+"` not found", sp)
						break
					}
					wsEnd := i > 0 && source[pos+i-1] == '-'
					advance(i + 2)
					emit(token{k: k, sp: sp, ws: ws, wsEnd: wsEnd})
					continue
				}
				if k == "ts" {
					if n, trimStart, ok := skipTag(source[pos:], "raw"); ok {
						start := pos + n
						scan := start
						found := false
						for scan < len(source) {
							i := strings.Index(source[scan:], d.BlockStart)
							if i < 0 {
								break
							}
							at := scan + i
							after := at + 2
							trimEnd := after < len(source) && source[after] == '-'
							if n, wsEnd, ok := skipTag(source[after:], "endraw"); ok {
								text := source[start:at]
								if trimStart {
									text = rustrt.TrimStart(text)
								}
								if trimEnd {
									text = rustrt.TrimEnd(text)
								}
								advance(after + n - pos)
								emit(token{k: "raw", s: text, sp: sp, ws: ws, wsEnd: wsEnd})
								found = true
								break
							}
							scan = after
						}
						if !found {
							fail("unexpected end of raw block", sp)
							break
						}
						continue
					}
				}
				state = 1
				if k == "ts" {
					state = 2
				}
				emit(token{k: k, sp: sp, ws: ws})
				continue
			}
			n := len(rest)
			for _, m := range []string{d.VariableStart, d.BlockStart, d.CommentStart} {
				if i := strings.Index(rest, m); i >= 0 && i < n {
					n = i
				}
			}
			advance(n)
			emit(token{k: "content", s: rest[:n], sp: sp})
			continue
		}
		if asciiSpace(rest[0]) {
			n := 0
			for n < len(rest) && asciiSpace(rest[n]) {
				n++
			}
			advance(n)
			continue
		}
		end, k := d.VariableEnd, "ve"
		if state == 2 {
			end, k = d.BlockEnd, "te"
		}
		ws := strings.HasPrefix(rest, "-"+end)
		if ws || strings.HasPrefix(rest, end) {
			n := 2
			if ws {
				n++
			}
			advance(n)
			emit(token{k: k, sp: sp, ws: ws})
			state = 0
			continue
		}
		matched := false
		for _, op := range []string{"...", "//", "**", "==", "!=", ">=", "<=", "</", "?.", "?["} {
			if strings.HasPrefix(rest, op) {
				advance(len(op))
				emit(token{k: op, sp: sp})
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		c := rest[0]
		if strings.ContainsRune("+-*/%!.,:~|=><()[]{}@", rune(c)) {
			advance(1)
			emit(token{k: string(c), sp: sp})
			continue
		}
		if c == '\'' || c == '"' || c == '`' {
			n := 1
			escaped := false
			for n < len(rest) {
				if escaped {
					escaped = false
					n++
					continue
				}
				if rest[n] == '\\' {
					escaped = true
					n++
					continue
				}
				if rest[n] == c {
					break
				}
				n++
			}
			if n >= len(rest) {
				fail(fmt.Sprintf("String opened with `%c` is missing its closing `%c`", c, c), sp)
				break
			}
			s := rest[1:n]
			advance(n + 1)
			var b strings.Builder
			valid := true
			for i := 0; i < len(s); {
				r, z := utf8.DecodeRuneInString(s[i:])
				i += z
				if r == '\\' {
					if i >= len(s) {
						valid = false
						break
					}
					r, z = utf8.DecodeRuneInString(s[i:])
					i += z
					switch r {
					case '\'', '"', '/', '\\':
					case 'n':
						r = '\n'
					case 't':
						r = '\t'
					case 'r':
						r = '\r'
					default:
						valid = false
					}
				}
				b.WriteRune(r)
			}
			if !valid {
				fail("unexpected escape character", sp)
				break
			}
			emit(token{k: "str", s: b.String(), v: NormalString(b.String()), sp: sp})
			continue
		}
		if digit(c) {
			n := 0
			dot := false
			for n < len(rest) {
				if digit(rest[n]) {
					n++
					continue
				}
				if !dot && rest[n] == '.' {
					dot = true
					n++
					continue
				}
				break
			}
			s := rest[:n]
			advance(n)
			if dot {
				f, e := strconv.ParseFloat(s, 64)
				if e != nil {
					fail("Invalid float", sp)
					break
				}
				emit(token{k: "float", v: F64(f), sp: sp})
			} else {
				i, e := strconv.ParseInt(s, 10, 64)
				if e != nil {
					fail("Invalid Integer", sp)
					break
				}
				emit(token{k: "int", v: I64(i), sp: sp})
			}
			continue
		}
		if asciiAlpha(c) || c == '_' {
			n := 1
			for n < len(rest) && (asciiAlpha(rest[n]) || digit(rest[n]) || rest[n] == '_') {
				n++
			}
			s := rest[:n]
			advance(n)
			t := token{k: "id", s: s, sp: sp}
			if s == "true" || s == "True" || s == "false" || s == "False" {
				t.k = "bool"
				t.v = Bool(s == "true" || s == "True")
			}
			emit(t)
			continue
		}
		fail("Unexpected character", sp)
		break
	}
	filtered := []token{}
	remove := false
	for i, t := range out {
		switch t.k {
		case "content", "raw":
			if remove {
				t.s = rustrt.TrimStart(t.s)
				remove = false
			}
			if t.k == "raw" && t.wsEnd {
				remove = true
			}
			if i+1 < len(out) && out[i+1].ws && (out[i+1].k == "vs" || out[i+1].k == "ts" || out[i+1].k == "comment" || out[i+1].k == "raw") {
				t.s = rustrt.TrimEnd(t.s)
			}
			t.k = "content"
		case "comment":
			if t.wsEnd {
				remove = true
			}
			t.k = "content"
			t.s = ""
		case "ve", "te":
			if t.ws {
				remove = true
			}
		default:
			remove = false
		}
		filtered = append(filtered, t)
	}
	return filtered
}
