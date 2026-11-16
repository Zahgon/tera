package tera

import (
	"iter"
	"math"
	"math/big"
)

// Filter transforms a value.
type Filter interface {
	// Call transforms a value using the supplied arguments and rendering state.
	Call(Value, Kwargs, *State) (Value, error)
}

// Test evaluates a predicate.
type Test interface {
	// Call tests a value using the supplied arguments and rendering state.
	Call(Value, Kwargs, *State) (bool, error)
}

// Function produces a value from keyword arguments.
type Function interface {
	// Call computes a value from arguments and rendering state.
	Call(Kwargs, *State) (Value, error)
}

// FilterFunc adapts a Go function to Filter.
type FilterFunc func(Value, Kwargs, *State) (Value, error)

// Call invokes the filter.
func (f FilterFunc) Call(v Value, k Kwargs, s *State) (Value, error) { return f(v, k, s) }

// TestFunc adapts a Go predicate to Test.
type TestFunc func(Value, Kwargs, *State) (bool, error)

// Call invokes the test.
func (f TestFunc) Call(v Value, k Kwargs, s *State) (bool, error) { return f(v, k, s) }

// FunctionFunc adapts a Go function to Function.
type FunctionFunc func(Kwargs, *State) (Value, error)

// Call invokes the function.
func (f FunctionFunc) Call(k Kwargs, s *State) (Value, error) { return f(k, s) }

// TypedFilter converts its first argument using Rust argument conversion rules.
func TypedFilter[A any](conv func(Value) (A, error), f func(A, Kwargs, *State) (Value, error)) Filter {
	return FilterFunc(func(v Value, k Kwargs, s *State) (Value, error) {
		a, e := conv(v)
		if e != nil {
			return Undefined(), e
		}
		return f(a, k, s)
	})
}

// TypedTest converts its first argument using Rust argument conversion rules.
func TypedTest[A any](conv func(Value) (A, error), f func(A, Kwargs, *State) (bool, error)) Test {
	return TestFunc(func(v Value, k Kwargs, s *State) (bool, error) {
		a, e := conv(v)
		if e != nil {
			return false, e
		}
		return f(a, k, s)
	})
}

// FunctionResult converts a Go result to a Value.
func FunctionResult[T any](v T, err error) (Value, error) {
	if err != nil {
		return Undefined(), err
	}
	return ValueFrom(v)
}

// Kwargs contains ordered keyword arguments.
type Kwargs struct{ values *Map }

// NewKwargs constructs keyword arguments from a map.
func NewKwargs(m *Map) Kwargs { return Kwargs{m} }

// Get retrieves an unconverted keyword argument.
func (k Kwargs) Get(n string) (Value, bool) { return k.values.GetString(n) }

// Iter iterates arguments in insertion order.
func (k Kwargs) Iter() iter.Seq2[Key, Value] {
	return func(yield func(Key, Value) bool) {
		for _, p := range k.values.Iter() {
			if !yield(p.Key, p.Value) {
				return
			}
		}
	}
}

// KwargsGet retrieves and converts an optional keyword argument.
func KwargsGet[T any](k Kwargs, n string, conv func(Value) (T, error)) (T, bool, error) {
	var z T
	v, ok := k.Get(n)
	if !ok {
		return z, false, nil
	}
	x, e := conv(v)
	return x, true, e
}

// KwargsMustGet retrieves and converts a required keyword argument.
func KwargsMustGet[T any](k Kwargs, n string, conv func(Value) (T, error)) (T, error) {
	x, ok, e := KwargsGet(k, n, conv)
	if e != nil {
		return x, e
	}
	if !ok {
		return x, NewError(KindMissingArgument{n})
	}
	return x, nil
}

// ArgBool converts only boolean values.
func ArgBool(v Value) (bool, error) {
	if v.IsBool() {
		return v.b, nil
	}
	return false, invalidArg("bool", v)
}

// ArgF64 converts numeric values to f64.
func ArgF64(v Value) (float64, error) {
	if v.IsF64() {
		return v.f, nil
	}
	if n := v.integer(); n != nil {
		f, _ := new(big.Float).SetInt(n).Float64()
		return f, nil
	}
	return 0, invalidArg("f64", v)
}

