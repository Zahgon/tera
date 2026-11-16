package tera

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Keats/tera/internal/rustrt"
)

// ValueKind is the underlying kind of a Value.
type ValueKind uint8

// Value kinds distinguish all Rust scalar and container variants.
const (
	// KindUndefined identifies undefined values.
	KindUndefined ValueKind = iota
	// KindNone identifies none values.
	KindNone
	// KindBool identifies bool values.
	KindBool
	// KindU64 identifies u64 values.
	KindU64
	// KindI64 identifies i64 values.
	KindI64
	// KindU128 identifies u128 values.
	KindU128
	// KindI128 identifies i128 values.
	KindI128
	// KindF64 identifies f64 values.
	KindF64
	// KindString identifies string values.
	KindString
	// KindArray identifies array values.
	KindArray
	// KindMap identifies map values.
	KindMap
	// KindBytes identifies bytes values.
	KindBytes
)

// Value is an immutable template value. Its zero value is undefined.
type Value struct {
	kind  ValueKind
	b     bool
	u     uint64
	i     int64
	f     float64
	n     *big.Int
	s     string
	safe  bool
	a     []Value
	m     *Map
	bytes []byte
}

// Undefined constructs a missing value.
func Undefined() Value { return Value{} }

// None constructs an explicit null value.
func None() Value { return Value{kind: KindNone} }

// Bool constructs a boolean.
func Bool(b bool) Value { return Value{kind: KindBool, b: b} }

// U64 constructs an unsigned integer.
func U64(n uint64) Value { return Value{kind: KindU64, u: n} }

// I64 constructs a signed integer.
func I64(n int64) Value { return Value{kind: KindI64, i: n} }

// U128 constructs an unsigned 128-bit integer, copying n.
func U128(n *big.Int) Value {
	if n == nil {
		return U64(0)
	}
	return Value{kind: KindU128, n: new(big.Int).Set(n)}
}

// I128 constructs a signed 128-bit integer, copying n.
func I128(n *big.Int) Value {
	if n == nil {
		return I64(0)
	}
	return Value{kind: KindI128, n: new(big.Int).Set(n)}
}

// F64 constructs a float.
func F64(n float64) Value { return Value{kind: KindF64, f: n} }

// NormalString constructs an ordinary string.
func NormalString(s string) Value { return Value{kind: KindString, s: s} }

// SafeString constructs a string exempt from autoescaping.
func SafeString(s string) Value { return Value{kind: KindString, s: s, safe: true} }

// Bytes constructs an opaque byte string.
func Bytes(b []byte) Value { return Value{kind: KindBytes, bytes: append([]byte{}, b...)} }

// Array constructs an array.
func Array(a []Value) Value { return Value{kind: KindArray, a: append([]Value{}, a...)} }

// MapValue constructs an ordered map value.
func MapValue(m *Map) Value {
	if m == nil {
		m = NewMap()
	}
	return Value{kind: KindMap, m: m.Clone()}
}

// Kind returns the underlying variant.
func (v Value) Kind() ValueKind { return v.kind }

// Name returns the Rust type name.
func (v Value) Name() string {
	return [...]string{"undefined", "none", "bool", "u64", "i64", "u128", "i128", "f64", "string", "array", "map/struct", "bytes"}[v.kind]
}

// AsStr returns a string value.
func (v Value) AsStr() (string, bool) { return v.s, v.kind == KindString }

// AsBool returns a boolean value.
func (v Value) AsBool() (bool, bool) { return v.b, v.kind == KindBool }

// AsArray returns the array elements.
func (v Value) AsArray() ([]Value, bool) { return v.a, v.kind == KindArray }

// AsMap returns the ordered map.
func (v Value) AsMap() (*Map, bool) { return v.m, v.kind == KindMap }

// AsBytes returns the underlying bytes.
func (v Value) AsBytes() ([]byte, bool) { return v.bytes, v.kind == KindBytes }
func (v Value) integer() *big.Int {
	switch v.kind {
	case KindU64:
		return new(big.Int).SetUint64(v.u)
	case KindI64:
		return big.NewInt(v.i)
	case KindU128, KindI128:
		return new(big.Int).Set(v.n)
	}
	return nil
}

var maxI128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 127), big.NewInt(1))
var minI128 = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127))
var maxU128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))

// AsI128 returns a copy of an integral value that fits in signed 128 bits.
func (v Value) AsI128() (*big.Int, bool) {
	n := v.integer()
	return n, n != nil && n.Cmp(minI128) >= 0 && n.Cmp(maxI128) <= 0
}

// AsU128 returns a copy of an integral value that fits in unsigned 128 bits.
func (v Value) AsU128() (*big.Int, bool) {
	n := v.integer()
	return n, n != nil && n.Sign() >= 0 && n.Cmp(maxU128) <= 0
}

