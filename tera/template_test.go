// Ports the unit tests of tera/src/template.rs: can_find_parents, detects_circular_extends and
// nested_blocks_are_tracked.

package tera

import (
	"errors"
	"reflect"
	"testing"
)

func TestCanFindParents(t *testing.T) {
	engine := New()
	mustAdd(t, engine, RawTemplate{"a", ""}, RawTemplate{"b", "{% extends 'a' %}"}, RawTemplate{"c", "{% extends 'b' %}"})
	for name, want := range map[string][]string{"a": {}, "b": {"a"}, "c": {"a", "b"}} {
		got := []string{}
		tp := engine.resolve(name)
		for tp.parent != "" {
			tp = engine.resolve(tp.parent)
			got = append([]string{tp.name}, got...)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: expected %v, got %v", name, want, got)
		}
	}
}
func TestDetectsCircularExtends(t *testing.T) {
	for _, templates := range [][]RawTemplate{{{"a", "{% extends 'a' %}"}}, {{"a", "{% extends 'b' %}"}, {"b", "{% extends 'a' %}"}}, {{"a", "{% extends 'b' %}"}, {"b", "{% extends 'c' %}"}, {"c", "{% extends 'b' %}"}}} {
		err := New().AddRawTemplates(templates)
		var e *Error
		if !errors.As(err, &e) {
			t.Fatalf("expected circular error, got %v", err)
		}
		if _, ok := e.Kind.(KindCircularExtend); !ok {
			t.Errorf("expected circular extends, got %v", err)
		}
	}
}
func TestNestedBlocksAreTracked(t *testing.T) {
	tp, err := parseTemplate("mid", `{% block hey %}hi {% block ending %}sincerely{% endblock ending %}{% endblock hey %}`, DefaultDelimiters())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hey", "ending"} {
		if _, ok := tp.blocks[name]; !ok {
			t.Errorf("missing block %s", name)
		}
	}
	// The equivalent validation invariant: the nested block introduces an extension
	// point and does not require a matching block in the parent.
	engine := New()
	mustAdd(t, engine, RawTemplate{"base", "{% block hey %}{% endblock %}"}, RawTemplate{"mid", `{% extends "base" %}{% block hey %}hi {% block ending %}sincerely{% endblock ending %}{% endblock hey %}`})
}
