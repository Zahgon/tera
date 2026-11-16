package tera

import (
	"fmt"
	"io"
	"math/big"
	"strings"
)

func pathParts(x *expr) ([]*expr, bool) {
	if x == nil {
		return nil, false
	}
	if x.k == "var" && x.name != "__tera_context" {
		return []*expr{x}, true
	}
	if x.k == "attr" && !x.optional {
		a, ok := pathParts(x.left)
		return append(a, x), ok
	}
	return nil, false
}
func (s *State) path(x *expr, write bool) (Value, error) {
	a, _ := pathParts(x)
	v := s.get(a[0].name)
	if v.IsUndefined() && (len(a) > 1 || write) {
		return Undefined(), s.undefinedVar(a[0].name, a[0].sp)
	}
	for i, e := range a[1:] {
		w := v.getAttr(e.name)
		if w.IsUndefined() && (write || i+1 < len(a)-1) {
			return Undefined(), s.undefinedField(v, e.name, e.sp)
		}
		v = w
	}
	return v, nil
}
func (s *State) eval(x *expr) (Value, error) {
	if x == nil {
		return None(), nil
	}
	if _, ok := pathParts(x); ok {
		return s.path(x, false)
	}
	switch x.k {
	case "const":
		return x.v, nil
	case "var":
		if x.name == "__tera_context" {
			return s.dump(), nil
		}
		return s.get(x.name), nil
	case "attr":
		v, e := s.eval(x.left)
		if e != nil {
			return Undefined(), e
		}
		if x.optional && (v.IsNone() || v.IsUndefined()) {
			return Undefined(), nil
		}
		if v.IsUndefined() {
			return Undefined(), s.renderError("Field `"+x.name+"` is not defined", stackSpan(x.left))
		}
		return v.getAttr(x.name), nil
	case "item":
		v, e := s.eval(x.left)
		if e != nil {
			return Undefined(), e
		}
		k, e := s.eval(x.right)
		if e != nil {
			return Undefined(), e
		}
		if x.optional && (v.IsUndefined() || v.IsNone()) {
			return Undefined(), nil
		}
		if v.IsUndefined() {
			return Undefined(), s.renderError("Cannot index into an undefined value", valueSpan(x.left))
		}
		if k.IsUndefined() {
			return Undefined(), s.renderError("Index expression is undefined", valueSpan(x.right))
		}
		r, e := v.getItem(k)
		if e != nil {
			return Undefined(), s.renderError(e.Error(), valueSpan(x.right))
		}
		return r, nil
	case "slice":
		v, e := s.eval(x.left)
		if e != nil {
			return Undefined(), e
		}
		if x.optional && (v.IsUndefined() || v.IsNone()) {
			return Undefined(), nil
		}
		if v.IsUndefined() {
			return Undefined(), s.renderError("Cannot slice an undefined value", valueSpan(x.left))
		}
		args := []*expr{x.right, x.third, x.items[0]}
		ns := make([]*big.Int, 3)
		for i, a := range args {
			if a == nil {
				continue
			}
			v, e := s.eval(a)
			if e != nil {
				return Undefined(), e
			}
			n := []string{"start", "end", "step"}[i]
			if v.IsNone() {
				continue
			}
			if v.IsUndefined() {
				return Undefined(), s.renderError("Slice "+n+" is undefined", valueSpan(a))
			}
			z, ok := v.AsI128()
			if !ok {
				return Undefined(), s.renderError("Slice "+n+" must be an integer, got `"+v.Name()+"`", valueSpan(a))
			}
			ns[i] = z
		}
		r, e := v.slice(ns[0], ns[1], ns[2])
		if e != nil {
			return Undefined(), s.renderError(e.Error(), valueSpan(x.left))
		}
		return r, nil
	case "unary":
		v, e := s.eval(x.left)
		if e != nil {
			return Undefined(), e
		}
		if x.op == "not" {
			return Bool(!v.IsTruthy()), nil
		}
		r, e := negate(v)
		if e != nil {
			return Undefined(), s.renderError(e.Error(), valueSpan(x.left))
		}
		return r, nil
	case "binary":
		a, e := s.eval(x.left)
		if e != nil {
			return Undefined(), e
		}
		if x.op == "and" && !a.IsTruthy() || x.op == "or" && a.IsTruthy() {
			return a, nil
		}
		b, e := s.eval(x.right)
		if e != nil {
			return Undefined(), e
		}
		if x.op == "and" || x.op == "or" {
			return b, nil
		}
		sp := valueSpan(x.left).expand(valueSpan(x.right))
		switch x.op {
		case "~":
			return NormalString(a.String() + b.String()), nil
		case "==":
			return Bool(a.Equal(b)), nil
		case "!=":
			return Bool(!a.Equal(b)), nil
		case "<", ">", "<=", ">=":
			c, ok := a.PartialCompare(b)
			if !ok {
				return Undefined(), s.renderError("Cannot compare `"+a.Name()+"` with `"+b.Name()+"`", sp)
			}
			return Bool(x.op == "<" && c < 0 || x.op == ">" && c > 0 || x.op == "<=" && c <= 0 || x.op == ">=" && c >= 0), nil
		case "in":
			b, e := b.contains(a)
			if e != nil {
				return Undefined(), s.renderError(e.Error(), valueSpan(x.right))
			}
			return Bool(b), nil
		case "+":
			if !a.IsNumber() || !b.IsNumber() {
				return Undefined(), s.renderError("`+` requires both operands to be numbers, found `"+a.Name()+"` and `"+b.Name()+"`", sp)
			}
		default:
			if !a.IsNumber() {
				return Undefined(), s.renderError("Math operations can only be done on numbers, found `"+a.Name()+"`", valueSpan(x.left))
			}
			if !b.IsNumber() {
				return Undefined(), s.renderError("Math operations can only be done on numbers, found `"+b.Name()+"`", valueSpan(x.right))
			}
		}
		r, e := arithmetic(x.op, a, b)
		if e != nil {
			if strings.Contains(e.Error(), "divide by 0") {
				sp = valueSpan(x.right)
			}
			return Undefined(), s.renderError(e.Error(), sp)
		}
		return r, nil
	case "ternary":
		v, e := s.eval(x.left)
		if e != nil {
			return Undefined(), e
		}
		if v.IsTruthy() {
			return s.eval(x.right)
		}
		return s.eval(x.third)
	case "array":
		a := []Value{}
		for _, x := range x.items {
			spread := x.k == "spread"
			y := x
			if spread {
				y = x.left
			}
			v, e := s.eval(y)
			if e != nil {
				return Undefined(), e
			}
			if spread {
				if !v.IsArray() {
					return Undefined(), s.renderError("Spread operator requires an array, found `"+v.Name()+"`", valueSpan(y))
				}
				a = append(a, v.a...)
			} else {
				a = append(a, v)
			}
		}
		return Array(a), nil
	case "map":
		m, e := s.evalEntries(x.entries)
		if e != nil {
			return Undefined(), e
		}
		return MapValue(m), nil
	case "filter", "test":
		v, e := s.eval(x.left)
		if e != nil {
			return Undefined(), e
		}
		kw, e := s.evalKwargs(x.kwargs)
		if e != nil {
			return Undefined(), e
		}
		return s.callValue(x, v, kw, valueSpan(x.left))
	case "function":
		kw, e := s.evalKwargs(x.kwargs)
		if e != nil {
			return Undefined(), e
		}
		if x.name == "super" {
			if len(s.blocks) == 0 {
				return Undefined(), s.renderError("super() called outside of a block", x.sp)
			}
			f := s.blocks[len(s.blocks)-1]
			line := s.lineage(f.name)
			if f.level+1 >= len(line) {
				return Undefined(), s.renderError("Tried to use super() in the top level block", x.sp)
			}
			var b strings.Builder
			_, e := s.renderBlockLevel(f.name, f.level+1, &b)
			if e != nil {
				return Undefined(), e
			}
			if e := validateRendered(b.String()); e != nil {
				return Undefined(), e
			}
			return SafeString(b.String()), nil
		}
		f := s.tera.functions[x.name]
		if f == nil {
			return Undefined(), s.renderError("Unknown function `"+x.name+"`", x.sp)
		}
		v, e := f.Call(kw, s)
		if e != nil {
			return Undefined(), s.renderError(e.Error(), x.sp)
		}
		return v, nil
	case "component":
		var b strings.Builder
		if e := s.renderComponentExpr(x, &b); e != nil {
			return Undefined(), e
		}
		if e := validateRendered(b.String()); e != nil {
			return Undefined(), e
		}
		v := NormalString(b.String())
		if s.autoescape {
			v = v.MarkSafe()
		}
		return v, nil
	case "comprehension":
		v, e := s.eval(x.right)
		if e != nil {
			return Undefined(), e
		}
		pairs, e := s.iterate(v, x.key, valueSpan(x.right))
		if e != nil {
			return Undefined(), e
		}
		a := []Value{}
		for i, p := range pairs {
			s.loops = append(s.loops, loopScope{locals: NewContext(), keyName: x.key, valueName: x.val, key: p.key, value: p.value, index: i, length: len(pairs), comprehension: true})
			take := true
			if x.third != nil {
				c, er := s.eval(x.third)
				if er != nil {
					s.loops = s.loops[:len(s.loops)-1]
					return Undefined(), er
				}
				take = c.IsTruthy()
			}
			if take {
				c, er := s.eval(x.left)
				if er != nil {
					s.loops = s.loops[:len(s.loops)-1]
					return Undefined(), er
				}
				a = append(a, c)
			}
			s.loops = s.loops[:len(s.loops)-1]
		}
		return Array(a), nil
	}
	return Undefined(), nil
}
func valueSpan(x *expr) Span {
	if x == nil {
		return Span{StartLine: 1, EndLine: 1}
	}
	if a, ok := pathParts(x); ok && len(a) > 1 {
		return a[0].sp
	}
	if x.k == "binary" {
		return valueSpan(x.left).expand(valueSpan(x.right))
	}
	if x.k == "unary" {
		return valueSpan(x.left)
	}
	if x.k == "item" {
		return valueSpan(x.left).expand(valueSpan(x.right))
	}
	if x.k == "slice" {
		return valueSpan(x.left)
	}
	if x.k == "ternary" {
		return valueSpan(x.third)
	}
	return x.sp
}
func (s *State) evalKwargs(a []namedExpr) (Kwargs, error) {
	m := NewMap()
	for _, p := range a {
		v, e := s.eval(p.e)
		if e != nil {
			return Kwargs{}, e
		}
		m.InsertString(p.name, v)
	}
	return NewKwargs(m), nil
}
func (s *State) evalEntries(a []exprEntry) (*Map, error) {
	vals := make([]Value, len(a))
	hasSpread := false
	for i, p := range a {
		v, e := s.eval(p.value)
		if e != nil {
			return nil, e
		}
		vals[i] = v
		if p.spread {
			hasSpread = true
		}
	}
	m := NewMap()
	if !hasSpread {
		for i, p := range a {
			m.Insert(p.key, vals[i])
		}
		return m, nil
	}
	for i := len(a) - 1; i >= 0; i-- {
		p := a[i]
		if p.spread {
			if !vals[i].IsMap() {
				return nil, s.renderError("Spread operator requires a map, found `"+vals[i].Name()+"`", valueSpan(p.value))
			}
			for _, p := range vals[i].m.entries {
				if _, ok := m.Get(p.Key); !ok {
					m.Insert(p.Key, p.Value)
				}
			}
		} else if _, ok := m.Get(p.key); !ok {
			m.Insert(p.key, vals[i])
		}
	}
	return m, nil
}
func (s *State) callValue(x *expr, v Value, k Kwargs, sp Span) (Value, error) {
	var r Value
	var e error
	if x.k == "test" {
		t := s.tera.tests[x.name]
		if t == nil {
			return Undefined(), s.renderError("Unknown test `"+x.name+"`", x.sp)
		}
		b, er := t.Call(v, k, s)
		r, e = Bool(b), er
	} else {
		f := s.tera.filters[x.name]
		if f == nil {
			return Undefined(), s.renderError("Unknown filter `"+x.name+"`", x.sp)
		}
		r, e = f.Call(v, k, s)
	}
	if e != nil {
		if er, ok := e.(*Error); ok {
			if _, ok := er.Kind.(KindInvalidArgument); ok {
				return Undefined(), s.renderError(e.Error(), sp)
			}
		}
		return Undefined(), s.renderError(e.Error(), x.sp)
	}
	return r, nil
}

