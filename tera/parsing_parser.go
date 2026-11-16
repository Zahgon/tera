package tera

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Keats/tera/internal/rustrt"
)

type bodyContext struct {
	name string
	sp   Span
}
type parser struct {
	tokens                                          []token
	at                                              int
	current                                         Span
	tpl                                             *template
	contexts                                        []bodyContext
	depth, nesting, complexity, dimension, brackets int
	err                                             error
}

func parseTemplate(name, src string, d Delimiters) (*template, error) {
	t := &template{name: name, source: src, blocks: map[string]*node{}, components: map[string]*componentDef{}}
	p := &parser{tokens: lex(src, d), tpl: t}
	t.nodes = p.parseUntil(nil)
	if p.err != nil {
		if e, ok := p.err.(*Error); ok {
			if k, ok := e.Kind.(KindSyntaxError); ok {
				k.Filename = name
				k.Source = src
				e.Kind = k
			}
		}
		return nil, p.err
	}
	return t, nil
}
func (p *parser) peek() token {
	if p.at >= len(p.tokens) {
		return token{k: "eof", sp: p.current}
	}
	t := p.tokens[p.at]
	if t.err != nil && p.err == nil {
		p.err = t.err
	}
	return t
}
func (p *parser) next() token {
	t := p.peek()
	if t.k != "eof" {
		p.at++
		p.current = t.sp
	} else if p.err == nil {
		sp := p.current
		sp.StartLine = sp.EndLine
		sp.StartCol = sp.EndCol
		p.err = syntaxError("Unexpected end of input", sp)
	}
	return t
}
func (p *parser) expect(k, display string) token {
	t := p.next()
	if t.k != k && p.err == nil {
		p.err = syntaxError("Found "+t.display()+" but expected "+display+".", p.current)
	}
	return t
}
func (p *parser) is(k string) bool { return p.peek().k == k }
func (p *parser) id(s string) bool { t := p.peek(); return t.k == "id" && t.s == s }
func (p *parser) fail(s string, sp Span) {
	if p.err == nil {
		p.err = syntaxError(s, sp)
	}
}
func (p *parser) note(s string, sp Span, label string, prev Span) {
	if p.err != nil {
		return
	}
	e := syntaxError(s, sp)
	k := e.Kind.(KindSyntaxError)
	k.notes = append(k.notes, reportNote{label, p.tpl.name, p.tpl.source, prev})
	e.Kind = k
	p.err = e
}
func (p *parser) bump() {
	p.complexity++
	if p.complexity > 256 {
		p.fail("The expression is too complex", p.current)
	}
}
func (p *parser) expression(bp int) *expr {
	if p.err != nil {
		return &expr{k: "const", v: Undefined(), sp: p.current}
	}
	if p.depth == 0 {
		p.complexity = 0
	}
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 40 {
		p.fail("The expression is too complex", p.current)
		return &expr{k: "const", sp: p.current}
	}
	t := p.next()
	sp := t.sp
	var x *expr
	switch t.k {
	case "int", "float", "str", "bool":
		x = &expr{k: "const", v: t.v, sp: sp}
	case "id":
		if t.s == "none" || t.s == "None" || t.s == "null" {
			x = &expr{k: "const", v: None(), sp: sp}
		} else if t.s == "not" {
			x = p.unary(t)
		} else {
			x = p.identifier(t)
		}
	case "-":
		x = p.unary(t)
	case "(":
		x = p.expression(0)
		p.expect(")", ")")
	case "[":
		x = p.array(sp)
	case "{":
		x = p.mapExpr(sp)
	case "<":
		if !p.is("id") {
			p.fail("Found `<` but expected identifier after it for component call. Use `<` for comparisons in proper context.", p.current)
			x = &expr{k: "const", sp: sp}
		} else {
			x = p.componentCall(sp, false)
		}
	default:
		p.fail("Found "+t.display()+" but expected one of: integer, float, string, bool, ident, `-`, `not`, `<`, `{`, `[` or `(`", p.current)
		return &expr{k: "const", sp: sp}
	}
	negated := false
	for p.err == nil {
		t := p.peek()
		op := t.k
		if op == "id" {
			op = t.s
		}
		if op == "[" {
			p.bump()
			x = p.subscript(x)
			continue
		}
		if op == "if" {
			if bp > 0 {
				break
			}
			p.next()
			cond := p.expression(0)
			e := p.expect("id", "else")
			if e.s != "else" {
				p.fail("Found "+e.display()+" but expected else.", p.current)
			}
			other := p.expression(0)
			x = &expr{k: "ternary", left: cond, right: x, third: other, sp: sp.expand(p.current)}
			return x
		}
		if op == "not" {
			if bp > 5 {
				break
			}
			p.next()
			if !p.id("in") {
				p.fail("`not` is only valid here as part of `not in`", p.current)
				break
			}
			negated = true
			continue
		}
		lb, rb := binding(op)
		if lb < bp || lb < 0 {
			break
		}
		p.bump()
		p.next()
		if op == "is" && p.id("not") {
			p.next()
			negated = true
		}
		if op == "|" || op == "is" {
			name := p.expect("id", "identifier")
			kw := []namedExpr{}
			if p.is("(") {
				kw = p.kwargs()
			}
			k := "filter"
			if op == "is" {
				k = "test"
			}
			x = &expr{k: k, name: name.s, left: x, kwargs: kw, sp: name.sp.expand(p.current)}
		} else {
			r := p.expression(rb)
			if op == "~" && r.k == "unary" {
				p.fail("`"+r.op+"` is not allowed after `~`", p.current)
			}
			x = &expr{k: "binary", op: op, left: x, right: r, sp: sp.expand(p.current)}
		}
		if negated {
			x = &expr{k: "unary", op: "not", left: x, sp: sp.expand(p.current)}
			negated = false
		}
	}
	return x
}
func binding(op string) (int, int) {
	switch op {
	case "or":
		return 1, 2
	case "and":
		return 3, 4
	case "in", "is":
		return 5, 6
	case "==", "!=", "<", "<=", ">", ">=":
		return 7, 8
	case "+", "-":
		return 11, 12
	case "*", "/", "%", "~", "//":
		return 13, 14
	case "**":
		return 16, 15
	case "|":
		return 17, 18
	}
	return -1, -1
}
func (p *parser) unary(t token) *expr {
	op := t.k
	if op == "id" {
		op = t.s
	}
	if p.is("-") || p.id("not") {
		p.fail("`-` and `not` cannot be used consecutively.", p.peek().sp)
	}
	bp := 20
	if op == "not" {
		bp = 5
	}
	x := p.expression(bp)
	sp := t.sp.expand(p.current)
	if op == "-" && x.k == "const" {
		if x.v.IsI64() {
			return &expr{k: "const", v: I64(-x.v.i), sp: sp}
		}
		if x.v.IsF64() {
			return &expr{k: "const", v: F64(-x.v.f), sp: sp}
		}
	}
	return &expr{k: "unary", op: op, left: x, sp: sp}
}
func (p *parser) identifier(t token) *expr {
	if p.is("(") {
		kw := p.kwargs()
		return &expr{k: "function", name: t.s, kwargs: kw, sp: t.sp.expand(p.current)}
	}
	x := &expr{k: "var", name: t.s, sp: t.sp}
	for p.err == nil {
		switch p.peek().k {
		case ".", "?.":
			p.bump()
			optional := p.next().k == "?."
			a := p.expect("id", "identifier")
			if t.s == "loop" && p.inLoop() {
				if a.s != "index" && a.s != "index0" && a.s != "first" && a.s != "last" && a.s != "length" {
					p.fail("Found invalid field of `loop`: "+a.s+". Only `index`, `index0`, `first`, `last` and `length` exist.", p.current)
				}
				x = &expr{k: "var", name: "__tera_loop_" + a.s, sp: a.sp}
			} else {
				x = &expr{k: "attr", name: a.s, left: x, sp: a.sp, optional: optional}
			}
		case "[", "?[":
			p.bump()
			x = p.subscript(x)
		case "(":
			p.fail("Function calls are only allowed at the first element of a chain", p.current)
			return x
		default:
			return x
		}
	}
	return x
}
func (p *parser) subscript(x *expr) *expr {
	optional := p.next().k == "?["
	p.brackets++
	defer func() { p.brackets-- }()
	if p.brackets > 4 {
		p.fail("Identifiers can only have up to 4 nested brackets.", p.current)
	}
	var start, end, step *expr
	if !p.is(":") {
		start = p.expression(0)
	}
	slice := false
	if p.is(":") {
		slice = true
		p.next()
		if !p.is(":") && !p.is("]") {
			end = p.expression(0)
		}
		if p.is(":") {
			p.next()
			step = p.expression(0)
		}
	}
	p.expect("]", "]")
	sp := x.sp.expand(p.current)
	if slice {
		return &expr{k: "slice", left: x, right: start, third: end, items: []*expr{step}, optional: optional, sp: sp}
	}
	return &expr{k: "item", left: x, right: start, optional: optional, sp: sp}
}
func (p *parser) kwargs() []namedExpr {
	p.expect("(", "(")
	a := []namedExpr{}
	seen := map[string]Span{}
	for !p.is(")") && p.err == nil {
		if len(a) > 0 {
			p.expect(",", ",")
		}
		if p.is(")") {
			break
		}
		n := p.expect("id", "identifier")
		if sp, ok := seen[n.s]; ok {
			p.note("Keyword argument `"+n.s+"` is defined more than once", n.sp, "first defined here", sp)
		}
		seen[n.s] = n.sp
		p.expect("=", "=")
		a = append(a, namedExpr{n.s, p.expression(0)})
	}
	p.expect(")", ")")
	sort.Slice(a, func(i, j int) bool { return a[i].name < a[j].name })
	return a
}
func (p *parser) mapExpr(sp Span) *expr {
	es := []exprEntry{}
	literal := true
	for !p.is("}") && p.err == nil {
		if len(es) > 0 {
			p.expect(",", ",")
		}
		if p.is("}") {
			break
		}
		if p.is("...") {
			p.next()
			es = append(es, exprEntry{value: p.expression(0), spread: true})
			literal = false
			continue
		}
		t := p.next()
		if t.k != "str" && t.k != "int" && t.k != "bool" {
			p.fail("Found "+t.display()+" but expected `...`, a string, an integer or a bool.", t.sp)
		}
		key, _ := KeyFromValue(t.v)
		p.expect(":", ":")
		v := p.expression(0)
		if v.k != "const" {
			literal = false
		}
		es = append(es, exprEntry{key: key, value: v})
	}
	p.expect("}", "}")
	sp = sp.expand(p.current)
	if literal {
		m := NewMap()
		for _, e := range es {
			m.Insert(e.key, e.value.v)
		}
		return &expr{k: "const", v: MapValue(m), sp: sp}
	}
	return &expr{k: "map", entries: es, sp: sp}
}
func (p *parser) array(sp Span) *expr {
	p.dimension++
	defer func() { p.dimension-- }()
	if p.dimension > 2 {
		p.fail("Arrays can have a maximum of 2 dimensions.", sp)
	}
	items := []*expr{}
	literal := true
	for !p.is("]") && p.err == nil {
		if len(items) > 0 {
			p.expect(",", ",")
		}
		if p.is("]") {
			break
		}
		spread := p.is("...")
		if spread {
			p.next()
		}
		x := p.expression(0)
		if spread {
			x = &expr{k: "spread", left: x, sp: x.sp}
		}
		if len(items) == 0 && p.id("for") {
			p.dimension--
			result := p.comprehension(x, sp)
			p.dimension++
			return result
		}
		items = append(items, x)
		if x.k != "const" {
			literal = false
		}
	}
	p.expect("]", "]")
	sp = sp.expand(p.current)
	if literal {
		a := []Value{}
		for _, x := range items {
			a = append(a, x.v)
		}
		return &expr{k: "const", v: Array(a), sp: sp}
	}
	return &expr{k: "array", items: items, sp: sp}
}

