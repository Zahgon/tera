package tera

import (
	"bytes"
	"strings"
	"testing"
)

func TestVarAccessBySquareBrackets(t *testing.T) {
	m := map[string]string{"true": "yes", "false": "no", "with space": "works", "with/slash": "works"}
	c := testContext(map[string]any{"var": map[string]any{"a": "hi", "b": "i_am_actually_b", "c": []string{"fred"}}, "zero": 0, "a": "b", "map": m, "deep_map": map[string]any{"inner_map": m}, "bool_vec": []string{"true", "false"}})
	for _, tc := range [][2]string{{`{{var.a}}`, "hi"}, {`{{var['a']}}`, "hi"}, {`{{var["a"]}}`, "hi"}, {`{{var['c'][0]}}`, "fred"}, {`{{var['c'][zero]}}`, "fred"}, {`{{var[a]}}`, "i_am_actually_b"}, {`{{map['with space']}}`, "works"}, {`{{map['with/slash']}}`, "works"}, {`{{deep_map['inner_map'][bool_vec[zero]]}}`, "yes"}} {
		got, err := OneOff(tc[0], c, true)
		if err != nil {
			t.Error(err)
		} else if got != tc[1] {
			t.Errorf("%s: expected %q, got %q", tc[0], tc[1], got)
		}
	}
}
func TestCanGetValueIfKeyContainsPeriod(t *testing.T) {
	c := testContext(map[string]any{"name": "Mt. Robson Provincial Park", "tag_info": map[string]string{"Mt. Robson Provincial Park": "hello"}})
	got, err := OneOff(`{{ tag_info[name] }}`, c, true)
	if err != nil || got != "hello" {
		t.Fatalf("expected hello, got %q, %v", got, err)
	}
}
func TestVarAccessByLoopIndex(t *testing.T) {
	for _, body := range []string{`{% set i = loop.index - 1 %}{{ ics[i] }}`, `{{ ics[loop.index0] }}`} {
		_, err := OneOff(`{% set ics = ["fa-rocket","fa-paper-plane","fa-diamond","fa-signal"] %}{% for a in ics %}`+body+`{% endfor %}`, NewContext(), true)
		if err != nil {
			t.Error(err)
		}
	}
}
func TestRenderingInclude(t *testing.T) {
	engine := New()
	mustAdd(t, engine, RawTemplate{"world", `{% set a = "world" %}[Include => (a={{a}}, name={{ name }}, b={{ b }})]`}, RawTemplate{"hello", `<h1>Hello {% set b = 1 %} {% include "world" %} {% if a %}shouldfail{% endif %}name={{name}}</h1>`})
	checkRender(t, engine, "hello", testContext(map[string]any{"name": "Bob"}), "<h1>Hello  [Include => (a=world, name=Bob, b=1)] name=Bob</h1>")
}
func realisticBenchmark(t testing.TB) (*Tera, *Context) {
	t.Helper()
	engine := New()
	var templates []RawTemplate
	for _, name := range []string{"index.html", "components.html", "page.html"} {
		templates = append(templates, RawTemplate{name, readTestFile(t, "testdata/benches/realistic/"+name)})
	}
	mustAdd(t, engine, templates...)
	items := make([]string, 20)
	for i := range items {
		items[i] = "Hello world"
	}
	c := testContext(map[string]any{"base_url": "https://tera.netlify.app/", "description": "Some description", "content": "<a>Some HTML</a>", "title": "Tera", "items": items, "show_ad": true})
	return engine, c
}
func TestRenderingRealisticBenchmark(t *testing.T) {
	engine, c := realisticBenchmark(t)
	got, err := engine.Render("page.html", c)
	if err != nil {
		t.Fatal(err)
	}
	_, want := readSnapshot(t, "testdata/snapshots/basic__rendering_realistic_benchmark.snap")
	assertSnapshot(t, got, want)
}
func TestCanRenderToWrite(t *testing.T) {
	engine := New()
	mustAdd(t, engine, RawTemplate{"hello", "Hello {{name}}"})
	var out bytes.Buffer
	if err := engine.RenderTo("hello", testContext(map[string]any{"name": "Bob"}), &out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "Hello Bob"; got != want {
		t.Fatalf("RenderTo wrote %q, want %q", got, want)
	}
}
func TestRenderStrCanUseLocallyDefinedComponent(t *testing.T) {
	got, err := New().RenderStr(`{% component greet(name="world") %}Hello {{name}}!{% endcomponent greet %}{{<greet name="Bob"/>}}`, NewContext(), false)
	if err != nil || got != "Hello Bob!" {
		t.Fatalf("expected Hello Bob!, got %q, %v", got, err)
	}
}

type boldTestFilter struct{}

func (boldTestFilter) Call(v Value, _ Kwargs, _ *State) (Value, error) {
	return NormalString("<b>" + v.String() + "</b>"), nil
}
func (boldTestFilter) IsSafe() bool { return true }

type safeHTMLTestFunction struct{}

func (safeHTMLTestFunction) Call(_ Kwargs, _ *State) (Value, error) {
	return NormalString("<em>hello</em>"), nil
}
func (safeHTMLTestFunction) IsSafe() bool { return true }
func TestFilterIsSafeSkipsAutoescape(t *testing.T) {
	engine := New()
	engine.AutoescapeOn([]string{".html"})
	engine.RegisterFilter("bold", boldTestFilter{})
	mustAdd(t, engine, RawTemplate{"tpl.html", "{{ name | bold }}"})
	checkRender(t, engine, "tpl.html", testContext(map[string]any{"name": "world"}), "<b>world</b>")
}
func TestFunctionIsSafeSkipsAutoescape(t *testing.T) {
	engine := New()
	engine.AutoescapeOn([]string{".html"})
	engine.RegisterFunction("safe_html", safeHTMLTestFunction{})
	mustAdd(t, engine, RawTemplate{"tpl.html", "{{ safe_html() }}"})
	checkRender(t, engine, "tpl.html", NewContext(), "<em>hello</em>")
}
func TestFilterNotSafeGetsAutoescaped(t *testing.T) {
	engine := New()
	engine.AutoescapeOn([]string{".html"})
	engine.RegisterFilter("wrap_b", FilterFunc(func(v Value, _ Kwargs, _ *State) (Value, error) {
		return NormalString("<b>" + v.String() + "</b>"), nil
	}))
	mustAdd(t, engine, RawTemplate{"tpl.html", "{{ name | wrap_b }}"})
	got, err := engine.Render("tpl.html", testContext(map[string]any{"name": "world"}))
	if err != nil || !strings.Contains(got, "&lt;b&gt;") {
		t.Fatalf("expected escaped b tag, got %q, %v", got, err)
	}
}
