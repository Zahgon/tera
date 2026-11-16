package tera

import (
	"sort"
	"strings"
)

type validationReport struct {
	name string
	pos  int
	text string
}

func validationError(a []validationReport) error {
	sort.SliceStable(a, func(i, j int) bool {
		if a[i].name != a[j].name {
			return a[i].name < a[j].name
		}
		return a[i].pos < a[j].pos
	})
	b := []string{}
	for _, r := range a {
		b = append(b, r.text)
	}
	return ErrorMessage(strings.Join(b, "\n\n"))
}
func walkExpr(e *expr, fn func(*expr), nf func(*node)) {
	if e == nil {
		return
	}
	fn(e)
	walkExpr(e.left, fn, nf)
	walkExpr(e.right, fn, nf)
	walkExpr(e.third, fn, nf)
	for _, x := range e.items {
		walkExpr(x, fn, nf)
	}
	for _, p := range e.kwargs {
		walkExpr(p.e, fn, nf)
	}
	for _, p := range e.entries {
		walkExpr(p.value, fn, nf)
	}
	walkNodes(e.body, fn, nf)
}
func walkNodes(a []*node, fn func(*expr), nf func(*node)) {
	for _, n := range a {
		nf(n)
		walkExpr(n.e, fn, nf)
		for _, f := range n.filters {
			walkExpr(f, fn, nf)
		}
		walkNodes(n.body, fn, nf)
		walkNodes(n.other, fn, nf)
	}
}
func (t *Tera) validate(tp *template, components map[string]*componentDef) []validationReport {
	a := []validationReport{}
	report := func(msg string, sp Span) {
		a = append(a, validationReport{tp.name, sp.Start, generateReport(ReportError{Message: msg, Filename: tp.name, Source: tp.source, Span: sp})})
	}
	ef := func(e *expr) {
		kind := ""
		switch e.k {
		case "filter":
			if t.filters[e.name] == nil {
				kind = "filter"
			}
		case "test":
			if t.tests[e.name] == nil {
				kind = "test"
			}
		case "function":
			if e.name != "super" && t.functions[e.name] == nil {
				kind = "function"
			}
		case "component":
			if components[e.name] == nil && tp.components[e.name] == nil {
				kind = "component"
			}
		}
		if kind != "" {
			report("Unknown "+kind+" `"+e.name+"`", e.sp)
		}
	}
	nf := func(n *node) {
		if n.k == "include" && t.resolve(n.name) == nil {
			report("Unknown template `"+n.name+"`", n.sp)
		}
	}
	walkNodes(tp.nodes, ef, nf)
	for _, d := range tp.components {
		walkNodes(d.body, ef, nf)
	}
	if tp.parent != "" {
		for name, b := range tp.blocks {
			if !b.global {
				continue
			}
			parent := t.resolve(tp.parent)
			found := false
			for parent != nil {
				if _, ok := parent.blocks[name]; ok {
					found = true
					break
				}
				if parent.parent == "" {
					break
				}
				parent = t.resolve(parent.parent)
			}
			if !found {
				report("Block `"+name+"` is not defined in any parent template", b.sp)
			}
		}
	}
	return a
}
func (t *Tera) priority(n string) int {
	for i, p := range t.prefixes {
		if strings.HasPrefix(n, p) {
			return i + 1
		}
	}
	return 0
}
func (t *Tera) finalize() error {
	cs := map[string]*componentDef{}
	names := t.GetTemplateNames()
	for _, name := range names {
		tp := t.templates[name]
		parents := []string{}
		cur := tp
		for cur.parent != "" {
			p := t.resolve(cur.parent)
			if p == nil {
				return NewError(KindMissingParent{cur.name, cur.parent})
			}
			cycle := p.name == tp.name
			for _, n := range parents {
				if n == p.name {
					cycle = true
				}
			}
			parents = append(parents, p.name)
			if cycle {
				return NewError(KindCircularExtend{tp.name, parents})
			}
			cur = p
		}
		if e := t.checkIncludes(tp, []string{tp.name}, map[string]bool{}); e != nil {
			return e
		}
		cn := []string{}
		for n := range tp.components {
			cn = append(cn, n)
		}
		sort.Strings(cn)
		for _, n := range cn {
			d := tp.components[n]
			if old, ok := cs[n]; ok {
				a, b := t.priority(tp.name), t.priority(old.tpl.name)
				if a == b {
					return ErrorMessage("Component `" + n + "` is defined in both `" + old.tpl.name + "` and `" + tp.name + "`")
				}
				if a > b {
					continue
				}
			}
			cs[n] = d
		}
	}
	es := []validationReport{}
	for _, n := range names {
		es = append(es, t.validate(t.templates[n], cs)...)
	}
	if len(es) > 0 {
		return validationError(es)
	}
	t.components = cs
	return nil
}
func (t *Tera) checkIncludes(tp *template, stack []string, seen map[string]bool) error {
	a := append([]namedSpan{}, tp.includes...)
	sort.Slice(a, func(i, j int) bool { return a[i].name < a[j].name })
	for _, n := range a {
		p := t.resolve(n.name)
		if p == nil {
			continue
		}
		cycle := false
		for _, n := range stack {
			if n == p.name {
				cycle = true
			}
		}
		if cycle {
			return NewError(KindCircularInclude{p.name, append(append([]string{}, stack...), p.name)})
		}
		if seen[p.name] {
			continue
		}
		if e := t.checkIncludes(p, append(stack, p.name), seen); e != nil {
			return e
		}
		seen[p.name] = true
	}
	return nil
}

