package contrib

import (
	"github.com/Keats/tera/tera"
	"testing"
)

func value(t testing.TB, input any) tera.Value {
	t.Helper()
	v, err := tera.ValueFrom(input)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func kwargs(t testing.TB, input map[string]any) tera.Kwargs {
	t.Helper()
	m, ok := value(t, input).AsMap()
	if !ok {
		t.Fatal("expected map")
	}
	return tera.NewKwargs(m)
}
func checkFilter(t testing.TB, f tera.Filter, input any, k map[string]any, want any) {
	t.Helper()
	got, err := f.Call(value(t, input), kwargs(t, k), tera.NewState(tera.NewContext()))
	if err != nil {
		t.Fatal(err)
	}
	expected := value(t, want)
	if !got.Equal(expected) {
		t.Errorf("expected %q, got %q", expected.String(), got.String())
	}
}
func checkFilterError(t testing.TB, f tera.Filter, input any, k map[string]any) {
	t.Helper()
	got, err := f.Call(value(t, input), kwargs(t, k), tera.NewState(tera.NewContext()))
	if err == nil {
		t.Errorf("expected error, got %v", got)
	}
}
func TestRegister(t *testing.T) {
	engine := tera.New()
	Register(engine)
	// Exercise each registration through the public rendering API.
	src := `{{ "hello" | b64_encode | b64_decode }}|{{ 1024 | filesize_format }}|{{ 42 | format(spec="05") }}|{{ {"a":1} | json_encode }}|{{ "Hello World" | slug }}|{{ "a/b" | urlencode }}|{{ "a/b" | urlencode_strict }}|{{ "<b>Joel</b>" | striptags }}|{{ "<p> </p>" | spaceless }}|{{ "a1" | regex_replace(pattern="\\d",rep="X") }}|{% if "abc" is matching(pat="b") %}yes{% endif %}|{{ get_random(start=0,end=1) }}|{{ [] | shuffle }}|{{ 1482720453 | date }}|{% if 500 is before(other=1000) and 1000 is after(other=500) %}yes{% endif %}|{{ now() is string }}`
	if err := engine.AddRawTemplate("test", src); err != nil {
		t.Fatal(err)
	}
	got, err := engine.Render("test", tera.NewContext())
	if err != nil {
		t.Fatal(err)
	}
	want := `hello|1 KiB|00042|{"a":1}|hello-world|a/b|a%2Fb|Joel|<p></p>|aX|yes|0|[]|2016-12-26|yes|true`
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
