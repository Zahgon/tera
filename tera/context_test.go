// Ports the unit tests of tera/src/context.rs: context_macro_builder and context_macro_can_capture_local_name
// (TestContextBuilder: Go has no macros, so the context is built with NewContext and Insert), context_tests
// (TestContext), context_from_serialize and context_from_serialize_non_map_fails.

package tera

import "testing"

func testContext(values map[string]any) *Context {
	c := NewContext()
	for k, v := range values {
		c.Insert(k, v)
	}
	return c
}
func mustValue(t testing.TB, v any) Value {
	t.Helper()
	x, err := ValueFrom(v)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
func mustAdd(t testing.TB, engine *Tera, templates ...RawTemplate) {
	t.Helper()
	if err := engine.AddRawTemplates(templates); err != nil {
		t.Fatal(err)
	}
}
func checkRender(t testing.TB, engine *Tera, name string, c *Context, want string) {
	t.Helper()
	got, err := engine.Render(name, c)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
func TestContextBuilder(t *testing.T) {
	// Go uses ordinary construction and insertion in place of Rust's context! macro.
	foo, con := "Bar", 69
	for _, c := range []*Context{testContext(map[string]any{"foo": "Bar", "con": 69}), testContext(map[string]any{"foo": foo, "con": con})} {
		right := NewContext()
		right.Insert("foo", "Bar")
		right.Insert("con", 69)
		if !c.value().Equal(right.value()) {
			t.Fatal("context builders differ")
		}
	}
}
func TestContext(t *testing.T) {
	c := testContext(map[string]any{"name": "John Doe", "age": 42})
	if !c.ContainsKey("age") {
		t.Fatal("age missing")
	}
	v, ok := c.Get("age")
	if !ok || !v.Equal(I64(42)) {
		t.Fatal("age must be 42")
	}
}
func TestContextFromSerialize(t *testing.T) {
	person := struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}{"Alice", 30}
	c, err := ContextFromValue(mustValue(t, person))
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]Value{"name": NormalString("Alice"), "age": I64(30)} {
		v, ok := c.Get(k)
		if !ok || !v.Equal(want) {
			t.Errorf("%s: expected %v, got %v", k, want, v)
		}
	}
}
func TestContextFromSerializeNonMapFails(t *testing.T) {
	if _, err := ContextFromValue(I64(42)); err == nil {
		t.Fatal("expected error")
	}
}
