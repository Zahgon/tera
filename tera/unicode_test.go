//go:build tera_unicode

package tera

import "testing"

// These tests retain the Rust unicode-feature contract. The reference command
// uses scalar strings; run separately with -tags=tera_unicode to assess this
// optional feature without mixing its contradictory string-length expectations.
func TestLenAndReverseUseGraphemes(t *testing.T) {
	v := NormalString("e\u0301cole")
	n, ok := v.Len()
	if !ok || n != 5 {
		t.Errorf("expected length 5, got %d", n)
	}
	out, err := v.Reverse()
	if err != nil || out.String() != "eloce\u0301" {
		t.Errorf("expected grapheme reversal, got %q, %v", out.String(), err)
	}
}
func TestCanTruncateGraphemes(t *testing.T) {
	checkFilter(t, "truncate", "日本語", map[string]any{"length": 2}, "日本…")
	checkFilter(t, "truncate", "👨‍👩‍👧‍👦 family", map[string]any{"length": 5}, "👨‍👩‍👧‍👦 fam…")
}
func TestCanIterateOnGraphemes(t *testing.T) {
	src := `{% for c in string -%}
{{loop.index}}.{{c}}
{% endfor %}`
	engine := New()
	mustAdd(t, engine, RawTemplate{"tpl", src})
	got, err := engine.Render("tpl", testContext(map[string]any{"string": "नमस्ते"}))
	if err != nil {
		t.Fatal(err)
	}
	_, want := readSnapshot(t, snapshotRoot+"/snapshots/tera__snapshot_tests__rendering__can_iterate_on_graphemes.snap")
	assertSnapshot(t, got, want)
}
func TestCanSliceOnGraphemes(t *testing.T) {
	src := `
{{ string[::-1] }}
{{ string[1:] }}
{{ string[:-1] }}
`
	engine := New()
	mustAdd(t, engine, RawTemplate{"tpl", src})
	got, err := engine.Render("tpl", testContext(map[string]any{"string": "नमस्ते"}))
	if err != nil {
		t.Fatal(err)
	}
	_, want := readSnapshot(t, snapshotRoot+"/snapshots/tera__snapshot_tests__rendering__can_slice_on_graphemes.snap")
	assertSnapshot(t, got, want)
}
