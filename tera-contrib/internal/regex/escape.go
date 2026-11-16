package regex

import (
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode/utf8"
)

const incompleteEscape = "incomplete escape sequence, reached end of pattern prematurely"

func rawClass(s runeSet) *syntax.Regexp {
	rs := make([]rune, 0, 2*len(s))
	for _, v := range s {
		rs = append(rs, v.lo, v.hi)
	}
	return &syntax.Regexp{Op: syntax.OpCharClass, Rune: rs}
}
func nodeSet(n *syntax.Regexp) runeSet {
	if n.Op == syntax.OpCapture && n.Cap >= boundaryCaptureBase {
		return runeSet{{assertionBase, assertionBase}}
	}
	if n.Op == syntax.OpLiteral && len(n.Rune) == 1 {
		return runeSet{{n.Rune[0], n.Rune[0]}}
	}
	var s runeSet
	for i := 0; i+1 < len(n.Rune); i += 2 {
		s = append(s, interval{n.Rune[i], n.Rune[i+1]})
	}
	return s
}
func (p *parser) escape(inClass bool) (*syntax.Regexp, *rune, error) {
	start := p.pos
	p.pos++
	if p.pos == len(p.source) {
		return nil, nil, p.errorAt(start, p.pos, incompleteEscape)
	}
	c := p.take()
	if c >= '0' && c <= '9' {
		return nil, nil, p.errorAt(start, p.pos, "backreferences are not supported")
	}
	literal := func(r rune) (*syntax.Regexp, *rune, error) {
		if inClass {
			return &syntax.Regexp{Op: syntax.OpLiteral, Rune: []rune{r}}, &r, nil
		}
		return p.literal(r, start, p.pos), &r, nil
	}
	switch c {
	case 'd', 'D', 's', 'S', 'w', 'W':
		set := p.fold(perl(byte(c), p.flags&flagUnicode == 0))
		if p.flags&flagUnicode == 0 && c >= 'A' && c <= 'Z' {
			p.translation(start, p.pos, "pattern can match invalid UTF-8")
		}
		if inClass {
			return rawClass(set), nil, nil
		}
		return p.classNode(set, start, p.pos), nil, nil
	case 'p', 'P':
		p.skip()
		if p.pos == len(p.source) {
			return nil, nil, p.errorAt(p.pos, p.pos, incompleteEscape)
		}
		name := ""
		if p.peek() == '{' {
			p.pos++
			var nameBuf strings.Builder
			p.skip()
			for p.pos < len(p.source) && p.peek() != '}' {
				nameBuf.WriteRune(p.take())
				p.skip()
			}
			if p.pos == len(p.source) {
				return nil, nil, p.errorAt(p.pos, p.pos, incompleteEscape)
			}
			p.pos++
			name = nameBuf.String()
		} else {
			if p.peek() == '\\' {
				return nil, nil, p.errorAt(p.pos, p.pos+1, "invalid Unicode character class")
			}
			name = string(p.take())
			p.skip()
		}
		negated := c == 'P'
		if strings.Contains(name, "!=") {
			name = strings.Replace(name, "!=", "=", 1)
			negated = !negated
		}
		set, msg := property(name)
		if p.flags&flagUnicode == 0 {
			p.translation(start, p.pos, "Unicode not allowed here")
		} else if msg != "" {
			p.translation(start, p.pos, msg)
		}
		set = p.fold(set)
		if negated {
			set = complement(set, utf8.MaxRune)
		}
		if inClass {
			return rawClass(set), nil, nil
		}
		return p.classNode(set, start, p.pos), nil, nil
	case 'x', 'u', 'U':
		p.skip()
		if p.pos == len(p.source) {
			return nil, nil, p.errorAt(p.pos, p.pos, incompleteEscape)
		}
		digitsStart := p.pos
		digitsEnd := p.pos
		var hex strings.Builder
		braced := p.peek() == '{'
		if braced {
			brace := p.pos
			p.pos++
			digitsStart = p.pos
			p.skip()
			for p.pos < len(p.source) && p.peek() != '}' {
				r := p.peek()
				if !hexDigit(r) {
					_, n := utf8.DecodeRuneInString(p.source[p.pos:])
					return nil, nil, p.errorAt(p.pos, p.pos+n, "invalid hexadecimal digit")
				}
				hex.WriteRune(p.take())
				p.skip()
			}
			if p.pos == len(p.source) {
				return nil, nil, p.errorAt(brace, p.pos, incompleteEscape)
			}
			digitsEnd = p.pos
			p.pos++
			p.skip()
			if hex.Len() == 0 {
				return nil, nil, p.errorAt(brace, p.pos, "hexadecimal literal empty")
			}
		} else {
			n := 2
			if c == 'u' {
				n = 4
			} else if c == 'U' {
				n = 8
			}
			for i := 0; i < n; i++ {
				if p.pos == len(p.source) {
					return nil, nil, p.errorAt(p.pos, p.pos, incompleteEscape)
				}
				r := p.peek()
				if !hexDigit(r) {
					_, n := utf8.DecodeRuneInString(p.source[p.pos:])
					return nil, nil, p.errorAt(p.pos, p.pos+n, "invalid hexadecimal digit")
				}
				hex.WriteRune(p.take())
				p.skip()
			}
			digitsEnd = p.pos
		}
		n, err := strconv.ParseUint(hex.String(), 16, 32)
		if err != nil || n > utf8.MaxRune || n >= 0xd800 && n <= 0xdfff {
			return nil, nil, p.errorAt(digitsStart, digitsEnd, "hexadecimal literal is not a Unicode scalar value")
		}
		r := rune(n)
		if p.flags&flagUnicode == 0 && r >= 128 {
			if c == 'x' && !braced {
				p.translation(start, p.pos, "pattern can match invalid UTF-8")
			} else if inClass {
				p.translation(start, p.pos, "Unicode not allowed here")
			}
		}
		if inClass {
			return &syntax.Regexp{Op: syntax.OpLiteral, Rune: []rune{r}}, &r, nil
		}
		// A byte escape is permitted in byte mode when it denotes ASCII.
		if p.flags&flagUnicode == 0 && r >= 128 {
			return &syntax.Regexp{Op: syntax.OpLiteral, Rune: []rune{r}}, &r, nil
		}
		return rawClass(p.fold(runeSet{{r, r}})), &r, nil
	case 'a':
		return literal(7)
	case 'f':
		return literal(12)
	case 't':
		return literal('\t')
	case 'n':
		return literal('\n')
	case 'r':
		return literal('\r')
	case 'v':
		return literal(11)
	case 'A', 'z', 'b', 'B', '<', '>':
		if inClass && (c == 'A' || c == 'z') {
			return boundary(wordBoundary, false), nil, nil
		}
		switch c {
		case 'A':
			return &syntax.Regexp{Op: syntax.OpBeginText}, nil, nil
		case 'z':
			return &syntax.Regexp{Op: syntax.OpEndText}, nil, nil
		}
		code := wordBoundary
		switch c {
		case 'B':
			code = notWordBoundary
		case '<':
			code = wordStart
		case '>':
			code = wordEnd
		}
		if c == 'b' && p.peek() == '{' {
			brace := p.pos
			p.pos++
			p.skip()
			if p.pos == len(p.source) {
				return nil, nil, p.errorAt(start, p.pos, "found either the beginning of a special word boundary or a bounded repetition on a \\b with an opening brace, but no closing brace")
			}
			valid := func(r rune) bool { return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '-' }
			if !valid(p.peek()) {
				p.pos = brace
			} else {
				begin := p.pos
				var name strings.Builder
				for p.pos < len(p.source) && valid(p.peek()) {
					name.WriteRune(p.take())
					p.skip()
				}
				end := p.pos
				if p.peek() != '}' {
					return nil, nil, p.errorAt(brace, p.pos, "special word boundary assertion is either unclosed or contains an invalid character")
				}
				p.pos++
				switch name.String() {
				case "start":
					code = wordStart
				case "end":
					code = wordEnd
				case "start-half":
					code = wordStartHalf
				case "end-half":
					code = wordEndHalf
				default:
					return nil, nil, p.errorAt(begin, end, "unrecognized special word boundary assertion, valid choices are: start, end, start-half or end-half")
				}
			}
		}
		return boundary(code, p.flags&flagUnicode == 0), nil, nil
	}
	if c < 128 && !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
		return literal(c)
	}
	return nil, nil, p.errorAt(start, p.pos, "unrecognized escape sequence")
}
func hexDigit(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'
}
