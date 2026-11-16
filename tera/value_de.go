package tera

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Keats/tera/internal/rustrt"
)

// Deserialize converts a Value into a Go value using strict serde-style type conversion.
// The destination must be a non-nil pointer. Struct fields use their json tag names.
func (v Value) Deserialize(out any) error {
	r := reflect.ValueOf(out)
	if !r.IsValid() || r.Kind() != reflect.Pointer || r.IsNil() {
		return ErrorMessage("deserialization failed: destination must be a non-nil pointer")
	}
	if e := deserializeReflect(v, r.Elem(), 0); e != nil {
		return ErrorMessage("deserialization failed: " + e.Error())
	}
	return nil
}

// ValueAs deserializes a value into a typed Go result.
func ValueAs[T any](v Value) (T, error) { var out T; e := v.Deserialize(&out); return out, e }

// Deserialize converts keyword arguments into a Go value.
func (k Kwargs) Deserialize(out any) error { return MapValue(k.values).Deserialize(out) }

// KwargsDeserialize deserializes keyword arguments into a typed Go result.
func KwargsDeserialize[T any](k Kwargs) (T, error) { return ValueAs[T](MapValue(k.values)) }

func unexpectedValue(v Value) string {
	switch v.kind {
	case KindUndefined, KindNone:
		return "unit value"
	case KindBool:
		return fmt.Sprintf("boolean `%t`", v.b)
	case KindU64, KindI64, KindI128, KindU128:
		return "integer `" + v.String() + "`"
	case KindF64:
		return "floating point `" + rustrt.FloatDisplay(v.f) + "`"
	case KindString:
		return "string " + rustrt.DebugString(v.s)
	case KindBytes:
		return "byte array"
	case KindArray:
		return "sequence"
	case KindMap:
		return "map"
	}
	return v.Name()
}
func decodeExpected(r reflect.Value) string {
	switch r.Kind() {
	case reflect.Bool:
		return "a boolean"
	case reflect.String:
		return "a string"
	case reflect.Int:
		return "isize"
	case reflect.Int8:
		return "i8"
	case reflect.Int16:
		return "i16"
	case reflect.Int32:
		return "i32"
	case reflect.Int64:
		return "i64"
	case reflect.Uint:
		return "usize"
	case reflect.Uint8:
		return "u8"
	case reflect.Uint16:
		return "u16"
	case reflect.Uint32:
		return "u32"
	case reflect.Uint64:
		return "u64"
	case reflect.Float32:
		return "f32"
	case reflect.Float64:
		return "f64"
	case reflect.Array, reflect.Slice:
		return "a sequence"
	case reflect.Map:
		return "a map"
	case reflect.Struct:
		return "struct " + r.Type().Name()
	}
	return r.Type().String()
}
func deserializeReflect(v Value, r reflect.Value, depth int) error {
	if depth > 128 {
		return fmt.Errorf("recursion limit exceeded")
	}
	if r.Type() == reflect.TypeOf(Value{}) {
		r.Set(reflect.ValueOf(v))
		return nil
	}
	if r.Type() == reflect.TypeOf(Map{}) {
		if !v.IsMap() {
			return fmt.Errorf("invalid type: %s, expected a map", unexpectedValue(v))
		}
		r.Set(reflect.ValueOf(*v.m.Clone()))
		return nil
	}
	if r.Kind() == reflect.Pointer {
		if v.IsNone() || v.IsUndefined() {
			r.SetZero()
			return nil
		}
		if r.IsNil() {
			r.Set(reflect.New(r.Type().Elem()))
		}
		return deserializeReflect(v, r.Elem(), depth+1)
	}
	invalid := func() error {
		return fmt.Errorf("invalid type: %s, expected %s", unexpectedValue(v), decodeExpected(r))
	}
	switch r.Kind() {
	case reflect.Interface:
		if v.IsNone() || v.IsUndefined() {
			r.SetZero()
			return nil
		}
		decoded := reflect.ValueOf(valueInterface(v))
		if !decoded.Type().AssignableTo(r.Type()) {
			return invalid()
		}
		r.Set(decoded)
		return nil
	case reflect.Bool:
		if !v.IsBool() {
			return invalid()
		}
		r.SetBool(v.b)
		return nil
	case reflect.String:
		if !v.IsString() {
			return invalid()
		}
		r.SetString(v.s)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n := v.integer()
		if n == nil {
			return invalid()
		}
		if !n.IsInt64() || r.OverflowInt(n.Int64()) {
			return fmt.Errorf("invalid value: %s, expected %s", unexpectedValue(v), decodeExpected(r))
		}
		r.SetInt(n.Int64())
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n := v.integer()
		if n == nil {
			return invalid()
		}
		if !n.IsUint64() || r.OverflowUint(n.Uint64()) {
			return fmt.Errorf("invalid value: %s, expected %s", unexpectedValue(v), decodeExpected(r))
		}
		r.SetUint(n.Uint64())
		return nil
	case reflect.Float32, reflect.Float64:
		f, e := ArgF64(v)
		if e != nil {
			return invalid()
		}
		r.SetFloat(f)
		return nil
	case reflect.Array, reflect.Slice:
		if !v.IsArray() {
			return invalid()
		}
		if r.Kind() == reflect.Array && len(v.a) != r.Len() {
			return fmt.Errorf("invalid length %d, expected an array of length %d", len(v.a), r.Len())
		}
		if r.Kind() == reflect.Slice {
			r.Set(reflect.MakeSlice(r.Type(), len(v.a), len(v.a)))
		}
		for i, x := range v.a {
			if e := deserializeReflect(x, r.Index(i), depth+1); e != nil {
				return e
			}
		}
		return nil
	case reflect.Map:
		if !v.IsMap() {
			return invalid()
		}
		r.Set(reflect.MakeMapWithSize(r.Type(), v.m.Len()))
		for _, p := range v.m.Iter() {
			key := reflect.New(r.Type().Key()).Elem()
			if e := deserializeReflect(p.Key.AsValue(), key, depth+1); e != nil {
				return e
			}
			val := reflect.New(r.Type().Elem()).Elem()
			if e := deserializeReflect(p.Value, val, depth+1); e != nil {
				return e
			}
			r.SetMapIndex(key, val)
		}
		return nil
	case reflect.Struct:
		if !v.IsMap() {
			return invalid()
		}
		for i := 0; i < r.NumField(); i++ {
			field := r.Type().Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			val, ok := v.m.GetString(name)
			if !ok {
				if r.Field(i).Kind() == reflect.Pointer {
					r.Field(i).SetZero()
					continue
				}
				return fmt.Errorf("missing field `%s`", name)
			}
			if e := deserializeReflect(val, r.Field(i), depth+1); e != nil {
				return e
			}
		}
		return nil
	}
	return invalid()
}
func valueInterface(v Value) any {
	switch v.kind {
	case KindUndefined, KindNone:
		return nil
	case KindBool:
		return v.b
	case KindU64:
		return v.u
	case KindI64:
		return v.i
	case KindU128, KindI128:
		return v.integer()
	case KindF64:
		return v.f
	case KindString:
		return v.s
	case KindBytes:
		return append([]byte{}, v.bytes...)
	case KindArray:
		a := make([]any, len(v.a))
		for i, x := range v.a {
			a[i] = valueInterface(x)
		}
		return a
	case KindMap:
		m := map[any]any{}
		for _, p := range v.m.Iter() {
			key := valueInterface(p.Key.AsValue())
			if p.Key.v.IsI128() || p.Key.v.IsU128() {
				key = p.Key.String()
			}
			m[key] = valueInterface(p.Value)
		}
		return m
	}
	return nil
}
