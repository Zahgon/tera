package tera

import (
	"io"
	"strings"
	"testing"
)

func TestStateCanHandleEscaping(t *testing.T) {
	engine := New()
	engine.RegisterFunction("force_esc", TypedFunction(func(k Kwargs, s *State) (string, error) {
		text, err := KwargsMustGet(k, "text", ArgStr)
		if err != nil {
			return "", err
		}
		return s.Escape(text)
	}))
	engine.RegisterFunction("esc_if_needed", TypedFunction(func(k Kwargs, s *State) (string, error) {
		text, err := KwargsMustGet(k, "text", ArgValue)
		if err != nil {
			return "", err
		}
		return s.EscapeIfNeeded(text)
	}))
	c := testContext(map[string]any{"name": "<script>"})
	engine.AutoescapeOn([]string{".txt"})
	mustAdd(t, engine, RawTemplate{"tpl.html", "{{ force_esc(text=name) | safe }} - {{esc_if_needed(text=name)}}"}, RawTemplate{"tpl2.html", "{% set safe_name = name | safe %}{{ esc_if_needed(text=name) | safe }} - {{ esc_if_needed(text=safe_name) | safe }}"})
	checkRender(t, engine, "tpl.html", c, "&lt;script&gt; - <script>")
	engine.SetEscapeFn(func(input string, w io.Writer) error { _, err := io.WriteString(w, strings.ToUpper(input)); return err })
	checkRender(t, engine, "tpl.html", c, "<SCRIPT> - <script>")
	engine.ResetEscapeFn()
	engine.AutoescapeOn([]string{".html"})
	checkRender(t, engine, "tpl2.html", c, "&lt;script&gt; - <script>")
}
