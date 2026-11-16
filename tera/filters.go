package tera

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Keats/tera/internal/rustrt"
)

func kwDefault[T any](k Kwargs, n string, conv func(Value) (T, error), d T) (T, error) {
	v, ok, e := KwargsGet(k, n, conv)
	if !ok {
		return d, e
	}
	return v, e
}
func (t *Tera) registerBuiltins() {
	for _, name := range strings.Fields("safe default upper lower wordcount escape escape_html escape_xml newlines_to_br pluralize trim trim_start trim_end replace capitalize title truncate indent str int float length reverse split abs round first last nth join sort unique get values keys pairs group_by") {
		n := name
		t.RegisterFilter(n, FilterFunc(func(v Value, k Kwargs, s *State) (Value, error) { return builtinFilter(n, v, k, s) }))
	}
	t.registerTests()
	t.registerFunctions()
}
func builtinFilter(name string, v Value, k Kwargs, st *State) (Value, error) {
	switch name {
	case "safe":
		return SafeString(v.String()), nil
	case "str":
		return NormalString(v.String()), nil
	case "default":
		d, e := KwargsMustGet(k, "value", ArgValue)
		if e != nil {
			return Undefined(), e
		}
		b, e := kwDefault(k, "boolean", ArgBool, false)
		if e != nil {
			return Undefined(), e
		}
		if b && !v.IsTruthy() || !b && v.IsUndefined() {
			return d, nil
		}
		return v, nil
	case "upper", "lower", "trim", "trim_start", "trim_end", "replace", "capitalize", "title", "indent", "escape":
		s, e := ArgStringInput(v)
		if e != nil {
			return Undefined(), e
		}
		text := s.AsStr()
		switch name {
		case "upper":
			return s.InheritSafety(rustrt.Upper(text)), nil
		case "lower":
			return s.InheritSafety(rustrt.Lower(text)), nil
		case "capitalize":
			r, z := utf8.DecodeRuneInString(text)
			if z == 0 {
				return s.InheritSafety(""), nil
			}
			return s.InheritSafety(rustrt.UpperChar(r) + rustrt.Lower(text[z:])), nil
		case "title":
			var b strings.Builder
			cap := true
			for _, c := range text {
				if c < 128 && strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", c) || unicode.IsSpace(c) {
					b.WriteRune(c)
					if c != '\'' {
						cap = true
					}
				} else if cap {
					b.WriteString(rustrt.UpperChar(c))
					cap = false
				} else {
					b.WriteString(rustrt.LowerChar(c))
				}
			}
			return s.InheritSafety(b.String()), nil
		case "escape":
			if s.IsSafe() {
				return v, nil
			}
			x, e := st.Escape(text)
			return SafeString(x), e
		case "trim", "trim_start", "trim_end":
			pat, ok, e := KwargsGet(k, "pat", ArgStr)
			if e != nil {
				return Undefined(), e
			}
			if ok {
				if pat != "" {
					if name != "trim_end" {
						for strings.HasPrefix(text, pat) {
							text = strings.TrimPrefix(text, pat)
						}
					}
					if name != "trim_start" {
						for strings.HasSuffix(text, pat) {
							text = strings.TrimSuffix(text, pat)
						}
					}
				}
			} else {
				if name != "trim_end" {
					text = rustrt.TrimStart(text)
				}
				if name != "trim_start" {
					text = rustrt.TrimEnd(text)
				}
			}
			return s.InheritSafety(text), nil
		case "replace":
			from, e := KwargsMustGet(k, "from", ArgStr)
			if e != nil {
				return Undefined(), e
			}
			to, e := KwargsMustGet(k, "to", ArgStringInput)
			if e != nil {
				return Undefined(), e
			}
			safe := st.autoescape && (s.IsSafe() || to.IsSafe())
			rep := to.AsStr()
			if safe {
				text, e = s.Rendered(st)
				if e != nil {
					return Undefined(), e
				}
				rep, e = to.Rendered(st)
				if e != nil {
					return Undefined(), e
				}
			}
			out := NormalString(rustReplace(text, from, rep))
			if safe {
				out = out.MarkSafe()
			}
			return out, nil
		case "indent":
			width, e := kwDefault(k, "width", ArgUsize, uint(4))
			if e != nil {
				return Undefined(), e
			}
			if width > 1000 {
				width = 1000
			}
			ind, has, e := KwargsGet(k, "indentation", ArgStringInput)
			if e != nil {
				return Undefined(), e
			}
			first, e := kwDefault(k, "first", ArgBool, false)
			if e != nil {
				return Undefined(), e
			}
			blank, e := kwDefault(k, "blank", ArgBool, false)
			if e != nil {
				return Undefined(), e
			}
			if has && utf8.RuneCountInString(ind.AsStr()) != 1 {
				return Undefined(), ErrorMessage("The `indentation` argument must contain exactly one character")
			}
			safe := st.autoescape && (s.IsSafe() || has && ind.IsSafe())
			indent := " "
			if has {
				indent = ind.AsStr()
			}
			if safe {
				text, e = s.Rendered(st)
				if e != nil {
					return Undefined(), e
				}
				if has {
					indent, e = ind.Rendered(st)
					if e != nil {
						return Undefined(), e
					}
				}
			}
			indent = strings.Repeat(indent, int(width))
			a := rustrt.Lines(text)
			for i := range a {
				if i == 0 && first || i > 0 && (a[i] != "" || blank) {
					a[i] = indent + a[i]
				}
			}
			out := strings.Join(a, "\n")
			if strings.HasSuffix(text, "\n") {
				out += "\n"
			}
			if safe {
				return SafeString(out), nil
			}
			return NormalString(out), nil
		}
	case "wordcount", "escape_html", "escape_xml", "newlines_to_br", "truncate", "split":
		text, e := ArgStr(v)
		if e != nil {
			return Undefined(), e
		}
		switch name {
		case "wordcount":
			return U64(uint64(len(strings.Fields(text)))), nil
		case "escape_html":
			var b strings.Builder
			e := EscapeHTML(text, &b)
			return NormalString(b.String()), e
		case "escape_xml":
			return NormalString(strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;").Replace(text)), nil
		case "newlines_to_br":
			return NormalString(strings.NewReplacer("\r\n", "<br>", "\n", "<br>", "\r", "<br>").Replace(text)), nil
		case "truncate":
			length, e := KwargsMustGet(k, "length", ArgUsize)
			if e != nil {
				return Undefined(), e
			}
			end, e := kwDefault(k, "end", ArgStr, "…")
			if e != nil {
				return Undefined(), e
			}
			r := []rune(text)
			if length < uint(len(r)) {
				return NormalString(string(r[:length]) + end), nil
			}
			return NormalString(text), nil
		case "split":
			pat, e := KwargsMustGet(k, "pat", ArgStr)
			if e != nil {
				return Undefined(), e
			}
			a := strings.Split(text, pat)
			if pat == "" {
				a = append([]string{""}, a...)
				a = append(a, "")
			}
			vs := []Value{}
			for _, x := range a {
				vs = append(vs, NormalString(x))
			}
			return Array(vs), nil
		}
	case "pluralize":
		sing, e := kwDefault(k, "singular", ArgStr, "")
		if e != nil {
			return Undefined(), e
		}
		plural, e := kwDefault(k, "plural", ArgStr, "s")
		if e != nil {
			return Undefined(), e
		}
		n, ok := v.AsI128()
		if !ok {
			return Undefined(), ErrorMessage("pluralize filter requires an integer, got `" + v.Name() + "`")
		}
		if new(big.Int).Abs(n).Cmp(big.NewInt(1)) == 0 {
			return NormalString(sing), nil
		}
		return NormalString(plural), nil
	case "int":
		return filterInt(v, k)
	case "float":
		if v.IsString() {
			s := rustrt.Trim(v.s)
			f, e := rustrt.ParseFloat(s)
			if e != nil {
				return Undefined(), ErrorMessage("The string `" + s + "` cannot be converted to a float")
			}
			return F64(f), nil
		}
		if n, ok := v.AsNumber(); ok {
			return F64(n.AsFloat()), nil
		}
		return Undefined(), ErrorMessage("Value of type " + v.Name() + " cannot be converted to a float")
	case "length":
		if n, ok := v.Len(); ok {
			return U64(uint64(n)), nil
		}
		return Undefined(), ErrorMessage("Value of type " + v.Name() + " has no length")
	case "reverse":
		return v.Reverse()
	case "abs":
		if v.IsU64() || v.IsU128() {
			return v, nil
		}
		if v.IsF64() {
			return F64(math.Abs(v.f)), nil
		}
		if n := v.integer(); n != nil {
			if v.IsI128() && n.Cmp(minI128) == 0 {
				return Undefined(), ErrorMessage("Errored while getting absolute value: it is i128::MIN value.")
			}
			n.Abs(n)
			if v.IsI64() && n.IsInt64() {
				return I64(n.Int64()), nil
			}
			return I128(n), nil
		}
		return Undefined(), ErrorMessage("This filter can only be used on a number, received `" + v.Name() + "`.")
	case "round":
		f, e := ArgF64(v)
		if e != nil {
			return Undefined(), e
		}
		method, has, e := KwargsGet(k, "method", ArgStr)
		if e != nil {
			return Undefined(), e
		}
		precision, e := kwDefault(k, "precision", ArgI32, int32(0))
		if e != nil {
			return Undefined(), e
		}
		mult := math.Pow(10, float64(precision))
		x := mult * f
		switch {
		case !has:
			x = math.Round(x)
		case method == "ceil":
			x = math.Ceil(x)
		case method == "floor":
			x = math.Floor(x)
		default:
			return Undefined(), ErrorMessage("Invalid argument for `method`: " + method + ". Only `ceil` and `floor` are allowed. Do not fill this parameter if you want a classic round.")
		}
		return F64(x / mult), nil
	case "first", "last", "nth", "join", "sort", "unique", "group_by":
		a, e := ArgValueSlice(v)
		if e != nil {
			return Undefined(), e
		}
		switch name {
		case "first":
			if len(a) == 0 {
				return None(), nil
			}
			return a[0], nil
		case "last":
			if len(a) == 0 {
				return None(), nil
			}
			return a[len(a)-1], nil
		case "nth":
			n, e := KwargsMustGet(k, "n", ArgUsize)
			if e != nil {
				return Undefined(), e
			}
			if n >= uint(len(a)) {
				return None(), nil
			}
			return a[n], nil
		case "unique":
			out := []Value{}
			for _, x := range a {
				found := false
				for _, y := range out {
					if x.Equal(y) {
						found = true
						break
					}
				}
				if !found {
					out = append(out, x)
				}
			}
			return Array(out), nil
		case "join":
			sep, has, e := KwargsGet(k, "sep", ArgStringInput)
			if e != nil {
				return Undefined(), e
			}
			safe := st.autoescape && has && sep.IsSafe()
			for _, x := range a {
				if st.autoescape && x.IsString() && x.IsSafe() {
					safe = true
				}
			}
			parts := []string{}
			for _, x := range a {
				text := x.String()
				if safe && !x.IsUndefined() {
					text, e = st.EscapeIfNeeded(x)
					if e != nil {
						return Undefined(), e
					}
				}
				parts = append(parts, text)
			}
			separator := ""
			if has {
				separator = sep.AsStr()
				if safe {
					separator, e = sep.Rendered(st)
					if e != nil {
						return Undefined(), e
					}
				}
			}
			out := NormalString(strings.Join(parts, separator))
			if safe {
				out = out.MarkSafe()
			}
			return out, nil
		case "sort":
			if len(a) == 0 {
				return Array(nil), nil
			}
			attr, has, e := KwargsGet(k, "attribute", ArgStr)
			if e != nil {
				return Undefined(), e
			}
			type decor struct{ key, value Value }
			ds := make([]decor, len(a))
			for i, x := range a {
				key := x
				if has {
					var ok bool
					key, ok = x.GetFromPath(attr)
					if !ok {
						return Undefined(), ErrorMessage(fmt.Sprintf("Value at index %d (a %s) does not have an attribute at path: %s", i, x.Name(), attr))
					}
				}
				ds[i] = decor{key, x}
			}
			sort.SliceStable(ds, func(i, j int) bool { return ds[i].key.Compare(ds[j].key) < 0 })
			for i := 1; i < len(ds); i++ {
				x, y := ds[i-1].key, ds[i].key
				if !x.IsNone() && !y.IsNone() {
					if _, ok := x.PartialCompare(y); !ok {
						return Undefined(), ErrorMessage("Cannot sort: `" + x.Name() + "` and `" + y.Name() + "` are not comparable")
					}
				}
			}
			out := []Value{}
			for _, d := range ds {
				out = append(out, d.value)
			}
			return Array(out), nil
		case "group_by":
			m := NewMap()
			if len(a) == 0 {
				return MapValue(m), nil
			}
			attr, e := KwargsMustGet(k, "attribute", ArgStr)
			if e != nil {
				return Undefined(), e
			}
			for i, x := range a {
				key, ok := x.GetFromPath(attr)
				if !ok {
					return Undefined(), ErrorMessage(fmt.Sprintf("Value at index %d (a %s) does not have an attribute at path: %s", i, x.Name(), attr))
				}
				if key.IsNone() {
					continue
				}
				k, e := KeyFromValue(key)
				if e != nil {
					return Undefined(), e
				}
				old, _ := m.Get(k)
				m.Insert(k, Array(append(old.a, x)))
			}
			return MapValue(m), nil
		}
	case "get", "values", "keys", "pairs":
		m, e := ArgMapRef(v)
		if e != nil {
			return Undefined(), e
		}
		if name == "get" {
			key, e := KwargsMustGet(k, "key", ArgStr)
			if e != nil {
				return Undefined(), e
			}
			d, has, e := KwargsGet(k, "default", ArgValue)
			if e != nil {
				return Undefined(), e
			}
			if x, ok := m.GetString(key); ok {
				return x, nil
			}
			if has {
				return d, nil
			}
			return Undefined(), ErrorMessage("Map does not have a key " + key + " and no default values were defined")
		}
		a := []Value{}
		for _, p := range m.entries {
			switch name {
			case "keys":
				a = append(a, p.Key.AsValue())
			case "values":
				a = append(a, p.Value)
			case "pairs":
				a = append(a, Array([]Value{p.Key.AsValue(), p.Value}))
			}
		}
		return Array(a), nil
	}
	return Undefined(), ErrorMessage("Unknown filter `" + name + "`")
}
func rustReplace(s, from, to string) string {
	if from != "" {
		return strings.ReplaceAll(s, from, to)
	}
	a := []string{to}
	for _, r := range s {
		a = append(a, string(r), to)
	}
	return strings.Join(a, "")
}
func filterInt(v Value, k Kwargs) (Value, error) {
	base, e := kwDefault(k, "base", ArgU32, uint32(10))
	if e != nil {
		return Undefined(), e
	}
	if base < 2 || base > 36 {
		return Undefined(), ErrorMessage(fmt.Sprintf("int filter `base` must be between 2 and 36, got %d", base))
	}
	floatInt := func(f float64) (Value, error) {
		if n, ok := FloatNumber(f).AsInteger(); ok {
			return I128(n), nil
		}
		return Undefined(), ErrorMessage("The float " + rustrt.FloatDisplay(f) + " cannot be converted to an int (non-integer or out of i128 range)")
	}
	if v.IsString() {
		s := rustrt.Trim(v.s)
		prefix := ""
		switch base {
		case 2:
			prefix = "0b"
		case 8:
			prefix = "0o"
		case 16:
			prefix = "0x"
		}
		if prefix != "" {
			for strings.HasPrefix(s, prefix) {
				s = strings.TrimPrefix(s, prefix)
			}
		}
		valid := s != ""
		digits := s
		if strings.HasPrefix(digits, "+") || strings.HasPrefix(digits, "-") {
			digits = digits[1:]
		}
		if digits == "" {
			valid = false
		}
		for _, r := range digits {
			n := int(r - '0')
			if r >= 'a' && r <= 'z' {
				n = int(r-'a') + 10
			} else if r >= 'A' && r <= 'Z' {
				n = int(r-'A') + 10
			}
			if n < 0 || n >= int(base) {
				valid = false
			}
		}
		if n, ok := new(big.Int).SetString(s, int(base)); ok && valid && n.Cmp(minI128) >= 0 && n.Cmp(maxI128) <= 0 {
			return I128(n), nil
		}
		if strings.Contains(s, ".") {
			if f, e := rustrt.ParseFloat(s); e == nil {
				return floatInt(f)
			}
		}
		return Undefined(), ErrorMessage(fmt.Sprintf("The string `%s` cannot be converted to an int in base %d", s, base))
	}
	if v.IsF64() {
		return floatInt(v.f)
	}
	if v.IsI64() {
		return I128(big.NewInt(v.i)), nil
	}
	if v.IsNumber() {
		return v, nil
	}
	return Undefined(), ErrorMessage("Value of type " + v.Name() + " cannot be converted to an int")
}