type iterationPair struct{ key, value Value }

func (s *State) iterate(v Value, key string, sp Span) ([]iterationPair, error) {
	if !v.IsMap() && !v.IsArray() && !v.IsString() && !v.IsBytes() {
		return nil, s.renderError("Iteration not possible on type `"+v.Name()+"`", sp)
	}
	if key != "" && !v.IsMap() {
		return nil, s.renderError("Key/value iteration is not possible on type `"+v.Name()+"`, only on maps.", sp)
	}
	a := []iterationPair{}
	switch v.kind {
	case KindMap:
		for _, p := range v.m.entries {
			a = append(a, iterationPair{p.Key.AsValue(), p.Value})
		}
	case KindArray:
		for _, x := range v.a {
			a = append(a, iterationPair{None(), x})
		}
	case KindString:
		for _, r := range v.s {
			a = append(a, iterationPair{None(), NormalString(string(r))})
		}
	case KindBytes:
		for _, r := range v.bytes {
			a = append(a, iterationPair{None(), U64(uint64(r))})
		}
	}
	return a, nil
}
func (s *State) renderNodes(nodes []*node, b io.Writer) (string, error) {
	for _, n := range nodes {
		switch n.k {
		case "content":
			if e := writeRaw(b, n.text); e != nil {
				return "", e
			}
		case "expression":
			if n.e.k == "component" {
				if e := s.renderComponentExpr(n.e, b); e != nil {
					return "", e
				}
				continue
			}

			var v Value
			var e error
			if _, ok := pathParts(n.e); ok {
				v, e = s.path(n.e, true)
			} else {
				v, e = s.eval(n.e)
			}
			if e != nil {
				return "", e
			}
			if v.IsUndefined() {
				return "", s.renderError("Tried to render a variable that is undefined", valueSpan(n.e))
			}
			if e := s.writeValue(v, b); e != nil {
				return "", e
			}
		case "set":
			v, e := s.eval(n.e)
			if e != nil {
				return "", e
			}
			s.store(n.name, v, n.global)
		case "if":
			v, e := s.eval(n.e)
			if e != nil {
				return "", e
			}
			body := n.other
			if v.IsTruthy() {
				body = n.body
			}
			flow, e := s.renderNodes(body, b)
			if e != nil || flow != "" {
				return flow, e
			}
		case "for":
			v, e := s.eval(n.e)
			if e != nil {
				return "", e
			}
			pairs, e := s.iterate(v, n.key, valueSpan(n.e))
			if e != nil {
				return "", e
			}
			if len(pairs) == 0 {
				flow, e := s.renderNodes(n.other, b)
				if e != nil || flow != "" {
					return flow, e
				}
			}
			for i, p := range pairs {
				s.loops = append(s.loops, loopScope{locals: NewContext(), keyName: n.key, valueName: n.name, key: p.key, value: p.value, index: i, length: len(pairs)})
				flow, e := s.renderNodes(n.body, b)
				s.loops = s.loops[:len(s.loops)-1]
				if e != nil {
					return "", e
				}
				if flow == "break" {
					break
				}
			}
		case "break", "continue":
			return n.k, nil
		case "capture":
			var c strings.Builder
			if _, e := s.renderNodes(n.body, &c); e != nil {
				return "", e
			}
			if e := validateRendered(c.String()); e != nil {
				return "", e
			}
			v := NormalString(c.String())
			if s.autoescape {
				v = v.MarkSafe()
			}
			sp := n.sp
			if len(n.filters) > 0 {
				sp = n.filters[0].sp
			}
			for _, f := range n.filters {
				kw, e := s.evalKwargs(f.kwargs)
				if e != nil {
					return "", e
				}
				v, e = s.callValue(f, v, kw, sp)
				if e != nil {
					return "", e
				}
				sp = f.sp
			}
			if n.name != "" {
				s.store(n.name, v, n.global)
			} else {
				if e := s.writeValue(v, b); e != nil {
					return "", e
				}
			}
		case "include":
			t := s.tera.resolve(n.name)
			if t == nil {
				return "", NewError(KindTemplateNotFound(n.name))
			}
			child := NewState(s.context)
			child.tera = s.tera
			child.tpl = t
			child.root = t
			child.autoescape = s.autoescape
			child.escapeFn = s.escapeFn
			child.includeParent = s
			child.componentParent = s
			child.depth = s.depth
			_, e := child.renderNodes(t.nodes, b)
			if e != nil {
				return "", addCallNote(e, s, n.sp)
			}
		case "block":
			if n.name == s.captureBlock {
				var buf strings.Builder
				if _, e := s.renderBlockLevel(n.name, 0, &buf); e != nil {
					return "", e
				}
				s.captured = buf.String()
			} else if flow, e := s.renderBlockLevel(n.name, 0, b); e != nil || flow != "" {
				return flow, e
			}

		}
	}
	return "", nil
}

