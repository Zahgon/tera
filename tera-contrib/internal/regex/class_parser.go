package regex

import (
	"strings"
	"unicode/utf8"
)

func (p *parser) asciiClass() (runeSet, bool) {
	start := p.pos
	if !strings.HasPrefix(p.source[start:], "[:") {
		return nil, false
	}
	end := strings.Index(p.source[start+2:], ":]")
	if end < 0 {
		return nil, false
	}
	name := p.source[start+2 : start+2+end]
	neg := strings.HasPrefix(name, "^")
	name = strings.TrimPrefix(name, "^")
	var s runeSet
	switch name {
	case "alnum":
		s = runeSet{{'0', '9'}, {'A', 'Z'}, {'a', 'z'}}
	case "alpha":
		s = runeSet{{'A', 'Z'}, {'a', 'z'}}
	case "ascii":
		s = runeSet{{0, 127}}
	case "blank":
		s = runeSet{{9, 9}, {32, 32}}
	case "cntrl":
		s = runeSet{{0, 31}, {127, 127}}
	case "digit":
		s = runeSet{{'0', '9'}}
	case "graph":
		s = runeSet{{33, 126}}
	case "lower":
		s = runeSet{{'a', 'z'}}
	case "print":
		s = runeSet{{32, 126}}
	case "punct":
		s = runeSet{{33, 47}, {58, 64}, {91, 96}, {123, 126}}
	case "space":
		s = runeSet{{9, 13}, {32, 32}}
	case "upper":
		s = runeSet{{'A', 'Z'}}
	case "word":
		s = perl('w', true)
	case "xdigit":
		s = runeSet{{'0', '9'}, {'A', 'F'}, {'a', 'f'}}
	default:
		return nil, false
	}
	p.pos = start + end + 4
	s = p.fold(s)
	if neg {
		max := rune(utf8.MaxRune)
		if p.flags&flagUnicode == 0 {
			max = 255
		}
		s = complement(s, max)
		if p.flags&flagUnicode == 0 {
			p.translation(start, p.pos, "pattern can match invalid UTF-8")
		}
	}
	return s, true
}
func (p *parser) class() (runeSet, error) {
	start := p.pos
	p.pos++
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 10000 {
		return nil, p.errorAt(start, start+1, "exceed the maximum number of nested parentheses/brackets (250)")
	}
	p.skip()
	if p.pos == len(p.source) {
		return nil, p.errorAt(start, p.pos, "unclosed character class")
	}
	neg := false
	if p.peek() == '^' {
		p.pos++
		neg = true
		p.skip()
		if p.pos == len(p.source) {
			return nil, p.errorAt(start, p.pos, "unclosed character class")
		}
	}
	var current runeSet
	var currentInfos []*nestNode
	for p.peek() == '-' {
		currentInfos = append(currentInfos, &nestNode{span: span{p.pos, p.pos + 1}})
		p.pos++
		current = union(current, runeSet{{'-', '-'}})
		p.skip()
		if p.pos == len(p.source) {
			return nil, p.errorAt(start, start, "unclosed character class")
		}
	}
	if len(current) == 0 && p.peek() == ']' {
		currentInfos = append(currentInfos, &nestNode{span: span{p.pos, p.pos + 1}})
		p.pos++
		current = runeSet{{']', ']'}}
		p.skip()
		if p.pos == len(p.source) {
			return nil, p.errorAt(start, p.pos, "unclosed character class")
		}
	}
	openEnd := p.pos
	var accumulated runeSet
	var accumulatedInfo *nestNode
	op := ""
	apply := func() {
		info := &nestNode{span: span{p.pos, p.pos}}
		if len(currentInfos) == 1 {
			info = currentInfos[0]
		} else if len(currentInfos) > 1 {
			info = &nestNode{span: span{currentInfos[0].start, currentInfos[len(currentInfos)-1].end}, composite: true, children: currentInfos}
		}
		if op == "" {
			accumulatedInfo = info
		} else {
			accumulatedInfo = &nestNode{span: span{accumulatedInfo.start, info.end}, composite: true, children: []*nestNode{accumulatedInfo, info}}
		}
		currentInfos = nil
		switch op {
		case "&&":
			accumulated = intersection(accumulated, current)
		case "--":
			accumulated = difference(accumulated, current)
		case "~~":
			accumulated = union(difference(accumulated, current), difference(current, accumulated))
		default:
			accumulated = current
		}
		current = nil
	}
	for {
		p.skip()
		if p.pos == len(p.source) {
			return nil, p.errorAt(start, openEnd, "unclosed character class")
		}
		if p.peek() == ']' {
			p.pos++
			apply()
			if neg {
				max := rune(utf8.MaxRune)
				if p.flags&flagUnicode == 0 {
					max = 255
				}
				accumulated = complement(accumulated, max)
				if p.flags&flagUnicode == 0 {
					p.translation(start, p.pos, "pattern can match invalid UTF-8")
				}
			}
			p.classNest = &nestNode{span: span{start, p.pos}, composite: true, children: []*nestNode{accumulatedInfo}}
			return accumulated, nil
		}
		if p.pos+1 < len(p.source) {
			pair := p.source[p.pos : p.pos+2]
			if pair == "&&" || pair == "--" || pair == "~~" {
				apply()
				op = pair
				p.pos += 2
				continue
			}
		}
		if p.peek() == '[' {
			itemStart := p.pos
			if ascii, ok := p.asciiClass(); ok {
				current = union(current, ascii)
				currentInfos = append(currentInfos, &nestNode{span: span{itemStart, p.pos}})
				continue
			}
			nested, err := p.class()
			if err != nil {
				return nil, err
			}
			current = union(current, nested)
			currentInfos = append(currentInfos, p.classNest)
			continue
		}
		primStart := p.pos
		first, literal, err := p.classItem()
		if err != nil {
			return nil, err
		}
		primEnd := p.pos
		itemEnd := primEnd
		p.skip()
		if p.pos == len(p.source) {
			return nil, p.errorAt(start, openEnd, "unclosed character class")
		}
		if p.peek() == '-' {
			save := p.pos
			p.pos++
			p.skip()
			next := p.peek()
			p.pos = save
			if next != ']' && next != '-' {
				p.pos++
				p.skip()
				if p.pos == len(p.source) {
					return nil, p.errorAt(start, openEnd, "unclosed character class")
				}
				secondStart := p.pos
				second, secondLiteral, err := p.classItem()
				if err != nil {
					return nil, err
				}
				if literal == nil {
					return nil, p.errorAt(primStart, primEnd, "invalid range boundary, must be a literal")
				}
				if secondLiteral == nil {
					return nil, p.errorAt(secondStart, p.pos, "invalid range boundary, must be a literal")
				}
				if *literal > *secondLiteral {
					return nil, p.errorAt(primStart, p.pos, "invalid character class range, the start must be <= the end")
				}
				_ = second
				first = runeSet{{*literal, *secondLiteral}}
				itemEnd = p.pos
			}
		}
		if len(first) > 0 && first[0].lo > utf8.MaxRune {
			return nil, p.errorAt(primStart, primEnd, "invalid escape sequence found in character class")
		}
		current = union(current, p.fold(first))
		currentInfos = append(currentInfos, &nestNode{span: span{primStart, itemEnd}})
	}
}
func (p *parser) classItem() (runeSet, *rune, error) {
	if p.peek() == '\\' {
		node, literal, err := p.escape(true)
		if err != nil {
			return nil, nil, err
		}
		return nodeSet(node), literal, nil
	}
	start := p.pos
	r := p.take()
	if p.flags&flagUnicode == 0 && r >= 128 {
		p.translation(start, p.pos, "Unicode not allowed here")
	}
	return runeSet{{r, r}}, &r, nil
}
