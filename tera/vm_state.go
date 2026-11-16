package tera

import (
	"sort"
	"strings"

	"github.com/Keats/tera/internal/rustrt"
)

type loopScope struct {
	locals             *Context
	keyName, valueName string
	key, value         Value
	index, length      int
	comprehension      bool
}
type blockFrame struct {
	name  string
	level int
}

// State exposes the current rendering scope to filters, functions and tests.
type State struct {
	tera                           *Tera
	context, globals, sets         *Context
	loops                          []loopScope
	includeParent, componentParent *State
	tpl, root                      *template
	autoescape                     bool
	escapeFn                       EscapeFn
	blocks                         []blockFrame
	depth                          int
	captureBlock, captured         string
}

// NewState constructs a state for testing filters and functions.
func NewState(c *Context) *State {
	if c == nil {
		c = NewContext()
	}
	return &State{context: c, sets: NewContext(), autoescape: true, escapeFn: EscapeHTML}
}

// Get retrieves a value from the current scope.
func (s *State) Get(n string) (Value, bool) { v := s.get(n); return v, !v.IsUndefined() }

// StateGet retrieves and converts a value in scope.
func StateGet[T any](s *State, n string, conv func(Value) (T, error)) (T, bool, error) {
	v := s.get(n)
	if v.IsUndefined() {
		var z T
		return z, false, nil
	}
	x, e := conv(v)
	return x, true, e
}

// AutoescapingEnabled reports whether the current output is escaped.
func (s *State) AutoescapingEnabled() bool { return s.autoescape }

// Escape escapes text with the configured escape function.
func (s *State) Escape(input string) (string, error) {
	var b strings.Builder
	fn := s.escapeFn
	if fn == nil {
		fn = EscapeHTML
	}
	if e := fn(input, &b); e != nil {
		return "", ioError(e)
	}
	if msg := rustrt.UTF8Error([]byte(b.String())); msg != "" {
		return "", ErrorMessage("String `" + input + "` could not be converted to UTF-8 after escaping: " + msg)
	}
	return b.String(), nil
}

// EscapeIfNeeded formats and conditionally escapes a value.
func (s *State) EscapeIfNeeded(v Value) (string, error) {
	if v.IsUndefined() {
		return "", ErrorMessage("Tried to escape an undefined value")
	}
	if !s.autoescape || v.IsSafe() {
		return v.String(), nil
	}
	return s.Escape(v.String())
}

// CallFilter calls a filter registered with this rendering state.
func (s *State) CallFilter(n string, v Value, k Kwargs) (Value, error) {
	if s.tera != nil {
		if f, ok := s.tera.filters[n]; ok {
			return f.Call(v, k, s)
		}
	}
	return Undefined(), ErrorMessage("Filter `" + n + "` is not registered")
}
func (s *State) get(n string) Value {
	for i := len(s.loops) - 1; i >= 0; i-- {
		l := s.loops[i]
		if !l.comprehension {
			switch n {
			case "__tera_loop_index":
				return U64(uint64(l.index + 1))
			case "__tera_loop_index0":
				return U64(uint64(l.index))
			case "__tera_loop_first":
				return Bool(l.index == 0)
			case "__tera_loop_last":
				return Bool(l.index+1 == l.length)
			case "__tera_loop_length":
				return U64(uint64(l.length))
			}
		}

		if v, ok := l.locals.Get(n); ok {
			return v
		}
		if n == l.valueName {
			return l.value
		}
		if n == l.keyName && n != "" {
			return l.key
		}

	}
	if v, ok := s.sets.Get(n); ok {
		return v
	}
	if s.includeParent != nil {
		v := s.includeParent.get(n)
		if !v.IsUndefined() {
			return v
		}
	}
	if v, ok := s.context.Get(n); ok {
		return v
	}
	v, _ := s.globals.Get(n)
	return v
}
func (s *State) implicit(n string) Value {
	v := s.get(n)
	if !v.IsUndefined() {
		return v
	}
	if s.componentParent != nil {
		return s.componentParent.implicit(n)
	}
	return Undefined()
}
func (s *State) store(n string, v Value, global bool) {
	if !global && len(s.loops) > 0 {
		s.loops[len(s.loops)-1].locals.InsertValue(n, v)
	} else {
		s.sets.InsertValue(n, v)
	}
}
func (s *State) available() []string {
	m := map[string]bool{}
	for _, c := range []*Context{s.globals, s.context, s.sets} {
		for _, k := range c.keys() {
			m[k] = true
		}
	}
	for _, l := range s.loops {
		for _, k := range l.locals.keys() {
			m[k] = true
		}
	}
	a := []string{}
	for k := range m {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}
func (s *State) dump() Value {
	c := NewContext()
	c.Extend(s.globals)
	c.Extend(s.context)
	c.Extend(s.sets)
	for _, l := range s.loops {
		c.Extend(l.locals)
	}
	return c.value()
}
func (s *State) renderError(msg string, sp Span) error {
	return NewError(KindRenderingError{ReportError{Message: msg, Filename: s.tpl.name, Source: s.tpl.source, Span: sp}})
}
func (s *State) undefinedVar(name string, sp Span) error {
	a := s.available()
	for _, n := range a {
		if n == name {
			return s.renderError("Variable `"+name+"` exists but its value is undefined", sp)
		}
	}
	msg := "Variable `" + name + "` is not defined."
	if len(a) > 0 {
		msg += " Available variables: " + strings.Join(a, ", ")
	}
	return s.renderError(msg, sp)
}
func (s *State) undefinedField(v Value, name string, sp Span) error {
	msg := "Field `" + name + "` is not defined."
	if v.IsMap() {
		a := []string{}
		for _, p := range v.m.Iter() {
			a = append(a, p.Key.String())
		}
		sort.Strings(a)
		if len(a) > 0 {
			msg += " Available fields: " + strings.Join(a, ", ")
		}
	}
	return s.renderError(msg, sp)
}
func addCallNote(e error, s *State, sp Span) error {
	if er, ok := e.(*Error); ok {
		if k, ok := er.Kind.(KindRenderingError); ok {
			k.notes = append(k.notes, reportNote{"called from", s.tpl.name, s.tpl.source, sp})
			er.Kind = k
		}
	}
	return e
}
