package contrib

import (
	"github.com/Keats/tera/tera"
	"testing"
)

func TestGetRandom(t *testing.T) {
	got, err := GetRandom.Call(kwargs(t, map[string]any{"start": 0, "end": 10, "seed": 42}), tera.NewState(tera.NewContext()))
	if err != nil {
		t.Fatal(err)
	}
	n, ok := got.AsI64()
	if !ok || n < 0 || n >= 10 || n != 9 {
		t.Fatalf("expected 9 in [0,10), got %v", got)
	}
}
func TestShuffle(t *testing.T) {
	input := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	v := value(t, input)
	got, err := Shuffle.Call(v, kwargs(t, map[string]any{"seed": 42}), tera.NewState(tera.NewContext()))
	if err != nil {
		t.Fatal(err)
	}
	a, ok := got.AsArray()
	if !ok || len(a) != len(input) || got.Equal(v) {
		t.Fatalf("expected shuffled 10-element list, got %v", got)
	}
}
