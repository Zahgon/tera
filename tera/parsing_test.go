package tera

import "testing"

func TestParserExtendsSuccess(t *testing.T) {
	for _, tc := range [][2]string{{"{% extends 'a.html' %}", "a.html"}, {`{% extends "base\t.html" %}`, "base\t.html"}} {
		tp, err := parseTemplate("", tc[0], DefaultDelimiters())
		if err != nil || tp.parent != tc[1] {
			t.Errorf("expected parent %q, got %v, %v", tc[1], tp, err)
		}
	}
}
func TestParserConstantArrayValues(t *testing.T) {
	// The Go evaluator uses the AST instead of Rust's folded bytecode constants.
	for _, tc := range []struct {
		src  string
		want any
	}{{`{{ [1, 2, 3] }}`, []int{1, 2, 3}}, {`{{ [-1, {"value": -1.5}] }}`, []any{-1, map[string]float64{"value": -1.5}}}} {
		tp, err := parseTemplate("tpl", tc.src, DefaultDelimiters())
		if err != nil {
			t.Fatal(err)
		}
		if len(tp.nodes) != 1 {
			t.Fatal("expected a single expression")
		}
		got, err := OneOff(tc.src, NewContext(), false)
		want := mustValue(t, tc.want).String()
		if err != nil || got != want {
			t.Errorf("expected %q, got %q, %v", want, got, err)
		}
	}
}
