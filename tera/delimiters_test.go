// Ports the unit test of tera/src/delimiters.rs: errors_on_invalid_delimiters.

package tera

import "testing"

func TestErrorsOnInvalidDelimiters(t *testing.T) {
	for _, s := range []string{"", "[[[", "日"} {
		d := DefaultDelimiters()
		d.BlockStart = s
		if err := d.validate(); err == nil {
			t.Errorf("expected invalid delimiter: %q", s)
		}
	}
	d := DefaultDelimiters()
	d.BlockStart = "[["
	d.CommentStart = "[["
	if err := d.validate(); err == nil {
		t.Fatal("expected duplicate delimiter error")
	}
}