// AsI64 returns an integer that fits in signed 64 bits.
func (v Value) AsI64() (int64, bool) {
	n := v.integer()
	if n == nil || !n.IsInt64() {
		return 0, false
	}
	return n.Int64(), true
}

// AsU64 returns an integer that fits in unsigned 64 bits.
func (v Value) AsU64() (uint64, bool) {
	n := v.integer()
	if n == nil || !n.IsUint64() {
		return 0, false
	}
	return n.Uint64(), true
}

// AsF64 converts floats and integers with magnitude at most 2^53.
func (v Value) AsF64() (float64, bool) {
	if v.kind == KindF64 {
		return v.f, true
	}
	n := v.integer()
	if n == nil || new(big.Int).Abs(n).Cmp(new(big.Int).Lsh(big.NewInt(1), 53)) > 0 {
		return 0, false
	}
	f, _ := new(big.Float).SetInt(n).Float64()
	return f, true
}

// AsNumber converts arithmetic values into Number.
func (v Value) AsNumber() (Number, bool) {
	if v.kind == KindF64 {
		return FloatNumber(v.f), true
	}
	if n, ok := v.AsI128(); ok {
		return IntegerNumber(n), true
	}
	return Number{}, false
}

// IsSafe reports whether autoescaping is unnecessary.
func (v Value) IsSafe() bool {
	switch v.kind {
	case KindString:
		return v.safe
	case KindArray, KindMap, KindBytes:
		return false
	}
	return true
}

// MarkSafe marks strings safe.
func (v Value) MarkSafe() Value {
	if v.kind == KindString {
		v.safe = true
	}
	return v
}

// IsTruthy implements Tera truthiness.
func (v Value) IsTruthy() bool {
	switch v.kind {
	case KindUndefined, KindNone:
		return false
	case KindBool:
		return v.b
	case KindU64:
		return v.u != 0
	case KindI64:
		return v.i != 0
	case KindU128, KindI128:
		return v.n.Sign() != 0
	case KindF64:
		return v.f != 0
	case KindString:
		return v.s != ""
	case KindArray:
		return len(v.a) > 0
	case KindMap:
		return v.m.Len() > 0
	case KindBytes:
		return len(v.bytes) > 0
	}
	return false
}

// Len returns a container's length, counting Unicode scalar values for strings.
func (v Value) Len() (int, bool) {
	switch v.kind {
	case KindString:
		return utf8.RuneCountInString(v.s), true
	case KindArray:
		return len(v.a), true
	case KindMap:
		return v.m.Len(), true
	case KindBytes:
		return len(v.bytes), true
	}
	return 0, false
}

// String renders a value with Rust formatting rules.
func (v Value) String() string {
	switch v.kind {
	case KindUndefined, KindNone:
		return ""
	case KindBool:
		return strconv.FormatBool(v.b)
	case KindU64:
		return strconv.FormatUint(v.u, 10)
	case KindI64:
		return strconv.FormatInt(v.i, 10)
	case KindU128, KindI128:
		return v.n.String()
	case KindF64:
		return rustrt.FloatDebug(v.f)
	case KindString:
		return v.s
	case KindBytes:
		return rustrt.LossyUTF8(v.bytes)
	case KindArray:
		a := make([]string, len(v.a))
		for i, x := range v.a {
			a[i] = x.String()
			if x.kind == KindString {
				a[i] = rustrt.DebugString(x.s)
			}
		}
		return "[" + strings.Join(a, ", ") + "]"
	case KindMap:
		a := make([]string, 0, v.m.Len())
		for _, p := range v.m.entries {
			k := p.Key.String()
			if p.Key.v.kind == KindString {
				k = rustrt.DebugString(k)
			}
			s := p.Value.String()
			if p.Value.kind == KindString {
				s = rustrt.DebugString(s)
			}
			a = append(a, k+": "+s)
		}
		return "{" + strings.Join(a, ", ") + "}"
	}
	return ""
}

// Equal compares values, including numbers of different numeric kinds.
func (v Value) Equal(w Value) bool {
	if v.IsNumber() && w.IsNumber() {
		c, _ := v.PartialCompare(w)
		return c == 0
	}
	if v.kind != w.kind {
		return false
	}
	switch v.kind {
	case KindUndefined, KindNone:
		return true
	case KindBool:
		return v.b == w.b
	case KindString:
		return v.s == w.s
	case KindBytes:
		return bytes.Equal(v.bytes, w.bytes)
	case KindArray:
		if len(v.a) != len(w.a) {
			return false
		}
		for i, x := range v.a {
			if !x.Equal(w.a[i]) {
				return false
			}
		}
		return true
	case KindMap:
		if v.m.Len() != w.m.Len() {
			return false
		}
		for _, p := range v.m.entries {
			x, ok := w.m.Get(p.Key)
			if !ok || !x.Equal(p.Value) {
				return false
			}
		}
		return true
	}
	return false
}