type blockBody struct {
	tpl *template
	n   *node
}

func (s *State) lineage(n string) []blockBody {
	a := []blockBody{}
	t := s.root
	for t != nil {
		if b, ok := t.blocks[n]; ok {
			a = append(a, blockBody{t, b})
		}
		if t.parent == "" {
			break
		}
		t = s.tera.resolve(t.parent)
	}
	return a
}
func (s *State) renderBlockLevel(name string, level int, b io.Writer) (string, error) {
	a := s.lineage(name)
	if level >= len(a) {
		return "", ErrorMessage(fmt.Sprintf("Block '%s' has no block lineage in template '%s'. This usually means the template was not properly finalized.", name, s.root.name))
	}
	old := s.tpl
	s.tpl = a[level].tpl
	s.blocks = append(s.blocks, blockFrame{name, level})
	flow, e := s.renderNodes(a[level].n.body, b)
	s.blocks = s.blocks[:len(s.blocks)-1]
	s.tpl = old
	return flow, e
}
func (d *componentDef) buildContext(m *Map, parent *State, body *Value) (*Context, error) {
	c := NewContext()
	rest := NewMap()
	known := map[string]bool{}
	for _, a := range d.info.args {
		known[a.name] = true
	}
	unknown := []string{}
	for _, p := range m.Iter() {
		k, ok := p.Key.AsStr()
		if !ok {
			continue
		}
		if !known[k] {
			if d.info.restParam != "" {
				rest.InsertString(k, p.Value)
			} else {
				unknown = append(unknown, "`"+k+"`")
			}
		}
	}
	if len(unknown) > 0 {
		msg := "Unknown argument(s) " + strings.Join(unknown, ", ") + " in component call."
		if len(d.info.args) > 0 {
			a := []string{}
			for _, arg := range d.info.args {
				a = append(a, "`"+arg.name+"`")
			}
			msg += " Possible argument(s) are: " + strings.Join(a, ", ")
		}
		return nil, ErrorMessage(msg)
	}
	for _, a := range d.info.args {
		v, ok := m.GetString(a.name)
		if !ok && a.implicit {
			v = parent.implicit(a.name)
			ok = !v.IsUndefined()
		}
		if ok {
			if a.argType != "" && !a.argType.matches(v) {
				return nil, ErrorMessage(fmt.Sprintf("Component argument `%s` (type: `%s`) does not match expected type: `%s`", a.name, v.Name(), a.argType))
			}
			c.InsertValue(a.name, v)
		} else if a.defaultValue != nil {
			c.InsertValue(a.name, *a.defaultValue)
		} else {
			typ := ""
			if a.argType != "" {
				typ = " (type: `" + a.argType.AsStr() + "`)"
			}
			if a.implicit {
				return nil, ErrorMessage("Implicit argument `" + a.name + "`" + typ + " not found in context or passed directly.")
			}
			return nil, ErrorMessage("Argument `" + a.name + "`" + typ + " missing.")
		}
	}
	if d.info.restParam != "" {
		c.InsertValue(d.info.restParam, MapValue(rest))
	}
	if body != nil {
		c.InsertValue("body", *body)
	}
	return c, nil
}

