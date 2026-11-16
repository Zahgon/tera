package tera

import (
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func TestCanNegate(t *testing.T) {
	bound := new(big.Int).Lsh(big.NewInt(1), 127)
	min := new(big.Int).Neg(bound)
	max := new(big.Int).Sub(bound, big.NewInt(1))
	umax := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	for _, v := range []Value{I128(min), U128(umax)} {
		if _, err := negate(v); err == nil || !strings.Contains(err.Error(), "overflow") {
			t.Errorf("expected overflow, got %v", err)
		}
	}
	for _, tc := range [][2]Value{{I64(5), I64(-5)}, {I64(-5), I64(5)}, {I128(max), I128(new(big.Int).Neg(max))}, {U128(big.NewInt(5)), I128(big.NewInt(-5))}} {
		got, err := negate(tc[0])
		if err != nil || !got.Equal(tc[1]) {
			t.Errorf("expected %v, got %v, %v", tc[1], got, err)
		}
	}
}
func TestArithmeticInI128(t *testing.T) {
	for _, tc := range []struct {
		op   string
		a, b Value
		want int64
	}{{"+", I64(-1), U128(big.NewInt(2)), 1}, {"-", U128(big.NewInt(5)), U64(10), -5}} {
		got, err := arithmetic(tc.op, tc.a, tc.b)
		if err != nil || !got.Equal(I64(tc.want)) {
			t.Errorf("expected %d, got %v, %v", tc.want, got, err)
		}
	}
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))
	umax := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	for _, tc := range []struct {
		a, b    Value
		message string
	}{{I128(max), I128(big.NewInt(1)), "Unable to perform"}, {U128(umax), U64(0), "out of range"}} {
		_, err := arithmetic("+", tc.a, tc.b)
		if err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Errorf("expected %s, got %v", tc.message, err)
		}
	}
}
func TestBoolAndIntMapKeys(t *testing.T) {
	min := I128(new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127)))
	max := U128(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)))
	for _, tc := range []struct {
		input any
		key   Value
		want  string
	}{{map[bool]string{true: "yes"}, Bool(true), "yes"}, {map[int64]string{-5: "neg"}, I64(-5), "neg"}, {map[uint64]string{7: "pos"}, U64(7), "pos"}, {testValueKeyMap(t, min, "min"), min, "min"}, {testValueKeyMap(t, max, "max"), max, "max"}} {
		v := mustValue(t, tc.input)
		m, ok := v.AsMap()
		if !ok {
			t.Fatal("expected map")
		}
		key, err := KeyFromValue(tc.key)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := m.Get(key)
		if !ok || got.String() != tc.want {
			t.Errorf("expected %s, got %v", tc.want, got)
		}
	}
}
func TestFloatMapKeyFails(t *testing.T) {
	_, err := ValueFrom(map[float64]string{3.15: "value"})
	if err == nil || !strings.Contains(err.Error(), "map key must be") {
		t.Fatalf("expected map key error, got %v", err)
	}
}
func TestFromSerializableValueIsCloned(t *testing.T) {
	v := mustValue(t, map[string]string{"a": "b"})
	roundTrip := mustValue(t, v)
	a, _ := v.AsMap()
	b, _ := roundTrip.AsMap()
	if a != b {
		t.Fatal("Value conversion must share its map")
	}
}
func TestFromSerializableNestedValuesAreCloned(t *testing.T) {
	child := mustValue(t, map[string]string{"a": "b"})
	section := mustValue(t, struct {
		Name     string  `json:"name"`
		Children []Value `json:"children"`
	}{"w", []Value{child}})
	name, ok := section.GetFromPath("name")
	if !ok || name.String() != "w" {
		t.Fatal("missing name")
	}
	nested, ok := section.GetFromPath("children.0")
	if !ok {
		t.Fatal("missing child")
	}
	a, _ := child.AsMap()
	b, _ := nested.AsMap()
	if a != b {
		t.Fatal("nested Value conversion must share its map")
	}
}
func TestRegularSerializationWorks(t *testing.T) {
	inner := mustValue(t, map[string]string{"a": "b"})
	key, err := KeyFromValue(NormalString("k"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewMap()
	m.Insert(key, inner)
	v := mustValue(t, m)
	entry, ok := v.GetFromPath("k")
	if !ok {
		t.Fatal("missing k")
	}
	a, _ := inner.AsMap()
	b, _ := entry.AsMap()
	if a != b {
		t.Fatal("map-key serialization must preserve nested Value")
	}
}
func TestDeserialize(t *testing.T) {
	// Go represents serde's externally tagged enum variants with maps and slices.
	type content struct {
		Text      string                     `json:"text"`
		NumLikes  uint64                     `json:"num_likes"`
		Published bool                       `json:"published"`
		Kind      string                     `json:"kind"`
		Kind2     map[string]string          `json:"kind2"`
		Kind3     map[string][]int           `json:"kind3"`
		Kind4     map[string]map[string]bool `json:"kind4"`
		Maybe     *uint64                    `json:"maybe"`
		Nothing   *uint64                    `json:"nothing"`
		UserID    uint64                     `json:"user_id"`
	}
	maybe := uint64(42)
	instance := content{"hello", 10, true, "Article", map[string]string{"Comment": ""}, map[string][]int{"Tuple": {1, 1}}, map[string]map[string]bool{"Other": {"truthy": true}}, &maybe, nil, 1}
	v := mustValue(t, instance)
	for i := 0; i < 2; i++ {
		var out content
		if err := v.Deserialize(&out); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(out, instance) {
			t.Fatalf("expected %+v, got %+v", instance, out)
		}
	}
}

func testValueKeyMap(t testing.TB, key Value, value string) *Map {
	t.Helper()
	k, err := KeyFromValue(key)
	if err != nil {
		t.Fatal(err)
	}
	m := NewMap()
	m.Insert(k, NormalString(value))
	return m
}