// PartialCompare compares compatible values and reports incompatible types.
func (v Value) PartialCompare(w Value) (int, bool) {
	if v.IsNumber() && w.IsNumber() {
		if v.kind == KindF64 || w.kind == KindF64 {
			if v.kind == KindF64 && w.kind == KindF64 {
				if math.IsNaN(v.f) {
					if math.IsNaN(w.f) {
						return 0, true
					}
					return 1, true
				}
				if math.IsNaN(w.f) {
					return -1, true
				}
				if v.f < w.f {
					return -1, true
				}
				if v.f > w.f {
					return 1, true
				}
				return 0, true
			}
			if v.kind != KindF64 {
				c, ok := w.PartialCompare(v)
				return -c, ok
			}
			if math.IsNaN(v.f) || math.IsInf(v.f, 1) {
				return 1, true
			}
			if math.IsInf(v.f, -1) {
				return -1, true
			}
			x := new(big.Rat).SetFloat64(v.f)
			return x.Cmp(new(big.Rat).SetInt(w.integer())), true
		}
		return v.integer().Cmp(w.integer()), true
	}
	if v.kind != w.kind {
		return 0, false
	}
	switch v.kind {
	case KindNone, KindUndefined:
		return 0, true
	case KindBool:
		if v.b == w.b {
			return 0, true
		}
		if v.b {
			return 1, true
		}
		return -1, true
	case KindString:
		return strings.Compare(v.s, w.s), true
	case KindBytes:
		return bytes.Compare(v.bytes, w.bytes), true
	case KindArray:
		for i := 0; i < len(v.a) && i < len(w.a); i++ {
			c, ok := v.a[i].PartialCompare(w.a[i])
			if !ok {
				return 0, false
			}
			if c != 0 {
				return c, true
			}
		}
		return cmpInt(len(v.a), len(w.a)), true
	}
	return 0, false
}
func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// Compare gives a total order using Tera's type ordering for incompatible values.
func (v Value) Compare(w Value) int {
	if c, ok := v.PartialCompare(w); ok {
		return c
	}
	rank := func(v Value) int {
		switch v.kind {
		case KindBool:
			return 0
		case KindU64, KindI64, KindU128, KindI128, KindF64:
			return 1
		case KindString:
			return 2
		case KindArray:
			return 3
		case KindMap:
			return 4
		case KindBytes:
			return 5
		case KindNone:
			return 6
		}
		return 7
	}
	return cmpInt(rank(v), rank(w))
}

// GetFromPath looks up a dotted path through maps and arrays.
func (v Value) GetFromPath(path string) (Value, bool) {
	if v.IsUndefined() {
		return Undefined(), false
	}
	if v.IsNone() {
		return v, true
	}
	for _, p := range strings.Split(path, ".") {
		if i, e := strconv.ParseUint(p, 10, 64); e == nil {
			if v.kind != KindArray || i >= uint64(len(v.a)) {
				return Undefined(), false
			}
			v = v.a[i]
		} else {
			if v.kind != KindMap {
				return Undefined(), false
			}
			var ok bool
			v, ok = v.m.GetString(p)
			if !ok {
				return Undefined(), false
			}
		}
	}
	return v, true
}
func (v Value) getAttr(attr string) Value {
	if v.kind == KindMap {
		x, _ := v.m.GetString(attr)
		return x
	}
	return Undefined()
}
func (v Value) getItem(k Value) (Value, error) {
	if v.kind == KindMap {
		key, e := KeyFromValue(k)
		if e != nil {
			return Undefined(), ErrorMessage(fmt.Sprintf("Map keys must be strings, integers, or bools, got `%s`.", k.Name()))
		}
		x, _ := v.m.Get(key)
		return x, nil
	}
	if v.kind != KindArray && v.kind != KindString {
		return Undefined(), nil
	}
	n, ok := k.AsI128()
	if !ok {
		if k.kind == KindU128 {
			return Undefined(), nil
		}
		name := "Array"
		if v.kind == KindString {
			name = "String"
		}
		return Undefined(), ErrorMessage(fmt.Sprintf("%s index must be an integer, got `%s`.", name, k.Name()))
	}
	l, _ := v.Len()
	if n.Sign() < 0 {
		n.Add(n, big.NewInt(int64(l)))
	}
	if !n.IsInt64() || n.Sign() < 0 || n.Int64() >= int64(l) {
		return Undefined(), nil
	}
	idx := int(n.Int64())
	if v.kind == KindArray {
		return v.a[idx], nil
	}
	v.s = string([]rune(v.s)[idx])
	return v, nil
}
func (v Value) contains(n Value) (bool, error) {
	switch v.kind {
	case KindArray:
		for _, x := range v.a {
			if x.Equal(n) {
				return true, nil
			}
		}
		return false, nil
	case KindString:
		if n.kind == KindString {
			return strings.Contains(v.s, n.s), nil
		}
		return false, nil
	case KindMap:
		k, e := KeyFromValue(n)
		if e != nil {
			return false, nil
		}
		_, ok := v.m.Get(k)
		return ok, nil
	}
	return false, ErrorMessage(fmt.Sprintf("`in` cannot be used on a container of type `%s`. It can only be used on arrays, strings and map/structs", v.Name()))
}