// GetTemplateVariables returns the sorted top-level variables referenced by templates and includes.
func (t *Tera) GetTemplateVariables(n string) ([]string, error) {
	tp := t.resolve(n)
	if tp == nil {
		return nil, NewError(KindTemplateNotFound(n))
	}
	vars := map[string]bool{}
	seen := map[string]bool{}
	var visit func(*template)
	visit = func(tp *template) {
		if tp == nil || seen[tp.name] {
			return
		}
		seen[tp.name] = true
		locals := []map[string]bool{{}}
		var expression func(*expr)
		var nodes func([]*node)
		expression = func(e *expr) {
			if e == nil {
				return
			}
			if e.k == "var" && !strings.HasPrefix(e.name, "__tera_") {
				found := false
				for _, m := range locals {
					if m[e.name] {
						found = true
					}
				}
				if !found {
					vars[e.name] = true
				}
			}
			if e.k == "comprehension" {
				expression(e.right)
				m := map[string]bool{e.val: true}
				if e.key != "" {
					m[e.key] = true
				}
				locals = append(locals, m)
				expression(e.third)
				expression(e.left)
				locals = locals[:len(locals)-1]
				return
			}
			expression(e.left)
			expression(e.right)
			expression(e.third)
			for _, x := range e.items {
				expression(x)
			}
			for _, p := range e.kwargs {
				expression(p.e)
			}
			for _, p := range e.entries {
				expression(p.value)
			}
			nodes(e.body)
		}
		nodes = func(a []*node) {
			for _, n := range a {
				expression(n.e)
				for _, f := range n.filters {
					expression(f)
				}
				switch n.k {
				case "set":
					idx := len(locals) - 1
					if n.global {
						idx = 0
					}
					locals[idx][n.name] = true
				case "for":
					m := map[string]bool{n.name: true}
					if n.key != "" {
						m[n.key] = true
					}
					locals = append(locals, m)
					nodes(n.body)
					locals = locals[:len(locals)-1]
					nodes(n.other)
				case "capture":
					nodes(n.body)
					if n.name != "" {
						locals[len(locals)-1][n.name] = true
					}
				case "include":
					visit(t.resolve(n.name))
				default:
					nodes(n.body)
					nodes(n.other)
				}
			}
		}
		nodes(tp.nodes)
		if tp.parent != "" {
			visit(t.resolve(tp.parent))
		}
	}
	visit(tp)
	a := []string{}
	for n := range vars {
		a = append(a, n)
	}
	sort.Strings(a)
	return a, nil
}
