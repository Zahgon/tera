package tera

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type jsonParser struct {
	data      []byte
	at, depth int
}

func (j *jsonParser) error(msg string, peek bool) error {
	end := j.at
	if peek && end < len(j.data) {
		end++
	}
	line, col := 1, 0
	for _, b := range j.data[:end] {
		if b == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return fmt.Errorf("%s at line %d column %d", msg, line, col)
}
func (j *jsonParser) space() {
	for j.at < len(j.data) {
		c := j.data[j.at]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return
		}
		j.at++
	}
}
func (j *jsonParser) peek() byte {
	if j.at >= len(j.data) {
		return 0
	}
	return j.data[j.at]
}

// ValueFromJSON parses JSON using serde_json's number, ordering and diagnostic semantics.
func ValueFromJSON(data []byte) (Value, error) {
	j := jsonParser{data: data}
	v, e := j.value()
	if e != nil {
		return Undefined(), e
	}
	j.space()
	if j.at < len(data) {
		return Undefined(), j.error("trailing characters", true)
	}
	return v, nil
}
func (j *jsonParser) value() (Value, error) {
	j.space()
	if j.at >= len(j.data) {
		return Undefined(), j.error("EOF while parsing a value", true)
	}
	switch j.peek() {
	case 'n', 't', 'f':
		word := "null"
		v := None()
		if j.peek() == 't' {
			word = "true"
			v = Bool(true)
		} else if j.peek() == 'f' {
			word = "false"
			v = Bool(false)
		}
		for i := 0; i < len(word); i++ {
			if j.at >= len(j.data) {
				return Undefined(), j.error("EOF while parsing a value", false)
			}
			c := j.data[j.at]
			j.at++
			if c != word[i] {
				return Undefined(), j.error("expected ident", false)
			}
		}
		return v, nil
	case '"':
		s, e := j.string()
		return NormalString(s), e
	case '[', '{':
		j.depth++
		defer func() { j.depth-- }()
		if j.depth >= 128 {
			return Undefined(), j.error("recursion limit exceeded", true)
		}
		mapKind := j.peek() == '{'
		j.at++
		if mapKind {
			return j.object()
		}
		return j.array()
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return j.number()
	default:
		return Undefined(), j.error("expected value", true)
	}
}
func (j *jsonParser) array() (Value, error) {
	a := []Value{}
	for {
		j.space()
		if j.at >= len(j.data) {
			return Undefined(), j.error("EOF while parsing a list", true)
		}
		if j.peek() == ']' {
			j.at++
			return Array(a), nil
		}
		if len(a) > 0 {
			if j.peek() != ',' {
				return Undefined(), j.error("expected `,` or `]`", true)
			}
			j.at++
			j.space()
			if j.at >= len(j.data) {
				return Undefined(), j.error("EOF while parsing a value", true)
			}
			if j.peek() == ']' {
				return Undefined(), j.error("trailing comma", true)
			}
		}
		v, e := j.value()
		if e != nil {
			return Undefined(), e
		}
		a = append(a, v)
	}
}
func (j *jsonParser) object() (Value, error) {
	m := NewMap()
	first := true
	for {
		j.space()
		if j.at >= len(j.data) {
			return Undefined(), j.error("EOF while parsing an object", true)
		}
		if j.peek() == '}' {
			j.at++
			return MapValue(m), nil
		}
		if !first {
			if j.peek() != ',' {
				return Undefined(), j.error("expected `,` or `}`", true)
			}
			j.at++
			j.space()
			if j.at >= len(j.data) {
				return Undefined(), j.error("EOF while parsing a value", true)
			}
			if j.peek() == '}' {
				return Undefined(), j.error("trailing comma", true)
			}
		}
		first = false
		if j.peek() != '"' {
			return Undefined(), j.error("key must be a string", true)
		}
		key, e := j.string()
		if e != nil {
			return Undefined(), e
		}
		j.space()
		if j.at >= len(j.data) {
			return Undefined(), j.error("EOF while parsing an object", true)
		}
		if j.peek() != ':' {
			return Undefined(), j.error("expected `:`", true)
		}
		j.at++
		v, e := j.value()
		if e != nil {
			return Undefined(), e
		}
		m.InsertString(key, v)
	}
}
func (j *jsonParser) number() (Value, error) {
	start := j.at
	if j.peek() == '-' {
		j.at++
	}
	if j.at >= len(j.data) {
		return Undefined(), j.error("EOF while parsing a value", false)
	}
	c := j.peek()
	j.at++
	if c == '0' {
		if digit(j.peek()) {
			return Undefined(), j.error("invalid number", true)
		}
	} else if c >= '1' && c <= '9' {
		for digit(j.peek()) && j.at < len(j.data) {
			j.at++
		}
	} else {
		return Undefined(), j.error("invalid number", false)
	}
	if j.peek() == '.' {
		j.at++
		n := j.at
		for digit(j.peek()) && j.at < len(j.data) {
			j.at++
		}
		if j.at == n {
			msg := "invalid number"
			if j.at >= len(j.data) {
				msg = "EOF while parsing a value"
			}
			return Undefined(), j.error(msg, true)
		}
	}
	if j.peek() == 'e' || j.peek() == 'E' {
		j.at++
		if j.peek() == '+' || j.peek() == '-' {
			j.at++
		}
		if j.at >= len(j.data) {
			return Undefined(), j.error("EOF while parsing a value", false)
		}
		c := j.peek()
		j.at++
		if !digit(c) {
			return Undefined(), j.error("invalid number", false)
		}
		for digit(j.peek()) && j.at < len(j.data) {
			j.at++
		}
	}
	v, e := jsonNumber(string(j.data[start:j.at]))
	if e != nil {
		return Undefined(), j.error("number out of range", false)
	}
	return v, nil
}
func (j *jsonParser) hex() (uint16, error) {
	if len(j.data)-j.at < 4 {
		j.at = len(j.data)
		return 0, j.error("EOF while parsing a string", false)
	}
	s := string(j.data[j.at : j.at+4])
	j.at += 4
	n, e := strconv.ParseUint(s, 16, 16)
	if e != nil {
		return 0, j.error("invalid escape", false)
	}
	return uint16(n), nil
}
func (j *jsonParser) string() (string, error) {
	j.at++
	var b strings.Builder
	for j.at < len(j.data) {
		c := j.data[j.at]
		j.at++
		if c == '"' {
			s := b.String()
			if !utf8.ValidString(s) {
				return "", j.error("invalid unicode code point", false)
			}
			return s, nil
		}
		if c < 32 {
			return "", j.error("control character (\\u0000-\\u001F) found while parsing a string", false)
		}
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		if j.at >= len(j.data) {
			return "", j.error("EOF while parsing a string", false)
		}
		c = j.data[j.at]
		j.at++
		switch c {
		case '"', '\\', '/':
			b.WriteByte(c)
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			n, e := j.hex()
			if e != nil {
				return "", e
			}
			if n >= 0xdc00 && n <= 0xdfff {
				return "", j.error("lone leading surrogate in hex escape", false)
			}
			if n >= 0xd800 && n <= 0xdbff {
				for _, expected := range []byte{'\\', 'u'} {
					if j.at >= len(j.data) {
						return "", j.error("EOF while parsing a string", false)
					}
					c = j.data[j.at]
					j.at++
					if c != expected {
						return "", j.error("unexpected end of hex escape", false)
					}
				}
				low, e := j.hex()
				if e != nil {
					return "", e
				}
				if low < 0xdc00 || low > 0xdfff {
					return "", j.error("lone leading surrogate in hex escape", false)
				}
				b.WriteRune(rune(0x10000 + (uint32(n)-0xd800)*1024 + (uint32(low) - 0xdc00)))
			} else {
				b.WriteRune(rune(n))
			}
		default:
			return "", j.error("invalid escape", false)
		}
	}
	return "", j.error("EOF while parsing a string", false)
}

// MarshalJSON implements encoding/json.Marshaler using Tera's Serialize semantics.
func (v Value) MarshalJSON() ([]byte, error) { s, e := MarshalJSON(v, false); return []byte(s), e }

// UnmarshalJSON implements encoding/json.Unmarshaler using ordered JSON conversion.
func (v *Value) UnmarshalJSON(data []byte) error {
	if v == nil {
		return fmt.Errorf("cannot unmarshal into a nil Value")
	}
	decoded, e := ValueFromJSON(data)
	if e != nil {
		return e
	}
	*v = decoded
	return nil
}