func writeRaw(w io.Writer, text string) error {
	e := writeAll(w, []byte(text))
	if e != nil {
		return ioError(e)
	}
	return nil
}
func (s *State) writeValue(v Value, w io.Writer) error {
	if !s.autoescape || v.IsSafe() {
		return writeRaw(w, v.String())
	}
	fn := s.escapeFn
	if fn == nil {
		fn = EscapeHTML
	}
	if e := fn(v.String(), w); e != nil {
		return ioError(e)
	}
	return nil
}

func (s *State) renderComponentExpr(x *expr, w io.Writer) error {
	var body *Value
	if !x.selfClosing {
		var b strings.Builder
		if _, e := s.renderNodes(x.body, &b); e != nil {
			return e
		}
		if e := validateRendered(b.String()); e != nil {
			return e
		}
		v := NormalString(b.String())
		if s.autoescape {
			v = v.MarkSafe()
		}
		body = &v
	}
	m, e := s.evalEntries(x.entries)
	if e != nil {
		return e
	}
	def := s.tera.components[x.name]
	if def == nil {
		def = s.root.components[x.name]
	}
	if def == nil {
		return NewError(KindComponentNotFound(x.name))
	}
	c, e := def.buildContext(m, s, body)
	if e != nil {
		return s.renderError(e.Error(), x.sp)
	}
	if s.depth+1 > 20 {
		return ErrorMessage("Maximum render recursion depth for components exceeded.")
	}
	child := NewState(c)
	child.tera = s.tera
	child.tpl = def.tpl
	child.root = s.root
	child.autoescape = s.autoescape
	child.escapeFn = s.escapeFn
	child.componentParent = s
	child.depth = s.depth + 1
	_, e = child.renderNodes(def.body, w)
	if e != nil {
		return addCallNote(e, s, x.sp)
	}
	return nil
}

// stackSpan is the span the original attaches to an evaluated expression: a subscript result spans from its
// container to the end of the index expression, without the closing bracket.
func stackSpan(x *expr) Span {
	if x.k == "item" && x.left != nil && x.right != nil {
		return stackSpan(x.left).expand(x.right.sp)
	}
	return x.sp
}