// ArgF32 converts numeric values, detecting finite overflow.
func ArgF32(v Value) (float32, error) {
	f, e := ArgF64(v)
	if e != nil {
		return 0, invalidArg("f32", v)
	}
	x := float32(f)
	if math.IsInf(float64(x), 0) && !math.IsInf(f, 0) {
		return 0, outOfRange("f32", v)
	}
	return x, nil
}
func argInt(v Value, t string, bits uint, signed bool) (*big.Int, error) {
	n := v.integer()
	if n == nil {
		if !v.IsF64() || math.Trunc(v.f) != v.f {
			return nil, invalidArg(t, v)
		}
		if math.IsInf(v.f, 0) || v.f < -math.Exp2(127) || v.f >= math.Exp2(127) {
			return nil, outOfRange(t, v)
		}
		n, _ = new(big.Float).SetFloat64(v.f).Int(nil)
	}
	lo := big.NewInt(0)
	hi := new(big.Int).Lsh(big.NewInt(1), bits)
	if signed {
		hi.Rsh(hi, 1)
		lo.Neg(hi)
	}
	if n.Cmp(lo) < 0 || n.Cmp(hi) >= 0 {
		return nil, outOfRange(t, v)
	}
	return n, nil
}

// ArgString formats any value into an owned string.
func ArgString(v Value) (string, error) { return v.String(), nil }

// ArgStr accepts only a string.
func ArgStr(v Value) (string, error) {
	if v.IsString() {
		return v.s, nil
	}
	return "", invalidArg("&str", v)
}

// ArgCowStr formats values as a string.
func ArgCowStr(v Value) (string, error) { return v.String(), nil }

// ArgValue returns a value unchanged.
func ArgValue(v Value) (Value, error) { return v, nil }

// ArgValueRef returns a value unchanged.
func ArgValueRef(v Value) (Value, error) { return v, nil }

// ArgNumber converts an arithmetic number.
func ArgNumber(v Value) (Number, error) {
	if n, ok := v.AsNumber(); ok {
		return n, nil
	}
	if v.IsNumber() {
		return Number{}, ErrorMessage("Number `" + v.String() + "` is out of range for i128")
	}
	return Number{}, invalidArg("Number", v)
}

// ArgMap converts an ordered map.
func ArgMap(v Value) (*Map, error) {
	if v.IsMap() {
		return v.m.Clone(), nil
	}
	return nil, invalidArg("Map", v)
}

// ArgMapRef returns an ordered map.
func ArgMapRef(v Value) (*Map, error) {
	if v.IsMap() {
		return v.m, nil
	}
	return nil, invalidArg("Map", v)
}

// ArgVec converts an array element by element.
func ArgVec[T any](conv func(Value) (T, error)) func(Value) ([]T, error) {
	return func(v Value) ([]T, error) {
		if !v.IsArray() {
			return nil, invalidArg("Vec<Value>", v)
		}
		a := make([]T, len(v.a))
		for i, x := range v.a {
			y, e := conv(x)
			if e != nil {
				return nil, e
			}
			a[i] = y
		}
		return a, nil
	}
}

// ArgValueSlice accepts only arrays.
func ArgValueSlice(v Value) ([]Value, error) {
	if v.IsArray() {
		return v.a, nil
	}
	return nil, invalidArg("&[Value]", v)
}

// StringInput carries string content and safety.
type StringInput struct{ inner Value }

// ArgStringInput accepts only strings and preserves safety.
func ArgStringInput(v Value) (StringInput, error) {
	if v.IsString() {
		return StringInput{v}, nil
	}
	return StringInput{}, invalidArg("string", v)
}

// AsStr returns the string content.
func (s StringInput) AsStr() string { return s.inner.s }

// IsSafe returns its safety flag.
func (s StringInput) IsSafe() bool { return s.inner.IsSafe() }

// InheritSafety constructs a string inheriting the input safety.
func (s StringInput) InheritSafety(v string) Value {
	if s.IsSafe() {
		return SafeString(v)
	}
	return NormalString(v)
}

// IntoValue returns the original value.
func (s StringInput) IntoValue() Value { return s.inner }

// Rendered escapes the string according to the current state.
func (s StringInput) Rendered(st *State) (string, error) {
	if s.IsSafe() || !st.AutoescapingEnabled() {
		return s.AsStr(), nil
	}
	return st.Escape(s.AsStr())
}

// ArgU8 converts a numeric argument to u8.
func ArgU8(v Value) (uint8, error) {
	n, e := argInt(v, "u8", 8, false)
	if e != nil {
		return 0, e
	}
	return uint8(n.Uint64()), nil
}

