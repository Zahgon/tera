package tera

import "sort"

// Context holds named values for rendering.
type Context struct{ data map[string]Value }

// NewContext constructs an empty context.
func NewContext() *Context { return &Context{data: map[string]Value{}} }

// ContextFromValue converts a map into a context with stringified keys.
func ContextFromValue(v Value) (*Context, error) {
	if !v.IsMap() {
		return nil, ErrorMessage("from_serialize requires a struct or map, got " + v.Name())
	}
	c := NewContext()
	for _, p := range v.m.entries {
		c.data[p.Key.String()] = p.Value
	}
	return c, nil
}

// Insert converts v like ValueFrom and stores it under k. It returns the conversion error, leaving the context
// unchanged, when v cannot be represented as a Value.
func (c *Context) Insert(k string, v any) error {
	x, e := ValueFrom(v)
	if e != nil {
		return e
	}
	c.InsertValue(k, x)
	return nil
}

// InsertValue inserts a preconstructed value.
func (c *Context) InsertValue(k string, v Value) {
	if c.data == nil {
		c.data = map[string]Value{}
	}
	c.data[k] = v
}

// Remove removes and returns a named value.
func (c *Context) Remove(k string) (Value, bool) { v, ok := c.Get(k); delete(c.data, k); return v, ok }

// Extend inserts the values of another context.
func (c *Context) Extend(o *Context) {
	if o != nil {
		for k, v := range o.data {
			c.InsertValue(k, v)
		}
	}
}

// ContainsKey checks for a named value.
func (c *Context) ContainsKey(k string) bool { _, ok := c.Get(k); return ok }

// Get retrieves a named value.
func (c *Context) Get(k string) (Value, bool) {
	if c == nil {
		return Undefined(), false
	}
	v, ok := c.data[k]
	return v, ok
}
func (c *Context) keys() []string {
	a := []string{}
	if c != nil {
		for k := range c.data {
			a = append(a, k)
		}
	}
	sort.Strings(a)
	return a
}
func (c *Context) clone() *Context { x := NewContext(); x.Extend(c); return x }
func (c *Context) value() Value {
	m := NewMap()
	for _, k := range c.keys() {
		m.InsertString(k, c.data[k])
	}
	return MapValue(m)
}
