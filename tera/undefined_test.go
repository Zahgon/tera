package tera

import "testing"

func TestHandlesUndefinedCorrectly(t *testing.T) {
	m := NewMap()
	m.InsertString("key", Undefined())
	c := testContext(map[string]any{"existing": map[string]string{"label": ""}})
	c.InsertValue("m", MapValue(m))
	for _, tc := range []struct {
		src, want string
		fails     bool
	}{
		{`{{ hey }}`, "", true}, {`{{ existing.hey }}`, "", true}, {`{{ hey or 1 }}`, "1", false}, {`{{ hey.other or 1 }}`, "", true},
		{`{% if hey or true %}truthy{% endif %}`, "truthy", false}, {`{% if hey.other or true %}truthy{% endif %}`, "", true},
		{`{{ m.key | default(value='fallback') }}`, "fallback", false}, {`{{ m.key.foo | default(value='fallback') }}`, "", true},
	} {
		t.Run(tc.src, func(t *testing.T) {
			engine := New()
			mustAdd(t, engine, RawTemplate{"tpl", tc.src})
			got, err := engine.Render("tpl", c)
			if tc.fails {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("expected %q, got %q, %v", tc.want, got, err)
			}
		})
	}
}