// ArgU16 converts a numeric argument to u16.
func ArgU16(v Value) (uint16, error) {
	n, e := argInt(v, "u16", 16, false)
	if e != nil {
		return 0, e
	}
	return uint16(n.Uint64()), nil
}

// ArgU32 converts a numeric argument to u32.
func ArgU32(v Value) (uint32, error) {
	n, e := argInt(v, "u32", 32, false)
	if e != nil {
		return 0, e
	}
	return uint32(n.Uint64()), nil
}

// ArgU64 converts a numeric argument to u64.
func ArgU64(v Value) (uint64, error) {
	n, e := argInt(v, "u64", 64, false)
	if e != nil {
		return 0, e
	}
	return uint64(n.Uint64()), nil
}

// ArgU128 converts a numeric argument to u128.
func ArgU128(v Value) (*big.Int, error) {
	n, e := argInt(v, "u128", 128, false)
	if e != nil {
		return nil, e
	}
	return n, nil
}

// ArgUsize converts a numeric argument to usize.
func ArgUsize(v Value) (uint, error) {
	n, e := argInt(v, "usize", 64, false)
	if e != nil {
		return 0, e
	}
	return uint(n.Uint64()), nil
}

// ArgI8 converts a numeric argument to i8.
func ArgI8(v Value) (int8, error) {
	n, e := argInt(v, "i8", 8, true)
	if e != nil {
		return 0, e
	}
	return int8(n.Int64()), nil
}

// ArgI16 converts a numeric argument to i16.
func ArgI16(v Value) (int16, error) {
	n, e := argInt(v, "i16", 16, true)
	if e != nil {
		return 0, e
	}
	return int16(n.Int64()), nil
}

// ArgI32 converts a numeric argument to i32.
func ArgI32(v Value) (int32, error) {
	n, e := argInt(v, "i32", 32, true)
	if e != nil {
		return 0, e
	}
	return int32(n.Int64()), nil
}

// ArgI64 converts a numeric argument to i64.
func ArgI64(v Value) (int64, error) {
	n, e := argInt(v, "i64", 64, true)
	if e != nil {
		return 0, e
	}
	return int64(n.Int64()), nil
}

// ArgI128 converts a numeric argument to i128.
func ArgI128(v Value) (*big.Int, error) {
	n, e := argInt(v, "i128", 128, true)
	if e != nil {
		return nil, e
	}
	return n, nil
}

// ArgIsize converts a numeric argument to isize.
func ArgIsize(v Value) (int, error) {
	n, e := argInt(v, "isize", 64, true)
	if e != nil {
		return 0, e
	}
	return int(n.Int64()), nil
}

// SafeOutput is optionally implemented by filters and functions whose strings are always safe.
type SafeOutput interface {
	// IsSafe exempts returned strings from autoescaping.
	IsSafe() bool
}

// TypedFilterResult adapts a typed filter with any serializable return value.
func TypedFilterResult[A, R any](conv func(Value) (A, error), f func(A, Kwargs, *State) (R, error)) Filter {
	return TypedFilter(conv, func(a A, k Kwargs, s *State) (Value, error) { v, e := f(a, k, s); return FunctionResult(v, e) })
}

// TypedFilterValue adapts a typed infallible filter with any serializable return value.
func TypedFilterValue[A, R any](conv func(Value) (A, error), f func(A, Kwargs, *State) R) Filter {
	return TypedFilter(conv, func(a A, k Kwargs, s *State) (Value, error) { return ValueFrom(f(a, k, s)) })
}

// TypedFunction adapts a function with any serializable return value.
func TypedFunction[R any](f func(Kwargs, *State) (R, error)) Function {
	return FunctionFunc(func(k Kwargs, s *State) (Value, error) { v, e := f(k, s); return FunctionResult(v, e) })
}

// FunctionValue adapts an infallible function with any serializable return value.
func FunctionValue[R any](f func(Kwargs, *State) R) Function {
	return FunctionFunc(func(k Kwargs, s *State) (Value, error) { return ValueFrom(f(k, s)) })
}

// TypedTestValue adapts a typed infallible predicate.
func TypedTestValue[A any](conv func(Value) (A, error), f func(A, Kwargs, *State) bool) Test {
	return TypedTest(conv, func(a A, k Kwargs, s *State) (bool, error) { return f(a, k, s), nil })
}