// Reverse reverses arrays, bytes, and Unicode scalar strings.
func (v Value) Reverse() (Value, error) {
	switch v.kind {
	case KindString:
		r := []rune(v.s)
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		return NormalString(string(r)), nil
	case KindArray:
		a := append([]Value{}, v.a...)
		for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
			a[i], a[j] = a[j], a[i]
		}
		return Array(a), nil
	case KindBytes:
		a := make([]Value, len(v.bytes))
		for i, x := range v.bytes {
			a[len(a)-1-i] = U64(uint64(x))
		}
		return Array(a), nil
	}
	return Undefined(), ErrorMessage("Value of type " + v.Name() + " cannot be reversed")
}
func (v Value) slice(start, end, step *big.Int) (Value, error) {
	s := int64(1)
	if step != nil {
		if step.IsInt64() {
			s = step.Int64()
		} else if step.Sign() < 0 {
			s = math.MinInt64
		} else {
			s = math.MaxInt64
		}
	}
	if s == 0 {
		return Undefined(), ErrorMessage("Slicing step cannot be 0")
	}
	if v.kind != KindArray && v.kind != KindString {
		return Undefined(), ErrorMessage(fmt.Sprintf("Slicing can only be used on arrays or strings, not on `%s`.", v.Name()))
	}
	l, _ := v.Len()
	lo, hi := int64(0), int64(l)
	if s < 0 {
		lo = -1
		hi = int64(l) - 1
	}
	resolve := func(p *big.Int, d int64) int64 {
		if p == nil {
			return d
		}
		n := new(big.Int).Set(p)
		if n.Sign() < 0 {
			n.Add(n, big.NewInt(int64(l)))
		}
		if n.Cmp(big.NewInt(lo)) < 0 {
			return lo
		}
		if n.Cmp(big.NewInt(hi)) > 0 {
			return hi
		}
		return n.Int64()
	}
	a, b := resolve(start, lo), resolve(end, hi)
	if s < 0 {
		a, b = resolve(start, hi), resolve(end, lo)
	}
	out := []Value{}
	var str strings.Builder
	r := []rune(v.s)
	for i := a; (s > 0 && i < b) || (s < 0 && i > b); {
		if v.kind == KindArray {
			out = append(out, v.a[i])
		} else {
			str.WriteRune(r[i])
		}
		if s > 0 && i > math.MaxInt64-s || s < 0 && i < math.MinInt64-s {
			break
		}
		i += s
	}
	if v.kind == KindArray {
		return Array(out), nil
	}
	v.s = str.String()
	return v, nil
}

// IsUndefined checks the value variant.
func (v Value) IsUndefined() bool { return v.kind == KindUndefined }

// IsNone checks the value variant.
func (v Value) IsNone() bool { return v.kind == KindNone }

// IsBool checks the value variant.
func (v Value) IsBool() bool { return v.kind == KindBool }

// IsU64 checks the value variant.
func (v Value) IsU64() bool { return v.kind == KindU64 }

// IsI64 checks the value variant.
func (v Value) IsI64() bool { return v.kind == KindI64 }

// IsU128 checks the value variant.
func (v Value) IsU128() bool { return v.kind == KindU128 }

// IsI128 checks the value variant.
func (v Value) IsI128() bool { return v.kind == KindI128 }

// IsF64 checks the value variant.
func (v Value) IsF64() bool { return v.kind == KindF64 }

// IsString checks the value variant.
func (v Value) IsString() bool { return v.kind == KindString }

// IsArray checks the value variant.
func (v Value) IsArray() bool { return v.kind == KindArray }

// IsMap checks the value variant.
func (v Value) IsMap() bool { return v.kind == KindMap }

// IsBytes checks the value variant.
func (v Value) IsBytes() bool { return v.kind == KindBytes }

// IsNumber checks for any numeric variant.
func (v Value) IsNumber() bool { return v.kind >= KindU64 && v.kind <= KindF64 }
