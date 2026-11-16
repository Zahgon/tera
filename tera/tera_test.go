package tera

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGlobalContext(t *testing.T) {
	engine := New()
	engine.GlobalContext().Insert("name", "John Doe")
	for _, tc := range []struct {
		src, want string
		c         *Context
	}{{"Hello, {{ name }}!", "Hello, John Doe!", NewContext()}, {"UserID: {{ id }}, Username: {{ name }}", "UserID: 7489, Username: John Doe", testContext(map[string]any{"id": 7489})}} {
		got, err := engine.RenderStr(tc.src, tc.c, false)
		if err != nil || got != tc.want {
			t.Errorf("expected %q, got %q, %v", tc.want, got, err)
		}
	}
}
func TestCanFullReload(t *testing.T) {
	engine := New()
	if err := engine.LoadFromGlob("examples/basic/templates/**/*"); err != nil {
		t.Fatal(err)
	}
	if err := engine.FullReload(); err != nil {
		t.Fatal(err)
	}
	if !engine.ContainsTemplate("base.html") {
		t.Fatal("base.html missing")
	}
}
func writeFixture(t testing.TB, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestErrorOnMalformedTemplateInGlob(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "bad.html"), "{% if foo %}oops")
	if err := New().LoadFromGlob(filepath.ToSlash(dir) + "/**/*"); err == nil {
		t.Fatal("expected syntax error")
	}
}
func TestFailedFullReloadPreservesExistingTemplates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "good.html")
	writeFixture(t, path, "Hello {{ name }}")
	engine := New()
	if err := engine.LoadFromGlob(filepath.ToSlash(dir) + "/**/*"); err != nil {
		t.Fatal(err)
	}
	c := testContext(map[string]any{"name": "world"})
	checkRender(t, engine, "good.html", c, "Hello world")
	writeFixture(t, path, "{% if x %}oops")
	if err := engine.FullReload(); err == nil {
		t.Fatal("expected reload failure")
	}
	checkRender(t, engine, "good.html", c, "Hello world")
}
func TestLoadFromGlobPreservesManualTemplates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "page.html")
	writeFixture(t, path, "page")
	engine := New()
	mustAdd(t, engine, RawTemplate{"main.html", "main"})
	if err := engine.LoadFromGlob(filepath.ToSlash(dir) + "/**/*"); err != nil {
		t.Fatal(err)
	}
	checkRender(t, engine, "main.html", NewContext(), "main")
	checkRender(t, engine, "page.html", NewContext(), "page")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := engine.FullReload(); err != nil {
		t.Fatal(err)
	}
	checkRender(t, engine, "main.html", NewContext(), "main")
	if engine.ContainsTemplate("page.html") {
		t.Fatal("deleted page survived reload")
	}
}
func TestAddRawTemplateFailurePreservesExisting(t *testing.T) {
	engine := New()
	mustAdd(t, engine, RawTemplate{"good.html", "Hello {{ name }}"})
	if err := engine.AddRawTemplate("bad.html", "{{ name | no_such_filter }}"); err == nil {
		t.Fatal("expected unknown filter error")
	}
	if !engine.ContainsTemplate("good.html") || engine.ContainsTemplate("bad.html") {
		t.Fatal("failed transaction changed templates")
	}
}
func TestRenderingInvalidUTF8BytesDoesNotPanic(t *testing.T) {
	engine := New()
	mustAdd(t, engine, RawTemplate{"page.html", "{{ data }}"})
	c := NewContext()
	c.InsertValue("data", Bytes([]byte{255, 254, 253}))
	checkRender(t, engine, "page.html", c, "\ufffd\ufffd\ufffd")
}
func TestRenderComponent(t *testing.T) {
	engine := New()
	mustAdd(t, engine, RawTemplate{"components.html", `{% component Button(label, variant="primary") %}<button class="{{ variant }}">{{ label }}</button>{% endcomponent Button %}
{% component Card(title) %}<div><h1>{{ title }}</h1>{{ body }}</div>{% endcomponent Card %}
{% component Display(content) %}{{ content }}{% endcomponent Display %}
{% component show(@name, @lang = "fr") %}{{ name }}-{{ lang }}{% endcomponent show %}
{% component wrapper() %}{{ <show /> }}{% endcomponent wrapper %}
{% component attrs(...rest) %}{% for k, v in rest %}{{k}}={{v}} {% endfor %} || {{ <show /> }}{% endcomponent attrs %}`}, RawTemplate{"components.txt", `{% component Raw(content) %}{{ content }}{% endcomponent Raw %}`})
	body := "<p>body</p>"
	for _, tc := range []struct {
		name string
		c    *Context
		body *string
		auto bool
		want string
	}{
		{"Button", testContext(map[string]any{"label": "Click"}), nil, true, `<button class="primary">Click</button>`},
		{"Button", testContext(map[string]any{"label": "X", "variant": "secondary"}), nil, true, `<button class="secondary">X</button>`},
		{"Card", testContext(map[string]any{"title": "T"}), &body, true, "<div><h1>T</h1><p>body</p></div>"},
		{"Display", testContext(map[string]any{"content": "<script>"}), nil, true, "&lt;script&gt;"},
		{"Raw", testContext(map[string]any{"content": "<script>"}), nil, false, "<script>"},
	} {
		got, err := engine.RenderComponent(tc.name, tc.c, tc.body, tc.auto)
		if err != nil || got != tc.want {
			t.Errorf("%s: expected %q, got %q, %v", tc.name, tc.want, got, err)
		}
	}
	var buffer bytes.Buffer
	if err := engine.RenderComponentTo("Button", testContext(map[string]any{"label": "Y"}), nil, true, &buffer); err != nil {
		t.Fatal(err)
	}
	if buffer.String() != `<button class="primary">Y</button>` {
		t.Error(buffer.String())
	}
	implicits := testContext(map[string]any{"name": "Vincent"})
	for _, tc := range []struct {
		name string
		c    *Context
		want string
	}{{"wrapper", NewContext(), "Vincent-fr"}, {"show", NewContext(), "Vincent-fr"}, {"show", testContext(map[string]any{"name": "Jane"}), "Jane-fr"}, {"attrs", testContext(map[string]any{"class": "btn"}), "class=btn  || Vincent-fr"}} {
		got, err := engine.RenderComponentWithImplicits(tc.name, tc.c, implicits, nil, true)
		if err != nil || got != tc.want {
			t.Errorf("%s: expected %q, got %q, %v", tc.name, tc.want, got, err)
		}
	}
	for _, tc := range []struct {
		name string
		c    *Context
	}{{"Button", testContext(map[string]any{"label": "x", "bad": "y"})}, {"Nope", NewContext()}, {"Button", NewContext()}} {
		if got, err := engine.RenderComponent(tc.name, tc.c, nil, true); err == nil {
			t.Errorf("expected error, got %q", got)
		}
	}
	if got, err := engine.RenderComponentWithImplicits("show", testContext(map[string]any{"bad": "y"}), implicits, nil, true); err == nil {
		t.Errorf("expected error, got %q", got)
	}
}
func TestAddTemplateFileErrorsOnNonUTF8Path(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Rust test is Unix-only")
	}
	err := New().AddTemplateFile(filepath.Join(t.TempDir(), string([]byte{255, 254})+".html"), "")
	if err == nil || !strings.Contains(err.Error(), "not valid UTF-8") {
		t.Fatalf("expected UTF-8 error, got %v", err)
	}
}
func TestCustomDelimiters(t *testing.T) {
	engine := New()
	if err := engine.SetDelimiters(Delimiters{"<%", "%>", "<<", ">>", "<#", "#>"}); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, engine, RawTemplate{"test", "Hello, <# This is a comment #><% if show %><< name >>!<% endif %>"})
	checkRender(t, engine, "test", testContext(map[string]any{"name": "World", "show": true}), "Hello, World!")
}
func TestFallbackPrefixesResolveTemplates(t *testing.T) {
	engine := New()
	if err := engine.SetFallbackPrefixes([]string{"themes/cool/"}); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, engine, RawTemplate{"themes/cool/base.html", "{% block content %}default{% endblock %}"}, RawTemplate{"themes/cool/partial.html", "partial"}, RawTemplate{"child.html", `{% extends "base.html" %}{% block content %}child-{% include "partial.html" %}{% endblock %}`})
	checkRender(t, engine, "child.html", NewContext(), "child-partial")
}
func TestFallbackPrefixExactMatchTakesPriority(t *testing.T) {
	engine := New()
	if err := engine.SetFallbackPrefixes([]string{"themes/cool/"}); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, engine, RawTemplate{"base.html", "exact"}, RawTemplate{"themes/cool/base.html", "fallback"})
	checkRender(t, engine, "base.html", NewContext(), "exact")
}
func TestAddingTemplateReResolvesLineageProperly(t *testing.T) {
	engine := New()
	if err := engine.SetFallbackPrefixes([]string{"themes/cool/"}); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, engine, RawTemplate{"themes/cool/base.html", "fallback"})
	mustAdd(t, engine, RawTemplate{"child.html", `{% extends "base.html" %}`})
	checkRender(t, engine, "child.html", NewContext(), "fallback")
	mustAdd(t, engine, RawTemplate{"base.html", "main"})
	checkRender(t, engine, "child.html", NewContext(), "main")
}
func TestGetTemplatePriority(t *testing.T) {
	engine := New()
	if err := engine.SetFallbackPrefixes([]string{"themes/child/", "themes/parent/"}); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{"index.html": 0, "themes/child/base.html": 1, "themes/parent/base.html": 2} {
		if got := engine.priority(name); got != want {
			t.Errorf("%s: expected %d, got %d", name, want, got)
		}
	}
}
func TestComponentDuplicateErrorSamePriority(t *testing.T) {
	engine := New()
	if err := engine.SetFallbackPrefixes([]string{"themes/"}); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, engine, RawTemplate{"a.html", "{% component Foo() %}A{% endcomponent Foo %}"})
	err := engine.AddRawTemplate("b.html", "{% component Foo() %}B{% endcomponent Foo %}")
	if err == nil || !strings.Contains(err.Error(), "Component `Foo` is defined in both") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}
