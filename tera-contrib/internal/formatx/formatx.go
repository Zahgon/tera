// Package formatx implements formatx's runtime formatting for Tera scalar values.
package formatx

import (
	"fmt"
	"github.com/Keats/tera/internal/rustrt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type spec struct {
	fill                  rune
	align                 byte
	plus, alt, zero, star bool
	width, precision      int
	kind                  string
}
type segment struct {
	literal     string
	placeholder bool
	arg         string
	implicit    bool
	spec        spec
}
type value struct {
	kind byte
	s    string
	n    *big.Int
	f    float64
	b    bool
}

// FormatStr formats a string with the spec supplied to Tera's format filter.
func FormatStr(spec, s string) (string, error) { return format(spec, value{kind: 's', s: s}) }

// FormatInt formats an i128 or u128 integer with a runtime format spec.
func FormatInt(spec string, n *big.Int) (string, error) {
	if n == nil {
		n = new(big.Int)
	}
	return format(spec, value{kind: 'i', n: n})
}

// FormatFloat formats an f64 with a runtime format spec.
func FormatFloat(spec string, f float64) (string, error) { return format(spec, value{kind: 'f', f: f}) }

// FormatBool formats a boolean with a runtime format spec.
func FormatBool(spec string, b bool) (string, error) { return format(spec, value{kind: 'b', b: b}) }
func parseError(pos int, msg string) error           { return fmt.Errorf("parse error at byte %d: %s", pos, msg) }
func digit(b byte) bool                              { return b >= '0' && b <= '9' }
func ident(b byte) bool                              { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_' }
func alignment(r rune) bool                          { return r == '<' || r == '>' || r == '^' }
func parse(source string) ([]segment, error) {
	var segs []segment
	for pos := 0; pos < len(source); {
		switch source[pos] {
		case '{':
			if pos+1 < len(source) && source[pos+1] == '{' {
				segs = append(segs, segment{literal: "{"})
				pos += 2
				continue
			}
			start := pos
			pos++
			if pos == len(source) {
				return nil, parseError(pos, "unexpected end of format string")
			}
			seg := segment{placeholder: true, implicit: true, spec: spec{fill: ' ', width: -1, precision: -1}}
			if source[pos] != '}' && source[pos] != ':' {
				argStart := pos
				if digit(source[pos]) {
					for pos < len(source) && digit(source[pos]) {
						pos++
					}
				} else if ident(source[pos]) {
					for pos < len(source) && (ident(source[pos]) || digit(source[pos])) {
						pos++
					}
				}
				if pos == argStart || pos >= len(source) || source[pos] != '}' && source[pos] != ':' {
					return nil, parseError(argStart, "invalid placeholder argument")
				}
				seg.implicit = false
				seg.arg = source[argStart:pos]
				if source[argStart] >= '0' && source[argStart] <= '9' {
					if index, e := strconv.ParseUint(seg.arg, 10, 64); e == nil {
						seg.arg = strconv.FormatUint(index, 10)
					}
				}
			}
			if pos < len(source) && source[pos] == ':' {
				pos++
				var err error
				seg.spec, err = parseSpec(source, &pos)
				if err != nil {
					return nil, err
				}
			}
			if pos >= len(source) || source[pos] != '}' {
				return nil, parseError(start, "unmatched `{`")
			}
			pos++
			segs = append(segs, seg)
		case '}':
			if pos+1 < len(source) && source[pos+1] == '}' {
				segs = append(segs, segment{literal: "}"})
				pos += 2
			} else {
				return nil, parseError(pos, "unmatched `}`")
			}
		default:
			start := pos
			for pos < len(source) && source[pos] != '{' && source[pos] != '}' {
				pos++
			}
			segs = append(segs, segment{literal: source[start:pos]})
		}
	}
	return segs, nil
}
func parseSpec(source string, pos *int) (spec, error) {
	s := spec{fill: ' ', width: -1, precision: -1}
	ended := func() bool { return *pos >= len(source) || source[*pos] == '}' }
	if ended() {
		return s, nil
	}
	first, n := utf8.DecodeRuneInString(source[*pos:])
	second, m := utf8.DecodeRuneInString(source[*pos+n:])
	if m > 0 && alignment(second) {
		s.fill = first
		s.align = byte(second)
		*pos += n + m
	} else if alignment(first) {
		s.align = byte(first)
		*pos += n
	}
	if ended() {
		return s, nil
	}
	if source[*pos] == '+' || source[*pos] == '-' {
		s.plus = source[*pos] == '+'
		*pos++
	}
	if ended() {
		return s, nil
	}
	if source[*pos] == '#' {
		s.alt = true
		*pos++
	}
	if ended() {
		return s, nil
	}
	if source[*pos] == '0' && *pos+1 < len(source) {
		c := source[*pos+1]
		if digit(c) || c == '}' || c == '.' || strings.ContainsRune("?oxXbeEp", rune(c)) {
			s.zero = true
			*pos++
		}
	}
	if ended() {
		return s, nil
	}
	count := func() (int, error) {
		start := *pos
		for *pos < len(source) && digit(source[*pos]) {
			*pos++
		}
		if start == *pos {
			return -1, nil
		}
		n, e := strconv.ParseUint(source[start:*pos], 10, 64)
		if e != nil || n > uint64(^uint(0)>>1) {
			return -1, parseError(start, "invalid count: `"+source[start:*pos]+"`")
		}
		return int(n), nil
	}
	var err error
	s.width, err = count()
	if err != nil {
		return s, err
	}
	if ended() {
		return s, nil
	}
	if source[*pos] == '.' {
		*pos++
		if ended() {
			return s, parseError(*pos-1, "expected precision value after `.`")
		}
		if source[*pos] == '*' {
			s.star = true
			*pos++
		} else {
			s.precision, err = count()
			if err != nil {
				return s, err
			}
			if s.precision < 0 {
				return s, parseError(*pos-1, "expected precision value after `.`")
			}
		}
	}
	if ended() {
		return s, nil
	}
	start := *pos
	c := source[*pos]
	if !strings.ContainsRune("?oxXbeEp", rune(c)) {
		return s, parseError(start, fmt.Sprintf("unknown format type: `%c`", rune(c)))
	}
	*pos++
	s.kind = string(c)
	if (c == 'x' || c == 'X') && *pos < len(source) && source[*pos] == '?' {
		*pos++
		s.kind += "?"
	}
	return s, nil
}
func format(sp string, v value) (string, error) {
	segs, err := parse("{:" + sp + "}")
	if err != nil {
		return "", err
	}
	var out strings.Builder
	implicit := 0
	for _, seg := range segs {
		if !seg.placeholder {
			out.WriteString(seg.literal)
			continue
		}
		s := seg.spec
		if s.star {
			text := v.raw(spec{width: -1, precision: -1})
			if implicit > 0 {
				return "", parseError(0, "not enough arguments for `.*` precision")
			}
			implicit++
			if _, e := strconv.ParseUint(strings.TrimPrefix(text, "+"), 10, 64); e != nil || strings.HasPrefix(text, "-") {
				return "", parseError(0, "`.*` precision argument `"+text+"` is not a valid usize")
			}
		}
		name := seg.arg
		if seg.implicit {
			name = strconv.Itoa(implicit)
			implicit++
		}
		if name != "0" {
			return "", fmt.Errorf("missing argument: `%s`", name)
		}
		trait := map[string]string{"o": "Octal", "x": "LowerHex", "X": "UpperHex", "b": "Binary", "e": "LowerExp", "E": "UpperExp", "p": "Pointer"}[s.kind]
		if trait != "" {
			return "", fmt.Errorf("unsupported format trait: `%s`", trait)
		}
		raw := v.raw(s)
		width := s.width
		if width < 0 {
			out.WriteString(raw)
			continue
		}
		if (s.kind == "x?" || s.kind == "X?") && s.align == 0 {
			out.WriteString(raw)
			continue
		}
		pad := width - utf8.RuneCountInString(raw)
		if pad <= 0 {
			out.WriteString(raw)
			continue
		}
		align, fill := s.align, s.fill
		if align == 0 {
			align = '<'
			if v.kind == 'i' || v.kind == 'f' {
				align = '>'
				if s.zero {
					fill = '0'
					if strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") {
						out.WriteByte(raw[0])
						raw = raw[1:]
					}
				}
			}
			if v.kind == 's' && s.kind != "" {
				out.WriteString(raw)
				continue
			}
		}
		left, right := 0, 0
		switch align {
		case '<':
			right = pad
		case '^':
			left = pad / 2
			right = pad - left
		default:
			left = pad
		}
		out.WriteString(strings.Repeat(string(fill), left))
		out.WriteString(raw)
		out.WriteString(strings.Repeat(string(fill), right))
	}
	return out.String(), nil
}
func (v value) raw(s spec) string {
	hexDebug := s.kind == "x?" || s.kind == "X?"
	if hexDebug {
		s.plus = false
		s.alt = false
		s.precision = -1
	}
	switch v.kind {
	case 's':
		if s.kind != "" {
			return debugString(v.s)
		}
		rs := []rune(v.s)
		if s.precision >= 0 && s.precision < len(rs) {
			rs = rs[:s.precision]
		}
		return string(rs)
	case 'b':
		text := strconv.FormatBool(v.b)
		if s.precision >= 0 && s.precision < len(text) {
			text = text[:s.precision]
		}
		return text
	case 'i':
		if hexDebug {
			n := new(big.Int).Set(v.n)
			if n.Sign() < 0 {
				n.Add(n, new(big.Int).Lsh(big.NewInt(1), 128))
			}
			text := n.Text(16)
			if s.kind == "X?" {
				text = strings.ToUpper(text)
			}
			return text
		}
		text := v.n.String()
		if s.plus && v.n.Sign() >= 0 {
			text = "+" + text
		}
		return text
	case 'f':
		var text string
		if math.IsNaN(v.f) {
			return "NaN"
		}
		if math.IsInf(v.f, 0) {
			text = rustrt.FloatDisplay(v.f)
		} else if s.precision >= 0 {
			text = strconv.FormatFloat(v.f, 'f', s.precision, 64)
		} else if s.kind != "" {
			text = rustrt.FloatDebug(v.f)
		} else {
			text = rustrt.FloatDisplay(v.f)
		}
		if s.plus && !math.Signbit(v.f) {
			text = "+" + text
		}
		return text
	}
	return ""
}

func debugString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case 0:
			b.WriteString(`\0`)
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		default:
			escaped := !unicode.IsPrint(r) || r != 32 && unicode.Is(unicode.Zs, r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Other_Grapheme_Extend, r)
			if debugInRanges(debugEscapeAdded, r) {
				escaped = true
			}
			if debugInRanges(debugEscapeRemoved, r) {
				escaped = false
			}
			if escaped {
				b.WriteString(`\u{` + strconv.FormatInt(int64(r), 16) + `}`)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func debugInRanges(rs [][2]rune, r rune) bool {
	i := sort.Search(len(rs), func(i int) bool { return rs[i][1] >= r })
	return i < len(rs) && rs[i][0] <= r
}
