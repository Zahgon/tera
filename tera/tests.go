package tera

import (
	"math/big"
	"strings"

	"github.com/Keats/tera/internal/rustrt"
)

func (t *Tera) registerTests() {
	for _, name := range strings.Fields("string number map bool array integer float none iterable defined undefined odd even divisible_by starting_with ending_with containing") {
		n := name
		t.RegisterTest(n, TestFunc(func(v Value, k Kwargs, s *State) (bool, error) {
			switch n {
			case "string":
				return v.IsString(), nil
			case "number":
				return v.IsNumber(), nil
			case "map":
				return v.IsMap(), nil
			case "bool":
				return v.IsBool(), nil
			case "array":
				return v.IsArray(), nil
			case "integer":
				return v.IsNumber() && !v.IsF64(), nil
			case "float":
				return v.IsF64(), nil
			case "none":
				return v.IsNone(), nil
			case "iterable":
				return v.IsMap() || v.IsArray() || v.IsString() || v.IsBytes(), nil
			case "defined":
				return !v.IsUndefined(), nil
			case "undefined":
				return v.IsUndefined(), nil
			case "odd", "even", "divisible_by":
				num, e := ArgNumber(v)
				if e != nil {
					return false, e
				}
				var d *big.Int
				if n == "divisible_by" {
					d, e = KwargsMustGet(k, "divisor", ArgI128)
					if e != nil {
						return false, e
					}
					if d.Sign() == 0 {
						return false, nil
					}
				}
				if num.isFloat {
					msg := "cannot determine if it's " + n
					if n == "divisible_by" {
						msg = "cannot check divisibility"
					}
					return false, ErrorMessage("Value `" + rustrt.FloatDisplay(num.float) + "` is a float; " + msg)
				}
				if n == "divisible_by" {
					return new(big.Int).Rem(num.integer, d).Sign() == 0, nil
				}
				odd := num.integer.Bit(0) == 1
				if n == "odd" {
					return odd, nil
				}
				return !odd, nil
			case "starting_with", "ending_with":
				s, e := ArgStr(v)
				if e != nil {
					return false, e
				}
				pat, e := KwargsMustGet(k, "pat", ArgStr)
				if e != nil {
					return false, e
				}
				if n == "starting_with" {
					return strings.HasPrefix(s, pat), nil
				}
				return strings.HasSuffix(s, pat), nil
			case "containing":
				pat, e := KwargsMustGet(k, "pat", ArgValue)
				if e != nil {
					return false, e
				}
				if v.IsString() {
					x, e := ArgStr(pat)
					if e != nil {
						return false, e
					}
					return strings.Contains(v.s, x), nil
				}
				if v.IsArray() || v.IsMap() {
					return v.contains(pat)
				}
				return false, ErrorMessage("Value `" + v.String() + "` is not a container; cannot check for containment")
			}
			return false, nil
		}))
	}
}
