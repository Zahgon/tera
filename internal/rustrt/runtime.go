// Package rustrt implements the Rust standard library semantics used by Tera.
package rustrt

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FloatDisplay formats a float like Rust Display.
func FloatDisplay(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	return rustShortest(f, 'f')
}

// FloatDebug formats a float like Rust Debug.
func FloatDebug(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return FloatDisplay(f)
	}
	x := math.Abs(f)
	if x != 0 && (x < 1e-4 || x >= 1e16) {
		s := rustShortest(f, 'e')
		a := strings.Split(s, "e")
		e, _ := strconv.Atoi(a[1])
		return a[0] + "e" + strconv.Itoa(e)
	}
	s := FloatDisplay(f)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// JSONFloat formats a finite float as serde_json does.
func JSONFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	x := math.Abs(f)
	if x != 0 && (x < 1e-5 || x >= 1e16) {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		a := strings.Split(s, "e")
		e, _ := strconv.Atoi(a[1])
		sign := ""
		if e >= 0 {
			sign = "+"
		}
		return a[0] + "e" + sign + strconv.Itoa(e)
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// DebugString formats a string using Rust's str Debug escaping.
func DebugString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i, r := range []rune(s) {
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
			if debugEscape(r, i == 0) {
				b.WriteString(`\u{` + strconv.FormatInt(int64(r), 16) + `}`)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ParseFloat accepts Rust's decimal and special float grammar.
func ParseFloat(s string) (float64, error) {
	t := strings.ToLower(s)
	u := t
	if strings.HasPrefix(u, "+") || strings.HasPrefix(u, "-") {
		u = u[1:]
	}
	if u == "nan" {
		return math.NaN(), nil
	}
	if u == "inf" || u == "infinity" {
		if strings.HasPrefix(t, "-") {
			return math.Inf(-1), nil
		}
		return math.Inf(1), nil
	}
	if strings.ContainsAny(s, "xXpP_") || strings.TrimSpace(s) != s {
		return 0, strconv.ErrSyntax
	}
	f, e := strconv.ParseFloat(s, 64)
	if e != nil && math.IsInf(f, 0) {
		return f, nil
	}
	return f, e
}

// Trim removes Rust Unicode White_Space.
func Trim(s string) string { return strings.TrimFunc(s, unicode.IsSpace) }

// TrimStart removes leading Rust Unicode White_Space.
func TrimStart(s string) string { return strings.TrimLeftFunc(s, unicode.IsSpace) }

// TrimEnd removes trailing Rust Unicode White_Space.
func TrimEnd(s string) string { return strings.TrimRightFunc(s, unicode.IsSpace) }

// Lines implements str::lines, including CRLF handling.
func Lines(s string) []string {
	if s == "" {
		return nil
	}
	a := strings.Split(s, "\n")
	if a[len(a)-1] == "" {
		a = a[:len(a)-1]
	}
	for i := range a {
		if i < len(a)-1 || strings.HasSuffix(s, "\n") {
			a[i] = strings.TrimSuffix(a[i], "\r")
		}
	}
	return a
}

// UpperChar performs full Unicode uppercase mapping.
func UpperChar(r rune) string {
	if s, ok := upperSpecial[r]; ok {
		return s
	}
	return string(r)
}

// LowerChar performs full Unicode lowercase mapping.
func LowerChar(r rune) string {
	if s, ok := lowerSpecial[r]; ok {
		return s
	}
	return string(r)
}

// Upper performs full Unicode uppercase mapping.
func Upper(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteString(UpperChar(r))
	}
	return b.String()
}

// Lower performs full Unicode lowercase mapping with contextual final sigma.
func Lower(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		if r == 'Σ' {
			before, after := false, false
			for j := i - 1; j >= 0; j-- {
				if inRanges(rs[j], caseIgnorable) {
					continue
				}
				before = inRanges(rs[j], cased)
				break
			}
			for j := i + 1; j < len(rs); j++ {
				if inRanges(rs[j], caseIgnorable) {
					continue
				}
				after = inRanges(rs[j], cased)
				break
			}
			if before && !after {
				b.WriteRune('ς')
				continue
			}
		}
		b.WriteString(LowerChar(r))
	}
	return b.String()
}

// Rust's shortest formatter resolves exact decimal ties away from zero.
func rustShortest(f float64, format byte) string {
	text := strconv.FormatFloat(f, format, -1, 64)
	if f == 0 || math.IsInf(f, 0) || math.IsNaN(f) {
		return text
	}
	abs := math.Abs(f)
	sci := strconv.FormatFloat(abs, 'e', -1, 64)
	mant, expText, _ := strings.Cut(sci, "e")
	exp, _ := strconv.Atoi(expText)
	digits := strings.ReplaceAll(mant, ".", "")
	power := exp - len(digits) + 1
	n, _ := new(big.Int).SetString(digits, 10)
	unit := new(big.Rat)
	p := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(absInt(power))), nil)
	if power >= 0 {
		unit.SetInt(p)
	} else {
		unit.SetFrac(big.NewInt(1), p)
	}
	candidate := new(big.Rat).Mul(new(big.Rat).SetInt(n), unit)
	diff := new(big.Rat).Sub(new(big.Rat).SetFloat64(abs), candidate)
	if diff.Sign() <= 0 || new(big.Rat).Mul(diff, big.NewRat(2, 1)).Cmp(unit) != 0 {
		return text
	}
	n.Add(n, big.NewInt(1))
	ds := n.String()
	if format == 'e' {
		exponent := power + len(ds) - 1
		mant = ds[:1]
		if len(ds) > 1 {
			mant += "." + ds[1:]
		}
		text = mant + "e" + strconv.Itoa(exponent)
	} else if power >= 0 {
		text = ds + strings.Repeat("0", power)
	} else {
		point := len(ds) + power
		if point > 0 {
			text = ds[:point] + "." + ds[point:]
		} else {
			text = "0." + strings.Repeat("0", -point) + ds
		}
	}
	if f < 0 {
		text = "-" + text
	}
	return text
}
func absInt(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

// LossyUTF8 replaces ill-formed UTF-8 using Rust's maximal-subpart behavior.
func LossyUTF8(data []byte) string {
	var b strings.Builder
	for i := 0; i < len(data); {
		c := data[i]
		if c < 0x80 {
			b.WriteByte(c)
			i++
			continue
		}
		size := 0
		switch {
		case c >= 0xc2 && c <= 0xdf:
			size = 2
		case c >= 0xe0 && c <= 0xef:
			size = 3
		case c >= 0xf0 && c <= 0xf4:
			size = 4
		}
		if size == 0 {
			b.WriteRune('�')
			i++
			continue
		}
		consumed := 1
		for consumed < size && i+consumed < len(data) {
			n := data[i+consumed]
			if n < 0x80 || n > 0xbf || consumed == 1 && (c == 0xe0 && n < 0xa0 || c == 0xed && n >= 0xa0 || c == 0xf0 && n < 0x90 || c == 0xf4 && n >= 0x90) {
				break
			}
			consumed++
		}
		if consumed == size {
			b.Write(data[i : i+size])
		} else {
			b.WriteRune('�')
		}
		i += consumed
	}
	return b.String()
}

// UTF8Error describes the first UTF-8 validation failure like Rust's Utf8Error.
func UTF8Error(data []byte) string {
	for i := 0; i < len(data); {
		c := data[i]
		if c < 0x80 {
			i++
			continue
		}
		size := 0
		switch {
		case c >= 0xc2 && c <= 0xdf:
			size = 2
		case c >= 0xe0 && c <= 0xef:
			size = 3
		case c >= 0xf0 && c <= 0xf4:
			size = 4
		}
		if size == 0 {
			return "invalid utf-8 sequence of 1 bytes from index " + strconv.Itoa(i)
		}
		consumed := 1
		for consumed < size {
			if i+consumed >= len(data) {
				return "incomplete utf-8 byte sequence from index " + strconv.Itoa(i)
			}
			n := data[i+consumed]
			if n < 0x80 || n > 0xbf || consumed == 1 && (c == 0xe0 && n < 0xa0 || c == 0xed && n >= 0xa0 || c == 0xf0 && n < 0x90 || c == 0xf4 && n >= 0x90) {
				return "invalid utf-8 sequence of " + strconv.Itoa(consumed) + " bytes from index " + strconv.Itoa(i)
			}
			consumed++
		}
		i += size
	}
	return ""
}

// DebugPath formats Unix path bytes using Rust's OsStr Debug rules.
func DebugPath(path string) string {
	var b strings.Builder
	b.WriteByte('"')
	first := true
	for len(path) > 0 {
		r, n := utf8.DecodeRuneInString(path)
		if r == utf8.RuneError && n == 1 {
			b.WriteString(`\x`)
			const hex = "0123456789ABCDEF"
			b.WriteByte(hex[path[0]>>4])
			b.WriteByte(hex[path[0]&15])
			path = path[1:]
			first = false
			continue
		}
		path = path[n:]
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
			if debugEscape(r, first) {
				b.WriteString(`\u{` + strconv.FormatInt(int64(r), 16) + `}`)
			} else {
				b.WriteRune(r)
			}
		}
		first = false
	}
	b.WriteByte('"')
	return b.String()
}
