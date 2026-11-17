// Ports the unit tests of tera/src/filters.rs: test_title, test_str, test_indent_indentation, test_int, test_float,
// test_escape_xml, test_abs, test_round and truncate_splits_on_char_boundary. can_truncate_graphemes tests the
// `unicode` Cargo feature, which this port does not provide.

package tera

import (
	"math/big"
	"testing"
)

func checkFilter(t testing.TB, name string, input any, kwargs map[string]any, want any) {
	t.Helper()
	v, err := New().filters[name].Call(mustValue(t, input), testKwargs(t, kwargs), NewState(NewContext()))
	if err != nil {
		t.Fatal(err)
	}
	expected := mustValue(t, want)
	if !v.Equal(expected) {
		t.Errorf("%s(%v): expected %v, got %v", name, input, expected, v)
	}
}
func checkFilterError(t testing.TB, name string, input any, kwargs map[string]any) {
	t.Helper()
	v, err := New().filters[name].Call(mustValue(t, input), testKwargs(t, kwargs), NewState(NewContext()))
	if err == nil {
		t.Errorf("%s(%v): expected error, got %v", name, input, v)
	}
}
func TestTitle(t *testing.T) {
	for _, tc := range [][2]string{{"foo bar", "Foo Bar"}, {"foo\tbar", "Foo\tBar"}, {"foo  bar", "Foo  Bar"}, {"f bar f", "F Bar F"}, {"foo-bar", "Foo-Bar"}, {"FOO\tBAR", "Foo\tBar"}, {"foo (bar)", "Foo (Bar)"}, {"foo (bar) ", "Foo (Bar) "}, {"foo {bar}", "Foo {Bar}"}, {"foo [bar]", "Foo [Bar]"}, {"foo <bar>", "Foo <Bar>"}, {"  foo  bar", "  Foo  Bar"}, {"\tfoo\tbar\t", "\tFoo\tBar\t"}, {"foo bar ", "Foo Bar "}, {"foo bar\t", "Foo Bar\t"}, {"foo's bar", "Foo's Bar"}} {
		checkFilter(t, "title", tc[0], nil, tc[1])
	}
}
func TestStr(t *testing.T) {
	for _, tc := range []struct {
		input any
		want  string
	}{{2.1, "2.1"}, {2, "2"}, {true, "true"}, {[]int{1, 2, 3}, "[1, 2, 3]"}, {map[string]any{"hello": "world", "other": 2}, `{"hello": "world", "other": 2}`}} {
		checkFilter(t, "str", tc.input, nil, tc.want)
	}
}
func TestIndentIndentation(t *testing.T) {
	checkFilter(t, "indent", "one\ntwo", map[string]any{"width": 2, "indentation": "·"}, "one\n··two")
	for _, indentation := range []string{"", "ab"} {
		checkFilterError(t, "indent", "one\ntwo", map[string]any{"indentation": indentation})
	}
}
func TestInt(t *testing.T) {
	for _, tc := range []struct {
		input any
		want  int64
	}{{"0", 0}, {"-5", -5}, {"9223372036854775807", 9223372036854775807}, {"1.00", 1}} {
		checkFilter(t, "int", tc.input, nil, tc.want)
	}
	checkFilter(t, "int", "0b1010", map[string]any{"base": 2}, 10)
	checkFilter(t, "int", I128(big.NewInt(-5)), nil, I128(big.NewInt(-5)))
	for _, input := range []any{1.12, "hello", []int{1, 2}} {
		checkFilterError(t, "int", input, nil)
	}
}
func TestFloat(t *testing.T) {
	for _, tc := range []struct {
		input any
		want  float64
	}{{"1", 1}, {"3.16", 3.16}, {1, 1}, {1.12, 1.12}} {
		checkFilter(t, "float", tc.input, nil, tc.want)
	}
	for _, input := range []any{"hello", []int{1, 2}} {
		checkFilterError(t, "float", input, nil)
	}
}
func TestEscapeXML(t *testing.T) {
	for _, tc := range [][2]string{{`hey-&-ho`, `hey-&amp;-ho`}, {`hey-'-ho`, `hey-&apos;-ho`}, {`hey-&'-ho`, `hey-&amp;&apos;-ho`}, {`hey-&'"-ho`, `hey-&amp;&apos;&quot;-ho`}, {`hey-&'"<-ho`, `hey-&amp;&apos;&quot;&lt;-ho`}, {`hey-&'"<>-ho`, `hey-&amp;&apos;&quot;&lt;&gt;-ho`}} {
		checkFilter(t, "escape_xml", tc[0], nil, tc[1])
	}
}
func TestAbs(t *testing.T) {
	for _, input := range []any{1, int64(-1), -1.0} {
		checkFilter(t, "abs", input, nil, 1)
	}
	min := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127))
	checkFilterError(t, "abs", I128(min), nil)
	checkFilterError(t, "abs", "hello", nil)
}
func TestRound(t *testing.T) {
	checkFilter(t, "round", 2.1, nil, 2)
	checkFilter(t, "round", 2.1, map[string]any{"method": "ceil"}, 3)
	checkFilter(t, "round", 2.9, map[string]any{"method": "floor"}, 2)
	checkFilter(t, "round", 2.245, map[string]any{"precision": 2}, 2.25)
}
func TestTruncateSplitsOnCharBoundary(t *testing.T) {
	for _, tc := range []struct {
		input  string
		length int
		want   string
	}{{"😀test", 1, "😀…"}, {"日本語hello", 3, "日本語…"}} {
		checkFilter(t, "truncate", tc.input, map[string]any{"length": tc.length}, tc.want)
	}
}
