// Ports tera/tests/introspection.rs: test_get_component_definition,
// negative_component_defaults_preserve_value_and_type and test_get_template_variables.

package tera

import (
	"reflect"
	"sort"
	"testing"
)

func TestGetComponentDefinition(t *testing.T) {
	engine := New()
	mustAdd(t, engine, RawTemplate{"components.html", `{% component Button(label: String, size: Integer = 1, variant = "primary", message, ...restant) {"doc": "An alert box", "deprecated": true} %} %}<button>{{ label }}</button>{% endcomponent Button %}`})
	info, ok := engine.GetComponentDefinition("Button")
	if !ok {
		t.Fatal("missing Button")
	}
	rest, ok := info.RestParam()
	if info.Name() != "Button" || len(info.Args()) != 4 || !ok || rest != "restant" {
		t.Fatalf("unexpected info: %+v", info)
	}
	args := map[string]ComponentArg{}
	for _, arg := range info.Args() {
		args[arg.Name()] = arg
	}
	label := args["label"]
	kind, ok := label.ArgType()
	if label.Name() != "label" || !label.IsRequired() || !ok || kind != ComponentString {
		t.Fatalf("unexpected label: %+v", label)
	}
	if _, ok := label.Default(); ok {
		t.Fatal("label has a default")
	}
	for _, tc := range []struct {
		name  string
		kind  ComponentArgType
		value Value
	}{{"size", ComponentInteger, I64(1)}, {"variant", ComponentString, NormalString("primary")}} {
		arg := args[tc.name]
		kind, ok := arg.ArgType()
		v, hasDefault := arg.Default()
		if arg.IsRequired() || !ok || kind != tc.kind || !hasDefault || !v.Equal(tc.value) {
			t.Errorf("unexpected %s argument: %+v", tc.name, arg)
		}
	}
	for k, want := range map[string]Value{"doc": NormalString("An alert box"), "deprecated": Bool(true)} {
		got, ok := info.Metadata().GetString(k)
		if !ok || !got.Equal(want) {
			t.Errorf("metadata %s: expected %v, got %v", k, want, got)
		}
	}
	if _, ok := engine.GetComponentDefinition("DoesNotExist"); ok {
		t.Fatal("unexpected component")
	}
}
func TestNegativeComponentDefaultsPreserveValueAndType(t *testing.T) {
	engine := New()
	mustAdd(t, engine, RawTemplate{"components.html", `{% component Metrics(integer=-1, float=-1.5, positive_array=[1], negative_array=[-1], negative_map={"foo": -1}) %}{% endcomponent Metrics %}`})
	info, ok := engine.GetComponentDefinition("Metrics")
	if !ok {
		t.Fatal("missing Metrics")
	}
	expected := map[string]struct {
		kind  ComponentArgType
		value any
	}{
		"integer": {ComponentInteger, -1}, "float": {ComponentFloat, -1.5}, "positive_array": {ComponentArray, []int{1}}, "negative_array": {ComponentArray, []int{-1}}, "negative_map": {ComponentMap, map[string]int{"foo": -1}},
	}
	for _, arg := range info.Args() {
		kind, ok := arg.ArgType()
		v, hasDefault := arg.Default()
		if !ok || kind != expected[arg.Name()].kind || !hasDefault || !v.Equal(mustValue(t, expected[arg.Name()].value)) {
			t.Errorf("%s: unexpected type/default: %s %v", arg.Name(), kind, v)
		}
	}
}
func TestGetTemplateVariables(t *testing.T) {
	engine := New()
	mustAdd(t, engine,
		RawTemplate{"page.html", `{{ name }} {{ user.name }}
{% set x = 1 %}{{ x }}
{% set a = some_value %}
{{ y }}{% set y = 2 %}
{% if cond %}{% set maybe = 1 %}{% endif %}{{ maybe }}
{% if cond2 %}{% set z = 1 %}{% else %}{% set z = 2 %}{% endif %}{{ z }}
{{ item }}{% for item in items %}{% set_global g = 1 %}{{ item }} {{ loop.index }}{% endfor %}
{{ g }}`},
		RawTemplate{"parent.html", `{% set nav = "main" %}{{ title }} {{ nav }}{% block content %}{% endblock %}`},
		RawTemplate{"child.html", `{% extends "parent.html" %}{% block content %}{{ body }}{% include "inc.html" %}{% endblock %}`},
		RawTemplate{"inc.html", `{{ inc_var }}{% include "inc_nested.html" %}`}, RawTemplate{"inc_nested.html", `{{ inc_nested_var }}`})
	for name, want := range map[string][]string{"page.html": {"name", "user", "some_value", "y", "cond", "cond2", "item", "items"}, "child.html": {"title", "body", "inc_var", "inc_nested_var"}} {
		got, err := engine.GetTemplateVariables(name)
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: expected %v, got %v", name, want, got)
		}
	}
}
