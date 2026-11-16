// Package regex implements Rust regex syntax and Unicode-aware matching using
// Go's regexp instruction compiler.
package regex

import (
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode/utf8"
)

const assertionBase rune = 0x110000
const boundaryCaptureBase = 1 << 24
const (
	wordBoundary = iota + 1
	notWordBoundary
	wordStart
	wordEnd
	wordStartHalf
	wordEndHalf
	lineStartCRLF
	lineEndCRLF
)

// Regex is an immutable compiled expression, safe for concurrent use.
type Regex struct {
	prog     *syntax.Prog
	names    map[string]int
	captures int
}

// Compile parses a Rust regex, reporting errors in regex::Error's Display form.
func Compile(pattern string) (*Regex, error) {
	p := parser{source: pattern, flags: flagUnicode, names: make(map[string]int), nameSpans: make(map[string]span), nest: make(map[*syntax.Regexp]*nestNode)}
	ast, err := p.expression(false)
	if err != nil {
		return nil, err
	}
	if err := p.checkNesting(p.nest[ast], 0); err != nil {
		return nil, err
	}
	if p.translateErr != nil {
		return nil, p.translateErr
	}
	if ast == nil {
		ast = &syntax.Regexp{Op: syntax.OpEmptyMatch}
	}
	if nfaMemory(ast)+208 > 10485760 {
		return nil, compileSizeError{}
	}
	prog, err := syntax.Compile(ast.Simplify())
	if err != nil {
		return nil, p.errorAt(0, len(pattern), "invalid regular expression")
	}
	return &Regex{prog: prog, names: p.names, captures: p.captures + 1}, nil
}

type compileSizeError struct{}

func (compileSizeError) Error() string { return "Compiled regex exceeds size limit of 10485760 bytes." }

// nfaMemory accounts for the intermediate Thompson states used by Rust's
// compiler. State storage is 32 bytes, with four bytes per union alternative.
// UTF-8 classes use an estimate because their compressed byte trie differs
// from the rune-oriented program used here.
func nfaMemory(r *syntax.Regexp) uint64 {
	const limit = uint64(10485761)
	add := func(a, b uint64) uint64 {
		if a >= limit || b >= limit || a+b >= limit {
			return limit
		}
		return a + b
	}
	mul := func(a uint64, n int) uint64 {
		if n <= 0 || a == 0 {
			return 0
		}
		if a >= limit || uint64(n) > limit/a {
			return limit
		}
		return a * uint64(n)
	}
	switch r.Op {
	case syntax.OpLiteral:
		var n uint64
		for _, c := range r.Rune {
			n += uint64(utf8.RuneLen(c)) * 32
		}
		if n == 0 {
			return 32
		}
		return n
	case syntax.OpCharClass:
		if len(r.Rune) == 2 && r.Rune[0] == r.Rune[1] {
			return uint64(utf8.RuneLen(r.Rune[0])) * 32
		}
		if len(r.Rune) > 0 && r.Rune[len(r.Rune)-1] > 127 {
			return unicodeClassMemory(r.Rune)
		}
		return uint64(64 + len(r.Rune)*4)
	case syntax.OpConcat:
		var n uint64
		for _, sub := range r.Sub {
			n = add(n, nfaMemory(sub))
		}
		return n
	case syntax.OpAlternate:
		var n uint64 = 64 + uint64(len(r.Sub))*4
		for _, sub := range r.Sub {
			n = add(n, nfaMemory(sub))
		}
		return n
	case syntax.OpCapture:
		if r.Cap >= boundaryCaptureBase {
			return 32
		}
		return add(64, nfaMemory(r.Sub[0]))
	case syntax.OpQuest:
		return add(72, nfaMemory(r.Sub[0]))
	case syntax.OpPlus:
		return add(40, nfaMemory(r.Sub[0]))
	case syntax.OpStar:
		return add(40, nfaMemory(r.Sub[0]))
	case syntax.OpRepeat:
		sub := nfaMemory(r.Sub[0])
		minimum := mul(sub, r.Min)
		if r.Min == 0 {
			minimum = 32
		}
		if r.Max == r.Min {
			return minimum
		}
		if r.Max < 0 {
			return add(minimum, add(sub, 40))
		}
		return add(add(minimum, 32), mul(add(sub, 40), r.Max-r.Min))
	default:
		return 32
	}
}

func isWord(r rune, ascii bool) bool {
	if ascii {
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_'
	}
	return inSet(unicodeWord(), r)
}
func assertion(code rune, rs []rune, pos int) bool {
	code -= assertionBase
	ascii := code >= 32
	if ascii {
		code -= 32
	}
	prev, next := rune(-1), rune(-1)
	if pos > 0 {
		prev = rs[pos-1]
	}
	if pos < len(rs) {
		next = rs[pos]
	}
	left, right := isWord(prev, ascii), isWord(next, ascii)
	switch code {
	case wordBoundary:
		return left != right
	case notWordBoundary:
		return left == right
	case wordStart:
		return !left && right
	case wordEnd:
		return left && !right
	case wordStartHalf:
		return !left
	case wordEndHalf:
		return !right
	case lineStartCRLF:
		return pos == 0 || prev == '\n' || prev == '\r' && next != '\n'
	case lineEndCRLF:
		return pos == len(rs) || next == '\r' || next == '\n' && prev != '\r'
	}
	return false
}

