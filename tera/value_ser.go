package tera

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/Keats/tera/internal/rustrt"
)

// ValueFrom converts Go serializable values while preserving Value safety and numeric kinds.
func ValueFrom(v any) (Value, error) { return fromReflect(reflect.ValueOf(v), 0) }
func fromReflect(r reflect.Value, depth int) (Value, error) {
	if depth > 128 {
		return Undefined(), ErrorMessage("serialization failed: recursion limit exceeded")
	}
	if !r.IsValid() {
		return None(), nil
	}
	if (r.Kind() == reflect.Pointer || r.Kind() == reflect.Interface) && r.IsNil() {
		return None(), nil
	}
	if r.CanInterface() {
		switch x := r.Interface().(type) {
		case Value:
			return x, nil
		case Number:
			return x.IntoValue(), nil
		case Key:
			return x.AsValue(), nil
		case Map:
			return MapValue(&x), nil
		case *Map:
			return MapValue(x), nil
		case *big.Int:
			return I128(x), nil
		case big.Int:
			return I128(&x), nil
		case json.Number:
			return jsonNumber(string(x))
		case []byte:
			return Bytes(x), nil
		}
	}
	switch r.Kind() {
	case reflect.Interface, reflect.Pointer:
		if r.IsNil() {
			return None(), nil
		}
		return fromReflect(r.Elem(), depth+1)
	case reflect.Bool:
		return Bool(r.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return I64(r.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return U64(r.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return F64(r.Float()), nil
	case reflect.String:
		return NormalString(r.String()), nil
	case reflect.Array, reflect.Slice:
		a := make([]Value, r.Len())
		for i := range a {
			x, e := fromReflect(r.Index(i), depth+1)
			if e != nil {
				return Undefined(), e
			}
			a[i] = x
		}
		return Array(a), nil
	case reflect.Map:
		m := NewMap()
		ks := r.MapKeys()
		type pair struct {
			k Key
			r reflect.Value
		}
		ps := make([]pair, 0, len(ks))
		for _, k := range ks {
			x, e := fromReflect(k, depth+1)
			if e != nil {
				return Undefined(), e
			}
			key, e := KeyFromValue(x)
			if e != nil {
				return Undefined(), ErrorMessage("serialization failed: map key must be string, integer, or bool")
			}
			ps = append(ps, pair{key, r.MapIndex(k)})
		}
		sort.Slice(ps, func(i, j int) bool { return ps[i].k.Compare(ps[j].k) < 0 })
		for _, p := range ps {
			x, e := fromReflect(p.r, depth+1)
			if e != nil {
				return Undefined(), e
			}
			m.Insert(p.k, x)
		}
		return MapValue(m), nil
	case reflect.Struct:
		m := NewMap()
		for i := 0; i < r.NumField(); i++ {
			f := r.Type().Field(i)
			if f.PkgPath != "" {
				continue
			}
			name := f.Name
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if tag == "-" {
				continue
			}
			if tag != "" {
				name = tag
			}
			x, e := fromReflect(r.Field(i), depth+1)
			if e != nil {
				return Undefined(), e
			}
			m.InsertString(name, x)
		}
		return MapValue(m), nil
	}
	return Undefined(), ErrorMessage("serialization failed: unsupported type " + r.Type().String())
}
func jsonNumber(s string) (Value, error) {
	positive := !strings.HasPrefix(s, "-")
	text := strings.TrimPrefix(s, "-")
	if !strings.ContainsAny(text, ".eE") {
		if n, e := strconv.ParseUint(text, 10, 64); e == nil {
			if positive {
				return U64(n), nil
			}
			if n > 0 && n <= uint64(1)<<63 {
				return I64(-int64(n)), nil
			}
			return F64(-float64(n)), nil
		}
	}
	var sig uint64
	exponent := 0
	i := 0
	overflow := false
	for i < len(text) && text[i] >= '0' && text[i] <= '9' {
		digit := uint64(text[i] - '0')
		if overflow || sig > (^uint64(0)-digit)/10 {
			overflow = true
			exponent++
		} else {
			sig = sig*10 + digit
		}
		i++
	}
	if i < len(text) && text[i] == '.' {
		i++
		overflow = false
		for i < len(text) && text[i] >= '0' && text[i] <= '9' {
			digit := uint64(text[i] - '0')
			if overflow || sig > (^uint64(0)-digit)/10 {
				overflow = true
			} else {
				sig = sig*10 + digit
				exponent--
			}
			i++
		}
	}
	if i < len(text) && (text[i] == 'e' || text[i] == 'E') {
		i++
		neg := false
		if i < len(text) && (text[i] == '+' || text[i] == '-') {
			neg = text[i] == '-'
			i++
		}
		e := 0
		for ; i < len(text); i++ {
			if e < 100000 {
				e = e*10 + int(text[i]-'0')
			}
		}
		if neg {
			exponent -= e
		} else {
			exponent += e
		}
	}
	f := float64(sig)
	for exponent < -308 && f != 0 {
		f /= 1e308
		exponent += 308
	}
	if exponent > 308 {
		if sig != 0 {
			return Undefined(), fmt.Errorf("number out of range")
		}
	} else if exponent >= 0 {
		f *= jsonPowers[exponent]
	} else if exponent >= -308 {
		f /= jsonPowers[-exponent]
	}
	if math.IsInf(f, 0) {
		return Undefined(), fmt.Errorf("number out of range")
	}
	if !positive {
		f = -f
	}
	return F64(f), nil
}

var jsonPowers = func() [309]float64 {
	var a [309]float64
	for i := range a {
		a[i], _ = strconv.ParseFloat("1e"+strconv.Itoa(i), 64)
	}
	return a
}()

// MarshalJSON serializes a Value like serde_json, optionally using two-space indentation.
func MarshalJSON(v Value, pretty bool) (string, error) {
	var b strings.Builder
	marshalValue(&b, v, pretty, 0)
	return b.String(), nil
}
func jsonQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if c < 32 {
				fmt.Fprintf(&b, `\u%04x`, c)
			} else {
				b.WriteRune(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
func marshalValue(b *strings.Builder, v Value, p bool, d int) {
	switch v.kind {
	case KindNone, KindUndefined:
		b.WriteString("null")
	case KindString:
		b.WriteString(jsonQuote(v.s))
	case KindF64:
		b.WriteString(rustrt.JSONFloat(v.f))
	case KindArray, KindBytes:
		open, close := "[", "]"
		b.WriteString(open)
		a := v.a
		if v.kind == KindBytes {
			a = make([]Value, len(v.bytes))
			for i, x := range v.bytes {
				a[i] = U64(uint64(x))
			}
		}
		for i, x := range a {
			if i > 0 {
				b.WriteByte(',')
			}
			if p {
				b.WriteByte('\n')
				b.WriteString(strings.Repeat("  ", d+1))
			}
			marshalValue(b, x, p, d+1)
		}
		if p && len(a) > 0 {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat("  ", d))
		}
		b.WriteString(close)
	case KindMap:
		b.WriteByte('{')
		for i, x := range v.m.entries {
			if i > 0 {
				b.WriteByte(',')
			}
			if p {
				b.WriteByte('\n')
				b.WriteString(strings.Repeat("  ", d+1))
			}
			b.WriteString(jsonQuote(x.Key.String()))
			b.WriteByte(':')
			if p {
				b.WriteByte(' ')
			}
			marshalValue(b, x.Value, p, d+1)
		}
		if p && v.m.Len() > 0 {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat("  ", d))
		}
		b.WriteByte('}')
	default:
		b.WriteString(v.String())
	}
}
