package tera

import (
	"fmt"
	"math"
	"math/big"

	"github.com/Keats/tera/internal/rustrt"
)

// Number holds either an i128 arithmetic integer or a float.
type Number struct {
	integer *big.Int
	float   float64
	isFloat bool
}

// IntegerNumber constructs an integer Number.
func IntegerNumber(n *big.Int) Number {
	if n == nil {
		return Number{integer: big.NewInt(0)}
	}
	return Number{integer: new(big.Int).Set(n)}
}

// FloatNumber constructs a float Number.
func FloatNumber(f float64) Number { return Number{float: f, isFloat: true} }

// IsFloat checks the float variant.
func (n Number) IsFloat() bool { return n.isFloat }

// IsInteger checks the integer variant.
func (n Number) IsInteger() bool { return !n.isFloat }

// AsFloat converts to f64.
func (n Number) AsFloat() float64 {
	if n.isFloat {
		return n.float
	}
	if n.integer == nil {
		return 0
	}
	f, _ := new(big.Float).SetInt(n.integer).Float64()
	return f
}

// AsInteger converts an exact integral number to i128.
func (n Number) AsInteger() (*big.Int, bool) {
	if !n.isFloat {
		if n.integer == nil {
			return big.NewInt(0), true
		}
		return new(big.Int).Set(n.integer), true
	}
	f := n.float
	if math.IsNaN(f) || math.IsInf(f, 0) || math.Trunc(f) != f || f < -math.Exp2(127) || f >= math.Exp2(127) {
		return nil, false
	}
	z, _ := new(big.Float).SetFloat64(f).Int(nil)
	return z, true
}

// IntoValue converts to the corresponding Value variant.
func (n Number) IntoValue() Value {
	if n.isFloat {
		return F64(n.float)
	}
	if n.integer == nil {
		return I128(big.NewInt(0))
	}
	return I128(n.integer)
}

// String returns the Rust-compatible text representation.
func (n Number) String() string {
	if n.isFloat {
		return rustrt.FloatDisplay(n.float)
	}
	if n.integer == nil {
		return "0"
	}
	return n.integer.String()
}
func mathArgError(v Value) error {
	if v.IsNumber() {
		return ErrorMessage(fmt.Sprintf("Operand `%s` is out of range for integer arithmetic (must fit in i128)", v))
	}
	return ErrorMessage(fmt.Sprintf("Only numbers can be used in arithmetic. This is a `%s`", v.Name()))
}
func arithmetic(op string, a, b Value) (Value, error) {
	x, ok := a.AsNumber()
	if !ok {
		return Undefined(), mathArgError(a)
	}
	y, ok := b.AsNumber()
	if !ok {
		return Undefined(), mathArgError(b)
	}
	if (op == "/" || op == "//" || op == "%") && y.AsFloat() == 0 {
		return Undefined(), ErrorMessage("Cannot divide by 0")
	}
	if op == "/" {
		return F64(x.AsFloat() / y.AsFloat()), nil
	}
	if x.isFloat || y.isFloat || op == "**" && y.integer.Sign() < 0 {
		a, b := x.AsFloat(), y.AsFloat()
		switch op {
		case "+":
			return F64(a + b), nil
		case "-":
			return F64(a - b), nil
		case "*":
			return F64(a * b), nil
		case "**":
			return F64(rustrt.Pow(a, b)), nil
		case "%":
			r := math.Mod(a, b)
			if r < 0 {
				r += math.Abs(b)
			}
			return F64(r), nil
		case "//":
			q := math.Trunc(a / b)
			r := math.Mod(a, b)
			if r < 0 {
				if b > 0 {
					q--
				} else {
					q++
				}
			}
			return F64(q), nil
		}
	}
	if (op == "%" || op == "//") && x.integer.Cmp(minI128) == 0 && y.integer.Cmp(big.NewInt(-1)) == 0 {
		return Undefined(), ErrorMessage(fmt.Sprintf("Unable to perform %s %s %s", a, op, b))
	}
	z := new(big.Int)
	switch op {
	case "+":
		z.Add(x.integer, y.integer)
	case "-":
		z.Sub(x.integer, y.integer)
	case "*":
		z.Mul(x.integer, y.integer)
	case "%", "//":
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(x.integer, y.integer, r)
		if r.Sign() < 0 {
			if y.integer.Sign() > 0 {
				q.Sub(q, big.NewInt(1))
				r.Add(r, y.integer)
			} else {
				q.Add(q, big.NewInt(1))
				r.Sub(r, y.integer)
			}
		}
		if op == "%" {
			z = r
		} else {
			z = q
		}
	case "**":
		if !y.integer.IsUint64() || y.integer.Uint64() > math.MaxUint32 {
			return Undefined(), ErrorMessage(fmt.Sprintf("Exponent %s is out of range for integer ** (must fit in u32)", y.integer))
		}
		exp := y.integer.Uint64()
		base := new(big.Int).Set(x.integer)
		z.SetInt64(1)
		for exp > 0 {
			if exp&1 != 0 {
				z.Mul(z, base)
				if z.Cmp(minI128) < 0 || z.Cmp(maxI128) > 0 {
					break
				}
			}
			exp >>= 1
			if exp > 0 {
				base.Mul(base, base)
				if base.BitLen() > 128 && x.integer.Sign() != 0 {
					z.Set(new(big.Int).Lsh(big.NewInt(1), 128))
					break
				}
			}
		}
	}
	if z.Cmp(minI128) < 0 || z.Cmp(maxI128) > 0 {
		return Undefined(), ErrorMessage(fmt.Sprintf("Unable to perform %s %s %s", a, op, b))
	}
	return I128(z), nil
}
func negate(v Value) (Value, error) {
	n, ok := v.AsNumber()
	if !ok {
		if v.IsNumber() {
			return Undefined(), ErrorMessage(fmt.Sprintf("Cannot negate %s: result would overflow i128", v))
		}
		return Undefined(), ErrorMessage(fmt.Sprintf("Only numbers can be negated. This is a `%s`", v.Name()))
	}
	if n.isFloat {
		return F64(-n.float), nil
	}
	if n.integer.Cmp(minI128) == 0 {
		return Undefined(), ErrorMessage(fmt.Sprintf("Cannot negate %s: result would overflow i128", v))
	}
	return I128(new(big.Int).Neg(n.integer)), nil
}

// Equal compares arithmetic numbers across integer and float variants.
func (n Number) Equal(o Number) bool { return n.IntoValue().Equal(o.IntoValue()) }

// Compare compares arithmetic numbers with Tera's numeric ordering.
func (n Number) Compare(o Number) int { return n.IntoValue().Compare(o.IntoValue()) }