var reserved = map[string]bool{"true": true, "True": true, "false": true, "False": true, "loop": true, "self": true, "and": true, "or": true, "not": true, "is": true, "in": true, "continue": true, "break": true, "none": true, "None": true, "null": true}

func (p *parser) loopNames(kind string) (string, string) {
	n := p.expect("id", "identifier")
	if reserved[n.s] {
		p.fail(n.s+" is a reserved keyword of Tera, it cannot be used as a "+kind+" variable.", p.current)
	}
	key, val := "", n.s
	if p.is(",") {
		p.next()
		key = val
		n = p.expect("id", "identifier")
		val = n.s
		if reserved[val] {
			p.fail(val+" is a reserved keyword of Tera, it cannot be used as a "+kind+" variable.", p.current)
		}
	}
	t := p.expect("id", "in")
	if t.s != "in" {
		p.fail("Found "+t.display()+" but expected in.", p.current)
	}
	return key, val
}
func (p *parser) comprehension(x *expr, sp Span) *expr {
	p.next()
	k, v := p.loopNames("list comprehension")
	target := p.expression(1)
	var cond *expr
	if p.id("if") {
		p.next()
		cond = p.expression(1)
	}
	if p.id("for") {
		p.next()
		p.fail("List comprehensions support only a single `for` clause.", p.current)
	}
	p.expect("]", "]")
	return &expr{k: "comprehension", left: x, right: target, third: cond, key: k, val: v, sp: sp.expand(p.current)}
}
func (p *parser) dottedName() string {
	n := p.expect("id", "identifier").s
	for p.is(".") && p.err == nil {
		p.next()
		n += "." + p.expect("id", "identifier").s
	}
	return n
}
func (p *parser) componentCall(sp Span, body bool) *expr {
	name := p.dottedName()
	es := []exprEntry{}
	for p.err == nil {
		if p.is("id") {
			n := p.next()
			var x *expr
			if p.is("=") {
				p.next()
				if p.is("str") {
					t := p.next()
					x = &expr{k: "const", v: t.v, sp: p.current}
				} else if p.is("{") {
					p.next()
					x = p.expression(0)
					p.expect("}", "}")
				} else {
					p.fail("Expected \"string\" or {expression} but found "+p.peek().display(), p.peek().sp)
					break
				}
			} else {
				x = &expr{k: "var", name: n.s, sp: n.sp}
			}
			es = append(es, exprEntry{key: StringKey(n.s), value: x})
		} else if p.is("{") {
			p.next()
			p.expect("...", "...")
			x := p.expression(0)
			p.expect("}", "}")
			es = append(es, exprEntry{value: x, spread: true})
		} else {
			break
		}
	}
	x := &expr{k: "component", name: name, entries: es, sp: sp}
	if !body {
		if p.is("/") {
			p.next()
			p.expect(">", ">")
			x.selfClosing = true
		} else if p.is(">") {
			p.next()
			p.fail("Components with body content must use tag syntax: {% <component()> %}...{% </component> %}", p.current)
		} else {
			p.fail("Found "+p.peek().display()+" but expected `/` or `>`", p.current)
		}
	} else {
		p.expect(">", ">")
		p.expect("te", "%}")
		p.contexts = append(p.contexts, bodyContext{"component body", sp})
		x.body = p.parseUntil([]string{"</"})
		p.contexts = p.contexts[:len(p.contexts)-1]
		if p.is("eof") {
			p.fail("Unclosed component '"+name+"'", p.current)
			return x
		}
		p.expect("</", "</")
		end := p.dottedName()
		if end != name {
			p.fail("Closing tag '"+end+"' doesn't match opening tag '"+name+"'", p.current)
		}
		p.expect(">", ">")
	}
	x.sp = sp.expand(p.current)
	return x
}
func (p *parser) inLoop() bool {
	for _, c := range p.contexts {
		if c.name == "for loop" {
			return true
		}
	}
	return false
}
func (p *parser) parseUntil(ends []string) []*node {
	p.nesting++
	defer func() { p.nesting-- }()
	if p.nesting > 40 {
		p.fail("The template nesting is too deep", p.current)
		return nil
	}
	nodes := []*node{}
	for p.peek().k != "eof" && p.err == nil {
		t := p.next()
		child := p.tpl.parent != "" && len(p.contexts) == 0
		switch t.k {
		case "content":
			if t.s != "" {
				if child && rustrt.Trim(t.s) != "" {
					p.unreachable(t.sp)
				}
				nodes = append(nodes, &node{k: "content", text: t.s, sp: t.sp})
			}
		case "vs":
			x := p.expression(0)
			p.expect("ve", "}}")
			if child {
				p.unreachable(t.sp.expand(p.current))
			}
			nodes = append(nodes, &node{k: "expression", e: x, sp: x.sp})
		case "ts":
			next := p.peek()
			key := next.k
			if key == "id" {
				key = next.s
			}
			for _, e := range ends {
				if key == e {
					return nodes
				}
			}
			first := true
			for _, n := range nodes {
				if n.k != "content" || rustrt.Trim(n.text) != "" {
					first = false
				}
			}
			n := p.tag(first)
			p.expect("te", "%}")
			if n != nil {
				if child && n.k != "block" {
					p.unreachable(t.sp.expand(p.current))
				}
				nodes = append(nodes, n)
			}
		default:
			p.fail("Unexpected token when parsing: "+t.debug(), t.sp)
		}
	}
	return nodes
}
func (p *parser) unreachable(sp Span) {
	p.fail("This content would be ignored: only `extends`, `block` and `component` are allowed at the top level of a child template.", sp)
}
func (p *parser) tag(first bool) *node {
	t := p.next()
	if t.k == "<" {
		x := p.componentCall(t.sp, true)
		return &node{k: "expression", e: x, sp: x.sp}
	}
	if t.k != "id" {
		p.fail("Unknown tag", p.current)
		return nil
	}
	switch t.s {
	case "include":
		n := p.expect("str", "string")
		p.tpl.includes = append(p.tpl.includes, namedSpan{n.s, n.sp})
		return &node{k: "include", name: n.s, sp: n.sp}
	case "extends":
		n := p.expect("str", "string")
		if p.tpl.parent != "" {
			p.fail("Template is already extending `"+p.tpl.parent+"`", p.current)
		}
		if !first {
			p.fail("`extends` needs to be the first tag of the template", p.current)
		}
		if len(p.contexts) > 0 {
			c := p.contexts[len(p.contexts)-1]
			p.note("`extends` cannot be nested in other tags.", p.current, "the "+c.name+" starts here", c.sp)
		}
		p.tpl.parent = n.s
		return nil
	case "set", "set_global":
		return p.set(t.s == "set_global", t.sp)
	case "if":
		return p.ifNode(t.sp)
	case "for":
		k, v := p.loopNames("loop")
		x := p.expression(0)
		p.expect("te", "%}")
		p.contexts = append(p.contexts, bodyContext{"for loop", t.sp})
		body := p.parseUntil([]string{"endfor", "else"})
		p.contexts = p.contexts[:len(p.contexts)-1]
		var other []*node
		if p.id("else") {
			p.next()
			p.expect("te", "%}")
			other = p.parseUntil([]string{"endfor"})
		}
		p.next()
		return &node{k: "for", key: k, name: v, e: x, body: body, other: other, sp: x.sp}
	case "block":
		for _, c := range p.contexts {
			if c.name != "block" && c.name != "if statement" && c.name != "filter section" && c.name != "`set` block" && c.name != "component body" {
				p.note("Blocks cannot be written inside a "+c.name+".", p.current, "the "+c.name+" starts here", c.sp)
				break
			}
		}
		n := p.expect("id", "identifier")
		if _, ok := p.tpl.blocks[n.s]; ok {
			p.fail("Template already contains a block named `"+n.s+"`", p.current)
		}
		topLevel := true
		for _, c := range p.contexts {
			if c.name == "block" {
				topLevel = false
			}
		}
		nd := &node{k: "block", name: n.s, sp: n.sp, global: topLevel}
		p.tpl.blocks[n.s] = nd
		p.expect("te", "%}")
		p.contexts = append(p.contexts, bodyContext{"block", t.sp})
		nd.body = p.parseUntil([]string{"endblock"})
		p.next()
		if p.is("id") {
			e := p.next()
			if e.s != n.s {
				p.fail("block was named `"+n.s+"` in the opening tag, found `"+e.s+"` as name in the end tag", p.current)
			}
		}
		p.contexts = p.contexts[:len(p.contexts)-1]
		return nd
	case "filter":
		fs := []*expr{}
		for p.err == nil {
			fs = append(fs, p.filterOnly())
			if !p.is("|") {
				break
			}
			p.next()
		}
		p.expect("te", "%}")
		p.contexts = append(p.contexts, bodyContext{"filter section", t.sp})
		b := p.parseUntil([]string{"endfilter"})
		p.next()
		p.contexts = p.contexts[:len(p.contexts)-1]
		return &node{k: "capture", body: b, filters: fs, sp: t.sp}
	case "component":
		p.componentDefinition(t.sp)
		return nil
	case "break", "continue":
		inLoop := false
		for i := len(p.contexts) - 1; i >= 0; i-- {
			c := p.contexts[i]
			if c.name == "for loop" {
				inLoop = true
				break
			}
			if c.name == "filter section" || c.name == "`set` block" || c.name == "component body" {
				p.note("`"+t.s+"` cannot be used inside a "+c.name, p.current, "the "+c.name+" starts here", c.sp)
				return nil
			}
		}
		if !inLoop {
			p.fail(t.s+" can only be used in a for loop", p.current)
		}
		return &node{k: t.s, sp: t.sp}
	default:
		p.fail("Unknown tag", p.current)
	}
	return nil
}
func (p *parser) ifNode(sp Span) *node {
	x := p.expression(0)
	p.expect("te", "%}")
	p.contexts = append(p.contexts, bodyContext{"if statement", sp})
	b := p.parseUntil([]string{"endif", "else", "elif"})
	var other []*node
	if p.id("elif") {
		t := p.next()
		other = []*node{p.ifNode(t.sp)}
		p.contexts = p.contexts[:len(p.contexts)-1]
		return &node{k: "if", e: x, body: b, other: other, sp: sp}
	} else if p.id("else") {
		p.next()
		p.expect("te", "%}")
		other = p.parseUntil([]string{"endif"})
	}
	p.contexts = p.contexts[:len(p.contexts)-1]
	if p.id("endif") {
		p.next()
	} else if p.err == nil {
		p.next()
	}
	return &node{k: "if", e: x, body: b, other: other, sp: sp}
}
func (p *parser) filterOnly() *expr {
	n := p.expect("id", "identifier")
	kw := []namedExpr{}
	if p.is("(") {
		kw = p.kwargs()
	}
	return &expr{k: "filter", name: n.s, kwargs: kw, sp: n.sp.expand(p.current)}
}
func (p *parser) set(global bool, sp Span) *node {
	n := p.expect("id", "identifier")
	if reserved[n.s] {
		p.fail(n.s+" is a reserved keyword of Tera, it cannot be assigned to.", p.current)
	}
	if p.is("=") {
		p.next()
		x := p.expression(0)
		return &node{k: "set", name: n.s, e: x, global: global, sp: x.sp}
	}
	if !p.is("|") && !p.is("te") {
		p.fail("Invalid syntax for `set`: expecting an `=` followed by an expression or a `%}}`", p.current)
		return nil
	}
	fs := []*expr{}
	for p.is("|") && p.err == nil {
		p.next()
		fs = append(fs, p.filterOnly())
	}
	p.expect("te", "%}")
	p.contexts = append(p.contexts, bodyContext{"`set` block", sp})
	b := p.parseUntil([]string{"endset"})
	p.contexts = p.contexts[:len(p.contexts)-1]
	p.next()
	return &node{k: "capture", name: n.s, body: b, filters: fs, global: global, sp: sp}
}
func (p *parser) componentDefinition(sp Span) {
	if len(p.contexts) > 0 {
		c := p.contexts[len(p.contexts)-1]
		p.note("Component definitions cannot be written in another tag.", p.current, "the "+c.name+" starts here", c.sp)
	}
	name := p.dottedName()
	nameSpan := p.current
	if prev, ok := p.tpl.components[name]; ok {
		p.note("Template already contains a component named `"+name+"`", nameSpan, "first defined here", prev.sp)
	}
	def := &componentDef{info: ComponentInfo{name: name, metadata: NewMap()}, tpl: p.tpl, sp: nameSpan}
	p.tpl.components[name] = def
	p.expect("(", "(")
	seen := map[string]Span{}
	for !p.is(")") && p.err == nil {
		if len(def.info.args) > 0 {
			p.expect(",", ",")
		}
		if p.is(")") {
			break
		}
		if p.is("...") {
			p.next()
			n := p.expect("id", "identifier")
			if n.s == "body" {
				p.fail("The name `body` is reserved for component body content.", n.sp)
			}
			if _, ok := seen[n.s]; ok {
				p.fail("Rest parameter `"+n.s+"` conflicts with an existing parameter.", n.sp)
			}
			def.info.restParam = n.s
			if !p.is(")") {
				p.fail("Rest parameter must be the last parameter in component definition.", p.current)
			}
			break
		}
		implicit := false
		if p.is("@") {
			p.next()
			implicit = true
		}
		n := p.expect("id", "identifier")
		if n.s == "body" {
			p.fail("The name `body` is reserved for component body content.", n.sp)
		}
		if prev, ok := seen[n.s]; ok {
			p.note("Component argument `"+n.s+"` is defined more than once", n.sp, "first defined here", prev)
		}
		seen[n.s] = n.sp
		a := ComponentArg{name: n.s, implicit: implicit}
		if p.is(":") {
			p.next()
			t := p.next()
			if t.k != "id" {
				p.fail("Found "+t.display()+" but the only types allowed are: string, bool, integer, float, number, bytes, array and map", t.sp)
			} else {
				switch strings.ToLower(t.s) {
				case "string", "bool", "integer", "float", "number", "bytes", "array", "map":
					a.argType = componentType(t.s)
				default:
					p.fail(fmt.Sprintf("Found %s but the only types allowed are: string, bool, integer, float, number, array, map and bytes", t.s), t.sp)
				}
			}
		}
		if p.is("=") {
			p.next()
			t := p.peek()
			if t.k == "-" {
				p.next()
				t = p.peek()
				if t.k != "int" && t.k != "float" {
					p.fail("Found "+t.display()+" but `-` can only be followed by an integer or a float", t.sp)
				}
				p.at--
				p.current = p.tokens[p.at].sp
			}
			x := p.expression(18)
			if x.k != "const" {
				what := "component default arguments can only be one of: string, bool, integer, float, array or map"
				if t.k == "[" {
					p.fail("Invalid default argument: this array should only contain literal values.", x.sp)
				} else if t.k == "{" {
					p.fail("Invalid default argument: this map should only contain literal values.", x.sp)
				} else {
					p.fail("Found "+t.display()+" but "+what, t.sp)
				}
			}
			v := x.v
			a.defaultValue = &v
			if a.argType == "" {
				a.argType = typeFromValue(v)
			}
			if a.argType != "" && !v.IsNone() && !a.argType.matches(v) {
				typ := typeFromValue(v).AsStr()
				if typ == "" {
					typ = v.Name()
				}
				p.fail("Default value for `"+n.s+"` has type `"+typ+"` but the argument has type `"+a.argType.AsStr()+"`", p.current)
			}
		}
		def.info.args = append(def.info.args, a)
	}
	p.expect(")", ")")
	sort.Slice(def.info.args, func(i, j int) bool { return def.info.args[i].name < def.info.args[j].name })
	if p.is("{") {
		t := p.next()
		x := p.mapExpr(t.sp)
		if x.k != "const" {
			p.fail("Invalid component metadata: this map should only contain literal values.", x.sp)
		} else if x.v.IsMap() {
			es := x.v.m.Iter()
			sort.SliceStable(es, func(i, j int) bool { return es[i].Key.String() < es[j].Key.String() })
			for _, e := range es {
				def.info.metadata.InsertString(e.Key.String(), e.Value)
			}
		}
	}
	p.expect("te", "%}")
	p.contexts = append(p.contexts, bodyContext{"component definition", sp})
	def.body = p.parseUntil([]string{"endcomponent"})
	p.next()
	if p.is("id") {
		end := p.dottedName()
		if end != name {
			p.fail("component was named `"+name+"` in the opening tag, found `"+end+"` as name in the end tag", p.current)
		}
	}
	p.contexts = p.contexts[:len(p.contexts)-1]
}