func TestComponentOverrideChain(t *testing.T) {
	engine := New()
	if err := engine.SetFallbackPrefixes([]string{"child/", "parent/"}); err != nil {
		t.Fatal(err)
	}
	for _, tpl := range []RawTemplate{{"parent/c.html", "{% component X() %}parent{% endcomponent X %}"}, {"child/c.html", "{% component X() %}child{% endcomponent X %}"}, {"user.html", "{% component X() %}user{% endcomponent X %}"}, {"test.html", "{{<X/>}}"}} {
		mustAdd(t, engine, tpl)
	}
	checkRender(t, engine, "test.html", NewContext(), "user")
}
func TestFallbackTemplateResolution(t *testing.T) {
	engine := New()
	if err := engine.SetFallbackPrefixes([]string{"themes/cool/"}); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, engine, RawTemplate{"themes/cool/base.html", "theme base"})
	mustAdd(t, engine, RawTemplate{"index.html", `{% extends "base.html" %}`})
	checkRender(t, engine, "base.html", NewContext(), "theme base")
}
func TestRenderStr(t *testing.T) {
	engine := New()
	mustAdd(t, engine, RawTemplate{"partial.html", "I am partial"}, RawTemplate{"components.html", `{% component Greet(name) %}Hello {{ name }}!{% endcomponent Greet %}`})
	engine.RegisterFilter("shout", TypedFilterValue(ArgStr, func(s string, _ Kwargs, _ *State) string { return strings.ToUpper(s) }))
	got, err := engine.RenderStr(`Hello {{ name }}!. {% include "partial.html" %} - {{<Greet name="World"/>}}`, testContext(map[string]any{"name": "world"}), false)
	if err != nil || got != "Hello world!. I am partial - Hello World!" {
		t.Fatalf("got %q, %v", got, err)
	}
}
func TestRenderStrRejectsInheritanceAndBlocks(t *testing.T) {
	for _, src := range []string{`{% extends "base.html" %}hi`, `Before {% block content %}default{% endblock content %} After`} {
		if got, err := New().RenderStr(src, NewContext(), false); err == nil {
			t.Errorf("expected error, got %q", got)
		}
	}
}
func TestRenderStrAutoescape(t *testing.T) {
	for auto, want := range map[bool]string{true: "&lt;script&gt;", false: "<script>"} {
		got, err := New().RenderStr("{{ html }}", testContext(map[string]any{"html": "<script>"}), auto)
		if err != nil || got != want {
			t.Errorf("expected %q, got %q, %v", want, got, err)
		}
	}
}
func TestRenderBlockWorks(t *testing.T) {
	engine := New()
	mustAdd(t, engine, RawTemplate{"base.html", "{% block nav %}nav{% endblock %}{% block content %}default{% endblock %}"}, RawTemplate{"child.html", `{% extends "base.html" %}{% block content %}child-{{super()}}{% endblock %}`}, RawTemplate{"nested.html", "{% block outer %}<o>{% block inner %}inner{% endblock %}</o>{% endblock %}"})
	if got, err := engine.RenderBlock("child.html", "unknown", NewContext()); err == nil {
		t.Errorf("expected unknown block error, got %q", got)
	}
	for _, tc := range [][3]string{{"child.html", "content", "child-default"}, {"nested.html", "inner", "inner"}, {"nested.html", "outer", "<o>inner</o>"}} {
		got, err := engine.RenderBlock(tc[0], tc[1], NewContext())
		if err != nil || got != tc[2] {
			t.Errorf("expected %q, got %q, %v", tc[2], got, err)
		}
	}
}
