package tera

import (
	"fmt"
	"math/big"
)

func (t *Tera) registerFunctions() {
	t.RegisterFunction("throw", FunctionFunc(func(k Kwargs, s *State) (Value, error) {
		m, e := KwargsMustGet(k, "message", ArgStr)
		if e != nil {
			return Undefined(), e
		}
		return Undefined(), ErrorMessage(m)
	}))
	t.RegisterFunction("range", FunctionFunc(func(k Kwargs, s *State) (Value, error) {
		start, e := kwDefault(k, "start", ArgI128, big.NewInt(0))
		if e != nil {
			return Undefined(), e
		}
		end, e := KwargsMustGet(k, "end", ArgI128)
		if e != nil {
			return Undefined(), e
		}
		step, e := kwDefault(k, "step_by", ArgI128, big.NewInt(1))
		if e != nil {
			return Undefined(), e
		}
		if start.Cmp(end) > 0 && step.Sign() > 0 {
			return Undefined(), ErrorMessage("Function `range` was called with a `start` argument greater than the `end` one")
		}
		if step.Sign() == 0 {
			return Undefined(), ErrorMessage("Function `range` was called with a `step_by` argument of 0")
		}
		overflow := func() (Value, error) {
			return Undefined(), ErrorMessage("Function `range` was called with arguments that overflow i128")
		}
		span := new(big.Int).Sub(end, start)
		stp := new(big.Int).Set(step)
		if step.Sign() < 0 {
			if start.Cmp(end) <= 0 {
				return Array(nil), nil
			}
			span.Neg(span)
			stp.Neg(stp)
		}
		if span.Cmp(maxI128) > 0 || stp.Cmp(maxI128) > 0 {
			return overflow()
		}
		length := new(big.Int).Add(span, new(big.Int).Sub(stp, big.NewInt(1)))
		if length.Cmp(maxI128) > 0 {
			return overflow()
		}
		length.Quo(length, stp)
		if length.Cmp(big.NewInt(100000)) > 0 {
			return Undefined(), ErrorMessage(fmt.Sprintf("Function `range` would produce %s elements, which exceeds the limit of 100000", length))
		}
		a := []Value{}
		for i := int64(0); i < length.Int64(); i++ {
			n := new(big.Int).Add(start, new(big.Int).Mul(big.NewInt(i), step))
			a = append(a, I128(n))
		}
		return Array(a), nil
	}))
}
