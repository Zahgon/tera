package tera

import (
	"strings"
	"testing"
)

func testKwargs(t testing.TB, values map[string]any) Kwargs {
	t.Helper()
	m, ok := mustValue(t, values).AsMap()
	if !ok {
		t.Fatal("expected map")
	}
	return NewKwargs(m)
}
func TestCanGetKwargWithType(t *testing.T) {
	k := testKwargs(t, map[string]any{"hello": "world", "num": 1.1})
	s, ok, err := KwargsGet(k, "hello", ArgStr)
	if err != nil || !ok || s != "world" {
		t.Fatalf("got %q %t %v", s, ok, err)
	}
	n, ok, err := KwargsGet(k, "num", ArgF64)
	if err != nil || !ok || n != 1.1 {
		t.Fatalf("got %v %t %v", n, ok, err)
	}
	_, ok, err = KwargsGet(k, "unknown", ArgI64)
	if err != nil || ok {
		t.Fatal("unknown must be absent")
	}
	var data struct {
		Hello string  `json:"hello"`
		Num   float64 `json:"num"`
	}
	if err := k.Deserialize(&data); err != nil {
		t.Fatal(err)
	}
	if data.Hello != "world" || data.Num != 1.1 {
		t.Fatalf("bad data: %+v", data)
	}
}
func TestIntOutOfRangeReportsRangeNotType(t *testing.T) {
	_, _, err := KwargsGet(testKwargs(t, map[string]any{"n": 300}), "n", ArgU8)
	if err == nil || err.Error() != "Value `300` is out of range for `u8`" {
		t.Fatalf("expected u8 range error, got %v", err)
	}
	_, _, err = KwargsGet(testKwargs(t, map[string]any{"n": -1}), "n", ArgUsize)
	if err == nil || err.Error() != "Value `-1` is out of range for `usize`" {
		t.Fatalf("expected usize range error, got %v", err)
	}
	_, _, err = KwargsGet(testKwargs(t, map[string]any{"n": 1e40}), "n", ArgI128)
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected i128 range error, got %v", err)
	}
}
