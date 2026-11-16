package tera

import (
	"testing"
	"unicode/utf8"
)

// FuzzExpression exercises the same expression wrapper as the Rust fuzz target.
func FuzzExpression(f *testing.F) {
	for _, seed := range []string{"", "name", "1 + 2", "[1, 2, 3]", "user?.name", "1 if true else 2", "foo | default(value=1)", "{\"a\": -1}", "(((("} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if !utf8.Valid(data) {
			return
		}
		_ = New().AddRawTemplate("fuzzing.html", "{{ "+string(data)+" }}")
	})
}

// FuzzTag exercises the same statement wrapper as the Rust fuzz target.
func FuzzTag(f *testing.F) {
	for _, seed := range []string{"", "if true", "set x = 1", "include 'missing'", "for x in [1,2]", "endfor", "component Button(label)", "raw", "break"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if !utf8.Valid(data) {
			return
		}
		_ = New().AddRawTemplate("fuzzing.html", "{% "+string(data)+" %}")
	})
}

// FuzzTemplate compiles arbitrary valid UTF-8 templates without rendering them.
func FuzzTemplate(f *testing.F) {
	for _, seed := range []string{"", "Hello {{ name }}!", "{% for x in [1,2] %}{{x}}{% endfor %}", "{% if true %}yes{% else %}no{% endif %}", "{# comment #}", "{% raw %}{{ x }}{% endraw %}", "{% component X() %}x{% endcomponent %}{{<X/>}}", "{{ [x * 2 for x in [1,2]] }}"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if !utf8.Valid(data) {
			return
		}
		_ = New().AddRawTemplate("fuzzing.html", string(data))
	})
}
