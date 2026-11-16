//go:build !tera_unicode

package tera

import "testing"

func TestLenAndReverseUseChars(t *testing.T) {
	v := NormalString("e\u0301cole")
	n, ok := v.Len()
	if !ok || n != 6 {
		t.Fatalf("expected length 6, got %d", n)
	}
	out, err := v.Reverse()
	if err != nil || out.String() != "eloc\u0301e" {
		t.Fatalf("expected scalar reversal, got %q, %v", out.String(), err)
	}
}
