package regex

import (
	"fmt"
	"regexp/syntax"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	flagCase uint16 = 1 << iota
	flagMulti
	flagDot
	flagUngreedy
	flagUnicode
	flagVerbose
	flagCRLF
)

type span struct{ start, end int }
type parseError struct {
	pattern, message string
	spans            []span
}

func (e *parseError) Error() string {
	var out strings.Builder
	out.WriteString("regex parse error:\n")
	multi := strings.ContainsRune(e.pattern, '\n')
	divider := strings.Repeat("~", 79)
	if multi {
		out.WriteString(divider + "\n")
	}
	lines := strings.Split(e.pattern, "\n")
	lineWidth := len(strconv.Itoa(len(lines)))
	spans := append([]span(nil), e.spans...)
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	offset := 0
	var notes []string
	for i, line := range lines {
		if i == len(lines)-1 && line == "" && multi {
			break
		}
		display := strings.TrimSuffix(line, "\r")
		padding := 4
		if multi {
			out.WriteString(fmt.Sprintf("%*d: ", lineWidth, i+1))
			padding = lineWidth + 2
		} else {
			out.WriteString("    ")
		}
		out.WriteString(display + "\n")
		cursor := 0
		var carets strings.Builder
		for _, s := range spans {
			startLine := strings.Count(e.pattern[:s.start], "\n")
			endLine := strings.Count(e.pattern[:s.end], "\n")
			if startLine != endLine {
				if startLine == i {
					col := utf8.RuneCountInString(e.pattern[strings.LastIndex(e.pattern[:s.start], "\n")+1:s.start]) + 1
					last := strings.LastIndex(e.pattern[:s.end], "\n") + 1
					endCol := utf8.RuneCountInString(e.pattern[last:s.end])
					notes = append(notes, fmt.Sprintf("on line %d (column %d) through line %d (column %d)", startLine+1, col, endLine+1, endCol))
				}
				continue
			}
			if startLine != i {
				continue
			}
			col := utf8.RuneCountInString(e.pattern[offset:s.start])
			width := utf8.RuneCountInString(e.pattern[s.start:s.end])
			if width == 0 {
				width = 1
			}
			if carets.Len() == 0 {
				carets.WriteString(strings.Repeat(" ", padding))
			}
			if col > cursor {
				carets.WriteString(strings.Repeat(" ", col-cursor))
				cursor = col
			}
			carets.WriteString(strings.Repeat("^", width))
			cursor += width
		}
		if carets.Len() > 0 {
			out.WriteString(carets.String() + "\n")
		}
		offset += len(line) + 1
	}
	if multi {
		out.WriteString(divider + "\n")
		for _, n := range notes {
			out.WriteString(n + "\n")
		}
	}
	out.WriteString("error: " + e.message)
	return out.String()
}

type parser struct {
	source               string
	pos, depth, captures int
	flags                uint16
	names                map[string]int
	nameSpans            map[string]span
	translateErr         error
	nest                 map[*syntax.Regexp]*nestNode
	classNest            *nestNode
}