type state struct {
	pc   uint32
	pos  int
	caps []int
}
type position struct {
	pc  uint32
	pos int
}

func (r *Regex) find(rs []rune, start int) []int {
	// States that failed at an earlier start also fail at later starts, because
	// assertions inspect the complete subject rather than the start offset.
	seen := make(map[position]bool)
	for begin := start; begin <= len(rs); begin++ {
		caps := make([]int, r.captures*2)
		for i := range caps {
			caps[i] = -1
		}
		caps[0] = begin
		stack := []state{{uint32(r.prog.Start), begin, caps}}
		for len(stack) > 0 {
			st := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for {
				key := position{st.pc, st.pos}
				if seen[key] {
					break
				}
				seen[key] = true
				inst := &r.prog.Inst[st.pc]
				switch inst.Op {
				case syntax.InstFail:
					goto failed
				case syntax.InstMatch:
					st.caps[1] = st.pos
					return st.caps
				case syntax.InstNop:
					st.pc = inst.Out
				case syntax.InstCapture:
					if inst.Arg >= 2*boundaryCaptureBase {
						if inst.Arg%2 == 0 && !assertion(assertionBase+rune(inst.Arg/2-boundaryCaptureBase), rs, st.pos) {
							goto failed
						}
						st.pc = inst.Out
						continue
					}
					copyCaps := append([]int(nil), st.caps...)
					if int(inst.Arg) < len(copyCaps) {
						copyCaps[inst.Arg] = st.pos
					}
					st.caps = copyCaps
					st.pc = inst.Out
				case syntax.InstAlt, syntax.InstAltMatch:
					stack = append(stack, state{inst.Arg, st.pos, st.caps})
					st.pc = inst.Out
				case syntax.InstEmptyWidth:
					prev, next := rune(-1), rune(-1)
					if st.pos > 0 {
						prev = rs[st.pos-1]
					}
					if st.pos < len(rs) {
						next = rs[st.pos]
					}
					if syntax.EmptyOpContext(prev, next)&syntax.EmptyOp(inst.Arg) != syntax.EmptyOp(inst.Arg) {
						goto failed
					}
					st.pc = inst.Out
				case syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
					if st.pos == len(rs) || !inst.MatchRune(rs[st.pos]) {
						goto failed
					}
					st.pos++
					st.pc = inst.Out
				}
			}
		failed:
		}
	}
	return nil
}

// IsMatch reports whether any part of s matches the expression.
func (r *Regex) IsMatch(s string) bool { return r.find([]rune(s), 0) != nil }

// ReplaceAll replaces nonoverlapping matches with Rust's $ capture expansion.
func (r *Regex) ReplaceAll(s, rep string) string {
	rs := []rune(s)
	offsets := make([]int, 0, len(rs)+1)
	for i := range s {
		offsets = append(offsets, i)
	}
	offsets = append(offsets, len(s))
	var out strings.Builder
	search, last, previousEnd := 0, 0, -1
	for search <= len(rs) {
		caps := r.find(rs, search)
		if caps == nil {
			break
		}
		a, b := caps[0], caps[1]
		// An empty match abutting a preceding nonempty match is suppressed.
		if a == b && a == previousEnd {
			search = a + 1
			continue
		}
		out.WriteString(s[offsets[last]:offsets[a]])
		out.WriteString(r.expand(rep, s, offsets, caps))
		last = b
		previousEnd = b
		search = b
		if a == b {
			search++
		}
	}
	out.WriteString(s[offsets[last]:])
	return out.String()
}
func replacementName(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_'
}
func (r *Regex) expand(rep, s string, offsets, caps []int) string {
	var out strings.Builder
	for i := 0; i < len(rep); {
		if rep[i] != '$' {
			out.WriteByte(rep[i])
			i++
			continue
		}
		start := i
		i++
		if i < len(rep) && rep[i] == '$' {
			out.WriteByte('$')
			i++
			continue
		}
		name := ""
		if i < len(rep) && rep[i] == '{' {
			end := strings.IndexByte(rep[i+1:], '}')
			if end >= 0 {
				name = rep[i+1 : i+1+end]
				i += end + 2
			} else {
				out.WriteByte('$')
				continue
			}
		} else {
			begin := i
			for i < len(rep) && replacementName(rep[i]) {
				i++
			}
			if begin == i {
				out.WriteByte('$')
				continue
			}
			name = rep[begin:i]
		}
		group := -1
		if n, e := strconv.ParseUint(name, 10, 32); e == nil {
			group = int(n)
		} else if n, ok := r.names[name]; ok {
			group = n
		}
		if group >= 0 && group < len(caps)/2 && caps[2*group] >= 0 && caps[2*group+1] >= 0 {
			out.WriteString(s[offsets[caps[2*group]]:offsets[caps[2*group+1]]])
		}
		if i <= start {
			i = start + 1
		}
	}
	return out.String()
}

var stripTagsRegex = func() *Regex { r, _ := Compile(`(<!--.*?-->|<[^>]*>)`); return r }()
var spacelessRegex = func() *Regex { r, _ := Compile(`>\s+<`); return r }()

// StripTags removes matches of (<!--.*?-->|<[^>]*>).
func StripTags(s string) string { return stripTagsRegex.ReplaceAll(s, "") }

// Spaceless removes Unicode whitespace between adjacent HTML tags.
func Spaceless(s string) string { return spacelessRegex.ReplaceAll(s, "><") }