func (p *parser) errorAt(start, end int, msg string, aux ...span) error {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(p.source) {
		end = len(p.source)
	}
	ss := append([]span{{start, end}}, aux...)
	return &parseError{p.source, msg, ss}
}
func (p *parser) translation(start, end int, msg string) {
	if p.translateErr == nil {
		p.translateErr = p.errorAt(start, end, msg)
	}
}
func (p *parser) peek() rune {
	if p.pos >= len(p.source) {
		return -1
	}
	r, _ := utf8.DecodeRuneInString(p.source[p.pos:])
	return r
}
func (p *parser) take() rune { r, n := utf8.DecodeRuneInString(p.source[p.pos:]); p.pos += n; return r }
func (p *parser) skip() {
	if p.flags&flagVerbose == 0 {
		return
	}
	for p.pos < len(p.source) {
		r := p.peek()
		if unicode.IsSpace(r) {
			p.take()
			continue
		}
		if r == '#' {
			for p.pos < len(p.source) && p.take() != '\n' {
			}
			continue
		}
		break
	}
}
func (p *parser) combine(op syntax.Op, subs []*syntax.Regexp, start, end int) *syntax.Regexp {
	if len(subs) == 0 {
		return p.note(&syntax.Regexp{Op: syntax.OpEmptyMatch}, &nestNode{span: span{start, end}})
	}
	if len(subs) == 1 {
		return subs[0]
	}
	children := make([]*nestNode, len(subs))
	for i, n := range subs {
		children[i] = p.nest[n]
	}
	return p.note(&syntax.Regexp{Op: op, Sub: subs}, &nestNode{span: span{start, end}, children: children, composite: true})
}
func (p *parser) expression(inGroup bool) (*syntax.Regexp, error) {
	exprStart, partStart := p.pos, p.pos
	var alternatives, parts []*syntax.Regexp
	canRepeat := false
	for {
		p.skip()
		if p.pos == len(p.source) {
			break
		}
		start := p.pos
		c := p.peek()
		if c == ')' {
			if !inGroup {
				return nil, p.errorAt(start, start+1, "unopened group")
			}
			break
		}
		if c == '|' {
			p.pos++
			alternatives = append(alternatives, p.combine(syntax.OpConcat, parts, partStart, start))
			parts = nil
			partStart = p.pos
			canRepeat = false
			continue
		}
		if c == '*' || c == '+' || c == '?' || c == '{' {
			if !canRepeat || len(parts) == 0 {
				return nil, p.errorAt(start, start+1, "repetition operator missing expression")
			}
			node, err := p.repeat(parts[len(parts)-1])
			if err != nil {
				return nil, err
			}
			parts[len(parts)-1] = node
			continue
		}
		var node *syntax.Regexp
		var err error
		switch c {
		case '(':
			node, err = p.group()
		case '[':
			var set runeSet
			set, err = p.class()
			if err == nil {
				node = p.classNode(set, start, p.pos)
				p.note(node, p.classNest)
			}
		case '\\':
			node, _, err = p.escape(false)
		case '.':
			p.pos++
			set := runeSet{{0, utf8.MaxRune}}
			if p.flags&flagDot == 0 {
				set = difference(set, runeSet{{'\n', '\n'}})
				if p.flags&flagCRLF != 0 {
					set = difference(set, runeSet{{'\r', '\r'}})
				}
			}
			node = p.classNode(set, start, p.pos)
		case '^', '$':
			p.pos++
			op := syntax.OpBeginText
			if c == '$' {
				op = syntax.OpEndText
			}
			if p.flags&flagMulti != 0 {
				if p.flags&flagCRLF != 0 {
					code := lineStartCRLF
					if c == '$' {
						code = lineEndCRLF
					}
					node = boundary(code, false)
				} else {
					op = syntax.OpBeginLine
					if c == '$' {
						op = syntax.OpEndLine
					}
				}
			}
			if node == nil {
				node = &syntax.Regexp{Op: op}
			}
		default:
			p.take()
			node = p.literal(c, start, p.pos)
		}
		if err != nil {
			return nil, err
		}
		canRepeat = node != nil
		if node == nil {
			node = &syntax.Regexp{Op: syntax.OpEmptyMatch}
		}
		if p.nest[node] == nil {
			p.note(node, &nestNode{span: span{start, p.pos}})
		}
		parts = append(parts, node)
	}
	alternatives = append(alternatives, p.combine(syntax.OpConcat, parts, partStart, p.pos))
	return p.combine(syntax.OpAlternate, alternatives, exprStart, p.pos), nil
}
func boundary(code int, ascii bool) *syntax.Regexp {
	if ascii {
		code += 32
	}
	return &syntax.Regexp{Op: syntax.OpCapture, Cap: boundaryCaptureBase + code, Sub: []*syntax.Regexp{{Op: syntax.OpEmptyMatch}}}
}
func (p *parser) literal(r rune, start, end int) *syntax.Regexp {
	if p.flags&flagCase != 0 && (p.flags&flagUnicode != 0 || r < 128) {
		return p.classNode(p.fold(runeSet{{r, r}}), start, end)
	}
	return &syntax.Regexp{Op: syntax.OpLiteral, Rune: []rune{r}}
}
func (p *parser) group() (*syntax.Regexp, error) {
	start := p.pos
	p.pos++
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 10000 {
		return nil, p.errorAt(start, start+1, "exceed the maximum number of nested parentheses/brackets (250)")
	}
	saved := p.flags
	capture := true
	name := ""
	if p.peek() == '?' {
		p.pos++
		if p.peek() == ')' {
			return nil, p.errorAt(start+1, start+2, "repetition operator missing expression")
		}
		if p.peek() == '=' || p.peek() == '!' {
			return nil, p.errorAt(start, p.pos+1, "look-around, including look-ahead and look-behind, is not supported")
		}
		if p.peek() == '<' && p.pos+1 < len(p.source) && (p.source[p.pos+1] == '=' || p.source[p.pos+1] == '!') {
			return nil, p.errorAt(start, p.pos+2, "look-around, including look-ahead and look-behind, is not supported")
		}
		if strings.HasPrefix(p.source[p.pos:], "P<") || p.peek() == '<' {
			if p.peek() == 'P' {
				p.pos++
			}
			p.pos++
			nameStart := p.pos
			for p.pos < len(p.source) && p.peek() != '>' {
				r := p.peek()
				if !(nameAlphabetic(r) || r == '_' || p.pos > nameStart && (nameNumeric(r) || r == '.' || r == '[' || r == ']')) {
					_, n := utf8.DecodeRuneInString(p.source[p.pos:])
					return nil, p.errorAt(p.pos, p.pos+n, "invalid capture group character")
				}
				p.take()
			}
			if p.pos == len(p.source) {
				return nil, p.errorAt(p.pos, p.pos, "unclosed capture group name")
			}
			if p.pos == nameStart {
				return nil, p.errorAt(p.pos, p.pos, "empty capture group name")
			}
			name = p.source[nameStart:p.pos]
			if original, ok := p.nameSpans[name]; ok {
				return nil, p.errorAt(nameStart, p.pos, "duplicate capture group name", original)
			}
			p.nameSpans[name] = span{nameStart, p.pos}
			p.pos++
		} else {
			capture = false
			seen := make(map[rune]span)
			negative := false
			minus := -1
			any := false
			for p.pos < len(p.source) && p.peek() != ')' && p.peek() != ':' {
				i := p.pos
				r := p.take()
				if r == '-' {
					if minus >= 0 {
						return nil, p.errorAt(i, p.pos, "flag negation operator repeated", span{minus, minus + 1})
					}
					minus = i
					negative = true
					continue
				}
				bit := uint16(0)
				switch r {
				case 'i':
					bit = flagCase
				case 'm':
					bit = flagMulti
				case 's':
					bit = flagDot
				case 'U':
					bit = flagUngreedy
				case 'u':
					bit = flagUnicode
				case 'x':
					bit = flagVerbose
				case 'R':
					bit = flagCRLF
				default:
					return nil, p.errorAt(i, p.pos, "unrecognized flag")
				}
				if old, ok := seen[r]; ok {
					return nil, p.errorAt(i, p.pos, "duplicate flag", old)
				}
				seen[r] = span{i, p.pos}
				any = true
				if negative {
					p.flags &^= bit
				} else {
					p.flags |= bit
				}
			}
			if p.pos == len(p.source) {
				if !any && minus < 0 {
					return nil, p.errorAt(start, start+1, "unclosed group")
				}
				return nil, p.errorAt(p.pos, p.pos, "expected flag but got end of regex")
			}
			if minus >= 0 && p.pos == minus+1 {
				return nil, p.errorAt(minus, minus+1, "dangling flag negation operator")
			}
			if p.peek() == ')' {
				p.pos++
				return nil, nil
			}
			p.pos++
		}
	}
	capID := 0
	if capture {
		p.captures++
		capID = p.captures
		if name != "" {
			p.names[name] = capID
		}
	}
	sub, err := p.expression(true)
	if err != nil {
		return nil, err
	}
	if p.pos == len(p.source) {
		return nil, p.errorAt(start, start+1, "unclosed group")
	}
	p.pos++
	p.flags = saved
	info := &nestNode{span: span{start, p.pos}, composite: true, children: []*nestNode{p.nest[sub]}}
	if capture {
		return p.note(&syntax.Regexp{Op: syntax.OpCapture, Cap: capID, Name: name, Sub: []*syntax.Regexp{sub}}, info), nil
	}
	return p.note(&syntax.Regexp{Op: syntax.OpConcat, Sub: []*syntax.Regexp{sub}}, info), nil
}
func (p *parser) repeat(sub *syntax.Regexp) (*syntax.Regexp, error) {
	start := p.pos
	c := p.take()
	node := &syntax.Regexp{Sub: []*syntax.Regexp{sub}}
	switch c {
	case '*':
		node.Op = syntax.OpStar
	case '+':
		node.Op = syntax.OpPlus
	case '?':
		node.Op = syntax.OpQuest
	case '{':
		node.Op = syntax.OpRepeat
		p.skip()
		if p.pos == len(p.source) {
			return nil, p.errorAt(start, p.pos, "unclosed counted repetition")
		}
		count := func() (int, error) {
			for p.pos < len(p.source) && unicode.IsSpace(p.peek()) {
				p.take()
			}
			begin := p.pos
			var digits strings.Builder
			for p.pos < len(p.source) && p.peek() >= '0' && p.peek() <= '9' {
				digits.WriteRune(p.take())
				p.skip()
			}
			spanEnd := p.pos
			for p.pos < len(p.source) && unicode.IsSpace(p.peek()) {
				p.take()
				p.skip()
			}
			if digits.Len() == 0 {
				return 0, p.errorAt(begin, spanEnd, "repetition quantifier expects a valid decimal")
			}
			n, e := strconv.ParseUint(digits.String(), 10, 32)
			if e != nil {
				return 0, p.errorAt(begin, spanEnd, "decimal literal invalid")
			}
			return int(n), nil
		}
		minimum, countErr := count()
		if p.pos == len(p.source) {
			return nil, p.errorAt(start, p.pos, "unclosed counted repetition")
		}
		node.Min = minimum
		node.Max = minimum
		if p.peek() == ',' {
			p.pos++
			p.skip()
			if p.pos == len(p.source) {
				return nil, p.errorAt(start, p.pos, "unclosed counted repetition")
			}
			if countErr != nil {
				return nil, countErr
			}
			node.Max = -1
			if p.peek() != '}' {
				var err error
				node.Max, err = count()
				if err != nil {
					return nil, err
				}
			}
		} else if countErr != nil {
			return nil, countErr
		}
		if p.peek() != '}' {
			return nil, p.errorAt(start, p.pos, "unclosed counted repetition")
		}
		p.pos++
	}

	lazy := p.flags&flagUngreedy != 0
	p.skip()
	if p.peek() == '?' {
		p.pos++
		lazy = !lazy
	}
	if lazy {
		node.Flags |= syntax.NonGreedy
	}
	if node.Op == syntax.OpRepeat && node.Max >= 0 && node.Min > node.Max {
		return nil, p.errorAt(start, p.pos, "invalid repetition count range, the start must be <= the end")
	}
	begin := start
	if info := p.nest[sub]; info != nil {
		begin = info.start
	}
	return p.note(node, &nestNode{span: span{begin, p.pos}, composite: true, children: []*nestNode{p.nest[sub]}}), nil
}

// nestNode retains source-level nesting after character classes and flags have
// been lowered to instructions. Rust limits syntactic nesting before lowering.
type nestNode struct {
	span
	composite bool
	children  []*nestNode
}

func (p *parser) note(n *syntax.Regexp, info *nestNode) *syntax.Regexp { p.nest[n] = info; return n }
func (p *parser) checkNesting(n *nestNode, depth int) error {
	if n == nil {
		return nil
	}
	if n.composite {
		depth++
		if depth > 250 {
			return p.errorAt(n.start, n.end, "exceed the maximum number of nested parentheses/brackets (250)")
		}
	}
	for _, child := range n.children {
		if err := p.checkNesting(child, depth); err != nil {
			return err
		}
	}
	return nil
}
